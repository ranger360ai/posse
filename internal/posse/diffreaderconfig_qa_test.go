package posse

// TWO PARKED PROBES, both found verifying ranger-base-8djyy (the four-close
// batch h137b / eifaz / 6s00n / o2v6n). Each is a LIVE defect measured
// through the real rendered hook, each is red today, and each is parked so
// the suite stays green until the code lane takes it. Unpark by deleting the
// t.Skip line — nothing else here changes.
//
// Both are about the same thing from opposite ends: WHAT THE WALL IS HANDED
// is not what the wall thinks it is handed.

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
	t.Skip("PARKED: red today — ranger-base-h3s6q finding 1, the code lane's; the fix is --no-textconv on diffReaderShape")
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
	t.Skip("PARKED: red today — ranger-base-h3s6q finding 2, the code lane's")
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
