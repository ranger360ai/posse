package rhq

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
// means it is on main" stays true for laurie's verify pass (ADR 0006 §3).
//
// WHAT WAS MEASURED, so the next reader does not have to (bd 0.49.1, git
// 2.39.3, in a throwaway repo with a tracked `.beads/issues.jsonl`):
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
//     ignored and bd goes on reading the main graph. Measured in all three
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
	root := ExpandTilde(a.CfgGet("worktrees", DefaultWorktreeRoot()))
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
// prompt, the pass's tree and merge-back lines, `posse worktrees`, and the
// merge-back bead's own title and body. Base == "" is reachable at all of
// them (ranger-base-nfgh), and the ones that skipped it read "never merge
// to  yourself".
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
// Who reads it, since bd 0.49.1 does not (see the CORRECTED bullet above):
// POSSE does. beadsHome (beadloss.go) resolves this file, and the seatbelt
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
	if b, err := os.ReadFile(filepath.Join(src, "redirect")); err == nil {
		if p := strings.TrimSpace(string(b)); p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(t.Repo, p)
			}
			target = filepath.Clean(p)
		}
	}
	dst := filepath.Join(t.Path, ".beads")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return Die("session worktree beads redirect: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "redirect"), []byte(target+"\n"), 0o644); err != nil {
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
// Commits is counted, Merged is only true when the main checkout's branch
// really moved (or had nothing to move for), and Reason names the obstacle
// in the operator's words when it did not.
type MergeOutcome struct {
	Branch  string
	Base    string
	Commits int      // commits on the branch that base did not have
	Merged  bool     // base now contains them
	Rebased bool     // base had moved, so the branch was replayed onto it
	Dirty   []string // uncommitted paths left in the worktree (never merged)
	Reason  string   // why not merged ("" when Merged)
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
	if t.Base == "" {
		o.Reason = fmt.Sprintf("%s has a detached HEAD — there is no branch for %s to land on", AbbrevHome(t.Repo), t.Branch)
		return o, nil
	}
	if !branchExists(t.Repo, t.Branch) {
		o.Merged, o.Reason = true, ""
		return o, nil // nothing was ever committed on it, or it is already gone
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
		o.Merged = true
		return o, nil
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
	if _, err := git(t.Repo, "merge", "--ff-only", t.Branch); err == nil {
		o.Merged = true
		return o, nil
	}
	// Not a fast-forward: the repo's branch moved while the session worked.
	// Replay the session's commits onto it — in the SESSION's tree, so the
	// operator's checkout is untouched by the risky half.
	if len(o.Dirty) > 0 {
		o.Reason = fmt.Sprintf("%s moved on and %s has uncommitted changes (%s), which git will not rebase over", t.Base, AbbrevHome(t.Path), strings.Join(o.Dirty, " "))
		return o, nil
	}
	if _, err := git(t.Path, "rebase", t.Base); err != nil {
		_, _ = git(t.Path, "rebase", "--abort")
		o.Reason = fmt.Sprintf("%s moved on and replaying %s onto it conflicts — the branch is untouched and still holds the work", t.Base, t.Branch)
		return o, nil
	}
	o.Rebased = true
	// The rebase is the slow half, and the operator can switch branches
	// during it. Ask again rather than trust the answer from before it.
	if why := notOnBase(t); why != "" {
		o.Reason = why
		return o, nil
	}
	if _, err := git(t.Repo, "merge", "--ff-only", t.Branch); err != nil {
		o.Reason = fmt.Sprintf("%s still would not fast-forward after the rebase: %v", t.Base, err)
		return o, nil
	}
	o.Merged = true
	return o, nil
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
// while anything would be lost — uncommitted changes, or commits the repo's
// branch does not have — because a session worktree is the only copy of what
// is in it. force is the operator's override and says so in the caller.
func RemoveSessionTree(t *SessionTree, force bool) error {
	if !force {
		if d := dirtyPaths(t.Path); len(d) > 0 {
			return Die("%s has uncommitted changes (%s) — not removed", AbbrevHome(t.Path), strings.Join(d, " "))
		}
		if branchExists(t.Repo, t.Branch) {
			// No base is not "nothing is ahead" — it is "the question
			// cannot be asked", and the safe answer to an unanswerable
			// question about destroying work is no.
			if t.Base == "" {
				return Die("%s still exists and %s has a detached HEAD, so what is unmerged cannot be known — not removed", t.Branch, AbbrevHome(t.Repo))
			}
			n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+t.Branch)
			if err != nil {
				return err
			}
			if n != "0" {
				return Die("%s has %s commit(s) not on %s — not removed", t.Branch, n, t.Base)
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
		if force {
			del = "-D"
		}
		if _, err := git(t.Repo, "branch", del, t.Branch); err != nil {
			return err
		}
	}
	return nil
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

// SessionTreesIn finds every session worktree of the given repos. It reads
// GIT, not the meta dir, on purpose: a kill that could not land its work
// removes the session's meta and leaves the tree standing, so the one record
// that survives every path is the one git keeps.
func SessionTreesIn(dirs []string) ([]*SessionTree, error) {
	var out []*SessionTree
	seen := map[string]bool{}
	for _, dir := range dirs {
		repo, ok := MainCheckout(ExpandTilde(dir))
		if !ok || seen[repo] {
			continue
		}
		seen[repo] = true
		base := repoBranch(repo)
		list, err := git(repo, "worktree", "list", "--porcelain")
		if err != nil {
			return nil, err
		}
		path, branch := "", ""
		flush := func() {
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

// ListSessionTrees prints every session worktree and what each still holds.
func ListSessionTrees(w io.Writer, dirs []string) error {
	trees, err := SessionTreesIn(dirs)
	if err != nil {
		return err
	}
	if len(trees) == 0 {
		fmt.Fprintln(w, "no session worktrees")
		return nil
	}
	for _, t := range trees {
		fmt.Fprintf(w, "%s\n  %s → %s  ·  %s\n", t.Branch, AbbrevHome(t.Path), orDetached(t.Base), treeState(t))
	}
	return nil
}

// LandSessionTrees merges every session branch that will land. It is the
// catch-up for the one case the kill defers: `posse kill` takes the launcher
// lock without waiting so the cockpit cannot freeze on it, and a kill that
// lost that race leaves a merged-nothing tree with its meta already gone
// (rangerhq-09o2).
//
// It MERGES and never removes, and that restraint is the point: this reads
// git, so it cannot tell a tree whose session ended from one a persona is
// working in right now, and removing the second would destroy live work. A
// tree it leaves reading "nothing unlanded" is one a human can retire.
//
// The whole sweep runs under one launcher lock (ADR 0011 §1), taken blocking
// — this is a command a person ran and waiting is the honest thing for it to
// do.
func LandSessionTrees(w io.Writer, a *App, dirs []string) error {
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
		o, err := MergeSessionWork(t)
		switch {
		case err != nil:
			fmt.Fprintf(w, "⚠ %s not landed: %v\n", t.Branch, err)
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

// treeState is the one phrase that says whether anything would be lost by
// removing this tree — the only question the listing exists to answer.
func treeState(t *SessionTree) string {
	var parts []string
	if t.Base == "" {
		parts = append(parts, "repo HEAD is detached — cannot say what is unmerged")
	} else if n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+t.Branch); err == nil && n != "0" {
		parts = append(parts, n+" commit(s) not on "+t.Base)
	}
	if d := dirtyPaths(t.Path); len(d) > 0 {
		parts = append(parts, fmt.Sprintf("%d uncommitted path(s)", len(d)))
	}
	if len(parts) == 0 {
		return "nothing unlanded"
	}
	return strings.Join(parts, ", ")
}
