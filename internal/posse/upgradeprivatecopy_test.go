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
	"runtime"
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

// ─── and where that copy LIVES ───────────────────────────────────────────────

// Pin for ranger-base-ua625 finding 2, from the verify at
// ranger-base-pxprm. Re-hashing the objects closed what the copy is MADE of;
// it said nothing about who can write the copy afterwards. The copy sat
// directly under $TMPDIR else /tmp, and seatbeltTempGrants puts both in the
// writable set of EVERY rendered profile, blanket — so for the whole of the
// `make test` window a seat could write the tree that `make test` then runs
// UNCAGED, as this verb's caller.
//
// The fix is a fixed name under a temp root the deny names unconditionally,
// denied by name in every profile (upgradePrivateRootName says why the root
// moved no further than a temp root at all). So the pin has six rows and they
// have to agree: the root is the same whatever TMPDIR the verb reads (1a) and
// is denied for whatever TMPDIR a launch read (1b), the carve-out denies under
// every blanket grant (2) and the rendered profile carries it (3), the verb
// stages where the carve-out denies (4), a link planted at that name is
// refused rather than followed (5), and a root nothing denies is refused
// before anything is fetched (6).
//
// ROWS 1 AND 6 ARE ranger-base-18brx, finding 1 of the verify at ranger-base-m5izv.
// This pin used to set ONE TMPDIR and use it for both halves, so the one
// thing its five rows could not see was the two halves DISAGREEING — and
// they are read by different processes, the launch that rendered the profile
// and the run that stages the copy. Where the verb's TMPDIR resolved
// strictly inside one of the launch's blanket grants, the copy was in a
// seat's write grant under a name no deny covered.
func TestQAUpgradeStagesThePrivateCopyWhereEveryProfileDeniesWrites(t *testing.T) {
	// No t.Parallel: TMPDIR and HOME.
	root := wallUngrantedDir(t)
	tmpdir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpdir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmpdir)

	grants := seatbeltTempGrants(runtime.GOOS, tmpdir)
	if len(grants) == 0 {
		t.Skip("NOTHING MEASURED: seatbeltTempGrants is stubbed empty on this box, so there is no blanket grant to narrow")
	}

	// 1a. THE ROOT DOES NOT MOVE WITH THE VERB'S TMPDIR. Every value below
	// is one an operator or an installer could be running under, and the
	// third and fifth are the shape of the hole: a path strictly inside a
	// blanket grant, which a deny over the grant's ROOT does not cover.
	// WHICH member of the deny set it is does not matter and is not graded:
	// every member is denied in every profile, so `denied[0]` in place of
	// the last entry is green on purpose (mutation m2,
	// docs/notes.d/ranger-base-18brx.md). What is graded is that it does not
	// MOVE.
	fixed := upgradePrivateRoot()
	for _, verb := range []string{"", "/tmp", "/tmp/op", tmpdir, filepath.Join(tmpdir, "deeper"), "/var/folders/ab/cd1234/T"} {
		t.Setenv("TMPDIR", verb)
		if got := upgradePrivateRoot(); got != fixed {
			t.Errorf("upgradePrivateRoot() moved to %s under TMPDIR=%q (it was %s under TMPDIR=%q) — the deny is rendered at a SEAT'S launch and this is read when the VERB runs, so a root that moves with this reading is a root the deny can miss (ranger-base-18brx)", got, verb, fixed, tmpdir)
		}
	}
	t.Setenv("TMPDIR", tmpdir)

	// 1b. AND IT IS DENIED FOR EVERY LAUNCH TMPDIR, which is the other half
	// of the same question: whatever a seat's own launch read, the profile
	// it rendered denies the name this verb stages under. Both GOOS values,
	// for seatbeltTempGrants' reason — the darwin answer has to be readable
	// on this box, and darwin's default TMPDIR (/var/folders/…) is the row
	// that was fail-safe for a reason no deny provided.
	for _, goos := range []string{"linux", "darwin"} {
		r := upgradePrivateRootFor(goos)
		if r == "" {
			t.Errorf("upgradePrivateRootFor(%s) = \"\" over grants %v", goos, seatbeltTempGrants(goos, ""))
			continue
		}
		for _, launch := range []string{"", "/tmp", "/tmp/seat", tmpdir, filepath.Join(tmpdir, "deeper"), "/var/folders/ab/cd1234/T"} {
			d := upgradePrivateDenied(goos, launch)
			if !writeGranted(d, absResolve(r)) {
				t.Errorf("the %s staging root %s is covered by no deny rendered for a launch whose TMPDIR was %q (%v) — a seat launched that way writes the tree `make test` then runs out of, uncaged, as the verb's caller (ranger-base-18brx)", goos, r, launch, d)
			}
		}
		// Said once in the production spelling too, so the refusal the verb
		// makes at run time is graded and not only the list it reads.
		if !upgradePrivateRootDenied(goos, r) {
			t.Errorf("upgradePrivateRootDenied(%s, %s) = false, so upgradePrivateCopy would refuse to stage at its own root", goos, r)
		}
	}
	// And the predicate's FALSE side, which is the hole's own shape: a root
	// one level down inside a blanket grant, and a root under a TMPDIR no
	// grant names. A predicate that says yes to these refuses nothing.
	for _, bad := range []string{
		"",
		filepath.Join("/tmp/op", upgradePrivateRootName),
		filepath.Join(tmpdir, "deeper", upgradePrivateRootName),
		filepath.Join("/var/folders/ab/cd1234/T", upgradePrivateRootName),
	} {
		if upgradePrivateRootDenied("linux", bad) {
			t.Errorf("upgradePrivateRootDenied(linux, %q) = true, and no profile rendered on this box denies that name (%v)", bad, upgradePrivateDenied("linux", ""))
		}
	}

	// 2. EVERY blanket temp grant has that name denied, not just this run's.
	// A seat's profile is rendered at the seat's launch, which is before the
	// verb picks anything, so the deny cannot depend on the verb's TMPDIR.
	denied := upgradePrivateDenied(runtime.GOOS, tmpdir)
	if len(denied) != len(grants) {
		t.Fatalf("upgradePrivateDenied = %v over grants %v — one deny per blanket grant, or a grant is left wide open", denied, grants)
	}
	for i, g := range grants {
		if want := filepath.Join(g, upgradePrivateRootName); denied[i] != want {
			t.Errorf("grant %s is narrowed by %s, want %s", g, denied[i], want)
		}
	}

	// 3. A RENDERED carve-out carries them, and the grant it narrows is
	// really there — otherwise the deny is true and vacuous.
	f := weFixtureIn(t, root)
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, f.rt.StateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, f.rt.StateDirs...)
	// The grant this deny narrows is the one the root hangs off — the temp
	// root itself — and not merely this launch's TMPDIR. Both are asked:
	// without the first the deny below is true and vacuous, and the second
	// is the per-launch grant the deny's other members narrow.
	for _, g := range []string{filepath.Dir(absResolve(upgradePrivateRoot())), absResolve(tmpdir)} {
		if !writeGranted(w, g) {
			t.Fatalf("the writable set does not reach %s, so this pin is grading a grant that is not there: %v", g, w)
		}
	}
	if !writeGranted(c.Deny, absResolve(upgradePrivateRoot())) {
		t.Errorf("no deny covers %s, which every seat's blanket temp grant reaches and `posse upgrade` runs `make test` out of (ranger-base-ua625)\n deny: %v", upgradePrivateRoot(), c.Deny)
	}
	if c.FileShaped(absResolve(upgradePrivateRoot())) {
		t.Errorf("the staging root is marked file-shaped; it is a directory the verb writes into, and the bwrap deny arm would render nothing for it")
	}
	// AND AT THE MECHANISM, not only in the carve-out: the deny has to reach
	// the rendered profile, which is the thing a seat actually runs behind.
	// MEASURED 2026-11-15 on this box, linux, TMPDIR unset: the line reads
	// `  (subpath "/tmp/posse-upgrade")` in the profile's deny block.
	path, err := f.a.RenderSeatbelt(f.ag, f.dir, f.rt.StateDirs...)
	if err != nil {
		t.Fatal(err)
	}
	prof, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "(subpath \"" + absResolve(upgradePrivateRoot()) + "\")"; !strings.Contains(string(prof), want) {
		t.Errorf("the rendered profile has no %s — the carve-out says the root is denied and the wall a seat runs behind does not", want)
	}

	// 4. AND THE VERB ACTUALLY STAGES THERE. Without this row the three
	// above are a deny over a name nothing uses: MEASURED by putting
	// upgradePrivateCopy back on os.MkdirTemp("", …), which left rows 1-3
	// green.
	//
	// THE GRANTS MOVE INTO THE FIXTURE FIRST, and the root moves with them
	// because it is derived from them — which is the ranger-base-18brx fix
	// stated as a test seam. Rows 4 and 5 write at the root and row 5 plants
	// a LINK at it, and the production root is the box's own `/tmp`: shared
	// with every other seat, with the other two suite arms, and with a real
	// `posse upgrade`. Before the root was TMPDIR-independent this isolation
	// came from the t.Setenv above; it comes from the stub now, and the
	// agreement rows 1-3 grade are the production function's.
	hadGrants := seatbeltTempGrants
	seatbeltTempGrants = func(string, string) []string { return []string{tmpdir} }
	t.Cleanup(func() { seatbeltTempGrants = hadGrants })
	if got, wantRoot := upgradePrivateRoot(), filepath.Join(tmpdir, upgradePrivateRootName); got != wantRoot {
		t.Fatalf("upgradePrivateRoot() = %s under a grant set of %v, want %s — the root no longer follows the deny set, so rows 4 and 5 would write the box's shared temp root", got, []string{tmpdir}, wantRoot)
	}
	uf := newUpgradeFixture(t)
	copied, done, err := upgradePrivateCopy(upgradePlan{Repo: uf.repo, Tip: uf.c, To: uf.c, From: uf.a})
	if err != nil {
		t.Fatalf("upgradePrivateCopy over a clean fixture: %v", err)
	}
	if !underDir(upgradePrivateRoot(), copied) {
		t.Errorf("the private copy is at %s, outside the denied staging root %s — `make test` runs out of that tree, uncaged, as this verb's caller (ranger-base-ua625)", copied, upgradePrivateRoot())
	}
	done()
	// The root this run made, taken back so row 5 can plant its link at the
	// name. It is empty once `done` has removed the run dir, so a plain
	// Remove is the whole of it — and an error here means the cleanup left
	// something behind, which is worth hearing about.
	if err := os.Remove(upgradePrivateRoot()); err != nil {
		t.Fatalf("the staging root is not empty after done(): %v", err)
	}

	// 5. A link planted at the name is refused and not followed. This is the
	// one shape a fixed name buys that a random one did not have: the verb
	// would otherwise stage — and `make test` would run — wherever the
	// planter pointed.
	elsewhere := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, upgradePrivateRoot()); err != nil {
		t.Fatal(err)
	}
	_, _, err = upgradePrivateCopy(upgradePlan{Repo: root, Tip: "a", To: "a", From: "a"})
	if err == nil || !strings.Contains(err.Error(), upgradePrivateRootName) {
		t.Fatalf("upgradePrivateCopy over a link at %s = %v, want a refusal naming it", upgradePrivateRoot(), err)
	}
	if ents, derr := os.ReadDir(elsewhere); derr != nil || len(ents) != 0 {
		t.Errorf("the refused run wrote into the link's target %s: %v %v", elsewhere, ents, derr)
	}

	// 6. AND THE VERB ASKS, rather than trusting that its root is denied
	// because it derived it from the deny set (ranger-base-18brx). That
	// refusal cannot be reached by any TMPDIR now the root reads no TMPDIR —
	// which is the fix — so it is reached the one way left: with the grant
	// set empty there is no name to stage under that any profile denies, and
	// the verb says so and fetches nothing. Fail-closed, and a row, because
	// a belt no test reaches is a belt the next edit deletes green.
	seatbeltTempGrants = func(string, string) []string { return nil }
	if got := upgradePrivateRoot(); got != "" {
		t.Fatalf("upgradePrivateRoot() = %q over an empty grant set, want \"\" — there is no denied name to stage under", got)
	}
	_, _, err = upgradePrivateCopy(upgradePlan{Repo: root, Tip: "a", To: "a", From: "a"})
	if err == nil {
		t.Fatal("upgradePrivateCopy staged a private copy at a name no rendered profile denies — `make test` runs out of that tree, uncaged, as the verb's caller (ranger-base-18brx)")
	}
	if !strings.Contains(err.Error(), "denies") {
		t.Errorf("the refusal does not say the root is undenied:\n%v", err)
	}
}
