package rhq

// QA attack on the L3 behavior probe (ranger-base-jw3w, verifying the close of
// ranger-base-3c3). The fix that landed is real: a hook that is deleted, empty,
// stripped of its execute bit, or overwritten with `exit 0` is caught by the
// probe and reaches the operator as a DEGRADED line, and a deleted one is
// reinstalled before it is asked. The bottom half of this file pins those.
//
// The top half pins the two forms that got past it, filed as ranger-base-flz7.
// One root cause: probeL3Hooks asks a question about a FILE — "does
// <git-common-dir>/hooks/<slot> exit 1 when I exec it" — and git's question is
// "what does core.hooksPath say, and does THAT program refuse". Where the two
// disagree the probe reports a wall git will never run.
//
// THE FIRST THREE TESTS ASSERT A LIVE DEFECT, NOT A CONTRACT. They are written
// green on purpose, because NOTES.md's silent-revert lesson is that a skipped
// pin is how a defect stays green — a red pin would be deleted, a skipped one
// forgotten. When ranger-base-flz7 lands they must FAIL, and the fix inverts
// them: that failure is the signal, and it is the whole point of the shape.

import (
	"os"
	"os/exec"
	"path/filepath"
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

// ─── ranger-base-flz7: what got past the probe ───────────────────────────────

// ESCAPE A. One `git config` — no hook file written, so installHook's
// foreign-hook refusal never fires — moves git's dispatch out from under an
// armed slot, and the probe keeps calling that slot armed.
//
// LIVE-DEFECT PIN: inverted by the fix for ranger-base-flz7.
func TestL3ProbeIgnoresCoreHooksPathToday(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	qaArm(t, hooks, "pre-push", "prepare-commit-msg")
	if got := probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
		t.Fatalf("fixture must start armed: %+v", got)
	}

	// An EMPTY directory: git will run no hook at all for either slot.
	empty := t.TempDir()
	qaGit(t, repo, "config", "core.hooksPath", empty)

	if dir, err := hooksDir(repo); err != nil || dir != hooks {
		t.Fatalf("hooksDir(%s) = %q, %v — want the unchanged %q", repo, dir, err, hooks)
	}
	got := probeL3Hooks(repo, true)
	if !got.PrePush || !got.CommitGuard {
		t.Fatalf("ranger-base-flz7 looks FIXED (probe = %+v). Invert this test: the "+
			"probe must now fail both arms under a redirected core.hooksPath, and "+
			"TestParityClaimsL3UnderARedirectedHooksPath must assert a DEGRADED line.", got)
	}
	if got.HooksDir != hooks {
		t.Errorf("probe named %s; git dispatches from %s", got.HooksDir, empty)
	}
}

// ESCAPE A end to end: the launch's install-then-probe sequence writes into a
// directory git no longer reads, reports no degradation, and Realizes both L3
// layers by name.
//
// LIVE-DEFECT PIN: inverted by the fix for ranger-base-flz7.
func TestParityClaimsL3UnderARedirectedHooksPath(t *testing.T) {
	repo, _ := qaHookRepo(t)
	empty := t.TempDir()
	qaGit(t, repo, "config", "core.hooksPath", empty)

	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	ag := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n")

	// Exactly what a launch does — herdrback.go:1155-1164.
	InstallPrePushHook(repo)
	a.InstallCommitGuardHook(repo)
	p := a.CheckParityIn(ag, claude, CageShims, TierStrong, repo)

	if len(p.Degraded) != 0 {
		t.Fatalf("ranger-base-flz7 looks FIXED (degraded: %v). Invert this test.", p.Degraded)
	}
	for _, gate := range ag.Deny {
		if !strings.Contains(p.Realized[gate], "behavior probed") {
			t.Fatalf("ranger-base-flz7 looks FIXED (%s -> %q). Invert this test.", gate, p.Realized[gate])
		}
	}
}

// ESCAPE B. The probe's signature is a fixed string in a file the planter can
// read, so a hook can refuse the probe and pass everything else. This is not
// the TOCTOU the probe's type doc names — nothing changes between the probe
// and the use; the hook lies. The launch's reconcile cannot help: the body
// carries no ownership marker, so installHook leaves it alone.
//
// LIVE-DEFECT PIN: inverted by whatever ranger-base-flz7 decides here.
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

	if got := probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
		t.Fatalf("ranger-base-flz7 escape B looks FIXED (probe = %+v). Invert this test.", got)
	}
	if PrePushHookInstalled(repo) || CommitGuardHookInstalled(repo) {
		t.Error("the planted body carries no marker; the launch must not have overwritten it")
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

			if got := probeL3Hooks(repo, true); got.PrePush || got.CommitGuard {
				t.Errorf("tampering went unreported: %+v", got)
			}

			home := t.TempDir()
			a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
			InstallPrePushHook(repo)
			a.InstallCommitGuardHook(repo)
			after := probeL3Hooks(repo, true)
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
		"L3 pre-push hook", "did not refuse git push with exit 1",
		"L3 prepare-commit-msg hook", "shared-index and beads visibility guards are not realized",
		AbbrevHome(hooks),
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("degradation missing %q in:\n%s", want, joined)
		}
	}
}
