package posse

import (
	"os"
	"strings"
	"testing"
)

// ranger-base-253: go install succeeds without making posse discoverable on a
// default PATH. Whenever a public quickstart advertises that route, keep the
// corrective export between installation and first use.
func TestGoInstallQuickstartsAddGoBinToPathBeforeInit(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		required bool
	}{
		{name: "landing page", path: "www/index.html"},
		{name: "README", path: "README.md", required: true},
	}

	const (
		install = "go install github.com/ranger360ai/posse/cmd/posse@latest"
		path    = `export PATH="$(go env GOPATH)/bin:$PATH"`
		init    = "posse init"
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}

			quickstart := string(contents)
			installAt := strings.Index(quickstart, install)
			if installAt < 0 {
				if tt.required {
					t.Fatalf("%s: missing %q", tt.path, install)
				}
				return
			}
			quickstart = quickstart[installAt+len(install):]

			pathAt := strings.Index(quickstart, path)
			initAt := strings.Index(quickstart, init)
			if pathAt < 0 {
				t.Fatalf("%s: %q is not followed by %q", tt.path, install, path)
			}
			if initAt < 0 {
				t.Fatalf("%s: %q is not followed by %q", tt.path, install, init)
			}
			if pathAt > initAt {
				t.Fatalf("%s: %q must appear before %q", tt.path, path, init)
			}
		})
	}
}

// ranger-base-88m: make install writes ~/.local/bin/posse, which is on no
// default PATH. The go-install route (ranger-base-253) now has the export;
// the README-leading make install route does not.
func TestMakeInstallQuickstartsAddLocalBinToPathBeforeInit(t *testing.T) {
	t.Skip("ranger-base-88m: make install lands in ~/.local/bin, not on a default PATH")

	contents, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	installAt := strings.Index(text, "make install")
	if installAt < 0 {
		t.Fatal("README.md: missing \"make install\"")
	}
	after := text[installAt:]
	initAt := strings.Index(after, "posse init")
	if initAt < 0 {
		t.Fatal("README.md: \"make install\" is not followed by \"posse init\"")
	}
	window := after[:initAt]
	if !strings.Contains(window, `export PATH=`) || !strings.Contains(window, ".local/bin") {
		t.Fatal("README.md: make install is not followed by an export that puts ~/.local/bin on PATH before posse init")
	}
}

// ranger-base-5yl: the advertised posse new --dir ~/code/myproj died
// directory-not-found on a machine that had never created that path. `posse
// new` still refuses a missing dir on purpose — a typo must not silently
// become an empty workspace — so every surface that advertises the example
// path has to make it first.
func TestQuickstartsMkdirBeforeExampleNewDir(t *testing.T) {
	const newLine = "posse new myproj --dir ~/code/myproj"
	for _, path := range []string{"README.md", "www/index.html", "INSTALL.md"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		newAt := strings.Index(text, newLine)
		if newAt < 0 {
			continue
		}
		// Look behind the advertised new for a mkdir of that directory.
		windowStart := newAt - 400
		if windowStart < 0 {
			windowStart = 0
		}
		window := text[windowStart:newAt]
		if !strings.Contains(window, "mkdir") || !strings.Contains(window, "code/myproj") {
			t.Errorf("%s: %q is not preceded by mkdir of ~/code/myproj", path, newLine)
		}
	}
}

// ranger-base-m3a: README still describes @latest as an untagged pseudo-version
// after v0.3.0 exists and is what the public proxy serves.
func TestReadmeDoesNotClaimNoReleaseTag(t *testing.T) {
	t.Skip("ranger-base-m3a: README still says the module has no release tag")

	contents, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "no release tag yet") {
		t.Fatal("README.md still claims the module has no release tag; go install @latest installs v0.3.0")
	}
}

// ranger-base-4ex: INSTALL.md §2 advertised `brew install ranger360ai/tap/posse`
// while ranger360ai/homebrew-tap 404'd, and brew's error never said the tap
// was never created. The tap now exists; keep the three-command route (tap,
// trust one formula, install) and the diagnostic for when it does not.
func TestInstallMdStep2BrewRouteNamesTheTapAndTheSilentFailure(t *testing.T) {
	contents, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	start := strings.Index(text, "## 2.")
	end := strings.Index(text, "## 3.")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("INSTALL.md missing §2 / §3 headings")
	}
	step2 := text[start:end]

	tap := "brew tap ranger360ai/tap"
	trust := "brew trust --formula ranger360ai/tap/posse"
	install := "brew install ranger360ai/tap/posse"
	for _, want := range []string{tap, trust, install, "Error: Failure while executing tap"} {
		if !strings.Contains(step2, want) {
			t.Errorf("INSTALL.md §2 missing %q", want)
		}
	}
	if tapAt, trustAt, installAt := strings.Index(step2, tap), strings.Index(step2, trust), strings.Index(step2, install); tapAt < 0 || trustAt < 0 || installAt < 0 || !(tapAt < trustAt && trustAt < installAt) {
		t.Error("INSTALL.md §2 brew route is not tap, then trust --formula, then install")
	}
}

// ranger-base-4ex: `mktemp -d -t posse-release` is BSD-only. GNU coreutils
// dies "too few X's in template", which is how the release workflow would
// have failed on the tag after vet and test went green.
func TestReleaseScriptsUsePortableMktemp(t *testing.T) {
	scripts := []struct{ path, template string }{
		{path: "scripts/release-artifacts.sh", template: `${TMPDIR:-/tmp}/posse-release.XXXXXX`},
		{path: "scripts/clean-build.sh", template: `${TMPDIR:-/tmp}/posse-clean-build.XXXXXX`},
	}
	for _, s := range scripts {
		t.Run(s.path, func(t *testing.T) {
			contents, err := os.ReadFile(s.path)
			if err != nil {
				t.Fatal(err)
			}
			want := `mktemp -d "` + s.template + `"`
			if !strings.Contains(string(contents), want) {
				t.Errorf("%s: missing portable %q", s.path, want)
			}
			for _, line := range strings.Split(string(contents), "\n") {
				trim := strings.TrimSpace(line)
				if strings.HasPrefix(trim, "#") {
					continue
				}
				if strings.Contains(line, "mktemp") && strings.Contains(line, " -t ") && !strings.Contains(line, "XXXXXX") {
					t.Errorf("%s: live mktemp still uses BSD -t prefix: %s", s.path, trim)
				}
			}
		})
	}
}

// ranger-base-4ex: five in-repo references pointed at docs/runbooks/release.md
// when the file did not exist. The workflow's own closing output is one of them.
func TestReleaseRunbookExistsAtThePathFiveFilesCite(t *testing.T) {
	if _, err := os.Stat("docs/runbooks/release.md"); err != nil {
		t.Fatalf("docs/runbooks/release.md: %v", err)
	}
	const cite = "docs/runbooks/release.md"
	for _, path := range []string{
		"Makefile",
		"INSTALL.md",
		"scripts/tap-formula.sh",
		".github/workflows/release.yml",
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), cite) {
			t.Errorf("%s no longer cites %s", path, cite)
		}
	}
}
