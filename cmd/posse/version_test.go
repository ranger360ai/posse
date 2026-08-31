package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// ranger-base-bzu: a binary the Makefile did not build must still name its
// commit. `posse version` read one source of truth — the -ldflags stamp — so
// every other build called itself "dev" with its sha sitting in its own
// build info.
//
// The fixture is a throwaway git repo holding a copy of this working tree,
// not the checkout the suite runs in, for two reasons: go stamps vcs info
// only when it finds a `.git` DIRECTORY (vcs.go, `{filename: ".git",
// isDir: true}`), and every persona runs this suite from a linked worktree
// where `.git` is a file — measured go1.26.5, and silent: even
// -buildvcs=true stamps nothing and exits 0. And a repo of our own has a
// commit we chose, so the assertion is exact rather than "whatever HEAD is".
func TestVersionNamesTheCommitWithoutTheLdflag(t *testing.T) {
	repo, sha, git := tempRepoOfWorkingTree(t)
	short := sha[:7]
	bin := filepath.Join(t.TempDir(), "posse")
	build := func() string {
		t.Helper()
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/posse")
		cmd.Dir = repo
		if out, err := cmd.Output(); err != nil {
			t.Fatalf("go build: %v\n%s", err, out)
		}
		return posseVersion(t, bin)
	}
	scratch := filepath.Join(repo, "cmd", "posse", "scratch.go")

	if got, want := build(), "posse "+posse.Version+"+"+short+" (herdr-native)"; got != want {
		t.Errorf("a plain `go build` reports %q, want %q", got, want)
	}

	// The Makefile's -dirty means "the sha does not fully name this
	// binary"; go's +dirty means the same thing, so it must read the same
	// way.
	if err := os.WriteFile(scratch, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := build(), "posse "+posse.Version+"+"+short+"-dirty (herdr-native)"; got != want {
		t.Errorf("a build of an edited tree reports %q, want %q", got, want)
	}

	// qa's case on this bead: an exact tag, sitting in the binary's own
	// build info, and the version line still said "dev". Rendered as the
	// bare version — "0.4.0+v0.4.0" says the same thing twice. The tag is
	// spelled from posse.Version, not a literal: this stanza is ABOUT the two
	// agreeing, so a release bump must not be able to leave it asserting the
	// collapse against last release's tag (ranger-base-qlrx).
	if err := os.Remove(scratch); err != nil {
		t.Fatal(err)
	}
	git("tag", "v"+posse.Version)
	if got, want := build(), "posse "+posse.Version+" (herdr-native)"; got != want {
		t.Errorf("a build at the release tag reports %q, want %q", got, want)
	}
}

// ranger-base-qyws: `make build`'s own dirty stamp must match
// SourceBuildStamp's content fingerprint for the same tree, not the bare
// "-dirty" bit it used to compose by hand (GIT_DIRTY) — see cmd/buildstamp,
// which the Makefile now shells out to instead of recomposing the
// fingerprint in make/shell. A mismatch here is exactly the false STALE
// ranger-base-qyws exists to prevent: CageAgeVsPosse compares a `posse cage
// build` image's fingerprinted stamp against a `make build` posse's
// VersionString(), and the two must agree on a byte-identical dirty tree.
func TestMakeBuildStampMatchesSourceBuildStamp(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("no make: nothing can drive the Makefile's own stamp")
	}
	repo, _, _ := tempRepoOfWorkingTree(t)
	// An uncommitted edit exercises the fingerprint half, not just the sha
	// — the bare bit and the fingerprint agree on a clean tree either way.
	if err := os.WriteFile(filepath.Join(repo, "cmd", "posse", "scratch.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("make", "build")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make build: %v\n%s", err, out)
	}

	got := posseVersion(t, filepath.Join(repo, "bin", "posse-go"))
	want := "posse " + posse.SourceBuildVersion(repo) + " (herdr-native)"
	if got != want {
		t.Errorf("`make build` reports %q, SourceBuildStamp of the same tree says %q", got, want)
	}
}

func posseVersion(t *testing.T, bin string) string {
	t.Helper()
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("posse version: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// tempRepoOfWorkingTree copies the module into a fresh repo and commits it,
// returning the directory and the commit. The commit is written with
// plumbing because `git commit` is denied to fleet personas
// (.claude/settings.json), and a fixture no persona can build is a test the
// fleet never runs.
func tempRepoOfWorkingTree(t *testing.T) (dir, sha string, git func(...string) string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git: nothing can stamp a revision")
	}
	dir = t.TempDir()
	copyTree(t, filepath.Join("..", ".."), dir)

	git = func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=posse", "GIT_AUTHOR_EMAIL=posse@invalid",
			"GIT_COMMITTER_NAME=posse", "GIT_COMMITTER_EMAIL=posse@invalid")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", ".")
	git("add", "-A")
	sha = git("commit-tree", git("write-tree"), "-m", "fixture")
	git("update-ref", "HEAD", sha)
	return dir, sha, git
}

// copyTree copies src into dst, skipping .git — the fixture gets a repo of
// its own — and anything unreadable, which a shared checkout can hold.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == src {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer in.Close()
		out, err := os.Create(filepath.Join(dst, rel))
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatalf("copy working tree: %v", err)
	}
}
