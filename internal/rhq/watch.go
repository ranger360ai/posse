package rhq

// posse dispatch --watch: passes in a loop. Pass, sleep, repeat; a quiet
// pass (nothing dispatched) doubles the sleep up to a cap, a busy one
// snaps it back to the base interval. The context ends the loop between
// passes — a pass in flight (prompt --wait) finishes first, so a persona
// mid-turn is never left with a half-run pass.
//
// The loop is also where "unattended" is defined: Watch is the only thing
// that sets Dispatcher.Unattended, which is what lets the plan guard fail
// closed after a bounded blind window (rangerhq-6h1). Not a TTY check — the
// premise is that a fail-open stderr line has a witness when a human typed
// the command and none when a timer did, and that is knowable here.

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// NextInterval is the backoff schedule: dispatched work resets to base;
// a quiet pass doubles the current interval, capped at max.
func NextInterval(cur, base, max time.Duration, dispatched int) time.Duration {
	if dispatched > 0 || cur <= 0 {
		return base
	}
	next := cur * 2
	if next > max {
		next = max
	}
	if next < base {
		next = base
	}
	return next
}

// ParseInterval accepts a Go duration ("30s", "2m") or bare seconds ("30").
func ParseInterval(s string) (time.Duration, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return 0, Die("interval must be positive: %s", s)
		}
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, Die("bad interval %q (use 30s, 2m, or seconds)", s)
	}
	return d, nil
}

// Watch runs passes until ctx is done. Returns the number of passes run.
// A pass error is reported and the loop continues (bd or herdr hiccups
// are transient by nature); only ctx ends it.
func (d *Dispatcher) Watch(ctx context.Context, dirFilter, personaFilter string, max int, base, maxInterval time.Duration) int {
	if maxInterval < base {
		maxInterval = base
	}
	// A timer typed this command, not a human: the plan guard's fail-open
	// line has no witness from here on, so blindness gets a clock
	// (rangerhq-6h1). Seeded at loop start, not at the epoch — the first
	// pass of a fresh loop gets the whole grace rather than an instant skip.
	d.Unattended = true
	d.blindSince = d.now()
	// Leave a liveness record for anything that would otherwise infer the
	// loop from the workspace — plugin/autostart.sh above all (rangerhq-ct9).
	defer d.dropWatchPid()
	d.stampWatchPid()
	passes := 0
	wait := base
	for {
		passes++
		fmt.Fprintf(d.Out, "── pass %d · %s\n", passes, time.Now().Format("15:04:05"))
		n, err := d.Run(dirFilter, personaFilter, max)
		if err != nil {
			fmt.Fprintf(d.Out, "✗ pass failed: %v\n", err)
		}
		if ctx.Err() != nil {
			return passes
		}
		wait = NextInterval(wait, base, maxInterval, n)
		fmt.Fprintf(d.Out, "   %d dispatched · next pass in %s (ctrl-c to stop)\n", n, wait.Round(time.Second))
		select {
		case <-ctx.Done():
			return passes
		case <-time.After(wait):
		}
	}
}

// stampWatchPid records this loop at $RHQ_HOME/state/dispatch-watch.pid.
// An unwritable state dir costs the record, never the loop: the hook then
// sees no live loop and falls back to its old kill-and-replace, which is
// what it did before the file existed.
func (d *Dispatcher) stampWatchPid() {
	path := WatchPidPath(d.App)
	if prev, ok := ReadWatchPid(path); ok && prev.Pid != os.Getpid() && prev.Alive() {
		fmt.Fprintf(d.errw(), "warning: another dispatch --watch loop looks live (pid %d) — two loops share one queue\n", prev.Pid)
	}
	rec := WatchPid{Pid: os.Getpid(), Started: d.now(), Cmd: strings.Join(os.Args, " ")}
	if err := WriteWatchPid(path, rec); err != nil {
		fmt.Fprintf(d.errw(), "warning: cannot record the watch loop at %s: %v\n", path, err)
	}
}

func (d *Dispatcher) dropWatchPid() { RemoveWatchPid(WatchPidPath(d.App), os.Getpid()) }
