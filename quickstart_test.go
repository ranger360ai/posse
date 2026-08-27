package posse

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ranger-base-253: go install succeeds without making posse discoverable on a
// default PATH. Whenever a public quickstart advertises that route, keep the
// corrective export between installation and first use.
//
// ranger-base-4ash: judged per ROUTE, not per file. The first cut of this pin
// took the FIRST `go install` line and searched the whole rest of the file, so
// a second block appended later — the shape a "from source" or "on Linux"
// section arrives in — inherited the first block's export and stayed green
// while advertising the original defect verbatim.
const (
	goInstallCmd = "go install github.com/ranger360ai/posse/cmd/posse@latest"
	goBinExport  = `export PATH="$(go env GOPATH)/bin:$PATH"`
	goInstallUse = "posse init"
)

func TestGoInstallQuickstartsAddGoBinToPathBeforeInit(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		required bool
	}{
		{name: "landing page", path: "www/index.html"},
		{name: "README", path: "README.md", required: true},

		// INSTALL.md is deliberately absent: §3 ("From outside a checkout the
		// same binary installs with") advertises the route with no export
		// anywhere in the file, and the next thing it tells the reader is that
		// `posse init` works — measured exit 0 then exit 127, ranger-base-4ash,
		// filed to dinesh. Add the row in the same commit as the doc fix; the
		// logic below already rejects that shape, pinned as a fixture case in
		// TestGoInstallPathPinRejectsTheHistoricalGaps.
		//
		// etc/cleanroom/README.md must NEVER be added. It is a transcript OF
		// the failure — an export line there would destroy the evidence.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			advertised, err := goInstallExportsGoBin(string(contents))
			if !advertised {
				if tt.required {
					t.Fatalf("%s: missing %q", tt.path, goInstallCmd)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", tt.path, err)
			}
		})
	}
}

// The pin is only as strong as what it rejects. Case one is the shape that
// shipped and produced ranger-base-253; the last two are the shapes that
// survived the first cut of the pin.
func TestGoInstallPathPinRejectsTheHistoricalGaps(t *testing.T) {
	canonical := goInstallCmd + "\n" + goBinExport + "\n" + goInstallUse + "\n"

	if advertised, err := goInstallExportsGoBin(canonical); !advertised || err != nil {
		t.Fatalf("canonical three-line form: advertised=%v err=%v", advertised, err)
	}
	if advertised, err := goInstallExportsGoBin("make install\nposse init\n"); advertised || err != nil {
		t.Fatalf("a surface that does not advertise the route must skip, not fail; advertised=%v err=%v", advertised, err)
	}

	cases := []struct {
		name string
		text string
	}{
		{
			name: "the shape that shipped (ranger-base-253)",
			text: goInstallCmd + "\n" + goInstallUse + "\n",
		},
		{
			name: "export after first use",
			text: goInstallCmd + "\n" + goInstallUse + "\n" + goBinExport + "\n",
		},
		{
			name: "export of the wrong directory",
			text: goInstallCmd + "\nexport PATH=\"$HOME/.local/bin:$PATH\"\n" + goInstallUse + "\n",
		},
		{
			name: "a second route inherits the first one's export (ranger-base-4ash)",
			text: canonical + "\n## Also available\n\n" + goInstallCmd + "\n" + goInstallUse + "\n",
		},
		{
			name: "INSTALL.md §3: the route, then prose promising init works (ranger-base-4ash)",
			text: "```sh\n$ " + goInstallCmd + "\n```\n\nThat build carries the seed tree (`examples/`) embedded, so `" +
				goInstallUse + "` works with no repo beside it.\n",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			advertised, err := goInstallExportsGoBin(tt.text)
			if !advertised {
				t.Fatal("fixture advertises the route; it must be judged, not skipped")
			}
			if err == nil {
				t.Fatal("historical gap passed the pin")
			}
		})
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("README with the export line deleted", func(t *testing.T) {
		stripped := strings.Replace(string(readme), goBinExport, "", 1)
		advertised, err := goInstallExportsGoBin(stripped)
		if !advertised || err == nil {
			t.Fatalf("deleting the export from README must fail the pin; advertised=%v err=%v", advertised, err)
		}
	})
}

// goInstallExportsGoBin judges EVERY go-install route a surface advertises.
// Each occurrence of the install command opens a window that ends at the next
// occurrence, so two blocks in one file cannot borrow each other's export.
// Plain substring search, not line anchoring: the landing page carries its
// commands inside <span> markup, and the panel is the surface that broke.
func goInstallExportsGoBin(text string) (advertised bool, firstErr error) {
	for off := 0; ; {
		at := strings.Index(text[off:], goInstallCmd)
		if at < 0 {
			return advertised, firstErr
		}
		advertised = true
		off += at + len(goInstallCmd)

		window := text[off:]
		if next := strings.Index(window, goInstallCmd); next >= 0 {
			window = window[:next]
		}
		if err := goInstallRouteExportsGoBin(window); err != nil && firstErr == nil {
			firstErr = err
		}
	}
}

// goInstallRouteExportsGoBin judges one route: the text between its install
// command and the next one.
func goInstallRouteExportsGoBin(window string) error {
	exportAt := strings.Index(window, goBinExport)
	useAt := strings.Index(window, goInstallUse)
	if exportAt < 0 {
		return fmt.Errorf("%q is not followed by %q", goInstallCmd, goBinExport)
	}
	if useAt < 0 {
		return fmt.Errorf("%q is not followed by %q", goInstallCmd, goInstallUse)
	}
	if exportAt > useAt {
		return fmt.Errorf("%q must appear before %q", goBinExport, goInstallUse)
	}
	return nil
}

// ranger-base-g2u / ranger-base-4mg: brew install of a third-party formula
// without the trust line is the PATH-line failure on a different axis — the
// command is a no-op grant on some brew versions (full name is itself the
// trust) and a hard refusal on others, and either way a stranger's machine
// is the first witness. Same table shape as the go-install pin: IF a surface
// advertises the install, the predecessors have to precede it, in order.
// Whole-tap trust is not a substitute — that grant covers future formulae.
func TestBrewInstallQuickstartsCarryTapAndTrustBeforeInstall(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		required bool
	}{
		{name: "landing page", path: "www/index.html", required: true},
		{name: "README", path: "README.md"},
		{name: "INSTALL.md", path: "INSTALL.md", required: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			advertised, err := advertisedBrewInstallSequence(string(contents))
			if !advertised {
				if tt.required {
					t.Fatalf("%s: missing %q", tt.path, brewInstallCmd)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", tt.path, err)
			}
		})
	}
}

// The pin above is only as strong as what it rejects. These are the shapes
// that shipped (or would ship) while the suite stayed green.
func TestBrewInstallSequenceRejectsTheHistoricalGaps(t *testing.T) {
	canonical := brewTapCmd + "\n" + brewTrustCmd + "\n" + brewInstallCmd
	if advertised, err := advertisedBrewInstallSequence(canonical); !advertised || err != nil {
		t.Fatalf("canonical three-command form: advertised=%v err=%v", advertised, err)
	}
	if advertised, err := advertisedBrewInstallSequence("make install\nposse init\n"); advertised || err != nil {
		t.Fatalf("a surface that does not advertise brew install must skip, not fail; advertised=%v err=%v", advertised, err)
	}

	cases := []struct {
		name string
		text string
	}{
		{name: "install with no trust (ranger-base-4mg)", text: brewTapCmd + "\n" + brewInstallCmd},
		{name: "trust after install", text: brewTapCmd + "\n" + brewInstallCmd + "\n" + brewTrustCmd},
		{name: "install with no tap", text: brewTrustCmd + "\n" + brewInstallCmd},
		{name: "tap after trust", text: brewTrustCmd + "\n" + brewTapCmd + "\n" + brewInstallCmd},
		{name: "whole-tap trust is the wrong grant", text: brewTapCmd + "\nbrew trust ranger360ai/tap\n" + brewInstallCmd},
		{name: "trust without --formula", text: brewTapCmd + "\nbrew trust ranger360ai/tap/posse\n" + brewInstallCmd},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			advertised, err := advertisedBrewInstallSequence(tt.text)
			if !advertised {
				t.Fatal("fixture advertises brew install; sequence must be judged, not skipped")
			}
			if err == nil {
				t.Fatal("historical gap passed the pin")
			}
		})
	}

	index, err := os.ReadFile("www/index.html")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("landing page with trust line deleted", func(t *testing.T) {
		stripped := strings.ReplaceAll(string(index), brewTrustCmd, "")
		advertised, err := advertisedBrewInstallSequence(stripped)
		if !advertised || err == nil {
			t.Fatalf("deleting the trust line from the landing page must fail the pin; advertised=%v err=%v", advertised, err)
		}
	})
	t.Run("landing page with whole-tap trust", func(t *testing.T) {
		whole := strings.ReplaceAll(string(index), brewTrustCmd, "brew trust ranger360ai/tap")
		advertised, err := advertisedBrewInstallSequence(whole)
		if !advertised || err == nil {
			t.Fatalf("replacing --formula with whole-tap trust must fail the pin; advertised=%v err=%v", advertised, err)
		}
	})
}

// ranger-base-g2u / ranger-base-l9y: a bare brew trust on a landing-page
// panel is the shape of a supply-chain lure. The panel has to link into
// INSTALL.md §2 (the explanation) rather than paraphrase it. Pin presence
// and target, not the link text — that copy sits on a 35-char budget and
// will move. Pin the heading too: a fragment that does not match §2 dumps
// the reader at the top of a 400-line file while the test stays green.
func TestLandingPageBrewPanelLinksToInstallStep2(t *testing.T) {
	index, err := os.ReadFile("www/index.html")
	if err != nil {
		t.Fatal(err)
	}
	install, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := githubHeadingAnchor("2. Get the harness and build it"), "2-get-the-harness-and-build-it"; got != want {
		t.Fatalf("githubHeadingAnchor(%q)=%q, want %q (GitHub's heading id)", "2. Get the harness and build it", got, want)
	}
	if err := landingPageBrewPanelLink(string(index), string(install)); err != nil {
		t.Fatal(err)
	}
}

func TestLandingPageBrewPanelLinkRejectsAMissingOrUnanchoredHref(t *testing.T) {
	headingMD := "## 2. Get the harness and build it\n\n## 3. Next\n"
	href := `https://github.com/ranger360ai/posse/blob/main/INSTALL.md#2-get-the-harness-and-build-it`
	panel := func(inner string) string {
		return `<pre class="terminal">` + inner + `</pre>`
	}
	canonicalInner := brewTapCmd + "\n" +
		`<a href="` + href + `">why</a>` + "\n" +
		brewTrustCmd + "\n" + brewInstallCmd + "\n"

	if err := landingPageBrewPanelLink(panel(canonicalInner), headingMD); err != nil {
		t.Fatalf("canonical panel: %v", err)
	}

	cases := []struct {
		name    string
		index   string
		install string
	}{
		{
			name:    "no link in the panel",
			index:   panel(brewTapCmd + "\n" + brewTrustCmd + "\n" + brewInstallCmd),
			install: headingMD,
		},
		{
			name:    "link is in the caption, not the panel",
			index:   panel(brewTapCmd+"\n"+brewTrustCmd+"\n"+brewInstallCmd) + `<p class="terminal-caption"><a href="` + href + `">why</a></p>`,
			install: headingMD,
		},
		{
			name:    "href to INSTALL.md with no fragment",
			index:   panel(brewTapCmd + "\n" + `<a href="https://github.com/ranger360ai/posse/blob/main/INSTALL.md">why</a>` + "\n" + brewTrustCmd + "\n" + brewInstallCmd),
			install: headingMD,
		},
		{
			name:    "fragment is §1, not §2",
			index:   panel(brewTapCmd + "\n" + `<a href="https://github.com/ranger360ai/posse/blob/main/INSTALL.md#1-prerequisites">why</a>` + "\n" + brewTrustCmd + "\n" + brewInstallCmd),
			install: headingMD,
		},
		{
			name:    "URL as comment text, not an href",
			index:   panel(brewTapCmd + "\n# " + href + "\n" + brewTrustCmd + "\n" + brewInstallCmd),
			install: headingMD,
		},
		{
			name:    "INSTALL.md heading no longer matches the fragment",
			index:   panel(canonicalInner),
			install: "## 2. Install the binary\n\n## 3. Next\n",
		},
		{
			name:    "panel no longer advertises brew",
			index:   panel("go install github.com/ranger360ai/posse/cmd/posse@latest\n"),
			install: headingMD,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := landingPageBrewPanelLink(tt.index, tt.install); err == nil {
				t.Fatal("gap passed the pin")
			}
		})
	}

	index, err := os.ReadFile("www/index.html")
	if err != nil {
		t.Fatal(err)
	}
	install, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("real panel with href stripped", func(t *testing.T) {
		panelHTML, err := terminalPanel(string(index))
		if err != nil {
			t.Fatal(err)
		}
		stripped := strings.Replace(string(index), panelHTML, strings.ReplaceAll(panelHTML, `href="`, `data-x="`), 1)
		if err := landingPageBrewPanelLink(stripped, string(install)); err == nil {
			t.Fatal("stripping href from the real panel must fail the pin")
		}
	})
}

// ranger-base-88m: `make install` writes $(BINDIR)/posse — ~/.local/bin by
// default, which is on no default macOS or Linux PATH, and which the install
// itself may have just created (Debian's skel .profile prepends it only when it
// already existed at login). The install exits 0 and the very next command the
// README advertises is `posse: command not found`. Same defect as
// ranger-base-253 on the go-install route, different destination: an install
// that succeeds without making the binary reachable.
//
// Two halves, pinned separately below: every surface that advertises `make
// install` carries the export before the first use of the installed binary, and
// the install target itself says so when the shell cannot find what it wrote.
func TestMakeInstallQuickstartsAddLocalBinToPathBeforeInit(t *testing.T) {
	for _, path := range []string{"README.md", "INSTALL.md"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := makeInstallExportsLocalBin(string(contents)); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

// The pin is only as strong as what it rejects. Case one is the shape that
// shipped and produced the bead.
func TestMakeInstallPathPinRejectsTheHistoricalGaps(t *testing.T) {
	const export = `export PATH="$HOME/.local/bin:$PATH"`
	cases := []struct {
		name string
		text string
		want bool // want an error
	}{
		{
			name: "the shape that shipped (ranger-base-88m)",
			text: "make build\nmake install\nposse init\n",
			want: true,
		},
		{
			name: "export after first use",
			text: "make install\nposse init\n" + export + "\n",
			want: true,
		},
		{
			name: "export of the wrong directory",
			text: "make install\nexport PATH=\"$(go env GOPATH)/bin:$PATH\"\nposse init\n",
			want: true,
		},
		{
			name: "prose about PATH is not an export",
			text: "make install\nit lands in ~/.local/bin, put that on your PATH\nposse init\n",
			want: true,
		},
		{
			name: "canonical",
			text: "make install\n" + export + "\nposse init\n",
		},
		{
			name: "INSTALL.md's $-prefixed form",
			text: "$ make install\n$ " + export + "\n$ posse version\n",
		},
		{
			name: "a surface that does not advertise make install is not judged",
			text: "brew install ranger360ai/tap/posse\nposse init\n",
		},
		{
			name: "prose mentions are not commands",
			text: "running `make install` from a checkout puts `posse init` within reach\n",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := makeInstallExportsLocalBin(tt.text)
			if tt.want && err == nil {
				t.Fatal("gap passed the pin")
			}
			if !tt.want && err != nil {
				t.Fatalf("canonical form rejected: %v", err)
			}
		})
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("README with the export line deleted", func(t *testing.T) {
		stripped := strings.Replace(string(readme), export, "", 1)
		if makeInstallExportsLocalBin(stripped) == nil {
			t.Fatal("deleting the export from README must fail the pin")
		}
	})
}

// makeInstallExportsLocalBin reports whether a surface that advertises `make
// install` puts BINDIR on PATH before the first command that runs the binary it
// just installed. Only command lines count on either side: the docs discuss
// `make install` and `posse init` in prose constantly, and prose is not a
// sequence a reader types.
func makeInstallExportsLocalBin(text string) error {
	installAt := commandLineAt(text, "make install")
	if installAt < 0 {
		return nil // this surface does not advertise the make route
	}
	rest := text[installAt:]
	useAt := len(rest)
	for _, use := range []string{"posse init", "posse version", "posse new", "make link-plugin"} {
		if at := commandLineAt(rest, use); at >= 0 && at < useAt {
			useAt = at
		}
	}
	if useAt == len(rest) {
		return fmt.Errorf("%q is not followed by any command that runs the installed binary", "make install")
	}
	window := rest[:useAt]
	exportAt := commandLineAt(window, "export PATH=")
	if exportAt < 0 || !strings.Contains(lineAt(window, exportAt), ".local/bin") {
		return fmt.Errorf("%q is not followed by an export putting ~/.local/bin on PATH before %q",
			"make install", strings.TrimSpace(lineAt(rest, useAt)))
	}
	return nil
}

// commandLineAt finds the offset of the first line the reader would TYPE that
// begins with cmd — bare, or with INSTALL.md's `$ ` prompt. A backticked
// mention mid-sentence is prose and does not count.
func commandLineAt(text, cmd string) int {
	for off := 0; off < len(text); {
		end := strings.IndexByte(text[off:], '\n')
		if end < 0 {
			end = len(text) - off
		}
		line := strings.TrimSpace(text[off : off+end])
		line = strings.TrimPrefix(line, "$ ")
		if strings.HasPrefix(line, cmd) {
			return off
		}
		off += end + 1
	}
	return -1
}

func lineAt(text string, off int) string {
	if off < 0 || off >= len(text) {
		return ""
	}
	line := text[off:]
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end]
	}
	return line
}

// The docs half above is advice; this is the half that reaches a reader who
// never read them. `make install` must say, on the spot, that the shell cannot
// find what it just wrote — and print the exact export, the way herdr's own
// installer does for the same directory.
func TestInstallWarnsWhenBindirIsNotOnPath(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(makefile), "\t@scripts/path-warning.sh '$(BINDIR)'\n") {
		t.Error("Makefile install no longer runs scripts/path-warning.sh with BINDIR — a promote that cannot be found is ranger-base-88m again")
	}
	info, err := os.Stat("scripts/path-warning.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/path-warning.sh is not executable")
	}

	bindir := filepath.Join(t.TempDir(), ".local", "bin")
	if err := os.MkdirAll(bindir, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(bindir, "posse")
	if err := os.WriteFile(installed, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "posse"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("bindir not on PATH", func(t *testing.T) {
		stdout, stderr := pathWarning(t, bindir, "/usr/bin:/bin")
		if stdout != "" {
			t.Errorf("stdout = %q, want the warning on stderr only", stdout)
		}
		if !strings.Contains(stderr, bindir+" is not in your PATH") {
			t.Errorf("stderr does not name the unreachable directory:\n%s", stderr)
		}
		// The exact line, copy-pasteable, with $PATH left for the shell to expand.
		if want := `export PATH="` + bindir + `:$PATH"`; !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr)
		}
	})

	t.Run("bindir on PATH is silent", func(t *testing.T) {
		stdout, stderr := pathWarning(t, bindir, bindir+":/usr/bin:/bin")
		if stdout != "" || stderr != "" {
			t.Errorf("a correct PATH must print nothing; stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("a different posse answers first (ranger-base-253)", func(t *testing.T) {
		_, stderr := pathWarning(t, bindir, other+":"+bindir+":/usr/bin:/bin")
		if !strings.Contains(stderr, "PATH resolves posse to "+filepath.Join(other, "posse")) {
			t.Errorf("stale posse earlier on PATH went unreported:\n%s", stderr)
		}
	})

	t.Run("a symlink to the installed binary is the installed binary", func(t *testing.T) {
		link := filepath.Join(other, "posse")
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(installed, link); err != nil {
			t.Fatal(err)
		}
		defer func() {
			os.Remove(link)
			os.WriteFile(link, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		}()
		if _, stderr := pathWarning(t, bindir, other+":/usr/bin:/bin"); stderr != "" {
			t.Errorf("a link to the promoted binary is not a stale posse: %s", stderr)
		}
	})
}

// pathWarning runs the warning with a PATH of the test's choosing. It must
// always exit 0: a PATH the installer cannot edit is not a failed build, and
// `make install` on a correctly-configured box has to stay green.
func pathWarning(t *testing.T, bindir, path string) (stdout, stderr string) {
	t.Helper()
	cmd := exec.Command("sh", "scripts/path-warning.sh", bindir)
	cmd.Env = []string{"PATH=" + path, "HOME=" + t.TempDir()}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("scripts/path-warning.sh %s (PATH=%s): %v\n%s", bindir, path, err, errb.String())
	}
	return out.String(), errb.String()
}

// ranger-base-5yl: the advertised posse new --dir ~/code/myproj died
// directory-not-found on a machine that had never created that path. `posse
// new` still refuses a missing dir on purpose — a typo must not silently
// become an empty workspace — so every surface that advertises the example
// path has to make it first.
func TestQuickstartsMkdirBeforeExampleNewDir(t *testing.T) {
	const newLine = "posse new myproj --dir ~/code/myproj"
	for _, path := range []string{"README.md", "www/index.html", "INSTALL.md"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		newAt := strings.Index(text, newLine)
		if newAt < 0 {
			continue
		}
		// Look behind the advertised new for a mkdir of that directory.
		windowStart := newAt - 400
		if windowStart < 0 {
			windowStart = 0
		}
		window := text[windowStart:newAt]
		if !strings.Contains(window, "mkdir") || !strings.Contains(window, "code/myproj") {
			t.Errorf("%s: %q is not preceded by mkdir of ~/code/myproj", path, newLine)
		}
	}
}

// ranger-base-m3a: README still describes @latest as an untagged pseudo-version
// after v0.3.0 exists and is what the public proxy serves.
func TestReadmeDoesNotClaimNoReleaseTag(t *testing.T) {
	t.Skip("ranger-base-m3a: README still says the module has no release tag")

	contents, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "no release tag yet") {
		t.Fatal("README.md still claims the module has no release tag; go install @latest installs v0.3.0")
	}
}

// ranger-base-4ex: INSTALL.md §2 advertised `brew install ranger360ai/tap/posse`
// while ranger360ai/homebrew-tap 404'd, and brew's error never said the tap
// was never created. The tap now exists; keep the three-command route (tap,
// trust one formula, install) and the diagnostic for when it does not.
func TestInstallMdStep2BrewRouteNamesTheTapAndTheSilentFailure(t *testing.T) {
	contents, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	start := strings.Index(text, "## 2.")
	end := strings.Index(text, "## 3.")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("INSTALL.md missing §2 / §3 headings")
	}
	step2 := text[start:end]

	tap := "brew tap ranger360ai/tap"
	trust := "brew trust --formula ranger360ai/tap/posse"
	install := "brew install ranger360ai/tap/posse"
	for _, want := range []string{tap, trust, install, "Error: Failure while executing tap"} {
		if !strings.Contains(step2, want) {
			t.Errorf("INSTALL.md §2 missing %q", want)
		}
	}
	if tapAt, trustAt, installAt := strings.Index(step2, tap), strings.Index(step2, trust), strings.Index(step2, install); tapAt < 0 || trustAt < 0 || installAt < 0 || !(tapAt < trustAt && trustAt < installAt) {
		t.Error("INSTALL.md §2 brew route is not tap, then trust --formula, then install")
	}
}

// ranger-base-n5i: INSTALL.md §1 pinned bd 0.49.1 and named herdr, but typed
// no install command for either — so its own Verify died command-not-found on
// a clean machine, and the runbook's "do not continue" rule stopped the
// install at step 1. The get-it trail was a URL for herdr and a bare
// destination for bd. Keep §1 carrying typed, copy-pasteable installs —
// herdr's install script, the pinned bd release tarball, and the PATH line
// for where both land — each before the Verify that needs them.
func TestInstallMdStep1TypesTheHerdrAndBdInstalls(t *testing.T) {
	contents, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	start := strings.Index(text, "## 1.")
	end := strings.Index(text, "## 2.")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("INSTALL.md missing §1 / §2 headings")
	}
	step1 := text[start:end]

	const verify = "go version && herdr --version && bd version && git --version"
	verifyAt := strings.Index(step1, verify)
	if verifyAt < 0 {
		t.Fatalf("INSTALL.md §1 missing the Verify line %q", verify)
	}
	for _, want := range []string{
		"curl -fsSL https://herdr.dev/install.sh | sh",
		"https://github.com/gastownhall/beads/releases/download/v0.49.1/beads_0.49.1_${os}_${arch}.tar.gz",
		`export PATH="$HOME/.local/bin:$PATH"`,
	} {
		at := strings.Index(step1, want)
		if at < 0 {
			t.Errorf("INSTALL.md §1 missing typed %q", want)
			continue
		}
		if at > verifyAt {
			t.Errorf("INSTALL.md §1: %q must precede the Verify that needs it", want)
		}
	}
}

// ranger-base-n5i: the beads repo moved — github.com/steveyegge/beads 301s to
// github.com/gastownhall/beads. A public surface that routes a stranger
// through the redirect is one upstream housecleaning away from a 404, and the
// bead pin (0.49.1 exactly) makes these the only download trail we offer.
func TestPublicSurfacesNameTheBeadsRepoPostRename(t *testing.T) {
	for _, path := range []string{"README.md", "INSTALL.md", "www/index.html"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// The bare name may appear as history ("formerly steveyegge/beads");
		// what must not appear is the linkable URL through the redirect.
		if strings.Contains(string(contents), "github.com/steveyegge/beads") {
			t.Errorf("%s still links the pre-rename beads repo (github.com/steveyegge/beads 301s; link gastownhall/beads)", path)
		}
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "github.com/gastownhall/beads") {
		t.Error("README.md no longer names the beads repo (github.com/gastownhall/beads)")
	}
}

// ranger-base-n5i: the landing page's only word on the two required tools was
// a caption reading "see the readme", and the readme's §1 pointer led to a
// section with no typed installs. Keep the caption linking straight to
// INSTALL.md §1, the section the test above holds to typed commands. Same
// heading-anchor lockstep as the §2 panel-link pin.
func TestLandingPageCaptionLinksHerdrAndBeadsToInstallStep1(t *testing.T) {
	index, err := os.ReadFile("www/index.html")
	if err != nil {
		t.Fatal(err)
	}
	install, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	heading := ""
	for _, line := range strings.Split(string(install), "\n") {
		if strings.HasPrefix(line, "## 1.") {
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			break
		}
	}
	if heading == "" {
		t.Fatal("INSTALL.md missing ## 1. heading")
	}
	anchor := githubHeadingAnchor(heading)
	if anchor != "1-prerequisites" {
		t.Fatalf("INSTALL.md §1 heading %q anchors to %q; the caption link and this pin expect %q", heading, anchor, "1-prerequisites")
	}

	captionAt := strings.Index(string(index), `class="terminal-caption"`)
	if captionAt < 0 {
		t.Fatal("www/index.html has no terminal-caption")
	}
	caption := string(index)[captionAt:]
	if end := strings.Index(caption, "</p>"); end >= 0 {
		caption = caption[:end]
	}
	for _, href := range hrefs(caption) {
		if strings.Contains(href, "github.com/ranger360ai/posse") &&
			strings.Contains(href, "INSTALL.md") &&
			strings.HasSuffix(href, "#"+anchor) {
			return
		}
	}
	t.Fatalf("terminal caption has no href to github.com/ranger360ai/posse … INSTALL.md#%s", anchor)
}

// ranger-base-4ex: `mktemp -d -t posse-release` is BSD-only. GNU coreutils
// dies "too few X's in template", which is how the release workflow would
// have failed on the tag after vet and test went green.
func TestReleaseScriptsUsePortableMktemp(t *testing.T) {
	scripts := []struct{ path, template string }{
		{path: "scripts/release-artifacts.sh", template: `${TMPDIR:-/tmp}/posse-release.XXXXXX`},
		{path: "scripts/clean-build.sh", template: `${TMPDIR:-/tmp}/posse-clean-build.XXXXXX`},
	}
	for _, s := range scripts {
		t.Run(s.path, func(t *testing.T) {
			contents, err := os.ReadFile(s.path)
			if err != nil {
				t.Fatal(err)
			}
			want := `mktemp -d "` + s.template + `"`
			if !strings.Contains(string(contents), want) {
				t.Errorf("%s: missing portable %q", s.path, want)
			}
			for _, line := range strings.Split(string(contents), "\n") {
				trim := strings.TrimSpace(line)
				if strings.HasPrefix(trim, "#") {
					continue
				}
				if strings.Contains(line, "mktemp") && strings.Contains(line, " -t ") && !strings.Contains(line, "XXXXXX") {
					t.Errorf("%s: live mktemp still uses BSD -t prefix: %s", s.path, trim)
				}
			}
		})
	}
}

// ranger-base-4ex: five in-repo references pointed at docs/runbooks/release.md
// when the file did not exist. The workflow's own closing output is one of them.
func TestReleaseRunbookExistsAtThePathFiveFilesCite(t *testing.T) {
	if _, err := os.Stat("docs/runbooks/release.md"); err != nil {
		t.Fatalf("docs/runbooks/release.md: %v", err)
	}
	const cite = "docs/runbooks/release.md"
	for _, path := range []string{
		"Makefile",
		"INSTALL.md",
		"scripts/tap-formula.sh",
		".github/workflows/release.yml",
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), cite) {
			t.Errorf("%s no longer cites %s", path, cite)
		}
	}
}

const (
	brewTapCmd     = "brew tap ranger360ai/tap"
	brewTrustCmd   = "brew trust --formula ranger360ai/tap/posse"
	brewInstallCmd = "brew install ranger360ai/tap/posse"
)

// advertisedBrewInstallSequence reports whether text advertises the brew
// install, and if it does, whether tap and formula-trust precede it in that
// order. advertised=false, err=nil is the optional-surface skip.
func advertisedBrewInstallSequence(text string) (bool, error) {
	installAt := strings.Index(text, brewInstallCmd)
	if installAt < 0 {
		return false, nil
	}
	before := text[:installAt]
	tapAt := strings.Index(before, brewTapCmd)
	trustAt := strings.Index(before, brewTrustCmd)
	if tapAt < 0 {
		return true, fmt.Errorf("%q is not preceded by %q", brewInstallCmd, brewTapCmd)
	}
	if trustAt < 0 {
		return true, fmt.Errorf("%q is not preceded by %q", brewInstallCmd, brewTrustCmd)
	}
	if tapAt > trustAt {
		return true, fmt.Errorf("%q must appear before %q", brewTapCmd, brewTrustCmd)
	}
	return true, nil
}

func terminalPanel(html string) (string, error) {
	const open = `<pre class="terminal">`
	start := strings.Index(html, open)
	if start < 0 {
		return "", fmt.Errorf(`missing %s`, open)
	}
	rest := html[start+len(open):]
	end := strings.Index(rest, "</pre>")
	if end < 0 {
		return "", fmt.Errorf("unclosed terminal panel")
	}
	return rest[:end], nil
}

func hrefs(html string) []string {
	var out []string
	for _, quote := range []string{`"`, `'`} {
		remain := html
		open := "href=" + quote
		for {
			i := strings.Index(remain, open)
			if i < 0 {
				break
			}
			remain = remain[i+len(open):]
			j := strings.Index(remain, quote)
			if j < 0 {
				break
			}
			out = append(out, remain[:j])
			remain = remain[j+1:]
		}
	}
	return out
}

func installStep2Heading(installMD string) (string, error) {
	for _, line := range strings.Split(installMD, "\n") {
		if strings.HasPrefix(line, "## 2.") {
			return strings.TrimSpace(strings.TrimPrefix(line, "## ")), nil
		}
	}
	return "", fmt.Errorf("INSTALL.md missing ## 2. heading")
}

// githubHeadingAnchor is GitHub's heading-id slug: lowercase, drop
// punctuation, spaces to hyphens. Kept in lockstep with the INSTALL.md §2
// fragment the panel links to — if this drifts from GitHub, the known-pair
// assertion in TestLandingPageBrewPanelLinksToInstallStep2 fails first.
func githubHeadingAnchor(heading string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-':
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func landingPageBrewPanelLink(index, installMD string) error {
	panel, err := terminalPanel(index)
	if err != nil {
		return err
	}
	if !strings.Contains(panel, brewInstallCmd) {
		return fmt.Errorf("terminal panel does not advertise %q", brewInstallCmd)
	}
	heading, err := installStep2Heading(installMD)
	if err != nil {
		return err
	}
	anchor := githubHeadingAnchor(heading)
	for _, href := range hrefs(panel) {
		if !strings.Contains(href, "github.com/ranger360ai/posse") {
			continue
		}
		if !strings.Contains(href, "INSTALL.md") {
			continue
		}
		hash := strings.LastIndex(href, "#")
		if hash < 0 {
			continue
		}
		if href[hash+1:] == anchor {
			return nil
		}
	}
	return fmt.Errorf("terminal panel has no href to github.com/ranger360ai/posse … INSTALL.md#%s (INSTALL.md heading %q)", anchor, heading)
}
