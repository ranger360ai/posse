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
required_maximum_version = "1.0.5"
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

// gpRowFailed: did THIS row fail, on its own line?
//
// A bare Contains(out, "FAIL") is satisfied by any other failing row, so once a
// fixture can fail more than one check the whole-output form stops naming which.
// Every row assertion here goes through this.
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
	// `maximum_version = "1.0.5"` is a SUFFIX of `required_maximum_version =
	// "1.0.5"`, so a bare Contains for it is satisfied by the hard-ceiling line
	// alone — delete the soft ceiling and the assertion stays green over its own
	// bug. Anchoring to line start is what makes the two rows independent.
	for _, want := range []string{
		"\nposse_pinned_version = \"1.0.5\"",
		"\nauto_update = false",
		"\nmaximum_version = \"1.0.5\"",
		// OPERATOR RULING 2026-08-28 (rangerhq-iy3y): the hard bound is SET.
		// grok refuses to start above it, so an unreviewed upgrade is a loud
		// fleet-wide stop instead of a silent un-re-audited run.
		"\nrequired_maximum_version = \"1.0.5\"",
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
	cfg := gpCfg(t, "[cli]\nauto_update = true\nmaximum_version = \"1.0.5\"\nrequired_maximum_version = \"1.0.5\"\n")
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
	gpStubGrok(t, bin, "1.0.5", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = false\nmaximum_version = \"1.0.6\"\nrequired_maximum_version = \"1.0.5\"\n")
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
	gpStubGrok(t, bin, "1.0.5", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = false\nmaximum_version = \"1.0.5\"\n")
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
	gpStubGrok(t, bin, "1.0.5", gpCompactOK)
	cfg := gpCfg(t, "[cli]\nauto_update = false\nmaximum_version = \"1.0.5\"\nrequired_maximum_version = \"1.0.6\"\n")
	out, code := gpRun(t, root, cfg, bin)
	if code != 1 {
		t.Fatalf("drifted hard ceiling: exit %d, want 1\n%s", code, out)
	}
	if !gpRowFailed(out, "config required_max_ver") || !strings.Contains(out, "1.0.6") {
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
			cfg:      "[cli]\nauto_update = false\nmaximum_version = \"1.0.6\"\nrequired_maximum_version = \"1.0.5\"\n",
			wantFail: "config maximum_version",
			wantOK:   "config required_max_ver",
		},
		{
			name:     "hard drifts, soft holds, hard listed first",
			cfg:      "[cli]\nauto_update = false\nrequired_maximum_version = \"1.0.6\"\nmaximum_version = \"1.0.5\"\n",
			wantFail: "config required_max_ver",
			wantOK:   "config maximum_version",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gpRoot(t)
			bin := t.TempDir()
			gpStubGrok(t, bin, "1.0.5", gpCompactOK)
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
	// The control: identical but for a quoted version, and it must MOVE.
	// Without it the rows below would be satisfied by a script that failed
	// every payload, including the one the gate is supposed to fire on.
	t.Run("control: quoted version still trips the gate", func(t *testing.T) {
		root := gpRoot(t)
		bin := t.TempDir()
		gpStubGrok(t, bin, "1.0.5", `{"autoUpdate":false,"latestVersion":"1.9.9"}`)
		out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
		if code != 0 {
			t.Fatalf("readable upstream: exit %d, want 0\n%s", code, out)
		}
		if !strings.Contains(out, "UPSTREAM MOVED") || !strings.Contains(out, "1.9.9") {
			t.Errorf("control must print the re-audit list:\n%s", out)
		}
		if gpRowFailed(out, "grok update: latestVersion") {
			t.Errorf("a readable version is not a failure:\n%s", out)
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
			gpStubGrok(t, bin, "1.0.5", tc.json)
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
	gpStubGrok(t, bin, "1.0.5", "")
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
	if !strings.Contains(out, "pin intact at 1.0.5") {
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
			compact:      `{"autoUpdate":true,"nested":{"autoUpdate":false},"latestVersion":"1.0.5"}`,
			wantCode:     1,
			wantSubstr:   "true",
			refuseSubstr: "pin intact",
		},
		{
			// The mirror: the outer version is the one to re-audit against.
			name:         "nested latestVersion must not mask the outer one",
			compact:      `{"autoUpdate":false,"latestVersion":"1.9.9","nested":{"latestVersion":"1.0.5"}}`,
			wantCode:     0,
			wantSubstr:   "UPSTREAM MOVED",
			refuseSubstr: "nothing to re-audit",
		},
	} {
		for _, shape := range []struct{ label, json string }{
			{"compact", tc.compact},
			{"pretty", pretty(tc.compact)},
		} {
			t.Run(tc.name+"/"+shape.label, func(t *testing.T) {
				root := gpRoot(t)
				bin := t.TempDir()
				gpStubGrok(t, bin, "1.0.5", shape.json)
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

// A key spelled inside a STRING VALUE is prose, not an answer. Anchoring is
// what tells them apart: unanchored, the first payload below reads the word
// out of the error message and reports a `false` grok never said.
func TestQAGrokPinKeyInsideAStringValueIsNotAnAnswer(t *testing.T) {
	root := gpRoot(t)
	bin := t.TempDir()
	gpStubGrok(t, bin, "1.0.5", `{"error":"autoUpdate: false is not supported","autoUpdate":true,"latestVersion":"1.0.5"}`)
	out, code := gpRun(t, root, gpCfg(t, gpGoodCfg), bin)
	if code != 1 {
		t.Fatalf("exit %d, want 1 — prose was read as the answer\n%s", code, out)
	}
	if !gpRowFailed(out, "grok update: autoUpdate") {
		t.Errorf("the real autoUpdate:true must FAIL:\n%s", out)
	}
}
