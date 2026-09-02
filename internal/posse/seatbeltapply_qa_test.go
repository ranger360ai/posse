package posse

// ranger-base-xjw9: a probe that cannot apply a sandbox must SKIP, not fail.
//
// A dispatched persona session that is itself caged has sandbox-exec on PATH
// and may not use it: the kernel refuses the nested sandbox_apply outright.
//
//	$ sandbox-exec -p '(version 1)(allow default)' /bin/echo hi
//	sandbox-exec: sandbox_apply: Operation not permitted
//
// Every probe in reachability_qa_test.go, seatbeltcarveout_qa_test.go and
// seatbeltworktreegit_qa_test.go IS a sandbox-exec, so inside such a session
// all of them measure nothing. They did not say so. Six top-level tests went
// red — which reads to whoever opens the log next as broken code, in a shop
// where "suite green on close" is a metric — and the negative controls did
// something worse: a blanket refusal is exactly the refusal they assert, so
// TestQARecordReachFailsOnTheOytaControl went GREEN having measured nothing.
// Nothing measured must read as neither pass nor fail.
//
// So the gate is not "is sandbox-exec on PATH" — SeatbeltAvailable() asks
// only that, and inside the cage the answer is still yes. It is "does this
// process get to apply one", which only an apply answers.
//
// MEASURED on darwin 25.4.0, all four corners (TestQASandboxApplyProbe*):
//
//	outer profile        inner profile        nested apply
//	(allow default)      (allow default)      allowed
//	(allow default)      any deny             REFUSED
//	any deny             (allow default)      REFUSED
//	any deny             any deny             REFUSED
//
// A deny anywhere refuses. The real cage sits in row three or four, so the
// bead's own one-liner detects it — but the probe carries a deny anyway,
// because of row TWO: under a lenient allow-default wrapper the bead's
// one-liner reports "sandboxable" while every profile these tests actually
// render (SeatbeltProfile emits `(deny file-write*)` unconditionally) is
// refused. The probe has to be shaped like what it predicts.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sbApplyProbeProfile is the production probe's profile, and the two are
// the same object on purpose: ranger-base-heur moved this question into
// production (seatbelt.go), because the reachability row asked the PATH
// question instead and reported a refused apply as a store of record the
// profile denies. A guard that measured applicability its own way could
// skip a suite the code under test would still be wrong in.
const sbApplyProbeProfile = sandboxApplyProbeProfile

// sbSandboxApplyRefusal is "" when this process may apply a seatbelt profile
// and the kernel's own words when it may not. Production's reader, cached
// there once per binary: the carve-out table alone would otherwise pay for
// it 23 times.
func sbSandboxApplyRefusal() string {
	return sandboxApplyRefusal()
}

// sbSkipUnlessSandboxable gates every test whose measurement IS a
// sandbox-exec. It is also called from inside sbRun and wgRun, so a probe
// added later that forgets the top-level line still skips instead of
// reporting a refusal it never earned.
func sbSkipUnlessSandboxable(t *testing.T) {
	t.Helper()
	if !SeatbeltAvailable() {
		t.Skip("no sandbox-exec on this host")
	}
	if why := sbSandboxApplyRefusal(); why != "" {
		t.Skipf("NOTHING MEASURED: this session may not apply a seatbelt profile (%s) — every probe here is a sandbox-exec, so it can neither pass nor fail (ranger-base-xjw9)", why)
	}
}

// sbNest runs `sandbox-exec -p inner /usr/bin/true` under an outer profile,
// and reports what the kernel said. Both arguments are profile bodies.
func sbNest(t *testing.T, outer, inner string) (bool, string) {
	t.Helper()
	prof := sbRenderProfile(t, "outer.sb", outer)
	out, err := exec.Command("sandbox-exec", "-f", prof, "/bin/sh", "-c",
		"sandbox-exec -p '"+inner+"' /usr/bin/true").CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}

// sbCagedOuter is the shape a caged persona session runs under: everything
// allowed, one deny, which is all it takes.
func sbCagedOuter(t *testing.T) string {
	t.Helper()
	return "(version 1)\n(allow default)\n(deny file-write* (subpath " + sbQuote(filepath.Join(t.TempDir(), "nothing")) + "))\n"
}

const sbLenientOuter = "(version 1)\n(allow default)\n"

// The whole grid, run. This is the table in the header, and it is what
// licenses both the guard and the shape of its profile.
func TestQASandboxApplyProbeGrid(t *testing.T) {
	t.Parallel()
	sbSkipUnlessSandboxable(t)
	for _, tc := range []struct {
		what        string
		outer       func(*testing.T) string
		inner       string
		wantAllowed bool
	}{
		{"lenient outer, lenient inner", func(*testing.T) string { return sbLenientOuter }, sbLenientOuter, true},
		{"lenient outer, inner with a deny", func(*testing.T) string { return sbLenientOuter }, sbApplyProbeProfile, false},
		{"caged outer, lenient inner", sbCagedOuter, sbLenientOuter, false},
		{"caged outer, inner with a deny", sbCagedOuter, sbApplyProbeProfile, false},
	} {
		t.Run(tc.what, func(t *testing.T) {
			ok, out := sbNest(t, tc.outer(t), tc.inner)
			if ok != tc.wantAllowed {
				t.Fatalf("nested apply allowed=%v, want %v: %q", ok, tc.wantAllowed, out)
			}
			if !ok && !strings.Contains(out, "sandbox_apply") {
				t.Errorf("the refusal must be the one the guard keys on, got: %q", out)
			}
		})
	}
}

// Row two is why the probe carries a deny: the obvious probe — the bead's
// own one-liner — is UNDISCRIMINATING under a lenient wrapper. It reports
// "sandboxable" there while the REAL rendered profile, which is what the
// suite is about to apply twenty-three times, is refused. Agreement with the
// real thing is the property; the probe is a stand-in for it, not a proxy
// for the word "sandbox-exec".
func TestQASandboxApplyProbeAgreesWithARenderedProfile(t *testing.T) {
	t.Parallel()
	sbSkipUnlessSandboxable(t)
	rendered := SeatbeltProfile("developer", []string{t.TempDir()}, nil, SeatbeltCarveOut{})
	if !strings.Contains(rendered, "(deny file-write*)") {
		t.Fatalf("a rendered profile no longer carries a blanket deny; the probe's shape needs rechecking:\n%s", rendered)
	}
	if ok, out := sbNest(t, sbLenientOuter, rendered); ok {
		t.Fatalf("the rendered profile applied under the lenient wrapper — row two is gone, recheck the grid: %q", out)
	}
	if ok, _ := sbNest(t, sbLenientOuter, sbApplyProbeProfile); ok {
		t.Error("the probe says sandboxable where the rendered profile is refused — it predicts the wrong thing")
	}
	// And the naive probe is the mutant that would: same wrapper, allowed.
	if ok, out := sbNest(t, sbLenientOuter, sbLenientOuter); !ok {
		t.Errorf("the control did not run, so nothing above discriminates: %q", out)
	}
}

// SeatbeltAvailable() is not the question. Inside the cage sandbox-exec is
// still on PATH — LookPath's exact test — so a guard built on it lets the
// six reds straight through. Pinned because reverting to it is the shape
// this bug comes back in.
func TestQASandboxExecStaysOnPathInsideTheCage(t *testing.T) {
	t.Parallel()
	sbSkipUnlessSandboxable(t)
	prof := sbRenderProfile(t, "outer.sb", sbCagedOuter(t))
	out, err := exec.Command("sandbox-exec", "-f", prof, "/bin/sh", "-c", "command -v sandbox-exec").CombinedOutput()
	if err != nil {
		t.Fatalf("could not look for sandbox-exec inside the cage: %v %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); !strings.HasSuffix(got, "/sandbox-exec") {
		t.Fatalf("PATH lookup inside the cage returned %q — this test measures nothing", got)
	}
}

// The probing files must all be behind the gate. A new probe file that
// shells out to sandbox-exec and forgets it is how this comes back, and it
// comes back as a red nobody can reproduce off the box.
func TestQAEverySandboxProbeFileIsGated(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var scanned []string
	for _, f := range files {
		src := mustRead(t, f)
		// The two ways a test file reaches the kernel's sandbox: its own
		// exec, or the production reach probe, which is one underneath.
		if !strings.Contains(src, `exec.Command("sandbox-exec"`) && !strings.Contains(src, "seatbeltReachRow(") {
			continue
		}
		scanned = append(scanned, f)
		if !strings.Contains(src, "sbSkipUnlessSandboxable") {
			t.Errorf("%s runs sandbox-exec but never asks whether this session may apply one (ranger-base-xjw9)", f)
		}
	}
	// A corpus assertion that scanned nothing is satisfied by an empty
	// corpus: name what it actually read.
	if len(scanned) != 6 {
		t.Errorf("expected the six sandbox-probing test files, scanned %d: %v", len(scanned), scanned)
	}
}
