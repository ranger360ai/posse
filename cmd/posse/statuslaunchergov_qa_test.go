package main

// ranger-base-y13h7 — the bead's symptom is a `posse status` VIEW, so one pin
// reads it where the operator met it: the lag sentence and the condition in
// the same output, and no all-clear under them.
//
// Everything about the threshold, the key and the class is pinned inside the
// package (internal/posse/launcherlaggov_qa_test.go). What no test in the tree
// could reach before this file is the wiring: `posse status` prints the lag
// LINE from its own reading (with the cwd candidate) and ShopCheck raises the
// ROW from the instance's (without it), and the two have to land in one view.
//
// IT STAMPS THE BINARY IT BUILDS. The reading keys off VersionString(), whose
// stamp for a plain `go build` is this worktree's own HEAD — counted against
// the operator's live `main`, which is red per HOUR rather than per commit
// (ranger-base-rp2y). `-X …/internal/posse.Build=<planted sha>` is the seam
// the Makefile already uses, and it points the whole reading at a scratch
// repo: the planted checkout is the first `beads:` entry and holds the stamp,
// so FindLauncher resolves there and never reaches the cwd candidate at all.

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// lagRig is a scratch posse checkout on `main`, behind by n commits, plus the
// short sha a binary built there would be stamped with.
func lagRig(t *testing.T, n int) (repo, stamp string) {
	t.Helper()
	repo = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	// The module declaration is half of what makes this checkout "this
	// binary's" — FindLauncher counts nowhere else (launcherlag.go
	// isPosseCheckout), which is also why the planted repo cannot be
	// mistaken for the operator's.
	if err := os.WriteFile(filepath.Join(repo, "go.mod"),
		[]byte("module github.com/ranger360ai/posse\n\ngo 1.26.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte(repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "go.mod", "seed.txt")
	// Path-limited, because the gates dir is on this suite's PATH and the
	// git shim there refuses an unqualified commit whatever tree it is in
	// (`deny: Bash(git commit unless --)`). The form is correct anyway.
	git("commit", "-q", "-m", "seed", "--", "go.mod", "seed.txt")
	stamp = git("rev-parse", "--short=7", "HEAD")
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(repo, "fix.txt"),
			[]byte(strconv.Itoa(i)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "--", "fix.txt")
		git("commit", "-q", "-m", "a fix nobody is getting", "--", "fix.txt")
	}
	return repo, stamp
}

// buildStampedRhq builds the command with its version stamp set, the way the
// Makefile's install does.
func buildStampedRhq(t *testing.T, stamp string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build",
		"-ldflags", "-X github.com/ranger360ai/posse/internal/posse.Build="+stamp,
		"-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// The view the bead quoted, both halves of it. The control arm is a launcher
// one commit short of the threshold: it prints the same lag sentence, and
// `nothing needs a human` under it is CORRECT there — which is what makes the
// deep arm a bug rather than a preference.
func TestStatusCarriesTheLauncherLagAsACondition(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, c := range []struct {
		name   string
		behind int
		cond   bool
	}{
		{"one short of the threshold is still just a reading", 15, false},
		{"past it, the same reading needs a human", 16, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, stamp := lagRig(t, c.behind)
			bin := buildStampedRhq(t, stamp)
			home := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, "config.yaml"),
				[]byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "status")
			cmd.Env = statusEnv(t, home)
			out, err := cmd.CombinedOutput()
			got := string(out)

			// The LINE, in every case: an operator who typed a command is
			// owed the number whether or not it is a condition.
			want := strconv.Itoa(c.behind) + " commit(s) behind main"
			if !strings.Contains(got, want) {
				t.Fatalf("the status view does not carry %q — the rig is not pointing at the planted repo:\n%s", want, got)
			}
			if !c.cond {
				if err != nil {
					t.Errorf("a reading that is not a condition must not move the exit code: %v\n%s", err, got)
				}
				if !strings.Contains(got, "nothing needs a human") {
					t.Errorf("want the all-clear beneath a lag inside the threshold:\n%s", got)
				}
				return
			}
			if err == nil {
				t.Errorf("a condition must exit non-zero:\n%s", got)
			}
			if !strings.Contains(got, "G11") || !strings.Contains(got, "LANE") {
				t.Errorf("want the G11 LANE row:\n%s", got)
			}
			if strings.Contains(got, "nothing needs a human") {
				t.Errorf("'only installing closes it' and 'nothing needs a human' in the same view — this IS the bead:\n%s", got)
			}
		})
	}
}

// The cockpit's half of "the status view and the cockpit both report it"
// (ranger-base-y13h7's done-when). The block itself is generic over the set —
// it draws whatever ShopCheck returned, which cockpit_test.go's GOVERNANCE
// arms already pin — so the only thing left to hold is that this screen does
// not stand the reading down, and it holds it by composition: the cockpit
// hands in nothing, and a nil seam is the instance's own reading
// (internal/posse TestShopCheckDefaultsToThisInstancesOwnLauncherReading).
//
// It is an "is nil" assertion on purpose, and the regression it exists for is
// the one shape that would read as wiring: a seam filled with a ZERO
// LauncherLag. Known() is true over an empty Why, Behind is 0, and the row
// would be silent on a launcher 101 commits behind with every surface looking
// correctly plumbed.
func TestTheCockpitDoesNotStandTheLauncherReadingDown(t *testing.T) {
	t.Setenv("RHQ_HOME", filepath.Join(t.TempDir(), "posse"))
	a, err := posse.NewApp()
	if err != nil {
		t.Fatal(err)
	}
	if in := newCockpit(a, nil, io.Discard).govInputs(); in.Lag != nil {
		t.Error("the cockpit hands the shop check a launcher reading of its own — G11 then reports whatever that seam says, and a zero LauncherLag is a silent all-clear")
	}
}
