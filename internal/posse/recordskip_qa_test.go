//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for the ADR 0013 §4 record stage (ranger-base-6jz): what a pass
// does when the runtime settles and the store of record says the work is not
// done.
//
// MEASURED, and the reason the stage exists: 3/3 dispatched codex sessions
// on 2026-08-24 did the work and skipped the bead — no comment, no close
// (ranger-base-0fb). Claude and grok closed theirs the same evening. So
// `record:` is a per-runtime declaration, and these pin the three things it
// buys: the pass never ticks work nobody recorded, it says which runtime's
// declared degrade it is looking at, and the bead is left in a state a later
// pass RETRIES rather than one a human has to notice.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codexPersona is a PID on the runtime that is `record: untrusted` for a
// measured reason.
func codexPersona(t *testing.T, a *App, name, labels string) {
	t.Helper()
	os.MkdirAll(a.AgentsDir, 0o755)
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\nruntime: codex\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The tick belongs to the bead. A settled agent is a hint (ADR 0011), and on
// an untrusted runtime it is a hint that has been wrong three times out of
// three.
func TestUntrustedSettleWithoutCloseIsReviewedNotDone(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	codexPersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if strings.Contains(out, "✓") {
		t.Errorf("a settle with the bead still open was ticked:\n%s", out)
	}
	if !strings.Contains(out, "◑ a-1") || !strings.Contains(out, `issue is "in_progress"`) {
		t.Errorf("the pass must say the bead is not done:\n%s", out)
	}
	// And WHOSE degrade it is, so the operator reads it as the declared
	// contract rather than as a mystery to investigate every pass.
	for _, want := range []string{"codex is record: untrusted", "--resume re-prompts"} {
		if !strings.Contains(out, want) {
			t.Errorf("the line must name %q:\n%s", want, out)
		}
	}
	// The harness does not close on the agent's behalf: that hides the
	// defect and puts a human back in the loop dispatch exists to replace.
	if log := bdCalls(t, fake); strings.Contains(log, "close a-1") {
		t.Errorf("the harness closed the bead for the persona:\n%s", log)
	}
	_ = repo
}

// The claim is what makes the retry safe: the bead stays this persona's, so
// the next pass finds a holder to re-prompt rather than a free bead to
// re-route (at-least-once + claim-as-fence).
func TestUntrustedSettleWithoutCloseKeepsTheClaim(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	codexPersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--status open") {
		t.Errorf("the pass handed the bead back after a settle it could not judge:\n%s", log)
	}
}

// The other half of the degrade (ranger-base-f0g): unattended `--resume` is
// what turns "reviewed-not-done" into a retry. Without it the bead sits open
// behind a live idle session until a human re-prompts it by hand — which is
// what happened, three times in one evening.
func TestResumeRePromptsAnUntrustedSettleWithoutClose(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	codexPersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	session := SessionForBead("ranger", repo, "a-1")

	// A second pass, over the same live idle session and the same open bead.
	// Without --resume it is left alone on purpose (rangerhq-zom): a persona
	// that stopped and said so is not re-prompted every pass.
	//
	// LEFT ALONE SILENTLY, and that is the contract since ranger-base-eh1kr
	// (ranger-base-bknod found the pin). A claimed bead reaches a pass
	// through interruptedRuns now, and a holder herdr calls settled is not
	// in that population unless `--resume` asks for it — so the fire loop's
	// settled skip never runs and its line ("held by …, … idle — stopped on
	// purpose? (--resume re-prompts)", dispatch.go) is not printed. This arm
	// used to assert that line, and it could only ever have come from a
	// fixture: it needs an in_progress row in `bd ready`, which no store
	// answers with. What reports a holder that stopped without closing is
	// the governance surface's G2 row (`settled:<bead>`, govern.go), on its
	// own cadence rather than once per pass — the trade eh1kr named.
	//
	// Counted on the HERDR CALL LOG and taken either side of the pass. The
	// pass transcript is the wrong stream to ask — `agent prompt` is a call
	// herdr makes, not a line a pass prints, so a Contains over
	// dispatcherOut is satisfied by any pass at all.
	before := strings.Count(calls(t, fake), "agent prompt")
	quiet := newTestDispatcher(t, b)
	if _, err := quiet.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(calls(t, fake), "agent prompt"); got != before {
		t.Errorf("an unattended pass re-prompted the settled holder (%d → %d):\n%s",
			before, got, dispatcherOut(quiet))
	}

	again := newTestDispatcher(t, b)
	again.Resume = true
	if _, err := again.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(again)
	if !strings.Contains(out, session) {
		t.Fatalf("--resume did not reach the holding session %s:\n%s", session, out)
	}
	// ...and the silence above was this pass's to break: the count moves
	// now. Without this arm the unchanged count is satisfied by a rig that
	// cannot prompt at all.
	if after := strings.Count(calls(t, fake), "agent prompt"); after <= before {
		t.Errorf("--resume added no prompt (before %d, after %d) — the silence above pins nothing", before, after)
	}
	// Re-prompt means THIS session (ADR 0004 §3), not a fresh Dial F one
	// beside it, and a resume into a live CLI is typed — the launch line
	// was typed once, when the session was created.
	if !strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("--resume did not re-prompt the holder:\n%s", calls(t, fake))
	}
	if n := strings.Count(calls(t, fake), "workspace create"); n != 1 {
		t.Errorf("--resume created %d workspaces for one bead — a twin beside the holder", n)
	}
}

// A `record: trusted` runtime gets the same honest ◑ and no clause. A
// runtime measured to close its beads that has stopped closing them is the
// signal record-skip-rate exists to catch; a reassuring parenthesis beside
// it would be the harness explaining away its own evidence.
func TestTrustedSettleWithoutCloseGetsNoUntrustedClause(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]") // claude: record: trusted
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if strings.Contains(out, "✓") {
		t.Errorf("a settle with the bead still open was ticked:\n%s", out)
	}
	if !strings.Contains(out, "review ") {
		t.Errorf("a trusted runtime's settle-without-close is still reviewed-not-done:\n%s", out)
	}
	if strings.Contains(out, "record: untrusted") {
		t.Errorf("claude is record: trusted and must not wear the clause:\n%s", out)
	}
}
