package posse

// QA pins for ranger-base-9vg3 — the Homebrew bottles.
//
// THE DEFECT these exist to keep fixed. INSTALL.md §2 sells
// `brew install ranger360ai/tap/posse` as "a release binary, no Go needed".
// A formula with no bottle is the opposite: brew takes its build-from-source
// path (formula_installer.rb, `unless pour_bottle?`) and runs
// `fatal_build_from_source_checks` — Xcode, the CLT version, the SDK — BEFORE
// it unpacks anything, so on a Mac whose Command Line Tools are behind its
// macOS the install dies with "Your Command Line Tools are too outdated"
// having never read our formula. Measured both arms on Homebrew 6.0.20 /
// macOS 26.4.1 arm64, CLT 15.3: with a bottle brew pours and installs; with
// the bottle block deleted the same install on the same box refuses.
//
// WHY PINS AND NOT JUST THE PROBE. `scripts/macos-install-probe.sh bottle`
// runs the whole route for real — it is the instrument, and it is what
// answers "does this work". But it needs macOS, a Homebrew clone and four
// `go build`s, so it runs on demand and not in CI. These are the arms CI can
// hold: that the two generators agree on every filename brew will ask for,
// that each bottle carries what `def install` declares, and that a release
// actually ships the things.
//
// THE FILENAME IS THE TRAP. brew fetches `<name>-<version>.<tag>.bottle.tar.gz`
// from a non-GitHub-Packages root_url — ONE dash, `Bottle::Filename#url_encode`.
// Its own cache, its docs and every `brew bottle` output spell the same file
// with TWO. Uploading the two-dash name 404s at install time on the platform
// nobody tested, which is the failure this whole file is arranged around.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// the rendered formula
// ---------------------------------------------------------------------------

// bottleBlock is the `bottle do … end` stanza, parsed the way brew reads it.
type bottleBlock struct {
	rootURL string
	// tag -> sha256, and tag -> the cellar it was declared with. Two maps
	// rather than one struct because a missing cellar and a missing sha are
	// different failures and the messages should say which.
	sha    map[string]string
	cellar map[string]string
	order  []string
}

func parseBottleBlock(t *testing.T, body string) bottleBlock {
	t.Helper()
	b := bottleBlock{sha: map[string]string{}, cellar: map[string]string{}}
	in := false
	for _, line := range strings.Split(body, "\n") {
		s := strings.TrimSpace(line)
		switch {
		case s == "bottle do":
			in = true
		case in && s == "end":
			in = false
		case in && strings.HasPrefix(s, `root_url "`):
			b.rootURL = quoted(s)
		case in && strings.HasPrefix(s, "sha256 "):
			// sha256 cellar: :any_skip_relocation, arm64_sonoma: "…"
			rest := strings.TrimPrefix(s, "sha256 ")
			var cellar string
			if strings.HasPrefix(rest, "cellar:") {
				comma := strings.Index(rest, ",")
				if comma < 0 {
					t.Fatalf("bottle sha256 line has a cellar: but no tag: %q", s)
				}
				cellar = strings.TrimSpace(strings.TrimPrefix(rest[:comma], "cellar:"))
				rest = strings.TrimSpace(rest[comma+1:])
			}
			colon := strings.Index(rest, ":")
			if colon < 0 {
				t.Fatalf("bottle sha256 line names no tag: %q", s)
			}
			tag := strings.TrimSpace(rest[:colon])
			b.sha[tag] = quoted(rest)
			b.cellar[tag] = cellar
			b.order = append(b.order, tag)
		}
	}
	return b
}

// The four bottle digests must land on their own tags, and the block must
// carry the root_url that says where to fetch them from. Without root_url brew
// looks in GitHub Packages, which we do not publish to, and every install
// falls back to building from source — the defect, restored silently.
func TestTapFormulaMapsEachBottleDigestToItsOwnTag(t *testing.T) {
	dir := t.TempDir()
	const version = "0.3.0"
	checksums := writeChecksums(t, dir, version, "")

	body, err := exec.Command("sh", "scripts/tap-formula.sh",
		"--version", "v"+version, "--checksums", checksums,
		"--repo", "ranger360ai/posse").Output()
	if err != nil {
		t.Fatalf("tap-formula.sh: %v", err)
	}
	block := parseBottleBlock(t, string(body))

	if len(block.order) == 0 {
		t.Fatalf("the rendered formula has no bottle block — brew would build from source, which is fatal on a Mac with stale Command Line Tools:\n%s", body)
	}
	want := "https://github.com/ranger360ai/posse/releases/download/v" + version
	if block.rootURL != want {
		t.Errorf("bottle root_url is %q, want %q — we do not publish to GitHub Packages, so without this brew cannot find a bottle at all", block.rootURL, want)
	}
	if len(block.order) != len(tapBottleFixture) {
		t.Errorf("bottle block declares %d tags, want %d: %v", len(block.order), len(tapBottleFixture), block.order)
	}
	for _, w := range tapBottleFixture {
		got, ok := block.sha[w.tag]
		if !ok {
			t.Errorf("no sha256 for bottle tag %s — that platform builds from source while every other one pours, and only on someone else's machine", w.tag)
			continue
		}
		if got != w.digest {
			t.Errorf("bottle tag %s carries %s, want %s — that is %s's digest on another tag, which fails at pour time on exactly the platform the releaser does not own",
				w.tag, got, w.digest, bottleAssetOf(got, version))
		}
		if block.cellar[w.tag] != ":any_skip_relocation" {
			t.Errorf("bottle tag %s declares cellar %q, want :any_skip_relocation — the keg is one static binary and two markdown files, and any other cellar makes brew refuse the bottle on a prefix of a different length (`compatible_locations?`)",
				w.tag, block.cellar[w.tag])
		}
	}
}

func bottleAssetOf(sha, version string) string {
	for _, a := range tapBottleFixture {
		if a.digest == sha {
			return "posse-" + version + "." + a.tag + ".bottle.tar.gz"
		}
	}
	for _, a := range tapFormulaFixture {
		if a.digest == sha {
			return "the TARBALL posse_" + version + "_" + a.goos + "_" + a.goarch + ".tar.gz"
		}
	}
	return "no asset in this release"
}

// A build that produced three bottles instead of four must not render. Same
// reason a missing tarball must not: the formula would pour everywhere the
// releaser looked and build from source — fatally, on a stale-CLT Mac —
// everywhere else.
func TestTapFormulaRefusesAChecksumsFileMissingABottle(t *testing.T) {
	dir := t.TempDir()
	const version = "0.3.0"
	missing := "posse-" + version + ".arm64_sonoma.bottle.tar.gz"
	checksums := writeChecksums(t, dir, version, missing)
	out := filepath.Join(dir, "posse.rb")

	b, err := exec.Command("sh", "scripts/tap-formula.sh",
		"--version", "v"+version, "--checksums", checksums, "--out", out).CombinedOutput()
	if err == nil {
		t.Fatalf("tap-formula.sh rendered a formula from a checksums file with no %s:\n%s", missing, b)
	}
	if !strings.Contains(string(b), missing+" is not in") {
		t.Errorf("the refusal does not name the missing bottle:\n%s", b)
	}
	// `bottle_digest` runs inside a command substitution, where `exit 1` leaves
	// only the subshell. The refusal is real only if that status reaches the
	// caller AND no half-formula is left for someone to commit.
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("a partial %s survived the refusal (stat err: %v)", out, err)
	}
}

// ---------------------------------------------------------------------------
// the two generators, run against each other
// ---------------------------------------------------------------------------

// The tags a release must carry, in one place. macOS gets ONE tag per arch, at
// HOMEBREW_MACOS_OLDEST_SUPPORTED: brew falls back to a bottle built for an
// older macOS (OS::Mac::Bottles::Collector#find_older_compatible_tag), so a
// sonoma bottle pours on sequoia and tahoe — verified by pouring one on macOS
// 26. Linux has no such fallback, the override being macOS-only, so its two
// tags are exact and complete.
var bottleTags = []string{"arm64_sonoma", "sonoma", "arm64_linux", "x86_64_linux"}

// releaseFixture builds a throwaway repo that scripts/release-artifacts.sh
// will accept, and runs it with a stub `go` — so the whole release path
// executes in milliseconds and the assertions are about names and layout, not
// about compiling four binaries. GOBIN is the script's own documented seam.
func releaseFixture(t *testing.T, version string) (out string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	// A temp HOME: release-artifacts.sh compares --out against $HOME, and no
	// test in this package may read or write the operator's real one.
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "internal", "rhq"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(repo, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join("internal", "rhq", "app.go"),
		"package rhq\n\nconst (\n\tVersion       = \""+version+"\"\n)\n")
	write("README.md", "readme fixture\n")
	write("INSTALL.md", "install fixture\n")
	write("LICENSE", "license fixture\n")

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.email=probe@example.invalid", "-c", "user.name=probe",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main",
		}, args...)...)
		cmd.Dir = repo
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
	}
	git("init", "-q", ".")
	git("add", "-A")
	git("commit", "-qm", "fixture", "--",
		filepath.Join("internal", "rhq", "app.go"), "README.md", "INSTALL.md", "LICENSE")

	// The stub compiler. It honours only `-o <path>`, which is the whole of
	// what release-artifacts.sh asks of it.
	goStub := filepath.Join(root, "go")
	if err := os.WriteFile(goStub, []byte(
		"#!/bin/sh\nout=\nwhile [ $# -gt 0 ]; do case $1 in -o) out=$2; shift 2 ;; *) shift ;; esac; done\n"+
			"[ -n \"$out\" ] || exit 3\nprintf 'stub posse\\n' > \"$out\"; chmod +x \"$out\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out = filepath.Join(root, "dist")
	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "release-artifacts.sh"),
		"--version", "v"+version, "--out", out)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GOBIN="+goStub, "HOME="+home)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("release-artifacts.sh: %v\n%s", err, b)
	}
	return out
}

func tarMembers(t *testing.T, path string) []string {
	t.Helper()
	b, err := exec.Command("tar", "tzf", path).Output()
	if err != nil {
		t.Fatalf("tar tzf %s: %v", path, err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		// Trailing slashes are kept: they are how tar distinguishes a
		// directory entry from a file, and the caller needs that distinction
		// to say "the keg carries something def install does not".
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// The load-bearing pin: whatever release-artifacts.sh names its bottles,
// tap-formula.sh must be able to find every one of them in the checksums.txt
// they share. The two carry the tag table separately — one to name files, one
// to name DSL keys — so this is the arm that catches them drifting apart, and
// it catches it BEFORE a release rather than at somebody's `brew install`.
func TestReleaseArtifactsAndTapFormulaAgreeOnEveryBottleName(t *testing.T) {
	const version = "9.9.9"
	dist := releaseFixture(t, version)

	// Every bottle brew could ask for is a real file, spelled the way brew
	// spells it in a URL: ONE dash between name and version.
	for _, tag := range bottleTags {
		name := "posse-" + version + "." + tag + ".bottle.tar.gz"
		if _, err := os.Stat(filepath.Join(dist, name)); err != nil {
			t.Errorf("no %s in the release output — brew fetches exactly that name from root_url and gets a 404 on %s (%v)", name, tag, err)
		}
		// The two-dash spelling is brew's CACHE name and its docs' name. If it
		// ever shows up here, something switched to `brew bottle`'s output
		// convention and the release will 404 for everyone.
		wrong := "posse--" + version + "." + tag + ".bottle.tar.gz"
		if _, err := os.Stat(filepath.Join(dist, wrong)); err == nil {
			t.Errorf("the release wrote %s — that is brew's CACHE spelling; the URL it fetches uses one dash", wrong)
		}
	}

	sums, err := os.ReadFile(filepath.Join(dist, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dist)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		if !strings.Contains(string(sums), "  "+e.Name()+"\n") {
			t.Errorf("%s is not in checksums.txt — tap-formula.sh reads that manifest, and a bottle missing from it cannot be rendered into the formula", e.Name())
		}
	}

	// And the formula renders from it, with every bottle digest resolved to
	// the file release-artifacts.sh actually wrote.
	body, err := exec.Command("sh", "scripts/tap-formula.sh",
		"--version", "v"+version, "--checksums", filepath.Join(dist, "checksums.txt")).Output()
	if err != nil {
		t.Fatalf("tap-formula.sh could not render from release-artifacts.sh's own manifest: %v", err)
	}
	block := parseBottleBlock(t, string(body))
	// The positive witness. Without it every assertion below is vacuous over a
	// formula that lost its bottle block entirely, and this test stays green
	// through the exact regression it exists to catch (measured: it did).
	if len(block.sha) != len(bottleTags) {
		t.Fatalf("the formula rendered from release-artifacts.sh's own manifest declares %d bottle tags, want %d: %v",
			len(block.sha), len(bottleTags), block.order)
	}
	for _, tag := range bottleTags {
		if _, ok := block.sha[tag]; !ok {
			t.Errorf("the formula declares no bottle for %s, which release-artifacts.sh built", tag)
		}
	}
	for tag, sha := range block.sha {
		name := "posse-" + version + "." + tag + ".bottle.tar.gz"
		if !strings.Contains(string(sums), sha+"  "+name+"\n") {
			t.Errorf("the formula's %s digest %s is not the digest of %s in checksums.txt", tag, sha, name)
		}
	}
}

// A bottle is poured straight into the Cellar, so its contents ARE the
// install. If it disagrees with the formula's `def install`, a poured user and
// a source-built user get different files and nothing says so. The two live in
// different scripts, which is exactly why this is asserted rather than assumed.
func TestBottleContentsAreExactlyWhatTheFormulaInstalls(t *testing.T) {
	const version = "9.9.9"
	dist := releaseFixture(t, version)

	body, err := exec.Command("sh", "scripts/tap-formula.sh",
		"--version", "v"+version, "--checksums", filepath.Join(dist, "checksums.txt")).Output()
	if err != nil {
		t.Fatalf("tap-formula.sh: %v", err)
	}

	// What `def install` declares, read out of the formula rather than
	// restated here — restating it would pass over the two of them drifting.
	var bins, docs []string
	for _, line := range strings.Split(string(body), "\n") {
		s := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(s, "bin.install "):
			bins = quotedList(strings.TrimPrefix(s, "bin.install "))
		case strings.HasPrefix(s, "doc.install "):
			docs = quotedList(strings.TrimPrefix(s, "doc.install "))
		}
	}
	if len(bins) == 0 {
		t.Fatal("the formula's def install declares no bin.install — nothing would be pinned below")
	}
	if len(docs) == 0 {
		t.Fatal("the formula's def install declares no doc.install — nothing would be pinned below")
	}

	want := map[string]bool{}
	for _, b := range bins {
		want["posse/"+version+"/bin/"+b] = true
	}
	for _, d := range docs {
		want["posse/"+version+"/share/doc/posse/"+d] = true
	}

	for _, tag := range bottleTags {
		path := filepath.Join(dist, "posse-"+version+"."+tag+".bottle.tar.gz")
		got := map[string]bool{}
		for _, m := range tarMembers(t, path) {
			// Directories are structure, not content; brew creates them either
			// way, and `def install` never names one.
			if strings.HasSuffix(m, "/") {
				continue
			}
			got[m] = true
		}
		for w := range want {
			if !got[w] {
				t.Errorf("%s bottle is missing %s — `def install` puts it there, so a poured install would be short a file a source install has",
					tag, w)
			}
		}
		for g := range got {
			// A keg with MORE than the formula installs is the same defect
			// pointing the other way: LICENSE ships in the tarball and the
			// formula does not install it, so it must not be in the bottle.
			if !want[g] {
				t.Errorf("%s bottle carries %s, which `def install` does not install — a poured keg must not differ from a source-built one", tag, g)
			}
		}
	}
}

func quotedList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if v := quoted(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// the release and the instrument
// ---------------------------------------------------------------------------

// Built bottles that never reach the release are bottles that do not exist.
// The upload glob is separate from the build, so it is asserted separately.
func TestReleaseWorkflowShipsTheBottlesBesideTheTarballs(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	wf := string(b)
	for _, want := range []string{
		"dist/posse_*.tar.gz",
		"dist/posse-*.bottle.tar.gz",
		"dist/checksums.txt",
		"dist/posse.rb",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("release.yml does not upload %s — a release missing it makes `brew install` build from source, which is fatal on a Mac with stale Command Line Tools", want)
		}
	}
	// The tarball glob must not be the thing that happens to match the
	// bottles: `posse_*` and `posse-*` are different names on purpose.
	if strings.Contains(wf, "dist/posse*.tar.gz") {
		t.Error("release.yml uses the merged glob dist/posse*.tar.gz; keep the two explicit so a rename of either is visible")
	}
}

// The probe is the only thing that actually installs a bottle, and its control
// arm is the only thing that makes a green run mean something on a box that
// would have installed either way. Both are pinned here because a probe whose
// control quietly disappears reports success for the wrong reason.
func TestMacosInstallProbeBottleModeIsWiredAndControlled(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("scripts", "macos-install-probe.sh"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"paths|quarantine|tap|brew|bottle|all)", // the mode is accepted
		"bottle) probe_bottle ;;",               // and dispatched
		"probe_bottle()",                        // and defined
		"probe_brew; probe_bottle ;;",           // `all` runs it
		"Command Line Tools are too outdated",   // the control's verdict
		"CONTROL:",                              // the control reports as one
		"scripts/release-artifacts.sh",          // it builds the real artifacts
		"scripts/tap-formula.sh",                // from the real generator
	} {
		if !strings.Contains(src, want) {
			t.Errorf("scripts/macos-install-probe.sh no longer contains %q", want)
		}
	}
	// The published-tap probe must ask whether brew POURED, not merely whether
	// it installed: on a box with current developer tools the two look the same
	// and a tap that lost its bottles would regress in silence.
	if !strings.Contains(src, "Pouring posse-") {
		t.Error("the probe no longer checks that brew poured a bottle; on a box with current developer tools a bottle-less formula installs fine and the regression is invisible")
	}
	// --help must reach the end of the header block. A range that stops short
	// hides the option it is documenting.
	if !strings.Contains(src, "sed -n '2,69p'") {
		t.Error("the --help range moved; re-check that it still ends at the last header line and not inside the code")
	}
}
