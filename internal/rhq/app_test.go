package rhq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAppHomeSelection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		preferred bool
		legacy    bool
		explicit  bool
		want      string
		notice    bool
	}{
		{name: "fresh install", want: "preferred"},
		{name: "legacy install", legacy: true, want: "legacy", notice: true},
		{name: "both homes", preferred: true, legacy: true, want: "preferred"},
		{name: "RHQ_HOME", preferred: true, legacy: true, explicit: true, want: "explicit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			preferred := filepath.Join(root, ".config", "posse")
			legacy := filepath.Join(root, ".config", "rhq")
			explicit := filepath.Join(root, "chosen")
			for path, create := range map[string]bool{preferred: tc.preferred, legacy: tc.legacy} {
				if create {
					if err := os.MkdirAll(path, 0o755); err != nil {
						t.Fatal(err)
					}
				}
			}
			t.Setenv("HOME", root)
			if tc.explicit {
				t.Setenv("RHQ_HOME", explicit)
			} else {
				t.Setenv("RHQ_HOME", "")
			}

			var stderr strings.Builder
			a := newApp(&stderr)
			want := map[string]string{
				"preferred": preferred,
				"legacy":    legacy,
				"explicit":  explicit,
			}[tc.want]
			if a.Home != want {
				t.Fatalf("NewApp home = %s, want %s", a.Home, want)
			}
			gotNotice := stderr.String()
			if tc.notice {
				if !strings.Contains(gotNotice, preferred) || !strings.Contains(gotNotice, legacy) {
					t.Fatalf("legacy notice %q must name preferred %s and legacy %s", gotNotice, preferred, legacy)
				}
				newApp(&stderr)
				if stderr.String() != gotNotice {
					t.Errorf("legacy notice repeated: %q", stderr.String())
				}
			} else if gotNotice != "" {
				t.Errorf("unexpected home notice: %q", gotNotice)
			}
		})
	}
}

func TestNewAppHomeSelectionEdges(t *testing.T) {
	t.Run("legacy file is not a home", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("HOME", root)
		t.Setenv("RHQ_HOME", "")
		if err := os.MkdirAll(filepath.Join(root, ".config"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".config", "rhq"), []byte("not-a-dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stderr strings.Builder
		a := newApp(&stderr)
		want := filepath.Join(root, ".config", "posse")
		if a.Home != want {
			t.Fatalf("home = %s, want preferred %s", a.Home, want)
		}
		if stderr.String() != "" {
			t.Errorf("unexpected notice: %q", stderr.String())
		}
	})
	t.Run("dangling preferred symlink falls back", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("HOME", root)
		t.Setenv("RHQ_HOME", "")
		preferred := filepath.Join(root, ".config", "posse")
		legacy := filepath.Join(root, ".config", "rhq")
		if err := os.MkdirAll(filepath.Join(root, ".config"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, ".config", "no-such-posse"), preferred); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(legacy, 0o755); err != nil {
			t.Fatal(err)
		}
		var stderr strings.Builder
		a := newApp(&stderr)
		if a.Home != legacy {
			t.Fatalf("home = %s, want legacy %s", a.Home, legacy)
		}
		if !strings.Contains(stderr.String(), preferred) || !strings.Contains(stderr.String(), legacy) {
			t.Fatalf("legacy notice %q must name both paths", stderr.String())
		}
	})
}
