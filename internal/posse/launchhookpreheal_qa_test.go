//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-2mogn — part 1 of ranger-base-qxwd's "actual hole,
// in two parts" (part 2, the enumerator, is ranger-base-ixv4's SweepHookWall,
// pinned in hookwallsweep_qa_test.go).
//
// THE DEFECT. planLaunch (herdrback.go) called a.InstallCommitGuardHook(dir)
// immediately before a.CheckParityIn(...) on the next line. A launch into a
// repo whose installed hook had drifted from config re-stamped it and THEN
// probed the fresh render it had just written — so the parity check always
// saw this binary's own render, never the drift that was actually on disk a
// moment earlier. The operator never learned the wall was wrong, or for how
// long; SweepHookWall does not help here either, because it only ever sees
// the same post-heal state everyone else does.
//
// THE FIX. planLaunch now probes the commit-guard slot BEFORE the install
// call overwrites it, and — if that pre-heal probe disagreed with what
// config now says — warns naming the disagreement, before proceeding with
// the (still self-healing) launch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A repo whose installed prepare-commit-msg stamp disagrees with what config
// NOW says (planted PUBLIC, config says private — the direction that fails
// toward disclosure, ranger-base-qxwd) is reported as a finding naming the
// disagreement BEFORE the launch's own install call silently repairs it.
func TestQALaunchReportsPreHealHookDriftBeforeSilentlyRepairingIt(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	a := b.App

	// Plant a whole, current, marker-bearing PUBLIC render — stale in
	// exactly the one line that decides the exemption, same fixture shape
	// TestHookWallSweepCatchesAStampThatDisagreesWithConfig uses.
	hook := hwsHook(t, repo, "prepare-commit-msg")
	if err := WriteExecutable(hook, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.BeadsVisibility(repo); v != VisibilityPrivate {
		t.Fatalf("fixture config did not take: visibility is %q", v)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("launch was refused rather than self-healed: %v", err)
	}
	got := warn.String()
	if !strings.Contains(got, repo) {
		t.Fatalf("no pre-heal finding names the repo:\n%s", got)
	}
	if !strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("no pre-heal finding names the drift:\n%s", got)
	}
	if !strings.Contains(got, "ours but stale") {
		t.Fatalf("finding does not carry the probe's own reason:\n%s", got)
	}

	// The launch still self-heals: the installed hook now carries the
	// CURRENT config's render (identity ∧ behavior, ADR 0023), so the
	// session that opens is protected — this bead asks for a loud finding,
	// not a refusal.
	if post := a.probeL3Hooks(repo, false); !post.CommitGuard {
		t.Fatalf("the launch did not re-stamp the hook to the current render: %s", post.CommitGuardDegraded)
	}
}

// The control: a repo whose wall already carries this binary's current
// render is not reported as drifted. Without this arm, a launch that always
// prints the pre-heal line would pass the positive pin above for the wrong
// reason.
func TestQALaunchIsQuietWhenTheWallAlreadyCarriesTheCurrentRender(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	a := b.App
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("launch into a fresh wall was refused: %v", err)
	}
	if got := warn.String(); strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("a fresh wall was reported as drifted:\n%s", got)
	}
}

// ─── ranger-base-kd1f1: the alarm on the shapes the fleet actually launches ──
//
// The pins above launch into the repo ITSELF. Every dispatched launch on this
// box is Worktree:true — the session gets its own tree, cut fresh, and `dir`
// is that tree from EnsureSessionTree onward, so the pre-heal probe asks its
// question of a directory that did not exist when the launch began. That is
// the shape ranger-base-kd1f1 suspected of a structurally dead alarm ("a first
// launch into a session dir that does not exist yet"), and the shape the one
// occurrence in the watch log was: the line names the WORKTREE while the file
// it faults is the shared repo's.
//
// It is a live alarm, MEASURED — the pin passes on the code as it stands. What
// it is here to hold is that it keeps passing: the probe reads the session
// tree, the install writes the repo, and both render through the one
// commitGuardLiterals derivation (gates.go). A second literal source on either
// side, or a probe that moved above EnsureSessionTree, kills the alarm on
// every dispatched launch and no non-worktree pin would notice.

// The pin ranger-base-kd1f1 asked for: a first launch into a session dir that
// does not exist yet must still report the drift it is about to repair.
func TestQALaunchReportsPreHealHookDriftOnAFirstWorktreeLaunch(t *testing.T) {
	t.Parallel()
	b, repo := lhpWorktreeFixture(t, VisibilityPrivate)
	a := b.App

	hook := hwsHook(t, repo, "prepare-commit-msg")
	if err := WriteExecutable(hook, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}

	// The premise this arm exists for, asserted rather than assumed: the tree
	// the probe will read is not there yet when the launch is asked for.
	wt, err := a.SessionTreePath(repo, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("premise: %s already exists (%v) — this is the re-launch shape, not the first-launch one", wt, err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", Worktree: true}); err != nil {
		t.Fatalf("worktree launch was refused rather than self-healed: %v", err)
	}
	got := warn.String()
	if !strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("no pre-heal finding on a first worktree launch:\n%s", got)
	}
	// The line has to name the file to fix, which is the SHARED repo's hook —
	// a worktree has no hooks dir of its own. Naming the session tree there
	// would send the operator to a path that does not hold the file.
	if !strings.Contains(got, hook) {
		t.Errorf("the finding does not name the shared repo's hook %s:\n%s", hook, got)
	}
	if !strings.Contains(got, "ours but stale") {
		t.Errorf("finding does not carry the probe's own reason:\n%s", got)
	}
	if post := a.probeL3Hooks(repo, false); !post.CommitGuard {
		t.Errorf("the worktree launch did not re-stamp the shared hook: %s", post.CommitGuardDegraded)
	}
}

// The control for the arm above, and the one candidate ranger-base-kd1f1
// named that a positive pin cannot rule out: the probe's render is derived
// from the SESSION dir and the install's from the repo, so a launch could
// report every worktree as drifted for a reason that is only about where the
// two derivations were rooted. A fresh wall plus a first worktree launch is
// the arm that fails if they ever stop agreeing.
func TestQALaunchIsQuietOnAFirstWorktreeLaunchIntoAFreshWall(t *testing.T) {
	t.Parallel()
	b, repo := lhpWorktreeFixture(t, VisibilityPrivate)
	a := b.App
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", Worktree: true}); err != nil {
		t.Fatalf("worktree launch into a fresh wall was refused: %v", err)
	}
	if got := warn.String(); strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("a fresh wall was reported as drifted from a worktree launch:\n%s", got)
	}
}

// The incident's own shape (ranger-base-kd1f1, ranger-base-swg17): in
// ~/src/posse a foreign shim holds the slot, so posse's gate lives at
// posse-prepare-commit-msg behind the prescribed chain dispatcher, and it is
// THAT file both the watch sweep and verify-hook-freshness.sh named stale.
// The dispatcher itself is byte-current in that state — an alarm that only
// ever looked at the slot would read the chain as fresh and say nothing about
// the gate rotting behind it.
func TestQALaunchReportsPreHealDriftBehindTheChainDispatcher(t *testing.T) {
	t.Parallel()
	b, repo := lhpWorktreeFixture(t, VisibilityPrivate)
	a := b.App

	slot := hwsHook(t, repo, "prepare-commit-msg")
	member := filepath.Join(filepath.Dir(slot), "posse-prepare-commit-msg")
	// The third party's hook, the dispatcher that runs ours first, and our
	// gate — stale in the one line that decides the exemption.
	if err := WriteExecutable(filepath.Join(filepath.Dir(slot), "theirs-prepare-commit-msg"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(slot, []byte(chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(member, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", Worktree: true}); err != nil {
		t.Fatalf("launch into a chained repo was refused rather than self-healed: %v", err)
	}
	got := warn.String()
	if !strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("no pre-heal finding for a stale gate behind the chain dispatcher:\n%s", got)
	}
	// The file to fix is the chained member, not the slot: the slot is the
	// dispatcher and re-rendering it is not the repair.
	if !strings.Contains(got, member) {
		t.Errorf("the finding does not name %s:\n%s", member, got)
	}
	// And the launch still repaired it in place, without touching the
	// dispatcher the third party's hook hangs off.
	if post := a.probeL3Hooks(repo, false); !post.CommitGuard {
		t.Errorf("the launch did not refresh the chained member: %s", post.CommitGuardDegraded)
	}
	if body, _ := os.ReadFile(slot); string(body) != chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg") {
		t.Errorf("the launch rewrote the chain dispatcher, which is the third party's arrangement")
	}
}

// ─── ranger-base-gw9o5: absence is not foreignness, and neither is a re-stamp ─
//
// THE DEFECT, MEASURED on the operator's work box and reproduced here by the
// fixture alone: `git init` a directory and launch into it, and the launch
// printed the pre-heal alarm — "found the L3 prepare-commit-msg wall WRONG
// before this launch just silently re-stamped it — foreign hook, posse cannot
// vouch for a hook it did not write" — naming a path that held no file at all.
// Three things were wrong in one line. The wall was not WRONG, it was absent.
// Nothing was RE-STAMPED, it was installed for the first time. And the file
// was not FOREIGN: l3Identity reached "no file here" through the same return
// as "a hook somebody else wrote", so every absent slot was worded as
// somebody else's. SweepHookWall had already met that third one and corrected
// it in its own caller (hookWallLine), which is why the launch still said it.
//
// THE FIX. The probe returns which state it found (l3Verdict), so the wording
// is decided once; the launch's alarm asks for the one state its own install
// silently repairs.

// The bead's own shape: a repo created by plain `git init`, launched into for
// the first time. The wall gets installed and the launch says nothing about
// it.
func TestQALaunchIsQuietOnAFirstLaunchIntoARepoWithNoHookAtAll(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	a := b.App

	// The premise, asserted rather than assumed: nothing in the slot.
	hook := hwsHook(t, repo, "prepare-commit-msg")
	if _, err := os.Lstat(hook); !os.IsNotExist(err) {
		t.Fatalf("premise: %s already exists (%v) — this is not the fresh-repo shape", hook, err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("first launch into a fresh repo was refused: %v", err)
	}
	got := warn.String()
	if strings.Contains(got, "WRONG before this launch") {
		t.Errorf("a slot that held no hook was reported as a wall this launch found wrong:\n%s", got)
	}
	if strings.Contains(got, "foreign hook") {
		t.Errorf("an absent hook was reported as foreign — the reader goes looking for a file that is not there:\n%s", got)
	}
	// And the install still happened: silence here is "nothing to report",
	// never "nothing was done".
	if post := a.probeL3Hooks(repo, false); !post.CommitGuard {
		t.Errorf("the launch did not install the wall it was quiet about: %s", post.CommitGuardDegraded)
	}
}

// A hook posse did not write is refused by installHook (ADR 0002 §3), so the
// launch cannot have re-stamped it. It is reported — by the parity check, on
// the post-heal state the repo is actually still in — and not by an alarm
// claiming a repair that did not happen.
func TestQALaunchDoesNotClaimItReStampedAForeignHook(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	hook := hwsHook(t, repo, "prepare-commit-msg")
	foreign := "#!/bin/sh\n# somebody else's\nexit 0\n"
	if err := WriteExecutable(hook, []byte(foreign), 0o755); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"})
	if err == nil {
		t.Fatal("a launch into a repo whose commit wall is a foreign hook must be refused, not marked")
	}
	if !strings.Contains(err.Error(), "foreign hook") {
		t.Errorf("the refusal does not name what is in the slot:\n%v", err)
	}
	if got := warn.String(); strings.Contains(got, "re-stamped") {
		t.Errorf("the launch claimed it re-stamped a hook install refuses to touch:\n%s", got)
	}
	// The claim the alarm would have made, checked against the disk.
	if body, _ := os.ReadFile(hook); string(body) != foreign {
		t.Errorf("the foreign hook was overwritten:\n%s", body)
	}
}

// The wording pins, at the probe, where the wording is now decided. Each is a
// state l3Identity has always told apart and the degraded line has not.
func TestQAProbeWordsAnAbsentSlotAsAbsentAndAMissingMemberAsMissing(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	a := b.App

	// 1. Nothing there at all.
	p := a.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict != l3Uninstalled {
		t.Fatalf("an empty hooks dir is not l3Uninstalled: verdict=%d line=%q", p.CommitGuardVerdict, p.CommitGuardDegraded)
	}
	if !strings.Contains(p.CommitGuardDegraded, "no hook installed at all") || strings.Contains(p.CommitGuardDegraded, "foreign") {
		t.Errorf("an absent hook is not a foreign hook: %q", p.CommitGuardDegraded)
	}
	if p.CommitGuardVerdict.reStamped() {
		t.Error("installing into an empty slot is not a re-stamp")
	}

	// 2. Our chain dispatcher holding the slot with its member gone — the
	// state posse's own uninstall line leaves behind, and the one install
	// RESTORES. The remedy is ours, so the line must not send the reader
	// looking for somebody else's hook.
	slot := hwsHook(t, repo, "prepare-commit-msg")
	member := filepath.Join(filepath.Dir(slot), "posse-prepare-commit-msg")
	if err := WriteExecutable(filepath.Join(filepath.Dir(slot), "theirs-prepare-commit-msg"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(slot, []byte(chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg")), 0o755); err != nil {
		t.Fatal(err)
	}
	p = a.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict != l3MemberGone {
		t.Fatalf("a chain dispatcher with no member is not l3MemberGone: verdict=%d line=%q", p.CommitGuardVerdict, p.CommitGuardDegraded)
	}
	if !strings.Contains(p.CommitGuardDegraded, member) || strings.Contains(p.CommitGuardDegraded, "foreign") {
		t.Errorf("the line must name the missing member and not call it foreign: %q", p.CommitGuardDegraded)
	}
	if !p.CommitGuardVerdict.reStamped() {
		t.Error("install restores the member, so a launch that does must still say so")
	}

	// 3. The wrong-arm control: a hook somebody else wrote is still foreign.
	if err := WriteExecutable(slot, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p = a.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict != l3Foreign || !strings.Contains(p.CommitGuardDegraded, "foreign hook") {
		t.Errorf("a real foreign hook lost its own wording: verdict=%d line=%q", p.CommitGuardVerdict, p.CommitGuardDegraded)
	}
	if p.CommitGuardVerdict.reStamped() {
		t.Error("installHook refuses a foreign hook, so no launch re-stamps one")
	}
}
