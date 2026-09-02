package posse

// QA pins filed verifying four closes (verify bead ranger-base-uqslx):
// ranger-base-ws09 (the account ledger's writability half),
// ranger-base-38a1 (the memory credential scan's unreadable-change arm) and
// ranger-base-f6lk (the two widened auto-reap arms). Each close arrived with
// its own pins and its own mutation sweep; these are the arms that sweep did
// not have, found by attacking the fix rather than by reading it, and each
// one was checked against a build with the thing it names removed.
//
// Kept in a file of its own under ADR 0022 (single writer per file): the
// three subjects live in three other test files, each with a different
// owner, and a QA pin must not have to wait on one of them.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// E1: fresh install — cap set, StateDir does not exist yet. The probe must
// not brake: it creates the dir and a temp file and removes it.
func TestQAVerifyE1FreshStateDirUnderACapStillLaunches(t *testing.T) {
	f := oneCodexBead(t, "uncounted_cap_codex: 5\n")
	if _, err := os.Stat(f.b.App.StateDir); err == nil {
		os.RemoveAll(f.b.App.StateDir)
	}
	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("a fresh StateDir under an armed cap must still launch, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "cannot be appended to") {
		t.Errorf("fresh install must not read as unwritable:\n%s", out)
	}
	if got := f.uncountedLedger(t); len(got) != 1 {
		t.Errorf("the launch must be recorded: %v", got)
	}
	ents, err := os.ReadDir(f.b.App.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		// The probe writes into StateDir to answer the question; a probe
		// that leaves its answer behind is a file the next reader has to
		// explain.
		if strings.HasPrefix(e.Name(), ".ledger-probe-") {
			t.Errorf("the probe left its temp file behind: %s", e.Name())
		}
	}
}

// E2: ws09's class one level up — the ledger absent and its DIRECTORY
// unwritable. The count reads 0 (absent = empty), so the cap would look like
// room. Nothing reaches the account stage at all: the launcher lock lives in
// the same StateDir, so the pass refuses before routing and says so on
// stderr. Zero launches, nothing claimed, and the refusal is named — which is
// the property ws09 is about, kept by an earlier guard.
func TestQAVerifyE2UnwritableStateDirRefusesThePass(t *testing.T) {
	f := oneCodexBead(t, "uncounted_cap_codex: 1\n")
	if err := os.MkdirAll(f.b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.b.App.StateDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(f.b.App.StateDir, 0o755) })
	// 0555 is a promise about a uid, not about the process.
	if err := f.b.App.AppendUncounted(LedgerEntry{Runtime: "codex"}); err == nil {
		t.Skip("test process can write into a 0555 dir")
	}
	n, rerr := f.d.Run("", "", 0)
	if n != 0 || rerr == nil {
		t.Fatalf("an unwritable StateDir must refuse the pass, got n=%d err=%v:\n%s", n, rerr, dispatcherOut(f.d))
	}
	if !strings.Contains(rerr.Error(), "launcher lock") {
		t.Errorf("the refusal must name what could not be written: %v", rerr)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("nothing may be claimed: %s", calls)
	}
}

// E3: unreadable AND unwritable at once must be named unreadable — the fault
// an operator fixes first (readOverflowCount's order, the closer's second claim).
func TestQAVerifyE3UnreadableBeatsUnwritable(t *testing.T) {
	f := oneCodexBead(t, "uncounted_cap_codex: 1\n")
	os.MkdirAll(f.b.App.StateDir, 0o755)
	if err := os.MkdirAll(f.b.App.UncountedLogPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.b.App.StateDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(f.b.App.StateDir, 0o755) })
	line, kind := f.d.uncountedSkip("codex")
	if kind != "uncounted_cap_codex: ledger unreadable" {
		t.Errorf("kind = %q, want the unreadable one (line: %s)", kind, line)
	}
}

// ─── ranger-base-38a1 hostile arms (QA, verifying the close) ─────────────────

// H1: a RENAME of a binary file that also gains a credential. `--numstat -z`
// spells a detected rename `-\t-\t\0old\0new\0` — the record's own path
// field is EMPTY and the two names are spent on the next two records — so an
// arm that reads the path field alone sees "" and passes the file, and one
// that does not CONSUME the two extra records walks off by two. Either way
// the new blob reaches the commit unscanned.
//
// The fixture is large on purpose: git only reports a rename when the two
// blobs are similar enough, and a 15-byte file plus a credential line is not.
// Measured on git 2.50.1 — a 300-line binary file renamed and appended to is
// one rename record; the small one is an add and a delete, which the ordinary
// arm catches and which therefore measures nothing about this walk.
func TestQAVerifyH1RenamedBinaryWithACredentialHolds(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	var body strings.Builder
	body.WriteString("A\x00")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&body, "line %d filler content here\n", i)
	}
	write(t, filepath.Join(repo, ConstitutionSourceDir, "personas", "dev", "capture.bin"), body.String())
	mustGit(t, repo, "add", "--", ConstitutionSourceDir+"/personas/dev/capture.bin")
	mustGit(t, repo, "commit", "-q", "-m", "a binary artefact", "--", ConstitutionSourceDir+"/personas/dev/capture.bin")
	before := mustGit(t, repo, "rev-parse", "HEAD")

	// STAGED, so `git diff HEAD` reports a rename rather than a delete plus
	// an untracked file — the untracked arm reads whole files off disk and
	// would catch that one before this walk is reached.
	const leaked = "sk-ant-api03-HHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHH"
	mustGit(t, repo, "mv", "--", ConstitutionSourceDir+"/personas/dev/capture.bin", ConstitutionSourceDir+"/personas/dev/renamed.bin")
	write(t, filepath.Join(repo, ConstitutionSourceDir, "personas", "dev", "renamed.bin"), body.String()+leaked+"\n")
	if d := mustGit(t, repo, "diff", "HEAD", "--numstat", "--", ConstitutionSourceDir+"/personas/dev"); !strings.Contains(d, "=>") {
		t.Fatalf("fixture is not one rename record to git, so the rename walk is not measured:\n%s", d)
	}

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("a renamed binary file was committed unscanned as %s:\n%s", after, headFiles(t, repo))
	}
	line := landing.Memory.Line()
	if !strings.Contains(line, "renamed.bin") || !strings.Contains(line, "not checked for credentials") {
		t.Errorf("the hold must name the file and say it was not checked: %q", line)
	}
	if strings.Contains(line, leaked) {
		t.Errorf("the refusal echoed the credential: %q", line)
	}
}

// H2: a DELETED binary sorted BEFORE a MODIFIED one. A deletion counts `-`
// `-` too and is skipped by design; the arm must go on to the record after it
// rather than answering on the first `-` `-` it meets.
func TestQAVerifyH2DeletionBeforeAModifiedBinaryStillHolds(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	gone := filepath.Join(repo, ConstitutionSourceDir, "personas", "dev", "a-gone.bin")
	kept := filepath.Join(repo, ConstitutionSourceDir, "personas", "dev", "z-kept.bin")
	write(t, gone, "one\x00two\n")
	write(t, kept, "three\x00four\n")
	mustGit(t, repo, "add", "--", ConstitutionSourceDir+"/personas/dev")
	mustGit(t, repo, "commit", "-q", "-m", "two binary artefacts", "--", ConstitutionSourceDir+"/personas/dev")
	before := mustGit(t, repo, "rev-parse", "HEAD")

	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	const leaked = "sk-ant-api03-IIIIIIIIIIIIIIIIIIIIIIIIIIIIIIII"
	write(t, kept, "three\x00four\n"+leaked+"\n")

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("a modified binary after a deletion was committed unscanned as %s:\n%s", after, headFiles(t, repo))
	}
	if line := landing.Memory.Line(); !strings.Contains(line, "z-kept.bin") {
		t.Errorf("the hold must name the file it could not read: %q", line)
	}
}

// H3: the same two mistakes at once — a rename record FOLLOWED by a modified
// binary. If the rename's two extra records are not consumed, the walk is off
// by two and the modified file behind it is never reached.
func TestQAVerifyH3RenameThenModifiedBinaryBothReached(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	dir := filepath.Join(repo, ConstitutionSourceDir, "personas", "dev")
	write(t, filepath.Join(dir, "a-moved.bin"), "aaa\x00bbb\n")
	write(t, filepath.Join(dir, "z-kept.bin"), "ccc\x00ddd\n")
	mustGit(t, repo, "add", "--", ConstitutionSourceDir+"/personas/dev")
	mustGit(t, repo, "commit", "-q", "-m", "two binary artefacts", "--", ConstitutionSourceDir+"/personas/dev")
	before := mustGit(t, repo, "rev-parse", "HEAD")

	mustGit(t, repo, "mv", "--", ConstitutionSourceDir+"/personas/dev/a-moved.bin", ConstitutionSourceDir+"/personas/dev/a-newname.bin")
	const leaked = "sk-ant-api03-JJJJJJJJJJJJJJJJJJJJJJJJJJJJJJJJ"
	write(t, filepath.Join(dir, "z-kept.bin"), "ccc\x00ddd\n"+leaked+"\n")

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("committed unscanned as %s:\n%s", after, headFiles(t, repo))
	}
	if line := landing.Memory.Line(); !strings.Contains(line, "not checked for credentials") {
		t.Errorf("the hold must say it was not checked: %q", line)
	}
}

// ─── ranger-base-f6lk hostile arms (QA, verifying the close) ─────────────────

// R1: the crew arm's "its bead reads closed" half, asked in the direction the
// pins do not ask it. Every crew-arm pin in reapresidue_test.go feeds a CLOSED
// bead and varies the age, the name or the tree; none varies the bead. A crew
// mark on a dispatch-made session whose bead is still OPEN is the operator in
// a conversation about live work, and no grace makes that residue.
func TestQAVerifyR1CrewArmNeverTakesAnOpenBead(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1", Crew: true},
		"in_progress", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a crew session over an OPEN bead is a conversation about live work, at any age:\n%s", dispatcherOut(d))
	}
}

// R2: the bead's hardest constraint — "never reap a tree holding uncommitted
// work" — asked of the CREW arm. Both dirty-tree pins in reapresidue_test.go
// build the UNPOINTED shape (no bead, no mark), so the refusal is measured on
// one of the two populations that rest on it. The two arms share residueHolds
// today; this is what goes red the day they stop sharing it, and the crew arm
// is the one whose session had an operator in it.
func TestQAVerifyR2CrewArmNeverTakesADirtyTree(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := SessionForBead("ranger", repo, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: repo, Bead: "a-1", Crew: true},
		"closed", DefaultCrewReapAfter+30*24*time.Hour)
	write(t, filepath.Join(repo, "unsaved.txt"), "353 lines nobody committed\n")
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a crew session over a tree holding uncommitted work is never reaped, however old:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "NOT reaped") || !strings.Contains(out, "unsaved.txt") {
		t.Errorf("the refusal must name the session AND what it holds, every pass it is true:\n%s", out)
	}
}
