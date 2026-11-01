//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0077 D1, surface 1 (ranger-base-49v2h): the typed runtime line pins
// the runtime-home variable posse resolved, in both arms, so the seat reads
// the file posse seeded.
//
// WHAT MAKES THIS PINNABLE WITHOUT A BILLED LAUNCH. The failure these pins
// are about is not "posse wrote the wrong key" — the key was right
// throughout c33j6 — it is "posse wrote the right thing into a file the seat
// does not open". That is two claims, and both are readable here:
//
//  1. the LINE carries the variable in posse's own arm, on every path that
//     types a runtime line (launch and relaunch), and
//  2. a process whose environment is that line's environment resolves the
//     file posse seeded — asked by actually running the fragment under
//     `sh -c` against a DECOY value, which is the only way to tell an
//     effective assignment from a string that looks like one.
//
// What is left for the live verify, and is said on the bead rather than
// implied by a green here: that the seat's claude then opens with no trust
// dialog. These pins cover posse's half of that.

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// homeVarRuntimes is the three built-ins with a home override, paired with
// the resolver posse reads that same root through. The pin and the resolver
// are asserted against each other, which is the "one resolver, two
// consumers" clause: a pin naming a variable no posse-side reading follows
// would be a lever that reads as connected and is not.
func homeVarRuntimes(t *testing.T) []struct {
	runtime string
	v       string
	resolve func(home string) string
} {
	t.Helper()
	return []struct {
		runtime string
		v       string
		resolve func(home string) string
	}{
		{"claude", ClaudeConfigDirVar, ClaudeConfigDirIn},
		{"codex", CodexHomeVar, codexHomeIn},
		{"grok", GrokHomeVar, grokHomeIn},
	}
}

func loadRuntimeForTest(t *testing.T, name string) *Runtime {
	t.Helper()
	rt, err := (&App{}).LoadRuntime(name)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// Arm 1: the fragment itself, both arms, against the resolver beside it.
//
// The empty arm is the one worth having and the one prose cannot hold:
// posse treats a set-but-EMPTY override as unset everywhere, so the pin must
// say UNSET too. Typing the empty value instead would hand claude a
// cwd-relative config directory (ClaudeConfigDirIn's measured `??` vs `||`
// divergence) — the launch resolving a path under whatever the pane's cwd
// happens to be, which is ranger-base-58b5's shape with a seat in it.
func TestRuntimeHomePinIsTheArmPosseResolvedIn(t *testing.T) {
	home := t.TempDir()
	for _, tc := range homeVarRuntimes(t) {
		rt := loadRuntimeForTest(t, tc.runtime)
		if rt.HomeVar != tc.v {
			t.Fatalf("fixture: %s declares HomeVar %q, want %q — this table and the runtime disagree about which variable moves this CLI's root", tc.runtime, rt.HomeVar, tc.v)
		}

		named := filepath.Join(t.TempDir(), "named-root")
		t.Setenv(tc.v, named)
		if got, want := RuntimeHomePin(rt), tc.v+"="+shQuote(named)+" "; got != want {
			t.Errorf("%s with %s in force: pin = %q, want %q", tc.runtime, tc.v, got, want)
		}
		if got := tc.resolve(home); got != named {
			t.Errorf("fixture: %s's resolver answered %q, not the override %q — the pin and the resolver must be reading one variable", tc.runtime, got, named)
		}

		// Unset, and set-but-empty, are ONE answer: `env -u`.
		for _, how := range []string{"unset", "empty"} {
			if how == "unset" {
				unsetenvForTest(t, tc.v)
			} else {
				t.Setenv(tc.v, "")
			}
			want := "/usr/bin/env -u " + tc.v + " "
			got := RuntimeHomePin(rt)
			if got != want {
				t.Errorf("%s with %s %s: pin = %q, want %q", tc.runtime, tc.v, how, got, want)
			}
			if strings.Contains(got, tc.v+"=") {
				t.Errorf("%s with %s %s: pin ASSIGNS the variable (%q) — an empty value is a third state the runtime reads differently from unset, and typing it sends claude's config dir to the cwd", tc.runtime, tc.v, how, got)
			}
			if got := tc.resolve(home); got == "" || got == named {
				t.Errorf("fixture: %s's resolver with %s %s answered %q — it must have fallen back to the home, or `env -u` is pinning agreement with nothing", tc.runtime, tc.v, how, got)
			}
		}
	}
}

// A runtime with no home override renders NOTHING, and bob is the measured
// instance of it rather than a hypothetical: bobHomeIn is the one resolver
// of the three with no env branch at all (25 distinct BOB_* names in the
// bundle, none a home — ranger-base-9k9ff, re-read on 2.0.9). A nil runtime
// renders nothing too, because WrapWithGates documents rt as nillable.
func TestRuntimeHomePinIsSilentWhereThereIsNoOverrideToPin(t *testing.T) {
	t.Parallel()
	if got := RuntimeHomePin(nil); got != "" {
		t.Errorf("RuntimeHomePin(nil) = %q, want \"\" — WrapWithGates takes a nil runtime", got)
	}
	rt, err := (&App{}).LoadRuntime("bob")
	if err != nil {
		t.Fatal(err)
	}
	if rt.HomeVar != "" {
		t.Errorf("bob declares HomeVar %q — bob has no home override (bobHomeIn has no env branch), so pinning a name would be pinning a variable nothing on either side reads", rt.HomeVar)
	}
	if got := RuntimeHomePin(rt); got != "" {
		t.Errorf("RuntimeHomePin(bob) = %q, want \"\"", got)
	}
}

// Arm 2: the fragment RUNS, and does what it says, against a DECOY already
// in the environment. This is the half a string comparison cannot reach —
// `CLAUDE_CONFIG_DIR=/x ` and `/usr/bin/env -u CLAUDE_CONFIG_DIR ` are both
// just text until a shell parses them in front of a command, and the whole
// point is the value the SEAT's process ends up with.
//
// The decoy stands in for the multiplexer server's frozen environment: the
// variable is already set to something else when the line is typed, which is
// exactly c33j6's board one direction over.
func TestTheHomePinOverridesWhatTheSeatsEnvironmentAlreadyCarried(t *testing.T) {
	for _, tc := range homeVarRuntimes(t) {
		rt := loadRuntimeForTest(t, tc.runtime)
		decoy := filepath.Join(t.TempDir(), "the-servers-frozen-answer")

		// In force: the assignment wins over the decoy.
		named := filepath.Join(t.TempDir(), "what-posse-resolved")
		t.Setenv(tc.v, named)
		out, err := runWithEnv(t, RuntimeHomePin(rt)+"printenv "+tc.v, tc.v+"="+decoy)
		if err != nil {
			t.Errorf("%s: the pin did not run: %v (%s)", tc.runtime, err, out)
		} else if got := strings.TrimSpace(out); got != named {
			t.Errorf("%s: a seat whose environment carried %s=%s read %q through the pin, want %q", tc.runtime, tc.v, decoy, got, named)
		}

		// Not in force: `env -u` must leave the seat with NO value, not
		// with the decoy and not with an empty string.
		unsetenvForTest(t, tc.v)
		out, err = runWithEnv(t, RuntimeHomePin(rt)+"printenv "+tc.v, tc.v+"="+decoy)
		if err == nil {
			t.Errorf("%s: `printenv %s` succeeded through the unset arm and printed %q — the variable survived, so the seat resolves the server's answer and not posse's", tc.runtime, tc.v, strings.TrimSpace(out))
		}
	}
}

// And the composition: GatePrefix's assignments reach through the `env -u`
// arm. They are prefix assignments on `/usr/bin/env` rather than on the
// runtime, so if `env` did not export them the seat would lose its PATH wall
// in exactly the arm ADR 0077 D1 added — a silent L1 bypass bought with a
// trust fix (ADR 0002 §3, ADR 0009 §1).
func TestTheGatePrefixSurvivesTheUnsetArmOfTheHomePin(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	rt := loadRuntimeForTest(t, "claude")
	unsetenvForTest(t, rt.HomeVar)
	_, bin, shell, err := a.RenderGates("pinprobe", []string{"Bash(git push:*)"})
	if err != nil {
		t.Fatal(err)
	}
	pin := RuntimeHomePin(rt)
	if !strings.HasPrefix(pin, "/usr/bin/env -u ") {
		t.Fatalf("fixture: this arm is about the `env -u` shape, got %q", pin)
	}
	out, err := runWithEnv(t, GatePrefix(bin, shell)+pin+"printenv PATH")
	if err != nil {
		t.Fatalf("the prefix did not run: %v (%s)", err, out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), bin+string(os.PathListSeparator)) {
		t.Errorf("PATH through the pin = %q — the gates dir must still LEAD it, or the shims lose the lookup race they exist to win", strings.TrimSpace(out))
	}
	out, err = runWithEnv(t, GatePrefix(bin, shell)+pin+"printenv SHELL")
	if err != nil {
		t.Fatalf("the prefix did not run: %v (%s)", err, out)
	}
	if got := strings.TrimSpace(out); got != shell {
		t.Errorf("SHELL through the pin = %q, want the gate shell %q (ADR 0009 §2)", got, shell)
	}
}

// runWithEnv runs a shell fragment and returns its stdout, with `extra`
// overlaid on this process's environment. `sh` and not the gate shell: what
// is under test is POSIX prefix-assignment semantics, which is the contract
// the typed line rests on whatever the pane's shell turns out to be.
func runWithEnv(t *testing.T, script string, extra ...string) (string, error) {
	t.Helper()
	c := exec.Command("sh", "-c", script)
	c.Env = append(os.Environ(), extra...)
	b, err := c.CombinedOutput()
	return string(b), err
}

// Arm 3: the bead's own "done when", on both paths that type a runtime line.
//
// A launch from a process with CLAUDE_CONFIG_DIR=X seeds X/.claude.json and
// types a line on which claude resolves THAT file; a relaunch re-types the
// same pin. The resolution is not asserted by spelling the path a second
// time — it is asked of ClaudeConfigFile() under the environment the line
// builds, which is the one resolver claude's own rule is transcribed into.
func TestEveryTypedRuntimeLineCarriesTheHomePinPosseSeededUnder(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(ClaudeConfigDirVar, configDir)

	b, fake := newTestBackend(t)
	a := b.App
	// Resolved the way NewHerdrBackend resolves it, so the file this launch
	// seeds is the file the launcher's own environment names — which is the
	// fact the pin is about. Spelling the path here instead would pin the
	// test's arithmetic rather than posse's.
	b.ClaudeConfig = ClaudeConfigFile()
	if want := filepath.Join(configDir, ".claude.json"); b.ClaudeConfig != want {
		t.Fatalf("fixture: ClaudeConfigFile() = %q under CLAUDE_CONFIG_DIR=%s, want %q", b.ClaudeConfig, configDir, want)
	}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\ndeny: [Bash(git push:*)]\n---\nYou are developer.\n"), 0o644)

	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "d1", Agent: "developer", Dir: dir})
	if !ClaudeTrusted(readConfig(t, b.ClaudeConfig), dir) {
		t.Fatalf("fixture: the launch seeded nothing into %s", b.ClaudeConfig)
	}

	rt := loadRuntimeForTest(t, "claude")
	pin := RuntimeHomePin(rt)
	if pin != ClaudeConfigDirVar+"="+shQuote(configDir)+" " {
		t.Fatalf("fixture: pin = %q under a set CLAUDE_CONFIG_DIR", pin)
	}
	_, bin, shell, err := a.RenderGates("developer", []string{"Bash(git push:*)"})
	if err != nil {
		t.Fatal(err)
	}
	// Whole, in one piece, and IN ORDER: the pin sits immediately after the
	// gate prefix, so the prefix's assignments are in force for it and
	// `/usr/bin/env` — the other arm at this same position — is the command
	// they apply to.
	want := GatePrefix(bin, shell) + pin
	launched := rawCalls(t, fake) + spilled(t, a, "d1")
	if !strings.Contains(launched, want) {
		t.Errorf("the launch line must carry the gate prefix and the home pin in one piece:\nwant %q\n%s", want, launched)
	}

	// The claim the pin exists for: a process whose environment is this
	// line's resolves the file posse seeded.
	out, err := runWithEnv(t, pin+"printenv "+ClaudeConfigDirVar, ClaudeConfigDirVar+"="+filepath.Join(t.TempDir(), "the-servers-answer"))
	if err != nil {
		t.Fatalf("the pin did not run: %v (%s)", err, out)
	}
	t.Setenv(ClaudeConfigDirVar, strings.TrimSpace(out))
	if got := ClaudeConfigFile(); got != b.ClaudeConfig {
		t.Errorf("under the environment the typed line builds, claude's config file resolves to %q — posse seeded %q. That gap IS ranger-base-c33j6: a grant written into a file the seat never opens.", got, b.ClaudeConfig)
	}
	t.Setenv(ClaudeConfigDirVar, configDir)

	// RelaunchAgent renders its own line (it trusts nothing in the meta),
	// so it has to re-type the same pin — the one path that types into a
	// pane that already exists.
	os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("d1")
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	if ok, err := b.RelaunchAgent("d1", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	retyped := rawCalls(t, fake) + spilled(t, a, "d1")
	if !strings.Contains(retyped, want) {
		t.Errorf("the relaunch must re-type the home pin verbatim:\nwant %q\n%s", want, retyped)
	}
}

// The other arm of the same launch: with the variable not in force, the line
// UNSETS it. Without this a seat dispatched from an operator shell would
// inherit whatever the multiplexer server was started with — which on this
// box is another seat's CLAUDE_CONFIG_DIR, i.e. the c33j6 divergence with
// the two files swapped.
func TestATypedRuntimeLineUnsetsTheHomeVarWhenPosseHasNone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetenvForTest(t, ClaudeConfigDirVar)

	b, fake := newTestBackend(t)
	a := b.App
	b.ClaudeConfig = ClaudeConfigFile()
	if want := filepath.Join(home, ".claude.json"); b.ClaudeConfig != want {
		t.Fatalf("fixture: ClaudeConfigFile() = %q with no override, want %q", b.ClaudeConfig, want)
	}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\ndeny: [Bash(git push:*)]\n---\nYou are developer.\n"), 0o644)
	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "d2", Agent: "developer", Dir: dir})

	launched := rawCalls(t, fake) + spilled(t, a, "d2")
	if !strings.Contains(launched, "/usr/bin/env -u "+ClaudeConfigDirVar+" ") {
		t.Errorf("the line must UNSET %s when posse has none:\n%s", ClaudeConfigDirVar, launched)
	}
	if strings.Contains(launched, ClaudeConfigDirVar+"=") {
		t.Errorf("the line ASSIGNS %s where posse resolved without it — an empty or guessed value is a different config file:\n%s", ClaudeConfigDirVar, launched)
	}
}

// ─── surface 1's second half: the refusal when the seed cannot be written ────

// ADR 0077 D1: "A caged launcher with the var unset still cannot write
// $HOME; that refusal stays, and its message names the cage and the two ways
// out instead of a bare `operation not permitted`."
//
// MEASURED (ranger-base-c33j6's first attempt): shedding the variable so
// posse targeted $HOME/.claude.json made the `.posse-seed-*` temp file in
// $HOME a write the launcher's own seatbelt refused. The bare error names a
// filename posse invented, in a directory the operator did not name, from a
// launch that mentioned neither.
func TestTheTrustSeedsWriteRefusalNamesTheCageAndTheTwoWaysOut(t *testing.T) {
	dir := t.TempDir()
	cfgDir := t.TempDir()
	cfg := filepath.Join(cfgDir, ".claude.json")
	// THE FIXTURE IS THE CAGE'S OWN SHAPE, not a blunter one. A caged seat's
	// L2 profile grants the runtime's declared state files AND their
	// atomic-write siblings — `<p>.` plus anything, SeatbeltSiblings — so
	// `~/.claude.json` and `~/.claude.json.posse-lock` are both writable
	// there and the lock is NOT the refused write. What is left outside
	// every grant is `$HOME/.posse-seed-*`, a name in the config's DIRECTORY
	// rather than beside its file, which is exactly what c33j6 measured.
	//
	// Reproduced by making the directory unwritable with both of those files
	// already in it: opening an existing file for write needs no write
	// permission on its directory, so the lock still takes and CREATING the
	// temp file is the one call that fails. A plain chmod with an empty
	// directory stages the lock failure instead and measures the wrong
	// sentence (which is how this comment came to be written).
	if err := os.WriteFile(cfg, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeConfigLockFile(cfg), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cfgDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cfgDir, 0o700) })
	// Staged, not assumed: a user who can write a mode-0500 directory (root
	// in a container being the case that matters on CI) cannot reach this
	// arm at all, and a test that silently measured the success path would
	// be worse than one that says so.
	if f, err := os.CreateTemp(cfgDir, "probe-"); err == nil {
		f.Close()
		os.Remove(f.Name())
		t.Skip("this user can write a mode-0500 directory, so the permission refusal cannot be staged here")
	}

	t.Setenv("RHQ_CAGE", CageSeatbelt)
	t.Setenv(EnvPersona, "developer")
	_, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err == nil {
		t.Fatal("an unwritable config dir must refuse the launch, not report a seed")
	}
	msg := err.Error()
	for _, want := range []string{
		".posse-seed-*",            // WHICH write was refused
		"RHQ_CAGE=" + CageSeatbelt, // the cage, named
		"developer",                // whose seat it is
		"two ways out",
		"CLAUDE_CONFIG_DIR", // way out 1
		"uncaged shell",     // way out 2
		"accept the dialog by hand",
		dir, // the directory to accept it IN
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, msg)
		}
	}

	// And the cage sentence is conditional on the cage being READABLE. A
	// confident wrong diagnosis is worse than the bare errno, so posse does
	// not assert a cage it cannot see — the two ways out stay either way.
	unsetenvForTest(t, "RHQ_CAGE")
	_, err = SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err == nil {
		t.Fatal("fixture: the second call did not fail")
	}
	if msg := err.Error(); strings.Contains(msg, "caged seat") {
		t.Errorf("with no RHQ_CAGE in posse's own environment the refusal must not claim a cage:\n%s", msg)
	} else if !strings.Contains(msg, "two ways out") {
		t.Errorf("the two ways out are not conditional on the cage:\n%s", msg)
	}
}

// Nor does it claim a cage for a failure that is not a permission refusal —
// a disk-full, a read-only volume, or this arm's ENOTDIR. Same reason: the
// sentence is a diagnosis, and posse makes it only on the two facts it has.
func TestTheTrustSeedsWriteRefusalClaimsNoCageForANonPermissionFailure(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(notADir, ".claude.json")

	t.Setenv("RHQ_CAGE", CageSeatbelt)
	_, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err == nil {
		t.Fatal("fixture: writing under a regular file must fail")
	}
	if errors.Is(err, fs.ErrPermission) {
		t.Skip("this platform reports ENOTDIR as a permission error, so the arm cannot be staged")
	}
	if msg := err.Error(); strings.Contains(msg, "caged seat") {
		t.Errorf("a non-permission failure must not be diagnosed as a cage:\n%s", msg)
	}
}

// ─── surface 1's RELATIVE arm (ranger-base-sczjy, from ranger-base-49iga) ────

// Arm 4: a relative value in a runtime-home variable is made ABSOLUTE at the
// one read, so both directions carry one path.
//
// WHY EVERY ARM ABOVE MISSES THIS. They are not wrong about what they
// measure; they measure the ABSOLUTE arm, because every fixture on this path
// names its directory with t.TempDir(). A verbatim value keeps the pin's
// guarantee there and inverts it for a relative one: the launcher resolves
// the value against ITS cwd and the seat resolves the same string against
// the session dir, so posse seeds one file and the seat opens another —
// ranger-base-c33j6's own shape, surviving the fix built to end it.
//
// MEASURED 2026-10-31 before the fix, by a QA seat verifying
// ranger-base-49v2h at 8077fef3, with CLAUDE_CONFIG_DIR=relcfg:
//
//	ClaudeConfigFile()  = "relcfg/.claude.json"   (IsAbs=false)
//	the seed LANDED at    <launcher cwd>/relcfg/.claude.json
//	typed pin           = "CLAUDE_CONFIG_DIR='relcfg' "
//	the seat resolved     <session dir>/relcfg/.claude.json   exists=false
//
// A launch that completes, a line correct by the pin's own assertion, a
// grant that is written, and no trust dialog suppressed.
//
// All THREE runtimes, because runtimeHomeOverride is one function and
// codexHomeIn/grokHomeIn took the same value verbatim — which is also where
// this class was last paid for one rung down, as a relative
// CODEX_HOME/GROK_HOME reaching the credential wall (ranger-base-o05yg,
// credentialDenyable).
func TestARelativeRuntimeHomeIsMadeAbsoluteAtTheOneRead(t *testing.T) {
	launcherCwd := t.TempDir()
	t.Chdir(launcherCwd)
	// os.Getwd and not launcherCwd: on darwin a path under $TMPDIR has a
	// /private prefix in its resolved spelling, and what filepath.Abs joins
	// onto is whatever Getwd answers — so this is the cwd posse itself
	// resolves against, asked rather than assumed.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()

	for _, tc := range homeVarRuntimes(t) {
		rt := loadRuntimeForTest(t, tc.runtime)

		// The spellings a shell export actually takes, including the two
		// that Clean changes and the one it does not.
		for _, rel := range []string{"relcfg", "./relcfg", "a/b", "relcfg/"} {
			t.Setenv(tc.v, rel)
			want := filepath.Join(wd, rel)

			got := tc.resolve(home)
			if !filepath.IsAbs(got) {
				t.Errorf("%s with %s=%q: the resolver answered the RELATIVE path %q — posse joins a file name onto this and then types the variable onto a seat with a different cwd", tc.runtime, tc.v, rel, got)
			}
			if got != want {
				t.Errorf("%s with %s=%q: the resolver answered %q, want %q (the value resolved against the LAUNCHER's cwd, which is the one posse read it in)", tc.runtime, tc.v, rel, got, want)
			}

			pin := RuntimeHomePin(rt)
			if w := tc.v + "=" + shQuote(want) + " "; pin != w {
				t.Errorf("%s with %s=%q: pin = %q, want %q", tc.runtime, tc.v, rel, pin, w)
			}
			// Said the other way round too, so the failure names the bug
			// rather than an arithmetic mismatch: the relative string the
			// operator exported must not be what reaches the seat.
			if pin == tc.v+"="+shQuote(rel)+" " {
				t.Errorf("%s: the pin types %s=%q VERBATIM — the seat resolves that against the SESSION dir, so posse seeds one file and the seat opens another (ranger-base-c33j6's shape, ranger-base-sczjy)", tc.runtime, tc.v, rel)
			}
			// The whole claim, in the form the two directions have to agree
			// in: one path, from the resolver and from the line.
			if pin != tc.v+"="+shQuote(got)+" " {
				t.Errorf("%s with %s=%q: the resolver reads %q and the line types %q — one variable, two answers", tc.runtime, tc.v, rel, got, pin)
			}
		}

		// CONTROL: an ABSOLUTE value is handed on BYTE-IDENTICAL — not
		// Cleaned, not symlink-resolved. That is the arm the whole pin rests
		// on (the operator's own spelling is what the seat must receive, and
		// shQuote already carries six hostile values through /bin/sh
		// unchanged), and it is why the fix is gated on filepath.IsAbs
		// rather than calling filepath.Abs unconditionally.
		for _, abs := range []string{
			filepath.Join(t.TempDir(), "named-root"),
			filepath.Join(t.TempDir(), "named-root") + "/",
			filepath.Join(t.TempDir(), "named-root") + "/./sub",
			filepath.Join(t.TempDir(), "two words"),
		} {
			t.Setenv(tc.v, abs)
			if got := tc.resolve(home); got != abs {
				t.Errorf("%s with an ABSOLUTE %s=%q: the resolver answered %q — an absolute value is the operator's own spelling and posse must not rewrite it", tc.runtime, tc.v, abs, got)
			}
			if got, want := RuntimeHomePin(rt), tc.v+"="+shQuote(abs)+" "; got != want {
				t.Errorf("%s with an ABSOLUTE %s=%q: pin = %q, want %q", tc.runtime, tc.v, abs, got, want)
			}
		}
	}
}

// Arm 4b: the same rule for CLAUDE_SECURESTORAGE_CONFIG_DIR, which is read
// in credentialDirNamed and NOT through runtimeHomeOverride (its set-and-empty
// arm shadows CLAUDE_CONFIG_DIR, and runtimeHomeOverride's rule is v != "").
//
// Before ranger-base-8sxf5 that read returned the value verbatim, so a
// relative value was typed onto the seat as-is (credentialDirPin ->
// credentialDirPinJSON), resolved there against the SESSION dir while posse
// resolved it against its own cwd, and credentialDenyable dropped it — no
// seatbelt read-deny over the directory the seat's CLI writes its token into.
//
// All four readings are asserted because each is a different consumer of
// the one read: the resolver, the file posse reads, the wall, and the line.
func TestARelativeSecureStorageDirIsMadeAbsoluteAtCredentialDirNamed(t *testing.T) {
	launcherCwd := t.TempDir()
	t.Chdir(launcherCwd)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(ClaudeConfigDirVar, "")

	pinnedSec := func(t *testing.T) string {
		t.Helper()
		var payload struct {
			Env map[string]string `json:"env"`
		}
		raw := credentialDirPinJSON()
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatalf("credentialDirPinJSON is not JSON: %v: %s", err, raw)
		}
		v, ok := payload.Env["CLAUDE_SECURESTORAGE_CONFIG_DIR"]
		if !ok {
			t.Fatalf("credentialDirPinJSON carries no CLAUDE_SECURESTORAGE_CONFIG_DIR: %s", raw)
		}
		return v
	}

	check := func(t *testing.T, value, want string) {
		t.Helper()
		t.Setenv("CLAUDE_SECURESTORAGE_CONFIG_DIR", value)

		dir, named, err := credentialDirNamed()
		if err != nil {
			t.Fatal(err)
		}
		if !named {
			t.Errorf("%q: credentialDirNamed says no variable named the directory", value)
		}
		if !filepath.IsAbs(dir) {
			t.Errorf("%q: credentialDirNamed answered the RELATIVE %q — the seat resolves that against the session dir, posse against its own cwd (ranger-base-8sxf5)", value, dir)
		}
		if dir != want {
			t.Errorf("%q: credentialDirNamed = %q, want %q", value, dir, want)
		}

		file, err := CredentialsFile()
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(file) {
			t.Errorf("%q: CredentialsFile = %q, not absolute", value, file)
		}
		if !credentialDenyable(file) {
			t.Errorf("%q: credentialDenyable(%q) is false — the seatbelt read-deny names nothing over the directory the seat writes its token into", value, file)
		}

		if got := pinnedSec(t); got != want {
			t.Errorf("%q: the settings pin types CLAUDE_SECURESTORAGE_CONFIG_DIR=%q, want %q (the path the wall names)", value, got, want)
		}
	}

	for _, rel := range []string{"relsec", "./relsec", "a/b", "relsec/"} {
		check(t, rel, filepath.Join(wd, rel))
	}

	// CONTROL: an absolute value reaches all four byte-identical — not
	// Cleaned, which is why the fix is gated on filepath.IsAbs.
	for _, abs := range []string{
		filepath.Join(t.TempDir(), "sec"),
		filepath.Join(t.TempDir(), "sec") + "/",
		filepath.Join(t.TempDir(), "sec") + "/./sub",
	} {
		check(t, abs, abs)
	}

	// CONTROL: set-and-EMPTY still shadows CLAUDE_CONFIG_DIR and is not
	// named — the arm that keeps this read out of runtimeHomeOverride.
	t.Setenv(ClaudeConfigDirVar, filepath.Join(t.TempDir(), "cfg"))
	t.Setenv("CLAUDE_SECURESTORAGE_CONFIG_DIR", "")
	home, _ := os.UserHomeDir()
	dir, named, err := credentialDirNamed()
	if err != nil {
		t.Fatal(err)
	}
	if named || dir != filepath.Join(home, ".claude") {
		t.Errorf("set-and-empty: credentialDirNamed = (%q, %v), want (%q, false)", dir, named, filepath.Join(home, ".claude"))
	}
}

// Arm 5: the same thing end to end, which is the observable the bead was
// filed for — a seat whose cwd is its session dir opens the very file the
// launch seeded, with a relative CLAUDE_CONFIG_DIR in the launcher's
// environment.
//
// This is that QA probe with the logs turned into assertions, plus one
// thing it did not do: the relative directory is NOT pre-created here, so
// the arm also covers the bead's second outcome. writeJSONInPlace MkdirAll's
// the config file's directory, and before the fix the directory it created
// was `<launcher cwd>/relcfg` while the typed line sent the seat looking
// under `<session dir>/relcfg` — and where the launcher could not create it,
// claudeTrustWriteRefusal's remedy ("set CLAUDE_CONFIG_DIR to a directory
// this launcher can write") was advice a relative value satisfies while
// reproducing the divergence.
//
// The seat's half is asked and not spelled: the pin fragment is RUN, over a
// decoy value, and ClaudeConfigFile() is then called from the SESSION dir —
// the two things that differ between the launcher and the seat, environment
// and cwd, both taken from the line.
func TestTheTrustSeedLandsWhereASeatWithARelativeConfigDirResolvesIt(t *testing.T) {
	launcherCwd := t.TempDir()
	t.Chdir(launcherCwd)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(ClaudeConfigDirVar, "relcfg")

	b, fake := newTestBackend(t)
	a := b.App
	b.ClaudeConfig = ClaudeConfigFile()
	if !filepath.IsAbs(b.ClaudeConfig) {
		t.Fatalf("ClaudeConfigFile() = %q under a RELATIVE %s — every path posse joins onto this is relative to whatever cwd the reader has, and posse and the seat do not share one", b.ClaudeConfig, ClaudeConfigDirVar)
	}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\ndeny: [Bash(git push:*)]\n---\nYou are developer.\n"), 0o644)

	dir := t.TempDir() // the SEAT's cwd: its session dir
	mustCreate(t, b, NewSessionOpts{Name: "d1", Agent: "developer", Dir: dir})
	if _, err := os.Stat(b.ClaudeConfig); err != nil {
		t.Fatalf("the launch seeded no file at %s: %v", b.ClaudeConfig, err)
	}
	if !ClaudeTrusted(readConfig(t, b.ClaudeConfig), dir) {
		t.Fatalf("fixture: the launch seeded nothing for %s into %s", dir, b.ClaudeConfig)
	}

	rt := loadRuntimeForTest(t, "claude")
	pin := RuntimeHomePin(rt)
	launched := rawCalls(t, fake) + spilled(t, a, "d1")
	if !strings.Contains(launched, pin) {
		t.Fatalf("the launch line must carry the home pin %q:\n%s", pin, launched)
	}

	// The seat's environment, built by actually running the fragment against
	// a decoy — the only way to tell an effective assignment from a string
	// that looks like one.
	out, err := runWithEnv(t, pin+"printenv "+ClaudeConfigDirVar,
		ClaudeConfigDirVar+"="+filepath.Join(t.TempDir(), "the-servers-answer"))
	if err != nil {
		t.Fatalf("the pin did not run: %v (%s)", err, out)
	}
	t.Setenv(ClaudeConfigDirVar, strings.TrimSpace(out))
	t.Chdir(dir) // and the seat's cwd, which is the half that diverged

	got := ClaudeConfigFile()
	if got != b.ClaudeConfig {
		t.Errorf("from the session dir %s, under the environment the typed line builds, claude's config file resolves to %q — posse seeded %q. That gap IS ranger-base-sczjy: a grant written into a file the seat never opens, with the launch green and the line correct by the pin's own assertion.", dir, got, b.ClaudeConfig)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("the seat resolves %q and there is no file there: %v", got, err)
	} else if !ClaudeTrusted(readConfig(t, got), dir) {
		t.Errorf("the seat resolves %q, which holds no grant for %s — the trust dialog it was seeded to suppress is the observable", got, dir)
	}
}

// ─── the BIT, not only the value: ranger-base-uz4cs ──────────────────────────

// claudeHistoryPath (sentline.go) is a CONSUMER of the one read, and until
// this bead it took half of it: the VALUE came from ClaudeConfigDirIn and
// the in-force BIT was spelled by hand beside it, as a bare
// os.Getenv("CLAUDE_CONFIG_DIR") == "". Both answers were `v != ""`, so no
// input made them disagree — what a second spelling costs is not a wrong
// answer, it is the coupling: a mutant that broke the in-force test inside
// runtimeHomeOverride left this function answering correctly regardless,
// because it never called it to find out (ranger-base-api7c's shape inside
// ClaudeConfigFile, one site over).
//
// SO THE WANT IS A LITERAL "" AND NOT ClaudeConfigDirIn's answer. That is
// the whole difference from TestClaudeHistoryPathAgreesWithClaudeConfigDirIn
// (ghostcomposer_qa_test.go, arm 2), which is the AGREEMENT half and is
// green under that mutant by construction: it derives its want from the same
// function it is testing through, so a mutant moves both sides together.
// Here the no-home arms name the answer outright, and a mutant reporting
// "in force" for an unset variable is read back as a history file under
// whatever cwd the caller happens to have — govern's G2 tick, dispatch's
// settle judgment (ranger-base-58b5's shape). That is also why every arm
// asserts the answer is "" or ABSOLUTE and never anything between.
//
// MEASURED 2026-10-31, this worktree, go1.26.5 darwin/arm64, with the
// `v == ""` bail deleted from runtimeHomeOverride:
//
//   - the PRE-FIX line SURVIVES that mutant — `ok`, which is the whole
//     defect this bead is about. It answers "" from its own os.Getenv and
//     never asks the function the mutant broke.
//   - the line as it stands REDS on it, on both no-variable arms, with
//     claudeHistoryPath() = <cwd>/history.jsonl. Not the bare relative
//     `history.jsonl`: the empty value the mutant now calls "in force" is
//     not absolute either, so the one read's OTHER job resolves it against
//     the launcher's cwd on the way through. Same store in a directory
//     nobody named, one filepath.Abs further along.
func TestClaudeHistoryPathTakesItsInForceBitFromTheOneRead(t *testing.T) {
	absDir := filepath.Join(t.TempDir(), "named-cfg")
	relCwd := t.TempDir()
	t.Chdir(relCwd)
	wd, err := os.Getwd() // $TMPDIR's /private prefix: ask, don't assume
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		what      string
		configDir string // "" means UNSET, which is not the empty value
		set       bool
		want      string
	}{
		// The two arms a bare `os.Getenv(…) == ""` also got right, kept
		// because they are posse's in-force RULE (set AND non-empty) and a
		// fix that moved the bit must not have moved the rule with it.
		{what: "no home, no variable", want: ""},
		{what: "no home, the variable present but EMPTY", configDir: "", set: true, want: ""},
		// And the arms that say "no home" alone does not silence the reader:
		// the variable names a store, so there is one.
		{what: "no home, an absolute variable", configDir: absDir, set: true,
			want: filepath.Join(absDir, claudeHistoryFile)},
		// The one read's other job reaches this consumer too: a relative
		// value is absolute by the time a path is joined onto it.
		{what: "no home, a RELATIVE variable", configDir: "relcfg", set: true,
			want: filepath.Join(wd, "relcfg", claudeHistoryFile)},
	} {
		// No home at all, which is the arm the hand-spelled bit guarded and
		// the only one where the two halves of the condition both matter.
		// os.UserHomeDir IS $HOME on unix, so this is the error branch.
		t.Setenv("HOME", "")
		if c.set {
			t.Setenv(ClaudeConfigDirVar, c.configDir)
		} else {
			unsetenvForTest(t, ClaudeConfigDirVar)
		}

		got := claudeHistoryPath()
		if got != c.want {
			t.Errorf("%s: claudeHistoryPath() = %q, want %q", c.what, got, c.want)
		}
		if got != "" && !filepath.IsAbs(got) {
			t.Errorf("%s: claudeHistoryPath() = %q, a RELATIVE store of record — lastSubmitted stats it from whatever cwd the caller has (govern's G2 tick, dispatch's settle judgment), so this is a history file in a directory nobody named", c.what, got)
		}
	}
}

// hvHomeVarNames is the three names as a census target: the string literals
// AND the constants that hold them, because a second read spelled either way
// is the same second read.
var hvHomeVarNames = map[string]bool{
	ClaudeConfigDirVar: true, CodexHomeVar: true, GrokHomeVar: true,
	"ClaudeConfigDirVar": true, "CodexHomeVar": true, "GrokHomeVar": true,
}

// hvEnvReadsOfAHomeVar reports every environment READ in one parsed file
// whose argument NAMES one of the runtime-home variables, as
// "<line>: <callee>(<arg>)".
//
// It matches the read and not the name: this package's prose says
// CLAUDE_CONFIG_DIR some seventy times and several launch paths pass the
// name to a message (noHomeSilence("CLAUDE_CONFIG_DIR", …), interstitial.go)
// or carry it in a list of names to ASK about rather than read
// (credentialDirVars, credential.go — a second spelling that is deliberate,
// documented and pinned, because it reads no value at all). None of those is
// a second answer to "which file is in play"; a Getenv is.
//
// runtimeHomeOverride itself is not an exemption and does not need to be: it
// takes the name as a PARAMETER, so `os.Getenv(name)` names nothing and this
// finds nothing there. That is why the census's expected answer is zero
// sites rather than one, and why the matcher below is pinned against
// synthetic sources — a census that must find nothing passes just as well
// broken.
func hvEnvReadsOfAHomeVar(t *testing.T, path string) []string {
	t.Helper()
	// Mode 0: comments are not retained, so prose naming the variable cannot
	// be read as a call (planMeterCalls, planmeteronce_qa_test.go).
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return hvEnvReadsIn(fset, file)
}

func hvEnvReadsIn(fset *token.FileSet, file *ast.File) []string {
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || (pkg.Name != "os" && pkg.Name != "syscall") {
			return true
		}
		if sel.Sel.Name != "Getenv" && sel.Sel.Name != "LookupEnv" {
			return true
		}
		arg := ""
		switch a := call.Args[0].(type) {
		case *ast.BasicLit:
			if a.Kind == token.STRING {
				if v, err := strconv.Unquote(a.Value); err == nil {
					arg = v
				}
			}
		case *ast.Ident:
			arg = a.Name
		}
		if !hvHomeVarNames[arg] {
			return true
		}
		pos := fset.Position(call.Pos())
		out = append(out, fmt.Sprintf("%s:%d: %s.%s(%s)",
			filepath.Base(pos.Filename), pos.Line, pkg.Name, sel.Sel.Name, arg))
		return true
	})
	return out
}

// THE CENSUS. ADR 0077 D1's rule is "one read of these names", and the
// reason it is a rule rather than a preference is that since the pin these
// names travel in two directions — posse resolves its own view of a CLI's
// state root through them and TYPES them onto the seat it hires. A name
// spelled twice is how those two come to name different files: the class
// cost ranger-base-x5f6p, ranger-base-wd4be and ranger-base-api7c before
// this bead, and ranger-base-uz4cs was the fourth, surviving inside the
// function whose own doc already recorded the third.
//
// Prose has held this rule three times and lost three times, so it is
// derived here instead. SCOPE, stated rather than implied: this package's
// own non-test .go files, which is where every posse-side reader of these
// variables lives (cmd/ carries none, measured by the same grep this
// replaces). Reaching cmd/ would mean climbing to the repo root, which puts
// a pin in the tree-wide class and wants a Makefile door (AGENTS.md's
// register); this one reads its own directory and stays out of it.
func TestNoSecondReadOfARuntimeHomeVariable(t *testing.T) {
	t.Parallel() // a parse of this directory's sources: no env, no cwd move
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var found, scanned int
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		for _, hit := range hvEnvReadsOfAHomeVar(t, name) {
			found++
			t.Errorf("%s reads a runtime-home variable's name directly — a SECOND answer to \"which file is in play\", beside runtimeHomeOverride (runtimehome.go), which is the one read both directions share. Ask it instead: the value for a path, the bit for in-force-ness. (ADR 0077 D1; the class is ranger-base-x5f6p / -wd4be / -api7c / -uz4cs.)", hit)
		}
	}
	if scanned < 50 {
		t.Fatalf("the census scanned %d non-test .go files in this package — it is reading the wrong directory, and a census that reads nothing passes", scanned)
	}
	if found == 0 && testing.Verbose() {
		t.Logf("census clean over %d files", scanned)
	}
}

// The matcher, against sources that cannot drift: the census above must find
// NOTHING in a clean tree, which is exactly the state a broken matcher also
// reports. These arms are what distinguishes the two, and the first of them
// is the line ranger-base-uz4cs removed, verbatim.
func TestTheRuntimeHomeReadCensusMatchesTheReadAndNotTheName(t *testing.T) {
	t.Parallel() // sources in memory, nothing shared
	for _, c := range []struct {
		what string
		src  string
		hit  bool
	}{
		{"the line this bead removed", `package p
func f() { if (err != nil || home == "") && os.Getenv("CLAUDE_CONFIG_DIR") == "" { return } }`, true},
		{"the same read spelled with the constant", `package p
func f() { _ = os.Getenv(ClaudeConfigDirVar) }`, true},
		{"LookupEnv, the other spelling of the same question", `package p
func f() { _, ok := os.LookupEnv("CODEX_HOME"); _ = ok }`, true},
		{"syscall, below os", `package p
func f() { _ = syscall.Getenv("GROK_HOME") }`, true},
		{"prose naming the variable is not a read", `package p
// CLAUDE_CONFIG_DIR moves the config dir, and os.Getenv("CLAUDE_CONFIG_DIR")
// is how this used to ask.
func f() {}`, false},
		{"the name passed to a message is not a read", `package p
func f() { _ = noHomeSilence("CLAUDE_CONFIG_DIR", "cannot tell") }`, false},
		{"a list of names to ask ABOUT is not a read", `package p
var credentialDirVars = []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR"}`, false},
		{"the one read itself: the name is a parameter", `package p
func runtimeHomeOverride(name string) (string, bool) { v := os.Getenv(name); return v, v != "" }`, false},
		{"another variable entirely", `package p
func f() { _ = os.Getenv("RHQ_CAGE") }`, false},
		{"a different package's Getenv", `package p
func f() { _ = cfg.Getenv("CLAUDE_CONFIG_DIR") }`, false},
	} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "scratch.go", c.src, 0)
		if err != nil {
			t.Fatalf("%s: fixture does not parse: %v", c.what, err)
		}
		got := len(hvEnvReadsIn(fset, file)) > 0
		if got != c.hit {
			t.Errorf("%s: matcher says hit=%v, want %v — the census over a clean package answers zero either way, so this table is the only thing that says it is still looking", c.what, got, c.hit)
		}
	}
}
