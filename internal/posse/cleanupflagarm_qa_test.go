//go:build !posse_arm2 && !posse_arm3

package posse

// THE CLEANUP MODE CAN ARRIVE AS A FLAG, and then the message arm cannot see
// it (ranger-base-dgfax, found by the verify of ranger-base-r5j00).
//
// messageArm asks for git's cleanup mode exactly once, and it asks git's
// CONFIG for it:
//
//	posse_clean=$(git config --get commit.cleanup 2>/dev/null) || posse_clean=''
//
// `git commit --cleanup=<mode>` sets the mode for that one commit and writes
// nothing to config, so the read comes back EMPTY and the arm takes its
// "default" branch, where $posse_typed decides — and $posse_typed is
// GIT_EDITOR=":", which is git saying it opened no editor
// (ranger-base-r5j00). Put an editor on the same command line and the two
// disagree: git's mode is "verbatim", so it KEEPS every '#' line its own
// template wrote, while the arm reads the file through `git stripspace
// --strip-comments` and checks 3 and 4 scan less than git commits. That is
// the ranger-base-6y3z2 class — git's own template landing unscanned under a
// keep mode — surviving where the mode is spelled as a flag rather than as
// config, which is the one spelling 6y3z2's fix does not reach.
//
// MEASURED 2026-11-14, git 2.56.0, commit.cleanup UNSET, one '#' line in the
// message and a recorder hook printing $GIT_EDITOR, over the six modes crossed
// with the editor. Three rows keep the line while the arm would strip it:
//
//	flag mode      editor   GIT_EDITOR   git      the arm reads
//	(none)         no       :            KEEPS    whole    ok
//	(none)         yes      <editor>     strips   strip    ok
//	whitespace     no       :            KEEPS    whole    ok
//	whitespace     yes      <editor>     KEEPS    strip    UNSCANNED
//	verbatim       no       :            KEEPS    whole    ok
//	verbatim       yes      <editor>     KEEPS    strip    UNSCANNED
//	scissors       no       :            KEEPS    whole    ok
//	scissors       yes      <editor>     KEEPS    strip    UNSCANNED
//	strip          no       :            strips   whole    over-refusal
//	strip          yes      <editor>     strips   strip    ok
//	default        no       :            KEEPS    whole    ok
//	default        yes      <editor>     strips   strip    ok
//
// WHY IT IS NOT ALREADY STATED. messageArm's own RESIDUALS paragraph names
// this spelling — "`--cleanup=scissors` as a flag is invisible like every
// other" — but files it under **Fail-CLOSED**, and the three rows above are
// the fail-OPEN half of the same invisibility. The paragraph beside it is
// narrower still: "The one way GIT_EDITOR and git's cleanup disagree is a
// caller who exports GIT_EDITOR=: themselves and passes `-e`: ... over-refusal,
// the fail-closed side." These are three more ways, all in the leak
// direction. ranger-base-r5j00 re-derived and rewrote that very sentence, so
// this is its escape in the same sense r5j00 was ranger-base-mfryj's: the
// claim was made narrower than the fact while the evidence was reachable.
//
// WHAT THE HOOK CAN AND CANNOT DO. prepare-commit-msg is handed ($1 message
// file, $2 source, $3 sha) and nothing else, so the flag is not in its argv
// and no reading of the mode from config can find it. The halves are
// therefore: either a reader that can see it (the parent's own command line,
// or the commit-msg hook this arm's banner already names as "the second layer"
// for the editor path), or the records move — the residual list gains a
// fail-OPEN row and the "one way" sentence loses its absolute. Whichever
// half moves, this pin is the arm that was missing.
//
// THIS PIN SHIPS GREEN ASSERTING THE HOLE. When the hole closes it goes red
// with a message saying to flip it; it is not a t.Skip, so it cannot die with
// the fix (ranger-base-r5j00's own pin took this shape under
// ranger-base-4wmiz).

import (
	"strings"
	"testing"
)

// TestQACeilingMessageArmUnderACleanupFlagThatKeepsGitsTemplate drives the
// ceiling's message arm with the keep mode as a FLAG and an editor open. It is
// the branch-name probe of TestQACeilingMessageArmUnderCommitCleanupVerbatim-
// ReadsTheBranchName with the mode moved from config onto the command line,
// which is the whole of the difference under test — so that pin's green is
// what says this one's reading is the flag and not the mode.
func TestQACeilingMessageArmUnderACleanupFlagThatKeepsGitsTemplate(t *testing.T) {
	w := qaCeilingWall(t, "")

	// FIXTURE PREMISE: the mode must NOT be in config, or the arm sees it
	// through the one reader it has and this measures nothing.
	if out, err := w.git(w.priv, nil, "config", "--get", "commit.cleanup"); err == nil {
		t.Fatalf("fixture premise: commit.cleanup must be unset — the flag is the subject here, got %q", out)
	}
	env := qaWiringEditor(t, w)

	// CONTROL 1 / fixture premise: a clean editor commit with the mode on the
	// command line LANDS and git KEEPS its own template. Without this the
	// probe below is a commit with nothing in it that could have leaked.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "--cleanup=verbatim", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: a clean editor commit under --cleanup=verbatim must land: %v\n%s", err, out)
	}
	qaKeptItsTemplate(t, w, w.priv)

	// PROBE: the branch NAME carries the ceiling class and nothing else does,
	// so the only route into the object is git's own "On branch" line — the
	// line this mode keeps and the arm strips.
	branch := qaExportStem + "41"
	if out, err := w.git(w.priv, nil, "branch", "-m", branch); err != nil {
		t.Fatalf("git branch -m: %v %s", err, out)
	}
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, cerr := w.git(w.priv, env, "commit", "--cleanup=verbatim", "--", "internal/posse/probe.go")

	if cerr != nil {
		if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
			t.Fatalf("the commit was refused, but not by the commit-message arm — refused for some other reason?\n%s", out)
		}
		t.Errorf("THE HOLE IS CLOSED (ranger-base-dgfax): the message arm now reads WHOLE under a cleanup " +
			"mode passed as a FLAG, so a keep mode no longer leaves git's own template unscanned.\n" +
			"FLIP THIS PIN: the want is now `cerr != nil` and the class must NOT reach the object; retype " +
			"this file's head comment, and correct messageArm's residual list (the flag spelling is filed " +
			"under Fail-CLOSED) and the \"one way GIT_EDITOR and git's cleanup disagree\" sentence above it.")
		return
	}

	landed, lerr := w.git(w.priv, nil, "log", "-1", "--format=%B")
	if lerr != nil {
		t.Fatalf("git log: %v %s", lerr, landed)
	}
	if !strings.Contains(landed, branch) {
		t.Fatalf("fixture premise: the commit landed but git did NOT keep its \"On branch\" line, so there "+
			"was nothing here for the arm to miss — --cleanup=verbatim must keep the template:\n%s", landed)
	}

	// THE HOLE, asserted as the state of the tree today: the class is in the
	// object and the arm said nothing. refusals.log is the second observable —
	// it never carries the class itself (qaNoCeilingVocabulary), so its
	// SILENCE about this commit is what says no arm spoke.
	if rl := w.log(t); strings.Contains(rl, "commit message") {
		t.Errorf("the class reached the object AND refusals.log mentions the message arm — read both:\n"+
			"object:\n%s\nrefusals.log:\n%s", landed, rl)
	}

	// The reading itself, so a `-v` run is the evidence and not just a green
	// tick: this is what landed, unscanned.
	t.Logf("THE HOLE (ranger-base-dgfax), measured: `git commit --cleanup=verbatim` with an editor open "+
		"landed a ceiling class git wrote into its OWN template, and no arm spoke.\ncommit object:\n%s\n"+
		"refusals.log:\n%s", landed, w.log(t))

	// CONTROL 2: the wall is awake in this repo at this moment, so the silence
	// above is this branch of the arm and not a dead gate.
	w.stage(t, w.priv, "internal/posse/ctl2.go", "package posse\n")
	if o, e := w.git(w.priv, w.persona, "commit", "-m", "cite "+qaCeilingHit, "--", "internal/posse/ctl2.go"); e == nil {
		t.Fatalf("CONTROL premise: the same class typed with -m must still be refused:\n%s", o)
	}
}
