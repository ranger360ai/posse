package posse

// ─── the git child deadline (ranger-base-zfza8) ──────────────────────────────
//
// FOUND by review 2026-09-09 against f97cf1fc (docs/notes.d/ranger-base-b0fsz.md
// §4): posse's two OTHER children are bounded — beads.go's `BdTimeout` and
// herdr.go's `HerdrControlTimeout` each carry a context deadline and a
// termination grace — and its git children were not. `git()` ran
// `exec.Command(...).Run()` with no clock, `patchIDsVerbatim` started two
// children on a pipe and waited for both, and `gitRaw` ran `.Output()`.
// dispatch.go's mergeBack takes the launcher lock and then calls
// MergeSessionWork, which reaches all three. A git child that never returns
// therefore held the lock forever: the watchdog warns, and a warning
// releases nothing.
//
// Code-inspected, never reproduced in production — that is the honest
// provenance of the finding, and it is the same provenance the bd arm was
// built on. What makes it credible anyway is that a git child is the one
// posse controls LEAST: `commit` hands control to whatever the repo's hooks
// are (in this shop, two of bd's), and hooks, filesystem operations and
// their grandchildren are outside the dispatcher's own progress logic.
//
// THE DEADLINE IS SIZED BY THE CALL, for the same reason herdr's is: posse
// makes git calls whose legitimate costs differ by three orders of
// magnitude, and one number cannot bound the fast ones usefully without
// killing the slow ones.
//
// MEASURED 2026-09-11, this box (macOS 26.4.1/APFS, git 2.50.1 Apple
// Git-155), warm cache, against the two live repos an instance of this
// shape runs on — a SOURCE repo of 1,798 commits with 106MB of .git, and the
// QUEUE repo the store of record lives in (ADR 0015 §4), 1,350 commits over
// 5.73GiB of loose objects:
//
//	rev-parse --git-common-dir                      0.084s
//	status --porcelain                              0.128s
//	rev-list --count (900-commit range)             0.127s
//	diff --name-only -z (900-commit range)          0.123s
//	merge-base                                      0.164s
//	log --grep over all history                     0.148s
//	rev-list --full-history -- <path>               0.191s
//	merge-tree --write-tree                         0.115s
//	worktree list                                   0.098s
//	for-each-ref refs/heads                         0.074s
//	worktree add -b <branch> (full checkout)        0.595s
//	log -p | patch-id --verbatim (900 commits)      2.842s
//	log -p | patch-id --verbatim (all 1,798)        2.400s
//	bundle create --all  (the source repo)          2.622s
//	bundle create --all  (the queue repo)         150.827s
//
// Two earlier measurements in this tree agree and extend it: 970 patch-ids
// over a 987-commit range in 5.5s (worktree.go baseHoldsBytes, 2026-09-06),
// and `bundle create --all` over 1.17GiB of loose objects in 12s
// (backup.go, 2026-09-01).
//
// Read the bundle rows together: the SAME VERB is 2.6s in one repo and
// 150.8s in another, and the slow one got 12.5x slower in the ten days
// between the two measurements because the queue's object store grew 5x. A
// flat two-minute deadline over `git()` would have broken `posse backup` on
// this instance the day it landed. That is why there is a table here and not
// a constant, and why the archive tier is sized for growth rather than for
// today's reading.
//
// None of these numbers is a latency budget. Each is the line past which
// posse stops waiting and says so — the difference between a launcher that
// reports a wedged child and a launcher that stops.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// GitTimeout bounds every git call that is not in the table below. The
// slowest ordinary call measured anywhere in this tree is the 5.5s patch-id
// range walk of 2026-09-06, so this is roughly twenty times the worst
// legitimate reading — and six hundred times every call MergeSessionWork
// makes under the launcher lock, which is the number that matters most here:
// it is the longest a wedged git can now hold that lock.
const GitTimeout = 2 * time.Minute

// GitCommitTimeout bounds `git commit`, which is not really a git call at
// all: it runs the repo's configured hooks, and in this shop both slots it
// fills are bd's (`bd hook pre-commit` and `bd hooks run
// prepare-commit-msg`). So the child posse is actually waiting on is a bd,
// and killing it sooner than posse is willing to wait for its OWN bd
// children would make a commit fail where a bd call would have been allowed
// to finish. Sized off BdTimeout for exactly that reason, plus the same
// grace again for the hooks' own work around it.
//
// It matters that this stays modest: `git commit` IS on a launcher-lock
// path — commitQueue (dispatch.go) takes the lock and commits the queue's
// projection through queuejsonl.go.
const GitCommitTimeout = BdTimeout + 2*time.Minute

// GitArchiveTimeout bounds `git bundle`, the one verb MEASURED in minutes
// (150.8s over the queue repo, 2026-09-11) and the one whose cost scales
// with the WHOLE object store rather than with a range a caller bounded. Ten
// times the last reading, because the previous reading was twelve times
// smaller ten days earlier and a deadline that has to be re-measured every
// fortnight is a deadline that will one day be wrong on a Sunday.
//
// Nothing on this tier is on a launcher-lock path: backup.go is the only
// caller.
const GitArchiveTimeout = 30 * time.Minute

// gitKillGrace is how long a TERMed git has to exit before it is killed, and
// how long os/exec keeps waiting on pipes a descendant still holds.
//
// TERM and not KILL — the bd arm's choice rather than the herdr arm's, and
// for a sharper reason than bd's. git installs its own signal handling
// around the lock files it writes (`index.lock`, `<ref>.lock`), and removes
// them on TERM; on KILL it cannot, and what it leaves behind is a repo where
// the NEXT git refuses with "Unable to create '.git/index.lock': File
// exists" and a human has to go and delete it. Half of posse's git calls are
// mutations, so the polite signal is not a courtesy here, it is what keeps a
// blown deadline from needing an operator.
const gitKillGrace = 10 * time.Second

// gitHangw is where a blown git deadline writes its one line. A var only so
// a test can read what was written, on the same reasoning (and in the same
// shape) as beads.go's noticeWriter: callers that discard a git error are
// the rule in this package and not the exception — `_, _ = git(...)` appears
// on the abort and prune paths — so a hang that only became a returned error
// would still be a silent one.
var gitHangw io.Writer = os.Stderr

// GitHangError is a git child that blew its deadline and was signalled.
// Typed for the same reason BdHangError and HerdrHangError are, and for one
// more that is this child's alone: "git said no" and "git said nothing at
// all" are different facts about the REPOSITORY, not just about the box. A
// git that answered left the repo in a state its exit code describes; a git
// that was signalled mid-mutation left it in a state nobody has read.
type GitHangError struct {
	Argv   []string      // the whole child argv, `git` first
	Dir    string        // the -C directory it ran in
	Limit  time.Duration // the deadline it blew
	Waited time.Duration // how long posse actually waited before signalling
	// Mutation is whether this verb could have moved a ref, HEAD or the
	// working tree. It is the difference between a hang a caller may retry
	// and one it must not — see gitReadOnlyVerbs.
	Mutation bool
}

// Error is also the log line. The tail is the part that is about the repo:
// a caller reading this after a mutation has to go and look before it acts,
// and the field's own name for why is at-least-once delivery — a blown
// deadline is a statement about posse's silence, never a verdict that the
// work did not happen (Helland, *Idempotence Is Not a Medical Condition*;
// the shop's note is the distributed-systems skill's
// references/delivery-and-idempotency.md).
func (e *GitHangError) Error() string {
	where := e.Dir
	if where == "" {
		where = "."
	}
	tail := "it is a read, so the repository is as it was"
	if e.Mutation {
		tail = "it is a mutation and MAY HAVE TAKEN EFFECT — read git before acting on this, and do not retry it blind"
	}
	return fmt.Sprintf("git child hung: %s (in %s) — no answer in %s (deadline %s), signalled; %s",
		strings.Join(e.Argv, " "), where, e.Waited.Round(time.Millisecond), e.Limit, tail)
}

// IsGitHang reports whether err is a blown git child deadline.
func IsGitHang(err error) bool {
	var ge *GitHangError
	return errors.As(err, &ge)
}

// gitGlobalValueOpts are the `git` global options that EAT THE NEXT WORD, so
// the verb walk below does not read an option's VALUE as the verb. This is
// gates.go's globalValueOpts lesson applied to the other binary: without it
// `git -c core.hooksPath=x commit` resolves to the verb `core.hooksPath=x`
// and takes the ordinary deadline instead of the commit one.
//
// The `--opt=value` spellings need no entry: they are one word, they start
// with `-`, and the walk skips them.
var gitGlobalValueOpts = map[string]bool{
	"-C": true, "-c": true,
	"--exec-path": true, "--git-dir": true, "--work-tree": true,
	"--namespace": true, "--config-env": true, "--attr-source": true,
	"--super-prefix": true,
}

// gitVerb is the subcommand in args, "" when there is none.
func gitVerb(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return ""
		}
		if strings.HasPrefix(a, "-") {
			if gitGlobalValueOpts[a] {
				i++
			}
			continue
		}
		return a
	}
	return ""
}

// gitReadOnlyVerbs is POSITIVE EVIDENCE that a signalled child cannot have
// moved a ref, HEAD or the working tree. Read-only and not mutating is the
// direction that has to be proved, because the unsafe error here is telling
// an operator nothing changed when something did — so a verb this table does
// not name is reported as a possible mutation, and adding one is a claim.
//
// Some of these refresh the index as a side effect, which is why the
// sentence GitHangError prints is about refs, HEAD and the working tree
// rather than about "nothing". `status` is deliberately NOT here for that
// reason taken to its conclusion: it rewrites the index whenever the stat
// cache is racy, MEASURED on this box 2026-09-10 (dirtyPaths' header).
var gitReadOnlyVerbs = map[string]bool{
	"cat-file": true, "cherry": true, "diff": true, "diff-tree": true,
	"for-each-ref": true, "log": true, "ls-files": true, "ls-tree": true,
	"merge-base": true, "merge-tree": true, "patch-id": true,
	"rev-list": true, "rev-parse": true, "show": true,
}

// gitDeadline is the whole of the sizing rule: the table above, and
// GitTimeout for everything else.
func gitDeadline(args []string) time.Duration {
	switch gitVerb(args) {
	case "commit":
		return GitCommitTimeout
	case "bundle":
		return GitArchiveTimeout
	}
	return GitTimeout
}

// gitHang builds the finding and writes it where it happened.
func gitHang(dir string, args []string, limit, waited time.Duration) *GitHangError {
	hang := &GitHangError{
		Argv:     append([]string{"git", "-C", dir}, args...),
		Dir:      dir,
		Limit:    limit,
		Waited:   waited,
		Mutation: !gitReadOnlyVerbs[gitVerb(args)],
	}
	fmt.Fprintf(gitHangw, "◷ %s\n", hang.Error())
	return hang
}

// bindGitDeadline puts ctx, the TERM and the grace on one git child. Every
// git process posse starts goes through here — the runner, the raw runner
// and the patch-id pipe pair — so there is one choke point and no call site
// that can forget the clock.
func bindGitDeadline(ctx context.Context, cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = gitKillGrace
}

// gitCapture runs one bounded git child in dir. It returns git's stdout
// verbatim, git's stderr trimmed, and the run error — a *GitHangError when
// the deadline blew, and whatever os/exec said otherwise. Turning either
// into the sentence a caller prints is the caller's, because git() and
// gitRaw() word it differently.
func gitCapture(limit time.Duration, dir string, args []string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	bindGitDeadline(ctx, cmd)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	started := time.Now()
	err := cmd.Run()
	if ctx.Err() != nil {
		return out.Bytes(), strings.TrimSpace(errb.String()), gitHang(dir, args, limit, time.Since(started))
	}
	return out.Bytes(), strings.TrimSpace(errb.String()), err
}
