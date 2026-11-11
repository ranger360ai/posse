//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-6hjm5 — the record-reach gate (ADR 0013 §4) asks
// for exactly the git dirs the wall grants, in every shape.
//
// Claim: for a launch in `cwd`, every target `recordTargets` names is inside
// some entry of the writable set `SeatbeltWritable` produces for that same
// launch. Not "a reasonable set" and not "bd's wishlist": the two are read by
// the gate and by the wall for the same launch, and a target the wall does
// not grant refuses the launch for a write nothing can satisfy, while a grant
// the gate does not probe is a wall nobody measures.
//
// WHAT WAS WRONG, and what makes these arms the shapes they are
// (ranger-base-6hjm5, from ranger-base-o8us6's reading). The gate derived its
// git targets from `beadsGitDirs(home)` — which answers with `--git-dir` and
// `--git-common-dir`, so in a LINKED WORKTREE the common dir `<repo>/.git`
// WHOLE. Since ranger-base-m2wf no PID is granted that directory: a session
// worktree gets `objects`, `logs`, `packed-refs.lock` and its own ref beneath
// it, and nothing more. So an OPEN PID — no deny at all — whose store of
// record is inside its own repository was probed for a write the wall
// deliberately withholds, the row read unrealized, and the launch refused.
//
// MEASURED 2026-11-11, D1 host, through the two shipped producers over this
// fixture, open PID, grants narrowed to the session's own repo by location:
//
//	open-PID repo-local grants:
//	  <cwd>, <repo>/.git/worktrees/<session>, <repo>/.git/objects,
//	  <repo>/.git/logs, <repo>/.git/packed-refs.lock,
//	  <repo>/.git/refs/heads/posse/<session>{,.lock}
//	recordTargets:
//	  <store>, <repo>/.git/worktrees/<session>, <repo>/.git
//	PROBED BUT NOT GRANTED: <repo>/.git
//
// WHY IT IS NOT ranger-base-8t6j1's NARROWING ONE SHAPE OVER. That bead
// narrowed the gate for a whole-tree-deny PID, whose wall holds no git grant
// at all, and its control was "the open PID reads OK either way" — taken in a
// MAIN CHECKOUT, where `cwd` is granted whole and covers `.git`. In a linked
// worktree the open PID's git grants are narrowed and that control does not
// transfer. So the arms below are BOTH checkout shapes for BOTH PID classes,
// and the main-checkout arm asserts that `<cwd>/.git` is still a target —
// narrowing the gate by "is the store in my own tree" alone would have taken
// that probe with it, and that probe measures a real grant.
//
// PRODUCER-LEVEL AND DELIBERATELY SO. The fixture sits under /tmp, whose
// blanket grant would mask a refusal under a real `sandbox-exec` or `bwrap`
// (ADR 0086's own caveat about its "move main" row, and sbRoot's reason for
// skipping) — so what is asserted is that the two shipped functions AGREE,
// with the grant narrowed to the session's own repository by location
// (wgRepoGrants) and never by subtracting a subtree. The behavioural arms
// over a rendered wall are seatbeltworktreegit_qa_test.go's and
// reachability_qa_test.go's; this is the one they cannot run on this host.

import (
	"path/filepath"
	"strings"
	"testing"
)

// rrgUngranted is the whole question, once: the targets of a launch that the
// writable set for that same launch does not reach, with both sides narrowed
// to the session's own repository so the blanket temp grant cannot answer for
// anything. It fatals when either side is empty in this repo — a vacuous
// comparison is the failure these arms exist to catch.
func rrgUngranted(t *testing.T, f wgFixture, ag *AgentFile, cwd string) []string {
	t.Helper()
	grants := wgGrants(t, f, ag, cwd)
	targets := wgRepoGrants(recordTargets(ag, cwd, beadsHome(cwd)), f.repo, f.tree)
	if len(targets) == 0 {
		t.Fatalf("the gate names no target inside this repo for %s in %s, so this arm compares nothing:\n  %s",
			ag.Name, cwd, strings.Join(wgKinds(recordTargets(ag, cwd, beadsHome(cwd))), "\n  "))
	}
	var out []string
	for _, p := range targets {
		if !anyUnderDir(grants, p) {
			out = append(out, p)
		}
	}
	return out
}

// 1. The shape the defect was in: an OPEN PID, a linked worktree, and the
// store of record inside the session's own repository. The gate's git targets
// have to be the narrowed set the wall actually grants there.
//
// The control is the pre-fix derivation itself, run over the same fixture: if
// `beadsGitDirs` stopped naming the common dir whole here, the arm above
// would pass over a shape that is no longer the defective one.
func TestQARecordTargetsAskOnlyForGitDirsTheWallGrantsInALinkedWorktree(t *testing.T) {
	f := wgDenyFixture(t)
	sbMkdir(t, filepath.Join(f.tree, beadsDirName))
	if got := beadsHome(f.tree); got != absResolve(filepath.Join(f.tree, beadsDirName)) {
		t.Fatalf("the local store does not resolve, so this arm measures nothing: beadsHome = %s", got)
	}

	if bad := rrgUngranted(t, f, f.ag, f.tree); len(bad) != 0 {
		t.Errorf("the record-reach gate probes %s, which no PID is granted in a linked worktree (ranger-base-m2wf) — the launch refuses for a write the wall deliberately withholds:\n  %s",
			strings.Join(abbrevAll(bad), " "), strings.Join(wgKinds(bad), "\n  "))
	}

	// Non-vacuity: the pre-fix target set, derived the way the gate used to
	// derive it, is NOT covered by this grant.
	wide := wgRepoGrants(beadsGitDirs(beadsHome(f.tree)), f.repo, f.tree)
	if !bgHas(wide, f.common) {
		t.Fatalf("beadsGitDirs no longer names the common git dir whole in this shape, so the arm above is not over the defect:\n  %s",
			strings.Join(wgKinds(wide), "\n  "))
	}
	if anyUnderDir(wgGrants(t, f, f.ag, f.tree), f.common) {
		t.Fatalf("the common git dir IS granted whole to an open PID in a linked worktree — ranger-base-m2wf's narrowing is gone, and the arm above holds for the wrong reason")
	}
}

// 2. ranger-base-8t6j1's own control, kept: in a MAIN CHECKOUT the open PID's
// `.git` probe measures a real grant (`cwd` whole covers it), so it stays a
// target. A gate narrowed by "is the store inside my tree" alone would have
// dropped it, and nothing would have said so.
func TestQARecordTargetsStillProbeTheGitDirAMainCheckoutGrants(t *testing.T) {
	f := wgDenyFixture(t)
	sbMkdir(t, filepath.Join(f.repo, beadsDirName))

	got := recordTargets(f.ag, f.repo, beadsHome(f.repo))
	if !bgHas(got, filepath.Join(f.repo, ".git")) {
		t.Errorf("the open PID in a main checkout is no longer asked for its own `.git` — that probe measures the grant `cwd` whole carries, and it is ranger-base-8t6j1's control:\n  %s",
			strings.Join(wgKinds(got), "\n  "))
	}
	if bad := rrgUngranted(t, f, f.ag, f.repo); len(bad) != 0 {
		t.Errorf("the gate probes what the main-checkout grant does not reach:\n  %s", strings.Join(wgKinds(bad), "\n  "))
	}
}

// 3. The store of record in ANOTHER repository — the shape every store on
// this instance has. Its git dirs are granted WHOLE for every PID, because bd
// commits there, so they stay targets whole and the agreement is the other
// way round: the gate must not go narrower than the wall here either.
func TestQARecordTargetsKeepTheStoreRepoGitDirsBothPidsAreGranted(t *testing.T) {
	f := wgDenyFixture(t)
	store := sbMkdir(t, filepath.Join(filepath.Dir(f.repo), "store"))
	sbGitInit(t, store)
	sbMkdir(t, filepath.Join(store, beadsDirName))
	sbMkdir(t, filepath.Join(f.tree, beadsDirName))
	sbWrite(t, filepath.Join(f.tree, beadsDirName, beadsRedirect), filepath.Join(store, beadsDirName)+"\n")
	home := beadsHome(f.tree)
	if home != absResolve(filepath.Join(store, beadsDirName)) {
		t.Fatalf("the redirect does not resolve, so this arm measures nothing: beadsHome = %s", home)
	}

	for _, ag := range []*AgentFile{f.ag, wgDeny(f)} {
		got := recordTargets(ag, f.tree, home)
		for _, want := range []string{home, filepath.Join(store, ".git")} {
			if !bgHas(got, want) {
				t.Errorf("%s lost %s as a target — the store of record in another repo keeps its git dirs granted for every PID, so the gate keeps probing them:\n  %s",
					ag.Name, want, strings.Join(wgKinds(got), "\n  "))
			}
		}
		if !anyUnderDir(f.a.SeatbeltWritable(ag, f.tree, f.gates), filepath.Join(store, ".git")) {
			t.Fatalf("the store repo's git dir is not granted to %s, so the arm above asserts a probe of an ungranted path", ag.Name)
		}
	}
}

// 4. Both PID classes, both checkout shapes, one loop: the agreement is a
// property of the pair of producers and not of the four cases someone thought
// to write down. A fifth shape added tomorrow — the store inside the tree but
// outside `<cwd>/.beads` (ranger-base-f5dg's REPRO 2), which is the one
// ranger-base-o8us6 was about — is in the table because it is the shape the
// two conditions are deliberately one sentence for.
func TestQARecordTargetsAgreeWithTheWallForEveryPidAndCheckoutShape(t *testing.T) {
	for _, shape := range []struct {
		name  string
		store func(t *testing.T, f wgFixture, cwd string)
	}{
		{"local store", func(t *testing.T, f wgFixture, cwd string) {
			sbMkdir(t, filepath.Join(cwd, beadsDirName))
		}},
		{"redirect inside this repo", func(t *testing.T, f wgFixture, cwd string) {
			inner := sbMkdir(t, filepath.Join(cwd, "inner", beadsDirName))
			sbMkdir(t, filepath.Join(cwd, beadsDirName))
			sbWrite(t, filepath.Join(cwd, beadsDirName, beadsRedirect), inner+"\n")
		}},
	} {
		for _, in := range []string{"worktree", "checkout"} {
			for _, pid := range []string{"open", "deny"} {
				t.Run(shape.name+"/"+in+"/"+pid, func(t *testing.T) {
					f := wgDenyFixture(t)
					cwd := f.tree
					if in == "checkout" {
						cwd = f.repo
					}
					ag := f.ag
					if pid == "deny" {
						ag = wgDeny(f)
					}
					shape.store(t, f, cwd)
					if bad := rrgUngranted(t, f, ag, cwd); len(bad) != 0 {
						t.Errorf("the gate asks %s for a write the wall does not grant it:\n  %s",
							ag.Name, strings.Join(wgKinds(bad), "\n  "))
					}
				})
			}
		}
	}
}

// 5. EVERY TARGET IS A DIRECTORY, which is the other half of "the gate asks
// for exactly what the wall grants" and the half nothing pinned
// (ranger-base-n2yaw, finding 1 of the ranger-base-3fm26 verify).
//
// `recordTargets` filters its git half through `isDirPath`, and that filter
// BECAME LOAD-BEARING with ranger-base-6hjm5. Before it, the git half was
// always `beadsGitDirs`, which names directories only, so the filter had
// nothing to do — it has been there since 9c00e192 for a `.git` that does
// not exist. The new arm asks `sessionGitGrants`, and three of that
// function's six entries are FILE paths: `<common>/packed-refs.lock`,
// `<common>/refs/heads/<branch>` and that path plus `.lock` (seatbelt.go,
// ADR 0059 D1). The ref file is real and present in a dispatched session,
// not hypothetical.
//
// WHAT IT COSTS IF THE FILTER GOES: `seatbeltReachProbe` runs
// `: > <target>/posse-reach-probe.<pid>`, which on a regular file is ENOTDIR
// — MEASURED, `/bin/sh` answers `Not a directory` — so the row reads
// unrealized and the launch refuses, for exactly the open-PID /
// linked-worktree / store-inside-its-own-repo shape ranger-base-6hjm5 exists
// to fix. The four arms above cannot see it by construction: they assert the
// targets are a SUBSET of the grants, and the three lock paths ARE granted.
//
// So this arm reads the KIND and not the containment, the way
// seatbeltdenygitgrant's arms do and for the same reason — "inside a git dir"
// and "is it a file" are different questions, and only the second decides
// whether the probe can answer at all.
func TestQARecordTargetsAreAllDirectoriesTheProbeCanWriteIn(t *testing.T) {
	f := wgDenyFixture(t)
	sbMkdir(t, filepath.Join(f.tree, beadsDirName))
	home := beadsHome(f.tree)
	if home != absResolve(filepath.Join(f.tree, beadsDirName)) {
		t.Fatalf("the local store does not resolve, so this arm measures nothing: beadsHome = %s", home)
	}

	// Non-vacuity FIRST, because this arm's live answer is "every target is a
	// directory" either way once the filter is there, and a filter nobody
	// feeds a file cannot be told from a filter that is gone: the producer
	// underneath it has to be handing this shape a path the probe could not
	// write in. The day it stops, this arm is no longer over the defect and
	// says so here rather than passing quietly.
	raw := recordGitTargets(f.ag, f.tree, home)
	var unprobeable []string
	for _, p := range raw {
		if !isDirPath(p) {
			unprobeable = append(unprobeable, p)
		}
	}
	if len(unprobeable) == 0 {
		t.Fatalf("recordGitTargets hands this shape nothing but directories, so the isDirPath filter in recordTargets has nothing to do and this arm is not over ranger-base-n2yaw's finding — sessionGitGrants' lock and ref entries are what made that filter load-bearing:\n  %s",
			strings.Join(wgKinds(raw), "\n  "))
	}

	got := recordTargets(f.ag, f.tree, home)
	var bad []string
	for _, p := range got {
		if !isDirPath(p) {
			bad = append(bad, p)
		}
	}
	if len(bad) != 0 {
		t.Errorf("the record-reach gate would probe %s, and no probe can answer there: seatbeltReachProbe writes `<target>/posse-reach-probe.<pid>` and the kernel says ENOTDIR on a regular file and ENOENT on an absent one, so the row reads unrealized and the launch refuses for a write nothing could have satisfied. recordGitTargets hands this shape %d such path(s) and recordTargets must keep filtering them out:\n  %s",
			strings.Join(abbrevAll(bad), " "), len(unprobeable), strings.Join(wgKinds(got), "\n  "))
	}
}
