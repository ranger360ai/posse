//go:build posse_arm3

package posse

// The GIT_EXTERNAL_DIFF row, for ranger-base-csfbj. Found verifying
// ranger-base-nn161 on this box; the reader half went to the code lane as
// ranger-base-xw51s and has landed.
//
// THE DEFECT THIS PINS. The row's own disclosure said the opposite of what
// the row does, and it named its own blind spot in the same sentence:
//
//	GIT_EXTERNAL_DIFF=""    attack ran a marker script as the diff driver of
//	                        `git diff HEAD~1`; "" quiet and --shortstat
//	                        identical to unset.
//
// The attack arm was real. The CLEARING arm was `--shortstat`, which is one
// of the formats that never reaches a diff driver at all — so it could not
// have shown a cost whatever the value was. Every summary format is quiet and
// the patch formats die rc 128 `error: cannot run :`, because git does not
// read set-but-empty as unset: it execs `""`. That is the GIT_SSH_COMMAND=""
// trap, which this table warns about two rows up and then walked into.
//
// WHAT THIS FILE HOLDS, and why it is two properties and not one.
//
//   - The BEHAVIOUR, by execution, both directions. A summary-format arm that
//     is byte-identical under the pin, a patch-format arm that dies under it,
//     and a marker-driver census that grades INVOCATION rather than the exit
//     code of `""` — because "the empty command happened to be harmless" and
//     "the driver never ran" are different facts, and only the second one is
//     a reason a format is safe. The census carries its own failing wrong arm
//     (`--ext-diff`, which turns the log family's default back on), so a
//     green here cannot mean the marker was never reachable.
//
//   - The DISCLOSURE, held to a contract that survives either answer to
//     ranger-base-5sph1: the row is pinned WITH its cost readable in the row,
//     or it is absent from BOTH ends and named in `ALSO NOT COVERED` with a
//     reason. What it may never be again is pinned while reading neutral.
//
// THE DECISION WAS NOT HERE, AND IT HAS BEEN MADE. Keeping the row at the
// price of `git diff` patch format in every posse-launched seat, versus
// dropping it and accepting the inlet the way ranger-base-37y0z already
// accepts GIT_CONFIG_COUNT and GIT_CONFIG_GLOBAL, was the operator's on
// ranger-base-5sph1. They ruled 2026-09-06 to DROP it, and ranger-base-888fv
// applied that at both ends: the row is out of inletPin() and out of
// etc/claude/managed-settings.d/10-posse-inlet-pin.json (23 keys to 22), and
// the measurement moved into inletpin.go's ALSO NOT COVERED paragraph beside
// GIT_CONFIG_GLOBAL. So the contract test below now runs its NOT-PINNED arm,
// which is the branch that had never executed against a real absence.
//
// The file is left written for BOTH answers deliberately. The two behaviour
// tests are unconditional and do not care which way it went — they measure
// git, not posse — and the disclosure test still fails under the third thing
// a future edit could produce: a row that costs something and does not say
// so, or a gap that is silent about being a gap. Re-pinning this name is a
// legitimate future call and it does not need this file rewritten to make it.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// extDiffPatchForms invoke a diff driver, so `""` is rc 128 on each. Measured
// 2026-09-06, git 2.50.1 (Apple Git-155), darwin 25.4.0.
var extDiffPatchForms = [][]string{
	{"diff"},
	{"diff", "--", "f"},
	{"diff", "--cached"},
	{"diff", "HEAD"},
	{"diff", "-U0"},
	{"diff", "--exit-code"},
}

// extDiffQuietForms never reach a driver, so `""` is byte-identical to unset
// on each. The first entry is the one that matters historically: --shortstat
// is what cleared the row, and it is in this list, not the one above.
var extDiffQuietForms = [][]string{
	{"diff", "--shortstat"},
	{"diff", "--stat"},
	{"diff", "--numstat"},
	{"diff", "--name-only"},
	{"diff", "--name-status"},
	{"diff", "--raw"},
	{"diff", "--check"},
	{"diff", "--no-ext-diff"},
	{"show", "HEAD"},
	{"log", "-p", "-1"},
	{"format-patch", "-1", "--stdout"},
	{"diff-tree", "-p", "HEAD"},
	{"diff-index", "-p", "HEAD"},
}

// TestQATheEmptyExternalDiffValueIsNotNeutral is the measurement the row was
// cleared without. It fails if git ever stops executing the empty value — at
// which point the row's prose is what needs rewriting, not this test.
func TestQATheEmptyExternalDiffValueIsNotNeutral(t *testing.T) {
	t.Parallel()
	repo := extDiffProbeRepo(t)

	// Both arms of every form, run from an environment with this name and
	// the GIT_CONFIG_* family taken out: seats used to carry the pin itself,
	// the operator's own environment may carry the name at any time now that
	// posse does not pin it, and either way an ambient value would sit
	// underneath every arm and make the "unset" column a lie.
	base := extDiffScrubbedEnv()
	arms := func(t *testing.T, argv []string) (unsetOut, emptyOut string, unsetRC, emptyRC int) {
		t.Helper()
		unsetOut, unsetRC = extDiffRun(repo, base, argv)
		emptyOut, emptyRC = extDiffRun(repo, append(base, "GIT_EXTERNAL_DIFF="), argv)
		return
	}

	for _, argv := range extDiffPatchForms {
		out, _, urc, erc := arms(t, argv)
		if urc != 0 && !(argv[len(argv)-1] == "--exit-code" && urc == 1) {
			t.Fatalf("RIG DEAD: `git %s` is rc %d with the variable UNSET (%s) — the fixture, not the pin; every arm below would false-pass", strings.Join(argv, " "), urc, strings.TrimSpace(out))
		}
		if erc == urc {
			t.Errorf("`git %s` is rc %d under GIT_EXTERNAL_DIFF=\"\" as well as unset. It is listed as a PATCH form, which runs the driver — if git has stopped executing the empty value, the GIT_EXTERNAL_DIFF row in inletpin.go is now a paragraph that lies and this list is stale (ranger-base-csfbj)", strings.Join(argv, " "), erc)
		}
	}

	for _, argv := range extDiffQuietForms {
		uo, eo, urc, erc := arms(t, argv)
		if urc != 0 {
			t.Fatalf("RIG DEAD: `git %s` is rc %d with the variable UNSET (%s)", strings.Join(argv, " "), urc, strings.TrimSpace(uo))
		}
		if erc != urc || eo != uo {
			t.Errorf("`git %s` is NOT identical under GIT_EXTERNAL_DIFF=\"\" (rc %d vs %d). It is listed as a form that never reaches a driver — the row's format table is what has to move (ranger-base-csfbj)", strings.Join(argv, " "), erc, urc)
		}
	}
}

// TestQAOnlySomeGitDiffFormsInvokeTheDriver grades INVOCATION with a real
// driver, which is the arm the original measurement needed and did not have.
// `""` being quiet on `--shortstat` says nothing about the pin; the driver not
// being RUN on `--shortstat` is the fact underneath it, and it is the reason a
// count of what this row breaks has to grade the format and not the verb.
func TestQAOnlySomeGitDiffFormsInvokeTheDriver(t *testing.T) {
	t.Parallel()
	repo := extDiffProbeRepo(t)
	log := filepath.Join(t.TempDir(), "calls")
	driver := extDiffMarkerDriver(t, log)
	base := append(extDiffScrubbedEnv(), "GIT_EXTERNAL_DIFF="+driver)

	invoked := func(t *testing.T, argv ...string) bool {
		t.Helper()
		_ = os.Remove(log)
		extDiffRun(repo, base, argv)
		_, err := os.Stat(log)
		return err == nil
	}

	// The failing wrong arm, first: --ext-diff turns the log family's default
	// back on. Without it, every "no" below could equally mean the marker was
	// never reachable from this fixture at all.
	if !invoked(t, "log", "-p", "--ext-diff", "-1") {
		t.Fatal("CONTROL FAILED: `git log -p --ext-diff -1` did not run the driver, so no arm in this test can tell an immune format from a broken fixture")
	}

	for _, argv := range extDiffPatchForms {
		if !invoked(t, argv...) {
			t.Errorf("`git %s` did not run the driver. It is listed as a patch form, and the row in inletpin.go prices the pin on exactly this list", strings.Join(argv, " "))
		}
	}
	for _, argv := range extDiffQuietForms {
		if invoked(t, argv...) {
			t.Errorf("`git %s` RAN the driver. It is listed as immune, which is what makes it safe to clear a row with — and `--shortstat` being on that list wrongly is how this row got its measurement wrong the first time (ranger-base-csfbj)", strings.Join(argv, " "))
		}
	}
}

// TestQATheExternalDiffRowIsPinnedWithItsCostOrNotPinnedAtAll is the
// disclosure contract, written to pass under either answer to
// ranger-base-5sph1 and to fail under the state that was shipped.
func TestQATheExternalDiffRowIsPinnedWithItsCostOrNotPinnedAtAll(t *testing.T) {
	t.Parallel()
	const key = "GIT_EXTERNAL_DIFF"

	var pinnedHere bool
	for _, v := range inletPin() {
		if v.Key == key {
			pinnedHere = true
			if v.Value != "" {
				t.Errorf("inletPin()[%s] = %q. There is no non-empty neutral: any value is EXECUTED as the diff driver, and git's \"use the internal diff\" is the --no-ext-diff FLAG, which an env pin cannot supply. A non-empty value here runs that program on every `git diff` in the fleet", key, v.Value)
			}
		}
	}

	const path = "../../etc/claude/managed-settings.d/10-posse-inlet-pin.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the policy-tier half of the pin is missing: %v", err)
	}
	var dropIn struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(b, &dropIn); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	if _, inDropIn := dropIn.Env[key]; inDropIn != pinnedHere {
		t.Errorf("%s is in inletPin()=%v but in the drop-in=%v. The two ends are held equal by TestQAThePolicyDropInMatchesTheInletPin; whichever way ranger-base-5sph1 goes, it goes at both ends", key, pinnedHere, inDropIn)
	}

	src, err := os.ReadFile("inletpin.go")
	if err != nil {
		t.Fatalf("cannot read the table to check its own disclosure: %v", err)
	}
	text := string(src)

	if !pinnedHere {
		// Option B: dropped. Then it owes the file the same thing
		// GIT_CONFIG_GLOBAL owes it — a named gap with a measured reason.
		//
		// Bounded at `func inletPin(` and not read to EOF, which is what it
		// did when only the Option-A arm had ever run: the table's own body
		// comment names the three uncovered git variables so a reader of the
		// code sees them, and a region running to EOF counts that mention as
		// the disclosure. Measured under this change (ranger-base-888fv):
		// with the whole measured bullet renamed out of the paragraph and
		// only the body comment's mention left, the EOF-bounded version was
		// GREEN. The disclosure this test is for is the MEASUREMENT, not the
		// name appearing somewhere in the file.
		const marker, end = "ALSO NOT COVERED", "func inletPin("
		i := strings.Index(text, marker)
		if i < 0 {
			t.Fatalf("inletpin.go has no %q paragraph", marker)
		}
		j := strings.Index(text, end)
		if j <= i {
			t.Fatalf("inletpin.go has no %q after its %q paragraph — the region this arm reads is gone, so it is measuring nothing; re-anchor it before trusting a green", end, marker)
		}
		if !strings.Contains(text[i:j], key) {
			t.Errorf("%s is pinned at neither end and is not named in the %q paragraph either. The file's contract is that a name which is not in the table is READABLE as not covered — dropping the row and saying nothing is the same defect as pinning it and saying nothing (ranger-base-csfbj)", key, marker)
		}
		return
	}

	// Option A: kept. Then the row has to price it, inside the row and not
	// merely somewhere in the file — anchoring is the point, because the
	// sentence that was wrong was IN the row.
	const rowHead, nextRow = `GIT_EXTERNAL_DIFF=""`, `GIT_PAGER=""`
	i, j := strings.Index(text, rowHead), strings.Index(text, nextRow)
	if i < 0 || j <= i {
		t.Fatalf("inletpin.go has no %q row followed by the %q row — the region this test reads is gone, so it is measuring nothing; re-anchor it before trusting a green", rowHead, nextRow)
	}
	// Normalised before it is scanned: this row is a wrapped comment, so a
	// phrase that spans two lines is invisible to a raw Contains and the
	// retracted claim below was itself wrapped when it shipped.
	row := extDiffFlatten(text[i:j])

	// The retracted claim, in the two shapes it shipped in, and the exact
	// reason each was wrong. Saying either again is the defect returning,
	// not a rewording of it. Kept narrow deliberately: "byte-identical to
	// unset" is the TRUE half of the corrected row, describing the formats
	// that never reach a driver, and a blunter ban would red the fix.
	for _, banned := range []string{`"" quiet`, "--shortstat identical to unset"} {
		if strings.Contains(row, banned) {
			t.Errorf("the %s row still says %q. It is not quiet: `git diff` in patch format dies rc 128 `error: cannot run :` under it, and `--shortstat` — the arm that cleared it — is one of the forms that never reaches a driver at all (ranger-base-csfbj)", key, banned)
		}
	}
	// What a reader of this row has to be able to see, having been told the
	// wrong thing once: that it is not neutral, which formats pay for it,
	// which scope it is effective at, and who owns the open call.
	for _, want := range []string{
		"NOT NEUTRAL",
		"--shortstat",
		"--no-ext-diff",
		"flag-scope-effective",
		"ranger-base-5sph1",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("the %s row does not say %q. The row costs every posse-launched seat its `git diff` patch output, and the first version of it read as free — a reader has to be able to see the price, the formats that pay it, and that keeping it is an open operator call (ranger-base-csfbj)", key, want)
		}
	}
}

// extDiffFlatten strips the `//\t` comment prefix and collapses runs of
// whitespace, so a phrase the row wraps across two lines reads as one string.
// A per-line scan of a hanging-indent table is blind to exactly the sentences
// this test exists to catch.
func extDiffFlatten(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "//", " ")), " ")
}

// ─── fixtures ────────────────────────────────────────────────────────────────

// extDiffScrubbedEnv drops this name and the GIT_CONFIG_* family: the arms
// supply their own, and a seat carries the pin under test in its own
// environment.
func extDiffScrubbedEnv() []string {
	var out []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if k == "GIT_EXTERNAL_DIFF" || strings.HasPrefix(k, "GIT_CONFIG") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// extDiffRun runs one git form and returns its combined output and exit code.
func extDiffRun(repo string, env, argv []string) (string, int) {
	c := exec.Command("git", argv...)
	c.Dir, c.Env = repo, env
	out, err := c.CombinedOutput()
	if c.ProcessState == nil {
		// git never started. -1 matches no arm's expectation, so a caller
		// reads it as a difference rather than as a quiet zero.
		return string(out) + " [git did not start: " + err.Error() + "]", -1
	}
	return string(out), c.ProcessState.ExitCode()
}

// extDiffMarkerDriver is a diff driver that records having been run and exits
// 0 — so an arm reads "was it invoked", never "did the empty command happen
// to be harmless".
func extDiffMarkerDriver(t *testing.T, log string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "marker-driver")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho ran >> "+log+"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// extDiffProbeRepo has two commits, one unstaged change and one STAGED change
// — the staged half so `--cached` has content, which is a form that reads
// quiet on an empty index whatever the variable says.
func extDiffProbeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir, c.Env = dir, extDiffScrubbedEnv()
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", ".")
	run("config", "user.email", "qa@example.invalid")
	run("config", "user.name", "qa")
	write("f", "a\nb\nc\n")
	write("g", "x\n")
	run("add", "--", "f", "g")
	run("commit", "-q", "-m", "one", "--", "f", "g")
	write("f", "a\nB\nc\n")
	run("add", "--", "f")
	run("commit", "-q", "-m", "two", "--", "f")

	write("f", "a\nZ\nc\n") // unstaged, for the worktree forms
	write("g", "y\n")
	run("add", "--", "g") // staged, for --cached

	// The fixture has to bite before anything leans on it: an empty index or
	// a clean worktree makes every form below quiet for the wrong reason.
	for _, argv := range [][]string{{"diff", "--name-only"}, {"diff", "--cached", "--name-only"}} {
		if out, rc := extDiffRun(dir, extDiffScrubbedEnv(), argv); rc != 0 || strings.TrimSpace(out) == "" {
			t.Fatalf("RIG DEAD: `git %s` is empty (rc %d) in the probe repo, so every arm would pass on nothing to diff", strings.Join(argv, " "), rc)
		}
	}
	return dir
}
