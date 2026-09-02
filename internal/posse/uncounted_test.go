package posse

// Hermetic tests for the account stage's degrade and brake (ADR 0013 §5,
// ranger-base-9mz). Same substrate as the overflow tests — fake herdr, fake
// bd, the test binary re-execing as both — and no plan guard and no budget
// caps, so the only thing deciding anything here is the account stage.
//
// codex is the fixture's runtime because it is a shipped built-in whose
// dollars posse cannot read: its adapter counts turns and tokens and prices
// none of them, which is the account stage's degrade. The column is a
// property of the shipped adapters, not of a yaml a test invented.
//
// It was grok until ranger-base-0lg6, when grok's adapter — which reads the
// provider's own per-turn dollars — was finally reflected in the account
// stage and grok became counted.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// codexPID pins a persona to the account-degraded runtime. Nothing else is declared,
// so parity is clean and the bead's labels pick the tier.
func codexPID(name string) string {
	return "---\nname: " + name + "\ndescription: test\nlabels: [go]\nruntime: codex\n---\nYou are " + name + ".\n"
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
		if err := os.WriteFile(filepath.Join(b.App.AgentsDir, p+".md"), []byte(codexPID(p)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo := planRepo(t, ready, `[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, cfg)
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	return &uncountedFixture{d: d, errb: errb, b: b, fake: fake, repo: repo}
}

// oneCodexBead is the common case: one ready bead, one seat, one launch.
func oneCodexBead(t *testing.T, cfg string) *uncountedFixture {
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
	f := oneCodexBead(t, "")

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("unset is unlimited: the bead must launch, got n=%d:\n%s", n, out)
	}
	for _, want := range []string{
		"account-degraded codex: sent 1 bead(s) this pass",
		"1 in the last 7d",
		"prices none of them",
		"uncounted_cap_codex: is unset — unlimited and loud",
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
	if len(fields) != 4 || fields[1] != "codex" || fields[2] != "a-1" || fields[3] != "ranger" {
		t.Errorf("ledger line = %q, want `RFC3339 codex a-1 ranger`", l[0])
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
	f := oneCodexBead(t, "uncounted_cap_codex: 2\n")
	now := time.Now()
	f.seedUncounted(t,
		LedgerEntry{now.Add(-2 * time.Hour), "codex", "old-1", "ranger"},
		LedgerEntry{now.Add(-6 * 24 * time.Hour), "codex", "old-2", "ranger"},
	)

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "account-degraded: uncounted_cap_codex 2/2 in 7d — skipped") {
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
	f := oneCodexBead(t, "uncounted_cap_codex: 3\n")
	now := time.Now()
	f.seedUncounted(t,
		LedgerEntry{now.Add(-2 * time.Hour), "codex", "old-1", "ranger"},
		LedgerEntry{now.Add(-8 * 24 * time.Hour), "codex", "old-2", "ranger"},
		LedgerEntry{now.Add(-30 * 24 * time.Hour), "codex", "old-3", "ranger"},
		LedgerEntry{now.Add(-time.Hour), "gemini", "old-4", "ranger"},
	)

	if got, err := f.b.App.UncountedCount("codex", now); err != nil || got != 1 {
		t.Errorf("UncountedCount(codex) = %d, %v — want 1 (entries past 7d do not count)", got, err)
	}
	if got, _ := f.b.App.UncountedCount("gemini", now); got != 1 {
		t.Errorf("UncountedCount(gemini) = %d, want 1 — the count is per runtime", got)
	}

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("a cap with room must not skip, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "account-degraded codex: sent 1 bead(s) this pass, 2/3 in 7d (uncounted_cap_codex:)") {
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
	f := uncountedPass(t, "uncounted_cap_codex: 1\n",
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`,
		"ranger", "scout")

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("exactly one launch fits under a cap of 1, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "account-degraded: uncounted_cap_codex 1/1 in 7d — skipped") {
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
			f := oneCodexBead(t, "uncounted_cap_codex: "+raw+"\n")

			n, _ := f.d.Run("", "", 0)
			out := dispatcherOut(f.d)
			if n != 1 {
				t.Fatalf("a malformed cap must not brake, got n=%d:\n%s", n, out)
			}
			lines := strings.Split(strings.TrimRight(f.errb.String(), "\n"), "\n")
			want := fmt.Sprintf("uncounted_cap_codex: %q is not a positive bead count", raw)
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
	f := oneCodexBead(t, "uncounted_cap_codex: 5\n")
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

// The other half of that rule (ranger-base-ws09). A ledger that can be READ
// but not appended to is worse than an unreadable one: it counts, so the cap
// looks armed, but it counts the same number every pass because nothing this
// pass launches ever reaches it. Cap 1 over an empty unwritable ledger admits
// one launch per pass forever and records none of them, so the appendability
// is checked before the count is spent, not warned about after the launch.
func TestUncountedCapUnwritableLedgerSkips(t *testing.T) {
	f := oneCodexBead(t, "uncounted_cap_codex: 1\n")
	os.MkdirAll(f.b.App.StateDir, 0o755)
	if err := os.WriteFile(f.b.App.UncountedLogPath(), nil, 0o444); err != nil {
		t.Fatal(err)
	}
	// 0444 is a promise about a uid, not about the process: root keeps its
	// write and would turn a zero-launch pass into a false pass.
	if err := f.b.App.AppendUncounted(LedgerEntry{Runtime: "codex"}); err == nil {
		t.Skip("test process can append to a 0444 ledger")
	}

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 0 || !strings.Contains(out, "a cap whose launches cannot be recorded is a cap of zero that reads as room; skipped") {
		t.Fatalf("an unwritable ledger under an armed cap must park, got n=%d:\n%s", n, out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("nothing may be claimed: %s", calls)
	}
	if b, err := os.ReadFile(f.b.App.UncountedLogPath()); err != nil || len(b) != 0 {
		t.Errorf("the ledger must be left exactly as found: %q err=%v", b, err)
	}
}

// The refill tally's grouping key for that skip is its own, not the
// unreadable one's: inside a refill only the kind survives, and an operator
// reading "N uncounted_cap_codex: ledger unwritable" has to be sent to a
// different fix than "unreadable" would send them to.
func TestUncountedUnwritableSkipKindIsItsOwn(t *testing.T) {
	f := oneCodexBead(t, "uncounted_cap_codex: 1\n")
	os.MkdirAll(f.b.App.StateDir, 0o755)
	if err := os.WriteFile(f.b.App.UncountedLogPath(), nil, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := f.b.App.AppendUncounted(LedgerEntry{Runtime: "codex"}); err == nil {
		t.Skip("test process can append to a 0444 ledger")
	}
	line, kind := f.d.uncountedSkip("codex")
	if want := "uncounted_cap_codex: ledger unwritable"; kind != want {
		t.Errorf("kind = %q, want %q (line: %s)", kind, want, line)
	}
}

// An UNSET cap is unlimited, and unlimited does not become a brake because
// the ledger is unwritable: there is no cap to spend, so there is nothing to
// fail closed on and the launch happens. What it must not do is go quiet —
// the failed append is named on stderr, and the pass's account line says the
// 7d count is short from now on, because that number is this pass's memory
// and the file the next pass reads does not have it.
func TestUncountedUnsetCapLaunchesAndNamesTheShortfall(t *testing.T) {
	f := oneCodexBead(t, "")
	os.MkdirAll(f.b.App.StateDir, 0o755)
	if err := os.WriteFile(f.b.App.UncountedLogPath(), nil, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := f.b.App.AppendUncounted(LedgerEntry{Runtime: "codex"}); err == nil {
		t.Skip("test process can append to a 0444 ledger")
	}

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("an unset cap is unlimited whatever the ledger can do, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "1 of them never reached") || !strings.Contains(out, "short by that many from now on") {
		t.Errorf("the account line must name the shortfall:\n%s", out)
	}
	if e := f.errb.String(); !strings.Contains(e, "uncounted ledger not written for a-1") {
		t.Errorf("the failed append must still be named on stderr: %q", e)
	}
}

// overflowUnlogged's twin. The probe cannot see a write that fails for a
// reason an open does not — a full disk is the obvious one — so when the
// append itself fails the pass treats the ledger as unwritable from that
// moment: the rest of the pass parks on this runtime rather than spending a
// cap against a file that has just proved it cannot record the spending, and
// the report carries the shortfall into the operator's reading.
func TestUncountedFailedAppendArmsTheBrake(t *testing.T) {
	f := oneCodexBead(t, "uncounted_cap_codex: 5\n")
	os.MkdirAll(f.b.App.StateDir, 0o755)
	if err := os.WriteFile(f.b.App.UncountedLogPath(), nil, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := f.b.App.AppendUncounted(LedgerEntry{Runtime: "codex"}); err == nil {
		t.Skip("test process can append to a 0444 ledger")
	}
	// The pool exactly as a passing probe would have left it — readable,
	// appendable as far as any open could tell — so the only thing under
	// test here is what the failing append itself does.
	p := &uncountedPool{Cap: 5, Raw: "5", Why: accountDegrade(&Runtime{Name: "codex"})}
	f.d.uncounted = map[string]*uncountedPool{"codex": p}

	f.d.noteUncounted(RepoIssue{BdIssue: BdIssue{ID: "a-1"}}, "ranger", "codex")
	if p.Unlogged != 1 || p.Unappendable == nil {
		t.Fatalf("a failed append must be carried: Unlogged=%d Unappendable=%v", p.Unlogged, p.Unappendable)
	}
	if p.Used != 1 || p.Sent != 1 {
		t.Errorf("the launch still happened and is still booked: Used=%d Sent=%d", p.Used, p.Sent)
	}
	line, kind := f.d.uncountedSkip("codex")
	if !strings.Contains(line, "cannot be appended to") || kind != "uncounted_cap_codex: ledger unwritable" {
		t.Errorf("the rest of the pass must park on this runtime: %q / %q", line, kind)
	}
	f.d.uncountedReport()
	if out := dispatcherOut(f.d); !strings.Contains(out, "1/5 in 7d (uncounted_cap_codex:), 1 of them never reached") {
		t.Errorf("the account line must name the shortfall:\n%s", out)
	}
}

// --dry-run acts on nothing: no ledger line, and the report says what the
// pass WOULD have sent rather than what it did.
func TestUncountedDryRunReportsWithoutSpending(t *testing.T) {
	f := oneCodexBead(t, "")
	f.d.DryRun = true

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 || !strings.Contains(out, "account-degraded codex: would send 1 bead(s) this pass") {
		t.Fatalf("a dry pass must report in the conditional, got n=%d:\n%s", n, out)
	}
	if l := f.uncountedLedger(t); l != nil {
		t.Errorf("a dry run spends nothing and ledgers nothing: %v", l)
	}
}

// An ADR 0010 overflow move onto an account-degraded pool is on BOTH
// ledgers: the overflow log answers "what did the plan guard move", this one
// answers "what went somewhere posse cannot price". Neither number answers
// the other's question, and the cap that applies is the pool the bead LANDS
// on.
//
// The target is codex, not grok: since ranger-base-0lg6 an overflow move
// onto grok lands on a COUNTED pool and this ledger correctly stays empty
// (TestOverflowOntoACountedPoolIsNotUncountedSpend below).
func TestUncountedCountsAnOverflowMove(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: codex\nplan_guard_overflow_cap: 5\n",
		overflowPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("the eligible bead must still move, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "account-degraded codex: sent 1 bead(s) this pass") {
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

// ── the dead key on a counted runtime (ADR 0010 §3, ranger-base-2eeb) ───────
//
// grok left the account-degraded column (ranger-base-0lg6): its adapter
// prices what it reads, so uncountedFor returns nil for it before the cap
// key is ever read and `uncounted_cap_grok:` brakes nothing. That much is
// ADR 0013 §5's law and stays. What must not happen is it happening
// QUIETLY — a key the operator set and still believes in is exactly the
// cap-that-stopped-capping failure uncounted.go is written against.

// pidOnRuntime is codexPID retargeted at another runtime — the account
// stage keys on the runtime a launch resolves to, and nothing else in the
// PID matters here.
func pidOnRuntime(name, runtime string) string {
	return strings.Replace(codexPID(name), "runtime: codex", "runtime: "+runtime, 1)
}

// uncountedPassOn is uncountedPass with every seat pinned to `runtime`
// instead of codex. uncountedPass writes the codex PIDs itself, so this
// overwrites them before the pass reads them.
func uncountedPassOn(t *testing.T, runtime, cfg, ready string, personas ...string) *uncountedFixture {
	t.Helper()
	f := uncountedPass(t, cfg, ready, personas...)
	for _, p := range personas {
		if err := os.WriteFile(filepath.Join(f.b.App.AgentsDir, p+".md"), []byte(pidOnRuntime(p, runtime)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// A set cap on a counted runtime is named ONCE for the pass — not once per
// bead, and not once per seat — and it brakes nothing: two beads over two
// seats both launch under a cap of 1, which is what "the key is dead" means.
func TestCountedCapKeyIsNamedDeadOncePerPass(t *testing.T) {
	f := uncountedPassOn(t, "grok", "uncounted_cap_grok: 1\n",
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`,
		"ranger", "scout")

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out, errs := dispatcherOut(f.d), f.errb.String()
	if n != 2 {
		t.Fatalf("a dead cap of 1 must brake neither bead, got n=%d:\n%s\n%s", n, out, errs)
	}
	// Positive witness that the launches really went to grok: without it a
	// PID that failed to retarget leaves this measuring codex, where the
	// key is alive and the silence would be correct. Both lines, because
	// one bead on grok and one on codex would also read as two launches.
	if got := strings.Count(out, "creating session "); got != 2 || strings.Count(out, "grok/") != 2 {
		t.Fatalf("want two create lines, both on grok, got %d create lines:\n%s", got, out)
	}
	if got := strings.Count(errs, "uncounted_cap_grok:"); got != 1 {
		t.Errorf("want the dead key named exactly once a pass, got %d:\n%s", got, errs)
	}
	for _, want := range []string{
		`config uncounted_cap_grok: "1" does not apply`,
		"prices grok's spend",
		"the brake on grok is budget_pass:/budget_day: over those dollars",
		"(ADR 0010 §3)",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("the line must carry %q:\n%s", want, errs)
		}
	}
	// Unarmed is not "armed at 0%": the pool guard is only offered here,
	// never reported as a threshold nobody set.
	if strings.Contains(errs, "grok_guard_week: 0%") {
		t.Errorf("an unset pool guard must not be printed as a threshold:\n%s", errs)
	}
	// Dead means dead in both directions: no brake, no report, no ledger.
	if strings.Contains(out, "account-degraded") {
		t.Errorf("grok is counted; the pass must not degrade it:\n%s", out)
	}
	if got := len(f.uncountedLedger(t)); got != 0 {
		t.Errorf("nothing may be ledgered for a counted runtime, got %d lines", got)
	}
}

// The line points at the brake that DOES apply, and for grok that is the
// pool guard wherever it is armed — an operator whose dead key was standing
// in for a pool brake wants grok_guard_week:, not the wallet caps.
func TestDeadCapKeyPointsAtTheArmedPoolGuard(t *testing.T) {
	f := uncountedPassOn(t, "grok", "uncounted_cap_grok: 1\ngrok_guard_week: 70\n"+grokPoolCfg,
		`[{"id":"a-1","title":"t","labels":["go"]}]`, "ranger")

	if _, err := f.d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	errs := f.errb.String()
	if got := strings.Count(errs, "uncounted_cap_grok:"); got != 1 {
		t.Fatalf("want the dead key named exactly once a pass, got %d:\n%s", got, errs)
	}
	want := "the brakes on grok are grok_guard_week: 70% over the pool and budget_pass:/budget_day: over the dollars"
	if !strings.Contains(errs, want) {
		t.Errorf("want the armed pool guard named as the brake (%q):\n%s", want, errs)
	}
}

// No key, no line. The runtime is counted and the operator set nothing —
// there is no news, and a standing line per counted runtime per pass is the
// noise the report above already refuses to make.
func TestCountedRuntimeWithNoCapKeyIsSilent(t *testing.T) {
	f := uncountedPassOn(t, "grok", "", `[{"id":"a-1","title":"t","labels":["go"]}]`, "ranger")

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("the bead must launch, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(createLineOf(t, out), "grok/") {
		t.Fatalf("the create line must name grok:\n%s", out)
	}
	if strings.Contains(f.errb.String(), "uncounted_cap") {
		t.Errorf("an unset key is not news:\n%s", f.errb.String())
	}
}

// The mirror, and the reason the warning keys on CostPriced() and not on
// "an adapter exists": codex's adapter reads it and prices nothing, so
// codex KEEPS the column and keeps its cap. Its key brakes and is never
// called dead.
func TestUncountedRuntimeCapKeyIsNotCalledDead(t *testing.T) {
	f := uncountedPass(t, "uncounted_cap_codex: 1\n",
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`,
		"ranger", "scout")

	n, _ := f.d.Run("", "", 0)
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("the cap must still brake the second bead, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "account-degraded: uncounted_cap_codex 1/1 in 7d — skipped") {
		t.Errorf("codex's cap is alive; it must brake:\n%s", out)
	}
	if strings.Contains(f.errb.String(), "does not apply") {
		t.Errorf("a live cap must not be called dead:\n%s", f.errb.String())
	}
}
