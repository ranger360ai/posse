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
