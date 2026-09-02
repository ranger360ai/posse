package posse

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
//
// That live half HAS since been run, once, by hand and with the operator's
// authorization: ranger-base-i0qp, 2026-08-29, real codex and real grok.
// Its table is the answer to this bead and lives on those two beads, not in
// this file. Two cells it could not fill without handing fixture beads back
// to the shared queue are pinned at the bottom of this file instead.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PROMPT DELIVERY is the column the landing turn depends on and does not
// read (ranger-base-ewq9). Pinned here so a change to it is noticed by the
// side of the harness that will silently keep typing.
func TestPromptDeliveryColumnIsWhatTheLandingTurnMustAsk(t *testing.T) {
	t.Parallel()
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

// ranger-base-ewq9, RE-SCOPED BY MEASUREMENT AND NOW CLOSED — read this
// before writing an argv landing path, because the failure this test was
// filed against is not there. What was PREDICTED — that the typed landing
// prompt does not reach a codex or grok pane — was measured on real
// sessions (the QA lane's live pass, 2026-08-29, ranger-base-i0qp) and
// REFUTED: four landings, both runtimes, including on a pane that had never
// had a turn, and grok's landing turn did the durable half in full
// (ORDERS.md lessons + a bead comment). ADR 0013 §2's "a pane with no turn
// matches no rule" did not reproduce on manifest 2026.07.16.105.
//
// The consistency half was then RESOLVED, 2026-08-31 (d3a3fed): landThePlane
// stays branchless on purpose. Every other AgentPrompt caller consults
// PromptMode() to place a CREATE's work prompt — argv line or typed — and
// landThePlane never creates, so typed delivery is the only mechanism there
// is on every runtime.
//
// So this stays a SKIP: the question it was parked for has an answer, and
// the answer is pinned behaviourally rather than here —
// TestQALandingTurnIsTypedOnEveryRuntimePromptArgvIncluded
// (landingparity_qa_test.go) relaunches a session on claude, codex and grok
// and asserts the typed `agent prompt` goes out on all three, so the branch
// this test was going to demand is now the thing the suite refuses. What is
// pinned here is the column that decision would have read.
func TestLandingTurnAsksTheRuntimeHowToDeliverIt(t *testing.T) {
	t.Parallel()
	t.Skip("ranger-base-ewq9: CLOSED 2026-08-31 — landThePlane is branchless by decision, not by oversight (d3a3fed); the delivery itself is pinned by TestQALandingTurnIsTypedOnEveryRuntimePromptArgvIncluded")
}

// THE REAP GUARD's message defect (ranger-base-8ogq, the bead's 4d9x).
// MEASURED 2026-08-27 on git 2.x: `git status --porcelain` emits a LEADING
// SPACE in the status column for a worktree-modified path (" M alpha.txt"),
// git() returns strings.TrimSpace of the WHOLE output, so only the FIRST
// line loses that space — and dirtyPaths' ln[3:] then cuts one real
// character off it. Second and later lines keep their space and survive, so
// the bug is invisible in any single-file test.
//
// For a path of normal length that is a message defect and not a loss — the
// guard still REFUSES, it just names the tree wrong in the one line the
// operator reads to recognize it. The next test is where it stops being
// cosmetic.
func TestDirtyPathsKeepsEveryPathWhole(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "alpha.txt"), "seed\n")
	write(t, filepath.Join(repo, "beta.txt"), "seed\n")
	mustGit(t, repo, "add", "alpha.txt", "beta.txt")
	mustGit(t, repo, "commit", "-q", "-m", "two tracked files")
	write(t, filepath.Join(repo, "alpha.txt"), "seed\nchanged\n")
	write(t, filepath.Join(repo, "beta.txt"), "seed\nchanged\n")

	// The fixture is only worth what its first line is: assert git really
	// emits the leading-space form here (gitRaw, since git() is the very
	// trim under test), or a green test proves nothing.
	raw, err := gitRaw(repo, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), " M alpha.txt") {
		t.Fatalf("fixture does not produce a worktree-modified FIRST line, so this pins nothing: %q", raw)
	}

	got := strings.Join(dirtyPaths(repo), " ")
	if want := "alpha.txt beta.txt"; got != want {
		t.Errorf("dirtyPaths = %q, want %q", got, want)
	}
}

// The same defect's worse half, which the bead (ranger-base-8ogq) called out
// and did not reproduce: a ONE-character worktree-modified path renders as
// " M a", four bytes — trim the blob and it is three, which the old
// `len(ln) > 3` dropped outright. That is not a misnamed path, it is a dirty
// file reported CLEAN, and the reap guard would have removed the tree.
func TestDirtyPathsSeesTheShortestPossiblePath(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "a"), "seed\n")
	mustGit(t, repo, "add", "a")
	mustGit(t, repo, "commit", "-q", "-m", "one short name")
	write(t, filepath.Join(repo, "a"), "seed\nchanged\n")

	if got := dirtyPaths(repo); len(got) != 1 || got[0] != "a" {
		t.Errorf("dirtyPaths = %q, want [a] — a dirty file the guard cannot see is one it will destroy", got)
	}
}

// The other two porcelain shapes — staged ("M  alpha.txt") and untracked
// ("?? zulu.txt"), both with the column filled and no leading space to lose.
// Stated honestly: no mutation of the CURRENT fix breaks this one (the wrong
// fix, trimming each line before slicing, is caught by the test above, and
// the trailing TrimSpace(ln[3:]) absorbs the rest). It is breadth, not a
// discriminating pin: it is here so a future rewrite of this parse — to
// --porcelain=v2, to -z records — has the forms the other two tests do not
// cover written down.
func TestDirtyPathsKeepsStagedAndUntrackedPathsWhole(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "alpha.txt"), "seed\n")
	mustGit(t, repo, "add", "alpha.txt")
	mustGit(t, repo, "commit", "-q", "-m", "one tracked file")
	write(t, filepath.Join(repo, "alpha.txt"), "seed\nchanged\n")
	mustGit(t, repo, "add", "alpha.txt")               // "M  alpha.txt"
	write(t, filepath.Join(repo, "zulu.txt"), "new\n") // "?? zulu.txt"

	got := strings.Join(dirtyPaths(repo), " ")
	if want := "alpha.txt zulu.txt"; got != want {
		t.Errorf("dirtyPaths = %q, want %q", got, want)
	}
}

// THE CAGE dimension architect added (ranger-base-il14): codex and grok have
// no decided container credential, so a caged launch must REFUSE with that
// reason rather than start a session that cannot authenticate. Verified
// here rather than by manufacturing a container run the bead forbids.
func TestCageCredentialRefusesTheUndecidedRuntimes(t *testing.T) {
	t.Parallel()
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

// ─── the two cells the live pass could not fill (ranger-base-i0qp) ───────────
//
// The QA lane's operator-authorized live pass (2026-08-29) filled every
// other row of this bead's table on real codex and real grok sessions. Two cells came
// back honestly NOT MEASURED, both for good reasons:
//
//   merge-back from the DISPATCH LOOP — the live pass landed both runtimes'
//   branches through `posse worktrees --land` (the sweep), including the
//   conflict path. Driving the loop's own trigger would have meant handing
//   the fixture beads back to the shared queue, which is the one thing that
//   fixture existed to avoid.
//
//   prune on READ — no session died underneath its meta during the pass, so
//   only prune-on-KILL was exercised (six times, both runtimes).
//
// Both were argued AGNOSTIC BY CONSTRUCTION, and an argument from reading is
// what these two tests turn into a measurement. Neither costs a session.

// MERGE-BACK, per runtime, through the function the dispatch loop calls when
// it judges a close (dispatch.go gather -> d.mergeBack). It takes the bead,
// the persona and the SESSION NAME — the runtime reaches it only as a field
// on the meta it reads, and MergeSessionWork below it never sees one at all.
// So the pin is the whole table producing the same landing: same ⤴ line,
// same commit count, same work on the repo's own branch.
//
// MUTATION-CHECKED, because a green parity table is exactly the shape that
// can be green for no reason: with `if m.Runtime != DefaultRuntime { return }`
// added at the top of mergeBack, claude PASSES and codex and grok both FAIL —
// the three arms this table is for. (Gutting mergeBack entirely fails all
// three, which is the control that says the pin measures this function and
// not the fixture.)
func TestMergeBackLandsEveryRuntimesSessionTheSameWay(t *testing.T) {
	t.Parallel()
	for _, rt := range parityRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			parityPersona(t, b.App, "ranger", "[go]", rt.name)
			repo := wtqaRepo(t, b.App, `[]`, `[{"id":"a-1","status":"closed"}]`)
			idleClaude(t, fake)

			session := SessionForBead("ranger", repo, "a-1")
			mustCreate(t, b, NewSessionOpts{
				Name: session, Dir: repo, Agent: "ranger", Runtime: rt.name,
				Worktree: true, Bead: "a-1",
			})
			m, ok := b.readMeta(session)
			if !ok {
				t.Fatal("the session has no meta")
			}
			// The fixture is only worth what its runtime field says: a
			// session that recorded `claude` would make all three subtests
			// the same test three times.
			if m.Runtime != rt.name {
				t.Fatalf("meta records runtime %q, want %q — this row measures nothing", m.Runtime, rt.name)
			}
			tr := SessionTreeOf(m)
			if tr == nil {
				t.Fatalf("a dispatched session got no tree: %+v", m)
			}
			commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")

			var out strings.Builder
			d := &Dispatcher{App: b.App, HB: b, Out: &out}
			d.mergeBack(RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t"}, Dir: repo}, "ranger", session)

			if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the persona's work\n" {
				t.Fatalf("%s: a closed bead's commit did not reach %s: %v\n%s", rt.name, tr.Base, err, out.String())
			}
			for _, want := range []string{"a-1", "1 commit(s)", "fast-forwarded", tr.Branch, tr.Base} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("%s: the merge-back line does not say %q:\n%s", rt.name, want, out.String())
				}
			}
		})
	}
}

// PRUNE ON READ, per runtime. Sessions() is where a meta whose workspace
// died is unlinked, and its decision reads the herdr listing, the socket and
// the server generation — the runtime is carried through as an output field
// and is never an input. Three metas differing ONLY in `runtime:`, all three
// naming a workspace this server can prove is gone.
//
// The wrong arm is in the same test on purpose: three more metas, same three
// runtimes, written against ANOTHER herdr's socket. Those must all be KEPT,
// because "this server never held it" is not "it died" (rangerhq-snd).
//
// MUTATION-CHECKED, three ways. A runtime gate on the prune (`if m.Runtime ==
// "grok" { continue }`) fails the dead arm on grok alone. Making
// cannotAnswerFor answer "" — this server can speak for every meta — fails
// the kept arm on all three. And one that does NOT kill it, worth writing
// down: deleting the cannotAnswerFor call from the LISTING side changes
// nothing, because reclaim re-asks it under the launch lock before the
// unlink. The socket guard has two arms, and the second one is the one that
// actually holds the file.
func TestPruneOnReadIsBlindToTheRuntime(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, _ := newTestBackend(t)
	warn := &syncBuf{}
	b.Warn = warn

	// A live session keeps the board non-empty: an empty listing is one of
	// the three shapes Sessions() refuses to prune on at all, and a test
	// that hit it would spare every meta for a reason that has nothing to
	// do with runtimes.
	mustCreate(t, b, NewSessionOpts{Name: "live"})

	runtimeMeta := func(name, runtime, socket string) {
		t.Helper()
		if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, b.metaPath(name), "name: "+name+"\nworkspace: w404\npane: w404:p1\nemoji: x\n"+
			"runtime: "+runtime+"\nsocket: "+socket+"\n")
	}
	for _, rt := range parityRuntimes {
		runtimeMeta("dead-"+rt.name, rt.name, raceSock)                   // ours, and gone
		runtimeMeta("foreign-"+rt.name, rt.name, "/tmp/other/herdr.sock") // another server's
	}

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}

	for _, rt := range parityRuntimes {
		if _, ok := b.readMeta("dead-" + rt.name); ok {
			t.Errorf("%s: a meta whose workspace this server proved dead was kept — prune-on-read is not runtime-blind", rt.name)
		}
		if _, ok := b.readMeta("foreign-" + rt.name); !ok {
			t.Errorf("%s: a meta written against ANOTHER herdr was deleted — absence from this listing is not death (rangerhq-snd)", rt.name)
		}
	}
}
