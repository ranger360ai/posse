package main

// QA, ranger-base-6s5mo (fix half of ranger-base-atq78) — the bead's own
// repro, through the built binary, because the bytes that broke a live seat
// are only readable through the command.
//
//	$ sed -n '160p' $RHQ_HOME/state/gates/<p>/bin/git
//	[ "${HOME+x}:${HOME-}" = 'x:/home/posse' ] || posse_hs_gm=1
//	$ HOME=/tmp/somewhere posse gates <p>      # exit 0, no output line
//	$ sed -n '160p' $RHQ_HOME/state/gates/<p>/bin/git
//	[ "${HOME+x}:${HOME-}" = 'x:/tmp/somewhere' ] || posse_hs_gm=1
//	$ git -C $(mktemp -d) init -q -b main      # from the SEAT's own shell
//	refused by posse gate: ... (rule: hookskip:config-env)
//
// That shim's hook-skip arm bakes the launcher's HOME in as a literal and
// refuses any git whose HOME is not it (ADR 0079 D1, ranger-base-r3pri). The
// report re-rendered the shims as a side effect of being asked to describe
// them, from the environment of whoever typed the command — this seat, a
// sibling seat with `posse gates <other>`, or an operator in another shell —
// so a RUNNING seat's git was refused by its own gate from then on: 26 FAIL
// in that seat's `make test`, 24 of them fixtures' own `git init`, exit 0,
// and nothing in the output saying anything had been written (MEASURED
// 2026-11-11, ranger-base-atq78).
//
// THE RULE NOW: a report may CREATE a wall that is not there and never
// REPLACES one that is. The first render stays, because the reason to render
// at all is that a report whose head line names a file that is not there is
// worse than no report; what a second run does is READ, which is also the
// true answer — on-disk is what a live session is running behind.
//
// Driven through the binary beside this command's other three pins
// (gatesreadonly_qa_test.go, gatesruntimemissing_qa_test.go,
// gateshellinherit_qa_test.go): both halves live in the `gates` case.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// grwHome writes a persona denying the two verbs every crew PID denies, so
// the render has shims to make, and returns RHQ_HOME.
func grwHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// `cage: seatbelt` so the WALL writers are on this path too and not only
	// the shim renderer: the report's own render (BwrapReport /
	// SeatbeltReport) and parity's two probes, which are reached once per
	// runtime in the catalog. Without it this host renders no wall for the
	// persona and the second arm below asserts over a path nothing ever
	// wrote.
	pid := "---\nname: builder\ndescription: builder\nruntime: claude\ncage: seatbelt\n" +
		"deny: [\"Bash(pkill:*)\", \"Bash(killall:*)\"]\n---\nbuilder\n"
	if err := os.WriteFile(filepath.Join(home, "agents", "builder.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

// grwRun types `posse gates builder` with HOME set to `home`, the way the
// bead's step 2 does, and returns the output and the exit code.
func grwRun(t *testing.T, bin, rhq, home string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, "gates", "builder")
	// A cwd with a store of record in it, so parity's reachability row has
	// something to reach and therefore RENDERS the wall and applies it
	// (reachability.go). That probe is one of the four writers this bead
	// found on the report path; a cwd with no store abstains before the
	// render and the arms would pass over a probe that never ran.
	cmd.Dir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(cmd.Dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmd.Dir, ".beads", "issues.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd.Env = []string{"HOME=" + home, "RHQ_HOME=" + rhq, "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse gates builder: %v\n%s", err, out)
	}
	return string(out), code
}

// grwTree is every file under the persona's gates dir, by content, so an arm
// can say "nothing here changed" rather than "this one file did not". The
// logs are left out: refusals.log and shell.log are a live seat's own
// append-only trail and are the two files this command may legitimately find
// longer than it left them.
func grwTree(t *testing.T, rhq string) map[string]string {
	t.Helper()
	root := filepath.Join(rhq, "state", "gates", "builder")
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case fi.IsDir():
			return nil
		case strings.HasSuffix(p, ".log"):
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("no wall under %s, so an arm comparing it before and after measures nothing", root)
	}
	return out
}

// Acceptance pin 1: render a wall, then run the report from a DIFFERENT HOME
// and the wall is byte-identical.
func TestQAGatesReportDoesNotRewriteALiveWallFromTheCallersEnvironment(t *testing.T) {
	bin := buildRhq(t)
	rhq := grwHome(t)
	launchHome := t.TempDir()

	// The first run is the render — the wall a seat would then be launched
	// behind. Load-bearing twice: it makes the dir the arm below reads, and
	// it proves the report still says when it wrote one.
	first, code := grwRun(t, bin, rhq, launchHome)
	if code != 0 {
		t.Fatalf("the first `posse gates builder` exited %d, so the arm below would measure the wrong failure:\n%s", code, first)
	}
	if !strings.Contains(first, "WROTE this dir just now") {
		t.Errorf("the render that DID write the gates dir did not say so. Until this bead the command wrote it on every run and printed nothing at all about it, which is half of why the defect went unnoticed:\n%s", first)
	}
	before := grwTree(t, rhq)
	shim := filepath.Join(rhq, "state", "gates", "builder", "bin", "git")
	baked, err := os.ReadFile(shim)
	if err != nil {
		t.Fatalf("the render wrote no git shim, and the git shim is what the defect broke: %v", err)
	}
	// The literal the bead's step 1 reads. Named here so the arm below cannot
	// pass over a shim that bakes nothing — if this ever stops being true the
	// hook-skip arm has changed and this pin has to be re-read, not relaxed.
	if !strings.Contains(string(baked), "'x:"+launchHome+"'") {
		t.Fatalf("the rendered git shim does not bake the launching HOME %s in as a literal, so a re-render from another HOME would change nothing and every arm here is vacuous (ADR 0079 D1, rule hookskip:config-env):\n%s", launchHome, baked)
	}

	// The bead's step 2, from a HOME that is not the one the wall was
	// rendered with — the same shape whether the typist is this seat, a
	// sibling seat or the operator.
	other := t.TempDir()
	out, code := grwRun(t, bin, rhq, other)
	if code != 0 {
		t.Errorf("`HOME=<other> posse gates builder` exited %d — reading a wall must not fail:\n%s", code, out)
	}
	if got, err := os.ReadFile(shim); err != nil {
		t.Errorf("the report REMOVED the git shim (%v) — a live seat's every git runs through that file", err)
	} else if string(got) != string(baked) {
		t.Errorf("the report REWROTE the git shim from the caller's environment. Every git the live seat runs is then refused by its own gate (rule hookskip:config-env): 26 FAIL in that seat's suite, 24 of them fixtures' own `git init` (ranger-base-atq78). The shim now bakes:\n%s", grwBakedLine(string(got)))
	}
	// ...and not only that one file.
	after := grwTree(t, rhq)
	for name, sum := range before {
		switch got, ok := after[name]; {
		case !ok:
			t.Errorf("the report removed %s from the live gates dir", name)
		case got != sum:
			t.Errorf("the report rewrote %s in the live gates dir. Only a launch writes there — a reader renders from its own environment, and what it replaces is what a running seat is behind (ranger-base-6s5mo)", name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			t.Errorf("the report added %s to the live gates dir", name)
		}
	}

	// And the report still describes the wall it did not write: the shims the
	// verify recipes grep for, and a line saying where the bytes came from.
	for _, want := range []string{"bin/pkill", "bin/killall", "NOT re-rendered"} {
		if !strings.Contains(out, want) {
			t.Errorf("the second report does not carry %q, so `posse gates <persona> | grep -E 'pkill|killall'` answers nothing and nobody is told the shims were read rather than rendered:\n%s", want, out)
		}
	}
}

// grwBakedLine is the one line of a shim worth printing in a failure: the
// HOME literal the hook-skip arm compares against.
func grwBakedLine(shim string) string {
	for _, ln := range strings.Split(shim, "\n") {
		if strings.Contains(ln, `"${HOME+x}:${HOME-}"`) {
			return strings.TrimSpace(ln)
		}
	}
	return "(no HOME literal in the shim at all)"
}

// Acceptance pin 3, as far as the REPORT's own render reaches: the host-side
// prep bwrap needs is a write outside the namespace in the seat's name, made
// from the caller's environment, and a command whose job is to describe the
// wall makes none of it.
//
// This arm is the REPORT's own render, which is all ranger-base-6s5mo
// closed: parity's two probes (applyRecordReach, applyWallWitness) render
// AND APPLY, so they kept the prep and still made it in whatever HOME typed
// the command — once per runtime in the catalog. That residual is
// ranger-base-qd9ei, and the arm below it is the one that says the whole
// command writes nothing there.
func TestQAGatesReportMakesNoHostPrepOfItsOwn(t *testing.T) {
	bin := buildRhq(t)
	rhq := grwHome(t)
	launchHome := t.TempDir()
	if _, code := grwRun(t, bin, rhq, launchHome); code != 0 {
		t.Fatalf("the first render exited %d", code)
	}
	// A wall is on disk now, so this run renders nothing at all — which is
	// the whole claim: the one file a bwrap launch would write here is the
	// artifact, and a reader does not.
	other := t.TempDir()
	if _, code := grwRun(t, bin, rhq, other); code != 0 {
		t.Fatalf("the reading run exited %d", code)
	}
	for _, name := range []string{"bwrap.args", "bwrap.txt", "seatbelt.sb"} {
		p := filepath.Join(rhq, "state", "gates", "builder", name)
		if _, err := os.Stat(p); err == nil {
			t.Errorf("a report wrote %s. The wall a launch reads is the launch's to render — a reader's copy goes under its own dir and is named on the head line (ranger-base-6s5mo)", p)
		}
	}
}

// ranger-base-qd9ei, the residual, through the same binary: `posse gates
// <persona>` makes NO path in the HOME it was typed from — not in its own
// render, and not in parity's two probes either.
//
// THE BEAD'S OWN MEASUREMENT, as an arm. MEASURED 2026-11-12 from a
// bwrap-caged seat with `HOME=<fresh empty dir>`: `.bob`,
// `.cache/posse/plan-usage`, `.cache/posse-seat/<session>`,
// `.config/go/telemetry/local` and 24 members of `.grok` created in that
// HOME — a mkdir or an O_EXCL touch per path this PID's grants name, made
// once per runtime in the catalog because that is how often the parity table
// renders. `~/.grok/config.toml` among them, seeded EMPTY, which posse's own
// project-config classifier then reads as `invalid JSON: unexpected end of
// JSON input` and refuses a later launch in that home over
// (ranger-base-f67jx, docs/notes.d/ranger-base-zibko.md).
//
// A WHOLE-TREE READING AND NOT A LIST OF NAMES (ADR 0078's rule): the paths
// are a function of the catalog this instance ships and of every grant every
// runtime declares, so an arm naming `.grok` would go quiet the day a
// runtime is renamed. What is pinned is the count: a fresh HOME is empty
// before and empty after.
//
// TWO RUNS, because they are different claims. The first renders the wall
// and the shims (nothing on disk yet); the second finds them there and
// reads. Either could have been the one that prepped.
func TestQAGatesReportMakesNoPathInTheHomeItWasTypedFrom(t *testing.T) {
	bin := buildRhq(t)
	rhq := grwHome(t)

	for _, tc := range []struct{ what string }{
		{"the run that renders the wall"},
		{"the run that reads it back"},
	} {
		home := t.TempDir()
		if left := grwTreeUnder(t, home); len(left) != 0 {
			t.Fatalf("%s: the fixture HOME is not empty to begin with, so this arm measures nothing:\n  %s", tc.what, strings.Join(left, "\n  "))
		}
		out, code := grwRun(t, bin, rhq, home)
		if code != 0 {
			t.Fatalf("%s: `posse gates builder` exited %d — reading a wall must not fail:\n%s", tc.what, code, out)
		}
		if left := grwTreeUnder(t, home); len(left) != 0 {
			t.Errorf("%s: the command made %d path(s) in the HOME it was typed from:\n  %s\nThe host-side prep is a write outside the namespace in the seat's name, from whatever environment typed the command. Only a LAUNCH makes it; a reader's probe finds it already there or says it measured nothing (ranger-base-qd9ei)", tc.what, len(left), strings.Join(left, "\n  "))
		}
		// And the command is not thereby empty: it still printed the parity
		// table for every runtime, which is where the two probes live. An
		// arm whose subject is "it wrote nothing" passes for free over a
		// command that did nothing.
		if !strings.Contains(out, "cage") {
			t.Errorf("%s: no parity block in the output, so the arm above passes over a command that printed no wall at all:\n%s", tc.what, out)
		}
	}
}

// grwTreeUnder is every path under a directory, so an arm can say "nothing
// was made in here" rather than "not the one path I thought of".
func grwTreeUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != root {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
