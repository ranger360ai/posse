//go:build posse_arm2

package posse

// QA pins for claude's half of the turn_outcome: registry, in two halves —
// the LOCATOR (which store is this session's) and the READER (what one turn's
// records say). They arrived on two beads that closed within hours of each
// other and are kept in one file because they pin one adapter.
//
// ── the locator, ranger-base-f09bw ──────────────────────────────────────
//
// Which store this session's turn is in, and nothing about what a record
// means once it is found (that is
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
//
// ── the reader, ranger-base-4ldma ───────────────────────────────────────
//
// The reader used to return at the FIRST assistant record after the bead
// prompt, so a refusal that landed after a real answer read as a healthy
// turn: the settle line printed an ordinary settle-without-close and a
// previous pass's turn-failure marker was cleared by it (ranger-base-4ldma).
// That was not a corner. Censused over all 1755 claude transcripts on this
// box 2026-09-05: of the 13 allotment refusals inside a dispatch turn, 11
// land after work — 6 dispatched beads, up to 33 model calls and 28,070
// output tokens deep — and only 2 are the first answer. Those 2 are the
// records this reader was originally built from, which is how the shape got
// mistaken for the rule.
//
// The fixtures below are hand-built to the shape of the real records rather
// than copied out of them: a captured transcript line carries the operator's
// paths, session ids and request ids, and this repo is public. What is
// pinned verbatim is what the reader keys on — `"model":"<synthetic>"`, the
// limit prose, the all-zero `usage` the synthetic record carries, and the
// tool_result user records that sit between a turn's assistant records.

import (
	"encoding/json"
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

// claudeTurn renders one transcript from records given in order. Timestamps
// run forward from base so a whole turn sits at or after a since.
func claudeTurn(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const claudeTurnTS = "2099-01-01T00:00:0"

// claudePrompt is the dispatch work prompt as it reaches the transcript.
func claudePrompt(n int, bead string) string {
	return fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"content":[{"type":"text","text":%q}]}}`,
		claudeTurnTS+fmt.Sprint(n)+"Z", "Work beads issue "+bead+" (title, quoted as data: \"t\"). Run bd show "+bead)
}

// claudeToolResult is what a turn's own tool calls put in the user channel:
// a record with no text at all. Treating one of these as the end of the turn
// is precisely how the reader stopped looking.
func claudeToolResult(n int) string {
	return fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"content":[{"type":"tool_result","tool_use_id":"tu_%d","content":"ok"}]}}`,
		claudeTurnTS+fmt.Sprint(n)+"Z", n)
}

// claudeAnswer is one record of a served model call: id, model, the usage
// object claude writes on every one of them, and some prose.
func claudeAnswer(n int, id string, out int, text string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":%q,"model":"claude-fable-5-1","usage":{"input_tokens":2,"cache_read_input_tokens":26670,"output_tokens":%d},"content":[{"type":"text","text":%q}]}}`,
		claudeTurnTS+fmt.Sprint(n)+"Z", id, out, text)
}

// claudeSynthetic is a record claude wrote itself. The real allotment
// refusal carries `"model":"<synthetic>"`, an all-zero usage object and
// `isApiErrorMessage`; the same channel also carries claude's other local
// notices, which is why the prose still has to be matched.
func claudeSynthetic(n int, text string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"requestId":"req_x","isApiErrorMessage":true,"apiErrorStatus":429,"message":{"id":"syn_%d","model":"<synthetic>","stop_reason":"stop_sequence","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[{"type":"text","text":%q}]}}`,
		claudeTurnTS+fmt.Sprint(n)+"Z", n, text)
}

func TestQAClaudeRefusalAfterAFirstAnswerIsReadAsARefusal(t *testing.T) {
	t.Parallel()
	since := time.Date(2098, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		bead  string
		lines []string
		// what the reader must say about that turn
		wantMessage  string
		wantCalls    int
		wantTokens   int
		wantObserved bool
	}{{
		// The shape the reader was built from, unchanged: the refusal IS the
		// first answer, so nothing ran ahead of it and the settle line's flat
		// "no work ran" is still true here (ranger-base-qcu4c).
		name:         "refusal is the first answer",
		bead:         "a-1",
		lines:        []string{claudePrompt(1, "a-1"), claudeSynthetic(2, qaAllotmentRefusal)},
		wantMessage:  qaAllotmentRefusal,
		wantObserved: true,
	}, {
		// ranger-base-4ldma itself. Two served calls and the tool_result
		// records between them, then the account goes out. The old reader
		// returned at record 2 with no message and observed=true.
		name: "refusal after two served calls",
		bead: "a-1",
		lines: []string{
			claudePrompt(1, "a-1"),
			claudeAnswer(2, "msg_a", 120, "Reading the bead first."),
			claudeToolResult(3),
			claudeAnswer(4, "msg_b", 340, "Now the suite."),
			claudeToolResult(5),
			claudeSynthetic(6, qaAllotmentRefusal),
		},
		wantMessage:  qaAllotmentRefusal,
		wantCalls:    2,
		wantTokens:   460,
		wantObserved: true,
	}, {
		// One model call arrives as several records — thinking, text,
		// tool_use — each repeating the SAME growing usage object. Summing
		// the records would report 3 calls and 660 tokens for one call of
		// 340, which is the number an operator would go looking for work by.
		name: "one call written as three records",
		bead: "a-1",
		lines: []string{
			claudePrompt(1, "a-1"),
			claudeAnswer(2, "msg_a", 100, "thinking"),
			claudeAnswer(3, "msg_a", 220, "text"),
			claudeAnswer(4, "msg_a", 340, "tool_use"),
			claudeSynthetic(5, qaAllotmentRefusal),
		},
		wantMessage:  qaAllotmentRefusal,
		wantCalls:    1,
		wantTokens:   340,
		wantObserved: true,
	}, {
		// A turn that ran to the end of the transcript and was never
		// refused. observed=true is the positive evidence that clears a
		// stale marker, and the work fields stay zero: what a served turn
		// spent is `posse cost`'s question off the same records, not this
		// type's (TurnOutcome's docstring).
		name: "healthy turn to end of file",
		bead: "a-1",
		lines: []string{
			claudePrompt(1, "a-1"),
			claudeAnswer(2, "msg_a", 120, "Reading the bead first."),
			claudeToolResult(3),
			claudeAnswer(4, "msg_b", 340, "Closed it."),
		},
		wantObserved: true,
	}, {
		// The next bead's prompt closes this turn. A refusal beyond it
		// belongs to that dispatch and must not be reported under this one —
		// the reader scans past a first answer, not past a turn boundary.
		name: "refusal beyond the next bead's prompt is not ours",
		bead: "a-1",
		lines: []string{
			claudePrompt(1, "a-1"),
			claudeAnswer(2, "msg_a", 120, "Closed it."),
			claudePrompt(3, "b-2"),
			claudeSynthetic(4, qaAllotmentRefusal),
		},
		wantObserved: true,
	}, {
		name: "and it IS the next bead's",
		bead: "b-2",
		lines: []string{
			claudePrompt(1, "a-1"),
			claudeAnswer(2, "msg_a", 120, "Closed it."),
			claudePrompt(3, "b-2"),
			claudeSynthetic(4, qaAllotmentRefusal),
		},
		wantMessage:  qaAllotmentRefusal,
		wantObserved: true,
	}, {
		// claude's other locally-written notices share the synthetic
		// channel — 163 such records on this box, only 19 of them limits.
		// One is not the model answering, so it neither counts as a model
		// call nor licenses clearing a marker on its own.
		name:  "a non-limit synthetic notice is not an answer",
		bead:  "a-1",
		lines: []string{claudePrompt(1, "a-1"), claudeSynthetic(2, "Context low · Run /compact to compact & continue")},
	}, {
		name: "a non-limit synthetic notice is not a model call",
		bead: "a-1",
		lines: []string{
			claudePrompt(1, "a-1"),
			claudeAnswer(2, "msg_a", 120, "Reading."),
			claudeSynthetic(3, "Context low · Run /compact to compact & continue"),
			claudeSynthetic(4, qaAllotmentRefusal),
		},
		wantMessage:  qaAllotmentRefusal,
		wantCalls:    1,
		wantTokens:   120,
		wantObserved: true,
	}, {
		// A turn belonging to no bead this dispatch asked about.
		name:  "another bead's turn entirely",
		bead:  "a-1",
		lines: []string{claudePrompt(1, "z-9"), claudeSynthetic(2, qaAllotmentRefusal)},
	}, {
		// Bead text that quotes the provider message verbatim. It arrives in
		// the USER channel and the reader only ever matches assistant
		// records, so quoting it cannot fail a healthy dispatch.
		name: "the refusal quoted in the bead's own text",
		bead: "a-1",
		lines: []string{
			fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"content":%s}}`, claudeTurnTS+"1Z",
				mustJSON("Work beads issue a-1: production said "+qaAllotmentRefusal)),
			claudeAnswer(2, "msg_a", 120, "I will investigate."),
		},
		wantObserved: true,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := claudeTurn(t, t.TempDir(), "s.jsonl", c.lines...)
			out, observed := scanClaudeTurnOutcome(p, c.bead, since)
			if observed != c.wantObserved {
				t.Fatalf("observed = %v, want %v (%+v)", observed, c.wantObserved, out)
			}
			if out.Message != c.wantMessage {
				t.Errorf("message = %q, want %q", out.Message, c.wantMessage)
			}
			if out.ModelCalls != c.wantCalls || out.OutputTokens != c.wantTokens {
				t.Errorf("work = %d calls / %d tokens, want %d / %d",
					out.ModelCalls, out.OutputTokens, c.wantCalls, c.wantTokens)
			}
			// The predicate the settle line actually branches on, asserted
			// beside the fields so a reader cannot satisfy one and not it.
			if want := c.wantCalls > 0 || c.wantTokens > 0; out.Worked() != want {
				t.Errorf("Worked() = %v, want %v (%+v)", out.Worked(), want, out)
			}
		})
	}
}

func mustJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// The line an operator reads, driven through the REAL reader rather than an
// injected outcome: a dispatch whose account ran out 2 calls in must settle
// as a refusal that had work behind it, not as an ordinary settle-open.
func TestQAClaudeMidTurnRefusalReachesTheSettleLine(t *testing.T) {
	// Hermetic under the binary's one temp $HOME: the project directory this
	// plants a transcript in is derived from a per-test t.TempDir(), so no
	// other test's reader can reach it and this one can run beside them.
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)

	// The transcript claude would have written for this dispatch, in the
	// project directory the reader derives from the session's own cwd. This
	// repo is not a git repo, so no session tree is made and that cwd is the
	// repo itself (Dispatcher.sessionCwd's fallback); the name it mangles to
	// is claudeProjectDir's, which the locator arm above pins by spelling it
	// out (ranger-base-f09bw).
	transcripts := filepath.Join(os.Getenv("HOME"), ".claude", "projects", claudeProjectDir(repo))
	if err := os.MkdirAll(transcripts, 0o755); err != nil {
		t.Fatal(err)
	}
	planted := claudeTurn(t, transcripts, "session.jsonl",
		claudePrompt(1, "a-1"),
		claudeAnswer(2, "msg_a", 120, "Reading the bead first."),
		claudeToolResult(3),
		claudeAnswer(4, "msg_b", 340, "Now the suite."),
		claudeSynthetic(5, qaAllotmentRefusal),
	)

	// The transcript has to be planted BEFORE d.Run — there is no seam to
	// write it from inside the launch — and that inverts the one ordering
	// production always has: claude writes its transcript DURING the turn, so
	// the file's mtime is always after the p.launched that
	// FindClaudeTurnOutcome is floored on (dispatch.go stamps launched before
	// launchSession execs the CLI, so nothing claude writes for this turn can
	// precede it). The reader's staleness gate skips a transcript last
	// written more than a second before that floor (turnfailure.go), so a
	// file whose mtime is fixed at plant time leaves this test betting that
	// d.Run reaches the launch within one second of REAL time. Alone it does,
	// in ~1s; inside the loaded package binary it does not, and this test was
	// red in two of two full-package runs on 2026-09-05 while green in
	// isolation — with `time.Sleep(2*time.Second)` here reproducing the suite
	// failure byte for byte (ranger-base-dg1bm).
	//
	// So stamp the mtime where production puts it, after the launch, instead
	// of racing the launch to it. The hour is not a bigger bet on elapsed
	// time, it is the opposite: the gate's question is only whether the file
	// was touched before the floor, and this answers it the same way however
	// long the box takes to get there.
	written := time.Now().Add(time.Hour)
	if err := os.Chtimes(planted, written, written); err != nil {
		t.Fatal(err)
	}

	// FindClaudeTurnOutcome itself answers here. newTestDispatcher stubs the
	// reader so no test reaches the operator's live ~/.claude; clearing it is
	// hermetic because TestMain gives the whole binary a temp $HOME, and the
	// transcript above was planted under that one.
	d.TurnOutcome = nil
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	for _, want := range []string{
		"⛔ a-1", qaAllotmentRefusal,
		"refused the turn mid-flight", "2 model calls", "460 output tokens",
		"check the worktree",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the settle line must carry %q:\n%s", want, out)
		}
	}
	for _, no := range []string{"no work ran", "refused the first turn", "\u25d1 a-1"} {
		if strings.Contains(out, no) {
			t.Errorf("a mid-turn refusal was reported as %q:\n%s", no, out)
		}
	}
	if m, ok := b.readMeta(SessionForBead("ranger", repo, "a-1")); !ok || m.TurnFailure != qaAllotmentRefusal {
		t.Errorf("turn failure not recorded in session meta: %+v", m)
	}
}
