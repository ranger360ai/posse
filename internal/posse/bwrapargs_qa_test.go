//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-qoamy — ADR 0080 D17's render, the pure function
// from the L2 lists to a bwrap argv.
//
// EVERY ARM RUNS ON DARWIN, which is the point of the render being pure: the
// function stats a fixture tree and appends strings, so the only thing the
// host decides is whether the fixture is there. The mechanism it renders for
// is Linux's and no arm here puts a wall up — that is D18's live arm and the
// probe's (docs/adr/0080-bwrap-tier.probe.sh).
//
// WHAT MAKES THESE WORTH PINNING RATHER THAN READING. Three of the four
// failure modes in this file are SILENT at the mechanism:
//
//  1. mount ORDER. bwrap applies mounts in argv order and a later mount
//     covers an earlier one, so a nested grant emitted before its container
//     is buried and the seat silently loses the narrower grant. Nothing
//     refuses it; the argv is well-formed either way.
//  2. a mount NOT RENDERED for a path that is not there yet. A deny whose
//     path the session can still create is a hole that opens the first time
//     it does, and every signal at launch says the wall went up.
//  3. a writable FILE. B8 measured that an atomic replace of a bound file is
//     refused, which is ranger-base-cypy1's lost-write failure WITH the
//     grant in place — exit 0, the CLI printing that it saved, the file
//     unchanged. The render refuses so that it cannot be reached.
//  4. and the one that is loud: a path posse could not stat. Pinned anyway,
//     because the tempting spelling (os.IsNotExist alone) turns it into
//     case 2.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bwTree makes a fixture tree and answers its resolved root plus a joiner.
// Resolved, because every path BwrapArgs is handed in production has been
// through absResolve already (SeatbeltWritable, SeatbeltCarveOut) and the
// renderer deliberately does not resolve a second time — a fixture spelled
// /var/folders/... where the tree is at /private/var/folders/... would be
// testing a resolution this function does not do.
func bwTree(t *testing.T, dirs []string, files []string) (string, func(...string) string) {
	t.Helper()
	root := absResolve(t.TempDir())
	at := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	for _, d := range dirs {
		if err := os.MkdirAll(at(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(at(f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at(f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, at
}

// bwJoin renders an argv as one line, for an assertion that is about the
// ORDER of two mounts rather than about the whole render.
func bwJoin(argv []string) string { return strings.Join(argv, " ") }

// bwPrep renders the prep list the same way, so a table row can name it.
func bwPrep(prep []PrepStep) []string {
	var out []string
	for _, s := range prep {
		out = append(out, s.String())
	}
	return out
}

// ─── the base ────────────────────────────────────────────────────────────────

// D17 item 1, in its exact order and with nothing else in it. The base is
// the one part of the argv that is the same at every seat, and it is also
// what bwrapApplyProbeArgv predicts — a host that refuses --ro-bind / /
// refuses every render of this function, which is why the probe is shaped
// like it (bwrap.go).
func TestBwrapArgsRendersD17Base(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj"}, nil)

	argv, prep, err := BwrapArgs(at("proj"), nil, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--die-with-parent",
		"--ro-bind", "/", "/",
		"--dev-bind", "/dev", "/dev",
		"--chdir", at("proj"),
	}
	if bwJoin(argv) != bwJoin(want) {
		t.Fatalf("base argv =\n  %v\nwant\n  %v", argv, want)
	}
	if len(prep) != 0 {
		t.Fatalf("a render with no grants asked the launcher for %v", bwPrep(prep))
	}
}

// The flags D17's Alternatives rejected, each for a thing the seat needs from
// the host: its own pids (a seat kills them and checkorphans reads the box's
// table), egress (the container tier's), the shared TMPDIR paths, and a
// controlling terminal for the TUI. An assertion of ABSENCE, so it is written
// against the flag SPELLINGS rather than against a count — and it is the arm
// that would catch a later bead reaching for a namespace flag to close
// something else.
func TestBwrapArgsNeverUnsharesWhatTheSeatNeeds(t *testing.T) {
	t.Parallel()
	root, at := bwTree(t, []string{"proj", "state"}, []string{"cred"})

	argv, _, err := BwrapArgs(at("proj"),
		[]string{at("proj"), at("state")},
		SeatbeltCarveOut{Deny: []string{at("state", "gates")}, DenyRead: []string{at("cred")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, never := range []string{"--unshare-pid", "--unshare-net", "--unshare-all", "--proc", "--tmpfs", "--new-session"} {
		for _, a := range argv {
			if a == never {
				t.Fatalf("%s is in the render (ADR 0080 D17, Alternatives rejected): %v", never, argv)
			}
		}
	}
	_ = root
}

// A render with no session dir — `posse gates <persona>` asks about a persona
// and not about a launch — omits --chdir rather than passing it "". bwrap
// would refuse an empty argument, so this is the difference between a surface
// that prints a wall and one that prints an error.
func TestBwrapArgsWithNoCwdOmitsChdir(t *testing.T) {
	t.Parallel()
	argv, _, err := BwrapArgs("", nil, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range argv {
		if a == "--chdir" {
			t.Fatalf("--chdir rendered with no cwd, argument %q: %v", argv[i+1:], argv)
		}
		if a == "" {
			t.Fatalf("an empty argument at %d: %v", i, argv)
		}
	}
}

// ─── shape 1: the write grants ───────────────────────────────────────────────

// Shallowest first, which is the whole of what the order has to buy: the
// container is mounted before the nested grant, or the nested mount is buried
// under it and the seat silently loses the narrower grant. The fixture is the
// shape every dispatched session has — cwd and cwd/.beads — plus a third
// grant that sorts between them, so the arm measures an ORDER and not a pair.
func TestBwrapArgsBindsWriteGrantsShallowestFirst(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj/.beads", "proj/.git", "state"}, nil)

	// Handed deepest-first, so a renderer that preserved input order fails.
	argv, prep, err := BwrapArgs(at("proj"),
		[]string{at("state"), at("proj", ".git"), at("proj", ".beads"), at("proj")},
		SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	for _, p := range []string{at("proj"), at("proj", ".beads"), at("proj", ".git"), at("state")} {
		if !strings.Contains(line, "--bind "+p+" "+p) {
			t.Fatalf("no rw bind of %s:\n%s", p, line)
		}
	}
	// The containment pairs, each asserted as a position: this is the claim,
	// and "they are all present" is not.
	for _, pair := range [][2]string{
		{at("proj"), at("proj", ".beads")},
		{at("proj"), at("proj", ".git")},
	} {
		if strings.Index(line, "--bind "+pair[0]+" ") > strings.Index(line, "--bind "+pair[1]+" ") {
			t.Fatalf("%s is mounted after %s, which buries it:\n%s", pair[0], pair[1], line)
		}
	}
	if len(prep) != 0 {
		t.Fatalf("grants that are all there asked for prep: %v", bwPrep(prep))
	}
}

// A grant that is not there yet. seatbelt granted such paths before they
// existed (resolveExisting) and bwrap cannot bind a source that is not there,
// so the launcher makes it — and the mount is still rendered, because the
// grant is real the moment the prep runs.
func TestBwrapArgsPrepsAMissingWriteGrant(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj"}, nil)
	missing := at("proj", ".beads")

	argv, prep, err := BwrapArgs(at("proj"), []string{at("proj"), missing}, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	if got := bwPrep(prep); len(got) != 1 || got[0] != "mkdir -p "+missing {
		t.Fatalf("prep = %v, want one mkdir of %s", got, missing)
	}
	if prep[0].Kind != PrepMkdirAll || prep[0].Why == "" {
		t.Fatalf("step = %+v; a prep step with no reason beside it is a host write nobody can check", prep[0])
	}
	if !strings.Contains(bwJoin(argv), "--bind "+missing+" "+missing) {
		t.Fatalf("the missing grant was prepped and not mounted: %v", argv)
	}
}

// B8: a writable entry that is a regular FILE is refused, naming the file.
// The two entries in today's writable set that reach here are the two the
// next beads remove upstream — claude's config file (D19) and git's ref leaf
// with its .lock sibling (D20) — so this error is what makes those beads
// prerequisites of a launch rather than improvements to one.
//
// And the render refuses WHOLE: no argv, no prep. A partial render plus an
// error is a thing a caller can use by accident.
func TestBwrapArgsRefusesAWritableRegularFile(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj"}, []string{"claude.json"})
	file := at("claude.json")

	argv, prep, err := BwrapArgs(at("proj"), []string{at("proj"), file}, SeatbeltCarveOut{})
	if err == nil {
		t.Fatalf("a writable regular file rendered a wall: %v", argv)
	}
	if !strings.Contains(err.Error(), file) {
		t.Fatalf("the refusal does not name the file: %v", err)
	}
	// The remedy, not just the refusal: an operator reading this has to know
	// that the answer is a narrower writable set and not a wider wall.
	for _, want := range []string{"not a directory", "D19", "D20"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal does not say %q: %v", want, err)
		}
	}
	if argv != nil || prep != nil {
		t.Fatalf("a refused render returned argv %v and prep %v", argv, bwPrep(prep))
	}
}

// ─── shape 2: the denies ─────────────────────────────────────────────────────

// The deny goes over the grant that contains it, which is last-match-wins at
// the other mechanism: SBPL puts the deny block after the allow block, bwrap
// puts the ro-bind after the bind. An order assertion and not a presence one,
// for the same reason as the nesting arm above.
func TestBwrapArgsRoBindsEachDenyOverItsGrant(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj/agents", "state/gates"}, nil)

	argv, prep, err := BwrapArgs(at("proj"),
		[]string{at("proj"), at("state")},
		SeatbeltCarveOut{Deny: []string{at("state", "gates"), at("proj", "agents")}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	for _, d := range []string{at("proj", "agents"), at("state", "gates")} {
		if !strings.Contains(line, "--ro-bind "+d+" "+d) {
			t.Fatalf("no read-only bind of %s:\n%s", d, line)
		}
	}
	// Every write grant is mounted before every deny: the deny is the later
	// statement at this mechanism exactly as it is at the other.
	lastBind := strings.LastIndex(line, "--bind "+at("state")+" ")
	if firstDeny := strings.Index(line, "--ro-bind "+at("proj", "agents")+" "); firstDeny < lastBind {
		t.Fatalf("a deny is mounted before a write grant, so the grant buries it:\n%s", line)
	}
	if len(prep) != 0 {
		t.Fatalf("denies that are all there asked for prep: %v", bwPrep(prep))
	}
}

// A deny whose path is not there and whose containing grant IS: the session
// can create it, so an unmounted name is a hole that opens the first time it
// does. The launcher makes it as an empty directory and the mount goes over
// that.
func TestBwrapArgsPrepsAMissingDenyInsideAGrant(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj", "state"}, nil)
	deny := at("state", "gates")

	argv, prep, err := BwrapArgs(at("proj"), []string{at("proj"), at("state")},
		SeatbeltCarveOut{Deny: []string{deny}})
	if err != nil {
		t.Fatal(err)
	}
	if got := bwPrep(prep); len(got) != 1 || got[0] != "mkdir -p "+deny {
		t.Fatalf("prep = %v, want one mkdir of %s", got, deny)
	}
	if !strings.Contains(bwJoin(argv), "--ro-bind "+deny+" "+deny) {
		t.Fatalf("the created deny was not mounted: %v", argv)
	}
}

// And the other half, which is what keeps the arm above from being "mkdir
// every deny": a deny no grant contains cannot be brought into being from
// inside the wall, so it is neither created on the host nor mounted. The
// launcher runs as the seat's user and outside the namespace — every prep
// step is a real write on the box in the seat's name, so a step with no
// reason is a directory posse leaves behind for nothing.
func TestBwrapArgsRendersNothingForAMissingDenyOutsideEveryGrant(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj"}, nil)
	deny := at("elsewhere", "promoted")

	argv, prep, err := BwrapArgs(at("proj"), []string{at("proj")},
		SeatbeltCarveOut{Deny: []string{deny}})
	if err != nil {
		t.Fatal(err)
	}
	if len(prep) != 0 {
		t.Fatalf("a deny nothing can create was created anyway: %v", bwPrep(prep))
	}
	if strings.Contains(bwJoin(argv), deny) {
		t.Fatalf("a deny whose source does not exist was mounted — bwrap would refuse this argv: %v", argv)
	}
	if _, err := os.Stat(deny); !os.IsNotExist(err) {
		t.Fatalf("the render touched the tree: %s exists (%v)", deny, err)
	}
}

// THE DIVERGENCE ARM (and the reason BwrapArgs asks "is the deny inside a
// write grant" rather than D17's literal "is its nearest EXISTING ancestor
// inside a write grant").
//
// The two readings disagree on exactly one shape, and this fleet has it: a
// write grant that does not exist yet. Here state/ is a grant and is absent,
// state/gates is a deny, and the deny's nearest existing ancestor is the
// root, which no grant covers — so D17's letter renders no mount while the
// grant one line above is about to be created and bound rw. The session could
// then make state/gates and write in it, behind a wall that reported success.
//
// MUTATION-CHECKED: implementing the letter (nearest existing ancestor) reds
// this arm alone and leaves every other arm in this file green.
func TestBwrapArgsPrepsAMissingDenyInsideAMissingGrant(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj"}, nil)
	grant, deny := at("state"), at("state", "gates")

	argv, prep, err := BwrapArgs(at("proj"), []string{at("proj"), grant},
		SeatbeltCarveOut{Deny: []string{deny}})
	if err != nil {
		t.Fatal(err)
	}
	if got := bwPrep(prep); len(got) != 2 || got[0] != "mkdir -p "+grant || got[1] != "mkdir -p "+deny {
		t.Fatalf("prep = %v, want the grant then the deny inside it", got)
	}
	line := bwJoin(argv)
	if !strings.Contains(line, "--ro-bind "+deny+" "+deny) {
		t.Fatalf("a deny inside a grant that is about to be created got no mount — the session can make it:\n%s", line)
	}
	if strings.Index(line, "--bind "+grant+" ") > strings.Index(line, "--ro-bind "+deny+" ") {
		t.Fatalf("the deny is mounted before the grant that contains it:\n%s", line)
	}
}

// Seal renders NOTHING, and this is the arm that says so rather than a
// comment: the renderer ignoring one of its four inputs is otherwise
// indistinguishable from a renderer that forgot it.
//
// It is a decision with two measurements under it (B3, B4). At L2 the seal
// exists because a subpath deny is a statement about a PATH, so a rename
// carries the denied tree out from under its own deny; a mount is a statement
// about a mount POINT, and renaming a mountpoint or an ancestor is refused.
//
// Asserted as "the render is byte-identical with the seal and without it",
// which is stronger than grepping for the path: the seal's directories are
// also grants, so their names are in the argv either way.
func TestBwrapArgsRendersNothingForSeal(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj/agents", "state/gates"}, nil)
	writable := []string{at("proj"), at("state")}
	carve := SeatbeltCarveOut{Deny: []string{at("proj", "agents"), at("state", "gates")}}

	bare, barePrep, err := BwrapArgs(at("proj"), writable, carve)
	if err != nil {
		t.Fatal(err)
	}
	sealed := carve
	sealed.Seal = []string{at("proj"), at("state"), at("elsewhere")}
	withSeal, sealPrep, err := BwrapArgs(at("proj"), writable, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if bwJoin(bare) != bwJoin(withSeal) {
		t.Fatalf("the seal changed the argv:\n  without %v\n  with    %v", bare, withSeal)
	}
	if len(barePrep) != 0 || len(sealPrep) != 0 {
		t.Fatalf("the seal asked the launcher for %v (control %v)", bwPrep(sealPrep), bwPrep(barePrep))
	}
}

// ─── Keep: the session's own audit trail ─────────────────────────────────────

// The two logs are bound back rw OVER the deny that closed the gates dir, and
// as files: --bind and not --ro-bind, because an append to a bound file is an
// in-place write, which is the one thing B8 does not touch (B8 is a rename).
// Under sandbox-exec they are created on first append from inside the cage; a
// bind has no such move, so the launcher makes them.
func TestBwrapArgsBindsKeepFilesBackOverTheDeny(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj", "state/gates/dev"}, []string{"state/gates/dev/refusals.log"})
	gates := at("state", "gates")
	have, missing := at("state", "gates", "dev", "refusals.log"), at("state", "gates", "dev", "shell.log")

	argv, prep, err := BwrapArgs(at("proj"),
		[]string{at("proj"), at("state"), at("state", "gates", "dev")},
		SeatbeltCarveOut{Deny: []string{gates}, Keep: []string{have, missing}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	for _, f := range []string{have, missing} {
		if !strings.Contains(line, "--bind "+f+" "+f) {
			t.Fatalf("no rw bind of the audit log %s:\n%s", f, line)
		}
		// Read-only would be the same wall with the record lost: both are
		// appended to from INSIDE the namespace.
		if strings.Contains(line, "--ro-bind "+f+" "+f) {
			t.Fatalf("the audit log %s is bound read-only:\n%s", f, line)
		}
	}
	// After the deny, or the deny buries it and the session's own refusals
	// log is unwritable behind a wall that reported success.
	if strings.Index(line, "--ro-bind "+gates+" ") > strings.Index(line, "--bind "+have+" ") {
		t.Fatalf("a Keep file is mounted before the deny it is re-allowed over:\n%s", line)
	}
	// Only the missing one is created, and as a FILE.
	if got := bwPrep(prep); len(got) != 1 || got[0] != "touch "+missing {
		t.Fatalf("prep = %v, want one touch of %s", got, missing)
	}
	if prep[0].Kind != PrepTouch || prep[0].Why == "" {
		t.Fatalf("step = %+v", prep[0])
	}
}

// A DIRECTORY at a Keep path is refused. The Keep list is two literal files
// (SeatbeltCarveOut), and binding a directory there would hand the session
// back the tree the deny just closed — a wider wall than anything asked for,
// and one no reader could spot in the argv. Fail-closed, which is the tier's
// own direction.
func TestBwrapArgsRefusesADirectoryAtAKeepPath(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj", "state/gates/refusals.log"}, nil)
	keep := at("state", "gates", "refusals.log")

	argv, prep, err := BwrapArgs(at("proj"), []string{at("proj"), at("state")},
		SeatbeltCarveOut{Deny: []string{at("state", "gates")}, Keep: []string{keep}})
	if err == nil {
		t.Fatalf("a directory at a Keep path rendered a wall: %v", argv)
	}
	if !strings.Contains(err.Error(), keep) {
		t.Fatalf("the refusal does not name the path: %v", err)
	}
	if argv != nil || prep != nil {
		t.Fatalf("a refused render returned argv %v and prep %v", argv, bwPrep(prep))
	}
}

// ─── shape 5: the credential read masks ──────────────────────────────────────

// /dev/null over the file, so a read gets no bytes. A MISSING one renders
// nothing and asks for no prep: creating another runtime's credential path to
// mask it would be posse writing into a store that is not its business, and
// the residual is stated rather than closed (D17 Consequences — a credential
// mask is a snapshot at launch, and parity names each missing file).
func TestBwrapArgsMasksEachExistingDenyReadAndSkipsTheRest(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj"}, []string{"codex/auth.json"})
	have, missing := at("codex", "auth.json"), at("grok", "auth.json")

	argv, prep, err := BwrapArgs(at("proj"), []string{at("proj")},
		SeatbeltCarveOut{DenyRead: []string{have, missing}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	if !strings.Contains(line, "--ro-bind /dev/null "+have) {
		t.Fatalf("no /dev/null mask over %s:\n%s", have, line)
	}
	if strings.Contains(line, missing) {
		t.Fatalf("a mask was rendered over a path that is not there — bwrap would refuse this argv:\n%s", line)
	}
	if len(prep) != 0 {
		t.Fatalf("the read mask asked the launcher to create something: %v", bwPrep(prep))
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("the render created %s (%v)", missing, err)
	}
}

// ─── the whole render, and its order ─────────────────────────────────────────

// One fixture carrying every shape, pinned as the EXACT argv. The per-shape
// arms above each measure one claim; this one measures the thing no single
// claim is: the five groups in D17's order, with the prep list in the order
// the launcher must run it.
//
// The prep ordering here is load-bearing and is why the list is a list: the
// Keep file lives inside a DENY directory that is itself not there yet, so
// its touch can only succeed after that deny's mkdir, which can only succeed
// after the grant above it. Three steps, in that order, from three different
// shapes.
func TestBwrapArgsRendersTheFiveShapesInD17Order(t *testing.T) {
	t.Parallel()
	root, at := bwTree(t,
		[]string{"proj/.beads", "proj/agents", "state"},
		[]string{"codex/auth.json"})
	cwd := at("proj")

	writable := []string{at("proj"), at("proj", ".beads"), at("state"), at("fresh")}
	carve := SeatbeltCarveOut{
		Deny:     []string{at("proj", "agents"), at("state", "gates"), at("elsewhere", "promoted")},
		Seal:     []string{at("proj")},
		Keep:     []string{at("state", "gates", "refusals.log")},
		DenyRead: []string{at("codex", "auth.json"), at("grok", "auth.json")},
	}

	argv, prep, err := BwrapArgs(cwd, writable, carve)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--die-with-parent",
		"--ro-bind", "/", "/",
		"--dev-bind", "/dev", "/dev",
		"--chdir", cwd,
		// shape 1, shallowest first
		"--bind", at("fresh"), at("fresh"),
		"--bind", at("proj"), at("proj"),
		"--bind", at("proj", ".beads"), at("proj", ".beads"),
		"--bind", at("state"), at("state"),
		// shape 2 — elsewhere/promoted is in no grant, so no mount
		"--ro-bind", at("proj", "agents"), at("proj", "agents"),
		"--ro-bind", at("state", "gates"), at("state", "gates"),
		// Keep, back over the deny above it
		"--bind", at("state", "gates", "refusals.log"), at("state", "gates", "refusals.log"),
		// shape 5 — grok's credential is not on this box, so no mask
		"--ro-bind", "/dev/null", at("codex", "auth.json"),
	}
	if bwJoin(argv) != bwJoin(want) {
		t.Fatalf("argv =\n  %s\nwant\n  %s", strings.ReplaceAll(bwJoin(argv), root, "R"), strings.ReplaceAll(bwJoin(want), root, "R"))
	}
	wantPrep := []string{
		"mkdir -p " + at("fresh"),
		"mkdir -p " + at("state", "gates"),
		"touch " + at("state", "gates", "refusals.log"),
	}
	if got := bwPrep(prep); strings.Join(got, "\n") != strings.Join(wantPrep, "\n") {
		t.Fatalf("prep =\n  %v\nwant\n  %v", got, wantPrep)
	}
	// Nothing was written: the prep list is a REQUEST to the launcher, and a
	// renderer that acted on it could not be called twice or printed by
	// `posse gates` without changing the box.
	for _, p := range []string{at("fresh"), at("state", "gates"), at("elsewhere")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("the render created %s (%v)", p, err)
		}
	}
}

// Two renders of one launch must be the same argv: D18 writes it to a file an
// operator reads, and the relaunch path re-renders it. So the order cannot
// depend on the order the producers happened to append, and a duplicate
// cannot become a second mount — a repeated grant is one decision rendered
// twice, and an operator reading two identical lines reads two.
func TestBwrapArgsIsDeterministicAndDedupes(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj/.beads", "state/gates"}, nil)
	carve := SeatbeltCarveOut{Deny: []string{at("state", "gates"), at("state", "gates")}}

	forward, _, err := BwrapArgs(at("proj"), []string{at("proj"), at("proj", ".beads"), at("state"), at("proj"), ""}, carve)
	if err != nil {
		t.Fatal(err)
	}
	reversed, _, err := BwrapArgs(at("proj"), []string{"", at("state"), at("proj"), at("proj", ".beads")}, carve)
	if err != nil {
		t.Fatal(err)
	}
	if bwJoin(forward) != bwJoin(reversed) {
		t.Fatalf("input order changed the wall:\n  %v\n  %v", forward, reversed)
	}
	if n := strings.Count(bwJoin(forward), "--bind "+at("proj")+" "+at("proj")); n != 1 {
		t.Fatalf("the duplicated grant rendered %d mounts: %v", n, forward)
	}
	if n := strings.Count(bwJoin(forward), "--ro-bind "+at("state", "gates")); n != 1 {
		t.Fatalf("the duplicated deny rendered %d mounts: %v", n, forward)
	}
	for _, a := range forward {
		if a == "" {
			t.Fatalf("an empty entry rendered an empty argument: %v", forward)
		}
	}
}

// ─── "could not ask" is not an answer ────────────────────────────────────────

// A path posse cannot stat refuses the render, naming the path and the LIST
// it came from. The tempting spelling — os.Stat, test os.IsNotExist, treat
// every other error as "not there" — turns an unreadable path into a missing
// one, which for a deny means no mount at all: the silent failure mode of
// this whole file, reached by a chmod rather than by an edit.
//
// Reached by making the parent unreadable, so stat returns EACCES. Skipped
// when the stat succeeds anyway, which is what root sees — a skip rather than
// a pass, because a green arm here would be a green arm over nothing.
func TestBwrapArgsRefusesAPathItCannotStat(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("no mode bits to take away")
	}
	_, at := bwTree(t, []string{"proj", "closed/inner"}, nil)
	closed, inner := at("closed"), at("closed", "inner")
	if err := os.Chmod(closed, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(closed, 0o755) })
	if _, err := os.Stat(inner); err == nil || os.IsNotExist(err) {
		t.Skipf("this process can still stat %s (%v) — nothing to measure", inner, err)
	}

	for _, tc := range []struct {
		shape string
		carve SeatbeltCarveOut
		w     []string
	}{
		{"write grant", SeatbeltCarveOut{}, []string{at("proj"), inner}},
		{"deny", SeatbeltCarveOut{Deny: []string{inner}}, []string{at("proj")}},
		{"keep", SeatbeltCarveOut{Keep: []string{inner}}, []string{at("proj")}},
		{"read deny", SeatbeltCarveOut{DenyRead: []string{inner}}, []string{at("proj")}},
	} {
		argv, prep, err := BwrapArgs(at("proj"), tc.w, tc.carve)
		if err == nil {
			t.Fatalf("%s: an unreadable path rendered a wall: %v", tc.shape, argv)
		}
		if !strings.Contains(err.Error(), inner) || !strings.Contains(err.Error(), tc.shape) {
			t.Fatalf("%s: the refusal names neither the path nor the list: %v", tc.shape, err)
		}
		if argv != nil || prep != nil {
			t.Fatalf("%s: a refused render returned argv %v and prep %v", tc.shape, argv, bwPrep(prep))
		}
	}
}

// ─── the inputs are the ones the tier already computes ───────────────────────

// The render's inputs are SeatbeltWritable and SeatbeltCarveOut's output and
// nothing else — that is D16's whole claim about one tier with two mechanisms
// (ADR 0080: "Only the last step, list to wall, differs"). This arm drives it
// end to end from a real PID rather than from a hand-built list, so a change
// to either producer that this renderer cannot spell is a red here and not a
// surprise on the host.
//
// It asserts the SHAPE and not the membership: the writable set is a fact
// about this box's home, and a pin on its contents belongs with the producer.
func TestBwrapArgsRendersTheTiersOwnLists(t *testing.T) {
	t.Parallel()
	_, at := bwTree(t, []string{"proj", "home/state"}, nil)
	a := &App{Home: at("home"), StateDir: at("home", "state")}
	ag := &AgentFile{Name: "developer", MemoryDir: at("home", "personas", "developer")}
	gatesDir := filepath.Join(a.StateDir, "gates", ag.Name)

	writable := a.SeatbeltWritable(ag, at("proj"), gatesDir)
	carve := a.SeatbeltCarveOut(ag, at("proj"), gatesDir, writable)
	argv, prep, err := BwrapArgs(at("proj"), writable, carve)
	if err != nil {
		t.Fatalf("the tier's own lists do not render: %v", err)
	}
	if len(argv) < len(writable) {
		t.Fatalf("%d argv entries for %d grants: %v", len(argv), len(writable), argv)
	}
	line := bwJoin(argv)
	// The ordering this launch exercises, and the one no fixture above
	// invents: the persona's own gates dir is a GRANT, state/gates is a DENY
	// above it, and the two audit logs are Keep entries inside both. SBPL
	// resolves that by last-match-wins — the deny beats the grant, and the
	// two literals beat the deny — so the mount order has to be grant, deny,
	// Keep, in that order, or the mechanisms disagree about the same lists.
	// Reading it the other way round (the grant surviving the deny) would
	// hand the session back its own shims and seatbelt.sb, which is what
	// SeatbeltCarveOut item 2 exists to close.
	grant := strings.Index(line, "--bind "+absResolve(gatesDir)+" ")
	deny := strings.Index(line, "--ro-bind "+absResolve(filepath.Join(a.StateDir, "gates"))+" ")
	keep := strings.Index(line, "--bind "+absResolve(filepath.Join(gatesDir, "refusals.log"))+" ")
	if grant < 0 || deny < 0 || keep < 0 {
		t.Fatalf("grant/deny/keep at %d/%d/%d — one of the three lists rendered nothing:\n%s", grant, deny, keep, line)
	}
	if !(grant < deny && deny < keep) {
		t.Fatalf("the gates dir's grant, deny and Keep are mounted in order %d/%d/%d, not grant→deny→Keep:\n%s", grant, deny, keep, line)
	}
	// Every prep step names a path inside a grant: the launcher writes these
	// on the host in the seat's name, so a step outside the wall's own
	// footprint is a write posse has no business making.
	for _, s := range prep {
		if !writeGranted(writable, s.Path) {
			t.Fatalf("prep step %q is outside every write grant", s)
		}
	}
}
