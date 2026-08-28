package rhq

// Hermetic tests for the account stage's degrade and brake (ADR 0013 §5,
// ranger-base-9mz). Same substrate as the overflow tests — fake herdr, fake
// bd, the test binary re-execing as both — and no plan guard and no budget
// caps, so the only thing deciding anything here is the account stage.
//
// grok is the fixture's runtime because it is a shipped built-in with no
// `cost_adapter:`: the uncounted column is a property of the runtime table,
// not of a yaml a test invented.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// grokPID pins a persona to the uncounted runtime. Nothing else is declared,
// so parity is clean and the bead's labels pick the tier.
func grokPID(name string) string {
	return "---\nname: " + name + "\ndescription: test\nlabels: [go]\nruntime: grok\n---\nYou are " + name + ".\n"
}

type uncountedFixture struct {
	d    *Dispatcher
	errb *strings.Builder
	b    *HerdrBackend
	fake string
	repo string
}

// uncountedPass wires one pass over `ready` with a seat per persona named.
// No plan_guard_*, no budget_*: Dial E is dormant and the guard takes no
// reading, so nothing in the output belongs to another dial.
func uncountedPass(t *testing.T, cfg, ready string, personas ...string) *uncountedFixture {
	t.Helper()
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	for _, p := range personas {
		if err := os.WriteFile(filepath.Join(b.App.AgentsDir, p+".md"), []byte(grokPID(p)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo := planRepo(t, ready, `[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, cfg)
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	return &uncountedFixture{d: d, errb: errb, b: b, fake: fake, repo: repo}
}

// oneGrokBead is the common case: one ready bead, one seat, one launch.
func oneGrokBead(t *testing.T, cfg string) *uncountedFixture {
	t.Helper()
	return uncountedPass(t, cfg, `[{"id":"a-1","title":"t","labels":["go"]}]`, "ranger")
}

// uncountedLedger reads $StateDir/uncounted.log back as lines.
func (f *uncountedFixture) uncountedLedger(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(f.b.App.UncountedLogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// seedUncounted writes a ledger by hand — the rolling window's history.
func (f *uncountedFixture) seedUncounted(t *testing.T, es ...LedgerEntry) {
	t.Helper()
	os.MkdirAll(f.b.App.StateDir, 0o755)
	var s strings.Builder
	for _, e := range es {
		s.WriteString(e.line())
	}
	if err := os.WriteFile(f.b.App.UncountedLogPath(), []byte(s.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// §5's first obligation with the cap UNSET: the launch happens — unset is
// unlimited, not off — and the pass says out loud how many beads it sent to
// a runtime nothing meters, naming the key that would brake it.
func TestUncountedUnsetIsUnlimitedAndLoud(t *testing.T) {
	f := oneGrokBead(t, "")

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("unset is unlimited: the bead must launch, got n=%d:\n%s", n, out)
	}
	for _, want := range []string{
		"account-degraded grok: sent 1 bead(s) this pass",
		"1 in the last 7d",
		"no cost adapter reads grok",
		"uncounted_cap_grok: is unset — unlimited and loud",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the pass must say %q:\n%s", want, out)
		}
	}
	if f.errb.Len() != 0 {
		t.Errorf("an unset cap is not a config error: %q", f.errb.String())
	}
	// The ledger is the count's only store, so an uncapped pass still writes
	// it — a cap set next week counts the weeks before it.
	l := f.uncountedLedger(t)
	if len(l) != 1 {
		t.Fatalf("want exactly one ledger line, got %v", l)
	}
	fields := strings.Fields(l[0])
	if len(fields) != 4 || fields[1] != "grok" || fields[2] != "a-1" || fields[3] != "ranger" {
		t.Errorf("ledger line = %q, want `RFC3339 grok a-1 ranger`", l[0])
	}
	if _, err := time.Parse(time.RFC3339, fields[0]); err != nil {
		t.Errorf("ledger timestamp %q is not RFC3339: %v", fields[0], err)
	}
}

// A counted runtime is silent: no account line, no ledger, nothing. The
// degrade is a property of the missing adapter, not of dispatch.
func TestUncountedCountedRuntimeSaysNothing(t *testing.T) {
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	writePersona(t, b.App, "ranger", "[go]") // no runtime: → claude, which is counted
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 || strings.Contains(out, "account-degraded") {
		t.Fatalf("a counted runtime is not degraded, got n=%d:\n%s", n, out)
	}
	if _, err := os.Stat(b.App.UncountedLogPath()); !os.IsNotExist(err) {
		t.Errorf("nothing may be ledgered for a counted runtime (%v)", err)
	}
	if errb.Len() != 0 {
		t.Errorf("and nothing on stderr: %q", errb.String())
	}
}

// The brake: a cap already reached inside the rolling window skips further
// launches to that runtime, and the skip line names the numbers that stopped
// it. Nothing is claimed and nothing is appended.
func TestUncountedCapSkipsFurtherLaunches(t *testing.T) {
	f := oneGrokBead(t, "uncounted_cap_grok: 2\n")
	now := time.Now()
	f.seedUncounted(t,
		LedgerEntry{now.Add(-2 * time.Hour), "grok", "old-1", "ranger"},
		LedgerEntry{now.Add(-6 * 24 * time.Hour), "grok", "old-2", "ranger"},
	)

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "account-degraded: uncounted_cap_grok 2/2 in 7d — skipped") {
		t.Fatalf("a reached cap must skip and say so, got n=%d:\n%s", n, out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a bead the cap stopped must not be claimed: %s", calls)
	}
	if got := len(f.uncountedLedger(t)); got != 2 {
		t.Errorf("a skipped bead appends nothing: %d lines", got)
	}
	// Nothing was sent, so there is no per-pass total to report — the skip
	// line above is what makes this pass loud.
	if strings.Contains(out, "sent 1 bead") {
		t.Errorf("a pass that launched nothing must not report a launch:\n%s", out)
	}
}

// §5's window is a rolling seven days of beads, per runtime: entries inside
// it count, entries past it do not, and another runtime's week is not this
// one's. With room left the bead launches and the report carries both
// numbers.
func TestUncountedCapRolling7d(t *testing.T) {
	f := oneGrokBead(t, "uncounted_cap_grok: 3\n")
	now := time.Now()
	f.seedUncounted(t,
		LedgerEntry{now.Add(-2 * time.Hour), "grok", "old-1", "ranger"},
		LedgerEntry{now.Add(-8 * 24 * time.Hour), "grok", "old-2", "ranger"},
		LedgerEntry{now.Add(-30 * 24 * time.Hour), "grok", "old-3", "ranger"},
		LedgerEntry{now.Add(-time.Hour), "codex", "old-4", "ranger"},
	)

	if got, err := f.b.App.UncountedCount("grok", now); err != nil || got != 1 {
		t.Errorf("UncountedCount(grok) = %d, %v — want 1 (entries past 7d do not count)", got, err)
	}
	if got, _ := f.b.App.UncountedCount("codex", now); got != 1 {
		t.Errorf("UncountedCount(codex) = %d, want 1 — the count is per runtime", got)
	}

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("a cap with room must not skip, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "account-degraded grok: sent 1 bead(s) this pass, 2/3 in 7d (uncounted_cap_grok:)") {
		t.Errorf("the report must carry the window count against the cap:\n%s", out)
	}
	if got := len(f.uncountedLedger(t)); got != 5 {
		t.Errorf("want the four seeded lines plus this launch, got %d", got)
	}
}

// The cap counts this pass's own launches, so it bites WITHIN a pass and not
// only between them: a cap of one over two seats launches once and skips the
// second bead. Without this a --watch loop with a long gather could spend a
// whole week's cap in one pass and only notice next pass.
func TestUncountedCapBitesInsideOnePass(t *testing.T) {
	f := uncountedPass(t, "uncounted_cap_grok: 1\n",
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`,
		"ranger", "scout")

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("exactly one launch fits under a cap of 1, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "account-degraded: uncounted_cap_grok 1/1 in 7d — skipped") {
		t.Errorf("the second bead must be stopped by the cap, not by a seat:\n%s", out)
	}
	if strings.Contains(out, "lane busy") {
		t.Errorf("the second seat was free — the cap is what stopped it:\n%s", out)
	}
	if got := len(f.uncountedLedger(t)); got != 1 {
		t.Errorf("one launch, one ledger line, got %d", got)
	}
}

// A typo is not a cap. It is named once on stderr and the runtime stays
// unlimited and loud — the rule budget_pass: and plan_guard_overflow_cap:
// already keep, because a cap that silently stopped capping looks exactly
// like one nobody set.
func TestUncountedCapMalformedIsUnlimitedAndNamed(t *testing.T) {
	for _, raw := range []string{"lots", "0", "-3"} {
		t.Run(raw, func(t *testing.T) {
			f := oneGrokBead(t, "uncounted_cap_grok: "+raw+"\n")

			n, _ := f.d.Run("", "", 0)
			out := dispatcherOut(f.d)
			if n != 1 {
				t.Fatalf("a malformed cap must not brake, got n=%d:\n%s", n, out)
			}
			lines := strings.Split(strings.TrimRight(f.errb.String(), "\n"), "\n")
			want := fmt.Sprintf("uncounted_cap_grok: %q is not a positive bead count", raw)
			if len(lines) != 1 || !strings.Contains(lines[0], want) {
				t.Errorf("want exactly one stderr line naming the key, got %q", f.errb.String())
			}
			if !strings.Contains(out, "is not a cap — unlimited and loud") {
				t.Errorf("the pass report must not call a typo a cap:\n%s", out)
			}
		})
	}
}

// An armed cap over a ledger nobody can read is the unarmed case wearing the
// armed case's clothes. The same rule the overflow ledger and Dial E keep:
// an unreadable ledger is not a licence to spend.
func TestUncountedCapUnreadableLedgerSkips(t *testing.T) {
	f := oneGrokBead(t, "uncounted_cap_grok: 5\n")
	os.MkdirAll(f.b.App.StateDir, 0o755)
	if err := os.MkdirAll(f.b.App.UncountedLogPath(), 0o755); err != nil {
		t.Fatal(err)
	}

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "a cap that counts nothing is not a brake; skipped") {
		t.Fatalf("an unreadable ledger under an armed cap must park, got n=%d:\n%s", n, out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("nothing may be claimed: %s", calls)
	}
}

// --dry-run acts on nothing: no ledger line, and the report says what the
// pass WOULD have sent rather than what it did.
func TestUncountedDryRunReportsWithoutSpending(t *testing.T) {
	f := oneGrokBead(t, "")
	f.d.DryRun = true

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 || !strings.Contains(out, "account-degraded grok: would send 1 bead(s) this pass") {
		t.Fatalf("a dry pass must report in the conditional, got n=%d:\n%s", n, out)
	}
	if l := f.uncountedLedger(t); l != nil {
		t.Errorf("a dry run spends nothing and ledgers nothing: %v", l)
	}
}

// An ADR 0010 overflow move onto an uncounted pool is on BOTH ledgers: the
// overflow log answers "what did the plan guard move", this one answers
// "what went somewhere nothing meters". Neither number answers the other's
// question, and the cap that applies is the pool the bead LANDS on.
func TestUncountedCountsAnOverflowMove(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n",
		overflowPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("the eligible bead must still move, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "account-degraded grok: sent 1 bead(s) this pass") {
		t.Errorf("a bead moved onto an uncounted pool is still uncounted spend:\n%s", out)
	}
	if got := len(f.ledger(t)); got != 1 {
		t.Errorf("overflow ledger: want 1 line, got %d", got)
	}
	b, err := os.ReadFile(f.b.App.UncountedLogPath())
	if err != nil {
		t.Fatalf("uncounted ledger: %v", err)
	}
	if got := strings.Count(strings.TrimSpace(string(b)), "\n") + 1; got != 1 {
		t.Errorf("uncounted ledger: want 1 line, got %d (%q)", got, b)
	}
}
