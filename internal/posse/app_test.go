//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearHomeEnv unsets every environment name newApp reads ahead of HOME, so
// a test that means to exercise the HOME fallback actually reaches it.
// newApp (app.go) reads RHQ_HOME first and falls back to POSSE_HOME for the
// one-release both-names window of ranger-base-mlc Q2, and every
// posse-dispatched session exports POSSE_HOME — so a test clearing only
// RHQ_HOME is green on a bare shell, red under dispatch, and asserts nothing
// about the HOME branch either way. That red was filed three times
// independently (ranger-base-kvecg, ranger-base-rajsj, ranger-base-hpocz),
// which is the argument for one list rather than a clear per subtest: when
// the POSSE_HOME fallback read is dropped from newApp, drop its line here
// too; when a further name is added there, add it here once.
func clearHomeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("RHQ_HOME", "")
	t.Setenv("POSSE_HOME", "")
}

func TestNewAppHomeSelection(t *testing.T) {
	for _, tc := range []struct {
		name         string
		preferred    bool
		legacy       bool
		explicit     bool // RHQ_HOME
		posseHome    bool // POSSE_HOME
		bothExplicit bool // both RHQ_HOME and POSSE_HOME, at different paths
		want         string
		notice       bool
	}{
		{name: "fresh install", want: "preferred"},
		{name: "legacy install", legacy: true, want: "legacy", notice: true},
		{name: "both homes", preferred: true, legacy: true, want: "preferred"},
		{name: "RHQ_HOME", preferred: true, legacy: true, explicit: true, want: "explicit"},
		// ranger-base-mlc Q2: POSSE_HOME is a fallback read for the one-release
		// both-names window, and RHQ_HOME wins when both are set — an already
		// exported RHQ_HOME, or an installed hook that only knows the old name,
		// must not be silently overridden by the new one.
		{name: "POSSE_HOME alone", preferred: true, legacy: true, posseHome: true, want: "explicit"},
		{name: "RHQ_HOME wins over POSSE_HOME", preferred: true, legacy: true, bothExplicit: true, want: "explicit"},
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
			clearHomeEnv(t)
			switch {
			case tc.bothExplicit:
				t.Setenv("RHQ_HOME", explicit)
				t.Setenv("POSSE_HOME", filepath.Join(root, "posse-home-loser"))
			case tc.posseHome:
				t.Setenv("RHQ_HOME", "")
				t.Setenv("POSSE_HOME", explicit)
			case tc.explicit:
				t.Setenv("RHQ_HOME", explicit)
			default:
				t.Setenv("RHQ_HOME", "")
			}

			var stderr strings.Builder
			a, err := newApp(&stderr)
			if err != nil {
				t.Fatal(err)
			}
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
				if _, err := newApp(&stderr); err != nil {
					t.Fatal(err)
				}
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
		clearHomeEnv(t)
		if err := os.MkdirAll(filepath.Join(root, ".config"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".config", "rhq"), []byte("not-a-dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stderr strings.Builder
		a, err := newApp(&stderr)
		if err != nil {
			t.Fatal(err)
		}
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
		clearHomeEnv(t)
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
		a, err := newApp(&stderr)
		if err != nil {
			t.Fatal(err)
		}
		if a.Home != legacy {
			t.Fatalf("home = %s, want legacy %s", a.Home, legacy)
		}
		if !strings.Contains(stderr.String(), preferred) || !strings.Contains(stderr.String(), legacy) {
			t.Fatalf("legacy notice %q must name both paths", stderr.String())
		}
	})
}

// ranger-base-33vp: AbbrevHome is what every printed path passes through,
// and cmd/posse's census tests now assert against it — so a degenerate
// AbbrevHome (one that returned "", say) would make those
// strings.Contains checks trivially true and silently stop asserting.
// Pin the four cases here so that assertion rests on something.
func TestAbbrevHome(t *testing.T) {
	home := "/tmp/somehome"
	t.Setenv("HOME", home)
	for _, c := range []struct{ in, want string }{
		{home, "~"},
		{home + "/src/posse", "~/src/posse"},
		{"/var/lib/elsewhere", "/var/lib/elsewhere"},
		// A sibling that merely shares the prefix as a string is NOT under
		// home: the check is home+"/", not home.
		{home + "2/src", home + "2/src"},
	} {
		if got := AbbrevHome(c.in); got != c.want {
			t.Errorf("AbbrevHome(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ranger-base-eku36: the package's $HOME is a tempdir, and on macOS
// os.MkdirTemp hands back /var/folders/… — a symlink to
// /private/var/folders/…. AbbrevHome is a string-PREFIX test, so a path a
// test derives from a real tree (git and filepath.EvalSymlinks both hand
// back the resolved spelling) failed that prefix and printed absolute
// where the operator sees ~/…. 328 production call sites in 54 files render
// a path through AbbrevHome, so on macOS every pin reading one of those
// messages was measuring a shape no operator ever sees; ubuntu-latest,
// where the temp dir is not behind a link, was the sole reader of the real
// spelling and found one the hard way (ranger-base-tiidc, a ~-carrying
// prescription handed to exec.Command). TestMain resolves the temp home,
// and this is the pin that says so: drop the EvalSymlinks call there and
// both halves below go red.
func TestTheTestHomeIsResolvedSoAbbrevHomeCanSeeUnderIt(t *testing.T) {
	t.Parallel()
	home := os.Getenv("HOME")
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", home, err)
	}
	if resolved != home {
		t.Errorf("the test $HOME is unresolved: %q resolves to %q, so AbbrevHome finds no prefix in any real path under it", home, resolved)
	}
	// The property itself and not only its cause: a directory made under
	// the home, spelled the way the filesystem spells it back, abbreviates.
	base, err := os.MkdirTemp(home, "abbrev-probe-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	dir := filepath.Join(base, "worktrees", "sess")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "~/" + filepath.Base(base) + "/worktrees/sess"
	if got := AbbrevHome(real); got != want {
		t.Errorf("AbbrevHome(%q) = %q, want %q — an operator-facing path the suite reads as absolute", real, got, want)
	}
}

// ranger-base-a3t1: with $HOME unset both helpers used to INVENT a home at
// the filesystem root — AbbrevHome("/etc/x") returned "~/etc/x" because
// HasPrefix(p, ""+"/") is true for every absolute path, and
// ExpandTilde("~/x") returned "/x" because ""+"/"+"x" is "/x". The first
// only misprints; the second misresolves, so `posse beads check --dir
// ~/src/posse` under `env -i` would stat /src/posse and report a repo that
// is there as missing. An unknown home abbreviates and expands to nothing:
// both return p unchanged, and "~" stays "~" rather than becoming "".
func TestHomeHelpersInventNoHomeWhenHomeIsUnset(t *testing.T) {
	t.Setenv("HOME", "")
	for _, c := range []struct{ in, want string }{
		{"/etc/x", "/etc/x"},
		{"/", "/"},
		{"~", "~"},
		{"~/src/posse", "~/src/posse"},
		{"relative/x", "relative/x"},
		{"", ""},
	} {
		if got := AbbrevHome(c.in); got != c.want {
			t.Errorf("HOME unset: AbbrevHome(%q) = %q, want %q", c.in, got, c.want)
		}
		if got := ExpandTilde(c.in); got != c.want {
			t.Errorf("HOME unset: ExpandTilde(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
