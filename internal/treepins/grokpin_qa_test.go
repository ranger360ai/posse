package treepins

// QA pins for rangerhq-y7jr (verified under rangerhq-n0bu).
//
// Claim: grok is pinned at 1.0.52 via [cli] auto_update = false and
// maximum_version, declared in etc/grok/version-pin.toml and asserted by
// scripts/verify-grok-pin.sh. The script is also the re-audit gate: when
// upstream stable moves past the pin it prints security's list and still
// exits 0 (the pin is holding; lifting it is the operator's).
//
// THE PIN GOVERNS THE HOMEBREW CASK `grok-build` from 2026-10-31 (operator
// ruling on ranger-base-yq1ar, applied under ranger-base-7qey3). It governed
// the `~/.grok` managed install before that; the box moved to the cask on
// 2026-10-12 and the ruling is to keep it. So the script now reads two
// sources that cannot stand in for each other — grok's own config, and brew —
// and the fixtures stub BOTH. `brew upgrade` is not grok's updater and obeys
// none of grok's config, which is the whole reason the brew rows exist.
//
// AND UPSTREAM IS READ FROM THE TAP, NOT FROM GROK. The arms below used to
// move upstream by moving grok's own `latestVersion`; that field is clamped to
// maximum_version, so it equals the pin whenever the pin is intact and could
// never have tripped the gate on a healthy box. MEASURED 2026-10-31
// (ranger-base-7qey3), one scratch $GROK_HOME, three `grok update --check
// --json` runs differing only in that key: no ceiling → "1.0.70"; ceiling
// 1.0.52 → "1.0.52"; ceiling 1.0.5 → "1.0.5", below the installed binary.
// So `latestVersion` is a CONTROL ON THE SOFT CEILING here — the one thing no
// reading of config.toml can establish, namely that the updater obeys it —
// and the tap version from `brew info --cask` drives UPSTREAM MOVED.
//
// Live grok 1.0.5 emits compact update JSON (`"autoUpdate":false` with no
// space). The hermetic cases below stub grok and brew so make test does not
// need the operator's ~/.grok, a Homebrew prefix, or a network.
//
// ranger-base-ocfh: a space after the colon, or a pretty-printed payload, is
// the SAME answer and must read the same. The old extractor captured nothing
// there and fell into the offline arm — config false, updater true, exit 0
// "pin intact". The pretty rows below are that pin, plus the arm it exposed:
// only an EMPTY payload is offline; a payload we cannot parse FAILs.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const gpCompactOK = `{"currentVersion":"1.0.52","latestVersion":"1.0.52","updateAvailable":false,"autoUpdate":false,"error":null}`

// The healthy fixture config, and since ranger-base-nhxdx it carries THREE
// required keys rather than two ceilings: ADR 0076 Amendment 3 (D12/D13) makes
// `[ui.notifications] program_status = "on"` a key posse requires in the
// operator's own config, because it is what makes grok write its own OSC 7501
// record on a `multiplexer: rex` instance. It is in the same file and asserted
// by the same script for the same reason the ceilings are — posse declares what
// it needs in that config and never writes it.
//
// Its own TABLE, and the row that reads it is section-aware (the script's
// `tomlpath`), so this fixture is also the shape that proves the two halves do
// not read each other's keys.
const gpGoodCfg = `[cli]
installer = "internal"
auto_update = false
maximum_version = "1.0.52"
required_maximum_version = "1.0.52"

[ui.notifications]
program_status = "on"
`

func gpRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc", "grok"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("scripts/verify-grok-pin.sh")
	if err != nil {
		t.Fatal(err)
	}
	pin, err := os.ReadFile("etc/grok/version-pin.toml")
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(filepath.Join(root, "scripts", "verify-grok-pin.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "grok", "version-pin.toml"), pin, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func gpStubGrok(t *testing.T, binDir, version, checkJSON string) {
	t.Helper()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var body string
	if checkJSON == "" {
		body = "#!/bin/sh\n" +
			"if [ \"$1\" = \"--version\" ]; then echo \"grok " + version + " (deadbeef) [stable]\"; exit 0; fi\n" +
			"if [ \"$1\" = \"update\" ]; then exit 0; fi\n" +
			"echo \"stub grok: $*\" >&2; exit 99\n"
	} else {
		body = "#!/bin/sh\n" +
			"if [ \"$1\" = \"--version\" ]; then echo \"grok " + version + " (deadbeef) [stable]\"; exit 0; fi\n" +
			"if [ \"$1\" = \"update\" ]; then\n" +
			"cat <<'JSON'\n" + checkJSON + "\nJSON\n" +
			"exit 0\nfi\n" +
			"echo \"stub grok: $*\" >&2; exit 99\n"
	}
	if err := WriteExecutable(filepath.Join(binDir, "grok"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// gpPinnedVer is what etc/grok/version-pin.toml declares. It is a literal
// here rather than a read of the pin file on purpose: a literal that disagrees
// with the declaration reds the arms below, which is how a half-done version
// move gets caught instead of absorbed.
const gpPinnedVer = "1.0.52"

// gpMovedVer is a tap ABOVE the pin, which is the shape that makes the
// re-audit block print. "1.1.0" is the literal the arms in this file already
// use for it; this names it so an assertion can compare against the same
// thing the stub was handed.
const gpMovedVer = "1.1.0"

// gpCask is the cask the pin governs, and gpBrewMarkerNoBrew is how an arm
// says "leave brew OFF this PATH" — see gpRun, which otherwise plants a
// healthy brew stub for every call site that does not ask for a sick one.
const (
	gpCask             = "grok-build"
	gpBrewMarkerNoBrew = ".gp-no-brew"
)

// gpBrew is what the brew stub answers. Every field's zero value is the
// HEALTHY answer except where noted, so an arm names only the one thing it is
// breaking and cannot accidentally depend on a second difference.
type gpBrew struct {
	installed string // `brew list --cask --versions`; default gpPinnedVer, "none" = not installed (exit 1, silent)
	tap       string // the version in the `brew info --cask` header; default gpPinnedVer, "none" = a header with no version in it
	unpinned  bool   // leave the cask out of `brew list --pinned`
	infoExit  int    // `brew info --cask` exit code; default 0
	noInfo    bool   // `brew info --cask` prints NOTHING (the unknown-cask shape)
	noRoom    bool   // do not create Caskroom/<cask>/<roomVer> at all
	roomVer   string // the Caskroom version segment; default gpPinnedVer
	roomElse  bool   // create that directory EMPTY instead of pointing it at binDir
}

// gpStubBrew plants a brew on binDir that answers the four calls
// verify-grok-pin.sh makes — `--prefix`, `list --pinned`,
// `list --cask --versions <cask>` and `info --cask <cask>` — and builds the
// Caskroom tree under a scratch prefix to match.
//
// The healthy shape points Caskroom/<cask>/<ver> AT binDir, which is where the
// grok stub lives, so "rollback target on disk" and "grok resolves into the
// pin" both read ok for the same reason they do on a real box: the binary that
// answers is the one under the declared directory. roomElse creates that
// directory empty instead, which is the drifted-but-present shape — a rollback
// target for a binary nobody runs.
func gpStubBrew(t *testing.T, binDir string, b gpBrew) {
	t.Helper()
	if b.installed == "" {
		b.installed = gpPinnedVer
	}
	if b.tap == "" {
		b.tap = gpPinnedVer
	}
	if b.roomVer == "" {
		b.roomVer = gpPinnedVer
	}

	prefix := t.TempDir()
	parent := filepath.Join(prefix, "Caskroom", gpCask)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	room := filepath.Join(parent, b.roomVer)
	switch {
	case b.noRoom:
		// nothing: the declared directory is gone
	case b.roomElse:
		if err := os.MkdirAll(room, 0o755); err != nil {
			t.Fatal(err)
		}
	default:
		if err := os.Symlink(binDir, room); err != nil {
			t.Fatal(err)
		}
	}

	pinned := gpCask
	if b.unpinned {
		pinned = ""
	}
	versions := "echo \"" + gpCask + " " + b.installed + "\"; exit 0"
	if b.installed == "none" {
		versions = "exit 1"
	}
	header := "==> " + gpCask + " (Grok Build): " + b.tap
	if b.tap == "none" {
		// A `version :latest` cask: a header with no version-shaped token in
		// it at all. The row must FAIL rather than read empty and skip the
		// gate — the same rule latestVersion got under ranger-base-phxj.
		header = "==> " + gpCask + " (Grok Build): latest"
	}
	// The exit code and the header are INDEPENDENT knobs, because on a real
	// box they are independent facts and this fixture used to tie them
	// (ranger-base-zf8pf). It rendered every non-zero exit as `exit 1` with
	// no header, which made "brew failed" and "there was nothing to read"
	// the same arm -- so the case that actually happens on this box, a
	// non-zero exit WITH a readable header, had no arm at all and the script
	// could throw that header away unnoticed for as long as it liked. It
	// did. `noInfo` is now the only way to ask for silence, and it is the
	// unknown-cask shape: brew says its piece on stderr and prints no
	// header, whatever the exit code beside it.
	info := "cat <<'HDR'\n" + header + "\nHDR\nexit " + itoa(b.infoExit)
	if b.noInfo {
		info = "exit " + itoa(b.infoExit)
	}

	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--prefix\" ]; then echo \"" + prefix + "\"; exit 0; fi\n" +
		"if [ \"$1\" = \"list\" ] && [ \"$2\" = \"--pinned\" ]; then printf '%s\\n' \"" + pinned + "\"; exit 0; fi\n" +
		"if [ \"$1\" = \"list\" ] && [ \"$2\" = \"--cask\" ]; then " + versions + "; fi\n" +
		"if [ \"$1\" = \"info\" ] && [ \"$2\" = \"--cask\" ]; then " + info + "; fi\n" +
		"echo \"stub brew: $*\" >&2; exit 99\n"
	if err := WriteExecutable(filepath.Join(binDir, "brew"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// itoa without strconv, which nothing else in this file needs.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}

// gpNoBrew: this arm wants brew ABSENT from the stub PATH. Without the marker
// gpRun plants a healthy one, which is what keeps every other call site from
// having to name brew at all.
func gpNoBrew(t *testing.T, binDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(binDir, gpBrewMarkerNoBrew), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func gpCfg(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

// gpDownloads plants per-version binaries in $GROK_HOME/downloads/ — the
// managed install's rollback cache (ranger-base-bttb1). The default fixture
// home has no downloads/ at all, which is the cask-route shape this box is in.
func gpDownloads(t *testing.T, grokHome string, names ...string) {
	t.Helper()
	dir := filepath.Join(grokHome, "downloads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		// WriteExecutable, not os.WriteFile: a real downloads/ holds
		// executables, and the tree routes every executable write through it
		// (ETXTBSY window, golang/go#22315 — execwrite-check is the door).
		if err := WriteExecutable(filepath.Join(dir, n), []byte("stub binary\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// gpRun runs the script with ONLY the stubs on PATH, and plants a healthy
// brew for any arm that did not plant its own.
//
// Defaulting rather than requiring every call site to name brew is deliberate.
// The script grew its brew rows under ranger-base-7qey3, and the ~28 arms that
// predate them are each about something else — a JSON shape, a ceiling, a
// prose line. Making them all name a cask would put a second subject into
// every one of them and invite exactly the drift gpRowFailed exists to stop,
// where an arm goes green because a DIFFERENT row failed. An arm that wants a
// sick brew calls gpStubBrew first and this leaves it alone; an arm that wants
// no brew at all calls gpNoBrew.
func gpRun(t *testing.T, root, grokHome, binDir string) (string, int) {
	t.Helper()
	if binDir != "" {
		_, planted := os.Stat(filepath.Join(binDir, "brew"))
		_, suppressed := os.Stat(filepath.Join(binDir, gpBrewMarkerNoBrew))
		if planted != nil && suppressed != nil {
			gpStubBrew(t, binDir, gpBrew{})
		}
	}
	cmd := exec.Command(filepath.Join(root, "scripts", "verify-grok-pin.sh"))
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GROK_HOME=") || strings.HasPrefix(kv, "PATH=") {
			continue
		}
		env = append(env, kv)
	}
	path := "/usr/bin:/bin"
	if binDir != "" {
		path = binDir + string(os.PathListSeparator) + path
	}
	cmd.Env = append(env, "PATH="+path, "GROK_HOME="+grokHome, "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("verify-grok-pin.sh: %v\n%s", err, out)
		}
	}
	gpRefuseVoidSz7uDefense(t, string(out))
	return string(out), code
}

// gpRefuseVoidSz7uDefense: the void sz7u no-handler premise must not come back
// as a present-tense defense in ANY block this script prints.
//
// ranger-base-g4294 voided the premise: the 1.0.5 claim that the consent record
// has no server handler is false from 1.0.52 on, where consent is
// server-persisted. ranger-base-bttb1 fixed the SPELLING the assert matched.
// ranger-base-hnddx is the SCOPE: the assert lived in the body of
// TestQAGrokPinScriptPrintsReauditWhenUpstreamMoves, so it governed the text
// THAT one arm prints — the shared re-audit list and the "pin is holding"
// paragraph — and the script's other blocks shipped the restatement green.
// The three it missed are the three THIS BOX PRINTS: verify-grok-pin.sh here
// exits 1 with 3 FAIL rows and takes the NOT-HOLDING + hard-ceiling +
// no-rollback-artifact path, so the blocks an operator reads during a live pin
// break were the uncovered ones. Asserted in gpRun it covers every call site
// at its source — all 28 — so no arm has to remember to re-assert it.
//
// What that does and does not reach, MEASURED 2026-10-19 (ranger-base-su3qt),
// the sentence below planted into each of the script's 20 printed blocks one
// at a time: 19 CAUGHT, 1 SURVIVED. Coverage is every block some fixture
// REACHES, which is not the same as every block the script HAS — the survivor
// was the `missing $pin` guard (verify-grok-pin.sh:32), unreachable because
// gpRoot writes the pin into every fake root. It is reached now, by
// TestQAGrokPinScriptMissingPinFileExits2RatherThanReportingDrift, so the
// census is 20 of 20; a block added later under a scenario no fixture sets up
// would again not be governed until one does.
//
// Match the PHRASE, case-folded, not the one spelling the pre-fix script
// happened to use. MEASURED 2026-10-15 (ranger-base-bttb1), five restatements
// planted into re-audit item 1 one at a time: the old
// `Contains(out, "has NO server handler")` caught 1 of 5 — "has no server
// handler", the binary's own "consent record not sent; no server handler yet"
// and notes-grok.md's "has **no server handler**" all shipped green.
// Case-folding the haystack and dropping "has " catches 4 of the 5; the fifth
// ("cannot persist: there is no handler") no literal grep catches.
//
// No false positive to answer for: `grep -ic "server handler"
// scripts/verify-grok-pin.sh` is 0, so the live script contains the phrase
// nowhere, in any block.
//
// MEASURED 2026-10-17 (ranger-base-hnddx), this worktree, the sentence "Note:
// the consent record has no server handler, so this drift sent nothing."
// planted into scripts/verify-grok-pin.sh one block at a time and the package
// run with -count=1: THE PIN IS NOT HOLDING, AND THE FLEET IS STOPPED and
// THERE IS NO LOCAL ROLLBACK ARTIFACT each SURVIVED before this move and are
// each CAUGHT after it. Any `-run` of a test here needs -count=1: the cache
// does not see an edit to the shell script gpRoot copies.
func gpRefuseVoidSz7uDefense(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(strings.ToLower(out), "no server handler") {
		t.Errorf("verify-grok-pin.sh restates the void sz7u no-handler defense:\n%s", out)
	}
}

// gpRowFailed: did THIS row fail, on its own line?
//
// A bare Contains(out, "FAIL") is satisfied by any other failing row, so once a
// fixture can fail more than one check the whole-output form stops naming which.
// Every row assertion here goes through this.
// gpRow is the row's VALUE, where gpRowFailed below is its verdict: the first
// field after the label, which is the middle column of every row this script
// prints. The codex twin has carried one of these (cpRow) since
// ranger-base-k4lza, for the same reason it is needed here -- "the row is not
// red" and "the row holds the version brew named" are different assertions,
// and a gate that discards a reading can satisfy the first (ranger-base-zf8pf).
func gpRow(out, label string) string {
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

func gpRowFailed(out, label string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, label) && strings.Contains(line, "FAIL") {
			return true
		}
	}
	return false
}

func TestQAGrokPinDeclarationAndMakefile(t *testing.T) {
	pin, err := os.ReadFile("etc/grok/version-pin.toml")
	if err != nil {
		t.Fatal(err)
	}
	body := string(pin)
	// The soft ceiling is asserted with a leading newline ON PURPOSE.
	// `maximum_version = "1.0.52"` is a SUFFIX of `required_maximum_version =
	// "1.0.52"`, so a bare Contains for it is satisfied by the hard-ceiling line
	// alone — delete the soft ceiling and the assertion stays green over its own
	// bug. Anchoring to line start is what makes the two rows independent.
	for _, want := range []string{
		"\nposse_pinned_version = \"1.0.52\"",
		"\nauto_update = false",
		"\nmaximum_version = \"1.0.52\"",
		// OPERATOR RULING 2026-08-28 (rangerhq-iy3y): the hard bound is SET.
		// grok refuses to start above it, so an unreviewed upgrade is a loud
		// fleet-wide stop instead of a silent un-re-audited run.
		"\nrequired_maximum_version = \"1.0.52\"",
		// OPERATOR RULING 2026-10-31 (ranger-base-yq1ar, applied under
		// ranger-base-7qey3): the pin governs the Homebrew cask. These four
		// are the brew half, and each is READ by the script — a declaration
		// that loses one of them does not make the row lenient, it makes it
		// compare against the empty string and FAIL, so the pin here is about
		// the file staying self-describing rather than about the gate.
		"\nroute = \"homebrew-cask\"",
		"\nformula = \"grok-build\"",
		"\npin_state = \"pinned\"",
		"\ncaskroom_dir = \"Caskroom/grok-build/1.0.52\"",
		// The off-disk rollback target, and the reason it is here at all: old
		// cask versions are NOT installable (no `grok-build@<v>` cask,
		// homebrew-cask not cloned), so when the Caskroom copy goes this URL
		// and this sha are the whole of what is left. MEASURED 2026-10-31 —
		// x.ai still serves the superseded releases by templated URL.
		"\nurl = \"https://x.ai/cli/grok-1.0.52-macos-aarch64\"",
		"\nsha256 = \"",
		// ADR 0076 Amendment 3 (D12/D13, ranger-base-nhxdx): the one required
		// key that is not a version ceiling. Both halves are asserted — the
		// TABLE, because the script reads this key section-aware and a
		// `program_status` that lost its table would be read by neither the
		// script nor grok, and the VALUE, because `on` is the only value posse
		// admits (`auto`, the shipped default, is MEASURED both silent on Rex
		// and leaking the `n,auth` tail of Rex's reply into grok's composer).
		"\n[ui.notifications]",
		"\nprogram_status = \"on\"",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("etc/grok/version-pin.toml missing %q", want)
		}
	}
	// Floors stay unset: a floor is the wrong direction for a pin and would
	// block rolling back to the known-good build the recovery path depends on.
	for _, refuse := range []string{"\nminimum_version =", "\nrequired_minimum_version ="} {
		if strings.Contains(body, refuse) {
			t.Errorf("version-pin.toml must not set %q — floors block rollback (rangerhq-iy3y)", refuse)
		}
	}

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "verify-grok-pin:\n\tscripts/verify-grok-pin.sh\n") {
		t.Error("Makefile lost the verify-grok-pin target")
	}

	info, err := os.Stat("scripts/verify-grok-pin.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/verify-grok-pin.sh is not executable")
	}
}

func TestQAGrokPinScriptHappyCompactJSON(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "pin intact at 1.0.52") {
		t.Errorf("missing intact line:\n%s", out)
	}
	if strings.Contains(out, "UPSTREAM MOVED") {
		t.Errorf("1.0.52==1.0.52 must not print the re-audit list:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenAutoUpdateIsOn(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = true\nmaximum_version = \"1.0.52\"\nrequired_maximum_version = \"1.0.52\"\n")
	out, code := gpRun(t, root, cfg, bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "config auto_update") {
		t.Errorf("must FAIL the auto_update row:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenMaximumVersionDrifts(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = false\nmaximum_version = \"1.0.70\"\nrequired_maximum_version = \"1.0.52\"\n")
	out, code := gpRun(t, root, cfg, bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "config maximum_version") {
		t.Errorf("must FAIL the maximum_version row:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenBinaryDrifts(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.70", gpCompactOK)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "grok --version") || !strings.Contains(out, "1.0.70") || !strings.Contains(out, "FAIL") {
		t.Errorf("must FAIL grok --version 1.0.70 vs pin 1.0.52:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenGrokReportsAutoUpdateTrue(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", `{"currentVersion":"1.0.52","latestVersion":"1.0.52","updateAvailable":false,"autoUpdate":true,"error":null}`)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "grok update: autoUpdate") || !strings.Contains(out, "true") || !strings.Contains(out, "FAIL") {
		t.Errorf("compact autoUpdate:true must FAIL even when config is false:\n%s", out)
	}
}

// The re-audit gate, driven by the TAP (ranger-base-7qey3). It used to be
// driven by grok's `latestVersion`, and that is why it had never fired on a
// healthy box: that field is clamped to maximum_version, which the pin always
// sets to the pinned version, so it equalled the pin exactly when the pin was
// intact. This arm keeps latestVersion AT the ceiling — the shape a real
// pinned box answers — and moves the tap instead, so a regression back to the
// clamped field reds here with "nothing to re-audit" over a tap three
// releases ahead.
func TestQAGrokPinScriptPrintsReauditWhenUpstreamMoves(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	gpStubBrew(t, bin, gpBrew{tap: "1.1.0"})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("pin holding with upstream moved must exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "UPSTREAM MOVED") || !strings.Contains(out, "1.1.0") {
		t.Errorf("must name the moved upstream:\n%s", out)
	}
	for _, want := range []string{
		"rangerhq-sz7u",
		"x.ai/consent/record",
		"/privacy/coding-data-retention",
		"rangerhq-vjl",
		"--permission-mode",
		"make verify-detection",
		"pin intact at 1.0.52",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("re-audit list missing %q:\n%s", want, out)
		}
	}
	// The void sz7u no-handler negative is NOT asserted here (ranger-base-hnddx).
	// Asserted in this arm's body it governed only the text this arm prints, and
	// the three blocks it missed are the three this box prints during a live pin
	// break. It lives in gpRun now — gpRefuseVoidSz7uDefense — which runs it over
	// every call site's output, this one included.
}

// gpStubGrokNoVersion: a grok that is on PATH and answers `update --check`
// but whose `--version` says nothing — the only shape that leaves live_ver
// empty, and the one arm below that cannot name a version.
func gpStubGrokNoVersion(t *testing.T, binDir, checkJSON string) {
	t.Helper()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then exit 1; fi\n" +
		"if [ \"$1\" = \"update\" ]; then\ncat <<'JSON'\n" + checkJSON + "\nJSON\nexit 0\nfi\n" +
		"echo \"stub grok: $*\" >&2; exit 99\n"
	if err := WriteExecutable(filepath.Join(binDir, "grok"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The prose the arm above prints but never read (ranger-base-fed4n, the grok
// twin of ranger-base-9ycqa finding 1). UPSTREAM MOVED is gated on the TAP
// alone, and the paragraph under it said "The pin is holding — nothing has
// changed on this machine" about a box it had not asked. live_ver was
// computed and read by exactly one thing, the version row.
//
// The four arms are each other's controls, and each pair differs in one
// thing only. 1 vs 2: the same tap, the same config, only the version the
// box RUNS — so a block that goes back to printing one paragraph for both
// reds one of them whichever paragraph it keeps. 2 vs 3: the same moved box,
// only its direction relative to the ceiling. 3 vs 4: whether `--version`
// answers at all.
//
// The ceiling arm is grok's and not codex's, which is why the remedy was not
// copied over: codex has no version ceiling, so a box past its pin runs. grok
// refuses to START above required_maximum_version (rangerhq-iy3y), so a box
// past THAT is a fleet-wide outage the operator is standing in while they
// read the paragraph — and a box that moved DOWNWARD is not, which is what
// arm 3 holds the arm to.
func TestQAGrokPinReAuditTextSaysWhetherTheBoxItselfHasMoved(t *testing.T) {
	const past = "1.1.0"
	// `moved` is the payload a HEALTHY pinned box answers — latestVersion at
	// the soft ceiling, because grok clamps it there (ranger-base-7qey3). The
	// move is the TAP's now, planted per-arm with gpStubBrew, so the name
	// still says what varies: upstream has moved, and each arm differs only in
	// what the BOX is doing about it.
	const moved = gpCompactOK

	// The re-audit list is the same work on every box: a branch that dropped
	// it on a moved box would leave the operator with a verdict and no list.
	//
	// Item 1's premise was rewritten under ranger-base-g4294: the 1.0.5
	// no-handler defense is void from 1.0.52, so the list must name the
	// strings the next re-audit greps for, and the lift gate.
	reaudit := []string{
		"1. Coding-data consent (rangerhq-sz7u, ranger-base-g4294, security)",
		"x.ai/consent/record",
		"VOID from 1.0.52",
		"\"/consent/accept\"",
		"\"/privacy/coding-data-retention\"",
		"cannot select [Opt in]",
		"default focus is not [Opt in]",
		"with no key sent",
		"2. Permission-mode precedence (rangerhq-vjl, ranger-base-ejd, security)",
		"3. The rule dialect and launcher contract",
		"4. The startup-splash detection override",
		"make verify-detection",
		"Runbook to lift the pin",
	}

	t.Run("the box is still at the pin", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrok(t, bin, "1.0.52", moved)
		gpStubBrew(t, bin, gpBrew{tap: past}) // only the TAP moved
		out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
		if code != 0 {
			t.Fatalf("exit %d, want 0 — a tap above a holding pin is not a failure\n%s", code, out)
		}
		for _, want := range append([]string{
			"UPSTREAM MOVED: grok stable is " + past,
			"The pin is holding",
			"nothing has changed on this machine",
		}, reaudit...) {
			if !strings.Contains(out, want) {
				t.Errorf("a box still at 1.0.52 must be told %q:\n%s", want, out)
			}
		}
		for _, no := range []string{"THE PIN IS NOT HOLDING", "AND THE FLEET IS STOPPED RIGHT NOW"} {
			if strings.Contains(out, no) {
				t.Errorf("a box whose binary is still 1.0.52 was told %q:\n%s", no, out)
			}
		}
	})

	// The loud shape the two ceilings exist to produce: the binary got past
	// the pin by some route the config cannot refuse, and every dispatched
	// pane is failing to start while this prints.
	t.Run("the box has moved past the hard ceiling", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrok(t, bin, past, moved)
		gpStubBrew(t, bin, gpBrew{tap: past})
		out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
		if code != 1 {
			t.Fatalf("exit %d, want 1\n%s", code, out)
		}
		for _, no := range []string{"The pin is holding", "nothing has changed on this machine"} {
			if strings.Contains(out, no) {
				t.Errorf("a box already running %s was told %q:\n%s", past, no, out)
			}
		}
		for _, want := range append([]string{
			"THE PIN IS NOT HOLDING: this box runs " + past + ", not 1.0.52.",
			// ranger-base-bttb1: the rollback sentence is MEASURED now, and
			// this fixture's GROK_HOME holds no downloads/ — which is this
			// box's own shape (the Homebrew-cask route: ~/.grok/downloads/
			// and ~/.grok/bin/ both absent, the Caskroom holding only the
			// installed version). What used to print here was a flat promise
			// that putting the pin back "needs no re-fetch", naming a
			// directory that is gone, at the one moment this block fires.
			//
			// ranger-base-7qey3 measured the half bttb1 could only call
			// unverified, and the text has to carry BOTH halves of what it
			// found: the re-fetch works, AND it does not restore the cask —
			// it recreates the managed install, which can self-update again.
			// A paragraph that promised the first without the second would
			// send an operator into a rollback that quietly un-pins the box.
			"THERE IS NO LOCAL ROLLBACK ARTIFACT",
			"old cask versions are not installable",
			"RE-FETCH over the network",
			"recreates the managed\ninstall",
			"can self-update again",
			"AND THE FLEET IS STOPPED RIGHT NOW: " + past + " is above the hard ceiling",
			"config required_maximum_version = 1.0.52",
		}, reaudit...) {
			if !strings.Contains(out, want) {
				t.Errorf("a box already running %s must be told %q:\n%s", past, want, out)
			}
		}
		if strings.Contains(out, "needs no re-fetch") {
			t.Errorf("a box with no downloads/ was promised a local rollback:\n%s", out)
		}
	})

	// The other half of that measurement, and the control on it: a box that
	// DOES still have the managed install's per-version cache gets the
	// original promise, because there it is true. Without this arm the
	// no-artifact assert above would pass over a script that had simply
	// dropped the rollback paragraph.
	t.Run("a moved box that still has downloads/ is told the rollback is local", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrok(t, bin, past, moved)
		gpStubBrew(t, bin, gpBrew{tap: past})
		home := gpCfg(t, gpGoodCfg)
		gpDownloads(t, home, "grok-1.0.5-macos-aarch64", "grok-1.0.0-macos-aarch64")
		out, code := gpRun(t, root, home, bin)
		if code != 1 {
			t.Fatalf("exit %d, want 1\n%s", code, out)
		}
		for _, want := range []string{
			"THE PIN IS NOT HOLDING: this box runs " + past + ", not 1.0.52.",
			"needs no re-fetch",
			"2 grok binaries, one file per version",
			// The two cautions this arm gained under ranger-base-7qey3. A
			// downloads/ tree on THIS box is not the old managed install the
			// promise was written for — it is what a rollback re-fetch just
			// created beside the cask, so it is both self-updatable and
			// possibly not what PATH resolves to. The sentence that used to
			// sit here, "no sha to re-verify", is gone on purpose: the pin
			// file carries a sha256 now, so there is one.
			"can self-update again",
			"which tree answers",
		} {
			if strings.Contains(out, "no sha to re-verify") {
				t.Errorf("the pin declares a sha256 now — nothing may say there is none:\n%s", out)
			}
			if !strings.Contains(out, want) {
				t.Errorf("a box with 2 cached binaries must be told %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "THERE IS NO LOCAL ROLLBACK ARTIFACT") {
			t.Errorf("a box with 2 cached binaries was told it has none:\n%s", out)
		}
	})

	// One cached binary, for the plural the count is printed with: "1 grok
	// binaries" is the shape a reader stops trusting.
	t.Run("one cached binary reads as one binary", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrok(t, bin, past, moved)
		gpStubBrew(t, bin, gpBrew{tap: past})
		home := gpCfg(t, gpGoodCfg)
		gpDownloads(t, home, "grok-1.0.5-macos-aarch64")
		out, code := gpRun(t, root, home, bin)
		if code != 1 {
			t.Fatalf("exit %d, want 1\n%s", code, out)
		}
		if !strings.Contains(out, "1 grok binary, one file per version") {
			t.Errorf("one cached binary must read as a singular:\n%s", out)
		}
	})

	// The other direction, and the control on the ceiling arm: a box BELOW
	// the pin is off it too — the version row FAILs and the pin is not
	// holding — but grok starts, so telling this operator the fleet is
	// stopped would send them after an outage that is not happening.
	t.Run("the box has moved but is under the hard ceiling", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrok(t, bin, "1.0.4", moved)
		gpStubBrew(t, bin, gpBrew{tap: past})
		out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
		if code != 1 {
			t.Fatalf("exit %d, want 1\n%s", code, out)
		}
		if !strings.Contains(out, "THE PIN IS NOT HOLDING: this box runs 1.0.4, not 1.0.52.") {
			t.Errorf("a box running 1.0.4 must be told the pin is not holding:\n%s", out)
		}
		if strings.Contains(out, "The pin is holding") {
			t.Errorf("a box running 1.0.4 was told the pin is holding:\n%s", out)
		}
		if strings.Contains(out, "AND THE FLEET IS STOPPED RIGHT NOW") {
			t.Errorf("a box UNDER the hard ceiling was told the fleet is stopped:\n%s", out)
		}
	})

	// live_ver empty. The paragraph still has to say the pin is not holding,
	// and it must not name a version it does not have — `${live_ver:-...}` is
	// the only reason the sentence reads at all here. No ceiling claim
	// either: nothing was compared to it.
	t.Run("the binary answers no version at all", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrokNoVersion(t, bin, moved)
		gpStubBrew(t, bin, gpBrew{tap: past})
		out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
		if code != 1 {
			t.Fatalf("exit %d, want 1\n%s", code, out)
		}
		if !strings.Contains(out, "THE PIN IS NOT HOLDING: this box runs an unreadable grok, not 1.0.52.") {
			t.Errorf("an unreadable binary must be named as one, not left blank:\n%s", out)
		}
		if strings.Contains(out, "The pin is holding") {
			t.Errorf("a box with an unreadable grok was told the pin is holding:\n%s", out)
		}
		if strings.Contains(out, "AND THE FLEET IS STOPPED RIGHT NOW") {
			t.Errorf("no version was read, so nothing was compared to the ceiling:\n%s", out)
		}
	})
}

func TestQAGrokPinScriptMissingGrokOrConfigExits2(t *testing.T) {
	root := gpRoot(t)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), "")
	if code != 2 {
		t.Fatalf("no grok on PATH: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "grok not on PATH") {
		t.Errorf("want grok-missing diagnostic:\n%s", out)
	}

	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	empty := t.TempDir()
	out, code = gpRun(t, root, empty, bin)
	if code != 2 {
		t.Fatalf("missing config: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("want missing-config diagnostic:\n%s", out)
	}
}

// A missing PIN FILE exits 2, rather than reporting the box as drifted
// (ranger-base-su3qt). This is the third of the script's three exit-2 guards
// and the one no arm reached: gpRoot writes the pin into every fake root, so
// scripts/verify-grok-pin.sh:32 was unreachable from all 28 gpRun call sites.
//
// What behaves differently if this arm is deleted (ADR 0006 section 7): with
// the guard regressed the script does not say the declaration is gone — every
// want_* reads EMPTY, so the version/ceiling rows FAIL against "", and it
// prints the full drift report over them. MEASURED 2026-10-19, this worktree,
// the guard's line removed and the pin file absent: "UPSTREAM MOVED: ... the
// fleet is pinned at ." and "THE PIN IS NOT HOLDING: this box runs 1.0.5,
// not ." with the whole re-audit list, exit 1. So the operator is told the
// BOX moved off the pin when in fact the pin file is missing, and the caller
// gets exit 1 (the pin is broken) where exit 2 (this cannot run) is the
// routing it asked for. Asserting the diagnostic alone would not catch that;
// the negatives below are the arm.
func TestQAGrokPinScriptMissingPinFileExits2RatherThanReportingDrift(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	if err := os.Remove(filepath.Join(root, "etc", "grok", "version-pin.toml")); err != nil {
		t.Fatal(err)
	}
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 2 {
		t.Fatalf("missing pin file: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "missing etc/grok/version-pin.toml") {
		t.Errorf("want a diagnostic naming the absent pin file:\n%s", out)
	}
	// The guard must fire BEFORE any reporting: these are the sentences the
	// regressed script printed, and each names an empty pinned version.
	for _, refuse := range []string{
		"UPSTREAM MOVED",
		"THE PIN IS NOT HOLDING",
		"Coding-data consent",
		"verify-grok-pin: pin intact",
	} {
		if strings.Contains(out, refuse) {
			t.Errorf("missing pin file must not report %q — the declaration is gone, not the box:\n%s", refuse, out)
		}
	}
}

const gpPrettyFalse = `{
  "currentVersion": "1.0.52",
  "latestVersion": "1.0.52",
  "updateAvailable": false,
  "autoUpdate": false,
  "error": null
}`

func TestQAGrokPinPrettyJSONAutoUpdateIsNotTreatedAsOffline(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	pretty := `{
  "currentVersion": "1.0.52",
  "latestVersion": "1.0.52",
  "updateAvailable": false,
  "autoUpdate": true,
  "error": null
}`
	gpStubGrok(t, bin, "1.0.52", pretty)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("pretty autoUpdate:true: exit %d, want 1 (got treated as offline?)\n%s", code, out)
	}
	if strings.Contains(out, "offline") || strings.Contains(out, "pin intact") {
		t.Errorf("pretty autoUpdate:true must not be the offline/intact arm:\n%s", out)
	}
}

// The other half of ocfh: a spaced/pretty `false` is a real answer, not silence.
// Reading it as offline would be a false PASS the day the field flips.
func TestQAGrokPinPrettyJSONAutoUpdateFalseIsRead(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpPrettyFalse)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("pretty autoUpdate:false: exit %d, want 0\n%s", code, out)
	}
	if strings.Contains(out, "offline") {
		t.Errorf("pretty autoUpdate:false is an answer, not silence:\n%s", out)
	}
	if !strings.Contains(out, "grok update: autoUpdate") || !strings.Contains(out, "pin intact at 1.0.52") {
		t.Errorf("want the autoUpdate row read ok and the pin intact:\n%s", out)
	}
}

// latestVersion has the same colon-spacing hazard as autoUpdate, and both
// directions of misreading it are a silent pass.
//
// Its SUBJECT moved under ranger-base-7qey3 — it is the clamp control now, not
// the upstream read — but the hazard did not: a pretty payload must be READ.
// Read as empty it lands on the "grok answered; no latestVersion in it" arm,
// which is at least loud; read as empty on an EMPTY payload it would land on
// offline and pass. This arm pins the pretty shape at the ceiling reading ok,
// and the one below it pins a pretty shape ABOVE the ceiling failing — same
// payload style, opposite verdict, so neither can be satisfied by a script
// that simply cannot parse pretty JSON.
func TestQAGrokPinPrettyJSONLatestVersionAtTheCeilingIsRead(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", `{
  "currentVersion": "1.0.52",
  "latestVersion": "1.0.52",
  "updateAvailable": false,
  "autoUpdate": false,
  "error": null
}`)
	gpStubBrew(t, bin, gpBrew{tap: "1.1.0"})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("pin holding with the tap moved must exit 0, got %d\n%s", code, out)
	}
	if gpRowFailed(out, "grok update: latestVersion") {
		t.Errorf("a pretty latestVersion AT the ceiling is not a failure:\n%s", out)
	}
	if strings.Contains(out, "no latestVersion in it") {
		t.Errorf("a pretty payload was read as having no latestVersion:\n%s", out)
	}
	// And the gate itself still fires, from the tap.
	if !strings.Contains(out, "UPSTREAM MOVED") || !strings.Contains(out, "1.1.0") {
		t.Errorf("the tap moved — the re-audit list must print:\n%s", out)
	}
}

// The soft ceiling, verified by EXECUTION rather than by reading config.toml
// (ranger-base-7qey3). `latestVersion` is grok's own answer about what it would
// install, clamped to maximum_version — MEASURED: no ceiling → 1.0.70, ceiling
// 1.0.52 → 1.0.52, ceiling 1.0.5 → 1.0.5. So a value ABOVE the declared soft
// ceiling means the clamp is not holding: the updater has stopped honouring
// the key, or it is reading a config that is not the one the rows above
// checked. Either is the pin failing, and before this arm existed it read as
// an upstream move and exited 0.
//
// Both shapes, because the extractor is the same one the arm above exercises.
func TestQAGrokPinLatestVersionAboveTheSoftCeilingFails(t *testing.T) {
	for _, tc := range []struct{ name, json string }{
		{"compact", `{"currentVersion":"1.0.52","latestVersion":"1.1.0","updateAvailable":true,"autoUpdate":false,"error":null}`},
		{"pretty", `{
  "currentVersion": "1.0.52",
  "latestVersion": "1.1.0",
  "updateAvailable": true,
  "autoUpdate": false,
  "error": null
}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.52", tc.json)
			out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
			if code != 1 {
				t.Fatalf("exit %d, want 1 — the clamp is not holding and nothing said so\n%s", code, out)
			}
			if !gpRowFailed(out, "grok update: latestVersion") {
				t.Errorf("want the latestVersion row FAILing on its own line:\n%s", out)
			}
			if strings.Contains(out, "pin intact") {
				t.Errorf("an unclamped updater is not an intact pin:\n%s", out)
			}
		})
	}
}

// Only an EMPTY payload is offline — that arm still exists and still exits 0,
// so the fail-closed arm below cannot creep over a genuinely absent network.
func TestQAGrokPinEmptyCheckOutputIsStillOffline(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", "")
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("no update output: exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "offline?") || !strings.Contains(out, "pin intact at 1.0.52") {
		t.Errorf("empty payload is the offline arm:\n%s", out)
	}
}

// grok answered and we could not read it. That is not silence, and passing on
// it is exactly the ocfh failure one rename away — FAIL instead.
func TestQAGrokPinUnreadableAnswerFailsInsteadOfPassing(t *testing.T) {
	for _, tc := range []struct{ name, json string }{
		{"no autoUpdate field", `{"currentVersion":"1.0.52","error":"network unreachable"}`},
		{"autoUpdate as a string", `{"autoUpdate": "true", "latestVersion": "1.0.52"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.52", tc.json)
			out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
			if code != 1 {
				t.Fatalf("exit %d, want 1\n%s", code, out)
			}
			if strings.Contains(out, "offline") || strings.Contains(out, "pin intact") {
				t.Errorf("an unreadable answer must not read as offline/intact:\n%s", out)
			}
		})
	}
}

// --- the hard ceiling (rangerhq-iy3y) -------------------------------------
//
// OPERATOR RULING 2026-08-28: set required_maximum_version, so a grok that ends
// up above the pin refuses to START rather than quietly running an un-re-audited
// build. maximum_version alone only refuses to INSTALL — a gate with a hinge.
//
// Measured against the live 1.0.5 binary before it was applied, both arms:
//   required_maximum_version = "1.0.4"  -> "This version of Grok (1.0.5) is
//                                          newer than the maximum allowed by
//                                          your organization (1.0.4)."
//   required_maximum_version = "1.0.5"  -> starts (fails later, on auth/model)
// and the CONFIG KEY gates, not only GROK_REQUIRED_MAXIMUM_VERSION. The same
// probe with only the SOFT ceiling lowered started fine, which is the whole
// distinction these two rows exist to keep apart.

// The state the ruling exists to make impossible: the pin declares a hard
// ceiling and the live config has none. That is a silent un-gate — the fleet
// keeps starting — so it must read as FAIL, not as an absent/skipped row.
func TestQAGrokPinFailsWhenRequiredMaximumVersionIsUnset(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = false\nmaximum_version = \"1.0.52\"\n")
	out, code := gpRun(t, root, cfg, bin)
	if code != 1 {
		t.Fatalf("unset hard ceiling: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "config required_max_ver") {
		t.Errorf("must FAIL the required_max_ver row when it is unset:\n%s", out)
	}
	if strings.Contains(out, "pin intact") {
		t.Errorf("an unset hard ceiling is not an intact pin:\n%s", out)
	}
}

func TestQAGrokPinFailsWhenRequiredMaximumVersionDrifts(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = false\nmaximum_version = \"1.0.52\"\nrequired_maximum_version = \"1.0.70\"\n")
	out, code := gpRun(t, root, cfg, bin)
	if code != 1 {
		t.Fatalf("drifted hard ceiling: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "config required_max_ver") || !strings.Contains(out, "1.0.70") {
		t.Errorf("must FAIL the required_max_ver row naming the drift:\n%s", out)
	}
}

// The two ceilings must be read INDEPENDENTLY, and this is the pin that says so.
//
// `maximum_version` is a suffix of `required_maximum_version`, so the extractor
// separates them only by its `^` anchor. Lose that anchor, or paste the wrong
// key into either read, and one row starts answering for both: a drifted soft
// ceiling would be reported as an ok hard one, or the hard ceiling could vanish
// behind a soft one that happens to match. Each arm below drifts exactly ONE
// key and requires exactly one row to fail — a shared read cannot satisfy both.
// The second arm deliberately writes required_ FIRST, so a `head -1` over an
// unanchored match would capture the wrong value.
func TestQAGrokPinReadsTheTwoCeilingsIndependently(t *testing.T) {
	for _, tc := range []struct {
		name, cfg, wantFail, wantOK string
	}{
		{
			name:     "soft drifts, hard holds",
			cfg:      "[cli]\nauto_update = false\nmaximum_version = \"1.0.70\"\nrequired_maximum_version = \"1.0.52\"\n",
			wantFail: "config maximum_version",
			wantOK:   "config required_max_ver",
		},
		{
			name:     "hard drifts, soft holds, hard listed first",
			cfg:      "[cli]\nauto_update = false\nrequired_maximum_version = \"1.0.70\"\nmaximum_version = \"1.0.52\"\n",
			wantFail: "config required_max_ver",
			wantOK:   "config maximum_version",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.52", gpCompactOK)
			out, code := gpRun(t, root, gpCfg(t, tc.cfg), bin)
			if code != 1 {
				t.Fatalf("exit %d, want 1\n%s", code, out)
			}
			var failed, ok bool
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, tc.wantFail) && strings.Contains(line, "FAIL") {
					failed = true
				}
				if strings.Contains(line, tc.wantOK) && strings.HasSuffix(strings.TrimRight(line, " "), "ok") {
					ok = true
				}
			}
			if !failed {
				t.Errorf("%q must FAIL on its own line:\n%s", tc.wantFail, out)
			}
			if !ok {
				t.Errorf("%q must still read ok — the rows are not independent:\n%s", tc.wantOK, out)
			}
		})
	}
}

// --- the Rex precondition row (ADR 0076 D12/D13, ranger-base-nhxdx) --------
//
// The third key posse requires in the operator's config, and the only one read
// SECTION-AWARE. `program_status` is a plausible name under more than one
// table, and the one table that counts is `[ui.notifications]` — the layer grok
// actually reads `ui.*` from. A section-blind row (the `val` extractor the two
// ceilings use, which is right for keys unique to one table) would take the key
// out of ANY table and report green over a launch posse refuses, which is the
// gate lying in the direction that costs a claim, a worktree and a billed turn
// on a `prompt: argv` seat.
//
// Each arm below writes a config grok reports NOTHING on and requires the row
// to fail. The last two are the section-blindness arms: the right value in the
// wrong table, and the right value one table short.
func TestQAGrokPinReadsTheProgramStatusKeySectionAware(t *testing.T) {
	const ceilings = "[cli]\nauto_update = false\nmaximum_version = \"1.0.52\"\nrequired_maximum_version = \"1.0.52\"\n"
	// pre goes BEFORE the [cli] table and tail after it, which matters for
	// exactly one arm: a dotted key is "at the root" only while no table
	// header has been seen yet. Written after the ceilings it would be
	// `cli.ui.notifications.program_status` — a different key, correctly
	// refused — and an arm that got that wrong would read as the reader
	// rejecting a spelling it in fact accepts.
	for _, tc := range []struct {
		name, pre, tail string
		fails           bool
	}{
		{"the key absent entirely", "", "", true},
		{"auto, the shipped default", "", "\n[ui.notifications]\nprogram_status = \"auto\"\n", true},
		{"off, which also reports nothing", "", "\n[ui.notifications]\nprogram_status = \"off\"\n", true},
		{"the right value under the wrong table", "", "\n[marketplace]\nprogram_status = \"on\"\n", true},
		{"the right value one table short", "", "\n[ui]\nprogram_status = \"on\"\n", true},
		{"the right value under [cli], where the ceilings live", "", "program_status = \"on\"\n", true},
		{"a dotted key that lands under [cli]", "", "ui.notifications.program_status = \"on\"\n", true},
		{"the table spelling grok reads", "", "\n[ui.notifications]\nprogram_status = \"on\"\n", false},
		// The two spellings that are the same document to grok's parser, so
		// the row must read them the same way the Go side does.
		{"a dotted key under [ui]", "", "\n[ui]\nnotifications.program_status = \"on\"\n", false},
		{"a fully dotted key at the root", "ui.notifications.program_status = \"on\"\n", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.52", gpCompactOK)
			cfg := tc.pre + ceilings + tc.tail
			out, code := gpRun(t, root, gpCfg(t, cfg), bin)
			if gpRowFailed(out, "config program_status") != tc.fails {
				t.Fatalf("the config program_status row failed=%v, want %v, over:\n%s\n---\n%s", !tc.fails, tc.fails, cfg, out)
			}
			// The two ceilings are untouched in every arm, so a row that
			// started answering for another key reds here rather than
			// passing: the halves must not read each other's keys.
			for _, other := range []string{"config maximum_version", "config required_max_ver", "config auto_update"} {
				if gpRowFailed(out, other) {
					t.Errorf("%q failed over a config whose [cli] table is healthy — the section-aware row is reading past its own table:\n%s", other, out)
				}
			}
			if tc.fails {
				if code != 1 {
					t.Fatalf("exit %d, want 1\n%s", code, out)
				}
				if strings.Contains(out, "pin intact") {
					t.Errorf("a config that makes grok silent on Rex is not an intact pin:\n%s", out)
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit %d, want 0\n%s", code, out)
			}
		})
	}
}

// And the PIN side of the same row: "on" is written into the assertion rather
// than taken from the pin file, because that IS the assertion — posse admits
// exactly `on` (D12), so a pin file edited to declare `auto` would otherwise
// agree with itself and the gate would pass a box that reports nothing.
func TestQAGrokPinRefusesAPinThatDeclaresAValuePosseWouldNotAdmit(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", gpCompactOK)
	pin := filepath.Join(root, "etc", "grok", "version-pin.toml")
	b, err := os.ReadFile(pin)
	if err != nil {
		t.Fatal(err)
	}
	moved := strings.Replace(string(b), "program_status = \"on\"", "program_status = \"auto\"", 1)
	if moved == string(b) {
		t.Fatal("the fixture pin does not declare program_status = \"on\" — this pin has nothing to move")
	}
	if err := os.WriteFile(pin, []byte(moved), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "pin program_status") {
		t.Errorf("a pin declaring `auto` must FAIL its own row — posse admits only `on`:\n%s", out)
	}
	// And the CONFIG row still reads ok, because the operator's own file is
	// healthy: the two rows answer about two different files.
	if gpRowFailed(out, "config program_status") {
		t.Errorf("the config row failed over a healthy config — the two rows are not independent:\n%s", out)
	}
}

// --- latestVersion is an answer too (ranger-base-phxj) ---------------------
//
// ocfh gave `autoUpdate` three arms — empty payload is offline, a readable
// value is the answer, an unreadable one FAILs — and left `latestVersion` with
// one. That asymmetry is worse than the bug it replaced: an empty `$upstream`
// skips the whole UPSTREAM MOVED block in silence, so `null`, an unquoted
// number, a rename or an empty string all read as "nothing to re-audit", exit
// 0, "pin intact". The gate that makes lifting the pin a security action is
// off and nothing says so. Measured before the fix: all four rows below were
// silent exit-0 passes.
//
// Every row differs from the control ONLY in the shape of latestVersion, so a
// green row cannot be some other check failing.
func TestQAGrokPinUnreadableLatestVersionFailsInsteadOfSilence(t *testing.T) {
	// The control: identical but for a readable, quoted version AT the soft
	// ceiling. Without it the rows below would be satisfied by a script that
	// failed every payload, including the healthy one — and since
	// ranger-base-7qey3 that is the easier bug to write, because the readable
	// value is now CHECKED rather than merely printed.
	t.Run("control: a quoted version at the ceiling reads ok", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrok(t, bin, "1.0.52", `{"autoUpdate":false,"latestVersion":"1.0.52"}`)
		out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
		if code != 0 {
			t.Fatalf("readable clamped version: exit %d, want 0\n%s", code, out)
		}
		if gpRowFailed(out, "grok update: latestVersion") {
			t.Errorf("a readable version at the ceiling is not a failure:\n%s", out)
		}
		if !strings.Contains(out, "pin intact at 1.0.52") {
			t.Errorf("control must be an intact pin:\n%s", out)
		}
	})

	for _, tc := range []struct{ name, json string }{
		{"null", `{"autoUpdate":false,"latestVersion":null}`},
		{"unquoted number", `{"autoUpdate":false,"latestVersion":1.9}`},
		{"renamed key", `{"autoUpdate":false,"latest_version":"1.9.9"}`},
		{"empty string", `{"autoUpdate":false,"latestVersion":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.52", tc.json)
			out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
			if code != 1 {
				t.Fatalf("exit %d, want 1 — an unreadable version passed in silence\n%s", code, out)
			}
			if !gpRowFailed(out, "grok update: latestVersion") {
				t.Errorf("want the latestVersion row FAILing on its own line:\n%s", out)
			}
			if strings.Contains(out, "offline") || strings.Contains(out, "pin intact") {
				t.Errorf("grok answered — this is neither offline nor an intact pin:\n%s", out)
			}
		})
	}
}

// The offline arm must survive the one above. A genuinely absent network is
// still exit 0, for BOTH fields — otherwise the fail-closed arm creeps over
// the case it was explicitly carved out of, and `make verify-grok-pin` starts
// failing on a plane.
func TestQAGrokPinEmptyPayloadIsOfflineForLatestVersionToo(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.52", "")
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("empty payload: exit %d, want 0\n%s", code, out)
	}
	if gpRowFailed(out, "grok update: latestVersion") {
		t.Errorf("an absent network is not an unreadable answer:\n%s", out)
	}
	var offline int
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "offline?") {
			offline++
		}
	}
	if offline != 2 {
		t.Errorf("want BOTH fields on the offline arm, got %d row(s):\n%s", offline, out)
	}
	if !strings.Contains(out, "pin intact at 1.0.52") {
		t.Errorf("offline is not a failure:\n%s", out)
	}
}

// A compact payload and a pretty one must agree about WHICH occurrence of a
// repeated key is authoritative.
//
// The old extractors led with a greedy `.*`, so on a single line the LAST
// match won while `head -1` made the FIRST win across lines: the same answer,
// pretty-printed, read differently. The direction that matters is the compact
// one — `{"autoUpdate":true,"nested":{"autoUpdate":false}}` reported `false
// ok`, exit 0, the true answer masked by a later false one. Each case below is
// asserted in both shapes and must give the same verdict.
func TestQAGrokPinFirstOccurrenceWinsInBothShapes(t *testing.T) {
	pretty := func(compact string) string {
		return strings.NewReplacer(",", ",\n  ", "{", "{\n  ", "}", "\n}").Replace(compact)
	}
	for _, tc := range []struct {
		name, compact string
		wantCode      int
		wantSubstr    string
		refuseSubstr  string
	}{
		{
			// The masked true. This is the one that passed.
			name:         "nested autoUpdate false must not mask the outer true",
			compact:      `{"autoUpdate":true,"nested":{"autoUpdate":false},"latestVersion":"1.0.52"}`,
			wantCode:     1,
			wantSubstr:   "true",
			refuseSubstr: "pin intact",
		},
		{
			// The mirror, and since ranger-base-7qey3 a sharper discriminator
			// than it was. The OUTER latestVersion is 1.9.9, above the soft
			// ceiling, so the row must FAIL naming it; the nested one is the
			// ceiling exactly, so a script that let the nested value win would
			// read ok and pass. Before the clamp check the two answers differed
			// only in which version got printed beside "UPSTREAM MOVED".
			name:         "nested latestVersion must not mask the outer one",
			compact:      `{"autoUpdate":false,"latestVersion":"1.9.9","nested":{"latestVersion":"1.0.52"}}`,
			wantCode:     1,
			wantSubstr:   "1.9.9",
			refuseSubstr: "pin intact",
		},
	} {
		for _, shape := range []struct{ label, json string }{
			{"compact", tc.compact},
			{"pretty", pretty(tc.compact)},
		} {
			t.Run(tc.name+"/"+shape.label, func(t *testing.T) {
				root := gpRoot(t)
				bin := t.TempDir()
				gpStubGrok(t, bin, "1.0.52", shape.json)
				out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
				if code != tc.wantCode {
					t.Fatalf("exit %d, want %d\npayload: %s\n%s", code, tc.wantCode, shape.json, out)
				}
				if !strings.Contains(out, tc.wantSubstr) {
					t.Errorf("missing %q:\npayload: %s\n%s", tc.wantSubstr, shape.json, out)
				}
				if strings.Contains(out, tc.refuseSubstr) {
					t.Errorf("must not contain %q:\npayload: %s\n%s", tc.refuseSubstr, shape.json, out)
				}
			})
		}
	}
}

// A key that is not the head of its own JSON pair is not an answer.
//
// Well-formed JSON escapes the quotes inside a string value, so a key can
// never literally appear there — the reachable hazard is grok printing a plain
// line alongside the payload. `head -1` then takes the PROSE line: the old
// extractor read `false` out of a warning and reported `false ok`, exit 0,
// over a grok that had just said true.
//
// ORDER IS THE TEST. With the prose line FIRST the old extractor answers from
// it; with the prose LAST it lands on the real field and passes, so that arm
// alone is green over the bug it exists to catch. Both are asserted; the first
// is the one that discriminates.
func TestQAGrokPinProseLineIsNotAnAutoUpdateAnswer(t *testing.T) {
	const jsonLine = `{"autoUpdate":true,"latestVersion":"1.0.52"}`
	const prose = `warning: config key "autoUpdate": false ignored`
	for _, tc := range []struct{ name, json string }{
		{"prose line first", prose + "\n" + jsonLine},
		{"prose line last", jsonLine + "\n" + prose},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.52", tc.json)
			out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
			if code != 1 {
				t.Fatalf("exit %d, want 1 — a warning line was read as the answer\n%s", code, out)
			}
			if !gpRowFailed(out, "grok update: autoUpdate") {
				t.Errorf("the real autoUpdate:true must FAIL:\n%s", out)
			}
			if strings.Contains(out, "pin intact") {
				t.Errorf("the updater says true — that is not an intact pin:\n%s", out)
			}
		})
	}
}

// The mirror, and the worse one: a version named in a plain line is not the
// version grok answered. Reading it means the clamp is checked against a
// number grok never reported — a row that says ok, or FAILs, for a reason
// that has nothing to do with the updater.
//
// Under ranger-base-7qey3 the verdict flipped and that makes the arm stronger
// rather than weaker. grok's own answer here is 1.1.0, ABOVE the soft ceiling,
// so the row must FAIL naming 1.1.0; the prose line says 9.9.9, which is also
// above the ceiling, so a script reading the prose would ALSO fail — and the
// `9.9.9` negative is what tells the two apart. Before, both readings printed
// UPSTREAM MOVED and only the printed number differed.
func TestQAGrokPinProseLineIsNotTheVersionGrokAnswered(t *testing.T) {
	const jsonLine = `{"latestVersion":"1.1.0","autoUpdate":false}`
	const prose = `note: "latestVersion": "9.9.9" is not a stable release`
	for _, tc := range []struct{ name, json string }{
		{"prose line first", prose + "\n" + jsonLine},
		{"prose line last", jsonLine + "\n" + prose},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.52", tc.json)
			out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
			if code != 1 {
				t.Fatalf("exit %d, want 1 — 1.1.0 is above the soft ceiling\n%s", code, out)
			}
			if !gpRowFailed(out, "grok update: latestVersion") || !strings.Contains(out, "1.1.0") {
				t.Errorf("must read grok's own 1.1.0 and FAIL on it:\n%s", out)
			}
			if strings.Contains(out, "9.9.9") {
				t.Errorf("9.9.9 came from a prose line, not from latestVersion:\n%s", out)
			}
		})
	}
}

// --- the brew half of the pin (ranger-base-7qey3) --------------------------
//
// OPERATOR RULING 2026-10-31 (ranger-base-yq1ar): the pin governs the Homebrew
// cask `grok-build`. `brew upgrade` is not grok's updater and obeys none of
// grok's config, so the [cli] ceilings above cannot see it coming — these rows
// are the half that can. The ruling asked for a lift-gate that "fails loud",
// and this is it: the script FAILs whenever what brew records differs from
// what the pin declares, whichever route moved it.
//
// The ruling also said `brew pin` does not apply to casks. It does on this
// Homebrew, which is why the pin_state row exists at all. MEASURED 2026-10-31
// on Homebrew 7.0.10/7.0.11: `brew pin --cask grok-build` exits 0,
// `brew list --pinned` then names it, and `brew upgrade --cask --greedy
// grok-build` answers "Error: Not upgrading 1 pinned package:". It is still
// the SECOND gate that is load-bearing — a cask declaring `auto_updates true`
// can update itself outside Homebrew, and `brew pin` has that documented hole
// — so the row asserts the belt while required_maximum_version keeps the
// braces.

// The lift-gate the ruling asked for. A `brew upgrade` that moves the cask is
// the one route the old script could not see: it read `grok --version` only,
// so a cask upgraded while a managed install stayed first on PATH reported an
// intact pin over a box whose next relink would move it.
func TestQAGrokPinFailsWhenInstalledCaskVersionDrifts(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpStubBrew(t, bin, gpBrew{installed: "1.1.0"})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("drifted cask: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "installed cask version") {
		t.Errorf("must FAIL the installed-cask row:\n%s", out)
	}
	// And it must not be absorbed by the binary row, which is green here: the
	// two readings are different questions and this fixture is exactly the
	// box where they disagree.
	if gpRowFailed(out, "grok --version") {
		t.Errorf("the binary is at the pin — that row must stay ok:\n%s", out)
	}
	if strings.Contains(out, "pin intact") {
		t.Errorf("a drifted cask is not an intact pin:\n%s", out)
	}
}

// A cask brew does not record as installed at all. The row must FAIL rather
// than read empty and pass — the same rule every extractor in the script
// follows (ranger-base-s8b4g): a reader that answers nothing must never be
// indistinguishable from an answer.
func TestQAGrokPinFailsWhenCaskIsNotInstalled(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpStubBrew(t, bin, gpBrew{installed: "none"})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("cask not installed: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "installed cask version") {
		t.Errorf("must FAIL the installed-cask row:\n%s", out)
	}
}

// The cask pin lifted. Nothing breaks the moment it happens — this is the
// silent un-gate the pin_state row exists to make loud, exactly as an unset
// required_maximum_version is.
func TestQAGrokPinFailsWhenTheCaskIsUnpinned(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpStubBrew(t, bin, gpBrew{unpinned: true})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("unpinned cask: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "brew cask pin") {
		t.Errorf("must FAIL the brew-pin row:\n%s", out)
	}
	if strings.Contains(out, "pin intact") {
		t.Errorf("an unpinned cask is not an intact pin:\n%s", out)
	}
}

// brew absent is exit 2 — "nothing measured" — and not exit 1, which would
// tell the caller the pin is broken. verify-box routes on that distinction.
// The grok guard fires FIRST, which is why this arm plants a grok: otherwise
// it would be asserting the wrong diagnostic.
func TestQAGrokPinMissingBrewExits2(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpNoBrew(t, bin)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 2 {
		t.Fatalf("no brew on PATH: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "brew not on PATH") {
		t.Errorf("want a brew-missing diagnostic:\n%s", out)
	}
	for _, refuse := range []string{"UPSTREAM MOVED", "THE PIN IS NOT HOLDING", "pin intact"} {
		if strings.Contains(out, refuse) {
			t.Errorf("nothing was measured — must not report %q:\n%s", refuse, out)
		}
	}
}

// The rollback target, and the two ways of losing it. A cask keeps exactly one
// version and old cask versions are not installable (MEASURED 2026-10-31: no
// `grok-build@<v>` cask exists and homebrew-cask is not cloned on this box), so
// the Caskroom directory is the only on-disk copy of the pinned build and
// `brew cleanup` is what takes it.
//
// Both arms are needed and neither implies the other: GONE is the directory
// absent, and "present but not what grok runs" is a rollback target for a
// binary nobody uses — the shape verify-codex-pin.sh learned to tell apart
// under ranger-base-g29az finding 3.
func TestQAGrokPinFailsWhenTheRollbackTargetIsGone(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpStubBrew(t, bin, gpBrew{noRoom: true})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("Caskroom gone: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "rollback target on disk") {
		t.Errorf("must FAIL the rollback-target row:\n%s", out)
	}
	// One broken thing, one red row: "does grok resolve into a directory that
	// does not exist" has no answer, and a second FAIL for it is how a reader
	// learns to skim the list.
	if gpRowFailed(out, "grok resolves into the pin") {
		t.Errorf("nothing to resolve into is not a second failure:\n%s", out)
	}
	if !strings.Contains(out, "nothing to resolve into") {
		t.Errorf("want the row said to be unmeasured rather than counted:\n%s", out)
	}
}

func TestQAGrokPinFailsWhenGrokDoesNotResolveIntoThePin(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	// The declared directory exists but is empty: grok answers from somewhere
	// else entirely. On a real box that is the managed install a rollback
	// re-fetch creates — `$GROK_HOME/bin/grok` ahead of the cask on PATH —
	// which is precisely the state the runbook warns about, and every other
	// row reads green in it.
	gpStubBrew(t, bin, gpBrew{roomElse: true})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("grok outside the pin: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "grok resolves into the pin") {
		t.Errorf("must FAIL the resolves-into-the-pin row:\n%s", out)
	}
	if gpRowFailed(out, "rollback target on disk") {
		t.Errorf("the declared directory is present — that row must stay ok:\n%s", out)
	}
}

// A declared caskroom_dir that names no version is always "present", because
// Homebrew never removes the parent and any installed version sits under it.
// So a prefix compare against the parent reads "grok resolves into the pin" ok
// whatever is running — ranger-base-g29az finding 3, pinned here for grok
// because the declaration is hand-written and this is the way to get it wrong
// that still looks right.
func TestQAGrokPinFailsWhenCaskroomDirNamesNoVersion(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpStubBrew(t, bin, gpBrew{})
	pin := filepath.Join(root, "etc", "grok", "version-pin.toml")
	body, err := os.ReadFile(pin)
	if err != nil {
		t.Fatal(err)
	}
	unversioned := strings.Replace(string(body),
		"caskroom_dir = \"Caskroom/grok-build/"+gpPinnedVer+"\"",
		"caskroom_dir = \"Caskroom/grok-build\"", 1)
	if unversioned == string(body) {
		t.Fatalf("fixture did not find the caskroom_dir line to blunt")
	}
	if err := os.WriteFile(pin, []byte(unversioned), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("unversioned caskroom_dir: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "rollback target on disk") || !strings.Contains(out, "unversioned") {
		t.Errorf("must FAIL naming the declaration as unversioned:\n%s", out)
	}
	if !strings.Contains(out, "names no version") {
		t.Errorf("the resolve row must say why it has no answer:\n%s", out)
	}
}

// The tap read gets the same arms the JSON fields do, and for the same reason
// (ranger-base-phxj, ranger-base-7qey3): it is the ONLY input to the re-audit
// gate now, so a reader that answers nothing is a gate that is silently off.
// The subject is NO VERSION TO READ, in each of the shapes it comes in -- the
// unknown cask that prints no header at all (with and without the non-zero
// exit that accompanies it on a real box), and the `version :latest` cask
// that prints a header with no version-shaped token in it.
//
// What is NOT in this table, and used to be: "brew exited non-zero". That is
// not a shape of having nothing to read, and asserting it here is what let
// the script discard a header it had already read -- see the sibling test
// below, which is the arm that was missing (ranger-base-zf8pf).
func TestQAGrokPinUnreadableTapVersionFailsInsteadOfSilence(t *testing.T) {
	for _, tc := range []struct {
		name string
		brew gpBrew
	}{
		{"brew info prints no header", gpBrew{noInfo: true}},
		{"brew info prints no header and exits non-zero", gpBrew{noInfo: true, infoExit: 1}},
		{"header carries no version", gpBrew{tap: "none"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
			gpStubBrew(t, bin, tc.brew)
			out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
			if code != 1 {
				t.Fatalf("exit %d, want 1 — an unreadable tap passed in silence\n%s", code, out)
			}
			if !gpRowFailed(out, "tap version") {
				t.Errorf("want the tap row FAILing on its own line:\n%s", out)
			}
			if strings.Contains(out, "pin intact") {
				t.Errorf("a gate that cannot read upstream is not an intact pin:\n%s", out)
			}
		})
	}
}

// The arm the table above used to swallow, and the one a caged seat lands in
// (ranger-base-j1eeh, ranger-base-zf8pf): `brew info --cask` prints the
// header out of its API cache and THEN exits non-zero because it could not
// refresh that cache -- it needs
// /opt/homebrew/var/homebrew/locks/<cask>.json.download.lock for write, and
// six of the eleven live PIDs declare `cage: seatbelt` over an allowlist that
// names no /opt/homebrew. MEASURED 2026-10-31: exit 1, and
// `==> grok-build (Grok Build): 1.0.52 -> 1.0.70` on stdout.
//
// A version that was READ must be USED. The old row required exit 0 and threw
// that header away, so the gate the operator's lift ruling asked to be loud
// was red on a cage artifact from the majority of seats -- and nothing here
// could see it, because the fixture made every non-zero exit silent.
func TestQAGrokPinNonZeroBrewWithAReadableHeaderStillReadsTheTap(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpStubBrew(t, bin, gpBrew{tap: gpMovedVer, infoExit: 1})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("exit %d, want 0 -- the tap version was printed and read; brew's exit code is not the question\n%s", code, out)
	}
	if got := gpRow(out, "tap version"); got != gpMovedVer {
		t.Errorf("tap version row = %q, want %q -- a header brew printed was discarded over its exit code\n%s", got, gpMovedVer, out)
	}
	if gpRowFailed(out, "tap version") {
		t.Errorf("the tap was read; the row must not be red:\n%s", out)
	}
	if !strings.Contains(out, "UPSTREAM MOVED") {
		t.Errorf("tap above the pin: the re-audit list is the whole point of reading it\n%s", out)
	}
	// Green, but not silent about it: a reader must be able to tell this row
	// from one brew was happy to answer.
	if !strings.Contains(out, "brew exited 1") {
		t.Errorf("want the non-zero brew named on the row it annotates:\n%s", out)
	}
}

// And the control on all of it: a tap AT the pin is not a move, and the
// script must say so in the one line that distinguishes "read it, nothing to
// do" from "did not read it". Without this arm every tap assertion above
// would be satisfied by a script that failed or skipped the read outright.
func TestQAGrokPinTapAtThePinIsNotAMove(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, gpPinnedVer, gpCompactOK)
	gpStubBrew(t, bin, gpBrew{tap: gpPinnedVer})
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("tap at the pin: exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "nothing to re-audit") {
		t.Errorf("want the tap read and named as no move:\n%s", out)
	}
	if strings.Contains(out, "UPSTREAM MOVED") {
		t.Errorf("the tap equals the pin — nothing moved:\n%s", out)
	}
}

// A gate must not change the box it reads, and must not report what it read
// off the box: the helper demands HOMEBREW_NO_AUTO_UPDATE=1 and
// HOMEBREW_NO_ANALYTICS=1 both, above every brew read. The reasoning, the
// analytics-egress ruling (ranger-base-w2bha, applied under
// ranger-base-esz16), and the correction of the mechanism this comment used to
// claim — brew's auto-update is not what `brew info` does; the API-cache
// re-download is — are in brewguard_qa_test.go's head comment, stated once for
// all three pin gates.
func TestQAGrokPinScriptDoesNotLetBrewAutoUpdate(t *testing.T) {
	bgAssertBrewReadsAreGuarded(t, "scripts/verify-grok-pin.sh")
}
