package main

// LIVE DEFECT, pinned GREEN. Filed while verifying the close of
// ranger-base-83rv under ranger-base-w7h58, and refiled as its own bead.
//
// ranger-base-83rv's root cause was "this test builds its exec env from
// os.Getenv("PATH"), so a persona's own gate shim intercepts git". The
// close scrubbed qaSh (and an earlier commit scrubbed the add/ci probe),
// and reasoned the rest out of scope because those sites "run the installed
// hook files by absolute path, so the caller's PATH doesn't reach git
// there". Measured against this tree's HEAD with a logging `git` planted in
// a gates-shaped dir at the head of PATH, one run of
// TestQAInstallRefusalPrescriptionIsRunnable and
// TestQAInstallHooksChainFlagTakesOverBdsShim sent 20 git invocations
// through it — `init`, `config user.email`, `rev-parse --git-dir`,
// `--git-common-dir`, `--git-path hooks`, `--show-toplevel`, `hash-object`
// and six `diff --cached` forms — because the installed hook scripts run
// bare `git` themselves, under the raw PATH those two exec sites hand them.
// With that shim refusing instead of logging, both tests fail outright.
//
// So the leak the bead names is still open. This pin measures the cheapest
// and most direct of those sites — qaForeignBoth's own `git init`, a bare
// name with no cmd.Env at all — and goes RED the day it is scrubbed, which
// is the day to delete this file and let the suite's own hermeticity speak.
//
// DECLINED, not owed: ranger-base-1xln6 (this leak, refiled) was closed "not
// doing" by the operator triage sweep of 2026-09-04 — "test hygiene
// at three QA sites with no observed failure; not a defect". Re-measured at
// 374d3b8 under ranger-base-09yjv, and the reason holds for a reason worth
// writing down: a full GREEN `cmd/posse` run pushes 284 git invocations
// through a gates-shaped PATH element (136 rev-parse, 34 diff, 18 log, 17
// config, 12 init, 12 hash-object, … and two `git commit`), and every one of
// those argv forms is ALLOWED by the crew's rendered gate — the two commits
// are path-limited, which is exactly what `Bash(git commit unless --)`
// permits. The leak costs nothing while that stays true; the day a crew deny
// covers one of those verbs, the three sites go red or, worse, green on a
// refusal the probes cannot tell from the gate chain's own.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAGateschainSetupStillResolvesGitThroughTheCallersPath(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	// A gates-shaped directory: PathOutsideGates drops any PATH element
	// containing a "/gates/" segment, so a shim planted here is invisible
	// to every exec site that scrubs and reachable by every one that does
	// not. That is the whole discriminator.
	plant := filepath.Join(t.TempDir(), "state", "gates", "probe", "bin")
	if err := os.MkdirAll(plant, 0o755); err != nil {
		t.Fatal(err)
	}
	seen := filepath.Join(t.TempDir(), "git-calls.log")
	real := gitOutsideGates(t) // the honest binary, resolved past the gates
	shim := "#!/bin/sh\necho \"$*\" >> " + seen + "\nexec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(plant, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", plant+string(os.PathListSeparator)+os.Getenv("PATH"))

	qaForeignBoth(t)

	b, err := os.ReadFile(seen)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("FIXED: nothing in this file's setup resolves git through the caller's PATH any more — delete this pin (ranger-base-83rv)")
		}
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "init -q -b main") {
		t.Errorf("FIXED: the setup's `git init` no longer resolves through the caller's PATH — delete this pin (ranger-base-83rv). Calls seen:\n%s", b)
	}
}
