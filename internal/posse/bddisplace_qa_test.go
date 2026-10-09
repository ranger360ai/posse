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

// ─── the SECOND bd install (bead ranger-base-00a5l) ─────────────────────────
//
// The state INSTALL.md §9 names in as many words — "any later `bd init` (a
// re-init, `--from-jsonl`) or `bd import` displaces the gates again" — which
// is the recurring one, because the operator has by then followed posse's
// prescription once and bd's install is still something `bd init` runs on its
// own.
//
// It used to be the one state in which the ONE command all three new
// surfaces prescribe REFUSED: posse's first chain parks bd's shim at
// `bd-<slot>`, a second bd install takes the slot back, and `chainBdShim`
// refused outright over that leftover — "bd-<slot> already exists — not
// overwriting" — so both walls stayed down for as long as the operator
// followed the instructions (ranger-base-5ayqc, finding 1; the pin that
// recorded the hole lived here and these are its replacement).
//
// Two claims now, and they are separate: the row NAMES the live gate at
// `posse-<slot>` — the one file neither of bd's two renames can be read off
// — and the prescription WORKS, over both of bd's suffixes, leaving bd's
// shim reachable exactly as round 1 did.
func TestQADisplacementRemedyWorksOnTheSecondBdInstall(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{".backup", ".old"} {
		t.Run("round 2 suffix "+suffix, func(t *testing.T) {
			t.Parallel()
			repo, hooks := qaHookRepo(t)
			a := qaProbeApp(t)
			qaInstallBothGates(t, repo)

			// Round 1: bd displaces, the operator runs the prescription.
			qaBdDisplace(t, hooks, "pre-push", ".backup")
			qaBdDisplace(t, hooks, "prepare-commit-msg", ".backup")
			if _, err := InstallPrePushHookChained(repo); err != nil {
				t.Fatalf("premise: round-1 --chain over pre-push: %v", err)
			}
			if _, _, _, err := a.InstallCommitGuardHookChained(repo); err != nil {
				t.Fatalf("premise: round-1 --chain over prepare-commit-msg: %v", err)
			}
			if got := a.probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
				t.Fatalf("premise: round-1 --chain must hold both walls: %+v", got)
			}

			// Round 2: bd's install runs again, over the chained slots.
			qaBdDisplace(t, hooks, "pre-push", suffix)
			qaBdDisplace(t, hooks, "prepare-commit-msg", suffix)

			down := a.probeL3Hooks(repo, true)
			if down.PrePush || down.CommitGuard {
				t.Fatalf("premise: a second bd install takes both slots: %+v", down)
			}
			if down.PrePushVerdict != l3Displaced || down.CommitGuardVerdict != l3Displaced {
				t.Errorf("verdicts = %v/%v, want l3Displaced: the gate is live a filename away and one command puts it back",
					down.PrePushVerdict, down.CommitGuardVerdict)
			}
			// The line names the LIVE gate, not the round-1 leftover and
			// not posse's own dispatcher: `posse-<slot>` does not begin
			// with the slot, so no prefix scan over the hooks dir reaches
			// it. And it does not claim bd moved a file posse wrote.
			joined := down.PrePushDegraded + "\n" + down.CommitGuardDegraded
			for _, want := range []string{
				AbbrevHome(filepath.Join(hooks, "posse-pre-push")),
				AbbrevHome(filepath.Join(hooks, "posse-prepare-commit-msg")),
				"TAKEN BACK",
				"posse gates install-hooks --chain",
				"shared-index guards are not realized",
			} {
				if !strings.Contains(joined, want) {
					t.Errorf("the second-install line must say %q:\n%s", want, joined)
				}
			}
			if strings.Contains(joined, "MOVED ASIDE") {
				t.Errorf("posse put that file there, not bd — the line must not report a rename nobody did:\n%s", joined)
			}
			for _, line := range []string{down.PrePushDegraded, down.CommitGuardDegraded} {
				if strings.Contains(line, "\n") {
					t.Errorf("a Degraded entry is ONE line: %q", line)
				}
			}

			// THE OTHER HALF OF THE SAME SENTENCE: the install refusal.
			// The degraded row above is one of the two surfaces that word
			// round 2 apart from round 1; this is the other, and it was
			// reached and unpinned (ranger-base-c4uyj finding 1, escaped
			// from ranger-base-00a5l). An install WITHOUT --chain in this
			// state finds posse's own dispatcher member at `posse-<slot>`,
			// not a file bd renamed, so the deletion advice is wrong here:
			// `posse-<slot>` is the file the dispatcher execs FIRST, and
			// removing it exits every push 127 and every commit the same
			// (INSTALL.md:1399). Round 1 — where displacedPosseHook returns
			// bd's `.backup` and the deletion advice is correct — is pinned
			// by TestQAInstallRefusalOverBdsShimNamesTheFlag above, and
			// nothing read this branch until here.
			_, refusal := InstallPrePushHook(repo)
			if refusal == nil {
				t.Fatal("install-hooks without --chain must still refuse bd's shim on round 2")
			}
			for _, want := range []string{
				AbbrevHome(filepath.Join(hooks, "posse-pre-push")),
				"where posse's own chain put it",
				"leave it where it is",
			} {
				if !strings.Contains(refusal.Error(), want) {
					t.Errorf("the round-2 install refusal must say %q:\n%s", want, refusal)
				}
			}
			if strings.Contains(refusal.Error(), "delete it") {
				t.Errorf("that file is posse's own chain member and the dispatcher execs it first — telling the operator to delete it exits every push 127:\n%s", refusal)
			}

			// THE REMEDY, the second time: the prescribed command runs.
			if _, err := InstallPrePushHookChained(repo); err != nil {
				t.Fatalf("round-2 --chain over pre-push refused: %v", err)
			}
			if _, _, _, err := a.InstallCommitGuardHookChained(repo); err != nil {
				t.Fatalf("round-2 --chain over prepare-commit-msg refused: %v", err)
			}
			if got := a.probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
				t.Errorf("--chain must restore both walls on the second bd install too: %+v", got)
			}
			// And bd's hook is still reachable, which is the other half of
			// the chain's promise on every round: posse took the slot, not
			// bd's hook. The file at bd-<slot> is bd's own shim — the copy
			// round 2 planted, replacing the copy round 1 parked.
			for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
				b, err := os.ReadFile(filepath.Join(hooks, "bd-"+slot))
				if err != nil || !isBdShim(string(b)) {
					t.Errorf("bd's shim must survive round 2 at bd-%s: %v", slot, err)
				}
			}
		})
	}
}

// The refusal `bd-<slot>` still earns: a file there that is NOT bd's shim is
// the operator's own, or another tool's chain, and posse's rename would
// destroy it without a word. Overwriting one copy of bd's shim with another
// is the one case that loses nothing, and it is the only one taken.
func TestQAChainStillRefusesAForeignBdSlotNeighbour(t *testing.T) {
	t.Parallel()
	repo, hooks := qaHookRepo(t)
	qaInstallBothGates(t, repo)
	qaBdDisplace(t, hooks, "pre-push", ".backup")
	mine := filepath.Join(hooks, "bd-pre-push")
	if err := WriteExecutable(mine, []byte("#!/bin/sh\n# the operator's own\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := InstallPrePushHookChained(repo)
	if err == nil {
		t.Fatal("--chain must refuse to overwrite a bd-pre-push that is not bd's shim")
	}
	for _, want := range []string{AbbrevHome(mine), "not bd's shim", bdShimMarker, "INSTALL.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must say %q:\n%s", want, err)
		}
	}
	if b, readErr := os.ReadFile(mine); readErr != nil || !strings.Contains(string(b), "the operator's own") {
		t.Errorf("the refusal must leave that file byte for byte: %v", readErr)
	}
}
