package posse

// QA pins for the account stage's one answer (ranger-base-0lg6).
//
// The defect: `Runtime.Counted()` read a hand-written `cost_adapter:` string
// on the builtin table while `CostProviderFor` read the adapter registry.
// grok gained an adapter in ranger-base-k7nb (2418bde) and no one wrote the
// string, so for two days `posse dispatch` printed "no cost adapter reads
// grok, so none of this spend is in `posse cost`" about spend that
// `posse cost` was already totalling, and `posse runtime check grok` pointed
// at a cap for a pool that had a meter.
//
// The fix deleted the second declaration: the registry is the whole answer,
// and the adapter itself says what it reads (Reads) and whether that reading
// ends in dollars (Prices). These pins hold the three readers to one answer
// and hold Prices to what the decoders actually produce — a declaration
// nothing measures is exactly the shape that drifted.

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// The three readers, one answer, per shipped built-in. The expected column
// is spelled as a literal rather than derived, so re-declaring a runtime's
// account state has to change this table too.
func TestQAAccountStageReadersGiveOneAnswer(t *testing.T) {
	a := checkApp(t)
	for _, c := range []struct {
		name string
		// state is the account stage's answer: counted (dollars reach
		// `posse cost`), unpriced (an adapter reads it and prices none of
		// it), or uncounted (nothing reads it).
		state string
	}{
		{"claude", "counted"},
		{"codex", "unpriced"},
		{"grok", "counted"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rt, err := a.LoadRuntime(c.name)
			if err != nil {
				t.Fatal(err)
			}
			_, registered := CostProviderFor(c.name)

			// Reader 1: the registry, which is what `posse cost` and the
			// cockpit key on (CountUncounted, sessionCost).
			if wantRead := c.state != "uncounted"; registered != wantRead {
				t.Errorf("registry: adapter registered = %v, want %v", registered, wantRead)
			}
			// Reader 2: the runtime's own predicates, which dispatch's
			// account stage keys on.
			if want := c.state == "counted"; rt.CostPriced() != want {
				t.Errorf("CostPriced = %v, want %v", rt.CostPriced(), want)
			}
			// Reader 3: the `posse runtime check` grid.
			var b bytes.Buffer
			a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
			out := b.String()
			row := accountRowOf(t, out)
			marker := map[string]string{
				"counted": "counted — ", "unpriced": "UNPRICED — ", "uncounted": "UNCOUNTED — ",
			}[c.state]
			if !strings.Contains(row, marker) {
				t.Errorf("account row does not say %q:\n%s", marker, row)
			}
			// The sentence the bug printed. It is false about any runtime an
			// adapter reads, whether or not that adapter prices anything.
			if registered && strings.Contains(out, "no cost adapter reads") {
				t.Errorf("%s HAS an adapter; the grid still says nothing reads it:\n%s", c.name, row)
			}
		})
	}
}

// accountRowOf returns the account stage's block from a RuntimeCheck grid.
func accountRowOf(t *testing.T, out string) string {
	t.Helper()
	i := strings.Index(out, "account")
	if i < 0 {
		t.Fatalf("no account row in the grid:\n%s", out)
	}
	rest := out[i:]
	if j := strings.Index(rest, "\ntier"); j > 0 {
		rest = rest[:j]
	}
	return rest
}

// Prices() is a claim about the decoder, so it is pinned against the
// decoder. This is the arm that would have caught the original drift from
// the other side: grok's adapter puts real dollars on a segment, and any
// reading of the account stage that says otherwise is wrong about a number
// that is already in `posse cost`.
func TestQAPricesMatchesWhatTheDecoderProduces(t *testing.T) {
	t.Run("grok", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		body := grokUser("Work beads issue a-1 (t)") + grokTurn("p1", 250_000_000, 10, 0, 1)
		p := writeGrokSession(t, home, "/src/posse", body)
		segs, err := grokCost{}.Decode(p, time.Time{})
		if err != nil || len(segs) != 1 {
			t.Fatalf("decode: %v, %d segments", err, len(segs))
		}
		if segs[0].CostUSD <= 0 || !segs[0].Priced() {
			t.Fatalf("a grok segment carries the provider's own dollars: %+v", segs[0])
		}
		if !(grokCost{}).Prices() {
			t.Error("grok decodes dollars and must declare Prices() true — the declaration is what the account stage reads")
		}
	})

	t.Run("codex", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		p := writeCodexRollout(t, home,
			codexMeta("/src/posse"),
			codexTurnCtx("2026-08-28T09:00:00Z", "gpt-5.6-sol"),
			codexUser("2026-08-28T09:00:01Z", "Work beads issue a-1 (t)"),
			codexCount("2026-08-28T09:00:02Z", codexUsage(1000, 0, 0, 200, 0), nil),
		)
		segs, err := codexCost{}.Decode(p, time.Time{})
		if err != nil || len(segs) != 1 {
			t.Fatalf("decode: %v, %d segments", err, len(segs))
		}
		segs[0].Total()
		if segs[0].CostUSD != 0 || segs[0].Priced() {
			t.Fatalf("a codex segment carries tokens and no dollars: %+v", segs[0])
		}
		// ...and tokens it DOES carry, which is the whole difference
		// between unpriced and uncounted.
		if len(segs[0].Msgs) == 0 {
			t.Fatal("a codex segment carries token counts; without them this is uncounted, not unpriced")
		}
		if (codexCost{}).Prices() {
			t.Error("codex prices nothing and must declare Prices() false — a plan seat reports no cost and no list rate applies")
		}
	})
}

// The behavioural half of the fix, from the brake's side. A runtime that is
// READ but never PRICED keeps ADR 0013 §5's brake, because what the brake
// stands in for — no dollar meter on this pool — is still the case; a
// runtime whose dollars ARE counted loses it. Before ranger-base-0lg6 grok
// was in the first group with the second group's meter.
func TestQAUnpricedKeepsTheBrakeAndPricedLosesIt(t *testing.T) {
	for _, c := range []struct {
		runtime  string
		degraded bool
	}{
		{"codex", true},
		{"grok", false},
	} {
		t.Run(c.runtime, func(t *testing.T) {
			f := uncountedPass(t, "uncounted_cap_"+c.runtime+": 1\n",
				`[{"id":"a-1","title":"t","labels":["go"]}]`, "ranger")
			// uncountedPass writes a codex PID; retarget it.
			pid := strings.Replace(codexPID("ranger"), "runtime: codex", "runtime: "+c.runtime, 1)
			if err := os.WriteFile(f.b.App.AgentsDir+"/ranger.md", []byte(pid), 0o644); err != nil {
				t.Fatal(err)
			}

			n, err := f.d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(f.d)
			if n != 1 {
				t.Fatalf("the bead must launch either way, got n=%d:\n%s", n, out)
			}
			// Positive witness: the launch really went to the runtime under
			// test. Without it a silent PID-rewrite failure leaves the grok
			// arm measuring codex and passing for the wrong reason (nothing
			// degraded either way).
			//
			// Two readings, because they fail independently. The PID is what
			// dispatch resolved; the launch line is what the pass SAID, and
			// it only says it since ranger-base-pjoy — before that this
			// witness had to be the PID alone, and the comment here said so.
			ag, err := f.b.App.LoadAgent("ranger")
			if err != nil || ag.Runtime != c.runtime {
				t.Fatalf("the persona dispatch ran is on %q, want %q (%v)", ag.Runtime, c.runtime, err)
			}
			if !strings.Contains(out, "creating session ") || !strings.Contains(createLineOf(t, out), c.runtime+"/") {
				t.Fatalf("the create line must name %s:\n%s", c.runtime, out)
			}
			// Not "account-degraded <runtime>" but any degrade at all: a
			// launch that silently landed on some OTHER degraded runtime
			// must not read as this one's silence.
			got := strings.Contains(out, "account-degraded")
			if got && !strings.Contains(out, "account-degraded "+c.runtime) {
				t.Errorf("some other runtime was degraded, not %s:\n%s", c.runtime, out)
			}
			if got != c.degraded {
				t.Errorf("account-degraded line = %v, want %v:\n%s", got, c.degraded, out)
			}
			_, ledgered := os.Stat(f.b.App.UncountedLogPath())
			if (ledgered == nil) != c.degraded {
				t.Errorf("uncounted ledger written = %v, want %v", ledgered == nil, c.degraded)
			}
			// The cap key is only a brake where the degrade is. A set cap on
			// a counted runtime must not stop a launch, and since
			// ranger-base-2eeb it must not go quiet either: the pass's own
			// output stays clean of the key (this is not an outcome of the
			// pass) and stderr carries ADR 0010 §3's dead-key line instead.
			// Both halves, because either alone is a state the amendment
			// refuses — a silent dead key, or a brake that came back.
			if !c.degraded && strings.Contains(out, "uncounted_cap_"+c.runtime) {
				t.Errorf("a counted runtime's cap key must not brake it:\n%s", out)
			}
			deadKey := strings.Contains(f.errb.String(), "config uncounted_cap_"+c.runtime+": \"1\" does not apply")
			if deadKey == c.degraded {
				t.Errorf("dead-key line = %v, want %v (ADR 0010 §3):\n%s", deadKey, !c.degraded, f.errb.String())
			}
			// The sentence the bug printed, from the pass's side.
			if strings.Contains(out, "no cost adapter reads "+c.runtime) {
				t.Errorf("an adapter reads %s; the pass must not say otherwise:\n%s", c.runtime, out)
			}
		})
	}
}

// An ADR 0010 overflow move onto a COUNTED pool is not account-degraded
// spend: `posse cost` has the dollars, so the bead-count stand-in must not
// be written. The mirror of TestUncountedCountsAnOverflowMove.
func TestQAOverflowOntoACountedPoolIsNotDegradedSpend(t *testing.T) {
	f := overflowPass(t, "plan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n",
		overflowPID, `["go","tier:standard"]`)

	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(f.d)
	if n != 1 {
		t.Fatalf("the eligible bead must still move, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "account-degraded") {
		t.Errorf("grok is counted; a move onto it is not an account degrade:\n%s", out)
	}
	if _, err := os.Stat(f.b.App.UncountedLogPath()); !os.IsNotExist(err) {
		t.Errorf("nothing may be ledgered for a counted pool (%v)", err)
	}
	// The overflow ledger, which answers a different question, still has it.
	if got := len(f.ledger(t)); got != 1 {
		t.Errorf("overflow ledger: want 1 line, got %d", got)
	}
}
