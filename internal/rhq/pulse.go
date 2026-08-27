package rhq

// The pulse: a shop-check ticker inside the dispatch --watch process (ADR
// 0013 §1-2, rangerhq-4ish, from rangerhq-wxd). Sensing only — this bead
// computes the condition set, fingerprints it to state/pulse.yaml and logs
// it; nothing here prompts anyone. That is ADR 0013 §3-4 (rangerhq-44w1).
//
// It starts with the watch loop and dies with it (Watch's ctx), never a
// second loop of its own — a hand-typed pass never pulses, the same premise
// as Unattended (watch.go): only a timer with no human witness needs a
// shop check running behind it. Touches herdr and local git only, never bd
// and never the plan endpoint.

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

// PulseConfig is the pulse_* config family, autostart_* style (flat YAML,
// plugin/autostart.sh): presence of pulse_interval: is the arm switch.
// Renag/RenagMax belong to delivery (ADR 0013 §3-4, rangerhq-44w1) and are
// read here so the family lands in one config pass; this bead's ticker
// does not use them.
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

// ShopCheck computes the ADR 0013 §1 condition set:
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

// WritePulseState fingerprints the condition set to disk. The ticker
// goroutine is this file's only writer; a second watch loop's writes and
// this one's interleave last-writer-wins, which ADR 0013's consequences
// accept — one loop per queue is the invariant (ADR 0011 §1) in practice,
// and a stale fingerprint costs one missed pulse, not a wrong one.
func WritePulseState(path string, at time.Time, conditions []string) error {
	var s strings.Builder
	fmt.Fprintf(&s, "at: %s\n", at.UTC().Format(time.RFC3339))
	fmt.Fprintf(&s, "fingerprint: %s\n", strings.Join(conditions, "|"))
	if len(conditions) == 0 {
		fmt.Fprintf(&s, "conditions: []\n")
	} else {
		fmt.Fprintf(&s, "conditions:\n")
		for _, c := range conditions {
			fmt.Fprintf(&s, "  - %s\n", c)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s.String()), 0o644)
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

// pulseOnce is one tick: compute, fingerprint to disk, log non-empty.
func (d *Dispatcher) pulseOnce(cfg PulseConfig) {
	conditions, err := ShopCheck(d.HB, d.App.BeadsDirs(), cfg.Persona)
	if err != nil {
		fmt.Fprintf(d.errw(), "pulse: shop check failed: %v\n", err)
		return
	}
	path := PulsePath(d.App)
	if err := WritePulseState(path, d.now(), conditions); err != nil {
		fmt.Fprintf(d.errw(), "pulse: cannot write %s: %v\n", AbbrevHome(path), err)
	}
	if len(conditions) > 0 {
		fmt.Fprintf(d.Out, "pulse: %s\n", strings.Join(conditions, "; "))
	}
}
