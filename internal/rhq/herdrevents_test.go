package rhq

// The hint adapter, against a hermetic Unix-socket server (ADR 0016 §3).
// Nothing here reaches a real herdr: the fake CLI stays the truth for
// Sessions() and Run, and this listener is the only thing the subscription
// ever dials.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// hintServer is the smallest thing that behaves like herdr's events socket:
// accept, read one subscribe request, answer it, then push whatever the test
// pushes. It can also refuse, answer wrongly, hang up, and talk nonsense —
// the failure shapes the adapter must survive. Like the real server it takes
// exactly one subscribe request per connection.
type hintServer struct {
	t    *testing.T
	path string
	ln   net.Listener

	subs  chan string   // the raw subscribe request line, one per connection
	ready chan net.Conn // connections that got their acknowledgement

	mu       sync.Mutex
	ack      func(id string) string // "" hangs up without answering
	ackDelay time.Duration          // held before answering, to open the race a poke lands in
}

func newHintServer(t *testing.T) *hintServer {
	t.Helper()
	// Short, because a Unix socket path is capped near 104 bytes and
	// t.TempDir() under a long test name blows straight through it.
	path := filepath.Join(shortTempDir(t), "herdr.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen %s: %v", path, err)
	}
	s := &hintServer{
		t: t, path: path, ln: ln,
		subs:  make(chan string, 16),
		ready: make(chan net.Conn, 16),
		ack: func(id string) string {
			return fmt.Sprintf(`{"id":%q,"result":{"type":"subscription_started"}}`, id)
		},
	}
	t.Cleanup(func() { ln.Close() })
	go s.accept()
	return s
}

func (s *hintServer) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.serve(c)
	}
}

func (s *hintServer) serve(c net.Conn) {
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		c.Close()
		return
	}
	s.subs <- line
	var req struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(line), &req)
	s.mu.Lock()
	ack, delay := s.ack, s.ackDelay
	s.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	answer := ack(req.ID)
	if answer == "" {
		c.Close()
		return
	}
	if _, err := c.Write([]byte(answer + "\n")); err != nil {
		c.Close()
		return
	}
	s.ready <- c
}

// setAck changes what the next connection is answered with.
func (s *hintServer) setAck(f func(id string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ack = f
}

// conn waits for the next acknowledged connection.
func (s *hintServer) conn() net.Conn {
	s.t.Helper()
	select {
	case c := <-s.ready:
		return c
	case <-time.After(5 * time.Second):
		s.t.Fatal("the adapter never subscribed")
		return nil
	}
}

// subscribed waits for the next subscribe request line.
func (s *hintServer) subscribed() string {
	s.t.Helper()
	select {
	case l := <-s.subs:
		return l
	case <-time.After(5 * time.Second):
		s.t.Fatal("no events.subscribe request arrived")
		return ""
	}
}

func (s *hintServer) push(c net.Conn, envelope string) {
	s.t.Helper()
	c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write([]byte(envelope + "\n")); err != nil {
		s.t.Fatalf("push %s: %v", envelope, err)
	}
}

// settled is herdr's real pushed shape for a pane-scoped status change:
// DOTTED event name (the lifecycle envelopes are underscored, these are
// not), status and ids at the top of data.
func settled(pane, status string) string {
	ws := pane
	if i := strings.Index(pane, ":"); i > 0 {
		ws = pane[:i]
	}
	return fmt.Sprintf(`{"data":{"agent":"claude","agent_status":%q,"pane_id":%q,"workspace_id":%q,`+
		`"type":"pane_agent_status_changed"},"event":"pane.agent_status_changed"}`, status, pane, ws)
}

func recvHint(t *testing.T, ch <-chan HerdrHint, within time.Duration) HerdrHint {
	t.Helper()
	select {
	case h, ok := <-ch:
		if !ok {
			t.Fatal("the hint channel closed while a hint was expected")
		}
		return h
	case <-time.After(within):
		t.Fatalf("no hint within %s", within)
		return HerdrHint{}
	}
}

func collect(mu *sync.Mutex, lines *[]string) func(string) {
	return func(l string) {
		mu.Lock()
		*lines = append(*lines, l)
		mu.Unlock()
	}
}

func panesAre(ids ...string) func() []string {
	return func() []string { return ids }
}

// The subscribe request is the whole contract with herdr, and it is one
// request per connection: the lifecycle events unfiltered, plus the settle
// event once per pane. Unscoped pane.agent_status_changed is refused by the
// real server and takes the connection with it, so a request that grew one
// would deliver nothing at all.
func TestHerdrHintsSubscribeRequest(t *testing.T) {
	s := newHintServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	herdrHints(ctx, s.path, 20*time.Millisecond, panesAre("w1:p1", "w2:p1", "w1:p1", ""), nil, isSettleHint, collect(&mu, &lines))

	var req struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			Subscriptions []map[string]string `json:"subscriptions"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(s.subscribed()), &req); err != nil {
		t.Fatalf("the request must be one line of JSON: %v", err)
	}
	if req.Method != "events.subscribe" {
		t.Errorf("method = %q, want events.subscribe", req.Method)
	}
	if req.ID == "" {
		t.Error("the request needs an id — the acknowledgement is verified against it")
	}
	var lifecycle, scoped []string
	for _, sub := range req.Params.Subscriptions {
		switch sub["type"] {
		case HerdrPaneSubscription:
			if sub["pane_id"] == "" {
				t.Error("herdr refuses pane.agent_status_changed without a pane_id and closes the connection over it")
			}
			scoped = append(scoped, sub["pane_id"])
		default:
			if len(sub) != 1 {
				t.Errorf("subscription %v is scoped; the lifecycle events are taken unfiltered (ADR 0016 §1)", sub)
			}
			lifecycle = append(lifecycle, sub["type"])
		}
	}
	if strings.Join(lifecycle, ",") != strings.Join(HerdrLifecycleSubscriptions, ",") {
		t.Errorf("lifecycle subscriptions = %v, want %v", lifecycle, HerdrLifecycleSubscriptions)
	}
	if strings.Join(scoped, ",") != "w1:p1,w2:p1" {
		t.Errorf("scoped subscriptions = %v; want one per pane, deduplicated, no empties", scoped)
	}
}

// What counts as a settle, and what does not. herdr has already done the
// edge detection for the pane events, so this is a filter and not a state
// machine — but it is a filter with opinions: `blocked` still holds a bead.
func TestHerdrSettleHintsFilter(t *testing.T) {
	s := newHintServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	hints := herdrHints(ctx, s.path, 20*time.Millisecond, panesAre("w1:p1", "w2:p1"), nil, isSettleHint, collect(&mu, &lines))
	c := s.conn()

	// Not settles: still working, blocked on the operator, an event kind
	// this consumer does not care about, an envelope it cannot read.
	s.push(c, settled("w1:p1", "working"))
	s.push(c, settled("w1:p1", "blocked"))
	s.push(c, `{"data":{"pane_id":"w1:p1","type":"pane_scroll_changed"},"event":"pane_scroll_changed"}`)
	s.push(c, `{"event":"","data":{}}`)
	// A settle.
	s.push(c, settled("w2:p1", "idle"))

	h := recvHint(t, hints, 5*time.Second)
	if h.PaneID != "w2:p1" || h.AgentStatus != "idle" || h.WorkspaceID != "w2" {
		t.Fatalf("first hint = %+v; working, blocked, an unknown kind and an unreadable envelope are all not settles", h)
	}
	if h.Kind != "pane_agent_status_changed" {
		t.Errorf("kind = %q — the dotted spelling herdr pushes must fold to the underscored one", h.Kind)
	}
	// done is a settle too, and so is a workspace that went away.
	s.push(c, settled("w1:p1", "done"))
	if h := recvHint(t, hints, 5*time.Second); h.AgentStatus != "done" || h.PaneID != "w1:p1" {
		t.Errorf("done hint = %+v", h)
	}
	s.push(c, `{"data":{"type":"workspace_closed","workspace":{"workspace_id":"w9"}},"event":"workspace_closed"}`)
	if h := recvHint(t, hints, 5*time.Second); h.Kind != "workspace_closed" || h.WorkspaceID != "w9" {
		t.Errorf("workspace_closed hint = %+v", h)
	}
}

// A pane cannot be added to a live subscription — the server takes one
// subscribe per connection — so a newly detected agent pane is answered by
// redialling with the pane set as it now stands. Silently: nothing failed.
func TestHerdrHintsResubscribesWhenThePaneSetMoves(t *testing.T) {
	s := newHintServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	var panes struct {
		sync.Mutex
		ids []string
	}
	panes.ids = []string{"w1:p1"}
	hints := herdrHints(ctx, s.path, 20*time.Millisecond, func() []string {
		panes.Lock()
		defer panes.Unlock()
		return append([]string(nil), panes.ids...)
	}, nil, isSettleHint, collect(&mu, &lines))

	s.subscribed()
	c := s.conn()
	// A seat comes up: herdr detects its agent, and the pane set moves.
	panes.Lock()
	panes.ids = []string{"w1:p1", "w2:p1"}
	panes.Unlock()
	s.push(c, `{"data":{"agent":"claude","pane_id":"w2:p1","workspace_id":"w2","type":"pane_agent_detected"},"event":"pane_agent_detected"}`)

	second := s.subscribed()
	if !strings.Contains(second, `"w2:p1"`) {
		t.Fatalf("the new pane must be in the second subscription: %s", second)
	}
	c2 := s.conn()
	s.push(c2, settled("w2:p1", "idle"))
	if h := recvHint(t, hints, 5*time.Second); h.PaneID != "w2:p1" {
		t.Fatalf("the settle of the pane that appeared = %+v", h)
	}
	mu.Lock()
	got := append([]string(nil), lines...)
	mu.Unlock()
	if len(got) != 0 {
		t.Errorf("re-subscribing is the adapter keeping up, not an outage; it said: %v", got)
	}
}

// The pane set's level-triggered belt: the consumer pokes after every pass,
// and the subscription redials with the list as it now stands. MEASURED
// live — a seat that appeared after the subscription was dialled had its
// settle missed, and no lifecycle event arrived to say the set had moved, so
// the poke is what makes the set eventually right.
func TestHerdrHintsRefreshPokeRedials(t *testing.T) {
	s := newHintServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	refresh := make(chan struct{}, 1)
	var panes struct {
		sync.Mutex
		ids []string
	}
	panes.ids = []string{"w1:p1"}
	hints := herdrHints(ctx, s.path, 20*time.Millisecond, func() []string {
		panes.Lock()
		defer panes.Unlock()
		return append([]string(nil), panes.ids...)
	}, refresh, isSettleHint, collect(&mu, &lines))

	s.subscribed()
	s.conn()
	panes.Lock()
	panes.ids = []string{"w1:p1", "w7:p1"}
	panes.Unlock()
	refresh <- struct{}{}

	if req := s.subscribed(); !strings.Contains(req, `"w7:p1"`) {
		t.Fatalf("a poke must redial with the pane set as it now stands: %s", req)
	}
	c := s.conn()
	s.push(c, settled("w7:p1", "done"))
	if h := recvHint(t, hints, 5*time.Second); h.PaneID != "w7:p1" {
		t.Fatalf("the settle of the seat the poke picked up = %+v", h)
	}
	mu.Lock()
	got := append([]string(nil), lines...)
	mu.Unlock()
	if len(got) != 0 {
		t.Errorf("a poked redial is not an outage; it said: %v", got)
	}
}

// A poke that lands in the window between writing the subscribe and reading
// its acknowledgement is still a poke. Seen live: the pass-end poke arrives
// there routinely, and reporting it as an outage would put a false
// "events unavailable" line in the loop's output on every pass.
func TestHerdrHintsPokeDuringHandshakeIsNotAnOutage(t *testing.T) {
	s := newHintServer(t)
	s.mu.Lock()
	s.ackDelay = 300 * time.Millisecond
	s.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	refresh := make(chan struct{}, 1)
	herdrHints(ctx, s.path, 20*time.Millisecond, panesAre("w1:p1"), refresh, isSettleHint, collect(&mu, &lines))

	s.subscribed() // the request is out; the acknowledgement is 300ms away
	refresh <- struct{}{}
	s.subscribed() // it redialled

	s.mu.Lock()
	s.ackDelay = 0
	s.mu.Unlock()
	time.Sleep(400 * time.Millisecond)
	mu.Lock()
	got := append([]string(nil), lines...)
	mu.Unlock()
	for _, l := range got {
		if strings.Contains(l, "events unavailable") {
			t.Fatalf("a poke mid-handshake is not an outage: %v", got)
		}
	}
}

// A burst must never block herdr's writer. The channel holds one hint —
// "look again", not "look again N times" — and the socket reader keeps
// draining while nobody is receiving.
func TestHerdrHintsBurstNeverBlocksTheReader(t *testing.T) {
	s := newHintServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	hints := herdrHints(ctx, s.path, 20*time.Millisecond, panesAre("w1:p1"), nil, isSettleHint, collect(&mu, &lines))
	c := s.conn()

	// 400 settles with nothing receiving. Each push carries a write
	// deadline, so a reader that stopped draining fails this as a timeout.
	for i := 0; i < 200; i++ {
		s.push(c, settled("w1:p1", "working"))
		s.push(c, settled("w1:p1", "idle"))
	}
	// Coalesced, not queued: once the reader has drained the socket the
	// whole burst is one waiting hint.
	time.Sleep(300 * time.Millisecond)
	drained := 0
	for draining := true; draining; {
		select {
		case h := <-hints:
			drained++
			if h.PaneID != "w1:p1" {
				t.Fatalf("hint after the burst = %+v", h)
			}
		default:
			draining = false
		}
	}
	if drained != 1 {
		t.Fatalf("the burst left %d hints waiting; the channel holds one", drained)
	}
	// Still live afterwards.
	s.push(c, settled("w1:p1", "done"))
	if h := recvHint(t, hints, 5*time.Second); h.AgentStatus != "done" {
		t.Errorf("the subscription must survive the burst: %+v", h)
	}
}

// Every way the stream can die is one outage line and a redial, and the
// recovery is one line. Nothing here is fatal to the caller.
func TestHerdrHintsRetriesAndReportsOnce(t *testing.T) {
	cases := []struct {
		name string
		kill func(s *hintServer, c net.Conn)
	}{
		{"eof", func(s *hintServer, c net.Conn) { c.Close() }},
		{"malformed", func(s *hintServer, c net.Conn) {
			c.Write([]byte("{this is not json\n"))
			c.Close()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newHintServer(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var mu sync.Mutex
			var lines []string
			hints := herdrHints(ctx, s.path, 20*time.Millisecond, panesAre("w1:p1"), nil, isSettleHint, collect(&mu, &lines))

			s.subscribed()
			tc.kill(s, s.conn())

			// It redials, and the second subscription delivers.
			s.subscribed()
			c2 := s.conn()
			s.push(c2, settled("w1:p1", "idle"))
			if h := recvHint(t, hints, 5*time.Second); h.PaneID != "w1:p1" {
				t.Fatalf("hint after the redial = %+v", h)
			}
			mu.Lock()
			got := append([]string(nil), lines...)
			mu.Unlock()
			var outages, recoveries int
			for _, l := range got {
				switch {
				case strings.Contains(l, "events unavailable — polling"):
					outages++
				case strings.Contains(l, "restored"):
					recoveries++
				}
			}
			if outages != 1 || recoveries != 1 {
				t.Errorf("one outage line and one recovery line per episode; got %d/%d: %v", outages, recoveries, got)
			}
		})
	}
}

// A refusal is an answer, and it is not an acknowledgement. Same for an
// acknowledgement that answers a request this did not send. The one refusal
// that is NOT an outage is a pane that went away between the listing and the
// subscribe — the next dial re-reads the list, and that is the whole fix.
func TestHerdrHintsRejectsBadAcknowledgements(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ack   func(id string) string
		want  string
		quiet bool
	}{
		{name: "error", ack: func(string) string {
			return `{"id":"","error":{"code":"invalid_request","message":"missing field pane_id"}}`
		}, want: "refused"},
		{name: "wrong id", ack: func(string) string {
			return `{"id":"somebody-else","result":{"type":"subscription_started"}}`
		}, want: "not \"posse-hints"},
		{name: "wrong type", ack: func(id string) string {
			return fmt.Sprintf(`{"id":%q,"result":{"type":"pane_read"}}`, id)
		}, want: "not subscription_started"},
		{name: "silence", ack: func(string) string { return "" }, want: "no answer"},
		{name: "stale pane", ack: func(id string) string {
			return fmt.Sprintf(`{"id":"%s:sub:0","error":{"code":"pane_not_found","message":"pane w9:p9 not found"}}`, id)
		}, quiet: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newHintServer(t)
			s.setAck(tc.ack)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var mu sync.Mutex
			var lines []string
			herdrHints(ctx, s.path, 20*time.Millisecond, panesAre("w9:p9"), nil, isSettleHint, collect(&mu, &lines))
			s.subscribed()

			if tc.quiet {
				// It keeps redialling — with a fresh pane list each time —
				// and says nothing, because nothing is wrong with herdr.
				s.subscribed()
				mu.Lock()
				got := append([]string(nil), lines...)
				mu.Unlock()
				if len(got) != 0 {
					t.Errorf("a pane that went away is not an outage: %v", got)
				}
				return
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				mu.Lock()
				got := append([]string(nil), lines...)
				mu.Unlock()
				if len(got) > 0 {
					if !strings.Contains(got[0], "events unavailable — polling") || !strings.Contains(got[0], tc.want) {
						t.Fatalf("outage line must name what went wrong (%q): %q", tc.want, got[0])
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("a subscription that was never acknowledged must be reported as an outage")
		})
	}
}

// Cancellation closes the socket under a blocking read and closes the
// channel; it does not wait out the retry delay.
func TestHerdrHintsCancelClosesPromptly(t *testing.T) {
	s := newHintServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var lines []string
	hints := herdrHints(ctx, s.path, time.Hour, panesAre("w1:p1"), nil, isSettleHint, collect(&mu, &lines))
	s.conn()

	start := time.Now()
	cancel()
	select {
	case _, ok := <-hints:
		if ok {
			t.Fatal("a cancelled subscription must close its channel, not deliver")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the hint channel stayed open after cancel")
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("cancel took %s — it must not wait out the retry delay", d)
	}
}

// No socket at all is the degraded arm, and it is not an error anyone sees
// twice: one line, then quiet retrying.
func TestHerdrHintsWithNoSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	dead := filepath.Join(shortTempDir(t), "nothing-here.sock")
	herdrHints(ctx, dead, 50*time.Millisecond, panesAre("w1:p1"), nil, isSettleHint, collect(&mu, &lines))
	time.Sleep(250 * time.Millisecond)
	mu.Lock()
	got := append([]string(nil), lines...)
	mu.Unlock()
	if len(got) != 1 || !strings.Contains(got[0], "events unavailable — polling") {
		t.Fatalf("a missing socket is one outage line however many redials it takes; got %v", got)
	}
}

// A herdr that cannot be asked for its panes costs the settle events and
// leaves the lifecycle ones: degraded, never fatal, and never a subscription
// this server would refuse.
func TestAgentPanesDegradesToNone(t *testing.T) {
	b, fake := newTestBackend(t)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"working","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)
	if got := b.AgentPanes(); strings.Join(got, ",") != "w1:p1,w2:p1" {
		t.Errorf("AgentPanes = %v, want both panes herdr reports an agent in", got)
	}
	subs := herdrSubscriptions(nil)
	if len(subs) != len(HerdrLifecycleSubscriptions) {
		t.Errorf("with no pane source the request is the lifecycle events alone; got %v", subs)
	}
}

// ─── the watch loop (the bead's DONE WHEN) ───────────────────────────────────

// A settling session produces a logged hint in the watch process within ~1s,
// and it changes nothing else: this slice logs, and the tick still decides
// when the next pass runs.
func TestWatchLogsSettleHintAndLeavesTheTickAlone(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	// The seat that is about to settle: the subscription is built per pane,
	// so a pane herdr does not report an agent in is a pane no hint arrives
	// for.
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"working","pane_id":"w4:p1","workspace_id":"w4"}]`), 0o644)

	s := newHintServer(t)
	// The production path, end to end: clear the hermetic stub and point the
	// socket resolver at this listener.
	d.Hints = nil
	t.Setenv("HERDR_SOCKET_PATH", s.path)

	tap := newPassTap(1)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	// A pass interval long enough that nothing but a bug starts a second
	// pass during this test.
	go func() { p, _ := d.Watch(ctx, "", "", 0, time.Hour, time.Hour); done <- p }()

	if req := s.subscribed(); !strings.Contains(req, `"w4:p1"`) {
		t.Fatalf("the loop must subscribe to the panes herdr reports agents in: %s", req)
	}
	s.conn()
	select {
	case <-tap.reached:
	case <-time.After(30 * time.Second):
		t.Fatalf("the loop never announced a pass:\n%s", tap.String())
	}
	// The pass pokes the subscription so a seat it found gets subscribed;
	// that redial is the connection the settle arrives on.
	if req := s.subscribed(); !strings.Contains(req, `"w4:p1"`) {
		t.Fatalf("the post-pass re-subscription must still carry the pane set: %s", req)
	}
	c := s.conn()
	settledAt := time.Now()
	s.push(c, settled("w4:p1", "idle"))

	var took time.Duration
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if strings.Contains(tap.String(), "settle hint") {
			took = time.Since(settledAt)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	out := tap.String()
	if took == 0 {
		t.Fatalf("a settling session must log a hint in the watch process:\n%s", out)
	}
	if took > time.Second {
		t.Errorf("the hint took %s to reach the log; the ADR's claim is ~1s", took)
	}
	if !strings.Contains(out, "w4:p1") || !strings.Contains(out, "idle") {
		t.Errorf("the hint line must say which seat settled and how:\n%s", out)
	}
	// Hint-only: the tick still governs. A hint that started a pass would
	// show a second header here, an hour early.
	if n := strings.Count(out, passHeader); n != 1 {
		t.Errorf("this slice logs the hint and nothing else — %d passes, want 1:\n%s", n, out)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("watch never returned after cancel")
	}
}

// Herdr dead or absent: the loop says so once and keeps ticking. The events
// are a latency path, never a dependency (ADR 0016 §2).
func TestWatchDegradesToTheTickWithNoHerdr(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	d.Hints = nil
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(shortTempDir(t), "no-herdr.sock"))

	const wantPasses = 3
	tap := newPassTap(wantPasses)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	var passes int
	select {
	case passes = <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}
	out := tap.String()
	if passes < wantPasses {
		t.Errorf("the tick must keep the loop running with no herdr; got %d passes:\n%s", passes, out)
	}
	if !strings.Contains(out, "events unavailable — polling") {
		t.Errorf("a dead events socket is reported, once, in the loop's own output:\n%s", out)
	}
	if strings.Contains(out, "settle hint") {
		t.Errorf("no socket, no hints:\n%s", out)
	}
}
