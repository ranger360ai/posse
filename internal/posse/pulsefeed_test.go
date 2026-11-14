//go:build posse_arm3

package posse

// The pulse's delta feed (ADR 0093 D2, ranger-base-hkqpq): a due tick appends
// ONE line to state/pulse.feed saying what moved — `+` appeared, `-` cleared,
// `~` a count that moved, `standing: N` — instead of re-sending the whole
// standing set the way the typed pulse does.
//
// WHY A DELTA AT ALL, measured rather than argued (ADR 0093, 70h of watch
// log to 2026-11-13): 321 deliveries, 296 of them caused by exactly one key
// appearing or clearing, and every one of those re-sent all seven or eight
// standing keys around the one that moved. The renag — an identical set
// re-sent on its interval — was 14 of the 321.
//
// Arm 3, beside pulse_test.go and pulsedeliverystate_qa_test.go, so a
// `-run 'Pulse|pulse'` over this package needs `-tags posse_arm3` to reach
// any of the three. The FILE SHAPE and old-record pins are next door in
// pulsedeliverystate_qa_test.go; delivery's own rules (dedup, renag, retry
// after a skip) in pulse_delivery_test.go. What is here is the line.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// feedEntry splits one feed entry into its sequence number and its body,
// checking the two fields in front of the body on the way through: an
// integer sequence and an RFC3339 timestamp, which is the whole of what
// `posse pulse wait --after <seq>` has to be able to read (ADR 0093 D1).
//
// Split on the first two spaces only — the body carries spaces of its own,
// and a helper that tokenized the lot would quietly pass a line whose
// `standing: 3` had become three fields.
func feedEntry(t *testing.T, entry string) (int, string) {
	t.Helper()
	f := strings.SplitN(entry, " ", 3)
	if len(f) != 3 {
		t.Fatalf("feed entry is not `<seq> <time> <body>`: %q", entry)
	}
	seq, err := strconv.Atoi(f[0])
	if err != nil {
		t.Fatalf("feed entry %q: first field is not a sequence number: %v", entry, err)
	}
	if _, err := time.Parse(time.RFC3339, f[1]); err != nil {
		t.Fatalf("feed entry %q: second field is not RFC3339: %v", entry, err)
	}
	// ...and the reading the waiter will actually use agrees with this one.
	if got, ok := PulseFeedSeq(entry); !ok || got != seq {
		t.Fatalf("PulseFeedSeq(%q) = %d, %v, want %d, true", entry, got, ok, seq)
	}
	return seq, f[2]
}

// wantFeed reads the feed and asserts its entries' bodies, in order. The
// sequence numbers are asserted too and are not an argument: a feed whose
// numbers do not rise by one per entry is one an armed waiter cannot follow.
func wantFeed(t *testing.T, b *HerdrBackend, want ...string) {
	t.Helper()
	entries := pulseFeedEntries(PulseFeedPath(b.App))
	if len(entries) != len(want) {
		t.Fatalf("feed has %d entries, want %d:\n%s", len(entries), len(want), strings.Join(entries, "\n"))
	}
	for i, entry := range entries {
		seq, body := feedEntry(t, entry)
		if seq != i+1 {
			t.Errorf("entry %d has seq %d, want %d: %q", i, seq, i+1, entry)
		}
		if body != want[i] {
			t.Errorf("entry %d body = %q, want %q", i, body, want[i])
		}
	}
}

// A delta, end to end through pulseOnce, over the three things that can
// happen to a key: it appears, another appears beside it, one of them
// clears.
//
// The load-bearing arm is the SECOND one, and it is the whole bead: the
// standing unpushed key must not be on that line. A feed that re-sent the
// set would pass every other assertion here.
func TestPulseFeedAppendsADeltaNotTheSet(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	cid := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	did := personaSession(t, b, fake, "developer-work", "developer", "idle", false)
	repo := unpushedRepo(t, b)
	// personaSession writes the listing per session, so the second call
	// dropped the first's row: put both back, both idle.
	setAgentStatuses(t, fake, agentState{cid, "idle"}, agentState{did, "idle"})

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}
	unpushed := "unpushed:" + repo + ":1"

	d.pulseOnce(cfg)
	wantFeed(t, b, "+"+unpushed+" standing: 1")

	// A second condition arrives one minute in — a new set, so due at once.
	setAgentStatuses(t, fake, agentState{cid, "idle"}, agentState{did, "blocked"})
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)
	wantFeed(t, b,
		"+"+unpushed+" standing: 1",
		"+blocked:developer-work standing: 2")

	// THE POINT, said as its own assertion so a failure names it: the key
	// that did not move is not on the line that reports the one that did.
	entries := pulseFeedEntries(PulseFeedPath(b.App))
	if _, body := feedEntry(t, entries[1]); strings.Contains(body, repo) {
		t.Errorf("the delta re-sent a standing key: %q", body)
	}

	// And it clears again.
	setAgentStatuses(t, fake, agentState{cid, "idle"}, agentState{did, "idle"})
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)
	wantFeed(t, b,
		"+"+unpushed+" standing: 1",
		"+blocked:developer-work standing: 2",
		"-blocked:developer-work standing: 1")

	// The record holds the KEYS now, not their fingerprint, and it holds
	// what the last line left standing.
	if s := ReadPulseState(PulsePath(b.App)); strings.Join(s.DeliveredKeys, "|") != unpushed {
		t.Errorf("delivered_keys = %q, want just %q", s.DeliveredKeys, unpushed)
	}
}

// A count that moves is ONE token and not a clear beside an appearance.
// `unpushed:<repo>:N` carries its count in the key, so a commit renames the
// condition: of the 321 deliveries ADR 0093 measured, 9 were nothing but
// such a move, and read as a `-`/`+` pair each one says a repo stopped
// having unpushed commits and then started again.
func TestPulseFeedCountMoveIsOneTildeToken(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	repo := unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)
	wantFeed(t, b, "+unpushed:"+repo+":1 standing: 1")

	commitIn(t, repo, "extra2.txt", "y", "extra2") // 1 unpushed commit → 2
	clock = clock.Add(time.Minute)                 // deep inside the renag window
	d.pulseOnce(cfg)
	wantFeed(t, b,
		"+unpushed:"+repo+":1 standing: 1",
		"~unpushed:"+repo+":1→2 standing: 1")

	// Said twice, because the pair is what this replaces and a renderer
	// that emitted both the tilde AND the pair would pass the line above
	// under a Contains.
	_, body := feedEntry(t, pulseFeedEntries(PulseFeedPath(b.App))[1])
	for _, never := range []string{"+unpushed", "-unpushed"} {
		if strings.Contains(body, never) {
			t.Errorf("a moved count must not also render %q: %q", never, body)
		}
	}
}

// The renag (ADR 0093 D2: "the renag keeps its one interval") is the one
// line that NAMES the standing keys, because its whole content is that
// nothing has changed — and "nothing has changed" with no subject is not a
// message. 14 of the 321 measured deliveries were this.
//
// The first arm is the control: a tick inside the window with nothing to say
// appends nothing at all. Without it this test would pass over a feed that
// wrote a line every tick.
func TestPulseFeedRenagSaysStillAndNamesTheKeys(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	repo := unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}
	unpushed := "unpushed:" + repo + ":1"

	d.pulseOnce(cfg)
	wantFeed(t, b, "+"+unpushed+" standing: 1")

	clock = clock.Add(time.Minute) // unchanged, and nowhere near the interval
	d.pulseOnce(cfg)
	wantFeed(t, b, "+"+unpushed+" standing: 1")

	clock = clock.Add(30 * time.Minute) // the interval elapses
	d.pulseOnce(cfg)
	wantFeed(t, b,
		"+"+unpushed+" standing: 1",
		"still: "+unpushed)
}

// A partial read is never a clear (ADR 0093 D2): a delivered key that this
// tick could not see is HELD — carried in the record and named by the next
// line that names keys — not marked `-`.
//
// The lever is bd, which is the store of record for G2/G3/G9: an
// unresolvable bd with beads dirs configured makes ShopCheck report
// "G2/G3/G9 unknown" and the tick PARTIAL, while herdr still answers, so
// unlike the whole-shop refusal in pulsedeliverystate_qa_test.go this tick
// still has a condition, still has a live target, and still delivers. That
// is the tick the rule is about.
//
// The planted record is what makes the hold visible: a key from the store
// that went dark, which this tick cannot compute and must not bury.
func TestPulseFeedPartialReadHoldsRatherThanClears(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	repo := unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	good := d.Bd
	// An absolute path that does not exist, rather than a zero Bd: the
	// resolver searches PATH for a bare name, and this box HAS a bd.
	d.Bd = Bd{Bin: filepath.Join(t.TempDir(), "no-such-bd")}
	if d.Bd.Available() {
		t.Fatalf("fixture: %s resolved", d.Bd.Bin)
	}
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	unpushed := "unpushed:" + repo + ":1"
	gone := "question:ranger-base-gone"
	// PromptedAt stays zero so the tick is due on its own, whatever the
	// delta turns out to be.
	if err := WritePulseState(PulsePath(b.App), PulseState{
		DeliveredKeys: []string{gone, unpushed},
	}); err != nil {
		t.Fatal(err)
	}

	d.pulseOnce(cfg)

	// Nothing moved that this tick could see, so the line is the renag's —
	// and it names BOTH keys, the held one included.
	wantFeed(t, b, "still: "+gone+"; "+unpushed)
	// By TOKEN, not by substring: bead ids carry hyphens, and a Contains
	// here would have read `ranger-base-gone` as a clear.
	_, body := feedEntry(t, pulseFeedEntries(PulseFeedPath(b.App))[0])
	for _, tok := range strings.Fields(body) {
		if strings.HasPrefix(tok, "-") {
			t.Errorf("a partial tick must not mark anything cleared: %q in %q", tok, body)
		}
	}
	if s := ReadPulseState(PulsePath(b.App)); s.Fingerprint() != gone+"|"+unpushed {
		t.Errorf("delivered_keys = %q, want the held key kept alongside what was seen", s.DeliveredKeys)
	}

	// ...and the hold ENDS. With the store readable the key really is gone,
	// and it clears — which is the arm that keeps this from passing for "a
	// key is never cleared at all".
	d.Bd = good
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)
	wantFeed(t, b,
		"still: "+gone+"; "+unpushed,
		"-"+gone+" standing: 1")
}

// The cap, over pulseFeedWrite itself rather than 500 ticks of a watch loop:
// the newest PulseFeedLines entries are kept and the SEQUENCE DOES NOT
// RESTART. A cap that renumbered would hand an armed waiter `--after 505`
// and then a feed counting up from 1, which looks exactly like a shop where
// nothing is happening.
func TestPulseFeedKeepsTheNewestEntriesAndKeepsCounting(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pulse.feed")
	at := time.Now()
	const over = 5
	for i := 1; i <= PulseFeedLines+over; i++ {
		seq, err := pulseFeedWrite(path, at, fmt.Sprintf("+k%d standing: 1", i))
		if err != nil {
			t.Fatal(err)
		}
		if seq != i {
			t.Fatalf("write %d got seq %d", i, seq)
		}
	}

	entries := pulseFeedEntries(path)
	if len(entries) != PulseFeedLines {
		t.Fatalf("feed kept %d entries, want the newest %d", len(entries), PulseFeedLines)
	}
	oldest, _ := feedEntry(t, entries[0])
	newest, body := feedEntry(t, entries[len(entries)-1])
	if oldest != over+1 {
		t.Errorf("oldest kept seq = %d, want %d — the cap must drop from the FRONT", oldest, over+1)
	}
	if newest != PulseFeedLines+over {
		t.Errorf("newest seq = %d, want %d", newest, PulseFeedLines+over)
	}
	if want := fmt.Sprintf("+k%d standing: 1", PulseFeedLines+over); body != want {
		t.Errorf("newest body = %q, want %q", body, want)
	}
	// The rewrite is atomic, so it leaves nothing half-written beside it for
	// a waiter to trip over.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("the temp file survived the rename: %v", err)
	}
}

// A feed whose tail cannot be read must not restart the sequence: the next
// number is one past the HIGHEST readable one, not one past the last line.
// The failure this rules out is silent on both sides — the writer is happy
// and the waiter simply never wakes again.
func TestPulseFeedSequenceSurvivesACorruptTail(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pulse.feed")
	at := time.Now()
	for i := 1; i <= 3; i++ {
		if _, err := pulseFeedWrite(path, at, "+k standing: 1"); err != nil {
			t.Fatal(err)
		}
	}
	// A half-written line, the shape a kill between write and rename could
	// leave on a reader's disk if the rename were not there.
	entries := pulseFeedEntries(path)
	if err := os.WriteFile(path, []byte(strings.Join(entries, "\n")+"\nnot-a-seq whatever\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	seq, err := pulseFeedWrite(path, at, "+k4 standing: 1")
	if err != nil {
		t.Fatal(err)
	}
	if seq != 4 {
		t.Errorf("next seq = %d, want 4 — one past the highest READABLE entry", seq)
	}

	// ...and the arm that separates "highest readable" from "the last line
	// that parsed", which the corrupt tail above does NOT (ranger-base-xim7t,
	// verifying ranger-base-hkqpq). An unparseable tail is skipped by BOTH
	// rules, so both answer 4 over the fixture above and the `>= seq` half of
	// pulseFeedWrite's guard was asserted by nothing: MEASURED 2026-11-14,
	// cutting it to a bare `if n, ok := PulseFeedSeq(ln); ok` left this whole
	// file green.
	//
	// What tells them apart is a PARSEABLE line whose number is LOWER than one
	// above it. Out of order rather than corrupt, so there is nothing for a
	// parse to reject, and last-line-wins then re-issues a number an armed
	// waiter has already passed — which is this test's own stated failure, the
	// one that is silent on both sides.
	outOfOrder := []string{
		"1 " + at.UTC().Format(time.RFC3339) + " +k1 standing: 1",
		"2 " + at.UTC().Format(time.RFC3339) + " +k2 standing: 2",
		"5 " + at.UTC().Format(time.RFC3339) + " +k5 standing: 3",
		"3 " + at.UTC().Format(time.RFC3339) + " -k3 standing: 2",
	}
	if err := os.WriteFile(path, []byte(strings.Join(outOfOrder, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seq, err = pulseFeedWrite(path, at, "+k6 standing: 3")
	if err != nil {
		t.Fatal(err)
	}
	if seq != 6 {
		t.Errorf("over entries 1, 2, 5, 3 the next seq = %d, want 6 — one past the HIGHEST readable entry and not one past the last one (which would re-issue 4, a number a waiter on --after 5 has already passed)", seq)
	}
}

// The rewrite goes through `path + ".tmp"` and a rename, which is the one
// property the feed has for the sake of a reader that is not its writer:
// `posse pulse wait` may be mid-read at any moment, and a reader of a feed
// written in place can see a file with its oldest lines gone and its newest
// not yet there (ADR 0093 D1/D2, ranger-base-cgr1h is the waiter).
//
// Pinned here because the cap test's `os.Stat(path + ".tmp")` cannot pin it:
// that is an ABSENCE, and a writer which creates no temp file at all
// satisfies it. MEASURED 2026-11-14 (ranger-base-xim7t, verifying
// ranger-base-hkqpq): replacing the temp+rename with a direct
// `os.WriteFile(path, …)` left every pulse pin in all three arms green.
//
// So the control is positive and it is the temp path itself: a DIRECTORY
// planted at `path + ".tmp"` is a path os.WriteFile cannot write, so a
// writer that goes through it fails and a writer that does not succeeds.
// The second half is the atomicity the rename buys — a write that failed
// leaves the reader's file exactly as it was, never half-replaced.
func TestPulseFeedRewriteGoesThroughATempFileAndARename(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pulse.feed")
	at := time.Now()
	if _, err := pulseFeedWrite(path, at, "+k1 standing: 1"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	seq, err := pulseFeedWrite(path, at, "+k2 standing: 2")
	if err == nil {
		t.Errorf("pulseFeedWrite returned seq %d and no error with a directory at %s — the rewrite is not going through that temp path, so nothing stands between a mid-read waiter and a half-replaced file", seq, filepath.Base(path+".tmp"))
	}

	// And the rename's own guarantee: the failed write left the feed whole.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the feed itself did not survive a failed rewrite: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("a failed rewrite changed the feed:\n before %q\n after  %q", before, after)
	}

	// With the temp path writable again the same call goes through, so the
	// arm above failed on the plant and not on something about the fixture.
	if err := os.Remove(path + ".tmp"); err != nil {
		t.Fatal(err)
	}
	if seq, err = pulseFeedWrite(path, at, "+k2 standing: 2"); err != nil || seq != 2 {
		t.Fatalf("pulseFeedWrite after the plant was removed = %d, %v; want 2, nil", seq, err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("the temp file survived the rename: %v", err)
	}
}

// pulseDiff's own rules, at the level they live, because the pairing is the
// part with cases no end-to-end fixture can cheaply reach: a counted key
// with no partner, two counted keys under different stems, and what a
// partial tick does to each outcome.
func TestPulseDiffPairsCountsAndHoldsOnPartial(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                    string
		delivered, now          []string
		partial                 bool
		appeared, cleared, held []string
		moved                   []pulseMove
	}{{
		name:     "first delivery is all appeared",
		now:      []string{"a", "b"},
		appeared: []string{"a", "b"},
	}, {
		name:      "a count that moved is a move and nothing else",
		delivered: []string{"unpushed:/r:5"},
		now:       []string{"unpushed:/r:7"},
		moved:     []pulseMove{{Stem: "unpushed:/r", From: "5", To: "7"}},
	}, {
		name:      "a counted key with no partner is an ordinary appearance",
		delivered: []string{"a"},
		now:       []string{"a", "unpushed:/r:7"},
		appeared:  []string{"unpushed:/r:7"},
	}, {
		name:      "counted keys under DIFFERENT stems do not pair",
		delivered: []string{"unpushed:/one:5"},
		now:       []string{"unpushed:/two:7"},
		appeared:  []string{"unpushed:/two:7"},
		cleared:   []string{"unpushed:/one:5"},
	}, {
		name:      "a key with no trailing number never pairs",
		delivered: []string{"blocked:alice"},
		now:       []string{"blocked:bob"},
		appeared:  []string{"blocked:bob"},
		cleared:   []string{"blocked:alice"},
	}, {
		name:      "partial holds what vanished and still reports what appeared",
		delivered: []string{"a", "gone"},
		now:       []string{"a", "fresh"},
		partial:   true,
		appeared:  []string{"fresh"},
		held:      []string{"gone"},
	}, {
		name:      "partial does not hold a move: both its halves were observed",
		delivered: []string{"unpushed:/r:5"},
		now:       []string{"unpushed:/r:7"},
		partial:   true,
		moved:     []pulseMove{{Stem: "unpushed:/r", From: "5", To: "7"}},
	}, {
		// The pairing is a MATCHING: one gone key cannot be the `from` of
		// two moves. No set this tree computes can hold two counted keys
		// under one stem (`hook-wall:<slot>:<config>` carries a config
		// path), so this case is planted — the rule is generic and this is
		// what generic has to mean.
		name:      "one gone key is consumed by one move, not by both",
		delivered: []string{"k:1"},
		now:       []string{"k:2", "k:3"},
		appeared:  []string{"k:3"},
		moved:     []pulseMove{{Stem: "k", From: "1", To: "2"}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := pulseDiff(tc.delivered, tc.now, tc.partial)
			eq := func(what string, got, want []string) {
				if strings.Join(got, "|") != strings.Join(want, "|") {
					t.Errorf("%s = %q, want %q", what, got, want)
				}
			}
			eq("appeared", d.Appeared, tc.appeared)
			eq("cleared", d.Cleared, tc.cleared)
			eq("held", d.Held, tc.held)
			if fmt.Sprint(d.Moved) != fmt.Sprint(tc.moved) {
				t.Errorf("moved = %v, want %v", d.Moved, tc.moved)
			}
			// Empty is the dedup answer, so it must ignore Held and nothing
			// else: a tick whose only news is a hold has nothing to say.
			wantEmpty := len(tc.appeared) == 0 && len(tc.cleared) == 0 && len(tc.moved) == 0
			if d.Empty() != wantEmpty {
				t.Errorf("Empty() = %v, want %v for %+v", d.Empty(), wantEmpty, d)
			}
		})
	}
}
