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
// shop check running behind it. Sensing touches herdr and local git only,
// never bd and never the plan endpoint; delivery adds one more herdr call
// (AgentPrompt) and nothing else.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultPulsePersona is who the pulse watches for absence (condition c)
// when config pulse_persona: is unset.
const DefaultPulsePersona = "monica"

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
		Persona:  a.CfgGet("pulse_persona", DefaultPulsePersona),
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

// ShopCheck computes the ADR 0027 §1 condition set:
//
//   - (a) any live session whose agent herdr reports blocked
//   - (b) unpushed commits (@{u}..HEAD) on a config beads: repo — no
//     upstream configured reads as no condition, not an error
//   - (c) no live session for persona
//
// Each condition is one stable string; the caller sorts nothing further —
// the return is already sorted so it can be joined straight into a
// fingerprint and diffed by a later reader without parsing anything.
//
// Herdr and local git only — never bd, never the plan endpoint (the same
// boundary Unattended draws): this runs off a timer with no human watching
// it spend.
func ShopCheck(hb *HerdrBackend, beadsDirs []string, persona string) ([]string, error) {
	sessions, err := hb.Sessions()
	if err != nil {
		return nil, err
	}
	var conditions []string
	livePersona := false
	for _, s := range sessions {
		if s.Agent == persona {
			livePersona = true
		}
		if s.Status == "blocked" {
			conditions = append(conditions, "blocked:"+s.Name)
		}
	}
	if !livePersona {
		conditions = append(conditions, "no-live:"+persona)
	}

	seen := map[string]bool{}
	for _, dir := range beadsDirs {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		n, err := git(dir, "rev-list", "--count", "@{u}..HEAD")
		if err != nil {
			// No upstream (or not a repo at all) is the absence of a
			// condition, not a failure to read one — a repo this check
			// cannot ask reads the same as a repo with nothing to report,
			// which is the point: silence off a timer nobody is watching,
			// never a false alarm.
			continue
		}
		if count, err := strconv.Atoi(n); err == nil && count > 0 {
			conditions = append(conditions, fmt.Sprintf("unpushed:%s:%d", dir, count))
		}
	}

	sort.Strings(conditions)
	return conditions, nil
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
	conditions, err := ShopCheck(d.HB, d.App.BeadsDirs(), cfg.Persona)
	if err != nil {
		fmt.Fprintf(d.errw(), "pulse: shop check failed: %v\n", err)
		return
	}
	path := PulsePath(d.App)
	state := ReadPulseState(path)
	state.At = d.now()
	state.Conditions = conditions
	state.Fingerprint = strings.Join(conditions, "|")

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
		fmt.Fprintf(d.Out, "pulse: %s\n", strings.Join(conditions, "; "))
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
