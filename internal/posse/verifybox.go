package posse

// The live-box detective checks, read back (bead ranger-base-jj2ax, from the
// operator's 2026-09-06 ruling on ranger-base-0x1wc: "d plus b, yes to
// launchagent after g code lands").
//
// scripts/verify-box.sh runs the checks that assert what THIS MACHINE is —
// the three version pins, the credential paths, the L3 hooks, the operator's
// gate copy, the shared store's relate pairs. Until this file it printed to
// whoever typed it and nothing else: the run left no trace, so a schedule
// could be installed and stop, or run and come back red every night, and the
// only surface that would have said so was a terminal somebody closed. That
// is the same shape the script itself exists for one level down — a control
// nobody notices is unrun (ranger-base-51z8j).
//
// So the script writes ONE state file at the end of every run and this reads
// it. The file is the whole store: there is no second stamp, no index, and
// nothing else writes it.
//
// **The freshness rule is mandatory and it is the point** (the ruling's own
// words). A verdict is not a fact about the box, it is a fact about the box
// AT A MOMENT, and a verdict nobody has refreshed since the schedule's
// interval says nothing about now. So a reading older than
// `verify_box_max_age:` (26h, a daily schedule plus two hours of slack) is
// STALE and never green — "checked recently and clean" is the only green
// this surface knows how to render. The same rule catches the run that DIED
// before it could write a verdict: the old file stays, ages out, and the row
// appears. What killed it is in the log the plist points at
// (VerifyBoxLogPath), which is why the stale row names that path.
//
// **A stamp from the future is not a reading.** BlindFor renders every
// negative duration as "0s", so a state file stamped ahead of this box's
// clock would read as the freshest possible verdict for as long as the stamp
// led — the exact defect ADR 0036 §6 hit on backup archives
// (ranger-base-rgv61). It is reported and never dated.
//
// **Suppression, and its inverse.** A check that is red BY DESIGN and
// already tracked by a bead must not cry wolf every cycle, so
// `verify_box_accepted:` maps a check name to the bead id that owns it and
// the row names the id beside the red check. The red is still ON the row —
// the operator ruled the naming, not an auto-filed bead and not silence
// (option (a), held back) — because a suppression that HIDES is how a check
// goes permanently dark. The inverse is guarded too: an acceptance whose
// check now passes, or that names no check the run knows about, suppresses
// nothing and is reported, because a suppression outliving its cause is the
// same darkness arriving slowly.
//
// **How an instance stops being asked.** There is deliberately no
// `verify_box_max_age: 0` escape hatch — the ruling made the freshness rule
// mandatory, and a key that switches off "is anyone checking this box" is
// the schedule nobody installed wearing a config value. An instance that
// genuinely does not run this control removes the state file and the two
// config keys, and reads as unarmed, which is what it is. An instance that
// keeps the file gets the row, which is also what it is.
//
// **This is G10, and that is a real row and not a row count.** ADR 0029 as
// simplified 2026-09-05 retired the closed-at-nine fiction in as many words
// and set the bar for a new observation: a documented predicate, owner,
// scope and class. All four are here and in 0029's table.
//
// LANE, never URGENT, in every case. 0029 defines URGENT as "the shop is
// stopped": a codex pin that moved, a hook that went stale, a control that
// has not run since Tuesday — none of them stops a dispatch, and making the
// one class that means stop-everything also mean "the box has drifted" would
// cost the pulse the distinction it escalates on. It is the same argument
// the backup carry-over makes, and it lands the same way.

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

// DefaultVerifyBoxMaxAge is the freshness budget when the operator has named
// none: a daily schedule plus two hours, so a job that starts at 04:20 and
// takes a while is never called stale for finishing late.
const DefaultVerifyBoxMaxAge = 26 * time.Hour

// The four statuses the script writes, which are the three each check
// defines (0 ok / 1 finding / 2 nothing measured) plus the runner's verdict
// on a check that answered something else or could not be run at all.
const (
	VerifyBoxOK         = "ok"
	VerifyBoxFinding    = "finding"
	VerifyBoxUnmeasured = "not-measured"
	VerifyBoxError      = "error"
)

// VerifyBoxStatePath is the one store: the last run's verdict, written by
// scripts/verify-box.sh and read here. It sits in state/ beside the watch
// log and pause.yaml because it is exactly that kind of fact — what a
// process on this box last observed, kept where the next process can find
// it.
func VerifyBoxStatePath(a *App) string { return filepath.Join(a.StateDir, "verify-box.yaml") }

// VerifyBoxLogPath is the OTHER half, and it is not this file's store: the
// launchd job's stdout and stderr (com.posse.verify-box.plist, versioned in
// $CONSTITUTION/scripts/launchd/, whose StandardOutPath and StandardErrorPath
// name it).
// A run that finishes writes a verdict; a run that is killed, or that dies
// in the shell before it reaches the write, leaves only what it had printed
// — and with no such file it leaves nothing at all, which is how a schedule
// stops in silence. The stale row names this path because it is the answer
// to the next question the row raises.
func VerifyBoxLogPath(a *App) string { return filepath.Join(a.StateDir, "verify-box.log") }

// VerifyBoxCheck is one roster row's answer in the last run.
type VerifyBoxCheck struct {
	Name   string
	Status string

	// Bead is the accepted-risk entry that owns this check's red, "" when
	// there is none. It is read from config, not from the run.
	Bead string
}

// Red reports whether this answer is one a human has to look at.
//
// "Nothing measured" is deliberately not red PER CHECK — a box with no codex
// installed answers 2 for the codex pin forever and that is not a finding —
// but a run in which NOTHING was measured is (see VerifyBoxReading.
// Unmeasured), because that is a green board over an empty room.
//
// EVERYTHING ELSE IS RED, including a token this reader does not know and an
// empty one. That is the default an asymmetric question deserves: the two
// benign answers are enumerated and anything else is a record this reader
// cannot interpret. Reading an unknown token as "not red" would let the
// producer grow a fifth status — or a typo — and take a check off the
// surface without taking it off the roster, which is the same green board
// over an unrun check that this whole row exists to end.
func (c VerifyBoxCheck) Red() bool {
	switch c.Status {
	case VerifyBoxOK, VerifyBoxUnmeasured:
		return false
	}
	return true
}

// VerifyBoxReading is what the state file says, plus the acceptances the
// config pairs with it.
type VerifyBoxReading struct {
	Armed  bool
	Path   string
	Log    string
	MaxAge time.Duration

	// Ran is false when the instance is armed and no run has ever been
	// recorded — the predecessor's exact failure, an arrangement installed
	// and never fired, and it is STALE rather than silent for that reason.
	Ran    bool
	At     time.Time
	Age    time.Duration
	Stale  bool
	Ahead  time.Duration // >0 when the stamp leads this box's clock
	RC     int
	Checks []VerifyBoxCheck
	Err    error

	// Accepted is `verify_box_accepted:` as it stood when the file was
	// read. It is carried on the reading rather than re-read by each
	// rendering: two readers of one config that disagree about which reds
	// are tracked is a suppression nobody can reason about.
	Accepted map[string]string
}

// VerifyBoxMaxAge is how old the last verdict may be before it stops being a
// statement about now. Same grammar and same typo handling as every other
// interval key: a value that does not parse is named and the default stands.
func (a *App) VerifyBoxMaxAge(errw io.Writer) time.Duration {
	return a.attnAge("verify_box_max_age", DefaultVerifyBoxMaxAge, errw)
}

// VerifyBoxAccepted is the accepted-risk list: check name → the bead id that
// tracks its red. A one-level map, read with the same reader every other
// config map goes through, so the grammar is posse's and not one invented
// here.
//
//	verify_box_accepted:
//	  verify-codex-pin: ranger-base-femsg
func (a *App) VerifyBoxAccepted() map[string]string {
	out := map[string]string{}
	for _, kv := range YamlMapPairs(a.ConfigPath, "verify_box_accepted") {
		if k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]); k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

// VerifyBoxConfigured reports whether the operator has said anything about
// this control — either key is enough. It is the half of ARMED that a state
// file cannot supply: an instance whose schedule was installed and has never
// fired has no file to read, and reading that as "nothing to say" would
// reproduce the failure this whole row exists to catch.
func (a *App) VerifyBoxConfigured() bool {
	return yamlHasKey(a.ConfigPath, "verify_box_max_age") || yamlHasKey(a.ConfigPath, "verify_box_accepted")
}

// VerifyBoxFreshness reads the state file. It never writes, never runs a
// check and never opens anything else: `posse status` and the cockpit run
// this on every tick.
func (a *App) VerifyBoxFreshness(now time.Time, errw io.Writer) VerifyBoxReading {
	r := VerifyBoxReading{
		Path:   VerifyBoxStatePath(a),
		Log:    VerifyBoxLogPath(a),
		MaxAge: a.VerifyBoxMaxAge(errw),
	}
	r.Accepted = a.VerifyBoxAccepted()
	st, err := os.Stat(r.Path)
	switch {
	case err != nil && os.IsNotExist(err):
		// No run recorded. On an instance that asked for this control that
		// is the loudest reading there is; on any other it is a file posse
		// has no business having an opinion about.
		r.Armed = a.VerifyBoxConfigured()
		r.Stale = r.Armed
		return r
	case err != nil:
		r.Armed = a.VerifyBoxConfigured()
		r.Err = err
		return r
	case st.IsDir():
		r.Armed = true
		r.Err = Die("%s is a directory, not the verdict scripts/verify-box.sh writes", AbbrevHome(r.Path))
		return r
	}
	// A file exists, so this instance is running the control however it was
	// arranged — by a schedule, or by a person typing `make verify-box`.
	r.Armed = true

	raw := strings.TrimSpace(YamlGet(r.Path, "at"))
	if raw == "" {
		r.Err = Die("%s carries no at: stamp — it is not a verdict this reader can date", AbbrevHome(r.Path))
		return r
	}
	at, perr := time.Parse(time.RFC3339, raw)
	if perr != nil {
		r.Err = Die("%s: at: %q is not an RFC3339 stamp (%v)", AbbrevHome(r.Path), raw, perr)
		return r
	}
	r.Ran = true
	r.At = at.UTC()
	if r.At.After(now) {
		// Reported, never dated (the backup archives' lesson,
		// ranger-base-rgv61): BlindFor renders a negative age as "0s", so
		// dating this would print the freshest possible verdict for as long
		// as the stamp led the clock.
		r.Ahead = r.At.Sub(now)
		r.Stale = true
	} else {
		r.Age = now.Sub(r.At)
		r.Stale = r.Age > r.MaxAge
	}

	// -1 for "no rc line, or one that is not a number": the cross-check
	// below then reports the record as contradicting itself, which is what a
	// verdict file with no verdict in it is. Atoi and not Sscanf, which
	// stops at the first non-digit and would read "1x" as a clean 1.
	r.RC = -1
	if v := strings.TrimSpace(YamlGet(r.Path, "rc")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			r.RC = n
		}
	}
	for _, kv := range YamlMapPairs(r.Path, "checks") {
		name, status := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		if name == "" {
			continue
		}
		r.Checks = append(r.Checks, VerifyBoxCheck{Name: name, Status: status, Bead: r.Accepted[name]})
	}
	// The record must not contradict itself. `rc:` is the runner's own
	// verdict and the `checks:` map is what it was computed from, so the two
	// are recomputable from each other — and a file where they disagree is a
	// file something has edited, truncated or half-written, which is not a
	// verdict at all.
	//
	// This is what keeps `rc:` from being decoration. A field nothing reads
	// is a second store that can drift from the first in silence, and this
	// file's whole claim is that there is ONE store; either the runner's
	// verdict is checked against the checks or it should not be written.
	if want := verifyBoxVerdict(r.Checks); r.RC != want {
		r.Err = Die("%s says rc: %d but its %d check(s) compute %d — the record contradicts itself and is not a verdict",
			AbbrevHome(r.Path), r.RC, len(r.Checks), want)
	}
	return r
}

// verifyBoxVerdict recomputes the runner's exit status from the per-check
// answers, by the runner's own rule: any red is 1, an all-unmeasured run is
// 2 (never a pass), and everything else is 0.
func verifyBoxVerdict(checks []VerifyBoxCheck) int {
	red, unmeasured := 0, 0
	for _, c := range checks {
		switch {
		case c.Red():
			red++
		case c.Status == VerifyBoxUnmeasured:
			unmeasured++
		}
	}
	switch {
	case red > 0:
		return 1
	case len(checks) > 0 && unmeasured == len(checks):
		return 2
	}
	return 0
}

// Red is the checks a human has to look at, in the order the run reported
// them.
func (r VerifyBoxReading) Red() []VerifyBoxCheck {
	var out []VerifyBoxCheck
	for _, c := range r.Checks {
		if c.Red() {
			out = append(out, c)
		}
	}
	return out
}

// Unmeasured reports the run in which NOTHING could be measured — every
// check that ran answered 2. That is the answer a box with none of the
// runtimes installed gives, and the script's own exit status calls it out
// rather than laundering it: "a schedule that treats 2 as green is a green
// light on an empty room".
func (r VerifyBoxReading) Unmeasured() bool {
	if len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if c.Status != VerifyBoxUnmeasured {
			return false
		}
	}
	return true
}

// StaleAcceptances is the suppression's inverse: every `verify_box_accepted:`
// entry that is suppressing nothing, because the check it names came back ok
// in the last run or is not in the run at all (a bead that landed, a check
// that was renamed, a typo). One row each, named, because an acceptance
// nobody retires is a check that goes dark quietly — which is the same
// defect as an unrun control, arriving a bead at a time.
//
// A check that answered "not measured" is left alone: that is neither the
// finding being fixed nor the entry naming nothing, and calling it stale
// would file a row against every box that has no codex installed.
func (r VerifyBoxReading) StaleAcceptances() [][2]string {
	ran := map[string]VerifyBoxCheck{}
	for _, c := range r.Checks {
		ran[c.Name] = c
	}
	var out [][2]string
	for name, bead := range r.Accepted {
		// Red() and not a status literal, so this stays the exact inverse of
		// the row it guards: a check whose token this reader does not know
		// is RED, and an acceptance suppressing one is doing its job. Keying
		// on the three known strings instead would call that acceptance
		// stale and tell the operator to retire a live one.
		c, ok := ran[name]
		if ok && (c.Red() || c.Status == VerifyBoxUnmeasured) {
			continue
		}
		out = append(out, [2]string{name, bead})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// Counts is the ok / red / not-measured split, for the one-line rendering.
func (r VerifyBoxReading) Counts() (ok, red, unmeasured int) {
	for _, c := range r.Checks {
		switch {
		case c.Status == VerifyBoxOK:
			ok++
		case c.Red():
			red++
		case c.Status == VerifyBoxUnmeasured:
			unmeasured++
		}
	}
	return ok, red, unmeasured
}

// Line is the quiet half `posse status` prints on any armed instance — the
// verdict and its AGE, whether or not it is a condition. The loud half is
// the G10 row below; this line is here for the reason the backup line is
// (ADR 0036 §6): a shop check prints conditions, and "the box was checked
// clean 3h ago" is not one, it is the standing reading the conditions are
// judged against. Without it a green board and an unarmed control look
// identical, which is the silence this bead is about.
func (r VerifyBoxReading) Line() string {
	switch {
	case r.Err != nil:
		return fmt.Sprintf("verify-box · %s could not be read: %v", AbbrevHome(r.Path), r.Err)
	case !r.Ran:
		return fmt.Sprintf("verify-box · NEVER RUN · no verdict at %s (max age %s)", AbbrevHome(r.Path), BlindFor(r.MaxAge))
	case r.Ahead > 0:
		return fmt.Sprintf("verify-box · NOT A READING · %s is stamped %s AHEAD of this box's clock",
			AbbrevHome(r.Path), BlindFor(r.Ahead))
	}
	ok, red, unmeasured := r.Counts()
	stale := ""
	if r.Stale {
		stale = fmt.Sprintf(" · STALE, older than %s", BlindFor(r.MaxAge))
	}
	return fmt.Sprintf("verify-box · %s ago · %d ok, %d red, %d not measured%s%s",
		BlindFor(r.Age), ok, red, unmeasured, stale, r.acceptedSuffix())
}

func (r VerifyBoxReading) acceptedSuffix() string {
	var named []string
	for _, c := range r.Red() {
		if c.Bead != "" {
			named = append(named, c.Name+" tracked by "+c.Bead)
		}
	}
	if len(named) == 0 {
		return ""
	}
	return " · " + strings.Join(named, ", ")
}

// GovRows is G10: the governance rendering of that same reading.
//
// STALE short-circuits everything below it, and that is the freshness rule
// doing its work. A verdict this reader will not date cannot be reported
// check by check either — the reds in a two-week-old file are two weeks old,
// and rendering them as today's would be the one thing the ruling forbade.
func (r VerifyBoxReading) GovRows() []GovCondition {
	if !r.Armed || r.Err != nil {
		return nil
	}
	row := func(key, detail string) GovCondition {
		return GovCondition{ID: "G10", Class: GovLane, Key: key, Detail: detail}
	}
	switch {
	case !r.Ran:
		return []GovCondition{row("verify-box-stale", fmt.Sprintf(
			"the live-box checks have never recorded a verdict — nothing has written %s, so nothing on this box is checking its own pins, credential paths or hooks (config verify_box_max_age: %s)",
			AbbrevHome(r.Path), BlindFor(r.MaxAge)))}
	case r.Ahead > 0:
		return []GovCondition{row("verify-box-stale", fmt.Sprintf(
			"the last live-box verdict is stamped %s AHEAD of this box's clock (%s) — not a usable reading, so the box is unchecked as far as this surface knows; %s catches a run that dies before its verdict",
			BlindFor(r.Ahead), AbbrevHome(r.Path), AbbrevHome(r.Log)))}
	case r.Stale:
		return []GovCondition{row("verify-box-stale", fmt.Sprintf(
			"the last live-box verdict is %s old, past verify_box_max_age: %s — checked-recently-and-clean is the only green, and this is not recent (%s; a run that died before its verdict leaves a line in %s)",
			BlindFor(r.Age), BlindFor(r.MaxAge), AbbrevHome(r.Path), AbbrevHome(r.Log)))}
	}

	var out []GovCondition
	if r.Unmeasured() {
		out = append(out, row("verify-box-unmeasured", fmt.Sprintf(
			"the last live-box run measured NOTHING — all %d check(s) answered \"nothing measured\" %s ago. That is not a pass; it is a green light on an empty room (%s)",
			len(r.Checks), BlindFor(r.Age), AbbrevHome(r.Path))))
	} else if red := r.Red(); len(red) > 0 {
		names := make([]string, 0, len(red))
		parts := make([]string, 0, len(red))
		for _, c := range red {
			names = append(names, c.Name)
			p := c.Name + " " + c.Status
			if c.Bead != "" {
				// The suppression, and the whole of it: the red stays on
				// the row and carries the bead that owns it, so a reader
				// sees a tracked condition rather than a new alarm.
				p += " (tracked by " + c.Bead + ")"
			}
			parts = append(parts, p)
		}
		sort.Strings(names)
		out = append(out, row("verify-box:"+strings.Join(names, ","), fmt.Sprintf(
			"the live-box checks came back red %s ago: %s — the box is not what the repo says it is (%s)",
			BlindFor(r.Age), strings.Join(parts, "; "), AbbrevHome(r.Path))))
	}
	for _, sa := range r.StaleAcceptances() {
		out = append(out, row("verify-box-accept-stale:"+sa[0], fmt.Sprintf(
			"verify_box_accepted: still suppresses %s as tracked by %s, but the last run did not report it red — retire the entry, or the day that check goes red again nobody will be told",
			sa[0], sa[1])))
	}
	return out
}
