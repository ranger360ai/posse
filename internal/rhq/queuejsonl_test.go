package rhq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qRepo is a queue repo the way scripts/queue-cutover.sh leaves one: a git
// checkout holding `.beads/` and nothing else, with the projection already
// under version control and no remote.
func qRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, ".beads", beadsJSONL), `{"id":"q-1","title":"seed"}`+"\n")
	mustGit(t, repo, "add", filepath.Join(".beads", beadsJSONL))
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	return repo
}

// qWork is a repo the fleet actually works in: its `.beads` holds a redirect
// and nothing else, which is what every project checkout and session
// worktree looks like after the cutover.
func qWork(t *testing.T, store string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".beads", beadsRedirect), store+"\n")
	return dir
}

// qApp is an App whose config names the queue repo, or names nothing when
// queue is "". It also puts the fake bd on the path the code under test
// resolves: the export is bd's and the commit is ours, and a suite that
// shelled out to the real binary would be testing the operator's database.
func qApp(t *testing.T, queue string) *App {
	t.Helper()
	t.Setenv("RHQ_FAKE_HERDR", "1")
	t.Setenv("RHQ_FAKE_DIR", t.TempDir())
	t.Setenv("RHQ_BD_BIN", os.Args[0])
	a := wtApp(t)
	if queue != "" {
		write(t, a.ConfigPath, "queue_repo: "+queue+"\n")
	} else {
		write(t, a.ConfigPath, "coordinator: coordinator\n")
	}
	return a
}

// closeBead is what a close does to the projection: bd exports the new
// state, and the file on disk differs from the one in the last commit.
func closeBead(t *testing.T, store, line string) {
	t.Helper()
	write(t, filepath.Join(store, beadsJSONL), line+"\n")
}

// The shipped default is the pre-cutover world, and it must stay a no-op
// there: a posse that knows how to commit the jsonl is installed BEFORE the
// window (`make install` is the operator's, and it does not wait for a
// runbook), and until the key is written the store is still inside the
// constitution repo — the one repo ADR 0015 exists to stop writing to.
func TestQueueCommitIsInertUntilTheStoreHasMoved(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	work := qWork(t, store)
	a := qApp(t, "")
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	before := mustGit(t, repo, "rev-parse", "HEAD")
	c, err := a.CommitQueueJSONL(NewBd(), work, "beads: q-1 closed by developer")
	if err != nil {
		t.Fatalf("an unconfigured instance must not error: %v", err)
	}
	if c.SHA != "" {
		t.Errorf("it committed with no queue_repo: configured: %+v", c)
	}
	if !strings.Contains(c.Skipped, "queue_repo") {
		t.Errorf("the skip must name the key that is missing, got %q", c.Skipped)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Errorf("HEAD moved in a repo nothing was configured for: %s -> %s", before, after)
	}
}

// The close path's whole point: the projection reaches a commit, in the
// queue repo, reachable by the bead's own id — `git log --grep <id>` is how
// a later reader connects a bead to the state it left behind.
func TestQueueCommitLandsTheProjectionThroughARedirect(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	work := qWork(t, store)
	a := qApp(t, repo)
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), work, "beads: q-1 closed by developer")
	if err != nil {
		t.Fatalf("CommitQueueJSONL: %v", err)
	}
	if c.SHA == "" {
		t.Fatalf("nothing was committed: %+v", c)
	}
	if c.Store != store {
		t.Errorf("the redirect was not followed: Store = %q, want %q", c.Store, store)
	}
	if msg := mustGit(t, repo, "log", "-1", "--format=%s"); !strings.Contains(msg, "q-1") {
		t.Errorf("the commit message must name the bead: %q", msg)
	}
	files := mustGit(t, repo, "show", "--name-only", "--format=", "HEAD")
	if strings.TrimSpace(files) != ".beads/"+beadsJSONL {
		t.Errorf("the commit must hold the projection and nothing else:\n%s", files)
	}
	// And the census — the reason any of this is committed at all — can now
	// see the state this close left.
	if body := mustGit(t, repo, "show", "HEAD:.beads/"+beadsJSONL); !strings.Contains(body, "closed") {
		t.Errorf("the committed projection is not the post-close one: %q", body)
	}
}

// The queue repo's index is as shared as any other, and bd drops untracked
// files of its own into `.beads` (`daemon-error`, `daemon.log` — measured on
// the ranger-base-tjfw rehearsal). A commit that swept those would put bd's
// runtime noise, and anything a concurrent hand had staged, into the store
// of record's history.
func TestQueueCommitNamesOnlyItsOwnPaths(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	work := qWork(t, store)
	a := qApp(t, repo)
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)
	write(t, filepath.Join(store, "daemon-error"), "DATABASE MISMATCH DETECTED!\n")
	write(t, filepath.Join(repo, "somebody-elses.txt"), "staged by another hand\n")
	mustGit(t, repo, "add", "somebody-elses.txt")

	if _, err := a.CommitQueueJSONL(NewBd(), work, "beads: q-1 closed by developer"); err != nil {
		t.Fatalf("CommitQueueJSONL: %v", err)
	}
	files := mustGit(t, repo, "show", "--name-only", "--format=", "HEAD")
	for _, unwanted := range []string{"daemon-error", "somebody-elses.txt"} {
		if strings.Contains(files, unwanted) {
			t.Errorf("the commit swept %s:\n%s", unwanted, files)
		}
	}
	if staged := mustGit(t, repo, "diff", "--cached", "--name-only"); staged != "somebody-elses.txt" {
		t.Errorf("the other hand's staging did not survive: %q", staged)
	}
}

// A close that produces no change in the projection is not an error, but it
// IS the shape of a close whose record did not reach git — so it has to say
// which of the two happened rather than reporting success.
func TestQueueCommitSaysSoWhenTheProjectionDidNotMove(t *testing.T) {
	repo := qRepo(t)
	a := qApp(t, repo)

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, filepath.Join(repo, ".beads")), "beads: q-1 closed by developer")
	if err != nil {
		t.Fatalf("CommitQueueJSONL: %v", err)
	}
	if c.SHA != "" {
		t.Errorf("an unchanged projection must not produce an empty commit: %+v", c)
	}
	if c.Skipped == "" {
		t.Error("it reported neither a commit nor a reason")
	}
}

// The key names ONE repo, and a store that is not inside it is not this
// launcher's to commit in. Belt for the config typo that would otherwise
// point the launcher back at the constitution repo — the exact write ADR
// 0015 §4 moves the queue to prevent.
func TestQueueCommitRefusesAStoreOutsideTheQueueRepo(t *testing.T) {
	elsewhere := qRepo(t) // stands in for the constitution repo
	a := qApp(t, t.TempDir())
	before := mustGit(t, elsewhere, "rev-parse", "HEAD")
	closeBead(t, filepath.Join(elsewhere, ".beads"), `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, filepath.Join(elsewhere, ".beads")), "beads: q-1 closed")
	if err != nil {
		t.Fatalf("CommitQueueJSONL: %v", err)
	}
	if c.SHA != "" {
		t.Errorf("it committed in a repo queue_repo: does not name: %+v", c)
	}
	if after := mustGit(t, elsewhere, "rev-parse", "HEAD"); after != before {
		t.Errorf("HEAD moved in the wrong repo: %s -> %s", before, after)
	}
}

// ADR 0015 §4, verbatim: "Never an automatic push." Asserted behaviorally
// rather than by reading the source — the queue repo is given a remote it
// COULD push to, and the remote must not move. The cutover script leaves
// the real one with no remote at all, so this is the second wall, not the
// first.
func TestQueueCommitNeverPushes(t *testing.T) {
	bare := t.TempDir()
	mustGit(t, bare, "init", "-q", "--bare", ".")
	repo := qRepo(t)
	mustGit(t, repo, "remote", "add", "origin", bare)
	// …and an upstream for `main`, set with config rather than by pushing.
	// Without it this pin had no teeth against the likeliest regression:
	// measured on ranger-base-lpz4, a mutant that ran a bare `git push`
	// after the commit left this test GREEN, because `push.default=simple`
	// with no upstream fails at the client before it reaches the remote.
	// The remote it must not move has to be one a bare push would reach.
	mustGit(t, repo, "config", "branch.main.remote", "origin")
	mustGit(t, repo, "config", "branch.main.merge", "refs/heads/main")
	store := filepath.Join(repo, ".beads")
	a := qApp(t, repo)
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	if err != nil || c.SHA == "" {
		t.Fatalf("CommitQueueJSONL = (%+v, %v)", c, err)
	}
	if refs := mustGit(t, bare, "for-each-ref"); strings.TrimSpace(refs) != "" {
		t.Errorf("the commit reached the remote:\n%s", refs)
	}
}

// The export before the commit is not optional: the database is the store of
// record and the file is a projection of it, so committing without asking bd
// to flush first commits the state from before the close.
func TestQueueCommitFlushesBeforeItCommits(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	a := qApp(t, repo)
	fake := os.Getenv("RHQ_FAKE_DIR")
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	if _, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer"); err != nil {
		t.Fatalf("CommitQueueJSONL: %v", err)
	}
	log, err := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if err != nil {
		t.Fatalf("the fake bd was never called: %v", err)
	}
	if !strings.Contains(string(log), "sync --flush-only") {
		t.Errorf("the launcher must ask for the git-free export, got:\n%s", log)
	}
	// …and never the form that would push. `bd sync --full` pulls, commits
	// and PUSHES (measured on 0.49.1), which is why the export is spelled
	// out rather than left to `bd sync`'s default.
	if strings.Contains(string(log), "--full") {
		t.Errorf("the launcher asked for the pushing form of sync:\n%s", log)
	}
}

// The wiring, end to end: a pass that judges a close must leave the
// projection in a commit. Asserted through Run rather than by calling the
// commit directly, because the thing that breaks is the CALL SITE — the
// close path is one of several places a bead can reach "closed", and the
// ADR's verification item 6 is about what a pass actually did, not about
// what a function does when someone remembers to call it.
func TestDispatchPassCommitsTheQueueJSONLOnAClose(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	queue := qRepo(t)
	store := filepath.Join(queue, ".beads")
	repo := t.TempDir()
	write(t, filepath.Join(repo, ".beads", beadsRedirect), store+"\n")
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\nqueue_repo: "+queue+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "pane-run-starts-agent"), nil, 0o644)
	// The close is what moves the projection; the fake bd does not export,
	// so the change bd would have made is written here.
	closeBead(t, store, `{"id":"a-1","title":"t","status":"closed"}`)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "closed by ranger") {
		t.Fatalf("the pass did not judge the close:\n%s", out)
	}
	if !strings.Contains(out, "committed in") {
		t.Errorf("the pass said nothing about the queue commit:\n%s", out)
	}
	if msg := mustGit(t, queue, "log", "-1", "--format=%s"); !strings.Contains(msg, "a-1") {
		t.Errorf("the close left no commit naming the bead: %q\n%s", msg, out)
	}
}

// …and an instance that has not cut over stays silent about it. A line per
// close saying "no queue repo configured" would be noise on every pass of
// every instance that never moves its store.
func TestDispatchPassSaysNothingAboutTheQueueBeforeTheCutover(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "pane-run-starts-agent"), nil, 0o644)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "closed by ranger") {
		t.Fatalf("the pass did not judge the close:\n%s", out)
	}
	if strings.Contains(out, "queue") {
		t.Errorf("an instance with no queue_repo: must not mention one:\n%s", out)
	}
}

// ADR 0015 §4's actual promise, and the one that is checkable by reading a
// set rather than auditing a deny-list: once the store lives in its own
// repo, a session working anywhere else holds no write grant into the
// CONSTITUTION repo at all. The redirect grant is what used to carry one —
// it grants the store's directory and that repo's git dirs to every caged
// session (ranger-base-rhw) — so the property is not that the grant went
// away, it is that the grant now lands somewhere else.
func TestSeatbeltGrantFollowsTheStoreOutOfTheConstitutionRepo(t *testing.T) {
	t.Setenv("TMPDIR", "") // the blanket temp grant would cover the fixtures

	constitution := qRepo(t) // stands in for ~/src/ranger-base after the move
	queue := qRepo(t)
	store := filepath.Join(queue, ".beads")
	work := qWork(t, store) // a project checkout, redirecting to the queue

	ag := &AgentFile{Name: "developer"}
	got := NewAppAt(t.TempDir()).SeatbeltWritable(ag, work, t.TempDir())

	var sawStore bool
	for _, w := range got {
		if underDir(store, w) {
			sawStore = true
		}
		if underDir(constitution, w) {
			t.Errorf("a session in %s may write the constitution repo: %s", work, w)
		}
	}
	if !sawStore {
		t.Errorf("bd cannot reach its own store: the writable set is\n  %s", strings.Join(got, "\n  "))
	}
	// And the store's git dirs with it, or the launcher's jsonl commit and
	// bd's own writes fail on .git/index.lock rather than on the file.
	var sawGit bool
	for _, w := range got {
		if underDir(filepath.Join(queue, ".git"), w) {
			sawGit = true
		}
	}
	if !sawGit {
		t.Errorf("the queue repo's git dir is not writable, so nothing can commit the projection:\n  %s", strings.Join(got, "\n  "))
	}
}

// ranger-base-mp0v — ranger-base-3c3's defect, one repo over. The
// prepare-commit-msg slot in the QUEUE repo carries the beads visibility
// stamp, and after the cutover it is the only wall between a launcher
// commit and the store of record's history. Until this, one runbook step
// installed it (scripts/queue-cutover.sh, step 6) and nothing ever looked
// again: the launch probe reads the SESSION dir, and no session starts in
// the queue repo. So the commit path reconciles the slot itself, and a
// close cannot commit unstamped merely because the step was skipped.
func TestQueueCommitInstallsTheStampItCommitsThrough(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	a := qApp(t, repo)
	write(t, a.ConfigPath, "queue_repo: "+repo+"\nbeads_visibility:\n  "+repo+": private\n")
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	// The arm proves nothing if the fixture is already armed: `git init`
	// leaves prepare-commit-msg.sample, never the slot itself.
	slot := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	if _, err := os.Stat(slot); err == nil {
		t.Fatalf("the fixture already carries %s — a repo the cutover has not stamped is the case under test", slot)
	}

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	if err != nil || c.SHA == "" {
		t.Fatalf("CommitQueueJSONL = (%+v, %v)", c, err)
	}
	body, err := os.ReadFile(slot)
	if err != nil {
		t.Fatalf("the projection committed with nothing in the slot: %v", err)
	}
	if string(body) != CommitGuardHook(VisibilityPrivate) {
		t.Errorf("the slot does not carry the render config's visibility calls for:\n%s", body)
	}
	if fi, _ := os.Stat(slot); fi != nil && fi.Mode()&0o111 == 0 {
		t.Errorf("the hook is not executable: %v", fi.Mode())
	}
}

// The other half, and the one that fails toward disclosure: a slot that is
// there but is not ours. Nothing can tell a foreign hook that refuses
// everything from one that refuses only the probe, so posse does not
// guess — it will not commit the store of record through a wall it cannot
// vouch for, and says so on the pass rather than committing unguarded.
func TestQueueCommitRefusesThroughANeuteredStamp(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	a := qApp(t, repo)
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	// What "neutered" looks like from git's side: present, executable,
	// plausible, and exit 0 on everything.
	const neutered = "#!/bin/sh\nexit 0\n"
	slot := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	write(t, slot, neutered)
	if err := os.Chmod(slot, 0o755); err != nil {
		t.Fatal(err)
	}

	before := mustGit(t, repo, "rev-parse", "HEAD")
	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	if err == nil {
		t.Fatalf("the projection committed through a neutered stamp: %+v", c)
	}
	if c.SHA != "" {
		t.Errorf("a refusal reported a commit: %+v", c)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Errorf("HEAD moved in the queue repo: %s -> %s", before, after)
	}
	// The refusal has to be actionable: which slot, and what to run.
	for _, want := range []string{"prepare-commit-msg", "install-hooks", AbbrevHome(slot)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must carry %q, got: %v", want, err)
		}
	}
	// …and the foreign file is untouched. That is why it stays neutered and
	// why the refusal is the only outcome left: install will not overwrite a
	// hook it did not write (ADR 0002 §3).
	if b, _ := os.ReadFile(slot); string(b) != neutered {
		t.Errorf("the reconcile overwrote a foreign hook:\n%s", b)
	}
}

// The stamp is config-driven and install-time — a repo re-marked in
// `beads_visibility:` after the one install keeps the old stamp until
// somebody reinstalls. The queue repo is where that drifts unseen, because
// nobody launches a session there to restamp it. Reconciling on the commit
// path is what closes it: the next close carries the mark config actually
// says. Ordered the way the runbook warns about — the two config edits land
// together, so the first close after them is where the drift shows.
func TestQueueCommitRestampsASlotConfigHasReMarked(t *testing.T) {
	repo := qRepo(t)
	store := filepath.Join(repo, ".beads")
	a := qApp(t, repo) // unmarked in beads_visibility: — fail closed, public
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
	slot := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	if b, _ := os.ReadFile(slot); string(b) != CommitGuardHook(VisibilityPublic) {
		t.Fatalf("fixture: the slot does not carry the public stamp to drift from")
	}

	write(t, a.ConfigPath, "queue_repo: "+repo+"\nbeads_visibility:\n  "+repo+": private\n")
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	if err != nil || c.SHA == "" {
		t.Fatalf("CommitQueueJSONL = (%+v, %v)", c, err)
	}
	if b, _ := os.ReadFile(slot); string(b) != CommitGuardHook(VisibilityPrivate) {
		t.Errorf("the slot still carries the stamp config no longer calls for")
	}
}
