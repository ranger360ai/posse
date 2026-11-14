//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-hi2lt — ADR 0095 D2: what D1's refusal COSTS the one
// launch path that walks into it by itself.
//
// Three callers render a bwrap seat over a main checkout: `posse new` and a
// recipe, which never take a worktree (ADR 0008 — a crew session works in the
// operator's checkout), and DISPATCH, when `PlanSessionTree` answers nil. The
// first two are an operator typing a command and reading what it says. The
// third is a loop: dispatch asks for a worktree, a detached HEAD gives it
// none, it falls back to the shared checkout, and from D1 on that render is
// refused. So the question D2 asks is not whether the refusal is right — it is
// whether a refusal INSIDE a watch loop costs one line per pass or a
// claim/unclaim storm, and the ADR marks the answer ASSUMED.
//
// THE ANSWER, MEASURED here and no worse than assumed: one reason line per
// pass, and the bd writes are one claim and one unclaim on the argv path and
// NOTHING AT ALL on the typed one. Three arms, because the cost has three
// independent halves:
//
//   - ARM 1, the shape: the fallback really does render over the main
//     checkout, so the refusal really is this path's. Without this arm the
//     cost arms could be measuring any refusal at all — and the first time
//     this one ran they were, twice over (D19's CLAUDE_CONFIG_DIR demand
//     fires one decision earlier, and a fixture whose repo sits under the
//     blanket `/tmp` grant refuses in a LINKED worktree too; see the arm).
//   - ARM 2, the argv path's cost: claimed, refused, handed back ONCE
//     through `unclaimAfterLaunchFailure`, and the SLOT benched for the rest
//     of the pass. The bench is what makes it one line rather than one per
//     bead: the refusal is a plain error, so the fire loop's `default` arm
//     reads it as a fact about the persona on this runtime — which it is,
//     since every bead routed here renders the same wall over the same
//     checkout — and stops offering that slot (dispatch.go, ADR 0013 §2
//     property 4).
//   - ARM 3, the typed path's: no claim at all. On a `prompt: typed` runtime
//     the claim comes AFTER the create (dispatch.go's launch body), so a
//     create that refuses never reaches it. This is the cheaper half and it
//     is where this box lives today — claude is `prompt: typed` and is the
//     only runtime that renders a bwrap wall and is dispatched here — so the
//     line D2 worried about is quieter still in practice.
//
// WHAT STILL COSTS SOMETHING, said rather than wished away: the next PASS
// tries again and pays the same again. That is one line per pass, which is
// the rate every launch refusal on this route has — and a detached HEAD is
// the operator's own transient state, so a route that went quiet about it
// would be worse. The worktree is NOT cut (PlanSessionTree answered nil,
// which is the premise of the shape), so this refusal leaves nothing behind
// for `posse worktrees --land` to sweep — the one way it is cheaper than ADR
// 0076 D7's.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bmcDetached is a one-commit repo put on a detached HEAD: `MainCheckout` says
// yes, `repoBranch` says nothing, no session branch exists, so
// PlanSessionTree answers nil and a `Worktree: true` launch falls back to this
// very directory (worktree.go).
func bmcDetached(t *testing.T, repo string) string {
	t.Helper()
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, "README.md"), "seed\n")
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")
	if b := repoBranch(repo); b != "" {
		t.Fatalf("fixture: %s still reports branch %q, so PlanSessionTree would cut a tree and this is not the fallback shape", repo, b)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "config")); err != nil {
		t.Fatalf("fixture: no %s to deny (%v) — a worktree, not a main checkout", filepath.Join(repo, ".git", "config"), err)
	}
	return repo
}

// bmcSkipUnlessBwrap: the refusal is a mount mechanism's. On darwin the wall
// is a path rule and there is no mount for a rename to take off, so there is
// nothing here to measure either way (ADR 0095 Consequences).
func bmcSkipUnlessBwrap(t *testing.T) {
	t.Helper()
	if WallMechanism() != WallBwrap || !AvailableCages[CageSeatbelt] {
		t.Skipf("NOTHING MEASURED: this host realizes the seatbelt tier with %s, and D1's refusal is bwrap's", WallMechanism())
	}
}

// ARM 1. The fallback's own render, asked directly, with no pass around it.
// A non-claude runtime, for bwlSeat's reason: D19's config-dir demand is its
// own pin and not a precondition of this one.
//
// THE FIXTURE ROOT IS `wallUngrantedDir` AND THAT IS LOAD-BEARING, which is
// the second thing this arm found. `bwrapSeatTmp` answers "" when the home or
// the session dir is itself under /tmp — every fixture in this package is —
// and with no private /tmp the render keeps the blanket `/tmp` write grant
// SeatbeltWritable hands every seat. A repo under /tmp is then inside a write
// grant whatever tree shape it has, so D1's condition holds for a LINKED
// worktree there too and the control below would refuse. That is not a bug in
// either half — a seat really can write that config, and `bwrapSeatTmp`'s own
// comment says a box in that shape has no seat to protect — but it is not the
// shape D1 is about, and a fixture that cannot tell the two apart measures
// nothing.
func TestQADetachedHeadFallbackMeetsTheMainCheckoutRefusal(t *testing.T) {
	// No t.Parallel: this file's other arms set env vars, and all three read
	// the package-level cage table.
	bmcSkipUnlessBwrap(t)
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(a.RuntimesDir(), "mycli.yaml"), "command: mycli {file}\nstate_dir: ~/.mycli-bmc\n")
	write(t, filepath.Join(a.AgentsDir, "ranger.md"),
		"---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\n---\nwork\n")
	root := wallUngrantedDir(t)
	repo := bmcDetached(t, filepath.Join(root, "repo"))

	// The CONTROL first, so the refusal below is this shape's and not every
	// render's: the same persona, the same runtime, the same home, in a
	// linked worktree of the same repo — which is what D1's remedy names and
	// what every dispatched seat on this box is.
	wt := filepath.Join(root, "wt")
	mustGit(t, repo, "worktree", "add", "-q", "--detach", wt, "HEAD")
	if _, err := b.planLaunch(NewSessionOpts{Name: "bmc-wt", Dir: wt, Agent: "ranger"}); err != nil {
		t.Fatalf("a linked worktree's render was refused — then D1 is refusing the remedy it names, and no bwrap seat on this box can launch at all: %v", err)
	}

	_, err := b.planLaunch(NewSessionOpts{Name: "bmc-main", Dir: repo, Agent: "ranger", Worktree: true})
	if err == nil {
		t.Fatalf("dispatch's detached-HEAD fallback rendered a bwrap seat over %s. That seat launches green and is killed by the next hire, reap or landing check in this repo, filing a P1 security bead each time (ADR 0095 D1/D2, ranger-base-166va)", repo)
	}
	// It is D1's refusal and not some other one the fixture tripped. Named
	// by the file and the citation, because this is also what the cost arms
	// read.
	for _, want := range []string{filepath.Join(repo, ".git", "config"), "ADR 0095", "linked worktree"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the fallback was refused for some other reason than D1 (%q missing): %v", want, err)
		}
	}
	// AND THE WORKTREE WAS NOT CUT, which is the half that differs from ADR
	// 0076 D7's refusals: there the tree is made two hundred lines above
	// every refusal and waits for a sweep. Here PlanSessionTree answered nil
	// — that nil IS the premise of the shape — so there is nothing to sweep.
	tree, err := a.SessionTreePath(repo, "bmc-main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tree); err == nil {
		t.Errorf("the refused launch left a session tree at %s — on a detached HEAD PlanSessionTree answers nil and nothing should have been cut", tree)
	}
}

// ARMS 2 AND 3. The cost through a real pass, on both delivery paths.
//
// WHICH RUNTIME IS THE WHOLE OF THE SPLIT, and it is why this is a table and
// not one test. `unclaimAfterLaunchFailure` is on the ARGV path, where the
// claim is the fence and goes FIRST (ADR 0013 §2 step 1) — grok is this
// fleet's `prompt: argv` runtime that posse wraps (codex declares
// SelfSandbox, so no posse wall is rendered for it at any tier and it can
// never meet D1). On a `prompt: typed` runtime the claim comes after the
// create, so a refused create costs no bd write at all — and claude, the one
// runtime a bwrap seat on this box actually runs, is typed.
func TestQADetachedHeadFallbackRefusalCostsOneLinePerPass(t *testing.T) {
	// No t.Parallel: t.Setenv.
	bmcSkipUnlessBwrap(t)
	// D19's precondition, satisfied so a claude render reaches D1's refusal
	// instead of stopping at claude's config file. On this box the launcher
	// sets it in its unit, which is what D19's message asks for.
	t.Setenv(ClaudeConfigDirVar, filepath.Join(t.TempDir(), "claude"))
	for _, tc := range []struct {
		runtime string
		claims  int // bd claim/unclaim pairs this refusal costs
	}{
		{"grok", 1},   // argv: claim is the fence, so the refusal hands it back
		{"claude", 0}, // typed: the claim never happened
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			write(t, filepath.Join(b.App.AgentsDir, "ranger.md"),
				"---\nname: ranger\ndescription: t\nruntime: "+tc.runtime+"\ncage: seatbelt\nlabels: [go]\n---\nwork\n")
			repo := bmcDetached(t, t.TempDir())
			write(t, filepath.Join(repo, "fake-ready.json"), `[{"id":"a-1","title":"t","labels":["go"]}]`)
			write(t, b.App.ConfigPath, "beads:\n  - "+repo+"\n")

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if n != 0 {
				t.Fatalf("a refused render dispatched %d bead(s):\n%s", n, out)
			}
			// The premise: this pass really met D1 and not some earlier
			// refusal. Without it every assertion below is satisfiable by a
			// launch that died for an unrelated reason.
			if !strings.Contains(out, "ADR 0095") {
				t.Fatalf("the pass did not print D1's refusal, so this row is measuring some other failure:\n%s", out)
			}
			// ONE line, and it is the refusal itself. Counted rather than
			// read, because "one reason line per pass" is the whole of what
			// D2 asked and a loop that printed it per bead or per tick would
			// still print it.
			if got := strings.Count(out, "ADR 0095"); got != 1 {
				t.Errorf("D1's refusal was printed %d times in one pass, want 1:\n%s", got, out)
			}
			bd := bdCalls(t, fake)
			if got := strings.Count(bd, "--claim"); got != tc.claims {
				t.Errorf("%d claims for one refused launch on a %s seat, want %d — a refusal inside a watch loop must not fan out into a claim storm:\n%s", got, tc.runtime, tc.claims, bd)
			}
			// Handed back exactly as often as it was taken: the bead must not
			// be left in_progress with nobody working it (rangerhq-81d), and
			// must not be unclaimed more than once either.
			if got := strings.Count(bd, "--status open"); got != tc.claims {
				t.Errorf("%d unclaims for one refused launch on a %s seat, want %d:\n%s", got, tc.runtime, tc.claims, bd)
			}
			if tc.claims > 0 && !strings.Contains(out, "unclaimed") {
				t.Errorf("the argv path claimed and did not say it handed the bead back (ADR 0013 §2 step 5):\n%s", out)
			}
			// And the slot is benched, which is what makes it one line per
			// PASS rather than one per bead: a plain error is a fact about
			// the persona on this runtime, and every bead routed here renders
			// the same wall over the same checkout.
			if !strings.Contains(out, "skipped for the rest of this pass") {
				t.Errorf("the refusal did not bench the persona/runtime slot — every bead routed here would pay the same again (ADR 0013 §2 property 4):\n%s", out)
			}
		})
	}
}
