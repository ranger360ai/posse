//go:build !posse_arm2 && !posse_arm3

package posse

// QA, ranger-base-6s5mo (fix half of ranger-base-atq78) — ONLY A LAUNCH
// WRITES A PERSONA'S GATES DIR.
//
// `posse gates <persona>` is a report: AGENTS.md points a seat at it to READ
// its own wall. It was not one. It re-rendered the named persona's wall and
// shims from the environment of whoever typed the command, and the git shim
// bakes the launcher's HOME in as a literal and refuses any git whose HOME is
// not it (rule hookskip:config-env, ADR 0079 D1). So `HOME=/elsewhere posse
// gates <p>` — from that seat, a sibling seat, or an operator shell — left
// the running seat's git refused by its own gate: 26 FAIL in that seat's
// `make test`, 24 of them fixtures' own `git init`, exit 0, and not one line
// of output saying anything had been written (MEASURED 2026-11-11 by the
// verifying seat, ranger-base-atq78; the security lane's verdict on it is
// breakage only, so this is an availability and integrity defect of the
// L1/L2 rails and nothing more).
//
// THREE WRITERS WERE ON THAT PATH and this file pins all three:
//   - the shims, through `RenderGates` in the `gates` case (cmd/posse) —
//     that one is pinned where it lives, gatesreportwrites_qa_test.go, because
//     the bytes that broke the seat are only readable through the command;
//   - the wall, through `BwrapReport` / `SeatbeltReport`;
//   - the wall AGAIN, twice per runtime in the catalog, through parity's two
//     probes (applyRecordReach, applyWallWitness), which RENDER and then
//     APPLY — and which `posse gates` reaches for every runtime it prints.
//
// The render itself is unchanged in all three: the persona's live gates dir
// is still an INPUT to it, so the set a report prints and a probe judges is
// the set a launch would grant. What changed is where the bytes land
// (reportWallDir, wall.go) and, for the report alone, that the host-side prep
// is not run.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rwwPlantLive puts a sentinel where the LAUNCH writes this mechanism's
// artifact and hands back a check that it is still byte-identical. A sentinel
// and not a stat, because the defect was a REWRITE: a file that is there
// afterwards says nothing, and the one that broke the reporting seat was
// there the whole time.
func rwwPlantLive(t *testing.T, a *App, persona, name string) func(what string) {
	t.Helper()
	gates := a.GatesDir(persona)
	if err := os.MkdirAll(gates, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(gates, name)
	const sentinel = "# a live seat's wall, as a launch left it\n"
	if err := os.WriteFile(p, []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}
	return func(what string) {
		t.Helper()
		got, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s REMOVED the live wall at %s (%v) — only a launch writes that file", what, p, err)
			return
		}
		if string(got) != sentinel {
			t.Errorf("%s REWROTE the live wall at %s. A reader renders from its own environment, and the wall it replaced is what a running seat is behind — the shim half of exactly this left a live seat's git refused by its own gate (ranger-base-atq78). Got:\n%s", what, p, got)
		}
	}
}

// 1. The report. Host-neutral: BwrapReport is a pure render plus a print, so
// the Linux arm reads the same from darwin (the rule every host-shaped
// decision in ADR 0080 follows).
func TestQABwrapReportDoesNotWriteTheLiveWall(t *testing.T) {
	// No t.Parallel: bwlStubBin writes a package-level var.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	intact := rwwPlantLive(t, f.a, f.ag.Name, bwrapArgsName)
	txt := rwwPlantLive(t, f.a, f.ag.Name, bwrapTextName)

	var b strings.Builder
	if err := f.a.BwrapReport(f.ag, f.rt, f.dir, &b); err != nil {
		t.Fatalf("BwrapReport: %v", err)
	}
	intact("the bwrap report")
	txt("the bwrap report")

	// And the report is not thereby empty: it printed a wall, off a copy that
	// is really there. A pin whose subject is "it wrote nothing" passes for
	// free over a report that did nothing.
	dir, err := f.a.reportWallDir(f.ag.Name)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{bwrapArgsName, bwrapTextName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the report wrote no %s under %s (%v) — the head line names that file, and a head line naming a file that is not there is worse than no report", name, dir, err)
		}
	}
	if !strings.Contains(b.String(), "    w "+AbbrevHome(f.dir)) {
		t.Errorf("the report printed no mount set, so every assertion above passes over a command that did nothing:\n%s", b.String())
	}
}

// The same for the other mechanism's report, driven from either host for the
// reason above: a rule realized at one mechanism and not the other is a
// `posse gates` that means two things on two boxes, which is the class
// wall.go's own header refuses.
func TestQASeatbeltReportDoesNotWriteTheLiveProfile(t *testing.T) {
	t.Parallel()
	f := bwlFixture(t)
	intact := rwwPlantLive(t, f.a, f.ag.Name, "seatbelt.sb")

	var b strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.dir, &b, SeatbeltStateDirs(f.rt)...); err != nil {
		t.Fatalf("SeatbeltReport: %v", err)
	}
	intact("the seatbelt report")
	dir, err := f.a.reportWallDir(f.ag.Name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "seatbelt.sb")); err != nil {
		t.Errorf("the report wrote no profile copy under %s (%v) — its head line names one", dir, err)
	}
	if !strings.Contains(b.String(), "    w "+AbbrevHome(f.dir)) {
		t.Errorf("the report printed no writable set, so the arm above passes over a command that did nothing:\n%s", b.String())
	}
}

// 2. The prep, which is the other half of "a report writes nothing": the
// bwrap render asks the LAUNCHER for host-side writes outside the namespace,
// in the seat's name, and from the caller's environment they land in the
// caller's HOME (ranger-base-atq78's second measurement; the writes
// themselves are docs/notes.d/ranger-base-zibko.md's and f67jx's subject).
//
// THE CONTROL IS THE LAUNCH'S OWN RENDER. "no prep ran" proves nothing unless
// the identical render WITH prep makes those same paths — otherwise the arm
// would pass over a render that asked for nothing.
func TestQATheReportRenderRunsNoPrepAndTheLaunchRenderDoes(t *testing.T) {
	// No t.Parallel: bwlStubBin writes a package-level var.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")

	report, err := f.a.wallWriteToRead(f.ag.Name, false)
	if err != nil {
		t.Fatal(err)
	}
	_, r, err := f.a.renderBwrapWall(f.ag, f.rt, f.dir, report)
	if err != nil {
		t.Fatalf("the report render: %v", err)
	}
	// Only the steps whose path was NOT already on the host: a prep step over
	// something that exists is a no-op either way and would make this arm
	// green for the wrong reason.
	var asked []string
	for _, s := range r.Prep {
		if _, err := os.Lstat(s.Path); os.IsNotExist(err) {
			asked = append(asked, s.Path)
		}
	}
	if len(asked) == 0 {
		t.Fatal("this render asks the launcher for no prep over a path that is missing, so neither arm below measures anything — the fixture must produce at least one (bwrap binds only a source that exists, so a write grant that is not there is a mkdir)")
	}
	for _, p := range asked {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("the REPORT's render made %s on the host. A report describes the wall; it does not write outside the namespace in a seat's name, least of all from whatever environment typed the command (ranger-base-6s5mo)", p)
		}
	}

	// The control.
	if _, _, err := f.a.renderBwrapWall(f.ag, f.rt, f.dir, wallWrite{}); err != nil {
		t.Fatalf("the launch render: %v", err)
	}
	for _, p := range asked {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("the LAUNCH's render did not make %s either (%v) — then the arm above is measuring a render that asks for nothing, and this mechanism binds only a source that exists (ADR 0080 D17/D18)", p, err)
		}
	}
}

// 3. The probes. They are not reports — they RENDER AND APPLY, and that is
// why they keep the prep — but `posse gates <persona>` walks the same parity
// matrix for every runtime in the catalog, so before this bead they rewrote a
// live seat's wall twice per runtime from the caller's environment, in
// silence, on the one surface an operator reads a wall off.
//
// Live, because the subject is the probe that RAN: an abstention writes
// nothing and would pass this for free (that arm is
// TestQAWallWitnessAbstainsWhereNothingRanBehindThePrefix's, one file over).
func TestLiveWallWitnessProbeDoesNotWriteTheLiveWall(t *testing.T) {
	// No t.Parallel: wwRoot sets TMPDIR and HOME for this process.
	wwSkipUnlessTheWallIsReal(t)
	f := wwFixtureIn(t, wwRoot(t))
	name := wallArtifactName(WallMechanism())
	if name == "" {
		t.Skipf("NOTHING MEASURED: this host's mechanism %q renders no artifact", WallMechanism())
	}
	intact := rwwPlantLive(t, f.a, f.ag.Name, name)

	p := wwClaim(CageSeatbelt, "Edit")
	f.a.applyWallWitness(p, f.ag, f.rt, f.tree)
	// The probe really ran — otherwise the sentinel survives for the one
	// reason this pin must not accept.
	if got := p.Realized["Edit"].Detail; !strings.Contains(got, "witnessed at launch") {
		t.Fatalf("the witness did not witness, so nothing was rendered and the arm below is vacuous: %q", got)
	}
	intact("parity's Edit/Write witness")
	wwNoWitnessFileLeft(t, f.root)
}
