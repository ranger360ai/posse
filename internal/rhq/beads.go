package rhq

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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Bd struct {
	Bin string // bd binary; RHQ_BD_BIN overrides (testing)
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

func (b Bd) run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(b.Bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = bdStdoutError(out.Bytes())
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, Die("bd %s: %s", strings.Join(args, " "), msg)
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
func (b Bd) Ready(dir, assignee string) ([]BdIssue, error) {
	args := []string{"ready", "--json", "--limit", "0"}
	if assignee != "" {
		args = append(args, "--assignee", assignee)
	}
	out, err := b.run(dir, args...)
	if err != nil {
		return nil, err
	}
	return parseBdIssues(out)
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
// --limit 0 for the same reason Ready carries it (rangerhq-47v): a capped
// page of a set the caller is counting is a silent undercount.
func (b Bd) OpenLabeledAny(dir string, labels ...string) ([]BdIssue, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	out, err := b.run(dir, "list", "--label-any", strings.Join(labels, ","), "--json", "--limit", "0")
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
	Actor       string   // bd audit actor
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
