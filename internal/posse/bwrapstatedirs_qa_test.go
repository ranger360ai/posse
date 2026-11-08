//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-tc41o — ADR 0080 D19: claude on the bwrap wall.
//
// Three claims, and each one is silent at the mechanism if it is wrong:
//
//  1. claude's state grant is the DIRECTORY its CLAUDE_CONFIG_DIR names, and
//     the declared `~/.claude.json` entry drops out. Get this wrong the other
//     way and BwrapArgs refuses every claude launch (loud); get it wrong by
//     dropping the directory and the seat has no config at all.
//  2. with the variable out of force the render REFUSES. The failure it
//     prevents has no error in it: the runtime writes $HOME/.claude.json,
//     which is outside every directory grant, so the wall goes up, reports
//     success, and loses every config write the seat makes — ranger-base-cypy1
//     with the grant in place.
//  3. on a non-darwin host claude's credential joins the read mask for every
//     OTHER runtime's seat. Before D19 a Linux codex seat could read it, and
//     nothing anywhere said so.
//
// EVERY ARM RUNS ON DARWIN, for BwrapArgs' own reason (bwrapargs_qa_test.go):
// the render is pure, goos is a parameter of the credential selection, and no
// arm here puts a wall up.
//
// No t.Parallel in this file except the one arm driven at the pure function:
// every other one sets HOME and CLAUDE_CONFIG_DIR, which are process-wide.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// bwsHome puts HOME and the three config-dir variables in a known state and
// answers the home. The operator's own box exports CLAUDE_CONFIG_DIR, so an
// arm that did not unset it would be measuring this box.
func bwsHome(t *testing.T, cfg string) string {
	t.Helper()
	home := absResolve(t.TempDir())
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	if cfg == "" {
		unsetenvForTest(t, ClaudeConfigDirVar)
	} else {
		t.Setenv(ClaudeConfigDirVar, cfg)
	}
	return home
}

// bwsClaude is claude's runtime as this box declares it, so the arms below
// read the DECLARATION rather than restating it — a state_dir overlay or a
// change to builtinRuntimes reaches these pins instead of walking past them.
func bwsClaude(t *testing.T) *Runtime {
	t.Helper()
	rt, err := NewAppAt(t.TempDir()).LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// ─── the config-dir grant ────────────────────────────────────────────────────

// D19's first two items: the grant is the directory, and the file entry goes.
//
// The config dir is NOT the home's `.claude` in this arm, deliberately. With
// CLAUDE_CONFIG_DIR pointed at the default location the config dir and the
// declared `~/.claude` tree are one path, and every assertion below would
// pass over a function that ignored the variable entirely.
func TestBwrapStateDirsGrantsClaudesConfigDirAndDropsTheConfigFile(t *testing.T) {
	cfg := absResolve(t.TempDir())
	home := bwsHome(t, cfg)
	rt := bwsClaude(t)

	// CONTROL on the premise: the declaration this is reshaping really does
	// carry a FILE, and that file is the one at the home. Without this the
	// arms below are green over a declaration someone already changed.
	if !bwsHas(rt.StateDirs, "~/.claude") || !bwsHas(rt.StateDirs, "~/"+claudeConfigFileName) {
		t.Fatalf("claude declares %v — this pin reshapes `~/.claude` plus `~/%s` and cannot measure anything else", rt.StateDirs, claudeConfigFileName)
	}

	got, err := BwrapStateDirs(rt)
	if err != nil {
		t.Fatalf("a claude seat with %s set to an absolute directory must render: %v", ClaudeConfigDirVar, err)
	}
	if len(got) == 0 || got[0] != cfg {
		t.Fatalf("BwrapStateDirs = %v — %s's directory %s is claude's state grant at this mechanism and is not first (ADR 0080 D19)", got, ClaudeConfigDirVar, cfg)
	}
	// The file entry is gone, under every spelling that would reach a mount:
	// the raw declaration and the path it expands to.
	for _, p := range got {
		if strings.Contains(p, claudeConfigFileName) {
			t.Errorf("BwrapStateDirs keeps %s — a writable FILE has no spelling at this mechanism (B8), so BwrapArgs would refuse the whole render", p)
		}
	}
	if bwsHas(got, filepath.Join(home, claudeConfigFileName)) {
		t.Errorf("BwrapStateDirs = %v, which names the config file at the home — the directory grant above covers the file the runtime actually writes", got)
	}
	// And the TREE stays. Dropping it would read as "not a claude seat" to
	// credentialReadDenyLiterals' own(), which is the arm below.
	if !bwsHas(got, "~/.claude") {
		t.Errorf("BwrapStateDirs = %v — D19 drops the config FILE entry and not the declared tree", got)
	}
}

// The grant is the LAUNCHER's resolved value, which is the value ADR 0077 D1
// types onto the seat (RuntimeHomePin) and the value credential.go walls. One
// resolver or the three disagree, which is surface 1's whole defect
// (ranger-base-c33j6): posse seeded one file and the seat opened another.
func TestBwrapStateDirsReadsTheOneConfigDirResolver(t *testing.T) {
	cfg := absResolve(t.TempDir())
	home := bwsHome(t, cfg)
	rt := bwsClaude(t)

	got, err := BwrapStateDirs(rt)
	if err != nil {
		t.Fatal(err)
	}
	if want := ClaudeConfigDirIn(home); got[0] != want {
		t.Fatalf("BwrapStateDirs grants %s; ClaudeConfigDirIn answers %s — the wall must grant the directory the seat resolves", got[0], want)
	}
	// The pin and the grant are one value, not two derivations of one rule.
	// This is the assertion that reds if either side grows its own spelling
	// of CLAUDE_CONFIG_DIR.
	if pin := RuntimeHomePin(rt); !strings.Contains(pin, got[0]) {
		t.Fatalf("the typed line carries %q and the wall grants %s — the seat would write a directory the wall does not name (ADR 0077 D1)", pin, got[0])
	}
}

// ─── the refusals ────────────────────────────────────────────────────────────

// D19 item 3. Unset is a refusal and not a fallback to `<home>/.claude`,
// because with the variable out of force the runtime's config FILE is
// $HOME/.claude.json — outside every directory grant, unbindable as a file,
// and lost silently.
func TestBwrapStateDirsRefusesClaudeWithTheConfigDirUnset(t *testing.T) {
	bwsHome(t, "")
	rt := bwsClaude(t)

	got, err := BwrapStateDirs(rt)
	if err == nil {
		t.Fatalf("BwrapStateDirs = %v with %s unset — the render must refuse rather than wall a directory the runtime does not write into", got, ClaudeConfigDirVar)
	}
	// The message is the deliverable here: this refusal is the only thing
	// standing between the host and a silent lost-write, and it has to carry
	// the variable and the one line of host procedure that fixes it for every
	// seat at once (ranger-base-yyq64/offid).
	bwsRefusalNames(t, err)
}

// An EMPTY value is the third state, not unset and not a directory — posse's
// rule everywhere is that empty is out of force (runtimeHomeOverride), and
// the runtime's own resolvers disagree with each other about it
// (ClaudeConfigDirIn's measured divergence: `??` for the directory, `||` for
// the file). A render that accepted it would grant whatever the launcher's
// cwd happens to be.
func TestBwrapStateDirsRefusesAnEmptyConfigDir(t *testing.T) {
	home := absResolve(t.TempDir())
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	t.Setenv(ClaudeConfigDirVar, "")
	rt := bwsClaude(t)

	if got, err := BwrapStateDirs(rt); err == nil {
		t.Fatalf("BwrapStateDirs = %v with %s set to the empty string — empty is out of force everywhere else in posse and must not become a cwd-relative grant", got, ClaudeConfigDirVar)
	} else {
		bwsRefusalNames(t, err)
	}
}

// The arm the environment cannot produce, driven at the pure function.
//
// runtimeHomeOverride makes a relative value absolute at the single read of
// these variables (ranger-base-sczjy), so `in force and relative` survives
// only where os.Getwd itself failed. D19's demand is "set AND absolute" and
// the reason is ADR 0077 D1's: the launcher resolves a relative value against
// its own directory and the seat against the session's, so the directory
// walled and the directory written would be two directories. Unreachable is
// not the same as unnecessary, and a pin is how it stays spelled.
func TestBwrapStateDirsRefusesARelativeConfigDir(t *testing.T) {
	t.Parallel()
	declared := []string{"~/.claude", "~/" + claudeConfigFileName}
	got, err := bwrapClaudeStateDirs(declared, "relcfg", true, "/Users/probe")
	if err == nil {
		t.Fatalf("bwrapClaudeStateDirs = %v for a relative config dir — the launcher and the seat resolve it against different directories", got)
	}
	bwsRefusalNames(t, err)
	if !strings.Contains(err.Error(), "relcfg") {
		t.Errorf("the refusal does not name the value it refused: %v", err)
	}
	// CONTROL: the same declaration with an absolute value renders, so the
	// row above measures the relative-ness and not the fixture.
	if _, err := bwrapClaudeStateDirs(declared, "/Users/probe/.claude", true, "/Users/probe"); err != nil {
		t.Fatalf("CONTROL: the same inputs with an absolute config dir must render: %v", err)
	}
}

// bwsRefusalNames is D19's message contract, asked of every refusing arm:
// the variable, the mechanism that refused, and the remedy.
func bwsRefusalNames(t *testing.T, err error) {
	t.Helper()
	for _, want := range []string{ClaudeConfigDirVar, "Environment=", WallBwrap, "claude"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the D19 refusal does not name %q — %v", want, err)
		}
	}
}

// ─── every other runtime passes through ──────────────────────────────────────

// D19 is claude's item and nobody else's: codex, grok and bob declare
// DIRECTORIES, which this mechanism binds as they are. A function that
// reshaped them would be inventing a grant no ADR asked for.
func TestBwrapStateDirsPassesEveryOtherRuntimeThrough(t *testing.T) {
	bwsHome(t, "")
	a := NewAppAt(t.TempDir())
	for _, name := range []string{"codex", "grok", "bob"} {
		t.Run(name, func(t *testing.T) {
			rt, err := a.LoadRuntime(name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := BwrapStateDirs(rt)
			if err != nil {
				t.Fatalf("%s has no config-dir demand and must not be refused: %v", name, err)
			}
			if !reflect.DeepEqual(got, rt.StateDirs) {
				t.Fatalf("BwrapStateDirs(%s) = %v, want its declaration %v untouched", name, got, rt.StateDirs)
			}
		})
	}
	// A nil runtime is the no-runtime caller SeatbeltWritable already has an
	// answer for (it grants no runtime state at all, the fail-closed
	// direction) and not a panic one launch short of a wall.
	if got, err := BwrapStateDirs(nil); err != nil || got != nil {
		t.Fatalf("BwrapStateDirs(nil) = %v, %v — want no state grants and no refusal, which is SeatbeltWritable's own answer for a caller with no runtime", got, err)
	}
}

// ─── the two halves hold each other up ──────────────────────────────────────

// The coupling nothing about either function's own output reveals: D19 keeps
// `~/.claude` in claude's list, and credentialReadDenyLiterals' own() is keyed
// to that declared literal. Drop it as "covered by the config dir" and a
// Linux claude seat is denied its own credential — on that host the store of
// record (ADR 0019 D2), so the seat cannot authenticate at all.
//
// Driven from BwrapStateDirs' real output, not from a hand-built list, which
// is the only arrangement under which the two can be wrong together.
func TestBwrapStateDirsStillReadsAsAClaudeSeatToTheCredentialWall(t *testing.T) {
	cfg := absResolve(t.TempDir())
	home := bwsHome(t, cfg)
	a := NewAppAt(t.TempDir())
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	state, err := BwrapStateDirs(rt)
	if err != nil {
		t.Fatal(err)
	}
	creds := filepath.Join(cfg, ".credentials.json")
	homeCreds := filepath.Join(home, ".claude", ".credentials.json")

	deny := credentialReadDenyLiterals("linux", state)
	for _, p := range deny {
		if p == creds || p == homeCreds {
			t.Fatalf("a linux claude seat's DenyRead names %s — that file is claude's store of record on linux and this is its own seat; the seat cannot authenticate behind that mask (ADR 0019, ADR 0080 D19)", p)
		}
	}
	// CONTROL, and it is the bead's own done-condition: the same box, one
	// seat over, DOES carry it. Without this arm the assertion above is
	// satisfied by a function that denies nothing at all.
	codex, err := a.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}
	cstate, err := BwrapStateDirs(codex)
	if err != nil {
		t.Fatal(err)
	}
	cdeny := credentialReadDenyLiterals("linux", cstate)
	if !bwsHas(cdeny, creds) {
		t.Fatalf("a linux codex seat's DenyRead is %v and does not name %s — before ADR 0080 D19 every non-claude seat on linux could read claude's credential", cdeny, creds)
	}
	if !bwsHas(cdeny, homeCreds) {
		t.Errorf("a linux codex seat's DenyRead is %v and does not name the HOME spelling %s — whatever the runtime wrote before the variable moved is still sitting there (ADR 0019 D2 store 3)", cdeny, homeCreds)
	}
}

// ─── the darwin render must not change ──────────────────────────────────────

// D19 reshapes claude's list for ONE mechanism, and the cheap way to do that
// would have been to drop `~/.claude.json` from the declaration — which would
// take `(subpath "$HOME/.claude.json")` and its sibling regex out of the
// darwin profile too, putting ranger-base-cypy1's lost config write back on
// every seatbelt seat on this box with no line anywhere saying so.
//
// So: the declaration still carries the file, SeatbeltSiblings still grants
// its scratch namespace, and BwrapStateDirs is the only thing that knows
// otherwise.
func TestD19LeavesTheDarwinSeatbeltRenderAlone(t *testing.T) {
	home := bwsHome(t, absResolve(t.TempDir()))
	rt := bwsClaude(t)

	if !bwsHas(rt.StateDirs, "~/"+claudeConfigFileName) {
		t.Fatalf("claude declares %v — D19 is a render-side reshape and must not have removed the config file from the declaration (ranger-base-cypy1)", rt.StateDirs)
	}
	sibs := SeatbeltSiblings(rt.StateDirs, SeatbeltCarveOut{})
	if !bwsHas(sibs, filepath.Join(home, claudeConfigFileName)) {
		t.Fatalf("SeatbeltSiblings(%v) = %v — the darwin profile's sibling grant for the config file is what makes a caged claude seat's MCP adds and history land at all (ranger-base-cypy1)", rt.StateDirs, sibs)
	}
	// And the bwrap list is the one that differs, which is what says the
	// reshape happened at the mechanism rather than in the declaration.
	got, err := BwrapStateDirs(rt)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(got, rt.StateDirs) {
		t.Fatalf("BwrapStateDirs = %v, byte-identical to the declaration — then nothing was reshaped and BwrapArgs refuses this launch on the config file (B8)", got)
	}
}

// bwsHas is membership, spelled once: every arm above asks it of a []string
// whose order is a separate claim.
func bwsHas(in []string, want string) bool {
	for _, p := range in {
		if p == want {
			return true
		}
	}
	return false
}

// ─── end to end: this is what makes a claude launch renderable at all ───────

// D17 said it in the refusal it wrote for this: "claude's config file is ADR
// 0080 D19's". So the claim worth pinning is not that BwrapStateDirs returns
// a tidier list — it is that the tier's own producers, fed that list, render,
// and fed the declaration, refuse.
//
// Both arms over ONE fixture home with a real `.claude.json` file in it,
// because the refusal needs the file to be there: a path that is not there
// yet is a prep step and not a refusal (BwrapArgs shape 1), so a fixture
// without the file would make the control pass for the wrong reason.
func TestBwrapArgsRendersAClaudeSeatOnlyThroughD19(t *testing.T) {
	cfg := absResolve(t.TempDir())
	home := bwsHome(t, cfg)
	if err := os.WriteFile(filepath.Join(home, claudeConfigFileName), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := absResolve(t.TempDir())
	cwd := bwsMkdir(t, filepath.Join(root, "proj"))
	a := &App{Home: bwsMkdir(t, filepath.Join(root, "home")), StateDir: bwsMkdir(t, filepath.Join(root, "home", "state"))}
	ag := &AgentFile{Name: "developer", MemoryDir: bwsMkdir(t, filepath.Join(root, "home", "personas", "developer"))}
	gates := bwsMkdir(t, filepath.Join(a.StateDir, "gates", ag.Name))
	rt := bwsClaude(t)

	render := func(state []string) ([]string, error) {
		w := a.SeatbeltWritable(ag, cwd, gates, state...)
		c := a.SeatbeltCarveOut(ag, cwd, gates, w, state...)
		argv, _, err := BwrapArgs(cwd, w, c)
		return argv, err
	}

	// CONTROL first, so the arm below cannot be read as a function that
	// renders everything: the DECLARATION refuses, by name, on the file.
	if argv, err := render(rt.StateDirs); err == nil {
		t.Fatalf("the raw declaration %v renders %v — a writable FILE must refuse (B8), or a claude seat gets a wall that reports success and loses every config write", rt.StateDirs, argv)
	} else if !strings.Contains(err.Error(), claudeConfigFileName) {
		t.Fatalf("the declaration's refusal does not name the config file: %v", err)
	}

	state, err := BwrapStateDirs(rt)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := render(state)
	if err != nil {
		t.Fatalf("a claude seat does not render through D19's list %v: %v", state, err)
	}
	// The grant is there, as a read-write bind of the DIRECTORY.
	if want := "--bind " + cfg + " " + cfg; !strings.Contains(strings.Join(argv, " "), want) {
		t.Fatalf("the argv carries no %q — claude's config directory is its state grant at this mechanism (ADR 0080 D19):\n%s", want, strings.Join(argv, " "))
	}
	// And nothing in it names the file, under either spelling.
	if line := strings.Join(argv, " "); strings.Contains(line, filepath.Join(home, claudeConfigFileName)) {
		t.Errorf("the argv names the config file at the home:\n%s", line)
	}
}

// bwsMkdir makes a fixture directory and answers its resolved path. Resolved,
// because every path these producers emit has been through absResolve and the
// render deliberately does not resolve a second time.
func bwsMkdir(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return absResolve(p)
}
