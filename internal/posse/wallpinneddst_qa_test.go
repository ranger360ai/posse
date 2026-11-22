//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ADR 0100 D4: a PRIVATE DESTINATION IS A MOUNT IN EVERY WALLED
// NAMESPACE THAT CAN WRITE ITS PARENT.
//
// (a) per render: every destination that differs from its source is PINNED —
// D2(b)'s predicate, the same function wallpinnedsource_qa_test.go drives
// over the source side.
// (b) box-wide: THE PRIVATE SET FOLLOWS THE GRANT, NOT THE RUNTIME.
//
// WHAT THE RULE IS FOR, and why it is a different attack from the source
// arm's. MEASURED 2026-11-15 (ADR 0100's Context, the dst-rename probe, this
// box, bubblewrap 0.13.1): a namespace holding NO mount on `<cfg>/<member>`
// renames it away mid-session and creates a new one; both steps succeed, the
// victim's own bind stays alive on the renamed dentry, and the victim's next
// open of `<cfg>/<member>/…` reads the PLANTED file. A namespace holding the
// same bind gets EBUSY for the same mv. So this is a standing rename at any
// moment of the victim's session and not a launch race — which is why no
// check at launch can close it, and why the answer is that everyone who can
// write the parent holds the child as a mount.
//
// For shell-snapshots and session-env the victim SOURCES what is in there
// ahead of every Bash command, so an unpinned private destination is a
// walled-to-walled lateral move.
//
// THE FIXTURE HOMES ARE UNDER wallUngrantedDir, for the reason
// wallpinnedsource_qa_test.go's header gives: `/tmp` is granted
// unconditionally, so a home under it puts everything inside a grant and
// both rules stand down there on purpose (bwrapWallRoot).
//
//	go test ./internal/posse -run 'WallPinnedDst' -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ─── D4(a): the census, as a pin ────────────────────────────────────────────

// Pin AND CENSUS (ADR 0100 D4(a) item 2): every destination that differs from
// its source is pinned, in every render this package makes — the seat render
// for each runtime posse launches, the landed-check judge and the wall venue.
//
// MEASURED 2026-11-20 by running this: all five renders pass, with ONE miss
// before the fix beside it — grok's `daemon/run`, loose at `<grok
// home>/daemon`, exactly as the ADR assumed. codex's `tmp/` and
// `shell_snapshots/`, which the ADR left to this census, PASS: each is a
// direct child of the granted codex home and is its own tmpfs, so each is
// the mount that pins it. `<cfg>/<member>`, `<cfg>/history.jsonl`, `/tmp`,
// `<common>/objects` and `<own>/seat-objects-base` pass as the ADR said they
// would.
//
// IT IS THE RENDER AND NOT A LIST OF PATHS, so a bead that adds a private
// destination is censused by writing nothing: the row it has to pass is this
// loop over the argv its own producer lands in.
func TestQAWallPinnedDstEveryRenderPinsEveryDestination(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	if err := os.MkdirAll(ClaudeConfigDirIn(f.home), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex", "grok"} {
		r, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err != nil {
			t.Fatalf("%s: the render is refused (%v) — if that is this rule's refusal, the render needs an in-place bind of the loose parent and not an exemption", name, err)
		}
		if n := wpdCountPinnedDsts(t, name, r.Argv, shallowestFirst(r.Set)); n < 2 {
			t.Errorf("CONTROL: the %s render has %d destinations that differ from their source, so this arm measured nothing:\n %s", name, n, bwJoin(r.Argv))
		}
	}
	g := newLandedCageFixture(t)
	if WallMechanism() != WallBwrap {
		t.Skip("the judge and venue arms want this host's bwrap render; its mechanism is " + WallMechanism())
	}
	writable := []string{g.run}
	for _, kind := range []landedCageKind{landedCageJudge, landedCageVenue} {
		cage, err := boxLandedCage(&App{}, kind, filepath.Join(g.run, "judge"), g.tree, writable, landedRunEnv(g.run))
		if err != nil {
			t.Fatalf("%s: the render is refused (%v) — same reading as the seat arms above", kind, err)
		}
		if n := wpdCountPinnedDsts(t, kind.String(), cage.Argv, writable); n == 0 {
			t.Errorf("CONTROL: the %s render has no destination that differs from its source:\n %s", kind, bwJoin(cage.Argv))
		}
	}
}

// wpdCountPinnedDsts is the census over one argv: it reds every destination
// that is loose and answers how many it judged, so a caller can say its arm
// measured something.
func wpdCountPinnedDsts(t *testing.T, who string, argv, grants []string) int {
	t.Helper()
	ms := bwrapMounts(argv)
	dsts := bwrapMountDsts(ms)
	var n int
	for _, m := range ms {
		if m.Src == m.Dst {
			continue
		}
		n++
		if loose, ok := bwrapPinned(m.Dst, grants, dsts); !ok {
			t.Errorf("the %s render mounts %q over %s and %s lies strictly inside a write grant with no mount on it — a walled seat, or anything this one leaves behind, renames it mid-session and plants its own directory at that name (ADR 0100 D4(a))\n argv: %s", who, m.Src, m.Dst, loose, bwJoin(argv))
		}
	}
	return n
}

// Pin: the one census miss, and that the render REFUSES it rather than
// shipping it. The miss has a bead of its own, named in ADR 0100's D4(a)
// amendment and only there, by bead id and nothing else, because the surface
// it lies on is an open residual (operator ruling 2026-11-11).
// Its SHAPE is D4(a)'s own, and the rule is the whole
// description: the member itself is a tmpfs, so that name is a mount and
// cannot be renamed from inside a namespace holding it, while the parent
// BETWEEN it and the granted home was a mount of nothing — so a leftover of
// that very seat renames the parent away and plants its own directory at
// that name, the tmpfs riding the renamed dentry while the NAME resolves to
// the plant.
//
// The fix is the one the ADR prescribes for a miss: an in-place read-write
// bind of the loose parent, emitted by the render that already grants it
// (runtimeEphemeralDirParents). This is that fix asserted from both ends —
// the shipped render carries the in-place bind, and the same render with it
// taken out is refused by name.
//
// MUTATION RUN: deleting the runtimeEphemeralDirParents loop in bwrapRender
// reds the first arm; deleting the bwrapRefuseUnpinnedDst call in BwrapArgs
// step 6 reds the second.
func TestQAWallPinnedDstTheCensusMissParentIsBoundInPlace(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("grok"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	eph := grokEphemeralDirs(f.home, runtimeStateGrants([]string{"~/.grok"}))
	if len(eph) != 1 {
		t.Fatalf("CONTROL: this fixture's grok seat renders %d ephemeral dirs, so the rows below are about another shape: %v", len(eph), eph)
	}
	parent := filepath.Dir(eph[0])
	// The shipped render: the parent is a mount of its own, in place.
	if !containsString(bwrapMountDsts(bwrapMounts(r.Argv)), parent) {
		t.Errorf("the grok render mounts a tmpfs over %s and nothing over its parent %s, so a leftover of this seat renames the parent and plants the name (ADR 0100 D4(a))\n argv: %s", eph[0], parent, bwJoin(r.Argv))
	}
	if !writeGranted(r.Set, parent) {
		t.Errorf("CONTROL: %s lies inside no write grant of this render, so the in-place bind above would be handing out a write nobody asked for: %v", parent, r.Set)
	}

	// The same render with that in-place bind dropped: refused, by name.
	carve := r.Carve
	carve.Private = nil
	for _, pb := range r.Carve.Private {
		if pb.Src == parent && pb.Dst == parent {
			continue
		}
		carve.Private = append(carve.Private, pb)
	}
	if len(carve.Private) == len(r.Carve.Private) {
		t.Fatalf("CONTROL: the shipped render carries no in-place bind of %s to drop, so the arm below measures nothing", parent)
	}
	argv, _, err := BwrapArgs(f.dir, r.Set, carve, r.Keep...)
	switch {
	case err == nil:
		t.Errorf("the render mounted a tmpfs over %s with its parent %s pinned by nothing (ADR 0100 D4(a))\n argv: %s", eph[0], parent, bwJoin(argv))
	case !strings.Contains(err.Error(), parent):
		t.Errorf("the refusal does not name the loose component %s, which is the whole of its instruction: %v", parent, err)
	}
}

// ─── D4(b): the private set follows the grant, not the runtime ──────────────

// Pin (ADR 0100 D4(b)): a NON-CLAUDE PID with a `writable:` extra covering
// claude's config dir renders claude's private table — every member of
// claudeSessionStateDirNames plus history.jsonl — so that seat holds each
// one as a mount and gets EBUSY for the rename, whichever runtime it runs.
//
// WHY IT HAS TO BE THE GRANT. The old gate was the LAUNCHING RUNTIME's state
// grants, which made the table a property of which CLI a seat runs. The
// attack is a property of who can WRITE the directory: a codex seat granted
// `<cfg>` by a PID extra could rename a live claude seat's shell-snapshots
// directory out from under it and plant its own, and that directory is code
// the victim sources ahead of every Bash command.
//
// MUTATION RUN, which is the ADR's own ("put the old gate back -> reds"):
// the last arm here IS the old gate, called with the same inputs, asserted
// to render nothing — so a revert of bwrapRender's one line reds the first
// arm while that arm stays green.
func TestQAWallPinnedDstPrivateSetFollowsTheGrantNotTheRuntime(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	cfg := absResolve(ClaudeConfigDirIn(f.home))
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	f.ag.Writable = []string{cfg}
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if !writeGranted(r.Set, cfg) {
		t.Fatalf("CONTROL: the `writable:` extra did not put %s in this codex render's write set, so nothing below is measured: %v", cfg, r.Set)
	}
	dsts := bwrapMountDsts(bwrapMounts(r.Argv))
	want := append([]string{}, claudeSessionStateDirNames...)
	want = append(want, claudeHistoryFile)
	for _, n := range want {
		p := filepath.Join(cfg, n)
		if !containsString(dsts, p) {
			t.Errorf("a codex seat granted %s binds nothing over %s, so it can rename that member out from under a live claude seat and plant its own — and for shell-snapshots and session-env that is code the victim sources ahead of every Bash command (ADR 0100 D4(b))\n argv: %s", cfg, p, bwJoin(r.Argv))
		}
	}

	// THE OLD GATE, with the same inputs: it rendered none of them. This is
	// the mutation the ADR names, kept as a row so the arm above cannot go
	// green by accident.
	if old := claudePrivateSessionState(f.home, f.dir, runtimeStateGrants([]string{"~/.codex"})); len(old) != 0 {
		t.Errorf("CONTROL: the runtime-state-grant gate renders %d binds for a codex seat, so the arm above does not measure the change of gate: %v", len(old), old)
	}
	// And the grant-keyed gate is not simply "always": a seat that cannot
	// write `<cfg>` still gets nothing, because there is no member of
	// anyone's namespace to protect from it.
	if got := claudePrivateSessionState(f.home, f.dir, []string{f.dir}); len(got) != 0 {
		t.Errorf("a render whose write set does not reach %s binds %d private members all the same: %v", cfg, len(got), got)
	}
}

// Pin: the one place the grant-keyed gate does NOT apply, so that the
// stand-down is a measured decision and not a line nobody reads. On a box
// whose home lies under the shared `/tmp` every render's write set reaches
// `<cfg>` through the blanket grant, nothing there is pinned by anything,
// and both of ADR 0100's refusals are disabled — so a grant-keyed table
// would be a mount with no EBUSY behind it. The gate reads the launching
// runtime's state grants there, which is what it read before D4(b).
//
// MUTATION RUN: dropping claudePrivateStateGate's stand-down reds the codex
// arm here, and reds TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere and
// TestQAWallClaudeProjectsRendersOwnKeyInsideAReadOnlyProjects beside it —
// both of which run under a t.TempDir() home, which is to say under `/tmp`.
//
// THE HOME IS wallStandDownDir's AND NOT A `t.TempDir()`, and that is what
// keeps this pin from going dark in the one venue built to run it
// (ranger-base-ddn56). The stand-down this arm is named for needs the home
// under `bwrapTmpMount`; `t.TempDir()` is there only while the box's TMPDIR
// is, so under `make tmpdir-check` — TMPDIR at a `/var/tmp` root, the whole
// point of that door — this test SKIPPED and the package line read `ok` over
// it. MEASURED 2026-11-22 (linux 7.2.15, go1.27.3, at 91381cab), this test
// run under a `-run` filter beside
// TestQAWallClaudeProjectsRendersOwnKeyInsideAReadOnlyProjects with `-v`:
// `SKIP (0.00s)` under that venue and `PASS (0.00s)` with TMPDIR unset, `ok`
// both times. An abstention is never a pass (ADR 0097 D5), and the fix is a
// root that measures in BOTH shapes rather than a door that catches the skip.
// It was dark on the VENUE OF RECORD too — ADR 0098 D5 makes that ci.yml's
// macos legs, where `t.TempDir()` is under `/var/folders/…` — so this pin had
// never measured there on any push.
//
// SO THE STAND-DOWN IS NOW A CONTROL AND NOT A SKIP. With the home taken from
// a root this fixture chose, a non-empty bwrapWallRoot() is no longer a fact
// about the box this pin has to live with — it is this fixture sitting
// somewhere it did not mean to, which is a red.
// TestQANoWallPinGoesDarkUnderTheOffTmpVenue (wallvenuedark_qa_test.go) is the
// door that keeps the class shut.
func TestQAWallPinnedDstPrivateSetGateStandsDownWithTheWall(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, wallStandDownDir(t))
	if bwrapWallRoot() != "" {
		t.Fatalf("CONTROL: this fixture's home is %s, cut under a stand-down root, and bwrapWallRoot() still answers %s — so the stand-down this pin is named for is not in force and every row below measures the other case (ranger-base-ddn56)", f.home, bwrapWallRoot())
	}
	cfg := absResolve(ClaudeConfigDirIn(f.home))
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	set := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, "~/.codex")
	if !writeGranted(set, cfg) {
		t.Fatalf("CONTROL: a home under %s does not put %s inside this render's write set, so the stand-down is not what keeps the row below green: %v", bwrapTmpMount, cfg, set)
	}
	if got := claudePrivateSessionState(f.home, f.dir, claudePrivateStateGate(set, runtimeStateGrants([]string{"~/.codex"}))); len(got) != 0 {
		t.Errorf("a codex seat on a box with no wall gets %d private claude binds whose sources it can swap itself: %v", len(got), got)
	}
	// The claude seat on the same box keeps the table it always had: the
	// stand-down returns the runtime's grants, not nothing.
	if got := claudePrivateSessionState(f.home, f.dir, claudePrivateStateGate(set, runtimeStateGrants([]string{"~/.claude"}))); len(got) == 0 {
		t.Errorf("the stand-down took the private table off a claude seat as well, which is a change this bead did not decide")
	}
}

// ─── The live half: the probe against a real rendered argv ──────────────────

// The live pin ADR 0100 D4 rests on, run against a REAL bwrap and a REAL
// rendered argv rather than a hand-made one: a namespace that holds
// `<cfg>/<member>` as a mount cannot rename it (EBUSY), and the same render
// with the private binds removed — which is what a non-claude seat granted
// `<cfg>` got before D4(b) — renames it and plants its own directory at that
// name.
//
// THE RENAME IS THE WHOLE ATTACK. The holder's refusal is what gives every
// walled seat on the box a member nobody can swap; the non-holder's success
// is what the stale-wall residual still is, until every walled seat launched
// before this landing has relaunched.
func TestLiveWallPinnedDstAHolderCannotRenameAPrivateMember(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	bwrapNestSkip(t) // ADR 0096 D3: both arms below exec a bwrap
	root := wallUngrantedDir(t)
	home, proj := filepath.Join(root, "home"), filepath.Join(root, "proj")
	cfg := filepath.Join(home, ".claude")
	for _, d := range []string{filepath.Join(home, ".cache"), proj, cfg} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv(ClaudeConfigDirVar, cfg)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount)
	}
	private := claudePrivateSessionState(home, proj, []string{absResolve(cfg)})
	if len(private) == 0 {
		t.Fatalf("CONTROL: no private table renders for a write set that grants %s, so neither arm below is measured", cfg)
	}
	member := filepath.Join(absResolve(cfg), "shell-snapshots")
	render := func(p []PrivateBind) []string {
		t.Helper()
		argv, prep, err := BwrapArgs(proj, []string{absResolve(cfg)}, SeatbeltCarveOut{Private: p})
		if err != nil {
			t.Fatalf("BwrapArgs refused the fixture render: %v", err)
		}
		// runBwrapPrep and not a copy of its switch: the launcher's own
		// half makes the mount points this argv binds over.
		if err := runBwrapPrep(prep); err != nil {
			t.Fatalf("the launcher half of the render failed: %v", err)
		}
		return argv
	}
	mv := func(argv []string, to string) (string, error) {
		out, err := exec.Command("bwrap", append(append([]string{}, argv...), "mv", member, to)...).CombinedOutput()
		// The abstention (ranger-base-k2quj). BOTH of this test's arms read
		// this return as an answer about the mount: the fix arm reds when
		// `err == nil` and the control fatals when it does not, so a seat
		// the box SIGKILLs — silent, 128+9 — is a false pass in one and a
		// false red in the other. ADR 0097 D3's exit 3 instead.
		bgSeatAbstainIfOutside(t, "the renaming seat inside the rendered wall", err, string(out))
		return string(out), err
	}

	// THE FIX: the rendered seat argv holds the member as a mount.
	out, err := mv(render(private), member+".stolen")
	switch {
	case err == nil:
		t.Errorf("a namespace holding the rendered bind over %s renamed it anyway — then nothing on this host stops a sibling swapping a live seat's shell snapshots, and ADR 0100 D4's answer does not hold here\n%s", member, out)
	case !strings.Contains(out, "busy"):
		t.Errorf("the rename was refused by something other than the mount (%v): %s", err, out)
	}

	// THE CONTROL, which is also what the old gate rendered for a seat that
	// is not a claude seat: no mount over the member, and the rename lands.
	if _, err := mv(render(nil), member+".stolen"); err != nil {
		t.Fatalf("NOTHING MEASURED: a namespace with no mount over %s could not rename it either (%v), so the refusal above is not the mount's", member, err)
	}
	if _, err := os.Lstat(member + ".stolen"); err != nil {
		t.Errorf("the control's rename reported success and %s is not there: %v", member+".stolen", err)
	}
}
