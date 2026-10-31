//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-nc91t — relaunch's two readings are the SLOT's
// listing, so a relaunch on a non-herdr instance aims at the recorded seat
// or refuses, and never at the wrong one (ADR 0076 §5).
//
// Both readings used to be herdr's raw `workspace list` behind §4's door,
// which does not answer on another occupant at all. What that cost was not
// damage — it was a relaunch that could never run: the preflight's name
// question refused first ("this herdr did not list its workspaces"), and
// behind it the aiming question would have fallen to clearDeadMeta and been
// refused honestly by mustNotOrphan. Nothing broken, nothing refreshed, and
// ranger-base-q5gc2's notes §5 left it open on purpose.
//
// THE CLAIM IS NOT "IT WORKS NOW". It is the pair, because only one of them
// can be read off a green relaunch:
//
//  1. a listing that WAS taken aims the kill at the seat the record names,
//     under the occupant's own identity rules (no fence on Rex, herdr's
//     fence on herdr);
//  2. a listing that could NOT be taken refuses — at both call sites, with
//     nothing closed and the record left standing.
//
// (2) is the one worth a pin: it is the arm an occupant's outage reaches,
// and a conversion that returned "not ours" instead of an error would read
// as success on every board a test is likely to build.
//
// The stub occupant embeds Mux as a nil value, so a verb these paths have no
// business calling panics by method name rather than answering a zero value
// (muxMetaStub's rule, one bead over).

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type relaunchAimStub struct {
	Mux
	name   string
	addr   string
	seats  []SeatInfo
	err    error
	lists  int
	closed []Seat
}

func (s *relaunchAimStub) Server() MuxServer { return MuxServer{Name: s.name, Addr: s.addr} }

func (s *relaunchAimStub) List() ([]SeatInfo, error) {
	s.lists++
	if s.err != nil {
		return nil, s.err
	}
	return s.seats, nil
}

func (s *relaunchAimStub) Close(seat Seat) error {
	s.closed = append(s.closed, seat)
	return nil
}

// relaunchAimBoard is a Rex-shaped instance holding one record. The listing
// carries a decoy FIRST — another seat, wearing a label this home would
// never write — so a reading that took the head of the list, or matched on
// anything but the recorded session id, aims at it.
func relaunchAimBoard(t *testing.T, seatPresent bool) (*HerdrBackend, *relaunchAimStub, *HerdrMeta) {
	t.Helper()
	b, _ := newTestBackend(t)
	stub := &relaunchAimStub{name: "rex", addr: "127.0.0.1:7777",
		seats: []SeatInfo{{Seat: Seat{Session: "other-uuid", Pane: "block-9"}, Label: "somebody-else"}}}
	if seatPresent {
		// Labelled with something this home would never write for the name:
		// herdr's stranger fence firing at all would make this a seat the
		// relaunch refuses to aim at, and on an occupant whose ids are never
		// reissued a present id IS the seat the record meant (ADR 0076 §5).
		stub.seats = append(stub.seats,
			SeatInfo{Seat: Seat{Session: "01a1902c-live", Pane: "block-1"}, Label: "renamed-in-rex"})
	}
	b.Slot = stub
	m := &HerdrMeta{
		Name: "s1", Workspace: "01a1902c-live", Pane: "block-1", Emoji: "🤠", Agent: "scout",
		Mux: "rex", Socket: "127.0.0.1:7777",
		Launched: time.Now().Add(-2 * PruneGrace),
	}
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	return b, stub, m
}

// ─── claim 1: a listing that was taken aims at the recorded seat ─────────────

func TestQAARelaunchAimsAtTheRecordedSeatOnANonHerdrOccupant(t *testing.T) {
	t.Parallel()
	b, stub, m := relaunchAimBoard(t, true)

	if ours, err := b.holdsRecorded(m); err != nil || !ours {
		t.Fatalf("holdsRecorded on a non-herdr occupant answered (%v, %v), want (true, nil) — the aiming question is the slot's one listing, and reading herdr's raw `workspace list` here is what refused every relaunch on such an instance (ADR 0076 §5)", ours, err)
	}
	if err := b.closeRecorded(m, nil); err != nil {
		t.Fatalf("closeRecorded refused a seat its own occupant is holding: %v", err)
	}
	if len(stub.closed) != 1 || stub.closed[0] != (Seat{Session: "01a1902c-live", Pane: "block-1"}) {
		t.Errorf("the kill closed %+v, want exactly the seat the RECORD names — a relaunch aimed by anything else closes a stranger and leaves the session it was asked to refresh running (rangerhq-9jk1)", stub.closed)
	}
	if _, ok := b.readMeta("s1"); ok {
		t.Error("the record outlived the kill, so the recreate will meet it at mustNotOrphan and refuse to take its own name back")
	}
}

// The label half of herdr's fence is not asked of an occupant that does not
// recycle ids, and the seat above proves it: it is labelled `renamed-in-rex`
// and was still aimed at. MUTATION: drop notOurWorkspace's non-herdr arm and
// this is a live seat a relaunch will not aim at and cannot ever clear.
func TestQAANonHerdrSeatIsAimedAtWithNoIdentityFence(t *testing.T) {
	t.Parallel()
	b, _, m := relaunchAimBoard(t, true)
	if why := b.notOurWorkspace(m, "renamed-in-rex", b.Mux().Server().Gen); why != "" {
		t.Errorf("the fence fenced a non-herdr seat: %s\nRex's ids are UUIDs and are never reissued, so a present id is the seat the record meant; the label half alone reads a rename as a recycle, which is the reading that predicate's own doc rejects (ADR 0076 §5)", why)
	}
}

// ─── claim 2: a listing that could not be taken refuses, at both sites ───────

func TestQAARelaunchRefusesWhenTheOccupantsListingCannotBeTaken(t *testing.T) {
	t.Parallel()
	b, stub, m := relaunchAimBoard(t, true)
	stub.err = errors.New("rex server is not running")

	// Site 1 — the preflight's NAME question, which is the one an operator
	// actually meets: it is asked before the landing turn and before the
	// lock, so it is what refuses first on an instance whose listing is
	// unreadable.
	err := b.provenNameTakeable(m)
	if err == nil {
		t.Fatal("the preflight proved the name takeable from a listing it never read — on the far side of the kill that name may be gone, which is rangerhq-v52t's loss reached through a silent reading")
	}
	if !strings.Contains(err.Error(), "was NOT closed") || !strings.Contains(err.Error(), "rex") {
		t.Errorf("the refusal must say nothing was destroyed and name the occupant that would not answer, so the operator knows which server to look at: %v", err)
	}

	// Site 2 — the AIMING question. An error, not "not ours": the two mean
	// opposite things to closeRecorded, and only the error reaches the guard.
	if _, err := b.holdsRecorded(m); err == nil {
		t.Error("holdsRecorded read an unreadable listing as an answer — absence from a listing nobody took is not absence, and `false, nil` here would send the pass to clearDeadMeta claiming the seat was not ours")
	}
	if err := b.closeRecorded(m, nil); err == nil {
		t.Fatal("closeRecorded proceeded over a listing it could not read")
	}
	if len(stub.closed) != 0 {
		t.Errorf("a seat was closed on no evidence: %+v", stub.closed)
	}
	if _, ok := b.readMeta("s1"); !ok {
		t.Error("the only record of what to recreate was destroyed over a listing that was never read — state/ is outside git, so the ids are not recoverable from anywhere else (rangerhq-cpeh)")
	}
}

// A seat the occupant is NOT holding is still not aimed at by id, and what
// may be done to its record stays clearDeadMeta's call — unchanged by the
// conversion, and pinned here because the new reading is where "not in the
// listing" is now decided.
func TestQAAnAbsentNonHerdrSeatIsNotClosedById(t *testing.T) {
	t.Parallel()
	b, stub, m := relaunchAimBoard(t, false)
	if ours, err := b.holdsRecorded(m); err != nil || ours {
		t.Fatalf("holdsRecorded answered (%v, %v) for a seat absent from the listing, want (false, nil)", ours, err)
	}
	// From here the count is closeRecorded's own, so the assertion below is
	// about the pass and not about this test's probe.
	stub.lists = 0
	if err := b.closeRecorded(m, nil); err != nil {
		t.Fatalf("a seat absent from a listing this occupant DID give is dead by ADR 0076 §5's own rule, so the record is clearable: %v", err)
	}
	if len(stub.closed) != 0 {
		t.Errorf("a seat nothing lists was closed by id: %+v", stub.closed)
	}
	if _, ok := b.readMeta("s1"); ok {
		t.Error("the record of a seat proven gone was kept, so its name can never be reused")
	}
	// Proven by LISTING, and more than once: the aiming question's snapshot
	// is not evidence about the store the guard is about to act on
	// (rangerhq-9nso), so clearDeadMeta's own proof takes its own — twice,
	// once outside the launcher lock and once inside it.
	if stub.lists < 3 {
		t.Errorf("closeRecorded called List() %d time(s), want 3 — the aiming question's, and clearDeadMeta's own proof taken twice. Fewer means the guard acted on the snapshot the aiming question handed it, which is the rangerhq-9nso damage shape", stub.lists)
	}
}

// ─── the name question reads the occupant's labels ───────────────────────────

func TestQAANameWornElsewhereReadsTheOccupantsListing(t *testing.T) {
	t.Parallel()
	b, stub, m := relaunchAimBoard(t, true)
	// A second seat wearing the label the recreate is about to ask for. It
	// will still be wearing it on the far side of the kill, so the refusal
	// belongs before anything is destroyed (rangerhq-v52t).
	stub.seats = append(stub.seats,
		SeatInfo{Seat: Seat{Session: "01a1902c-squat", Pane: "block-3"}, Label: b.App.WorkspaceLabel("s1")})

	other, label, err := b.nameWornElsewhere(m)
	if err != nil {
		t.Fatalf("the name question refused a listing the occupant gave: %v", err)
	}
	if other != "01a1902c-squat" || label != b.App.WorkspaceLabel("s1") {
		t.Fatalf("nameWornElsewhere answered (%q, %q), want the squatting seat — on a non-herdr instance this read herdr's raw listing and so refused outright, which is the whole of ranger-base-nc91t", other, label)
	}
	err = b.provenNameTakeable(m)
	if err == nil {
		t.Fatal("a seat already wearing this session's label let the relaunch through: the kill would land and the recreate could never take the name back")
	}
	if !strings.Contains(err.Error(), "01a1902c-squat") || !strings.Contains(err.Error(), "was NOT closed") {
		t.Errorf("the refusal must name the seat in the way and say nothing was destroyed: %v", err)
	}
	// And the session's OWN seat is not the obstacle, which is the arm the
	// id comparison carries.
	stub.seats = stub.seats[:len(stub.seats)-1]
	stub.seats[len(stub.seats)-1].Label = b.App.WorkspaceLabel("s1")
	if other, _, err := b.nameWornElsewhere(m); err != nil || other != "" {
		t.Errorf("nameWornElsewhere called the session's own seat a squatter (%q, %v) — a relaunch could then never refresh any session at all", other, err)
	}
}

// ─── herdr's own fence still fires through the new reading ───────────────────

// The conversion must not cost herdr the identity half. A workspace wearing
// the recorded id under a label this home would not write, in a generation
// the record cannot vouch for, is a STRANGER: aiming at it closes somebody
// else's workspace with no landing turn, which is rangerhq-9jk1's damage.
//
// MUTATION-CHECKED: drop the notOurWorkspace call from holdsRecorded and the
// kill closes w1 and clears the record.
func TestQAARelaunchDoesNotAimAtAStrangerWearingTheRecordedId(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")

	m, ok := b.readMeta("s1")
	if !ok {
		t.Fatal("no record written by the create")
	}
	// Same id, a label this home would never write for `s1`, and a
	// generation nothing can vouch for (hermeticGen): the two readings —
	// renamed, or recycled to a stranger — leave identical evidence, and
	// "not proven ours" is the only honest answer.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: m.Workspace, Label: "someone-elses-work"}})

	if ours, err := b.holdsRecorded(m); err != nil || ours {
		t.Fatalf("holdsRecorded answered (%v, %v) for a workspace wearing the recorded id under a stranger's label, want (false, nil) — herdr's allocator is max(live id)+1 recomputed at every server start, so the ids stale metas hold are exactly the ones it hands out again (rangerhq-6bg7)", ours, err)
	}
	if err := b.closeRecorded(m, nil); err == nil {
		t.Fatal("the relaunch proceeded over a workspace it could not prove was this session's")
	}
	if log := calls(t, fake); strings.Contains(log, "workspace close") {
		t.Errorf("a workspace nobody proved was ours was closed, with no landing turn:\n%s", log)
	}
	if _, ok := b.readMeta("s1"); !ok {
		t.Error("the record was cleared over a workspace that answered alive — silence is never death, and neither is a stranger's answer (ADR 0011 §2)")
	}
}
