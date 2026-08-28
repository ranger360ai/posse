package rhq

// The herdr event-hint adapter (ADR 0016 §1), and the settle channel ADR
// 0028 §1 fires refills from. One Unix-socket connection per long-running
// process, one `events.subscribe` request on it, and a capacity-one channel
// of hints out.
//
// A hint is never a fact. Every consumer re-reads its existing truth path
// (bd, herdr listings) before acting, so a duplicated, replayed, coalesced
// or lost event costs latency and never correctness — which is what lets
// this file be as small as it is: no cursor, no checkpoint, no replay
// request, no persisted subscriber state.
//
// MEASURED against the installed herdr 0.8.0, socket protocol 19
// (2026-08-27, ranger-base-0s36) — every one of these was probed on the
// live socket, and the first is why this file does not look like ADR 0016
// §1's selector list:
//
//   - `pane.agent_status_changed` is REFUSED unscoped: the request schema
//     requires `pane_id`, and the server answers
//     `invalid_request: missing field "pane_id"` and then CLOSES the
//     connection. ADR 0016 §1's "the installed schema accepts unscoped
//     subscriptions" does not hold for the one event the watch loop wants,
//     and its rejected alternative — scoping to today's pane ids, with a
//     mutable registry to keep them current — is the only way to subscribe
//     to it directly.
//   - `pane.updated` IS accepted unscoped and carries the whole PaneInfo,
//     `agent_status` and `workspace_id` included. So the settle signal is
//     recovered by edge detection: the stream is level-triggered (every
//     revision bump repeats the current status), and settleGate turns it
//     into the working→idle/done edge 0016 §2 asks for. This is the
//     substitution, not a new mechanism: the same event, the same status
//     field, one map of last-seen levels.
//   - `workspace.created`, `workspace.closed`, `pane.agent_detected` are
//     all accepted unscoped, exactly as 0016 §1 assumed.
//   - Subscriptions are spelled dotted (`pane.updated`); the envelopes
//     pushed back are spelled with underscores (`pane_updated`). Both
//     spellings are folded to one internal kind.
//
// The divergence is filed for the architect (ranger-base-0s36 → richard);
// this file implements what herdr actually accepts.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// HerdrHint is one wake hint: an event kind and the little of its payload
// posse is willing to look at. Nothing here is applied to posse state.
type HerdrHint struct {
	Kind        string // canonical (underscore) event kind
	WorkspaceID string
	PaneID      string
	AgentStatus string
}

func (h HerdrHint) String() string {
	where := h.PaneID
	if where == "" {
		where = h.WorkspaceID
	}
	s := h.Kind
	if where != "" {
		s += " " + where
	}
	if h.AgentStatus != "" {
		s += " " + h.AgentStatus
	}
	return s
}

// HerdrHintSubscriptions is what the adapter asks the socket for, in the
// dotted spelling `events.subscribe` takes. Unfiltered by pane and
// workspace deliberately: a subscription scoped to today's ids would miss
// everything created after it and would need a second mutable registry to
// stay current (ADR 0016 §1).
var HerdrHintSubscriptions = []string{
	"pane.updated",
	"pane.agent_detected",
	"workspace.created",
	"workspace.closed",
}

// herdrHintRetry is how long a dead subscription waits before redialling.
// The consumer's own timer is still armed throughout, so this delay costs
// latency on a hint and nothing else (ADR 0016 §1).
const herdrHintRetry = 5 * time.Second

// SettledStatuses are the agent states this shop reads as "the seat's turn
// is over" for hint purposes (ADR 0016 §2). `blocked` is deliberately not
// here: dispatch's own AgentWait settles on it, but a blocked agent still
// holds its bead, so waking a pass for one buys nothing.
var SettledStatuses = []string{"idle", "done"}

// HerdrSettleHints subscribes to herdr and yields one hint per settle: a
// pane whose agent status changed into idle or done, or a workspace that
// closed. The channel has capacity one — a burst means "look again" once,
// not N times — and is closed when ctx ends.
//
// report gets one line per transition: an outage when the stream dies, a
// recovery when it comes back. It is never fatal; with no herdr at all the
// caller simply never receives a hint (ADR 0016 §2).
func HerdrSettleHints(ctx context.Context, sock string, report func(string)) <-chan HerdrHint {
	return herdrHints(ctx, sock, herdrHintRetry, settleGate(), report)
}

func herdrHints(ctx context.Context, sock string, retry time.Duration, want func(HerdrHint) bool, report func(string)) <-chan HerdrHint {
	out := make(chan HerdrHint, 1)
	go func() {
		defer close(out)
		down := false
		for ctx.Err() == nil {
			err := streamHerdrHints(ctx, sock, out, want, func() {
				if down {
					report("herdr events restored")
					down = false
				}
			})
			if ctx.Err() != nil {
				return
			}
			if !down {
				down = true
				report(fmt.Sprintf("herdr events unavailable — polling (%v)", err))
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(retry):
			}
		}
	}()
	return out
}

// streamHerdrHints owns one connection for its whole life: dial, subscribe,
// verify the acknowledgement, then decode until something goes wrong. Every
// exit is an error the caller retries — including a clean EOF, because a
// subscription that ended is a subscription that is no longer delivering.
func streamHerdrHints(ctx context.Context, sock string, out chan<- HerdrHint, want func(HerdrHint) bool, up func()) error {
	if sock == "" {
		return fmt.Errorf("no herdr socket to subscribe to")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	// Cancellation closes the socket under the decoder; a blocking Decode
	// is the whole reason this needs a helper rather than a ctx check.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}
	}()

	id := fmt.Sprintf("posse-hints-%d", os.Getpid())
	subs := make([]map[string]string, 0, len(HerdrHintSubscriptions))
	for _, t := range HerdrHintSubscriptions {
		subs = append(subs, map[string]string{"type": t})
	}
	req, err := json.Marshal(map[string]any{
		"id":     id,
		"method": "events.subscribe",
		"params": map[string]any{"subscriptions": subs},
	})
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(req, '\n')); err != nil {
		return err
	}

	dec := json.NewDecoder(conn)
	var ack struct {
		ID     string `json:"id"`
		Result struct {
			Type string `json:"type"`
		} `json:"result"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := dec.Decode(&ack); err != nil {
		return fmt.Errorf("events.subscribe: no answer: %w", err)
	}
	switch {
	case ack.Error != nil:
		return fmt.Errorf("events.subscribe refused: %s", ack.Error.Message)
	case ack.ID != id:
		return fmt.Errorf("events.subscribe answered id %q, not %q", ack.ID, id)
	case ack.Result.Type != "subscription_started":
		return fmt.Errorf("events.subscribe answered %q, not subscription_started", ack.Result.Type)
	}
	up()

	for {
		var env herdrEventEnvelope
		if err := dec.Decode(&env); err != nil {
			return err
		}
		h := env.hint()
		if h.Kind == "" || !want(h) {
			continue
		}
		// Capacity-one, never blocking: the reader must keep draining the
		// socket whatever the consumer is doing, or a slow pass would stall
		// herdr's writer (ADR 0016 §1).
		select {
		case out <- h:
		default:
		}
	}
}

// herdrEventEnvelope is the pushed shape, decoded down to the four fields
// posse looks at. Unknown fields and unknown kinds are ignored by
// construction — an envelope this cannot read yields an empty Kind.
type herdrEventEnvelope struct {
	Event string `json:"event"`
	Data  struct {
		PaneID      string          `json:"pane_id"`
		WorkspaceID string          `json:"workspace_id"`
		AgentStatus string          `json:"agent_status"`
		FinalStatus string          `json:"final_status"`
		Pane        *herdrEventNode `json:"pane"`
		Workspace   *herdrEventNode `json:"workspace"`
	} `json:"data"`
}

// herdrEventNode is the common shape of the `pane` and `workspace` objects
// nested in an event payload.
type herdrEventNode struct {
	PaneID      string `json:"pane_id"`
	WorkspaceID string `json:"workspace_id"`
	AgentStatus string `json:"agent_status"`
}

func (e herdrEventEnvelope) hint() HerdrHint {
	if e.Event == "" {
		return HerdrHint{}
	}
	// Protocol 19 spells subscriptions dotted and pushes underscored; both
	// reach the same internal kind so neither spelling is load-bearing.
	h := HerdrHint{
		Kind:        strings.ReplaceAll(e.Event, ".", "_"),
		PaneID:      e.Data.PaneID,
		WorkspaceID: e.Data.WorkspaceID,
		AgentStatus: e.Data.AgentStatus,
	}
	if h.AgentStatus == "" {
		h.AgentStatus = e.Data.FinalStatus
	}
	for _, n := range []*herdrEventNode{e.Data.Pane, e.Data.Workspace} {
		if n == nil {
			continue
		}
		if h.PaneID == "" {
			h.PaneID = n.PaneID
		}
		if h.WorkspaceID == "" {
			h.WorkspaceID = n.WorkspaceID
		}
		if h.AgentStatus == "" {
			h.AgentStatus = n.AgentStatus
		}
	}
	if h.WorkspaceID == "" && h.PaneID != "" {
		// Pane ids are "<workspace>:<pane>"; an envelope that named only
		// the pane still says which seat it was.
		if i := strings.Index(h.PaneID, ":"); i > 0 {
			h.WorkspaceID = h.PaneID[:i]
		}
	}
	return h
}

// settleGate turns herdr's level-triggered stream into edges. `pane.updated`
// repeats the current agent status on every revision bump, so "settled" is
// the transition INTO idle/done, held per pane in a map of last-seen levels.
//
// The first sighting of a pane is a level, not a transition, and is not a
// hint: a subscriber that started while eleven seats sat idle would
// otherwise open with eleven wake-ups saying nothing happened. A seat that
// settles later still fires, and the consumer's timer covers whatever the
// edge missed.
//
// The closure is not safe for concurrent use; one subscriber owns one gate.
func settleGate() func(HerdrHint) bool {
	last := map[string]string{}
	return func(h HerdrHint) bool {
		switch h.Kind {
		case "workspace_closed":
			// A seat that went away has settled in the only sense that
			// matters here: it is no longer busy.
			return true
		case "pane_closed", "pane_exited":
			delete(last, h.PaneID)
			return false
		}
		if h.PaneID == "" || h.AgentStatus == "" {
			return false
		}
		prev, seen := last[h.PaneID]
		last[h.PaneID] = h.AgentStatus
		if !seen || prev == h.AgentStatus {
			return false
		}
		return isSettledStatus(h.AgentStatus)
	}
}

func isSettledStatus(s string) bool {
	for _, ok := range SettledStatuses {
		if s == ok {
			return true
		}
	}
	return false
}
