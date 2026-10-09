//go:build posse_arm3

package posse

// QA for ranger-base-2msgj — the displacement `bd hooks install` performs
// silently, and what posse says about it.
//
// THE STATE, from the operator's shakedown of a cold box (github.com/
// ranger360ai/posse issue #2, 2026-10-08): bd 0.50.3's hook install — which
// `bd init` and `bd import` run on their own, not only `bd hooks install`
// typed by hand — renames posse's gate out of the slot and plants its own
// two-line shim there, printing nothing about either half. The gates are not
// damaged, they are not on the commit path, and before this bead the only
// thing posse said about it was "foreign hook, posse cannot vouch for a hook
// it did not write" — true of an employer's husky install, true of a
// colleague's script, and useless here, because here the wall WAS installed,
// IS still on disk a filename away, and one flag puts it back.
//
// The claim these pins hold: that state is reported as the displacement it
// is, naming the file posse's gate is in and the `--chain` that ends it —
// and the cases that merely RESEMBLE it are not reported as it, because a
// line that promises `--chain` over a hook `--chain` refuses is worse than
// the generic one it replaced.
//
// Nothing here runs bd. Every crew PID denies `Bash(bd hook install:*)` and
// `Bash(bd hooks install:*)` (ADR 0015 §3), so the fixture is built from
// what the binary does rather than by doing it: posse's own gates installed
// for real, renamed the way bd renames, with bd's own shim body in the slot.
// bd 0.50.3 carries both spellings of the rename — `.backup` in its default
// mode and `.old` in its own chain mode — and both are pinned, because the
// detection deliberately keys on CONTENT and not on either suffix.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaBdDisplace is the state bd leaves in one slot: the gate moved aside to
// slot+suffix, bd's shim in the slot. It returns the path the gate went to.
func qaBdDisplace(t *testing.T, hooks, slot, suffix string) string {
	t.Helper()
	moved := filepath.Join(hooks, slot+suffix)
	if err := os.Rename(filepath.Join(hooks, slot), moved); err != nil {
		t.Fatal(err)
	}
	shim := "#!/bin/sh\n" + bdShimMarker + "\nexec bd hooks run " + slot + " \"$@\"\n"
	if err := WriteExecutable(filepath.Join(hooks, slot), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	return moved
}

func qaProbeApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	return &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
}

// The finding an operator reads: both slots, the verdict, the file the gate
// is actually in, and the one command that chains it back. The guards the
// commit slot lost are still named — the gate's bytes being readable a
// filename away does not realize one of them.
func TestQADisplacedGateIsNamedWithItsRemedy(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{".backup", ".old"} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()
			repo, hooks := qaHookRepo(t)
			qaInstallBothGates(t, repo)
			pushMoved := qaBdDisplace(t, hooks, "pre-push", suffix)
			commitMoved := qaBdDisplace(t, hooks, "prepare-commit-msg", suffix)

			got := qaProbeApp(t).probeL3Hooks(repo, true)
			if got.PrePush || got.CommitGuard {
				t.Fatalf("a displaced gate must not certify either slot: %+v", got)
			}
			if got.PrePushVerdict != l3Displaced || got.CommitGuardVerdict != l3Displaced {
				t.Errorf("verdicts = %v/%v, want l3Displaced for both", got.PrePushVerdict, got.CommitGuardVerdict)
			}
			joined := got.PrePushDegraded + "\n" + got.CommitGuardDegraded
			for _, want := range []string{
				"MOVED ASIDE",
				"bd's own shim",
				"bd init",
				"posse gates install-hooks --chain",
				AbbrevHome(pushMoved),
				AbbrevHome(commitMoved),
				// The consequence clause is the slot's, not the
				// displacement's: nothing in the moved file runs.
				"shared-index guards are not realized",
			} {
				if !strings.Contains(joined, want) {
					t.Errorf("the displacement line must say %q:\n%s", want, joined)
				}
			}
			// Degraded flows into session meta, a flat-file format with no
			// quoting for embedded newlines (yamlflat.go): a multi-line
			// value truncates silently on read-back.
			for _, line := range []string{got.PrePushDegraded, got.CommitGuardDegraded} {
				if strings.Contains(line, "\n") {
					t.Errorf("a Degraded entry is ONE line: %q", line)
				}
			}
		})
	}
}

// The two neighbours of that state, neither of which is it. A line that
// named a displaced gate where there is none would send the operator looking
// for a file bd deleted (`--force`: "Overwrite existing hooks without
// backup"), and one that prescribed `--chain` over a husky hook would
// prescribe a command that refuses: `--chain` takes over bd's known shim
// shape and nothing else (rangerhq-mgdk).
func TestQAOnlyARealDisplacementIsReportedAsOne(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		break_ func(t *testing.T, hooks string)
	}{
		{"bd's shim, no gate beside it", func(t *testing.T, hooks string) {
			for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
				if err := os.Remove(filepath.Join(hooks, slot)); err != nil {
					t.Fatal(err)
				}
				shim := "#!/bin/sh\n" + bdShimMarker + "\nexit 0\n"
				if err := WriteExecutable(filepath.Join(hooks, slot), []byte(shim), 0o755); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{"a foreign hook that is not bd's, gate moved aside", func(t *testing.T, hooks string) {
			for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
				if err := os.Rename(filepath.Join(hooks, slot), filepath.Join(hooks, slot+".backup")); err != nil {
					t.Fatal(err)
				}
				if err := WriteExecutable(filepath.Join(hooks, slot), []byte("#!/bin/sh\n# husky\nexit 0\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			repo, hooks := qaHookRepo(t)
			qaInstallBothGates(t, repo)
			c.break_(t, hooks)

			got := qaProbeApp(t).probeL3Hooks(repo, true)
			if got.PrePush || got.CommitGuard {
				t.Fatalf("a foreign slot must not certify: %+v", got)
			}
			if got.PrePushVerdict != l3Foreign || got.CommitGuardVerdict != l3Foreign {
				t.Errorf("verdicts = %v/%v, want l3Foreign for both", got.PrePushVerdict, got.CommitGuardVerdict)
			}
			joined := got.PrePushDegraded + "\n" + got.CommitGuardDegraded
			for _, unwanted := range []string{"MOVED ASIDE", "--chain"} {
				if strings.Contains(joined, unwanted) {
					t.Errorf("this is not a displacement posse can chain; line must not say %q:\n%s", unwanted, joined)
				}
			}
		})
	}
}

// git's own samples sit beside every slot and start with its name. They are
// not posse's gate, and reading one as a displacement would turn bd's shim
// in a fresh clone into a report about a file nobody moved.
func TestQAGitsOwnSampleIsNotADisplacedGate(t *testing.T) {
	t.Parallel()
	repo, hooks := qaHookRepo(t)
	qaInstallBothGates(t, repo)
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if err := os.Remove(filepath.Join(hooks, slot)); err != nil {
			t.Fatal(err)
		}
		if err := WriteExecutable(filepath.Join(hooks, slot+".sample"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		shim := "#!/bin/sh\n" + bdShimMarker + "\nexit 0\n"
		if err := WriteExecutable(filepath.Join(hooks, slot), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := qaProbeApp(t).probeL3Hooks(repo, true); got.PrePushVerdict != l3Foreign || got.CommitGuardVerdict != l3Foreign {
		t.Errorf("a `.sample` is not a displaced gate: verdicts = %v/%v, want l3Foreign for both",
			got.PrePushVerdict, got.CommitGuardVerdict)
	}
}

// The remedy the line prescribes has to BE one: `--chain` over the displaced
// state puts the wall back, in one command, with no paste — and leaves the
// file bd moved alone, because that file is the operator's now and posse
// deletes nothing it did not write.
func TestQAChainRepairsADisplacement(t *testing.T) {
	t.Parallel()
	repo, hooks := qaHookRepo(t)
	a := qaProbeApp(t)
	qaInstallBothGates(t, repo)
	moved := qaBdDisplace(t, hooks, "pre-push", ".backup")
	qaBdDisplace(t, hooks, "prepare-commit-msg", ".backup")

	if _, err := InstallPrePushHookChained(repo); err != nil {
		t.Fatalf("--chain over a displaced pre-push: %v", err)
	}
	if _, _, _, err := a.InstallCommitGuardHookChained(repo); err != nil {
		t.Fatalf("--chain over a displaced prepare-commit-msg: %v", err)
	}
	if got := a.probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
		t.Errorf("--chain must restore both walls: %+v", got)
	}
	if _, err := os.Stat(moved); err != nil {
		t.Errorf("the file bd moved is the operator's, not posse's to remove: %v", err)
	}
	// And bd's shim is still reachable, which is the other half of the
	// chain's promise: posse took the slot, not bd's hook.
	if b, err := os.ReadFile(filepath.Join(hooks, "bd-pre-push")); err != nil || !isBdShim(string(b)) {
		t.Errorf("bd's shim must survive the chain at bd-pre-push: %v", err)
	}
}

// Without `--chain`, install-hooks refuses — and the refusal is where an
// operator who typed the wrong one of the two commands is standing, so it
// names bd, names the flag, and names where the gate went. The pasteable
// prescription is still printed behind it (its own pins:
// cmd/posse/gateschain_qa_test.go).
func TestQAInstallRefusalOverBdsShimNamesTheFlag(t *testing.T) {
	t.Parallel()
	repo, hooks := qaHookRepo(t)
	qaInstallBothGates(t, repo)
	moved := qaBdDisplace(t, hooks, "pre-push", ".backup")

	_, err := InstallPrePushHook(repo)
	if err == nil {
		t.Fatal("install-hooks without --chain must refuse bd's shim")
	}
	for _, want := range []string{"bd's own shim", bdShimMarker, "--chain", AbbrevHome(moved), "\ncd ", "chmod +x pre-push"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must say %q:\n%s", want, err)
		}
	}
}
