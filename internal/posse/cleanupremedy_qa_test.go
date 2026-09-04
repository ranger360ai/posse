package posse

// THE REMEDY DID NOT FIT THE MODES THAT KEEP GIT'S TEMPLATE (ranger-base-b21e0,
// the stated cost of ranger-base-6y3z2 rather than a defect it left behind).
//
// 6y3z2 made the MESSAGE arm read "$1" WHOLE under commit.cleanup=verbatim,
// whitespace or scissors, because under those modes git strips no comment line
// out of that file — its own template included. The verdict is
// true and the direction is fail-closed. The REMEDY was not: prepare-commit-msg
// runs BEFORE the editor opens, so on the editor path one untracked file whose
// NAME carries a class refuses the commit with "rewrite the commit message",
// naming a rewrite that has not happened yet and cannot clear a hit the writer
// never typed. That is ranger-base-h3s6q finding 2 narrowed to the writers whose
// config keeps comments.
//
// THE FIX IS THE BEAD'S SHAPE (1) AND NOT (2): the refusal now says which mode
// is live and what actually clears it — delete git's block in the editor, or
// leave commit.cleanup at its default. It does not need the commit-msg hook ADR
// 0050 D5 and ADR 0024 D2 name as the missing second layer, and it does not
// pretend to be one: the note says "if", because the hook has the file and not
// the writer's hands.
//
// MEASURED AT THE SHELL FIRST, git 2.50.1 (Apple Git-155), not inferred:
//
//	verbatim, whitespace   git's whole template LANDS in the object — the
//	                       "Please enter", "On branch", staged and UNTRACKED
//	                       blocks, every line of it.
//	scissors               git puts its template BELOW the cut line and
//	                       truncates all of it: "wire it" landed alone. So
//	                       under scissors the whole-file read scans a block
//	                       that never reaches the object — over-refusal on top
//	                       of over-refusal, and the note says so rather than
//	                       matching git's cut line, which would be a second
//	                       copy of git's rule (gates.go's argument against a
//	                       literal '#').
//	-m under verbatim      "$2" is "message" and git appends NO template: the
//	                       file is the typed message alone. The remedy there is
//	                       doable exactly as written, so the note stays quiet —
//	                       which is what the absence pin below measures.
//
// MUTATION-CHECKED, per pin and per alternative, because a green pin over a
// wall that never had the hole measures nothing. Three mutants through
// `go test -overlay` against gates.go, run 2026-09-04; each was killed by the
// pin it belongs to and by NO other, which is what says the pins measure three
// different things and not one:
//
//	keptModeNote returns "" always     (the PRE-FIX arm) — the two presence
//	                                   pins RED, the absence pin GREEN.
//	the `[ "${2:-}" != "message" ]`    — the absence pin RED on its verbatim
//	guard removed from messageArm        arm alone, both presence pins GREEN.
//	the `= scissors` test replaced      — the scissors pin RED, everything
//	by `false`                           else GREEN.
//
// The absence pin is the one this change could break rather than the one it
// fixes: it is green pre-fix by construction, and its whole job is the second
// mutant above.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaFlat collapses every whitespace run to one space. The refusal is HARD
// WRAPPED at render time, so a phrase a writer reads as one ("delete git's
// block in the editor") is two lines in the bytes, and a substring assertion
// over the raw output is a line scan blind to exactly the phrases worth
// pinning. What is under test is the sentence, not where it wraps.
func qaFlat(s string) string { return strings.Join(strings.Fields(s), " ") }

// qaCleanupRemedyProbe arms repo at mode, refuses ONE editor commit over an
// untracked file whose NAME carries the box's username, and returns what the
// wall printed. The untracked path is the shortest reachable spelling of "a
// class the writer did not type": never staged, never edited, git's own status
// block is the only thing that carries it into the message.
//
// CONTROL 1, before the probe: a clean editor commit under the same config
// LANDS — the wall is not refusing everything in this repo at this moment.
func qaCleanupRemedyProbe(t *testing.T, w *visWall, mode string) string {
	t.Helper()
	env := qaVerbatimRepo(t, w, w.pub, mode)
	username := w.literal(t, "username")

	w.stage(t, w.pub, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.pub, env, "commit", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: a clean editor commit under commit.cleanup=%s must land: %v\n%s", mode, err, out)
	}

	write(t, filepath.Join(w.pub, username+"-notes.txt"), "x\n")
	w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.pub, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		t.Fatalf("fixture premise: under commit.cleanup=%s the arm reads the message file whole, so the "+
			"untracked file's NAME in git's status block must refuse this commit — there is no remedy to "+
			"measure if nothing was refused:\n%s", mode, out)
	}
	if !strings.Contains(qaFlat(out), "an operator identity literal in the commit MESSAGE") {
		t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that spoke, "+
			"since the remedy under test is that arm's:\n%s", out)
	}
	w.unstage(t, w.pub, "internal/posse/probe.go")
	return out
}

// THE PIN: under a template-keeping mode the refusal names the mode and the two
// things that clear it, and it still carries the remedy it always carried — the
// note is additive. A writer whose hit IS in their own typed text must not lose
// "rewrite the commit message" to gain the paragraph about git's block.
//
// RUN AGAINST THE PRE-FIX ARM 2026-09-04, RED: the refusal named no mode.
func TestQAMessageRemedyNamesTheCleanupModeThatKeptGitsTemplate(t *testing.T) {
	for _, mode := range []string{"verbatim", "whitespace"} {
		t.Run(mode, func(t *testing.T) {
			w := newVisWall(t)
			out := qaCleanupRemedyProbe(t, w, mode)

			// FIXTURE PREMISE: git really does keep its template under this
			// mode — CONTROL 1's commit is the one that landed, so it is the
			// one that proves it. Without this the pin is a refusal over
			// bytes that were never going anywhere.
			qaKeptItsTemplate(t, w, w.pub)

			for _, want := range []string{
				`git's cleanup mode here is "` + mode + `"`,
				"config commit.cleanup",
				"delete git's block in the editor",
				"leave commit.cleanup at its default",
				"rewrite the commit message",
				"not a false alarm",
			} {
				if !strings.Contains(qaFlat(out), qaFlat(want)) {
					t.Errorf("the refusal must carry %q — the writer is refused BEFORE the editor opens, "+
						"over a line git wrote and they cannot rewrite, and %q is the mode that decided "+
						"it (ranger-base-b21e0):\n%s", want, mode, out)
				}
			}
			// Not the scissors clause: nothing is below a cut line here,
			// and this refusal is the honest kind, not over-refusal.
			if strings.Contains(qaFlat(out), "BELOW the cut line") || strings.Contains(out, "over-refuses") {
				t.Errorf("commit.cleanup=%s has no cut line and truncates nothing, so the refusal must not "+
					"describe one:\n%s", mode, out)
			}
		})
	}
}

// SCISSORS PUTS GIT'S BLOCK BELOW THE CUT LINE, AND THE READ STOPS THERE
// (ranger-base-xfgcn). b21e0 shipped the other answer here: the arm did not
// match git's cut line, so under `scissors` it read a block git truncates and
// the note said so rather than refusing quietly. The same untruncated read
// also refused an editor commit over an UNCHANGED context line, and over the
// REMOVAL of a classed line, once `commit -v` put a diff below that marker
// (verbosescissors_qa_test.go) — so the cut line is matched now, and this pin
// is the `scissors` side of that one change.
//
// ARM (a): git's status block is on the far side of the cut, so the untracked
// filename that refused every editor commit under this mode now LANDS. Its
// FIXTURE PREMISE is measured both ways: with the hook bypassed the same
// commit's message does NOT carry the name (so git really truncates it, and
// the refusal it used to draw really was over-refusal), and the identical
// probe under `verbatim` — where git keeps its block ABOVE any cut — is still
// REFUSED, which is what says arm (a) measures the cut line and not a wall
// that fell asleep.
//
// ARM (b): what is above the cut is still read, and still refused. A
// commit.template body is the shortest reachable spelling of that: git puts
// it at the top of the file, above its own cut line, and `scissors` strips no
// comment line out of what it keeps, so those bytes reach the object exactly
// as scanned. The refusal names the mode and carries the scissors note; it
// must NOT carry the paragraph about git's status block being scanned, which
// is no longer true under this mode.
//
// RUN AGAINST THE PRE-FIX ARM (gates.go before ranger-base-xfgcn): arm (a) is
// RED — the untracked name refuses — and arm (b) is red on the note text.
func TestQAMessageArmStopsAtGitsCutLineUnderScissors(t *testing.T) {
	t.Run("git's block below the cut is no longer read", func(t *testing.T) {
		w := newVisWall(t)
		env := qaVerbatimRepo(t, w, w.pub, "scissors")
		username := w.literal(t, "username")

		// CONTROL: the same probe under `verbatim`, where git's block is
		// above any cut line, is REFUSED. Same wall, same instant, same
		// untracked name — so a landing commit below is the cut line and
		// not a wall that stopped refusing.
		ctl := newVisWall(t)
		ctlEnv := qaVerbatimRepo(t, ctl, ctl.pub, "verbatim")
		write(t, filepath.Join(ctl.pub, ctl.literal(t, "username")+"-notes.txt"), "x\n")
		ctl.stage(t, ctl.pub, "internal/posse/probe.go", "package posse\n")
		if out, err := ctl.git(ctl.pub, ctlEnv, "commit", "--", "internal/posse/probe.go"); err == nil {
			t.Fatalf("control: under commit.cleanup=verbatim git's block is ABOVE any cut line and must "+
				"still refuse this, or the arm below measures nothing:\n%s", out)
		}

		write(t, filepath.Join(w.pub, username+"-notes.txt"), "x\n")
		w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n")
		if out, err := w.git(w.pub, env, "commit", "--", "internal/posse/probe.go"); err != nil {
			t.Errorf("under commit.cleanup=scissors git puts its status block BELOW its cut line and "+
				"truncates all of it, so the arm must not read the UNTRACKED filename it lists there — "+
				"the remedy 'rewrite the commit message' clears a name the writer never typed:\n%s", out)
		}

		// FIXTURE PREMISE, measured here rather than taken from the manual:
		// git truncates that block. Bypassing the hook is the only way to
		// land the commit the pre-fix wall refused (visWall.plant's reason).
		w.stage(t, w.pub, "internal/posse/premise.go", "package posse\n")
		if o, e := w.git(w.pub, env, "-c", "core.hooksPath=/dev/null", "commit", "--", "internal/posse/premise.go"); e != nil {
			t.Fatalf("planting the premise commit: %v %s", e, o)
		}
		landed, err := w.git(w.pub, nil, "log", "-1", "--format=%B")
		if err != nil {
			t.Fatalf("git log: %v %s", err, landed)
		}
		if strings.Contains(landed, username) {
			t.Fatalf("fixture premise: under commit.cleanup=scissors git must TRUNCATE the block carrying "+
				"the untracked name — it did not, so the read above was not over-refusal:\n%s", landed)
		}
	})

	t.Run("what is above the cut is still read", func(t *testing.T) {
		w := newVisWall(t)
		env := qaVerbatimRepo(t, w, w.pub, "scissors")
		username := w.literal(t, "username")

		// git puts a commit.template body ABOVE its cut line, and keeps
		// every byte of it under this mode.
		tpl := filepath.Join(t.TempDir(), "template")
		write(t, tpl, "refs "+username+"\n")
		if out, err := w.git(w.pub, nil, "config", "commit.template", tpl); err != nil {
			t.Fatalf("git config commit.template: %v %s", err, out)
		}
		w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n")
		out, err := w.git(w.pub, env, "commit", "--", "internal/posse/probe.go")
		if err == nil {
			t.Fatalf("a commit.template body sits ABOVE git's cut line and lands in the object under "+
				"commit.cleanup=scissors, so the arm must still read it:\n%s", out)
		}
		if !strings.Contains(qaFlat(out), "an operator identity literal in the commit MESSAGE") {
			t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that "+
				"spoke, since the remedy under test is that arm's:\n%s", out)
		}
		for _, want := range []string{
			`git's cleanup mode here is "scissors"`,
			"This read stops at that cut line",
			"neither scanned nor kept",
		} {
			if !strings.Contains(qaFlat(out), qaFlat(want)) {
				t.Errorf("under scissors the refusal must carry %q — what is above the cut is what was "+
					"read and what lands (ranger-base-xfgcn):\n%s", want, out)
			}
		}
		// The verbatim/whitespace paragraphs are about git's status block,
		// which this mode puts below the cut: printing them here would send
		// the writer to delete a block the arm never read.
		for _, no := range []string{"not a false alarm", "the staged, unstaged and UNTRACKED lists"} {
			if strings.Contains(qaFlat(out), qaFlat(no)) {
				t.Errorf("under scissors git's status block is below the cut and is not read, so the "+
					"refusal must not carry %q:\n%s", no, out)
			}
		}
	})
}

// THE ABSENCE PIN, and the only direction this change can break: where the
// writer TYPED the message, "rewrite the commit message" is doable exactly as
// written and the note is noise. "$2" is "message" for -m/-F and git appends no
// template on that path (measured), so the mode variable stays empty there —
// under a template-keeping config AND under an unset one.
//
// The crew's own commit form is `git commit -F - -- <paths>` (AGENTS.md), so the
// message is typed the way the crew types one.
//
// MUTATION: remove `[ "${2:-}" != "message" ]` from messageArm and this pin goes
// RED on the verbatim arm while both presence pins stay green — so it measures
// the guard and not the note.
func TestQAMessageRemedyStaysQuietWhereTheWriterTypedTheMessage(t *testing.T) {
	for _, mode := range []string{"verbatim", ""} {
		name := mode
		if name == "" {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			w := newVisWall(t)
			if mode != "" {
				if o, e := w.git(w.pub, nil, "config", "commit.cleanup", mode); e != nil {
					t.Fatalf("git config commit.cleanup %s: %v %s", mode, e, o)
				}
			}
			username := w.literal(t, "username")
			w.stage(t, w.pub, "internal/posse/probe.go", "package posse\n")
			out, err := w.gitIn(w.pub, w.persona, "followed "+username+"'s note\n",
				"commit", "-F", "-", "--", "internal/posse/probe.go")
			if err == nil {
				t.Fatalf("fixture premise: a literal typed into the message must be refused, or there is no "+
					"remedy here to read:\n%s", out)
			}
			if !strings.Contains(qaFlat(out), "rewrite the commit message") {
				t.Fatalf("fixture premise: the MESSAGE arm's remedy must be the one that spoke:\n%s", out)
			}
			if strings.Contains(qaFlat(out), "git's cleanup mode here is") {
				t.Errorf("commit.cleanup=%q with -F/-m appends no template, so every line of this message is "+
					"the writer's own and \"rewrite the commit message\" clears it — the mode note is noise "+
					"here (ranger-base-b21e0):\n%s", name, out)
			}
		})
	}
}

// The rendered hook is shell, and a note that only ever prints on a refusal is
// a branch no other pin in this file reaches until something is already wrong.
// `sh -n` over the installed file is the cheap standing check that the block
// parses at all — it costs one process, and it is the difference between a
// syntax error found here and one found by a writer being refused.
func TestQAInstalledCommitHookParses(t *testing.T) {
	w := newVisWall(t)
	hook := filepath.Join(w.pub, ".git", "hooks", "prepare-commit-msg")
	if _, err := os.Stat(hook); err != nil {
		t.Fatalf("fixture premise: the wall installs a prepare-commit-msg hook: %v", err)
	}
	out, err := exec.Command("sh", "-n", hook).CombinedOutput()
	if err != nil {
		t.Fatalf("the rendered prepare-commit-msg hook must parse as /bin/sh: %v\n%s", err, out)
	}
}
