//go:build posse_arm3

package posse

// CHECK 3'S TWO REMEDY SENTENCES GET A TEST READER (ranger-base-2nj35,
// discovered from ranger-base-ckcly).
//
// THE CLASS is ranger-base-68631 finding 6, "the remedy promises more than
// the set allows" — ADR 0068's class one step in, a way through that does
// not go through. A refused writer does what the sentence tells them; if
// the sentence is wrong about what the rule allows, the remedy is a loop.
// The only thing that keeps a sentence and a rule in step is a test that
// reads BOTH, and check 4 has one on each of its arms
// (TestVulnPrivateHitsAcceptsEveryDecorationTheRemedyPromises over
// VulnPrivateWayThrough, TestVulnPrivateMessageRemedyPromisesOnlyWhatThe
// RuleAllows over VulnPrivateMessageWayThrough).
//
// CHECK 3 HAD NEITHER. MEASURED 2026-11-13, a grep for IdentityWayThrough
// and IdentityMessageWayThrough over every tracked .go file: the two
// declarations in visibility.go, their three call sites in gates.go, and
// nothing else. Every check-3 pin in this tree reads the RULE — what the
// wall refuses, in which arm, with which flags — and none of them reads
// what the refusal then tells the writer to do.
//
// NOT A COPY OF CHECK 4'S PINS. Check 3's rule is not vulnPrivateBare over
// a line of text but a derived identity-literal match
// (DeriveIdentityLiterals) across three subjects, so the nouns and the
// lines are its own, and the content/path sentence makes two claims about
// BEHAVIOUR that check 4's makes none of: "generalize whatever carries it
// (a comment, a fixture, a hardcoded path)" — none of those three is
// exempt — and "a rename is a fresh added entry and is scanned again, so
// the new name has to be clean too". Both are measured here against the
// real rendered hook, through the visWall fixture the rest of the check-3
// cluster uses.
//
// HELD FROM BOTH ENDS, the shape ranger-base-68631 settled on. Each row
// names a noun the SENTENCE uses and the line or path that noun describes,
// and asserts the noun is still in the sentence AND the wall still does
// what it says. So a narrowing of the rule that leaves the sentence alone
// reds, and a rewording that drops a promise reds too.
//
// THAT IS TWO OF THE THREE DIRECTIONS, and this file's first version said
// it was all of them. The claim was that a reword re-promising what the
// rule refuses "has to displace one of them to be coherent" — MEASURED
// FALSE 2026-11-13 (ranger-base-mxbq0 finding 1, from the verify at
// ranger-base-6rj8w): a clause APPENDED to either constant displaces no
// phrase any row greps for, and both mutants stayed green in all three
// arms. One of the two was also false about the rule, which is the loop
// ranger-base-68631 finding 6 names — a remedy sending a refused writer
// somewhere they are refused again. A promise that is ADDED has by
// construction no phrase for a pin to be missing, so nothing shaped like
// these rows can see it; the third direction needs the sentence read as a
// CLOSED set, which is TestQARemedySentencesAreClosedSets
// (remedyclosedset_test.go, over all four remedy constants in the tree).
// What that reader buys is not the catch itself but the STEP: an edit to
// either constant reds there until its author has come back here and
// measured the clause they wrote. Which is why these test names — "Promises
// Only What Check Three Allows" — are true now and were not before.
//
// ARM 3, with the message pins (checkthreemessage_qa_test.go): both tests
// drive the real hook through a visWall, so they belong in one arm rather
// than compiled into all three, and the message arm is the live route by
// check 3's own reasoning — git normalizes what it stages and does not
// normalize a commit message.
//
// ONE PROMISE HAS NO READER HERE, deliberately: "move the content to the
// instance tree" / "write it in the instance tree". The instance tree is
// outside every stamped repo in this fixture, so there is no commit a test
// can make in it; what the fixture CAN hold is the half that makes the
// advice true — the wall speaks only where the sentence says it speaks,
// which is the private-repo control below.
//
// MUTATION-CHECKED, per alternative, through `go test -overlay` with the
// tracked tree unmodified — eight mutants, all red here. FOUR of them were
// GREEN in all three arms before this file, measured over every test in
// internal/posse named Identity|CheckThree|Visibility|WayThrough|Remedy,
// and all four are the drift this bead was filed over — a sentence
// re-promising what the rule refuses:
//   - the content remedy reworded to exempt its own three carriers ("a
//     comment, a fixture or a hardcoded path may keep it in a test");
//   - the rename clause reworded to promise the opposite ("a rename git
//     pairs with the original is the same entry, so the new name is not
//     scanned again");
//   - the message remedy dropping "your tree and index are exactly where
//     you left them";
//   - the message remedy dropping the bound on the residual it names
//     ("that nothing ever clears" for "that your next commit overwrites").
//
// The rule-direction mutants (the content arm exempting '//' lines,
// --no-renames dropped from the path listing, the message arm reading the
// subject line only) were already caught by the pins that read the RULE,
// and so was the message remedy's first clause — PIN (m) asserts the
// refusal PRINTS "rewrite the commit message", though nothing performed
// one. This file is a second reader that catches each in the sentence's
// own words, and the first that does what the sentence says.
// docs/notes.d/ranger-base-2nj35.md has the full table, the commands, and
// the one clause no render mutation can falsify.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// remedyPromises and the closed-set reader over all four remedy constants
// live in remedyclosedset_test.go, untagged so they run in every arm.
// Reading a phrase out of a sentence is HALF the pattern (ranger-base-mxbq0
// finding 1): it catches a promise dropped and never one added, so the rows
// below are what measures each clause and
// TestQARemedySentencesAreClosedSets is what makes an author write one.

// TestQAIdentityRemedyPromisesOnlyWhatCheckThreeAllows reads
// IdentityWayThrough — the remedy BOTH the content arm and the path arm
// print — against the wall that prints it.
func TestQAIdentityRemedyPromisesOnlyWhatCheckThreeAllows(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	instance := w.literal(t, "instance-path-abs")

	// What the sentence says is REFUSED. These are the phrases a re-promise
	// has to displace: "generalize whatever carries it" is what makes the
	// three carriers below a list of things to FIX rather than a list of
	// places the literal may stay.
	for _, refuses := range []string{
		"nothing that names this operator, this box, or this instance's path belongs in a public repo's history",
		"generalize whatever carries it",
		"a rename is a fresh added entry and is scanned again, so the new name has to be clean too",
	} {
		if !remedyPromises(IdentityWayThrough, refuses) {
			t.Errorf("the remedy no longer says %q, and a sentence that stops naming what it refuses is how ranger-base-68631 finding 6 happened: %q", refuses, IdentityWayThrough)
		}
	}

	// THE CONTENT ARM. Each row is one of the three carriers the sentence
	// names, as a line: `carries` must be refused, and `generalized` — the
	// remedy performed on the same file, which is what a refused writer
	// actually does next — must commit.
	for i, row := range []struct{ noun, carries, generalized string }{
		{
			"a comment",
			"package posse\n\n// " + username + " asked for this one.\n",
			"package posse\n\n// the operator asked for this one.\n",
		},
		{
			"a fixture",
			"package posse\n\nvar qaRemedyWant = \"" + username + "\"\n",
			"package posse\n\nvar qaRemedyWant = \"an-operator\"\n",
		},
		{
			// The instance path's ABSOLUTE form: the ~-abbreviated one is a
			// literal too, so a "generalized" line spelling `~/instance`
			// would be refused by the other literal and this row would be
			// measuring the wrong thing.
			"a hardcoded path",
			"package posse\n\nconst qaRemedyStore = \"" + instance + "/.beads\"\n",
			"package posse\n\nconst qaRemedyStore = \"$BEADS_DIR\"\n",
		},
	} {
		if !remedyPromises(IdentityWayThrough, row.noun) {
			t.Errorf("the remedy no longer names %q as something to generalize, and this row exists because the sentence and the rule can drift either way: %q", row.noun, IdentityWayThrough)
		}
		rel := "internal/posse/remedy" + string(rune('a'+i)) + ".go"
		w.stage(t, w.pub, rel, row.carries)
		out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
		if err == nil {
			t.Errorf("the remedy says to generalize %s, so %s carrying the literal must be refused:\n%s", row.noun, row.noun, out)
			continue
		}
		if !strings.Contains(out, "an operator identity literal in a staged file") {
			t.Errorf("%s must be refused by the CONTENT arm, which is the arm whose remedy this is:\n%s", row.noun, out)
		}
		// remedyPromises and not a raw Contains, for the reason it exists:
		// the refusal prints this sentence HARD-WRAPPED, and "generalize
		// whatever" straddles the break in the constant's current wrapping.
		// MEASURED 2026-11-13 (ranger-base-mxbq0): a pure RE-WRAP of
		// IdentityWayThrough, no word changed, red this line — a pin on the
		// wrapping, which is this bead's own class one file over.
		if !remedyPromises(out, "generalize whatever") {
			t.Errorf("the refusal must print the remedy this test reads:\n%s", out)
		}
		// The remedy performed. A refusal whose way through is still
		// refused is the whole class, so this half is the point of the row.
		w.stage(t, w.pub, rel, row.generalized)
		if out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel); err != nil {
			t.Errorf("the remedy says generalizing %s is the way through — generalized, it must commit: %v\n%s", row.noun, err, out)
		}
	}

	// "belongs in a public repo's history": the gate, and the nearest thing
	// this fixture can hold of "move the content to the instance tree". The
	// instance tree is outside every stamped repo here, so what is measured
	// is that the wall speaks only where the sentence says it speaks.
	if !remedyPromises(IdentityWayThrough, "belongs in a public repo's history") {
		t.Errorf("the remedy no longer scopes itself to a public repo, and check 3 is inside the visibility gate: %q", IdentityWayThrough)
	}
	const privRel = "internal/posse/remedypriv.go"
	w.stage(t, w.priv, privRel, "package posse\n\n// "+username+" asked for this one.\n")
	if out, err := w.git(w.priv, w.persona, "commit", "-m", "x", "--", privRel); err != nil {
		t.Errorf("the same carrier in a PRIVATE-stamped repo must commit: %v\n%s", err, out)
	}

	// THE PATH ARM, from the one starting position the sentence's rename
	// clause is actually about: a path ALREADY in history.
	// TestQAIdentityPathArmIsAddedEntriesOnly measures that such a path may
	// be modified and deleted freely — so a writer could reasonably read
	// the wall as having grandfathered the name. The sentence says
	// otherwise, in as many words, and that is what is measured here.
	for _, promise := range []string{
		"For a PATH: name the file without it",
		"a rename is a fresh added entry and is scanned again",
	} {
		if !remedyPromises(IdentityWayThrough, promise) {
			t.Errorf("the remedy no longer promises %q: %q", promise, IdentityWayThrough)
		}
	}
	legacy := "docs/runbooks/" + username + "-legacy.md"
	w.plant(t, w.pub, legacy, "already in history\n")
	dirty := "docs/runbooks/" + username + "-current.md"
	if out, err := w.git(w.pub, nil, "mv", legacy, dirty); err != nil {
		t.Fatalf("git mv: %v %s", err, out)
	}
	out, err := w.git(w.pub, w.persona, "commit", "-m", "rename", "--", legacy, dirty)
	if err == nil {
		t.Fatalf("the remedy says the new name has to be clean too — a rename to another identity-bearing path must be refused:\n%s", out)
	}
	if !strings.Contains(out, "an operator identity literal in a staged PATH") || !strings.Contains(out, dirty) {
		t.Errorf("the refusal must come from the PATH arm and name the rename's DESTINATION:\n%s", out)
	}
	// And only the destination: the source is already in history, which is
	// the half TestQAIdentityPathArmIsAddedEntriesOnly holds. A rule that
	// refused the source too would make "name the file without it"
	// unreachable — there would be no rename that lands.
	if strings.Contains(out, legacy) {
		t.Errorf("the grandfathered SOURCE path must not be refused — the remedy's rename would then have nowhere to go:\n%s", out)
	}

	// "name the file without it", performed.
	const cleanName = "docs/runbooks/deploy-notes.md"
	if out, err := w.git(w.pub, nil, "mv", dirty, cleanName); err != nil {
		t.Fatalf("git mv: %v %s", err, out)
	}
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "rename", "--", legacy, cleanName); err != nil {
		t.Errorf("the remedy says to name the file without it — a clean new name must commit: %v\n%s", err, out)
	}
}

// TestQAIdentityMessageRemedyPromisesOnlyWhatTheRuleAllows is the same
// shape one constant over, for IdentityMessageWayThrough — the arm that
// matters more, by check 3's own reasoning: git normalizes what it stages
// and does not normalize a commit message.
//
// PIN (r) in checkthreemessage_qa_test.go already measures two of this
// sentence's claims (HEAD is unchanged, .git/COMMIT_EDITMSG still holds the
// message) and nothing reads the rest. What is new here is the sentence's
// FIRST clause — "rewrite the commit message", the way through itself,
// which no pin performed — and the three claims the writer's next command
// depends on: the index is where they left it, the rewritten message
// commits, and it overwrites the residual the previous sentence just told
// them about.
func TestQAIdentityMessageRemedyPromisesOnlyWhatTheRuleAllows(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")

	// What the sentence says is REFUSED, the half a re-promise displaces.
	for _, refuses := range []string{
		"Nothing that names this operator, this box, or this instance's path belongs in a public repo's history",
		"a message replicates with the branch exactly as a staged line does",
		"generalize whatever carries it",
	} {
		if !remedyPromises(IdentityMessageWayThrough, refuses) {
			t.Errorf("the message remedy no longer says %q: %q", refuses, IdentityMessageWayThrough)
		}
	}

	// A HEAD to move off: the scratch repo starts with no commit at all.
	w.plant(t, w.pub, "internal/posse/rbase.go", "package posse\n")
	head0, err := w.git(w.pub, nil, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("fixture premise: the scratch repo must have a HEAD: %v %s", err, head0)
	}

	const rel = "internal/posse/rmsg.go"
	const body = "package posse\n\n// nothing identifying in here at all.\n"
	out, err := qaMsgCommit(t, w, w.pub, rel, body, "-F -", "wire the reaper\n\n"+username+" asked for it\n", w.persona)
	if err == nil {
		t.Fatalf("fixture premise: the message must be refused:\n%s", out)
	}

	// "Nothing was committed: HEAD is unchanged and your tree and index are
	// exactly where you left them." PIN (r) holds the HEAD half; the INDEX
	// half is what the NEXT sentence rests on — a writer told to rewrite the
	// message has to still have something staged to commit.
	//
	// WHAT THIS HALF IS A PIN ON, stated because it is not our render.
	// MEASURED 2026-11-13, git 2.56.0: at prepare-commit-msg time
	// GIT_INDEX_FILE points at `.git/next-index-<pid>.lock`, so a hook that
	// resets writes a throwaway (rc 0, the real index untouched), and with
	// the variable unset the reset fails rc 128 on the lock. A refusing
	// hook therefore CANNOT un-stage the writer's work, whatever it does —
	// so this clause is a pin on git's protocol and on the day somebody
	// adds a layer that mutates the index before refusing, not on the
	// check-3 render. Two mutants of that render were measured and neither
	// could falsify it (ranger-base-2nj35; docs/notes.d has the table).
	if !remedyPromises(IdentityMessageWayThrough, "your tree and index are exactly where you left them") {
		t.Errorf("the message remedy no longer promises the index survives, and the rewrite it asks for needs it to: %q", IdentityMessageWayThrough)
	}
	staged, err := w.git(w.pub, nil, "diff", "--cached", "--name-only", "HEAD")
	if err != nil {
		t.Fatalf("git diff --cached: %v %s", err, staged)
	}
	if !strings.Contains(staged, rel) {
		t.Errorf("the remedy says the index is where you left it — %s must still be staged, got %q", rel, staged)
	}
	if b, rerr := os.ReadFile(filepath.Join(w.pub, filepath.FromSlash(rel))); rerr != nil || string(b) != body {
		t.Errorf("the remedy says the tree is where you left it — %s must be untouched (err=%v): %q", rel, rerr, string(b))
	}

	// "the way through: rewrite the commit message" — performed, with
	// nothing else changed. This is the clause no pin had: a remedy whose
	// own instruction is still refused is the class this file exists for.
	// The rewrite is the sentence's own prescription: say what you did
	// without the identifier, and cite the bead id.
	for _, promise := range []string{"rewrite the commit message", "cite the bead id"} {
		if !remedyPromises(IdentityMessageWayThrough, promise) {
			t.Errorf("the message remedy no longer promises %q: %q", promise, IdentityMessageWayThrough)
		}
	}
	const rewritten = "wire the reaper\n\nthe operator asked for it (ranger-base-2nj35)\n"
	if out, err := w.gitIn(w.pub, w.persona, rewritten, "commit", "-F", "-", "--", rel); err != nil {
		t.Fatalf("the way through must go through — a rewritten message must commit: %v\n%s", err, out)
	}
	head1, err := w.git(w.pub, nil, "rev-parse", "HEAD")
	if err != nil || head1 == head0 {
		t.Fatalf("the rewrite must really commit: %q -> %q (%v)", head0, head1, err)
	}

	// "The message you typed is still in .git/COMMIT_EDITMSG ... and ...
	// your next commit overwrites" it. PIN (r) measures the first half; the
	// overwrite is the half that says the residual is bounded, and a
	// residual sentence that overstates its own cleanup is the same defect
	// pointing the other way.
	if !remedyPromises(IdentityMessageWayThrough, "that your next commit overwrites") {
		t.Errorf("the message remedy no longer bounds the residual it names: %q", IdentityMessageWayThrough)
	}
	editmsg, rerr := os.ReadFile(filepath.Join(w.pub, ".git", "COMMIT_EDITMSG"))
	if rerr != nil {
		t.Fatalf("reading COMMIT_EDITMSG: %v", rerr)
	}
	if strings.Contains(string(editmsg), username) || !strings.Contains(string(editmsg), "the operator asked for it") {
		t.Errorf("the remedy says the next commit overwrites the residual — COMMIT_EDITMSG must hold the rewritten message and not the literal: %q", string(editmsg))
	}

	// "a message replicates with the branch exactly as a staged line does":
	// the same text, both subjects, each refused in its own arm's words —
	// which is what makes the claim a reason and not a flourish.
	carrier := "// " + username + " asked for it"
	const lineRel = "internal/posse/rparity.go"
	w.stage(t, w.pub, lineRel, "package posse\n\n"+carrier+"\n")
	outLine, errLine := w.git(w.pub, w.persona, "commit", "-m", "parity", "--", lineRel)
	if errLine == nil {
		t.Fatalf("fixture premise: the same text as a staged LINE must be refused:\n%s", outLine)
	}
	// A refusal leaves its own subject staged (ranger-base-uzgkz); the
	// message half below must be refused by the MESSAGE arm and nothing
	// else, so the content arm is given nothing to see.
	w.unstage(t, w.pub, lineRel)
	outMsg, errMsg := qaMsgCommit(t, w, w.pub, "internal/posse/rparitym.go", "package posse\n", "-F -", "wire it\n\n"+carrier+"\n", w.persona)
	if errMsg == nil {
		t.Fatalf("the same text in the MESSAGE must be refused:\n%s", outMsg)
	}
	if !strings.Contains(outLine, "an operator identity literal in a staged file") {
		t.Errorf("the staged-line half must be refused by the content arm:\n%s", outLine)
	}
	if !strings.Contains(outMsg, "an operator identity literal in the commit MESSAGE") {
		t.Errorf("the message half must be refused by the MESSAGE arm:\n%s", outMsg)
	}
}
