package posse

// Every beads invocation goes through Bd.run — the third substrate runner
// (herdr = presentation/oversight, beads = durable work graph). posse
// never grows an issue tracker; it grows a dispatcher over bd. Beads
// databases are per-repo (`bd init`), so calls carry a working directory.
//
// Persona binding: a persona's durable identity is its beads assignee name
// (herdr agent names die with the process). `bd ready --assignee/--label`
// is the routing surface the dispatch loop selects work with.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Bd struct {
	Bin string // bd binary; RHQ_BD_BIN overrides (testing)

	// The child deadline (ranger-base-wj7e9), zero in production and set
	// only by the pin — see BdTimeout and runOnce. Hangw takes the one line
	// a blown deadline writes; nil = os.Stderr.
	Timeout time.Duration // 0 = BdTimeout
	Hangw   io.Writer
}

func NewBd() Bd {
	bin := os.Getenv("RHQ_BD_BIN")
	if bin == "" {
		bin = "bd"
	}
	return Bd{Bin: bin}
}

func (b Bd) Available() bool {
	_, err := exec.LookPath(b.Bin)
	return err == nil
}

// bdGlobalFlags go in front of every verb Bd.run invokes.
//
// `--no-daemon` is a 12x speedup, not a preference (ranger-base-cwu7,
// measured 2026-08-30 on bd 0.49.1 against the fleet's own store, 1275
// issues): every store-touching call cost ~5.6s, and ~5.3s of that was bd
// trying to reach a daemon it could not start — flat across result size,
// so one row and 180 rows cost the same. `--no-auto-import`,
// `--no-auto-flush` and `--allow-stale` each changed nothing; `--sandbox`
// matched `--no-daemon` exactly, so the daemon dial is the whole of it.
// The same calls with the flag: 0.36-0.49s. That is 5.3s off each of the
// cockpit's two scans, off every dispatch pass and off every `posse status`.
//
// It is not a semantic change here, and that is measured rather than
// assumed: `bd info` in the configured beads dir already answers
// `Mode: direct, Connected: no` — bd pays the full dial, gives up, and does
// the work directly anyway. Same store, same rows. Writes and
// `sync --flush-only` behave the same way and still export to JSONL, so the
// flag belongs on the runner rather than on a hand-picked list of read
// verbs; scripts/verify-bd-pin.sh and scripts/queue-cutover.sh already
// spell our calls this way, and CageBdFlags carries it into the cage.
//
// The one delta, for whoever revisits this once a daemon CAN connect
// here: a live daemon auto-imports a JSONL newer than the
// database, and direct mode refuses instead — "Database out of sync with
// JSONL". It fails CLOSED and loudly, it is already what our scans get
// today because the daemon does not start, and it is the side this repo
// has twice chosen on its own: worktree.go declines to hand a persona
// `--allow-stale` or `bd sync --import-only` for that same message, and
// WarnLostBeads (rangerhq-fuom) exists because that auto-import can delete
// rows and log nothing when it does.
//
// UPDATED ranger-base-p969: "twice chosen" above was about a persona
// hand-walking a *different* trap — the worktree's own materialized jsonl,
// which never actually triggers this check (worktree.go). This is the real
// thing, and refusing it unconditionally turned out to be the wrong default
// for a --no-daemon reader running alongside daemon-path writers. Measured
// 2026-08-30 (ranger-base-p969): a dispatch --watch pass failed its ready
// scan on this exact message nine times across ~20 minutes, triggered by
// ANY daemon-path bd write (close/create/comment) from another actor in the
// same repo in roughly the preceding ten minutes — and a `sync
// --import-only` run immediately after a failure, by hand, repeatedly
// reported "0 created / 0 updated": nothing was actually stale. The refusal
// is a timestamp/marker check, not a content check, so a daemon flush that
// rewrites issues.jsonl without changing it still trips it. WarnLostBeads
// runs every pass regardless of why an import happened, so it remains the
// backstop for the case this staleness check exists to catch — an import
// that silently drops rows; it just no longer stands between that backstop
// and the pass being able to see the queue at all. So `run` now treats this
// one message as recoverable: import once, retry the call once. A second
// failure (including a second "out of sync") is returned as-is — no loop.
var bdGlobalFlags = []string{"--no-daemon"}

// staleDBMessage is the substring bd 0.49.1 puts in both stdout and stderr
// when a --no-daemon reader finds issues.jsonl newer than the database it
// resolved to and refuses rather than importing (worktree.go, beads.go
// above). It is the one bd error `run` treats as self-healing rather than
// fatal.
const staleDBMessage = "Database out of sync with JSONL"

func (b Bd) run(dir string, args ...string) ([]byte, error) {
	out, err := b.runOnce(dir, args...)
	if err == nil || !strings.Contains(err.Error(), staleDBMessage) {
		return out, err
	}
	// The import call goes through runOnce directly, not run: a failure here
	// (e.g. "database is locked") must not itself be treated as a stale-db
	// hit and retried, or a lock contest turns into a loop.
	if _, ierr := b.runOnce(dir, "sync", "--import-only"); ierr != nil {
		return nil, err
	}
	return b.runOnce(dir, args...)
}

// ─── the child deadline (ranger-base-wj7e9) ──────────────────────────────────
//
// The bead was filed over a herdr child a --watch loop held for 7h11m on an
// unbounded exec.Command — a sleep and not a hang, as herdr.go's block
// records — and its ask is "every exec of herdr (and any other child) from
// the watch loop". bd is the other one: a pass reads the
// ready set, shows a bead, syncs and comments, all through this runner, and
// a store that stops answering would wedge the loop in exactly the same way
// and for exactly the same reason. No bd hang has been observed here; the
// deadline is not a diagnosis, it is the same missing clock.
//
// MEASURED on this box 2026-09-03 against the fleet's own store: `bd
// --no-daemon ready --json` answers in 177ms. BdTimeout is a thousand times
// that, because it also has to cover the slowest thing this runner does —
// `sync --import-only` over the whole JSONL, which `run` fires on its own
// after a stale-db refusal.
//
// UNLIKE the herdr one, the cancel is a SIGTERM with a kill behind it. bd
// writes SQLite, and half of these calls are writes: a TERM gives it the
// chance to close the database and roll the WAL back cleanly, and WaitDelay
// is what makes sure a bd that ignores the TERM is still not this loop's
// problem.
const BdTimeout = 3 * time.Minute

// bdKillGrace is how long a TERMed bd has to exit before it is killed, and
// how long os/exec will keep waiting on pipes a descendant still holds.
const bdKillGrace = 10 * time.Second

// BdHangError is a bd child that blew its deadline and was signalled. Typed
// for the same reason HerdrHangError is: "bd said no" and "bd said nothing
// at all" are different facts, and only the second one is about this box.
type BdHangError struct {
	Argv   []string
	Dir    string
	Limit  time.Duration
	Waited time.Duration
}

func (e *BdHangError) Error() string {
	where := e.Dir
	if where == "" {
		where = "."
	}
	return fmt.Sprintf("bd child hung: %s (in %s) — no answer in %s (deadline %s), signalled; the caller is not waiting on it",
		strings.Join(e.Argv, " "), where, e.Waited.Round(time.Millisecond), e.Limit)
}

// IsBdHang reports whether err is a blown bd child deadline.
func IsBdHang(err error) bool {
	var be *BdHangError
	return errors.As(err, &be)
}

func (b Bd) hangw() io.Writer {
	if b.Hangw != nil {
		return b.Hangw
	}
	return os.Stderr
}

func (b Bd) timeout() time.Duration {
	if b.Timeout > 0 {
		return b.Timeout
	}
	return BdTimeout
}

func (b Bd) runOnce(dir string, args ...string) ([]byte, error) {
	argv := append(append([]string{}, bdGlobalFlags...), args...)
	limit := b.timeout()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, b.Bin, argv...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = bdKillGrace
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	started := time.Now()
	err := cmd.Run()
	if ctx.Err() != nil {
		hang := &BdHangError{Argv: append([]string{b.Bin}, argv...), Dir: dir, Limit: limit, Waited: time.Since(started)}
		fmt.Fprintf(b.hangw(), "◷ %s\n", hang.Error())
		return nil, hang
	}
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = bdStdoutError(out.Bytes())
		}
		if msg == "" {
			msg = err.Error()
		}
		// The full argv, flags included: an operator who retypes the verb
		// without them gets the daemon path, and possibly a different answer.
		return nil, Die("bd %s: %s", strings.Join(argv, " "), msg)
	}
	return out.Bytes(), nil
}

// bdStdoutError recovers the reason from a --json verb that reported its
// failure on STDOUT. bd 0.49.1 prints `{"error": "..."}` there and leaves
// stderr empty for at least `dep list` (measured, rangerhq-aas), so a
// runner that reads stderr only hands the operator "exit status 1" and
// drops the sentence that says what went wrong.
//
// It returns "" for anything that is not that shape — a JSON array, a
// half-written page, an object with no error key — so the caller keeps its
// err.Error() fallback rather than quoting a payload at the operator.
func bdStdoutError(stdout []byte) string {
	var v struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(bytes.TrimSpace(stdout), &v) != nil {
		return ""
	}
	return strings.TrimSpace(v.Error)
}

type BdIssue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	IssueType   string    `json:"issue_type"`
	Assignee    string    `json:"assignee"`
	Labels      []string  `json:"labels"`
	Created     time.Time `json:"created_at"`
	Updated     time.Time `json:"updated_at"`
	// Scorecard fields (rangerhq-h2c). CreatedBy is the bd actor at create
	// time — a persona's name when it filed the bead from its session.
	CreatedBy   string     `json:"created_by"`
	Owner       string     `json:"owner"`
	ClosedAt    *time.Time `json:"closed_at"`
	CloseReason string     `json:"close_reason"`
	// DeferUntil is the date `bd defer --until` parks a bead until
	// (ranger-base-5aln): a defer with a future date is an answer someone
	// already gave — the answer is a date — not silence.
	//
	// It is orthogonal to Status, which `bd defer` does not touch
	// (ranger-base-03ada): on 0.50.3 a deferred bead reads back status
	// "open" with a date, and a status "deferred" bead can carry no date
	// at all. Read this field alone; never gate it on the status string.
	DeferUntil *time.Time `json:"defer_until"`
}

// ─── the work class (ADR 0006 §1, amended 2026-09-02) ────────────────────────
//
// One reader for one rule, because three surfaces answer with it and a
// second derivation is a second answer: the scorecard's class census
// (beadpulse.go), the shop pulse line `posse status`, the cockpit and the
// watch log print, and verify-after's filer, which stamps the class it
// inherits onto the bead it mints. Read through BeadClass; never re-derive
// from a title, a graph, or any label but `debt`.
//
// The rule, in the ADR's order — issue_type wins, then the label, then
// nothing:
//
//	issue_type feature -> feature
//	issue_type bug     -> bug
//	label debt         -> debt
//	otherwise          -> unclassified
//
// Type wins over the label deliberately: `-t bug` carrying `-l debt` is a
// filing error the groom clears, and until it does the bead is a bug. And
// unclassified is a REPORTED bucket, never an inferred class — a shop where
// most beads are unclassified must see that number, not a census quietly
// rounded into feature/bug/debt (ranger-base-dwlb1; 0 of 153 open beads
// carried `debt` on the day this was written, so the gap is the whole
// point).
const (
	ClassFeature      = "feature"
	ClassBug          = "bug"
	ClassDebt         = "debt"
	ClassUnclassified = "unclassified"
)

// BeadClasses is the REPORTING order, and it is the one the shop pulse line
// spells (`open 19F/59B/52D/13U`), so an eye moving from the line to the
// scorecard's table does not have to re-map the columns. It is deliberately
// NOT a precedence: verify-after picks the most urgent class of a batch in
// its own order (bug, feature, debt, unclassified — ADR 0006 §3), which is a
// different question from what order a census prints in.
var BeadClasses = []string{ClassFeature, ClassBug, ClassDebt, ClassUnclassified}

// BeadClass is the one reader of the class rule above.
func BeadClass(is BdIssue) string {
	switch is.IssueType {
	case ClassFeature:
		return ClassFeature
	case ClassBug:
		return ClassBug
	}
	for _, l := range is.Labels {
		if strings.TrimSpace(l) == ClassDebt {
			return ClassDebt
		}
	}
	return ClassUnclassified
}

// Flush exports the database to its JSONL projection and leaves git alone:
// `bd sync --flush-only` is documented as "only export pending changes to
// JSONL (skip git operations)", where a plain `bd sync` also skips them
// today but does not say so in its name, and `bd sync --full` commits AND
// pushes (measured on 0.49.1, `bd sync --help`) — which no persona and no
// launcher of ours may do.
//
// It is what the launcher calls before committing the projection in the
// queue repo (ADR 0015 §4, queuejsonl.go): the database is the store of
// record, so committing without exporting first commits the state from
// before the close.
func (b Bd) Flush(dir string) error {
	_, err := b.run(dir, "sync", "--flush-only")
	return err
}

// ListAll returns every issue in the repo, closed included (bd list --all,
// no limit) — the scorecard's raw material.
func (b Bd) ListAll(dir string) ([]BdIssue, error) {
	out, err := b.run(dir, "list", "--all", "--json", "--limit", "0")
	if err != nil {
		return nil, err
	}
	return parseBdIssues(out)
}

// Ready lists unblocked open work in the repo at dir, optionally filtered to
// one assignee (persona) — the head of the dispatch loop. --limit 0 lifts
// bd's default cap of 10 (rangerhq-47v): dispatch and the cockpit must see
// every unblocked bead, not the first page.
//
// It is `bd ready` MINUS `bd blocked`, because `bd ready` alone does not
// mean unblocked in every store bd makes (ranger-base-lpz0o). MEASURED
// 2026-09-01, one bd 0.50.3 binary, two stores, the same argv:
//
//	a store `bd init` writes today — .beads/config.yaml carries
//	`no-db: true`, JSONL only, no beads.db at all: `dep add <trigger>
//	<blocker>` against a blocker already holding `discovered-from:<trigger>`
//	is ACCEPTED, no cycle check, and <trigger> then comes back in `bd ready`
//	AND in `bd blocked` at once.
//
//	a SQLite beads.db (the operator's queue, and every repo an older bd
//	inited): the same add is REFUSED — "cannot add dependency: would create
//	a cycle", exit 1 — so the edge never lands and <trigger> stays ready.
//
// Either way the block does not take. Only its loudness varies, and which
// one a repo gets is a property of the store, not of the bd version — so no
// caller of ours may reason from the refusal, and none may take `bd ready`
// as the definition of unblocked. This asks the store its own "what is
// stuck" question and subtracts the answer, which is right under both
// shapes: on SQLite `bd blocked` lists nothing extra, and on the JSONL store
// it lists exactly the bead `bd ready` should not have.
//
// It cannot hide work whose blocker is gone: closing the blocker takes the
// bead out of `bd blocked` immediately — measured in both stores and both
// arms, with `bd ready` returning it again in the same breath.
//
// A `bd blocked` that fails makes the repo's queue UNKNOWN, not ready
// (rangerhq-llse), so the error is returned rather than swallowed: serving
// the raw `bd ready` set on a failed cross-check would put the silent shape
// back exactly when the store is least trustworthy. It costs one extra read
// per call — `bd blocked --json` measured at 0.13-0.17s against the 1551-bead
// queue db, the same as `bd ready` itself.
func (b Bd) Ready(dir, assignee string) ([]BdIssue, error) {
	args := []string{"ready", "--json", "--limit", "0"}
	if assignee != "" {
		args = append(args, "--assignee", assignee)
	}
	out, err := b.run(dir, args...)
	if err != nil {
		return nil, err
	}
	issues, err := parseBdIssues(out)
	if err != nil {
		return nil, err
	}
	blocked, err := b.Blocked(dir)
	if err != nil {
		return nil, err
	}
	if len(blocked) == 0 {
		return issues, nil
	}
	stuck := make(map[string]bool, len(blocked))
	for _, r := range blocked {
		stuck[r.ID] = true
	}
	kept := issues[:0]
	for _, is := range issues {
		if !stuck[is.ID] {
			kept = append(kept, is)
		}
	}
	return kept, nil
}

// InProgress lists the repo's claimed work — every bead somebody is holding
// (`bd list --status in_progress`). The mirror of Ready, and the raw material
// of the cockpit's IN PROGRESS section (ADR 0004 §2): ready work is what is
// waiting, this is what is being done. --limit 0 for the same reason Ready
// carries it (rangerhq-47v) — the cockpit must see every held bead, not the
// first page.
func (b Bd) InProgress(dir string) ([]BdIssue, error) {
	out, err := b.run(dir, "list", "--status", "in_progress", "--json", "--limit", "0")
	if err != nil {
		return nil, err
	}
	return parseBdIssues(out)
}

// OpenLabeledAny lists the repo's OPEN issues carrying at least one of the
// labels (`bd list --label-any a,b`). It is the governance surface's G3
// query: `-l question` / `-l risk` beads are decisions waiting on a human,
// and unlike Ready they are wanted whether or not they are unblocked — a
// question that is itself dep-blocked is still a question nobody answered.
//
// OPEN IS THIS FUNCTION'S PROMISE, NOT bd's. `bd list --label-any` drops
// closed rows on the shop's SQLite store (391 of 396 `-l qa` beads are
// closed and 5 come back) and KEEPS them on the `no-db: true` JSONL store
// `bd init` writes on bd 0.50.3 — both measured 2026-09-04 (ranger-base-
// bwrp8, found by ciwatch_live_test.go, which runs against a store of the
// second class and failed on it). Every reader here is asking what is still
// WAITING, and a closed bead is answered: G3 would count answered questions
// into a gate it is holding open, and the closed-dirty handoff's dedupe
// would adopt a handoff somebody already finished and never file the next
// one. So the closed rows are dropped HERE, where the doc comment promises
// it, on both store classes.
//
// The drop is a filter and not `--status open`, which would be a narrowing:
// bd's statuses are open, in_progress, blocked, deferred and closed, and
// `--status open` answers with only the first — measured on the shop store,
// `--label-any qa` is 3 open and 2 in_progress, and a held question is
// still unanswered. Only `closed` is an answer.
//
// --limit 0 for the same reason Ready carries it (rangerhq-47v): a capped
// page of a set the caller is counting is a silent undercount. A cap would
// apply BEFORE this filter runs, so on the keep-closed store it would be
// spent on rows about to be dropped — the reason this stays uncapped rather
// than merely paged.
func (b Bd) OpenLabeledAny(dir string, labels ...string) ([]BdIssue, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	out, err := b.run(dir, "list", "--label-any", strings.Join(labels, ","), "--json", "--limit", "0")
	if err != nil {
		return nil, err
	}
	issues, err := parseBdIssues(out)
	if err != nil {
		return nil, err
	}
	open := issues[:0]
	for _, is := range issues {
		if is.Status == "closed" {
			continue
		}
		open = append(open, is)
	}
	return open, nil
}

// AllLabeledAny is the same query with `--all` and no open filter: every
// issue carrying one of the labels, CLOSED ONES INCLUDED — and included on
// both store classes, where OpenLabeledAny excludes them on both.
//
// It exists for one reader — the merge-back handoff's dedupe
// (priorMergeBlocked) — and the two queries are kept apart rather than
// merged because the callers want opposite things from a closed row.
// OpenLabeledAny's readers (governance G3, the closed-dirty handoff) are
// asking what is still WAITING, and a closed bead is answered. This one is
// asking what has already been ANSWERED, and the answer is the whole point:
// a merge-back block closed do-not-land is a verdict, and re-asking costs a
// dispatched seat (ranger-base-j8qmj).
//
// --limit 0 for the reason Ready carries it and for a sharper one here:
// bd's default cap is 50 and the closed rows are the old ones, so a capped
// page of `--all` is exactly the page with the verdicts missing.
func (b Bd) AllLabeledAny(dir string, labels ...string) ([]BdIssue, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	out, err := b.run(dir, "list", "--all", "--label-any", strings.Join(labels, ","), "--json", "--limit", "0")
	if err != nil {
		return nil, err
	}
	return parseBdIssues(out)
}

// BdBlocked is one row of `bd blocked --json`: an issue that is not in
// `bd ready` because something else is holding it, and the ids doing the
// holding.
type BdBlocked struct {
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	BlockedBy []string `json:"blocked_by"`
}

// Blocked lists every blocked issue in the repo with its blockers — ONE
// call for the whole graph, where `dep list --direction=up` is one call per
// bead asked about.
//
// That difference is the whole reason this exists. The governance surface's
// G3 asks "does this question hold work out of the queue" of every aging
// question bead; on the store this was written against that was 24 `dep
// list` calls per view (~4.5s of the ~5.6s measured, rangerhq-81y0), and
// this is one. It is also the better question: `dep list --direction=up`
// answers "what points at me", while this answers "what is actually stuck",
// which is what the row means.
func (b Bd) Blocked(dir string) ([]BdBlocked, error) {
	out, err := b.run(dir, "blocked", "--json")
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	var bl []BdBlocked
	if err := json.Unmarshal(trimmed, &bl); err != nil {
		return nil, Die("bd blocked: bad JSON: %v", err)
	}
	return bl, nil
}

// BdComment is one row of `bd comments <id> --json`. The text is what the
// escalation ladder's protocol prefixes (`BLOCKED:`, `REFUSED:`) are read
// out of — see govern.go's ladderSubtype.
type BdComment struct {
	ID      int       `json:"id"`
	IssueID string    `json:"issue_id"`
	Author  string    `json:"author"`
	Text    string    `json:"text"`
	Created time.Time `json:"created_at"`
}

// Comments returns an issue's comments, oldest first (bd's own order). The
// mirror of CommentCount, which is deliberately kept: a caller that only
// wants "are there any" should not pay to decode them.
func (b Bd) Comments(dir, id string) ([]BdComment, error) {
	out, err := b.run(dir, "comments", id, "--json")
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	var cs []BdComment
	if err := json.Unmarshal(trimmed, &cs); err != nil {
		return nil, Die("bd comments %s: bad JSON: %v", id, err)
	}
	return cs, nil
}

// ClaimLostError reports that the bead is held by somebody else: the claim
// went to another actor. Callers that race for work (dispatch) treat this as
// a clean skip; every other claim error is a real failure.
type ClaimLostError struct {
	ID     string
	Holder string
}

func (e ClaimLostError) Error() string {
	if e.Holder == "" {
		return fmt.Sprintf("claim on %s lost", e.ID)
	}
	return fmt.Sprintf("claim on %s lost: held by %s", e.ID, e.Holder)
}

// Claim claims an issue for actor, reporting resumed=true when the bead was
// already this actor's rather than freshly won. actor becomes the assignee —
// pass a persona name to claim on its behalf; "" uses bd's default actor
// (BD_ACTOR / git user).
//
// The exit code cannot decide this (rangerhq-kux): bd 0.49.1 refuses a claim
// with "already claimed by X" on stderr, empty stdout — and exit 0. So the
// outcome is read from the bead itself. A won claim prints the updated issue;
// anything else is settled by reading the bead back, which also covers the
// assignee-routed case: bd refuses --claim on a bead already assigned to this
// actor and leaves it open, so the status is set here instead.
func (b Bd) Claim(dir, id, actor string) (resumed bool, err error) {
	out, runErr := b.run(dir, bdArgs(actor, "update", id, "--claim", "--json")...)
	if runErr == nil {
		// The claim was won iff bd handed back the claimed issue.
		if issues, perr := parseBdIssues(out); perr == nil && len(issues) == 1 &&
			issues[0].Status == "in_progress" && (actor == "" || issues[0].Assignee == actor) {
			return false, nil
		}
	}
	cur, showErr := b.Show(dir, id)
	if showErr != nil {
		if runErr != nil {
			return false, runErr
		}
		return false, showErr
	}
	if cur.Assignee == "" {
		// Nobody holds it and no claim came back: the update did not take.
		if runErr != nil {
			return false, runErr
		}
		return false, Die("bd update %s --claim: no claim taken (status %q, unassigned)", id, cur.Status)
	}
	// With no actor name there is nothing to compare the holder against, so
	// a refused claim reads as lost — the honest answer for `posse claim` and
	// the cockpit, which claim as the operator.
	if actor == "" || cur.Assignee != actor {
		return false, ClaimLostError{ID: id, Holder: cur.Assignee}
	}
	// Held by this actor already: an interrupted run, or a bead routed by
	// its assignee that bd will not --claim. The work is ours either way;
	// make the status say so rather than leaving it open forever.
	if cur.Status == "open" {
		if _, err := b.run(dir, bdArgs(actor, "update", id, "--status", "in_progress", "--json")...); err != nil {
			return false, err
		}
	}
	return true, nil
}

// bdArgs prefixes bd's global --actor flag (bd wants it before the verb).
func bdArgs(actor string, rest ...string) []string {
	if actor == "" {
		return rest
	}
	return append([]string{"--actor", actor}, rest...)
}

// Unclaim reverts a Claim: status back to open, assignee cleared. Used when
// the dispatcher claimed a bead on a persona's behalf but the prompt never
// reached the agent (rangerhq-81d) — leaving it claimed strands the bead
// as in_progress with nobody working it.
//
// keepAssignee leaves the assignee in place: the bead was resumed, not
// claimed by this pass, so its routing was somebody else's decision (usually
// the operator's) and clearing it would throw that away (rangerhq-kux).
func (b Bd) Unclaim(dir, id, actor string, keepAssignee bool) error {
	args := []string{"update", id, "--status", "open"}
	if !keepAssignee {
		args = append(args, "--assignee", "")
	}
	_, err := b.run(dir, bdArgs(actor, append(args, "--json")...)...)
	return err
}

// Show fetches one issue's current state (bd show returns a one-item array).
func (b Bd) Show(dir, id string) (BdIssue, error) {
	out, err := b.run(dir, "show", id, "--json")
	if err != nil {
		return BdIssue{}, err
	}
	issues, err := parseBdIssues(out)
	if err != nil || len(issues) == 0 {
		return BdIssue{}, Die("bd show %s: no issue in response", id)
	}
	return issues[0], nil
}

// BdDep is one entry of `bd dep list <id> --json`: a parent of the issue
// with the relation type (blocks | discovered-from | parent-child | …).
type BdDep struct {
	BdIssue
	DependencyType string `json:"dependency_type"`
}

// DepList returns the issue's dependencies (parents) with relation types.
func (b Bd) DepList(dir, id string) ([]BdDep, error) {
	return b.deps(dir, id, "")
}

// DepAdd makes id depend on (be blocked by) blocker: `bd dep add <id>
// <blocker>`, whose default type is `blocks`. It is the ASK rung's own
// mechanism — the edge that takes a bead out of `bd ready` until a question
// is answered — and dispatch files one for a bead it has stopped
// re-prompting (settleopen.go).
//
// The exit code is deliberately NOT the whole answer for callers: an edge
// that is already there is a refusal with nothing wrong. Callers that care
// read the graph back.
func (b Bd) DepAdd(dir, id, blocker, actor string) error {
	_, err := b.run(dir, bdArgs(actor, "dep", "add", id, blocker)...)
	return err
}

// Dependents returns the issues that depend on this one — the child side of
// a handoff edge (ADR 0006 §1: `discovered-from` is the edge a handoff
// leaves behind). `bd dep list --direction=up`.
func (b Bd) Dependents(dir, id string) ([]BdDep, error) {
	return b.deps(dir, id, "up")
}

func (b Bd) deps(dir, id, direction string) ([]BdDep, error) {
	args := []string{"dep", "list", id, "--json"}
	if direction != "" {
		args = append(args, "--direction="+direction)
	}
	out, err := b.run(dir, args...)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	var deps []BdDep
	if err := json.Unmarshal(trimmed, &deps); err != nil {
		return nil, Die("bd dep list %s: bad JSON: %v", id, err)
	}
	return deps, nil
}

// CommentCount returns how many comments an issue has (0 on any error —
// the prompt only says "comments carry decisions" when it knows).
func (b Bd) CommentCount(dir, id string) int {
	out, err := b.run(dir, "comments", id, "--json")
	if err != nil {
		return 0
	}
	var cs []json.RawMessage
	if json.Unmarshal(bytes.TrimSpace(out), &cs) != nil {
		return 0
	}
	return len(cs)
}

// Close marks an issue done.
func (b Bd) Close(dir, id, actor string) error {
	_, err := b.run(dir, bdArgs(actor, "close", id, "--json")...)
	return err
}

// BdNew is a bead to file: the fields the harness itself ever sets. The
// only writer today is verify-after (ADR 0006 §3) — personas file their own
// beads by typing `bd create`, which is the point.
type BdNew struct {
	Title       string
	Description string
	Assignee    string
	Labels      []string
	Deps        []string // "type:id" — e.g. "discovered-from:rangerhq-8q3"
	Priority    string   // "" = bd's default
	// Type is bd's `-t`: bug | feature | task | epic | chore | decision
	// (measured on bd 0.50.3's `bd create --help`). "" passes no flag and
	// leaves bd's own default, which is `task` — and `task` with no `debt`
	// label is the UNCLASSIFIED bucket ADR 0006 §1 reports rather than
	// guesses at, so an empty Type is an answer here, not a gap.
	Type  string
	Actor string // bd audit actor
}

// Create files a bead and returns its id. bd --json answers with the created
// issue as an object; the array form and a bare id are both accepted so a bd
// that changes its mind does not break the caller.
func (b Bd) Create(dir string, n BdNew) (string, error) {
	args := []string{"create", n.Title, "--json"}
	if n.Description != "" {
		args = append(args, "-d", n.Description)
	}
	if n.Assignee != "" {
		args = append(args, "-a", n.Assignee)
	}
	if len(n.Labels) > 0 {
		args = append(args, "-l", strings.Join(n.Labels, ","))
	}
	if len(n.Deps) > 0 {
		args = append(args, "--deps", strings.Join(n.Deps, ","))
	}
	if n.Priority != "" {
		args = append(args, "-p", n.Priority)
	}
	if n.Type != "" {
		args = append(args, "-t", n.Type)
	}
	out, err := b.run(dir, bdArgs(n.Actor, args...)...)
	if err != nil {
		return "", err
	}
	// bd prints its advisory notes on stderr, but a note that ever lands on
	// stdout must not cost us the id: start at the first JSON delimiter.
	trimmed := bytes.TrimSpace(out)
	if i := bytes.IndexAny(trimmed, "{["); i > 0 {
		trimmed = trimmed[i:]
	}
	var one BdIssue
	if json.Unmarshal(trimmed, &one) == nil && one.ID != "" {
		return one.ID, nil
	}
	if issues, perr := parseBdIssues(trimmed); perr == nil && len(issues) == 1 && issues[0].ID != "" {
		return issues[0].ID, nil
	}
	if id := strings.TrimSpace(string(trimmed)); beadIDRe.MatchString(id) {
		return id, nil
	}
	return "", Die("bd create: no issue id in response")
}

// Comment adds a comment to an issue.
func (b Bd) Comment(dir, id, text, actor string) error {
	_, err := b.run(dir, bdArgs(actor, "comments", "add", id, text)...)
	return err
}

// ─── multi-repo aggregation ──────────────────────────────────────────────────

// BeadsDirs is where ready work is gathered from: every configured `beads:`
// path, or the current directory when the key is absent. Configured paths are
// kept even when they cannot be resolved so ReadyAll can report their queues
// as unknown; dropping them here can silently turn an all-missing list into
// the caller's cwd. A present-but-empty list names no repos.
func (a *App) BeadsDirs() []string {
	var out []string
	for _, d := range YamlList(a.ConfigPath, "beads") {
		out = append(out, ExpandTilde(d))
	}
	if len(out) == 0 && !yamlHasKey(a.ConfigPath, "beads") {
		cwdFallbackNotice(noticeWriter, a.ConfigPath)
		out = append(out, "")
	}
	return out
}

var cwdFallbackNotices sync.Map

// noticeWriter is where BeadsDirs says it fell back. A var only so a test can
// assert the silence half of this fix — that a configured `beads:` key emits
// nothing — which is not observable if the notice goes straight to os.Stderr.
var noticeWriter io.Writer = os.Stderr

// cwdFallbackNotice is the other half of rangerhq-wmrb. Whether the cwd
// fallback should exist was settled elsewhere and kept (ranger-base-5b5), so
// the defect that survives is not the fallback — it is that it says nothing.
// A scan that FAILS in the cwd now names it (DirLabel), but a scan that
// SUCCEEDS was the silent case the bead is titled for: `posse dispatch` typed
// in the wrong directory, or run under a second instance whose config never
// set `beads:`, dispatches whatever repo the process happened to start in and
// looks exactly like a correct pass. One line on stderr is the whole fix: the
// queue is still served, and the operator can see which repo served it.
//
// Read-only and said once per config path, for legacyHomeNotice's reason —
// BeadsDirs is called several times per command, and a notice that repeats is
// a notice that gets filtered out.
func cwdFallbackNotice(stderr io.Writer, configPath string) {
	if stderr == nil {
		return
	}
	if _, loaded := cwdFallbackNotices.LoadOrStore(configPath, struct{}{}); loaded {
		return
	}
	fmt.Fprintf(stderr, "posse: no `beads:` in %s — using the process cwd %s as the only beads source\n",
		AbbrevHome(configPath), cwdLabel())
}

// RepoIssue is a BdIssue tagged with the repo it came from.
type RepoIssue struct {
	BdIssue
	Dir string
}

// ScanError is one repo's failed scan, carrying the repo so a caller can
// name it. It exists so an aggregated scan can hand back its failures
// instead of folding them into an empty result (rangerhq-llse): a repo that
// could not be read is a repo whose queue is UNKNOWN, and reporting that as
// "no ready work" is how a --watch loop goes idle without saying why.
type ScanError struct {
	Dir string
	Err error
}

func (e ScanError) Error() string { return DirLabel(e.Dir) + ": " + e.Err.Error() }
func (e ScanError) Unwrap() error { return e.Err }

// DirLabel names a beads source for the operator. The "" BeadsDirs returns
// for an absent `beads:` key is not "no directory": it is the process cwd,
// which is what bd inherits when cmd.Dir is unset. Rendering it verbatim is
// how a failed scan of that source came out as `ready scan failed: :`, an
// error naming no repo at all — the silent-cwd hazard (rangerhq-wmrb) wearing
// an error message. Whether the cwd fallback is the right default is settled
// elsewhere and deliberately (ranger-base-5b5 kept it, and reports configured
// paths that do not resolve rather than dying); what is not defensible is
// running there without saying so. The suffix is the point: it tells the
// operator this source came from the fallback, not from config.yaml.
func DirLabel(dir string) string {
	if dir != "" {
		return AbbrevHome(dir)
	}
	return cwdLabel() + " (process cwd)"
}

// cwdLabel names the directory bd inherits when cmd.Dir is unset.
func cwdLabel() string {
	wd, err := os.Getwd()
	if err != nil {
		return "the process cwd"
	}
	return AbbrevHome(wd)
}

// UnresolvedDirs is the configured paths that are not there at all, as
// ScanErrors. BeadsDirs keeps an unresolvable path on purpose (ranger-base-5b5)
// so callers can name it, and a caller that reaches bd learns of it from bd's
// own chdir failure. The git census cannot: it is deliberately quiet where a
// repo has no census (beadloss.go, TestLostBeadsQuietWithoutACensus), which
// makes "no repo here" read exactly like "nothing was ever dropped here". A
// caller that walks BeadsDirs without bd asks this instead. "" is the cwd
// fallback for the unset key and always resolves.
func UnresolvedDirs(dirs []string) []error {
	var failed []error
	for _, d := range dirs {
		if d == "" {
			continue
		}
		st, err := os.Stat(d)
		switch {
		case err != nil:
			failed = append(failed, ScanError{Dir: d, Err: Die("no such directory")})
		case !st.IsDir():
			failed = append(failed, ScanError{Dir: d, Err: Die("not a directory")})
		}
	}
	return failed
}

// ReadyAll aggregates ready work across the configured beads repos. Repos
// whose bd call fails (most commonly: no database) are skipped rather than
// fatal — the cockpit shouldn't die because one repo isn't bd-initialized
// yet — but they come back as ScanErrors, and every caller must say so.
// Skipping is not the same as finding nothing there.
//
// The result is ONE queue, ordered by OrderBeads across every source, not a
// concatenation of per-repo lists (ranger-base-xotg). The concatenation is
// exactly where priority died: each repo's beads kept whatever order bd's
// query gave them and the second repo's P1 landed behind the first repo's
// P3s, so raising a bead's priority moved it backward. Sorting here rather
// than at each call site means a queue can only be read in queue order —
// `posse ready`, the cockpit's READY WORK, and the dispatch pass alike.
func (b Bd) ReadyAll(a *App, assignee string) ([]RepoIssue, []error) {
	var out []RepoIssue
	var failed []error
	for _, dir := range a.BeadsDirs() {
		issues, err := b.Ready(dir, assignee)
		if err != nil {
			failed = append(failed, ScanError{Dir: dir, Err: err})
			continue
		}
		for _, is := range issues {
			out = append(out, RepoIssue{BdIssue: is, Dir: dir})
		}
	}
	OrderBeads(out, false)
	return out, failed
}

// InProgressAll aggregates claimed work across the configured beads repos —
// the mirror of ReadyAll, skipping repos whose bd call fails so a repo that
// isn't bd-initialized cannot take the cockpit down. It stays silent about
// them: this list is a display, and the queue dispatch acts on is ReadyAll's.
func (b Bd) InProgressAll(a *App) []RepoIssue {
	var out []RepoIssue
	for _, dir := range a.BeadsDirs() {
		issues, err := b.InProgress(dir)
		if err != nil {
			continue
		}
		for _, is := range issues {
			out = append(out, RepoIssue{BdIssue: is, Dir: dir})
		}
	}
	return out
}

func parseBdIssues(out []byte) ([]BdIssue, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	var issues []BdIssue
	if err := json.Unmarshal(trimmed, &issues); err != nil {
		return nil, Die("bd: bad JSON output: %v", err)
	}
	return issues, nil
}
