package posse

// The launch ledger — `RFC3339 runtime bead persona`, one line per launch,
// append-only, never rotated or pruned by posse.
//
// One ledger has this shape today: `uncounted.log`, the beads dispatch sent
// to a runtime whose dollars no cost adapter prices (uncounted.go, ADR 0013
// §5). It had two while ADR 0010's automatic overflow existed, and the shape
// and its helpers stayed here when that mechanism was removed — not out of
// generality, but because what these three functions encode is what it cost
// to get a count off a file right, and none of that is about overflow:
//
//   - a torn or hand-edited line is a launch nobody can date, so it is an
//     ERROR and not a skip — the one thing it is not is evidence that
//     nothing was launched (ranger-base-lasj);
//   - a ledger that can be READ but not APPENDED to counts every pass at
//     whatever it already says, so a cap of 1 over an empty one admits a
//     launch per pass forever and records none of them (ranger-base-2y96);
//   - whether a file can be appended to is answered by an OPEN and never by
//     its mode bits: 0444 is a promise about a uid, and root — or an ACL —
//     defeats it in both directions (ranger-base-c00).

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// LedgerWindow is the rolling window a ledger count is taken over. Seven
// days and beads, not dollars: a pool with no meter posse can read cannot be
// counted in the unit that matters, so the count of what dispatch itself
// sent there is the only honest number in reach. Rolling rather than
// calendar because a weekly pool's reset day is the provider's secret — a
// rolling window upper-bounds every calendar week without knowing it.
const LedgerWindow = 7 * 24 * time.Hour

// LedgerEntry is one line of a launch ledger.
type LedgerEntry struct {
	At      time.Time
	Runtime string
	Bead    string
	Persona string
}

func (e LedgerEntry) line() string {
	return fmt.Sprintf("%s %s %s %s\n", e.At.UTC().Format(time.RFC3339), e.Runtime, e.Bead, e.Persona)
}

// appendLedger records one launch. Append-only and never rotated or pruned
// by posse: it is the only evidence of what a pool with no meter was spent
// on, and the metrics are read off it.
func (a *App) appendLedger(path string, e LedgerEntry) error {
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(e.line()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// countLedger is how many beads went to this runtime inside the window
// ending at now — the number a cap is compared against. Counted per runtime,
// so changing which runtime a cap names does not charge the new pool for the
// old one's week.
//
// A missing ledger is zero, not an error: the first launch creates it.
//
// A line that is not a ledger entry is an ERROR, not a skip (ranger-base-lasj).
// Skipping it reads as "that was not a launch", and the one thing a torn or
// hand-edited line is not is evidence that nothing was launched — it is a
// launch nobody can date, so the week's total is unknown. The caller already
// fails closed on an unreadable ledger (uncountedSkip), which is the honest
// answer here too: an unknown count is not a licence to spend a pool with no
// meter. Whole-blank lines are the one exception: appendLedger writes a
// newline-terminated line in one call, so a torn write leaves a prefix and
// never an empty line, and an empty line carries no record to lose.
//
// The shape is the one appendLedger writes — RFC3339, runtime, bead, persona —
// so a short line is a truncated one and counts as corrupt on the same reading.
func countLedger(path, runtime string, now time.Time, window time.Duration) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()
	cutoff := now.Add(-window)
	n := 0
	ln := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		ln++
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 4 {
			return 0, fmt.Errorf("line %d is not a ledger entry (%d fields, want %s runtime bead persona)", ln, len(fields), time.RFC3339)
		}
		at, err := time.Parse(time.RFC3339, fields[0])
		if err != nil {
			return 0, fmt.Errorf("line %d is not dated: %v", ln, err)
		}
		if fields[1] != runtime || at.Before(cutoff) {
			continue
		}
		n++
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return n, nil
}

// ledgerAppendable reports whether appendLedger could write to path right
// now, and writes nothing to it either way. It is the precondition for
// spending a pool the ledger is the only record of: a cap counted off a file
// nothing can be added to is a cap of zero that reads as room forever.
//
// The probe is an OPEN, never the mode bits: a 0444 file is a promise about
// a uid, and root — or an ACL — defeats it in both directions
// (ranger-base-c00). O_CREATE is deliberately absent so a pass that launches
// nothing on an uncounted runtime leaves no ledger behind; when there is no
// ledger yet, what has to be writable is the directory the first append will
// create it in, so that is what gets opened instead, and the probe file is
// taken away again.
func (a *App) ledgerAppendable(path string) error {
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		return f.Close()
	}
	if !os.IsNotExist(err) {
		return err
	}
	probe, err := os.CreateTemp(a.StateDir, ".ledger-probe-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Remove(name)
}
