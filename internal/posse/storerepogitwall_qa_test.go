//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-kfrpv — THE STORE REPO'S `.git` IS READ-ONLY TO
// EVERY SEAT, AT EVERY TIER.
//
// WHAT WAS WRONG. A seat whose store of record lives in ANOTHER repository
// (`.beads/redirect`, ADR 0012 D3-C — the shape every store on this
// instance has) was granted that repo's git dirs WHOLE: `beadsGitDirs(home)`
// answers `<root>/.git` plus `--git-dir` and `--git-common-dir`, unnarrowed,
// and both `SeatbeltWritable` (L2, and L3 through it) and
// `launchWritableRoots` (the self-caging runtimes' `--add-dir` set) put
// every one of them in the write set. VERIFIED by the security lane on a
// live bwrap seat 2026-11-15 (ranger-base-julaf, main 955667e8): an rw bind on
// `<store>/.git` whole, ro binds only on its `config` and `hooks`, NO
// deeper mount on `objects`, `objects/info`, `refs/heads`, `packed-refs`,
// `refs/replace`, `info/`, `index` or `logs`, and access(W_OK) true on
// `.git`, `objects` and `refs/heads`.
//
// That is ADR 0099's D1 hole for the store repo, and D1's own answer — the
// per-seat object store — is scoped to the SESSION's repo. A compromised
// seat could overwrite or unlink the object files every uncaged reader
// trusts (git never re-hashes on read), move `refs/heads/main` for the
// operator's next push, or plant `objects/info/alternates` and
// `refs/replace/*` so uncaged git serves content of the seat's choosing.
//
// WHY THE GRANT IS GONE RATHER THAN NARROWED. Its stated reason was "`bd
// sync` COMMITS the JSONL there" (ranger-base-rhw, 2026-08-26, written when
// the store lived in the constitution repo). MEASURED 2026-11-20, bd
// 0.50.3: `bd sync --help` says "Does NOT stage or commit - that's the
// user's job"; the committing spelling is `bd sync --full`, which every crew
// PID denies; and since ADR 0015 §4 the JSONL's committer is the LAUNCHER,
// uncaged and under the launcher lock (`App.CommitQueueJSONL`). MEASURED the
// same day by `scripts/store-git-write-census.py` over every session
// transcript on this box: 631 files, 190 seat-session commands naming the
// store repo, every git verb among them a read, and the WRITE set empty —
// all ten writes are the operator's own uncaged session. MEASURED by construction with the store repo's
// `.git` bound read-only (control: a create there answers `Read-only file
// system`): `bd show`, `bd list`, `bd ready`, `bd sync --status` and `bd
// comments add` all exit 0.
//
// L4 ALREADY RULED THIS WAY and its own comment says so ("The target alone,
// not its `.git` … the grant L2 needs for a sync buys nothing here and
// would mount a second repo's history read-write"). Its arm is
// `TestRedirectedBeadStoreCrossesTheBoundaryReadWrite` (cageoverlay_test.go),
// which has asserted `wantNoMount(<store>/.git)` since that tier shipped and
// is green here unchanged — there is no L4 arm in this file for that reason.
// These arms are the same ruling at the three tiers that never followed it.
//
// WHAT MAKES EACH ARM WORTH A PIN rather than a reading of the producer:
//
//  1. the PRODUCERS are three (L2/L3, the self-caging line, the gate) and a
//     grant restored in any one of them is one `append` nothing else says
//     anything about. The gate is the subtle one — it must lose the probe
//     with the grant, or every such launch refuses for a write nothing can
//     satisfy (ranger-base-6hjm5's defect, one shape over).
//  2. the RENDER is where a write grant becomes a bind, and the bead's own
//     prescription asked for this arm by name. A set that no longer names
//     the path renders no `--bind` over it; the control is that the pre-fix
//     set still does.
//  3. the DENY LIST is the adjacent availability cost. An EXISTING deny
//     renders a read-only bind whatever the grants say
//     (`bwrapPrivateArgs`' loop only skips an ABSENT one) and wall-watch
//     lists that bind, so while `sessionGitConfigFiles` and
//     `sessionHooksDirs` named the store repo, any operator `git config` or
//     `branch -D` there killed EVERY live bwrap seat on the box.

import (
	"path/filepath"
	"strings"
	"testing"
)

// srgFixture is wgDenyFixture plus a store of record in a SECOND repository,
// reached by a `.beads/redirect` from the session tree — the shape the hole
// was verified in, and the one the bead asked these pins to render.
type srgFixture struct {
	wgFixture
	store string // the store repo's root
	home  string // <store>/.beads RESOLVED, the path a wall literal carries
	spelt string // the same store as beadsHome SPELLS it — a producer's input
	git   string // <store>/.git, which is both its git dir and its common dir
}

func srgNew(t *testing.T) srgFixture {
	t.Helper()
	f := wgDenyFixture(t)
	store := sbMkdir(t, filepath.Join(filepath.Dir(f.repo), "store"))
	sbGitInit(t, store)
	home := sbMkdir(t, filepath.Join(store, beadsDirName))
	sbMkdir(t, filepath.Join(f.tree, beadsDirName))
	sbWrite(t, filepath.Join(f.tree, beadsDirName, beadsRedirect), home+"\n")
	// TWO FIELDS FOR ONE DIRECTORY, deliberately (ranger-base-1b8yv).
	// beadsHome answers bd's own spelling and not a resolved path
	// (beadloss.go), so `spelt` is what the producers are HANDED at launch
	// and `home` is what their answers must CARRY — the path the kernel
	// matches. Where the fixture root is behind a symlink the two differ,
	// which is every darwin box (`/var` → `/private/var`), and the arms
	// below feed the one and assert the other.
	s := srgFixture{wgFixture: f, store: store,
		home:  absResolve(home),
		spelt: wgStoreIs(t, "the redirect does not reach the store, so every arm here measures nothing", f.tree, home),
		git:   absResolve(filepath.Join(store, ".git"))}
	// A MAIN checkout, which is what the verified shape is and what `bd
	// worktree create` always makes: git-dir and common dir are one path,
	// so there is no per-worktree dir anywhere in the store repo to hide a
	// grant in.
	if gd := beadsGitDirs(s.spelt); len(gd) == 0 || !bgHas(gd, s.git) {
		t.Fatalf("beadsGitDirs does not name %s for this store, so the negative arms below are not over the defect: %v", s.git, gd)
	}
	return s
}

// srgPaths is what a grant or a bind must not reach: the store repo's git dir
// and the five regions under it the verified hole made writable. Named
// individually rather than tested by containment alone, because a producer
// that re-granted ONE of them — `objects` for a store (ADR 0099's shape), or
// `refs/heads` for a ref move — is the defect back at a narrower spelling,
// and a containment test over the parent reads the same either way.
func (f srgFixture) srgPaths() []string {
	return []string{
		f.git,
		filepath.Join(f.git, "objects"),
		filepath.Join(f.git, "objects", "info"),
		filepath.Join(f.git, "refs", "heads"),
		filepath.Join(f.git, "refs", "replace"),
		filepath.Join(f.git, "info"),
	}
}

// 1. THE PRODUCERS. Every tier's write set, and the gate's target list, for
// both PID classes: the store DIRECTORY and nothing of its repo's git.
//
// Narrowed to the store's repo by location (wgRepoGrants' rule, applied to
// the other repo), because this fixture sits under /tmp and the profile's
// blanket temp grant would otherwise answer for every path in it — a mask
// that would make all four arms pass over the live defect.
func TestQANoProducerGrantsTheStoreRepoGitDirs(t *testing.T) {
	f := srgNew(t)

	for _, pid := range []struct {
		name string
		ag   *AgentFile
	}{{"open", f.ag}, {"whole-tree deny", wgDeny(f.wgFixture)}} {
		sets := map[string][]string{
			"SeatbeltWritable (L2, and L3 through BwrapWritable)": f.a.SeatbeltWritable(pid.ag, f.tree, f.gates),
			"launchWritableRoots (the self-caging --add-dir set)": launchWritableRoots(f.tree),
			// f.spelt and not f.home: the gate is handed beadsHome's own
			// answer at launch, so that is what it is handed here.
			"recordTargets (ADR 0013 §4's gate)": recordTargets(pid.ag, f.tree, f.spelt),
		}
		for where, set := range sets {
			inStore := wgRepoGrants(set, f.store, f.store)
			if !anyUnderDir(inStore, f.home) {
				t.Errorf("%s / %s: the store of record itself is not named, which is the grant every tier owes (ranger-base-0fb, ranger-base-rhw):\n  %s",
					where, pid.name, strings.Join(abbrevAll(set), "\n  "))
			}
			for _, bad := range f.srgPaths() {
				if anyUnderDir(inStore, bad) {
					t.Errorf("%s / %s: %s is writable — nothing in a seat commits in the store repo (ranger-base-kfrpv):\n  %s",
						where, pid.name, AbbrevHome(bad), strings.Join(abbrevAll(inStore), "\n  "))
				}
			}
		}
	}
}

// 2. THE RENDER, which is the arm the bead asked for by name: at L3 a write
// grant becomes a `--bind`, so the property has to be read off the argv and
// not off the set.
//
// The control is the pre-fix set, assembled here as the producer used to
// assemble it (the shipped set plus `beadsGitDirs`): it must render the rw
// bind this arm asserts is gone, or the arm is over nothing.
func TestQABwrapRendersNoWriteBindInTheStoreRepoGitDir(t *testing.T) {
	f := srgNew(t)
	set := f.a.SeatbeltWritable(f.ag, f.tree, f.gates)
	carve := f.a.SeatbeltCarveOut(f.ag, f.tree, f.gates, set)
	bset, keep := BwrapWritable(f.ag, set, f.tree)
	argv, _, err := BwrapArgs(f.tree, bset, carve, keep...)
	if err != nil {
		t.Fatalf("the render refuses this shape, so nothing below is measured: %v", err)
	}

	if !srgBound(argv, "--bind", f.home) {
		t.Errorf("the store of record is not bound read-write, so this seat records nothing:\n  %s", strings.Join(argv, " "))
	}
	for _, bad := range f.srgPaths() {
		if srgBound(argv, "--bind", bad) {
			t.Errorf("the render binds %s READ-WRITE — that is the verified hole (ranger-base-julaf):\n  %s", AbbrevHome(bad), strings.Join(argv, " "))
		}
	}
	// Nothing under that git dir at all, by containment, so a region added
	// to git tomorrow is covered without being listed above.
	for i, a := range argv {
		if a == "--bind" && i+1 < len(argv) && underDir(f.git, absResolve(argv[i+1])) {
			t.Errorf("the render binds %s read-write, inside the store repo's git dir:\n  %s", argv[i+1], strings.Join(argv, " "))
		}
	}

	// The control: the exact pre-fix set — beadsGitDirs handed beadsHome's
	// own spelling, as the producer handed it, and then RESOLVED on the way
	// into the write set, as that producer's `add` resolved it (seatbelt.go,
	// ranger-base-1b8yv). Resolved and not left as spelt, because the two
	// differ wherever the fixture root sits behind a symlink and this render
	// refuses an unresolved component outright (ADR 0100 D1,
	// ranger-base-wsgus) — a control that died there would say nothing about
	// the grant it exists to exhibit.
	pre := append(append([]string{}, set...), wgResolved(beadsGitDirs(f.spelt))...)
	pbset, pkeep := BwrapWritable(f.ag, pre, f.tree)
	pargv, _, err := BwrapArgs(f.tree, pbset, carve, pkeep...)
	if err != nil {
		t.Fatalf("the pre-fix set does not render, so the control says nothing: %v", err)
	}
	if !srgBound(pargv, "--bind", f.git) {
		t.Fatalf("the pre-fix set renders no read-write bind over %s, so the arms above are not over ranger-base-kfrpv's defect:\n  %s",
			AbbrevHome(f.git), strings.Join(pargv, " "))
	}
}

// 3. THE DENY LIST, and the availability cost it carried. `<store>/.git/config`
// and `<store>/.git/hooks` were denied because that `.git` was GRANTED — the
// ADR 0095 shape, a read-only bind inside a writable bind. With no grant
// there is nothing to narrow, and what the deny cost instead was every live
// seat on the box: a deny that exists is bound read-only whatever the grants
// say, wall-watch lists the bind, and a host rename of the path under it
// detaches the mount and kills the seat. So an operator `git config` or
// `branch -D` in the store repo ended EVERY live bwrap seat.
//
// The session's OWN repo keeps both denies, which is the control: the
// producers still work, they just answer for one repo now.
func TestQATheStoreRepoIsInNoDenyThatCanKillTheSeat(t *testing.T) {
	f := srgNew(t)
	set := f.a.SeatbeltWritable(f.ag, f.tree, f.gates)
	carve := f.a.SeatbeltCarveOut(f.ag, f.tree, f.gates, set)

	for _, d := range carve.Deny {
		if underDir(f.git, absResolve(d)) {
			t.Errorf("%s is denied — a deny inside the store repo's git is a read-only bind wall-watch lists, so a host write there kills every live seat (ranger-base-kfrpv):\n  %s",
				AbbrevHome(d), strings.Join(abbrevAll(carve.Deny), "\n  "))
		}
	}
	// The producers' own answers, read directly, so a deny that reached the
	// carve-out by another route is still caught and an empty carve-out
	// cannot pass this arm.
	for _, got := range [][]string{sessionGitConfigFiles(f.tree), sessionHooksDirs(f.tree)} {
		if len(got) == 0 {
			t.Fatal("a session's own config/hooks producer answers nothing, so this arm's control is empty")
		}
		for _, p := range got {
			if underDir(f.git, absResolve(p)) {
				t.Errorf("%s names the STORE repo, whose git nothing grants any more (ranger-base-kfrpv): %v", AbbrevHome(p), abbrevAll(got))
			}
			if !underDir(f.repo, absResolve(p)) && !underDir(f.tree, absResolve(p)) {
				t.Errorf("%s is in neither the session's repo nor its tree — these producers answer for the session's own repo and nothing else: %v", AbbrevHome(p), abbrevAll(got))
			}
		}
	}
}

// 4. THE DARWIN PROFILE, where the wall is a text and not a mount table.
// SBPL is last-match-wins, so what matters is the rendered artifact: the
// store directory is granted as a subpath and the store repo's git dir
// appears nowhere in the profile at all.
func TestQASeatbeltProfileGrantsNoStoreRepoGitDir(t *testing.T) {
	f := srgNew(t)
	set := f.a.SeatbeltWritable(f.ag, f.tree, f.gates)
	carve := f.a.SeatbeltCarveOut(f.ag, f.tree, f.gates, set)
	prof := SeatbeltProfile(f.ag.Name, set, SeatbeltSiblings(nil, carve), carve, sessionRefDirsFor(f.ag, f.tree)...)

	if !strings.Contains(prof, `(subpath "`+f.home+`")`) {
		t.Errorf("the profile does not grant the store of record:\n%s", prof)
	}
	for _, bad := range f.srgPaths() {
		if strings.Contains(prof, `"`+bad+`"`) {
			t.Errorf("the profile names %s — no line of it may mention the store repo's git, granted or denied (ranger-base-kfrpv):\n%s", bad, prof)
		}
	}
}

// srgBound answers whether argv carries `<kind> <path> …` for exactly this
// path as a bind SOURCE. By spelling and resolved, because the render
// resolves what it binds and a containment test here would read a bind of a
// parent as a bind of this name.
func srgBound(argv []string, kind, path string) bool {
	want := absResolve(path)
	for i, a := range argv {
		if a == kind && i+1 < len(argv) && absResolve(argv[i+1]) == want {
			return true
		}
	}
	return false
}
