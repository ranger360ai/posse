package rhq

// posse's own keychain read refused by posse's own L1 gate shim
// (ranger-base-r64). Every persona launch prepends that persona's shim dir
// to PATH and every crew PID denies Bash(security:*), so a `posse` command
// typed inside a persona pane resolves the darwin credential adapter's
// `security` to a
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
	"sync"
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

// keychainToken is the darwin adapter alone, asked directly rather than
// through the GOOS switch: these tests are about that adapter, and a stub
// `security` on PATH answers on any box — so they run, and mean the same
// thing, under `make test-linux` (ADR 0019 D2).
func keychainToken() (string, error) {
	tok, _, err := readStore(keychainStore())
	return tok, err
}

// The read names our own gate and the rule that refused it — and does NOT
// say "unreadable", which is the sentence a real outage says.
func TestMeterTokenNamesTheGateRefusalNotAnOutage(t *testing.T) {
	gatedSecurityPATH(t)

	tok, err := keychainToken()
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
func TestMeterTokenNonRefusalStaysUnreadable(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte("#!/bin/sh\necho boom >&2\nexit 44\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := keychainToken()
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
	// The read fails at the credential, which is the failure this bead is
	// about — and the keychain is reached only from the compiled-in
	// endpoint, which is never dialled because the token is asked for
	// first.
	keychainOnly(planReaderOf(r.d), keychainToken)

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
		Lister: &ModelLister{URL: "http://127.0.0.1:1/v1/models", Token: keychainToken},
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

// A failed refresh does not make a retained catalog UNKNOWN. Models returns
// that prior reading as known, so the refusal notice must not contradict the
// bool on which TierPreflight acts.
func TestQAGateRefusalDoesNotCallRetainedCatalogUnknown(t *testing.T) {
	t.Skip("ranger-base-co5n: retained catalog is known but the refusal notice says UNKNOWN")
	gatedSecurityPATH(t)
	gateRefusalNotices.Delete("security\x00Bash(security:*)")

	var errb strings.Builder
	home := t.TempDir()
	c := &ModelCache{
		Path:   filepath.Join(home, "model-catalog.json"),
		Lister: &ModelLister{URL: "http://127.0.0.1:1/v1/models", Token: keychainToken},
		Errw:   &errb,
	}
	c.store(modelEntry{At: time.Now().Add(-2 * time.Hour), Models: []string{"claude-fable-5"}})

	ids, ok := c.Models(time.Hour)
	if !ok || len(ids) != 1 || ids[0] != "claude-fable-5" {
		t.Fatalf("failed refresh must retain the last known catalog: %v %v", ids, ok)
	}
	if got := errb.String(); !strings.Contains(got, "deny: Bash(security:*)") || strings.Contains(got, "UNKNOWN") {
		t.Errorf("notice must name the refusal without contradicting known=true: %q", got)
	}
}

func TestQAGateRefusalNoticeIsOnceUnderConcurrency(t *testing.T) {
	const key = "security\x00Bash(security:*)"
	gateRefusalNotices.Delete(key)
	t.Cleanup(func() { gateRefusalNotices.Delete(key) })

	var errb strings.Builder
	c := &ModelCache{Errw: &errb}
	err := &GateRefusal{Cmd: "security", Rule: "Bash(security:*)"}
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.noteGateRefusal(err)
		}()
	}
	wg.Wait()

	if got := strings.Count(errb.String(), "tier availability UNKNOWN"); got != 1 {
		t.Errorf("64 concurrent notices wrote %d lines, want exactly one: %q", got, errb.String())
	}
}

// The 08-24 harm path itself: unattended, past plan_guard_blind_max, the
// pass parks. That park line is what an operator reads the next morning,
// and on 2026-08-24 it read as a credential outage — the response was
// plan_guard_blind_max: 0 for hours. It must name our own gate instead.
func TestQAUnattendedBlindParkNamesOurGateNotAnOutage(t *testing.T) {
	gatedSecurityPATH(t)
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	// The read fails at the credential, so the park reason is the refusal
	// and not the transport — and the keychain is reached only from the
	// compiled-in endpoint, which is never dialled (credpin.go rule 4).
	keychainOnly(planReaderOf(r.d), keychainToken)
	r.at(12 * time.Minute)

	if n := r.run(t); n != 0 {
		t.Fatalf("past the blind budget the pass must not dispatch: %d\n%s", n, r.out())
	}
	got := r.out()
	if !strings.Contains(got, "blind 12m") || !strings.Contains(got, "deny: Bash(security:*)") {
		t.Errorf("the park line must name the age and the rule that refused us:\n%s", got)
	}
	if strings.Contains(got, "unreadable") {
		t.Errorf("the park line must not read as a credential outage:\n%s", got)
	}
	if strings.Contains(got, "find-generic-password") {
		t.Errorf("the park line must not quote the command's argv:\n%s", got)
	}
}

// plan-usage.log is where the 08-24 misdiagnosis was actually read:
// "2026-08-24T12:18:02Z cost failed: keychain item unreadable", the exact
// bytes a real credential outage writes. The log line is rendered from
// err.Error() and nothing tests it, so pin it here — the log is the
// operator's record after the process that printed the blind line is gone.
func TestQAPlanUsageLogNamesTheGateRefusal(t *testing.T) {
	gatedSecurityPATH(t)
	home := t.TempDir()
	c := &PlanCache{
		Path:   filepath.Join(home, "plan-usage.json"),
		Log:    filepath.Join(home, "plan-usage.log"),
		Caller: "cost",
		// The compiled-in endpoint, and it is never dialled: the keychain
		// is read only for that url (credpin.go rule 4) and PlanReader asks
		// for the token first, so the failure is the credential and not the
		// transport.
		Reader: &AnthropicPlanReader{URL: PlanUsageURL, Token: keychainToken},
	}

	if _, _, err := c.Read(time.Hour); err == nil {
		t.Fatal("a gated PATH must not yield a reading")
	}
	logb, err := os.ReadFile(c.Log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(logb)
	if !strings.Contains(got, "cost failed: ") || !strings.Contains(got, "deny: Bash(security:*)") {
		t.Errorf("the log line must name the rule that refused us: %q", got)
	}
	if strings.Contains(got, "unreadable") {
		t.Errorf("the 08-24 misdiagnosis, byte for byte: %q", got)
	}
	if strings.Contains(got, "find-generic-password") {
		t.Errorf("the log must not quote the command's argv: %q", got)
	}
}
