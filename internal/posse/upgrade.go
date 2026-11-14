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
//	2. test     `make test` in a fresh detached worktree of that sha — all
//	            three arms, suite slots and all. Red, or no answer, refuses.
//	3. build    scripts/clean-build.sh in that same worktree, so the binary
//	            is built from the tree the tests passed on.
//	4. keep     the live binary is copied to <target>.prev. One level of
//	            rollback, as the operator asked.
//	5. swap     rename(2) over the target, never install(1) — see below.
//	6. epilogue `make install`'s own two checks, from the worktree.
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
// WHAT IS NOT HERE. The bd argv gate copy arm is D5 and belongs to
// ranger-base-41l0r. D4 — the status/gates snapshot taken before the swap,
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
	return upgradePlan{Repo: repo, Target: target, From: fromFull, To: to, Count: n}, nil
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
// worktree this verb deletes minutes later — a stamp guaranteed dead by the
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

// upgradeEpilogue is D3.6 — `make install`'s own two checks, run from the
// worktree, and it answers D6's gate word. Advisory both: a stale hook or
// gate copy is not a reason to fail an install that already happened (the
// Makefile's own words).
var upgradeEpilogue = func(w io.Writer, wt, target string) string {
	gate := upgradeGateWord(upgradeStep(w, wt, filepath.Join(wt, "scripts", "verify-gate-freshness.sh")))
	hook := protocolCmd(exec.Command(filepath.Join(wt, "scripts", "verify-hook-freshness.sh"), "--quiet"))
	hook.Dir = wt
	hook.Stdout, hook.Stderr = w, w
	// POSSE is the binary verify-hook-freshness renders hooks through, and
	// it must be the one just installed — the Makefile passes $(BINDIR)/posse
	// for the same reason. Through protocolEnv, because assigning Env
	// replaces the gates-free PATH protocolCmd put on this child.
	hook.Env = protocolEnv(append(os.Environ(), "POSSE="+target))
	_ = hook.Run()
	return gate
}

// upgradeWorktree is D3.2's fresh detached worktree, and the function that
// removes it. A fresh mktemp -d every time: a directory that is new needs no
// rm of a previous one, which is the shape that never prompts and never
// deletes the wrong tree (AGENTS.md).
func upgradeWorktree(repo, sha string) (dir string, done func(), err error) {
	tmp, err := os.MkdirTemp("", "posse-upgrade.")
	if err != nil {
		return "", nil, Die("posse upgrade: %v", err)
	}
	// git worktree add refuses a directory that already exists, so the
	// worktree is a child of the temp dir rather than the temp dir itself —
	// clean-build.sh's own arrangement, for the same reason.
	wt := filepath.Join(tmp, "src")
	if _, err := git(repo, "worktree", "add", "--detach", "--quiet", wt, sha); err != nil {
		os.RemoveAll(tmp)
		return "", nil, err
	}
	return wt, func() {
		_, _ = git(repo, "worktree", "remove", "--force", wt)
		os.RemoveAll(tmp)
	}, nil
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

// upgradeGateWord is D6's gate field, read off verify-gate-freshness's exit
// code: 0 clean, 1 findings, 2 nothing measured (that script's own header).
//
// WHY IT IS RUN WITHOUT --warn, which is the one spelling that differs from
// `make install`'s epilogue. `--warn` exists so a finding cannot fail an
// install, and it buys that by exiting 0 whatever it found — which would
// leave this field with nothing to read. The verb gets the same property by
// consuming the code rather than propagating it: the operator sees the
// identical report, the upgrade is not failed by it, and D6's line says what
// it said.
//
// `installed` and `unchanged` are D5's words and are not reachable yet: the
// arm that installs the gate copy is ranger-base-41l0r. Until it lands, a
// stale copy is reported as stale and left exactly where it was.
func upgradeGateWord(err error) string {
	switch upgradeExitCode(err) {
	case 0:
		return "fresh"
	case 1:
		return "stale"
	default:
		return "unknown"
	}
}

// upgradeExitCode is a child's exit status, or -1 when it did not run.
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
	// D4 amendment 1's repos, read from the same place G12 reads them and
	// filtered the same way: a skipped repo raises no G12 row, and a
	// managed one is a repo posse must not be told to write (ADR 0052 D1).
	// Read here rather than at verify time so that the plan carries
	// everything the run prints, as Personas does.
	for _, r := range a.SweepHookWallIdentity().Repos {
		if r.Skip == "" {
			plan.Repos = append(plan.Repos, r.Dir)
		}
	}
	return upgradeRunPlan(w, plan)
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

	wt, done, err := upgradeWorktree(plan.Repo, plan.To)
	if err != nil {
		return err
	}
	defer done()

	// The worktree path is PRINTED, because it is the one thing about this
	// run that the environment decides and the reader of a red suite needs.
	// It is `mktemp -d`, so it is TMPDIR, and this tree's suite moves with
	// TMPDIR in three ways that all read as "the commit is red" (MEASURED
	// 2026-11-13 at 983ae5d, one -run filter per test; ranger-base-6ca29
	// carries the matrix and owns whatever is done about it):
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
	fmt.Fprintf(w, "\n── make test at %s (ADR 0092 D3.2) ──\n  worktree: %s\n", shortSha(plan.To), wt)
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

	fmt.Fprintf(w, "\n── epilogue (ADR 0092 D3.6) ──\n")
	gate := upgradeEpilogue(w, wt, plan.Target)

	// D6 — the LAST stdout line, and the one the delegate posts on the
	// install bead. Everything above may be long; this is the record.
	fmt.Fprintf(w, "\nupgraded %s..%s (%d commits) · gate %s · rollback: %s\n",
		shortSha(plan.From), shortSha(plan.To), plan.Count, gate, upgradeRollbackCmd(plan.Target))
	return nil
}
