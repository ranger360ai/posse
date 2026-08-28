package rhq

// ADR 0008 (bead rangerhq-b3p): a crew session is one the operator talks
// to, and dispatch treats it as if it did not exist. These tests pin the
// three halves of that: who sets the mark, who clears it, and what a pass
// does when it meets one.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The mark is set by origin and by the operator's hands, never by a clock:
// `posse new` hands a session over, dispatch's own create does not, the
// toggle goes both ways, and a refresh is not a change of hands.
func TestCrewMarkerLifecycle(t *testing.T) {
	b, _ := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "mine", Crew: true})
	mustCreate(t, b, NewSessionOpts{Name: "fleet"})

	if m, ok := b.readMeta("mine"); !ok || !m.Crew {
		t.Errorf("posse new must mark crew: %+v", m)
	}
	if m, ok := b.readMeta("fleet"); !ok || m.Crew {
		t.Errorf("dispatch's create must not mark crew: %+v", m)
	}

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	crew := map[string]bool{}
	for _, s := range sessions {
		crew[s.Name] = s.Crew
	}
	if !crew["mine"] || crew["fleet"] {
		t.Errorf("the mark must ride the session list: %+v", crew)
	}

	// `posse list` tags the crew session, and only it.
	var out strings.Builder
	if err := b.CmdList(&out); err != nil {
		t.Fatal(err)
	}
	for _, ln := range strings.Split(out.String(), "\n") {
		if strings.Contains(ln, "mine") && !strings.Contains(ln, CrewTag) {
			t.Errorf("crew session untagged in posse list: %q", ln)
		}
		if strings.Contains(ln, "fleet") && strings.Contains(ln, CrewTag) {
			t.Errorf("fleet session wearing the crew tag: %q", ln)
		}
	}

	// The toggle: `posse crew --off` releases, `posse crew` takes.
	if err := b.SetCrew("mine", false); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.readMeta("mine"); m.Crew {
		t.Error("--off did not release the session")
	}
	if err := b.SetCrew("fleet", true); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.readMeta("fleet"); !m.Crew {
		t.Error("toggle did not take the session")
	}
	if err := b.SetCrew("nope", true); err == nil {
		t.Error("marking a session that does not exist must fail")
	}

	// Relaunch recreates from the meta: the operator's session is still
	// theirs on the other side of a refresh.
	var w strings.Builder
	if err := b.RelaunchSession(&w, RelaunchOpts{Name: "fleet", NoLand: true}); err != nil {
		t.Fatal(err)
	}
	if m, ok := b.readMeta("fleet"); !ok || !m.Crew {
		t.Errorf("crew mark lost across relaunch: %+v", m)
	}
}

// `posse prompt` marks the session only when the operator is the one
// prompting: a persona's session carries RHQ_PERSONA, so a coordinator handing
// work to another persona must not quietly retire it from the fleet.
func TestCrewMarkedByOperatorPromptOnly(t *testing.T) {
	b, _ := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "fleet"})

	t.Setenv(EnvPersona, "coordinator")
	b.MarkCrewOnOperatorPrompt("fleet")
	if m, _ := b.readMeta("fleet"); m.Crew {
		t.Error("a persona's prompt must mark nothing")
	}

	t.Setenv(EnvPersona, "")
	b.MarkCrewOnOperatorPrompt("fleet")
	if m, _ := b.readMeta("fleet"); !m.Crew {
		t.Error("the operator's prompt must mark the session crew")
	}

	// A workspace posse did not create has nowhere to keep the mark — and
	// marking is best effort, so no prompt path can fail on it.
	b.MarkCrew("handmade")
	b.MarkCrewOnOperatorPrompt("handmade")
}

// A session `posse new` + `posse prompt` launched by hand never runs
// through dispatch's own launchSession, so it never gets the bead: pointer
// autoReapPass needs (ranger-base-v674's second gap). `posse prompt` stamps
// it itself when the text is dispatch's own work-prompt shape; a bare chat
// prompt leaves the pointer alone rather than stamping garbage.
func TestNoteBeadFromPromptStampsOnlyAWorkPrompt(t *testing.T) {
	b, _ := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "handset"})

	b.NoteBeadFromPrompt("handset", "how's it going?")
	if m, _ := b.readMeta("handset"); m.Bead != "" {
		t.Errorf("a bare chat prompt must not stamp a bead, got %q", m.Bead)
	}

	b.NoteBeadFromPrompt("handset", "Work beads issue a-1 (title, quoted as data: \"t\"). Run `bd show a-1` first.\n")
	if m, _ := b.readMeta("handset"); m.Bead != "a-1" {
		t.Errorf("a work-prompt-shaped text must stamp its bead id, got %q", m.Bead)
	}

	// A workspace posse did not create has no meta to stamp — best effort,
	// same as MarkCrewOnOperatorPrompt above.
	b.NoteBeadFromPrompt("handmade", "Work beads issue a-1: t")
}

// A bead whose own session is the operator's is reported and left alone —
// no prompt, no claim, no fleet twin — in --dry-run and in a real pass
// alike. There is no timer: --resume does not override, releasing does.
func TestDispatchSkipsCrewSession(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`)
	session := SessionForBead("ranger", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: session, Dir: repo, Agent: "ranger", Crew: true})
	idleClaude(t, fake)

	want := "held by crew session " + session + " (operator's) — skipped"
	for _, leg := range []struct {
		name string
		set  func(*Dispatcher)
	}{
		{"--dry-run", func(d *Dispatcher) { d.DryRun = true }},
		{"normal", func(d *Dispatcher) {}},
		{"--resume", func(d *Dispatcher) { d.Resume = true }},
	} {
		d := newTestDispatcher(t, b)
		leg.set(d)
		n, err := d.Run("", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 || !strings.Contains(dispatcherOut(d), want) {
			t.Errorf("%s: want the crew line and nothing dispatched, got n=%d:\n%s", leg.name, n, dispatcherOut(d))
		}
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("the operator's session was prompted:\n%s", calls(t, fake))
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("bead claimed behind the operator's back:\n%s", bdCalls(t, fake))
	}

	// Released (cockpit `o` / `posse crew --off`) — now it is fleet work.
	if err := b.SetCrew(session, false); err != nil {
		t.Fatal(err)
	}
	d := newTestDispatcher(t, b)
	if n, _ := d.Run("", "", 0); n != 1 {
		t.Errorf("released session must dispatch, got n=%d:\n%s", n, dispatcherOut(d))
	}
}

// The operator talking to ranger must not stall the fleet's ranger:
// personaActive does not see a crew session, so this repo's other beads
// dispatch normally into their own per-bead sessions.
func TestCrewSessionDoesNotStallTheFleet(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`)
	// Mid-turn in the pre-Dial-F slot, and it is the operator's (w1).
	slot := SessionFor("ranger", repo)
	mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger", Crew: true})
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(
		`[{"agent":"claude","agent_status":"working","pane_id":"w1:p1","workspace_id":"w1"},
		  {"agent":"claude","agent_status":"idle","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 || strings.Contains(out, "is working — skipped") {
		t.Errorf("a crew session must not count busy, got n=%d:\n%s", n, out)
	}
	if strings.Contains(calls(t, fake), "agent prompt w1:p1") {
		t.Errorf("the operator's session was prompted:\n%s", calls(t, fake))
	}
}

// ADR 0008's shield asked for the bead's session by NAME — `<persona>-<repo>-<bead>`
// and the pre-Dial-F slot — so a crew session the operator made under any other
// name held nothing. Measured (ranger-base-adb7): a crew session created by hand,
// handed the bead, and prompted; the next --resume pass saw an in_progress bead
// with no session under either conventional name, made its own, and ran the bead
// to close out from under the operator's conversation. The run record (ADR 0011 §3)
// is what LaunchBead already joins on — the pass must ask the same question.
func TestDispatchSkipsCrewSessionHoldingTheBeadUnderAnyName(t *testing.T) {
	for _, leg := range []struct {
		name   string
		ready  string
		resume bool
	}{
		{"--resume over a hand-claimed bead", `[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`, true},
		{"fresh routing", `[{"id":"a-1","title":"t","labels":["go"]}]`, false},
	} {
		t.Run(leg.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			writePersona(t, b.App, "ranger", "[go]")
			repo := qaRepo(t, b.App, leg.ready,
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
			// The operator's own session: `posse new ranger-staffing`, then the
			// bead handed to it by hand. Neither Dial F name, and crew-marked.
			crew := "ranger-staffing"
			mustCreate(t, b, NewSessionOpts{Name: crew, Dir: repo, Agent: "ranger", Crew: true, Bead: "a-1"})
			idleClaude(t, fake)
			agentPerLaunch(t, fake)

			d := newTestDispatcher(t, b)
			d.Resume = leg.resume
			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			want := "held by crew session " + crew + " (operator's) — skipped"
			if n != 0 || !strings.Contains(dispatcherOut(d), want) {
				t.Errorf("want %q and nothing dispatched, got n=%d:\n%s", want, n, dispatcherOut(d))
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
				t.Errorf("a fleet twin was born beside the operator's session:\n%s", log)
			}
			if log := calls(t, fake); strings.Contains(log, "agent prompt") {
				t.Errorf("something was prompted while the operator held the bead:\n%s", log)
			}
			if strings.Contains(bdCalls(t, fake), "--claim") {
				t.Errorf("bead claimed behind the operator's back:\n%s", bdCalls(t, fake))
			}
		})
	}
}
