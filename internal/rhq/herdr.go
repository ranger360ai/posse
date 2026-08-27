package rhq

// Every herdr invocation goes through Herdr.Run — the herdr twin of the Tmux
// runner, and the second backend behind the seam DIRECTION.md describes.
// herdr CLI control commands print a JSON envelope ({"id":...,"result":{...}}
// or {"error":{"code","message"}}); Run decodes it and hands back the result.
//
// posse state (emoji, env-set names, persona) deliberately never lives in
// herdr — see herdrback.go for the meta files.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Herdr struct {
	Bin string // herdr binary; RHQ_HERDR_BIN overrides (testing)
}

func NewHerdr() Herdr {
	bin := os.Getenv("RHQ_HERDR_BIN")
	if bin == "" {
		bin = "herdr"
	}
	return Herdr{Bin: bin}
}

func (h Herdr) Available() bool {
	_, err := exec.LookPath(h.Bin)
	return err == nil
}

// KnownAgentKinds is the set of agent kinds this herdr recognizes — the
// `launch` stage's first observable (ADR 0013 §1): a runtime whose argv0
// herdr cannot name has no detection, so `working` and every settled state
// are guesses and dispatch is blind on it.
//
// Read from `herdr agent start --help`, which is the only place herdr
// enumerates them; it is a clap `[possible values: …]` line, not the JSON
// envelope Run decodes. Parsing --help is a soft dependency by
// construction, so a shape change here degrades to "unknown" (nil) and
// never to a wrong "no".
func (h Herdr) KnownAgentKinds() []string {
	if !h.Available() {
		return nil
	}
	out, _, _ := h.capture([]string{"agent", "start", "--help"})
	for _, ln := range strings.Split(string(out), "\n") {
		i := strings.Index(ln, "[possible values:")
		if i < 0 {
			continue
		}
		v := ln[i+len("[possible values:"):]
		if j := strings.Index(v, "]"); j >= 0 {
			v = v[:j]
		}
		var kinds []string
		for _, k := range strings.Split(v, ",") {
			if k = strings.TrimSpace(k); k != "" {
				kinds = append(kinds, k)
			}
		}
		return kinds
	}
	return nil
}

// AgentManifest asks herdr whether it has a detection manifest for one agent
// LABEL, and which version answered. This is the `launch` stage's first
// observable, asked the way herdr itself resolves it rather than by pattern
// matching a --help line.
//
// It exists beside KnownAgentKinds because the two answer different
// questions, and the difference is load-bearing for a third party. The kind
// list is clap's compiled `[possible values:]`; a manifest can also be
// reached through an `aliases = [...]` entry on another agent's manifest,
// which is the ONLY route a CLI herdr was not built with has to detection at
// all (MEASURED on herdr 0.8.0, 2026-08-27: a standalone
// ~/.config/herdr/agent-detection/<newname>.toml is ignored outright —
// `agent explain --agent <newname>` answers unknown_agent with a null
// manifest, and the file never appears in `server agent-manifests` — while
// `--agent grok-build`, an alias in our own grok.toml, resolves to grok's
// manifest and matches its rules). A check that only read the kind list
// would tell an operator who aliased their CLI correctly that it is
// undetectable.
//
// ok is false when herdr could not be asked at all: absent, or an envelope
// this cannot read. That is UNKNOWN, never a "no" — the same rule
// KnownAgentKinds applies to parsing --help.
func (h Herdr) AgentManifest(label string) (version string, known, ok bool) {
	if !h.Available() || label == "" {
		return "", false, false
	}
	// `agent explain` needs a screen to explain. An empty one is the point:
	// no rule can match it, so what comes back is purely "is there a
	// manifest for this label", with fallback_reason distinguishing a known
	// agent whose screen said nothing (default_known_agent_idle_fallback)
	// from a label herdr has never heard of (unknown_agent).
	f, err := os.CreateTemp("", "posse-detect-*.txt")
	if err != nil {
		return "", false, false
	}
	f.Close()
	defer os.Remove(f.Name())
	out, _, _ := h.capture([]string{"agent", "explain", "--file", f.Name(), "--agent", label, "--json"})
	var v struct {
		Fallback string  `json:"fallback_reason"`
		Manifest *string `json:"manifest_version"`
	}
	if json.Unmarshal(bytes.TrimSpace(out), &v) != nil {
		return "", false, false
	}
	if v.Manifest != nil {
		return *v.Manifest, true, true
	}
	if v.Fallback == "unknown_agent" {
		return "", false, true
	}
	return "", false, false
}

type herdrError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HerdrAPIError is a herdr API envelope error. The code is the part callers
// can branch on — a --wait that ran out of patience ("timeout") says nothing
// about whether the prompt landed, while agent_prompt_stalled /
// agent_not_ready say it did not (rangerhq-1z0). Matching on the message
// text would be a guess; the code is herdr's contract.
type HerdrAPIError struct {
	Code    string
	Message string
}

func (e HerdrAPIError) Error() string { return fmt.Sprintf("herdr: %s (%s)", e.Message, e.Code) }

// IsHerdrCode reports whether err is a herdr API error with this code.
func IsHerdrCode(err error, code string) bool {
	var he HerdrAPIError
	return errors.As(err, &he) && he.Code == code
}

type herdrEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *herdrError     `json:"error"`
}

func (h Herdr) capture(args []string) (stdout []byte, stderr string, runErr error) {
	cmd := exec.Command(h.Bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr = cmd.Run()
	return out.Bytes(), strings.TrimSpace(errb.String()), runErr
}

// RunText executes herdr and returns stdout verbatim — for commands like
// `pane read` whose output is plain text, never a JSON envelope (terminal
// text that happens to look like JSON must not be interpreted).
func (h Herdr) RunText(args ...string) (string, error) {
	out, errb, runErr := h.capture(args)
	if runErr != nil {
		if errb == "" {
			errb = runErr.Error()
		}
		return "", Die("herdr %s: %s", strings.Join(args, " "), errb)
	}
	return string(out), nil
}

// decodeEnvelope parses one herdr JSON envelope, returning nil when b is not
// one.
func decodeEnvelope(b []byte) *herdrEnvelope {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return nil
	}
	var env herdrEnvelope
	if json.Unmarshal(b, &env) != nil {
		return nil
	}
	return &env
}

// errEnvelope finds the error envelope herdr printed on stderr. herdr 0.8.0
// prints CLI server errors as JSON on stderr with stdout empty and exit 1
// (rangerhq-gnd) — reading only stdout turned a typed timeout into an
// untyped Die, and gather then unclaimed live work. Any leading log noise is
// skipped: the envelope is whichever line parses as one.
func errEnvelope(stderr string) *herdrEnvelope {
	if env := decodeEnvelope([]byte(stderr)); env != nil && env.Error != nil {
		return env
	}
	lines := strings.Split(stderr, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if env := decodeEnvelope([]byte(lines[i])); env != nil && env.Error != nil {
			return env
		}
	}
	return nil
}

// Run executes herdr and returns the envelope's result (raw stdout when the
// output isn't an envelope). API errors surface as errors, not JSON —
// whichever stream herdr printed them on.
func (h Herdr) Run(args ...string) (json.RawMessage, error) {
	out, errb, runErr := h.capture(args)

	trimmed := bytes.TrimSpace(out)
	if env := decodeEnvelope(trimmed); env != nil {
		if env.Error != nil {
			return nil, HerdrAPIError{Code: env.Error.Code, Message: env.Error.Message}
		}
		if env.Result != nil {
			return env.Result, nil
		}
	}
	if runErr != nil {
		if env := errEnvelope(errb); env != nil {
			return nil, HerdrAPIError{Code: env.Error.Code, Message: env.Error.Message}
		}
		if errb == "" {
			errb = runErr.Error()
		}
		return nil, Die("herdr %s: %s", strings.Join(args, " "), errb)
	}
	return trimmed, nil
}

// ─── typed views of the API responses ────────────────────────────────────────

type HerdrWorkspace struct {
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
	AgentStatus string `json:"agent_status"` // working|blocked|idle|... ("" = none)
	ActiveTabID string `json:"active_tab_id"`
	TabCount    int    `json:"tab_count"`
	PaneCount   int    `json:"pane_count"`
}

type HerdrAgent struct {
	Agent         string `json:"agent"` // kind: claude, codex, ...
	AgentStatus   string `json:"agent_status"`
	PaneID        string `json:"pane_id"`
	TabID         string `json:"tab_id"`
	WorkspaceID   string `json:"workspace_id"`
	Cwd           string `json:"cwd"`
	Focused       bool   `json:"focused"`
	TerminalTitle string `json:"terminal_title_stripped"`
}

func (h Herdr) Workspaces() ([]HerdrWorkspace, error) {
	res, err := h.Run("workspace", "list")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Workspaces []HerdrWorkspace `json:"workspaces"`
	}
	if err := json.Unmarshal(res, &payload); err != nil {
		return nil, Die("herdr workspace list: bad response: %v", err)
	}
	return payload.Workspaces, nil
}

// WorkspaceGet asks this herdr server about one workspace by id. It is
// the evidence a prune needs and a listing cannot give: `workspace list` is
// a snapshot taken at one instant, and a workspace created by another
// process after that instant is missing from it while being perfectly alive
// (rangerhq-9nso). `workspace get` is answered now, for this id.
//
// Three answers, and the caller must keep them apart: the workspace
// (found); provably gone (herdr's own workspace_not_found); or err — the
// server did not answer, or answered something we cannot read, which is not
// evidence of death.
//
// It hands back the workspace itself, not just "yes", because the id it was
// asked about does not identify a session: herdr recomputes its allocator
// from the live set at every server process start, so ids recycle across a
// restart and across a handoff (rangerhq-6bg7). What the answer proves is
// decided by the caller, against the workspace's label — see
// notOurWorkspace in herdrback.go.
func (h Herdr) WorkspaceGet(id string) (HerdrWorkspace, bool, error) {
	var ws HerdrWorkspace
	if id == "" {
		return ws, false, Die("herdr workspace get: no workspace id")
	}
	res, err := h.Run("workspace", "get", id)
	if err != nil {
		if IsHerdrCode(err, "workspace_not_found") {
			return ws, false, nil
		}
		return ws, false, err
	}
	var payload struct {
		Workspace HerdrWorkspace `json:"workspace"`
	}
	if err := json.Unmarshal(res, &payload); err != nil || payload.Workspace.WorkspaceID == "" {
		return ws, false, Die("herdr workspace get %s: bad response", id)
	}
	return payload.Workspace, true, nil
}

// WorkspaceAlive answers only the liveness half of the two questions a
// per-id query conflates: SOME workspace holds this id. It never says the
// one the meta recorded does — that is identity, and it needs the workspace
// itself (WorkspaceGet). Kept for callers that genuinely only ask whether an
// id is held.
func (h Herdr) WorkspaceAlive(id string) (bool, error) {
	_, found, err := h.WorkspaceGet(id)
	return found, err
}

func (h Herdr) Agents() ([]HerdrAgent, error) {
	res, err := h.Run("agent", "list")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Agents []HerdrAgent `json:"agents"`
	}
	if err := json.Unmarshal(res, &payload); err != nil {
		return nil, Die("herdr agent list: bad response: %v", err)
	}
	return payload.Agents, nil
}

// CreateWorkspace makes an unfocused workspace and returns its id and root
// pane id. Env values ride the same known-exposure path as tmux -e (argv).
func (h Herdr) CreateWorkspace(label, cwd string, env []EnvVar) (wsID, rootPane string, err error) {
	args := []string{"workspace", "create", "--label", label, "--no-focus"}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	for _, v := range env {
		args = append(args, "--env", v.Key+"="+v.Value)
	}
	res, err := h.Run(args...)
	if err != nil {
		return "", "", err
	}
	var payload struct {
		Workspace HerdrWorkspace `json:"workspace"`
		RootPane  struct {
			PaneID string `json:"pane_id"`
		} `json:"root_pane"`
	}
	if err := json.Unmarshal(res, &payload); err != nil || payload.Workspace.WorkspaceID == "" {
		return "", "", Die("herdr workspace create: bad response")
	}
	return payload.Workspace.WorkspaceID, payload.RootPane.PaneID, nil
}

func (h Herdr) FocusWorkspace(id string) error {
	_, err := h.Run("workspace", "focus", id)
	return err
}

func (h Herdr) CloseWorkspace(id string) error {
	_, err := h.Run("workspace", "close", id)
	return err
}

// PaneRun types a command into the pane's interactive shell — the herdr twin
// of tmux send-keys; posse never wraps the process. herdr's agent detection
// then classifies whatever starts (claude, codex, ...) on its own.
func (h Herdr) PaneRun(paneID, command string) error {
	_, err := h.Run("pane", "run", paneID, command)
	return err
}

// PaneRead returns the pane's terminal text (plain output, not an envelope).
// The tail is taken client-side: herdr's own --lines counts screen rows from
// the bottom, and the visible buffer pads with blank rows, so asking herdr
// for the last N lines of a quiet pane returns only blanks.
func (h Herdr) PaneRead(paneID string, lines int) (string, error) {
	res, err := h.RunText("pane", "read", paneID, "--format", "text")
	if err != nil {
		return "", err
	}
	text := strings.TrimRight(res, "\n\t ")
	if lines > 0 {
		all := strings.Split(text, "\n")
		if len(all) > lines {
			all = all[len(all)-lines:]
		}
		text = strings.Join(all, "\n")
	}
	return text, nil
}

// AgentPrompt submits text to an agent (unique name or hosting pane id).
// wait=true blocks until the first settled idle|done|blocked state.
func (h Herdr) AgentPrompt(target, text string, wait bool, timeoutMS int) (json.RawMessage, error) {
	args := []string{"agent", "prompt", target, text}
	if wait {
		args = append(args, "--wait")
	}
	if timeoutMS > 0 {
		args = append(args, "--timeout", strconv.Itoa(timeoutMS))
	}
	return h.Run(args...)
}

func (h Herdr) AgentWait(target string, until []string, timeoutMS int) (json.RawMessage, error) {
	args := []string{"agent", "wait", target}
	for _, u := range until {
		args = append(args, "--until", u)
	}
	if timeoutMS > 0 {
		args = append(args, "--timeout", strconv.Itoa(timeoutMS))
	}
	return h.Run(args...)
}

// AgentSendKeys presses keys in an agent's pane — herdr's canonical key
// names ("esc", "enter", ...), not text. A prompt is submitted with
// AgentPrompt; this is for the screens that hold the keyboard before the
// composer does and that no prompt can reach (rangerhq-7sbo).
func (h Herdr) AgentSendKeys(target string, keys ...string) error {
	_, err := h.Run(append([]string{"agent", "send-keys", target}, keys...)...)
	return err
}

// AgentDetection is the part of `agent explain` posse reads: the state herdr
// settled on, the manifest rule that produced it, and whether it was
// produced by a rule at all.
//
// The rule id is one point — "blocked" alone does not say what is on
// screen, and a startup splash the launcher may clear and a permission
// dialog it must never answer are both blocked.
//
// Whether a rule matched at all is the other. herdr answers `idle` for a
// pane it has identified as a known agent even when NO rule matched, and
// says so in the same object; the two shapes are (rangerhq-3hb5, both
// verified against real captures):
//
//	seen   {"state":"idle","matched_rule":{"id":"live_prompt_box",…},
//	        "visible_idle":true,"fallback_reason":null}
//	guess  {"state":"idle","matched_rule":null,"visible_idle":false,
//	        "fallback_reason":"default_known_agent_idle_fallback"}
//
// `null` for matched_rule decodes to the zero Rule, so an empty Rule.ID is
// how a guess reads here.
type AgentDetection struct {
	State string `json:"state"`
	Rule  struct {
		ID string `json:"id"`
	} `json:"matched_rule"`
	VisibleIdle bool `json:"visible_idle"`
	// FallbackReason is herdr's own word for why it guessed —
	// "default_known_agent_idle_fallback" is the one dispatch meets. It is
	// reported, never tested: Seen asks for positive evidence instead, so a
	// fallback herdr names differently tomorrow is still not readiness.
	FallbackReason string `json:"fallback_reason"`
	// EvaluatedRules is herdr's working: every rule it tried, and what the
	// region that rule reads actually held. Read only when nothing matched
	// — see WhatHerdrSaw and ranger-base-3j8 for why a launch failure that
	// does not say this costs a hand-launch and a peek to diagnose.
	EvaluatedRules []EvaluatedRule `json:"evaluated_rules"`
}

// EvaluatedRule is one line of herdr's working: a manifest rule, whether it
// fired, the screen region it reads, and — the part that matters when
// NOTHING fired — how many bytes were in that region and a preview of them.
//
// An empty region is the diagnosis that took a hand-launch to get on
// ranger-base-3j8: grok's three idle recognizers all read OSC chrome that a
// fresh grok pane has not emitted yet, so `region_bytes: 0` on osc_title is
// "the CLI has not spoken yet", while 600 bytes of splash text under
// `whole_recent` is "the CLI is up and sitting on a screen posse does not
// know". Those two need opposite responses and the old message described
// them identically.
type EvaluatedRule struct {
	ID       string `json:"id"`
	Matched  bool   `json:"matched"`
	Region   string `json:"region"`
	State    string `json:"state"`
	Evidence struct {
		RegionBytes   int    `json:"region_bytes"`
		RegionPreview string `json:"region_preview"`
	} `json:"evidence"`
}

// Seen reports whether herdr actually recognized what is on the screen,
// rather than guessing from the fact that a known agent lives in the pane.
// Positive evidence only: a matched rule, or chrome herdr can see.
func (d AgentDetection) Seen() bool { return d.Rule.ID != "" || d.VisibleIdle }

// AgentExplain asks herdr why an agent is in the state it is in. `explain
// --json` prints a bare object, not a result envelope — Run hands that back
// verbatim.
func (h Herdr) AgentExplain(target string) (AgentDetection, error) {
	var det AgentDetection
	res, err := h.Run("agent", "explain", target, "--json")
	if err != nil {
		return det, err
	}
	if err := json.Unmarshal(res, &det); err != nil {
		return det, Die("herdr agent explain %s: bad response", target)
	}
	return det, nil
}
