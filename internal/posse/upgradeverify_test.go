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
	"os/exec"
	"path/filepath"
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

	// Render is what `posse gates render-probe <dir>` answers (ADR 0092 D4
	// amendment 3), and the zero value is the healthy one — 0, every render
	// refuses — so every pin written before this field stays about what it
	// was written about.
	Render int
	// NoRenderProbe is a binary from BEFORE that verb existed, and it does
	// not answer 2: MEASURED 2026-11-13 at 34bbe053, `posse gates
	// render-probe` on such a binary falls through the gates sub-verb
	// ladder to LoadAgent and dies with `no such agent: render-probe`,
	// EXIT 1 — the same code the verb itself uses for "a render does not
	// refuse". That collision is why the reading is a mark on stdout and
	// not an exit code, and this flag is how the pins exercise it.
	NoRenderProbe bool
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
	if f.NoRenderProbe {
		b.WriteString("NO-RENDER-PROBE\n")
	}
	fmt.Fprintf(&b, "RENDER %d\n", f.Render)
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
		case ln == "NO-RENDER-PROBE":
			f.NoRenderProbe = true
		case strings.HasPrefix(ln, "RENDER "):
			f.Render, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "RENDER ")))
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
// upgradeProbeLog records the cwd each probed verb was run in, which is what
// ADR 0092 D4 amendment 2 made a property of this verb rather than of
// whoever typed it. Keyed by the whole argv so `gates dev` and `gates ops`
// are two entries and `version` is its own.
type upgradeProbeLog struct{ Dir map[string]string }

func stubUpgradeProbe(t *testing.T) *upgradeProbeLog {
	t.Helper()
	log := &upgradeProbeLog{Dir: map[string]string{}}
	saved := upgradeProbe
	t.Cleanup(func() { upgradeProbe = saved })
	upgradeProbe = func(dir, bin string, argv ...string) (int, string) {
		log.Dir[strings.Join(argv, " ")] = dir
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
		// Before the two-argument `gates <persona>` arm, because this one
		// is three arguments and reading it as a persona named
		// "render-probe" is exactly the fall-through the real OLD binary
		// does (and the reason NoRenderProbe exists).
		case len(argv) >= 2 && argv[0] == "gates" && argv[1] == "render-probe":
			if f.NoRenderProbe {
				return 1, "posse: no such agent: render-probe (looked in ~/fake/agents)\n"
			}
			slot := "refuses"
			if f.Render == 1 {
				slot = "DOES NOT REFUSE"
			}
			if f.Render == 2 {
				return 2, RenderProbeMark + " could not ask — fake\n"
			}
			return f.Render, fmt.Sprintf("%s ~/fake · pre-push %s · prepare-commit-msg %s\n", RenderProbeMark, slot, slot)
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
	return log
}

// d4Fixture is upgradeFixture plus the two fake binaries D4 compares: the
// one at the target now, and the one the build will produce.
type d4Fixture struct {
	upgradeFixture
	now, next fakePosse
	probe     *upgradeProbeLog
	// repos is what CmdUpgrade reads off `beads_visibility:` and the plan
	// carries into the verify block's re-render line. Empty unless a pin
	// asks for it: the fixture declares no repos of its own.
	repos []string
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
	f.probe = stubUpgradeProbe(t)
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error { return nil },
		func(w io.Writer, dir, repo, out string) error {
			return os.WriteFile(out, f.next.bytes(), 0o644)
		},
		func(w io.Writer, wt string, plan upgradePlan) string {
			// The epilogue must not run on a rollback, so a pin that
			// expects one asserts on this through the report rather than
			// here: a t.Error in a shared helper would fire for the pins
			// that legitimately reach it.
			fmt.Fprintln(w, "EPILOGUE RAN")
			return upgradeGateUnchanged
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
	plan.Repos = f.repos
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
		func(w io.Writer, wt string, plan upgradePlan) string { return upgradeGateUnchanged })
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
		// G12 IS THE OTHER INSTALL SUBJECT (ADR 0092 D4 amendment 1,
		// ranger-base-qfhph). A repo's git hook is a COPY of this binary's
		// render, and D7 keeps the re-render out of this verb's scope — so
		// a G12 row after the swap is the designed state after every
		// landing that changes the render, saying the operator's half has
		// not happened yet and nothing about whether the new binary is
		// broken. Identity cannot answer that question in any case: it
		// cannot tell a binary that renders a BROKEN hook from one that
		// renders a DIFFERENT one. The row is reported, never a trigger —
		// TestUpgradeReportsTheReRenderWhenTheSwapRaisesG12 pins what the
		// operator is told instead.
		{"a G12 hook-wall row", GovCondition{ID: "G12", Class: GovLane, Key: "hook-wall:prepare-commit-msg:~/src/x", Detail: "the wall at ~/src/x is not this binary's render"}, false, ""},
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
	upgradeProbe = func(dir, bin string, argv ...string) (int, string) {
		if len(argv) == 1 && argv[0] == "status" {
			if read, err := readFakePosse(bin); err == nil && read.Stamp == f.c {
				return 2, "panic: assignment to entry in nil map\n"
			}
		}
		return saved(dir, bin, argv...)
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

// ─── D4 amendment 1 · what the operator is told about the hook wall ──────────

// The G12 rows a good install raises are not a trigger, and they are not
// silence either: ADR 0092 D4 amendment 1 gives the verb one duty over them,
// which is to print the re-render D7 keeps out of its scope.
func TestUpgradeReportsTheReRenderWhenTheSwapRaisesG12(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa"},
		fakePosse{Conds: GovSet{
			{ID: "G12", Class: GovLane, Key: "hook-wall:pre-push:~/src/x", Detail: "ours but stale"},
			{ID: "G12", Class: GovLane, Key: "hook-wall:prepare-commit-msg:~/src/x", Detail: "ours but stale"},
		}})
	f.repos = []string{"/srv/src/x", "/srv/src/y"}
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("two new G12 rows rolled the install back: %v\n%s", err, out)
	}
	if !strings.Contains(out, "gained 2 G12 hook-wall row(s)") {
		t.Errorf("the report does not say how many hook-wall rows the swap raised:\n%s", out)
	}
	if !strings.Contains(out, "posse gates install-hooks /srv/src/x /srv/src/y") {
		t.Errorf("the report does not print the re-render over the declared repos:\n%s", out)
	}
	if !strings.Contains(out, "never runs it") {
		t.Errorf("the report does not say the verb leaves that command to the operator (ADR 0092 D7):\n%s", out)
	}
	// And the install really stood: the line is a note, not a refusal.
	if body, _ := os.ReadFile(f.target); !bytes.Equal(body, f.next.bytes()) {
		t.Errorf("the new binary is not in place: %q", body)
	}
}

// The other direction. An install that raised no hook-wall row has nothing
// to tell the operator, and a standing line would train them to skip it.
func TestUpgradeDoesNotPrintTheReRenderWithoutANewG12Row(t *testing.T) {
	standing := GovCondition{ID: "G12", Class: GovLane, Key: "hook-wall:pre-push:~/src/x", Detail: "ours but stale"}
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Conds: GovSet{standing}},
		fakePosse{Conds: GovSet{standing}})
	f.repos = []string{"/srv/src/x"}
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("a standing G12 row rolled the install back: %v\n%s", err, out)
	}
	if strings.Contains(out, "install-hooks") {
		t.Errorf("the report prescribes a re-render over a wall this install did not change:\n%s", out)
	}
}

// ─── D4 amendment 2 · where the gates readings are taken ─────────────────────

// `posse gates <p>` probes the L3 hooks OF ITS CWD, so the delegate's cwd
// used to decide which tree D4's two readings compared — and a delegate
// standing in a worktree of a declared repo read a wall the render change
// had just made stale. The cwd is the verb's now, fixed to the source repo
// D3.1 named, whatever directory the command was typed in.
func TestUpgradeGatesReadingsRunInTheSourceRepoWhateverTheCallersCwd(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Gates: map[string]fakeGates{"dev": {}}},
		fakePosse{Gates: map[string]fakeGates{"dev": {}}})
	// Somewhere that is NOT the source repo, which is the whole premise:
	// the old code inherited exactly this.
	elsewhere := t.TempDir()
	t.Chdir(elsewhere)
	if out, err := f.run(t, "dev"); err != nil {
		t.Fatalf("upgradeRunPlan: %v\n%s", err, out)
	}
	if got := f.probe.Dir["gates dev"]; got != f.repo {
		t.Errorf("`posse gates dev` ran in %q, want the source repo %q — the cwd of this reading is the verb's, not the delegate's (ADR 0092 D4 amendment 2)", got, f.repo)
	}
	// version and status are left on the inherited cwd deliberately:
	// neither reads the cwd's hooks, and pinning it says so rather than
	// leaving the next reader to wonder whether it was an oversight.
	for _, verb := range []string{"version", "status"} {
		if got, ok := f.probe.Dir[verb]; !ok || got != "" {
			t.Errorf("`posse %s` ran with cwd %q (recorded=%v), want the inherited one — only the gates readings are placed", verb, got, ok)
		}
	}
}

// And the probe itself really changes directory, which is the half a stub
// cannot show. Nothing is planted: `pwd` is a binary that is already on the
// box and whose whole output is the answer.
//
// NO t.Parallel: the second half reads this process's own cwd, and the test
// above chdirs it. Serial is the honest answer for a pin whose subject is
// where a child process starts.
func TestUpgradeProbeRunsTheBinaryInTheDirectoryItIsGiven(t *testing.T) {
	pwd, err := exec.LookPath("pwd")
	if err != nil {
		t.Skipf("no pwd(1) on this box: %v", err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	code, out := upgradeProbe(dir, pwd)
	if code != 0 {
		t.Fatalf("pwd in %s exited %d: %s", dir, code, out)
	}
	if got := strings.TrimSpace(out); got != dir {
		t.Errorf("upgradeProbe ran in %q, want %q", got, dir)
	}
	// An empty dir inherits this process's cwd, which is what the version
	// and status readings pass.
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	here, err = filepath.EvalSymlinks(here)
	if err != nil {
		t.Fatal(err)
	}
	if _, out := upgradeProbe("", pwd); strings.TrimSpace(out) != here {
		t.Errorf("an empty dir ran in %q, want this process's cwd %q", strings.TrimSpace(out), here)
	}
}

// ─── D4 amendment 3 · the hook wall judged by behaviour ──────────────────────
//
// The three readings the amendment distinguishes, and the whole of why the
// mark exists:
//
//	new-red         the new binary answers non-zero where the old answered
//	                0 — this binary's own render stopped refusing. Roll back.
//	both-red-same   both binaries answer the same non-zero code — a box
//	                property (a broken /bin/sh, an unwritable $TMPDIR). The
//	                fleet was already running with it: report, do not roll
//	                back.
//	old-unasked     the old binary has no such verb. Its exit 1 is `no such
//	                agent` and not a render verdict (MEASURED, see
//	                fakePosse.NoRenderProbe), so nothing excuses the new
//	                binary and it must answer 0.

func TestUpgradeRollsBackWhenTheNewBinarysRenderStopsRefusing(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Render: 0},
		fakePosse{Render: 1})
	out, err := f.run(t)
	if err == nil {
		t.Fatalf("a binary whose own hook render no longer refuses was left installed:\n%s", out)
	}
	if !strings.Contains(out, "render-probe` exited 1") {
		t.Errorf("the report does not name the render probe's answer:\n%s", out)
	}
	if !strings.Contains(out, "no longer refuses") {
		t.Errorf("the report does not say what exit 1 means:\n%s", out)
	}
	f.rolledBack(t)
}

// Exit 2 is "could not ask", and through the NEW binary alone it is still a
// rollback: an install nobody could judge on ADR 0023's behaviour half is an
// install this gate did not clear.
func TestUpgradeRollsBackWhenOnlyTheNewBinaryCannotBeAsked(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Render: 0},
		fakePosse{Render: 2})
	out, err := f.run(t)
	if err == nil {
		t.Fatalf("an install whose behaviour probe could not be taken was left in place:\n%s", out)
	}
	if !strings.Contains(out, "could not be taken at all") {
		t.Errorf("the report does not say the probe could not be taken:\n%s", out)
	}
	f.rolledBack(t)
}

// both-red-same. A box property is reported and is not a reason to roll
// back — the fleet has been running on the old binary with the same answer.
func TestUpgradeDoesNotRollBackWhenBothBinariesGiveTheSameRenderAnswer(t *testing.T) {
	for _, code := range []int{1, 2} {
		t.Run(fmt.Sprintf("exit%d", code), func(t *testing.T) {
			f := newD4Fixture(t,
				fakePosse{Stamp: "aaaaaaa", Render: code},
				fakePosse{Render: code})
			out, err := f.run(t)
			if err != nil {
				t.Fatalf("an answer both binaries give rolled the install back: %v\n%s", err, out)
			}
			if !strings.Contains(out, fmt.Sprintf("render-probe` exits %d through BOTH binaries", code)) {
				t.Errorf("the report does not say the answer is a property of the box:\n%s", out)
			}
			if !strings.Contains(out, "not of the install") {
				t.Errorf("the report does not say the install is not implicated:\n%s", out)
			}
			if body, _ := os.ReadFile(f.target); !bytes.Equal(body, f.next.bytes()) {
				t.Errorf("the new binary is not in place: %q", body)
			}
		})
	}
}

// And the note is not standing furniture: a clean probe on both sides says
// nothing.
func TestUpgradeSaysNothingAboutTheRenderProbeWhenBothSidesAnswerZero(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{})
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("a clean render probe rolled the install back: %v\n%s", err, out)
	}
	if strings.Contains(out, "BOTH binaries") {
		t.Errorf("the report carries the box-property note over a clean probe:\n%s", out)
	}
	// The reading is still printed in the before/after lines, because a
	// delegate reading the block has to see that it was taken.
	if n := strings.Count(out, "render-probe 0"); n < 2 {
		t.Errorf("the before and after lines do not both carry the probe's answer (%d):\n%s", n, out)
	}
}

// old-unasked, and the reading this whole mechanism turns on. The OLD binary
// here is one from before the verb existed: it exits 1, which is also the
// verb's own code for "a render does not refuse". Were the exit code the
// reading, this fixture would be both-red-same and a NEW binary whose render
// really had stopped refusing would have been excused.
func TestUpgradeDoesNotExcuseANewRedWithAnOldBinaryThatLacksTheVerb(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", NoRenderProbe: true},
		fakePosse{Render: 1})
	out, err := f.run(t)
	if err == nil {
		t.Fatalf("exit 1 from a binary WITHOUT the verb excused a real render failure:\n%s", out)
	}
	if !strings.Contains(out, "did not have the verb, so nothing excuses it") {
		t.Errorf("the report reads the old binary's exit 1 as a render verdict:\n%s", out)
	}
	if !strings.Contains(out, "NOT ASKED") {
		t.Errorf("the before line does not say the old binary was not asked:\n%s", out)
	}
	f.rolledBack(t)
}

// The same old binary with a GOOD new one installs cleanly — the first
// upgrade after this verb lands is exactly this reading, and a gate that
// refused it would make the verb unlandable.
func TestUpgradeStandsWhenTheOldBinaryLacksTheVerbAndTheNewOneAnswersZero(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", NoRenderProbe: true},
		fakePosse{Render: 0})
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("the first install of the verb rolled itself back: %v\n%s", err, out)
	}
	if body, _ := os.ReadFile(f.target); !bytes.Equal(body, f.next.bytes()) {
		t.Errorf("the new binary is not in place: %q", body)
	}
}

// A NEW binary that answers without the mark cannot be judged, and this is
// the runtime half of the pin that keeps the verb in the binary: a release
// that dropped it reads here as "not asked" and rolls back.
func TestUpgradeRollsBackWhenTheNewBinaryHasNoRenderProbeVerb(t *testing.T) {
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Render: 0},
		fakePosse{NoRenderProbe: true})
	out, err := f.run(t)
	if err == nil {
		t.Fatalf("a new binary without the verb was left installed:\n%s", out)
	}
	if !strings.Contains(out, "did not answer through the new binary") {
		t.Errorf("the report does not say the verb did not answer:\n%s", out)
	}
	f.rolledBack(t)
}

// The reading is taken in the source repo and names it as the verb's
// argument too — amendment 2's defect one verb along: a default resolved
// inside the probed binary could differ from the one resolved here.
func TestUpgradeRenderProbeIsAskedAboutTheSourceRepo(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{})
	if _, err := f.run(t); err != nil {
		t.Fatalf("upgradeRunPlan: %v", err)
	}
	key := "gates render-probe " + f.repo
	if got, ok := f.probe.Dir[key]; !ok || got != f.repo {
		t.Errorf("the render probe ran as %q in %q, want it asked about %s and run there too:\n%v",
			key, got, f.repo, f.probe.Dir)
	}
}

// upgradeRenderAsked reads the MARK and not the exit code, and it reads it
// off any line of the output — the verb prints its reason line second on an
// exit 1, and a `could not ask` line is the whole of an exit 2.
func TestUpgradeRenderAskedReadsTheMarkAndNotTheExitCode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		out  string
		want bool
	}{
		{"the verb's clean line", RenderProbeMark + " ~/src/posse · pre-push refuses · prepare-commit-msg refuses\n", true},
		{"the verb's could-not-ask line", RenderProbeMark + " could not ask — /bin/sh is not there\n", true},
		{"a binary without the verb", "posse: no such agent: render-probe (looked in ~/homes/x/agents)\n", false},
		{"a leading blank line", "\n  " + RenderProbeMark + " ~/x · pre-push refuses · prepare-commit-msg refuses\n", true},
		{"silence", "", false},
		{"the mark in the middle of a sentence", "nothing here mentions render-probe: at the start of a line\n", false},
	} {
		if got := upgradeRenderAsked(tc.out); got != tc.want {
			t.Errorf("%s: upgradeRenderAsked = %v, want %v", tc.name, got, tc.want)
		}
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
	// One REFUSED line and one ✗ row. The ✓, ⚖ and ⊘ rows are each a
	// verdict that is NOT "a gate the wall does not hold", and the DEGRADED
	// HEADER is a summary of the ✗ rows beneath it rather than a row of its
	// own (ADR 0092 D4 amendment 2).
	if got, want := upgradeGatesRed(report), 2; got != want {
		t.Errorf("upgradeGatesRed = %d, want %d — over:\n%s", got, want, report)
	}
	if got := upgradeGatesRed("  claude @ shims/standard: RATIFIED DEGRADATION\n"); got != 0 {
		t.Errorf("RATIFIED DEGRADATION counts as %d not-held row(s), want 0 — it is a difference the promoted constitution already waived, standing on every reading", got)
	}
	// The header alone, with nothing under it — which Parity.String cannot
	// actually produce (the word is written exactly when there is at least
	// one ✗ row) and which is therefore the cleanest statement of the
	// claim: the header counts nothing by itself.
	if got := upgradeGatesRed("  codex @ shims/standard: DEGRADED\n"); got != 0 {
		t.Errorf("the parity header counts as %d not-held row(s) on its own, want 0 — it summarizes the ✗ rows below it, and counting both counts one wall twice", got)
	}
	// And the word is not simply dropped: the multiplexer line is a real
	// refusal of ADR 0076's that opens with it and is nobody's header.
	mux := "multiplexer: DEGRADED — herdr has no detection manifest for argv0 \"zsh\", so posse will not dispatch here\n"
	if got := upgradeGatesRed(mux); got != 1 {
		t.Errorf("a DEGRADED line that is not a parity header counts as %d, want 1 — the exclusion is the header's SHAPE, not the word:\n%s", got, mux)
	}
}

// ADR 0092 D4 amendment 2's other half: the L3 hook rows come out of the
// count. `posse gates <p>` asks ADR 0023's IDENTITY question of its cwd's
// hooks — byte-for-byte against this binary's own render — so a landing that
// changes the render makes those rows appear the instant the new binary is
// in place, in every repo posse guards. That is G12's situation one surface
// over, and it is the install's follow-through rather than evidence about
// the binary.
//
// The fixture's L3 lines come from l3DegradeLine itself, not from a string
// this file invents, so a reword over there reds here rather than silently
// emptying the exclusion.
func TestUpgradeGatesRedLeavesOutTheL3HookRows(t *testing.T) {
	t.Parallel()
	l3 := func(slot string) string {
		return "    ✗ " + l3DegradeLine(slot, "/x/.git/hooks/"+slot, "this layer is not realized", "", l3Stale) + "\n"
	}
	// A report whose ONLY not-held rows are L3 rows counts zero — the
	// header included, or the exclusion would buy nothing.
	onlyL3 := "  claude @ shims/standard: DEGRADED\n" + l3("pre-push") + l3("prepare-commit-msg")
	if got := upgradeGatesRed(onlyL3); got != 0 {
		t.Errorf("upgradeGatesRed = %d over a report whose only not-held rows are L3 hook rows, want 0 — a render change makes those rows in every guarded repo, which is what D7 leaves to the operator:\n%s", got, onlyL3)
	}
	// And a non-L3 row in the same block still counts. This is the half
	// that says the exclusion did not turn the gates arm off.
	withOne := onlyL3 + "    ✗ Bash(bd admin:*) → not realized at this tier\n"
	if got := upgradeGatesRed(withOne); got != 1 {
		t.Errorf("upgradeGatesRed = %d, want 1 — the wall lost a gate that is not a hook, and that is exactly what this count exists for:\n%s", got, withOne)
	}
	// A forward gap is NOT an L3 hook row: it is about an employer's hook
	// this session would skip (ADR 0052 M4) and names no slot of posse's.
	gap := "    ✗ L3 managed hooks — ~/src/x/.githooks/commit-msg — managed hook commit-msg not forwarded\n"
	if got := upgradeGatesRed(gap); got != 1 {
		t.Errorf("a managed-hook forward gap counts as %d, want 1 — the exclusion is posse's own two slots, not every line that says L3:\n%s", got, gap)
	}
	// The ✓ half of the same probe mentions the slot too, and a realized
	// gate was never a not-held row to begin with.
	ok := "    ✓ Bash(git push:*)             → cooperative (L1 shim) + L3 pre-push hook (render probed, dispatch verified)\n"
	if got := upgradeGatesRed(ok); got != 0 {
		t.Errorf("a realized gate counts as %d not-held row(s), want 0:\n%s", got, ok)
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
	if got := upgradeCondSource("G12"); got != upgradeInstallSubject {
		t.Errorf("G12 reads as %v, want install-subject — a repo's git hook is a COPY of this binary's render, and D7 leaves the re-render to the operator (ADR 0092 D4 amendment 1)", got)
	}
	if got := upgradeCondSource("G7"); got != upgradeBinaryRendered {
		t.Errorf("G7 reads as %v, want binary-rendered", got)
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
