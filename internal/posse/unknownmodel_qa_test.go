//go:build !posse_arm2 && !posse_arm3

package posse

// `unknown_model:` — what a CLI does with a model id it does not know, and
// the two operator-visible sentences that are chosen on it (ranger-base-jzm04).
//
// THE DEFECT THESE PINS ARE WRITTEN AGAINST, measured 2026-09-09 on this
// box: a canary launched as
//
//	posse new <name> --agent <persona> --runtime grok --tier strong --model grok-4.7
//
// came up clean on grok 1.0.5 with its composer border reading "Grok 4.6
// (high)". grok-4.7 does not exist — the operator confirmed it — so nothing
// was rolling out and nothing refused: the CLI ran its own default for an id
// it did not recognise and said nothing. posse printed "the provider is
// asked, not the catalog, and its refusal is the canary's answer" anyway,
// and `posse list` printed `@grok/strong=grok-4.7` beside a session that was
// answering as Grok 4.6. Both sentences were claims about argv wearing the
// clothes of a measurement.
//
// So the fix is a DECLARATION, not a name-keyed branch (ADR 0013 §7, ADR
// 0017 §3): the launch clause and the listing mark are chosen on
// Runtime.UnknownModel, which is measured per runtime and overlayable per
// box. These pins are two-armed for that reason — a sentence that is right
// for grok and wrong for codex is exactly what shipped, so every arm here
// has the other runtime's answer beside it.
//
// What is NOT here, deliberately: reading the model off the session's own
// pane, which is what would let posse print "requested grok-4.7, running
// grok-4.6". That is a second name-keyed pane observation, and ADR 0013 §7
// grants that shape to the pane-MODE reading alone, by name. The bead says
// so and hands the decision to the architect lane.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRuntimeFile is writeRuntime without the load: the arms below want the
// yaml on disk and then ask LoadRuntime for its error, which writeRuntime
// turns into a t.Fatal.
func writeRuntimeFile(t *testing.T, a *App, name, body string) {
	t.Helper()
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// unknownModelArms is the three declarations as they stand on the built-ins,
// read out of the built-ins rather than spelled here: a release that moves
// one moves this table with it, and an arm that stops being reachable
// (grok gaining a refusal, say) reds in the loop below rather than passing
// over a value nobody has.
func unknownModelArms(t *testing.T, a *App) map[string]*Runtime {
	t.Helper()
	out := map[string]*Runtime{}
	for _, n := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(n)
		if err != nil {
			t.Fatalf("LoadRuntime(%q): %v", n, err)
		}
		out[n] = rt
	}
	if out["grok"].UnknownModel != UnknownModelSwap {
		t.Fatalf("grok declares unknown_model: %q — these arms are written against the measured swap (ranger-base-jzm04); re-measure before changing it", out["grok"].UnknownModel)
	}
	if out["codex"].UnknownModel != UnknownModelCarry {
		t.Fatalf("codex declares unknown_model: %q — the contrast arm needs the measured carry", out["codex"].UnknownModel)
	}
	if out["claude"].UnknownModel != "" {
		t.Fatalf("claude declares unknown_model: %q — the UNDECLARED arm needs a runtime nobody has measured; if one was measured, move this arm to a fixture rather than deleting it", out["claude"].UnknownModel)
	}
	return out
}

// ─── the launch line ─────────────────────────────────────────────────────────

// The clause an operator reads at launch is the runtime's own answer, and
// the three answers are three different sentences. The swap arm is the one
// the bead was filed for; the carry arm is what the old line said for
// everybody; the undeclared arm is what posse must say where nobody has
// measured, which is neither of the other two softened.
func TestExactModelClauseIsTheRuntimesOwnDeclaration(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	rts := unknownModelArms(t, b.App)

	for _, c := range []struct {
		runtime        string
		want, unwanted []string
	}{{
		runtime: "grok",
		want: []string{
			"running its OWN default and saying nothing",
			"is NOT the provider taking grok-4.7",
			ModelUncheckedMark,
			"unknown_model: " + UnknownModelSwap,
		},
		// The sentence that shipped. A launch on a CLI that cannot refuse
		// must not promise a refusal, and must not present the id as the
		// one being asked about.
		unwanted: []string{"refusal is the canary's answer", "UNMEASURED"},
	}, {
		runtime: "codex",
		want: []string{
			"takes an id it does not know to the provider",
			"refusal is the canary's answer",
			"unknown_model: " + UnknownModelCarry,
		},
		unwanted: []string{"OWN default", ModelUncheckedMark, "UNMEASURED"},
	}, {
		runtime: "claude",
		want: []string{
			"UNMEASURED here (unknown_model: unset)",
			"not yet evidence",
			"posse runtime check claude",
		},
		// Neither measured answer may be borrowed for a runtime nobody
		// measured — the whole point of the third clause.
		unwanted: []string{"refusal is the canary's answer", "declared unknown_model", ModelUncheckedMark},
	}} {
		line := ExactModelLine("canary", c.runtime, TierStrong, "grok-4.7", rts[c.runtime])
		t.Logf("%s: %s", c.runtime, line)
		// Every arm still says what D3 asks the line to say, and none of
		// them names the automatic substitution ADR 0003 §3 removed — the
		// two halves TestExactModelSkipsTierVerdict holds for the shipped
		// claude line, held here for all three declarations.
		for _, want := range append([]string{"EXACT model grok-4.7", "tier availability verdict is skipped"}, c.want...) {
			if !strings.Contains(line, want) {
				t.Errorf("%s's canary line does not say %q:\n%s", c.runtime, want, line)
			}
		}
		for _, no := range c.unwanted {
			if strings.Contains(line, no) {
				t.Errorf("%s's canary line says %q, which is another runtime's answer:\n%s", c.runtime, no, line)
			}
		}
		if m := namesTheRemovedSubstitution.FindString(line); m != "" {
			t.Errorf("%s's canary line names the removed automatic substitution (%q):\n%s", c.runtime, m, line)
		}
	}
	// A runtime that could not be loaded at all leaves the clause at the
	// loud default rather than at either measurement: the caller passing
	// nil is a path with no declaration to read, and silence there would be
	// the failure this dimension exists to remove.
	if line := ExactModelLine("canary", "mycli", TierStrong, "x-1", nil); !strings.Contains(line, "UNMEASURED here") {
		t.Errorf("with no runtime to read, the clause is not the loud default: %s", line)
	}
}

// The line is what the LAUNCH prints, not only what the renderer returns:
// the fix is worth nothing if planLaunch still hands the old text to the
// operator. One real canary launch per declaration, read off the warn
// stream (ADR 0053 D3 prints it where the tier verdict would go).
func TestACanaryLaunchPrintsItsRuntimesClause(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ runtime, want string }{
		{"grok", "running its OWN default and saying nothing"},
		{"codex", "refusal is the canary's answer"},
	} {
		b, _ := newTestBackend(t)
		var warn strings.Builder
		b.Warn = &warn
		canaryPersona(t, b, "architect")
		o := NewSessionOpts{
			Name: "architect-canary", Agent: "architect",
			Runtime: c.runtime, Tier: TierStrong, Model: "some-9.9",
			Dir: t.TempDir(), Crew: true,
		}
		if err := b.CreateSession(o); err != nil {
			t.Fatalf("%s canary launch: %v", c.runtime, err)
		}
		if !strings.Contains(warn.String(), "EXACT model some-9.9") {
			t.Fatalf("%s: the launch printed no exact-model line at all: %q", c.runtime, warn.String())
		}
		if !strings.Contains(warn.String(), c.want) {
			t.Errorf("%s's launch does not say %q:\n%s", c.runtime, c.want, warn.String())
		}
	}
}

// ─── the listing ─────────────────────────────────────────────────────────────

// `posse list` and the cockpit render one tag, so the mark is pinned on the
// tag and then on the row it reaches. The control is the same id on the two
// runtimes that do not swap: without it a green here would be a mark that
// never came off.
func TestExactModelListingMarksARuntimeThatSwaps(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	unknownModelArms(t, b.App)

	if got, want := b.App.RuntimeTierModelTag("grok", TierStrong, "grok-4.7"), "@grok/strong=grok-4.7"+ModelUncheckedMark; got != want {
		t.Errorf("grok canary tag = %q, want %q — the row presents an id nothing verified as the model that answered (ranger-base-jzm04)", got, want)
	}
	for _, c := range []struct{ runtime, want string }{
		{"codex", "@codex/strong=some-9.9"},
		{"claude", "@claude/strong=some-9.9"},
	} {
		if got := b.App.RuntimeTierModelTag(c.runtime, TierStrong, "some-9.9"); got != c.want {
			t.Errorf("%s canary tag = %q, want %q — only a runtime MEASURED to swap is marked", c.runtime, got, c.want)
		}
	}
	// An ordinary session on the same runtime: no exact model, nothing to
	// mark. The mark qualifies a recorded id, and inventing one where no id
	// was named would be a second claim.
	if got, want := b.App.RuntimeTierModelTag("grok", TierStrong, ""), "@grok/strong"; got != want {
		t.Errorf("an ordinary grok session's tag = %q, want %q", got, want)
	}

	// And the row an operator actually reads.
	canaryPersona(t, b, "architect")
	if err := b.CreateSession(NewSessionOpts{
		Name: "architect-canary", Agent: "architect",
		Runtime: "grok", Tier: TierStrong, Model: "grok-4.7",
		Dir: t.TempDir(), Crew: true,
	}); err != nil {
		t.Fatalf("canary launch: %v", err)
	}
	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list.String(), "🎭architect@grok/strong=grok-4.7"+ModelUncheckedMark) {
		t.Errorf("posse list does not mark the canary row:\n%s", list.String())
	}
}

// ─── the declaration itself ──────────────────────────────────────────────────

// A yaml that names something other than the two measured words REFUSES the
// load, on the template path and on a built-in's overlay alike — the
// record:/rules_precedence: rule, and for the sharper reason: a value posse
// accepted and could not read would decide which sentence an operator is
// told about a canary.
func TestUnknownModelYamlRefusesAWordItCannotRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, body string }{
		{"mycli", "command: mycli --sys {file}\nunknown_model: sometimes\n"},
		{"claude", "unknown_model: sometimes\n"},
	} {
		a := checkApp(t)
		writeRuntimeFile(t, a, c.name, c.body)
		_, err := a.LoadRuntime(c.name)
		if err == nil {
			t.Fatalf("%s: unknown_model: sometimes loaded clean", c.name)
		}
		for _, want := range []string{"unknown_model:", UnknownModelCarry, UnknownModelSwap} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not name %q: %v", c.name, want, err)
			}
		}
	}
	// The valid words load, on both paths, and the why rides with them.
	for _, c := range []struct{ name, body, want string }{
		{"mycli", "command: mycli --sys {file}\nunknown_model: " + UnknownModelCarry + "\nunknown_model_why: measured here\n", UnknownModelCarry},
		{"grok", "unknown_model: " + UnknownModelCarry + "\nunknown_model_why: 1.0.6 grew a refusal\n", UnknownModelCarry},
	} {
		a := checkApp(t)
		writeRuntimeFile(t, a, c.name, c.body)
		rt, err := a.LoadRuntime(c.name)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if rt.UnknownModel != c.want || rt.UnknownModelWhy == "" {
			t.Errorf("%s: loaded unknown_model %q / why %q", c.name, rt.UnknownModel, rt.UnknownModelWhy)
		}
		// The overlay is the path that matters most here: a new CLI release
		// is declared on the box before it is declared in a posse, and that
		// declaration has to reach the two sentences.
		if c.name == "grok" {
			if got := a.RuntimeUnknownModel("grok"); got != c.want {
				t.Errorf("the listing's reading ignores the overlay: %q", got)
			}
			if got := a.RuntimeTierModelTag("grok", TierStrong, "grok-4.7"); strings.Contains(got, ModelUncheckedMark) {
				t.Errorf("grok declared %s on this box and the row is still marked: %q", c.want, got)
			}
		}
	}
}

// The display reading never renders an unreadable value as a measurement.
// LoadRuntime refuses it loudly at launch, naming the file; a listing drawn
// from the same yaml must fall to the loud default instead of marking or
// un-marking a row on a typo.
func TestTheListingReadingIgnoresAnUnreadableDeclaration(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeRuntimeFile(t, a, "grok", "unknown_model: sometimes\n")
	if got := a.RuntimeUnknownModel("grok"); got != UnknownModelSwap {
		t.Errorf("a yaml typo moved the listing's reading to %q — it must stay on the built-in's measured value until the yaml says one of the two words", got)
	}
	if got := a.RuntimeUnknownModel("nosuchcli"); got != "" {
		t.Errorf("a runtime nobody has heard of reads %q, not the loud default", got)
	}
	if got := (*App)(nil).RuntimeUnknownModel("grok"); got != UnknownModelSwap {
		t.Errorf("with no instance to read a yaml from, the built-in's own declaration must still answer: %q", got)
	}
}
