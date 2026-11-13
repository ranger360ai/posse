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
// WHAT MAKES THESE WORTH PINNING RATHER THAN READING. Four of the five
// failure modes in this file are SILENT at the mechanism:
//
//  1. mount ORDER. bwrap applies mounts in argv order and a later mount
//     covers an earlier one, so a nested grant emitted before its container
//     is buried and the seat silently loses the narrower grant. Nothing
//     refuses it; the argv is well-formed either way.
//  2. a mount NOT RENDERED for a path that is not there yet. A deny whose
//     path the session can still create is a hole that opens the first time
//     it does, and every signal at launch says the wall went up.
//  3. a prep step of the wrong KIND. The prep list is the one part of the
//     wall that happens outside it: the launcher makes these paths on the
//     host, in the SHARED repo, in the seat's name. A `mkdir -p` at a path
//     its producer means as a FILE is silent at launch and loud later in
//     somebody else's git — `could not lock config file` for every reader of
//     that repo, with commit and status still working (ranger-base-ppnds).
//     The carve-out is the only thing that can tell the kinds apart, because
//     an absent path's shape cannot be stat'd.
//  4. a writable FILE. B8 measured that an atomic replace of a bound file is
//     refused, which is ranger-base-cypy1's lost-write failure WITH the
//     grant in place — exit 0, the CLI printing that it saved, the file
//     unchanged. The render refuses so that it cannot be reached.
//  5. and the one that is loud: a path posse could not stat. Pinned anyway,
//     because the tempting spelling (os.IsNotExist alone) turns it into
//     case 2.

import (
	"os"
	"path/filepath"
	"reflect"
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
	// bwAt and not `at`: cmd/testparallel resolves a called NAME against every
	// `at := func(...)` in the package, and one of those reaches the plan-clock
	// package var blindT — so every t.Parallel test here is reported as
	// reaching that var and goes UNCLEARED, which is exactly what
	// qcwDiffLines' own comment (canonicalwall_qa_test.go) records. A unique
	// name is the honest fix; a parallelOK line naming blindT would clear a
	// reason these tests do not have.
	bwAt := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	for _, d := range dirs {
		if err := os.MkdirAll(bwAt(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(bwAt(f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(bwAt(f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, bwAt
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
	_, bwAt := bwTree(t, []string{"proj"}, nil)

	argv, prep, err := BwrapArgs(bwAt("proj"), nil, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--die-with-parent",
		"--ro-bind", "/", "/",
		"--dev-bind", "/dev", "/dev",
		// ranger-base-racfc: a child user namespace, rendered and not left
		// to the bwrap build; the /proc/<host pid>/root guard
		// (ranger-base-di8da) rests on it
		"--unshare-user",
		"--chdir", bwAt("proj"),
		// ranger-base-ludpp: the agent and bus addresses, always
		"--unsetenv", "DBUS_SESSION_BUS_ADDRESS",
		"--unsetenv", "SSH_AUTH_SOCK",
		"--unsetenv", "GPG_AGENT_INFO",
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
	root, bwAt := bwTree(t, []string{"proj", "state"}, []string{"cred"})

	argv, _, err := BwrapArgs(bwAt("proj"),
		[]string{bwAt("proj"), bwAt("state")},
		SeatbeltCarveOut{Deny: []string{bwAt("state", "gates")}, DenyRead: []string{bwAt("cred")}})
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
	_, bwAt := bwTree(t, []string{"proj/.beads", "proj/.git", "state"}, nil)

	// Handed deepest-first, so a renderer that preserved input order fails.
	argv, prep, err := BwrapArgs(bwAt("proj"),
		[]string{bwAt("state"), bwAt("proj", ".git"), bwAt("proj", ".beads"), bwAt("proj")},
		SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	for _, p := range []string{bwAt("proj"), bwAt("proj", ".beads"), bwAt("proj", ".git"), bwAt("state")} {
		if !strings.Contains(line, "--bind "+p+" "+p) {
			t.Fatalf("no rw bind of %s:\n%s", p, line)
		}
	}
	// The containment pairs, each asserted as a position: this is the claim,
	// and "they are all present" is not.
	for _, pair := range [][2]string{
		{bwAt("proj"), bwAt("proj", ".beads")},
		{bwAt("proj"), bwAt("proj", ".git")},
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
	_, bwAt := bwTree(t, []string{"proj"}, nil)
	missing := bwAt("proj", ".beads")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), missing}, SeatbeltCarveOut{})
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
	_, bwAt := bwTree(t, []string{"proj"}, []string{"claude.json"})
	file := bwAt("claude.json")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), file}, SeatbeltCarveOut{})
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
	_, bwAt := bwTree(t, []string{"proj/agents", "state/gates"}, nil)

	argv, prep, err := BwrapArgs(bwAt("proj"),
		[]string{bwAt("proj"), bwAt("state")},
		SeatbeltCarveOut{Deny: []string{bwAt("state", "gates"), bwAt("proj", "agents")}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	for _, d := range []string{bwAt("proj", "agents"), bwAt("state", "gates")} {
		if !strings.Contains(line, "--ro-bind "+d+" "+d) {
			t.Fatalf("no read-only bind of %s:\n%s", d, line)
		}
	}
	// Every write grant is mounted before every deny: the deny is the later
	// statement at this mechanism exactly as it is at the other.
	lastBind := strings.LastIndex(line, "--bind "+bwAt("state")+" ")
	if firstDeny := strings.Index(line, "--ro-bind "+bwAt("proj", "agents")+" "); firstDeny < lastBind {
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
	_, bwAt := bwTree(t, []string{"proj", "state"}, nil)
	deny := bwAt("state", "gates")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), bwAt("state")},
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
	_, bwAt := bwTree(t, []string{"proj"}, nil)
	deny := bwAt("elsewhere", "promoted")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj")},
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
	_, bwAt := bwTree(t, []string{"proj"}, nil)
	grant, deny := bwAt("state"), bwAt("state", "gates")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), grant},
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

// A deny that is not there, is inside a grant, and whose PRODUCER means it as
// a FILE: neither created nor mounted, where every other missing-deny-inside-a-
// grant above is `mkdir -p`'d and mounted.
//
// This is ranger-base-ppnds' half of that verify's finding 1, and the arm the
// mkdir escaped through: `mkdir -p` was this loop's answer for every absent
// deny, and several Deny entries are file paths that are absent on every
// healthy repo — `<gitdir>/config.lock`, `config.worktree`,
// `config.worktree.lock`. The step runs on the host, outside the namespace,
// in the SHARED repo, and a directory at `config.lock` makes `git config`
// die with `could not lock config file` for every reader of that repo while
// commit and status go on working, so it is found late and nowhere near
// here. A session cannot clear it either: the path is behind the wall it is
// denied.
//
// Both halves in one table, because each alone is satisfied by a renderer
// that is wrong in the other direction: a renderer that mkdirs everything
// passes the directory row, and one that renders nothing for any missing deny
// passes the file row (and would quietly undo
// TestBwrapArgsPrepsAMissingDenyInsideAGrant's whole subject).
func TestBwrapArgsRendersNothingForAMissingFileShapedDeny(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		fileKind  bool
		wantPrep  []string
		wantMount bool
	}{
		// The control, and it is the same path in the same grant: what
		// decides is the producer's statement about the KIND and nothing
		// else about the render.
		{"a directory-shaped deny is made and mounted", false, []string{"mkdir -p %s"}, true},
		{"a file-shaped deny is neither", true, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, bwAt := bwTree(t, []string{"proj", "gitdir"}, nil)
			deny := bwAt("gitdir", "config.lock")
			carve := SeatbeltCarveOut{Deny: []string{deny}}
			if tc.fileKind {
				carve.DenyFile = []string{deny}
			}

			argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), bwAt("gitdir")}, carve)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, w := range tc.wantPrep {
				want = append(want, strings.ReplaceAll(w, "%s", deny))
			}
			if got := bwPrep(prep); !reflect.DeepEqual(got, want) {
				t.Fatalf("prep = %v, want %v", got, want)
			}
			if mounted := strings.Contains(bwJoin(argv), deny); mounted != tc.wantMount {
				t.Fatalf("mounted = %v, want %v:\n%s", mounted, tc.wantMount, bwJoin(argv))
			}
			// The render touches nothing either way — the prep list is a
			// REQUEST to the launcher — so this is what says the file row
			// leaves no empty file behind for `git config` to trip over, and
			// that the directory row's mkdir is still the launcher's to make.
			if _, err := os.Stat(deny); !os.IsNotExist(err) {
				t.Fatalf("the render itself created %s (%v)", deny, err)
			}
		})
	}
}

// FileShaped is a statement about a path in Deny and about no other list: an
// entry that is not in Deny denies nothing, and marking one changes nothing
// about the grants, the Keep files or the read masks. Asserted because the
// field is a second list beside the one that carries the mount order, which
// is exactly the shape a later reader could start treating as a list of its
// own.
func TestBwrapArgsFileShapedOnlySpeaksForTheDenyList(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj"}, nil)
	grant, keep, mask := bwAt("proj", "state"), bwAt("proj", "shell.log"), bwAt("proj", "cred.json")
	if err := os.WriteFile(mask, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	carve := SeatbeltCarveOut{Keep: []string{keep}, DenyRead: []string{mask}}
	marked := carve
	marked.DenyFile = []string{grant, keep, mask}

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	mArgv, mPrep, mErr := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), grant}, marked)
	if mErr != nil {
		t.Fatal(mErr)
	}
	if bwJoin(argv) != bwJoin(mArgv) || !reflect.DeepEqual(bwPrep(prep), bwPrep(mPrep)) {
		t.Fatalf("marking a grant, a Keep file and a read mask file-shaped moved the render:\n argv  %s\n       %s\n prep  %v\n       %v",
			bwJoin(argv), bwJoin(mArgv), bwPrep(prep), bwPrep(mPrep))
	}
	// And the grant's own missing-path arm is untouched: shape 1 still mkdirs
	// what it cannot stat, which is the residual bwrap.go names (D19 and D20
	// remove its live file-shaped members, and a `state_dir:` declaration
	// cannot say the kind the way a Deny producer can).
	if got, want := bwPrep(prep), []string{"mkdir -p " + grant, "touch " + keep}; !reflect.DeepEqual(got, want) {
		t.Fatalf("prep = %v, want %v", got, want)
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
	_, bwAt := bwTree(t, []string{"proj/agents", "state/gates"}, nil)
	writable := []string{bwAt("proj"), bwAt("state")}
	carve := SeatbeltCarveOut{Deny: []string{bwAt("proj", "agents"), bwAt("state", "gates")}}

	bare, barePrep, err := BwrapArgs(bwAt("proj"), writable, carve)
	if err != nil {
		t.Fatal(err)
	}
	sealed := carve
	sealed.Seal = []string{bwAt("proj"), bwAt("state"), bwAt("elsewhere")}
	withSeal, sealPrep, err := BwrapArgs(bwAt("proj"), writable, sealed)
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
	_, bwAt := bwTree(t, []string{"proj", "state/gates/dev"}, []string{"state/gates/dev/refusals.log"})
	gates := bwAt("state", "gates")
	have, missing := bwAt("state", "gates", "dev", "refusals.log"), bwAt("state", "gates", "dev", "shell.log")

	argv, prep, err := BwrapArgs(bwAt("proj"),
		[]string{bwAt("proj"), bwAt("state"), bwAt("state", "gates", "dev")},
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
	_, bwAt := bwTree(t, []string{"proj", "state/gates/refusals.log"}, nil)
	keep := bwAt("state", "gates", "refusals.log")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), bwAt("state")},
		SeatbeltCarveOut{Deny: []string{bwAt("state", "gates")}, Keep: []string{keep}})
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
	_, bwAt := bwTree(t, []string{"proj"}, []string{"codex/auth.json"})
	have, missing := bwAt("codex", "auth.json"), bwAt("grok", "auth.json")

	argv, prep, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj")},
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
	root, bwAt := bwTree(t,
		[]string{"proj/.beads", "proj/agents", "state"},
		[]string{"codex/auth.json"})
	cwd := bwAt("proj")

	writable := []string{bwAt("proj"), bwAt("proj", ".beads"), bwAt("state"), bwAt("fresh")}
	carve := SeatbeltCarveOut{
		Deny:     []string{bwAt("proj", "agents"), bwAt("state", "gates"), bwAt("elsewhere", "promoted")},
		Seal:     []string{bwAt("proj")},
		Keep:     []string{bwAt("state", "gates", "refusals.log")},
		DenyRead: []string{bwAt("codex", "auth.json"), bwAt("grok", "auth.json")},
	}

	argv, prep, err := BwrapArgs(cwd, writable, carve)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--die-with-parent",
		"--ro-bind", "/", "/",
		"--dev-bind", "/dev", "/dev",
		"--unshare-user",
		"--chdir", cwd,
		// ranger-base-ludpp: the agent and bus addresses, always
		"--unsetenv", "DBUS_SESSION_BUS_ADDRESS",
		"--unsetenv", "SSH_AUTH_SOCK",
		"--unsetenv", "GPG_AGENT_INFO",
		// shape 1, shallowest first
		"--bind", bwAt("fresh"), bwAt("fresh"),
		"--bind", bwAt("proj"), bwAt("proj"),
		"--bind", bwAt("proj", ".beads"), bwAt("proj", ".beads"),
		"--bind", bwAt("state"), bwAt("state"),
		// shape 2 — elsewhere/promoted is in no grant, so no mount
		"--ro-bind", bwAt("proj", "agents"), bwAt("proj", "agents"),
		"--ro-bind", bwAt("state", "gates"), bwAt("state", "gates"),
		// Keep, back over the deny above it
		"--bind", bwAt("state", "gates", "refusals.log"), bwAt("state", "gates", "refusals.log"),
		// shape 5 — grok's credential is not on this box, so no mask
		"--ro-bind", "/dev/null", bwAt("codex", "auth.json"),
	}
	if bwJoin(argv) != bwJoin(want) {
		t.Fatalf("argv =\n  %s\nwant\n  %s", strings.ReplaceAll(bwJoin(argv), root, "R"), strings.ReplaceAll(bwJoin(want), root, "R"))
	}
	wantPrep := []string{
		"mkdir -p " + bwAt("fresh"),
		"mkdir -p " + bwAt("state", "gates"),
		"touch " + bwAt("state", "gates", "refusals.log"),
	}
	if got := bwPrep(prep); strings.Join(got, "\n") != strings.Join(wantPrep, "\n") {
		t.Fatalf("prep =\n  %v\nwant\n  %v", got, wantPrep)
	}
	// Nothing was written: the prep list is a REQUEST to the launcher, and a
	// renderer that acted on it could not be called twice or printed by
	// `posse gates` without changing the box.
	for _, p := range []string{bwAt("fresh"), bwAt("state", "gates"), bwAt("elsewhere")} {
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
	_, bwAt := bwTree(t, []string{"proj/.beads", "state/gates"}, nil)
	carve := SeatbeltCarveOut{Deny: []string{bwAt("state", "gates"), bwAt("state", "gates")}}

	forward, _, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), bwAt("proj", ".beads"), bwAt("state"), bwAt("proj"), ""}, carve)
	if err != nil {
		t.Fatal(err)
	}
	reversed, _, err := BwrapArgs(bwAt("proj"), []string{"", bwAt("state"), bwAt("proj"), bwAt("proj", ".beads")}, carve)
	if err != nil {
		t.Fatal(err)
	}
	if bwJoin(forward) != bwJoin(reversed) {
		t.Fatalf("input order changed the wall:\n  %v\n  %v", forward, reversed)
	}
	if n := strings.Count(bwJoin(forward), "--bind "+bwAt("proj")+" "+bwAt("proj")); n != 1 {
		t.Fatalf("the duplicated grant rendered %d mounts: %v", n, forward)
	}
	if n := strings.Count(bwJoin(forward), "--ro-bind "+bwAt("state", "gates")); n != 1 {
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
	_, bwAt := bwTree(t, []string{"proj", "closed/inner"}, nil)
	closed, inner := bwAt("closed"), bwAt("closed", "inner")
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
		{"write grant", SeatbeltCarveOut{}, []string{bwAt("proj"), inner}},
		{"deny", SeatbeltCarveOut{Deny: []string{inner}}, []string{bwAt("proj")}},
		{"keep", SeatbeltCarveOut{Keep: []string{inner}}, []string{bwAt("proj")}},
		{"read deny", SeatbeltCarveOut{DenyRead: []string{inner}}, []string{bwAt("proj")}},
	} {
		argv, prep, err := BwrapArgs(bwAt("proj"), tc.w, tc.carve)
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
//
// THE CWD IS A REAL LINKED WORKTREE, and for two beads it was a fixture
// DIRECTORY that was not a git repo (ranger-base-ppnds, finding 3 of the
// verify at ranger-base-vnp01 — and it is why finding 1 escaped). With a
// non-repo cwd `gitPath` fails, `LinkedGitDirs` answers nothing, and
// sessionGitConfigFiles, sessionWorktreeIdentityFiles and sessionGitGrants
// all return nil: not one file-shaped deny and not one git grant reached the
// render in ANY arm of this file, which is most of what the one
// producer-driven arm exists to carry.
//
// `wgNewFixture` is that worktree (seatbeltworktreegit_qa_test.go) and
// `bgSwap` is the shipped path through D20's substitution
// (bwrapgitrefs_qa_test.go), so this arm hand-assembles neither. It is what
// D18 will call: SeatbeltWritable, then BwrapWritable, then BwrapArgs with
// the carve-out and the keep file. D20's own arms drive the same fixture with
// an EMPTY carve-out — what is only here is a REAL one, and the KIND of
// every prep step it produces.
//
// Not t.Parallel: the fixture sets HOME, which is also what keeps the home
// outside TMPDIR so the blanket temp grant does not sweep the constitution
// denies in and make this reading say more than it measured.
func TestBwrapArgsRendersTheTiersOwnLists(t *testing.T) {
	f := wgNewFixture(t)
	writable, keep := bgSwap(t, f, f.tree)
	carve := f.a.SeatbeltCarveOut(f.ag, f.tree, f.gates, writable)
	argv, prep, err := BwrapArgs(f.tree, writable, carve, keep...)
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
	grant := strings.Index(line, "--bind "+absResolve(f.gates)+" ")
	deny := strings.Index(line, "--ro-bind "+absResolve(filepath.Join(f.a.StateDir, "gates"))+" ")
	kept := strings.Index(line, "--bind "+absResolve(filepath.Join(f.gates, "refusals.log"))+" ")
	if grant < 0 || deny < 0 || kept < 0 {
		t.Fatalf("grant/deny/keep at %d/%d/%d — one of the three lists rendered nothing:\n%s", grant, deny, kept, line)
	}
	if !(grant < deny && deny < kept) {
		t.Fatalf("the gates dir's grant, deny and Keep are mounted in order %d/%d/%d, not grant→deny→Keep:\n%s", grant, deny, kept, line)
	}
	// Every prep step names a path inside a grant: the launcher writes these
	// on the host in the seat's name, so a step outside the wall's own
	// footprint is a write posse has no business making.
	//
	// AND THE KIND OF EACH STEP FITS WHAT ITS PRODUCER MEANS THE PATH TO BE,
	// which is the half that was missing: `writeGranted` asks whether a step
	// is inside a grant and never whether a `mkdir -p` belongs at that path,
	// so it stayed green over every one of finding 1's five directories. The
	// carve-out is the only thing that can answer — an absent path's shape
	// cannot be stat'd — and a step whose kind contradicts it is the defect.
	//
	// The two producers of a TOUCH are named, not counted: the carve-out's
	// Keep literals (the session's own audit logs) and D20's ref keep file.
	// A count would go green over a touch at a third path nobody declared.
	touchable := append(append([]string(nil), carve.Keep...), keep...)
	for _, s := range prep {
		if !writeGranted(writable, s.Path) {
			t.Fatalf("prep step %q is outside every write grant", s)
		}
		if s.Kind == PrepMkdirAll && carve.FileShaped(s.Path) {
			t.Fatalf("prep asks the launcher for %q at a path its producer means as a FILE — on the host, outside the namespace, in the SHARED repo, in the seat's name, and a session cannot clear it from behind the wall that denies it (ranger-base-ppnds finding 1; a directory at <gitdir>/config.lock kills `git config` for every reader of that repo)", s)
		}
		if s.Kind == PrepTouch && !bwqaIn(touchable, s.Path) {
			t.Fatalf("prep asks the launcher for %q at a path no producer declared a file — the carve-out's Keep literals and D20's ref keep file are the only touches this render has a producer for", s)
		}
	}
	// THE POSITIVE WITNESS, without which both assertions above are satisfied
	// by a render that was handed no file-shaped deny at all — which is
	// exactly the state this arm was in for two beads. A dispatched session's
	// per-worktree git dir is granted WHOLE (sessionGitGrants), and
	// `config.worktree`/`config.worktree.lock` inside it are file-shaped
	// denies that are absent on every healthy repo (measured absent through a
	// whole session's life, seatbeltgitidentity_qa_test.go), so the arm under
	// test — a missing deny, inside a grant, file-shaped — really runs here.
	var witness []string
	for _, d := range carve.DenyFile {
		if !writeGranted(writable, d) {
			continue
		}
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			continue
		}
		if strings.Contains(line, " "+d+" ") {
			t.Fatalf("a file-shaped deny that is not there was mounted — bwrap cannot bind a source that does not exist, so this argv is refused at the host: %s", d)
		}
		// Under the session's GIT dirs specifically. The constitution's two
		// files are file-shaped denies too, and they would go on satisfying a
		// bare count the day the git half stopped reaching the render at all,
		// which is the exact failure this arm is about.
		for _, g := range LinkedGitDirs(f.tree) {
			if underDir(g, d) {
				witness = append(witness, d)
			}
		}
	}
	if len(witness) == 0 {
		t.Fatalf("no file-shaped deny under this session's git dirs is both absent and inside a grant, so the arm this test exists for did not run — the state a non-repo cwd leaves it in.\n DenyFile %v\n writable %v", carve.DenyFile, writable)
	}
}

// bwqaIn is membership, for the one assertion that has to tell a declared
// touch — a Keep literal, or D20's ref keep file — from a prep step no
// producer asked for.
func bwqaIn(list []string, p string) bool {
	for _, q := range list {
		if q == p {
			return true
		}
	}
	return false
}
