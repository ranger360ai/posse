//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-1fsc9 — upgradeHookWallRepos, the ADR 0052 D1 filter
// on `posse upgrade`'s D4 amendment 1 repo list.
//
// WHY THIS FILE EXISTS. The filter used to be three lines inside cmdUpgrade,
// which no pin in the tree reached in either direction: the verb's reads go
// in through upgradeReads but SweepHookWallIdentity does not, so the loop sat
// behind a real build stamp, a real checkout and a box lock, and d4Fixture.run
// — the one fixture that drives the plan — assigns plan.Repos outright rather
// than deriving it (upgradeverify_test.go). So the mutant `if true` in place
// of `if r.Skip == ""` was GREEN over 77 tests in 2.2s, and a managed hooks
// checkout and an absent one both landed in the operator's `posse gates
// install-hooks` line, which is the one instruction ADR 0052 D1 says this
// verb must never give. The derivation moved to a package function; these are
// the pins that function exists to make possible.
//
// The claim, in one sentence: a repo that was not MEASURED never appears in a
// command prescribing a write there, whichever of the three reasons it was
// not measured — and a repo that was measured always does, in config order.

import (
	"slices"
	"testing"
)

// hookWallSweepOf builds a sweep by hand — which is the whole point of the
// seam this file pins. Each entry is (dir, skip): an empty skip is a repo the
// sweep MEASURED, and a non-empty one is each of the three reasons it did not.
func hookWallSweepOf(t *testing.T, entries ...[2]string) HookWallSweep {
	t.Helper()
	var s HookWallSweep
	for _, e := range entries {
		r := HookWallRepo{Config: e[0], Dir: e[0], Skip: e[1]}
		s.Declared++
		switch {
		case r.Skip == "":
			s.Measured++
		case r.Skip == managedHooksLineForTest:
			r.Managed = true
			s.Managed++
		}
		s.Repos = append(s.Repos, r)
	}
	return s
}

// managedHooksLineForTest is the line the managed arm of the sweep sets Skip
// to, taken from the producer rather than retyped: hookfresh.go's sweep does
// `r.Skip, r.Managed = m.line(), true`, so a pin that invented its own string
// would be pinning a repo state the sweep cannot produce.
var managedHooksLineForTest = managedHooks{
	Dir:     "/employer/hooks",
	Managed: true,
	Owner:   "0",
	Mode:    "0555",
}.line()

// TestUpgradeHookWallReposDropsEveryUnmeasuredRepo is the ADR 0052 D1 claim.
// The managed arm is the one the ADR is about and the other two come with it:
// all three are a repo where nothing of posse's was measured, so none of them
// can be the subject of an install-hooks instruction.
//
// MUTATION-CHECKED (2026-11-14, linux, this worktree): `if true` in place of
// `if r.Skip == ""` — the mutant that was green before this file existed —
// fails here naming all three skipped repos. Dropping the `append` entirely
// fails the config-order pin below.
func TestUpgradeHookWallReposDropsEveryUnmeasuredRepo(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		what string
		skip string
	}{
		{"a managed hooks path — the one ADR 0052 D1 is about", managedHooksLineForTest},
		{"a repo config outlived", "absent — no such directory"},
		{"a directory that is not a checkout", "not a git repository"},
	} {
		t.Run(c.what, func(t *testing.T) {
			t.Parallel()
			s := hookWallSweepOf(t, [2]string{"/src/skipped", c.skip})
			if got := upgradeHookWallRepos(s); len(got) != 0 {
				t.Errorf("upgradeHookWallRepos returned %q for %s — `posse upgrade` would print `posse gates install-hooks %s`, which is the write ADR 0052 D1 refuses", got, c.what, got[0])
			}
		})
	}
}

// TestUpgradeHookWallReposKeepsEveryMeasuredRepoInConfigOrder is the half a
// reader assumes and so the half that can rot silently: the filter drops the
// skipped repos and NOTHING else, and the order it returns is the order the
// sweep reported, which is the order config declares. A filter that kept only
// the first measured repo, or that reversed them, passes the pin above.
func TestUpgradeHookWallReposKeepsEveryMeasuredRepoInConfigOrder(t *testing.T) {
	t.Parallel()
	s := hookWallSweepOf(t,
		[2]string{"/src/alpha", ""},
		[2]string{"/src/managed", managedHooksLineForTest},
		[2]string{"/src/beta", ""},
		[2]string{"/src/gone", "absent — no such directory"},
		[2]string{"/src/gamma", ""},
	)
	want := []string{"/src/alpha", "/src/beta", "/src/gamma"}
	if got := upgradeHookWallRepos(s); !slices.Equal(got, want) {
		t.Errorf("upgradeHookWallRepos = %q, want %q — the measured repos, in config order", got, want)
	}
}

// TestUpgradeHookWallReposIsEmptyForASweepThatMeasuredNothing is the state
// every pin in this package's scratch homes is actually in — a home whose
// config declares no beads_visibility: repo at all — and it is a positive
// control for the two above: it says the empty answer they assert is reachable
// without the filter firing, so neither of them is asserting on a function
// that always returns nothing.
func TestUpgradeHookWallReposIsEmptyForASweepThatMeasuredNothing(t *testing.T) {
	t.Parallel()
	if got := upgradeHookWallRepos(HookWallSweep{}); got != nil {
		t.Errorf("upgradeHookWallRepos(zero sweep) = %q, want nil — a home that declares no repo prints no install-hooks line", got)
	}
}
