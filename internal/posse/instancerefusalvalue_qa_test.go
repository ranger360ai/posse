package posse

// AN INSTANCE PATTERN'S REFUSAL NAMES THE CLASS, NEVER THE VALUE
// (ADR 0048 D2, ranger-base-8114t, from ranger-base-856sv step 4).
//
// WHAT WAS MEASURED. With the operator's own pattern stamped and a bare
// pre-publication name staged in a .md, the refusal was correct and its body
// was not: `<class>: <the whole ERE>` followed by `matched: <the name>`, on
// stderr and therefore in the terminal, the transcript, and whatever bead
// someone pastes a refusal onto. The value is the one thing the config key
// exists to keep out of a public tree, so a wall that prints it back has
// moved the leak from the commit to the refusal.
//
// THE RULE, and it is per ENTRY and not per check: an entry from config
// beads_visibility_patterns: is refused by CLASS and HIT COUNT alone,
// wherever it is scanned — check 0's beads jsonl and all THREE of check 3's
// arms since ranger-base-qk8i9 gave it the commit message (the message arm's
// own withholding pin is TestQAInstancePatternRefusesTheCommitMessage-
// ClassOnly, checkthreemessage_qa_test.go).
// A shipped OpsPattern keeps ADR 0024 D2's shape: its text is in this
// repo's own source, so showing a writer the string they tripped on costs
// nothing and is what makes the refusal actionable.
//
// THE ONE RESIDUAL, stated: check 3's PATH arm still prints the offending
// PATH, which is the only thing that says which file — and a path that
// matched necessarily carries the vocabulary. It is the writer's own staged
// artifact, already in their tree and their terminal; what it is not is the
// operator's pattern or a line of someone else's content. The pins below
// hold the path to exactly that: the name may appear in the path line and
// nowhere else.
//
// EVERY PIN HERE IS MUTATION-CHECKED. The five mutants, and what each one
// reds — run on ranger-base-8114t, one at a time against a golden copy:
//
//	M1  exChecks renders plain (opsCheckCall(..., false)), so check 3's arms
//	    disclose again ..... PIN 1, PIN 2, PIN 4 (and, since qk8i9, the
//	    message arm's withholding pin — one closure, all three arms)
//	M2  check 0's set.Extra loop renders plain ..... PIN 3, PIN 4, and
//	    TestInstanceOpsPatternGuardsAPublicRepo
//	M3  posse_check's class-only branch writes $2 and the matched text
//	    after all ..... PIN 1, PIN 2, PIN 3, and TestInstanceOpsPattern-
//	    GuardsAPublicRepo — but NOT PIN 4, which reads the render and not a
//	    refusal, and that split is why both kinds of pin are here
//	M4  opsCheckCall appends the switch unconditionally, so a shipped
//	    pattern goes silent too ..... PIN 3 (its shipped half), PIN 4, and
//	    TestQAShippedPatternsStayMarkdownOnly
//	M5  opsCheckCall never appends it ..... all four, and
//	    TestInstanceOpsPatternGuardsAPublicRepo
//	M6  posse_check writes a literal `1 hit(s)` instead of $posse_n ..... PIN
//	    1 alone, whose fixture is the only one that matches twice
//
// Check 3's content, path and message arms render from ONE closure at two
// indents, so no mutant separates PIN 1 from PIN 2. They are still two pins: the
// path arm is the one place a matching subject is legitimately printed, and
// PIN 2 is the assertion that says exactly how far that goes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaHookFile reads the prepare-commit-msg the wall actually stamped. A pin
// over a Go value would be green against a render nothing installs.
func qaHookFile(t *testing.T, repo string) string {
	t.Helper()
	dir, err := hooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "prepare-commit-msg"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// qaNoVocabulary is the assertion every arm makes: the pattern's ERE is
// nowhere in the text, the disclosure line posse_check writes for a shipped
// entry is nowhere in it, and the NAME appears only on lines that carry one
// of the allowed subjects (a staged path, for the path arm; nothing at all
// for the others). Line-scoped rather than whole-text, because "the name is
// absent" and "the name is absent except where the writer put it" are
// different claims and only the second one is true of the path arm.
func qaNoVocabulary(t *testing.T, what, text string, allowedOn ...string) {
	t.Helper()
	if strings.Contains(text, qaInstanceERE) {
		t.Errorf("%s carried the instance pattern's ERE — that IS the confidential value:\n%s", what, text)
	}
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, qaInstanceName) {
			continue
		}
		ok := false
		for _, allow := range allowedOn {
			if allow != "" && strings.Contains(line, allow) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s printed the instance vocabulary on a line that is not an allowed subject:\n\t%q\nfull text:\n%s", what, line, text)
		}
	}
}

// PIN 1: check 3's CONTENT arm — the shape ranger-base-8114t measured, one
// file over.
// The refusal names the class and a count; the ERE and the matched text are
// gone from stdout, stderr and refusals.log.
//
// MUTATION-CHECKED: M1 and M5 (the render stops muting) and M3 (the shell
// function stops withholding) each red this pin; M2 and M4, which move the
// switch on check 0 and on the shipped list, leave it green — this pin owns
// check 3's content arm and says nothing about the other two sites. M6, a
// posse_check that writes a literal `1 hit(s)` instead of $posse_n, reds
// this pin ALONE: it is the only one whose fixture matches twice.
func TestQAInstanceRefusalWithholdsTheValueInCode(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = "internal/posse/notes.go"
	// TWO matching added lines, and the count is what says the number is
	// counted rather than spelled: a refusal that always says "1 hit(s)"
	// would pass every other assertion here.
	w.stage(t, w.pub, rel, "package posse\n\n// the "+qaInstanceName+" harness shipped this in 2025.\n"+
		"// and the "+qaInstanceName+" wall came later.\n")

	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Fatalf("fixture premise: the instance pattern must refuse this line:\n%s", out)
	}
	// It is still a usable refusal: the class, a count, and the remedy.
	for _, want := range []string{
		"an instance-defined visibility class in a staged file",
		qaInstanceClass + ": 2 hit(s)",
		"never the text it matched", // the remedy says so on one line, wrapped prose or not
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
	// And the `matched: <text>` line a shipped entry writes is not there.
	if strings.Contains(out, "matched: ") {
		t.Errorf("an instance entry must not write posse_check's matched-text line:\n%s", out)
	}
	qaNoVocabulary(t, "the terminal refusal", out)
	qaNoVocabulary(t, "refusals.log", w.log(t))
}

// PIN 2: check 3's PATH arm. The path IS printed — it is the only thing
// that names the file, and it is the writer's own staged artifact — and it
// is the ONLY place the name may appear.
//
// MUTATION-CHECKED: M1, M3 and M5 red it, the same three that red PIN 1 —
// one closure feeds both arms. What is NOT shared is the assertion: this is
// the only pin that says the name may appear on the path line, so a fix
// that suppressed the path itself would leave PIN 1 green and red this one
// on the missing subject.
func TestQAInstanceRefusalWithholdsTheValueOnAPath(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = "internal/" + qaInstanceName + "/doc.go"
	// Content spotless: the CONTENT arm must not be what refuses this, or
	// this pin is PIN 1 wearing a path.
	w.stage(t, w.pub, rel, "package doc\n")

	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Fatalf("fixture premise: a staged PATH carrying the name must be refused:\n%s", out)
	}
	for _, want := range []string{
		"an instance-defined visibility class in a staged PATH",
		rel, // the subject, which is what makes the refusal actionable
		qaInstanceClass + ": 1 hit(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the path refusal must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "matched: ") {
		t.Errorf("an instance entry must not write posse_check's matched-text line:\n%s", out)
	}
	qaNoVocabulary(t, "the terminal refusal", out, rel)
	qaNoVocabulary(t, "refusals.log", w.log(t))
}

// PIN 3: check 0, the beads jsonl — the arm ADR 0048 D2 deliberately LEFT
// the instance patterns in, and the one whose refusal is a mixed list. One
// staged line trips a shipped class and this instance's, and the two
// entries print differently in the SAME refusal: that is what says the
// switch is per entry rather than a muted check.
//
// MUTATION-CHECKED:
//   - M2, check 0's own loop renders plain: reds this pin and PIN 4 and
//     leaves both check 3 pins GREEN — they stage a .go file, which check 0
//     never reads, so this is the only pin that covers that site.
//   - M4, everything class-only: reds this pin's SHIPPED half — the
//     `matched: $715/wk` line goes away — together with
//     TestQAShippedPatternsStayMarkdownOnly. "The shipped list is
//     unchanged" is a claim in both directions, and a one-way assertion is
//     met by a wall that has gone quiet about everything.
func TestQAInstanceRefusalWithholdsTheValueInTheBeadsDb(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = ".beads/issues.jsonl"
	const spend = "$715/wk"
	w.stage(t, w.pub, rel, `{"id":"x-1","title":"onboarding","description":"the `+
		qaInstanceName+` handoff, and last window burned `+spend+`"}`+"\n")

	out, err := w.git(w.pub, w.persona, "commit", "-m", "bd sync", "--", rel)
	if err == nil {
		t.Fatalf("fixture premise: check 0 must refuse this line:\n%s", out)
	}
	for _, want := range []string{
		"ops-class content in a public repo's beads db",
		qaInstanceClass + ": 1 hit(s)", // this instance's: class and count
		"cost:",                        // a shipped class in the same refusal...
		"matched: " + spend,            // ...keeps ADR 0024 D2's shape
	} {
		if !strings.Contains(out, want) {
			t.Errorf("check 0's refusal must carry %q:\n%s", want, out)
		}
	}
	qaNoVocabulary(t, "the terminal refusal", out)
	qaNoVocabulary(t, "refusals.log", w.log(t))
}

// PIN 4: the render itself, which is what every hooked repo on this box
// carries. Exactly the three instance call sites are class-only; no shipped
// pattern and no derived identity literal is. Reading it off the stamped
// FILE rather than off a Go value is the point — the file is what runs.
//
// MUTATION-CHECKED: M1 drops the count to 1 and M2 to 2; M5 drops it to 0;
// M4 raises it past 3 and trips the per-line check, which is what makes
// this a claim about WHICH entries rather than about how many. M3 leaves it
// green — it changes what the shell function prints, not what is stamped —
// and that is the pin split this file is built on.
func TestQAOnlyInstancePatternsAreClassOnlyInTheRender(t *testing.T) {
	w := qaInstanceWall(t)
	hook := qaHookFile(t, w.pub)

	var classOnly, calls int
	for _, line := range strings.Split(hook, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "posse_check ") {
			continue
		}
		calls++
		if !strings.HasSuffix(trimmed, " "+opsClassOnlyArg) {
			continue
		}
		classOnly++
		if !strings.HasPrefix(trimmed, "posse_check '"+qaInstanceClass+"'") {
			t.Errorf("a call that is not this instance's pattern was rendered class-only:\n\t%s", trimmed)
		}
	}
	// check 0, check 3's content arm, check 3's path arm.
	if classOnly != 3 {
		t.Errorf("want the instance pattern class-only at its 3 call sites, got %d of %d posse_check calls:\n%s", classOnly, calls, hook)
	}
	// FIXTURE PREMISE: there are plain calls too, or the count above is
	// green over a hook that renders one list and nothing else.
	if calls <= classOnly {
		t.Fatalf("fixture premise: the hook must also render shipped and identity calls, got %d calls", calls)
	}
}
