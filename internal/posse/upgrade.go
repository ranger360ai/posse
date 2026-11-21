package posse

// `posse upgrade [<sha>]` — ADR 0092 D1-D3, and the rollback-line half of D4.
//
// WHY A VERB OF THE INSTALLED BINARY AND NOT A SCRIPT (D1). The operator
// ruled on 2026-11-13 that `make install` is delegated to one persona under
// five guardrails, because 99 commits of security fixes sat uninstalled on
// 2026-11-12/13 and every re-verify stalled behind them. A persona-run
// installer has to be code the persona cannot edit on its way in: a script
// under `scripts/`, allowed by path, is matched in the SEAT'S OWN WORKTREE,
// which the seat may rewrite, so its guardrails can be edited out by the
// seat they bind. This file is compiled into the binary that is already
// installed, and that binary got there through this same gate or by the
// operator's hand. `make install` stays denied for everyone, the delegate
// included.
//
// THE ORDER IS THE GUARDRAIL (D3), and each step refuses before the next
// one runs:
//
//	1. target   `<sha>` must be an ancestor of refs/remotes/origin/main and
//	            a descendant of the installed binary's own stamp. The verb
//	            never fetches — the remote-tracking ref moved because the
//	            coordinator pushed, and that push is the whole of the
//	            evidence that `posse-land-check` and the vuln-private scan
//	            ran. Nothing here can see those; being on origin/main is the
//	            record that the coordinator says they did.
//	2. test     `make test` in a private, re-hashed copy of that sha (a fetch
//	            into a new repo, never a worktree of the shared store — see
//	            upgradePrivateCopy) — all three arms, suite slots and all.
//	            Red, or no answer, refuses.
//	3. build    scripts/clean-build.sh in that same copy, so the binary
//	            is built from the tree the tests passed on.
//	4. keep     the live binary is copied to <target>.prev. One level of
//	            rollback, as the operator asked.
//	5. swap     rename(2) over the target, never install(1) — see below.
//	6. epilogue `make install`'s own two checks, from that copy.
//
// WHY rename(2) AND NOT install(1) (D3.5). install(1) unlinks the target and
// then writes it, so a launch that execs the binary inside that window gets
// a missing or a short file. ADR 0092 recorded that as ASSUMED from GNU
// coreutils behaviour. MEASURED 2026-11-13 on this box (GNU coreutils 9.12,
// linux 7.2.15, on TMPFS, 64 MiB over 64 MiB, a reader stat-ing the target
// in a tight loop, three trials): install(1) left the target absent for
// 1,791-2,473 of ~84,000 observations and short for 12,606-13,384 of them,
// over a 58 ms window, with ~10,600-11,253 distinct intermediate sizes. The
// same probe over write-beside-then-rename(2) saw ENOENT 0, short 0, one
// distinct size, every trial. Tmpfs is the FASTEST surface this box has, so
// 58 ms is a floor and a real install to a disk-backed ~/.local/bin has a
// wider window, not a narrower one. The rename closes it either way.
// docs/notes.d/ranger-base-zhw0h.md has the probe.
//
// WHAT IS NOT HERE. The bd argv gate copy arm is D5 and lives in
// upgradegate.go (ranger-base-41l0r), reached from this file's epilogue,
// which is also where D6's gate word now comes from. D4 — the status/gates
// snapshot taken before the swap,
// the diff taken after it through the NEW binary, and the automatic rollback
// on a new red — landed under ranger-base-v87a1 and lives in
// upgradeverify.go, which also carries the partition of `posse status`
// condition codes that decides what counts as a new red. This file prints
// the rollback command before it swaps, which is the half of D4 that costs
// nothing and is worth everything if the next step goes wrong, and calls
// into that one between the swap and the epilogue.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// UpgradeDelegateKey is the config key naming the one persona that may run
// this verb (ADR 0092 D2). It is config and not a PID line for the reason
// ADR 0031 and promote.go give: one check, in the verb, read from promoted
// config, so no other PID needs a new deny. Its exit hatch is to delete the
// key, which puts the verb back in the operator's hands alone.
const UpgradeDelegateKey = "install_delegate"

// upgradePrivateRootName is the ONE directory name this verb stages its
// private copy under, and the reason it is a constant rather than a
// `mktemp -d` straight into the temp root (ranger-base-ua625 finding 2, from
// the verify at ranger-base-pxprm).
//
// WHAT WAS WRONG. upgradePrivateCopy's own comment used to say "the new
// repository's store is under this run's mktemp -d, which no seat is
// granted". At the bwrap mechanism that is true and stated where it is true
// (bwrap.go step 1c: the seat's /tmp is a private bind, "so no seat writes
// where uncaged builds stage what they execute"). At the SEATBELT mechanism
// it was false both ways round: seatbeltTempGrants puts $TMPDIR, /private/tmp
// and /tmp in the writable set of EVERY rendered profile — "the same three,
// whatever its PID, its cwd or its persona says" — and os.MkdirTemp("", …)
// resolves to $TMPDIR else /tmp, so the copy is inside grant 1 or grant 3
// and there is no setting of TMPDIR that puts it outside the three. Seats
// and the uncaged install_delegate run as one uid, so 0700 separates
// nothing. MEASURED 2026-11-14 through the production functions:
//
//	seatbeltTempGrants("darwin", "/var/folders/ab/cd1234/T")
//	  = [/var/folders/ab/cd1234/T  /private/tmp  /tmp]
//	os.MkdirTemp("", "posse-upgrade.") with TMPDIR unset
//	  -> /tmp/posse-upgrade.453727933, mode 0700, inside the grant /tmp
//
// A seat could therefore write the private copy's working tree and its
// object store for the whole of the `make test` window — more than twenty
// minutes — and that suite runs from the copy, UNCAGED, as this verb's
// caller. What ranger-base-la61c already closed is the install: clean-build
// re-fetches from the copy and index-pack re-hashes. What was open is the
// execution.
//
// WHY A STABLE NAME AND NOT A DIFFERENT ROOT. The bead's own suggestion was
// a copy under posse's state dir, "a root no seat is granted". That moves
// the copy out of TMPDIR, and this tree's suite MOVES WITH TMPDIR in three
// ways that all read as "the commit is red" (MEASURED 2026-11-13 at 983ae5d,
// ranger-base-6ca29; upgradeRunPlan quotes the matrix) — one of them is
// exactly "under $HOME", where AbbrevHome renders the path as `~/…` and
// every test asserting its own absolute temp path reds. So the copy stays
// where the suite was measured green and the GRANT is narrowed instead: one
// fixed child of the temp root, denied in every rendered profile by name
// (SeatbeltCarveOut, upgradePrivateDenied), whether or not an upgrade is
// running. A per-run random name could not be denied at all — a seat's
// profile is rendered at ITS launch, which is before this verb picks a name.
//
// WHICH TEMP ROOT, exactly, is upgradePrivateRoot's question and no longer
// os.TempDir()'s answer: the deny is rendered for the LAUNCH's TMPDIR and
// the root was read from the VERB's, and ranger-base-18brx measured the
// copy landing inside a blanket grant under a name no deny covered when
// those two disagreed.
const upgradePrivateRootName = "posse-upgrade"

// upgradePrivateRoot is the directory this run's private copy is made
// inside: one fixed name under the temp root EVERY rendered profile denies,
// and not under the one this process happens to read out of its own
// environment (ranger-base-18brx, finding 1 of the verify at
// ranger-base-m5izv).
//
// WHAT WAS WRONG. This used to be `filepath.Join(os.TempDir(), …)` — "the
// same root os.MkdirTemp("", …) would have used" — and said that os.TempDir
// is MkdirTemp's own rule, "so the two cannot disagree about which temp root
// this is". The two that have to agree are not those two. os.TempDir() is
// read in the process that RUNS this verb; upgradePrivateDenied is rendered
// in the process that LAUNCHED a seat, minutes or days earlier, and it names
// one child per blanket grant ROOT rather than that name wherever it appears
// under a grant. So where the two TMPDIRs disagree and the verb's resolves
// STRICTLY INSIDE one of the launch's blanket grants, the copy sits in a
// seat's write grant under a name no deny covers — ranger-base-ua625
// finding 2 again, reopened by a variable. MEASURED 2026-11-21 on this box
// (linux 7.2.15-1-omarchy), through these functions:
//
//	launch TMPDIR  verb TMPDIR           verb's staging root           reached  denied
//	""             ""                    /tmp/posse-upgrade            yes      yes
//	""             /tmp/op               /tmp/op/posse-upgrade         yes      NO    ← hole
//	/tmp/seat      /tmp/op               /tmp/op/posse-upgrade         yes      NO    ← hole
//	""             /var/folders/ab/cd/T  /var/folders/…/posse-upgrade  no       no
//
// The last row is the fail-safe direction and is why the hole was narrow: a
// verb TMPDIR outside every blanket grant is outside every seat's reach as
// well, so only a verb TMPDIR strictly INSIDE one of them is in the shape.
// docs/notes.d/ranger-base-18brx.md has the reading and the mutation.
//
// SO THE ROOT READS THE DENY SET ITSELF, and the two cannot disagree because
// they are one expression. The member it takes is the last of
// upgradePrivateDenied for an EMPTY tmpdir — which is the deny over the
// grants seatbeltTempGrants names whatever its arguments, and whose last
// entry is the `/tmp` that function appends unconditionally, for every GOOS.
// Three things follow, and all three are why this is the arm taken:
//
//   - every rendered profile denies this name, whatever TMPDIR the process
//     that rendered it read, because the grant it hangs off is in every set
//     that function returns. A reorder there moves this root to another
//     member of the same set, which is still denied in every profile;
//   - at bwrap the seat's /tmp is a private bind (bwrap.go 1c), so on Linux
//     the staging root is not in a seat's view at all — where a root under
//     some other TMPDIR need not have been;
//   - `/tmp` is the one temp root this tree's suite was MEASURED green in
//     (upgradeRunPlan's matrix, ranger-base-6ca29). "Under $HOME" and a
//     quota'd tmpfs both read as "the commit is red", and a verb run with
//     either as its TMPDIR used to stage the copy — and so run the suite —
//     there.
//
// WHAT IT DOES NOT CLOSE, said here because the close it amends said it too:
// a seat launched BEFORE a deny existed carries no deny for it, because a
// profile is fixed at its own launch. That is every deny's property, it
// cannot be closed from the render, and the remedy is the one it always was
// — a seat launched after the swap carries the new profile.
func upgradePrivateRoot() string {
	return upgradePrivateRootFor(runtime.GOOS)
}

// upgradePrivateRootFor is that root for a GOOS, parameterized for
// seatbeltTempGrants' own reason — a darwin pin must be able to read the
// darwin answer on this box. "" when the grants are stubbed empty, which
// upgradePrivateCopy refuses rather than guessing a root nothing denies.
func upgradePrivateRootFor(goos string) string {
	denied := upgradePrivateDenied(goos, "")
	if len(denied) == 0 {
		return ""
	}
	return denied[len(denied)-1]
}

// upgradePrivateRootDenied reports whether root is a name every rendered
// profile denies whatever TMPDIR its renderer read: a member of the deny set
// for an EMPTY tmpdir, which is the part of that set no launch can vary.
//
// Asked at run time by upgradePrivateCopy although upgradePrivateRoot
// derives from this same call, because the cost of asking is one string
// compare and the cost of it not holding is an uncaged `make test` over a
// tree a seat may write for the twenty minutes it runs. The shape that did
// not hold was `filepath.Join(os.TempDir(), …)`, which reads like the right
// answer and refuses nothing (ranger-base-18brx).
func upgradePrivateRootDenied(goos, root string) bool {
	if root == "" {
		return false
	}
	for _, d := range upgradePrivateDenied(goos, "") {
		if absResolve(d) == absResolve(root) {
			return true
		}
	}
	return false
}

// upgradePrivateDenied names the staging roots a rendered profile must deny,
// for a GOOS and a TMPDIR: one under each of the blanket temp grants, because
// the grant is what has to be narrowed and a launch's TMPDIR need not be the
// upgrade run's. Parameterized for seatbeltTempGrants' reason — a darwin pin
// must be able to read the darwin answer on this box — and nil when that var
// is stubbed empty, which is correct: no temp grant, nothing to narrow.
func upgradePrivateDenied(goos, tmpdir string) []string {
	var out []string
	for _, d := range seatbeltTempGrants(goos, tmpdir) {
		out = append(out, filepath.Join(d, upgradePrivateRootName))
	}
	return out
}

// upgradeDefaultRef is the target when the operator names no sha. A
// remote-tracking ref and never a local branch: `main` in the source
// checkout carries crew landings the coordinator has not pushed, and
// installing those is the guardrail-1 breach `make install` commits by
// construction (ADR 0092, Context).
const upgradeDefaultRef = "refs/remotes/origin/main"

// UpgradeOpts is what `posse upgrade` was asked for.
type UpgradeOpts struct {
	// Sha is the commit to install, "" for the tip of upgradeDefaultRef.
	Sha string

	// Classes is `--classes`: print this binary's own upgrade partition and
	// install nothing (ADR 0092 D4 rule 4). Not a mode of the install but
	// an alternative to it — see cmdUpgrade, which answers it before the
	// verb's first reading.
	Classes bool
}

// UpgradeDelegate is config `install_delegate:`, trimmed, "" when unset.
func (a *App) UpgradeDelegate() string {
	return strings.TrimSpace(a.CfgGet(UpgradeDelegateKey, ""))
}

// upgradeFence is D2, as a pure function of the two strings it reads, so the
// four arms an operator cares about — no key, wrong persona, right persona,
// no persona — are pinnable without a config file or a process environment.
//
// An ABSENT key refuses every persona and not merely an unnamed one: a verb
// that installed the fleet's binary for anybody the day somebody deleted a
// config line is a fence whose failure mode is wide open. No RHQ_PERSONA is
// the operator, who may always run it.
func upgradeFence(persona, delegate string) error {
	if persona == "" {
		return nil
	}
	if delegate == "" {
		return Die("posse upgrade: no %s: is configured, so this verb is the operator's alone (ADR 0092 D2) — refusing under %s=%s", UpgradeDelegateKey, EnvPersona, persona)
	}
	if delegate != persona {
		return Die("posse upgrade is %s's (config %s:, ADR 0092 D2): refusing under %s=%s", delegate, UpgradeDelegateKey, EnvPersona, persona)
	}
	return nil
}

// UpgradeLockPath is the one upgrade lock of this box (ADR 0092 D3). Beside
// the suite slots, in the same cache dir and for the same reason: the thing
// being serialized is box-wide, so the lock cannot live in any one session's
// tree. "" when there is no home to hang it off, which lockUpgrade refuses.
func UpgradeLockPath() string {
	d := posseCacheDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "upgrade.lock")
}

// upgradeLock is a held upgrade lock. Same shape as the launcher lock
// (launchlock.go): flock(2), released by closing the fd, which is also what
// process death does — so forgetting it is a leak and never a stale lock.
type upgradeLock struct{ f *os.File }

// Release drops the lock. Idempotent, so a caller may defer it and still
// drop it early.
func (l *upgradeLock) Release() {
	if l == nil || l.f == nil {
		return
	}
	l.f.Close()
	l.f = nil
}

// lockUpgrade takes the box-wide upgrade lock, WAITING rather than refusing
// when another upgrade holds it. An upgrade is minutes of `make test`, and a
// second delegate who typed the verb wants it to happen once the first is
// done — not to be told no and have to decide whether to retype it.
func lockUpgrade(w io.Writer) (*upgradeLock, error) {
	path := UpgradeLockPath()
	if path == "" {
		return nil, Die("posse upgrade: no HOME and no XDG_CACHE_HOME, so there is nowhere to put the upgrade lock — refusing rather than upgrading unserialized")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, Die("upgrade lock: %v", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, Die("upgrade lock: %v", err)
	}
	// EWOULDBLOCK is the ONLY one of these that means another upgrade is
	// running; EBADF, ENOLCK and EINVAL are the kernel refusing to answer
	// the question, and this box is then broken rather than busy
	// (tryLockLaunches' header, ranger-base-zppcv). Conflating them would
	// print "held by" over a lock nobody holds and then block on LOCK_EX
	// against nobody.
	err = flock(f, syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		fmt.Fprintf(w, "⏳ upgrade lock held by %s — waiting (ADR 0092 D3)\n", lockHolder(path))
		err = flock(f, syscall.LOCK_EX)
	}
	if err != nil {
		f.Close()
		return nil, Die("upgrade lock %s: %v", AbbrevHome(path), err)
	}
	stampLockHolder(f)
	return &upgradeLock{f: f}, nil
}

// upgradePlan is what the verb resolved before it wrote anything. Every
// refusal of D3.1 happens while building one of these, so a plan in hand
// means the target passed the guardrail and not that a write has begun.
type upgradePlan struct {
	Repo   string // the posse source checkout, resolved from the binary's own stamp
	Target string // the binary this run replaces
	From   string // the installed binary's stamped commit, as git resolved it
	To     string // the commit to install
	Tip    string // refs/remotes/origin/main as it resolved, the ancestor test's other end
	Count  int    // commits in From..To
	// Personas is the roster D4 snapshots `posse gates <p>` for. It is a
	// field of the plan rather than a read inside the run because the run is
	// what the pins drive, and a roster read from the ambient home would
	// make every one of them depend on whose box the suite is on.
	Personas []string
	// Repos is the set of guarded checkouts a G12 row after the swap would
	// be ABOUT: the repos `beads_visibility:` declares, minus the ones the
	// sweep skips. The verify block prints the operator's re-render over
	// them (ADR 0092 D4 amendment 1) and this verb never runs it. A field
	// of the plan for Personas' reason.
	Repos []string
	// Loop reads this home's dispatch --watch loop after the swap
	// (ranger-base-rrgnc); nil prints nothing, which is every pin that does
	// not ask. A field for Personas' reason.
	Loop func() (WatchImage, bool)
	// Box reads EVERY dispatch loop on this box running the binary this run
	// replaced (ranger-base-is1qp, ADR 0092 D8) — the box-wide half of
	// Loop's subject, because D3.5 renames one file and every instance's
	// loop execs it. This verb is the only reader of it: G18 stays
	// per-instance (boxloops.go says why). nil prints nothing, same as Loop.
	Box func() BoxReading
}

// upgradeWatchLine is the line before D6's (ranger-base-rrgnc): the live
// dispatch loop the rename left on the old inode, if there is one. Without
// it, "relaunch seats" reads as sufficient and is not — the loop is what
// renders a relaunch, so a relaunch through it renders the old walls.
//
// ALL THREE BRANCHES CARRY THE SCOPE CLAUSE (ranger-base-nqaa1, finding 1),
// and the two that are not the stale one are why it had to be said at all:
// this verb runs in the delegate's instance, reads that instance's loop, and
// the renamed binary is every instance's. "already runs" and "could not be
// read" are the two spellings an operator would otherwise finish believing
// the box was covered — the first because it sounds like an all-clear, the
// second because it sounds like there is nothing to restart. The stale branch
// gets the same clause from WatchImage.Sentence, so the row and the line
// cannot disagree about their reach any more than about their verdict.
func upgradeWatchLine(plan upgradePlan) string {
	if plan.Loop == nil {
		return ""
	}
	img, ok := plan.Loop()
	if !ok {
		// Why is set only where the reader can explain the abstention, and
		// this is the line it exists for: "no loop could be read" over a
		// loop whose LOCK is held sends the reader looking for a process
		// that is running (ranger-base-bekle).
		what := "no running dispatch --watch loop of this home could be read"
		if img.Why != "" {
			what = "this home's dispatch --watch loop could not be identified — " + img.Why
		}
		return "dispatch: " + what + " — seats launched after this line render the new walls only if whatever launches them was started after the swap. " + watchScopeSentence(img.Home)
	}
	if !img.Stale() {
		return fmt.Sprintf("dispatch: loop pid %d already runs %s — relaunched seats render the new walls. %s", img.Pid, AbbrevHome(img.Path), watchScopeSentence(img.Home))
	}
	return "dispatch: " + img.Sentence()
}

// upgradeBoxLine is the box-wide companion to upgradeWatchLine
// (ranger-base-is1qp, ADR 0092 D8): every dispatch loop on this box still
// running the image the rename replaced, not just this instance's.
//
// IT DOES NOT REPLACE THE LINE ABOVE IT and neither line claims the other's
// reach. upgradeWatchLine reads one instance — this delegate's — through its
// watch lock and its own pid record, which is a stronger reading of that one
// loop than a /proc census is (a lock the kernel holds, against a record a
// pid names itself by), and it is the loop whose restart the delegate's own
// next seat comes through. This one is weaker per loop and wider: it is the
// only thing in this report that can say a SIBLING instance was left behind.
// So both print, each carrying its own scope sentence, and an abstention here
// says the per-instance line is the whole of the coverage rather than
// printing nothing — which is how the hole this closes was invisible.
func upgradeBoxLine(plan upgradePlan) string {
	if plan.Box == nil {
		return ""
	}
	return plan.Box().Sentence()
}

// upgradeLoopReaders installs the two readers the report's last two loop
// lines come through: this instance's `dispatch --watch` loop
// (ranger-base-rrgnc) and every dispatch loop on this box running the file
// the swap renames (ranger-base-is1qp, ADR 0092 D8).
//
// ONE FUNCTION BECAUSE THE WIRING WAS THE PART NOTHING HELD. Both
// assignments used to sit inline in cmdUpgrade, which no pin in the tree
// reaches in either direction: the verb's reads go in through upgradeReads,
// but a plan is built behind a real build stamp, a real checkout and a box
// lock, and d4Fixture.run — the one fixture that drives a plan — assigns the
// plan's fields outright rather than deriving them. So the renderers were
// each witnessed by eight mutants over boxloops.go and the wiring by nothing.
// MEASURED 2026-11-21 (ranger-base-r1zdr, over the verify lane's finding in
// ranger-base-msir3): deleting the Box assignment, deleting the Loop one, and
// making either line's print unreachable each left 98 tests green in 3.2s, so
// ADR 0092 D8's "Both lines print, and neither claims the other's reach"
// could become false with nothing in the suite to say so. Same shape and the
// same answer as upgradeHookWallRepos one derivation over
// (ranger-base-1fsc9): a package function the pins can call is the whole of
// the fix, and nothing was added to move it — cmdUpgrade calls this instead.
//
// THE TWO SUBJECTS DIFFER ON PURPOSE, which is the half a reader assumes and
// so the half worth saying. Loop reads the APP — one instance's watch lock
// and that instance's own dispatch-watch.pid. Box reads plan.TARGET, the path
// this run replaces, because that file is box-wide and every instance's loop
// execs it, so it is not this home's anything (ADR 0092 D8).
func upgradeLoopReaders(plan *upgradePlan, a *App) {
	plan.Loop = func() (WatchImage, bool) { return ReadWatchImage(a) }
	plan.Box = func() BoxReading { return ReadBoxLoops(plan.Target) }
}

// gitCount is `git rev-list --count <range>` as a number.
func gitCount(repo, rng string) (int, error) {
	out, err := git(repo, "rev-list", "--count", rng)
	if err != nil {
		return 0, err
	}
	n, cerr := strconv.Atoi(strings.TrimSpace(out))
	if cerr != nil {
		return 0, Die("git rev-list --count %s in %s answered %q", rng, AbbrevHome(repo), out)
	}
	return n, nil
}

// gitAncestorOrEqual answers "is a reachable from b", which is
// `git merge-base --is-ancestor a b` without the exit code.
//
// NOT merge-base: that command says false by EXITING 1, and this package's
// git() turns every non-zero exit into a Die carrying git's stderr — so a
// false and a broken repository would arrive as the same error and the
// refusal would have to guess which it was reading. `rev-list --count b..a`
// exits 0 for both answers and says which by its number: zero means every
// commit reachable from a is reachable from b.
func gitAncestorOrEqual(repo, a, b string) (bool, error) {
	n, err := gitCount(repo, b+".."+a)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

// gitCommit resolves a revision to a full commit sha, or says why not.
func gitCommit(repo, rev string) (string, error) {
	return git(repo, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
}

// upgradeResolve is D3.1 whole: which commit this run would install, and
// every reason it must not.
//
// `from` is the installed binary's stamp as FindLauncher read it — a short
// sha, or the tag a release install carries. It already resolved in this
// repo, because FindLauncher would not have named the repo otherwise.
func upgradeResolve(repo, target, from, want string) (upgradePlan, error) {
	var p upgradePlan
	tip, err := gitCommit(repo, upgradeDefaultRef)
	if err != nil || tip == "" {
		return p, Die("posse upgrade: %s names no commit in %s — this verb never fetches (ADR 0092 D3.1), so the coordinator's push is what moves it; `git -C %s fetch origin` is the operator's to run",
			upgradeDefaultRef, AbbrevHome(repo), AbbrevHome(repo))
	}
	fromFull, err := gitCommit(repo, from)
	if err != nil || fromFull == "" {
		return p, Die("posse upgrade: the running binary is stamped %s, which %s cannot resolve to a commit — nothing can say what this install would be an upgrade FROM", from, AbbrevHome(repo))
	}
	if want == "" {
		want = upgradeDefaultRef
	}
	to, err := gitCommit(repo, want)
	if err != nil || to == "" {
		return p, Die("posse upgrade: %s names no commit in %s (and this verb never fetches)", want, AbbrevHome(repo))
	}

	if to == fromFull {
		return p, Die("posse upgrade: %s is already the installed binary (%s) — nothing to install", shortSha(to), from)
	}
	// origin/main FIRST, because "that commit is not on origin/main" is the
	// guardrail the operator ruled for, and a sha that fails both tests
	// should be refused by the one that is about the push.
	onMain, err := gitAncestorOrEqual(repo, to, tip)
	if err != nil {
		return p, err
	}
	if !onMain {
		return p, Die("posse upgrade: %s is not an ancestor of %s in %s (ADR 0092 D3.1) — only a commit the coordinator has PUSHED may be installed, because that push is the record that posse-land-check and the vuln-private scan ran",
			shortSha(to), upgradeDefaultRef, AbbrevHome(repo))
	}
	forward, err := gitAncestorOrEqual(repo, fromFull, to)
	if err != nil {
		return p, err
	}
	if !forward {
		return p, Die("posse upgrade: %s is not a descendant of the installed binary's stamp %s (ADR 0092 D3.1) — going backwards is a ROLLBACK, and a rollback is one coreutils command the swap below prints, never this verb: install -m 0755 %s %s",
			shortSha(to), from, AbbrevHome(upgradePrevPath(target)), AbbrevHome(target))
	}
	n, err := gitCount(repo, fromFull+".."+to)
	if err != nil {
		return p, err
	}
	return upgradePlan{Repo: repo, Target: target, From: fromFull, To: to, Tip: tip, Count: n}, nil
}

// upgradePrevPath is the one level of rollback the operator asked for
// (D3.4). Beside the target so the rollback command needs no second path,
// and overwritten by each upgrade: one level, not a history.
func upgradePrevPath(target string) string { return target + ".prev" }

// upgradeRollbackCmd is D4's command, and the reason it is a COREUTILS
// command rather than a verb of this binary: a new binary that does not run
// is exactly the case that needs it, so the rollback must not go through
// the thing being rolled back.
func upgradeRollbackCmd(target string) string {
	return fmt.Sprintf("install -m 0755 %s %s", AbbrevHome(upgradePrevPath(target)), AbbrevHome(target))
}

// ─── the steps, as seams ─────────────────────────────────────────────────────
//
// `make test` is fifteen to twenty-five minutes and clean-build.sh compiles
// the tree. Neither belongs inside a unit pin, and both must be refusable
// in one — "a red suite refuses before anything is written" is the single
// most important thing this file promises. So each shells out through a
// package var a test replaces, and the ORDER that reads them is the thing
// under test.

// upgradeStep runs one child in dir, streaming both of its streams to w.
//
// One stream and not two: what the delegate posts on the install bead is
// this command's output, and a report interleaved by the terminal but split
// in a capture is a report that reads differently depending on who reads it.
//
// protocolCmd, and it matters more here than almost anywhere (ADR 0077 D3).
// This verb runs from a persona seat whose PATH LEADS with that seat's gates
// bin dir, which holds a shim for every verb the PID denies — so a bare
// `make` resolved through this process's own PATH could be answered by a
// shim rather than by make. The child is resolved outside every gates dir
// and handed that PATH.
var upgradeStep = func(w io.Writer, dir, name string, argv ...string) error {
	cmd := protocolCmd(exec.Command(name, argv...))
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = w, w
	return cmd.Run()
}

// upgradeTest is D3.2 — the repo's own full suite, as the repo defines it.
// `make test` and never a bare `go test ./...`: the three arms, and the box
// suite slots that keep five of them from running at once (AGENTS.md).
var upgradeTest = func(w io.Writer, dir string) error {
	return upgradeStep(w, dir, "make", "test")
}

// upgradeBuild is D3.3 — scripts/clean-build.sh, which is what `make
// install` builds through, writing straight to the staging path beside the
// target so the swap below is a rename within one directory.
//
// POSSE_BUILD_SOURCE is why this is not just the script's own default. The
// script stamps the checkout it was run in as the binary's build source
// (ranger-base-eiwhk), and the checkout it is run in HERE is a throwaway
// copy this verb deletes minutes later — a stamp guaranteed dead by the
// time anything reads it, which is exactly the `$src` case that script's own
// pin refuses. The real source checkout is handed in instead.
var upgradeBuild = func(w io.Writer, dir, repo, out string) error {
	cmd := protocolCmd(exec.Command(filepath.Join(dir, "scripts", "clean-build.sh"), out))
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = w, w
	// protocolEnv, because assigning Env REPLACES what protocolCmd put
	// there — and this script's own children are `git` and `go`, both of
	// which a seat's gates dir can shim.
	cmd.Env = protocolEnv(append(os.Environ(), "POSSE_BUILD_SOURCE="+repo))
	return cmd.Run()
}

// upgradeEpilogue is D3.6 and D5 — the gate copy arm, then `make install`'s
// own hook-freshness check, both run from the private copy, and it answers D6's
// gate word. Advisory both: a stale hook, or a gate copy that could not be
// refreshed, is not a reason to fail an install that already happened (the
// Makefile's own words).
//
// THE GATE HALF IS AN ARM NOW, not a reading. It used to be
// `verify-gate-freshness` run for its exit code — a detective control
// reporting that the box's copy was stale, which is what it had been doing
// at `make install` since ranger-base-d0jo and which moved nothing. D5 gave
// the arm to this verb: same source, same review path, same blast class as
// the binary. upgradegate.go is the whole of it, and the word it returns is
// the four-word set D6 settled on at acceptance.
var upgradeEpilogue = func(w io.Writer, wt string, plan upgradePlan) string {
	gate := upgradeGateArm(w, wt, plan)
	hook := protocolCmd(exec.Command(filepath.Join(wt, "scripts", "verify-hook-freshness.sh"), "--quiet"))
	hook.Dir = wt
	hook.Stdout, hook.Stderr = w, w
	// POSSE is the binary verify-hook-freshness renders hooks through, and
	// it must be the one just installed — the Makefile passes $(BINDIR)/posse
	// for the same reason. Through protocolEnv, because assigning Env
	// replaces the gates-free PATH protocolCmd put on this child.
	hook.Env = protocolEnv(append(os.Environ(), "POSSE="+plan.Target))
	_ = hook.Run()
	return gate
}

// upgradePrivateCopy is D3.2's fresh checkout of the commit, and the function
// that removes it. It is a FETCH into a new repository, never a `git worktree
// add` of the source checkout (ranger-base-p8ymi).
//
// WHY. git trusts an object's bytes by the name it is filed under and does
// not re-hash a loose or packed object on a local read, and every walled
// worktree seat is granted the shared <common>/objects whole. A worktree add
// of a sha therefore checks out whatever bytes sit under that sha's names —
// so a seat that rewrites one blob origin/main reaches gets ITS code run by
// `make test` here, uncaged, as this verb's caller, and (before
// ranger-base-la61c) built and installed as the fleet's binary under an
// unchanged stamp. A fetch goes through the pack protocol, and the receiving
// side hashes every object it is sent: bytes that do not hash to the name
// they were asked for leave the commit's tree incomplete and the fetch
// fails "did not send all necessary objects" before anything is checked out.
// The new repository's store is under this run's mktemp -d, inside
// upgradePrivateRoot, which no seat is granted at either mechanism — a
// private /tmp at bwrap, a named deny at seatbelt, and that constant's
// comment has what the sentence used to claim and why it was false
// (ranger-base-ua625). So a swap after the fetch has nothing to swap. make
// test, the build (clean-build.sh's own fetch then reads THIS store) and the
// epilogue all run from it.
//
// WHAT IS FETCHED IS THE TIP, and D3.1's two ancestry answers are taken
// again here. upgradeResolve read them out of the shared store, where a
// rewritten commit object can claim any parent it likes; in this store every
// commit reachable from the tip has been re-hashed, so "to is on origin/main"
// and "from is behind to" are answers about the commits those names really
// are. A `to` the true history does not reach is simply absent here, and the
// rev-list that looks for it refuses.
//
// --no-replace-objects on every call: refs/replace is not seat-writable
// today, and a replace ref is the one way git will serve DIFFERENT bytes
// under a name on purpose. The flag sets GIT_NO_REPLACE_OBJECTS in the
// environment, so the upload-pack the fetch spawns inherits it. A belt.
//
// MEASURED 2026-11-14, git 2.56.0, linux: a fetch by sha from a local path
// into an empty repository succeeds with no uploadpack.allow* setting; a
// substituted loose blob and a commit object rewritten to claim another
// parent were both refused by it. A `checkout` after the refused fetch
// exits 0 with an incomplete tree, so the fetch's own exit is what refuses.
// Cost on this repo, the tip's full history: docs/notes.d/ranger-base-p8ymi.md.
//
// A fresh mktemp -d every time: a directory that is new needs no rm of a
// previous one (AGENTS.md). The repository is a child of it, as in
// clean-build.sh.
func upgradePrivateCopy(plan upgradePlan) (dir string, done func(), err error) {
	if plan.Tip == "" || plan.To == "" || plan.From == "" {
		return "", nil, Die("posse upgrade: the plan names no %s, so there is nothing to fetch and check — this is a bug in the verb, not in the repo", upgradeDefaultRef)
	}
	// THE ROOT IS LSTAT'D BEFORE IT IS USED (ranger-base-ua625). It is a
	// fixed name under a world-of-one-uid temp root, so a link planted at
	// it would send this run's copy — and the uncaged `make test` that runs
	// out of it — wherever the planter chose, which is the whole hole the
	// deny closes. A link or a file at the name is refused rather than
	// removed: this verb does not delete things it did not make, and the
	// operator who is told the name can look at it.
	root := upgradePrivateRoot()
	// AND THE ROOT IS DENIED, asked and not assumed (ranger-base-18brx).
	// upgradePrivateRootDenied's comment says why this is checked when the
	// root is derived from the same deny set it is checked against: the
	// answer is free and the shape it refuses — a root read out of this
	// process's own environment — reads like the right one and leaves the
	// copy inside a seat's write grant.
	if !upgradePrivateRootDenied(runtime.GOOS, root) {
		return "", nil, Die("posse upgrade: the private copy would be staged at %q, which is not one of the names every rendered seat profile denies (%v) — a seat could then write the tree this verb runs `make test` out of, UNCAGED, as its caller. Refusing before anything is fetched (ranger-base-18brx)", root, upgradePrivateDenied(runtime.GOOS, ""))
	}
	switch fi, err := os.Lstat(root); {
	case err == nil && !fi.IsDir():
		return "", nil, Die("posse upgrade: %s is a %s and not a directory — this verb stages its private copy there and every rendered profile denies that name, so something outside the wall put it there. Look at it, remove it, and run this again (ranger-base-ua625)", AbbrevHome(root), fi.Mode().Type())
	case err != nil && !os.IsNotExist(err):
		return "", nil, Die("posse upgrade: cannot read %s, where the private copy is staged: %v", AbbrevHome(root), err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", nil, Die("posse upgrade: %v", err)
	}
	// `posse-` heads the per-run name as well as the root, because the
	// tree-wide census reads the PREFIX literal at the call site and scores
	// an ambiguous parent in (internal/treepins/temproot_qa_test.go): a root
	// outside posse's temp namespace is collected by no class of
	// scripts/reap-test-tmp.sh at any age, and the remover here is a
	// `defer`, which runs on no SIGKILL. Redundant under a parent already
	// named `posse-upgrade`, and the redundancy is the point — the run dir
	// is reapable on its own if it ever outlives its root.
	tmp, err := os.MkdirTemp(root, "posse-upgrade-run.")
	if err != nil {
		return "", nil, Die("posse upgrade: %v", err)
	}
	src := filepath.Join(tmp, "src")
	cleanup := func() { os.RemoveAll(tmp) }
	refuse := func(why string, err error) (string, func(), error) {
		cleanup()
		return "", nil, Die("posse upgrade: %s — refusing before the suite runs (ranger-base-p8ymi). Every object is re-hashed on the way in, so this means %s's object store does not hold the bytes its names promise; `git -C %s fsck` says which. Nothing was written. (%v)",
			why, AbbrevHome(plan.Repo), AbbrevHome(plan.Repo), err)
	}
	if _, err := git(tmp, "--no-replace-objects", "init", "--quiet", src); err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := git(src, "--no-replace-objects", "fetch", "--quiet", "--no-tags", plan.Repo, plan.Tip); err != nil {
		return refuse(fmt.Sprintf("fetching %s (%s) did not verify", upgradeDefaultRef, shortSha(plan.Tip)), err)
	}
	if got, err := git(src, "--no-replace-objects", "rev-parse", "FETCH_HEAD"); err != nil || got != plan.Tip {
		return refuse(fmt.Sprintf("the fetch answered %q, not %s", got, plan.Tip), err)
	}
	if ok, err := gitAncestorOrEqual(src, plan.To, plan.Tip); err != nil || !ok {
		return refuse(fmt.Sprintf("%s is not an ancestor of %s once its history is re-hashed (ADR 0092 D3.1)", shortSha(plan.To), shortSha(plan.Tip)), err)
	}
	if ok, err := gitAncestorOrEqual(src, plan.From, plan.To); err != nil || !ok {
		return refuse(fmt.Sprintf("%s is not a descendant of the installed stamp %s once its history is re-hashed (ADR 0092 D3.1)", shortSha(plan.To), shortSha(plan.From)), err)
	}
	// The suite ran in a checkout that could see refs/remotes/origin/main,
	// and the fetch writes no ref, so give it the one the plan read.
	if _, err := git(src, "--no-replace-objects", "update-ref", upgradeDefaultRef, plan.Tip); err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := git(src, "--no-replace-objects", "checkout", "--quiet", "--detach", plan.To); err != nil {
		cleanup()
		return "", nil, err
	}
	return src, cleanup, nil
}

// upgradeSwap is D3.4 and D3.5: keep, then swap.
//
// The keep is a COPY and not a move, because the live binary stays live
// until the rename — this process is running out of it. It goes through
// WriteExecutable for the reason execwrite.go gives: the rollback command
// execs <target>.prev, and a write descriptor a sibling fork inherits
// between this open and its close makes that exec answer ETXTBSY
// (golang/go#22315). Nothing in this verb forks concurrently, so the hold
// is uncontended; it is here so a later edit that does fork cannot
// reintroduce the window.
//
// The swap is rename(2), which is atomic: a launch in flight gets the whole
// old binary or the whole new one and never the gap install(1) leaves (the
// measurement in this file's header). Like install(1) it gives the target a
// new inode, so a running process and any bind mount of the old name keep
// the old one — no change from today.
func upgradeSwap(target, staged string) error {
	live, err := os.ReadFile(target)
	if err != nil {
		return Die("posse upgrade: cannot read the live binary %s to keep a rollback copy: %v", AbbrevHome(target), err)
	}
	prev := upgradePrevPath(target)
	if err := WriteExecutable(prev, live, 0o755); err != nil {
		return Die("posse upgrade: cannot write %s, so there would be nothing to roll back to: %v", AbbrevHome(prev), err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return Die("posse upgrade: cannot make %s executable: %v", AbbrevHome(staged), err)
	}
	if err := os.Rename(staged, target); err != nil {
		return Die("posse upgrade: cannot swap %s into place: %v", AbbrevHome(target), err)
	}
	return nil
}

// upgradeExitCode is a child's exit status, or -1 when it did not run.
//
// Read by the gate arm as well as by the steps above, and there it is read
// WITHOUT `--warn` — the one spelling that differs from `make install`'s
// epilogue. `--warn` exists so a finding cannot fail an install, and it
// buys that by exiting 0 whatever it found, which would leave D6's field
// with nothing to read. The verb gets the same property by consuming the
// code rather than propagating it: the operator sees the identical report,
// the upgrade is not failed by it, and D6's line says what it said.
func upgradeExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// ─── the verb ────────────────────────────────────────────────────────────────

// upgradeReads is the verb's three readings of the live box, handed in
// rather than taken — so the one function that WIRES the guardrails
// together is reachable from a pin (ranger-base-anczm).
//
// WHY IT EXISTS. Until this seam, nothing in the tree reached the verb, and
// each of the three guardrails it adds could be deleted with the suite and
// `make tree-check` both green: the D2 fence call below, the dirty-stamp
// refusal, and the D3 flock. MEASURED 2026-11-13 — three one-line
// mutations, `go test -run TestUpgrade ./internal/posse` green for all
// three. What WAS pinned is upgradeFence as a pure function and
// upgradeRunPlan's step order, which is every part except the wiring.
//
// A SEAM for dispatch.Lag's reason. All three readings key off the running
// binary's own build stamp, and a TEST binary carries none at all
// (cagestale.go's header) — VersionString() is "+dev", so an end-to-end
// reading abstains before the verb's first refusal is reached. Handing them
// in is what makes a real stamp and a real repo available to a pin.
//
// PARAMETERS and not package vars like the three steps below, because all
// three call sites are in one function and this package's suite is
// parallel: a var swapped under one test answers questions no other test
// asked (runningPosse's own header, applied to the test side).
//
// WHAT IS STILL UNREACHABLE, said here so nobody reads the seam as total:
// CmdUpgrade, which binds these three names to the real readings and does
// nothing else — no branch, no refusal. That is the residue every seam of
// this shape leaves — dispatch.Lag and GovInputs.Lag leave the same one —
// and it is the smallest piece this file can leave unreached.
type upgradeReads struct {
	Bin  func() PosseBinary                    // RunningPosse
	Lag  func() LauncherLag                    // (*App).LauncherOverFleet
	Lock func(io.Writer) (*upgradeLock, error) // lockUpgrade
}

// CmdUpgrade installs a commit of posse that is already on origin/main and
// whose suite is green, over the binary this process is running out of.
func (a *App) CmdUpgrade(w io.Writer, o UpgradeOpts) error {
	return a.cmdUpgrade(w, o, upgradeReads{
		Bin:  RunningPosse,
		Lag:  a.LauncherOverFleet,
		Lock: lockUpgrade,
	})
}

// cmdUpgrade is the verb over readings a pin can supply: D2's fence, D1's
// stamp refusal, D3's lock and the plan, each refusing before the next one
// reads anything.
func (a *App) cmdUpgrade(w io.Writer, o UpgradeOpts, reads upgradeReads) error {
	// ADR 0092 D4 rule 4 — the partition this binary compiled in, printed
	// for the OLD binary that is deciding whether to keep it.
	//
	// FIRST, ABOVE EVERYTHING, and that position is the feature. The ADR
	// gives it in as many words: read-only, no lock, no `install_delegate:`.
	// The process asking it is another binary's `posse upgrade` mid-install,
	// which already holds the box-wide flock (nesting it would deadlock —
	// flock is per open file description, lockUpgrade's own header) and
	// which cannot know whether the NEW binary's config names the same
	// delegate. A refusal here would be read as "did not answer", and rule
	// 5 trips on that: the fence would roll back the install that installed
	// it. It also reads nothing from `reads`, which is what lets a pin hand
	// this path a zero upgradeReads and have a nil func panic if the flag
	// ever starts taking a reading.
	if o.Classes {
		return upgradeWriteClasses(w)
	}

	if err := upgradeFence(os.Getenv(EnvPersona), a.UpgradeDelegate()); err != nil {
		return err
	}

	// WHICH BINARY, and it is the one that is RUNNING rather than a path
	// this file guesses. `make install` writes $(BINDIR)/posse with BINDIR
	// defaulting to ~/.local/bin, and a second spelling of that default
	// here would be a path to go wrong on a box that moved it. The running
	// binary needs no default and cannot be wrong, and it is also D1's own
	// subject: the installer a seat cannot edit is the one it is executing.
	bin := reads.Bin()
	if bin.Err != nil || bin.Exe == "" {
		return Die("posse upgrade: this process cannot name its own binary (%v), so there is no target to replace", bin.Err)
	}

	// The stamp is the fence D1 rests on, so a binary without a clean one
	// does not get to install the fleet's. `go run`, a `go build` with no
	// ldflags and a build from a dirty tree all land here: each is a binary
	// whose code is not a commit anybody pushed, which is the whole of what
	// guardrail 1 is about.
	lag := reads.Lag()
	if lag.Dirty {
		return Die("posse upgrade: the running binary (%s) was built from a tree with uncommitted edits — its code is in no commit, so nothing can say it came through this gate (ADR 0092 D1). Install from a clean commit first.", bin.Version)
	}
	if !lag.Known() || lag.Repo == "" {
		return Die("posse upgrade: %s (looked for the posse source checkout via config posse_repo:, then beads:, then this binary's build stamp — ADR 0092 D3.1 needs one to read %s in)", lag.Why, upgradeDefaultRef)
	}

	lock, err := reads.Lock(w)
	if err != nil {
		return err
	}
	defer lock.Release()

	plan, err := upgradeResolve(lag.Repo, bin.Exe, lag.Rev, strings.TrimSpace(o.Sha))
	if err != nil {
		return err
	}
	// D4's roster: every persona this home declares, snapshotted before the
	// swap and read again after it. ListAgents and not the live sessions —
	// `posse gates <p>` describes the wall a persona WOULD launch behind, so
	// a persona with no seat open is exactly as much evidence as one with.
	plan.Personas = a.ListAgents()
	// D4 amendment 1's repos. Read here rather than at verify time so that
	// the plan carries everything the run prints, as Personas does; the
	// filter itself is upgradeHookWallRepos, which is where a pin can read
	// it.
	plan.Repos = append(plan.Repos, upgradeHookWallRepos(a.SweepHookWallIdentity())...)
	// The report's last two loop lines, installed through one function so
	// that a pin can witness both of them being installed at all
	// (ranger-base-r1zdr). Inline here, they were held by nothing.
	upgradeLoopReaders(&plan, a)
	return upgradeRunPlan(w, plan)
}

// upgradeHookWallRepos is D4 amendment 1's repo list: which of the walls
// config declares may appear in the report's `posse gates install-hooks`
// line. Read from the same place G12 reads them and filtered the same way —
// a skipped repo raises no G12 row, and a managed one is a repo posse must
// not be told to write (ADR 0052 D1). Repos come back in config order, which
// is the order the sweep reports and the order the operator's config lists,
// so the printed line reads as the config does.
//
// A FUNCTION over the sweep value, and not the three lines it replaces
// inside cmdUpgrade, because of where cmdUpgrade sits: its reads go in
// through upgradeReads but SweepHookWallIdentity does not, so reaching the
// loop from a test meant a real stamp, a real checkout and a box lock, and
// d4Fixture.run overwrites plan.Repos outright rather than deriving it. The
// ADR 0052 D1 filter — the whole of this verb's obligation to that ADR — was
// therefore deletable green: `if true` in place of `if r.Skip == ""` put a
// managed checkout and an absent one both into the one instruction that ADR
// says this verb must never give, over 77 tests, in 2.2s, ok
// (ranger-base-1fsc9, escaped from ranger-base-hm4gf). A HookWallSweep is a
// slice of structs, so here the same filter needs no binary, no App and no
// seam. Nothing was added to move it: cmdUpgrade calls this instead.
func upgradeHookWallRepos(s HookWallSweep) []string {
	var out []string
	for _, r := range s.Repos {
		if r.Skip == "" {
			out = append(out, r.Dir)
		}
	}
	return out
}

// upgradeRunPlan is D3.2 through D3.6 and D6 — everything this verb WRITES,
// in the order that is the guardrail.
//
// Separate from cmdUpgrade because the order is the thing worth pinning and
// it is pinnable here with no seam at all: a plan is four strings, so this
// half needs neither a build stamp nor a box lock to reach. cmdUpgrade is
// the four reads that produce a plan; this is what the plan then does.
//
// It used to say cmdUpgrade "cannot be reached from a test", which was true
// of the reads and not of the wiring — three guardrails were deletable
// green because of it (upgradeReads' header). The reads go in through
// upgradeReads now; that is the only part of this split that moved.
func upgradeRunPlan(w io.Writer, plan upgradePlan) error {
	fmt.Fprintf(w, "posse upgrade · %s → %s (%d commit(s)) · source %s · target %s\n",
		shortSha(plan.From), shortSha(plan.To), plan.Count, AbbrevHome(plan.Repo), AbbrevHome(plan.Target))
	fmt.Fprintf(w, "  %s is at %s; this verb never fetches (ADR 0092 D3.1)\n", upgradeDefaultRef, shortSha(plan.To))

	// Can this run write where it will have to write, BEFORE it spends
	// twenty minutes of box on a suite? The staging file is the first thing
	// the build produces and lives in the target's own directory, so
	// creating and removing it answers every question the swap will ask:
	// directory writable, path not a directory, no stale file in the way.
	// A refusal here costs nothing; the same refusal after step 2 costs a
	// full suite and two suite slots other seats were queueing for.
	staged := plan.Target + ".new"
	// 0o600 and not 0o755: this is a probe and nothing execs it. The real
	// staging file gets its mode from the build and from upgradeSwap's
	// chmod, which is the one place that decides what the fleet runs.
	if f, err := os.OpenFile(staged, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err != nil {
		return Die("posse upgrade: cannot write %s, so the new binary would have nowhere to land: %v", AbbrevHome(staged), err)
	} else {
		f.Close()
		os.Remove(staged)
	}

	wt, done, err := upgradePrivateCopy(plan)
	if err != nil {
		return err
	}
	defer done()

	// The copy's path is PRINTED, because the reader of a red suite needs
	// to know which tree it ran in. It is a run dir under
	// upgradePrivateRoot, which since ranger-base-18brx is `/tmp` and NOT
	// this process's TMPDIR — so the copy itself is no longer one of the
	// variables below. The suite it runs still inherits that TMPDIR for its
	// own temp dirs, and this tree's suite moves with TMPDIR in three ways
	// that all read as "the commit is red" (MEASURED 2026-11-13 at 983ae5d,
	// one -run filter per test; ranger-base-6ca29 carries the matrix and
	// owns whatever is done about it):
	//
	//	a quota'd tmpfs    every one of ~30 failures was "disk quota
	//	                   exceeded", from git, the linker and the tests.
	//	under $HOME        AbbrevHome renders such a path as `~/…`, so every
	//	                   test asserting its own absolute temp path reds.
	//	anywhere but /tmp  the bwrap sandbox's view of /tmp is not its view
	//	                   of /var/tmp, and ADR 0086 D5's wall-witness pin
	//	                   reads the wall differently there.
	//
	// None of it is anything about the commit. A reader who can see the
	// path can see that in one line; one who cannot goes looking for a bug.
	fmt.Fprintf(w, "\n── make test at %s (ADR 0092 D3.2) ──\n  private copy: %s\n", shortSha(plan.To), wt)
	if err := upgradeTest(w, wt); err != nil {
		return Die("posse upgrade: `make test` at %s did not pass (%v) — nothing was written, and nothing will be. The binary in place is the one that was there before this command ran.", shortSha(plan.To), err)
	}

	fmt.Fprintf(w, "\n── clean-build at %s (ADR 0092 D3.3) ──\n", shortSha(plan.To))
	if err := upgradeBuild(w, wt, plan.Repo, staged); err != nil {
		os.Remove(staged)
		return Die("posse upgrade: the build at %s failed (%v) — nothing was swapped", shortSha(plan.To), err)
	}

	// D4's snapshot, taken HERE: after the build, so it is as close to the
	// swap as a reading can be, and before the keep so that nothing about
	// the target has moved when it is taken. Its cost is one `posse status`
	// and one `posse gates` per persona, which is seconds against the suite
	// above — and it is spent on a run that is already going to swap, so it
	// never delays a refusal.
	fmt.Fprintf(w, "\n── snapshot before the swap (ADR 0092 D4) ──\n")
	before := upgradeRead(plan.Target, plan.Repo, plan.Personas)
	fmt.Fprintf(w, "  %s\n", before.Line())

	// BEFORE the swap, not after: the reader who needs this line most is
	// the one whose next command is the one that broke (ADR 0092 D4).
	fmt.Fprintf(w, "\nrollback, if the new binary misbehaves: %s\n", upgradeRollbackCmd(plan.Target))
	if err := upgradeSwap(plan.Target, staged); err != nil {
		os.Remove(staged)
		return err
	}
	fmt.Fprintf(w, "installed: %s (%s)\n", AbbrevHome(plan.Target), shortSha(plan.To))

	// D4's judgement, BEFORE the epilogue and not after it. The epilogue
	// runs two scripts out of the new worktree and hands one of them the
	// just-installed binary as POSSE — so on a binary this step is about to
	// undo, it is work spent through the thing under suspicion, and its gate
	// word would describe a binary that is no longer there by the time D6's
	// line prints.
	fmt.Fprintf(w, "\n── verify (ADR 0092 D4) ──\n")
	if err := upgradeVerify(w, plan, before); err != nil {
		return err
	}

	fmt.Fprintf(w, "\n── epilogue: the gate copy arm (ADR 0092 D5) and the hook wall reading (D3.6) ──\n")
	gate := upgradeEpilogue(w, wt, plan)

	// The line before D6's: which live loop still renders the old walls
	// (ranger-base-rrgnc). Restarting it is not this verb's (guardrail 3).
	if line := upgradeWatchLine(plan); line != "" {
		fmt.Fprintf(w, "\n%s\n", line)
	}
	// And the box-wide one, because the file just renamed is box-wide and
	// the line above reads one instance (ranger-base-is1qp, ADR 0092 D8).
	// AFTER it, not instead of it: the per-instance line names the loop this
	// delegate's own seats come through, which is the one its reader will act
	// on first, and each line states its own reach.
	if line := upgradeBoxLine(plan); line != "" {
		fmt.Fprintf(w, "%s\n", line)
	}

	// D6 — the LAST stdout line, and the one the delegate posts on the
	// install bead. Everything above may be long; this is the record.
	fmt.Fprintf(w, "\nupgraded %s..%s (%d commits) · gate %s · rollback: %s\n",
		shortSha(plan.From), shortSha(plan.To), plan.Count, gate, upgradeRollbackCmd(plan.Target))
	return nil
}
