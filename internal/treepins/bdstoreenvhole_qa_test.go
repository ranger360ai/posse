package treepins

// LIVE-DEFECT PIN for ranger-base-751ha (verifying ranger-base-9mjxb's close).
//
// THE HOLE. ranger-base-9mjxb's answer to github issue 9 was: stop handing bd
// a store-selecting flag, name the repo that OWNS the store and chdir into it,
// and then there is no second repository in the question. The audit, the census
// pin (TestQANoShippedPosseCallHandsBdAStoreSelectingFlag) and the ORDERS
// bullet are all right about the flag. The fix is defeated by an environment
// variable the same bullet names: a chdir does not bind bd's store while
// `BEADS_DIR` is set, and posse sets `BEADS_DIR` in EVERY session it launches
// (ADR 0055, AGENTS.md "Landing the plane"). That bullet's own prescription is
// `env -u BEADS_DIR bd <...>`; scripts/prune-bd-relates-to.sh's three writes do
// not shed it.
//
// MEASURED 2026-10-09, darwin 25.4.0, bd 0.50.3 pinned, two scratch git repos
// each holding a touched copy of a real beads.db, controls first:
//
//	cwd=repoA, BEADS_DIR shed               -> repoA/.beads          (control)
//	cwd=repoB, BEADS_DIR shed               -> repoB/.beads          (control)
//	cwd=repoA, BEADS_DIR=repoB/.beads       -> repoB/.beads          (BEADS_DIR WINS)
//
// and the write, not just the resolution: `bd --no-daemon comments add` run
// from inside repoA with BEADS_DIR naming repoB left repoA at 0 marker rows and
// put 1 in repoB. The same thing bdStoreEnvShed (internal/posse/beads.go) was
// written for a month earlier and MEASURED then — which is why the Go runner is
// clean and this script is not.
//
// IT IS LOUD, WHICH IS THE ONE MERCY: the script's own post-apply re-read of
// the store BEADS_DB names still finds the pairs, so it exits 1 saying "pairs
// remain after --apply". The refusal arrives AFTER the three writes have landed
// in the other repository, which is the cross-repository write the bead is
// about.
//
// THIS PIN SHIPS GREEN, asserting the hole. When the script sheds the variable
// — `env -u BEADS_DIR bd --no-daemon …`, or a `BEADS_DIR=` rebound to the
// owner's own store beside the chdir — both arms below red and say so in their
// failure message. That is the signal to delete this file and let
// TestQAPruneRelatesToAppliesFromTheStoresOwnRepo carry the behaviour, with
// `BEADS_DIR` added to its fixture environment.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Arm 1, the source reading: the script runs bd without shedding the one
// environment variable that outranks its chdir.
func TestQAPruneRelatesToStillInheritsBeadsDir(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"
	b, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, `(cd "$OWNER" && bd --no-daemon`) {
		t.Fatalf("CONTROL: %s no longer runs bd as `(cd \"$OWNER\" && bd --no-daemon …)`, so this pin is aimed at a line that moved — re-read the script", script)
	}
	if strings.Contains(body, "BEADS_DIR") {
		t.Errorf("HOLE CLOSED: %s now names BEADS_DIR. If it sheds it at the bd calls, delete this file and add BEADS_DIR to TestQAPruneRelatesToAppliesFromTheStoresOwnRepo's fixture env instead — that pin should own the behaviour.", script)
	}
}

// Arm 2, the behaviour: with BEADS_DIR naming another repo's store, all three
// writes land there and the store BEADS_DB names keeps its pairs.
//
// The only difference from TestQAPruneRelatesToAppliesFromTheStoresOwnRepo is
// the fake bd, which resolves BEADS_DIR first and the cwd second — what the
// real bd 0.50.3 does, MEASURED above. That pin's fake reads `$PWD/.beads`
// only, and `prunRun` passes the runner's own BEADS_DIR straight through, so
// it cannot see this.
func TestQAPruneRelatesToWritesTheStoreBeadsDirNames(t *testing.T) {
	const script = "scripts/prune-bd-relates-to.sh"
	store := prunRepo(t, prunPair)
	elsewhere := prunRepo(t, prunPair)

	env := []string{
		"BEADS_DB=" + filepath.Join(store, ".beads", "beads.db"),
		// Exactly what posse puts in every session's environment (ADR 0055).
		"BEADS_DIR=" + filepath.Join(elsewhere, ".beads"),
	}
	log := prunFakeBdHonoringBeadsDir(t, &env)

	out, code := prunRun(t, script, elsewhere, env, "--apply")
	calls := prunCalls(t, log)
	if len(calls) != 3 {
		t.Fatalf("CONTROL: want the 3 bd calls --apply makes, got %d — the rig is talking, not the subject:\n%s\n--- script output:\n%s",
			len(calls), strings.Join(calls, "\n"), out)
	}
	// CONTROL: the chdir itself is right, which is what makes this a hole in
	// the fix rather than an absence of one.
	for _, c := range calls {
		if !strings.Contains(c, "cwd="+mustEval(t, store)) {
			t.Fatalf("CONTROL: the chdir to the owning repo is gone, so this pin is measuring something else: %q", c)
		}
	}

	if got := prunRelatesRows(t, store); got != "2" {
		t.Errorf("HOLE CLOSED: the store BEADS_DB names now holds %s relates-to row(s) instead of 2 — the writes reached the repo the chdir named, so the script sheds or rebinds BEADS_DIR. Delete this file; TestQAPruneRelatesToAppliesFromTheStoresOwnRepo owns the behaviour, with BEADS_DIR in its fixture env.", got)
	}
	if got := prunRelatesRows(t, elsewhere); got != "0" {
		t.Errorf("HOLE CLOSED: the other repository's store kept %s relates-to row(s) of 2 — it is no longer being written across repositories. Delete this file (see above).", got)
	}
	// The script does say so, after the fact. If that stops being true the
	// writes have gone silent, which is strictly worse than the hole.
	if !strings.Contains(out, "pairs remain after --apply") || code == 0 {
		t.Errorf("the misdirected prune must at least still be LOUD — the post-apply re-read is the only thing that reports it: exit %d\n%s", code, out)
	}
}

// Same shape as prunFakeBd, with the one difference that matters: store
// resolution is BEADS_DIR first, then $PWD/.beads.
func prunFakeBdHonoringBeadsDir(t *testing.T, env *[]string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	body := "#!/bin/sh\n" +
		"store=${BEADS_DIR:-$PWD/.beads}\n" +
		"printf 'cwd=%s store=%s argv=%s\\n' \"$(pwd -P)\" \"$store\" \"$*\" >> " + shQuote(log) + "\n" +
		"for a in \"$@\"; do\n" +
		"\tif [ \"$a\" = unrelate ]; then\n" +
		"\t\tsqlite3 \"$store/beads.db\" \"DELETE FROM dependencies WHERE type='relates-to';\" || exit 1\n" +
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

// mustEval is the physical spelling of p, because the fake bd logs `pwd -P`
// and every temp dir here sits under a symlinked /var.
func mustEval(t *testing.T, p string) string {
	t.Helper()
	q, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
