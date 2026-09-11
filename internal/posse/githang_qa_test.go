//go:build posse_arm3

package posse

// ranger-base-zfza8: the pins for the clock posse's THIRD child never had.
//
// watchhang_qa_test.go, beside this file, pins the same property for herdr
// and bd (ranger-base-wj7e9). The git child was left out of that round, and
// the review of 2026-09-09 found it still running on a bare
// `exec.Command(...).Run()` — under the launcher lock, via mergeBack →
// MergeSessionWork.
//
// Three things have to be true and each is pinned by execution here: the
// deadline ends the wait, the child is actually signalled (a caller that
// merely gave up would satisfy every assertion about the error and none
// about the box), and the finding is WRITTEN where it happened, because
// `_, _ = git(...)` is an ordinary spelling in this package.
//
// The deadlines themselves are minutes, so the hang arms are run through the
// two seams that take a limit — gitCapture and patchIDsVerbatimWithin. A
// test that waited a production number out would be the hang.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hangingGit puts a `git` on PATH that never answers. It writes its pid
// under pids keyed by the VERB, because the patch-id arm starts two children
// at once and one pid file would only ever hold the loser.
//
// PATH is prepended, not replaced: the script's own `sleep` is resolved
// through it.
func hangingGit(t *testing.T, sleep time.Duration) (pids string) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir, pids := t.TempDir(), t.TempDir()
	// $3 is the verb: every call this package makes is spelled
	// `git -C <dir> <verb> …` (gitCapture builds the argv).
	script := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = --warm ] && exit 0\necho $$ > %q/\"$3\"\nexec sleep %.3f\n",
		pids, sleep.Seconds())
	// WriteExecutable and a --warm run, for the ETXTBSY reason hangingBin's
	// header gives: this script is exec'd on the next line in a package
	// where hundreds of parallel tests fork.
	if err := WriteExecutable(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(filepath.Join(dir, "git"), "--warm").Run(); err != nil {
		t.Fatalf("the hanging git cannot be run at all (%v) — this test would measure nothing", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return pids
}

// captureGitHangw redirects the one line a blown deadline writes. Restored
// on cleanup; safe because every test in this file is env-tainted and
// therefore serial (t.Setenv), which is the same guarantee that lets
// bdshim_test.go shim PATH.
func captureGitHangw(t *testing.T, into *strings.Builder) {
	t.Helper()
	was := gitHangw
	gitHangw = into
	t.Cleanup(func() { gitHangw = was })
}

// The runner arm, in the shape the review described: a git that never
// returns, and a caller that must not wait for it. Run twice, because the
// SENTENCE is different for a read and for a mutation and only the mutation
// one is a fact about the repository.
func TestQAHungGitChildIsSignalledAndNamed(t *testing.T) {
	const sleep = 6 * time.Second
	pids := hangingGit(t, sleep)
	repo := t.TempDir()

	for _, arm := range []struct {
		name string
		args []string
		tail string
		anti string
	}{
		{
			name: "merge",
			args: []string{"merge", "--ff-only", "posse/s-1"},
			tail: "MAY HAVE TAKEN EFFECT",
			anti: "the repository is as it was",
		},
		{
			// The other half, and the one that keeps the mutation sentence
			// meaning something: a read that claimed the repo might have
			// moved would cry wolf on every `rev-list` in a pass.
			name: "rev-list",
			args: []string{"rev-list", "--count", "main..posse/s-1"},
			tail: "the repository is as it was",
			anti: "MAY HAVE TAKEN EFFECT",
		},
	} {
		t.Run(arm.name, func(t *testing.T) {
			var log strings.Builder
			captureGitHangw(t, &log)

			started := time.Now()
			_, _, err := gitCapture(250*time.Millisecond, repo, arm.args)
			waited := time.Since(started)
			pid := awaitPid(t, filepath.Join(pids, arm.name))

			if !IsGitHang(err) {
				t.Fatalf("a git that never answered returned %v, want a GitHangError", err)
			}
			if waited > sleep/2 {
				t.Fatalf("the call waited %s of the child's %s sleep — the deadline did not bound it", waited, sleep)
			}
			for _, want := range append([]string{"git child hung", repo, arm.name, "signalled"}, arm.tail) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the hang error does not name %q:\n%s", want, err.Error())
				}
			}
			if strings.Contains(err.Error(), arm.anti) {
				t.Errorf("a %s hang is described as the other kind:\n%s", arm.name, err.Error())
			}
			if !strings.Contains(log.String(), "git child hung") {
				t.Errorf("nothing was written where the git hang happened; gitHangw got:\n%s", log.String())
			}
			assertReaped(t, pid)
		})
	}
}

// The pipe pair, which is the half of the finding git() alone does not
// cover: two children that wait on EACH OTHER, so either one wedging holds
// the other and the caller. One context has to end both.
func TestQAHungGitPatchIDPairIsSignalled(t *testing.T) {
	const sleep = 6 * time.Second
	pids := hangingGit(t, sleep)
	var log strings.Builder
	captureGitHangw(t, &log)
	repo := t.TempDir()

	started := time.Now()
	ids, err := patchIDsVerbatimWithin(250*time.Millisecond, repo, "main..posse/s-1")
	waited := time.Since(started)

	if !IsGitHang(err) {
		t.Fatalf("a wedged patch-id pair returned (%v, %v), want a GitHangError", ids, err)
	}
	if ids != nil {
		t.Errorf("a hung range walk returned ids (%v) — a partial stream must never be read as an answer, because a commit with no entry is the caller's fail-closed case", ids)
	}
	if waited > sleep/2 {
		t.Fatalf("the call waited %s of the child's %s sleep — the pair's deadline did not bound it", waited, sleep)
	}
	// BOTH, which is the whole point of one context over two children: the
	// old code would have left whichever one it was not waiting on.
	for _, verb := range []string{"log", "patch-id"} {
		assertReaped(t, awaitPid(t, filepath.Join(pids, verb)))
	}
}

// The sizing rule, which is the part a measurement decided (githang.go's
// header carries the readings). Pinned as a table because the numbers are
// the design: one deadline over all of them would have killed `posse
// backup` on this instance — `bundle create --all` over the queue repo was
// MEASURED at 150.8s on 2026-09-11, against 2.6s for the same verb over the
// source repo.
func TestQAGitDeadlineIsSizedByTheCall(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		args []string
		want time.Duration
		why  string
	}{
		{[]string{"rev-parse", "HEAD"}, GitTimeout, "an ordinary read"},
		{[]string{"merge", "--ff-only", "b"}, GitTimeout, "the landing mutation, under the launcher lock"},
		{[]string{"rebase", "main"}, GitTimeout, "the replay, under the launcher lock"},
		{[]string{"commit", "-m", "x", "--", "."}, GitCommitTimeout, "a commit runs the repo's hooks, which in this shop are bd's"},
		{[]string{"bundle", "create", "/tmp/x", "--all"}, GitArchiveTimeout, "MEASURED in minutes, and growing with the object store"},
		// The option-aware half, which is gates.go's globalValueOpts lesson
		// asked of the other binary: a global option that eats the next word
		// must not make that word the verb.
		{[]string{"-c", "core.hooksPath=/dev/null", "commit", "-m", "x"}, GitCommitTimeout, "a global option's VALUE is not the verb"},
		{[]string{"--git-dir=/x/.git", "bundle", "create", "/tmp/x", "--all"}, GitArchiveTimeout, "the joined spelling is one word and is skipped"},
		{[]string{"-c", "commit.gpgsign=false", "rev-parse", "HEAD"}, GitTimeout, "and the value is not read as a verb even when it looks like one"},
	} {
		if got := gitDeadline(c.args); got != c.want {
			t.Errorf("git %s got %s, want %s — %s", strings.Join(c.args, " "), got, c.want, c.why)
		}
	}
	// The commit tier is not a number typed twice: it is bd's, because the
	// child a commit is really waiting on is a bd. If BdTimeout moves and
	// this does not, a commit starts being killed for a bd posse elsewhere
	// is still willing to wait for.
	if GitCommitTimeout <= BdTimeout {
		t.Errorf("GitCommitTimeout (%s) must leave room above BdTimeout (%s) — a commit's hooks ARE bd", GitCommitTimeout, BdTimeout)
	}
	// Sized for the reading, not to it. 150.8s was 12.5x the reading taken
	// ten days earlier over the same repo.
	if GitArchiveTimeout < 10*151*time.Second {
		t.Errorf("GitArchiveTimeout (%s) is under ten times the slowest MEASURED bundle (150.8s, 2026-09-11) — the last two readings were a factor of 12.5 apart", GitArchiveTimeout)
	}
}

// The read/mutate split, which decides the sentence an operator acts on.
// Asked as "these verbs are NOT reads", because the unsafe error here is
// telling someone nothing changed when something did: a verb the table does
// not name is reported as a possible mutation, so the table is a claim and
// every entry in it has to be earned.
func TestQAGitMutatingVerbsAreNeverCalledReads(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{
		"merge", "rebase", "commit", "checkout", "reset", "revert",
		"cherry-pick", "update-ref", "branch", "worktree", "add", "mv",
		"rm", "restore", "clean", "init", "config", "read-tree", "bundle",
		// `status` is the one that looks like a read and is not: it
		// rewrites the index whenever the stat cache is racy, MEASURED on
		// this box 2026-09-10 (dirtyPaths' header, ranger-base-a8tqz).
		"status",
	} {
		if gitReadOnlyVerbs[verb] {
			t.Errorf("`git %s` is listed read-only; a hang in it would be reported as leaving the repository as it was", verb)
		}
	}
	// And the positive half, or the table above is satisfied by an empty
	// map and the mutation sentence is printed over every `rev-parse` in a
	// pass.
	for _, verb := range []string{"rev-parse", "rev-list", "log", "diff", "merge-base", "cat-file"} {
		if !gitReadOnlyVerbs[verb] {
			t.Errorf("`git %s` is not listed read-only, so a hang in it would tell an operator the repository may have moved", verb)
		}
	}
}

// ─── the landing arms ────────────────────────────────────────────────────────

// The re-read a blown deadline owes its caller, and its third state. "Could
// not ask" is not "no": a sentence that flattened them would tell an
// operator the fast-forward did not take effect on the strength of a git
// that would not answer.
func TestQABaseHoldsBranchIsTriState(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "seed.txt", "one\n", "seed")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if held, known := baseHoldsBranch(tr); !known || !held {
		t.Errorf("a branch with nothing ahead of its base reads (held=%v, known=%v), want held and known", held, known)
	}

	commitIn(t, tr.Path, "work.txt", "two\n", "the persona's commit")
	if held, known := baseHoldsBranch(tr); !known || held {
		t.Errorf("a branch one commit ahead reads (held=%v, known=%v), want known and not held", held, known)
	}

	// The third state, and it is reached the way production reaches it — by
	// asking about a base that is not there.
	gone := &SessionTree{Repo: tr.Repo, Path: tr.Path, Branch: tr.Branch, Base: "no-such-base"}
	if held, known := baseHoldsBranch(gone); known {
		t.Errorf("a question git could not answer came back known (held=%v) — ignorance must not be reported as an answer", held)
	}
}

// The sentence a signalled fast-forward gets, over both answers of the
// re-read. The two must not be the same words: one of them means the base
// moved and a human has a landing to check, and the other means it did not.
func TestQAMergeHangReasonSaysWhatItReadBack(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "seed.txt", "one\n", "seed")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	hang := &GitHangError{Argv: []string{"git", "merge", "--ff-only", tr.Branch}, Limit: GitTimeout, Mutation: true}

	commitIn(t, tr.Path, "work.txt", "two\n", "the persona's commit")
	notLanded := mergeHangReason(tr, hang)
	if !strings.Contains(notLanded, "did not take effect") {
		t.Errorf("a base that does not hold the branch must be said so plainly:\n%s", notLanded)
	}

	// Now the shape the retry rule exists for: the fast-forward DID happen,
	// and posse only stopped hearing about it.
	mustGit(t, repo, "merge", "--ff-only", tr.Branch)
	landed := mergeHangReason(tr, hang)
	for _, want := range []string{"DOES now hold", "took effect", "part-way through updating"} {
		if !strings.Contains(landed, want) {
			t.Errorf("a fast-forward that took effect does not say %q:\n%s", want, landed)
		}
	}
	if landed == notLanded {
		t.Fatalf("both answers of the re-read produce one sentence, so the re-read buys nothing:\n%s", landed)
	}
	// And in both: the rule itself, in the bead a person opens some
	// unbounded time later.
	for _, r := range []string{notLanded, landed} {
		if !strings.Contains(r, "nothing was retried") {
			t.Errorf("the reason does not say the retry was refused:\n%s", r)
		}
	}
}

// A replay posse stopped hearing from is not a conflicted one. The old
// wording would have been printed over it verbatim — "the rebase was
// aborted, so this attempt changed nothing" — which is a promise about a
// tree whose state is exactly what is unknown.
func TestQARebaseHangReasonIsNotAConflictReport(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "seed.txt", "one\n", "seed")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	hang := &GitHangError{Argv: []string{"git", "rebase", "main"}, Limit: GitTimeout, Mutation: true}
	r := rebaseHangReason(tr, hang, nil)

	for _, want := range []string{"signalled with no answer", "this is not a conflict", "Nothing was retried"} {
		if !strings.Contains(r, want) {
			t.Errorf("the reason does not say %q:\n%s", want, r)
		}
	}
	// The two sentences the old arms print, asserted ABSENT — and grepped
	// for as they are spelled in MergeSessionWork today, because an
	// assertion of absence dies silently when the wording moves.
	for _, absent := range []string{"conflicts — the rebase was aborted", "so there are no conflicts to resolve"} {
		if strings.Contains(r, absent) {
			t.Errorf("a signalled replay is reported in the conflict arm's words (%q):\n%s", absent, r)
		}
	}
	if !strings.Contains(r, "holds the work") {
		t.Errorf("the reason does not say what the tree reads as after the abort:\n%s", r)
	}
}

// THE ORDERING PIN. The arms above prove the sentences; this proves they are
// reached — that MergeSessionWork asks whether the child was signalled
// BEFORE it decides to replay again.
//
// Structural, because the refusal it guards cannot be executed cheaply: the
// production deadline is two minutes and a test that waited one out would be
// the hang. So the kill is pinned by execution above and the ordering is
// pinned here, which is the division this shop already uses for the seatbelt
// profile's last-match-wins block.
func TestQAMergeSessionWorkRefusesToRetryAHungMerge(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "worktree.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if d, ok := d.(*ast.FuncDecl); ok && d.Name.Name == "MergeSessionWork" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("MergeSessionWork is not in worktree.go — this pin is reading the wrong file")
	}
	src, err := os.ReadFile("worktree.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])

	// Both mutations it makes under the launcher lock: the fast-forward (it
	// runs it twice, before and inside the replay loop) and the replay.
	if n := strings.Count(body, "IsGitHang(err)"); n < 3 {
		t.Errorf("MergeSessionWork guards %d of its git children on IsGitHang, want 3 — the two fast-forwards and the replay", n)
	}
	for _, want := range []string{"mergeHangReason(t, err)", "rebaseHangReason(t, err,"} {
		if !strings.Contains(body, want) {
			t.Errorf("MergeSessionWork never calls %s, so a signalled child is reported in some other arm's words", want)
		}
	}

	// The order inside the replay loop, which is the defect itself. `nowAt`
	// is the measured-move read that licenses another replay; a
	// fast-forward posse stopped hearing from can have caused that move
	// ITSELF, so the hang has to be answered above it or the loop replays a
	// branch onto a base that already holds it.
	loop := body[strings.Index(body, "for attempt := 1;"):]
	hangAt, retryAt := strings.Index(loop, "IsGitHang(err)"), strings.Index(loop, "nowAt := refSHA(")
	switch {
	case hangAt < 0 || retryAt < 0:
		t.Fatalf("the replay loop no longer spells one of the two landmarks (hang %d, retry %d)", hangAt, retryAt)
	case hangAt > retryAt:
		t.Errorf("the replay loop reads the base's move before it asks whether its own fast-forward was signalled — that move can be this call's own effect")
	}
}
