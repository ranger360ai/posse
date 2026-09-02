package posse

// A stamp from the FUTURE, in both readers of the archive directory (bead
// ranger-base-rgv61; ADR 0036 §4 and §6).
//
// The defect this file pins was one comparison wide and it took out both
// surfaces at once. `backupTick` asked `now - stamp < interval` and returned
// when that was true; a negative age is under EVERY interval, so one archive
// stamped ahead of the clock stopped the schedule for as long as the stamp
// led it — silently, nothing on stderr. And `BackupFreshness` derived its
// age from the same stamp, where `BlindFor` renders every negative duration
// as "0s": the surface that exists to catch a missing archive answered
// "0s ago", the freshest reading there is. Measured at 624f579: one 18-byte
// file named `posse-backup-<now+72h>.tar.gz`, two ticks, nothing written and
// nothing said.
//
// `listBackups` is not a second line of defence and is not asked to be: it
// filters on the name prefix, the suffix and a parseable stamp, so any file
// with the right name counts. Nothing here deletes or renames the planted
// file either — a clock that was ahead when an archive was published and
// then corrected leaves a perfectly good archive wearing a time nobody can
// trust, and the treatment is to stop reading it as a clock, not to destroy
// it.
//
// Every pin below carries a CONTROL arm, because the two obvious wrong fixes
// both pass a one-armed version of these tests: a tick made unconditional
// passes "it wrote an archive", and a freshness reading made unconditionally
// stale passes "it did not say fresh".

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// plantBackup writes a file that listBackups will count as a published
// archive, stamped at `at`. Eighteen bytes of nothing: the point is that the
// NAME is the whole of what both readers under test look at, so a file that
// is not an archive at all is the honest fixture.
func plantBackup(t *testing.T, a *App, at time.Time) string {
	t.Helper()
	name := backupPrefix + at.UTC().Format(backupStamp) + backupSuffix
	write(t, filepath.Join(a.BackupDir(), name), "not an archive\n")
	return name
}

// ─── the level trigger (ADR 0036 §4) ─────────────────────────────────────────

// One archive from the future must not stop the schedule, and it must not do
// it quietly either.
func TestBackupLoopRunsPastAFutureStampedArchive(t *testing.T) {
	t.Parallel()
	d, a, at := backupLoopRig(t, "1h")
	var errs strings.Builder
	d.Err = &errs
	cfg, err := LoadBackupConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	planted := plantBackup(t, a, at.Add(72*time.Hour))

	d.backupTick(cfg)

	got := archives(t, a)
	if len(got) != 2 {
		t.Fatalf("a tick next to a 72h-ahead stamp left %d archives (%v), want the planted one plus a real one", len(got), got)
	}
	// And a REAL one: the scheduled run is RunBackup entire, so the archive
	// it published is one VerifyBackup opens. Without this the pin would
	// pass on a tick that had merely touched a file.
	var wrote string
	for _, n := range got {
		if n != planted {
			wrote = n
		}
	}
	if wrote == "" {
		t.Fatalf("nothing but the planted file is in %v", got)
	}
	if _, err := VerifyBackup(filepath.Join(a.BackupDir(), wrote)); err != nil {
		t.Fatalf("the archive the tick published does not verify: %v", err)
	}
	// The planted file is still there. This loop reports a stamp it cannot
	// use; it does not clean up after the operator's clock.
	if _, err := os.Stat(filepath.Join(a.BackupDir(), planted)); err != nil {
		t.Errorf("the tick removed the future-stamped file: %v", err)
	}
	// Said, not just done.
	for _, want := range []string{planted, "AHEAD", "72h00m"} {
		if !strings.Contains(errs.String(), want) {
			t.Errorf("the tick's narration is missing %q:\n%s", want, errs.String())
		}
	}

	// It is still a LEVEL trigger. The archive just published is a stamp
	// this tick CAN date, so the next tick inside the interval declines —
	// the fix must not have turned the loop into "write on every tick for
	// as long as the future stamp stands", which under a 6h interval and a
	// 72h lead would be twelve archives nobody asked for.
	d.backupTick(cfg)
	if got := archives(t, a); len(got) != 2 {
		t.Fatalf("a second tick inside the interval left %d archives (%v), want the same two", len(got), got)
	}
}

// The control arm for the pin above, and the one that keeps the fix from
// being "tick unconditionally": a usable archive inside the interval still
// declines, future stamp or no future stamp — and the future stamp is still
// named while it declines, because a level trigger reports a level for as
// long as it stands.
func TestBackupLoopStillDeclinesBehindAUsableArchive(t *testing.T) {
	t.Parallel()
	d, a, at := backupLoopRig(t, "1h")
	var errs strings.Builder
	d.Err = &errs
	cfg, _ := LoadBackupConfig(a)
	planted := plantBackup(t, a, at.Add(72*time.Hour))
	fresh := plantBackup(t, a, at.Add(-5*time.Minute))

	d.backupTick(cfg)
	d.backupTick(cfg)

	got := archives(t, a)
	if len(got) != 2 {
		t.Fatalf("two ticks behind a 5m-old archive wrote %d archives (%v), want the two planted", len(got), got)
	}
	if !strings.Contains(errs.String(), planted) {
		t.Errorf("the future stamp went unmentioned while the tick declined behind %s:\n%s", fresh, errs.String())
	}
	// Twice, once per tick: the condition is a level, and a line printed
	// once hours ago in a scrollback nobody is watching is the silence this
	// bead was about.
	if n := strings.Count(errs.String(), planted); n != 2 {
		t.Errorf("the future stamp was named %d times over two ticks, want 2:\n%s", n, errs.String())
	}
}

// ─── the freshness surface (ADR 0036 §6) ─────────────────────────────────────

// The reading that used to say "0s ago". Nothing on the box can be dated, so
// the instance is stale and the line says why.
func TestFreshnessRefusesToDateAFutureStamp(t *testing.T) {
	a := freshRig(t, "backup_max_age: 12h\n", -72*time.Hour)
	planted := backupPrefix + govNow.Add(72*time.Hour).UTC().Format(backupStamp) + backupSuffix

	f := a.BackupFreshness(govNow, os.Stderr)

	if f.Newest != "" || f.Age != 0 {
		t.Fatalf("a future stamp was taken as a reading: newest=%q age=%v", f.Newest, f.Age)
	}
	if !f.Armed || !f.Stale {
		t.Fatalf("armed=%v stale=%v — an instance whose only archive cannot be dated has no backup it can prove", f.Armed, f.Stale)
	}
	if f.Count != 1 || f.Future != 1 || f.FutureNewest != planted || f.FutureAhead != 72*time.Hour {
		t.Errorf("count=%d future=%d newest=%q ahead=%v, want the one planted file at +72h", f.Count, f.Future, f.FutureNewest, f.FutureAhead)
	}
	line := f.Line()
	for _, want := range []string{"NO USABLE ARCHIVE", planted, "AHEAD", "72h00m", "1 on box"} {
		if !strings.Contains(line, want) {
			t.Errorf("the line is missing %q:\n%s", want, line)
		}
	}
	// The exact word the defect produced, and the only one an operator
	// scanning `posse status` would have read as done.
	if strings.Contains(line, "0s ago") {
		t.Errorf("the line still calls a stamp from the future fresh:\n%s", line)
	}

	// CONTROL: the same rig with the same file dated in the PAST reads
	// normally. Without this, a BackupFreshness hard-wired to "stale" would
	// pass every assertion above.
	b := freshRig(t, "backup_max_age: 12h\n", 90*time.Minute)
	if g := b.BackupFreshness(govNow, os.Stderr); g.Stale || g.Future != 0 || !strings.Contains(g.Line(), "1h30m ago") {
		t.Errorf("a 90m-old archive no longer reads as fresh: stale=%v future=%d\n%s", g.Stale, g.Future, g.Line())
	}
}

// A future stamp does not hide the archives BEHIND it either: the reading
// falls through to the newest one it can date, which is the whole point of
// splitting rather than refusing.
func TestFreshnessFallsThroughToTheNewestDatableArchive(t *testing.T) {
	a := freshRig(t, "backup_max_age: 12h\n", -72*time.Hour, 26*time.Hour, 3*time.Hour)
	f := a.BackupFreshness(govNow, os.Stderr)

	want := backupPrefix + govNow.Add(-3*time.Hour).UTC().Format(backupStamp) + backupSuffix
	if f.Newest != want || f.Age != 3*time.Hour {
		t.Fatalf("newest=%q age=%v, want the 3h-old archive", f.Newest, f.Age)
	}
	if f.Stale {
		t.Errorf("a 3h-old archive under a 12h threshold reads stale: %s", f.Line())
	}
	if f.Count != 3 || f.Future != 1 {
		t.Errorf("count=%d future=%d, want 3 on box of which 1 is ahead", f.Count, f.Future)
	}
	// Fresh AND flagged: the age is honest and the fault is still named, so
	// the operator whose clock jumped hears about it before the archive
	// behind it ages out.
	line := f.Line()
	if !strings.Contains(line, "3h00m ago") || !strings.Contains(line, "AHEAD") {
		t.Errorf("the line must carry both the real age and the future stamp:\n%s", line)
	}
}

// The loud half, one layer over: the governance row fires on an armed
// instance whose only archive is undatable. Both surfaces move together —
// the pair going quiet at the same time is the combination this bead found,
// and the one with no witness left over.
func TestGovernanceRaisesStaleOnAnUndatableArchive(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "backup_max_age: 12h\n")
	plantBackup(t, b.App, govNow.Add(72*time.Hour))

	if keys := shopKeys(t, govIn(t, b)); !containsStr(keys, "backup-stale") {
		t.Errorf("conditions = %v, want backup-stale on an instance with no datable archive", keys)
	}
	if d := b.App.BackupFreshness(govNow, os.Stderr).GovDetail(); !strings.Contains(d, "ahead of the clock") {
		t.Errorf("the governance detail does not name the reason:\n%s", d)
	}

	// CONTROL: a datable archive inside the threshold clears the row, in the
	// same directory, with the future-stamped file still sitting in it.
	plantBackup(t, b.App, govNow.Add(-time.Hour))
	if keys := shopKeys(t, govIn(t, b)); containsStr(keys, "backup-stale") {
		t.Errorf("a 1h-old archive still reads stale next to a future stamp: %v", keys)
	}
}

// ─── the split itself ────────────────────────────────────────────────────────

// The boundary is `After`, and it is not a matter of taste: an archive's
// stamp is written by Format at one-second granularity from the clock that
// is publishing it, so a stamp is never AHEAD of its own writer and a grace
// window here would only widen the blind spot the bead is about. Equal to
// now is a reading.
func TestSplitBackupsAtIsStrictlyAfter(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 1, 3, 15, 0, 0, time.UTC)
	name := func(d time.Duration) string {
		return backupPrefix + at.Add(d).UTC().Format(backupStamp) + backupSuffix
	}
	names := []string{name(-time.Hour), name(0), name(time.Second), name(72 * time.Hour)}

	usable, future := splitBackupsAt(names, at)
	if len(usable) != 2 || usable[1] != name(0) {
		t.Errorf("usable = %v, want the -1h and the exactly-now archives", usable)
	}
	if len(future) != 2 || future[0] != name(time.Second) {
		t.Errorf("future = %v, want the +1s and the +72h archives", future)
	}
	// Order survives, because both callers take the last element as their
	// newest.
	if future[1] != name(72*time.Hour) {
		t.Errorf("future is not oldest-first: %v", future)
	}
	// A listing with nothing ahead of the clock hands back the listing and
	// no second slice — the ordinary case, unchanged.
	if u, f := splitBackupsAt(names[:2], at); len(u) != 2 || f != nil {
		t.Errorf("splitBackupsAt(past only) = %v, %v; want the listing and nil", u, f)
	}
}
