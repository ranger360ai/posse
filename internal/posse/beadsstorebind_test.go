//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-ub2x9: a bd call is bound to the store of the directory the
// CALLER named, and an inherited `BEADS_DIR` cannot redirect it.
//
// The bug (codex review finding 1, docs/notes.d/ranger-base-b0fsz.md):
// `Bd.runOnce` set `cmd.Dir` and nothing else, and bd resolves `$BEADS_DIR`
// BEFORE its working directory. posse itself puts that variable in every
// session it launches (planLaunch, herdrback.go), so a posse command run
// inside a session read the session's store while `ReadyAll` stamped the
// returned rows `RepoIssue{Dir: <the directory it meant to query>}` — one
// repo's queue wearing another repo's name.
//
// WHY THE PACKAGE'S OWN FAKE CANNOT ASK THIS. `fakeBd` (herdr_test.go)
// reads its fixtures by RELATIVE path, so it resolves by working directory
// alone and answers identically whether or not the child inherited a
// contradictory binding — every existing scan test passes over the defect.
// The fake here is the smallest one that models the substrate's actual
// precedence, `$BEADS_DIR` else `$cwd/.beads`, measured on the pinned bd
// 0.50.3 (see bdStoreEnv, beads.go). The arms below then assert on RETURNED
// ROWS, not on argv: the row carries the id of the store that served it, so
// "which store answered" and "which directory was asked" are two facts the
// test can compare rather than one it has to take on trust.
//
// MUTATION-CHECKED against the unfixed runner (`cmd.Dir` alone, 2026-09-10):
// four arms red and each one prints the defect — Ready against a second
// store returns `a-1`, the redirect arm returns `a-1`, the store-less
// directory returns `a-1`, and ReadyAll hands back the SAME row twice under
// two different `Dir` labels. The inherited-binding, environment and
// substrate-model arms stay green, which is what makes them controls rather
// than duplicates.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// storeBindCall is one recorded child: what it resolved as its store, and
// whether it was handed a BEADS_DIR at all.
type storeBindCall struct {
	beadsDir string // "" when the variable was absent from the child's env
	set      bool
	actor    string // BD_ACTOR, the other variable a bd child is launched with
	cwd      string
	argv     string
}

// storeBindBd is a bd that resolves its store the way bd does — `$BEADS_DIR`
// else `$cwd/.beads` — and serves that store's `rows.json` as its answer, so
// the rows say which store was actually opened. `blocked` answers `[]` for
// every store: Ready subtracts it, and a blocked set that echoed the ready
// set would empty every arm below.
func storeBindBd(t *testing.T) (Bd, func() []storeBindCall) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	bin := filepath.Join(dir, "bd")
	body := "#!/bin/sh\n" +
		"store=\"${BEADS_DIR:-$(pwd)/.beads}\"\n" +
		// `${BEADS_DIR+1}` rather than a sentinel value: "set to the empty
		// string" and "absent" are different answers here, and no in-band
		// marker can say so — a NUL one is eaten by printf.
		"printf '%s\\t%s\\t%s\\t%s\\t%s\\n' \"${BEADS_DIR+set}\" \"$BEADS_DIR\" \"$BD_ACTOR\" \"$(pwd)\" \"$*\" >> " + log + "\n" +
		"case \" $* \" in *\\ blocked\\ *) echo '[]'; exit 0;; esac\n" +
		"if [ -f \"$store/rows.json\" ]; then cat \"$store/rows.json\"; else echo '[]'; fi\n"
	if err := WriteExecutable(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return Bd{Bin: bin}, func() []storeBindCall {
		b, err := os.ReadFile(log)
		if err != nil {
			return nil
		}
		var calls []storeBindCall
		for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
			f := strings.SplitN(line, "\t", 5)
			if len(f) != 5 {
				continue
			}
			calls = append(calls, storeBindCall{
				set: f[0] == "set", beadsDir: f[1], actor: f[2], cwd: f[3], argv: f[4],
			})
		}
		return calls
	}
}

// storeBindRepo is a repo whose store serves exactly one row, named for the
// repo, so a row can be traced back to the store that held it.
func storeBindRepo(t *testing.T, id string) string {
	t.Helper()
	dir := t.TempDir()
	home := filepath.Join(dir, beadsDirName)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	row := `[{"id":"` + id + `","title":"row held by ` + id + `"}]`
	if err := os.WriteFile(filepath.Join(home, "rows.json"), []byte(row), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// storeBindIDs is what came back, by id, so a failure prints the store that
// answered rather than a struct dump.
func storeBindIDs(issues []BdIssue) []string {
	out := make([]string, 0, len(issues))
	for _, is := range issues {
		out = append(out, is.ID)
	}
	return out
}

// lastCall is the child the assertion is about — the arms below each end on
// the call they care about (Ready's own `blocked` cross-check runs against
// the same directory, so either call would do).
func lastCall(t *testing.T, calls []storeBindCall) storeBindCall {
	t.Helper()
	if len(calls) == 0 {
		t.Fatal("the fake bd recorded no call at all — nothing below means anything")
	}
	return calls[len(calls)-1]
}

// The defect, verbatim: an inherited binding naming one repo, a call naming
// another, and rows that must belong to the directory that was ASKED for.
func TestBdCallsBindToTheDirectoryTheyName(t *testing.T) {
	b, calls := storeBindBd(t)
	a := storeBindRepo(t, "a-1")
	c := storeBindRepo(t, "c-1")
	// The session's own binding, exactly as planLaunch writes it.
	t.Setenv("BEADS_DIR", filepath.Join(a, beadsDirName))

	issues, err := b.Ready(c, "")
	if err != nil {
		t.Fatalf("Ready(%s): %v", c, err)
	}
	if got := storeBindIDs(issues); len(got) != 1 || got[0] != "c-1" {
		t.Errorf("Ready named %s and must read %s's store; got rows %v — the inherited BEADS_DIR redirected the query",
			AbbrevHome(c), AbbrevHome(c), got)
	}
	if call := lastCall(t, calls()); call.beadsDir != filepath.Join(c, beadsDirName) {
		t.Errorf("the child's BEADS_DIR = %q, want the store of the directory asked for (%q)",
			call.beadsDir, filepath.Join(c, beadsDirName))
	}

	// CONTROL, in the same process and the same inherited environment: the
	// call that names A still reads A. A fix that bound every call to the
	// same wrong answer would pass the arm above and fail here.
	issues, err = b.Ready(a, "")
	if err != nil {
		t.Fatalf("Ready(%s): %v", a, err)
	}
	if got := storeBindIDs(issues); len(got) != 1 || got[0] != "a-1" {
		t.Errorf("Ready named %s and must read its store; got rows %v", AbbrevHome(a), got)
	}
}

// The redirect hop is part of "which store", and it is beadsHome's — the
// same answer the census, the seatbelt grant and the cage mount take. The
// inherited value names a THIRD store here, so nothing but the hop can
// produce the expected row.
func TestBdCallsFollowTheRedirectOfTheDirectoryTheyName(t *testing.T) {
	b, calls := storeBindBd(t)
	a := storeBindRepo(t, "a-1")
	target := storeBindRepo(t, "t-1")
	work := t.TempDir()
	home := filepath.Join(work, beadsDirName)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, beadsRedirect),
		[]byte(filepath.Join(target, beadsDirName)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BEADS_DIR", filepath.Join(a, beadsDirName))

	issues, err := b.Ready(work, "")
	if err != nil {
		t.Fatalf("Ready(%s): %v", work, err)
	}
	if got := storeBindIDs(issues); len(got) != 1 || got[0] != "t-1" {
		t.Errorf("a redirect names the store of record; got rows %v", got)
	}
	if call := lastCall(t, calls()); call.beadsDir != filepath.Join(target, beadsDirName) {
		t.Errorf("the child's BEADS_DIR = %q, want the redirect target (%q)",
			call.beadsDir, filepath.Join(target, beadsDirName))
	}
}

// A directory with no `.beads` has no store of its own, and the inherited
// value naming another repo is the whole of the bug — so the variable is
// SHED rather than left, and the question goes back to bd's own resolution
// from the working directory.
func TestBdCallsShedAnInheritedStoreWhereTheDirectoryHasNone(t *testing.T) {
	b, calls := storeBindBd(t)
	a := storeBindRepo(t, "a-1")
	bare := t.TempDir() // no .beads at all
	t.Setenv("BEADS_DIR", filepath.Join(a, beadsDirName))

	issues, err := b.Ready(bare, "")
	if err != nil {
		t.Fatalf("Ready(%s): %v", bare, err)
	}
	if got := storeBindIDs(issues); len(got) != 0 {
		t.Errorf("a directory with no store must not answer with another repo's queue; got rows %v", got)
	}
	if call := lastCall(t, calls()); call.set {
		t.Errorf("the child carried BEADS_DIR = %q into a directory that has no store; it must carry none", call.beadsDir)
	}
}

// The deliberate exception, and the one AGENTS.md states to personas: a call
// that names NO directory names no store either, so the session's own
// binding stands. Shedding it here would move every cwd-relative posse call
// off the store its session was launched against.
func TestBdCallsWithNoDirectoryKeepTheSessionBinding(t *testing.T) {
	b, calls := storeBindBd(t)
	a := storeBindRepo(t, "a-1")
	want := filepath.Join(a, beadsDirName)
	t.Setenv("BEADS_DIR", want)

	if _, err := b.Ready("", ""); err != nil {
		t.Fatalf("Ready(\"\"): %v", err)
	}
	if call := lastCall(t, calls()); call.beadsDir != want {
		t.Errorf("a call naming no directory must inherit the session's store: BEADS_DIR = %q, want %q", call.beadsDir, want)
	}
}

// Binding the store replaces the child's whole environment, so the rest of
// it has to survive the rebuild: BD_ACTOR — the variable that makes a bd
// write attribute to the persona — and everything else a bd child is given
// are not this fix's to drop.
func TestBdCallsKeepTheRestOfTheEnvironment(t *testing.T) {
	b, calls := storeBindBd(t)
	a := storeBindRepo(t, "a-1")
	t.Setenv("BEADS_DIR", filepath.Join(a, beadsDirName))
	t.Setenv("BD_ACTOR", "an-actor")

	if _, err := b.Ready(a, ""); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if call := lastCall(t, calls()); call.actor != "an-actor" {
		t.Errorf("the child's BD_ACTOR = %q, want %q — cmd.Env REPLACES the environment, it does not extend it", call.actor, "an-actor")
	}

	// And the rebuild is that one variable and nothing else: same length,
	// one BEADS_DIR, so neither a dropped entry nor a second copy (whose
	// winner is the child's own last-wins rule, not ours) can hide here.
	got, env := bdStoreEnv(os.Environ(), a), os.Environ()
	if len(got) != len(env) {
		t.Errorf("the rebuilt environment is %d entries against the process's %d — nothing but BEADS_DIR is this fix's to change", len(got), len(env))
	}
	var beads int
	for _, kv := range got {
		if strings.HasPrefix(kv, "BEADS_DIR=") {
			beads++
		}
	}
	if beads != 1 {
		t.Errorf("want exactly one BEADS_DIR in the child's environment, got %d", beads)
	}
}

// The aggregator's claim, which is where the defect was actually visible: a
// row's Dir is the repo that HELD it, not the directory the loop happened to
// be on. Two configured repos, an inherited binding naming the first, and
// the rows must line up with their stores.
func TestReadyAllLabelsRowsWithTheStoreThatHeldThem(t *testing.T) {
	back, _ := newTestBackend(t)
	b, _ := storeBindBd(t)
	one := storeBindRepo(t, "a-1")
	two := storeBindRepo(t, "c-1")
	scanConfig(t, back.App, one, two)
	t.Setenv("BEADS_DIR", filepath.Join(one, beadsDirName))

	issues, failed := b.ReadyAll(back.App, "")
	if len(failed) != 0 {
		t.Fatalf("neither repo should fail its scan: %v", failed)
	}
	held := map[string]string{"a-1": one, "c-1": two}
	if len(issues) != len(held) {
		t.Fatalf("want one row from each configured repo, got %d: %+v", len(issues), issues)
	}
	for _, is := range issues {
		want, ok := held[is.ID]
		if !ok {
			t.Errorf("row %q came from neither configured repo", is.ID)
			continue
		}
		if is.Dir != want {
			t.Errorf("row %q is held by %s and is labelled %s — a bead id does not authenticate its repo",
				is.ID, AbbrevHome(want), AbbrevHome(is.Dir))
		}
	}
}

// bdStoreEnv itself, at the boundary the arms above cannot reach through a
// child: the binding is REPLACED rather than appended to, however many
// copies the inherited environment carried — a child handed two would settle
// it by its own last-wins rule, which is not a rule this runner may borrow.
// Matched by exact name, so `BEADS_DIRS` and anything else merely sharing
// the prefix is left alone.
func TestBdStoreEnvReplacesEveryInheritedBinding(t *testing.T) {
	t.Parallel()
	a := storeBindRepo(t, "a-1")
	in := []string{"PATH=/usr/bin", "BEADS_DIR=/one", "BD_ACTOR=an-actor", "BEADS_DIR=/two", "BEADS_DIRS=/not-it"}
	got := bdStoreEnv(in, a)
	want := []string{"PATH=/usr/bin", "BD_ACTOR=an-actor", "BEADS_DIRS=/not-it", "BEADS_DIR=" + filepath.Join(a, beadsDirName)}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("bdStoreEnv:\n got %q\nwant %q", got, want)
	}
	// A directory with no store sheds the binding and keeps the rest.
	got = bdStoreEnv(in, t.TempDir())
	for _, kv := range got {
		if strings.HasPrefix(kv, "BEADS_DIR=") {
			t.Errorf("a directory with no store must shed the binding, got %q", kv)
		}
	}
	if len(got) != 3 {
		t.Errorf("want the other three entries kept, got %q", got)
	}
}

// A guard on the fake, not on the product: if the substrate model here ever
// stops preferring BEADS_DIR over the working directory, every arm above
// passes for the wrong reason. This is the old runner's exact shape — a
// chdir into one repo with the other's binding inherited — and it must still
// answer with the inherited store.
func TestStoreBindFakeModelsTheSubstratePrecedence(t *testing.T) {
	t.Parallel()
	b, _ := storeBindBd(t)
	a := storeBindRepo(t, "a-1")
	c := storeBindRepo(t, "c-1")

	cmd := exec.Command(b.Bin, "ready", "--json")
	cmd.Dir = c
	cmd.Env = append(os.Environ(), "BEADS_DIR="+filepath.Join(a, beadsDirName))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("fake bd: %v", err)
	}
	var issues []BdIssue
	if err := json.Unmarshal(out, &issues); err != nil {
		t.Fatalf("fake output %q: %v", out, err)
	}
	if len(issues) != 1 || issues[0].ID != "a-1" {
		t.Fatalf("cmd.Dir named %s and BEADS_DIR named %s: the fake must answer from the VARIABLE, got %v",
			AbbrevHome(c), AbbrevHome(a), storeBindIDs(issues))
	}
	// …and from the working directory when there is no variable, or the
	// arms above could not tell a shed binding from a broken fake.
	cmd = exec.Command(b.Bin, "ready", "--json")
	cmd.Dir = c
	cmd.Env = bdStoreEnv(os.Environ(), "") // beadsHome("") is ./.beads: nothing here
	if out, err = cmd.Output(); err != nil {
		t.Fatalf("fake bd: %v", err)
	}
	issues = nil
	if err := json.Unmarshal(out, &issues); err != nil {
		t.Fatalf("fake output %q: %v", out, err)
	}
	if len(issues) != 1 || issues[0].ID != "c-1" {
		t.Fatalf("with no BEADS_DIR the fake must answer from the working directory, got %v", storeBindIDs(issues))
	}
}
