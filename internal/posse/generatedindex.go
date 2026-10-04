package posse

// A GENERATED FILE IS REPRODUCED AT LANDING, NEVER REPLAYED (ranger-base-7h8k4).
//
// docs/notes.d/README.md is a pure function of the fragments beside it:
// `python3 scripts/notes-index.py` writes it, `--check` asks whether it is
// current, and internal/treepins' TestNotesFragmentIndexIsCurrent reds on
// main when it is not. Every seat that writes a fragment is told to run the
// generator and every seat eventually forgets, and the landing — rebase, then
// fast-forward (worktree.go's MergeSessionWork) — had no step that noticed.
//
// MEASURED 2026-10-03, one day, one landing lane: three landings reached
// main with a fragment whose index line was missing (ranger-base-rb05v at
// 09:1x, -knux2 at 14:1x, -4ch00 at 18:0x), each turning the tree pin red on
// main until the index was regenerated and committed as a follow-up; and five
// branches could not fast-forward at all (-wcy4s, -sqxo1, -f1ytb, -99gww,
// -q114b/-sjwhr) because their own index edit conflicted with main's, each
// landed by hand with the same regenerate-then-continue.
//
// Both halves are the same defect read from two ends: a file nobody writes by
// hand was being treated as a file somebody wrote. A hunk against a generated
// file is not information — the inputs are, and they are on the branch. So the
// landing REGENERATES it from the tree it is about to put on main:
//
//   - on the way to a fast-forward, the index is reproduced on the branch and
//     committed there, so what lands is the branch's tip and the tip is
//     current (refreshGeneratedIndex, worktree.go).
//   - when the replay stops on a conflict in nothing BUT the generated file,
//     it is reproduced, staged and the rebase continued, so two fragment
//     branches land back to back without a human in the middle
//     (continuePastGeneratedIndex, below).
//
// WHOSE GENERATOR RUNS, and why that is a decision rather than a detail. The
// program is taken from the BASE's checkout and the inputs from the session
// tree. Running the branch's copy would be the launcher executing
// persona-authored code, uncaged, operator-side, before any human has read the
// diff — the class the constitution belt two screens up in worktree.go exists
// for. So the program is run only while the base's copy and the SESSION
// TREE's copy are byte-identical (generatorIsTheTreesOwn) — which is one
// question covering both ways they can part: a branch that changes the
// generator, and an operator mid-edit on it in their own checkout. Either way
// the landing writes nothing and the index lands as the seat committed it.
// That is not a gap for the first case: the tree pin that reds over a stale
// index runs the TREE's generator against the TREE's index, so a branch
// carrying a new generator has already been asked this by its own suite.
//
// WHAT IT WILL NOT DO. It never writes the generated file while the persona's
// own copy of it is uncommitted (ADR 0041: uncommitted work is theirs, and a
// regeneration would overwrite it silently), and it never touches a tree whose
// branch changed nothing under the fragment directory — a session tree's mtime
// is ADR 0058 fact 4, and a landing that wrote every tree it looked at would
// keep the grace clock from ever reaching zero (ranger-base-9u5zy's shape).
//
// THE RESIDUAL, and it is narrower than it looks. A branch that already
// carries a merge-back block on record is re-probed only when the base has
// moved past the base that replay ran against AND mergesCleanly says the
// whole-branch merge would be taken (landsweep.go's blockStillStands) —
// MEASURED 2026-10-03, git 2.50.1, that filter answers CLEAN for exactly this
// shape: `merge-tree` merges two index insertions in one go where the
// commit-by-commit replay of the same two commits conflicts, which is the
// divergence mergesCleanly's own header states. So a branch blocked before
// this existed does get landed by the next pass whose base has moved. What
// this does not reach is a generated-file conflict merge-tree ALSO refuses:
// there the verdict stands, and a person lands it with the regenerate the
// block's own reason names.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The one member of the class today. A second would need its generator to
// take the same `--directory <dir>` contract, because that is how a program
// in the base's checkout is pointed at a session tree's inputs.
const (
	notesIndexPath      = "docs/notes.d/README.md" // the generated file
	notesIndexGenerator = "scripts/notes-index.py" // the program that writes it
	notesFragmentDir    = "docs/notes.d"           // the inputs it is a function of
	// notesIndexCommand is the command the refusal prescribes, spelled the
	// way the generator's own stderr and NOTES.md spell it — a human reading
	// a refusal should be able to paste it, and should not have to learn a
	// second spelling for the same thing.
	notesIndexCommand = "python3 " + notesIndexGenerator
)

// generatorTimeout bounds the one non-git child the landing path starts. The
// generator reads a directory of markdown and writes one file; a run that
// takes longer than this is not slow, it is stuck, and the landing says so
// rather than holding the launcher lock on it.
const generatorTimeout = 2 * time.Minute

// notesIndexContinues bounds the replay-resolution loop below. Each turn
// resolves ONE replayed commit's conflict, so the bound is a claim about how
// many of a branch's commits may conflict in the generated file alone before
// the landing stops and hands it to a person. Generous rather than tight: the
// loop's own guard is that every conflicted path is the generated file, which
// no amount of looping can widen.
const notesIndexContinues = 20

// runNotesIndex runs the BASE's generator over the SESSION TREE's fragments —
// the program from the checkout a human reviews, the inputs from the branch
// being landed (the header's "whose generator runs"). The error carries the
// program's own stderr, which is the only part of a generator failure worth
// reading.
func runNotesIndex(repo, tree string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), generatorTimeout)
	defer cancel()
	argv := append([]string{
		filepath.Join(repo, notesIndexGenerator),
		"--directory", filepath.Join(tree, notesFragmentDir),
	}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...)
	cmd.Dir = tree
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		said := strings.TrimSpace(oneLine(errb.String()))
		if said == "" {
			said = err.Error()
		}
		return Die("%s: %s", notesIndexCommand, said)
	}
	return nil
}

// regenerateNotesIndex writes the index. This is the one call here that
// changes the session tree.
func regenerateNotesIndex(repo, tree string) error { return runNotesIndex(repo, tree) }

// notesIndexCurrent is the generator's OWN question (`--check`), asked before
// anything is written. It is here for the write it avoids: a seat that ran the
// generator itself has nothing to reproduce, and regenerating anyway would
// rewrite a byte-identical file and touch the session tree's mtime, which is
// ADR 0058 fact 4 — the thing that decides whether that tree can ever be
// retired (ranger-base-9u5zy measured what an every-pass tree write costs).
//
// A nonzero exit is read as "there is something to do" and NOT as a
// diagnosis: the script exits 1 for a stale index and python exits 1 for an
// uncaught exception, and nothing in the exit status tells those apart. So
// the answer only gates the write, and the write's own exit status is what
// any refusal is built from.
func notesIndexCurrent(repo, tree string) bool {
	return runNotesIndex(repo, tree, "--check") == nil
}

// generatorShipped reports whether the base's checkout ships the generator at
// all. A repo without it has no generated index to reproduce, which is every
// repo but this one — the landing sweep walks whatever repos the home names.
func generatorShipped(repo string) bool {
	return fileExists(filepath.Join(repo, notesIndexGenerator))
}

// generatorIsTheTreesOwn is the licence to run the program at all: the base's
// copy and the session tree's copy are the same bytes, so running the base's
// copy — which is the only one this may run — produces what the tree's own
// suite would.
//
// It is a FILE COMPARE and not a reading of the branch's diff, because the
// question has to be answerable in the middle of a replay too: mid-rebase the
// tree holds a partially replayed branch and `base...head` is not a statement
// about anything. The bytes in front of it are. False is "do not run it",
// never a refusal — the index lands as the seat committed it, and
// refreshGeneratedIndex's header says why that is not a gap.
func generatorIsTheTreesOwn(repo, tree string) bool {
	base, err := os.ReadFile(filepath.Join(repo, notesIndexGenerator))
	if err != nil {
		return false
	}
	mine, err := os.ReadFile(filepath.Join(tree, notesIndexGenerator))
	return err == nil && bytes.Equal(base, mine)
}

// differsFromHEAD is "this path is not what the last commit says it is", in
// the tree or in the index or not tracked at all. `status` and not `diff
// HEAD`, because an UNTRACKED generated file is one of the answers that has
// to come back true: a fragment directory arriving whole brings its index
// with it, and `diff HEAD` cannot see a file git has never been told about.
func differsFromHEAD(tree, path string) bool {
	out, err := git(tree, "status", "--porcelain", "--untracked-files=all", "--", path)
	return err == nil && strings.TrimSpace(out) != ""
}

// commitGeneratedIndex mints the landing commit: the generated file alone, on
// the branch, so the fast-forward behind it moves the base to a tip that is
// current. The launcher writing a commit on a persona's behalf is not new —
// LandPersonaMemory does it at a kill — and the message says what occasioned
// it for the same reason.
//
// Path-limited, with the `add` in front of it that a path git has never seen
// needs (rangerhq-4pbt). `--no-verify` is the bd flush hook and nothing else:
// bd's pre-commit stages `.beads/issues.jsonl`, a path-limited commit does not
// take it, and what is left behind is a staged entry over a tree that already
// matches HEAD — which the very next pass reads as the persona's uncommitted
// work and files a closed-dirty handoff about (AGENTS.md, rangerhq-be7k).
// MEASURED 2026-10-03, git 2.50.1: `--no-verify` skips pre-commit and
// prepare-commit-msg still runs, so posse's own commit wall — the data
// ceiling, the visibility guard, the constitution paths — is kept and only
// the flush is skipped. A wall that refuses this commit is reported as a
// refusal to land, like any other obstacle.
//
// THE BEAD ID IS IN THE SUBJECT either way, which is what AGENTS.md asks of
// every commit here: the branch name carries it by construction
// (SessionForBead), so `git log --grep <id>` finds this commit even where the
// branch record was never stamped and t.Bead is empty. The `Bead:` trailer is
// added when the record does name one, the way a memory landing does.
func commitGeneratedIndex(t *SessionTree, path string) error {
	if _, err := git(t.Path, "add", "--", path); err != nil {
		return err
	}
	subject := "landing: regenerate " + path + " (" + t.Branch + ")"
	body := "Committed by posse on the branch before the fast-forward: " + path + " is\n" +
		"generated from " + notesFragmentDir + "/, this branch changed those inputs, and the\n" +
		"committed index did not match them. A landing reproduces a generated file\n" +
		"rather than replaying a hunk against it (ranger-base-7h8k4); `" + notesIndexCommand + "`\n" +
		"is what reproduces it by hand.\n"
	if t.Bead != "" {
		body += "\nBead: " + t.Bead + "\n"
	}
	_, err := git(t.Path, "commit", "--no-verify", "-m", subject, "-m", body, "--", path)
	return err
}

// restoreGeneratedIndex puts the generated file back the way the last commit
// has it, after a regeneration this landing could not finish. The tree is the
// persona's and a landing that refuses must leave nothing behind for the next
// pass to read as their uncommitted work (ADR 0041 §1–§2) — path-limited and
// through `git restore`, which is the undo AGENTS.md prescribes for exactly
// this, never a reset.
func restoreGeneratedIndex(tree, path string) {
	_, _ = git(tree, "restore", "--source=HEAD", "--staged", "--worktree", "--", path)
}

// conflictedPaths is what a stopped replay is waiting on: the unmerged entries
// in the index, in git's own -z spelling so a path holding a quote or a
// control byte comes back as its real bytes (ranger-base-qg0k8's lesson, one
// reader over).
func conflictedPaths(dir string) []string {
	out, err := git(dir, "diff", "--name-only", "--diff-filter=U", "--no-renames", "-z")
	if err != nil {
		return nil
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// continuePastGeneratedIndex drives a stopped replay past a conflict in
// NOTHING BUT the generated index, and answers whether the replay then
// finished. false is "this is not that conflict" or "it could not be
// resolved", and in both the caller aborts and reports the conflict it would
// have reported before this existed — the resolution is an attempt, never a
// promise, and a landing that cannot make it is still a landing that refuses
// rather than one that guesses.
//
// Each turn: regenerate from the tree as the replay has it so far (git has
// already merged every non-conflicted path into it), stage the result, and
// continue. `core.editor=true` because `git rebase --continue` OPENS THE
// EDITOR for the replayed commit's message — MEASURED 2026-10-03, git 2.50.1:
// with GIT_EDITOR=false it exits 1 with "there was a problem with the editor"
// and the rebase stays stopped; with the editor a no-op it keeps the original
// message, which is the one that belongs on the commit.
//
// A regeneration that leaves the replayed commit with NOTHING in it — a
// persona's own index-only commit whose content main already holds — is
// skipped rather than continued, because git refuses an empty `--continue`
// ("No changes") and the commit genuinely has nothing left to say.
func continuePastGeneratedIndex(t *SessionTree) bool {
	if !generatorIsTheTreesOwn(t.Repo, t.Path) {
		return false // not this landing's program to run (see the header)
	}
	for i := 0; i < notesIndexContinues; i++ {
		paths := conflictedPaths(t.Path)
		if len(paths) == 0 {
			return false // not a conflict this knows how to read
		}
		for _, p := range paths {
			if p != notesIndexPath {
				return false // something a person has to resolve
			}
		}
		if err := regenerateNotesIndex(t.Repo, t.Path); err != nil {
			return false
		}
		if _, err := git(t.Path, "add", "--", notesIndexPath); err != nil {
			return false
		}
		verb := "--continue"
		if staged, err := git(t.Path, "diff", "--cached", "--name-only", "HEAD"); err == nil && strings.TrimSpace(staged) == "" {
			verb = "--skip"
		}
		if _, err := git(t.Path, "-c", "core.editor=true", "rebase", verb); err != nil {
			// Stopped again on the next commit is this loop's own business;
			// anything else — a signalled child, a rebase that ended — is the
			// caller's to report, and it reads the tree for itself.
			if IsGitHang(err) || !rebaseStopped(t.Path) {
				return false
			}
			continue
		}
		return true
	}
	return false
}

// branchChangedIndexInputs asks whether this branch changed anything the
// generated index is a function of. ok is false when the question could not be
// put to git at all, which the caller reads as "nothing to do": the
// constitution belt above it refuses to land on a diff it cannot read, so by
// the time anything asks this, this same diff has already been read once this
// pass.
//
// The generated file itself is not an input — a branch whose only change under
// the directory is the index is a seat that already ran the generator, and
// reproducing it there would be a landing commit with nothing in it. Nor is
// the GENERATOR: a change to it is answered by generatorIsTheTreesOwn, in
// bytes, which is the form that also works mid-replay. Asked in the three-dot
// form, like the belt, so a base carrying somebody else's fragment is not this
// branch's to answer for.
func branchChangedIndexInputs(t *SessionTree) (touched, ok bool) {
	head, have := workHead(t)
	if !have {
		return false, false
	}
	out, err := git(t.Repo, "diff", "--name-only", "-z", "--no-renames", t.Base+"..."+head)
	if err != nil {
		return false, false
	}
	for _, p := range strings.Split(out, "\x00") {
		if p != "" && p != notesIndexPath && strings.HasPrefix(p, notesFragmentDir+"/") {
			touched = true
		}
	}
	return touched, true
}

// addPath adds paths to a list without repeating one already there. The
// Regenerated list is a SET of files and the commit count beside it is a
// count: a landing that reproduces the same generated file on two replay
// attempts reproduced one file and made two commits, and both numbers are
// read by a human looking for a commit they did not write.
func addPath(list []string, add ...string) []string {
	for _, p := range add {
		seen := false
		for _, have := range list {
			if have == p {
				seen = true
				break
			}
		}
		if !seen {
			list = append(list, p)
		}
	}
	return list
}
