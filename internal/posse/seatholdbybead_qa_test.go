package posse

// ranger-base-25cit: a lane ran over its seat cap because a settle released
// the seat by NAME instead of by the bead holding it.
//
// The bead was filed against the same-pass seat count — "seat 2/2 was
// granted twice because the first launch was not counted before the second
// offer". MEASURED against the log the bead cites
// (~/.config/posse/state/dispatch-watch.log, pass 271, watch pid 94728) that
// is not what happened, and the first test here is the pin that says so: the
// same-pass count worked, on the shipped binary and at HEAD, and the log
// records it working eight lines before the over-cap —
//
//	· ranger-base-hr5j4 label:qa (seat 2/2: laurie; holden busy)
//	· ranger-base-hr5j4 creating session laurie-posse-ranger-base-hr5j4
//	– ranger-base-jaqx2 qa lane busy: holden, laurie — waits for a later pass
//	↻ refill for settled seat laurie-posse (ranger-base-6z06r settled)
//	· ranger-base-jaqx2 label:qa (seat 2/2: laurie; holden busy)
//	· ranger-base-jaqx2 creating session laurie-posse-ranger-base-jaqx2
//
// — jaqx2 was refused the seat by the fire pass and then handed it by the
// refill twelve lines later. What freed it was ranger-base-6z06r's settle:
// 6z06r had settled at 14:50:18, its result sat in the results channel while
// the fire pass put hr5j4 on the seat at 14:53:18, and `judge` then deleted
// the hold under the seat's name — retiring a launch three minutes younger
// than the settle that retired it. The QA lane ran 3/2 for the length of a
// verify.
//
// Two pins, and neither is the other's control:
//
//   - the same-pass count, so the mechanism the bead names cannot regress
//     unnoticed while a different one is being fixed. MUTATION: drop
//     `seats.hold(slot, is.ID)` after the fire in fireLoop → the one-seat
//     lane fires both beads and prints no busy line → red (run).
//   - the release, keyed on the bead. MUTATION: `delete(busy, seat)`
//     unconditionally in judge → the settle frees a seat another bead holds
//     → red (run). Its own second arm is the control: a settle for the bead
//     that IS on the seat must still free it, or the fix is a seat that
//     never releases and every lane silently narrows to nothing.

import (
	"strings"
	"testing"
)

// The mechanism the bead names, pinned where it lives: a lane of one seat
// offered two ready beads in a single fire pass launches exactly one, and
// the second is told the lane is busy — not because a reading found the
// persona working (nothing has had time to), but because the launch this
// same pass just made is counted.
func TestQASamePassSecondOfferSeesTheFirstLaunch(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "laurie", "[qa]")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["qa"]},{"id":"a-2","title":"u","labels":["qa"]}]`,
		`[{"id":"a-1","status":"closed"},{"id":"a-2","status":"closed"}]`)
	agentPerLaunch(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 {
		t.Errorf("a one-seat lane offered two beads in one pass must fire one, got %d:\n%s", n, out)
	}
	// The witness that the fixture ran at all: an assertion that only counts
	// launches is satisfied by a pass that launched nothing.
	if !strings.Contains(out, "· a-1            creating session") {
		t.Fatalf("the first bead must launch, or nothing here is measured:\n%s", out)
	}
	// A lane of one prints ADR 0020 §2's single-seat form, not the lane form.
	if !strings.Contains(out, "– a-2            qa lane busy: laurie") {
		t.Errorf("the second bead must be refused by the launch this pass just made:\n%s", out)
	}
}

// The release, keyed on the bead. Both arms run `judge` — the call site that
// held the defect — over a busy map with a bead already on the seat.
func TestQASettleFreesOnlyItsOwnSeatHold(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		held    string // what the seat holds when the settle is judged
		freed   bool   // must the settle release it
		wantOut string
	}{
		{
			// The measured shape: hr5j4 on the seat, 6z06r's settle judged
			// after it. The hold must survive, and the log must say why —
			// a refill that hires nowhere is otherwise indistinguishable
			// from a queue with nothing ready in it.
			name: "stale settle over a seat retaken since", held: "a-2", freed: false,
			wantOut: "stays held by a-2",
		},
		{
			// The control, and it is load-bearing rather than decorative:
			// the fix is one `if` around the delete, and an `if` that never
			// takes the release arm is a seat cap of zero. This arm is what
			// separates "released the wrong hold" from "stopped releasing".
			name: "settle for the bead that holds the seat", held: "a-1", freed: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writePersona(t, b.App, "laurie", "[qa]")
			// Nothing ready: the refill under the release returns before it
			// offers anything, so what this test reads is the release and
			// not a launch decision taken behind it.
			repo := qaRepo(t, b.App, `[]`, `[{"id":"a-1","status":"closed"}]`)
			d.Refill = true

			seat := SessionFor("laurie", repo)
			busy := map[string]string{seat: tc.held}
			is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"qa"}}, Dir: repo}
			g := gathered{p: &pendingBead{is: is, persona: "laurie"}, is: is, persona: "laurie"}

			d.judge(g, "", "", 0, busy, map[string]int{})

			out := dispatcherOut(d)
			switch held, ok := busy[seat]; {
			case tc.freed && ok:
				t.Errorf("a settle must free the seat its OWN bead holds, still %q:\n%s", held, out)
			case !tc.freed && held != tc.held:
				t.Errorf("the seat holds %s and a-1's settle says nothing about it; got %q — this is the QA lane running 3/2 (ranger-base-25cit):\n%s", tc.held, held, out)
			}
			if tc.wantOut != "" && !strings.Contains(out, tc.wantOut) {
				t.Errorf("a settle that frees no seat must say so, want %q:\n%s", tc.wantOut, out)
			}
		})
	}
}
