package posse

// THE TWO GUARDS THAT NARROW THE `auto` DETECTION ARE NOT MEASURED BY
// ANYTHING (ranger-base-n3s4s, verifying ranger-base-vzx2n's close).
//
// vzx2n's fix reads the comment character back off the template git already
// wrote into "$1", because for `auto` the character is a decision git makes
// per message and stripspace has no message to make it against. The read is
// admitted by THREE conditions in messageArm's rendered block: the first
// character of the LAST line is one git can choose, the file carries a BARE
// line of it, and at least four lines start with it. The second and third
// are what keep the detection from firing on a message that has no template
// at all — and where it fires wrongly it strips lines git KEEPS, which is
// the fail-open vzx2n exists to close, reopened.
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

// PARKED, AND RED TODAY: `commit.verbose` TAKES THE CHARACTER AWAY AGAIN
// (ranger-base-n3s4s, verifying ranger-base-vzx2n; filed as its own bug bead
// because the arm is a constitutional wall).
//
// vzx2n reads the comment character off the LAST line of the file git handed
// the hook, which under `auto` is git's status block. Set `commit.verbose`
// and it is not: git appends its scissors line and then the whole staged
// DIFF below it, so the last line of the file is a diff line — MEASURED,
// git 2.50.1 (Apple Git-155), the hook was handed a file ending `+# third`.
// A diff line starts with '+', ' ' or '-', none of which git can choose, so
// the detection falls through to the whole-file read.
//
// Fail-CLOSED, and that is the whole cost — nothing leaks. What returns is
// ranger-base-h3s6q's finding 2, the over-refusal
// TestQACeilingMessageArmUnderAnAutoCommentCharStillReadsGitsTemplate exists
// to keep closed: everything git is about to throw away is scanned, which is
// the ';'-prefixed status block AND everything below the scissors line. One
// UNTRACKED file whose NAME carries a class, never staged and never typed,
// refuses an editor commit whose typed message is clean, under the remedy
// "rewrite the commit message" — which clears nothing, because the writer
// did not write it. That pin does not see this: it leaves commit.verbose
// unset, so its last line is a comment and the detection succeeds.
//
// Reachable from the WRITER's config alone, two ordinary keys in
// ~/.gitconfig, no intent. RUN UNPARKED 2026-09-04 on 2d2f139, RED: refused
// by the MESSAGE arm on the ceiling's export-name class, one hit, over the
// untracked path in git's own listing.
//
// NOT A REGRESSION, measured rather than assumed: the same pin unparked
// against the PRE-vzx2n arm (gates.go at 36efc82, `go test -overlay`) is red
// too, on the same assertion. Plain stripspace answered '#' and never
// stripped the ';' block either. vzx2n closed this over-refusal for the
// ordinary case and left the verbose one open without saying so; its own
// close states only the fail-OPEN residual.
//
// UNPARK BY REMOVING THE SKIP BELOW when the arm learns the character from
// something the verbose diff cannot move — git's own answer for the file
// rather than the file's last line.
func TestQACeilingMessageArmUnderAutoWithCommitVerbose(t *testing.T) {
	t.Skip("PARKED (ranger-base-n3s4s): red today — commit.verbose puts a diff line last, so the auto character detection falls through to the whole-file read and h3s6q finding 2 returns")

	w := qaCeilingWall(t, "")
	env := qaAutoCommentCharRepo(t, w, w.priv, "# notes\n")
	if out, err := w.git(w.priv, nil, "config", "commit.verbose", "true"); err != nil {
		t.Fatalf("git config commit.verbose: %v %s", err, out)
	}

	// CONTROL: nothing classed anywhere — the editor path must still land.
	w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "--", "internal/posse/ctl.go"); err != nil {
		t.Fatalf("fixture premise: a clean editor commit must land: %v\n%s", err, out)
	}
	// PREMISE: git really did decline '#' and write its template with ';', or
	// the two readers never disagreed here at all.
	qaAutoSwitched(t, w, w.priv)

	// PROBE: ONE untracked file, never staged, never typed. Its name reaches
	// the file only because git listed it, and git throws that listing away.
	write(t, filepath.Join(w.priv, qaExportStem+"42.csv"), "x\n")
	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		return // the defect is gone; nothing else to say.
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: the probe must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
	}
	t.Errorf("commit.verbose put a diff line last, so the arm could not read the character git chose and "+
		"scanned the file whole — an UNTRACKED path git listed in a block it discards refused an editor "+
		"commit whose typed message is clean, and the remedy names a message the writer did not write:\n%s", out)
}
