package rhq

// QA attack on the L3 behavior probe (ranger-base-jw3w, verifying the close of
// ranger-base-3c3). The fix that landed is real: a hook that is deleted, empty,
// stripped of its execute bit, or overwritten with `exit 0` is caught by the
// probe and reaches the operator as a DEGRADED line, and a deleted one is
// reinstalled before it is asked. The bottom half of this file pins those.
//
// The top half pinned the two forms that got past it, filed as
// ranger-base-flz7. Both shared a shape: probeL3Hooks asked a question about a
// FILE — "does <git-common-dir>/hooks/<slot> exit 1 when I exec it" — where
// git's question is "what does core.hooksPath say, and does THAT program
// refuse". Where the two disagree the probe reported a wall git will never run.
//
// ESCAPE A IS CLOSED (ranger-base-flz7, with the install half rangerhq-b38m):
// hooksDir() asks `git rev-parse --git-path hooks`, so install, probe, and
// parity all address the directory git dispatches from. Its two pins were
// INVERTED — they now assert the contract, and the shape did its job: the
// hooksDir assertion failed the moment the fix landed, which is what said the
// pins had to be rewritten rather than deleted.
//
// ESCAPE B IS CLOSED (ADR 0023, ranger-base-ujdg, from the design bead
// ranger-base-vqvl). The whole class — asking the file at the dispatch path
// a behavioral question — is gone, not just the probe's signature: identity
// (byte-exact against posse's render, or the prescribed chain to it) decides
// whether a slot counts, and the only bytes ever exec'd are posse's own
// render from a private temp file. TestL3ProbeIsDefeatedByItsOwnSignature was
// the live-defect pin, green on purpose per NOTES.md's silent-revert lesson;
// it is now INVERTED, asserting the contract. The tests after it pin the rest
// of ADR 0023's verification list: the probe never execs foreign bytes, the
// prescribed chain certifies by identity, and a foreign hook — even one that
// genuinely refuses everything — degrades rather than certifies.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func qaHookRepo(t *testing.T) (repo, hooks string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	h, err := hooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	return repo, h
}

func qaArm(t *testing.T, hooks string, slots ...string) {
	t.Helper()
	for _, slot := range slots {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func qaGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// ─── the premise, measured rather than assumed ───────────────────────────────

// git dispatches from core.hooksPath when it is set, and the slot under
// <git-common-dir>/hooks stays inert. Everything below rests on this, so it is
// pinned on its own: if a future git changes it, this fails first and names why.
func TestGitRunsCoreHooksPathNotTheGitDirHooks(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	qaGit(t, repo, "config", "user.email", "qa@example.invalid")
	qaGit(t, repo, "config", "user.name", "qa")

	fired := filepath.Join(repo, "gitdir-hook-fired")
	os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"),
		[]byte("#!/bin/sh\n: > "+fired+"\nexit 0\n"), 0o755)
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644)
	qaGit(t, repo, "add", "a.txt")
	qaGit(t, repo, "commit", "-qm", "one")
	if _, err := os.Stat(fired); err != nil {
		t.Fatalf("baseline: the .git/hooks slot did not run at all: %v", err)
	}

	elsewhere := t.TempDir()
	other := filepath.Join(repo, "other-hook-fired")
	os.WriteFile(filepath.Join(elsewhere, "prepare-commit-msg"),
		[]byte("#!/bin/sh\n: > "+other+"\nexit 0\n"), 0o755)
	os.Remove(fired)
	qaGit(t, repo, "config", "core.hooksPath", elsewhere)
	os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b\n"), 0o644)
	qaGit(t, repo, "add", "b.txt")
	qaGit(t, repo, "commit", "-qm", "two")

	if _, err := os.Stat(other); err != nil {
		t.Errorf("the core.hooksPath slot did not run: %v", err)
	}
	if _, err := os.Stat(fired); err == nil {
		t.Error("the .git/hooks slot still ran under core.hooksPath")
	}
	// git's own answer follows the redirect; hooksDir's does not. This is the
	// discriminator ranger-base-flz7's fix wants.
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got == "" || strings.HasSuffix(got, ".git/hooks") {
		t.Errorf("`git rev-parse --git-path hooks` = %q, want the redirect", got)
	}
}

// ─── ranger-base-flz7: escape A, closed ──────────────────────────────────────
//
// These two were the live-defect pins for escape A, now INVERTED as their own
// failure messages instructed. hooksDir() asks `git rev-parse --git-path
// hooks` instead of deriving `<git-common-dir>/hooks`, so every L3 claim is
// about the directory git actually dispatches from.
//
// Escape B below is NOT closed by that change and keeps its live-defect pin.

// ESCAPE A. One `git config` — no hook file written, so installHook's
// foreign-hook refusal never fires — used to move git's dispatch out from
// under an armed slot while the probe kept calling that slot armed. The probe
// must now follow the redirect: an armed .git/hooks and an EMPTY core.hooksPath
// is a repo with no wall, and it has to read as one.
func TestL3ProbeFollowsCoreHooksPath(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	a := &App{}
	InstallPrePushHook(repo)
	a.InstallCommitGuardHook(repo)
	if got := a.probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
		t.Fatalf("fixture must start armed: %+v", got)
	}

	// An EMPTY directory: git will run no hook at all for either slot.
	empty := t.TempDir()
	qaGit(t, repo, "config", "core.hooksPath", empty)

	dir, err := hooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if dir != empty {
		t.Fatalf("hooksDir(%s) = %q — want the redirect %q, not the git dir %q",
			repo, dir, empty, hooks)
	}
	got := a.probeL3Hooks(repo, true)
	if got.PrePush || got.CommitGuard {
		t.Errorf("probe still claims a wall git will not run: %+v", got)
	}
	if got.HooksDir != empty {
		t.Errorf("probe named %s; git dispatches from %s", got.HooksDir, empty)
	}
}

// ESCAPE A end to end, both directions. The launch's install-then-probe
// sequence must PUT the gates where git will run them — and where it cannot
// (a foreign hook already sitting at the redirect), the operator must be told
// in a line that names the redirected directory, not the git dir.
func TestParityFollowsCoreHooksPath(t *testing.T) {
	newLaunch := func(t *testing.T, repo string) (*App, *Runtime, *AgentFile) {
		t.Helper()
		home := t.TempDir()
		a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
		claude, err := a.LoadRuntime("claude")
		if err != nil {
			t.Fatal(err)
		}
		ag := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n")
		return a, claude, ag
	}

	t.Run("install lands at the redirect and parity is honest about it", func(t *testing.T) {
		repo, gitHooks := qaHookRepo(t)
		redirect := t.TempDir()
		qaGit(t, repo, "config", "core.hooksPath", redirect)
		a, claude, ag := newLaunch(t, repo)

		// Exactly what a launch does — herdrback.go:1155-1164.
		if _, err := InstallPrePushHook(repo); err != nil {
			t.Fatalf("install pre-push: %v", err)
		}
		if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
			t.Fatalf("install commit guard: %v", err)
		}
		for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
			if _, err := os.Stat(filepath.Join(redirect, slot)); err != nil {
				t.Errorf("%s must be installed where git runs it: %v", slot, err)
			}
			if _, err := os.Stat(filepath.Join(gitHooks, slot)); err == nil {
				t.Errorf("%s was written to the inert git dir as well", slot)
			}
		}
		p := a.CheckParityIn(ag, claude, CageShims, TierStrong, repo)
		if len(p.Degraded) != 0 {
			t.Errorf("the wall is genuinely armed at the redirect; degraded: %v", p.Degraded)
		}
		for _, gate := range ag.Deny {
			if !strings.Contains(p.Realized[gate], "render probed, dispatch verified") {
				t.Errorf("%s -> %q, want an identity-verified L3 claim", gate, p.Realized[gate])
			}
		}
	})

	t.Run("a foreign hook at the redirect degrades and names it", func(t *testing.T) {
		repo, gitHooks := qaHookRepo(t)
		redirect := t.TempDir()
		qaGit(t, repo, "config", "core.hooksPath", redirect)
		// Armed at the git dir, so a probe that looked there would go green;
		// waved through at the redirect, which is the only slot git runs.
		qaArm(t, gitHooks, "pre-push", "prepare-commit-msg")
		for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
			if err := os.WriteFile(filepath.Join(redirect, slot), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		a, claude, ag := newLaunch(t, repo)
		InstallPrePushHook(repo)       // both refuse: the slots are foreign
		a.InstallCommitGuardHook(repo) //nolint:errcheck // best effort, as at launch

		p := a.CheckParityIn(ag, claude, CageShims, TierStrong, repo)
		joined := strings.Join(p.Degraded, "\n")
		if joined == "" {
			t.Fatal("a pass-through hook at core.hooksPath must degrade the launch")
		}
		if !strings.Contains(joined, AbbrevHome(redirect)) {
			t.Errorf("degradation must name the directory git dispatches from (%s):\n%s",
				AbbrevHome(redirect), joined)
		}
		if strings.Contains(joined, AbbrevHome(gitHooks)) {
			t.Errorf("degradation named the inert git dir %s:\n%s", AbbrevHome(gitHooks), joined)
		}
	})
}

// ESCAPE B, CLOSED (ADR 0023, ranger-base-ujdg). The probe's signature used
// to be a fixed string in a file the planter can read, so a hook could
// refuse the probe and pass everything else — not the TOCTOU the probe's
// type doc names; nothing changes between probe and use, the hook lies. No
// secret signature fixes a black box the adversary's own code is answering.
// The fix instead stops asking the file a question: identity (byte-exact
// dispatch, ADR 0023 Decision 1) plus behavior of OUR OWN render, never the
// planted bytes (Decision 2). This test used to be the live-defect pin,
// GREEN on purpose per NOTES.md's silent-revert lesson; it is now INVERTED,
// asserting the contract instead of the defect.
//
// InstallPrePushHook/InstallCommitGuardHook refusing to overwrite a hook
// that carries no marker of ours (ranger-base-3c3) is untouched by this fix
// and still holds — the probe's answer changed, not install's refusal rule.
func TestL3ProbeIsDefeatedByItsOwnSignature(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	body := "#!/bin/sh\n[ \"$RHQ_PERSONA\" = probe ] && exit 1\nexit 0\n"
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		os.WriteFile(filepath.Join(hooks, slot), []byte(body), 0o755)
	}
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	InstallPrePushHook(repo)
	a.InstallCommitGuardHook(repo)

	got := a.probeL3Hooks(repo, true)
	if got.PrePush || got.CommitGuard {
		t.Fatalf("the signature-discriminating hook must be reported DOWN on both arms — no identity, so no L3 claim: %+v", got)
	}
	if got.PrePushDegraded == "" || !strings.Contains(got.PrePushDegraded, "foreign hook") {
		t.Errorf("pre-push must degrade naming the foreign file: %q", got.PrePushDegraded)
	}
	if got.CommitGuardDegraded == "" || !strings.Contains(got.CommitGuardDegraded, "foreign hook") {
		t.Errorf("prepare-commit-msg must degrade naming the foreign file: %q", got.CommitGuardDegraded)
	}
	if PrePushHookInstalled(repo) || CommitGuardHookInstalled(repo) {
		t.Error("the planted body carries no marker; the launch must not have overwritten it")
	}
}

// ADR 0023 Decision 2, pinned directly: the probe must never exec the file
// at the dispatch path, so a planted hook's own code never runs during a
// probe, whatever it would have done. A canary the hook body would drop
// proves it — after the probe, the canary must not exist.
func TestL3ProbeNeverExecsForeignBytes(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	canary := filepath.Join(repo, "canary")
	body := "#!/bin/sh\n: > " + canary + "\nexit 1\n"
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		os.WriteFile(filepath.Join(hooks, slot), []byte(body), 0o755)
	}
	a := &App{}
	a.probeL3Hooks(repo, true)
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("the probe ran the planted hook's own bytes — the canary exists")
	}
}

// ADR 0023 Decision 1, chain pin: the prescribed chain dispatcher, built
// byte-exact with posse-<slot> copied from the render, certifies by
// identity — no marker sits at the slot itself. Flip one byte of the member
// and it degrades, naming the member (the dispatch-path file, not the
// dispatcher).
func TestL3ProbeCertifiesThePrescribedChain(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	a := &App{}
	os.WriteFile(filepath.Join(hooks, "pre-push"), []byte(chainHookDispatcherWith("pre-push", "theirs-pre-push")), 0o755)
	os.WriteFile(filepath.Join(hooks, "theirs-pre-push"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
	os.WriteFile(filepath.Join(hooks, "posse-pre-push"), []byte(PrePushHook), 0o755)

	if got := a.probeL3Hooks(repo, true); !got.PrePush {
		t.Fatalf("byte-exact chain must certify: %+v (degraded: %q)", got, got.PrePushDegraded)
	}

	// One byte of the member differs from the render.
	tampered := strings.Replace(PrePushHook, "exit 0", "exit  0", 1)
	if tampered == PrePushHook {
		t.Fatal("fixture did not actually change a byte")
	}
	os.WriteFile(filepath.Join(hooks, "posse-pre-push"), []byte(tampered), 0o755)
	got := a.probeL3Hooks(repo, true)
	if got.PrePush {
		t.Fatalf("a one-byte-flipped member must not certify: %+v", got)
	}
	if !strings.Contains(got.PrePushDegraded, "posse-pre-push") {
		t.Errorf("degradation must name the member, not the dispatcher: %q", got.PrePushDegraded)
	}
}

// ADR 0023 Decision 3, foreign-refuser pin, stated as its own test rather
// than folded into a larger fixture: an unconditional `exit 1` at the
// dispatch path, no marker, must DEGRADE — even though it refuses every
// operation the probe asks about — and the degraded line must name the
// foreign file and the chain remedy. This is the honest trade ADR 0023's
// Consequences section calls out: a legitimately refusing foreign hook that
// used to certify now degrades, because a black-box probe cannot tell it
// apart from ranger-base-vqvl's liar.
func TestL3ProbeDegradesAForeignRefuser(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	qaArm(t, hooks, "pre-push", "prepare-commit-msg")
	a := &App{}
	got := a.probeL3Hooks(repo, true)
	if got.PrePush || got.CommitGuard {
		t.Fatalf("an unconditional foreign refuser must not certify: %+v", got)
	}
	for _, degraded := range []string{got.PrePushDegraded, got.CommitGuardDegraded} {
		if !strings.Contains(degraded, "foreign hook") {
			t.Errorf("must name the foreign hook: %q", degraded)
		}
		if !strings.Contains(degraded, "posse gates install-hooks") {
			t.Errorf("must print the chain remedy: %q", degraded)
		}
	}
}

// ─── what the fix does hold: the tamper forms that were caught ───────────────

// Every way of neutralizing the slot AT THE PATH THE PROBE KNOWS is reported,
// and a deleted hook is put back before it is asked. The execute bit matters
// on its own: git silently skips a non-executable hook, so 0644 is a wall that
// is present on disk and absent in practice.
func TestL3ProbeCatchesTamperingAtTheKnownPath(t *testing.T) {
	for _, c := range []struct {
		name   string
		break_ func(hooks string)
		heals  bool // does the launch's reconcile put it back?
	}{
		{"deleted", func(h string) {
			os.Remove(filepath.Join(h, "pre-push"))
			os.Remove(filepath.Join(h, "prepare-commit-msg"))
		}, true},
		{"execute bit stripped", func(h string) {
			os.Chmod(filepath.Join(h, "pre-push"), 0o644)
			os.Chmod(filepath.Join(h, "prepare-commit-msg"), 0o644)
		}, false},
		{"truncated to empty", func(h string) {
			os.WriteFile(filepath.Join(h, "pre-push"), nil, 0o755)
			os.WriteFile(filepath.Join(h, "prepare-commit-msg"), nil, 0o755)
		}, false},
		{"marker-bearing pass-through", func(h string) {
			os.WriteFile(filepath.Join(h, "pre-push"), []byte("#!/bin/sh\n"+prePushMarker+"\nexit 0\n"), 0o755)
			os.WriteFile(filepath.Join(h, "prepare-commit-msg"), []byte("#!/bin/sh\n"+sharedIndexMarker+"\nexit 0\n"), 0o755)
		}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, hooks := qaHookRepo(t)
			qaArm(t, hooks, "pre-push", "prepare-commit-msg")
			c.break_(hooks)

			home := t.TempDir()
			a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
			if got := a.probeL3Hooks(repo, true); got.PrePush || got.CommitGuard {
				t.Errorf("tampering went unreported: %+v", got)
			}

			InstallPrePushHook(repo)
			a.InstallCommitGuardHook(repo)
			after := a.probeL3Hooks(repo, true)
			if healed := after.PrePush && after.CommitGuard; healed != c.heals {
				t.Errorf("reconcile healed = %v, want %v: %+v", healed, c.heals, after)
			}
		})
	}
}

// The degradation an operator actually reads names the slot, the directory it
// probed, and what the lost layer was protecting — the prepare-commit-msg line
// has to say "beads visibility" out loud, because that is the half of the slot
// that fails toward disclosure rather than toward a loud refusal.
func TestFailedL3ProbeNamesWhatWasLost(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		os.WriteFile(filepath.Join(hooks, slot), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	}
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	ag := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n")
	joined := strings.Join(a.CheckParityIn(ag, claude, CageShims, TierStrong, repo).Degraded, "\n")
	for _, want := range []string{
		"L3 pre-push hook", "foreign hook, posse cannot vouch for a hook it did not write",
		"L3 prepare-commit-msg hook", "beads visibility, constitution-path and shared-index guards are not realized",
		AbbrevHome(hooks),
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("degradation missing %q in:\n%s", want, joined)
		}
	}
}

// ─── rangerhq-b38m: §9's own Verify, run against a redirected dispatch dir ───
//
// The install half of rangerhq-b38m was fixed in code (hooksDir asks git), but
// INSTALL.md §9 still told the operator to `mv` and to probe a literal
// `.git/hooks/<slot>`. Under a set `core.hooksPath` that is the same defect one
// layer up, and it fails the same way — silently green: the gates install where
// git dispatches, the operator probes a directory git never consults, and
// whatever refusing file is sitting there (a stale bd shim, a chain built
// before the redirect) passes all four probes over a repo with no wall.
//
// So pin the recipe by RUNNING it, not by reading it: extract §9's Verify block
// out of INSTALL.md verbatim and run it over two fixtures whose verdicts must
// differ. Both arms carry a witness, because "did not read 1/1/1/0" is
// otherwise satisfied by a block that measures nothing.
//
//	ARMED   gates at the dispatch dir, .git/hooks empty — §9 must read 1/1/1/0.
//	HOLLOW  gates in .git/hooks, dispatch dir empty — §9 must NOT, while the
//	        pre-fix spelling still does. That is the bead's false green,
//	        reproduced: four green probes over a wall git will never run.
func TestQADocSection9VerifyProbesFollowGitsDispatchDir(t *testing.T) {
	block := qaSection9VerifyBlock(t)
	if !strings.Contains(block, `"$h"/pre-push`) || !strings.Contains(block, `"$h"/prepare-commit-msg`) {
		t.Fatalf("INSTALL.md §9's Verify block no longer runs the slot at git's dispatch dir:\n%s", block)
	}
	if !strings.Contains(block, "git config --get core.hooksPath") {
		t.Error("INSTALL.md §9's Verify block no longer shows the operator core.hooksPath (rangerhq-b38m)")
	}
	preFix := strings.ReplaceAll(block, `"$h"/`, ".git/hooks/")
	if preFix == block {
		t.Fatal("the pre-fix spelling is identical to the block — the control would measure nothing")
	}
	green := []int{1, 1, 1, 0}

	t.Run("armed at the dispatch dir", func(t *testing.T) {
		repo, gitdirHooks := qaHookRepo(t)
		elsewhere := filepath.Join(repo, "myhooks")
		if err := os.Mkdir(elsewhere, 0o755); err != nil {
			t.Fatal(err)
		}
		qaGit(t, repo, "config", "core.hooksPath", elsewhere)
		qaInstallBothGates(t, repo)
		qaHasSlots(t, elsewhere, true)
		qaHasSlots(t, gitdirHooks, false)

		got, out := qaRunSection9Probes(t, repo, block)
		if !qaCodesAre(got, green) {
			t.Errorf("§9's Verify read %v over an installed wall, want %v:\n%s", got, green, out)
		}
	})

	t.Run("hollow: armed only where git does not look", func(t *testing.T) {
		repo, gitdirHooks := qaHookRepo(t)
		qaInstallBothGates(t, repo) // no redirect yet: these land in .git/hooks
		empty := filepath.Join(repo, "myhooks")
		if err := os.Mkdir(empty, 0o755); err != nil {
			t.Fatal(err)
		}
		qaGit(t, repo, "config", "core.hooksPath", empty)
		qaHasSlots(t, gitdirHooks, true)
		qaHasSlots(t, empty, false)

		// The witness: this repo has no wall, and the pre-fix spelling says
		// it does. Without this the arm below could pass for any reason.
		bad, badOut := qaRunSection9Probes(t, repo, preFix)
		if !qaCodesAre(bad, green) {
			t.Fatalf("the fixture is not the false green it is meant to be — the .git/hooks spelling read %v, want %v:\n%s", bad, green, badOut)
		}
		got, out := qaRunSection9Probes(t, repo, block)
		if qaCodesAre(got, green) {
			t.Errorf("§9's Verify certified a wall git will never run (rangerhq-b38m):\n%s", out)
		}
	})
}

// qaInstallBothGates installs the two L3 gates into repo, wherever git says
// this repo's hooks are dispatched from.
func qaInstallBothGates(t *testing.T, repo string) {
	t.Helper()
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := (&App{}).InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
}

// qaHasSlots asserts both L3 slots are, or are not, present in dir. The
// fixtures below are only discriminating while exactly one directory is armed.
func qaHasSlots(t *testing.T, dir string, want bool) {
	t.Helper()
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		_, err := os.Stat(filepath.Join(dir, slot))
		if want && err != nil {
			t.Fatalf("fixture: %s is missing from %s: %v", slot, dir, err)
		}
		if !want && err == nil {
			t.Fatalf("fixture: %s is also in %s — the two arms would not differ", slot, dir)
		}
	}
}

func qaCodesAre(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// qaSection9VerifyBlock returns the shell block under §9's "Verify — by
// running the hooks" heading, as the operator would paste it: the `$ ` prompt
// stripped, continuation lines untouched.
func qaSection9VerifyBlock(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	i := strings.Index(doc, "**Verify — by running the hooks")
	if i < 0 {
		t.Fatal("INSTALL.md §9: the hook Verify block is gone — the pin has stopped reading its subject")
	}
	rest := doc[i:]
	open := strings.Index(rest, "```sh\n")
	if open < 0 {
		t.Fatal("INSTALL.md §9: the hook Verify heading is no longer followed by a shell block")
	}
	rest = rest[open+len("```sh\n"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		t.Fatal("INSTALL.md §9: the hook Verify block has no terminator")
	}
	var lines []string
	for _, ln := range strings.Split(rest[:end], "\n") {
		lines = append(lines, strings.TrimPrefix(ln, "$ "))
	}
	return strings.Join(lines, "\n")
}

// qaRunSection9Probes runs the pasted block in repo and returns the exit codes
// its `echo $?` lines printed, in order, plus the whole transcript.
func qaRunSection9Probes(t *testing.T, repo, script string) ([]int, string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = repo
	cmd.Env = qaEnvWithout("RHQ_PERSONA", "RHQ_TOOLS_DENY", "GIT_INDEX_FILE")
	b, _ := cmd.CombinedOutput()
	out := string(b)
	var codes []int
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		n, err := strconv.Atoi(ln)
		if err == nil {
			codes = append(codes, n)
		}
	}
	return codes, out
}
