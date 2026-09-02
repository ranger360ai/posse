package posse

// rangerhq-lvx9 — verifying rangerhq-9jk1. The delivered pins walk the
// record-survives / advised-retry / unmatched-close boards and the two
// halves in isolation. They do not walk the board where Resolve falls
// back to the stranger (ours hidden, stranger listed and inhabited), so
// a landing turn typed at wForeign would not have failed
// TestRelaunchRefusesAWorkspaceAlreadyWearingTheName: on that test the
// session is listed and Resolve prefers ours even without the preflight.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lvx9Board(t *testing.T) (*HerdrBackend, string, string) {
	t.Helper()
	b, fake, ws := i2g9Board(t)
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "wForeign", Label: "s1"}))
	hideFromTheListing(t, fake, ws)
	// A live agent in the stranger, so a misaimed landing turn is visible
	// in the call log as `agent prompt wForeign:p1`.
	if err := os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"wForeign:p1","workspace_id":"wForeign"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fake, "calls.log"))
	return b, fake, ws
}

func TestQA9jk1DoesNotLandTheStranger(t *testing.T) {
	b, fake, ws := lvx9Board(t)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("expected a refusal:\n%s", out.String())
	}
	log := calls(t, fake)
	if strings.Contains(log, "agent prompt") {
		t.Errorf("landing turn addressed someone on the 9jk1 board:\n%s", log)
	}
	if strings.Contains(log, "workspace close") {
		t.Errorf("closed a workspace on a preflight refusal:\n%s", log)
	}
	if metaOf(t, b, "s1").Workspace != ws {
		t.Errorf("record moved: want %s", ws)
	}
	if alive, aerr := b.H.WorkspaceAlive(ws); aerr != nil || !alive {
		t.Errorf("live session closed (alive=%v err=%v)", alive, aerr)
	}
	if alive, aerr := b.H.WorkspaceAlive("wForeign"); aerr != nil || !alive {
		t.Errorf("stranger closed (alive=%v err=%v)", alive, aerr)
	}
}

// The close's measured refusal: names the stranger, says nothing was closed,
// and does not advise the retry that used to finish the orphaning
// ("its recipe was kept … retry with: posse relaunch s1") nor `posse attach
// s1`, which Resolve would send to the stranger.
func TestQA9jk1RefusalNamesTheStrangerAndDoesNotAdviseABlindRetry(t *testing.T) {
	b, _, ws := lvx9Board(t)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatal("expected a refusal")
	}
	msg := err.Error()
	for _, want := range []string{"s1", "wForeign", "NOT closed", "labelled"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal missing %q: %v", want, err)
		}
	}
	if strings.Contains(msg, "recipe was kept") {
		t.Errorf("preflight talked like a post-kill rollback: %v", err)
	}
	if strings.Contains(msg, "retry with:") && strings.Contains(msg, "posse relaunch s1") {
		t.Errorf("preflight advertised the retry that used to orphan: %v", err)
	}
	if strings.Contains(msg, "posse attach s1") {
		t.Errorf("preflight sent the operator to attach, which resolves to the stranger: %v", err)
	}
	if metaOf(t, b, "s1").Workspace != ws {
		t.Errorf("record blanked behind the refusal")
	}
}

// Follow the advice the refusal prints: close the stranger, then relaunch.
// The session's own workspace is still missing from this snapshot, so the
// retry must refuse (i2g9), not take the name and leave w2 running unnamed.
func TestQA9jk1AdvisedCleanupDoesNotOrphanWhileTheSnapshotIsStale(t *testing.T) {
	b, fake, ws := lvx9Board(t)

	var out strings.Builder
	_ = b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})

	var kept []fakeWS
	for _, w := range fakeLoadWSFrom(t, fake) {
		if w.WorkspaceID != "wForeign" {
			kept = append(kept, w)
		}
	}
	saveWSTo(t, fake, kept)
	os.Remove(filepath.Join(fake, "calls.log"))
	out.Reset()
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	m := metaOf(t, b, "s1")
	alive, aerr := b.H.WorkspaceAlive(ws)
	if aerr != nil {
		t.Fatal(aerr)
	}
	if alive && m.Workspace != ws {
		t.Fatalf("following the advice orphaned the live session: meta=%s live=%s err=%v\n%s",
			m.Workspace, ws, err, out.String())
	}
	if err == nil {
		t.Fatalf("retry over a still-hidden live workspace succeeded:\n%s", out.String())
	}
}

// --no-land is the operator's override of the landing turn, not of v52t.
func TestQA9jk1NoLandStillRefusesAWornName(t *testing.T) {
	b, fake, ws := lvx9Board(t)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1", NoLand: true})
	if err == nil {
		t.Fatalf("--no-land walked into a worn name:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "NOT closed") {
		t.Errorf("expected a preflight refusal: %v", err)
	}
	if strings.Contains(calls(t, fake), "workspace close") {
		t.Errorf("--no-land closed something:\n%s", calls(t, fake))
	}
	if metaOf(t, b, "s1").Workspace != ws {
		t.Errorf("record moved under --no-land")
	}
}

func TestQA9jk1EmptyLabelIsNotAWornName(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})
	mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
	m1 := metaOf(t, b, "s1")
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "wEmpty", Label: ""}))
	os.Remove(filepath.Join(fake, "calls.log"))

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("empty label refused a relaunch: %v\n%s", err, out.String())
	}
	if m2 := metaOf(t, b, "s1"); m2.Workspace == "" || m2.Workspace == m1.Workspace {
		t.Errorf("session was not rebuilt: %+v", m2)
	}
	if !strings.Contains(calls(t, fake), "workspace close "+m1.Workspace) {
		t.Errorf("ordinary close missed:\n%s", calls(t, fake))
	}
}

func TestQA9jk1TwoStrangersStillRefuse(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})
	mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
	m1 := metaOf(t, b, "s1")
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "wF1", Label: "s1"},
		fakeWS{WorkspaceID: "wF2", Label: "s1"},
	))

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("two namesakes were not a refusal:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "wF1") && !strings.Contains(err.Error(), "wF2") {
		t.Errorf("refusal named neither stranger: %v", err)
	}
	if strings.Contains(calls(t, fake), "workspace close") {
		t.Errorf("closed something:\n%s", calls(t, fake))
	}
	if metaOf(t, b, "s1").Workspace != m1.Workspace {
		t.Errorf("record moved")
	}
}

// A kept recipe (workspace blank) still refuses if a stranger wears the name:
// the retry the ordinary rollback advertises must not walk into v52t.
func TestQAKeptRecipeRefusesWhileAStrangerWearsTheName(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})
	mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
	m := metaOf(t, b, "s1")
	if err := b.KillSession("s1"); err != nil {
		t.Fatal(err)
	}
	if kept := b.keepRecipe(m, nil); kept != "" {
		t.Fatalf("keepRecipe after a real kill must write, kept %q", kept)
	}
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "wForeign", Label: "s1"}))
	os.Remove(filepath.Join(fake, "calls.log"))

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("kept-recipe retry walked into a worn name:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "wForeign") {
		t.Errorf("refusal did not name the stranger: %v", err)
	}
	if strings.Contains(calls(t, fake), "workspace close") {
		t.Errorf("closed the stranger from a kept recipe:\n%s", calls(t, fake))
	}
	if metaOf(t, b, "s1").Workspace != "" {
		t.Errorf("kept recipe gained a workspace")
	}
}
