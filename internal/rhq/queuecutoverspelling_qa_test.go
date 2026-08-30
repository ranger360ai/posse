package rhq

// QA pin for ranger-base-e8hp, filed as a bug against scripts/queue-cutover.sh.
//
// WHAT ranger-base-l9aa FIXED. The cutover fan-out used to rewrite a LIST of
// trees, so a checkout that already redirected at the constitution and was on
// nobody's list kept that redirect after the store moved and became hop one of
// a two-hop chain — which bd 0.49.1 refuses, silently in the arm that matters.
// The fix walks the constitution's parent and rewrites what it finds.
//
// WHAT IT STILL MISSES. The scan decides a tree is "pointed at the
// constitution" with an EXACT string compare of the redirect's first line
// against $SRC_BEADS:
//
//	cur=$(head -n 1 "$b/redirect" ...); [ "$cur" = "$SRC_BEADS" ] || continue
//
// bd is looser than that. MEASURED against bd 0.49.1 (2026-08-30), every one of
// these spellings of the SAME directory is a live redirect bd follows to the
// canonical store — trailing slash, trailing space, leading space, a doubled
// slash, a `/./` segment, a `/../` segment, and a CRLF line ending. The exact
// compare sees none of them. So a tree spelled any of those ways is left
// behind by the fan-out and becomes exactly the two-hop chain l9aa was filed
// for.
//
// This is not hypothetical bookkeeping: the originating instance's own
// redirect — the tree l9aa was about — was repointed BY HAND, out of band,
// with no bead recording it (41 bytes, no trailing newline, where the script
// writes 42). Hand-edited redirects are how this fleet actually gets them,
// and a hand does not spell paths the way a script does.
//
// The fix wants a compare that normalises before matching (or resolves both
// sides to a real path) rather than a looser one: a stranger tree pointed at a
// DIFFERENT store must still be left alone, which is what
// TestQueueCutoverFindsTheTreesTheListForgets already holds from the other
// side.
//
// The setup witness below holds on BOTH sides of the fix — it asserts that the
// sloppy spelling is a redirect bd resolves, which is true before and after —
// so un-skipping this after the fix measures the fix and not the fixture
// (ranger-base-e8hp).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qcSloppySpellings are spellings of one directory that bd 0.49.1 follows and
// the cutover scan's exact compare does not recognise. Each is a suffix/shape
// transform applied to the canonical `<constitution>/.beads`.
var qcSloppySpellings = []struct {
	name  string
	spell func(store string) string
}{
	{"trailing slash", func(s string) string { return s + "/" }},
	{"trailing space", func(s string) string { return s + " " }},
	{"leading space", func(s string) string { return " " + s }},
	{"doubled slash", func(s string) string { return filepath.Dir(s) + "//" + filepath.Base(s) }},
	{"dot segment", func(s string) string { return filepath.Dir(s) + "/./" + filepath.Base(s) }},
	{"parent segment", func(s string) string {
		d := filepath.Dir(s)
		return d + "/../" + filepath.Base(d) + "/" + filepath.Base(s)
	}},
	{"CRLF line ending", func(s string) string { return s + "\r" }},
}

// A forgotten tree whose redirect names the constitution in any spelling bd
// accepts must be brought along by the fan-out. Today only the byte-exact
// spelling is, so this is the repro.
func TestQueueCutoverFindsAForgottenTreeWhateverTheSpelling(t *testing.T) {
	t.Skip("ranger-base-e8hp: the fan-out's exact compare misses every redirect spelling bd accepts but does not write itself")

	for _, sp := range qcSloppySpellings {
		t.Run(sp.name, func(t *testing.T) {
			constitution, _ := qcConstitution(t)
			store := filepath.Join(constitution, ".beads")
			queue := filepath.Join(t.TempDir(), "queue")
			root := filepath.Dir(constitution)

			// The forgotten tree, spelled sloppily. Beside the constitution,
			// so it is inside the derived scan root.
			forgotten := filepath.Join(root, "retired-checkout-"+strings.ReplaceAll(sp.name, " ", "-"))
			if err := os.MkdirAll(filepath.Join(forgotten, ".beads"), 0o755); err != nil {
				t.Fatal(err)
			}
			spelled := sp.spell(store)
			if err := os.WriteFile(filepath.Join(forgotten, ".beads", beadsRedirect),
				[]byte(spelled+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(forgotten) })

			// SETUP WITNESS, and it holds on both sides of the fix: this
			// spelling really does name the constitution's store. Without it
			// a green could come from a spelling that points nowhere, which
			// would be a tree correctly left alone rather than a bug.
			if filepath.Clean(strings.TrimSpace(strings.TrimRight(spelled, "\r"))) != filepath.Clean(store) {
				t.Fatalf("fixture does not name the store: %q cleans to %q, want %q",
					spelled, filepath.Clean(strings.TrimSpace(spelled)), filepath.Clean(store))
			}

			project := qcWork(t, t.TempDir(), store)
			out, err := qcRun(t, constitution, queue, t.TempDir(), []string{project})
			if err != nil {
				t.Fatalf("queue-cutover.sh: %v\n%s", err, out)
			}

			dst := filepath.Join(queue, ".beads")
			got := qcRedirect(t, forgotten)
			if filepath.Clean(strings.TrimSpace(got)) != filepath.Clean(dst) {
				t.Errorf("a forgotten tree spelled %q still redirects to %q, want %q\n"+
					"that is hop one of the two-hop chain ranger-base-l9aa was filed for\n%s",
					spelled, got, dst, out)
			}
		})
	}
}
