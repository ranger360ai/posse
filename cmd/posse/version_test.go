package main

import (
	"errors"
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
// returning the directory and the commit.
//
// The commit is PATH-QUALIFIED (`git commit … -- .`), and not the
// `write-tree`/`commit-tree`/`update-ref` plumbing it used to be
// (ranger-base-ltk6x). The reason the plumbing was here still holds — a
// fixture no persona can build is a test the fleet never runs — but the
// spelling was one wall out of date in both directions:
//
//   - the PID rule is `Bash(git commit unless --)`, which denies the
//     UNQUALIFIED form only, so a path-qualified commit was never denied;
//   - and since ranger-base-crlty the L1 git shim REFUSES `commit-tree`
//     outright (hookskip:plumbing-commit, ADR 0052), because a commit
//     object written by plumbing is one no pre-commit or commit-msg hook
//     ever saw. So the plumbing was the spelling no persona could build:
//     both tests in this file were red at arm 1 of `make test` for every
//     gated seat, which also kept arms 2 and 3 (separate recipe lines) from
//     running at all.
//
// Why internal/posse's identical commit-tree fixtures are green and this
// one was not: posse's own `git` helper resolves git OUTSIDE the gates dirs
// (ADR 0077 D3, protocolCmd) and never meets the shim. This helper is a
// bare exec.Command, so it resolves git from the test process's PATH, which
// on a gated seat leads with the seat's shim. It is a FLEET-only red —
// ci.yml is ungated and cannot see it.
//
// A real commit dispatches hooks, which is the point of the shim's rule;
// here there are none to dispatch. The repo is `git init`-fresh, with no
// core.hooksPath and no init.templateDir in any scope, so nothing runs that
// either test's subject — a version string and a build stamp — could see.
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
			// Carry stderr: a gate refusal says why on stderr and
			// nothing on stdout, so without this the failure reads
			// "exit status 1" and names neither the rule nor the
			// wall (ranger-base-ltk6x — that is how this fixture
			// stayed unreadable for a day).
			var ee *exec.ExitError
			if errors.As(err, &ee) && len(ee.Stderr) > 0 {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, ee.Stderr)
			}
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", ".")
	// `git add -A` first: a pathspec matches only what git already has an
	// index entry for, and every file here is new to this repo.
	git("add", "-A")
	git("commit", "-q", "-m", "fixture", "--", ".")
	sha = git("rev-parse", "HEAD")
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
