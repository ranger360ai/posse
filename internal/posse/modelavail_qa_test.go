package posse

// ranger-base-7vp — verifying the close of rangerhq-oay (the tier
// availability preflight).
//
// The delivered pins walk the resolution, the map, the cache and ONE
// launch. What they never walk is the SECOND launch: the mark rule (2)
// writes into the meta is re-derived at every launch from the pair the
// launch was asked for, and `posse relaunch` asks for the pair the LAST
// launch fell to. The two tests below that carry a skip are the boards
// that walk finds, measured 2026-08-28.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// qaPID writes a PID whose gates the wall fully realizes at shims, with
// whatever extra front-matter lines the board needs.
func qaPID(t *testing.T, b *HerdrBackend, name, tier string, extra ...string) {
	t.Helper()
	os.MkdirAll(b.App.AgentsDir, 0o755)
	body := fmt.Sprintf("---\nname: %s\ntier: %s\n%sdeny: [Bash(git push:*)]\n---\nYou are %s.\n",
		name, tier, strings.Join(extra, ""), name)
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// qaFellSession creates a persona session on a catalog that has lost the
// strong model, and returns the backend with the fallback already recorded.
func qaFellSession(t *testing.T, name string) (*HerdrBackend, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	b.Warn = &strings.Builder{}
	qaPID(t, b, "architect", TierStrong)
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
	if err := b.CreateSession(NewSessionOpts{Name: name, Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatalf("the preflight must never refuse a launch (rule 3): %v", err)
	}
	m, ok := b.readMeta(name)
	if !ok || m.Tier != TierStandard || m.Fallback == "" {
		t.Fatalf("board not set up: the create must have fallen and recorded it: %+v", m)
	}
	return b, fake
}

// ─── the second launch ───────────────────────────────────────────────────────

// The crash-restart path re-types into the live pane and writes the meta
// back, so the mark rides through. This is the arm that works, and it is
// here because it is the contrast that makes the next one a finding rather
// than a preference: nothing about a refresh makes the fact less true.
func TestQA7vpRelaunchAgentKeepsTheFallbackMark(t *testing.T) {
	b, fake := qaFellSession(t, "ra")

	m, _ := b.readMeta("ra")
	m.Launched = time.Now().Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fake, "agents.json")) // the CLI died
	ok, err := b.RelaunchAgent("ra", time.Second)
	if err != nil || !ok {
		t.Fatalf("relaunch: ok=%v err=%v", ok, err)
	}

	m2, _ := b.readMeta("ra")
	if m2.Fallback == "" {
		t.Errorf("the re-type dropped the mark: %+v", m2)
	}
	if m2.Tier != TierStandard {
		t.Errorf("meta tier = %q, want the tier that is really running", m2.Tier)
	}
	if log := calls(t, fake); !strings.Contains(log, "--model 'claude-opus-5'") {
		t.Errorf("the re-typed line must still name the substitute:\n%s", log)
	}
}

// MEASURED RED 2026-08-28 (ranger-base-twaq). `posse relaunch` recreates from
// RecreateOpts, which carries Tier: m.Tier — the SUBSTITUTE. The preflight
// then finds standard/opus available, so nothing falls, so nothing is
// recorded: the session goes on running opus while the PID says strong and
// `fallback:` is empty. `posse list` and the cockpit drop ⤵️fallback,
// describePlan prints no FALLBACK:, and dispatch's effectiveTier — which
// answers ONLY for a meta that records a fallback — hands the work prompt
// the bead's resolved tier, `strong`, for a session running opus. That last
// one is the sentence dispatch.go calls "the exact lie this preflight exists
// to kill".
func TestQA7vpFallbackMarkSurvivesPosseRelaunch(t *testing.T) {
	b, _ := qaFellSession(t, "pr")

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "pr", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}

	m, _ := b.readMeta("pr")
	if m.Tier != TierStandard {
		t.Fatalf("the refresh moved the tier as well: %+v", m)
	}
	if m.Fallback == "" {
		t.Errorf("a session still running the substitute wears no mark saying so: %+v", m)
	}
	if !strings.Contains(out.String(), "FALLBACK:") {
		t.Errorf("the receipt does not say the recreate is a degraded one:\n%s", out.String())
	}
	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list.String(), FallbackTag) {
		t.Errorf("posse list lost the mark across a refresh:\n%s", list.String())
	}
	// The downstream lie, which is the reason this matters beyond a tag.
	d := &Dispatcher{App: b.App, HB: b}
	if _, tier, fell := d.effectiveTier("pr", "claude", TierStrong); tier != TierStandard || fell == "" {
		t.Errorf("dispatch tells the persona it is thinking at %q with fell=%q; it is running claude-opus-5", tier, fell)
	}
}

// The other arm of the carry (ranger-base-twaq): a session that hopped to
// another RUNTIME is off its PID's pair by runtime, not by tier, and its
// mark rides a refresh for the same reason. Without it, collapsing the
// carry's condition to the tier half alone would silently un-mark every
// hopped session.
func TestQA7vpARuntimeHopKeepsItsMarkAcrossPosseRelaunch(t *testing.T) {
	b, _ := newTestBackend(t)
	b.Warn = &strings.Builder{}
	qaPID(t, b, "security", TierStrong)
	writeCfg(t, b.App, "tier_fallback:\n  security: codex\n")
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
	if err := b.CreateSession(NewSessionOpts{Name: "hr", Agent: "security", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.readMeta("hr"); m.Runtime != "codex" || m.Fallback == "" {
		t.Fatalf("board not set up: the create must have hopped and recorded it: %+v", m)
	}

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "hr", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("hr")
	if m.Runtime != "codex" {
		t.Fatalf("the refresh moved the runtime as well: %+v", m)
	}
	if m.Fallback == "" {
		t.Errorf("a session still running the substitute RUNTIME wears no mark saying so: %+v", m)
	}
	if !strings.Contains(out.String(), "FALLBACK:") {
		t.Errorf("the receipt does not say the recreate is a hopped one:\n%s", out.String())
	}
}

// The negative arm, and the reason the carry is conditioned rather than
// unconditional (ranger-base-twaq). The mark states a FACT — this session is
// not running the pair its PID names — so it is carried only while that fact
// holds. An operator who edits `tier:` down to what the session is really
// running has made the substitute the asked-for pair, and the old line
// ("tier strong wants claude-fable-5") is then false. It is dropped.
//
// TestQA7vpFallbackMarkSurvivesPosseRelaunch is the control: the same
// fixture, the same refresh, no PID edit, and the mark stays.
func TestQA7vpTheCarriedMarkIsDroppedOnceThePIDAsksForWhatIsRunning(t *testing.T) {
	b, _ := qaFellSession(t, "cu") // architect: tier strong, fell to standard

	qaPID(t, b, "architect", TierStandard) // the operator settles for what it got

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "cu", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("cu")
	if m.Tier != TierStandard {
		t.Fatalf("the refresh moved the tier: %+v", m)
	}
	if m.Fallback != "" {
		t.Errorf("the session runs exactly what its PID asks for; the mark is a lie now: %q", m.Fallback)
	}
	if strings.Contains(out.String(), "FALLBACK:") {
		t.Errorf("the receipt marks a launch that fell nowhere:\n%s", out.String())
	}
	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(list.String(), FallbackTag) {
		t.Errorf("posse list marks a session that is on its PID's own pair:\n%s", list.String())
	}
}

// ─── what a substitution may still cost ─────────────────────────────────────

// rangerhq-u2p's shape end to end, which the delivered pins walk only as far
// as the resolution: a per-persona `tier_fallback:` naming a RUNTIME hops the
// launch across, types that runtime's own line, and records the pair it
// really got. Rule (3) holds through the hop — nothing refuses.
func TestQA7vpARuntimeHopLaunchesAndIsRecordedAsTheRuntimeItGot(t *testing.T) {
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	qaPID(t, b, "security", TierStrong)
	writeCfg(t, b.App, "tier_fallback:\n  security: codex\n")
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone

	if err := b.CreateSession(NewSessionOpts{Name: "hop", Agent: "security", Dir: t.TempDir()}); err != nil {
		t.Fatalf("rule (3): a runtime hop must not turn a launch into a refusal: %v", err)
	}
	m, _ := b.readMeta("hop")
	if m.Runtime != "codex" || m.Tier != TierStrong {
		t.Errorf("the meta must record the pair that really launched: %+v", m)
	}
	if m.Fallback == "" || !strings.Contains(m.Fallback, "on codex") {
		t.Errorf("meta fallback = %q", m.Fallback)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "RHQ_RUNTIME=codex") {
		t.Errorf("the session was not launched on the substitute runtime:\n%s", log)
	}
	if strings.Contains(log, "claude-fable-5") {
		t.Errorf("the typed line still names the unavailable model:\n%s", log)
	}
	// The wrong arm: with the strong model present the hop must not happen,
	// or the assertions above are measuring a launch that always goes to
	// codex.
	b2, fake2 := newTestBackend(t)
	b2.Warn = &strings.Builder{}
	qaPID(t, b2, "security", TierStrong)
	writeCfg(t, b2.App, "tier_fallback:\n  security: codex\n")
	seedCatalog(t, b2.App, time.Minute, "claude-fable-5", "claude-opus-5")
	if err := b2.CreateSession(NewSessionOpts{Name: "nohop", Agent: "security", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if m2, _ := b2.readMeta("nohop"); m2.Runtime != "claude" || m2.Fallback != "" {
		t.Errorf("available means stay put: %+v", m2)
	}
	if log := calls(t, fake2); !strings.Contains(log, "RHQ_RUNTIME=claude") {
		t.Errorf("control launched somewhere else:\n%s", log)
	}
}

// The sharpest edge of rule (3), pinned as the deliberate reading it is
// rather than left to be rediscovered as a surprise. `tier_floor:` is the
// operator saying in advance "never run me below this"; the preflight hands
// the wall the SUBSTITUTED pair, so an unavailable strong model turns into a
// refused launch for a PID that set the floor at strong. The preflight still
// did not refuse — it printed its line and the floor refused — but the
// observable outcome for that PID is that the outage costs it the session.
//
// REFUTED while verifying ranger-base-7vp: the sibling worry, that a RUNTIME
// hop could cost a launch through the parity wall, does not reproduce. At
// cage shims every deny measured (Bash(git push:*), Edit, Write, Bash,
// Read(~/.ssh/**), Edit(**), Bash(security:*), NotebookEdit, Task,
// Bash(rm:*), WebFetch, WebSearch) refuses or launches identically on
// claude, codex and grok, so the hop moves no verdict. The first attempt at
// that board refused on codex only because the PID's WebFetch/WebSearch deny
// refuses on claude too — the control had not been run.
func TestQA7vpTierFloorStillRefusesTheSubstitutedPair(t *testing.T) {
	b, _ := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	qaPID(t, b, "floored", TierStrong, "tier_floor: strong\n")
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone

	err := b.CreateSession(NewSessionOpts{Name: "tf", Agent: "floored", Dir: t.TempDir()})
	if err == nil {
		t.Fatal("tier_floor: strong must rule on the pair that would really launch")
	}
	if !strings.Contains(err.Error(), "tier_floor") || !strings.Contains(err.Error(), "launching at standard") {
		t.Errorf("the refusal must name the floor and the substituted tier: %v", err)
	}
	// And the preflight's own line was printed first, so the operator reads
	// WHY the floor was hit and not just that it was.
	if !strings.Contains(warn.String(), "unavailable, falling back to claude-opus-5") {
		t.Errorf("the refusal arrives with no explanation: %q", warn.String())
	}
	// The wrong arm: the same PID launches when the model is there, so the
	// refusal above is the availability substitution and not the floor
	// misreading its own tier.
	b2, _ := newTestBackend(t)
	b2.Warn = &strings.Builder{}
	qaPID(t, b2, "floored", TierStrong, "tier_floor: strong\n")
	seedCatalog(t, b2.App, time.Minute, "claude-fable-5", "claude-opus-5")
	if err := b2.CreateSession(NewSessionOpts{Name: "tf2", Agent: "floored", Dir: t.TempDir()}); err != nil {
		t.Fatalf("control: an available strong model must launch under the same floor: %v", err)
	}
}

// ─── how old is "read" ───────────────────────────────────────────────────────

// Characterization, and it holds: a snapshot older than model_probe_ttl:
// whose re-read fails is still answered as KNOWN, and it still demotes. The
// file header's rule is "only a list that was actually read, and that does
// not contain the wanted id" — this is the measurement of how loosely
// "actually read" is bounded, which is not at all. The demotion is loud, so
// rule (2) holds and this is not the silent downgrade the design is written
// against; it is the arm to look at first if a launch is ever demoted
// against a catalog nobody recognises.
func TestQA7vpAnExpiredSnapshotStillDemotesWhenTheEndpointCannotBeReread(t *testing.T) {
	a := preflightApp(t) // unconfigured lister: every re-read fails
	seedCatalog(t, a, 30*24*time.Hour, "claude-opus-5", "claude-sonnet-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if !pf.Fell() || pf.Tier != TierStandard {
		t.Errorf("a month-old snapshot is the newest fact anyone has and is used as one: %+v", pf)
	}
	// The wrong arm: with NO snapshot at all the same failing re-read is
	// UNKNOWN, and UNKNOWN launches what it was asked to. If this stopped
	// failing, the test above would be measuring nothing.
	b := preflightApp(t)
	if pf := b.TierPreflight("architect", "claude", TierStrong, nil); pf.Fell() || pf.Tier != TierStrong {
		t.Errorf("no snapshot must be UNKNOWN, not unavailable: %+v", pf)
	}
}

// ─── the two operational keys ────────────────────────────────────────────────

// model_probe_ttl: was added by this bead and nothing pinned how it parses.
// The house duration form, 0 as "ask every launch", and the rule that a typo
// is NAMED and does not become a silent zero — a zero read out of a typo
// would put an HTTP request in front of every launch on this box.
func TestQA7vpModelProbeTTLForms(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want time.Duration
		says bool
	}{
		{"", ModelProbeTTLDefault, false},
		{"1h", time.Hour, false},
		{"90", 90 * time.Second, false},
		{"0", 0, false},
		{"15m", 15 * time.Minute, false},
		{"-5m", ModelProbeTTLDefault, true},
		{"1 hour", ModelProbeTTLDefault, true},
		{"true", ModelProbeTTLDefault, true},
	} {
		a := preflightApp(t)
		if c.raw != "" {
			writeCfg(t, a, "model_probe_ttl: "+c.raw+"\n")
		}
		var errw strings.Builder
		got := a.ModelProbeTTL(&errw)
		if got != c.want {
			t.Errorf("model_probe_ttl: %q → %s, want %s", c.raw, got, c.want)
		}
		if said := strings.Contains(errw.String(), "is not a duration"); said != c.says {
			t.Errorf("model_probe_ttl: %q said=%v want %v (%q)", c.raw, said, c.says, errw.String())
		}
	}
}

// The off switch is one word and only one word: `false` turns the preflight
// off, and every other spelling — including the ones an operator reaches for
// — leaves it ON, which is the fail-safe direction but is not what they
// typed. Pinned so the reading is deliberate rather than incidental.
func TestQA7vpModelPreflightOffIsExactlyTheWordFalse(t *testing.T) {
	for raw, want := range map[string]bool{
		"false": false, " false ": false,
		"no": true, "0": true, "off": true, "False": true, "true": true, "": true,
	} {
		a := preflightApp(t)
		writeCfg(t, a, "model_preflight: "+raw+"\n")
		if got := a.ModelPreflight(); got != want {
			t.Errorf("model_preflight: %q → on=%v, want on=%v", raw, got, want)
		}
	}
}
