//go:build posse_arm2

package posse

// The BELT behind the commit wall's constitution arm (ranger-base-ak3e):
// the launcher refuses to land a session branch whose merge-base..HEAD diff
// touches the constitution class.
//
// Why a second wall for one rule. The L3 arm keys on RHQ_PERSONA and so
// stands down to `env -i` — the residual PrePushHook has documented for its
// own marker since it was written. This one runs in the LAUNCHER's process:
// operator-side, under the launcher lock, about a branch that is already
// written. A session cannot scrub an environment it is not in. What it costs
// is that it is a report and not a prevention — the commit exists either way
// — and what it buys is that the commit does not become main's without a
// person having read it, which is the whole of ADR 0015 §3.
//
// These pins commit through `commitIn`, the path-limited form, and never
// install the L3 hook: the belt has to hold with the wall absent, which is
// the exact state `env -i` produces.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// constitutionLandTree is a session tree on a repo that is, or is not, the
// constitution repo. The marker is committed on MAIN, not written into the
// worktree, so the class is a property of the repo being landed into.
func constitutionLandTree(t *testing.T, constitution bool) (*App, string, *SessionTree) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	if constitution {
		commitIn(t, repo, ConstitutionRepoMarker+"/keep.md", "the law\n", "seed the constitution")
	}
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	return a, repo, tr
}

// TestQAConstitutionLandRefusesEveryClassMember is the belt's half of the
// bead's DONE WHEN, one subtest per member of the same written-out spec the
// L3 pins use — so the two walls are measured against one list and cannot
// drift into covering different sets.
func TestQAConstitutionLandRefusesEveryClassMember(t *testing.T) {
	t.Parallel()
	for _, member := range constitutionClassSpec {
		t.Run(member, func(t *testing.T) {
			_, repo, tr := constitutionLandTree(t, true)
			rel := member + "/probe.md"
			commitIn(t, tr.Path, rel, "rewritten\n", "s-1: edit the law")

			o, err := MergeSessionWork(tr)
			if err != nil {
				t.Fatal(err)
			}
			if o.Merged {
				t.Fatalf("a branch touching %s must not land: %+v", rel, o)
			}
			// "Nothing was landed" and not "%s still holds every commit"
			// (ranger-base-eq3ba): this string is o.Reason, and a merge-back
			// bead embeds it verbatim for a seat to read later. The claim
			// pinned here is what the launcher DID; that the branch is still
			// there is TestMergeBlockedReasonsNeverPromiseTheBranch's subject
			// and it is asserted nowhere.
			for _, want := range []string{rel, "class: " + member, "ADR 0015 §2/§3", "Nothing was landed and nothing here was changed", "merge --ff-only"} {
				if !strings.Contains(o.Reason, want) {
					t.Errorf("the reason must carry %q, got:\n%s", want, o.Reason)
				}
			}
			// Nothing was landed and nothing was destroyed: the base is
			// where it was and the branch still holds the commit.
			if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); err == nil {
				t.Errorf("%s reached the main checkout — the refusal landed the work it refused", rel)
			}
			if o.Commits != 1 {
				t.Errorf("the branch must still hold its commit, got Commits=%d", o.Commits)
			}
			out, gerr := git(repo, "log", "--format=%s", "-1", tr.Branch)
			if gerr != nil || !strings.Contains(out, "edit the law") {
				t.Errorf("the branch must be untouched, got %q (%v)", out, gerr)
			}
		})
	}
}

// TestQAConstitutionLandPassesOrdinaryWork is the other side of it. The belt
// is a path class, not a freeze on the constitution repo: drafting there is
// ordinary work (ADR 0015 §2) and a launcher that stopped landing it would be
// worked around within the day.
func TestQAConstitutionLandPassesOrdinaryWork(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{
		"docs/notes.d/proposed-settings.json", // the prescribed route has to land
		ConstitutionSourceDir + "/personas/developer/ORDERS.md",
		ConstitutionSourceDir + "/state/gates/refusals.log",
		"scripts/thing.sh",
	} {
		t.Run(rel, func(t *testing.T) {
			_, repo, tr := constitutionLandTree(t, true)
			commitIn(t, tr.Path, rel, "ordinary\n", "s-1: draft")
			o, err := MergeSessionWork(tr)
			if err != nil || !o.Merged {
				t.Fatalf("ordinary work must land: %+v %v", o, err)
			}
			if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
				t.Errorf("%s did not reach the main checkout: %v", rel, err)
			}
		})
	}
}

// TestQAConstitutionLandScopesThePromotedSetToTheConstitutionRepo is the same
// split the L3 arm makes, measured on the land path: the promoted set is only
// law where a constitution lives; the settings file carries the session's own
// deny list everywhere (ranger-base-az93).
func TestQAConstitutionLandScopesThePromotedSetToTheConstitutionRepo(t *testing.T) {
	t.Parallel()
	t.Run("no marker: recipes lands", func(t *testing.T) {
		_, _, tr := constitutionLandTree(t, false)
		commitIn(t, tr.Path, ConstitutionSourceDir+"/recipes/thing.yaml", "not a constitution\n", "s-1: draft")
		o, err := MergeSessionWork(tr)
		if err != nil || !o.Merged {
			t.Fatalf("must land in a repo that is not the constitution: %+v %v", o, err)
		}
	})
	t.Run("no marker: settings.json is still refused", func(t *testing.T) {
		_, _, tr := constitutionLandTree(t, false)
		commitIn(t, tr.Path, ClaudeProjectConfig, "{}\n", "s-1: unfence myself")
		o, err := MergeSessionWork(tr)
		if err != nil {
			t.Fatal(err)
		}
		if o.Merged || !strings.Contains(o.Reason, ClaudeProjectConfig) {
			t.Fatalf("%s must not land in any repo: %+v", ClaudeProjectConfig, o)
		}
	})
}

// TestQAConstitutionLandRefusesARetiredTreesBranch is why the diff is taken
// from workHead rather than from a sha read on the way in: when the worktree
// is gone, workHead falls back to the branch ref, and the branch is then the
// only copy. A belt that could only read a live tree would wave through
// exactly the sessions nobody is watching any more — which is the population
// the landing sweep exists for (landsweep.go).
//
// The neighbouring case, a tree whose HEAD is detached OFF its own branch, is
// refused before this check by notOnBase and for a better reason: no merge
// there can reach the work at all.
func TestQAConstitutionLandRefusesARetiredTreesBranch(t *testing.T) {
	t.Parallel()
	_, repo, tr := constitutionLandTree(t, true)
	rel := ConstitutionRepoMarker + "/developer.md"
	commitIn(t, tr.Path, rel, "rewritten\n", "s-1: edit the law")
	if out, err := git(repo, "worktree", "remove", "--force", tr.Path); err != nil {
		t.Skipf("git worktree remove: %v %s", err, out)
	}
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatalf("a retired tree's branch touching %s must not land: %+v", rel, o)
	}
	if !strings.Contains(o.Reason, rel) {
		t.Errorf("the reason must name %s, got:\n%s", rel, o.Reason)
	}
}

// TestQAConstitutionLandStillReportsWorkAlreadyOnTheBase keeps the belt from
// turning ranger-base-g2xf back on. A branch whose commits the base already
// holds under other shas has nothing left to land, so it must report as
// already-landed and not as a refusal repeated on every pass forever.
func TestQAConstitutionLandStillReportsWorkAlreadyOnTheBase(t *testing.T) {
	t.Parallel()
	_, repo, tr := constitutionLandTree(t, true)
	rel := ConstitutionRepoMarker + "/developer.md"
	commitIn(t, tr.Path, rel, "rewritten\n", "s-1: edit the law")
	sha, err := git(tr.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	// The operator reads it and applies it themselves — the prescribed route
	// — which puts the same patch on the base under a different sha.
	if out, err := git(repo, "cherry-pick", "-x", sha); err != nil {
		t.Skipf("git cherry-pick: %v %s", err, out)
	}
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || len(o.Equivalent) == 0 {
		t.Fatalf("work the base already holds must report as landed, not refused: %+v", o)
	}
}

// TestQAConstitutionClassMatchesExactlyOrAsAPrefix is the matcher itself,
// pinned where both walls read it. The interesting cases are the near
// misses: a sibling whose name merely starts with a class member's is not in
// the class, and a wall that used plain string prefixing would take it.
func TestQAConstitutionClassMatchesExactlyOrAsAPrefix(t *testing.T) {
	t.Parallel()
	cfg := ConstitutionSourceDir + "/config.yaml"
	class := []string{ConstitutionRepoMarker, cfg, ClaudeProjectConfig}
	for _, c := range []struct {
		path string
		want string
	}{
		{ConstitutionRepoMarker, ConstitutionRepoMarker},
		{ConstitutionRepoMarker + "/developer.md", ConstitutionRepoMarker},
		{ConstitutionRepoMarker + "/nested/deep.md", ConstitutionRepoMarker},
		{cfg, cfg},
		{ClaudeProjectConfig, ClaudeProjectConfig},
		{ConstitutionRepoMarker + "uary.md", ""},
		{cfg + ".bak", ""},
		{"docs/" + ConstitutionRepoMarker + "/x.md", ""},
		{ConstitutionSourceDir, ""},
		{"", ""},
	} {
		if got := InConstitutionClass(class, c.path); got != c.want {
			t.Errorf("InConstitutionClass(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestQAConstitutionClassInReadsTheRepo pins the detector both ways round, so
// the belt's class and the hook's are answering the same question about the
// same tree.
func TestQAConstitutionClassInReadsTheRepo(t *testing.T) {
	t.Parallel()
	plain := t.TempDir()
	if got := strings.Join(ConstitutionClassIn(plain), " "); got != ClaudeProjectConfig+" "+ClaudeProjectConfigLocal {
		t.Errorf("a repo with no constitution gets the settings files only, got %q", got)
	}
	con := t.TempDir()
	if err := os.MkdirAll(filepath.Join(con, ConstitutionRepoMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	got := ConstitutionClassIn(con)
	for _, want := range constitutionClassSpec {
		if InConstitutionClass(got, want) != want {
			t.Errorf("the constitution repo's class is missing %q: %v", want, got)
		}
	}
	// A FILE at the marker path is not a constitution: the marker is the PID
	// directory, and a repo that happens to hold a file of that name has not
	// become the fleet's law.
	file := t.TempDir()
	if err := os.MkdirAll(filepath.Join(file, ConstitutionSourceDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(file, filepath.FromSlash(ConstitutionRepoMarker)), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(ConstitutionClassIn(file)) != 2 {
		t.Errorf("a file at the marker path must not make a repo the constitution: %v", ConstitutionClassIn(file))
	}
}

// TestQAConstitutionLandPrescribesPromoteOnlyForPromotedPaths is a small
// precision the refusal has to keep. `posse promote` is what puts constitution
// prose in force; it does nothing whatever about `.claude/settings.json`,
// which is in the class in every repo. A refusal that prescribed it there
// would send the operator to run a command that does not touch what they just
// read — the near-right instruction that teaches people to skim refusals.
func TestQAConstitutionLandPrescribesPromoteOnlyForPromotedPaths(t *testing.T) {
	t.Parallel()
	t.Run("a promoted path names promote", func(t *testing.T) {
		_, _, tr := constitutionLandTree(t, true)
		commitIn(t, tr.Path, ConstitutionRepoMarker+"/developer.md", "rewritten\n", "s-1: edit the law")
		o, err := MergeSessionWork(tr)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(o.Reason, "posse promote") {
			t.Errorf("a promoted path must be told how it is put in force, got:\n%s", o.Reason)
		}
	})
	t.Run("settings.json does not", func(t *testing.T) {
		_, _, tr := constitutionLandTree(t, false)
		commitIn(t, tr.Path, ClaudeProjectConfig, "{}\n", "s-1: unfence myself")
		o, err := MergeSessionWork(tr)
		if err != nil {
			t.Fatal(err)
		}
		if o.Merged {
			t.Fatalf("%s must not land: %+v", ClaudeProjectConfig, o)
		}
		if strings.Contains(o.Reason, "posse promote") {
			t.Errorf("no promote puts a settings file in force; the refusal must not prescribe one:\n%s", o.Reason)
		}
	})
}
