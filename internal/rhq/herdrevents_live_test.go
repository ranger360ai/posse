package rhq

// Live pin for ranger-base-0s36: what the installed herdr's events socket
// actually accepts. herdrevents_test.go proves what posse DOES with the
// stream; only a real server can say which subscriptions exist, and this is
// the measurement herdrevents.go's request shape is built from — including
// the two facts that contradict ADR 0016 §1.
//
//	RHQ_LIVE_HERDR_EVENTS=1 go test ./internal/rhq -run TestLiveHerdrEvent -v
//
// It subscribes and reads. It creates nothing, types nothing at any pane,
// and spends no API turn, so it is safe against the live fleet — but it is
// skipped by default, because `go test ./...` must never reach a real herdr
// (ADR 0016 §3).
//
// MEASURED 2026-08-27, herdr 0.8.0, socket protocol 19:
//
//	pane.agent_status_changed   unscoped → invalid_request "missing field
//	                            `pane_id`", and the server closes the socket;
//	                            pane-scoped → subscription_started
//	pane.updated                unscoped → subscription_started, and ~4
//	                            envelopes/second on a five-seat fleet — but
//	                            NOT the settle: a settling pane stops
//	                            emitting, and the transition to idle/done
//	                            arrives only on pane.agent_status_changed
//	                            (observed on two seats, 2026-08-27 22:31 and
//	                            22:35)
//	workspace.created/.closed   unscoped → subscription_started
//	pane.agent_detected         unscoped → subscription_started
//	two subscribes on one conn  the second is not answered; the server
//	                            closes the connection
//	an unknown pane id          pane_not_found, and the connection closes —
//	                            one stale id costs every subscription on it
//	spelling                    lifecycle envelopes come back underscored
//	                            (pane_updated), pane-scoped ones dotted
//	                            (pane.agent_status_changed)

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// liveSubscribe sends one events.subscribe and returns the raw first answer.
func liveSubscribe(t *testing.T, sock string, subs ...map[string]string) string {
	t.Helper()
	conn, err := net.DialTimeout("unix", sock, 5*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", sock, err)
	}
	t.Cleanup(func() { conn.Close() })
	req, _ := json.Marshal(map[string]any{
		"id": "posse-live-probe", "method": "events.subscribe",
		"params": map[string]any{"subscriptions": subs},
	})
	if _, err := conn.Write(append(req, '\n')); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read answer to %v: %v", subs, err)
	}
	return strings.TrimSpace(line)
}

func liveSocket(t *testing.T) string {
	t.Helper()
	if os.Getenv("RHQ_LIVE_HERDR_EVENTS") == "" {
		t.Skip("set RHQ_LIVE_HERDR_EVENTS=1 (and HERDR_SOCKET_PATH for a scratch server) — see the file comment")
	}
	sock := herdrSocketPath()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no herdr socket at %s: %v", sock, err)
	}
	return sock
}

func TestLiveHerdrEventSelectors(t *testing.T) {
	sock := liveSocket(t)

	// The refusal ADR 0016 §1 did not have: this is why the settle
	// subscription is pane-scoped, and why the registry the ADR rejected is
	// not optional. If a later herdr accepts it unscoped, this goes red and
	// the registry can be retired.
	if got := liveSubscribe(t, sock, map[string]string{"type": HerdrPaneSubscription}); !strings.Contains(got, "invalid_request") ||
		!strings.Contains(got, "pane_id") {
		t.Errorf("herdr answered %s unscoped with %s\n"+
			"— it used to refuse it for want of a pane_id, which is the whole reason posse subscribes per pane", HerdrPaneSubscription, got)
	}

	// The lifecycle selectors, each on its own connection: a refusal closes
	// the socket, so a list would hide which member failed.
	for _, ty := range HerdrLifecycleSubscriptions {
		if got := liveSubscribe(t, sock, map[string]string{"type": ty}); !strings.Contains(got, "subscription_started") {
			t.Errorf("herdr will not subscribe to %s unscoped: %s", ty, got)
		}
	}

	// A pane id herdr does not know refuses the request that carries it.
	// Posse answers this by re-reading the pane list on the next dial, so a
	// changed error code here is a change in what "stale pane" costs.
	if got := liveSubscribe(t, sock,
		map[string]string{"type": HerdrPaneSubscription, "pane_id": "wNOSUCH:p9"}); !strings.Contains(got, "pane_not_found") {
		t.Errorf("an unknown pane id must be refused, and named: %s", got)
	}

	// And the real request shape posse sends, against the panes this herdr
	// really has: lifecycle unfiltered plus one scoped subscription each,
	// all in ONE request, because a second one on the same connection is
	// not answered.
	b := &HerdrBackend{App: NewAppAt(t.TempDir()), H: NewHerdr()}
	panes := b.AgentPanes()
	if len(panes) == 0 {
		t.Skip("this herdr reports no agent panes — nothing to scope a subscription to")
	}
	subs := herdrSubscriptions(func() []string { return panes })
	if got := liveSubscribe(t, sock, subs...); !strings.Contains(got, "subscription_started") {
		t.Errorf("posse's own subscription (%d panes) was refused: %s", len(panes), got)
	}
}

// The settle really does arrive, on the pane-scoped subscription, and it
// decodes into a hint. Needs a seat to finish its turn, so it waits — run it
// while the fleet is turning over.
func TestLiveHerdrEventSettleArrives(t *testing.T) {
	sock := liveSocket(t)
	b := &HerdrBackend{App: NewAppAt(t.TempDir()), H: NewHerdr()}
	panes := b.AgentPanes()
	if len(panes) == 0 {
		t.Skip("no agent panes to watch")
	}
	conn, err := net.DialTimeout("unix", sock, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	req, _ := json.Marshal(map[string]any{
		"id": "posse-live-settle", "method": "events.subscribe",
		"params": map[string]any{"subscriptions": herdrSubscriptions(func() []string { return panes })},
	})
	conn.Write(append(req, '\n'))
	wait := 10 * time.Minute
	if s := os.Getenv("RHQ_LIVE_HERDR_WAIT"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			wait = d
		}
	}
	conn.SetReadDeadline(time.Now().Add(wait))
	dec := json.NewDecoder(conn)
	var ack struct {
		Result struct{ Type string } `json:"result"`
	}
	if err := dec.Decode(&ack); err != nil || ack.Result.Type != "subscription_started" {
		t.Fatalf("subscribe: %v (%+v)", err, ack)
	}
	for {
		var env herdrEventEnvelope
		if err := dec.Decode(&env); err != nil {
			t.Skipf("no seat settled within %s: %v", wait, err)
		}
		h := env.hint()
		if !isSettleHint(h) {
			continue
		}
		fmt.Fprintf(os.Stderr, "live settle: %+v\n", h)
		if h.Kind == "pane_agent_status_changed" && (h.PaneID == "" || h.WorkspaceID == "") {
			t.Errorf("a settle must name its seat: %+v", h)
		}
		return
	}
}
