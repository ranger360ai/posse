package rhq

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

// RuntimeCheck prints the dispatch-contract grid for one runtime.
func (a *App) RuntimeCheck(rt *Runtime, h Herdr, w io.Writer) {
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
		settleRow(),
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
	if len(rt.NativeRules) > 0 {
		wrapGrid(w, "rulebooks", strings.Join(rt.NativeRules, ", "))
		wrapGrid(w, "", "posse loads none of these and rewrites none of them — they are the operator's files in a shared checkout. Whether one outranks the PID is a probe, not a patch (ADR 0013 §4).")
	} else {
		wrapGrid(w, "rulebooks", "none declared — native_rules: in the yaml names the files this CLI loads by itself, ahead of anything posse types")
	}
	fmt.Fprintf(w, "\n  onboarding a runtime is filling this grid: runtimes/%s.yaml takes command:, prompt:,\n", rt.Name)
	fmt.Fprintln(w, "  startup_wait:, record: (+ record_why:), native_rules:, model_flag:/model_<tier>:,")
	fmt.Fprintln(w, "  skills_flag:, egress:, cage_cred:, gate_shell:. Undeclared is loud, never silent.")
}

func (a *App) launchRow(rt *Runtime, h Herdr) stageRow {
	exe := rt.Exe()
	seen := "herdr recognition UNKNOWN (herdr not on PATH, or its kind list moved)"
	if kinds := h.KnownAgentKinds(); kinds != nil {
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
		state := "state unknown"
		if in.Probe != nil {
			ok, why := in.Probe()
			mark := "NOT SILENCED"
			if ok {
				mark = "silenced"
			}
			state = mark + ": " + why
		}
		r.note = append(r.note, "interstitial: "+in.Screen)
		r.note = append(r.note, "  key: "+in.Key+" in "+in.Where+" — "+state)
		r.note = append(r.note, "  operator silences it: "+in.Silence)
		if in.Danger != "" {
			r.note = append(r.note, "  LAUNCH REFUSE until silenced — "+in.Danger)
			r.note = append(r.note, "  posse never answers this: nothing blind-sends Enter (ADR 0013 §2).")
		}
	}
	if len(rt.Interstitials) == 0 {
		r.note = append(r.note, "interstitials: none declared. A first-run dialog whose default action mutates the machine is a launch refuse until the operator's own config silences it — posse names that key and never writes it (ADR 0013 §2).")
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
	return r
}

func settleRow() stageRow {
	return stageRow{
		stage:   "settle",
		value:   "herdr idle/done/blocked from a MATCHED rule — Seen(), not the idle-fallback",
		by:      "herdr detection (nothing to declare)",
		missing: "the existing ignorance path: the claim is kept",
	}
}

func (a *App) accountRow(rt *Runtime) stageRow {
	if rt.Counted() {
		return stageRow{
			stage:   "account",
			value:   "counted — " + rt.CostAdapter,
			by:      "cost adapter (ADR 0012 D4)",
			missing: "account-degraded, named loudly every pass",
		}
	}
	cap := a.CfgGet("uncounted_cap_"+rt.Name, "")
	capline := "uncounted_cap_" + rt.Name + ": unset — unlimited and loud (the budget_* dormancy pattern)"
	if cap != "" {
		capline = "uncounted_cap_" + rt.Name + ": " + cap + " beads / rolling 7 days"
	}
	r := stageRow{
		stage:   "account",
		value:   "UNCOUNTED — no cost adapter reads this runtime. Uncounted is a degrade, never $0 (ADR 0003 §4)",
		by:      "nothing — the ADR 0012 D4 adapter seam is unfilled here",
		missing: "account-degraded: dispatchable, named loudly every pass; the cap is the brake (ADR 0013 §5)",
	}
	r.note = append(r.note, capline)
	r.note = append(r.note, "the cap counts beads posse itself launched, not a bill — no autonomous spending.")
	return r
}

func (a *App) tierLine(rt *Runtime) string {
	var mapped []string
	for _, t := range Tiers {
		if id := rt.Model(t); id != "" {
			mapped = append(mapped, t+"="+id)
		}
	}
	if len(mapped) == 0 {
		return "UNMAPPED — {model} renders empty, so the runtime picks. A PID's `tier: strong` here is intent, not a guarantee; display is " +
			rt.Name + "/default, never " + rt.Name + "/strong (ADR 0013 §6)"
	}
	return strings.Join(mapped, " ") + " (rendered with " + rt.ModelFlag + ")"
}
