//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0030 (bead ranger-base-um9a, discovered from ranger-base-adb7 via
// ranger-base-vn3o): an in_progress bead no live session holds under ANY
// name — no run record, no Dial F name, no slot — is genuinely ambiguous
// at the recovery moment: a crashed fleet run to relaunch, or the
// operator's own hand-work, typed straight into a crew session that
// stamps no record and carries no naming-convention name. A live crew
// session of the assignee in the bead's repo, asked only at that one
// ambiguity, parks the bead instead of guessing — visible, no twin, no
// claim, `--resume` does not override.

import (
	"strings"
	"testing"
)

// The typed route: the operator opens a crew session under an arbitrary
// name, claims the bead by hand (any tool, not `posse prompt`), and works
// it there. Neither the Dial F name nor the slot names it, and nothing
// stamped a run record — a fresh pass and `--resume` must both park it,
// and `--dry-run` must say the same line a real pass would.
func TestDispatchParksOrphanedClaimUnderTheTypedRoute(t *testing.T) {
	t.Parallel()
	for _, leg := range []struct {
		name string
		set  func(*Dispatcher)
	}{
		{"fresh pass", func(d *Dispatcher) {}},
		{"--resume", func(d *Dispatcher) { d.Resume = true }},
		{"--dry-run", func(d *Dispatcher) { d.DryRun = true }},
	} {
		t.Run(leg.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			writePersona(t, b.App, "ranger", "[go]")
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
			// `posse new ranger-adhoc`, then the bead claimed and worked by
			// hand: no Dial F name, no slot, no `bead:` record.
			crew := "ranger-adhoc"
			mustCreate(t, b, NewSessionOpts{Name: crew, Dir: repo, Agent: "ranger", Crew: true})
			idleClaude(t, fake)
			agentPerLaunch(t, fake)

			d := newTestDispatcher(t, b)
			leg.set(d)
			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			want := "claimed by ranger, no session posse started — crew session " + crew + " is live"
			if n != 0 || !strings.Contains(out, want) {
				t.Errorf("want the park line and nothing dispatched, got n=%d:\n%s", n, out)
			}
			if !strings.Contains(out, "posse prompt it with the bead") || !strings.Contains(out, "posse crew "+crew+" --off") {
				t.Errorf("park line does not name both releases:\n%s", out)
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

// ADR 0030 §2's accepted cost, pinned on purpose: dispatch cannot tell "the
// operator is hand-working this" from "the runner died mid-bead" from
// here — no record survives a crash any more than the typed route ever
// wrote one — so a genuinely crashed run of the SAME persona also parks
// while the assignee's crew session is live in the bead's repo, exactly as
// the typed route does. The cost is visible every pass and one keypress
// (`posse crew <name> --off`, or `posse prompt` it) to release; the
// alternative is the twin ADR 0030 exists to prevent.
func TestDispatchParksACrashedRunWhileAssigneesCrewSessionIsLive(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	// No Dial F session and no slot survive the crash; the operator's own
	// conversation, unrelated to this bead by name, is the only live
	// session left standing for this persona in this repo.
	crew := "ranger-standup"
	mustCreate(t, b, NewSessionOpts{Name: crew, Dir: repo, Agent: "ranger", Crew: true})
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	d.Resume = true // crash recovery is exactly what a --resume pass is for
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	want := "claimed by ranger, no session posse started — crew session " + crew + " is live"
	if n != 0 || !strings.Contains(out, want) {
		t.Errorf("recovery must park rather than twin while the crew session lives, got n=%d:\n%s", n, out)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
		t.Errorf("recovery built a fleet twin beside the operator's session:\n%s", log)
	}
}

// ADR 0030 §2 stands: the tiebreak only ever asks about an in_progress
// claim, never about available work, so a READY bead of this persona
// dispatches normally while the operator's own conversation is live in the
// same repo — parking every bead of a persona for one conversation is what
// 0008 §2 already exists to prevent, and this decision moves that line by
// one notch, not one section.
func TestReadyBeadDispatchesDuringCrewChatUnderADR0030(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`)
	crew := "ranger-standup"
	mustCreate(t, b, NewSessionOpts{Name: crew, Dir: repo, Agent: "ranger", Crew: true})
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 || strings.Contains(out, "no session posse started") {
		t.Errorf("a ready bead must dispatch past the operator's conversation, got n=%d:\n%s", n, out)
	}
	if strings.Contains(calls(t, fake), "agent prompt "+crew) {
		t.Errorf("the operator's session was prompted:\n%s", calls(t, fake))
	}
}

// The cockpit's `d` (LaunchBead) asks the same tiebreak fireLoop does and
// refuses with the same reason — the pass and the cockpit are the two
// launchers ADR 0030 names, and neither creates anything.
func TestLaunchBeadRefusesAnOrphanedClaimWhileAssigneesCrewSessionIsLive(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}, Status: "in_progress", Assignee: "ranger"}, Dir: repo}
	crew := "ranger-adhoc"
	mustCreate(t, b, NewSessionOpts{Name: crew, Dir: repo, Agent: "ranger", Crew: true})
	idleClaude(t, fake)

	_, err := d.LaunchBead(is)
	want := "claimed by ranger, no session posse started — crew session " + crew + " is live"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want the orphaned-claim refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "not dispatched") {
		t.Errorf("refusal must say not dispatched: %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a work prompt reached the crew pane:\n%s", log)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("bead claimed onto the operator's session:\n%s", log)
	}
}
