package posse

// QA pins for the refusals fold (ADR 0025 §4, refusalfold.go), filed while
// verifying the close of ranger-base-l40c under ranger-base-w7h58. Two
// shapes the close's own pins do not reach:
//
//   - the SIZE arm of the tamper switch. refusalfold_test.go's
//     TestFoldDetectsTruncationByOffset uses a 24-byte spool, and
//     os.ReadFile hands back a slice with a 512-byte MINIMUM capacity, so
//     `content[:cur.Offset]` in the hash arm below it reads inside cap and
//     the tamper line comes out of the HASH arm instead. Measured: with
//     `case size < cur.Offset:` disabled, that test still passes. Below
//     512 bytes the two arms cannot be told apart; above it they can.
//   - the fold's line reader is a bufio.Scanner at its default token cap,
//     which is the trail's own erasure hole (see the second pin).
//
// The QA suffix is the file's, not the assertions': these run hermetically
// with the rest of the package.

import (
	"os"
	"strings"
	"testing"
)

// A spool the caged process truncates to zero must fold as tampered even
// when it held more than os.ReadFile's minimum buffer — the arm that says
// so is `size < cur.Offset`, and without it this is not a wrong answer but
// a `slice bounds out of range [:1600] with capacity 512` panic in the
// dispatch sweep that calls the fold.
func TestQAFoldDetectsTruncationOfASpoolLargerThanReadFilesBuffer(t *testing.T) {
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	// 1600 bytes: past os.ReadFile's 512-byte floor, so a cursor offset
	// taken here cannot be sliced out of an emptied file by accident.
	if err := os.WriteFile(spool, []byte(strings.Repeat("refused a thing\n", 100)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spool, nil, 0o644); err != nil { // `: > refusals.log` inside
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatalf("the fold must survive a spool emptied under a large cursor, not error or panic: %v", err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "refusals spool tampered [fold] session=s1") {
		t.Errorf("a spool emptied under a 1600-byte cursor must fold as tampered:\n%s", b)
	}
}

// LIVE DEFECT, pinned GREEN: filed as its own bead from ranger-base-w7h58.
//
// foldedLines reads the spool with a bufio.Scanner at its default 64KiB
// token cap and ignores sc.Err(). A line longer than that stops the scan:
// the fold appends nothing further, and the cursor still advances past the
// whole chunk — so every complete line AFTER the long one is dropped from
// the canonical log permanently, with no tamper line. The spool is the one
// file mounted rw into the cage, so the caged persona writes both.
//
// This asserts the hole, deliberately. When the fold learns to carry a long
// line (or to refuse to advance past one), THIS TEST GOES RED — that is the
// signal to delete it and pin the fixed behaviour instead.
func TestQAFoldSilentlyDropsEveryLineAfterAnOverlongOne(t *testing.T) {
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	body := "kept\n" + strings.Repeat("A", 70*1024) + "\n" + "swallowed refusal\n"
	if err := os.WriteFile(spool, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	if !strings.Contains(log, "session=s1 kept") {
		t.Errorf("setup: the line before the overlong one must fold normally:\n%s", log)
	}
	if strings.Contains(log, "swallowed refusal") {
		t.Errorf("FIXED: the fold now carries lines past an overlong one. Delete this pin and assert the new behaviour — the hole it documents is closed:\n%s", log)
	}
	if strings.Contains(log, "tampered") {
		t.Errorf("FIXED: the fold now marks an overlong line rather than swallowing it silently. Delete this pin and assert the new behaviour:\n%s", log)
	}
	// And it is unrecoverable: a second fold has already advanced past it.
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b2, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b2), "swallowed refusal") {
		t.Errorf("FIXED: a re-fold now recovers what the overlong line swallowed. Delete this pin:\n%s", b2)
	}
}

// The documented residual of ADR 0025 §4, pinned GREEN — and the ADR now
// cites this test by name for it (amended 2026-09-01, ranger-base-j3r6z,
// raised under ranger-base-w7h58; the §4 text said "the truncation is
// detected" until then).
//
// Detection fires on `size < cur.Offset` or a hash mismatch over the folded
// prefix, so it only ever sees a truncation BELOW what the canonical log
// already holds — the case where nothing is lost. A truncation back to the
// cursor, to any point above it, or before the first fold ever ran erases
// un-folded refusals and leaves the compared prefix byte-identical: no
// tamper line, no marker. What the design does hold is that the canonical
// log only grows from inside a cage.
//
// When detection grows to reach past the cursor, THIS TEST GOES RED — that
// is the signal to delete it and rewrite the ADR paragraph with it.
func TestQAFoldDoesNotDetectATruncationBackToItsOwnCursor(t *testing.T) {
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	if err := os.WriteFile(spool, []byte("early refusal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(spool)
	if err != nil {
		t.Fatal(err)
	}
	mark := fi.Size() // what the caged process reads before it misbehaves

	f, err := os.OpenFile(spool, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("the refusal it wants gone\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Truncate(spool, mark); err != nil { // back to where it started
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	if !strings.Contains(log, "session=s1 early refusal") {
		t.Errorf("setup: what was folded before the truncation must still be there:\n%s", log)
	}
	if strings.Contains(log, "tampered") {
		t.Errorf("FIXED: a truncation back to the cursor is now detected. Delete this pin and assert the new behaviour — ADR 0025 §4's residual is now true as written:\n%s", log)
	}
}
