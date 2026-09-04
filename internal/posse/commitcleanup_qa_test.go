package posse

// THE MESSAGE ARM KEYED ON THE SOURCE ARGUMENT, AND THE CLEANUP MODE IS WHAT
// DECIDES (ranger-base-6y3z2, found fixing ranger-base-vzx2n in the same
// reader; measured at the shell, git 2.50.1 (Apple Git-155), not inferred).
//
// ranger-base-h3s6q split the message read in two — the whole file when "$2"
// is "message" (-m/-F, where git's cleanup is "whitespace" and a '#'-leading
// line is KEPT) and `git stripspace --strip-comments` otherwise (the editor
// path, where the cleanup is "strip" and git's own template is thrown away).
// "$2" says whether git will EDIT the message. That picks the cleanup mode
// only while the mode is "default", and it stops being the mode the moment
// commit.cleanup is set — which is one line in ~/.gitconfig, no intent
// required.
//
// DIRECTION 1, FAIL-OPEN, and it is the one that matters. Under
// commit.cleanup=verbatim git keeps its OWN template in the commit object:
// the Please-enter paragraph, "On branch <name>", the staged list and the
// UNTRACKED file list. The arm read that file through stripspace and
// stripped exactly those lines out of the scan, so a class or an operator
// identity literal carried by a branch name, an untracked path or a merge's
// conflict list reached a PUBLIC repo's commit object with no wall speaking.
//
// DIRECTION 2, over-refusal, the cheap one. Under commit.cleanup=strip git
// strips a '#'-leading line on the -m/-F path too — a line the arm read
// whole and refused over, with a remedy the writer really can clear.
//
// WHAT THE HOOK CAN SEE OF THE MODE, measured in a hook that printed both:
// `git config --get commit.cleanup` answers for ~/.gitconfig, the repo
// config AND `git -c commit.cleanup=... commit` (git exports that one in
// GIT_CONFIG_PARAMETERS). It does NOT answer for `git commit --cleanup=...`:
// rc 1, empty, and nothing in the environment carries it. The flag form is
// this fix's stated residual and no pin here claims otherwise — the config
// form is the one that needs no intent.
//
// EVERY PIN HERE WAS RUN AGAINST THE PRE-FIX ARM AND WAS RED, through
// `go test -overlay` against gates.go at 36efc82; the runs are recorded at
// each pin. Each carries the control that says the wall is awake in that
// repo at that moment, so a green pin cannot be a rig that refuses nothing.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaVerbatimRepo arms a repo the way a writer with one ~/.gitconfig line
// does, and returns the editor env. The editor PREPENDS its line and leaves
// git's template in place, which is what "verbatim" is about: under that
// mode the template is not a template, it is message.
func qaVerbatimRepo(t *testing.T, w *visWall, repo, mode string) []string {
	t.Helper()
	if out, err := w.git(repo, nil, "config", "commit.cleanup", mode); err != nil {
		t.Fatalf("git config commit.cleanup %s: %v %s", mode, err, out)
	}
	ed := filepath.Join(t.TempDir(), "editor.sh")
	write(t, ed, "#!/bin/sh\n"+
		"{ printf '%s\\n' 'wire it'; cat \"$1\"; } > \"$1.new\" && mv \"$1.new\" \"$1\"\n")
	if err := os.Chmod(ed, 0o755); err != nil {
		t.Fatal(err)
	}
	return append(append([]string(nil), w.persona...), "GIT_EDITOR="+ed)
}

// qaKeptItsTemplate is the FIXTURE PREMISE both direction-1 pins rest on:
// git really did keep its own template in the object under this mode. Without
// it the pin is a commit with nothing in it that could have leaked.
func qaKeptItsTemplate(t *testing.T, w *visWall, repo string) {
	t.Helper()
	landed, err := w.git(repo, nil, "log", "-1", "--format=%B")
	if err != nil {
		t.Fatalf("git log: %v %s", err, landed)
	}
	if !strings.Contains(landed, "# On branch ") {
		t.Fatalf("fixture premise: under this cleanup mode git must KEEP its own template in the commit "+
			"object, or there is nothing here for the arm to have missed:\n%s", landed)
	}
}

// THE HOLE, in the wall that is live on a box like this one: check 3's
// message arm (ADR 0024 D2, ADR 0048 D2) renders in a PUBLIC repo with the
// classes this box derives, and under commit.cleanup=verbatim git writes an
// UNTRACKED file's NAME into the message it is about to keep. The arm read
// that file through stripspace, which took the whole '#' block out of the
// scan, so the name reached a public repo's commit object unrefused.
//
// The untracked path is the shortest reachable spelling. A branch name and a
// merge's "# Conflicts:" list are the same template, the same reader and the
// same verdict; the ceiling pin below takes the branch name.
//
// AFTER THE FIX the commit is REFUSED, and that is the honest verdict rather
// than an over-refusal: under verbatim those bytes really do land. What it
// costs is stated in gates.go — the writer is refused before the editor
// opens, so the remedy line's "rewrite the commit message" names a rewrite
// that has not happened yet, and the layer that can tell those apart is the
// commit-msg hook that file already names as missing.
//
// CONTROL 1, before the probe: a clean editor commit under the same config
// LANDS and carries git's template — the wall is not refusing everything and
// verbatim really keeps the block.
// CONTROL 2, after: the same literal typed with -m is refused.
//
// RUN AGAINST THE PRE-FIX ARM 2026-09-04, RED: the commit landed with
// "# Untracked files:" and the username-bearing path in the object.
func TestQACheckThreeMessageArmUnderCommitCleanupVerbatim(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	env := qaVerbatimRepo(t, w, w.pub, "verbatim")

	// CONTROL 1 / fixture premise.
	w.stage(t, w.pub, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.pub, env, "commit", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: a clean editor commit must land: %v\n%s", err, out)
	}
	qaKeptItsTemplate(t, w, w.pub)

	// PROBE: ONE untracked file, never staged, never typed — git lists it in
	// the template and verbatim keeps the list.
	write(t, filepath.Join(w.pub, username+"-notes.txt"), "x\n")
	w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.pub, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		landed, lerr := w.git(w.pub, nil, "log", "-1", "--format=%B")
		if lerr != nil {
			t.Fatalf("git log: %v %s", lerr, landed)
		}
		if strings.Contains(landed, username) {
			t.Errorf("an operator identity literal git wrote into the message ITSELF — the untracked-file list — "+
				"reached a PUBLIC repo's commit object: commit.cleanup=verbatim keeps that block and the arm "+
				"read the file through `git stripspace --strip-comments`, which takes it out of the scan:\n%s", landed)
		}
	} else {
		if !strings.Contains(out, "identity literal in the commit MESSAGE") {
			t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke:\n%s", out)
		}
		w.unstage(t, w.pub, "internal/posse/probe.go")
	}

	// CONTROL 2: the wall is awake in this repo at this moment.
	w.stage(t, w.pub, "internal/posse/ctl2.go", "package posse\n")
	if o, e := w.git(w.pub, w.persona, "commit", "-m", "followed "+username+"'s note", "--", "internal/posse/ctl2.go"); e == nil {
		t.Fatalf("fixture premise: the same literal typed with -m must still be refused:\n%s", o)
	}
}

// THE SAME HOLE IN THE OTHER WALL, through the other thing git writes into
// its template: the BRANCH NAME. ADR 0050 D2's subject is content that may
// not exist in a durable replicated copy at all, and "On branch <name>" is a
// line git keeps verbatim under this mode. The ceiling runs under EVERY
// visibility stamp, so the private repo is the right place to measure it.
//
// CONTROL 1: the clean editor commit on the unclassed branch lands and
// carries the template. CONTROL 2: the same class typed with -m is refused.
//
// RUN AGAINST THE PRE-FIX ARM 2026-09-04, RED: the commit landed with
// "# On branch quokka-export-7" in the object.
func TestQACeilingMessageArmUnderCommitCleanupVerbatimReadsTheBranchName(t *testing.T) {
	w := qaCeilingWall(t, "")
	env := qaVerbatimRepo(t, w, w.priv, "verbatim")

	// CONTROL 1 / fixture premise, on the unclassed branch.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: a clean editor commit must land: %v\n%s", err, out)
	}
	qaKeptItsTemplate(t, w, w.priv)

	// PROBE: the branch NAME carries the class, and nothing else does.
	branch := qaExportStem + "7"
	if out, err := w.git(w.priv, nil, "branch", "-m", branch); err != nil {
		t.Fatalf("git branch -m: %v %s", err, out)
	}
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		landed, lerr := w.git(w.priv, nil, "log", "-1", "--format=%B")
		if lerr != nil {
			t.Fatalf("git log: %v %s", lerr, landed)
		}
		if strings.Contains(landed, branch) {
			t.Errorf("a ceiling class carried by the BRANCH NAME reached the commit object — git keeps its "+
				"\"On branch\" line under commit.cleanup=verbatim and the arm stripped it out of the scan:\n%s", landed)
		}
	} else {
		if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
			t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke:\n%s", out)
		}
		w.unstage(t, w.priv, "internal/posse/probe.go")
	}

	// CONTROL 2: the wall is awake in this repo at this moment.
	w.stage(t, w.priv, "internal/posse/ctl2.go", "package posse\n")
	if o, e := w.git(w.priv, w.persona, "commit", "-m", "cite "+qaCeilingHit, "--", "internal/posse/ctl2.go"); e == nil {
		t.Fatalf("fixture premise: the same class typed with -m must still be refused:\n%s", o)
	}
}

// DIRECTION 2, and it is ranger-base-h3s6q finding 2 wearing a config: under
// commit.cleanup=strip git strips a '#'-leading line on the -m/-F path too,
// so the arm's whole-file read on that path refuses over text that cannot
// reach the commit object. Cheaper than the hole — the writer clears it by
// rewriting — but it is the same defect, in the direction the split was
// built to close.
//
// The crew's own commit form is `git commit -F - -- <paths>` (AGENTS.md), so
// the message is typed the way the crew types one.
//
// FIXTURE PREMISE, asserted after: git really did strip the line — if it
// landed, the read was right to refuse and this pin is measuring the wrong
// thing. CONTROL: the identical class NOT on a comment line, same repo, same
// config, is still refused.
//
// RUN AGAINST THE PRE-FIX ARM 2026-09-04, RED: refused with "data-ceiling
// content in the commit MESSAGE".
func TestQACeilingMessageArmUnderCommitCleanupStripOnTheDashMPath(t *testing.T) {
	w := qaCeilingWall(t, "")
	if out, err := w.git(w.priv, nil, "config", "commit.cleanup", "strip"); err != nil {
		t.Fatalf("git config commit.cleanup strip: %v %s", err, out)
	}

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	msg := "# cite " + qaCeilingHit + " here\nreal body\n"
	out, err := w.gitIn(w.priv, w.persona, msg, "commit", "-F", "-", "--", "internal/posse/probe.go")
	if err != nil {
		if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
			t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke:\n%s", out)
		}
		t.Errorf("the arm refused over a '#'-leading line that commit.cleanup=strip throws away before git "+
			"writes the object — the whole-file read on the -m/-F path is keyed on \"$2\", not on the "+
			"cleanup mode that actually decides:\n%s", out)
		w.unstage(t, w.priv, "internal/posse/probe.go")
	} else {
		landed, lerr := w.git(w.priv, nil, "log", "-1", "--format=%B")
		if lerr != nil {
			t.Fatalf("git log: %v %s", lerr, landed)
		}
		if strings.Contains(landed, qaCeilingHit) {
			t.Fatalf("fixture premise: commit.cleanup=strip must have removed the '#' line — it did not, so "+
				"this commit is a LEAK and the arm was right to read the file whole:\n%s", landed)
		}
	}

	// CONTROL: the wall is awake in this repo at this moment — the same class
	// on a line git keeps is still refused under the same config.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if o, e := w.gitIn(w.priv, w.persona, "cite "+qaCeilingHit+" here\n", "commit", "-F", "-", "--", "internal/posse/ctl.go"); e == nil {
		t.Fatalf("fixture premise: the same class on a line git KEEPS must still be refused:\n%s", o)
	}
}
