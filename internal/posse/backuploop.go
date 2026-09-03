package posse

// The backup's clock: a ticker goroutine inside `posse dispatch --watch`
// (ADR 0036 §4, bead ranger-base-zv3y6). The verb, its refusals, its
// verify-before-publish and its freshness surface shipped with
// ranger-base-a0ln0; nothing scheduled it, so an archive existed only when
// somebody typed the command — which is the exact shape of this record's own
// Context, the predecessor's plist that was never installed.
//
// §4 was deliberately not half-built by a0ln0: no `backup_interval:` key was
// defined, because a key that reads like a schedule and schedules nothing is
// that plist wearing a config key. So the key and the loop land together
// here, or not at all.
//
// The pattern is the pulse's, exactly (ADR 0027, pulse.go):
//
//	starts with Watch's ctx      one standing process owns the clock, and
//	                             the harness already has one. No second
//	                             daemon, no launchd job failing silently in
//	                             a log nobody opens.
//	dies with it, JOINED         a tick already inside RunBackup is writing
//	                             into this instance's state/, so a Watch
//	                             that returned on the cancel alone would
//	                             leave a goroutine writing after its caller
//	                             believed the loop was over (the pulse's own
//	                             ranger-base-el3g). Watch joins it.
//	disarmed unless armed        no `backup_interval:` in config starts no
//	                             goroutine at all. Installing posse arms
//	                             nothing.
//
// Two things this loop is NOT, and both are §4 in as many words:
//
//   - It is not the pass, the Run's return, or the epoch tick. ADR 0028
//     taught that a duty parked "on the pass" starves when a rolling Run
//     refills for hours (ranger-base-ad4y); this clock has its own goroutine
//     and its own lifetime.
//   - It is not gated by `posse pause`. Pause stops DISPATCHING, and the
//     queue still mutates in a paused shop — a paused fleet whose store went
//     unbacked-up is the failure this verb exists for. Nothing below reads
//     the pause file, and TestBackupLoopRunsWhileThePassIsPaused is what
//     keeps it that way.
//
// Level-triggered, never edge-triggered: every tick asks the archive
// DIRECTORY how old the newest archive is, not "did my ticker fire". A loop
// that restarts every five minutes under an interval of an hour therefore
// makes one archive an hour, not one per restart — the directory is the
// durable state, and it is the same single store §6 gives freshness.
//
// SAMPLED FASTER THAN IT FIRES (ranger-base-wj7e9). A level trigger read
// once per interval is a level trigger with the interval's own drift built
// into it, and this is the arithmetic that says so, from the box's own
// records on 2026-09-03:
//
//	state/dispatch-watch.pid     started 2026-09-03T01:53:52Z
//	state/backup/                newest  posse-backup-20260902T052114Z
//	config.yaml                  backup_interval: 24h
//
// At the start evaluation the archive was 20h32m old — under 24h, so the
// tick correctly did nothing. The next look was the ticker's, 24h later, at
// 01:53Z on 09-04, by which time the archive would have been 44h32m old. The
// duty due at 05:21Z was not late because the box slept through it: it was
// never looked at. (It was reported on this bead as a symptom of a 7h11m
// hang. There was no hang — that window was a sleep — and this is not a
// symptom of the sleep either: no backup tick fell inside 04:53Z-12:05Z at
// all. The sampling defect is the box's own arithmetic and would hold on a
// machine that never slept.)
//
// The reading is a directory listing, so taking it often is free, and the
// period it is taken at is the whole of the error: sampling every N makes
// the archive at most interval+N old instead of at most 2x interval. So the
// ticker runs at backupSampleEvery, and cfg.Interval decides only when the
// LEVEL is up — which is what "level-triggered" was supposed to mean.

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// BackupMaxAgeFactor is ADR 0036 §6's threshold rule: an instance that has
// armed the schedule and has not produced an archive in two of its own
// intervals has missed one and is stale. It is a factor and not a second
// key so that an operator who changes the cadence does not have to remember
// to change the alarm with it.
const BackupMaxAgeFactor = 2

// BackupConfig is the backup schedule, autostart_*-style flat YAML: the
// PRESENCE of `backup_interval:` is the arm switch, exactly as
// `pulse_interval:` is the pulse's.
type BackupConfig struct {
	Armed    bool
	Interval time.Duration
}

// LoadBackupConfig reads config.yaml's `backup_interval:`. Absent returns a
// zero BackupConfig and no error — an unset key is not a misconfiguration,
// it is the default, and it is what every deployer who has not asked for a
// schedule gets.
//
// A key that is present and unparseable IS an error, and the caller decides
// what that costs: Watch disarms this one loop and says so, rather than
// refusing to dispatch over a backup cadence.
func LoadBackupConfig(a *App) (BackupConfig, error) {
	if !yamlHasKey(a.ConfigPath, "backup_interval") {
		return BackupConfig{}, nil
	}
	interval, err := ParseInterval(a.CfgGet("backup_interval", ""))
	if err != nil {
		return BackupConfig{}, Die("config backup_interval: %v", err)
	}
	return BackupConfig{Armed: true, Interval: interval}, nil
}

// backupLoop runs the level trigger every cfg.Interval until ctx ends — the
// watch loop's own lifetime, never a second loop of its own.
//
// It evaluates ONCE before the first tick, and that is the level trigger
// taken seriously rather than an eager start: a loop that comes up next to a
// three-day-old archive under a one-day interval is looking at a duty that
// is already overdue, and waiting out a fresh interval before noticing would
// make the archive four days old. The same reading is what makes a restart
// cheap — a directory whose newest archive is younger than the interval is
// a tick that does nothing at all.
// The sampling cadence — see "SAMPLED FASTER THAN IT FIRES" above. The
// divisor keeps a short interval's sampling proportional; the cap keeps a
// long one's lateness bounded by a number an operator can name rather than
// by a fraction of a day; the floor keeps a fast test from spinning, and the
// clamp back to cfg.Interval keeps a sub-second interval behaving exactly as
// it did before this bead.
const (
	BackupSampleDivisor = 8
	BackupSampleMax     = 15 * time.Minute
	BackupSampleMin     = time.Second
)

// backupSampleEvery is how often the level is READ, which is not how often
// an archive is written: the write is still gated on cfg.Interval by
// backupTick, and a sample that finds the newest archive young enough costs
// one directory listing.
func backupSampleEvery(interval time.Duration) time.Duration {
	every := interval / BackupSampleDivisor
	if every > BackupSampleMax {
		every = BackupSampleMax
	}
	if every < BackupSampleMin {
		every = BackupSampleMin
	}
	if every > interval {
		every = interval
	}
	return every
}

func (d *Dispatcher) backupLoop(ctx context.Context, cfg BackupConfig) {
	if ctx.Err() != nil {
		return
	}
	d.backupTick(cfg)
	ticker := time.NewTicker(backupSampleEvery(cfg.Interval))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.backupTick(cfg)
		}
	}
}

// backupTick is one evaluation of the level: read the directory, and run the
// verb only if the newest published archive is older than the interval.
//
// Nothing here re-implements any part of the verb. RunBackup is the whole
// duty — the on-box refusal, the source's no-remote check, the disk floor,
// the flock, the staging, the verify-before-publish and the prune — and a
// scheduled run that skipped any of them would be a second, weaker backup
// path with the same name. A failure is a line and the next tick tries
// again: this is a timer, and a store that could not be read at 03:15 is not
// a reason to stop the fleet.
//
// It writes through the QUIET pair (dispatch.go, LastWrite): this is a clock
// on its own goroutine, and its lines are not the loop writing
// (ranger-base-0fz98 finding 3).
func (d *Dispatcher) backupTick(cfg BackupConfig) {
	dir := d.App.BackupDir()
	names, err := listBackups(dir)
	if err != nil {
		d.equietf("backup: cannot read %s: %v — no archive this tick\n", AbbrevHome(dir), err)
		return
	}
	// A stamp AFTER now is not a reading, and splitBackupsAt is where that
	// is decided once for this loop and for §6's freshness surface both
	// (bead ranger-base-rgv61). Before it, one archive stamped in the
	// future stopped this schedule for as long as the stamp led the clock:
	// the age below went negative, and a negative age is under every
	// interval. It said nothing while it did it, and the freshness line
	// reading the same stamp said "0s ago".
	usable, future := splitBackupsAt(names, d.now())
	if len(future) > 0 {
		// Every tick, for as long as it stands. This is a level trigger and
		// the condition is a level: a line printed once, hours ago, in a
		// scrollback nobody is watching is the silence this bead was about.
		newest := future[len(future)-1]
		d.equietf("backup: %s\n", backupFutureClause(len(future), newest, backupTimeOf(newest).Sub(d.now())))
	}
	if len(usable) > 0 {
		newest := usable[len(usable)-1]
		// backupTimeOf reads the archive's own stamp, which is the name it
		// was published under; listBackups has already dropped anything
		// whose stamp does not parse, so this age is never a zero time
		// masquerading as "infinitely old".
		if age := d.now().Sub(backupTimeOf(newest)); age < cfg.Interval {
			return
		}
	}
	var say strings.Builder
	res, err := d.App.RunBackup(BackupOpts{Out: &say, Now: d.now})
	// The verb's own narration first, whichever way it went: the disk-floor
	// refusal and the config complaints are written there, and losing them
	// on the error path is losing the reason.
	for _, line := range strings.Split(strings.TrimSuffix(say.String(), "\n"), "\n") {
		if line != "" {
			d.quietf("   %s\n", line)
		}
	}
	if err != nil {
		d.equietf("backup: scheduled archive failed: %v\n", err)
		return
	}
	d.quietf("backup · scheduled · %s · next in %s\n", AbbrevHome(res.Archive), cfg.Interval)
}

// BackupScheduleLine is what `posse backup status` says about the clock: one
// line, and it names the key either way. An operator reading a stale backup
// needs to know whether anything was ever supposed to write one.
func (a *App) BackupScheduleLine() string {
	cfg, err := LoadBackupConfig(a)
	switch {
	case err != nil:
		return fmt.Sprintf("  schedule · %v — nothing is scheduled", err)
	case !cfg.Armed:
		return "  schedule · none (config backup_interval: unset) — the verb runs when it is run"
	default:
		return fmt.Sprintf("  schedule · every %s, from the dispatch --watch loop (config backup_interval:)", BlindFor(cfg.Interval))
	}
}
