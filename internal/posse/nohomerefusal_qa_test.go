//go:build posse_arm2

package posse

// ranger-base-58b5's fifth site, ruled REFUSE by the operator on
// ranger-base-5flf8: posse's OWN home. newApp's fallback is
// filepath.Join(os.Getenv("HOME"), ".config"), and Join DROPS an empty
// element, so with no $HOME the home was ".config/posse" — relative,
// resolved against whatever cwd the process happened to have, read for
// config and written to for state.
//
// The refusal is last of the three names on purpose: RHQ_HOME and
// POSSE_HOME each say where the home is, and a caller that has answered the
// question must not be refused for the environment it did not need.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAppRefusesAHomeItWouldHaveToInvent(t *testing.T) {
	named := t.TempDir()

	for _, tc := range []struct {
		name             string
		rhq, posse, home string
		wantHome         string // "" = refuse
	}{
		{"no name at all refuses", "", "", "", ""},
		{"RHQ_HOME answers it", named, "", "", named},
		{"POSSE_HOME answers it", "", named, "", named},
		{"HOME answers it", "", "", named, filepath.Join(named, ".config", "posse")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RHQ_HOME", tc.rhq)
			t.Setenv("POSSE_HOME", tc.posse)
			t.Setenv("HOME", tc.home)

			var stderr strings.Builder
			a, err := newApp(&stderr)
			if tc.wantHome == "" {
				if err == nil {
					t.Fatalf("newApp resolved home %q with no RHQ_HOME, no POSSE_HOME and no $HOME — that home is under the cwd", a.Home)
				}
				if a != nil {
					t.Errorf("a refusal must hand back no App, got one rooted at %q", a.Home)
				}
				// The refusal has to name all three names it consulted,
				// so an operator can tell what was checked, AND what they
				// can DO about it — the half a bare "unset" leaves out.
				// Naming only the remedy still reads as a complete
				// sentence, which is how a message that dropped the
				// diagnosis survived this pin's first cut.
				for _, want := range []string{"$HOME", "RHQ_HOME", "POSSE_HOME", ".config/posse"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q must name %s", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("newApp refused a home the environment named: %v", err)
			}
			if a.Home != tc.wantHome {
				t.Errorf("home = %s, want %s", a.Home, tc.wantHome)
			}
		})
	}
}

// The defect itself, from the side that shows what the old code did with
// it: a cwd holding the very directory the empty Join lands in. Pre-fix
// newApp answered ".config/posse" here and every path on the App hung off
// it — so posse read a config nobody installed.
func TestNewAppNoHomeNeverRootsAtTheCWD(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".config", "posse"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".config", "posse", "config.yaml"), []byte("crew: [nobody]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("RHQ_HOME", "")
	t.Setenv("POSSE_HOME", "")
	t.Setenv("HOME", "")

	var stderr strings.Builder
	a, err := newApp(&stderr)
	if err == nil {
		t.Fatalf("newApp took the cwd's .config/posse as posse's home: %q (ConfigPath %q)", a.Home, a.ConfigPath)
	}
	if stderr.String() != "" {
		t.Errorf("a refusal is not a notice: stderr %q", stderr.String())
	}
}
