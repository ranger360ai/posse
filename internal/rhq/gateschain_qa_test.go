package rhq

// QA, rangerhq-y1je — the chain INSTALL.md §9 prescribes, exercised rather
// than read. §9 has to hand both slots to bd's shims AND keep the posse gates,
// so the gate runs as its own process with its status checked and then execs
// bd's hook. Everything here runs the hook files the way git does; nothing is
// asserted from their text. Self-contained (own repo, own fixtures) so it
// stands whatever the next persona does to gates_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaChainRepo builds the state §9 leaves behind: posse-<slot> holds the gate,
// bd-<slot> stands in for `bd hooks install`'s shim (it records the argv and
// stdin it was handed, which is the only way to see whether the chain reached
// it and with what), and <slot> is §9's dispatcher, copied verbatim.
func qaChainRepo(t *testing.T) (repo, witness string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	witness = filepath.Join(t.TempDir(), "bd.log")
	for slot, stdin := range map[string]string{"pre-push": " </dev/null", "prepare-commit-msg": ""} {
		if err := os.Rename(filepath.Join(hooks, slot), filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Fatal(err)
		}
		bd := "#!/bin/sh\nprintf 'argv[%s]\\n' \"$*\" >> " + witness +
			"\nprintf 'stdin[%s]\\n' \"$(cat)\" >> " + witness + "\nexit 0\n"
		if err := os.WriteFile(filepath.Join(hooks, "bd-"+slot), []byte(bd), 0o755); err != nil {
			t.Fatal(err)
		}
		chain := "#!/bin/sh\nd=$(dirname \"$0\")\n" +
			"\"$d/posse-" + slot + "\" \"$@\"" + stdin + " || exit $?\n" +
			"[ -x \"$d/bd-" + slot + "\" ] || exit 0\n" +
			"exec \"$d/bd-" + slot + "\" \"$@\"\n"
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(chain), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo, witness
}

// runHook runs one slot the way git does — argv, stdin, and an env that
// carries nothing this suite's own pane exported.
func runHook(t *testing.T, repo, slot, stdin string, env ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(repo, ".git", "hooks", slot))
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append([]string{"PATH=" + PathOutsideGates("")}, env...)
	switch slot {
	case "pre-push":
		cmd.Args = append(cmd.Args, "origin", "https://example.invalid/x.git")
	default:
		cmd.Args = append(cmd.Args, filepath.Join(repo, ".git", "COMMIT_EDITMSG"), "message")
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v", slot, err)
	}
	return string(out), code
}

// The whole point of the chain: the gate's refusal ends the slot (bd's hook
// is never reached), and when the gate passes, bd's hook gets its argv and —
// on pre-push, where git feeds a ref list — the stdin the gate did not eat.
func TestQADocChainRefusesFirstAndOtherwiseReachesBdsHook(t *testing.T) {
	repo, witness := qaChainRepo(t)
	refs := "refs/heads/main a1 refs/heads/main b1\nrefs/tags/v1 a2 refs/tags/v1 b2\n"
	read := func() string { b, _ := os.ReadFile(witness); return string(b) }

	out, code := runHook(t, repo, "pre-push", refs, "RHQ_PERSONA=qa", "RHQ_TOOLS_DENY=Bash(git push:*)", "RHQ_GATES_DIR="+t.TempDir())
	if code != 1 || !strings.Contains(out, "refused by posse gate: git push") {
		t.Errorf("denied push must refuse with exit 1: code=%d %q", code, out)
	}
	if read() != "" {
		t.Errorf("a refused push must not reach bd's hook: %q", read())
	}

	out, code = runHook(t, repo, "prepare-commit-msg", "", "RHQ_PERSONA=qa", "RHQ_GATES_DIR="+t.TempDir())
	if code != 1 || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("unqualified commit must refuse with exit 1: code=%d %q", code, out)
	}
	if read() != "" {
		t.Errorf("a refused commit must not reach bd's hook: %q", read())
	}

	if out, code := runHook(t, repo, "pre-push", refs); code != 0 {
		t.Errorf("no RHQ_TOOLS_DENY must pass: code=%d %q", code, out)
	}
	if got := read(); !strings.Contains(got, "argv[origin https://example.invalid/x.git]") ||
		!strings.Contains(got, "refs/heads/main a1") || !strings.Contains(got, "refs/tags/v1 a2") {
		t.Errorf("bd's pre-push hook must get its argv and the WHOLE ref list on stdin: %q", got)
	}

	if err := os.Truncate(witness, 0); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "prepare-commit-msg", ""); code != 0 {
		t.Errorf("no RHQ_PERSONA (the operator's own commit) must pass: code=%d %q", code, out)
	}
	if got := read(); !strings.Contains(got, "COMMIT_EDITMSG message]") {
		t.Errorf("bd's prepare-commit-msg hook must get $1 and $2 unchanged: %q", got)
	}
}

// rangerhq-xo65: the chain execs bd-<slot> unconditionally, so a repo where
// bd never took that slot (older bd, `bd hooks install --beads/--shared`) has
// no commit path at all — the failure hits the operator, whom the gate is
// careful to exempt.
func TestQADocChainSurvivesAMissingNeighbourHook(t *testing.T) {
	repo, _ := qaChainRepo(t)
	if err := os.Remove(filepath.Join(repo, ".git", "hooks", "bd-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "prepare-commit-msg", ""); code != 0 {
		t.Errorf("the operator's commit must survive a missing neighbour: code=%d %q", code, out)
	}
	// A neighbour that is there but not executable execs to 126 just the
	// same, so the guard is -x, not -e.
	if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", "bd-prepare-commit-msg"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "prepare-commit-msg", ""); code != 0 {
		t.Errorf("the operator's commit must survive a non-executable neighbour: code=%d %q", code, out)
	}
	// And the degrade is only about the neighbour: the gate behind the
	// dispatcher still refuses what it refused before.
	if out, code := runHook(t, repo, "prepare-commit-msg", "", "RHQ_PERSONA=probe"); code != 1 ||
		!strings.Contains(out, "refused by posse gate") {
		t.Errorf("the gate must still refuse with a missing neighbour: code=%d %q", code, out)
	}
	// The same for pre-push, whose exec carries git's ref list.
	if err := os.Remove(filepath.Join(repo, ".git", "hooks", "bd-pre-push")); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "pre-push", "refs/heads/main a1 refs/heads/main b1\n"); code != 0 {
		t.Errorf("a push must survive a missing neighbour: code=%d %q", code, out)
	}
}

// rangerhq-xo65, the same defect from git's side: with the chain naming a
// neighbour that is not there, a plain operator commit — no RHQ_PERSONA, the
// case the guard exists to exempt — must still land.
func TestQAOperatorCanCommitThroughAChainMissingItsNeighbour(t *testing.T) {
	repo, _ := qaChainRepo(t)
	if err := os.Remove(filepath.Join(repo, ".git", "hooks", "bd-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "X.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = []string{"PATH=" + PathOutsideGates(""), "HOME=" + t.TempDir(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := git("add", "X.md"); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if out, err := git("commit", "-qm", "operator commit", "--", "X.md"); err != nil {
		t.Errorf("the operator's own commit must not be blocked by a missing neighbour: %v %s", err, out)
	}
}

// rangerhq-lrnp: the guard's own comment exempts the commits git drives
// itself, on the strength of a marker file in the git dir. A clean
// `git revert` writes none of them before prepare-commit-msg runs (git
// 2.39.3), so the persona is refused — and refused only after the revert is
// already staged in the shared index the guard exists to protect.
func TestQAGuardLetsAGitDrivenRevertThrough(t *testing.T) {
	t.Skip("rangerhq-lrnp: no REVERT_HEAD exists at prepare-commit-msg time, so the guard refuses the revert")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(extra []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), env...), extra...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	git(nil, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644)
	git(nil, "add", "a.txt")
	git(nil, "commit", "-qm", "add a")
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	if out, err := git(persona, "revert", "--no-edit", "HEAD"); err != nil {
		t.Errorf("a git-driven revert must be let through: %v %s", err, out)
	}
	if out, _ := git(nil, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Errorf("a refused revert must not leave the shared index dirty: %q", out)
	}
}

// rangerhq-b38m: git runs hooks from core.hooksPath when it is set, and
// hooksDir() never read it — so both gates landed in .git/hooks, install
// reported success, §9's probes (which run the file directly) went green, and
// git ran none of it. Closed with ranger-base-flz7, which was the same defect
// seen from the probe's side: hooksDir() now asks `git rev-parse --git-path
// hooks`, so install and probe address the one directory git dispatches from.
func TestQAInstallHooksHonoursCoreHooksPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	elsewhere := filepath.Join(repo, "myhooks")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	exec.Command("git", "-C", repo, "init", "-q", "-b", "main").Run()
	exec.Command("git", "-C", repo, "config", "core.hooksPath", elsewhere).Run()
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "pre-push")); err != nil {
		t.Errorf("the gate must go where git will run it (core.hooksPath): %v", err)
	}
}

// rangerhq-j4sq — §9's closing claim, run rather than read: a repo where
// `bd hooks install` got there first is covered by NOTHING when you dispatch
// into it. This is session create's own install path (herdrback.go: both
// installs are best effort, their errors discarded), not the CLI's, so the
// silence is the point — the operator sees no failure anywhere.
//
// bd's shim stands in as a hook carrying its `# bd-shim v1` header and an
// exit-0 body: installHook keys only on the ABSENCE of our own marker, and a
// stand-in keeps the real `exec bd hooks run` off a throwaway repo.
func TestQASessionCreateInstallsNothingIntoABdHookedRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := "#!/usr/bin/env sh\n# bd-shim v1\nexit 0\n"
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Verbatim what a session create does, error handling included.
	InstallPrePushHook(repo)
	installCommitGuard(repo)

	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		b, err := os.ReadFile(filepath.Join(hooks, slot))
		if err != nil {
			t.Fatal(err)
		}
		if got := string(b); got != shim {
			t.Fatalf("%s: session create's install changed the slot; §9 may now be understating what a dispatch does:\n%s", slot, got)
		}
	}
	// And the gate is not hiding under another name either.
	ents, err := os.ReadDir(hooks)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		b, _ := os.ReadFile(filepath.Join(hooks, e.Name()))
		if strings.Contains(string(b), "posse-gate") {
			t.Fatalf("found a posse gate at %s — §9's closing paragraph needs re-checking", e.Name())
		}
	}
	// The consequence, at the slot: §9's own probe 1 goes green-through.
	out, code := runHook(t, repo, "pre-push", "refs/heads/main a refs/heads/main b\n",
		"RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)")
	if code != 0 || strings.Contains(out, "refused by posse gate") {
		t.Fatalf("pre-push refused after all (exit %d): %s — §9's closing paragraph needs re-checking", code, out)
	}
}

// rangerhq-mgdk: the CLI's own --chain, exercised against a bd-shimmed
// repo the way TestQASessionCreateInstallsNothingIntoABdHookedRepo shows
// dispatch leaves untouched. Both InstallXxxChained calls must build the
// same chain INSTALL.md §9 walks by hand — bd's shim moved to bd-<slot>,
// ours at posse-<slot>, the real slot holding the dispatcher — and the
// gate must still run first and bd's shim must still be reachable behind
// it. hookInstalled() (via PrePushHookInstalled/CommitGuardHookInstalled)
// must see through the result, since neither slot carries our marker
// directly once chained.
func TestQAChainedInstallTakesOverBdsShimAndStaysDetected(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(t.TempDir(), "bd.log")
	shim := "#!/bin/sh\n# bd-shim v1\nprintf 'ran[%s]\\n' \"$0\" >> " + witness + "\nexit 0\n"
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if PrePushHookInstalled(repo) || CommitGuardHookInstalled(repo) {
		t.Fatal("a repo holding only bd's shim must not report our gate installed")
	}

	if _, err := InstallPrePushHookChained(repo); err != nil {
		t.Fatalf("chained pre-push install must take over bd's shim: %v", err)
	}
	if _, _, _, err := (&App{}).InstallCommitGuardHookChained(repo); err != nil {
		t.Fatalf("chained commit-guard install must take over bd's shim: %v", err)
	}

	if !PrePushHookInstalled(repo) || !CommitGuardHookInstalled(repo) {
		t.Error("hookInstalled must see through the chain it just built")
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooks, "bd-"+slot)); err != nil {
			t.Errorf("bd's shim must survive, moved aside: %v", err)
		}
		if _, err := os.Stat(filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Errorf("our gate must be installed behind the dispatcher: %v", err)
		}
	}

	if out, code := runHook(t, repo, "pre-push", "refs/heads/main a refs/heads/main b\n",
		"RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)"); code != 1 || !strings.Contains(out, "refused by posse gate") {
		t.Errorf("denied push must still refuse through the chain: code=%d %q", code, out)
	}
	if b, _ := os.ReadFile(witness); len(b) != 0 {
		t.Errorf("a refused push must not reach bd's shim: %q", b)
	}
	if out, code := runHook(t, repo, "pre-push", "refs/heads/main a refs/heads/main b\n"); code != 0 {
		t.Errorf("an allowed push must fall through to bd's shim: code=%d %q", code, out)
	}
	if b, _ := os.ReadFile(witness); !strings.Contains(string(b), "bd-pre-push") {
		t.Errorf("bd's shim must have run: %q", b)
	}
}

// rangerhq-mgdk: --chain only recognizes bd's own shim (the `# bd-shim v1`
// header). A hook of unknown shape must still be refused, chain or no
// chain — chaining an arbitrary foreign hook without knowing its exit-code
// semantics is exactly the risk the manual prescription exists to make an
// operator, not rhq, decide.
func TestQAChainedInstallStillRefusesAGenuinelyUnknownHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPrePushHookChained(repo); err == nil || !strings.Contains(err.Error(), "not a posse hook") {
		t.Fatalf("an unrecognized foreign hook must still be refused under --chain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hooks, "bd-pre-push")); err == nil {
		t.Error("a non-bd hook must not be moved aside as if it were bd's shim")
	}
}

// rangerhq-xo65: §9's hand-built chain and the one posse writes itself
// (chainBdShim / the printed prescription) are the same arrangement, and a
// fix applied to one and not the other leaves half the fleet dead at the
// next missing neighbour. The doc is the operator's copy of chainRender —
// pin it byte for byte, with bd's names filled in, so drift is a red test
// rather than a repo that cannot commit.
func TestQADocChainMatchesTheRenderedDispatcher(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		want := chainHookDispatcherWith(slot, "bd-"+slot)
		// The block as INSTALL.md pastes it: a heredoc into the slot.
		open := "$ cat > .git/hooks/" + slot + " <<'EOF'\n"
		i := strings.Index(doc, open)
		if i < 0 {
			t.Errorf("INSTALL.md §9 no longer writes %s with a heredoc", slot)
			continue
		}
		rest := doc[i+len(open):]
		j := strings.Index(rest, "EOF\n")
		if j < 0 {
			t.Errorf("INSTALL.md §9's %s heredoc has no terminator", slot)
			continue
		}
		if got := rest[:j]; got != want {
			t.Errorf("INSTALL.md §9's %s chain has drifted from chainHookDispatcherWith:\n got %q\nwant %q", slot, got, want)
		}
	}
}

// rangerhq-xo65: every repo chained before the fix carries the unguarded
// dispatcher, and nothing about it is ever rewritten — so the fix would reach
// new installs only and leave the existing fleet one missing neighbour away
// from a repo that cannot commit. installHook already refreshes the
// posse-<slot> behind a recognized chain on every launch; it upgrades the
// dispatcher itself in the same pass. Safe because the body matched our own
// render byte for byte: we know what it is, and we write back the same shape
// and the same neighbour.
func TestQALegacyChainIsUpgradedInPlace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		t.Run(slot, func(t *testing.T) {
			repo := t.TempDir()
			if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			hooks := filepath.Join(repo, ".git", "hooks")
			install := InstallPrePushHook
			if slot == "prepare-commit-msg" {
				install = installCommitGuard
			}
			if _, err := install(repo); err != nil {
				t.Fatal(err)
			}
			// The pre-fix arrangement, exactly as an older posse left it.
			if err := os.Rename(filepath.Join(hooks, slot), filepath.Join(hooks, "posse-"+slot)); err != nil {
				t.Fatal(err)
			}
			legacy := legacyChainHookDispatcherWith(slot, "bd-"+slot)
			if err := os.WriteFile(filepath.Join(hooks, slot), []byte(legacy), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(hooks, "bd-"+slot), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}

			// A launch-time reinstall, the pass that already refreshes the
			// chained member.
			got, err := install(repo)
			if err != nil {
				t.Fatalf("reinstall over a pre-xo65 chain must succeed: %v", err)
			}
			if want := filepath.Join(hooks, "posse-"+slot); got != want {
				t.Errorf("reinstall must report the chained member: got %q want %q", got, want)
			}
			b, err := os.ReadFile(filepath.Join(hooks, slot))
			if err != nil {
				t.Fatal(err)
			}
			if want := chainHookDispatcherWith(slot, "bd-"+slot); string(b) != want {
				t.Errorf("the dispatcher must be upgraded in place:\n got %q\nwant %q", string(b), want)
			}
			// The neighbour it names is untouched, and a foreign hook stays foreign.
			if nb, err := os.ReadFile(filepath.Join(hooks, "bd-"+slot)); err != nil || string(nb) != "#!/bin/sh\nexit 0\n" {
				t.Errorf("the neighbour must not be rewritten: %q %v", string(nb), err)
			}
		})
	}
}

// The upgrade is only for a body that is byte-for-byte one of our own
// renders. A foreign hook that merely looks chain-shaped is still refused,
// untouched — ADR 0002 §3.
func TestQALegacyUpgradeLeavesForeignHooksAlone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	slot := filepath.Join(hooks, "prepare-commit-msg")
	if err := os.Rename(slot, filepath.Join(hooks, "posse-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	// Ours dispatched, but its exit status discarded — not a chain.
	foreign := "#!/bin/sh\nd=$(dirname \"$0\")\n\"$d/posse-prepare-commit-msg\" \"$@\"\nexec \"$d/bd-prepare-commit-msg\" \"$@\"\n"
	if err := os.WriteFile(slot, []byte(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := installCommitGuard(repo); err == nil {
		t.Error("a foreign hook must still be refused, not upgraded")
	}
	if b, _ := os.ReadFile(slot); string(b) != foreign {
		t.Errorf("a refused hook must be left byte-identical: %q", string(b))
	}
}
