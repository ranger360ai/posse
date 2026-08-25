package rhq

// Enforcement parity (ADR 0002 §4–5): at launch, for the chosen
// (runtime × cage), which PID gates does at least one wall layer realize?
// L0 (a runtime's own flags) never counts — except codex's -s read-only,
// which is OS-enforced. If anything is unrealized the launch refuses with
// the list, unless the operator passes --allow-degraded; then it launches,
// prints the list, and the session is marked degraded in meta and cockpit.
// Dispatch never allows degradation on its own.
//
// ADR 0003 §3 adds the tier to the same check: a cheaper model follows the
// PID's prose less reliably, so tier `fast` runs only where the wall
// realizes every gate — --allow-degraded is never accepted there — and a
// PID may pin a `tier_floor:` below which it refuses to run at all.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cage tiers, cheapest first. Demanding a tier this host cannot provide is
// degrading, like any other gate the launch cannot realize.
const (
	CageShims     = "shims"
	CageSeatbelt  = "seatbelt"
	CageContainer = "container"
	DefaultCage   = CageShims
)

var Cages = []string{CageShims, CageSeatbelt, CageContainer}

// AvailableCages is what this build can actually provide *host-wide*:
// shims always, seatbelt when sandbox-exec exists (seatbelt.go's init).
// container is deliberately absent — its engine is configurable per
// RHQ_HOME, so availability is a question for an App, not for the package:
// ask a.cageAvailable / a.ContainerAvailable (cage.go).
var AvailableCages = map[string]bool{CageShims: true}

func ValidCage(c string) bool {
	for _, x := range Cages {
		if x == c {
			return true
		}
	}
	return false
}

func cageRank(c string) int {
	for i, x := range Cages {
		if x == c {
			return i
		}
	}
	return -1
}

// Parity is the realization matrix for one launch.
type Parity struct {
	Runtime    string
	Cage       string            // the tier the session actually gets
	Tier       string            // the model tier the launch resolved to (ADR 0003)
	NoDegrade  bool              // the degradation below cannot be waived: --allow-degraded is not accepted at fast
	Realized   map[string]string // gate → layer(s) that realize it
	Unrealized []string          // gate → reason
	Degraded   []string          // everything that makes the launch degrading (unrealized gates, cage shortfall)
}

// CheckParity computes the directory-independent matrix for a PID on a
// runtime at a cage tier, running at a model tier. It cannot count L3: hook
// ownership and behavior are facts about a concrete repo, added only by
// CheckParityIn after executing the slots.
func (a *App) CheckParity(ag *AgentFile, rt *Runtime, cage, tier string) Parity {
	p := Parity{Runtime: rt.Name, Cage: cage, Tier: tier, Realized: map[string]string{}}
	if cage == "" {
		cage = DefaultCage
		p.Cage = cage
	}
	if tier == "" {
		tier = DefaultTier
		p.Tier = tier
	}
	// The PID's minimum tier.
	if ag.Cage != "" && cageRank(ag.Cage) > cageRank(cage) {
		p.Degraded = append(p.Degraded, fmt.Sprintf("cage: PID demands %s, launching at %s", ag.Cage, cage))
	}
	// The PID's model floor (ADR 0003 §3): a guardrail that lives only in
	// the persona's prose ("no commitments", "findings only") needs a model
	// that follows prose. Below the floor the launch refuses in the same
	// shape as an unrealized gate — it is the same kind of statement: this
	// PID asked for something this launch does not give it.
	if BelowFloor(ag, tier) {
		p.Degraded = append(p.Degraded, fmt.Sprintf("tier_floor: PID demands %s or better, launching at %s", ag.TierFloor, tier))
	}
	if !a.cageAvailable(cage) {
		p.Degraded = append(p.Degraded, fmt.Sprintf("cage %s is not available on this host/build", cage))
	}
	// A runtime that sandboxes its own children cannot sit inside our
	// seatbelt: macOS refuses nested sandbox_apply. Its own sandbox is the
	// file gate there (counted below via Enforced); the seatbelt is not.
	seatbeltIncompatible := cage == CageSeatbelt && rt.SelfSandbox
	if seatbeltIncompatible {
		p.Degraded = append(p.Degraded, fmt.Sprintf("cage seatbelt cannot wrap %s: its own child sandbox does not nest (sandbox_apply: Operation not permitted) — use cage: shims; %s's sandbox is OS-enforced there", rt.Name, rt.Name))
	}
	// What the runtime enforces at OS level (codex read-only).
	enforced := map[string]bool{}
	if rt.Realize != nil {
		for _, r := range rt.Realize(ag.Allow, ag.Deny, ag.MemoryDir).Enforced {
			enforced[r] = true
		}
	}
	shims := ParseShimRules(ag.Deny)
	// The seatbelt is applied at its own tier and nowhere else: ADR 0002 §3
	// — sandbox-exec around the engine cages the *client*, not the
	// container, so L2 does not stack under L4; the mount boundary is what
	// replaces it there.
	seatbelt := a.cageAvailable(cage) && cage == CageSeatbelt && !seatbeltIncompatible
	// container: the session really runs inside the cage. An unavailable
	// container tier degrades to a host launch (CreateSession skips the
	// wrap), and on the host the L1/L3 claims below are true again.
	container := a.cageAvailable(cage) && cage == CageContainer
	// The egress route is the engine's to express, not just this build's:
	// docker (and anything that answers to its CLI) spells it, an engine
	// yaml that leaves net_create:/proxy_up: unsaid does not, and a gate
	// nobody can render is unrealized however strong the tier is.
	egress := container && ContainerEgress && a.EngineEgress()
	// The inner wall (rangerhq-6so). Cumulative-in-realization is a promise
	// about what the tier *renders*, and the render happens in the image —
	// so the image is what gets asked. An image with no Linux posse cannot run
	// `posse gates wrap`, and a container tier that cannot is a container tier
	// with no shims and no gate shell: unrealized, refused, exactly like any
	// other gate the launch cannot hold.
	inner := container && ContainerInnerGates && a.CageInnerGates()
	for _, rule := range ag.Deny {
		switch {
		case strings.HasPrefix(rule, "Bash("):
			// L1 and L3 do not follow a process into a container by
			// themselves: a shim execs the real binary resolved at render
			// time on the *host* (/opt/homebrew/bin/git is not in a Linux
			// image) and the gate shell points at the host's zsh. They are
			// rendered INSIDE instead (cageinner.go) — by a posse that has to
			// be in the image. When it is not, a shell-verb deny is
			// unrealized at this tier and the launch refuses like any other.
			if container && !inner {
				p.unrealized(rule, "cage container renders L1/L3 inside the cage, and image "+a.CageImage()+" cannot: it answers no to `posse gates wrap --probe`, so it carries no Linux posse (run `posse cage build`). The host's shims exec host paths and its gate shell is the host's zsh, so neither crosses the boundary")
				continue
			}
			// Without the gate shell (ADR 0009 §2), a runtime that re-execs a
			// login shell per command puts the L1 shim behind /usr/bin
			// whatever the launcher prepends, and nothing about the rule's
			// shape can save it. L3 may recover git rules, but only after the
			// directory-aware check executes a concrete repo's hooks.
			if rt.NoGateShell {
				p.unrealized(rule, "L1 shim cannot hold on "+rt.Name+" (gate_shell: false): a runtime that re-execs a login shell lets path_helper demote the gates dir below /usr/bin; L3 counts only after CheckParityIn behavior-probes the hook")
				continue
			}
			cmd := shimCommand(rule)
			if cmd == "" || shims[cmd] == nil {
				p.unrealized(rule, "not a plain command — no shim can be rendered")
				continue
			}
			// How the shim actually matches decides what we may claim: a
			// subcommand deny on a command whose global options we do not
			// know is best-effort, not a wall (rangerhq-2zm).
			r := ParseShimRules([]string{rule})[cmd][0]
			kind, faithful := matcherFor(cmd, r)
			if !faithful {
				p.unrealized(rule, fmt.Sprintf("L1 shim has no global-option table for %s, so an option taking a separate value before %q hides it (deny the whole verb: Bash(%s)) — best-effort only", cmd, r.Words[0], cmd))
				continue
			}
			layers := "L1 shim (" + kind + ")"
			if inner {
				layers = "L1 shim (" + kind + ") rendered inside the cage"
			}
			p.Realized[rule] = layers
		case rule == "Edit" || rule == "Write" || rule == "NotebookEdit":
			switch {
			case inner:
				p.Realized[rule] = "L4 mount boundary (repo mounted :ro)"
			case seatbelt:
				p.Realized[rule] = "L2 seatbelt"
			case enforced[rule]:
				p.Realized[rule] = rt.Name + " sandbox (OS-enforced)"
			case container:
				p.unrealized(rule, "cage container mounts the repo :ro for this deny, but image "+a.CageImage()+" is not one this posse built (`posse cage build`) — and L2 does not stack under this tier: sandbox-exec around the engine cages the client, not the container")
			default:
				p.unrealized(rule, "needs cage: seatbelt (or codex -s read-only) — native flags are politeness")
			}
		default: // WebFetch, WebSearch, mcp__*, other tool names
			// ADR 0002 §4 gives WebFetch/WebSearch their own row and every
			// other tool-name deny another, and the difference is real: a
			// fetch leaves the container over TCP and meets the proxy, while
			// a stdio MCP server never leaves the container at all and an
			// egress allowlist has nothing to say about it. Claiming the
			// second on the strength of the first would be the tier
			// promising a gate it does not hold.
			web := rule == "WebFetch" || rule == "WebSearch"
			switch {
			case egress && web:
				p.Realized[rule] = "L4 container, as far as egress: goes (the proxy stops unknown hosts, not a fetch through an allowed one)"
			case container && web:
				p.unrealized(rule, "cage container realizes this only as far as egress: goes, and engine "+a.ResolveEngine()+" spells no route (net_create:/proxy_up:)")
			case container:
				p.unrealized(rule, "cage container realizes a tool-name deny only as far as egress: goes, and this one need never leave the container (a stdio MCP server is a child process) — runtime-native only")
			default:
				p.unrealized(rule, "runtime-native only below cage: container")
			}
		}
	}
	if len(ag.Egress) > 0 {
		gate := "egress: " + strings.Join(ag.Egress, ",")
		switch {
		case egress:
			p.Realized[gate] = "L4 --internal network + CONNECT proxy (the route, not the env var)"
			// rangerhq-rm5's instruction, and the one honest limit of this
			// gate: the proxy sees the CONNECT authority and nothing else,
			// so a PID that denies fetching AND names hosts is asking for
			// something the tier gives only in part. Degraded, not
			// Unrealized — both gates are enforced as far as they go; this
			// says what the launch does not buy, which is the operator's
			// call to accept.
			if deniesWebReach(ag.Deny) {
				p.Degraded = append(p.Degraded, "egress: + WebFetch/WebSearch deny — the proxy stops unknown hosts, not a fetch through an allowed one, so the fetch gate holds only to the edge of the allowlist")
			}
		case container:
			p.unrealized(gate, "engine "+a.ResolveEngine()+" spells no --internal network and no proxy (net_create:/proxy_up: in its cages/*.yaml), so nothing renders the route")
		default:
			p.unrealized(gate, "container tier only")
		}
	}
	// Skills (ADR 0007 §3): `skills:` on a PID says the persona's work
	// depends on them, so it is checked like a gate — a runtime with no
	// per-session skill surface degrades the launch. Not a *wall* claim
	// though: it goes to Degraded, never to Unrealized, because nothing here
	// is being enforced. A skill the persona would merely like belongs in
	// the runtime's global config, which this never touches.
	if len(ag.Skills) > 0 {
		names := strings.Join(ag.Skills, ", ")
		switch flag, ok := rt.SkillsText(ag.SkillsStateDir, ag.Skills); {
		case !ok:
			p.Degraded = append(p.Degraded, fmt.Sprintf("skills: %s — %s has no per-session skill surface", names, rt.Name))
		case rt.SkillsCwd:
			// No flag to show: the binding is the symlinks the launch writes
			// into the session dir, which is as session-scoped as the cwd is.
			p.Realized["skills: "+names] = rt.Name + " reads " + AgentsSkillsPath + " in the session dir (symlinked at launch, additive)"
		default:
			p.Realized["skills: "+names] = rt.Name + " " + flag + " (session-only, additive)"
		}
	}
	// ADR 0003 §3, the security review's caveat made a rule: at fast the model behind
	// the wall is the least reliable reader of the PID's prose, so the wall
	// is all that is left — anything it does not realize refuses the launch
	// and no flag waives it. Dial D: fast is reached only by an explicit
	// label or tier_by_label signal, and only with full parity.
	if tier == TierFast && len(p.Degraded) > 0 {
		p.NoDegrade = true
	}
	return p
}

// ProjectConfigTrust is the one part of the matrix that depends on *where*
// the session starts: a runtime whose config posse made the session dir able
// to supply (Runtime.ProjectConfig — codex's directory trust, which the
// fleet types because an unattended session otherwise hangs on the trust
// dialog) reads that file before any model turn. Verified on codex-cli
// 0.147.0 (rangerhq-pmz, reproduced in rangerhq-b7m): the keys posse types
// on the line win over the project's, but the keys it does not type are
// the repo's — [mcp_servers.*] and notify are spawned by the runtime
// itself, outside its own per-command sandbox, with the whole session env.
// So the channel is a file in the repo getting exec on the box with no
// model and no PID in front of it, which is worth a refusal the operator
// has to answer rather than a line nobody reads.
//
// Returns "" when there is nothing to say: no such runtime surface, no
// such file, or the PID opted in with trust_project_config: true. It is a
// Degraded entry and never an Unrealized one — like the cage shortfall, it
// says what this launch gives away, not which gate went unenforced.
func ProjectConfigTrust(rt *Runtime, ag *AgentFile, dir string) string {
	if rt == nil || rt.ProjectConfig == "" || dir == "" {
		return ""
	}
	if ag != nil && ag.TrustProjectConfig {
		return ""
	}
	p := filepath.Join(dir, rt.ProjectConfig)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return fmt.Sprintf("trust: %s reads %s from the session dir before any turn — its mcp_servers/notify entries run outside %s's sandbox with the whole session env (the keys posse types, -s/-a/developer_instructions, still win); opt in with trust_project_config: true on the PID, or remove the file",
		rt.Name, AbbrevHome(p), rt.Name)
}

// CheckParityIn is CheckParity plus what depends on the directory the
// session starts in. CreateSession and `posse gates` use it; CheckParity
// stays the dir-independent matrix (a PID × runtime × cage × tier
// statement) so nothing that only describes a persona has to invent a cwd.
func (a *App) CheckParityIn(ag *AgentFile, rt *Runtime, cage, tier, dir string) Parity {
	p := a.CheckParity(ag, rt, cage, tier)
	applyL3Probe(&p, ag, rt, dir)
	if why := ProjectConfigTrust(rt, ag, dir); why != "" {
		p.Degraded = append(p.Degraded, why)
	}
	// Same rule as the dir-independent gates: at fast the operator's consent
	// is not on offer. Recompute because a successful L3 behavior probe can
	// also replace CheckParity's conservative NoGateShell verdict.
	p.NoDegrade = p.Tier == TierFast && len(p.Degraded) > 0
	return p
}

// applyL3Probe adds a fact about this launch directory to CheckParity's
// directory-independent matrix. A marker is intentionally never
// consulted: legitimate chain dispatchers have none, and marker-bearing
// scripts are writable files. The behavioral check is the evidence.
func applyL3Probe(p *Parity, ag *AgentFile, rt *Runtime, dir string) {
	wantPrePush := deniesGitPush(ag.Deny)
	probe := probeL3Hooks(dir, wantPrePush)
	if !probe.Repo {
		return
	}
	where := AbbrevHome(probe.HooksDir)
	for _, rule := range ag.Deny {
		switch {
		case deniesGitPush([]string{rule}):
			applyHookResult(p, rule, "L3 pre-push hook (behavior probed)", probe.PrePush,
				"L1 shim cannot hold on "+rt.Name+" (gate_shell: false)")
		case deniesUnqualifiedCommit([]string{rule}):
			applyHookResult(p, rule, "L3 prepare-commit-msg hook (behavior probed)", probe.CommitGuard,
				"L1 shim cannot hold on "+rt.Name+" (gate_shell: false)")
		}
	}
	if wantPrePush && !probe.PrePush {
		p.Degraded = append(p.Degraded, "L3 pre-push hook — behavior probe in "+where+" did not refuse git push with exit 1; this layer is not realized")
	}
	if !probe.CommitGuard {
		p.Degraded = append(p.Degraded, "L3 prepare-commit-msg hook — behavior probe in "+where+" did not refuse an unqualified commit with exit 1; the shared-index and beads visibility guards are not realized")
	}
}

func applyHookResult(p *Parity, gate, observed string, works bool, noL1 string) {
	if !works {
		return
	}
	if layer := p.Realized[gate]; layer != "" {
		p.Realized[gate] = layer + " + " + observed
		return
	}
	clearGateDegradation(p, gate)
	p.Realized[gate] = observed + " — " + noL1
}

func clearGateDegradation(p *Parity, gate string) {
	prefix := gate + " — "
	keep := func(lines []string) []string {
		out := lines[:0]
		for _, line := range lines {
			if !strings.HasPrefix(line, prefix) {
				out = append(out, line)
			}
		}
		return out
	}
	p.Unrealized = keep(p.Unrealized)
	p.Degraded = keep(p.Degraded)
}

// tierRank orders Tiers dearest-first (strong 0 … fast 2), so a *higher*
// rank is a cheaper model. -1 for anything not a tier.
func tierRank(t string) int {
	for i, x := range Tiers {
		if x == t {
			return i
		}
	}
	return -1
}

// BelowFloor: the resolved tier is cheaper than the PID's tier_floor:.
// A floor that is not a tier name is a PID error (posse agent check, and the
// launch refuses outright) — here it reads as "nothing is good enough",
// which is the safe way for a typo to fail.
func BelowFloor(ag *AgentFile, tier string) bool {
	if ag == nil || ag.TierFloor == "" {
		return false
	}
	return tierRank(tier) > tierRank(ag.TierFloor)
}

// CheckTier is the pair of ADR 0003 §3 rules that hold for every prompt,
// not just for a fresh launch: a tier below the PID's floor, and fast on a
// (runtime × cage) that leaves any gate to politeness. Dispatch calls it
// per bead — the tier comes from the bead's labels, so the same live
// persona takes a standard bead and refuses a fast one. Gate degradation
// that the tier did not cause stays the launch's business (CreateSession),
// which is where the operator's --allow-degraded is read.
func (a *App) CheckTier(ag *AgentFile, rt *Runtime, cage, tier string, allowDegraded bool) error {
	if ag == nil || rt == nil {
		return nil
	}
	p := a.CheckParity(ag, rt, cage, tier)
	if p.NoDegrade || (BelowFloor(ag, tier) && !allowDegraded) {
		return degradedError{p}
	}
	return nil
}

func (p *Parity) unrealized(gate, why string) {
	p.Unrealized = append(p.Unrealized, gate+" — "+why)
	p.Degraded = append(p.Degraded, gate+" — "+why)
}

// deniesWebReach: does this deny list carry a gate on reaching out that the
// egress allowlist can only partly hold?
func deniesWebReach(deny []string) bool {
	for _, r := range deny {
		if r == "WebFetch" || r == "WebSearch" {
			return true
		}
	}
	return false
}

// shimCommand extracts the command name of a Bash(...) rule ("" if none).
func shimCommand(rule string) string {
	for cmd := range ParseShimRules([]string{rule}) {
		return cmd
	}
	return ""
}

// degradedError is what a launch returns when parity fails and degradation
// was not allowed.
type degradedError struct{ p Parity }

func (e degradedError) Error() string {
	tail := "  (launch anyway with --allow-degraded; the session will be marked degraded)"
	if e.p.NoDegrade {
		tail = "  (tier fast runs only where the wall realizes every gate — --allow-degraded is never accepted there, ADR 0003 §3; raise the cage, or the tier)"
	}
	return fmt.Sprintf("refused: %s at cage %s, tier %s does not realize every gate —\n  %s\n%s",
		e.p.Runtime, e.p.Cage, e.p.Tier, strings.Join(e.p.Degraded, "\n  "), tail)
}

// String renders the matrix for posse gates.
func (p Parity) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s @ %s/%s:", p.Runtime, p.Cage, p.Tier)
	if len(p.Degraded) == 0 {
		b.WriteString(" all gates realized\n")
	} else {
		b.WriteString(" DEGRADED\n")
	}
	for gate, layer := range p.Realized {
		fmt.Fprintf(&b, "    ✓ %-28s %s\n", gate, layer)
	}
	for _, u := range p.Degraded {
		fmt.Fprintf(&b, "    ✗ %s\n", u)
	}
	return b.String()
}

// ResolveCage: explicit (--cage / dispatch) > PID cage: > shims.
func ResolveCage(explicit string, ag *AgentFile) string {
	if explicit != "" {
		return explicit
	}
	if ag != nil && ag.Cage != "" {
		return ag.Cage
	}
	return DefaultCage
}
