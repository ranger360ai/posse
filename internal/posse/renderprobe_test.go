//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-apf87 — `posse gates render-probe`, ADR 0092 D4
// amendment 3's behaviour-only L3 verb.
//
// The claim is three sentences:
//
//	the answer    exit 0 when every render of THIS BINARY refuses what it
//	              gates, 1 when one does not, 2 when the probe could not be
//	              taken at all. The third is the one the exits exist for:
//	              `posse upgrade` rolls back on 1 and reports a 2 both
//	              binaries give.
//	the subject   the answer is a property of the BINARY and not of any
//	              repo's installed hooks. Fresh, stale, foreign or absent at
//	              the dispatch path, the verb says the same thing — which is
//	              the whole reason D4 was amended to ask this instead of
//	              asking identity (MEASURED, see the pin below).
//	the footprint it writes no hook, reads nothing at a dispatch path, and
//	              leaves no temp file behind.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renderProbeRepo is a scratch git checkout with an empty hooks dir, plus
// the App the verb is asked through. qaNoHome renders the migration-case
// body (no home stamp), which is enough: this verb's question is whether
// the body REFUSES, and no literal in it decides that.
func renderProbeRepo(t *testing.T) (*App, string, string) {
	t.Helper()
	repo, hooks := qaHookRepo(t)
	return qaNoHome, repo, hooks
}

func renderProbe(t *testing.T, a *App, dir string) (int, string) {
	t.Helper()
	var b bytes.Buffer
	code := a.RunGatesRenderProbe(dir, &b)
	return code, b.String()
}

func TestRenderProbeExitsZeroWhenEveryRenderRefuses(t *testing.T) {
	t.Parallel()
	a, repo, _ := renderProbeRepo(t)
	code, out := renderProbe(t, a, repo)
	if code != 0 {
		t.Fatalf("render-probe exited %d over this binary's own renders, want 0:\n%s", code, out)
	}
	if !strings.HasPrefix(out, RenderProbeMark) {
		t.Errorf("the verdict line does not open with %q — upgradeRenderAsked reads exactly that:\n%s", RenderProbeMark, out)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if !strings.Contains(out, slot+" refuses") {
			t.Errorf("the verdict line does not say %s refuses:\n%s", slot, out)
		}
	}
}

// THE MEASUREMENT ADR 0092 D4 amendment 3 rests on, as a pin: the answer
// does not move with what is installed at the dispatch path. If it did, this
// verb would be the identity reading under another name and the amendment
// would have bought nothing — a render change would make it non-zero in
// every declared repo and roll back exactly the landings D7 expects to leave
// a stale copy behind.
//
// MEASURED 2026-11-13 on this box over the real source checkout too, by
// moving git's hook dispatch path rather than its files: fresh, neutered
// (`exit 0` bodies) and absent all answered 0 with the same verdict line.
// docs/notes.d/ranger-base-apf87.md has the readings.
func TestRenderProbeAnswerDoesNotMoveWithTheInstalledHooks(t *testing.T) {
	t.Parallel()
	a, repo, hooks := renderProbeRepo(t)

	want, wantOut := renderProbe(t, a, repo)
	if want != 0 {
		t.Fatalf("the absent-hooks reading is %d, want 0:\n%s", want, wantOut)
	}

	for _, state := range []struct {
		name string
		body string
	}{
		// A wall that refuses nothing — what a repo whose hooks were
		// neutered, or whose employer's dispatcher lost our arm, looks
		// like. The one state that would flip an identity-or-behaviour
		// reading of the DISPATCH PATH.
		{"neutered", "#!/bin/sh\nexit 0\n"},
		// Foreign bytes, and not ours at any version.
		{"foreign", "#!/bin/sh\n# somebody else's hook\nexit 3\n"},
		// Ours, as this binary renders it — the "fresh" state.
		{"ours", ""},
	} {
		t.Run(state.name, func(t *testing.T) {
			for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
				body := state.body
				if body == "" {
					body = PrePushHook
					if slot == "prepare-commit-msg" {
						visibility, _ := a.BeadsVisibility(hookRepo(repo))
						identity, _ := a.commitGuardLiterals(repo)
						body = a.CommitGuardHook(visibility, a.OpsPatternSet(), identity...)
					}
				}
				if err := WriteExecutable(filepath.Join(hooks, slot), []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			code, out := renderProbe(t, a, repo)
			if code != want {
				t.Errorf("with %s hooks installed render-probe exits %d, want the same %d it gave with none:\n%s", state.name, code, want, out)
			}
			if out != wantOut {
				t.Errorf("with %s hooks installed the verdict line reads\n  %q\nwant\n  %q", state.name, out, wantOut)
			}
		})
	}
}

// The footprint: nothing is written at the dispatch path, and the private
// scratch directory is gone when the call returns.
func TestRenderProbeWritesNoHookAndLeavesNoScratchBehind(t *testing.T) {
	a, repo, hooks := renderProbeRepo(t)

	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	if code, out := renderProbe(t, a, repo); code != 0 {
		t.Fatalf("render-probe exited %d: %s", code, out)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooks, slot)); err == nil {
			t.Errorf("render-probe installed a %s hook — it is read-only by contract (ADR 0092 D4 amendment 3)", slot)
		}
	}
	left, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range left {
		if strings.HasPrefix(e.Name(), "posse-l3-probe-") {
			t.Errorf("render-probe left its scratch directory %s behind", e.Name())
		}
	}
}

// Exit 2, and it is not exit 1: "I could not ask" and "a render does not
// refuse" are the two readings `posse upgrade` must never confuse — the
// first is a property of the box and the second rolls an install back.
func TestRenderProbeExitsTwoWhenItCannotAsk(t *testing.T) {
	a, repo, _ := renderProbeRepo(t)
	// No private directory to render into: probeScratchDir's MkdirTemp is
	// the first thing the probe needs and $TMPDIR is where it asks for it.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-such-dir"))
	code, out := renderProbe(t, a, repo)
	if code != 2 {
		t.Fatalf("a probe that could not be taken exited %d, want 2:\n%s", code, out)
	}
	if !strings.Contains(out, "could not ask") {
		t.Errorf("the line does not say the probe could not be taken:\n%s", out)
	}
	if !strings.HasPrefix(out, RenderProbeMark) {
		t.Errorf("even the could-not-ask line must carry the mark, or the caller reads it as a binary without the verb:\n%s", out)
	}
}

// A directory that is not a checkout is exit 2 and not exit 1: both renders
// are exec'd with it as their cwd and both run git, so there is nothing to
// ask rather than a render that failed.
func TestRenderProbeExitsTwoOnSomethingThatIsNotACheckout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		dir  func(t *testing.T) string
		say  string
	}{
		{"a plain directory", func(t *testing.T) string { return t.TempDir() }, "not a git checkout"},
		{"a path that is not there", func(t *testing.T) string { return filepath.Join(t.TempDir(), "gone") }, "not a directory"},
		{"a file", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "f")
			if err := os.WriteFile(p, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return p
		}, "not a directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out := renderProbe(t, qaNoHome, tc.dir(t))
			if code != 2 {
				t.Fatalf("%s exited %d, want 2:\n%s", tc.name, code, out)
			}
			if !strings.Contains(out, tc.say) {
				t.Errorf("the line does not say %q:\n%s", tc.say, out)
			}
		})
	}
}

// The verb's exit 1 arm, asked one level down because the only way to reach
// it through the verb is a binary whose own render is broken. execOwnRenders'
// two-valued face cannot tell this from "could not ask", which is why
// execOwnRendersAsked exists.
func TestExecOwnRendersAskedTellsADeadRenderFromAnUnaskableProbe(t *testing.T) {
	repo, _ := qaHookRepo(t)

	push, commit, err := execOwnRendersAsked(repo, true, "#!/bin/sh\nexit 0\n")
	if err != nil {
		t.Fatalf("a render that exits 0 is an ANSWER, not an unaskable probe: %v", err)
	}
	if !push {
		t.Errorf("the pre-push render is this binary's own and must refuse")
	}
	if commit {
		t.Errorf("a commit render that exits 0 must read as not refusing")
	}
	// And the two-valued face folds both into false, which is what every
	// launch-time caller wants.
	if p, c := execOwnRenders(repo, true, "#!/bin/sh\nexit 0\n"); !p || c {
		t.Errorf("execOwnRenders = (%v, %v), want (true, false)", p, c)
	}

	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-such-dir"))
	if _, _, err := execOwnRendersAsked(repo, true, qaNoHome.CommitGuardHook(VisibilityPublic, OpsPatternSet{})); err == nil {
		t.Error("a probe with no directory to render into reported an ANSWER")
	}
}
