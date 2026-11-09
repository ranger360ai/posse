//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-sg7a7 — ADR 0080 D23: WHAT THE LAUNCHER MAY CREATE.
//
// WHAT WENT WRONG, because the gap is the subject. D17/D18's prep shape 1
// exists for a real reason — bwrap binds only a source that exists, where
// sandbox-exec granted paths that did not — so a write grant that is not
// there becomes a `mkdir -p` outside the namespace, in the seat's name. It
// landed with no perimeter, and two things that cost nothing at the other
// mechanism became host writes:
//
//  1. AN ABSOLUTE `writable:` IN A PID WAS A DIRECTORY POSSE CREATED. One
//     line — `writable: [scratch, /opt/shared]` in parity_test's fixture —
//     was a clean launch on darwin (SBPL grants a path whether or not it is
//     there and nothing writes it), a REFUSED launch on this box
//     (`mkdir /opt/shared: permission denied`) and a new directory on
//     ci.yml's ubuntu runner, created silently on every red run. Nothing
//     said so before the launch.
//  2. A LAUNCH MATERIALIZED `<cwd>/.beads`, AND THAT MOVED A LATER LAUNCH'S
//     PARITY ROW. The store of record is in the writable set, so prep made
//     it; and ADR 0013 §4's reachability row probes only a store that
//     EXISTS. In TestSeatbeltProfileAndLaunch a grok seatbelt launch
//     therefore handed a codex seat three arms later — same tree, `-s
//     read-only` — a real store behind a read-only sandbox where it had
//     none before, and the row went from "nothing to reach" to unrealized
//     and REFUSED that launch. Same tree, same PIDs, a different answer
//     depending on which seat launched there first.
//
// Both were found by driving a launch at the bwrap mechanism from darwin
// (ranger-base-temhf's `wallMechanismHost`), and neither was reported by
// anything: on ci.yml's ubuntu arms the first was a silent host write and
// the second reddened three lines of one test that are about something else.
//
// THE CLAIM. Prep creates the session's own furniture and nothing else. A
// `writable:` extra the PID named ABSOLUTELY and the STORE OF RECORD are
// bound where the host has them, render no mount where it does not, and are
// never created — and each absence is said out loud at a surface an operator
// reads before the launch. A RELATIVE extra keeps its prep, because it
// resolves inside the session tree posse made and retires.
//
// EVERY ARM RUNS ON DARWIN, the rule this mechanism's pins all follow: the
// render is pure but for the prep, the prep is `os.MkdirAll` either way, and
// the mechanism is a parameter (`wallMechanismForTest`, `bwrapBin`). What no
// arm here claims is a byte-exact Linux render — `SeatbeltWritable` reads
// `runtime.GOOS` for the home-cache grants, which is `homeCacheGrants`' own
// pin.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bpaRender renders this fixture's bwrap wall AND runs its prep, which is
// what a launch does (RenderBwrap → runBwrapPrep). The render alone would
// measure the argv and miss the whole subject: what the launcher WRITES.
func bpaRender(t *testing.T, s bwlSeat) (bwrapRendered, error) {
	t.Helper()
	gates := s.a.GatesDir(s.ag.Name)
	if err := os.MkdirAll(gates, 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := s.a.bwrapRender(s.ag, s.rt, s.dir, gates)
	if err != nil {
		return r, err
	}
	return r, runBwrapPrep(r.Prep)
}

// bpaBinds reports whether the argv binds p read-write, by the pair the
// render emits and not by a substring: a `--ro-bind` of the same path and a
// bind of a longer path that starts with it are both the wrong answer.
func bpaBinds(argv []string, p string) bool {
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == "--bind" && argv[i+1] == p && argv[i+2] == p {
			return true
		}
	}
	return false
}

// bpaPreps reports whether the launcher was asked to make p.
func bpaPreps(prep []PrepStep, p string) bool {
	for _, s := range prep {
		if s.Path == p {
			return true
		}
	}
	return false
}

// ─── member 1: a `writable:` extra the PID named absolutely ─────────────────

// The defect, in both directions, at the one function that writes. The
// ABSENT arm is the bead: posse must not make `/opt/shared`. The PRESENT arm
// is the control that keeps the fix from being "drop every absolute grant" —
// a host that has the directory is walled exactly as it was.
func TestQABwrapPrepDoesNotCreateAnAbsoluteWritableExtra(t *testing.T) {
	t.Parallel()
	host := absResolve(t.TempDir())
	absent := filepath.Join(host, "shared-absent")
	present := filepath.Join(host, "shared-present")
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatal(err)
	}

	s := bwlFixture(t)
	s.ag.Writable = []string{absent, present, "scratch"}
	relative := filepath.Join(absResolve(s.dir), "scratch")

	r, err := bpaRender(t, s)
	if err != nil {
		t.Fatalf("a PID naming a `writable:` this host does not have must still render a wall and launch — the grant is narrower, not a refusal: %v", err)
	}

	// The whole of the finding: the host did not gain a directory.
	if _, serr := os.Stat(absent); serr == nil {
		t.Errorf("the launcher CREATED %s — an absolute `writable:` is a host path the operator declared, not one posse derived, and making it turns one PID line into a silent host write (ADR 0080 D23; this is /opt/shared on ci.yml's runner)", absent)
	}
	if bpaPreps(r.Prep, absent) {
		t.Errorf("the render asked the launcher to make %s:\n%v", absent, bwPrep(r.Prep))
	}
	if bpaBinds(r.Argv, absent) {
		t.Errorf("the render binds %s, which is not there — bwrap would refuse the launch at `Can't find source path`:\n%s", absent, bwJoin(r.Argv))
	}
	if !containsString(r.Absent, absent) {
		t.Errorf("an absent no-create grant must be REPORTED and not merely dropped — a grant that silently disappears is the wall-with-a-note shape this tier refuses. Absent = %v", r.Absent)
	}

	// The control, same render: a host that HAS the path is granted it.
	if !bpaBinds(r.Argv, present) {
		t.Errorf("%s is on this host and must be bound read-write exactly as before — D23 is about what posse CREATES, not about which paths it grants:\n%s", present, bwJoin(r.Argv))
	}
	if containsString(r.Absent, present) {
		t.Errorf("%s exists and must not be reported absent: %v", present, r.Absent)
	}

	// And the other half of the line D23 draws: a RELATIVE extra resolves
	// inside the session tree, which posse made and retires with the
	// session, so it keeps its prep. Without this the grant would be dead
	// for every persona that denies file writes, since cwd itself is not
	// granted to one.
	if !bpaPreps(r.Prep, relative) {
		t.Errorf("a relative `writable:` extra must still be created by the launcher — it is inside the session tree, and for a persona denied file writes nothing else can make it:\n%v", bwPrep(r.Prep))
	}
	if _, serr := os.Stat(relative); serr != nil {
		t.Errorf("the launcher did not make %s: %v", relative, serr)
	}
	if !bpaBinds(r.Argv, relative) {
		t.Errorf("the render does not bind %s:\n%s", relative, bwJoin(r.Argv))
	}
}

// A FILE at an absolute `writable:` is still B8's refusal, named and loud.
// This is the arm that keeps D23 from quietly swallowing the one failure
// shape this mechanism must never render: a bound file loses every atomic
// replace its writer makes, exit 0, nothing said (ranger-base-cypy1 with the
// grant in place).
func TestQABwrapAnAbsoluteWritableFileIsStillB8sRefusal(t *testing.T) {
	t.Parallel()
	file := filepath.Join(absResolve(t.TempDir()), "config.json")
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := bwlFixture(t)
	s.ag.Writable = []string{file}
	_, err := bpaRender(t, s)
	if err == nil {
		t.Fatal("a `writable:` naming a FILE must refuse the render: binding one refuses the atomic replace its writer makes (probe B8), which is worse than no grant")
	}
	if !strings.Contains(err.Error(), file) || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("the refusal must name the path and the reason, so D23's drop cannot be read as having handled it: %v", err)
	}
}

// ─── member 2: the store of record ──────────────────────────────────────────

// The second half of the bead, at the launch that caused it. A persona that
// denies file writes gets `<cwd>/.beads` as its own grant (SeatbeltWritable),
// and in a tree with no store that grant used to be a `mkdir -p`.
//
// Driven through planLaunch and not through the render, because what the
// incident was is a LAUNCH leaving a store behind for the next reader of the
// same tree — and the second half of the assertion is exactly that reader:
// ADR 0013 §4's row, asked after the launch, in the same directory.
func TestQABwrapLaunchDoesNotMaterializeTheStoreOfRecord(t *testing.T) {
	wallMechanismForTest(t, "linux")
	bwlStubBin(t, "/usr/bin/bwrap")
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
	// Not claude: D19's config-dir demand is its own pin, not a precondition
	// of this one (bwlSeat's reason).
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
		[]byte("command: mycli {file}\nstate_dir: ~/.mycli-bpa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// `deny: [Edit, Write]` is what makes `<cwd>/.beads` a grant of its own
	// rather than a path inside a granted cwd — deniesFileWrite, the branch
	// the incident ran through.
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\ndeny: [Edit, Write]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	store := filepath.Join(absResolve(repo), beadsDirName)
	ag, err := a.LoadAgent("ranger")
	if err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("mycli")
	if err != nil {
		t.Fatal(err)
	}
	// The row BEFORE the launch, so the comparison below is against a
	// measured baseline and not against a sentence this test assumes.
	before := a.CheckParityIn(ag, rt, CageSeatbelt, DefaultTier, repo).Realized[RecordReachGate].Detail
	if !strings.Contains(before, "nothing to reach") {
		t.Fatalf("fixture: a tree with no .beads must read `nothing to reach` before anything launches in it, got %q", before)
	}

	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("planLaunch at the bwrap mechanism in a tree with no store of record: %v", err)
	}

	if _, serr := os.Stat(store); serr == nil {
		t.Errorf("the launch CREATED %s. The store of record's EXISTENCE is read as a fact — ADR 0013 §4's row probes only a store that is there, and bd creating it on first write is that row's own sentence — so a launcher that makes one moves the next launch's verdict in the same tree (ranger-base-sg7a7, ADR 0080 D23)", store)
	}
	// The reader that was moved, asked again: same tree, same PIDs, same
	// answer. This is the half that would have caught the incident, and it
	// is why the assertion above is not enough on its own.
	after := a.CheckParityIn(ag, rt, CageSeatbelt, DefaultTier, repo).Realized[RecordReachGate].Detail
	if after != before {
		t.Errorf("a launch in this tree moved ADR 0013 §4's row for the next launch.\nbefore: %s\n after: %s", before, after)
	}
	// And the row says who will not create one, which is the only place an
	// operator can learn that this host's launcher differs from darwin's.
	if !strings.Contains(after, "ADR 0080 D23") {
		t.Errorf("the no-store row at the bwrap mechanism must say that the launcher does not create one either: %q", after)
	}
}

// The same row at the OTHER mechanism carries no such clause, which is the
// half that says this is a mechanism fact and not a new sentence everybody
// gets. On darwin SBPL grants the path whether or not it is there, so the
// session can make the store itself and "bd creates it on first write" needs
// nothing added to it.
func TestQATheNoStoreRowNamesTheLauncherOnlyAtTheMountMechanism(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mech  string
		cage  string
		avail bool
		says  bool
	}{
		{WallBwrap, CageSeatbelt, true, true},
		{WallSandboxExec, CageSeatbelt, true, false},
		{WallBwrap, CageShims, true, false},
		{WallBwrap, CageSeatbelt, false, false},
	} {
		got := recordReachNoStoreClause(tc.mech, tc.cage, tc.avail)
		if (got != "") != tc.says {
			t.Errorf("recordReachNoStoreClause(%s, %s, available=%v) = %q, want a clause: %v", tc.mech, tc.cage, tc.avail, got, tc.says)
		}
	}
}

// ─── the producer's own line ────────────────────────────────────────────────

// D23's two members and nothing else, read off the producer. A table here
// rather than only through a render, because the LIST is the decision: a
// future entry that joins it, or one that drops out, is a change of policy
// and should read as one.
func TestQABwrapNoCreateIsTheTwoMembersD23Names(t *testing.T) {
	t.Parallel()
	root := absResolve(t.TempDir())
	cwd := filepath.Join(root, "proj")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "host")
	ag := &AgentFile{Name: "developer", Writable: []string{host, "scratch", "~/declared-home"}}

	got := bwrapNoCreate(ag, cwd)
	for _, want := range []string{
		host, // an absolute extra
		absResolve(ExpandTilde("~/declared-home")), // `~` is absolute too (homeCacheGrants' reason)
		filepath.Join(cwd, beadsDirName),           // the store, as cwd spells it
	} {
		if !containsString(got, want) {
			t.Errorf("bwrapNoCreate does not name %s:\n%v", want, got)
		}
	}
	// The relative extra is NOT in it: it is inside the session tree.
	if rel := filepath.Join(cwd, "scratch"); containsString(got, rel) {
		t.Errorf("a relative `writable:` extra must keep its prep — %s is in the no-create list:\n%v", rel, got)
	}
	// And the gates dir, the state dir and the memory dir are the furniture
	// this decision exists to keep creating. Asked of the producer rather
	// than of a render, because their absence from this list is the whole
	// reason a fresh home still comes up.
	for _, furniture := range []string{filepath.Join(root, "home", "state"), filepath.Join(root, "home", "personas", "developer")} {
		if containsString(got, furniture) {
			t.Errorf("%s is posse's own and must stay prep-creatable:\n%v", furniture, got)
		}
	}
}

// Why the store has ONE spelling in that list and not the two the key has.
// SeatbeltWritable grants the store under `<cwd>/.beads` and, where a
// redirect moves it, under beadsHome's resolved target — and the second can
// never be absent, because beadsRedirectHop follows a redirect only after
// stat'ing its target as a directory. So an entry for the target would be a
// control with no failing arm, and this is the measurement that says so
// rather than a sentence asserting it.
func TestQABwrapTheStoreHasOneSpellingThatCanBeAbsent(t *testing.T) {
	t.Parallel()
	root := absResolve(t.TempDir())
	cwd := filepath.Join(root, "proj")
	local := filepath.Join(cwd, beadsDirName)
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(target string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(local, beadsRedirect), []byte(target+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A redirect naming a directory that is NOT there is not followed, so
	// the store resolves back to the one spelling the no-create list carries.
	gone := filepath.Join(root, "elsewhere", ".beads")
	write(gone)
	if got := beadsHome(cwd); got != local {
		t.Fatalf("a redirect to a path that is not a directory must not be followed, got beadsHome = %s, want %s", got, local)
	}
	if got := bwrapNoCreate(nil, cwd); !containsString(got, local) || containsString(got, absResolve(gone)) {
		t.Errorf("the no-create list must name the local store and nothing bd is not reading:\n%v", got)
	}

	// And one that IS there is followed — and is therefore a path no prep
	// can ever be asked to make.
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	write(gone)
	if got := beadsHome(cwd); got != gone {
		t.Fatalf("a redirect to a real directory must be followed, got beadsHome = %s, want %s", got, gone)
	}
	if _, _, absentErr := bwrapBindExisting([]string{gone}, []string{gone}); absentErr != nil {
		t.Fatal(absentErr)
	}
	if _, abs, _ := bwrapBindExisting([]string{gone}, []string{gone}); len(abs) != 0 {
		t.Errorf("a followed redirect target exists by construction, so nothing can report it absent: %v", abs)
	}
}

// ─── the three surfaces that say so ─────────────────────────────────────────

// The parity row: a declared difference, never degradation, and silent at
// the other mechanism. Degrading it would put a safe-by-design narrowing
// behind --allow-degraded and refuse dispatch for a path that is nobody's
// gate, which is the reading ranger-base-d17a closed.
func TestQAAnAbsoluteWritableThisHostLacksIsADeclaredDifference(t *testing.T) {
	t.Parallel()
	root := absResolve(t.TempDir())
	absent := filepath.Join(root, "not-here")
	present := filepath.Join(root, "here")
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatal(err)
	}
	ag := &AgentFile{Name: "developer", Writable: []string{absent, present, "scratch"}}

	d := bwrapAbsoluteWritableDifference(WallBwrap, CageSeatbelt, true, ag)
	if d == "" {
		t.Fatal("an absolute `writable:` this host does not have must be said before the launch — the alternative is a failed write inside the seat, which reads as a wall nobody can explain")
	}
	if !strings.Contains(d, AbbrevHome(absent)) {
		t.Errorf("the row must name the path the operator has to go and create or re-spell: %s", d)
	}
	for _, quiet := range []string{AbbrevHome(present), "scratch"} {
		if strings.Contains(d, quiet) {
			t.Errorf("the row names %s, which is not absent and not a host path: %s", quiet, d)
		}
	}
	// Silent everywhere the decision does not apply: the other mechanism
	// grants an absent path, shims has no file wall, and an unavailable tier
	// is a claim about a wall nobody has.
	for _, tc := range []struct{ mech, cage string }{
		{WallSandboxExec, CageSeatbelt},
		{WallBwrap, CageShims},
		{WallBwrap, CageContainer},
	} {
		if got := bwrapAbsoluteWritableDifference(tc.mech, tc.cage, true, ag); got != "" {
			t.Errorf("bwrapAbsoluteWritableDifference(%s, %s) must be silent: %s", tc.mech, tc.cage, got)
		}
	}
	if got := bwrapAbsoluteWritableDifference(WallBwrap, CageSeatbelt, false, ag); got != "" {
		t.Errorf("a tier this host cannot realize gets no row about the shape of its grants: %s", got)
	}
}

// …and it reaches a real parity matrix in the DeclaredDifference list and in
// nothing else. CheckParityIn and not CheckParity, because the row stats,
// which is the line parity.go's header draws.
func TestQATheAbsoluteWritableRowDegradesNoLaunch(t *testing.T) {
	wallMechanismForTest(t, "linux")
	bwlStubBin(t, "/usr/bin/bwrap")
	had := AvailableCages[CageSeatbelt]
	AvailableCages[CageSeatbelt] = true
	t.Cleanup(func() {
		if !had {
			delete(AvailableCages, CageSeatbelt)
		}
	})

	b, _ := newTestBackend(t)
	a := b.App
	absent := filepath.Join(absResolve(t.TempDir()), "not-here")
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: grok\ncage: seatbelt\nwritable: ["+absent+"]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ag, err := a.LoadAgent("ranger")
	if err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	p := a.CheckParityIn(ag, rt, CageSeatbelt, DefaultTier, t.TempDir())
	var found bool
	for _, d := range p.DeclaredDifference {
		if strings.Contains(d, AbbrevHome(absent)) {
			found = true
		}
	}
	if !found {
		t.Errorf("the row does not reach a real matrix: %v", p.DeclaredDifference)
	}
	for _, d := range append(append([]string{}, p.Degraded...), p.Ratified...) {
		if strings.Contains(d, AbbrevHome(absent)) {
			t.Errorf("a `writable:` extra is nobody's gate and must never degrade a launch or need --allow-degraded (ADR 0017 §2): %s", d)
		}
	}
	// A launch, to say the same thing as behaviour rather than as a field:
	// the seat comes up, narrower, with no --allow-degraded anywhere.
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Errorf("the launch must proceed — the grant is narrower, not a gate the wall failed to realize: %v", err)
	}
}

// The wall's own readable copy, which is the one place an operator can read
// what the launcher did and did not do on the host. The prep list was that
// place for the writes; this is the other half of the same sentence.
func TestQATheReadableWallNamesTheGrantsItDidNotCreate(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(absResolve(t.TempDir()), "not-here")
	s := bwlFixture(t)
	s.ag.Writable = []string{absent}
	gates := s.a.GatesDir(s.ag.Name)
	if err := os.MkdirAll(gates, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.a.RenderBwrap(s.ag, s.rt, s.dir); err != nil {
		t.Fatal(err)
	}
	txt, err := os.ReadFile(filepath.Join(gates, bwrapTextName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(txt), AbbrevHome(absent)) || !strings.Contains(string(txt), "ADR 0080 D23") {
		t.Errorf("%s must name the declared grant this render made no mount for and did not create — nothing else written by a launch says it:\n%s", bwrapTextName, txt)
	}
	// And the argv file, which IS the wall, must not name it at all.
	args, err := os.ReadFile(filepath.Join(gates, bwrapArgsName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), absent) {
		t.Errorf("%s binds a source that is not there; bwrap would refuse the launch at `Can't find source path`:\n%s", bwrapArgsName, args)
	}
}

// And `posse agent check`, where the operator writing the PID is standing —
// the surface the bead asked for by name. A WARNING and not an exit-1
// finding: the PID is coherent and may well be launched on a host that has
// the path.
func TestQAAgentCheckWarnsOnAnAbsoluteWritable(t *testing.T) {
	t.Parallel()
	a, _ := bpaCheckApp(t, "writable: [/opt/shared, docs/adr]\n")
	findings, warnings, err := a.CheckAgent("ranger")
	if err != nil {
		t.Fatal(err)
	}
	var warned bool
	for _, w := range warnings {
		if strings.Contains(w, "/opt/shared") && strings.Contains(w, "ADR 0080 D23") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("`posse agent check` says nothing about an absolute `writable:` — which is where an operator writing that PID would hear it:\n%v", warnings)
	}
	for _, w := range warnings {
		if strings.Contains(w, "docs/adr") && strings.Contains(w, "ADR 0080 D23") {
			t.Errorf("a RELATIVE extra resolves inside the session tree and must not warn: %s", w)
		}
	}
	for _, f := range findings {
		if strings.Contains(f, "ADR 0080 D23") {
			t.Errorf("this must not fail the lint: the PID is coherent, and the host it lands on is what decides whether the grant is realized — %s", f)
		}
	}
}

// bpaCheckApp is a home with one loadable PID carrying `extra` in its
// frontmatter, for a lint arm. Separate from bwlFixture because CheckAgent
// reads the FILE and the agents dir, where that fixture builds the structs.
func bpaCheckApp(t *testing.T, extra string) (*App, string) {
	t.Helper()
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.AgentsDir, "ranger.md")
	body := "---\nname: ranger\ndescription: t\nruntime: grok\ncage: seatbelt\n" + extra + "---\n" +
		"You are the ranger of the crew.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return a, path
}
