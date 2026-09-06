//go:build posse_arm3

package posse

// ranger-base-inomb, verifying the close of ranger-base-ymec: ADR
// 0036 §4's ticker, "starts with Watch's ctx, dies with it".
//
// "Dies with it" is JOINED, not merely signalled, and backuploop.go's own
// header says so in as many words — a tick already inside RunBackup is
// staging tens of megabytes under this instance's state/, so a Watch that
// returned on the cancel alone would leave a goroutine writing after its
// caller believed the loop was over. That is the pulse's ranger-base-el3g
// failure ("TempDir RemoveAll cleanup: ... state: directory not empty"),
// which the pulse pins with TestWatchWaitsForAPulseTickInFlight.
//
// The backup clock had no such pin. MEASURED 2026-09-02: deleting the
// `<-backupDone` receive from watch.go — the whole of the join, one line —
// left every test in backuploop_test.go green, and `backup_interval:`
// appears in no other test file in the tree, so nothing else could see it
// either. This is that arm, written as the pulse's twin.
//
// Where it parks: the tick emits nothing until RunBackup has returned (the
// verb narrates into a strings.Builder the tick flushes afterwards), so the
// hold is on the tick's own "backup · scheduled ·" line, with the goroutine
// still inside backupTick. That is enough to make the join the only thing
// keeping Watch inside Watch, which is what this pin is about.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// backupParkTap is parkTap's shape (pulse_test.go) matching the backup
// tick's line instead of the pulse's. It holds the FIRST writer of a
// "backup · scheduled ·" line and lets every other write through: the
// buffer lock is released BEFORE the park, so a parked tick cannot be what
// keeps another goroutine out of the sink.
type backupParkTap struct {
	mu      sync.Mutex
	buf     strings.Builder
	once    sync.Once
	freed   sync.Once
	parked  chan struct{}
	release chan struct{}
}

func newBackupParkTap() *backupParkTap {
	return &backupParkTap{parked: make(chan struct{}), release: make(chan struct{})}
}

func (p *backupParkTap) let() { p.freed.Do(func() { close(p.release) }) }

func (p *backupParkTap) Write(b []byte) (int, error) {
	p.mu.Lock()
	n, err := p.buf.Write(b)
	p.mu.Unlock()
	if strings.Contains(string(b), "backup · scheduled ·") {
		held := false
		p.once.Do(func() { held = true; close(p.parked) })
		if held {
			<-p.release
		}
	}
	return n, err
}

func (p *backupParkTap) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.String()
}

func TestWatchWaitsForABackupTickInFlight(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	write(t, b.App.ConfigPath, "queue_repo: "+backupClockQueue(t)+"\nbackup_interval: 2s\nbeads:\n  - "+repo+"\n")

	tap := newBackupParkTap()
	d.Out = tap

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer tap.let() // never leave the tick parked, however this ends
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond)
	}()

	select {
	case <-tap.parked:
	case <-time.After(60 * time.Second):
		t.Fatalf("the backup clock never wrote a scheduled line to park on:\n%s", tap.String())
	}

	cancel()
	select {
	case <-done:
		t.Fatal("Watch returned with a backup tick still in flight: the tick goes on writing this instance's state/ after the caller believes the loop is over (ADR 0036 §4, the pulse's ranger-base-el3g one file over)")
	case <-time.After(500 * time.Millisecond):
	}

	tap.let()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("Watch never returned after the parked backup tick was released")
	}
}
