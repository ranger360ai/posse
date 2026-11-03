package posse

// What a RELEASE build knows about itself, and why the wall reading could not
// answer without it (ranger-base-ctath, found in the x4g3h shakedown on a
// release-installed box).
//
// THE SYMPTOM. On a box that installed posse from the tap — `brew install
// ranger360ai/tap/posse`, 0.5.3+482a1256 — and holds no checkout of
// github.com/ranger360ai/posse, `posse gates <persona> | head -1` reads
//
//	UNKNOWN whether this binary carries the wall renders and the launch-time
//	checks that landed: no checkout of github.com/ranger360ai/posse here
//	holds 482a1256 (looked in …) … read this as UNKNOWN and never as OK
//
// Every word of that is true, and it is PERMANENT. The freshness reading is
// defined against a LOCAL checkout (launcherlag.go: the stamp picks the repo,
// and a repo that does not hold the stamp cannot say how far behind it is), so
// a release user does not have a stale answer — they have no answer, on every
// install of every release, forever. An abstention that can never resolve is
// an abstention nobody reads, which is the same end state wallrenderer.go's
// header is about by a different route: the surface whose whole job is "what
// is in force here" says nothing an operator can act on.
//
// WHAT A RELEASE BUILD CAN ANSWER WITHOUT A NETWORK CALL. Its own identity. A
// release binary is cut by scripts/release-artifacts.sh from one tagged
// commit, in a throwaway worktree of that commit, and the walls it renders
// and the checks it decides are therefore exactly the ones that tag carries —
// definitionally, with nothing to compare against and nothing to be stale
// relative to. It did not know this before: the release build stamped only
// its SHA (`-X …posse.Build=$sha`), which is byte-identical to what a dev
// `make build` of the same commit stamps, so the binary could not tell a
// reader which of the two it was. Now it carries the tag as well, and that
// stamp is the whole of the distinction — NOTHING else sets it. `make build`,
// `make release`/scripts/clean-build.sh, `go build`, `go install …@latest`
// and every cage build (cagestale.go) leave ReleaseTag empty, so the release
// state below is not reachable from a dev build at all.
//
// WHAT IT STILL DOES NOT ANSWER, said in the line rather than papered over:
// whether a NEWER release has been published. That is a question about
// github.com, not about this binary, and the bead rules out a network call on
// a launch path — `posse gates` is read at every launch and the promote
// epilogue is the operator's hot path. So the release state names which
// release is in force and the remedy that moves it, and never claims to be
// the latest. It is a THIRD state, not a rebranded OK:
//
//	known lag        → the lag reading, unchanged (a release binary on a box
//	                   that DOES hold its commit is counted like any other —
//	                   the richer answer wins, and the release state never
//	                   masks a real lag)
//	release identity → the walls are that tag's, no local comparison possible
//	anything else    → UNKNOWN, exactly as before
//
// AND NEVER FOR A DIRTY OR UNNAMED BUILD, which is the bead's own condition
// and the three refusals below. A tag that disagrees with the compiled-in
// Version, a tree that had uncommitted edits, and a build that names no
// commit each make the stamp unbelievable — and an unbelievable stamp is said
// OUT LOUD beside the UNKNOWN rather than dropped, because a forged or
// mis-stamped release identity that renders as silence is indistinguishable
// from the ordinary dev build this file is careful not to touch.

import "strings"

// ReleaseTag is the release tag this binary was cut from — "v0.5.3" — stamped
// beside Build by scripts/release-artifacts.sh via -ldflags, and by nothing
// else in the tree. Empty for every other build, which is what makes it an
// identity rather than a label.
var ReleaseTag = ""

// ReleaseTagSymbol is the linker symbol ReleaseTag is stamped through,
// spelled once here so a pin can compare the release script's text against
// the GO name instead of against a second copy of the string.
//
// It has to be pinned both ways, and this is the half a grep can hold. `go
// build -ldflags "-X <symbol>=<value>"` on a symbol that does not exist is
// SILENT — no warning, exit 0, the variable keeps its default
// (cagestale_test.go measured this for the Build stamp) — so renaming the
// variable above and leaving the script alone would ship releases that answer
// UNKNOWN again with every test in the tree green. The other half is an
// executable arm: cmd/posse's pin builds a binary stamped through THIS
// constant and reads the state back off `posse gates`, which fails if the
// name is wrong.
const ReleaseTagSymbol = "github.com/ranger360ai/posse/internal/posse.ReleaseTag"

// ReleaseIdentity is one binary's answer to "am I a published release, and
// which one?".
//
// Every field is kept as measured, for LauncherLag's reason: a line has to be
// able to name what it read, and a reader has to be able to tell a build that
// was never stamped from one whose stamp was refused.
type ReleaseIdentity struct {
	// Stamped is the raw ReleaseTag, whatever it says. Empty is the ordinary
	// case — every build but a release's.
	Stamped string
	// Tag is the release this binary IS, and is set only when the stamp was
	// believed. Empty otherwise, including when Stamped is not.
	Tag string
	// Disagreement is why a stamp that was there could not be believed. It
	// is NOT set for an unstamped build: that is not a disagreement, it is a
	// dev build, and treating the two alike is how a mis-stamp goes quiet.
	Disagreement string
}

// IsRelease is whether this binary may be read as a published release.
func (r ReleaseIdentity) IsRelease() bool { return r.Tag != "" }

// Release is the running binary's own reading.
func Release() ReleaseIdentity { return readRelease(ReleaseTag, Version, VersionString()) }

// readRelease decides whether a stamped tag may be read as a release
// identity. version is the compiled-in Version const and versionString is
// VersionString(); both are passed rather than read so one reading carries
// what it was judged against (LauncherLag's rule) and so a pin reaches every
// arm without building a binary.
func readRelease(stamped, version, versionString string) ReleaseIdentity {
	r := ReleaseIdentity{Stamped: strings.TrimSpace(stamped)}
	if r.Stamped == "" {
		return r
	}
	// THE TAG AGAINST THE SOURCE. scripts/release-artifacts.sh already
	// refuses to build when the tag and internal/posse.Version disagree, so
	// this cannot be reached by the release path as it stands — it is here
	// for the build that gets the stamp some other way, where the two
	// numbers are the only cross-check available and believing either would
	// name a release whose code this is not.
	if want := "v" + version; r.Stamped != want {
		r.Disagreement = "this binary is stamped as release " + r.Stamped +
			" and its own compiled-in version is " + version + " (" + want +
			"), so one of the two is wrong and neither is read as a release here"
		return r
	}
	// A DIRTY TREE, the bead's first named condition. `strings.Contains` and
	// not a suffix test, because the Makefile's dev stamp spells it
	// `<sha>-dirty-<content fingerprint>` (cagestale.go dirtyIdent) while a
	// release or go-build-info stamp would spell it `<sha>-dirty` — one check
	// covers both, and a suffix test would miss the longer form entirely.
	if strings.Contains(versionString, "-dirty") {
		r.Disagreement = "this binary is stamped as release " + r.Stamped +
			" and the tree it was built from had uncommitted edits (" + versionString +
			"), so the walls it renders are in no commit of that release"
		return r
	}
	// A BUILD THAT NAMES NO COMMIT, the bead's second. A release is cut from
	// one commit and stamps it; a tag wearing "+dev" is a build that got the
	// tag from somewhere other than the release path.
	if _, _, ok := stampRev(versionString); !ok {
		r.Disagreement = "this binary is stamped as release " + r.Stamped +
			" and names no commit at all (" + versionString +
			"), so there is nothing to say it is that release's code"
		return r
	}
	r.Tag = r.Stamped
	return r
}
