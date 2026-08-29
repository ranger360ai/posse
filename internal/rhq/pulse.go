package rhq

// The pulse: a shop-check ticker inside the dispatch --watch process (ADR
// 0027, rangerhq-wxd). Sensing (§1-2, rangerhq-4ish) computes the condition
// set and fingerprints it to state/pulse.yaml. Delivery (§3-4, rangerhq-44w1)
// decides, on a non-empty set, whether it is due — new fingerprint, or an
// unchanged one past its renag backoff — and if so prompts pulse_persona's
// live session idle-only, with no authority and no crew mark.
//
// It starts with the watch loop and dies with it (Watch's ctx), never a
// second loop of its own — a hand-typed pass never pulses, the same premise
// as Unattended (watch.go): only a timer with no human witness needs a
// shop check running behind it.
//
// Sensing is no longer this file's: the condition set moved to govern.go
// when the governance surface (ADR 0029) widened it to the G-table (bead
// rangerhq-81y0 — see that file's header). The pulse is now one of three renderings of
// ShopCheck, and it keeps exactly two things of its own: the fingerprint
// that dedups DELIVERY, and delivery itself. state/pulse.yaml stays dedup
// state and is never a record anyone reads for truth.
//
// That widening broke ADR 0027 §1's "never bd, never the plan endpoint"
// boundary, deliberately and with the design's eyes open: G2/G3/G9 are bd
// facts and G5/G6 are meter facts, and a surface that cannot see them is
// blind to most of what actually stops a shop. The cost is bounded (a few
// list calls per tick) and measured on the bead. What the boundary was
// protecting — silence off a timer nobody is watching — survives as a rule
// rather than an abstinence: a store that cannot be read is logged and the
// tick moves on, never a false alarm and never a fatal.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pulsePersona is who the pulse watches for absence (condition c) and
// delivers to. Config `pulse_persona:` when set; otherwise the instance's
// `coordinator:` (ADR 0018 §1) — the same persona ADR 0027 §3 named, spelled
// as the config key that already holds it rather than as a name compiled in.
// The engine ships carrying no crew name (App.Coordinator's rangerhq-gk4k
// rule, ranger-base-q3gp): a fresh deployer whose config names neither gets
// "", which is a pulse with no target at all — sensed, drawn by `posse
// status`, delivered to nobody — and not a permanent no-live: condition for
// a persona that never existed here.
func pulsePersona(a *App) string { return a.CfgGet("pulse_persona", a.Coordinator()) }

// DefaultPulseRenag and DefaultPulseRenagMax are the renag backoff bounds
// (ADR 0027 §3) when pulse_renag:/pulse_renag_max: are unset: re-prompt an
// unchanged condition set after 30m, doubling per repeat, capped at 4h.
const (
	DefaultPulseRenag    = 30 * time.Minute
	DefaultPulseRenagMax = 4 * time.Hour
)

// PulseConfig is the pulse_* config family, autostart_* style (flat YAML,
// plugin/autostart.sh): presence of pulse_interval: is the arm switch.
// Renag/RenagMax are delivery's renag backoff bounds (ADR 0027 §3-4).
type PulseConfig struct {
	Armed    bool
	Interval time.Duration
	Persona  string
	Renag    time.Duration
	RenagMax time.Duration
}

// LoadPulseConfig reads config.yaml's pulse_* keys. Disarmed (pulse_interval
// absent) returns a zero PulseConfig and no error — an unset family is not
// a misconfiguration, it is the default.
func LoadPulseConfig(a *App) (PulseConfig, error) {
	if !yamlHasKey(a.ConfigPath, "pulse_interval") {
		return PulseConfig{}, nil
	}
	interval, err := ParseInterval(a.CfgGet("pulse_interval", ""))
	if err != nil {
		return PulseConfig{}, Die("config pulse_interval: %v", err)
	}
	cfg := PulseConfig{
		Armed:    true,
		Interval: interval,
		Persona:  pulsePersona(a),
		Renag:    DefaultPulseRenag,
		RenagMax: DefaultPulseRenagMax,
	}
	if v := a.CfgGet("pulse_renag", ""); v != "" {
		if d, err := ParseInterval(v); err == nil {
			cfg.Renag = d
		}
	}
	if v := a.CfgGet("pulse_renag_max", ""); v != "" {
		if d, err := ParseInterval(v); err == nil {
			cfg.RenagMax = d
		}
	}
	return cfg, nil
}

// PulsePath is state/pulse.yaml, the pulse's own fingerprint — machine-local
// like everything under state/ (gitignored).
func PulsePath(a *App) string { return filepath.Join(a.StateDir, "pulse.yaml") }

// PulseState is the ticker's whole persisted record: rangerhq-4ish's sensing
// fields (At, Conditions, Fingerprint) plus this bead's delivery bookkeeping
// — the fingerprint actually prompted, when, and the renag interval now in
// force. One file, one writer (pulseOnce), so delivery reads exactly what
// sensing just computed instead of a second store racing it.
type PulseState struct {
	At                  time.Time
	Conditions          []string
	Fingerprint         string
	PromptedFingerprint string        // "" once the condition set clears
	PromptedAt          time.Time     // zero until the first successful prompt
	RenagInterval       time.Duration // the backoff in force for the next re-prompt
}

// WritePulseState fingerprints the condition set and the delivery
// bookkeeping to disk. The ticker goroutine is this file's only writer; a
// second watch loop's writes and this one's interleave last-writer-wins,
// which ADR 0027's consequences accept — one loop per queue is the
// invariant (ADR 0011 §1) in practice, and a stale fingerprint costs one
// missed pulse, not a wrong one.
func WritePulseState(path string, s PulseState) error {
	var b strings.Builder
	fmt.Fprintf(&b, "at: %s\n", s.At.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "fingerprint: %s\n", s.Fingerprint)
	if len(s.Conditions) == 0 {
		fmt.Fprintf(&b, "conditions: []\n")
	} else {
		fmt.Fprintf(&b, "conditions:\n")
		for _, c := range s.Conditions {
			fmt.Fprintf(&b, "  - %s\n", c)
		}
	}
	fmt.Fprintf(&b, "prompted_fingerprint: %s\n", s.PromptedFingerprint)
	if s.PromptedAt.IsZero() {
		fmt.Fprintf(&b, "prompted_at:\n")
	} else {
		fmt.Fprintf(&b, "prompted_at: %s\n", s.PromptedAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "renag_interval: %s\n", s.RenagInterval)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ReadPulseState reads back state/pulse.yaml's delivery bookkeeping. A
// missing or unreadable file, or a field that fails to parse, reads as that
// field's zero value — the pulse's first tick, not an error worth failing
// the loop over.
func ReadPulseState(path string) PulseState {
	var s PulseState
	s.PromptedFingerprint = YamlGet(path, "prompted_fingerprint")
	if v := YamlGet(path, "prompted_at"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			s.PromptedAt = t
		}
	}
	if v := YamlGet(path, "renag_interval"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			s.RenagInterval = d
		}
	}
	return s
}

// pulseLoop runs the shop check every cfg.Interval until ctx ends — the
// watch loop's own lifetime, never a second loop of its own.
func (d *Dispatcher) pulseLoop(ctx context.Context, cfg PulseConfig) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.pulseOnce(cfg)
		}
	}
}

// pulseOnce is one tick: compute, decide whether the non-empty set is due
// for delivery (ADR 0027 §3-4), fingerprint + bookkeeping to disk, log
// non-empty.
func (d *Dispatcher) pulseOnce(cfg PulseConfig) {
	set, failed := ShopCheck(d.govInputs(cfg))
	// A store that could not be read is a condition set that is PARTIAL,
	// never one that is empty. Off a timer that is a line in the log, not a
	// halt: the tick delivers what it did see and says what it could not.
	for _, err := range failed {
		fmt.Fprintf(d.errw(), "pulse: shop check partial: %v\n", err)
	}
	conditions := set.Keys()
	path := PulsePath(d.App)
	state := ReadPulseState(path)
	state.At = d.now()
	state.Conditions = conditions
	state.Fingerprint = set.Fingerprint()

	if len(conditions) == 0 {
		// Cleared set resets the renag clock — the next non-empty set, even
		// one with the same fingerprint as a previous episode, is a fresh
		// prompt, not a renag.
		state.PromptedFingerprint = ""
		state.PromptedAt = time.Time{}
		state.RenagInterval = 0
	} else {
		d.deliverPulse(cfg, &state)
	}

	if err := WritePulseState(path, state); err != nil {
		fmt.Fprintf(d.errw(), "pulse: cannot write %s: %v\n", AbbrevHome(path), err)
	}
	if len(conditions) > 0 {
		// The watch-log rendering (the third of the three): the same stable
		// tokens the prompt carries and the fingerprint is made of, so the
		// blocked-time-to-intervention metric can be read straight out of
		// this log against herdr's state changes.
		fmt.Fprintf(d.Out, "pulse: %s\n", GovLines(set))
	}
}

// govInputs is the pulse tick's view of the shop check: the watch process's
// own guard streak (G4 lives nowhere else) and its cost-scan seam, with the
// pulse persona marked live so the `no-live:` carry-over keeps meaning what
// it meant before the widening.
//
// Errw is left nil on purpose. A config typo is worth one line where a human
// asked (`posse status`, dispatch's own passes) and is noise written every
// two minutes forever here.
func (d *Dispatcher) govInputs(cfg PulseConfig) GovInputs {
	return GovInputs{
		App:               d.App,
		HB:                d.HB,
		Bd:                d.Bd,
		Now:               d.now,
		Caller:            "pulse",
		PulsePersona:      cfg.Persona,
		Pulsing:           true,
		GuardTrippedSince: d.guardStreak(),
		Spend:             d.Spend,
		Plan:              d.Plan,
	}
}

// deliverPulse decides whether this tick's non-empty condition set is due
// for a prompt — a fingerprint not yet prompted, or an unchanged one whose
// renag interval has elapsed — then attempts idle-only delivery to
// cfg.Persona's live session. Only an actually-delivered prompt advances
// state's bookkeeping; a skip or an undeliverable tick leaves it untouched
// so the same fingerprint is retried next tick, never gated behind renag
// for a prompt that never went out.
//
// This targets cfg.Persona's session directly via herdr's AgentPrompt, not
// `posse prompt` or personaActive/crewHeld — the ADR 0008 §2 carve-out
// (amended by ADR 0027): it may reach a crew-marked session, and because
// nothing here calls MarkCrew/MarkCrewOnOperatorPrompt, it sets no crew
// mark. The prompt itself carries no authority; the stores stay the record.
func (d *Dispatcher) deliverPulse(cfg PulseConfig, state *PulseState) {
	changed := state.PromptedFingerprint != state.Fingerprint
	due := changed
	if !due && !state.PromptedAt.IsZero() {
		interval := state.RenagInterval
		if interval <= 0 {
			interval = cfg.Renag
		}
		due = state.At.Sub(state.PromptedAt) >= interval
	}
	if !due {
		return
	}

	if cfg.Persona == "" {
		// Neither pulse_persona: nor coordinator: is set, so there is no
		// target to deliver to — a different thing from a target that is
		// asleep, which is condition (c) and is sensed. Say so on the same
		// line the no-live case uses: an armed pulse that reaches nobody
		// must be visible in the watch log, not silent.
		fmt.Fprintf(d.Out, "pulse: %s → undeliverable (no pulse_persona: and no coordinator:)\n",
			strings.Join(state.Conditions, "; "))
		return
	}

	sessions, err := d.HB.Sessions()
	if err != nil {
		fmt.Fprintf(d.errw(), "pulse: cannot read sessions: %v\n", err)
		return
	}
	name, status, found := pulseTarget(sessions, cfg.Persona)
	if !found {
		// Condition (c) from the sensing bead — already in state.Conditions.
		// Never create a session to deliver into; log and retry next tick.
		fmt.Fprintf(d.Out, "pulse: %s → undeliverable (no live session for %s)\n",
			strings.Join(state.Conditions, "; "), cfg.Persona)
		return
	}
	if status != "idle" && status != "done" {
		reason := status
		if reason == "" {
			reason = "no agent"
		}
		fmt.Fprintf(d.Out, "pulse: skipped (%s)\n", reason)
		return
	}

	text := pulsePromptText(state.Conditions)
	if _, err := d.HB.H.AgentPrompt(name, text, false, 0); err != nil {
		fmt.Fprintf(d.errw(), "pulse: prompt failed for %s: %v\n", name, err)
		return
	}
	fmt.Fprintf(d.Out, "pulse: %s → prompted %s\n", strings.Join(state.Conditions, "; "), name)

	if changed {
		state.RenagInterval = cfg.Renag
	} else {
		next := state.RenagInterval * 2
		if next <= 0 {
			next = cfg.Renag
		}
		if cfg.RenagMax > 0 && next > cfg.RenagMax {
			next = cfg.RenagMax
		}
		state.RenagInterval = next
	}
	state.PromptedFingerprint = state.Fingerprint
	state.PromptedAt = state.At
}

// pulseTarget finds cfg.Persona's live session among sessions — first match
// by agent — returning its name and herdr status. ("", "", false) is
// condition (c): no live session for persona. Deliberately not filtered by
// Crew: the whole point of the ADR 0008 §2 carve-out is that this prompt may
// reach the operator's own conversation.
func pulseTarget(sessions []HerdrSession, persona string) (name, status string, found bool) {
	if persona == "" {
		// A session herdr reports with no agent would otherwise match, and
		// delivering a shop check into an arbitrary session is worse than
		// not delivering one.
		return "", "", false
	}
	for _, s := range sessions {
		if s.Agent == persona {
			return s.Name, s.Status, true
		}
	}
	return "", "", false
}

// pulsePromptText is the fixed 'Pulse check' marker plus the observed
// conditions as hints (ADR 0027 §3): the prompt carries no authority, so it
// tells the session what to re-verify rather than what to do — dispatch's
// own stores (rhq list, git, bd) stay the record. One line, like every
// other prompt this codebase assembles (workPrompt, LandingPrompt).
func pulsePromptText(conditions []string) string {
	return fmt.Sprintf(
		"Pulse check: %s — re-verify against live state (rhq list, git, bd) before acting, then work your standing intents. No authority; the stores stay the record.",
		strings.Join(conditions, "; "))
}
