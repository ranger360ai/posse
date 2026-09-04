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
	if got := time.Duration(n) * unit; got > time.Second {
		t.Errorf("stormWindow is %s: the exemption is for a window a test COUNTS in, "+
			"and anything a second or longer is patience wearing its name — spend hintWait", got)
	}
}
