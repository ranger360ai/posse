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
//
// The error is the refusal to start: another loop of this RHQ_HOME already
// holds the watch lock, and one loop per queue is the invariant. It is
// returned before any pass, so a refused Watch has done nothing.
func (d *Dispatcher) Watch(ctx context.Context, dirFilter, personaFilter string, max int, base, maxInterval time.Duration) (int, error) {
	if maxInterval < base {
		maxInterval = base
	}
	// The loop's liveness, kernel-owned for its whole life (rangerhq-gir5).
	// Taken before anything else so a second loop refuses without having
	// touched the queue.
	lock, held, err := lockWatch(d.App)
	if held {
		return 0, Die("another dispatch --watch loop of %s is already running — one loop per queue (ADR 0011 §1)", AbbrevHome(d.App.StateDir))
	}
	if err != nil {
		// Degraded, never fatal: an unwritable state dir costs the record,
		// not the loop. Say it, because while it lasts the autostart hook
		// reads this loop as no loop and may replace it.
		fmt.Fprintf(d.errw(), "warning: cannot hold the watch lock at %s: %v — the autostart hook cannot see this loop\n", AbbrevHome(WatchLockPath(d.App)), err)
	}
	defer lock.Release()
	// A timer typed this command, not a human: the plan guard's fail-open
	// line has no witness from here on, so blindness gets a clock
	// (rangerhq-6h1). Seeded at loop start, not at the epoch — the first
	// pass of a fresh loop gets the whole grace rather than an instant skip.
	d.Unattended = true
	d.blindSince = d.now()
	// Identity, not liveness: which pid, since when, under what argv. The
	// lock above is what anything asking "is the loop running?" tests
	// (rangerhq-gir5); this is what it quotes once the answer is yes.
	defer d.dropWatchPid()
	d.stampWatchPid()
	// The pulse (ADR 0027 §1-2, rangerhq-4ish): a shop-check ticker that
	// starts with this loop and dies with it. Disarmed (no pulse_interval:
	// in config) starts nothing; a config error disarms this run rather
	// than failing the watch loop over it.
	if cfg, err := LoadPulseConfig(d.App); err != nil {
		fmt.Fprintf(d.errw(), "pulse: %v — disarmed for this loop\n", err)
	} else if cfg.Armed {
		go d.pulseLoop(ctx, cfg)
	}
	// The settle-event channel (ADR 0016 §1, ADR 0028 §1). One subscription
	// for the life of this loop, and in THIS slice (ranger-base-0s36) the
	// hint is logged and nothing else: the tick below is untouched, so a
	// hint changes no dispatch behaviour and a lost one costs not even
	// latency yet. Waking the next pass early is S4's (ranger-base-zk5u).
	//
	// No herdr, no socket, a dead server: the subscriber says so once and
	// this loop goes on ticking. It is a latency path, never a dependency.
	subscribe := d.Hints
	if subscribe == nil {
		subscribe = func(ctx context.Context, report func(string)) <-chan HerdrHint {
			return HerdrSettleHints(ctx, herdrSocketPath(), report)
		}
	}
	hints := subscribe(ctx, func(line string) { fmt.Fprintf(d.Out, "   %s\n", line) })
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
			return passes, nil
		}
		wait = NextInterval(wait, base, maxInterval, n)
		fmt.Fprintf(d.Out, "   %d dispatched · next pass in %s (ctrl-c to stop)\n", n, wait.Round(time.Second))
		// One timer per pass, and a hint does NOT reset it: hints are
		// logged here, the tick still decides when the next pass runs.
		timer := time.NewTimer(wait)
		for tick := false; !tick; {
			select {
			case <-ctx.Done():
				timer.Stop()
				return passes, nil
			case h, ok := <-hints:
				if !ok {
					// The subscriber is gone for good (ctx ended, or it
					// stopped); wait out the timer on a nil channel.
					hints = nil
					continue
				}
				fmt.Fprintf(d.Out, "   settle hint · %s — logged only, the tick still sets the next pass (ADR 0028 §1)\n", h)
			case <-timer.C:
				tick = true
			}
		}
	}
}

// stampWatchPid records who this loop is at $RHQ_HOME/state/dispatch-watch.pid.
// An unwritable state dir costs the record, never the loop.
//
// It overwrites whatever it finds, and no longer inspects it first. The old
// "another loop looks live (pid N)" warning read a live pid in a stale
// record as a second loop — the inference this bead removed, and a reliable
// source of false alarms once a pid was recycled. The lock is what knows,
// and it has already answered by the time this runs: a genuine second loop
// never reaches here, because it refused above.
func (d *Dispatcher) stampWatchPid() {
	path := WatchPidPath(d.App)
	rec := WatchPid{Pid: os.Getpid(), Started: d.now(), Cmd: strings.Join(os.Args, " ")}
	if err := WriteWatchPid(path, rec); err != nil {
		fmt.Fprintf(d.errw(), "warning: cannot record the watch loop at %s: %v\n", path, err)
	}
}

func (d *Dispatcher) dropWatchPid() { RemoveWatchPid(WatchPidPath(d.App), os.Getpid()) }
