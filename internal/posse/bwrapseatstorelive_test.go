//go:build !posse_arm2 && !posse_arm3

package posse

// Live pins for ranger-base-21i98 (ADR 0099 D1, D3-D5): a committing seat's
// `<common>/objects` is its OWN store, bound over that name by the bwrap
// render, with the shared store read-only at a side path. Run against a real
// bwrap on a real linked worktree, rendered by the shipped producers — the
// writable set from SeatbeltWritable through BwrapWritable, the store binds
// from seatStoreBinds, the store itself from PrepareSessionHead's walled arm —
// so a drift between the design and the render reds here.
//
//	go test ./internal/posse -run TestLiveBwrapSeatStore -v
//
// Gated like its neighbours in bwrapgitrefslive_test.go (bgLiveSkip): Linux,
// bwrap and git, and not from inside a seat (ADR 0096 D1).
//
// EVERY REFUSAL IS READ BACK FROM OUTSIDE. A wall row that trusts the exit
// code measures the shell; the shared store's file, its mode and what the
// operator's `cat-file` serves are what the hole was about.

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bsLive is a walled session tree: a real posse session tree (so the landing
// half runs MergeSessionWork), prepared by the walled launch's own call, and
// the argv the render gives it — with and without the store binds.
type bsLive struct {
	a          *App
	repo       string
	tr         *SessionTree
	own        string
	common     string
	store      string
	base       string
	set        []string
	keep       []string
	argv       []string // the shipped render: the store binds in place
	argvBefore []string // the render this bead replaced: no store binds
}

func bsLiveFixture(t *testing.T) bsLive {
	t.Helper()
	bgLiveSkip(t)
	f := bsLive{a: wtApp(t), repo: wtRepo(t)}
	tr, err := f.a.EnsureSessionTree(f.repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	f.tr = tr
	if err := PrepareSessionHead(tr, true, false, io.Discard); err != nil {
		t.Fatalf("the walled prepare refused the fixture's tree: %v", err)
	}
	dirs := LinkedGitDirs(tr.Path)
	if len(dirs) != 2 {
		t.Fatalf("no linked worktree: %v", dirs)
	}
	f.own, f.common = absResolve(dirs[0]), absResolve(dirs[1])
	l, ok := seatStoreFor(tr.Path)
	if !ok {
		t.Fatalf("no seat store layout for the linked worktree %s", tr.Path)
	}
	f.store, f.base = l.Store, l.Base
	if b, err := os.ReadFile(filepath.Join(f.store, "info", "alternates")); err != nil || strings.TrimSpace(string(b)) != f.base {
		t.Fatalf("the walled prepare did not leave a store whose alternates name the side path %s: %q %v", f.base, b, err)
	}
	ag := &AgentFile{Name: "bs-live"}
	f.set, f.keep = BwrapWritable(ag, bgLiveSessionGrants(t, f.repo, tr.Path, ag), tr.Path)
	if !wantsPath(f.set, filepath.Join(f.common, "objects")) {
		t.Fatalf("CONTROL: the writable set does not grant <common>/objects, so the store bind has nothing to cover: %v", f.set)
	}
	binds, err := seatStoreBinds(ag, bsLiveHome(t), tr.Path)
	if err != nil {
		t.Fatalf("seatStoreBinds refused the fixture's tree: %v", err)
	}
	f.argv = f.render(t, f.set, binds)
	if len(binds) != 2 {
		t.Fatal("CONTROL: seatStoreBinds rendered no store for a committing PID in a linked worktree")
	}
	f.argvBefore = f.render(t, f.set, nil)
	return f
}

// bsLiveHome is the home the render composes this seat's wall dir from (ADR
// 0100 D1), which is where the store now lives. The binary's own temp HOME,
// the one every fixture here runs under.
func bsLiveHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("no home for the fixture, so no seat wall dir and no store: %v", err)
	}
	return home
}

// binds is the fixture's store binds, re-rendered.
func (f bsLive) binds(t *testing.T) []PrivateBind {
	t.Helper()
	b, err := seatStoreBinds(&AgentFile{Name: "bs-live"}, bsLiveHome(t), f.tr.Path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// render is BwrapArgs over the set with these private binds, plus the
// launcher's half of the prep, run in the list's order.
func (f bsLive) render(t *testing.T, set []string, private []PrivateBind) []string {
	t.Helper()
	argv, prep, err := BwrapArgs(f.tr.Path, set, SeatbeltCarveOut{Private: private}, f.keep...)
	if err != nil {
		t.Fatalf("BwrapArgs refused the seat store render: %v", err)
	}
	// runBwrapPrep and not a copy of its switch (ranger-base-ua625). This
	// loop was written before D18 made the launcher's half production code,
	// and it knew two step kinds of the four: the render grew a PrepSeed
	// here — the anchor inside an empty private-bind mount point — and both
	// copies fell into their own `default` and refused a step the launcher
	// runs without comment. A reduced launcher measures a wall the launcher
	// does not put up, so the test plays launcher by CALLING it.
	if err := runBwrapPrep(prep); err != nil {
		t.Fatalf("prep: %v", err)
	}
	return argv
}

func (f bsLive) in(t *testing.T, argv []string, stdin string, arg ...string) (bool, string) {
	t.Helper()
	cmd := exec.Command("bwrap", append(append([]string{}, argv...), arg...)...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}

func (f bsLive) git(arg ...string) []string {
	return append([]string{"git", "-c", "user.name=p", "-c", "user.email=p@example.com", "-C", f.tr.Path}, arg...)
}

// sharedObject writes an object into the SHARED store from outside, as
// history already holds it, and returns its name and its file.
func (f bsLive) sharedObject(t *testing.T, body string) (string, string) {
	t.Helper()
	cmd := exec.Command("git", "-C", f.repo, "hash-object", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(body)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(out))
	p := filepath.Join(f.common, "objects", sha[:2], sha[2:])
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("CONTROL: %s is not a loose object in the shared store: %v", sha, err)
	}
	return sha, p
}

// Pin (1), the design's live row and the sign-off's (1): under the shipped render a
// committing seat creates new objects and commits, and every way of changing
// an EXISTING shared object's file — unlink, chmod, rename over it, rewrite
// it in place — is refused, by its real name and by the side path. The same
// holds when another grant covers `<common>` WHOLE (a store of record inside
// the same repo), which is what says the store's bind is the deepest mount —
// and that shape needs no grant on `<common>/worktrees`, because the render
// binds that component in place itself (bwrapGrantMounts, ranger-base-dot6w).
// bsWideGrantPinsTheLooseComponent is the arm that says the pin is there and
// is load-bearing, rather than a comment claiming it.
//
// The control is the render this bead replaced, against a second object: the
// same unlink succeeds there, so the arms measure the binds and not the file
// mode (a loose object is 0444, which stops an in-place write and nothing
// else).
//
// MUTATION RUN: dropping the store bind from seatStoreBinds reds every arm
// (the seat writes the shared store again); dropping the side bind reds the
// read-through arm and the commit (the alternates name an empty dir).
func TestLiveBwrapSeatStoreRefusesRewritingASharedObject(t *testing.T) {
	t.Parallel()
	f := bsLiveFixture(t)
	honest := "the honest history\n"
	sha, file := f.sharedObject(t, honest)
	ctlSha, ctlFile := f.sharedObject(t, "the control's object\n")

	if ok, out := f.in(t, f.argvBefore, "", "rm", "-f", ctlFile); !ok {
		t.Fatalf("CONTROL: the pre-store render refused unlinking %s, so the arms below cannot be told from the file's own mode:\n%s", ctlSha[:12], out)
	}
	if _, err := os.Stat(ctlFile); err == nil {
		t.Fatal("CONTROL: the pre-store render's unlink exited 0 and the shared object is still there")
	}

	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	fi0, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	side := filepath.Join(f.base, sha[:2], sha[2:])

	// THE WIDE SHAPE'S GRANTS: the writable set plus `<common>`, and nothing
	// else (ranger-base-49rxe). A grant covering `<common>` whole puts
	// `<common>/worktrees` strictly inside it with no mount between, and the
	// seat's own per-worktree git dir `<common>/worktrees/<name>` is a grant
	// under THAT, with the store's side bind one component deeper again — so
	// under ADR 0100 this set renders only with the component between bound
	// in place as well. The RENDER does that: bwrapGrantMounts binds every
	// directory strictly between a grant and the grant above it, which is
	// the fix that ADR prescribes for this shape and never an exemption, so
	// the set the production producer would hand BwrapArgs is the one this
	// arm renders.
	//
	// THAT PIN IS READ BY A TEST OF ITS OWN and no longer from here
	// (ranger-base-bx9wj): bsWideGrantPinsTheLooseComponent needs a home
	// outside every blanket temp grant to measure anything at all, which is
	// a HOME this test cannot set — it is `t.Parallel`, and the home is the
	// whole binary's. Called from here it returned at ADR 0100's stand-down
	// under every uncaged run and measured only in ADR 0097 D1's venue.
	// TestLiveBwrapSeatStoreWideGrantPinsTheLooseComponent is that test.
	wide := append(append([]string{}, f.set...), f.common)

	for _, shape := range []struct {
		name string
		argv []string
	}{
		{"as rendered", f.argv},
		{"with <common> granted whole", f.render(t, wide, f.binds(t))},
	} {
		for _, p := range []string{file, side} {
			for _, c := range []struct {
				name string
				arg  []string
			}{
				{"unlink", []string{"rm", "--", p}},
				{"chmod", []string{"chmod", "0644", p}},
				{"rename over", []string{"sh", "-c", "printf evil > " + p + ".new && mv -f " + p + ".new " + p}},
				{"rewrite in place", []string{"sh", "-c", "chmod u+w " + p + " && printf evil > " + p}},
			} {
				if ok, out := f.in(t, shape.argv, "", c.arg...); ok {
					t.Errorf("%s: a committing seat could %s %s:\n%s", shape.name, c.name, p, out)
				}
			}
		}
		if ok, out := f.in(t, shape.argv, "", f.git("cat-file", "-p", sha)...); !ok || out+"\n" != honest {
			t.Errorf("%s: the seat cannot read history through its store's alternates (ok=%v): %q", shape.name, ok, out)
		}
		ok, out := f.in(t, shape.argv, "a new object "+shape.name+"\n", f.git("hash-object", "-w", "--stdin")...)
		if !ok {
			t.Fatalf("%s: a committing seat cannot write a NEW object:\n%s", shape.name, out)
		}
		if _, err := os.Stat(filepath.Join(f.store, out[:2], out[2:])); err != nil {
			t.Errorf("%s: the new object %s is not in the seat's own store: %v", shape.name, out[:12], err)
		}
		if _, err := os.Stat(filepath.Join(f.common, "objects", out[:2], out[2:])); err == nil {
			t.Errorf("%s: the new object %s was written into the SHARED store", shape.name, out[:12])
		}
		if ok, out := f.in(t, shape.argv, "", f.git("commit", "-q", "--allow-empty", "-m", "21i98 "+shape.name)...); !ok {
			t.Errorf("%s: a committing seat cannot commit under its store:\n%s", shape.name, out)
		}
	}

	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("the shared object %s is gone: %v", sha[:12], err)
	}
	if string(after) != string(before) {
		t.Errorf("the shared object %s's file changed under the wall", sha[:12])
	}
	if fi, err := os.Stat(file); err != nil || fi.Mode() != fi0.Mode() {
		t.Errorf("the shared object %s's mode changed under the wall: %v -> %v (%v)", sha[:12], fi0.Mode(), fi.Mode(), err)
	}
	if got := mustGit(t, f.repo, "cat-file", "-p", sha); got+"\n" != honest {
		t.Errorf("the operator's cat-file serves %q for %s", got, sha[:12])
	}
}

// bsWideGrantPinsTheLooseComponent is why the wide shape above renders at
// all: the component between — `<common>/worktrees` — is bound IN PLACE by
// the render itself, and that bind is load-bearing. Cut it out of the
// finished argv and ADR 0100's refusal names that component, so the mount is
// the ADR's prescribed pin and not a line a later reader may drop.
//
// IT ASSERTS THE PIN AND NOT THE GRANT, which is what the readings forced
// (ranger-base-49rxe). As ranger-base-4gyib filed it, this arm rendered the
// same set WITHOUT a `<common>/worktrees` grant and asserted the render
// REFUSED it — true when it landed, and false one landing later:
// ranger-base-dot6w generalized D2(b)'s remedy to every directory strictly
// between a grant and the grant above it (bwrapGrantMounts), and this
// fixture's shape is exactly that — `<common>` granted, the seat's own git
// dir granted two components inside it — so the component is pinned by the
// render and the set 4gyib expected to be refused is renderable BY DESIGN.
// Two halves each green on its own base: 4gyib's arm never saw dot6w's list
// and dot6w's branch was cut one landing before 4gyib's. The claim worth
// holding survives the merge, one step further in: the pin exists, and
// without it the render is refused.
//
// THE ATTACK IT MEASURES IS THIS FIXTURE'S OWN SHAPE. A seat granted
// `<common>` whole may rename `<common>/worktrees`; its per-worktree git dir
// is bound at `<common>/worktrees/<name>` IN PLACE, read-write, the store's
// side bind lands at `<common>/worktrees/<name>/seat-objects-base`, and bwrap
// re-resolves both strings at mount(2) — so a leftover of this seat plants a
// link at the renamed name and the next launch mounts whatever it points at,
// with no race to win. The in-place mount at the component is what refuses
// that rename from inside the namespace (EBUSY).
//
// IT READS THE REFUSAL AND NOT ONE ARM OF IT, which is 4gyib's reading and
// still holds. Both of ADR 0100's arms name this same component: D2(b)'s
// source arm (bwrapRefuseUnpinnedSource, ranger-base-dw2du) over the
// in-place grant, and D4(a)'s destination arm (bwrapRefuseUnpinnedDst,
// ranger-base-gn050) over the side bind's destination. MEASURED 2026-11-21:
// putting dw2du's `m.Src == m.Dst` skip back leaves the cut argv refused by
// D4(a) with the same loose component, so an arm that named one decision
// would be measuring neither. What reds this pin as it stands is dropping
// shape 1's pin arm from bwrapGrantMounts.
//
// ITS CALLER OWNS THE HOME IT RUNS UNDER, and that is ranger-base-bx9wj
// (from ranger-base-4gyib, whose whole red this arm was). Both of ADR 0100's
// refusals stand down when this uid's home lies under /tmp (bwrapWallRoot) —
// right for a box where /tmp is granted unconditionally, and true of the
// binary's own temp HOME, which is a `MkdirTemp("", ...)` under TMPDIR. So
// under an operator's uncaged `go test` this arm used to RETURN at that
// stand-down and measure nothing, while in ADR 0097 D1's venue — TMPDIR under
// the posse state dir, so a home that is not under /tmp — it measured and
// reds. A green control over a rule that was not running is what let 4gyib's
// red sit on main for four landings.
//
// So the stand-down is a CONTROL FAILURE here now and never a quiet return:
// the one caller sets HOME to a fixture home under wallUngrantedDir first
// (bsUngrantedHome), where the two refusals are live in both venues, and an
// empty wall root under such a home means the arm is about to measure
// nothing again.
func bsWideGrantPinsTheLooseComponent(t *testing.T, f bsLive, wide []string, wt string) {
	t.Helper()
	if bwrapWallRoot() == "" {
		t.Fatalf("CONTROL: the wall root is empty for the home %s, so both of ADR 0100's refusals stand down and the arms below measure nothing — which is exactly how ranger-base-4gyib's red reached main. The caller owes this arm a home outside every blanket temp grant (bsUngrantedHome)", bsLiveHome(t))
	}
	argv, _, err := BwrapArgs(f.tr.Path, wide, SeatbeltCarveOut{Private: f.binds(t)}, f.keep...)
	if err != nil {
		t.Fatalf("a render granting <common> whole is refused (%v) — that is a seat in a linked worktree that does not launch at all, and the fix ADR 0100 prescribes is the in-place bind the render makes for itself and never an exemption (ranger-base-dot6w)", err)
	}
	if !containsString(bwrapMountDsts(bwrapMounts(argv)), wt) {
		t.Fatalf("the render makes no mount at %s, which lies strictly inside the <common> grant and holds the seat's own git dir: a leftover of this seat renames it and the next launch binds what the name points at, read-write (ADR 0100 D2(b))\n argv: %q", wt, argv)
	}
	cut := bsArgvWithoutInPlaceBind(argv, wt)
	if len(cut) == len(argv) {
		t.Fatalf("CONTROL: %s is a mount destination of this render by some option other than an in-place --bind, so cutting that bind measures nothing this arm was written for\n argv: %q", wt, argv)
	}
	grants := shallowestFirst(wide)
	cutErr := bwrapRefuseUnpinnedSource(cut, grants)
	if cutErr == nil {
		cutErr = bwrapRefuseUnpinnedDst(cut, grants)
	}
	switch {
	case cutErr == nil:
		t.Errorf("with the in-place bind of %s cut from the argv, neither of ADR 0100's arms refuses the render — so that mount is decoration, and a producer that stops emitting it leaves the seat's own git dir under a renameable component with every pin green (ADR 0100 D4(a)/D2(b))\n argv: %q", wt, cut)
	// The loose component by name AND by its clause, never a bare substring
	// of the message: `<common>/worktrees` is a prefix of both paths the
	// refusal names — the bind source and the destination under it — so a
	// Contains of the path alone reads true for any refusal about this git
	// dir at all, and it did under the mutation above while 4gyib's arm was
	// being written.
	case !strings.Contains(cutErr.Error(), wt+" lies inside a write grant"):
		t.Errorf("the refusal does not name %s as the loose component, which is the only instruction it carries: %v", wt, cutErr)
	}
}

// bsArgvWithoutInPlaceBind is argv with every `--bind p p` triple for this
// one path removed, and nothing else touched — the cut that turns the
// render's own pin back into a loose component so the arm above can read
// what ADR 0100 says about it.
//
// IT CUTS THE IN-PLACE FORM ONLY, by both operands and not by the
// destination alone: `p` is also the destination of nothing else in this
// render, but a cut keyed on one operand would silently take a `--ro-bind
// src p` or a `--tmpfs p` a later step adds, and then the arm above would be
// measuring a render that lost a wall rather than a render that lost a pin.
// Its caller asserts the cut removed something, so a spelling this misses is
// a CONTROL failure and never a green.
func bsArgvWithoutInPlaceBind(argv []string, p string) []string {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--bind" && i+2 < len(argv) && argv[i+1] == p && argv[i+2] == p {
			i += 2
			continue
		}
		out = append(out, argv[i])
	}
	return out
}

// bsUngrantedHome points $HOME at a fixture home outside every blanket temp
// grant and answers it — the whole of ranger-base-bx9wj, and the reason the
// pin below is not simply an arm of the test above.
//
// WHAT IT BUYS. ADR 0100's two refusals — bwrapRefuseUnpinnedSource (D2(b))
// and bwrapRefuseUnpinnedDst (D4(a)) — stand down when bwrapWallRoot() is
// empty, and it is empty when this uid's home lies under /tmp: that box
// grants /tmp unconditionally (seatbeltTempGrants), every private source is
// in the shared directory already, and there is no seat for the wall root to
// protect. Right for production, and true of this binary's own HOME, which
// TestMain makes with `MkdirTemp("", …)` — so the home follows TMPDIR, and
// the two refusals are keyed on it:
//
//   - uncaged, TMPDIR unset: HOME is under /tmp and both refusals are DORMANT
//     for every fixture in this package.
//   - in ADR 0097 D1's venue: TMPDIR is `<state>/landed-check/run-<n>/tmp`,
//     HOME is not under /tmp, and both are LIVE.
//
// A green uncaged control therefore said nothing about this family of rules,
// and a venue red could not be reproduced by the command the red's own advice
// names first (ADR 0097 D6). MEASURED 2026-11-21, same tree and same
// command: TMPDIR unset read `ok 0.13s` where the venue's shape read D2(b)'s
// refusal — ranger-base-4gyib's whole red, which sat on main for four
// landings behind that green (docs/notes.d/ranger-base-4gyib.md).
//
// wallUngrantedDir IS THE EXISTING SPELLING and the pins written FOR these
// rules already use it (wallseatwall_qa_test.go's header,
// wallpinnedsource_qa_test.go's, wallpinneddst_qa_test.go's): a root under
// /var/tmp, which no blanket temp grant covers, and which ADR 0097 D1 binds
// privately into the venue for exactly these 18 pins. So a home under it
// stands the rules UP in both venues, which is what makes the control and the
// venue agree.
//
// PER-TEST AND NOT THE BINARY'S, which is the cost this helper accepts
// rather than hides. Moving the binary's own home is not one line: it is the
// whole process's (herdr_test.go's TestMain, FenceCLIHomes), roughly a
// thousand tests read it, and every fixture keyed on "a t.TempDir() is inside
// a blanket grant" would flip with it (sbRoot's stand-down, 123 tests). A
// t.Setenv is the narrow lever, and its price is the one t.Setenv always
// charges: the caller cannot be a parallel test.
func bsUngrantedHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(wallUngrantedDir(t), "home")
	// `.cache`, because the wall root is `<home>/.cache/posse-wall` and the
	// render's prep makes the wall dir under a `.cache` that is there.
	if err := os.MkdirAll(filepath.Join(home, ".cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	return home
}

// THE ASYMMETRY ITSELF, as a pin, because until ranger-base-bx9wj it was only
// a paragraph: the binary's own HOME decides whether ADR 0100's refusals run
// at all, and it decides it differently in the two places this tree's wall
// pins are read.
//
// TWO CLAIMS, and the second is the one the live pin above rests on:
//
//  1. the binary's home answers bwrapWallRoot by the documented rule and
//     nothing else — empty exactly when the home lies under /tmp — so the
//     dormancy is read off the rule rather than asserted of a box. Uncaged
//     with TMPDIR unset that is TRUE (TestMain's `MkdirTemp("", …)` home is
//     under /tmp, the refusals are dormant); in ADR 0097 D1's venue TMPDIR is
//     under the posse state dir and the same expression is FALSE (they are
//     live). One pin, both venues, no skip.
//  2. bsUngrantedHome's home answers a NON-EMPTY root under either venue's
//     TMPDIR, and lies in none of the write grants every seat gets whatever
//     its PID says (seatbeltTempGrants). That is what makes the control and
//     the venue measure the same rule, which is ADR 0097 D6's own premise for
//     this family.
//
// TMPDIR IS SET HERE THOUGH bwrapWallRoot READS ONLY $HOME, deliberately: the
// defect was that the rule's real input was TMPDIR-shaped, through the home
// derived from it. A later bead that makes the wall root read a temp root
// directly reds this row instead of going dormant in silence.
func TestQABwrapSeatStoreFixtureHomeStandsADR0100sRulesUpInBothVenues(t *testing.T) {
	// No t.Parallel: bsUngrantedHome sets HOME, and this sets TMPDIR.
	binHome, err := os.UserHomeDir()
	if err != nil || binHome == "" {
		t.Fatalf("no home for this binary, so neither claim below is measured: %v", err)
	}
	dormant := underDir(bwrapTmpMount, binHome)
	if (bwrapWallRoot() == "") != dormant {
		t.Errorf("bwrapWallRoot() = %q for this binary's home %s, which is%s under %s — ADR 0100's two refusals stand down exactly when the home is, and a third answer means the dormancy this bead is about is keyed on something else now", bwrapWallRoot(), binHome, map[bool]string{true: "", false: " not"}[dormant], bwrapTmpMount)
	}

	home := bsUngrantedHome(t)
	for _, tmpdir := range []string{"", filepath.Join(home, "tv")} {
		t.Setenv("TMPDIR", tmpdir)
		if tmpdir == "" {
			os.Unsetenv("TMPDIR")
		}
		if root := bwrapWallRoot(); root == "" {
			t.Errorf("TMPDIR=%q: the wall root is empty for the fixture home %s, so ADR 0100's refusals are dormant under this venue's shape and a pin written over them measures nothing here", tmpdir, home)
		}
		for _, g := range seatbeltTempGrants(runtime.GOOS, os.Getenv("TMPDIR")) {
			if underDir(g, home) {
				t.Errorf("TMPDIR=%q: the fixture home %s lies under %s, which every seat is granted whatever its PID says — then every component of every private source is inside a grant and nothing can be pinned", tmpdir, home, g)
			}
		}
	}
}

// ADR 0100 D2(b)/D4(a), live and in BOTH venues: the wide shape's loose
// component is pinned by the render, that pin is load-bearing, and the mount
// is what refuses the rename from inside the seat.
//
// THE POINT OF THIS TEST EXISTING AT ALL is the home, and it is
// ranger-base-bx9wj's deliverable: bsUngrantedHome says why. The arm this
// test wraps used to run inside TestLiveBwrapSeatStoreRefusesRewritingAShared
// Object, which is `t.Parallel` and so cannot set HOME — so it returned at
// ADR 0100's stand-down under every uncaged run and measured only in the
// venue. Here the home is under wallUngrantedDir, both refusals are live
// whatever TMPDIR says, and the arm's own stand-down guard is a CONTROL
// failure rather than a quiet return.
//
// THE LIVE HALF IS THE MECHANISM BEHIND THE REFUSAL, and nothing measured it
// before: ADR 0100's answer to a loose component is that a mount on it cannot
// be renamed by a namespace holding that mount (its P4, EBUSY). So the seat
// rendered from the wide set is handed `mv <common>/worktrees …` and must be
// refused, and the CONTROL is the same argv with that one in-place bind cut —
// the render BwrapArgs declines to produce — handed to bwrap directly, where
// the rename lands. Without the control the refusal above could be the grant,
// the kernel or the fixture rather than the mount.
func TestLiveBwrapSeatStoreWideGrantPinsTheLooseComponent(t *testing.T) {
	// No t.Parallel: bsUngrantedHome sets HOME, which the wall root, the
	// seat's wall dir and so this seat's whole store are composed from.
	bgLiveSkip(t) // ADR 0096 D3 first, so this is a counted venue member
	home := bsUngrantedHome(t)
	root := bwrapWallRoot()
	if root == "" {
		t.Fatalf("CONTROL: the wall root is empty for the fixture home %s, so ADR 0100's two refusals stand down here too and this test measures what the uncaged control always measured, which is nothing. bsUngrantedHome owes this test a home outside every blanket temp grant — under /tmp or $TMPDIR it is the dormancy ranger-base-bx9wj was filed for", home)
	}
	if want := SeatWallRoot(home); root != want {
		t.Fatalf("CONTROL: bwrapWallRoot() = %s for the fixture home %s, whose wall root is %s — the root a grant is judged against and the root the sources are composed under must be one directory (ADR 0100 D2(a))", root, home, want)
	}
	f := bsLiveFixture(t)
	if !insideDir(root, f.store) {
		t.Fatalf("CONTROL: the fixture's store %s is not under the wall root %s this home composes, so the shape below is some other seat's", f.store, root)
	}

	wt := filepath.Join(f.common, "worktrees")
	wide := append(append([]string{}, f.set...), f.common)
	bsWideGrantPinsTheLooseComponent(t, f, wide, wt)

	// The live half: the pin the arm above read off the argv, as the kernel
	// enforces it.
	argv := f.render(t, wide, f.binds(t))
	moved := wt + ".stolen"
	switch ok, out := f.in(t, argv, "", "mv", "-f", wt, moved); {
	case ok:
		t.Errorf("a seat granted %s whole renamed %s, which this render binds in place — then the next launch binds whatever a leftover of this seat plants at that name, read-write and with no race to win (ADR 0100 D2(b))\n%s", f.common, wt, out)
	case !strings.Contains(out, "busy"):
		t.Errorf("the rename of %s was refused by something other than the mount: %s", wt, out)
	}

	// THE CONTROL, and it is the cut argv handed to bwrap rather than to the
	// render: BwrapArgs refuses to produce this one (the arm above is what
	// says so), and bwrap has no such scruple. The rename lands, so the
	// refusal above is the in-place mount's and not the grant's.
	cut := bsArgvWithoutInPlaceBind(argv, wt)
	if len(cut) == len(argv) {
		t.Fatalf("CONTROL: %s is a mount destination of this render by some option other than an in-place --bind, so the row below measures nothing\n argv: %q", wt, argv)
	}
	if ok, out := f.in(t, cut, "", "mv", "-f", wt, moved); !ok {
		t.Fatalf("NOTHING MEASURED: a namespace with no mount over %s could not rename it either, so the refusal above is not the mount's:\n%s", wt, out)
	}
	if _, err := os.Lstat(moved); err != nil {
		t.Errorf("the control's rename reported success and %s is not there: %v", moved, err)
	}
	// Put it back: the fixture's own cleanup prunes this worktree through
	// git, which reads `<common>/worktrees` by name.
	if err := os.Rename(moved, wt); err != nil {
		t.Errorf("the control left %s renamed, which the fixture's cleanup reads: %v", wt, err)
	}
}

// ADR 0100 D1, live: the seat reaches its store's CONTENTS through the mount
// at `<common>/objects` and cannot touch the store's own HOST path, nor any
// component of it. That is the whole of what moving the source bought — the
// store used to sit inside the per-worktree git dir, which this seat is
// granted, so a leftover of it could rename `seat-objects` to a link at
// `<common>/objects` and have the NEXT launch bind the SHARED store
// read-write over that name, no race to win.
//
// MUTATION RUN: putting the store back under the git dir (seatStoreFrom's
// Store) reds every row here — the seat can write its own store's name
// again — and reds TestQABwrapSeatStoreSourceIsPinnedInTheRender beside it.
func TestLiveBwrapSeatStoreHostPathIsUnreachableInTheWall(t *testing.T) {
	t.Parallel()
	f := bsLiveFixture(t)
	wall := filepath.Dir(f.store)
	if !insideDir(SeatWallRoot(bsLiveHome(t)), f.store) {
		t.Fatalf("CONTROL: the store %s is not under this box's wall root, so the rows below measure some other directory", f.store)
	}
	// The CONTENTS are reachable, at the mount and only there: the control
	// that says the rows below are about the name and not about the store.
	if ok, out := f.in(t, f.argv, "x\n", f.git("hash-object", "-w", "--stdin")...); !ok {
		t.Fatalf("CONTROL: the seat cannot write an object at all, so nothing below is about the wall:\n%s", out)
	}
	for _, c := range []struct{ name, sh string }{
		{"write into the store by its host path", "printf evil > " + f.store + "/evil"},
		{"rename the store itself", "mv -f " + f.store + " " + f.store + ".x"},
		{"replace the store with a link to the shared one", "rm -rf -- " + f.store + " && ln -s " + filepath.Join(f.common, "objects") + " " + f.store},
		{"write in the wall dir above it", "printf evil > " + wall + "/evil"},
		{"rename the wall dir", "mv -f " + wall + " " + wall + ".x"},
	} {
		if ok, out := f.in(t, f.argv, "", "sh", "-c", c.sh); ok {
			t.Errorf("a walled seat could %s — the next launch's bind source is the seat's to choose (ADR 0100 D1)\n %s\n%s", c.name, c.sh, out)
		}
	}
	// And the store is still a directory with the alternates the launcher
	// wrote, so none of the rows above half-succeeded.
	if fi, err := os.Lstat(f.store); err != nil || !fi.IsDir() {
		t.Fatalf("the store is no longer a directory after those attempts: %v %v", fi, err)
	}
	if b, err := os.ReadFile(filepath.Join(f.store, "info", "alternates")); err != nil || strings.TrimSpace(string(b)) != f.base {
		t.Errorf("the store's alternates changed under the wall: %q %v", b, err)
	}
}

// Pin (3): end to end through the real wall and the real landing. An honest
// seat's commit lands on the base unchanged; a seat that rewrites one of ITS
// OWN objects after committing (which its store lets it) lands nothing: the
// name it claimed stays absent from the shared store, main does not move, and
// the tree is kept.
//
// MUTATION RUN: the import copying the seat's loose files straight into the
// shared store (no re-hash) reds the tampered arm; dropping the store bind
// reds the honest arm's control (the commit is in the shared store at once).
func TestLiveBwrapSeatStoreLandsHonestWorkAndNeverATamperedObject(t *testing.T) {
	t.Parallel()
	t.Run("honest", func(t *testing.T) {
		t.Parallel()
		f := bsLiveFixture(t)
		ok, out := f.in(t, f.argv, "", "sh", "-c",
			"printf 'the work\\n' > fix.txt && "+strings.Join(f.git("add", "fix.txt"), " ")+" && "+strings.Join(f.git("commit", "-q", "-m", "s-1:fix"), " "))
		if !ok {
			t.Fatalf("the seat could not commit under its store:\n%s", out)
		}
		head := mustGit(t, f.tr.Path, "rev-parse", "HEAD")
		if siHas(t, f.repo, head) {
			t.Fatal("CONTROL: the seat's commit is in the shared store before any import, so the store bind did not take")
		}
		o, err := MergeSessionWork(f.a, f.tr)
		if err != nil {
			t.Fatal(err)
		}
		if !o.Merged {
			t.Fatalf("an honest walled seat's work did not land: %+v", o)
		}
		if got := mustGit(t, f.repo, "rev-parse", "main"); got != head {
			t.Fatalf("main is at %s, not the seat's commit %s", got[:12], head[:12])
		}
		if got := mustGit(t, f.repo, "show", "main:fix.txt"); got != "the work" {
			t.Fatalf("main:fix.txt reads %q", got)
		}
	})
	t.Run("tampered", func(t *testing.T) {
		t.Parallel()
		f := bsLiveFixture(t)
		ok, out := f.in(t, f.argv, "", "sh", "-c",
			"printf 'the claimed bytes\\n' > fix.txt && "+strings.Join(f.git("add", "fix.txt"), " ")+" && "+strings.Join(f.git("commit", "-q", "-m", "s-1:fix"), " "))
		if !ok {
			t.Fatalf("the seat could not commit under its store:\n%s", out)
		}
		// Read inside the wall: outside, before the import, the tip's
		// objects are not there to read (ADR 0099 D6).
		ok, claimed := f.in(t, f.argv, "", f.git("rev-parse", "HEAD:fix.txt")...)
		if !ok {
			t.Fatalf("the seat cannot read its own commit:\n%s", claimed)
		}
		ok, forged := f.in(t, f.argv, "the forged bytes\n", f.git("hash-object", "-w", "--stdin")...)
		if !ok {
			t.Fatalf("the seat could not write the forged object:\n%s", forged)
		}
		// THROUGH `<common>/objects`, which is the only name the seat has
		// for its own store since ADR 0100 D1: the store's host path is
		// under the seat's wall dir, which no render grants, so inside the
		// namespace it is under the read-only `/`. The arm below measures
		// that; here the point is the tamper, and the tamper is what a seat
		// can really do.
		inStore := func(sha string) string { return filepath.Join(f.common, "objects", sha[:2], sha[2:]) }
		from, to := inStore(forged), inStore(claimed)
		if ok, out := f.in(t, f.argv, "", "mv", "-f", from, to); !ok {
			t.Fatalf("CONTROL: the seat could not rewrite an object in its OWN store, so this is not the tamper the landing is for:\n%s", out)
		}
		before := mustGit(t, f.repo, "rev-parse", "main")
		o, err := MergeSessionWork(f.a, f.tr)
		if err != nil {
			t.Fatal(err)
		}
		if o.Merged {
			t.Fatalf("a tip naming a tampered object landed: %+v", o)
		}
		if got := mustGit(t, f.repo, "rev-parse", "main"); got != before {
			t.Fatalf("main moved to %s over a refused landing", got[:12])
		}
		// MEASURED 2026-11-15 (git 2.56.0, linux 7.2.15): the tree that
		// names the claimed blob travels in the same pack, so `index-pack
		// --strict` refuses the WHOLE pack ("did not receive expected
		// object") and nothing of the store lands — not the forged bytes
		// under their true name either. xcucy's predicted-name pin is the
		// shape where no tree names the plant and the true name does land.
		if !strings.Contains(o.Reason, "could not be imported") {
			t.Errorf("the refusal is not the import's: %s", o.Reason)
		}
		for name, sha := range map[string]string{"the name the seat claimed": claimed, "the forged bytes' true name": forged} {
			if siHas(t, f.repo, sha) {
				t.Errorf("the shared store holds %s, %s, from a store whose import was refused", sha[:12], name)
			}
		}
	})
}

// Pin (4): an operator `gc` succeeds right after a watch-pass import while a
// walled seat holds a staged blob only its store has — and fails before it,
// which is the control that says the blob really was in the seat's store.
func TestLiveBwrapSeatStoreOperatorGcAfterTheWatchPassImport(t *testing.T) {
	t.Parallel()
	f := bsLiveFixture(t)
	ok, out := f.in(t, f.argv, "", "sh", "-c",
		"printf 'staged, never committed\\n' > staged.txt && "+strings.Join(f.git("add", "staged.txt"), " "))
	if !ok {
		t.Fatalf("the seat could not stage under its store:\n%s", out)
	}
	if _, err := git(f.repo, "gc", "-q"); err == nil {
		t.Fatal("CONTROL: gc succeeded with a staged blob only the seat store holds, so the import has nothing to fix")
	}
	if err := importAllSeatStores(f.repo); err != nil {
		t.Fatal(err)
	}
	if _, err := git(f.repo, "gc", "-q"); err != nil {
		t.Fatalf("the operator's gc still fails after the watch-pass import: %v", err)
	}
}
