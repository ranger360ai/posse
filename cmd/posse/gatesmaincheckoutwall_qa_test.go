package main

// QA pin for ranger-base-hi2lt — ADR 0095 D3: `posse gates <persona>` names
// the main-checkout refusal before a launch does.
//
// WHY THERE IS A PIN ON A PRINT, and it is the same reason D3 is a decision at
// all. D1's refusal lands at the render, so the first thing an operator would
// otherwise learn from is a `posse new` that dies — or, worse, one that does
// not: before D1 the seat launched green and was killed minutes later by the
// next dispatch hire or reap in that repo, filing a P1 security bead each
// time (ranger-base-166va, -uppgy). `posse gates` is the surface this shop
// reads a wall off, so the refusal has to be legible there, standing in the
// repo, with nothing launched.
//
// WHAT IT PINS THAT THE in-package ARMS CANNOT. internal/posse's arms
// (TestBwrapArgsRefusesAMainCheckoutsGitConfigDenyInsideAGrant and its
// neighbours) pin the refusal and its message. This pins that the message
// REACHES the operator: the gates report's wall block is the only call site
// that renders this wall for a reader, and it prints the error through one
// generic arm (`<mech> wall not rendered: <err>`, main.go). An arm that
// swallowed, truncated or summarised the error would leave every in-package
// arm green and the operator with a wall of rows and no reason.
//
// MEASURED BY HOST, like the cage-realize pin next door: the refusal is
// bwrap's, so there is something to read on a Linux host and nothing to read
// on darwin, where seatbelt carves by path rule and has no mount for a rename
// to take off (ADR 0095's Consequences). The arm says which it is rather than
// asserting one shape on both.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestQAGatesNamesTheMainCheckoutGitConfigRefusal(t *testing.T) {
	t.Parallel()
	if posse.WallMechanism() != posse.WallBwrap || !posse.AvailableCages[posse.CageSeatbelt] {
		t.Skipf("NOTHING MEASURED: this host realizes the seatbelt tier with %s and D1's refusal is bwrap's — on darwin the wall is a path rule with no mount for a rename to detach (ADR 0095 Consequences)", posse.WallMechanism())
	}
	bin := buildRhq(t)
	home := gcrHome(t, "")

	// THE CWD IS THE WHOLE FIXTURE. `posse gates` computes the matrix and the
	// wall for THIS SHELL's cwd (main.go says so in words), so standing in a
	// main checkout is standing where the refusal lives: `.git` is a
	// directory inside the session dir, the session dir is the grant, and
	// `.git/config`'s deny is therefore a mount inside a writable directory.
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	config := filepath.Join(repo, ".git", "config")
	if _, err := os.Stat(config); err != nil {
		t.Fatalf("a `git init` left no %s to deny (%v) — this fixture is not the shape D1 refuses", config, err)
	}

	cmd := exec.Command(bin, "gates", "light")
	cmd.Dir = repo
	cmd.Env = []string{
		"HOME=" + t.TempDir(),
		"RHQ_HOME=" + home,
		"RHQ_PERSONA=",
		// D19's precondition, satisfied so this arm can reach D1's. Without
		// it the bwrap render refuses one decision earlier — claude resolves
		// its config to a FILE outside every directory grant, and binding a
		// file loses every write its writer makes (ADR 0080 D19, probe B8) —
		// and this pin would read that refusal as D3's row. It is a real
		// refusal and not fixture noise: on this box the launcher sets the
		// variable in its unit, which is what the D19 message asks for.
		posse.ClaudeConfigDirVar + "=" + filepath.Join(t.TempDir(), "claude"),
		// gcrGates' reason: a reader that reached the operator's live herdr
		// socket would be red per-box. The rows this arm reads print anyway.
		"HERDR_SOCKET_PATH=" + filepath.Join(t.TempDir(), "no-such.sock"),
		"PATH=" + os.Getenv("PATH"),
	}
	out, _ := cmd.CombinedOutput()
	got := string(out)

	// THE ROW, CUT OUT FIRST (ranger-base-pl3ck's lesson, two files over):
	// asserted over the whole output this would be satisfied by any line that
	// happens to mention the repo — and the report prints the session dir as
	// a write grant on a green run, so "names the path" is not a claim about
	// the refusal at all.
	row := ""
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "wall not rendered") {
			row = l
			break
		}
	}
	if row == "" {
		t.Fatalf("`posse gates` standing in a main checkout printed no wall-refusal row, so an operator reads a wall of green for a seat that cannot launch (ADR 0095 D3):\n%s", got)
	}
	// Everything a reader has to get from this one line: which mechanism, the
	// file, why a mount cannot hold it, whose rule kills the seat, where the
	// decision is, and what to do instead.
	for _, want := range []string{
		posse.WallBwrap,
		config,
		"rename",
		"ranger-base-0qpsw",
		"ADR 0095",
		"linked worktree",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("the gates wall row does not name %q — the refusal reached the operator with part of its reason missing:\n%s", want, row)
		}
	}
}
