package posse

// THE TWO GUARDS THAT NARROW THE `auto` DETECTION ARE NOT MEASURED BY
// ANYTHING (ranger-base-n3s4s, verifying ranger-base-vzx2n's close).
//
// vzx2n's fix reads the comment character back off the template git already
// wrote into "$1", because for `auto` the character is a decision git makes
// per message and stripspace has no message to make it against. The read is
// admitted by TWO conditions in messageArm's rendered block: the file
// carries a BARE line of a character git can choose — the LAST such line is
// the one taken — and at least four lines start with that character. Both
// are what keep the detection from firing on a message that has no template
// at all — and where it fires wrongly it strips lines git KEEPS, which is
// the fail-open vzx2n exists to close, reopened.
//
// THAT FIRST CONDITION WAS TWO WHEN THIS FILE WAS WRITTEN (ranger-base-vl9g8,
// ranger-base-dgh7y): vzx2n took the first character of the file's LAST line
// and required a bare line of it as well, and under `commit -v` the last line
// is a diff line, so no character was found at all. vl9g8 collapsed the pair
// into the last BARE comment-character line. The arms below did not change
// and neither did what they say — each still reds under exactly the mutant
// that relaxes its OWN guard, re-measured against the new arm in the table
// at the bottom of this comment.
//
// MEASURED on the close's own tree (2d2f139), `go test -overlay`: relax the
// count to `-ge 1`, or drop the bare-line conjunct, and all four of vzx2n's
// pins plus TestQACeilingMessageArmFollowsTheWritersCommentChar stay GREEN.
// The close's own mutation run flipped the `auto` test itself, which every
// pin sees; the two conditions BELOW it are unpinned, and the fail-open they
// bound is a P1 constitutional wall (ADR 0024 D2 check 3).
//
// Both arms here are reachable from the WRITER's config alone — commit.status
// = false plus core.commentChar = auto, no intent — and in both git appends
// no comment block, so under `--cleanup=strip` every line of the message
// LANDS. Each arm is red under exactly the mutant that relaxes its OWN
// condition and green under the other, so the pin does not merely count
// refusals: it says which guard did the refusing.
//
// MUTATION-CHECKED, run 2026-09-04 on 2d2f139, `go test -overlay`:
//	M-count      `-ge 4` -> `-ge 1`                reds the bare-line arm ALONE
//	M-bareline   the `grep -qaxF` conjunct dropped  reds the four-line arm ALONE
//	M-auto       the `auto` test disabled            reds BOTH
//
// M-auto redding both is expected and is not what these arms are for: with
// `auto` unrecognized the read falls back to plain stripspace, which strips
// '#' whatever git chose, so vzx2n's own hole swallows these fixtures too.
// The discriminating result is the first two rows — under M-count and
// M-bareline every one of vzx2n's four pins and
// TestQACeilingMessageArmFollowsTheWritersCommentChar stay GREEN, and only
// the arm below reds.
//
// RE-MEASURED 2026-09-04 on 50af010, against the arm as vl9g8 left it, same
// method (ranger-base-dgh7y). M-bareline has to be written against the new
// selector to mean the same thing — the bare-line match relaxed to a line
// that merely STARTS with a candidate, `grep -axE` -> `grep -aE '^...'`
// plus `cut -c1`, which is vzx2n's shape generalized:
//	M-count      `-ge 4` -> `-ge 1`      reds the bare-line arm ALONE
//	M-bareline   bare line -> `^` match  reds the four-line arm, AND both
//	                                     `commit -v` ceiling pins
//	M-auto       the `auto` test disabled  reds every `auto` pin in both files,
//	                                       while the explicit-character pin
//	                                       TestQACeilingMessageArmFollowsThe-
//	                                       WritersCommentChar stays green
//
// Run under M-count and M-bareline: vzx2n's four pins and both `commit -v`
// ceiling pins, so each row above is a whole-set result and not one test's.
// Each arm here is still the only thing that reds under the mutant of its
// own guard, and the count guard is measured by exactly one arm.
// M-bareline now reaching the `-v` pins as well is the change vl9g8 made:
// the bare-line rule stopped being a second conjunct and became the selector
// itself, so the ceiling's verbose pin measures it too.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQACheckThreeMessageArmUnderAutoNarrowsWhatCountsAsGitsTemplate(t *testing.T) {
	// A message with FOUR '#'-leading lines and no bare '#' line: the count
	// alone would admit it, and the bare-line conjunct is the only thing
	// that says git wrote no block here.
	t.Run("four commented lines and no bare one is not a template", func(t *testing.T) {
		qaAutoDetectionArm(t, func(username string) string {
			return "# alpha, a note\n" +
				"# beta, another\n" +
				"# gamma, a third\n" +
				"# delta, from " + username + "\n"
		})
	})

	// A message carrying a BARE '#' line but only two '#'-leading lines: the
	// bare-line conjunct alone would admit it, and the count is the only
	// thing that says git wrote no block here (the smallest block git
	// appends was measured at ten lines on vzx2n).
	t.Run("a bare comment line under the count is not a template", func(t *testing.T) {
		qaAutoDetectionArm(t, func(username string) string {
			return "#\n" +
				"# gamma, from " + username + "\n"
		})
	})
}

// qaAutoDetectionArm runs one shape of message-with-no-template through the
// real rendered hook in a PUBLIC repo under core.commentChar=auto and
// commit.status=false, and says the operator's own username may not reach
// the commit object. body is handed the literal so each arm carries it on a
// line the detection would strip if it fired.
func qaAutoDetectionArm(t *testing.T, body func(username string) string) {
	t.Helper()
	w := newVisWall(t)
	username := w.literal(t, "username")
	env := qaAutoCommentCharRepo(t, w, w.pub, body(username))
	if out, err := w.git(w.pub, nil, "config", "commit.status", "false"); err != nil {
		t.Fatalf("git config commit.status: %v %s", err, out)
	}

	w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n\n// nothing identifying in here at all.\n")
	out, err := w.git(w.pub, env, "commit", "--", "internal/posse/probe.go")

	// PREMISE: git appended no comment block, so every line of this message
	// is one git KEEPS and the detection has nothing legitimate to find.
	msg, rerr := os.ReadFile(filepath.Join(w.pub, ".git", "COMMIT_EDITMSG"))
	if rerr != nil {
		t.Fatalf("reading COMMIT_EDITMSG: %v", rerr)
	}
	if strings.Contains(string(msg), "\n; ") {
		t.Fatalf("fixture premise: commit.status=false must have left git's template out of the file:\n%s", msg)
	}

	if err == nil {
		landed, lerr := w.git(w.pub, nil, "log", "-1", "--format=%B")
		if lerr != nil {
			t.Fatalf("git log: %v %s", lerr, landed)
		}
		if strings.Contains(landed, username) {
			t.Errorf("an operator identity literal reached a PUBLIC repo's commit object: the arm took this message "+
				"for git's own template under core.commentChar=auto and stripped lines git KEEPS (ranger-base-vzx2n's "+
				"fail-open, through a detection guard nothing measures):\n%s", landed)
		}
		return
	}
	if !strings.Contains(out, "identity literal in the commit MESSAGE") {
		t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke:\n%s", out)
	}

	// CONTROL: the same repo, the same config, a message with nothing classed
	// in it must still LAND. Reading the file whole is the shape that
	// over-refuses, and this is what says it did not start refusing every
	// commit in this repo.
	clean := qaAutoCommentCharRepo(t, w, w.pub, "# a heading nobody can be identified by\n")
	w.stage(t, w.pub, "internal/posse/ctl.go", "package posse\n")
	if o, e := w.git(w.pub, clean, "commit", "--", "internal/posse/ctl.go"); e != nil {
		t.Fatalf("fixture premise: a commit with nothing classed in it must land: %v\n%s", e, o)
	}
}

// THE `commit.verbose` ARM THAT WAS PARKED HERE IS NOW ONE FILE OVER
// (ranger-base-dgh7y, filed off ranger-base-n3s4s' verify of vzx2n).
//
// What it said: under `auto` vzx2n read the character off the LAST line of
// "$1", and `commit.verbose=true` makes that a DIFF line — git appends its
// scissors marker and the staged diff below its block — so no character was
// detected, the file was read whole, and ranger-base-h3s6q's finding-2
// over-refusal came back: one UNTRACKED path git listed in a block it
// discards refuses an editor commit whose typed message is clean, under a
// remedy ("rewrite the commit message") that clears nothing. Fail-CLOSED, so
// nothing leaked; reachable from two ordinary keys in the writer's own
// config with no intent.
//
// ranger-base-vl9g8 fixed it while this bead was open — the selector is now
// the last BARE comment-character line, which a unified diff cannot contain
// — and landed its own pin for the shape:
// TestQACeilingMessageArmUnderAnAutoCommentCharWithAVerboseCommit, in
// commentcharauto_qa_test.go. The parked pin here was the same wall, the
// same two config keys and the same untracked probe, with a weaker fixture
// premise (it did not assert that the diff was actually appended), so it is
// retired rather than unparked and duplicated.
//
// MEASURED before retiring it, 2026-09-04 on 50af010, `go test -overlay`,
// both pins run side by side: unparked, both GREEN; with vzx2n's selector
// (`sed -n '$p' | cut -c1`) put back, both RED on their own assertion, each
// naming the MESSAGE arm and the untracked path, while the three run beside
// them — the detection arm above with both its subtests, and vzx2n's
// TestQACeilingMessageArmUnderAnAutoCommentCharStillReadsGitsTemplate and
// TestQACheckThreeMessageArmUnderAutoWithNoTemplateAppended — stayed green.
// The surviving pin sees every mutant this one saw.
