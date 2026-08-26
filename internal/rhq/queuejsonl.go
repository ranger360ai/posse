package rhq

// The queue's git history is not bookkeeping — it is the bead-loss census
// (beadloss.go). `LostBeads` IS the git log of `.beads/issues.jsonl` in
// whatever repo the redirect lands in, so an id that never reaches a commit
// is an id the alarm can never notice leaving. While the store lived in the
// constitution repo the operator's own commits carried the jsonl along (bd's
// pre-commit hook stages it into whatever commit is being made). ADR 0015 §4
// moves the store into a repo nobody commits in for any other reason, and
// takes that free ride away with it: `bd sync` exports the JSONL and does
// NOT commit (measured, `--help`), and `bd sync --full` commits AND pushes,
// which no persona may do.
//
// So the launcher commits it, at the moment it already owns a git act: the
// close it just judged, where it fast-forwards the session branch onto main
// (dispatch.go mergeBack). "Closed means it is on main" gains "and the store
// of record's projection is in a commit".
//
// Measured alternative, rejected and recorded so it is not re-discovered:
// bd 0.49.1 has `bd daemon start --auto-commit` (and a separate --auto-push),
// which would commit the jsonl with no posse code at all. It commits on a 5s
// timer with no bead to name, and its git failures land in daemon.log where
// nobody reads them — including a refusal from the visibility gate, which is
// exactly the failure this repo's hooks exist to make loud. The launcher's
// commit says one line on the pass instead.

import (
	"os"
	"path/filepath"
	"strings"
)

// queueJSONLPaths are the two files the queue repo tracks that a bead can
// change: the projection bd exports, and the deletion ledger beside it. The
// database itself is gitignored by bd's own `.beads/.gitignore` and must
// stay that way — it is 9MB of binary that changes on every mutation.
var queueJSONLPaths = []string{beadsJSONL, beadsDeleted}

// QueueRepo is config `queue_repo:` — the repo ADR 0015 §4 moves the store
// of record into. Absent means the move has not happened, and absent is the
// shipped default: this whole path is inert until the operator writes the
// key, so installing a posse that knows how to commit the jsonl does not
// start committing in the constitution repo the day before the cutover.
func (a *App) QueueRepo() string {
	v := a.CfgGet("queue_repo", "")
	if v == "" {
		return ""
	}
	return filepath.Clean(ExpandTilde(v))
}

// QueueCommit is one attempt's outcome. Skipped carries why nothing was
// committed and is "" exactly when SHA is set — a caller reporting on a
// pass needs to tell "not configured" from "configured and it did nothing",
// because the second one is a close whose record did not reach git.
type QueueCommit struct {
	Repo    string   // the queue repo the store resolved into
	Store   string   // the .beads directory itself
	Paths   []string // what was committed, relative to Store
	SHA     string   // the commit, "" when none was made
	Skipped string   // why not, "" when one was
}

// CommitQueueJSONL flushes the database to its JSONL projection and commits
// that projection in the queue repo. It NEVER pushes: nothing here runs
// `git push`, and the queue repo is created with no remote at all (see
// scripts/queue-cutover.sh), so the guarantee is structural rather than a
// promise about this function's future.
//
// The commit is path-limited — `git commit -- <paths>` — for the reason
// every commit in this codebase is: an unqualified commit takes whatever is
// staged, and the queue repo's index is as shared as any other. It is also
// what makes the commit ignore bd's own untracked droppings (`daemon-error`,
// `daemon.log`) rather than sweeping them in.
//
// dir is any repo whose redirect resolves to the store; msg is the commit
// message, which should name the bead so `git log --grep <id>` finds it.
func (a *App) CommitQueueJSONL(bd Bd, dir, msg string) (QueueCommit, error) {
	q := a.QueueRepo()
	if q == "" {
		return QueueCommit{Skipped: "config queue_repo: is unset — the store has not moved yet (ADR 0015 §4)"}, nil
	}
	store := beadsHome(dir)
	if !underDir(q, store) {
		return QueueCommit{Repo: q, Store: store, Skipped: AbbrevHome(store) + " is not inside " + AbbrevHome(q)}, nil
	}
	c := QueueCommit{Repo: q, Store: store}

	// The database is the store of record and the JSONL is a projection of
	// it; committing without exporting first commits the state before the
	// close. --flush-only is the export with git left alone (measured: the
	// plain `bd sync` does not commit either, but --flush-only says so in
	// its name and cannot grow a git step in a later bd).
	if err := bd.Flush(dir); err != nil {
		return c, err
	}

	for _, name := range queueJSONLPaths {
		if _, err := os.Stat(filepath.Join(store, name)); err == nil {
			c.Paths = append(c.Paths, name)
		}
	}
	if len(c.Paths) == 0 {
		c.Skipped = "no issues.jsonl in " + AbbrevHome(store)
		return c, nil
	}

	// git is run from inside the store directory, so the pathspecs above
	// need no repo root — the same trick beadloss.go uses to walk a
	// redirect target's history without working out where its repo starts.
	dirty, err := git(store, append([]string{"status", "--porcelain", "--"}, c.Paths...)...)
	if err != nil {
		return c, err
	}
	if strings.TrimSpace(dirty) == "" {
		c.Skipped = "the projection already matches its last commit"
		return c, nil
	}
	if _, err := git(store, append([]string{"commit", "-m", msg, "--"}, c.Paths...)...); err != nil {
		return c, err
	}
	sha, err := git(store, "rev-parse", "--short", "HEAD")
	if err != nil {
		return c, err
	}
	c.SHA = strings.TrimSpace(sha)
	return c, nil
}
