//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-vkmv3 — ADR 0082 D7: the ONE helper that renders a
// path a refusal prints DEGRADES where canonicalPath refuses, and it degrades
// per half.
//
// WHY A PIN AND NOT JUST A SIGNATURE. D5 made canonicalPath refuse a
// directory that is there and this process may not name it, and D6 made every
// renderer of the WALL propagate that refusal: no second spelling may be
// written where ADR 0023's probe compares two processes' bytes.
// CanonicalAbbrevPath (D4's one helper) is the opposite consumer — a line
// being PRINTED once for an operator to read and paste — and it is the one
// place in the tree where the honest answer is a spelling and not a refusal,
// because both of its call sites are the last thing that happens on their
// path and a remedy with no path in it prescribes nothing. That is a decision
// somebody has to make, so these arms make it: the degrade is REACHED, it is
// not "", and it names the directory.
//
// WHY TWO ARMS. The helper has two callees that can refuse independently, and
// the right degrade is different for each — so a single arm would leave half
// the decision unpinned, and a helper that fell all the way back to
// AbbrevHome(p) whenever EITHER half refused would pass it. The home arm is
// the one that catches that: there the path itself IS nameable, and throwing
// its canonical spelling away to abbreviate against a home nobody can name
// loses a reading that was available.
//
// THE COST, STATED RATHER THAN HIDDEN. A degraded answer may be
// non-canonical, so it may never be stored, compared or rendered into the
// wall. The path arm measures that cost directly: it asks the case-changed
// spelling of the same directory, with reach and without, and the two answers
// differ. That divergence is exactly what D5 refuses for the wall, and these
// arms assert that this helper accepts it — in a string that is printed and
// then dropped.
//
// DARWIN ONLY, and that is the platform half rather than a thin pin: off
// darwin kernelPath answers errNoKernelPath for EVERY path, so canonicalPath
// never reaches the permission branch and there is no refusal to degrade.
//
// MODE 0111 AND NOT CHMOD 0, in the home arm, and it is the whole reason that
// arm can be written at all. F_GETPATH's reading is an open(dir, O_RDONLY),
// which needs READ on the directory; reaching a path BELOW it needs only
// SEARCH. So a directory at 0111 is a directory this process may not name but
// may still reach through — the one configuration where the home half refuses
// while the path half answers. chmod 0 removes both bits, and a path under it
// refuses at the path (the sibling pin's "a path below an unreadable
// component" row), which is this file's first arm and not its second.
//
// MEASURED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
// case-insensitive, by these two arms (`-v` logs each reading):
//
//	home at 0755   kernelPath(home) answers; CanonicalAbbrevPath(repo) -> ~/Repo
//	home at 0111   kernelPath(home) -> EACCES; kernelPath(repo) still answers,
//	               so the degrade is the canonical ABSOLUTE <root>/home/Repo
//	repo chmod 0   kernelPath(repo) -> EACCES; the degrade is AbbrevHome's of
//	               the caller's own spelling, and asked through the
//	               case-changed spelling that is a DIFFERENT string from the
//	               one reach would have rendered
//
// MUTATION-CHECKED 2026-11-08, each mutant run over these two names alone —
// see docs/notes.d/ranger-base-vkmv3.md for the table.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestQAARefusalSpellingDegradesForAPathItCannotName is D7's first half: the
// path has no canonical spelling, so the printed line carries the caller's.
//
// The assertion that matters is the NEGATIVE one — not "" — because "" is
// D5's own answer and the right one everywhere a literal is written. Here it
// would render `add : <private|public>` and tell the operator nothing.
func TestQAARefusalSpellingDegradesForAPathItCannotName(t *testing.T) {
	// Parallel-safe: this arm reads HOME but never writes it.
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skipf("%s has no kernel reading (ADR 0082 D1's non-darwin half IS the fallback), so canonicalPath has no refusal to degrade", runtime.GOOS)
	}
	root := qaCanonicalPath(t, t.TempDir())
	dir := filepath.Join(root, "instance")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cased := filepath.Join(root, "INSTANCE")
	caseFolds := samePath(dir, cased)

	// The reference, with reach, through BOTH spellings: one answer, the
	// kernel's. Without this the arm below cannot tell a degrade from a
	// no-op.
	withReach := CanonicalAbbrevPath(cased)
	if want := CanonicalAbbrevPath(dir); caseFolds && withReach != want {
		t.Fatalf("premise: with reach CanonicalAbbrevPath(%q) = %q and (%q) = %q — one directory, so they must render one string", cased, withReach, dir, want)
	}

	if !qcwNoReach(t, dir) {
		t.Skip("reach could not be withdrawn on this box")
	}

	// The premise of the whole arm: the refusal is actually reached. A box
	// where chmod 0 does not deny the open measures nothing here.
	if _, err := canonicalPath(dir); !errors.Is(err, errPathUnnameable) {
		t.Fatalf("premise: canonicalPath(%q) with no reach did not refuse (%v), so there is no degrade to measure", dir, err)
	}

	// THE DECISION. A refusal has to print something, so the helper answers
	// the caller's spelling — the only spelling of that directory left in
	// existence — rather than D5's loud "".
	got := CanonicalAbbrevPath(dir)
	if got == "" {
		t.Errorf("CanonicalAbbrevPath(%q) = \"\" with no reach — this value is PRINTED as the `beads_visibility:` key a refusal tells the operator to paste (ADR 0082 D4), and `add : <private|public>` prescribes nothing. D5's empty answer is right for a literal that must not be WRITTEN and is the one answer this site cannot give (D7)", dir)
	}
	if want := AbbrevHome(dir); got != want {
		t.Errorf("CanonicalAbbrevPath(%q) = %q with no reach, want AbbrevHome's %q — the degrade is per half, and the half that refused is the path's", dir, got, want)
	}

	// THE COST, measured and not asserted: the degraded answer is NOT
	// canonical, which is why D7 confines it to a string that is printed
	// once. Asked through the case-changed spelling of the same directory,
	// reach and no reach render two strings — exactly what D5 refuses at the
	// wall, accepted here.
	if caseFolds {
		if degraded := CanonicalAbbrevPath(cased); degraded == withReach {
			t.Errorf("premise: the case-changed spelling degraded to %q, the same string reach rendered — so this arm never measured a second spelling at all", degraded)
		} else {
			t.Logf("the cost, measured: CanonicalAbbrevPath(%q) = %q with no reach against %q with it — a second spelling, printed and dropped, never stored or compared (ADR 0082 D7)", cased, degraded, withReach)
		}
	} else {
		t.Logf("case row skipped: %s is on a case-SENSITIVE volume, so the two spellings are two directories", root)
	}
}

// TestQAARefusalSpellingKeepsTheCanonicalPathWhenOnlyHomeCannotBeNamed is
// D7's second half, and the arm that fences out the one-line degrade: when
// the PATH is nameable and only HOME is not, the `~` form is what is lost and
// the canonical absolute spelling is what is kept.
//
// A helper written as `if either half refused { return AbbrevHome(p) }` is
// green on the arm above and red here: it would throw away a kernel reading
// it successfully took, and abbreviate against a home this process cannot
// name — the second spelling one rung in, which is canonicalAbbrevHome's own
// reason for propagating rather than degrading (ADR 0082 D2/D5).
func TestQAARefusalSpellingKeepsTheCanonicalPathWhenOnlyHomeCannotBeNamed(t *testing.T) {
	// NOT parallel: t.Setenv("HOME") is the whole fixture.
	if runtime.GOOS != "darwin" {
		t.Skipf("%s has no kernel reading (ADR 0082 D1's non-darwin half IS the fallback), so canonicalPath has no refusal to degrade", runtime.GOOS)
	}
	root := qaCanonicalPath(t, t.TempDir())
	home := filepath.Join(root, "home")
	repo := filepath.Join(home, "Repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	// The reference, with reach: the repo sits under HOME, so the rendered
	// spelling is the `~` form. This is the reading the degrade loses, and
	// an arm whose fixture rendered no `~` form would measure nothing.
	withReach := CanonicalAbbrevPath(repo)
	if !strings.HasPrefix(withReach, "~/") {
		t.Fatalf("premise: CanonicalAbbrevPath(%q) = %q with HOME=%q, want a `~` form — ADR 0082 D2's abbreviation is what this arm measures the loss of", repo, withReach, home)
	}

	// SEARCH but not READ: the one configuration where the home half refuses
	// and the path half still answers. 0111 rather than chmod 0 is load
	// bearing and the head comment says why.
	if os.Geteuid() == 0 {
		t.Skip("reach cannot be withdrawn from root — mode bits do not bind uid 0")
	}
	if err := os.Chmod(home, 0o111); err != nil {
		t.Skipf("cannot chmod %s: %v", home, err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o755) })

	// Both premises, because the arm is exactly the gap between them.
	if _, err := canonicalAbbrevHome(repo); !errors.Is(err, errPathUnnameable) {
		t.Skipf("HOME at 0111 is still nameable here (canonicalAbbrevHome err %v) — nothing to measure", err)
	}
	canon, err := canonicalPath(repo)
	if err != nil {
		t.Skipf("the path under a 0111 home is not reachable here either (%v), which is the arm above and not this one", err)
	}
	t.Logf("the gap, measured: HOME at 0111 refuses (errPathUnnameable) while canonicalPath(%q) still answers %q", repo, canon)

	// THE DECISION: keep the reading that was taken; lose only the `~`.
	got := CanonicalAbbrevPath(repo)
	if got == "" {
		t.Errorf("CanonicalAbbrevPath(%q) = \"\" when only HOME is unnameable — the path itself has a canonical spelling and this line is printed as a key to paste (ADR 0082 D4/D7)", repo)
	}
	if got != canon {
		t.Errorf("CanonicalAbbrevPath(%q) = %q, want canonicalPath's own %q — only the `~` half refused, so only the `~` half is given up (ADR 0082 D7's per-half degrade)", repo, got, canon)
	}
	// And specifically NOT the caller's spelling abbreviated against an
	// unnameable home, which is what a single-line degrade would answer.
	if fallen := AbbrevHome(repo); got == fallen && fallen != canon {
		t.Errorf("CanonicalAbbrevPath(%q) = %q, which is AbbrevHome against a home this process may not name — the kernel reading of the path was taken and then thrown away", repo, got)
	}
}
