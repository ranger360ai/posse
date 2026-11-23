//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-nqtz5 (from ranger-base-56gd0's verify of
// ranger-base-p51zu): G19's exclusion rule is written twice and one witness
// covered the pair, so either copy deleted green.
//
// THE TWO COPIES. The Foreign / Crew / beadless exclusion is in the walk
// (govern.go's G19 block) and in the predicate (unpromptedSeat). MEASURED
// 2026-11-22 (linux 7.2.15-1-omarchy, go1.27.3, main at 814776ed), each
// mutant run alone with the file restored from a checksummed golden copy,
// filter `-run '(TestGovG19|TestQATheSettleTimeout|TestQAASettleTimeout|
// TestQABothSettleWindow|TestQAARexWaitTimeout)' ./internal/posse`:
//
//	M3  unpromptedSeat's own `case s.Foreign, s.Crew:` and
//	    `case s.Bead == "":` arms deleted (14 lines)          -> GREEN
//	M2  govern.go's `if s.Foreign || s.Crew || s.Bead == ""
//	    { continue }` deleted (3 lines)                       -> GREEN
//	M4  control, so the file is not simply unread:
//	    unpromptedGrace 15m -> 1ns                            -> RED
//
// TestGovG19CrewAndBeadlessSeatsRaiseNothing is written as the pin for the
// two populations that are somebody else's, but it goes through ShopCheck, so
// it grades the COMPOSITE and neither copy alone. unpromptedSeat's own
// docstring claims it "is the predicate alone, which is what makes it the
// same answer twice" — that sentence was the thing no run tested.
//
// AND THE TWO COPIES NEED TWO DIFFERENT KINDS OF WITNESS, which is why this
// file holds a behavioural pin and a source one.
//
// The predicate's copy is behaviour: call it directly and a deleted arm
// answers differently. The walk's copy is NOT behaviour at all — delete it
// and readMeta is called for every listed session, the predicate still
// excludes all three populations, and every output byte is identical. Its
// whole effect is the cost its own comment states (ONE FILE READ PER
// CANDIDATE), so no behavioural pin anywhere can red M2 and the walk is
// witnessed by being READ or by nothing. That is the same key as this
// package's other source censuses ($(QA_PKGCENSUS_PINS)).
//
// Neither copy is wrong and neither should go: the walk's is a cost guard
// with a stated reason, and the predicate's exclusions are what make it safe
// for a second caller.
//
// ─── AND THE PREDICATE PIN IS THE MERGE OF TWO (ranger-base-zpagn F7) ───────
//
// govern_test.go carried `TestGovG19ThePredicateItselfExcludesEachPopulation`
// and `TestGovG19ThePredicateAbstainsOnARecordItCannotAge`
// (ranger-base-56gd0), landed 25 minutes before this file in the same
// lineage and making the same claim about the same predicate. MEASURED
// 2026-11-23, M3 again — unpromptedSeat's own `case s.Foreign, s.Crew:` and
// `case s.Bead == "":` arms deleted, 14 lines — RED on BOTH copies, with the
// four older ShopCheck G19 pins green beside them. Two pins over one claim
// with neither naming the other is the drift this house counts, so there is
// one, and it is this one: the other two were forced SERIAL by reading the
// written package var `govNow` (their own comments say so) and sat in an
// UNTAGGED file, so one claim about a pure predicate was graded three times
// per `make test`, once per arm.
//
// It carries the arms they had that this one did not: an excluded seat raises
// NOTHING rather than a blank row (unpromptedRaisesNothing, below — a claim
// about the walk, which hands whatever comes back to `add`), and a record
// with no launch clock to AGE is an abstention of its own.

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ─── the predicate alone (M3) ────────────────────────────────────────────────

// unpromptedRow is one candidate as the walk hands it in: the listing's own
// fields, and the record the caller read for it.
func unpromptedRow(s HerdrSession, launched, prompted time.Time) (HerdrSession, *HerdrMeta) {
	return s, &HerdrMeta{Name: s.Name, Launched: launched, Prompted: prompted}
}

// unpromptedSeat is G19's row for ONE session, asked with no ShopCheck
// between the arms and the answer — which is the claim its docstring makes
// and the one the four G19 pins could not reach, because all four grade the
// composite.
//
// Every arm is a seat that is past the grace with no prompt stamp, so the
// POSITIVE row is what the three exclusions are each turning off. Without
// that control these are three ways of printing nothing, and a predicate
// that returned false for everything would satisfy them all.
func TestGovG19PredicateAloneExcludesTheThreePopulations(t *testing.T) {
	t.Parallel()
	// Its own clock, and not the shared one the four ShopCheck pins read
	// through govIn: unpromptedSeat takes `now` as a parameter and reads
	// nothing, so a test of the predicate alone has no reason to reach a
	// package var — and a parallel test that reaches a WRITTEN one is what
	// cmd/testparallel has to clear first (its parallelOK map names the
	// reader, so a new reader is a new line there). The instant is the
	// incident's own, 01:21Z, when both live records were read.
	now := time.Date(2026, 11, 23, 1, 21, 0, 0, time.UTC)
	launched := now.Add(-20 * time.Minute)
	base := HerdrSession{Name: "developer-x", Agent: "developer", Bead: "bd-1"}

	// THE POSITIVE ROW, first, because it is the discriminator for the three
	// below: this is the record pair the live seats showed — `bead:` set,
	// `prompted:` absent, `launched:` old.
	s, m := unpromptedRow(base, launched, time.Time{})
	c, yes := unpromptedSeat(s, m, now)
	if !yes {
		t.Fatalf("the predicate does not raise its own population — the three arms below cannot "+
			"tell an exclusion from a predicate that answers false for everything: %+v", c)
	}
	if c.ID != "G19" || c.Key != "unprompted:developer-x" || c.Class != GovLane {
		t.Errorf("predicate row = %+v, want G19 / unprompted:developer-x / LANE", c)
	}

	for _, arm := range []struct {
		name string
		mut  func(*HerdrSession)
		why  string
	}{
		{
			"foreign", func(s *HerdrSession) { s.Foreign = true },
			"a foreign session has no record of ours to read, so there is no `prompted:` to be absent",
		},
		{
			"crew", func(s *HerdrSession) { s.Crew = true },
			"ADR 0008 — the operator's own session does not exist as far as any sweep is concerned, " +
				"and an unprompted one is a conversation waiting for them to type",
		},
		{
			"beadless", func(s *HerdrSession) { s.Bead = "" },
			"nobody ever aimed this session at a bead, and a seat that was never aimed is not a seat " +
				"that missed its prompt — the stampless population is the reaper's (autoreap.go)",
		},
	} {
		t.Run(arm.name, func(t *testing.T) {
			sess := base
			arm.mut(&sess)
			s, m := unpromptedRow(sess, launched, time.Time{})
			c, yes := unpromptedSeat(s, m, now)
			if yes {
				t.Errorf("unpromptedSeat raises G19 over a %s seat: %s.\n"+
					"The exclusion is in the predicate as well as in govern.go's walk ON PURPOSE — it "+
					"is what makes the predicate safe for a second caller, and the docstring's claim "+
					"that it 'is the predicate alone, which is what makes it the same answer twice' "+
					"is this test (ranger-base-nqtz5 F2, mutant M3). Row: %+v",
					arm.name, arm.why, c)
				return
			}
			unpromptedRaisesNothing(t, c)
		})
	}

	// The exclusions that are NOT about population — kept here beside them
	// because the predicate's answer is one switch and a reordering that
	// broke these would otherwise be graded only through ShopCheck.
	t.Run("a record with the stamp", func(t *testing.T) {
		s, m := unpromptedRow(base, launched, launched.Add(time.Minute))
		c, yes := unpromptedSeat(s, m, now)
		if yes {
			t.Errorf("a seat whose record carries prompted: raises G19 — any launcher's prompt counts, "+
				"the hand `posse prompt` included (ADR 0011 §3): %+v", c)
			return
		}
		unpromptedRaisesNothing(t, c)
	})
	t.Run("no record", func(t *testing.T) {
		c, yes := unpromptedSeat(base, nil, now)
		if yes {
			t.Errorf("a session with no readable record raises G19 — `prompted:` absent and the whole "+
				"record absent are not the same claim (ranger-base-82e40): %+v", c)
		} else {
			unpromptedRaisesNothing(t, c)
		}
		s, m := unpromptedRow(base, launched, time.Time{})
		m.Unreadable = true
		c, yes = unpromptedSeat(s, m, now)
		if yes {
			t.Errorf("an unreadable record raises G19: %+v", c)
			return
		}
		unpromptedRaisesNothing(t, c)
	})
	// A RECORD WITH NO LAUNCH CLOCK TO AGE, the retired copy's third
	// abstention (see the merge note in this file's head comment): the
	// predicate's condition is an AGE, so a record whose `launched:` is the
	// zero time is one it cannot date — and `now.Sub(zero)` is tens of
	// thousands of hours, which is past every grace there will ever be. This
	// is the arm that says the abstention is deliberate and not arithmetic.
	t.Run("a record with no launch stamp", func(t *testing.T) {
		c, yes := unpromptedSeat(base, &HerdrMeta{Name: base.Name}, now)
		if yes {
			t.Errorf("a record with no launch stamp raises G19 — the condition is the AGE of a launch "+
				"and there is no launch here to age, so this is the zero time read as 50 years past the "+
				"grace (unprompted.go's own abstention): %+v", c)
			return
		}
		unpromptedRaisesNothing(t, c)
	})
}

// unpromptedRaisesNothing is the retired copy's own arm, kept because it is a
// claim about the WALK and not about the predicate's bool: ShopCheck hands
// whatever comes back to `add`, so an excluded seat that still carries a row
// is a blank G19 on the shop set rather than no G19 at all. The bool and the
// row are two returns and only one of them was ever asserted here.
func unpromptedRaisesNothing(t *testing.T, c GovCondition) {
	t.Helper()
	if c != (GovCondition{}) {
		t.Errorf("excluded and still carrying a row: %+v — ShopCheck's walk hands whatever comes back "+
			"to `add`, so a non-zero condition beside a false answer is a blank row on the shop set", c)
	}
}

// ─── the stamp the hand path writes, read by both readers (F1's other half) ──

// What cmd/posse's TestQAHandPromptStampsThePromptedFieldOnTheRecord cannot
// reach from package main: that a `prompted:` stamp of the shape the hand
// `posse prompt` writes is the one BOTH readers ranger-base-p51zu's close
// names actually read. That pin holds the verb leaves the stamp in the
// record; this one holds the stamp stands both readers down. Together they
// are the chain, and neither end needs the other's fixture.
func TestMarkPromptedSatisfiesBothReadersOfTheStamp(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	// G19's own row going in: a launch old enough to be past the grace and
	// no prompt stamp under it.
	govLaunchStamps(t, b, "developer-x", time.Now().Add(-20*time.Minute), time.Time{})

	d := newTestDispatcher(t, b)
	// CONTROL: both readers say what the incident said before the stamp is
	// written, or neither assertion below is about the write.
	m, ok := b.readMeta("developer-x")
	if !ok {
		t.Fatal("no meta for developer-x")
	}
	if _, yes := unpromptedSeat(HerdrSession{Name: "developer-x", Agent: "developer", Bead: "bd-1"}, m, time.Now()); !yes {
		t.Fatal("CONTROL: the unstamped record does not raise G19 — this fixture is not the row")
	}
	if _, recent := d.promptedRecently("developer-x"); recent {
		t.Fatal("CONTROL: an unstamped record reads as prompted recently — this fixture is not the row")
	}

	b.MarkPrompted("developer-x", time.Now())

	// Reader 1: G19 stands down, so the remedy its own Detail prints
	// (`posse prompt <session>`) can clear the condition.
	m, ok = b.readMeta("developer-x")
	if !ok {
		t.Fatal("no meta for developer-x after the stamp")
	}
	if c, yes := unpromptedSeat(HerdrSession{Name: "developer-x", Agent: "developer", Bead: "bd-1"}, m, time.Now()); yes {
		t.Errorf("G19 still raises over a seat whose record was just stamped — the measured recovery "+
			"cannot clear its own row: %+v", c)
	}
	// Reader 2: a pass in ANOTHER process sees the prompt, which is the
	// whole reason the stamp is persisted (rangerhq-tzdf's remaining half).
	// A second Dispatcher, so the answer cannot come from the in-process
	// map the first one would hold.
	other := newTestDispatcher(t, b)
	if _, recent := other.promptedRecently("developer-x"); !recent {
		t.Error("a second Dispatcher does not read the stamp as a recent prompt — the cross-process " +
			"half of PromptGrace (ADR 0011 §3) is the reason the record carries it at all, and a pass " +
			"that cannot see it types a second instruction into a seat mid-turn")
	}
}

// ─── the walk's cost guard (M2) ──────────────────────────────────────────────

// The three free clauses: fields the SESSION LISTING already carries, so
// testing them costs nothing and each one that passes is a record not
// opened. Named as fields rather than as a line, because the guard being one
// `if` with three `||` is a spelling and three separate `continue`s would be
// the same guard.
var g19FreeClauses = []string{"s.Foreign", "s.Crew", "s.Bead"}

// The expensive call the guard stands in front of, and the predicate that
// identifies which walk this is. `readMeta` is assembled so this file is not
// its own first hit if the needle is ever widened to the package's tests.
const (
	g19MetaRead  = "read" + "Meta("
	g19WalkOwner = "unpromptedSeat("
)

var g19WalkHead = regexp.MustCompile(`^\tfor .*\brange sessions\b.*\{$`)

// govern.go's G19 walk tests every free clause, and skips, before it opens a
// record.
//
// A SOURCE PIN AND NOT A BEHAVIOURAL ONE, deliberately. The walk's copy of
// the exclusion changes no output: with it deleted the predicate still
// excludes all three populations and ShopCheck returns the identical set, so
// the only thing the guard buys is the file read it avoids — eight records
// per pass on a full box against one per listed session, at the fleet's own
// cadence. A guard whose deletion is invisible to every run is held by
// reading the walk or by nothing at all (ranger-base-nqtz5 F2, mutant M2).
//
// Its door is the same as the package's other source censuses — the pins are
// read off ONE named file in this package's own directory, so nothing outside
// internal/posse is in the radius and no tree-wide door is owed.
func TestQAG19WalkTestsTheFreeClausesBeforeItOpensARecord(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("govern.go")
	if err != nil {
		t.Fatal(err)
	}
	block, at, ok := g19WalkBlock(string(src))
	if !ok {
		t.Fatalf("no `for … range sessions` block in govern.go calls %s — this pin has stopped "+
			"reading what it is about (the G19 walk moved, or the predicate was renamed)", g19WalkOwner)
	}
	// Control: the subject of the pin is a guard standing in FRONT of a
	// read, so a block with no read in it is a reading that has broken and
	// not a walk that got cheaper.
	if !strings.Contains(strings.Join(block, "\n"), g19MetaRead) {
		t.Fatalf("govern.go:%d's G19 walk opens no record at all — if the per-candidate read really "+
			"has gone, this pin has no subject and should go with it", at)
	}
	for _, miss := range g19WalkUnguarded(block) {
		t.Errorf("govern.go:%d: the G19 walk reaches %s at line %d without having tested %s first.\n"+
			"That clause is carried by the session LISTING already in hand, so testing it costs "+
			"nothing and every seat it skips is a record not opened — `ONE FILE READ PER CANDIDATE, "+
			"and only for a session that passes the free clauses first` (govern.go's own comment). "+
			"The same exclusion in unpromptedSeat is what makes the predicate safe for a second "+
			"caller, NOT this guard's witness: deleting this guard changes no output anywhere, so "+
			"nothing but this pin reds for it (ranger-base-nqtz5 F2, mutant M2).",
			at+miss.readAt, g19MetaRead, at+miss.readAt, miss.clause)
	}

	// The reading against text this test owns, so a needle edited to green
	// the census reds here instead. Three fixtures: the walk as it stands,
	// the mutant, and the reordering.
	guarded := []string{
		"\tfor _, s := range sessions {",
		"\t\tif s.Foreign || s.Crew || s.Bead == \"\" {",
		"\t\t\tcontinue",
		"\t\t}",
		"\t\tm, ok := in.HB." + g19MetaRead + "s.Name)",
		"\t\tif !ok {",
		"\t\t\tcontinue",
		"\t\t}",
		"\t\tif c, yes := " + g19WalkOwner + "s, m, now); yes {",
		"\t\t\tadd(c.ID, c.Class, c.Key, c.Detail)",
		"\t\t}",
		"\t}",
	}
	if got := g19WalkUnguarded(guarded); len(got) != 0 {
		t.Fatalf("the reading calls the guarded walk unguarded: %+v", got)
	}
	// M2 itself: the three lines gone.
	if got := g19WalkUnguarded(append(guarded[:1:1], guarded[4:]...)); len(got) != len(g19FreeClauses) {
		t.Fatalf("the reading does not see M2 (the guard deleted): %d misses, want %d",
			len(got), len(g19FreeClauses))
	}
	// And the other way the guard stops guarding without being deleted: the
	// read moved above it. A pin that only catches the delete would go green
	// over a walk that pays for every record and skips afterwards.
	reordered := append([]string{guarded[0], guarded[4]}, guarded[1:4]...)
	reordered = append(reordered, guarded[5:]...)
	if got := g19WalkUnguarded(reordered); len(got) != len(g19FreeClauses) {
		t.Fatalf("the reading does not see a read hoisted above the guard: %d misses, want %d",
			len(got), len(g19FreeClauses))
	}
	// The negative case the clause list must not be satisfied by: a block
	// that MENTIONS the fields without skipping on them is not a guard.
	mentions := append([]string{guarded[0],
		"\t\t_ = s.Foreign || s.Crew || s.Bead == \"\""}, guarded[4:]...)
	if got := g19WalkUnguarded(mentions); len(got) != len(g19FreeClauses) {
		t.Fatalf("a block that names the clauses and never skips reads as guarded: %d misses, want %d",
			len(got), len(g19FreeClauses))
	}
}

// ─── the census's reading of the walk ────────────────────────────────────────

// g19WalkBlock returns the lines of the `for … range sessions` block that
// owns the G19 predicate, and the 1-indexed file line its head is on.
//
// Brace counting from the head rather than a parse: in gofmt'd source a `for`
// at one tab opens the block and `}` at one tab closes it, and `make
// fmt-check` is a prerequisite of `make test`.
func g19WalkBlock(src string) (block []string, at int, ok bool) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if !g19WalkHead.MatchString(line) {
			continue
		}
		var body []string
		for j := i; j < len(lines); j++ {
			body = append(body, lines[j])
			if lines[j] == "\t}" {
				break
			}
		}
		if strings.Contains(strings.Join(body, "\n"), g19WalkOwner) {
			return body, i + 1, true
		}
	}
	return nil, 0, false
}

// g19WalkMiss is one free clause the walk did not test before its read, with
// the block-relative line the read is on.
type g19WalkMiss struct {
	clause string
	readAt int
}

// g19WalkUnguarded reads the block and reports the free clauses that are not
// SKIPPED ON before the record is opened.
//
// A clause is guarded when the block holds a line naming it, at or before the
// read, with a `continue` at or after that line and still before the read —
// which is "tested and skipped on", in any spelling: one `if` with three
// `||`, three separate ifs, or a switch. A line that merely names the field
// is not a guard, which is what the `continue` is doing in the test.
func g19WalkUnguarded(block []string) []g19WalkMiss {
	read := -1
	for i, line := range block {
		if strings.Contains(line, g19MetaRead) {
			read = i
			break
		}
	}
	if read < 0 {
		return nil
	}
	var missing []g19WalkMiss
	for _, clause := range g19FreeClauses {
		guarded := false
		for i := 0; i < read && !guarded; i++ {
			if !strings.Contains(block[i], clause) {
				continue
			}
			for j := i; j < read; j++ {
				if strings.Contains(block[j], "continue") {
					guarded = true
					break
				}
			}
		}
		if !guarded {
			missing = append(missing, g19WalkMiss{clause: clause, readAt: read})
		}
	}
	return missing
}
