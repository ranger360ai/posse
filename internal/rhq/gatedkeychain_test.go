package rhq

// posse's own keychain read refused by posse's own L1 gate shim
// (ranger-base-r64). Every persona launch prepends that persona's shim dir
// to PATH and every crew PID denies Bash(security:*), so a `posse` command
// typed inside a persona pane resolves KeychainToken's `security` to a
// refusal shim. Both consumers then failed in opposite directions and
// neither said why: the plan guard reported "keychain item unreadable" —
// byte-identical to a real credential outage, and on 2026-08-24 read as one
// — and the launch preflight went silently UNKNOWN.
//
// These tests run the REAL rendered shim rather than a hand-written stub, so
// the parser is pinned to the bytes gates.go actually emits: if the refusal
// line's shape moves, this fails rather than quietly returning to
// "unreadable".
//
// Part A only. Resolving /usr/bin/security absolutely (part B) must not land
// before the endpoint pin (ranger-base-17i) — see the bead.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gatedSecurityPATH renders a persona's real gates with Bash(security:*)
// denied and puts the shim dir first on PATH, which is exactly what a
// persona pane does to any posse command typed inside it.
func gatedSecurityPATH(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	_, binDir, _, err := a.RenderGates("ranger", []string{"Bash(security:*)"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// The read names our own gate and the rule that refused it — and does NOT
// say "unreadable", which is the sentence a real outage says.
func TestKeychainTokenNamesTheGateRefusalNotAnOutage(t *testing.T) {
	gatedSecurityPATH(t)

	tok, err := KeychainToken()
	if err == nil {
		t.Fatalf("a gated PATH must not yield a token: %q", tok)
	}
	var g *GateRefusal
	if !errors.As(err, &g) {
		t.Fatalf("want *GateRefusal, got %T: %v", err, err)
	}
	if g.Cmd != "security" || g.Rule != "Bash(security:*)" {
		t.Errorf("refusal must name the command and the deny rule: %+v", g)
	}
	if !strings.Contains(err.Error(), "posse's own gate") || strings.Contains(err.Error(), "unreadable") {
		t.Errorf("the misdiagnosis this bead exists to kill: %q", err)
	}
	if strings.Contains(err.Error(), "find-generic-password") {
		t.Errorf("the refusal must not quote the command's argv: %q", err)
	}
}

// An ordinary exec failure is still the ordinary error: nothing about a
// missing or broken `security` may be reported as our gate.
func TestKeychainTokenNonRefusalStaysUnreadable(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte("#!/bin/sh\necho boom >&2\nexit 44\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := KeychainToken()
	var g *GateRefusal
	if err == nil || errors.As(err, &g) {
		t.Fatalf("a plain exec failure is not a gate refusal: %v", err)
	}
	if !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("want the unreadable error: %q", err)
	}
}

// The plan guard's blind line carries the rule, so the operator reads "our
// gate refused our reader" where they used to read a credential outage.
func TestPlanGuardBlindLineNamesTheDenyRule(t *testing.T) {
	gatedSecurityPATH(t)
	r := newBlindRig(t, guardOn)
	// The endpoint stays up: the read fails at the credential, which is the
	// failure this bead is about.
	r.d.Plan.Token = KeychainToken

	if n := r.run(t); n != 1 {
		t.Fatalf("blind still fails open when attended: %d dispatched\n%s", n, r.out())
	}
	errs := r.err()
	if !strings.Contains(errs, "plan guard: ") || !strings.Contains(errs, "deny: Bash(security:*)") {
		t.Errorf("the blind line must name the rule that refused us: %q", errs)
	}
	if strings.Contains(errs, "unreadable") {
		t.Errorf("the blind line must not report our gate as a credential outage: %q", errs)
	}
}

// The preflight's UNKNOWN branch is silent by design — it launches at the
// asked-for tier and writes only to model-catalog.log. That is right for an
// outage and wrong for a refusal we configured ourselves, so this one case
// says so on stderr: once per process, not once per launch.
func TestPreflightUNKNOWNSaysOurGateRefusedUsOnce(t *testing.T) {
	gatedSecurityPATH(t)
	// Process-global by design (app.go's legacyHomeNotices shape); another
	// test in this binary may have spent this key already.
	gateRefusalNotices.Delete("security\x00Bash(security:*)")

	var errb strings.Builder
	home := t.TempDir()
	c := &ModelCache{
		Path:   filepath.Join(home, "model-catalog.json"),
		Log:    filepath.Join(home, "model-catalog.log"),
		Caller: "preflight",
		Lister: &ModelLister{URL: "http://127.0.0.1:1/v1/models", Token: KeychainToken},
		Errw:   &errb,
	}

	if ids, ok := c.Models(time.Hour); ok || len(ids) != 0 {
		t.Fatalf("a refused credential is UNKNOWN, never a catalog: %v %v", ids, ok)
	}
	said := errb.String()
	if !strings.Contains(said, "deny: Bash(security:*)") || !strings.Contains(said, "UNKNOWN") {
		t.Errorf("the silent UNKNOWN must say our gate refused us: %q", said)
	}
	if strings.Contains(said, "unreadable") {
		t.Errorf("not a credential outage: %q", said)
	}

	c.Models(time.Hour)
	if errb.String() != said {
		t.Errorf("once per process, not once per launch:\n%q", errb.String())
	}
	// It is still in the log every time, which is where the cadence lives.
	logb, _ := os.ReadFile(c.Log)
	if n := strings.Count(string(logb), "deny: Bash(security:*)"); n != 2 {
		t.Errorf("model-catalog.log must record both reads: %d\n%s", n, logb)
	}
}
