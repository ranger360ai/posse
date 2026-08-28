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
