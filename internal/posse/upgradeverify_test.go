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

	// Classes OVERLAYS what `posse upgrade --classes` answers (ADR 0092 D4
	// rules 4-6): the fake prints THIS binary's own table with these rows
	// replaced, plus any row here that the table does not name. So the zero
	// value is "the same partition the judge compiled in", which is every
	// ordinary install, and a fixture asks for a crossing or a new row by
	// naming only that row.
	Classes map[string]string
	// NoClassesFlag is a binary from BEFORE rule 4. MEASURED 2026-11-21 at
	// eb3f34fc (UpgradeClassMark's header): nothing on stdout, `unknown
	// flag: --classes` on stderr, exit 1 — the same 1 a refusal of anything
	// else gives, which is why "asked" is the mark and this flag is how the
	// pins exercise the reading rule 5 trips on.
	NoClassesFlag bool
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
	// Sorted for the GATES rows' reason, which is the whole of this
	// marshaller's header: every rollback pin compares bytes from one call
	// to bytes from another.
	for _, row := range slices.Sorted(maps.Keys(f.Classes)) {
		fmt.Fprintf(&b, "CLASS %s %s\n", row, f.Classes[row])
	}
	if f.NoClassesFlag {
		b.WriteString("NO-CLASSES-FLAG\n")
	}
	return []byte(b.String())
}

// classLines is what this fake's `posse upgrade --classes` prints: the REAL
// renderer's answer for every row this binary's table names, with the
// fixture's overlay applied and its unknown rows appended.
//
// Through UpgradeClassLines rather than a format this file invents, so a pin
// that excuses a row excuses it over the bytes upgradeParseClasses really has
// to read — a landing that changed the line's shape would red here instead of
// silently emptying every class the reader sees.
func (f fakePosse) classLines() []string {
	out := make([]string, 0, len(upgradeConditionSource)+len(f.Classes))
	for _, ln := range UpgradeClassLines() {
		row := strings.Fields(ln)[1]
		if word, ok := f.Classes[row]; ok {
			ln = UpgradeClassMark + " " + row + " " + word
		}
		out = append(out, ln)
	}
	for _, row := range slices.Sorted(maps.Keys(f.Classes)) {
		if _, named := upgradeConditionSource[row]; !named {
			out = append(out, UpgradeClassMark+" "+row+" "+f.Classes[row])
		}
	}
	return out
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
		case strings.HasPrefix(ln, "CLASS "):
			p := strings.Fields(strings.TrimPrefix(ln, "CLASS "))
			if len(p) == 2 {
				if f.Classes == nil {
					f.Classes = map[string]string{}
				}
				f.Classes[p[0]] = p[1]
			}
		case ln == "NO-CLASSES-FLAG":
			f.NoClassesFlag = true
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
		// ADR 0092 D4 rule 4's answer, read by rule 5. A binary from before
		// the flag refuses it in cmd/posse's flag ladder, above everything
		// — no stdout, no lock, exit 1 (MEASURED, see NoClassesFlag).
		case len(argv) == 2 && argv[0] == "upgrade" && argv[1] == "--classes":
			if f.NoClassesFlag {
				return 1, "posse: unknown flag: --classes — usage: posse upgrade [<sha>]\n"
			}
			var b strings.Builder
			for _, ln := range f.classLines() {
				fmt.Fprintln(&b, ln)
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
	// readers, when set, installs the plan's two loop readers — the seam
	// ranger-base-r1zdr's report pins need. The report's last two loop lines
	// print only for a plan that carries a reader and upgradeResolve sets
	// neither, so without this hook a run through this fixture prints
	// neither line and the wiring that installs them is unwitnessed. Nil for
	// every pin that does not ask, which is the state they were all written
	// in: a nil reader prints nothing (upgradeWatchLine, upgradeBoxLine).
	readers func(*upgradePlan)
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
	if f.readers != nil {
		f.readers(&plan)
	}
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
	// version, status and rule 4's `upgrade --classes` are left on the
	// inherited cwd deliberately: none of them reads the cwd's hooks — the
	// third reads no file at all — and pinning it says so rather than
	// leaving the next reader to wonder whether it was an oversight.
	for _, verb := range []string{"version", "status", "upgrade --classes"} {
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

// ─── D4 rules 5-6 · a row the judge cannot name is classed by the target ─────
//
// The old-reader/new-writer case, and the defect it answers is MEASURED
// (ranger-base-i9fq8, 2026-11-21): the judge is the table compiled into the
// binary RUNNING the verb, which is the old one, so a G-row added inside the
// range is a row the judge cannot name — and the unknown-code rule classes
// it binary-rendered. 0e1cb6c0..65d90a5e rolled back on one LANE G18 row
// that ranger-base-rrgnc had added in that range and classed
// install-subject. The fix for such a crossing cannot be installed either,
// because every target that holds it is judged by the table that predates
// it, which is what makes this a wall rather than one bad install.
//
// So the writer describes its own data (rule 4's `--classes`) and the reader
// consults that description for exactly the rows it cannot name itself:
//
//	unknown + shop or install   not a trigger, and a note names the row
//	unknown + binary            trips, as today
//	unknown + no class named    trips, as today
//	unknown + did not answer    trips, as today
//	known                       the judge's class rules and the target's
//	                            answer is read for rule 6's WORDING alone
//
// WHY THIS IS NOT A CANDIDATE VOTING ON ITS OWN INSTALL: the target can only
// excuse a row whose existence is its own speech alone — a row it could have
// escaped by not printing at all — so its class there gives it no power it
// did not have. Every row the judge can name is still the judge's.

// upgradeNewRow is a condition row nothing in this tree prints and this
// binary's table therefore cannot name: the shape of a G-row a LATER build
// adds, which is the whole subject of rules 5 and 6. Safe to spell here
// because the census that requires every row to carry a class reads shipped
// sources only (TestQAEveryStatusConditionCodeHasAnUpgradeClass), so naming
// one in a test is not a claim that this package can print it.
func upgradeNewRow() GovCondition {
	return GovCondition{ID: "G19", Class: GovLane, Key: "a-later-builds-row", Detail: "a condition this binary has no class for"}
}

// Rule 5 whole, over the four readings it distinguishes. The pair is the
// claim, exactly as D4's third bullet's pair is: the same mechanism must
// stand on a row the target excuses and roll back on every other answer,
// including the two that are silences.
func TestUpgradeClassesAnUnknownRowByTheTargetsOwnAnswer(t *testing.T) {
	row := upgradeNewRow()
	for _, c := range []struct {
		name    string
		classes map[string]string
		noFlag  bool
		stands  bool
		says    []string
	}{
		// The row ranger-base-i9fq8 measured, in the shape that defect had:
		// the target knows it is the install's own footprint, and the judge
		// has never heard of it.
		{"the target classes it install", map[string]string{"G19": "install"}, false, true,
			[]string{"gained 1 LANE G19 row(s)", "the target classes G19 install", "this binary had no class for it"}},
		// Same rule, the other excusing word: a shop-state row added in the
		// range would otherwise roll back every upgrade from the day it
		// first appears.
		{"the target classes it shop", map[string]string{"G19": "shop"}, false, true,
			[]string{"gained 1 LANE G19 row(s)", "the target classes G19 shop", "this binary had no class for it"}},
		// The target says the row is its own rendering, which is the answer
		// that trips — and the one answer a binary wanting to escape the row
		// would never give.
		{"the target classes it binary", map[string]string{"G19": "binary"}, false, false,
			[]string{"no class for at all", "the target classes G19 binary itself"}},
		// It answered, and its partition does not carry the row it just
		// printed. Positive evidence only: an answer that names no class is
		// not an answer that says `shop`.
		{"the target answered and named no class for it", nil, false, false,
			[]string{"no class for at all", "does not name exactly one class for G19"}},
		// And the silence. A binary with no `--classes` flag at all cannot
		// describe anything, and rule 5 reads that as not asked — never off
		// the exit code, which is the same 1 for a refused flag and for a
		// render that does not refuse.
		{"the target has no --classes flag", nil, true, false,
			[]string{"no class for at all", "NOT ASKED"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newD4Fixture(t,
				fakePosse{Stamp: "aaaaaaa"},
				fakePosse{Conds: GovSet{row}, Classes: c.classes, NoClassesFlag: c.noFlag})
			out, err := f.run(t)
			if c.stands {
				if err != nil {
					t.Fatalf("a row this binary cannot name, which the TARGET excuses, rolled the install back — and the fix for such a crossing can then never be installed (ADR 0092 D4 rule 5):\n%v\n%s", err, out)
				}
				if body, _ := os.ReadFile(f.target); !bytes.Equal(body, f.next.bytes()) {
					t.Errorf("the new binary is not in place after a clean verify: %q", body)
				}
			} else {
				if err == nil {
					t.Fatalf("an unknown row the target does not excuse was left installed:\n%s", out)
				}
				f.rolledBack(t)
			}
			for _, say := range c.says {
				if !strings.Contains(out, say) {
					t.Errorf("the report does not say %q — rule 5's four readings are four different things to go and look at:\n%s", say, out)
				}
			}
		})
	}
}

// The note is not standing furniture either: an install that gained no such
// row says nothing about anybody's partition.
func TestUpgradeSaysNothingAboutTheTargetsClassesWithoutAnUnknownRow(t *testing.T) {
	standing := upgradeNewRow()
	f := newD4Fixture(t,
		fakePosse{Stamp: "aaaaaaa", Conds: GovSet{standing}, Classes: map[string]string{"G19": "install"}},
		fakePosse{Conds: GovSet{standing}, Classes: map[string]string{"G19": "install"}})
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("a row that was there before the swap rolled the install back: %v\n%s", err, out)
	}
	if strings.Contains(out, "had no class for it") {
		t.Errorf("the report carries rule 5's note over a row this install did not gain:\n%s", out)
	}
}

// Rule 5's other half and rule 6. For a row the judge CAN name, the judge's
// class rules — in both directions — and the only thing the target's answer
// buys is the line that says a reclassification takes effect one install
// late.
func TestUpgradeJudgesAKnownRowItselfAndSaysSoWhenTheTargetDisagrees(t *testing.T) {
	for _, c := range []struct {
		name    string
		cond    GovCondition
		classes map[string]string
		back    bool
		says    []string
		silent  []string
	}{
		// G7 is binary-rendered here and `install` in the target: it trips
		// under this binary's class, and the line names both so that nobody
		// has to recover the disagreement from two trees.
		{
			name:    "a binary row the target calls install",
			cond:    GovCondition{ID: "G7", Class: GovUrgent, Key: "arm-broken", Detail: "autostart_interval: \"5m\" is not an interval"},
			classes: map[string]string{"G7": "install"},
			back:    true,
			says: []string{
				"gained 1 URGENT G7 row(s)",
				"the target classes G7 install",
				"this binary classes it binary",
				"this binary's class is the one that judges",
				"the crossing is the operator's `make install`",
			},
		},
		// The same row with the two tables AGREEING: it still trips, and
		// the crossing sentence is not printed. A line reading "the target
		// classes G7 binary; this binary classes it binary" on every
		// ordinary rollback is a line the delegate learns to skip, and rule
		// 6's whole subject is a DISAGREEMENT.
		{
			name:    "a binary row the target also calls binary",
			cond:    GovCondition{ID: "G7", Class: GovUrgent, Key: "arm-broken", Detail: "autostart_via: \"cron\" is not a via"},
			classes: map[string]string{"G7": "binary"},
			back:    true,
			says:    []string{"gained 1 URGENT G7 row(s)"},
			silent:  []string{"the target classes G7", "the one that judges"},
		},
		// And the direction that must NOT make the target's answer matter:
		// a row this binary excludes stays excluded however the target
		// classes it. A reading that consulted `--classes` here would let a
		// new binary turn one of the judge's own exclusions into a rollback.
		{
			name:    "a shop row the target calls binary",
			cond:    GovCondition{ID: "G1", Class: GovLane, Key: "blocked:sess", Detail: "sess (claude) is blocked on an approval"},
			classes: map[string]string{"G1": "binary"},
			back:    false,
			silent:  []string{"the target classes G1", "G1 row(s)"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newD4Fixture(t,
				fakePosse{Stamp: "aaaaaaa"},
				fakePosse{Conds: GovSet{c.cond}, Classes: c.classes})
			out, err := f.run(t)
			if c.back {
				if err == nil {
					t.Fatalf("a row this binary classes binary-rendered was excused by the target's own answer — rule 5 reads that answer for rows the judge CANNOT name, and reading it here is the candidate voting on its own install:\n%s", out)
				}
				f.rolledBack(t)
			} else if err != nil {
				t.Fatalf("a row this binary excludes rolled the install back because the TARGET calls it binary-rendered: %v\n%s", err, out)
			}
			for _, say := range c.says {
				if !strings.Contains(out, say) {
					t.Errorf("the report does not say %q (ADR 0092 D4 rule 6):\n%s", say, out)
				}
			}
			for _, quiet := range c.silent {
				if strings.Contains(out, quiet) {
					t.Errorf("the report says %q over a row the judge excluded, so something read the target's answer for a row this binary names:\n%s", quiet, out)
				}
			}
		})
	}
}

// THE CROSSING PIN — ADR 0092 D4's promised companion, and the one this work
// exists for: for EVERY row this binary classifies, judge it with a table
// that predates the row, the after-reading gaining it, and the real table as
// the target's answer. The install must stand exactly when the row's real
// class is one of the two that excuse.
//
// WHAT IT PROTECTS, in process terms: the first delegated install after any
// bead that adds a G-row. Without it that install rolls back, the report
// blames a binary that did nothing, and the fix is uninstallable because
// every target that carries it is judged by the table that predates it
// (ranger-base-i9fq8, 2026-11-21).
//
// NO t.Parallel, and it mutates the package table: a judge whose table
// predates one row is exactly this binary's table minus that entry, and
// there is no other way to be that binary. Restored per row.
func TestUpgradeEveryRowCrossingIsClassedByTheTargetsAnswer(t *testing.T) {
	real := maps.Clone(upgradeConditionSource)
	if len(real) < 12 {
		t.Fatalf("upgradeConditionSource has %d row(s) — too few for this pin to be crossing anything", len(real))
	}
	// What a binary CARRYING the row prints: rule 4's answer over the real
	// table. Taken before the deletions below, because that is the table
	// the target compiled in.
	answer := map[string]string{}
	for row, class := range real {
		answer[row] = upgradeClassWord(class)
	}

	const stamp, to = "abcdef12", "abcdef1234567890"
	reading := func(status map[string]int, classes map[string]string, asked bool) upgradeReading {
		return upgradeReading{
			Ran: true, Stamp: stamp, StatusRan: true,
			Status:       status,
			Gates:        map[string]upgradeGatesReading{},
			Classes:      classes,
			ClassesAsked: asked,
			RenderAsked:  true,
		}
	}

	for _, row := range slices.Sorted(maps.Keys(real)) {
		t.Run(row, func(t *testing.T) {
			// The judge: this binary's table with the row deleted — the
			// binary that was installed before the bead that added it.
			delete(upgradeConditionSource, row)
			t.Cleanup(func() { upgradeConditionSource[row] = real[row] })

			before := reading(map[string]int{}, answer, true)
			after := reading(map[string]int{GovLane + " " + row: 1}, answer, true)

			why := upgradeVerdict(to, before, after)
			stands := len(why) == 0
			want := real[row] != upgradeBinaryRendered
			if stands != want {
				t.Fatalf("a crossing of %s (real class %s) %s, want it to %s:\n  %s",
					row, upgradeClassWord(real[row]),
					map[bool]string{true: "stood", false: "rolled the install back"}[stands],
					map[bool]string{true: "stand", false: "roll back"}[want],
					strings.Join(why, "\n  "))
			}
			if stands {
				note := upgradeClassNote(before, after)
				for _, say := range []string{row, answer[row], "this binary had no class for it"} {
					if !strings.Contains(note, say) {
						t.Errorf("the note over a stood crossing of %s does not say %q:\n%s", row, say, note)
					}
				}
			} else {
				if n := len(why); n != 1 {
					t.Fatalf("a crossing of %s produced %d reason(s), want the one row's: %v", row, n, why)
				}
				if !strings.Contains(why[0], "no class for at all") {
					t.Errorf("the reason does not say the judge cannot name %s:\n%s", row, why[0])
				}
				// Rule 6's line is for a row the judge CAN name, and
				// printing it here would name a class this binary does not
				// have.
				if strings.Contains(why[0], "the one that judges") {
					t.Errorf("the reason carries rule 6's crossing sentence over a row this binary has no class for:\n%s", why[0])
				}
			}

			// THE CONTROL, and it is what makes the arm above a measurement
			// of the ANSWER rather than of the row's absence: the same
			// reading with nothing asked trips for every row, including the
			// ones that just stood.
			if why := upgradeVerdict(to, before, reading(map[string]int{GovLane + " " + row: 1}, nil, false)); len(why) == 0 {
				t.Errorf("a crossing of %s stood with no `--classes` answer at all — then the class is not what judged it, and this pin measures nothing", row)
			}
		})
	}
}

// ─── the readers ─────────────────────────────────────────────────────────────

// upgradeParseClasses is the far side of a wire format, so what it must do
// with a line it cannot open is the behaviour — and both silences rule 5
// trips on are readings this function produces.
func TestUpgradeParseClassesReadsTheMarkedLinesAlone(t *testing.T) {
	t.Parallel()
	mark := UpgradeClassMark + " "
	for _, tc := range []struct {
		name  string
		out   string
		want  map[string]string
		asked bool
	}{
		{"the verb's own answer", strings.Join(UpgradeClassLines(), "\n") + "\n", nil, true},
		{"a binary without the flag", "posse: unknown flag: --classes — usage: posse upgrade [<sha>]\n", map[string]string{}, false},
		{"silence", "", map[string]string{}, false},
		{"one row", mark + "G19 install\n", map[string]string{"G19": "install"}, true},
		// Asked is the MARK and not the map: a build that answered and
		// named nothing this reader could use still answered, and a reader
		// that said "not asked" there would report the wrong one of rule
		// 5's four readings.
		{"a marked line with no class", mark + "G19\n", map[string]string{}, true},
		{"a marked line with a fourth field", mark + "G19 install (promoted)\n", map[string]string{}, true},
		// A row answered twice is dropped rather than resolved by order:
		// either order is right for one spelling and wrong for the other,
		// and the direction that costs something is excusing a row on an
		// answer that contradicts itself.
		{"a row answered twice", mark + "G19 shop\n" + mark + "G19 binary\n", map[string]string{}, true},
		{"a row answered twice, the other way", mark + "G19 binary\n" + mark + "G19 shop\n", map[string]string{}, true},
		{"a row answered three times", mark + "G19 shop\n" + mark + "G19 shop\n" + mark + "G19 shop\n", map[string]string{}, true},
		// The mark opens a LINE. A sentence that mentions it is prose from
		// a build whose header this reader was never taught.
		{"the mark inside a sentence", "nothing here opens with upgrade-class: at all\n", map[string]string{}, false},
		{"a leading blank line and one row", "\n" + mark + "G19 shop\n", map[string]string{"G19": "shop"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, asked := upgradeParseClasses(tc.out)
			if asked != tc.asked {
				t.Errorf("upgradeParseClasses(%q) asked=%v, want %v", tc.out, asked, tc.asked)
			}
			if tc.want == nil {
				// The verb's own answer: every row of this binary's table,
				// with this binary's word for it.
				if len(got) != len(upgradeConditionSource) {
					t.Fatalf("the reader found %d row(s) in the printer's own answer, want the table's %d: %v", len(got), len(upgradeConditionSource), got)
				}
				for row, class := range upgradeConditionSource {
					if got[row] != upgradeClassWord(class) {
						t.Errorf("the reader read %s as %q, want %q", row, got[row], upgradeClassWord(class))
					}
				}
				return
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("upgradeParseClasses(%q) = %v, want %v", tc.out, got, tc.want)
			}
		})
	}
}

// And the word test rule 5 turns on, over the three published words and a
// fourth nobody published: only `shop` and `install` excuse a row, which is
// upgradeClassWord's fail-closed default read from the far side.
func TestUpgradeClassExcusesOnlyTheTwoExcludingWords(t *testing.T) {
	t.Parallel()
	for word, want := range map[string]bool{
		"shop":    true,
		"install": true,
		"binary":  false,
		"":        false,
		"SHOP":    false,
		"later":   false,
	} {
		if got := upgradeClassExcuses(word); got != want {
			t.Errorf("upgradeClassExcuses(%q) = %v, want %v — a word this binary does not publish must trip, or a later build excuses a row on a token nobody agreed on", word, got, want)
		}
	}
}

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

// An unknown row name is BINARY-rendered rather than shop-state, which is
// the half upgradeCondSource adds over the table it reads. The direction is
// the whole point: that table is complete for this binary (the QA census pin
// says so), so a row the NEW binary printed and this one cannot name is a
// condition it invented.
//
// The four reasoned spot checks this test used to carry — G1, G7, G11, G12 —
// moved into upgradeClassValuePin below, where the same assertion is made
// over EVERY row instead of the four somebody thought of. Two hand-kept
// lists of classes is the folklore this file's header warns about.
func TestUpgradeUnknownConditionCodeIsTreatedAsBinaryRendered(t *testing.T) {
	t.Parallel()
	if got := upgradeCondSource("G404"); got != upgradeBinaryRendered {
		t.Errorf("a code with no class reads as %v, want binary-rendered — the safe answer is the one that rolls back", got)
	}
}

// upgradeClassValuePin is every row `posse status` can print with the class
// upgradeConditionSource must give it, spelled a second time HERE
// (ranger-base-nqaa1, finding 2).
//
// WHY A SECOND COPY IS THE MECHANISM AND NOT THE DEFECT. The census pin
// (TestQAEveryStatusConditionCodeHasAnUpgradeClass) asserts that every row
// HAS a class and no class names a dead row. It says nothing about WHICH,
// and the values are what the rollback is computed over — so until this
// table existed, changing one entry of upgradeConditionSource was a change no
// test could see. MEASURED 2026-11-21 at tree 3c16c00c by the verify of
// ranger-base-rrgnc: `"G18": upgradeInstallSubject` edited to
// `upgradeBinaryRendered` exercised 125 tests under
// `-run '(Upgrade|upgradeCond|Install|Condition|G18)'` and red NONE. G18
// appears after EVERY install with a live dispatch loop, by construction — so
// that one token, which nothing asserted, rolls back every good install on
// this box, every time, and the next reader's first hypothesis is that the
// new binary is broken.
//
// The copy cannot drift, because the test below is bidirectional: a row in
// the source table with no row here fails, and a row here the source table
// does not name fails. So a G-row added tomorrow arrives with a class that
// something asserts, instead of a class somebody has to remember to assert.
// What it cannot force is the THINKING — a red here can be answered by
// copying the new value across — which is why `why` exists and why a row
// whose wrong value has a named cost carries it.
var upgradeClassValuePin = []struct {
	row  string
	want upgradeCondClass
	why  string
}{
	{"G1", upgradeShopState, "herdr agent status: seats block and unblock while the suite runs"},
	{"G2", upgradeShopState, ""},
	{"G3", upgradeShopState, ""},
	{"G4", upgradeShopState, ""},
	{"G5", upgradeShopState, ""},
	{"G6", upgradeShopState, ""},
	{"G7", upgradeBinaryRendered, "the arm-broken keys refuse `autostart_interval:`/`autostart_via:` through THIS binary's grammar, so a narrowed grammar would disarm the fleet's only instrument silently — a code is as binary-rendered as its most binary predicate (ADR 0092 D4, the code the ADR did not name)"},
	{"G8", upgradeShopState, ""},
	{"G9", upgradeShopState, ""},
	{"G10", upgradeShopState, ""},
	{"G11", upgradeInstallSubject, "it counts commits from the installed stamp to the source tip, and this verb moves that stamp: as binary-rendered it would roll back on its own footprint"},
	{"G12", upgradeInstallSubject, "a repo's git hook is a COPY of this binary's render and D7 leaves the re-render to the operator, so a G12 row after the swap is the designed state: as binary-rendered the verb refused exactly the landings that change the wall (ADR 0092 D4 amendment 1, ranger-base-qfhph)"},
	{"G13", upgradeShopState, ""},
	{"G14", upgradeShopState, ""},
	{"G15", upgradeShopState, ""},
	{"G16", upgradeShopState, ""},
	{"G17", upgradeBinaryRendered, "a static map judged by this binary's own cage resolver: nothing but the swap can have changed the answer"},
	{"G18", upgradeInstallSubject, "the rename CREATES this row by construction — rename(2) leaves the live dispatch loop on the old inode — so as binary-rendered it rolls back every good install on a box with a loop running (ranger-base-rrgnc, watchimage.go)"},
	{GovCondition{}.Row(), upgradeShopState, "the three pulse-era carry-overs: git, herdr and the clock, all three moving on their own"},
}

// The value pin itself, and the two completeness arms that keep the table
// above from drifting from the one it reads.
func TestUpgradeEveryConditionClassIsPinnedByValue(t *testing.T) {
	t.Parallel()
	name := func(c upgradeCondClass) string {
		switch c {
		case upgradeShopState:
			return "shop-state (excluded: its store moves on its own)"
		case upgradeBinaryRendered:
			return "binary-rendered (ROLLS BACK)"
		case upgradeInstallSubject:
			return "install-subject (excluded: this verb moves the row itself)"
		}
		return fmt.Sprintf("upgradeCondClass(%d), which is not a class this pin knows — add it here", int(c))
	}

	pinned := map[string]bool{}
	for _, p := range upgradeClassValuePin {
		if pinned[p.row] {
			t.Errorf("%s is pinned twice in upgradeClassValuePin", p.row)
			continue
		}
		pinned[p.row] = true
		got, ok := upgradeConditionSource[p.row]
		if !ok {
			t.Errorf("upgradeConditionSource has no class for %s, which this pin says is %s — either the row was deleted (drop its row here and the measurement table's) or the entry was lost", p.row, name(p.want))
			continue
		}
		if got == p.want {
			continue
		}
		msg := fmt.Sprintf("upgradeConditionSource[%q] = %s, want %s", p.row, name(got), name(p.want))
		if p.why != "" {
			msg += "\n  why: " + p.why
		}
		t.Errorf("%s\n  The class decides whether this row APPEARING between the two readings undoes the install (ADR 0092 D4). Do not answer this red by copying the new value across: read upgradeverify.go's measurement table, and if the row's store of record really changed, change the table's row and this one together and say so on a bead.", msg)
	}

	// The other direction: a row with a class and no value pin is the hole
	// this table exists to close, so it has to fail HERE rather than wait to
	// be noticed by a mutant somebody happened to run.
	var unpinned []string
	for row := range upgradeConditionSource {
		if !pinned[row] {
			unpinned = append(unpinned, row)
		}
	}
	slices.Sort(unpinned)
	if len(unpinned) > 0 {
		t.Errorf(`%d row(s) carry a class that nothing asserts the VALUE of: %s

Add each to upgradeClassValuePin above, with the class the measurement table in
upgradeverify.go gives it and — if the wrong value has a named cost — what that cost
is. The census pin next door only requires that a row HAS a class; a row whose value
no test reads is a token that can be edited green (ranger-base-nqaa1, finding 2).`,
			len(unpinned), strings.Join(unpinned, ", "))
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
