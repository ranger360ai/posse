package rhq

// Live pin for ranger-base-0s36: what the installed herdr's events socket
// actually accepts. herdrevents_test.go proves what posse DOES with the
// stream; only a real server can say which subscriptions exist, and this is
// the measurement herdrevents.go's selector list is set from — including the
// one that contradicts ADR 0016 §1.
//
//	RHQ_LIVE_HERDR_EVENTS=1 go test ./internal/rhq -run TestLiveHerdrEventSelectors -v
//
// It subscribes and reads. It creates nothing, types nothing at any pane,
// and spends no API turn, so it is safe against the live fleet — but it is
// skipped by default, because `go test ./...` must never reach a real herdr
// (ADR 0016 §3).
//
// MEASURED 2026-08-27, herdr 0.8.0, socket protocol 19:
//
//	pane.agent_status_changed   unscoped → invalid_request "missing field
//	                            `pane_id`", and the server closes the socket
//	pane.updated                unscoped → subscription_started, then the
//	                            whole PaneInfo (agent_status, workspace_id)
//	                            on every revision bump
//	workspace.created/.closed   unscoped → subscription_started
//	pane.agent_detected         unscoped → subscription_started
//	pushed envelopes            underscored (`pane_updated`), where the
//	                            subscription is dotted (`pane.updated`)

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

// liveSubscribe sends one events.subscribe for types and returns the raw
// first answer.
func liveSubscribe(t *testing.T, sock string, types ...string) string {
	t.Helper()
	conn, err := net.DialTimeout("unix", sock, 5*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", sock, err)
	}
	t.Cleanup(func() { conn.Close() })
	subs := make([]map[string]string, 0, len(types))
	for _, ty := range types {
		subs = append(subs, map[string]string{"type": ty})
	}
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
		t.Fatalf("read answer to %v: %v", types, err)
	}
	return strings.TrimSpace(line)
}

func TestLiveHerdrEventSelectors(t *testing.T) {
	if os.Getenv("RHQ_LIVE_HERDR_EVENTS") == "" {
		t.Skip("set RHQ_LIVE_HERDR_EVENTS=1 (and HERDR_SOCKET_PATH for a scratch server) — see the file comment")
	}
	sock := herdrSocketPath()
	if _, err := os.Stat(sock); err != nil {
		t.Skipf("no herdr socket at %s: %v", sock, err)
	}

	// The refusal ADR 0016 §1 did not have: this is why the settle signal
	// comes off pane.updated. If a later herdr accepts it unscoped, this
	// test goes red and the substitution can be retired.
	if got := liveSubscribe(t, sock, "pane.agent_status_changed"); !strings.Contains(got, "invalid_request") ||
		!strings.Contains(got, "pane_id") {
		t.Errorf("herdr answered pane.agent_status_changed unscoped with %s\n"+
			"— it used to refuse it for want of a pane_id, which is the whole reason posse subscribes to pane.updated instead", got)
	}

	// The selectors posse does use, each one on its own connection: a
	// refusal closes the socket, so a list would hide which member failed.
	for _, ty := range HerdrHintSubscriptions {
		got := liveSubscribe(t, sock, ty)
		if !strings.Contains(got, "subscription_started") {
			t.Errorf("herdr will not subscribe to %s unscoped: %s", ty, got)
		}
	}

	// And the stream really does carry the status level posse edge-detects,
	// in the underscored spelling.
	conn, err := net.DialTimeout("unix", sock, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	req, _ := json.Marshal(map[string]any{
		"id": "posse-live-stream", "method": "events.subscribe",
		"params": map[string]any{"subscriptions": []map[string]string{{"type": "pane.updated"}}},
	})
	conn.Write(append(req, '\n'))
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	dec := json.NewDecoder(conn)
	var ack struct {
		Result struct{ Type string } `json:"result"`
	}
	if err := dec.Decode(&ack); err != nil || ack.Result.Type != "subscription_started" {
		t.Fatalf("subscribe to pane.updated: %v (%+v)", err, ack)
	}
	// A live fleet bumps pane revisions constantly; a quiet one may not, and
	// a timeout here is "nothing happened", not a defect.
	var env herdrEventEnvelope
	if err := dec.Decode(&env); err != nil {
		t.Skipf("no pane event within 30s (a quiet server): %v", err)
	}
	h := env.hint()
	if h.Kind != "pane_updated" {
		t.Errorf("pushed kind = %q, want the underscored spelling", h.Kind)
	}
	if h.PaneID == "" || h.WorkspaceID == "" || h.AgentStatus == "" {
		t.Errorf("pane.updated must carry the level posse edge-detects; got %+v", h)
	}
	fmt.Fprintf(os.Stderr, "live pane.updated: %+v\n", h)
}
