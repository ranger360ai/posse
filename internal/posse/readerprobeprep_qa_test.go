//go:build !posse_arm2 && !posse_arm3

package posse

// QA, ranger-base-qd9ei (residual of ranger-base-6s5mo, out of
// ranger-base-atq78) — A READER MAKES NO PATH ON THE HOST, AND SAYS SO WHEN
// THAT LEAVES IT NOTHING TO MEASURE.
//
// 6s5mo moved the ARTIFACT of every non-launch render out of the persona's
// live gates dir and stopped the REPORT's own render making the host-side
// prep. It deliberately left the prep alone in parity's two probes
// (applyRecordReach, applyWallWitness), because those two render and then
// APPLY: bwrap binds only a source that exists, so a probe with no prep
// measures a wall that cannot come up and reports the difference as this
// host's. Both probes are reached from checkParityIn, which `posse gates
// <persona>` calls ONCE PER RUNTIME in the catalog — so a report typed with
// a HOME that was not the launcher's still made that HOME's prep paths.
//
// MEASURED 2026-11-12 (the bead), from a bwrap-caged seat, `HOME=<fresh
// empty dir> posse gates builder` in a scratch RHQ_HOME: `.bob`,
// `.cache/posse/plan-usage`, `.cache/posse-seat/<session>`,
// `.config/go/telemetry/local` and 24 members of `.grok` created in that
// HOME. Every one a mkdir or an O_EXCL touch of a path this PID's own grants
// name — and NOT therefore harmless: `~/.grok/config.toml` is seeded EMPTY,
// posse's own project-config classifier reads an empty file as `invalid JSON:
// unexpected end of JSON input`, and a later launch in that home is REFUSED
// over it. That is ranger-base-f67jx, a P1 red in one arm of `make test` that
// no `-run` filter could reproduce because one test plants the file and
// another reads it (docs/notes.d/ranger-base-zibko.md has the chain, and
// names TestQATheAbsoluteWritableRowDegradesNoLaunch — a CheckParityIn
// caller, which is to say a reader — as the planting test).
//
// THE FIX IS NOT A BLANKET ABSTENTION, which is what the bead's candidate (a)
// would have been. Every prep step is emitted only where its path is ABSENT
// (PrepStep's own note, pinned here by the first arm), so the list's
// emptiness is a verdict: a render that asked for no prep is a wall whose
// every bind source is already on this host, and a reader may apply it
// without writing anything. So a reader's probe measures wherever the
// furniture is there and abstains only where it is not — which keeps the row
// on the one surface an operator reads a wall off, and gives up only the
// readings that were about a wall nobody could run.
//
// WHERE IT ABSTAINS IT IS THE THIRD ANSWER AND NOT A FINDING: not granted,
// not denied — NOT APPLIED (ADR 0013 §4, ranger-base-heur; ADR 0086 D5).
// Sharper than the abstention beside it, even: the launch this row describes
// is the very thing that makes the path, so "this wall cannot come up" would
// be false about the only launch the row is about — and, through
// `unrealized`, would refuse it.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the fixture ─────────────────────────────────────────────────────────────

// rppSeat is bwlSeat under a HOME of its own. The HOME is the point: the
// render's home-side grants (the seat cache, the plan-usage dir, a runtime's
// own config home) are where the prep lands, so an arm whose subject is
// "what did this render write outside the namespace" has to own the HOME it
// would have written into — and bwlFixture does not set one, which is how
// ranger-base-f67jx's seed reached the whole package's shared home.
type rppSeat struct {
	bwlSeat
	home string
}

// rppFixture is that seat. t.Setenv, so no arm in this file may t.Parallel.
func rppFixture(t *testing.T) rppSeat {
	t.Helper()
	home := absResolve(t.TempDir())
	t.Setenv("HOME", home)
	return rppSeat{bwlSeat: bwlFixture(t), home: home}
}

// rppAsked renders this seat's wall as a REPORT does — the one wallPrep that
// neither makes the host paths nor refuses over them — and returns the steps
// the render asked for. It is how every arm below learns what the launch
// would have made without making it.
func rppAsked(t *testing.T, f rppSeat) []PrepStep {
	t.Helper()
	w, err := f.a.wallWriteToRead(f.ag.Name, prepSkip)
	if err != nil {
		t.Fatal(err)
	}
	_, r, err := f.a.renderBwrapWall(f.ag, f.rt, f.dir, w)
	if err != nil {
		t.Fatalf("the report render: %v", err)
	}
	return r.Prep
}

// rppHomeTree is every path under this fixture's HOME, so an arm can say
// "this render wrote nothing at all in there" rather than "it did not write
// the one path I thought of". The bead's measurement is a LIST of 28-odd
// paths in a fresh HOME, and the arms that matter are the ones that read it
// the same way.
func rppHomeTree(t *testing.T, home string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != home {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ─── 1. the invariant prepRequire rests on ───────────────────────────────────

// A PREP STEP NAMES A PATH THAT IS NOT THERE — both directions, because
// prepRequire reads the LIST'S EMPTINESS as "this host already has the wall".
// Each of BwrapArgs' emission sites is guarded on a stat; one that stops
// being is a reader's probe that silently starts measuring a wall it cannot
// put up, and nothing else in the suite would say so.
//
// Host-neutral: the render is pure of the host except for the tree it stats,
// which is the rule every host-shaped decision in ADR 0080 follows.
func TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere(t *testing.T) {
	// No t.Parallel: rppFixture sets HOME, bwlStubBin writes a package var.
	f := rppFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")

	asked := rppAsked(t, f)
	if len(asked) == 0 {
		t.Fatal("this render asks for no prep at all, so neither direction below measures anything — the fixture must produce at least one step (a fresh HOME has none of the home-side grants bwrap binds)")
	}
	for _, s := range asked {
		if _, err := os.Lstat(s.Path); err == nil {
			t.Errorf("the render asked for %s (%s) over a path that is ALREADY there. prepRequire reads an empty step list as `this host has the whole wall`, so a step over an existing path makes a reader's probe abstain over nothing — and a `touch` over an existing file is one O_TRUNC away from emptying a live seat's refusals.log (runBwrapPrep's own note)", s.Path, s.Kind)
		}
	}

	// The other direction: once the host HAS them, the render asks for
	// nothing. This is the whole of what prepRequire measures.
	if err := runBwrapPrep(asked); err != nil {
		t.Fatalf("runBwrapPrep: %v", err)
	}
	if again := rppAsked(t, f); len(again) != 0 {
		var left []string
		for _, s := range again {
			left = append(left, s.String())
		}
		t.Errorf("the same render still asks for %d step(s) after the prep ran:\n  %s\nThen `no prep asked` is not `this host has the wall`, and a reader's probe would abstain on every launch dir forever (wallPrep, wall.go)", len(again), strings.Join(left, "\n  "))
	}
}

// ─── 2. the render: a reader refuses rather than writes ──────────────────────

// prepRequire makes nothing and refuses, naming the first path and the count
// — and the LAUNCH's render over the same fixture makes those same paths,
// which is the control: "it wrote nothing" proves nothing over a render that
// asked for nothing.
func TestQAAReadersProbeRenderRefusesRatherThanMakeAPathOnTheHost(t *testing.T) {
	// No t.Parallel: rppFixture sets HOME, bwlStubBin writes a package var.
	f := rppFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")

	asked := rppAsked(t, f)
	if len(asked) == 0 {
		t.Fatal("the fixture's render asks for no prep, so this arm measures nothing")
	}
	before := rppHomeTree(t, f.home)

	w, err := f.a.wallWriteToRead(f.ag.Name, prepRequire)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = f.a.renderBwrapWall(f.ag, f.rt, f.dir, w)
	absent := wallPrepAbsent(err)
	if absent == nil {
		t.Fatalf("a reader's probe rendered a wall that needs %d host path(s) it may not make, and did not refuse: err=%v. The probe APPLIES what it renders, so what came back is either a wall that cannot come up — reported as this host's — or a write into whatever HOME typed the command (ranger-base-qd9ei)", len(asked), err)
	}
	if absent.Step.Path != asked[0].Path || absent.N != len(asked) {
		t.Errorf("the refusal names step %q (%d of them); the render asked for %q first, %d in all. The row quotes this path, and BwrapArgs emits shallowest-first, so the FIRST step is the one worth naming", absent.Step.Path, absent.N, asked[0].Path, len(asked))
	}
	// errors.As through the production wrapping, not a type assertion: the
	// two probes read it with wallPrepAbsent and a render that one day wraps
	// this error must stay readable to them.
	var typed *wallPrepAbsentError
	if !errors.As(err, &typed) {
		t.Errorf("the refusal does not come back through errors.As, so both probes would read it as R0's render failure and take the row away (ADR 0086 D5)")
	}
	if msg := err.Error(); !strings.Contains(msg, AbbrevHome(asked[0].Path)) || !strings.Contains(msg, "only a LAUNCH") {
		t.Errorf("the refusal does not name the path and who may make it: %q", msg)
	}

	// Nothing of the host moved — not the named path, and not anything else
	// in the HOME this render would have written into.
	for _, s := range asked {
		if _, err := os.Lstat(s.Path); err == nil {
			t.Errorf("a reader's render MADE %s (%s). The prep is a write outside the namespace in the seat's name, from whatever environment typed the command — and an empty seeded config there is refused by posse's own classifier at the next launch (ranger-base-f67jx)", s.Path, s.Kind)
		}
	}
	if got := rppHomeTree(t, f.home); len(got) != len(before) {
		t.Errorf("a reader's render changed this HOME: %d entries before, %d after:\n  %s", len(before), len(got), strings.Join(got, "\n  "))
	}

	// THE CONTROL. The identical render on a LAUNCH's behalf makes every one
	// of them, so the arms above are about who is asking and not about a
	// render that asks for nothing.
	w.Prep = prepMake
	if _, _, err := f.a.renderBwrapWall(f.ag, f.rt, f.dir, w); err != nil {
		t.Fatalf("the launch's own render refuses too (%v) — then this whole file is measuring a broken fixture", err)
	}
	for _, s := range asked {
		if _, err := os.Lstat(s.Path); err != nil {
			t.Errorf("the LAUNCH's render did not make %s either (%v) — bwrap binds only a source that exists, so a launch that leaves it absent is a seat with no wall (ADR 0080 D17/D18)", s.Path, err)
		}
	}
}

// ─── 3. and it still judges a wall this host already has ─────────────────────

// The arm that says this is not the bead's candidate (a). Where the host
// already has the whole wall — which is every tree a seat has been launched
// in — a reader's probe renders exactly what a launch renders, writes its
// copy under reportWallDir, and keeps the row it would have earned.
func TestQAAReadersProbeStillRendersAWallThisHostAlreadyHas(t *testing.T) {
	// No t.Parallel: rppFixture sets HOME, bwlStubBin writes a package var.
	f := rppFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")

	// A launch's worth of furniture, by the launcher's own door.
	if err := runBwrapPrep(rppAsked(t, f)); err != nil {
		t.Fatalf("runBwrapPrep: %v", err)
	}
	// A live wall on disk, so this arm also re-states 6s5mo's rule for the
	// render it is about: a reader renders and does not REPLACE.
	intact := rwwPlantLive(t, f.a, f.ag.Name, bwrapArgsName)

	w, err := f.a.wallWriteToRead(f.ag.Name, prepRequire)
	if err != nil {
		t.Fatal(err)
	}
	args, _, err := f.a.renderBwrapWall(f.ag, f.rt, f.dir, w)
	if err != nil {
		t.Fatalf("a reader's probe refused a wall whose every source is on this host: %v — then the fix is a blanket abstention and the operator has lost the row on the one surface they read a wall off (ranger-base-qd9ei's candidate (a), which this is not)", err)
	}
	if _, err := os.Stat(args); err != nil {
		t.Errorf("the render returned %s and did not write it (%v) — the probe applies this file", args, err)
	}
	if dir, derr := f.a.reportWallDir(f.ag.Name); derr == nil && !strings.HasPrefix(args, dir) {
		t.Errorf("a reader's artifact landed at %s, outside its own render dir %s (ranger-base-6s5mo)", args, dir)
	}
	intact("a reader's probe over a host that has the whole wall")
}

// ─── 4. the two doors ────────────────────────────────────────────────────────

// rppDoorSeat is a parity door's fixture: the seat above plus a store of
// record under its launch dir, at the one mechanism that has prep.
//
// The MECHANISM is pinned rather than skipped on, so the arm reads the same
// on both hosts — the rule wall.go follows throughout. The tier is pinned
// available with it, because what this arm drives is the probe and not this
// box's kernel.
func rppDoorSeat(t *testing.T) rppSeat {
	t.Helper()
	wallMechanismForTest(t, "linux")
	bwlStubBin(t, "/usr/bin/bwrap")
	// This process could put a namespace up as far as these arms care: the
	// question they ask comes AFTER that one, and a real refusal here would
	// land them on the abstention beside the one they are about.
	wwStubApplyRefusal(t, "")
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	t.Cleanup(func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	})
	f := rppFixture(t)
	if err := os.MkdirAll(filepath.Join(f.dir, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

// rppClaim is the matrix the witness judges: `wholeTreeWriteWall`'s seatbelt
// claim over this PID's rule (wwClaim's own shape, with this file's runtime).
func rppClaim(f rppSeat, gates ...string) *Parity {
	p := &Parity{Runtime: f.rt.Name, Cage: CageSeatbelt, Tier: TierStrong, Realized: map[string]RealizedGate{}}
	for _, g := range gates {
		p.Realized[g] = RealizedGate{Class: Enforced, Detail: wholeTreeSeatbeltClaim}
	}
	return p
}

// ADR 0086 D5's witness, asked by a READER over a wall this host has not got
// the furniture for: the row stays realized, says NOT WITNESSED and why, the
// launch is not degraded, and nothing is made on the host.
func TestQAAReadersWitnessAbstainsWhereTheWallNeedsALaunchsPrep(t *testing.T) {
	// No t.Parallel: rppDoorSeat sets HOME and writes package vars.
	f := rppDoorSeat(t)
	asked := rppAsked(t, f)
	if len(asked) == 0 {
		t.Fatal("the fixture's render asks for no prep, so the witness has nothing to abstain over")
	}
	before := rppHomeTree(t, f.home)

	p := rppClaim(f, "Edit")
	f.a.applyWallWitness(p, f.ag, f.rt, f.dir, askedByReader)

	row, ok := p.Realized["Edit"]
	switch {
	case !ok:
		t.Fatalf("the witness TOOK THE ROW AWAY over furniture that is merely absent. That is R0's finding — a wall that cannot be built — and it refuses the launch; the launch this row describes is the thing that would make the path (ranger-base-qd9ei). Unrealized: %v", p.Unrealized)
	case !strings.Contains(row.Detail, "NOT WITNESSED"):
		t.Errorf("a witness that measured nothing must not read like one that passed: %q", row.Detail)
	case !strings.Contains(row.Detail, AbbrevHome(asked[0].Path)):
		t.Errorf("the abstention does not name the path the wall needs, so nobody can act on it: %q", row.Detail)
	case strings.Contains(row.Detail, "witnessed at launch"):
		t.Errorf("the row claims the wall was witnessed AND that nothing witnessed it: %q", row.Detail)
	case !strings.HasPrefix(row.Detail, wholeTreeSeatbeltClaim):
		// wallWitnessAbstain keeps parity's own claim and appends the
		// reason; a row that lost the claim is one no later reader — the
		// witness itself included, which finds its rows BY that Detail —
		// can recognise.
		t.Errorf("the abstention dropped parity's own claim from the row: %q", row.Detail)
	}
	if len(p.Degraded) != 0 || len(p.Unrealized) != 0 {
		t.Errorf("a check that did not run must not degrade the launch (ADR 0013 §4, ranger-base-heur): degraded=%v unrealized=%v", p.Degraded, p.Unrealized)
	}
	for _, s := range asked {
		if _, err := os.Lstat(s.Path); err == nil {
			t.Errorf("the witness MADE %s (%s) in the caller's HOME — which is this bead (ranger-base-qd9ei)", s.Path, s.Kind)
		}
	}
	if got := rppHomeTree(t, f.home); len(got) != len(before) {
		t.Errorf("a reader's witness changed this HOME: %d entries before, %d after", len(before), len(got))
	}

	// THE CONTROL: the same door on a LAUNCH's behalf renders, makes the
	// prep, and whatever its verdict is it is not THIS abstention. Without
	// it the arms above pass over a witness that abstains on every host.
	q := rppClaim(f, "Edit")
	f.a.applyWallWitness(q, f.ag, f.rt, f.dir, askedByLaunch)
	switch got, ok := q.Realized["Edit"]; {
	case ok && strings.Contains(got.Detail, "only a LAUNCH makes what the namespace cannot"):
		t.Errorf("a LAUNCH's witness abstained for want of a path it is allowed to make: %q", got.Detail)
	case ok && got.Detail == wholeTreeSeatbeltClaim:
		// Whatever the verdict, the launch's witness RAN: it renders, preps,
		// and then upgrades the row, abstains for another reason, or takes
		// it away. A row left exactly as parity wrote it means the door
		// returned before rendering, and then the control proves nothing.
		t.Errorf("a LAUNCH's witness left the row untouched (%q), so it never reached the render and this control says nothing about who may prep", got.Detail)
	}
	for _, s := range asked {
		if _, err := os.Lstat(s.Path); err != nil {
			t.Errorf("a LAUNCH's witness did not make %s either (%v) — then the arm above is measuring a render that asks for nothing", s.Path, err)
		}
	}
}

// ADR 0013 §4's reachability row, the same way. Its third answer already has
// two spellings — a process that cannot apply the wall, and a store that is
// not there — and this is the third: a wall whose sources are not all here.
func TestQAAReadersRecordReachAbstainsWhereTheWallNeedsALaunchsPrep(t *testing.T) {
	// No t.Parallel: rppDoorSeat sets HOME and writes package vars.
	f := rppDoorSeat(t)
	asked := rppAsked(t, f)
	if len(asked) == 0 {
		t.Fatal("the fixture's render asks for no prep, so this row has nothing to abstain over")
	}
	if n := len(recordTargets(f.ag, f.dir, beadsHome(f.dir))); n == 0 {
		t.Fatalf("the fixture has no store of record under %s, so this row abstains one arm earlier (`nothing to reach`) and says nothing about the prep", f.dir)
	}
	before := rppHomeTree(t, f.home)

	p := &Parity{Runtime: f.rt.Name, Cage: CageSeatbelt, Tier: TierStrong, Realized: map[string]RealizedGate{}}
	f.a.applyRecordReach(p, f.ag, f.rt, f.dir, askedByReader)

	row, ok := p.Realized[RecordReachGate]
	switch {
	case !ok:
		t.Fatalf("the row is missing entirely; unrealized=%v. A check that measured nothing is a printed row and never a silence (ranger-base-fm4p)", p.Unrealized)
	case !strings.Contains(row.Detail, "NOT MEASURED"):
		t.Errorf("the row must say nothing was measured: %q", row.Detail)
	case !strings.Contains(row.Detail, AbbrevHome(asked[0].Path)):
		t.Errorf("the abstention does not name the path the wall needs: %q", row.Detail)
	case strings.Contains(row.Detail, "probed at launch"):
		t.Errorf("nothing was probed: %q", row.Detail)
	case row.Class != "":
		t.Errorf("an unmeasured row carries no enforcement class, got %q — Class is what `posse gates` prints as a realized gate (ADR 0025)", row.Class)
	}
	if len(p.Degraded) != 0 || len(p.Unrealized) != 0 {
		t.Errorf("a check that did not run must not degrade the launch (ADR 0013 §4, ranger-base-heur): degraded=%v unrealized=%v", p.Degraded, p.Unrealized)
	}
	for _, s := range asked {
		if _, err := os.Lstat(s.Path); err == nil {
			t.Errorf("the reachability probe MADE %s (%s) in the caller's HOME — which is this bead (ranger-base-qd9ei)", s.Path, s.Kind)
		}
	}
	if got := rppHomeTree(t, f.home); len(got) != len(before) {
		t.Errorf("a reader's reachability probe changed this HOME: %d entries before, %d after", len(before), len(got))
	}

	// THE CONTROL, as above: a LAUNCH's probe makes the prep and does not
	// land on this abstention.
	q := &Parity{Runtime: f.rt.Name, Cage: CageSeatbelt, Tier: TierStrong, Realized: map[string]RealizedGate{}}
	f.a.applyRecordReach(q, f.ag, f.rt, f.dir, askedByLaunch)
	if got := q.Realized[RecordReachGate].Detail; strings.Contains(got, "only a LAUNCH makes what the namespace cannot") {
		t.Errorf("a LAUNCH's probe abstained for want of a path it is allowed to make: %q", got)
	}
	for _, s := range asked {
		if _, err := os.Lstat(s.Path); err != nil {
			t.Errorf("a LAUNCH's probe did not make %s either (%v) — then the arm above is measuring a render that asks for nothing", s.Path, err)
		}
	}
}
