package posse

// Live pin for ranger-base-23oo, run against the real bd rather than the
// fake — because the fake is what got this wrong.
//
//	RHQ_LIVE_BD=1 go test ./internal/posse/ -run TestLiveSettleEscalationBlocksTheStuckBead -v
//
// The settle-open escalation has to do two things bd 0.49.1 will not do
// together: record where the question came from, and BLOCK the bead it came
// from. bd's cycle check spans every dependency type, not only `blocks`, so a
// question carrying `discovered-from:<stuck>` makes `dep add <stuck> <qid>` a
// cycle and bd refuses it, exit 1 — and the stuck bead stays in `bd ready`
// and `--resume` re-prompts it forever, which is the loop the whole rung
// exists to stop. Ten green pins missed that, because herdr_test.go's fake bd
// granted every `dep add` and ignored `--deps` outright.
//
// So this drives escalateSettleOpen itself — the shipped function, its real
// argv — against real bd, and asserts the STOP: the edge is in the graph and
// the bead is out of `bd ready`.
//
// The second arm is what makes the first one mean something. It files the
// same escalation WITH the `discovered-from` edge, the way the code did
// before this bead, and requires bd to refuse the block. A pin whose wrong
// arm also passes has measured nothing.
//
// Env-gated and skipped by default, like the other live pins: it shells out
// to the operator's bd, which has a version and a daemon, neither of which
// belongs in a hermetic suite. Everything happens inside one t.TempDir — the
// `bd init` here is the throwaway-database case, never a repo anybody keeps.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveSettleEscalationBlocksTheStuckBead(t *testing.T) {
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	bin, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("no bd on PATH")
	}

	repo := t.TempDir()
	// Bd.run execs Bin directly, so --no-daemon rides in a wrapper rather
	// than in the argv the code under test builds: a daemon per throwaway db
	// is the leak ranger-base's daemon note is about, and the point here is
	// the graph, not the transport.
	wrapper := filepath.Join(t.TempDir(), "bd-nodaemon")
	script := "#!/bin/sh\nexec " + bin + " --no-daemon \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	sh := func(args ...string) (string, error) {
		cmd := exec.Command(wrapper, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := sh("init", "--prefix", "esc"); err != nil {
		t.Skipf("bd init did not take in a throwaway repo: %v %s", err, out)
	}

	b, _ := newTestBackend(t)
	d, errb := settleDispatcher(t, b)
	d.Bd = Bd{Bin: wrapper}

	create := func(title string) string {
		id, err := d.Bd.Create(repo, BdNew{Title: title, Description: "x", Priority: "2"})
		if err != nil {
			t.Fatalf("create %q: %v", title, err)
		}
		return id
	}
	inReady := func(id string) bool {
		out, err := sh("ready", "--json", "--limit", "0")
		if err != nil {
			t.Fatalf("bd ready: %v\n%s", err, out)
		}
		return strings.Contains(out, `"`+id+`"`)
	}
	blockedBy := func(id string) []string {
		deps, err := d.Bd.DepList(repo, id)
		if err != nil {
			t.Fatalf("dep list %s: %v", id, err)
		}
		var ids []string
		for _, dep := range deps {
			ids = append(ids, dep.ID)
		}
		return ids
	}

	// ─── the pin: the shipped rung, end to end, against real bd ──────────
	stuck := create("PROBE stuck")
	if !inReady(stuck) {
		t.Fatalf("%s is not in bd ready before the escalation — the fixture says nothing", stuck)
	}
	p := &pendingBead{
		is:      RepoIssue{BdIssue: BdIssue{ID: stuck, Title: "the thing"}, Dir: repo},
		persona: "ranger",
		session: "ranger-posse-" + stuck,
	}
	d.escalateSettleOpen(p, "idle", "in_progress")

	qs, err := d.Bd.OpenLabeledAny(repo, SettleQuestionLabel)
	if err != nil {
		t.Fatalf("listing the questions: %v", err)
	}
	var qid, qdesc string
	for _, q := range qs {
		if settleStuckSource(q.Title) == stuck {
			qid, qdesc = q.ID, q.Description
		}
	}
	if qid == "" {
		t.Fatalf("no escalation was filed for %s: %v", stuck, qs)
	}
	if deps := blockedBy(stuck); len(deps) != 1 || deps[0] != qid {
		t.Fatalf("THE STOP DID NOT LAND: %s is blocked by %v, want [%s]\nstderr: %s", stuck, deps, qid, errb)
	}
	if inReady(stuck) {
		t.Fatalf("%s is STILL in bd ready — --resume re-prompts it forever\nstderr: %s", stuck, errb)
	}
	if !strings.Contains(qdesc, discoveredFromMarkerPrefix+stuck) {
		t.Errorf("the escalation body does not carry the provenance the edge cannot:\n%s", qdesc)
	}
	if errb.String() != "" {
		t.Errorf("unexpected stderr from a rung that worked: %s", errb)
	}

	// ─── the wrong arm: with the edge, the stop does not land ────────────
	// This is the shape that shipped before ranger-base-23oo, and it MUST
	// still fail, or the arm above proves nothing about why the edge was
	// dropped. What it must fail AT is the observable — the trigger stays
	// in `bd ready`, which is the loop --resume then re-prompts forever —
	// and NOT the mechanism, which is a property of the bd on the box:
	//
	//   0.49.1 refuses the `dep add` outright ("would create a cycle").
	//   0.50.3 ACCEPTS it and answers `bd ready` with the bead anyway —
	//   MEASURED 2026-09-01 (laurie, ranger-base-coxn8, filed as
	//   ranger-base-lpz0o): the same bead is in `bd ready` and in
	//   `bd blocked` at once, over three consecutive reads.
	//
	// Pinning the refusal made this arm red the hour the box's bd moved,
	// while the defect it guards was untouched — the same shape re-wired
	// into escalateSettleOpen still leaves the trigger in ready on 0.50.3.
	// So: assert the loop, log the mechanism, and fire if some future bd
	// honors the block, because then the whole rung can be simpler.
	other := create("PROBE stuck (control)")
	qWithEdge, err := d.Bd.Create(repo, BdNew{
		Title:       settleStuckTitle(other, "ranger", "in_progress"),
		Labels:      []string{SettleQuestionLabel},
		Deps:        []string{"discovered-from:" + other},
		Priority:    "1",
		Description: "y",
	})
	if err != nil {
		t.Fatalf("create with the discovered-from edge: %v", err)
	}
	switch addErr := d.Bd.DepAdd(repo, other, qWithEdge, VerifyActor); {
	case addErr == nil:
		t.Logf("this bd ACCEPTED dep add %s %s against a discovered-from edge (0.50.3 shape, ranger-base-lpz0o) — the block is silent, not refused", other, qWithEdge)
	case strings.Contains(addErr.Error(), "would create a cycle"):
		t.Logf("this bd refused the block outright (0.49.1 shape): %v", addErr)
	default:
		t.Errorf("bd refused the block for some other reason: %v", addErr)
	}
	if !inReady(other) {
		t.Errorf("THE PRE-FIX SHAPE STOPPED THE LOOP: %s left bd ready even carrying the discovered-from edge — this bd honors the block through a cycle, so ranger-base-23oo's premise is stale and the rung can be re-decided rather than trusted", other)
	}
}
