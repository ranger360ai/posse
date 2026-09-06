package posse

// Helpers lifted out of dispatch_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// qaRepo writes a ready list (and optional show state) into a fresh repo dir
// and points the config's beads: list at it.
func qaRepo(t *testing.T, a *App, ready, show string) string {
	t.Helper()
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(ready), 0o644)
	if show != "" {
		os.WriteFile(filepath.Join(repo, "fake-show.json"), []byte(show), 0o644)
	}
	os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	return repo
}

func idleClaude(t *testing.T, fake string) {
	t.Helper()
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
}

// agentPerLaunch makes every created workspace get an idle agent as soon
// as its command is typed — what Dial F's fresh-session-per-bead needs
// when a test dispatches more than one bead.
func agentPerLaunch(t *testing.T, fake string) {
	t.Helper()
	os.WriteFile(filepath.Join(fake, "pane-run-starts-agent"), nil, 0o644)
}

// workingClaude: herdr sees the session's agent mid-turn — the state that
// makes a --wait timeout a check-in rather than a failure (rangerhq-1z0).
func workingClaude(t *testing.T, fake string) {
	t.Helper()
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"working","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
}

// delivered is what a dispatched session was actually asked, whichever
// channel carried it. Since ADR 0013 §2 there are two: a `prompt: typed`
// runtime gets `agent prompt <pane> <text>` in herdr's call log, and a
// `prompt: argv` one gets the same text in state/prompts/<session>.txt,
// read by the `$(cat …)` on its launch line. A test that asks "what did the
// persona hear" must read both or it is asking about the transport.
func delivered(t *testing.T, a *App, fake string) string {
	t.Helper()
	out := calls(t, fake)
	ents, _ := os.ReadDir(a.WorkPromptDir())
	for _, e := range ents {
		b, _ := os.ReadFile(filepath.Join(a.WorkPromptDir(), e.Name()))
		out += "\n--- " + e.Name() + " ---\n" + string(b)
	}
	return out
}

func bdCalls(t *testing.T, fake string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	return string(b)
}

// deadPersonaSession creates the persona session for repo as an earlier
// pass would have, then makes it look like the agent died `age` ago: the
// workspace is alive, herdr sees no agent, and the launch record reads that
// old.
//
// The age is a PARAMETER, and age 0 writes no stamp at all, because of
// ranger-base-5i4c. It used to age every session by a fixed hour, and the
// two grace tests then flipped it back to `time.Now()` with their own
// unchecked readMeta/writeMeta pair. That flip is the whole fixture: the
// record is the store of record RelaunchAgent and settledForReap measure
// their graces against, so a flip that does not land does not weaken the
// fixture, it INVERTS it into TestDispatchRelaunchesDeadAgent's — a session
// launched an hour ago, which the pass then correctly relaunches, reported
// by the caller as "want no relaunch inside the grace window, got n=1".
// Measured: with the record left at -1h and nothing else changed, the pass
// prints `relaunching ranger in <session>`, returns n=1 and types a second
// `pane run` — byte for byte the two lines this bead was filed with, seen in
// a full-package run and never reproducible alone (the flip lands every time
// on a quiet box).
//
// So the young fixture no longer travels through the old one. startPlanned
// already stamps `launched: <now>` on a persona create and CHECKS that
// write, so age 0 is the record CreateSession itself wrote and there is no
// second write to fail.
//
// The read-back below is for the callers that do age it: the record has to
// be shown to say what the caller asked for BEFORE the pass reads it, so a
// stamp that did not land fails as a fixture and never as a verdict.
func deadPersonaSession(t *testing.T, b *HerdrBackend, fake, persona, repo, bead string, age time.Duration) string {
	t.Helper()
	name := SessionForBead(persona, repo, bead)
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: repo, Agent: persona}); err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		m, ok := b.readMeta(name)
		if !ok {
			t.Fatal("no meta after CreateSession")
		}
		m.Launched = time.Now().Add(-age)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := launchAgeIs(b, name, age); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	os.Remove(filepath.Join(fake, "agents.json")) // no agent anywhere
	return name
}

// launchStampSlack is how much later than the fixture asked for a record may
// read and still be the fixture. The create, the stamp and this read-back are
// filesystem round trips on a box that runs the whole fleet's suites at once,
// so it cannot be zero; and it is derived from the grace the callers measure
// against rather than hand-picked, because the one thing it must not be is
// LARGER than that grace. A slack over RelaunchGrace would let a record the
// pass is going to relaunch into walk past this guard and fail at the
// verdict, which is the failure this whole change exists to stop.
const launchStampSlack = DefaultRelaunchGrace / 3

// launchAgeIs reads the launch record back and says whether it reports the
// age the fixture asked for. It is a function rather than four lines inside
// deadPersonaSession so that the discriminator itself can be pinned
// (TestQADeadPersonaSessionRefusesARecordThatDidNotTakeTheStamp): a guard
// nobody has watched fail is a guard nobody has measured.
//
// BOTH SIDES, and the second one was added verifying this bead's close
// (ranger-base-xk9ag). The guard shipped one-sided — it refused a record
// reading OLDER than asked and accepted one reading YOUNGER — which leaves
// the bead's own defect standing in the other direction and in this same
// helper: with `age` an hour, a stamp that does not take reads as a session
// launched now, walks past the guard, and lands as
// TestDispatchRelaunchesDeadAgent's verdict ("want relaunch + dispatch, got
// n=0"). Measured: with `m.Launched = time.Now()` in place of
// `time.Now().Add(-age)` under `go test -overlay`, the one-sided guard let
// exactly that through. A fixture that cannot be built must say so as a
// fixture whichever way it drifted.
func launchAgeIs(b *HerdrBackend, name string, want time.Duration) error {
	m, ok := b.readMeta(name)
	if !ok {
		return fmt.Errorf("no launch record for %s to read the stamp back from", name)
	}
	if m.Launched.IsZero() {
		return fmt.Errorf("%s carries no launched: stamp, so every grace reads it as infinitely old", name)
	}
	got := time.Since(m.Launched)
	if got > want+launchStampSlack {
		return fmt.Errorf("%s reads as launched %s ago, not %s — the stamp did not land, and the pass would relaunch into it", name, got.Round(time.Second), want)
	}
	// The young side has no slack to give: the read-back can only ever be
	// LATER than the stamp, so a record younger than the age asked for is
	// never the clock and always the write. Rounding is the one thing to
	// allow for, and launchStampSlack is not needed for it — a record the
	// caller asked to be an hour old that reads as seconds old is the
	// inversion, not a rounding.
	if want > 0 && got < want-launchStampSlack {
		return fmt.Errorf("%s reads as launched %s ago, not %s — the stamp did not age, and the pass would decline to relaunch into it", name, got.Round(time.Second), want)
	}
	return nil
}

// insideGrace answers the question RelaunchAgent asks of the same record.
func insideGrace(b *HerdrBackend, name string, grace time.Duration) bool {
	m, ok := b.readMeta(name)
	return ok && time.Since(m.Launched) < grace
}

func dispatchParallelPass(t *testing.T, createDelayMS string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	writePersona(t, b.App, "scout", "[py]")
	repoA, repoB := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(repoA, "fake-ready.json"), []byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(filepath.Join(repoA, "fake-show.json"), []byte(`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`), 0o644)
	os.WriteFile(filepath.Join(repoB, "fake-ready.json"), []byte(`[{"id":"b-1","title":"u","labels":["py"]}]`), 0o644)
	os.WriteFile(filepath.Join(repoB, "fake-show.json"), []byte(`[{"id":"b-1","title":"u","status":"closed","assignee":"scout"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repoA+"\n  - "+repoB+"\n"), 0o644)
	// Both workspaces get an idle agent as soon as they are created.
	os.WriteFile(filepath.Join(fake, "pane-run-starts-agent"), nil, 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"},{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)
	// Every prompt is held until both are in flight, then both are released
	// together — the pass either gathers or it deadlocks on the barrier.
	os.WriteFile(filepath.Join(fake, "prompt-barrier"), []byte("2"), 0o644)
	if createDelayMS != "" {
		os.WriteFile(filepath.Join(fake, "create-delay-ms"), []byte(createDelayMS), 0o644)
	}

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 2 || strings.Count(out, "closed by") != 2 {
		t.Errorf("want both beads dispatched and closed, got n=%d:\n%s", n, out)
	}
	if strings.Count(calls(t, fake), "workspace create") != 2 || strings.Count(calls(t, fake), "agent prompt") != 2 {
		t.Errorf("want two sessions and two prompts:\n%s", calls(t, fake))
	}
	// Gathered, not serial: both prompts were in flight at the same moment.
	// Nothing here is timed, because every wall-clock margin in this
	// assertion has eventually been eaten by a loaded box — first the 900ms
	// ceiling on the whole pass (rangerhq-g6lx), then the 500ms the prompts
	// were allowed to be staggered by (rangerhq-3ig1), both of which grow
	// with load and accused a dispatcher that was gathering correctly. The
	// barrier turns the invariant into the fake's release condition: a
	// gathered pass reaches two-in-flight whatever the machine is doing,
	// and a serial one cannot reach it at all, so each of its prompts is
	// released alone by the barrier's timeout.
	w := promptWindows(t, fake)
	if len(w) != 2 {
		t.Fatalf("want two barrier-held prompts, got %d:\n%s", len(w), calls(t, fake))
	}
	for i, p := range w {
		if p.release != "gathered" {
			t.Errorf("prompt %d was released %q after %s — it was the only one in flight, so the pass awaited serially rather than gathering",
				i, p.release, time.Duration(p.end-p.start))
		}
	}
	// True by construction once both cleared the barrier — kept because the
	// overlap, not the barrier, is what this test is about.
	if gap := max(w[0].start, w[1].start) - min(w[0].end, w[1].end); gap >= 0 {
		t.Errorf("prompts never overlapped — awaited serially, not gathered (%s between them)", time.Duration(gap))
	}
	if !strings.Contains(out, "2 prompt(s) in flight") {
		t.Errorf("gather banner missing:\n%s", out)
	}
	// Two beads for one persona still go one per session per pass.
	if strings.Count(out, "(prompted, ") != 2 {
		t.Errorf("want exactly two prompted lines:\n%s", out)
	}
}

// joinPrompts waits out the `agent prompt` legs a bare fireLoop left in
// flight, so the test's own t.TempDir cleanup is the LAST writer to its tree
// (ranger-base-nqtvs, the sibling of ranger-base-06bvw).
//
// A launch hands its wait to a goroutine and returns (dispatch.go): the pass
// is not what joins it, gather is, and a test that calls fireLoop directly
// has no gather. That goroutine forks a fake herdr child whose RHQ_FAKE_DIR
// is a t.TempDir being removed right about then, and every write the child
// makes lands in the middle of that RemoveAll. Two outcomes, one producer:
// the child recreates 002/calls.log between the unlink of it and the rmdir
// of 002, so cleanup reds with "directory not empty" and the test that was
// green fails anyway; or the child gets far enough to MkdirAll and the whole
// tree is silently rebuilt behind a green test, which is what fills the
// operator's $TMPDIR.
//
// MEASURED 2026-09-02, isolated GOTMPDIR, with the binary held open so the
// goroutine reaches cmd.Start() at all (standalone, os.Exit beats it — which
// is why this is invisible outside a full-package run):
//
//	HEAD   `agent prompt` still absent from calls.log when cleanup began,
//	       every run, both tests; and with the leg held on prompt-delay-ms,
//	       one stale tree per run holding only 002/prompt-windows/<ns>-<pid>
//	joined the line is always already there; no stale tree, immediately or
//	       after a settle
//
// Mode reads the outer `Test*` directory ONLY: testing.TempDir makes that one
// 0700 (MkdirTemp) and every numbered child 0777&^umask = 0755
// (testing.go:1481), so a 0755 `002` is ordinary and proves nothing — it is a
// 0755 OUTER dir that means something MkdirAll'd the tree back.
//
// The result channel IS the join, and nothing weaker would be: the goroutine
// sends only after Herdr.Run's cmd.Run() has waited the child out, so a
// receive means that child is exited and reaped. An `unseen` pending has no
// goroutine behind it at all (dispatch.go says so where it makes one) and
// therefore nothing to receive.
//
// Register it in a t.Cleanup taken AFTER every t.TempDir the fixture takes —
// newTestBackend's three and qaRepo's — so LIFO runs the join FIRST.
func joinPrompts(t *testing.T, pending []*pendingBead) {
	t.Helper()
	for _, p := range pending {
		if p.unseen {
			continue
		}
		select {
		case r := <-p.result:
			// Put it back: an arm that gathers after the join still gets to
			// read its own result out of the buffered channel.
			p.result <- r
		case <-time.After(joinWait):
			t.Errorf("the %s prompt leg never returned in %s — its fake herdr child outlives this test and rebuilds the tree t.TempDir is removing", p.session, joinWait)
		}
	}
}
