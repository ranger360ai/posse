package posse

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
// Part B landed (ranger-base-ypf5): the adapter execs /usr/bin/security
// absolutely, so a shim on PATH no longer reaches it and these tests name
// the shim to the adapter instead of planting one. They are now a REGRESSION
// GUARD — if the resolution ever goes back to a bare command name, the
// refusal must still be told from an outage.
// TestKeychainReadResolvesSecurityAbsolutelyNotThroughPATH is the pin on the
// absolute resolution itself, and
// TestQAKeychainStoreIsWiredToTheAbsoluteBinary is the pin on the one line
// that connects the two.

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

// gatedSecurityShim renders a persona's real gates with Bash(security:*)
// denied and hands back the path of the rendered `security` shim — the same
// bytes a persona pane puts first on the PATH of any posse command typed
// inside it. It is NAMED to the adapter rather than planted on PATH, because
// since ranger-base-ypf5 the adapter does not read PATH.
func gatedSecurityShim(t *testing.T) string {
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
	return filepath.Join(binDir, "security")
}

// keychainStub writes a stand-in `security` and hands back its path. The
// adapter execs an absolute path, so a stub is named, never planted.
func keychainStub(t *testing.T, script string) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	p := filepath.Join(t.TempDir(), "security")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// keychainTokenAt is the darwin adapter alone, asked directly rather than
// through the GOOS switch and with its binary named: these tests are about
// that adapter, and a stub answers on any box — so they run, and mean the
// same thing, under `make test-linux` (ADR 0019 D2).
func keychainTokenAt(bin string) func() (string, error) {
	return func() (string, error) {
		tok, _, err := readStore(keychainStoreAt(bin))
		return tok, err
	}
}

// The read names our own gate and the rule that refused it — and does NOT
// say "unreadable", which is the sentence a real outage says.
func TestMeterTokenNamesTheGateRefusalNotAnOutage(t *testing.T) {
	t.Parallel()
	shim := gatedSecurityShim(t)

	tok, err := keychainTokenAt(shim)()
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
	t.Parallel()
	bin := keychainStub(t, "#!/bin/sh\necho boom >&2\nexit 44\n")

	_, err := keychainTokenAt(bin)()
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
	shim := gatedSecurityShim(t)
	r := newBlindRig(t, guardOn)
	// The read fails at the credential, which is the failure this bead is
	// about — and the keychain is reached only from the compiled-in
	// endpoint, which is never dialled because the token is asked for
	// first.
	keychainOnly(planReaderOf(r.d), keychainTokenAt(shim))

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
	shim := gatedSecurityShim(t)
	// Process-global by design (app.go's legacyHomeNotices shape); another
	// test in this binary may have spent this key already.
	gateRefusalNotices.Delete("security\x00Bash(security:*)")

	var errb strings.Builder
	home := t.TempDir()
	c := &ModelCache{
		Path:   filepath.Join(home, "model-catalog.json"),
		Log:    filepath.Join(home, "model-catalog.log"),
		Caller: "preflight",
		Lister: &ModelLister{URL: "http://127.0.0.1:1/v1/models", Token: keychainTokenAt(shim)},
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

// A failed refresh does not make a retained catalog that still RULES
// UNKNOWN. Models returns such a reading as known, so the refusal notice
// must not contradict the bool on which TierPreflight acts — and must say
// how old the reading it hands on is, which is the fact the operator decides
// on (ranger-base-co5n).
//
// The fixture is `posse runtimes --probe`, because after the lease rule
// (ADR 0039 D3c) that is where the two questions come apart: maxAge 0 asks
// again, and the reading it falls back to may still be inside
// model_probe_ttl and still rule. A launch's own read cannot reach this
// branch — inside the lease it never asks at all.
func TestQAGateRefusalDoesNotCallRetainedCatalogUnknown(t *testing.T) {
	shim := gatedSecurityShim(t)
	gateRefusalNotices.Delete("security\x00Bash(security:*)")

	var errb strings.Builder
	home := t.TempDir()
	now := time.Now()
	c := &ModelCache{
		Path:   filepath.Join(home, "model-catalog.json"),
		Lister: &ModelLister{URL: "http://127.0.0.1:1/v1/models", Token: keychainTokenAt(shim)},
		Errw:   &errb,
		Now:    func() time.Time { return now },
	}
	c.store(modelEntry{At: now.Add(-30 * time.Minute), Models: []string{"claude-fable-5"}})

	ids, ok, _ := c.ModelsRead(0, time.Hour)
	if !ok || len(ids) != 1 || ids[0] != "claude-fable-5" {
		t.Fatalf("failed refresh must retain the last known catalog: %v %v", ids, ok)
	}
	got := errb.String()
	if !strings.Contains(got, "deny: Bash(security:*)") || strings.Contains(got, "UNKNOWN") {
		t.Errorf("notice must name the refusal without contradicting known=true: %q", got)
	}
	// Positive witness: silence, or the UNKNOWN sentence with the word
	// filed off, would both satisfy the line above. The notice has to name
	// the reading the launch is about to rule on, and its age.
	if !strings.Contains(got, "the catalog read 30m ago") {
		t.Errorf("notice must name the retained reading and how old it is: %q", got)
	}

	// The other side of the same seam: the same refusal over a reading PAST
	// its lease hands on no verdict, so the notice is the UNKNOWN one again.
	// Saying "launches rule on that reading" there would describe a launch
	// that no longer happens.
	gateRefusalNotices.Delete("security\x00Bash(security:*)")
	var stale strings.Builder
	c.Errw = &stale
	c.store(modelEntry{At: now.Add(-2 * time.Hour), Models: []string{"claude-fable-5"}})
	if ids, ok, _ := c.ModelsRead(0, time.Hour); ok {
		t.Errorf("a reading past its lease may not be handed on as known: %v", ids)
	}
	if s := stale.String(); !strings.Contains(s, "deny: Bash(security:*)") || !strings.Contains(s, "UNKNOWN") {
		t.Errorf("past the lease the notice must be the UNKNOWN one: %q", s)
	}
}

// The two halves of the notice are one decision, so pin the seam rather
// than only the two sentences: the bool is the one Models returns — a
// reading that is kept AND inside its lease — and everything else, a
// retained-but-empty snapshot, an undatable one, one past its lease, is
// UNKNOWN like any other no-snapshot.
func TestQAGateRefusalNoticeFollowsTheBoolItReports(t *testing.T) {
	const key = "security\x00Bash(security:*)"
	const lease = 2 * time.Hour
	err := &GateRefusal{Cmd: "security", Rule: "Bash(security:*)"}
	now := time.Now()
	for _, tc := range []struct {
		name    string
		e       modelEntry
		have    bool
		want    string
		wantNot string
	}{
		{"no snapshot", modelEntry{}, false, "UNKNOWN", "catalog read"},
		{"snapshot with no models", modelEntry{At: now.Add(-time.Hour)}, true, "UNKNOWN", "catalog read"},
		{"retained", modelEntry{At: now.Add(-90 * time.Minute), Models: []string{"claude-fable-5"}}, true, "the catalog read 1h30m ago", "UNKNOWN"},
		// Past the lease and undatable both hand on nothing to rule with,
		// so both are UNKNOWN — the first by the operator's number, the
		// second because a reading with no `at` has no lease to be inside.
		{"retained, past its lease", modelEntry{At: now.Add(-3 * time.Hour), Models: []string{"claude-fable-5"}}, true, "UNKNOWN", "rule on that reading"},
		{"retained, undatable", modelEntry{Models: []string{"claude-fable-5"}}, true, "UNKNOWN", "rule on that reading"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gateRefusalNotices.Delete(key)
			t.Cleanup(func() { gateRefusalNotices.Delete(key) })
			var errb strings.Builder
			c := &ModelCache{Errw: &errb}
			c.noteGateRefusal(err, kept(tc.e, tc.have) && withinLease(tc.e, now, lease), catalogAge(tc.e, now))
			got := errb.String()
			if !strings.Contains(got, "deny: Bash(security:*)") {
				t.Fatalf("every arm names the rule that refused us: %q", got)
			}
			if !strings.Contains(got, tc.want) || strings.Contains(got, tc.wantNot) {
				t.Errorf("want %q and not %q: %q", tc.want, tc.wantNot, got)
			}
		})
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
			c.noteGateRefusal(err, false, 0)
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
	shim := gatedSecurityShim(t)
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true
	// The read fails at the credential, so the park reason is the refusal
	// and not the transport — and the keychain is reached only from the
	// compiled-in endpoint, which is never dialled (credpin.go rule 4).
	keychainOnly(planReaderOf(r.d), keychainTokenAt(shim))
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
	t.Parallel()
	shim := gatedSecurityShim(t)
	home := t.TempDir()
	c := &PlanCache{
		Path:   filepath.Join(home, "plan-usage.json"),
		Log:    filepath.Join(home, "plan-usage.log"),
		Caller: "cost",
		// The compiled-in endpoint, and it is never dialled: the keychain
		// is read only for that url (credpin.go rule 4) and PlanReader asks
		// for the token first, so the failure is the credential and not the
		// transport.
		Reader: &AnthropicPlanReader{URL: PlanUsageURL, Token: keychainTokenAt(shim)},
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

// PART B (ranger-base-ypf5), and the pin on it. The adapter must resolve
// /usr/bin/security ABSOLUTELY, so a persona's Bash(security:*) shim sitting
// first on PATH cannot reach posse's own monitoring read.
//
// Neither half executes the real binary: reading the operator's live
// keychain in a test is exactly the class of live-state read this package
// has been burned by, and a `security find-generic-password` here could put
// a keychain prompt on their screen.
func TestKeychainReadResolvesSecurityAbsolutelyNotThroughPATH(t *testing.T) {
	// A refusal shim on PATH under the name the adapter used to ask for.
	// With the old bare-name resolution this is what answered.
	shimDir := filepath.Dir(gatedSecurityShim(t))
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Half one: what the read resolves to, asked without running it.
	// exec.Command records a bare name's LookPath answer in .Path, so a
	// regression to `security` lands the shim's path here.
	if got := keychainCmd(securityBin).Path; got != "/usr/bin/security" {
		t.Errorf("the keychain read must exec /usr/bin/security absolutely, got %q", got)
	}

	// Half two: the store execs the binary it was NAMED even with a shim
	// first on PATH — which is what makes half one's constant the whole
	// story rather than a value nothing consults.
	const secret = "sk-ant-oat01-ABSOLUTE-PATH-FIXTURE"
	stub := keychainStub(t, "#!/bin/sh\ncat <<'JSON'\n"+envelope(secret, time.Now().Add(time.Hour).UnixMilli())+"\nJSON\n")
	tok, _, err := readStore(keychainStoreAt(stub))
	if err != nil {
		t.Fatalf("the named binary must answer, not the shim on PATH: %v", err)
	}
	if tok != secret {
		t.Errorf("token came from somewhere other than the named binary: %q", tok)
	}
}

// ranger-base-gs0t, verifying ranger-base-ypf5: the WIRING, which the pin
// above does not reach.
//
// TestKeychainReadResolvesSecurityAbsolutelyNotThroughPATH asks
// keychainCmd(securityBin) and keychainStoreAt(stub) — both of which are
// handed the binary by the test. Nothing asked the one line production
// actually goes through, keychainStore(). MEASURED: with that line changed
// to keychainStoreAt("security") — the exact defect ypf5 fixed, the read
// back on PATH and back inside the persona's own refusal shim — the whole
// internal/rhq package stays green.
//
// The invariant is not "securityBin has the right value", it is "no read
// posse makes resolves `security` on PATH". Source is the only place to ask
// that without executing the operator's real keychain (rp2y's class, and a
// keychain prompt on their screen), and it is how
// TestVisibilityOverrideIsNeverDispatched asks its own never-question.
func TestQAKeychainStoreIsWiredToTheAbsoluteBinary(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("credential.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `func keychainStore() runtimeStore { return keychainStoreAt(securityBin) }`) {
		t.Errorf("keychainStore must hand the adapter securityBin — it is the only wiring between the constant and the read (ranger-base-ypf5)")
	}
	// And nothing anywhere execs the bare word a persona's shim answers to.
	for _, root := range []string{".", "../../cmd"} {
		filepath.Walk(root, func(p string, fi os.FileInfo, werr error) error {
			if werr != nil || fi.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			for _, form := range []string{`exec.Command("security"`, `keychainStoreAt("security")`, `keychainCmd("security")`} {
				if strings.Contains(string(b), form) {
					t.Errorf("%s: %s resolves on PATH, and every persona launch puts a Bash(security:*) refusal shim first there (ranger-base-ypf5/r64)", p, form)
				}
			}
			return nil
		})
	}
}
