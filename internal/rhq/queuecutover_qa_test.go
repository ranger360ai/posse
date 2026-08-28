package rhq

// QA pins for scripts/queue-cutover.sh (ADR 0015 §4, ranger-base-tjfw,
// verified on ranger-base-lpz4). The script had no automated coverage at
// all: it was rehearsed once by hand on a copy of the live store, and the
// fixture was deleted afterwards. What the rehearsal proved is worth
// keeping, because the thing it proves is not "the script runs" — it is
// that the bead-loss census (beadloss.go) SURVIVES the move. A queue repo
// that starts at one fresh commit reports "no lost beads" forever, with no
// error anywhere, and that is indistinguishable from a healthy fleet.
//
// Most fixtures below keep the store CLEAN — the working tree matches the
// last commit and nothing untracked sits in `.beads`. That used to be
// load-bearing: with drift the script reached an UNQUALIFIED
// `git add -A .beads && git commit -m '<msg>'`, a persona cage denying
// `Bash(git commit unless --)` refused it, and `set -eu` aborted the script
// between the `mv` and the redirect write with no message at all — measured
// on lpz4, filed as ranger-base-nzyn. Fixed there: the commit is
// path-qualified, it runs LAST (after the redirects, where a failure costs
// one commit and nothing else), and every step below the preflight prints
// the half-state it left. qcDrift + the three nzyn pins at the bottom cover
// that, and they are why this file no longer avoids drift.
//
// These tests do NOT strip a persona's gate shim from PATH: post-nzyn
// nothing here runs a command a cage denies, so a caged session running them
// is a free extra arm rather than a red suite.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// qcScript resolves the cutover script, skipping when this checkout does not
// carry it (a tarball, a worktree pruned for a build).
func qcScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../scripts/queue-cutover.sh")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("cutover script not present: %v", err)
	}
	return p
}

// qcRunbook is the same for docs/runbooks/queue-cutover.md — the window step
// the script is only half of. Two of the pins below are about its text: the
// operator reads it under time pressure with the fleet quiesced, which is
// the worst moment for a step to be missing.
func qcRunbook(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../docs/runbooks/queue-cutover.md")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("runbook not present: %v", err)
	}
	return string(b)
}

// qcConstitution is a constitution repo in miniature: a root `.gitignore`
// that excludes part of `.beads` (the live one excludes `.beads/export-state/`
// and `.beads/interactions.jsonl`), a bd-shaped `.beads/.gitignore`, and a
// history that adds three beads and then DROPS one without a ledger record.
// That last commit is the census's raw material — the whole point of the
// replay is that it still reads after the move.
func qcConstitution(t *testing.T) (repo, droppedID string) {
	t.Helper()
	repo = t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, ".gitignore"), ".beads/export-state/\n.beads/interactions.jsonl\n")
	write(t, filepath.Join(repo, ".beads", ".gitignore"), "*.db\n*.db?*\ndaemon.log\ndaemon.pid\nredirect\n")
	write(t, filepath.Join(repo, "PROSE.md"), "the constitution's own content\n")
	mustGit(t, repo, "add", ".gitignore", ".beads/.gitignore", "PROSE.md")
	mustGit(t, repo, "commit", "-q", "-m", "seed", "--", ".gitignore", ".beads/.gitignore", "PROSE.md")

	jsonl := filepath.Join(repo, ".beads", beadsJSONL)
	write(t, jsonl, `{"id":"q-1","title":"one"}`+"\n"+`{"id":"q-2","title":"two"}`+"\n"+`{"id":"q-3","title":"three"}`+"\n")
	write(t, filepath.Join(repo, ".beads", beadsDeleted), "")
	mustGit(t, repo, "add", ".beads/"+beadsJSONL, ".beads/"+beadsDeleted)
	mustGit(t, repo, "commit", "-q", "-m", "beads: three beads", "--", ".beads")

	// A commit of the constitution's OWN prose, which must not become a
	// commit in the queue's log: an unchanged .beads is not a queue commit.
	write(t, filepath.Join(repo, "PROSE.md"), "more prose, no beads\n")
	mustGit(t, repo, "commit", "-q", "-m", "prose only", "--", "PROSE.md")

	droppedID = "q-2"
	write(t, jsonl, `{"id":"q-1","title":"one"}`+"\n"+`{"id":"q-3","title":"three"}`+"\n")
	mustGit(t, repo, "commit", "-q", "-m", "beads: q-2 leaves, with no ledger record", "--", ".beads")
	return repo, droppedID
}

// qcWork is a repo the fleet works in: `.beads` holding a redirect and
// nothing else, pointed at the store's CURRENT home.
func qcWork(t *testing.T, dir, store string) string {
	t.Helper()
	write(t, filepath.Join(dir, ".beads", beadsRedirect), store+"\n")
	return dir
}

// qcRun drives the script. Every path is overridden, always: the defaults
// are the LIVE ones (~/src/ranger-base, ~/src/ranger-queue, ~/src/posse,
// ~/.posse/worktrees), and `--only-redirect` rather than `--redirect`
// because `--redirect` APPENDS to a default that is the live posse checkout.
func qcRun(t *testing.T, constitution, queue, worktrees string, redirects []string, extra ...string) (string, error) {
	t.Helper()
	return qcRunEnv(t, nil, constitution, queue, worktrees, redirects, extra...)
}

// qcRunEnv is qcRun with entries appended to the script's environment — the
// seam the abort pins drive: a `git` shim on PATH, or a GIT_TEMPLATE_DIR
// whose pre-commit hook refuses.
func qcRunEnv(t *testing.T, env []string, constitution, queue, worktrees string, redirects []string, extra ...string) (string, error) {
	t.Helper()
	args := []string{qcScript(t),
		"--constitution", constitution,
		"--queue", queue,
		"--worktrees", worktrees,
	}
	for i, r := range redirects {
		if i == 0 {
			args = append(args, "--only-redirect", r)
			continue
		}
		args = append(args, "--redirect", r)
	}
	args = append(args, extra...)
	cmd := exec.Command("sh", args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// qcDrift makes the live store differ from the constitution's last commit,
// the way the real one does every minute it is up: a bead added to the
// projection and a database beside it. It is what makes the script reach its
// commit at all.
func qcDrift(t *testing.T, constitution string) {
	t.Helper()
	write(t, filepath.Join(constitution, ".beads", beadsJSONL),
		`{"id":"q-1","title":"one"}`+"\n"+`{"id":"q-3","title":"three"}`+"\n"+`{"id":"q-4","title":"four, uncommitted"}`+"\n")
	write(t, filepath.Join(constitution, ".beads", "beads.db"), "not really a database\n")
}

// qcCageShim puts a `git` on PATH that refuses an unqualified `git commit`
// the way a persona cage does, and returns the PATH entry for it. Modelled
// on the rendered gate (skip git's leading global options, then match the
// verb; `--` counts only with an operand after it) so that what it refuses
// is what the real one refuses.
func qcCageShim(t *testing.T) string {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("no git on PATH: %v", err)
	}
	dir := t.TempDir()
	shim := filepath.Join(dir, "git")
	write(t, shim, `#!/bin/sh
qualified() {
  while [ $# -gt 0 ]; do
    if [ "$1" = '--' ]; then [ $# -gt 1 ] && return 0; return 1; fi
    shift
  done
  return 1
}
verb() {
  while [ $# -gt 0 ]; do
    case $1 in
      -C|-c|--git-dir|--work-tree) [ $# -ge 2 ] || break; shift 2 ;;
      -*) shift ;;
      *) printf '%s' "$1"; return 0 ;;
    esac
  done
}
if [ "$(verb "$@")" = commit ] && ! qualified "$@"; then
  echo "refused by the cage: git $* (deny: Bash(git commit unless --))" >&2
  exit 1
fi
exec '`+gitBin+`' "$@"
`)
	if err := os.Chmod(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The claim the whole replay exists for, asserted the way it fails in
// production: not "does the script run" but "does the alarm still ring". A
// queue repo whose history starts fresh answers "no lost beads" to a store
// that has lost one, forever, and says nothing about it — so the
// counterfactual is asserted in the same test, or the pin proves nothing.
func TestQueueCutoverCarriesTheCensusIntoTheQueueRepo(t *testing.T) {
	constitution, dropped := qcConstitution(t)
	queue := filepath.Join(t.TempDir(), "queue")
	worktrees := t.TempDir()
	project := qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))

	before, err := removedBeads(constitution)
	if err != nil {
		t.Fatalf("removedBeads(constitution): %v", err)
	}
	if _, ok := before[dropped]; !ok {
		t.Fatalf("the fixture's census does not see %s leaving; it has nothing to preserve", dropped)
	}

	out, err := qcRun(t, constitution, queue, worktrees, []string{project})
	if err != nil {
		t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
	}

	after, err := removedBeads(project) // through the redirect the script rewrote
	if err != nil {
		t.Fatalf("removedBeads(project): %v", err)
	}
	lb, ok := after[dropped]
	if !ok {
		t.Fatalf("the census is DISARMED after the move: %s no longer reads as lost\n%s", dropped, out)
	}
	// The replay renames every commit, so the sha is expected to differ —
	// the author time is what identifies the same removal across it, and
	// it is what the census reports to a human.
	if !lb.When.Equal(before[dropped].When) {
		t.Errorf("the replayed removal is not the same event: %v, want %v", lb.When, before[dropped].When)
	}
	if lb.Commit == before[dropped].Commit {
		t.Errorf("the replay did not rename the commit (%s) — it is not a replay", lb.Commit)
	}

	// The counterfactual, in the same test: the naive move — copy the
	// projection into a repo that starts at one commit — and the alarm goes
	// quiet with no error anywhere.
	naive := t.TempDir()
	mustGit(t, naive, "init", "-q", "-b", "main", ".")
	mustGit(t, naive, "config", "user.email", "t@example.com")
	mustGit(t, naive, "config", "user.name", "t")
	body, err := os.ReadFile(filepath.Join(queue, ".beads", beadsJSONL))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(naive, ".beads", beadsJSONL), string(body))
	mustGit(t, naive, "add", ".beads/"+beadsJSONL)
	mustGit(t, naive, "commit", "-q", "-m", "fresh start", "--", ".beads")
	if got, err := removedBeads(naive); err != nil {
		t.Errorf("removedBeads(naive): %v", err)
	} else if len(got) != 0 {
		t.Errorf("the counterfactual is not what it claims: a fresh-start repo censused %d removal(s)", len(got))
	}
}

// What the constitution repo is left holding, and what the queue repo is
// NOT given. "Never pushes" is structural because there is nothing to push
// to; a remote added later is a decision somebody has to make out loud
// (ranger-base-xhsb is the operator's open question about exactly that).
func TestQueueCutoverLeavesARedirectAndARepoWithNoRemote(t *testing.T) {
	constitution, _ := qcConstitution(t)
	queue := filepath.Join(t.TempDir(), "queue")
	project := qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))

	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
	if err != nil {
		t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
	}

	left, err := os.ReadDir(filepath.Join(constitution, ".beads"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].Name() != beadsRedirect {
		var names []string
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Errorf("the constitution keeps more than a redirect: %v", names)
	}
	if got := strings.TrimSpace(mustGit(t, queue, "remote", "-v")); got != "" {
		t.Errorf("the queue repo was given a remote it could push to:\n%s", got)
	}
	// The constitution's prose must not have come along: the replay carries
	// `.beads` and nothing else, which is what turns 152M of objects into
	// single-digit megabytes.
	if files := mustGit(t, queue, "ls-files"); strings.Contains(files, "PROSE.md") {
		t.Errorf("the queue repo carries the constitution's prose:\n%s", files)
	}
	// …and the untracking is staged, not committed: that is what makes the
	// runbook's rollback cheap.
	if staged := mustGit(t, constitution, "diff", "--cached", "--name-only"); !strings.Contains(staged, ".beads/") {
		t.Errorf("the .beads untracking is not staged, so a rollback has nothing to reset: %q", staged)
	}
}

// Redirect discovery walks directories the operator names, so it can reach
// the queue repo itself — and a redirect INSIDE the store is a one-hop cycle
// that resolves to the directory it is already in. It looks fine until
// something follows the chain twice.
func TestQueueCutoverNeverPointsTheStoreAtItself(t *testing.T) {
	constitution, _ := qcConstitution(t)
	queue := filepath.Join(t.TempDir(), "queue")
	worktrees := t.TempDir()
	project := qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))
	session := qcWork(t, filepath.Join(worktrees, "posse", "sess-1"), filepath.Join(constitution, ".beads"))

	// The queue repo is named as a redirect target, which is the operator
	// error the guard exists for.
	out, err := qcRun(t, constitution, queue, worktrees, []string{queue, project})
	if err != nil {
		t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
	}

	store := filepath.Join(queue, ".beads")
	if _, err := os.Stat(filepath.Join(store, beadsRedirect)); err == nil {
		t.Errorf("the store redirects to itself: %s/%s exists", store, beadsRedirect)
	}
	for _, r := range []string{project, session} {
		got, err := os.ReadFile(filepath.Join(r, ".beads", beadsRedirect))
		if err != nil {
			t.Fatalf("%s: %v", r, err)
		}
		if strings.TrimSpace(string(got)) != store {
			t.Errorf("%s still redirects to %q, want %q", r, strings.TrimSpace(string(got)), store)
		}
	}
	// The session worktree above was found by discovery, not named — that
	// is the arm that keeps the fleet's OPEN sessions working, and the one
	// with nobody to notice if it silently stops walking.
	if !strings.Contains(out, session) {
		t.Errorf("discovery did not reach the session worktree under --worktrees:\n%s", out)
	}
}

// Every preflight refusal is a statement that the window is not open yet,
// and each one fails CLOSED — the script writes nothing before it refuses.
func TestQueueCutoverRefusesGroundItDoesNotExpect(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, constitution, queue string)
		want  string
	}{
		{"a queue repo that already exists", func(t *testing.T, _, queue string) {
			write(t, filepath.Join(queue, "keep-me"), "not mine to write into\n")
		}, "already exists"},
		{"a store that already redirects", func(t *testing.T, c, _ string) {
			write(t, filepath.Join(c, ".beads", beadsRedirect), "/somewhere/else\n")
		}, "already redirects"},
		// This test's own pid, and not pid 1: the guard is `kill -0`, which
		// a non-root caller cannot send to a root process — it comes back
		// EPERM and the script reads that as "no daemon". Fail-open, and
		// the one arm of this preflight that is (bd's daemon runs as the
		// operator, so it does not bite here).
		{"a live daemon holding the database", func(t *testing.T, c, _ string) {
			write(t, filepath.Join(c, ".beads", "daemon.pid"), strconv.Itoa(os.Getpid())+"\n")
		}, "daemon is running"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			constitution, _ := qcConstitution(t)
			queue := filepath.Join(t.TempDir(), "queue")
			tc.setup(t, constitution, queue)

			out, err := qcRun(t, constitution, queue, t.TempDir(), []string{qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))})
			if err == nil {
				t.Fatalf("the script ran on ground it should have refused:\n%s", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("the refusal does not say why (want %q):\n%s", tc.want, out)
			}
			if _, err := os.Stat(filepath.Join(constitution, ".beads", beadsJSONL)); err != nil {
				t.Errorf("it moved the store before refusing: %v", err)
			}
		})
	}
}

// ranger-base-imfi: the script's own closing instructions omit
// `bd migrate --update-repo-id` and tell the operator to start the daemon
// instead. The runbook marks that step NOT OPTIONAL — bd stamps the
// database with a repo id, the queue repo is a different repo, and bd's own
// words for the mismatch are that the git-history backfill "may treat your
// local issues as deleted". Measured on lpz4: after the move, bd drops
// `.beads/daemon-error` carrying exactly that text. It fails closed, so
// this costs a window rather than a database — but the script's last four
// lines are what an operator is looking at when they run step 3.
func TestQueueCutoverInstructionsNameTheRepoIdMigrate(t *testing.T) {
	t.Skip("ranger-base-imfi: the script's next-steps block omits bd migrate --update-repo-id")
	body, err := os.ReadFile(qcScript(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "--update-repo-id") {
		t.Error("the closing instructions never name the migrate the runbook marks NOT OPTIONAL")
	}
}

// ranger-base-4l2z: `git add -A .beads` in the queue repo stages files the
// CONSTITUTION deliberately gitignores. The exclusions live in the
// constitution's ROOT `.gitignore`, and the replay carries the `.beads`
// subtree only — so `.beads/export-state/` and `.beads/interactions.jsonl`
// arrive in the queue repo unignored and become tracked. Measured on lpz4:
// 44 export-state files, each naming a worktree path, a persona and a bead
// id, version-controlled in the store of record.
func TestQueueCutoverDoesNotVersionWhatTheConstitutionIgnores(t *testing.T) {
	t.Skip("ranger-base-4l2z: the queue repo's first commit sweeps .beads paths the constitution's root .gitignore excludes")
	constitution, _ := qcConstitution(t)
	write(t, filepath.Join(constitution, ".beads", "export-state", "abc.json"),
		`{"worktree_root":"/Users/someone/.posse/worktrees/posse/dinesh-x"}`+"\n")
	queue := filepath.Join(t.TempDir(), "queue")

	out, _ := qcRun(t, constitution, queue, t.TempDir(), []string{qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))})
	for _, probe := range [][]string{{"ls-files", ".beads/export-state"}, {"diff", "--cached", "--name-only"}} {
		got, err := git(queue, probe...)
		if err != nil {
			continue
		}
		if strings.Contains(got, "export-state") {
			t.Errorf("`git %s` shows the constitution's ignored store files in the queue repo:\n%s\n%s",
				strings.Join(probe, " "), got, out)
		}
	}
}

// ranger-base-g1js, the other half: the runbook's rollback moves the store
// home with `mv ~/src/ranger-queue/.beads/* ...`, and that glob does not
// match dotfiles. Rehearsed on lpz4: `.beads/.gitignore` stays behind, and
// the constitution repo comes back with `beads.db` UNTRACKED AND UNIGNORED —
// one `git add -A` away from committing 10MB of binary database into the
// repo ADR 0015 exists to keep clean, with the tracked `.gitignore` showing
// as deleted beside it.
func TestQueueRollbackCarriesTheStoresDotfilesHome(t *testing.T) {
	t.Skip("ranger-base-g1js: the rollback's `mv .beads/*` leaves .beads/.gitignore in the queue repo")
	body := qcRunbook(t)
	i := strings.Index(body, "## Rollback")
	if i < 0 {
		t.Fatal("the runbook has no rollback section")
	}
	if strings.Contains(body[i:], "/.beads/* ") {
		t.Error("the rollback moves the store home with a glob that skips dotfiles, " +
			"so .beads/.gitignore stays behind and beads.db comes back unignored")
	}
}

// ─── ranger-base-nzyn: the window, and what an abort in it says ─────────────

// The commit the script makes must be one every session can make. It used to
// be `git commit -m <msg>` with no pathspec, which a persona cage denying
// `Bash(git commit unless --)` refuses — and it sat between the mv and the
// redirect, so the refusal killed bd fleet-wide (see the abort pin below).
// `git add -A .beads` stages nothing else, so the qualified form is the same
// commit: measured, same tree sha, deletions included.
func TestQueueCutoverCommitsDriftWithAPathQualifiedCommit(t *testing.T) {
	shimDir := qcCageShim(t)
	// The witness that the fixture blocks anything at all: the shim refuses
	// the unqualified form in a repo of its own. Without it this pin is
	// green against a shim that never fires.
	scratch := t.TempDir()
	mustGit(t, scratch, "init", "-q", "-b", "main", ".")
	write(t, filepath.Join(scratch, "f"), "x\n")
	caged := exec.Command(filepath.Join(shimDir, "git"), "-C", scratch, "commit", "-q", "-m", "unqualified")
	if out, err := caged.CombinedOutput(); err == nil {
		t.Fatalf("the cage shim does not refuse an unqualified commit; this pin measures nothing:\n%s", out)
	}

	constitution, _ := qcConstitution(t)
	qcDrift(t, constitution)
	queue := filepath.Join(t.TempDir(), "queue")
	project := qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))

	out, err := qcRunEnv(t, []string{"PATH=" + shimDir + string(os.PathListSeparator) + os.Getenv("PATH")},
		constitution, queue, t.TempDir(), []string{project})
	if err != nil {
		t.Fatalf("the script cannot run to completion from a caged session: %v\n%s", err, out)
	}
	// The drift is IN the queue repo's history, not merely on disk: an
	// uncommitted store is a store the census cannot read.
	if head := mustGit(t, queue, "show", "--stat", "--oneline", "HEAD"); !strings.Contains(head, beadsJSONL) {
		t.Errorf("the live store's drift was not committed in the queue repo:\n%s\n%s", head, out)
	}
	if body := mustGit(t, queue, "show", "HEAD:.beads/"+beadsJSONL); !strings.Contains(body, "q-4") {
		t.Errorf("the committed projection is not the live one: %q", body)
	}
}

// The bead's headline (ranger-base-nzyn, hit on lpz4): an abort between the
// mv and the redirect left the constitution's `.beads` EMPTY, the store in
// the queue repo, every redirect in the fleet naming the empty directory —
// and said nothing, because `set -eu` exits silently. Whatever aborts it, it
// must name the half-state and how to undo it.
func TestQueueCutoverAbortInTheMoveWindowNamesTheHalfStateAndItsUndo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory mode this fixture blocks the mv with")
	}
	constitution, _ := qcConstitution(t)
	src := filepath.Join(constitution, ".beads")
	queue := filepath.Join(t.TempDir(), "queue")
	project := qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))

	// A read-only store directory: every rename OUT of it fails, which is
	// this window's failure with no cage and no hook involved.
	if err := os.Chmod(src, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(src, 0o755) })

	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
	if err == nil {
		t.Fatalf("the fixture did not block the move, so this pin measures nothing:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(src, beadsJSONL)); statErr != nil {
		t.Fatalf("the fixture blocked something other than the mv (%v):\n%s", statErr, out)
	}

	for _, want := range []string{
		"ABORTED",
		`stage "move"`,
		src,
		filepath.Join(queue, ".beads"),
		"UNDO",
		"Rollback",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the abort never says %q:\n%s", want, out)
		}
	}

	// The control, on its own fixture: a run that succeeds says none of it.
	clean, _ := qcConstitution(t)
	ok, err := qcRun(t, clean, filepath.Join(t.TempDir(), "queue"), t.TempDir(),
		[]string{qcWork(t, t.TempDir(), filepath.Join(clean, ".beads"))})
	if err != nil {
		t.Fatalf("the control run failed: %v\n%s", err, ok)
	}
	if strings.Contains(ok, "ABORTED") {
		t.Errorf("a clean run reports an abort:\n%s", ok)
	}
}

// The other half of the fix: the constitution's redirect is written BEFORE
// anything that can fail for an ordinary reason, and the queue's own commit
// runs LAST. So the failure laurie actually hit — a refused commit — now
// leaves a fleet that reads and writes, with one commit outstanding. The
// trigger here is a pre-commit hook (a full disk and a gate refusal land in
// the same place); GIT_TEMPLATE_DIR gets it into the repo the script clones.
func TestQueueCutoverAbortAtTheCommitLeavesTheFleetResolving(t *testing.T) {
	tmpl := t.TempDir()
	hook := filepath.Join(tmpl, "hooks", "pre-commit")
	write(t, hook, "#!/bin/sh\necho 'pre-commit refuses' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}

	constitution, _ := qcConstitution(t)
	qcDrift(t, constitution)
	queue := filepath.Join(t.TempDir(), "queue")
	project := qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))

	out, err := qcRunEnv(t, []string{"GIT_TEMPLATE_DIR=" + tmpl},
		constitution, queue, t.TempDir(), []string{project})
	if err == nil {
		t.Fatalf("the fixture did not block the commit, so this pin measures nothing:\n%s", out)
	}
	if !strings.Contains(out, "pre-commit refuses") {
		t.Fatalf("something other than the hook failed:\n%s", out)
	}

	store := filepath.Join(queue, ".beads")
	// The claim, asserted as state and not as prose: bd resolves.
	got, readErr := os.ReadFile(filepath.Join(constitution, ".beads", beadsRedirect))
	if readErr != nil {
		t.Fatalf("the constitution has no redirect after the abort — bd is dead fleet-wide: %v\n%s", readErr, out)
	}
	if strings.TrimSpace(string(got)) != store {
		t.Errorf("the constitution redirects to %q, want %q", strings.TrimSpace(string(got)), store)
	}
	if _, statErr := os.Stat(filepath.Join(store, beadsJSONL)); statErr != nil {
		t.Errorf("the store is not whole in the queue repo: %v", statErr)
	}
	if r, readErr := os.ReadFile(filepath.Join(project, ".beads", beadsRedirect)); readErr != nil {
		t.Errorf("%s: %v", project, readErr)
	} else if strings.TrimSpace(string(r)) != store {
		t.Errorf("the fan-out did not run before the commit: %s still names %q",
			project, strings.TrimSpace(string(r)))
	}

	// …and it says so, with the one command that finishes it.
	for _, want := range []string{
		`stage "commit"`,
		"reads and\n  writes normally",
		"git commit -m 'beads: the store of record moves into its own repo (ADR 0015 §4)' -- .beads",
		"does NOT need a rollback",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the abort never says %q:\n%s", want, out)
		}
	}
}
