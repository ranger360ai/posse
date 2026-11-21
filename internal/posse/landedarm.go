package posse

// A LAUNCHER THAT ARMS READS THE TIP IT INHERITS (ADR 0088 D6.6, from
// ranger-base-gihmm). landedcheck.go judges the tip a LANDING scheduled and
// landedinterrupt.go is D6.5's answer to who stopped the judge. This file is
// the other half of that amendment, and it is about the tip NOTHING
// scheduled.
//
// THE DEFECT. `once()` runs only when a landing schedules it, so a tip that
// moved while no loop was running is never read: the new loop does not judge
// main until something lands. MEASURED 2026-11-21 (ranger-base-gihmm): after
// the 00:08:53 restart, main `16c844cb` / tree `2722c3fb` had been unjudged
// for about four minutes and would have stayed that way until the next
// landing. A restart is also not bad luck — ranger-base-rrgnc makes one part
// of every install, and installs follow landings — so the loop that comes up
// is routinely the one holding a tip no landing of its own will ever name.
// A stop makes the same hole from the other side: D6.5 DROPS the pending set
// when the launcher is stopping, and the line it prints for each dropped
// repo ends "the next launcher reads the tip". This is that reader.
//
// SO A LOOP THAT ARMS SCHEDULES ONE CHECK PER REPO IT LANDS INTO, at the
// base it lands to. Both halves are derived from what this dispatcher
// already has, which is why the amendment persists nothing new:
//   - THE REPOS are `BeadsDirs` through `mainCheckoutsOf`, which is the
//     enumeration every landing path in this package already makes
//     (landClosedTrees' own first two lines) — so a repo this sweep names is
//     exactly a repo a landing could name, and the dirFilter narrows both
//     the same way.
//   - THE BASE is `repoBranch(repo)`, the branch the main checkout has
//     checked out. That is where PlanSessionTree cuts a tree from and where
//     MergeSessionWork fast-forwards it back to, so it is "the base it lands
//     to" for every tree this loop could land today.
//
// WHAT IT COSTS IS D4's CACHE, which is the whole reason this is affordable
// at arm: a tip already on record costs one `git config` read and `once`
// returns, and a repo that ships no judge returns one `cat-file` later. A
// tip that is NOT on record is judged about 2.5 minutes after the loop arms
// instead of at the next landing.
//
// ONLY A LOOP. A one-shot `posse dispatch` has no loop to arm and its own
// landings schedule their own checks, so Watch is the one caller (ADR 0088
// D6.6's last sentence). Deleting that one call restores the behaviour this
// amendment found: judged at the next landing.

// scheduleInheritedTips is that sweep, and it is `Schedule` and nothing
// else. The run loop reads each repo's tip for itself when it gets there
// (the pending set holds a BRANCH, not a sha), so this costs one git call
// per repo on the arming goroutine and judges on another.
//
// EVERY REFUSAL IS A SKIP AND NOT A LINE, because each of them is a state in
// which no landing could have happened either, so there is no inherited tip
// to be silent about: a configured dir that is not a git repo is dropped by
// mainCheckoutsOf, and a repo whose HEAD is DETACHED has no base at all —
// `repoBranch` answers "" and `Schedule` drops it, which is the same answer
// PlanSessionTree gives such a repo when it refuses to cut a tree in it.
//
// A DRY RUN SCHEDULES NOTHING, on the rule the pass's own filing sweeps keep
// (verify-after, the CI watch, the parked-work note): a judge writes D4's
// record, may file D5's bead and spends minutes of box on a caged `make`,
// and all three of those are acting. It is not a hole a dry run could have
// reached by accident either — a `--dry-run` landing sweep prints "would
// land" and never schedules a check — so this is where that exclusion has to
// be said, and it is the one skip here that is not about a repo.
func (d *Dispatcher) scheduleInheritedTips(dirFilter string) {
	if d.DryRun {
		return
	}
	dirs := d.App.BeadsDirs()
	if dirFilter != "" {
		dirs = []string{dirFilter}
	}
	for _, repo := range mainCheckoutsOf(dirs) {
		d.landedChecks().Schedule(repo, repoBranch(repo))
	}
}
