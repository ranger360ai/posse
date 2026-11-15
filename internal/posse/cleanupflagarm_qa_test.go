//go:build !posse_arm2 && !posse_arm3

package posse

// THE CLEANUP MODE CAN ARRIVE AS A FLAG, and the message arm reads it from
// git's own template (ranger-base-cufvv, the finding ranger-base-dgfax's
// verify filed against ranger-base-r5j00).
//
// messageArm asks git's CONFIG for the cleanup mode:
//
//	posse_clean=$(git config --get commit.cleanup 2>/dev/null) || posse_clean=''
//
// `git commit --cleanup=<mode>` sets the mode for that one commit and writes
// nothing to config, so the read comes back EMPTY and the arm takes its
// "default" branch, where $posse_typed decides. $posse_typed is
// GIT_EDITOR=":", which is git saying it opened no editor
// (ranger-base-r5j00). With an editor on the same command line, the two
// disagreed: git's mode was "verbatim", so it KEPT every '#' line of its own
// template, while the arm read the file through `git stripspace
// --strip-comments`. Checks 3 and 4 scanned less than git committed. That is
// the ranger-base-6y3z2 class (git's own template landing unscanned under a
// keep mode), surviving where the mode is spelled as a flag.
//
// MEASURED 2026-11-14, git 2.56.0, commit.cleanup UNSET, one '#' line in the
// message and a recorder hook printing $GIT_EDITOR, over the six modes crossed
// with the editor. "the arm reads" is the PRE-FIX arm:
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
// THE FIX (ranger-base-cufvv). git writes the live mode into its template
// before this hook runs. MEASURED 2026-11-15, git 2.56.0, LC_ALL=C:
// `whitespace` and `verbatim` write "... will be kept; you may remove them
// yourself if you want to.", and default and `strip` write "... will be
// ignored ...". So when the arm would strip on the editor path and the
// "will be kept" line is present, it reads WHOLE. That line can only ever add
// to the scan. A forger over-refuses their own commit, and a reworded or
// translated sentence leaves the pre-fix read.
//
// The `scissors` row is NOT this hole. Under that flag, git puts its whole
// template below its cut line and truncates it (measured, same run), so
// nothing git wrote lands. What its row measured was a '#' line ABOVE the
// cut, which git kept and the arm stripped. That residual, and the
// translated sentence, are ranger-base-0wb2u. Both are fail-OPEN, and both
// were the pre-fix read.
//
// THIS PIN WAS GREEN ASSERTING THE HOLE at 67f99d9a (ranger-base-dgfax), and
// it is flipped here: the probe is now refused, and the class does not reach
// the object.

import (
	"strings"
	"testing"
)

// TestQACeilingMessageArmUnderACleanupFlagThatKeepsGitsTemplate drives the
// ceiling's message arm with the keep mode as a FLAG and an editor open. It is
// the branch-name probe of TestQACeilingMessageArmUnderCommitCleanupVerbatim-
// ReadsTheBranchName with the mode moved from config onto the command line,
// which is the whole of the difference under test. That pin's green is what
// says this one's reading is the flag and not the mode.
func TestQACeilingMessageArmUnderACleanupFlagThatKeepsGitsTemplate(t *testing.T) {
	for _, mode := range []string{"verbatim", "whitespace"} {
		t.Run(mode, func(t *testing.T) {
			w := qaCeilingWall(t, "")
			flag := "--cleanup=" + mode

			// FIXTURE PREMISE: the mode must NOT be in config, or the arm
			// sees it through the config reader and this measures nothing.
			if out, err := w.git(w.priv, nil, "config", "--get", "commit.cleanup"); err == nil {
				t.Fatalf("fixture premise: commit.cleanup must be unset — the flag is the subject here, got %q", out)
			}
			env := qaWiringEditor(t, w)

			// CONTROL 1 / fixture premise: a clean editor commit with the mode
			// on the command line LANDS and git KEEPS its own template. That
			// says the fix refuses a class, not every flag commit, and that
			// git really does keep the block the probe puts the class into.
			w.stage(t, w.priv, "internal/posse/ctl.go", "package posse\n")
			if out, err := w.git(w.priv, env, "commit", flag, "--", "internal/posse/ctl.go"); err != nil {
				t.Fatalf("fixture premise: a clean editor commit under %s must land: %v\n%s", flag, err, out)
			}
			qaKeptItsTemplate(t, w, w.priv)
			head, _ := w.git(w.priv, nil, "rev-parse", "HEAD")

			// PROBE: the branch NAME carries the ceiling class and nothing else
			// does, so the only route into the object is git's own "On branch"
			// line, which this mode keeps.
			branch := qaExportStem + "41"
			if out, err := w.git(w.priv, nil, "branch", "-m", branch); err != nil {
				t.Fatalf("git branch -m: %v %s", err, out)
			}
			w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
			out, cerr := w.git(w.priv, env, "commit", flag, "--", "internal/posse/probe.go")
			if cerr == nil {
				landed, _ := w.git(w.priv, nil, "log", "-1", "--format=%B")
				t.Fatalf("THE HOLE IS OPEN (ranger-base-cufvv): `git commit %s` with an editor open landed a "+
					"ceiling class git wrote into its OWN template, unrefused.\ncommit object:\n%s\nrefusals.log:\n%s",
					flag, landed, w.log(t))
			}
			if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
				t.Fatalf("the commit was refused, but not by the commit-message arm:\n%s", out)
			}
			// The note names where the mode came from, so the writer is not
			// sent to a config key they never set.
			if !strings.Contains(out, "a --cleanup flag") || !strings.Contains(out, "whitespace or verbatim") {
				t.Errorf("the refusal must say the mode came from a --cleanup flag:\n%s", out)
			}
			qaNoCeilingVocabulary(t, "the refusal", out)
			if now, _ := w.git(w.priv, nil, "rev-parse", "HEAD"); now != head {
				t.Fatalf("the probe was reported refused but HEAD moved:\n%s", out)
			}
			if rl := w.log(t); !strings.Contains(rl, "commit message") {
				t.Errorf("refusals.log must record the message arm's refusal:\n%s", rl)
			}

			// CONTROL 2: the same branch, the same editor and NO flag. git
			// strips its template there, so the arm must strip too, and the
			// commit lands. Without this, a fix that read every editor commit
			// whole would pass the probe and bring back ranger-base-h3s6q's
			// over-refusal.
			if out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go"); err != nil {
				t.Fatalf("CONTROL: an editor commit with no flag on the classed branch must land (git strips "+
					"its template there): %v\n%s", err, out)
			}
			if landed, _ := w.git(w.priv, nil, "log", "-1", "--format=%B"); strings.Contains(landed, branch) {
				t.Fatalf("CONTROL premise: with no flag git must strip its \"On branch\" line:\n%s", landed)
			}

			// CONTROL 3: the wall is awake in this repo at this moment.
			w.stage(t, w.priv, "internal/posse/ctl2.go", "package posse\n")
			if o, e := w.git(w.priv, w.persona, "commit", "-m", "cite "+qaCeilingHit, "--", "internal/posse/ctl2.go"); e == nil {
				t.Fatalf("CONTROL premise: the same class typed with -m must still be refused:\n%s", o)
			}
		})
	}
}
