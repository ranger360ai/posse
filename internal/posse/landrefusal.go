package posse

// What a landing turn that never settled was actually looking at —
// ranger-base-4yefx, ask 2.
//
// THE INCIDENT. MEASURED 2026-11-23 by the coordinating persona on this
// instance (multiplexer: rex): a dispatched seat <persona>-<repo>-ranger-base-tvokd,
// whose bead closed 2026-11-21T09:22:47-05:00, read `working` for ~41.6h while its pane held three claude
// background tasks ("Block until the suite exits, then read its verdict",
// twice "Block on the suite and read its verdict") and its tree held nothing.
// `posse kill` refused it twice — at the default 10m and again at
// `--timeout 20m` — with one sentence:
//
//	NOT killed: <name> is still working after 10m0s — let it finish and kill
//	it again (or --timeout 20m0s, or --no-land to close it without the turn)
//
// Every word of that is true and none of it is evidence. The operator spent
// 30 minutes on the two waits it recommends before reaching `--no-land`,
// which was the right move from the first refusal: nothing was going to
// settle, because what claude reported `working` about was background work
// the turn had already handed off, and the tree was clean, so the turn the
// refusal was protecting had nothing left to land.
//
// WHAT WAS AVAILABLE AND DROPPED. Both of the facts that answer "working at
// what?" were one call away at the moment of the refusal:
//
//   - the LAST READING. `Mux.Wait`'s refusal carries it structurally —
//     MuxWaitTimeout.Last, which ranger-base-hcig1 put there for precisely
//     this ("a launcher that abandons one of them owes the operator the
//     difference"), rather than leaving a caller to parse the state back out
//     of a diagnostic. landThePlane's timeout arm answered `return false,
//     nil` and threw the error away, so the one caller the field was built
//     for never read it.
//   - the PANE's own hold. PaneHold (panework.go) is claude's footer summary
//     of its live background tasks, and since ranger-base-r5131 it is read
//     off a capture on an occupant with no `agent explain` — which is this
//     instance. "3 shells still running" is the sentence the operator had to
//     reconstruct by attaching to the pane.
//
// And for a kill, one more that nothing but the kill knows: whether a kill
// would TAKE anything. `residueHolds` is RemoveSessionTree's own refusal
// asked as a question (autoreap.go), and it is what turns "--no-land is
// available" into "--no-land loses nothing" — the judgment the operator made
// by hand with `git status` before typing it.
//
// WHAT THIS DOES NOT DO. It decides nothing. The refusal still refuses, on
// the same evidence and in the same direction: an unsettled turn stops the
// kill, because closing a workspace whose agent may be mid-commit is the
// loss that whole path exists to prevent (landThePlane's doc). Nothing here
// may stand a refusal down, and the reading it prints may not be read back
// as a state by anybody — a blown deadline is a refusal and not a reading
// (Mux's own Wait row), and this is a sentence for a human, not a branch.
//
// EVERY READING IS BEST-EFFORT, which is panework.go's rule and the same one
// here: a pane posse cannot read, a Wait refusal with no reading on it, a
// `git` that will not answer. Each of those drops its own clause and the
// refusal prints what is left, down to the bare sentence it printed before
// this file existed. A refusal that failed to explain itself is still a
// refusal; an explanation that invented a clause would be worse than the
// silence it replaced.

import (
	"fmt"
	"strings"
)

// landRefusal is what one unsettled landing turn was looking at. The zero
// value is "nothing could be read about it", which renders as no clauses at
// all.
type landRefusal struct {
	// Last is the reading the occupant held when its Wait window closed, and
	// Reading says whether it carried one — herdr's own timeout envelope
	// carries none (MuxWaitLastReading), and "no reading at all" is a
	// different fact about a seat from "the last reading was `working`".
	Last    MuxReading
	Reading bool

	// Hold is what the pane was holding, read after the refusal and off the
	// same stores every other consequence site reads (sessionHolding).
	Hold PaneHold

	// Stage is which leg of the turn refused, in the operator's terms: the
	// settle wait before the prompt, or the wait on the turn the prompt
	// started. They are different facts — the second means the text landed —
	// and the sentence that names neither reads as though the turn was never
	// typed.
	Stage string
}

const (
	// landStageSettle is the wait for the pane to be promptable at all: no
	// landing prompt has been typed.
	landStageSettle = "waiting for it to go idle before typing the landing prompt"
	// landStagePrompt is the wait on the turn the landing prompt started.
	landStagePrompt = "waiting for the landing turn it was handed to end"
)

// landRefusalOf reads what it can about a refusal: the reading the Wait
// error carried, and the pane's own hold, taken now.
//
// The error is not interpreted beyond asking it for its reading — which
// refusal this was is the caller's business, and `IsHerdrCode(err,
// "timeout")` has already answered it where this is called.
func (b *HerdrBackend) landRefusalOf(m *HerdrMeta, stage string, err error) landRefusal {
	r := landRefusal{Stage: stage}
	if last, ok := MuxWaitLastReading(err); ok {
		r.Last, r.Reading = last, true
	}
	if m != nil {
		r.Hold = b.sessionHolding(m.Name)
	}
	return r
}

// landRefusalRead is landRefusalOf for the arm that HAS a reading rather
// than an error carrying one: the settle wait that came back with a state
// outside the triple it asked for. Mux.Wait cannot do that today — it
// returns a state from `until` or an error — and the arm is defensive, so
// the reading is carried rather than discarded on the chance that an
// occupant one day answers something nobody branches on.
func (b *HerdrBackend) landRefusalRead(m *HerdrMeta, stage string, read MuxReading) landRefusal {
	r := landRefusal{Last: read, Reading: true, Stage: stage}
	if m != nil {
		r.Hold = b.sessionHolding(m.Name)
	}
	return r
}

// Lines is the refusal's evidence, one clause per line, already indented for
// a Die message's continuation lines. Empty when nothing could be read.
//
// Stage is what tells a refusal that was READ from the zero value: both
// constructors set it, and nothing else does. So an arm that returns the
// zero refusal — a submission herdr refused outright, a caller that never
// reached the turn — renders no clauses at all, and the sentence above them
// is exactly the sentence it was before this file existed. The reading
// clauses are inside that test for the same reason: "the occupant named no
// state" is a claim about an occupant that was ASKED, which the zero value
// says nothing about.
func (r landRefusal) Lines() []string {
	if r.Stage == "" {
		return nil
	}
	out := []string{"  the turn was " + r.Stage}
	switch {
	case r.Reading && r.Last.State != "":
		line := fmt.Sprintf("  the wait ended with last reading `%s`", r.Last.State)
		if r.Last.Evidence != "" {
			// The occupant's own words, verbatim: on rex this is the
			// program_status record — state, app, the runtime's own `msg`
			// and the `updated_at` that says how fresh the claim is
			// (rexEvidence) — and a stale timestamp on a `working` record
			// is the one thing in this whole sentence that can say the
			// report itself has stopped moving.
			line += ": " + strings.TrimSpace(r.Last.Evidence)
		}
		out = append(out, line)
	case r.Reading:
		// MuxReading's own rule: an occupant that was asked and named no
		// state reads as "", and that is an answer rather than a gap.
		out = append(out, "  the wait ended with no readable state at all")
	default:
		// No reading on the refusal at all, which is herdr: its `agent
		// wait` runs the window out inside its own server and hands back a
		// code and a message, with no reading posse ever held
		// (MuxWaitTimeout's last paragraph). The sentence above this one
		// says "still working", and on this route that is the WAIT running
		// out and not something anybody read — said here rather than left
		// for the operator to assume, because the whole of this bead is an
		// operator acting on that assumption twice.
		out = append(out, "  the occupant named no state when its window closed, so `still working` above is the wait running out and not a reading")
	}
	if why := r.Hold.Why(); why != "" {
		out = append(out, "  its pane is holding "+why)
	}
	if r.Hold.Work != "" {
		// The sentence this bead is for. It is said only where the footer
		// actually named live background work, because it is a claim about
		// what the `working` reading is ABOUT, and posse has no other
		// evidence for that claim.
		out = append(out, "  background work claude carries across a turn boundary is not an edit in flight — "+
			"if that is all this seat is holding, no --timeout settles it")
	}
	return out
}

// landRefusalTail renders clauses onto the end of a one-line Die message:
// a leading newline and one line each, or nothing at all when there are no
// clauses, so a refusal that could read nothing prints exactly the sentence
// it printed before this file existed.
func landRefusalTail(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "\n" + strings.Join(lines, "\n")
}

// landRefusalKillLines is Lines plus the one clause only a kill can add:
// what the kill would take. "" from residueHolds is the strong answer — the
// tree is clean and the branch holds no commits of its own — and it is what
// makes `--no-land` safe to type rather than merely available.
//
// The persona's standing orders are committed on every kill, `--no-land`
// included (killAndLand's own comment), so what that flag actually skips is
// the agent's wrap-up: ORDERS.md lessons, bead comments, a commit of work in
// progress. Over a tree that holds nothing there is no third thing left for
// it to lose, and the sentence says exactly that much and no more.
func (b *HerdrBackend) landRefusalKillLines(m *HerdrMeta, r landRefusal) []string {
	out := r.Lines()
	if m == nil {
		return out
	}
	if held := residueHolds(m); held != "" {
		return append(out, "  a kill WOULD take something: "+held+" — that is what the landing turn is for")
	}
	return append(out, "  a kill would take nothing uncommitted: "+killTakesNothing(m)+
		" — so `--no-land` skips only the agent's own wrap-up (its standing orders are committed either way)")
}

// killTakesNothing names the records residueHolds just read as empty, so the
// clause above says WHERE it looked rather than asserting a clean bill over
// paths the reader has to guess at. A shared checkout has one record and a
// worktree session has two.
func killTakesNothing(m *HerdrMeta) string {
	if t := SessionTreeOf(m); t != nil {
		return fmt.Sprintf("%s is clean and %s holds nothing %s does not", AbbrevHome(t.Path), t.Branch, t.Base)
	}
	return AbbrevHome(m.Dir) + " is clean"
}
