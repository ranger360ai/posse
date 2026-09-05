package posse

// QA pins for claude's half of the turn_outcome: registry — the LOCATOR, and
// nothing about what a record means once it is found (that is
// TestScanClaudeTurnOutcomeReadsOnlyTheSyntheticAssistantOutcome's, cost_test.go).
//
// The bead these were filed on is ranger-base-f09bw, and it is one defect in
// two halves: the reader was handed the repo the bead lives in, and it then
// derived a project directory name by replacing slashes only. Both halves had
// to be wrong for the blindness to be total, and both were:
//
//	MEASURED on this box 2026-09-05 — 1354 project directories under
//	~/.claude/projects, 1301 of them ~/.posse/worktrees paths, and all 1301
//	carrying at least one "Work beads issue" transcript. Not one of them was
//	reachable from ~/src/posse, and none of the 1349 project directories that
//	carry a transcript is the slash-only mangling of the `cwd` its own first
//	record names unless that cwd has no "." in it (43 of 1349).
//
// The defect was LOUD — turnOutcomeClause prints "looked for a turn outcome
// and found none this pass" for (no outcome, false) — but loud about the one
// runtime posse was supposed to be able to read.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const qaClaudeRefusal = "You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model."

// qaClaudeProject plants one session transcript in the project directory
// claude would key `cwd` on — spelled by the test with the literal name, not
// by calling claudeProjectDir, so a mangling that agrees with itself does not
// pass this by construction.
func qaClaudeProject(t *testing.T, home, project, bead, at, text, model string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTranscript(t, dir, "session.jsonl",
		`{"type":"user","timestamp":"`+at+`","message":{"content":"Work beads issue `+bead+`: do the work"}}`,
		`{"type":"assistant","timestamp":"`+at+`","message":{"model":"`+model+`","content":[{"type":"text","text":`+fmt.Sprintf("%q", text)+`}]}}`,
	)
}

// The half the fix turns on: a dispatched session's cwd is its WORKTREE, and
// every worktree on this box lives under a dot directory. The mangling that
// replaced slashes only spelled `-Users-example-.posse-...` for a store claude
// wrote to `-Users-example--posse-...`, so the reader found nothing there
// either way round.
func TestQAClaudeTurnOutcomeReadsAWorktreeSessionsProjectDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	since := time.Now().Add(-time.Minute)
	at := since.Add(time.Second).UTC().Format(time.RFC3339Nano)

	repo := "/Users/example/src/posse"
	tree := "/Users/example/.posse/worktrees/posse/ranger-posse-a-1"
	qaClaudeProject(t, home, "-Users-example--posse-worktrees-posse-ranger-posse-a-1",
		"a-1", at, qaClaudeRefusal, "<synthetic>")

	out, observed := FindClaudeTurnOutcome(tree, "a-1", since)
	if out.Message != qaClaudeRefusal || !observed {
		t.Errorf("a refusal recorded under the session's own tree was not read: %q %v", out.Message, observed)
	}

	// The control, and the reason this is a locator pin and not a scan pin:
	// the repo the bead lives in has no store of its own, so asking about it
	// is the (no outcome, false) rung the whole fleet was getting. Without
	// this the arm above could pass on a reader that had simply stopped
	// looking at the directory at all.
	if out, observed := FindClaudeTurnOutcome(repo, "a-1", since); out.Message != "" || observed {
		t.Errorf("the repo has no transcripts of its own: %q %v, want %q false", out.Message, observed, "")
	}
}

// claudeProjectDir against pairs claude itself wrote. Every row is a real
// (cwd, directory name) pair off a box's ~/.claude/projects, read out of the
// transcripts' own `cwd` field — the fixture comes from the store under
// test's PRODUCER, so a rule invented here cannot satisfy it.
//
// With the operator's home and the persona renamed on BOTH sides, which ADR
// 0012 App.A 5 requires of anything the tree ships. The substitution is
// alphanumeric-for-alphanumeric, so what each row actually encodes — which
// characters survive and which fold to "-" — is untouched by it.
func TestQAClaudeProjectDirIsTheEncodingClaudeWrote(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ cwd, want string }{
		// The shape 43 of 1349 have: no "." anywhere, which is why replacing
		// slashes alone looked correct for as long as it did.
		{"/Users/example/src/posse", "-Users-example-src-posse"},
		// The shape the other 1306 have, and every dispatched session's: the
		// worktree root is a dot directory, so one "/." becomes "--".
		{"/Users/example/.posse/worktrees/posse/ranger-posse-a-1",
			"-Users-example--posse-worktrees-posse-ranger-posse-a-1"},
		// The one cwd on that box with a space in it. It is the arm that says
		// the rule is "not alphanumeric", not "slash or dot".
		{"/Users/example/Documents/Jeep Troubleshooting", "-Users-example-Documents-Jeep-Troubleshooting"},
		// A scratchpad session claude rooted under a tree: a dot MID-component
		// ("rulebook.pk0fZh"), not just a leading dot directory, and the shape
		// the exactness pin below is built on.
		{"/private/tmp/claude-501/-Users-example--posse-worktrees-posse-ranger-posse-a-1/1987c10d-2998-4c50-8f93-655264255b4d/scratchpad/rulebook.pk0fZh",
			"-private-tmp-claude-501--Users-example--posse-worktrees-posse-ranger-posse-a-1-1987c10d-2998-4c50-8f93-655264255b4d-scratchpad-rulebook-pk0fZh"},
	} {
		if got := claudeProjectDir(c.cwd); got != c.want {
			t.Errorf("claudeProjectDir(%q) = %q, want %q", c.cwd, got, c.want)
		}
	}
}

// The project directory is named EXACTLY, never matched as a substring. A
// session rooted under the tree — the scratchpad claude hands every session —
// mangles to a name that CONTAINS the tree's own: MEASURED, 6 worktree
// project names on this box are a substring of 11 such directories.
//
// The arm is the rung that costs something. This session's own transcript is
// not readable yet, which is the (no outcome, false) the reader exists to keep
// distinct from a healthy turn; a substring match reaches the stranger
// instead, and the stranger holds a synthetic refusal — so the pass would
// stop the bead with "claude refused the first turn" about a turn nothing
// read about this session.
func TestQAClaudeTurnOutcomeNamesOneProjectDirNotEveryNameContainingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	since := time.Now().Add(-time.Minute)
	at := since.Add(time.Second).UTC().Format(time.RFC3339Nano)

	tree := "/Users/example/.posse/worktrees/posse/ranger-posse-a-1"
	own := "-Users-example--posse-worktrees-posse-ranger-posse-a-1"
	stranger := "-private-tmp-claude-501-" + own + "-0d1e-scratchpad-probe"
	if !strings.Contains(stranger, own) {
		t.Fatalf("the fixture must contain the session's own project name: %q vs %q", stranger, own)
	}
	qaClaudeProject(t, home, stranger, "a-1", at, qaClaudeRefusal, "<synthetic>")

	if out, observed := FindClaudeTurnOutcome(tree, "a-1", since); out.Message != "" || observed {
		t.Errorf("another session's transcript answered for this one: %q %v, want %q false", out.Message, observed, "")
	}
	// The control: the stranger's store IS readable, asked by its own cwd, so
	// the arm above is exactness talking and not an unreadable fixture.
	if out, _ := FindClaudeTurnOutcome("/private/tmp/claude-501/"+own+"/0d1e/scratchpad/probe", "a-1", since); out.Message != qaClaudeRefusal {
		t.Errorf("the stranger fixture is not readable at all: %q", out.Message)
	}
}

// The other half of the same defect, and the one that made it total: WHICH
// directory the pass asks about. The reader is handed the session's own cwd —
// its worktree on every dispatch launch — and not p.is.Dir, the repo the bead
// lives in.
//
// Driven through production Dispatcher.Run over a real git repo, so the tree
// is the one EnsureSessionTree really made and the path is the one
// startPlanned really handed the pane. The injected reader is the CONSUMER
// here: what is under test is the argument, not the reading.
func TestQAWorktreeDispatchAsksAboutTheSessionsTreeNotTheRepo(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)

	var asked []string
	d.TurnOutcome = func(cwd, bead string, since time.Time) (TurnOutcome, bool) {
		asked = append(asked, cwd)
		return TurnOutcome{}, true
	}
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	session := SessionForBead("ranger", repo, "a-1")
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("no session meta:\n%s", out)
	}
	// The arm's own precondition: this pass really did put the session in a
	// tree of its own. Without it a launch that fell back to the shared
	// checkout would make every assertion below trivially true.
	if m.Dir == repo || m.Dir == "" {
		t.Fatalf("this pass made no session tree (dir %q, repo %q) — the arm proves nothing:\n%s", m.Dir, repo, out)
	}
	if len(asked) != 1 {
		t.Fatalf("a declared reader must be asked exactly once, asked %dx:\n%s", len(asked), out)
	}
	if asked[0] == repo {
		t.Errorf("the reader was handed the repo %q; claude keys its store on the session's cwd %q:\n%s", repo, m.Dir, out)
	}
	if asked[0] != m.Dir {
		t.Errorf("the reader was asked about %q, want the session's own cwd %q:\n%s", asked[0], m.Dir, out)
	}
}
