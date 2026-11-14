//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-8ukiy — the landed-check judge hides the system run dir and
// every registered runtime's state root, so a pathname socket is not a way
// around `--unshare-net`.
//
// The process this protects: `posse dispatch`'s landing path →
// landedChecker.judge → boxLandedCage, and through ADR 0097 D1 the wall
// venue, which is that same render plus three additions and so inherits
// whatever is pinned here. `--unshare-net` takes the network namespace and
// nothing else: a unix socket with a pathname is reached through the MOUNT
// namespace, at the same uid, so before this layer the judge answered
// /run/tailscale/tailscaled.sock — whose LocalAPI carries a dial endpoint —
// exactly as the operator does. The venue cannot take ranger-base-6rndn's
// Landlock step instead (a Landlock FS domain forbids mount and the venue
// exists to nest), so for the venue a hide is the only layer there is.
//
// Delete these rows and the judge goes back to the host's dbus, the host's
// docker.sock and the operator's own CLI control sockets, with no test red.

import (
	"os"
	"path/filepath"
	"testing"
)

// Row 1 is the one the bead asks for by name: ADDING A RUNTIME WITHOUT A
// STATE ROOT REDS. It is not equivalent to the derivation by construction —
// landedJudgeRuntimeRoots walks builtinRuntimes and could walk it entirely
// while contributing nothing for a member that declares neither `StateDirs`
// nor a `HomeVar`, which is precisely the shape a fifth built-in lands in.
// So this asks, per runtime, that the list carries a root of ITS own.
func TestQALandedJudgeHidesEveryRegisteredRuntimeStateRoot(t *testing.T) {
	// No t.Parallel: the fixture sets HOME, and the roots are resolved
	// against it.
	f := newLandedCageFixture(t)
	home := f.ssh.home
	if os.Getenv("HOME") == "" {
		t.Fatal("the fixture left HOME unset, so every declared `~/…` root resolves to nothing and the rows below would pass over an empty list")
	}
	// A home variable in force and pointing somewhere OTHER than the
	// declared literal, so the both-spellings row below can discriminate.
	// The fixture sets CLAUDE_CONFIG_DIR to `<home>/.claude`, which is
	// claude's declared literal — with only that, dropping the HomeVar arm
	// from the derivation is equivalent to keeping it (MEASURED by mutation,
	// 2026-11-14) and this test would be green over a judge that hides a
	// moved state root nowhere.
	moved := filepath.Join(f.root, "moved-grok-home")
	t.Setenv(GrokHomeVar, moved)

	roots := landedJudgeRuntimeRoots()
	if len(roots) == 0 {
		t.Fatal("landedJudgeRuntimeRoots returned nothing, so the judge hides no runtime state at all (ranger-base-8ukiy)")
	}
	for i := range builtinRuntimes {
		// A VALUE COPY and not &builtinRuntimes[i]: cmd/testparallel reads a
		// taken address as a WRITE, and marking this table written taints
		// every parallel test that names it — 27 of them, MEASURED by
		// `make corpus-check` on the pointer form (ranger-base-8ukiy).
		rt := builtinRuntimes[i]
		// This runtime's own roots, restated rather than taken from the
		// function under test: the declared literals and, where its home
		// variable is in force, the directory that variable names. A
		// runtime declaring neither leaves `want` empty and reds on the
		// sentence below, which is the whole point of the row.
		var want []string
		for _, d := range rt.StateDirs {
			if p := ExpandTilde(d); filepath.IsAbs(p) {
				want = append(want, absResolve(p))
			}
		}
		if rt.HomeVar != "" {
			if v, inForce := runtimeHomeOverride(rt.HomeVar); inForce && filepath.IsAbs(v) {
				want = append(want, absResolve(v))
			}
		}
		if len(want) == 0 {
			t.Errorf("runtime %q declares no state root this render can hide — neither `StateDirs` nor a `HomeVar` in force — so the judge and the ADR 0097 venue reach whatever control socket this CLI binds in its own home. Declare the CLI's state root on the runtime, do not delete this row (ranger-base-8ukiy)", rt.Name)
			continue
		}
		// EVERY spelling and not merely one, which is
		// credentialReadDenyLiterals' rule in this list's terms: whatever the
		// CLI wrote before its home variable moved the write is still sitting
		// in the declared literal, and a hide over an absent path costs
		// nothing — the hide loop skips what is not there.
		for _, w := range want {
			if !containsString(roots, w) {
				t.Errorf("runtime %q declares the state root %s and landedJudgeRuntimeRoots does not carry it (%v) — the judge does not hide this CLI's state, so whatever control socket it binds there is reachable past --unshare-net (ranger-base-8ukiy)", rt.Name, w, roots)
			}
		}
	}

	// A declared entry naming a FILE is SKIPPED, not refused and not hidden:
	// every claude install declares `~/.claude.json` beside `~/.claude`, a
	// tmpfs mounts over a directory only, and refusing would be a judge that
	// never runs on the box it was written for.
	file := filepath.Join(home, ".claude.json")
	writeFile(t, file, "{}\n", 0o600)
	if !containsString(roots, absResolve(file)) {
		t.Fatalf("%s is not among the runtime roots %v, so the file-skip row below has no premise: claude declares it in StateDirs and this row asserts the hide list drops it rather than refusing over it (ranger-base-8ukiy)", file, roots)
	}
	hides, err := landedJudgeHides()
	if err != nil {
		t.Fatalf("landedJudgeHides refused over the declared FILE %s: %v — a state entry that is not a directory is skipped, because a tmpfs can only be mounted over one and every claude install has this file (ranger-base-8ukiy)", file, err)
	}
	if containsString(hides, absResolve(file)) {
		t.Errorf("landedJudgeHides carries the FILE %s, and BwrapArgs refuses a hide that is not a directory — the judge would not render at all (ranger-base-8ukiy):\n  %v", file, hides)
	}
	// The system run dir, and the user runtime dir it already contains.
	if !containsString(hides, absResolve(landedCheckRunDir)) {
		t.Errorf("landedJudgeHides does not carry %s, so every system daemon's pathname socket under it answers the judge at the launcher's own uid — `--unshare-net` does not reach a unix socket (ranger-base-8ukiy):\n  %v", landedCheckRunDir, hides)
	}
	live, err := liveBwrapRuntimeDirs()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range live {
		if !containsString(hides, d) {
			t.Errorf("landedJudgeHides dropped the user runtime dir %s that liveBwrapRuntimeDirs names (ranger-base-ludpp, ranger-base-8ukiy):\n  %v", d, hides)
		}
	}
}

// Row 2: what the real renderer puts in the real argv. Every hide that EXISTS
// as a directory on this host is a `--tmpfs` in what boxLandedCage rendered —
// which is the only form the kernel ever sees, and the form ADR 0097 D1's
// venue inherits.
func TestQALandedJudgeCageArgvHidesTheRunDirAndTheRuntimeRoots(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := newLandedCageFixture(t)
	// Every built-in's declared root, MADE to exist, so the argv row is not
	// vacuous on a box where a CLI was never installed: the hide loop skips
	// what is not there, and a pin reading the argv has to ask about
	// directories that are. An entry already present as a non-directory is
	// left alone — that is claude's `~/.claude.json`, whose skip is row 1's.
	for i := range builtinRuntimes {
		for _, d := range builtinRuntimes[i].StateDirs {
			p := ExpandTilde(d)
			if !filepath.IsAbs(p) {
				continue
			}
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				continue
			}
			if err := os.MkdirAll(p, 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}

	cage, err := boxLandedCage(&App{}, landedCageJudge, filepath.Join(f.run, "judge"), f.tree, []string{f.run}, nil)
	if err != nil {
		t.Skip("NOTHING MEASURED: this host has no landed-check cage to render: " + err.Error())
	}
	switch WallMechanism() {
	case WallBwrap:
		hides, err := landedJudgeHides()
		if err != nil {
			t.Fatal(err)
		}
		line := bwJoin(cage.Argv)
		var asked int
		for _, h := range hides {
			if fi, err := os.Stat(h); err != nil || !fi.IsDir() {
				continue // the hide loop skips it too
			}
			asked++
			if bwIndex(cage.Argv, "--tmpfs", h) < 0 {
				t.Errorf("the judge's cage does not hide %s, so a pathname socket in it is reachable past --unshare-net (ranger-base-8ukiy):\n  %s", h, line)
			}
		}
		if asked == 0 {
			t.Fatalf("no hide in %v exists as a directory on this host, so this row asked nothing", hides)
		}
		if bwIndex(cage.Argv, "--tmpfs", absResolve(landedCheckRunDir)) < 0 {
			t.Errorf("the judge's cage does not hide %s — the dbus system bus, docker.sock and tailscaled.sock all sit under it (ranger-base-8ukiy):\n  %s", landedCheckRunDir, line)
		}
	case WallSandboxExec:
		// Hide is bwrap-only: SBPL mediates operations, and a path rule
		// cannot stop a connect(2) on a socket the profile can read
		// (SeatbeltCarveOut.Hide). The darwin half of this question is
		// ranger-base-1xyso's and not this pin's.
		t.Skip("NOTHING MEASURED: Hide has no seatbelt spelling, so there is no argv row on this mechanism")
	default:
		t.Fatalf("boxLandedCage rendered a cage on mechanism %q, which this pin does not know how to read", WallMechanism())
	}
}

// Row 3: the guard. A state root that is `/`, a top-level directory, or a
// directory holding the home REFUSES — bwrapRuntimeDirs' rule, applied to
// the same shape one list over, because an empty tmpfs over any of them is a
// judge with no tree rather than a narrower wall.
func TestQALandedJudgeHideRefusal(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		what, p, home string
		refuse        bool
	}{
		{"the root", "/", "/home/u", true},
		{"a top-level directory", "/opt", "/home/u", true},
		{"the home itself", "/home/u", "/home/u", true},
		// `/home` is caught by the top-level arm before the home arm is
		// reached. Both answers are "refuse", so the row asks the only thing
		// that matters and a reader does not have to know which arm fired.
		{"a directory holding the home", "/home", "/home/u", true},
		{"an ordinary state root", "/home/u/.grok", "/home/u", false},
		{"a state root beside the home", "/srv/state/grok", "/home/u", false},
		{"no home to swallow", "/home/u/.grok", "", false},
	} {
		err := landedJudgeHideRefusal(c.p, c.home)
		if c.refuse && err == nil {
			t.Errorf("%s: landedJudgeHideRefusal(%q, %q) allowed the hide — an empty tmpfs there leaves the judge with no tree (ranger-base-8ukiy)", c.what, c.p, c.home)
		}
		if !c.refuse && err != nil {
			t.Errorf("%s: landedJudgeHideRefusal(%q, %q) refused: %v", c.what, c.p, c.home, err)
		}
	}
}
