//go:build !posse_arm2 && !posse_arm3

package posse

// ONE NUL BYTE MAKES A MARKDOWN FILE BINARY, AND EVERY '+' LINE READER IN
// THE HOOK THEN JUDGES NOTHING, SILENTLY (ranger-base-h137b).
//
// `git diff --cached` with no --text prints "Binary files a/x and b/x
// differ" for a file git classifies binary — never a '+' line, never a
// '+++ b/' header. The comment at the head of check 3 stated that exclusion
// as intentional, and for a real blob it is: there is no prose in a PNG to
// scan. The defect is a TEXT file git calls binary. A markdown NOTES or RCA
// file with captured terminal output appended carries a NUL, and then:
//
//	check 0   the .beads/*.jsonl reader
//	check 2   the shipped OpsPatterns over staged markdown
//	check 3   the identity literals and instance patterns over staged files
//	ceiling   ADR 0050's data ceiling, which renders through check 3's
//	          renderer and is the ONLY wall standing in a private repo
//
// all read an empty $posse_added and fall through with no refusal and no
// "judged nothing" line. git's own summary reads "1 file changed, 0
// insertions(+), 0 deletions(-)", which is the tell.
//
// TWO CHANGES, and either alone still fails silently — so there are two
// mutants for every pin here:
//
//	M1  --text off diffReaderShape: git is back to "Binary files ...
//	    differ" and every probe arm below commits clean. The controls stay
//	    green, which is what says these pins measure the binary axis and
//	    not the pattern.
//	M2  --text on but `-a` off the greps: grep collapses a NUL-bearing
//	    stream to "Binary file (standard input) matches", the ops scan
//	    downstream matches nothing, and every probe arm below commits
//	    clean again — with the controls still green. This is the mutant a
//	    one-change fix leaves alive.
//
// SECOND AXIS, same reader, same blanking: the WRITER's own git config.
// core.attributesFile pointing at a file that says `*.md -diff` marks
// markdown binary for a reader that diffReaderShape's four flags do not
// touch (ranger-base-0fz98 finding 2 is the same class). --text closes it
// too, measured; it is a cell here rather than a bead of its own.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaNulBefore is the ordinary move that carries the NUL: captured output
// appended to a markdown file, ahead of the line the wall must judge.
const qaNulBefore = "captured output\x00here\n"

// qaAssertBinary is the FIXTURE PREMISE of every probe arm: git must really
// classify the staged file as binary. A fixture git still diffs as text
// makes the probe green against the unfixed wall and measures nothing.
func qaAssertBinary(t *testing.T, w *visWall, repo, rel string) {
	t.Helper()
	out, err := w.git(repo, nil, "diff", "--cached", "--no-color", "--", rel)
	if err != nil {
		t.Fatalf("git diff --cached: %v %s", err, out)
	}
	if !strings.Contains(out, "Binary files") {
		t.Fatalf("fixture premise: git must classify %s as binary, got:\n%s", rel, out)
	}
}

// qaCommitLanded reports whether the path-limited commit through the real
// hook succeeded, and gives the "0 insertions(+)" tell back to the caller.
func qaCommitLanded(t *testing.T, w *visWall, repo, rel string, extraGit ...string) (string, bool) {
	t.Helper()
	args := append(append([]string(nil), extraGit...), "commit", "-m", "x", "--", rel)
	out, err := w.git(repo, w.persona, args...)
	return out, err == nil
}

// ─── check 2: the shipped OpsPatterns over staged markdown ────────────────

// The bead's own repro, both arms, control first. The ops line is the one
// TestQAMarkdownScanSeesAnOpsLineAddedOnAMove uses; the only difference
// between the arms is the NUL, so a green probe cannot be a green wall.
func TestQABinaryClassifiedMarkdownIsStillScannedByCheck2(t *testing.T) {
	const opsLine = "the pilot cost $715/wk to run.\n"
	prose := strings.Repeat("a line of perfectly public prose\n", 40)

	t.Run("control: plain markdown", func(t *testing.T) {
		w := newVisWall(t)
		w.stage(t, w.pub, "docs/adr/ctl.md", prose+opsLine)
		out, landed := qaCommitLanded(t, w, w.pub, "docs/adr/ctl.md")
		if landed {
			t.Fatalf("the control must be refused by check 2 — without it a green probe measures nothing:\n%s", out)
		}
		if !strings.Contains(out, "ops-class content in staged markdown") || !strings.Contains(out, "$715/wk") {
			t.Errorf("refused, but not by check 2:\n%s", out)
		}
	})

	t.Run("probe: one NUL byte earlier in the file", func(t *testing.T) {
		w := newVisWall(t)
		rel := "docs/adr/nul.md"
		w.stage(t, w.pub, rel, prose+qaNulBefore+opsLine)
		qaAssertBinary(t, w, w.pub, rel)
		out, landed := qaCommitLanded(t, w, w.pub, rel)
		if landed {
			t.Fatalf("a NUL byte must not blank check 2's reader — the ops line is in the public tree:\n%s", out)
		}
		if !strings.Contains(out, "ops-class content in staged markdown") || !strings.Contains(out, "$715/wk") {
			t.Errorf("refused, but not by check 2:\n%s", out)
		}
	})

	// The pattern on the FAR side of the NUL is the half `grep -a` alone
	// buys: with --text and no -a, grep says "Binary file (standard input)
	// matches" and neither side is scanned.
	t.Run("probe: the NUL AFTER the ops line", func(t *testing.T) {
		w := newVisWall(t)
		rel := "docs/adr/nulafter.md"
		w.stage(t, w.pub, rel, prose+opsLine+qaNulBefore)
		qaAssertBinary(t, w, w.pub, rel)
		out, landed := qaCommitLanded(t, w, w.pub, rel)
		if landed {
			t.Fatalf("a NUL after the ops line must not blank check 2's reader either:\n%s", out)
		}
	})

	// The writer's own git config, same blanking, same fix.
	t.Run("probe: core.attributesFile marks *.md -diff", func(t *testing.T) {
		w := newVisWall(t)
		attrs := filepath.Join(w.home, "attrs")
		if err := os.WriteFile(attrs, []byte("*.md -diff\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rel := "docs/adr/attr.md"
		w.stage(t, w.pub, rel, prose+opsLine)
		out, landed := qaCommitLanded(t, w, w.pub, rel, "-c", "core.attributesFile="+attrs)
		if landed {
			t.Fatalf("the writer's own core.attributesFile must not blank check 2's reader:\n%s", out)
		}
		if !strings.Contains(out, "ops-class content in staged markdown") {
			t.Errorf("refused, but not by check 2:\n%s", out)
		}
	})
}

// ─── check 0: the .beads jsonl reader ─────────────────────────────────────

// A bead record is a JSON line, and a bead description carrying pasted
// terminal output carries the NUL with it into the db.
func TestQABinaryClassifiedBeadsDbIsStillScannedByCheck0(t *testing.T) {
	const rel = ".beads/issues.jsonl"
	row := `{"id":"x-1","title":"t","description":"the pilot cost $715/wk to run."}` + "\n"

	t.Run("control", func(t *testing.T) {
		w := newVisWall(t)
		w.stage(t, w.pub, rel, row)
		out, landed := qaCommitLanded(t, w, w.pub, rel)
		if landed {
			t.Fatalf("the control must be refused by check 0:\n%s", out)
		}
		if !strings.Contains(out, "ops-class content in a public repo's beads db") {
			t.Errorf("refused, but not by check 0:\n%s", out)
		}
	})

	t.Run("probe: one NUL byte in the db", func(t *testing.T) {
		w := newVisWall(t)
		w.stage(t, w.pub, rel, `{"id":"x-0","title":"pasted`+"\x00"+`output"}`+"\n"+row)
		qaAssertBinary(t, w, w.pub, rel)
		out, landed := qaCommitLanded(t, w, w.pub, rel)
		if landed {
			t.Fatalf("a NUL byte must not blank check 0's reader:\n%s", out)
		}
		if !strings.Contains(out, "ops-class content in a public repo's beads db") {
			t.Errorf("refused, but not by check 0:\n%s", out)
		}
	})
}

// ─── check 3: the identity literals over every staged file ───────────

func TestQABinaryClassifiedTextIsStillScannedByCheck3(t *testing.T) {
	t.Run("control", func(t *testing.T) {
		w := newVisWall(t)
		rel := "internal/x.go"
		w.stage(t, w.pub, rel, "package x\n\n// owned by "+w.literal(t, "username")+"\n")
		out, landed := qaCommitLanded(t, w, w.pub, rel)
		if landed {
			t.Fatalf("the control must be refused by check 3:\n%s", out)
		}
		if !strings.Contains(out, "an operator identity literal in a staged file") {
			t.Errorf("refused, but not by check 3's content arm:\n%s", out)
		}
	})

	t.Run("probe: one NUL byte earlier in the file", func(t *testing.T) {
		w := newVisWall(t)
		rel := "internal/y.go"
		w.stage(t, w.pub, rel, "package y\n\n// "+qaNulBefore+"// owned by "+w.literal(t, "username")+"\n")
		qaAssertBinary(t, w, w.pub, rel)
		out, landed := qaCommitLanded(t, w, w.pub, rel)
		if landed {
			t.Fatalf("a NUL byte must not blank check 3's reader:\n%s", out)
		}
		if !strings.Contains(out, "an operator identity literal in a staged file") {
			t.Errorf("refused, but not by check 3's content arm:\n%s", out)
		}
	})
}

// ─── the data ceiling: the only wall standing in a private repo ───────────

// ADR 0050's ceiling renders through check 3's renderer and inherits the
// hole exactly (measured on ranger-base-h137b). A private repo runs no visibility
// check at all, so here the blanked reader is the whole wall.
func TestQABinaryClassifiedTextIsStillScannedByTheDataCeiling(t *testing.T) {
	t.Run("control", func(t *testing.T) {
		w := qaCeilingWall(t, "")
		rel := "docs/notes.d/ctl.md"
		w.stage(t, w.priv, rel, "# handoff\n\n"+qaCeilingHit+" — do not forward\n")
		out, landed := qaCommitLanded(t, w, w.priv, rel)
		if landed {
			t.Fatalf("the control must be refused by the ceiling:\n%s", out)
		}
		if !strings.Contains(out, "data-ceiling content in a staged file") {
			t.Errorf("refused, but not by the ceiling:\n%s", out)
		}
		qaNoCeilingVocabulary(t, "the ceiling refusal", out, rel)
	})

	t.Run("probe: one NUL byte earlier in the file", func(t *testing.T) {
		w := qaCeilingWall(t, "")
		rel := "docs/notes.d/nul.md"
		w.stage(t, w.priv, rel, "# handoff\n\n"+qaNulBefore+qaCeilingHit+" — do not forward\n")
		qaAssertBinary(t, w, w.priv, rel)
		out, landed := qaCommitLanded(t, w, w.priv, rel)
		if landed {
			t.Fatalf("a NUL byte must not blank the ceiling's reader — this is the only wall in a private repo:\n%s", out)
		}
		if !strings.Contains(out, "data-ceiling content in a staged file") {
			t.Errorf("refused, but not by the ceiling:\n%s", out)
		}
		qaNoCeilingVocabulary(t, "the ceiling refusal", out, rel)
	})
}

// ─── the shape, asserted on the render ────────────────────────────────────

// Both halves of the fix, in the rendered hook, so a later edit that drops
// one cannot pass the pins above by accident of ordering.
func TestQADiffReadersCarryTextAndBinarySafeGreps(t *testing.T) {
	t.Parallel()
	if !strings.Contains(diffReaderShape, "--text") {
		t.Errorf("diffReaderShape must carry --text: %q", diffReaderShape)
	}
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{
		Ceiling: []OpsPattern{{Class: "c", ERE: "zzz"}},
	})
	for _, line := range strings.Split(render, "\n") {
		if !strings.Contains(line, "grep") || !strings.Contains(line, "'^+") {
			continue
		}
		if !strings.Contains(line, "grep -a") {
			t.Errorf("a '+' line reader that is not binary-safe (grep -a):\n\t%s", line)
		}
	}
	if strings.Contains(render, "Binary files are already excluded") {
		t.Errorf("the rendered hook still claims binary files are excluded — ranger-base-h137b")
	}
}

// AND NO SCOPE CLAIM SAYS "TEXT" ANY MORE (ranger-base-z771z, asked for on
// that bead by the security lane). Until the sweep, the hook's head
// comment, check 2's and check 3's rendered headers, the ceiling's THREE
// ARMS paragraph and three of the refusal RULE strings all scoped a wall to
// "every staged text file" — the reader's mechanism written down as if it
// were the rule, and once --text landed it was a hole the words invented:
// there is no such thing as an unscanned staged file. The hook body is what
// a refused writer reads and what the launcher's L3 probe hashes, so the
// claim is pinned where it is RENDERED and not only in source.
//
// FLATTENED WITH THE COMMENT PREFIX STRIPPED, not scanned per line: in the
// ceiling's head the claim wrapped as "...every staged\ntext file, any
// path..." (gates.go before this sweep), which a per-line grep reads as two
// innocent lines — and shComment renders each of those lines with its own
// "# ", so joining them is not enough either: the marker sits BETWEEN the
// wrapped words. Measured: a flatten that only collapsed whitespace left
// the wrapped mutant green.
//
// NOT "the render contains no 'text file'", which is what the ask said and
// would be wrong: the --text comment legitimately says git "classifies a
// text file carrying one NUL byte as BINARY". That sentence is this pin's
// CONTROL — without it, a hook that rendered no prose at all would satisfy
// the absence assertion above.
//
// MUTATION-CHECKED, one per rendered prose source (runs on
// ranger-base-z771z): M-A restores "every staged text file" in the head
// comment (gates.go commitGuardHead), M-B restores it WRAPPED in the
// ceiling's THREE ARMS paragraph, M-D restores it in a refusal RULE string
// (visibility.go IdentityRule) — each reds the absence arm alone. M-C
// rewrites the --text sentence and reds the control alone.
func TestQARenderedHookMakesNoTextFileScopeClaim(t *testing.T) {
	t.Parallel()
	// Every prose source at once: the head comment, check 2, check 3 with
	// both its sources, and the ceiling with all three arms. A claim in a
	// block this render omits is a claim this pin cannot see.
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{
		Extra:   []OpsPattern{{Class: "e", ERE: "zzz"}},
		Ceiling: []OpsPattern{{Class: "c", ERE: "yyy"}},
	}, IdentityLiteral{Class: "username", Value: "qa-fixture-operator"})

	var prose strings.Builder
	for _, line := range strings.Split(render, "\n") {
		prose.WriteString(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		prose.WriteString(" ")
	}
	flat := strings.Join(strings.Fields(strings.ToLower(prose.String())), " ")
	if i := strings.Index(flat, "staged text file"); i >= 0 {
		t.Errorf("the rendered hook still scopes a wall to TEXT files (ADR 0048 D2 as amended 2026-09-04):\n\t...%s...", flat[max(0, i-80):min(len(flat), i+80)])
	}
	// CONTROL: the --text mechanism prose is still rendered, so the absence
	// above is a swept claim and not an empty hook.
	if !strings.Contains(render, "classifies a text file carrying one NUL byte as BINARY") {
		t.Error("the --text mechanism comment is gone from the render — the assertion above now measures nothing (ranger-base-h137b)")
	}
}
