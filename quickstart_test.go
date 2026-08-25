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
