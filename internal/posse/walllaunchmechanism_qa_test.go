//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-temhf — A LAUNCH AT THE OTHER HOST'S WALL MECHANISM.
//
// WHAT WENT WRONG, because the gap is the whole subject. ADR 0080 D18
// (ranger-base-djl3c) made `cage: seatbelt` two mechanisms: `sandbox-exec -f
// <prof>` on darwin, `3<args <bwrap> --args 3 --` on Linux (RenderWall,
// wall.go). It landed with thorough pins for the bwrap RENDER, every one of
// them drivable from darwin — bwrapResolves, bwrapApplyRefusal and bwrapBin
// are package vars for exactly that, and `renderWallOn` takes the mechanism
// as a parameter. It landed `make test` green on darwin and RED ON MAIN:
// seven tests, all three arms, all six ubuntu jobs of ci.yml, which is the
// only gate a commit here ever passes.
//
// None of the seven was about the wall. They were about a state_dir reaching
// a launch, a PID binding a skill, a self_sandbox: key, a credential-dir
// refusal on relaunch — and each one read `state/gates/<p>/seatbelt.sb` by
// name, or grepped a launch line for `sandbox-exec`, or launched a claude
// seat that ADR 0080 D19 refuses at the bwrap mechanism with
// CLAUDE_CONFIG_DIR unset (which TestMain leaves it). Every one of those is a
// DARWIN assertion wearing the tier's name.
//
// And no pin could have caught them, because the mechanism parameter stops at
// `renderWallOn`: a LAUNCH asks `WallMechanism()`, so no test could put
// planLaunch or CreateSession behind the other mechanism. `wallMechanismHost`
// (wall.go) is the var that closes that, and this is the pin it is for.
//
// THE CLAIM, in both directions: a launch at either mechanism renders that
// mechanism's artifact, types that mechanism's prefix in front of the runtime
// command, and carries the launching runtime's declaration into it — and the
// one thing the two mechanisms do not share, D19's refusal of a claude seat
// with no config dir named, reaches the LAUNCH and not only the render.
//
// AND THE DESIGN CLAIM THE SAME LAUNCH SETTLES, which is the question
// ranger-base-ybz6u was filed on and the one the host cutover rests on:
// `seatbelt (bwrap)` is `CageLabel`'s LABEL for one tier realized by this
// host's mechanism (D16) and NOT a `cage_realize_<goos>:` mapping (ADR 0083
// D7). The label makes the two easy to confuse and the difference is total.
// A MAPPING is an explicit promoted line that maps one TIER to another,
// fails closed when absent, and marks every session it lowers with a
// ratified waiver. A MECHANISM maps nothing. So with no `cage_realize_*` key
// in this home at all, the demanded tier IS the realized tier — D7's own
// second bullet, "a tier maps to itself where `AvailableCages` has it", and
// D6 option C's "the D7 map flips from `shims` to the bwrap tier, one
// promoted line", which is why the host's `cage_realize_linux:` carries no
// `seatbelt` entry once this tier is available. The last block of
// TestQALaunchAtTheOtherHostsWallMechanism says it out loud over the launch
// it has already made.
//
// WHAT IT IS NOT. Driving the Linux mechanism from darwin is not a Linux
// render: `SeatbeltWritable` reads `runtime.GOOS` for the home-cache grants
// and for `/private/tmp` (seatbelt.go), so the SET being bound here is still
// darwin's. That is deliberate and is the division of labour the whole bead
// rests on — which paths the wall covers is `homeCacheGrants`' own pin, and
// WHICH WALL A LAUNCH PUTS UP is this one. A green here plus a green there is
// not a substitute for ci.yml's ubuntu arms, and nothing in this file claims
// to be.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Serial: this writes wallMechanismHost, bwrapBin and AvailableCages, all
// package vars every launch path reads (cmd/testparallel flags exactly that).
func TestQALaunchAtTheOtherHostsWallMechanism(t *testing.T) {
	for _, tc := range []struct {
		goos string
		mech string
	}{
		{"darwin", WallSandboxExec},
		{"linux", WallBwrap},
	} {
		t.Run(tc.mech, func(t *testing.T) {
			wallMechanismForTest(t, tc.goos)
			bwlStubBin(t, "/usr/bin/bwrap")
			if got := WallMechanism(); got != tc.mech {
				t.Fatalf("pinned the host to %s and the mechanism is %q, want %q", tc.goos, got, tc.mech)
			}
			had := AvailableCages[CageSeatbelt]
			AvailableCages[CageSeatbelt] = true
			t.Cleanup(func() {
				if !had {
					delete(AvailableCages, CageSeatbelt)
				}
			})

			b, _ := newTestBackend(t)
			a := b.App
			if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
				t.Fatal(err)
			}
			// Not claude: D19's demand is the arm below, not a precondition
			// of this one (bwlSeat's own reason).
			if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
				[]byte("command: mycli {file}\nstate_dir: ~/.mycli-wlm\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
				[]byte("---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\n---\nwork\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger"})
			if err != nil {
				t.Fatalf("planLaunch at the %s mechanism: %v", tc.mech, err)
			}
			wrap, artifact := wallLaunchWrapForTest(t, a.GatesDir("ranger"), "ranger", dir)
			// The LINE. `wrap` ends in a space and the runtime command
			// follows it, so this says the prefix is typed in front of the
			// command and not merely somewhere on the line.
			if !strings.Contains(p.Cmd, wrap+"mycli") {
				t.Errorf("a launch at the %s mechanism does not type its wrap in front of the runtime command.\nwant: %q\n got: %q", tc.mech, wrap+"mycli", p.Cmd)
			}
			// …and never the OTHER mechanism's binary, which is the half that
			// says the switch switched rather than widened.
			for _, other := range []string{WallSandboxExec, WallBwrap} {
				if other != tc.mech && strings.Contains(p.Cmd, other) {
					t.Errorf("a launch at the %s mechanism names %s on its line: %q", tc.mech, other, p.Cmd)
				}
			}
			// The ARTIFACT, written and naming the launching runtime's
			// declaration. A grant is a path in both renderings — an SBPL
			// `(subpath …)` line on one, a `--bind` argv pair on the other —
			// so the declaration is readable as text either way, and the
			// gates dir is the witness that this is a real wall and the
			// assertion can fail.
			got, rerr := os.ReadFile(artifact)
			if rerr != nil {
				t.Fatalf("a launch at the %s mechanism wrote no %s: %v", tc.mech, filepath.Base(artifact), rerr)
			}
			if !bytes.Contains(got, []byte(a.GatesDir("ranger"))) {
				t.Fatalf("the %s wall does not carry even this persona's gates dir — measuring the wrong file:\n%s", tc.mech, got)
			}
			if !bytes.Contains(got, []byte(ExpandTilde("~/.mycli-wlm"))) {
				t.Errorf("the launching runtime declared state_dir: ~/.mycli-wlm and the %s wall the launch rendered does not grant it:\n%s", tc.mech, got)
			}

			// AND THE DESIGN HALF (ranger-base-ybz6u): the launch above
			// came up at the tier the PID DEMANDED, with no mapping
			// anywhere in it. `seatbelt (bwrap)` is a label, never a value
			// — see the file comment for why the distinction is the whole
			// of what the host cutover rests on. This home declares no
			// `cage_realize_*` key, so the claim is that the demanded tier
			// is the realized tier: the bare constant, an empty `Ratified`
			// and an empty `Degraded`.
			//
			// It holds at either mechanism and does not depend on which one
			// this subtest pinned: `realizeCageIn` keys on `runtime.GOOS`
			// and not on `wallMechanismHost` (cagerealize.go), so what is
			// measured here is the ABSENCE of a mapping, which is the fact
			// D7 and D6 option C are about. Reading it inside both subtests
			// is deliberate all the same — a mechanism that ever did reach
			// this answer would red here and nowhere else.
			ag, lerr := a.LoadAgent("ranger")
			if lerr != nil {
				t.Fatal(lerr)
			}
			rt, lerr := a.LoadRuntime("mycli")
			if lerr != nil {
				t.Fatal(lerr)
			}
			if par := a.CheckParity(ag, rt, CageSeatbelt, TierStrong); par.Cage != CageSeatbelt || len(par.Ratified) != 0 || len(par.Degraded) != 0 {
				t.Errorf("an unmapped `cage: seatbelt` launch at the %s mechanism must be the DEMANDED tier with no ratified waiver and no degradation (ADR 0083 D7's own \"a tier maps to itself where AvailableCages has it\"; %q is CageLabel's label and never a value): cage=%q ratified=%v degraded=%v",
					tc.mech, CageLabel(CageSeatbelt), par.Cage, par.Ratified, par.Degraded)
			}
		})
	}
}

// D19's refusal where ranger-base-tc41o could not put it and
// TestQAClaudeOnTheBwrapWallRefusesWithoutTheConfigDirAtTheLaunch still could
// not: at the LAUNCH. That pin calls renderWallOn directly, so what it holds
// is that the render refuses; this holds that the refusal reaches planLaunch,
// which is where a seat is actually hired — and that it does NOT reach a
// darwin launch, because SBPL grants claude's config FILE by path and has
// nothing to refuse.
//
// The asymmetry is the finding: three of the seven tests that reddened main
// launched a claude seat at `cage: seatbelt` with the variable unset and
// nothing anywhere said that was a Linux-only refusal.
func TestQAClaudeAtTheBwrapMechanismIsRefusedAtTheLaunchAndNotOnDarwin(t *testing.T) {
	for _, tc := range []struct {
		goos    string
		mech    string
		refuses bool
	}{
		{"darwin", WallSandboxExec, false},
		{"linux", WallBwrap, true},
	} {
		t.Run(tc.mech, func(t *testing.T) {
			wallMechanismForTest(t, tc.goos)
			bwlStubBin(t, "/usr/bin/bwrap")
			had := AvailableCages[CageSeatbelt]
			AvailableCages[CageSeatbelt] = true
			t.Cleanup(func() {
				if !had {
					delete(AvailableCages, CageSeatbelt)
				}
			})
			// What TestMain leaves every test in this binary holding, and
			// what a Linux box without the one line D19 asks for holds.
			t.Setenv(ClaudeConfigDirVar, "")

			b, _ := newTestBackend(t)
			a := b.App
			if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
				[]byte("---\nname: ranger\ndescription: t\nruntime: claude\ncage: seatbelt\n---\nwork\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"})
			switch {
			case tc.refuses && err == nil:
				t.Errorf("a claude seat launched at the %s mechanism with %s unset — the runtime would resolve its config to the $HOME/.claude.json FILE, outside every directory grant, and the wall would go up, report success and lose every config write (ADR 0080 D19)", tc.mech, ClaudeConfigDirVar)
			case tc.refuses && !strings.Contains(err.Error(), ClaudeConfigDirVar):
				t.Errorf("the launch refusal at the %s mechanism must name the variable and the one line of host procedure that sets it: %v", tc.mech, err)
			case !tc.refuses && err != nil:
				t.Errorf("a claude seat at the %s mechanism must launch with %s unset — this mechanism grants the config file by path and has nothing to refuse: %v", tc.mech, ClaudeConfigDirVar, err)
			}

			// And the other half of the same demand: SET and absolute, the
			// launch renders on both.
			claudeConfigDirForTest(t)
			if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
				t.Errorf("a claude seat at the %s mechanism with %s set and absolute must launch: %v", tc.mech, ClaudeConfigDirVar, err)
			}
		})
	}
}
