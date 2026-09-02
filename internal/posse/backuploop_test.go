package posse

// ADR 0036 §4's ticker (bead ranger-base-zv3y6): the `backup_interval:` key
// and the goroutine it arms, together.
//
// The bead's three "done when" clauses are the three integration tests at the
// bottom of this file — an armed watch loop writes an archive, a restarted
// loop makes no second one inside the interval, and `posse pause` does not
// stop it. Everything above them is the config surface those three ride on.
//
// Two clocks, on purpose. The TICKER runs on real time (a Go ticker does),
// so the loop tests use intervals in the tens of milliseconds; the LEVEL —
// how old the newest archive is — is read through the dispatcher's own
// clock, so the tests that measure the level freeze it. Freezing is not a
// convenience here: an archive's name carries a stamp of one-second
// granularity, so a real-clock age against a sub-second interval is
// dominated by where in the second the run happened to land, and a
// restart test written on it would pass or fail by rounding.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// backupClockQueue is a scratch queue repo: a git checkout with a .beads
// store, which is everything RunBackup asks of a source. It is backupRig's
// queue half, lifted out so a loop test can attach one to a herdr-backed
// App instead of to backupRig's own bare home.
func backupClockQueue(t *testing.T) string {
	t.Helper()
	queue := filepath.Join(t.TempDir(), "queue")
	store := filepath.Join(queue, ".beads")
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	mustGit(t, queue, "init", "-q", "-b", "main", ".")
	mustGit(t, queue, "config", "user.email", "t@example.com")
	mustGit(t, queue, "config", "user.name", "t")
	write(t, filepath.Join(store, beadsJSONL), `{"id":"x-1","title":"one"}`+"\n")
	mustSqlite(t, filepath.Join(store, "beads.db"), "create table issues(id text); insert into issues values('x-1');")
	mustGit(t, queue, "add", "--", filepath.Join(".beads", beadsJSONL))
	mustGit(t, queue, "commit", "-q", "-m", "seed", "--", filepath.Join(".beads", beadsJSONL))
	return queue
}

// archives is every published archive in the app's backup dir. It goes
// through listBackups deliberately: a `.part` is a run in flight and must
// never count as an archive that exists, and a test that read the directory
// itself would count one.
func archives(t *testing.T, a *App) []string {
	t.Helper()
	names, err := listBackups(a.BackupDir())
	if err != nil {
		t.Fatalf("listing %s: %v", a.BackupDir(), err)
	}
	return names
}

// ─── the key ─────────────────────────────────────────────────────────────────

// No key is disarmed, and it is not an error: installing a posse that knows
// how to back up must not start a clock nobody asked for.
func TestLoadBackupConfigUnarmedWithoutTheKey(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, "queue_repo: /tmp/nope\n")
	cfg, err := LoadBackupConfig(a)
	if err != nil {
		t.Fatalf("no backup_interval: must not be an error: %v", err)
	}
	if cfg.Armed || cfg.Interval != 0 {
		t.Errorf("no backup_interval: must not arm the clock, got %+v", cfg)
	}
}

func TestLoadBackupConfigArmedAndBad(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, "backup_interval: 6h\n")
	cfg, err := LoadBackupConfig(a)
	if err != nil {
		t.Fatalf("backup_interval: 6h: %v", err)
	}
	if !cfg.Armed || cfg.Interval != 6*time.Hour {
		t.Errorf("backup_interval: 6h = %+v, want armed at 6h", cfg)
	}
	// Bare seconds, ParseInterval's other spelling — the same grammar
	// pulse_interval: takes, so an operator learns one.
	write(t, a.ConfigPath, "backup_interval: 900\n")
	if cfg, err := LoadBackupConfig(a); err != nil || cfg.Interval != 15*time.Minute {
		t.Errorf("backup_interval: 900 = %+v, %v; want 15m", cfg, err)
	}
	// Present and unparseable is an ERROR, not a silent disarm: the
	// operator asked for a schedule and must hear that they did not get one.
	write(t, a.ConfigPath, "backup_interval: nightly\n")
	if _, err := LoadBackupConfig(a); err == nil {
		t.Error("a bad backup_interval: must error, not silently disarm or default")
	}
}

// The key arms the freshness reading too. An instance that has said how
// often it wants an archive and holds none is the predecessor's exact
// failure — configured, never ran — and must not read as an instance that
// never asked (ADR 0036 §6, "armed and EMPTY is stale").
func TestBackupIntervalArmsTheFreshnessReading(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, "backup_interval: 6h\n")
	if !a.BackupConfigured() {
		t.Fatal("backup_interval: alone must arm the freshness reading")
	}
	f := a.BackupFreshness(time.Now(), os.Stderr)
	if !f.Armed || !f.Stale {
		t.Errorf("armed and empty must read stale, got %+v", f)
	}
}

// ADR 0036 §6: the default threshold is 2x the interval, and only the
// no-schedule instance falls back to 48h. An explicit backup_max_age: still
// wins over both — it is its own key and says only what it means.
func TestBackupMaxAgeDefaultsToTwiceTheInterval(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	for _, tc := range []struct {
		cfg  string
		want time.Duration
	}{
		{"queue_repo: /tmp/q\n", DefaultBackupMaxAge},
		{"backup_interval: 6h\n", 12 * time.Hour},
		{"backup_interval: 900\n", 30 * time.Minute},
		// A schedule the operator wrote and posse cannot read is not a
		// threshold: 48h, and the complaint belongs to the watch loop.
		{"backup_interval: nightly\n", DefaultBackupMaxAge},
		// The explicit key outranks the derived default in both directions.
		{"backup_interval: 6h\nbackup_max_age: 30m\n", 30 * time.Minute},
		{"backup_max_age: 30m\n", 30 * time.Minute},
	} {
		write(t, a.ConfigPath, tc.cfg)
		if got := a.BackupMaxAge(os.Stderr); got != tc.want {
			t.Errorf("config %q: max age = %s, want %s", tc.cfg, got, tc.want)
		}
	}
}

// The status line names the key either way. A stale age the operator cannot
// explain is a stale age plus a question; this is the answer.
func TestBackupScheduleLineNamesTheKey(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, "queue_repo: /tmp/q\n")
	if got := a.BackupScheduleLine(); !strings.Contains(got, "backup_interval:") || !strings.Contains(got, "none") {
		t.Errorf("unarmed schedule line = %q, want it to name the unset key", got)
	}
	write(t, a.ConfigPath, "backup_interval: 6h\n")
	if got := a.BackupScheduleLine(); !strings.Contains(got, "6h") || !strings.Contains(got, "watch") {
		t.Errorf("armed schedule line = %q, want the cadence and where it runs", got)
	}
	write(t, a.ConfigPath, "backup_interval: nightly\n")
	if got := a.BackupScheduleLine(); !strings.Contains(got, "nothing is scheduled") {
		t.Errorf("broken schedule line = %q, want it to say nothing is scheduled", got)
	}
}

// ─── the level trigger ───────────────────────────────────────────────────────

// backupLoopRig is a dispatcher whose App has a scratch queue and a frozen
// clock, ready to run backupLoop directly. Direct is the point for the level
// tests: Watch's other machinery (a pass, the settle subscription, the hook
// wall sweep) is measured elsewhere and only adds ways for these to flake.
func backupLoopRig(t *testing.T, interval string) (*Dispatcher, *App, *time.Time) {
	t.Helper()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	write(t, b.App.ConfigPath, "queue_repo: "+backupClockQueue(t)+"\nbackup_interval: "+interval+"\n")
	at := time.Date(2026, 9, 1, 3, 15, 0, 0, time.UTC)
	d.Now = func() time.Time { return at }
	return d, b.App, &at
}

// The first tick of an armed loop, on a directory with nothing in it, writes
// an archive: an empty level is the oldest level there is.
func TestBackupLoopWritesTheFirstArchive(t *testing.T) {
	d, a, _ := backupLoopRig(t, "50ms")
	cfg, err := LoadBackupConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	d.backupTick(cfg)
	if got := archives(t, a); len(got) != 1 {
		t.Fatalf("one tick on an empty directory wrote %v, want exactly one archive", got)
	}
	// Published means verified (ADR 0036 §3): the archive is only named
	// after it has been read back, so opening it here is not a second
	// check, it is this test refusing to call a filename a backup.
	if _, err := VerifyBackup(filepath.Join(a.BackupDir(), archives(t, a)[0])); err != nil {
		t.Fatalf("the scheduled archive does not verify: %v", err)
	}
}

// The level, not the edge. Ten ticks inside one interval make ONE archive —
// the trigger reads the directory, and the directory has not aged.
func TestBackupLoopIsLevelTriggeredNotEdgeTriggered(t *testing.T) {
	d, a, _ := backupLoopRig(t, "1h")
	cfg, _ := LoadBackupConfig(a)
	for i := 0; i < 10; i++ {
		d.backupTick(cfg)
	}
	if got := archives(t, a); len(got) != 1 {
		t.Fatalf("ten ticks inside one interval wrote %d archives (%v), want one", len(got), got)
	}
}

// ...and it does age. Move the clock past the interval and the next tick
// runs: a level trigger that can never fire twice is a latch, not a clock.
func TestBackupLoopRunsAgainOnceTheIntervalHasPassed(t *testing.T) {
	d, a, at := backupLoopRig(t, "1h")
	cfg, _ := LoadBackupConfig(a)
	d.backupTick(cfg)
	*at = at.Add(90 * time.Minute)
	d.backupTick(cfg)
	if got := archives(t, a); len(got) != 2 {
		t.Fatalf("a tick 90m into a 1h interval left %d archives (%v), want two", len(got), got)
	}
}

// runBackupLoop is one whole backupLoop lifetime: start it, wait for it to
// reach want archives (or burn grace waiting), stop it, and JOIN it. The
// join is not tidiness — an unjoined tick goes on staging tens of megabytes
// under a t.TempDir that the test framework is about to remove.
//
// It returns how long the wait actually took, which is what lets an absence
// arm below say something instead of merely waiting: a window that is longer
// than the window a real write needed is a window that could have caught one.
func runBackupLoop(t *testing.T, d *Dispatcher, a *App, cfg BackupConfig, want int, grace time.Duration) time.Duration {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); d.backupLoop(ctx, cfg) }()
	start := time.Now()
	deadline := time.After(grace)
	for len(archives(t, a)) < want {
		select {
		case <-deadline:
			cancel()
			<-done
			return time.Since(start)
		case <-time.After(5 * time.Millisecond):
		}
	}
	took := time.Since(start)
	cancel()
	<-done
	return took
}

// The bead's second clause, and the reason §4 says level-triggered at all: a
// loop that goes away and comes back does not re-run the duty.
//
// The interval is an hour, so the TICKER cannot fire inside this test at
// all: everything any of these three loop lifetimes writes, it writes in the
// evaluation it makes at start. That is what makes this a restart test and
// not a ticker test.
//
// The middle arm is the control, and without it the last arm measures
// nothing: it moves the clock past the interval and watches a restarted loop
// write a second archive — through the same helper, the same goroutine, the
// same poll — so the silence in the third arm is a loop that DECLINED rather
// than a loop nobody waited for.
func TestBackupLoopRestartMakesNoSecondArchiveInsideTheInterval(t *testing.T) {
	d, a, at := backupLoopRig(t, "1h")
	cfg, _ := LoadBackupConfig(a)

	// Arm 1: a cold start writes one.
	runBackupLoop(t, d, a, cfg, 1, 60*time.Second)
	if got := archives(t, a); len(got) != 1 {
		t.Fatalf("the first loop wrote %v, want one archive", got)
	}

	// Arm 2, the control: 90 minutes into a 1h interval, a RESTARTED loop
	// writes on its start evaluation. This is how long a write takes to
	// become visible in this rig.
	*at = at.Add(90 * time.Minute)
	took := runBackupLoop(t, d, a, cfg, 2, 60*time.Second)
	if got := archives(t, a); len(got) != 2 {
		t.Fatalf("a restart past the interval wrote %v, want two archives", got)
	}

	// Arm 3: the clock does not move, so the archive arm 2 just wrote is
	// fresh — and two more whole loop lifetimes write nothing. The window
	// each one gets is a dead wait in a package that is already twenty
	// minutes long, so it is set from the control rather than generously:
	// arm 2's write became visible in `took`, and the assertion below is
	// what refuses to let this shrink under it.
	const grace = 2 * time.Second
	for i := 0; i < 2; i++ {
		runBackupLoop(t, d, a, cfg, 3, grace)
	}
	if got := archives(t, a); len(got) != 2 {
		t.Fatalf("two restarts inside the interval wrote %d archives (%v), want the two from before", len(got), got)
	}
	if took >= grace {
		t.Fatalf("the control write took %s and the absence arms waited %s — the absence arms measured nothing", took, grace)
	}
}

// A failing run is a line and a next tick, never a stopped loop: this is a
// timer, and a store that could not be read at 03:15 is not a reason to
// stop the fleet.
func TestBackupLoopSurvivesARefusal(t *testing.T) {
	d, a, _ := backupLoopRig(t, "1h")
	cfg, _ := LoadBackupConfig(a)
	say := dispatcherErr(t, d)
	// The source refusal (ADR 0036 §3, the 2c ruling): a queue repo that
	// grew a remote is refused before anything is read from it.
	mustGit(t, a.QueueRepo(), "remote", "add", "origin", "https://example.invalid/q.git")
	d.backupTick(cfg)
	if got := archives(t, a); len(got) != 0 {
		t.Fatalf("a refused run published %v", got)
	}
	if !strings.Contains(say.String(), "scheduled archive failed") {
		t.Errorf("a refused scheduled run must say so, got:\n%s", say.String())
	}
	mustGit(t, a.QueueRepo(), "remote", "remove", "origin")
	d.backupTick(cfg)
	if got := archives(t, a); len(got) != 1 {
		t.Fatalf("the tick after the refusal cleared wrote %v, want one archive", got)
	}
}

// ─── inside the watch loop ───────────────────────────────────────────────────

// The bead's first clause, through the real Watch: an armed loop writes an
// archive and nobody typed the command.
//
// The interval is 2s and the archive arrives sooner than that, which is the
// level trigger and not a lucky race — the loop reads the level at start, so
// on a directory with nothing in it the first archive is due before the
// first tick. The bead asks for one "within two ticks"; this is inside zero,
// and a short interval is deliberately NOT used here: at second-granularity
// stamps a sub-second interval makes every tick due, and the test would be
// measuring how many archives a burst can write rather than that one does.
func TestWatchBackupClockWritesAnArchive(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	write(t, b.App.ConfigPath, "queue_repo: "+backupClockQueue(t)+"\nbackup_interval: 2s\nbeads:\n  - "+repo+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	deadline := time.After(60 * time.Second)
	for len(archives(t, b.App)) == 0 {
		select {
		case <-deadline:
			t.Fatalf("the watch loop never wrote an archive:\n%s", dispatcherOut(d))
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("watch never returned after cancel")
	}
	if out := dispatcherOut(d); !strings.Contains(out, "backup · scheduled ·") {
		t.Errorf("the scheduled archive must be visible in the watch log:\n%s", out)
	}
}

// Disarmed starts nothing at all. No key, no goroutine, no directory —
// installing posse arms nothing, and that rule is the same one queue_repo:
// keeps.
func TestWatchBackupUnarmedStartsNoClock(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	// A queue_repo: it COULD back up, and no backup_interval: to say so.
	write(t, b.App.ConfigPath, "queue_repo: "+backupClockQueue(t)+"\nbeads:\n  - "+repo+"\n")

	const wantPasses = 3
	tap := newPassTap(wantPasses)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond); done <- p }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("watch never returned")
	}
	if got := archives(t, b.App); len(got) != 0 {
		t.Errorf("an unarmed watch loop wrote %v", got)
	}
	if _, err := os.Stat(b.App.BackupDir()); err == nil {
		t.Errorf("an unarmed watch loop must not even create %s", b.App.BackupDir())
	}
	if strings.Contains(tap.String(), "backup · scheduled") {
		t.Errorf("an unarmed watch loop must never log a scheduled backup:\n%s", tap.String())
	}
}

// A backup_interval: posse cannot read disarms THIS loop and says so. It
// does not wedge the watch, and it does not stop dispatching: a broken
// backup cadence is not a reason to stop the fleet.
func TestWatchBackupBadIntervalDisarmsRatherThanWedges(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	write(t, b.App.ConfigPath, "queue_repo: "+backupClockQueue(t)+"\nbackup_interval: nightly\nbeads:\n  - "+repo+"\n")
	say := dispatcherErr(t, d)

	tap := newPassTap(2)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond); done <- p }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("watch never returned: a bad backup_interval: must disarm the clock, not wedge the loop")
	}
	if got := archives(t, b.App); len(got) != 0 {
		t.Errorf("a broken interval must schedule nothing, wrote %v", got)
	}
	if got := say.String(); !strings.Contains(got, "backup_interval") || !strings.Contains(got, "disarmed") {
		t.Errorf("a broken backup_interval: must be said out loud, got:\n%s", got)
	}
}

// The bead's third clause, and ADR 0036 §4 in as many words: `posse pause`
// stops DISPATCHING, and the queue still mutates in a paused shop. A paused
// fleet whose store went unbacked-up is the failure this verb exists for.
func TestBackupLoopRunsWhileThePassIsPaused(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	write(t, b.App.ConfigPath, "queue_repo: "+backupClockQueue(t)+"\nbackup_interval: 2s\nbeads:\n  - "+repo+"\n")
	if _, err := WritePause(b.App, "operator", "measuring the backup clock", time.Now(), os.Stderr); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	deadline := time.After(60 * time.Second)
	for len(archives(t, b.App)) == 0 {
		select {
		case <-deadline:
			t.Fatalf("a paused shop must still back up; the loop wrote nothing:\n%s", dispatcherOut(d))
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("watch never returned after cancel")
	}
	// The control arm: the pause was real and the pass DID decline. Without
	// this the test above is green on a pause that never took effect, which
	// would measure nothing at all.
	if !ReadPause(PausePath(b.App)).Present {
		t.Fatal("the pause file went away — this test measured an unpaused shop")
	}
	if out := dispatcherOut(d); !strings.Contains(strings.ToLower(out), "pause") {
		t.Errorf("the pass must have declined for the pause; log:\n%s", out)
	}
}
