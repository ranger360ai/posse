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
// Since ADR 0093 D2 a due tick also appends one DELTA line to
// state/pulse.feed — what moved since the last delivery, not the whole set
// — and state/pulse.yaml's dedup half is `delivered_keys` (the keys that
// were said) in place of the fingerprint it computed them into. The feed is
// the channel D1's `posse pulse wait` reads.
//
// And since D3 (ranger-base-r3qz6) the FEED is the primary channel: a due
// tick probes the waiter's flock, and a waiter that is listening gets the
// delta on the feed with nothing typed into any pane — which is what ends
// the splice class by construction. The keyboard is the fallback for a
// coordinator that is not armed, it says so in the text it types (`arm the
// feed:`), it refuses to type over text the operator left in the box on rex
// (D4), and `pulse-unarmed` carries the same fact to `posse status`
// (govern.go). `pulse_feed: off` is the exit hatch: no feed, no probe, no
// reminder, no condition, and ADR 0027's typed path exactly as it was.
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
	"sort"
	"strconv"
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

	// Feed is ADR 0093's channel: the delta feed, the waiter lock that
	// switches delivery onto it (D3), the typed fallback's reminder and the
	// `pulse-unarmed` condition. `pulse_feed: off` turns all four off and
	// leaves ADR 0027's typed path exactly as it was — the ADR's exit
	// hatch, and the default is ON.
	//
	// WHAT THE HATCH DOES NOT TURN OFF, said here because "unchanged" is
	// the ADR's word and this is a deliberate reading of it: D4's composer
	// hold stays armed (deliverPulse). The hatch exists so that a feed or a
	// waiter that misbehaves cannot silence the shop check; D4 is a guard
	// on the KEYBOARD, not part of the feed, and switching it off with the
	// feed would make the exit hatch a splice regression — the operator
	// would reach for it precisely when the pulse is typing most, which is
	// when the hold matters. ranger-base-r3qz6's bead carries this.
	Feed bool
}

// pulseFeedOff spells the hatch. `off`, `never`, `0` and `false` all read as
// off, which is graceAfter's vocabulary plus the boolean spelling
// model_preflight: uses — one shop, one way to say off.
//
// ANY OTHER VALUE READS AS ON, including a typo. Both directions cost
// something and this is the cheaper one: a mistyped hatch leaves the new
// channel running, where a rule of "anything but `on` is off" would let one
// mistyped character take the feed, the waiter and the condition out
// silently. It is not an error either — LoadPulseConfig's error disarms the
// whole pulse for the loop, which is far too much for a hatch nobody is
// obliged to set.
func pulseFeedOff(v string) bool {
	switch strings.TrimSpace(v) {
	case "off", "never", "0", "false":
		return true
	}
	return false
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
		Feed:     !pulseFeedOff(a.CfgGet("pulse_feed", "")),
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

// PulseFeedPath is state/pulse.feed, the delta feed a due tick appends to
// (ADR 0093 D2) and `posse pulse wait` reads (D1) — machine-local under
// state/ like the record beside it, and deliberately a plain text file one
// line per delivery, so a human can tail it and the waiter needs nothing but
// the first field of a line.
//
// It holds no state anything else depends on (ADR 0093's exit hatch): lose
// the file and the next delivery starts a fresh sequence, which costs an
// armed waiter one `gap`.
func PulseFeedPath(a *App) string { return filepath.Join(a.StateDir, "pulse.feed") }

// PulseFeedLines is the cap: an append keeps the newest 500 entries and
// drops the rest (ADR 0093 D2).
//
// By LINE COUNT and not by age or bytes, because the one reader that cares
// asks for a sequence number — `--after <seq>` — and a cap on lines is the
// only cap whose consequence for that reader is statable: at most 500
// entries of backlog, and a `--after` older than the oldest kept entry gets
// `gap`. At the measured 4.6 deliveries an hour (ADR 0093's 70h window)
// that is about four and a half days, and the deltas this feed carries are
// much quieter than the sets they replaced.
const PulseFeedLines = 500

// pulseWedgeBound is how old a lock-holding waiter may be before D3's probe
// reads it as wedged rather than as a listener: one pulse_renag plus one
// tick (ranger-base-r3qz6's bead, from ADR 0093's "a waiter older than that
// is wedged").
//
// WHY THE EXTRA TICK. The waiter's own maximum life is exactly pulse_renag
// (pulseWaitLife), so a healthy waiter armed one instant before a tick is
// `renag` old at the tick that would judge it — and judging it wedged there
// would be judging a waiter that is at this moment exiting on purpose, with
// its replacement a turn away. The slack is one INTERVAL because the thing
// being tolerated is the gap between the waiter's clock and the tick that
// reads it, and that gap is at most one tick by construction.
//
// A disarmed or unconfigured pulse answers zero, which ages nothing: a
// bound guessed from defaults would judge a holder against an interval this
// instance does not run.
func pulseWedgeBound(cfg PulseConfig) time.Duration {
	if cfg.Renag <= 0 {
		return 0
	}
	return cfg.Renag + cfg.Interval
}

// PulseState is the ticker's whole persisted record, and since ADR 0027's
// 2026-09-05 simplification that is delivery bookkeeping and nothing else:
// the keys actually delivered, and when. One file, one writer (pulseOnce).
//
// `delivered_keys` replaced `prompted_fingerprint` with ADR 0093 D2, and it
// is the same datum one step less cooked: the fingerprint was these keys
// joined, and a dedup is all a joined string can answer. A DELTA needs the
// keys themselves — which appeared, which cleared, which one's count moved —
// so the field now holds what the fingerprint was made of. Dedup is
// unchanged by it: an empty delta is exactly a fingerprint that matched.
//
// ADR 0093 D2 is explicit that this partly reverses ADR 0027's "no persisted
// conditions", and why it is not the thing that rule forbade: this field is
// what was SAID, not a mirror of status, and nothing reads it as health —
// `posse status` still recomputes the set from the stores.
//
// The old field needs no migration job, for the third time in this file's
// life and by the same mechanism: the reader asks for the key it wants and
// never for the one that left, so a `prompted_fingerprint:` record loads
// delivered_keys EMPTY — delivery is unknown, not known-empty, and both
// render the same way — and the first due tick after the upgrade says the
// whole standing set as `+`. One re-announcement per upgrade, which is the
// cost the renag interval already budgets for.
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
	DeliveredKeys []string  // nil once the condition set clears
	PromptedAt    time.Time // zero until the first successful prompt
}

// Fingerprint is the delivered keys joined the way GovSet.Fingerprint joins
// a computed set's, so the two compare directly. It is the dedup question
// and nothing else; the delta is pulseDiff's.
func (s PulseState) Fingerprint() string { return strings.Join(s.DeliveredKeys, "|") }

// pulseTick is one tick's OBSERVATION: computed in memory, handed to
// delivery, and dropped when the tick ends (ADR 0027 — "compute observation
// time, conditions and current fingerprint in memory each tick"). Nothing
// here reaches state/pulse.yaml.
//
// The fingerprint left with ADR 0093 D2 and is not replaced by a second
// field: delivery's question is now the delta, which is computed from
// Conditions against the record, and a joined copy carried beside them would
// be a second spelling of the same answer for nobody to read. `set.Keys()`
// is sorted, so PulseState.Fingerprint() still reconstructs it exactly where
// the dedup question is asked of the RECORD.
type pulseTick struct {
	At         time.Time
	Conditions []string

	// Partial says a store this tick asked could not be read, so the
	// condition set is what was SEEN and not what is there (pulseOnce's
	// `failed`). It rides on the observation because that is what it is a
	// property of, and delivery needs it: under ADR 0093 D2 a partial read
	// is never a clear, so a key that vanished from an unreadable store is
	// HELD rather than marked `-`.
	Partial bool
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
	fmt.Fprintf(&b, "delivered_keys: %s\n", strings.Join(s.DeliveredKeys, "|"))
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
// This is also the whole migration for every earlier shape of the file: it
// asks YamlGet for the two keys it wants and never for the ones that left —
// the six-field record's four (2026-09-05) and `prompted_fingerprint` (ADR
// 0093 D2) — so an old record loads what still exists and the first tick
// after the upgrade rewrites the file two keys wide. No migration job, and
// no version stamp to get wrong (ADR 0027).
//
// `delivered_keys` is read the way it is written, `|`-joined on one flat
// line, which is the shape the fingerprint already had: every YamlGet
// property the old field relied on (the " #" cut, the matched-quote strip,
// a value that is one line) is the same property, over the same bytes.
func ReadPulseState(path string) PulseState {
	var s PulseState
	for _, k := range strings.Split(YamlGet(path, "delivered_keys"), "|") {
		if k = strings.TrimSpace(k); k != "" {
			s.DeliveredKeys = append(s.DeliveredKeys, k)
		}
	}
	if v := YamlGet(path, "prompted_at"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			s.PromptedAt = t
		}
	}
	return s
}

// pulseMove is one condition whose trailing COUNT moved while the condition
// itself stood: `unpushed:<dir>:5` becoming `unpushed:<dir>:7`. Stem is the
// key without that count, From and To the counts as written.
type pulseMove struct{ Stem, From, To string }

// pulseDelta is what one due tick has to SAY: the keys that appeared, the
// keys that cleared, and the counted keys whose number moved (ADR 0093 D2).
// Held is the fourth outcome and the only one that is not news — delivered
// keys this tick could not see, because a store could not be read, which are
// carried forward rather than announced as cleared.
type pulseDelta struct {
	Appeared []string
	Cleared  []string
	Moved    []pulseMove
	Held     []string
}

// Empty is "nothing to say", which is also exactly the old dedup answer: a
// delta with no appeared, cleared or moved key is a condition set whose
// fingerprint matched the delivered one. Held does not count — a key that
// went unread is not a change, and announcing one would turn every outage
// of a store into a delivery.
func (d pulseDelta) Empty() bool {
	return len(d.Appeared) == 0 && len(d.Cleared) == 0 && len(d.Moved) == 0
}

// pulseCountStem splits a condition key that ends in a decimal count:
// `unpushed:/src/posse:7` → (`unpushed:/src/posse`, `7`, true). Anything
// else is (…, false).
//
// GENERIC on purpose, rather than keyed on the one prefix that has a count
// today. The whole point of D2's `~` is that a number ticking over is one
// condition standing and not a condition clearing beside a new one
// appearing; a rule that knew only `unpushed:` would render the next counted
// key somebody adds as a `+`/`-` pair, which is the noise this exists to
// remove, and it would do it silently. The cost of being generic is that a
// key whose trailing number is not a count reads as one — `guard-credential:2`
// after `guard-credential:1` renders `~guard-credential:1→2` — and that line
// is still true and still shorter than the pair it replaces.
func pulseCountStem(key string) (stem, count string, ok bool) {
	i := strings.LastIndex(key, ":")
	if i <= 0 || i == len(key)-1 {
		return "", "", false
	}
	n := key[i+1:]
	for j := 0; j < len(n); j++ {
		if n[j] < '0' || n[j] > '9' {
			return "", "", false
		}
	}
	return key[:i], n, true
}

// pulseDiff is the delta from the keys last delivered to the keys this tick
// computed. Both arrive sorted (GovSet is sorted by Key, and delivered_keys
// was written from such a set), and every output here preserves that order,
// so a feed line's tokens read in the same order the keys do everywhere else
// in this codebase.
//
// partial is the tick's own Partial: with a store unread, a key that
// disappeared is HELD rather than cleared (ADR 0093 D2). The hold is blanket
// rather than attributed, and that is a deliberate widening of the ADR's
// sentence: ShopCheck's `failed` carries errors, not the keys those errors
// would have produced, so there is no way to tell which vanished key belongs
// to the store that refused. Holding every vanished key on a partial tick is
// the only version of the rule this code can actually enforce, and it fails
// in the direction the rule names — a condition held one tick too long,
// never an unobserved condition announced as cleared. Keys that APPEARED are
// still news on a partial tick: those were seen.
//
// A move survives a partial tick for the same reason — both halves of it
// were observed, the old count in the record and the new one on the screen.
func pulseDiff(delivered, now []string, partial bool) pulseDelta {
	var d pulseDelta
	had := make(map[string]bool, len(delivered))
	for _, k := range delivered {
		had[k] = true
	}
	have := make(map[string]bool, len(now))
	for _, k := range now {
		have[k] = true
	}

	var gone, fresh []string
	for _, k := range delivered {
		if !have[k] {
			gone = append(gone, k)
		}
	}
	for _, k := range now {
		if !had[k] {
			fresh = append(fresh, k)
		}
	}

	// Pair what left against what arrived, by stem. A set is keyed by the
	// WHOLE key, so one stem can contribute at most one gone key and one
	// fresh key, and first-wins is therefore total rather than a tie-break.
	goneCount := map[string]string{}
	for _, k := range gone {
		if stem, n, ok := pulseCountStem(k); ok {
			if _, dup := goneCount[stem]; !dup {
				goneCount[stem] = n
			}
		}
	}
	paired := map[string]bool{}
	for _, k := range fresh {
		stem, to, ok := pulseCountStem(k)
		if !ok {
			continue
		}
		from, ok := goneCount[stem]
		if !ok {
			continue
		}
		d.Moved = append(d.Moved, pulseMove{Stem: stem, From: from, To: to})
		paired[k] = true
		paired[stem+":"+from] = true
		// Consumed, so this is a MATCHING and not a lookup: one gone key
		// cannot be the `from` of two moves. Not reachable through today's
		// keys — the only other counted key, `hook-wall:<slot>:<config>`,
		// carries a config path and never bare digits, so no stem can have
		// two members of one set — but the rule above is generic by design
		// and a second counted family is exactly what it is generic for.
		delete(goneCount, stem)
	}

	for _, k := range fresh {
		if !paired[k] {
			d.Appeared = append(d.Appeared, k)
		}
	}
	for _, k := range gone {
		switch {
		case paired[k]:
		case partial:
			d.Held = append(d.Held, k)
		default:
			d.Cleared = append(d.Cleared, k)
		}
	}
	return d
}

// pulseDelivered is what delivered_keys becomes once this tick's line is
// out: what the tick saw, plus what it held. Sorted, because delivered_keys
// is compared against a sorted GovSet's fingerprint and a held key spliced
// in at the end would read as a changed set forever after.
func pulseDelivered(now, held []string) []string {
	if len(held) == 0 {
		return now
	}
	out := make([]string, 0, len(now)+len(held))
	out = append(out, now...)
	out = append(out, held...)
	sort.Strings(out)
	return out
}

// pulseFeedBody renders one entry's payload — everything after the sequence
// number and the timestamp (ADR 0093 D2).
//
// A delta is sigil-per-token and space-separated, `+` then `-` then `~`, in
// the ADR's own order; `standing: N` closes it with a count and no keys,
// because the full set is `posse status`'s to print and re-reading it here
// is the thing D2 replaced. Every token is space-free — a move is rendered
// as its key with the count replaced (`~unpushed:/src/posse:5→7`) rather
// than as the ADR's prose `unpushed 5→7` — so the whole line splits on
// whitespace and `posse pulse wait` needs no grammar to find the sequence
// number or to count what moved.
//
// An empty delta is the renag, and it is the one line that names keys: the
// coordinator is being told nothing has changed, so "nothing has changed
// about WHAT" has to be on the line or it carries no information at all.
func pulseFeedBody(d pulseDelta, standing []string) string {
	if d.Empty() {
		return "still: " + strings.Join(standing, "; ")
	}
	tok := make([]string, 0, len(d.Appeared)+len(d.Cleared)+len(d.Moved)+1)
	for _, k := range d.Appeared {
		tok = append(tok, "+"+k)
	}
	for _, k := range d.Cleared {
		tok = append(tok, "-"+k)
	}
	for _, m := range d.Moved {
		tok = append(tok, fmt.Sprintf("~%s:%s→%s", m.Stem, m.From, m.To))
	}
	tok = append(tok, fmt.Sprintf("standing: %d", len(standing)))
	return strings.Join(tok, " ")
}

// pulseFeedEntries reads the feed's kept entries. A missing or unreadable
// feed reads as no entries — the first delivery on a fresh instance, not an
// error worth failing a tick over, same rule as ReadPulseState's.
func pulseFeedEntries(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// PulseFeedSeq is an entry's sequence number, the first whitespace-separated
// field. (0, false) for a line that does not start with one — exported
// because the waiter's `--after <seq>` compares against exactly this
// reading, and two implementations of a line format is how a feed and its
// reader drift.
func PulseFeedSeq(line string) (int, bool) {
	f, _, ok := strings.Cut(strings.TrimSpace(line), " ")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(f)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// pulseFeedMaxSeq is the highest sequence number the feed can still account
// for, and 0 for a feed with no readable entry at all. The MAXIMUM and not
// the last line's, for the reason pulseFeedWrite's own note gives: a corrupt
// tail would otherwise restart the sequence.
//
// Two readers, one reading (ranger-base-r3qz6): the next sequence number to
// hand out is this plus one, and the cursor the typed fallback tells the
// coordinator to arm with is this exactly — the newest entry it could
// already have. Arming with it means the coordinator's first wake carries
// the delta the fallback just typed, once, which is the direction that
// repeats a line rather than losing one.
func pulseFeedMaxSeq(entries []string) int {
	max := 0
	for _, ln := range entries {
		if n, ok := PulseFeedSeq(ln); ok && n > max {
			max = n
		}
	}
	return max
}

// pulseFeedCursor is pulseFeedMaxSeq over the feed on disk.
func pulseFeedCursor(path string) int { return pulseFeedMaxSeq(pulseFeedEntries(path)) }

// pulseFeedWrite appends one entry to the feed and returns its sequence
// number. Read, append, cap, atomic rewrite — a whole-file writer like
// WritePulseState beside it, and for the same reason: the ticker goroutine
// is the only writer, so the cheapest correct thing is to own the file.
//
// The rewrite goes through a temp file and a rename because this one has a
// READER that is not the writer: `posse pulse wait` may be mid-read at any
// moment, and a reader of an O_APPEND-then-truncate feed can see a file with
// its oldest lines gone and its newest not yet there. A rename is one step
// for that reader.
//
// The sequence number is the MAXIMUM parseable one in the file plus one, not
// the last line's: a corrupt tail would otherwise restart the sequence and
// hand an armed waiter numbers it has already passed, which looks exactly
// like nothing happening.
func pulseFeedWrite(path string, at time.Time, body string) (int, error) {
	entries := pulseFeedEntries(path)
	seq := pulseFeedMaxSeq(entries) + 1
	entries = append(entries, fmt.Sprintf("%d %s %s", seq, at.UTC().Format(time.RFC3339), body))
	if len(entries) > PulseFeedLines {
		entries = entries[len(entries)-PulseFeedLines:]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(entries, "\n")+"\n"), 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	return seq, nil
}

// appendPulseFeed is pulseFeedWrite with this loop's logging: the entry also
// goes to the watch log, on its own `pulse: feed` line, so the feed's rate
// and the typed path's can be read against each other out of one log — which
// is what ADR 0093's done-when rows 1 and 3 ask for.
//
// WHETHER A FAILED APPEND IS A FAILED DELIVERY DEPENDS ON THE ARM, which is
// why this hands back `ok` since ADR 0093 D3 (ranger-base-r3qz6). On the
// typed arm the prompt has already gone out by the time this is called, so a
// failed append costs the feed one line and the delivery nothing — the
// record still advances, because what it records is that the coordinator was
// told. On the FEED arm the append IS the delivery: a line that did not land
// said nothing, so the record must not advance and the next tick retries.
// Both readings are at the call sites; what is here is the diagnostic, which
// is the same either way.
func (d *Dispatcher) appendPulseFeed(at time.Time, body string) (int, bool) {
	path := PulseFeedPath(d.App)
	seq, err := pulseFeedWrite(path, at, body)
	if err != nil {
		d.equietf("pulse: cannot append %s: %v\n", AbbrevHome(path), err)
		return 0, false
	}
	d.quietf("pulse: feed %d %s\n", seq, body)
	return seq, true
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
	// The suspend pair FIRST, before anything else on this tick reads a
	// clock (ADR 0064 D3). After a wake the witness's 30s ticker and this
	// loop's 2m one both resume with whatever was left on each, so the
	// witness is not guaranteed to have seen the gap by the time this tick
	// computes its set — about one wake in eight lands this way — and a
	// wall-clock condition built from a ledger that has not been told yet
	// is the false URGENT this whole page exists to end (the 2026-10-01
	// wake: `guard-blind:5h; loop-mute` one second after a 5h30m sleep).
	//
	// Calling it here cannot double-count: suspendTick is under mu and
	// re-seeds the pair, so whichever goroutine sees the gap records it and
	// prints the line, and the other measures an awake box. The write also
	// freshens this log's mtime, which would clear `loop-mute` on its own
	// — incidental, not the mechanism; the subtraction is what is pinned.
	d.suspendTick()
	set, failed := ShopCheck(d.govInputs(cfg))
	// A store that could not be read is a condition set that is PARTIAL,
	// never one that is empty. Off a timer that is a line in the log, not a
	// halt: the tick delivers what it did see and says what it could not.
	for _, err := range failed {
		d.equietf("pulse: shop check partial: %v\n", err)
	}
	tick := pulseTick{
		At:         d.now(),
		Conditions: set.Keys(),
		Partial:    len(failed) > 0,
	}
	path := PulsePath(d.App)
	state := ReadPulseState(path)

	if len(tick.Conditions) == 0 {
		// "Cleared" is only ever a statement about what this tick COMPUTED,
		// and the partial lines above are the other half of it: a store that
		// could not be read contributes no keys, so an empty set here can
		// mean "nothing is wrong" or "we could not look".
		//
		// Until ADR 0093 D2 the tick reset the bookkeeping either way, and
		// the argument that made that safe was "it forgets a delivery, it
		// does not assert an all-clear, and nothing downstream reads this
		// file for one". Half of that argument is now false twice over: the
		// next tick's delta reads this field, and the feed line this arm
		// would append — `cleared all` — is an all-clear in as many words,
		// said to the coordinator. So the reset is gated on the read having
		// been COMPLETE, which is D2's "a partial read is never a clear"
		// applied to the one case where every key vanished at once.
		//
		// A partial empty tick therefore holds everything and says nothing:
		// no feed line, and the record untouched, so the next complete tick
		// decides. Held, not forgotten — forgetting would cost the whole
		// standing set re-announced as `+` when the store came back, which
		// is the re-reading of seven unchanged keys D2 exists to end.
		//
		// A COMPLETE empty tick is the clear it always was: it appends one
		// `cleared all` line if there was anything to clear, and resets the
		// renag clock so the next non-empty set — even an identical episode
		// — is a fresh delivery rather than a renag.
		if !tick.Partial {
			// Gated on the hatch like every other line (`pulse_feed: off`
			// writes no feed at all); the RECORD is reset either way,
			// because dedup state is ADR 0027's and predates the channel.
			if cfg.Feed && len(state.DeliveredKeys) > 0 {
				d.appendPulseFeed(tick.At, "cleared all")
			}
			state.DeliveredKeys = nil
			state.PromptedAt = time.Time{}
		}
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

// govInputs is the pulse tick's view of the shop check: the two readings that
// live nowhere but this process's memory — the guard streak (G4) and the
// suspend ledger (ADR 0064 D2) — and its cost-scan seam, with the pulse
// persona marked live so the `no-live:` carry-over keeps meaning what it meant
// before the widening.
//
// Suspended is why this tick's wall-clock rows are awake-denominated and
// `posse status`'s are not: the witness that can prove the box was asleep runs
// here (suspend.go), and pulseOnce has already read the pair for this tick
// before building the set, so a wake that beat the witness's own ticker is
// still subtracted (D3).
//
// Errw is left nil on purpose. A config typo is worth one line where a human
// asked (`posse status`, dispatch's own passes) and is noise written every
// two minutes forever here.
//
// Lag is the loop's own launcher-lag seam, passed through rather than
// resolved: nil in every real loop, so G11 takes the instance's reading
// (govern.go's lag()), and a pin that drives the loop with a planted repo and
// a real stamp gets the same reading in the pulse's set as in the pass line.
func (d *Dispatcher) govInputs(cfg PulseConfig) GovInputs {
	return GovInputs{
		App:               d.App,
		HB:                d.HB,
		Bd:                d.Bd,
		Now:               d.now,
		Caller:            "pulse",
		PulsePersona:      cfg.Persona,
		Pulsing:           true,
		PulseFeed:         cfg.Feed,
		PulseWedge:        pulseWedgeBound(cfg),
		GuardTrippedSince: d.guardStreak(),
		Suspended:         d.suspendedSince,
		Spend:             d.Spend,
		Plan:              d.Plan,
		Lag:               d.Lag,
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
	// The delta is computed FIRST, from the record as it stands, because it
	// is both halves of the question: whether this tick has anything to say
	// (the old `changed` arm — an empty delta is a fingerprint that matched)
	// and, if it is due, what the feed line says (ADR 0093 D2).
	delta := pulseDiff(state.DeliveredKeys, tick.Conditions, tick.Partial)
	changed := !delta.Empty()
	// A record with a fingerprint but no readable timestamp is a HALF-read
	// record, and it reads as never prompted — not as a window that has not
	// elapsed. Guarding the renag arm with `!IsZero()` alone would answer
	// the second: the dedup arm says "already prompted" off the surviving
	// fingerprint while the renag arm has no clock to measure from, so the
	// set goes silent until it changes. That is the suppression ADR 0027
	// rules out by name, and the direction this whole function is written
	// to fail in is the other one — a repeat, which the renag interval
	// already budgets for (ranger-base-bv2nq).
	due := changed || state.PromptedAt.IsZero() || tick.At.Sub(state.PromptedAt) >= cfg.Renag
	if !due {
		return
	}

	// ── ADR 0093 D3 · which channel this delivery goes out on ────────────
	//
	// The switch is a non-blocking probe of the waiter's flock, and the
	// whole reason it is a lock and not a record is that release IS death
	// (pulsewait.go): a coordinator whose waiter, session or box has gone
	// cannot leave the switch on behind it.
	//
	// A LISTENING WAITER MEANS THE FEED IS THE DELIVERY and nothing is
	// typed, which is what ends the splice class by construction rather
	// than by care — the measured failure is three spliced coordinator
	// turns on 2026-11-13 alone, two writers on one byte stream with no
	// lock (ADR 0093 Context). It also means delivery cannot be refused: a
	// waiter is blocked on a FILE, so there is no idle-only gate to pass,
	// no startup wait and no undeliverable class. Everything below this arm
	// — the persona, the target, the readiness gate, the composer — exists
	// because the old channel was a keyboard.
	//
	// THE PROBE'S ERROR IS NOT AN ANSWER. PulseListener.Listening concedes
	// on it, and the line below says so: a probe nobody could take read as
	// "nobody is listening" is exactly what types into the pane of a
	// session that is.
	if cfg.Feed {
		listener := ReadPulseListener(d.App, tick.At, pulseWedgeBound(cfg))
		if listener.Err != nil {
			d.equietf("pulse: waiter lock %s %s — delivering on the feed anyway, because a probe that could not be taken is ignorance and never `nobody is listening` (ADR 0093 D3)\n",
				AbbrevHome(PulseWaitLockPath(d.App)), listener.Line())
		}
		if listener.Listening() {
			d.deliverPulseOnFeed(tick, state, delta, listener)
			return
		}
		// Free, or held by a waiter past its own maximum life. Either way
		// no one is listening, so the typed fallback runs and the text
		// gains the reminder that arms the feed (D4, pulsePromptText) — a
		// coordinator that forgot to re-arm is reminded through the old
		// channel rather than going dark, and `pulse-unarmed` says the same
		// thing on `posse status` (govern.go).
		d.quietf("pulse: %s — falling back to the typed pulse (ADR 0093 D3)\n", listener.Line())
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
	name, seat, status, found := pulseTarget(sessions, cfg.Persona)
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
	det, note, err := d.HB.AwaitPromptable(name, seat)
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

	// ── ADR 0093 D4 · the fallback refuses to type over the operator ─────
	//
	// The reading is ranger-base-qzujz's, off ONE capture of this seat's
	// screen: the composer, then the echo check against the line this
	// pane's own runtime says it last submitted, then the ghost check for a
	// line claude drew faint (capturedcomposer.go). What is left after
	// those two is text somebody TYPED and has not sent, and typing a shop
	// check after it submits one garbled message made of two.
	//
	// REX ONLY, and that is the whole of what distinguishes this from the
	// gate ranger-base-wr624 removed. herdr's composer reading is the
	// phantom-prone one — a region that still carried a line the operator
	// had already sent, ~586 consecutive skips and the pulse arm off for
	// ~10 hours of 2026-09-04 — and it cannot clear itself, because only a
	// new operator message replaces the phantom. It stays report-only
	// (above). This reading has the discriminator that one lacked: a store
	// of record for "was this submitted", asked FIRST, and every failure on
	// the way answers "not an echo" and leaves the text a hold.
	//
	// THE HOLD IS A SKIP AND NOT A FAILURE: the record is left unadvanced,
	// so the same delta is retried next tick and nothing is gated behind
	// renag for a delivery that never went out. That is the same shape as
	// every other skip above it.
	//
	// THE RESIDUAL IS HONEST (ADR 0093 D4): the operator can type in the
	// few ms between this capture and `rex send`, or between `send` and the
	// Enter. D1's waiter is what closes that window; this shrinks it on the
	// path D3 should be making rare, and the line below is what explains a
	// garbled turn after the fact.
	if hold, ok := d.HB.rexComposerHold(seat); ok {
		switch {
		case hold.Typed != "":
			d.quietf("pulse: held — %s box holds operator text (%q)\n", name, ellipsis(hold.Typed, 60))
			return
		case hold.Sent != "":
			// Reported, never acted on: this is the reading wr624's gate
			// could not take, and saying it is what makes the hold above
			// evidence of a reading rather than of a fixture.
			d.quietf("pulse: %s box echoes its own last submit (%q) — not a hold\n", name, ellipsis(hold.Sent, 60))
		case hold.Ghost != "":
			d.quietf("pulse: %s box holds claude's own suggestion (%q) — not a hold\n", name, ellipsis(hold.Ghost, 60))
		}
	}

	// The reminder rides on the typed text when the feed is armed at all
	// (D3): the cursor is the newest entry the feed holds NOW, so the
	// coordinator's first wake after arming carries this delivery's own
	// line — once — rather than starting after it.
	arm := ""
	if cfg.Feed {
		arm = PulseWaitCommand(pulseFeedCursor(PulseFeedPath(d.App)))
	}
	text := pulsePromptText(tick.Conditions, arm)
	if _, err := d.HB.Mux().Prompt(seat, text, false, 0); err != nil {
		d.equietf("pulse: prompt failed for %s: %v\n", name, err)
		return
	}
	d.quietf("pulse: %s → prompted %s\n", strings.Join(tick.Conditions, "; "), name)

	// Delivered, so the clock restarts here and nowhere else — same two
	// fields whether this was a new delta or a renag, because there is one
	// interval now and no ladder position to advance.
	//
	// Held keys go back into the record alongside what was seen, so the
	// delta the NEXT tick computes is against everything standing and not
	// against the subset this tick could read.
	state.DeliveredKeys = pulseDelivered(tick.Conditions, delta.Held)
	state.PromptedAt = tick.At

	// The feed line goes out HERE, after the prompt landed, and it still
	// does under D3 — which is the one place this file reads D3's prose
	// narrowly, so it is argued rather than assumed (ranger-base-r3qz6).
	//
	// D3 says that on a free lock "the watch appends the line anyway and
	// falls back to the typed pulse". Read as "append on any due tick,
	// whatever happens to the typed delivery", that is one of the two
	// orders ranger-base-hkqpq measured and rejected, and the rejection
	// does not weaken with a waiter in the world:
	//
	//   - append and ADVANCE on a due tick: the ticks where the typed pulse
	//     cannot land — 986 of 1323 in ADR 0093's own 70h window, a
	//     coordinator blocked or working — consume their delta into a feed
	//     that, on this arm, nobody is holding the lock on. Said to nobody,
	//     by a record that says it was said.
	//   - append on a due tick, advance only on delivery: every such tick
	//     re-appends the same delta. At a 2m interval the 500-line cap is
	//     full in about 17 hours of a sleeping coordinator and the oldest
	//     real entries are gone.
	//
	// So the invariant is the one hkqpq landed and D3 moves rather than
	// retires: THE APPEND AND THE RECORD ADVANCE ARE ONE ACT, with the
	// delivery. When a waiter is listening the feed IS the delivery and
	// both happen there (deliverPulseOnFeed); when it is not, the typed
	// pulse is the delivery and both happen here. A tick that said nothing
	// writes nothing, and the delta it could not say is still owed, because
	// the record it is computed against has not moved.
	//
	// What that costs, exactly: a coordinator that re-arms after a stretch
	// of ticks it could not be typed to gets the NET delta at the next due
	// tick, not one line per intermediate step. A key that appeared and
	// cleared inside that stretch is never named. The alternative is one of
	// the two measured failures above.
	//
	// Under the hatch there is no line at all: `pulse_feed: off` is ADR
	// 0027's typed path unchanged, and a path that still wrote the feed
	// would be a hatch that turned off only the half of the channel the
	// coordinator can see.
	if cfg.Feed {
		d.appendPulseFeed(tick.At, pulseFeedBody(delta, state.DeliveredKeys))
	}
}

// deliverPulseOnFeed is delivery when a waiter is listening (ADR 0093 D3):
// the feed line IS the delivery, nothing is typed, and the record advances
// with the line that carried it.
//
// IT CANNOT BE REFUSED and that is the point of the channel — a waiter is
// blocked on a file, not sitting at a keyboard, so there is no idle-only
// rule to pass, no readiness gate, no composer to read and no undeliverable
// case. The 1002 ticks ADR 0093 measured as skipped (986 blocked, 16
// working) are deliveries on this arm.
//
// A FAILED APPEND IS A FAILED DELIVERY HERE, which is the one asymmetry with
// the typed arm: the line is the whole of what was said, so when it does not
// land the record stays where it was and the next tick retries the same
// delta. Nothing is lost and nothing is announced that was not said.
//
// The listener rides in only to be named in the log. It decided the arm
// already, and the line is what lets ADR 0093's done-when rows 1 and 3 be
// read out of one log: a `→ feed` with the holder beside it, against a
// `→ prompted`.
func (d *Dispatcher) deliverPulseOnFeed(tick pulseTick, state *PulseState, delta pulseDelta, l PulseListener) {
	// Held keys go back into the record alongside what was seen, exactly as
	// on the typed arm, so the NEXT tick's delta is against everything
	// standing and not against the subset this tick could read.
	standing := pulseDelivered(tick.Conditions, delta.Held)
	seq, ok := d.appendPulseFeed(tick.At, pulseFeedBody(delta, standing))
	if !ok {
		return
	}
	state.DeliveredKeys = standing
	state.PromptedAt = tick.At
	d.quietf("pulse: %s → feed %d (%s)\n", strings.Join(tick.Conditions, "; "), seq, l.Line())
}

// pulseTarget finds cfg.Persona's live session among sessions — first match
// by agent — returning its session name, herdr status, and the SEAT herdr's
// real AgentPrompt/AgentExplain addresses (HerdrSession.Seat(), the workspace
// and root pane on record from session creation — not the session name every
// other caller in this codebase resolves through HerdrBackend.AgentTarget
// before speaking to herdr; ranger-base-5qe6). ("", Seat{}, "", false) is
// condition (c): no live session for persona. Deliberately not filtered by
// Crew: the whole point of the ADR 0008 §2 carve-out is that this prompt may
// reach the operator's own conversation.
func pulseTarget(sessions []HerdrSession, persona string) (name string, seat Seat, status string, found bool) {
	if persona == "" {
		// A session herdr reports with no agent would otherwise match, and
		// delivering a shop check into an arbitrary session is worse than
		// not delivering one.
		return "", Seat{}, "", false
	}
	for _, s := range sessions {
		if s.Agent == persona {
			return s.Name, s.Seat(), s.Status, true
		}
	}
	return "", Seat{}, "", false
}

// pulsePromptText is the fixed 'Pulse check' marker plus the observed
// conditions as hints (ADR 0027 §3): the prompt carries no authority, so it
// tells the session what to re-verify rather than what to do — dispatch's
// own stores (rhq list, git, bd) stay the record. One line, like every
// other prompt this codebase assembles (workPrompt, LandingPrompt).
//
// `arm` is ADR 0093 D3's reminder — the command that arms the feed, with the
// cursor in it — and "" is every delivery that does not need one: a pulse
// with `pulse_feed: off`, which is ADR 0027's path unchanged. A typed pulse
// only ever goes out when no waiter is listening, so when the feed is on,
// the reminder is on every one of them: the coordinator is being typed at
// BECAUSE it is not armed, and a reminder it only gets sometimes is one it
// cannot rely on.
//
// Appended rather than woven in, which keeps the marker and the conditions
// byte-identical to what the fingerprint, the watch log and every grep over
// this text already see (GovLines is the same bytes — see govern.go).
func pulsePromptText(conditions []string, arm string) string {
	text := fmt.Sprintf(
		"Pulse check: %s — re-verify against live state (rhq list, git, bd) before acting, then work your standing intents. No authority; the stores stay the record.",
		strings.Join(conditions, "; "))
	if arm != "" {
		text += " — arm the feed: " + arm
	}
	return text
}
