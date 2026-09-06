//go:build posse_arm2

package posse

// WHAT THE WALL IS HANDED IS NOT WHAT THE WALL THINKS IT IS HANDED — the
// two findings of ranger-base-8djyy's verify, from opposite ends, plus the
// three pins that hold the fixes' MECHANISM rather than their verdict
// (ranger-base-h3s6q).
//
// The first two arrived here PARKED and red, one per finding, each measured
// through the real rendered hook. Both are unparked and green: the reader
// carries --no-textconv, and the message arm reads through
// `git stripspace --strip-comments` exactly where git's cleanup mode says
// git will strip comments too (ranger-base-6y3z2 re-keyed that from "$2",
// which was a proxy for the mode and broke under commit.cleanup). The rest
// of the file is what stops a later edit from taking either fix back out
// quietly.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PARKED (ranger-base-8djyy finding 1, bundled as ranger-base-h3s6q). The five
// `git diff --cached` readers pin their shape on `diffReaderShape`
// (gates.go), five flags chosen so "the readers cannot drift on which
// settings they survive": --no-color, --no-ext-diff, --src-prefix,
// --dst-prefix and, since ranger-base-h137b, --text. A SIXTH setting of the
// same class is not pinned: `diff.<driver>.textconv`, reachable from the
// writer's config alone through `core.attributesFile`. git applies textconv
// BEFORE the diff, so the reader is handed the converted text and refuses
// nothing — silently, and with no "0 insertions(+)" tell, because git's own
// commit summary is computed without it.
//
// This shop has already closed this class once, on the other reader:
// ranger-base-r5wpk put --no-textconv (with --no-relative and four more) on
// memoryDiff in memoryland.go for exactly these routes. The wall's reader
// did not get the same list.
//
// FIXTURE PREMISE, asserted first: with no textconv the same commit IS
// refused. Without that, a green probe is a rig that refuses nothing.
//
// The fix is one flag on diffReaderShape. When it lands, delete the t.Skip.
func TestQADiffReadersSurviveATextconvDriver(t *testing.T) {
	w := qaCeilingWall(t, "")

	// CONTROL / fixture premise: plain config, the ceiling refuses.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n\n// from the "+qaCeilingHit+" banner\n")
	out, err := w.git(w.priv, w.persona, "commit", "-m", "ctl", "--", "internal/posse/ctl.go")
	if err == nil {
		t.Fatalf("fixture premise: the ceiling must refuse this commit with no textconv configured:\n%s", out)
	}
	if !strings.Contains(out, "data-ceiling content in a staged file") {
		t.Fatalf("fixture premise: it must be the CONTENT arm that refused, not another wall:\n%s", out)
	}

	// The writer's config alone: an attributes file and a textconv driver.
	// Nothing is added to the repo's tree and no .gitattributes is staged.
	dir := t.TempDir()
	conv := filepath.Join(dir, "redact.sh")
	write(t, conv, "#!/bin/sh\ngrep -v "+qaCeilingWord+" \"$1\"\nexit 0\n")
	if err := os.Chmod(conv, 0o755); err != nil {
		t.Fatal(err)
	}
	attrs := filepath.Join(dir, "attributes")
	write(t, attrs, "* diff=redact\n")
	for _, kv := range [][2]string{{"core.attributesFile", attrs}, {"diff.redact.textconv", conv}} {
		if out, err := w.git(w.priv, nil, "config", kv[0], kv[1]); err != nil {
			t.Fatalf("git config %s: %v %s", kv[0], err, out)
		}
	}

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n\n// from the "+qaCeilingHit+" banner\n")
	out, err = w.git(w.priv, w.persona, "commit", "-m", "probe", "--", "internal/posse/probe.go")
	if err == nil {
		t.Errorf("a textconv driver in the WRITER's config blanked the reader and the ceiling committed clean — "+
			"diffReaderShape needs --no-textconv (ranger-base-r5wpk carries it on memoryDiff):\n%s", out)
	}
}

// PARKED (ranger-base-8djyy finding 2, bundled as ranger-base-h3s6q). The data
// ceiling's THIRD arm (ranger-base-o2v6n) scans every line of the message
// file with no case on "$2", on the stated reasoning that "a full-file scan
// is fail-safe on every path git takes". On the EDITOR path it is not: the
// hook runs BEFORE the editor, so the file it is handed is git's own
// TEMPLATE — the "On branch <name>" line and the '#'-prefixed status block
// listing staged, unstaged and UNTRACKED paths. Every one of those lines is
// scanned, and git strips all of them (`--cleanup=strip` is the editor
// path's default), so the arm refuses over text that can never reach the
// commit object, with a remedy — "rewrite the commit message" — that cannot
// clear it.
//
// It also falsifies the sentence the arm ships with, in gates.go and in ADR
// 0050 D5: "A message typed in the EDITOR is NOT scanned here and cannot be
// ... handed git's template alone". The template is the thing that refuses.
//
// TWO ARMS. Control: the same editor commit with no classed path in the
// template LANDS (D5's stated residual, which pin (j) already holds). Probe:
// one UNTRACKED file whose NAME carries a ceiling class, nothing else
// changed — refused.
//
// WHAT THE FIX IS is the code lane's call, and this pin takes D5 at its
// word: the editor path is the stated exclusion, so it must land. If the
// product decides the refusal should stand instead, this pin becomes "the
// refusal names the PATH and not the message" — the one thing it may not
// stay is a refusal whose remedy is impossible.
func TestQACeilingMessageArmDoesNotRefuseOnGitsOwnTemplate(t *testing.T) {
	w := qaCeilingWall(t, "")
	ed := filepath.Join(t.TempDir(), "editor.sh")
	write(t, ed, "#!/bin/sh\nprintf '%s\\n' 'wire it' > \"$1\"\n")
	if err := os.Chmod(ed, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR="+ed)

	// CONTROL: nothing in the template carries a class — the editor path
	// lands, which is D5's residual and this pin's proof it is not a rig
	// that refuses everything.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: the EDITOR path is D5's stated exclusion and must land: %v\n%s", err, out)
	}

	// PROBE: one UNTRACKED file whose NAME carries the export class. It is
	// never staged and never named in the message the writer types.
	write(t, filepath.Join(w.priv, qaExportStem+"42.csv"), "x\n")
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		return // the defect is gone; nothing else to say.
	}
	// FIXTURE PREMISE for the probe: it must be the MESSAGE arm that spoke,
	// over git's template, and not some other wall reacting to the new file.
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: the probe must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
	}
	t.Errorf("an UNTRACKED path git listed in its own '#' status template refused an editor commit whose typed "+
		"message is clean, with the remedy 'rewrite the commit message' — git strips every one of those lines "+
		"(--cleanup=strip), so no rewrite clears it and the class never reaches the commit object:\n%s", out)
}

// PIN: the message arm follows the WRITER'S comment character, because git
// does (ranger-base-h3s6q finding 2). The fix reads the message file through
// `git stripspace --strip-comments` on every path but -m/-F, and the whole
// reason it is stripspace and not `grep -v '^#'` is core.commentChar: a
// literal '#' in the hook is a second copy of git's own rule, and this is
// the arm where a drifted copy hands the scan a template git is about to
// throw away.
//
// FIXTURE PREMISE, asserted before the verdict: the template git wrote for
// this commit REALLY carried a ceiling class, on a REALLY ';'-prefixed line.
// Without that the probe is a commit with nothing in it to trip on, and it
// would be green against a wall that reads nothing at all. The editor
// PREPENDS its line and leaves the template in place, so .git/COMMIT_EDITMSG
// is still the evidence after the commit lands.
//
// SECOND PREMISE: the same class typed into a -m message in the same repo,
// same moment, is still REFUSED. That is what says the ceiling is alive here
// and that a landing commit is a verdict rather than a dead wall.
//
// MUTATION: replace the stripspace read with `grep -v '^#'` and this pin
// reds (the ';' template reaches the scan and the commit is refused) while
// every '#'-template pin stays green — which is why the comment char is a
// pin of its own and not a line in the fix's comment.
func TestQACeilingMessageArmFollowsTheWritersCommentChar(t *testing.T) {
	w := qaCeilingWall(t, "")
	if out, err := w.git(w.priv, nil, "config", "core.commentChar", ";"); err != nil {
		t.Fatalf("git config core.commentChar: %v %s", err, out)
	}
	ed := filepath.Join(t.TempDir(), "editor.sh")
	// The editor PREPENDS its line and leaves git's template where it is,
	// so COMMIT_EDITMSG is still the evidence after the commit lands.
	write(t, ed, "#!/bin/sh\n"+
		"{ printf '%s\\n' 'wire it'; cat \"$1\"; } > \"$1.new\" && mv \"$1.new\" \"$1\"\n")
	if err := os.Chmod(ed, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR="+ed)

	// One UNTRACKED file whose NAME carries the export class: git lists it
	// in the ';' status block and strips the whole block at cleanup.
	write(t, filepath.Join(w.priv, qaExportStem+"42.csv"), "x\n")
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err != nil {
		t.Fatalf("a ';'-prefixed template is a template: git strips it and the arm must not read it: %v\n%s", err, out)
	}
	msg, rerr := os.ReadFile(filepath.Join(w.priv, ".git", "COMMIT_EDITMSG"))
	if rerr != nil {
		t.Fatalf("reading COMMIT_EDITMSG: %v", rerr)
	}
	if !strings.Contains(string(msg), ";\t"+qaExportStem) {
		t.Fatalf("fixture premise: git's template must have carried the class on a ';' comment line:\n%s", msg)
	}
	landed, err := w.git(w.priv, nil, "log", "-1", "--format=%B")
	if err != nil {
		t.Fatalf("git log: %v %s", err, landed)
	}
	if strings.Contains(landed, qaExportStem) {
		t.Errorf("the class reached the commit object after all — the arm was right to be asked:\n%s", landed)
	}

	// PREMISE, the other way: the wall is awake in this repo at this moment.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.priv, w.persona, "commit", "-m", "cite "+qaExportStem+"42", "--", "internal/posse/ctl.go"); err == nil {
		t.Fatalf("fixture premise: a class typed into a -m message must still be refused:\n%s", out)
	}
}

// PIN: a merge's own "# Conflicts:" list is git's writing too, and it is the
// second live spelling of finding 2 (ranger-base-h3s6q). git puts every
// CONFLICTED PATH into MERGE_MSG as a comment line and strips them all at
// cleanup; before the fix a conflicted path whose NAME carried a class
// refused the merge commit, with the same impossible remedy — rewrite a
// message that does not contain the hit.
//
// The path is planted through the hook (w.plant) because a path arm that
// works is what refuses to ADD it; this pin is about the MESSAGE arm and
// needs the path to already be history.
//
// FIXTURE PREMISE, asserted after the fact: COMMIT_EDITMSG really did carry
// the conflicted path on a '#' line, so the arm really was handed the class.
func TestQACeilingMessageArmDoesNotRefuseOnAMergesConflictList(t *testing.T) {
	w := qaCeilingWall(t, "")
	rel := "assets/" + qaExportStem + "42.csv"
	w.plant(t, w.priv, rel, "base\n")
	for _, step := range [][]string{
		{"checkout", "-q", "-b", "side"},
	} {
		if out, err := w.git(w.priv, nil, step...); err != nil {
			t.Fatalf("git %v: %v %s", step, err, out)
		}
	}
	w.plant(t, w.priv, rel, "side\n")
	if out, err := w.git(w.priv, nil, "checkout", "-q", "main"); err != nil {
		t.Fatalf("git checkout main: %v %s", err, out)
	}
	w.plant(t, w.priv, rel, "main\n")

	if out, err := w.git(w.priv, w.persona, "merge", "side"); err == nil {
		t.Fatalf("fixture premise: the merge must CONFLICT on %s, or there is no conflict list to read:\n%s", rel, out)
	}
	write(t, filepath.Join(w.priv, filepath.FromSlash(rel)), "resolved\n")
	if out, err := w.git(w.priv, nil, "add", "--", rel); err != nil {
		t.Fatalf("git add %s: %v %s", rel, err, out)
	}
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR=true")
	if out, err := w.git(w.priv, env, "merge", "--continue"); err != nil {
		t.Fatalf("git wrote the conflicted path into its own '#' block and strips it — the merge commit must land: %v\n%s", err, out)
	}
	msg, rerr := os.ReadFile(filepath.Join(w.priv, ".git", "COMMIT_EDITMSG"))
	if rerr != nil {
		t.Fatalf("reading COMMIT_EDITMSG: %v", rerr)
	}
	if !strings.Contains(string(msg), "#\t"+rel) {
		t.Fatalf("fixture premise: git's conflict list must have carried the class on a '#' line:\n%s", msg)
	}
}

// PIN, structural: ONE message reader per wall, in TWO forms, keyed on git's
// CLEANUP MODE — and the -m/-F fallback is still the whole file
// (ranger-base-h3s6q, re-keyed by ranger-base-6y3z2). The split is the fix;
// the `cat` half is the hole the fix may not open. A pasted '#'-leading line
// on the -m path commits (git's cleanup there is "whitespace"), so a hook
// that stripped comments on every path would hand the paste this arm exists
// for straight through — the behaviour pins for that are in
// dataceiling_qa_test.go and checkthreemessage_qa_test.go; this one holds
// the shape they rest on.
//
// WHY "$2" IS NO LONGER THE KEY, and why it is still in the block: "$2" says
// whether git will EDIT the message, which picks the cleanup mode only while
// the mode is "default". commit.cleanup=verbatim on the editor path made git
// KEEP its own template — the "On branch" line and the UNTRACKED file list —
// in the commit object while this arm stripped exactly those lines out of
// the scan (ranger-base-6y3z2, measured, git 2.50.1). So the mode is asked
// for by name and "$2" survives as the "default" row's proxy alone; a hook
// that goes back to keying on "$2" first reds here.
func TestQAMessageArmReadsTwoWaysKeyedOnTheCleanupMode(t *testing.T) {
	t.Parallel() // renders and reads strings; no repo, no env (ranger-base-pj87l)
	set := OpsPatternSet{
		Extra:   []OpsPattern{{Class: qaInstanceClass, ERE: qaInstanceERE}},
		Ceiling: []OpsPattern{{Class: qaCeilingClass, ERE: qaCeilingERE}},
	}
	hook := CommitGuardHook(VisibilityPublic, set, IdentityLiteral{Class: "username", Value: "qa-fixture-operator"})
	for _, want := range []struct {
		frag string
		n    int
	}{
		// The mode is READ, from git, once per wall.
		{`posse_clean=$(git config --get commit.cleanup 2>/dev/null)`, 2},
		// The three modes that keep comments read the file WHOLE. All
		// three, because git keeps its own template under every one of
		// them (measured) and dropping any is the same fail-open again.
		// Three fragments rather than one line since ranger-base-b21e0,
		// because the arm gained the remedy's mode capture and the two
		// walls render it at different indents — so each fragment is
		// asserted indent-free, the way the one-liner always was.
		{`verbatim|whitespace|scissors)`, 2},
		{`posse_clean=whole ;;`, 2},
		// The remedy's handle on WHICH of the three is live, captured in
		// the same arm rather than by reading the mode list a second time:
		// a second copy of that list is the drift this file argues against
		// everywhere else (ranger-base-b21e0). It is empty where "$2" is
		// "message", because git appends no template on that path and the
		// unqualified "rewrite the commit message" is doable as written.
		{`if [ "${2:-}" != "message" ]; then posse_kept=$posse_clean; fi`, 2},
		{`posse_kept=''`, 2},
		// "strip" strips on EVERY path, -m/-F included: that is the
		// over-refusal half of ranger-base-6y3z2.
		{`strip) posse_clean=strip ;;`, 2},
		// "$2" survives as the fallback for "default"/unset and nothing
		// else — and there it still reads -m/-F whole.
		{`*) if [ "${2:-}" = "message" ]; then posse_clean=whole; else posse_clean=strip; fi ;;`, 2},
		// The file is read once per wall, ABOVE the branch: the scissors
		// cut is git's on every mode, so both reads take the same bytes
		// (ranger-base-xfgcn).
		{`posse_msg=$(cat "$1" 2>/dev/null)`, 2},
		{`posse_added=$posse_msg`, 2},
		{"posse_added=$(printf '%s\\n' \"$posse_msg\" | git stripspace --strip-comments 2>/dev/null)", 2},
	} {
		if n := strings.Count(hook, want.frag); n != want.n {
			t.Errorf("want %d × %q — one per wall — got %d", want.n, want.frag, n)
		}
	}
	// The mode must not be the FIRST question again: "$2" may only be
	// reached from the `*)` arm above, where the newline after `then` is
	// the tell that a whole branch hangs off it.
	if strings.Contains(hook, "if [ \"${2:-}\" = \"message\" ]; then\n") {
		t.Error(`"$2" is a proxy for the cleanup mode and it breaks the moment commit.cleanup is set ` +
			`(ranger-base-6y3z2): the arm must read the mode first and fall back to "$2" only for "default"`)
	}
	if strings.Contains(hook, "grep -v '^#'") || strings.Contains(hook, `grep -av '^#'`) {
		t.Error("the comment character is core.commentChar, the WRITER's config: the hook must ask git, not carry a literal '#'")
	}
}
