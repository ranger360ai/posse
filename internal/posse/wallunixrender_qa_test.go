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
		name string
		mech string
		abi  int
		want bool
	}{
		{"bwrap, ABI 10 — the D1 host", WallBwrap, 10, true},
		{"bwrap, ABI exactly 9 — the version that added RESOLVE_UNIX", WallBwrap, WallUnixABIMin, true},
		{"bwrap, ABI 8 — handled_access_fs would be refused outright", WallBwrap, 8, false},
		{"bwrap, no Landlock syscall at all", WallBwrap, -38, false},
		{"sandbox-exec, whatever the ABI says", WallSandboxExec, 10, false},
	} {
		wallUnixABI = func() int { return c.abi }
		if got := wallUnixAvailableFor(c.mech); got != c.want {
			t.Errorf("%s: wallUnixAvailableFor(%q) = %v, want %v", c.name, c.mech, got, c.want)
		}
		// And the report says which, in both directions: the degraded arm is
		// the one that must never be silence (bead ranger-base-6rndn: "make
		// posse gates and the wall-watch say DEGRADED, never silent").
		d := WallUnixDegraded(c.mech)
		switch {
		case c.mech != WallBwrap && d != "":
			t.Errorf("%s: the wall report calls a seatbelt line degraded (%q) — there is nothing degraded about a mechanism this layer was never part of", c.name, d)
		case c.mech == WallBwrap && c.want && d != "":
			t.Errorf("%s: the layer is up and the report still says %q", c.name, d)
		case c.mech == WallBwrap && !c.want && d == "":
			t.Errorf("%s: no socket wall on a bwrap line and the report says NOTHING. A seat whose socket wall is absent can connect(2) to any host pathname socket in its view (ranger-base-l2gk9), and silence is how that reads as walled", c.name)
		case c.mech == WallBwrap && !c.want && !strings.Contains(d, "DEGRADED"):
			t.Errorf("%s: the report's sentence does not carry the word DEGRADED, which is what an operator scans for: %q", c.name, d)
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
