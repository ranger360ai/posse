package posse

// The GIT_CONFIG_* family, for ranger-base-44or9. Filed from the verify bead
// ranger-base-onx3x as finding 2 of the bundle ranger-base-r2s9l and handed
// to the devops lane there (ADR 0006 §1).
//
// THE DEFECT THIS PINS. ranger-base-rflee's fix spec named four git exec
// inlets — "GIT_SSH_COMMAND / GIT_EXTERNAL_DIFF / GIT_PAGER / GIT_CONFIG_*"
// — and the landed table pinned three. GIT_CONFIG_* appeared nowhere in
// inletpin.go: not as a row, and not in its "NOT COVERED HERE, deliberately"
// paragraph. That silence is the defect, because the file's own contract is
// that its coverage is exactly the names in the table and "a name that is not
// here is not covered, and a reader has to be able to see that".
//
// WHAT MEASUREMENT MADE OF THE FOUR-WAY FIX THE BEAD ASKED FOR. Two of the
// family can be pinned and now are. Two cannot be, and the cost of pinning
// each was measured rather than assumed, so they are disclosed in the file
// instead — which is the other half of what this test holds:
//
//   - GIT_CONFIG_SYSTEM=/dev/null   pinned. Neutral on Apple git 2.50.1.
//   - GIT_CONFIG_PARAMETERS=""      pinned. The family member nobody named.
//   - GIT_CONFIG_COUNT              NOT pinnable: `0` also switches off
//                                   posse's own L3 hooks redirect (ADR 0052
//                                   D2), which rides this exact mechanism;
//                                   ≥1 is rc 128 on every git command.
//   - GIT_CONFIG_GLOBAL             NOT pinnable: every closing spelling
//                                   drops ~/.gitconfig, and git then commits
//                                   under a gecos ident at rc 0 — silent
//                                   misattribution, fleet-wide.
//
// So this file asserts three things and not one: the two rows are pinned at
// both ends, the two that are not pinned are DISCLOSED, and the live gap is
// still exactly the gap the disclosure describes. That last arm is what stops
// the paragraph going stale: if a future git closes one of these by itself,
// the test fails and says the prose is now wrong.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitConfigPinned is the half of the family that has a neutral spelling, with
// the value measured neutral for each (2026-09-05, git 2.50.1 Apple Git-155).
var gitConfigPinned = map[string]string{
	"GIT_CONFIG_SYSTEM":     os.DevNull,
	"GIT_CONFIG_PARAMETERS": "",
}

// gitConfigDisclosed is the half that has none, and therefore has to be
// visible in the file's own prose instead. The value is the phrase a reader
// looking for the reason would search for.
var gitConfigDisclosed = map[string]string{
	"GIT_CONFIG_COUNT":  "no pinnable value",
	"GIT_CONFIG_GLOBAL": "no neutral spelling",
}

func TestQATheInletPinCoversTheGitConfigFamilyOrSaysWhyNot(t *testing.T) {
	t.Parallel()

	pinned := map[string]string{}
	for _, v := range inletPin() {
		pinned[v.Key] = v.Value
	}
	for k, want := range gitConfigPinned {
		got, ok := pinned[k]
		if !ok {
			t.Errorf("inletPin() does not carry %s — ranger-base-rflee's fix spec named GIT_CONFIG_* beside the three git names the table does pin, and a settings payload can only SET keys, so a name that is not here is not covered", k)
			continue
		}
		if got != want {
			t.Errorf("inletPin()[%s] = %q, want %q (the value measured neutral)", k, got, want)
		}
	}
	// The other direction, and it is not symmetry for its own sake: pinning
	// either of these is a fleet outage, so the pin refusing them is a
	// property to hold, not an omission to tolerate.
	for k := range gitConfigDisclosed {
		if v, ok := pinned[k]; ok {
			t.Errorf("inletPin() carries %s=%q. It must not: %s closes the inlet but is not neutral — see the ALSO NOT COVERED paragraph in inletpin.go for the measurement, and re-measure before overruling it", k, v, k)
		}
	}

	// Both ends of the pin or neither: the drop-in is what covers the
	// operator's own uncaged session, which is the end this bead is about.
	const path = "../../etc/claude/managed-settings.d/10-posse-inlet-pin.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the policy-tier half of the pin is missing: %v", err)
	}
	var dropIn struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(b, &dropIn); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	for k := range gitConfigPinned {
		if _, ok := dropIn.Env[k]; !ok {
			t.Errorf("%s does not carry %s either, so the operator's own session is uncovered too", path, k)
		}
	}
	for k := range gitConfigDisclosed {
		if _, ok := dropIn.Env[k]; ok {
			t.Errorf("%s carries %s — the drop-in is the tier that reaches the operator's OWN sessions, so this is where the cost of that pin is paid first", path, k)
		}
	}

	// The disclosure itself. A gap this file's contract calls uncovered has
	// to be READABLE as uncovered, which is the whole finding.
	src, err := os.ReadFile("inletpin.go")
	if err != nil {
		t.Fatalf("cannot read the table to check its own disclosure: %v", err)
	}
	const marker = "ALSO NOT COVERED"
	i := strings.Index(string(src), marker)
	if i < 0 {
		t.Fatalf("inletpin.go has no %q paragraph — the two names below are inlets that fire, and the file's contract is that a reader can see which names are not covered", marker)
	}
	gaps := string(src)[i:]
	for k, why := range gitConfigDisclosed {
		if !strings.Contains(gaps, k) {
			t.Errorf("inletpin.go's %q paragraph does not name %s. It is an inlet that fires past every pinned row, and silence about it is the defect ranger-base-44or9 filed", marker, k)
		}
		if !strings.Contains(gaps, why) {
			t.Errorf("inletpin.go's %q paragraph does not say %q for %s — a name listed as uncovered without the measured reason invites the next reader to just pin it", marker, why, k)
		}
	}

	// ── the live half ────────────────────────────────────────────────────
	//
	// Arms carry whatever inletPin() currently says, applied LAST, because
	// that is the direction of the real thing: a settings `env` block is
	// assigned OVER process.env, so the pin overwrites an attacker's value
	// rather than the other way round.
	repo := gitConfigProbeRepo(t)
	hooks := gitConfigProbeHooks(t)

	fires := func(t *testing.T, attack ...string) bool {
		t.Helper()
		hookMarker := filepath.Join(hooks, "marker")
		_ = os.Remove(hookMarker)
		env := append(gitConfigScrubbedEnv(), attack...)
		for _, v := range inletPin() {
			env = append(env, v.Key+"="+v.Value)
		}
		gitConfigCheckoutRoundTrip(repo, env)
		_, err := os.Stat(hookMarker)
		return err == nil
	}

	// Control: the pin alone must not run the attacker's hook, or every arm
	// below is measuring the fixture instead of the inlet.
	if fires(t) {
		t.Fatal("CONTROL FAILED: the hook fired with no GIT_CONFIG_* attack at all")
	}

	if fires(t, "GIT_CONFIG_SYSTEM="+gitConfigProbeGlobal(t, hooks)) {
		t.Errorf("GIT_CONFIG_SYSTEM still reaches past the pin: an attacker's system-scope config ran a post-checkout hook with the whole inlet pin applied")
	}
	if fires(t, "GIT_CONFIG_PARAMETERS='core.hooksPath'='"+hooks+"'") {
		t.Errorf("GIT_CONFIG_PARAMETERS still reaches past the pin: git's own command-line-config channel ran a post-checkout hook with the whole inlet pin applied")
	}

	// The two disclosed gaps, asserted OPEN. This is the arm that keeps the
	// prose honest: it fails when the gap closes, and a gap that closed is a
	// paragraph that now lies.
	if !fires(t, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0="+hooks) {
		t.Errorf("core.hooksPath via GIT_CONFIG_COUNT no longer reaches past the pin. Good news, but inletpin.go's %q paragraph says it does — re-measure and rewrite it", marker)
	}
	if !fires(t, "GIT_CONFIG_GLOBAL="+gitConfigProbeGlobal(t, hooks)) {
		t.Errorf("GIT_CONFIG_GLOBAL no longer reaches past the pin. Good news, but inletpin.go's %q paragraph says it does — re-measure and rewrite it", marker)
	}
}

// TestQATheGitConfigCountPinWouldBreakThePosseHooksRedirect is the
// measurement that keeps GIT_CONFIG_COUNT out of the table, run rather than
// cited. ADR 0052 D2 aims every session's git at posse's rendered hooks dir
// through GIT_CONFIG_COUNT/KEY_n/VALUE_n on the pane env
// (gitConfigHooksPathVars, appended at herdrback.go). A settings `env` block
// is assigned over process.env — so a pinned GIT_CONFIG_COUNT=0 would zero
// the count the pane set and strand KEY_0/VALUE_0, switching off the bd argv
// gate and the employer's managed hooks fleet-wide.
//
// The attack arm and posse's own redirect are byte-identical in shape, which
// is exactly why closing one closes the other and why this cannot be argued
// either way from reading.
func TestQATheGitConfigCountPinWouldBreakThePosseHooksRedirect(t *testing.T) {
	t.Parallel()
	repo := gitConfigProbeRepo(t)
	hooks := gitConfigProbeHooks(t)
	marker := filepath.Join(hooks, "marker")

	// What herdrback appends today, for a session whose env carried none of
	// its own: count 1, index 0.
	pane := gitConfigHooksPathVars(nil, hooks)
	env := gitConfigScrubbedEnv()
	for _, v := range pane {
		env = append(env, v.Key+"="+v.Value)
	}

	_ = os.Remove(marker)
	gitConfigCheckoutRoundTrip(repo, env)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("CONTROL FAILED: posse's own hooks redirect did not dispatch %s with the pane vars alone, so the arm below measures nothing", marker)
	}

	// Now the hypothetical pin, assigned over it the way a settings env is.
	_ = os.Remove(marker)
	gitConfigCheckoutRoundTrip(repo, append(env, "GIT_CONFIG_COUNT=0"))
	if _, err := os.Stat(marker); err == nil {
		t.Errorf("GIT_CONFIG_COUNT=0 laid over the pane redirect left it dispatching. If that is true now, the ALSO NOT COVERED paragraph in inletpin.go is out of date and the row may be pinnable after all — re-measure before pinning it")
	}
}

// gitConfigScrubbedEnv is this process's environment with every GIT_CONFIG_*
// name removed. A seat launched by posse carries the L3 redirect in its own
// env (ADR 0052 D2), which would otherwise sit underneath every arm above and
// make the control fire.
func gitConfigScrubbedEnv() []string {
	var out []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GIT_CONFIG_") || strings.HasPrefix(e, "GIT_CONFIG=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// gitConfigCheckoutRoundTrip moves HEAD and back: post-checkout runs on each.
func gitConfigCheckoutRoundTrip(repo string, env []string) {
	for _, rev := range []string{"HEAD~1", "-"} {
		c := exec.Command("git", "checkout", "-q", rev)
		c.Dir, c.Env = repo, env
		_ = c.Run()
	}
}

// gitConfigProbeHooks is the attacker's (or, in the redirect arm, posse's)
// hooks dir: a post-checkout that leaves a marker beside itself.
func gitConfigProbeHooks(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\necho fired >> " + filepath.Join(dir, "marker") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "post-checkout"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// gitConfigProbeRepo is two commits in a scratch repo — enough for a
// checkout to move and fire post-checkout.
func gitConfigProbeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = gitConfigScrubbedEnv()
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", ".")
	run("config", "user.email", "qa@example.invalid")
	run("config", "user.name", "qa")
	for _, body := range []string{"one\n", "two\n"} {
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "--", "f")
		run("commit", "-q", "-m", "probe", "--", "f")
	}
	return dir
}

// gitConfigProbeGlobal writes the config file an attacker would point
// GIT_CONFIG_GLOBAL or GIT_CONFIG_SYSTEM at.
func gitConfigProbeGlobal(t *testing.T, hooks string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "evil.gitconfig")
	if err := os.WriteFile(p, []byte("[core]\n\thooksPath = "+hooks+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestQATheGitConfigSystemPinDropsASystemScopeIdentityLiteral is the cost of
// the row that IS pinned, for ranger-base-nn161 (found verifying
// ranger-base-44or9 on ranger-base-53y2k). The close above disclosed that
// GIT_CONFIG_SYSTEM=/dev/null "WOULD suppress /etc/gitconfig off darwin" and
// stopped there. Two paragraphs up, the same file spells that exact cost out
// in full as its whole reason for refusing GIT_CONFIG_GLOBAL: emptying a
// scope takes an e-mail literal out of the visibility wall, silently. The
// asymmetry was the finding — a reader of the row that LANDED could not see a
// cost the file writes out for the row it rejected, in a file whose contract
// is that its coverage is readable.
//
// So this holds both ends of that clause, because prose nobody can watch fail
// is how the first omission survived a close:
//
//   - the DISCLOSURE, anchored INSIDE the GIT_CONFIG_SYSTEM row and not
//     merely somewhere in the file. Anchoring is the whole point: the file
//     already named DeriveIdentityLiterals in the GIT_CONFIG_GLOBAL
//     paragraph while the row said nothing, which is exactly the state this
//     test has to call red.
//   - the BEHAVIOUR, by execution, three arms. It fails if git ever stops
//     dropping the literal — at which point the clause above is a paragraph
//     that lies, the same standing the two disclosed gaps are held to.
//
// Not live on this box and not a bug: system scope here is empty, there is no
// /etc/gitconfig, and the full config listing is byte-identical under the pin
// (that is 44or9's measurement, reproduced). It needs a box whose user.email
// lives in system scope, and the fleet is not darwin by contract — the
// LD_PRELOAD row two rows up says so. Whether such a box wants the row at all
// is the operator's, on ranger-base-zz08i.
func TestQATheGitConfigSystemPinDropsASystemScopeIdentityLiteral(t *testing.T) {
	// t.Setenv: DeriveIdentityLiterals shells out with this process's own
	// environment, so the arms move the real thing and cannot be parallel.

	// ── the disclosure, on the ROW ───────────────────────────────────────
	src, err := os.ReadFile("inletpin.go")
	if err != nil {
		t.Fatalf("cannot read the table to check its own disclosure: %v", err)
	}
	const rowHead, nextRow = "GIT_CONFIG_SYSTEM=" + os.DevNull, `GIT_CONFIG_PARAMETERS=""`
	i := strings.Index(string(src), rowHead)
	j := strings.Index(string(src), nextRow)
	if i < 0 || j <= i {
		t.Fatalf("inletpin.go has no %q row followed by the %q row — the region this test reads is gone, so it is measuring nothing; re-anchor it before trusting a green", rowHead, nextRow)
	}
	row := string(src)[i:j]
	for _, want := range []string{"DeriveIdentityLiterals", "user.email"} {
		if !strings.Contains(row, want) {
			t.Errorf("the GIT_CONFIG_SYSTEM row does not name %q. Suppressing a config scope takes an e-mail out of the ADR 0024 D2 check 3 wall with no error — the file writes that cost out in full for GIT_CONFIG_GLOBAL, and a reader of the row that LANDED has to be able to see it too (ranger-base-nn161)", want)
		}
	}

	// ── the behaviour, three arms ────────────────────────────────────────
	//
	// Every GIT_CONFIG_* name off first: a posse-launched seat carries the
	// L3 hooks redirect in its own env (ADR 0052 D2), and an ambient
	// GIT_CONFIG_GLOBAL or _SYSTEM would sit underneath every arm.
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && strings.HasPrefix(k, "GIT_CONFIG") {
			unsetenvForTest(t, k)
		}
	}
	// Global scope closed so the SYSTEM arm is the only scope that moves.
	// This is the variable the table refuses to pin, used here as a fixture
	// and never as a pin — its cost is exactly what is being demonstrated.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)

	repo := gitConfigProbeRepo(t)
	const addr = "systemscope@example.invalid"
	sys := filepath.Join(t.TempDir(), "system.gitconfig")
	if err := os.WriteFile(sys, []byte("[user]\n\temail = "+addr+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	walled := func(t *testing.T, arm, value string) bool {
		t.Helper()
		if value == "" {
			unsetenvForTest(t, "GIT_CONFIG_SYSTEM")
		} else {
			t.Setenv("GIT_CONFIG_SYSTEM", value)
		}
		lits, err := DeriveIdentityLiterals(repo)
		if err != nil {
			t.Fatalf("%s: DeriveIdentityLiterals: %v", arm, err)
		}
		var found bool
		for _, l := range lits {
			if l.Class == "email" && l.Value == addr {
				found = true
			}
		}
		t.Logf("arm %-42s %d literals, %s walled: %v", arm, len(lits), addr, found)
		return found
	}

	// Unset: the fixture is not on this box, so it can only arrive through
	// the variable. Without this arm the attack arm below proves nothing.
	if walled(t, "GIT_CONFIG_SYSTEM unset", "") {
		t.Fatalf("CONTROL FAILED: %s is already in the wall with GIT_CONFIG_SYSTEM unset, so neither arm below is measuring this variable", addr)
	}
	// The failing wrong arm: system scope DOES reach the wall. A pin whose
	// attack arm never fired has measured nothing.
	if !walled(t, "GIT_CONFIG_SYSTEM at a config with user.email", sys) {
		t.Fatalf("CONTROL FAILED: a user.email in system scope did not reach DeriveIdentityLiterals at all, so the pinned arm below cannot show it being dropped — `git config --get-all` may have stopped walking every scope, which is a bigger finding than this test")
	}
	// The landed pin. Silently: no error, one fewer literal.
	if walled(t, "GIT_CONFIG_SYSTEM="+os.DevNull+" (the landed pin)", os.DevNull) {
		t.Errorf("GIT_CONFIG_SYSTEM=%s no longer empties system scope for DeriveIdentityLiterals. Good news, but the GIT_CONFIG_SYSTEM row in inletpin.go now says it does — re-measure and rewrite the clause", os.DevNull)
	}
}
