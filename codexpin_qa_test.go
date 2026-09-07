package posse

// QA pins for ranger-base-poj5.
//
// Claim: codex is pinned at 0.153.4 by the Homebrew cask plus
// check_for_update_on_startup, declared in etc/codex/version-pin.toml and
// asserted by scripts/verify-codex-pin.sh, which also prints the re-audit
// list when the tap moves past the pin and still exits 0 (the pin is
// holding; lifting it is the operator's).
//
// The shape differs from grokpin_qa_test.go because the mechanism does.
// codex has NO version-ceiling config key — required_maximum_version,
// maximum_version, minimum_version and auto_update each appear zero times in
// the binary against a positive control (measured on 0.150.1, re-measured on
// 0.153.4 when the pin moved: control 27 -> 28, the four keys still 0) — so there is no hard bound
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

// The declared pin. Moving it means moving etc/codex/version-pin.toml with
// it — TestQACodexPinDeclarationAndMakefile asserts the file names this exact
// version in two places — and that is the point: the constant is the one
// place the two are joined.
const cpPinnedVer = "0.153.4"

// A tap version ABOVE the pin, for the arms that exercise the re-audit block.
// It has to sort above cpPinnedVer or those arms measure nothing: the gate is
// ver_gt(tap, pin), so a stale literal left below a raised pin turns every
// "UPSTREAM MOVED" assertion into a silent no-op rather than a failure. It is
// a stub value in a hermetic box, not a version anyone has run.
const cpPastVer = "0.199.0"

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
	b.brew(t, "pinned", cpPinnedVer, cpPinnedVer, 0)
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
// and info --cask. infoRC is its own knob because a brew that FAILED must not
// read as "nothing to re-audit" — that would switch the re-audit gate off in
// silence, which is the failure ranger-base-phxj found in the grok twin.
//
// `installed` is a parameter and not cpPinnedVer because the two versions
// coming apart is the whole subject of ranger-base-k4lza. The header shape
// changes with them, measured on Homebrew 6.0.22:
//
//	installed == tap    ==> codex (Codex): 0.153.4
//	installed <  tap    ==> codex (Codex): 0.150.1 → 0.151.0
//
// The read this stubs used to be `brew outdated`, which prints NOTHING in
// the first shape whatever the tap is — so a box already upgraded past the
// pin got the pin's own version echoed back as the tap. A stub that always
// spoke the second shape could not have shown that, which is why this one
// renders whichever shape the pair calls for. It draws the arrow whenever the
// two differ, where real brew draws it only when installed is BEHIND; no arm
// here sets installed above the tap, so that shape is unmeasured either way.
func (b *cpBox) brew(t *testing.T, pinState, installed, tap string, infoRC int) {
	t.Helper()
	header := "==> codex (Codex): " + tap
	if installed != tap {
		header = "==> codex (Codex): " + installed + " \u2192 " + tap
	}
	b.brewRaw(t, pinState, header, infoRC)
}

// brewRaw is brew with the info header written out, for the arms where the
// header is the thing under test rather than the version pair in it.
func (b *cpBox) brewRaw(t *testing.T, pinState, header string, infoRC int) {
	t.Helper()
	pinned := ""
	if pinState == "pinned" {
		pinned = "codex"
	}
	body := "#!/bin/bash\n" +
		"case \"$1\" in\n" +
		"  --prefix) echo '" + b.prefix + "'; exit 0 ;;\n" +
		"  list) echo '" + pinned + "'; exit 0 ;;\n" +
		"  info) echo '" + header + "'; echo 'https://github.com/openai/codex'; exit " + strconv.Itoa(infoRC) + " ;;\n" +
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

// declarePin rewrites one line of the box's own copy of version-pin.toml —
// old must appear exactly once, the way the pin's own extractor reads a
// single first-match line, so a typo in a test fixture fails loudly instead
// of silently declaring nothing.
func (b *cpBox) declarePin(t *testing.T, old, new string) {
	t.Helper()
	p := filepath.Join(b.root, "etc", "codex", "version-pin.toml")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if n := strings.Count(s, old); n != 1 {
		t.Fatalf("declarePin: %q appears %d times in version-pin.toml, want 1", old, n)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(s, old, new, 1)), 0o644); err != nil {
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
// no such config key — measured on the 0.150.1 binary and again on 0.153.4,
// each against a positive control — so a required_maximum_version here would
// be a pin that refuses
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
	b.brew(t, "pinned", cpPinnedVer, cpPastVer, 0)
	out, code := b.run(t)
	if code != 0 {
		t.Fatalf("upstream moving is not a failure: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "UPSTREAM MOVED: the codex cask is "+cpPastVer) {
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

// cpRow: the VALUE column of the row labelled `label` — the first field after
// the label, not the rest of the line, which also carries the row's status
// word (`ok` / `read`) and any `<-- FAIL (...)` tail. An assertion can then
// name the number the row reported rather than only whether it said FAIL.
func cpRow(out, label string) string {
	for _, line := range strings.Split(out, "\n") {
		i := strings.Index(line, label)
		if i < 0 {
			continue
		}
		f := strings.Fields(line[i+len(label):])
		if len(f) == 0 {
			return ""
		}
		return f[0]
	}
	return ""
}

// The box of ranger-base-k4lza, and the reason the tap row stopped being read
// out of `brew outdated`: the cask was upgraded past the pin with nothing
// pinning it, so the INSTALLED version caught up to the tap and both sat
// three minor versions above the declaration. `brew outdated` says nothing
// about a cask that is not behind, so the old read got an empty answer and
// its `|| upstream=$want_ver` fallback filled in the pin itself — the run
// printed the pin's own version as the tap and "== the pin; nothing to
// re-audit", suppressing the entire re-audit list at exactly the moment the
// operator needed it. The version row failing is not a substitute: it says
// the box moved, not what it must be re-audited against.
func TestQACodexPinTapReadWhenTheBoxIsAlreadyPastThePin(t *testing.T) {
	const past = cpPastVer
	b := cpNewBox(t)
	b.installCodex(t, past, true)
	if err := os.RemoveAll(filepath.Join(b.prefix, "Caskroom", "codex", cpPinnedVer)); err != nil {
		t.Fatal(err)
	}
	b.brew(t, "unpinned", past, past, 0) // installed == tap, both above the pin

	out, code := b.run(t)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if got := cpRow(out, "tap version"); got != past {
		t.Errorf("tap version row = %q, want %q — brew was not asked for the tap\n%s", got, past, out)
	}
	if !strings.Contains(out, "UPSTREAM MOVED: the codex cask is "+past) {
		t.Errorf("a tap %s above the pin must print the re-audit list:\n%s", past, out)
	}
	if strings.Contains(out, "nothing to re-audit") {
		t.Errorf("the run called a tap %s past the pin %q settled:\n%s", past, cpPinnedVer, out)
	}
}

// The prose the arm above prints but does not read (ranger-base-9ycqa
// finding 1). The block was written for a box the tap had moved AHEAD of,
// and the gate that reaches it asks only about the tap; ranger-base-k4lza
// unsuppressed it on a box that had ALREADY moved, where "the pin is holding
// — nothing has changed on this machine" is false and "the moment 0.153.4
// lands, brew cleanup deletes the only 0.150.1 copy" is an instruction about
// a thing that has already happened. Those two versions are quoted from the
// run of 2026-09-06, when the declaration still said 0.150.1; the pin has
// since moved to 0.153.4 (ranger-base-femsg) and the arms below read the
// declared version out of cpPinnedVer, not out of that sentence. Two rows above say so: the version row
// FAILS and the rollback target reads GONE.
//
// The two arms are each other's control. The same tap read, the same gate,
// and the only difference is which version the box RUNS — so a block that
// went back to printing one paragraph for both boxes reds one of them
// whichever paragraph it chose.
func TestQACodexPinReAuditTextSaysWhetherTheBoxItselfHasMoved(t *testing.T) {
	const past = cpPastVer

	t.Run("the box is still at the pin", func(t *testing.T) {
		b := cpNewBox(t)
		b.brew(t, "pinned", cpPinnedVer, past, 0) // only the TAP moved
		out, code := b.run(t)
		if code != 0 {
			t.Fatalf("exit %d, want 0 — a tap above a holding pin is not a failure\n%s", code, out)
		}
		for _, want := range []string{
			"UPSTREAM MOVED: the codex cask is " + past,
			"The pin is holding",
			"FETCH THE ROLLBACK ARTIFACT FIRST",
			// Read as ONE line: the sentence wraps after "The moment", and
			// a Contains over the pair spans a newline and never matches
			// — which is a vacuous assertion in the positive arm and a
			// vacuous refusal in the negative one below.
			past + " lands, `brew cleanup` deletes the only " + cpPinnedVer + " copy",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("a box still at %s must be told %q:\n%s", cpPinnedVer, want, out)
			}
		}
		if strings.Contains(out, "THE PIN IS NOT HOLDING") {
			t.Errorf("a box whose binary is still %s was told the pin is not holding:\n%s", cpPinnedVer, out)
		}
	})

	// The box of the finding: installed == tap == 0.153.4 and `brew cleanup`
	// has taken the rollback target. Every sentence asserted here was
	// printed, wrong, by the run of 2026-09-06.
	t.Run("the box has already moved and the rollback target is gone", func(t *testing.T) {
		b := cpNewBox(t)
		b.installCodex(t, past, true)
		if err := os.RemoveAll(filepath.Join(b.prefix, "Caskroom", "codex", cpPinnedVer)); err != nil {
			t.Fatal(err)
		}
		b.brew(t, "unpinned", past, past, 0)
		out, code := b.run(t)
		if code != 1 {
			t.Fatalf("exit %d, want 1\n%s", code, out)
		}
		for _, no := range []string{
			"The pin is holding",
			"FETCH THE ROLLBACK ARTIFACT FIRST",
			past + " lands, `brew cleanup` deletes the only " + cpPinnedVer + " copy",
			"nothing has changed on this machine",
		} {
			if strings.Contains(out, no) {
				t.Errorf("a box already running %s was told %q:\n%s", past, no, out)
			}
		}
		for _, want := range []string{
			"THE PIN IS NOT HOLDING",
			"THE PIN IS NOT HOLDING: this box runs " + past,
			"roll back to " + cpPinnedVer + ", or to re-audit what is installed",
			"THE ROLLBACK ARTIFACT IS ALREADY GONE",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("a box already running %s must be told %q:\n%s", past, want, out)
			}
		}
		if strings.Contains(out, "THE ROLLBACK ARTIFACT IS STILL HERE") {
			t.Errorf("a cleaned-away rollback target was reported as still fetchable:\n%s", out)
		}
		// Items 2-4 are the re-audit and they are the same work on either
		// box. A branch that dropped them on this arm would leave the
		// operator with a verdict and no list.
		for _, want := range []string{"2. The dispatch contract", "3. The startup-update key",
			"4. Interstitial detection", "Runbook: docs/notes.d/ranger-base-poj5.md"} {
			if !strings.Contains(out, want) {
				t.Errorf("the re-audit list lost %q on the moved-box arm:\n%s", want, out)
			}
		}
	})

	// The other half of item 1: the box moved but `brew cleanup` has not run
	// yet, so the last copy of the PINNED version is still on disk and saying
	// it is gone would send the operator to re-fetch something they are
	// standing on.
	t.Run("the box has already moved and the rollback target survives", func(t *testing.T) {
		b := cpNewBox(t) // leaves Caskroom/codex/<cpPinnedVer> in place
		b.installCodex(t, past, true)
		b.brew(t, "unpinned", past, past, 0)
		out, code := b.run(t)
		if code != 1 {
			t.Fatalf("exit %d, want 1\n%s", code, out)
		}
		if !strings.Contains(out, "THE ROLLBACK ARTIFACT IS STILL HERE") {
			t.Errorf("a rollback target still on disk was not offered:\n%s", out)
		}
		if strings.Contains(out, "THE ROLLBACK ARTIFACT IS ALREADY GONE") {
			t.Errorf("a rollback target still on disk was called gone:\n%s", out)
		}
		if strings.Contains(out, "The pin is holding") {
			t.Errorf("a box already running %s was told the pin is holding:\n%s", past, out)
		}
	})
}

// The other half of removing the fallback: with nothing to fall back TO, both
// ways of not getting an answer have to fail the row. A non-zero brew is the
// arm in the table below; this is the one that exits 0 and answers a header
// with no version in it, which no fallback now rescues into a green "read".
func TestQACodexPinUnreadableTapFailsTheRow(t *testing.T) {
	b := cpNewBox(t)
	b.brewRaw(t, "pinned", "==> codex (Codex): latest", 0)
	out, code := b.run(t)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !cpRowFailed(out, "tap version") {
		t.Errorf("a header with no version must fail the tap row:\n%s", out)
	}
	if strings.Contains(out, "UPSTREAM MOVED") || strings.Contains(out, "nothing to re-audit") {
		t.Errorf("an unread tap must claim nothing about upstream:\n%s", out)
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
			b.brew(t, "unpinned", cpPinnedVer, cpPinnedVer, 0)
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
			b.brew(t, "pinned", cpPinnedVer, cpPinnedVer, 2)
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

// ranger-base-g29az finding 3: caskroom_dir compared as a PREFIX of the
// resolved binary's directory let a version-less declaration
// ("Caskroom/codex") pass both rollback rows — Homebrew never removes that
// parent, and whichever version IS installed sits underneath it, so the row
// answered "yes" about a directory that names no version to roll back to.
// The version-ful wrong arm (a stale version that is actually gone) is
// TestQACodexPinEachRowFailsAlone's "rollback target cleaned away" — this
// test is the sibling wrong arm that survived it.
func TestQACodexPinRollbackTargetMustNameTheVersion(t *testing.T) {
	b := cpNewBox(t)
	b.declarePin(t, `caskroom_dir = "Caskroom/codex/`+cpPinnedVer+`"`, `caskroom_dir = "Caskroom/codex"`)
	out, code := b.run(t)
	if code != 1 {
		t.Fatalf("a caskroom_dir naming no version must fail: exit %d\n%s", code, out)
	}
	if !cpRowFailed(out, "rollback target on disk") {
		t.Errorf("rollback target row did not fail against a version-less caskroom_dir:\n%s", out)
	}
	// Not a FAIL of its own — same shape as the "rollback target gone" arm:
	// nothing to resolve into is said, not measured wrong.
	if !strings.Contains(out, "codex resolves into the pin") || cpRowFailed(out, "codex resolves into the pin") {
		t.Errorf("codex-resolves row must say nothing to resolve into, not FAIL:\n%s", out)
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
