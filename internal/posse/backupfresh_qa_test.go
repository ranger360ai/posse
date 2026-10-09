//go:build posse_arm3

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
	"fmt"
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
	// `queue_repo:` is part of the rig, not of any case: an instance with no
	// store of record has nothing to archive and reads as such (beads
	// ranger-base-0q7rp, ranger-base-00a5l), so every AGE reading below
	// presumes a store exists. A real one: a path that merely EXISTS is a
	// store-of-record gap too, which is the hole ranger-base-00a5l closed,
	// and a rig built on one would carry a no-store clause into every age
	// reading here. The no-store arms have their own rig.
	write(t, a.ConfigPath, config+"queue_repo: "+freshQueue(t)+"\n")
	dir := a.BackupDir()
	for _, age := range ages {
		name := backupPrefix + govNow.Add(-age).UTC().Format(backupStamp) + backupSuffix
		write(t, filepath.Join(dir, name), "not a real archive, and this reader never opens one\n")
	}
	return a
}

// freshQueue is the cheapest thing that answers all three of RunBackup's
// store-of-record questions (backupStoreGap): a git checkout with a `.beads`
// directory. It holds no beads db, because nothing in this file runs a
// backup to completion — that is backupClockQueue's job, and it needs
// sqlite3 on the box.
func freshQueue(t *testing.T) string {
	t.Helper()
	q := filepath.Join(gitTempDir(t), "queue")
	if err := os.MkdirAll(filepath.Join(q, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustGit(t, q, "init", "-q", "-b", "main", ".")
	return q
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
	appendConfig(t, b.App, "backup_max_age: 12h\nqueue_repo: "+freshQueue(t)+"\n")
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
	appendConfig(t, b.App, "backup_max_age: 12h\nqueue_repo: "+freshQueue(t)+"\n")

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

// ─── and the store the row is about (beads ranger-base-0q7rp,
// ranger-base-00a5l) ────────────────────────────────────────────────────────

// Armed, empty, and NO STORE OF RECORD. The row used to say "no backup of
// the store of record on this box — <dir> is empty", whose only remedy is
// `posse backup`, and that verb answers that there is nothing to back up:
// two surfaces disagreeing about whether a store of record exists, on the
// same config. ADR 0036 decides the verb's half ("refuse an unset
// queue_repo"), so the ROW is what moves — it names the state, and both say
// it in the same words, from one place.
//
// All THREE of the verb's store-of-record refusals, which is
// ranger-base-00a5l's half: ranger-base-0q7rp's complaint was general and
// its fix keyed on the unset key alone, so the other two reproduced the
// original symptom one config VALUE over — a `queue_repo:` naming a plain
// directory, which is the state that close's own control fixtures sat in.
func TestNoStoreOfRecordRowAgreesWithTheVerb(t *testing.T) {
	for _, c := range []struct {
		name  string
		queue func(t *testing.T) string
		// what the gap must be ABOUT, beyond the words both surfaces share
		says string
	}{
		{"queue_repo: unset", func(t *testing.T) string { return "" }, "is unset"},
		{"queue_repo: names a path with no beads store", func(t *testing.T) string {
			return gitTempDir(t)
		}, "has no beads store at"},
		{"queue_repo: names a beads store that is not a checkout", func(t *testing.T) string {
			q := filepath.Join(gitTempDir(t), "queue")
			if err := os.MkdirAll(filepath.Join(q, ".beads"), 0o700); err != nil {
				t.Fatal(err)
			}
			return q
		}, "is not a git repository"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			cfg := "backup_max_age: 12h\nbackup_interval: 6h\n"
			if q := c.queue(t); q != "" {
				cfg += "queue_repo: " + q + "\n"
			}
			appendConfig(t, b.App, cfg)

			f := b.App.BackupFreshness(govNow, os.Stderr)
			if !f.Armed || !f.NoStore || !f.Stale {
				t.Fatalf("armed=%v nostore=%v stale=%v — an armed arrangement that cannot run is still a condition", f.Armed, f.NoStore, f.Stale)
			}
			if !strings.Contains(f.NoStoreWhy, c.says) {
				t.Errorf("the reading does not say WHICH gap it is: %q, want %q in it", f.NoStoreWhy, c.says)
			}
			// The verb, on the same instance: the sentence the operator
			// meets if they follow the row. Compared against the reading's
			// own words, not against a constant — a constant can agree with
			// one of the three and leave the others disagreeing, which is
			// exactly how this escaped (bead ranger-base-00a5l).
			_, err := b.App.RunBackup(BackupOpts{Now: func() time.Time { return govNow }})
			if err == nil || !strings.Contains(err.Error(), f.NoStoreWhy) {
				t.Fatalf("posse backup = %v, want the refusal the row reports (%q)", err, f.NoStoreWhy)
			}
			// The row says that same thing, and names a remedy that works.
			var row *GovCondition
			set := shopSet(t, govIn(t, b))
			for i := range set {
				if set[i].Key == "backup-stale" {
					row = &set[i]
				}
			}
			if row == nil {
				t.Fatalf("an armed backup arrangement that cannot run raised nothing: %v", set.Keys())
			}
			if !strings.Contains(row.Detail, f.NoStoreWhy) {
				t.Errorf("the row and the verb disagree.\nrow:  %s\nverb: %s", row.Detail, err)
			}
			if !strings.Contains(row.Detail, "queue_repo:") || !strings.Contains(row.Detail, "backup_*") {
				t.Errorf("the row does not name a remedy that works: %q", row.Detail)
			}
			// And it no longer sends the operator to the verb that refuses.
			if strings.Contains(row.Detail, "no backup of the store of record on this box") {
				t.Errorf("the row still reports the empty directory as the condition: %q", row.Detail)
			}
			// The archive directory is still the EVIDENCE beside the
			// subject (storelessReading). Pinned because nothing did:
			// stubbing that helper to "" survived every test in the tree
			// and left the row reading "…nothing to back up — : point
			// queue_repo: …" (bead ranger-base-5ayqc, finding 3).
			if !strings.Contains(row.Detail, AbbrevHome(f.Dir)+" holds no archive") {
				t.Errorf("the row drops the directory evidence: %q", row.Detail)
			}
			// The quiet line carries the sentence too: `posse backup status`
			// exits non-zero over this reading, and its other lines can
			// both be silent about the state.
			if !strings.Contains(f.Line(), f.NoStoreWhy) {
				t.Errorf("the freshness line does not say why the directory is empty:\n%s", f.Line())
			}
			// And the third surface, the one this bead's finding 2 found
			// WORSE than before: an armed clock whose every tick refuses
			// must not promise a cadence it cannot keep.
			sched := b.App.BackupScheduleLine()
			if !strings.Contains(sched, "refuses") || !strings.Contains(sched, b.App.backupStoreGap()) {
				t.Errorf("the schedule line promises a cadence nothing can keep: %q", sched)
			}
			// SHORT, not the whole sentence: the freshness line directly
			// above it in `posse backup status` already carries that.
			if strings.Contains(sched, "nothing to back up") {
				t.Errorf("the schedule line repeats the freshness line's whole sentence:\n%s", sched)
			}
		})
	}
}

// The CONTROL, with a real store of record and the directory still empty:
// the condition goes back to being the missing archive, in its own words,
// and the verb stops refusing for this reason. Without this arm a row
// hard-wired to the no-store sentence passes everything above.
func TestWithAStoreOfRecordTheRowIsAboutTheArchive(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "backup_max_age: 12h\nbackup_interval: 6h\nqueue_repo: "+freshQueue(t)+"\n")

	g := b.App.BackupFreshness(govNow, os.Stderr)
	if g.NoStore || !g.Stale {
		t.Fatalf("nostore=%v stale=%v with a real store over an empty directory", g.NoStore, g.Stale)
	}
	if d := g.GovDetail(); !strings.Contains(d, "is empty") || strings.Contains(d, "queue_repo") {
		t.Errorf("the row on an instance WITH a store still talks about the key: %q", d)
	}
	if l := g.Line(); strings.Contains(l, "nothing to back up") {
		t.Errorf("the quiet line carries a no-store clause over a real store:\n%s", l)
	}
	if s := b.App.BackupScheduleLine(); strings.Contains(s, "refuses") {
		t.Errorf("an instance with a store was told its ticks refuse: %q", s)
	}
	if _, err := b.App.RunBackup(BackupOpts{Now: func() time.Time { return govNow }}); err == nil || strings.Contains(err.Error(), "nothing to back up") {
		t.Errorf("the verb still refuses for want of a store it was given: %v", err)
	}
}

// storelessReading's other two arms, which the row reaches through the same
// branch and which no test named either (bead ranger-base-5ayqc, finding 3).
// The `default` arm was suspected unreachable through GovDetail — the row
// fires on Stale, and Stale over a DATABLE archive is a reachable state, so
// it is reachable and this is the pin that says so.
func TestTheNoStoreRowReportsWhateverTheArchiveDirectoryHolds(t *testing.T) {
	for _, c := range []struct {
		name string
		age  time.Duration // negative is a stamp AHEAD of the clock
		want string
	}{
		{"nothing on disk", 0, "%s holds no archive"},
		{"only a stamp ahead of the clock", -2 * time.Hour, "%s holds 1 archive(s), none of them a usable reading"},
		{"a datable archive, past the max", 30 * time.Hour, "the newest archive in %s is 30h00m old"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			appendConfig(t, b.App, "backup_max_age: 12h\n")
			if c.age != 0 {
				stamp := govNow.Add(-c.age).UTC().Format(backupStamp)
				write(t, filepath.Join(b.App.BackupDir(), backupPrefix+stamp+backupSuffix), "x\n")
			}
			f := b.App.BackupFreshness(govNow, os.Stderr)
			if !f.NoStore || !f.Stale {
				t.Fatalf("nostore=%v stale=%v — the row must fire for this arm to be reachable at all", f.NoStore, f.Stale)
			}
			want := fmt.Sprintf(c.want, AbbrevHome(f.Dir))
			if d := f.GovDetail(); !strings.Contains(d, want) {
				t.Errorf("the row's evidence clause is missing %q:\n%s", want, d)
			}
		})
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

// ─── the second and third store-of-record refusals (bead ranger-base-00a5l)
// ──────────────────────────────────────────────────────────────────────────
//
// The pin that used to sit here shipped GREEN over a hole:
// ranger-base-0q7rp's complaint was general — "the governance row and the
// command disagree about whether a store of record exists" — and the close
// keyed its whole fix on ONE of the three store-of-record refusals
// `RunBackup` can make before it writes anything, `queue_repo:` unset. The
// other two survived one config VALUE over, in the state the close's own
// control fixtures sat in (`queue_repo: `+t.TempDir() — a directory with no
// `.beads`), and the schedule line was WORSE than the state the bead filed:
// it promised "every 6h00m, from the dispatch --watch loop" with no clause
// at all.
//
// The agreement is asserted in TestNoStoreOfRecordRowAgreesWithTheVerb
// above, which now runs all three refusals as its own table and compares the
// row, the verb and the schedule line against the READING's words rather
// than against a constant — the shape that let a one-of-three fix pass.
// BackupFreshness.NoStore is backupStoreGap's answer, which is RunBackup's
// own three questions in RunBackup's own order.
