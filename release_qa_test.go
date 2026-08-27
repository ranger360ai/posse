package posse

// QA pins for ranger-base-dbe (verified under ranger-base-lsj).
//
// Two claims, both load-bearing for a tag:
//
//  1. workflow_dispatch used to vet and test the triggering branch while
//     building --rev of the tag. checkout now takes
//     ref: ${{ inputs.tag || github.ref }}, so the tested tree and the
//     shipped tree are the same commit on both triggers.
//  2. The suite had only ever run on darwin. scripts/test-linux.sh is the
//     same gate the release workflow runs (go vet ./... && make test), in a
//     throwaway container, repo mounted read-only, as the invoking user.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReleaseWorkflowChecksOutTheTagItBuilds(t *testing.T) {
	contents, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	body := string(contents)
	const want = `ref: ${{ inputs.tag || github.ref }}`
	if !strings.Contains(body, want) {
		t.Fatalf("release.yml checkout dropped %q — that ref is what keeps the tested tree and the shipped tree the same commit (ranger-base-dbe)", want)
	}
	// The pre-fix shape: checkout with fetch-depth only, no ref. A comment
	// mentioning fetch-depth is fine; a with: block that is only fetch-depth
	// is the original bug.
	if strings.Contains(body, "uses: actions/checkout@v4\n        with:\n          fetch-depth: 0\n") {
		t.Fatal("release.yml checkout is back to triggering-ref only (no ref:); workflow_dispatch would test main and ship the tag")
	}
}

func TestTestLinuxScriptIsTheReleaseGateOnLinux(t *testing.T) {
	script, err := os.ReadFile("scripts/test-linux.sh")
	if err != nil {
		t.Fatal(err)
	}
	body := string(script)
	for _, want := range []string{
		`--user "$(id -u):$(id -g)"`,
		`-v "$REPO_ROOT:/repo:ro"`,
		`gate='go vet ./... && make test'`,
		`IMAGE="${IMAGE:-golang:$go_minor}"`,
		`go_minor=$(sed -n 's/^go \([0-9][0-9]*\.[0-9][0-9]*\).*$/\1/p' "$REPO_ROOT/go.mod" | head -1)`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scripts/test-linux.sh missing %q", want)
		}
	}
	info, err := os.Stat("scripts/test-linux.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/test-linux.sh is not executable")
	}

	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	mk := string(makefile)
	if !strings.Contains(mk, "test-linux:\n\tscripts/test-linux.sh\n") {
		t.Error("Makefile lost the test-linux target")
	}
	if !strings.Contains(mk, " test-linux ") && !strings.Contains(mk, " test-linux\n") {
		t.Error("Makefile .PHONY no longer lists test-linux")
	}

	contrib, err := os.ReadFile("CONTRIBUTING.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contrib), "make test-linux") {
		t.Error("CONTRIBUTING.md no longer tells macOS contributors to run make test-linux")
	}
}

func TestTestLinuxScriptInvokesTheReleaseGateViaDocker(t *testing.T) {
	runArgs, exit := tlDocker(t, nil)
	if exit != 0 {
		t.Fatalf("default run exit %d, want 0", exit)
	}
	tlMustHave(t, runArgs, "--rm")
	tlMustHave(t, runArgs, "--user", strconv.Itoa(os.Getuid())+":"+strconv.Itoa(os.Getgid()))
	repo, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	tlMustHave(t, runArgs, "-v", repo+":/repo:ro")
	tlMustHave(t, runArgs, "-w", "/repo")
	image := "golang:" + tlGoMinor(t)
	if !tlContains(runArgs, image) {
		t.Errorf("docker run image: got %v, want %s (from go.mod)", runArgs, image)
	}
	if !tlContains(runArgs, "go vet ./... && make test") {
		t.Errorf("docker run command: got %v, want the release gate", runArgs)
	}

	t.Run("false exits 1", func(t *testing.T) {
		_, exit := tlDocker(t, nil, "false")
		if exit != 1 {
			t.Fatalf("false: exit %d, want 1", exit)
		}
	})

	t.Run("PLATFORM override", func(t *testing.T) {
		runArgs, exit := tlDocker(t, []string{"PLATFORM=linux/amd64"}, "true")
		if exit != 0 {
			t.Fatalf("PLATFORM=linux/amd64: exit %d", exit)
		}
		tlMustHave(t, runArgs, "--platform", "linux/amd64")
	})
}

// ranger-base-dbe landed make test-linux and fixed checkout's ref, but the
// operator runbook still describes the pre-fix world: dispatch tests main
// and ships the tag, and precondition 3 is an ad-hoc docker one-liner with
// a hardcoded uid 1000 rather than make test-linux.
func TestReleaseRunbookDoesNotStillClaimDispatchDiverges(t *testing.T) {
	contents, err := os.ReadFile("docs/runbooks/release.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(contents)
	if strings.Contains(body, "workflow_dispatch tests a different commit") ||
		strings.Contains(body, "checks out with no `ref:`") {
		t.Error("docs/runbooks/release.md still claims workflow_dispatch tests a different commit than it builds — that was ranger-base-dbe, fixed in b0a8ec7")
	}
	if strings.Contains(body, "-u 1000:1000") {
		t.Error("docs/runbooks/release.md Linux rehearsal still hardcodes uid 1000; this host is not 1000, and scripts/test-linux.sh uses $(id -u):$(id -g)")
	}
	if !strings.Contains(body, "make test-linux") {
		t.Error("docs/runbooks/release.md precondition 3 never names make test-linux")
	}
}

// PLATFORM=linux/amd64 pulls golang:<minor> as amd64 and leaves that tag
// pointing at the amd64 blob. A later default run (no PLATFORM) then qemu-
// emulates amd64 instead of testing the host arch NOTES.md says it tests.
// Always passing --platform, defaulting to the host, stops the tag poison.
func TestTestLinuxDefaultRunPinsPlatformSoAnAmd64OverrideCannotPoisonTheTag(t *testing.T) {
	t.Skip("ranger-base-1qm5: default docker run has no --platform; PLATFORM=linux/amd64 poisons the golang:<minor> tag")
	runArgs, exit := tlDocker(t, nil)
	if exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	if !tlContains(runArgs, "--platform") {
		t.Fatal("default docker run has no --platform; PLATFORM=linux/amd64 therefore poisons the golang:<minor> tag for every later default run")
	}
}

// Every dispatched session works in a linked worktree (AGENTS.md, "Landing
// the plane"), where .git is a FILE naming a gitdir OUTSIDE the tree. Mount
// only the worktree and git in the container resolves nothing: the three
// seedpub publication-boundary tests (ADR 0012) go red 40s in looking exactly
// like product failures, and the honest report becomes "green except three
// known env failures" — one step from "green enough" (ranger-base-v0gm).
// Every git dir outside $REPO_ROOT must be mounted at the SAME absolute path
// git names, because that pointer is baked into the .git file — and :ro,
// which is the property that must not move.
func TestTestLinuxMountsGitDirsLivingOutsideTheWorktree(t *testing.T) {
	const common = "/elsewhere/posse/.git"
	const linked = common + "/worktrees/session"

	t.Run("linked worktree", func(t *testing.T) {
		runArgs, exit := tlDocker(t, []string{"FAKE_GIT_REV_PARSE=" + common + "\n" + linked}, "true")
		if exit != 0 {
			t.Fatalf("exit %d", exit)
		}
		tlMustHave(t, runArgs, "-v", common+":"+common+":ro")
		if tlContains(runArgs, linked+":"+linked+":ro") {
			t.Errorf("gitdir %s is inside %s; it needs no mount of its own: %v", linked, common, runArgs)
		}
		tlMustHave(t, runArgs, "-e", "GIT_CONFIG_COUNT=2")
		tlMustHave(t, runArgs, "-e", "GIT_CONFIG_VALUE_1="+common)
	})

	t.Run("gitdir outside the common dir is mounted too", func(t *testing.T) {
		const apart = "/apart/session.gitdir"
		runArgs, exit := tlDocker(t, []string{"FAKE_GIT_REV_PARSE=" + common + "\n" + apart}, "true")
		if exit != 0 {
			t.Fatalf("exit %d", exit)
		}
		tlMustHave(t, runArgs, "-v", common+":"+common+":ro")
		tlMustHave(t, runArgs, "-v", apart+":"+apart+":ro")
		tlMustHave(t, runArgs, "-e", "GIT_CONFIG_COUNT=3")
	})

	t.Run("ordinary checkout mounts nothing extra", func(t *testing.T) {
		repo, err := filepath.Abs(".")
		if err != nil {
			t.Fatal(err)
		}
		gitdir := filepath.Join(repo, ".git")
		runArgs, exit := tlDocker(t, []string{"FAKE_GIT_REV_PARSE=" + gitdir + "\n" + gitdir}, "true")
		if exit != 0 {
			t.Fatalf("exit %d", exit)
		}
		if tlContains(runArgs, gitdir+":"+gitdir+":ro") {
			t.Errorf("git dir is already inside the /repo mount; mounting it again is noise: %v", runArgs)
		}
		tlMustHave(t, runArgs, "-e", "GIT_CONFIG_COUNT=1")
	})

	// Whatever the layout, the repo and its git dir stay read-only: a gate run
	// must not be able to write to either.
	t.Run("every mount stays read-only", func(t *testing.T) {
		runArgs, _ := tlDocker(t, []string{"FAKE_GIT_REV_PARSE=" + common + "\n" + linked}, "true")
		for i, a := range runArgs {
			if i == 0 || runArgs[i-1] != "-v" {
				continue
			}
			if strings.HasPrefix(a, "/gocache") || strings.Contains(a, ":/gocache") || strings.Contains(a, ":/gomodcache") {
				continue // the build cache is the one writable thing, and it lives outside the repo
			}
			if !strings.HasSuffix(a, ":ro") {
				t.Errorf("mount %q is writable; repo and git dir must both be :ro", a)
			}
		}
	})
}

func tlGoMinor(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "go ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "go "))
		parts := strings.Split(rest, ".")
		if len(parts) < 2 {
			t.Fatalf("go.mod go directive %q", line)
		}
		return parts[0] + "." + parts[1]
	}
	t.Fatal("go.mod has no go directive")
	return ""
}

// tlDocker runs scripts/test-linux.sh against a fake docker that logs argv
// and never talks to a daemon. extra is additional env entries (KEY=val).
func tlDocker(t *testing.T, extra []string, scriptArgs ...string) (runArgs []string, exit int) {
	t.Helper()
	scratch := t.TempDir()
	logPath := filepath.Join(scratch, "docker.log")
	fake := filepath.Join(scratch, "docker")
	script := `#!/bin/sh
log=${FAKE_DOCKER_LOG:?}
{
	printf 'DOCKER'
	for a; do printf '\t%s' "$a"; done
	printf '\n'
} >>"$log"
case "${1:-}" in
info) exit 0 ;;
run)
	prev=
	for a; do
		if [ "$prev" = "-c" ] && [ "$a" = "false" ]; then exit 1; fi
		prev=$a
	done
	exit 0
	;;
*) exit 0 ;;
esac
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// A fake git in front of the real one, inert unless FAKE_GIT_REV_PARSE is
	// set: that is how the linked-worktree branch (ranger-base-v0gm) gets
	// exercised on a machine that is not in a linked worktree, and vice versa.
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	gitStub := "#!/bin/sh\n" +
		"if [ -n \"${FAKE_GIT_REV_PARSE:-}\" ] && [ \"${1:-}\" = rev-parse ]; then\n" +
		"\tprintf '%s\\n' \"$FAKE_GIT_REV_PARSE\"\n" +
		"\texit 0\n" +
		"fi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(scratch, "git"), []byte(gitStub), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("./scripts/test-linux.sh", scriptArgs...)
	cmd.Env = append(os.Environ(),
		"PATH="+scratch+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_DOCKER_LOG="+logPath,
		"XDG_CACHE_HOME="+filepath.Join(scratch, "cache"),
	)
	cmd.Env = append(cmd.Env, extra...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		exit = 0
	} else if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else {
		t.Fatalf("test-linux.sh: %v\n%s", err, out)
	}
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("fake docker log: %v\n%s", err, out)
	}
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if !strings.HasPrefix(line, "DOCKER\trun") {
			continue
		}
		runArgs = strings.Split(line, "\t")[1:]
	}
	if runArgs == nil && exit == 0 {
		t.Fatalf("fake docker never saw `run`\nlog:\n%s\nout:\n%s", b, out)
	}
	return runArgs, exit
}

// ranger-base-v0gm shipped TWO answers to the linked-worktree problem: mount
// the git dir when git can name it (pinned above), and — when `.git` is a
// FILE whose gitdir git cannot resolve at all — die up front naming it,
// instead of failing 40s in through three seedpub tests that look like
// product failures. Only the first had a pin; this is the second
// (found verifying that close, ranger-base-nrnc).
//
// Hermetic rather than machine-shaped: the skeleton below is a repo root as
// the script reads one — itself and go.mod — with a `.git` file naming a
// gitdir that exists nowhere. So the guard is exercised on an ordinary
// checkout as well as in a session worktree, and no fake git is needed
// because nothing here resolves for real.
func TestTestLinuxDiesUpFrontOnAnUnresolvableGitdir(t *testing.T) {
	const ghost = "/nowhere/does/not/exist/.git/worktrees/ghost"

	scratch := t.TempDir()
	root := filepath.Join(scratch, "repo")
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("scripts/test-linux.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "test-linux.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), mod, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+ghost+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(scratch, "docker.log")
	bin := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\n" +
		"{ printf 'DOCKER'; for a; do printf '\\t%s' \"$a\"; done; printf '\\n'; } >>\"${FAKE_DOCKER_LOG:?}\"\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("./scripts/test-linux.sh", "true")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_DOCKER_LOG="+logPath,
		"XDG_CACHE_HOME="+filepath.Join(scratch, "cache"),
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("an unresolvable gitdir must fail the run, not start one:\n%s", out)
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Errorf("want exit 1, got %v:\n%s", err, out)
	}
	// Naming the gitdir is the whole point: the reader has to be told which
	// pointer dangled, or the message is one more thing to go and find out.
	if !strings.Contains(string(out), ghost) {
		t.Errorf("the refusal must name the gitdir %q:\n%s", ghost, out)
	}
	// And it must be UP FRONT: no container, so no 40s and no red tests that
	// read like the product.
	if b, err := os.ReadFile(logPath); err == nil && strings.Contains(string(b), "DOCKER\trun") {
		t.Errorf("the container must never start on an unresolvable gitdir:\n%s", b)
	}
}

func tlContains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func tlMustHave(t *testing.T, args []string, want ...string) {
	t.Helper()
	if len(want) == 0 {
		return
	}
	for i := 0; i+len(want) <= len(args); i++ {
		match := true
		for j := range want {
			if args[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Errorf("docker argv missing %s\nargv: %s", want, strings.Join(args, " | "))
}
