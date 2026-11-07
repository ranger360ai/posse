//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-nhxdx — ADR 0076 Amendment 3's D12 and D13: grok is
// admitted on Rex by its OWN report, behind one operator-owned config line.
//
// THE CLAIM, in four parts:
//
//  1. The READER. `[ui.notifications] program_status` out of
//     `$GROK_HOME/config.toml`, SECTION-AWARE — which is the half tomlFlag
//     cannot do and the half that matters, since a section-blind read would
//     take the key out of any table and admit a seat on it.
//  2. The VERDICT. Exactly `"on"` admits. `auto` (the shipped default), `off`,
//     an absent key, an absent file and an unreadable one all refuse, and the
//     refusal names the line to add.
//  3. The PLACEMENT, which is the decision. Both D1 call sites read it before
//     the claim: dispatch.go above the argv ladder, planLaunch as the backstop
//     for every other bead-carrying path — with D1's interactive asymmetry
//     (dispatched refuses, `posse new` warns and proceeds).
//  4. The VERSION, D13's pin on a NATIVE report: grok will not confirm the key
//     (`inspect --json` reports no value and its validator calls the table
//     unrecognised, both MEASURED), so the release the measurement was taken
//     on is declared and a grok on another one is refused — with the probe's
//     own arms named as the re-measurement.
//
// EVERY ARM RUNS AGAINST A FIXTURE $GROK_HOME and a pinned version seam, so
// nothing here reads the operator's own config or the box's own grok. That is
// not only hygiene: the box this was written on HAS the line, so a pin that
// read the live config would be green on this machine and undefined
// everywhere else. `GROK_HOME` is the override posse already resolves grok's
// root through (ADR 0077 D1, grokHomeIn), which is why a fixture is one
// t.Setenv — and why these tests are not t.Parallel.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// grokFixtureHome writes a fixture grok home carrying body as its
// config.toml and points $GROK_HOME at it. "" writes NO config.toml at all,
// which is the absent-file arm.
func grokFixtureHome(t *testing.T, body string) string {
	t.Helper()
	h := filepath.Join(t.TempDir(), ".grok")
	if err := os.MkdirAll(h, 0o755); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(h, "config.toml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(GrokHomeVar, h)
	return h
}

// grokPinVersion pins the `--version` seam at the release grok's built-in
// declares, so every arm below measures the PRECONDITION and never the
// version pin standing in for it. parity.go's own seam, the one
// RexVersionDrift reads through (skillsDriftVersion).
func grokPinVersion(t *testing.T, rt *Runtime, v string) {
	t.Helper()
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return v }
}

// ─── 1 and 2: the reader and the verdict ─────────────────────────────────────

func TestQAGrokProgramStatusIsReadSectionAwareAndOnlyOnAdmits(t *testing.T) {
	// THE THIRD COLUMN IS THE POINT. tomlFlag (interstitial.go) is
	// line-level and section-blind on purpose — its two keys are unique in
	// grok's config.toml — and reusing it here would admit a seat on a
	// `program_status = "on"` sitting under `[cli]`, which grok does not read
	// as this key at all. The `wrong table` arms are that bug, pinned.
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"the table spelling the ADR names", "[ui.notifications]\nprogram_status = \"on\"\n", true},
		{"a dotted key under [ui]", "[ui]\nnotifications.program_status = \"on\"\n", true},
		{"a fully dotted key at the root", "ui.notifications.program_status = \"on\"\n", true},
		{"no space around the equals", "[ui.notifications]\nprogram_status=\"on\"\n", true},
		{"a trailing comment", "[ui.notifications]\nprogram_status = \"on\" # posse reads this\n", true},
		{"the key beside the operator's own [ui] keys", "[ui]\npermission_mode = \"always-approve\"\n[ui.notifications]\nprogram_status = \"on\"\n[models]\ndefault = \"grok-4.6\"\n", true},

		{"auto, the shipped default", "[ui.notifications]\nprogram_status = \"auto\"\n", false},
		{"off", "[ui.notifications]\nprogram_status = \"off\"\n", false},
		{"the key absent from a readable config", "[cli]\nauto_update = false\n", false},
		{"an empty value", "[ui.notifications]\nprogram_status = \"\"\n", false},

		// Section-blindness, four ways. Each of these is a config grok
		// reports NOTHING on, and each would read as `on` to a line-level
		// matcher.
		{"the right value under the wrong table", "[cli]\nprogram_status = \"on\"\n", false},
		{"the right value at the root", "program_status = \"on\"\n", false},
		{"the right value one table short", "[ui]\nprogram_status = \"on\"\n", false},
		{"the right value in an array of tables", "[[ui.notifications]]\nprogram_status = \"on\"\n", false},
		{"commented out", "[ui.notifications]\n# program_status = \"on\"\n", false},
		// A spelling posse does not read. REFUSING is the contract (the
		// reader's own doc): the cost is one line re-spelled, and the cost of
		// guessing is a seat whose every state is unknown.
		{"an inline table", "[ui]\nnotifications = { program_status = \"on\" }\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			grokFixtureHome(t, tc.body)
			read, ok := grokProgramStatusRead()
			if ok != tc.want {
				t.Fatalf("grokProgramStatusRead() = (%q, %v), want ok=%v for:\n%s", read, ok, tc.want, tc.body)
			}
			// The reading is handed back in every arm, because a refusal
			// that cannot say WHAT it read is a wall with no door on it.
			if read == "" {
				t.Error("the reader admitted or refused and said nothing about what it read")
			}
			if !strings.Contains(read, "config.toml") {
				t.Errorf("the reading does not name the file it read: %q", read)
			}
		})
	}
}

func TestQAGrokProgramStatusUnreadableIsUnknownAndRefuses(t *testing.T) {
	// ADR 0013 §1's rule, which D12 names in its own words ("unknown is
	// never yes"), and the one arm where an operator may have the line and
	// still be refused: posse walled the read, not the operator.
	h := grokFixtureHome(t, "[ui.notifications]\nprogram_status = \"on\"\n")
	p := filepath.Join(h, "config.toml")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Skipf("cannot make the fixture config unreadable: %v", err)
	}
	t.Cleanup(func() { os.Chmod(p, 0o644) })
	if os.Geteuid() == 0 {
		t.Skip("running as root: a 0000 file is still readable")
	}
	read, ok := grokProgramStatusRead()
	if ok {
		t.Fatalf("an unreadable config was read as the precondition MET (%q) — unknown is never yes (ADR 0013 §1)", read)
	}
	for _, want := range []string{"cannot read", "config.toml"} {
		if !strings.Contains(read, want) {
			t.Errorf("the unknown does not say %q, so an onboarder cannot tell it from `auto`: %q", want, read)
		}
	}
	// An ABSENT file is a READING and not an unknown (grokPrivacyProbe's
	// rule: grok writes this file itself), and it refuses either way.
	grokFixtureHome(t, "")
	if read, ok := grokProgramStatusRead(); ok || !strings.Contains(read, "no ") {
		t.Errorf("an absent config = (%q, %v), want a refusal that says the file is not there", read, ok)
	}
	// No home at all: Join("", ".grok") is a relative path under this
	// process's cwd — another box's answer reported as this operator's
	// (grokHomeIn's note, ranger-base-58b5).
	t.Setenv(GrokHomeVar, "")
	t.Setenv("HOME", "")
	if read, ok := grokProgramStatusRead(); ok || !strings.Contains(read, GrokHomeVar) {
		t.Errorf("with no home at all = (%q, %v), want a refusal naming %s", read, ok, GrokHomeVar)
	}
}

func TestQAGrokPreconditionReadsThroughTheDeclarationAndNotTheName(t *testing.T) {
	// ADR 0017 §3: the launch paths ask the DECLARATION. A registry key, a
	// registry, and a refusal for a key no reader implements — which is the
	// opposite direction from turnOutcomeReaders' nil, because an
	// unresolvable ADMISSION precondition is a launch posse would be making
	// on a reading nobody performed.
	grokFixtureHome(t, "[ui.notifications]\nprogram_status = \"on\"\n")
	if read, ok := RexPreconditionMet(builtinRuntime(t, "grok")); !ok {
		t.Fatalf("grok was refused on a fixture home that carries the line: %q", read)
	}
	// The runtimes that declare none are met by construction: there is
	// nothing to read and nothing to say about it.
	for _, name := range []string{"claude", "codex"} {
		rt := builtinRuntime(t, name)
		if rt.RexPrecondition != "" {
			t.Fatalf("%s declares a Rex precondition %q — this pin's premise is that only grok has one", name, rt.RexPrecondition)
		}
		if read, ok := RexPreconditionMet(rt); !ok || read != "" {
			t.Errorf("%s was asked a precondition it declares none of = (%q, %v)", name, read, ok)
		}
	}
	if read, ok := RexPreconditionMet(nil); !ok || read != "" {
		t.Errorf("a nil runtime = (%q, %v), want met with nothing to say", read, ok)
	}
	// A declared key with no reader REFUSES. Reached by hand, because no
	// built-in can carry one and no yaml can set the field at all.
	bogus := builtinRuntime(t, "grok")
	bogus.RexPrecondition = "a-reader-nobody-wrote"
	read, ok := RexPreconditionMet(bogus)
	if ok {
		t.Fatalf("a precondition with no registered reader was ADMITTED (%q) — that is an admission resting on a reading nobody performed", read)
	}
	if !strings.Contains(read, "a-reader-nobody-wrote") {
		t.Errorf("the refusal for an unregistered precondition does not name it: %q", read)
	}
}

func TestQAGrokPreconditionRefusalNamesTheLineAndTheDoor(t *testing.T) {
	grokFixtureHome(t, "[ui.notifications]\nprogram_status = \"auto\"\n")
	rt := builtinRuntime(t, "grok")
	read, ok := RexPreconditionMet(rt)
	if ok {
		t.Fatalf("`auto` was admitted (%q) — it is the SHIPPED DEFAULT and is MEASURED both silent on Rex and leaking `n,auth` into grok's composer", read)
	}
	err := RexPreconditionRefusal(rt, read)
	if err == nil {
		t.Fatal("RexPreconditionRefusal returned nil")
	}
	// The line to add, the value read, the ADR, the instance key, the
	// placement, and herdr as the door that needs nothing added. A refusal
	// missing the line is the one shape an operator cannot act on.
	for _, want := range []string{
		"grok", "auto", "[ui.notifications]", "program_status", "\\\"on\\\"",
		"herdr", MultiplexerKey + ": " + MuxRex, "ADR 0076 D12", "before the claim",
		"never posse's", "verify-grok-pin",
	} {
		if !strings.Contains(err.Error(), strings.ReplaceAll(want, "\\\"", "\"")) {
			t.Errorf("the precondition refusal does not name %q:\n%v", want, err)
		}
	}
	// It never sends the operator to a session: refusing here is exactly the
	// decision that no session exists yet.
	if strings.Contains(err.Error(), "posse peek") {
		t.Errorf("a pre-claim refusal points at a session, and its whole placement is that there is none:\n%v", err)
	}
	// The interactive arm carries the same facts and says DEGRADED, which is
	// the asymmetry every reading around it has (ADR 0013 §1 rule 5).
	w := RexPreconditionDegraded(rt, read)
	for _, want := range []string{"DEGRADED", "grok", "auto", "program_status", "herdr"} {
		if !strings.Contains(w, want) {
			t.Errorf("the interactive warning does not carry %q:\n%s", want, w)
		}
	}
}

// ─── 3: the placement, which IS the decision (ADR 0076 D12) ──────────────────

// grokRexBackend is a `multiplexer: rex` instance with the fake Rex slot, a
// persona, a fixture grok home carrying body, and the version seam pinned at
// grok's own declaration — so the gate under test is the precondition and
// never the version pin standing in for it.
func grokRexBackend(t *testing.T, body string) *HerdrBackend {
	t.Helper()
	b, _ := newTestBackend(t)
	if err := os.WriteFile(b.App.ConfigPath, []byte(MultiplexerKey+": "+MuxRex+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.Slot, _ = newTestRexMux(t)
	writePersona(t, b.App, "ranger", "[go]")
	grokFixtureHome(t, body)
	grokPinVersion(t, builtinRuntime(t, "grok"), builtinRuntime(t, "grok").RexVerified)
	return b
}

// The arm that goes red if this refusal is ever rebuilt as a launch-then-wait,
// and the one the muxslot placement pins cannot hold: they are about D1's
// declaration and a yaml runtime, where this is about a runtime D1 ADMITS and
// a reading of the box one step later. grok is `prompt: argv`, so a
// try-and-see here is a claim, a worktree and a real billed turn nobody
// observes (ADR 0013 §1 rule 3).
func TestQADispatchRefusesGrokOnRexForTheProgramStatusPreconditionBeforeTheClaim(t *testing.T) {
	b := grokRexBackend(t, "[ui.notifications]\nprogram_status = \"auto\"\n")
	_, fake := b, fakeDirOf(t)
	d := newTestDispatcher(t, b)
	rt := builtinRuntime(t, "grok")
	if !RexAdmits(rt) {
		t.Fatal("grok is no longer admitted by D1, so this pin is measuring the declaration and not the precondition — re-aim it")
	}
	if _, ok := RexVersionDrift(rt); !ok {
		t.Fatal("the version seam is not pinned at grok's declaration, so the version refusal could stand in for the precondition's")
	}

	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	_, err := d.launchSession(is, "ranger", "ranger-x-a-1", "grok", "fast", func() string { return "work prompt" }, nil, false)
	if err == nil {
		t.Fatal("a grok launch on a Rex instance was allowed over a config that says `auto` — MEASURED: no record through a working turn, so every state of that seat would be a guess (ADR 0076 D12)")
	}
	for _, want := range []string{"grok", "auto", "program_status", "herdr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q — it may be some other gate standing in for the precondition:\n%v", want, err)
		}
	}
	// A plain error, not a sessionFailure: fireLoop's busy-key split has to
	// bench the persona/runtime SLOT, because every bead routed here meets
	// the same missing line (ADR 0013 §2 property 4).
	if _, isSession := err.(sessionFailure); isSession {
		t.Errorf("the refusal came back as a sessionFailure — that benches the pane and re-claims the next bead onto the same gap:\n%v", err)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("the bead was claimed before the precondition was read:\n%s", log)
	}
	for _, c := range rexCalls(t) {
		for _, verb := range []string{" new ", " send ", " send-key ", " kill "} {
			if strings.Contains(" "+c+" ", verb) {
				t.Errorf("a refused launch ran rex%s— nothing may be created or typed before the precondition is read: %s", verb, c)
			}
		}
	}
}

// The BACKSTOP, for every bead-carrying path that is not the dispatch loop —
// a cockpit `d` on a session it must create, a recipe, a recreate — with the
// interactive asymmetry. Four arms, because the condition can fail open in
// either direction: a reading that refuses everything would red the last arm,
// and one that refuses nothing would red the first two.
func TestQAPlanLaunchIsTheGrokPreconditionBackstop(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		o       NewSessionOpts
		refused bool
	}{
		{"a recipe or cockpit launch carrying a bead", "[ui.notifications]\nprogram_status = \"auto\"\n", NewSessionOpts{Bead: "ranger-base-x"}, true},
		{"a recreate, which plans before it kills", "[ui.notifications]\nprogram_status = \"off\"\n", NewSessionOpts{Recreate: true}, true},
		{"no config at all", "", NewSessionOpts{Bead: "ranger-base-x"}, true},
		{"the operator's own, at the keyboard", "[ui.notifications]\nprogram_status = \"auto\"\n", NewSessionOpts{}, false},
		{"the line is there", "[ui.notifications]\nprogram_status = \"on\"\n", NewSessionOpts{Bead: "ranger-base-x"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := grokRexBackend(t, tc.body)
			o := tc.o
			o.Name, o.Dir, o.Agent, o.Runtime = "s1", t.TempDir(), "ranger", "grok"
			_, err := b.planLaunch(o)
			warn := warnBuf(t, b).String()

			// ADR 0013 §1 rule 3: the verdict is read off a declaration and
			// one small ReadFile, so no seat is opened and no occupant is
			// asked, on any arm. A try-and-see rebuild shows up here.
			if calls := rexCalls(t); len(calls) != 0 {
				t.Errorf("planLaunch made %d rex call(s) — the precondition is read before the claim, and a plan that asks the occupant has already spent what refusing was meant to save:\n%s", len(calls), strings.Join(calls, "\n"))
			}

			if !tc.refused {
				if err != nil {
					t.Fatalf("planLaunch refused this launch: %v", err)
				}
				if tc.o.Bead == "" && !tc.o.Recreate && tc.body != "" && !strings.Contains(tc.body, "\"on\"") {
					// The interactive escape hatch still has to SAY it.
					if !strings.Contains(warn, "DEGRADED") || !strings.Contains(warn, "program_status") {
						t.Errorf("an interactive launch on a grok Rex cannot read proceeded without saying so:\n%s", warn)
					}
				}
				return
			}
			if err == nil {
				t.Fatal("planLaunch planned a grok launch on a `multiplexer: rex` instance whose config does not carry the line — without this backstop a cockpit `d`, a recipe or a recreate opens a seat whose every state is unknown (ADR 0076 D12)")
			}
			for _, want := range []string{"grok", "program_status", "herdr", MultiplexerKey + ": " + MuxRex, "ADR 0076"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q, so it may be some other planLaunch gate standing in for the precondition:\n%v", want, err)
				}
			}
			if warn != "" {
				t.Errorf("a refused launch also printed a line — the asymmetry is refuse OR warn, never both:\n%s", warn)
			}
		})
	}
}

// ─── 4: D13's version pin on a NATIVE report ─────────────────────────────────

// The pin exists on grok for a reason claude's native report does not have,
// and the sentence has to say which: grok's record is its own output, but the
// line that makes it report is a key grok itself will not confirm. So the
// refusal names the PRECONDITION's probe arms as the re-measurement, not
// codex's route probes — and this pin is what stops the two arms of
// rexVersionLines/rexVersionDoor collapsing back into one.
func TestQAGrokRexVersionPinNamesTheProgramStatusProbeAndNotTheRoute(t *testing.T) {
	grokFixtureHome(t, "[ui.notifications]\nprogram_status = \"on\"\n")
	gk, cx, cl := builtinRuntime(t, "grok"), builtinRuntime(t, "codex"), builtinRuntime(t, "claude")
	if gk.RexVerified == "" {
		t.Fatal("the grok built-in carries no rex_verified — D13's pin is the only thing between a bumped grok and an admission resting on a measurement nobody retook")
	}
	grokPinVersion(t, gk, gk.RexVerified)
	if got, ok := RexVersionDrift(gk); !ok {
		t.Errorf("the release the key WAS measured on was refused (read %q, want %q)", got, gk.RexVerified)
	}
	// A trailing bracketed field is BOX STATE and not release identity (ADR
	// 0007 §6c, skillsVersionRelease): this box's grok prints ` [stable]`
	// when its update-check cache exists and nothing when it does not, and
	// neither may read as drift.
	skillsDriftVersion = func(string) string { return gk.RexVerified + " [stable]" }
	if got, ok := RexVersionDrift(gk); !ok {
		t.Errorf("a grok printing its update-check channel was read as drift (%q) — the suffix is box state (ADR 0007 §6c)", got)
	}

	skillsDriftVersion = func(string) string { return "grok 1.0.70 (deadbeefcafe)" }
	got, ok := RexVersionDrift(gk)
	if ok {
		t.Fatalf("a grok reporting %q was admitted on a key measured on %q (ADR 0076 D13)", got, gk.RexVerified)
	}
	err := RexVersionRefusal(gk, got)
	for _, want := range []string{"1.0.70", gk.RexVerified, "0076-grok-hook.probe.sh", "auto on off onidle", "ADR 0076 D13", "before the claim", "herdr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("grok's version refusal does not name %q:\n%v", want, err)
		}
	}
	// It must NOT describe grok's record as posse's route: the whole reason
	// the pin is on a native report is the config key, and a reader sent to
	// the trust-ask probe would re-measure a mechanism grok does not use.
	for _, never := range []string{"0076-trust-ask.probe.sh", "session-flag hooks", "review dialog"} {
		if strings.Contains(err.Error(), never) {
			t.Errorf("grok's version refusal describes codex's ROUTE (%q) — grok writes its own record and posse types nothing for it:\n%v", never, err)
		}
	}
	if w := RexVersionDegraded(gk, got); !strings.Contains(w, "DEGRADED") || !strings.Contains(w, "1.0.70") {
		t.Errorf("the interactive arm does not carry the same facts:\n%s", w)
	}
	// A grok that will not say which grok it is: unknown is never no.
	skillsDriftVersion = func(string) string { return "" }
	if got, ok := RexVersionDrift(gk); ok {
		t.Errorf("a grok that would not answer `--version` was admitted (read %q)", got)
	} else if !strings.Contains(RexVersionRefusal(gk, got).Error(), "would not answer") {
		t.Errorf("the refusal for an unreadable version reads as a release called nothing:\n%v", RexVersionRefusal(gk, got))
	}
	// And the ROUTED arm is untouched: codex still gets its own sentence and
	// its own probes, which is what keeps this from being one message with a
	// name swapped into it.
	skillsDriftVersion = func(string) string { return "codex-cli 0.154.0" }
	cxGot, cxOK := RexVersionDrift(cx)
	if cxOK {
		t.Fatal("codex's version pin stopped refusing another release")
	}
	cxErr := RexVersionRefusal(cx, cxGot).Error()
	if !strings.Contains(cxErr, "0076-trust-ask.probe.sh") || !strings.Contains(cxErr, "ADR 0076 D9") {
		t.Errorf("codex's version refusal lost its own route sentence:\n%s", cxErr)
	}
	if strings.Contains(cxErr, "0076-grok-hook.probe.sh") {
		t.Errorf("codex's version refusal names grok's probe:\n%s", cxErr)
	}
	// claude carries no pin at all, and that is still not an omission: its
	// record rests on nothing but its own output (D9's realization note).
	skillsDriftVersion = func(string) string { return "claude 9.9.9" }
	if _, ok := RexVersionDrift(cl); !ok {
		t.Error("claude was refused over a version it declares no pin on")
	}
}

// ─── the surface (ADR 0076 D12: `posse runtime check` shows the row) ─────────

func TestQAGrokPreconditionIsVisibleToTheOperatorBeforeALaunch(t *testing.T) {
	// The reading an operator can get WITHOUT spending a launch, which is
	// what makes the refusal actionable rather than a surprise. The row
	// itself is rendered in cmd/posse (the Rex block under MultiplexerLine);
	// what is pinned here is that the two facts the row needs — the reading
	// and the line — are available and name each other.
	grokFixtureHome(t, "[ui.notifications]\nprogram_status = \"auto\"\n")
	rt := builtinRuntime(t, "grok")
	read, ok := RexPreconditionMet(rt)
	if ok {
		t.Fatal("the fixture says `auto` and the reading admitted it")
	}
	if !strings.Contains(read, "auto") || !strings.Contains(read, "config.toml") {
		t.Errorf("the row's reading does not say what was read where: %q", read)
	}
	// One spelling of the line, shared by the refusal, the warning and the
	// row — so none of them can tell the operator to add something another
	// one does not read.
	for _, want := range []string{"[ui.notifications]", "program_status", GrokProgramStatusOn} {
		if !strings.Contains(GrokProgramStatusLine, want) {
			t.Errorf("GrokProgramStatusLine does not carry %q: %q", want, GrokProgramStatusLine)
		}
	}
	if !strings.Contains(RexPreconditionRefusal(rt, read).Error(), strings.SplitN(GrokProgramStatusLine, "\n", 2)[0]) {
		t.Error("the refusal does not quote the one shared spelling of the line")
	}
}

// ─── the ORDER of the three readings, which is also a decision ──────────────
//
// ranger-base-obaim. Both call sites carry it in a comment calling it "THE
// ORDER IS THE DECISION, not an optimisation of it: a grok the key was not
// measured on falsifies the measurement the precondition rests on", and no
// pin held it. Every precondition arm above fixes the version AT
// `rex_verified` (grokRexBackend, and the two t.Fatal guards inside the
// dispatch arm say so in as many words), and every version arm seeds a config
// that carries the line — so the one box state where the order is observable,
// BOTH readings refusing at once, was never driven.
//
// WHAT THE INVERSION COSTS, MEASURED 2026-11-07 with the two blocks swapped at
// dispatch.go's call site, over a grok reporting a release the key was never
// measured on whose config also lacks the line:
//
//	as landed  RexVersionRefusal     — names the release read, the release it
//	                                   was measured on, and `re-measure the
//	                                   key and move rex_verified:`
//	inverted   RexPreconditionRefusal — "ADD THE LINE ... program_status =
//	                                   \"on\"", and it NEVER NAMES THE
//	                                   VERSION AT ALL
//
// So inverted, the operator is told to edit their config on a release nobody
// measured that key on — which is the admission D13's order exists to refuse
// — and is told nothing about the version. The refusals do not compose: each
// returns, so exactly one is ever seen.
//
// grokDriftedRelease is a release this instance's reading was never taken on.
// It is the same spelling the version pin above uses, so the two arms cannot
// disagree about what drift looks like.
const grokDriftedRelease = "grok 1.0.70 (deadbeefcafe)"

// grokRexBothRefusing is grokRexBackend's board with BOTH readings failing:
// a config with the table present and `program_status` absent — not an empty
// home, so the arm cannot be satisfied by the absent-FILE reading — and the
// version seam re-pinned at a drifted release. grokPinVersion is called a
// second time on purpose: its cleanup restores whatever the previous call set,
// so the two nest.
func grokRexBothRefusing(t *testing.T) (*HerdrBackend, *Runtime) {
	t.Helper()
	b := grokRexBackend(t, "[cli]\nmodel = \"grok-4\"\n\n[ui.notifications]\nsound = false\n")
	rt := builtinRuntime(t, "grok")
	grokPinVersion(t, rt, grokDriftedRelease)
	// The board's own control: BOTH readings have to be refusing, or this is
	// one of the single-reading arms above wearing another name.
	if got, ok := RexVersionDrift(rt); ok {
		t.Fatalf("the version seam reads %q and admits it, so the order is not observable on this board", got)
	}
	if read, ok := RexPreconditionMet(rt); ok {
		t.Fatalf("the fixture config was admitted by the precondition (%q), so the order is not observable on this board", read)
	}
	return b, rt
}

// wantGrokVersionRefusalAndNotThePrecondition is the assertion both call sites
// share: the message is the VERSION one. The positive half is the release —
// the precondition's refusal never names a version at all, so naming it is
// decisive on its own — and the negative half is the remedy, because that is
// what the operator would act on.
func wantGrokVersionRefusalAndNotThePrecondition(t *testing.T, where, msg string, rt *Runtime) {
	t.Helper()
	for _, want := range []string{grokDriftedRelease, rt.RexVerified, "re-measure the key", grokProgramStatusProbe} {
		if !strings.Contains(msg, want) {
			t.Errorf("%s: with both readings refusing, the message does not name %q — the precondition was read before the version, so the operator is told nothing about the release their grok actually is (ADR 0076 D13):\n%s",
				where, want, msg)
		}
	}
	// The remedy that must NOT be reached: adding the line is the fix for a
	// box whose RELEASE is the one the key was measured on, and on any other
	// release it is advice to act on a measurement nobody retook.
	for _, never := range []string{"ADD THE LINE", GrokProgramStatusLine} {
		if strings.Contains(msg, never) {
			t.Errorf("%s: with both readings refusing, the operator is sent to %q on a release the key was never measured on — the two blocks are in the wrong order (ADR 0076 D13: a grok the key was not measured on falsifies the measurement the precondition rests on):\n%s",
				where, never, msg)
		}
	}
}

// CALL SITE 1: the dispatch loop (dispatch.go, above the argv ladder).
func TestQAGrokVersionIsReadBeforeThePreconditionInTheDispatchLoop(t *testing.T) {
	b, rt := grokRexBothRefusing(t)
	d := newTestDispatcher(t, b)
	_, fake := b, fakeDirOf(t)
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: t.TempDir()}
	_, err := d.launchSession(is, "ranger", "ranger-x-a-1", "grok", "fast", func() string { return "work prompt" }, nil, false)
	if err == nil {
		t.Fatal("a grok launch was allowed over a board on which BOTH the version pin and the precondition refuse")
	}
	wantGrokVersionRefusalAndNotThePrecondition(t, "dispatch", err.Error(), rt)
	// Unchanged either way, and asserted so that a reordering cannot be
	// excused as harmless: nothing is claimed, created or typed on either
	// order, because both readings are a declaration and one ReadFile.
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("the bead was claimed before the readings:\n%s", log)
	}
	// The same verb list the precondition's own dispatch arm uses, and for
	// its reason: launchSession reads the board (`ls`, `whoami`) before any
	// gate, so what must not happen is a seat being created or typed into.
	for _, c := range rexCalls(t) {
		for _, verb := range []string{" new ", " send ", " send-key ", " kill "} {
			if strings.Contains(" "+c+" ", verb) {
				t.Errorf("a refused launch ran rex%s— nothing may be created or typed before the readings: %s", verb, c)
			}
		}
	}
}

// CALL SITE 2: planLaunch (herdrback.go), with BOTH arms of the asymmetry —
// the refusal and the interactive DEGRADED warning carry the same order, and
// the warning is the arm an operator at the keyboard actually reads.
func TestQAGrokVersionIsReadBeforeThePreconditionInPlanLaunch(t *testing.T) {
	t.Run("a bead-carrying launch refuses on the version", func(t *testing.T) {
		b, rt := grokRexBothRefusing(t)
		_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger",
			Runtime: "grok", Bead: "ranger-base-x"})
		if err == nil {
			t.Fatal("planLaunch planned a grok launch over a board on which BOTH readings refuse")
		}
		wantGrokVersionRefusalAndNotThePrecondition(t, "planLaunch", err.Error(), rt)
		if warn := warnBuf(t, b).String(); warn != "" {
			t.Errorf("a refused launch also warned — the asymmetry is refuse OR warn, never both:\n%s", warn)
		}
	})
	t.Run("the operator at the keyboard is warned about the version", func(t *testing.T) {
		b, rt := grokRexBothRefusing(t)
		if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger",
			Runtime: "grok"}); err != nil {
			t.Fatalf("an interactive launch was refused rather than warned: %v", err)
		}
		warn := warnBuf(t, b).String()
		if !strings.Contains(warn, "DEGRADED") {
			t.Fatalf("an interactive grok launch on an unmeasured release proceeded without saying so:\n%s", warn)
		}
		wantGrokVersionRefusalAndNotThePrecondition(t, "planLaunch interactive", warn, rt)
	})
}
