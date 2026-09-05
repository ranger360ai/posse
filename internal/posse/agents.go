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
//   overflow: false             # never move this lane to the plan guard's second pool (ADR 0010)
//   ---
//   You are the operations copilot of the crew.
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
const ClaudeFleetSettings = `{"permissions":{"defaultMode":"auto"},"skillOverrides":{"auto-mode-setup":"off"}}`

// DefaultAgentCommand is the claude runtime's template — what a PID with
// neither runtime: nor command: launches with.
const DefaultAgentCommand = `claude ` + ClaudeFleetFlags + ` --append-system-prompt "$(cat {file})" --add-dir {memory} {settings} {skills} {allow} {deny}`

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
func ClaudeFleetSettingsJSON() string {
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
	out, err := json.Marshal(m)
	if err != nil {
		return ClaudeFleetSettings
	}
	return string(out)
}

type AgentFile struct {
	Name        string
	Description string
	Command     string   // template for the PID's own runtime; may contain {file} {memory} {allow} {deny} {skills}
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
	// NoOverflow is `overflow: false` on the PID: this lane is never moved
	// to the plan guard's overflow runtime (ADR 0010 §2c), whatever the
	// parity check says. The opt-out exists because parity cannot see
	// everything a pool differs in — a lane that drives through repo shell
	// scripts stalls on a target whose unattended mode refuses to run an
	// unknown local script, and no gate matrix can express that. Absent =
	// eligible: the default is the one that costs nothing to be wrong about
	// (a bad move is one skipped bead, not a lost gate).
	NoOverflow bool
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
	ag.NoOverflow = yamlGetLines(front, "overflow") == "false"
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
// runtime has no surface — the parity check has already ruled on that).
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
	tmpl := rt.Command
	if rt.Name == ownRuntime && ag.Command != "" {
		tmpl = ag.Command
	}
	out := strings.ReplaceAll(tmpl, "{file}", shellQuote(ag.Path))
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
	out = renderPlaceholder(out, "{settings}", rt.FleetSettingsText())
	out = renderPlaceholder(out, "{skills}", skills)
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

// ─── the Intents table ───────────────────────────────────────────────────────

// IntentDoneWhen returns the `## Intents` row whose intent slug matches one
// of a bead's labels: the slug and its "done when" cell. That cell is the
// one sentence a reviewer checks a closed bead against (ADR 0001), which is
// exactly what a verify bead needs to carry (ADR 0006 §2).
//
// Best effort by design: the table is prose in a markdown file, so no match
// (and a table that isn't a table) is an absent line, never an error.
func (ag *AgentFile) IntentDoneWhen(labels []string) (intent, doneWhen string) {
	for _, row := range markdownRows(BodySection(ag.Body, "## Intents")) {
		if len(row) < 3 {
			continue
		}
		slug := strings.ToLower(strings.TrimSpace(row[0]))
		if slug == "intent" || strings.Trim(slug, "-: ") == "" {
			continue // header, or the |---|---| separator
		}
		for _, l := range labels {
			if intentMatchesLabel(slug, l) {
				return slug, strings.TrimSpace(row[2])
			}
		}
	}
	return "", ""
}

// markdownRows splits a GitHub-style table into trimmed cell rows. Header
// and separator rows come back too; callers skip them by content.
func markdownRows(section string) [][]string {
	var rows [][]string
	for _, ln := range strings.Split(section, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(ln, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

// intentMatchesLabel: a bead label matches an intent slug when it is one of
// the slug's words, give or take a plural — `bug` matches `fix-bugs`,
// `feature` matches `build-features`, `design` matches `implement-designs`.
// Deliberately loose: the payoff is one line of context in a description,
// and there is no label→intent vocabulary in bd to be exact against.
func intentMatchesLabel(slug, label string) bool {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return false
	}
	if slug == label {
		return true
	}
	for _, w := range strings.Split(slug, "-") {
		if w == label || w == label+"s" || label == w+"s" {
			return true
		}
	}
	return false
}
