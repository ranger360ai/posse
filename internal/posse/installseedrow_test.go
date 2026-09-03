package posse

// ranger-base-1vpf. INSTALL.md §14's first Troubleshooting row hands a reader
// a procedure — move the wrong seed tree aside and re-run `posse init` — and
// §14 is where somebody goes after the happy path already failed them. The
// re-run adds files to recipes/ and skills/, both of which are in
// PromotedPaths, so a procedure that fills the gap without re-stamping ends
// at exit 0 on a home every DISPATCHED launch refuses (ADR 0015 §3). The
// re-stamp landed with ranger-base-e6y; this pins the row's own procedure,
// verbatim, so the doc and the code cannot drift apart again.
//
// The half the re-stamp deliberately does NOT cover is a PROMOTED manifest —
// there the manifest is a claim about a commit and only `posse promote` may
// restate it (ranger-base-pith). The row has to say so, because init says
// nothing at all in that case; that is the second test here.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// foreignSeedDir is the row's stated cause: a real seed tree that is not the
// embed, sitting one level above the binary and winning over it.
func foreignSeedDir(t *testing.T) string {
	t.Helper()
	dir := writeSeedDir(t, filepath.Join(t.TempDir(), "examples"))
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("stale: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		"agents/architect.md":  "---\nname: architect\n---\nstale\n",
		"recipes/scratch.yaml": "purpose: stale\n",
		"envs/default.env":     "X=1\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The row, run to the letter: init from the wrong seed, move it aside, init
// again. EXPECTED is the row's own promise — a reader who follows it ends on
// a home a dispatched launch will accept.
func TestInstallSeedingRowLeavesAHomeADispatchWillLaunchOn(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var out strings.Builder
	if err := a.initFrom(&out, os.DirFS(foreignSeedDir(t)), "stale"); err != nil {
		t.Fatalf("the wrong seed: %v\n%s", err, out.String())
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("the fixture does not start verifiable: %+v", v)
	}

	// THE ROW'S FIX: the directory is aside, so init reaches the embed.
	out.Reset()
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("the re-run: %v\n%s", err, out.String())
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("INSTALL.md §14's seeding row ends on a home every dispatched launch refuses: %s\ninit said:\n%s", v.Line(), out.String())
	}
	// The row tells the reader to read this line back; a silent re-stamp is
	// one they cannot check.
	if s := out.String(); !strings.Contains(s, "re-stamped") {
		t.Errorf("the re-run repaired the manifest and did not say so:\n%s", s)
	}
	// And the line the row QUOTES is the line init prints. A doc that quotes
	// output is a doc that goes stale silently, so the quote is checked
	// against the real thing rather than against a copy of itself.
	body, err := os.ReadFile("../../INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	quoted := "filled <n> missing seed file(s) and re-stamped"
	if !strings.Contains(seedingRow(t, string(body)), quoted) {
		t.Fatalf("the row no longer quotes init's re-stamp line; re-point this check at what it quotes now")
	}
	for _, half := range strings.Split(quoted, "<n>") {
		if !strings.Contains(out.String(), half) {
			t.Errorf("INSTALL.md tells the reader to look for %q and init does not print it:\n%s", half, out.String())
		}
	}
}

// A promoted home takes no re-stamp from init — by design — and since
// ranger-base-39jnl it takes no COPY from init either: the run refuses
// before the first write. The row still has to name it, because "init
// refused" is a symptom a reader following §14 has to be able to place.
func TestInstallSeedingRowNamesPromoteForAPromotedHome(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var out strings.Builder
	if err := a.initFrom(&out, os.DirFS(foreignSeedDir(t)), "stale"); err != nil {
		t.Fatalf("the wrong seed: %v\n%s", err, out.String())
	}
	// What `posse promote` leaves behind: a manifest that is not `seeded`.
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("the fixture must be armed: %+v %v", m, err)
	}
	m.Seeded = false
	if m.Files, err = HashPromotedSet(a.Home); err != nil {
		t.Fatal(err)
	}
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	err = a.initFrom(&out, posse.Seed, "embedded")
	if err == nil {
		t.Fatalf("the re-run on a promoted home returned nil — it must refuse (ranger-base-39jnl):\n%s", out.String())
	}
	// The refusal has to say WHICH kind of home this is and where the write
	// belongs, or the reader following the row cannot get from the message
	// to the fix.
	for _, want := range []string{"promoted constitution", "posse promote"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("init's refusal does not name %q:\n%v", want, err)
		}
	}
	// And it refused BEFORE writing: the seed's files are what the launch
	// verify would have reported as `unpromoted` for the rest of this home's
	// life, and the whole point of refusing is that they never land.
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("the refused init still moved the home off its manifest: %s", v.Line())
	}
	body, err := os.ReadFile("../../INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	row := seedingRow(t, string(body))
	for _, want := range []string{"posse promote", "carries a promoted constitution"} {
		if !strings.Contains(row, want) {
			t.Errorf("INSTALL.md §14's seeding row does not name %q, so a reader who hits init's refusal cannot place it:\n%s", want, row)
		}
	}
}

// ranger-base-g4cm: the third outcome the row has to name. A seed tree that
// misses NOTHING the embed carries (fullForeignSeedDir, shared with the QA
// pin in installseedsilence_qa_test.go) leaves the re-run's `wrote` at 0 on a
// SEEDED home — the same silence init used to give a PROMOTED home, which is
// what made the row's old inference (silence ⇒ promoted) unsound. init now
// names this case; the row must too, and by the sentence init actually
// prints, not by a position ("the re-run's second line") that
// retireExamplePIDs can push around.
func TestInstallSeedingRowNamesNothingMissingForASeededHomeWithNoGap(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var out strings.Builder
	if err := a.initFrom(&out, os.DirFS(fullForeignSeedDir(t)), "stale"); err != nil {
		t.Fatalf("the wrong seed: %v\n%s", err, out.String())
	}
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil || !m.Seeded {
		t.Fatalf("the fixture must be a seeded home, or this measures nothing: %+v %v", m, err)
	}

	out.Reset()
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("the re-run: %v\n%s", err, out.String())
	}
	quoted := "nothing missing:"
	if !strings.Contains(out.String(), quoted) {
		t.Fatalf("a seeded home whose re-run fills no gap must say so by name, not by silence:\n%s", out.String())
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("a seeded home the re-run filled no gap in must still verify: %s\n%s", v.Line(), out.String())
	}

	body, err := os.ReadFile("../../INSTALL.md")
	if err != nil {
		t.Fatal(err)
	}
	row := seedingRow(t, string(body))
	if !strings.Contains(row, quoted) {
		t.Errorf("INSTALL.md §14's seeding row no longer quotes %q, the line that disambiguates this case from a promoted home:\n%s", quoted, row)
	}
}

// seedingRow is §14's first Troubleshooting row — the one the reader lands on
// when init announces the wrong seed.
func seedingRow(t *testing.T, doc string) string {
	t.Helper()
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "| `posse init` prints `(seed: <dir>/examples)`") {
			return line
		}
	}
	t.Fatal("INSTALL.md no longer has a §14 row for the wrong-seed symptom")
	return ""
}
