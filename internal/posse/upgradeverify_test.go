//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-v87a1 — `posse upgrade`'s D4 half: the status/gates
// snapshot, the diff through the NEW binary, and the automatic rollback.
//
// The claim this file defends is one sentence: an upgrade that made the shop
// worse is undone before the command exits, and an upgrade that merely
// happened while the shop moved is not. Both halves are failures that pass
// every other test in the tree — a missed rollback leaves a broken binary
// dispatching a live fleet, and a false one makes the mechanism something a
// delegate learns to re-run twice and stop reading.
//
//	the exec test   a binary that does not run, or does not name the commit
//	                this run built, is rolled back.
//	the diff        a condition code THIS BINARY RENDERS rolls back when it
//	                gains a row; a shop-state code does not, however URGENT.
//	the order       the verify runs before the epilogue, so nothing is spent
//	                through a binary that is about to be undone.
//	the rollback    goes through no binary at all, leaves the target byte
//	                for byte equal to .prev, and leaves .prev where it is.

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// ─── a posse binary whose bytes say what it will answer ──────────────────────

// fakePosse is a posse binary as these pins need one. The file's CONTENT is
// what `posse version`, `posse status` and `posse gates <p>` answer, so a
// reading taken before the rename and one taken after it differ exactly when
// the swap really happened — which is the ordering every pin below is about.
// Nothing here stubs upgradeRead, upgradeVerdict or upgradeRollback: the
// seam is one level lower, at the process boundary.
type fakePosse struct {
	Broken bool // does not exec at all
	Stamp  string
	Conds  GovSet
	Gates  map[string]fakeGates
}

type fakeGates struct{ Exit, Red int }

// bytes marshals a fake binary to the file the probe will read.
//
// The GATES rows go out in sorted persona order, and that is load-bearing
// rather than tidy: every rollback pin writes the fixture with one call and
// then asserts on a SECOND call's output (rolledBack compares the live file
// to f.now.bytes()), so a map-ranged marshaller makes the two disagree
// whenever Go hands them different orders. MEASURED 2026-11-13 at 2dc8dd75:
// the one pin with two personas in Gates red 7 of 20 runs, with BOTH orders
// among the failures — the tell that neither order was wrong and the
// marshaller simply was not a function of its receiver (ranger-base-fpny6).
func (f fakePosse) bytes() []byte {
	var b strings.Builder
	if f.Broken {
		b.WriteString("BROKEN\n")
	}
	fmt.Fprintf(&b, "STAMP %s\n", f.Stamp)
	for _, c := range f.Conds {
		fmt.Fprintf(&b, "COND %s\t%s\t%s\t%s\n", c.Class, c.ID, c.Key, c.Detail)
	}
	for _, p := range slices.Sorted(maps.Keys(f.Gates)) {
		g := f.Gates[p]
		fmt.Fprintf(&b, "GATES %s %d %d\n", p, g.Exit, g.Red)
	}
	return []byte(b.String())
}

// readFakePosse is the other half, and it is deliberately tolerant of lines
// it does not know: the fixtures in upgrade_test.go write "OLD BINARY\n",
// which must read as a binary that runs and names no commit rather than as
// an error.
func readFakePosse(path string) (fakePosse, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return fakePosse{}, err
	}
	f := fakePosse{Gates: map[string]fakeGates{}}
	for _, ln := range strings.Split(string(body), "\n") {
		switch {
		case ln == "BROKEN":
			f.Broken = true
		case strings.HasPrefix(ln, "STAMP "):
			f.Stamp = strings.TrimSpace(strings.TrimPrefix(ln, "STAMP "))
		case strings.HasPrefix(ln, "COND "):
			p := strings.Split(strings.TrimPrefix(ln, "COND "), "\t")
			if len(p) == 4 {
				f.Conds = append(f.Conds, GovCondition{Class: p[0], ID: p[1], Key: p[2], Detail: p[3]})
			}
		case strings.HasPrefix(ln, "GATES "):
			p := strings.Fields(strings.TrimPrefix(ln, "GATES "))
			if len(p) == 3 {
				exit, _ := strconv.Atoi(p[1])
				red, _ := strconv.Atoi(p[2])
				f.Gates[p[0]] = fakeGates{Exit: exit, Red: red}
			}
		}
	}
	return f, nil
}

// The fixture's own marshaller is pinned, because every rollback pin in this
// file compares bytes written by one call to bytes rendered by another, and a
// disagreement between them reds as "the rollback did not restore the binary"
// — a message that sends the next reader into upgradeRollback, which is not
// where the fault was (ranger-base-fpny6). Two personas is the smallest fixture
// that can expose it; the loop is what makes a map range fail here reliably
// rather than one run in three.
func TestFakePosseBytesIsAFunctionOfItsReceiver(t *testing.T) {
	t.Parallel()
	f := fakePosse{Stamp: "aaaaaaa", Gates: map[string]fakeGates{
		"dev": {Red: 1}, "ops": {Exit: 1, Red: 2}, "sec": {}, "qa": {Red: 3},
	}}
	want := f.bytes()
	for i := 0; i < 200; i++ {
		if got := f.bytes(); !bytes.Equal(got, want) {
			t.Fatalf("two renderings of one fixture differ on call %d:\n  %q\n  %q", i, want, got)
		}
	}
	if !strings.Contains(string(want), "GATES dev 0 1\nGATES ops 1 2\n") {
		t.Errorf("the GATES rows are not in sorted persona order:\n%s", want)
	}
}

// stubUpgradeProbe makes every `posse <verb>` this package would exec answer
// out of the named binary's own bytes.
//
// The STATUS output goes through GovReport, the real renderer, rather than
// through a format this file invents — so upgradeParseStatus is pinned
// against the thing it actually has to read, and a landing that changes
// GovReport's columns reds here rather than silently emptying the diff.
func stubUpgradeProbe(t *testing.T) {
	t.Helper()
	saved := upgradeProbe
	t.Cleanup(func() { upgradeProbe = saved })
	upgradeProbe = func(bin string, argv ...string) (int, string) {
		f, err := readFakePosse(bin)
		if err != nil {
			return -1, ""
		}
		if f.Broken {
			// What a shell says about a file it cannot exec, and an exit
			// code that is not 0 — the two things D4's first bullet reads.
			return 126, "posse: cannot execute binary file\n"
		}
		switch {
		case len(argv) == 1 && argv[0] == "version":
			return 0, "posse 0.0.0+" + f.Stamp + " (fake)\n"
		case len(argv) == 1 && argv[0] == "status":
			var b strings.Builder
			fmt.Fprintf(&b, "shop check · %s · ~/fake\n", GovSummary(f.Conds))
			GovReport(&b, f.Conds, nil)
			// The real command exits 1 on any open condition, which is the
			// measurement that makes the OUTPUT the reading (ADR 0092 D4).
			if len(f.Conds) > 0 {
				return 1, b.String()
			}
			return 0, b.String()
		case len(argv) == 2 && argv[0] == "gates":
			g := f.Gates[argv[1]]
			var b strings.Builder
			fmt.Fprintf(&b, "parity (fake) — %s:\n", argv[1])
			for i := 0; i < g.Red; i++ {
				fmt.Fprintf(&b, "    ✗ Bash(gate%d:*) → not realized at this tier\n", i)
			}
			return g.Exit, b.String()
		}
		return 2, "fake posse: unknown verb\n"
	}
}

// d4Fixture is upgradeFixture plus the two fake binaries D4 compares: the
// one at the target now, and the one the build will produce.
type d4Fixture struct {
	upgradeFixture
	now, next fakePosse
}

func newD4Fixture(t *testing.T, now, next fakePosse) d4Fixture {
	t.Helper()
	f := d4Fixture{upgradeFixture: newUpgradeFixture(t), now: now, next: next}
	// The commit the fixture installs is `c` (see newUpgradeFixture), so a
	// `next` that does not say otherwise is stamped with it — "the binary
	// that is what this run built" is the ordinary case, and a pin about
	// something else has to ask for it.
	if f.next.Stamp == "" && !f.next.Broken {
		f.next.Stamp = f.c
	}
	if err := WriteExecutable(f.target, f.now.bytes(), 0o755); err != nil {
		t.Fatal(err)
	}
	stubUpgradeProbe(t)
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error { return nil },
		func(w io.Writer, dir, repo, out string) error {
			return os.WriteFile(out, f.next.bytes(), 0o644)
		},
		func(w io.Writer, wt, target string) string {
			// The epilogue must not run on a rollback, so a pin that
			// expects one asserts on this through the report rather than
			// here: a t.Error in a shared helper would fire for the pins
			// that legitimately reach it.
			fmt.Fprintln(w, "EPILOGUE RAN")
			return "fresh"
		})
	return f
}

// run drives the whole plan with the roster D4 snapshots.
func (f d4Fixture) run(t *testing.T, personas ...string) (string, error) {
	t.Helper()
	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	plan.Personas = personas
	var out bytes.Buffer
	err = upgradeRunPlan(&out, plan)
	return out.String(), err
}

// rolledBack is the claim every rollback pin makes, and it is deliberately
// about BYTES: "the binary in place is the one that was there before this
// command ran" is only true if nothing of the new one survived.
func (f d4Fixture) rolledBack(t *testing.T) {
	t.Helper()
	live, err := os.ReadFile(f.target)
	if err != nil {
		t.Fatalf("the target is gone after a rollback: %v", err)
	}
	if !bytes.Equal(live, f.now.bytes()) {
		t.Errorf("the target holds %q after the rollback, want the binary that was live before the run", live)
	}
	prev, err := os.ReadFile(upgradePrevPath(f.target))
	if err != nil {
		t.Fatalf("%s is gone after a rollback: %v — the printed rollback command names it, and a report that names a file nobody kept is worse than no report", upgradePrevPath(f.target), err)
	}
	if !bytes.Equal(live, prev) {
		t.Errorf("the target and %s differ after a rollback:\n  target %q\n  .prev  %q", upgradePrevPath(f.target), live, prev)
	}
	if st, err := os.Stat(f.target); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Errorf("the rolled-back binary is not executable: %v %v", st, err)
	}
	// A rollback that left its own staging file beside the target is a file
	// the next run has to tell from a real one.
	if _, err := os.Stat(f.target + ".rollback"); err == nil {
		t.Error("the rollback staging file is still beside the target")
	}
}

// ─── D4 bullet 1 · the new binary must run, and must be the one built ────────

func TestUpgradeRollsBackABinaryThatDoesNotExec(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{Broken: true})
	out, err := f.run(t)
	if err == nil {
		t.Fatal("a binary that does not exec was installed and left in place")
	}
	if !strings.Contains(out, "does not run") {
		t.Errorf("the report does not say the new binary does not run:\n%s", out)
	}
	f.rolledBack(t)
	// And nothing was spent through it: the epilogue runs two scripts out of
	// the new worktree and hands one of them the just-installed binary as
	// POSSE, which on a binary about to be undone is work through the thing
	// under suspicion.
	if strings.Contains(out, "EPILOGUE RAN") {
		t.Errorf("the epilogue ran on a binary that was rolled back:\n%s", out)
	}
	// D6's record line claims an install. A rolled-back run installed
	// nothing and must not print it.
	if strings.Contains(out, "upgraded ") {
		t.Errorf("D6's record line was printed over a rollback:\n%s", out)
	}
}

func TestUpgradeRollsBackABinaryStampedWithAnotherCommit(t *testing.T) {
	// The build produced something, it runs, and it is not what this run
	// built — a stale staging file, a build that silently reused a cached
	// binary. `make test` passed on the commit; the file in place is not it.
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{Stamp: "beefbeef"})
	out, err := f.run(t)
	if err == nil {
		t.Fatal("a binary stamped with another commit was left installed")
	}
	if !strings.Contains(out, "beefbeef") {
		t.Errorf("the report does not name the stamp it read:\n%s", out)
	}
	f.rolledBack(t)
}

// A stamp that is a PREFIX of the commit is the ordinary case and must not
// roll back: what the Makefile stamps is `git rev-parse --short`, whose
// length is git's to choose and is eight characters on this repo today.
func TestUpgradeAcceptsTheShortStampTheBuildWrites(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{})
	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	// Re-write the staged binary with a SHORT stamp rather than the full
	// commit the fixture defaults to.
	short := fakePosse{Stamp: plan.To[:8]}
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error { return nil },
		func(w io.Writer, dir, repo, out string) error {
			return os.WriteFile(out, short.bytes(), 0o644)
		},
		func(w io.Writer, wt, target string) string { return "fresh" })
	var out bytes.Buffer
	if err := upgradeRunPlan(&out, plan); err != nil {
		t.Fatalf("a binary stamped with git's own short form was rolled back: %v\n%s", err, out.String())
	}
	if body, _ := os.ReadFile(f.target); !bytes.Equal(body, short.bytes()) {
		t.Errorf("the target does not hold the new binary: %q", body)
	}
}

// ─── D4 bullet 3 · the status diff, and what it must NOT fire on ─────────────

// The bead's two headline cases, side by side, because the pair is the
// claim: the same mechanism must fire on one and stay silent on the other.
func TestUpgradeRollsBackOnAGainedBinaryRenderedConditionAndNotOnAShopOne(t *testing.T) {
	for _, c := range []struct {
		name string
		cond GovCondition
		back bool
		says string
	}{
		// G12 — an L3 hook wall is degraded, judged against THIS binary's
		// own render of the wall. Nothing but the swap can have changed it.
		{"a G12 hook-wall row", GovCondition{ID: "G12", Class: GovLane, Key: "hook-wall:pre-commit:~/src/x", Detail: "the wall at ~/src/x is not this binary's render"}, true, "LANE G12"},
		// G17 — a cage map that lowers a tier, read out of promoted config
		// and judged by this binary's own resolver.
		{"a G17 cage-map row", GovCondition{ID: "G17", Class: GovLane, Key: "cage-realize:linux", Detail: "cage_realize_linux: seatbelt→shims"}, true, "LANE G17"},
		// G7 — the code ADR 0092 D4 did not name. `arm-broken` reads
		// autostart_interval: out of a static config through THIS BINARY's
		// ParseInterval, so a new binary whose grammar narrowed disarms the
		// fleet's only instrument and says so here.
		{"a G7 arm-broken row", GovCondition{ID: "G7", Class: GovUrgent, Key: "arm-broken", Detail: "autostart_interval: \"5m\" is not an interval"}, true, "URGENT G7"},
		// A row this binary cannot name at all. The new binary is reporting
		// a condition this one has no class for, which is the one case where
		// "I do not know" must mean "roll back".
		{"a code this binary has no class for", GovCondition{ID: "G99", Class: GovLane, Key: "something-new", Detail: "a row a later binary invented"}, true, "no class for"},

		// And the other half. Every one of these moves on its own inside the
		// twelve minutes `make test` takes, so a row that appeared says
		// nothing about the binary.
		{"a G1 blocked seat", GovCondition{ID: "G1", Class: GovLane, Key: "blocked:sess", Detail: "sess (claude) is blocked on an approval"}, false, ""},
		{"a G3 question past its age", GovCondition{ID: "G3", Class: GovUrgent, Key: "question:x-1", Detail: "x-1 has been open 5h"}, false, ""},
		{"a G6 budget stop", GovCondition{ID: "G6", Class: GovUrgent, Key: "budget-stop:week", Detail: "week is at 100%"}, false, ""},
		{"a G8 pause a human wrote", GovCondition{ID: "G8", Class: GovUrgent, Key: "paused", Detail: "paused by the operator"}, false, ""},
		// G11 is about WHICH BINARY IS INSTALLED, and origin/main may have
		// moved further forward than this install did while the suite ran.
		// A good install can deepen it, so it can never be evidence.
		{"a G11 launcher-behind row", GovCondition{ID: "G11", Class: GovLane, Key: "launcher-behind:16", Detail: "the launcher is 17 commits behind"}, false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newD4Fixture(t,
				fakePosse{Stamp: "aaaaaaa"},
				fakePosse{Conds: GovSet{c.cond}})
			out, err := f.run(t)
			if !c.back {
				if err != nil {
					t.Fatalf("a %s rolled the install back — that condition moves on its own while `make test` runs, so a run that appeared during it is not evidence about the binary:\n%v\n%s", c.name, err, out)
				}
				if body, _ := os.ReadFile(f.target); !bytes.Equal(body, f.next.bytes()) {
					t.Errorf("the new binary is not in place after a clean verify: %q", body)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s did not roll the install back:\n%s", c.name, out)
			}
			if !strings.Contains(out, c.says) {
				t.Errorf("the report does not say %q:\n%s", c.says, out)
			}
			f.rolledBack(t)
		})
	}
}

// A condition that was ALREADY there before the swap is not a gain, whatever
// its class — otherwise every upgrade on a shop with one standing G12 would
// roll itself back and the mechanism would be unusable on the only kind of
// box it runs on.
func TestUpgradeDoesNotRollBackAStandingCondition(t *testing.T) {
	standing := GovCondition{ID: "G12", Class: GovLane, Key: "hook-wall:pre-commit:~/src/x", Detail: "stale"}
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Conds: GovSet{standing}},
		fakePosse{Conds: GovSet{standing}})
	if out, err := f.run(t); err != nil {
		t.Fatalf("a condition that was there before the swap rolled the install back: %v\n%s", err, out)
	}
}

// The DETAIL is documented as the half that may move freely (govern.go), so
// the same condition re-worded, re-measured or re-aged is not a gain. This
// is the pin under this file's one reading of D4 that differs from the ADR's
// words: the trigger is counted by (class, row), not keyed by subject,
// because the two sides of the diff are two different binaries.
func TestUpgradeDoesNotRollBackAConditionWhoseDetailMoved(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Conds: GovSet{{ID: "G7", Class: GovUrgent, Key: "loop-mute", Detail: "the watch log is 31m old"}}},
		fakePosse{Conds: GovSet{{ID: "G7", Class: GovUrgent, Key: "loop-mute", Detail: "the watch log is 43m old"}}})
	if out, err := f.run(t); err != nil {
		t.Fatalf("a condition whose DETAIL moved rolled the install back: %v\n%s", err, out)
	}
}

// A status that answered before the swap and does not answer after it is a
// binary that cannot report on the shop — and every condition test above
// would otherwise read its silence as an all-clear.
func TestUpgradeRollsBackWhenStatusStopsAnswering(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{})
	// A binary that runs, names the right commit, and dies inside `posse
	// status`. Layered over the fixture's own probe rather than replacing
	// it, so everything but that one verb answers exactly as it would.
	saved := upgradeProbe
	t.Cleanup(func() { upgradeProbe = saved })
	upgradeProbe = func(bin string, argv ...string) (int, string) {
		if len(argv) == 1 && argv[0] == "status" {
			if read, err := readFakePosse(bin); err == nil && read.Stamp == f.c {
				return 2, "panic: assignment to entry in nil map\n"
			}
		}
		return saved(bin, argv...)
	}
	out, err := f.run(t)
	if err == nil {
		t.Fatalf("a binary whose `posse status` died was left installed:\n%s", out)
	}
	if !strings.Contains(out, "printed no shop check") {
		t.Errorf("the report does not say status stopped answering:\n%s", out)
	}
	f.rolledBack(t)
}

// ─── D4 bullet 2 · the gates diff ────────────────────────────────────────────

func TestUpgradeRollsBackWhenAGatesRunNewlyExitsNonZero(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Gates: map[string]fakeGates{"dev": {Exit: 0}}},
		fakePosse{Gates: map[string]fakeGates{"dev": {Exit: 1}}})
	out, err := f.run(t, "dev")
	if err == nil {
		t.Fatalf("a persona whose gates run went from 0 to 1 did not roll the install back:\n%s", out)
	}
	if !strings.Contains(out, "posse gates dev") {
		t.Errorf("the report does not name the persona whose gates run broke:\n%s", out)
	}
	f.rolledBack(t)
}

func TestUpgradeRollsBackWhenAParityBlockGainsANotHeldRow(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Gates: map[string]fakeGates{"dev": {Red: 1}, "ops": {Red: 0}}},
		fakePosse{Gates: map[string]fakeGates{"dev": {Red: 1}, "ops": {Red: 2}}})
	out, err := f.run(t, "dev", "ops")
	if err == nil {
		t.Fatalf("a parity block that gained two not-held rows did not roll the install back:\n%s", out)
	}
	if !strings.Contains(out, "posse gates ops") {
		t.Errorf("the report does not name the persona whose wall got worse:\n%s", out)
	}
	// And it did not blame the persona whose count was unchanged.
	if strings.Contains(out, "posse gates dev") {
		t.Errorf("the report blames a persona whose wall did not change:\n%s", out)
	}
	f.rolledBack(t)
}

// A wall that got BETTER is not a trigger — an upgrade whose whole purpose
// is landing a gate fix must not undo itself for landing it.
func TestUpgradeDoesNotRollBackWhenAParityBlockLosesARow(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Gates: map[string]fakeGates{"dev": {Red: 3}}},
		fakePosse{Gates: map[string]fakeGates{"dev": {Red: 0}}})
	if out, err := f.run(t, "dev"); err != nil {
		t.Fatalf("an upgrade that CLOSED three degraded rows rolled itself back: %v\n%s", err, out)
	}
}

// ─── the readers ─────────────────────────────────────────────────────────────

// upgradeGatesRed counts ADR 0017 §2's three presentation words and not a
// fourth. RATIFIED DEGRADATION is the one that matters: it is standing state
// on every reading of a host whose cage map lowers a tier, so counting it
// would put a permanent non-zero on both sides of every diff and — worse —
// would make the count move whenever the ✓ rows beside it did.
func TestUpgradeGatesRedCountsTheNotHeldRowsAndNotTheRatifiedOnes(t *testing.T) {
	t.Parallel()
	const report = `wall renderer (gates) · 0.5.3+70ab74b7 is 6 commit(s) behind main
multiplexer: rex — seats are Rex sessions
  launches here: claude
  REFUSED here, before the claim — run them on a herdr instance: bob
parity (ADR 0002 §4, ADR 0003 §3) — what the wall realizes per runtime:
  claude @ shims/standard: RATIFIED DEGRADATION
    ✓ Bash(git push:*)             → cooperative (L1 shim)
    ⚖ cage_realize: linux seatbelt→shims (promoted 7a0cf34460ba)
  codex @ shims/standard: DEGRADED
    ✗ Bash(bd admin:*) → not realized at this tier
    ⊘ SelfSandbox: declared difference
`
	// One REFUSED line, one DEGRADED header, one ✗ row. The ✓, ⚖ and ⊘ rows
	// are each a verdict that is NOT "a gate the wall does not hold".
	if got, want := upgradeGatesRed(report), 3; got != want {
		t.Errorf("upgradeGatesRed = %d, want %d — over:\n%s", got, want, report)
	}
	if got := upgradeGatesRed("  claude @ shims/standard: RATIFIED DEGRADATION\n"); got != 0 {
		t.Errorf("RATIFIED DEGRADATION counts as %d not-held row(s), want 0 — it is a difference the promoted constitution already waived, standing on every reading", got)
	}
}

// upgradeParseStatus reads GovReport's two columns and nothing else in
// `posse status`'s output. The lines it must skip are the ones that would
// turn a reading into a condition that never existed.
func TestUpgradeParseStatusReadsTheConditionRowsAlone(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("posse 0.5.3+70ab74b7 (herdr-native)\n")
	b.WriteString("the launcher is 6 commit(s) behind main\n")
	b.WriteString("shop check · 1 URGENT · 2 LANE · ~/homes/x\n")
	GovReport(&b, GovSet{
		{ID: "G8", Class: GovUrgent, Key: "paused", Detail: "paused by the operator"},
		{ID: "G12", Class: GovLane, Key: "hook-wall:a", Detail: "stale"},
		{ID: "", Class: GovLane, Key: "backup-stale", Detail: "the backup is 3h old"},
	}, []error{Die("herdr: no socket")})
	b.WriteString("shop pulse · 86 closed today\n")
	// A DETAIL could open with any word at all, URGENT included. A reader
	// that keyed on the first field alone would invent a condition here.
	b.WriteString("LANE    not-a-row  a line whose second field is not a row name\n")

	got := upgradeParseStatus(b.String())
	want := map[string]int{
		"URGENT G8":                    1,
		"LANE G12":                     1,
		"LANE " + GovCondition{}.Row(): 1,
	}
	if len(got) != len(want) {
		t.Fatalf("upgradeParseStatus found %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("upgradeParseStatus[%q] = %d, want %d (whole: %v)", k, got[k], n, got)
		}
	}
}

func TestUpgradeStampOfReadsTheCommitOutOfTheVersionLine(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"posse 0.5.3+70ab74b7 (herdr-native)\n": "70ab74b7",
		"posse 0.5.3+70ab74b7-dirty (rex)\n":    "70ab74b7",
		// A release tag names no commit, which is not a stamp this verb can
		// check against — and D4 rolls back rather than guessing.
		"posse 0.5.3 (herdr-native)\n": "",
		"":                             "",
		"posse\n":                      "",
	} {
		if got := upgradeStampOf(in); got != want {
			t.Errorf("upgradeStampOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every row name the partition knows is a class, and an unknown one is
// BINARY-rendered rather than shop-state. The direction is the whole point:
// this table is complete for this binary (the QA pin says so), so a row the
// NEW binary printed and this one cannot name is a condition it invented.
func TestUpgradeUnknownConditionCodeIsTreatedAsBinaryRendered(t *testing.T) {
	t.Parallel()
	if got := upgradeCondSource("G12"); got != upgradeBinaryRendered {
		t.Errorf("G12 reads as %v, want binary-rendered", got)
	}
	if got := upgradeCondSource("G1"); got != upgradeShopState {
		t.Errorf("G1 reads as %v, want shop-state", got)
	}
	if got := upgradeCondSource("G11"); got != upgradeInstallSubject {
		t.Errorf("G11 reads as %v, want install-subject", got)
	}
	if got := upgradeCondSource("G404"); got != upgradeBinaryRendered {
		t.Errorf("a code with no class reads as %v, want binary-rendered — the safe answer is the one that rolls back", got)
	}
}

// ─── the rollback itself ─────────────────────────────────────────────────────

// upgradeRollback is the one step that must work when everything else does
// not, so it is pinned on its own as well as through the verb: no child
// process, no PATH, no shim between the decision and the file.
func TestUpgradeRollbackRestoresPrevByteForByteAndKeepsIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := dir + "/posse"
	old := bytes.Repeat([]byte("O"), 4096)
	if err := WriteExecutable(upgradePrevPath(target), old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(target, []byte("THE BROKEN NEW ONE"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := upgradeRollback(target); err != nil {
		t.Fatalf("upgradeRollback: %v", err)
	}
	if body, _ := os.ReadFile(target); !bytes.Equal(body, old) {
		t.Errorf("the target holds %d bytes after the rollback, want the %d of .prev", len(body), len(old))
	}
	// .prev stays. The command this run printed before the swap names it,
	// and overwriting it with the binary being rolled back would destroy the
	// only copy of the one being rolled back TO.
	if body, _ := os.ReadFile(upgradePrevPath(target)); !bytes.Equal(body, old) {
		t.Errorf("%s no longer holds the binary that was live", upgradePrevPath(target))
	}
	if st, err := os.Stat(target); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Errorf("the rolled-back binary is not executable: %v %v", st, err)
	}
}

// With no .prev there is nothing to roll back to, and the one thing that
// must not happen is a silent return: the binary in place is the new one,
// and the report has to say so in the same breath as it says it failed.
func TestUpgradeRollbackSaysSoWhenThereIsNothingToRollBackTo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := dir + "/posse"
	if err := WriteExecutable(target, []byte("THE BROKEN NEW ONE"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := upgradeRollback(target)
	if err == nil {
		t.Fatal("upgradeRollback returned nil with no .prev to restore")
	}
	for _, say := range []string{"cannot be read", "is the one this run installed"} {
		if !strings.Contains(err.Error(), say) {
			t.Errorf("the refusal does not say %q:\n%v", say, err)
		}
	}
}
