package posse

// QA pins for ranger-base-poj5.
//
// Claim: codex is pinned at 0.150.1 by the Homebrew cask plus
// check_for_update_on_startup, declared in etc/codex/version-pin.toml and
// asserted by scripts/verify-codex-pin.sh, which also prints the re-audit
// list when the tap moves past the pin and still exits 0 (the pin is
// holding; lifting it is the operator's).
//
// The shape differs from grokpin_qa_test.go because the mechanism does.
// codex has NO version-ceiling config key — required_maximum_version,
// maximum_version, minimum_version and auto_update each appear zero times in
// the 0.150.1 binary against a positive control — so there is no hard bound
// to assert and the declaration says so out loud instead of quietly omitting
// it. TestQACodexPinDeclaresNoHardCeiling is that pin: the day someone
// "completes" the file by copying grok's keys into it, this fails.
//
// Everything below is hermetic: codex and brew are stubbed, so `make test`
// needs neither the operator's ~/.codex nor Homebrew nor a network.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const cpPinnedVer = "0.150.1"

// cpBox is one stubbed machine: a Homebrew prefix with a Caskroom, a codex
// on PATH that resolves into it, and a ~/.codex/config.toml.
type cpBox struct {
	root     string // the checkout copy (scripts/ + etc/)
	prefix   string // brew --prefix
	bin      string // the PATH dir
	codexHom string // CODEX_HOME
}

func cpRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"scripts", filepath.Join("etc", "codex")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []struct {
		src, dst string
		mode     os.FileMode
	}{
		{"scripts/verify-codex-pin.sh", filepath.Join("scripts", "verify-codex-pin.sh"), 0o755},
		{"etc/codex/version-pin.toml", filepath.Join("etc", "codex", "version-pin.toml"), 0o644},
	} {
		b, err := os.ReadFile(f.src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, f.dst), b, f.mode); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// cpNewBox builds the HAPPY machine. Every failing case below is this box
// with exactly one thing changed, so a red row names the thing that changed
// and nothing else.
func cpNewBox(t *testing.T) *cpBox {
	t.Helper()
	b := &cpBox{root: cpRoot(t), prefix: t.TempDir(), bin: t.TempDir(), codexHom: t.TempDir()}
	b.installCodex(t, cpPinnedVer, true)
	b.brew(t, "pinned", cpPinnedVer, 0)
	b.config(t, "check_for_update_on_startup = false\n")
	return b
}

// installCodex writes the stub into the Caskroom tree for ver and, when
// linked, symlinks it onto PATH the way the cask's `binary` stanza does.
// linked=false is the "something else won PATH" arm: npm, bun, pnpm and the
// standalone installer are all separate codex update channels, and a pin
// asserted against a binary nothing runs is not a pin.
func (b *cpBox) installCodex(t *testing.T, ver string, linked bool) {
	t.Helper()
	room := filepath.Join(b.prefix, "Caskroom", "codex", ver, "bin")
	if err := os.MkdirAll(room, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/bash\nif [ \"$1\" = \"--version\" ]; then echo \"codex-cli " + ver + "\"; exit 0; fi\nexit 99\n"
	real := filepath.Join(room, "codex")
	if err := os.WriteFile(real, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	onPath := filepath.Join(b.bin, "codex")
	os.Remove(onPath)
	if linked {
		if err := os.Symlink(real, onPath); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.WriteFile(onPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// brew stubs the three subcommands the script asks: --prefix, list --pinned,
// and outdated. outdatedRC is its own knob because a brew that FAILED must
// not read as "nothing is outdated" — that would switch the re-audit gate off
// in silence, which is the failure ranger-base-phxj found in the grok twin.
func (b *cpBox) brew(t *testing.T, pinState, tap string, outdatedRC int) {
	t.Helper()
	pinned := ""
	if pinState == "pinned" {
		pinned = "codex"
	}
	line := ""
	if tap != cpPinnedVer {
		line = "codex (" + cpPinnedVer + ") != " + tap + " [pinned at " + cpPinnedVer + "]"
	}
	body := "#!/bin/bash\n" +
		"case \"$1\" in\n" +
		"  --prefix) echo '" + b.prefix + "'; exit 0 ;;\n" +
		"  list) echo '" + pinned + "'; exit 0 ;;\n" +
		"  outdated) [ -n '" + line + "' ] && echo '" + line + "'; exit " + strconv.Itoa(outdatedRC) + " ;;\n" +
		"esac\nexit 99\n"
	if err := os.WriteFile(filepath.Join(b.bin, "brew"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (b *cpBox) config(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(b.codexHom, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (b *cpBox) run(t *testing.T) (string, int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(b.root, "scripts", "verify-codex-pin.sh"))
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CODEX_HOME=") || strings.HasPrefix(kv, "PATH=") || strings.HasPrefix(kv, "HOME=") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env,
		"PATH="+b.bin+string(os.PathListSeparator)+"/usr/bin:/bin",
		"CODEX_HOME="+b.codexHom,
		"HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("verify-codex-pin.sh: %v\n%s", err, out)
		}
	}
	return string(out), code
}

// cpRowFailed: did THIS row fail, on its own line? A bare Contains(out,
// "FAIL") is satisfied by any other failing row, so once a box can fail more
// than one check the whole-output form stops naming which.
func cpRowFailed(out, label string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, label) && strings.Contains(line, "FAIL") {
			return true
		}
	}
	return false
}

func TestQACodexPinDeclarationAndMakefile(t *testing.T) {
	body, err := os.ReadFile("etc/codex/version-pin.toml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	// Anchored to line start for the same reason the grok pin's rows are:
	// every one of these keys is also discussed in the file's prose, and a
	// bare Contains would be satisfied by the explanation of a key that had
	// been deleted.
	for _, want := range []string{
		"\nposse_pinned_version = \"" + cpPinnedVer + "\"",
		"\nformula = \"codex\"",
		"\npin_state = \"pinned\"",
		"\ncheck_for_update_on_startup = false",
		"\ncaskroom_dir = \"Caskroom/codex/" + cpPinnedVer + "\"",
		"\nsha256 = ",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("etc/codex/version-pin.toml missing %q", want)
		}
	}

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "verify-codex-pin:\n\tscripts/verify-codex-pin.sh\n") {
		t.Error("Makefile lost the verify-codex-pin target")
	}
	if !strings.Contains(string(mk), ".PHONY:") || !strings.Contains(string(mk), "verify-codex-pin ") {
		t.Error("verify-codex-pin is not in .PHONY")
	}

	info, err := os.Stat("scripts/verify-codex-pin.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/verify-codex-pin.sh is not executable")
	}
}

// The declaration must not grow a hard ceiling it cannot have. codex carries
// no such config key — measured on the 0.150.1 binary against a positive
// control — so a required_maximum_version here would be a pin that refuses
// nothing while reading, to anyone scanning the file, exactly like grok's
// braces. The accepted risk is stated instead, in both the file and every run
// of the script.
func TestQACodexPinDeclaresNoHardCeiling(t *testing.T) {
	body, err := os.ReadFile("etc/codex/version-pin.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, refuse := range []string{
		"\nrequired_maximum_version =",
		"\nmaximum_version =",
		"\nminimum_version =",
		"\nauto_update =",
	} {
		if strings.Contains(string(body), refuse) {
			t.Errorf("version-pin.toml must not set %q — codex has no such key (ranger-base-poj5)", refuse)
		}
	}
	if !strings.Contains(string(body), "ACCEPTED RISK") {
		t.Error("version-pin.toml must state the accepted risk: a codex above the pin still STARTS")
	}

	sh, err := os.ReadFile("scripts/verify-codex-pin.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sh), "ACCEPTED RISK") {
		t.Error("verify-codex-pin.sh must print the accepted risk on every run, not only in a comment")
	}
}

func TestQACodexPinHappyBox(t *testing.T) {
	b := cpNewBox(t)
	out, code := b.run(t)
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "pin intact at "+cpPinnedVer) {
		t.Errorf("missing intact line:\n%s", out)
	}
	if strings.Contains(out, "UPSTREAM MOVED") {
		t.Errorf("tap == pin must not print the re-audit list:\n%s", out)
	}
	if !strings.Contains(out, "ACCEPTED RISK") {
		t.Errorf("a green run must still say codex has no hard ceiling:\n%s", out)
	}
}

// The gate: the tap moving is NOT a failure — the pin is holding, and the
// script's job is to say what must be re-audited before the operator lifts it.
func TestQACodexPinUpstreamMovedStillPasses(t *testing.T) {
	b := cpNewBox(t)
	b.brew(t, "pinned", "0.151.0", 0)
	out, code := b.run(t)
	if code != 0 {
		t.Fatalf("upstream moving is not a failure: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "UPSTREAM MOVED: the codex cask is 0.151.0") {
		t.Errorf("missing the re-audit block:\n%s", out)
	}
	for _, want := range []string{
		"FETCH THE ROLLBACK ARTIFACT FIRST", // a cask keeps one version
		"check_for_update_on_startup",       // the key can be renamed upstream
		"verify-detection",                  // the blocked-screen rules
		"posse runtime check codex",         // the launch-line flags
	} {
		if !strings.Contains(out, want) {
			t.Errorf("re-audit list lost %q:\n%s", want, out)
		}
	}
}

// Every row fails on its own, and only its own. Written as a table because
// the interesting property is the ISOLATION: one broken thing must red one
// row, or a green run stops being evidence about the other four.
func TestQACodexPinEachRowFailsAlone(t *testing.T) {
	// `also` is the coupling, written down rather than worked around: an
	// upgrade moves the version AND the directory the binary resolves from,
	// so demanding one red row there would only be satisfiable by a break
	// that cannot happen on a real box. Every row not named here must stay
	// green, which is the property this test exists for.
	cases := []struct {
		name   string
		break_ func(t *testing.T, b *cpBox)
		row    string
		also   []string
	}{
		{"binary moved off the pin", func(t *testing.T, b *cpBox) {
			b.installCodex(t, "0.151.0", true)
		}, "codex --version", []string{"codex resolves into the pin"}},
		{"cask unpinned", func(t *testing.T, b *cpBox) {
			b.brew(t, "unpinned", cpPinnedVer, 0)
		}, "brew cask pin", nil},
		{"startup check back on", func(t *testing.T, b *cpBox) {
			b.config(t, "check_for_update_on_startup = true\n")
		}, "config check_for_update", nil},
		{"startup check absent", func(t *testing.T, b *cpBox) {
			b.config(t, "model = \"gpt-5.6-sol\"\n")
		}, "config check_for_update", nil},
		// `brew cleanup` after an upgrade takes the old version tree. The
		// binary is put on PATH directly here so the break is ONLY the missing
		// rollback target: removing the tree under a linked codex would take
		// the binary with it, and the script would exit 2 having measured
		// nothing at all.
		{"rollback target cleaned away", func(t *testing.T, b *cpBox) {
			b.installCodex(t, cpPinnedVer, false)
			os.RemoveAll(filepath.Join(b.prefix, "Caskroom", "codex", cpPinnedVer))
		}, "rollback target on disk", nil},
		{"another channel won PATH", func(t *testing.T, b *cpBox) {
			b.installCodex(t, cpPinnedVer, false)
		}, "codex resolves into the pin", nil},
		{"brew could not answer", func(t *testing.T, b *cpBox) {
			b.brew(t, "pinned", cpPinnedVer, 2)
		}, "tap version", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := cpNewBox(t)
			tc.break_(t, b)
			out, code := b.run(t)
			if code != 1 {
				t.Fatalf("exit %d, want 1\n%s", code, out)
			}
			if !cpRowFailed(out, tc.row) {
				t.Errorf("row %q did not fail:\n%s", tc.row, out)
			}
			allowed := map[string]bool{tc.row: true}
			for _, a := range tc.also {
				allowed[a] = true
				if !cpRowFailed(out, a) {
					t.Errorf("breaking %q was declared to also fail %q, and did not — the coupling is stale:\n%s", tc.name, a, out)
				}
			}
			for _, other := range cases {
				if allowed[other.row] {
					continue
				}
				if cpRowFailed(out, other.row) {
					t.Errorf("breaking %q also failed row %q — the rows are not independent:\n%s", tc.name, other.row, out)
				}
			}
		})
	}
}

// A comment is not a setting. Both files carry the key in prose; the
// extractor is anchored at ^ so neither can be read as the live value.
func TestQACodexPinIgnoresCommentedKeys(t *testing.T) {
	b := cpNewBox(t)
	b.config(t, "# check_for_update_on_startup = true\ncheck_for_update_on_startup = false\n")
	out, code := b.run(t)
	if code != 0 {
		t.Fatalf("a commented key must not be read as the value: exit %d\n%s", code, out)
	}
	b.config(t, "# check_for_update_on_startup = false\ncheck_for_update_on_startup = true\n")
	out, code = b.run(t)
	if code != 1 || !cpRowFailed(out, "config check_for_update") {
		t.Errorf("a commented FALSE must not rescue a live TRUE: exit %d\n%s", code, out)
	}
}

// Exit 2 is "nothing was measured", and it is not exit 1: a box with no codex
// on it has not failed the pin, and a CI run that reported it as a failure
// would train the fleet to ignore the red.
func TestQACodexPinExitsTwoWhenNothingToMeasure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		break_ func(t *testing.T, b *cpBox)
		want   string
	}{
		{"no codex", func(t *testing.T, b *cpBox) { os.Remove(filepath.Join(b.bin, "codex")) }, "codex not on PATH"},
		{"no brew", func(t *testing.T, b *cpBox) { os.Remove(filepath.Join(b.bin, "brew")) }, "brew not on PATH"},
		{"no config", func(t *testing.T, b *cpBox) { os.Remove(filepath.Join(b.codexHom, "config.toml")) }, "missing"},
		{"no declaration", func(t *testing.T, b *cpBox) {
			os.Remove(filepath.Join(b.root, "etc", "codex", "version-pin.toml"))
		}, "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := cpNewBox(t)
			tc.break_(t, b)
			out, code := b.run(t)
			if code != 2 {
				t.Fatalf("exit %d, want 2 (nothing measured)\n%s", code, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("missing %q:\n%s", tc.want, out)
			}
		})
	}
}
