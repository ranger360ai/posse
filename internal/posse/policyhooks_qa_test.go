//go:build posse_arm3

package posse

// Pins for ranger-base-bm9cd — the policy-tier hooks drop-in,
// `etc/claude/managed-settings.d/30-posse-hooks.json`.
//
// WHAT THE FILE IS FOR. A higher settings scope cannot refuse a hook a lower
// one planted: hook lists CONCATENATE (measured, fieldpin.go). The one lever
// that refuses a persona-planted user-scope hook is `allowManagedHooksOnly`
// at the POLICY tier, and the runtime's resolver spells the cost of using it
// alone (claude 2.1.263, verbatim structure, re-read at this version):
//
//	if policySettings.disableAllHooks       -> {}
//	if policySettings.allowManagedHooksOnly -> policySettings.hooks
//	if strictPluginOnlyCustomization[hooks] -> policySettings.hooks
//	if merged.disableAllHooks               -> policySettings.hooks
//	else                                    -> merged.hooks
//
// The flag does not filter hooks, it REPLACES the set with the policy tier's
// own. So a lockdown that does not re-declare the crew's two hooks does not
// harden the box, it disarms it — the bd argv gate and herdr's reporter stop
// running and nothing says so. That is the whole reason these pins exist.
//
// WHY THE FILE IS A DROP-IN AND NOT THE @HOME@ TEMPLATE IT REPLACES. The
// template (`etc/claude/21-posse-hooks-lockdown.json.in`) spelled the two
// hook paths absolutely, so it could not be valid JSON in a public repo
// without shipping one box's home directory, and could not live in
// managed-settings.d without a glob install picking up a placeholder. Both
// constraints dissolve on a measurement the template's bead could not make
// without starting a session (2026-09-05, claude 2.1.263, four fresh headless
// sessions): a hook command IS shell-expanded, so `$HOME` resolves. The
// arm: a flag-scope PreToolUse hook spelled `cp "$HOME/.claude/hooks/…" …`
// copied the file. `$HOME` therefore carries the paths, the file is ordinary
// JSON beside the other two drop-ins, and no home directory ships.
//
// AND WHY THE GATE ARM FAILS CLOSED, which `$HOME` makes necessary. A
// PreToolUse hook exiting non-zero-but-not-2 is FAIL-OPEN by Claude Code's
// own contract, and posse runs sessions under a scratch HOME. Under one,
// `$HOME/.config/posse/gate/bd-argv-gate.sh` does not exist and a bare
// `exec` would exit 127 — fail-open, i.e. the lockdown would REMOVE the
// fence it re-declares, in silence, for exactly the seats the fence is for.
// The shipped string tests for the gate and exits 2. The reporter arm is
// deliberately the opposite (exit 0 when absent): a SessionStart hook that
// died would print on every session start on the box and fence nothing.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const policyHooksPath = "../../etc/claude/managed-settings.d/30-posse-hooks.json"

// policyHooksFile is the shape the drop-in is read back in. Deliberately not
// the runtime's schema: only what a pin here asserts.
type policyHooksFile struct {
	AllowManagedHooksOnly bool `json:"allowManagedHooksOnly"`
	Hooks                 map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func readPolicyHooks(t *testing.T) (policyHooksFile, string) {
	t.Helper()
	b, err := os.ReadFile(policyHooksPath)
	if err != nil {
		t.Fatalf("the policy hooks drop-in is missing: %v", err)
	}
	var got policyHooksFile
	if err := json.Unmarshal(b, &got); err != nil {
		// Unlike the template it replaces, this file IS installed by a glob
		// over managed-settings.d, so invalid JSON is a broken install and
		// not a placeholder waiting to be rendered.
		t.Fatalf("%s is not valid JSON, and a drop-in must be: %v", policyHooksPath, err)
	}
	return got, string(b)
}

// commandFor returns the single command string declared for event on the
// given matcher, or "" — so an assertion below names WHERE it looked.
func commandFor(f policyHooksFile, event, matcher string) string {
	for _, m := range f.Hooks[event] {
		if m.Matcher != matcher {
			continue
		}
		for _, h := range m.Hooks {
			return h.Command
		}
	}
	return ""
}

// The static half: the file locks hooks down AND hands back the two the crew
// runs, at the scope a persona cannot write.
func TestQAThePolicyHooksDropInRedeclaresTheCrewsOwnHooks(t *testing.T) {
	t.Parallel()
	got, raw := readPolicyHooks(t)

	if !got.AllowManagedHooksOnly {
		t.Errorf("%s does not set allowManagedHooksOnly — without it the file ADDS hooks to the persona-writable ones instead of replacing them, which is the whole point", policyHooksPath)
	}
	// Named by the SCRIPT each runs, not the whole command line, so a change
	// to quoting or a timeout is not a red.
	for _, row := range []struct{ event, matcher, script string }{
		{"PreToolUse", "Bash", "bd-argv-gate.sh"},
		{"SessionStart", "*", "herdr-agent-state.sh"},
	} {
		cmd := commandFor(got, row.event, row.matcher)
		if !strings.Contains(cmd, row.script) {
			t.Errorf("%s locks hooks to the policy tier and declares no %s on %s/%s — installing it would stand that hook down on every session on the box", policyHooksPath, row.script, row.event, row.matcher)
		}
	}
	// A hook that exits non-zero-but-not-2 is FAIL-OPEN. Under a scratch HOME
	// the gate is not there, and a bare exec would exit 127.
	if pre := commandFor(got, "PreToolUse", "Bash"); !strings.Contains(pre, "exit 2") {
		t.Errorf("%s's PreToolUse command has no `exit 2` arm — a $HOME that holds no posse install would fail OPEN and the lockdown would remove the fence it re-declares: %s", policyHooksPath, pre)
	}
	// posse is public. The paths travel as $HOME or they do not travel.
	if strings.Contains(raw, "/Users/") || strings.Contains(raw, "/home/") {
		t.Errorf("%s carries an absolute home directory — the paths are $HOME-relative precisely so one box's home does not ship in a public repo", policyHooksPath)
	}
}

// Two policy sources setting one key make the winner an ordering accident —
// the same argument the inlet pin's drop-in test makes, now that the
// directory holds three files.
func TestQAThePolicyDropInsDoNotSetTheSameKeyTwice(t *testing.T) {
	t.Parallel()
	const dir = "../../etc/claude/managed-settings.d"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the policy drop-in directory is missing: %v", err)
	}
	owner := map[string]string{}
	seen := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s is not valid JSON: %v", e.Name(), err)
		}
		seen++
		for k := range m {
			if prev, dup := owner[k]; dup {
				t.Errorf("%q is set by both %s and %s — two policy sources for one key make the winner an ordering accident", k, prev, e.Name())
				continue
			}
			owner[k] = e.Name()
		}
	}
	// Without this the loop above passes on an empty directory.
	if seen < 3 {
		t.Errorf("read %d drop-ins, want at least 3 (inlet, field, hooks) — a sweep over nothing agrees with everything", seen)
	}
}

// The execution half of the PreToolUse string. It is graded DIRECTLY rather
// than in a session on purpose: this box pins CLAUDE_CONFIG_DIR at the policy
// tier, so the operator's real user-scope gate loads in every session and
// refuses the same verb — an in-session deny names no gate. Here $HOME is the
// whole story.
func TestQAThePolicyHooksGateStringRefusesAFencedVerbAndFailsClosed(t *testing.T) {
	t.Parallel()
	got, _ := readPolicyHooks(t)
	cmd := commandFor(got, "PreToolUse", "Bash")
	if cmd == "" {
		t.Fatal("no PreToolUse/Bash command to grade")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		// Not a skip: the gate's own fail-closed arm is what a box with no
		// python3 gets, and it is still measurable. Only the two arms that
		// need the parser are dropped, and loudly.
		t.Errorf("python3 is not on PATH — the two arms that need the parser cannot run here; only the fail-closed arm below is measured")
	}

	// A $HOME holding a posse gate, built from the repo's own source copy.
	live := t.TempDir()
	gateDir := filepath.Join(live, ".config", "posse", "gate")
	if err := os.MkdirAll(gateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"bd-argv-gate.sh", "bd-argv-gate.py"} {
		b, err := os.ReadFile(filepath.Join("../../scripts", f))
		if err != nil {
			t.Fatalf("the gate source is missing from the repo: %v", err)
		}
		if err := os.WriteFile(filepath.Join(gateDir, f), b, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// And a $HOME holding none — a scratch HOME, which posse really runs.
	bare := t.TempDir()
	// And a third shape, which is the one the `-x` in the shipped string is
	// FOR: the gate is there and is not executable. A `cp` without a mode, an
	// install that lost one, a checkout on a filesystem that dropped the bit.
	// `exec` on it exits 126 — non-zero-but-not-2, i.e. FAIL-OPEN — so the
	// difference between the shipped `[ -x "$p" ]` and a plausible
	// `[ -r "$p" ]` is the whole fence, and only a row with the file PRESENT
	// can see it. MEASURED under ranger-base-ir42u, verifying
	// ranger-base-bm9cd: with `-r` in its place all four pins here stayed
	// green while this shape returned 126 (ranger-base-ir42u).
	unexec := t.TempDir()
	unexecGate := filepath.Join(unexec, ".config", "posse", "gate")
	if err := os.MkdirAll(unexecGate, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"bd-argv-gate.sh", "bd-argv-gate.py"} {
		b, err := os.ReadFile(filepath.Join("../../scripts", f))
		if err != nil {
			t.Fatalf("the gate source is missing from the repo: %v", err)
		}
		// The shim unreadable-as-a-program, the parser beside it untouched:
		// the shipped string tests the shim and only the shim, so the row is
		// about that one file's mode and not about a broken install.
		mode := os.FileMode(0o755)
		if f == "bd-argv-gate.sh" {
			mode = 0o644
		}
		if err := os.WriteFile(filepath.Join(unexecGate, f), b, mode); err != nil {
			t.Fatal(err)
		}
	}

	run := func(home, bashCommand string) (int, string, string) {
		t.Helper()
		payload, err := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse",
			"tool_name":       "Bash",
			"tool_input":      map[string]string{"command": bashCommand},
		})
		if err != nil {
			t.Fatal(err)
		}
		c := exec.Command("sh", "-c", cmd)
		c.Env = append(os.Environ(), "HOME="+home)
		c.Stdin = strings.NewReader(string(payload))
		var out, errb strings.Builder
		c.Stdout = &out
		c.Stderr = &errb
		_ = c.Run()
		return c.ProcessState.ExitCode(), out.String(), errb.String()
	}

	// A refusal arrives as rc 0 with JSON on stdout, NOT as a non-zero exit —
	// classifying on the exit code counts a refusing gate as a passing one.
	decision := func(stdout string) string {
		var d struct {
			HookSpecificOutput struct {
				PermissionDecision string `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal([]byte(stdout), &d) != nil {
			return ""
		}
		return d.HookSpecificOutput.PermissionDecision
	}

	// The fenced verb. A --help form on purpose: if the gate ever misses, the
	// miss is loud and harmless rather than destructive.
	if rc, out, errs := run(live, "bd"+" daemon --help"); rc != 0 || decision(out) != "deny" {
		t.Errorf("the shipped PreToolUse string did not refuse a fenced verb: rc=%d decision=%q stderr=%q — `exec` must hand the gate this hook's stdin, stdout and exit status", rc, decision(out), errs)
	}
	// The passing control. A string that refuses everything is not a win, and
	// only this arm tells the two apart.
	if rc, out, errs := run(live, "bd"+" list --status open"); rc != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("the shipped PreToolUse string refused an ALLOWED verb: rc=%d stdout=%q stderr=%q", rc, out, errs)
	}
	// The arm the `$HOME` spelling exists to make safe.
	rc, out, errs := run(bare, "bd"+" list --status open")
	if rc != 2 {
		t.Errorf("with no gate under $HOME the shipped string exited %d, want 2: any other non-zero is FAIL-OPEN by Claude Code's own contract, so a scratch-HOME session would run with no fence and nothing would say so (stdout=%q stderr=%q)", rc, out, errs)
	}
	if !strings.Contains(errs, "fail closed") {
		t.Errorf("the fail-closed exit says nothing a reader can act on; stderr was %q", errs)
	}

	// The same question asked of the shape that keeps `-x` honest. A test on
	// PRESENCE alone reads this row as a gate and hands it to `exec`.
	rc, out, errs = run(unexec, "bd"+" list --status open")
	if rc != 2 {
		t.Errorf("with a NON-EXECUTABLE gate under $HOME the shipped string exited %d, want 2: `exec` on it exits 126, which is non-zero-but-not-2 and therefore FAIL-OPEN by Claude Code's own contract — every Bash call on the box would run unfenced and nothing would say so (stdout=%q stderr=%q)", rc, out, errs)
	}
	if !strings.Contains(errs, "fail closed") {
		t.Errorf("the non-executable-gate exit says nothing a reader can act on; stderr was %q", errs)
	}
}

// The execution half of the SessionStart string, and the asymmetry: absent,
// it must be SILENT, because this one runs on every session start on the box
// and blocks nothing by failing.
func TestQAThePolicyHooksReporterStringExecsWhenPresentAndIsQuietWhenAbsent(t *testing.T) {
	t.Parallel()
	got, _ := readPolicyHooks(t)
	cmd := commandFor(got, "SessionStart", "*")
	if cmd == "" {
		t.Fatal("no SessionStart/* command to grade")
	}

	live := t.TempDir()
	hookDir := filepath.Join(live, ".claude", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(live, "ran")
	// A stand-in, because herdr installs the real reporter and posse does not
	// ship it. It records the argument, which is the half of the command line
	// that is this file's to get right.
	stub := "#!/bin/sh\nprintf '%s' \"$1\" > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(hookDir, "herdr-agent-state.sh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	bare := t.TempDir()

	run := func(home string) (int, string) {
		t.Helper()
		c := exec.Command("sh", "-c", cmd)
		c.Env = append(os.Environ(), "HOME="+home)
		c.Stdin = strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"x"}`)
		var out strings.Builder
		c.Stdout = &out
		c.Stderr = &out
		_ = c.Run()
		return c.ProcessState.ExitCode(), out.String()
	}

	if rc, out := run(live); rc != 0 {
		t.Errorf("the shipped SessionStart string exited %d with a reporter present: %q", rc, out)
	}
	b, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("the shipped SessionStart string did not run the reporter under $HOME: %v", err)
	}
	if string(b) != "session" {
		t.Errorf("the reporter was run with %q, want \"session\" — the argument is what selects herdr's session arm; without it the reporter exits 0 and reports nothing", b)
	}
	// The failing wrong arm for the assertion above: with no reporter under
	// $HOME the marker cannot appear, so its presence above is this file's
	// doing and not a leftover.
	rc, out := run(bare)
	if rc != 0 {
		t.Errorf("with no reporter under $HOME the shipped string exited %d, want 0 — this runs on every session start on the box: %q", rc, out)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("with no reporter under $HOME the shipped string printed %q — every session start on the box would carry it", out)
	}
}
