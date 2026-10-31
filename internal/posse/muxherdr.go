package posse

// HerdrMux is herdr's occupant of the multiplexer slot (ADR 0076 §1): the
// ten verbs of mux.go over the Herdr client of herdr.go, and nothing of its
// own. Every mapping here is one or two client calls; the client keeps the
// argv, the envelope decoding and the child deadline, which is why this file
// has no herdr knowledge in it beyond which verb answers which question.
//
// IT IS ALSO WHERE THE HERDR-ONLY SURFACE LIVES (ADR 0076 §4). Those verbs
// are reached by a type assertion on the occupant — HerdrBackend.herdrMux —
// and never through Mux, so an occupant that does not have them cannot be
// asked and the caller's "not available" arm is the one that runs. They are
// forwarders with the client's own names, so a reader following a call from
// a §4 site lands on the same method it always did — with ONE exception,
// `Explain`, which forwards `AgentExplain`. ADR 0061 D3's reading is allowed
// two homes, herdr.go and reportedagent.go, and a pin holds it to them
// (reportedreading_qa_test.go): a second function of that name is how a
// second reading starts, and nothing mechanical can tell a one-line
// forwarder from one. The name gives way; the pin does not.

import "encoding/json"

// NewHerdrMux is the production occupant: the ambient herdr binary, or
// RHQ_HERDR_BIN.
func NewHerdrMux() HerdrMux { return HerdrMux{H: NewHerdr()} }

type HerdrMux struct{ H Herdr }

// ─── the ten verbs (ADR 0076 §2) ─────────────────────────────────────────────

// Create is `workspace create`. The seat it hands back carries the workspace
// and its root pane, which is the pane Start types into.
func (m HerdrMux) Create(label, cwd string, env []EnvVar) (Seat, error) {
	wsID, rootPane, err := m.H.CreateWorkspace(label, cwd, env)
	if err != nil {
		return Seat{}, err
	}
	return Seat{Session: wsID, Pane: rootPane}, nil
}

// Start is `pane run` — the second half of §2's Create row, on its own
// because posse types the line after the meta is written and again into
// settled panes (mux.go's Mux doc carries both reasons).
func (m HerdrMux) Start(s Seat, line string) error { return m.H.PaneRun(s.Pane, line) }

func (m HerdrMux) Close(s Seat) error { return m.H.CloseWorkspace(s.Session) }

func (m HerdrMux) Focus(s Seat) error { return m.H.FocusWorkspace(s.Session) }

// List is `workspace list` joined to `agent list`, in that order, and it
// fails on either: a listing missing half of itself would report every
// workspace as holding no runtime, which is the one reading that must never
// be guessed (ADR 0013 §1).
//
// THE JOIN IS WHERE "unknown MEANS NONE" IS APPLIED. herdr answers
// `agent_status: "unknown"` for a workspace holding nothing but a shell, so
// a row's State is its workspace status only when `agent list` put a runtime
// in it, and "" otherwise. That rule used to live in listSessions; it is
// herdr's own quirk, so it belongs to herdr's occupant.
func (m HerdrMux) List() ([]SeatInfo, error) {
	wss, err := m.H.Workspaces()
	if err != nil {
		return nil, err
	}
	agents, err := m.H.Agents()
	if err != nil {
		return nil, err
	}
	panes := map[string][]SeatPane{}
	for _, ag := range agents {
		panes[ag.WorkspaceID] = append(panes[ag.WorkspaceID],
			SeatPane{Pane: ag.PaneID, Kind: ag.Agent, State: ag.AgentStatus})
	}
	out := make([]SeatInfo, 0, len(wss))
	for _, ws := range wss {
		row := SeatInfo{
			Seat:    Seat{Session: ws.WorkspaceID},
			Label:   ws.Label,
			Focused: ws.Focused,
			Panes:   panes[ws.WorkspaceID],
		}
		if len(row.Panes) > 0 {
			row.Pane, row.Kind = row.Panes[0].Pane, row.Panes[0].Kind
			row.State = ws.AgentStatus
		}
		out = append(out, row)
	}
	return out, nil
}

// SendText is `pane send-text`: the keystrokes with no Enter behind them.
//
// Straight onto Run rather than onto a client verb, because the client's one
// send-text caller is the report-agent route, which composes it with a
// send-keys and a wait of its own (reportedagent.go) and cannot be expressed
// as this verb.
func (m HerdrMux) SendText(s Seat, text string) error {
	_, err := m.H.Run("pane", "send-text", s.Pane, text)
	return err
}

func (m HerdrMux) SendKeys(s Seat, keys ...string) error {
	return m.H.AgentSendKeys(s.Pane, keys...)
}

// Read is `pane read --format text`, tailed client-side, or `agent read
// --format ansi` whole.
//
// ansi IGNORES lines, and that is herdr's shape rather than a shortcut: the
// ansi read exists for one corpus measured on one argv (ranger-base-6o7wm,
// AgentReadANSI's doc), and a tail applied to it would be a different
// snapshot than the one the discriminator was measured through.
func (m HerdrMux) Read(s Seat, lines int, ansi bool) (string, error) {
	if ansi {
		return m.H.AgentReadANSI(s.Pane)
	}
	return m.H.PaneRead(s.Pane, lines)
}

// State is `agent explain`, flattened to the reading every occupant can
// answer. The evaluated rules, the composer regions and the fallback's own
// name stay behind AgentExplain below (ADR 0076 §4) — this is the part a
// caller can act on without knowing which multiplexer answered.
func (m HerdrMux) State(s Seat) (MuxReading, error) {
	det, err := m.H.AgentExplain(s.Pane)
	if err != nil {
		return MuxReading{}, err
	}
	return herdrReading(det), nil
}

// herdrReading maps one detection onto the slot's reading. Owner is the
// REPORTED label and empty for a pane herdr detected itself, which is ADR
// 0076 §2's field read literally: the owner is who else's word this reading
// rests on, and herdr's own detection rests on nobody's.
func herdrReading(det AgentDetection) MuxReading {
	return MuxReading{
		State:    det.State,
		Owner:    det.Reported,
		Seen:     det.Seen(),
		Evidence: herdrEvidence(det),
	}
}

// herdrEvidence is the one-line "what said so" a reader wants beside a
// state: the rule that matched, else the name herdr gave its own guess, else
// the chrome it could see. It is free text and nothing branches on it — the
// §4 sites that need herdr's full working call WhatHerdrSaw instead.
func herdrEvidence(det AgentDetection) string {
	switch {
	case det.Rule.ID != "":
		return "matched rule " + det.Rule.ID
	case det.FallbackReason != "":
		return det.FallbackReason
	case det.VisibleIdle:
		return "visible idle chrome"
	}
	return ""
}

// Wait is `agent wait`. Running out of patience is an error carrying herdr's
// `timeout` code and not a reading — callers branch on IsHerdrCode, which
// says nothing about whether anything landed (rangerhq-1z0).
func (m HerdrMux) Wait(s Seat, until []string, timeoutMS int) (MuxReading, error) {
	res, err := m.H.AgentWait(s.Pane, until, timeoutMS)
	if err != nil {
		return MuxReading{}, err
	}
	return waitReading(res), nil
}

// Prompt is `agent prompt`, which stays NATIVE on herdr: herdr owns the
// typed-delivery contract, including the report-agent route AgentPrompt
// chooses for a pane herdr labels but does not detect. An occupant without
// such a verb builds this one out of SendText, SendKeys and Wait (ADR 0076
// §2); herdr does not need to.
func (m HerdrMux) Prompt(s Seat, text string, wait bool, timeoutMS int) (MuxReading, error) {
	res, err := m.H.AgentPrompt(s.Pane, text, wait, timeoutMS)
	if err != nil {
		return MuxReading{}, err
	}
	return waitReading(res), nil
}

// waitReading is what a prompt/wait envelope says. Seen stays false and
// Owner empty on purpose: an envelope reports a STATE and never what
// recognized it, so filling either from one would be inventing evidence.
// Evidence is the envelope verbatim, which is what `posse wait` prints.
func waitReading(res json.RawMessage) MuxReading {
	return MuxReading{State: agentStatusFromResult(res), Evidence: string(res)}
}

// agentStatusFromResult digs the settled state out of an agent.prompt/wait
// result ({"type":"agent_prompted","agent":{"agent_status":...}}). "" is an
// answer with no state in it — see MuxReading's own doc.
func agentStatusFromResult(raw []byte) string {
	var payload struct {
		Status string `json:"status"`
		Agent  struct {
			AgentStatus string `json:"agent_status"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if payload.Agent.AgentStatus != "" {
		return payload.Agent.AgentStatus
	}
	return payload.Status
}

// ─── herdr's own verbs (ADR 0076 §4) ─────────────────────────────────────────
//
// Reached through HerdrBackend.HerdrMux, whose false answer is the "not
// available" every one of these call sites already has an arm for.

// Available is whether this herdr binary is on PATH at all.
func (m HerdrMux) Available() bool { return m.H.Available() }

// Explain is the detection WITH herdr's working: the matched rule, the
// fallback's own name, and every rule it evaluated beside the bytes that
// rule was reading. State above is the part of it any occupant can answer;
// this is the part that is herdr's alone, and it is what the readiness
// gates, the composer readers and the unrecognized-screen report all read.
//
// Not named AgentExplain, which is the client verb it forwards: see the head
// comment, and ADR 0061 D3's one-door pin.
func (m HerdrMux) Explain(s Seat) (AgentDetection, error) {
	return m.H.AgentExplain(s.Pane)
}

// PaneReadDetection is the screen in the shape a detection FIXTURE is —
// `--source detection`, whole, no tail — which is the source `agent explain
// --file` reads back the same way (ADR 0066 D3). Read is a reader's snippet
// and this is a capture; the client's own doc carries both measurements.
func (m HerdrMux) PaneReadDetection(s Seat) (string, error) {
	return m.H.PaneReadDetection(s.Pane)
}

// PaneAgentSession is the RUNTIME's own id for the conversation in the seat
// — claude's session uuid, the join posse has no other way to make.
func (m HerdrMux) PaneAgentSession(s Seat) (string, error) {
	return m.H.PaneAgentSession(s.Pane)
}

// Workspaces is herdr's raw workspace listing, for the three readers whose
// question is herdr's own id arithmetic and not a seat's state: a workspace
// id identifies nothing across a server restart or a handoff, so telling
// "ours" from "a stranger wearing our id" needs the label and the server
// GENERATION beside it (rangerhq-6bg7, notOurWorkspace). Rex has no such
// question — its ids are UUIDs and are never reissued, so a seat is dead
// when its id is missing from the listing (ADR 0076 §5) — and that is why
// this is here rather than on Mux.
//
// It is also what keeps those readers at one herdr call. List joins `agent
// list` onto this, which none of them has any use for.
func (m HerdrMux) Workspaces() ([]HerdrWorkspace, error) { return m.H.Workspaces() }

// Agents is herdr's raw agent listing — the argv `posse probe` measures its
// detection observable through (ADR 0013 §1's `launch` stage, observable 4 of
// the probe's four). It is herdr's because the question is: the probe CREATED
// the workspace and holds its id, so joining a `workspace list` onto this
// would add a herdr call that answers nothing, and the one caller already
// holds the herdr occupant rather than the slot — a probe that could be
// handed another multiplexer would be measuring something it has no rule for.
//
// Everything that asks about a seat it did NOT just create reads List
// instead, where the join is what tells a detected runtime from a plain
// shell.
func (m HerdrMux) Agents() ([]HerdrAgent, error) { return m.H.Agents() }

// WorkspaceGet asks this server about ONE workspace id, now. It is the
// evidence a prune needs and a listing cannot give — a listing is a snapshot
// taken at an instant, and absence from one is never death (ADR 0011 §2,
// rangerhq-9nso). Three answers, kept apart by the caller: the workspace,
// herdr's own workspace_not_found, or no answer at all.
func (m HerdrMux) WorkspaceGet(id string) (HerdrWorkspace, bool, error) {
	return m.H.WorkspaceGet(id)
}

// KnownAgentKinds, AgentManifest and HasReportAgent are the detection
// preflight — the `launch` stage's observables (ADR 0013 §1), each of which
// degrades to UNKNOWN and never to a wrong "no". The client's docs carry why
// there are three of them and why none is derived from a version string.
func (m HerdrMux) KnownAgentKinds() []string { return m.H.KnownAgentKinds() }

func (m HerdrMux) AgentManifest(label string) (version string, known, ok bool) {
	return m.H.AgentManifest(label)
}

func (m HerdrMux) HasReportAgent() (present, ok bool) { return m.H.HasReportAgent() }

// ManifestReader is the detection preflight's three questions, named so the
// readers that ask them (detection.go, and `posse runtimes`' stage rows) can
// be handed herdr's occupant without naming its type — and so a test can
// hand them the bare client.
type ManifestReader interface {
	AgentManifest(label string) (version string, known, ok bool)
	KnownAgentKinds() []string
	HasReportAgent() (present, ok bool)
}

// muxNotHerdr is the sentence a §4 verb answers with when the slot holds
// something else. "not available" and never "no" (ADR 0076 §4): a reading
// nobody could take says nothing about the pane.
func muxNotHerdr(verb string) error {
	return Die("%s is herdr's own verb and this instance's multiplexer is not herdr — the reading is not available (ADR 0076 §4)", verb)
}

var _ Mux = HerdrMux{}
var _ ManifestReader = HerdrMux{}
var _ ManifestReader = Herdr{}

// muxNoManifest is the detection preflight (ADR 0076 §4) asked of a slot
// that is not herdr's: every answer is UNKNOWN — ok false, no kind list —
// and never a "no". That is the same direction the herdr answers already
// degrade in when herdr cannot be asked at all, and for the same reason: a
// launch refused because a diagnostic verb went missing is the wall those
// three verbs exist not to build (ADR 0013 §1).
type muxNoManifest struct{}

func (muxNoManifest) AgentManifest(string) (version string, known, ok bool) { return "", false, false }

func (muxNoManifest) KnownAgentKinds() []string { return nil }

func (muxNoManifest) HasReportAgent() (present, ok bool) { return false, false }

var _ ManifestReader = muxNoManifest{}
