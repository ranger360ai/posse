//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-q5gc2 — the session record names its multiplexer,
// and an absent key is herdr (ADR 0076 §5).
//
// Three claims, and the first two are about SILENCE. A new key in a record
// format whose reader is a flat-YAML grep cannot fail loudly: a record that
// reads back wrong reads back as a record, and the session it describes is
// then pruned, withheld or addressed on somebody else's evidence. So:
//
//  1. a record with no `mux:` key reads as herdr. Every record on this box
//     is one, and the key is new, so the default IS the fleet.
//  2. a record already on disk is rewritten byte-identical. Four writers
//     rewrite records nobody edited — the socket/gen backfill on every
//     listing, a relaunch's blanking, a prompt stamp, a turn-failure stamp
//     — so a key that appeared in them would appear in hundreds of files
//     before anyone looked at one.
//  3. for a seat on an occupant that is not herdr, death is absence from
//     that occupant's listing, with no identity fence (§5: Rex's ids are
//     UUIDs and are never reused, so `gen:` stays empty and a present id is
//     the seat the record meant).
//
// The stub occupant below is how claim 3 is reachable at all: herdr is the
// only occupant in the tree today, so the non-herdr arms have nothing to run
// against until Rex lands (ranger-base-qlxb5).

import (
	"os"
	"strings"
	"testing"
	"time"
)

// muxMetaStub is an occupant that answers two things — who it is, and what
// it holds — and panics on every other verb, which is the right failure for
// a pin: a prune that reached for a verb ADR 0076 §5 does not name would be
// a rule nobody wrote down.
//
// The embedded interface is nil, so an unimplemented verb is a nil-pointer
// panic naming the method. Deliberate: a stub that answered every verb with
// a zero value would let a wrong call pass as a reading.
type muxMetaStub struct {
	Mux
	name  string
	addr  string
	seats []SeatInfo
	lists int
	// Close's side, added with ADR 0080 D10 (reclaimdestroy_qa_test.go):
	// the prune now DESTROYS a proven-dead non-herdr seat before it unlinks
	// the record, so Close is a verb §5's prune reaches and a nil-interface
	// panic here would be the pin above failing as a crash.
	closes  []Seat
	onClose func(Seat) error
}

func (s *muxMetaStub) Server() MuxServer { return MuxServer{Name: s.name, Addr: s.addr} }

func (s *muxMetaStub) List() ([]SeatInfo, error) {
	s.lists++
	return s.seats, nil
}

func (s *muxMetaStub) Close(seat Seat) error {
	s.closes = append(s.closes, seat)
	if s.onClose != nil {
		return s.onClose(seat)
	}
	return nil
}

// ─── claim 1: no key means herdr ─────────────────────────────────────────────

func TestQAAMetaWithNoMuxKeyReadsAsHerdr(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	// Written as BYTES, not through writeMeta: the claim is about the
	// records already on disk, and a record this reader produced could only
	// say that it agrees with itself.
	if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.metaPath("legacy"), []byte(
		"name: legacy\nworkspace: 7\npane: %3\nemoji: 🤠\nagent: scout\nsocket: /tmp/h.sock\ngen: 1:2:3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta("legacy")
	if !ok {
		t.Fatal("no record read back")
	}
	if m.Mux != "" {
		t.Errorf("mux: is absent from the file and the field reads %q — a record is being invented, not read", m.Mux)
	}
	if got := m.MuxName(); got != MuxHerdr {
		t.Errorf("a record with no mux: key reads as %q, not %q — every record on this box has no key, so this is the whole fleet reading as another multiplexer (ADR 0076 §5)", got, MuxHerdr)
	}
	// And the default is not vacuous: a key that IS there is read.
	if err := os.WriteFile(b.metaPath("onrex"), []byte(
		"name: onrex\nworkspace: 01a1902c-de\npane: block-1\nemoji: 🤠\nmux: rex\nsocket: 127.0.0.1:7777\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, ok = b.readMeta("onrex")
	if !ok {
		t.Fatal("no record read back for the non-herdr case")
	}
	if m.Mux != "rex" || m.MuxName() != "rex" {
		t.Errorf("mux: rex reads back as field %q / MuxName %q — the key is not read at all, so claim 1 above is only a default with nothing behind it", m.Mux, m.MuxName())
	}
	// The record's Seat is its two ids whoever issued them (ADR 0076 §2).
	if want := (Seat{Session: "01a1902c-de", Pane: "block-1"}); m.Seat() != want {
		t.Errorf("record's seat is %+v, want %+v", m.Seat(), want)
	}
}

// ─── claim 2: a record on disk is rewritten byte-identical ───────────────────

func TestQAAnExistingMetaIsRewrittenByteIdentical(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// Two shapes: a record carrying every key this writer emits, in its
	// order, and the four-line minimum it always emits. Both are the bytes
	// a pre-key posse wrote.
	for _, c := range []struct{ name, raw string }{
		{"full", "name: full\nworkspace: 12\npane: %7\nemoji: 🤠\nenvs: a+b\nagent: scout\n" +
			"runtime: claude\ntier: standard\nmodel: claude-opus-5\ndir: /tmp/w\nrepo: /tmp/r\n" +
			"branch: posse/full\ncage: seatbelt\nsockets: herdr\ndegraded: none\n" +
			"turn_failure: provider refused\nbead: ranger-base-q5gc2\nhooks_mode: redirect\n" +
			"managed_hooks: /tmp/h\ncrew: true\nsocket: /tmp/h.sock\ngen: 1:2:3\n" +
			"launched: 2026-10-30T12:00:00Z\nprompted: 2026-10-30T12:01:00Z\n"},
		{"bare", "name: bare\nworkspace: 13\npane: %8\nemoji: 🤠\n"},
	} {
		if err := os.WriteFile(b.metaPath(c.name), []byte(c.raw), 0o644); err != nil {
			t.Fatal(err)
		}
		m, ok := b.readMeta(c.name)
		if !ok {
			t.Fatalf("%s: no record read back", c.name)
		}
		if err := b.writeMeta(m); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, err := os.ReadFile(b.metaPath(c.name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != c.raw {
			t.Errorf("%s: a record on disk did not survive a read and a write.\n got: %q\nwant: %q\n"+
				"  Every listing may rewrite a record it did not edit (backfillServer), so a key that appears here appears in every record on the box.",
				c.name, got, c.raw)
		}
	}
	// The same discipline from the other side: writeMeta never emits the
	// key for herdr, however the field is spelled. Both spellings are one
	// answer (HerdrMeta.MuxName), so neither may reach the file.
	for _, v := range []string{"", MuxHerdr} {
		if err := b.writeMeta(&HerdrMeta{Name: "h", Workspace: "9", Pane: "%1", Mux: v}); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(b.metaPath("h"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "mux:") {
			t.Errorf("Mux = %q wrote a mux: line (%q) — herdr is the absent key, and a record that spells it out is a record the next reader has to know two forms of", v, raw)
		}
	}
	// And a non-herdr occupant's name does reach the file, or claim 2 is a
	// writer that cannot write the key at all.
	if err := b.writeMeta(&HerdrMeta{Name: "r", Workspace: "01a1-de", Pane: "block-1", Mux: "rex"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.metaPath("r"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "mux: rex\n") {
		t.Errorf("a non-herdr record carries no mux: line: %q", raw)
	}
}

// ─── claim 3: liveness for a seat that is not herdr's ────────────────────────

// muxMetaRexBoard is the board claim 3 is read off: a stub occupant called
// "rex" holding one seat of its own, and a record for the session under
// test, stamped with that occupant and server and old enough to be past the
// prune grace.
//
// The seat the stub holds is NOT the session's: a listing has to be
// non-empty for the prune's own belt to stand down (emptyBoard), and the
// belt is the arm that would otherwise answer every subtest here.
func muxMetaRexBoard(t *testing.T, name, workspace string, alive bool) (*HerdrBackend, *muxMetaStub) {
	t.Helper()
	b, _ := newTestBackend(t)
	stub := &muxMetaStub{name: "rex", addr: "127.0.0.1:7777",
		seats: []SeatInfo{{Seat: Seat{Session: "other-uuid"}, Label: "somebody-else"}}}
	if alive {
		// Labelled with something this home would never write for the
		// name, which is herdr's stranger fence firing if it is asked at
		// all (notOurWorkspace, ADR 0076 §5).
		stub.seats = append(stub.seats, SeatInfo{Seat: Seat{Session: workspace}, Label: "renamed-in-rex"})
	}
	b.Slot = stub
	if err := b.writeMeta(&HerdrMeta{
		Name: name, Workspace: workspace, Pane: "block-1", Emoji: "🤠", Agent: "scout",
		Mux: "rex", Socket: "127.0.0.1:7777",
		Launched: time.Now().Add(-2 * PruneGrace),
	}); err != nil {
		t.Fatal(err)
	}
	return b, stub
}

func TestQAANonHerdrSeatIsDeadWhenAbsentFromItsOccupantsListing(t *testing.T) {
	t.Parallel()
	b, stub := muxMetaRexBoard(t, "gone", "01a1902c-dead", false)
	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("gone"); ok {
		t.Error("a seat absent from its own occupant's listing was not pruned — ADR 0076 §5 makes that listing the whole liveness rule for a non-herdr seat, and nothing else can ever prove it (herdr's per-id query is §4's and refuses the call)")
	}
	// It proved death by LISTING, more than once: the snapshot the pass came
	// in holding is not evidence about another store (rangerhq-9nso), so the
	// prune takes its own, and reclaim takes a third under the launcher lock.
	if stub.lists < 2 {
		t.Errorf("the prune called List() %d time(s) — it acted on the snapshot it was handed, which is the rangerhq-9nso damage shape", stub.lists)
	}
}

func TestQAANonHerdrSeatInItsListingIsKeptAndNeedsNoFence(t *testing.T) {
	t.Parallel()
	b, _ := muxMetaRexBoard(t, "live", "01a1902c-live", true)
	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("live"); !ok {
		t.Fatal("a seat its occupant is still holding had its record pruned")
	}
	var found bool
	for _, s := range sessions {
		if s.Name == "live" {
			found = true
		}
	}
	if !found {
		t.Error("a live non-herdr seat was withheld from the listing — its label is not the one this home writes, which is herdr's recycled-id fence being asked of ids that are never recycled (ADR 0076 §5). A withheld session holds its own name and its dispatch seat with nothing that can clear it.")
	}
}

// The other half of §5's rule, and the one that costs a file rather than a
// session: a pass whose occupant is not the one that wrote the record cannot
// answer for it at all. Two occupants' server addresses are not comparable,
// so the mux is asked first (cannotAnswerFor).
func TestQAAHerdrPassCannotAnswerForANonHerdrRecord(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	// A herdr board with one workspace of its own, so the listing is not
	// empty and the belt is not what answers.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "mine"}})
	if err := b.writeMeta(&HerdrMeta{Name: "mine", Workspace: "w1", Pane: "%1", Emoji: "🤠",
		Socket: SocketID(), Launched: time.Now().Add(-2 * PruneGrace)}); err != nil {
		t.Fatal(err)
	}
	// Two non-herdr records. The first is the ordinary shape, an address in
	// the other occupant's own spelling. The second carries THIS pass's own
	// socket, and it is the one that pins the arm: with the same address on
	// both sides, the socket comparison behind it has nothing to say, so
	// only "a different multiplexer wrote this" can refuse. An address is a
	// string inside one occupant's namespace and means nothing across two,
	// which is why the mux is asked first and not as a tie-break.
	for _, c := range []struct{ name, sock string }{
		{"onrex", "127.0.0.1:7777"},
		{"onrexsamesock", SocketID()},
	} {
		if err := b.writeMeta(&HerdrMeta{Name: c.name, Workspace: "01a1902c-de-" + c.name, Pane: "block-1", Emoji: "🤠",
			Mux: "rex", Socket: c.sock, Launched: time.Now().Add(-2 * PruneGrace)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"onrex", "onrexsamesock"} {
		if _, ok := b.readMeta(name); !ok {
			t.Errorf("%s: a herdr pass pruned a record written by another multiplexer — its own listing says nothing about that seat, and a delete on no evidence is the whole of rangerhq-snd", name)
		}
		if err := b.mustNotOrphan(name); err == nil {
			t.Errorf("%s: a herdr pass would overwrite a record written by another multiplexer — on the write side, proving nothing is a refusal (rangerhq-jeu2)", name)
		}
	}
}
