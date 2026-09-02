package posse

// ranger-base-e9d9, the other half: the loop can only honour a drain it is
// TOLD about. internal/posse pins what a cancelled context does to a pass in
// flight (TestQADrainEndsAWatchLoopWaitingOnASession); this pins that both
// signals an operator actually sends reach that context.
//
// A census and not a behavioural test on purpose: delivering a real signal to
// the test binary would stop `go test`, and the mechanism under it is
// stdlib. What can rot here is the wiring — one signal dropped from the
// Notify list, or a `context.Background()` handed to Watch beside a cancel
// nothing carries — and that is exactly what a reader of the source can see.
//
// MUTATION RUN: drop syscall.SIGTERM from the list → red; hand Watch
// context.Background() instead of ctx → red.

import (
	"os"
	"strings"
	"testing"
)

func TestQADispatchWatchWiresBothDrainSignals(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("cmd/posse/main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	// The --watch branch, and nothing else: `posse dispatch` without an
	// interval is a one-shot pass that ends on its own, and the cockpit has
	// its own handlers for its own reasons.
	at := strings.Index(src, "if watch > 0 {")
	if at < 0 {
		t.Fatal("fixture: the --watch branch is no longer `if watch > 0 {` in cmd/posse/main.go — this census is reading past it and asserting nothing")
	}
	end := strings.Index(src[at:], "\n\t\tn, err := d.Run(")
	if end < 0 {
		t.Fatal("fixture: cannot find the end of the --watch branch (the one-shot Run below it) — the window below is unbounded")
	}
	branch := src[at : at+end]

	for _, want := range []string{
		"signal.NotifyContext(", // the context IS the signal
		"os.Interrupt",          // ctrl-c, and SIGINT from a drain script
		"syscall.SIGTERM",       // what `kill` and a shutdown send by default
		"d.Watch(ctx,",          // …and it is the context the loop is given
	} {
		if !strings.Contains(branch, want) {
			t.Errorf("dispatch --watch must drain on TERM and INT: %q missing from the --watch branch:\n%s", want, branch)
		}
	}
}
