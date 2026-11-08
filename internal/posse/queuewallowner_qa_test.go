//go:build posse_arm3

package posse

// QA pins for ranger-base-ynzig — ADR 0081 D7, "the queue repo's guard write
// follows D3; when D3 says no, the projection commit is refused".
//
// WHAT WAS WRONG. CommitQueueJSONL reconciles the queue repo's
// prepare-commit-msg slot before every projection commit (ranger-base-mp0v,
// and the reason is in queuejsonl.go: no session starts in the queue repo, so
// nothing else ever looks at that wall). That reconcile was an unconditional
// `a.InstallCommitGuardHook(q)` — which made it the THIRD cross-home writer
// of a repo's SHARED commit wall, beside the launch (D3, ranger-base-lh9nj)
// and `posse gates install-hooks` (D4, ranger-base-hnujh). `q` is
// `queue_repo:`, read from whichever home ran the close, and the body is a
// function of THAT home's config: MEASURED 2026-11-06 (ranger-base-q0np6),
// same binary and same repo with only RHQ_HOME changed, 1897 lines against
// 1137. So any home that copied a `queue_repo:` line rewrote the owner's wall
// once per close, at the one write posse makes into the store of record.
//
// WHAT HOLDS NOW. The call site asks D3's rule first, of the same predicate
// the other two writers ask (ForeignCommitWallOwner → sharedCommitWallWrite),
// and installs only when it allows. Ownership stays D1 alone —
// `beads_visibility:` — because `queue_repo:` is one line any home can copy
// with the rest of a config, which is how a scratch home comes to name the
// real queue repo at all (D7a, rejected). A non-write leaves the owner's
// render in place and refuses THIS commit, naming the stamping home and both
// remedies. The projection is a full export, so the next close from the
// owning home commits what this one did not.
//
// THE THREE DONE-WHEN ARMS, and why each needs its own:
//
//	non-owner, foreign stamp   → wall untouched (bytes AND mtime), HEAD
//	                             still where it was, error names the home
//	owner, foreign stamp       → re-stamps and commits (D6: two owners is
//	                             visible misconfiguration, not a refusal)
//	absent / unstamped /       → still written (D3 cases 1, 4, 3) — the pin
//	same-home stamp              that keeps alternative (c) rejected: a
//	                             non-owner that refused every shared write
//	                             would stop arming the slot in any repo
//	                             nobody has stamped
//
// MUTATION-CHECKED 2026-11-07, two mutants:
//
//	the rule removed (back to the unconditional install)
//	  → the non-owner arm reds five times over: the commit succeeds, the
//	    wall's BYTES change, its MTIME changes, and HEAD moves. Nothing else
//	    moves, which is what says the other two arms are about the states
//	    this call still writes rather than about the rule
//	ownership dropped (`|| a.OwnsCommitWall(dir)` out of sharedCommitWallWrite)
//	  → the owning-home arm reds on the commit, carrying the refusal it
//	    should never have been handed. D6's case over the queue repo would
//	    go silent, and every close from the owning home would refuse the
//	    moment any other home had stamped that wall once

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── the fixture ─────────────────────────────────────────────────────────────

// qwoApp is qApp plus D1's answer about the queue repo, stated as config
// rather than left to the default. The non-owning case lists a DIFFERENT
// path rather than nothing at all: D1 is a question about ONE repo, and an
// empty `beads_visibility:` would satisfy an arm that read "does this home
// list anything".
func qwoApp(t *testing.T, repo string, owns bool) *App {
	t.Helper()
	a := qApp(t, repo)
	listed := repo
	if !owns {
		listed = filepath.Join(t.TempDir(), "somebody-elses")
	}
	write(t, a.ConfigPath, "queue_repo: "+repo+"\nbeads_visibility:\n  "+listed+": private\n")
	if got := a.OwnsCommitWall(repo); got != owns {
		t.Fatalf("fixture premise: OwnsCommitWall(%s) = %v, want %v", repo, got, owns)
	}
	return a
}

// qwoForeignWall installs the queue repo's commit guard from a SECOND home,
// so the slot holds that home's current render under that home's stamp. The
// home is returned because every assertion below is about naming it.
func qwoForeignWall(t *testing.T, repo string, launcher *App) (*App, string) {
	t.Helper()
	x := NewAppAt(t.TempDir())
	if samePath(x.Home, launcher.Home) {
		t.Fatalf("fixture premise: the foreign home and the committing home are one directory (%s)", x.Home)
	}
	slot, _, _, err := x.InstallCommitGuardHook(repo)
	if err != nil {
		t.Fatalf("install from the foreign home: %v", err)
	}
	// Measured, not assumed: the committing home reads this body as another
	// home's render, by its stamp and not by its length.
	if p := launcher.probeL3Hooks(repo, false); p.CommitGuardVerdict != l3OtherHome || p.CommitGuardHome != x.Home {
		t.Fatalf("fixture premise: verdict=%d home=%q, want l3OtherHome (%d) stamped %q — line: %q",
			p.CommitGuardVerdict, p.CommitGuardHome, l3OtherHome, x.Home, p.CommitGuardDegraded)
	}
	return x, slot
}

// qwoSlot is the slot's bytes and its mtime in one reading. The mtime is
// half the assertion rather than a nicety: a same-content rewrite is still a
// write — it is what two homes with identical configs do to each other — and
// bytes alone cannot see one.
func qwoSlot(t *testing.T, slot string) (string, time.Time) {
	t.Helper()
	b, err := os.ReadFile(slot)
	if err != nil {
		t.Fatalf("read %s: %v", slot, err)
	}
	fi, err := os.Stat(slot)
	if err != nil {
		t.Fatalf("stat %s: %v", slot, err)
	}
	return string(b), fi.ModTime()
}

// ─── D7: a non-owning home leaves the wall and refuses the commit ────────────

// THE DONE WHEN, first arm.
func TestQAAQueueCommitFromANonOwningHomeLeavesTheWallAndRefuses(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	a := qwoApp(t, repo, false)
	x, slot := qwoForeignWall(t, repo, a)
	beforeBody, beforeMtime := qwoSlot(t, slot)
	beforeHEAD := mustGit(t, repo, "rev-parse", "HEAD")
	// There must be something to commit, or a refusal is indistinguishable
	// from a projection that already matched its last commit.
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	// Errorf and not Fatalf, all the way down: a mutant that takes the rule
	// out makes this call SUCCEED, and the three facts below are what the
	// arm is actually about. Fatalf here would stop at "it committed" and
	// leave the wall and HEAD unmeasured in the one run that needed them.
	if err == nil {
		t.Errorf("the projection committed through another home's wall: %+v", c)
	}
	if c.SHA != "" {
		t.Errorf("a refusal reported a commit: %+v", c)
	}

	// The wall: bytes and mtime where they were.
	afterBody, afterMtime := qwoSlot(t, slot)
	if afterBody != beforeBody {
		t.Errorf("the queue commit rewrote another home's wall:\n--- before\n%s\n--- after\n%s", beforeBody, afterBody)
	}
	if !afterMtime.Equal(beforeMtime) {
		t.Errorf("the queue commit rewrote another home's wall with the bytes it already had: mtime %s -> %s", beforeMtime, afterMtime)
	}
	// And HEAD: the whole point of refusing is that nothing lands.
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != beforeHEAD {
		t.Errorf("HEAD moved in the queue repo: %s -> %s", beforeHEAD, after)
	}

	// The refusal has to be actionable, and D7 names exactly two remedies:
	// mark the repo here (which is also ADR 0015 §4 cutover step 5 and
	// G13's queue-unmarked row), or close from the home that rendered what
	// is there. The stamping home is in both the finding and the second
	// remedy, so it is asserted by name.
	if err == nil {
		return // already reported; there is no refusal to read
	}
	for _, want := range []string{
		AbbrevHome(x.Home),
		AbbrevHome(repo),
		"beads_visibility:",
		"ADR 0081",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must carry %q, got: %v", want, err)
		}
	}
	// Not "ours but stale", and not install-hooks' remedy either: a reader
	// told to reinstall from here would put the second writer straight back
	// (ADR 0081 D4 ships no --force for the same reason).
	for _, never := range []string{"stale", "install-hooks"} {
		if strings.Contains(err.Error(), never) {
			t.Errorf("the refusal reads as %q, which is the finding D2's stamp exists to replace: %v", never, err)
		}
	}
}

// D6 over the queue repo: ownership is what the rule turns on, so a home
// that DOES mark the repo re-stamps a foreign-stamped body and commits. Two
// owners is misconfiguration made visible, never a refusal — and if it were
// a refusal here, the owning home's closes would stop reaching git the
// moment any other home had stamped that wall once.
func TestQAAnOwningHomesQueueCommitReStampsAForeignWallAndCommits(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	a := qwoApp(t, repo, true)
	_, slot := qwoForeignWall(t, repo, a)
	beforeBody, _ := qwoSlot(t, slot)
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	if err != nil || c.SHA == "" {
		t.Fatalf("the owning home's close did not commit: (%+v, %v)", c, err)
	}
	afterBody, _ := qwoSlot(t, slot)
	if afterBody == beforeBody {
		t.Fatalf("the owning home did not re-stamp the wall it owns (D6):\n%s", afterBody)
	}
	if got := commitGuardHomeOf(afterBody); !samePath(got, a.Home) {
		t.Errorf("the re-stamped wall names home %q, want this home %q", got, a.Home)
	}
	if afterBody != a.CommitGuardHook(VisibilityPrivate, a.OpsPatternSet(), testIdentity(t, repo)...) {
		t.Errorf("the slot does not carry the render this home's config calls for:\n%s", afterBody)
	}
}

// D3 cases 1, 3 and 4, asked of THIS call site. These are the states in
// which the rule must still write, and they are the pin that keeps
// alternative (c) rejected: a non-owner that refused every shared write
// would stop arming the slot in every repo nobody has stamped — which,
// before this ADR's stamp existed, was the only signal a non-owner had.
//
// Every arm runs from a home that does NOT own the repo, because that is the
// only home for which the rule can answer no at all.
func TestQAAQueueCommitFromANonOwningHomeStillWritesAnUnclaimedSlot(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, a *App, repo, slot string)
		d3    string
	}{{
		// What `git init` leaves, and what a cutover step nobody performed
		// leaves: prepare-commit-msg.sample and no slot at all.
		name:  "nothing in the slot",
		setup: func(t *testing.T, a *App, repo, slot string) {},
		d3:    "case 1",
	}, {
		// This home's own current render, stamp and all.
		name: "the stamp names this home",
		setup: func(t *testing.T, a *App, repo, slot string) {
			if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
				t.Fatal(err)
			}
		},
		d3: "case 3",
	}, {
		// The one-shot migration: every body installed before ADR 0081
		// carries no stamp, so the first close per repo re-stamps it. An
		// unstamped body read as "somebody else's" would leave every queue
		// repo on the box refusing closes and naming a home nobody can name.
		name: "the body carries no stamp",
		setup: func(t *testing.T, a *App, repo, slot string) {
			if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
				t.Fatal(err)
			}
			body, _ := qwoSlot(t, slot)
			var kept []string
			for _, line := range strings.Split(body, "\n") {
				if !strings.HasPrefix(line, commitGuardHomeStampPrefix) {
					kept = append(kept, line)
				}
			}
			if len(kept) == len(strings.Split(body, "\n")) {
				t.Fatalf("fixture premise: no %q line to strip, so this arm measures a stamped body", commitGuardHomeStampPrefix)
			}
			write(t, slot, strings.Join(kept, "\n"))
			if err := os.Chmod(slot, 0o755); err != nil {
				t.Fatal(err)
			}
			if commitGuardHomeOf(strings.Join(kept, "\n")) != "" {
				t.Fatalf("fixture premise: the body still carries a stamp")
			}
		},
		d3: "case 4",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := qRepo(t)
			store := filepath.Join(repo, ".beads")
			a := qwoApp(t, repo, false)
			slot := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
			c.setup(t, a, repo, slot)
			closeBead(t, store, `{"id":"q-1","title":"closed"}`)

			got, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
			if err != nil || got.SHA == "" {
				t.Fatalf("D3 %s is a state this call still writes, and the close did not commit: (%+v, %v)", c.d3, got, err)
			}
			body, _ := qwoSlot(t, slot)
			if h := commitGuardHomeOf(body); !samePath(h, a.Home) {
				t.Errorf("D3 %s: the slot is stamped %q, want this home %q — the write did not happen", c.d3, h, a.Home)
			}
		})
	}
}
