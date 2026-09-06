//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

// QA pin for ranger-base-u2etb finding 2, escaped from ranger-base-60p4b.
//
// THE DEFECT. `posse runtime check codex` printed "all three AGENTS rules
// broken, the PID's obeyed". The trace it cites
// (docs/adr/0013-rules-precedence-probe.md, "Verdicts") says codex "emitted
// *neither* token — it dropped PID rule 2 as well as the AGENTS one", so the
// SAME silence was counted as a broken rule on one side of the sentence and
// an obeyed one on the other; two of the three PID rules were decidable.
// The 2026-09-02 note on 60p4b asked the record to stop a later reader taking
// codex to have named itself. The trace does. The why string — the only one of
// the two a `runtime check` reader ever sees — did not.
//
// THE PIN, and it is two-way on purpose: whether a runtime's reply named a
// token is a fact the ADR records, and the why string must agree with it in
// whichever direction the ADR points. Grok emitted the PID's own token and
// its why says so; codex emitted neither and its why must say that. Neither
// side is hardcoded here — the ADR paragraph decides which arm runs, so a
// re-measurement that flips the fact reds this until the why is rewritten.
func TestQAPrecedenceWhyAgreesWithTheTraceAboutTheToken(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../docs/adr/0013-rules-precedence-probe.md")
	if err != nil {
		t.Fatal(err)
	}
	// Anchor on the Verdicts section: "**Codex — " also opens a paragraph in
	// the structural-placement section above, which decides nothing about a
	// token and would silently make this pin read the wrong text.
	adr := string(b)
	if i := strings.Index(adr, "\n### Verdicts\n"); i < 0 {
		t.Fatalf("docs/adr/0013-rules-precedence-probe.md has no Verdicts section — this pin is reading nothing")
	} else {
		adr = adr[i+len("\n### Verdicts\n"):]
		if j := strings.Index(adr, "\n### "); j > 0 {
			adr = adr[:j]
		}
	}
	// A why that discloses a token nobody emitted. "the PID's own token" —
	// grok's, a token that WAS emitted — deliberately does not match.
	noToken := regexp.MustCompile(`(?i)neither[^.;]{0,40}token|no token|token[^.;]{0,40}(not emitted|never)`)

	a := checkApp(t)
	for _, c := range []struct{ name, marker string }{
		{"codex", "**Codex — "},
		{"grok", "**Grok — "},
	} {
		i := strings.Index(adr, c.marker)
		if i < 0 {
			t.Fatalf("the ADR carries no %q verdict paragraph, so this pin is reading nothing about %s — re-derive it against the trace rather than deleting it", c.marker, c.name)
		}
		para := adr[i:]
		if j := strings.Index(para, "\n\n"); j > 0 {
			para = para[:j]
		}
		if !strings.Contains(para, "token") {
			t.Fatalf("the %s verdict paragraph no longer decides anything about a token:\n%s", c.name, para)
		}
		rt, err := a.LoadRuntime(c.name)
		if err != nil {
			t.Fatal(err)
		}
		// The surface a reader consults is the printed grid, not the source
		// comment above the entry — which had the caveat right all along.
		var out bytes.Buffer
		a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &out)
		flat := strings.Join(strings.Fields(out.String()), " ")
		why := strings.Join(strings.Fields(rt.RulesPrecedenceWhy), " ")
		if !strings.Contains(flat, why) {
			t.Fatalf("%s: the grid does not print rules_precedence_why:, so pinning the why says nothing about what a reader sees:\n%s", c.name, out.String())
		}

		traceSaysNone := strings.Contains(para, "neither")
		switch {
		case traceSaysNone && !noToken.MatchString(why):
			t.Errorf("%s: the trace records a reply that emitted NEITHER token, so the verdict rests on a two-signal read of the other rules — but the why a reader is shown does not say the token went unemitted, and counts that one silence as a broken AGENTS rule and an obeyed PID one at the same time (ranger-base-u2etb, ranger-base-60p4b):\n  trace: %s\n  why:   %s", c.name, para, why)
		case !traceSaysNone && noToken.MatchString(why):
			t.Errorf("%s: the why says no token was emitted, but the trace records one:\n  trace: %s\n  why:   %s", c.name, para, why)
		}
	}
}
