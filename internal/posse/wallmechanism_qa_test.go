//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-eoxo1: the L2 wall's mechanism is a host fact
// (ADR 0080 D16). `cage: seatbelt` names the TIER; `sandbox-exec` and
// `bwrap` are what realize it, on darwin and on Linux respectively.
//
// WHY A PIN, AND WHY IT DRIVES A HOST IT IS NOT RUNNING ON. The whole of
// this change is unreachable on the box that builds it: the Linux arm of
// `seatbeltTierAvailableOn` decides whether the seatbelt tier exists on the
// host ADR 0083 moves the crew to, and on darwin it is never entered. An
// untested arm that answers "yes, this host has an L2 write wall" is the one
// answer nobody may be handed on a guess — posse would print a wall it never
// put up, in front of every caged seat on that box. So the GOOS is a
// PARAMETER here and the two host measurements are variables, and the pin
// drives all four corners of the Linux decision from darwin.
//
// MEASURED BY HOST where the host is the subject: the mechanism-name arm
// says which GOOS it is reading and asserts per OS, and the live-probe arm
// (bwrapApplyProbe with no bwrap to resolve) skips where a bwrap is on the
// gates-free PATH, because there the answer is the kernel's and not the
// resolver's.

import (
	"runtime"
	"strings"
	"testing"
)

// wmStub swaps the two host measurements the Linux availability decision
// reads and restores them. No t.Parallel in anything that calls it: these
// are package-level func vars (cmd/testparallel's own rule).
func wmStub(t *testing.T, resolves bool, refusal string) {
	t.Helper()
	hadR, hadA := bwrapResolves, bwrapApplyRefusal
	bwrapResolves = func() bool { return resolves }
	bwrapApplyRefusal = func() string { return refusal }
	t.Cleanup(func() { bwrapResolves, bwrapApplyRefusal = hadR, hadA })
}

// The naming half: which mechanism realizes the tier, per OS. Pure, so the
// answer for the other host is readable from this one.
func TestQAWallMechanismNamesTheBinaryThatRealizesTheSeatbeltTier(t *testing.T) {
	// No t.Parallel: the last arm deletes AvailableCages[CageSeatbelt] to
	// show that the NAME does not read the table, and that map is a package
	// var every parity test in this package reads.
	for _, tc := range []struct{ goos, want string }{
		{"darwin", WallSandboxExec},
		{"linux", WallBwrap},
		// A GOOS with neither is the "" D16 names, and the only one: the
		// tier has no third mechanism, so a host that is not one of these
		// two has no L2 wall to label at all.
		{"windows", ""},
		{"freebsd", ""},
	} {
		if got := wallMechanismOn(tc.goos); got != tc.want {
			t.Errorf("wallMechanismOn(%q) = %q, want %q", tc.goos, got, tc.want)
		}
	}
	// And the live reading agrees with this box, so the parameterized
	// function above is the one production reads and not a parallel table.
	if got, want := WallMechanism(), wallMechanismOn(runtime.GOOS); got != want {
		t.Errorf("WallMechanism() = %q on %s, but wallMechanismOn says %q", got, runtime.GOOS, want)
	}
	// A NAMING QUESTION AND NOT AN AVAILABILITY ONE, which is the half a
	// later reader is most likely to collapse: the name does not consult
	// the table, so a host whose kernel refuses the namespace still reads
	// `bwrap` and says so over its ✗ rows.
	had := AvailableCages[CageSeatbelt]
	delete(AvailableCages, CageSeatbelt)
	defer func() {
		if had {
			AvailableCages[CageSeatbelt] = true
		}
	}()
	if got, want := WallMechanism(), wallMechanismOn(runtime.GOOS); got != want {
		t.Errorf("WallMechanism() reads the availability table: %q with the tier deleted, %q with it", got, want)
	}
}

// The availability half, every corner of it, on a host that gives one.
func TestQASeatbeltTierAvailabilityOnLinuxNeedsBwrapAndTheApplyProbe(t *testing.T) {
	// No t.Parallel: wmStub writes package-level func vars.
	for _, tc := range []struct {
		name     string
		goos     string
		resolves bool
		refusal  string
		want     bool
	}{
		{"both measured", "linux", true, "", true},
		// The two halves that each alone would be a wall posse announces
		// and does not have.
		{"no bwrap on PATH", "linux", false, "", false},
		{"kernel refuses the namespace", "linux", true, "bwrap: No permissions to creating new namespace", false},
		{"could not even ask", "linux", true, "could not ask: bwrap: executable file not found outside posse's gates dirs", false},
		{"neither", "linux", false, "bwrap: Operation not permitted", false},
		// The control that makes the GOOS gate a measurement: both
		// readings green and the answer is still no, because bwrap is not
		// what realizes the tier there.
		{"darwin, both green", "darwin", true, "", false},
		{"windows, both green", "windows", true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wmStub(t, tc.resolves, tc.refusal)
			if got := bwrapAvailableOn(tc.goos); got != tc.want {
				t.Errorf("bwrapAvailableOn(%q) with resolves=%v refusal=%q = %v, want %v", tc.goos, tc.resolves, tc.refusal, got, tc.want)
			}
		})
	}

	// And the tier's own decision routes through it: the Linux arm of the
	// one function init calls is what the table fill reads, so a pin on
	// bwrapAvailableOn alone would leave the route untested.
	t.Run("the tier decision routes the linux arm through it", func(t *testing.T) {
		wmStub(t, true, "")
		if !seatbeltTierAvailableOn("linux") {
			t.Error("a Linux host with bwrap and a passing probe provides the seatbelt tier")
		}
		wmStub(t, true, "bwrap: No permissions to creating new namespace")
		if seatbeltTierAvailableOn("linux") {
			t.Error("a Linux host whose kernel refuses the namespace does NOT provide the seatbelt tier, however available the binary is")
		}
		// A GOOS with no mechanism takes neither arm, whatever the
		// measurements say.
		wmStub(t, true, "")
		if seatbeltTierAvailableOn("windows") {
			t.Error("a host with no wall mechanism cannot provide the tier")
		}
	})

	// Neither measurement is taken off Linux. The probe forks, and a host
	// that will never use it would pay for that fork at every process
	// start — including every one of posse's own gate shims.
	t.Run("neither measurement is taken off linux", func(t *testing.T) {
		asked := 0
		hadR, hadA := bwrapResolves, bwrapApplyRefusal
		bwrapResolves = func() bool { asked++; return true }
		bwrapApplyRefusal = func() string { asked++; return "" }
		t.Cleanup(func() { bwrapResolves, bwrapApplyRefusal = hadR, hadA })
		bwrapAvailableOn("darwin")
		if asked != 0 {
			t.Errorf("the darwin arm took %d host measurement(s); the GOOS test must short-circuit before either", asked)
		}
		if bwrapAvailableOn("linux"); asked == 0 {
			t.Error("the linux arm took no measurement at all — the stubs are not the ones the decision reads")
		}
	})
}

// The probe's own abstention, measured on this box: "could not ask" is a
// reason and never "". There it is the resolver's refusal rather than the
// kernel's, which is the only one of the probe's two failure modes a darwin
// box can produce — and it is the one that matters, because "" here would
// make AvailableCages claim a wall nothing was asked about.
func TestQABwrapApplyProbeNeverAnswersSilenceWhenItCouldNotAsk(t *testing.T) {
	t.Parallel()
	if _, err := protocolBin("bwrap"); err == nil {
		t.Skip("NOTHING MEASURED: a bwrap resolves on the gates-free PATH here, so this probe's answer is the kernel's and not the resolver's")
	}
	why := bwrapApplyProbe()
	if why == "" {
		t.Fatal(`bwrapApplyProbe answered "" with no bwrap to resolve — "" is read as "this host provides the L2 wall", so the one answer it may not give on a guess is the one it gave`)
	}
	if !strings.Contains(why, "could not ask") {
		t.Errorf("the abstention must say it is one, so a reader can tell it from the kernel's words: %q", why)
	}
	// The argv is D17's base mount and nothing lenient: a probe shaped
	// unlike the wall it predicts reports "walled" on a host that refuses
	// every render (ranger-base-xjw9's grid, the same lesson).
	if got, want := strings.Join(bwrapApplyProbeArgv, " "), "--ro-bind / / /bin/true"; got != want {
		t.Errorf("the probe's argv is %q, want %q — the base mount of every D17 render", got, want)
	}
}

// The row an operator reads the wall off. CageLabel is a LABEL: the seatbelt
// tier carries the mechanism, every other tier is its own name, and
// Parity.Cage stays the bare value it is compared against.
func TestQAParityCageRowNamesTheMechanismThatRealizesTheTier(t *testing.T) {
	t.Parallel()
	m := WallMechanism()
	if m == "" {
		t.Skip("NOTHING MEASURED: this GOOS has no L2 wall mechanism, so there is no suffix for a row to carry")
	}
	if got, want := CageLabel(CageSeatbelt), CageSeatbelt+" ("+m+")"; got != want {
		t.Errorf("CageLabel(%q) = %q, want %q", CageSeatbelt, got, want)
	}
	// Only the seatbelt tier. shims has no wall to name and the container
	// tier's mechanism is the home's engine, reported where that is read.
	for _, c := range []string{CageShims, CageContainer} {
		if got := CageLabel(c); got != c {
			t.Errorf("CageLabel(%q) = %q — only the seatbelt tier carries a mechanism", c, got)
		}
	}
	p := Parity{Runtime: "claude", Cage: CageSeatbelt, Tier: TierStrong, Realized: map[string]RealizedGate{}}
	row, _, _ := strings.Cut(p.String(), "\n")
	if want := "claude @ " + CageSeatbelt + " (" + m + ")/" + TierStrong + ":"; !strings.HasPrefix(row, want) {
		t.Errorf("the parity cage row must name the mechanism (ADR 0080 D16):\n got %q\nwant prefix %q", row, want)
	}
	// NOTHING ELSE RENDERS DIFFERENTLY, which is the bead's own done-when:
	// drop the suffix and the row is byte-identical to what it was before
	// the mechanism had a name.
	if got, want := strings.Replace(row, " ("+m+")", "", 1), "claude @ "+CageSeatbelt+"/"+TierStrong+": all gates realized"; got != want {
		t.Errorf("the suffix is the ONLY change to this row:\n got %q\nwant %q", got, want)
	}
	// And the field is untouched, because it is compared against the
	// constant and written into session records.
	if p.Cage != CageSeatbelt {
		t.Errorf("Parity.Cage must stay the bare tier, got %q", p.Cage)
	}
}
