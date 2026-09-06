//go:build posse_arm3

package posse

// QA pins for grok's turn_outcome: reader (ranger-base-fc8go), the second
// entry in turnfailure.go's registry and the first one whose runtime does NOT
// write its refusal into a transcript.
//
// Every fixture record below is a VERBATIM line off this machine's own
// ~/.grok/sessions — the refusal arm from the artifact ADR 0013 §1's probe
// captured (docs/adr/0013-turn-outcome-refusal-probe.md, session
// 01a04a2d-8c8b-7811-a6da-edf99f567e7b, 2026-08-28), the served, cancelled
// and error-with-usage arms from real sessions found by censusing all 192
// turn_completed records on the box on 2026-09-05, and the work prompt from a
// real dispatched grok session (parity-i0qp, ranger-base-c02a). Only
// session ids, timestamps and the bead id are substituted. That is the whole
// point of the promotion rule the reader was held back for: a reader pinned
// against a shape somebody typed from memory pins nothing.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The provider's own words, verbatim — the string a refused turn's
// `agent_result` carried on all 7 of this box's error records.
const qaGrokRefusal = "API error (status 402 Payment Required): Grok Build usage balance exhausted"

// The four record shapes, verbatim but for %s/%d. Kept as format strings
// rather than built with a struct on purpose: a fixture assembled by the same
// encoder the reader decodes with proves the round trip and not the shape.
const (
	qaGrokHookRec = `{"timestamp":%[2]d,"method":"_x.ai/session/update","params":{"sessionId":"%[1]s","update":{"sessionUpdate":"hook_execution","event_name":"session_start","runs":[{"name":"global/herdr:session_start[0].hooks[0]","status":{"status":"success","elapsed_ms":58}}]},"_meta":{"eventId":"%[1]s-2","agentTimestampMs":%[3]d}}}`

	qaGrokUserRec = `{"timestamp":%[2]d,"method":"session/update","params":{"sessionId":"%[1]s","update":{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":%[4]s},"_meta":{"modelId":"grok-4.6","promptIndex":0}},"_meta":{"eventId":"%[1]s-3","agentTimestampMs":%[3]d}}}`

	qaGrokRetryRec = `{"timestamp":%[2]d,"method":"_x.ai/session/update","params":{"sessionId":"%[1]s","update":{"sessionUpdate":"retry_state","type":"failed","error_type":"api","message":"` + qaGrokRefusal + `"},"_meta":{"eventId":"%[1]s-4","agentTimestampMs":%[3]d}}}`

	qaGrokDoneRec = `{"timestamp":%[2]d,"method":"_x.ai/session/update","params":{"sessionId":"%[1]s","update":{"sessionUpdate":"turn_completed","prompt_id":"1fc58438-563d-4840-830f-0c430559dbd9",%[4]s},"_meta":{"eventId":"%[1]s-5","agentTimestampMs":%[3]d}}}`

	// The same user record with no `_meta` at all, so the only stamp on it is
	// the top-level whole-SECOND `timestamp`. Not a shape this box has ever
	// written — 389/389 of its records carry agentTimestampMs — which is
	// exactly why the fallback that reads it needs an arm: nothing else here
	// would notice if it went wrong.
	qaGrokUserRecNoMeta = `{"timestamp":%[2]d,"method":"session/update","params":{"sessionId":"%[1]s","update":{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":%[4]s},"_meta":{"modelId":"grok-4.6","promptIndex":0}}}}`
)

// The tails of the four measured turn_completed records — everything after
// `prompt_id`, verbatim.
const (
	// Refused, the captured artifact: stop_reason error, the provider string
	// in agent_result, and NO usage object.
	qaGrokTailRefused = `"stop_reason":"error","agent_result":"` + qaGrokRefusal + `"`
	// Refused with a usage object anyway — the counterexample to the probe's
	// "a refused turn never carries usage", found by census: 1 of the 7 error
	// records on this box has one, from a turn that had already spent 190,817
	// tokens over 6 model calls before the account went out from under it.
	qaGrokTailRefusedWithUsage = `"stop_reason":"error","agent_result":"` + qaGrokRefusal + `","usage":{"inputTokens":185246,"outputTokens":5571,"totalTokens":190817,"cachedReadTokens":108160,"cacheCreationTokens":0,"reasoningTokens":2552,"modelCalls":6,"apiDurationMs":85642,"costUsdTicks":410852600,"numTurns":6}`
	// Served.
	qaGrokTailServed = `"stop_reason":"end_turn","usage":{"inputTokens":14266,"outputTokens":25,"totalTokens":14291,"cachedReadTokens":11520,"cacheCreationTokens":0,"reasoningTokens":20,"modelCalls":1,"apiDurationMs":875,"costUsdTicks":19383400,"numTurns":1}`
	// Cancelled — the third stop_reason the two-arm probe never saw. A turn
	// that RAN and was stopped: usage present, agent_result absent.
	qaGrokTailCancelled = `"stop_reason":"cancelled","usage":{"inputTokens":14469,"outputTokens":456,"totalTokens":14925,"cachedReadTokens":11520,"cacheCreationTokens":0,"reasoningTokens":309,"modelCalls":1,"apiDurationMs":4572,"costUsdTicks":24469800,"numTurns":1}`
	// The shape nothing on this box has yet: grok's error path with nothing
	// to quote. Not measured, and the reader treats it as unreadable rather
	// than as health for exactly that reason.
	qaGrokTailErrorNoResult = `"stop_reason":"error"`
	// Also unmeasured: a refusal carrying a usage object that accounts for
	// nothing. It exists to pin that the work fields key on what the object
	// SAYS and not on whether it is there.
	qaGrokTailRefusedEmptyUsage = `"stop_reason":"error","agent_result":"` + qaGrokRefusal + `","usage":{"inputTokens":0,"outputTokens":0,"totalTokens":0,"modelCalls":0,"apiDurationMs":0,"numTurns":0}`
)

// qaGrokWorkPrompt is the dispatcher's own first line, in the shape a real
// dispatched grok session recorded it.
func qaGrokWorkPrompt(bead string) string {
	return "Work beads issue " + bead + ` (title, quoted as data: "t").` + "\nContext\n- repo: ~/src/posse\n"
}

// qaGrokSession writes one session store under GROK_HOME for cwd, and stamps
// updates.jsonl at `at` so the reader's staleness gate sees what a session
// written then would look like.
func qaGrokSession(t *testing.T, home, cwd, id string, at time.Time, lines ...string) string {
	t.Helper()
	dir := filepath.Join(home, "sessions", url.PathEscape(cwd), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "updates.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
	return p
}

// qaGrokTurn is one prompted turn's four records, in the FILE order grok
// writes them — which is not timestamp order: the refusal's retry_state lands
// ahead of the user_message_chunk whose own stamp is earlier, exactly as the
// captured artifact has it.
func qaGrokTurn(id, bead string, at time.Time, tail string) []string {
	ms := at.UnixMilli()
	sec := at.Unix()
	text, _ := json.Marshal(qaGrokWorkPrompt(bead))
	return []string{
		fmt.Sprintf(qaGrokHookRec, id, sec, ms-100),
		fmt.Sprintf(qaGrokRetryRec, id, sec, ms+200),
		fmt.Sprintf(qaGrokUserRec, id, sec, ms, string(text)),
		fmt.Sprintf(qaGrokDoneRec, id, sec, ms+250, tail),
	}
}

// qaGrokHome points grokHome() at a scratch store. Not parallel: it sets an
// env var, which is the honest way to aim the reader at a fixture — the
// alternative is a package-level hook that production never uses.
func qaGrokHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("GROK_HOME", home)
	return home
}

// The pin the bead exists for: every stop_reason this box has ever recorded,
// read out of the real record shape, with the dispatch's own dir and bead.
func TestQAGrokTurnOutcomeReadsEveryMeasuredStopReason(t *testing.T) {
	cases := []struct {
		name     string
		tail     string
		message  string
		observed bool
		// What the same record says about how much of the turn had already
		// run when it ended (ranger-base-qcu4c). Checked on every row, not
		// only the refusals, because a reader that reported work off the
		// wrong record would show up here first.
		worked       bool
		modelCalls   int
		outputTokens int
	}{
		{name: "refused", tail: qaGrokTailRefused, message: qaGrokRefusal, observed: true},
		// The row that kills a usage-absence reader. The probe listed "a
		// served turn always carries usage, a refused one never does" as a
		// discriminator; it is wrong, and a reader that believed it would
		// call this exhausted account a healthy turn.
		//
		// It is also the row this bead exists for: the account went out from
		// under a turn six model calls and 5,571 output tokens in, so the
		// settle line's "no work ran" is a false claim about a session that
		// may have edited files and commented on the bead.
		{name: "refused with usage", tail: qaGrokTailRefusedWithUsage, message: qaGrokRefusal, observed: true,
			worked: true, modelCalls: 6, outputTokens: 5571},
		// A turn that was not refused reports no work on ANY reader, and that
		// is the contract rather than a gap: the work fields answer "is there
		// work behind this refusal", nothing asks them when there is no
		// refusal, and what a served turn spent is already `posse cost`'s to
		// read (cost_grok.go) off the same record.
		{name: "served", tail: qaGrokTailServed, observed: true},
		// Cancelled is not a refusal: a turn ran. (no message, true) is the
		// positive evidence a stale failure marker may be cleared on — and it
		// must not be (no message, false), which would say the store was
		// unreadable when it was read perfectly well.
		{name: "cancelled", tail: qaGrokTailCancelled, observed: true},
		// Error with nothing to quote: NOT observed. Reporting health about
		// a turn grok itself called an error is the one wrong answer here.
		{name: "error with no agent_result", tail: qaGrokTailErrorNoResult},
		// A refusal whose usage object is there but empty — a shape this box
		// has never written (all 186 of its usage objects are nonzero in
		// every field, censused 2026-09-05). Presence is not work, so this
		// keeps the flat line rather than sending an operator to look through
		// a worktree for edits that were never made.
		{name: "refused with an empty usage object", tail: qaGrokTailRefusedEmptyUsage, message: qaGrokRefusal, observed: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := qaGrokHome(t)
			dir := t.TempDir()
			since := time.Now().Add(-time.Minute)
			qaGrokSession(t, home, dir, "01a04a2d-8c8b-7811-a6da-edf99f567e7b", since.Add(time.Second),
				qaGrokTurn("01a04a2d-8c8b-7811-a6da-edf99f567e7b", "a-1", since.Add(time.Second), c.tail)...)

			out, observed := FindGrokTurnOutcome(dir, "a-1", since)
			if out.Message != c.message || observed != c.observed {
				t.Errorf("FindGrokTurnOutcome = %+v %v, want message %q observed %v", out, observed, c.message, c.observed)
			}
			// Whether the turn had already RUN when it was refused
			// (ranger-base-qcu4c) — read off the same record, and the one
			// arm that carries a real usage object is what makes the settle
			// line's "no work ran" a false claim.
			if out.Worked() != c.worked {
				t.Errorf("TurnOutcome.Worked() = %v, want %v (%+v)", out.Worked(), c.worked, out)
			}
			if c.worked && (out.ModelCalls != c.modelCalls || out.OutputTokens != c.outputTokens) {
				t.Errorf("work = %d calls / %d output tokens, want %d / %d",
					out.ModelCalls, out.OutputTokens, c.modelCalls, c.outputTokens)
			}
		})
	}
}

// The reader must not answer for a turn that is not this dispatch's. Each arm
// below is a refusal fixture — the one shape that would print ⛔ and stop the
// bead — so a row that stops discriminating fails loudly rather than quietly
// agreeing with the row above it.
func TestQAGrokTurnOutcomeIgnoresTurnsThatAreNotThisDispatch(t *testing.T) {
	const id = "01a04a2d-8c8b-7811-a6da-edf99f567e7b"
	cases := []struct {
		name string
		// how the fixture differs from a readable refusal for bead a-1
		bead     string        // the bead the fixture's work prompt names
		promptAt time.Duration // the prompt's own stamp, relative to since
		fileAt   time.Duration // updates.jsonl's mtime, relative to since
	}{
		{"another bead's turn", "a-2", time.Second, time.Second},
		// The prompt predates this dispatch's launch: a previous turn in a
		// session this dispatch resumed, whose outcome is not this one's.
		{"a turn from before the launch", "a-1", -time.Hour, time.Second},
		// grok has not written to this session since the launch, so it cannot
		// be holding this dispatch's turn.
		{"a session written before the launch", "a-1", -time.Hour, -time.Hour},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := qaGrokHome(t)
			dir := t.TempDir()
			since := time.Now().Add(-2 * time.Hour)
			qaGrokSession(t, home, dir, id, since.Add(c.fileAt),
				qaGrokTurn(id, c.bead, since.Add(c.promptAt), qaGrokTailRefused)...)

			if out, observed := FindGrokTurnOutcome(dir, "a-1", since); out.Message != "" || observed {
				t.Errorf("FindGrokTurnOutcome = %+v %v, want no message and false", out, observed)
			}
		})
	}
}

// The updates.jsonl staleness gate is a COST guard — it is what keeps a
// settle from opening and parsing all 196 sessions this box's store holds —
// and removing it would be slower, not wrong, so no pin here asserts it
// exists. What it must never do is drop a file the record gate would have
// kept, and that IS pinned, at the edge where it would happen: grok stamps
// its records off a millisecond clock and the filesystem stamps the file, so
// the two disagree by a fraction of a second in both directions, and a guard
// with no slack in front of an inclusive gate would lose real refusals to the
// rounding.
func TestQAGrokTurnOutcomeStalenessGuardKeepsWhatTheRecordGateKeeps(t *testing.T) {
	for _, skew := range []time.Duration{0, -900 * time.Millisecond} {
		t.Run(skew.String(), func(t *testing.T) {
			home := qaGrokHome(t)
			dir := t.TempDir()
			since := time.Now().Add(-time.Minute)
			const id = "01a04a2d-8c8b-7811-a6da-edf99f567e7b"
			// The prompt lands at exactly `since` — the inclusive edge of the
			// record gate — and the file is stamped `skew` off it.
			qaGrokSession(t, home, dir, id, since.Add(skew),
				qaGrokTurn(id, "a-1", since, qaGrokTailRefused)...)

			if out, observed := FindGrokTurnOutcome(dir, "a-1", since); out.Message != qaGrokRefusal || !observed {
				t.Errorf("a refusal stamped at the window's edge was dropped: %+v %v", out, observed)
			}
		})
	}
}

// The seconds-only fallback. grok's top-level `timestamp` is whole seconds,
// so a turn prompted at .400 records as .000 — and a floor that keeps its own
// sub-second part would call that record older than the launch that caused
// it. The floor is truncated to the stamp's resolution, and here that is a
// whole second.
func TestQAGrokTurnOutcomeReadsARecordWithNoMillisecondStamp(t *testing.T) {
	home := qaGrokHome(t)
	dir := t.TempDir()
	// A launch stamped 400ms into its second, which is what time.Now() gives
	// the dispatcher and what makes this arm discriminate at all.
	since := time.Now().Truncate(time.Second).Add(-time.Minute + 400*time.Millisecond)
	const id = "01a04a2d-8c8b-7811-a6da-edf99f567e7b"
	text, _ := json.Marshal(qaGrokWorkPrompt("a-1"))
	ms := since.UnixMilli()
	qaGrokSession(t, home, dir, id, since,
		fmt.Sprintf(qaGrokUserRecNoMeta, id, since.Unix(), ms, string(text)),
		fmt.Sprintf(qaGrokDoneRec, id, since.Unix(), ms+250, qaGrokTailRefused))

	if out, observed := FindGrokTurnOutcome(dir, "a-1", since); out.Message != qaGrokRefusal || !observed {
		t.Errorf("a refusal whose prompt carries only a whole-second stamp was dropped: %+v %v", out, observed)
	}
}

// The settle line is one line — `⛔ <bead> grok refused the first turn: <msg>`,
// or its mid-flight arm — so the message is folded to one, the same fold
// FindClaudeTurnOutcome does to claude's synthetic prose. NOT measured: all 7 of this box's agent_result
// strings are single-line API errors, and the value below is synthetic and
// says so. The fold is insurance against the day a provider quotes a body,
// and this arm is what keeps it from being dropped as decorative.
func TestQAGrokTurnOutcomeFoldsAMultiLineProviderMessage(t *testing.T) {
	home := qaGrokHome(t)
	dir := t.TempDir()
	since := time.Now().Add(-time.Minute)
	const id = "01a04a2d-8c8b-7811-a6da-edf99f567e7b"
	qaGrokSession(t, home, dir, id, since.Add(time.Second),
		qaGrokTurn(id, "a-1", since.Add(time.Second),
			`"stop_reason":"error","agent_result":"API error (status 500):\n  upstream said\n\n  no\n"`)...)

	out, observed := FindGrokTurnOutcome(dir, "a-1", since)
	if !observed {
		t.Fatalf("multi-line agent_result was not read: %+v %v", out, observed)
	}
	if want := "API error (status 500): upstream said no"; out.Message != want {
		t.Errorf("message = %q, want %q", out.Message, want)
	}
}

// A dispatched session's working directory is its WORKTREE, and the dir the
// reader is handed is the repo the bead lives in — neither a substring of the
// other. A reader that FILTERED candidates by grok's encoded cwd would answer
// "nothing readable" for every worktree dispatch there is, which is the whole
// fleet: this is the arm that says the cwd orders the search and does not
// bound it.
func TestQAGrokTurnOutcomeReadsASessionRunInTheWorktreeNotTheRepo(t *testing.T) {
	home := qaGrokHome(t)
	repo := filepath.Join(t.TempDir(), "src", "posse")
	worktree := filepath.Join(t.TempDir(), ".posse", "worktrees", "posse", "persona-posse-a-1")
	since := time.Now().Add(-time.Minute)
	const id = "01a04a2d-8c8b-7811-a6da-edf99f567e7b"
	qaGrokSession(t, home, worktree, id, since.Add(time.Second),
		qaGrokTurn(id, "a-1", since.Add(time.Second), qaGrokTailRefused)...)

	out, observed := FindGrokTurnOutcome(repo, "a-1", since)
	if out.Message != qaGrokRefusal || !observed {
		t.Errorf("a refusal recorded in the session's worktree cwd was not read: %+v %v", out, observed)
	}
}

// Two sessions hold a turn on this bead in the window. The one grok ran in
// THIS dir is the dispatch's own; the stranger is read only if it has to be.
func TestQAGrokTurnOutcomePrefersTheSessionRunInThisDir(t *testing.T) {
	home := qaGrokHome(t)
	// The stranger's cwd sorts FIRST, so a reader that lost the own/rest
	// split and just walked the store in order would answer with it: without
	// this the arm passes on the order t.TempDir() happens to hand out.
	base := t.TempDir()
	other := filepath.Join(base, "aaa-elsewhere")
	dir := filepath.Join(base, "zzz-this-dispatch")
	since := time.Now().Add(-time.Minute)
	// Same record shape, a different provider string, so which candidate
	// answered is legible in the failure message.
	const strangerMsg = "API error (status 500 Internal Server Error): elsewhere"
	qaGrokSession(t, home, other, "01a0000a-0000-7000-8000-00000000000a", since.Add(time.Second),
		qaGrokTurn("01a0000a-0000-7000-8000-00000000000a", "a-1", since.Add(time.Second),
			`"stop_reason":"error","agent_result":"`+strangerMsg+`"`)...)
	qaGrokSession(t, home, dir, "01a0000b-0000-7000-8000-00000000000b", since.Add(time.Second),
		qaGrokTurn("01a0000b-0000-7000-8000-00000000000b", "a-1", since.Add(time.Second), qaGrokTailRefused)...)

	if out, _ := FindGrokTurnOutcome(dir, "a-1", since); out.Message != qaGrokRefusal {
		t.Errorf("the session run in this dir must answer first, got %q", out.Message)
	}
	// The control: name a dir neither session ran in and the same store
	// answers with the stranger. Without it "prefers" could be read off a
	// store that only ever had one usable candidate.
	if out, _ := FindGrokTurnOutcome(filepath.Join(base, "nowhere"), "a-1", since); out.Message != strangerMsg {
		t.Errorf("with nothing in this dir the reader must widen, got %q", out.Message)
	}
}

// A store that is not there is not a refusal, and neither is one that cannot
// be read: both are ("", false), the rung turnOutcomeClause prints as "looked
// and found none this pass".
func TestQAGrokTurnOutcomeUnreadableStoreIsNotHealth(t *testing.T) {
	home := qaGrokHome(t)
	dir := t.TempDir()
	if out, observed := FindGrokTurnOutcome(dir, "a-1", time.Now().Add(-time.Minute)); out.Message != "" || observed {
		t.Errorf("an empty store = %+v %v, want no message and false", out, observed)
	}
	// A sessions root replaced by a file: unreadable, not empty.
	if err := os.WriteFile(filepath.Join(home, "sessions"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, observed := FindGrokTurnOutcome(dir, "a-1", time.Now().Add(-time.Minute)); out.Message != "" || observed {
		t.Errorf("an unreadable store = %+v %v, want no message and false", out, observed)
	}
}

// A zero floor is not an open window. Once the encoded cwd stops filtering
// the candidates, `since` is the only thing standing between this reader and
// every refusal grok ever recorded on this bead, so a caller that has no
// window gets "cannot tell" rather than the oldest matching turn on the box.
func TestQAGrokTurnOutcomeRefusesAZeroWindow(t *testing.T) {
	home := qaGrokHome(t)
	dir := t.TempDir()
	const id = "01a04a2d-8c8b-7811-a6da-edf99f567e7b"
	long := time.Now().Add(-90 * 24 * time.Hour)
	qaGrokSession(t, home, dir, id, long, qaGrokTurn(id, "a-1", long, qaGrokTailRefused)...)

	// The control: with a window that reaches back to it, the same fixture
	// reads — so the arm below is the floor talking and not an empty store.
	if out, _ := FindGrokTurnOutcome(dir, "a-1", long.Add(-time.Minute)); out.Message != qaGrokRefusal {
		t.Fatalf("the fixture is not readable at all: %+v", out)
	}
	if out, observed := FindGrokTurnOutcome(dir, "a-1", time.Time{}); out.Message != "" || observed {
		t.Errorf("a zero floor = %+v %v, want no message and false", out, observed)
	}
}

// The registry half: the key grok's built-in declares resolves to this reader
// and is on offer to any runtime that writes the same store.
func TestQAGrokTurnOutcomeIsRegistered(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt, err := a.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	if TurnOutcomeReaderFor(rt) == nil {
		t.Fatalf("grok declares turn_outcome: %q and resolves to no reader", rt.TurnOutcomeAdapter)
	}
	found := false
	for _, n := range TurnOutcomeAdapters() {
		if n == TurnOutcomeGrokSessionStore {
			found = true
		}
	}
	if !found {
		t.Errorf("%q is not on offer: %v", TurnOutcomeGrokSessionStore, TurnOutcomeAdapters())
	}
}
