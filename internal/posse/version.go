package posse

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// Build is the git SHA (+ "-dirty") stamped by the Makefile via -ldflags.
// "dev" means the binary was built some other way — see VersionString, which
// does not stop there.
var Build = "dev"

// readBuildInfo is runtime/debug.ReadBuildInfo, a var so a test can hand the
// fallback a build info instead of the test binary's own.
var readBuildInfo = debug.ReadBuildInfo

// pseudoVersion matches the module version go synthesises for a commit that
// is not itself a tag — v0.0.0-<14-digit UTC timestamp>-<12-char sha> with
// no tag behind it, v0.3.1-0.<timestamp>-<sha> with one — whose tail is the
// only place a build from the module cache carries its commit.
var pseudoVersion = regexp.MustCompile(`^v[0-9].*[-.][0-9]{14}-([0-9a-f]{12})$`)

// VersionString is what `posse version` and the cockpit header show.
//
// The Makefile's stamp wins when it is there. Nothing else runs those
// ldflags, so `go install ...@latest` and a plain `go build ./cmd/posse`
// used to report the literal "dev" even though go had already written the
// commit into the binary's own build info (ranger-base-bzu). When the stamp
// is missing we read that instead, and only say "dev" when there is nothing
// to read.
func VersionString() string { return versionString(Build, readBuildInfo) }

func versionString(stamped string, read func() (*debug.BuildInfo, bool)) string {
	if stamped != "" && stamped != "dev" {
		return Version + "+" + stamped
	}
	ident, dirty := buildFromInfo(read)
	switch {
	case ident == "":
		return Version + "+dev"
	case ident == "v"+Version && !dirty:
		// Installed from the tag this source is. "0.3.0+v0.3.0" says the
		// same thing twice; the bare version is the honest render. A tag
		// plus edits is not that build, so it does not collapse.
		return Version
	case dirty:
		// Same suffix the Makefile stamps, same meaning: the tree had
		// uncommitted edits, so the ident does not fully name this binary.
		return Version + "+" + ident + "-dirty"
	default:
		return Version + "+" + ident
	}
}

// buildFromInfo names the build out of go's own build info. Since go 1.24
// the main module carries a version even when built from a checkout: an
// exact tag as "vX.Y.Z", anything else as a pseudo-version ending in the
// commit, with "+dirty" appended when the tree had edits. Older toolchains
// leave it "(devel)" and put the commit in vcs.revision, so both are read.
// Empty when neither is there — a binary built with -buildvcs=false outside
// a repository really has nothing to name itself with.
func buildFromInfo(read func() (*debug.BuildInfo, bool)) (ident string, dirty bool) {
	info, ok := read()
	if !ok || info == nil {
		return "", false
	}
	mod, dirty := strings.CutSuffix(info.Main.Version, "+dirty")
	if mod != "" && mod != "(devel)" {
		if m := pseudoVersion.FindStringSubmatch(mod); m != nil {
			return shortSha(m[1]), dirty
		}
		return mod, dirty
	}
	var rev string
	dirty = false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "", false
	}
	return shortSha(rev), dirty
}

// shortSha trims a revision to git's 7-character short form — what the
// Makefile stamps — so one commit reads the same however it was built. The
// pseudo-version carries 12, a checkout build carries 40; a reader comparing
// `posse version` across two boxes should not have to notice which.
func shortSha(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}
