package rhq

// ADR 0015 verification item 5, which the ADR itself marked "unverified
// until built: needs the §3 profile change removing the legacy hardcoded
// state grant". This file is that verification (ranger-base-cpyb).
//
// The claim being pinned is structural, not a carve-out: after §2 the home
// is a real directory holding the promoted constitution beside `state/`,
// and what keeps a promoted copy in force is that no caged session can
// write it. So the test is not "agents/ is denied" — the profile denies
// file-write* by default and grants a list. The test is that the LIST does
// not reach the constitution area, in either direction of containment, and
// that the two things which must stay writable still are.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sbRoot is where every fixture in this file lives, and the reason it is a
// helper rather than a t.TempDir() call is a failure these tests had on
// their first run: the profile carries three unconditional temp grants
// (TMPDIR, /tmp, /private/tmp), and a fixture home under any of them is
// writable for a reason that has nothing to do with what is being tested.
// The real home is ~/.config/posse and the real constitution repo is
// ~/src/…, neither under a temp root, so the honest fixture is one outside
// them too: TMPDIR is pointed at a SIBLING of the root, which holds none of
// it. Filtering the blanket grants out of the set afterwards would have
// been the same thing said less carefully.
func sbRoot(t *testing.T) string {
	t.Helper()
	wtqaHome(t)         // HOME elsewhere too — .claude, .cache and friends
	tmp := t.TempDir()  // what TMPDIR will name
	root := t.TempDir() // a sibling of it: granted by nothing
	if underDir("/tmp", root) {
		t.Skip("TMPDIR resolves under /tmp; the profile's blanket temp grant would cover the fixture")
	}
	t.Setenv("TMPDIR", tmp)
	return root
}

// sbCovers reports whether a writable set reaches p — the question the
// sandbox asks, not string equality: a grant of the parent covers it.
func sbCovers(w []string, p string) bool {
	for _, g := range w {
		if underDir(g, p) {
			return true
		}
	}
	return false
}

func sbMkdir(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// homeWithConstitution builds the post-§2 home: a real directory with the
// promoted set, the manifest, envs/, state/ and — when personas is given —
// a personas/ SYMLINK into a constitution repo (§5, the one link that
// survives the cutover).
func homeWithConstitution(t *testing.T, a *App, personas string) {
	t.Helper()
	for _, d := range []string{"agents", "recipes", "skills", "envs", "state"} {
		sbMkdir(t, filepath.Join(a.Home, d))
	}
	for _, f := range []string{"config.yaml", PromoteManifestFile, "agents/dinesh.md", "envs/default.env"} {
		if err := os.WriteFile(filepath.Join(a.Home, f), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if personas != "" {
		if err := os.Symlink(personas, a.PersonasDir()); err != nil {
			t.Fatal(err)
		}
	}
}

// Item 5, first clause: with cwd elsewhere, a write to home/agents is
// refused — structurally, because nothing in the writable set reaches it —
// and so is every other file the promotion put in force.
func TestQAConstitutionAreaIsInNoWritableSet(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	personas := sbMkdir(t, filepath.Join(root, "constitution", "rhq", "personas", "dinesh"))
	homeWithConstitution(t, a, filepath.Dir(personas))

	ag := &AgentFile{Name: "dinesh", MemoryDir: filepath.Join(a.PersonasDir(), "dinesh")}
	w := a.SeatbeltWritable(ag, sbMkdir(t, filepath.Join(root, "work")), a.GatesDir("dinesh"))

	if bad := a.ConstitutionGrants(w); len(bad) > 0 {
		t.Errorf("the writable set reaches the constitution %v:\n  %s", bad, strings.Join(w, "\n  "))
	}
	// Spelled out one path at a time as well, because ConstitutionGrants is
	// as much under test as the profile is: a list that had quietly gone
	// empty would satisfy the assertion above and nothing else.
	for _, p := range append(a.HomeConstitutionPaths(), filepath.Join(a.Home, "agents", "dinesh.md"), a.Home) {
		if sbCovers(w, p) {
			t.Errorf("%s must be in no writable set (ADR 0015 §2/§7):\n  %s", p, strings.Join(w, "\n  "))
		}
	}
}

// The other half of the same set: the paths under the home a session MUST
// keep. `state/` is what the legacy hardcoded grant was reaching for and
// getting wrong — it named ~/.config/rhq/state literally, so a fleet on the
// §2 home rendered a profile with no state grant at all, and a second
// RHQ_HOME got a grant into the first one's state instead (rangerhq-qfzr).
func TestQAStateStaysWritableAndIsDerivedFromTheHome(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	gates := a.GatesDir("dinesh")
	cwd := sbMkdir(t, filepath.Join(root, "work"))
	ag := &AgentFile{Name: "dinesh", MemoryDir: filepath.Join(a.PersonasDir(), "dinesh")}
	w := a.SeatbeltWritable(ag, cwd, gates)

	for _, p := range []string{a.StateDir, gates, filepath.Join(a.StateDir, "skills")} {
		if !sbCovers(w, p) {
			t.Errorf("a session cannot write %s:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// Derived, not spelled: a second home in the same process gets its own
	// grant and not the first's — the property a literal path cannot have.
	other := NewAppAt(filepath.Join(root, "home2"))
	wo := other.SeatbeltWritable(ag, cwd, other.GatesDir("dinesh"))
	if sbCovers(wo, a.StateDir) {
		t.Errorf("a profile rendered for %s grants %s's state:\n  %s", other.Home, a.Home, strings.Join(wo, "\n  "))
	}
	if !sbCovers(wo, other.StateDir) {
		t.Errorf("a profile rendered for %s does not grant its own state:\n  %s", other.Home, strings.Join(wo, "\n  "))
	}
	// And the retired spelling is gone for good: nothing renders a grant
	// under the pre-0015 home unless that is where the App is rooted.
	if legacy := filepath.Join(os.Getenv("HOME"), ".config", "rhq", "state"); sbCovers(w, legacy) {
		t.Errorf("the legacy hardcoded grant is back: %s\n  %s", legacy, strings.Join(w, "\n  "))
	}
}

// Item 5, second and third clauses, and the reason §5 needs a test of its
// own: home/personas is a SYMLINK into the constitution repo, so "the
// session's own memory dir" and "another persona's" are one directory tree
// under two spellings. sandbox-exec matches real paths, so the grant has to
// be resolved through the link — otherwise the profile grants a path the
// kernel never sees and denies the one it does.
func TestQAOwnMemoryIsWritableThroughTheSymlinkAndNobodyElsesIs(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	constitution := filepath.Join(root, "constitution")
	personas := filepath.Join(constitution, "rhq", "personas")
	for _, p := range []string{"dinesh", "gilfoyle"} {
		sbMkdir(t, filepath.Join(personas, p))
	}
	homeWithConstitution(t, a, personas)

	ag := &AgentFile{Name: "dinesh", MemoryDir: filepath.Join(a.PersonasDir(), "dinesh")}
	w := a.SeatbeltWritable(ag, sbMkdir(t, filepath.Join(root, "work")), a.GatesDir("dinesh"))

	// Granted under BOTH spellings, because they resolve to one directory.
	for _, p := range []string{
		filepath.Join(a.PersonasDir(), "dinesh", "ORDERS.md"),
		filepath.Join(personas, "dinesh", "ORDERS.md"),
	} {
		if !sbCovers(w, p) {
			t.Errorf("own memory %s is not writable:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// Denied under both, for the same reason.
	for _, p := range []string{
		filepath.Join(a.PersonasDir(), "gilfoyle", "ORDERS.md"),
		filepath.Join(personas, "gilfoyle", "ORDERS.md"),
		personas,
		constitution,
		filepath.Join(constitution, ".git"),
	} {
		if sbCovers(w, p) {
			t.Errorf("%s must not be writable (ADR 0015 §5):\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// The profile the kernel reads must carry the RESOLVED path: a symlink
	// spelling in an SBPL subpath matches nothing.
	if prof := SeatbeltProfile("dinesh", w); !strings.Contains(prof, sbQuote(absResolve(filepath.Join(personas, "dinesh")))) {
		t.Errorf("profile does not grant the resolved memory dir:\n%s", prof)
	}
}

// A PID that names a constitution path in `writable:` is the smaller
// spelling of the same breach, and the one realistic way in — `writable:`
// is the only input to the set an operator edits by hand.
func TestQAConstitutionGrantsCatchesAWritableExtra(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	cwd := sbMkdir(t, filepath.Join(root, "work"))

	ag := &AgentFile{Name: "dinesh", Writable: []string{filepath.Join(a.Home, "agents")}}
	bad := a.ConstitutionGrants(a.SeatbeltWritable(ag, cwd, a.GatesDir("dinesh")))
	if len(bad) != 1 || bad[0] != filepath.Join(a.Home, "agents") {
		t.Errorf("ConstitutionGrants = %v, want just the agents dir", bad)
	}
	// A grant INSIDE the area is the same breach spelled smaller, and
	// containment the other way round is the only thing that sees it.
	ag2 := &AgentFile{Name: "dinesh", Writable: []string{filepath.Join(a.Home, "agents", "dinesh.md")}}
	if bad := a.ConstitutionGrants(a.SeatbeltWritable(ag2, cwd, a.GatesDir("dinesh"))); len(bad) != 1 {
		t.Errorf("a grant inside the promoted set is a breach too: got %v", bad)
	}
}

// Item 6's mechanism, at the half this bead owns: the property has to be
// READABLE, because it replaced a deny-list whose failure mode was that
// nobody could tell by looking whether it was still complete. `posse gates
// <persona>` prints the set and then the verdict over it.
func TestQAGatesPrintsTheConstitutionVerdictOverTheSet(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	cwd := sbMkdir(t, filepath.Join(root, "work"))
	ag := &AgentFile{Name: "dinesh", MemoryDir: filepath.Join(a.PersonasDir(), "dinesh")}

	var b strings.Builder
	if err := a.SeatbeltReport(ag, cwd, &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		"(writable set below):",
		"    w " + AbbrevHome(absResolve(a.StateDir)), // resolved: what the kernel matches
		"in no grant above — ADR 0015 §2/§7",
		"is granted and no other persona's is — ADR 0015 §5",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("gates report missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "GRANT REACHES") {
		t.Errorf("a clean home must not print a breach:\n%s", got)
	}

	// And the breach is loud, in the same place, for the same reader.
	ag.Writable = []string{filepath.Join(a.Home, "recipes")}
	b.Reset()
	if err := a.SeatbeltReport(ag, cwd, &b); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); !strings.Contains(got, "✗ GRANT REACHES THE CONSTITUTION: "+AbbrevHome(filepath.Join(a.Home, "recipes"))) {
		t.Errorf("a grant into the promoted set must be named:\n%s", got)
	}
}
