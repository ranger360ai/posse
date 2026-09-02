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
func (d *Dispatcher) backupLoop(ctx context.Context, cfg BackupConfig) {
	if ctx.Err() != nil {
		return
	}
	d.backupTick(cfg)
	ticker := time.NewTicker(cfg.Interval)
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
func (d *Dispatcher) backupTick(cfg BackupConfig) {
	dir := d.App.BackupDir()
	names, err := listBackups(dir)
	if err != nil {
		d.eprintf("backup: cannot read %s: %v — no archive this tick\n", AbbrevHome(dir), err)
		return
	}
	if len(names) > 0 {
		newest := names[len(names)-1]
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
			d.printf("   %s\n", line)
		}
	}
	if err != nil {
		d.eprintf("backup: scheduled archive failed: %v\n", err)
		return
	}
	d.printf("backup · scheduled · %s · next in %s\n", AbbrevHome(res.Archive), cfg.Interval)
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
