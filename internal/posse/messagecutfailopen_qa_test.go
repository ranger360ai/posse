package posse

// THE CUT CAN BE FORGED BY THE WRITER, AND THE CLASS BELOW IT LANDS
// (found verifying ranger-base-xfgcn's close; filed as its own bead, named
// in the findings bundle for ranger-base-xtgvp).
//
// ranger-base-xfgcn stopped the message arm at git's cut line, which is the
// right fix for the over-refusal it was filed about. The cut is guarded —
// taken only where `commit.cleanup` is `scissors`, or a `diff --` line
// stands below the marker, "which only a verbose commit appends" — and
// messageArm's WHY block discloses the residual that guard leaves:
//
//   "Fail-OPEN, narrow and deliberate: a file git did not write — a
//    commit.template body, MERGE_MSG — carrying BOTH the marker line at
//    column one AND a `diff --` line below it, under neither verbose nor
//    scissors, is cut where git would not cut it."
//
// It is narrower in that sentence than it is in the hook. Nothing in the
// arm asks who wrote the file, and a `diff --` line is four characters a
// writer can type, so the same shape is reachable from an ORDINARY commit
// message — `git commit -F`, no template, no MERGE_MSG, no `-v`, no
// `scissors`, no config of any kind. git keeps every byte below a marker it
// did not write, so the classed line lands in the object having been read by
// nothing.
//
// MEASURED BOTH DIRECTIONS at 4710e88. With messageArm's cut disabled — the
// read exactly as it was before ranger-base-xfgcn — both arms below are
// REFUSED; with the cut in place both LAND. So this arrived with that close
// rather than surviving it.
//
// THESE PINS SHIP GREEN AND ASSERT THE HOLE. That is the shop's idiom for a
// live defect a QA seat may not fix: each one requires the commit to LAND
// and says in its failure message what to do when it stops landing, which is
// to invert it. When the code lane closes this, these two go red, and the
// right edit is to turn each `err != nil` into the refusal assertion and
// delete this paragraph.
//
// EACH ARM CARRIES ITS CONTROLS, asserted before the verdict:
//   - the wall is AWAKE: the same class typed with -m in this repo at this
//     moment is refused, so a landing commit here is the cut and not a
//     sleeping gate; and
//   - git KEEPS the bytes: the same message committed with the hook bypassed
//     lands with the classed line still in the object, so what escaped the
//     scan is content that really is in the tree's history.

import (
	"path/filepath"
	"strings"
	"testing"
)

// qaForgedCutMessage is the writer's message: a subject, git's marker line at
// column one, the four characters that satisfy the guard, and the class.
func qaForgedCutMessage() string {
	return "wire it\n" +
		"# ------------------------ >8 ------------------------\n" +
		"diff --git a/x b/x\n" +
		qaCeilingHit + "\n"
}

// qaForgedCutLanded is the shared verdict: the commit was accepted and the
// classed line is in the object. Both halves are asserted, because a commit
// that landed WITHOUT the class would be git truncating rather than the wall
// missing, and only the second is this defect.
func qaForgedCutLanded(t *testing.T, w *visWall, out string, err error, how string) {
	t.Helper()
	if err != nil {
		t.Fatalf("REFUSED, so the hole is closed — invert this pin: it must now assert the refusal "+
			"(%s carried the class below a forged cut line and the arm read it):\n%s", how, out)
	}
	landed, gerr := w.git(w.priv, nil, "log", "-1", "--format=%B")
	if gerr != nil {
		t.Fatalf("git log: %v %s", gerr, landed)
	}
	if !strings.Contains(landed, qaCeilingHit) {
		t.Fatalf("premise: the commit landed but git did not keep the classed line, so nothing "+
			"escaped the scan here and this pin measured the wrong thing:\n%s", landed)
	}
}

// ARM 1, the residual as messageArm's WHY block words it — a commit.template
// body — plus the one line the guard licenses the cut on. This is pin 3 of
// verbosescissors_qa_test.go's fixture with `diff --git a/x b/x` added, and
// that pin passes: the difference between a guard that holds and a guard
// that does not is those four characters.
func TestQAMessageArmCutIsForgedFromACommitTemplate(t *testing.T) {
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

	// PREMISE: with no -v and no cleanup=scissors git keeps every byte below
	// a marker it did not write. Measured with the hook bypassed, which is
	// the only way to land a commit the wall is about to refuse.
	w.stage(t, w.priv, "internal/posse/premise.go", "package posse\n")
	if o, e := w.git(w.priv, env, "-c", "core.hooksPath=/dev/null", "commit", "--", "internal/posse/premise.go"); e != nil {
		t.Fatalf("planting the premise commit: %v %s", e, o)
	}
	if landed, e := w.git(w.priv, nil, "log", "-1", "--format=%B"); e != nil || !strings.Contains(landed, qaCeilingHit) {
		t.Fatalf("premise: git must KEEP the classed line here (%v):\n%s", e, landed)
	}

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	qaForgedCutLanded(t, w, out, err, "a commit.template body")
}

// ARM 2, and the one that says how wide this is: NO template, NO MERGE_MSG,
// no editor, no `-v`, no `commit.cleanup` — the writer types the marker and
// a `diff --` line into an ordinary `-F` message. This is the crew's own
// commit form, which is the population the ceiling exists to police.
func TestQAMessageArmCutIsForgedFromAPlainTypedMessage(t *testing.T) {
	w := qaCeilingWall(t, "")

	qaVerboseWallIsAwake(t, w, w.persona, "internal/posse/awake.go")

	msg := filepath.Join(t.TempDir(), "msg")
	write(t, msg, qaForgedCutMessage())

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, w.persona, "commit", "-F", msg, "--", "internal/posse/probe.go")
	qaForgedCutLanded(t, w, out, err, "an ordinary -F message")
}
