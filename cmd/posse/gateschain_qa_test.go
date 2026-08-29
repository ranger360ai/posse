package main

// QA, rangerhq-irrl — the chain prescription `posse gates install-hooks` prints
// when it finds a foreign hook, exercised by RUNNING IT rather than by reading
// it. The prescription exists because appending to the gate silently discarded
// its refusal (rangerhq-kk6e/rangerhq-0g1c); a prescription that cannot be
// followed as printed leaves the same hole open, so the assertion is on the
// state the printed block actually produces. Self-contained (own repo, own
// extractor) so it stands whatever the next persona does to main_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
)

// qaPrescription pulls the runnable block out of a refusal: everything from
// the `cd <hooks>` line through the `chmod +x <slot>` that ends it, with the
// bare `posse` of step 3 pointed at the binary under test. Anchored on the
// END marker and then the LAST `cd ` before it, because since rangerhq-mgdk
// one call reports both slots and so may print two prescriptions: anchoring
// on the first `cd ` would hand back the other slot's block with this one's
// tail glued on.
func qaPrescription(t *testing.T, out, slot, bin string) string {
	t.Helper()
	tail := "\nchmod +x " + slot + "\n"
	end := strings.Index(out, tail)
	if end < 0 {
		t.Fatalf("no runnable prescription for %s in:\n%s", slot, out)
	}
	start := strings.LastIndex(out[:end], "\ncd ")
	if start < 0 {
		t.Fatalf("no runnable prescription for %s in:\n%s", slot, out)
	}
	block := out[start+1 : end+len(tail)]
	return strings.Replace(block, "\nposse gates install-hooks ", "\n"+bin+" gates install-hooks ", 1)
}

// qaForeignBoth is the state `bd hooks install` leaves and INSTALL.md §9
// walks the operator out of: a foreign shim in each slot posse wants. It
// returns the HOME the whole exercise runs under as well as the repo.
//
// THE REPO LIVES UNDER THAT HOME, and that is load-bearing (ranger-base-rstk).
// Every path in a printed prescription is AbbrevHome'd (gates.go), so a repo
// under $HOME is prescribed as `cd ~/…` — the form the operator is actually
// handed for their own checkout. On darwin t.TempDir() is under /var/folders
// and $HOME is not, so nothing was ever abbreviated here and the block was a
// plain absolute path that ran from anywhere. On ubuntu-latest, where HOME=/tmp
// holds the temp dirs, the same block came out as `~/…` and this suite pasted
// it into a shell holding a DIFFERENT HOME: the `cd` resolved to a doubled path
// that does not exist, sh carried on regardless, and the hook files landed in
// whatever directory the test binary was standing in. Placing the repo under
// the home makes the abbreviated form the one every platform exercises.
func qaForeignBoth(t *testing.T) (home, repo string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home = t.TempDir()
	repo = filepath.Join(home, "checkout")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		shim := "#!/bin/sh\n# bd-shim v1\necho \"bd shim ran: " + slot + "\" >&2\nexit 0\n"
		if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", slot), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home, repo
}

// qaInstallHooks runs the command under test with the HOME its prescriptions
// will be pasted under. The printing process and the pasting shell must agree
// on what `~` means or the block is not runnable as printed, which is the
// whole claim these pins make.
func qaInstallHooks(t *testing.T, bin, home string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"gates", "install-hooks"}, args...)...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("gates install-hooks: %v %s", err, out)
	}
	return string(out), code
}

// gitOutsideGates resolves the REAL git binary, ignoring any posse L0 shim on
// the session PATH. `exec.Command("git", …)` resolves a bare name against the
// CURRENT process's PATH — never cmd.Env — so a persona running this suite ran
// its own `git` shim, and the shim's refusal of an unqualified `git commit`
// (deny: Bash(git commit unless --)) was reported as the chain killing the
// operator's commit. Red for every persona, green for the operator, and about
// neither the hook nor the chain.
func gitOutsideGates(t *testing.T) string {
	t.Helper()
	for _, dir := range filepath.SplitList(rhq.PathOutsideGates("")) {
		p := filepath.Join(dir, "git")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	t.Skip("no git outside the gates")
	return ""
}

// qaSh runs a prescription block the way an operator pastes it: one /bin/sh,
// no shell state carried in from this suite's own pane, and the same HOME the
// block was printed under so its `~` names the directory it meant.
//
// It runs in a scratch directory of our own, and that directory must be EMPTY
// afterwards. The block's first line is a `cd` into the hooks dir and every
// line after it is relative to that, so a `cd` that fails does not stop
// anything: sh carries on and writes the hook files wherever it is standing.
// Until ranger-base-rstk that was this package's own source directory — which
// a writable checkout absorbs in silence, and `make test-linux`, whose whole
// guarantee is that /repo is mounted read-only, reports as
// "cannot create pre-push: Read-only file system". The emptiness check is the
// arm that fails on every platform rather than only on the read-only one.
func qaSh(t *testing.T, home, script string) (string, int) {
	t.Helper()
	scratch := t.TempDir()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Dir = scratch
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("sh: %v", err)
	}
	left, rerr := os.ReadDir(scratch)
	if rerr != nil {
		t.Fatalf("read the scratch dir the prescription ran in: %v", rerr)
	}
	if len(left) > 0 {
		names := make([]string, 0, len(left))
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Errorf("the prescription wrote %v into the directory it was pasted from — its `cd` did not land:\n%s", names, out)
	}
	return string(out), code
}

// rangerhq-pon3: install-hooks is fatal on the pre-push slot and only
// reports on the commit slot, so once the pre-push dispatcher is pasted —
// step 5 of the first prescription — every later install-hooks run dies on
// it and the commit gate is never installed. The second prescription then
// renames a file that was never written and pastes a dispatcher pointing at
// nothing: exit 127, and every commit in the repo fails with it.
func TestQAInstallRefusalPrescriptionIsRunnable(t *testing.T) {
	bin := buildRhq(t)
	home, repo := qaForeignBoth(t)

	// The refusal for the first slot, and the block it prints.
	first, _ := qaInstallHooks(t, bin, home, repo)
	pre := qaPrescription(t, first, "pre-push", bin)
	preOut, code := qaSh(t, home, pre)
	if code != 0 {
		t.Errorf("the pre-push prescription must run as printed: code=%d\n%s", code, preOut)
	}
	// Step 3 of that block is itself an install-hooks run; it prints the
	// second slot's prescription, which the operator follows next.
	commit := qaPrescription(t, preOut, "prepare-commit-msg", bin)
	commitOut, code := qaSh(t, home, commit)
	if code != 0 {
		t.Errorf("the prepare-commit-msg prescription must run as printed: code=%d\n%s", code, commitOut)
	}

	qaAssertBothSlotsChained(t, repo)
}

// rangerhq-pon3, the other order. The bead's claim was about ORDER —
// "prescription A must be finished last, and nothing printed says so" — and
// since rangerhq-mgdk one call reports both slots, so the operator is handed
// both blocks at once and may paste either first. Pinning only A-then-B
// would pin one half of the claim.
func TestQAInstallRefusalPrescriptionsRunInEitherOrder(t *testing.T) {
	bin := buildRhq(t)
	home, repo := qaForeignBoth(t)

	// One call, both refusals, both prescriptions — the commit slot's first.
	first, _ := qaInstallHooks(t, bin, home, repo)
	commitOut, code := qaSh(t, home, qaPrescription(t, first, "prepare-commit-msg", bin))
	if code != 0 {
		t.Errorf("the prepare-commit-msg prescription must run as printed FIRST: code=%d\n%s", code, commitOut)
	}
	// Its own step 3 reprints the pre-push block, now against a repo whose
	// commit slot already holds our dispatcher: taking the second slot must
	// not disturb the first.
	preOut, code := qaSh(t, home, qaPrescription(t, commitOut, "pre-push", bin))
	if code != 0 {
		t.Errorf("the pre-push prescription must run as printed SECOND: code=%d\n%s", code, preOut)
	}

	qaAssertBothSlotsChained(t, repo)
}

// qaAssertBothSlotsChained is the end state both prescription orders owe:
// each gate installed behind its dispatcher, each slot refusing its probe
// with exit 1, and the form the gate passes still landing.
func qaAssertBothSlotsChained(t *testing.T, repo string) {
	t.Helper()
	hooks := filepath.Join(repo, ".git", "hooks")

	// Both gates must exist behind their dispatchers.
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Errorf("the %s gate must be installed after its prescription: %v", slot, err)
		}
	}

	// And the probe each prescription prints must hold: refusal, exit 1.
	msg := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	os.WriteFile(msg, []byte("m\n"), 0o644)
	for _, p := range []struct {
		slot, stdin string
		args        []string
		env         []string
	}{
		{"pre-push", "refs/heads/main a refs/heads/main b\n", []string{"origin", "x"},
			[]string{"RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)"}},
		{"prepare-commit-msg", "", []string{msg}, []string{"RHQ_PERSONA=probe"}},
	} {
		cmd := exec.Command(filepath.Join(hooks, p.slot), p.args...)
		cmd.Dir = repo
		cmd.Stdin = strings.NewReader(p.stdin)
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, p.env...)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		if code != 1 || !strings.Contains(string(out), "refused by posse gate") {
			t.Errorf("%s probe must refuse and exit 1: code=%d %q", p.slot, code, out)
		}
	}

	// The commit the gate PASSES must still work — a dispatcher whose
	// neighbour is missing exits 127 and takes every commit with it,
	// including this one. Path-limited because since rangerhq-lt2w the wall
	// covers a shell with no RHQ_PERSONA too, so the unqualified form is no
	// longer a way to ask this question.
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644)
	add := exec.Command(gitOutsideGates(t), "-C", repo, "add", "a.txt")
	add.Env = []string{"PATH=" + rhq.PathOutsideGates("")}
	add.Run()
	ci := exec.Command(gitOutsideGates(t), "-C", repo, "commit", "-qm", "operator commit", "--", "a.txt")
	ci.Env = []string{"PATH=" + rhq.PathOutsideGates(""), "HOME=" + t.TempDir(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	if out, err := ci.CombinedOutput(); err != nil {
		t.Errorf("a path-limited commit must survive the chain: %v %s", err, out)
	}
}

// rangerhq-mgdk: a single `install-hooks` call on a repo where BOTH slots
// are foreign must attempt and report both — pre-push failing must not
// cost prepare-commit-msg (or vice versa) — and must exit non-zero exactly
// when something was left uninstalled.
func TestQAInstallHooksAttemptsBothSlotsInOneCall(t *testing.T) {
	bin := buildRhq(t)
	home, repo := qaForeignBoth(t)

	out, code := qaInstallHooks(t, bin, home, repo)
	if code == 0 {
		t.Errorf("a call that installed neither slot must exit non-zero: %s", out)
	}
	if !strings.Contains(out, "not installed: pre-push") {
		t.Errorf("pre-push refusal must be reported: %s", out)
	}
	if !strings.Contains(out, "not installed: prepare-commit-msg") {
		t.Errorf("prepare-commit-msg must be ATTEMPTED and reported even though pre-push failed first: %s", out)
	}
}

// rangerhq-mgdk: --chain takes over a repo `bd hooks install` reached
// first — the state TestQASessionCreateInstallsNothingIntoABdHookedRepo
// shows dispatch leaves silently uncovered — in one call, both slots.
func TestQAInstallHooksChainFlagTakesOverBdsShim(t *testing.T) {
	bin := buildRhq(t)
	home, repo := qaForeignBoth(t)
	hooks := filepath.Join(repo, ".git", "hooks")

	out, code := qaInstallHooks(t, bin, home, repo, "--chain")
	if code != 0 {
		t.Fatalf("gates install-hooks --chain: code=%d %s", code, out)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooks, "bd-"+slot)); err != nil {
			t.Errorf("bd's shim must be moved aside for %s: %v", slot, err)
		}
		if _, err := os.Stat(filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Errorf("our gate must be installed for %s: %v", slot, err)
		}
	}

	msg := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	os.WriteFile(msg, []byte("m\n"), 0o644)
	cmd := exec.Command(filepath.Join(hooks, "pre-push"), "origin", "x")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader("refs/heads/main a refs/heads/main b\n")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)"}
	pushOut, pushErr := cmd.CombinedOutput()
	pushCode := 0
	if ee, ok := pushErr.(*exec.ExitError); ok {
		pushCode = ee.ExitCode()
	}
	if pushCode != 1 || !strings.Contains(string(pushOut), "refused by posse gate") {
		t.Errorf("denied push must still refuse through the --chain result: code=%d %q", pushCode, pushOut)
	}
}
