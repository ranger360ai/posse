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
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"runtime"
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
			want = landlockAccessFSResolveUnix | landlockAccessFSRefer
		}
		if r.Access_fs != want {
			t.Errorf("ABI %d: Access_fs = %#x, want %#x — RESOLVE_UNIX from ABI %d and nothing below it, where the kernel refuses the ruleset outright and the seat would not open; REFER on exactly those arms, because a domain handling any filesystem right without it denies every cross-directory rename and link with EXDEV (ranger-base-p5i2c)", abi, r.Access_fs, want, WallUnixABIMin)
		}
	}
}

// TestQAWallUnixRulesetHandsReferBackWhereItHandlesAnyFSRight is
// ranger-base-p5i2c's ruleset pin, and it is the half a reader is most likely
// to half-do: handling REFER without granting it back is the same EXDEV with
// an extra syscall, because Landlock is allow-only and a handled right with
// no rule is denied everywhere.
//
// SO IT PINS BOTH DIRECTIONS AT EVERY ABI THE STEP IS TYPED FOR: the arm that
// handles a filesystem right grants REFER and only REFER, and the arm that
// handles none grants nothing — the ABI 6-8 arm has no refer default-deny to
// undo, and an `/` rule there would be a rule on a ruleset handling no
// filesystem right at all, which the kernel refuses.
//
// AND IT PINS THE GRANT'S MASK, which is what keeps the socket wall a wall:
// the whole of that wall is RESOLVE_UNIX being handled and granted NOWHERE,
// so a grant that carried it would be this layer granting back the one thing
// it exists to take. The live pin (TestLiveWallUnixLeavesCrossDirectoryRename)
// reads the kernel's side of both.
func TestQAWallUnixRulesetHandsReferBackWhereItHandlesAnyFSRight(t *testing.T) {
	t.Parallel()
	if landlockAccessFSRefer != 0x2000 {
		t.Fatalf("landlockAccessFSRefer = %#x, and linux/landlock.h's LANDLOCK_ACCESS_FS_REFER is 0x2000 (x/sys v0.47.0 agrees, and wallunix_linux.go asserts it at compile time on the GOOS that can)", landlockAccessFSRefer)
	}
	if wallUnixReferPath != "/" {
		t.Errorf("the REFER grant names %q, and the decision (ADR 0080 D34) names `/`: the grant has to reach everywhere the seat can already write, which the bwrap mounts decide and this layer does not — a narrower rule brings the same EXDEV back at whatever path the next PID grants", wallUnixReferPath)
	}
	for abi := WallUnixSignalABIMin; abi <= 12; abi++ {
		fs, grant := wallUnixRuleset(abi).Access_fs, wallUnixReferGrant(abi)
		switch {
		case fs == 0:
			if grant != 0 {
				t.Errorf("ABI %d: the ruleset handles no filesystem right (Access_fs = 0) and the step would still add a rule granting %#x on %s — there is no refer default-deny to undo on this arm, and the kernel refuses a path rule whose right the ruleset does not handle", abi, grant, wallUnixReferPath)
			}
		default:
			if grant != landlockAccessFSRefer {
				t.Errorf("ABI %d: the ruleset handles Access_fs = %#x and the step grants %#x on %s, want exactly REFER (%#x) — handled with no rule is DENIED, so a domain that handles a filesystem right and does not grant REFER back makes every cross-directory rename(2) and link(2) EXDEV for everything in this seat (ranger-base-p5i2c, ADR 0080 D33)", abi, fs, grant, wallUnixReferPath, uint64(landlockAccessFSRefer))
			}
			if grant&landlockAccessFSResolveUnix != 0 {
				t.Errorf("ABI %d: the grant is %#x and carries RESOLVE_UNIX — the socket wall IS that right handled and granted nowhere, so granting it on %s would undo the whole layer (ADR 0080 D28)", abi, grant, wallUnixReferPath)
			}
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

// wuStubApplyMarker is the sentence the apply stub returns in the two pins
// below, so an arm that was MEANT to reach the apply can prove it did by the
// error it got back rather than by a counter alone.
const wuStubApplyMarker = "the apply stub: this pin never applies a real Landlock domain to the test binary"

// TestQAWallUnixStepRefusesOffTheThreadGroupLeader is ranger-base-vxz74's pin:
// the step refuses, BEFORE the apply, when the goroutine it runs on is not on
// the thread group leader.
//
// WHAT IS BEING DEFENDED. Three of the four things this step delivers are
// per-task — PR_SET_NO_NEW_PRIVS, the Landlock domain
// (`landlock_restrict_self(fd, 0)`, no TSYNC) and bwrap's `--die-with-parent`
// pdeathsig, which is zero on every thread the Go runtime clones — and
// `execve` from a non-leader thread keeps THAT task. So a step that restricted
// one thread and exec'd from another would open a seat with no socket wall, no
// signal scope and no die-with-parent, with the render and `posse gates` both
// still reporting the layer as up: the fail-OPEN direction ADR 0080 D4
// forbids. RunWallUnix holds runtime.LockOSThread from its first statement,
// which closes the window, and this check is what makes the locked thread the
// one the pdeathsig is on.
//
// THE CONTROL ARM IS THE POINT, as everywhere else in this file's family: a
// refusal proves nothing unless the same body with equal ids reaches the
// apply. Without that arm, a check written `if true` reads green.
//
// BOTH ARMS STUB THE APPLY, and that is not a convenience: the real apply
// would put a signal-scoped Landlock domain and NO_NEW_PRIVS on the test
// binary's own thread for the rest of the run, which every later pin that
// signals a host process would then read as its own failure.
//
// SERIAL, no t.Parallel: wallUnixABI, restrictWallUnix and wallUnixTask are
// package-level vars.
func TestQAWallUnixStepRefusesOffTheThreadGroupLeader(t *testing.T) {
	restoreABI, restoreApply, restoreTask := wallUnixABI, restrictWallUnix, wallUnixTask
	t.Cleanup(func() {
		wallUnixABI, restrictWallUnix, wallUnixTask = restoreABI, restoreApply, restoreTask
	})
	wallUnixABI = func() int { return WallUnixABIMin }

	applied := 0
	restrictWallUnix = func(int) error {
		applied++
		return errors.New(wuStubApplyMarker)
	}

	for _, c := range []struct {
		name     string
		tid, pid int
		refuses  bool
	}{
		// The control: the same body on the leader must reach the apply. If
		// this arm ever stops doing so, the arm below it is not reading the
		// leader check and must not be read as a pass.
		{"control: on the leader, where tid == pid", 4242, 4242, false},
		{"a thread the runtime cloned", 4243, 4242, true},
		{"and one of another process's threads, which is the same class", 1, 4242, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			applied = 0
			wallUnixTask = func() (int, int) { return c.tid, c.pid }
			err := RunWallUnix([]string{"posse", WallUnixVerb, "--", "true"})
			if err == nil {
				t.Fatalf("tid %d, pid %d: RunWallUnix returned nil — on success this call does not return at all (it becomes the runtime), so a nil here is a seat that opened", c.tid, c.pid)
			}
			if !c.refuses {
				if applied != 1 {
					t.Fatalf("tid %d == pid %d is the LEADER and the apply was called %d time(s), want 1 — the leader check is refusing the one thread the step is allowed to run on, so every seat on this host would fail to open: %v", c.tid, c.pid, applied, err)
				}
				if !strings.Contains(err.Error(), wuStubApplyMarker) {
					t.Errorf("tid %d == pid %d: the error came from somewhere other than the apply (%v) — this arm must get PAST the leader check", c.tid, c.pid, err)
				}
				return
			}
			if applied != 0 {
				t.Errorf("tid %d of pid %d: the apply was called %d time(s), so the step restricted a thread it is not about to exec from — NNP, the domain and the pdeathsig are per-task and the exec would keep another one (ADR 0080 D4): %v",
					c.tid, c.pid, applied, err)
			}
			said := err.Error()
			for _, want := range []string{
				WallUnixVerb,
				"thread group leader",
				"per-task",
				"die-with-parent",
				"ADR 0080 D4",
				"runtime.LockOSThread",
				strconv.Itoa(c.tid),
				strconv.Itoa(c.pid),
			} {
				if !strings.Contains(said, want) {
					t.Errorf("tid %d of pid %d: the refusal does not name %q — this sentence is the whole of what an operator reading a seat that would not open has to go on, and it has to name the thread, the process and why a relaunch rather than a kernel is the answer: %s",
						c.tid, c.pid, want, said)
				}
			}
		})
	}
}

// TestQAWallUnixStepRefusesFromARealNonLeaderThread is the same refusal read
// through the LIVE probe on a real cloned thread, which is the pin
// ranger-base-vxz74 asks for by name. The pin above stubs wallUnixTask, so it
// cannot see a liveWallUnixTask that answered the wrong pair — `getpid` twice,
// say, which would read "always the leader" and restore the fail-open with
// every stubbed arm still green.
//
// THE THREAD IS LOCKED AND THEN READ, never assumed: a fresh goroutine is
// overwhelmingly on a cloned M, but nothing guarantees it, so the pin asks the
// production probe which thread it got and says NOTHING MEASURED rather than
// passing on an arm it could not set up.
//
// WHICH THREAD IT IS ON COMES FROM THE KERNEL AND NOT FROM THE PROBE, through
// /proc/thread-self, and the two readings are compared. Without that, the one
// mutant this pin exists for would not red but SKIP: a liveWallUnixTask that
// answered `getpid` twice reads "always the leader" everywhere, the stubbed pin
// above stays green because it stubs the probe, and this pin would decide it
// had no non-leader thread and abstain. A skip is green. So the kernel's tid
// decides whether the arm is set up, and a probe that disagrees with it is a
// failure in its own right.
//
// OFF LINUX IT SKIPS, at the procfs read: gettid and the thread group leader
// are Linux ideas, liveWallUnixTask's non-Linux twin answers "leader" by
// construction (wallunix_other.go says why it must), and the darwin reading of
// this refusal is the pin above.
//
// THE APPLY IS STUBBED for the pin above's reason, and here it is load-bearing
// twice over: if the leader check ever stopped refusing, this arm would
// otherwise apply a real domain from a thread the test framework will reuse.
//
// SERIAL, no t.Parallel.
func TestQAWallUnixStepRefusesFromARealNonLeaderThread(t *testing.T) {
	restoreABI, restoreApply := wallUnixABI, restrictWallUnix
	t.Cleanup(func() { wallUnixABI, restrictWallUnix = restoreABI, restoreApply })
	wallUnixABI = func() int { return WallUnixABIMin }

	applied := 0
	restrictWallUnix = func(int) error {
		applied++
		return errors.New(wuStubApplyMarker)
	}

	type reading struct {
		ktid, tid, pid int
		kerr           error
		err            error
	}
	got := make(chan reading, 1)
	go func() {
		// Locked FIRST, so the ids read below are the ids RunWallUnix will
		// read: an unlocked goroutine could answer one thread here and run
		// the step on another, which is the very hazard this pin is about.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		r := reading{pid: os.Getpid()}
		r.tid, _ = wallUnixTask()
		r.ktid, r.kerr = wuKernelTID()
		if r.kerr == nil && r.ktid != r.pid {
			r.err = RunWallUnix([]string{"posse", WallUnixVerb, "--", "true"})
		}
		got <- r
	}()
	r := <-got

	if r.kerr != nil {
		t.Skipf("the kernel's own answer for this thread is unreadable (%v) — on Linux that is /proc/thread-self/status, and the stubbed reading of this refusal is TestQAWallUnixStepRefusesOffTheThreadGroupLeader", r.kerr)
	}
	if r.tid != r.ktid {
		t.Errorf("liveWallUnixTask answered thread %d and the kernel says this thread is %d (/proc/thread-self/status, process %d) — the probe the step's leader check reads is not reading the calling thread, so that check would answer about some other task and the pins that stub the probe cannot see it (ranger-base-vxz74)", r.tid, r.ktid, r.pid)
	}
	if r.ktid == r.pid {
		t.Skipf("NOTHING MEASURED: the goroutine locked onto thread %d, which IS the leader of process %d, so there was no non-leader thread to run the step on. Nothing guarantees a new goroutine gets a cloned M; re-run before reading this as a finding", r.ktid, r.pid)
	}
	if r.err == nil {
		t.Fatalf("thread %d of process %d: RunWallUnix returned nil from a real non-leader thread — on success this call does not return at all, so this is a seat that would have exec'd with NNP, the Landlock domain and the pdeathsig on a task execve does not keep (ADR 0080 D4, ranger-base-vxz74)", r.ktid, r.pid)
	}
	if applied != 0 {
		t.Errorf("thread %d of process %d: the apply was called %d time(s) from a real non-leader thread: %v", r.ktid, r.pid, applied, r.err)
	}
	if !strings.Contains(r.err.Error(), "thread group leader") {
		t.Errorf("thread %d of process %d: the refusal is not the leader check's (%v) — this pin reads the live probe, so an arm refused for any other reason has measured nothing", r.ktid, r.pid, r.err)
	}
}

// wuKernelTID is the calling THREAD's id as the kernel reports it, read
// through /proc/thread-self — the magic link that resolves to the reading
// thread's own directory, so no tid has to be known in order to ask which tid
// this is. The `Pid:` row of a THREAD's status file is that thread's id;
// `Tgid:` is the process's.
//
// A FILE READ AND NOT unix.Gettid(), for one reason: this file is read from
// darwin (the package's suite runs there), x/sys has Gettid on Linux only, and
// a second reading of the tid is worthless if it comes from the same place as
// the first. Off Linux the open fails and the pin above skips on the error,
// which is the honest answer rather than a fallback.
func wuKernelTID() (int, error) {
	b, err := os.ReadFile("/proc/thread-self/status")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(wuStatusField(string(b), "Pid:"))
}

// TestQAWallUnixStepLocksItsOSThreadFirstAndNeverUnlocks is the OTHER half of
// ranger-base-vxz74's fix, and it is a source pin because the half it covers
// has no observable a test can read.
//
// WHY THERE IS NOTHING TO MEASURE. The lock does not change what any syscall
// answers; it removes the scheduler's freedom to move this goroutine between
// the restrict and the exec. A run without it passes every other pin in this
// file and every live pin in wallunixlive_test.go, and it passed 600 of 600
// live execs on this box before the fix (the MEASURED paragraph on
// RunWallUnix) — the hazard is latent and an attacker cannot steer it, which
// is the whole reason the finding is a P2 and the whole reason a pin has to
// read the SOURCE: the first thing to notice a lost lock would otherwise be a
// seat that came up unwalled, saying so nowhere.
//
// IT ALSO READS "FIRST", and that is not style. Everything after it can park
// the goroutine — exec.LookPath opens files, the ABI read is a syscall — so a
// lock taken after any of them is a lock on whatever thread the goroutine
// landed on, and the leader check below it would then refuse a seat that was
// perfectly fine when it started. First statement, or the refusal rate becomes
// the scheduler's business.
//
// AND NO UNLOCK ANYWHERE IN THE FILE: the lock has to outlive this function in
// the one direction that matters (on success there is no "after" — the process
// is replaced), so a `defer runtime.UnlockOSThread()` would undo it at exactly
// the moment a refusal path returns and leave nothing pinned at all.
//
// THE FILE SET IS SPELLED HERE and nothing is enumerated, so this pin is
// outside internal/treepins' one-package-census class and needs no Makefile
// door of its own (treewidedoor_qa_test.go's derived key: discovered file sets
// need a door, named ones do not).
func TestQAWallUnixStepLocksItsOSThreadFirstAndNeverUnlocks(t *testing.T) {
	t.Parallel()
	const file = "wallunix.go"
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(src, []byte("UnlockOSThread")) {
		t.Errorf("%s calls runtime.UnlockOSThread. The step locks its thread so that PR_SET_NO_NEW_PRIVS, landlock_restrict_self and the exec all land on the one task bwrap set a pdeathsig on; an unlock on the way out gives that freedom back on every path that can still reach the exec (ranger-base-vxz74)", file)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "RunWallUnix" {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil || len(fn.Body.List) == 0 {
		t.Fatalf("RunWallUnix is not in %s with a body — this pin reads that function's first statement, so a rename has to bring it along", file)
	}
	first, ok := fn.Body.List[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("RunWallUnix's first statement is a %T and not a call — it must be runtime.LockOSThread(), because every statement that could come before it can park the goroutine onto another thread (ranger-base-vxz74)", fn.Body.List[0])
	}
	got := types.ExprString(first.X)
	if got != "runtime.LockOSThread()" {
		t.Errorf("RunWallUnix's first statement is %s, want runtime.LockOSThread(). The three things this step applies are per-task and the exec keeps one task, so a goroutine the scheduler is still free to move would exec a seat with no socket wall, no signal scope and no die-with-parent while `posse gates` said the layer was up (ADR 0080 D4, ranger-base-vxz74)", got)
	}
}

// TestQAWallUnixStepReArmsDieWithParent is ranger-base-6ec6n's stubbed pin:
// after the apply, the step sets PR_SET_PDEATHSIG again and THEN re-reads its
// ppid, and only an unchanged ppid reaches the exec.
//
// WHY THE ORDER IS THE ASSERTION. The ppid check is the standard race guard
// for a pdeathsig set late: a parent that died BEFORE the prctl delivers
// nothing, and the seat was reparented instead. Read before the prctl, the
// check has a window after it that the kernel will not cover; so the call log
// below must read ppid (at entry), arm, ppid, exec — and nothing else.
//
// THE CONTROL ARM IS THE POINT: an unchanged ppid must reach the exec stub
// with the runtime's argv. Without it, a step that refused every seat would
// read green on the two refusal arms.
//
// The live half — the re-arm actually reaching the agent's leader, from a
// launcher that never set one — is TestLiveWallUnixStepReArmsAClearedDieWithParent.
//
// SERIAL, no t.Parallel: every seam here is a package-level var.
func TestQAWallUnixStepReArmsDieWithParent(t *testing.T) {
	restoreABI, restoreApply, restoreTask := wallUnixABI, restrictWallUnix, wallUnixTask
	restoreArm, restorePpid, restoreExec := armWallUnixPdeathsig, wallUnixPpid, wallUnixExec
	t.Cleanup(func() {
		wallUnixABI, restrictWallUnix, wallUnixTask = restoreABI, restoreApply, restoreTask
		armWallUnixPdeathsig, wallUnixPpid, wallUnixExec = restoreArm, restorePpid, restoreExec
	})
	wallUnixABI = func() int { return WallUnixABIMin }
	restrictWallUnix = func(int) error { return nil }
	wallUnixTask = func() (int, int) { return 4242, 4242 }
	const execMarker = "the exec stub: this pin never replaces the test binary"

	for _, c := range []struct {
		name    string
		ppids   []int // what each wallUnixPpid call answers, in order
		armErr  error
		wantLog string
		want    []string // in the error; execMarker means the exec was reached
	}{
		{"control: the parent is still there, so the seat starts", []int{77, 77}, nil,
			"ppid arm ppid exec", []string{execMarker}},
		{"the parent died before the re-arm", []int{77, 1}, nil,
			"ppid arm ppid", []string{WallUnixVerb, "parent (pid 77) died", "reparented to pid 1", "ranger-base-6ec6n"}},
		{"the re-arm itself failed", []int{77, 77}, errors.New("EINVAL from the stub"),
			"ppid arm", []string{WallUnixVerb, "PR_SET_PDEATHSIG", "EINVAL from the stub", "ranger-base-6ec6n"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var log []string
			n := 0
			wallUnixPpid = func() int {
				log = append(log, "ppid")
				p := c.ppids[len(c.ppids)-1]
				if n < len(c.ppids) {
					p = c.ppids[n]
				}
				n++
				return p
			}
			armWallUnixPdeathsig = func() error { log = append(log, "arm"); return c.armErr }
			var execArgv []string
			wallUnixExec = func(_ string, argv []string, _ []string) error {
				log = append(log, "exec")
				execArgv = argv
				return errors.New(execMarker)
			}
			err := RunWallUnix([]string{"posse", WallUnixVerb, "--", "true", "x"})
			if err == nil {
				t.Fatalf("RunWallUnix returned nil — only the exec stub can end this call and it returns an error")
			}
			if got := strings.Join(log, " "); got != c.wantLog {
				t.Errorf("the step's calls read %q, want %q — the ppid has to be re-read AFTER the re-arm, and the exec reached only on an unchanged one (ranger-base-6ec6n): %v", got, c.wantLog, err)
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("the step's answer does not name %q: %v", w, err)
				}
			}
			if c.want[0] == execMarker && strings.Join(execArgv, " ") != "true x" {
				t.Errorf("the exec got argv %q, want the runtime's own %q", execArgv, "true x")
			}
		})
	}
}
