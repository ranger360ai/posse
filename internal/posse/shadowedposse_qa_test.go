//go:build posse_arm2

package posse

// ranger-base-39jnl — the 2026-09-02 outage, pinned in four places.
//
// THE INCIDENT. `brew install ranger360ai/tap/posse` put a 08-29 release at
// /opt/homebrew/bin/posse, which precedes ~/.local/bin on the fleet PATH.
// Every `posse` from then on — two watch relaunches included — ran the
// release binary. Its PromotedPaths predated `runtimes` joining the promoted
// set (ADR 0039 D2, ranger-base-ight8), so HashPromotedSet never walked
// runtimes/ while the manifest, written by the promoted binary, named
// runtimes/claude.yaml. VerifyPromoted reported `missing runtimes/claude.yaml`
// — a file that was present, readable and hash-identical — and dispatch
// refused EVERY launch for ~90 minutes, burning the whole `-n 30` ration on
// refusals. The same `posse init` re-seeded examples/ and secrets/ over a
// promoted home.
//
// Four fixes, four pins:
//  1. the manifest records its writer and its set; the verdict leads with the
//     drift instead of a file (TestQA...DriftLeads..., ...NamesBothBinaries)
//  2. status/watch name the running binary and warn on a shadow (TestQAShadowed...)
//  3. a constitution refusal costs no launch ration (TestQAConstitutionRefusal...)
//  4. init refuses a promoted home before it writes (TestQAInitRefuses...)

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse"
)

// futureRoot is a promoted-set member this binary does not have and never
// will — the incident's shape with the roles swapped, so the pin needs no
// mutation of PromotedPaths (a package-level var that the parallel suite
// reads from many goroutines).
const futureRoot = "notaposseroot"

// TestQAPromotedSetDriftLeadsTheVerdict is fix 1's core claim: when the
// manifest was written for a set this binary does not walk, the line says so
// FIRST. The old line led with `missing <file>` and sent the operator to look
// at a file that was fine — 40 minutes of the incident went there.
//
// MUTATION: make setDrift return "" → the lead is gone → red on the first
// check. Make Line() append the drift after the classes instead of before →
// red on the prefix check.
func TestQAPromotedSetDriftLeadsTheVerdict(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	if err := os.MkdirAll(a.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	// A manifest a NEWER posse wrote: it names a root this binary does not
	// walk, and it says which binary wrote it and what set that was.
	m := &PromoteManifest{
		Version: promoteManifestVersion,
		Posse:   "0.4.0+92da1bc",
		Set:     append(append([]string{}, PromotedPaths...), futureRoot),
		Files:   map[string]string{futureRoot + "/claude.yaml": strings.Repeat("a", 64)},
	}
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	// The file IS there — exactly as runtimes/claude.yaml was on the box.
	if err := os.MkdirAll(filepath.Join(a.Home, futureRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Home, futureRoot, "claude.yaml"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := a.VerifyPromoted()
	if v.OK() {
		t.Fatal("fixture: this binary must not walk " + futureRoot + ", so the manifest's entry must go unmatched")
	}
	line := v.Line()
	// THE WRONG ARM, and the one the operator actually read: the verdict
	// must not open by naming the file.
	if strings.HasPrefix(line, "constitution does not match") {
		t.Errorf("the verdict still opens with the file classes — that line sent the operator to a file that was present and hash-matched:\n  %s", line)
	}
	for _, want := range []string{"OLDER posse", "which -a posse", "0.4.0+92da1bc", VersionString(), futureRoot} {
		if !strings.Contains(line, want) {
			t.Errorf("the verdict does not name %q — it must name both sets and both binaries:\n  %s", want, line)
		}
	}
	// The classes still follow: a drifted binary can also be looking at
	// genuinely changed prose, and dropping that would trade one blind spot
	// for another.
	if !strings.Contains(line, "missing") {
		t.Errorf("the drift replaced the path classes instead of leading them:\n  %s", line)
	}
}

// A manifest written before the `set` field still names its set — in the top
// segment of every key it carries. That is the manifest that was ON THE BOX
// during the incident, so a fix that only works on manifests written after it
// would have fixed nothing that day.
//
// MUTATION: make manifestRoots return nil when m.Set is empty → red.
func TestQAPromotedSetDriftIsReadableFromAnOldManifest(t *testing.T) {
	t.Parallel()
	m := &PromoteManifest{
		Version: promoteManifestVersion,
		Files: map[string]string{
			"config.yaml":                strings.Repeat("a", 64),
			futureRoot + "/claude.yaml":  strings.Repeat("b", 64),
			futureRoot + "/codex.yaml":   strings.Repeat("c", 64),
			"agents/somebody.md":         strings.Repeat("d", 64),
			"skills/ds/SKILL.md":         strings.Repeat("e", 64),
			"recipes/morning-sweep.yaml": strings.Repeat("f", 64),
		},
	}
	drift := setDrift(m)
	if drift == "" {
		t.Fatal("a pre-`set` manifest naming a root this binary does not walk must still read as drift — that is the manifest the outage happened on")
	}
	if !strings.Contains(drift, futureRoot) || !strings.Contains(drift, VersionString()) {
		t.Errorf("the derived drift must name the root and this binary: %s", drift)
	}
	// It must NOT claim to know the writer's whole declaration: the derived
	// set is a subset of what its writer walked (a home with no skills/
	// contributes no `skills` key), and reading a subset as the whole thing
	// is how a diagnosis becomes a lie.
	if !strings.Contains(drift, "at least") {
		t.Errorf("a derived set must be reported as a floor, not as the writer's declaration: %s", drift)
	}
}

// The other half of the same claim, and the one that protects the fleet: a
// manifest whose set this binary DOES walk, and a home merely missing a
// promoted root, drift not at all. Without this the fix would refuse every
// ordinary install (a home with no skills/) and every ordinary upgrade.
//
// MUTATION: drop the `recorded` guard in setDrift so the derived subset is
// compared both ways → the no-skills arm reds.
func TestQAPromotedSetDriftIsSilentWhenTheSetsAgree(t *testing.T) {
	t.Parallel()
	full := &PromoteManifest{Set: append([]string{}, PromotedPaths...), Files: map[string]string{}}
	if d := setDrift(full); d != "" {
		t.Errorf("a manifest written for this binary's own set must not drift: %s", d)
	}
	// A home with no skills/ and no recipes/: the derived roots are a strict
	// subset, which is an absence, not a different posse.
	subset := &PromoteManifest{Files: map[string]string{
		"config.yaml":        strings.Repeat("a", 64),
		"agents/somebody.md": strings.Repeat("b", 64),
	}}
	if d := setDrift(subset); d != "" {
		t.Errorf("a home that simply has no skills/ is not a drifted binary: %s", d)
	}
}

// Fix 1's other half: both writers stamp WHO wrote the manifest and WHAT SET
// they walked. Without this the drift is only ever derivable in one
// direction, and an OLDER binary reading a NEWER manifest — the incident —
// can never name the binary that produced the file it is complaining about.
//
// MUTATION: drop either stampWriter() call → red.
func TestQAManifestRecordsItsWriterAndItsSet(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	if err := os.MkdirAll(a.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: /tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("manifest %+v %v", m, err)
	}
	if m.Posse != VersionString() {
		t.Errorf("posse = %q, want %q — a manifest that cannot name its writer cannot name a mismatch's cause", m.Posse, VersionString())
	}
	if strings.Join(m.Set, " ") != strings.Join(PromotedPaths, " ") {
		// Fatal: the alias check below indexes m.Set, and a pin that
		// panics reports its finding as a stack trace.
		t.Fatalf("set = %v, want %v", m.Set, PromotedPaths)
	}
	// And it is a COPY: a manifest that aliased PromotedPaths would compare
	// equal to it forever, which is a drift check that can never fire.
	m.Set[0] = "mutated"
	if PromotedPaths[0] == "mutated" {
		t.Fatal("stampWriter aliased PromotedPaths — the promoted set is not the manifest's to hold a handle on")
	}
}

// ─── fix 2: which posse is answering ─────────────────────────────────────────

// TestQAShadowedPosseIsNamedNotGuessed: a posse that is not the one PATH
// resolves says so, naming both paths. Nothing on any surface said this
// during the incident, and the coordinator relaunched the watch loop twice
// into the same shadow.
//
// MUTATION: make Shadowed() return false → red. Drop the Warning() call from
// ReportPosseBinary → red on the report arm.
func TestQAShadowedPosseIsNamedNotGuessed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	running := filepath.Join(dir, "local", "posse")
	brewed := filepath.Join(dir, "brew", "posse")
	for _, p := range []string{running, brewed} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	b := runningPosse(
		func() (string, error) { return running, nil },
		func() (string, error) { return brewed, nil },
	)
	if !b.Shadowed() {
		t.Fatalf("a brew keg ahead of the running binary must read as a shadow: %+v", b)
	}
	warn := b.Warning()
	for _, want := range []string{brewed, running, "which -a posse"} {
		if !strings.Contains(warn, want) {
			t.Errorf("the warning does not name %q — an operator has to be able to see WHICH two binaries:\n  %s", want, warn)
		}
	}
	var sb strings.Builder
	b.report(&sb)
	if !strings.Contains(sb.String(), "posse binary ·") || !strings.Contains(sb.String(), "which -a posse") {
		t.Errorf("the report must carry both the identity and the warning:\n%s", sb.String())
	}

	// THE WRONG ARM: the same binary at both ends is not a finding. Without
	// this the warning fires on every ordinary box and is read by nobody.
	same := runningPosse(
		func() (string, error) { return running, nil },
		func() (string, error) { return running, nil },
	)
	if same.Shadowed() || same.Warning() != "" {
		t.Errorf("PATH agreeing with the running binary is not a finding: %q", same.Warning())
	}
}

// A persona session's PATH leads with its own gate shim dir, and a PID that
// denies `Bash(posse promote:*)` — every crew PID does — has a `posse` shim
// in it. Comparing raw paths there warns in EVERY session on a box whose
// PATH is perfectly correct: that false positive is what made row 2 of
// verify-bd-pin useless until ranger-base-43v1, and this pin is why it is not
// repeated here. What the shim EXECS is read out of the shim itself.
//
// MUTATION: drop the gateShimTarget branch in runningPosse → the shim path
// becomes First → Shadowed() → red.
func TestQAGateShimIsNotAShadow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	running := filepath.Join(dir, "local", "posse")
	if err := os.MkdirAll(filepath.Dir(running), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(running, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The shim as renderGates actually writes it, not a hand-typed copy —
	// so a change to the header or the exec line reds this instead of
	// silently un-teaching the reader.
	shimDir := filepath.Join(dir, "gates", "testpersona", "bin")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(shimDir, "posse")
	body := renderShim("testpersona", "posse", running, filepath.Join(dir, "refusals.log"), "/bin/date",
		ParseShimRules([]string{"Bash(posse promote:*)"})["posse"])
	if err := os.WriteFile(shim, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	b := runningPosse(
		func() (string, error) { return running, nil },
		func() (string, error) { return shim, nil },
	)
	if b.Shim == "" {
		t.Fatalf("the shim was not recognised as one — the header renderShim stamps is what identifies it: %+v", b)
	}
	if b.Shadowed() {
		t.Errorf("a gate shim that execs the running binary is the PID working, not a shadow: %s", b.Warning())
	}
	// And a shim in front of a DIFFERENT posse still is one: the shim is
	// not a blanket exemption, it is a redirection that has to be followed.
	other := filepath.Join(dir, "brew", "posse")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(shimDir, "stale-posse")
	staleBody := renderShim("testpersona", "posse", other, filepath.Join(dir, "refusals.log"), "/bin/date",
		ParseShimRules([]string{"Bash(posse promote:*)"})["posse"])
	if err := os.WriteFile(stale, []byte(staleBody), 0o755); err != nil {
		t.Fatal(err)
	}
	c := runningPosse(
		func() (string, error) { return running, nil },
		func() (string, error) { return stale, nil },
	)
	if !c.Shadowed() {
		t.Errorf("a shim frozen onto another posse is exactly the shadow this looks for: %+v", c)
	}
}

// ─── fix 3: a refusal is not an attempt ──────────────────────────────────────

// TestQAConstitutionRefusalCostsNoLaunchRation. ADR 0028 §2 denominates
// `-n`/`autostart_max_beads` in launch ATTEMPTS, failures included, because a
// failure still cost the box a session and the persona a turn. The launch
// verify's refusal costs neither: it fires in planLaunch, before a session is
// created, before the bead is claimed, before any prompt is sent. On
// 2026-09-02 the pass spent the whole `-n 30` epoch ration on thirty such
// refusals and then sat out the hour with the fix already in place.
//
// Two claims, and the second is why the first is affordable: the attempt is
// handed back, and the pass STOPS — the fact is one reading of one home, so
// walking the rest of the queue only reprints it once per seat.
//
// MUTATION: drop the `attempts--` → the ration is spent → red. Drop the
// `unratifiedHome = true` → the second bead is refused too → red on the
// single-refusal check.
func TestQAConstitutionRefusalCostsNoLaunchRation(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	a := b.App
	writePersona(t, a, "hopper", "[rust]")
	repo := qaRepo(t, a, `[{"id":"c-1","title":"t","labels":["rust"]},{"id":"c-2","title":"t","labels":["rust"]}]`, "")
	agentPerLaunch(t, fake)
	idleClaude(t, fake)

	// A home that does not match its manifest: the manifest names a file the
	// home does not have, which is what the launch verify refuses on.
	files, err := HashPromotedSet(a.Home)
	if err != nil {
		t.Fatal(err)
	}
	files["agents/nobody-wrote-this.md"] = strings.Repeat("a", 64)
	m := &PromoteManifest{Version: promoteManifestVersion, Files: files, SHA: strings.Repeat("b", 40)}
	m.stampWriter()
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if a.VerifyPromoted().OK() {
		t.Fatal("fixture: the home must not match its manifest, or nothing is refused")
	}

	beads := []RepoIssue{
		{BdIssue: BdIssue{ID: "c-1", Title: "t", Labels: []string{"rust"}}, Dir: repo},
		{BdIssue: BdIssue{ID: "c-2", Title: "t", Labels: []string{"rust"}}, Dir: repo},
	}
	dispatched, pending, attempts, err := d.fireLoop(beads, "", 0, map[string]string{}, map[string]int{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 || dispatched != 0 {
		t.Fatalf("fixture: nothing may launch on an unratified home (dispatched=%d pending=%d)", dispatched, len(pending))
	}
	out := dispatcherOut(d)
	// The witness that the fixture ran at all — an absence-only assertion is
	// satisfied by a pass that never reached the launch verify.
	if !strings.Contains(out, "ADR 0015 §3") {
		t.Fatalf("the fixture never reached the launch verify:\n%s", out)
	}
	if attempts != 0 {
		t.Errorf("attempts = %d, want 0 — a refusal that created no session, claimed no bead and sent no prompt attempted nothing (ADR 0028 §2)\n%s", attempts, out)
	}
	if n := strings.Count(out, "dispatch refuses to launch"); n != 1 {
		t.Errorf("the refusal printed %d times — the home is one reading, so the pass stops rather than repeating it per seat:\n%s", n, out)
	}
}

// ─── fix 4: init does not write a promoted home ──────────────────────────────

// TestQAInitRefusesAPromotedHomeBeforeItWrites. ADR 0031 §2's fence keys on
// the persona marker, so it covers sessions and not the operator's own hands
// — and on 2026-09-02 the operator's own hands ran the work-laptop install
// steps on the fleet box, re-seeding examples/ and secrets/ under a
// constitution `posse promote` owns.
//
// The claim under test is BEFORE, not merely "refuses": a refusal that
// happened after the copy would leave exactly the broken home it exists to
// prevent (ranger-base-pith).
//
// MUTATION: move the refusal below the MkdirAll/copy block → the shelf file
// appears → red. Change the guard to `man != nil` alone → the seeded-home
// upgrade arm reds.
func TestQAInitRefusesAPromotedHomeBeforeItWrites(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	// Turn it into what `posse promote` leaves: a manifest that is a claim
	// about a commit, Seeded false.
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("fixture: %+v %v", m, err)
	}
	m.Seeded = false
	m.SHA = strings.Repeat("c", 40)
	m.Repo = "/somewhere/constitution"
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	// Empty the shelf and secrets/ — the two directories the incident
	// re-seeded — so a copy that happens anyway is visible.
	if err := os.RemoveAll(a.ExampleAgentsDir()); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	err = a.initFrom(&out, posse.Seed, "embedded")
	if err == nil {
		t.Fatalf("init wrote a promoted home:\n%s", out.String())
	}
	for _, want := range []string{"promoted constitution", "posse promote", "RHQ_HOME=<scratch>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q — a refusal with no way forward is a wall:\n%v", want, err)
		}
	}
	if ents, _ := os.ReadDir(a.ExampleAgentsDir()); len(ents) != 0 {
		t.Errorf("init re-seeded the shelf on a promoted home (%d file(s)) — the refusal must come before the first copy", len(ents))
	}
	if out.String() != "" {
		t.Errorf("init printed progress before refusing, which reads as a run that happened:\n%s", out.String())
	}
	// And the home still verifies: refusing is only a fix if what it
	// protects survives it.
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("the refused init still moved the home off its manifest: %s", v.Line())
	}
}

// And the reading is really ON the watch preamble, not merely available.
// The incident's two relaunches wrote ninety minutes of log with no line
// anywhere naming the binary that wrote it — a stopped context runs zero
// passes, so anything in d.Out here is the preamble's.
//
// MUTATION: drop the ReportPosseBinary call from Watch → red.
func TestQAWatchPreambleNamesTheRunningPosse(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	qaRepo(t, b.App, `[]`, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, RunningPosse().Line()) {
		t.Errorf("--watch must name the posse binary that is writing the log:\n%s", out)
	}
}

// The two directions of drift are different findings, and reading them the
// same way is how a diagnosis becomes an alarm nobody believes. A binary
// that walks MORE than the manifest was written for is the ordinary upgrade
// order — a release that widened the promoted set, installed and not yet
// promoted — and every such release would otherwise announce "a different,
// older posse is answering here" on a box where nothing is wrong.
//
// MUTATION: collapse the two tails to one string → red.
func TestQAUpgradeOrderIsNotReportedAsAShadow(t *testing.T) {
	t.Parallel()
	// A manifest an OLDER posse wrote: it declares a set this binary's
	// PromotedPaths has grown past.
	older := &PromoteManifest{Posse: "0.3.0+aaaaaaa", Set: PromotedPaths[:len(PromotedPaths)-1], Files: map[string]string{}}
	up := setDrift(older)
	if up == "" {
		t.Fatal("a manifest written for a narrower set is still a drift worth naming")
	}
	if strings.Contains(up, "OLDER posse") {
		t.Errorf("the upgrade order must not be reported as a shadowed binary:\n  %s", up)
	}
	if !strings.Contains(up, "re-promote") {
		t.Errorf("the upgrade direction has one fix and must name it:\n  %s", up)
	}
	// And the incident's direction still reads as the incident.
	newer := &PromoteManifest{Posse: "0.4.0+bbbbbbb", Set: append(append([]string{}, PromotedPaths...), futureRoot), Files: map[string]string{}}
	if d := setDrift(newer); !strings.Contains(d, "OLDER posse") {
		t.Errorf("a manifest naming a root this binary cannot walk can only be an older posse answering:\n  %s", d)
	}
}
