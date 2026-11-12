package posse

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
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sbRoot is where every fixture in this file lives, and the reason it is a
// helper rather than a t.TempDir() call is a failure these tests had on
// their first run: the profile carries three BLANKET temp grants (TMPDIR,
// /tmp, /private/tmp — seatbeltTempGrants), and a fixture home under any of
// them is writable for a reason that has nothing to do with what is being
// tested. The real home is ~/.config/posse and the real constitution repo
// is ~/src/…, neither under a temp root, so the honest fixture is one
// outside them too: TMPDIR is pointed at a SIBLING of the root, which holds
// none of it.
//
// WHICH IS ONLY HALF THE PREMISE, AND THE HALF THE ENVIRONMENT CAN REFUSE
// (ranger-base-u8xk4). The root comes from t.TempDir(), so where TMPDIR is
// itself under /tmp — unset, which is this box and the CI leg, or set to
// /tmp by hand — the root lands under /tmp and the blanket `/tmp` grant
// covers it after all. This helper used to answer that with `t.Skip`, and
// the cost of the skip was the whole class it guards: MEASURED 2026-11-11
// on this box (linux 7.2.15 x86_64, go1.27.3, TMPDIR unset), **123 tests
// and subtests skipped on that line** — 53 in arm 1, 22 in arm 2, 48 in
// arm 3, across the 14 QA files that call sbRoot. Every sandbox grant pin
// in internal/posse, measuring nothing, in silence, on the only box that
// runs them.
//
// So where the box cannot offer a root outside those grants, the GRANTS
// move instead: sbNoBlanketTempGrants stubs production's own
// seatbeltTempGrants to nil for the test, and every fixture below then
// measures the profile a launch outside a temp root would have had. It is
// the same premise reached the other way round, and it needs nothing of the
// box. On a host where the placement works — darwin, whose TMPDIR is under
// /var/folders — the stub is not installed and the grants are the real
// ones, because that premise is stronger and it is the one a kernel reads.
func sbRoot(t *testing.T) string {
	t.Helper()
	before := os.Getenv("HOME")
	wtqaHome(t) // HOME elsewhere too — .claude, .cache and friends
	sbAssertHomeIsAFixture(t, before)
	tmp := gitTempDir(t)  // what TMPDIR will name
	root := gitTempDir(t) // a sibling of it: granted by nothing it names
	t.Setenv("TMPDIR", tmp)
	sbNoBlanketTempGrants(t, root)
	return root
}

// sbNoBlanketTempGrants takes the profile's blanket temp grants away for the
// duration of this test, and ONLY when the fixture root is inside one of
// them — which is the case sbRoot's placement cannot answer. The reason is
// in seatbeltTempGrants' own comment; this is where the lever is pulled.
//
// A package var written under a t.Cleanup, so it needs the test to be
// sequential. It is: sbRoot calls t.Setenv, which makes `t.Parallel` a
// panic in every test downstream of it, so no two of them can be in here at
// once and no parallel test overlaps a sequential one at the top level.
//
// It fires on this box and on ci.yml's ubuntu legs; it does not fire on
// darwin, where TMPDIR is under /var/folders and the placement holds. The
// one host it changes a RENDERED profile for is a darwin box whose operator
// exported TMPDIR=/tmp — which, before this, was a host where every
// execution pin here graded a wall the blanket grant had already opened.
func sbNoBlanketTempGrants(t *testing.T, root string) {
	t.Helper()
	blanket := seatbeltTempGrants(runtime.GOOS, "")
	covered := false
	for _, g := range blanket {
		if underDir(absResolve(g), absResolve(root)) {
			covered = true
		}
	}
	if !covered {
		return
	}
	old := seatbeltTempGrants
	seatbeltTempGrants = func(string, string) []string { return nil }
	t.Cleanup(func() { seatbeltTempGrants = old })
}

// TestQASbRootHandsOutARootNoGrantReaches is sbRoot's contract as a test:
// however the premise was arranged — the fixture placed outside the blanket
// grants, or the grants taken away — a path under the root it returns is
// reached by nothing in a rendered writable set. Every grant pin downstream
// asks "does the grant under test reach my fixture", and that question has
// an answer only while this holds (ranger-base-u8xk4).
func TestQASbRootHandsOutARootNoGrantReaches(t *testing.T) {
	root := sbRoot(t)
	if tmp := os.Getenv("TMPDIR"); tmp == "" {
		t.Fatal("sbRoot left TMPDIR unset — every fixture it hands out then lands wherever the box's temp root is, which is the premise this helper exists to take away from the box")
	} else if absResolve(tmp) == absResolve(root) || underDir(absResolve(tmp), absResolve(root)) {
		t.Fatalf("sbRoot pointed TMPDIR (%s) at or above the root it returned (%s) — the root is then granted by the profile's own TMPDIR entry and no omission wall under it can be measured", tmp, root)
	}
	a := NewAppAt(sbMkdir(t, filepath.Join(root, "posse-home")))
	ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}
	w := a.SeatbeltWritable(ag, sbMkdir(t, filepath.Join(root, "work")), a.GatesDir("developer"))
	if sbCovers(w, filepath.Join(root, "nothing-grants-this")) {
		t.Errorf("a grant reaches the fixture root %s — the hermetic premise is gone and every grant pin below sbRoot is vacuous:\n  %s", root, strings.Join(w, "\n  "))
	}
	// And which of the two arrangements answered, said out loud, so a
	// reader of a failure above knows which one to go and look at. On a
	// host that grants a directory the root is inside, the stub must have
	// fired and the set must carry no blanket grant at all; on a host whose
	// TMPDIR is elsewhere the grants are real and the placement is what
	// holds.
	for _, g := range seatbeltTempGrants(runtime.GOOS, "") {
		if !underDir(absResolve(g), absResolve(root)) {
			continue
		}
		if containsString(w, absResolve(g)) {
			t.Errorf("the rendered set still carries the blanket temp grant %s while the fixture root (%s) is inside it — sbNoBlanketTempGrants did not fire:\n  %s", g, root, strings.Join(w, "\n  "))
		}
	}
}

// sbAssertHomeIsAFixture is the line that makes the one above load-bearing
// rather than decorative. Every sandbox pin downstream of sbRoot spells real
// operator-owned paths off the home it finds — `~/Library/Application
// Support/go/telemetry/mode`, which is exactly what `go telemetry off`
// writes; `~/.claude.json`, which the atomic-siblings fixture OVERWRITES with
// `{}` — and then mkdirs, writes and removes them. Every one of those is a
// fixture for exactly one reason: wtqaHome pointed HOME at a t.TempDir()
// first. Nothing checked that it had.
//
// ranger-base-g8ezm read the telemetry pin's trailing os.Remove as an
// unconditional delete of the operator's `go telemetry off` setting and filed
// it P2. It does not reproduce at HEAD — measured uncaged 2026-09-02 on
// darwin 25.4.0 / go1.26.5, the bead's own repro leaves the file intact and
// `go env GOTELEMETRY` still reads `off`. But it is one deleted line away
// from true: with the wtqaHome call above commented out, that same repro was
// re-run here and the pin passed GREEN while the operator's real mode file
// was deleted, and the atomic-siblings fixture would have truncated the real
// `~/.claude.json` on the way past. A bug report that is only wrong because
// of a coupling nothing states is a bug report worth making impossible.
//
// The HOME comparison is the hard arm: it needs no cgo, no passwd database
// and no privileges, so the guard cannot quietly become a no-op. The passwd
// read is the second arm and is skipped when unreadable — it catches the case
// the first cannot, a HOME that moved but moved to the operator's own home.
func sbAssertHomeIsAFixture(t *testing.T, before string) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if home == before {
		t.Fatalf("REFUSING TO RUN: HOME was not swapped for a fixture and still reads %s.\n"+
			"  every sandbox pin below this spells operator-owned paths off that home and then creates, overwrites and removes them —\n"+
			"  ~/Library/Application Support/go/telemetry/mode is the `go telemetry off` setting, ~/.claude.json is the runtime's config —\n"+
			"  so running on with the real home destroys the operator's files instead of measuring a wall (ranger-base-g8ezm)", home)
	}
	if u, err := user.Current(); err == nil && absResolve(home) == absResolve(u.HomeDir) {
		t.Fatalf("REFUSING TO RUN: HOME moved to %s, but that is still the operator's real home (passwd says %s).\n"+
			"  the pins below would create, overwrite and remove real files there rather than fixtures (ranger-base-g8ezm)", home, u.HomeDir)
	}
}

// sbCovers reports whether a writable set reaches p — the question the
// sandbox asks, not string equality: a grant of the parent covers it.
//
// The raw set, deliberately: what keeps a fixture from being reached by a
// grant it is not about is sbRoot's premise and not a filter here
// (ranger-base-u8xk4), so this stays the question the kernel would ask and
// every pin that renders a profile for a real kernel is asking the same one
// the set pins are.
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
	for _, f := range []string{"config.yaml", PromoteManifestFile, "agents/developer.md", "envs/default.env"} {
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
	personas := sbMkdir(t, filepath.Join(root, "constitution", ConstitutionSourceDir, "personas", "developer"))
	homeWithConstitution(t, a, filepath.Dir(personas))

	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(a.PersonasDir(), "developer")}
	w := a.SeatbeltWritable(ag, sbMkdir(t, filepath.Join(root, "work")), a.GatesDir("developer"))

	// Production's own detector over the rendered set. It answers "which of
	// my constitution paths does this set reach", so it is only a question
	// about this launch's grants while the fixture is outside the blanket
	// temp ones — which is sbRoot's premise, and which this box could not
	// supply until ranger-base-u8xk4 arranged it. Under a blanket `/tmp`
	// this named all eight promoted members for every fixture.
	if bad := a.ConstitutionGrants(w); len(bad) > 0 {
		t.Errorf("the writable set reaches the constitution %v:\n  %s", bad, strings.Join(w, "\n  "))
	}
	// Spelled out one path at a time as well, because ConstitutionGrants is
	// as much under test as the profile is: a list that had quietly gone
	// empty would satisfy the assertion above and nothing else.
	for _, p := range append(a.HomeConstitutionPaths(), filepath.Join(a.Home, "agents", "developer.md"), a.Home) {
		if sbCovers(w, p) {
			t.Errorf("%s must be in no writable set (ADR 0015 §2/§7):\n  %s", p, strings.Join(w, "\n  "))
		}
	}
}

// The other half of the same set: the one path under posse's state dir a
// session MUST keep, and the rest of that dir which it must NOT. `state/`
// is what the legacy hardcoded grant was reaching for and getting wrong —
// it named ~/.config/rhq/state literally, so a fleet on the §2 home
// rendered a profile with no state grant at all, and a second RHQ_HOME got
// a grant into the first one's state instead (rangerhq-qfzr).
//
// THE GRANT IS THE GATES DIR AND NOT THE STATE DIR, since ranger-base-6akzw
// (the fix half of the security lane's finding ranger-base-y438o). That dir
// holds what the LAUNCHER and the dispatcher write and later run or trust
// outside every wall — spilled launch lines a pane sources, session records
// carrying a seat's `cage:`, rendered skills trees, backups, the
// dispatcher's pid and lock files, work prompts — so a seat that could
// write them could plant something an uncaged process runs next.
// SeatbeltWritable's own comment is the long form; the census behind it
// found no seat writer at all.
//
// This pin asserted the OPPOSITE until ranger-base-u8xk4 — `a.StateDir` and
// `state/skills` writable, which is y438o's finding stated as a
// requirement — and read green through the narrowing, because its fixture
// home is under /tmp and every rendered profile grants a blanket `/tmp`.
// The security claim 6akzw landed had no pin in this file either way.
func TestQAStateStaysWritableAndIsDerivedFromTheHome(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	gates := a.GatesDir("developer")
	cwd := sbMkdir(t, filepath.Join(root, "work"))
	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(a.PersonasDir(), "developer")}
	w := a.SeatbeltWritable(ag, cwd, gates)

	// What a seat writes: its own gates dir, which carries its two audit
	// logs (SeatbeltCarveOut's Keep re-allows those by literal).
	for _, p := range []string{gates, filepath.Join(gates, "refusals.log"), filepath.Join(gates, "shell.log")} {
		if !sbCovers(w, p) {
			t.Errorf("a session cannot write %s:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// What the launcher and the dispatcher write and something uncaged runs
	// or trusts later (ranger-base-6akzw, ADR 0015 §3).
	for _, p := range []string{a.StateDir, filepath.Join(a.StateDir, "skills"), filepath.Join(a.StateDir, "launch"), filepath.Join(a.StateDir, "herdr")} {
		if sbCovers(w, p) {
			t.Errorf("a session can write %s — ranger-base-6akzw took that grant away because an uncaged process runs or trusts what is there:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// Derived, not spelled: a second home in the same process gets its own
	// grant and not the first's — the property a literal path cannot have.
	// Asked of the gates dir, which is the grant that survived.
	other := NewAppAt(filepath.Join(root, "home2"))
	otherGates := other.GatesDir("developer")
	wo := other.SeatbeltWritable(ag, cwd, otherGates)
	if sbCovers(wo, gates) {
		t.Errorf("a profile rendered for %s grants %s's gates dir:\n  %s", other.Home, a.Home, strings.Join(wo, "\n  "))
	}
	if !sbCovers(wo, otherGates) {
		t.Errorf("a profile rendered for %s does not grant its own gates dir:\n  %s", other.Home, strings.Join(wo, "\n  "))
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
	personas := filepath.Join(constitution, ConstitutionSourceDir, "personas")
	for _, p := range []string{"developer", "devops"} {
		sbMkdir(t, filepath.Join(personas, p))
	}
	homeWithConstitution(t, a, personas)

	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(a.PersonasDir(), "developer")}
	w := a.SeatbeltWritable(ag, sbMkdir(t, filepath.Join(root, "work")), a.GatesDir("developer"))

	// Granted under BOTH spellings, because they resolve to one directory.
	for _, p := range []string{
		filepath.Join(a.PersonasDir(), "developer", "ORDERS.md"),
		filepath.Join(personas, "developer", "ORDERS.md"),
	} {
		if !sbCovers(w, p) {
			t.Errorf("own memory %s is not writable:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// Denied under both, for the same reason.
	for _, p := range []string{
		filepath.Join(a.PersonasDir(), "devops", "ORDERS.md"),
		filepath.Join(personas, "devops", "ORDERS.md"),
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
	if prof := SeatbeltProfile("developer", w, nil, SeatbeltCarveOut{}); !strings.Contains(prof, sbQuote(absResolve(filepath.Join(personas, "developer")))) {
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

	ag := &AgentFile{Name: "developer", Writable: []string{filepath.Join(a.Home, "agents")}}
	bad := a.ConstitutionGrants(a.SeatbeltWritable(ag, cwd, a.GatesDir("developer")))
	if len(bad) != 1 || bad[0] != filepath.Join(a.Home, "agents") {
		t.Errorf("ConstitutionGrants = %v, want just the agents dir", bad)
	}
	// A grant INSIDE the area is the same breach spelled smaller, and
	// containment the other way round is the only thing that sees it.
	ag2 := &AgentFile{Name: "developer", Writable: []string{filepath.Join(a.Home, "agents", "developer.md")}}
	if bad := a.ConstitutionGrants(a.SeatbeltWritable(ag2, cwd, a.GatesDir("developer"))); len(bad) != 1 {
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
	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(a.PersonasDir(), "developer")}

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

// The exclusion list is itself a deny-list, and that is the one failure
// mode this whole design set out to avoid: nobody can tell by looking
// whether it is still complete. Measured (ranger-base-0djg) — with
// HomeConstitutionPaths returning only `PromotedPaths`, i.e. having
// quietly lost `envs/` and `promoted.json`, the entire rhq package stays
// green. Both survivors are the entries that are respelled literals here
// rather than read off promote.go the way the promoted four are, so they
// are exactly the two a later edit can drop without arguing with any other
// symbol.
//
// What is at stake in each: `envs/` is §7's secret values, and
// `promoted.json` is the manifest the launch verify hashes against — a
// trust anchor a session could rewrite is not one. Losing either from the
// list does not open a grant; it blinds the detector that would say so,
// and `posse gates` then prints the all-clear over a set nobody checked
// those two against.
func TestQAConstitutionExclusionListStaysComplete(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")

	got := map[string]bool{}
	for _, p := range a.HomeConstitutionPaths() {
		got[p] = true
	}
	// The promoted set, read off the same symbol promote copies.
	for _, rel := range PromotedPaths {
		if p := filepath.Join(a.Home, rel); !got[p] {
			t.Errorf("HomeConstitutionPaths has lost the promoted path %s: %v", p, a.HomeConstitutionPaths())
		}
	}
	// And the two §7/§3 entries that are spelled here and nowhere else.
	for _, p := range []string{a.EnvsDir, a.PromoteManifestPath()} {
		if !got[p] {
			t.Errorf("HomeConstitutionPaths has lost %s — the detector is now blind to it: %v", p, a.HomeConstitutionPaths())
		}
	}
	// The other direction, so the list cannot be "completed" by adding the
	// home wholesale: the three things at the home a session MUST keep are
	// not in it, and NotPromoted names two of them.
	for _, p := range []string{a.StateDir, a.PersonasDir(), a.GatesDir("developer")} {
		if got[p] {
			t.Errorf("%s must NOT be an exclusion — a session needs it (ADR 0015 §5, and state/ is what the runtime data IS)", p)
		}
	}
}

// The behavioural half of the same pin, and the one that dies on the
// mutation above: a `writable:` extra reaching `envs/` or the manifest is
// a breach ConstitutionGrants must NAME, not merely a path it happens to
// list. Item 5's clauses cover agents/ and recipes/; these two never had a
// case of their own.
func TestQAConstitutionGrantsCatchesEnvsAndTheManifest(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	cwd := sbMkdir(t, filepath.Join(root, "work"))

	for _, tc := range []struct{ what, grant string }{
		{"§7's env secrets", a.EnvsDir},
		{"a single env file inside them", filepath.Join(a.EnvsDir, "default.env")},
		{"the promote manifest", a.PromoteManifestPath()},
	} {
		ag := &AgentFile{Name: "developer", Writable: []string{tc.grant}}
		w := a.SeatbeltWritable(ag, cwd, a.GatesDir("developer"))
		if bad := a.ConstitutionGrants(w); len(bad) == 0 {
			t.Errorf("a grant on %s (%s) is unreported:\n  %s", tc.what, tc.grant, strings.Join(w, "\n  "))
		}
		// And loud in the place an operator reads, not just in the return.
		var b strings.Builder
		if err := a.SeatbeltReport(ag, cwd, &b); err != nil {
			t.Fatal(err)
		}
		if got := b.String(); !strings.Contains(got, "GRANT REACHES THE CONSTITUTION") {
			t.Errorf("gates prints the all-clear over a grant on %s:\n%s", tc.what, got)
		}
	}
}
