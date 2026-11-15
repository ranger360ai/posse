//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-p8ymi — `posse upgrade` tests, builds and runs its
// epilogue from a PRIVATE copy of the commit whose every object was
// re-hashed on the way in, never from a `git worktree add` of the source
// checkout.
//
// The subject is the shared object store. git trusts an object's bytes by
// the name it is filed under and does not re-hash them on a local read, and
// every walled worktree seat is granted <common>/objects. So a worktree of
// the source checks out whatever bytes a seat left under each name, and the
// verb's caller — uncaged — runs them as `make test`. Each pin below plants
// that in a fixture repo and asks the verb to refuse before upgradeTest is
// reached:
//
//	a substituted blob     one loose blob origin/main reaches, rewritten.
//	                       Red if upgradePrivateCopy is put back to a
//	                       worktree add: the premise below shows that shape
//	                       checking the substituted bytes out.
//	a forged ancestry      a plan whose `to` the shared store said was on
//	                       origin/main and the re-hashed history does not
//	                       reach. Red if the copy stops taking D3.1's
//	                       answers again in its own store.

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// upgradeObjectsDir is the absolute object directory a checkout reads.
func upgradeObjectsDir(t *testing.T, dir string) string {
	t.Helper()
	p := mustGit(t, dir, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p
}

// substituteLooseBlob overwrites the loose object filed under sha with a
// well-formed blob of different bytes — what a seat granted <common>/objects
// can do. The name stays; only the content moves.
func substituteLooseBlob(t *testing.T, repo, sha, body string) {
	t.Helper()
	path := filepath.Join(upgradeObjectsDir(t, repo), sha[:2], sha[2:])
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("premise: %s is not a loose object in the fixture: %v", sha, err)
	}
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	fmt.Fprintf(zw, "blob %d\x00%s", len(body), body)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	// Loose objects are 0444; the seat's grant is to the directory, so it
	// can replace the file whatever its mode.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o444); err != nil {
		t.Fatal(err)
	}
}

// upgradeStepsThatMustNotRun stubs every step and fails the pin if any is
// reached: the refusal under test happens before the suite.
func upgradeStepsThatMustNotRun(t *testing.T) {
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error {
			body, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
			t.Errorf("`make test` ran over a store that does not hold what its names promise (f.txt reads %q) — it runs uncaged as the verb's caller (ranger-base-p8ymi)", body)
			return nil
		},
		func(w io.Writer, dir, repo, out string) error {
			t.Error("the build ran over a store that does not hold what its names promise")
			return nil
		},
		func(w io.Writer, wt string, plan upgradePlan) string {
			t.Error("the epilogue ran over a store that does not hold what its names promise")
			return upgradeGateUnchanged
		})
}

func TestUpgradeRefusesASubstitutedBlobBeforeTheSuite(t *testing.T) {
	f := newUpgradeFixture(t)
	upgradeStepsThatMustNotRun(t)
	// commit c is origin/main and carries f.txt = "v1\n"; d is the
	// unpushed landing above it with "v2\n", so this blob is reachable
	// from origin/main and from nowhere a later commit replaced it.
	blob := mustGit(t, f.repo, "rev-parse", f.c+":f.txt")
	substituteLooseBlob(t, f.repo, blob, "SEAT CODE\n")

	// PREMISE, and the shape this pin reds on: every name still resolves,
	// and a worktree of c checks out the seat's bytes under c's own sha.
	if got := mustGit(t, f.repo, "cat-file", "-p", blob); got != "SEAT CODE" {
		t.Fatalf("premise: the substituted blob reads %q, want the seat's bytes", got)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	mustGit(t, f.repo, "worktree", "add", "--detach", "--quiet", wt, f.c)
	if body, _ := os.ReadFile(filepath.Join(wt, "f.txt")); string(body) != "SEAT CODE\n" {
		t.Fatalf("premise: a worktree add of %s checked out %q, not the substituted bytes — the attack this pin is about is not reproduced", shortSha(f.c), body)
	}
	if got := mustGit(t, wt, "rev-parse", "HEAD"); got != f.c {
		t.Fatalf("premise: the worktree's HEAD is %s, want %s unchanged", got, f.c)
	}

	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v — a blob swap is invisible to D3.1's sha reads, which is the premise", err)
	}
	var out bytes.Buffer
	err = upgradeRunPlan(&out, plan)
	if err == nil {
		t.Fatalf("a substituted blob reachable from %s did not refuse the upgrade:\n%s", upgradeDefaultRef, out.String())
	}
	if !strings.Contains(err.Error(), "did not verify") || !strings.Contains(err.Error(), "fsck") {
		t.Errorf("the refusal does not say the fetch did not verify, or does not name fsck:\n%v", err)
	}
	if body, _ := os.ReadFile(f.target); string(body) != "OLD BINARY\n" {
		t.Errorf("the target changed: %q", body)
	}
	for _, p := range []string{f.staged, upgradePrevPath(f.target)} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s exists after the refusal — nothing is written before the suite answers", filepath.Base(p))
		}
	}
}

// D3.1 reads its two ancestry answers out of the shared store, where a
// rewritten commit object may claim any parent. A plan carrying an answer
// the re-hashed history does not back must refuse in the private copy.
// The plan is built by hand rather than by forging a commit object,
// because git re-hashes a LOOSE commit when rev-list parses it — the pin is
// about what the copy checks, whatever the shared store answered.
func TestUpgradeRefusesAnAncestryTheReHashedHistoryDoesNotBack(t *testing.T) {
	f := newUpgradeFixture(t)
	upgradeStepsThatMustNotRun(t)

	plan, err := upgradeResolve(f.repo, f.target, f.a, f.c)
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	// d is the unpushed landing above origin/main.
	plan.To = f.d
	err = upgradeRunPlan(io.Discard, plan)
	if err == nil {
		t.Fatalf("a `to` that %s does not reach was tested and installed", upgradeDefaultRef)
	}
	if !strings.Contains(err.Error(), "not an ancestor") {
		t.Errorf("the refusal does not say %s is not on %s:\n%v", shortSha(f.d), upgradeDefaultRef, err)
	}

	// And the other end: a `from` that is not behind `to`.
	plan, err = upgradeResolve(f.repo, f.target, f.a, f.c)
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	plan.From = f.d
	err = upgradeRunPlan(io.Discard, plan)
	if err == nil {
		t.Fatal("a `from` the re-hashed history does not put behind `to` was accepted")
	}
	if !strings.Contains(err.Error(), "not a descendant") {
		t.Errorf("the refusal does not say %s is not behind the installed stamp:\n%v", shortSha(f.c), err)
	}
}
