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
// EVERY PIN HERE WAS PARKED (t.Skip) AND RED when it was written — the run
// is on ranger-base-27mqp. They were unparked by ranger-base-vzx2n, which
// fixed the reader; each still carries the control that says the wall is
// awake in that repo at that moment, so a green pin cannot be a rig that
// refuses nothing. MUTATION, run 2026-09-04: disable the `auto` branch of
// the reader in messageArm (gates.go) and all three go red again, each on
// its own assertion.
//
// WHAT THE FIX IS was the code lane's call and these pins did not prescribe
// one. What landed: for `auto` — and only for `auto`, because every explicit
// value stripspace already resolves — the character is read back off the
// template git ALREADY wrote into "$1" (git makes the choice before this
// hook runs), and is handed to stripspace. Where git appended no template it
// strips nothing under `auto`, so the file is read whole; the pin for that
// half is TestQACheckThreeMessageArmUnderAutoWithNoTemplateAppended below.

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

// PIN, the other half of ranger-base-vzx2n's fix: `auto` WITH NO TEMPLATE
// APPENDED. The fix takes the comment character off the template git wrote,
// so it has to say what it does when git wrote none — commit.status=false
// here, and the same file shape reaches the hook on the merge path (git
// hands it MERGE_MSG as merge wrote it) and under `--amend --no-edit`.
//
// The answer git itself gives: under `auto` the character is chosen so that
// it starts no line of the message, so the only lines git ever strips are
// the ones git appended. No template, nothing stripped — so the whole file
// is what will land and the whole file is what must be scanned. A reader
// that guessed a character here would strip a body line git keeps, which is
// the hole this bead closed, pointing the other way.
//
// FIXTURE PREMISE, asserted before the verdict: git really appended no
// comment block for this commit, so the reader really is in the fallback.
//
// CONTROL, asserted after the verdict: the SAME editor commit over a
// template with nothing classed in it LANDS. The fallback reads the file
// whole, and a whole-file read is the shape that over-refuses — this is what
// says it did not simply start refusing every editor commit in the repo.
//
// MUTATION: disable the reader's `auto` branch and this pin reds — the
// stripspace that answers '#' takes the identity literal's line out of the
// scan and the commit lands with it in a PUBLIC repo.
func TestQACheckThreeMessageArmUnderAutoWithNoTemplateAppended(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	env := qaAutoCommentCharRepo(t, w, w.pub, "# followed "+username+"'s note from tuesday\n")
	if out, err := w.git(w.pub, nil, "config", "commit.status", "false"); err != nil {
		t.Fatalf("git config commit.status: %v %s", err, out)
	}

	w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n\n// nothing identifying in here at all.\n")
	out, err := w.git(w.pub, env, "commit", "--", "internal/posse/probe.go")

	// PREMISE: no comment block was appended, so the fallback is what ran.
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
			t.Errorf("an operator identity literal reached a PUBLIC repo's commit object on a '#' line git KEEPS — "+
				"there was no template to strip under core.commentChar=auto and the arm stripped anyway:\n%s", landed)
		}
		return
	}
	if !strings.Contains(out, "identity literal in the commit MESSAGE") {
		t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke:\n%s", out)
	}

	// CONTROL: the same editor commit over a clean template must land.
	clean := qaAutoCommentCharRepo(t, w, w.pub, "# a heading nobody can be identified by\n")
	w.stage(t, w.pub, "internal/posse/ctl.go", "package posse\n")
	if o, e := w.git(w.pub, clean, "commit", "--", "internal/posse/ctl.go"); e != nil {
		t.Fatalf("fixture premise: an editor commit with nothing classed in it must land: %v\n%s", e, o)
	}
}

// PIN, ranger-base-vl9g8, found verifying vzx2n's own fix. The reader takes
// the comment character off the block git already wrote, and the first cut
// of it took the LAST LINE of "$1" and used that line's first character.
// git's block is last only when nothing follows it, and under `commit -v` /
// commit.verbose=true something does: git appends the scissors marker and
// then the staged DIFF, so "$1" ends in `+line`. No character is detected,
// the reader falls back to reading the file WHOLE, and h3s6q's over-refusal
// is back — git's status block, untracked filenames and all, handed to the
// scan on an editor commit whose typed message is clean, with the remedy
// "rewrite the commit message" that clears none of it. `-v` is one flag, or
// one config line the writer owns.
//
// This is TestQACeilingMessageArmUnderAnAutoCommentCharStillReadsGitsTemplate
// with commit.verbose turned on, and it is a separate pin because that one
// stays green across the whole change: without `-v` the block IS last, so
// the old selector and the new one agree, and only this shape tells them
// apart.
//
// WHAT MAKES IT PASS: the character is taken from the last line of "$1" that
// is a bare comment character alone. A unified diff cannot contain one —
// every line of it carries ' ', '+', '-', '@', '\' or a header word in
// column one — so the last such line is git's block under `-v` and without
// it alike.
//
// FIXTURE PREMISE, asserted before the verdict: git really wrote a ';'
// template (so the two readers had something to disagree about) AND really
// appended the diff after it (so the file does not end in git's block).
//
// CONTROL, asserted first: the same `-v` editor commit with nothing classed
// anywhere lands, so a green verdict is not a wall that refuses nothing.
//
// RUN BEFORE THE FIX, RED: "data-ceiling content in the commit MESSAGE ...
// export-name: 1 hit(s)". MUTATION: put the old `sed -n '$p' | cut -c1`
// selector back and this reds while the other four stay green.
func TestQACeilingMessageArmUnderAnAutoCommentCharWithAVerboseCommit(t *testing.T) {
	w := qaCeilingWall(t, "")
	env := qaAutoCommentCharRepo(t, w, w.priv, "# notes\n")
	if out, err := w.git(w.priv, nil, "config", "commit.verbose", "true"); err != nil {
		t.Fatalf("git config commit.verbose: %v %s", err, out)
	}

	// CONTROL: nothing classed anywhere — this `-v` editor commit must land.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: a clean -v editor commit must land: %v\n%s", err, out)
	}
	msg := qaAutoSwitched(t, w, w.priv)
	if !strings.Contains(msg, "\ndiff --git ") {
		t.Fatalf("fixture premise: commit.verbose=true must have appended the diff after git's block, "+
			"or \"$1\" still ends in that block and this pin is the one next door:\n%s", msg)
	}

	// PROBE: ONE untracked file, never staged, never typed. git lists its
	// NAME in the status block it is about to strip.
	write(t, filepath.Join(w.priv, qaExportStem+"42.csv"), "x\n")
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		return // the defect is gone; nothing else to say.
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: the probe must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
	}
	t.Errorf("under core.commentChar=auto a `-v` commit's file ends in the diff, not in git's block, so the "+
		"arm detected no comment character, read the whole file and refused an editor commit over an "+
		"UNTRACKED path git listed in a template it strips — the remedy 'rewrite the commit message' "+
		"clears nothing:\n%s", out)
}
