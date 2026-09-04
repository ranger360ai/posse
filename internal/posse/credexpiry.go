package posse

// Expiry is a first-class answer (ADR 0019 D5, bead ranger-base-k6ha).
//
// The seam already knows when a credential dies — CredMeta.ExpiresAt, from
// the envelope's own `expiresAt` for the meter and from the `# expires=`
// stamp for a session mint. This file is the half that makes that knowledge
// arrive BEFORE the shop stops: `posse refresh` answers on demand
// (refresh.go), and these two surfaces answer unasked — the cockpit header
// once inside the window, and one stderr line per dispatch pass.
//
// Three rules the whole file is shaped by:
//
//   - Expiry gates NOTHING. No park, no degrade, no clock, no skipped
//     launch. The read's success or failure stays the only actuator (ADR
//     0019 D5), and a warning that can stop the shop is a second brake
//     nobody armed.
//   - A zero ExpiresAt is "cannot tell", and cannot-tell warns nothing. It
//     is never rendered as freshness, and it never becomes a warning that
//     an operator would have to disprove.
//   - What is warned about is what a HUMAN HAND replaces. That is the one
//     judgement here and it is a narrowing of D5's letter, so it is written
//     out in full below.
//
// DIVERGED from ADR 0019 D5's letter (ranger-base-k6ha; the amendment is
// filed as its own architecture bead): the timer surfaces carry the
// posse-owned SESSION mints only. The meter credential keeps its expiry in
// the `posse refresh` report — asked, answered — and gets no header segment
// and no per-pass line. Two reasons, both from the ADR's own text:
//
//  1. It would never be quiet. The meter credential is the runtime's
//     rotating OAuth access token, and ADR 0019's own measurement is that
//     the store of record rotates within days ("provably stale within
//     days", the darwin rejection). A rotation horizon shorter than the
//     window means every reading is inside the window, so the warning fires
//     every pass forever — and a warning that is always on is a warning
//     nobody reads, which is how the guard-off line got mistaken for noise
//     once already.
//  2. There is no hand to warn. D4 is explicit that posse writes nothing
//     here and that the runtime's login loop is the credential's only
//     writer; the loop refreshes it without an operator. Warning about the
//     next scheduled rotation of a self-refreshing credential asks for an
//     action that does not exist. When that loop is actually dead, the
//     failure is a failed READ — blindness, ADR 0018, already loud, already
//     clocked — and that is the actuator D5 says it should be.
//
//     MEASURED FALSE 2026-09-03 (ranger-base-4poib), and left standing here
//     because the behaviour it justifies is ADR 0019 D5's and not this
//     file's to overturn: the loop does NOT refresh it without an operator.
//     The access token's expiresAt is exactly 8h after the last interactive
//     `claude` write and nothing else moves it — the refreshToken beside it
//     in the same item, valid three weeks, is never used. So the hand this
//     paragraph says does not exist is the whole mechanism, and reason 1
//     (it would never be quiet) is now the only one of the two standing.
//     The amendment is ranger-base-z089h; until it lands, what 4poib bought
//     is that the failed READ this paragraph points at finally names the
//     expiry it read (planusage.go AuthFailure) instead of guessing.
//
// The narrowing is also what keeps these surfaces free: a session mint's
// expiry is a stamp in a file posse already owns, so the pass reads a few
// hundred bytes. Warning about the meter would mean execing `security` on
// every dispatch pass and every cockpit tick, against a store the whole
// instance deliberately reads once per TTL through one shared cache
// (rangerhq-tdy8).

import (
	"fmt"
	"sort"
	"time"
)

// CredExpiry is one credential posse can name a death date for, plus the
// verb that replaces it. It carries a TIME and not a rendering: the three
// surfaces render it differently (a header has a column, a pass has a line)
// and only one of them may decide what "8d" means — expiryIn does.
//
// At is never zero in a value that reaches a caller: a credential posse
// cannot date is not a warning, it is a "cannot tell" in the report.
type CredExpiry struct {
	Runtime string
	Purpose CredPurpose
	// Set is the env set holding it and Key the variable NAME inside that
	// set. Neither is the value; nothing in this file ever holds one.
	Set string
	Key string
	At  time.Time
}

// Expired is the distinct state ADR 0019 V6 asks for: past, not merely
// close. A credential already dead is not "expires in -3d".
func (e CredExpiry) Expired(now time.Time) bool { return !e.At.After(now) }

// Fix is the operator's verb, spelled so it can be pasted. It names the env
// set explicitly even when only one holds the variable, because the warning
// may be read on a box where a second set has since appeared and `posse
// refresh` refuses an ambiguous target rather than choosing.
func (e CredExpiry) Fix() string {
	return fmt.Sprintf("posse refresh %s --env-set %s", e.Runtime, e.Set)
}

// Warning is the full sentence: which credential, where it lives, when it
// dies, and what to type. The date goes through renderExpiry, which is the
// report's own renderer — a warning and the report it sends you to must not
// print two different dates for one stamp.
func (e CredExpiry) Warning(now time.Time) string {
	return fmt.Sprintf("%s %s in env set %s (%s) — %s · re-mint: %s",
		e.Runtime, e.Purpose, e.Set, e.Key, renderExpiry(e.At, now), e.Fix())
}

// Brief is the header's form, which has a column and not a line: the
// runtime's name is beside it and the date is one `posse refresh` away, so
// what is left worth spending cells on is how long there is.
func (e CredExpiry) Brief(now time.Time) string {
	if e.Expired(now) {
		return "EXPIRED"
	}
	return expiryIn(e.At.Sub(now))
}

// expiryAgo is expiryIn's inverse, and it exists because a credential can
// be found already dead: "expires in -37m" is not a sentence, and "EXPIRED"
// alone does not say whether the operator's last login wore off half an
// hour ago or last week. That distinction is the whole diagnosis for the
// meter credential, whose access token lives 8h (MEASURED 2026-09-03,
// ranger-base-4poib) — an expiry an hour old means the login loop is simply
// not being run, and one a week old means something else entirely.
//
// It carries minutes below an hour where expiryIn does not, for the same
// reason: expiryIn's smallest caller is a 14-day window where "in 0h" is a
// rounding, while an age of 37 minutes rendered "0h ago" is the answer
// erased. Above an hour the two agree unit for unit, so a reading of one
// stamp by two surfaces cannot disagree about how long "8d" is.
//
// It truncates, like expiryIn — an age reported short is the direction that
// errs towards "look again", never towards "you have time".
func expiryAgo(d time.Duration) string {
	if d < time.Hour {
		if d < time.Minute {
			return "just now"
		}
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 48*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// expiryIn is the one place a duration becomes words. Hours under two days
// and whole days above: a mint with 40 hours left is a today problem and
// "in 1d" reads like a tomorrow one, while a mint with 300 hours left told
// in hours is a number nobody can size at a glance.
//
// It truncates rather than rounds, deliberately — "in 8d" on something that
// dies in 8d20h is early, and early is the direction a warning may err in.
func expiryIn(d time.Duration) string {
	if d <= 0 {
		return "EXPIRED"
	}
	if d < 48*time.Hour {
		return fmt.Sprintf("in %dh", int(d.Hours()))
	}
	return fmt.Sprintf("in %dd", int(d.Hours()/24))
}

// ExpiringCredentials is the one reader both warning surfaces use: every
// posse-owned session mint whose `# expires=` stamp is inside
// RefreshExpiryWindow or already past, soonest first.
//
// It reads the env sets ON DISK — the same files, through the same stamp
// parser, as `posse refresh`'s report (envSetsWith/readStamps). Neither a
// dispatch pass nor a cockpit has the launched session's environment, and
// even if it did, the question these surfaces ask is about the file the
// NEXT launch will read.
//
// What it never returns: a credential with no stamp. That is the zero
// ExpiresAt, it means "cannot tell", and cannot-tell warns nothing (ADR
// 0019 D5/V6). Silence here is therefore ambiguous by construction —
// nothing is expiring, or nothing is dated — and that ambiguity is resolved
// in exactly one place, the report, which says which of the two it is for
// every credential on the box.
func (a *App) ExpiringCredentials(now time.Time) []CredExpiry {
	var out []CredExpiry
	for _, name := range a.ListRuntimes() {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			// A runtime whose yaml does not parse is `posse doctor`'s
			// sentence and the report's; a credential warning is not the
			// place an operator should first hear about it.
			continue
		}
		key := CageCredential(rt)
		if key == "" {
			continue // no session credential decided for this runtime
		}
		for _, s := range a.envSetsWith(key) {
			if s.Expires.IsZero() || s.Expires.Sub(now) > RefreshExpiryWindow {
				continue
			}
			out = append(out, CredExpiry{
				Runtime: rt.Name, Purpose: CredSession,
				Set: s.Set, Key: key, At: s.Expires,
			})
		}
	}
	// Soonest first: the one a surface with room for exactly one is
	// obliged to show. Ties break on names so a header does not flicker
	// between two equally dead credentials every tick.
	sort.Slice(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}
		if out[i].Runtime != out[j].Runtime {
			return out[i].Runtime < out[j].Runtime
		}
		return out[i].Set < out[j].Set
	})
	return out
}
