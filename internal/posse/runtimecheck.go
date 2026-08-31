package posse

// `posse runtime check <name>` — the ADR 0013 §1 grid on one screen.
//
// This is how a runtime is ONBOARDED: fill the six stages, rather than
// discover each quirk in production one evening at a time. Every row says
// the same three things — what is observable, WHO declared it (a key in
// runtimes/<name>.yaml, a built-in default, herdr, an adapter), and what
// happens when it is missing, which is always a named degrade or a named
// refuse and never a patch.
//
// The unknown runtime is the one this command exists for. A template-only
// yaml carrying nothing but `command:` prints six honest rows: typed
// delivery on a 45s claude-shaped wait, untrusted record, no cost adapter,
// no tier mapping. Dispatchable and noisy — ADR 0013 rejected refusing it.

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// stageRow is one line of the grid plus its declared-by / missing-→ line.
type stageRow struct {
	stage   string
	value   string
	by      string
	missing string
	note    []string // extra indented lines under the row (interstitials, rulebooks)
}

// gridWidth is where the rows wrap. The ADR asks for one screen, and a row
// that wraps at the terminal's mercy is not one screen — it is a paragraph
// that moves when the window does.
const gridWidth = 78

// wrapGrid emits text under the stage column, hanging-indented, so every
// line of the grid starts in the same place whatever the terminal is.
func wrapGrid(w io.Writer, lead, text string) {
	const col = 14 // 2 + the 11-wide stage column + 1
	// Leading spaces in text are a sub-indent (an interstitial's detail
	// lines under its screen name), not content: they survive the wrap
	// instead of being eaten by Fields.
	sub := len(text) - len(strings.TrimLeft(text, " "))
	indent := strings.Repeat(" ", col+sub)
	fmt.Fprintf(w, "  %-11s %s", lead, strings.Repeat(" ", sub))
	n := col + sub
	for i, word := range strings.Fields(text) {
		if i > 0 && n+1+runeLen(word) > gridWidth+col {
			fmt.Fprint(w, "\n", indent)
			n = col + sub
		} else if i > 0 {
			fmt.Fprint(w, " ")
			n++
		}
		fmt.Fprint(w, word)
		n += runeLen(word)
	}
	fmt.Fprintln(w)
}

// runeLen: the wrap counts characters, not bytes. Every row here carries
// §, →, — and ✓, so a byte count wraps the grid a third of a line early.
func runeLen(s string) int { return len([]rune(s)) }

func (r stageRow) write(w io.Writer) {
	wrapGrid(w, r.stage, r.value)
	wrapGrid(w, "", "by "+r.by)
	wrapGrid(w, "", "missing → "+r.missing)
	for _, n := range r.note {
		wrapGrid(w, "", n)
	}
}

// declaredBy names the source of one contract key: the yaml that set it, or
// the default it fell back to. A built-in's defaults are declarations too —
// they were written against a measurement — so they say so.
func (rt *Runtime) declaredBy(key string) string {
	yaml := "runtimes/" + rt.Name + ".yaml"
	if rt.Path != "" && YamlGet(rt.Path, key) != "" {
		return yaml + " (" + key + ":)"
	}
	if rt.Builtin {
		return "built-in default"
	}
	return "nothing — " + key + ": unset in " + yaml + ", so this is the loud default"
}

// declaredByList is declaredBy for a key whose value may be a BLOCK list.
// `egress:` with its hosts on the following lines leaves nothing after the
// colon, so YamlGet reads it as unset and the provenance line would credit a
// built-in default the yaml had in fact overridden — the grid lying about
// exactly the fact it exists to carry.
func (rt *Runtime) declaredByList(key string) string {
	if rt.Path != "" && len(YamlList(rt.Path, key)) > 0 {
		return "runtimes/" + rt.Name + ".yaml (" + key + ":)"
	}
	return rt.declaredBy(key)
}

// RuntimeCheck prints the dispatch-contract grid for one runtime, then the
// preflight (ADR 0012 D4): the gaps that stop a launch working at all, each
// reported by name. It returns whether the preflight is CLEAN — no blocking
// gap — so the command can exit non-zero, which is what makes this usable
// as an onboarding gate rather than as a thing to read and nod at.
func (a *App) RuntimeCheck(rt *Runtime, h Herdr, w io.Writer) bool {
	kind := "template-only (no native realizer; every gate goes to the wall)"
	if rt.Builtin {
		kind = "built-in"
	}
	fmt.Fprintf(w, "%s — dispatch contract (ADR 0013 §1) · %s\n", rt.Name, kind)
	fmt.Fprintf(w, "  %s\n\n", rt.Command)

	for _, r := range []stageRow{
		a.launchRow(rt, h),
		a.promptableRow(rt),
		workRow(),
		recordRow(rt),
		settleRow(rt),
		a.accountRow(rt),
	} {
		r.write(w)
	}

	// Tier is not one of the six stages — it is ADR 0013 §6 — but an
	// onboarder reads it in the same breath, because a PID that says
	// `tier: strong` on an unmapped runtime is wearing a name nothing
	// behind it honours.
	fmt.Fprintln(w)
	wrapGrid(w, "tier", a.tierLine(rt))
	wrapGrid(w, "", "by "+rt.tierBy())
	if len(rt.NativeRules) > 0 {
		wrapGrid(w, "rulebooks", strings.Join(rt.NativeRules, ", "))
		wrapGrid(w, "", "posse loads none of these and rewrites none of them — they are the operator's files in a shared checkout.")
		if ValidRulesPrecedence(rt.RulesPrecedence) {
			val := "precedence: " + rt.RulesPrecedence
			if rt.RulesPrecedenceWhy != "" {
				val += " — " + rt.RulesPrecedenceWhy
			}
			wrapGrid(w, "", val)
		} else {
			wrapGrid(w, "", "precedence UNMEASURED — the PID-wins prompt line is the only reconciliation (ADR 0013 §4)")
		}
	} else {
		wrapGrid(w, "rulebooks", "none declared — native_rules: in the yaml names the files this CLI loads by itself, ahead of anything posse types")
	}
	// The dimensions the Runtime struct declares that the six ADR 0013
	// stages do not (ADR 0017 §1: the checklist IS the struct, and a
	// dimension a field expresses and no row prints is git archaeology
	// waiting to happen). Same row shape and the same declaredBy
	// provenance, because an onboarder reads them in the same pass — and
	// ADR 0017 §2's vocabulary throughout: a measured-to-differ dimension
	// reads as a DECLARED DIFFERENCE, an unmeasured one as UNDECLARED, and
	// the two are never spelled the same way.
	fmt.Fprintln(w)
	for _, r := range []stageRow{
		a.skillsRow(rt),
		egressRow(rt),
		cageCredRow(rt),
		projectConfigRow(rt),
		sandboxRow(rt),
	} {
		r.write(w)
	}

	// The onboarding footer is about a runtime you DECLARE. Printed under a
	// built-in it names the ADR 0021 overlay instead: runtimes/<name>.yaml
	// IS read there, but only for the keys that name a measured instance
	// fact — command:/skills_flag: refuse, because those change the launch
	// mechanism a built-in's realizer and verified skill surface already
	// wear, not a number this box measured.
	if rt.Builtin {
		fmt.Fprintf(w, "\n  %s is a BUILT-IN: runtimes/%s.yaml is a per-key OVERLAY onto it (ADR 0021) — the yaml\n", rt.Name, rt.Name)
		fmt.Fprintln(w, "  wins for a MEASURED fact (model_<tier>:, model_flag:, prompt:, startup_wait:, record: (+")
		fmt.Fprintln(w, "  record_why:), native_rules:, egress:, cage_cred:, gate_shell:), the built-in supplies the")
		fmt.Fprintln(w, "  rest; command: and skills_flag: REFUSE there — the launch mechanism, not a measured fact.")
		fmt.Fprintln(w, "  Onboarding your OWN CLI, by contrast, is filling this WHOLE grid: it takes command:, prompt:,")
	} else {
		fmt.Fprintf(w, "\n  onboarding a runtime is filling this grid: runtimes/%s.yaml takes command:, prompt:,\n", rt.Name)
	}
	fmt.Fprintln(w, "  startup_wait:, record: (+ record_why:), turn_outcome:, native_rules:,")
	fmt.Fprintln(w, "  rules_precedence: (+ rules_precedence_why:),")
	fmt.Fprintln(w, "  model_flag:/model_<tier>:, skills_flag: OR skills_cwd:, self_sandbox:, unattended:,")
	fmt.Fprintln(w, "  project_config: (+ project_config_keys:), egress:, cage_cred:, gate_shell:,")
	fmt.Fprintln(w, "  state_dir:, env_required:, interstitial_<name>:. Undeclared is loud, never")
	fmt.Fprintln(w, "  silent — and a key none of these names is warned on load, because a dropped")
	fmt.Fprintln(w, "  declaration never arrives (ADR 0012 D4).")

	return a.writePreflight(rt, h, w)
}

// writePreflight is the second half of the screen: what a launch on this
// runtime would find on THIS machine, as opposed to what the profile
// declares. The two are separate on purpose — a profile can be perfectly
// authored and still be unlaunchable here because the CLI is not installed,
// and an operator debugging one of those needs to know which half is wrong.
func (a *App) writePreflight(rt *Runtime, h Herdr, w io.Writer) bool {
	fmt.Fprintln(w)
	// state_dir and env_required are declarations, so they print whether or
	// not they are a gap — an onboarder reading this grid has to be able to
	// see that the key is UNSET, which is the state that costs them the
	// evening.
	if len(rt.StateDirs) > 0 {
		wrapGrid(w, "state_dir", strings.Join(rt.StateDirs, " ")+" — joins the seatbelt writable set, so `cage: seatbelt` leaves this CLI's own config writable")
	} else {
		wrapGrid(w, "state_dir", "none declared. A CLI that keeps state under the home gets a READ-ONLY one under `cage: seatbelt` — it then re-runs its first-run flow every launch, or dies on a config write, and neither says why")
	}
	if len(rt.EnvRequired) > 0 {
		wrapGrid(w, "env_req", strings.Join(rt.EnvRequired, " ")+" — checked by NAME at launch preflight; a missing one refuses the launch. posse never reads what they hold")
	} else {
		wrapGrid(w, "env_req", "none declared — this runtime is taken to authenticate from its own state dir. The Bedrock shape (AWS_* in the session env) is declared here, not remembered")
	}

	// probe — a DECLARATION line like the two above, printed whether or not
	// it is a gap. The state an onboarder has to be able to see is the one
	// that costs them the evening: their Bash(...) denies are counted by
	// nothing until a live probe says otherwise, and the record is what says
	// which binary was measured (ADR 0032 §1).
	if rt.Builtin {
		wrapGrid(w, "probe", "not applicable — "+rt.Name+" is a built-in and its shell argv table was probed in ADR 0009 (rangerhq-e43). `posse runtime probe` measures a runtime you DECLARE")
	} else {
		st := a.ProbeState(rt)
		lead := "ASSUMED — Bash(...) denies here are not measured: they land in the launch's Degraded list. "
		if st.Current {
			lead = "MEASURED — Bash(...) denies here are realized by L1 + the gate shell, on evidence. "
		}
		wrapGrid(w, "probe", lead+st.Why)
		if st.Record != nil {
			wrapGrid(w, "", "record: "+AbbrevHome(a.ProbeRecordPath(rt.Name))+" — 4 observables (shim precedence, refusal through direct/sh -c/script, unattended turn, herdr detection)")
		}
	}

	gaps := a.RuntimeGaps(rt, h)
	if len(gaps) == 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  preflight ✓ clean — %s is installed, herdr can name it, and every key in the profile arrives\n", rt.Name)
		return true
	}
	clean := true
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  preflight — %d gap(s):\n", len(gaps))
	for _, g := range gaps {
		mark := "⚠️ "
		if g.Blocking {
			mark = "✗"
			clean = false
		}
		wrapGrid(w, "", fmt.Sprintf("%s %s: %s", mark, g.Name, g.Line))
	}
	if clean {
		fmt.Fprintln(w, "  nothing blocking — every gap above is a named degrade, not a refusal")
	}
	return clean
}

func (a *App) launchRow(rt *Runtime, h Herdr) stageRow {
	exe := rt.Exe()
	// Asked the way herdr resolves it — a manifest reached through another
	// agent's `aliases = [...]` counts, and on herdr 0.8.0 that is the only
	// route a CLI it was not built with has (Herdr.AgentManifest).
	seen := "herdr recognition UNKNOWN (herdr not on PATH, or its output moved)"
	if ver, known, ok := h.AgentManifest(exe); ok {
		seen = fmt.Sprintf("herdr does NOT recognize argv0 %q — no detection here, so work/settle are guesses", exe)
		if known {
			seen = fmt.Sprintf("herdr recognizes argv0 %q (detection manifest %s)", exe, ver)
		}
	} else if kinds := h.KnownAgentKinds(); kinds != nil {
		seen = fmt.Sprintf("herdr does NOT recognize argv0 %q — no detection here, so work/settle are guesses", exe)
		for _, k := range kinds {
			if k == exe {
				seen = fmt.Sprintf("herdr recognizes argv0 %q", exe)
				break
			}
		}
	}
	un := "unattended flag " + rt.Unattended + " on the line"
	if rt.Unattended == "" {
		un = "NO unattended flag known — a tool call may sit unapproved with nobody watching"
	}
	return stageRow{
		stage:   "launch",
		value:   seen + "; PID delivered by the template; " + un,
		by:      "runtime template + herdr manifest (ADR 0002 / 0012 D4)",
		missing: "refuse the launch",
	}
}

func (a *App) promptableRow(rt *Runtime) stageRow {
	var val string
	switch rt.PromptMode() {
	case PromptArgv:
		val = `argv — the work prompt is appended to the launch line as "$(cat <file>)"; no screen is the delivery channel`
	default:
		val = "typed — create → await promptable → claim → type, with " + rt.Wait().String() + " of patience"
		if rt.StartupWait == 0 {
			val += " (startup_wait: unset → the default)"
		}
	}
	r := stageRow{
		stage:   "promptable",
		value:   val,
		by:      rt.declaredBy("prompt"),
		missing: "refuse THIS launch, loudly — the session, not the persona (ADR 0013 §2 busy-key split)",
	}
	// Layer 2 of §2: what the operator has to silence before a fresh pane
	// of this runtime is promptable at all, and whether they have.
	for _, in := range rt.Interstitials {
		state := "state unknown — posse cannot read this CLI's config format"
		if in.Probe != nil {
			sil := in.Probe()
			switch {
			case sil.Unknown:
				state = "state unknown: " + sil.Why
			case sil.Silenced:
				state = "silenced: " + sil.Why
			default:
				state = "NOT SILENCED: " + sil.Why
			}
		}
		r.note = append(r.note, "interstitial: "+in.Screen)
		r.note = append(r.note, "  key: "+in.Key+" in "+in.Where+" — "+state)
		if in.Seeded {
			r.note = append(r.note, "  posse SEEDS it at launch: "+in.Silence)
		} else {
			r.note = append(r.note, "  operator silences it: "+in.Silence)
		}
		if in.Danger != "" {
			r.note = append(r.note, "  DEFAULT ACTION MUTATES THE MACHINE — "+in.Danger)
			// ADR 0013 §2's rule, and then what the code actually does. The
			// two were not the same until ranger-base-9r33 wired the refusal;
			// a grid that printed only the rule was making a promise nothing
			// kept, and one that printed only the rule NOW would hide the
			// asymmetry an operator meets the moment they type `posse new`.
			r.note = append(r.note, "  LAUNCH REFUSE until the operator's own config silences it (ADR 0013 §2): anything dispatched — a pass, the cockpit's `d`, a recipe — refuses before it claims. An INTERACTIVE launch warns DEGRADED and proceeds, because answering this screen is what you would open a session to do.")
			r.note = append(r.note, "  posse never answers this: nothing blind-sends Enter, and the launcher's one keystroke table was retired in rangerhq-6723 (ADR 0013 §2).")
		}
	}
	if len(rt.Interstitials) == 0 {
		r.note = append(r.note, "interstitials: none declared. Declare one with interstitial_<name>: { screen:, where:, key:, silence:, danger: } — posse NAMES that key and never writes it, and never presses anything at the screen (ADR 0013 §2).")
	}
	return r
}

func workRow() stageRow {
	return stageRow{
		stage:   "work",
		value:   "herdr `working`, then a settled state",
		by:      "herdr detection (nothing to declare)",
		missing: "the wait ladder as today (NOTES §6–7); a timeout is a check-in, NEVER an unclaim",
	}
}

func recordRow(rt *Runtime) stageRow {
	val := "untrusted — no dispatched session of this runtime has been measured to close its bead"
	if rt.RecordTrust() == RecordTrusted {
		val = "trusted"
		if rt.RecordWhy != "" {
			val += " — " + rt.RecordWhy
		}
	}
	r := stageRow{
		stage:   "record",
		value:   val,
		by:      rt.declaredBy("record"),
		missing: "settle-without-record is INCOMPLETE, never ✓; unattended --resume re-prompts (ADR 0013 §4)",
	}
	r.note = append(r.note, "the store of record is the bead (ADR 0011); agent settle is a hint. `bd close` stays the persona's — the harness never closes on its behalf.")
	r.note = append(r.note, "reap guard: a session of ANY runtime whose bead is still in_progress over an uncommitted cwd is not killed — `posse kill`/`posse relaunch` refuse it and name why (--force overrides).")
	return r
}

func settleRow(rt *Runtime) stageRow {
	r := stageRow{
		stage:   "settle",
		value:   "herdr idle/done/blocked from a MATCHED rule — Seen(), not the idle-fallback",
		by:      "herdr detection (nothing to declare)",
		missing: "the existing ignorance path: the claim is kept",
	}
	// The settle stage has a declared half, and it is the one that separates
	// two settles that look identical to herdr: a turn that ran, and a turn
	// an exhausted account refused (ranger-base-02zr). herdr sees the pane go
	// idle either way; only the runtime's own record says which happened.
	if rt.ReadsTurnOutcome() {
		r.note = append(r.note, "turn outcome: READ by the "+rt.TurnOutcomeAdapter+" adapter — an account that refused the turn stops the bead with ⛔ instead of settling as ◑, and the session is tagged in `posse list`.")
	} else {
		r.note = append(r.note, "turn outcome: NOT READ — no turn_outcome: adapter declared, so an exhausted account and an agent that worked without closing its bead are the SAME ◑ line. The settle line says so per bead; `turn_outcome: "+strings.Join(TurnOutcomeAdapters(), "|")+"` is what changes it (ADR 0012 D4's reader seam).")
	}
	r.note = append(r.note, "declared by: "+rt.declaredBy("turn_outcome"))
	return r
}

func (a *App) accountRow(rt *Runtime) stageRow {
	if rt.CostPriced() {
		return stageRow{
			stage:   "account",
			value:   "counted — " + rt.CostReading(),
			by:      "cost adapter (ADR 0012 D4)",
			missing: "account-degraded, named loudly every pass",
		}
	}
	// Parsed, not echoed: a value that is not a positive bead count is not a
	// cap, and printing it back as one is the grid saying a brake is armed
	// when nothing is (uncounted.go keeps the same rule for the pass).
	n, raw := a.UncountedCap(rt.Name, io.Discard)
	capline := "uncounted_cap_" + rt.Name + ": unset — unlimited and loud (the budget_* dormancy pattern)"
	switch {
	case n > 0:
		capline = fmt.Sprintf("uncounted_cap_%s: %d beads / rolling 7 days, ledgered in %s", rt.Name, n, AbbrevHome(a.UncountedLogPath()))
	case raw != "":
		capline = fmt.Sprintf("uncounted_cap_%s: %q is not a positive bead count — no cap: unlimited and loud", rt.Name, raw)
	}
	// Two degrades, not one, and the grid has to say which (ranger-base-0lg6):
	// nothing reads this runtime, or something reads it and prices none of
	// what it reads. Both are account-degraded — the cap is the brake for
	// both, because what the cap stands in for is a missing DOLLAR meter,
	// which is equally missing either way — but "no cost adapter reads this
	// runtime" is a false sentence about codex, whose rollouts posse counts
	// turn by turn.
	r := stageRow{
		stage:   "account",
		value:   "UNCOUNTED — no cost adapter reads this runtime. Uncounted is a degrade, never $0 (ADR 0003 §4)",
		by:      "nothing — the ADR 0012 D4 adapter seam is unfilled here",
		missing: "account-degraded: dispatchable, named loudly every pass; the cap is the brake (ADR 0013 §5)",
	}
	if reading := rt.CostReading(); reading != "" {
		r.value = "UNPRICED — " + reading + " reads this runtime's turns, tokens and beads and prices none of them. Unpriced is a degrade, never $0 (ADR 0003 §4)"
		r.by = "cost adapter (ADR 0012 D4) — reading, not pricing"
		r.note = append(r.note, "`posse cost` counts these sessions (they are NOT in its uncounted line) and prints a BLANK in the $ column; the cockpit shows `$unpriced` rather than `$uncounted`.")
	}
	r.note = append(r.note, capline)
	r.note = append(r.note, "the cap counts beads posse itself launched, not a bill — no autonomous spending.")
	return r
}

// tierLine is the ADR 0013 §6 row: which tiers this runtime actually
// renders a model for, and — the part that costs an evening when it is
// missing — which ones it does NOT. A runtime that ignores a tier has to
// SAY so here, because the only other way to learn it is to read
// runtime.go, which is how `tier: strong` sat inert on codex for as long
// as it did (ranger-base-arm).
func (a *App) tierLine(rt *Runtime) string {
	mapped, unmapped := rt.TierMap()
	inert := func(tiers []string) string {
		return "`tier: " + tiers[0] + "` here is intent, not a guarantee: {model} renders empty, the CLI picks its own, and the display is " +
			rt.Name + "/default, never " + rt.Name + "/" + tiers[0] + " (ADR 0013 §6). " + rt.tierFix()
	}
	switch {
	case len(mapped) == 0:
		return "UNMAPPED — this runtime ignores tier: entirely. " + inert(Tiers)
	case len(unmapped) == 0:
		return strings.Join(mapped, " ") + " (rendered with " + rt.ModelFlag + ")"
	}
	return strings.Join(mapped, " ") + " (rendered with " + rt.ModelFlag + "); UNMAPPED: " +
		strings.Join(unmapped, ", ") + " — " + inert(unmapped)
}

// tierFix says where the missing mapping would have to be declared. Before
// ADR 0021 that had to distinguish a built-in from a declared runtime — a
// built-in's own runtimes/<name>.yaml was read by nothing, so sending an
// operator to write model_<tier>: there was a remedy the value could never
// reach (ranger-base-arm). ADR 0021 made that file a per-key OVERLAY onto
// the built-in, and model_<tier>: is one of the overlay keys (Decision 1),
// so the remedy is the same file for a built-in and a declared runtime
// alike — only command:/skills_flag: still refuse there (Decision 2).
func (rt *Runtime) tierFix() string {
	return "Declare model_<tier>: (and model_flag:) in runtimes/" + rt.Name + ".yaml to change that"
}

// tierBy is the tier row's declared-by line, one attribution per MAPPED
// tier rather than one for the whole row — since ADR 0021 a built-in's
// overlay can set model_fast: alone and leave strong/standard on the
// built-in map, and a row that named a single source for all three would
// credit the yaml for two tiers it never touched. fast falls back to
// standard (Runtime.Model) when model_fast: is itself unset, so that
// tier's attribution follows the value it actually rendered rather than
// reporting the untouched key as unset.
func (rt *Runtime) tierBy() string {
	mapped, _ := rt.TierMap()
	if len(mapped) == 0 {
		return rt.declaredBy("model_<tier>")
	}
	var parts []string
	for _, t := range Tiers {
		switch {
		case rt.Models[t] != "":
			parts = append(parts, t+": "+rt.declaredBy("model_"+t))
		case t == TierFast && rt.Models[TierStandard] != "":
			parts = append(parts, t+": falls back to standard — "+rt.declaredBy("model_"+TierStandard))
		}
	}
	return strings.Join(parts, "; ")
}

// ─── the ADR 0002 / 0007 dimensions ──────────────────────────────────────
//
// Five rows the six-stage grid never carried, each a Runtime field a launch
// changes behaviour on. They are here rather than in a doc because ADR 0017
// §1's criterion is operational: onboarding a fourth runtime is filling this
// grid, and a dimension that is only in git history is one the onboarder
// meets in production instead.

// skillsRow is the three-way split parity.go already computes (flag surface
// / cwd-discovery / no surface at all), printed as the thing the CLI reads
// rather than as the name of a Go field. Until this row existed, the only
// trace of a skill surface on the screen was the {skills} placeholder inside
// the echoed command template — so a runtime with NO surface, whose every
// `skills:` PID refuses to launch, looked exactly like one with a flag
// (ranger-base-qm6e).
func (a *App) skillsRow(rt *Runtime) stageRow {
	// The rendered tree per persona, spelled as the path it actually takes,
	// so the flag arm shows what {skills} points at and not a placeholder.
	tree := AbbrevHome(filepath.Join(a.StateDir, "skills", "<persona>", "claude"))
	flag, ok := rt.SkillsText(tree, []string{"<skill>"})
	r := stageRow{
		stage:   "skills",
		by:      rt.declaredBy("skills_flag"),
		missing: "a PID with skills: cannot launch here at all — ADR 0007 §3 checks a bound skill like a gate, so the parity check REFUSES rather than degrading. A PID without skills: is unaffected",
	}
	switch {
	case !ok:
		r.value = "NO SURFACE — UNDECLARED: neither skills_flag: nor skills_cwd:, so posse has nothing to point this CLI at for one session"
		r.by = "nothing — neither skills_flag: nor skills_cwd: is set for " + rt.Name + ", so this is the loud default"
		r.note = append(r.note, "declare the one you MEASURED: skills_flag: (the printf form {skills} renders, e.g. \"--plugin-dir %s\") or skills_cwd: true (the CLI walks "+AgentsSkillsPath+" under the session dir itself). Declaring BOTH refuses at load — a runtime has one skill surface, and two half-bindings is a grid that cannot say which one the CLI read.")
	case rt.SkillsCwd:
		r.value = "cwd-discovery — " + AgentsSkillsPath + " under the session dir, symlinked at launch and ADDITIVE; {skills} renders nothing and the LINKS are the binding"
		r.by = rt.declaredBy("skills_cwd")
		r.note = append(r.note, "the dir belongs to the REPO, not to the persona: posse adds its own links, refuses to overwrite an entry it did not write, and sweeps a link whose target has left RHQ_HOME/skills. It is hidden from `git status` through .git/info/exclude, which hides it from git and not from the CLI (measured rangerhq-1qd).")
	default:
		r.value = "flag — " + flag + ", pointed at the tree posse renders per persona (session-only, additive)"
		r.note = append(r.note, "that tree binds each skill as a SYMLINK into RHQ_HOME/skills, and whether this CLI FOLLOWS one is the thing to measure next: grok validates and installs the very same tree and surfaces ZERO skills, where a `cp -RL` copy of it surfaces every one (ranger-base-65rc). A row that said `flag` and stopped is how that reaches a fourth runtime.")
	}
	return r
}

// egressRow: the hosts this runtime itself must reach for a caged session on
// it to be a session at all. ADR 0002 §4 — the launcher always adds them to
// the PID's own allowlist, because a cage that cannot reach its model is not
// an isolated persona, it is an offline one.
func egressRow(rt *Runtime) stageRow {
	r := stageRow{
		stage:   "egress",
		by:      rt.declaredByList("egress"),
		missing: "a `cage: container` session here reaches ONLY what its PID names, its own API not among them — and the failure shape is per-CLI: claude degrades quietly, codex retries ~70× in 35s and then errors hard (measured, rangerhq-89a)",
	}
	if len(rt.Egress) > 0 {
		r.value = strings.Join(rt.Egress, " ") + " — this runtime's OWN hosts, always added to a caged PID's egress: allowlist (ADR 0002 §4)"
		r.note = append(r.note, "telemetry hosts are deliberately NOT declared here: a caged persona's traffic is the operator's business, and every CLI measured degrades quietly without theirs.")
		return r
	}
	r.value = "UNDECLARED — no host of this runtime's own is known, so a caged session on it reaches only what its PID names"
	r.note = append(r.note, "posse has no business guessing an API host, so absent is the honest default rather than a table: measure this CLI's API host and the host its OAuth refresh goes to, and name them in egress:.")
	return r
}

// cageCredRow: the env var an authenticated CAGED session needs. A container
// has no keychain and posse never reads an on-disk credential file there
// either, so an undecided one refuses `cage: container` with the reason
// instead of spending the launch on a session that cannot reach its API.
func cageCredRow(rt *Runtime) stageRow {
	r := stageRow{
		stage:   "cage_cred",
		by:      rt.declaredBy("cage_cred"),
		missing: "`cage: container` REFUSES on this runtime, naming the reason (cage.go CheckCageCredential). Every other cage tier is unaffected — this is the container's credential, not the runtime's",
	}
	if name := CageCredential(rt); name != "" {
		r.value = name + " — the env NAME a containerised session authenticates with, checked by name at launch; posse never reads what it holds"
		if rt.CageCred == "" {
			r.by = "built-in table (cage.go cageCredential) — the operator's decision of 2026-08-20, rangerhq-kiz"
		}
	} else {
		r.value = "UNDECIDED — cage: container refuses on this runtime: a container has no keychain, and any on-disk credential file there is not the store of record and posse never reads it (ADR 0002 §4, rangerhq-kiz)"
		r.note = append(r.note, "cage_cred: in the yaml names it once the operator has minted one; every cage tier below container needs nothing here.")
	}
	r.note = append(r.note, "a METERED api key is not accepted as this credential — that is spending, and a persona is never the one who decides to spend. Mint it by hand, keep it in an env set (mode 600, never in the repo), and name that set in the PID's envs:.")
	return r
}

// projectConfigRow: the repo→box configuration channel. A trusted session
// directory means this CLI may read project-owned executable configuration
// before any model turn — no PID and no tool gate sits in front of it — so
// the launch checks for it and degrades unless the PID opts in.
func projectConfigRow(rt *Runtime) stageRow {
	r := stageRow{
		stage:   "project_cfg",
		by:      rt.declaredBy("project_config"),
		missing: "the silent one: ProjectConfigTrust skips a channel nobody declared, so an unguarded repo→box channel reads as a CLEAN launch (parity.go)",
	}
	if len(rt.ProjectConfig) == 0 {
		r.value = "none — no repo→box config surface declared for this runtime, so nothing in the session dir is taken to be executable configuration it reads"
		r.note = append(r.note, "if it DOES read one, project_config: <path relative to the session dir> is what makes the channel visible; the launch then degrades unless the PID sets trust_project_config: true.")
		return r
	}
	r.value = strings.Join(rt.ProjectConfig, " ") + " — read from the SESSION DIR at launch because posse made that directory trusted: project-owned executable configuration, running on the box with the whole session env, before any model turn or PID/tool gate can mediate it"
	if len(rt.ProjectConfigKeys) > 0 {
		r.note = append(r.note, "narrowed to top-level JSON keys: "+strings.Join(rt.ProjectConfigKeys, ", ")+" — only a file naming one of them degrades the launch. The floor holds either way: a keyed file that cannot be proved a readable top-level JSON object FAILS CLOSED, so keys declared over a TOML config degrade every launch instead of narrowing nothing quietly.")
		r.note = append(r.note, "declared by: "+rt.declaredByList("project_config_keys"))
	} else {
		r.note = append(r.note, "the WHOLE FILE is the predicate: its mere presence degrades the launch. That is the conservative side, and where project_config_keys: is unset it is where this stays.")
	}
	r.note = append(r.note, "the PID opts in with trust_project_config: true; otherwise the launch is degraded and names the file.")
	return r
}

// sandboxRow carries the two dimensions that decide which WALLS a launch on
// this runtime actually gets: whether posse's own seatbelt can wrap it, and
// whether the L1 gate shell survives on it. Both are DECLARED DIFFERENCES
// when set — ADR 0017 §2 — and nothing here may render one as a failure.
func sandboxRow(rt *Runtime) stageRow {
	r := stageRow{
		stage:   "sandbox",
		by:      rt.declaredBy("self_sandbox"),
		missing: "an undeclared self-sandboxing CLI is seatbelt-wrapped anyway and the launch dies with `sandbox_apply: Operation not permitted`, while the parity matrix claims an L2 it does not have",
	}
	if rt.SelfSandbox {
		r.value = "self_sandbox — a DECLARED DIFFERENCE, not a failure: this CLI wraps its own child commands and macOS refuses to nest seatbelts, so `cage: seatbelt` does NOT wrap it and its own sandbox is what enforces Edit/Write here (verified 2026-08-17)"
	} else {
		r.value = "posse's seatbelt wraps this runtime — self_sandbox: unset, so `cage: seatbelt` renders sandbox-exec in front of the line and L2 is this runtime's write wall"
	}
	if rt.NoGateShell {
		r.note = append(r.note, "gate_shell: false — a DECLARED DIFFERENCE (ADR 0009 §2): SHELL/GROK_SHELL are left alone because a wrapper named as the shell chokes this CLI. It is honest and it costs the L1 wall — every Bash(...) deny is UNREALIZED here but `git push`, which L3 catches as a git hook once CheckParityIn has behavior-probed it.")
	} else {
		r.note = append(r.note, "gate_shell: on — the launch points SHELL/GROK_SHELL at the gate shell, which is what keeps L1 alive on a CLI that re-execs a LOGIN shell per command: on macOS that hands PATH to path_helper, /usr/bin lands ahead of the gates dir and the shim never runs (measured on grok 1.0.5, rangerhq-vjl).")
	}
	r.note = append(r.note, "declared by: "+rt.declaredBy("gate_shell"))
	return r
}
