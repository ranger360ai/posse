package rhq

// The governance surface: where "needs a human" is raised.
//
// Design: ADR 0029 §1-2 (docs/adr/0029-governance-surface.md, restating the
// archive's governance-surface ADR; bead rangerhq-81y0, from archive bead
// rangerhq-e37c; amended for G9 by the archive's coordinator-is-not-a-lane
// ADR, cited by title — that one has no restatement here yet). The design's
// two load-bearing lines are quoted where they decide something below.
//
// **Facts get computed, decisions get beads.** A governance condition is a
// checkable fact — computable by any process, twice, with the same answer —
// never a judgement. Conditions are level-triggered and heal (an approval
// gets clicked, a window cools), so they are computed live from the store
// that owns each one and never written down. Decisions — a question, a risk
// acceptance, a REFUSE needing an operator call — are durable work items and
// are already bead-shaped (`-l question`, `-l risk`); that stays the only way
// a governance item enters the queue. There is deliberately no attention
// file and no attention bead: a snapshot the watch loop writes each pass is
// exactly as honest as that loop is alive, and the loop's own death (G7) is
// one of the conditions.
//
// **One function, three renderings.** ShopCheck computes the set; `posse
// status` (GovReport, and the cockpit's GOVERNANCE block), the pulse tick's
// prompt, and the watch log's lines are three views of that one computation.
// state/pulse.yaml stays delivery dedup state — never a record anyone reads
// for truth.
//
// The view does not depend on the watch loop. `posse status` reads the
// stores directly and reports a dead loop itself (G7, off the kernel's
// flock, where release IS death). What dies with the loop is DELIVERY only.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The two classes the set splits into — the honest split, and the only
// judgement this file makes: URGENT means the shop is stopped, LANE means
// one bead or session is stopped and the rest flows.
const (
	GovUrgent = "URGENT"
	GovLane   = "LANE"
)

// Default ages for the two conditions that need one. Code defaults, config
// keys optional (`attn_question_age:`, `attn_guard_stuck:`); no new arm key
// — the surface is always on, because a surface you have to arm is one more
// thing that can be off when it is needed.
const (
	DefaultAttnQuestionAge = 4 * time.Hour
	DefaultAttnGuardStuck  = 2 * time.Hour
)

// GovCondition is one row of the condition set, computed live.
//
// Key is the whole identity as far as any machine reader is concerned: it is
// what the pulse fingerprints, so it must be stable while the condition
// persists and must change when a different instance of it appears. Detail
// is for a human and may move freely (percentages, ages) — nothing keys on
// it. ID is the design's row name; the two conditions carried over from the
// pulse's own first cut have none and say so.
type GovCondition struct {
	ID     string // G1..G9, or "" for a pulse-era carry-over
	Class  string // GovUrgent | GovLane
	Key    string // stable fingerprint token
	Detail string // one line, for a human
}

// Row is the id as a reader sees it: a carry-over has no row name and must
// not be given one — inventing G10 would make the design's "closed,
// enumerated set" not closed.
func (c GovCondition) Row() string {
	if c.ID == "" {
		return "—"
	}
	return c.ID
}

// GovSet is the whole computation's answer, sorted by Key so that joining it
// into a fingerprint needs no further ordering.
type GovSet []GovCondition

func (s GovSet) Keys() []string {
	out := make([]string, 0, len(s))
	for _, c := range s {
		out = append(out, c.Key)
	}
	return out
}

// Fingerprint is the pulse's dedup key: the sorted keys joined. Unchanged in
// shape from the pulse's first cut, so a state/pulse.yaml written by the
// older binary still compares.
func (s GovSet) Fingerprint() string { return strings.Join(s.Keys(), "|") }

// Urgent reports whether any condition means the shop is stopped.
func (s GovSet) Urgent() bool {
	for _, c := range s {
		if c.Class == GovUrgent {
			return true
		}
	}
	return false
}

// Has reports whether the set holds a given G-row. It is how a rendering
// asks about one condition BY NAME rather than by counting — the summary
// says how many need a human, and G7 says whether any of them is reaching
// one, which is not the same question and must not be answered by a number
// (bead rangerhq-mgvx).
//
// A carry-over has no row name (Row() renders "—"), so the empty id matches
// nothing: asking whether the set holds "" is a bug in the caller, never a
// hit.
func (s GovSet) Has(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range s {
		if c.ID == id {
			return true
		}
	}
	return false
}

// GovInputs is everything the check may read, and the seams the three
// renderings differ on. Everything else it reads live from the store that
// owns the fact.
type GovInputs struct {
	App *App
	HB  *HerdrBackend
	Bd  Bd
	Now func() time.Time

	// PulsePersona is who the pulse would deliver to, and Pulsing says the
	// pulse is armed. Together they gate the `no-live:` carry-over: "the
	// coordinator's session is gone" is a fact about DELIVERY, so a shop
	// with no pulse armed is not missing anything by it. In the pulse's own
	// tick both are true by construction, which is why nothing regresses.
	PulsePersona string
	Pulsing      bool

	// GuardTrippedSince is G4's streak clock — when the plan guard first
	// tripped in the current unbroken run of tripped passes. It lives in the
	// watch process's memory like blindSince, lost on restart, and a fresh
	// loop earning a fresh grace is correct rather than a bug. Zero means no
	// streak is known, which is every process that is not the watch loop:
	// such a process reports no G4 at all rather than inventing a streak
	// from one reading (a guard that trips once is a SKIP — mechanism,
	// self-healing — and only a sustained one is a governance condition).
	GuardTrippedSince time.Time

	// Spend is the cost scan behind G6; nil = ScanCosts. Injected so a test
	// never reads the operator's live ledger.
	Spend func(time.Time) *CostReport

	// Plan is the plan-window reader behind G5/G6; nil = the instance's
	// adapter. The read goes through the shared snapshot either way, so the
	// whole instance still makes at most one request per TTL.
	Plan PlanReader

	// Caller is the name a plan-endpoint request lands under in
	// $StateDir/plan-usage.log ("status", "pulse", "cockpit"); "" reads as
	// "govern". The log is only evidence if it says who asked — that is
	// what settled whether the 2026-08-22 429 storm was ours.
	Caller string

	// Errw takes the config-typo lines the readings below would otherwise
	// swallow; nil = io.Discard (the pulse tick, which has its own writer
	// and no business printing a config complaint every two minutes).
	Errw io.Writer
}

func (in GovInputs) now() time.Time {
	if in.Now != nil {
		return in.Now()
	}
	return time.Now()
}

func (in GovInputs) caller() string {
	if in.Caller != "" {
		return in.Caller
	}
	return "govern"
}

func (in GovInputs) errw() io.Writer {
	if in.Errw != nil {
		return in.Errw
	}
	return io.Discard
}

// AttnQuestionAge is how long a question/risk bead may sit open before G3.
// Same grammar as every other interval key; a typo is named and the default
// stands, because a threshold nobody can see is worse than a wrong one.
func (a *App) AttnQuestionAge(errw io.Writer) time.Duration {
	return a.attnAge("attn_question_age", DefaultAttnQuestionAge, errw)
}

// AttnGuardStuck is how long the plan guard may skip before G4.
func (a *App) AttnGuardStuck(errw io.Writer) time.Duration {
	return a.attnAge("attn_guard_stuck", DefaultAttnGuardStuck, errw)
}

func (a *App) attnAge(key string, def time.Duration, errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, key))
	if raw == "" {
		return def
	}
	// Zero is meaningful here (raise it immediately), so this parses the
	// grammar directly rather than through ParseInterval, which rejects it.
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
		return d
	}
	fmt.Fprintf(errw, "governance: config %s: %q is not a duration (4h, 30m, or seconds) — using %s\n",
		key, raw, BlindFor(def))
	return def
}

// PausePath is state/pause.yaml — §3's file, and the one legitimately NEW
// store the design adds: pause intent is a new fact with a single writer,
// not a copy of another store's. This file only READS it, and reads a file
// nobody has written yet as the absence of a pause; the write half and the
// pass gate are pause.go and dispatch.go (§3, bead rangerhq-a2g6).
func PausePath(a *App) string { return filepath.Join(a.StateDir, "pause.yaml") }

// Pause is what state/pause.yaml says. Present=false is the ordinary case.
type Pause struct {
	Present bool
	By      string
	At      string
	Why     string
}

// ReadPause reads the pause file. A file that exists but parses to nothing
// is still a pause: the operator (or the coordinator) put it there, and
// declining to see it because a field is missing would be the surface
// deciding a malformed stop is no stop.
func ReadPause(path string) Pause {
	if _, err := os.Stat(path); err != nil {
		return Pause{}
	}
	return Pause{
		Present: true,
		By:      YamlGet(path, "by"),
		At:      YamlGet(path, "at"),
		Why:     YamlGet(path, "why"),
	}
}

// ShopCheck computes the condition set live from the stores. One function;
// `posse status`, the cockpit block, the pulse tick and the watch log are
// its renderings.
//
// The rows, with the store of record for each:
//
//	G1 session blocked on an approval          herdr agent status  LANE
//	G2 settled-but-holding (the zom skip)       bd + herdr          LANE
//	G3 question/risk bead past attn_question_age  bd                LANE/URGENT
//	G4 plan guard skipping past attn_guard_stuck  the guard + clock  URGENT
//	G5 guard blind past plan_guard_blind_max      the plan endpoint  URGENT
//	G6 Dial E stop / budget >= 100%               cost scan vs caps  URGENT
//	G7 watch loop dead while autostart armed      the loop's flock   URGENT
//	G8 paused                                     state/pause.yaml   URGENT
//	G9 ready bead routed to the coordinator       bd + config        LANE
//
// plus the two conditions the pulse's own first cut shipped and this
// widening deliberately does not drop — unpushed commits on a beads repo,
// and no live session for the pulse persona. Neither is a G-row (the table
// is closed at nine), both are things the coordinating persona owes, and
// removing shipped oversight is not something a bead titled "widen" should
// do quietly.
//
// A store that cannot be READ is not a store that says no. bd scan failures
// come back as errors alongside whatever was computed: the set is then
// PARTIAL, and every caller must say so rather than render it as an
// all-clear. The one exception is the pulse tick, which runs off a timer
// with no human watching it spend and whose failure mode must be silence —
// it logs and moves on.
func ShopCheck(in GovInputs) (GovSet, []error) {
	var set GovSet
	var failed []error
	now := in.now()
	add := func(id, class, key, detail string) {
		set = append(set, GovCondition{ID: id, Class: class, Key: key, Detail: detail})
	}

	sessions, err := in.HB.Sessions()
	if err != nil {
		failed = append(failed, fmt.Errorf("herdr: %w", err))
	}

	// ── G1 · a live session whose agent herdr reports blocked ────────────
	livePersona := false
	for _, s := range sessions {
		if in.PulsePersona != "" && s.Agent == in.PulsePersona {
			livePersona = true
		}
		if s.Status == "blocked" {
			add("G1", GovLane, "blocked:"+s.Name, fmt.Sprintf("%s (%s) is blocked on an approval", s.Name, s.Agent))
		}
	}

	// ── the bd half: G2, G3, G9 ──────────────────────────────────────────
	if in.Bd.Available() {
		failed = append(failed, in.beadConditions(now, sessions, add)...)
	} else if len(in.App.BeadsDirs()) > 0 {
		// bd is the store of record for three rows. Missing means UNKNOWN,
		// and reporting unknown as clear is the silence this surface exists
		// to end.
		failed = append(failed, Die("bd not found in PATH — G2/G3/G9 unknown"))
	}

	// ── G5 · the guard is blind, and past its budget ─────────────────────
	// Taken before G4 because G6 reuses the reading, and before G6 for the
	// same reason. An unarmed guard reads nothing and reports nothing: the
	// blind window is a state of a guard that exists.
	plan, planErr := in.planReading(now)
	// A read that found no store at all is not blindness and never becomes
	// a G5 (ADR 0019 D3). Caught here because it can arrive from the READ —
	// the availability check above returns nil,nil for the state it catches
	// itself, but a store that went away after that check comes back as an
	// error, and "monitoring itself is broken" is the wrong diagnosis for a
	// machine that has simply never been logged in. The guard's own pass
	// says the true one, once (dispatch.go planUnconfigured).
	if planErr != nil && NoSourceReason(planErr) == nil {
		blindFor, past := in.blindPast(now)
		if past {
			add("G5", GovUrgent, "guard-blind",
				fmt.Sprintf("plan guard blind %s (%v) — monitoring itself is broken", BlindFor(blindFor), planErr))
		}
	}

	// ── G4 · the guard is skipping, sustained ────────────────────────────
	// The reading is re-taken at view time (above); the STREAK is the watch
	// process's, and a process without one reports no G4. A guard that
	// trips is a skip — automatic, self-healing, pure mechanism — and only
	// a sustained one is a governance condition.
	if !in.GuardTrippedSince.IsZero() && planErr == nil {
		if stuck := now.Sub(in.GuardTrippedSince); stuck >= in.App.AttnGuardStuck(in.errw()) {
			if w := overThresholdWindow(in.App, plan, in.errw()); w != "" {
				add("G4", GovUrgent, "guard-stuck",
					fmt.Sprintf("plan guard has skipped for %s (%s)", BlindFor(stuck), w))
			}
		}
	}

	// ── G6 · Dial E stop ─────────────────────────────────────────────────
	st := in.dialE(now, plan)
	if st.Stop() {
		add("G6", GovUrgent, "budget-stop:"+st.Window,
			fmt.Sprintf("budget stop — %s", st.Line()))
	} else if st.Unreadable != nil {
		// ADR 0018 §3: part of the ledger could not be read, so the spend
		// is a FLOOR and "under the cap" is a floor being under the cap.
		// A row that says nothing here would be reporting an uncountable
		// ledger as $0 spent — an unreadable ledger is not a licence to
		// spend, and it is not an all-clear either.
		failed = append(failed, fmt.Errorf("cost ledger partly unreadable (%v) — G6 is a floor, not a total", st.Unreadable))
	}

	// ── G7 · the watch loop is dead while autostart is armed ─────────────
	// The meta-condition: no other condition gets DELIVERED without it. The
	// probe is the kernel's flock, where release is death — no staleness
	// class, nothing to reap, no argv to match. "Could not ask" is never
	// read as "no loop": that inference is what kill-and-replaces a live
	// loop, and here it would raise a false alarm on the one row whose
	// whole job is to be believed.
	if yamlHasKey(in.App.ConfigPath, "autostart_interval") {
		running, err := WatchLoopRunning(in.App)
		switch {
		case err != nil:
			failed = append(failed, fmt.Errorf("watch-loop lock: %w — G7 unknown", err))
		case !running:
			add("G7", GovUrgent, "loop-dead",
				fmt.Sprintf("autostart is armed and no watch loop holds %s — nothing is being delivered",
					AbbrevHome(WatchLockPath(in.App))))
		}
	}

	// ── G8 · paused ──────────────────────────────────────────────────────
	// URGENT by intent — reported, never alarmed: the shop IS stopped, and
	// a human meant it. The line names the pauser and the why, which is the
	// whole reason the file shape makes both mandatory.
	if p := ReadPause(PausePath(in.App)); p.Present {
		add("G8", GovUrgent, "paused", PauseLine(p))
	}

	// ── carry-over · unpushed commits on a beads repo ────────────────────
	seen := map[string]bool{}
	for _, dir := range in.App.BeadsDirs() {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		n, err := git(dir, "rev-list", "--count", "@{u}..HEAD")
		if err != nil {
			// No upstream (or not a repo at all) is the absence of a
			// condition, not a failure to read one — a repo this check
			// cannot ask reads the same as a repo with nothing to report.
			continue
		}
		if count, err := strconv.Atoi(n); err == nil && count > 0 {
			add("", GovLane, fmt.Sprintf("unpushed:%s:%d", dir, count),
				fmt.Sprintf("%d unpushed commit(s) in %s", count, DirLabel(dir)))
		}
	}

	// ── carry-over · no live session for the pulse persona ───────────────
	if in.Pulsing && in.PulsePersona != "" && !livePersona {
		add("", GovLane, "no-live:"+in.PulsePersona,
			fmt.Sprintf("no live session for %s — the pulse has nowhere to deliver", in.PulsePersona))
	}

	// Key order, and stable: the fingerprint is the joined keys, so a set
	// that reordered under an equal key would re-prompt the coordinator for
	// nothing.
	sort.SliceStable(set, func(i, j int) bool { return set[i].Key < set[j].Key })
	return set, failed
}

// PauseClause is the "— by X, at Y, why: Z" tail every rendering of a pause
// shares: G8's detail here, and §3's own pause/resume/decline lines
// (pause.go). One vocabulary — a pause named two ways is a pause somebody
// has to correlate.
func PauseClause(p Pause) string {
	var parts []string
	if p.By != "" {
		parts = append(parts, "by "+p.By)
	}
	if p.At != "" {
		parts = append(parts, "at "+p.At)
	}
	if p.Why != "" {
		parts = append(parts, "why: "+p.Why)
	}
	if len(parts) == 0 {
		return " (the pause file names neither pauser nor reason)"
	}
	return " — " + strings.Join(parts, ", ")
}

// beadConditions is the bd half of the set: G2, G3 and G9. It is the one
// real cost this widening adds — the pulse tick deliberately never touched
// bd — and it is bounded: three list calls per configured repo, plus one
// `dep list` per aging question bead and one `comments` per settled holder.
// Both of those are per FINDING, and a shop with many findings has a bigger
// problem than a bd call.
func (in GovInputs) beadConditions(now time.Time, sessions []HerdrSession, add func(id, class, key, detail string)) []error {
	var failed []error
	coord := in.App.Coordinator()
	maxAge := in.App.AttnQuestionAge(in.errw())

	for _, dir := range in.App.BeadsDirs() {
		// ── G2 · settled-but-holding ─────────────────────────────────────
		// The zom skip made visible. dispatch already declines to re-prompt
		// a bead whose holder settled — "stopped on purpose? (--resume
		// re-prompts)" — and that skip is exactly the state nobody was told
		// about: the bead stays in_progress forever and the queue looks busy.
		held, err := in.Bd.InProgress(dir)
		if err != nil {
			failed = append(failed, ScanError{Dir: dir, Err: err})
		}
		for _, is := range held {
			s := holderSession(sessions, dir, is)
			if s == nil || !settledStatus(s.Status) {
				// No session at all is NOT this row: dispatch self-heals
				// that one on the claim-held path (relaunch). Only the
				// settled case needs a human.
				continue
			}
			sub, why := in.ladderSubtype(dir, is.ID)
			add("G2", GovLane, "settled"+sub+":"+is.ID,
				fmt.Sprintf("%s held by %s, %s settled %q%s", is.ID, is.Assignee, s.Name, s.Status, why))
		}

		// ── G3 · a question or risk bead nobody has answered ─────────────
		asks, err := in.Bd.OpenLabeledAny(dir, "question", "risk")
		if err != nil {
			failed = append(failed, ScanError{Dir: dir, Err: err})
		}
		var holding map[string]int
		if len(asks) > 0 {
			holding, err = in.blockedBy(dir)
			if err != nil {
				failed = append(failed, ScanError{Dir: dir, Err: err})
			}
		}
		for _, is := range asks {
			if is.Created.IsZero() || now.Sub(is.Created) < maxAge {
				continue
			}
			age := now.Sub(is.Created)
			class, blocks := GovLane, ""
			// URGENT if it dep-blocks work: an unanswered question that
			// only costs its own bead is a lane stopped; one that holds
			// other beads out of `bd ready` is the shop stopped behind a
			// sentence nobody wrote.
			if n := holding[is.ID]; n > 0 {
				class = GovUrgent
				blocks = fmt.Sprintf(", blocking %d bead(s)", n)
			}
			add("G3", class, "question:"+is.ID,
				fmt.Sprintf("%s open %s unanswered%s — %s", is.ID, BlindFor(age), blocks, is.Title))
		}

		// ── G9 · ready work routed to the coordinator ────────────────────
		// She is not a lane, so dispatch refuses to hire her and the bead
		// sits in `bd ready` forever looking dispatchable. The predicate is
		// keyed on config `coordinator:` alone, exactly as the refusal is.
		if coord == "" {
			continue
		}
		ready, err := in.Bd.Ready(dir, "")
		if err != nil {
			failed = append(failed, ScanError{Dir: dir, Err: err})
		}
		for _, is := range ready {
			if isCoordinator(coord, is.Assignee) {
				add("G9", GovLane, "coordinator:"+is.ID,
					fmt.Sprintf("%s is ready and assigned to %s, who is not a lane — reassign or triage by hand", is.ID, coord))
			}
		}
	}
	return failed
}

// settledStatus is herdr's word for an agent that has stopped: idle or done.
// "" is no agent detected — a crashed session, which dispatch relaunches, so
// it is not this row either.
func settledStatus(s string) bool { return s == "idle" || s == "done" }

// holderSession finds the live session holding a bead: the one dispatch's
// own run record says was created for it, else the persona's reusable slot
// in that repo. Both come out of the sessions listing already taken — this
// row costs no extra herdr call.
func holderSession(sessions []HerdrSession, dir string, is BdIssue) *HerdrSession {
	for i := range sessions {
		s := &sessions[i]
		if !s.Foreign && s.Bead == is.ID && s.Checkout() == dir {
			return s
		}
	}
	if is.Assignee == "" {
		return nil
	}
	slot := SessionFor(is.Assignee, dir)
	for i := range sessions {
		if s := &sessions[i]; !s.Foreign && s.Name == slot {
			return s
		}
	}
	return nil
}

// ladderSubtype reads the bead's last comment for the escalation ladder's
// protocol prefixes. `BLOCKED:` and `REFUSED:` are strings the harness
// itself injects into every work prompt (dispatch.go's ladder), so reading
// them back is mechanism and not text divination: a persona that stopped
// this way was told to write exactly that.
//
// It returns a key suffix and a human clause. An unreadable comments call
// costs the subtype, never the row — the bead is settled-but-holding either
// way, and that is the fact.
func (in GovInputs) ladderSubtype(dir, id string) (suffix, clause string) {
	cs, err := in.Bd.Comments(dir, id)
	if err != nil || len(cs) == 0 {
		return "", ""
	}
	text := strings.TrimSpace(cs[len(cs)-1].Text)
	switch {
	case strings.HasPrefix(text, "BLOCKED:"):
		return "-blocked", " — BLOCKED: " + ladderHead(strings.TrimSpace(strings.TrimPrefix(text, "BLOCKED:")))
	case strings.HasPrefix(text, "REFUSED:"):
		return "-refused", " — REFUSED: " + ladderHead(strings.TrimSpace(strings.TrimPrefix(text, "REFUSED:")))
	}
	return "", ""
}

// ladderHead is the first line of a ladder comment, capped: the clause is a
// pointer to the bead, never a transcript of it.
func ladderHead(s string) string {
	s = firstLine(s)
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

// blockedBy is blocker id → how many OPEN beads it is holding out of the
// queue. One `bd blocked` call for the whole repo, whatever the question
// count: asking each aging question bead about its own dependents was 24
// calls and ~4.5s of a 5.6s view on the store this was written against
// (rangerhq-81y0), which is not a price a two-minute ticker should pay.
//
// `open` and nothing else. A CLOSED bead is not waiting; an IN_PROGRESS one
// is being worked despite the question; a DEFERRED one was parked on
// purpose. What is left is a bead that would be in `bd ready` but for a
// blocker, which is exactly what "dep-blocks ready work" means.
//
// A question with several blockers counts here under each of them. That
// over-counts by the beads that would still be blocked without this
// question — and over-counting costs one URGENT a human dismisses, where
// under-counting costs the row its whole point.
func (in GovInputs) blockedBy(dir string) (map[string]int, error) {
	blocked, err := in.Bd.Blocked(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, b := range blocked {
		if b.Status != "open" {
			continue
		}
		for _, by := range b.BlockedBy {
			out[by]++
		}
	}
	return out, nil
}

// planReading takes the guard's own reading through the shared snapshot, at
// the same staleness the guard itself decides on. An unarmed guard makes no
// request and reports no reading — nil, nil — which is neither blind nor
// tripped, because there is no meter.
func (in GovInputs) planReading(now time.Time) (PlanUsage, error) {
	if len(in.App.PlanGuardThresholds(io.Discard)) == 0 {
		return nil, nil
	}
	c := in.App.PlanCache(in.caller())
	c.Now = func() time.Time { return now }
	if in.Plan != nil {
		c.Reader, c.NoAdapter = in.Plan, nil
	}
	if c.Reader == nil {
		// Guard OFF, not guard blind: a meter that does not exist cannot be
		// read, and failing closed against it would be a brake with no
		// release. The guard says this out loud on its own path.
		return nil, nil
	}
	u, _, err := c.Read(planGuardMaxAge(in.App.PlanUsageTTL(io.Discard), in.App.PlanGuardBlindMax(io.Discard)))
	return u, err
}

// blindPast is how long the instance has been without a reading, and whether
// that is past `plan_guard_blind_max:`.
//
// The clock is the shared snapshot's own timestamp — the moment the last
// successful reading was TAKEN — not a per-process variable. That is what
// lets `posse status`, run from a fresh shell, answer G5 at all: the blind
// window is a fact about the instance, and the instance writes it down every
// time a reading succeeds. `plan_guard_blind_max: 0` is the documented
// escape hatch (never fail on blindness) and disarms this row with it.
func (in GovInputs) blindPast(now time.Time) (time.Duration, bool) {
	budget := in.App.PlanGuardBlindMax(in.errw())
	if budget <= 0 {
		return 0, false
	}
	at, ok := in.App.PlanCache(in.caller()).LastReadAt()
	if !ok {
		// No snapshot has ever been written on this machine. That is a
		// guard that has never had a reading, not one that lost one, and
		// there is no clock to be past.
		return 0, false
	}
	blind := now.Sub(at)
	return blind, blind > budget
}

// overThresholdWindow is the guard's verdict on a reading that succeeded:
// the first window over its `plan_guard_<window>:` threshold, in the
// adapter's own order, or "" when none is. Same comparison planGuard makes,
// so G4 never claims a skip the guard would not have made.
func overThresholdWindow(a *App, u PlanUsage, errw io.Writer) string {
	th := a.PlanGuardThresholds(errw)
	for _, w := range u {
		if t := th[w.Name]; t > 0 && w.Pct > t {
			return fmt.Sprintf("%s at %.0f%% > %.0f%%", w.Name, w.Pct, t)
		}
	}
	return ""
}

// dialE is G6's reading: the DAY window and the plan windows.
//
// The `budget_pass:` window is deliberately absent, and since ADR 0028 §2
// the reason has changed under it. It used to be that the window did not
// exist outside a pass: it opened when `Run` did, so a view rendered from
// another process had no pass to measure, and the day's spend under the pass
// cap would have been a made-up number wearing a real cap's name. The epoch
// is on the wall clock and any process can compute it, so that objection is
// gone — what keeps the row out now is only that G6's content is ADR 0029's
// to decide, and a scan of a second window is a cost this view does not take
// on its own authority (ranger-base-f0y3 filed the question).
//
// Unset caps mean Dial E is dormant and NOTHING is scanned — the same
// dormancy dispatch keeps, and the reason this row costs a transcript scan
// only where the operator armed one.
func (in GovInputs) dialE(now time.Time, plan PlanUsage) BudgetState {
	var st BudgetState
	_, st.DayCap = in.App.BudgetCaps(in.errw())
	st.Plan = plan
	if !st.Set() {
		return st
	}
	scan := in.Spend
	if scan == nil {
		scan = func(t time.Time) *CostReport { return ScanCosts("", t) }
	}
	rep := scan(startOfDay(now))
	st.DaySpend = rep.DayTotal(now)
	st.Unreadable = rep.ReadErr
	st.resolve()
	return st
}

// StatusInputs is what a one-shot process — `posse status`, the cockpit —
// hands ShopCheck.
//
// GuardTrippedSince is deliberately absent: this process is not the watch
// loop and has no streak, so it reports no G4 rather than inventing one from
// a single reading. Everything else is read live, which is the point — the
// view does not depend on the loop, and a killed loop is a condition it
// reports (G7) rather than a reason it cannot answer.
//
// errw takes the config-typo lines: a human asked, so a threshold that does
// not parse gets said out loud exactly once, here.
func StatusInputs(a *App, hb *HerdrBackend, errw io.Writer) GovInputs {
	in := GovInputs{App: a, HB: hb, Bd: NewBd(), Caller: "status", Errw: errw}
	// The pulse persona matters to one carry-over row and only when the
	// pulse is armed to deliver to it. A config error disarms the reading
	// rather than failing the whole view: LoadPulseConfig's own error is the
	// watch loop's to report.
	if cfg, err := LoadPulseConfig(a); err == nil && cfg.Armed {
		in.PulsePersona, in.Pulsing = cfg.Persona, true
	}
	return in
}

// ─── the three renderings ────────────────────────────────────────────────────

// GovLines is the log/prompt rendering: one stable token per condition,
// joined. It is what the watch log records and what the pulse's prompt
// carries as hints — the same bytes, so a persona greps one string and finds
// both.
func GovLines(s GovSet) string { return strings.Join(s.Keys(), "; ") }

// GovReport is the human rendering — `posse status` and, one row per line,
// the cockpit's GOVERNANCE block. URGENT first, then LANE, each in key
// order, because the split is the only ranking the set has.
func GovReport(w io.Writer, s GovSet, failed []error) {
	for _, err := range failed {
		fmt.Fprintf(w, "  ?  %v\n", err)
	}
	if len(s) == 0 {
		if len(failed) == 0 {
			fmt.Fprintln(w, "nothing needs a human")
		} else {
			fmt.Fprintln(w, "no condition found — but the set above could not be read, so this is not an all-clear")
		}
		return
	}
	for _, c := range GovOrdered(s) {
		fmt.Fprintf(w, "%-7s %-3s %s\n", c.Class, c.Row(), c.Detail)
	}
}

// GovOrdered is the display order: URGENT before LANE, key order within
// each. ShopCheck's own order is key order alone, because a fingerprint must
// not change when a condition changes class.
func GovOrdered(s GovSet) GovSet {
	out := append(GovSet(nil), s...)
	sort.SliceStable(out, func(i, j int) bool {
		ai, bi := out[i].Class == GovUrgent, out[j].Class == GovUrgent
		if ai != bi {
			return ai
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// GovSummary is the one-line form the cockpit heading and a `posse status`
// header use: "2 URGENT · 3 LANE", or the all-clear.
func GovSummary(s GovSet) string {
	urgent, lane := 0, 0
	for _, c := range s {
		if c.Class == GovUrgent {
			urgent++
		} else {
			lane++
		}
	}
	if urgent == 0 && lane == 0 {
		return "clear"
	}
	if urgent == 0 {
		return fmt.Sprintf("%d LANE", lane)
	}
	if lane == 0 {
		return fmt.Sprintf("%d URGENT", urgent)
	}
	return fmt.Sprintf("%d URGENT · %d LANE", urgent, lane)
}
