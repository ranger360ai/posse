package posse

// QA pins for rangerhq-i0n0's formula renderer, verified under rangerhq-0hzi.
//
// scripts/tap-formula.sh exists to make one class of mistake unavailable: a
// formula whose sha256 does not belong to the tarball its URL names. That
// failure installs fine on the machine that cut the release and fails
// `brew install` on exactly the architecture the releaser does not own, so no
// amount of local `brew install` can see it. Nothing pinned the mapping — the
// existing release pins (release_qa_test.go, quickstart_test.go) cover the
// workflow's checkout ref, the mktemp spelling and the runbook's citations,
// all of which stay green while `digest darwin arm64` is wired to the intel
// block.
//
// The runbook's own step-5 chain check does not close this either: its
// `grep -q "$h" checksums.txt` loop asks only whether each digest appears
// SOMEWHERE in the manifest, so two digests swapped between architectures
// print four OK lines (measured). What catches a swap there is the `diff
// posse.rb tap.rb` above it — which only works because the formula was
// generated. These pins are what make that generation trustworthy.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The four assets, each with a digest that names its own slot. Distinct
// values, so a renderer that reads the right file but writes the wrong line
// cannot pass by accident.
var tapFormulaFixture = []struct{ goos, goarch, digest string }{
	{"darwin", "arm64", "11111111111111111111111111111111111111111111111111111111aaaaaaaa"},
	{"darwin", "amd64", "22222222222222222222222222222222222222222222222222222222bbbbbbbb"},
	{"linux", "arm64", "33333333333333333333333333333333333333333333333333333333cccccccc"},
	{"linux", "amd64", "44444444444444444444444444444444444444444444444444444444dddddddd"},
}

// The four bottles (ranger-base-9vg3), each with a digest that names its own
// slot, distinct from every tarball digest above. A renderer that wired a
// tarball's sha into a bottle tag — or one bottle tag's sha into another —
// would install fine on the releaser's machine and pour a 404 or a checksum
// mismatch everywhere else.
var tapBottleFixture = []struct{ tag, digest string }{
	{"arm64_sonoma", "55555555555555555555555555555555555555555555555555555555eeeeeeee"},
	{"sonoma", "66666666666666666666666666666666666666666666666666666666ffffffff"},
	{"arm64_linux", "7777777777777777777777777777777777777777777777777777777799999999"},
	{"x86_64_linux", "8888888888888888888888888888888888888888888888888888888800000000"},
}

func writeChecksums(t *testing.T, dir, version string, skip string) string {
	t.Helper()
	var b strings.Builder
	for _, a := range tapFormulaFixture {
		name := "posse_" + version + "_" + a.goos + "_" + a.goarch + ".tar.gz"
		if name == skip {
			continue
		}
		b.WriteString(a.digest + "  " + name + "\n")
	}
	for _, a := range tapBottleFixture {
		name := "posse-" + version + "." + a.tag + ".bottle.tar.gz"
		if name == skip {
			continue
		}
		b.WriteString(a.digest + "  " + name + "\n")
	}
	path := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// renderedSlot is one (os block, arch block) pair as the formula declares it.
type renderedSlot struct{ osBlock, archBlock, url, sha string }

// parseFormula walks the Homebrew DSL the way brew does — by block, not by
// line order — so a url/sha256 pair moved into the wrong on_arm/on_intel
// block is visible rather than merely adjacent to the right digest.
func parseFormula(t *testing.T, body string) []renderedSlot {
	t.Helper()
	var slots []renderedSlot
	var osBlock, archBlock string
	var pending renderedSlot
	for _, line := range strings.Split(body, "\n") {
		s := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(s, "on_macos"):
			osBlock, archBlock = "darwin", ""
		case strings.HasPrefix(s, "on_linux"):
			osBlock, archBlock = "linux", ""
		case strings.HasPrefix(s, "on_arm"):
			archBlock = "arm64"
		case strings.HasPrefix(s, "on_intel"):
			archBlock = "amd64"
		case strings.HasPrefix(s, `url "`):
			pending = renderedSlot{osBlock: osBlock, archBlock: archBlock, url: quoted(s)}
		case strings.HasPrefix(s, `sha256 "`):
			pending.sha = quoted(s)
			slots = append(slots, pending)
			pending = renderedSlot{}
		}
	}
	return slots
}

func quoted(s string) string {
	i := strings.Index(s, `"`)
	j := strings.LastIndex(s, `"`)
	if i < 0 || j <= i {
		return ""
	}
	return s[i+1 : j]
}

func TestTapFormulaMapsEachDigestToItsOwnArchitecture(t *testing.T) {
	dir := t.TempDir()
	const version = "0.3.0"
	checksums := writeChecksums(t, dir, version, "")
	out := filepath.Join(dir, "posse.rb")

	cmd := exec.Command("sh", "scripts/tap-formula.sh",
		"--version", "v"+version, "--checksums", checksums,
		"--repo", "ranger360ai/posse", "--out", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tap-formula.sh: %v\n%s", err, b)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	slots := parseFormula(t, string(body))
	if len(slots) != len(tapFormulaFixture) {
		t.Fatalf("formula declares %d url/sha256 pairs, want %d — an architecture that is silently absent installs fine everywhere the author looked:\n%s",
			len(slots), len(tapFormulaFixture), body)
	}
	seen := map[string]bool{}
	for _, want := range tapFormulaFixture {
		asset := "posse_" + version + "_" + want.goos + "_" + want.goarch + ".tar.gz"
		found := false
		for _, got := range slots {
			if got.osBlock != want.goos || got.archBlock != want.goarch {
				continue
			}
			found = true
			seen[asset] = true
			if !strings.HasSuffix(got.url, "/"+asset) {
				t.Errorf("on_%s/on_%s names %q, want the block's own asset %s",
					want.goos, want.goarch, got.url, asset)
			}
			if !strings.Contains(got.url, "/releases/download/v"+version+"/") {
				t.Errorf("on_%s/on_%s url %q is not the tag's release-download path", want.goos, want.goarch, got.url)
			}
			if got.sha != want.digest {
				t.Errorf("on_%s/on_%s carries sha256 %s, want %s — that is %s's digest wired to another architecture's URL, which fails `brew install` only on the arch the releaser does not own",
					want.goos, want.goarch, got.sha, want.digest, assetOf(got.sha, version))
			}
		}
		if !found {
			t.Errorf("no on_%s/on_%s block in the rendered formula", want.goos, want.goarch)
		}
	}
	if len(seen) != len(tapFormulaFixture) {
		t.Errorf("formula covers %d of %d architectures", len(seen), len(tapFormulaFixture))
	}
}

// assetOf names whose digest a misplaced sha256 actually is, so the failure
// says "you have linux/amd64's hash here" rather than printing two hex blobs.
func assetOf(sha, version string) string {
	for _, a := range tapFormulaFixture {
		if a.digest == sha {
			return "posse_" + version + "_" + a.goos + "_" + a.goarch + ".tar.gz"
		}
	}
	return "no tarball in this release"
}

func TestTapFormulaRefusesAChecksumsFileMissingAnArchitecture(t *testing.T) {
	dir := t.TempDir()
	const version = "0.3.0"
	// The build that produced three tarballs instead of four. A formula
	// rendered from it would install everywhere the author tested.
	checksums := writeChecksums(t, dir, version, "posse_"+version+"_linux_arm64.tar.gz")
	out := filepath.Join(dir, "posse.rb")

	cmd := exec.Command("sh", "scripts/tap-formula.sh",
		"--version", "v"+version, "--checksums", checksums, "--out", out)
	b, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("tap-formula.sh rendered a formula from a checksums file with no linux/arm64 line:\n%s", b)
	}
	// `digest()` runs inside a command substitution, where `exit 1` leaves
	// only the subshell. The refusal is only real if that status reaches the
	// caller AND no partial formula is left behind for someone to commit.
	if !strings.Contains(string(b), "posse_"+version+"_linux_arm64.tar.gz is not in") {
		t.Errorf("refusal does not name the missing tarball:\n%s", b)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("a partial %s survived the refusal (stat err: %v)", out, err)
	}
}

// ranger-base-63q3. The formula must carry an explicit `version` stanza, and
// it must carry the tag's version.
//
// This reverses ranger-base-hza, which pinned the stanza ABSENT because
// `brew audit --strict` calls it "redundant with version scanned from URL".
// The scan is redundant only on a brew that has Homebrew commit bae7b0408a
// (2026-07-28, first tagged 6.0.14): before it, `Version.detect` has no
// releases/download UrlParser and falls through to the stem heuristic, which
// reads `64` out of `posse_0.4.0_darwin_arm64.tar.gz`. Measured against
// Homebrew's own version.rb at both tags, same URL:
//
//	6.0.13 -> 64        6.0.20 -> 0.4.0
//
// Before bottles that only mis-named the keg, because a source url is a
// literal string. The bottle block (ranger-base-9vg3) made the scanned string
// load-bearing: brew builds `<name>-<version>.<tag>.bottle.tar.gz` from the
// FORMULA's version, so a box that scanned `64` asks for
// posse-64.arm64_sonoma.bottle.tar.gz and `brew install` exits 1 on a curl
// 404 — which is how the published v0.4.0 failed on every brew from 5.x
// through 6.0.13. An explicit `@version` short-circuits the scan on those
// brews too (`Downloadable#version` in 6.0.13 returns it before it ever
// consults the url), which is why the stanza is the fix and not a mitigation.
func TestTapFormulaPinsTheVersionSoBrewNeedNotScanIt(t *testing.T) {
	dir := t.TempDir()
	// Two versions, because a generator that hard-coded one string would
	// satisfy any single-version assertion — and a stanza naming the wrong
	// version is worse than none: it names a bottle nobody uploaded.
	for _, version := range []string{"0.4.0", "9.9.9"} {
		t.Run(version, func(t *testing.T) {
			checksums := writeChecksums(t, dir, version, "")
			out, err := exec.Command("sh", "scripts/tap-formula.sh",
				"--version", "v"+version, "--checksums", checksums).Output()
			if err != nil {
				t.Fatalf("tap-formula.sh: %v", err)
			}
			rendered := string(out)

			line, ok := versionStanza(rendered)
			if !ok {
				t.Fatalf("the generated formula carries no `version` stanza, so brew scans the "+
					"version out of the url — and every brew before 6.0.14 scans `64`, then 404s on "+
					"posse-64.<tag>.bottle.tar.gz:\n%s", rendered)
			}
			if want := `version "` + version + `"`; line != want {
				t.Fatalf("the stanza is %q, want %q — brew builds the bottle filename from it, so a "+
					"wrong one 404s on every platform at once", line, want)
			}
			// audit's second complaint about the stanza, and the only one
			// that is ours to settle: Homebrew's component order puts
			// `version` before `license`.
			if v, l := strings.Index(rendered, "\n  version \""), strings.Index(rendered, "\n  license \""); v > l {
				t.Errorf("`version` is rendered after `license`; brew audit --strict wants it before")
			}
			// The urls must still carry the version too. They are what brew
			// scans on 6.0.14+, they are the fallback if this stanza is ever
			// dropped again, and `root_url` names the release the bottles
			// were actually uploaded to.
			urls := 0
			for _, u := range strings.Split(rendered, "\n") {
				if strings.Contains(u, "url \"") && !strings.Contains(u, "root_url \"") {
					urls++
					if !strings.Contains(u, "/v"+version+"/") || !strings.Contains(u, "_"+version+"_") {
						t.Errorf("url does not carry the version brew would scan: %s", strings.TrimSpace(u))
					}
				}
			}
			if urls != 4 {
				t.Fatalf("expected 4 urls (darwin/linux x arm64/amd64), got %d", urls)
			}
			if !strings.Contains(rendered, "root_url \"https://github.com/ranger360ai/posse/releases/download/v"+version+"\"") {
				t.Errorf("root_url does not name the v%s release, so the bottle url the stanza "+
					"builds would point at the wrong tag:\n%s", version, rendered)
			}

			// The wrong arms. Without them a predicate that never matches
			// anything passes over the formula that shipped the defect.
			t.Run("a formula with the stanza dropped is caught", func(t *testing.T) {
				regressed := strings.Replace(rendered, "  version \""+version+"\"\n", "", 1)
				if regressed == rendered {
					t.Fatal("could not build the regressed fixture")
				}
				if _, ok := versionStanza(regressed); ok {
					t.Fatal("the stanza was dropped and the check did not see it")
				}
			})
			t.Run("a stanza naming another version is caught", func(t *testing.T) {
				regressed := strings.Replace(rendered,
					"  version \""+version+"\"", "  version \"64\"", 1)
				got, ok := versionStanza(regressed)
				if !ok || got == line {
					t.Fatalf("the stanza was rewritten to the scanned garbage and the check did not see it (%q)", got)
				}
			})
		})
	}
}

// A top-level `version "X"`, which is what brew reads, and not the `version`
// inside the test block (`version.to_s`) or the word in a comment.
func versionStanza(formula string) (string, bool) {
	for _, line := range strings.Split(formula, "\n") {
		if strings.HasPrefix(line, "  version \"") {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}
