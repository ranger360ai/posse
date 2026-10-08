//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0069: the data ceiling's write-time arm lives in the bd SHIM, not the
// commit hook — so these pins run the rendered `bd` shim as a binary
// (exec.Command, no git repo, no real bd database) rather than committing
// through qaCeilingWall's fixture, which is dataceiling_qa_test.go's own
// subject (the commit hook's REPORTING reader over .beads/*.jsonl).
//
// The fixture vocabulary is qaCeilingWall's own (dataceiling_qa_helpers_test.go):
// QUOKKA/restricted-banner and quokka-export-/export-name, never this box's.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// qaBdCeilingShim renders a bd shim carrying the two fixture ceiling
// classes (or none, when ceiling is false) and points it at a fake "real"
// bd that records every invocation it was actually exec'd with — so a pin
// can tell "refused before exec" from "passed through" without a real bd
// binary or database. The deny rule names a verb ("sync") distinct from
// every argv these pins type, so ParseShimRules renders a bd shim without
// any verb rule refusing the test calls before the ceiling arm is reached.
func qaBdCeilingShim(t *testing.T, ceiling bool) (shimPath, refusalsLog string, execLog func() string) {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	if ceiling {
		cfgPath := filepath.Join(home, "config.yaml")
		if err := os.WriteFile(cfgPath, []byte(qaCeilingCfg), 0o644); err != nil {
			t.Fatal(err)
		}
		a.ConfigPath = cfgPath
		if set := a.OpsPatternSet(); len(set.CeilingRejected) > 0 || len(set.Ceiling) != 2 {
			t.Fatalf("fixture premise: both ceiling patterns must be accepted, got %+v %v", set.Ceiling, set.CeilingRejected)
		}
	}
	realBin := t.TempDir()
	execLogPath := filepath.Join(home, "exec.log")
	if err := os.WriteFile(execLogPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// Appends one line per real invocation — the shim's own exec, never a
	// refusal, which exits before this binary is ever reached.
	WriteExecutable(filepath.Join(realBin, "bd"), []byte(
		"#!/bin/sh\necho \"$@\" >> "+shQuote(execLogPath)+"\necho \"real bd $*\"\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	gatesDir, binDir, _, err := a.RenderGates("ceilingpersona", []string{"Bash(bd sync:*)"})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(binDir, "bd"), filepath.Join(gatesDir, "refusals.log"), func() string {
		b, _ := os.ReadFile(execLogPath)
		return string(b)
	}
}

func qaBdCeilingRun(t *testing.T, shim string, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(shim, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return out.String(), errb.String(), code
}

// VERIFICATION 1 & 6: a hit on the raw argv (not behind -f) is refused,
// class-only — the class and a hit count, never QUOKKA — and bd is never
// exec'd. refusals.log gains one "data ceiling scan [bd shim]" line naming
// no argv at all. `bd search '<banner>'` is explicitly the shape ADR 0069
// D2 calls out (a literal paste typed as a search term), so it is this
// pin's second argv shape rather than a separate one.
func TestQABdCeilingArgvRefusesAndLogsClassOnly(t *testing.T) {
	shim, log, execd := qaBdCeilingShim(t, true)
	for _, args := range [][]string{
		{"comments", "add", "x-1", "x " + qaCeilingHit + " x"},
		{"search", qaCeilingHit},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, errs, code := qaBdCeilingRun(t, shim, "", args...)
			if code != 1 || out != "" {
				t.Fatalf("must refuse with no stdout: code=%d out=%q err=%q", code, out, errs)
			}
			for _, want := range []string{
				"refused by posse gate: bd argv carries data-ceiling content — bd shim, session",
				qaCeilingClass + ": 1 hit(s)",
				"pattern and matched text withheld",
				"ADR 0050 D2",
				"retype the bd command with the system of record's id",
				"ADR 0068",
			} {
				if !strings.Contains(errs, want) {
					t.Errorf("stderr must carry %q:\n%s", want, errs)
				}
			}
			qaNoCeilingVocabulary(t, "stderr", errs)
			if strings.Contains(errs, "RHQ_VISIBILITY_OVERRIDE") || strings.Contains(errs, "override") {
				t.Errorf("D5: no override branch, but stderr mentions one:\n%s", errs)
			}
		})
		if execd() != "" {
			t.Fatalf("bd must never be exec'd on a refusal: %q", execd())
		}
	}
	logb, _ := os.ReadFile(log)
	qaNoCeilingVocabulary(t, "refusals.log", string(logb))
	for _, line := range strings.Split(strings.TrimRight(string(logb), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, "data ceiling scan [bd shim] (bd argv) session") {
			t.Errorf("refusals.log line in the wrong shape: %q", line)
		}
	}
	if n := strings.Count(string(logb), "\n"); n != 2 {
		t.Errorf("refusals.log: want exactly 2 lines (one per refusal above), got %d:\n%s", n, logb)
	}
}

// VERIFICATION 2: the same text behind -f, --file, --body-file and
// --body-file=<path> (both spellings) is refused the same way, and the
// file itself is untouched.
func TestQABdCeilingRefusesRegularFilesBothSpellings(t *testing.T) {
	shim, _, execd := qaBdCeilingShim(t, true)
	dir := t.TempDir()
	body := "pasted: " + qaCeilingHit + " — header\n"
	path := filepath.Join(dir, "comment.txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"comments", "add", "x-1", "-f", path},
		{"comments", "add", "x-1", "--file", path},
		{"comments", "add", "x-1", "--body-file", path},
		{"comments", "add", "x-1", "--body-file=" + path},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, errs, code := qaBdCeilingRun(t, shim, "", args...)
			if code != 1 || out != "" || !strings.Contains(errs, qaCeilingClass+": 1 hit(s)") {
				t.Fatalf("must refuse over the file's content: code=%d out=%q err=%q", code, out, errs)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != body {
				t.Errorf("the file must be untouched: err=%v got=%q", err, got)
			}
		})
	}
	if execd() != "" {
		t.Fatalf("bd must never be exec'd on any of these refusals: %q", execd())
	}
}

// VERIFICATION 4: a file named /dev/stdin, backed by a REGULAR file
// redirect, is never opened — the arm skips it by the /dev/ prefix before
// any `[ -f ]` test, so the call passes through to bd untouched and the
// content never shows up in a refusal. The census shape is `-f /dev/stdin
// < in`; exec.Cmd has no shell redirection operator, so this pin wires
// cmd.Stdin to the same fd Go hands the child as fd 0, which is what the
// shell's `< in` does under the hood.
func TestQABdCeilingNeverOpensDevStdin(t *testing.T) {
	shim, _, execd := qaBdCeilingShim(t, true)
	dir := t.TempDir()
	path := filepath.Join(dir, "in")
	body := qaCeilingHit + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cmd := exec.Command(shim, "comments", "add", "x-1", "-f", "/dev/stdin")
	cmd.Stdin = f
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("must pass through, not refuse: %v\nstderr=%s", err, errb.String())
	}
	if strings.Contains(errb.String(), "refused by posse gate") {
		t.Errorf("/dev/stdin must never be opened by this arm — it is D6's residual, caught at commit time instead:\n%s", errb.String())
	}
	if execd() == "" {
		t.Error("the call must have reached the real bd — nothing here should have refused it")
	}
}

// VERIFICATION 5: an instance with no ceiling configured renders a bd shim
// carrying none of this arm at all — not merely one that happens not to
// fire. Checked by ABSENCE of the arm's own markers, which is the direct
// claim ("renders nothing"), and by comparing the WITH-ceiling shim against
// the WITHOUT one: the only diff is this arm.
func TestQABdCeilingKeyAbsentRendersNothing(t *testing.T) {
	without, _, _ := qaBdCeilingShim(t, false)
	withCeiling, _, _ := qaBdCeilingShim(t, true)
	plain, err := os.ReadFile(without)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"ADR 0069", "posse_bd_ceiling_files", "posse_check()", "data ceiling scan"} {
		if strings.Contains(string(plain), marker) {
			t.Errorf("no ceiling configured, but the bd shim carries %q:\n%s", marker, plain)
		}
	}
	loaded, err := os.ReadFile(withCeiling)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loaded), "posse_bd_ceiling_files") {
		t.Errorf("a configured ceiling must render the arm, and it did not:\n%s", loaded)
	}
	// Same render, modulo the persona name and the paths rendered into the
	// header/RHQ_GATE_LOG lines (both shims are rendered under their own
	// t.TempDir()) — the two bodies must diverge starting exactly where the
	// ceiling arm begins and nowhere before it.
	pIdx := strings.Index(string(plain), "exec '")
	lIdx := strings.Index(string(loaded), "ADR 0069")
	if pIdx < 0 || lIdx < 0 || lIdx >= strings.Index(string(loaded), "exec '") {
		t.Errorf("the ceiling arm must sit before the exec line in the loaded shim, and the plain shim must have no such marker at all")
	}
}

// VERIFICATION 7: 20 bd calls through a rendered ceiling arm cost under 1s
// wall — the prototype's own measured range (0.30-0.48s, ADR 0069 Context)
// on an otherwise-idle box. This pin's own budget is 5s rather than 1s
// (credpinwire_qa_test.go's own generous bound, same reason): `make test`
// runs this alongside two other suite arms competing for the same cores,
// and a bound tight enough to pin the prototype's number is a bound that
// flakes under that contention (MEASURED: 1.29s for this same loop during
// a three-arm `make test` run) without catching anything a 5s bound would
// miss — the arm being guarded against is an accidental O(n^2) or a lost
// short-circuit, not a few hundred milliseconds of scheduler noise.
func TestQABdCeilingTwentyCallsFastEnough(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	shim, _, _ := qaBdCeilingShim(t, true)
	start := time.Now()
	for i := 0; i < 20; i++ {
		if _, _, code := qaBdCeilingRun(t, shim, "", "list", "--limit", "1"); code != 0 {
			t.Fatalf("call %d must pass through clean", i)
		}
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("20 bd calls through the ceiling arm took %s, want well under 5s", d)
	}
}

// A clean call with no ceiling hit anywhere passes straight through to bd,
// exactly as it did before this arm existed.
func TestQABdCeilingCleanCallPassesThrough(t *testing.T) {
	shim, log, execd := qaBdCeilingShim(t, true)
	out, errs, code := qaBdCeilingRun(t, shim, "", "show", "x-1")
	if code != 0 || !strings.Contains(out, "real bd show x-1") || errs != "" {
		t.Errorf("clean call must pass through untouched: code=%d out=%q err=%q", code, out, errs)
	}
	if execd() == "" {
		t.Error("bd must have been exec'd")
	}
	if logb, _ := os.ReadFile(log); strings.Contains(string(logb), "data ceiling") {
		t.Errorf("a clean call must not log anything: %q", logb)
	}
}
