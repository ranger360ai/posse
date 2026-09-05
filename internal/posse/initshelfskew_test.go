package posse

// ranger-base-788w: the retirement's LAST gate was still judged against the
// running binary's embed.
//
// ranger-base-8ehw moved the live file's identity test onto the table of
// digests posse has shipped (exampledigests.go), so an older release's
// generic is recognised as posse's own. The shelf gate one line down still
// asked `shelf == this binary's example`, and copyIfMissing never rewrites an
// occupied slot — so on every home whose shelf was written by an earlier
// posse the slot holds THAT release's example, reads as operator-edited, and
// stops the retirement. The population is exactly the homes 8ehw was filed to
// rescue: seed with posse N (shelf gets N's examples), upgrade to N+1, and
// the live file now passes while the shelf blocks.
//
// Both halves are pinned here, in one fixture with one thing different: an
// earlier release's shelf must not stop the retirement, and an operator's
// shelf still must. Delete the gate and the second goes red; put `string(b)
// != string(want)` back and the first does.
//
// Self-contained (own helpers, own fixtures): a pin an edit next door can
// carry away pinned nothing.

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ranger360ai/posse"
)

// olderSeed788w is posse.Seed with one extra line in every example PID —
// what a real release looks like from the next one's side (95c4b70 added
// `- Bash(posse promote:*)` to the deny list of all nine).
func olderSeed788w(t *testing.T) fs.FS {
	t.Helper()
	m := fstest.MapFS{}
	err := fs.WalkDir(posse.Seed, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(posse.Seed, p)
		if rerr != nil {
			return rerr
		}
		if strings.HasPrefix(p, "agents/") && strings.HasSuffix(p, ".md") {
			b = append(append([]byte{}, b...), []byte("\n# one line an older posse shipped\n")...)
		}
		m[p] = &fstest.MapFile{Data: b, Mode: 0o644}
		return nil
	})
	if err != nil {
		t.Fatalf("building the older seed: %v", err)
	}
	return m
}

// shipAsARelease788w enters the fixture's examples in posse's record of what
// posse has shipped, the way a real release is entered — by appending a
// digest. The real entries are untouched and the table is restored after, so
// these tests must not run in parallel with anything that reads it.
func shipAsARelease788w(t *testing.T, src fs.FS) {
	t.Helper()
	for _, n := range exampleAgentNames(src) {
		rel := "agents/" + n + ".md"
		b, err := fs.ReadFile(src, rel)
		if err != nil {
			t.Fatal(err)
		}
		was := shippedExampleDigests[rel]
		t.Cleanup(func() { shippedExampleDigests[rel] = was })
		shippedExampleDigests[rel] = append(append([]string{}, was...), sha256Bytes(b))
	}
}

// skewedShelfHome788w is a home an earlier posse left: the shelf holds THAT
// release's examples (it ran a build new enough to write one), and agents/
// holds the same release's generics, which is what every install that
// predates ranger-base-qajs still has in it. src is that binary's seed tree.
//
// The one difference from the ranger-base-8b3h fixture is the whole bead: the
// shelf is left in place rather than removed, because it is the shelf slot
// that blocks here.
func skewedShelfHome788w(t *testing.T, src fs.FS) *App {
	t.Helper()
	// Hermetic against the operator fence (ADR 0031 §2): see initTestApp.
	a := NewAppAt(filepath.Join(t.TempDir(), "home"))
	if err := a.initFrom(io.Discard, src, "older"); err != nil {
		t.Fatalf("seeding the older home: %v", err)
	}
	names := exampleAgentNames(src)
	if len(names) < 9 {
		t.Fatalf("fixture seed ships %d example PID(s), want the nine", len(names))
	}
	for _, n := range names {
		b, err := fs.ReadFile(src, "agents/"+n+".md")
		if err != nil {
			t.Fatal(err)
		}
		// The live generic that older posse seeded...
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		// ...and the shelf slot it wrote, which no later init rewrites.
		shelf := filepath.Join(a.ExampleAgentsDir(), n+".md")
		if got, err := os.ReadFile(shelf); err != nil || string(got) != string(b) {
			t.Fatalf("fixture: the shelf slot for %s is not the older release's copy (%v)", n, err)
		}
	}
	// Re-stamp the seeded manifest over that shape: it is the record the
	// older init left, and the retirement reads `seeded` off it.
	if err := os.Remove(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	return a
}

// assertSkew788w fails the FIXTURE (not the code) unless the bytes it planted
// really are a version posse shipped and really are not this build's — the
// two facts that make the shelf gate the thing under test.
func assertSkew788w(t *testing.T, src fs.FS) {
	t.Helper()
	for _, n := range exampleAgentNames(src) {
		rel := "agents/" + n + ".md"
		older, err := fs.ReadFile(src, rel)
		if err != nil {
			t.Fatal(err)
		}
		if !isShippedExample(rel, older) {
			t.Fatalf("fixture: %s is not entered as a release — the fixture is wrong, not the code", rel)
		}
		cur, err := fs.ReadFile(posse.Seed, rel)
		if err != nil {
			t.Fatal(err)
		}
		if string(older) == string(cur) {
			t.Fatalf("fixture: %s is byte-identical to the current embed — the fixture cannot express version skew", rel)
		}
	}
}

// TestAnEarlierReleasesShelfDoesNotBlockTheRetirement is the bead. Restore
// `shelfErr != nil || string(b) != string(want)` and every one of the nine is
// kept, on the exact population ranger-base-8ehw was filed to rescue.
func TestAnEarlierReleasesShelfDoesNotBlockTheRetirement(t *testing.T) {
	src := olderSeed788w(t)
	shipAsARelease788w(t, src)
	assertSkew788w(t, src)
	a := skewedShelfHome788w(t, src)

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	if live := a.ListAgents(); len(live) != 0 {
		t.Fatalf(`ranger-base-788w REGRESSED — %d generic(s) still routing after an upgrade
whose only obstacle was the shelf: %v. The shelf slot was written by an
earlier posse and copyIfMissing never rewrites an occupied one, so judging it
against the running binary's embed reads every pre-existing home as
operator-edited and blocks the retirement 8ehw opened. The slot's identity
comes from the digests posse has shipped (exampledigests.go), the same answer
the live file above it uses.
init said:
%s`, len(live), live, out.String())
	}
	if strings.Contains(out.String(), "the shelf copy differs") {
		t.Errorf(`init blamed the operator for a shelf slot posse itself wrote:
%s`, out.String())
	}
	if !strings.Contains(out.String(), "retired") {
		t.Errorf("init said %q — a retirement the operator cannot see is a file that vanished", out.String())
	}
	// A move, not a delete, and the shelf keeps what was RETIRED: those bytes
	// exist nowhere else in the home, while this seed's copy is in the binary
	// (ranger-base-xxar — the slot is the one thing init never lays down
	// again, which is why the older bytes are the ones worth the slot).
	for _, n := range exampleAgentNames(src) {
		was, err := fs.ReadFile(src, "agents/"+n+".md")
		if err != nil {
			t.Fatal(err)
		}
		shelf, err := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), n+".md"))
		if err != nil {
			t.Fatalf("%s left the home entirely: %v", n, err)
		}
		if string(shelf) != string(was) {
			t.Errorf("shelf copy of %s is not what was retired — the retired bytes are gone from the home", n)
		}
	}
}

// TestAnEditedShelfStillStopsTheRetirement is the negative control, and the
// pin above needs it to mean anything: same fixture, same run, one slot
// different — the operator wrote it. Nothing moves onto their file, and the
// other eight still retire, so a gate deleted outright fails here and a gate
// widened correctly passes both.
func TestAnEditedShelfStillStopsTheRetirement(t *testing.T) {
	src := olderSeed788w(t)
	shipAsARelease788w(t, src)
	assertSkew788w(t, src)
	a := skewedShelfHome788w(t, src)

	const mine = "qa"
	shelf := filepath.Join(a.ExampleAgentsDir(), mine+".md")
	b, err := os.ReadFile(shelf)
	if err != nil {
		t.Fatal(err)
	}
	edited := string(b) + "\n<!-- the shelf copy is mine -->\n"
	if err := os.WriteFile(shelf, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	if got, err := os.ReadFile(shelf); err != nil || string(got) != edited {
		t.Fatalf("the operator's shelf copy was overwritten: %v", err)
	}
	if live := a.ListAgents(); len(live) != 1 || live[0] != mine {
		t.Errorf("agents/ = %v, want %s.md alone — nothing may move onto a file init did not write, and nothing else may be held back by it", live, mine)
	}
	if !strings.Contains(out.String(), "the shelf copy differs") {
		t.Errorf("init said %q — it must say why it kept the file", out.String())
	}
}
