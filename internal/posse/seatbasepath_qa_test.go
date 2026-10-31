//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0077 D2, surfaces 2 and 4 (ranger-base-zfljf) — the seat's BASE PATH
// and its SHELL are typed by posse, not inherited from whatever process
// happened to autostart the multiplexer server.
//
// THE FAILURE, MEASURED 2026-10-31 (ranger-base-c33j6). A probe session read
// a gates dir mid-PATH that the launching process did not put there: the
// herdr server had started hours earlier from another seat and donated that
// seat's PATH to every session it would ever open. A seat is a child of the
// server, so its base layer is the server's — unreadable to posse, and
// nobody's PID. On the same box and the same day, a crew seat's $SHELL was
// that persona's own `<state>/gates/<persona>/shell/zsh`, so every Rex seat
// it dispatched got an interactive shell inside the LAUNCHER's gate wrapper.
//
// Both halves are the same rule: what posse reads to decide about a seat,
// posse writes onto the seat. The overlay carries PATH (one line, both
// occupants — herdr's `workspace create --env`, Rex's `/usr/bin/env K=V`),
// and the shell comes from realShell, which refuses a wrapper already.
//
// WHY THE CONTROLS MATTER MORE THAN THE ASSERTIONS HERE. Every arm below
// would pass over a tree with the fix deleted if the fixture's PATH were
// clean — "no gates element" and "not a wrapper" are both true of an honest
// launcher. So each arm first proves its own launcher is DIRTY, in the two
// shapes that bite: a gates bin dir by name, and a symlink alias to one that
// spells nothing a literal match can catch (ranger-base-z51g).
//
// MUTATION-CHECKED, five, each run against all four tests:
//   - dropping the `EnvVar{"PATH", SeatBasePath()}` append in planLaunch reds
//     the first test ("the launch env carries no PATH") and the second's
//     herdr and Rex arms. 3 and 4 stay green.
//   - SeatBasePath returning os.Getenv("PATH") unstripped reds the fourth
//     test's own assertion, and reds 1 and 2 THROUGH THE LAUNCH: parity
//     degrades, and a dispatch that degrades is refused, so both read
//     "refused: claude at cage shims … does not realize every gate". That
//     chain is the point — a broken strip stops a launch rather than warning
//     about one. 3 stays green.
//   - rexShell returning os.Getenv("SHELL") reds the third test alone, on
//     all three of its arms.
//   - CageEnvNames forwarding PATH reds the second test's cage arm alone.
//   - parity's row reading ForeignGatesElements over the launcher's PATH
//     again reds the fourth test (naming the row verbatim) and, through the
//     same refusal, 1 and 2.
//
// And one that reds the CONTROLS rather than the claims, honestly: judging a
// PATH element by its spelling alone (gatesSplitPath without resolveLinks)
// reds the alias half of all three dirty-launcher controls — "the fixture's
// launcher PATH is not dirty in both shapes" — because the controls read the
// same walk the product does. A fixture that cannot prove itself dirty is a
// test that has stopped testing, which is the reading to want there.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sbpDirtyLauncher gives this process the PATH a gated launcher really has:
// another persona's gates bin dir, a symlink alias to a THIRD persona's that
// spells nothing, this launcher's OWN gates bin dir, and one honest dir —
// over the real PATH's gates-free remainder, so the fixture's own git and
// shell still resolve.
//
// It returns the three gates spellings and the honest dir. Not parallel-safe
// by construction (t.Setenv), which is why no test here calls t.Parallel.
func sbpDirtyLauncher(t *testing.T, ownBin string) (foreign, alias, keep string) {
	t.Helper()
	home := t.TempDir()
	foreign = filepath.Join(home, "state", "gates", "alpha", "bin")
	third := filepath.Join(home, "state", "gates", "beta", "bin")
	keep = filepath.Join(t.TempDir(), "usr-bin")
	for _, d := range []string{foreign, third, keep, ownBin} {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	alias = filepath.Join(t.TempDir(), "tools") // no "/gates/" in its own name
	if err := os.Symlink(third, alias); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	parts := []string{foreign, alias, keep}
	if ownBin != "" {
		parts = append(parts, ownBin)
	}
	parts = append(parts, PathOutsideGates(""))
	t.Setenv("PATH", strings.Join(parts, sep))
	return foreign, alias, keep
}

// sbpEnv indexes an assembled launch env by name, LAST WINS — which is what
// both occupants do with a repeated name (`--env K=V` twice, `env K=1 K=2`),
// and so the only reading that says what the seat actually gets.
func sbpEnv(vars []EnvVar) map[string]string {
	env := map[string]string{}
	for _, v := range vars {
		env[v.Key] = v.Value
	}
	return env
}

// sbpBackend is a launcher with one persona on its own temp home.
func sbpBackend(t *testing.T) (*HerdrBackend, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(b.App.AgentsDir, "ranger.md"), "---\nname: ranger\ndescription: test\n---\nYou are ranger.\n")
	return b, fake
}

// Surface 2, the overlay: the assembled launch env carries a PATH, posse
// wrote it, and no element of it reaches anybody's gates dir — the
// launcher's own included.
func TestSeatOverlayPathIsTypedOutsideEveryGatesDir(t *testing.T) {
	b, _ := sbpBackend(t)
	ownBin := filepath.Join(b.App.GatesDir("ranger"), "bin")
	foreign, alias, keep := sbpDirtyLauncher(t, ownBin)

	// CONTROL: the launcher this test launches FROM is dirty in both
	// shapes. Without this the arms below are true of any clean box and the
	// fix could be deleted under them.
	if got := ForeignGatesElements(ownBin); len(got) != 2 || got[0] != foreign || got[1] != alias {
		t.Fatalf("the fixture's launcher PATH is not dirty in both shapes: ForeignGatesElements = %v, want [%s %s]", got, foreign, alias)
	}

	p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", ByHand: true})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	env := sbpEnv(p.Vars)
	path, ok := env["PATH"]
	if !ok {
		t.Fatal("the launch env carries no PATH — the seat's base layer is then the multiplexer SERVER's, which posse cannot read and which belongs to whichever seat autostarted it (ADR 0077 D2)")
	}
	if path != SeatBasePath() {
		t.Errorf("the overlay PATH is %q, want SeatBasePath() %q — posse must type the value it computed, not a value it found", path, SeatBasePath())
	}
	for _, el := range filepath.SplitList(path) {
		if isGatesPath(el) || isGatesPath(resolveLinks(el)) {
			t.Errorf("the typed base PATH carries a gates element: %s — the seat would run behind rules that are not its own PID's (ADR 0017)", el)
		}
	}
	// The launcher's OWN gates bin dir goes with the rest: it is foreign to
	// the seat posse is hiring, and the seat's own wall is typed onto the
	// line by GatePrefix, not inherited through the overlay.
	for _, dropped := range []string{foreign, alias, ownBin} {
		if strings.Contains(path+string(os.PathListSeparator), dropped+string(os.PathListSeparator)) {
			t.Errorf("the typed base PATH still names %s", dropped)
		}
	}
	// And it is a PATH and not an empty gesture: the honest element survives.
	if !strings.Contains(path, keep) {
		t.Errorf("the typed base PATH dropped the honest element %s:\n%s", keep, path)
	}
}

// Both occupants read the same overlay, and the cage boundary does not.
func TestSeatOverlayPathReachesBothOccupantsAndNotTheCage(t *testing.T) {
	b, fake := sbpBackend(t)
	hermeticGen(t)
	ownBin := filepath.Join(b.App.GatesDir("ranger"), "bin")
	if _, _, _ = sbpDirtyLauncher(t, ownBin); len(ForeignGatesElements(ownBin)) != 2 {
		t.Fatal("CONTROL: the fixture's launcher PATH is not dirty")
	}
	want := SeatBasePath()

	// herdr: `workspace create --env PATH=<typed>`, which is the wiring —
	// startPlanned hands mux.Create the plan's Vars and nothing else.
	if err := b.CreateSession(NewSessionOpts{Name: "s-herdr", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("herdr launch: %v", err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "--env PATH="+want) {
		t.Errorf("the herdr create did not carry the typed base PATH (--env PATH=%s):\n%s", want, log)
	}

	// Rex: the same overlay, in the block shell's argv (ADR 0076 §2 — Rex
	// blocks take no env, so it rides there).
	p, err := b.planLaunch(NewSessionOpts{Name: "s-rex", Dir: t.TempDir(), Agent: "ranger", ByHand: true})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	m, _ := newTestRexMux(t)
	if _, err := m.Create("box:dev", "/tmp/work", p.Vars); err != nil {
		t.Fatalf("rex create: %v", err)
	}
	cmd := rexNewCommand(t)
	if len(cmd) == 0 {
		t.Fatal("`rex new` was given no block command")
	}
	if !containsString(cmd, "PATH="+want) {
		t.Errorf("the Rex block's argv does not carry the typed base PATH:\n%q", cmd)
	}

	// The cage boundary: the engine forwards a NAME by value from the pane
	// (`-e PATH`), and the pane's value is a HOST path list. The image has
	// its own PATH and renders its own gates over it (cageinner.go).
	if containsString(CageEnvNames(p.Vars), "PATH") {
		t.Error("PATH is forwarded across the cage boundary — the host's directories would lead the image's own, in front of the inner wall's (ADR 0077 D1's HOME argument, one layer down)")
	}
}

// Surface 4: a Rex seat's shell. realShell is the resolver, so the wrapper a
// gated launcher carries in $SHELL cannot become the hired seat's shell.
func TestRexSeatShellIsNeverTheLaunchersGateWrapper(t *testing.T) {
	// The MEASURED launcher state: $SHELL is the launching persona's own
	// rendered gate shell — a shell's basename, a readable executable file,
	// and every test the old rexShell made was true of it.
	wrapper := filepath.Join(t.TempDir(), "state", "gates", "launcher", "shell", "zsh")
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(wrapper, []byte("#!/bin/sh\n# "+gateShellMarker+"\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", wrapper)
	// CONTROL: the fixture really is a wrapper by posse's own test, or this
	// arm is asserting nothing.
	if !isGateWrapper(wrapper) {
		t.Fatalf("the fixture %s is not recognised as a gate wrapper", wrapper)
	}

	got := rexShell()
	if got == wrapper || isGateWrapper(got) {
		t.Errorf("rexShell() = %q — every Rex seat dispatched from a gated launcher would run inside that launcher persona's wrapper, with another PID's rules in front of every command it types (ADR 0077 D2, surface 4; the exec-loop this class caused is ranger-base-f0ay)", got)
	}
	// Delegation and not a second resolver: one answer to "which shell does
	// posse name", so a wrapper refused in gates.go cannot be accepted here.
	if real, _ := realShell(""); got != real {
		t.Errorf("rexShell() = %q but realShell(\"\") = %q — the two must be one answer", got, real)
	}
	// An honest $SHELL is still the operator's own.
	honest := filepath.Join(t.TempDir(), "zsh")
	if err := WriteExecutable(honest, []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", honest)
	if got := rexShell(); got != honest {
		t.Errorf("rexShell() = %q, want the launcher's honest $SHELL %q", got, honest)
	}
	// And realShell's own rule rides along: a $SHELL that is neither bash
	// nor zsh is not taken. That is deliberate here rather than tolerated —
	// posse TYPES shell lines into this block, and GatePrefix's leading
	// `PATH=…:"$PATH" cmd` is a POSIX prefix assignment a fish could not
	// parse, so such a seat could not have been gated at all.
	fish := filepath.Join(t.TempDir(), "fish")
	if err := WriteExecutable(fish, []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", fish)
	if got := rexShell(); got == fish {
		t.Errorf("rexShell() = %q — a shell that cannot parse a POSIX prefix assignment cannot run a gated seat's typed line", got)
	}
}

// Surface 3: parity's foreign-gates row is replaced by an assertion over the
// PATH posse types, so a dirty LAUNCHER no longer degrades the launch — the
// hazard that row named is the one the typed base removes.
func TestParityDoesNotDegradeOverTheLaunchersOwnDirtyPath(t *testing.T) {
	b, _ := sbpBackend(t)
	ownBin := filepath.Join(b.App.GatesDir("ranger"), "bin")
	foreign, alias, _ := sbpDirtyLauncher(t, ownBin)

	// CONTROL, and the whole point of the arm: this launcher's PATH is
	// exactly the state the old row degraded on.
	if got := ForeignGatesElements(ownBin); len(got) != 2 {
		t.Fatalf("the fixture is not in the degrading state: ForeignGatesElements = %v, want [%s %s]", got, foreign, alias)
	}
	// The assertion the row became: empty by construction, because it is one
	// half of the same walk read back against the other.
	if left := SeatPathGatesElements(); len(left) > 0 {
		t.Fatalf("SeatBasePath left a gates element behind: %v", left)
	}

	ag := loadTestAgent(t, "---\nname: ranger\ndescription: test\n---\nYou are ranger.\n")
	p := b.App.CheckParity(ag, builtinRuntime(t, "claude"), CageShims, TierStandard)
	for _, d := range p.Degraded {
		if strings.Contains(d, "PATH") || strings.Contains(d, "gates dir") {
			t.Errorf("a dirty LAUNCHER PATH still degrades the launch: %q — the seat's base PATH is typed now, so this row is about a process that is not the seat's (ADR 0077 D2)", d)
		}
	}
	// And the sentence that remains is the one a reader can act on: it names
	// the strip, not a relaunch of somebody else's shell.
	if strings.Contains(strings.Join(p.Degraded, "\n"), "relaunch with a clean PATH") {
		t.Error("the deleted advice is back: 'relaunch with a clean PATH' is advice to a human about a third process (ADR 0077 D2)")
	}
}
