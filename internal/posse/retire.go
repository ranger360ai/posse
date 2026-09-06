package posse

// ADR 0058 — when a session worktree may be retired with nobody watching.
//
// The trees stand because `RemoveSessionTree` runs inside a kill and a kill
// runs only while herdr still lists the workspace. A herdr restart, a pane
// closed by hand, a `posse kill` that lost the launcher-lock race or a crash
// takes the workspace without the landing, and from then on the tree has no
// path to removal at all: the landing sweep lands it and `continue`s in
// silence on every later pass. MEASURED 2026-09-05 in ~/src/posse: 70 trees
// standing, 8 with a live session, and 38 dead, clean, closed and fully
// landed — 54% of the board, 36 of them landed by plain fast-forward, with
// nothing in posse that would ever take them. Every refusal on those paths
// ends "a human can retire the tree"; nobody was ever given that
// instruction and nobody has followed it.
//
// So this is the predicate that lets a pass do it instead, and the whole of
// the argument is WHAT EVIDENCE IS ENOUGH. The field's rule for reclaiming a
// resource another actor may still hold (safe-reclamation.md; this shop met
// it in ADR 0011 §2 for the session meta) is proof of death at reclaim time
// plus a grace covering the actors the scan cannot see — never "it looked
// dead in the listing". Four facts, and every one that cannot be answered
// fails CLOSED, because the costs are not symmetric: a wrong keep is 8.5M of
// disk and a line in a listing, and a wrong retire is somebody's only copy.
//
//  1. the bead is closed, read fresh from the store of record;
//  2. nothing would be lost — RemoveSessionTree's own unforced refusal, asked
//     as a question (treeHolds). Not a new predicate written to agree with
//     it: the one the reap already asks, over the same two records and the
//     same tips, held to the refusal's own answer by
//     TestRetireGuardsSeeADetachedTreesWork;
//  3. the session is proven gone, on ADR 0011 §2's own evidence and not a
//     liveness rule coined here (sessionGone);
//  4. the tree has been quiet for `retire_tree_after:`.
//
// WHY FACT 4 IS DENOMINATED IN TREE WRITES and not in time since the close:
// it exists for the one actor the board cannot show — a process in the tree
// whose workspace detection blinked, or the operator's own shell — and for
// those the last write is the only evidence there is. A `git status` in the
// tree resets it, which is the fail-safe direction.
//
// WHY IT IS READ FIRST, before facts 2 and 3. Reading a git tree writes to
// it. MEASURED on this box (2026-09-05, macOS/APFS, git 2.51): `git status`
// — which is what fact 2's dirty check is — rewrites the index whenever the
// stat cache it holds is not clean, which is the state of every tree just
// committed in and of every tree whose index is not newer than the entries
// it records. So the grace is read BEFORE this pass touches the tree, or the
// pass measures its own reading and keeps the tree for it, silently, on
// every pass forever. It is the same reason the re-read under the launcher
// lock below is facts 2 and 3 ONLY, and retiresweep_qa_test.go pins both
// halves over a tree whose index is deliberately left stale.

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRetireTreeAfter is how long a session tree must have been written
// to by NOBODY before an unattended pass may take it. Like the two reap
// graces it is a POLICY DIAL and not a measurement of anything — no
// tree-write cadence was read off the fleet — and like them it is config
// (`retire_tree_after:`), where `off`/`never` turns the retire back into the
// permanent skip it was before ADR 0058.
//
// An hour, on the same argument the unpointed reap grace makes for its own:
// what it protects against is not a conversation's gaps (fact 3 already
// keeps every tree whose session herdr can see) but a process posse never
// knew about — and the only ones that reach a tree whose bead is closed and
// whose work is on the base are an operator's own shell and a stray child.
const DefaultRetireTreeAfter = time.Hour

// retireVerdict is ADR 0058 D1's answer about one tree.
//
// `quiet` is the difference between the two kinds of keep, and it is a
// property of the FACT rather than of the caller: a tree inside its grace is
// kept for a reason that will stop being true by itself, and 36 of those on
// one board is noise nobody reads. Every other keep is a standing condition
// and is said on every pass it holds (kftx's rule).
type retireVerdict struct {
	retire bool
	why    string // why it was safe, or which fact refused
	quiet  bool   // this keep is not worth a line: it is transient, or the dial is off
}

// retirable is D1 over one tree: all four facts, in the order that makes the
// cheap and self-defeating ones come first (see the header on fact 4).
//
// `status` is the bead's status read FRESH by the caller, which both callers
// already hold — the sweep asks the store of record for every closed tree it
// visits, and `posse worktrees --retire` asks it per tree. Nothing here
// caches it: ADR 0011's rule is that a reclaim never acts on a status
// somebody read earlier.
func retirable(t *SessionTree, status string, hb *HerdrBackend, grace time.Duration) retireVerdict {
	if grace <= 0 {
		// `retire_tree_after: off` is the operator saying they want the
		// trees. Said once by the dial itself, not once per tree per pass.
		return retireVerdict{why: "`retire_tree_after:` is off", quiet: true}
	}
	if status != "closed" {
		// Not this record's population at all: an open bead's tree is a
		// seat, and a relaunch reuses it (ADR 0058 D4).
		return retireVerdict{why: fmt.Sprintf("its bead is %s, not closed", statusWord(status))}
	}
	quiet, ok := treeQuietFor(t)
	if !ok {
		return retireVerdict{why: fmt.Sprintf("when %s was last written cannot be read", AbbrevHome(t.Path))}
	}
	if quiet < grace {
		return retireVerdict{
			why:   fmt.Sprintf("%s was written %s ago, inside the %s grace", AbbrevHome(t.Path), quiet.Round(time.Second), grace),
			quiet: true,
		}
	}
	if why := retireHeldOrAlive(t, hb); why != "" {
		return retireVerdict{why: why}
	}
	return retireVerdict{retire: true, why: fmt.Sprintf(
		"its bead is closed, nothing here is unlanded, herdr proves its session gone, and nothing has written to %s in %s",
		AbbrevHome(t.Path), grace)}
}

// retireHeldOrAlive is facts 2 and 3 — the two the sweep RE-READS with the
// launcher lock held, and the reason it re-reads only these two.
//
// They are the facts about somebody else's store: what git holds, and what
// herdr says is alive. Evidence for either read before the lock is a fact
// about the instant it was read and not about the instant the removal lands
// (ADR 0011 §2's reclaim rule, rangerhq-3a5t) — a create for this session's
// name, or a commit in the tree, can arrive in between. Facts 1 and 4 are
// not re-read: the bead is the store's own answer about a bead nothing in
// this window reopens, and re-reading the grace HERE would read back the
// index refresh that fact 2's own `git status` may just have written.
//
// "" when both hold, and the refusal itself when either does not.
func retireHeldOrAlive(t *SessionTree, hb *HerdrBackend) string {
	if held := treeHolds(t); held != "" {
		return held
	}
	if hb == nil {
		// No herdr to ask is not "no session": the unanswerable question
		// fails closed like every other one here.
		return "nothing can be asked whether its session is still alive"
	}
	if gone, why := hb.sessionGone(SessionOfBranch(t.Branch)); !gone {
		return why
	}
	return ""
}

// statusWord renders a bead status for a sentence, including the one bd can
// hand back that is not a status at all.
func statusWord(status string) string {
	if strings.TrimSpace(status) == "" {
		return "unrecorded"
	}
	return status
}

// treeQuietFor is how long NOTHING has written to this session's tree, and
// whether that could be read at all. false is unanswerable, which fact 4
// treats like every other unanswerable question here.
func treeQuietFor(t *SessionTree) (time.Duration, bool) {
	last, ok := lastTreeWrite(t)
	if !ok {
		return 0, false
	}
	// A write dated in the FUTURE (a clock that moved, an unpacked archive)
	// is not "quiet for a negative hour": it reads as inside the grace,
	// which is the direction that keeps the tree.
	return time.Since(last), true
}

// lastTreeWrite is when anything last wrote to this session's tree: the
// newest mtime among the FILES of its own git dir — `.git/worktrees/<session>/`,
// where the index, HEAD, ORIG_HEAD and the reflogs live — and the commit
// dates of the tips a retire would drop the last reference to.
//
// THE FILES, AND NOT THE DIRECTORY. MEASURED on this box (2026-09-05,
// macOS/APFS, git 2.51): five consecutive `git status` runs in a quiescent
// worktree moved the git DIRECTORY's own mtime every single time, while the
// index file moved only on the first one and on the first status after a
// commit — git creates and renames its `index.lock` in there, and creating an
// entry in a directory moves the directory whether or not any file in it
// changed. A reading taken off the directory would therefore be reset by the
// sweep's own `git status`, and no tree on the board would ever be quiet
// enough to retire. The CHECKOUT directory is not the reading either, for
// the mirror-image reason: it does not move on a commit at all.
//
// The tips are asked because a commit is a write the git dir does not always
// carry: a commit made in the tree writes its reflog here, but one made in
// the shared checkout onto this branch, or a `branch -f` that moved it, does
// not touch this directory at all. removalTips is the same list fact 2 asks
// about, so what the grace covers and what the refusal protects cannot drift.
func lastTreeWrite(t *SessionTree) (time.Time, bool) {
	gd, err := git(t.Path, "rev-parse", "--absolute-git-dir")
	if err != nil || gd == "" {
		return time.Time{}, false
	}
	newest := time.Time{}
	files := 0
	filepath.WalkDir(gd, func(_ string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		fi, err := e.Info()
		if err != nil {
			return nil
		}
		files++
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
		return nil
	})
	if files == 0 {
		// A git dir with no readable file in it is not a quiet tree; it is
		// a question that could not be asked.
		return time.Time{}, false
	}
	for _, tip := range removalTips(t) {
		if ts, ok := commitTime(t.Repo, tip.ref); ok && ts.After(newest) {
			newest = ts
		}
	}
	return newest, true
}

// RetireAsk is what a surface OUTSIDE the landing sweep needs in order to
// ask D1 about a tree: the store of record, herdr, and the dial. The sweep
// carries all three on its Dispatcher already; `posse worktrees` and
// `posse worktrees --retire` carry none of them, and ADR 0058 D3's whole
// point is that both ask the SAME predicate rather than a cheaper lookalike
// written to agree with it.
//
// It is a value and not a package global because the two surfaces read the
// store DIFFERENTLY on purpose, and the difference is the ADR's own rule:
//
//   - the LISTING is a report, and may read the store once (`reported`);
//   - `--retire` is an ACT, and reads fact 1 per tree at the instant it acts
//     on that tree (`fresh`), because ADR 0011's reclaim rule is that
//     nothing destroys work on a status somebody read earlier.
//
// One command's, and not shared: `reported` fills its cache lazily, so a
// RetireAsk handed to two goroutines would race. Both callers walk one tree
// at a time and neither has a reason not to.
type RetireAsk struct {
	bd    Bd
	hb    *HerdrBackend
	grace time.Duration

	// listed is one `bd list --all` per repo, and it is the LISTING's
	// answer only. MEASURED 2026-09-06 on this box against the fleet's own
	// store (~1300 issues, `bd --no-daemon`): `bd show` 0.59s, `bd list
	// --all --json --limit 0` 1.5s for the whole store. Over the 70 trees
	// of ADR 0058's census the per-tree read would cost `posse worktrees`
	// 41 seconds and this one 1.5 — and a listing nobody waits for is a
	// listing nobody runs, which is how the sentence this record is
	// replacing came to be read by no one.
	listed map[string]beadStatuses
}

// beadStatuses is one repo's answer, including the case where there was
// none: an unreadable store is not "no such bead", and the two must not
// render as the same keep.
type beadStatuses struct {
	byID map[string]string
	err  error
}

// NewRetireAsk reads the dial once and holds it, the way the sweep reads it
// once per pass: `retire_tree_after:` is policy, and a command that read it
// per tree could answer two trees in one run under two different rules.
func NewRetireAsk(a *App, bd Bd, hb *HerdrBackend, errw io.Writer) *RetireAsk {
	return &RetireAsk{
		bd:    bd,
		hb:    hb,
		grace: a.graceAfter("retire_tree_after", DefaultRetireTreeAfter, errw),
	}
}

// fresh is fact 1 read at the instant the caller is about to act on THIS
// tree — one `bd show`, uncached, the same read the sweep makes.
func (r *RetireAsk) fresh(t *SessionTree) (string, error) {
	is, err := r.bd.Show(t.Repo, t.Bead)
	if err != nil {
		return "", err
	}
	return is.Status, nil
}

// reported is fact 1 for a surface that only REPORTS: one listing per repo,
// held for the run. A bead the listing does not carry reads as the empty
// status, which statusWord renders "unrecorded" and fact 1 keeps — the same
// direction every unanswerable question here fails in.
func (r *RetireAsk) reported(t *SessionTree) (string, error) {
	if r.listed == nil {
		r.listed = map[string]beadStatuses{}
	}
	got, ok := r.listed[t.Repo]
	if !ok {
		got = beadStatuses{byID: map[string]string{}}
		var all []BdIssue
		if all, got.err = r.bd.ListAll(t.Repo); got.err == nil {
			for _, is := range all {
				got.byID[is.ID] = is.Status
			}
		}
		r.listed[t.Repo] = got
	}
	return got.byID[t.Bead], got.err
}

// noRecordKeeps is ADR 0006's rule as ADR 0058 D4 restates it for a tree no
// bead record accounts for, and it is the sentence D3 leaves UNCHANGED: no
// record, no act. Such a tree is not this predicate's population at all, so
// the listing says that rather than running the four facts over it and
// reporting whichever one happens to refuse first.
const noRecordKeeps = "no record says which bead, so nothing unattended may retire it (ADR 0006) — retiring it stays a human's"

// clause is ADR 0058 D3's half of the listing: what will happen to this
// tree. It replaces "a human can retire the tree", which was true of nobody
// — MEASURED 2026-09-05, 38 dead landed trees standing and no human had ever
// run the command that sentence was pointing at.
//
// Three shapes and they are the ADR's: a tree the predicate passes says the
// next pass takes it; one it fails names the fact that failed, in the
// predicate's own words so that the listing and the sweep cannot drift; and
// a tree no record accounts for gets ADR 0006's sentence instead.
//
// A nil ask is a caller with no store to put the question to, which is the
// same unanswerable-question shape as the rest of this file and fails the
// same way: the tree is kept and the listing says why.
func (r *RetireAsk) clause(t *SessionTree) string {
	if r == nil {
		return "kept: nothing here can ask whether it is retirable"
	}
	if t.Bead == "" {
		return "kept: " + noRecordKeeps
	}
	status, err := r.reported(t)
	if err != nil {
		return fmt.Sprintf("kept: bd could not say whether %s is closed (%v)", t.Bead, err)
	}
	if v := retirable(t, status, r.hb, r.grace); !v.retire {
		return "kept: " + v.why
	}
	return "retirable — the next pass takes it"
}

// RetireSessionTrees is ADR 0058 D3: the operator's run of the sweep's own
// predicate, in `--land`'s shape — every tree, one blocking launcher lock
// for the whole run, one line per tree.
//
// It takes NO --force, and cmd/posse refuses `--retire --force` as an
// unknown flag rather than accepting and ignoring it. force is
// RemoveSessionTree's existing override, it stands down the one refusal
// that exists to say no while something would be lost, and it stays the
// two-command hand recipe those refusals print. A flag that skips that
// guard over every tree on the board in one keystroke is the one thing this
// record is not adding.
//
// THE PREDICATE IS ASKED ONCE HERE, and that is not the sweep's two
// readings weakened — it is the same rule met differently. The sweep reads
// facts 2 and 3 cheaply outside the lock so the common tree costs no lock at
// all, and then again inside it, because evidence read before the lock is a
// fact about the instant it was read. This command takes the lock BEFORE it
// reads anything (lockLaunches below, blocking — a person ran it and waiting
// is the honest thing for it to do), so its single reading is already the
// one taken with the lock held.
//
// EVERY KEEP PRINTS, including every keep an unattended pass says nothing
// about: a tree inside its grace, the dial turned off, an open bead's seat
// the sweep skips before the predicate is asked, and a tree ADR 0006 leaves
// alone. `quiet` is a property of a PASS's noise — 36 trees inside their
// grace on every pass forever is how a board stops being read — and this is
// a command a human just ran and is reading the output of. A tree it says
// nothing about is a tree they have to ask about twice.
func RetireSessionTrees(w io.Writer, a *App, r *RetireAsk, dirs []string) error {
	trees, err := SessionTreesIn(dirs)
	if err != nil {
		return err
	}
	if len(trees) == 0 {
		fmt.Fprintln(w, "no session worktrees")
		return nil
	}
	lock, err := lockLaunches(a, w)
	if err != nil {
		return err
	}
	defer lock.Release()
	for _, t := range trees {
		id := t.Bead
		if id == "" {
			// ADR 0006, unchanged: no record accounts for this tree, so
			// nothing here may act on it — and `--force` is not the answer
			// because there is no `--force` (see the header).
			fmt.Fprintf(w, "◑ %-14s %s kept: %s\n", "(no bead)", t.Branch, noRecordKeeps)
			continue
		}
		status, err := r.fresh(t)
		if err != nil {
			fmt.Fprintf(w, "◑ %-14s %s kept: bd could not say whether it is closed (%v)\n", id, t.Branch, err)
			continue
		}
		v := retirable(t, status, r.hb, r.grace)
		if !v.retire {
			fmt.Fprintf(w, "◑ %-14s %s kept: %s\n", id, t.Branch, v.why)
			continue
		}
		if err := RemoveSessionTree(t, false); err != nil {
			// The predicate said yes and the destroy still refused, which is
			// the disagreement worth printing loudly: RemoveSessionTree is
			// fact 2's own author and its answer is the one that governs.
			fmt.Fprintf(w, "⚠ %-14s %s not retired: %v\n", id, t.Branch, err)
			continue
		}
		fmt.Fprintf(w, "⌫ %-14s %s retired: %s\n", id, t.Branch, v.why)
	}
	return nil
}
