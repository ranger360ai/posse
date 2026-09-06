package posse

// ADR 0053 — `posse new --model <id>` is an explicit crew-session canary.
// The bead is ranger-base-1oyio; these are its pins, in the ADR's own
// verification order: the companions and the id, the rendered line, the
// record, the listing, the relaunch, the recovery command, and an ordinary
// launch left byte-for-byte alone.
//
// The load-bearing arm is TestExactModelSkipsTierSubstitution: everything
// else here would still pass if posse quietly launched the tier's own model
// under a `model:` record, which is precisely the failure D3 names. It is
// written as two arms over ONE fixture — the same missing model, one launch
// with --model and one without — so a "green" that measured nothing is
// impossible: the control arm has to FALL for the canary arm's refusal to
// fall to mean anything.

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// astraID is the operator's first canary (ADR 0053 D5). Spelled here once so
// a pin that stops naming it is visible as a diff rather than as a rename.
const astraID = "gpt-6-astra"

// canaryOpts is the ADR's target invocation as options:
//
//	posse new <name> --agent <p> --runtime codex --tier strong --model gpt-6-astra
func canaryOpts(name, agent string) NewSessionOpts {
	return NewSessionOpts{Name: name, Agent: agent, Runtime: "codex", Tier: TierStrong, Model: astraID, Crew: true}
}

// ─── D1: the companions, and the id ──────────────────────────────────────────

func TestExactModelCompanionRefusals(t *testing.T) {
	t.Parallel()
	full := func(mut func(*NewSessionOpts)) NewSessionOpts {
		o := canaryOpts("c", "architect")
		mut(&o)
		return o
	}
	for _, c := range []struct {
		name string
		o    NewSessionOpts
		want string // "" = must be accepted
	}{
		{"the ADR's own invocation", canaryOpts("c", "architect"), ""},
		{"no model at all is every other launch", NewSessionOpts{Name: "c"}, ""},
		{"no --agent", full(func(o *NewSessionOpts) { o.Agent = "" }), "needs --agent"},
		{"no --runtime", full(func(o *NewSessionOpts) { o.Runtime = "" }), "needs an explicit --runtime"},
		{"no --tier", full(func(o *NewSessionOpts) { o.Tier = "" }), "needs an explicit --tier"},
		{"a space", full(func(o *NewSessionOpts) { o.Model = "gpt-6 astra" }), "is not one token"},
		{"a tab", full(func(o *NewSessionOpts) { o.Model = "gpt-6\tastra" }), "is not one token"},
		{"a newline", full(func(o *NewSessionOpts) { o.Model = "gpt-6\nastra" }), "is not one token"},
		{"a trailing newline", full(func(o *NewSessionOpts) { o.Model = astraID + "\n" }), "is not one token"},
		{"a NUL", full(func(o *NewSessionOpts) { o.Model = "gpt-6\x00astra" }), "control character"},
		{"an escape", full(func(o *NewSessionOpts) { o.Model = "gpt-6\x1b[31mastra" }), "control character"},
		{"a lone continuation byte", full(func(o *NewSessionOpts) { o.Model = "gpt-6-\x80stra" }), "not valid UTF-8"},
		// A quote is not refused: shell quoting carries it, and refusing
		// characters a provider might legitimately use would be posse
		// judging model ids, which D5 says it does not do.
		{"a quote is carried, not refused", full(func(o *NewSessionOpts) { o.Model = "gpt-6'astra" }), ""},
	} {
		err := CheckExactModel(c.o)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: refused with %v, want accepted", c.name, err)
		case c.want != "" && err == nil:
			t.Errorf("%s: accepted, want a refusal naming %q", c.name, c.want)
		case c.want != "" && !strings.Contains(err.Error(), c.want):
			t.Errorf("%s: refusal %q does not name %q", c.name, err, c.want)
		}
	}
}

// The refusals are the LAUNCH's, not only the flag parser's: every path into
// a session goes through planLaunch, and a recreate does not re-type a flag.
// Nothing may exist on disk afterwards — no workspace, no meta, no worktree
// (ADR 0053 D1).
func TestExactModelRefusesBeforeAnythingExists(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	canaryPersona(t, b, "architect")
	o := canaryOpts("bad", "architect")
	o.Runtime = "" // the companion the launch itself must miss
	o.Dir = t.TempDir()

	if err := b.CreateSession(o); err == nil || !strings.Contains(err.Error(), "explicit --runtime") {
		t.Fatalf("planLaunch must make D1's refusal too: %v", err)
	}
	if _, ok := b.readMeta("bad"); ok {
		t.Error("a refused canary left a session record behind")
	}
	if log := calls(t, fake); strings.Contains(log, "bad") {
		t.Errorf("a refused canary reached herdr:\n%s", log)
	}
}

// D1's last precondition, and the one that is about the RENDER rather than
// about argv: a template with no {model} would drop the id silently and open
// the session on the tier's own model.
func TestExactModelRefusesATemplateWithNoModelSlot(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	canaryPersona(t, b, "architect")
	os.MkdirAll(b.App.RuntimesDir(), 0o755)
	// A template-only runtime that takes a prompt and nothing else. It gets
	// the default model_flag (--model %s) — so this is exactly the case the
	// flag check alone cannot see.
	os.WriteFile(filepath.Join(b.App.RuntimesDir(), "slotless.yaml"),
		[]byte("command: slotless --rules \"$(cat {file})\"\nmodel_strong: slot-1\n"), 0o644)

	o := canaryOpts("nc", "architect")
	o.Runtime, o.Dir = "slotless", t.TempDir()
	// A template-only runtime has no probe record, so ADR 0002 §4 refuses
	// the launch for parity long before the render. Waiving that is what
	// lets this test measure the thing it is about — and the waiver is what
	// makes the refusal below provably the MODEL's, not the wall's.
	o.AllowDegraded = true
	err := b.CreateSession(o)
	if err == nil || !strings.Contains(err.Error(), "does not carry --model "+astraID) {
		t.Fatalf("a template with no {model} must refuse the canary: %v", err)
	}
	if _, ok := b.readMeta("nc"); ok {
		t.Error("the refusal left a session record behind")
	}
	// The control: the same runtime with a {model} slot launches, which is
	// what makes the refusal above a statement about the slot and not about
	// the runtime being a template one.
	os.WriteFile(filepath.Join(b.App.RuntimesDir(), "slotted.yaml"),
		[]byte("command: slotted {model} --rules \"$(cat {file})\"\nmodel_strong: slot-1\n"), 0o644)
	ok := canaryOpts("yc", "architect")
	ok.Runtime, ok.Dir = "slotted", t.TempDir()
	ok.AllowDegraded = true
	if err := b.CreateSession(ok); err != nil {
		t.Fatalf("the same launch with a {model} slot must succeed: %v", err)
	}
}

// ─── D2: the rendered line ───────────────────────────────────────────────────

// The canary launch is a PERSONA launch: the exact id replaces the model and
// NOTHING else moves — the PID delivery, the gates prefix, the skills, the
// unattended mode and codex's own flags are the ordinary render.
func TestExactModelRendersTheCodexLine(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	canaryPersona(t, b, "architect")
	dir := t.TempDir()

	o := canaryOpts("architect-astra", "architect")
	o.Dir = dir
	if err := b.CreateSession(o); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	log := launchLog(t, b.App, fake)

	// The id, rendered through codex's OWN model flag and the existing shell
	// quoting — no second rendering, no raw command (D2).
	if !strings.Contains(log, "GATES codex -c model='"+astraID+"' ") {
		t.Errorf("the typed line does not carry the exact model through codex's flag:\n%s", log)
	}
	// The whole point of D3: the tier's own id is nowhere near this launch.
	if strings.Contains(log, codexModels[TierStrong]) {
		t.Errorf("the typed line still names the tier's model %s:\n%s", codexModels[TierStrong], log)
	}
	// The ordinary persona path, unchanged around it: the PID is delivered
	// as developer instructions, codex's unattended mode is on, and the
	// gates prefix is in front (calls() asserts that last one for us).
	for _, want := range []string{`-a never`, CodexFleetFlags, `-c developer_instructions="$(cat `} {
		if !strings.Contains(log, want) {
			t.Errorf("the canary line lost part of the ordinary persona render (%q):\n%s", want, log)
		}
	}
	// D2's last clause: posse does not touch reasoning effort — the operator's
	// codex config still decides it, so no launch of ours may name one.
	if strings.Contains(log, "reasoning") {
		t.Errorf("the canary line names a reasoning setting; ADR 0053 leaves that to the operator's config:\n%s", log)
	}
}

// ─── D3: no substitution ─────────────────────────────────────────────────────

// Two arms over one fixture. The control arm proves the fixture really does
// reach the availability verdict; the canary arm proves --model is what
// stops that verdict being reached about a model nobody launched. Without
// the control, a green here would be indistinguishable from a preflight that
// was never going to fire.
//
// The control used to assert a FALL. ADR 0003 §3 removed automatic
// substitution (ranger-base-hv2zr), so what the fixture now produces is the
// loud line and nothing else — which is still the thing D3 asks the canary
// to skip, and still a fixture that has to be shown to bite.
func TestExactModelSkipsTierSubstitution(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	pfPersona(t, b, "architect", TierStrong)
	// claude's strong id is gone from the account's catalog: an ordinary
	// launch at strong says so, loudly.
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5")

	// CONTROL — no --model: the verdict is reached and printed.
	if err := b.CreateSession(NewSessionOpts{Name: "ctl", Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatalf("control launch: %v", err)
	}
	if !strings.Contains(warn.String(), "tier strong wants claude-fable-5-1 — unavailable on this account") {
		t.Fatalf("the fixture did not reach the tier verdict — this test would measure nothing: %q", warn.String())
	}
	// And even the control does not move: availability chooses nothing.
	if ctl, _ := b.readMeta("ctl"); ctl.Tier != TierStrong || ctl.Runtime != "claude" {
		t.Fatalf("the control's pair moved: %+v", ctl)
	}

	// CANARY — the same account, the same missing id, an exact model named.
	warn.Reset()
	o := NewSessionOpts{Name: "canary", Agent: "architect", Runtime: DefaultRuntime, Tier: TierStrong, Model: "claude-6-astra", Dir: t.TempDir(), Crew: true}
	if err := b.CreateSession(o); err != nil {
		t.Fatalf("canary launch: %v", err)
	}
	m, ok := b.readMeta("canary")
	if !ok {
		t.Fatal("no canary meta")
	}
	if m.Tier != TierStrong {
		t.Errorf("the canary's tier moved: %q", m.Tier)
	}
	log := launchLog(t, b.App, fake)
	if !strings.Contains(log, "--model 'claude-6-astra'") {
		t.Errorf("the canary line does not name the exact model:\n%s", log)
	}
	// The line the operator reads instead of a preflight verdict: it says
	// what is being asked and that nothing will substitute.
	if !strings.Contains(warn.String(), "EXACT model claude-6-astra") || !strings.Contains(warn.String(), "substitution is skipped") {
		t.Errorf("the canary launch did not say it was skipping substitution: %q", warn.String())
	}
	// And it does NOT print a verdict about a model nobody launched — the
	// control above proved that same fixture reaches one.
	if strings.Contains(warn.String(), "unavailable on this account") {
		t.Errorf("the canary printed the tier's availability verdict: %q", warn.String())
	}
}

// ─── D4: the record, the listing, the relaunch, the recovery line ────────────

func TestExactModelRecordListingAndRecovery(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	canaryPersona(t, b, "architect")

	o := canaryOpts("architect-astra", "architect")
	o.Dir = t.TempDir()
	if err := b.CreateSession(o); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	// An ordinary session beside it, from the same binary and the same
	// backend: every "unchanged" claim below is measured against this row
	// rather than against a remembered shape.
	if err := b.CreateSession(NewSessionOpts{Name: "plain", Agent: "architect", Dir: t.TempDir(), Crew: true}); err != nil {
		t.Fatalf("ordinary launch: %v", err)
	}

	m, ok := b.readMeta("architect-astra")
	if !ok {
		t.Fatal("no meta")
	}
	if m.Model != astraID {
		t.Errorf("meta model = %q, want %q", m.Model, astraID)
	}
	raw, err := os.ReadFile(b.metaPath("architect-astra"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\nmodel: "+astraID+"\n") {
		t.Errorf("the record does not carry model: on its own line:\n%s", raw)
	}
	// An ordinary record is what it was: no key at all, not an empty one.
	plainRaw, err := os.ReadFile(b.metaPath("plain"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plainRaw), "model:") {
		t.Errorf("an ordinary session's record grew a model: key:\n%s", plainRaw)
	}
	if pm, _ := b.readMeta("plain"); pm.Model != "" {
		t.Errorf("an ordinary record reads back a model: %q", pm.Model)
	}

	// The listing tag (D4). It is never suppressed, and it names the tier
	// the operator typed.
	if got := b.App.RuntimeTierModelTag("codex", TierStrong, astraID); got != "@codex/strong="+astraID {
		t.Errorf("tag = %q", got)
	}
	if got := b.App.RuntimeTierModelTag(DefaultRuntime, DefaultTier, astraID); got != "@claude/strong="+astraID {
		t.Errorf("the default pair must still show the model: %q", got)
	}
	// No model = the tag posse has always rendered, including its suppression.
	for _, c := range []struct{ rt, tier, want string }{
		{DefaultRuntime, DefaultTier, ""},
		{"codex", TierStrong, "@codex/strong"},
		{"claude", TierFast, "@claude/fast"},
	} {
		if got := b.App.RuntimeTierModelTag(c.rt, c.tier, ""); got != c.want {
			t.Errorf("RuntimeTierModelTag(%q,%q,\"\") = %q, want %q (an ordinary session's tag is unchanged)", c.rt, c.tier, got, c.want)
		}
	}
	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list.String(), "🎭architect@codex/strong="+astraID) {
		t.Errorf("posse list does not show the canary:\n%s", list.String())
	}
	if !strings.Contains(list.String(), "🎭architect  ") {
		t.Errorf("the ordinary session's line changed:\n%s", list.String())
	}

	// The recreate: a refresh is the same session, so the canary rides it.
	rec := RecreateOpts(m)
	if rec.Model != astraID || rec.Runtime != "codex" || rec.Tier != TierStrong {
		t.Errorf("RecreateOpts dropped the canary: %+v", rec)
	}
	// And it re-enters planLaunch carrying all three companions, so the
	// recreate is not refused by D1.
	if err := CheckExactModel(rec); err != nil {
		t.Errorf("a recreate of a canary session refuses its own launch: %v", err)
	}
	plan, err := b.planLaunch(rec)
	if err != nil {
		t.Fatalf("replanning the canary: %v", err)
	}
	if plan.Model != astraID || !strings.Contains(plan.Cmd, "-c model='"+astraID+"'") {
		t.Errorf("the recreate's plan lost the model: %q / %q", plan.Model, plan.Cmd)
	}
	if d := describePlan(rec, plan); !strings.Contains(d, "model "+astraID) {
		t.Errorf("the relaunch receipt does not name the model: %q", d)
	}

	// The recovery line has to be pasteable, which under D1 means it carries
	// --runtime and --tier beside --model.
	rc := RecoverCommand(m)
	for _, want := range []string{"--agent architect", "--runtime codex", "--tier strong", "--model " + astraID} {
		if !strings.Contains(rc, want) {
			t.Errorf("RecoverCommand missing %q: %s", want, rc)
		}
	}
	if pm, _ := b.readMeta("plain"); strings.Contains(RecoverCommand(pm), "--model") {
		t.Errorf("an ordinary session's recovery line grew a --model: %s", RecoverCommand(pm))
	}

	// Relaunch retypes the canary, not the tier's model (D4: killing is what
	// ends the override, not a crash).
	if _, err := b.RelaunchAgent("architect-astra", 0); err != nil {
		t.Fatalf("RelaunchAgent: %v", err)
	}
	log := launchLog(t, b.App, fake)
	if strings.Count(log, "-c model='"+astraID+"'") < 2 {
		t.Errorf("the relaunch did not retype the exact model:\n%s", log)
	}
	if strings.Contains(log, codexModels[TierStrong]) {
		t.Errorf("the relaunch typed the tier's own model:\n%s", log)
	}
}

// A record written before `model:` existed reads back as an ordinary session
// and launches exactly as it did — the ADR's ASSUMED line, pinned.
func TestLegacyRecordWithoutModelIsUnchanged(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	canaryPersona(t, b, "architect")
	dir := t.TempDir()
	os.MkdirAll(b.metaDir(), 0o755)
	legacy := "name: old\nworkspace: w1\npane: p1\nemoji: 🪢\nagent: architect\nruntime: codex\ntier: strong\ndir: " + dir + "\ncrew: true\n"
	if err := os.WriteFile(b.metaPath("old"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta("old")
	if !ok || m.Model != "" {
		t.Fatalf("legacy record read back with a model: %+v", m)
	}
	// Rewritten by any pass that touches it, and still byte-for-byte itself.
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	back, err := os.ReadFile(b.metaPath("old"))
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != legacy {
		t.Errorf("a legacy record did not round-trip byte-for-byte:\nwant %q\ngot  %q", legacy, back)
	}
	plan, err := b.planLaunch(RecreateOpts(m))
	if err != nil {
		t.Fatalf("planning a legacy session: %v", err)
	}
	if !strings.Contains(plan.Cmd, "-c model='"+codexModels[TierStrong]+"'") {
		t.Errorf("a legacy session must launch on its tier's own model: %q", plan.Cmd)
	}
}

// canaryPersona writes a PID the wall fully realizes on codex, so these
// tests measure the model and not a parity refusal.
func canaryPersona(t *testing.T, b *HerdrBackend, name string) {
	t.Helper()
	os.MkdirAll(b.App.AgentsDir, 0o755)
	body := "---\nname: " + name + "\ndeny: [Bash(git push:*)]\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── D5: the override is a crew-session flag and nothing else ────────────────

// ADR 0053 D5 is a decision about what must NOT exist: no PID key, no runtime
// overlay key, no config key, no recipe field, no label rule and no dispatch
// flag. Nothing infers an exact model from a tier or from a persona — the
// operator types the id for every new canary session.
//
// Censused two ways, because a promise of absence that only reads one surface
// is a promise about that surface:
//
//  1. the STRUCTS a declaration would have to land in — a PID, a recipe and a
//     dispatch pass all carry their keys as fields, so a new key is a new
//     field and reflection sees it;
//  2. the READS — every one of those keys is loaded through YamlGet /
//     yamlGetLines / CfgGet, so a census of who asks a file for "model" names
//     any new declaration wherever its struct lives.
//
// The one legitimate reader is the session record's own (ADR 0053 D4).
func TestExactModelHasNoDeclarationSurface(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		what string
		v    any
	}{
		{"the PID (a persona may not carry a model)", AgentFile{}},
		{"a recipe", Recipe{}},
		{"a dispatch pass", Dispatcher{}},
	} {
		rt := reflect.TypeOf(c.v)
		for i := 0; i < rt.NumField(); i++ {
			if n := rt.Field(i).Name; n == "Model" || strings.HasPrefix(n, "Model") && n != "ModelFlag" {
				t.Errorf("%s grew a %s field — ADR 0053 D5 adds no declaration surface for an exact model; it is typed on `posse new --model` and recorded on the session", c.what, n)
			}
		}
	}

	// The reads. Anything asking a config, a PID or a runtime file for a
	// bare "model" key is a declaration surface by another name; the tier
	// map's model_<tier>/model_flag keys are ADR 0003's and are untouched.
	readRe := regexp.MustCompile(`(?:YamlGet|yamlGetLines|CfgGet)\([^,)]+,\s*"model"`)
	allowed := map[string]bool{
		// readMeta: the session record is the store of record (D4).
		"herdrback.go": true,
	}
	found := 0
	for _, dir := range []string{".", "../../cmd/posse"} {
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			src, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range readRe.FindAllString(string(src), -1) {
				found++
				if !allowed[e.Name()] {
					t.Errorf("%s reads a %q key — ADR 0053 D5: the only file that may declare an exact model is the session record (%s)", filepath.Join(dir, e.Name()), "model", m)
				}
			}
		}
	}
	// The census has to be able to SEE such a read, or its silence means
	// nothing: the one legitimate site is the proof that it can.
	if found != 1 {
		t.Errorf("the census found %d readers of a bare model: key, want exactly the session record's one — a scan that finds none is measuring nothing", found)
	}
}
