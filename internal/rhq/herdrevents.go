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
// (2026-08-27, ranger-base-0s36), on the live socket. These five are why the
// file is shaped the way it is, and every one of them contradicts something
// it would have been reasonable to assume:
//
//  1. `pane.agent_status_changed` — the settle event — CANNOT be subscribed
//     unscoped. The schema requires `pane_id`; without one the server
//     answers `invalid_request: missing field "pane_id"` and closes the
//     connection. ADR 0016 §1's "the installed schema accepts unscoped
//     subscriptions" does not hold for it, so its rejected alternative,
//     a pane registry, is the only route there is.
//  2. `pane.updated` IS accepted unscoped and carries the whole PaneInfo,
//     `agent_status` included — but it does NOT carry the settle. A working
//     pane emits it several times a second (3707 envelopes in 15 minutes on
//     a five-seat fleet); a settling one simply stops emitting, and the
//     status change rides only on `pane.agent_status_changed`. Measured
//     twice, on two seats, minutes apart: the settle never appeared on
//     `pane.updated`, before or after. Level-polling that stream is not a
//     substitute for the edge, it is a way to never see one.
//  3. A connection takes exactly ONE `events.subscribe`. A second request
//     on a live connection is not answered — the server closes it. So the
//     pane set cannot be amended in place; changing it means reconnecting.
//  4. One request may mix unscoped and pane-scoped subscriptions freely,
//     and is answered by a single `subscription_started`.
//  5. A pane id herdr does not know is refused (`pane_not_found`) and takes
//     the whole connection with it — one stale id in the list costs every
//     subscription on it. The list is therefore re-read on every dial, and
//     a refusal is just another reason to redial.
//
// Hence: subscribe unscoped to the three lifecycle events, pane-scoped to
// `pane.agent_status_changed` for every pane herdr currently reports an
// agent in, and treat `pane.agent_detected` and `workspace.created` as
// "the pane set moved" — reconnect with a fresh list rather than miss a
// seat that appeared after the subscription. The reconnect is planned, not
// an outage, and says nothing in the log.
//
// The scoping divergence is filed for the architect (ranger-base-7t1w).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
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

// HerdrLifecycleSubscriptions are the events posse takes unfiltered, in the
// dotted spelling `events.subscribe` wants. The first two say the pane set
// moved; the third is a settle in its own right.
var HerdrLifecycleSubscriptions = []string{
	"pane.agent_detected",
	"workspace.created",
	"workspace.closed",
}

// HerdrPaneSubscription is the settle event, and it is only subscribable one
// pane at a time (measurement 1).
const HerdrPaneSubscription = "pane.agent_status_changed"

// herdrHintRetry is how long a dead subscription waits before redialling.
// The consumer's own timer is still armed throughout, so this delay costs
// latency on a hint and nothing else (ADR 0016 §1).
const herdrHintRetry = 5 * time.Second

// SettledStatuses are the agent states this shop reads as "the seat's turn
// is over" (ADR 0016 §2). `blocked` is deliberately not here: dispatch's own
// AgentWait settles on it, but a blocked agent still holds its bead, so
// waking a pass for one buys nothing.
var SettledStatuses = []string{"idle", "done"}

// errRefreshPanes is a planned reconnect: the pane set moved, and the
// subscription that is up was fixed at dial time. Not an outage — nothing
// failed, and nothing is reported.
var errRefreshPanes = errors.New("pane set changed")

// HerdrSettleHints subscribes to herdr and yields one hint per settle: a
// pane whose agent status changed to idle or done, or a workspace that
// closed. The channel has capacity one — a burst means "look again" once,
// not N times — and is closed when ctx ends.
//
// panes reports the panes herdr currently has an agent in; it is re-asked on
// every dial, because a stale id refuses the whole subscription
// (measurement 5). A nil or failing panes costs the settle events and leaves
// the lifecycle ones, which is degraded, not broken.
//
// refresh is the level-triggered belt on that pane set: a poke redials with
// a fresh list. The consumer pokes it after every pass, because the pass is
// what actually knows the shop changed — MEASURED (2026-08-27): a seat that
// appeared after a subscription was dialled had its settle missed, and no
// `pane.agent_detected` or `workspace.created` arrived to say so. The
// event-driven refresh below is kept because it is free when it does fire;
// this is what makes the set eventually right either way.
//
// report gets one line per transition: an outage when the stream dies, a
// recovery when it comes back. It is never fatal; with no herdr at all the
// caller simply never receives a hint (ADR 0016 §2).
func HerdrSettleHints(ctx context.Context, sock string, panes func() []string, refresh <-chan struct{}, report func(string)) <-chan HerdrHint {
	return herdrHints(ctx, sock, herdrHintRetry, panes, refresh, isSettleHint, report)
}

func herdrHints(ctx context.Context, sock string, retry time.Duration, panes func() []string, refresh <-chan struct{}, want func(HerdrHint) bool, report func(string)) <-chan HerdrHint {
	out := make(chan HerdrHint, 1)
	go func() {
		defer close(out)
		down := false
		for ctx.Err() == nil {
			err := streamHerdrHints(ctx, sock, panes, refresh, out, want, func() {
				if down {
					report("herdr events restored")
					down = false
				}
			})
			if ctx.Err() != nil {
				return
			}
			// The pane set moved under a subscription that was fixed when
			// it was dialled: redial at once, and say nothing — this is the
			// adapter keeping up, not herdr failing.
			if errors.Is(err, errRefreshPanes) {
				continue
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

// streamHerdrHints owns one connection for its whole life: dial, subscribe
// to the pane set as it stands right now, verify the acknowledgement, then
// decode until something goes wrong. Every exit is an error the caller
// answers by redialling — including a clean EOF, because a subscription that
// ended is a subscription that is no longer delivering.
func streamHerdrHints(ctx context.Context, sock string, panes func() []string, refresh <-chan struct{}, out chan<- HerdrHint, want func(HerdrHint) bool, up func()) error {
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
	// is the whole reason this needs a helper rather than a ctx check. A
	// refresh poke closes it the same way, flagged so the redial is read as
	// planned rather than as an outage.
	stop := make(chan struct{})
	defer close(stop)
	var poked atomic.Bool
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-refresh:
			poked.Store(true)
			conn.Close()
		case <-stop:
		}
	}()

	// Everything from here on can be interrupted by a poke or a cancel,
	// both of which close the socket under whatever is blocked on it. A
	// poked failure is the adapter keeping up, not herdr failing, so it must
	// never come out as an outage — including in the window between writing
	// the subscribe and reading its acknowledgement, which is exactly where
	// a pass-end poke lands (seen live, 2026-08-27).
	fail := func(err error) error {
		if poked.Load() {
			return errRefreshPanes
		}
		return err
	}

	id := fmt.Sprintf("posse-hints-%d", os.Getpid())
	req, err := json.Marshal(map[string]any{
		"id":     id,
		"method": "events.subscribe",
		"params": map[string]any{"subscriptions": herdrSubscriptions(panes)},
	})
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(req, '\n')); err != nil {
		return fail(err)
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
		return fail(fmt.Errorf("events.subscribe: no answer: %w", err))
	}
	switch {
	case ack.Error != nil:
		// A pane that went away between the listing and the subscribe is
		// the ordinary case here, and the next dial re-reads the list — so
		// this is not worth an outage line either.
		if ack.Error.Code == "pane_not_found" {
			return fmt.Errorf("%w: %s", errRefreshPanes, ack.Error.Message)
		}
		return fmt.Errorf("events.subscribe refused: %s", ack.Error.Message)
	case !strings.HasPrefix(ack.ID, id):
		return fmt.Errorf("events.subscribe answered id %q, not %q", ack.ID, id)
	case ack.Result.Type != "subscription_started":
		return fmt.Errorf("events.subscribe answered %q, not subscription_started", ack.Result.Type)
	}
	up()

	for {
		var env herdrEventEnvelope
		if err := dec.Decode(&env); err != nil {
			return fail(err)
		}
		h := env.hint()
		if h.Kind == "" {
			continue
		}
		if want(h) {
			// Capacity-one, never blocking: the reader must keep draining
			// the socket whatever the consumer is doing, or a slow pass
			// would stall herdr's writer (ADR 0016 §1).
			select {
			case out <- h:
			default:
			}
		}
		// A new agent pane cannot be added to this subscription
		// (measurement 3), so the connection is what gets replaced.
		if h.Kind == "pane_agent_detected" || h.Kind == "workspace_created" {
			return errRefreshPanes
		}
	}
}

// herdrSubscriptions is one request's worth of subscriptions: the lifecycle
// events unfiltered, plus the settle event once per pane herdr has an agent
// in. Duplicates are dropped — the same pane twice would double every hint
// off it.
func herdrSubscriptions(panes func() []string) []map[string]string {
	subs := make([]map[string]string, 0, len(HerdrLifecycleSubscriptions)+4)
	for _, t := range HerdrLifecycleSubscriptions {
		subs = append(subs, map[string]string{"type": t})
	}
	if panes == nil {
		return subs
	}
	seen := map[string]bool{}
	for _, p := range panes() {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		subs = append(subs, map[string]string{"type": HerdrPaneSubscription, "pane_id": p})
	}
	return subs
}

// AgentPanes is the pane set the settle subscription is built from: every
// pane this herdr currently reports an agent in. A herdr that cannot be
// asked yields none, which costs the settle events and leaves the lifecycle
// ones — degraded, never fatal.
func (b *HerdrBackend) AgentPanes() []string {
	agents, err := b.H.Agents()
	if err != nil {
		return nil
	}
	panes := make([]string, 0, len(agents))
	for _, a := range agents {
		if a.PaneID != "" {
			panes = append(panes, a.PaneID)
		}
	}
	return panes
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
	// Protocol 19 is not consistent about the spelling it pushes: the
	// lifecycle envelopes come back underscored (`pane_updated`) and the
	// pane-scoped ones dotted (`pane.agent_status_changed`), both measured.
	// Folding to one spelling means neither is load-bearing.
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

// isSettleHint says whether an event is a seat's turn ending. herdr has
// already done the edge detection for the pane events — it pushes
// `agent_status_changed` on the change, not on a poll — so this is a filter
// and holds no state.
func isSettleHint(h HerdrHint) bool {
	switch h.Kind {
	case "workspace_closed":
		// A seat that went away has settled in the only sense that matters
		// here: it is no longer busy.
		return true
	case "pane_agent_status_changed":
		return isSettledStatus(h.AgentStatus)
	}
	return false
}

func isSettledStatus(s string) bool {
	for _, ok := range SettledStatuses {
		if s == ok {
			return true
		}
	}
	return false
}
