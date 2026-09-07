package posse

// QA pins for ranger-base-jj2ax — governance row G10, the live-box
// verify-box verdict, and the freshness rule that is the whole point of it.
//
// THE DEFECT THESE HOLD. scripts/verify-box.sh runs the checks that assert
// what this MACHINE is, and until this bead it printed to whoever typed it
// and nothing else. A schedule could be installed and stop, or come back red
// every night, and the only surface that would have said so was a terminal
// somebody closed — the same shape the script itself exists for one level
// down (ranger-base-51z8j: a control nobody notices is unrun).
//
// So the three pins the operator's ruling names, and they are three because
// they fail independently:
//
//	RED     a planted red verdict raises G10 and names the check
//	STALE   a planted OLD timestamp raises G10 whatever the verdict said
//	GREEN   only a verdict that is BOTH fresh and clean raises nothing
//
// The third is the control for the first two, and it is not decoration: a
// row that fired unconditionally would pass RED and STALE and be worthless,
// and a row that never fired would pass GREEN and be worthless the other
// way. Every pin below carries the arm that must come out the other way.
//
// AND THE FRESHNESS RULE IS RANKED ABOVE THE VERDICT, deliberately. A
// two-week-old file saying "7 ok" is not seven ok checks, it is seven checks
// nobody has run since; rendering its greens as today's is the exact
// laundering the ruling forbade ("a verdict older than the schedule interval
// renders STALE/unknown, never green"). TestStaleWinsOverAGreenVerdict is
// that ordering, pinned as an ordering rather than as two independent facts.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// vbNow is this file's clock: a fixed instant, so an age assertion is not a
// stopwatch race. It is a FUNCTION and not a package var on purpose — every
// pin below runs t.Parallel, and cmd/testparallel reads a package-level
// time.Time as shared state a parallel test must be cleared for. Declaring
// this file's own clock as a call costs nothing and keeps eleven clearance
// lines out of a list whose whole value is that each line was read.
//
// The one pin that does NOT use it is TestG10ReachesTheGovernanceSet: it goes
// through govIn, which injects vbNow() into ShopCheck, and a fixture whose
// clock differs from the one under test is a fixture measuring the gap.
func vbNow() time.Time { return time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC) }

// vbRig is an instance whose verify-box state file says what the caller
// wants. It writes the FILE and not a run: the state file is the whole
// store, the reader under test opens nothing else, and a rig that shelled
// out to the real script would be pinning the script's roster instead of
// this reader's rules.
//
// The BYTES are the script's own format, not a convenience shape — an `at:`
// line, an `rc:` line and a one-level `checks:` map, exactly as write_state
// emits them. A fixture that plants what the producer does not render pins
// nothing about the pair; TestTheScriptWritesWhatThisReaderParses below runs
// the real script and hands its output to the real reader.
func vbRig(t *testing.T, config string, at time.Time, rc int, checks ...string) *App {
	t.Helper()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, config)
	body := fmt.Sprintf("at: %s\nrc: %d\nchecks:\n", at.UTC().Format(time.RFC3339), rc)
	for _, c := range checks {
		body += "  " + c + "\n"
	}
	write(t, VerifyBoxStatePath(a), body)
	return a
}

// vbRow is the single G10 row a reading raises, or nil. Every pin here reads
// through GovRows rather than through a hand-built condition, because the
// KEY and the CLASS are what the pulse and the cockpit consume.
func vbRow(r VerifyBoxReading, keyPrefix string) *GovCondition {
	for _, c := range r.GovRows() {
		if strings.HasPrefix(c.Key, keyPrefix) {
			return &c
		}
	}
	return nil
}

func vbKeys(r VerifyBoxReading) []string {
	var out []string
	for _, c := range r.GovRows() {
		out = append(out, c.Key)
	}
	return out
}

// ─── green, and only green ───────────────────────────────────────────────────

// The control the other two pins rest on: a verdict that is fresh AND clean
// raises nothing at all. Both halves are varied against it below, one at a
// time, so a row that fires unconditionally cannot pass this file.
func TestFreshAndCleanRaisesNothing(t *testing.T) {
	t.Parallel()
	a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-2*time.Hour), 0,
		"verify-grok-pin: ok", "verify-codex-pin: ok", "verify-bd-pin: not-measured")
	r := a.VerifyBoxFreshness(vbNow(), os.Stderr)

	if !r.Armed || !r.Ran || r.Stale || r.Err != nil {
		t.Fatalf("armed=%v ran=%v stale=%v err=%v — a 2h-old clean verdict is the green case", r.Armed, r.Ran, r.Stale, r.Err)
	}
	if keys := vbKeys(r); len(keys) != 0 {
		t.Errorf("a fresh clean verdict raised %v — checked recently and clean is the one green this surface has", keys)
	}
	// The quiet line still says it, which is the half the row by design does
	// not: a shop check prints conditions, and "checked clean 2h ago" is not
	// one. Without this line a clean box and an unarmed control print the
	// same nothing.
	line := r.Line()
	for _, want := range []string{"verify-box ·", "2h00m ago", "2 ok", "0 red", "1 not measured"} {
		if !strings.Contains(line, want) {
			t.Errorf("the status line is missing %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "STALE") {
		t.Errorf("a 2h verdict under a 26h max reads as stale:\n%s", line)
	}
}

// ─── red ─────────────────────────────────────────────────────────────────────

// A planted red verdict raises G10, names the check IN THE KEY, and is LANE.
//
// The key carries the check names because the key is the identity a machine
// reader sees — it is what the pulse fingerprints and what its prompt
// carries instead of the detail (pulse.go pulsePromptText). A bare
// `verify-box` key would deduplicate a codex pin that moved against a
// credential path that regenerated, and the coordinator woken for one would
// never hear about the other.
func TestAPlantedRedVerdictRaisesG10(t *testing.T) {
	t.Parallel()
	a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-3*time.Hour), 1,
		"verify-grok-pin: ok", "verify-codex-pin: finding", "verify-hook-freshness: error")
	r := a.VerifyBoxFreshness(vbNow(), os.Stderr)

	row := vbRow(r, "verify-box:")
	if row == nil {
		t.Fatalf("a red verdict raised nothing: %v", vbKeys(r))
	}
	if row.ID != "G10" {
		t.Errorf("row id = %q, want G10 — ADR 0029's table carries this one by name", row.ID)
	}
	if row.Class != GovLane {
		t.Errorf("class = %s, want %s: URGENT means the shop is stopped (ADR 0029) and a moved pin stops no dispatch", row.Class, GovLane)
	}
	// Both reds in the key, sorted, so the fingerprint moves when a
	// DIFFERENT check goes red and holds still while the same one persists.
	if row.Key != "verify-box:verify-codex-pin,verify-hook-freshness" {
		t.Errorf("key = %q — it must name the red checks, sorted", row.Key)
	}
	// An ERROR is red too. It is the runner's verdict on a check that could
	// not be run at all, which is the shape that would otherwise let someone
	// delete a check and keep a green board.
	for _, want := range []string{"verify-codex-pin finding", "verify-hook-freshness error"} {
		if !strings.Contains(row.Detail, want) {
			t.Errorf("the detail does not name %q:\n%s", want, row.Detail)
		}
	}
	// The check that PASSED is not in the row. A row that listed the whole
	// roster would satisfy every assertion above and tell a reader nothing.
	if strings.Contains(row.Key, "verify-grok-pin") {
		t.Errorf("the key names a check that came back ok: %q", row.Key)
	}

	// The control: the same three checks, all ok, at the same age, raise
	// nothing. Without it the assertions above pass on a row keyed off the
	// roster rather than off the verdict.
	b := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-3*time.Hour), 0,
		"verify-grok-pin: ok", "verify-codex-pin: ok", "verify-hook-freshness: ok")
	if keys := vbKeys(b.VerifyBoxFreshness(vbNow(), os.Stderr)); len(keys) != 0 {
		t.Errorf("the same roster all green still raised %v", keys)
	}
}

// ─── stale ───────────────────────────────────────────────────────────────────

// A planted OLD timestamp raises G10 whatever the verdict said. The clock is
// the only input that differs between the two arms here.
func TestAPlantedOldTimestampIsStale(t *testing.T) {
	t.Parallel()
	a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-40*time.Hour), 0, "verify-grok-pin: ok")
	r := a.VerifyBoxFreshness(vbNow(), os.Stderr)

	row := vbRow(r, "verify-box-stale")
	if row == nil {
		t.Fatalf("a 40h-old verdict under a 26h max raised nothing: %v", vbKeys(r))
	}
	if row.ID != "G10" || row.Class != GovLane {
		t.Errorf("row = %s/%s, want G10/%s", row.ID, row.Class, GovLane)
	}
	if !strings.Contains(row.Detail, "verify_box_max_age") {
		t.Errorf("the row does not name the threshold it tripped:\n%s", row.Detail)
	}
	// The log path is on the row because it is the answer to the question the
	// row raises: a run that died before its verdict left its last words
	// there and nowhere else (the plist's StandardOutPath/StandardErrorPath).
	if !strings.Contains(row.Detail, "verify-box.log") {
		t.Errorf("the stale row does not name the log a dying run writes to:\n%s", row.Detail)
	}
	if !strings.Contains(r.Line(), "STALE") {
		t.Errorf("the status line does not say STALE:\n%s", r.Line())
	}

	// The control: the same clean verdict inside the budget raises nothing.
	b := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-25*time.Hour), 0, "verify-grok-pin: ok")
	if keys := vbKeys(b.VerifyBoxFreshness(vbNow(), os.Stderr)); len(keys) != 0 {
		t.Errorf("a 25h verdict under a 26h max raised %v", keys)
	}
}

// The ORDERING, which is the ruling's own sentence: a stale verdict is never
// rendered as its own greens. A two-week-old "7 ok" is not seven ok checks.
//
// This is not implied by the two pins above — both would pass a reader that
// raised the stale row AND went on to report the old file's contents as
// today's — so it is pinned as an ordering, with the arm that separates it:
// the same old file, red inside, still yields exactly one row and it is the
// stale one, because the reds in it are as old as the greens.
func TestStaleWinsOverTheVerdictItCarries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		rc   int
		rows []string
	}{
		{"an old CLEAN verdict", 0, []string{"verify-grok-pin: ok", "verify-codex-pin: ok"}},
		{"an old RED verdict", 1, []string{"verify-grok-pin: ok", "verify-codex-pin: finding"}},
		{"an old UNMEASURED verdict", 2, []string{"verify-grok-pin: not-measured"}},
	} {
		a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-14*24*time.Hour), tc.rc, tc.rows...)
		keys := vbKeys(a.VerifyBoxFreshness(vbNow(), os.Stderr))
		if len(keys) != 1 || keys[0] != "verify-box-stale" {
			t.Errorf("%s two weeks old yields %v, want exactly [verify-box-stale] — an unrefreshed verdict says nothing about now, in either direction", tc.name, keys)
		}
	}
}

// Armed and NEVER RUN is the predecessor's exact failure — an arrangement
// installed and never fired (ADR 0036's context, and the LaunchAgent that
// was not there when the codex cask moved). It reads STALE, never silent.
func TestArmedAndNeverRunIsStale(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, "verify_box_max_age: 26h\n")
	r := a.VerifyBoxFreshness(vbNow(), os.Stderr)

	if !r.Armed || r.Ran {
		t.Fatalf("armed=%v ran=%v — a config asking for this control with no verdict on disk is armed and has not run", r.Armed, r.Ran)
	}
	if row := vbRow(r, "verify-box-stale"); row == nil {
		t.Errorf("an armed instance with no verdict raised nothing: %v", vbKeys(r))
	}
	if !strings.Contains(r.Line(), "NEVER RUN") {
		t.Errorf("the status line does not say so:\n%s", r.Line())
	}

	// The control, and the inertness rule every other reading here keeps:
	// an instance that has said nothing about this control and has no
	// verdict on disk reports NOTHING. Installing posse arms no schedule,
	// and a row on every fresh instance is a row people learn to skip.
	b := NewAppAt(t.TempDir())
	write(t, b.ConfigPath, "attn_question_age: 4h\n")
	if q := b.VerifyBoxFreshness(vbNow(), os.Stderr); q.Armed || len(q.GovRows()) > 0 {
		t.Errorf("an instance that never asked for this control reports armed=%v rows=%v", q.Armed, vbKeys(q))
	}
}

// A stamp from the FUTURE is not a reading. BlindFor renders every negative
// duration as "0s", so dating one would print the freshest possible verdict
// for as long as the stamp led the clock — the defect ADR 0036 §6 hit on
// backup archives (ranger-base-rgv61), arriving here by the same route.
func TestAStampAheadOfTheClockIsNotAReading(t *testing.T) {
	t.Parallel()
	a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(6*time.Hour), 0, "verify-grok-pin: ok")
	r := a.VerifyBoxFreshness(vbNow(), os.Stderr)

	if !r.Stale || r.Ahead != 6*time.Hour {
		t.Fatalf("stale=%v ahead=%s — a stamp 6h ahead of the clock is not a fresh verdict", r.Stale, r.Ahead)
	}
	if r.Age != 0 {
		t.Errorf("age = %s — a future stamp must not be dated at all", r.Age)
	}
	row := vbRow(r, "verify-box-stale")
	if row == nil || !strings.Contains(row.Detail, "AHEAD") {
		t.Errorf("the row does not report the stamp as ahead of the clock: %+v", row)
	}
	if strings.Contains(r.Line(), "0s ago") {
		t.Errorf("the line dates a future stamp as the freshest possible reading:\n%s", r.Line())
	}
}

// ─── nothing measured ────────────────────────────────────────────────────────

// A fresh run in which every check answered 2 is not a pass. It is the answer
// a box with none of the runtimes installed gives, and the script's own exit
// status refuses to launder it — "a schedule that treats 2 as green is a
// green light on an empty room". The reader must refuse it too.
func TestAFreshRunThatMeasuredNothingIsNotGreen(t *testing.T) {
	t.Parallel()
	a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), 2,
		"verify-grok-pin: not-measured", "verify-codex-pin: not-measured")
	if row := vbRow(a.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box-unmeasured"); row == nil {
		t.Fatalf("an all-unmeasured run raised nothing: %v", vbKeys(a.VerifyBoxFreshness(vbNow(), os.Stderr)))
	}

	// The control, and it is the line between this row and a nuisance: ONE
	// check that measured something makes the run a measured run. A box with
	// no codex answers 2 for the codex pin forever and that is not a finding.
	b := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), 0,
		"verify-grok-pin: not-measured", "verify-codex-pin: ok")
	if keys := vbKeys(b.VerifyBoxFreshness(vbNow(), os.Stderr)); len(keys) != 0 {
		t.Errorf("one real pass among the 2s still raised %v", keys)
	}
}

// A verdict with NO checks in it at all — `checks:` empty, or absent
// altogether — must read the same as an all-unmeasured run, not as clean
// (ranger-base-lvzm7 finding 1). Before the fix, `len(checks) > 0 &&` in
// verifyBoxVerdict and the `len(r.Checks) == 0` guard in Unmeasured made a
// checkless record cross-check clean (rc: 0), raise no G10 row, and print
// "0 ok, 0 red, 0 not measured" — the doctrine every other status in this
// file gets, applied to nothing, at the scale where it matters most: a green
// board over a room with nothing in it.
func TestAVerdictWithNoChecksIsNotGreen(t *testing.T) {
	t.Parallel()
	// `checks:` with no rows under it — vbRig writes the header line and
	// nothing after it, exactly what the bug's repro plants by hand.
	a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), 2)
	r := a.VerifyBoxFreshness(vbNow(), os.Stderr)
	if r.Err != nil {
		t.Fatalf("unexpected read error: %v", r.Err)
	}
	if !r.Unmeasured() {
		t.Errorf("Unmeasured() = false for a checkless verdict — a room with nothing in it must not read as measured")
	}
	if row := vbRow(r, "verify-box-unmeasured"); row == nil {
		t.Errorf("a checkless verdict raised nothing: %v", vbKeys(r))
	}
	if ok, red, unmeasured := r.Counts(); ok != 0 || red != 0 || unmeasured != 0 {
		t.Errorf("Counts() = %d ok, %d red, %d not measured — a checkless verdict has zero of everything, not a clean board", ok, red, unmeasured)
	}

	// The control: ONE real check, even a red one, is enough to make this a
	// measured run and not the checkless case — without it, the assertions
	// above would pass on a reader that called every run unmeasured.
	b := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), 1, "verify-grok-pin: finding")
	rb := b.VerifyBoxFreshness(vbNow(), os.Stderr)
	if rb.Unmeasured() {
		t.Errorf("control: a run with one real check reads Unmeasured() = true")
	}
	if row := vbRow(rb, "verify-box-unmeasured"); row != nil {
		t.Errorf("control: a run with one real (red) check raised the checkless row: %+v", row)
	}
}

// verifyBoxVerdict itself, at the boundary the reader's cross-check trusts:
// zero checks must compute 2, matching Unmeasured() above — a mismatch
// between the two would surface as "rc: 0 but 0 checks compute 2", a
// self-contradiction error instead of the checkless-verdict row.
func TestVerifyBoxVerdictOfNoChecksIsTwo(t *testing.T) {
	t.Parallel()
	if got := verifyBoxVerdict(nil); got != 2 {
		t.Errorf("verifyBoxVerdict(nil) = %d, want 2 — a checkless run is nothing measured, not clean", got)
	}
	if got := verifyBoxVerdict([]VerifyBoxCheck{}); got != 2 {
		t.Errorf("verifyBoxVerdict(empty slice) = %d, want 2", got)
	}
}

// ─── suppression, and its inverse ────────────────────────────────────────────

// A check red BY DESIGN and already tracked names its bead on the row. The
// operator ruled this shape over auto-filed beads (option (a), held back):
// the red stays visible and carries the id, so a reader sees a tracked
// condition instead of a fresh alarm.
//
// What it must NOT do is hide the check. A suppression that removes the row
// is how a check goes permanently dark, and the day the bead closes without
// the entry being retired, nobody is told again — which is why the second
// half of this pin exists.
func TestAnAcceptedRedNamesItsBeadAndStaysOnTheRow(t *testing.T) {
	t.Parallel()
	a := vbRig(t, "verify_box_max_age: 26h\nverify_box_accepted:\n  verify-codex-pin: ranger-base-femsg\n",
		vbNow().Add(-time.Hour), 1, "verify-grok-pin: ok", "verify-codex-pin: finding")
	r := a.VerifyBoxFreshness(vbNow(), os.Stderr)

	row := vbRow(r, "verify-box:")
	if row == nil {
		t.Fatalf("an accepted red raised nothing — accepting a risk names it, it does not hide it: %v", vbKeys(r))
	}
	if !strings.Contains(row.Detail, "ranger-base-femsg") {
		t.Errorf("the row does not name the bead that tracks the red:\n%s", row.Detail)
	}
	if !strings.Contains(row.Key, "verify-codex-pin") {
		t.Errorf("the accepted check left the key %q — a suppression that removes the check is how it goes dark", row.Key)
	}
	if !strings.Contains(r.Line(), "tracked by ranger-base-femsg") {
		t.Errorf("the status line does not carry the tracking bead:\n%s", r.Line())
	}

	// The control: with no acceptance the SAME verdict raises the same row
	// with no bead on it, so the id above came from the config and not from
	// a string this row always prints.
	b := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), 1,
		"verify-grok-pin: ok", "verify-codex-pin: finding")
	brow := vbRow(b.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box:")
	if brow == nil || strings.Contains(brow.Detail, "tracked by") {
		t.Errorf("an unaccepted red claims to be tracked: %+v", brow)
	}
}

// The inverse, and it is the failure a suppression list has that no check
// has: an entry outliving its cause. The bead lands, the check goes green,
// the entry stays — and the day that check goes red again the row says
// "tracked by <a closed bead>" and a reader moves on. So an acceptance whose
// check is not red is itself a row.
func TestAnAcceptanceThatSuppressesNothingIsItsOwnRow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, checks string }{
		{"the check now passes", "verify-codex-pin: ok"},
		{"the entry names no check the run knows", "verify-grok-pin: ok"},
	} {
		a := vbRig(t, "verify_box_max_age: 26h\nverify_box_accepted:\n  verify-codex-pin: ranger-base-femsg\n",
			vbNow().Add(-time.Hour), 0, tc.checks)
		row := vbRow(a.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box-accept-stale:")
		if row == nil {
			t.Errorf("%s: the stale acceptance raised nothing", tc.name)
			continue
		}
		if row.Key != "verify-box-accept-stale:verify-codex-pin" || !strings.Contains(row.Detail, "ranger-base-femsg") {
			t.Errorf("%s: row = %q / %q", tc.name, row.Key, row.Detail)
		}
	}

	// Two controls, because this row has two ways to become a nuisance.
	// FIRST: an entry doing its job raises no stale row.
	live := vbRig(t, "verify_box_max_age: 26h\nverify_box_accepted:\n  verify-codex-pin: ranger-base-femsg\n",
		vbNow().Add(-time.Hour), 1, "verify-codex-pin: finding")
	if row := vbRow(live.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box-accept-stale:"); row != nil {
		t.Errorf("an acceptance suppressing a live red reads as stale: %q", row.Detail)
	}
	// SECOND: a token this reader does not KNOW is red, so an acceptance
	// suppressing one is doing its job. This arm is the exact inverse of
	// TestATokenThisReaderDoesNotKnowIsRed, and keying the stale check on
	// the three known status strings instead of on Red() puts them out of
	// step — the row would then tell the operator to retire a live entry.
	unknown := vbRig(t, "verify_box_max_age: 26h\nverify_box_accepted:\n  verify-codex-pin: ranger-base-femsg\n",
		vbNow().Add(-time.Hour), 1, "verify-codex-pin: skipped")
	if row := vbRow(unknown.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box-accept-stale:"); row != nil {
		t.Errorf("an acceptance suppressing an unknown-token red reads as stale: %q", row.Detail)
	}
	// THIRD: a check that answered "nothing measured" is neither the
	// finding being fixed nor the entry naming nothing. Calling it stale
	// would file a row against every box with no codex installed.
	blind := vbRig(t, "verify_box_max_age: 26h\nverify_box_accepted:\n  verify-codex-pin: ranger-base-femsg\n",
		vbNow().Add(-time.Hour), 0, "verify-codex-pin: not-measured", "verify-grok-pin: ok")
	if row := vbRow(blind.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box-accept-stale:"); row != nil {
		t.Errorf("an unmeasured check makes its acceptance read as stale: %q", row.Detail)
	}
}

// ─── a store that cannot be read ─────────────────────────────────────────────

// A store that cannot be READ is not a store that says no. A malformed
// verdict makes the set PARTIAL — `posse status` says so and exits non-zero
// — and raises no green, because rendering an unparseable file as a clean
// box is the silence this whole row exists to end.
func TestAMalformedVerdictIsPartialAndNeverGreen(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body string }{
		{"no stamp at all", "rc: 0\nchecks:\n  verify-grok-pin: ok\n"},
		{"a stamp that is not a date", "at: last tuesday\nrc: 0\n"},
	} {
		a := NewAppAt(t.TempDir())
		write(t, a.ConfigPath, "verify_box_max_age: 26h\n")
		write(t, VerifyBoxStatePath(a), tc.body)
		r := a.VerifyBoxFreshness(vbNow(), os.Stderr)
		if r.Err == nil {
			t.Errorf("%s: read without error, so an unparseable verdict would render as a box nobody has to look at", tc.name)
		}
		if len(r.GovRows()) != 0 {
			t.Errorf("%s: an unreadable store also raised a condition — it must be reported as unknown, not diagnosed", tc.name)
		}
		if !strings.Contains(r.Line(), "could not be read") {
			t.Errorf("%s: the status line hides the read failure:\n%s", tc.name, r.Line())
		}
	}
}

// A token this reader does not know is RED, not silence. The two benign
// answers are enumerated and everything else is a record this reader cannot
// interpret — because reading an unknown token as "not red" is how the
// producer could grow a fifth status, or a typo, and take a check off the
// surface while leaving it on the roster.
func TestATokenThisReaderDoesNotKnowIsRed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, status string }{
		{"a status nobody defined", "skipped"},
		{"a status with no value at all", ""},
	} {
		a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), 1,
			"verify-grok-pin: ok", "verify-codex-pin: "+tc.status)
		row := vbRow(a.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box:")
		if row == nil || !strings.Contains(row.Key, "verify-codex-pin") {
			t.Errorf("%s: %q left the check off the surface: %+v", tc.name, tc.status, row)
		}
	}
	// The control, and it is the line this rule must not cross: the two
	// benign tokens stay benign. A box with no codex answers "not measured"
	// for the codex pin forever and that is not a finding.
	b := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), 0,
		"verify-grok-pin: ok", "verify-codex-pin: not-measured")
	if keys := vbKeys(b.VerifyBoxFreshness(vbNow(), os.Stderr)); len(keys) != 0 {
		t.Errorf("ok and not-measured raised %v", keys)
	}
}

// `rc:` is checked against the checks it was computed from, so it cannot
// become a second store that drifts from the first in silence. A record
// whose two halves disagree has been edited, truncated or half-written, and
// it is not a verdict — reported as unknown, never rendered as a box.
func TestARecordThatContradictsItselfIsNotAVerdict(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		rc     int
		checks []string
	}{
		{"rc says clean over a red check", 0, []string{"verify-codex-pin: finding"}},
		{"rc says red over a clean map", 1, []string{"verify-codex-pin: ok"}},
		{"rc says nothing-measured over a real pass", 2, []string{"verify-codex-pin: ok"}},
		{"no rc line at all", -1, []string{"verify-codex-pin: ok"}},
	} {
		a := NewAppAt(t.TempDir())
		write(t, a.ConfigPath, "verify_box_max_age: 26h\n")
		body := "at: " + vbNow().Add(-time.Hour).Format(time.RFC3339) + "\n"
		if tc.rc >= 0 {
			body += fmt.Sprintf("rc: %d\n", tc.rc)
		}
		body += "checks:\n"
		for _, c := range tc.checks {
			body += "  " + c + "\n"
		}
		write(t, VerifyBoxStatePath(a), body)
		r := a.VerifyBoxFreshness(vbNow(), os.Stderr)
		if r.Err == nil {
			t.Errorf("%s: read clean, so a self-contradicting record renders as a verdict", tc.name)
		}
		if len(r.GovRows()) != 0 {
			t.Errorf("%s: an unreadable record was also diagnosed: %v", tc.name, vbKeys(r))
		}
	}

	// The controls, one per verdict the runner can reach, so this check
	// cannot pass by rejecting everything.
	for _, tc := range []struct {
		rc     int
		checks []string
	}{
		{0, []string{"verify-codex-pin: ok", "verify-grok-pin: not-measured"}},
		{1, []string{"verify-codex-pin: finding", "verify-grok-pin: ok"}},
		{2, []string{"verify-codex-pin: not-measured"}},
	} {
		a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-time.Hour), tc.rc, tc.checks...)
		if r := a.VerifyBoxFreshness(vbNow(), os.Stderr); r.Err != nil {
			t.Errorf("rc %d over %v is the runner's own arithmetic and reads as a contradiction: %v", tc.rc, tc.checks, r.Err)
		}
	}
}

// ─── the row reaches the one computation every rendering shares ──────────────

// The rows above are computed by verifybox.go; this is the pin that they
// arrive in ShopCheck, which is what makes them appear in `posse status`,
// the cockpit's GOVERNANCE block and the pulse's prompt at all. A reading
// nobody joins into the set is a reading on nobody's screen.
func TestG10ReachesTheGovernanceSet(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "verify_box_max_age: 12h\n")
	write(t, VerifyBoxStatePath(b.App), fmt.Sprintf("at: %s\nrc: 1\nchecks:\n  verify-codex-pin: finding\n",
		govNow.Add(-3*time.Hour).UTC().Format(time.RFC3339)))

	set := shopSet(t, govIn(t, b))
	row := find(set, "G10")
	if row == nil {
		t.Fatalf("a fresh red verdict raised no G10 in the shared set: %v", set.Keys())
	}
	if row.Row() != "G10" {
		t.Errorf("the row renders as %q — this one has a number and ADR 0029's table carries it", row.Row())
	}
	if len(set) == 0 {
		t.Fatal("the set is empty, so status would exit 0 over a red box")
	}

	// The control: the same instance with a fresh CLEAN verdict has no G10.
	write(t, VerifyBoxStatePath(b.App), fmt.Sprintf("at: %s\nrc: 0\nchecks:\n  verify-codex-pin: ok\n",
		govNow.Add(-3*time.Hour).UTC().Format(time.RFC3339)))
	if keys := shopKeys(t, govIn(t, b)); containsString(keys, "verify-box:verify-codex-pin") {
		t.Errorf("a clean verdict still raises the red row: %v", keys)
	}
}

// The threshold is the operator's, and a typo in it is NAMED rather than
// silently taken — same grammar and same handling as every other interval
// key, because a threshold nobody can see is worse than a wrong one.
func TestTheFreshnessBudgetIsConfigurableAndATypoIsNamed(t *testing.T) {
	t.Parallel()
	a := vbRig(t, "verify_box_max_age: 1h\n", vbNow().Add(-2*time.Hour), 0, "verify-grok-pin: ok")
	if row := vbRow(a.VerifyBoxFreshness(vbNow(), os.Stderr), "verify-box-stale"); row == nil {
		t.Error("a 2h verdict under an operator's 1h max is not stale — the key is not being read")
	}
	// The default, with no key at all but a file on disk: 26h, a daily
	// schedule plus two hours of slack.
	b := NewAppAt(t.TempDir())
	write(t, b.ConfigPath, "")
	write(t, VerifyBoxStatePath(b), fmt.Sprintf("at: %s\nrc: 0\nchecks:\n  verify-grok-pin: ok\n",
		vbNow().Add(-25*time.Hour).UTC().Format(time.RFC3339)))
	if r := b.VerifyBoxFreshness(vbNow(), os.Stderr); r.Stale || r.MaxAge != DefaultVerifyBoxMaxAge {
		t.Errorf("stale=%v maxAge=%s — the default budget is %s", r.Stale, r.MaxAge, DefaultVerifyBoxMaxAge)
	}
	// And 27h is past it, so the default is a threshold and not a shrug.
	write(t, VerifyBoxStatePath(b), fmt.Sprintf("at: %s\nrc: 0\nchecks:\n  verify-grok-pin: ok\n",
		vbNow().Add(-27*time.Hour).UTC().Format(time.RFC3339)))
	if r := b.VerifyBoxFreshness(vbNow(), os.Stderr); !r.Stale {
		t.Error("a 27h verdict under the 26h default is not stale")
	}

	var say strings.Builder
	c := vbRig(t, "verify_box_max_age: one day\n", vbNow().Add(-2*time.Hour), 0, "verify-grok-pin: ok")
	if got := c.VerifyBoxMaxAge(&say); got != DefaultVerifyBoxMaxAge {
		t.Errorf("a typo took the threshold to %s instead of the default", got)
	}
	if !strings.Contains(say.String(), "verify_box_max_age") {
		t.Errorf("a typo in the key is swallowed:\n%s", say.String())
	}
}

// The state file lives where posse's other state lives, and the log beside
// it. Pinned because the LaunchAgent plist hard-codes both paths —
// it cannot ask the binary — so a rename here silently un-wires the schedule
// and the surface goes quiet in exactly the way this bead exists to prevent.
func TestTheTwoPathsAreUnderTheInstancesStateDir(t *testing.T) {
	t.Parallel()
	a := NewAppAt(filepath.Join(t.TempDir(), "posse"))
	if got, want := VerifyBoxStatePath(a), filepath.Join(a.StateDir, "verify-box.yaml"); got != want {
		t.Errorf("state path = %q, want %q", got, want)
	}
	if got, want := VerifyBoxLogPath(a), filepath.Join(a.StateDir, "verify-box.log"); got != want {
		t.Errorf("log path = %q, want %q", got, want)
	}
}

// ─── the two readers of one file ─────────────────────────────────────────────

// The join, and it is the pin every fixture above depends on: the format
// scripts/verify-box.sh WRITES is the format this reader PARSES.
//
// Everything else in this file plants the bytes by hand, which measures the
// reader's rules and nothing about the pair. Two readers of one file that
// were never run against each other is how a producer's `not measured`
// becomes a consumer's unknown token in silence — the reader would report a
// check as neither ok nor red, and a red board would print as a green one.
//
// So this runs the SHIPPED script (through the same --self-test-run seam its
// own arms use, which re-execs the real run_roster rather than a replica)
// against a scratch RHQ_HOME, and reads the file back with VerifyBoxFreshness.
// The roster is planted so all four statuses appear in one run: the three a
// check defines, plus the ERROR the runner assigns a check it cannot run.
func TestTheScriptWritesWhatThisReaderParses(t *testing.T) {
	t.Parallel()
	script, err := filepath.Abs("../../scripts/verify-box.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("no %s to run: %v", script, err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(dir, "scripts", "verify-box.sh")
	if err := os.WriteFile(copied, body, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rc := range []int{0, 1, 2} {
		arm := filepath.Join(dir, "scripts", fmt.Sprintf("arm%d.sh", rc))
		if err := os.WriteFile(arm, []byte(fmt.Sprintf("#!/bin/sh\nexit %d\n", rc)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	home := filepath.Join(dir, "home")
	roster := strings.Join([]string{
		"green\tscripts/arm0.sh",
		"red\tscripts/arm1.sh",
		"blind\tscripts/arm2.sh",
		"broken\tscripts/gone.sh",
	}, "\n") + "\n"

	cmd := exec.Command(copied, "--self-test-run")
	cmd.Env = append(os.Environ(), "ROSTER_OVERRIDE="+roster, "RHQ_HOME="+home)
	out, runErr := cmd.CombinedOutput()
	// A roster with a finding in it exits 1 by design; what is under test is
	// the file it left behind.
	if runErr != nil && cmd.ProcessState.ExitCode() != 1 {
		t.Fatalf("the script exited %d: %v\n%s", cmd.ProcessState.ExitCode(), runErr, out)
	}

	a := NewAppAt(home)
	r := a.VerifyBoxFreshness(time.Now(), os.Stderr)
	if r.Err != nil {
		t.Fatalf("the reader cannot parse what the script wrote: %v\nrun output:\n%s", r.Err, out)
	}
	if !r.Ran || r.Stale {
		t.Fatalf("ran=%v stale=%v — a verdict written seconds ago is fresh\n%s", r.Ran, r.Stale, out)
	}
	got := map[string]string{}
	for _, c := range r.Checks {
		got[c.Name] = c.Status
	}
	// All four tokens, each read back as the constant this package branches
	// on. An unknown token would leave a check neither ok nor red — counted
	// nowhere, and invisible on the row.
	for name, want := range map[string]string{
		"green":  VerifyBoxOK,
		"red":    VerifyBoxFinding,
		"blind":  VerifyBoxUnmeasured,
		"broken": VerifyBoxError,
	} {
		if got[name] != want {
			t.Errorf("the script wrote %q for %s and this reader wants %q\nfile:\n%s", got[name], name, want, mustRead(t, VerifyBoxStatePath(a)))
		}
	}
	ok, red, unmeasured := r.Counts()
	if ok != 1 || red != 2 || unmeasured != 1 {
		t.Errorf("counts = %d ok / %d red / %d not measured, want 1/2/1 — a token this reader does not know is counted nowhere", ok, red, unmeasured)
	}
	if row := vbRow(r, "verify-box:"); row == nil || !strings.Contains(row.Key, "red") {
		t.Errorf("a real run with a finding in it raises no row: %v", vbKeys(r))
	}
	// The start line is the OTHER half of the ruling (b): a run that dies
	// before its verdict must still leave a line, and this is the line. It
	// is printed before the first check runs and it carries the same stamp
	// the record does, so a log with a start line and no verdict is exactly
	// as legible as one with both.
	if !strings.Contains(string(out), "run started "+r.At.Format(time.RFC3339)) {
		t.Errorf("the start line does not carry the stamp the record was written with (%s):\n%s", r.At.Format(time.RFC3339), out)
	}
}
