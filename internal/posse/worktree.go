package posse

// Per-session git worktrees (rangerhq-09o2; decided in rangerhq-nyqj,
// merge-back policy ruled by the operator in rangerhq-jbyr).
//
// Dispatch used to hand every persona the same checkout: N personas, one
// working tree, one `.git/index`, one HEAD. The measured damage was five
// distinct failures, and only one of them — a `bd sync` commit sweeping
// another persona's staged fix — is closed by the commit wall (ADR 0002 §3's
// prepare-commit-msg guard). `git checkout <path>` discarding a neighbour's
// edits, `git stash` taking someone else's WIP, half-written files landing
// under a green message, and two personas racing writes to one file all go
// straight through it — and the last two go through the *blessed* commit
// form, because `git commit -- <path>` commits the WORKING TREE content of
// the path you name (measured, rangerhq-2f5r). Isolation is the only fix for
// four fifths of the class, so isolation is what this is.
//
// The shape:
//
//	main checkout          ~/src/<repo>              the operator's, on main
//	session worktree       <root>/<repo>/<session>   one persona, one bead
//	session branch         posse/<session>           cut from the repo's HEAD
//
// and the merge-back is **option A** (rangerhq-jbyr, operator's ruling): the
// launcher merges. Personas keep their `Bash(git push:*)` deny and never
// touch main; posse fast-forwards the session branch onto the repo's own
// branch under the launcher lock it already holds (ADR 0011 §1), so "closed
// means it is on main" stays true for the QA verify pass (ADR 0006 §3).
//
// WHAT WAS MEASURED, so the next reader does not have to (bd 0.49.1, git
// 2.39.3, in a throwaway repo with a tracked `.beads/issues.jsonl`, and
// re-measured on bd 0.50.3 — see WHICH VERSION, WHICH STORE CLASS below,
// because on 0.50.3 the class is the whole of it):
//
//   - With an ABSOLUTE `redirect`, bd reads and writes the main database
//     from the worktree, creates no database of its own, and the graph does
//     not fork. A bead filed in the worktree is in the main repo's db.
//
//   - CORRECTED 2026-08-28 (ranger-base-vczf). This block used to say that a
//     linked worktree with NO redirect forks the graph — bd finding the
//     checked-out `issues.jsonl`, reporting "fresh clone detected", building
//     a second database beside it. It does not, on this bd. bd 0.49.1
//     resolves a linked worktree to the MAIN checkout's `.beads` by itself,
//     and while the main checkout has one it does not read the worktree's
//     `redirect` at all — a redirect pointing at a different LIVE database is
//     ignored and bd goes on reading the main graph. IN DATABASE MODE: that
//     resolution is FindBeadsDir's, and the no-db bullet below is the path
//     that never calls it. Measured in all three
//     shapes: worktree with a checked-out `.beads`, worktree with none, and
//     a main checkout holding a jsonl but no database yet, where the "fresh
//     clone" database is built in the MAIN checkout. bd falls back to the
//     worktree's own redirect only when the main checkout has no `.beads`,
//     which is the one shape seedBeadsRedirect declines to write for.
//     TestLiveWorktreeBdResolvesTheWorktreeItself pins that, so the day it
//     changes is a red test.
//
//   - The staleness trap named in rangerhq-09o2 does NOT fire through a
//     correct redirect. A worktree checkout does materialize the tracked
//     `issues.jsonl` with a fresh mtime, but bd's staleness check compares
//     the mtime of the jsonl beside the database it RESOLVED to — the main
//     repo's. Touching the worktree's own copy forward changes nothing;
//     touching the main repo's copy forward is what raises "Database out of
//     sync with JSONL". So the worktree's materialized jsonl is inert and is
//     left alone deliberately: deleting or back-dating it would dirty a tree
//     the persona is about to commit from, which is a worse bug than the one
//     it would prevent. No `--allow-stale`, no `bd sync --import-only`
//     suggestion for the persona to walk into.
//
//   - `redirect` is in bd's own bundled `.beads/.gitignore`, so writing one
//     leaves the worktree clean in any bd-initialised repo.
//
//   - WHICH VERSION, WHICH STORE CLASS (2026-09-04, ranger-base-9lrzx). What
//     the live pins in worktreelive_test.go assert out of the bullets above
//     is re-measured on bd 0.50.3 and holds there — for a SQLite-backed
//     store, which is what `bd init` built on 0.49.1 and what the operator's
//     queue is.
//     They do NOT hold for a no-db (JSONL-only) store: there bd reads the
//     worktree's `redirect` for the RESOLUTION (`bd where` answers the main
//     checkout's `.beads` and names the redirect that took it there) and
//     then reads and writes the worktree's own `issues.jsonl` anyway, so a
//     bead filed from the worktree never reaches the main graph. The fork is
//     invisible from a read, because the worktree's checked-out jsonl
//     carries the main rows by construction; only a write tells them apart.
//     TestLiveWorktreeNoDbStoreForksTheGraph pins it.
//
//   - NO-DB IS NOT A VERSION, IT IS A MODE, AND IT HAS FOUR DOORS
//     (2026-09-04, bd 0.50.3, ADR 0055). The mechanism is in bd's source,
//     not in a release: `cmd/bd/nodb.go` (read) and `main.go`'s
//     `PersistentPostRun` (write-back) resolve `$BEADS_DIR`, else
//     `$cwd/.beads`, and never call `FindBeadsDir` — so neither the redirect
//     nor the worktree's main repo is consulted, and `$cwd` is the CWD, not
//     the repo root (a no-db `bd` from a subdirectory says "no .beads
//     directory found"). Read in 0.49.1's source; behaviour measured on
//     0.50.3. The doors:
//       1. `no-db: true` in the resolved store's `config.yaml`;
//       2. `--no-db` on the command line — **posse opens this one itself**,
//          at the CONTAINER tier and only there (CageBdFlags, written onto
//          the inner PATH by renderCageBd, whose one non-test call site is
//          the container inner render in cageinner.go), so at that tier the
//          fork is the shipped configuration on EVERY store class, not a
//          store-class accident. NOT "every caged session", which this
//          bullet used to say and the clause after it already contradicted:
//          a `cage: seatbelt` seat's `bd` is the rendered gate shim, which
//          carries no `--no-db` at all — measured from inside a seatbelt
//          seat, `grep -c no-db $RHQ_GATES_DIR/bin/bd` -> 0
//          (ranger-base-43ux4);
//       3. `BD_NO_DB=true` in the environment (measured: it flips a plain
//          `bd create` with nothing in `config.yaml` to see);
//       4. a bd built without CGO falling back with a note on stdout — which
//          is what 0.50.3's default `--backend dolt` does here (ASSUMED as a
//          door; the other three are measured).
//     A store that merely lacks a database is NOT a door: 0.50.3 builds a
//     SQLite `beads.db` over a bare jsonl on the first plain read, so "jsonl
//     only" is a transient state.
//     THE FIX IS THE LAUNCH ENV, not this file (ADR 0055 D1): every session
//     posse launches carries `BEADS_DIR=beadsHome(dir)` (planLaunch,
//     herdrback.go), forwarded into the container by name (CageEnvNames).
//     Measured on a SINGLE-PREFIX scratch store: with it set, the no-db
//     create from the worktree lands in the MAIN store and the worktree's
//     `bd list` reads it — on the `no-db: true` store and on the `--no-db`
//     invocation over a database-class store alike. The store class is not
//     the only thing that decides this, and the store this shop runs on is
//     the counter-example: pointing `BEADS_DIR` at a store whose issues
//     carry more than one prefix, with `issue-prefix` unset in its
//     `config.yaml`, makes a `--no-db` bd REFUSE to start rather than fork
//     — "failed to detect prefix: issues have mixed prefixes" on stderr,
//     exit 1, no rows (measured 2026-09-04, bd 0.50.3, against this
//     instance's own store of record, whose rows carry two different id
//     prefixes with neither declared). Read the printed line and not `$?`
//     through a pipe: piped to `head` it looks like exit 0. That is a loud
//     failure and not a silent fork, so it is the better of the two, but it
//     is a second refusal mode ADR 0055's Consequences do not enumerate, and
//     its remedy is the instance's: `issue-prefix` in the store's
//     `config.yaml`. Both halves are routed to the architecture lane as
//     ranger-base-jl8q2. Latent here — every PID on this box is
//     `cage: seatbelt`, whose bd carries no `--no-db` (door 2 above).
//     What this file still cannot do is fix it for a bd run with
//     `BEADS_DIR` shed (`env -u`, `env -i`): the resolution is bd's, and
//     seedBeadsRedirect is already naming the right directory.
//
//   - `git merge --ff-only <branch>` in the main checkout succeeds with
//     unrelated uncommitted changes present, and refuses rather than
//     clobbering when they collide — so the operator's dirty tree is not a
//     reason to skip the merge, and never a reason it loses work.
//
// PLACEMENT IS OURS TO ENFORCE, because bd's net is PARTIAL — not because
// it is absent (ranger-base-9ypc corrects rangerhq-80fx, which said the
// latter and fails dangerous). Measured on bd 0.49.1 with a $HOME control on
// every arm: `FindBeadsDir` runs BEADS_DIR through `CanonicalizePath`, which
// EvalSymlinks it BEFORE `isPathInSafeBoundary` judges it, so /tmp is judged
// as /private/tmp and /private IS in unsafePrefixes — every tmp BEADS_DIR is
// refused, on the worktree-create arm included. But only the ~50 commands
// that call GetRepoContext ask at all: `bd list`/`bd status` accept /tmp,
// /var/tmp, even /etc. And `bd worktree create /tmp/<name>` succeeds while
// writing a redirect that does NOT resolve — the relative path is computed
// from the unresolved /tmp target and the tree lands at /private/tmp, one
// component deeper — which stays silent because FindBeadsDir's worktree
// branch reaches the main repo through git, never the redirect. Session
// worktrees go under $HOME because a session scratchpad is REAPED, and a
// reaped worktree under a live session destroys the work in it. WorktreeRoot
// refuses anything outside $HOME rather than trusting a net that holds only
// where someone happens to look.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SessionTree is one dispatched session's private checkout: its own working
// tree, its own index, its own HEAD, and the branch the launcher merges back.
type SessionTree struct {
	Repo   string // the main checkout the worktree hangs off
	Path   string // this session's working tree
	Branch string // the branch checked out in it
	Base   string // the branch it was cut from — the merge target, recorded on the branch
	Bead   string // the bead it was cut for, recorded on the branch ("" = not recorded; SessionTreesIn fills it)
}

// SessionBranch is the branch a session's worktree checks out. The session
// name already carries persona, repo basename and bead id (SessionForBead),
// so the branch is derivable from the name alone — which is what lets a kill
// or a merge find the branch with nothing but the meta.
func SessionBranch(session string) string { return "posse/" + session }

// SessionOfBranch is that inverse, and it is one line rather than a parse:
// the prefix is the whole of what SessionBranch adds, and a session name may
// hold anything a workspace label may. A branch that is not a session
// branch comes back unchanged, which is the only honest answer — no caller
// here has one.
func SessionOfBranch(branch string) string { return strings.TrimPrefix(branch, "posse/") }

// DefaultWorktreeRoot is where session worktrees live when config does not
// say. Not under RHQ_HOME: that is `~/.config/...`, which the runtimes'
// own classifiers treat as configuration and refuse to let a persona write
// in — and this is the directory the persona does ALL of its work in.
func DefaultWorktreeRoot() string {
	return filepath.Join(os.Getenv("HOME"), ".posse", "worktrees")
}

// WorktreeRoot resolves config `worktrees:` and enforces the one placement
// rule this feature owns: under $HOME. A root anywhere else is a config
// error and refuses, rather than quietly putting a persona's only copy of
// its work somewhere a reaper walks.
func (a *App) WorktreeRoot() (string, error) {
	def := a.WorktreeRootDefault
	if def == "" {
		def = DefaultWorktreeRoot()
	}
	root := ExpandTilde(a.CfgGet("worktrees", def))
	if !filepath.IsAbs(root) {
		return "", Die("worktrees: %s is not an absolute path", root)
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", Die("worktrees: $HOME is unset, so the under-$HOME rule cannot be checked")
	}
	if !pathUnder(root, home) {
		return "", Die("worktrees: %s is outside $HOME — session worktrees hold work nothing else has a copy of, and a reaped worktree under a live session is the failure this rule exists for", root)
	}
	return root, nil
}

// pathUnder answers whether p is home or inside it, comparing the deepest
// existing ancestor of each with symlinks resolved — /tmp and /var are
// symlinks on macOS, and a textual prefix test reads those as outside.
func pathUnder(p, home string) bool {
	rp, rh := resolveExisting(p), resolveExisting(home)
	if rp == rh {
		return true
	}
	return strings.HasPrefix(rp, strings.TrimSuffix(rh, string(filepath.Separator))+string(filepath.Separator))
}

// SessionTreePath is where a session's worktree goes: one directory per
// repo basename, one worktree per session inside it. Two repos with the
// same basename collide here — and they already collide in the session name
// itself (SessionFor uses the basename) and in the flat meta dir, so this
// adds no collision class that posse did not already have.
func (a *App) SessionTreePath(repo, session string) (string, error) {
	root, err := a.WorktreeRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.Base(repo), session), nil
}

// ─── git plumbing ────────────────────────────────────────────────────────────

// git runs a git command in dir and returns its trimmed stdout. The error
// carries git's stderr, which is the only part of a git failure worth
// reading. A caller that parses a porcelain format wants gitRaw
// (promote.go) instead — see dirtyPaths.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(out.String()), Die("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// MainCheckout answers the MAIN working tree of the repo dir belongs to, and
// false when dir is not a git repo at all. A linked worktree resolves to the
// checkout it hangs off, so session worktrees never nest.
func MainCheckout(dir string) (string, bool) {
	common, err := git(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(dir, common)
	}
	// <main checkout>/.git → the main checkout. A bare repo has no working
	// tree to hang a session off, and its common dir has no such parent to
	// mean anything, so it is refused by the HEAD check below either way.
	//
	// Returned in the caller's own spelling, NOT resolved: from the main
	// checkout git answers ".git" and the operator's path survives the join,
	// while from a linked worktree git answers an absolute path it has
	// already resolved through symlinks. So the same repo can come back
	// under two spellings depending on which tree asked — harmless, since
	// every caller either hands the answer to `git -C` or prints it, but
	// it means two answers must be compared with resolveExisting and never
	// as strings. Resolving here instead was measured on ranger-base-q5p1
	// and rejected: the answer is written into each session tree's
	// `.beads/redirect`, and normalizing it rewrites an operator-visible
	// file to buy nothing outside a symlinked checkout.
	return filepath.Clean(filepath.Dir(common)), true
}

// LinkedGitDirs names the git directories a session writes that are NOT
// under its own root, and only a linked worktree has any: a worktree's
// `.git` is a FILE pointing at `<repo>/.git/worktrees/<name>`, where its
// index, HEAD and lock live, and objects and refs land in the common dir
// beside it. Every write boundary posse draws around a session dir — the L2
// seatbelt profile, codex's `--add-dir` roots — has to name these two or the
// persona cannot commit in its own tree at all. In the main checkout `.git`
// is inside the root already, so the answer is empty and nothing is widened.
//
// Resolved the way the rest of the package resolves git dirs (join a
// relative answer against dir) rather than through --absolute-git-dir or
// --path-format, which are newer flags than the floor this code holds to.
func LinkedGitDirs(dir string) []string {
	one := func(flag string) string {
		out, err := git(dir, "rev-parse", flag)
		if err != nil || out == "" {
			return ""
		}
		if !filepath.IsAbs(out) {
			out = filepath.Join(dir, out)
		}
		return filepath.Clean(out)
	}
	gd, cd := one("--git-dir"), one("--git-common-dir")
	if gd == "" || cd == "" || resolveExisting(gd) == resolveExisting(cd) {
		return nil
	}
	return []string{gd, cd}
}

// launchWritableRoots names every directory a self-sandboxing runtime has to
// be TOLD it may write, because its own sandbox confines writes to the
// workspace it starts in:
//
//   - the store of record, which ADR 0012 D3-C usually puts outside the
//     session dir — <dir>/.beads holds a redirect and the database lives in
//     the instance repo it names. Unnamed, every `bd close` and
//     `bd comments add` is denied and the session goes silent
//     (ranger-base-0fb, five of them before anyone read the silence as a
//     cage rather than an agent skipping its bookkeeping).
//   - that store repo's git dirs, when the redirect leaves the session dir.
//     `bd sync` COMMITS the JSONL there, so it takes index.lock in the
//     per-worktree git dir and reads hooks and refs in the common one; with
//     the .beads granted and its git dirs not, `bd sync` and `bd export`
//     die on the lock exactly as they did under the pre-23c4e54 seatbelt
//     (measured, ranger-base-rhw; this call site, ranger-base-xqwr). Same
//     resolver and same two dirs SeatbeltWritable already grants at L2,
//     under the same condition — a store already inside the workspace needs
//     no grant, and <dir>/.git in a linked worktree is a FILE, not a root.
//   - in a session worktree, this tree's own git dirs, which hold its index
//     and the repo's objects and sit outside the tree (rangerhq-09o2).
//
// THE TRADE, stated rather than closed: --add-dir is directory-granular, so
// naming the store repo's git dirs grants its refs, hooks and config whole.
// ADR 0013 §4 and sessionGitGrants already accept that gap for the session's
// own repo and say so; this extends the same accepted trade to the store's,
// and nothing narrower is available at this wall — the flag cannot name a
// ref (ranger-base-xqwr).
//
// One function because three callers must agree: planLaunch renders the line
// the session runs, renderedLaunchLine renders the line ADR 0013 §4's
// reachability row JUDGES, and RelaunchAgent renders the line a session whose
// CLI died comes back on. Two spellings of "the same roots" is a row that
// passes a line nobody launches, or refuses one that would have worked — and
// the relaunch site is the one the row cannot cover at all, since it runs at
// CheckParity time against the launch line (ranger-base-qdtw).
func launchWritableRoots(dir string) []string {
	home := beadsHome(dir)
	roots := append([]string{home}, LinkedGitDirs(dir)...)
	if home != "" && !underDir(dir, home) {
		roots = append(roots, beadsGitDirs(home)...)
	}
	return dedupeStrings(roots)
}

// repoBranch is the branch checked out in the main repo — the base a session
// branch is cut from and the target the launcher merges back into. "" means
// a detached HEAD, which has no merge-back and so gets no worktree.
func repoBranch(repo string) string {
	b, err := git(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return b
}

// orDetached names a base for a human when there may not be one: a session
// branch cut before posseBase was recorded has no answer, and saying so
// beats printing an empty string mid-sentence.
//
// Every sentence that renders a base goes through it — the persona's work
// prompt, the pass's tree and merge-back lines, `posse worktrees`, the
// merge-back bead's own title and body, and the settle-open escalation's
// tree line (ranger-base-82d9). Base == "" is reachable at all of them
// (ranger-base-nfgh), and the ones that skipped it read
// "never merge to  yourself".
func orDetached(base string) string {
	if base == "" {
		return "the branch it was cut from"
	}
	return base
}

func branchExists(repo, branch string) bool {
	_, err := git(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// baseKey names where a session branch records the branch it was CUT from.
// It is git config on the branch, not a field in the run record, for the
// reason SessionTreesIn reads git too: a kill that could not land its work
// removes the session's meta and leaves the tree standing, so the one record
// that survives every path is the one git keeps. `git branch -d` takes the
// branch's config with it, so retiring a tree leaves nothing behind.
func baseKey(branch string) string { return "branch." + branch + ".posseBase" }

// recordBase writes down the branch the session branch was cut from, at the
// moment it is cut — the only moment the answer is known for certain.
func recordBase(repo, branch, base string) error {
	if base == "" {
		return nil
	}
	_, err := git(repo, "config", baseKey(branch), base)
	return err
}

// beadKey names where a session branch records the bead it was cut FOR — the
// run record ADR 0011 §3 asks for, kept where baseKey is kept and for the
// same reason (ranger-base-nurl). The session meta already carries `bead:`,
// and it is the wrong record for the one question the landing sweep asks:
// the meta is removed by a kill and by a clearDeadMeta, and both of those
// leave the tree and its branch standing. A pointer that disappears exactly
// when the work is stranded cannot be what finds stranded work.
//
// It is a POINTER and never a status, on the same rule as the meta's
// (ADR 0011): what it names is the bead whose store is then asked whether it
// is closed, so nothing here can disagree with the store of record.
func beadKey(branch string) string { return "branch." + branch + ".posseBead" }

// recordBead points a session branch at its bead. Written at every launch
// into the tree rather than only when the branch is cut, because the
// pre-Dial-F slot session (SessionFor(persona, dir)) is reused across beads
// and its pointer moves with it — the same semantics NoteBead gives the
// meta's copy. An empty id clears nothing: an interactive relaunch into a
// bead session must not erase which bead the branch holds.
func recordBead(repo, branch, id string) error {
	if id == "" || branch == "" {
		return nil
	}
	_, err := git(repo, "config", beadKey(branch), id)
	return err
}

// beadOf answers which bead a session branch's commits belong to, or "" when
// nothing recorded it — every branch cut before this landed, and every tree
// made by hand. "" is not a bead that is open: the sweep says out loud that
// it cannot tell rather than landing on a guess (landsweep.go).
func beadOf(repo, branch string) string {
	out, err := git(repo, "config", "--get", beadKey(branch))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// baseOf answers the branch a session's work must land on: the one it was
// cut from. Reading the repo's CURRENT branch instead was the bug in
// ranger-base-5s2o — an operator who switches branches while a persona works
// does not move the target, they just make it temporarily unreachable, and
// merge-back has to say so rather than land the work wherever HEAD happens
// to be.
//
// fallback is what a branch cut before this was recorded gets: the repo's
// own branch, which is the answer that shape always gave. Nothing can
// recover a legacy branch's true base after the fact, and refusing to land
// every tree that predates the fix would strand work that is already in
// flight.
func baseOf(repo, branch, fallback string) string {
	if out, err := git(repo, "config", "--get", baseKey(branch)); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b
		}
	}
	return fallback
}

// ─── the detached head a caged session works on (ranger-base-t4f1) ──────────

// detachedKey names where a session branch records that its tree was launched
// on a DETACHED HEAD. Kept in git config on the branch, beside baseKey and
// beadKey and for their reason: the landing sweep has to answer this about a
// tree whose session meta a kill already removed, and `git branch -d` takes
// the record with the branch so a retired tree leaves nothing behind.
//
// It exists because the SPLICE below and the ranger-base-dybv guard read the
// same fact — a worktree HEAD that is off its own branch — and mean opposite
// things by it. Designed-detached is the container tier working as built and
// its work is put back on the branch; accidental is the failure dybv measured
// twice in the field, where a close reported success over commits no ref
// reaches. Without a record the splice would silence the guard for every
// session, which is the one thing it must not do.
func detachedKey(branch string) string { return "branch." + branch + ".posseDetached" }

// recordDetached writes (or clears) that record. Written at every launch into
// the tree rather than only the first, because the tier is a property of the
// LAUNCH: the same tree is relaunched at another cage tier when the PID
// changes, and a record that only ever went on would keep splicing for a
// session posse never detached.
func recordDetached(repo, branch string, on bool) error {
	if branch == "" {
		return nil
	}
	if !on {
		// `--unset` on a key that is not there exits 5. Nothing to clear is
		// not a failure to clear.
		if _, err := git(repo, "config", "--get", detachedKey(branch)); err != nil {
			return nil
		}
		_, err := git(repo, "config", "--unset", detachedKey(branch))
		return err
	}
	_, err := git(repo, "config", detachedKey(branch), "1")
	return err
}

// launchedDetached answers whether posse put this session's tree on a
// detached HEAD. False for every branch cut before this landed and every tree
// made by hand — which is the safe default in both directions: no splice, and
// the dybv guard still reports an off-branch HEAD.
func launchedDetached(repo, branch string) bool {
	out, err := git(repo, "config", "--get", detachedKey(branch))
	return err == nil && strings.TrimSpace(out) == "1"
}

// treeDetachedHead is the tree's own HEAD sha when that tree exists and its
// HEAD is on no branch, ("", false) otherwise. Asked of the TREE and not of
// workHead, which deliberately falls back to the branch ref for a retired
// tree — here the fallback would read a tree that is gone as a detached one
// and splice a branch onto itself.
func treeDetachedHead(path string) (string, bool) {
	sha, err := git(path, "rev-parse", "HEAD")
	if err != nil || sha == "" {
		return "", false
	}
	if b, err := git(path, "symbolic-ref", "--quiet", "HEAD"); err == nil && b != "" {
		return "", false
	}
	return sha, true
}

// spliceDetachedWork puts a detached tree's commits back on its session
// branch — the exact command landed() prescribes to a human
// (`git -C <tree> branch -f <branch> HEAD`), run by the launcher for the
// sessions it detached on purpose.
//
// FAST-FORWARD ONLY, and that is not politeness: `branch -f` is a ref write
// with no ancestry check, so a branch tip the tree's HEAD does not reach is
// work this would DELETE. It cannot happen through posse's own paths — the
// branch of a detached tree moves only through this function — so the refusal
// is about the case nothing here controls (an operator's `branch -f`, a
// stale record on a reused branch), and it leaves the dybv guard to report
// the disagreement in the words it already has.
//
// A no-op on every shape that is not a designed detach: a tree that is gone,
// a HEAD on a branch, a branch already at HEAD.
func spliceDetachedWork(t *SessionTree) error {
	head, ok := treeDetachedHead(t.Path)
	if !ok {
		return nil
	}
	if tip := refSHA(t.Repo, "refs/heads/"+t.Branch); tip != "" {
		if tip == head {
			return nil
		}
		if !reaches(t.Repo, head, tip) {
			return Die("%s is at %s, which does not reach %s's tip %s — `branch -f` there would delete work, so the splice is refused",
				AbbrevHome(t.Path), abbrevSHA(head), t.Branch, abbrevSHA(tip))
		}
	}
	_, err := git(t.Path, "branch", "-f", t.Branch, "HEAD")
	return err
}

// PrepareSessionHead decides, at every launch into a session worktree,
// whether HEAD sits ON the session branch or off it — because at the
// container tier the answer is what the git grant is made of
// (ranger-base-t4f1, closing ranger-base-6q5e).
//
// A caged session's common-dir mount is `:ro` with read-write overlays of
// `worktrees/<own>`, `objects` and `logs` and NOTHING under `refs`
// (sessionCommonDirWrites, cage.go): the ref half of L2's grant is not a
// mount list, since a bind mount's source must exist and git creates
// `refs/heads/<branch>.lock` at commit time. On a detached HEAD there is no
// ref to write — a commit moves the per-worktree HEAD and lands its objects
// and reflog, all three inside the overlays, measured at L2 in
// seatbeltworktreegit_qa_test.go — so detaching is what buys the narrowing
// rather than costing the session its commit.
//
// Both directions, deliberately. The tier is a property of the launch and the
// tree outlives it: a PID that drops `cage: container`, or a `--cage seatbelt`
// relaunch, would otherwise inherit a detached HEAD forever and take the dybv
// guard's sensitivity with it. So an uncaged launch into a tree posse
// detached splices the work back and checks the branch out again.
//
// Errors REFUSE the launch, and the asymmetry is the point: a caged session
// whose HEAD could not be detached, or whose `logs/` could not be made,
// cannot commit at all inside the cage it is about to enter, and a persona
// discovering that at its first commit is worse than a launch that does not
// happen. The re-attach direction only warns — the work is already on the
// branch by then and the session runs fine off a detached HEAD.
//
// It runs where planLaunch resolves the tier, which on the relaunch path is
// the preflight — outside the launcher lock, and before the old session is
// killed (relaunch.go). That is the same position EnsureSessionTree already
// occupies, and the same reason makes it safe: everything written here is
// this session's own — its tree's HEAD, its own branch's ref and its own
// branch's config — never a store another launcher shares.
//
// The one case worth naming: a relaunch re-enters the plan with the tier the
// meta RECORDED (RecreateOpts passes m.Cage), so it normally finds the tree
// already in the state it wants and does nothing but move the branch up to
// the work. It can differ only when the PID's `cage:` was raised between two
// launches of one session name, and then the detach lands while the old
// session is still alive in that tree — a session this pass is about to kill,
// whose commits the splice keeps either way. Not free, and small enough to
// state rather than restructure the launch around.
func PrepareSessionHead(t *SessionTree, caged bool, warn io.Writer) error {
	if t == nil || t.Path == "" || t.Branch == "" {
		return nil
	}
	if !caged {
		if !launchedDetached(t.Repo, t.Branch) {
			return nil
		}
		if err := spliceDetachedWork(t); err != nil {
			fmt.Fprintf(warnw(warn), "posse: %s keeps a detached HEAD — %v\n", AbbrevHome(t.Path), err)
			return nil
		}
		if _, ok := treeDetachedHead(t.Path); ok {
			if _, err := git(t.Path, "checkout", t.Branch); err != nil {
				fmt.Fprintf(warnw(warn), "posse: %s stays on a detached HEAD (%v) — its work is on %s and lands from there\n", AbbrevHome(t.Path), err, t.Branch)
				return nil
			}
		}
		if err := recordDetached(t.Repo, t.Branch, false); err != nil {
			fmt.Fprintf(warnw(warn), "posse: %s is back on %s but the record did not clear (%v)\n", AbbrevHome(t.Path), t.Branch, err)
		}
		return nil
	}
	// `logs/` is one of the three overlays and a read-write bind of a source
	// that does not exist is DROPPED by cageOverlay's Stat guard, which would
	// leave `<common>/logs` `:ro` inside the cage. NOT because the session's
	// commit needs it — measured, and it does not: a detached commit's reflog
	// is the per-worktree `worktrees/<own>/logs/HEAD` (probe arms A5/A5b,
	// docs/adr/0014-l4-worktree-narrowing.probe.sh). What the mkdir buys is a
	// rendered mount set that does not depend on whether this repo has ever
	// updated a shared ref, and a cage where a git operation that does update
	// one gets its reflog rather than a fatal.
	dirs := LinkedGitDirs(t.Path)
	if len(dirs) != 2 {
		return Die("%s is not a linked worktree (git names no common dir), so the container tier's narrowed git grant cannot be built for it", AbbrevHome(t.Path))
	}
	if err := os.MkdirAll(filepath.Join(dirs[1], "logs"), 0o755); err != nil {
		return Die("session worktree: %s has no logs/ and one could not be made (%v) — the caged session would reach a :ro reflog for every shared-ref update it makes", AbbrevHome(dirs[1]), err)
	}
	// `config.worktree` is the one file in ADR 0038 decision 4b's `:ro`
	// bind set that does not exist yet: posse never sets
	// `extensions.worktreeConfig`, so no live worktree carries one, and the
	// deny direction's Stat (cageOverlayFile, cage.go) DROPS a bind whose
	// source is absent — which would leave the path creatable by the
	// session, under the read-write `worktrees/<own>` overlay, for the
	// repo where the operator did turn the extension on. Not the Stat's
	// fault and not fixable there: a `-v` bind of an absent source is not
	// refused by the engine, it creates the source as a DIRECTORY, and a
	// `config.worktree` directory makes every git command in the tree
	// fatal ("unknown error occurred while reading the configuration
	// files", MEASURED 2026-09-05, git 2.50.1, ranger-base-n3ywd). So the
	// launcher makes the source, like `logs/` above, and the wall is
	// unconditional instead of keyed on a config key the session cannot
	// reach anyway.
	//
	// An EMPTY file is inert in both directions (MEASURED): with the
	// extension off git never reads it, and with it on there are no keys
	// to read. Never truncating an existing one — that would be posse
	// deleting the operator's own per-worktree config.
	if cw := filepath.Join(dirs[0], "config.worktree"); !fileExists(cw) {
		f, err := os.OpenFile(cw, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return Die("session worktree: %s could not be made (%v) — the caged session's :ro bind of it would be dropped for want of a source, leaving the file that selects this tree's extra config writable inside the cage", AbbrevHome(cw), err)
		}
		if err := f.Close(); err != nil {
			return Die("session worktree: %s was made but not closed (%v)", AbbrevHome(cw), err)
		}
	}
	if _, ok := treeDetachedHead(t.Path); ok {
		// A relaunch into a tree this launcher already detached. Move the
		// branch up to the work FIRST — a kill between two caged sessions
		// would otherwise leave the commits of the first on no ref — and
		// leave HEAD where it is.
		if err := spliceDetachedWork(t); err != nil {
			fmt.Fprintf(warnw(warn), "posse: %s\n", err)
		}
	} else if _, err := git(t.Path, "checkout", "--detach"); err != nil {
		return Die("session worktree: %s could not be detached (%v) — a caged session commits on a detached HEAD and nothing else in this tier's git grant can write a ref", AbbrevHome(t.Path), err)
	}
	if err := recordDetached(t.Repo, t.Branch, true); err != nil {
		return Die("session worktree: %s was detached but the record did not stick (%v) — the close would read the tree's HEAD as an accidental detach and refuse to land it", AbbrevHome(t.Path), err)
	}
	return nil
}

// ─── creating a session's tree ───────────────────────────────────────────────

// PlanSessionTree answers WHERE a session's tree would be, without making
// anything. It exists because dispatch has to tell the persona which tree
// and which branch it is working in — in a work prompt written before the
// launch, on the argv path — and a predicted answer that the launch then
// contradicts is worse than no answer. Same predicate, one function, called
// twice: the plan and the launch cannot disagree.
//
// nil, nil is the honest "no worktree here": a dir that is not a git repo,
// or a repo on a detached HEAD with no session branch to return to (nothing
// to cut from, nothing to merge back into). Both fall back to the shared
// checkout, which is what posse did before this landed.
func (a *App) PlanSessionTree(dir, session string) (*SessionTree, error) {
	repo, ok := MainCheckout(dir)
	if !ok {
		return nil, nil
	}
	base := repoBranch(repo)
	branch := SessionBranch(session)
	// A detached checkout is only fatal to a tree this launch would have to
	// CUT: with no branch under HEAD there is no base to cut from. A session
	// branch that already exists needs none — it was cut from a base
	// recorded on the branch itself, and it is still that session's private
	// tree whether or not the operator can take its work today. Answering
	// nil here was ranger-base-q5p1: a relaunch while the operator bisected
	// blanked the recreated record's repo/branch, and close and kill then
	// read a live private tree as a shared checkout and skipped its landing
	// entirely. Deferring the merge-back is the honest cost of a detached
	// HEAD (MergeSessionWork says so in words); forgetting where the work
	// is, is not.
	if base == "" && !branchExists(repo, branch) {
		return nil, nil
	}
	path, err := a.SessionTreePath(repo, session)
	if err != nil {
		return nil, err
	}
	// A branch that already exists was cut from a base recorded then, and
	// that is where its work lands. For one this launch is about to cut, the
	// repo's branch IS the base and baseOf answers it.
	return &SessionTree{Repo: repo, Path: path, Branch: branch, Base: baseOf(repo, branch, base)}, nil
}

// EnsureSessionTree gives the session its own working tree, index and HEAD,
// and is idempotent: a relaunch, a resume, or a second pass over the same
// bead lands in the tree that already exists.
//
// It returns (nil, nil) — no worktree, the shared checkout, a line on warn —
// for the cases PlanSessionTree names. Fail-open is deliberate: a launch must
// not die because the operator is bisecting. Every other failure is an error,
// because a worktree that was asked for and half-made is not a state to
// launch a persona into.
func (a *App) EnsureSessionTree(dir, session string, warn io.Writer) (*SessionTree, error) {
	t, err := a.PlanSessionTree(dir, session)
	if err != nil {
		return nil, err
	}
	if t == nil {
		if repo, ok := MainCheckout(dir); ok {
			fmt.Fprintf(warnw(warn), "posse: %s has a detached HEAD — %s launches in the SHARED checkout (no session worktree, no merge-back)\n", AbbrevHome(repo), session)
		}
		return nil, nil
	}
	// The tree survives a detached checkout; only the landing waits for one.
	// Said here because this is where the operator is looking — the merge
	// itself may not be attempted for hours (ranger-base-q5p1).
	if repoBranch(t.Repo) == "" {
		fmt.Fprintf(warnw(warn), "posse: %s has a detached HEAD — %s keeps its own tree on %s; its work lands on %s once that branch is checked out there (posse worktrees --land)\n",
			AbbrevHome(t.Repo), session, t.Branch, orDetached(t.Base))
	}

	if have, err := existingTree(t); err != nil {
		return nil, err
	} else if have {
		// Seeding is idempotent and re-run on purpose: a redirect the
		// operator deleted, or a link added to config since the tree was
		// made, is repaired on the next launch into it.
		return t, seedTree(t, a)
	}
	if err := os.MkdirAll(filepath.Dir(t.Path), 0o755); err != nil {
		return nil, Die("session worktree: %v", err)
	}
	// A worktree dir removed by hand leaves a registration behind, and git
	// then refuses to re-add the path. Pruning first is what makes this
	// idempotent across an operator's `rm -rf`.
	_, _ = git(t.Repo, "worktree", "prune")
	add := []string{"worktree", "add", t.Path, t.Branch}
	cut := !branchExists(t.Repo, t.Branch)
	if cut {
		add = []string{"worktree", "add", "-b", t.Branch, t.Path, t.Base}
	}
	if _, err := git(t.Repo, add...); err != nil {
		return nil, err
	}
	if cut {
		// Written here and nowhere else: this is the one moment the branch's
		// base is a fact rather than a guess about the operator's checkout.
		if err := recordBase(t.Repo, t.Branch, t.Base); err != nil {
			return nil, err
		}
	}
	return t, seedTree(t, a)
}

// existingTree answers whether path is already this session's worktree. A
// path that exists but is something else is an error, never a thing to
// overwrite: it is either another repo's worktree or a directory somebody
// meant to keep.
func existingTree(t *SessionTree) (bool, error) {
	if _, err := os.Stat(t.Path); err != nil {
		return false, nil
	}
	got, ok := MainCheckout(t.Path)
	if !ok {
		return false, Die("session worktree: %s exists and is not a git worktree", t.Path)
	}
	if resolveExisting(got) != resolveExisting(t.Repo) {
		return false, Die("session worktree: %s belongs to %s, not %s", t.Path, got, t.Repo)
	}
	head, err := git(t.Path, "symbolic-ref", "--short", "HEAD")
	if err != nil || head != t.Branch {
		return false, Die("session worktree: %s is on %q, not %q", t.Path, head, t.Branch)
	}
	return true, nil
}

// seedTree gives the fresh checkout the two things the main checkout has
// that git does not carry: the beads redirect, so the work graph does not
// fork, and whatever gitignored paths the operator declared in
// `worktree_link:`.
func seedTree(t *SessionTree, a *App) error {
	if err := seedBeadsRedirect(t); err != nil {
		return err
	}
	return seedWorktreeLinks(t, a)
}

// seedBeadsRedirect points the worktree's `.beads` at the ONE database the
// repo already uses, resolving the main checkout's own redirect first so a
// chain is never built. Absolute, because bd's relative form is measured to
// resolve against the worktree ROOT and not against `.beads/` — one `..` off
// and bd warns once and silently falls back to a stale path (rangerhq-09o2).
// An absolute path has no such arithmetic to get wrong.
//
// Who reads it, since bd does not — not on 0.49.1 and not on 0.50.3 against
// the SQLite store posse runs on, where bd resolves the worktree to the main
// checkout by itself (see the CORRECTED and WHICH VERSION bullets above; a
// no-db store does read this file, and then answers from the worktree's own
// jsonl regardless, so it is no help there either): POSSE does. beadsHome (beadloss.go) resolves this file, and the seatbelt
// writable set and the codex launch line are built from what it answers
// (ADR 0012 D3-C). A worktree with no redirect leaves beadsHome answering
// the worktree's own `.beads`, so a caged persona is granted a directory bd
// never opens and denied the one it does — `failed to open database: …
// operation not permitted`, which is ranger-base-0fb verbatim, out of a bd
// resolution that was correct all along. So this is not belt-and-braces for
// the graph on today's bd; it is the cage's only account of where the store
// is, and it stays the belt for a bd that loses the worktree resolution.
func seedBeadsRedirect(t *SessionTree) error {
	src := filepath.Join(t.Repo, ".beads")
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return nil // no beads in this repo: nothing to keep unforked
	}
	target := src
	// isRegularFile (gates.go) before the open, the same guard the launch
	// path's other readers of this file carry (ranger-base-gs9r,
	// ranger-base-92n5p, ranger-base-fvfve): os.ReadFile on a FIFO with no
	// writer never returns, and this read is ABOVE all of theirs — seedTree
	// runs from EnsureSessionTree, which HerdrBackend calls before anything
	// else reads `dir`, so before planLaunch and before both
	// InstallCommitGuardHook and CheckParityIn. One mkfifo in a checkout
	// wedged every dispatched launch into it with nothing printed and no
	// deadline anywhere above (ranger-base-xc2s4). A special file is never a
	// redirect posse wrote and cannot be one, so it gets the answer every
	// other unreadable redirect already gets here: target stays src, reached
	// without the open.
	redirect := filepath.Join(src, "redirect")
	if isRegularFile(redirect) {
		if b, err := os.ReadFile(redirect); err == nil {
			if p := strings.TrimSpace(string(b)); p != "" {
				if !filepath.IsAbs(p) {
					p = filepath.Join(t.Repo, p)
				}
				target = filepath.Clean(p)
			}
		}
	}
	// os.MkdirAll Stats each component, so this still FOLLOWS a symlink at
	// <tree>/.beads itself and seeds into whatever directory it points at. That
	// is the residual of the write below, one component up, measured against
	// this fix and filed as ranger-base-42au9 (P3, security) rather than folded
	// in: the basename is fixed at "redirect" and the content at the repo's own
	// .beads path, so it creates or clobbers a file NAMED redirect and reaches
	// none of the classes ranger-base-d14e1 did. Cited here so the next reader
	// of this line knows it was looked at and bounded, not missed.
	dst := filepath.Join(t.Path, ".beads")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return Die("session worktree beads redirect: %v", err)
	}
	// And the WRITE, which is of the same class and was cleared as "the WRITE,
	// not a read" by the census above (ranger-base-lwfhe, escaping
	// ranger-base-xc2s4). It gets a stronger answer than the read's, because it
	// has a stronger problem: this destination is the SESSION's own worktree,
	// which every caged session can write by construction, and the writer is the
	// LAUNCHER, outside the cage. So the session chooses the file at that path
	// and an uncaged process writes to it — the file's TYPE (a FIFO: os.WriteFile
	// opens O_WRONLY|O_CREATE|O_TRUNC, and open(2) for write on a pipe with no
	// reader blocks exactly as open for read on one with no writer does, so one
	// mkfifo wedged every later dispatched launch into that tree with nothing
	// printed and no deadline above, ranger-base-lwfhe) and, through a symlink,
	// its PATH (a guard that Stats answers true for a symlink to a regular file
	// and the write goes THROUGH it, out of the tree and into any file the
	// launcher's uid can write — ranger-base-d14e1, P1, measured as a clobber of
	// the constitution/gate class ADR 0002 §3 walls off from persona writes, and
	// a dangling one CREATES a file out there).
	//
	// Both fall to one primitive, and it is not a check: NEVER OPEN THE
	// DESTINATION — the security lane's ruling on ranger-base-d14e1, carried out
	// here under ranger-base-aojiu. Write the bytes to a sibling temp (a fresh
	// inode, O_CREATE|O_EXCL, inside dst so the rename stays within one
	// filesystem) and rename it over the name. rename(2) replaces the
	// destination's last component without following it and without opening it:
	// a symlink is REPLACED and the file it pointed at is untouched, a FIFO is
	// replaced without the open that blocked, a directory errors (file exists)
	// so Die returns rather than hanging. All three MEASURED, 2026-09-05.
	//
	// It closes a fourth shape of the same class for free, which is the sign
	// the primitive is right rather than merely sufficient: a session that
	// chmods its own redirect 0444 killed every
	// later relaunch into that tree with "permission denied" on both earlier
	// revisions, because the write opened the file; a rename needs write on the
	// DIRECTORY, not on the file it replaces. An lstat guard would have closed
	// the symlink arm and left a check-then-write window a session's leftover
	// background process can race; a rename has no window. O_NOFOLLOW is the
	// wrong primitive here for the same reason the guard was — it refuses the
	// symlink (ELOOP, measured) and still OPENS a FIFO.
	//
	// isRegularFile is deliberately left on os.Stat at all five of its call
	// sites, the read above included — the security lane's census on
	// ranger-base-d14e1. They are READ guards asking "is what I would open
	// regular", where following the link is the point: os.Stat catches a symlink
	// to a FIFO and os.Lstat would not, and refuseNonRegularHook documents that a
	// symlinked hook installs by design. This write was the only site of its
	// class under the session tree (line 691 is O_EXCL; seedWorktreeLinks is
	// Lstat+Symlink).
	//
	// Rename-over-a-temp is already how this package replaces a file it must not
	// leave half-written: replaceMeta (herdrback.go), plancache.go ×2,
	// modelavail.go, trust.go, refresh.go. As there, a crash between create and
	// rename leaves litter rather than a wrong file; the temp is dot-prefixed so
	// nothing reading this directory for a redirect can mistake one for it.
	seeded := filepath.Join(dst, "redirect")
	f, err := os.CreateTemp(dst, ".redirect-*")
	if err != nil {
		return Die("session worktree beads redirect: %v", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename has taken it
	if _, err := f.WriteString(target + "\n"); err != nil {
		f.Close()
		return Die("session worktree beads redirect: %v", err)
	}
	// CreateTemp opens at 0600; the seeded redirect has been 0644 since this
	// function first wrote one, and beadsHome reads it on the launch path.
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return Die("session worktree beads redirect: %v", err)
	}
	if err := f.Close(); err != nil {
		return Die("session worktree beads redirect: %v", err)
	}
	if err := os.Rename(tmp, seeded); err != nil {
		return Die("session worktree beads redirect: %v", err)
	}
	return nil
}

// seedWorktreeLinks symlinks the repo-relative paths named in config
// `worktree_link:` from the main checkout into the fresh worktree. They are
// the gitignored things a checkout does not carry and a session may need —
// `plugin/bin/` and `bin/` in this repo, a local settings file in another.
// Declared rather than guessed: linking every gitignored path would link
// build output and caches two personas would then race over.
func seedWorktreeLinks(t *SessionTree, a *App) error {
	for _, rel := range YamlList(a.ConfigPath, "worktree_link") {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		clean := filepath.Clean(rel)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return Die("worktree_link: %q must be a path inside the repo", rel)
		}
		src := filepath.Join(t.Repo, clean)
		if _, err := os.Lstat(src); err != nil {
			continue // the main checkout does not have it either
		}
		dst := filepath.Join(t.Path, clean)
		if _, err := os.Lstat(dst); err == nil {
			continue // git checked it out, or a previous launch linked it
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return Die("worktree_link %s: %v", clean, err)
		}
		if err := os.Symlink(src, dst); err != nil {
			return Die("worktree_link %s: %v", clean, err)
		}
	}
	return nil
}

// ─── merging a session's work back ───────────────────────────────────────────

// MergeOutcome is what one merge-back attempt did. Nothing here is inferred:
// Commits is counted, Merged is only true when the base is MEASURED to
// contain the session's work (landed asks), and Reason names the obstacle in
// the operator's words when it did not.
type MergeOutcome struct {
	Branch  string
	Base    string
	Commits int      // commits holding the session's work that base did not have
	Merged  bool     // base now contains them — measured, never assumed
	Rebased bool     // base had moved, so the branch was replayed onto it
	Dirty   []string // uncommitted paths left in the worktree (never merged)
	Reason  string   // why not merged ("" when Merged)

	// Equivalent pairs each commit ahead of the base with the commit on the
	// base that already holds its work under another sha. Non-empty only
	// when EVERY commit ahead is accounted for — that is the whole point of
	// it, and the difference between "already landed, retire freely" and
	// "this tree is the only copy".
	Equivalent []string

	// Unmeasured is what that pairing rests on for the commits no
	// measurement of CONTENT accounts for — git's `-x` trailer, an identity
	// match on a replay, or both — in the words the two other surfaces
	// already use for it (unmeasuredClause). "" says every commit's account
	// is a patch-id measurement, which is the only evidence that can say
	// nothing here is unlanded.
	Unmeasured string
}

// EquivalentNote is the sentence that tells an already-landed branch apart
// from a stranded one. Before it, both printed the same words (the strand's)
// and only a hand measurement could say which was which — ranger-base-g2xf.
// "" when there is nothing to tell apart.
//
// It asks the evidence, because "nothing here is unlanded" is a measurement
// claim and two of the three kinds of pairing are not measurements
// (ranger-base-dmzk7). This is the surface that decides whether a human is
// told at all — Blocked() is false on every equivalence and no
// merge-back-blocked bead is filed — and over an identity match it was
// asserting the confident sentence while unaccountedFor, on the same tree in
// the same pass, said "compare before retiring the tree". Two confidences,
// one piece of evidence, and the confident one won where it cost the most:
// a session that AMENDS a commit the launcher already landed keeps the
// author, author date and subject, so the pairing still matches and the new
// bytes are on no ref but this branch.
//
// The unmeasured arm says what unaccountedFor and treeState say, and only
// that. It does NOT report a strand: contentNotOnBase cannot tell the amend
// above from the case this whole arm exists for — a rebase whose conflict a
// human resolved by keeping both sides, where the branch's bytes are on main
// nowhere and the work is nevertheless entirely landed (ranger-base-nw9zg,
// ranger-base-emgdb). Only a person reading the two can say which, so the
// sentence sends them to read it rather than guessing for them in either
// direction.
func (o MergeOutcome) EquivalentNote() string {
	if len(o.Equivalent) == 0 {
		return ""
	}
	if o.Unmeasured == "" {
		return fmt.Sprintf("%d commit(s) on %s are already on %s under other sha(s) (%s) — nothing here is unlanded",
			len(o.Equivalent), o.Branch, o.Base, strings.Join(o.Equivalent, ", "))
	}
	return fmt.Sprintf("%d commit(s) on %s are accounted for on %s under other sha(s), %s; compare (`git log %s..%s`) before retiring the tree",
		len(o.Equivalent), o.Branch, o.Base, o.Unmeasured, o.Base, o.Branch)
}

// Blocked is "the merge was attempted, answered, and the answer was no" —
// the one state that owes a human a handoff (noteMergeBlocked).
//
// Both halves are load-bearing. !Merged alone is also true of the ZERO
// outcome a MergeSessionWork that returned an error leaves behind: no
// branch was read, no obstacle was named, and a P1 whose whole body is
// "reason: " helps nobody. Reason is the witness that the function got as
// far as deciding — it is set on every nil-error return that did not merge
// (landed(), and every early return above it) and cleared on every one that
// did, which is the invariant MergeOutcome.Reason's own comment states.
func (o MergeOutcome) Blocked() bool {
	return !o.Merged && o.Reason != ""
}

// MergeSessionWork is the launcher's half of ADR-0011-§1-serialized option A
// (rangerhq-jbyr): fast-forward the session branch onto the repo's own
// branch, rebasing it first when the branch has moved underneath.
//
// It never merges non-fast-forward and it never commits: a fast-forward
// creates no commit, so the prepare-commit-msg wall is not in the path and
// there is no merge commit for a persona's hook to refuse. Every failure
// leaves the branch and the worktree exactly as they were — a conflicted
// rebase is aborted — so the persona's work is never the thing that is lost
// when a merge cannot happen.
//
// CALLERS MUST HOLD THE LAUNCHER LOCK. This moves the repo's branch, which
// is the same kind of check-then-act against a shared store that ADR 0011 §1
// serializes everything else for.
func MergeSessionWork(t *SessionTree) (MergeOutcome, error) {
	o := MergeOutcome{Branch: t.Branch, Base: t.Base}
	// THE SPLICE (ranger-base-t4f1), before every guard below it. A session
	// launched at the container tier works on a DETACHED HEAD, because that
	// is what lets its git grant name no ref at all (PrepareSessionHead), so
	// its commits are in the TREE and the branch is still where it was cut.
	// That is the shape ranger-base-dybv measured a false Merged over and
	// ranger-base-g2xf a false strand report over, and every guard from here
	// down — branchExists, the rev-list count, constitutionOnBranch, landed
	// — reads it as the anomaly it is for every OTHER session. Putting the
	// work back on the branch first is what keeps those guards catching the
	// ACCIDENTAL case: it runs only for a session posse recorded as detached
	// on purpose, and it is fast-forward only.
	//
	// Under the launcher lock like the rest of this function, and before
	// `t.Base == ""`: a repo whose own HEAD is detached has nowhere to land
	// today, and the work still belongs on its branch for the pass that can.
	//
	// A refused splice is not reported here: what refuses it is a branch tip
	// the tree's HEAD does not reach, which is exactly the disagreement
	// landed() already names in words that send the operator to read it. A
	// second sentence would be the same finding twice.
	if launchedDetached(t.Repo, t.Branch) {
		_ = spliceDetachedWork(t)
	}
	if t.Base == "" {
		o.Reason = fmt.Sprintf("%s has a detached HEAD — there is no branch for %s to land on", AbbrevHome(t.Repo), t.Branch)
		return o, nil
	}
	if !branchExists(t.Repo, t.Branch) {
		// The branch is gone. That is USUALLY "nothing was ever committed on
		// it" — and used to be read as that and nothing else, which is a
		// guess, and the wrong one whenever the tree is still sitting on
		// commits the branch was retired out from under. landed asks the
		// tree instead of guessing.
		return landed(o, t), nil
	}
	o.Dirty = dirtyPaths(t.Path)
	n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+t.Branch)
	if err != nil {
		return o, err
	}
	if o.Commits, err = strconv.Atoi(n); err != nil {
		return o, Die("git rev-list --count %s..%s: unreadable answer %q", t.Base, t.Branch, n)
	}
	if o.Commits == 0 {
		// Nothing ahead of the base ON THE BRANCH is not nothing ahead of it
		// in the TREE: a worktree whose HEAD is detached takes the persona's
		// commits with it and leaves the branch where it was cut. That read
		// as "nothing to merge" and reported a successful close over work no
		// ref would ever reach (ranger-base-dybv).
		return landed(o, t), nil
	}
	// `git merge` moves the branch the repo has CHECKED OUT, which is not
	// necessarily the one this session was cut from: the operator's checkout
	// is the one store here that the launcher lock does NOT govern (ADR 0011
	// §1), and a `git checkout -b` in it between reading the base and acting
	// on it used to land the persona's commit on the operator's own branch
	// while reporting the base merged (ranger-base-5s2o). Asked immediately
	// before the merge, so the window is as small as a check-then-act
	// against somebody else's checkout can be made.
	if why := notOnBase(t); why != "" {
		o.Reason = why
		return o, nil
	}
	// THE BELT BEHIND THE COMMIT WALL (ranger-base-ak3e). The L3 arm that
	// refuses a persona commit touching the constitution keys on RHQ_PERSONA
	// and so stands down to `env -i` — the residual PrePushHook documents for
	// its own marker. This is the same class asked where a session cannot
	// reach: the launcher's own process, operator-side, under the launcher
	// lock, about a branch that is already written. A commit that got past
	// the hook still does not become main's.
	//
	// Reported, never repaired: the branch keeps every commit, the sweep
	// prints its ⚠ line on every pass, and the operator lands it by hand once
	// they have read the diff. That is the intended workflow rather than a
	// degradation of it — ADR 0015 §3 makes putting constitution prose in
	// force the operator's act, and an unattended fast-forward is not that.
	if hit, why := constitutionOnBranch(t); why != "" {
		o.Reason = why
		return o, nil
	} else if len(hit) > 0 {
		// Already on the base under other shas is not a landing to refuse:
		// there is nothing left to land, and printing a refusal every pass
		// over work that is already on main is ranger-base-g2xf's shape with
		// a different sentence.
		if head, ok := workHead(t); ok {
			if eq := equivalentOnBase(t.Repo, t.Base, head); len(eq) > 0 {
				o.Equivalent, o.Unmeasured = equivNotes(eq), unmeasuredNote(eq, t.Base)
				o.Merged, o.Reason = true, ""
				return o, nil
			}
		}
		o.Reason = constitutionLandRefusal(t, hit)
		return o, nil
	}
	if _, err := git(t.Repo, "merge", "--ff-only", t.Branch); err == nil {
		return landed(o, t), nil
	}
	// Ahead by sha is not ahead by work. A commit cherry-picked onto the
	// base keeps its own sha here, so `rev-list --count` still calls the
	// branch ahead; and when the pick was resolved BY HAND the two patches
	// are not identical, so the rebase below cannot drop it by patch-id
	// either. What came out was a strand report word-for-word identical to
	// a real one, over work that was already on the base, re-printed every
	// pass forever (ranger-base-g2xf). Asked before the rebase so it also
	// answers the arms the rebase never reaches — a dirty tree, a checkout
	// that moved — and asked of the tree's HEAD rather than the branch for
	// the same reason landed() is: the branch is not always where the work
	// sits.
	if head, ok := workHead(t); ok {
		if eq := equivalentOnBase(t.Repo, t.Base, head); len(eq) > 0 {
			o.Equivalent, o.Unmeasured = equivNotes(eq), unmeasuredNote(eq, t.Base)
			o.Merged, o.Reason = true, ""
			return o, nil
		}
	}
	// Not a fast-forward: the repo's branch moved while the session worked.
	// Replay the session's commits onto it — in the SESSION's tree, so the
	// operator's checkout is untouched by the risky half.
	if len(o.Dirty) > 0 {
		o.Reason = fmt.Sprintf("%s moved on and %s has uncommitted changes (%s), which git will not rebase over", t.Base, AbbrevHome(t.Path), strings.Join(o.Dirty, " "))
		return o, nil
	}
	// The rebase and the fast-forward under it are ONE compare-and-swap on
	// the base's ref: the rebase computes a new tip from the base it read,
	// and `merge --ff-only` is the swap, which git refuses unless the base
	// is still exactly where the rebase read it. So a refusal here has two
	// meanings that used to print as one — this branch cannot land, or the
	// swap LOST — and only the second is a retry.
	//
	// It loses often, because the base is the one store in this path the
	// launcher lock does not govern (ADR 0011 §1): the operator commits on
	// main in the shared checkout while the rebase runs. Measured on
	// ranger-base-c02a — rebased onto 523dc33, main took 5ea6b23 forty
	// seconds later, the ff refused, a merge-back-blocked bead was filed at
	// 09:57:11, and the very next pass landed the same untouched branch at
	// 09:57:26. Fifteen seconds, and a bead for a human (ranger-base-59fs).
	//
	// Retried on the MEASURED move and on nothing else: the base's sha is
	// read before the rebase and compared with the one the ff refused over,
	// so a branch that genuinely cannot fast-forward still reports on the
	// first attempt and pays nothing for this loop. Bounded because the
	// launcher lock is held across it and a base that never holds still is
	// a report, not a spin.
	for attempt := 1; ; attempt++ {
		wasAt := refSHA(t.Repo, t.Base)
		if _, err := git(t.Path, "rebase", t.Base); err != nil {
			// EVERY non-zero exit used to print as a conflict, and the P1
			// under it told a persona to "fix the conflicts" — over a
			// rebase that never reached a merge at all (ranger-base-5hqa:
			// the box was at 100% disk, and the branch it named replayed
			// onto every main tip in the range cleanly). Ask the rebase
			// state, which only a stopped merge leaves behind, and say what
			// git said either way: git's stderr is the one witness to the
			// real cause and it was being thrown away.
			stopped := rebaseStopped(t.Path)
			said := gitSaid(err)
			_, _ = git(t.Path, "rebase", "--abort")
			// "the rebase was aborted" and not "the branch is untouched and
			// still holds the work" (ranger-base-m3195). The reason is
			// printed once by the pass, where either reading is true, and
			// then embedded VERBATIM in a merge-back bead that a seat opens
			// some unbounded time later — by which point the branch may have
			// been retired out from under it, and the old wording was a
			// promise about that future rather than a report of what this
			// attempt did. What this attempt did is abort, and that stays
			// true forever.
			if stopped {
				o.Reason = fmt.Sprintf("%s moved on and replaying %s onto it conflicts — the rebase was aborted, so this attempt changed nothing (git: %s)", t.Base, t.Branch, said)
			} else {
				o.Reason = fmt.Sprintf("%s moved on and replaying %s onto it failed before any merge — git said: %s — so there are no conflicts to resolve, and the rebase was aborted", t.Base, t.Branch, said)
			}
			return o, nil
		}
		o.Rebased = true
		// The rebase is the slow half, and the operator can switch branches
		// during it. Ask again rather than trust the answer from before it.
		if why := notOnBase(t); why != "" {
			o.Reason = why
			return o, nil
		}
		_, err := git(t.Repo, "merge", "--ff-only", t.Branch)
		if err == nil {
			return landed(o, t), nil
		}
		nowAt := refSHA(t.Repo, t.Base)
		if nowAt == wasAt || wasAt == "" || nowAt == "" {
			// The base is where the rebase left it, so the refusal is about
			// this branch and replaying it again would only ask git the same
			// question twice.
			o.Reason = fmt.Sprintf("%s still would not fast-forward after the rebase: %v", t.Base, err)
			return o, nil
		}
		if attempt >= mergeRebaseAttempts {
			// "nothing was landed" and not "%s still holds every commit"
			// (ranger-base-eq3ba, the same reading as the abort arm above):
			// this reason is embedded VERBATIM in a merge-back bead a seat
			// opens some unbounded time later, by which point the branch may
			// have been retired out from under it. What this attempt did is
			// land nothing, and that stays true forever.
			o.Reason = fmt.Sprintf("%s moved again under every one of %d replays (last %s → %s) and never held still long enough for %s to fast-forward onto it — nothing was landed, and the next pass retries",
				t.Base, mergeRebaseAttempts, abbrevSHA(wasAt), abbrevSHA(nowAt), t.Branch)
			return o, nil
		}
	}
}

// constitutionOnBranch names the constitution-class paths a session branch
// would land, rendered as the pairs the refusal prints. Two returns and not
// one because "nothing in the class" and "git would not say" are different
// answers and only the first is safe to land on: a diff that cannot be read
// is reported as an obstacle, never waved through. The class is read from the
// REPO the branch lands in, so a session worktree of the constitution repo
// gets the promoted set and a session anywhere else gets the settings files.
//
// The three-dot form, so a base that moved on carrying somebody else's
// constitution commit is not this branch's to answer for.
//
// The head is workHead's and not a sha read on the way in, for one reason
// that survives: when the WORKTREE is gone, workHead falls back to the branch
// ref, and a retired tree's branch is exactly the population the landing
// sweep exists for (landsweep.go). The neighbouring case — a tree whose HEAD
// is detached off its own branch (ranger-base-dybv) — never reaches here:
// notOnBase refuses it first, and for a better reason.
func constitutionOnBranch(t *SessionTree) ([]string, string) {
	head, ok := workHead(t)
	if !ok {
		return nil, ""
	}
	out, err := git(t.Repo, "diff", "--name-only", "-z", "--no-renames", t.Base+"..."+head)
	if err != nil {
		return nil, fmt.Sprintf("git could not diff %s...%s in %s (%v), so whether %s touches the constitution is unknown — nothing lands on an unread diff (ADR 0015 §2/§3)",
			t.Base, abbrevSHA(head), AbbrevHome(t.Repo), err, t.Branch)
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return ConstitutionTouched(ConstitutionClassIn(t.Repo), paths), ""
}

// constitutionLandRefusal is the sentence the sweep prints and the operator
// acts on: the paths, the rule, and the commands that finish the job by hand
// — none of which the launcher will run for them.
//
// `posse promote` is named only when a promoted path is actually in the hit.
// The settings files are in the class in every repo and no promote puts them
// in force, so prescribing one there would send the operator to run a command
// that does nothing about what they just read — the kind of near-right
// instruction that teaches people to skim the refusal.
//
// It reports what the launcher DID — landed nothing, changed nothing — and
// never that the branch still holds every commit (ranger-base-eq3ba). This
// sentence is o.Reason, and noteMergeBlocked embeds o.Reason verbatim in a
// merge-back bead read some unbounded time later; "%s still holds every
// commit" was that bead's ranger-base-m3195 promise in one more spelling, on
// an arm that close did not edit. The branch is still named — the `log -p`
// and the `merge --ff-only` need it — but as the thing to act on, not as a
// thing asserted to exist.
func constitutionLandRefusal(t *SessionTree, hit []string) string {
	promoted := false
	for _, h := range hit {
		if strings.HasPrefix(h, ConstitutionSourceDir+"/") {
			promoted = true
			break
		}
	}
	then := ""
	if promoted {
		then = ", then `posse promote`"
	}
	return fmt.Sprintf("it touches the constitution — %s — and ADR 0015 §2/§3 makes putting that in force the operator's act, not a fast-forward the launcher does unattended. Nothing was landed and nothing here was changed. To land it: `git -C %s log -p %s...%s` to read it, then `git -C %s merge --ff-only %s`%s",
		strings.Join(hit, ", "),
		AbbrevHome(t.Repo), t.Base, t.Branch,
		AbbrevHome(t.Repo), t.Branch, then)
}

// rebaseStopped answers whether a failed `git rebase` STOPPED — the merge
// waiting for a hand resolution — rather than failing outright. The merge
// backend (git's default since 2.26) leaves rebase-merge/ behind while it
// waits and the apply backend leaves rebase-apply/; a rebase that never got
// that far — a full disk, a pre-rebase hook, a lock, an unresolvable base —
// leaves neither. Asked BEFORE `rebase --abort`, which is what removes the
// directory, and asked of git rather than derived from .git, because a
// linked worktree keeps its rebase state under the common dir's worktrees/
// subtree and only git knows where (the same reason the hook probe asks for
// --git-path rather than joining one).
func rebaseStopped(dir string) bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		p, err := git(dir, "rev-parse", "--git-path", name)
		if err != nil || p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// gitSaid renders git's own complaint for a bead a person reads: one line,
// without the hint block git writes for a human sitting at a terminal, and
// bounded — a description is a handoff, not a transcript.
func gitSaid(err error) string {
	var keep []string
	for _, ln := range strings.Split(err.Error(), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "hint:") {
			continue
		}
		keep = append(keep, ln)
	}
	said := strings.Join(keep, "; ")
	if said == "" {
		said = strings.TrimSpace(oneLine(err.Error()))
	}
	const most = 400
	if r := []rune(said); len(r) > most {
		said = strings.TrimSpace(string(r[:most])) + "…"
	}
	return said
}

// mergeRebaseAttempts bounds the replay loop in MergeSessionWork. It is a
// bound and not a budget: every attempt past the first is evidence that the
// base moved under the last one, so the only thing a bigger number buys is a
// longer hold on the launcher lock while a busy base keeps winning. Three
// covers the fleet's measured cadence — main in posse moves every few
// minutes, and the rebase itself is seconds.
const mergeRebaseAttempts = 3

// refSHA is where a ref points right now, "" when it does not resolve. The
// whole reason it exists is to be asked TWICE around a slow operation, so
// that "the fast-forward refused" can be told apart from "the base moved out
// from under it" instead of the two sharing one sentence.
func refSHA(repo, ref string) string {
	sha, err := git(repo, "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return ""
	}
	return sha
}

// landed is the ONE place this file is allowed to say a merge happened, and
// it says it from a measurement: the base either reaches the commit the
// session's work is on, or it does not.
//
// Why it exists (ranger-base-dybv). Merged used to be INFERRED — from `git
// merge`'s exit status on the happy path, and from a guess on two others: a
// branch that does not exist was read as "nothing was ever committed", and a
// branch with nothing ahead of the base was read as "nothing to land".
// Neither holds when the persona's commits are in the TREE and not on the
// branch — a detached worktree HEAD, or a branch retired out from under a
// live tree — and both returned Merged with an empty Reason. That is a close
// reporting success over work that is in no tree anyone will ever check out,
// measured twice in the field (rangerhq-81y0/eb03495, rangerhq-vojc/6217c9f)
// and, because a false Merged is what lets `posse kill` go on to retire the
// tree, the same read that would DESTROY it.
//
// The question is asked of the tree's HEAD as it is NOW, never of a sha
// captured on the way in: a rebase legitimately rewrites the work, so the
// pre-rebase sha is on nothing afterwards while the tree's HEAD is on the
// base, and only the second is the fact anyone cares about.
func landed(o MergeOutcome, t *SessionTree) MergeOutcome {
	head, ok := workHead(t)
	if !ok {
		// No tree and no branch: there is no commit left for this session to
		// strand and none for anything here to lose. (A `posse kill --force`
		// can reach this having thrown work away — that is the operator's
		// documented override, and it is loud where it is taken.)
		o.Merged, o.Reason = true, ""
		return o
	}
	if reaches(t.Repo, t.Base, head) {
		o.Merged, o.Reason = true, ""
		return o
	}
	// Ahead by sha is not ahead by work HERE TOO (ranger-base-d8o6). The two
	// sentences below are about work that would be LOST, and neither is true
	// of work the base already holds under another sha: a tree whose commits
	// were picked onto the base keeps its own shas, so `reaches` says no over
	// a branch there is nothing left to land from. MergeSessionWork asks
	// equivalentOnBase of this same tip twice before it rebases — this is the
	// path that runs INSTEAD of those, when the branch itself never moved
	// (o.Commits == 0) or is already gone, and it was the one surface still
	// answering by ancestry alone. Measured: a detached tree whose work was
	// cherry-picked onto main was reported "unreferenced and a retire would
	// lose it" while the listing beside it said "nothing unlanded (… as an
	// equivalent patch on main)" — d8o6's disagreement with the sides
	// swapped, found by the pin that asserts the two agree.
	//
	// `len(eq) > 0` and not measuredOnBase, the same threshold the two
	// sites above use: this decides what to REPORT, and reporting an
	// equivalence with its evidence named (Unmeasured, EquivalentNote) is
	// what those print. What may DESTROY on the strength of it is
	// RemoveSessionTree, which asks the stricter question for itself
	// (ranger-base-as19, ranger-base-x8jp) and still refuses here.
	if eq := equivalentOnBase(t.Repo, t.Base, head); len(eq) > 0 {
		o.Equivalent, o.Unmeasured = equivNotes(eq), unmeasuredNote(eq, t.Base)
		o.Merged, o.Reason = true, ""
		return o
	}
	o.Merged = false
	// Report the honest count: the branch's own is 0 in exactly the case
	// this guard exists for, and "0 commit(s) did NOT reach main" reads as
	// nothing being wrong.
	if n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+head); err == nil {
		if c, cerr := strconv.Atoi(n); cerr == nil && c > o.Commits {
			o.Commits = c
		}
	}
	if o.Reason != "" {
		return o
	}
	if branchExists(t.Repo, t.Branch) && !reaches(t.Repo, t.Branch, head) {
		o.Reason = fmt.Sprintf("%s in %s is on neither %s nor %s — the tree's HEAD is off its own branch, so no merge here can reach it; `git -C %s branch -f %s HEAD` puts the work back on the branch and the next pass lands it",
			abbrevSHA(head), AbbrevHome(t.Path), t.Base, t.Branch, AbbrevHome(t.Path), t.Branch)
		return o
	}
	o.Reason = fmt.Sprintf("%s in %s is not on %s and no branch here reaches it — the work is unreferenced and a retire would lose it; `git -C %s branch -f %s HEAD` names it again",
		abbrevSHA(head), AbbrevHome(t.Path), t.Base, AbbrevHome(t.Path), t.Branch)
	return o
}

// workHead is the commit this session's work is ON, asked in the order the
// answer survives: the WORKTREE's HEAD first — that is where a persona's
// commit lands whatever branch, or no branch, is checked out there — then
// the branch, for a tree that has already been retired. ("", false) is a
// session with neither, which is nothing left to land.
func workHead(t *SessionTree) (string, bool) {
	if sha, err := git(t.Path, "rev-parse", "HEAD"); err == nil && sha != "" {
		return sha, true
	}
	if sha, err := git(t.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+t.Branch); err == nil && sha != "" {
		return sha, true
	}
	return "", false
}

// ─── pinning a blocked branch's work (ranger-base-m3195) ────────────────────

// blockedPinPrefix is the namespace posse pins a BLOCKED branch's work
// under. A merge-back block is a bead that outlives the reading it was
// filed from: the tree it names is retired the moment its merge is no
// longer refused, and every one of RemoveSessionTree's refusals hands the
// operator `git -C … worktree remove … && git -C … branch -D …` to run by
// hand — so the branch can go between the filing and the dispatch and
// nothing in posse is watching when it does.
//
// MEASURED, twice. ranger-base-g7br6's branch was gone at dispatch and its
// only commit survived as an unreachable object — no ref, no reflog, alive
// only because nothing on this box runs `gc --prune` on a schedule.
// ranger-base-nr3eq's went the same way, and its bead was still telling a
// seat the branch "is untouched and still holds every commit".
//
// A ref is the fix rather than a re-check at dispatch, because a re-check
// only narrows the window — the branch can be deleted between the check and
// the seat's first command — where a ref posse owns closes it: `gc` never
// prunes what a ref reaches, and `branch -D` cannot take work a second ref
// names. refs/posse/ and not refs/heads/: this is not a branch, nothing
// should check it out, and `git branch -a` must not grow a row per block.
const blockedPinPrefix = "refs/posse/merge-blocked/"

// blockedPinRef is where THIS branch's work is pinned. The branch name is
// the key and it needs no escaping: it is already a ref path (refs/heads/ +
// branch), so anything git accepted there it accepts here, and the D/F
// conflict a nested name could cause is one refs/heads/ would have had
// first.
func blockedPinRef(branch string) string { return blockedPinPrefix + branch }

// retiredTipPrefix is the namespace posse keeps a RETIRED tree's tip under,
// keyed the way the pin above is and for one reason more (ADR 0058's
// 2026-09-06 amendment, ranger-base-qz3cr).
//
// The class it exists for is the one D4 used to leave to a human: a tree
// whose commits the base accounts for only by git's `-x` trailer or by an
// identity match on a replay. Neither is a measurement of what the landing
// kept — MEASURED 2026-09-06 over the 14 such trees standing here, 16 of
// their 22 paired commits differ in bytes from the commit that landed them,
// and every trailer was typed by a persona replaying by hand, because
// MergeSessionWork writes none — so the pairing is a decision and a decision
// never licences a delete. That sentence stands. What it does not settle is
// where the bytes go: the 14 trees had stood up to nine days with seven
// unrun hand recipes filed against them, and a decayed do-not-land verdict
// re-files a P1 every time its dedupe window closes.
//
// So the tip is written HERE first and the licence is the ref, not anybody's
// word: `gc` never prunes what a ref reaches and `branch -D` cannot take
// work a second ref names, so after this write the branch is provably the
// last copy of nothing and heldByTip says so. refs/posse/ and not
// refs/heads/ for blockedPinPrefix's reasons, unchanged: nothing should
// check it out and `git branch -a` must not grow a row per retire.
//
// NOTHING PRUNES IT, and that is the whole cost of the record: one
// packed-refs line per retired tree, no objects added (they are already in
// the store), and no reader in posse that enumerates it —
// `git for-each-ref refs/posse/retired --sort=committerdate` is the
// operator's index and `git update-ref -d` their prune.
const retiredTipPrefix = "refs/posse/retired/"

// retiredTipRef is where THIS branch's tip is kept when its tree is retired.
// blockedPinRef's note on escaping applies unchanged: the branch name is
// already a ref path, so anything refs/heads/ accepted this accepts too.
func retiredTipRef(branch string) string { return retiredTipPrefix + branch }

// keepRetiredTip writes the branch's tip under retiredTipRef and reads it
// back, which is the whole of what makes the removal after it safe. "" is
// written and verified; anything else is the refusal, and a refusal here
// means NOTHING is removed (ADR 0058's amendment, point 3).
//
// THE ORDER IS THE POINT. The ref goes down before `worktree remove`, not
// after, because a retire that removed first and then failed to write would
// have taken the only copy of exactly the commits this class is defined by.
// Read back with rev-parse rather than trusting update-ref's exit status:
// the claim being made is "a ref now reaches this sha", and the only
// instrument for that is asking git what the ref resolves to.
//
// An existing ref at ANOTHER sha keeps the tree and names both. That is a
// reopened bead relaunched into the same seat name and retired twice, and
// overwriting would lose the first tip — the remedy is the operator's
// `update-ref -d`, so the sentence hands them it.
func keepRetiredTip(t *SessionTree) string {
	ref := retiredTipRef(t.Branch)
	sha, err := git(t.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+t.Branch)
	if err != nil || sha == "" {
		return fmt.Sprintf("%s's tip cannot be read, so there is nothing to keep at %s and nothing is removed", t.Branch, ref)
	}
	if have, err := git(t.Repo, "rev-parse", "--verify", "--quiet", ref); err == nil && have != "" && have != sha {
		return fmt.Sprintf("%s already holds %s and %s is at %s — nothing is removed, and `git -C %s update-ref -d %s` decides which tip is kept",
			ref, abbrevSHA(have), t.Branch, abbrevSHA(sha), AbbrevHome(t.Repo), ref)
	}
	if _, err := git(t.Repo, "update-ref", ref, sha); err != nil {
		return fmt.Sprintf("%s could not be written (%s), so nothing here is kept and nothing is removed", ref, gitSaid(err))
	}
	if back, err := git(t.Repo, "rev-parse", "--verify", "--quiet", ref); err != nil || back != sha {
		return fmt.Sprintf("%s did not read back at %s after it was written, so nothing is removed", ref, abbrevSHA(sha))
	}
	return ""
}

// pinBlockedWork makes a blocked branch's work reachable from a ref posse
// owns, and returns the sha it pinned and the ref it pinned it under. The
// sha comes back even when the pin does not, because a sha in the bead is
// still a handle a human can use today — it is only tomorrow it may be gone.
// ("", "") is a session with no head at all, which is nothing to pin.
//
// Best effort, on noteMergeBlocked's rule: a repo that will not take the ref
// must not cost a blocked merge the bead that says where its code is.
func pinBlockedWork(t *SessionTree) (sha, ref string) {
	sha, ok := workHead(t)
	if !ok {
		return "", ""
	}
	ref = blockedPinRef(t.Branch)
	// Asked of the REPO and not the tree, for workHeadTime's reason: the
	// tree may already be retired, and the object store both share outlives
	// it. The ref lives in the repo either way — a worktree-local ref would
	// be pruned with the worktree that held it, which is the exact failure.
	if _, err := git(t.Repo, "update-ref", ref, sha); err != nil {
		return sha, ""
	}
	return sha, ref
}

// unpinBlockedWork drops the pin for a branch. Its callers are the prune
// (prunePinnedBlocks) and nothing else: a pin is deleted when the block it
// serves has been ANSWERED, never because the branch it names went away —
// that is the case it exists for.
func unpinBlockedWork(repo, branch string) error {
	_, err := git(repo, "update-ref", "-d", blockedPinRef(branch))
	return err
}

// pinnedBlockedBranches is every branch this repo currently holds a pin for,
// read off git rather than off any record posse keeps: the pin is the record.
func pinnedBlockedBranches(repo string) []string {
	out, err := git(repo, "for-each-ref", "--format=%(refname)", blockedPinPrefix)
	if err != nil {
		return nil
	}
	var branches []string
	for _, ln := range strings.Split(strings.TrimSpace(out), "\n") {
		ln = strings.TrimSpace(ln)
		if b := strings.TrimPrefix(ln, blockedPinPrefix); b != ln && b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

// workHeadTime is WHEN this session's work last moved: the committer date of
// the commit workHead names, in the same order and for the same reason.
//
// The committer date and not the author's, because it is the one a REPLAY
// updates — a rebase, a cherry-pick or an amend rewrites it and keeps the
// author date — so "the branch is the same branch it was" is what this
// measures, not "the same patch was written".
//
// Its one reader is the merge-back dedupe (priorMergeBlocked), which needs
// to know whether a branch has moved since a verdict was recorded about it,
// and (zero, false) is the honest "cannot say" that reader files on.
func workHeadTime(t *SessionTree) (time.Time, bool) {
	sha, ok := workHead(t)
	if !ok {
		return time.Time{}, false
	}
	// Asked of the REPO and not the tree: a retired worktree is exactly the
	// state workHead's second arm covers, and the object store both share
	// outlives it.
	return commitTime(t.Repo, sha)
}

// commitTime is one commit's COMMITTER date — the one a replay updates,
// where the author date survives a rebase, a cherry-pick and an amend — as
// (zero, false) whenever git will not say. Its two readers ask for opposite
// reasons and both need the committer's: the merge-back dedupe asks whether
// a branch is the same branch a verdict was recorded about (workHeadTime),
// and ADR 0058's grace asks when this tip was last WRITTEN, which a replay
// is.
func commitTime(repo, ref string) (time.Time, bool) {
	out, err := git(repo, "show", "-s", "--format=%cI", ref)
	if err != nil {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, strings.TrimSpace(out))
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

// equivalentOnBase answers whether base ALREADY HOLDS the work of every
// commit tip is ahead of it by, under other shas, and names the pairing when
// it does. nil is the honest default and means "no": one commit it cannot
// account for and the branch is ahead by real work, which is a strand.
//
// Three ways a commit's work reaches the base under another sha, and the
// bugs that need all three (ranger-base-g2xf, ranger-base-emgdb):
//
//   - patch-id equivalence, which is what `git cherry` measures and what a
//     non-interactive rebase drops on its own. One call for the whole
//     branch, and the case that never reaches a human.
//   - git's own `-x` trailer, `(cherry picked from commit <sha>)`. That is
//     the only evidence a PICK resolved by hand leaves: the resolution
//     amends the patch, so the patch-ids differ, `git cherry` says `+`, and
//     the replay conflicts on the same hunk every time.
//   - the commit's own identity — author, AUTHOR date, subject — carried
//     through unchanged by the rebase that landed it. That is the only
//     evidence a REBASE resolved by hand leaves, because a rebase writes no
//     trailer at all. ADR 0051 measured 48 of 134 landings — 36% — as
//     rebases, so this is the arm the common case needs, not a corner:
//     on ranger-base-nw9zg, main held all five
//     commits (landed 2026-09-02 22:15:47) and only two were patch-id twins
//     — the other three had absorbed a sibling's line in a shared Makefile
//     .PHONY list and a scrub of the fixture literals main made on top. So
//     the strand report was word-for-word a real one over work that was
//     entirely on main, and it cost a P1 and a seat (ranger-base-emgdb).
//
// The three are not the same evidence, and every caller that would DESTROY
// something has to tell them apart, which is why each pairing carries how it
// was answered (ranger-base-as19). Patch-id equivalence is a measurement of
// content: the base holds this patch, so the branch is the last copy of
// nothing. The trailer is a record of somebody's decision and the identity
// pairing is an inference about a replay — neither can say whether the
// resolution kept every hunk, and 34a27b4/6a230eb above is a pair where it
// demonstrably did not — so both are read as "the base holds this work" and
// never as licence to throw the branch away.
func equivalentOnBase(repo, base, tip string) []equiv {
	out, err := git(repo, "rev-list", base+".."+tip)
	if err != nil || out == "" {
		return nil
	}
	upstream := map[string]bool{}
	if c, err := git(repo, "cherry", base, tip); err == nil {
		for _, ln := range strings.Split(c, "\n") {
			if strings.HasPrefix(ln, "- ") {
				upstream[strings.TrimSpace(ln[2:])] = true
			}
		}
	}
	var eq []equiv
	var replay map[string]string // built once, and only if a cheap arm misses
	for _, sha := range strings.Fields(out) {
		if upstream[sha] {
			eq = append(eq, equiv{note: abbrevSHA(sha) + " as an equivalent patch on " + base, byPatch: true})
			continue
		}
		// Bounded to the commits base has and tip does not: a pick of this
		// commit can only be in there, and the bound keeps a miss from
		// walking the whole history.
		pick, err := git(repo, "log", "--format=%H", "-1", "--fixed-strings",
			"--grep=cherry picked from commit "+sha, base, "--not", tip)
		if err == nil && pick != "" {
			eq = append(eq, equiv{note: abbrevSHA(sha) + " as " + abbrevSHA(pick)})
			continue
		}
		key := replayKey(repo, sha)
		if key == "" {
			return nil
		}
		if replay == nil {
			replay = replayIndex(repo, base, tip)
		}
		twin := replay[key]
		if twin == "" {
			return nil
		}
		eq = append(eq, equiv{
			note:     abbrevSHA(sha) + " as " + abbrevSHA(twin) + " (same author, author date and subject)",
			replayed: true,
		})
	}
	return eq
}

// replayKey is the three fields a rebase carries through a commit unchanged
// — author identity, AUTHOR date (not the committer's, which the replay
// rewrites), and subject — and nothing else. "" for a commit git will not
// describe, which the caller reads as unaccounted-for rather than looking
// up: "" is also replayIndex's mark for an AMBIGUOUS key, and an unreadable
// commit must not collide with one.
func replayKey(repo, sha string) string {
	k, err := git(repo, "log", "-1", "--format=%ae%x1f%aI%x1f%s", sha)
	if err != nil {
		return ""
	}
	return k
}

// replayIndex maps that key to the single commit on the base that carries
// it, over the same bound the trailer lookup uses. A key TWO commits share
// is mapped to "" — an ambiguous pairing is not a pairing, and the caller
// reads "" as unaccounted-for, which is the honest default everywhere else
// in equivalentOnBase.
//
// Measured before it was relied on: 1163 commits on this repo's main, 1163
// distinct keys, no collision at all (ranger-base-emgdb).
func replayIndex(repo, base, tip string) map[string]string {
	out, err := git(repo, "log", "--format=%H%x1f%ae%x1f%aI%x1f%s", base, "--not", tip)
	if err != nil {
		return map[string]string{}
	}
	idx := map[string]string{}
	for _, ln := range strings.Split(out, "\n") {
		f := strings.SplitN(ln, "\x1f", 2)
		if len(f) != 2 || f[0] == "" {
			continue
		}
		if _, dup := idx[f[1]]; dup {
			idx[f[1]] = "" // two commits, one key: nothing here is identified
			continue
		}
		idx[f[1]] = f[0]
	}
	return idx
}

// equiv is one commit's account of itself on the base: the sentence a human
// can check by hand, and which of the three kinds of evidence answered it.
//
// byPatch is the only one that is a measurement of CONTENT, and it is the
// one every destructive caller keys on (measuredOnBase). The other two are
// records or inferences about what somebody's landing did, and adding one
// can never widen what may be deleted.
type equiv struct {
	note     string
	byPatch  bool // measured by patch-id
	replayed bool // paired by author identity, author date and subject
	// neither: git's -x trailer said so
}

// equivNotes is the pairing as a reader sees it — the evidence is a fact
// about what may be DELETED, not about what to print.
func equivNotes(eqs []equiv) []string {
	var out []string
	for _, e := range eqs {
		out = append(out, e.note)
	}
	return out
}

// measuredOnBase is the destructive half's question, and it is stricter than
// the reporting half's: is EVERY commit's account a measurement of content
// rather than somebody's assertion? Only then is the branch provably the
// last copy of nothing, and only then may it be deleted without a human
// (ranger-base-as19). Empty is false: nothing accounted for is not proof.
func measuredOnBase(eqs []equiv) bool {
	for _, e := range eqs {
		if !e.byPatch {
			return false
		}
	}
	return len(eqs) > 0
}

// unmeasured is the pairing's UNMEASURED half: the commits no measurement of
// content accounts for. A refusal names those and counts those — it used to
// count every commit ahead of the base and list the patch-measured pairing
// among them, which reads as though a measurement were missing when it is
// not (ranger-base-x8jp).
func unmeasured(eqs []equiv) []string {
	var out []string
	for _, e := range eqs {
		if !e.byPatch {
			out = append(out, e.note)
		}
	}
	return out
}

// unmeasuredEvidence names what the unmeasured half actually rests on. A
// sentence naming the one kind of evidence it does not have is the same
// overstatement ranger-base-x8jp and ranger-base-hk02 both removed, and the
// third kind (ranger-base-emgdb) is not a record of anything.
func unmeasuredEvidence(eqs []equiv) string {
	trailer, replay := false, false
	for _, e := range eqs {
		switch {
		case e.byPatch:
		case e.replayed:
			replay = true
		default:
			trailer = true
		}
	}
	switch {
	case trailer && replay:
		return "git's own -x trailer and a replay of the same commit"
	case replay:
		return "a replay of the same commit"
	default:
		return "git's own -x trailer"
	}
}

// unmeasuredClause is the whole of what a listing may say about the
// unmeasured half: what accounts for it, and — in the same breath, because
// the two were separable once and the second went missing — that it is not a
// measurement of what the landing kept.
func unmeasuredClause(eqs []equiv, base string) string {
	var trailer, replay []string
	for _, e := range eqs {
		switch {
		case e.byPatch:
		case e.replayed:
			replay = append(replay, e.note)
		default:
			trailer = append(trailer, e.note)
		}
	}
	switch {
	case len(trailer) > 0 && len(replay) > 0:
		return fmt.Sprintf("recorded as landed in %s and replayed onto %s as %s — a decision and an identity match, and neither is a measurement of what the landing kept",
			strings.Join(trailer, "; "), base, strings.Join(replay, "; "))
	case len(replay) > 0:
		return fmt.Sprintf("replayed onto %s as %s, which is an identity match and not a measurement of what the replay kept",
			base, strings.Join(replay, "; "))
	default:
		return fmt.Sprintf("recorded as landed in %s, which is a decision and not a measurement of what the resolution kept",
			strings.Join(trailer, "; "))
	}
}

// unmeasuredNote is unmeasuredClause for a pairing that may be wholly
// measured: "" says every commit's account is a patch-id measurement of
// content, and it is the only answer that licenses the confident sentence
// (MergeOutcome.Unmeasured, EquivalentNote). Empty in, empty out — nothing
// accounted for has nothing to say about its evidence, and unmeasuredClause
// would render its trailer default over an empty list.
func unmeasuredNote(eqs []equiv, base string) string {
	if len(eqs) == 0 || measuredOnBase(eqs) {
		return ""
	}
	return unmeasuredClause(eqs, base)
}

// contentNotOnBase names the paths the branch touched whose BYTES the base
// does not hold anywhere. It is the destructive half's second question, and
// it exists because the first one is not what it claims (ranger-base-x8jp).
//
// `git cherry` compares PATCH-IDs, and `git patch-id` normalises whitespace.
// So a hand resolution that only re-indented — a gofmt, an editor, a YAML or
// Makefile fixup — leaves the patch-ids EQUAL: byPatch is true, the trailer
// arm is never consulted, and `branch -D` took the only copy of those exact
// bytes. "The base holds this patch" was never the same statement as "the
// base holds this content", and only the second licenses a delete.
//
// Asked in two layers, because the cheap question over-refuses. The tree
// comparison first: for the paths the branch changed since the merge-base,
// does the base's tree already agree byte for byte? That is the ordinary
// answer and it costs two git calls. Where it disagrees the base may still
// hold those bytes further back — a clean pick that the base then edited on
// top of is exactly that, and refusing it would put back the every-pass
// refusal ranger-base-as19 removed. So each disagreeing path is looked for
// in the commits the BASE has and the branch does not, the same bound the
// trailer lookup uses: a copy of this branch's bytes can only be in there.
//
// Renames are not detected, so a rename is read as the add and the delete it
// is and both sides get compared. A path the branch never touched is none of
// its business: the base moves on while a session works, and that is not
// something the branch is the last copy of.
//
// Strictly narrower than the guard it joins: every commit must still be
// patch-id equivalent first. Nothing this adds can license a deletion the
// old code refused.
//
// And its "no" is no longer the last word. A non-empty answer here is a
// question for the second instrument rather than a refusal on its own —
// baseHoldsBytes asks the whitespace-exact twin over the same bound, and
// EITHER licenses. This walk stays the first one asked because it is the
// cheap common answer and because the refusal's per-path `git diff` pointer
// is its; what it stopped being is the only one (ADR 0058, amendment
// 2026-09-06).
func contentNotOnBase(repo, base, tip string) ([]string, error) {
	touched, err := gitPathsZ(repo, "diff", "--name-only", "--no-renames", "-z", base+"..."+tip)
	if err != nil || len(touched) == 0 {
		return nil, err
	}
	differing, err := gitPathsZ(repo, append([]string{"diff", "--name-only", "--no-renames", "-z", base, tip, "--"}, literalPathspecs(touched)...)...)
	if err != nil || len(differing) == 0 {
		return nil, err
	}
	var lost []string
	for _, path := range differing {
		want := blobAt(repo, tip, path)
		if want == "" {
			// The branch does not have this path at all — it deleted it,
			// and a deletion is an intent, not bytes. There is no content
			// here for the branch to be the last copy of.
			continue
		}
		if !blobInRange(repo, tip+".."+base, path, want) {
			lost = append(lost, path)
		}
	}
	return lost, nil
}

// baseHoldsBytes is ADR 0058 D1 fact 2's second half asked WHOLE, and it is
// one function because two callers ask it (heldByTip, treeHolds) and their
// answers about one tree must be the same answer —
// TestRetireGuardsSeeADetachedTreesWork holds them to it.
//
// "The base holds the branch's bytes" is measured two ways and EITHER
// licenses (ADR 0058, amendment 2026-09-06):
//
//   - for every path the branch touched, the branch's BLOB was on the base
//     somewhere in `tip..base` (contentNotOnBase). The cheap common answer,
//     asked first, and the one that carries the refusal's per-path pointer;
//   - or every commit ahead has a whitespace-EXACT patch-id twin among the
//     base's own commits in the same bound (`git patch-id --verbatim`).
//
// The second exists because the first was empty over the shape this record
// was filed about. A tree reaches the equivalence row because its landing
// was not a fast-forward, and a landing that is not a fast-forward writes a
// NEW blob for every file the base moved in the meantime. For an
// append-heavy file (CHANGELOG.md, NOTES.md) the base has ALWAYS moved it,
// so the branch's blob is on the base nowhere, on any commit, and the tree
// is kept on every pass forever — for bytes the base holds line for line.
// MEASURED 2026-09-06 in ~/src/posse: the olwk tree's commit and its landing
// 7ff3e4da share a --verbatim id while the blob walk loses CHANGELOG.md and
// INSTALL.md.
//
// The twin is not the loose measurement the patch-id name suggests. Plain
// `git patch-id` NORMALISES WHITESPACE, which is the hole ranger-base-x8jp
// opened this second layer over; `--verbatim` is git's own flag for not
// doing that, and it hashes CONTEXT lines too, so a twin is the same hunks
// against the same neighbours, byte for byte. Lines the branch never touched
// are not its to lose — the rule contentNotOnBase already states for paths.
//
// Both arms FAIL CLOSED. A commit the range form prints no id for is a merge
// (or an empty commit) and is unmeasured, not paired. A git that rejects
// `--verbatim` (it is 2.39+, and it cannot be combined with `--stable`) is
// an error, and both callers read an error as a keep.
//
// held is the licence; lost and unpaired are the refusal's words and are
// only meaningful when held is false: the paths whose bytes the base does
// not hold, and the earliest commit ahead the base has no exact twin for.
//
// Cost: the twin arm walks the whole of `tip..base` and is paid ONLY by a
// tree that already passed patch-id and failed the blob walk — 970 ids over
// olwk's 987-commit range in 5.5s wall, measured the same day.
func baseHoldsBytes(repo, base, tip string) (held bool, lost []string, unpaired string, err error) {
	lost, err = contentNotOnBase(repo, base, tip)
	if err != nil {
		return false, nil, "", err
	}
	if len(lost) == 0 {
		return true, nil, "", nil
	}
	unpaired, err = verbatimUnpaired(repo, base, tip)
	if err != nil {
		return false, nil, "", err
	}
	if unpaired == "" {
		return true, nil, "", nil
	}
	return false, lost, unpaired, nil
}

// verbatimUnpaired names the earliest commit in base..tip that the base has
// no whitespace-exact patch-id twin for, and "" when every one of them has
// one. Earliest and not latest because a refusal names ONE commit and the
// first divergence is the one a human goes looking at.
//
// The ahead side is asked first and alone: a commit with no id at all is
// unmeasured, and saying so costs nothing while the twin lookup over the
// base is the expensive half. `rev-list` supplies the commits rather than
// the id stream, because the id stream is exactly what cannot see the merge
// it printed nothing for.
func verbatimUnpaired(repo, base, tip string) (string, error) {
	out, err := git(repo, "rev-list", base+".."+tip)
	if err != nil {
		return "", err
	}
	ahead := strings.Fields(out)
	if len(ahead) == 0 {
		return "", nil
	}
	// rev-list prints newest first; a sentence about the first divergence
	// wants the other order.
	for i, j := 0, len(ahead)-1; i < j; i, j = i+1, j-1 {
		ahead[i], ahead[j] = ahead[j], ahead[i]
	}
	ids, err := patchIDsVerbatim(repo, base+".."+tip)
	if err != nil {
		return "", err
	}
	for _, sha := range ahead {
		if ids[sha] == "" {
			return abbrevSHA(sha) + " (a merge or an empty commit — `git log -p` prints no patch for it, so there is nothing to compare)", nil
		}
	}
	onBase, err := patchIDsVerbatim(repo, tip+".."+base)
	if err != nil {
		return "", err
	}
	// COUNTED, not a set. Two commits ahead can carry one id — an add, a
	// revert and the same add again is the ordinary way — and a base holding
	// ONE of them holds one of them. A set would pair both against it and
	// license deleting the second, which is the whole failure mode this
	// function exists to refuse; a twin is consumed by the commit it pairs.
	twins := map[string]int{}
	for _, id := range onBase {
		twins[id]++
	}
	for _, sha := range ahead {
		if twins[ids[sha]] == 0 {
			return abbrevSHA(sha), nil
		}
		twins[ids[sha]]--
	}
	return "", nil
}

// patchIDsVerbatim is `git log -p --no-ext-diff --no-renames <range> | git
// patch-id --verbatim` — one process pair for the whole range rather than
// one per commit — read back as commit sha to patch id.
//
// A real pipe and not a buffer: the diff of a thousand-commit range is tens
// of megabytes, and the two fds are handed to the children directly so the
// shell's own semantics hold. When patch-id dies on the first line — an
// older git rejecting `--verbatim` — `git log` takes a broken pipe and dies
// too, so patch-id's failure is the one reported: git log's would be the
// same error with the cause filed off.
//
// A commit the stream carries no patch for gets no entry at all, and that
// absence is the caller's fail-closed case, not a lookup miss to shrug at.
func patchIDsVerbatim(repo, rng string) (map[string]string, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, Die("git patch-id --verbatim: %v", err)
	}
	defer pr.Close()
	defer pw.Close()

	logCmd := exec.Command("git", "-C", repo, "log", "-p", "--no-ext-diff", "--no-renames", rng)
	idCmd := exec.Command("git", "-C", repo, "patch-id", "--verbatim")
	var logErr, idErr, out strings.Builder
	logCmd.Stdout, logCmd.Stderr = pw, &logErr
	idCmd.Stdin, idCmd.Stdout, idCmd.Stderr = pr, &out, &idErr

	if err := idCmd.Start(); err != nil {
		return nil, Die("git patch-id --verbatim: %v", err)
	}
	pr.Close()
	if err := logCmd.Start(); err != nil {
		pw.Close()
		_ = idCmd.Wait()
		return nil, Die("git log -p %s: %v", rng, err)
	}
	// The parent's write end has to go or `git patch-id` never reads EOF.
	pw.Close()
	logWait := logCmd.Wait()
	idWait := idCmd.Wait()
	if idWait != nil {
		return nil, Die("git patch-id --verbatim: %s", gitErrText(idErr.String(), idWait))
	}
	if logWait != nil {
		return nil, Die("git log -p %s: %s", rng, gitErrText(logErr.String(), logWait))
	}

	ids := map[string]string{}
	for _, ln := range strings.Split(out.String(), "\n") {
		// "<patch id> SP <commit id>", one line per commit that had a patch.
		f := strings.Fields(ln)
		if len(f) != 2 {
			continue
		}
		ids[f[1]] = f[0]
	}
	return ids, nil
}

// gitErrText prefers what git said on stderr to what exec says about the
// exit status: "unknown option `verbatim'" is the sentence that tells the
// operator their git is too old, and "exit status 129" is not.
func gitErrText(stderr string, err error) string {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return msg
	}
	return err.Error()
}

// blobInRange answers whether any commit in the range holds oid at path. The
// range is the caller's bound, not this function's: an unbounded walk over a
// question about destroying work is the wrong kind of slow.
//
// --full-history because the default simplification follows ONE parent
// through a merge, and the commit that put these bytes on the base can be on
// the side it drops. Missing one is a refusal, which is survivable, but the
// question deserves the complete answer.
func blobInRange(repo, rng, path, oid string) bool {
	out, err := git(repo, "rev-list", "--full-history", rng, "--", ":(literal)"+path)
	if err != nil {
		return false
	}
	for _, sha := range strings.Fields(out) {
		if blobAt(repo, sha, path) == oid {
			return true
		}
	}
	return false
}

// blobAt is the object id of path in rev, or "" when rev does not have it.
// `ls-tree` rather than `rev-parse rev:path`, which splits on the FIRST
// colon and so mis-reads a path spelled with one.
func blobAt(repo, rev, path string) string {
	out, err := git(repo, "ls-tree", rev, "--", ":(literal)"+path)
	if err != nil || out == "" {
		return ""
	}
	f := strings.Fields(out)
	if len(f) < 3 || f[1] != "blob" {
		return ""
	}
	return f[2]
}

// literalPathspecs stops a path spelled with a `*` or a `[` from being read
// as a glob and comparing the wrong files.
func literalPathspecs(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, ":(literal)"+p)
	}
	return out
}

// gitPathsZ runs a `-z` path-listing git command and splits it. Not `git`:
// that trims, and a path is allowed to be spelled with whitespace — which is
// the whole subject here.
func gitPathsZ(repo string, args ...string) ([]string, error) {
	out, err := gitRaw(repo, args...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// reaches is the fast-forward precondition, asked of git rather than assumed
// from an exit status somewhere else: does ref already contain sha?
func reaches(repo, ref, sha string) bool {
	if ref == "" || sha == "" {
		return false
	}
	_, err := git(repo, "merge-base", "--is-ancestor", sha, ref)
	return err == nil
}

// abbrevSHA shortens a sha for a sentence a human reads. Not `git
// rev-parse --short`: this runs on a path where the repo may be the thing
// that is broken, and a message about lost work must not need another git
// call to render.
func abbrevSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// notOnBase says why the repo's checkout cannot take this session's work
// right now, and "" when it can. Not landing is a delay — the branch keeps
// the work and `posse worktrees --land` finishes it once the operator is
// back on the base — where landing on the wrong branch is a mess in the
// operator's history that nothing here could undo.
func notOnBase(t *SessionTree) string {
	cur := repoBranch(t.Repo)
	if cur == t.Base {
		return ""
	}
	if cur == "" {
		return fmt.Sprintf("%s has a detached HEAD, so %s cannot land on %s — check %s out there and `posse worktrees --land`",
			AbbrevHome(t.Repo), t.Branch, t.Base, t.Base)
	}
	return fmt.Sprintf("%s has %s checked out, not %s — %s was cut from %s and lands there or nowhere; check %s out there and `posse worktrees --land`",
		AbbrevHome(t.Repo), cur, t.Base, t.Branch, t.Base, t.Base)
}

// dirtyPaths lists what the worktree holds that no commit does. It is
// reported rather than acted on: uncommitted work is the persona's, and the
// only thing the harness owes it is to never destroy it silently.
func dirtyPaths(path string) []string {
	// gitRaw, not git: the status column's LEADING SPACE is data here. git()
	// trims the whole blob, which takes that space off the first line only —
	// and the ln[3:] below then cuts one real character off the first path
	// (ranger-base-8ogq). git()'s trim stays: every other caller reads a
	// whole value (a count, a branch name, a sha) and wants it.
	out, err := gitRaw(path, "status", "--porcelain")
	if err != nil {
		return nil
	}
	var paths []string
	for _, ln := range strings.Split(string(out), "\n") {
		// "XY path": two status columns, a space, then the path — so the
		// shortest real record is four bytes and anything shorter (the
		// empty line the trailing newline leaves) is not one.
		if len(ln) < 4 {
			continue
		}
		paths = append(paths, strings.TrimSpace(ln[3:]))
	}
	return paths
}

// RemoveSessionTree retires a session's worktree and its branch. It refuses
// while anything would be lost — uncommitted changes, or commits whose work
// the repo's branch does not have — because a session worktree is the only
// copy of what is in it. force is the operator's override and says so in the
// caller.
//
// "Does not have" is asked by patch, not by sha (ranger-base-as19). It used
// to be `rev-list --count base..branch != 0` and nothing else, which counts
// a commit cherry-picked onto the base as unlanded forever: the pick keeps
// its own sha here. So a branch MergeSessionWork had just reported Merged —
// with the pairing named — was refused retirement every pass, and the only
// escape left was the same override that stands down a real strand's
// refusal, one layer down.
//
// It honours only the half of that equivalence that MEASURES content, and
// then it checks what the measurement actually measured. git's `-x` trailer
// says only that a human decided this landed as that, and a hand resolution
// can drop a hunk while writing it — so on that evidence the tree is kept,
// and the refusal says which of the two this is instead of counting shas at
// the operator. THAT SENTENCE IS STILL THE RULE and the decision is still
// never the licence; what changed on 2026-09-06 is that there is now a
// second way for the tip to stop being the last copy of anything, and it is
// not evidence about the base at all. ADR 0058's amendment has the retire
// write the branch tip to refs/posse/retired/<branch> under the launcher
// lock FIRST (keepRetiredTip), and heldByTip then answers off that ref: a
// commit a ref posse owns reaches is held by something other than this
// branch, whatever anybody decided about its landing. So this refusal is
// what a HAND caller still reads over that class — nothing writes the ref
// for them — and the class stopped being one nothing in posse would ever
// take. A patch-id match says the base holds this PATCH, which is a
// weaker statement than it reads as: `git patch-id` normalises whitespace,
// so a resolution that only re-indented is "equivalent" while the bytes
// differ (ranger-base-x8jp). What licenses `branch -D` is both halves — every
// commit measured by patch-id, AND the base holding the branch's BYTES —
// which is itself two measurements, either of which licenses: the base's
// history holding the branch's blob for every path it touched
// (contentNotOnBase), or every commit ahead having a whitespace-exact
// patch-id twin among the base's own commits (`git patch-id --verbatim`).
// A third licence, and the only one that is not a reading of the base:
// retiredTipRef already reaching the tip (above).
// Asked as one question, in baseHoldsBytes, whose doc says why the second
// exists — the blob walk alone covered NONE of the class this guard's
// unattended caller was written for (ADR 0058's 2026-09-06 amendment). (The
// sha guard is not the only by-sha check in this path: `branch -d` refuses
// an unmerged branch too.)
//
// AND IT IS ASKED OF BOTH TIPS, not of the branch alone (ranger-base-v2rj7).
// `<base>..<branch>` is ZERO over a worktree whose HEAD is DETACHED — the
// shape a container-tier session is launched on ON PURPOSE, because on a
// detached HEAD a commit writes no ref and that is what buys the `:ro`
// common dir (PrepareSessionHead, ranger-base-t4f1) — so this refusal, the
// one that exists to say no while anything would be lost, read a whole
// session's committed work as nothing held. MEASURED 2026-09-05, before the
// fix: a stamped detached tree with one commit on it, clean, was removed by
// RemoveSessionTree(t, false) — worktree gone, branch deleted, the commit
// left referenced by nothing. It was not a live loss only because its one
// caller runs MergeSessionWork first and that splices; a guard that holds
// on somebody else's evidence is not a guard. removalTips says why both
// tips and not just the head: `branch -D` below takes the branch whatever
// the tree's HEAD reaches.
func RemoveSessionTree(t *SessionTree, force bool) error {
	// The branch is provably redundant: its every commit's work is on the
	// base under another sha, measured. Nothing here is the last copy.
	redundant := false
	if !force {
		if d := dirtyPaths(t.Path); len(d) > 0 {
			return Die("%s has uncommitted changes (%s) — not removed", AbbrevHome(t.Path), strings.Join(d, " "))
		}
		for _, tip := range removalTips(t) {
			// No base is not "nothing is ahead" — it is "the question
			// cannot be asked", and the safe answer to an unanswerable
			// question about destroying work is no.
			if t.Base == "" {
				return Die("%s still exists and %s has a detached HEAD, so what is unmerged cannot be known — not removed", tip.subject, AbbrevHome(t.Repo))
			}
			r, err := heldByTip(t, tip)
			if err != nil {
				return err
			}
			// Only the BRANCH's own redundancy licenses `branch -D` below:
			// that is the ref this deletes, and a measured tip is a
			// statement about the commits that tip reaches and no others.
			if r && tip.isBranch {
				redundant = true
			}
		}
	}
	if _, err := os.Stat(t.Path); err == nil {
		args := []string{"worktree", "remove", t.Path}
		if force {
			args = append(args, "--force")
		}
		if _, err := git(t.Repo, args...); err != nil {
			return err
		}
	}
	_, _ = git(t.Repo, "worktree", "prune")
	if branchExists(t.Repo, t.Branch) {
		del := "-d"
		// `branch -d` asks reachability, which is the same by-sha question
		// the guard above just answered by patch — so the measured case
		// needs -D or the refusal simply moves down here.
		if force || redundant {
			del = "-D"
		}
		if _, err := git(t.Repo, "branch", del, t.Branch); err != nil {
			return err
		}
	}
	return nil
}

// removalTip is one commit a retire would drop the last reference to, and
// the words a refusal about it has to be written in: what to call it, what
// to ask git about, and how the operator keeps it if they want it.
//
// There are two of them and not one because a session tree has two tips
// (ranger-base-v2rj7). The BRANCH is what `branch -D` deletes. The tree's
// own HEAD is what `worktree remove` drops — and on a detached HEAD those
// are different commits, because a commit made there writes NO ref at all,
// which is exactly why a container-tier session is launched detached
// (PrepareSessionHead, ranger-base-t4f1).
type removalTip struct {
	subject  string // what the refusal calls it
	ref      string // what git is asked about: a branch name, or a sha
	tail     string // the clause that says how to keep this one
	isBranch bool   // the session branch itself, the ref `branch -D` takes
}

// removalTips is every tip RemoveSessionTree must ask about before it
// destroys anything, in the order a refusal should reach them: the branch,
// then the tree's own HEAD when that is a DIFFERENT commit.
//
// Both, and not one. Asking the branch alone is what this bead exists for:
// `<base>..<branch>` is zero over a detached tree, so a whole session's
// committed work read as nothing held. Asking the head alone would be a
// trade, not a fix — a branch that holds a commit its worktree's HEAD does
// not reach (a rebase the tree walked away from, a splice that half ran) is
// guarded today and must stay guarded, and `branch -D` below takes that ref
// whatever the head says.
//
// Empty is a session with neither a branch nor a head, which is nothing to
// lose: workHead's own ("", false).
func removalTips(t *SessionTree) []removalTip {
	var tips []removalTip
	branchSHA := ""
	if branchExists(t.Repo, t.Branch) {
		branchSHA, _ = git(t.Repo, "rev-parse", "refs/heads/"+t.Branch)
		tips = append(tips, removalTip{
			subject:  t.Branch,
			ref:      t.Branch,
			isBranch: true,
			tail: fmt.Sprintf("`git -C %s worktree remove %s && git -C %s branch -D %s`",
				AbbrevHome(t.Repo), AbbrevHome(t.Path), AbbrevHome(t.Repo), t.Branch),
		})
	}
	// `head != branchSHA` and not "is the HEAD detached": what matters is
	// whether the branch tip above already accounted for this commit, and
	// when the two are the same commit it did — so this arm is the detached
	// case and only it, the same no-op treeState's fix is over a tree whose
	// HEAD is on its own branch.
	if head, ok := workHead(t); ok && head != branchSHA {
		tips = append(tips, removalTip{
			subject: fmt.Sprintf("%s in %s", abbrevSHA(head), AbbrevHome(t.Path)),
			ref:     head,
			// landed()'s cure, in landed()'s words: this work is off the
			// session branch, so landing the branch would not carry it and
			// a retire afterwards takes it. Naming the branch at it is what
			// puts it back where the next pass can land it.
			tail: fmt.Sprintf("the tree's HEAD is off %s, so landing the branch would not carry it — `git -C %s branch -f %s HEAD` names it first",
				t.Branch, AbbrevHome(t.Path), t.Branch),
		})
	}
	return tips
}

// heldByTip is RemoveSessionTree's refusal asked of ONE tip: nil when this
// tip is the last copy of nothing, and the refusal itself when it is not.
// redundant is the licence to force the ref away — every commit measured by
// patch-id AND the base holding the bytes, or a ref posse owns already
// reaching this tip — and it is only ever true of a tip nothing would lose.
func heldByTip(t *SessionTree, tip removalTip) (redundant bool, err error) {
	n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+tip.ref)
	if err != nil {
		return false, err
	}
	if n == "0" {
		return false, nil
	}
	// THE ONE ARM THAT IS NOT ABOUT THE BASE (ADR 0058's 2026-09-06
	// amendment, ranger-base-qz3cr). Everything below asks whether the BASE
	// accounts for these commits, and answers no for the class that class is
	// defined by — a landing recorded by git's `-x` trailer or paired by
	// identity, which is somebody's decision and not a measurement of what
	// it kept. This asks a different question: is anything OTHER than this
	// tip already keeping these commits? A ref under retiredTipPrefix is,
	// and it is posse's own, written under the launcher lock immediately
	// before the removal that reads it here (keepRetiredTip). So the licence
	// is the ref and never the decision — which is exactly what the
	// amendment decided and what every sentence below still says.
	//
	// It is asked of EACH tip and not of the tree, so v2rj7's shape is kept
	// unchanged: a worktree HEAD holding a commit its branch does not reach
	// is not reachable from a ref written at the branch tip, so this arm is
	// silent about it and the refusal below is the answer.
	if reaches(t.Repo, retiredTipRef(t.Branch), tip.ref) {
		return true, nil
	}
	eq := equivalentOnBase(t.Repo, t.Base, tip.ref)
	switch {
	case measuredOnBase(eq):
		// Patch-id said the base holds these patches. It did NOT say the
		// base holds these bytes — it normalises whitespace — and it is the
		// bytes that are about to be deleted (ranger-base-x8jp). Two
		// instruments answer that and either licenses (baseHoldsBytes, ADR
		// 0058's 2026-09-06 amendment); the refusal names both halves it
		// does not have.
		held, lost, unpaired, err := baseHoldsBytes(t.Repo, t.Base, tip.ref)
		if err != nil {
			return false, err
		}
		if !held {
			return false, Die("%s is ahead of %s by %s commit(s) git calls equivalent by patch-id (%s), but patch-id normalises whitespace, %s has no whitespace-exact twin on %s and %s does not hold its bytes for %s — so it is still the last copy of that content and is not removed here; read `git -C %s diff %s %s -- %s`, then %s",
				tip.subject, t.Base, n, strings.Join(equivNotes(eq), ", "), unpaired, t.Base, t.Base, strings.Join(lost, " "),
				AbbrevHome(t.Repo), t.Base, tip.ref, strings.Join(lost, " "), tip.tail)
		}
		return true, nil
	case len(eq) > 0:
		only := unmeasured(eq)
		return false, Die("%s is ahead of %s by %s commit(s), %d of which have no record of landing beyond %s (%s) — that is somebody's decision or an identity match, not a measurement of what the landing kept, so it is still the last copy of those patches and is not removed here; compare them, then %s",
			tip.subject, t.Base, n, len(only), unmeasuredEvidence(eq), strings.Join(only, ", "), tip.tail)
	default:
		return false, Die("%s has %s commit(s) not on %s — not removed; %s", tip.subject, n, t.Base, tip.tail)
	}
}

// treeHolds is what removing this session tree would take that nothing else
// holds — "" when a removal takes nothing at all. It is RemoveSessionTree's
// unforced refusal above asked as a QUESTION rather than performed as an
// act, over the same two records and the same tips, and it fails closed on
// every question it cannot answer.
//
// Its wording is the reap's and not the refusal's: three callers now ask it
// (the reap before it kills, residueHolds; ADR 0058's retire before the
// landing sweep removes anything; and `posse worktrees --retire`), and all
// three are printing one clause inside a longer sentence, where
// RemoveSessionTree is writing the whole refusal a human acts on. What must
// not drift is the ANSWER, and TestRetireGuardsSeeADetachedTreesWork holds
// the two to it over one fixture per shape.
//
// The tips are removalTips' — the branch, and the tree's own HEAD when that
// is a different commit (ranger-base-v2rj7). `<base>..<branch>` is ZERO over
// a detached worktree, which is how a container-tier session is launched on
// purpose, so asking the branch alone answered "nothing held" over the whole
// of such a session's committed work.
func treeHolds(t *SessionTree) string {
	if dirty := dirtyPaths(t.Path); len(dirty) > 0 {
		return fmt.Sprintf("%s has uncommitted work (%s)", AbbrevHome(t.Path), dirtyList(dirty))
	}
	tips := removalTips(t)
	if len(tips) == 0 {
		// Retired already — `posse worktrees --land` or a kill that landed
		// took the branch, and neither a branch nor a tree HEAD is the last
		// copy of nothing.
		return ""
	}
	// A repo with no base cannot be asked at all — the same unanswerable
	// question RemoveSessionTree refuses over, and it is one fact about the
	// tree rather than one per tip.
	if t.Base == "" {
		return fmt.Sprintf("what %s holds beyond %s cannot be counted", tips[0].subject, orDetached(t.Base))
	}
	for _, tip := range tips {
		n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+tip.ref)
		if err != nil {
			return fmt.Sprintf("what %s holds beyond %s cannot be counted", tip.subject, orDetached(t.Base))
		}
		if n == "0" {
			continue
		}
		if eq := equivalentOnBase(t.Repo, t.Base, tip.ref); measuredOnBase(eq) {
			held, lost, unpaired, err := baseHoldsBytes(t.Repo, t.Base, tip.ref)
			switch {
			case err != nil:
				// Unreadable is a keep, and it keeps for the reason below:
				// what this tip holds could not be established.
			case held:
				continue
			default:
				// THE ONE ARM THAT NEEDED ITS OWN SENTENCE (ADR 0058 D3).
				// Patch-id said the base holds these patches; it did not say
				// the base holds these BYTES, because it normalises
				// whitespace (ranger-base-x8jp), and heldByTip above writes
				// a whole paragraph about exactly that. This function used
				// to flatten it to the generic count below — the same words
				// a plain strand gets — which was survivable while the only
				// reader was a reap line, and stopped being so when D3 put
				// this sentence in the LISTING one line under treeState's
				// "nothing unlanded (… as an equivalent patch on main)".
				// Two answers about one tree from one pass, and the operator
				// with no way to tell which of them was the careful one.
				// MEASURED 2026-09-06 on this box: three trees on the board
				// read exactly that way.
				return fmt.Sprintf("%s's %s commit(s) are on %s as equivalent patches, but patch-id normalises whitespace, %s has no whitespace-exact twin there and %s does not hold their bytes for %s",
					tip.subject, n, orDetached(t.Base), unpaired, orDetached(t.Base), strings.Join(lost, " "))
			}
		}
		return fmt.Sprintf("%s holds %s commit(s) %s does not", tip.subject, n, orDetached(t.Base))
	}
	return ""
}

// warnw is the io.Writer fallback these helpers share: nil means stderr, the
// same contract HerdrBackend.Warn has.
func warnw(w io.Writer) io.Writer {
	if w == nil {
		return os.Stderr
	}
	return w
}

// ─── seeing what has not landed, and finishing it ───────────────────────────

// mainCheckoutsOf is every distinct main checkout the given dirs name, in
// the order they name them. Shared by the two readers that need repos and
// not trees: the session walk below, and the merge-back pin prune, which
// must reach a repo whose session trees have all been retired
// (ranger-base-m3195) and so cannot derive its repos from them.
func mainCheckoutsOf(dirs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, dir := range dirs {
		repo, ok := MainCheckout(ExpandTilde(dir))
		if !ok || seen[repo] {
			continue
		}
		seen[repo] = true
		out = append(out, repo)
	}
	return out
}

// SessionTreesIn finds every session worktree of the given repos. It reads
// GIT, not the meta dir, on purpose: a kill that could not land its work
// removes the session's meta and leaves the tree standing, so the one record
// that survives every path is the one git keeps.
//
// A DETACHED tree is one of ours too (ranger-base-t4f1). `worktree list
// --porcelain` prints `detached` where it would have printed `branch
// refs/heads/…`, so keying on that line alone dropped every container-tier
// session's tree out of the landing sweep, `posse worktrees` and the merge
// — over exactly the sessions whose work is in the tree and not on the
// branch. The name is recoverable without it: a session tree's directory IS
// its session name (SessionTreePath), and the branch is derivable from the
// name alone (SessionBranch), which is the property that already lets a kill
// find the branch with nothing but the meta.
//
// Confirmed against the repo rather than assumed from the path, so a
// stranger's worktree that happens to sit at `…/posse-something` is not
// claimed as a session. The residual, stated: a detached tree whose branch
// somebody deleted is invisible here — git refuses `branch -D` for a branch a
// worktree has CHECKED OUT, and a detached tree has none, so that guard is
// gone with it. It is the shape landed() reports when the sweep does reach it
// ("no branch here reaches it"), and there is nothing in the porcelain that
// separates such a tree from any other detached worktree.
func SessionTreesIn(dirs []string) ([]*SessionTree, error) {
	var out []*SessionTree
	for _, repo := range mainCheckoutsOf(dirs) {
		base := repoBranch(repo)
		list, err := git(repo, "worktree", "list", "--porcelain")
		if err != nil {
			return nil, err
		}
		path, branch := "", ""
		flush := func() {
			if path != "" && branch == "" && resolveExisting(path) != resolveExisting(repo) {
				// Never the main checkout itself: it appears in this listing
				// too, and a detached one would otherwise be claimed as a
				// session tree whenever `posse/<repo basename>` happens to be
				// a branch.
				if b := SessionBranch(filepath.Base(path)); branchExists(repo, b) {
					branch = b
				}
			}
			if path != "" && strings.HasPrefix(branch, "posse/") {
				// Each tree's own base: they need not agree, and after an
				// operator branch switch none of them is today's HEAD.
				out = append(out, &SessionTree{
					Repo: repo, Path: path, Branch: branch,
					Base: baseOf(repo, branch, base), Bead: beadOf(repo, branch),
				})
			}
		}
		for _, ln := range strings.Split(list, "\n") {
			switch {
			case strings.HasPrefix(ln, "worktree "):
				flush()
				path, branch = strings.TrimPrefix(ln, "worktree "), ""
			case strings.HasPrefix(ln, "branch "):
				branch = strings.TrimPrefix(strings.TrimPrefix(ln, "branch "), "refs/heads/")
			}
		}
		flush()
	}
	return out, nil
}

// ListSessionTrees prints every session worktree, what each still holds,
// and what is going to happen to it.
//
// THE LAST LINE OF EACH ENTRY IS ADR 0058 D3, and it replaces a promise.
// This listing used to end at treeState — "nothing unlanded", its one phrase
// for a tree that is safe to retire — and leave the acting to a sentence
// addressed to "a human". MEASURED 2026-09-05: 38 of the 70 trees standing
// in ~/src/posse were dead, clean, closed and fully landed, and no human had
// ever run the command that sentence was pointing at. So it now says which
// of the four facts is holding this tree and which pass will take it, in the
// predicate's own words (RetireAsk.clause, retire.go) — the same words the
// sweep and `--retire` print, because a listing that answers the question
// with a lookalike is how two surfaces drift.
//
// THE TWO QUESTIONS STAY TWO LINES, because they are two questions.
// treeState answers what would be LOST by removing this tree, which is a
// fact about git alone; the clause answers whether anything WILL remove it,
// which needs the store of record, herdr and the dial. They agree on the
// common tree and they are not the same answer — a tree with nothing
// unlanded is kept by a live session, and a tree holding work is kept by
// fact 2 whatever herdr says.
func ListSessionTrees(w io.Writer, dirs []string, r *RetireAsk) error {
	trees, err := SessionTreesIn(dirs)
	if err != nil {
		return err
	}
	if len(trees) == 0 {
		fmt.Fprintln(w, "no session worktrees")
		return nil
	}
	for _, t := range trees {
		fmt.Fprintf(w, "%s\n  %s → %s  ·  %s\n  %s\n",
			t.Branch, AbbrevHome(t.Path), orDetached(t.Base), treeState(t), r.clause(t))
	}
	return nil
}

// LandSessionTrees merges every session branch that will land. It is the
// catch-up for the one case the kill defers: `posse kill` takes the launcher
// lock without waiting so the cockpit cannot freeze on it, and a kill that
// lost that race leaves a merged-nothing tree with its meta already gone
// (rangerhq-09o2).
//
// It MERGES and never removes, and that restraint is still the point HERE:
// this reads git and nothing else, so it cannot tell a tree whose session
// ended from one a persona is working in right now, and removing the second
// would destroy live work.
//
// What changed under it is who else may remove one (ADR 0058). The sentence
// this paragraph used to end with — "a tree it leaves reading `nothing
// unlanded` is one a human can retire" — was read for two weeks as a design
// intent and MEASURED 2026-09-05 to have made the trees permanent: 70
// standing, 38 of them dead, clean, closed and fully landed, and no human
// had ever run the command it was pointing at. The retire went to the one
// site that holds more than git — the landing sweep, which reads the bead
// fresh, asks herdr, and takes the launcher lock (landsweep.go, retire.go).
// A tree this command leaves reading "nothing unlanded" is one the next pass
// takes when its session is provably gone and nobody has written to it since;
// `posse worktrees --retire` is the human's run of that same predicate. This
// function is unchanged by any of it, and `--force` here still stands down
// the branch-record gate and nothing else.
//
// The whole sweep runs under one launcher lock (ADR 0011 §1), taken blocking
// — this is a command a person ran and waiting is the honest thing for it to
// do.
//
// It READS THE BEAD RECORD FIRST, and a tree holding work that no record
// accounts for is reported and not landed unless force says otherwise
// (ranger-base-atxe). It used to merge every tree unconditionally, which is
// the one thing in this file that could put stale work back on the operator's
// branch: measured in the field on a session tree whose single commit
// 6217c9f is byte-identical to 2418bde already on main — re-landed by
// hand under another bead id, so no patch-id and no `-x` trailer connects the
// two and equivalentOnBase cannot see it. A blind pass would have replayed
// 778 stale lines onto main or conflicted trying, and the listing paired with
// it said only "1 commit(s) not on main" — true, and no help at all in
// telling that tree from a real strand.
//
// The record is branch.<branch>.posseBead (beadKey), the same one
// landClosedTrees refuses to guess past. Empty is not "nothing to answer
// for": a crew session's tree has no bead by design, and so does every tree
// cut before the stamp landed. Both are legitimate and both still get the
// refusal, because from git alone they are indistinguishable from the vojc
// shape — force is the operator saying which one it is.
//
// IT IS THE ONE LANDING SITE THAT FILES NO BEAD, decided rather than
// overlooked (ranger-base-5nf8m). The other three — the judged close
// (mergeBack), the sweep (landsweep.go) and the kill (noteUnlandedOnKill) —
// write a blocked merge and a dirty tree onto the bead because nobody is
// watching them; this one is a command a HUMAN just ran and is reading the
// output of, which is the whole thing a handoff bead exists to substitute
// for. It also could not file honestly as it stands: it never asks bd for a
// status — the gate reads the BRANCH record, and `--force` waives even that
// — so it cannot tell a closed bead's strand from an open bead's branch that
// simply will not fast-forward yet, and a P1 at a persona still working in
// that tree is a false handoff. Filing here would mean a `bd show` per tree,
// to tell the operator in a bead what the terminal in front of them already
// says.
func LandSessionTrees(w io.Writer, a *App, dirs []string, force bool) error {
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
		if why := unaccountedFor(t, force); why != "" {
			fmt.Fprintf(w, "◑ %s\n", why)
			continue
		}
		o, err := MergeSessionWork(t)
		switch {
		case err != nil:
			fmt.Fprintf(w, "⚠ %s not landed: %v\n", t.Branch, err)
		case len(o.Equivalent) > 0:
			fmt.Fprintf(w, "≡ %s\n", o.EquivalentNote())
		case o.Merged && o.Commits == 0:
			fmt.Fprintf(w, "· %s had nothing to land\n", t.Branch)
		case o.Merged:
			fmt.Fprintf(w, "⤴ %s %d commit(s) onto %s\n", t.Branch, o.Commits, t.Base)
		default:
			fmt.Fprintf(w, "⚠ %s did NOT reach %s: %s\n", t.Branch, orDetached(t.Base), o.Reason)
		}
		if len(o.Dirty) > 0 {
			fmt.Fprintf(w, "  %d uncommitted path(s) stay in %s\n", len(o.Dirty), AbbrevHome(t.Path))
		}
	}
	return nil
}

// unaccountedFor is `--land`'s gate: the sentence to print INSTEAD of
// landing, or "" when this tree may be landed. Asked of the branch record
// (beadKey), never of the branch name — SessionForBead joins persona, repo
// and bead with '-' and all three contain it, so parsing it back out is a
// guess, and landsweep.go refuses to make it for the same reason.
//
// Only a tree with something to land is gated. Nothing ahead of the base is
// nothing to get wrong, and a base that cannot be read at all (detached HEAD,
// unreadable count) lands nothing either — MergeSessionWork says why in
// words, which is more use than this refusal would be.
//
// SOMETHING TO LAND IS ASKED OF THE TIP THIS PASS WOULD ACTUALLY TAKE
// (ranger-base-qihvt). `<base>..<branch>` is ZERO over a worktree whose HEAD
// is DETACHED, because a commit made there writes no ref and the branch is
// still where it was cut — and that is what a container-tier session is
// launched on ON PURPOSE, since a ref-less commit is what buys the `:ro`
// common dir (PrepareSessionHead, ranger-base-t4f1). That zero opened this
// gate, and the merge behind it SPLICES a designed detach's work back onto
// the branch before it counts anything (spliceDetachedWork) — so the whole of
// such a session's work went onto the operator's branch with no record
// accounting for it, ADR 0006's rule waived silently by asking a tip the work
// is not on. MEASURED 2026-09-05: a stamped detached tree, one commit, no
// bead record, this gate "" and `⤴ 1 commit(s) onto main` under it.
//
// BOTH tips and not the head instead, nothingToLand's reason
// (ranger-base-vavx2): a branch holding a commit its own worktree walked away
// from is landable work the head does not reach, and a splice this gate
// cannot see refused leaves MergeSessionWork counting the branch anyway. The
// branch is asked FIRST because the sentences here are the branch's — `git
// log <base>..<branch>` is true of a commit on the branch and of nothing
// else — and the head is asked only when the branch has nothing.
//
// The head arm is asked of a tree posse detached ON PURPOSE and no other,
// which is the whole of what this gate can lose. An ACCIDENTAL detach gets no
// splice, so a land takes nothing from it and there is nothing here to refuse;
// landed() answers that tree in a sentence carrying the `git branch -f` cure,
// and a refusal here would only displace it with a worse one.
//
// The SENTENCE names the tip as well, or it sends the operator to read a log
// that is empty (ranger-base-3nn9c: the gate and the sentence are two
// questions). What changes for a designed detach is WHERE the commits are to
// be read, not the cure — `--land --force` is still it, because posse runs
// the `branch -f` itself, which is how this work became landable at all.
//
// The GATE asks the record and the SENTENCE says what the tree holds, and
// they are two questions (ranger-base-3nn9c). The refusal used to answer the
// first and then assert the second from a sha count alone — "holds N
// commit(s) not on main … NOT landed" — over a branch the listing beside it
// called nothing unlanded, because ahead by sha is not ahead by work
// (ranger-base-hk02). Both sentences were printed about the same tree by the
// same pass, and the false one sent the operator to --force for work that
// does not exist: --force there lands nothing either, because
// MergeSessionWork answers the same equivalence and reports ≡ instead of
// merging. So the gate still holds on every arm — no record accounts for this
// tree and that is the whole of ADR 0006's rule — and equivalentOnBase, the
// one call treeState already makes, says which of the three true things to
// say about it.
func unaccountedFor(t *SessionTree, force bool) string {
	if force || t.Bead != "" {
		return ""
	}
	tip, where := t.Branch, ""
	look := fmt.Sprintf("git log %s..%s", t.Base, t.Branch)
	n, ok := unlandedCount(t)
	if !ok || n == 0 {
		head, detached := splicedTip(t)
		if !detached {
			return ""
		}
		if n, ok = unlandedAhead(t, head); !ok || n == 0 {
			return ""
		}
		tip = head
		where = fmt.Sprintf(" (on the tree's detached HEAD %s, not on the branch — a land splices them onto it first)", abbrevSHA(head))
		look = fmt.Sprintf("git -C %s log %s..HEAD", AbbrevHome(t.Path), t.Base)
	}
	eq := equivalentOnBase(t.Repo, t.Base, tip)
	switch {
	case measuredOnBase(eq):
		// Every commit is a measured patch-id match on the base: nothing is
		// being lost, which is the only thing this gate protects. Refused
		// still — the record is what it asked and the record is silent — but
		// there is nothing here to land and nothing --force could add.
		//
		// AND IT SAYS WHO RETIRES IT, which used to be "a human can retire
		// the tree" (ADR 0058 D3). That clause was the whole of posse's
		// instruction to anybody about these trees and it was addressed to
		// nobody: MEASURED 2026-09-05, 38 dead landed trees standing and
		// not one of them ever taken. It is not replaced by "the next pass
		// takes it" either, because this arm is a tree NO RECORD ACCOUNTS
		// FOR — the one population fact 1 keeps forever (ADR 0006, D4). So
		// the ADR 0006 sentence stands unchanged and the tail now says what
		// follows from it.
		return fmt.Sprintf("%s holds %d commit(s) not on %s by sha%s and no record says which bead — but every one of them is already on %s as an equivalent patch (%s), so nothing here is unlanded; %s",
			t.Branch, n, t.Base, where, t.Base, strings.Join(equivNotes(eq), "; "), noRecordKeeps)
	case len(eq) > 0:
		// No measurement of content: somebody's decision that this landed
		// (the -x trailer), or an identity match on a replay. Neither says
		// what the resolution kept. Not landed and not settled either — the
		// same answer RemoveSessionTree gives before it declines to delete
		// this shape, and the clause names which of the two it has.
		return fmt.Sprintf("%s holds %d commit(s) not on %s by sha%s and no record says which bead — %s; compare (`%s`) before retiring the tree",
			t.Branch, n, t.Base, where, unmeasuredClause(eq, t.Base), look)
	}
	return fmt.Sprintf("%s holds %d commit(s) not on %s%s and no record says which bead — NOT landed; look at it (`%s`) and `posse worktrees --land --force` when you want it",
		t.Branch, n, t.Base, where, look)
}

// splicedTip is the tip MergeSessionWork would count for this tree that its
// BRANCH is not already at: the HEAD of a tree posse detached ON PURPOSE,
// which the merge splices back onto the branch before it counts anything
// (spliceDetachedWork, THE SPLICE). ("", false) for every other shape — a
// tree whose HEAD is on its own branch, a tree that is gone, and an
// ACCIDENTAL detach, whose work no splice will move and no land can take.
//
// No guard here for a head the branch has already reached: the only caller
// asks this after the BRANCH's own count came back zero or unreadable, and a
// head the branch is at counts the same as the branch did.
func splicedTip(t *SessionTree) (string, bool) {
	if !launchedDetached(t.Repo, t.Branch) {
		return "", false
	}
	return treeDetachedHead(t.Path)
}

// treeState is the one phrase that says whether anything would be lost by
// removing this tree — the only question the listing exists to answer.
//
// It names the bead alongside the count because the count alone was the
// operator's whole basis for running `--land`, and "1 commit(s) not on main"
// is true of a strand and of an already-re-landed duplicate alike
// (ranger-base-atxe). Which bead the work belongs to is the difference, and
// it is one git config read away.
//
// IT ASKS THE TREE'S TIP, NOT THE BRANCH'S (ranger-base-d8o6). Every other
// surface that decides anything about this tree asks workHead — landed()
// says why in its own words, "the branch is not always where the work sits"
// — and the listing asked `<base>..<branch>` alone. That is zero over a
// worktree whose HEAD is DETACHED, which is not an accident and not rare: a
// caged session is launched detached on purpose, because on a detached HEAD
// a commit writes no ref and that is what buys the `:ro` common dir
// (PrepareSessionHead, ranger-base-t4f1). So the listing printed "nothing
// unlanded" — its one phrase for a tree that is safe to retire — over the
// whole of such a session's work, while MergeSessionWork on the same tree
// answered that the work "is on neither" and named the `branch -f` that
// rescues it. Two answers from one binary about one tree, the operator-facing
// one wrong, which is this bead's other half (ranger-base-dybv is the same
// blindness fixed at the merge, and this is the listing beside it). Asking
// workHead changes nothing for a tree whose HEAD is on its branch — the two
// tips are the same commit — so the fix is the detached case and only it.
func treeState(t *SessionTree) string {
	var parts []string
	head, hasWork := workHead(t)
	if t.Base == "" {
		parts = append(parts, "repo HEAD is detached — cannot say what is unmerged")
	} else if n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+head); hasWork && err == nil && n != "0" {
		// Ahead by sha is not ahead by work (ranger-base-hk02): the count
		// alone reads the same for a strand and for a branch RemoveSessionTree
		// is about to delete on the next pass. equivalentOnBase is the same
		// question RemoveSessionTree asks before it deletes, and
		// MergeSessionWork before it reports — the listing wants that answer
		// too, not just the sha count.
		eq := equivalentOnBase(t.Repo, t.Base, head)
		switch {
		case measuredOnBase(eq):
			// Every commit is a measured patch-id match on the base. This
			// comment used to go on "…the same fact that lets
			// RemoveSessionTree delete it unattended", and that has not been
			// the fact since ADR 0058's 2026-09-06 amendment landed
			// (ranger-base-06y60): the ACT needs the base to hold the BYTES
			// as well, measured by the blob walk or by a whitespace-exact
			// twin, because patch-id normalises whitespace. 06y60 amended the
			// two sibling comments that said only the blob licenses and left
			// this one, which made the stronger claim (ranger-base-bbl6r
			// finding 5).
			//
			// So this line prints "nothing unlanded" over a tree
			// RemoveSessionTree then REFUSES, and does today: a hand landing
			// that only re-indented is patch-id equivalent while the branch
			// still holds the last copy of those exact bytes. What keeps the
			// entry honest is the SECOND line and not this one — ADR 0058 D3,
			// RetireAsk.clause reaching treeHolds, which says the base has no
			// whitespace-exact twin and does not hold the bytes for the paths
			// it names. Both directions are pinned in verbatimtwin_test.go, so
			// the day the two lines stop disagreeing in that shape, or the
			// second one stops carrying the careful sentence, something says
			// so. Narrowing what THIS line prints is a change to the
			// operator's screen and belongs to whoever revisits D3, not to a
			// comment fix.
			//
			// No off-branch clause below this arm, and it would be false if
			// there were: the base holds these patches whatever ref does or
			// does not name them.
			parts = append(parts, fmt.Sprintf("nothing unlanded (%s)", strings.Join(equivNotes(eq), "; ")))
		case len(eq) > 0:
			// No measurement of content: the -x trailer, or an identity match
			// on a replay. Neither says what a by-hand resolution kept, so
			// RemoveSessionTree still refuses to delete here — the listing
			// should not read as settled either.
			parts = append(parts, fmt.Sprintf("%s commit(s) not on %s by sha, %s — compare before retiring", n, t.Base, unmeasuredClause(eq, t.Base)))
			parts = appendOffBranch(parts, t, head)
		default:
			who := "no record says which bead"
			if t.Bead != "" {
				who = "for " + t.Bead
			}
			parts = append(parts, n+" commit(s) not on "+t.Base+", "+who)
			parts = appendOffBranch(parts, t, head)
		}
	}
	if d := dirtyPaths(t.Path); len(d) > 0 {
		parts = append(parts, fmt.Sprintf("%d uncommitted path(s)", len(d)))
	}
	if len(parts) == 0 {
		return "nothing unlanded"
	}
	return strings.Join(parts, ", ")
}

// appendOffBranch adds the second half of the count treeState just printed:
// WHERE that work is, when it is not on the session branch. The count alone
// sends an operator to `posse worktrees --land`, and landing moves the
// BRANCH — over a detached tree that lands nothing and says so, and a retire
// afterwards takes the commits with it.
//
// It is landed()'s sentence compressed to a listing's width, keeping the
// `branch -f` it prescribes: the two surfaces are reporting the same fact
// and the cure is the same one, and a listing that names the problem without
// it is a line the operator has to take to a second tool.
//
// ONE sentence where landed() has two, because its second cannot happen
// here. That one answers a tree NO branch reaches, and this listing never
// holds such a tree: SessionTreesIn emits a tree only with a `posse/` branch
// that EXISTS — it says so where it recovers the name of a detached one, and
// names a branchless detached tree as its own stated residual, invisible
// here entirely. An arm for it would be a sentence no fixture built through
// ListSessionTrees could reach.
//
// Reached only from the arms where something is genuinely unlanded. A
// measured equivalence is on the base whatever ref does or does not name it,
// so there is nothing there for a missing ref to lose.
func appendOffBranch(parts []string, t *SessionTree, head string) []string {
	if reaches(t.Repo, t.Branch, head) {
		return parts
	}
	return append(parts, fmt.Sprintf("on the tree's detached HEAD (%s) and not on %s, so landing the branch would not carry it — `git -C %s branch -f %s HEAD` first",
		abbrevSHA(head), t.Branch, AbbrevHome(t.Path), t.Branch))
}
