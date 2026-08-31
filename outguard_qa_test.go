package posse

// QA pins for ranger-base-9hyc: scripts/release-artifacts.sh --out was an
// unguarded `rm -rf` on a path a human types by hand, one step away from the
// irreversible one in docs/runbooks/release.md.
//
// The wipe itself is not the bug and is not removed here — a tarball left over
// from the previous version would be swept into checksums.txt and the upload
// glob. What is pinned is WHAT may be wiped: a directory that is absent, empty,
// or holds nothing but this build's own output.
//
// Every refusal below is an assertion of ABSENCE (the canary survived), which a
// rig that measures nothing also satisfies. So the file carries two positive
// witnesses: TestReleaseOutGuardControlArmDeletesTheCanary reconstructs the
// pre-fix script by excising the guard between its own markers and shows the
// same fixture losing the canary, and the accept cases require the guard to let
// a wipe through and the run to reach the build.
//
// The fixture is a throwaway git repo with its own $HOME: nothing here reads or
// writes the operator's tree, and the $HOME refusal is exercised against a
// $HOME that is a temp directory.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	outGuardOpen  = "# >>> out-guard (ranger-base-9hyc)"
	outGuardClose = "# <<< out-guard (ranger-base-9hyc)"
	// The version the fixture's app.go carries. Deliberately not
	// internal/posse.Version: this pin is about --out, and must not go red on a
	// release bump.
	outGuardVersion = "9.9.9"
)

// outGuardEnv is the environment every command in this file runs under. It
// strips the persona gate shims from PATH (they intercept `git commit`, which
// the fixture needs) and cuts the run off from the operator's git config and
// home directory.
func outGuardEnv(home string) []string {
	var path []string
	gates := os.Getenv("RHQ_GATES_DIR")
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if gates != "" && dir == filepath.Join(gates, "bin") {
			continue
		}
		path = append(path, dir)
	}
	return []string{
		"PATH=" + strings.Join(path, string(filepath.ListSeparator)),
		"HOME=" + home,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=posse qa", "GIT_AUTHOR_EMAIL=qa@example.invalid",
		"GIT_COMMITTER_NAME=posse qa", "GIT_COMMITTER_EMAIL=qa@example.invalid",
		// The guard runs before the build; `false` makes the accept cases
		// stop at the first `go build` instead of cross-compiling four
		// binaries to prove they got that far.
		"GOBIN=false",
		"TMPDIR=" + os.Getenv("TMPDIR"),
	}
}

// outGuardFixture builds a git repo the script will accept — one commit, an
// internal/posse/app.go whose Version matches outGuardVersion — and returns the
// repo path and the fixture $HOME beside it.
func outGuardFixture(t *testing.T) (repo, home string) {
	t.Helper()
	root := t.TempDir()
	repo = filepath.Join(root, "repo")
	home = filepath.Join(root, "home")
	for _, d := range []string{filepath.Join(repo, "internal", "posse"), home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	app := "package posse\n\nconst (\n\tVersion = \"" + outGuardVersion + "\"\n)\n"
	if err := os.WriteFile(filepath.Join(repo, "internal", "posse", "app.go"), []byte(app), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = outGuardEnv(home)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	// The safe form the posse gate demands (rangerhq-ojnw): named paths
	// after --, no -i. A fixture is not exempt from the crew's own deny.
	git("-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null",
		"commit", "-qm", "fixture", "--", filepath.Join("internal", "posse", "app.go"))
	return repo, home
}

// outGuardScript copies the real script into the fixture, optionally rewritten.
func outGuardScript(t *testing.T, repo string, rewrite func(string) string) string {
	t.Helper()
	body, err := os.ReadFile("scripts/release-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if rewrite != nil {
		text = rewrite(text)
	}
	dir := filepath.Join(repo, "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "release-artifacts.sh")
	if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func runOutGuard(t *testing.T, repo, home, script, out string) (exit int, combined string) {
	t.Helper()
	cmd := exec.Command("sh", script, "--version", "v"+outGuardVersion, "--out", out)
	cmd.Dir = repo
	cmd.Env = outGuardEnv(home)
	b, err := cmd.CombinedOutput()
	switch e := err.(type) {
	case nil:
		exit = 0
	case *exec.ExitError:
		exit = e.ExitCode()
	default:
		t.Fatalf("sh %s: %v\n%s", script, err, b)
	}
	return exit, string(b)
}

// plantCanary makes dir hold something the script did not write.
func plantCanary(t *testing.T, dir string) string {
	t.Helper()
	deep := filepath.Join(dir, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(deep, "notes.md")
	if err := os.WriteFile(canary, []byte("the operator's work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return canary
}

func TestReleaseOutGuardRefusesWhatItDidNotWrite(t *testing.T) {
	repo, home := outGuardFixture(t)
	script := outGuardScript(t, repo, nil)
	scratch := t.TempDir()

	// --out $HOME/sub/.. and a symlink into $HOME: the identity refusals are
	// on a canonicalised path, not on the string that was typed.
	if err := os.MkdirAll(filepath.Join(home, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	homeLink := filepath.Join(scratch, "home-link")
	if err := os.Symlink(home, homeLink); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(scratch, "dangling")
	if err := os.Symlink(filepath.Join(scratch, "nothing-here"), dangling); err != nil {
		t.Fatal(err)
	}
	regular := filepath.Join(scratch, "a-file")
	if err := os.WriteFile(regular, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A DIRECTORY whose name is on the allowlist. rm -rf would take it whole.
	masquerade := filepath.Join(scratch, "masquerade")
	if err := os.MkdirAll(filepath.Join(masquerade, "posse_"+outGuardVersion+"_darwin_arm64.tar.gz"), 0o755); err != nil {
		t.Fatal(err)
	}
	// One stray PLAIN FILE beside real output, and nothing else. Found by
	// mutation: widening the allowlist to `*` left every other case here green,
	// because each of them is a stray DIRECTORY.
	oneFile := filepath.Join(scratch, "one-file")
	if err := os.MkdirAll(oneFile, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"checksums.txt": "ours\n",
		"notes.md":      "the operator's work\n",
	} {
		if err := os.WriteFile(filepath.Join(oneFile, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A stray DOTFILE that is not .DS_Store: the hidden half of the glob.
	oneDotfile := filepath.Join(scratch, "one-dotfile")
	if err := os.MkdirAll(oneDotfile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oneDotfile, ".env"), []byte("SECRET=\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, out, want string
	}{
		{"the bead's repro", filepath.Join(scratch, "canary"), `holds "deep"`},
		{"relative dot is the repo root", ".", "that is the repository root"},
		{"repo root, absolute", repo, "that is the repository root"},
		{"repo root, trailing slash", repo + "/", "that is the repository root"},
		{"$HOME", home, "that is $HOME"},
		{"$HOME reached through ..", filepath.Join(home, "sub", ".."), "that is $HOME"},
		{"a symlink into $HOME", homeLink, "that is $HOME"},
		{"the filesystem root", "/", "that is the filesystem root"},
		{"an existing regular file", regular, "exists and is not a directory"},
		{"a dangling symlink", dangling, "symlink to a non-directory"},
		{"a parent that is not there", filepath.Join(scratch, "absent", "deeper"), "parent directory does not exist"},
		{"a directory named like a tarball", masquerade, `holds "posse_` + outGuardVersion + `_darwin_arm64.tar.gz"`},
		{"one stray file beside real output", oneFile, `holds "notes.md"`},
		{"one stray dotfile", oneDotfile, `holds ".env"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh canary per case: a probe that mutates the fixture the
			// next probe reads measures the last run, not this one.
			canaryDir := filepath.Join(scratch, "canary")
			if err := os.RemoveAll(canaryDir); err != nil {
				t.Fatal(err)
			}
			canary := plantCanary(t, canaryDir)

			exit, out := runOutGuard(t, repo, home, script, tc.out)
			if exit != 2 {
				t.Errorf("--out %q: exit %d, want 2 (refusal)\n%s", tc.out, exit, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("--out %q: refusal did not say %q\n%s", tc.out, tc.want, out)
			}
			if strings.Contains(out, "release-artifacts: v"+outGuardVersion+" from ") {
				t.Errorf("--out %q: run reached the build; the guard let it past\n%s", tc.out, out)
			}
			for _, survivor := range []string{canary, filepath.Join(oneFile, "notes.md")} {
				if b, err := os.ReadFile(survivor); err != nil || string(b) != "the operator's work\n" {
					t.Errorf("--out %q: canary %s is gone or changed (%v)", tc.out, survivor, err)
				}
			}
			if _, err := os.Stat(filepath.Join(oneDotfile, ".env")); err != nil {
				t.Errorf("--out %q: canary .env is gone: %v", tc.out, err)
			}
			// The repo-root and $HOME cases must not have been wiped either.
			if _, err := os.Stat(filepath.Join(repo, "internal", "posse", "app.go")); err != nil {
				t.Fatalf("--out %q: the fixture repo lost app.go: %v", tc.out, err)
			}
			if _, err := os.Stat(filepath.Join(home, "sub")); err != nil {
				t.Fatalf("--out %q: the fixture $HOME lost sub/: %v", tc.out, err)
			}
		})
	}
}

// The accept side. Without it, "refuses everything" would pass every test
// above, and the script would be useless rather than safe.
func TestReleaseOutGuardStillWipesItsOwnOutput(t *testing.T) {
	repo, home := outGuardFixture(t)
	script := outGuardScript(t, repo, nil)
	scratch := t.TempDir()

	stale := []string{
		"posse_" + outGuardVersion + "_darwin_arm64.tar.gz",
		"posse_" + outGuardVersion + "_linux_amd64.tar.gz",
		"checksums.txt",
		"posse.rb",
		".DS_Store", // Finder's, not the operator's work
	}

	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T, out string)
	}{
		{"absent", func(t *testing.T, out string) {}},
		{"empty", func(t *testing.T, out string) {
			if err := os.MkdirAll(out, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"holds only a previous build", func(t *testing.T, out string) {
			if err := os.MkdirAll(out, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range stale {
				if err := os.WriteFile(filepath.Join(out, name), []byte("stale\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(scratch, tc.name)
			if err := os.RemoveAll(out); err != nil {
				t.Fatal(err)
			}
			tc.prepare(t, out)

			exit, combined := runOutGuard(t, repo, home, script, out)
			if exit == 2 {
				t.Fatalf("--out %s was refused; the guard blocks its own output directory\n%s", out, combined)
			}
			// The witness that the guard was passed rather than skipped: the
			// run got as far as announcing the build (GOBIN=false then fails
			// it, which is why exit is 1 and not 0).
			if !strings.Contains(combined, "release-artifacts: v"+outGuardVersion+" from ") {
				t.Fatalf("--out %s never reached the build\n%s", out, combined)
			}
			entries, err := os.ReadDir(out)
			if err != nil {
				t.Fatalf("--out %s was not created: %v", out, err)
			}
			if len(entries) != 0 {
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Errorf("--out %s still holds %v; the wipe did not happen", out, names)
			}
		})
	}
}

// The positive witness for the whole file: the same fixture, the same canary,
// the script with the guard cut out. If this does not delete the canary, the
// refusals above prove nothing about the guard.
func TestReleaseOutGuardControlArmDeletesTheCanary(t *testing.T) {
	repo, home := outGuardFixture(t)
	var excised bool
	script := outGuardScript(t, repo, func(text string) string {
		open := strings.Index(text, outGuardOpen)
		close := strings.Index(text, outGuardClose)
		if open < 0 || close < open {
			return text
		}
		end := close + len(outGuardClose)
		excised = true
		return text[:open] + "case $OUT in /*) ;; *) OUT=$PWD/$OUT ;; esac\nrm -rf \"$OUT\"\nmkdir -p \"$OUT\"\n" + text[end:]
	})
	if !excised {
		t.Fatalf("scripts/release-artifacts.sh no longer carries the %q / %q markers, so the pre-fix control could not be built — the tests above are measuring nothing", outGuardOpen, outGuardClose)
	}

	out := filepath.Join(t.TempDir(), "canary")
	canary := plantCanary(t, out)
	exit, combined := runOutGuard(t, repo, home, script, out)
	if exit == 2 {
		t.Fatalf("the unguarded control refused; the excision did not remove the guard\n%s", combined)
	}
	if _, err := os.Stat(canary); !os.IsNotExist(err) {
		t.Fatalf("the unguarded control left %s (err=%v); this rig cannot observe a deletion, so the refusals above are not evidence", canary, err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("the unguarded control did not recreate %s: %v", out, err)
	}
}
