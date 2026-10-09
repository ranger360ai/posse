//go:build !posse_arm2 && !posse_arm3

package posse

// What `posse runtime filing` has to be true of, and it is one thing:
// the operator ends up holding ADR 0060 D2's material with no checkout
// anywhere near him (ranger-base-mhv7j, github issue #4).
//
// So every assertion here reads the EMBEDDED tree as the source of truth and
// never the working one. A test that compared the verb's output against
// `etc/herdr/agent-detection/upstream-bob.md` on disk would pass on this box
// for the same reason the defect passed on this box, which is the whole
// lesson of the bead.

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// filingRT is the built-in that carries a filing, found by DECLARATION — the
// test must not name bob, or it stops covering the second runtime that meets
// this and starts asserting today's roster instead of the property.
//
// A COPY, never `&builtinRuntimes[i]`: these tests run in parallel, and a
// pointer into the shared table is a mutable alias on it whatever this file
// does with it (cmd/testparallel reads one as a write, correctly). One of
// them edits the declaration to check a refusal, and it edits its own copy.
func filingRT(t *testing.T) Runtime {
	t.Helper()
	for i := range builtinRuntimes {
		if builtinRuntimes[i].DetectionFiling != "" {
			return builtinRuntimes[i]
		}
	}
	t.Skip("no built-in declares a DetectionFiling — nothing for this verb to hand over")
	return Runtime{}
}

// filingNoneRT is the other half of the roster: a built-in posse carries no
// filing for. Also a copy, and also found by declaration.
func filingNoneRT() (Runtime, bool) {
	for i := range builtinRuntimes {
		if builtinRuntimes[i].DetectionFiling == "" {
			return builtinRuntimes[i], true
		}
	}
	return Runtime{}, false
}

func TestRuntimeFilingPrintsTheCarriedNoteAlone(t *testing.T) {
	t.Parallel()
	rt := filingRT(t)
	// A zero App on purpose: this verb reads nothing from the instance, so
	// it answers on a box whose `posse init` has not run — which is the box
	// a brew install starts out as.
	var out, note bytes.Buffer
	if err := (&App{}).CmdRuntimeFiling(&rt, "", &out, &note); err != nil {
		t.Fatalf("CmdRuntimeFiling: %v", err)
	}
	rel := strings.TrimPrefix(rt.DetectionFiling, FilingRoot)
	want, err := fs.ReadFile(posseFilings(), rel)
	if err != nil {
		t.Fatalf("the binary does not carry %s: %v", rt.DetectionFiling, err)
	}
	// Byte-identical and nothing else: stdout is what gets sent, so a
	// banner, a trailing hint or a stripped final newline would all be
	// something the operator has to edit out of the filing first.
	if !bytes.Equal(out.Bytes(), want) {
		t.Errorf("stdout is not the filing verbatim (%d bytes vs %d) — stdout is what gets sent, so anything of ours in it is something the operator has to delete", out.Len(), len(want))
	}
	// And the attachments are NAMED, because prose down a pipe cannot carry
	// a .txt screen capture and the ask is not actionable without them.
	for _, want := range []string{"posse runtime filing " + rt.Name + " --out", rt.DetectionFiling} {
		if !strings.Contains(note.String(), want) {
			t.Errorf("the note stream does not name %q:\n%s", want, note.String())
		}
	}
	if note.Len() == 0 {
		t.Error("the note stream is empty — the attachments are what make the filing actionable")
	}
}

func TestRuntimeFilingOutWritesThePackageAndGuardsTheDirectory(t *testing.T) {
	t.Parallel()
	rt := filingRT(t)
	dir := filepath.Join(t.TempDir(), "filing")
	var out, note bytes.Buffer
	if err := (&App{}).CmdRuntimeFiling(&rt, dir, &out, &note); err != nil {
		t.Fatalf("CmdRuntimeFiling --out: %v", err)
	}
	filing, attach, err := FilingFor(&rt)
	if err != nil {
		t.Fatalf("FilingFor: %v", err)
	}
	// Every file of the package, by content, against the embedded copy.
	var want []string
	for _, p := range append([]string{filing}, attach...) {
		want = append(want, path.Base(p))
		src, err := fs.ReadFile(posseFilings(), p)
		if err != nil {
			t.Fatalf("read embedded %s: %v", p, err)
		}
		got, err := os.ReadFile(filepath.Join(dir, path.Base(p)))
		if err != nil {
			t.Errorf("--out did not write %s: %v", path.Base(p), err)
			continue
		}
		if !bytes.Equal(got, src) {
			t.Errorf("%s on disk is not the copy this binary carries", path.Base(p))
		}
	}
	sort.Strings(want)
	var got []string
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		got = append(got, e.Name())
	}
	sort.Strings(got)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("--out wrote %v, the package is %v", got, want)
	}
	if len(attach) == 0 {
		t.Errorf("%s's filing came with no attachments — ADR 0060 D2's draft manifest and fixtures are what make the ask actionable", rt.Name)
	}

	// Re-running over its own output is what a second attempt is, so it is
	// allowed — the same rule release-artifacts.sh's --out guard carries.
	if err := (&App{}).CmdRuntimeFiling(&rt, dir, &out, &note); err != nil {
		t.Errorf("a re-run over this verb's own output was refused: %v", err)
	}
	// Anything else in there is somebody's work and this verb does not know
	// what it is. No --force, deliberately.
	keep := filepath.Join(dir, "my-cover-letter.txt")
	if err := os.WriteFile(keep, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = (&App{}).CmdRuntimeFiling(&rt, dir, &out, &note)
	if err == nil {
		t.Fatal("--out overwrote a directory holding a file this verb did not write")
	}
	if !strings.Contains(err.Error(), "my-cover-letter.txt") {
		t.Errorf("the refusal does not name the stray file, so the operator cannot tell what stopped it: %v", err)
	}
	if b, rerr := os.ReadFile(keep); rerr != nil || string(b) != "mine\n" {
		t.Errorf("the refused run touched the stray file anyway: %q %v", b, rerr)
	}
}

func TestRuntimeFilingRefusesWhatItCannotHandOver(t *testing.T) {
	t.Parallel()
	// A built-in posse carries no filing for: the answer is the door, not an
	// empty file. Three of the four built-ins are this case and always were.
	none, ok := filingNoneRT()
	if !ok {
		t.Skip("every built-in declares a filing")
	}
	_, _, err := FilingFor(&none)
	if err == nil {
		t.Fatalf("%s carries no filing and FilingFor answered anyway", none.Name)
	}
	// The door, verbatim from the one function that renders it — so the
	// refusal cannot drift into saying something the four refusal surfaces
	// do not.
	if !strings.Contains(err.Error(), DetectionDoor(&none)) {
		t.Errorf("the refusal does not carry %s's door:\n%v", none.Name, err)
	}

	// A declaration outside the filings root cannot be embedded by any
	// directive, so it is refused by NAME rather than read off disk — the
	// distinction the whole bead turns on.
	stray := filingRT(t)
	stray.DetectionFiling = "docs/runbooks/agent-detection-manifest.md"
	if _, _, err := FilingFor(&stray); err == nil {
		t.Errorf("a DetectionFiling outside %s was accepted — nothing embeds it, so the verb would print whatever happened to be on the box", FilingRoot)
	}
}

// posseFilings is the embedded tree, reached the way production reaches it.
// A one-line indirection so the three tests above read the same FS the verb
// does and cannot be pointed at a working-tree copy by a later edit.
func posseFilings() fs.FS { return posse.Filings }
