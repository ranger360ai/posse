package posse

// THE CUT COULD BE FORGED BY THE WRITER, AND THE CLASS BELOW IT LANDED
// (ranger-base-d94zl, found verifying ranger-base-xfgcn's close; fixed in
// ranger-base-gyrnp).
//
// ranger-base-xfgcn stopped the message arm at git's cut line, which is the
// right fix for the over-refusal it was filed about. The cut was licensed by
// `commit.cleanup=scissors`, or a `diff --` line below the marker, "which
// only a verbose commit appends" — and nothing in the arm asked who wrote
// the file. A `diff --` line is four characters a writer can type, so the
// same shape was reachable from an ORDINARY commit message — `git commit
// -F`, no template, no MERGE_MSG, no `-v`, no `scissors`, no config of any
// kind — and git keeps every byte below a marker it did not write, so the
// classed line landed in the object having been read by nothing.
//
// THE FIX licenses the cut on what only git can be asked: the block below
// the marker is read MINUS the lines of the staged diff (`git diff
// --cached`, rendered the way `-v` renders it), and whatever is left is
// message. Under `-v` that is git's two comment lines; from a writer it is
// every line the index does not carry, class included.
//
// THESE PINS SHIPPED GREEN ASSERTING THE HOLE — the shop's idiom for a live
// defect a QA seat may not fix — and were inverted when the fix landed:
// each now requires the REFUSAL, by the MESSAGE arm, with the line in
// refusals.log. EACH ARM KEEPS ITS CONTROLS, asserted before the verdict:
//   - the wall is AWAKE: the same class typed with -m in this repo at this
//     moment is refused, so a refusal below is this arm's and not some other
//     wall's — and the verdict checks that the MESSAGE arm is what spoke; and
//   - git KEEPS the bytes: the same message committed with the hook bypassed
//     lands with the classed line still in the object, so what the arm
//     refused is content that really would have been in the tree's history.
//
// ARM 3 is new with the fix and is the licence's own edge: the staged diff
// pasted EXACTLY below a forged marker LANDS (nothing below the cut that the
// index does not already hold), and the same paste plus ONE classed line is
// refused. The landing half is the control that says the licence is the
// diff and not the marker.

import (
	"path/filepath"
	"strings"
	"testing"
)

// qaForgedCutMessage is the writer's message: a subject, git's marker line at
// column one, the four characters that satisfied the old guard, and the class.
func qaForgedCutMessage() string {
	return "wire it\n" +
		"# ------------------------ >8 ------------------------\n" +
		"diff --git a/x b/x\n" +
		qaCeilingHit + "\n"
}

// qaForgedCutRefused is the shared verdict: the commit was refused, by the
// MESSAGE arm, and refusals.log carries the message line.
func qaForgedCutRefused(t *testing.T, w *visWall, out string, err error, how string) {
	t.Helper()
	if err == nil {
		t.Fatalf("LANDED: %s carried a class below a forged cut line and the arm did not read it — "+
			"the cut is licensed on something a writer can type again (ranger-base-d94zl):\n%s", how, out)
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: %s must be refused by the MESSAGE arm if it is refused at all:\n%s", how, out)
	}
	if !strings.Contains(w.log(t), "data ceiling scan [prepare-commit-msg hook] (stamp: "+VisibilityPrivate+", commit message)") {
		t.Errorf("refusals.log must carry the message line for %s:\n%s", how, w.log(t))
	}
}

// qaGitKeepsTheBytes is the second control: the same commit with the hook
// bypassed lands with the classed line in the object. Bypassing the hook is
// the only way to land the commit the wall is about to refuse (visWall.plant's
// reason), and it leaves the premise commit as HEAD, which the probe then
// commits on top of.
func qaGitKeepsTheBytes(t *testing.T, w *visWall, env []string, args ...string) {
	t.Helper()
	w.stage(t, w.priv, "internal/posse/premise.go", "package posse\n")
	all := append([]string{"-c", "core.hooksPath=/dev/null", "commit"}, args...)
	if o, e := w.git(w.priv, env, append(all, "--", "internal/posse/premise.go")...); e != nil {
		t.Fatalf("planting the premise commit: %v %s", e, o)
	}
	if landed, e := w.git(w.priv, nil, "log", "-1", "--format=%B"); e != nil || !strings.Contains(landed, qaCeilingHit) {
		t.Fatalf("premise: git must KEEP the classed line below a marker it did not write (%v):\n%s", e, landed)
	}
}

// ARM 1, the residual as messageArm's WHY block once worded it — a
// commit.template body — plus the one line the old guard licensed the cut
// on. This is pin 3 of verbosescissors_qa_test.go's fixture with
// `diff --git a/x b/x` added, and that pin passed while this one landed: the
// difference between a guard that held and one that did not was those four
// characters.
func TestQAMessageArmCutIsNotForgedFromACommitTemplate(t *testing.T) {
	w := qaCeilingWall(t, "")
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR="+qaVerboseEditor(t, "wire it"))

	qaVerboseWallIsAwake(t, w, env, "internal/posse/awake.go")

	tpl := filepath.Join(t.TempDir(), "template")
	write(t, tpl, "from the template\n"+
		"# ------------------------ >8 ------------------------\n"+
		"diff --git a/x b/x\n"+
		qaCeilingHit+"\n")
	if out, err := w.git(w.priv, nil, "config", "commit.template", tpl); err != nil {
		t.Fatalf("git config commit.template: %v %s", err, out)
	}
	qaGitKeepsTheBytes(t, w, env)

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	qaForgedCutRefused(t, w, out, err, "a commit.template body")
}

// ARM 2, and the one that said how wide this was: NO template, NO MERGE_MSG,
// no editor, no `-v`, no `commit.cleanup` — the writer types the marker and
// a `diff --` line into an ordinary `-F` message. This is the crew's own
// commit form, which is the population the ceiling exists to police.
func TestQAMessageArmCutIsNotForgedFromAPlainTypedMessage(t *testing.T) {
	w := qaCeilingWall(t, "")

	qaVerboseWallIsAwake(t, w, w.persona, "internal/posse/awake.go")

	msg := filepath.Join(t.TempDir(), "msg")
	write(t, msg, qaForgedCutMessage())
	qaGitKeepsTheBytes(t, w, w.persona, "-F", msg)

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, w.persona, "commit", "-F", msg, "--", "internal/posse/probe.go")
	qaForgedCutRefused(t, w, out, err, "an ordinary -F message")
}

// ARM 3, the licence's edge. The writer pastes the staged diff itself —
// byte for byte, as `-v` would have written it — below a forged marker in
// an `-F` message. CONTROL: that lands, and the object carries the diff (git
// keeps the bytes; the arm took off the scan only what the index already
// holds). VERDICT: the same paste with one classed line after it is refused,
// because that line is not a line of the staged diff and so is message.
func TestQAMessageArmCutTakesOnlyTheStagedDiffOffTheScan(t *testing.T) {
	w := qaCeilingWall(t, "")

	qaVerboseWallIsAwake(t, w, w.persona, "internal/posse/awake.go")
	// A HEAD to diff against: the reference below is taken the way the hook
	// takes it, against HEAD, and this repo has no commit yet.
	w.plant(t, w.priv, "internal/posse/seed.go", "package posse\n")

	const marker = "wire it\n# ------------------------ >8 ------------------------\n"
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	ref, err := w.git(w.priv, nil, "diff", "--cached", "--no-color", "--no-ext-diff", "--no-relative", "HEAD")
	if err != nil || !strings.Contains(ref, "diff --git a/internal/posse/probe.go b/internal/posse/probe.go") {
		t.Fatalf("fixture premise: the staged diff must be the probe's (%v):\n%s", err, ref)
	}
	exact := filepath.Join(t.TempDir(), "exact")
	write(t, exact, marker+ref)
	out, err := w.git(w.priv, w.persona, "commit", "-F", exact, "--", "internal/posse/probe.go")
	if err != nil {
		t.Fatalf("control: the staged diff pasted exactly below a forged marker must LAND — nothing below "+
			"the cut is outside the index — or the licence is not the diff: %v\n%s", err, out)
	}
	if landed, e := w.git(w.priv, nil, "log", "-1", "--format=%B"); e != nil || !strings.Contains(landed, "+++ b/internal/posse/probe.go") {
		t.Fatalf("control premise: git must KEEP the pasted diff in the object (%v):\n%s", e, landed)
	}

	w.stage(t, w.priv, "internal/posse/probe2.go", "package posse\n")
	ref, err = w.git(w.priv, nil, "diff", "--cached", "--no-color", "--no-ext-diff", "--no-relative", "HEAD")
	if err != nil || !strings.Contains(ref, "probe2.go") || strings.Contains(ref, qaCeilingHit) {
		t.Fatalf("fixture premise: the second staged diff must be probe2's and carry no class (%v):\n%s", err, ref)
	}
	plus := filepath.Join(t.TempDir(), "plus")
	write(t, plus, marker+ref+qaCeilingHit+"\n")
	out, err = w.git(w.priv, w.persona, "commit", "-F", plus, "--", "internal/posse/probe2.go")
	qaForgedCutRefused(t, w, out, err, "the staged diff pasted below a forged marker, plus one classed line")
}
