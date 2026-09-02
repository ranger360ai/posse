package posse

// QA pin for the bd pin bump 0.49.1 -> 0.50.3 (ranger-base-yeg1 step 2,
// operator ruling on ranger-base-qrh1): 0.50.x no longer auto-flushes
// issues.jsonl after a write. Measured (NOTES.md, "the ~5.3s daemon dial is
// fixed upstream — by deletion, at 0.50.0"): `bd create` on 0.49.1 took
// issues.jsonl from 1 row to 2 with no explicit call; the same create on
// 0.50.3 leaves it at 1 row until `bd sync --flush-only`. On 0.49.1 a write
// was durable in the tracked file the instant it happened; on 0.50.x it is
// durable only once something flushes it, so JSONL-as-truth rests on two
// carriers and no third: the launcher's own writer, which flushes explicitly
// (Bd.Flush, queuejsonl.go:127, pinned by TestQueueCommitFlushesBeforeIt-
// Commits), and bd's pre-commit hook, for every commit taken by hand.
//
// WHY IT IS LANDED HERE AND LATE. yeg1's close comment of 2026-08-31 02:21
// reported this pin as landed at internal/rhq/bdflushdiscipline_qa_test.go.
// It was written and never committed: the file sat uncommitted in that
// bead's worktree, on a base older than the internal/rhq -> internal/posse
// rename (9c00e19), while etc/bd/version-pin.toml went on citing it as if it
// were in the tree (ranger-base-fyzqf). This is that file, ported to the
// package the rename left behind, with a fixture witness added.
//
// WHAT IT DOES AND DOES NOT PIN, said plainly so nobody re-reads it as more.
// bd's pre-commit hook is bd's own artifact; posse cannot pin its source. So
// this pins the SHAPE — a write that lands in a database and not in the jsonl
// projection — and the CONSEQUENCE of the hook being absent. It is a pin of
// git's carrier behaviour, like staleindex_qa_test.go's, not a measurement of
// any bd binary. What it would catch is the day git stops carrying a
// hook-staged path into a path-limited commit, which is the single git fact
// JSONL-as-truth now rests on; what it cannot catch is bd shipping a hook
// that does not flush.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bfdRepo is a scratch repo with a stand-in "database" (db.txt) and a tracked
// jsonl projection, seeded equal. When hook is true the pre-commit hook is
// bd's 0.50.x shape: it exports db.txt into the jsonl and stages it, whatever
// pathspec the commit named. Nothing else in this fixture ever touches
// issues.jsonl, which is the point — on 0.50.x nothing else does either.
//
// The hook goes wherever git says hooks go — `git rev-parse --git-path hooks`,
// never a derived `.git/hooks` (ranger-base-flz7).
func bfdRepo(t *testing.T, hook bool) string {
	t.Helper()
	repo := t.TempDir()
	siMust(t, repo, "init", "-q", "-b", "main", ".")
	siMust(t, repo, "config", "user.email", "qa@t")
	siMust(t, repo, "config", "user.name", "qa")
	write(t, filepath.Join(repo, "db.txt"), "row-1\n")
	write(t, filepath.Join(repo, ".beads", beadsJSONL), "row-1\n")
	siMust(t, repo, "add", "-A")
	siMust(t, repo, "commit", "-qm", "base")

	if hook {
		dir := strings.TrimSpace(siMust(t, repo, "rev-parse", "--git-path", "hooks"))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(repo, dir)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir hooks: %v", err)
		}
		body := "#!/bin/sh\ncp db.txt .beads/" + beadsJSONL + "\ngit add .beads/" + beadsJSONL + "\n"
		if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte(body), 0o755); err != nil {
			t.Fatalf("write hook: %v", err)
		}
	}

	// Fixture witness (ranger-base-z4vx). Both arms below read the jsonl out
	// of HEAD and compare it to a literal; a fixture where the jsonl never
	// reached HEAD at row-1 would make the no-hook arm true for the wrong
	// reason, and would make the hook arm's failure unreadable.
	if got := siShow(t, repo, "HEAD", ".beads/"+beadsJSONL); got != "row-1" {
		t.Fatalf("fixture: .beads/%s is %q in HEAD, want row-1 — nothing below measured anything", beadsJSONL, got)
	}
	return repo
}

// bfdWrite is a bd write on 0.50.x: it lands in the database and nowhere
// else. issues.jsonl is untouched until something flushes it — witnessed
// here, because an arm that asserts the jsonl did NOT move proves nothing if
// the write never happened either.
func bfdWrite(t *testing.T, repo, row string) {
	t.Helper()
	write(t, filepath.Join(repo, "db.txt"), row+"\n")
	if got := siShow(t, repo, "worktree", "db.txt"); got != row {
		t.Fatalf("fixture: db.txt is %q after the write, want %q", got, row)
	}
	if got := siShow(t, repo, "worktree", ".beads/"+beadsJSONL); got != "row-1" {
		t.Fatalf("fixture: .beads/%s is %q in the worktree, want the unflushed row-1 — this fixture is not modelling 0.50.x", beadsJSONL, got)
	}
}

// The claim: with bd's own pre-commit hook in place, a write reaches the
// committed jsonl whichever pathspec the commit names — including the
// path-limited form the commit wall requires, which names only the database.
// staleindex_qa_test.go asks the same carrier about the INDEX; this asks it
// about the CONTENT, because on 0.50.x the content is the whole claim: there
// is no auto-flush underneath it to fall back on.
func TestQABdFlushDisciplineHookCarriesTheWriteIntoTheCommit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unqualified", []string{"commit", "-qm", "x"}},
		{"path-limited, names only the database", []string{"commit", "-qm", "x", "--", "db.txt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := bfdRepo(t, true)
			bfdWrite(t, repo, "row-2")
			siMust(t, repo, "add", "db.txt")
			if out, code := siGit(t, repo, tc.args...); code != 0 {
				t.Fatalf("git %s: exit %d\n%s", strings.Join(tc.args, " "), code, out)
			}
			if got := siShow(t, repo, "HEAD", ".beads/"+beadsJSONL); got != "row-2" {
				t.Fatalf("HEAD .beads/%s = %q, want row-2 — the write did not reach the commit", beadsJSONL, got)
			}
		})
	}
}

// The residual the hook does not cover, named rather than assumed: a commit in
// a repo with no pre-commit hook installed — bd never `init`'d there, or the
// hook was never reinstalled — carries whatever the jsonl held at the LAST
// flush, not the write. On 0.49.1 this was survivable regardless, because the
// write had already reached the jsonl by the time any commit ran. On 0.50.x it
// is not: this is the cost NOTES.md prices as "JSONL-as-truth would then rest
// entirely on the pre-commit flush hook."
func TestQABdFlushDisciplineNoHookCommitsStaleJSONL(t *testing.T) {
	t.Parallel()
	repo := bfdRepo(t, false)
	bfdWrite(t, repo, "row-2")
	siMust(t, repo, "add", "db.txt")
	siMust(t, repo, "commit", "-qm", "x")

	if got := siShow(t, repo, "HEAD", ".beads/"+beadsJSONL); got != "row-1" {
		t.Fatalf("HEAD .beads/%s = %q, want the stale row-1 — nothing in a hookless repo would have moved it, which is exactly the gap this pin exists to keep visible", beadsJSONL, got)
	}
}
