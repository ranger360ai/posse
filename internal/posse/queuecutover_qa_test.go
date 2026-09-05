package posse

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

// qcRollbackRecipe is the rollback half of the window step, printed by the
// script under test with the FIXTURE's paths already substituted.
//
// It used to be read out of docs/runbooks/queue-cutover.md. ADR 0024 D4
// moved that runbook into the instance tree (ranger-base-99ps), which left
// the pins below reading $CONSTITUTION's copy by absolute path and
// t.Skipping when it was not there — so six of them ran on one box and never
// on this project's own gate (measured on ranger-base-l1vej: 5 top-level
// skips and 2 tests reporting PASS on a control arm while their real arms
// skipped). ranger-base-sssr had already ruled that out: "A permanent silent
// skip is neither" of the two acceptable outcomes.
//
// So the block moved to where the pins are instead of the pins moving to
// where the block is: `queue-cutover.sh --print-rollback` emits it, the same
// way the script's abort trap already emits the four partial UNDOs that
// queuecutover_undo_qa_test.go runs. The runbook keeps the prose — when to
// roll back, what the window costs, which incident taught which line — and
// cites the command for the executable half. Nothing here reads a path
// outside this checkout any more, so there is nothing left to skip on.
func qcRollbackRecipe(t *testing.T, f qcFixture) string {
	t.Helper()
	cmd := exec.Command("sh", qcScript(t), "--print-rollback",
		"--constitution", f.constitution, "--queue", f.queue,
		"--worktrees", f.worktrees, "--only-redirect", f.posse,
		"--scan", filepath.Dir(f.constitution))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("queue-cutover.sh --print-rollback: %v\n%s", err, out)
	}
	// The fence that matters on a box where the live paths EXIST: a printer
	// that ignored a flag would emit the operator's own tree and these pins
	// would run a real rollback against it. qcRollbackRun's `~/` guard cannot
	// see that — the script substitutes absolute paths, not `~`.
	for _, want := range []string{f.constitution, f.queue, f.worktrees, f.posse} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("the printed rollback does not name the fixture's %s — it is about some other tree:\n%s", want, out)
		}
	}
	return string(out)
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

// qcRedirect is the one line of `dir/.beads/redirect`, or "" when there is
// none — the whole of what the fan-out writes.
func qcRedirect(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".beads", beadsRedirect))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("%s: %v", dir, err)
	}
	return strings.TrimSpace(string(b))
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
// whose pre-commit hook refuses. The base is qcEnv, never this box's raw
// environment: the script commits, and whose identity it commits under is not
// something a test may borrow from the machine it happens to run on.
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
	// Appended last on purpose: os/exec uses the LAST value for a duplicate
	// key, so a caller may replace anything qcEnv supplies — which is how the
	// identity's wrong arm is driven.
	cmd.Env = append(qcEnv(t, qcFixtureEmail), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// qcFixtureEmail is the identity every script run below commits under. It is
// the FIXTURE's, and that is the whole point — see qcEnv.
const qcFixtureEmail = "cutover-fixture@example.invalid"

// qcIdentityEnv renders a git configuration carrying email as the only
// identity on offer, and returns the environment entries that make it the
// whole of git's configuration — the box's own global and system files are
// cut out along with it. An empty email renders a config with no identity at
// all: that is the wrong arm, which
// TestQueueCutoverCommitsUnderTheFixturesOwnIdentity drives.
//
// `useConfigOnly` is the load-bearing line. Without it git falls back to
// guessing an identity from the hostname, which SUCCEEDS wherever the
// hostname has a domain part and fails where it does not —
//
//	fatal: unable to auto-detect email address (got "root@d9b2c1bf2bfd.(none)")
//
// The queue repo is a CLONE and a clone carries no local config, so the
// commit the script makes in it at stage "commit" had nothing else to go on:
// five of the pins in this file were green on macos-latest and red on
// ubuntu-latest on ci.yml's very first run (ranger-base-rstk). Supplying the
// identity is only half the fix. With the guessing switched OFF too, a
// fixture that ever loses its identity fails on EVERY box rather than on the
// domainless ones only, which is what makes these pins measure anything on a
// dev box.
func qcIdentityEnv(t *testing.T, email string) []string {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	body := "[user]\n\tuseConfigOnly = true\n"
	if email != "" {
		body += "\tname = cutover fixture\n\temail = " + email + "\n"
	}
	write(t, cfg, body)
	return []string{"GIT_CONFIG_GLOBAL=" + cfg, "GIT_CONFIG_SYSTEM=/dev/null"}
}

// qcEnv is this box's environment with every OTHER route to a git identity
// cut out of it, plus the fixture's own. GIT_AUTHOR_*/GIT_COMMITTER_* outrank
// the config and EMAIL backstops it, so an operator who exports one of them
// would be testing their identity rather than the fixture's — and the wrong
// arm below would pass while proving nothing.
func qcEnv(t *testing.T, email string) []string {
	t.Helper()
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if k == "EMAIL" || strings.HasPrefix(k, "GIT_AUTHOR_") || strings.HasPrefix(k, "GIT_COMMITTER_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, qcIdentityEnv(t, email)...)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// The trees nobody lists. A checkout retired before the move still resolves
// through the constitution's `.beads`, so after the move it holds hop one of
// a two-hop chain — and bd 0.49.1 refuses the second hop:
//
//	Warning: redirect chains not allowed, ignoring redirect in <middle>
//	Error: no beads database found
//	Hint: run 'bd init' to create a database in the current directory
//
// stderr and exit 1, with a hint inviting a second store in an archived
// tree. The other arm is worse: when that middle tree KEPT a database, the
// same warning goes to stderr and the command exits 0 against the SUPERSEDED
// store. Both measured on ranger-base-l9aa, where the archived pre-POSSE
// checkout was dead in exactly this way for a day because the fan-out took a
// list and a list can be short.
func TestQueueCutoverFindsTheTreesTheListForgets(t *testing.T) {
	t.Parallel()
	constitution, _ := qcConstitution(t)
	store := filepath.Join(constitution, ".beads")
	queue := filepath.Join(t.TempDir(), "queue")

	// Both of these live beside the constitution, which is what the scan
	// root is derived from. One points at the constitution and is on no
	// list; the other points at an unrelated store and must be left alone.
	root := filepath.Dir(constitution)
	forgotten := qcWork(t, filepath.Join(root, "retired-checkout"), store)
	elsewhere := filepath.Join(t.TempDir(), "someone-elses-store", ".beads")
	stranger := qcWork(t, filepath.Join(root, "unrelated-repo"), elsewhere)
	t.Cleanup(func() { os.RemoveAll(forgotten); os.RemoveAll(stranger) })

	project := qcWork(t, t.TempDir(), store)
	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
	if err != nil {
		t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
	}

	dst := filepath.Join(queue, ".beads")
	if got := qcRedirect(t, forgotten); got != dst {
		t.Errorf("a tree on nobody's list still redirects to %q, want %q — that is a two-hop chain\n%s", got, dst, out)
	}
	if got := qcRedirect(t, stranger); got != elsewhere {
		t.Errorf("a tree pointed at an unrelated store was rewritten to %q; it must keep %q", got, elsewhere)
	}
	// The scan root is DERIVED from --constitution rather than hard-coded to
	// $HOME/src. A hard-coded one would follow this fixture's override onto
	// the live box and repoint the working fleet at a t.TempDir queue.
	if !strings.Contains(out, "scan: "+root) {
		t.Errorf("the scan did not run under the constitution's parent %q:\n%s", root, out)
	}
}

// The wrong arm for the pin above: same fixture, scan switched off, and the
// forgotten tree stays stale. Without this, a fan-out that rewrote every
// `.beads` it could reach — including trees pointed at other stores — would
// pass the pin above just as well.
func TestQueueCutoverScanCanBeSwitchedOffAndThenTheChainSurvives(t *testing.T) {
	t.Parallel()
	constitution, _ := qcConstitution(t)
	store := filepath.Join(constitution, ".beads")
	queue := filepath.Join(t.TempDir(), "queue")

	root := filepath.Dir(constitution)
	forgotten := qcWork(t, filepath.Join(root, "retired-checkout"), store)
	t.Cleanup(func() { os.RemoveAll(forgotten) })

	project := qcWork(t, t.TempDir(), store)
	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project}, "--no-scan")
	if err != nil {
		t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
	}

	if got := qcRedirect(t, forgotten); got != store {
		t.Errorf("--no-scan rewrote a tree anyway: %q, want it left at %q\n%s", got, store, out)
	}
	if got, want := qcRedirect(t, project), filepath.Join(queue, ".beads"); got != want {
		t.Errorf("--no-scan also skipped a repo that WAS named: %q, want %q", got, want)
	}
	if strings.Contains(out, "scan: ") {
		t.Errorf("--no-scan still announced a scan:\n%s", out)
	}
}

// Every preflight refusal is a statement that the window is not open yet,
// and each one fails CLOSED — the script writes nothing before it refuses.
func TestQueueCutoverRefusesGroundItDoesNotExpect(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	constitution, _ := qcConstitution(t)
	write(t, filepath.Join(constitution, ".beads", "export-state", "abc.json"),
		`{"worktree_root":"/Users/someone/.posse/worktrees/posse/developer-x"}`+"\n")
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

// ranger-base-g1js, the other half: the rollback block moved the store
// home with `mv ~/src/ranger-queue/.beads/* ...`, and that glob does not
// match dotfiles. Rehearsed on lpz4: `.beads/.gitignore` stayed behind — the
// only thing ignoring the database — and the constitution repo came back
// with `beads.db` UNTRACKED AND UNIGNORED, one `git add -A` away from
// committing 10MB of binary database into the repo ADR 0015 exists to keep
// clean, with the tracked `.gitignore` showing as deleted beside it.
//
// These pins RUN the block rather than reading it. A prose
// assertion over a recipe goes green the moment the recipe is reworded, and
// this block had three independent ways to be wrong (the glob, a bare `rm`
// that errors when no redirect was ever written, and a `git checkout` that
// restores the root ignore only). Each has a control arm below driving the
// block as it read BEFORE the fix through the same rig: without one, a
// rollback that moved nothing would satisfy every assertion here.

// qcRollbackBlock is the first of the two blocks the printer emits — the
// thing the operator pastes.
func qcRollbackBlock(t *testing.T, f qcFixture) string { return qcRollbackPart(t, f, 0) }

// qcRollbackCheck is the second — the verification the operator reads to
// decide the rollback worked. It is a block the pins RUN for the same reason
// the one above is: it is the only instrument in the window, and its failure
// mode is silence (ranger-base-4lks, reached again by ranger-base-jg26e).
func qcRollbackCheck(t *testing.T, f qcFixture) string { return qcRollbackPart(t, f, 1) }

// qcRollbackMarker opens each of the printer's two blocks. A comment line, so
// the operator can paste the whole output and the pins can still split it.
const qcRollbackMarker = "# --- rollback"

// qcRollbackPart returns the nth block of the printed recipe.
func qcRollbackPart(t *testing.T, f qcFixture, n int) string {
	t.Helper()
	// The first chunk still carries its own marker (nothing precedes it) and
	// every later one carries the marker line's tail, so both cases are the
	// same edit: drop through the first newline.
	var parts []string
	for _, chunk := range strings.Split(qcRollbackRecipe(t, f), "\n"+qcRollbackMarker) {
		i := strings.Index(chunk, "\n")
		if i < 0 {
			t.Fatalf("a printed rollback block is a marker line and nothing else:\n%s", chunk)
		}
		parts = append(parts, chunk[i+1:])
	}
	if len(parts) <= n {
		t.Fatalf("the printed rollback has %d block(s), fewer than the pins read", len(parts))
	}
	return parts[n]
}

// qcRollbackBefore is that block as it read before this bead (posse 43f0ec5),
// verbatim. It is the control: the rig has to be able to SEE the defect, or
// the pin above it measures nothing.
const qcRollbackBefore = `cd ~/src/ranger-queue && bd daemon stop
mv ~/src/ranger-queue/.beads/* ~/src/ranger-base/.beads/     # the store goes home
rm  ~/src/ranger-base/.beads/redirect
cd ~/src/ranger-base && git reset -q HEAD -- .beads .gitignore && git checkout -- .gitignore
# put every redirect back
printf '%s\n' ~/src/ranger-base/.beads > ~/src/posse/.beads/redirect
for w in ~/.posse/worktrees/*/*; do
  [ -d "$w/.beads" ] && printf '%s\n' ~/src/ranger-base/.beads > "$w/.beads/redirect"
done
cd ~/src/ranger-base && bd migrate --update-repo-id && bd daemon start
`

// qcRollbackRun hands a rollback block to `sh` the way the operator's
// terminal does — no `set -e`, because a pasted block does not stop at the
// first error either, which is exactly why a `rm` that fails has to be caught
// by reading its output. `bd` is stubbed: the block's three bd calls are the
// operator's and denied to personas (runbook, "Who runs it"). Every mv, rm
// and git in it is real.
//
// The substitution below is now only for qcRollbackBefore, the frozen control:
// the LIVE block arrives from `--print-rollback` with the fixture's paths
// already in it. It is left in place as a fence rather than deleted — a block
// that still names a live path after it must not reach `sh`.
func qcRollbackRun(t *testing.T, block string, f qcFixture) string {
	t.Helper()
	for _, sub := range [][2]string{
		{"~/src/ranger-queue", f.queue},
		{"~/src/ranger-base", f.constitution},
		{"~/src/posse", f.posse},
		{"~/.posse/worktrees", f.worktrees},
		// LAST, and only what the four above did not claim: the block ends
		// by walking `~/src` for trees that redirect at the queue and are on
		// no list (ranger-base-l9aa). Pointed at the fixture's own root, that
		// walk is exercised; left unsubstituted it would run against the live
		// box — which is what the guard below refuses to let happen.
		{"~/src", filepath.Dir(f.constitution)},
	} {
		block = strings.ReplaceAll(block, sub[0], sub[1])
	}
	if strings.Contains(block, "~/") {
		t.Fatalf("the rollback block still names a live path after substitution:\n%s", block)
	}
	stub := t.TempDir()
	write(t, filepath.Join(stub, "bd"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(stub, "bd"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", block)
	cmd.Env = append(os.Environ(), "PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, _ := cmd.CombinedOutput()
	return string(out)
}

type qcFixture struct{ constitution, queue, posse, worktrees string }

// qcRolledBack is the state a rollback undoes: a COMPLETED cutover, with the
// live store's drift and the two dotfiles the glob misses (`.beads/.gitignore`
// is the tracked one that hides the database; `.local_version` is bd's).
func qcRolledBack(t *testing.T) qcFixture {
	t.Helper()
	constitution, _ := qcConstitution(t)
	// Two edits that make the fixture's `.beads` the live one's shape, and
	// both are load-bearing here. bd's real ignore file also covers
	// `.local_version` and `last-touched` — the other dotfile the glob
	// leaves behind — and the live constitution TRACKS `.beads/metadata.json`
	// (`git -C ~/src/ranger-base ls-files .beads`), so bd's own stamp of it
	// during the window does not read as an untracked surprise the rollback
	// never caused.
	write(t, filepath.Join(constitution, ".beads", ".gitignore"),
		"*.db\n*.db?*\ndaemon.log\ndaemon.pid\nredirect\n.local_version\nlast-touched\n")
	write(t, filepath.Join(constitution, ".beads", "metadata.json"), "{}\n")
	mustGit(t, constitution, "add", ".beads/.gitignore", ".beads/metadata.json")
	mustGit(t, constitution, "commit", "-q", "-m", "beads: bd's ignore file and its stamp", "--", ".beads")

	qcDrift(t, constitution)
	write(t, filepath.Join(constitution, ".beads", ".local_version"), "0.49.1\n")
	f := qcFixture{
		constitution: constitution,
		queue:        filepath.Join(t.TempDir(), "queue"),
		posse:        qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads")),
		worktrees:    t.TempDir(),
	}
	qcWork(t, filepath.Join(f.worktrees, "posse", "developer-2-session"), filepath.Join(constitution, ".beads"))
	out, err := qcRun(t, f.constitution, f.queue, f.worktrees, []string{f.posse})
	if err != nil {
		t.Fatalf("the cutover this rollback undoes did not run: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(f.queue, ".beads", ".gitignore")); err != nil {
		t.Fatalf("the fixture has no dotfile for the glob to miss, so it measures nothing: %v", err)
	}
	return f
}

// The bead's headline, asserted where it bites: not "does the block mention
// dotfiles" but "is the database ignored again when the block has run".
func TestQueueRollbackCarriesTheStoresDotfilesHome(t *testing.T) {
	t.Parallel()
	t.Run("the store comes home whole", func(t *testing.T) {
		f := qcRolledBack(t)
		out := qcRollbackRun(t, qcRollbackBlock(t, f), f)

		for _, name := range []string{".gitignore", ".local_version", beadsJSONL} {
			if _, err := os.Stat(filepath.Join(f.constitution, ".beads", name)); err != nil {
				t.Errorf(".beads/%s did not come home: %v\n%s", name, err, out)
			}
		}
		// The window's drift is real work: a rollback that restores `.beads`
		// out of HEAD would satisfy every ignore assertion below and throw
		// away every bead written during the window.
		if body := readFile(t, filepath.Join(f.constitution, ".beads", beadsJSONL)); !strings.Contains(body, "q-4") {
			t.Errorf("the rollback clobbered the window's drift instead of carrying it home: %q", body)
		}
		status := mustGit(t, f.constitution, "status", "--porcelain", "--", ".beads", ".gitignore")
		for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
			if line == "" {
				continue
			}
			switch {
			case strings.HasPrefix(line, "??"):
				t.Errorf("the constitution came back with an UNIGNORED file — one `git add -A` from being committed: %q\nstatus:\n%s\n%s", line, status, out)
			case strings.Contains(line, "D"):
				t.Errorf("the rollback left a tracked file deleted: %q\nstatus:\n%s\n%s", line, status, out)
			}
		}
		if !strings.Contains(status, beadsJSONL) {
			t.Errorf("the drift the window produced is not in the tree at all:\n%s\n%s", status, out)
		}
	})

	// The control. Same rig, same fixture, the pre-fix block: it must leave
	// exactly the wreckage the bead measured, or the arm above proves
	// nothing about the glob.
	t.Run("the control: the glob-only block leaves the dotfiles behind", func(t *testing.T) {
		f := qcRolledBack(t)
		out := qcRollbackRun(t, qcRollbackBefore, f)
		status := mustGit(t, f.constitution, "status", "--porcelain", "--", ".beads", ".gitignore")
		if !strings.Contains(status, "?? .beads/beads.db") {
			t.Errorf("the rig cannot see an unignored database, so the pin above measures nothing:\n%s\n%s", status, out)
		}
		if !strings.Contains(status, "D .beads/.gitignore") {
			t.Errorf("the rig cannot see the ignore file left behind:\n%s\n%s", status, out)
		}
	})
}

// The same block's second defect, and the one an operator meets first: the
// rollback is also what you run after an abort in stage `move`, where no
// redirect was ever written. A bare `rm` on a path that is not there prints
// an error into the middle of a window nobody is calm in.
func TestQueueRollbackRunsCleanWhenNoRedirectWasEverWritten(t *testing.T) {
	t.Parallel()
	const missing = "No such file"
	t.Run("the block says nothing about a redirect that was never written", func(t *testing.T) {
		f := qcRolledBack(t)
		if err := os.Remove(filepath.Join(f.constitution, ".beads", beadsRedirect)); err != nil {
			t.Fatalf("the fixture still has a redirect, so this measures nothing: %v", err)
		}
		out := qcRollbackRun(t, qcRollbackBlock(t, f), f)
		if strings.Contains(out, missing) {
			t.Errorf("the rollback errors on a redirect an aborted cutover never wrote:\n%s", out)
		}
	})
	t.Run("the control: the bare rm does", func(t *testing.T) {
		f := qcRolledBack(t)
		if err := os.Remove(filepath.Join(f.constitution, ".beads", beadsRedirect)); err != nil {
			t.Fatal(err)
		}
		if out := qcRollbackRun(t, qcRollbackBefore, f); !strings.Contains(out, missing) {
			t.Errorf("the rig does not surface the bare rm's error, so the pin above measures nothing:\n%s", out)
		}
	})
}

// The block's third defect: `git checkout -- .gitignore` restores the ROOT
// ignore and never `.beads/.gitignore`. With the dotfile-safe move in front
// of it that looks like a belt — until the rollback is run a second time,
// which is exactly what an operator who ran the OLD block and then took the
// runbook's last line (`rm -rf ~/src/ranger-queue`) has to do. By then the
// ignore file is on no disk anywhere and git is the only copy left.
func TestQueueRollbackRestoresTheIgnoreThatHidesTheDatabase(t *testing.T) {
	t.Parallel()
	f := qcRolledBack(t)
	qcRollbackRun(t, qcRollbackBefore, f) // the half-rollback that left it behind
	if err := os.RemoveAll(f.queue); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.constitution, ".beads", ".gitignore")); err == nil {
		t.Fatal("the half-rollback did not leave the ignore file behind, so this measures nothing")
	}

	out := qcRollbackRun(t, qcRollbackBlock(t, f), f)
	if _, err := os.Stat(filepath.Join(f.constitution, ".beads", ".gitignore")); err != nil {
		t.Errorf("re-running the rollback never restores .beads/.gitignore, so the database stays unignored: %v\n%s", err, out)
	}
	if status := mustGit(t, f.constitution, "status", "--porcelain", "--", ".beads"); strings.Contains(status, "??") {
		t.Errorf("the constitution is still carrying an unignored file:\n%s\n%s", status, out)
	}
	// And it did not pay for that by throwing the window's work away.
	if body := readFile(t, filepath.Join(f.constitution, ".beads", beadsJSONL)); !strings.Contains(body, "q-4") {
		t.Errorf("the second rollback clobbered the drift the first one carried home: %q", body)
	}
}

// readFile is os.ReadFile with the test's error handling.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// ─── ranger-base-nzyn: the window, and what an abort in it says ─────────────

// The commit the script makes must be one every session can make. It used to
// be `git commit -m <msg>` with no pathspec, which a persona cage denying
// `Bash(git commit unless --)` refuses — and it sat between the mv and the
// redirect, so the refusal killed bd fleet-wide (see the abort pin below).
// `git add -A .beads` stages nothing else, so the qualified form is the same
// commit: measured, same tree sha, deletions included.
// The rollback half of ranger-base-l9aa. The cutover's fan-out discovers
// trees that redirect at the constitution; the rollback has to bring those
// same trees home, or the ones nobody listed keep naming a queue repo the
// last line of the rollback deletes — a redirect into thin air, which reads
// exactly like the two-hop chain that started this: no database, and a hint
// offering to make a new one.
func TestQueueRollbackBringsHomeTheTreesTheListForgets(t *testing.T) {
	t.Parallel()
	f := qcRolledBack(t)
	store := filepath.Join(f.constitution, ".beads")

	// Beside the constitution, redirecting at the queue — the state the
	// cutover left it in — and named nowhere in the rollback block.
	forgotten := qcWork(t, filepath.Join(filepath.Dir(f.constitution), "retired-checkout"),
		filepath.Join(f.queue, ".beads"))
	t.Cleanup(func() { os.RemoveAll(forgotten) })

	out := qcRollbackRun(t, qcRollbackBlock(t, f), f)

	if got := qcRedirect(t, forgotten); got != store {
		t.Errorf("the rollback left a tree pointed at the deleted queue: %q, want %q\n%s", got, store, out)
	}
	// The control: the block as it read before this bead walked nothing, so
	// the rig can see the defect.
	f2 := qcRolledBack(t)
	forgotten2 := qcWork(t, filepath.Join(filepath.Dir(f2.constitution), "retired-checkout"),
		filepath.Join(f2.queue, ".beads"))
	t.Cleanup(func() { os.RemoveAll(forgotten2) })
	qcRollbackRun(t, qcRollbackBefore, f2)
	if got := qcRedirect(t, forgotten2); got == filepath.Join(f2.constitution, ".beads") {
		t.Error("the control block brought the tree home too — this pin measures nothing")
	}
}

func TestQueueCutoverCommitsDriftWithAPathQualifiedCommit(t *testing.T) {
	t.Parallel()
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

// The other half of ranger-base-iycc's fix, on the SUCCESS path. The queue
// repo's working tree is emptied after the replay's checkout and before the
// move, so what lands in $DST_BEADS is the live store and nothing else. That
// makes the live store the store of record here too: a file tracked at the
// last commit but no longer live used to survive the move — the checkout put
// it there, the move loop never touched it, and `git add -A .beads` saw no
// change — so the queue's first commit RESURRECTED a file the store had
// deleted. Measured both ways before and after the fix.
func TestQueueCutoverDoesNotResurrectAFileTheLiveStoreDeleted(t *testing.T) {
	t.Parallel()
	constitution, _ := qcConstitution(t)
	src := filepath.Join(constitution, ".beads")
	write(t, filepath.Join(src, "stale.txt"), "committed, then deleted live\n")
	mustGit(t, constitution, "add", ".beads/stale.txt")
	mustGit(t, constitution, "commit", "-q", "-m", "beads: a file that later leaves", "--", ".beads")
	if err := os.Remove(filepath.Join(src, "stale.txt")); err != nil {
		t.Fatal(err)
	}
	qcDrift(t, constitution)

	queue := filepath.Join(t.TempDir(), "queue")
	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{qcWork(t, t.TempDir(), src)})
	if err != nil {
		t.Fatalf("the cutover failed: %v\n%s", err, out)
	}
	// The witness that the replay carried it at all — without this the pin
	// passes over a history that never had the file.
	if !strings.Contains(mustGit(t, queue, "log", "--format=%H", "--name-only", "--", ".beads/stale.txt"), "stale.txt") {
		t.Fatalf("the replayed history never carried stale.txt, so nothing here is measured")
	}
	if tree := mustGit(t, queue, "ls-tree", "-r", "--name-only", "HEAD"); strings.Contains(tree, "stale.txt") {
		t.Errorf("the queue's commit resurrected a file the live store had deleted:\n%s", tree)
	}
	if _, e := os.Stat(filepath.Join(queue, ".beads", "stale.txt")); e == nil {
		t.Errorf("the replayed copy of stale.txt is still in the queue's working tree")
	}
}

// The bead's headline (ranger-base-nzyn, hit on lpz4): an abort between the
// mv and the redirect left the constitution's `.beads` EMPTY, the store in
// the queue repo, every redirect in the fleet naming the empty directory —
// and said nothing, because `set -eu` exits silently. Whatever aborts it, it
// must name the half-state and how to undo it.
func TestQueueCutoverAbortInTheMoveWindowNamesTheHalfStateAndItsUndo(t *testing.T) {
	t.Parallel()
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
// runs LAST. So the failure qa actually hit — a refused commit — now
// leaves a fleet that reads and writes, with one commit outstanding. The
// trigger here is a pre-commit hook (a full disk and a gate refusal land in
// the same place); GIT_TEMPLATE_DIR gets it into the repo the script clones.
func TestQueueCutoverAbortAtTheCommitLeavesTheFleetResolving(t *testing.T) {
	t.Parallel()
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

// ─── ranger-base-rstk: whose identity these fixtures commit under ───────────

// The commit the script makes lands in a repo `git clone` left with no local
// identity, so until qcEnv every pin in this file was at the mercy of the
// hostname of the box running it. Both arms, in one test, because either one
// alone is green for the wrong reason: the run must commit under the
// FIXTURE's identity, and with that identity taken away it must not be able
// to commit at all — no hostname on any box may rescue it.
func TestQueueCutoverCommitsUnderTheFixturesOwnIdentity(t *testing.T) {
	t.Parallel()
	// A store with drift is what makes the script reach its commit at all.
	drift := func(t *testing.T) (constitution, queue, project string) {
		t.Helper()
		constitution, _ = qcConstitution(t)
		qcDrift(t, constitution)
		queue = filepath.Join(t.TempDir(), "queue")
		project = qcWork(t, t.TempDir(), filepath.Join(constitution, ".beads"))
		return constitution, queue, project
	}

	t.Run("the drift commit carries the fixture's identity", func(t *testing.T) {
		constitution, queue, project := drift(t)
		out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
		if err != nil {
			t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
		}
		if got := strings.TrimSpace(mustGit(t, queue, "log", "-1", "--format=%ce")); got != qcFixtureEmail {
			t.Errorf("the drift commit was made by %q, want the fixture's %q — the run is taking its identity from the box", got, qcFixtureEmail)
		}
	})

	t.Run("the wrong arm: with no fixture identity nothing can be committed", func(t *testing.T) {
		constitution, queue, project := drift(t)
		out, err := qcRunEnv(t, qcIdentityEnv(t, ""), constitution, queue, t.TempDir(), []string{project})
		if err == nil {
			t.Fatalf("the script committed with no identity configured, so git guessed one here and the arm above proves nothing on this box:\n%s", out)
		}
		if !strings.Contains(out, "auto-detection is disabled") {
			t.Errorf("the run failed for something other than the missing identity:\n%s", out)
		}
		if !strings.Contains(out, `stage "commit"`) {
			t.Errorf("the identity is missed somewhere other than the commit:\n%s", out)
		}
	})
}

// ─── ranger-base-4lks: two live defects in the same Rollback section ─────────
//
// Fixed by teaching the block which side of step 8 it is on (revert the
// commit when it already ran, before the store goes home) and by having it
// extend `.beads/.gitignore` with the two runtime files bd's own shipped
// ignore misses. Both arms now assert the repair, driven through the
// control fixtures that first proved the hole.

// The Rollback section's own verification step — the one ranger-base-g1js's
// close added — says a `??` line means the store did not come home whole.
// Two files bd leaves in `.beads` were matched by neither `.beads/.gitignore`
// (it lists `bd.sock` exactly, and no `daemon-error`) nor the constitution's
// root ignore, so a rollback that succeeded completely still printed them
// and told the operator to stop. `bd.sock.startlock` is in the live store
// now; `daemon-error` is named in ranger-base-g1js's own repro. The block
// now appends both patterns to `.beads/.gitignore` after the store comes
// home, so a complete rollback reports neither as untracked.
//
// ranger-base-0dylx added the third: bd 0.50.3 drops `.migration-hint-ts`
// (a unix timestamp that rate-limits its own migration notice) into any
// store it touches, and the whole-store move carries it home. It is
// per-machine generated state, the class bd already ignores as
// `.local_version`, so it joins the list rather than being excluded from
// the move — a growing exclusion list is how `.beads/.gitignore` itself got
// left behind (ranger-base-g1js). Written by the binary, so it needs no
// `write` below: the arm asserts it is covered wherever it came from.
//
// ranger-base-puqvd added the fourth, `.jsonl.lock`: bd takes it around the
// JSONL export and leaves it behind, empty. Same class again — 0.50.3's own
// shipped ignore template lists it under "Sync state (local-only,
// per-machine)" beside `.sync.lock`; the live store's copy is just an older
// one, written at init, that predates the entry. Unlike `.migration-hint-ts`
// it IS written below, because a dotfile in the queue store also exercises
// the move loop's `.[!.]*` arm on the way home.
func TestQueueRollbackVerificationFiresOnARollbackThatWorked(t *testing.T) {
	t.Parallel()
	f := qcRolledBack(t)
	for _, name := range []string{"bd.sock.startlock", "daemon-error", ".jsonl.lock", ".migration-hint-ts"} {
		write(t, filepath.Join(f.queue, ".beads", name), "runtime\n")
	}
	// `export-state/` is the one of the four patterns ranger-base-6dygg
	// measured that the store's own ignore does NOT carry and the runbook
	// deliberately does not append: the constitution's ROOT `.gitignore`
	// covers it, from a commit that pre-dates the one Rollback undoes. The
	// live store holds 638 of these, so if that line ever leaves the root
	// ignore the good path reports a rollback that worked as a broken one.
	const exportState = "9ae9377c9f59b1fc.json"
	write(t, filepath.Join(f.queue, ".beads", "export-state", exportState), "{}\n")
	out := qcRollbackRun(t, qcRollbackBlock(t, f), f)

	// The rollback itself must have worked, or this arm is measuring a
	// broken rig rather than a false alarm.
	if _, err := os.Stat(filepath.Join(f.constitution, ".beads", ".gitignore")); err != nil {
		t.Fatalf("the rig's rollback did not carry the ignore home, so nothing below is about the check: %v\n%s", err, out)
	}
	// The export-state file has to have COME HOME, or the absence of a `??`
	// line for it below is the move failing rather than an ignore working.
	if _, err := os.Stat(filepath.Join(f.constitution, ".beads", "export-state", exportState)); err != nil {
		t.Fatalf("the rollback did not carry .beads/export-state home, so the arm below measures nothing: %v\n%s", err, out)
	}
	status := mustGit(t, f.constitution, "status", "--porcelain", "--", ".beads", ".gitignore")
	for _, name := range []string{"bd.sock.startlock", "daemon-error", ".jsonl.lock", ".migration-hint-ts"} {
		if strings.Contains(status, "?? .beads/"+name) {
			t.Errorf("a complete rollback still reports .beads/%s as untracked — the runbook's check "+
				"still cries wolf on the good path.\nstatus:\n%s\n%s", name, status, out)
		}
	}
	if strings.Contains(status, "?? .beads/export-state") {
		t.Errorf("a complete rollback reports .beads/export-state as untracked — the constitution's root "+
			".gitignore no longer covers it, and the runbook does not append it either.\nstatus:\n%s\n%s", status, out)
	}
	ignore := readFile(t, filepath.Join(f.constitution, ".beads", ".gitignore"))
	for _, pat := range []string{"bd.sock.startlock", "daemon-error", ".migration-hint-ts", ".jsonl.lock"} {
		if !strings.Contains(ignore, pat) {
			t.Errorf("the restored .beads/.gitignore does not cover %q:\n%s", pat, ignore)
		}
	}
}

// Rollback used to open with "the constitution repo's .beads deletion is
// staged, not committed" as if that always held — but step 8 of the same
// runbook commits it, and the live window did. Once the untracking is
// committed, HEAD holds no `.beads/.gitignore`, so the block reverts step
// 8's commit instead of trying to `checkout` a path HEAD no longer has,
// which restores both ignores and tracking in one move — before the store
// itself goes home, so the incoming files land as modifications rather than
// colliding with what the revert recreates.
//
// That commit is NOT reliably HEAD, and this pin used to say it was: the
// fixture committed step 8 LAST, which made the runbook's "this runbook
// makes its commit the last one touching the constitution" true instead of
// testing it. On the live constitution at 2026-09-01 the window's commit
// (f46c766, 08-28) was 157 commits back, none of them touching `.beads` or
// `.gitignore` — so `git revert --no-edit HEAD` would have undone an
// unrelated commit and left `.beads/` in the root ignore, after which the
// runbook's own check prints nothing and reads as clean (ranger-base-jg26e).
// The fixture now puts unrelated commits on top, and the control arm drives
// the pre-fix line through the same rig to prove the wreckage is visible.
func TestQueueRollbackIsWrittenForAStateStepEightRemoves(t *testing.T) {
	t.Parallel()
	t.Run("step 8 is the last commit", func(t *testing.T) {
		f := qcSteppedEight(t, 0)
		qcAssertRolledBackToTracking(t, f, qcRollbackRun(t, qcRollbackBlock(t, f), f))
	})

	// The bead: the same state, reached the way the live window reached it.
	t.Run("unrelated commits landed on top of step 8", func(t *testing.T) {
		f := qcSteppedEight(t, 2)
		out := qcRollbackRun(t, qcRollbackBlock(t, f), f)
		qcAssertRolledBackToTracking(t, f, out)
		if body := readFile(t, filepath.Join(f.constitution, qcLaterFile)); !strings.Contains(body, qcLaterLast) {
			t.Errorf("the rollback undid an unrelated commit instead of step 8's: %s is %q, not %q\n%s",
				qcLaterFile, body, qcLaterLast, out)
		}
		// And the operator's instrument says so out loud.
		if check := qcRollbackRun(t, qcRollbackCheck(t, f), f); !strings.Contains(check, qcCheckRan) {
			t.Errorf("the verification step does not certify a rollback that worked:\n%s\n%s", check, out)
		}
	})

	// The commit the block finds is the one it reverts, so it has to refuse
	// a commit that carries more than the store. An operator who typed
	// `git commit -a` at step 8 — or squashed it — has one, and undoing it
	// would take unrelated work with it. Refusing is the cheap half; the
	// half that matters is that a refusal cannot then read as clean.
	t.Run("step 8's commit carries unrelated files", func(t *testing.T) {
		f := qcRolledBack(t)
		write(t, filepath.Join(f.constitution, qcLaterFile), "window work 0\n")
		mustGit(t, f.constitution, "add", "--", qcLaterFile)
		mustGit(t, f.constitution, "commit", "-q", "-m",
			"beads: the queue moves to its own repo, and a stray file (ADR 0015 §4)",
			"--", ".beads", ".gitignore", qcLaterFile)

		out := qcRollbackRun(t, qcRollbackBlock(t, f), f)
		if !strings.Contains(out, "ROLLBACK STOPPED") {
			t.Errorf("the rollback reverted a commit that also holds %s instead of stopping:\n%s", qcLaterFile, out)
		}
		if _, err := os.Stat(filepath.Join(f.constitution, qcLaterFile)); err != nil {
			t.Errorf("%s went with the revert after all: %v\n%s", qcLaterFile, err, out)
		}
		if check := qcRollbackRun(t, qcRollbackCheck(t, f), f); strings.Contains(check, qcCheckRan) {
			t.Errorf("a rollback that stopped still certifies itself:\n%s\n%s", check, out)
		}
	})

	// The control. Same rig, same fixture, the committed arm as it read
	// before this bead: it has to undo the wrong commit AND leave a check
	// that reads clean, or the two arms above measure nothing.
	t.Run("the control: reverting HEAD undoes the wrong commit and still reads clean", func(t *testing.T) {
		f := qcSteppedEight(t, 2)
		out := qcRollbackRun(t, qcRollbackHeadRevert(t, f), f)
		if body := readFile(t, filepath.Join(f.constitution, qcLaterFile)); strings.Contains(body, qcLaterLast) {
			t.Errorf("the rig cannot see the wrong commit being reverted, so the arms above prove nothing: %s is %q\n%s",
				qcLaterFile, body, out)
		}
		if status := mustGit(t, f.constitution, "status", "--porcelain", "--", ".beads", ".gitignore"); strings.TrimSpace(status) != "" {
			t.Errorf("the rig cannot see a store that came home untracked and ignored:\nstatus:\n%s\n%s", status, out)
		}
		if check := qcRollbackRun(t, qcRollbackCheck(t, f), f); strings.Contains(check, qcCheckRan) {
			t.Errorf("the verification step certifies a rollback that never restored tracking — "+
				"an empty status still reads as a checked one:\n%s\n%s", check, out)
		}
	})
}

// qcCheckRan is the sentinel the runbook's verification block prints when it
// reaches the end. Its absence is the signal the runbook tells the operator
// to read, so the pins read it too.
const qcCheckRan = "rollback check:"

// qcLaterFile is a file in the constitution that has nothing to do with the
// store, and qcLaterLast is what the LAST unrelated commit leaves in it —
// the one `git revert --no-edit HEAD` would undo.
const (
	qcLaterFile = "NOTES.md"
	qcLaterLast = "window work 1"
)

// qcSteppedEight is qcRolledBack with step 8 itself run — the untracking
// committed — and then `later` unrelated commits on top, touching neither
// `.beads` nor `.gitignore`. That last number is the whole point: at 0 the
// fixture makes the runbook's "step 8's commit is HEAD" assumption true
// rather than testing it (ranger-base-jg26e).
func qcSteppedEight(t *testing.T, later int) qcFixture {
	t.Helper()
	f := qcRolledBack(t)
	// Step 8, verbatim in effect: commit the staged untracking.
	mustGit(t, f.constitution, "commit", "-q", "-m",
		"beads: the queue moves to its own repo (ADR 0015 §4)", "--", ".beads", ".gitignore")
	if tree := mustGit(t, f.constitution, "ls-tree", "HEAD", "--name-only", ".beads/"); strings.TrimSpace(tree) != "" {
		t.Fatalf("step 8 did not untrack .beads in the fixture, so this arm is not about the live state:\n%s", tree)
	}
	step8 := strings.TrimSpace(mustGit(t, f.constitution, "rev-parse", "HEAD"))
	for i := 0; i < later; i++ {
		write(t, filepath.Join(f.constitution, qcLaterFile), "window work "+strconv.Itoa(i)+"\n")
		mustGit(t, f.constitution, "add", "--", qcLaterFile)
		mustGit(t, f.constitution, "commit", "-q", "-m", "unrelated work "+strconv.Itoa(i), "--", qcLaterFile)
	}
	if later > 0 {
		if head := strings.TrimSpace(mustGit(t, f.constitution, "rev-parse", "HEAD")); head == step8 {
			t.Fatalf("the unrelated commits did not land, so step 8 is still HEAD and the fixture is the old one")
		}
		if n := strings.TrimSpace(mustGit(t, f.constitution, "rev-list", "--count", step8+"..HEAD", "--", ".beads", ".gitignore")); n != "0" {
			t.Fatalf("the unrelated commits touch the store or the root ignore (%s of them), "+
				"which makes the assumption visibly false — the live 157 were not (ranger-base-jg26e)", n)
		}
	}
	return f
}

// qcAssertRolledBackToTracking is the state a post-step-8 rollback has to
// leave whichever commit step 8 turned out to be: both ignores back, the
// store tracked again, and a check with something to say.
func qcAssertRolledBackToTracking(t *testing.T, f qcFixture, out string) {
	t.Helper()
	if strings.Contains(out, "did not match any file(s) known to git") {
		t.Errorf("the rollback's checkout still fails after step 8 instead of reverting its commit:\n%s", out)
	}
	if strings.Contains(out, "ROLLBACK STOPPED") {
		t.Errorf("the rollback refused to identify step 8's commit:\n%s", out)
	}
	status := mustGit(t, f.constitution, "status", "--porcelain", "--", ".beads", ".gitignore")
	if strings.TrimSpace(status) == "" {
		t.Errorf("the verification step has nothing to say after a post-step-8 rollback — "+
			"that reads as clean whether or not it is.\n%s", out)
	}
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "??") {
			t.Errorf("a post-step-8 rollback left an untracked file: %q\nstatus:\n%s\n%s", line, status, out)
		}
	}
	tracked := mustGit(t, f.constitution, "ls-files", "--", ".beads")
	if !strings.Contains(tracked, beadsJSONL) {
		t.Errorf("the constitution does not track its store again after a post-step-8 rollback.\ntracked:\n%s\n%s", tracked, out)
	}
	if !strings.Contains(tracked, ".gitignore") {
		t.Errorf("the constitution does not track .beads/.gitignore again after a post-step-8 rollback.\ntracked:\n%s\n%s", tracked, out)
	}
}

// qcRollbackHeadRevert is the runbook's own block with its committed arm put
// back the way it read before this bead — `git revert --no-edit HEAD`, and
// nothing else changed. A mutation of the one line under test is a sharper
// control than a frozen copy of the whole block: everything around it stays
// current, so the arms above cannot pass because the rig drifted.
func qcRollbackHeadRevert(t *testing.T, f qcFixture) string {
	t.Helper()
	block := qcRollbackBlock(t, f)
	i := strings.Index(block, "\nelse\n")
	if i < 0 {
		t.Fatal("the rollback block no longer branches on whether step 8 has run — the control cannot be built")
	}
	j := strings.Index(block[i:], "\nfi\n")
	if j < 0 {
		t.Fatal("the rollback block's staged/committed branch is unterminated — the control cannot be built")
	}
	return block[:i] + "\nelse\n  git revert --no-edit HEAD" + block[i+j:]
}
