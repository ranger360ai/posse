package posse

// ranger-base-cplx, the arm ranger-base-twaq's condition does not reach.
//
// twaq carries the availability mark across `posse relaunch` and drops it
// again "once the PID asks for what is running" — an EQUALITY test on the
// pair. Any OTHER edit to the PID leaves the pair differing, so the mark
// rode through with its original sentence, naming a tier and a model the
// PID no longer asks for. TestQA7vpTheCarriedMarkIsDroppedOnceThePIDAsksFor
// WhatIsRunning is the arm that worked; this is the third-tier one.
//
// The fix re-derives rather than drops, and the assertions below are in two
// halves for that reason: what the mark must STOP saying (the stale tier and
// model), and what must survive saying it (a non-empty mark, so the
// ⤵️fallback tag, the receipt's FALLBACK: line and dispatch's effectiveTier
// go on describing a session that is not on its PID's pair).

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestQAACarriedMarkNamesThePIDAsItIsNow(t *testing.T) {
	t.Parallel()
	b, _ := qaFellSession(t, "cu") // architect: tier strong, fell to standard
	before, _ := b.readMeta("cu")
	qaPID(t, b, "architect", TierFast) // a THIRD tier: neither strong nor standard

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "cu", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("cu")
	if m.Tier != TierStandard || m.Runtime != "claude" {
		t.Fatalf("board not set up, the refresh must keep running the substitute: %+v", m)
	}
	if !strings.Contains(before.Fallback, TierStrong) {
		t.Fatalf("board not set up, the mark must have fallen FROM strong: %q", before.Fallback)
	}

	// What must stop being said: the PID asks neither of these any more.
	if strings.Contains(m.Fallback, TierStrong) {
		t.Errorf("the mark still says the PID asks for %s; it asks for %s: %q", TierStrong, TierFast, m.Fallback)
	}
	if strings.Contains(m.Fallback, "claude-fable-5-1") {
		t.Errorf("the mark still names the model that fall wanted: %q", m.Fallback)
	}
	if strings.Contains(out.String(), TierStrong) || strings.Contains(out.String(), "claude-fable-5-1") {
		t.Errorf("the receipt repeats it:\n%s", out.String())
	}

	// What must survive: the mark is EARNED — this session is not running
	// the pair its PID names — so it says so, about today's PID and today's
	// launch, and every consequence that hangs off a non-empty mark holds.
	if !strings.Contains(m.Fallback, "tier "+TierFast) || !strings.Contains(m.Fallback, "claude-sonnet-5") {
		t.Errorf("the mark does not name what this PID asks for now (%s → claude-sonnet-5): %q", TierFast, m.Fallback)
	}
	if !strings.Contains(m.Fallback, "claude-opus-5") {
		t.Errorf("the mark does not name what this session is really running: %q", m.Fallback)
	}
	if !strings.Contains(out.String(), "FALLBACK: "+m.Fallback) {
		t.Errorf("the receipt does not carry the re-derived mark:\n%s\nmark: %q", out.String(), m.Fallback)
	}
	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list.String(), FallbackTag) {
		t.Errorf("posse list lost the tag for a session that is still off its PID's pair:\n%s", list.String())
	}
	d := &Dispatcher{App: b.App, HB: b}
	if _, tier, fell := d.effectiveTier("cu", "claude", TierFast); tier != TierStandard || fell == "" {
		t.Errorf("dispatch tells the persona it is thinking at %q with fell=%q; it is running claude-opus-5", tier, fell)
	}
}

// The control, and the reason the re-derivation is conditioned: a PID that
// still asks for exactly what it asked for at the fall gets the sentence it
// fell with, to the byte. TestQA7vpFallbackMarkSurvivesPosseRelaunch is the
// same board asserted as non-empty; this one asserts nothing was rewritten,
// because a re-derivation that fired on every refresh would replace an
// operator's "unavailable, falling back to …" — the sentence that says WHY
// — with a weaker one that only says it happened.
func TestQAAAnUneditedPIDKeepsItsMarkToTheByte(t *testing.T) {
	t.Parallel()
	b, _ := qaFellSession(t, "un")
	before, _ := b.readMeta("un")

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "un", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("un")
	if m.Fallback != before.Fallback {
		t.Errorf("the mark was rewritten under an unedited PID:\n before %q\n after  %q", before.Fallback, m.Fallback)
	}
	if !strings.Contains(m.Fallback, "unavailable") {
		t.Errorf("the carried mark lost the reason the session fell: %q", m.Fallback)
	}
}

// The hop arm: a session that fell across RUNTIMES and whose PID then moved
// to a third tier. What is running is on another runtime, so the re-derived
// sentence has to name it — a mark that said "running gpt-5.6-sol" without
// "on codex" would read as a claude model id.
func TestQAAAHoppedSessionsReDerivedMarkNamesTheRuntime(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	b.Warn = &strings.Builder{}
	qaPID(t, b, "security", TierStrong)
	writeCfg(t, b.App, "tier_fallback:\n  security: codex\n")
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
	if err := b.CreateSession(NewSessionOpts{Name: "hp", Agent: "security", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.readMeta("hp"); m.Runtime != "codex" || m.Fallback == "" {
		t.Fatalf("board not set up: the create must have hopped and recorded it: %+v", m)
	}
	qaPID(t, b, "security", TierFast) // a third tier, on the PID's own runtime

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "hp", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("hp")
	if m.Runtime != "codex" || m.Fallback == "" {
		t.Fatalf("the refresh must keep running the substitute runtime, marked: %+v", m)
	}
	if strings.Contains(m.Fallback, TierStrong) {
		t.Errorf("the mark still says the PID asks for %s: %q", TierStrong, m.Fallback)
	}
	if !strings.Contains(m.Fallback, "on codex") {
		t.Errorf("the re-derived mark does not say the session is running on another runtime: %q", m.Fallback)
	}
}

// The other way a PID stops asking for what it asked for at the fall, and
// the reason the carry check is anchored on the separator as well as the
// clause: the TIER is untouched and the runtime's map under it moved. An
// overlay rolling `model_strong:` back to claude-fable-5 leaves a mark
// naming claude-fable-5-1 — a strict extension of what the PID asks for
// now, so a prefix test that stopped at the model id would read it as the
// same ask and carry it.
func TestQAAAModelBumpUnderAnUntouchedTierIsNotTheSameAsk(t *testing.T) {
	t.Parallel()
	b, _ := qaFellSession(t, "bp")
	if m, _ := b.readMeta("bp"); !strings.Contains(m.Fallback, "claude-fable-5-1") {
		t.Fatalf("board not set up, the fall must name the longer id: %q", m.Fallback)
	}
	// The PID still says strong; strong now means a different model.
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "claude", "model_strong: claude-fable-5\n")

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "bp", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}
	m, _ := b.readMeta("bp")
	if m.Fallback == "" {
		t.Fatalf("the session still runs the substitute; the mark must not be dropped: %+v", m)
	}
	if strings.Contains(m.Fallback, "claude-fable-5-1") {
		t.Errorf("the mark still names the id the PID's tier no longer maps: %q", m.Fallback)
	}
	if !strings.Contains(m.Fallback, "wants claude-fable-5 —") {
		t.Errorf("the mark does not name what tier %s means today: %q", TierStrong, m.Fallback)
	}
}
