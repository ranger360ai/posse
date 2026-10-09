package treepins

// QA pins for ranger-base-nusr.
//
// The finding: bd 0.49.1's non-atomic `create --deps` is not a socket problem.
// Measured on a VACUUM INTO snapshot of the fleet db, direct storage mode, no
// daemon and therefore no socket at all: `bd --no-daemon create ... --deps
// discovered-from:ranger-base-okbr` was killed at 90s with the issue row
// COMMITTED and the dependency absent. The 30s timeout developer saw through the
// daemon is incidental — raising it buys a longer hang, not an edge. The one
// thing that fixes it is removing the symmetric pairs: after the prune, the
// same create against okbr, x6ic and cpyb ran in 0.38-0.41s with the edge
// present.
//
// So the prune is the fix, and these pin the two tools that make it hold:
// `verify-bd-dep-safety.sh --gate` (drift detector) and
// `prune-bd-relates-to.sh` (the prune itself, dry by default).
//
// Also pinned here: the prune is DRY unless `--apply` is typed. Pruning the
// fleet store is a deletion on live state; a `make` target must never do it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// prunRun runs a repo script with cwd=root and BEADS_DB stripped, like bdsRun.
//
// A key the caller supplies in env is dropped from the inherited copy rather
// than appended behind it: a child shell handed two PATH rows reads whichever
// its libc picks, so an override that only appends is an override that
// sometimes does nothing (ranger-base-9mjxb, where the override puts a fake bd
// in front of the real one).
func prunRun(t *testing.T, script, root string, env []string, args ...string) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(script)
	if err != nil {
		t.Fatal(err)
	}
	override := map[string]bool{}
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			override[kv[:i]] = true
		}
	}
	cmd := exec.Command(abs, args...)
	cmd.Dir = root
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "BEADS_DB=") {
			continue
		}
		if i := strings.IndexByte(kv, '='); i > 0 && override[kv[:i]] {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("%s: %v\n%s", script, err, out)
		}
		code = ee.ExitCode()
	}
	return string(out), code
}

// prunSeed writes root/.beads/beads.db from the given SQL and returns root.
func prunSeed(t *testing.T, seed string) string {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	root := t.TempDir()
	beads := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sqlite3", filepath.Join(beads, "beads.db"))
	cmd.Stdin = strings.NewReader(`
CREATE TABLE dependencies (
  issue_id TEXT NOT NULL,
  depends_on_id TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'blocks',
  PRIMARY KEY (issue_id, depends_on_id, type)
);
` + seed)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seed: %v\n%s", err, out)
	}
	return root
}

const prunPair = `
INSERT INTO dependencies VALUES ('a','b','relates-to'), ('b','a','relates-to');
INSERT INTO dependencies VALUES ('c','a','blocks'), ('d','e','blocks');
`

func TestQABdDepSafetyGateFailsOnAnyPair(t *testing.T) {
	const script = "scripts/verify-bd-dep-safety.sh"

	out, code := prunRun(t, script, prunSeed(t, prunPair), nil, "--gate")
	if code != 1 {
		t.Fatalf("--gate must fail while a pair exists: exit %d\n%s", code, out)
	}
	// It has to name the nodes, or nobody can act on it.
	for _, want := range []string{"UNSAFE:", "\n  a\n", "\n  b\n", "prune-bd-relates-to.sh"} {
		if !strings.Contains(out, want) {
			t.Errorf("--gate output missing %q\n%s", want, out)
		}
	}
	// c only *reaches* the pair; it is not in one, so the gate does not list it.
	if strings.Contains(out, "\n  c\n") {
		t.Errorf("--gate listed a reacher as a pair member\n%s", out)
	}

	// The pruned store is the steady state this whole bead exists to reach.
	out, code = prunRun(t, script, prunSeed(t, "INSERT INTO dependencies VALUES ('c','a','blocks');"), nil, "--gate")
	if code != 0 {
		t.Fatalf("--gate must pass with no pair: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "clean:") {
		t.Errorf("--gate did not report clean\n%s", out)
	}
}

// A WAL-mode db whose -shm is gone cannot be opened `mode=ro` at all; sqlite
// returns CANTOPEN(14). bd leaves the store in exactly that state after a
// write when no daemon holds it, and the audit used to die with a bare sqlite
// error there — a checker that errors instead of answering is a checker that
// gets ignored.
func TestQABdDepSafetyReadsAWALDBWithNoShm(t *testing.T) {
	root := prunSeed(t, prunPair)
	db := filepath.Join(root, ".beads", "beads.db")
	if out, err := exec.Command("sqlite3", db, "PRAGMA journal_mode=WAL; INSERT INTO dependencies VALUES ('x','y','blocks');").CombinedOutput(); err != nil {
		t.Fatalf("switch to WAL: %v\n%s", err, out)
	}
	// sqlite3 checkpoints and removes -wal/-shm on a clean close; assert the
	// db really is WAL-mode and that a read-only open of it fails, so this
	// test is pinning the fallback and not a no-op.
	mode, err := exec.Command("sqlite3", db, "PRAGMA journal_mode;").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(mode)) != "wal" {
		t.Skipf("db did not stay in WAL mode (%q); nothing to pin", strings.TrimSpace(string(mode)))
	}
	if err := exec.Command("sqlite3", "file:"+db+"?mode=ro", "SELECT 1").Run(); err == nil {
		t.Skip("this sqlite build opens a shm-less WAL db read-only; nothing to pin")
	}

	out, code := prunRun(t, "scripts/verify-bd-dep-safety.sh", root, nil, "--gate")
	if code != 1 {
		t.Fatalf("gate on a shm-less WAL db: exit %d, want 1\n%s", code, out)
	}
	if strings.Contains(out, "unable to open database") {
		t.Errorf("gate leaked a raw sqlite error instead of reading a copy\n%s", out)
	}
}

func TestQAPruneRelatesToIsDryUnlessApplyIsTyped(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"
	root := prunSeed(t, prunPair)
	db := filepath.Join(root, ".beads", "beads.db")

	out, code := prunRun(t, script, root, nil)
	if code != 0 {
		t.Fatalf("dry run: exit %d\n%s", code, out)
	}
	// The planned argv, verbatim, including the flags --apply would really
	// use: a plan that prints a shorter command than the one it runs is a
	// plan an operator cannot check (ranger-base-9mjxb).
	for _, want := range []string{"symmetric relates-to pairs: 1", "rows to delete: 2", "bd --no-daemon dep unrelate a b", "dry run — nothing changed"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run missing %q\n%s", want, out)
		}
	}

	// The pair is still there. A dry run that mutates is the whole nightmare.
	left, err := exec.Command("sqlite3", db, "SELECT count(*) FROM dependencies WHERE type='relates-to';").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(left)) != "2" {
		t.Errorf("dry run changed the db: %s relates-to rows left, want 2", strings.TrimSpace(string(left)))
	}

	// Nothing to do is success, not an error — the steady state after a prune.
	out, code = prunRun(t, script, prunSeed(t, "INSERT INTO dependencies VALUES ('c','a','blocks');"), nil)
	if code != 0 || !strings.Contains(out, "nothing to prune") {
		t.Errorf("clean store: exit %d\n%s", code, out)
	}
}

// `bd dep unrelate` only speaks relates_to. A symmetric pair of another type is
// the same landmine but not this script's to guess at, so it refuses loudly
// rather than reporting a prune it did not do.
func TestQAPruneRelatesToRefusesAForeignPairAndABadFlag(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"

	out, code := prunRun(t, script, prunSeed(t, "INSERT INTO dependencies VALUES ('a','b','blocks'), ('b','a','blocks');"), nil)
	if code != 2 {
		t.Fatalf("foreign pair: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "blocks a b") {
		t.Errorf("refusal does not name the pair\n%s", out)
	}

	out, code = prunRun(t, script, prunSeed(t, prunPair), nil, "--yolo")
	if code != 2 || !strings.Contains(out, "usage:") {
		t.Errorf("unknown flag: exit %d\n%s", code, out)
	}

	// No db is "I could not check", never a green "nothing to prune".
	out, code = prunRun(t, script, t.TempDir(), nil)
	if code != 2 {
		t.Fatalf("missing db: exit %d, want 2\n%s", code, out)
	}
	if strings.Contains(out, "nothing to prune") {
		t.Errorf("missing db reported clean\n%s", out)
	}
}

// ─── the store-selecting flag, and the repo that owns the store ────────────
//
// github.com/ranger360ai/posse issue 9 (ranger-base-9mjxb): from a cwd inside
// repo A, bd 0.50.3 handed an explicit store-selecting flag naming repo B's
// database was MEASURED by the operator auto-importing A's `issues.jsonl`
// into B — a read verb writing one repository's records into another's store,
// silently. We are pinned at that version and will not file upstream
// (standing ruling 2026-10-08), so the posse-side answer is the only one that
// does not depend on bd: never hand bd a store-selecting flag at all. Name
// the repo that owns the store, chdir into it, and let bd resolve its own.
//
// This script was posse's one such call site — three writes, from whatever
// directory the operator happened to be in, against a database `BEADS_DB` is
// documented to point at in another repository. These two pins are the
// behaviour that replaced it; the census that says no other call site came
// back is TestQANoShippedPosseCallHandsBdAStoreSelectingFlag (bddbflag_qa
// _test.go).

// prunRepo is prunSeed plus the one property the script's store_owner
// requires of a store's parent: that it is a git work tree's toplevel. Plain
// t.TempDir() + `git init`, matching this package's other git fixtures
// (hookfreshness_qa_test.go's hfNewRig) — the tolerant-root rule is stated
// over internal/posse's corpus, not this one.
func prunRepo(t *testing.T, seed string) string {
	t.Helper()
	root := prunSeed(t, seed)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "qa@example.invalid"},
		{"config", "user.name", "qa"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, root, err, out)
		}
	}
	return root
}

// prunFakeBd puts a recording `bd` first on PATH, appends the override to
// env, and returns the path its log will be written to.
//
// It records the directory it was CALLED in and its whole argv — the two
// facts this bead is about — and it resolves the store the way the real bd
// does when no store-selecting flag is given: from that directory. So a
// chdir that put bd on the wrong store does not merely read wrong in the
// log; the unrelate deletes nothing, and the script's own post-apply re-read
// of the REAL store then reports the pairs still there. The assertion and
// the rig fail in the same direction.
func prunFakeBd(t *testing.T, env *[]string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	body := "#!/bin/sh\n" +
		"printf 'cwd=%s argv=%s\\n' \"$(pwd -P)\" \"$*\" >> " + shQuote(log) + "\n" +
		"for a in \"$@\"; do\n" +
		"\tif [ \"$a\" = unrelate ]; then\n" +
		"\t\tsqlite3 \"$PWD/.beads/beads.db\" \"DELETE FROM dependencies WHERE type='relates-to';\" || exit 1\n" +
		"\tfi\n" +
		"done\n" +
		"exit 0\n"
	// WriteExecutable, not os.WriteFile: this file is exec'd, and the write
	// descriptor must not be inheritable by a sibling fork (execwrite_test.go).
	if err := WriteExecutable(filepath.Join(dir, "bd"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	*env = append(*env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// shQuote single-quotes a path for the fake's body. t.TempDir() paths hold no
// quote on any box this runs on, so this is about saying so rather than
// hoping.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// prunCalls reads the fake's log, or returns nil when it was never written —
// "bd did not run" and "bd ran and logged nothing" must not be the same answer
// to a caller asserting the first.
func prunCalls(t *testing.T, log string) []string {
	t.Helper()
	b, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func prunRelatesRows(t *testing.T, root string) string {
	t.Helper()
	db := filepath.Join(root, ".beads", "beads.db")
	out, err := exec.Command("sqlite3", db, "SELECT count(*) FROM dependencies WHERE type='relates-to';").Output()
	if err != nil {
		t.Fatalf("counting %s: %v", db, err)
	}
	return strings.TrimSpace(string(out))
}

// --apply writes from inside the repo that owns the store, with no
// store-selecting flag, whatever directory it was invoked from.
func TestQAPruneRelatesToAppliesFromTheStoresOwnRepo(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"
	store := prunRepo(t, prunPair)
	// A DIFFERENT repo to stand in, with its own store and its own rows. If
	// the script ran bd from here, every assertion below reads it.
	elsewhere := prunRepo(t, "INSERT INTO dependencies VALUES ('z','y','blocks');")

	env := []string{"BEADS_DB=" + filepath.Join(store, ".beads", "beads.db")}
	log := prunFakeBd(t, &env)

	out, code := prunRun(t, script, elsewhere, env, "--apply")
	if code != 0 {
		t.Fatalf("--apply from another repo: exit %d\n%s", code, out)
	}

	calls := prunCalls(t, log)
	if len(calls) != 3 {
		t.Fatalf("want 3 bd calls (two comments, one unrelate), got %d:\n%s\n--- script output:\n%s",
			len(calls), strings.Join(calls, "\n"), out)
	}
	owner, err := filepath.EvalSymlinks(store)
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := filepath.EvalSymlinks(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range calls {
		if !strings.HasPrefix(c, "cwd="+owner+" ") {
			t.Errorf("a bd call was not made from the store's own repo (%s): %q", owner, c)
		}
		if strings.Contains(c, "cwd="+stranger+" ") {
			t.Errorf("a bd call was made from the invoking repo, which is the defect: %q", c)
		}
		// The flag itself, in both spellings bd accepts. A chdir that is
		// right and a flag that is still there is no fix at all: the flag is
		// what bd reads.
		if strings.Contains(c, "--db") {
			t.Errorf("a bd call still carries a store-selecting flag: %q", c)
		}
		if !strings.Contains(c, "--no-daemon") {
			t.Errorf("a bd call is missing --no-daemon: %q", c)
		}
	}
	for _, want := range []string{"comments add a ", "comments add b ", "dep unrelate a b"} {
		if !strings.Contains(strings.Join(calls, "\n"), want) {
			t.Errorf("no bd call ran %q:\n%s", want, strings.Join(calls, "\n"))
		}
	}

	// Both directions of the import the bead is about: the owning store lost
	// its pair, and the repo we stood in neither gained rows nor lost its own.
	if got := prunRelatesRows(t, store); got != "0" {
		t.Errorf("the owning store still holds %s relates-to row(s) after --apply", got)
	}
	if got := prunRelatesRows(t, elsewhere); got != "0" {
		t.Errorf("the invoking repo's store gained %s relates-to row(s) — records crossed repositories", got)
	}
	blocks, err := exec.Command("sqlite3", filepath.Join(elsewhere, ".beads", "beads.db"),
		"SELECT count(*) FROM dependencies WHERE type='blocks';").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(blocks)) != "1" {
		t.Errorf("the invoking repo's own rows changed: %s blocks row(s), want 1", strings.TrimSpace(string(blocks)))
	}
}

// A store whose owning repo cannot be named is refused BEFORE the first
// write, and the dry run over the same store still prints its plan — the two
// safe failure directions are opposite here, and both are the point.
func TestQAPruneRelatesToRefusesToApplyWithoutAnOwningRepo(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"

	// 1. The parent is no git work tree. prunSeed, not prunRepo.
	root := prunSeed(t, prunPair)
	var env []string
	log := prunFakeBd(t, &env)

	out, code := prunRun(t, script, root, env, "--apply")
	if code != 2 {
		t.Fatalf("--apply with no owning repo: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "cannot name the repo that owns") {
		t.Errorf("the refusal does not say what it could not name\n%s", out)
	}
	if calls := prunCalls(t, log); len(calls) != 0 {
		t.Errorf("the refusal came AFTER %d bd call(s), which is after the damage:\n%s", len(calls), strings.Join(calls, "\n"))
	}
	if got := prunRelatesRows(t, root); got != "2" {
		t.Errorf("a refused --apply still changed the store: %s relates-to rows, want 2", got)
	}

	// The dry run writes nothing, so it says so and carries on.
	out, code = prunRun(t, script, root, env)
	if code != 0 {
		t.Fatalf("dry run with no owning repo: exit %d, want 0\n%s", code, out)
	}
	for _, want := range []string{"cannot name the repo that owns", "dry run continues", "symmetric relates-to pairs: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run missing %q\n%s", want, out)
		}
	}
	if calls := prunCalls(t, log); len(calls) != 0 {
		t.Errorf("a dry run ran bd %d time(s):\n%s", len(calls), strings.Join(calls, "\n"))
	}

	// 2. The parent IS a git toplevel, and its own .beads redirects to
	// ANOTHER store. This is the shape this instance actually runs — two
	// repos whose .beads redirect at a third — and it is why condition 3 of
	// store_owner exists: a chdir there reads a database that is not the one
	// the SQL above measured, so the prune would report rows it never
	// touched.
	redirected := prunRepo(t, prunPair)
	target := prunRepo(t, prunPair)
	if err := os.WriteFile(filepath.Join(redirected, ".beads", "redirect"),
		[]byte(filepath.Join(target, ".beads")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env2 := []string{"BEADS_DB=" + filepath.Join(redirected, ".beads", "beads.db")}
	log2 := prunFakeBd(t, &env2)
	out, code = prunRun(t, script, redirected, env2, "--apply")
	if code != 2 {
		t.Fatalf("--apply on a store its repo redirects away from: exit %d, want 2\n%s", code, out)
	}
	if calls := prunCalls(t, log2); len(calls) != 0 {
		t.Errorf("bd ran against a store the chdir would not have reached:\n%s", strings.Join(calls, "\n"))
	}
	if got := prunRelatesRows(t, target); got != "2" {
		t.Errorf("the redirect TARGET changed: %s relates-to rows, want 2", got)
	}
}

func TestQAPruneRelatesToMakefileWiring(t *testing.T) {
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"verify-bd-no-relate-pairs:\n\tscripts/verify-bd-dep-safety.sh --gate\n",
		"prune-bd-relates-to:\n\tscripts/prune-bd-relates-to.sh\n",
	} {
		if !strings.Contains(string(mk), want) {
			t.Errorf("Makefile lost %q", strings.SplitN(want, ":", 2)[0])
		}
	}
	// The make target must stay dry: --apply deletes rows from the live store.
	// Recipe lines only — the surrounding comment names --apply on purpose.
	for _, line := range strings.Split(string(mk), "\n") {
		if strings.HasPrefix(line, "\t") && strings.Contains(line, "prune-bd-relates-to.sh") && strings.Contains(line, "--apply") {
			t.Errorf("a make recipe must never run the prune with --apply: %q", line)
		}
	}
	info, err := os.Stat("scripts/prune-bd-relates-to.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/prune-bd-relates-to.sh is not executable")
	}
}
