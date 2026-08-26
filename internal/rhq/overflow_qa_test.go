package rhq

// Adversarial pins for ADR 0010's rolling-window overflow cap. These use the
// same hermetic fake bd/herdr substrate as overflow_test.go, plus the real
// launcher flock: the claim under attack is a maximum across passes, not just
// a counter that works inside one process.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestQAOverflowFoundSessionKeepsItsRuntime(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 1\n",
		overflowPID, `["go","tier:standard"]`)
	session := SessionForBead("ranger", f.repo, "a-1")
	if err := f.b.CreateSession(NewSessionOpts{
		Name: session, Dir: f.repo, Agent: "ranger", Runtime: GuardedRuntime, Tier: TierStandard,
	}); err != nil {
		t.Fatal(err)
	}

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "already runs on "+GuardedRuntime+" (only sessions this pass creates move)") {
		t.Fatalf("found session must stay on its recorded runtime and park on that meter; n=%d\n%s", n, out)
	}
	if got := f.ledger(t); len(got) != 0 {
		t.Fatalf("found session is not an overflow move; ledger = %v", got)
	}
}

func TestQAOverflowCapStopsALaterBeadInTheSamePass(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 1\n",
		overflowPID, `["go","tier:standard"]`)
	writePersona(t, f.b.App, "other", "[js]")
	if err := os.WriteFile(filepath.Join(f.repo, "fake-ready.json"), []byte(`[
  {"id":"a-1","title":"one","labels":["go","tier:standard"]},
  {"id":"a-2","title":"two","labels":["js","tier:standard"]}
]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.repo, "fake-show.json"), []byte(`[
  {"id":"a-1","title":"one","status":"closed"},
  {"id":"a-2","title":"two","status":"closed"}
]`), 0o644); err != nil {
		t.Fatal(err)
	}
	agentPerLaunch(t, f.fake)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 || !strings.Contains(out, "overflow grok: 1/1 in 7d — skipped") {
		t.Fatalf("cap 1 must launch one eligible bead and park the later one; n=%d\n%s", n, out)
	}
	if got := len(f.ledger(t)); got != 1 {
		t.Fatalf("cap 1 wrote %d ledger entries; want 1", got)
	}
}

func TestQAOverflowExplicitOffMeterRuntimeIsUngated(t *testing.T) {
	f := overflowPass(t, "", overflowPID, `["go","tier:standard"]`)
	f.d.Runtime = "grok"

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 || strings.Contains(out, "— skipped") || strings.Contains(out, "← overflow") {
		t.Fatalf("explicit off-meter runtime must launch normally through the hot guarded meter; n=%d\n%s", n, out)
	}
	if got := f.ledger(t); len(got) != 0 {
		t.Fatalf("explicit off-meter runtime is not an overflow move; ledger = %v", got)
	}
}

func TestQAOverflowMissingTargetSkipsWithoutClaim(t *testing.T) {
	const target = "no-such-overflow-runtime"
	f := overflowPass(t, "plan_guard_overflow: "+target+"\nplan_guard_overflow_cap: 1\n",
		overflowPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, target) || !strings.Contains(out, "— skipped") {
		t.Fatalf("missing overflow runtime must skip this bead and name the target; n=%d\n%s", n, out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Fatalf("missing overflow runtime claimed a bead: %s", calls)
	}
}

func TestQAOverflowUnreadableLedgerDisablesThePass(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 1\n",
		overflowPID, `["go","tier:standard"]`)
	// Opening a directory succeeds, but scanning it returns an I/O error.
	// This exercises the read-error arm after the path has been opened.
	if err := os.MkdirAll(f.b.App.OverflowLogPath(), 0o755); err != nil {
		t.Fatal(err)
	}

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || !strings.Contains(f.errb.String(), "overflow ledger") || !strings.Contains(f.errb.String(), "unreadable") {
		t.Fatalf("unreadable ledger must turn overflow off for the pass; n=%d stderr=%q\n%s",
			n, f.errb.String(), dispatcherOut(f.d))
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Fatalf("unreadable ledger still claimed a bead: %s", calls)
	}
}

func TestQAOverflowTargetCannotBeTheGuardedRuntime(t *testing.T) {
	t.Skip("ranger-base-ay0h: the guarded runtime is accepted as its own overflow target")

	f := overflowPass(t, "plan_guard_overflow: claude\nplan_guard_overflow_cap: 1\n",
		overflowPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a hot %s meter launched %d bead on that same meter via plan_guard_overflow\n%s",
			GuardedRuntime, n, dispatcherOut(f.d))
	}
	if got := f.ledger(t); len(got) != 0 {
		t.Fatalf("same-meter target must spend and ledger nothing, got %v", got)
	}
}

func TestQAOverflowRefusesAReadableButUnwritableLedger(t *testing.T) {
	t.Skip("ranger-base-2y96: append failure only warns, so every pass spends against the same stale count")

	const cfg = "plan_guard_overflow: grok\nplan_guard_overflow_cap: 1\n"
	f1 := overflowPass(t, cfg, overflowPID, `["go","tier:standard"]`)
	f2 := overflowPass(t, cfg, overflowPID, `["go","tier:standard"]`)
	agentPerLaunch(t, f2.fake)
	f2.b.App.StateDir = f1.b.App.StateDir

	if err := os.MkdirAll(f1.b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f1.b.App.OverflowLogPath(), nil, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f1.b.App.OverflowLogPath(), 0o444); err != nil {
		t.Fatal(err)
	}
	// Establish that this environment realizes the hostile condition. A
	// root test process could otherwise turn the repro into a false pass.
	if err := f1.b.App.AppendOverflow(OverflowEntry{Runtime: "grok"}); err == nil {
		t.Skip("test process can append to a 0444 ledger")
	}

	total := 0
	for _, f := range []*overflowFixture{f1, f2} {
		n, err := f.d.Run("", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		total += n
	}
	if total != 0 {
		t.Fatalf("readable but unwritable overflow ledger admitted %d unrecorded launches with cap 1; want 0\npass 1 stderr: %s\npass 2 stderr: %s",
			total, f1.errb.String(), f2.errb.String())
	}
}

func TestQAOverflowCorruptTargetLedgerLineFailsClosed(t *testing.T) {
	t.Skip("ranger-base-lasj: malformed target lines are silently skipped, so the cap undercounts")

	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 1\n",
		overflowPID, `["go","tier:standard"]`)
	if err := os.MkdirAll(f.b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The target, bead and persona survived, but an interrupted/manual write
	// left a timestamp that is not RFC3339. The weekly count is unknowable;
	// treating the line as zero is the expensive answer.
	if err := os.WriteFile(f.b.App.OverflowLogPath(),
		[]byte("2026-08-26T12:00 grok prior-1 ranger\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("corrupt ledger line for the active target was counted as zero and admitted %d launch; want overflow off\n%s",
			n, dispatcherOut(f.d))
	}
}

func TestQAOverflowCapReadIsSerializedWithLaunch(t *testing.T) {
	t.Skip("ranger-base-af98: overflow count is read before the launcher flock, so two passes spend a cap of one")

	const cfg = "plan_guard_overflow: grok\nplan_guard_overflow_cap: 1\n"
	f1 := overflowPass(t, cfg, overflowPID, `["go","tier:standard"]`)
	f2 := overflowPass(t, cfg, overflowPID, `["go","tier:standard"]`)
	agentPerLaunch(t, f2.fake)

	// Two dispatch processes for one RHQ_HOME have separate App values but
	// share StateDir: that is both the overflow ledger and launcher-lock
	// namespace. Their beads repos remain distinct so a bd claim race cannot
	// hide a stale cap read.
	f2.b.App.StateDir = f1.b.App.StateDir
	var out1, out2 syncBuf
	f1.d.Out, f2.d.Out = &out1, &out2

	// Make both passes take the rolling-window reading before either may
	// launch. If that reading belongs to the launcher critical section, the
	// second pass must re-read the first pass's ledger entry after it waits.
	held, err := lockLaunches(f1.b.App, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	type result struct {
		n   int
		err error
	}
	results := make(chan result, 2)
	var started sync.WaitGroup
	started.Add(2)
	run := func(f *overflowFixture) {
		started.Done()
		n, err := f.d.Run("", "", 0)
		results <- result{n: n, err: err}
	}
	go run(f1)
	go run(f2)
	started.Wait()
	waitForOut(t, &out1, "launcher lock held")
	waitForOut(t, &out2, "launcher lock held")
	held.Release()

	total := 0
	for range 2 {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		total += r.n
	}
	t.Logf("concurrent cap result: total=%d\npass 1:\n%s\npass 2:\n%s", total, out1.String(), out2.String())
	if total > 1 {
		t.Fatalf("two concurrent passes launched %d overflow beads with cap 1; want at most 1\npass 1:\n%s\npass 2:\n%s",
			total, out1.String(), out2.String())
	}
	if got := len(f1.ledger(t)); got > 1 {
		t.Fatalf("overflow ledger has %d launches with cap 1; want at most 1", got)
	}
}
