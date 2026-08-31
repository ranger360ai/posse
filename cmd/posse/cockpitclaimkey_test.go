package main

// ranger-base-v5mh: after ranger-base-txio moved the two bd SCANS off the
// event loop, the cockpit had exactly two bd calls left on it, both from
// handleKey — the `c` key's Claim and the y that confirms a `u`. Both are
// WRITES, and a write is the expensive kind of bd call even with the daemon
// dial gone (ranger-base-cwu7): measured against a copy of this shop's own
// store, `bd update --claim` runs 1.25-2.14s and the unclaim 1.65-1.79s
// against 0.28s for a read, because a write re-exports the JSONL. Claim is
// up to three of those. So a keypress froze the TUI for seconds.
//
// These pin both halves of the fix: the keystroke returns immediately and
// the write lands through its own channel, and the keys that would put a
// second writer on the same bead are refused while one is in flight.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// slowClaimCockpit is qaProgFixture wired to a fake bd that takes `pause`
// over every call and records its argv, plus the claims channel runCockpit
// makes. The fake resolves the verb PAST bd's global flags, because
// posse.Bd.run leads every argv with --no-daemon (ranger-base-cwu7) and a
// fake keyed on $1 answers the wrong thing to everything.
func slowClaimCockpit(t *testing.T, pause string) (*cockpit, string) {
	t.Helper()
	dir := t.TempDir()
	log := dir + "/bd.log"
	bd := writeExec(t, dir, "bd", `#!/bin/sh
echo "$@" >> `+log+`
verb=""
for a in "$@"; do
  case "$a" in
  -*) ;;
  *) verb="$a"; break ;;
  esac
done
sleep `+pause+`
case "$verb" in
update) printf '[{"id":"r-a","status":"in_progress"}]\n' ;;
*) printf '[]\n' ;;
esac
exit 0
`)
	c := qaProgFixture()
	c.bd = posse.Bd{Bin: bd}
	c.claims = make(chan string, 4)
	for i := range c.issues {
		c.issues[i].Dir = dir
	}
	for i := range c.inprog {
		c.inprog[i].Dir = dir
	}
	return c, log
}

// take waits for the one line the claim goroutine owes the event loop and
// reports how long the whole thing actually took, so a fake that was not
// slow at all cannot make the latency assertions above it pass for free.
func takeClaim(t *testing.T, c *cockpit, start time.Time) (string, time.Duration) {
	t.Helper()
	select {
	case msg := <-c.claims:
		return msg, time.Since(start)
	case <-time.After(30 * time.Second):
		t.Fatal("the claim goroutine never reported")
	}
	return "", 0
}

// The bug itself. `c` is one bd write — several, when bd does not hand the
// claimed bead back — and on the event loop that is the whole TUI frozen:
// no draw, no refresh, no q, for as long as the store takes.
func TestCockpitClaimKeyDoesNotFreezeTheRenderLoop(t *testing.T) {
	c, log := slowClaimCockpit(t, "2")
	c.cursor = len(c.sessions) + len(c.inprog) // first ready row
	aimed := c.selIssue()
	if aimed == nil {
		t.Fatal("fixture: the cursor is not on a ready row")
	}

	start := time.Now()
	pressKeys(t, c, "c")
	if held := time.Since(start); held > time.Second {
		t.Errorf("the keystroke held the render loop for %s — the claim must run off it", held)
	}
	if !c.claiming {
		t.Error("nothing is in flight after c — the claim never started")
	}
	if !strings.Contains(c.status, "claiming "+aimed.ID) {
		t.Errorf("the operator is owed a line while it waits: %q", c.status)
	}

	msg, took := takeClaim(t, c, start)
	if msg != "claimed "+aimed.ID {
		t.Errorf("the landed status is %q, want %q", msg, "claimed "+aimed.ID)
	}
	// The rig has to be able to fail: if bd came back instantly the pin
	// above measured nothing.
	if took < 1500*time.Millisecond {
		t.Errorf("the fake bd returned in %s — a slow write is not being exercised", took)
	}
	if got := qaBdLog(t, log); !strings.Contains(got, "update "+aimed.ID+" --claim") {
		t.Errorf("bd was not asked to claim the aimed bead: %q", got)
	}
}

// Same for the write behind the confirm. The y is where the unclaim happens,
// so that is where the freeze was.
func TestCockpitUnclaimConfirmDoesNotFreezeTheRenderLoop(t *testing.T) {
	c, log := slowClaimCockpit(t, "2")
	c.cursor = len(c.sessions) // first in-progress row
	aimed := c.selInProg()
	if aimed == nil {
		t.Fatal("fixture: the cursor is not on a claimed row")
	}

	pressKeys(t, c, "u")
	start := time.Now()
	pressKeys(t, c, "y")
	if held := time.Since(start); held > time.Second {
		t.Errorf("the y held the render loop for %s — the unclaim must run off it", held)
	}
	if !c.claiming {
		t.Error("nothing is in flight after the y — the unclaim never started")
	}
	if c.mode != modeNormal {
		t.Errorf("the confirm must close on the y, not on the write: mode %d", c.mode)
	}
	if !strings.Contains(c.status, "unclaiming "+aimed.ID) {
		t.Errorf("the operator is owed a line while it waits: %q", c.status)
	}

	msg, took := takeClaim(t, c, start)
	if msg != "unclaimed "+aimed.ID {
		t.Errorf("the landed status is %q, want %q", msg, "unclaimed "+aimed.ID)
	}
	if took < 1500*time.Millisecond {
		t.Errorf("the fake bd returned in %s — a slow write is not being exercised", took)
	}
	if got := qaBdLog(t, log); !strings.Contains(got, "update "+aimed.ID+" --status open") {
		t.Errorf("bd was not asked to unclaim the aimed bead: %q", got)
	}
}

// A write that has left the event loop can be started twice, and the second
// one is aimed off a list the first has not refreshed yet. `c` and `u` share
// the flag because they are the same write on the same row; `d` shares it
// because a dispatch CLAIMS the bead it launches, and bd answers the loser
// of two concurrent claims with "claim on X lost", which reads as a mystery.
func TestCockpitRefusesASecondWriteWhileOneIsInFlight(t *testing.T) {
	c, log := slowClaimCockpit(t, "2")
	launched := false
	c.launcher = func(posse.RepoIssue, bool) (string, error) {
		launched = true
		return "some-session", nil
	}
	c.cursor = len(c.sessions) + len(c.inprog)
	start := time.Now()
	pressKeys(t, c, "c")
	if !c.claiming {
		t.Fatal("fixture: no claim is in flight")
	}

	for _, key := range []string{"c", "u", "d"} {
		c.status = ""
		pressKeys(t, c, key)
		if !strings.Contains(c.status, "already in flight") {
			t.Errorf("%s during an in-flight claim: status %q", key, c.status)
		}
		if c.mode != modeNormal {
			t.Errorf("%s during an in-flight claim opened mode %d", key, c.mode)
		}
	}
	if launched {
		t.Error("ESCAPE: d dispatched a bead while a claim was still writing it")
	}

	takeClaim(t, c, start)
	// One write reached bd, not four.
	if n := strings.Count(qaBdLog(t, log), "--claim"); n != 1 {
		t.Errorf("bd saw %d claims, want 1: %q", n, qaBdLog(t, log))
	}

	// The wrong arm: once the flight is over the same keys work again. A
	// flag that was never cleared would pass every assertion above.
	c.claiming = false
	c.status = ""
	pressKeys(t, c, "c")
	if !c.claiming || !strings.Contains(c.status, "claiming ") {
		t.Errorf("control: c after the flight landed did not start one: %q", c.status)
	}
	takeClaim(t, c, time.Now())
}

// A channel is only a status line if something drains it, and the drain is
// one case in runCockpit's select — which no test runs, because it wants a
// raw-mode terminal (the pin TestCockpitEventLoopDrainsPrompts makes, for
// the same reason). Without it the claim's result lands in a buffer nobody
// reads, c.claiming is never cleared, and `c`, `u` and `d` are all dead for
// the rest of the session while every test above stays green.
func TestCockpitEventLoopDrainsClaims(t *testing.T) {
	b, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{"case msg := <-c.claims:", "c.claiming = false"} {
		if !strings.Contains(src, want) {
			t.Errorf("runCockpit's event loop does not %s — nothing drains the claim channel", want)
		}
	}
	// And the refresh a claim is ordered against belongs to that case, not
	// to the goroutine: refreshSessions and kickBeads touch c.
	if strings.Contains(src, "c.refresh()\n\t\tc.claims <-") {
		t.Error("the claim goroutine refreshes — that is the event loop's, and it races it")
	}
}
