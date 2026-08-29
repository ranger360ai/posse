package posse

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ranger-base-hza. The clean room is Linux by construction, so the macOS
// install routes were written and never run until this bead ran them. These
// pin the four findings that survived, and the instrument that produced them.
// Findings and what they deliberately do not cover:
// docs/runbooks/macos-install-routes.md.

// The generator must not emit a `version` stanza. brew scans the version out
// of the url, and an explicit one beside it is the single thing `brew audit
// --strict` rejects in this formula — on all four os/arch pairs, measured both
// ways on Homebrew 6.0.20. The positive witness matters as much as the
// absence: dropping the stanza is only safe because every url still carries
// the version for brew to scan, so an assertion of pure absence would pass
// over a formula that had lost both.
func TestTapFormulaOmitsTheRedundantVersionStanza(t *testing.T) {
	dir := t.TempDir()
	checksums := writeChecksums(t, dir, "0.3.0", "")
	out, err := exec.Command("sh", "scripts/tap-formula.sh",
		"--version", "v0.3.0", "--checksums", checksums).Output()
	if err != nil {
		t.Fatalf("tap-formula.sh: %v", err)
	}
	rendered := string(out)

	if line, ok := versionStanza(rendered); ok {
		t.Fatalf("the generated formula carries %q; brew audit --strict rejects it as "+
			"redundant with the version scanned from the URL", line)
	}
	// The witness: brew can only scan what the urls carry. `root_url` is the
	// bottle block's (ranger-base-9vg3) and is deliberately excluded — brew
	// scans the version from the SOURCE urls, and counting root_url here would
	// both inflate the count and demand a version segment that stanza has no
	// reason to carry.
	urls := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "url \"") && !strings.Contains(line, "root_url \"") {
			urls++
			if !strings.Contains(line, "/v0.3.0/") || !strings.Contains(line, "_0.3.0_") {
				t.Fatalf("url has no scannable version, so dropping the stanza loses it: %s", strings.TrimSpace(line))
			}
		}
	}
	if urls != 4 {
		t.Fatalf("expected 4 urls (darwin/linux x arm64/amd64), got %d", urls)
	}

	// The wrong arm. Without this, a predicate that never matches anything
	// passes over a formula that reintroduced the stanza tomorrow.
	t.Run("a formula that carries the stanza is caught", func(t *testing.T) {
		regressed := strings.Replace(rendered,
			"  license \"Apache-2.0\"", "  version \"0.3.0\"\n  license \"Apache-2.0\"", 1)
		if regressed == rendered {
			t.Fatal("could not build the regressed fixture — the license line moved")
		}
		if _, ok := versionStanza(regressed); !ok {
			t.Fatal("the stanza was reintroduced and the check did not see it")
		}
	})
}

// A top-level `version "X"`, which is what brew audits, and not the `version`
// inside the test block (`version.to_s`) or the word in a comment.
func versionStanza(formula string) (string, bool) {
	for _, line := range strings.Split(formula, "\n") {
		if strings.HasPrefix(line, "  version \"") {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

// Each finding that cost a measurement to get is named by one token no other
// paragraph would carry, and each is checked with its own deletion arm — a
// page-wide Contains over prose is satisfied by any sentence that happens to
// mention the word.
func TestInstallMdCarriesTheMeasuredMacOSFindings(t *testing.T) {
	contents, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	page := string(contents)

	findings := []struct {
		name  string
		token string
		why   string
	}{
		{
			name:  "brew shellenv is a prerequisite of step 2's Verify",
			token: `eval "$(/opt/homebrew/bin/brew shellenv)"`,
			why: "on Apple Silicon /opt/homebrew/bin is not on the default PATH, so brew install " +
				"succeeds and posse is still command not found — ranger-base-253's shape on the route " +
				"advertised as avoiding it",
		},
		{
			name:  "a browser-downloaded binary is quarantined and hangs",
			token: "xattr -d com.apple.quarantine",
			why: "neither curl nor brew sets com.apple.quarantine, but a browser does, and an " +
				"ad-hoc-signed binary that carries it blocks with no output and no exit code",
		},
		{
			name:  "the error a pre-bottle tap still produces is named and explained",
			token: "Your Command Line Tools are too outdated",
			why: "brew takes its build-from-source path for a formula with no bottle, so an outdated " +
				"CLT refuses the install before our formula is ever read. ranger-base-9vg3 ships " +
				"bottles and the page now says the error means an older tap — but the string has to " +
				"stay, because it is what someone on one will paste into a search box",
		},
		{
			name:  "a poured install is what the page tells the reader to look for",
			token: "Pouring posse-<version>.<tag>.bottle.tar.gz",
			why: "ranger-base-9vg3: pouring a bottle is the whole difference between an install that " +
				"needs a toolchain and one that does not, and it is the one line on screen that " +
				"distinguishes them",
		},
		{
			name:  "tap-info reports tap trust, not formula trust",
			token: "it reports *tap* trust",
			why: "the narrow formula grant never flips tap-info, so a reader who checks it concludes " +
				"the trust line failed",
		},
	}

	for _, f := range findings {
		t.Run(f.name, func(t *testing.T) {
			if !strings.Contains(page, f.token) {
				t.Fatalf("INSTALL.md no longer carries %q.\n%s\nMeasured on ranger-base-hza; "+
					"re-run scripts/macos-install-probe.sh before deciding it stopped being true.", f.token, f.why)
			}
			// Uniqueness is what makes the check above discriminating: if the
			// token appeared twice, deleting the paragraph it was written for
			// would leave the other copy and this test green over the loss.
			if strings.Contains(strings.Replace(page, f.token, "", 1), f.token) {
				t.Fatalf("%q appears more than once, so deleting its paragraph would not fail this pin", f.token)
			}
		})
	}
}

// "Applied, verified by running it, captured in versioned config." The
// measurements are only worth something if the thing that made them is in the
// tree and reachable the way the clean room is.
func TestMacosInstallProbeIsVersionedAndWired(t *testing.T) {
	info, err := os.Stat("scripts/macos-install-probe.sh")
	if err != nil {
		t.Fatalf("the instrument that produced docs/runbooks/macos-install-routes.md is missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("scripts/macos-install-probe.sh is not executable; the Makefile target invokes it directly")
	}

	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"macos-install-probe:", "scripts/macos-install-probe.sh all"} {
		if !strings.Contains(string(makefile), want) {
			t.Fatalf("Makefile has no %q — an instrument nobody can run is a one-off, not a control", want)
		}
	}
	phony := ""
	for _, line := range strings.Split(string(makefile), "\n") {
		if strings.HasPrefix(line, ".PHONY:") {
			phony = line
			break
		}
	}
	if phony == "" {
		t.Fatal("the Makefile has no .PHONY line")
	}
	if !strings.Contains(phony, "macos-install-probe") {
		t.Fatal(".PHONY does not list macos-install-probe; a file of that name would shadow the target")
	}

	if _, err := os.Stat("docs/runbooks/macos-install-routes.md"); err != nil {
		t.Fatalf("the runbook the probe's findings live in is missing: %v", err)
	}
}

// Exit 2 is "nothing was measured", and it is not a pass. Same convention as
// verify-credential-paths.sh. These arms run on every platform because
// argument parsing happens before the Darwin check.
func TestMacosInstallProbeRefusesWhatItCannotMeasure(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "no mode named", args: nil},
		{name: "unknown mode", args: []string{"everything"}},
		{name: "unknown flag", args: []string{"paths", "--force"}},
		{name: "a version that is not a tag", args: []string{"paths", "--version", "0.3.0"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("bash", append([]string{"scripts/macos-install-probe.sh"}, tt.args...)...)
			err := cmd.Run()
			code := exitCodeOf(t, err)
			if code != 2 {
				t.Fatalf("expected exit 2 (nothing measured), got %d", code)
			}
		})
	}
	t.Run("a named mode is accepted", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("off darwin every mode exits 2, so this arm cannot discriminate here")
		}
		cmd := exec.Command("bash", "scripts/macos-install-probe.sh", "paths")
		out, err := cmd.CombinedOutput()
		if code := exitCodeOf(t, err); code == 2 {
			t.Fatalf("the paths probe measured nothing on darwin:\n%s", out)
		}
	})
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("not an exit error: %v", err)
	}
	return ee.ExitCode()
}

// The two rows of the zsh startup-file table that INSTALL.md §1 now rests on,
// measured rather than quoted. `~/.zshrc` is the file the page names, and
// `~/.zshenv` is the one that reads like the thorough choice and is the trap:
// macOS runs path_helper from /etc/zprofile, after .zshenv and before .zshrc,
// and it prepends the system paths — so a .zshenv export is still found at
// login and is no longer first, which is the ranger-base-253 ambiguity rather
// than a fix for it.
//
// This runs on ci.yml's macos-latest row. It depends on zsh's startup order
// and on /etc/zprofile, not on the runner's own PATH, so it does not measure
// the box it happens to be on.
func TestZshStartupFileCarriesThePathExportInstallMdNames(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the zsh/path_helper interaction this pins is macOS-only")
	}
	if _, err := os.Stat("/etc/zprofile"); err != nil {
		t.Skipf("no /etc/zprofile, so path_helper does not run and the table has a different shape: %v", err)
	}

	cases := []struct {
		file  string
		flags []string
		want  string
		why   string
	}{
		{file: ".zshrc", flags: []string{"-l", "-i"}, want: "first",
			why: "INSTALL.md §1 tells a macOS reader to put the export in ~/.zshrc; a login+interactive shell is what they are typing into"},
		{file: ".zshrc", flags: []string{"-i"}, want: "first",
			why: "a non-login interactive shell (some editors' terminals) must find it too"},
		{file: ".zshenv", flags: []string{"-l", "-i"}, want: "demoted",
			why: "the trap INSTALL.md §1 now warns about: path_helper runs after .zshenv and prepends the system paths"},
	}
	for _, tt := range cases {
		t.Run(tt.file+strings.Join(tt.flags, ""), func(t *testing.T) {
			got := zshPathProbe(t, tt.file, tt.flags)
			if got != tt.want {
				t.Fatalf("an export in %s from a %v shell is %q, expected %q.\n%s",
					tt.file, tt.flags, got, tt.want, tt.why)
			}
		})
	}

	// The control. Without it, a box where the fixture binary simply never
	// resolves would report "not-found" for every row and this test would be
	// measuring nothing while looking selective.
	t.Run("control: no export anywhere is not-found", func(t *testing.T) {
		if got := zshPathProbe(t, "", []string{"-l", "-i"}); got != "not-found" {
			t.Fatalf("with no export written at all the probe said %q; it is not measuring the export", got)
		}
	})
}

// Reports first | demoted | not-found for an export written into one zsh
// startup file, in an isolated HOME/ZDOTDIR. An empty file name writes no
// export at all, which is the control.
func zshPathProbe(t *testing.T, file string, flags []string) string {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "posse"), []byte("#!/bin/sh\necho HIT\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if file != "" {
		export := "export PATH=\"$HOME/.local/bin:$PATH\"\n"
		if err := os.WriteFile(filepath.Join(home, file), []byte(export), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	const script = `command -v posse >/dev/null || { print not-found; exit }
case $PATH in $HOME/.local/bin:*) print first ;; *) print demoted ;; esac`
	cmd := exec.Command("/bin/zsh", append(append([]string{}, flags...), "-c", script)...)
	cmd.Env = []string{"HOME=" + home, "ZDOTDIR=" + home, "TERM=dumb"}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("zsh %v: %v", flags, err)
	}
	return strings.TrimSpace(string(out))
}
