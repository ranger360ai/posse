//go:build !posse_arm2 && !posse_arm3

package posse

// THE MESSAGE ARM READ EVERYTHING BELOW THE SCISSORS LINE
// (ranger-base-xfgcn, ranger-base-dgh7y's other half, found verifying its
// close; the fix is the cut in messageArm).
//
// ranger-base-dgh7y stated the cost of the `commit.verbose` shape in two
// parts: "Everything git is about to throw away is scanned: the whole
// semicolon-prefixed status block AND everything below the scissors line."
// ranger-base-vl9g8's selector closed the FIRST part — the character git
// chose is now read off the last BARE comment-character line, which a
// unified diff cannot carry, so git's status block is stripped under `-v`
// and without it alike.
//
// The SECOND part was untouched, and dgh7y was closed with no product
// change. The read is `git stripspace --strip-comments`, which removes
// COMMENT lines; the diff git appends below its scissors marker under
// `commit -v` is not comment-prefixed, so it survived the strip and went to
// the scan whole.
//
// WHAT IT COST, and it is not a longer status block. The arm's sibling reads
// `git diff --cached -U0`: ADDED lines, zero context. What git writes under
// `-v` is the same diff with THREE lines of context, so the message arm
// scanned lines no other arm sees and the writer did not write in this
// commit:
//
//   - an UNCHANGED line within three of a staged hunk refused the commit
//     (pin 1), and
//   - REMOVING a classed line — the remediation the ceiling's own refusal
//     demands — was refused by the ceiling (pin 2), because the removal is
//     in the diff git appended.
//
// Both under the remedy "rewrite the commit message", which clears neither.
// One config key the writer owns (commit.verbose=true), or one flag. It was
// fail-CLOSED — an over-refusal, never a leak — which is why it was a
// findings bead and not a P1.
//
// NOT THE `auto` BRANCH'S. Pin 1 runs the same probe under
// core.commentChar=auto and under no comment-char config at all, and both
// arms refused: the strip is what kept the diff, on every config. So this
// predated vzx2n and vl9g8 alike and neither introduced it.
//
// THE FIX, and what each pin here holds it to. The read stops at git's cut
// line, where git's own cleanup stops: the FIRST line that is a comment
// prefix plus git's marker, never matched on a line a unified diff could
// carry. The cut is unconditional under commit.cleanup=scissors, where git
// truncates on the marker alone; under every other mode the block below the
// marker is read MINUS the lines of the staged diff — which is what `-v`
// writes there, and bytes the index already holds — and whatever is left is
// message (ranger-base-gyrnp; the earlier licence, a `diff --` line below
// the cut, was writer-typed, and messagecutfailopen_qa_test.go pins the
// forgery it allowed). Pins 1 and 2 are the defect; pin 3 is the guard (a
// marker line git did NOT write must not take the text below it off the
// scan); pin 4 is a marker forged above git's own under `-v`, where the
// residual is fail-CLOSED and this pin records the reversal; pin 5 is
// `--amend`, where git's diff is against HEAD^1 and so must the reference
// be, or the lines the two bases disagree on are read as message.
//
// EACH PIN CARRIES ITS CONTROLS, asserted before the verdict: the same class
// typed with -m in the same repo at the same moment is REFUSED (the wall is
// awake), and where the pin measures `-v`, the SAME edit with commit.verbose
// OFF LANDS (so it is `-v` that does it and not the edit).
//
// RUN PARKED AT c68431d, BOTH RED, three arms, each on its own assertion,
// each refused by the MESSAGE arm on the ceiling's restricted-banner class,
// one hit. MUTATION-CHECKED against the fix as ranger-base-gyrnp reshaped it,
// `go test -overlay`, three mutants, each read on the pins it reds:
//
//	nothing subtracted (the reference   pins 1, 2, 5 RED; arm 3's landing
//	replaced by an empty list)          control RED
//	everything subtracted (the block    pin 3, pin 4 and all three forgery
//	below the cut never read)           arms RED
//	the --amend base left at HEAD       pin 5 alone RED
//
// LIVE WHERE, stated exactly: the defect was not reached on this box at the
// instant it was found, and only because the installed render predates the
// h3s6q fix — the rendered prepare-commit-msg there still reads "$1" whole on
// every path, so it over-refuses more broadly already. Both the defect and
// this fix reach a box at its next `posse gates install-hooks`.

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

// PIN 3. THE GUARD, and the only direction this fix could break: the cut is
// git's truncation, not a marker line anyone can write. git truncates at its
// cut line when the commit is verbose or when commit.cleanup is `scissors`,
// and under every other mode a file that carries the marker is kept WHOLE —
// MEASURED, git 2.50.1: a commit.template body with the marker in it landed
// in the object with the text below the marker intact. So cutting on the
// marker alone would take exactly that text off the scan, which is fail-OPEN
// in a wall whose subject is text that may not land.
//
// The probe is that same template, under the default cleanup mode and no
// verbose: git keeps the classed line below the forgery, so the arm must
// still read it. Its FIXTURE PREMISE is measured here, with the hook
// bypassed, rather than assumed — if git dropped those bytes the refusal
// would be the over-refusal this file exists to remove.
func TestQACeilingMessageArmDoesNotCutOnAForgedScissorsLine(t *testing.T) {
	w := qaCeilingWall(t, "")
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR="+qaVerboseEditor(t, "wire it"))

	// The marker line git writes, forged into a commit.template body — with
	// the class BELOW it, and not comment-prefixed, so `strip` keeps it.
	tpl := filepath.Join(t.TempDir(), "template")
	write(t, tpl, "from the template\n# ------------------------ >8 ------------------------\n"+qaCeilingHit+"\n")
	if out, err := w.git(w.priv, nil, "config", "commit.template", tpl); err != nil {
		t.Fatalf("git config commit.template: %v %s", err, out)
	}

	// FIXTURE PREMISE: git keeps it. Bypassing the hook is the only way to
	// land the commit the wall is about to refuse (visWall.plant's reason).
	w.stage(t, w.priv, "internal/posse/premise.go", "package posse\n")
	if o, e := w.git(w.priv, env, "-c", "core.hooksPath=/dev/null", "commit", "--", "internal/posse/premise.go"); e != nil {
		t.Fatalf("planting the premise commit: %v %s", e, o)
	}
	landed, err := w.git(w.priv, nil, "log", "-1", "--format=%B")
	if err != nil {
		t.Fatalf("git log: %v %s", err, landed)
	}
	if !strings.Contains(landed, qaCeilingHit) {
		t.Fatalf("fixture premise: with no verbose and no commit.cleanup=scissors git must KEEP the text "+
			"below a marker line it did not write — it did not, so there is nothing here to be read:\n%s", landed)
	}

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		t.Errorf("the arm cut at a marker line git did not write: with no verbose and no "+
			"commit.cleanup=scissors git truncates nothing, so the classed line below it lands in the "+
			"commit object and the arm must read it:\n%s", out)
		return
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: this commit must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
	}
}

// PIN 4. A marker forged ABOVE git's own, under `-v`. git truncates at the
// FIRST such line — MEASURED, git 2.50.1: the class between the two is gone
// from the object — and the arm reads it anyway, because the licence is not
// "which marker" but "what below the cut is the staged diff": the class is
// not a line of that diff, so it is message here and is refused. That is
// the fail-CLOSED residual messageArm states, and this pin is the record of
// the reversal: before ranger-base-gyrnp it required this commit to LAND,
// because the cut was taken on the marker alone and the class was below it.
//
// CONTROL: the same template with a clean line in the class's place, under
// the same `-v`, LANDS — so it is the class that refuses and not the forged
// marker. FIXTURE PREMISE, measured with the hook bypassed: git really does
// drop the class, which is what makes this over-refusal and not a catch.
func TestQACeilingMessageArmReadsBetweenAForgedMarkerAndGitsOwn(t *testing.T) {
	w := qaCeilingWall(t, "")
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR="+qaVerboseEditor(t, "wire it"))
	qaVerboseWallIsAwake(t, w, env, "internal/posse/ctl.go")

	if out, err := w.git(w.priv, nil, "config", "commit.verbose", "true"); err != nil {
		t.Fatalf("git config commit.verbose: %v %s", err, out)
	}
	const forged = "from the template\n# ------------------------ >8 ------------------------\n"
	tpl := filepath.Join(t.TempDir(), "template")
	if out, err := w.git(w.priv, nil, "config", "commit.template", tpl); err != nil {
		t.Fatalf("git config commit.template: %v %s", err, out)
	}

	// CONTROL: a clean line between the forged marker and git's own lands.
	write(t, tpl, forged+"kept by nobody\n")
	w.stage(t, w.priv, "internal/posse/control.go", "package posse\n")
	if out, err := w.git(w.priv, env, "commit", "--", "internal/posse/control.go"); err != nil {
		t.Fatalf("control: a forged marker with a clean line below it must land under -v: %v\n%s", err, out)
	}

	// FIXTURE PREMISE: git truncates at the FIRST marker, so the class is
	// not in the object — with the hook bypassed, the only way to see that.
	write(t, tpl, forged+qaCeilingHit+"\n")
	w.stage(t, w.priv, "internal/posse/premise.go", "package posse\n")
	if o, e := w.git(w.priv, env, "-c", "core.hooksPath=/dev/null", "commit", "--", "internal/posse/premise.go"); e != nil {
		t.Fatalf("planting the premise commit: %v %s", e, o)
	}
	if landed, e := w.git(w.priv, nil, "log", "-1", "--format=%B"); e != nil || strings.Contains(landed, qaCeilingHit) {
		t.Fatalf("fixture premise: git must truncate at the FIRST marker line, so the class must NOT be in "+
			"the object (%v):\n%s", e, landed)
	}

	w.stage(t, w.priv, "internal/posse/probe.go", "package posse\n")
	out, err := w.git(w.priv, env, "commit", "--", "internal/posse/probe.go")
	if err == nil {
		t.Fatalf("LANDED: the class between a forged marker and git's own was taken off the scan — the cut "+
			"is being licensed on the marker again, not on the staged diff:\n%s", out)
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: if this commit is refused at all it must be the MESSAGE arm that "+
			"spoke:\n%s", out)
	}
}

// PIN 5. `--amend` under `-v`: git diffs the index against HEAD^1 there and
// writes THAT below its cut (measured inside the hook, git 2.50.1: "$2" is
// `commit`, "$3" is `HEAD`), so the reference must be the same diff or the
// lines the two bases disagree on are read as message. The shape that tells
// the bases apart is the remediation one turn later: HEAD removed the classed
// line, the amend stages one more edit, and the `-v` diff against HEAD^1
// carries the REMOVAL of the class where the diff against HEAD does not.
// CONTROL: the same amend with commit.verbose off lands. FIXTURE PREMISE,
// asserted: the two diffs really do disagree on exactly that line.
func TestQACeilingMessageArmCutHoldsUnderAmendWithCommitVerbose(t *testing.T) {
	w := qaCeilingWall(t, "")
	env := append(append([]string(nil), w.persona...), "GIT_EDITOR="+qaVerboseEditor(t, "wire it"))
	qaVerboseWallIsAwake(t, w, env, "internal/posse/ctl.go")

	const rel = "internal/posse/legacy.go"
	w.plant(t, w.priv, rel, "package posse\n\n// "+qaCeilingHit+"\n\nvar A = 1\n") // HEAD^1: the class
	w.plant(t, w.priv, rel, "package posse\n\nvar A = 1\n")                        // HEAD: its removal

	if out, err := w.git(w.priv, nil, "config", "commit.verbose", "false"); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	w.stage(t, w.priv, rel, "package posse\n\nvar A = 2\n")
	if out, err := w.git(w.priv, env, "commit", "--amend", "--", rel); err != nil {
		t.Fatalf("control: the amend with commit.verbose off must land: %v\n%s", err, out)
	}

	if out, err := w.git(w.priv, nil, "config", "commit.verbose", "true"); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	w.stage(t, w.priv, rel, "package posse\n\nvar A = 3\n")
	one, err := w.git(w.priv, nil, "diff", "--cached", "HEAD^1", "--", rel)
	if err != nil || !strings.Contains(one, "-// "+qaCeilingHit) {
		t.Fatalf("fixture premise: the diff against HEAD^1 must carry the class's removal (%v):\n%s", err, one)
	}
	zero, err := w.git(w.priv, nil, "diff", "--cached", "HEAD", "--", rel)
	if err != nil || strings.Contains(zero, qaCeilingHit) {
		t.Fatalf("fixture premise: the diff against HEAD must NOT carry the class (%v):\n%s", err, zero)
	}
	out, err := w.git(w.priv, env, "commit", "--amend", "--", rel)
	if err == nil {
		return
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") {
		t.Fatalf("fixture premise: the probe must be refused by the MESSAGE arm if it is refused at all:\n%s", out)
	}
	t.Errorf("under --amend with commit.verbose the reference was diffed against HEAD where git's diff "+
		"below its cut is against HEAD^1: the removal git wrote there, and will throw away, was read as "+
		"message and refused the amend:\n%s", out)
}
