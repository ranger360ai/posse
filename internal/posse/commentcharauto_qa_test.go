package posse

// `core.commentChar=auto` IS A THIRD ANSWER, AND THE MESSAGE ARM ONLY HAS
// TWO (ranger-base-27mqp, the verify of ranger-base-h3s6q).
//
// h3s6q's finding 2 fixed a message arm that read the whole file on every
// path: on the editor path "$1" is git's own template, git strips it
// (`--cleanup=strip`), and the arm refused over text that can never reach
// the commit object with a remedy no rewrite clears. The fix splits the read
// on "$2" — `cat` for -m/-F, where git's cleanup is "whitespace" and KEEPS a
// '#'-leading line, and `git stripspace --strip-comments` everywhere else —
// and its stated reason for stripspace over `grep -v '^#'` is that
//
//	"The comment character is the writer's config (core.commentChar,
//	 core.commentString since git 2.45), so a literal '#' here is a second
//	 copy of git's rule and a copy is a thing that drifts. `git stripspace
//	 --strip-comments` reads the same config git does"
//
// MEASURED, git 2.50.1 (Apple Git-155): it does not, for one value of that
// config. `core.commentChar=auto` tells git to PICK a character that does
// not start a line of the message it is about to write. `git commit` runs
// that choice (commit.c adjust_comment_line_char); `git stripspace` does
// not, and answers with a plain '#'. So the moment the message body carries
// a '#'-leading line — a pasted markdown heading in a commit.template, a
// MERGE_MSG, HEAD's message under --amend — the two disagree in BOTH
// directions at once:
//
//	git's template comments   ';'   git STRIPS them   stripspace KEEPS them
//	the writer's '#' line     '#'   git KEEPS it      stripspace STRIPS it
//
// which is finding 2's over-refusal AND a hole the wall's whole reason for
// reading a message at all, in one config line the WRITER owns. Under
// `--cleanup=strip` the '#' line lands in the commit object and replicates
// with the branch; the arm never saw it.
//
// EVERY PIN HERE IS PARKED (t.Skip) and every one of them was run unparked
// and RED first — the run is on ranger-base-27mqp. Each carries the control
// that says the wall is awake in that repo at that moment, so a green pin
// cannot be a rig that refuses nothing. Unpark by deleting the t.Skip line.
//
// WHAT THE FIX IS is the code lane's call and these pins do not prescribe
// one — asking git for the character it will actually use (`git config
// --get core.commentChar` resolved the way commit.c resolves it) and
// stripping with that, or reading the message the way git will KEEP it by
// another route, are both defensible and they are not the same change.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaAutoCommentCharRepo arms a repo the way a writer does: `auto`, plus a
// commit.template whose body starts with '#'. Nothing here is posse's.
// Returns the editor env — the editor PREPENDS its line and leaves the
// template in place, so .git/COMMIT_EDITMSG is still the evidence after.
func qaAutoCommentCharRepo(t *testing.T, w *visWall, repo, templateBody string) []string {
	t.Helper()
	tmpl := filepath.Join(t.TempDir(), "template")
	write(t, tmpl, templateBody)
	for _, kv := range [][2]string{{"core.commentChar", "auto"}, {"commit.template", tmpl}} {
		if out, err := w.git(repo, nil, "config", kv[0], kv[1]); err != nil {
			t.Fatalf("git config %s: %v %s", kv[0], err, out)
		}
	}
	ed := filepath.Join(t.TempDir(), "editor.sh")
	write(t, ed, "#!/bin/sh\n"+
		"{ printf '%s\\n' 'wire it'; cat \"$1\"; } > \"$1.new\" && mv \"$1.new\" \"$1\"\n")
	if err := os.Chmod(ed, 0o755); err != nil {
		t.Fatal(err)
	}
	return append(append([]string(nil), w.persona...), "GIT_EDITOR="+ed)
}

// qaAutoSwitched is the FIXTURE PREMISE every pin below rests on: git really
// did decline '#' for this commit and write its template with ';'. Without
// it the pin is a commit with nothing in it to disagree about.
func qaAutoSwitched(t *testing.T, w *visWall, repo string) string {
	t.Helper()
	msg, err := os.ReadFile(filepath.Join(repo, ".git", "COMMIT_EDITMSG"))
	if err != nil {
		t.Fatalf("reading COMMIT_EDITMSG: %v", err)
	}
	if !strings.Contains(string(msg), "\n; ") {
		t.Fatalf("fixture premise: core.commentChar=auto must have made git write a ';' template, or the two readers never disagreed:\n%s", msg)
	}
	return string(msg)
}

// PARKED. THE HOLE. A ceiling class on a '#'-leading line of the message git
// is about to KEEP is stripped by the arm's reader and lands in the commit
// object. ADR 0050 D2's whole subject is content that may not exist in a
// local file at all; this is it, in a commit, unrefused and replicating.
//
// The class arrives through commit.template here because that is the
// shortest reachable spelling — MERGE_MSG and `--amend`'s reuse of HEAD's
// message are the same file with the same '#' line and the same two
// disagreeing readers.
//
// CONTROL, asserted after the verdict: the identical class typed into a -m
// message in the same repo at the same moment is still REFUSED. That is
// what says the ceiling is alive here and a landing commit is a verdict.
//
// RUN UNPARKED 2026-09-04, RED: the commit landed
// `wire it\n# cite QUOKKA RESTRICTED here` and refusals.log carried nothing.
func TestQACeilingMessageArmUnderAnAutoCommentChar(t *testing.T) {
	t.Skip("PARKED (ranger-base-27mqp): red today — the fix for h3s6q finding 2 reads the message with a stripspace that does not resolve core.commentChar=auto")
	w := qaCeilingWall(t, "")
	env := qaAutoCommentCharRepo(t, w, w.priv, "# cite "+qaCeilingHit+" here\n")

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		qaAutoSwitched(t, w, w.priv)
		landed, lerr := w.git(w.priv, nil, "log", "-1", "--format=%B")
		if lerr != nil {
			t.Fatalf("git log: %v %s", lerr, landed)
		}
		if strings.Contains(landed, qaCeilingHit) {
			t.Errorf("a ceiling class on a '#' line of the message git KEEPS reached the commit object — "+
				"the arm read the file through `git stripspace --strip-comments`, which answers '#' for "+
				"core.commentChar=auto while git had already picked ';' for this message:\n%s", landed)
		}
	} else if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke:\n%s", out)
	}

	// CONTROL: the wall is awake in this repo at this moment.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if o, e := w.git(w.priv, w.persona, "commit", "-m", "cite "+qaCeilingHit, "--", "internal/posse/ctl.go"); e == nil {
		t.Fatalf("fixture premise: the same class typed with -m must still be refused:\n%s", o)
	}
}

// PARKED. THE SAME HOLE IN THE OTHER WALL, and this is the one that is live
// on a box like this one: check 3's message arm (ADR 0024 D2) renders in a
// PUBLIC repo with the classes this box derives — username, git email, the
// instance repo's path — and it shares the renderer h3s6q split, so it reads
// the message the same way and misses the same '#' line. An operator
// identity literal on that line lands in a public repo's commit object.
//
// CONTROL: the same literal typed with -m in the same repo is refused (pin
// (m), checkthreemessage_qa_test.go, is the standing version of that).
//
// RUN UNPARKED 2026-09-04, RED.
func TestQACheckThreeMessageArmUnderAnAutoCommentChar(t *testing.T) {
	t.Skip("PARKED (ranger-base-27mqp): red today — same reader, same auto-comment-char blind spot, in the public repo's identity arm")
	w := newVisWall(t)
	username := w.literal(t, "username")
	env := qaAutoCommentCharRepo(t, w, w.pub, "# followed "+username+"'s note from tuesday\n")

	w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n\n// nothing identifying in here at all.\n")
	out, err := w.git(w.pub, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		qaAutoSwitched(t, w, w.pub)
		landed, lerr := w.git(w.pub, nil, "log", "-1", "--format=%B")
		if lerr != nil {
			t.Fatalf("git log: %v %s", lerr, landed)
		}
		if strings.Contains(landed, username) {
			t.Errorf("an operator identity literal on a '#' line of the message git KEEPS reached a PUBLIC "+
				"repo's commit object — check 3's message arm reads through the same stripspace:\n%s", landed)
		}
	} else if !strings.Contains(out, "identity literal in the commit MESSAGE") {
		t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke:\n%s", out)
	}

	// CONTROL: the wall is awake in this repo at this moment.
	w.stage(t, w.pub, "internal/posse/ctl.go", "package posse\n")
	if o, e := w.git(w.pub, w.persona, "commit", "-m", "followed "+username+"'s note", "--", "internal/posse/ctl.go"); e == nil {
		t.Fatalf("fixture premise: the same literal typed with -m must still be refused:\n%s", o)
	}
}

// PARKED. THE OTHER DIRECTION, and it is h3s6q finding 2 word for word: with
// git's template written in ';' and the arm's reader stripping '#', the
// whole status block — staged, unstaged and UNTRACKED paths, the "On branch"
// line, a merge's "# Conflicts:" list — is handed to the scan again. One
// untracked file whose NAME carries a class refuses every editor commit in
// the repo, with the remedy "rewrite the commit message", which cannot clear
// a hit that is not in the message.
//
// TestQACeilingMessageArmFollowsTheWritersCommentChar is the green pin next
// door and it is not this: it sets core.commentChar=';' EXPLICITLY, which
// stripspace does resolve. `auto` is the value where the two readers part.
//
// CONTROL: the same editor commit with no classed path anywhere lands.
//
// RUN UNPARKED 2026-09-04, RED: "data-ceiling content in the commit MESSAGE
// ... export-name: 1 hit(s)".
func TestQACeilingMessageArmUnderAnAutoCommentCharStillReadsGitsTemplate(t *testing.T) {
	t.Skip("PARKED (ranger-base-27mqp): red today — finding 2's over-refusal returns whenever git's template is not '#'")
	w := qaCeilingWall(t, "")
	env := qaAutoCommentCharRepo(t, w, w.priv, "# notes\n")

	// CONTROL: nothing classed anywhere — the editor path is D5's stated
	// residual and must land.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: a clean editor commit must land: %v\n%s", err, out)
	}
	qaAutoSwitched(t, w, w.priv)

	// PROBE: ONE untracked file, never staged, never typed.
	write(t, filepath.Join(w.priv, qaExportStem+"42.csv"), "x\n")
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		return // the defect is gone; nothing else to say.
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: the probe must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
	}
	t.Errorf("git wrote its status block with ';' and the arm stripped '#', so an UNTRACKED path git listed "+
		"in its own template refused an editor commit whose typed message is clean — the remedy 'rewrite the "+
		"commit message' clears nothing:\n%s", out)
}
