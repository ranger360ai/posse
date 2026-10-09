package posse

// Agent personalities: agents/<name>.md — a markdown prompt body with a
// flat-YAML frontmatter block (same subset as everything else). The full
// shape is a Persona Intent Document (docs/adr/0001-persona-intent-documents.md);
// every key but `name` is optional:
//
//   ---
//   name: ops
//   description: terse ops copilot
//   runtime: claude             # claude | codex | grok | runtimes/<name>.yaml (ADR 0002)
//   labels: [ops]
//   route_order: 40             # tiebreak among label matches; lower first (default 50)
//   intents: [design]           # inventory slugs — read by humans/tools, not here
//   allow: [Bash(bd:*)]         # permission rules added to the repo floor
//   deny: [Bash(git push:*)]    # permission rules removed, deny wins
//   metrics: [closed-no-reopen] # metric-catalog ids — read by h2c, not here
//   envs: [gh]                  # env sets this persona's sessions receive
//   skills: [dataviz]           # skills bound to this persona (ADR 0007)
//   sockets: [herdr]            # container: host sockets the cage mounts (ADR 0002 §3)
//   trust_project_config: true  # let the runtime read the session dir's own config
//   ---
//   You are the operations copilot of the crew.
//
// `envs` is not decoration on a claude PID. A PID whose `deny:` shims the
// runtime's OWN credential binary — `Bash(security:*)`, the keychain-CLI
// tripwire every crew PID carries — launches only with
// CLAUDE_CODE_OAUTH_TOKEN among the env-set names it injects, and REFUSES
// otherwise, unwaivably by `--allow-degraded` (ADR 0042 D2). The shim is the
// design, so the fix is the mint and never dropping the rule; `posse agent
// check` warns when a PID is in that state (ranger-base-sl5sg).
//
// `runtime` names the launch profile (runtime.go); the runtime's template
// renders {file} (shell-quoted path of the .md, so the prompt body itself
// is what the CLI receives), {memory}, and {allow}/{deny} via the runtime's
// native realizer — or to nothing when the list is empty. `command` is the
// escape hatch: a template for this PID's *own* runtime only; a launch
// that overrides to another runtime uses that runtime's built-in template.
// The rendered command is typed into the session's shell like any recipe
// command — posse never sits between the multiplexer and the process.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ClaudeFleetSettings is the --settings JSON every unattended claude
// session should carry (rangerhq-4e5). Claude Code's auto-mode
// environment-setup dialog opens right after a turn ends whenever the
// session is in auto mode and the operator has never answered it — with
// "Set it up" preselected, so a dispatcher's text+Enter lands inside a
// wizard that configures shell-history/repo scanning. The dialog is gated
// on the auto-mode-setup skill being enabled; turning that skill off via
// skillOverrides suppresses it without touching auth. (CLAUDE_CODE_SIMPLE /
// --bare would also suppress it but never reads OAuth credentials, so a
// subscription-authenticated fleet lands on "Not logged in".)
//
// permissions.defaultMode: auto is the second layer of the OPERATOR
// DIRECTIVE ClaudeFleetFlags' --permission-mode auto already carries
// (rangerhq-qs5r, layer 1; rangerhq-slq6, this layer). A launch that loses
// the flag — template drift, a future CLI arg rename, a path that builds
// its own argv and forgets it — still lands auto from the settings payload,
// because it never loses --settings: DefaultAgentCommand renders both flags
// on the same line, so a line missing --permission-mode is a line that
// dropped one flag, not the whole template. Confirmed on claude 2.1.247
// (grep -a of the installed binary): the settings key is
// `["permissions","defaultMode"]`, and its accepted-without-a-trust-prompt
// set includes "auto" alongside "acceptEdits" and "bypassPermissions" — the
// same vocabulary --permission-mode takes. The CLI flag still wins when
// both are present (argv over settings is the general precedence), so this
// changes nothing about a launch that keeps its flag; it only gives a
// flag-stripped launch somewhere to fall back to instead of the CLI's own
// default, which is exactly the risk unattended_live_test.go's file
// comment names: that default has moved once already and does not error
// when it does.
//
// WHAT IS DELIBERATELY NOT HERE (ranger-base-d3fwo):
// `permissions.blockReadsOutsideWorkingDirectories`. The bead asked for it
// — the auto-mode outside-read notice names that key, and a persona that
// stops on the notice is a blocked session. It belongs on this line in
// neither value: `false` leaves the notice armed, because claude's guard
// tests strictly true, and `true` silences it by refusing every read
// outside the working directories. The launch answers that question where
// claude records the answer instead (ClaudeOutsideReadSeenKey, trust.go).
// A future launch that wants it here is welcome to it; it should read that
// measurement first.
//
// autoMemoryEnabled: false — the fleet writes no auto-memory (ranger-base-7uhip).
// claude resolves its auto-memory directory from the git WORKING-COPY root, not
// the session cwd, so every seat launched into ~/.posse/worktrees/posse/<session>
// resolved to the OPERATOR's own project memory and appended there. MEASURED
// 2026-09-06 (ranger-base-7uhip): 114 memory files under the ONE directory
// ~/.claude/projects/<sanitized main checkout>/memory/ carrying ~100 distinct
// originSessionIds, MEMORY.md at 199 lines against a harness cap of 200 past
// which the tail is cut; ZERO of the 1470 per-worktree project dirs has a
// memory/ subdir at all. So the fleet was filling a 200-line index the operator
// reads and no persona owns, and pruning it buys about a day at today's close
// rate.
//
// Turning it OFF rather than redirecting it is the ruling of the bead's design
// parent (ranger-base-bmr1c): ORDERS.md is the persona memory the constitution
// names and posse already commits (memoryland.go), and the auto-memory was a
// second, unowned channel beside it. A redirect to the persona dir would keep
// that second channel and only move it — and it would make this const
// per-persona, which is a shape change for a payload that is a const on purpose.
//
// The COST, named because it is real: a seat also stops READING that index, and
// the ~150 lessons in it today are largely posse engineering lore written by
// prior seats. ORDERS.md, AGENTS.md and docs/ are the channels that remain, and
// they are the ones with an owner and a commit.
//
// MEASURED to work at this scope, two headless arms on claude 2.1.263 that
// differ in this one key (ranger-base-7uhip): with `autoMemoryDirectory` alone
// the session names that directory back; adding `autoMemoryEnabled:false` it
// answers NONE. `--settings` is flagSettings; for the sibling key
// `autoMemoryDirectory` the resolver's own scope list ranks it above
// local/project/user and below policy alone (project scope is ignored for that
// key outright, "for security"). What the arms measured for `autoMemoryEnabled`
// is that flag scope beats the DEFAULT — the operator's `~/.claude` names the
// key nowhere today. Flag versus a user-scope `true` (which is what the /config
// toggle writes) is UNMEASURED and not measurable from a seat: the box's
// root-owned policy file pins CLAUDE_CONFIG_DIR, so there is no scratch user
// scope to plant in (ranger-base-i7cy4). If auto-memory ever comes back on for
// the fleet, that contest is the first thing to measure.
// Do NOT reach for CLAUDE_CODE_DISABLE_AUTO_MEMORY
// instead: a launcher-exported variable loses to any settings scope naming the
// same key (ranger-base-rq83c), which is the whole reason the pins travel in
// this payload.
const ClaudeFleetSettings = `{"autoMemoryEnabled":false,"permissions":{"defaultMode":"auto"},"skillOverrides":{"auto-mode-setup":"off"}}`

// DefaultAgentCommand is the claude runtime's template — what a PID with
// neither runtime: nor command: launches with.
const DefaultAgentCommand = `claude ` + ClaudeFleetFlags + ` --append-system-prompt "$(cat {file})" --add-dir {memory} {settings} {skills} {allow} {deny}`

// ClaudeAutoModeDefaults is the sentinel that keeps claude's own
// `autoMode.allow` list alive when the launch adds an entry of its own.
// LOAD-BEARING, and the one thing in this file that is dangerous to get
// wrong (ADR 0070 D2, MEASURED 2026-10-09 on claude 2.1.295): `claude
// auto-mode defaults` prints 17 built-in allow rules, and the binary's
// splice function substitutes them at this literal's FIRST position and
// otherwise returns the user array alone. So an `autoMode.allow` rendered
// without it does not add one exception — it DELETES all 17, for every seat
// that carries the blob. TestQAClaudeFleetAutoModeAllowKeepsTheDefaults
// refuses a payload whose first element is anything else.
const ClaudeAutoModeDefaults = "$defaults"

// ClaudeAutoModeCarveOut is the one standing statement posse makes to
// claude's auto-mode classifier, quoted verbatim from ADR 0070 D2.
//
// WHY A PROSE ENTRY AT ALL. Under `--permission-mode auto` a PID `allow:`
// rule is friction removal and nothing else (ADR 0070 D1): it lets a call
// whose EVERY shell segment matches a rule skip the classifier, and nothing
// posse renders binds the classifier itself. A `cd x && posse peek s`, a
// heredoc-assembled prompt, a `… &` background launch — each goes to the
// classifier whatever the allow list says, and there `posse new` (launching
// a sibling auto-mode claude) and `herdr pane send-keys` (typing into a
// sibling pane) ARE the Create Unsafe Agents and Tmux Self Drive patterns
// to a reader with no context. Those are SOFT rules, and the only thing
// that clears a soft rule is the statement that this instance of the
// pattern is a false positive — which an `allow` entry is, made standing.
//
// ONE fixed text, owned by posse, the same bytes on every PID that earns
// it, and derived from the allow list the PID already carries: no new PID
// key, no flag, no state (ADR 0070's rejected alternatives price the three
// shapes that would have added one). Posse's own verbs only — posse cannot
// know WHY an operator allowed `git push`, and a generated "X is fine to
// allow" per rule is the Auto-Mode Bypass clause written out.
//
// WHAT IS CLAIMED: a REDUCTION in refusals of these verbs in compound
// shapes, never an end to them (D3). The entry is prose read by a model,
// and that reader was measured unstable on argv in the ADR's own session.
// A persona whose work hangs on one of these verbs still writes it in the
// matching shape first — bare verb, prompt text written in a separate call
// — because the shape hole is the one an allow rule DOES close.
const ClaudeAutoModeCarveOut = "Posse fleet session verbs: when this session's permission allow list grants " +
	"them, `posse new`, `posse prompt`, `posse kill`, `posse peek` and `herdr " +
	"pane list|get|read|send-text|send-keys|wait-output` are this persona's own " +
	"verbs. They start, prompt, read, drive and end SIBLING terminal panes on this " +
	"machine, owned by the operator and pre-approved through this persona's " +
	"permission document; every session they launch carries its own permission " +
	"document, gates and `--permission-mode auto`. A flagged Auto-Mode Bypass, " +
	"Tmux Self Drive or Create Unsafe Agents on one of these verbs is a false " +
	"positive — fine to allow — whether the verb stands alone, follows a `cd` or " +
	"`&&`, or takes its prompt text from a heredoc or `$(cat …)`. Keystrokes aimed " +
	"at THIS session's own pane (`$TMUX_PANE`) are not covered, and neither is a " +
	"bare `kill`, `tmux` or `claude`."

// fleetSessionVerbs are the verbs the carve-out speaks for, as the WORDS a
// claude Bash rule would have to stand in front of (ADR 0070 D2's list).
// The carve-out's own text names the same set in prose; this is the half a
// matcher can read.
var fleetSessionVerbs = [][]string{
	{"posse", "new"},
	{"posse", "prompt"},
	{"posse", "kill"},
	{"posse", "peek"},
	{"herdr", "pane", "list"},
	{"herdr", "pane", "get"},
	{"herdr", "pane", "read"},
	{"herdr", "pane", "send-text"},
	{"herdr", "pane", "send-keys"},
	{"herdr", "pane", "wait-output"},
}

// grantsFleetSessionVerb reports whether a PID's allow list grants any of
// the verbs the carve-out speaks for — the one question that decides
// whether the blob carries an `autoMode` key at all.
//
// Read on the rule's WORDS, in claude's dialect: a rule is a command-line
// pattern, `:*` leaves everything after the last word open, and a rule
// without it matches that command line exactly. So `Bash(posse:*)` and
// `Bash(herdr pane:*)` grant (a prefix of the verb's words, left open),
// `Bash(posse new)` grants (the bare verb exactly), `Bash(posse new -l x)`
// grants (a longer line that still BEGINS with the verb), and
// `Bash(posse refresh:*)`, `Bash(posse)` and `Bash(herdr pane send-key:*)`
// grant nothing.
//
// EXACT on the word, deliberately, where gates.go's `grantsGitPushRule`
// over-approximates: that reader raises a lint alarm, where a false
// positive costs a line of prose on screen, and claude's own matcher wants
// a word boundary at `:*` anyway (`Bash(posse pee:*)` reaches no `peek`
// there). Here a false positive would put a standing statement about verbs
// this PID does not hold in front of the classifier, which is the shape
// ADR 0070 rejected for every allow rule posse does not own.
//
// A broad rule earns nothing on purpose: `Bash(*)` and bare `Bash` are
// SUSPENDED on entering auto mode (MEASURED, ADR 0070 Context), so a PID
// holding one of those and nothing else holds no session verb here — and
// the word reading answers that without a case of its own, since neither
// spells a `posse` or `herdr` the verb's first word can match.
func grantsFleetSessionVerb(allow []string) bool {
	for _, rule := range allow {
		if !strings.HasPrefix(rule, "Bash(") || !strings.HasSuffix(rule, ")") {
			continue // Edit, Write, WebFetch, mcp__* — other layers'
		}
		body := strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), ")")
		open := strings.HasSuffix(body, ":*")
		words := strings.Fields(strings.TrimSuffix(body, ":*"))
		if len(words) == 0 {
			continue // Bash() grants nothing
		}
		for _, verb := range fleetSessionVerbs {
			if ruleStandsOn(words, open, verb) {
				return true
			}
		}
	}
	return false
}

// ruleStandsOn reports whether a rule's words can stand where verb's words
// stand. Shorter than the verb it has to be left open by `:*` to reach the
// rest of it; as long or longer it already begins with the verb, and what
// follows is an invocation of that verb whatever it says. The command word
// is matched on its base name (`Bash(/usr/local/bin/posse new:*)`) and a
// `*` inside a word is claude's `.*`, both through gates.go's readers, so
// this file holds no second copy of that dialect.
func ruleStandsOn(words []string, open bool, verb []string) bool {
	if len(words) < len(verb) && !open {
		return false
	}
	for i := range verb {
		if i >= len(words) {
			break
		}
		if i == 0 {
			if !reachesCommand(words[0], verb[0], false) {
				return false
			}
			continue
		}
		if !reachesWord(words[i], verb[i], false) {
			return false
		}
	}
	return true
}

// claudeAutoModeAllowJSON renders the `autoMode` payload for a PID whose
// allow list earns the carve-out, and nil for one that does not — which is
// what makes "otherwise no `autoMode` key at all" (ADR 0070 D2) a property
// of one function rather than of its callers. The sentinel is FIRST and the
// carve-out second; see ClaudeAutoModeDefaults for what the order buys.
//
// An array of strings and nothing else, because of the payload-shape hazard
// the field pin carries (fieldpin.go, ranger-base-i7cy4): ONE wrong-typed
// row voids the whole `--settings`, taking the credential dirs and the
// permission mode with it. ADR 0070 Verification 2 is the live canary for
// exactly that — `claude --settings '<blob>' auto-mode config` must print
// 18 allow entries, where a voided payload prints 17.
//
// One thing a reader of the launch line will trip on: encoding/json escapes
// HTML by default, so the carve-out's `&&` reaches `ps` as `\u0026\u0026`.
// It is the same marshaller the rest of this payload already goes through,
// it parses back to the two characters before any model reads it, and it is
// inert inside the single quotes shellQuote puts around the flag. Compare
// the DECODED element, never the rendered line, when asking what the
// classifier will see.
func claudeAutoModeAllowJSON(allow []string) json.RawMessage {
	if !grantsFleetSessionVerb(allow) {
		return nil
	}
	b, err := json.Marshal(map[string][]string{"allow": {ClaudeAutoModeDefaults, ClaudeAutoModeCarveOut}})
	if err != nil {
		return nil
	}
	return b
}

// ClaudeFleetSettingsJSON is what {settings} carries: ClaudeFleetSettings
// above, plus the env pin this launch cannot express any other way
// (settingsPin) — the credential dirs (credentialDirPin,
// ranger-base-rq83c) and the transport/exec inlets (inletPin,
// ranger-base-rflee), in that order — plus the command-string FIELDS an env
// pin structurally cannot reach (fieldPin, ranger-base-i7cy4), which sit
// beside `env` rather than in it because they are top-level settings keys.
//
// The inlet half is why this function no longer renders the const alone on
// a box with no home directory: the credential-dir rows need a home to name
// and the inlet rows do not, so the launch keeps its exec and transport pin
// even where it cannot name a credential store.
//
// The pin has to travel INSIDE this payload rather than beside it. A second
// `--settings` on the line does not add a source: measured on claude
// 2.1.259, the last occurrence REPLACES the first, so an appended pin-only
// flag would take the fleet's permission mode and skill override off the
// line while it was busy fixing the credential dir.
//
// The const stays the readable half — it is what a reader of the launch
// line is looking for, and outsideread_test.go pins what it must not carry.
// This function only merges; key order is encoding/json's, which sorts, so
// the rendered line is stable across launches. A const that stopped being
// JSON, or a box with no home directory, renders the const alone: the
// launch still carries its permission mode, and the pin's absence is what
// TestQAClaudeFleetSettingsJSONCarriesTheCredentialDirPin refuses.
//
// allow is the PID's own `allow:` list, and the ONE thing in this payload
// that is a property of the persona rather than of the box: a PID whose
// allow list grants posse's or herdr's session verbs gets the auto-mode
// carve-out beside the pins (ADR 0070 D2, claudeAutoModeAllowJSON), and
// every other PID — allow: nil included — renders byte-for-byte what it
// rendered before the key existed. It arrives as a parameter rather than
// being read here because the render site already holds it
// (RenderCommandForModel's ag.Allow), and a second reader of the PID file
// would be a second answer to what this persona is allowed.
//
// A degraded return above drops the carve-out with the pins, and that is
// the right order of loss: the carve-out removes friction, the pins are
// the security guarantee, and the pin's absence is the condition the
// credential-dir pin already refuses. Nothing silently half-renders.
func ClaudeFleetSettingsJSON(allow []string) string {
	pin := settingsPin()
	if len(pin) == 0 {
		return ClaudeFleetSettings
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(ClaudeFleetSettings), &m); err != nil {
		return ClaudeFleetSettings
	}
	env := make(map[string]string, len(pin))
	for _, v := range pin {
		env[v.Key] = v.Value
	}
	b, err := json.Marshal(env)
	if err != nil {
		return ClaudeFleetSettings
	}
	m["env"] = b
	if !applyFieldPin(m) {
		return ClaudeFleetSettings
	}
	if am := claudeAutoModeAllowJSON(allow); am != nil {
		m["autoMode"] = am
	}
	out, err := json.Marshal(m)
	if err != nil {
		return ClaudeFleetSettings
	}
	return string(out)
}

type AgentFile struct {
	Name        string
	Description string
	Command     string   // template for the PID's own runtime; may contain {file} {memory} {allow} {deny} {skills} {mode}
	Runtime     string   // launch profile name (ADR 0002); "" = default (config default_runtime, else claude)
	Tier        string   // strong|standard|fast (ADR 0003); "" = config default_tier, else strong
	TierFloor   string   // lowest tier this persona may run at; enforced by the parity check (rangerhq-2uq)
	Cage        string   // minimum cage tier (ADR 0002 §5): shims | seatbelt | container; "" = shims
	Writable    []string // seatbelt: extra writable paths (consumed by rangerhq-5vt)
	Egress      []string // container: hosts allowed out (consumed by rangerhq-89a); implies cage: container
	Sockets     []string // container: host sockets passed in (ADR 0002 §5); only `herdr` is known, off by default
	WorkPrompt  string   // the PID's `## Work prompt` section, appended verbatim to every work prompt (ADR 0005 §3)
	Path        string
	Body        string
	MemoryDir   string   // persona-private memory: RHQ_HOME/personas/<name>/
	Labels      []string // beads labels this persona picks up (dispatch routing)
	Intents     []string // intent-inventory slugs this persona serves (descriptive)
	Allow       []string // permission rules added to the repo-global allowlist
	Deny        []string // permission rules removed regardless of any allowlist
	Metrics     []string // metric-catalog ids this persona is judged by
	Envs        []string // env-set names its sessions get — the only implicit env a persona receives (rangerhq-f2b)
	Skills      []string // skills bound to this persona (ADR 0007); declared means required — the parity check refuses a runtime that cannot materialize them
	// SkillsStateDir is where the launch renders the tree {skills} points
	// at: RHQ_HOME/state/skills/<persona>/claude. claude's plugin shape is
	// the only verified surface, and a template-only runtime with
	// skills_flag: borrows the same dir (skills.go).
	SkillsStateDir string
	// TrustProjectConfig opts this persona into a runtime reading
	// configuration out of the session directory (Runtime.ProjectConfig).
	// Off by default: directory trust can make project-owned executable
	// channels live before any turn. A launch whose runtime-specific file or
	// keyed JSON predicate hits degrades unless this is set (ADR 0002).
	TrustProjectConfig bool
	// RouteOrder is `route_order:` — where this PID sits among the personas
	// whose labels match a bead. Lower goes first; absent is
	// RouteOrderDefault, so a lane can be promoted or demoted without
	// negative numbers. See Route: the key exists so that "which persona
	// gets an unassigned bead" is a decision someone made, not a property
	// of how the agents dir happens to sort.
	RouteOrder int
}

func agentFrontmatter(data string) (front []string, body string) {
	lines := strings.Split(data, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, data
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[1:i], strings.Join(lines[i+1:], "\n")
		}
	}
	return nil, data
}

// RouteOrderDefault is where a PID with no `route_order:` sits in the label
// race. Mid-scale on purpose: a lane is promoted below it and demoted above
// it, both without a minus sign, and an instance that never touches the key
// keeps exactly the order it had (every PID ties, the tiebreak decides).
const RouteOrderDefault = 50

// parseRouteOrder reads `route_order:`. ok is false for absent AND for
// malformed, which both mean "this PID stated nothing usable" and take the
// default — a PID that would not load because of one mistyped ordering hint
// is a lane that goes silent, which is the failure this key exists to stop.
// `posse agent check` reports the malformed spelling (pidcheck.go) so it is
// not silent, only non-fatal.
func parseRouteOrder(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (a *App) LoadAgent(name string) (*AgentFile, error) {
	p := filepath.Join(a.AgentsDir, name+".md")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, Die("no such agent: %s (looked in %s)", name, a.AgentsDir)
	}
	front, body := agentFrontmatter(string(b))
	ag := &AgentFile{
		Name:        name,
		Description: yamlGetLines(front, "description"),
		Command:     yamlGetLines(front, "command"),
		Runtime:     yamlGetLines(front, "runtime"),
		Tier:        yamlGetLines(front, "tier"),
		TierFloor:   yamlGetLines(front, "tier_floor"),
		Cage:        yamlGetLines(front, "cage"),
		Writable:    yamlListLines(front, "writable"),
		Egress:      yamlListLines(front, "egress"),
		Sockets:     yamlListLines(front, "sockets"),
		Path:        p,
		Body:        body,
		Labels:      yamlListLines(front, "labels"),
		Intents:     yamlListLines(front, "intents"),
		Allow:       yamlListLines(front, "allow"),
		Deny:        yamlListLines(front, "deny"),
		Metrics:     yamlListLines(front, "metrics"),
		Envs:        yamlListLines(front, "envs"),
		Skills:      yamlListLines(front, "skills"),
	}
	ag.RouteOrder = RouteOrderDefault
	if n, ok := parseRouteOrder(yamlGetLines(front, "route_order")); ok {
		ag.RouteOrder = n
	}
	ag.TrustProjectConfig = yamlGetLines(front, "trust_project_config") == "true"
	if n := yamlGetLines(front, "name"); n != "" {
		ag.Name = n
	}
	if len(ag.Egress) > 0 && ag.Cage == "" {
		ag.Cage = CageContainer // egress: implies the container tier
	}
	ag.WorkPrompt = BodySection(body, "## Work prompt")
	// Command stays as authored: "" means "the runtime's template" (ADR
	// 0002) — filling in claude's here would make every PID claude-shaped
	// on its own runtime.
	ag.MemoryDir = filepath.Join(a.PersonasDir(), name)
	ag.SkillsStateDir = filepath.Join(a.StateDir, "skills", name, "claude")
	return ag, nil
}

// PersonasDir holds per-persona private memory (standing orders, notes) —
// the one memory kind posse owns; project memory belongs to beads.
func (a *App) PersonasDir() string { return filepath.Join(a.Home, "personas") }

// memoryIgnoreSeed is the per-persona ignore this dir is seeded with.
//
// The memory dir is where a persona works, not only where it writes prose,
// and LandPersonaMemory sweeps ALL of it: five `*.out` captures of test
// stdout were committed as one persona's standing orders before this file
// existed. The sweep is deliberately not narrowed to a list of blessed
// names: of the 29 files tracked under `personas/` on the instance that
// measured this, nine are neither an ORDERS.md nor under a `pending/`, and
// only five of those nine are the evidence — the other four are a rollback
// patch and three deliberate notes and scripts. An allowlist drops those
// four SILENTLY, which is the defect the landing exists to end reached from
// the other side; an ignore leaves them and takes the five out by name.
//
// So the answer is git's own, per persona and in the persona's own hands:
// `status` and `add` both honor this file, so a path named here never
// reaches the change list and cannot reach the commit. The two patterns are
// a starting list, not a ruling — a persona that wants a `.out` kept deletes
// the line, and one whose evidence is `.json` adds it.
const memoryIgnoreSeed = `# Not memory. posse commits this directory on the persona's behalf when a
# session ends — path-limited, scanned for credential shapes, never pushed —
# so a file here that belongs on a bead or in a scratch dir belongs on this
# list instead. It is yours to grow; nothing rewrites it once it exists.
*.out
*.log
`

// EnsureMemoryDir materializes the persona's memory dir at launch time,
// seeding an ORDERS.md the persona (or you) can grow and the ignore that
// keeps the rest of the dir from being committed as memory.
//
// Each file is seeded only when it is absent, so this is safe to run at
// every launch and never touches what a persona has written.
func (ag *AgentFile) EnsureMemoryDir() error {
	if err := os.MkdirAll(ag.MemoryDir, 0o755); err != nil {
		return err
	}
	orders := filepath.Join(ag.MemoryDir, "ORDERS.md")
	if _, err := os.Stat(orders); err != nil {
		seed := "# Standing orders — " + ag.Name + "\n\n(persona-private memory; injected at every launch)\n"
		if err := os.WriteFile(orders, []byte(seed), 0o644); err != nil {
			return err
		}
	}
	ignore := filepath.Join(ag.MemoryDir, ".gitignore")
	if _, err := os.Stat(ignore); err != nil {
		return os.WriteFile(ignore, []byte(memoryIgnoreSeed), 0o644)
	}
	return nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// renderPlaceholder expands one placeholder to text — or removes it (and
// the preceding space) when the text is empty, so templates mentioning it
// cost nothing.
func renderPlaceholder(cmd, placeholder, text string) string {
	if text == "" {
		cmd = strings.ReplaceAll(cmd, " "+placeholder, "")
		return strings.ReplaceAll(cmd, placeholder, "")
	}
	return strings.ReplaceAll(cmd, placeholder, text)
}

// RenderCommandFor renders the launch command on the given runtime: the
// PID's own command: when the runtime is the PID's own, else the runtime's
// template; {file}/{memory} shell-quoted, {allow}/{deny} through the
// runtime's native realizer (nothing for template-only runtimes — every
// gate goes to the wall), {model} to the runtime's model flag for the tier
// (ADR 0003; empty when unmapped), {skills} to the flag pointing at the
// rendered skills tree (ADR 0007; empty when the PID binds none or the
// runtime has no surface — the parity check has already ruled on that),
// {mode} to the flag selecting the custom mode the launch rendered this
// PID into (ADR 0062 D1; empty on every runtime that declares no such
// channel).
// ownRuntime is what the PID would run on with no override (ADR 0002 §1).
func (ag *AgentFile) RenderCommandFor(rt *Runtime, ownRuntime, tier string, writable ...string) string {
	return ag.RenderCommandForModel(rt, ownRuntime, tier, "", writable...)
}

// RenderCommandForModel is RenderCommandFor with an EXACT model id (ADR
// 0053): when model is not empty it is what {model} renders, in place of
// the id the runtime's tier map would have named. Everything else about the
// render is identical — the PID's own command:, the gates, the skills, the
// settings pin and the unattended mode — which is what keeps a canary
// launch a persona launch (D2).
//
// model == "" is every ordinary launch and renders byte-for-byte what it
// rendered before this existed.
func (ag *AgentFile) RenderCommandForModel(rt *Runtime, ownRuntime, tier, model string, writable ...string) string {
	// The one expression for "which template" — shared with the PID-channel
	// reading (pidchannel.go), because a reading taken from a template no
	// launch renders is a reading about nothing (ADR 0062 D3).
	tmpl := ag.LaunchTemplate(rt, ownRuntime)
	out := strings.ReplaceAll(tmpl, PIDChannelFile, shellQuote(ag.Path))
	out = strings.ReplaceAll(out, "{memory}", shellQuote(ag.MemoryDir))
	modelText := rt.ModelText(tier)
	if model != "" {
		modelText = rt.ExactModelText(model)
	}
	out = renderPlaceholder(out, "{model}", modelText)
	var r Realized
	if rt.Realize != nil {
		r = rt.Realize(ag.Allow, ag.Deny, ag.MemoryDir, writable...)
	}
	skills, _ := rt.SkillsText(ag.SkillsStateDir, ag.Skills)
	out = renderPlaceholder(out, "{settings}", rt.FleetSettingsText(ag.Allow))
	out = renderPlaceholder(out, "{skills}", skills)
	// {mode} is the PID channel for a CLI with no launch-time system flag:
	// it selects the custom mode the launch rendered into the session tree
	// from this very PID (ADR 0062 D1, personamode.go). Rendered off the
	// runtime's declared PersonaMode seam and so empty — placeholder and
	// preceding space both gone — on every runtime that declares none.
	//
	// Beside {skills} because it is the same shape one channel over: the
	// text here only POINTS at something, and the launch materializing it
	// is what makes the pointer true. The difference is what a stale
	// pointer costs — a missing skill is a degraded persona, a mode file
	// that was never written is no persona at all, which is why that write
	// refuses the launch and this render cannot.
	out = renderPlaceholder(out, PIDChannelMode, rt.PersonaModeText(ag.Name))
	out = renderPlaceholder(out, "{allow}", r.Allow)
	out = renderPlaceholder(out, "{deny}", r.Deny)
	// The unattended mode is a launch guarantee, not a template detail: a
	// PID's own command: is the one template posse did not write, and a
	// persona session that starts asking for approvals is a session nobody
	// is watching (rangerhq-qs5r). The credential-dir pin is the same kind
	// of guarantee for the same kind of template (ranger-base-rq83c).
	return rt.EnsureUnattended(rt.EnsureSettingsPin(out))
}

// RenderCommand renders on the PID's own runtime with claude's realizer
// semantics when the runtime is unknown to this process — the legacy path
// (tests, tools that have no App at hand). Launch sites use
// RenderCommandFor with a loaded runtime.
func (ag *AgentFile) RenderCommand() string {
	rt := &Runtime{Name: DefaultRuntime, Command: DefaultAgentCommand, Realize: realizeClaude, Skills: skillsClaude, Unattended: ClaudeFleetFlags, FleetSettings: ClaudeFleetSettingsJSON, SettingsPin: credentialDirPinJSON}
	own := ag.Runtime
	if own == "" {
		own = DefaultRuntime
	}
	if own != DefaultRuntime {
		for i := range builtinRuntimes {
			if builtinRuntimes[i].Name == own {
				x := builtinRuntimes[i]
				rt = &x
			}
		}
	}
	tier := ag.Tier
	if tier == "" {
		tier = DefaultTier
	}
	return ag.RenderCommandFor(rt, own, tier)
}

// ExampleAgentsDir is the reference shelf's agents/ — where `posse init`
// puts the shipped example PIDs (ranger-base-qajs). Derived rather than
// read straight off the field so an App built by hand in a test, with only
// Home set, still names a path under that home instead of a relative one.
//
// It is deliberately NOT AgentsDir and nothing loads from it: an example
// that is loadable is a lane, and a lane nobody staffed wins beads.
func (a *App) ExampleAgentsDir() string {
	dir := a.ExamplesDir
	if dir == "" {
		dir = filepath.Join(a.Home, "examples")
	}
	return filepath.Join(dir, "agents")
}

// ListAgents returns agent names (agents/*.md, extension stripped), sorted
// by persona name.
//
// The sort is explicit, and it is on the name rather than the file: this
// list is dispatch's tiebreak among personas that match a bead equally
// (Route), so its order is a decision the code makes and can be read, not
// whatever os.ReadDir hands back. ReadDir already returns filenames sorted,
// so this changes nothing an instance can see except the one shape where
// stripping `.md` reorders a pair (`a.md`, `a-x.md`) — there, the persona
// names are what the operator reads, so the persona names are what sorts.
func (a *App) ListAgents() []string {
	ents, _ := os.ReadDir(a.AgentsDir)
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(out)
	return out
}

// CanonAgent resolves a name that came from outside — an issues.jsonl
// assignee, a config value — to the agents-dir spelling of the PID it names,
// or reports that it names none.
//
// LoadAgent is not that check: it joins a *path* (AgentsDir/<name>.md), so it
// accepts spellings that are not persona names at all (for a PID `pid`:
// `./pid`, `pid/../pid` on any filesystem; `Pid` wherever the agents dir is
// case-insensitive, which is the APFS default) — and the string it accepted is
// the one the caller then writes into a session name, BD_ACTOR, RHQ_PERSONA
// and the PID path it cats. One persona must have one identity everywhere it
// is written down, and that identity is the directory entry: ValidName first
// (that is what rules out the path-shaped spellings, by construction), then
// the exact entry, then the entry that differs only in case — so a name
// resolves the same way whether or not the filesystem folds it (rangerhq-c6u6).
func (a *App) CanonAgent(name string) (string, bool) {
	if !ValidName(name) {
		return "", false
	}
	names := a.ListAgents()
	for _, e := range names {
		if e == name {
			return e, true
		}
	}
	for _, e := range names {
		if strings.EqualFold(e, name) {
			return e, true
		}
	}
	return "", false
}

// PlainPaneHint is what `posse new <name>` says when <name> is the name of a
// persona in this home and no `--agent` came with it: that this is a plain
// pane, and what the persona launch would have been. "" in every other case.
//
// WHY A HINT AND NOT A LAUNCH (ranger-base-qnn6j asked for one of the two,
// cited). A bare `posse new <name>` is not a mistake posse may correct: ADR
// 0008 §1 marks every `posse new` Crew — the operator made it to talk to it
// — so the name is a SESSION name, and a session named after a persona is a
// legitimate thing to open (a pane to read that persona's memory in, a
// second shell in their repo). And launching the persona would bind what
// only a PID may bind: ADR 0002's wall — cage tier, allow/deny, envs,
// skills — plus BD_ACTOR and RHQ_PERSONA, none of which the operator asked
// for and all of which a session then carries for its whole life. Inferring
// that from a name is the kind of near-right help that is worst when it is
// right most of the time.
//
// So the plain pane is created exactly as typed and the hint goes to stderr
// beside it: the operator who meant the pane has lost nothing, and the one
// who meant the persona has the line to retype. What they were missing was
// never a refusal — it was ever hearing that the two differ.
func (a *App) PlainPaneHint(name, agent string) string {
	if agent != "" {
		return ""
	}
	canon, ok := a.CanonAgent(name)
	if !ok {
		return ""
	}
	return fmt.Sprintf("posse: %q names a persona here and no --agent was given, so this is a plain pane — no PID, no wall, no BD_ACTOR (ADR 0002, ADR 0008 §1).\n"+
		"  the persona launch is: posse new %s --agent %s --dir <repo>", name, name, canon)
}

// HardRiskLines are the four crew-wide guardrails from ADR 0001. Every
// PID's ## Guardrails restates them verbatim so an audit can grep for
// them; the scaffold emits exactly this text.
const HardRiskLines = `Hard risk lines (crew-wide, verbatim):
1. Money: no autonomous spending, subscribing, or committing — ever.
2. Writing under the operator's name: drafts welcome, publishing never.
3. Deployed real-world systems: updates only with explicit per-change permission.
4. Visibility: nothing moves to a wider audience than the source it came
   from; where the audience is unclear, it does not move.`

// PIDHeadings are the body sections of a PID in contract order (ADR 0001).
// ScaffoldAgent emits them; `posse agent check` lints against them.
// "## Work prompt" (ADR 0005 §3) is optional: the linter warns, not fails.
var PIDHeadings = []string{
	"## Who you are", "## Intents", "## How you work", "## Guardrails", "## Handoffs",
	"## Done", "## Blocked", "## Memory", "## Metrics", "## Work prompt",
}

// OptionalPIDHeadings may be absent without failing `posse agent check`.
var OptionalPIDHeadings = map[string]bool{"## Work prompt": true}

// BodySection returns the text of one `## ` section of a PID body (from
// the heading line to the next `## ` heading or the end), trimmed; "" when
// absent. The same splitter serves LoadAgent and posse agent check.
func BodySection(body, heading string) string {
	i := strings.Index("\n"+body, "\n"+heading+"\n")
	if i < 0 {
		return ""
	}
	rest := body[i+len(heading)+1:] // "\n"+body offsets by one; skip heading + its newline
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// ScaffoldAgent writes a starter agent file if it doesn't exist. The
// starter is the PID shape (ADR 0001): every frontmatter key present —
// runtime: claude instead of a command: (ADR 0002), lists empty with a
// commented hint — and every body heading in contract
// order with a one-line hint, so a new persona starts as a PID rather
// than a job title. The output parses with LoadAgent as-is.
func (a *App) ScaffoldAgent(name string) (string, error) {
	if !ValidName(name) {
		return "", Die("bad agent name '%s' (letters, digits, - and _; may not start with -)", name)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(a.AgentsDir, name+".md")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return p, os.WriteFile(p, []byte(scaffoldPID(name)), 0o644)
}

func scaffoldPID(name string) string {
	return `---
name: ` + name + `
description: one line — what this persona is for (shown in listings)
runtime: claude
# tier: strong (design, audit, spec — anything judged) | standard (building, testing, ops) | fast (mechanical)
tier: standard
# tier_floor: standard      # refuse to run below this tier — for guardrails that live only in this PID's prose
labels:
  # bead labels this persona picks up (dispatch routing), e.g.
  # - code
# route_order: 50          # tiebreak when several personas' labels match one bead — lower first, default 50, ties by persona name
intents:
  # intent-inventory slugs this persona serves, e.g.
  # - build-features
allow:
  # permission rules added to the repo floor, e.g.
  # - Bash(bd:*)
deny:
  # The commit wall's L1 half (ADR 0002's layer table, rangerhq-lmq9):
  # refuses ` + "`git commit`" + ` unless argv carries ` + "`--`" + ` with a pathspec, so a
  # persona sharing a checkout commits only files it names. Every shipped
  # PID carries it, and it is the half that lands on the typed line, before
  # git runs, in repos where no L3 hook is installed.
  - Bash(git commit unless --)
  # more permission rules removed regardless of any allowlist, e.g.
  # - Bash(git push:*)
# envs: [crew]             # env sets this persona's sessions receive, by name
# A deny over the RUNTIME's own credential binary (` + "`Bash(security:*)`" + ` on
# claude) shims the runtime itself and not just the persona, so such a PID
# launches ONLY with CLAUDE_CODE_OAUTH_TOKEN in one of these sets and refuses
# otherwise, unwaivably (ADR 0042 D2). The shim is the design, so the fix is
# the mint and never dropping the rule: mint it with ` + "`claude setup-token`" + `,
# keep it in an env set (mode 600, never in the repo), name that set here.
metrics:
  # metric-catalog ids (1–2), e.g.
  # - closed-no-reopen
skills:
  # skills this persona's work depends on — names under RHQ_HOME/skills
  # (<name>/SKILL.md); materialized at launch, and a runtime that cannot
  # materialize them refuses to launch (ADR 0007), e.g.
  # - dataviz
---
You are <Name>, the <role> of the <crew>.

## Who you are
What you decide or produce; your bias; what you do not do.

## Intents
| intent | mode | done when |
|---|---|---|
| <slug from frontmatter> | crew or fleet or advisory | the one sentence a reviewer checks the closed bead against |

## How you work
The working method: ` + "`bd show <id>`" + ` first, read before write, what you output.

## Guardrails
` + HardRiskLines + `

Persona-specific:
- Prose here; where a rule can enforce it, add it to ` + "`deny:`" + ` too.

## Handoffs
Take from whom, hand to whom, in what form.

## Done
Definition of done, then ` + "`bd comments add <id> <summary>` and `bd close <id>`" + `.

## Blocked
What you say when blocked (exactly what you need) — and that you stop.

## Memory
Read $POSSE_PERSONA_DIR/ORDERS.md at start; append durable lessons there.

## Metrics
- ` + "`<id from frontmatter>`" + `: what it measures, in words, and the bd query idea.

## Work prompt
The standing per-bead instruction for this persona, appended verbatim to every
dispatched work prompt after the escalation ladder (ADR 0005). Optional; one
or two sentences.
`
}

func (a *App) DeleteAgent(name string) error {
	return os.Remove(filepath.Join(a.AgentsDir, name+".md"))
}
