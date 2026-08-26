package rhq

// QA suite for the rangerhq-yt1p identity guard (verifying its close,
// rangerhq-dv2r): a live workspace id is not this session's, because herdr
// re-issues ids across a server restart and a handoff.
//
// The fix's own tests (metaidentity_test.go) walk the incident's board — one
// stranger holding one stale meta's id. These are the boards beside it: the
// one where a SECOND workspace wears the session's name, and the one where
// the anchor is absent rather than wrong.
//
// The first of these pinned a filed bug (rangerhq-w4zp) with a t.Skip: it
// encoded the expected behavior and failed. The skip came off when the bead
// closed, and it now guards the fix.
//
// WHAT IS NOT PINNED HERE, because no test can settle it. Across a
// generation boundary a label MATCH is not proof of identity either: it is
// read as "nothing disproves it" and the session is listed over that
// workspace. herdr manufactures colliding labels for free — measured against
// herdr 0.8.0, `herdr workspace create --no-focus --cwd ~/src/posse` with
// no --label auto-labels the workspace `posse` — so a directory whose
// basename is a session name is enough. herdr offers no third anchor
// (rangerhq-yt1p weighed and rejected terminal_id and shell_pid: both are
// regenerated when a pane's terminal is rebuilt), so this is a residual of
// the anchors available, not a defect in the guard. It is recorded so the
// next reader does not mistake the label for an identity.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rangerhq-w4zp: RelaunchAgent's identity guard resolves the NAME and then
// types into the pane it read off the META, and those can name different
// workspaces. Resolve falls back to foreign workspaces by label, so an
// unrelated namesake — a workspace herdr auto-labelled after a directory —
// answers the guard while m.Pane still points at the stranger holding the
// re-issued id.
//
// The board: foo's id was re-issued to "zulu" across a restart, and some
// other workspace on this herdr happens to be labelled "foo". Sessions()
// correctly drops foo's meta as a stranger; the namesake then lists foreign
// under foo's name, the guard resolves IT, and the persona command goes into
// zulu's pane.
func TestRelaunchGuardIsWalkedByAnUnrelatedNamesake(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	mustCreate(t, b, NewSessionOpts{Name: "foo", Agent: "ranger"})
	m := metaOf(t, b, "foo")
	m.Launched = time.Now().Add(-time.Hour) // past the startup grace
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{
		{WorkspaceID: m.Workspace, Label: "zulu"}, // the stranger holding foo's old id
		{WorkspaceID: "w99", Label: "foo"},        // an unrelated namesake
	})
	// No agent anywhere: the namesake is a plain workspace, which is exactly
	// the board on which AgentTarget errors and RelaunchAgent goes on to type.
	os.Remove(filepath.Join(fake, "agents.json"))
	os.Remove(filepath.Join(fake, "calls.log"))

	wantLaunched := m.Launched
	relaunched, err := b.RelaunchAgent("foo", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// foo has no session of its own on this board, so the answer is "nothing
	// typed" — not merely "not into that pane". Asserting the whole absence
	// keeps a fix that types somewhere else from passing.
	if log := calls(t, fake); relaunched || strings.Contains(log, "pane run") {
		t.Errorf("relaunched=%v; a persona command reached a pane on a board where foo has no session — %s is zulu's, and the guard resolved w99:\n%s",
			relaunched, m.Pane, log)
	}
	if got := metaOf(t, b, "foo"); !got.Launched.Equal(wantLaunched) {
		t.Errorf("refusing the namesake rewrote launched: got %v want %v", got.Launched, wantLaunched)
	}
}

// The control the fix above must not break: with no namesake on the board,
// the guard already holds (metaidentity_test.go pins it from the other
// side). It is repeated here so a fix for rangerhq-w4zp that over-corrects —
// refusing every relaunch, say — is caught by this file rather than by the
// fleet.
func TestRelaunchStillRelaunchesItsOwnSession(t *testing.T) {
	idProbeSocket(t)
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	mustCreate(t, b, NewSessionOpts{Name: "foo", Agent: "ranger"})
	m := metaOf(t, b, "foo")
	m.Launched = time.Now().Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	// Same generation, the session's own workspace, no agent detected in it:
	// the persona's CLI died and this is what relaunch exists for.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: m.Workspace, Label: "foo"}})
	os.Remove(filepath.Join(fake, "agents.json"))
	os.Remove(filepath.Join(fake, "calls.log"))

	relaunched, err := b.RelaunchAgent("foo", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !relaunched {
		t.Fatal("refused to relaunch a session into its own live workspace")
	}
	if log := calls(t, fake); !strings.Contains(log, "pane run "+m.Pane) {
		t.Errorf("nothing typed into foo's own pane %s:\n%s", m.Pane, log)
	}
}

// The same control with the namesake sitting next to ours. Resolve prefers
// a non-foreign row, so this must still type into foo's pane — a namesake
// is a distractor, not a veto. The no-namesake control above would not
// catch a Resolve that started returning the foreign row first.
func TestRelaunchStillRelaunchesItsOwnSessionBesideANamesake(t *testing.T) {
	idProbeSocket(t)
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	mustCreate(t, b, NewSessionOpts{Name: "foo", Agent: "ranger"})
	m := metaOf(t, b, "foo")
	m.Launched = time.Now().Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	saveWSTo(t, fake, []fakeWS{
		{WorkspaceID: m.Workspace, Label: "foo"},
		{WorkspaceID: "w99", Label: "foo"},
	})
	os.Remove(filepath.Join(fake, "agents.json"))
	os.Remove(filepath.Join(fake, "calls.log"))

	relaunched, err := b.RelaunchAgent("foo", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !relaunched {
		t.Fatal("refused to relaunch ours because a namesake was also listed")
	}
	log := calls(t, fake)
	if !strings.Contains(log, "pane run "+m.Pane) {
		t.Errorf("nothing typed into foo's own pane %s:\n%s", m.Pane, log)
	}
	if strings.Contains(log, "pane run w99") {
		t.Errorf("typed into the namesake:\n%s", log)
	}
}

// The production caller of RelaunchAgent is dispatch's launchSession
// (rangerhq-vk2). On the namesake board it must not type a persona command
// into zulu either: RelaunchAgent refuses, then awaitAgent fails looking
// at the namesake. Creating a replacement is Resolve's foreign-by-label
// fallback, not this path — launchSession treats a resolved row as "the
// session exists".
func TestDispatchRelaunchOnNamesakeBoardDoesNotTypeIntoZulu(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	mustCreate(t, b, NewSessionOpts{Name: "foo", Agent: "ranger"})
	m := metaOf(t, b, "foo")
	m.Launched = time.Now().Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{
		{WorkspaceID: m.Workspace, Label: "zulu"},
		{WorkspaceID: "w99", Label: "foo"},
	})
	os.Remove(filepath.Join(fake, "agents.json"))
	os.Remove(filepath.Join(fake, "calls.log"))

	d := newTestDispatcher(t, b)
	d.StartupWait = 50 * time.Millisecond
	d.Poll = 5 * time.Millisecond
	_, err := d.launchSession(RepoIssue{Dir: t.TempDir(), BdIssue: BdIssue{ID: "b-1"}}, "ranger", "foo", "", "fast", nil)
	log := calls(t, fake)
	if strings.Contains(log, "pane run") {
		t.Errorf("dispatch typed a persona command on the namesake board — %s is zulu's:\n%s", m.Pane, log)
	}
	if strings.Contains(log, "workspace create") {
		t.Errorf("dispatch created a replacement; launchSession resolved the namesake as existing:\n%s", log)
	}
	if err == nil {
		t.Fatal("expected await-agent failure after the relaunch refuse, got success")
	}
}

// An empty label is read as "not disproven", so a workspace with no label
// holding a re-issued id IS listed under our name. That is deliberate
// (rangerhq-yt1p: positive evidence only, so a herdr release that stopped
// reporting labels could not turn the whole fleet into strangers in one
// step), and this test pins the decision rather than reporting it — a change
// here should be a decision, not a drift.
//
// Reachability, measured against herdr 0.8.0 rather than assumed: OMITTING
// --label does not reach it. herdr auto-labels an unlabelled workspace
// `basename(cwd)`, so the ordinary `herdr workspace create` yields a
// non-empty label. Only an explicit `herdr workspace create --label ”`
// produces `label: ""`, which it does accept. So the board exists but nobody
// walks onto it by accident.
func TestAnUnlabelledWorkspaceIsNotEvidenceOfAStranger(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "foo", Cmd: "claude"})
	ws := metaOf(t, b, "foo").Workspace

	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: ws, Label: ""}})

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	var listed bool
	for _, s := range sessions {
		if s.Name == "foo" && s.WorkspaceID == ws {
			listed = true
		}
	}
	if !listed {
		t.Errorf("an unlabelled workspace is now read as a stranger — positive evidence only was a decision (rangerhq-yt1p); if it changed, change it deliberately: %+v", sessions)
	}
}
