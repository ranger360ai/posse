package posse

// ADR 0039 D2 (ranger-base-ight8): `runtimes` joins PromotedPaths, so the
// per-key runtime overlay — the file that decides which model a tier
// launches on, read at every launch (ADR 0021) — becomes prose in force on
// the same terms as `config.yaml`: written only by `posse promote`, hashed
// into `promoted.json`, refused to every session's writable set, and walled
// from persona commits in the constitution repo.
//
// The addition is one token in one list, and that is exactly why it needs
// pins here rather than in promote.go: the value of a list every consequence
// is derived from is that nothing else changes, which also means nothing
// else FAILS if the token is dropped again. Each test below is written so
// that removing "runtimes" from PromotedPaths reds it — the wrong arm was
// measured, not assumed (2026-09-01, one run per test with the token
// removed):
//
//	dry-run names the arriving file     — the ratification diff is scoped to
//	                                      promotePathspecs, which omits
//	                                      rhq/runtimes without the token, so
//	                                      the new file is invisible: RED.
//	the verify calls a home file
//	unpromoted                          — HashPromotedSet does not walk
//	                                      runtimes/ without the token, so the
//	                                      hand-placed file is not Added and
//	                                      the verdict is OK: RED.
//	HomeConstitutionPaths / the grants  — the dir is in neither list: RED.
//	the rendered wall names rhq/runtimes — ConstitutionRepoPaths omits it: RED.
//
// The fifth pin (a home with no runtimes/ still verifies) is the opposite
// guard and passes in both arms on purpose: it is what keeps the fence from
// being tightened into "a fresh instance is missing a promoted path".

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// addRuntimeOverlayCommit puts the ADR 0021 overlay into the constitution
// repo the way 55c5581 put it into the live one: `rhq/runtimes/claude.yaml`,
// committed, nothing at the home.
func addRuntimeOverlayCommit(t *testing.T, src string, git func(args ...string) (string, error), body string) {
	t.Helper()
	dir := filepath.Join(src, "runtimes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git("add", "-A"); err != nil {
		t.Fatalf("git add: %s", out)
	}
	if out, err := git("commit", "-qm", "the runtime overlay"); err != nil {
		t.Fatalf("git commit: %s", out)
	}
}

// The ratification read is the operator's whole decision surface, and a
// model dial arriving is the change ADR 0039 exists to make routine. The
// promote before it is what gives the diff a baseline: `--dry-run` on a
// virgin home prints "promoting <sha> whole" and names no path at all, so
// the pin has to be the SECOND promote — which is also the shape every real
// dial bump has.
func TestQAPromoteDryRunNamesTheArrivingRuntimeOverlay(t *testing.T) {
	a, src, git := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})
	addRuntimeOverlayCommit(t, src, git, "model_strong: claude-fable-5-1\n")

	var b bytes.Buffer
	if err := a.CmdPromote(&b, PromoteOpts{Source: src, DryRun: true}); err != nil {
		t.Fatalf("promote --dry-run: %v\n%s", err, b.String())
	}
	out := b.String()
	if !strings.Contains(out, "rhq/runtimes/claude.yaml") {
		t.Errorf("the ratification diff does not name the arriving overlay — "+
			"promotePathspecs is not scoped to runtimes/ (ADR 0039 D2):\n%s", out)
	}
	if !strings.Contains(out, "model_strong: claude-fable-5-1") {
		t.Errorf("the diff does not show the dial being ratified:\n%s", out)
	}
	// A dry run is the read separated from the act: the overlay must still
	// not be at the home. Without this the test above would also pass over a
	// --dry-run that had quietly started writing.
	if _, err := os.Stat(filepath.Join(a.Home, "runtimes", "claude.yaml")); !os.IsNotExist(err) {
		t.Errorf("--dry-run wrote the overlay into the home: %v", err)
	}
}

// And the act itself: the overlay lands, the manifest names it, and the home
// verifies. This is verification item 1's `promoted.json` clause.
func TestQAPromoteCarriesTheRuntimeOverlayIntoTheManifest(t *testing.T) {
	a, src, git := promoteFixture(t)
	addRuntimeOverlayCommit(t, src, git, "model_strong: claude-fable-5-1\n")
	promote(t, a, PromoteOpts{Source: src})

	body, err := os.ReadFile(filepath.Join(a.Home, "runtimes", "claude.yaml"))
	if err != nil {
		t.Fatalf("the overlay was not promoted: %v", err)
	}
	if !strings.Contains(string(body), "claude-fable-5-1") {
		t.Errorf("promoted overlay reads %q", body)
	}
	m := mustManifest(t, a)
	if _, ok := m.Files["runtimes/claude.yaml"]; !ok {
		t.Errorf("the manifest does not name the overlay — the launch verify attests to nothing: %v", m.Files)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("the home promote just wrote does not verify: %s", v.Line())
	}
}

// The fence doing its job, and the reason the CHANGELOG's Upgrading note
// exists: a home whose manifest predates D2 carries a hand-placed overlay
// that no manifest entry names, so the first launch on the new binary reads
// `unpromoted runtimes/claude.yaml` and every dispatched session refuses
// until the operator promotes.
func TestQAVerifyPromotedCallsAnUnpromotedRuntimeOverlayUnpromoted(t *testing.T) {
	a, src, _ := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src}) // a manifest with no runtimes entry
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("the fixture home does not verify before the hand edit: %s", v.Line())
	}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "claude.yaml"),
		[]byte("model_strong: claude-fable-5-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := a.VerifyPromoted()
	if v.OK() {
		t.Fatalf("a hand-placed overlay the manifest does not name verifies clean — "+
			"the dial is again the one launch-read fact nothing attests to: %s", v.Line())
	}
	if !containsString(v.Added, "runtimes/claude.yaml") {
		t.Errorf("Added = %v, want runtimes/claude.yaml (changed=%v missing=%v)", v.Added, v.Changed, v.Missing)
	}
	if got := v.Line(); !strings.Contains(got, "unpromoted runtimes/claude.yaml") {
		t.Errorf("the refusal an operator reads in a dispatch log is %q, and ADR 0039 D2 predicts "+
			"`unpromoted runtimes/claude.yaml`", got)
	}
	// And the promote that clears it is the second half of the ritual the
	// CHANGELOG states — committed prose, not the file sitting at the home.
	if v := a.VerifyPromoted(); v.OK() {
		t.Error("the verdict changed on a second read with nothing between them")
	}
}

// The other direction, and the ADR's own words: the seed tree has no
// runtimes/, a home without the dir hashes nothing for it, so a fresh
// instance is not marked missing. Adding a promoted path must not turn every
// instance that has not got one into a refused launch.
func TestQAAHomeWithNoRuntimesDirStillVerifies(t *testing.T) {
	a, src, _ := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})
	if _, err := os.Stat(a.RuntimesDir()); !os.IsNotExist(err) {
		t.Fatalf("the fixture home has a runtimes dir, so this measures nothing: %v", err)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("a home with no runtimes/ must verify — absence is not a mismatch: %s", v.Line())
	}
	if m := mustManifest(t, a); len(m.Files) != 6 {
		t.Errorf("the manifest names %d files; a source with no runtimes/ still promotes the 6: %v",
			len(m.Files), m.Files)
	}
}

// A home overlay the constitution does not carry LEAVES, loudly — the clause
// the CHANGELOG warns about, and the end of the hand-placed-file era. This is
// promoteRemovals bounded to PromotedPaths, which is why it could not reach
// runtimes/ before D2 and can now.
func TestQAPromoteRemovesAHomeOverlayTheConstitutionDoesNotCarry(t *testing.T) {
	a, src, _ := promoteFixture(t)
	promote(t, a, PromoteOpts{Source: src})
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(a.RuntimesDir(), "bob.yaml")
	if err := os.WriteFile(stray, []byte("command: bob\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := promote(t, a, PromoteOpts{Source: src})
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("a home overlay no commit carries survived a promote: %v", err)
	}
	if !strings.Contains(out, "removed runtimes/bob.yaml") {
		t.Errorf("the removal was silent; ADR 0039 D2 says it prints:\n%s", out)
	}
}

// The seatbelt half. `~/.config/posse/runtimes` must be in no session's
// writable set, and the observable ADR 0039 names is that the dir appears in
// no grant — which requires HomeConstitutionPaths to know about it, since a
// detector that has never heard of a path reports the all-clear over it.
func TestQAHomeConstitutionPathsIncludesTheRuntimesDir(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	if !containsString(a.HomeConstitutionPaths(), a.RuntimesDir()) {
		t.Fatalf("HomeConstitutionPaths does not name %s: %v", a.RuntimesDir(), a.HomeConstitutionPaths())
	}
	// The detector answers on it, in both containment directions — a grant
	// that covers the dir and one that lands inside it are the same breach.
	for _, w := range [][]string{{a.RuntimesDir()}, {filepath.Join(a.RuntimesDir(), "claude.yaml")}, {a.Home}} {
		if bad := a.ConstitutionGrants(w); !containsString(bad, a.RuntimesDir()) {
			t.Errorf("a writable set %v is not reported as reaching the runtimes dir: %v", w, bad)
		}
	}
	// And the profile a real session gets does not reach it.
	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(a.PersonasDir(), "developer")}
	w := a.SeatbeltWritable(ag, sbMkdir(t, filepath.Join(root, "work")), a.GatesDir("developer"))
	if sbCovers(w, a.RuntimesDir()) {
		t.Errorf("%s is writable to a session (ADR 0039 D2):\n  %s", a.RuntimesDir(), strings.Join(w, "\n  "))
	}
}

// The commit-wall half, spelled as a LITERAL rather than derived from
// PromotedPaths. The generic render pin next door iterates the promoted set,
// so it goes green the moment the token is dropped — the whole class shrinks
// and takes the pin's own case list with it (the failure mode
// constitutionwall_qa_test.go's preamble records). This one names the string
// ADR 0039 D2 decides on, end to end from PromotedPaths through
// ConstitutionRepoPaths into the rendered sh.
func TestQAConstitutionWallNamesRhqRuntimes(t *testing.T) {
	const want = "rhq/runtimes"
	if !containsString(ConstitutionRepoPaths(), want) {
		t.Errorf("ConstitutionRepoPaths() = %v, want it to name %s (ADR 0039 D2)", ConstitutionRepoPaths(), want)
	}
	if got := InConstitutionClass(ConstitutionClassIn(constitutionRepoTop(t)), "rhq/runtimes/claude.yaml"); got != want {
		t.Errorf("a persona commit of rhq/runtimes/claude.yaml falls under class member %q, want %q", got, want)
	}
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if !strings.Contains(render, "\n"+want+"\n") {
		t.Errorf("the rendered commit wall does not name %s as a class member", want)
	}
}

// constitutionRepoTop is a tree that IS the constitution repo as far as
// ConstitutionClassIn is concerned: a top level carrying the marker.
func constitutionRepoTop(t *testing.T) string {
	t.Helper()
	top := t.TempDir()
	if err := os.MkdirAll(filepath.Join(top, filepath.FromSlash(ConstitutionRepoMarker)), 0o755); err != nil {
		t.Fatal(err)
	}
	return top
}
