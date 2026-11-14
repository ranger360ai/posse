//go:build posse_arm2

package posse

// The pulse waiter (ADR 0093 D1/D3, ranger-base-cgr1h): `posse pulse wait
// --after <seq>`.
//
// ARM 2 and not arm 3, where the rest of the pulse lives (pulse_test.go,
// pulsefeed_test.go, pulsedeliverystate_qa_test.go are all `posse_arm3`).
// What is here is mostly flock ACQUISITION — who holds the waiter lock,
// whether a released one reads as free, whether a killed holder's lock goes —
// and that family already lives in this arm beside watchlock.go's, under the
// serial discipline it needs: two acquisition tests side by side read a
// released lock as still held, 3-6 failures in 60 at -parallel 8
// (ranger-base-9l77f). So none of these carries t.Parallel, cmd/testparallel
// names them so a later pass cannot add it, and they sit in the arm where
// that reasoning is written down. A seat looking for the feed's own line
// format wants `-tags posse_arm3` and pulsefeed_test.go; a seat looking for
// the waiter wants `-tags posse_arm2` and this file.
//
// Real flock in a temp RHQ_HOME, no fake: flock is per open file
// description, so two Open+Flock pairs inside one test process contend
// exactly as two processes do, and a faked lock would only test the fake.
// One test does fork, because one claim here is specifically about a process
// that DIES.
//
// The feed these tests read is written by pulseFeedWrite — the real writer,
// never a hand-rolled line — so the format cannot drift between the thing
// that appends and the thing that waits. The one exception is seedFeedAt,
// which hand-writes a single high-numbered entry to reach a capped-out feed
// cheaply, and which asserts that what it wrote is a line the waiter's own
// reading accepts before any test leans on it.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// pwFeed appends bodies to the waiter's feed through the real writer and
// returns the last sequence number it handed out.
func pwFeed(t *testing.T, a *App, bodies ...string) int {
	t.Helper()
	last := 0
	for _, body := range bodies {
		seq, err := pulseFeedWrite(PulseFeedPath(a), time.Now(), body)
		if err != nil {
			t.Fatalf("appending %q to the feed: %v", body, err)
		}
		last = seq
	}
	return last
}

// seedFeedAt plants ONE entry numbered seq by hand, so a test can reach a
// feed whose oldest kept entry is far past a cursor without appending 500 of
// them (the writer's next-seq rule is max-parseable plus one, so the real
// writer carries on from here).
//
// It checks its own fixture: a seed line PulseFeedSeq cannot read would make
// every assertion below it an assertion about an empty feed.
func seedFeedAt(t *testing.T, a *App, seq int) {
	t.Helper()
	path := PulseFeedPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf("%d %s +seed standing: 1", seq, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, ok := PulseFeedSeq(line); !ok || n != seq {
		t.Fatalf("the seed line is not one the waiter's own reading accepts: %q", line)
	}
}

// pwLines is what the waiter printed, newline-split with the trailing blank
// dropped.
func pwLines(t *testing.T, buf *bytes.Buffer) []string {
	t.Helper()
	s := strings.TrimSuffix(buf.String(), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// wantCursorLast asserts ADR 0093 D1's re-arming contract: whatever else a
// waiter printed, its LAST line is the command to come back with, carrying
// the cursor. The coordinator's PID (ranger-base-05ooy) re-arms off this
// line, so a waiter that printed a delta and forgot the cursor has woken a
// session that cannot re-arm without guessing.
func wantCursorLast(t *testing.T, lines []string, after int) {
	t.Helper()
	if len(lines) == 0 {
		t.Fatal("the waiter printed nothing at all — every exit owes the cursor")
	}
	want := fmt.Sprintf("next: posse pulse wait --after %d", after)
	if got := lines[len(lines)-1]; got != want {
		t.Errorf("last line = %q, want %q", got, want)
	}
}

// The ordinary delivery: the entries past the cursor, and nothing else.
func TestPulseWaitPrintsTheEntriesPastTheCursor(t *testing.T) {
	a := watchApp(t)
	pwFeed(t, a, "+blocked:developer-work standing: 1", "+unpushed:/r:1 standing: 2", "-blocked:developer-work standing: 1")

	var buf bytes.Buffer
	started := time.Now()
	if err := PulseWait(context.Background(), a, PulseWaitOptions{After: 1, Renag: time.Minute}, &buf); err != nil {
		t.Fatal(err)
	}
	// A cursor the feed is already past must not block at all: the backlog
	// is why the cursor lives on the command line (D1).
	if d := time.Since(started); d > 10*time.Second {
		t.Errorf("a waiter with a backlog waited %v — it must return at once", d)
	}
	lines := pwLines(t, &buf)
	if len(lines) != 3 {
		t.Fatalf("want 2 entries and the cursor line, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, want := range []string{"+unpushed:/r:1 standing: 2", "-blocked:developer-work standing: 1"} {
		if !strings.HasSuffix(lines[i], want) {
			t.Errorf("line %d = %q, want an entry whose body is %q", i, lines[i], want)
		}
	}
	// Entry 1 is BEHIND the cursor and must not be re-said — re-sending what
	// the coordinator was already told is the noise D2 exists to remove.
	if strings.Contains(buf.String(), "+blocked:developer-work standing: 1") {
		t.Errorf("an entry at or behind the cursor was re-printed:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "gap") {
		t.Errorf("a cursor the feed can account for is not a gap:\n%s", buf.String())
	}
	wantCursorLast(t, lines, 3)
}

// The blocking half, which is the whole verb: nothing to say, then something.
func TestPulseWaitBlocksUntilAnEntryArrives(t *testing.T) {
	a := watchApp(t)
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const quiet = 150 * time.Millisecond
	go func() {
		time.Sleep(quiet)
		_, _ = pulseFeedWrite(PulseFeedPath(a), time.Now(), "+blocked:alice standing: 1")
	}()

	var buf bytes.Buffer
	started := time.Now()
	opt := PulseWaitOptions{After: 0, Renag: 30 * time.Second, Poll: 10 * time.Millisecond}
	if err := PulseWait(context.Background(), a, opt, &buf); err != nil {
		t.Fatal(err)
	}
	waited := time.Since(started)
	// The PIN, and the reason it is a duration: a waiter that returned
	// before the entry existed would have printed an empty delta and woken
	// the coordinator for nothing, and every content assertion below would
	// still pass if it then found the line on a second run.
	if waited < quiet {
		t.Errorf("the waiter returned after %v, before the entry was appended at %v — it did not block", waited, quiet)
	}
	if waited > 25*time.Second {
		t.Errorf("the waiter took %v to notice an appended entry", waited)
	}
	lines := pwLines(t, &buf)
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "+blocked:alice standing: 1") {
		t.Fatalf("want the appended entry then the cursor line, got:\n%s", strings.Join(lines, "\n"))
	}
	wantCursorLast(t, lines, 1)
}

// GAP, shape one: the cursor fell off the back of the 500-line cap.
func TestPulseWaitStaleCursorPrintsGapThenWhatIsKept(t *testing.T) {
	a := watchApp(t)
	seedFeedAt(t, a, 112)
	last := pwFeed(t, a, "+blocked:bob standing: 2")
	if last != 113 {
		t.Fatalf("the real writer continued from the seed at %d, want 113", last)
	}

	var buf bytes.Buffer
	if err := PulseWait(context.Background(), a, PulseWaitOptions{After: 5, Renag: time.Minute}, &buf); err != nil {
		t.Fatal(err)
	}
	lines := pwLines(t, &buf)
	if len(lines) == 0 || lines[0] != "gap" {
		t.Fatalf("a cursor older than the oldest kept entry must print gap FIRST, got:\n%s", strings.Join(lines, "\n"))
	}
	// ...and then what is kept, all of it: on a gap the question "what did I
	// miss" has no answer narrower than "what is left".
	if len(lines) != 4 {
		t.Fatalf("want gap, 2 kept entries and the cursor line, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, want := range []int{112, 113} {
		if n, ok := PulseFeedSeq(lines[i+1]); !ok || n != want {
			t.Errorf("kept line %d reads as seq %d (ok=%v), want %d: %q", i, n, ok, want, lines[i+1])
		}
	}
	wantCursorLast(t, lines, 113)
}

// GAP, shape two, and the one ADR 0093's prose does not name: the feed
// RESTARTED under the cursor. The ADR says losing state/pulse.feed costs an
// armed waiter one gap — and it only does if "older than the oldest kept
// line" is not the whole test, because after a restart every kept entry is
// numbered BELOW the cursor and the oldest-line reading sees a quiet shop.
//
// Measured the other way (what makes this a pin rather than a restatement):
// drop the `newest < after` arm from pulseFeedAfter and this test blocks
// until its renag and then self-exits with `still:`, on a feed that is in
// fact delivering — silent on both sides, which is the exact failure mode
// the feed's own next-seq rule exists to avoid at the writer.
func TestPulseWaitTreatsARestartedFeedAsAGap(t *testing.T) {
	a := watchApp(t)
	pwFeed(t, a, "+blocked:carol standing: 1", "+unpushed:/r:3 standing: 2")

	var buf bytes.Buffer
	opt := PulseWaitOptions{After: 400, Renag: 300 * time.Millisecond, Poll: 10 * time.Millisecond}
	if err := PulseWait(context.Background(), a, opt, &buf); err != nil {
		t.Fatal(err)
	}
	lines := pwLines(t, &buf)
	if len(lines) == 0 || lines[0] != "gap" {
		t.Fatalf("a feed whose newest entry is below the cursor is a gap, got:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Contains(buf.String(), "still:") {
		t.Errorf("a restarted feed read as a quiet shop — the waiter went silent over a feed that was delivering:\n%s", buf.String())
	}
	if len(lines) != 4 {
		t.Fatalf("want gap, both kept entries and the cursor line, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	wantCursorLast(t, lines, 2)
}

// The renag self-exit (ADR 0093's "what breaks if this is wrong"): a bounded
// life is what makes a WEDGED waiter — one holding the lock and silencing
// the typed path — tellable from a healthy one on a quiet shop.
func TestPulseWaitSelfExitsAfterTheRenagWithNothingToSay(t *testing.T) {
	a := watchApp(t)
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const life = 200 * time.Millisecond

	var buf bytes.Buffer
	started := time.Now()
	opt := PulseWaitOptions{After: 7, Renag: life, Poll: 10 * time.Millisecond}
	if err := PulseWait(context.Background(), a, opt, &buf); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(started); waited < life {
		t.Errorf("the waiter self-exited after %v, want at least its %v life", waited, life)
	}
	lines := pwLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("want the still: line and the cursor line, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[0], "still:") {
		t.Errorf("the self-exit line = %q, want it to open with still: — the word ADR 0093 gives it", lines[0])
	}
	// The cursor is handed back UNCHANGED. A self-exit saw nothing, so
	// advancing it would skip whatever lands next.
	wantCursorLast(t, lines, 7)
}

// One listener per feed. D3 switches delivery on "the lock is held", so two
// holders would be two coordinators woken by one delta — and the refusal has
// to name who holds it, because that is the only thing the operator can act
// on.
func TestPulseWaitRefusesASecondWaiter(t *testing.T) {
	a := watchApp(t)
	first, held, err := lockPulseWait(a, time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC))
	if err != nil || held {
		t.Fatalf("arranging the armed waiter: held=%v err=%v", held, err)
	}
	defer first.Release()
	pwFeed(t, a, "+blocked:developer-work standing: 1")

	var buf bytes.Buffer
	err = PulseWait(context.Background(), a, PulseWaitOptions{Renag: time.Minute}, &buf)
	if err == nil {
		t.Fatal("a second waiter on one feed must refuse")
	}
	for _, want := range []string{"already armed", fmt.Sprintf("pid %d", os.Getpid()), "2026-11-14T09:30:00Z"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q, got %q", want, err)
		}
	}
	// A refused waiter prints NOTHING: there is no cursor to hand back when
	// nothing was ever armed, and a `next:` line here would tell the
	// coordinator it had been listening.
	if buf.Len() != 0 {
		t.Errorf("a refused waiter printed %q", buf.String())
	}
	// And it did not deliver the entry it could see — the armed waiter owns
	// that delta.
	if strings.Contains(buf.String(), "blocked:developer-work") {
		t.Error("a refused waiter delivered the armed waiter's entry")
	}
}

// The identity half, and what it decides: nothing. A body nobody wrote still
// leaves the lock a working switch, and the refusal still forms a sentence.
func TestPulseWaitRefusalSurvivesAnUnrecordedHolder(t *testing.T) {
	a := watchApp(t)
	first, held, err := lockPulseWait(a, time.Now())
	if err != nil || held {
		t.Fatalf("arranging the armed waiter: held=%v err=%v", held, err)
	}
	defer first.Release()
	// Blank the body, keeping the lock: a waiter from a posse too old to
	// stamp one, or a truncated write.
	if err := os.WriteFile(PulseWaitLockPath(a), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if w, ok := ReadPulseWaiter(PulseWaitLockPath(a)); ok {
		t.Fatalf("fixture did not blank the record: %+v", w)
	}

	var buf bytes.Buffer
	err = PulseWait(context.Background(), a, PulseWaitOptions{Renag: time.Minute}, &buf)
	if err == nil {
		t.Fatal("an unrecorded holder still holds the lock — the second waiter must refuse")
	}
	if !strings.Contains(err.Error(), "holder unrecorded") {
		t.Errorf("the refusal must say the record is missing rather than trail off, got %q", err)
	}
}

// The two states the probe distinguishes, and nothing between them —
// PulseWaiterArmed is what the watch arm (ranger-base-r3qz6) reads to decide
// whether anything is listening.
func TestPulseWaiterArmedTracksTheLock(t *testing.T) {
	a := watchApp(t)

	if armed, err := PulseWaiterArmed(a); err != nil || armed {
		t.Fatalf("a fresh RHQ_HOME has no waiter: armed=%v err=%v", armed, err)
	}
	lock, held, err := lockPulseWait(a, time.Now())
	if err != nil || held || lock == nil {
		t.Fatalf("the first waiter must get the lock: held=%v err=%v", held, err)
	}
	if armed, err := PulseWaiterArmed(a); err != nil || !armed {
		t.Fatalf("a held lock is an armed waiter: armed=%v err=%v", armed, err)
	}
	// Two probes must not read each other as the waiter — that is why the
	// probe takes LOCK_SH and not LOCK_EX. A watch that read its own
	// previous tick's probe as the listener would never type a fallback.
	if _, err := PulseWaiterArmed(a); err != nil {
		t.Fatalf("second probe: %v", err)
	}
	if armed, _ := PulseWaiterArmed(a); !armed {
		t.Error("a probe consumed the answer")
	}
	lock.Release()
	if armed, err := PulseWaiterArmed(a); err != nil || armed {
		t.Fatalf("a released lock is no waiter: armed=%v err=%v", armed, err)
	}
	lock.Release() // idempotent
}

// The other half of D3's contract, and the one every content test above
// stays green without: the lock is held for the waiter's WHOLE life, not for
// the instant it took to arm. A waiter that dropped it and kept blocking
// would read as unarmed, so the watch would type into the pane of a session
// that is listening — the splice this ADR exists to end.
func TestPulseWaitHoldsTheLockWhileItBlocks(t *testing.T) {
	a := watchApp(t)
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- PulseWait(ctx, a, PulseWaitOptions{Renag: 30 * time.Second, Poll: 10 * time.Millisecond}, &buf)
	}()

	// Up when the lock reads as held from a second open file description,
	// which is the same contention a separate process gets.
	armed := false
	for i := 0; i < 500 && !armed; i++ {
		if ok, err := PulseWaiterArmed(a); err == nil && ok {
			armed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !armed {
		cancel()
		<-done
		t.Fatal("the waiter never took the lock")
	}
	// Blocked on a feed with nothing in it, well inside its life — and
	// still holding.
	time.Sleep(100 * time.Millisecond)
	if ok, err := PulseWaiterArmed(a); err != nil || !ok {
		cancel()
		<-done
		t.Fatalf("a blocked waiter must still hold the lock: armed=%v err=%v", ok, err)
	}
	// The record names this process, which is the identity half D3 asks for.
	w, ok := ReadPulseWaiter(PulseWaitLockPath(a))
	if !ok || w.Pid != os.Getpid() {
		cancel()
		<-done
		t.Fatalf("the lock body must name the holder: %+v (ok=%v), want pid %d", w, ok, os.Getpid())
	}
	if w.Started.IsZero() {
		cancel()
		<-done
		t.Error("D3 asks for the start time too — it is what makes a waiter older than its life tellable")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("an interrupted waiter is not an error: %v", err)
	}
	// And the lock goes when the waiter does.
	if ok, err := PulseWaiterArmed(a); err != nil || ok {
		t.Errorf("a waiter that exited must release the lock: armed=%v err=%v", ok, err)
	}
	// Interrupted, it still named the cursor it was waiting on.
	wantCursorLast(t, pwLines(t, &buf), 0)
}

// The claim a pidfile could never make, and ADR 0093 D3's whole reason for
// being a lock: liveness ends when the process does. A SIGKILLed waiter —
// the closed-pane case, the runtime taking its background commands down —
// leaves the lock free with nothing to reap: no staleness window, no
// `kill -0`, no argv to match. The watch's next tick reads it as unarmed and
// falls back to the typed pulse, which is how a coordinator that died does
// not go dark.
func TestPulseWaitLockDiesWithItsProcess(t *testing.T) {
	a := watchApp(t)
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestPulseWaitHolderChild$", "-test.v")
	// The child must run as a test binary, not as the fake substrate TestMain
	// turns it into whenever RHQ_FAKE_HERDR is set — the same filter the
	// other re-exec sites in this package carry, and spelled the same way
	// watchlock_test.go's does.
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") {
			env = append(env, kv)
		}
	}
	child.Env = append(env, "POSSE_PULSEWAIT_HOLD="+a.StateDir)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = child.Process.Kill()
		_, _ = child.Process.Wait()
	}()
	// The child says so once the lock is its own.
	buf := make([]byte, 64)
	deadline := time.Now().Add(30 * time.Second)
	got := ""
	for !strings.Contains(got, "armed") && time.Now().Before(deadline) {
		n, err := stdout.Read(buf)
		got += string(buf[:n])
		if err != nil {
			break
		}
	}
	if !strings.Contains(got, "armed") {
		t.Fatalf("child never took the lock: %q", got)
	}
	if armed, err := PulseWaiterArmed(a); err != nil || !armed {
		t.Fatalf("another process's waiter must read as armed: armed=%v err=%v", armed, err)
	}
	// The identity record is the child's, and it is about to become a lie —
	// which is exactly what the lock does not care about.
	w, ok := ReadPulseWaiter(PulseWaitLockPath(a))
	if !ok || w.Pid != child.Process.Pid {
		t.Errorf("the lock body must name the child: %+v (ok=%v), want pid %d", w, ok, child.Process.Pid)
	}

	if err := child.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if _, err := child.Process.Wait(); err != nil {
		t.Fatal(err)
	}
	armed, err := PulseWaiterArmed(a)
	if err != nil {
		t.Fatal(err)
	}
	if armed {
		t.Error("a killed waiter must read as unarmed immediately — that is the whole point of the lock")
	}
	// And the stale record survives, deciding nothing. If this ever starts
	// reading as unarmed-because-the-body-went, the probe has stopped
	// asking the kernel.
	if _, ok := ReadPulseWaiter(PulseWaitLockPath(a)); !ok {
		t.Error("the fixture no longer proves the body outlives the holder")
	}
}

// The child half of TestPulseWaitLockDiesWithItsProcess. Inert unless the
// env var selects it, so a plain `go test` never runs it.
func TestPulseWaitHolderChild(t *testing.T) {
	dir := os.Getenv("POSSE_PULSEWAIT_HOLD")
	if dir == "" {
		t.Skip("child of TestPulseWaitLockDiesWithItsProcess")
	}
	lock, held, err := lockPulseWait(&App{StateDir: dir}, time.Now())
	if err != nil || held {
		t.Fatalf("child could not arm the waiter: held=%v err=%v", held, err)
	}
	defer lock.Release()
	os.Stdout.WriteString("armed\n")
	time.Sleep(60 * time.Second) // killed by the parent long before this
}

// The re-arming contract as ONE pin over every exit a waiter has, because
// ranger-base-05ooy's coordinator PID re-arms off this line and nothing else:
// a delta, a gap, a renag self-exit and an interrupt all end by saying what
// to come back with. Each arm is also covered above for its CONTENT; what is
// here is that no exit forgets the cursor.
func TestPulseWaitEveryExitNamesTheCursorLast(t *testing.T) {
	cases := []struct {
		name  string
		arm   func(t *testing.T, a *App) context.Context
		after int
		want  int
	}{
		{
			name: "a delta past the cursor",
			arm: func(t *testing.T, a *App) context.Context {
				pwFeed(t, a, "+blocked:developer-work standing: 1")
				return context.Background()
			},
			after: 0, want: 1,
		},
		{
			name: "a cursor the feed cannot account for",
			arm: func(t *testing.T, a *App) context.Context {
				seedFeedAt(t, a, 112)
				return context.Background()
			},
			after: 5, want: 112,
		},
		{
			name: "the renag self-exit",
			arm: func(t *testing.T, a *App) context.Context {
				if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
					t.Fatal(err)
				}
				return context.Background()
			},
			after: 42, want: 42,
		},
		{
			name: "an interrupt",
			arm: func(t *testing.T, a *App) context.Context {
				if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					time.Sleep(50 * time.Millisecond)
					cancel()
				}()
				t.Cleanup(cancel)
				return ctx
			},
			after: 9, want: 9,
		},
	}
	for _, c := range cases {
		// Serial, like everything in this file: each case arms the lock.
		t.Run(c.name, func(t *testing.T) {
			a := watchApp(t)
			ctx := c.arm(t, a)
			var buf bytes.Buffer
			opt := PulseWaitOptions{After: c.after, Renag: 200 * time.Millisecond, Poll: 10 * time.Millisecond}
			if err := PulseWait(ctx, a, opt, &buf); err != nil {
				t.Fatal(err)
			}
			lines := pwLines(t, &buf)
			wantCursorLast(t, lines, c.want)
			// Exactly one, so a waiter cannot satisfy this by printing the
			// line on every poll.
			n := 0
			for _, ln := range lines {
				if strings.HasPrefix(ln, "next: ") {
					n++
				}
			}
			if n != 1 {
				t.Errorf("printed %d cursor lines, want exactly 1:\n%s", n, strings.Join(lines, "\n"))
			}
		})
	}
}

// pulseFeedAfter's own table, over the cases the end-to-end tests above
// reach only one at a time. The reading is three answers and they disagree
// in ways no single run shows: an empty feed is NOT a gap, a cursor the feed
// sits exactly on is NOT a gap, and a cursor one behind the oldest kept
// entry is not one either — the cap dropped nothing in that case.
// Parallel, unlike everything else in this file: it takes no lock, reads no
// file and calls the reading directly, so the 9l77f contention the rest of
// this file is serial for does not reach it.
func TestPulseFeedAfterReadsTheCursorAgainstWhatIsKept(t *testing.T) {
	t.Parallel()
	entry := func(seq int) string {
		return fmt.Sprintf("%d 2026-11-14T09:31:04Z +k standing: 1", seq)
	}
	cases := []struct {
		name    string
		entries []string
		after   int
		lines   int
		gap     bool
		last    int
	}{
		{"an empty feed is not a gap", nil, 7, 0, false, 7},
		{"a feed of only damage is not a gap", []string{"not a feed line"}, 7, 0, false, 7},
		{"nothing past the cursor", []string{entry(1), entry(2)}, 2, 0, false, 2},
		{"the ordinary delta", []string{entry(1), entry(2), entry(3)}, 1, 2, false, 3},
		{"a cold start takes what is kept", []string{entry(1), entry(2)}, 0, 2, false, 2},
		{"exactly one behind the oldest is no gap", []string{entry(4), entry(5)}, 3, 2, false, 5},
		{"two behind the oldest is a gap", []string{entry(4), entry(5)}, 2, 2, true, 5},
		{"a restarted feed is a gap", []string{entry(1), entry(2)}, 400, 2, true, 2},
		{"damage among the entries is skipped", []string{entry(1), "x", entry(2)}, 1, 1, false, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines, gap, last := pulseFeedAfter(c.entries, c.after)
			if len(lines) != c.lines || gap != c.gap || last != c.last {
				t.Errorf("pulseFeedAfter(%v, %d) = %d line(s), gap=%v, last=%d; want %d, %v, %d",
					c.entries, c.after, len(lines), gap, last, c.lines, c.gap, c.last)
			}
		})
	}
}

// pulse_renag is the waiter's life, and a DISARMED pulse must not make that
// life zero: LoadPulseConfig answers a config with no pulse_interval: with a
// zero PulseConfig, and reading that zero as the life exits the waiter the
// instant it arms — which, re-invoked by the coordinator's runtime, is a hot
// spin rather than a wait.
// Parallel for the same reason as the reading's table above: it arms
// nothing, and its config is its own temp file.
func TestPulseWaitLifeFallsBackRatherThanToZero(t *testing.T) {
	t.Parallel()
	a := watchApp(t)
	a.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")

	// Disarmed: no pulse_interval: at all.
	if err := os.WriteFile(a.ConfigPath, []byte("beads:\n  - /tmp/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := pulseWaitLife(a, PulseWaitOptions{}); got != DefaultPulseRenag {
		t.Errorf("a disarmed pulse gives a life of %v, want the %v default — zero is a spin", got, DefaultPulseRenag)
	}
	// Armed, with a renag of its own: that is the life.
	if err := os.WriteFile(a.ConfigPath, []byte("pulse_interval: 2m\npulse_renag: 12m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := pulseWaitLife(a, PulseWaitOptions{}); got != 12*time.Minute {
		t.Errorf("life = %v, want the configured pulse_renag of 12m", got)
	}
	// And an explicit option outranks both, which is what the tests above
	// lean on.
	if got := pulseWaitLife(a, PulseWaitOptions{Renag: time.Second}); got != time.Second {
		t.Errorf("life = %v, want the option's 1s", got)
	}
}
