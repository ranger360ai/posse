package treepins

// QA pins for ranger-base-sydnc — the two shell scripts on this box that hash,
// measured on a PATH that has no `shasum`.
//
// THE FINDING. `shasum` is not on every box's PATH. Arch — what the always-on
// Linux host of ADR 0080 runs — ships it with perl at /usr/bin/core_perl/shasum,
// which is off the default PATH, and carries coreutils' `sha256sum` instead.
// ubuntu and macOS carry both, which is why CI and the laptop were green over
// this for as long as either script has existed.
//
// MEASURED 2026-11-09 on the host by the coordinator's Linux lane, and here
// by hiding `shasum` from PATH while leaving `sha256sum` on it:
//
//   - scripts/verify-hook-freshness.sh: `shasum: command not found`, so BOTH
//     sides of every compare hashed to the EMPTY STRING, the empty strings
//     compared EQUAL, and the script printed `fresh` and exited 0 over a
//     planted stale hook. Nine arms of hookfreshness_qa_test.go red there with
//     "a stale body must be a finding (exit 1), got 0". The script is a
//     detective control, so that is not test noise: on that box it passed
//     every stale and every foreign hook, silently.
//   - scripts/gotest.sh: the cache key is a sha, so every build hashed to the
//     empty string, every build named the same cache file (`<slug>-.test`), and
//     the reuse branch handed back the FIRST binary ever cached for that
//     package, forever. `--self-test` reds three arms, the same three on both
//     boxes, the last of them "exit status: a red test reds the wrapper:
//     wrapper exited 0" — a changed package's new tests never ran, and a red
//     test greened the wrapper.
//
// Neither `set -e` nor `set -o pipefail` catches this class: a command
// substitution whose command is not found yields the empty string and the
// status of the assignment it feeds is discarded (MEASURED 2026-11-09, bash
// 3.2.57 on darwin; the host's own run shows the same, since it got past that
// line too). So each script picks its hasher by probe, proves it against a
// known digest before measuring anything, and treats a digest that is not 64
// hex characters as an error and never as a value.
//
// WHY THESE ARMS AND NOT A CENSUS. A census of `shasum` spellings under
// scripts/ would have caught this one too, but it would have been green over
// the scripts that already probe (scripts/release-artifacts.sh) and red over
// the darwin-only instruments that do not need to (the ADR probe scripts,
// scripts/macos-install-probe.sh) — a rule about text, in place of the
// behaviour. These arms run the two scripts, on the PATH the host has. The
// census is filed separately.
//
// HIDING ONE COMMAND, not its directory: on darwin `shasum` and `sha256sum`
// are both under /usr/bin, and dropping /usr/bin takes `git` and `mktemp` with
// it. hfPathWithout builds a directory of symlinks to everything the given
// PATH holds EXCEPT the named commands, which is the only faithful way to
// stand up the host's shape here.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hfPathWithout returns a one-directory PATH holding a symlink to every
// command the dirs of `path` hold, except those named in `hide`. The first
// spelling of a name wins, exactly as PATH lookup would resolve it, so the
// farm resolves what the original PATH resolved — minus what is hidden.
func hfPathWithout(t *testing.T, path string, hide ...string) string {
	t.Helper()
	hidden := map[string]bool{}
	for _, h := range hide {
		hidden[h] = true
	}
	farm := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(farm, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := 0
	for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue // not a directory on this box; PATH lookup skips it too
		}
		for _, e := range ents {
			name := e.Name()
			if hidden[name] {
				continue
			}
			dst := filepath.Join(farm, name)
			if _, err := os.Lstat(dst); err == nil {
				continue // an earlier PATH dir already wins this name
			}
			if err := os.Symlink(filepath.Join(dir, name), dst); err == nil {
				linked++
			}
		}
	}
	// A farm with nothing in it would make every arm below fail for want of
	// `git` rather than for want of a hasher, which is the one way these
	// arms could read as findings while measuring nothing.
	if linked == 0 {
		t.Fatalf("hfPathWithout linked no commands out of %q — the fixture, not the script, is what failed", path)
	}
	for _, h := range hide {
		if _, err := os.Lstat(filepath.Join(farm, h)); err == nil {
			t.Fatalf("hfPathWithout was asked to hide %q and did not", h)
		}
	}
	return farm
}

// hfSha256sumDir names the directory holding this box's `sha256sum`, which is
// the hasher the host has and `shasum` is not. It is /usr/bin on the host and
// /sbin on this darwin box, so the arms that hide `shasum` have to put it on
// the rig's PATH by name instead of relying on /usr/bin:/bin — otherwise
// hiding `shasum` here builds a box with NO hasher, which is a different arm.
func hfSha256sumDir(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("sha256sum")
	if err != nil {
		t.Skip("no sha256sum on this box, so the host's PATH shape cannot be built here")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(abs)
}

// ─── scripts/verify-hook-freshness.sh ────────────────────────────────────

// The control. A fresh box must still read fresh when the hasher is
// `sha256sum` rather than `shasum` — otherwise the next arm's exit 1 could be
// the farm talking and not the planted drift.
func TestQAHookFreshnessFreshBoxPassesWithoutShasum(t *testing.T) {
	dir := hfSha256sumDir(t)
	r := hfNewRig(t, map[string]string{"priv": "private", "pub": "public"})
	r.pathDirs = []string{dir}
	r.hideFromPath = []string{"shasum"}
	out, code := r.run(t)
	if code != 0 {
		t.Fatalf("a fresh box with shasum off PATH must still exit 0, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "fresh") {
		t.Errorf("the verdict no longer reads fresh:\n%s", out)
	}
}

// THE ARM THAT SHIPPED THE HOLE. The same planted drift as
// TestQAHookFreshnessCatchesAStaleBodyThatStillRefuses, with `shasum` off
// PATH: before ranger-base-sydnc this printed `fresh` and exited 0.
func TestQAHookFreshnessStillCatchesAStaleBodyWithoutShasum(t *testing.T) {
	dir := hfSha256sumDir(t)
	r := hfNewRig(t, map[string]string{"priv": "private"})
	body := hfRead(t, r.hook("priv"))
	const erba = "'git diff --no-ext-diff HEAD -- <paths>'"
	if !strings.Contains(body, erba) {
		t.Fatalf("rig never built: the current render does not carry %s", erba)
	}
	hfWrite(t, r.hook("priv"), strings.Replace(body, erba, "'git diff'", 1))

	r.pathDirs = []string{dir}
	r.hideFromPath = []string{"shasum"}
	out, code := r.run(t)
	if code != 1 {
		t.Fatalf("with shasum off PATH a stale body must still be a finding (exit 1), got %d — "+
			"this is the ranger-base-sydnc hole: the control passes every stale hook on that box\n%s", code, out)
	}
	if !strings.Contains(out, "prepare-commit-msg is STALE") {
		t.Errorf("staleness must be named as such:\n%s", out)
	}
}

// And with NO hasher at all the script must say nothing was measured (exit 2),
// never report a verdict. A box with no sha256 cannot be assessed; the one
// answer that must never come out of that is `fresh`.
func TestQAHookFreshnessRefusesToMeasureWithNoHasherOnPath(t *testing.T) {
	r := hfNewRig(t, map[string]string{"priv": "private"})
	r.hideFromPath = []string{"shasum", "sha256sum"}
	out, code := r.run(t)
	if code != 2 {
		t.Fatalf("with no sha256 on PATH the script must exit 2 (nothing measured), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "nothing measured") {
		t.Errorf("the refusal must say nothing was measured:\n%s", out)
	}
	// The clean verdict, by its own spelling — "fresh" on its own matches
	// this script's name.
	if strings.Contains(out, "verify-hook-freshness: fresh") {
		t.Errorf("a box with no hasher was called fresh — that is the finding this file exists for:\n%s", out)
	}
}

// ─── scripts/gotest.sh ───────────────────────────────────────────────────

// The wrapper's own self-test, on the host's PATH shape. Its arms are the
// control, so this says the whole instrument survives the move off `shasum`.
func TestQATheWrapperStillProvesItReusesTheBinaryWithoutShasum(t *testing.T) {
	farm := hfPathWithout(t, os.Getenv("PATH")+":"+hfSha256sumDir(t), "shasum")
	cmd := exec.Command("bash", testBinWrapper, "--self-test")
	cmd.Env = hfPathEnv(farm)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test failed with shasum off PATH: %v\n%s", testBinWrapper, err, out)
	}
	for _, want := range []string{
		"reuse: unchanged package keeps one inode",
		"control: a changed test file rebuilds",
		"control: the new test actually ran",
		"exit status: a red test reds the wrapper",
	} {
		if !strings.Contains(string(out), "ok    "+want) {
			t.Errorf("%s --self-test no longer proves %q with shasum off PATH — "+
				"the wrapper is keying its cache on something that is not a hash, "+
				"which makes it run a stale binary and report its result\n%s",
				testBinWrapper, want, out)
		}
	}
}

// With no hasher at all the wrapper must refuse loudly rather than key its
// cache on the empty string — a cache keyed on a hash that did not happen
// hands back whatever is already there.
func TestQATheWrapperRefusesWithNoHasherOnPath(t *testing.T) {
	farm := hfPathWithout(t, os.Getenv("PATH"), "shasum", "sha256sum")
	cmd := exec.Command("bash", testBinWrapper, "--self-test")
	cmd.Env = hfPathEnv(farm)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("%s ran with no sha256 on PATH — the cache key is a sha and there was nothing to compute it with\n%s",
			testBinWrapper, out)
	}
	if !strings.Contains(string(out), "sha256sum") || !strings.Contains(string(out), "shasum") {
		t.Errorf("the refusal must name what it looked for, so the reader can put one on PATH:\n%s", out)
	}
}

// hfPathEnv is this process's environment with PATH replaced. Everything else
// is kept: the wrapper needs HOME for its cache and TMPDIR for its builds.
func hfPathEnv(farm string) []string {
	env := []string{"PATH=" + farm}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "PATH=") {
			continue
		}
		env = append(env, kv)
	}
	return env
}
