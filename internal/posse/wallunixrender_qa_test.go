//go:build !posse_arm2 && !posse_arm3

package posse

// The RENDER half of ranger-base-6rndn's pins: the socket-wall step is on
// every bwrap seat's line, in both spellings, in the one position where it
// wraps the runtime and nothing else. The LIVE half — an uncaged process
// binds a socket after the seat launched and the seat gets EACCES, with a
// control arm that connects — is wallunixlive_test.go.
//
// Arm 1, and driven by the MECHANISM rather than by this box's GOOS, so the
// Linux line is read from darwin too (renderWallOn's rule, and the reason
// wallUnixAvailableFor takes a mechanism).

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// TestQAWallUnixStepIsOnBothBwrapSpellingsInThatOnePosition is the argv pin
// the bead asks for. It asserts the POSITION and not merely the presence: the
// step has to sit after bwrap's own `--`, because that is what makes the
// domain cover the runtime and everything the seat forks from it, and a step
// typed anywhere in FRONT of bwrap would apply a Landlock domain to the
// process that is about to call mount(2) — which a filesystem domain forbids
// (MEASURED U9/U10, docs/adr/0080-resolve-unix.probe.sh), so the seat would
// not open at all.
func TestQAWallUnixStepIsOnBothBwrapSpellingsInThatOnePosition(t *testing.T) {
	restore := wallUnixABI
	wallUnixABI = func() int { return 10 }
	t.Cleanup(func() { wallUnixABI = restore })

	self := "/opt/posse/bin/posse"
	step := WallUnixPrefix(self)
	if want := " " + WallUnixVerb + " -- "; !strings.HasSuffix(step, want) {
		t.Fatalf("WallUnixPrefix = %q, and it must end %q: what follows the step is the runtime command, and a step that did not close its own options with `--` would eat the runtime's flags", step, want)
	}
	if !strings.Contains(step, shellQuote(self)) {
		t.Errorf("WallUnixPrefix does not quote the binary it is handed (%q) — a home or a prefix with a space in it would split into two words, and a bare name on the line is one a gates-dir shim answers (BwrapPrefix's reason)", step)
	}

	args := "/h/state/gates/developer/bwrap.args"
	launch := WallWatchPrefix(self, "/h/state/gates/developer/bwrap.mounts", "developer", "/h/wt", "/usr/bin/bwrap", args) + step
	if !strings.HasSuffix(launch, step) {
		t.Fatalf("the launch line does not end with the socket-wall step:\n %q\nwant it to end %q — the step wraps the runtime command, so it is the last thing on the line", launch, step)
	}
	bwrapTail := "--args 3 -- "
	if head := strings.TrimSuffix(launch, step); !strings.HasSuffix(head, bwrapTail) {
		t.Errorf("the launch line puts the socket-wall step somewhere other than straight after bwrap's own %q:\n %q\nA step in FRONT of bwrap applies a filesystem Landlock domain to the process that then calls mount(2), which is refused (probe U9/U10) — the seat would not open at all", bwrapTail, launch)
	}
	// And NOT on `Prefix`. That is the probes' spelling, it carries no
	// launch-only wrapper (the rule the wall-watch sidecar already follows),
	// and the probes' subject is the FILE wall this layer does not touch. On
	// both, the witness probe gained a second binary to resolve and a new way
	// to abstain: it read `NOT WITNESSED … never printed its marker`, which
	// ADR 0086 D5 reads as an abstention rather than a pass.
	if p := BwrapPrefix("/usr/bin/bwrap", args); strings.Contains(p, WallUnixVerb) {
		t.Errorf("BwrapPrefix carries the socket-wall step (%q) — probes type that line, and a probe that has to resolve a second binary has one more way to measure nothing", p)
	}
}

// TestQAWallUnixStepIsOnTheLaunchLineAndNotOnTheProbeLine reads the same
// split off the RENDER rather than off the two prefix functions, so the pin
// above cannot pass over a render that wired them the other way round.
func TestQAWallUnixStepIsOnTheLaunchLineAndNotOnTheProbeLine(t *testing.T) {
	// No t.Parallel: wallUnixABI and wallWatchBin are package-level vars.
	restoreABI, restoreBin := wallUnixABI, wallWatchBin
	wallUnixABI = func() int { return 10 }
	wallWatchBin = func() (string, error) { return "/opt/posse", nil }
	t.Cleanup(func() { wallUnixABI, wallWatchBin = restoreABI, restoreBin })

	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	w, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatalf("the bwrap wall does not render for an ordinary seat: %v", err)
	}
	step := WallUnixPrefix("/opt/posse")
	if !strings.HasSuffix(w.Launch, step) {
		t.Errorf("WallRender.Launch does not end with the socket-wall step:\n %q\nwant it to end %q", w.Launch, step)
	}
	if strings.Contains(w.Prefix, WallUnixVerb) {
		t.Errorf("WallRender.Prefix carries the socket-wall step: %q — probes type it, and they run no watcher and need no socket wall", w.Prefix)
	}
}

// TestQAWallUnixStepIsRenderedForBwrapAndNotForSeatbelt holds the other half
// of the same sentence: the step is scoped to the mechanism that has a line
// to carry it. The darwin half is its own bead and its own mechanism, and a
// Landlock word typed into an SBPL launch would be a wall posse reports and
// the kernel never heard of.
func TestQAWallUnixStepIsRenderedForBwrapAndNotForSeatbelt(t *testing.T) {
	restore := wallUnixABI
	t.Cleanup(func() { wallUnixABI = restore })

	for _, c := range []struct {
		name   string
		mech   string
		abi    int
		step   bool // the step is typed (signal scope, ABI 6+)
		socket bool // and carries the socket wall (ABI 9+)
	}{
		{"bwrap, ABI 10 — the D1 host", WallBwrap, 10, true, true},
		{"bwrap, ABI exactly 9 — the version that added RESOLVE_UNIX", WallBwrap, WallUnixABIMin, true, true},
		{"bwrap, ABI 8 — RESOLVE_UNIX refused, SCOPE_SIGNAL still there", WallBwrap, 8, true, false},
		{"bwrap, ABI exactly 6 — the version that added SCOPE_SIGNAL", WallBwrap, WallUnixSignalABIMin, true, false},
		{"bwrap, ABI 5 — neither", WallBwrap, WallUnixSignalABIMin - 1, false, false},
		{"bwrap, no Landlock syscall at all", WallBwrap, -38, false, false},
		{"sandbox-exec, whatever the ABI says", WallSandboxExec, 10, false, false},
	} {
		wallUnixABI = func() int { return c.abi }
		if got := wallUnixAvailableFor(c.mech); got != c.step {
			t.Errorf("%s: wallUnixAvailableFor(%q) = %v, want %v — the step carries the signal scope from ABI %d, so a kernel that has it and not RESOLVE_UNIX must still get the step (ranger-base-e5un1)", c.name, c.mech, got, c.step, WallUnixSignalABIMin)
		}
		// And the report says which, in both directions: the degraded arm is
		// the one that must never be silence (bead ranger-base-6rndn: "make
		// posse gates and the wall-watch say DEGRADED, never silent").
		d := WallUnixDegraded(c.mech)
		switch {
		case c.mech != WallBwrap && d != "":
			t.Errorf("%s: the wall report calls a seatbelt line degraded (%q) — there is nothing degraded about a mechanism this layer was never part of", c.name, d)
		case c.mech == WallBwrap && c.socket && d != "":
			t.Errorf("%s: the layer is up and the report still says %q", c.name, d)
		case c.mech == WallBwrap && !c.socket && d == "":
			t.Errorf("%s: no socket wall on a bwrap line and the report says NOTHING. A seat whose socket wall is absent can connect(2) to any host pathname socket in its view (ranger-base-l2gk9), and silence is how that reads as walled", c.name)
		case c.mech == WallBwrap && !c.socket && !strings.Contains(d, "DEGRADED"):
			t.Errorf("%s: the report's sentence does not carry the word DEGRADED, which is what an operator scans for: %q", c.name, d)
		}
		// The degraded sentence also says which way the SIGNAL half went —
		// up on 6-8, absent below — so neither reads as the other.
		if c.mech == WallBwrap && !c.socket {
			up := strings.Contains(d, "signal scope IS up")
			if up != c.step {
				t.Errorf("%s: the degraded sentence says the signal scope is up = %v, and the step that applies it is typed = %v:\n  %q", c.name, up, c.step, d)
			}
		}
	}
}

// TestQAWallUnixRulesetScopesSignalAtEveryABITheStepIsTypedFor is
// ranger-base-e5un1's ruleset pin: the domain the step applies carries
// LANDLOCK_SCOPE_SIGNAL on every arm, and RESOLVE_UNIX only from ABI 9. The
// Linux file converts this struct into the syscall's argument rather than
// copying its fields, so dropping `Scoped` from the plan is dropping it from
// the kernel's ruleset, and reds here; the live pin
// (TestLiveWallUnixScopesSignalsToTheSeat) reads the kernel's side.
func TestQAWallUnixRulesetScopesSignalAtEveryABITheStepIsTypedFor(t *testing.T) {
	t.Parallel()
	if landlockScopeSignal != 0x2 {
		t.Fatalf("landlockScopeSignal = %#x, and linux/landlock.h's LANDLOCK_SCOPE_SIGNAL is 0x2 (x/sys v0.47.0 agrees)", landlockScopeSignal)
	}
	for abi := WallUnixSignalABIMin; abi <= 12; abi++ {
		r := wallUnixRuleset(abi)
		if r.Scoped&landlockScopeSignal == 0 {
			t.Errorf("ABI %d: the wall-unix ruleset's Scoped is %#x, without LANDLOCK_SCOPE_SIGNAL — a seat on the host's pid namespace (ADR 0080 D17) can then signal the launcher and every other seat (ranger-base-e5un1)", abi, r.Scoped)
		}
		if r.Scoped != landlockScopeSignal {
			t.Errorf("ABI %d: Scoped = %#x, and the decision (ranger-base-zqtoi) scopes SIGNAL and nothing else — LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET is an open question for the ADR, not a default", abi, r.Scoped)
		}
		if r.Access_net != 0 {
			t.Errorf("ABI %d: Access_net = %#x, and this layer handles no network right", abi, r.Access_net)
		}
		want := uint64(0)
		if abi >= WallUnixABIMin {
			want = landlockAccessFSResolveUnix
		}
		if r.Access_fs != want {
			t.Errorf("ABI %d: Access_fs = %#x, want %#x — RESOLVE_UNIX from ABI %d and nothing below it, where the kernel refuses the ruleset outright and the seat would not open", abi, r.Access_fs, want, WallUnixABIMin)
		}
	}
}

// TestQAWallUnixStepRefusesALineWithNoRuntime covers the one way the step can
// be reached wrongly: typed with nothing after its `--`. It must refuse and
// not exec something it guessed, because the thing it would be guessing at is
// what this seat runs.
func TestQAWallUnixStepRefusesALineWithNoRuntime(t *testing.T) {
	t.Parallel()
	for _, argv := range [][]string{
		{"posse", WallUnixVerb},
		{"posse", WallUnixVerb, "--"},
	} {
		err := RunWallUnix(argv)
		if err == nil {
			t.Fatalf("RunWallUnix(%q) returned nil — on success this call does not return at all (it becomes the runtime), so a nil here is a seat that opened with no runtime and no wall", argv)
		}
		if !strings.Contains(err.Error(), "nothing to exec") {
			t.Errorf("RunWallUnix(%q) = %v, and the refusal should say the line carried no command", argv, err)
		}
	}
}

// TestQAWallUnixStepRefusesBelowTheSignalABIFloor is the pin ranger-base-i95x2
// found missing (its finding 1, filed as ranger-base-c7fwc): the step re-reads
// the ABI ITSELF and refuses below WallUnixSignalABIMin, rather than applying a
// domain this kernel cannot carry and exec'ing the runtime anyway.
//
// WHY IT IS A SEPARATE PIN from the render's ABI pin
// (TestQALandedCheckJudgeTypesNoSocketWallStepBelowTheABIItNeeds). That one
// asserts the render does not TYPE the step below the floor. This asserts the
// step refuses when it is typed anyway — the host-changed-under-the-render
// case, which is the only reason the read is repeated inside the step instead
// of being carried on the line. MEASURED 2026-11-15 on D1 (Linux
// 7.2.15-1-omarchy, Landlock ABI 10): before this pin, changing the guard to
// `if false && abi < WallUnixSignalABIMin` left the whole wall-unix pin set
// green — the one survivor of six mutants aimed at ranger-base-e5un1's work.
//
// WHY THE APPLY IS STUBBED AS WELL AS THE ABI: restrictWallUnix's own comment
// has it. Short version — without that stub a guard-deleting mutant does not
// red, it EXECS, because an ABI 6+ kernel accepts the sub-floor plan and the
// test binary becomes the runtime.
//
// WHY THE COMMAND IS `true`: it resolves on every PATH, so the LookPath arm
// ABOVE the floor check cannot be what refuses and the refusal read here is
// the floor's.
//
// WHAT IS BEING DEFENDED, precisely. Not a security boundary: on a REAL
// sub-floor kernel the ruleset is rejected and the apply refuses one line
// later, so the seat does not open unwalled either way (DERIVED from the
// kernel's trailing-non-zero-field behaviour, not measured — this box is ABI
// 10). What has no other holder is the guard's WORDING and its D4 claim, so
// this pin reads the sentence.
//
// SERIAL, no t.Parallel: wallUnixABI and restrictWallUnix are package-level
// vars.
func TestQAWallUnixStepRefusesBelowTheSignalABIFloor(t *testing.T) {
	restoreABI, restoreApply := wallUnixABI, restrictWallUnix
	t.Cleanup(func() { wallUnixABI, restrictWallUnix = restoreABI, restoreApply })

	applied := 0
	restrictWallUnix = func(int) error {
		applied++
		return errors.New("the apply stub: every row of this pin is below the floor, so the step must refuse before it reaches the apply")
	}

	for _, c := range []struct {
		name string
		abi  int
	}{
		{"one below the floor", WallUnixSignalABIMin - 1},
		{"ABI 1, Landlock's first version", 1},
		{"the syscall absent (-ENOSYS)", -38},
	} {
		t.Run(c.name, func(t *testing.T) {
			applied = 0
			wallUnixABI = func() int { return c.abi }
			err := RunWallUnix([]string{"posse", WallUnixVerb, "--", "true"})
			if err == nil {
				t.Fatalf("ABI %d: RunWallUnix returned nil — on success this call does not return at all (it becomes the runtime), so a nil here is a seat that opened below the floor (ADR 0080 D4)", c.abi)
			}
			if applied != 0 {
				t.Errorf("ABI %d: the apply was called %d time(s), so the floor check did not refuse — the step read an ABI below %d and went on to build a ruleset for it: %v",
					c.abi, applied, WallUnixSignalABIMin, err)
			}
			said := err.Error()
			for _, want := range []string{
				WallUnixVerb,
				"the step's floor is " + strconv.Itoa(WallUnixSignalABIMin),
				"LANDLOCK_SCOPE_SIGNAL",
				"ADR 0080 D4",
				wallUnixABIText(),
			} {
				if !strings.Contains(said, want) {
					t.Errorf("ABI %d: the refusal does not name %q, and the wording is the whole of what this guard delivers (see the pin's comment): %s", c.abi, want, said)
				}
			}
		})
	}
}
