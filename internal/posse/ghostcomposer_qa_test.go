package posse

// QA pins for ranger-base-2hvtv: a box previewing the line the pane last
// SUBMITTED is echoing it, not holding it.
//
// ranger-base-wr624 took that reading out of the pulse's delivery path and
// left the two callers that still act on it — dispatch's --resume skip and
// govern's G2 row — with one instruction: either a discriminator with a
// FAILING WRONG ARM, or a written measurement showing those two cannot see
// the phantom. The measurement said they can (govern carried
// `settled-unsent:ranger-base-176sd` into the coordinator's pulse prompt at
// 2026-09-05T03:17:06Z), so this is the discriminator, and the arms below
// are laid out in pairs on purpose: every arm that stops acting has a twin,
// one field different, that still acts exactly as it did before this bead.
//
// The mechanism and its measurement are in sentline.go.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the store ───────────────────────────────────────────────────────────────

const probeSession = "f4987ef2-86df-48c7-8946-86a5c03e31f9"

// armSubmitted writes a claude submit log in the shape the live one has
// (2.1.261: display, pastedContents, project, sessionId, timestamp) and
// points the backend at it. Rows are given in order, oldest first.
func armSubmitted(t *testing.T, b *HerdrBackend, session string, rows ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.jsonl")
	var buf strings.Builder
	for i, r := range rows {
		line, err := json.Marshal(map[string]any{
			"display":        r,
			"pastedContents": map[string]any{},
			"project":        "/tmp/proj",
			"sessionId":      session,
			"timestamp":      1788000000000 + int64(i)*1000,
		})
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	b.ClaudeHistory = path
	return path
}

// armPaneSession makes the fake herdr answer `pane get` with a claude
// session id, which is the join the store is read through.
func armPaneSession(t *testing.T, fake, session string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fake, "pane-session"), []byte(session), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── the readings ────────────────────────────────────────────────────────────

// The store answers with the LAST row for THIS session — not the last row in
// the file, and never another session's.
func TestLastSubmittedIsThisSessionsMostRecentRow(t *testing.T) {
	t.Parallel()
	b := &HerdrBackend{}
	path := armSubmitted(t, b, probeSession, "how we doing?", "ok will check in later")
	// Another pane's session, appended after ours: the file is shared by
	// every claude on the box, so this is the ordinary case, not an edge.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(f, `{"display":"somebody else's line","sessionId":"other","project":"/tmp/p"}`+"\n")
	f.Close()

	got, ok := lastSubmitted(path, probeSession)
	if !ok || got != "ok will check in later" {
		t.Fatalf("lastSubmitted = %q, %v; want the session's own last row", got, ok)
	}
	if got, ok := lastSubmitted(path, "no-such-session"); ok || got != "" {
		t.Fatalf("a session with no rows answered %q, %v — it must answer nothing", got, ok)
	}
	if got, ok := lastSubmitted(path, ""); ok || got != "" {
		t.Fatalf("an unnamed session borrowed another pane's sends: %q, %v", got, ok)
	}
}

// Every way the store can fail to answer, and they all fail the same way:
// no echo, so the text stays a hold and the callers behave as before.
func TestLastSubmittedFailsTowardsToday(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.jsonl")
	junk := filepath.Join(dir, "junk.jsonl")
	if err := os.WriteFile(junk, []byte("not json at all\n{\"display\":\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		what, path string
	}{
		{"no path at all", ""},
		{"a store that is not there", missing},
		{"rows in a shape this does not know", junk},
	} {
		if got, ok := lastSubmitted(c.path, probeSession); ok || got != "" {
			t.Errorf("%s answered %q, %v — every failure must answer nothing", c.what, got, ok)
		}
	}
	// One unreadable row does not take the store down with it: the rows
	// around it still answer.
	mixed := filepath.Join(dir, "mixed.jsonl")
	body := "{ not json\n" +
		`{"display":"good row","sessionId":"` + probeSession + `"}` + "\n"
	if err := os.WriteFile(mixed, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := lastSubmitted(mixed, probeSession); !ok || got != "good row" {
		t.Errorf("one bad row hid the good ones: %q, %v", got, ok)
	}
}

// The comparison. Equality is the whole test unless herdr truncated the
// preview — which is the guard that keeps a short composer from matching
// every long submit that happens to start with it.
func TestSubmittedEchoNeedsTruncationBeforeItAcceptsAPrefix(t *testing.T) {
	t.Parallel()
	const long = "close the bead once the suite is green, and say on it which packages you ran"
	for _, c := range []struct {
		what              string
		typed, sent       string
		truncated, wanted bool
	}{
		{"the line it last submitted", "how we doing?", "how we doing?", false, true},
		{"a different line", "ok back", "ok goign to bed", false, false},
		{"a prefix of it, preview NOT cut", "close the bead", long, false, false},
		{"a prefix of it, preview cut", "close the bead", long, true, true},
		{"longer than the row it starts with", long, "close the bead", true, false},
		{"an empty box", "", "how we doing?", false, false},
		{"a store with nothing in it", "how we doing?", "", false, false},
		{"whitespace either side", " how we doing? ", "how we doing?", false, true},
	} {
		if got := submittedEcho(c.typed, c.sent, c.truncated); got != c.wanted {
			t.Errorf("%s: submittedEcho(%q, %q, %v) = %v, want %v",
				c.what, c.typed, c.sent, c.truncated, got, c.wanted)
		}
	}
}

// Truncation is asked per reading off herdr's own byte count, never
// assumed: `agent explain` reports the region's real size beside the preview
// it cut down.
func TestComposerTruncatedComesFromHerdrsByteCount(t *testing.T) {
	t.Parallel()
	whole := detectionWith("idle", "", unsentBox)
	if whole.ComposerTruncated() {
		t.Error("a preview herdr handed over whole was read as truncated")
	}
	cut := whole
	cut.EvaluatedRules = append([]EvaluatedRule(nil), whole.EvaluatedRules...)
	for i := range cut.EvaluatedRules {
		if cut.EvaluatedRules[i].Region == composerRegion {
			cut.EvaluatedRules[i].Evidence.RegionBytes = 4949
		}
	}
	if !cut.ComposerTruncated() {
		t.Error("a preview cut down from 4949 bytes was read as whole")
	}
	if detectionWith("idle", idleFooter, "").ComposerTruncated() {
		t.Error("a pane with no composer preview at all claimed a truncated one")
	}
}

// ─── the two callers ─────────────────────────────────────────────────────────

// ghostPass is settleWaitingPass with the store armed: the same two-pass
// resuming dispatch over a bead whose holder settled, with the box previewing
// `box` and claude's submit log holding `submitted` for this pane's session.
// An empty `submitted` arms the store with a DIFFERENT line, which is the
// wrong arm.
func ghostPass(t *testing.T, box string, submitted []string, join bool) string {
	t.Helper()
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := settleRepo(t)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	armScreen(t, fake, idleFooter, box)
	armSubmitted(t, b, probeSession, submitted...)
	if join {
		armPaneSession(t, fake, probeSession)
	}
	for i := 0; i < 2; i++ {
		if _, err := d.Run("", "", 0); err != nil {
			t.Fatalf("pass %d: %v", i+1, err)
		}
	}
	return dispatcherOut(d)
}

// unsentText is what unsentBox previews, once the mark and its nbsp are off.
const unsentText = "commit and close the bead once the suite is green"

// THE BEAD, dispatch half. A resuming pass over a holder whose box previews
// the line that pane last submitted re-prompts it, instead of parking the
// bead behind a reading that cannot go false.
func TestQAResumeRePromptsAHolderWhoseBoxEchoesItsLastSubmit(t *testing.T) {
	t.Parallel()
	out := ghostPass(t, unsentBox, []string{"an older line", unsentText}, true)
	if strings.Contains(out, "waiting, not re-prompted") {
		t.Fatalf("--resume parked a bead on a box echoing the line it already sent:\n%s", out)
	}
	if strings.Contains(out, "UNSENT") {
		t.Fatalf("the pass called an already-submitted line an unsent prompt:\n%s", out)
	}
}

// THE WRONG ARM, and the whole reason the one above is a discriminator
// rather than a third narrowing: the same box, the same pass, one field
// different — claude's log says the last thing submitted here was something
// else — and the skip stands. This is the state of the live fleet: at
// 04:2xZ on 2026-09-05 both boxes carrying text carried text that is NOT in
// the store, so both are still holds today.
func TestQAResumeStillWaitsOnABoxTheStoreDoesNotKnow(t *testing.T) {
	t.Parallel()
	out := ghostPass(t, unsentBox, []string{"ok goign to bed"}, true)
	if !strings.Contains(out, "waiting, not re-prompted") {
		t.Fatalf("a resuming pass typed over a prompt nobody has sent:\n%s", out)
	}
	if !strings.Contains(out, "UNSENT") || !strings.Contains(out, "commit and close the bead") {
		t.Fatalf("the pass did not show the operator the prompt that never landed:\n%s", out)
	}
}

// The join is herdr's to make, and a herdr that will not name a claude
// session for the pane leaves the rung exactly as it was — the same
// concession panework.go makes for a screen it cannot read. Same store, same
// box as the bead arm; only `pane get` is unarmed.
func TestQAResumeStillWaitsWhenHerdrNamesNoSession(t *testing.T) {
	t.Parallel()
	out := ghostPass(t, unsentBox, []string{"an older line", unsentText}, false)
	if !strings.Contains(out, "waiting, not re-prompted") {
		t.Fatalf("a pane herdr would not name a session for was treated as an echo:\n%s", out)
	}
}

// govG2Row runs the shop check over one settled holder with the box and the
// store armed, and hands back the G2 row.
func govG2Row(t *testing.T, box string, submitted []string) *GovCondition {
	t.Helper()
	b, fake := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	sessionAgent(t, fake, "developer-x", "idle")
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})
	armScreen(t, fake, idleFooter, box)
	armPaneSession(t, fake, probeSession)
	armSubmitted(t, b, probeSession, submitted...)
	return find(shopSet(t, govIn(t, b)), "G2")
}

// THE BEAD, govern half. The row the coordinator is told to fix by hand is
// the one where a prompt really is sitting unsent. A box echoing the line it
// last submitted is a plain settled holder, and the row says which reading
// changed its shape.
func TestQAGovG2DoesNotCallAnEchoAnUnsentPrompt(t *testing.T) {
	g := govG2Row(t, unsentBox, []string{"an older line", unsentText})
	if g == nil {
		t.Fatal("a settled holder dropped off the surface entirely")
	}
	if g.Key != "settled:bd-1" {
		t.Fatalf("G2 key = %q, want settled:bd-1 — the coordinator is still being sent to clear a sent line", g.Key)
	}
	if !strings.Contains(g.Detail, "echoes the line it last submitted") ||
		!strings.Contains(g.Detail, unsentText) {
		t.Errorf("the row does not say which reading changed its shape: %q", g.Detail)
	}
}

// THE WRONG ARM, govern half: same box, store says the last submit was
// something else, and the row is the -unsent one it has always been.
func TestQAGovG2KeepsUnsentWhenTheStoreDoesNotKnowTheBox(t *testing.T) {
	g := govG2Row(t, unsentBox, []string{"ok goign to bed"})
	if g == nil {
		t.Fatal("a session whose prompt never landed dropped off the surface entirely")
	}
	if g.Key != "settled-unsent:bd-1" {
		t.Fatalf("G2 key = %q, want settled-unsent:bd-1", g.Key)
	}
	if !strings.Contains(g.Detail, "UNSENT") {
		t.Errorf("the row does not tell the operator what to fix: %q", g.Detail)
	}
}

// The control the two above rest on: an EMPTY box with the very same store
// armed is the plain settled row, and it says nothing about an echo. Without
// it, "the key is settled:bd-1" is also what a pass that never read a
// composer prints.
func TestQAGovG2SaysNothingAboutAnEchoOnAnEmptyBox(t *testing.T) {
	g := govG2Row(t, emptyBox, []string{"an older line", unsentText})
	if g == nil || g.Key != "settled:bd-1" {
		t.Fatalf("the settled-but-holding row stopped firing for a holder that really did stop: %+v", g)
	}
	if strings.Contains(g.Detail, "echoes the line it last submitted") {
		t.Errorf("an empty box was reported as echoing something: %q", g.Detail)
	}
}
