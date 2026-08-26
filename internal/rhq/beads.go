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
	"os"
	"os/exec"
	"strings"
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
			msg = err.Error()
		}
		return nil, Die("bd %s: %s", strings.Join(args, " "), msg)
	}
	return out.Bytes(), nil
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
		out = append(out, "")
	}
	return out
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

func (e ScanError) Error() string { return AbbrevHome(e.Dir) + ": " + e.Err.Error() }
func (e ScanError) Unwrap() error { return e.Err }

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
