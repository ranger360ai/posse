package posse

// THE MESSAGE ARM STILL READS EVERYTHING BELOW THE SCISSORS LINE
// (ranger-base-dgh7y's other half, found verifying its close).
//
// ranger-base-dgh7y states the cost of the `commit.verbose` shape in two
// parts: "Everything git is about to throw away is scanned: the whole
// semicolon-prefixed status block AND everything below the scissors line."
// ranger-base-vl9g8's selector closed the FIRST part — the character git
// chose is now read off the last BARE comment-character line, which a
// unified diff cannot carry, so git's status block is stripped under `-v`
// and without it alike. MEASURED here, and it is a live fix: with vzx2n's
// `sed -n '$p' | cut -c1` selector put back under `go test -overlay`, only
// TestQACeilingMessageArmUnderAnAutoCommentCharWithAVerboseCommit reds and
// the five arms beside it stay green.
//
// The SECOND part is untouched, and nothing closed it. The read is
// `git stripspace --strip-comments`, which removes COMMENT lines; the diff
// git appends below its scissors marker under `commit -v` is not
// comment-prefixed, so it survives the strip and goes to the scan whole.
// vzx2n's own comment said so out loud — "The diff below it is read on every
// config already — it is not comment-prefixed and stripspace never took it
// out" — and 9f34e96 deleted that sentence while closing the block half.
//
// WHAT IT COSTS, and it is not what the deleted sentence assumed. The arm's
// sibling reads `git diff --cached -U0`: ADDED lines, zero context. What
// git writes under `-v` is the same diff with THREE lines of context, so the
// message arm scans lines no other arm sees and the writer did not write in
// this commit:
//
//   - an UNCHANGED line within three of a staged hunk refuses the commit
//     (pin 1), and
//   - REMOVING a classed line — the remediation the ceiling's own refusal
//     demands — is refused by the ceiling (pin 2), because the removal is
//     in the diff git appended.
//
// Both under the remedy "rewrite the commit message", which clears neither.
// One config key the writer owns (commit.verbose=true), or one flag. It is
// fail-CLOSED — an over-refusal, never a leak — which is why this is a
// findings bead and not a P1.
//
// NOT THE `auto` BRANCH'S. Pin 1 runs the same probe under
// core.commentChar=auto and under no comment-char config at all, and both
// arms refuse: the strip is what keeps the diff, on every config. So this
// predates vzx2n and vl9g8 alike and neither introduced it.
//
// PARKED, because the fix is the code lane's and the suite stays green.
// UNPARK: delete the t.Skip line in each pin below.
// RUN UNPARKED 2026-09-04 at main c68431d, git 2.50.1 (Apple Git-155),
// darwin 25.4.0: BOTH RED, each on its own assertion, each refused by the
// MESSAGE arm on the ceiling's restricted-banner class, one hit. Pin 1's
// two arms red together. Each pin's controls held first — the wall refuses
// the same class typed with -m (so it is awake), and the SAME edit with
// commit.verbose off LANDS (so it is `-v` that does this and not the edit).
//
// LIVE WHERE, stated exactly: not reached on this box at this instant, and
// only because the installed render predates the h3s6q fix — the rendered
// prepare-commit-msg here still reads "$1" whole on every path, so it
// over-refuses more broadly already. This goes live at the next
// `posse gates install-hooks` from a binary holding 9f34e96.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaVerboseEditor is the editor every pin here uses: it PREPENDS a clean
// line and leaves git's file alone, so .git/COMMIT_EDITMSG is still the
// evidence afterwards and nothing the wall could trip on is typed.
func qaVerboseEditor(t *testing.T, line string) string {
	t.Helper()
	ed := filepath.Join(t.TempDir(), "editor.sh")
	write(t, ed, "#!/bin/sh\n{ printf '%s\\n' '"+line+"'; cat \"$1\"; } > \"$1.new\" && mv \"$1.new\" \"$1\"\n")
	if err := os.Chmod(ed, 0o755); err != nil {
		t.Fatal(err)
	}
	return ed
}

// qaVerboseWallIsAwake is the control every pin rests on: the same class
// typed into a -m message in this repo at this moment is REFUSED. Without
// it a landing commit below would say nothing.
func qaVerboseWallIsAwake(t *testing.T, w *visWall, env []string, rel string) {
	t.Helper()
	w.stage(t, w.priv, rel, "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "-m", "cite "+qaCeilingHit, "--", rel); err == nil {
		t.Fatalf("control: the ceiling must refuse this class in a -m message here, or nothing below is a verdict:\n%s", out)
	}
	w.unstage(t, w.priv, rel)
}

// PIN 1. An UNCHANGED line refuses the commit, on both comment-char
// configs. The class sits two lines above the staged hunk: `-U0`, which is
// what the ADDED-line arm reads, never shows it; the `-v` diff's three
// lines of context do.
func TestQACeilingMessageArmReadsBelowTheScissorsUnderCommitVerbose(t *testing.T) {
	t.Skip("PARKED: ranger-base-dgh7y's other half — unpark by deleting this line")
	for _, tc := range []struct {
		name string
		auto bool
	}{
		{"core.commentChar=auto", true},
		{"no comment-char config at all", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := qaCeilingWall(t, "")
			var env []string
			if tc.auto {
				env = qaAutoCommentCharRepo(t, w, w.priv, "# notes\n")
			} else {
				env = append(append([]string(nil), w.persona...), "GIT_EDITOR="+qaVerboseEditor(t, "wire it"))
			}
			qaVerboseWallIsAwake(t, w, env, "internal/posse/ctl.go")

			const rel = "internal/posse/legacy.go"
			// HISTORY: a file already carrying the class, planted past the
			// hook — content that predates the ceiling's configuration,
			// which is the only way any of it is in a tree at all.
			w.plant(t, w.priv, rel, "package posse\n\n// "+qaCeilingHit+"\n\nvar A = 1\n")

			// CONTROL: the same edit with commit.verbose OFF must LAND —
			// nothing in this change trips any arm.
			if out, err := w.git(w.priv, nil, "config", "commit.verbose", "false"); err != nil {
				t.Fatalf("git config: %v %s", err, out)
			}
			w.stage(t, w.priv, rel, "package posse\n\n// "+qaCeilingHit+"\n\nvar A = 2\n")
			if out, err := w.git(w.priv, env, "commit", "--", rel); err != nil {
				t.Fatalf("control: the same edit with commit.verbose off must land, or `-v` is not what this pin measures: %v\n%s", err, out)
			}

			if out, err := w.git(w.priv, nil, "config", "commit.verbose", "true"); err != nil {
				t.Fatalf("git config: %v %s", err, out)
			}
			w.stage(t, w.priv, rel, "package posse\n\n// "+qaCeilingHit+"\n\nvar A = 3\n")
			// FIXTURE PREMISES: the ADDED-line arm's subject carries no
			// class, and the diff git is about to append does.
			zero, err := w.git(w.priv, nil, "diff", "--cached", "-U0", "--", rel)
			if err != nil {
				t.Fatalf("git diff --cached -U0: %v %s", err, zero)
			}
			if strings.Contains(zero, qaCeilingHit) {
				t.Fatalf("fixture premise: `-U0` must NOT carry the class, or the ADDED-line arm is what refuses:\n%s", zero)
			}
			ctx, err := w.git(w.priv, nil, "diff", "--cached", "--", rel)
			if err != nil {
				t.Fatalf("git diff --cached: %v %s", err, ctx)
			}
			if !strings.Contains(ctx, " // "+qaCeilingHit) {
				t.Fatalf("fixture premise: the diff git appends under `-v` must carry the class on a CONTEXT line:\n%s", ctx)
			}

			out, err := w.git(w.priv, env, "commit", "--", rel)
			if err == nil {
				return // the defect is gone; nothing else to say.
			}
			if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
				t.Fatalf("fixture premise: the probe must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
			}
			t.Errorf("under commit.verbose the message arm read the staged DIFF git appended below its "+
				"scissors marker and is about to throw away: an UNCHANGED line, three of context away "+
				"from the hunk and invisible to the `-U0` arm, refused an editor commit whose typed "+
				"message is clean, under the remedy 'rewrite the commit message', which clears "+
				"nothing:\n%s", out)
		})
	}
}

// PIN 2. The removal shape, and the sharpest reading of the same defect:
// the ceiling refuses the remediation its own refusal demands. Taking a
// classed line OUT puts it in the diff git appends, so the wall that exists
// to keep that class out of every local file refuses the commit that
// removes it.
func TestQACeilingMessageArmRefusesRemovingClassedContentUnderCommitVerbose(t *testing.T) {
	t.Skip("PARKED: ranger-base-dgh7y's other half — unpark by deleting this line")
	w := qaCeilingWall(t, "")
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR="+qaVerboseEditor(t, "take it out"))
	qaVerboseWallIsAwake(t, w, env, "internal/posse/ctl.go")

	const rel = "internal/posse/legacy.go"
	w.plant(t, w.priv, rel, "package posse\n\n// "+qaCeilingHit+"\n\nvar A = 1\n")

	// CONTROL: the identical removal with commit.verbose OFF lands. That is
	// what says the removal itself is clean by every other arm, and the pin
	// below measures `-v` and nothing else.
	if out, err := w.git(w.priv, nil, "config", "commit.verbose", "false"); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	w.stage(t, w.priv, rel, "package posse\n\nvar A = 1\n")
	if out, err := w.git(w.priv, env, "commit", "--", rel); err != nil {
		t.Fatalf("control: the removal with commit.verbose off must land: %v\n%s", err, out)
	}

	// Now the same removal, with `-v` on.
	w.plant(t, w.priv, rel, "package posse\n\n// "+qaCeilingHit+"\n\nvar A = 1\n")
	if out, err := w.git(w.priv, nil, "config", "commit.verbose", "true"); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	w.stage(t, w.priv, rel, "package posse\n\nvar A = 1\n")
	zero, err := w.git(w.priv, nil, "diff", "--cached", "-U0", "--", rel)
	if err != nil {
		t.Fatalf("git diff --cached -U0: %v %s", err, zero)
	}
	if !strings.Contains(zero, "-// "+qaCeilingHit) {
		t.Fatalf("fixture premise: the staged diff must show the classed line being REMOVED:\n%s", zero)
	}
	out, err := w.git(w.priv, env, "commit", "--", rel)
	if err == nil {
		return // the defect is gone.
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: the probe must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
	}
	t.Errorf("under commit.verbose the ceiling refused the REMOVAL of classed content — the one "+
		"remediation that takes the class out of the tree — through its MESSAGE arm, because the "+
		"removed line is in the diff git appended below the scissors and the arm reads it:\n%s", out)
}
