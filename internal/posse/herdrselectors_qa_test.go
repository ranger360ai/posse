package posse

// QA pin — the event-hint subscribe request's SELECTORS, by literal
// (ranger-base-b8ik4, verifying the close of ranger-base-kmcd).
//
// ranger-base-kmcd's acceptance criterion is "hermetic tests validate the
// exact request". The request's SHAPE is validated by
// TestHerdrHintsSubscribeRequest — method, an id to check the
// acknowledgement against, lifecycle entries unscoped, one deduplicated
// pane_id per pane. Its CONTENT was not: that test's lifecycle assertion
// reads
//
//	strings.Join(lifecycle, ",") != strings.Join(HerdrLifecycleSubscriptions, ",")
//
// and the production request is built by ranging over that same package
// variable, so the comparison is a tautology for every value the variable
// can take. Measured, not argued: with `pane.agent_detected` deleted from
// the slice the whole herdr-events set stayed green, and so did the live
// control arm — herdrevents_live_test.go:99 ranges over the same variable,
// subscribes to whatever it holds, and is acknowledged.
//
// What a silent drift costs is not cosmetic. ADR 0016 §1 makes
// `pane.agent_detected` the only announcement that the pane set moved, and
// a connection takes exactly one subscribe — so losing that selector does
// not lose one event kind, it strands every seat that appears after the
// dial on the timer sweep, which is the latency this whole ADR exists to
// remove. `workspace.created` is the cockpit's other redial trigger.
//
// So the three selectors are written out here as LITERALS. This file is the
// claim; herdrevents.go is the implementation; a protocol change has to
// come through both, which is the point.
//
// Both halves are asserted with the ADR text as the second reader, and the
// docs half carries its own floor: a check that "every literal appears in
// the ADR" is satisfied by an ADR nobody managed to read, so the byte count
// is asserted before the strings are.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The exact wire selectors of ADR 0016 §1, in the order the request emits
// them. Spelled dotted, which is what `events.subscribe` takes — the
// envelopes herdr pushes back are a different spelling and are folded in
// herdrEventEnvelope.hint().
var qhsLifecycle = []string{
	"pane.agent_detected",
	"workspace.created",
	"workspace.closed",
}

const qhsPane = "pane.agent_status_changed"

func TestHerdrSelectorsAreTheADRsLiterals(t *testing.T) {
	t.Parallel()
	if got := strings.Join(HerdrLifecycleSubscriptions, ","); got != strings.Join(qhsLifecycle, ",") {
		t.Errorf("HerdrLifecycleSubscriptions = %v, want ADR 0016 §1's three: %v", HerdrLifecycleSubscriptions, qhsLifecycle)
	}
	if HerdrPaneSubscription != qhsPane {
		t.Errorf("HerdrPaneSubscription = %q, want %q", HerdrPaneSubscription, qhsPane)
	}
}

// The variable above is what the request is BUILT from; this is what goes
// out on the socket. They are pinned apart because herdrSubscriptions can
// drop, reorder or rewrite an entry on its way to the wire without the
// variable moving at all.
func TestHerdrSubscribeRequestCarriesTheLiteralSelectors(t *testing.T) {
	t.Parallel()
	s := newHintServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	herdrHints(ctx, s.path, 20*time.Millisecond, panesAre("w1:p1", "w2:p1"), nil, isSettleHint, func(string) {})

	var req struct {
		Params struct {
			Subscriptions []map[string]string `json:"subscriptions"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(s.subscribed()), &req); err != nil {
		t.Fatalf("the request must be one line of JSON: %v", err)
	}
	var lifecycle, scoped []string
	for _, sub := range req.Params.Subscriptions {
		if sub["type"] == qhsPane {
			scoped = append(scoped, sub["pane_id"])
			continue
		}
		lifecycle = append(lifecycle, sub["type"])
	}
	if got := strings.Join(lifecycle, ","); got != strings.Join(qhsLifecycle, ",") {
		t.Errorf("the request's unscoped selectors = %v, want %v", lifecycle, qhsLifecycle)
	}
	// The settle selector reaching the wire at all is half of the mixed
	// request; the other half is that it is scoped, which
	// TestHerdrHintsSubscribeRequest already pins.
	if got := strings.Join(scoped, ","); got != "w1:p1,w2:p1" {
		t.Errorf("the request's %s entries = %v, want one per pane", qhsPane, scoped)
	}
}

// The ADR is the other reader of these four strings. If a protocol change
// moves the code and leaves the page saying something else, the page is the
// one an operator reads before believing what posse subscribes to.
func TestHerdrSelectorsAreNamedByADR0016(t *testing.T) {
	t.Parallel()
	path := filepath.Join(qibRepoRoot(t), "docs", "adr", "0016-herdr-event-hints.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	// The floor first: "every literal is in the page" is true of a page
	// that failed to arrive, so say how much page was actually read.
	const qhsADRFloor = 4000
	if len(body) < qhsADRFloor {
		t.Fatalf("only %d bytes of ADR 0016 read (floor %d) — the check below would pass on nothing", len(body), qhsADRFloor)
	}
	for _, sel := range append(append([]string(nil), qhsLifecycle...), qhsPane) {
		if !strings.Contains(string(body), sel) {
			t.Errorf("ADR 0016 never names the selector %q that posse subscribes to", sel)
		}
	}
}
