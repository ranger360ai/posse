package posse

// Account-degraded (ADR 0013 §5) — a runtime whose DOLLARS posse cannot see.
//
// ADR 0003 §4 already refuses to call an unreadable channel $0: `posse cost`
// reports such a runtime as UNCOUNTED, never as zero. That is honesty about
// the display and it puts no brake on the channel. A live spend channel with
// a human eyeballing it is the hole this file closes.
//
// **Which runtimes.** The test is `Runtime.CostPriced()` — do this runtime's
// dollars reach `posse cost` — and NOT "is there an adapter", because those
// came apart (ranger-base-0lg6). Two ways to fail it, both degrades, both
// braked here, and they are DIFFERENT FACTS that this file must not print
// the same sentence about (accountDegrade):
//
//	UNCOUNTED  nothing reads the runtime; its sessions are absent from every
//	           total. The state this file was written for.
//	UNPRICED   an adapter reads it — turns, tokens, per-bead attribution —
//	           and prices none of what it reads. codex: a plan seat reports
//	           no cost and no list rate applies to one. Saying "no cost
//	           adapter reads codex" here would be false, and it was.
//
// The brake covers both because what the brake stands in for is a missing
// DOLLAR meter, and that is equally missing either way. Registering an
// adapter that PRICES is how a runtime leaves this file; registering one
// that only reads is not, and neither is setting a cap.
//
// Either way the runtime stays dispatchable — refusing it would be refusing
// the fleet's second half over a missing reading — but never quiet. Two
// obligations:
//
//	every pass names how many beads it sent there (uncountedReport)
//	`uncounted_cap_<runtime>:` is the brake (uncountedSkip)
//
// The cap counts BEADS POSSE ITSELF LAUNCHED over a rolling seven days, in
// the ledger below. It is not a bill and posse does not invent one: no price
// table lives here, and the operator holds the spend numbers. Unset is
// **unlimited and loud** — the `budget_*` dormancy pattern, where an unset
// dial changes no behaviour at all and the degrade is still named every
// pass. Filling the ADR 0012 D4 cost-adapter seam is how a runtime leaves
// this column; setting a cap is not. When a runtime does leave — grok did,
// ranger-base-0lg6 — a cap key left behind it stops braking, and
// countedCapDead names that once a pass rather than letting it go quiet.

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// accountDegrade is the reason clause every line in this file shares: WHY
// this runtime's dollars are not in `posse cost`. It is a sentence, not a
// word, because the two degrades are two different facts and the operator
// acts on them differently — one needs an adapter written, the other needs
// nothing written at all, because there is no dollar to read.
func accountDegrade(rt *Runtime) string {
	if reading := rt.CostReading(); reading != "" {
		return "the " + reading + " adapter counts " + rt.Name +
			"'s turns and tokens but prices none of them, so no dollars of this spend are in `posse cost`"
	}
	return "no cost adapter reads " + rt.Name + ", so none of this spend is in `posse cost`"
}

// UncountedWindow is the rolling window the cap counts over: the shared
// ledger window (ledger.go), seven days of beads rather than dollars,
// because the pool this cap stands over has no meter posse can read.
const UncountedWindow = LedgerWindow

// UncountedLogPath is the ledger: `$StateDir/uncounted.log`, append-only,
// one line per launch on an uncounted runtime.
func (a *App) UncountedLogPath() string { return filepath.Join(a.StateDir, "uncounted.log") }

// AppendUncounted records one launch on an uncounted runtime.
func (a *App) AppendUncounted(e LedgerEntry) error {
	return a.appendLedger(a.UncountedLogPath(), e)
}

// UncountedAppendable is the other half of the cap's reading: whether the
// line a launch would owe can be written at all (ranger-base-2y96) — a count
// off a file nothing can be added to stays at whatever it already says, so a
// cap of 1 over an empty unwritable ledger admits one launch per pass
// forever and records none of them.
func (a *App) UncountedAppendable() error {
	return a.ledgerAppendable(a.UncountedLogPath())
}

// UncountedCount is how many beads went to this runtime inside the window
// ending at now — the number `uncounted_cap_<runtime>:` is compared against.
func (a *App) UncountedCount(runtime string, now time.Time) (int, error) {
	return countLedger(a.UncountedLogPath(), runtime, now, UncountedWindow)
}

// UncountedCap reads config `uncounted_cap_<runtime>:` — beads per rolling 7
// days. It returns the parsed cap and the raw string, so a caller that
// displays the key can tell "unset" from "set to something that is not a
// cap". Unset is 0, which means unlimited.
//
// A value that is not a positive whole number of beads is named on errw and
// treated as unset, the rule `budget_pass:` already keeps: a typo must be
// visible, because a cap that silently stopped
// capping is indistinguishable from one nobody set, and this is the only
// brake on a channel with no meter behind it.
func (a *App) UncountedCap(runtime string, errw io.Writer) (int, string) {
	raw := strings.TrimSpace(a.CfgGet("uncounted_cap_"+runtime, ""))
	if raw == "" {
		return 0, ""
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		fmt.Fprintf(errw, "account: config uncounted_cap_%s: %q is not a positive bead count — no cap on %s, which stays unlimited and loud (ADR 0013 §5)\n", runtime, raw, runtime)
		return 0, raw
	}
	return n, raw
}

// uncountedPool is one uncounted runtime's account state for one pass: the
// cap, the rolling-window count the cap is compared against (this pass's own
// launches included, so the brake bites within a pass and not only between
// them), and how many beads this pass sent there.
type uncountedPool struct {
	Cap  int    // 0 = `uncounted_cap_<runtime>:` unset or unusable → unlimited
	Raw  string // what the key said, so the report can tell unset from unusable
	Used int    // beads in the rolling window, including this pass's
	Sent int    // beads this pass sent there
	// Why is accountDegrade's sentence for this runtime, resolved once with
	// the loaded Runtime so the report does not have to load it again.
	Why string
	// Unreadable is the ledger read failure, when there was one: Used is
	// then not a count and no cap can be judged against it.
	Unreadable error
	// Unappendable is the ledger WRITE failure, when there was one: Used is
	// a count of a file this pass's launches cannot be added to, so it is
	// the same number next pass and the pass after (ranger-base-ws09).
	// PROBED only when a cap is set (uncountedFor); a failed append sets it
	// either way, and with no cap there is nothing for it to brake.
	Unappendable error
	// Unlogged is how many of THIS pass's launches were made and then not
	// recorded, because the append failed for a reason the probe above
	// could not see (a full disk is the obvious one). The ledger is short
	// by that many from now on, so the report names it rather than letting
	// a warning on stderr be the only trace.
	Unlogged int
}

// uncountedFor is the pass's memoized account state for a runtime it is
// about to launch on, or nil when the runtime is counted and none of this
// applies. Read once per runtime per pass: the cap's typo line and the
// ledger scan both belong to the pass, not to each bead.
//
// A runtime that will not load is nil too. It cannot launch either, and the
// launch is where that gets reported; guessing at the account stage of a
// runtime posse cannot read would put a brake on a name and not on a pool.
//
// The priced case is where a still-set `uncounted_cap_<runtime>:` goes dead,
// so it is where countedCapDead says so — once, because this is memoized.
func (d *Dispatcher) uncountedFor(name string) *uncountedPool {
	if p, ok := d.uncounted[name]; ok {
		return p
	}
	if d.uncounted == nil {
		d.uncounted = map[string]*uncountedPool{}
	}
	rt, err := d.App.LoadRuntime(name)
	if err != nil {
		d.uncounted[name] = nil
		return nil
	}
	if rt.CostPriced() {
		d.countedCapDead(rt)
		d.uncounted[name] = nil
		return nil
	}
	n, raw := d.App.UncountedCap(name, d.errWriter())
	p := &uncountedPool{Cap: n, Raw: raw, Why: accountDegrade(rt)}
	if used, err := d.App.UncountedCount(name, d.now()); err != nil {
		p.Unreadable = err
	} else {
		p.Used = used
	}
	// The count first, then the appendability, so a ledger that is both
	// corrupt and unwritable is named by the fault an operator has to fix
	// first.
	//
	// Only with a cap set: the brake is the only thing that acts on this,
	// an unlimited runtime is loud by the report and not by the ledger, and
	// the probe touches the StateDir (it creates and removes a file when
	// there is no ledger yet) which a pass that has nothing to gate on it
	// has no business doing.
	if p.Cap > 0 {
		p.Unappendable = d.App.UncountedAppendable()
	}
	d.uncounted[name] = p
	return p
}

// countedCapDead: a set `uncounted_cap_<runtime>:` on a runtime whose
// dollars DO reach `posse cost` is named once per pass as not applying,
// pointing at the brake that does. It does not keep the cap alive — ADR
// 0013 §5's law stands, a runtime leaves this column by having its dollars
// priced and nothing else — it makes the key's death audible.
//
// The rule was ratified as ADR 0010 §3's third amendment (ranger-base-2eeb),
// and the 2026-09-05 simplification folded that section away with the
// mechanism it belonged to. The key it is about is ADR 0013 §5's, so §5 is
// where the rule now lives in full and where this citation points
// (ranger-base-ubqcw).
//
// The failure this closes is the one the header is written against, arriving
// by the other door: not a cap that stopped capping because its value was a
// typo, but a cap that stopped capping because the RUNTIME moved out from
// under it. grok did exactly that (ranger-base-0lg6) and nothing said a word.
//
// On errw, not d.Out: this is a warning about the operator's config, not an
// outcome of the pass, and it is the same writer UncountedCap's typo line
// already uses for the same kind of fact. Unset is silent — the key nobody
// set is not news.
func (d *Dispatcher) countedCapDead(rt *Runtime) {
	raw := strings.TrimSpace(d.App.CfgGet("uncounted_cap_"+rt.Name, ""))
	if raw == "" {
		return
	}
	d.eprintf("account: config uncounted_cap_%s: %q does not apply — the %s adapter prices %s's spend, so its dollars are in `posse cost` and the account brake is off; %s (ADR 0013 §5)\n",
		rt.Name, raw, rt.CostReading(), rt.Name, d.countedBrake(rt))
}

// countedBrake names what still holds a priced runtime back, so the line
// above sends the operator somewhere rather than merely taking a dial away.
//
// Every priced runtime has Dial E: its dollars are in the totals
// `budget_pass:`/`budget_day:` are compared against, which is the whole
// reason the bead-count stand-in is gone. grok has a second one where it is
// armed — `grok_guard_week:` meters the pool the dollars come out of, and an
// operator whose dead key was a pool brake wants that one, not the wallet.
//
// Read with io.Discard: grokPoolGuard names a malformed grok_guard_week: on
// its own errw once a pass, and this peek must not say it a second time.
func (d *Dispatcher) countedBrake(rt *Runtime) string {
	if rt.Name == GrokPoolRuntime {
		if pct := d.App.GrokGuardWeek(io.Discard); pct > 0 {
			return fmt.Sprintf("the brakes on %s are grok_guard_week: %.0f%% over the pool and budget_pass:/budget_day: over the dollars", rt.Name, pct)
		}
		return fmt.Sprintf("the brake on %s is budget_pass:/budget_day: over those dollars, and grok_guard_week: if you want the pool metered too", rt.Name)
	}
	return fmt.Sprintf("the brake on %s is budget_pass:/budget_day: over those dollars", rt.Name)
}

// uncountedSkip is the brake: the line this bead gets instead of a launch,
// plus the refill tally's grouping key for that line — "" for both to
// launch. Called with the runtime the launch is actually going to, which
// since ADR 0002's precedence is not always the persona's own.
//
// The kind is named after the runtime and the cap it hit, not the shared
// "runtime cap" grokPoolSkip counts under: inside a refill only the kind
// survives (refillreport.go's skipf discards the formatted line), and a
// bare "N runtime cap" cannot tell an account brake from a pool guard, let
// alone say which runtime or how close to its cap. ranger-base-0yiy.
//
// No cap set is no brake — unlimited, by design — and the report below is
// what makes that state loud rather than silent.
func (d *Dispatcher) uncountedSkip(name string) (line, kind string) {
	p := d.uncountedFor(name)
	if p == nil || p.Cap == 0 {
		return "", ""
	}
	// The rule Dial E already keeps: an unreadable ledger is not a
	// licence to spend. An armed cap over a
	// ledger nobody can count is the unarmed case wearing the armed case's
	// clothes, and this pool has no second meter to fall back to.
	if p.Unreadable != nil {
		return fmt.Sprintf("account-degraded: no dollars are counted for %s and %s is unreadable (%v) — a cap that counts nothing is not a brake; skipped",
				name, AbbrevHome(d.App.UncountedLogPath()), p.Unreadable),
			fmt.Sprintf("uncounted_cap_%s: ledger unreadable", name)
	}
	// The other half of that same rule (ranger-base-ws09). Reading the
	// ledger is only half of what the cap needs; the rest is the line the
	// launch owes once it succeeds. A ledger that can be read but not
	// appended to counts every pass at whatever it already says — zero, for
	// an empty one — so the cap is spent again every pass and records none
	// of it. Unbounded, and it never heals, which is why this refuses
	// before the count is compared rather than warning after the launch.
	if p.Unappendable != nil {
		return fmt.Sprintf("account-degraded: no dollars are counted for %s and %s cannot be appended to (%v) — a cap whose launches cannot be recorded is a cap of zero that reads as room; skipped",
				name, AbbrevHome(d.App.UncountedLogPath()), p.Unappendable),
			fmt.Sprintf("uncounted_cap_%s: ledger unwritable", name)
	}
	if p.Used >= p.Cap {
		return fmt.Sprintf("account-degraded: uncounted_cap_%s %d/%d in 7d — skipped", name, p.Used, p.Cap),
			fmt.Sprintf("uncounted_cap_%s %d/%d in 7d", name, p.Used, p.Cap)
	}
	return "", ""
}

// noteUncounted books one launch against the pool. Called AFTER the launch
// succeeded, never after the decision: a bead that never reached its agent
// spent nothing, and the cap counts what was actually sent.
//
// A --dry-run pass books the count in memory and writes no ledger line. That
// is what makes a dry run over a reached cap show the same skips a real pass
// would, while acting on nothing.
func (d *Dispatcher) noteUncounted(is RepoIssue, persona, runtime string) {
	p := d.uncountedFor(runtime)
	if p == nil {
		return
	}
	p.Used++
	p.Sent++
	if d.DryRun {
		return
	}
	if err := d.App.AppendUncounted(LedgerEntry{At: d.now(), Runtime: runtime, Bead: is.ID, Persona: persona}); err != nil {
		// ranger-base-ws09. p.Used already carries
		// this launch for the rest of the pass, so the cap still bites here;
		// what was missing is that the file is short by one for good and
		// nothing said so past this one line on stderr. Two things now do:
		// the pass's account line names the shortfall, and the write failure
		// arms the same brake the probe arms, so the rest of the pass parks
		// on this runtime instead of spending a cap against a ledger that
		// has just proved it cannot record the spending.
		p.Unlogged++
		if p.Unappendable == nil {
			p.Unappendable = err
		}
		d.eprintf("account: uncounted ledger not written for %s (%v) — the 7d count will be short by one\n", is.ID, err)
	}
}

// uncountedReport is §5's first obligation: every pass names how many
// launches it sent to a runtime nothing meters. On d.Out and not stderr —
// this is an outcome the pass reached, not a warning about one — and every
// pass, capped or not, because the whole point of the degrade is that no
// number anywhere else will ever mention this spend.
//
// A runtime this pass sent nothing to prints nothing: there is no spend to
// name, and a fleet with codex and grok declared but unused must not grow
// two standing lines that mean "fine".
func (d *Dispatcher) uncountedReport() {
	names := make([]string, 0, len(d.uncounted))
	for name, p := range d.uncounted {
		if p != nil && p.Sent > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names) // one pass, one order, whatever the map felt like
	for _, name := range names {
		p := d.uncounted[name]
		verb := "sent"
		if d.DryRun {
			verb = "would send"
		}
		window := fmt.Sprintf("%d in the last 7d", p.Used)
		switch {
		case p.Unreadable != nil:
			window = fmt.Sprintf("7d count unreadable (%v)", p.Unreadable)
		case p.Cap > 0:
			window = fmt.Sprintf("%d/%d in 7d (uncounted_cap_%s:)", p.Used, p.Cap, name)
		}
		if p.Unlogged > 0 {
			// The count above is this pass's memory; the file is not. Say so
			// here, because the next pass reads the file and its number will
			// be lower by exactly this much, for good.
			window += fmt.Sprintf(", %d of them never reached %s so the 7d count is short by that many from now on",
				p.Unlogged, AbbrevHome(d.App.UncountedLogPath()))
		}
		brake := fmt.Sprintf("uncounted_cap_%s: is unset — unlimited and loud", name)
		switch {
		case p.Cap > 0:
			brake = "the cap is the brake"
		case p.Raw != "":
			// Set to something that is not a bead count. Saying "unset"
			// here would send the operator to add a key that is already
			// there; stderr named the typo, and this names the state it
			// left the runtime in.
			brake = fmt.Sprintf("uncounted_cap_%s: %q is not a cap — unlimited and loud", name, p.Raw)
		}
		d.printf("! account-degraded %s: %s %d bead(s) this pass, %s — %s; %s (ADR 0013 §5)\n",
			name, verb, p.Sent, window, p.Why, brake)
	}
}
