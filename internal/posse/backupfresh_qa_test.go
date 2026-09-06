package posse

// QA pin — freshness: "posse status / cockpit shows the age of the last
// backup and says loudly when it is older than a configured max" (the
// operator's 2026-09-01 sub-ruling on ranger-base-ay3dr; ADR 0036 §6; build
// bead ranger-base-a0ln0).
//
// The two halves are pinned apart because they are answered by different
// code and can regress independently: the AGE is a line, printed whenever
// the instance has asked for backups at all, and the LOUD is a condition in
// the one computation `posse status`, the cockpit's GOVERNANCE block and the
// pulse all render (ADR 0029 §2).
//
// And the third claim, which is the one the bead asked to be RESOLVED: the
// condition is a carry-over, with no G-row of its own. ADR 0036 §6 asked for
// the fact to reach the surface, not for a number. The pin is below and it is
// exact: the row's ID is empty and it renders "—".
//
// The argument that first settled it was "ADR 0029's table is closed at
// nine". That half is gone — 0029's 2026-09-05 simplification retired the
// closed-nine claim, and a real G10 landed under the bar it set instead
// (verifybox.go, bead ranger-base-jj2ax). The ASSERTION below is unchanged
// and still right: what 0036 §6 asked for is the fact, and nothing since has
// asked for the number.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// freshRig is an instance with archives of chosen ages in its backup dir.
// It writes the FILES rather than running backups: the archive directory is
// the freshness store (an archive is published only after it verifies), so
// the reader under test is a directory listing and the ages are the names.
func freshRig(t *testing.T, config string, ages ...time.Duration) *App {
	t.Helper()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, config)
	dir := a.BackupDir()
	for _, age := range ages {
		name := backupPrefix + govNow.Add(-age).UTC().Format(backupStamp) + backupSuffix
		write(t, filepath.Join(dir, name), "not a real archive, and this reader never opens one\n")
	}
	return a
}

// ─── the age, always ─────────────────────────────────────────────────────────

func TestBackupFreshnessLineCarriesTheAge(t *testing.T) {
	a := freshRig(t, "backup_max_age: 48h\n", 90*time.Minute, 26*time.Hour)
	f := a.BackupFreshness(govNow, os.Stderr)

	if !f.Armed || f.Stale {
		t.Fatalf("armed=%v stale=%v, want an armed and fresh instance", f.Armed, f.Stale)
	}
	if f.Count != 2 {
		t.Errorf("count = %d, want 2", f.Count)
	}
	line := f.Line()
	for _, want := range []string{"backup ·", "1h30m", "2 on box", AbbrevHome(f.Dir)} {
		if !strings.Contains(line, want) {
			t.Errorf("the line is missing %q:\n%s", want, line)
		}
	}
	// The NEWEST, not whichever the directory happened to list first.
	if !strings.Contains(line, govNow.Add(-90*time.Minute).UTC().Format(backupStamp)) {
		t.Errorf("the line does not name the newest archive:\n%s", line)
	}
	if strings.Contains(line, "STALE") {
		t.Errorf("a 1h30m backup under a 48h max reads as stale:\n%s", line)
	}
}

// The age comes from the archive's NAME, which is the manifest timestamp
// written at publish — not from an mtime, which a copy, a touch or a restore
// moves. This is the pin on that choice: an old archive whose mtime is now
// is still old.
func TestBackupAgeIsTheStampNotTheMtime(t *testing.T) {
	a := freshRig(t, "backup_max_age: 12h\n", 30*time.Hour)
	newest := filepath.Join(a.BackupDir(), backupPrefix+govNow.Add(-30*time.Hour).UTC().Format(backupStamp)+backupSuffix)
	now := time.Now()
	if err := os.Chtimes(newest, now, now); err != nil {
		t.Fatal(err)
	}
	f := a.BackupFreshness(govNow, os.Stderr)
	if !f.Stale || f.Age < 29*time.Hour {
		t.Fatalf("age = %s, stale = %v — a touched mtime hid a 30h-old archive", f.Age, f.Stale)
	}
}

// ─── the loud half, and the tenth-row ruling ─────────────────────────────────

// Past the max, the fact reaches the one computation every rendering shares.
// LANE and not URGENT is deliberate and is pinned: ADR 0029 defines URGENT
// as "the shop is stopped", and a stale backup stops nothing — spending the
// one class that means stop-everything on an overdue duty is what makes a
// surface stop being read.
func TestStaleBackupRaisesACarryOverNotATenthGRow(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "backup_max_age: 12h\n")
	stamp := govNow.Add(-30 * time.Hour).UTC().Format(backupStamp)
	write(t, filepath.Join(b.App.BackupDir(), backupPrefix+stamp+backupSuffix), "x\n")

	set := shopSet(t, govIn(t, b))
	var row *GovCondition
	for i := range set {
		if set[i].Key == "backup-stale" {
			row = &set[i]
		}
	}
	if row == nil {
		t.Fatalf("a 30h-old backup under a 12h max raised nothing: %v", set.Keys())
	}
	// The ruling, pinned exactly: no G-row. 0036 §6 asked for the fact and
	// not for a number, so this is a carry-over — empty ID, rendered "—" —
	// exactly like `unpushed:` and `no-live:`.
	if row.ID != "" || row.Row() != "—" {
		t.Errorf("the backup row is %q/%q — ADR 0036 §6 asked for the fact on the surface, not for a number", row.ID, row.Row())
	}
	if row.Class != GovLane {
		t.Errorf("class = %s, want %s: URGENT means the shop is stopped (ADR 0029 §1) and a stale backup stops nothing", row.Class, GovLane)
	}
	if !strings.Contains(row.Detail, "backup_max_age") {
		t.Errorf("the row does not name the threshold it tripped: %q", row.Detail)
	}
	// Loud is measured, not asserted: a non-empty set is what makes `posse
	// status` exit non-zero and what draws the cockpit's GOVERNANCE block.
	if len(set) == 0 {
		t.Fatal("the set is empty, so status would exit 0 over a stale backup")
	}

	// The control arm: with a fresh archive in the same directory the row is
	// gone. Without it, a row that fires unconditionally passes every
	// assertion above.
	write(t, filepath.Join(b.App.BackupDir(), backupPrefix+govNow.Add(-time.Hour).UTC().Format(backupStamp)+backupSuffix), "x\n")
	if keys := shopKeys(t, govIn(t, b)); containsStr(keys, "backup-stale") {
		t.Errorf("a 1h-old backup still reads as stale: %v", keys)
	}
}

// Armed and EMPTY is the predecessor's exact failure — the arrangement that
// was configured and never ran (ADR 0036 Context: the plist nobody
// installed). It must not read as "nothing to report".
func TestArmedWithNoArchiveIsStale(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "backup_max_age: 12h\n")

	f := b.App.BackupFreshness(govNow, os.Stderr)
	if !f.Armed || !f.Stale || f.Count != 0 {
		t.Fatalf("armed=%v stale=%v count=%d — a configured instance with no archive is the failure this row exists for", f.Armed, f.Stale, f.Count)
	}
	if !strings.Contains(f.Line(), "NONE on box") {
		t.Errorf("the line does not say there is no backup: %s", f.Line())
	}
	if keys := shopKeys(t, govIn(t, b)); !containsStr(keys, "backup-stale") {
		t.Errorf("conditions = %v, want backup-stale", keys)
	}
}

// And the other direction, which is the rule every optional surface in this
// harness keeps: installing posse arms nothing. An instance that has never
// written a backup key and holds no archive says nothing at all — no line,
// no condition, and `posse status` stays green.
func TestUnarmedInstanceSaysNothingAboutBackups(t *testing.T) {
	b, _ := newTestBackend(t)
	f := b.App.BackupFreshness(govNow, os.Stderr)
	if f.Armed {
		t.Fatalf("a home with no backup key and no archive reads as armed: %+v", f)
	}
	if keys := shopKeys(t, govIn(t, b)); containsStr(keys, "backup-stale") {
		t.Errorf("an unarmed instance raised a backup condition: %v", keys)
	}
	// A hand-typed run arms it without a config key: the archive on disk is
	// itself the opt-in, and from then on its age is reported.
	write(t, filepath.Join(b.App.BackupDir(), backupPrefix+govNow.Add(-time.Hour).UTC().Format(backupStamp)+backupSuffix), "x\n")
	if f := b.App.BackupFreshness(govNow, os.Stderr); !f.Armed {
		t.Errorf("an archive on disk did not arm the reading: %+v", f)
	}
}

// The default is the ADR's, and it is a number an operator can find: 48h,
// which is 2x the cadence the predecessor actually ran at (ADR 0036 §6 sets
// the threshold at 2x the interval; §4's interval is unbuilt).
func TestBackupMaxAgeDefaultAndTypo(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, "")
	if got := a.BackupMaxAge(os.Stderr); got != DefaultBackupMaxAge {
		t.Errorf("default max age = %s, want %s", got, DefaultBackupMaxAge)
	}
	// A typo is named out loud and the default stands — a threshold nobody
	// can see is worse than a wrong one (the rule attn_question_age keeps).
	var say strings.Builder
	write(t, a.ConfigPath, "backup_max_age: forty-eight hours\n")
	if got := a.BackupMaxAge(&say); got != DefaultBackupMaxAge {
		t.Errorf("a malformed max age read as %s", got)
	}
	if !strings.Contains(say.String(), "backup_max_age") {
		t.Errorf("a malformed max age was swallowed: %q", say.String())
	}
}
