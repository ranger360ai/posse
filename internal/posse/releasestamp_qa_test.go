//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-ctath — the third state of the wall reading, and
// the three stamps that must never reach it.
//
// THE BEAD. On a release install (`brew install ranger360ai/tap/posse`,
// 0.5.3+482a1256) with no checkout of github.com/ranger360ai/posse, `posse
// gates <persona> | head -1` read "UNKNOWN whether this binary carries the
// wall renders and the launch-time checks that landed … read this as UNKNOWN
// and never as OK" — honestly, and on every release install forever, because
// the freshness reading is defined against a local checkout a release user
// does not have. These arms hold the fix in both directions: the release
// state is REACHED where the bead found silence, and it is REFUSED for every
// build that is not a release.
//
// The wiring is pinned where it can be: a unit arm cannot tell whether
// `posse gates` prints any of this, so cmd/posse/gatesrelease_qa_test.go
// builds a binary stamped through ReleaseTagSymbol and reads the state back
// off the real command — the surface the bead quoted.

import (
	"strings"
	"testing"
)

// releaseStamp is the version string a release install carries: the bead's
// own, which is this source's Version plus the sha the release was cut from.
const releaseStamp = "0.5.3+482a1256"

// unreadLag is the lag reading a release install always gets: a stamp that
// names a commit, and no checkout anywhere that holds it. No git and no
// fixture — the abstention happens before a candidate is ever opened.
func unreadLag(t *testing.T, version string) LauncherLag {
	t.Helper()
	l := FindLauncher(version, nil)
	if l.Known() {
		t.Fatalf("the rig produced a COUNTED lag, so it is not measuring the bead: %+v", l)
	}
	return l
}

// The bead's repro, at the reading. A believed release identity turns the
// permanent UNKNOWN into a sentence that names which release's walls are in
// force — and the word UNKNOWN must be gone from it, because `| head -1` is
// how the operator met this and a reader who sees the word reads the rest as
// the same abstention.
func TestAReleaseInstallWithNoCheckoutReadsTheReleaseAndNotUnknown(t *testing.T) {
	t.Parallel()
	w := WallRenderer{
		Lag:     unreadLag(t, releaseStamp),
		Release: readRelease("v0.5.3", "0.5.3", releaseStamp),
	}
	if !w.Release.IsRelease() {
		t.Fatalf("a clean release stamp agreeing with its source was refused: %+v", w.Release)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	for _, want := range []string{
		"is release v0.5.3",
		"the walls it renders and the launch-time checks it decides are the ones tagged v0.5.3",
		// What a release user can actually do, and what they cannot: the
		// line has to carry the remedy, and has to say out loud that it is
		// not claiming to be the newest release.
		"brew upgrade ranger360ai/tap/posse",
		"never that it is the newest one",
		// The abstention underneath is still true and still said: this is a
		// third state, not a claim that a comparison happened.
		"no local comparison was possible",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the release reading is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "UNKNOWN") {
		t.Errorf("a release install still reads UNKNOWN — this IS the bead:\n%s", got)
	}
	// "current" is the tip arm's word and it is a claim about main. A
	// release may have been superseded an hour after it was cut and this
	// binary takes no network call to find out, so it may not borrow it.
	if strings.Contains(got, "are current") {
		t.Errorf("a release install claimed its walls are current, which it cannot know here:\n%s", got)
	}
	if w.Actionable() {
		t.Error("a believed release identity was reported as something to act on — nothing local can change it, which is what made the UNKNOWN permanent")
	}
}

// The release state must never mask a lag. A release binary on a box that
// DOES hold its commit — a fleet box that installed the tap by mistake
// (ranger-base-39jnl), or a developer running a release — is counted like
// any other binary, because the lag is the richer answer.
func TestTheReleaseStateNeverMasksACountedLag(t *testing.T) {
	t.Parallel()
	repo, stamp := wallRig(t)
	landCommits(t, repo, 3)
	commitIn(t, repo, WallRenderSources[0], "package posse // v2\n", "ranger-base-prjck: a fourth credential read-deny")

	w := ReadWallRenderer(FindLauncher("0.5.3+"+stamp, []string{repo}))
	// The identity is believed, and it still does not win.
	w.Release = readRelease("v0.5.3", "0.5.3", "0.5.3+"+stamp)
	if !w.Release.IsRelease() {
		t.Fatalf("the rig's release identity was refused, so this arm proves nothing: %+v", w.Release)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	for _, want := range []string{"4 commit(s) behind main", "1 of them rendering a wall", "ranger-base-prjck", "make install"} {
		if !strings.Contains(got, want) {
			t.Errorf("a release binary with a real lag did not report it (%q missing):\n%s", want, got)
		}
	}
	if strings.Contains(got, "is release v0.5.3") {
		t.Errorf("the release state masked a counted lag of 4 commits:\n%s", got)
	}
	if !w.Actionable() {
		t.Error("a 4-commit lag reported nothing to act on")
	}
}

// The bead's own condition, three ways: never OK for a dirty or unknown
// build. Each refusal is also SAID — a stamp that was there and could not be
// believed renders beside the UNKNOWN, because a mis-stamped or forged
// release identity that renders as silence looks exactly like the ordinary
// dev build this file does not touch.
func TestAReleaseStampThatCannotBeBelievedIsRefusedAndSaid(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		stamped string
		version string
		vstring string
		said    string
	}{{
		// The Makefile's dev stamp spells a dirty tree
		// `<sha>-dirty-<fingerprint>` (cagestale.go dirtyIdent), not a bare
		// `-dirty` suffix. Both spellings are arms here: a suffix test would
		// pass the first and miss the second, and the second is the one a
		// hand-built binary on this box would actually wear.
		name: "dirty, the Makefile's own spelling", stamped: "v0.5.3", version: "0.5.3",
		vstring: "0.5.3+482a1256-dirty-9f2c1ab4", said: "uncommitted edits",
	}, {
		name: "dirty, the release and build-info spelling", stamped: "v0.5.3", version: "0.5.3",
		vstring: "0.5.3+482a1256-dirty", said: "uncommitted edits",
	}, {
		name: "a tag that disagrees with the compiled-in version", stamped: "v9.9.9", version: "0.5.3",
		vstring: releaseStamp, said: "one of the two is wrong",
	}, {
		name: "a tag on a build that names no commit", stamped: "v0.5.3", version: "0.5.3",
		vstring: "0.5.3+dev", said: "names no commit at all",
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rel := readRelease(c.stamped, c.version, c.vstring)
			if rel.IsRelease() {
				t.Fatalf("%q was read as a release: %+v", c.name, rel)
			}
			if rel.Disagreement == "" {
				t.Fatalf("%q was refused in silence, which reads exactly like an unstamped dev build", c.name)
			}
			if !strings.Contains(rel.Disagreement, c.said) {
				t.Errorf("the refusal does not say why (%q missing): %s", c.said, rel.Disagreement)
			}
			w := WallRenderer{Lag: unreadLag(t, c.vstring), Release: rel}
			got := strings.Join(w.Lines("gates"), "\n")
			if !strings.Contains(got, "UNKNOWN") {
				t.Errorf("an unbelievable release stamp did not read UNKNOWN:\n%s", got)
			}
			if strings.Contains(got, "is release ") {
				t.Errorf("an unbelievable release stamp claimed a release:\n%s", got)
			}
			if !strings.Contains(got, rel.Disagreement) {
				t.Errorf("the reading dropped the disagreement it read:\n%s", got)
			}
			if !w.Actionable() {
				t.Error("a release stamp that could not be believed reported nothing to act on")
			}
		})
	}
}

// An UNSTAMPED build is not a disagreement — it is every build but a
// release's, including every build on this box. It must read exactly what it
// read before this bead: the plain UNKNOWN, one line, with nothing appended.
func TestAnUnstampedBuildIsTheOrdinaryUnknownAndNothingMore(t *testing.T) {
	t.Parallel()
	rel := readRelease("", "0.5.3", "0.5.3+dev")
	if rel.IsRelease() || rel.Disagreement != "" || rel.Stamped != "" {
		t.Fatalf("an unstamped build is neither a release nor a disagreement: %+v", rel)
	}
	// Whitespace only is the same thing: a `-X …ReleaseTag=` with an empty
	// value, which is what a release script bug would leave behind.
	if blank := readRelease("  \n", "0.5.3", releaseStamp); blank.IsRelease() || blank.Disagreement != "" {
		t.Fatalf("a blank stamp is not a release and not a disagreement: %+v", blank)
	}
	w := WallRenderer{Lag: unreadLag(t, "0.5.3+dev"), Release: rel}
	lines := w.Lines("gates")
	if len(lines) != 1 {
		t.Fatalf("the dev UNKNOWN grew a line it has nothing to put in:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[0], "UNKNOWN") || !strings.Contains(lines[0], "never as OK") {
		t.Errorf("the dev build no longer reads UNKNOWN:\n%s", lines[0])
	}
	if !w.Actionable() {
		t.Error("an unreadable dev build reported nothing to act on")
	}
}

// THIS binary — the test binary, and every binary built any way but through
// scripts/release-artifacts.sh — carries no release tag. The default is the
// whole of what keeps the state above out of a dev build's reach, so it is
// pinned as a value and not only as prose.
func TestThisBuildCarriesNoReleaseTag(t *testing.T) {
	t.Parallel()
	if ReleaseTag != "" {
		t.Fatalf("ReleaseTag defaults to %q — a build that is not a release can reach the release state", ReleaseTag)
	}
	if rel := Release(); rel.IsRelease() {
		t.Fatalf("this test binary reads as a published release: %+v", rel)
	}
}

// The tip and behind arms are unchanged by this bead, and the pin says so:
// a release identity is consulted only where the lag abstains, so a counted
// reading must be Actionable on exactly the old rule.
func TestActionableIsTheLagsRuleWhereverTheLagWasRead(t *testing.T) {
	t.Parallel()
	repo, stamp := wallRig(t)

	tip := ReadWallRenderer(FindLauncher("0.5.3+"+stamp, []string{repo}))
	tip.Release = readRelease("v0.5.3", "0.5.3", "0.5.3+"+stamp)
	if tip.Lag.Behind != 0 || !tip.Lag.Known() {
		t.Fatalf("the rig is not at the tip: behind = %d, why = %q", tip.Lag.Behind, tip.Lag.Why)
	}
	if tip.Actionable() {
		t.Error("a binary at the tip reported something to act on")
	}

	landCommits(t, repo, 1)
	behind := ReadWallRenderer(FindLauncher("0.5.3+"+stamp, []string{repo}))
	behind.Release = tip.Release
	if !behind.Actionable() {
		t.Error("a one-commit lag reported nothing to act on, under a release identity that must not silence it")
	}
}
