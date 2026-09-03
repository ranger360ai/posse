package posse

// THE EDGES OF CHECK 3'S TWO ARMS FOR AN INSTANCE PATTERN (ADR 0048 D2,
// ranger-base-uzgkz, added verifying it under ranger-base-qjqa3).
//
// instancepatternscope_qa_test.go pins the mechanism: a NEW .go file's added
// line, a NEW path, the marker exception, the shipped list staying markdown-
// only, and the empty-identity render. The close's claim is wider than those
// fixtures — "the ADDED lines of EVERY staged text file and EVERY added
// path" — and this file is the rest of that claim, measured rather than
// read: a file already in history, four other text types, a removal, a
// binary in the same commit, and three path shapes (a directory component, a
// space, a glob character).
//
// EVERY PIN HERE IS MUTATION-CHECKED against the two mutants that can tell
// the arms apart, and each names the ARM that must do the refusing — a pin
// that only asserts "the commit failed" is satisfied by check 1's docs-genre
// allowlist, which is what the first draft of the path pin below actually
// measured:
//   - M1, the pre-0048 scope: drop both `len(extra) > 0` sections from
//     identityGuardCheck. Reds every pin here except the REMOVAL pin, which
//     is green by construction under either arm.
//   - M2, the path section alone: drop instancePath's render and its
//     posse_cbad init. Reds the PATH pin ALONE and leaves the four content
//     pins green — which is what says the two arms are pinned separately.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P1: an added line in a file that is ALREADY IN HISTORY. Pin (a) stages a
// new file, where the whole body is added lines; a modified file is the
// commoner shape and a different diff.
func TestQAInstancePatternEdgeRefusesAnAddedLineInAModifiedFile(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = "internal/posse/existing.go"
	w.plant(t, w.pub, rel, "package posse\n\n// nothing to see.\n")
	w.stage(t, w.pub, rel, "package posse\n\n// nothing to see.\n// the "+qaInstanceName+" harness shipped this.\n")
	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Errorf("MODIFIED FILE: an added line in a tracked .go file must be refused:\n%s", out)
	}
}

// P2: a staged text file that is neither markdown nor Go — the claim is
// "every staged text file".
func TestQAInstancePatternEdgeRefusesOtherTextTypes(t *testing.T) {
	for _, rel := range []string{"data/thing.json", "etc/thing.yaml", "notes.txt", "Makefile", "scripts/run.sh"} {
		t.Run(rel, func(t *testing.T) {
			w := qaInstanceWall(t)
			w.stage(t, w.pub, rel, "a line about "+qaInstanceName+" here\n")
			out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
			if err == nil {
				t.Errorf("%s: an added line in a staged text file must be refused:\n%s", rel, out)
			}
		})
	}
}

// P3: REMOVING the name is the fix, not the leak. A deletion-only diff must
// commit — a wall that refuses the cleanup is a wall nobody can get past.
func TestQAInstancePatternEdgeTakesARemoval(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = "internal/posse/dirty.go"
	w.plant(t, w.pub, rel, "package posse\n\n// the "+qaInstanceName+" harness shipped this.\n")
	w.stage(t, w.pub, rel, "package posse\n\n// redacted.\n")
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel); err != nil {
		t.Errorf("REMOVAL: deleting the name must commit, else the fix is unlandable: %v\n%s", err, out)
	}
}

// P4: a staged BINARY file must not break the scan. The render's comment
// says git emits "Binary files ... differ" and never a '+' line; a text file
// carrying a hit in the SAME commit must still be refused BY THE INSTANCE
// ARM — asserted by name, because any other check refusing it would leave
// this pin green over a hole.
func TestQAInstancePatternEdgeSurvivesABinaryInTheSameCommit(t *testing.T) {
	w := qaInstanceWall(t)
	writeBin := func(root string) {
		p := filepath.Join(root, "assets", "blob.bin")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte{0x00, 0x01, 0x02, 'z', 'e', 'p', 'h', 'y', 'r', 0x00, 0xff}, 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := w.git(root, nil, "add", "--", "assets/blob.bin"); err != nil {
			t.Fatalf("add binary: %v %s", err, out)
		}
	}
	writeBin(w.pub)
	// A binary alone: the name is in its BYTES, and a binary has no added
	// lines, so it commits. (Its PATH does not carry the name.)
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", "assets/blob.bin"); err != nil {
		t.Errorf("BINARY ALONE: a binary blob has no added lines and must commit: %v\n%s", err, out)
	}

	// Binary + a text hit in one commit: the text hit must still be caught,
	// and caught by the INSTANCE arm.
	w2 := qaInstanceWall(t)
	p2 := filepath.Join(w2.pub, "assets", "blob2.bin")
	if err := os.MkdirAll(filepath.Dir(p2), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte{0x00, 0x01, 0xff, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := w2.git(w2.pub, nil, "add", "--", "assets/blob2.bin"); err != nil {
		t.Fatalf("add binary: %v %s", err, out)
	}
	w2.stage(t, w2.pub, "notes.txt", "a line about "+qaInstanceName+" here\n")
	out, err := w2.git(w2.pub, w2.persona, "commit", "-m", "x")
	if err == nil {
		t.Errorf("BINARY + TEXT: the text hit must still be refused:\n%s", out)
	} else if !strings.Contains(out, "an instance-defined visibility class in a staged file") {
		t.Errorf("BINARY + TEXT: refused, but not by the instance content arm:\n%s", out)
	}
}

// P5: the path arm over a DIRECTORY component, a path with a SPACE and a
// path with a GLOB character. Deliberately OUTSIDE docs/, because check 1's
// genre allowlist refuses any new docs/ file on its own and would keep this
// pin green over a dead path arm (measured: it did, on the first draft).
func TestQAInstancePatternEdgePathArmEdges(t *testing.T) {
	for _, rel := range []string{
		"internal/" + qaInstanceName + "/readme.md",
		"internal/notes about " + qaInstanceName + ".txt",
		"internal/glob[1]-" + qaInstanceName + ".txt",
	} {
		t.Run(rel, func(t *testing.T) {
			w := qaInstanceWall(t)
			w.stage(t, w.pub, rel, "spotless prose.\n")
			out, err := w.git(w.pub, w.persona, "commit", "-m", "x")
			if err == nil {
				t.Errorf("PATH ARM: %q carries the class in its path and must be refused:\n%s", rel, out)
			} else if !strings.Contains(out, "an instance-defined visibility class in a staged PATH") {
				t.Errorf("PATH ARM: %q was refused, but not by the instance PATH arm:\n%s", rel, out)
			}
		})
	}
}
