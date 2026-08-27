package rhq

// QA pins for SESSION MANAGEMENT parity across runtimes (ranger-base-fbxg).
//
// The bead's question: everything the harness does TO a session — not what
// the agent does inside it — must behave the same whichever runtime is in
// the pane. Most of that code is runtime-agnostic BY CONSTRUCTION, and this
// file pins the two places where it is not, plus the one message defect that
// rides along with the reap guard.
//
// What is deliberately NOT here: `posse new`, relaunch, and kill end to end
// on codex and grok. Those cost real sessions on the operator's own accounts
// and the bead says so — "COST: real sessions. Opt-in, operator-run, never
// CI." A test file is the wrong place to spend somebody's money.

import (
	"path/filepath"
	"strings"
	"testing"
)

// PROMPT DELIVERY is the column the landing turn depends on and does not
// read (ranger-base-ewq9). Pinned here so a change to it is noticed by the
// side of the harness that will silently keep typing.
func TestPromptDeliveryColumnIsWhatTheLandingTurnMustAsk(t *testing.T) {
	a := wtApp(t)
	for name, want := range map[string]string{
		"claude": PromptTyped, // works, and ADR 0013 left it alone
		"codex":  PromptArgv,  // first-run interstitial (ADR 0013 §2)
		"grok":   PromptArgv,  // a pane with no turn matches no herdr rule
	} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := rt.PromptMode(); got != want {
			t.Errorf("%s prompt mode = %q, want %q", name, got, want)
		}
	}
}

// The gap ranger-base-ewq9 names: landThePlane (relaunch.go:484) types the
// landing prompt into every runtime, where every other AgentPrompt call site
// branches on PromptMode first (dispatch.go:1915, herdrback.go:1231). On
// codex and grok that is delivery by the one mechanism ADR 0013 measured as
// unreliable there — so the turn that writes a session's lessons down and
// commits them is unverified on two of three runtimes.
func TestLandingTurnAsksTheRuntimeHowToDeliverIt(t *testing.T) {
	t.Skip("ranger-base-ewq9: landThePlane types into codex and grok, which are prompt: argv precisely because typing was measured unreliable — unskip with the fix")
}

// THE REAP GUARD's message defect (ranger-base-8ogq, the bead's 4d9x).
// MEASURED 2026-08-27 on git 2.x: `git status --porcelain` emits a LEADING
// SPACE in the status column for a worktree-modified path (" M alpha.txt"),
// git() returns strings.TrimSpace of the WHOLE output, so only the FIRST
// line loses that space — and dirtyPaths' ln[3:] then cuts one real
// character off it. Second and later lines keep their space and survive, so
// the bug is invisible in any single-file test.
//
// It is a message defect, not a loss: the guard still REFUSES, it just names
// the tree wrong in the one line the operator reads to recognize it.
func TestDirtyPathsKeepsEveryPathWhole(t *testing.T) {
	t.Skip("ranger-base-8ogq: git() TrimSpaces the whole porcelain output, so the first dirty path loses its first character — unskip with the fix")

	repo := wtRepo(t)
	write(t, filepath.Join(repo, "alpha.txt"), "seed\n")
	write(t, filepath.Join(repo, "beta.txt"), "seed\n")
	mustGit(t, repo, "add", "alpha.txt", "beta.txt")
	mustGit(t, repo, "commit", "-q", "-m", "two tracked files")
	write(t, filepath.Join(repo, "alpha.txt"), "seed\nchanged\n")
	write(t, filepath.Join(repo, "beta.txt"), "seed\nchanged\n")

	got := strings.Join(dirtyPaths(repo), " ")
	if want := "alpha.txt beta.txt"; got != want {
		t.Errorf("dirtyPaths = %q, want %q", got, want)
	}
}

// THE CAGE dimension richard added (ranger-base-il14): codex and grok have
// no decided container credential, so a caged launch must REFUSE with that
// reason rather than start a session that cannot authenticate. Verified
// here rather than by manufacturing a container run the bead forbids.
func TestCageCredentialRefusesTheUndecidedRuntimes(t *testing.T) {
	a := wtApp(t)
	for _, name := range []string{"codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if CageCredential(rt) != "" {
			t.Errorf("%s now names a container credential — rangerhq-kiz was settled and this pin is stale", name)
		}
		// Even with a full environment, an undecided runtime refuses: the
		// refusal is about the DECISION being open, not about the env.
		err = CheckCageCredential(rt, []string{"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY"})
		if err == nil {
			t.Fatalf("%s: a caged launch was allowed with no decided container credential", name)
		}
		if !strings.Contains(err.Error(), "no container credential is decided") {
			t.Errorf("%s refusal does not say why: %v", name, err)
		}
	}

	// claude is the one decided runtime, and its refusal is the other shape:
	// the credential is known, this session just does not carry it.
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckCageCredential(claude, nil); err == nil {
		t.Error("claude: a caged launch was allowed with no credential in the environment")
	}
	if err := CheckCageCredential(claude, []string{"CLAUDE_CODE_OAUTH_TOKEN"}); err != nil {
		t.Errorf("claude: an authenticated caged launch was refused: %v", err)
	}
}
