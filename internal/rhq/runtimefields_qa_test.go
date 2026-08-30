package rhq

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ADR 0017 §1's anti-drift device, and the only living copy of §3's field
// audit: every `Runtime` field is classified as **consumed** (a production
// caller changes behaviour on it), **display-by-design** (its contract IS
// the `runtime check` grid — provenance and measured-fact fields), or
// **internal**. A field added without an entry here fails this test, so a
// struct member can no longer arrive without somebody deciding who reads
// it. The named failure class is ranger-base-p84: `startup_wait:` was
// parsed, validated, documented, displayed and pinned by a test that
// asserted the GETTER, while no production caller read it — a lever that
// reads as connected and is not.
//
// WHAT THIS PROVES, exactly, so nobody reads more into a green than is
// there: that every field is classified, that no classification outlives
// its field, and that each named site is a production file which still
// mentions the field outside a `//` comment. It does NOT prove the mention
// is a live read — a field referenced only from dead code passes. Proving
// consumption is what the per-field consumer tests do (skills_cwd through
// RenderSkillsFor, unattended through the rendered line, project_config
// through CheckParityIn); this test is the census that says one must exist.
//
// The site list is what makes the entry falsifiable rather than a label:
// deleting or renaming the consumer file reds this without anyone
// remembering the ADR exists.
type runtimeFieldNote struct {
	class string
	sites []string // production files; a _test.go here is itself a failure
	// via names the accessors a site may read the field THROUGH, for a
	// field whose consumers never spell it: parity.go reads PIDVoid only as
	// PIDVoided(), runtimeprobe.go reads StartupWait only as Wait(). A site
	// satisfies the row by naming the field or any of these.
	via []string
	why string
}

const (
	fcConsumed = "consumed"
	fcDisplay  = "display-by-design"
	fcInternal = "internal"
)

// The audit, alphabetical by field. Verified by grep over production code
// 2026-08-30 (HEAD d635433 + this bead's keys); ADR 0017 §3's table is the
// dated snapshot of the same facts, and where the two disagree this file is
// the one that ran.
//
// Two rows moved since that snapshot, both away from INERT: StartupWait
// gained dispatch.go's agentWait (ranger-base-ze9p split it from the
// relaunch grace) and Interstitials gained DangerLine on both launch
// paths. CostAdapter left the struct entirely with the ADR 0012 D4 cost
// seam, which is why no row names it.
var runtimeFieldAudit = map[string]runtimeFieldNote{
	"Name":               {fcConsumed, []string{"agents.go", "cage.go", "dispatch.go"}, nil, "identity: the value every launch, cage and dispatch line keys on"},
	"Path":               {fcDisplay, []string{"runtimecheck.go"}, nil, "declaredBy — WHO declared each stage, the fact an onboarder reads a fallback off"},
	"Command":            {fcConsumed, []string{"agents.go"}, nil, "the launch template RenderCommandFor expands"},
	"Realize":            {fcConsumed, []string{"agents.go", "parity.go"}, nil, "PID rules → this CLI's own flags; parity's Realized/Enforced"},
	"Builtin":            {fcConsumed, []string{"runtime.go", "runtimecheck.go"}, nil, "LoadRuntime returns a built-in ahead of any yaml; the grid says so"},
	"Models":             {fcConsumed, []string{"runtime.go", "modelavail.go"}, nil, "tier → model id for {model} and the availability preflight"},
	"ModelFlag":          {fcConsumed, []string{"runtime.go"}, nil, "the printf form ModelText renders {model} with"},
	"NoGateShell":        {fcConsumed, []string{"gates.go", "parity.go"}, nil, "leaves SHELL/GROK_SHELL alone, and costs the L1 verdict for it"},
	"Skills":             {fcConsumed, []string{"agents.go", "parity.go"}, nil, "what {skills} renders to, and whether a skills: PID can launch here at all"},
	"SkillsCwd":          {fcConsumed, []string{"skills.go", "parity.go"}, nil, "materializes <cwd>/.agents/skills/<name>; the links are the binding"},
	"SelfSandbox":        {fcConsumed, []string{"herdrback.go", "parity.go"}, nil, "do not seatbelt-wrap — macOS refuses to nest — and degrade cage: seatbelt honestly"},
	"ProjectConfig":      {fcConsumed, []string{"parity.go"}, nil, "the session-dir files ProjectConfigTrust guards: a repo→box channel no PID sits in front of"},
	"ProjectConfigKeys":  {fcConsumed, []string{"parity.go"}, nil, "narrows that check to top-level JSON keys; empty keeps the whole-file predicate"},
	"Unattended":         {fcConsumed, []string{"runtime.go", "agents.go"}, nil, "EnsureUnattended puts the flag back on a line a hand-written command: rendered without it"},
	"PIDVoid":            {fcConsumed, []string{"runtime.go", "herdrback.go", "runtimeprobe.go"}, []string{"PIDVoided"}, "PIDVoided REFUSES a rendered line naming a flag that makes this CLI ignore the PID — what would open is a different session, not a degraded persona"},
	"CageCred":           {fcConsumed, []string{"cage.go"}, nil, "the env var a containerised session authenticates with; absent refuses cage: container"},
	"Egress":             {fcConsumed, []string{"egress.go"}, nil, "always added to a caged PID's allowlist — a cage that cannot reach its model is not isolated, it is offline"},
	"Prompt":             {fcConsumed, []string{"dispatch.go", "herdrback.go"}, nil, "argv vs typed work-prompt delivery"},
	"StartupWait":        {fcConsumed, []string{"dispatch.go", "promptready.go", "runtimeprobe.go"}, []string{"Wait()"}, "agentWait's per-runtime detection patience (ranger-base-ze9p retired the p84 inertness)"},
	"Record":             {fcConsumed, []string{"dispatch.go"}, nil, "recordClause and the ✓ suppression on a settle with the bead still open"},
	"RecordWhy":          {fcDisplay, []string{"runtimecheck.go"}, nil, "the measurement behind a trusted record — provenance, so a reader tells a promotion from an assumption"},
	"NativeRules":        {fcDisplay, []string{"runtimecheck.go"}, nil, "declared, never suppressed (ranger-base-00f): the grid names the other voice in the session"},
	"Interstitials":      {fcConsumed, []string{"interstitial.go", "runtimepreflight.go"}, nil, "DangerUnsilenced/DangerLine on both launch paths; the grid probes each key"},
	"StateDirs":          {fcConsumed, []string{"seatbelt.go"}, nil, "joins the L2 writable set, or the CLI re-runs its first-run flow every launch"},
	"EnvRequired":        {fcConsumed, []string{"runtimepreflight.go"}, nil, "checked by NAME at launch preflight; a missing one refuses"},
	"TurnOutcomeAdapter": {fcConsumed, []string{"turnfailure.go"}, nil, "which reader sees what this CLI's own first turn did — an exhausted account vs an agent that skipped its bead"},
}

func TestEveryRuntimeFieldIsClassified(t *testing.T) {
	rtype := reflect.TypeOf(Runtime{})
	seen := map[string]bool{}
	for i := 0; i < rtype.NumField(); i++ {
		f := rtype.Field(i)
		seen[f.Name] = true
		note, ok := runtimeFieldAudit[f.Name]
		if !ok {
			t.Errorf("Runtime.%s is unclassified — add it to runtimeFieldAudit as %s, %s or %s, and name the production file that reads it. A field nobody classified is a lever nobody decided the consumer of (ADR 0017 §1).",
				f.Name, fcConsumed, fcDisplay, fcInternal)
			continue
		}
		switch note.class {
		case fcConsumed, fcDisplay, fcInternal:
		default:
			t.Errorf("Runtime.%s: class %q is not one of %s/%s/%s", f.Name, note.class, fcConsumed, fcDisplay, fcInternal)
		}
		if strings.TrimSpace(note.why) == "" {
			t.Errorf("Runtime.%s: a classification with no why is a label — name the consumer or the contract", f.Name)
		}
		if len(note.sites) == 0 {
			t.Errorf("Runtime.%s: no site named. Even an %s field is read somewhere, and the site is what makes the row falsifiable.", f.Name, fcInternal)
		}
		for _, site := range note.sites {
			if strings.HasSuffix(site, "_test.go") {
				t.Errorf("Runtime.%s names %s: a test is not a consumer. That is the p84 shape exactly — a getter pinned by a test and read by no production caller.", f.Name, site)
				continue
			}
			b, err := os.ReadFile(site)
			if err != nil {
				t.Errorf("Runtime.%s names %s, which does not exist: %v", f.Name, site, err)
				continue
			}
			if !mentionsOutsideComments(string(b), f.Name, note.via...) {
				t.Errorf("Runtime.%s is no longer mentioned in %s outside a comment (nor as %v) — either the consumer moved (repoint the row) or the field went inert (reclassify it).", f.Name, site, note.via)
			}
		}
	}
	var stale []string
	for name := range runtimeFieldAudit {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("runtimeFieldAudit classifies fields Runtime no longer has: %s. A classification that outlives its field is how a dated snapshot becomes wrong quietly.", strings.Join(stale, ", "))
	}
}

// mentionsOutsideComments is deliberately crude — everything from `//` to
// end of line is dropped, string literals included. It can only ever be
// STRICTER than the truth (a reference hiding behind a `//` inside a string
// reads as absent), so its errors are false reds a maintainer fixes by
// naming another site, never false greens.
func mentionsOutsideComments(src, name string, via ...string) bool {
	for _, ln := range strings.Split(src, "\n") {
		if i := strings.Index(ln, "//"); i >= 0 {
			ln = ln[:i]
		}
		if strings.Contains(ln, name) {
			return true
		}
		for _, v := range via {
			if strings.Contains(ln, v) {
				return true
			}
		}
	}
	return false
}

// The same drift, one surface over: `posse runtime check`'s onboarding
// footer is the list an onboarder fills the grid from, and it is
// hand-maintained beside runtimeYamlKeys(), which is generated from
// nothing. It had already fallen behind — `turn_outcome:` shipped in
// ranger-base-02zr and never reached the footer, so a key posse reads was
// undiscoverable from the one screen that exists to list them.
//
// The unknown-key warning keeps runtimeYamlKeys() honest about what LOADS.
// This keeps the footer honest about what an operator can FIND.
func TestOnboardingFooterNamesEveryDeclarableKey(t *testing.T) {
	a := checkApp(t)
	rt := writeRuntime(t, a, "footercli", "command: footercli --sys {file}\n")
	var b strings.Builder
	a.RuntimeCheck(rt, Herdr{Bin: "no-such-herdr-binary"}, &b)
	out := b.String()

	const lead, tail = "onboarding a runtime is filling this grid", "(ADR 0012 D4)."
	i := strings.Index(out, lead)
	j := strings.Index(out, tail)
	if i < 0 || j < i {
		t.Fatalf("the onboarding footer is not on the screen any more (%q … %q):\n%s", lead, tail, out)
	}
	footer := out[i:j]

	for _, k := range runtimeYamlKeys() {
		want := k + ":"
		// The model_<tier>: family is expanded from Tiers rather than
		// spelled, in the key list and in the footer alike.
		if strings.HasPrefix(k, "model_") && k != "model_flag" {
			want = "model_<tier>:"
		}
		if !strings.Contains(footer, want) {
			t.Errorf("runtime check's onboarding footer never names %s — a key posse reads and no screen lists is a declaration an operator cannot find:\n%s", want, footer)
		}
	}
}
