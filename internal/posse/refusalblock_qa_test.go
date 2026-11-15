//go:build !posse_arm2 && !posse_arm3

package posse

// The two commit-guard findings of the ranger-base-eclrz verify, pinned
// (ranger-base-hegka). Both are about a refusal SURFACE, and both escaped
// every pin this tree had for a reason worth stating once:
//
//   - every refusal-text pin reads by strings.Contains, so none of them can
//     see a missing newline BETWEEN two substrings that are each present.
//     The matched block and the way-through sentence were rendering as one
//     line and 21 refusal sites' worth of assertions were green over it.
//   - a check whose only signal is output PASSES BY PRODUCING NONE, so no
//     assertion about what check 4 prints can tell a clean scan from a scan
//     that could not run.
//
// Which is why these two are EXECUTED rather than asserted on: each runs the
// real rendered shell under sh and reads the bytes that reach a committer.

import (
	"os/exec"
	"strings"
	"testing"
)

// qaRunRefusalShell runs a fragment of the rendered commit guard the way the
// hook does — POSIX sh, the gates dir out of PATH, RHQ_GATES_DIR unset so no
// refusals.log arm is taken — and hands back everything it wrote.
func qaRunRefusalShell(t *testing.T, script string) (string, int) {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = []string{"PATH=" + PathOutsideGates(""), "RHQ_PERSONA=qa"}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("sh: %v\n%s", err, out)
	}
	return string(out), code
}

// FINDING 2 (escaped ranger-base-g1jqt). The matched lines of a refusal must
// end their own line, whatever built them.
//
// THE TWO SHAPES ARE THE WHOLE TEST, because a fix that only served one of
// them would be worse than the defect: the arms that build a block line by
// line embed a trailing newline (the data ceiling's $posse_bad, the
// constitution class's $posse_cls_hit) and the arms that read one out of a
// `$(command substitution)` cannot, since that strips every trailing newline
// (check 4's two arms). The old template printed the variable with
// `printf '%s'` and added nothing, so the first shape rendered correctly and
// the second ran its last matched line into the way-through sentence.
// posse_matched_block gives both exactly one.
func TestQAEveryRefusalMatchedBlockEndsItsOwnLine(t *testing.T) {
	t.Parallel()
	// The real refusal from the finding: check 4's message arm, whose badVar
	// is a command substitution's output.
	r := vulnPrivateSource().message
	const hitLine = `  ranger-base-xxxxx: a line here, or a line continuing it, carries more than the bare id — the text is withheld (AGENTS.md "Landing the plane")`
	way := r.wayThrough
	if i := strings.IndexByte(way, '\n'); i >= 0 {
		way = way[:i]
	}
	firstWords := way
	if len(firstWords) > 24 {
		firstWords = firstWords[:24]
	}

	for _, leg := range []struct {
		name, badVar string
		wantBlank    bool
	}{
		// What a $(command substitution) hands the template: no trailing
		// newline at all. This is the shape that rendered as one line.
		{"from a command substitution", hitLine, false},
		// What an arm that builds its block line by line hands it. It
		// rendered correctly before and must still: one newline, not two.
		{"built with embedded newlines", hitLine + "\n", false},
		// Two lines, the multi-hit case, so the fix is not "print the last
		// line again".
		{"two lines, no trailing newline", hitLine + "\n" + hitLine, false},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			script := hookStampFunc + hookJudgeFunc + hookMatchedBlockFunc +
				"posse_bad=" + shQuote(leg.badVar) + "\n" + r.render("")
			out, code := qaRunRefusalShell(t, script)
			if code != 1 {
				t.Fatalf("the refusal must exit 1; got %d:\n%s", code, out)
			}
			// THE ASSERTION, and it is about a line boundary and not about
			// either sentence: the way-through must START a line of its own.
			// Contains() is what every pin before this one asked, and it was
			// true throughout the defect.
			if !strings.Contains(out, "\n"+way) {
				t.Errorf("the way-through does not start its own line — the matched block ran into it. "+
					"A refusal is the one surface this check exists to make readable (ranger-base-hegka, ranger-base-g1jqt).\nwant a line starting %q\ngot:\n%s",
					firstWords, out)
			}
			// …and exactly one newline, so the arms that embed their own are
			// not double-spaced into a different defect.
			if strings.Contains(out, hitLine+"\n\n") {
				t.Errorf("a blank line was added between the matched block and the way-through — posse_matched_block must give exactly ONE newline, whatever it is handed:\n%s", out)
			}
		})
	}
}

// And the census beside it: every matched block in the commit guard goes
// through posse_matched_block, so an arm written tomorrow inherits the
// property instead of rediscovering it.
//
// The bd shim is deliberately not in scope (it is a different render, with
// no helper in it, and its block is built line by line); this reads the
// commit guard alone.
func TestQAEveryCommitGuardMatchedBlockGoesThroughTheHelper(t *testing.T) {
	t.Parallel()
	render := qaNoHome.CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	calls := strings.Count(render, "posse_matched_block \"$")
	// Floor, not a count: the three arms that print one today are check 4's
	// two (through visGuardRefusal.render) and the data ceiling's report,
	// plus the constitution class — a bead that adds a refusal raises it.
	if calls < 3 {
		t.Errorf("the rendered commit guard calls posse_matched_block %d times; it prints more matched blocks than that — a refusal that prints one by hand is the defect ranger-base-hegka closed", calls)
	}
	if !strings.Contains(render, "posse_matched_block() {") {
		t.Fatal("the rendered commit guard calls posse_matched_block and does not define it")
	}
	for i, line := range strings.Split(render, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, `printf '%s' "$`) {
			continue
		}
		// posse_matched_block's own body is the one site that may: it is
		// where the normalization happens.
		if strings.Contains(trim, `printf '%s\n' "$(printf '%s' "$1")"`) {
			continue
		}
		t.Errorf("hook line %d prints a variable with no trailing newline of its own — if it is a refusal's matched block, print it through posse_matched_block (ranger-base-hegka):\n\t%s", i+1, trim)
	}
}

// FINDING 3 (pre-existing in ranger-base-ihp7y's check 4). The scan's own
// failure to RUN must not read as a clean scan.
//
// Driven with a shell FUNCTION named posse rather than a binary on PATH,
// because `command -v posse` finds a function and the hook's arm is
// byte-identical either way — so the three answers a real posse can give are
// three one-line fixtures here. The fourth leg has no posse at all, which is
// the finding's own repro.
func TestQACheckFourSaysWhenItCouldNotLook(t *testing.T) {
	t.Parallel()
	const skipped = "SKIPPED, not passed"
	for _, leg := range []struct {
		name, posse       string
		wantHit, wantSkip bool
		noPath            bool
	}{
		// A HIT: lines on stdout, nonzero. Must fold into $posse_bad and
		// must NOT say skipped — the leg that makes the fix a discrimination
		// rather than a blanket warning.
		{name: "a hit", posse: "posse() { echo '  ranger-base-xxxxx: more than the bare id'; return 1; }", wantHit: true},
		// A CLEAN scan: zero, nothing printed. Silent, as before.
		{name: "a clean scan", posse: "posse() { return 0; }"},
		// COULD NOT LOOK: nonzero, nothing printed — a session gate refusing
		// the verb, a panic, a usage error. Indistinguishable from the row
		// above until the exit code was read.
		{name: "a posse that refuses the verb", posse: "posse() { return 1; }", wantSkip: true},
		{name: "a posse that dies", posse: "posse() { return 2; }", wantSkip: true},
		// NO POSSE AT ALL, with PATH emptied so `command -v posse` fails.
		{name: "no posse on PATH", posse: "", wantSkip: true, noPath: true},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			script := hookVulnPrivateSkipFunc + leg.posse + "\n" +
				"posse_added='- ranger-base-xxxxx and a description of it'\nposse_bad=''\n" +
				vulnPrivateSource().checks("") +
				"printf 'BAD[%s]\\n' \"$posse_bad\"\n"
			cmd := exec.Command("sh", "-c", script)
			cmd.Env = []string{"PATH=" + PathOutsideGates(""), "RHQ_PERSONA=qa"}
			if leg.noPath {
				cmd.Env = []string{"PATH=", "RHQ_PERSONA=qa"}
			}
			raw, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sh: %v\n%s", err, raw)
			}
			out := string(raw)
			if got := strings.Contains(out, "BAD[  ranger-base-xxxxx"); got != leg.wantHit {
				t.Errorf("hit folded into $posse_bad = %v, want %v:\n%s", got, leg.wantHit, out)
			}
			if got := strings.Contains(out, skipped); got != leg.wantSkip {
				t.Errorf("said %q = %v, want %v — a scan that could not look must SAY so, and a clean scan must stay silent (ranger-base-hegka):\n%s",
					skipped, got, leg.wantSkip, out)
			}
			// It says it and still passes: a scan that could not run is not
			// a finding, which is ADR 0068's lockout lesson and the reason
			// check 4 fails open at all.
			if leg.wantSkip && strings.Contains(out, "refused by posse gate") {
				t.Errorf("a scan that could not look must not REFUSE the commit — the committer has no remedy for it (ADR 0068):\n%s", out)
			}
		})
	}
}

// The judge's half of that contract, where the shell now reads it: a hit
// PRINTS and answers true, a clean scan prints nothing and answers false.
// cmd/posse/main.go turns the bool into the exit code, so these two pairs are
// what make "nonzero with nothing on stdout" mean "could not look" and
// nothing else.
func TestQAVulnPrivateScanPrintsExactlyWhenItAnswersAHit(t *testing.T) {
	t.Parallel()
	const openResidual = `[{"id":"ranger-base-l751c","status":"in_progress","labels":["vuln-private"]}]`
	for _, leg := range []struct {
		name, text string
		wantHit    bool
	}{
		{"a residual described", "- ranger-base-l751c\nthe parser overflows\n", true},
		{"the bare id alone", "- ranger-base-l751c\n", false},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFakeOpenIssues(t, dir, openResidual)
			var out strings.Builder
			hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, VulnPrivateMessageArm, strings.NewReader(leg.text), &out)
			if hit != leg.wantHit {
				t.Fatalf("hit = %v, want %v", hit, leg.wantHit)
			}
			if printed := out.Len() > 0; printed != leg.wantHit {
				t.Errorf("printed %d bytes with hit=%v — the hook reads an empty stdout from a NONZERO exit as \"could not look\" "+
					"(ranger-base-hegka), so a hit that prints nothing, or a clean scan that prints, breaks that reading: %q",
					out.Len(), hit, out.String())
			}
		})
	}
}
