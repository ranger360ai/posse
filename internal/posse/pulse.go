package posse

// The pulse: a shop-check ticker inside the dispatch --watch process (ADR
// 0027, rangerhq-wxd). Sensing (§1-2, rangerhq-4ish) computes the condition
// set and fingerprints it to state/pulse.yaml. Delivery (§3-4, rangerhq-44w1)
// decides, on a non-empty set, whether it is due — new fingerprint, or an
// unchanged one past one fixed renag interval — and if so prompts
// pulse_persona's live session idle-only, with no authority and no crew mark.
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
// state and is never a record anyone reads for truth — since ADR 0027's
// 2026-09-05 simplification it holds ONLY those two delivery fields, and
// the observation itself (time, conditions, current fingerprint) lives in
// memory for the length of one tick.
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
// `coordinator:` (ADR 0033 §1) — the same persona ADR 0027 §3 named, spelled
// as the config key that already holds it rather than as a name compiled in.
// The engine ships carrying no crew name (App.Coordinator's rangerhq-gk4k
// rule, ranger-base-q3gp): a fresh deployer whose config names neither gets
// "", which is a pulse with no target at all — sensed, drawn by `posse
// status`, delivered to nobody — and not a permanent no-live: condition for
// a persona that never existed here.
func pulsePersona(a *App) string { return a.CfgGet("pulse_persona", a.Coordinator()) }

// DefaultPulseRenag is the one repeat interval (ADR 0027 §3) when
// pulse_renag: is unset: re-prompt an unchanged condition set after 30m,
// and after 30m again, for as long as it stands.
//
// It used to double per repeat up to a pulse_renag_max: of 4h, and the
// operator ruling of 2026-09-05 took the ladder out because 117.3h of
// dispatch-watch.log never climbed it: 397 deliveries fell into 386
// episodes of one identical key set each, 11 of them repeated once, and
// NOT ONE reached a second repeat — so the doubled 60m was computed,
// written to disk and never read, and the 4h cap never bound at all
// (ranger-base-thm0j). The set churns faster than the ladder: a new
// fingerprint is a fresh prompt, and that is the common case by 35:1.
const DefaultPulseRenag = 30 * time.Minute

// PulseConfig is the pulse_* config family, autostart_* style (flat YAML,
// plugin/autostart.sh): presence of pulse_interval: is the arm switch.
// Renag is delivery's one repeat interval (ADR 0027 §3-4).
type PulseConfig struct {
	Armed    bool
	Interval time.Duration
	Persona  string
	Renag    time.Duration
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
	}
	if v := a.CfgGet("pulse_renag", ""); v != "" {
		if d, err := ParseInterval(v); err == nil {
			cfg.Renag = d
		}
	}
	// pulse_renag_max: is gone with the ladder it capped (ADR 0027,
	// 2026-09-05). A config that still carries the key is not an error —
	// CfgGet is never asked for it, so it reads as any other unknown key
	// does, which is how every retired key in this family has left.
	return cfg, nil
}

// PulsePath is state/pulse.yaml, the pulse's delivery record — machine-local
// like everything under state/ (gitignored).
func PulsePath(a *App) string { return filepath.Join(a.StateDir, "pulse.yaml") }

// PulseState is the ticker's whole persisted record, and since ADR 0027's
// 2026-09-05 simplification that is delivery bookkeeping and nothing else:
// the fingerprint actually prompted, and when. One file, one writer
// (pulseOnce).
//
// It used to carry four more fields. Three of them — at, conditions,
// fingerprint — were rangerhq-4ish's sensing snapshot, written every tick
// and read back by nobody: ReadPulseState has never parsed them, and no
// other reader in this codebase opens the file (`posse status` recomputes
// the set from the stores, which is the rule that state/pulse.yaml is never
// evidence the shop is healthy). The fourth, renag_interval, was the
// doubling ladder's position, and the ladder is gone with it.
//
// What that costs: a human reading state/pulse.yaml no longer sees what the
// last tick observed. That reading was never trustworthy anyway — it is the
// last tick's snapshot, not the shop — and the watch log's own `pulse:` line
// carries the same keys with a pass header dating them.
type PulseState struct {
	PromptedFingerprint string    // "" once the condition set clears
	PromptedAt          time.Time // zero until the first successful prompt
}

// pulseTick is one tick's OBSERVATION: computed in memory, handed to
// delivery, and dropped when the tick ends (ADR 0027 — "compute observation
// time, conditions and current fingerprint in memory each tick"). Nothing
// here reaches state/pulse.yaml.
type pulseTick struct {
	At          time.Time
	Conditions  []string
	Fingerprint string
}

// WritePulseState writes the delivery bookkeeping to disk, whole-file, the
// way it always has (ADR 0027: the existing file replacement, no new writer
// and no lock service). The ticker goroutine is this file's only writer; a
// second watch loop's writes and this one's interleave last-writer-wins,
// which ADR 0027's consequences accept — one loop per queue is the
// invariant (ADR 0011 §1) in practice, and a stale fingerprint costs one
// missed pulse, not a wrong one.
//
// It writes on every tick, including the ticks that change nothing, and
// that is load-bearing: the file's EXISTENCE is what says an armed pulse
// ran at all, which is the only difference a disarmed watch leaves behind.
func WritePulseState(path string, s PulseState) error {
	var b strings.Builder
	fmt.Fprintf(&b, "prompted_fingerprint: %s\n", s.PromptedFingerprint)
	if s.PromptedAt.IsZero() {
		fmt.Fprintf(&b, "prompted_at:\n")
	} else {
		fmt.Fprintf(&b, "prompted_at: %s\n", s.PromptedAt.UTC().Format(time.RFC3339))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ReadPulseState reads back state/pulse.yaml's delivery bookkeeping. A
// missing or unreadable file, or a field that fails to parse, reads as that
// field's zero value — the pulse's first tick, not an error worth failing
// the loop over.
//
// This is also the whole migration for the six-field files written before
// the 2026-09-05 simplification: it asks YamlGet for the two keys it wants
// and never for the four that left, so an old record loads its dedup and
// the first tick after the upgrade rewrites the file two keys wide. No
// migration job, and no version stamp to get wrong (ADR 0027).
func ReadPulseState(path string) PulseState {
	var s PulseState
	s.PromptedFingerprint = YamlGet(path, "prompted_fingerprint")
	if v := YamlGet(path, "prompted_at"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			s.PromptedAt = t
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

// pulseOnce is one tick: compute in memory, decide whether the non-empty
// set is due for delivery (ADR 0027 §3-4), delivery bookkeeping to disk,
// log non-empty. The observation itself is never written.
func (d *Dispatcher) pulseOnce(cfg PulseConfig) {
	set, failed := ShopCheck(d.govInputs(cfg))
	// A store that could not be read is a condition set that is PARTIAL,
	// never one that is empty. Off a timer that is a line in the log, not a
	// halt: the tick delivers what it did see and says what it could not.
	for _, err := range failed {
		d.equietf("pulse: shop check partial: %v\n", err)
	}
	tick := pulseTick{At: d.now(), Conditions: set.Keys(), Fingerprint: set.Fingerprint()}
	path := PulsePath(d.App)
	state := ReadPulseState(path)

	if len(tick.Conditions) == 0 {
		// Cleared set resets the renag clock — the next non-empty set, even
		// one with the same fingerprint as a previous episode, is a fresh
		// prompt, not a renag.
		//
		// "Cleared" is only ever a statement about what this tick COMPUTED.
		// The partial lines above are the other half of it: a store that
		// could not be read contributes no keys, so an empty set here can
		// mean "nothing is wrong" or "we could not look", and the reset
		// claims the first while the log says which it was. What it must
		// never do is claim an unobserved condition cleared, and it does
		// not — it forgets a delivery, it does not assert an all-clear, and
		// nothing downstream reads this file for one (ADR 0027).
		state.PromptedFingerprint = ""
		state.PromptedAt = time.Time{}
	} else {
		d.deliverPulse(cfg, tick, &state)
	}

	if err := WritePulseState(path, state); err != nil {
		d.equietf("pulse: cannot write %s: %v\n", AbbrevHome(path), err)
	}
	if len(tick.Conditions) > 0 {
		// The shop pulse (ranger-base-dwlb1), on its OWN line above the
		// conditions rather than appended to them: the condition line is
		// the stable-token rendering a metric is greped out of, and a
		// moving number on it would break every reader of that log.
		//
		// It is computed only on the passes that print — a healthy shop
		// pays nothing, and a ticker that ran a `bd list --all` per repo
		// every two minutes forever to say the same number would be the
		// cost ADR 0027's boundary was protecting against.
		p, failedPulse := d.App.ReadBeadPulse(d.Bd, d.now())
		for _, ln := range PulseFailureLines(failedPulse) {
			d.equietf("%s\n", ln)
		}
		d.quietf("pulse: shop %s\n", p.Line())
		// The watch-log rendering (the third of the three): the same stable
		// tokens the prompt carries and the fingerprint is made of, so the
		// blocked-time-to-intervention metric can be read straight out of
		// this log against herdr's state changes.
		d.quietf("pulse: %s\n", GovLines(set))
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

// pulseTakesPrompt is the pulse's state rule, in one place because it is
// asked twice: once of herdr's agent listing as a cheap pre-filter, once of
// the detection the readiness gate opened on, which is the answer that
// counts. A shop check is an unsolicited nudge — it goes to a persona that
// has stopped, never into a turn in progress.
func pulseTakesPrompt(status string) bool { return status == "idle" || status == "done" }

// deliverPulse decides whether this tick's non-empty condition set is due
// for a prompt — a fingerprint not yet prompted, or an unchanged one whose
// one renag interval has elapsed since the last SUCCESSFUL delivery — then
// attempts idle-only delivery to cfg.Persona's live session. Only an
// actually-delivered prompt advances state's bookkeeping; a skip or an
// undeliverable tick leaves it untouched so the same fingerprint is retried
// next tick, never gated behind renag for a prompt that never went out.
//
// Both of those are best-effort, not exactly-once: an unreadable delivery
// record reads as "never prompted" and a crash between AgentPrompt here and
// pulseOnce's WritePulseState loses the record of a prompt that did go out. Each
// costs one repeat, which is the same cost the renag interval already
// budgets for, and the alternative — a durable log, a lock, a second store
// — is the machinery ADR 0027 declines to add for a hint that carries no
// authority.
//
// This targets cfg.Persona's session directly via herdr's AgentPrompt, not
// `posse prompt` or personaActive/crewHeld — the ADR 0008 §2 carve-out
// (amended by ADR 0027): it may reach a crew-marked session, and because
// nothing here calls MarkCrew/MarkCrewOnOperatorPrompt, it sets no crew
// mark. The prompt itself carries no authority; the stores stay the record.
func (d *Dispatcher) deliverPulse(cfg PulseConfig, tick pulseTick, state *PulseState) {
	changed := state.PromptedFingerprint != tick.Fingerprint
	due := changed || (!state.PromptedAt.IsZero() && tick.At.Sub(state.PromptedAt) >= cfg.Renag)
	if !due {
		return
	}

	if cfg.Persona == "" {
		// Neither pulse_persona: nor coordinator: is set, so there is no
		// target to deliver to — a different thing from a target that is
		// asleep, which is condition (c) and is sensed. Say so on the same
		// line the no-live case uses: an armed pulse that reaches nobody
		// must be visible in the watch log, not silent.
		d.quietf("pulse: %s → undeliverable (no pulse_persona: and no coordinator:)\n",
			strings.Join(tick.Conditions, "; "))
		return
	}

	sessions, err := d.HB.Sessions()
	if err != nil {
		d.equietf("pulse: cannot read sessions: %v\n", err)
		return
	}
	name, pane, status, found := pulseTarget(sessions, cfg.Persona)
	if !found {
		// Condition (c) from the sensing bead — already in state.Conditions.
		// Never create a session to deliver into; log and retry next tick.
		d.quietf("pulse: %s → undeliverable (no live session for %s)\n",
			strings.Join(tick.Conditions, "; "), cfg.Persona)
		return
	}
	if !pulseTakesPrompt(status) {
		reason := status
		if reason == "" {
			reason = "no agent"
		}
		d.quietf("pulse: skipped (%s)\n", reason)
		return
	}
	// ...and now the same question again, of evidence rather than of the
	// listing (ranger-base-k99a). `status` above came from herdr's agent
	// listing, which answers `idle` for a pane holding a known agent that
	// NO rule matched — the guess that let ranger-base-3p0 type a work
	// prompt into a CLI which did not have the keyboard yet, and turned it
	// into `/Work`. So the check above is only the cheap pre-filter that
	// keeps a visibly-working persona from costing a startup wait; the
	// gate is what decides, and it decides on a screen herdr has SEEN.
	//
	// Two readings, two rules. AwaitPromptable's own rule is "a CLI that
	// holds the keyboard", which is weaker than the pulse's: `posse prompt`
	// by hand may nudge an agent mid-turn, but a shop check is a nudge
	// nobody asked for and must not land on one. So the pulse re-applies
	// its idle|done rule to what the gate actually saw. Only on positive
	// evidence: the never-answered concession returns a zero detection, and
	// refusing a pulse because a diagnostic verb is missing would silence
	// the shop check against an older herdr entirely.
	det, note, err := d.HB.AwaitPromptable(name, pane)
	if err != nil {
		d.quietf("pulse: skipped (%s not promptable: %v)\n", name, err)
		return
	}
	if note != "" {
		d.quietf("pulse: %s\n", note)
	}
	if det.Seen() && !pulseTakesPrompt(det.State) {
		d.quietf("pulse: skipped (%s — herdr's listing said %q)\n", det.State, status)
		return
	}
	// ranger-base-htafy put a gate here and ranger-base-wr624 took it out.
	// It read the composer off the detection already in hand and SKIPPED on
	// text, because a composer with text in it is a prompt somebody sent
	// that never submitted, and a shop check typed after it makes one
	// garbled message out of two.
	//
	// MEASURED 2026-09-04, three episodes in one day: ~586 consecutive
	// skips naming text the operator had already SENT and the persona had
	// already answered, against a box `posse peek` showed EMPTY. herdr's
	// composer state for that pane still carried the last sent line after
	// the send, so every tick read the same phantom and the pulse arm was
	// off for ~10 of the day's hours while 108 commits stacked behind it.
	//
	// A screen-state matcher over a region that can hold ghost text cannot
	// clear itself: nothing the pulse or the persona can do makes the
	// reading go false — only a NEW operator message replaces the phantom,
	// which is the opposite of the condition the gate was written for. And
	// this pane is the harness's own arm; the operator types in it only to
	// talk to the persona the pulse is addressing. So the reading no longer
	// decides. What it was avoiding is one re-typed line on the rare tick
	// that lands mid-keystroke; what it charged was a day of pushes.
	//
	// It still REPORTS, so a garbled turn is explicable after the fact from
	// the watch log, and so that a herdr which starts clearing the composer
	// on submit is visible here as the line simply going quiet.
	//
	// Narrow on purpose. The same reading still gates dispatch's --resume
	// re-prompt and govern's G2 row (panework.go's other callers), which
	// are about a DISPATCHED holder's pane and fail towards not acting.
	// Neither has been shown to see the phantom, and widening a narrowing
	// nobody measured is how the first one got written; ranger-base-2hvtv
	// carries the discriminator and the two unmeasured questions.
	if typed := det.Composer(); typed != "" {
		d.quietf("pulse: %s box previews %q — prompting anyway; that reading has been ghost text (ranger-base-wr624)\n",
			name, ellipsis(typed, 60))
	}

	text := pulsePromptText(tick.Conditions)
	if _, err := d.HB.H.AgentPrompt(pane, text, false, 0); err != nil {
		d.equietf("pulse: prompt failed for %s: %v\n", name, err)
		return
	}
	d.quietf("pulse: %s → prompted %s\n", strings.Join(tick.Conditions, "; "), name)

	// Delivered, so the clock restarts here and nowhere else — same two
	// fields whether this was a new fingerprint or a repeat, because there
	// is one interval now and no ladder position to advance.
	state.PromptedFingerprint = tick.Fingerprint
	state.PromptedAt = tick.At
}

// pulseTarget finds cfg.Persona's live session among sessions — first match
// by agent — returning its session name, herdr status, and the pane herdr's
// real AgentPrompt/AgentExplain addresses (HerdrSession.PaneID, the root pane
// on record from session creation — not the session name every other caller
// in this codebase resolves to a pane through, HerdrBackend.AgentTarget,
// before speaking to herdr; ranger-base-5qe6). ("", "", "", false) is
// condition (c): no live session for persona. Deliberately not filtered by
// Crew: the whole point of the ADR 0008 §2 carve-out is that this prompt may
// reach the operator's own conversation.
func pulseTarget(sessions []HerdrSession, persona string) (name, pane, status string, found bool) {
	if persona == "" {
		// A session herdr reports with no agent would otherwise match, and
		// delivering a shop check into an arbitrary session is worse than
		// not delivering one.
		return "", "", "", false
	}
	for _, s := range sessions {
		if s.Agent == persona {
			return s.Name, s.PaneID, s.Status, true
		}
	}
	return "", "", "", false
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
