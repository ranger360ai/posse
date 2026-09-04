package posse

// QA pins for the meter that goes silent exactly when the fleet is most
// exposed (ranger-base-ddivo).
//
// MEASURED 2026-09-03. `plan_guard_5h:`/`plan_guard_7d:` had been commented
// out since 09-01 under the operator's full-speed ruling. That made every
// PlanCache on the box quiet (ranger-base-4rfw1's guard-off arm), so the
// last reading this instance took is stamped 2026-09-01T23:23 local (5h
// 46%, 7d 29%) and nothing asked for two days — while `budget_pass:` and
// `budget_day:` were set and `dispatch --watch` hired against a weekly
// window nobody could see. The operator found Fable exhausted by hand.
// Reproduced on the box: the cockpit opened twice with the cooldown fields
// cleared from state/plan-usage.json, and no request left the machine.
//
// So the pins here are the mirror image of 4rfw1's: that bead pinned "a
// quiet meter asks nothing", and every pin carried an arm whose count was
// not zero. These pin "a SPENDING shop is not quiet", and every one carries
// an arm whose count IS zero — an "it asked" assertion over a rig that asks
// unconditionally is green with the quiet rule deleted, which would undo
// 4rfw1 wholesale.

import (
	"io"
	"strings"
	"testing"
	"time"
)

// ddivoT is the measured hour: the operator's 09-03 evening, with the last
// reading anybody took stamped 46 hours earlier.
var ddivoT = time.Date(2026, 9, 3, 21, 23, 0, 0, time.UTC)

// ─── the coupling, at the choke point ────────────────────────────────────────

// Guard unarmed and something spending: the cache asks on its TTL. Guard
// unarmed and nothing spending: it does not. Same rig, same three callers,
// and the only thing that moves between the rows is what the config says
// about money.
func TestQAUnarmedGuardStillAsksWhileTheShopSpends(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  string
		want int64 // requests the three callers may make between them
	}{
		// The state that STAYS quiet, and the reason the quiet rule was
		// right in the first place: nothing armed, nothing spending, so no
		// request it costs is one anybody asked for.
		{"idle, guard off", "", 0},
		// The bead. A dollar cap is ADR 0018's ledger brake, it knows
		// nothing about the weekly window, and it is the operator saying
		// this shop spends enough to need a brake at all.
		{"budget_pass set", "budget_pass: 150", 1},
		{"budget_day set", "budget_day: 3000", 1},
		// Either cap, together, once — the TTL is what holds the instance
		// to one request, not the arithmetic of which key was read.
		{"both caps set", "budget_pass: 150\nbudget_day: 3000", 1},
		// A cap that will not parse is the MOST exposed state there is: the
		// operator believes there is a brake and Dial E has silently
		// dropped it. A typo must not be able to mute the shop's only
		// meter — PlanMeterQuiet's own rule for its own flag.
		{"cap that will not parse", "budget_pass: banana", 1},
		// The flag stays the full mute. It is a ruling somebody typed, it
		// holds with the guard armed, and it has to hold over a spender
		// too or the operator cannot drain a 429 window without also
		// switching off Dial E.
		{"quiet flag outranks a spender", "budget_pass: 150\nplan_usage_quiet: true", 0},
		{"quiet flag, armed guard", "plan_guard_5h: 70\nplan_usage_quiet: true", 0},
		// The control from the other side: an armed guard asks whatever the
		// caps say, exactly as it did before this bead.
		{"armed guard, no caps", "plan_guard_5h: 70", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, ps := quietRig(t, tc.cfg)
			for _, caller := range []string{"cockpit", "status", "cost"} {
				cacheOver(a, ps, caller, ddivoT).Read(5 * time.Minute)
			}
			if got := ps.hits.Load(); got != tc.want {
				t.Errorf("%s: %d requests, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// The second spender, and the one the incident actually ran under: a
// `posse dispatch --watch` loop with no cap set anywhere. Liveness is the
// kernel's — the loop holds flock on state/dispatch-watch.lock for its
// whole life — so this rig takes the real lock rather than planting a
// pidfile, and the control is the same box a moment after it is dropped.
func TestQAWatchLoopRunningUnmutesTheMeter(t *testing.T) {
	t.Parallel()
	a, ps := quietRig(t, "")

	// Before: no loop, no cap. Quiet, and that is the behaviour 4rfw1 bought.
	if why := a.PlanMeterSpender(); why != "" {
		t.Fatalf("nothing is spending on a fresh box: %q", why)
	}
	if a.PlanCache("cockpit").Quiet == nil {
		t.Fatal("an idle shop with an unarmed guard must stay quiet")
	}
	if _, _, err := cacheOver(a, ps, "cockpit", ddivoT).Read(5 * time.Minute); PlanQuietReason(err) == nil {
		t.Fatalf("want the quiet refusal, got %v", err)
	}
	if got := ps.hits.Load(); got != 0 {
		t.Fatalf("the idle arm made %d requests", got)
	}

	// During: a loop of this RHQ_HOME holds the lock.
	lock, held, err := lockWatch(a)
	if err != nil || held {
		t.Fatalf("could not take the watch-loop lock: held=%v err=%v", held, err)
	}
	if why := a.PlanMeterSpender(); !strings.Contains(why, "--watch") {
		t.Errorf("a live watch loop is a spender, got %q", why)
	}
	if q := a.PlanCache("cockpit").Quiet; q != nil {
		t.Errorf("a live watch loop must not read as quiet: %v", q)
	}
	if _, _, err := cacheOver(a, ps, "cockpit", ddivoT).Read(5 * time.Minute); err != nil {
		t.Errorf("the meter must be readable while a loop is hiring: %v", err)
	}
	if got := ps.hits.Load(); got != 1 {
		t.Errorf("%d requests while the loop was running, want 1", got)
	}

	// After: release IS process death — no staleness class, nothing to reap.
	//
	// Polled, and that is a measurement and not a hedge. On darwin the flock
	// this drops with close(2) is NOT visible as released to a
	// flock(LOCK_SH|LOCK_NB) issued microseconds later: run beside the other
	// pins in this file (a loaded box, three parallel tests forking children)
	// this read `running` at t+0ms and `free` at t+100ms, 2 runs in 12. The
	// kernel gets there; an assertion written on the same instruction does
	// not. Nothing in posse reads the lock that fast — the autostart hook is
	// the only cross-process asker and it runs on a timer — so this is the
	// test's problem and not the guard's, said here because the next reader
	// of watchlock.go's "release IS process death" deserves the footnote.
	lock.Release()
	var why string
	for i := 0; i < 40; i++ {
		if why = a.PlanMeterSpender(); why == "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if why != "" {
		t.Errorf("a released lock is no loop, two seconds later: %q", why)
	}
	if a.PlanCache("cockpit").Quiet == nil {
		t.Error("quiet must come back when the loop ends")
	}
}

// ─── the pass that was hiring while nobody asked ─────────────────────────────

// The incident's own shape: guard unarmed, a dollar cap set, passes firing.
// The pass keeps the shared reading alive on its TTL and rules on NOTHING —
// no threshold, no blind clock, no park, and the reading joins no Dial E
// comparison it did not join before.
func TestQADispatchPassRefreshesTheMeterWithTheGuardOff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  string
		want int64
	}{
		// Three passes inside one TTL is one request for the whole
		// instance — the sharing rangerhq-tdy8 bought is what makes this
		// affordable at all.
		{"spending", "budget_pass: 150", 1},
		// The control, and the behaviour of every shop that never armed
		// anything: not one request, however many passes run.
		{"idle", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			ps := newPlanServer(t, 99, 99) // would skip every pass if a threshold read it
			d, errb := planDispatcher(t, b, ps)
			writePersona(t, b.App, "ranger", "[go]")
			repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"closed"}]`)
			planConfig(t, b.App, repo, tc.cfg)
			idleClaude(t, fake)

			for i := 0; i < 3; i++ {
				n, err := d.Run("", "", 0)
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 && n != 1 {
					t.Fatalf("an unarmed guard gates nothing: %d dispatched\n%s", n, dispatcherOut(d))
				}
			}
			if got := ps.hits.Load(); got != tc.want {
				t.Errorf("three passes made %d requests, want %d", got, tc.want)
			}
			// A reading at 99% would stop Dial E dead if the meter-only
			// path let it into the comparison. It must not: no threshold
			// is set, so the guard decides nothing and the windows stay
			// off the Dispatcher.
			if d.planUsage != nil {
				t.Errorf("the guard is off — the reading must rule on nothing, got %v", d.planUsage)
			}
			if out := dispatcherOut(d) + errb.String(); strings.Contains(out, "park") ||
				strings.Contains(out, "guard blind") || strings.Contains(out, "— skipped") {
				t.Errorf("keeping a number warm is not a brake:\n%s", out)
			}
		})
	}
}

// ─── and the number said out loud ────────────────────────────────────────────

// With the meter readable again, an OLD reading is loud where it was
// silent — the loud line (ranger-base-lpoui) was gated on the same quiet
// rule, so two days of unread window printed nothing anywhere. The clause
// forks because the armed sentence would be a lie: with no threshold set
// there is no headroom rule ruling on anything, and what is actually true
// is worse.
func TestQAStaleLineNamesTheUnarmedGuardInsteadOfTheHeadroomRule(t *testing.T) {
	t.Parallel()
	// The measured snapshot, and the measured gap: 2026-09-01T23:23Z, 46
	// hours before the operator went looking by hand.
	at := ddivoT.Add(-46 * time.Hour)
	windows := PlanUsage{{Name: "5h", Pct: 46}, {Name: "7d", Pct: 29}}

	t.Run("unarmed and spending", func(t *testing.T) {
		t.Parallel()
		b, _ := newTestBackend(t)
		appendConfig(t, b.App, "budget_pass: 150")
		seedReading(t, b.App, at, windows)

		st := b.App.PlanStaleness("watch", ddivoT, io.Discard)
		if !st.Stale {
			t.Fatal("two days unread is stale under any threshold this ships with")
		}
		const want = "plan meter BLIND 46h00m: last reading 2026-09-01T23:23Z (5h 46% · 7d 29%) — " +
			"the plan guard is UNARMED (budget_pass:/budget_day: is set), so nothing is ruling on it; " +
			"no request has left this machine since"
		if got := st.Line(); got != want {
			t.Errorf("the line is the deliverable:\n got %q\nwant %q", got, want)
		}
	})

	// The control that keeps the fork honest in the other direction: with a
	// threshold set the headroom rule IS running, and the sentence
	// ranger-base-lpoui pinned is untouched.
	t.Run("armed", func(t *testing.T) {
		t.Parallel()
		a := staleApp(t, "")
		seedReading(t, a, at, windows)
		st := a.PlanStaleness("watch", ddivoT, io.Discard)
		if !st.Stale {
			t.Fatal("an armed guard on a 46h-old reading is stale")
		}
		if !strings.Contains(st.Line(), "ruling on it under the headroom rule") ||
			strings.Contains(st.Line(), "UNARMED") {
			t.Errorf("an armed guard keeps lpoui's sentence: %q", st.Line())
		}
	})

	// And the state that is still silent, because in it the line would be
	// the furniture 4rfw1 refused: nothing armed, nothing spending, nobody
	// asking, nothing to say.
	t.Run("unarmed and idle", func(t *testing.T) {
		t.Parallel()
		b, _ := newTestBackend(t)
		seedReading(t, b.App, at, windows)
		if st := b.App.PlanStaleness("watch", ddivoT, io.Discard); st.Stale {
			t.Errorf("an idle shop is ruling on nothing and says nothing: %q", st.Line())
		}
	})
}
