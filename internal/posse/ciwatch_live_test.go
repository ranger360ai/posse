package posse

// Live pin for ci-watch (ranger-base-x9e34, ciwatch.go), run against the
// real bd and a real `gh`-shaped child rather than against the fakes:
//
//	RHQ_LIVE_BD=1 go test ./internal/posse/ -run TestLiveCIWatchFiresOnceAndClears -v
//
// The bead asks for exactly this and says why: "turn main red on purpose in
// a scratch clone and watch the mechanism fire once, then green it and watch
// it clear." The fakes model bd, and the fake is what got the last one of
// these wrong (settleescalation_live_test.go's header). What only real bd
// can answer here:
//
//   - a bead filed with `-l ci-red,devops` really does come back out of
//     `bd list --label-any ci-red` — the dedupe is one query and a store
//     that answered it differently would file per pass;
//   - a bead a PERSONA closed really does leave that query, which is what
//     stops ci-watch adopting an answered bead forever (ci-watch itself does
//     not close: ADR 0013 §4, ciwatch.go's header);
//   - a comment ci-watch wrote on an earlier pass really does come back out
//     of `bd comments` — the drumbeat's cadence and the whole green half's
//     state are read back out of that one query;
//   - the description survives bd's own round trip with the marker line
//     intact, newlines and all — the marker IS the dedupe.
//
// The gh side is a real child process too: a two-line shell script serving a
// runs.json this test rewrites between passes, exec'd through the shipped
// ghRunList with the shipped argv. Turning the gate red and green is one
// file write, which is the "scratch clone" the bead asks for with no clone
// and no workflow run.
//
// Env-gated and skipped by default, like every other live pin here: it
// shells out to the operator's bd, which has a version and a daemon, neither
// of which belongs in a hermetic suite. Everything happens inside t.TempDir
// — the `bd init` is the throwaway-database case, never a repo anybody
// keeps, and nothing here can see the shop's own store.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveCIWatchFiresOnceAndClears(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	bdbin, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("no bd on PATH")
	}

	repo := t.TempDir()
	// Bd.run execs Bin directly, so --no-daemon rides in a wrapper rather
	// than in the argv the code under test builds (settleescalation's rule):
	// a daemon per throwaway db is a leak, and the point here is the store.
	wrapper := filepath.Join(t.TempDir(), "bd-nodaemon")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+bdbin+" --no-daemon \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sh := func(args ...string) (string, error) {
		cmd := exec.Command(wrapper, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := sh("init", "--prefix", "ciw"); err != nil {
		t.Skipf("bd init did not take in a throwaway repo: %v %s", err, out)
	}

	// The repo ReadCI will accept, and a gh that answers out of a file this
	// test rewrites — the gate, turned by hand.
	gitRun := func(args ...string) {
		t.Helper()
		if _, err := git(repo, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	if _, err := git(repo, "rev-parse", "--git-dir"); err != nil {
		gitRun("init", "-b", "main")
	}
	gitRun("remote", "add", "origin", "https://github.com/ranger360ai/posse.git")
	if err := os.MkdirAll(filepath.Join(repo, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".github", "workflows", "ci.yml"), []byte("name: ci\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ghdir := t.TempDir()
	runsPath := filepath.Join(ghdir, "runs.json")
	ghPath := filepath.Join(ghdir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nexec cat "+runsPath+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-2 * time.Hour)
	setGate := func(rows ...string) {
		t.Helper()
		if err := os.WriteFile(runsPath, []byte("["+strings.Join(rows, ",")+"]"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, _ := newTestBackend(t)
	a := b.App
	if err := os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The SHIPPED reading, with only the binary substituted — this is what
	// makes the gh half of this end to end rather than a stub.
	a.CIRead = func(q CIQuery) CIState {
		q.GhBin = ghPath
		return ReadCI(q)
	}
	bd := Bd{Bin: wrapper}
	pass := func() (int, string, string) {
		t.Helper()
		var out, errb strings.Builder
		n := a.CIWatch(bd, a.BeadsDirs(), &out, &errb)
		return n, out.String(), errb.String()
	}
	openCiRed := func() []BdIssue {
		t.Helper()
		is, err := bd.OpenLabeledAny(repo, CIRedLabel)
		if err != nil {
			t.Fatalf("bd list --label-any %s: %v", CIRedLabel, err)
		}
		return is
	}

	// ── green to start: nothing to say, nothing filed ────────────────────
	setGate(ciRunJSON("aaaaaaaa", "completed", "success", at))
	if n, out, errs := pass(); n != 0 || cwSay(out) != "" {
		t.Fatalf("a green gate acted %d and said %q (%s)", n, cwSay(out), errs)
	}
	if is := openCiRed(); len(is) != 0 {
		t.Fatalf("a green gate filed %d beads", len(is))
	}

	// ── red: one bead, and it stays one across four more passes ──────────
	setGate(
		ciRunJSON("bbbbbbbb", "completed", "failure", at.Add(20*time.Minute)),
		ciRunJSON("aaaaaaaa", "completed", "success", at),
	)
	n, out, errs := pass()
	if n != 1 {
		t.Fatalf("the red gate acted %d, want 1 (out %q err %q)", n, out, errs)
	}
	if !strings.Contains(out, "ci red ·") {
		t.Errorf("the filing pass said %q", out)
	}
	first := openCiRed()
	if len(first) != 1 {
		t.Fatalf("real bd answers --label-any %s with %d beads, want 1", CIRedLabel, len(first))
	}
	id := first[0].ID
	if !strings.Contains(first[0].Description, ciMarkerPrefix) {
		t.Errorf("the marker did not survive bd's round trip:\n%s", first[0].Description)
	}
	for i := 0; i < 4; i++ {
		if n, out, _ := pass(); n != 0 || cwSay(out) != "" {
			t.Fatalf("pass %d over the SAME red acted %d and said %q — one bead per episode, not per pass", i, n, cwSay(out))
		}
	}
	if is := openCiRed(); len(is) != 1 || is[0].ID != id {
		t.Fatalf("after five red passes the store holds %d ci-red beads", len(is))
	}

	// ── the drumbeat, against real bd's own comments ─────────────────────
	setGate(
		ciRunJSON("dddddddd", "completed", "failure", at.Add(80*time.Minute)),
		ciRunJSON("cccccccc", "completed", "failure", at.Add(50*time.Minute)),
		ciRunJSON("bbbbbbbb", "completed", "failure", at.Add(20*time.Minute)),
		ciRunJSON("aaaaaaaa", "completed", "success", at),
	)
	pass()
	cs, err := bd.Comments(repo, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 {
		t.Fatalf("streak 1 -> 3 earned %d comments, want 1", len(cs))
	}
	pass()
	if cs, _ := bd.Comments(repo, id); len(cs) != 1 {
		t.Errorf("a pass at the SAME streak commented again: %d comments", len(cs))
	}

	// ── green again: said on the bead, and NOT closed ────────────────────
	setGate(ciRunJSON("eeeeeeee", "completed", "success", at.Add(2*time.Hour)))
	n, out, errs = pass()
	if n != 1 {
		t.Fatalf("the green gate acted %d, want 1 (out %q err %q)", n, out, errs)
	}
	if !strings.Contains(out, "ci green ·") || !strings.Contains(out, "eeeeeeee") {
		t.Errorf("the clearing pass said %q", out)
	}
	shown, err := bd.Show(repo, id)
	if err != nil {
		t.Fatal(err)
	}
	if shown.Status == "closed" {
		t.Fatalf("ci-watch CLOSED %s — ADR 0013 §4 makes that the persona's", id)
	}
	if cs, _ := bd.Comments(repo, id); !ciAlreadyCleared(cs) {
		t.Fatalf("real bd does not answer with the clearing comment, so the next red would file over it")
	}
	if n, out, _ := pass(); n != 0 || cwSay(out) != "" {
		t.Errorf("a second green pass acted %d and said %q", n, cwSay(out))
	}

	// ── the persona's close, and the dedupe blind to it ──────────────────
	// The dedupe must step over a bead a persona closed. Whether `bd list
	// --label-any` still ANSWERS with it is the store class's business:
	// measured 2026-09-04, the shop's SQLite store drops closed rows here
	// and the `no-db: true` JSONL store `bd init` writes on bd 0.50.3 —
	// which is the store THIS rig has — keeps them. This is the arm that
	// found it, so it is also the live pin of the fix: OpenLabeledAny drops
	// them itself (ranger-base-bwrp8), which is asserted here against real
	// bd on the store class that does not.
	if out, err := sh("close", id, "-r", "the gate cleared itself"); err != nil {
		t.Fatalf("bd close %s: %v %s", id, err, out)
	}
	if is := openCiRed(); len(is) != 0 {
		t.Fatalf("OpenLabeledAny answered with %d bead(s) after the only one was closed (%s) — "+
			"open is this query's promise, not the store's", len(is), is[0].Status)
	}
	if is := ciOpenAdopted(t, bd, repo); is != nil {
		t.Fatalf("the dedupe adopted %s (%s) after a persona closed it — the next red would never be filed", is.ID, is.Status)
	}

	// ── and a NEW red is a NEW bead ──────────────────────────────────────
	setGate(
		ciRunJSON("ffffffff", "completed", "failure", at.Add(3*time.Hour)),
		ciRunJSON("eeeeeeee", "completed", "success", at.Add(2*time.Hour)),
	)
	if n, _, errs := pass(); n != 1 {
		t.Fatalf("the second red episode acted %d, want 1 (%s)", n, errs)
	}
	second := ciOpenAdopted(t, bd, repo)
	if second == nil || second.ID == id {
		t.Fatalf("the second episode did not get its own bead (adopted %v, first=%s)", second, id)
	}
}

// ciOpenAdopted is the bead ci-watch's own dedupe would adopt right now, or
// nil — the shipped ciOpenBeads, asked the same question the pass asks.
func ciOpenAdopted(t *testing.T, bd Bd, repo string) *BdIssue {
	t.Helper()
	st := CIState{Slug: "ranger360ai/posse", Workflow: "ci.yml", Branch: "main"}
	is, err := ciOpenBeads(bd, repo, st)
	if err != nil {
		t.Fatalf("ciOpenBeads: %v", err)
	}
	if len(is) == 0 {
		return nil
	}
	return &is[0]
}
