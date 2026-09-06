//go:build posse_arm2

package posse

// ranger-base-eupf: a PID that denies the binary the RUNTIME reads and
// writes its own credential with shims the runtime, not the persona.
//
// THE DEFECT. The gates dir is prepended on the TYPED line (ADR 0002 §3), so
// it leads the PATH of the runtime process itself and of everything that
// process spawns — not only of the persona's shells. claude's credential
// path execs `security` by BARE NAME, so under the crew's Bash(security:*)
// the runtime's own read resolved to this persona's refusal shim. MEASURED
// in a live session 2026-08-29: `claude -p` answers "Not logged in", and the
// same line with the gates dir stripped off PATH answers normally. The gate
// logs carried 875 such refusals across the crew since 2026-08-24 and
// nothing anywhere said whose they were.
//
// Why not the narrowed carve-out the bead sketched as its other arm: the
// credential WRITE goes primarily through that binary's stdin batch mode
// (measured on the darwin release artifact, claude 2.1.258). An argv matcher
// cannot see a command that arrives on stdin, so a shim that let the write
// through to keep refresh alive would be no deny at all, and one that let
// only the reads through would fix the logged-out symptom and leave expiry
// — the half nobody has ever watched — exactly where it is. That is a
// decision about the wall, not a matcher detail.
//
// WHAT IS PINNED HERE NOW (ranger-base-te3ib, ADR 0042 D2). The decision was
// taken: the deny STAYS, because the shim in front of the runtime's read is
// what keeps the operator's rotating credential pair single-writer (D1), and
// a crew runtime authenticates with the session mint posse injects instead.
// So ranger-base-eupf's warning is retired — both of its sentences are false
// under that reading — and its place is taken by a precondition, the one ADR
// 0002 §4 already applies to a caged launch:
//
//	with the mint among the names the launch injects  → the launch says NOTHING
//	without it                                        → the launch REFUSES,
//	  naming the rule, the binary, the missing key and the mint recipe, and
//	  --allow-degraded cannot waive it (a session that cannot authenticate is
//	  not a weaker session).
//
// At BOTH renderers of a persona line, which is the half ranger-base-az23f
// found missing for the warning: planLaunch and RelaunchAgent, the unattended
// one, where nobody is present to notice a refusal that never came.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cgcRuntime is a runtime declaring cmd as its credential binary.
func cgcRuntime(name, cmd string) *Runtime { return &Runtime{Name: name, CredBin: cmd} }

// cgcBinDir is a shim dir that exists but holds nothing — resolveOutside
// needs a directory to exclude, not a rendered gate.
func cgcBinDir(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

// cgcRealBin is a command that exists on this box outside the gates, so the
// collision test's "would a shim actually stand in front of a binary" arm is
// answered by the platform rather than assumed. `sh` is the one command this
// package already requires everywhere.
func cgcRealBin(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	return "sh"
}

// The positive arm: a PID denying the runtime's credential binary collides,
// and the collision names the PID's own rule verbatim.
func TestQACredGateCollisionNamesTheRuleThatShimsTheRuntimesCredentialBinary(t *testing.T) {
	t.Parallel()
	bin := cgcRealBin(t)
	rt := cgcRuntime("testcli", bin)

	got := CredGateCollision(rt, []string{"Edit", "Bash(" + bin + ":*)"}, cgcBinDir(t))
	if got != "Bash("+bin+":*)" {
		t.Fatalf("the collision must name the PID's rule verbatim, got %q", got)
	}
}

// A whole-binary rule is the one that refuses every form, so it is the rule
// reported even when the PID also carries a narrower one written first — a
// launch warning that quoted `Bash(sh -c:*)` while `Bash(sh)` was what
// refused the runtime would send the reader to the wrong line of the PID.
func TestQACredGateCollisionPrefersTheWholeBinaryRule(t *testing.T) {
	t.Parallel()
	bin := cgcRealBin(t)
	rt := cgcRuntime("testcli", bin)

	got := CredGateCollision(rt, []string{"Bash(" + bin + " -c:*)", "Bash(" + bin + ")"}, cgcBinDir(t))
	if got != "Bash("+bin+")" {
		t.Fatalf("want the whole-binary rule, got %q", got)
	}
}

// Three silences, each of which a warning that fired anyway would turn into
// noise on every launch: a PID that denies something else, a runtime that
// declares no credential binary, and a deny over a binary this box does not
// have — the platform arm, which is what keeps a `security` rule from
// warning on a host whose runtime reads a file instead.
func TestQACredGateCollisionIsSilentWhenThereIsNothingToCollide(t *testing.T) {
	t.Parallel()
	bin := cgcRealBin(t)
	binDir := cgcBinDir(t)
	deny := []string{"Bash(" + bin + ":*)"}

	for _, c := range []struct {
		name string
		rt   *Runtime
		deny []string
	}{
		{"the PID denies another binary", cgcRuntime("testcli", bin), []string{"Bash(git push:*)", "Edit"}},
		{"the runtime declares no credential binary", cgcRuntime("testcli", ""), deny},
		{"no runtime at all", nil, deny},
		{"the binary is not on this box", cgcRuntime("testcli", "no-such-binary-anywhere-eupf"), []string{"Bash(no-such-binary-anywhere-eupf:*)"}},
	} {
		if got := CredGateCollision(c.rt, c.deny, binDir); got != "" {
			t.Errorf("%s: want no collision, got %q", c.name, got)
		}
	}
}

// The refusal's substance. Four things a reader has to be handed to act:
// the rule in the PID's own spelling (which line to look at), the binary it
// shims (why that line is the one), the key that is missing (what to put in
// the env set) and the recipe (how to get one). A refusal that named only
// the runtime would send its reader back to the wall it must not take down.
func TestQACredGateRefusalNamesTheRuleTheBinaryTheKeyAndTheRecipe(t *testing.T) {
	t.Parallel()
	rt := &Runtime{Name: "claude", CredBin: "keybin"}
	err := CredGateRefusal("ranger", rt, "Bash(keybin:*)")
	if err == nil {
		t.Fatal("the refusal must be an error")
	}
	got := err.Error()
	for _, want := range []string{"ranger", "claude", "Bash(keybin:*)", "keybin", CageCredential(rt), "setup-token", "env set", "--allow-degraded"} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal never names %q:\n%s", want, got)
		}
	}
}

// A runtime that shims its own credential binary and has no decided session
// credential cannot be launched into either — but it must not be refused
// with a sentence naming an empty key, which reads as a posse bug rather
// than as the undecided runtime it is (the CheckCageCredential branch this
// borrows from, cage.go).
func TestQACredGateRefusalSaysSoWhenNoSessionCredentialIsDecided(t *testing.T) {
	t.Parallel()
	err := CredGateRefusal("ranger", &Runtime{Name: "nocred", CredBin: "keybin"}, "Bash(keybin:*)")
	if err == nil {
		t.Fatal("an undecided runtime under the shim must still refuse")
	}
	if got := err.Error(); !strings.Contains(got, "no session credential is decided") || strings.Contains(got, "is in none of") {
		t.Errorf("want the undecided sentence, got:\n%s", got)
	}
}

// CheckCredGate is the precondition itself, asked away from any launch: the
// collision decides whether the question is asked at all, and the env-set
// names decide the answer.
func TestQACheckCredGateIsTheMintQuestionAndOnlyUnderTheShim(t *testing.T) {
	t.Parallel()
	bin := cgcRealBin(t)
	binDir := cgcBinDir(t)
	rt := &Runtime{Name: "claude", CredBin: bin}
	key := CageCredential(rt)
	deny := []string{"Bash(" + bin + ":*)"}

	if err := CheckCredGate("ranger", rt, deny, binDir, []string{"BD_ACTOR", key}); err != nil {
		t.Errorf("the mint is in the env sets, so the launch is the design and must pass: %v", err)
	}
	if err := CheckCredGate("ranger", rt, deny, binDir, []string{"BD_ACTOR"}); err == nil {
		t.Error("no mint under the shim must refuse")
	}
	// No collision, no question — a PID that does not shim the credential
	// binary has never needed a mint to launch and must not start needing
	// one, or every non-claude session on a mintless env set dies here.
	if err := CheckCredGate("ranger", rt, []string{"Bash(git push:*)"}, binDir, nil); err != nil {
		t.Errorf("a PID that shims nothing must not be asked for a mint: %v", err)
	}
	if err := CheckCredGate("ranger", &Runtime{Name: "claude"}, deny, binDir, nil); err != nil {
		t.Errorf("a runtime that declares no credential binary must not be asked for a mint: %v", err)
	}
}

// cgcCrew sets up the consumer arms: a PID carrying the crew's real deny
// over the runtime's credential binary, and the env set that PID names —
// with the session mint in it or without. It returns the runtime, and skips
// on a box where the collision cannot exist at all (no credential binary
// declared, or no such binary here), because on such a box every launch arm
// below would be measuring silence for the wrong reason.
func cgcCrew(t *testing.T, b *HerdrBackend, mint bool) *Runtime {
	t.Helper()
	a := b.App
	rt, err := a.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CredBin == "" {
		t.Skipf("%s declares no credential binary", rt.Name)
	}
	if _, err := exec.LookPath(rt.CredBin); err != nil {
		t.Skipf("%s is not on this box", rt.CredBin)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\nenvs: [crew]\ndeny: [Bash("+rt.CredBin+":*)]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "crew.env"), cgcEnvBody(rt, mint), 0o600); err != nil {
		t.Fatal(err)
	}
	return rt
}

// cgcEnvBody is that env set's contents: the mint under the name the launch
// asks for, or a set that carries something else entirely — the state the
// precondition exists to catch, and the one an operator reaches by editing
// the set rather than by never having minted.
func cgcEnvBody(rt *Runtime, mint bool) []byte {
	if !mint {
		return []byte("FOO=bar\n")
	}
	return []byte(CageCredential(rt) + "=sk-ant-oat01-TEST\n")
}

// The consumer, arm 1 (ADR 0042 verification 1). An ordinary crew launch:
// the PID carries the deny over the runtime's credential binary and the env
// set carries the mint. That is the design, so the launch happens and says
// NOTHING about it — the polarity ranger-base-eupf's launch pin had, flipped
// by ADR 0042 D2. The arm that fails if a refusal fires unconditionally.
func TestQALaunchIsSilentWhenTheShimmedPIDCarriesTheMint(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	rt := cgcCrew(t, b, true)

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("a PID that shims the credential binary WITH the mint in its env set is the design and must launch: %v", err)
	}
	if got := warn.String(); strings.Contains(got, rt.CredBin) || strings.Contains(got, "ADR 0042") {
		t.Fatalf("the launch must say nothing about the collision it was built to carry:\n%s", got)
	}
}

// The consumer, arm 2 (ADR 0042 verification 2). The same PID, an env set
// without the mint: refused, naming the rule, the binary and the key — and
// no session is created. Two things a warning could never do, which is why
// this is the arm that goes red if the refusal is dropped.
func TestQALaunchRefusesTheShimmedPIDWithNoMintAndCreatesNoSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	rt := cgcCrew(t, b, false)
	key := CageCredential(rt)

	err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"})
	if err == nil {
		t.Fatal("a launch whose PID shims the runtime's credential binary with no mint in its env sets must be refused")
	}
	for _, want := range []string{"Bash(" + rt.CredBin + ":*)", rt.CredBin, key, "env set"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal never names %q:\n%v", want, err)
		}
	}
	if _, ok := b.readMeta("s1"); ok {
		t.Error("the refused launch left a session meta behind")
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("the refused launch created a workspace:\n%s", log)
	}
	// Not waivable (ADR 0042 D2): --allow-degraded is for a gate the wall
	// could not realize, and this is a session that cannot authenticate.
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: t.TempDir(), Agent: "ranger", AllowDegraded: true}); err == nil {
		t.Error("--allow-degraded must not waive the credential precondition")
	}
}

// The consumer, arm 3: the silence that keeps arm 1 honest from the other
// side. A PID that does not carry the deny launches with no mint anywhere —
// the shape of every non-crew session on this box — and must not be dragged
// into the precondition by a check that forgot to ask the collision first.
func TestQALaunchIsQuietWhenThePIDDoesNotShimTheCredentialBinary(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	rt, err := a.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CredBin == "" {
		t.Skipf("%s declares no credential binary", rt.Name)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndeny: [Bash(git push:*)]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("a PID that shims nothing of the runtime's must launch with no mint at all: %v", err)
	}
	if got := warn.String(); strings.Contains(got, rt.CredBin) {
		t.Fatalf("a PID that does not deny %s must not draw a credential-gate line:\n%s", rt.CredBin, got)
	}
}

// The OTHER renderer (ranger-base-az23f). RelaunchAgent re-types a persona
// command into a pane that is already open, and it is the UNATTENDED path
// (dispatch.launchSession) — the one where a crashed CLI comes back with
// nobody watching. It re-reads the PID and re-reads the env sets by name, so
// a mint edited out of the env set after the session opened arrives here
// first, and without this arm the revived session would be one that cannot
// authenticate, silently.
//
// The witness arm first: with the mint in place this rig really does
// re-type, so the refusal below is the missing key's doing and not a
// relaunch that was never going to happen.
func TestQARelaunchRefusesTheShimmedPIDWhenTheMintIsGone(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	rt := cgcCrew(t, b, true)
	if err := b.CreateSession(NewSessionOpts{Name: "rc", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("the launch with the mint must succeed: %v", err)
	}

	died := func() {
		t.Helper()
		m, _ := b.readMeta("rc")
		m.Launched = time.Now().Add(-time.Hour)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		os.Remove(filepath.Join(fake, "agents.json")) // the CLI died
	}
	died()
	if ok, err := b.RelaunchAgent("rc", time.Second); err != nil || !ok {
		t.Fatalf("witness: a session whose env set carries the mint must re-type: ok=%v err=%v", ok, err)
	}

	// The operator edits the mint out of the env set the meta names — the
	// state the launch would have refused, reached after the launch.
	if err := os.WriteFile(filepath.Join(b.App.EnvsDir, "crew.env"), cgcEnvBody(rt, false), 0o600); err != nil {
		t.Fatal(err)
	}
	died()
	ok, err := b.RelaunchAgent("rc", time.Second)
	if err == nil {
		t.Fatalf("a relaunch that re-renders the shim with no mint must be refused exactly as the launch was: ok=%v", ok)
	}
	for _, want := range []string{"Bash(" + rt.CredBin + ":*)", CageCredential(rt)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the relaunch refusal never names %q:\n%v", want, err)
		}
	}
}

// rt0 is the default runtime, loaded on its own for the arms that ask about
// the runtime table rather than about a launch.
func rt0(t *testing.T, b *HerdrBackend) *Runtime {
	t.Helper()
	rt, err := b.App.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// The twin of the widened rule in posse's own output. `posse runtime check`
// answered the credential question with "this is the container's credential,
// not the runtime's" — true until ADR 0042 D2 made the same key a
// precondition at `shims` for a runtime that declares a credential binary.
// A row still saying "every other cage tier is unaffected" is how the
// refusal below reads as a posse bug to the one person who went looking.
func TestQACageCredRowSaysTheShimsTierNeedsTheMintToo(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	rt := rt0(t, b)
	if rt.CredBin == "" {
		t.Skipf("%s declares no credential binary", rt.Name)
	}
	got := cageCredRow(rt)
	for _, want := range []string{rt.CredBin, "ADR 0042"} {
		if !strings.Contains(got.value+got.missing, want) {
			t.Errorf("the cage_cred row never names %q:\n%s\n%s", want, got.value, got.missing)
		}
	}
	if strings.Contains(got.missing, "Every other cage tier is unaffected") {
		t.Errorf("the row still says the shims tier is unaffected:\n%s", got.missing)
	}
	// The control: a runtime with no credential binary is untouched by ADR
	// 0042, and its row must keep the container-only sentence rather than
	// telling its reader to mint for a shim that will never be rendered.
	plain := cageCredRow(&Runtime{Name: "plain", CageCred: "PLAIN_TOKEN"})
	if !strings.Contains(plain.missing, "Every other cage tier is unaffected") {
		t.Errorf("a runtime with no credential binary must keep the container-only sentence:\n%s", plain.missing)
	}
}

// The declaration itself is the measurement's only home, so it is pinned
// where a reader looks for it: the default runtime names the binary its own
// credential path execs. A build that drops it turns every launch quiet
// again with nothing failing.
func TestQADefaultRuntimeDeclaresItsCredentialBinary(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	rt, err := b.App.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CredBin != "security" {
		t.Fatalf("%s declares CredBin %q — measured on the darwin release artifact it is `security` (ranger-base-eupf)", rt.Name, rt.CredBin)
	}
}
