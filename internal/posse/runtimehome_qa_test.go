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
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
