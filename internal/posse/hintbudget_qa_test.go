package posse

// ranger-base-fsil: two guards on the settle-hint budget, both source-level
// on purpose. What they want is "this file never waits on a bare number
// again", and a test cannot observe that by waiting — it would have to
// reproduce the flake to fail, which is the thing being fixed.
//
// The flake they close out: TestWatchSettleHintWakesTheNextPassEarly went
// red about one full `go test ./internal/rhq` run in three, always at
// 5.0x seconds. Measured on this box (1600 instrumented waits under load
// 62-89), the wait distribution is not a tail — it is bimodal, with nothing
// at all between 324ms and 5097ms, and everything above 5s is the subscribe
// handshake in the one test that goes through the production adapter. That
// gap is herdrHintRetry: when the loop's first subscribe has to be redialled
// the second attempt arrives one whole retry delay later, and the old 5s
// budget was that delay to the millisecond. A budget equal to a retry the
// code under test may legitimately spend is a coin flip, however patient it
// looks.
//
// So the first guard is a ratio, not a number: whatever the retry becomes,
// the budget has to clear it several times over.

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestQAHintBudgetClearsTheAdapterRetry(t *testing.T) {
	t.Parallel()
	if hintWait < 5*herdrHintRetry {
		t.Errorf("hintWait is %s and the adapter redials after %s: a test budget "+
			"under 5x the retry it may have to wait out is the ranger-base-fsil "+
			"flake, which failed at exactly one retry delay", hintWait, herdrHintRetry)
	}
	// And the retry cannot be raised into the budget from the other side.
	if herdrHintRetry > 10*time.Second {
		t.Errorf("herdrHintRetry is %s: raising it moves the same race back under "+
			"hintWait (%s) — raise both or neither", herdrHintRetry, hintWait)
	}
}

// Every wait in herdrevents_test.go spends the one named budget. A literal
// here is how the flake got in: four separate 5s deadlines, none of them
// reading as a decision.
//
// One exemption, added with ranger-base-7hjy4 and deliberately narrow.
// TestHerdrHintsRedialFloorBoundsAStorm does not wait for anything: it runs
// unbroken churn for a fixed stretch and COUNTS the dials the adapter gets
// out in it, so its deadline is the instrument, not patience, and spending
// hintWait on it would make the test a minute long and measure nothing new.
// The exemption is by name and the name is checked below — `stormWindow`
// must be declared under a second, so nobody can grow it back into the
// patience budget this guard exists to forbid.
func TestQAHintWaitsUseTheNamedBudget(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("herdrevents_test.go")
	if err != nil {
		t.Fatal(err)
	}
	// The three shapes a wait takes in that file. `within` is recvHint's own
	// parameter, which its callers fill with hintWait.
	budgets := []*regexp.Regexp{
		regexp.MustCompile(`time\.After\(([^)]*)\)`),
		regexp.MustCompile(`time\.Now\(\)\.Add\(([^)]*)\)`),
		// `recvHint(t, ` and not `recvHint(t *testing.T, `: the call sites,
		// not the declaration the third shape is named after.
		regexp.MustCompile(`recvHint\(t,\s*[^,]+,\s*([^)]*)\)`),
	}
	found := 0
	for _, re := range budgets {
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			found++
			switch arg := strings.TrimSpace(m[1]); arg {
			case "hintWait", "within":
			case "stormWindow": // a measurement window, not a budget; bounded below
			default:
				t.Errorf("a wait spends %q instead of hintWait: %s", arg, strings.TrimSpace(m[0]))
			}
		}
	}
	// Without this the guard passes a file that has no waits left in it at
	// all — renamed, rewritten, or deleted — which is the one way it could
	// go green while measuring nothing.
	if found < 12 {
		t.Errorf("only %d waits found in herdrevents_test.go; this guard has lost "+
			"its subject and needs to follow it", found)
	}

	// The exemption's own fence. `stormWindow` is allowed above only as a
	// stretch of churn to count dials in; a patience budget wearing the name
	// would be the exact defect this file was written for, so it has to
	// declare itself and it has to be short.
	decl := regexp.MustCompile(`stormWindow\s*=\s*(\d+)\s*\*\s*time\.(Millisecond|Second|Minute)`).FindStringSubmatch(string(src))
	if decl == nil {
		t.Fatal("stormWindow is exempted from the named budget but herdrevents_test.go does not declare it")
	}
	n, _ := strconv.Atoi(decl[1])
	unit := map[string]time.Duration{
		"Millisecond": time.Millisecond, "Second": time.Second, "Minute": time.Minute,
	}[decl[2]]
	// `>=` and not `>`: the doc comment above and this message both say
	// "under a second" / "a second or longer", and an exemption fence that
	// admits the first value it forbids is not a fence (ranger-base-0b0qg).
	if got := time.Duration(n) * unit; got >= time.Second {
		t.Errorf("stormWindow is %s: the exemption is for a window a test COUNTS in, "+
			"and anything a second or longer is patience wearing its name — spend hintWait", got)
	}
}

// The redial floor's ceiling, the herdrHintRetry check above in the shape it
// already uses: a constant in herdrevents.go whose bound ADR 0016 states as a
// number gets that bound pinned here, because nothing else reads the SHIPPED
// value — both of ranger-base-7hjy4's new pins take the floor as a parameter,
// so the constant itself shipped unmeasured (ranger-base-0b0qg, from
// ranger-base-8ouj8). Measured before this pin existed: the constant at 3s and
// at 10s ran `go test ./internal/posse -run "Herdr|Hint|Budget|Watch"` green
// both times.
//
// The bound is the ADR's and not a fresh number: §1 prices the floor's cost as
// "a pane that appears and settles inside the wait is missed by the stream and
// swept by the timer", and bounds it above by that sweep — the cockpit's
// two-second completeness tick (cmd/posse/cockpit.go, `time.NewTicker(2 *
// time.Second)`), "so the floor never outlives the timer that covers it". A
// literal here and not a reference: the tick is cmd/posse's, this is
// internal/posse, and the sibling check spells its bound the same way.
//
// `>=` because a floor equal to the sweep does not sit under it — the two land
// in the same instant and which goes first is scheduling, which is the
// stormWindow fence's defect one guard up.
//
// This is the CEILING only. §1's lower edge is the dial's own cost ("anything
// under ~33 ms would be decorative"), stated as an approximation rather than a
// bound, and one second's place inside the band is ASSUMED by the ADR in as
// many words — so a bead that moves the floor within the band moves it without
// touching this test, and only a floor that outlives its sweep reds.
func TestQAHerdrRedialFloorStaysUnderItsSweep(t *testing.T) {
	t.Parallel()
	const cockpitCompletenessTick = 2 * time.Second
	if herdrRedialFloor >= cockpitCompletenessTick {
		t.Errorf("herdrRedialFloor is %s and the cockpit's completeness tick is %s: "+
			"ADR 0016 §1 bounds the floor above by the sweep that covers the pane it "+
			"delays, and a floor at or past that tick outlives it — move the tick with "+
			"it, or bring the measurement the ADR asks for", herdrRedialFloor, cockpitCompletenessTick)
	}
}
