//go:build posse_arm2

package posse

// QA pins for rangerhq-be7k: a path-limited commit leaves the index holding a
// STALE entry for any path the pre-commit hook staged but the pathspec did not
// name. `git status` then reads `MM` over a worktree that already matches HEAD.
//
// WHY THESE EXIST. rangerhq-8rtf's rationale — carried in
// scripts/audit-silent-reverts.sh and in the commit wall's own comment — named
// exactly one producer of a stale shared index: a commit taken from a private
// `GIT_INDEX_FILE`. It cleared the blessed form in the same breath ("measured:
// it refreshes the shared index for the named paths"). That measurement was
// right and the clearance drawn from it was not: the refresh covers the NAMED
// paths, and a path bd's pre-commit hook flushes and stages is precisely a path
// the pathspec does not name. So the one form the wall permits AND prescribes
// is a second producer of the same stale index, and nobody was looking for it.
//
// THE MECHANISM, measured (git 2.39.3, macOS 25.4, 2026-08-29). git's partial
// commit builds two indexes: it writes the REAL index — refreshed for the
// pathspec only — and holds its lock, then commits from a separate
// `$GIT_DIR/next-index-<pid>.lock`, which is what `GIT_INDEX_FILE` points at
// while the pre-commit hook runs. The hook's `git add` therefore lands in the
// false index (so the flushed file does get committed) and never in the real
// one, which was already written before the hook was called. Attribution
// follows from that and is the thing the bead asked for: nobody WROTE a stale
// entry. git declined to refresh an entry it already had, because the path was
// not named. bd's hook is the trigger, not the writer, and it is doing the only
// thing it can with the index git handed it.
//
// These pins are of GIT's behaviour, not of ours — like the residual
// measurement in commitwall_qa_test.go. That is deliberate: no layer we own can
// close this one (the wall's slot runs before the commit, and git's real index
// lock is written before that), so what the docs claim about it has to be
// judged against a measurement that reruns, or it rots the way the clearance
// above did.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// siGit runs one real git command in dir. It resolves git OUTSIDE the posse
// gates bin: the L1 shim refuses an unqualified `git commit`, and three arms
// below have to run exactly that form to measure what it does.
func siGit(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	bin := resolveOutside("git", "")
	if bin == "" {
		t.Skip("no git outside the gates bin")
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(srEnv(), "PATH="+PathOutsideGates(""))
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), code
}

func siMust(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, code := siGit(t, dir, args...)
	if code != 0 {
		t.Fatalf("git %s: exit %d\n%s", strings.Join(args, " "), code, out)
	}
	return out
}

// siShow is the content of path in one of the three places it can live:
// rev "HEAD" for the commit, "" for the index, or read the working tree.
func siShow(t *testing.T, dir, rev, path string) string {
	t.Helper()
	if rev == "worktree" {
		b, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return strings.TrimSpace(string(b))
	}
	out, code := siGit(t, dir, "show", rev+":"+path)
	if code != 0 {
		return "<absent>"
	}
	return strings.TrimSpace(out)
}

// siRepo is a scratch repo with two tracked files at v1 and, when hook is
// true, a pre-commit hook in bd's shape: it rewrites gen.txt (bd flushes
// .beads/issues.jsonl) and stages it, whatever pathspec the commit named.
//
// The hook goes wherever git says hooks go — `git rev-parse --git-path hooks`,
// never a derived `.git/hooks` (ranger-base-flz7).
func siRepo(t *testing.T, hook bool) string {
	t.Helper()
	repo := t.TempDir()
	siMust(t, repo, "init", "-q", "-b", "main", ".")
	siMust(t, repo, "config", "user.email", "qa@t")
	siMust(t, repo, "config", "user.name", "qa")
	for name, body := range map[string]string{"src.txt": "src-v1\n", "gen.txt": "gen-v1\n"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	siMust(t, repo, "add", "--", "src.txt", "gen.txt")
	siMust(t, repo, "commit", "-qm", "base", "--", "src.txt", "gen.txt")

	if hook {
		dir := strings.TrimSpace(siMust(t, repo, "rev-parse", "--git-path", "hooks"))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(repo, dir)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir hooks: %v", err)
		}
		body := "#!/bin/sh\nprintf 'gen-FLUSHED\\n' > gen.txt\ngit add gen.txt\n"
		if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte(body), 0o755); err != nil {
			t.Fatalf("write hook: %v", err)
		}
	}

	// Fixture witness (ranger-base-z4vx): every arm below reads gen.txt out of
	// three places and compares them. A fixture where gen.txt never reached
	// HEAD would make several of those comparisons true for the wrong reason.
	if got := siShow(t, repo, "HEAD", "gen.txt"); got != "gen-v1" {
		t.Fatalf("fixture: gen.txt is %q in HEAD, want gen-v1 — nothing below measured anything", got)
	}
	return repo
}

// siEdit dirties src.txt, the path every commit form below names.
func siEdit(t *testing.T, repo, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "src.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write src.txt: %v", err)
	}
}

// TestQAStaleIndexAfterPathLimitedCommit is rangerhq-be7k's three measurements,
// taken as the bead took them, plus the control that makes them mean something.
func TestQAStaleIndexAfterPathLimitedCommit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		hook  bool
		stale bool
	}{
		{"hook stages a path the pathspec does not name", true, true},
		{"control: no hook, same commit", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := siRepo(t, tc.hook)
			siEdit(t, repo, "src-v2\n")
			siMust(t, repo, "commit", "-qm", "edit src", "--", "src.txt")

			head := siShow(t, repo, "HEAD", "gen.txt")
			index := siShow(t, repo, "", "gen.txt")
			work := siShow(t, repo, "worktree", "gen.txt")
			status := strings.TrimSpace(siMust(t, repo, "status", "--porcelain"))
			diffHead := strings.TrimSpace(siMust(t, repo, "diff", "HEAD", "--name-only"))

			if !tc.stale {
				if index != head || status != "" || diffHead != "" {
					t.Fatalf("control is not clean: index=%q head=%q status=%q diff HEAD=%q\n"+
						"without a hook the path-limited commit must leave nothing behind",
						index, head, status, diffHead)
				}
				return
			}

			// (1) the flush DID land in the commit — the false index carried it.
			if head != "gen-FLUSHED" {
				t.Fatalf("HEAD gen.txt = %q, want gen-FLUSHED: the hook's add did not reach the commit, so this is not the shape the bead measured", head)
			}
			// (2) the worktree already matches HEAD: there is no work here.
			if work != head || diffHead != "" {
				t.Fatalf("worktree gen.txt = %q, HEAD = %q, diff HEAD = %q; want them equal and the diff empty", work, head, diffHead)
			}
			// (3) and yet the index holds the PRE-flush blob, and status lies.
			if index != "gen-v1" {
				t.Fatalf("index gen.txt = %q, want the stale gen-v1 — git refreshed an entry outside the pathspec, which is the whole bug and it is gone", index)
			}
			if !strings.Contains(status, "MM gen.txt") {
				t.Fatalf("git status = %q, want it to report `MM gen.txt` over a tree that matches HEAD", status)
			}
			// The recovery the bead used, and its precondition: `diff HEAD` is
			// empty above, so unstaging can only discard a content-free entry.
			siMust(t, repo, "restore", "--staged", "--", "gen.txt")
			if got := strings.TrimSpace(siMust(t, repo, "status", "--porcelain")); got != "" {
				t.Fatalf("after `git restore --staged -- gen.txt` status = %q, want clean", got)
			}
		})
	}
}

// TestQAStaleIndexIsUniqueToThePathLimitedForm is the attribution half. The
// stale entry is not "what a pre-commit hook does"; it is what git's PARTIAL
// commit does with one. Give the same hook to `-a` and to `-i` and the index
// comes out refreshed, because both of those commit from a lock that BECOMES
// the real index, so the hook's add lands in the index that survives.
//
// This is why the fix is not in bd and not in the wall: the only form that
// produces the stale entry is the only form the wall permits.
func TestQAStaleIndexIsUniqueToThePathLimitedForm(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		args  []string
		stale bool
	}{
		{"path-limited (the blessed form)", []string{"commit", "-qm", "x", "--", "src.txt"}, true},
		{"commit -a", []string{"commit", "-qam", "x"}, false},
		{"commit -i -- <paths>", []string{"commit", "-qm", "x", "-i", "--", "src.txt"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := siRepo(t, true)
			siEdit(t, repo, "src-v2\n")
			if out, code := siGit(t, repo, tc.args...); code != 0 {
				t.Fatalf("git %s: exit %d\n%s", strings.Join(tc.args, " "), code, out)
			}
			head, index := siShow(t, repo, "HEAD", "gen.txt"), siShow(t, repo, "", "gen.txt")
			if head != "gen-FLUSHED" {
				t.Fatalf("HEAD gen.txt = %q, want gen-FLUSHED — the arm did not commit the flush, so its index verdict measures nothing", head)
			}
			if got := index != head; got != tc.stale {
				t.Fatalf("index gen.txt = %q, HEAD = %q: stale=%v, want stale=%v", index, head, got, tc.stale)
			}
		})
	}
}

// TestQAStaleIndexRevertCarriers asks the question that sizes the harm: once
// the stale entry is there, which later commit form actually takes the old blob
// back into HEAD?
//
// The bead expected `git commit -a` to be a carrier. It is not: -a re-reads
// every modified tracked file from the WORKING TREE, and the working tree is
// correct, so -a overwrites the stale entry rather than committing it. Nor does
// any pathspec commit, `-- .` included. The carriers are the unqualified form
// and `-i`.
//
// AND THEY ONLY CARRY WITH THE HOOK OUT OF THE WAY. In a live bd repo the hook
// that made the stale entry also REPAIRS it: it re-flushes and re-stages the
// file into the very index the next commit is taken from. So a carrier only
// reaches HEAD when the flush is skipped — which is `--no-verify`, and which is
// why TestQAStaleIndexNoVerifyCarrierIsStillWalled below is the one that
// matters. That is the whole of the harm's reachability, and it is the reason
// this is a status-line bug and not a data-loss one.
func TestQAStaleIndexRevertCarriers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		args     []string
		keepHook bool
		reverts  bool
	}{
		{"unqualified commit", []string{"commit", "-qm", "later"}, false, true},
		{"commit -i -- <paths>", []string{"commit", "-qm", "later", "-i", "--", "src.txt"}, false, true},
		{"commit -a", []string{"commit", "-qam", "later"}, false, false},
		{"commit -- .", []string{"commit", "-qm", "later", "--", "."}, false, false},
		{"commit -- <named>", []string{"commit", "-qm", "later", "--", "src.txt"}, false, false},
		// The same two carriers, with bd's hook left where it lives. Its flush
		// repairs the entry ahead of the commit, so neither carries.
		{"unqualified, hook still installed", []string{"commit", "-qm", "later"}, true, false},
		{"commit -i, hook still installed", []string{"commit", "-qm", "later", "-i", "--", "src.txt"}, true, false},
		// --no-verify skips the flush, so the repair goes and the carrier bites.
		{"unqualified --no-verify, hook installed", []string{"commit", "-qm", "later", "--no-verify"}, true, true},
		{"commit -i --no-verify, hook installed", []string{"commit", "-qm", "later", "--no-verify", "-i", "--", "src.txt"}, true, true},
		{"commit -a --no-verify, hook installed", []string{"commit", "-qam", "later", "--no-verify"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := siRepo(t, true)
			siEdit(t, repo, "src-v2\n")
			siMust(t, repo, "commit", "-qm", "the flush lands", "--", "src.txt")
			if got := siShow(t, repo, "", "gen.txt"); got != "gen-v1" {
				t.Fatalf("fixture: index gen.txt = %q, want the stale gen-v1 — there is no stale entry to carry", got)
			}

			hooks := strings.TrimSpace(siMust(t, repo, "rev-parse", "--git-path", "hooks"))
			if !filepath.IsAbs(hooks) {
				hooks = filepath.Join(repo, hooks)
			}
			hookPath := filepath.Join(hooks, "pre-commit")
			if !tc.keepHook {
				if err := os.Remove(hookPath); err != nil {
					t.Fatalf("remove hook: %v", err)
				}
			}
			// Fixture witness for the keepHook arms: an arm that credits the
			// hook with a repair, over a repo where the hook is gone, is
			// measuring the wrong thing (ranger-base-z4vx).
			if _, err := os.Stat(hookPath); (err == nil) != tc.keepHook {
				t.Fatalf("fixture: pre-commit present=%v, want %v", err == nil, tc.keepHook)
			}

			siEdit(t, repo, "src-v3\n")
			siMust(t, repo, "add", "--", "src.txt")
			if out, code := siGit(t, repo, tc.args...); code != 0 {
				t.Fatalf("git %s: exit %d\n%s", strings.Join(tc.args, " "), code, out)
			}

			// Positive witness that the later commit happened at all — an arm
			// asserting "gen.txt was NOT rolled back" is satisfied by a commit
			// that never ran (ranger-base-z4vx).
			if got := siShow(t, repo, "HEAD", "src.txt"); got != "src-v3" {
				t.Fatalf("HEAD src.txt = %q, want src-v3: the later commit did not land, so its verdict on gen.txt is worthless", got)
			}
			got := siShow(t, repo, "HEAD", "gen.txt")
			if reverted := got == "gen-v1"; reverted != tc.reverts {
				t.Fatalf("after the later commit HEAD gen.txt = %q: reverted=%v, want reverted=%v", got, reverted, tc.reverts)
			}
		})
	}
}

// TestQAStaleIndexNoVerifyCarrierIsStillWalled closes the loop. The one live
// route from a stale entry to a silent revert is a carrier run with
// `--no-verify` (the table above). `--no-verify` skips pre-commit — which is
// exactly why the wall was put in prepare-commit-msg instead, and
// prepare-commit-msg still runs. So the wall refuses the only reachable
// carrier, and this is the pin that says the slot choice is load-bearing rather
// than incidental.
func TestQAStaleIndexNoVerifyCarrierIsStillWalled(t *testing.T) {
	t.Parallel()
	repo, git, persona := commitWallRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out, err := git(persona, "add", "--", "f.txt"); err != nil {
		t.Fatalf("add: %v %s", err, out)
	}
	// Control: the wall lets the blessed form through, --no-verify or not, or
	// the refusal below is just "this repo refuses everything".
	if out, err := git(persona, "commit", "-m", "base", "--no-verify", "--", "f.txt"); err != nil {
		t.Fatalf("the blessed form must still pass with --no-verify: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out, err := git(persona, "add", "--", "f.txt"); err != nil {
		t.Fatalf("add: %v %s", err, out)
	}
	out, err := git(persona, "commit", "-m", "carrier", "--no-verify")
	if err == nil {
		t.Fatalf("an unqualified `git commit --no-verify` was ACCEPTED — --no-verify skips pre-commit, so this is the one form that can carry a stale index entry into HEAD:\n%s", out)
	}
	if !strings.Contains(out, "refused by posse gate") {
		t.Fatalf("refused, but not by the wall — prepare-commit-msg may not be running under --no-verify:\n%s", out)
	}
}

// TestQAAgentsMdNamesTheStaleIndexCheck pins the prescription where a persona
// reads it. The wall cannot say this: it fires on refusal, and the form that
// produces the stale entry is never refused. So the landing checklist is the
// only place it can be said, and a checklist line is only as good as something
// that notices when it goes.
//
// Pinned as SHAPE, not as prose (ranger-base-tff): the check command, the
// recovery command, and the precondition that makes the recovery safe.
func TestQAAgentsMdNamesTheStaleIndexCheck(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Skipf("AGENTS.md not present: %v", err)
	}
	doc := string(b)
	for _, want := range []string{
		"git status", // the signal that misreports
		// The check that tells the truth, and the flag that lets it run: a
		// seat's GIT_EXTERNAL_DIFF was set EMPTY by posse's own inlet pin,
		// and a bare `git diff HEAD` died rc 128 there on exactly the
		// non-empty case this check exists to detect (ranger-base-l1ix2).
		// The pin dropped that row on ranger-base-888fv, so a seat no longer
		// sets it — but the flag stays prescribed, because diff.external in
		// any gitconfig and a `diff=<driver>` attribute reach the same
		// output with no environment at all, and the env name is now an open
		// inlet rather than one posse holds shut.
		"git diff --no-ext-diff HEAD -- <paths>",
		"git restore --staged -- <paths>", // the recovery
		"rangerhq-be7k",                   // where the measurement lives
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("AGENTS.md's landing checklist no longer names %q — a persona who follows the blessed form has no way to read the `MM` it leaves", want)
		}
	}
}
