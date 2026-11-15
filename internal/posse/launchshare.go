package posse

// The launch share — ADR 0011 §5 ("A launch share: one label's class gets a
// reserved slice of `-n`, lent when it cannot use it", amended 2026-11-14,
// ranger-base-j11mf).
//
// An instance can run two kinds of work on one crew — the harness's own
// hardening and the operator's project work — and both draw on ONE
// launch-attempt ration, so whichever queue is deeper spends the epoch. That
// is not a hypothesis: MEASURED 2026-11-10 18:53 to 11-14 22:00 on the
// instance this was decided in (`dispatch-watch.log`, 30,965 lines, `-n 6`,
// both dollar caps set; docs/notes.d/ranger-base-j11mf.md), 66 of 101 epochs
// with a pass ran out of launch attempts — 434 exhaustion lines — and the
// dollar brake refused 0 launches. The attempt count is the contended
// resource, so the attempt count is what this splits. Dollars are not split;
// a dollar share is §5's exit hatch, to be built on the day a budget line
// first refuses.
//
// TWO FLAT KEYS, and they are flat because every other dispatch brake is:
//
//	launch_share_label: project   # a bead carrying this label is the share's class
//	launch_share: 2               # attempts per epoch reserved for it, out of -n
//
// THE LABEL IS A BUDGET CLASS, NEVER A §4 LANE. Routing does not read it: a
// share bead is routed by its other labels or by its assignee exactly as
// before. A PID that CARRIED the label would make it a lane under §4 and seat
// that PID on every share bead, so that configuration dormants the share and
// says which PID did it — the share must never re-route work.
//
// WHAT IS NOT HERE, and the honesty is the point. The per-class counts live
// on the Dispatcher beside `epochAttempts`, roll with the epoch and are
// restored in full by a supervised restart — the same per-process brake, for
// the reason epoch.go gives at length: making an attempt count durable adds
// the one-more-small-store ADR 0011 warns about, for a bound spend authority
// already covers.

import (
	"fmt"
	"strconv"
	"strings"
)

// LaunchShare is the resolved §5 split of one `-n`: which label is the
// share's class, how many of the epoch's attempts are reserved for it, how
// many are left for everything else, and — when the split is not armed — the
// one sentence saying which key made it dormant.
//
// Configured is not the same question as Armed, and the difference is what
// keeps this quiet on the instances that never asked for it. Configured means
// one of the two keys is PRESENT: the operator reached for the share, so
// every surface that prints the ration says what became of it. With neither
// key present there is no share to report and `LaunchCapLine` is
// byte-identical to what it printed before this file existed — a fact worth
// having, because that line is the top of every `--watch` log.
//
// Dormant then carries rangerhq-v83's rule: a typo is NAMED, never silently
// obeyed. A share that cannot be armed changes nothing about how the pass
// fires, and the operator reads why on the line they were already reading.
type LaunchShare struct {
	Label      string // launch_share_label, as configured
	Share      int    // attempts reserved for the label's class
	Remainder  int    // -n minus Share: the attempts left for every other bead
	Dormant    string // why the split is not armed; empty when it is
	Configured bool   // at least one of the two keys is present
}

// Armed reports that the split is in force: a class may be under or over its
// own allotment, and `fireLoop` owner-firsts and lends on that reading.
func (s LaunchShare) Armed() bool { return s.Configured && s.Dormant == "" }

// In reports whether a bead is in the share's class. Every other bead is in
// the remainder, which is what `-n` minus `launch_share` buys.
func (s LaunchShare) In(labels []string) bool {
	return s.Armed() && hasLabel(labels, s.Label)
}

// Line is the clause `LaunchCapLine` carries for the share: both allotments
// when it is armed, the dormant key when it is not, and nothing at all when
// the instance never configured one.
func (s LaunchShare) Line() string {
	switch {
	case !s.Configured:
		return ""
	case s.Dormant != "":
		return fmt.Sprintf(" · launch share DORMANT: %s — nothing is split (ADR 0011 §5)", s.Dormant)
	}
	return fmt.Sprintf(" · launch share: %d of them reserved for the %s class and %d for the remainder, per EPOCH — an idle class's allotment is lent to the other and never repaid (ADR 0011 §5; launch_share_label:/launch_share:)",
		s.Share, s.Label, s.Remainder)
}

// LaunchShareFor resolves the two keys against the `-n` they split.
//
// max is the operator's cap and not the room left in the epoch: the
// allotments are a property of the configuration, so they read the same at
// every pass of an epoch and the SPENDING against them is what moves.
//
// The dormant arms are in the order §5 states them, and each one is a
// configuration that would otherwise do something the operator did not ask
// for: no ration to split, no class to put a bead in, no count to reserve, a
// reservation that leaves the remainder zero (which starves every other bead
// by configuration, not by contention), and a share label that is already a
// lane.
func (a *App) LaunchShareFor(max int) LaunchShare {
	label := strings.TrimSpace(YamlGet(a.ConfigPath, "launch_share_label"))
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "launch_share"))
	if label == "" && raw == "" {
		return LaunchShare{}
	}
	s := LaunchShare{Label: label, Configured: true}
	switch {
	case max <= 0:
		s.Dormant = fmt.Sprintf("-n %d is no bounded ration to split (autostart_max_beads:)", max)
		return s
	case label == "":
		s.Dormant = "launch_share_label: is unset, so no bead is in the share's class"
		return s
	case raw == "":
		s.Dormant = fmt.Sprintf("launch_share: is unset, so nothing is reserved for %s", label)
		return s
	}
	n, err := strconv.Atoi(raw)
	switch {
	case err != nil || n <= 0:
		s.Dormant = fmt.Sprintf("launch_share: %q is not a count of launch attempts", raw)
		return s
	case n >= max:
		s.Dormant = fmt.Sprintf("launch_share: %d is not below -n %d — it would leave the remainder zero and starve every other class by configuration", n, max)
		return s
	}
	if who, labels := a.pidCarryingLabel(label); who != "" {
		s.Dormant = fmt.Sprintf("PID %s carries labels: [%s] — a share label on a PID is a §4 lane, and arming it would seat %s on every %s bead", who, labels, who, label)
		return s
	}
	s.Share, s.Remainder = n, max-n
	return s
}

// pidCarryingLabel names the first loaded PID carrying label, with its whole
// label list so the operator can see which key to edit.
//
// The coordinator is excluded for the reason labelMatches excludes her from
// the lane census: dispatch never hires that role (ADR 0033 §1), so a label
// she carries for triage anchors no lane and arming the share would not seat
// her on anything. A PID that does not load is not a lane either — §4 reads
// the roster through LoadAgent, so a file it cannot parse carries no labels
// anywhere in dispatch, and this reads the roster exactly as §4 does.
func (a *App) pidCarryingLabel(label string) (string, string) {
	coord := a.Coordinator()
	for _, name := range a.ListAgents() {
		if isCoordinator(coord, name) {
			continue
		}
		ag, err := a.LoadAgent(name)
		if err != nil {
			continue
		}
		if hasLabel(ag.Labels, label) {
			return name, strings.Join(ag.Labels, ", ")
		}
	}
	return "", ""
}

// shareSpend is a count of launch attempts split by §5 class — what a
// `fireLoop` call returns so its caller can charge the epoch per class.
//
// One count where there used to be a bare int, rather than two ints walked
// side by side: `total()` is the number `-n` bounds, and it cannot disagree
// with the two halves it is made of.
type shareSpend struct {
	share     int // charged to launch_share_label's class
	remainder int // charged to everything else
}

func (s shareSpend) total() int { return s.share + s.remainder }

// charge books n attempts (or hands one back, with n negative) to one class.
func (s *shareSpend) charge(inShare bool, n int) {
	if inShare {
		s.share += n
		return
	}
	s.remainder += n
}

// bookSpend charges a fire pass's or a refill's attempts to the epoch.
//
// THE ONE BOOKING SITE, and deliberately so: the total and the two class
// counts are three numbers that have to move together, and a second site
// that incremented `epochAttempts` alone would leave the classes reading an
// epoch that never happened — the share would be silently unbounded in
// exactly the pass that spent it. The observable is `epochClass.total() ==
// epochAttempts` at every instant, which the behavioural pins assert after a
// fire pass and after a refill (launchshare_test.go); a new caller that skips
// this method reds them. Both callers — the fire pass (dispatch.go) and the
// refill (passcarry.go) — book under their own `!d.DryRun`, because a dry
// pass launches nothing and must cost the epoch nothing.
func (d *Dispatcher) bookSpend(s shareSpend) {
	d.epochAttempts += s.total()
	d.epochClass.share += s.share
	d.epochClass.remainder += s.remainder
}

// classRoom is how many attempts a class may still take from its OWN
// allotment: its allotment, minus what the epoch has already booked to it,
// minus what this reading has spent on it.
//
// Never negative, because a class CAN be over its allotment — that is lending
// having happened — and "over" is not a quantity anything downstream needs.
// The number that bounds a lent launch is the epoch's total room, which
// `fireLoop` already breaks on.
func (d *Dispatcher) classRoom(sh LaunchShare, spent shareSpend, inShare bool) int {
	allot, used := sh.Remainder, d.epochClass.remainder+spent.remainder
	if inShare {
		allot, used = sh.Share, d.epochClass.share+spent.share
	}
	if room := allot - used; room > 0 {
		return room
	}
	return 0
}

// shareMark is what a launch line says about the class it was charged to.
//
// Empty when the share is dormant, so every pass report on every instance
// that never configured one is byte-identical to what it was. When the share
// is armed each launch names its class, and a launch made in the lending
// reading says so — otherwise an operator reading five launches of a class
// whose allotment is two has no way to tell a working lend from a broken
// bound.
func shareMark(sh LaunchShare, inShare, lending bool) string {
	if !sh.Armed() {
		return ""
	}
	class := "remainder"
	if inShare {
		class = sh.Label
	}
	if lending {
		return " [class: " + class + ", lent]"
	}
	return " [class: " + class + "]"
}

// shareFilingLine is the Context line a work prompt carries for a bead in
// the share's class — the share's one appearance anywhere but the
// operator's own surfaces.
//
// LABEL DRIFT IS THE SHARE'S FAILURE MODE, and the drift is done by hand.
// §5's class is the bead's LABELS and nothing else, so a handoff filed off a
// share bead without the label is in the remainder: it queues against every
// other bead in the shop instead of against the slice reserved for this
// work, and the share shrinks, epoch by epoch, to the beads the operator
// labelled personally. Nothing downstream can repair that by inference. A
// handoff files `-l <their label>` — the receiving LANE's label (ADR 0006 §1
// as amended 2026-09-01) — and a budget class is not a lane (§5), so the
// only reader who can put the share label on the next bead is the seat
// holding this one. This is how that seat is told.
//
// Rendered only for a bead IN the class, and only while the share is ARMED:
// a remainder bead has no label to carry on, and a dormant share would make
// this an instruction to label work against a split that is not in force
// (the whole of rangerhq-v83's rule is that a share which changes nothing
// says so rather than acting). `LaunchShare.In` asks both halves at once,
// which is why nothing here re-reads the config.
func shareFilingLine(label string) string {
	return fmt.Sprintf("launch share: this bead is in the `%s` class (ADR 0011 §5) — every bead you file for this work carries that label too (`-l <their label>,%s`), or it lands in the remainder and queues against every other bead instead of the launches reserved for this work.", label, label)
}

// exhaustionSplit is the clause the epoch's exhaustion line carries when the
// share is armed: which class spent the ration.
//
// Each half reads "<spent> of <allotted>", so a class over its allotment is
// visible as such — `project 5 of 2` is a lend that happened, and `remainder
// 4 of 4` beside it is the class that ran out. Without this the line says
// only that six of six attempts are gone, which is the one fact an operator
// who configured a share already knew.
func (d *Dispatcher) exhaustionSplit(sh LaunchShare) string {
	if !sh.Armed() {
		return ""
	}
	return fmt.Sprintf(" (launch share: %s %d of %d, remainder %d of %d)",
		sh.Label, d.epochClass.share, sh.Share, d.epochClass.remainder, sh.Remainder)
}
