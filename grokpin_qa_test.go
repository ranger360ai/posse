package posse

// QA pins for rangerhq-y7jr (verified under rangerhq-n0bu).
//
// Claim: grok is pinned at 1.0.5 via [cli] auto_update = false and
// maximum_version, declared in etc/grok/version-pin.toml and asserted by
// scripts/verify-grok-pin.sh. The script is also the re-audit gate: when
// upstream stable moves past the pin it prints hoover's list and still
// exits 0 (the pin is holding; lifting it is the operator's).
//
// Live grok 1.0.5 emits compact update JSON (`"autoUpdate":false` with no
// space). The hermetic cases below stub grok so make test does not need
// the operator's ~/.grok or a network.
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

const gpCompactOK = `{"currentVersion":"1.0.5","latestVersion":"1.0.5","updateAvailable":false,"autoUpdate":false,"error":null}`

const gpGoodCfg = `[cli]
installer = "internal"
auto_update = false
maximum_version = "1.0.5"
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
	if err := os.WriteFile(filepath.Join(root, "scripts", "verify-grok-pin.sh"), script, 0o755); err != nil {
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
		body = "#!/bin/bash\n" +
			"if [ \"$1\" = \"--version\" ]; then echo \"grok " + version + " (deadbeef) [stable]\"; exit 0; fi\n" +
			"if [ \"$1\" = \"update\" ]; then exit 0; fi\n" +
			"echo \"stub grok: $*\" >&2; exit 99\n"
	} else {
		body = "#!/bin/bash\n" +
			"if [ \"$1\" = \"--version\" ]; then echo \"grok " + version + " (deadbeef) [stable]\"; exit 0; fi\n" +
			"if [ \"$1\" = \"update\" ]; then\n" +
			"cat <<'JSON'\n" + checkJSON + "\nJSON\n" +
			"exit 0\nfi\n" +
			"echo \"stub grok: $*\" >&2; exit 99\n"
	}
	if err := os.WriteFile(filepath.Join(binDir, "grok"), []byte(body), 0o755); err != nil {
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

func gpRun(t *testing.T, root, grokHome, binDir string) (string, int) {
	t.Helper()
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
	return string(out), code
}

func TestQAGrokPinDeclarationAndMakefile(t *testing.T) {
	pin, err := os.ReadFile("etc/grok/version-pin.toml")
	if err != nil {
		t.Fatal(err)
	}
	body := string(pin)
	for _, want := range []string{
		`posse_pinned_version = "1.0.5"`,
		`auto_update = false`,
		`maximum_version = "1.0.5"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("etc/grok/version-pin.toml missing %q", want)
		}
	}
	// Floors and the hard ceiling are deliberately unset (rangerhq-iy3y).
	for _, refuse := range []string{"required_maximum_version =", "minimum_version =", "required_minimum_version ="} {
		if strings.Contains(body, refuse) {
			t.Errorf("version-pin.toml must not set %q — floors block rollback; the hard ceiling is the operator's (rangerhq-iy3y)", refuse)
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
	gpStubGrok(t, bin, "1.0.5", gpCompactOK)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "pin intact at 1.0.5") {
		t.Errorf("missing intact line:\n%s", out)
	}
	if strings.Contains(out, "UPSTREAM MOVED") {
		t.Errorf("1.0.5==1.0.5 must not print the re-audit list:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenAutoUpdateIsOn(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.5", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = true\nmaximum_version = \"1.0.5\"\n")
	out, code := gpRun(t, root, cfg, bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "config auto_update") || !strings.Contains(out, "FAIL") {
		t.Errorf("must FAIL the auto_update row:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenMaximumVersionDrifts(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.5", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = false\nmaximum_version = \"1.0.6\"\n")
	out, code := gpRun(t, root, cfg, bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "config maximum_version") || !strings.Contains(out, "FAIL") {
		t.Errorf("must FAIL the maximum_version row:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenBinaryDrifts(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.6", gpCompactOK)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "grok --version") || !strings.Contains(out, "1.0.6") || !strings.Contains(out, "FAIL") {
		t.Errorf("must FAIL grok --version 1.0.6 vs pin 1.0.5:\n%s", out)
	}
}

func TestQAGrokPinScriptFailsWhenGrokReportsAutoUpdateTrue(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.5", `{"currentVersion":"1.0.5","latestVersion":"1.0.5","updateAvailable":false,"autoUpdate":true,"error":null}`)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "grok update: autoUpdate") || !strings.Contains(out, "true") || !strings.Contains(out, "FAIL") {
		t.Errorf("compact autoUpdate:true must FAIL even when config is false:\n%s", out)
	}
}

func TestQAGrokPinScriptPrintsReauditWhenUpstreamMoves(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.5", `{"currentVersion":"1.0.5","latestVersion":"1.1.0","updateAvailable":true,"autoUpdate":false,"error":null}`)
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
		"rangerhq-vjl",
		"--permission-mode",
		"make verify-detection",
		"pin intact at 1.0.5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("re-audit list missing %q:\n%s", want, out)
		}
	}
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
	gpStubGrok(t, bin, "1.0.5", gpCompactOK)
	empty := t.TempDir()
	out, code = gpRun(t, root, empty, bin)
	if code != 2 {
		t.Fatalf("missing config: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("want missing-config diagnostic:\n%s", out)
	}
}

const gpPrettyFalse = `{
  "currentVersion": "1.0.5",
  "latestVersion": "1.0.5",
  "updateAvailable": false,
  "autoUpdate": false,
  "error": null
}`

func TestQAGrokPinPrettyJSONAutoUpdateIsNotTreatedAsOffline(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	pretty := `{
  "currentVersion": "1.0.5",
  "latestVersion": "1.0.5",
  "updateAvailable": false,
  "autoUpdate": true,
  "error": null
}`
	gpStubGrok(t, bin, "1.0.5", pretty)
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
	gpStubGrok(t, bin, "1.0.5", gpPrettyFalse)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("pretty autoUpdate:false: exit %d, want 0\n%s", code, out)
	}
	if strings.Contains(out, "offline") {
		t.Errorf("pretty autoUpdate:false is an answer, not silence:\n%s", out)
	}
	if !strings.Contains(out, "grok update: autoUpdate") || !strings.Contains(out, "pin intact at 1.0.5") {
		t.Errorf("want the autoUpdate row read ok and the pin intact:\n%s", out)
	}
}

// latestVersion has the same colon-spacing hazard, and losing it loses the
// re-audit gate silently: upstream moves, the list never prints.
func TestQAGrokPinPrettyJSONUpstreamMovePrintsReaudit(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.5", `{
  "currentVersion": "1.0.5",
  "latestVersion": "1.1.0",
  "updateAvailable": true,
  "autoUpdate": false,
  "error": null
}`)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("pin holding with upstream moved must exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "UPSTREAM MOVED") || !strings.Contains(out, "1.1.0") {
		t.Errorf("pretty latestVersion must still trip the re-audit gate:\n%s", out)
	}
}

// Only an EMPTY payload is offline — that arm still exists and still exits 0,
// so the fail-closed arm below cannot creep over a genuinely absent network.
func TestQAGrokPinEmptyCheckOutputIsStillOffline(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.5", "")
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 0 {
		t.Fatalf("no update output: exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "offline?") || !strings.Contains(out, "pin intact at 1.0.5") {
		t.Errorf("empty payload is the offline arm:\n%s", out)
	}
}

// grok answered and we could not read it. That is not silence, and passing on
// it is exactly the ocfh failure one rename away — FAIL instead.
func TestQAGrokPinUnreadableAnswerFailsInsteadOfPassing(t *testing.T) {
	for _, tc := range []struct{ name, json string }{
		{"no autoUpdate field", `{"currentVersion":"1.0.5","error":"network unreachable"}`},
		{"autoUpdate as a string", `{"autoUpdate": "true", "latestVersion": "1.0.5"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.5", tc.json)
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
