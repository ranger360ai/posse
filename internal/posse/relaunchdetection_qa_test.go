//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-enmu2 — ADR 0013 §1 property 3's two RELAUNCH arms.
//
// ranger-base-d8riq made the launch row's refuse run on the paths that CREATE
// a session (detection_qa_test.go, whose fixture and herdr lever these reuse).
// Refresh is the other half, and it is two paths with nothing in common but
// the reading:
//
//	posse relaunch   recreate: kill the session, build it again from its meta
//	RelaunchAgent    re-type: the session is kept, the launch line goes into
//	                 its root pane, because the agent in it is gone
//
// WHAT EACH ARM COSTS WHEN THE RUNTIME IS ONE HERDR CANNOT NAME.
//
// The recreate spends a session that is ALIVE. planLaunch is asked before the
// kill, so the refusal is free — and the arm is not decided by whether the
// meta carries a bead, which is what makes it more than a repeat of d8riq's
// pin: `posse relaunch` is typed by the operator and a crew session's meta
// carries no bead, so the interactive clause that keeps `posse new` open would
// have let a refresh through on exactly the runtime it was written to protect.
// The escape hatch is not narrowed by closing this: the session keeps running
// and `posse new` still opens one (ADR 0060 D2).
//
// The re-type spends more than a pane. It fires on "the workspace is alive and
// herdr sees no agent in it", which on a runtime herdr can name means the CLI
// exited and left a bare shell (rangerhq-vk2) — and on a runtime herdr CANNOT
// name is the steady state of a perfectly healthy CLI, because every reading
// of such a session is `agent_not_found`. So the persona's launch line is
// typed into a live TUI's composer as a chat turn. That is the sentence the
// ADR block was written from, and the pin for it is the call log: no second
// `pane run`.
//
// WHAT WOULD FAIL SILENTLY WITHOUT THEM:
//
//   - delete the RelaunchAgent refusal and every other pin in this file and in
//     detection_qa_test.go stays green — the create paths all refuse, and this
//     one types.
//   - drop `Recreate` from RecreateOpts and the recreate of a session with no
//     bead is warned at and killed. The bead-carrying subtest cannot see it.
//   - wrap either refusal as sessionFailure and the slot stops benching: the
//     pass blames the pane and claims the next bead to refuse it too.
//   - read UNKNOWN as a "no" on either arm and a box whose herdr is off PATH
//     can no longer refresh a session at all.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// undetectableSession is one persona session on the `mycli` runtime, created
// the way an operator's own `posse new` creates one.
//
// That create is itself a witness, and a load-bearing one: it is the
// interactive arm of ADR 0015 §3 (property 5), so if it ever starts refusing,
// every arm below fails at its fixture rather than reporting a verdict — which
// is the honest way round, because a rig that could not open the session has
// measured nothing about refreshing one.
func undetectableSession(t *testing.T, b *HerdrBackend, name, dir string) *HerdrMeta {
	t.Helper()
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: dir, Agent: "ranger"}); err != nil {
		t.Fatalf("the operator's own launch on an undetectable runtime must PROCEED — it is the only route that can capture the fixtures the manifest is written from (ADR 0013 §1 property 5): %v", err)
	}
	m, ok := b.readMeta(name)
	if !ok {
		t.Fatal("no meta for the session that just launched")
	}
	if m.Runtime != "mycli" {
		t.Fatalf("the session came up on runtime %q, not mycli — NOTHING MEASURED: this rule only exists where herdr cannot name the argv0", m.Runtime)
	}
	return m
}

// The recreate arm, over all three readings of the same profile. `posse
// relaunch` plans before it kills, so the refusal costs the operator nothing
// and the session it was asked to refresh is still theirs.
func TestQAUndetectableRuntimeRefusesTheRecreateBeforeTheKill(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		names  []string
		refuse bool
	}{
		// herdr's own answer that it has no manifest for `mycli`.
		{"unknown_agent refuses", []string{"claude", "codex", "grok"}, true},
		// The negative control. Without it every assertion below holds
		// equally for a `posse relaunch` that had simply stopped working.
		{"a manifest recreates", []string{"claude", "mycli"}, false},
		// UNKNOWN refuses nothing (property 1): a herdr off PATH must not
		// cost a box the ability to refresh its sessions.
		{"a herdr that cannot be asked recreates", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			agentPerLaunch(t, fake)
			herdrNames(t, fake, tc.names...)
			undetectableFixture(t, b, PromptTyped, `[]`)
			m1 := undetectableSession(t, b, "s1", t.TempDir())

			var out strings.Builder
			err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
			log := calls(t, fake)

			if !tc.refuse {
				if err != nil {
					t.Fatalf("this reading refuses nothing — the refresh must run exactly as it did before this rule existed: %v\n%s", err, out.String())
				}
				m2, ok := b.readMeta("s1")
				if !ok || m2.Workspace == m1.Workspace {
					t.Errorf("the session was not actually recreated, so this control measured nothing: %+v", m2)
				}
				return
			}
			if err == nil {
				t.Fatalf("a refresh onto a runtime herdr cannot name must refuse: the live session would be spent on one every reading of which is agent_not_found\n%s", out.String())
			}
			// The shape the ADR asks for: what did NOT happen is in the
			// sentence, because that is the operator's next question.
			if !strings.Contains(err.Error(), "was NOT closed") {
				t.Errorf("the refusal must say the session is still there:\n%v", err)
			}
			for _, want := range []string{`argv0 "mycli"`, "docs/runbooks/agent-detection-manifest.md", "posse runtime check mycli"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal must carry %q:\n%v", want, err)
				}
			}
			// Nothing was spent, in the three places it could have been: the
			// landing turn, the kill, and the record the kill unlinks.
			if strings.Contains(log, "agent prompt") {
				t.Errorf("a refused refresh must not spend the landing turn either — the reading is above it:\n%s", log)
			}
			if strings.Contains(log, "workspace close") {
				t.Errorf("the session was CLOSED and cannot be recreated — this is rangerhq-v52t's loss with a new cause:\n%s\n%s", log, out.String())
			}
			m2, ok := b.readMeta("s1")
			if !ok || m2.Workspace != m1.Workspace {
				t.Errorf("a refused refresh must leave the session exactly as it was: %+v (was %+v)", m2, m1)
			}
			if s, rerr := b.Resolve("s1"); rerr != nil || s.WorkspaceID != m1.Workspace {
				t.Errorf("the session the operator asked to refresh must still be listed: %+v (%v)", s, rerr)
			}
		})
	}
}

// The RE-TYPE arm — the one path that puts a launch line into a pane that
// already exists, and the only one where a refusal is about a session that is
// running rather than one that would be created.
//
// The fixture is the sequence run for real, because a refusal on a rig that
// was never going to re-type is not a measurement:
//
//	launch the session                        → allowed (the escape hatch)
//	age the launch past the grace, kill the CLI
//	relaunch                                  → what this pin is about
func TestQAUndetectableRuntimeRefusesTheInPlaceRelaunchAndTypesNothing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		names  []string
		refuse bool
	}{
		{"unknown_agent refuses", []string{"claude", "codex", "grok"}, true},
		{"a manifest re-types", []string{"claude", "mycli"}, false},
		{"a herdr that cannot be asked re-types", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			agentPerLaunch(t, fake)
			herdrNames(t, fake, tc.names...)
			undetectableFixture(t, b, PromptTyped, `[]`)
			m := undetectableSession(t, b, "s1", t.TempDir())

			// The state this path reads as "the CLI died": the launch is well
			// past the grace, and herdr sees no agent anywhere.
			m.Launched = time.Now().Add(-time.Hour)
			if err := b.writeMeta(m); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(fake, "agents.json")); err != nil {
				t.Fatal(err)
			}
			typedAtLaunch := strings.Count(calls(t, fake), "pane run")

			ok, err := b.RelaunchAgent("s1", time.Second)
			typed := strings.Count(calls(t, fake), "pane run") - typedAtLaunch

			if !tc.refuse {
				if err != nil || !ok {
					t.Fatalf("this reading refuses nothing — a dead CLI must come back exactly as it did before this rule existed: ok=%v err=%v", ok, err)
				}
				if typed != 1 {
					t.Errorf("the persona line was not re-typed (%d pane runs), so this control measured nothing", typed)
				}
				return
			}
			if err == nil {
				t.Fatalf("herdr reporting no agent is the STEADY STATE of a live CLI on a runtime it cannot name — re-typing there lands the launch line in the composer as a chat turn: ok=%v", ok)
			}
			// The pin the ADR block names: the line was not typed. Everything
			// else here could hold for a refusal that still put the command in
			// the pane, which would be a message and not a wall.
			if typed != 0 {
				t.Errorf("%d launch line(s) were typed into the live pane after the refusal:\n%s", typed, calls(t, fake))
			}
			if ok {
				t.Error("RelaunchAgent reported it re-typed the session it had just refused")
			}
			if !strings.Contains(err.Error(), "refusing to retype") {
				t.Errorf("the refusal must say what it declined to do — this arm typed into a live session, so there is nothing to call a launch:\n%v", err)
			}
			for _, want := range []string{"s1", `argv0 "mycli"`, "docs/runbooks/agent-detection-manifest.md", "posse runtime check mycli"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal must carry %q:\n%v", want, err)
				}
			}
			// The stamp this path measures its own grace against must be
			// untouched. Bumped, the refusal would come back next pass as "too
			// young to relaunch" — the same silence in a different costume.
			after, found := b.readMeta("s1")
			if !found || time.Since(after.Launched) < 30*time.Minute {
				t.Errorf("a refusal re-stamped launched: (%s ago) — the next pass would read this session as a CLI still starting up", time.Since(after.Launched).Round(time.Second))
			}
		})
	}
}

// The busy-key split for the re-type arm (property 4), and the only pin here
// that runs a whole dispatch pass.
//
// It works by the SHAPE of the error: launchSession hands RelaunchAgent's
// error straight back, and fire's three-way switch benches the slot on the
// default arm. Wrapping it as sessionFailure would blame the pane and let the
// pass take the next bead to refuse it too — which every other assertion in
// this file would stay green for.
func TestQAUndetectableRuntimeInPlaceRelaunchBenchesTheSlotNotTheBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 200 * time.Millisecond
	agentPerLaunch(t, fake)
	herdrNames(t, fake, "claude", "codex", "grok")
	repo := undetectableFixture(t, b, PromptTyped,
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`)
	session := deadPersonaSession(t, b, fake, "ranger", repo, "a-1", time.Hour)
	typedAtLaunch := strings.Count(calls(t, fake), "pane run")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 {
		t.Fatalf("dispatched %d bead(s) into a session whose runtime herdr cannot name:\n%s", n, out)
	}
	if typed := strings.Count(calls(t, fake), "pane run") - typedAtLaunch; typed != 0 {
		t.Errorf("%d launch line(s) were typed into %s's live pane:\n%s", typed, session, calls(t, fake))
	}
	if got := strings.Count(out, "refusing to retype"); got != 1 {
		t.Errorf("the refusal is one fact about this persona on this runtime, so it is said ONCE and the slot is benched; said %d times:\n%s", got, out)
	}
	// The second bead must never be reached at all. It has no session yet, so
	// a pass that kept going would refuse it on the CREATE arm instead — a
	// different sentence, and the tell that the slot did not bench.
	if strings.Contains(out, "launch refused") {
		t.Errorf("the second bead reached a launch of its own — the first refusal must bench the slot, not just fail one bead:\n%s", out)
	}
	if !strings.Contains(out, "skipped for the rest of this pass") {
		t.Errorf("the second bead must be skipped by name, not claimed and refused:\n%s", out)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("no bead may be claimed on a benched slot:\n%s\n%s", log, out)
	}
}
