package treepins

// QA pins for ranger-base-lle18 — the policy-tier drop-in staleness control.
//
// THE DEFECT, and it is worth stating precisely because the reasoning that
// produced it was careful. `etc/claude/managed-settings.d/*.json` in this repo
// is SOURCE; the files that pin the box are the root-owned copies the operator
// installs by hand, per change (inletpin_qa_test.go says so in as many words).
// Nothing compared the two. On 2026-09-06 ranger-base-888fv removed the
// GIT_EXTERNAL_DIFF row from inletPin() AND from the shipped drop-in — the two
// ends TestQAThePolicyDropInMatchesTheInletPin holds equal — and deliberately
// left the installed copy, closing "Root drop-in NOT reinstalled (no-op at that
// tier)". That rested on ranger-base-sn0w8, which measured that a policy-tier
// empty string does not OVERRIDE a name the process environment already
// carries. It is not a measurement that the row fails to REACH a name nothing
// else sets, and on 2026-09-11 that is exactly what it did: the one key present
// at only that end was in every seat's env, empty, and git execs a set-but-empty
// GIT_EXTERNAL_DIFF as the diff driver `""`. Bare `git diff` died `cannot run :`
// in every posse-launched seat on this box for five days.
//
// So the two ends this repo already holds equal were both right, and the box was
// wrong, and no arm anywhere could see the difference. scripts/verify-policy-pins.sh
// is the third end and these are its arms.
//
// THE ONES THAT CARRY THE BEAD:
//   - the EXTRA class. A file HEAD has stopped shipping is named by nothing on
//     the shipped side and is exactly as in force as a stale one, so a
//     comparison that walked only the shipped names would be blind to the whole
//     -file spelling of the same defect.
//   - the EMPTY arm, twice. No policy directory is NOT clean, and a reference
//     that ships no drop-in is NOT clean either — a check that scores either as
//     a pass is the silence this whole family exists for.
//   - the lle18 SHAPE itself, driven through the script with the real stale
//     bytes: shipped file plus the one GIT_EXTERNAL_DIFF row.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const ppScript = "scripts/verify-policy-pins.sh"

// The three drop-ins the fixture repo ships. Names, not contents: the check is
// byte identity against HEAD, so any bytes do, and the shipped-looking names are
// what the EXTRA glob is keyed on.
var ppShipped = map[string]string{
	"10-posse-inlet-pin.json": `{"env":{"GIT_PAGER":""}}` + "\n",
	"20-posse-field-pin.json": `{"apiKeyHelper":""}` + "\n",
	"30-posse-hooks.json":     `{"allowManagedHooksOnly":true}` + "\n",
}

// ppRun drives the real script against a scratch repo and a scratch policy dir.
// Both are flags rather than environment, which is the script's own rule: a seam
// that has to be typed cannot be inherited into a pass.
func ppRun(t *testing.T, repo, policyDir string) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(ppScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(abs, "--repo", repo, "--policy-dir", policyDir)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v\n%s", ppScript, err, out)
	}
	return string(out), code
}

// ppRepo builds a scratch git repo whose HEAD ships `files` under the drop-in
// directory. An empty map is a repo that ships none, which is its own arm.
func ppRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	dir := filepath.Join(repo, "etc", "claude", "managed-settings.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, body := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A repo with no tracked file at all has no HEAD, and every arm needs one.
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("qa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=qa", "GIT_AUTHOR_EMAIL=qa@example.invalid",
		"GIT_COMMITTER_NAME=qa", "GIT_COMMITTER_EMAIL=qa@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	for _, argv := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		// Path-limited, never bare: every crew PID denies the unqualified
		// form through a PATH shim that reads argv and not the tree, so it
		// refuses in a scratch fixture repo too (AGENTS.md, rangerhq-4pbt).
		{"commit", "-qm", "qa fixture", "--", "."},
	} {
		cmd := exec.Command("git", append([]string{"-C", repo}, argv...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", argv, err, out)
		}
	}
	return repo
}

// ppPolicy builds a scratch policy directory holding `files`.
func ppPolicy(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for n, body := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func ppCopy(extra map[string]string, drop ...string) map[string]string {
	out := map[string]string{}
	for k, v := range ppShipped {
		out[k] = v
	}
	for _, d := range drop {
		delete(out, d)
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestQAPolicyPinsInstalledAndMatchingIsClean(t *testing.T) {
	t.Parallel()
	out, code := ppRun(t, ppRepo(t, ppShipped), ppPolicy(t, ppShipped))
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	// A pass must say what it looked at, so "clean" cannot be earned by
	// comparing nothing.
	if !strings.Contains(out, "clean") || !strings.Contains(out, "3 shipped drop-in(s)") {
		t.Errorf("a clean run must report the count it compared:\n%s", out)
	}
}

// The whole-file spelling of the bead's defect: one row added to an installed
// file that the repo no longer ships.
func TestQAPolicyPinsFindsTheStaleInstalledFile(t *testing.T) {
	t.Parallel()
	stale := ppCopy(map[string]string{
		"10-posse-inlet-pin.json": `{"env":{"GIT_EXTERNAL_DIFF":"","GIT_PAGER":""}}` + "\n",
	})
	out, code := ppRun(t, ppRepo(t, ppShipped), ppPolicy(t, stale))
	if code != 1 {
		t.Fatalf("exit %d, want 1 — this is ranger-base-lle18 itself\n%s", code, out)
	}
	if !strings.Contains(out, "STALE") || !strings.Contains(out, "10-posse-inlet-pin.json") {
		t.Errorf("must name the stale file:\n%s", out)
	}
	// The other two must still read fresh: a check that goes red across the
	// board on one drift tells the operator nothing about where to look.
	for _, n := range []string{"20-posse-field-pin.json", "30-posse-hooks.json"} {
		if !strings.Contains(out, "fresh    "+n) {
			t.Errorf("%s should still read fresh:\n%s", n, out)
		}
	}
}

func TestQAPolicyPinsFindsTheUninstalledFile(t *testing.T) {
	t.Parallel()
	out, code := ppRun(t, ppRepo(t, ppShipped), ppPolicy(t, ppCopy(nil, "30-posse-hooks.json")))
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "MISSING") || !strings.Contains(out, "30-posse-hooks.json") {
		t.Errorf("must name the file the box is not carrying:\n%s", out)
	}
}

// The class a shipped-side walk cannot see. A retired drop-in is as in force as
// a stale one.
func TestQAPolicyPinsFindsTheRetiredFileStillInForce(t *testing.T) {
	t.Parallel()
	extra := ppCopy(map[string]string{"99-posse-retired.json": `{"env":{"GIT_EXTERNAL_DIFF":""}}` + "\n"})
	out, code := ppRun(t, ppRepo(t, ppShipped), ppPolicy(t, extra))
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a file HEAD stopped shipping is named by nothing on the shipped side\n%s", code, out)
	}
	if !strings.Contains(out, "EXTRA") || !strings.Contains(out, "99-posse-retired.json") {
		t.Errorf("must name the retired pin:\n%s", out)
	}
}

// It must not speak for drop-ins this repo does not ship, or the first employer
// policy file on the box teaches everyone to ignore it.
func TestQAPolicyPinsIgnoresADropInThisRepoDoesNotOwn(t *testing.T) {
	t.Parallel()
	other := ppCopy(map[string]string{"50-employer.json": `{"env":{"FOO":"bar"}}` + "\n"})
	out, code := ppRun(t, ppRepo(t, ppShipped), ppPolicy(t, other))
	if code != 0 {
		t.Fatalf("exit %d, want 0 — the glob is too wide\n%s", code, out)
	}
	if strings.Contains(out, "50-employer.json") {
		t.Errorf("a drop-in this repo does not ship is not this check's business:\n%s", out)
	}
}

// Nothing present means nothing measured. Without this, a box with the policy
// tier never installed at all reads as green.
func TestQAPolicyPinsRefusesToPassWithNoPolicyDirectory(t *testing.T) {
	t.Parallel()
	out, code := ppRun(t, ppRepo(t, ppShipped), filepath.Join(t.TempDir(), "absent"))
	if code != 2 {
		t.Fatalf("exit %d, want 2\n%s", code, out)
	}
	if strings.Contains(out, "clean") {
		t.Errorf("an unmeasured run must not say clean:\n%s", out)
	}
	if !strings.Contains(out, "nothing measured") {
		t.Errorf("must say it measured nothing:\n%s", out)
	}
}

// The other end of the same trap, and the one that would go unnoticed longest:
// a reference that ships no drop-in compares nothing and would otherwise score
// every box on earth clean, forever.
func TestQAPolicyPinsRefusesToPassWhenTheReferenceShipsNothing(t *testing.T) {
	t.Parallel()
	out, code := ppRun(t, ppRepo(t, nil), ppPolicy(t, ppShipped))
	if code != 2 {
		t.Fatalf("exit %d, want 2 — an empty reference is not a pass\n%s", code, out)
	}
	if strings.Contains(out, "clean") {
		t.Errorf("an empty reference must not say clean:\n%s", out)
	}
}

// The reference is HEAD, and a reader has to be told when what is in front of
// them is not it — otherwise the prescription names bytes they cannot see.
func TestQAPolicyPinsSaysWhenTheWorkingTreeIsNotTheReference(t *testing.T) {
	t.Parallel()
	repo := ppRepo(t, ppShipped)
	p := filepath.Join(repo, "etc", "claude", "managed-settings.d", "10-posse-inlet-pin.json")
	if err := os.WriteFile(p, []byte(`{"env":{"GIT_PAGER":"","X":"y"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := ppRun(t, repo, ppPolicy(t, ppShipped))
	if code != 0 {
		t.Fatalf("exit %d, want 0 — an uncommitted edit is not an installed-file finding\n%s", code, out)
	}
	if !strings.Contains(out, "differs from HEAD") {
		t.Errorf("must say the working tree is not the reference:\n%s", out)
	}
}

// ppWriteVerbs are the verbs that would make this an installer. It prints the
// line for the operator to type and runs none of them: writing the policy tier
// is a root write to a deployed system, and a flag away from a persona-writable
// tree is not far enough (crew guardrail 3).
var ppWriteVerbs = []string{"sudo", "rm", "mv", "cp", "install", "tee", "chmod", "chown", "ln", "dd", "truncate", "shred"}

// ppQuoted strips comments and both kinds of quoted string, so what is left is
// what the shell would EXECUTE. The repair lines the script prints name sudo and
// tee by design; the assertion is that no such word survives outside a string.
var ppQuoted = regexp.MustCompile(`"[^"]*"|'[^']*'`)

func TestQAPolicyPinsExecutesNoWriteAndIsWired(t *testing.T) {
	t.Parallel()
	info, err := os.Stat(ppScript)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("%s is not executable", ppScript)
	}
	body, err := os.ReadFile(ppScript)
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)

	// The scan needs a failing wrong arm of its own, or a regexp that ate the
	// whole file would pass this vacuously.
	if code := ppQuoted.ReplaceAllString("x=1\nsudo tee /etc/x\n", ""); !strings.Contains(code, "sudo") {
		t.Fatalf("RIG DEAD: the stripper removes an unquoted verb; every arm below is vacuous: %q", code)
	}
	for _, line := range strings.Split(src, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = ppQuoted.ReplaceAllString(s, "")
		for _, v := range ppWriteVerbs {
			if regexp.MustCompile(`\b` + v + `\b`).MatchString(s) {
				t.Errorf("%s must not execute %q — it prints the line for a person to type: %s", ppScript, v, line)
			}
		}
	}

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	// The roster census checks a rostered command against its target's recipe,
	// so these two spellings have to be the one string.
	if !strings.Contains(string(mk), "verify-policy-pins:\n\tscripts/verify-policy-pins.sh\n") {
		t.Error("Makefile lost the verify-policy-pins target")
	}
	if !strings.Contains(string(mk), "verify-policy-pins ") {
		t.Error("verify-policy-pins is not in .PHONY")
	}

	// A check nothing runs is the defect verify-box exists for, one level up.
	box, err := os.ReadFile("scripts/verify-box.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(box), "verify-policy-pins\tscripts/verify-policy-pins.sh") {
		t.Error("verify-policy-pins is not on the verify-box roster — a live-box check nothing runs is the condition that produced ranger-base-lle18")
	}
}
