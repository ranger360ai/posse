package posse

import (
	"runtime/debug"
	"testing"
)

func buildInfo(mod string, settings ...string) func() (*debug.BuildInfo, bool) {
	info := &debug.BuildInfo{}
	info.Main.Version = mod
	for i := 0; i+1 < len(settings); i += 2 {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: settings[i], Value: settings[i+1]})
	}
	return func() (*debug.BuildInfo, bool) { return info, true }
}

// ranger-base-bzu: every binary the Makefile did not build reported the
// literal "dev", even with its commit sitting in its own build info.
func TestVersionStringNamesTheBuildWithoutTheLdflag(t *testing.T) {
	noInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	// The table spells every expectation from Version, so a release bump does
	// not red it (ranger-base-qlrx). The one thing that must NOT track it is
	// otherTag: "go install at some other tag" is the arm that proves the
	// collapse is conditional, and if a bump ever made it equal to Version
	// that arm would quietly become a duplicate of the one above it.
	const otherTag = "v0.2.9"
	if otherTag == "v"+Version {
		t.Fatalf("otherTag %q is this source's own tag — the non-collapse arms measure nothing; pick another", otherTag)
	}

	for _, c := range []struct {
		name    string
		stamped string
		read    func() (*debug.BuildInfo, bool)
		want    string
	}{
		{"makefile stamp wins over build info", "83c3c10",
			buildInfo("v0.0.0-20260824022158-2d454c50b4bf"), Version + "+83c3c10"},
		{"makefile stamp of a dirty tree", "83c3c10-dirty", noInfo, Version + "+83c3c10-dirty"},
		{"go install at this source's tag", "dev",
			buildInfo("v" + Version), Version},
		{"go install at some other tag", "dev",
			buildInfo(otherTag), Version + "+" + otherTag},
		{"go install at an untagged commit", "dev",
			buildInfo("v0.0.0-20260824022158-2d454c50b4bf"), Version + "+2d454c5"},
		{"go install at a commit after a tag", "dev",
			buildInfo("v0.3.1-0.20260828050050-3e1eaa3cad35"), Version + "+3e1eaa3"},
		{"go build from a checkout, go 1.24+ module version", "dev",
			buildInfo("v0.0.0-20260828050050-3e1eaa3cad35", "vcs.revision", "3e1eaa3cad35deadbeef"), Version + "+3e1eaa3"},
		{"go build from an edited checkout, go 1.24+", "dev",
			buildInfo("v0.0.0-20260828050050-3e1eaa3cad35+dirty"), Version + "+3e1eaa3-dirty"},
		{"the release tag with edits does not collapse", "dev",
			buildInfo("v" + Version + "+dirty"), Version + "+v" + Version + "-dirty"},
		{"go build from a checkout, pre-1.24 toolchain", "dev",
			buildInfo("(devel)", "vcs", "git", "vcs.revision", "83c3c10ee3f0d0e0f1c2b3a49586d7e8f9a0b1c2"),
			Version + "+83c3c10"},
		{"go build from a dirty checkout, pre-1.24 toolchain", "dev",
			buildInfo("(devel)", "vcs.revision", "83c3c10ee3f0d0e0f1c2b3a49586d7e8f9a0b1c2", "vcs.modified", "true"),
			Version + "+83c3c10-dirty"},
		{"go build with -buildvcs=false", "dev", buildInfo("(devel)"), Version + "+dev"},
		{"no build info at all", "dev", noInfo, Version + "+dev"},
		{"empty ldflag reads as no stamp", "", buildInfo(otherTag), Version + "+" + otherTag},
	} {
		if got := versionString(c.stamped, c.read); got != c.want {
			t.Errorf("%s: versionString(%q) = %q, want %q", c.name, c.stamped, got, c.want)
		}
	}
}

// The exported entry point must be wired to the same fallback — a helper
// nobody calls fixes nothing.
func TestVersionStringUsesTheRealBuildInfo(t *testing.T) {
	defer func(b string, r func() (*debug.BuildInfo, bool)) {
		Build, readBuildInfo = b, r
	}(Build, readBuildInfo)

	Build = "dev"
	readBuildInfo = buildInfo("v0.0.0-20260824022158-2d454c50b4bf")
	if got, want := VersionString(), Version+"+2d454c5"; got != want {
		t.Errorf("VersionString() = %q, want %q", got, want)
	}
	Build = "83c3c10"
	if got, want := VersionString(), Version+"+83c3c10"; got != want {
		t.Errorf("stamped VersionString() = %q, want %q", got, want)
	}
}
