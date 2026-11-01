//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-npqem — the apply probe resolves its own `sandbox-exec` the
// way it RUNS it, and a resolution it cannot make is an abstention.
//
// The class is ranger-base-d0nyb's: a probe for a name a shipped
// protocolCmd/protocolBin site execs, reading THIS process's gated $PATH
// while the exec resolves against ADR 0077 D3's gates-free one. This member
// is the one whose wrong answer is PERMISSIVE and silent. sandboxApplyProbe
// gated on SeatbeltAvailable() — a gated-PATH lookup, which the shim
// satisfies — then exec'd the same name through protocolCmd, which refuses a
// bare name nothing outside the gates answers (ranger-base-cwsi8). The
// refusal arrives as `out ""` on cmd.Err, a non-apply error the call site
// drops by design, so sandboxApplyWhy stayed "" — and "" is read by every
// caller as "this process MAY apply a profile". reachability.go then
// believed a measurement that never happened, which is the failure
// ranger-base-heur was filed for.
//
// TWO SURFACES, TWO DIRECTIONS, and both are pinned here because the
// decision is the deliverable: sandboxApplyProbe moves to the gates-free
// PATH, SeatbeltAvailable deliberately does not (its binary is the one a
// herdr pane types, in herdr's process tree with the seat's PATH). A later
// sweep of this class that "fixes" the second one is what the second test
// exists to stop.
//
// NOT BEHIND sbSkipUnlessSandboxable, and must not be: every sandbox-exec
// these arms reach is a planted script on a fixture PATH, never the kernel,
// so there is no apply to be refused and nothing here needs a host that
// grants one. A real apply is pinned by its neighbours
// (seatbeltapply_qa_test.go, reachability_qa_test.go).
//
// MEASURED 2026-11-01 (darwin 25.6.0, this worktree at main ea4b5d08), each
// mutation against the one arm it should red — see the per-test comments.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sbgPlant writes a fake `sandbox-exec` into a new dir under parent, with
// the body given, and returns the dir. The body is shell; `$0 "$@"` is
// whatever the probe passed it.
func sbgPlant(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(filepath.Join(dir, "sandbox-exec"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// sbgGatesBin is the launching seat's wall: a gates bin dir (isGatesPath
// catches the 'gates' path ELEMENT) holding a `sandbox-exec` shim that marks
// itself if anything runs it. Nothing should.
func sbgGatesBin(t *testing.T) (dir, mark string) {
	t.Helper()
	mark = filepath.Join(t.TempDir(), "gates-sandbox-exec-ran")
	dir = sbgPlant(t, filepath.Join(t.TempDir(), "state", "gates", "launcher", "bin"),
		": > "+mark+"\nexit 0\n")
	return dir, mark
}

func sbgRan(t *testing.T, mark string) bool {
	t.Helper()
	_, err := os.Stat(mark)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

// The probe, over the four answers it has to tell apart. Only one of them
// may be "": the one it measured.
//
// MUTATION-CHECKED: dropping the protocolBin guard from sandboxApplyProbe —
// the code as it stood before this bead — reds the first arm alone, with
// `sandboxApplyProbe() = ""`, which is the live defect: a permissive answer
// nobody printed. The other three arms stay green on that mutant, which is
// the point of having them: each is a way the probe could have been made to
// abstain for the wrong reason.
func TestSandboxApplyProbeResolvesSandboxExecOutsideTheGates(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("SeatbeltAvailable is false off darwin, so the probe answers before any of this")
	}
	sep := string(os.PathListSeparator)

	// (a) The bug. A gates bin dir holding a `sandbox-exec`, and NOT ONE
	// non-gates element that answers — so the gated lookup says yes and the
	// gates-free resolution has nothing to reach.
	t.Run("only in a gates dir is an abstention, never a permission", func(t *testing.T) {
		gates, mark := sbgGatesBin(t)
		empty := filepath.Join(t.TempDir(), "bin") // an ordinary dir, holding no sandbox-exec
		if err := os.MkdirAll(empty, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", gates+sep+empty)

		// The fixture is the experiment: if the host question answers no,
		// the probe returns "" one line in and this arm measures nothing
		// (ranger-base-z4vx).
		if !SeatbeltAvailable() {
			t.Fatalf("fixture did not satisfy the gated lookup — SeatbeltAvailable() must find the shim in %s, or nothing below discriminates", gates)
		}
		why := sandboxApplyProbe()
		if why == "" {
			t.Errorf("sandboxApplyProbe() = \"\" — that reads as \"this process MAY apply a profile\", drawn from an exec that never ran: protocolCmd refuses a bare `sandbox-exec` nothing outside %s answers (ADR 0077 D3, ranger-base-cwsi8)", gates)
		}
		if !strings.Contains(why, "executable file not found outside posse's gates dirs") {
			t.Errorf("the abstention must name what it could not resolve: %q", why)
		}
		if sbgRan(t, mark) {
			t.Errorf("the gates-dir sandbox-exec ran — posse's own children run outside every gates dir (ADR 0077 D3)")
		}
	})

	// (b) The control for (a): the same planted script, in an ORDINARY dir
	// behind the same gates dir. The resolution answers, the probe runs, and
	// "" comes back — so (a)'s non-empty answer is the gates-ness and not
	// the fixture's shape.
	t.Run("resolvable outside the gates is measured, and answers \"\"", func(t *testing.T) {
		gates, gmark := sbgGatesBin(t)
		rmark := filepath.Join(t.TempDir(), "real-sandbox-exec-ran")
		real := sbgPlant(t, filepath.Join(t.TempDir(), "bin"), ": > "+rmark+"\nexit 0\n")
		t.Setenv("PATH", gates+sep+real)

		if why := sandboxApplyProbe(); why != "" {
			t.Errorf("sandboxApplyProbe() = %q, want \"\" — the apply was measured and it succeeded", why)
		}
		if !sbgRan(t, rmark) {
			t.Error("the non-gates sandbox-exec never ran, so this control measured nothing")
		}
		if sbgRan(t, gmark) {
			t.Error("the gates-dir sandbox-exec ran — it leads PATH, and posse's own children resolve past it (ADR 0077 D3)")
		}
	})

	// (c) The answer the function is named for still arrives verbatim: a
	// resolvable sandbox-exec that refuses the apply. Keyed on
	// `sandbox_apply` and not on the three words it shares with a refused
	// write (isSandboxApplyRefusal, ranger-base-heur).
	t.Run("a resolvable sandbox-exec that refuses the apply says so", func(t *testing.T) {
		const refusal = "sandbox-exec: sandbox_apply: Operation not permitted"
		real := sbgPlant(t, filepath.Join(t.TempDir(), "bin"), "echo '"+refusal+"' >&2\nexit 1\n")
		t.Setenv("PATH", real)

		if why := sandboxApplyProbe(); why != refusal {
			t.Errorf("sandboxApplyProbe() = %q, want the kernel's own words %q", why, refusal)
		}
	})

	// (d) And the deliberate pass-through is untouched: a probe failure that
	// is NOT an apply refusal stays "", because it is a real problem with
	// this host and the check that meets it should report it as itself
	// rather than disappear into an abstention. This is the arm a fix of the
	// shape "abstain on any probe error" would red.
	t.Run("a failure that is not an apply refusal is not an abstention", func(t *testing.T) {
		real := sbgPlant(t, filepath.Join(t.TempDir(), "bin"),
			"echo 'sandbox-exec: /nope/does-not-exist.sb: No such file or directory' >&2\nexit 65\n")
		t.Setenv("PATH", real)

		if why := sandboxApplyProbe(); why != "" {
			t.Errorf("sandboxApplyProbe() = %q, want \"\" — only an apply refusal is ours to abstain on", why)
		}
	})
}

// The OTHER direction, and the decision this bead had to make: SeatbeltAvailable
// keeps the gated lookup.
//
// It is not in the class its three siblings are (ranger-base-d0nyb fixed
// Herdr.Available and cage's engine probes; ranger-base-g1xnt fixed
// Bd.Available). Those probe a binary posse itself is about to exec. This one
// probes the `sandbox-exec` startPlanned types into a herdr pane, which runs
// in herdr's process tree carrying the seat's PATH — so the gated lookup is
// the one that matches how it will be resolved, and a gates-free probe would
// answer a question neither AvailableCages[CageSeatbelt] nor the launch gate
// asked.
//
// MUTATION-CHECKED: routing SeatbeltAvailable through protocolBin — the
// one-line "fix" the class invites — reds this test's first arm ("the HOST
// question must still answer yes"). It also reds arm (a) of the test above,
// at its FIXTURE guard and not at its claim: with the host question answering
// no, that arm's probe returns "" one line in, which is the same string the
// defect produced, and the guard is why it reports "fixture did not satisfy
// the gated lookup" instead of passing over a measurement it never made.
func TestSeatbeltAvailableStaysTheGatedHostQuestion(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("SeatbeltAvailable is false off darwin whatever PATH says")
	}
	sep := string(os.PathListSeparator)
	gates, _ := sbgGatesBin(t)
	empty := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", gates+sep+empty)
	if !SeatbeltAvailable() {
		t.Errorf("SeatbeltAvailable() = false with a sandbox-exec at %s — the HOST question must still answer yes where protocolBin refuses: the pane that types it runs in herdr's process tree, not in one of posse's children (ranger-base-npqem)", gates)
	}

	// The positive control, so the arm above is not satisfied by a function
	// that always says yes: nothing on PATH answers to the name at all.
	t.Setenv("PATH", empty)
	if SeatbeltAvailable() {
		t.Error("SeatbeltAvailable() = true with no sandbox-exec anywhere on PATH — this test's first arm discriminates nothing")
	}
}
