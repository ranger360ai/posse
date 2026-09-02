package posse

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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// EnforcementClass names which side of ADR 0025 §1 a realized gate sits on:
// held outside the gated process, surviving an adversarial one (Enforced —
// L2 seatbelt, L4 mount boundary, egress network+proxy, codex `-s
// read-only`), or held only by the process's own ordinary path, defeated by
// an emptied environment, `--no-verify`, `core.hooksPath`, or editing the
// slot (Cooperative — L1 shims, L3 hooks, the gate shell). "" is not a third
// class: it marks a Realized row that is not itself an adversarial gate
// claim (`skills:`, the record-reach probe's abstain/no-target/unmeasured
// rows) — nothing to class, nothing printed.
type EnforcementClass string

const (
	Enforced    EnforcementClass = "enforced"
	Cooperative EnforcementClass = "cooperative"
)

// RealizedGate is one row of the parity matrix's realized half. Class is
// typed so a printer (`posse gates`, a launch refusal, session meta) reads
// it off a field instead of parsing "L1 shim" / "L2 seatbelt" out of
// Detail's prose — ADR 0025 §1's whole point is that "realized" stopped
// being one word a printer could trust. Effect is ADR 0025 §3's push-effect
// note: set only for `Bash(git push:*)` at `cage: container`, and always a
// printed note, never a computed claim — the launcher does not know the
// remote's host.
type RealizedGate struct {
	Class  EnforcementClass
	Detail string
	Effect string
}

// String renders one realized row's tail the way `posse gates` prints it:
// "→ <class> (<detail>)", or bare Detail for a "" class (a row that is not
// itself an adversarial gate claim). The Effect note, when set, trails
// after an em dash.
func (g RealizedGate) String() string {
	s := g.Detail
	if g.Class != "" {
		s = "→ " + string(g.Class)
		if g.Detail != "" {
			s += " (" + g.Detail + ")"
		}
	}
	if g.Effect != "" {
		s += " — " + g.Effect
	}
	return s
}

// Parity is the realization matrix for one launch.
type Parity struct {
	Runtime    string
	Cage       string                  // the tier the session actually gets
	Tier       string                  // the model tier the launch resolved to (ADR 0003)
	NoDegrade  bool                    // the degradation below cannot be waived: --allow-degraded is not accepted at fast
	Realized   map[string]RealizedGate // gate → what realizes it, classed (ADR 0025 §1)
	Unrealized []string                // gate → reason
	Degraded   []string                // everything that makes the launch degrading (unrealized gates, cage shortfall)
	// DeclaredDifference (ADR 0017 §2): safe-by-design facts about the
	// runtime — a mechanism differs, not a wall. codex's own sandbox cannot
	// nest under our seatbelt; a SelfSandbox runtime that already realizes
	// the demanded wall by its own means, at a cage rank below what the PID
	// asked for. Never counted toward Degraded/NoDegrade: a declared
	// difference must never gate a launch or force --allow-degraded, or it
	// renders exactly like the missing wall it is not (ranger-base-d17a).
	DeclaredDifference []string
}

// CheckParity computes the directory-independent matrix for a PID on a
// runtime at a cage tier, running at a model tier. It cannot count L3: hook
// ownership and behavior are facts about a concrete repo, added only by
// CheckParityIn after executing the slots.
func (a *App) CheckParity(ag *AgentFile, rt *Runtime, cage, tier string) Parity {
	p := Parity{Runtime: rt.Name, Cage: cage, Tier: tier, Realized: map[string]RealizedGate{}}
	if cage == "" {
		cage = DefaultCage
		p.Cage = cage
	}
	if tier == "" {
		tier = DefaultTier
		p.Tier = tier
	}
	// What the runtime enforces at OS level (codex read-only). Computed
	// before the cage-rank check below, which needs it to tell a real cage
	// shortfall from a SelfSandbox runtime that already delivers the
	// demanded wall by its own means.
	enforced := map[string]bool{}
	if rt.Realize != nil {
		for _, r := range rt.Realize(ag.Allow, ag.Deny, ag.MemoryDir).Enforced {
			enforced[r] = true
		}
	}
	// The PID's minimum tier. A SelfSandbox runtime whose own sandbox
	// already enforces Edit/Write is not shorted the wall the PID's `cage:
	// seatbelt` was asking for — codex's -s read-only IS that wall, at
	// shims — so this reads as a DECLARED DIFFERENCE (ADR 0017 §2), never
	// as degradation: a first-class runtime must not need --allow-degraded
	// to launch at its own correct cage (ranger-base-d17a).
	if ag.Cage != "" && cageRank(ag.Cage) > cageRank(cage) {
		msg := fmt.Sprintf("cage: PID demands %s, launching at %s", ag.Cage, cage)
		if ag.Cage == CageSeatbelt && rt.SelfSandbox && enforced["Edit"] && enforced["Write"] {
			p.DeclaredDifference = append(p.DeclaredDifference, msg+" — "+rt.Name+"'s own sandbox already realizes the write wall (OS-enforced Edit/Write) here, an equivalent posture to seatbelt")
		} else {
			p.Degraded = append(p.Degraded, msg)
		}
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
	// This is the canonical DECLARED DIFFERENCE (ADR 0017 §2): the
	// mechanism differs, and whether any gate actually goes unrealized is
	// judged separately, per rule, below — never here.
	seatbeltIncompatible := cage == CageSeatbelt && rt.SelfSandbox
	if seatbeltIncompatible {
		p.DeclaredDifference = append(p.DeclaredDifference, fmt.Sprintf("cage seatbelt cannot wrap %s: its own child sandbox does not nest (sandbox_apply: Operation not permitted) — use cage: shims; %s's sandbox is OS-enforced there", rt.Name, rt.Name))
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
	// ADR 0032 §1 rule 1 — assumed-until-probed. Read at most once, and only
	// if a shell-verb deny gets that far: it is a fact about the RUNTIME,
	// not about a rule, so a PID with nine Bash denies must not read the
	// record nine times, and a PID with none must not read it at all.
	assumed, assumedRead := "", false
	assumedWhy := func() string {
		if !assumedRead {
			assumed, assumedRead = a.assumedUntilProbed(rt), true
		}
		return assumed
	}
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
			// ADR 0032 §1 rule 1. On a template-only runtime the L1 claim
			// below rests on three behaviours nobody measured for this CLI
			// (runtimeprobe.go names them); the silent one is a runtime that
			// re-execs a login shell it did not take from $SHELL, which is
			// the pre-ADR-0009 grok day. Degraded, not a flat refusal: the
			// waiver is on offer and the probe is the way out, and treating
			// every novel engine as permanently unrealized is what trains an
			// operator to type --allow-degraded out of habit.
			if why := assumedWhy(); why != "" {
				p.unrealized(rule, why)
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
				p.unrealized(rule, matcherWhy(cmd, r))
				continue
			}
			layers := "L1 shim (" + kind + ")"
			if inner {
				layers = "L1 shim (" + kind + ") rendered inside the cage"
			}
			p.Realized[rule] = RealizedGate{Class: Cooperative, Detail: layers}
		case rule == "Edit" || rule == "Write" || rule == "NotebookEdit":
			a.wholeTreeWriteWall(&p, rule, rule, rt, inner, seatbelt, container, enforced)
		case isPathScopedWrite(rule):
			// ADR 0014 §1: a parametrized file-write rule is a subtree deny,
			// not a tool name. Before this arm it fell to the default below
			// and `Edit(docs/adr/**)` was classified as an MCP server.
			d, _ := parsePathScopedWrite(rule)
			switch {
			case d.Bare:
				// `Edit(**)` / `Edit(*)` / `Edit(.)`: the whole tree, so the
				// bare rule's row verbatim. wholeTreeWriteDeny makes the
				// renderers read it the same way, so this claims nothing the
				// seatbelt and the mount do not do.
				a.wholeTreeWriteWall(&p, rule, d.Tool, rt, inner, seatbelt, container, enforced)
			case !d.Subtree:
				// Unrealized by construction, at every tier — the ADR's own
				// words, because the operator's next question is "then what
				// do I write instead".
				p.unrealized(rule, "not a directory-prefix glob; the wall realizes subtrees (Edit(docs/adr/**)), not file filters")
			case inner:
				p.Realized[rule] = RealizedGate{Class: Enforced, Detail: "L4 :ro overlay (" + d.Path + ")"}
			case seatbelt:
				p.Realized[rule] = RealizedGate{Class: Enforced, Detail: "L2 trailing deny (subpath " + d.Path + ")"}
			case enforced[d.Tool]:
				// Only reachable when the PID ALSO denies the whole tree
				// bare, which is what turns codex's -s read-only on: the
				// subtree is inside a tree that is already OS-unwritable, so
				// the gate holds. `posse agent check` calls the pair
				// redundant; refusing the launch over it would be a lie in
				// the other direction. `-s read-only` alone never lands
				// here — ADR 0014 §2's row is that it has no per-path
				// surface, and over-enforcement is not realization.
				p.Realized[rule] = RealizedGate{Class: Enforced, Detail: rt.Name + " sandbox (OS-enforced): the whole tree this subtree is in, bought by the PID's bare Edit/Write"}
			case container:
				p.unrealized(rule, "cage container overlays this subtree :ro, but image "+a.CageImage()+" is not one this posse built (`posse cage build`) — and L2 does not stack under this tier: sandbox-exec around the engine cages the client, not the container")
			default:
				p.unrealized(rule, "needs cage: seatbelt (or container) — a path-scoped write is not a tool-name deny")
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
				p.Realized[rule] = RealizedGate{Class: Enforced, Detail: "L4 container, as far as egress: goes (the proxy stops unknown hosts, not a fetch through an allowed one)"}
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
			p.Realized[gate] = RealizedGate{Class: Enforced, Detail: "L4 --internal network + CONNECT proxy (the route, not the env var)"}
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
			// Class "": a skills binding is not an adversarial gate claim.
			p.Realized["skills: "+names] = RealizedGate{Detail: rt.Name + " reads " + AgentsSkillsPath + " in the session dir (symlinked at launch, additive)"}
		default:
			p.Realized["skills: "+names] = RealizedGate{Detail: rt.Name + " " + flag + " (session-only, additive)"}
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
// the session starts: a trusted runtime may read project-owned executable
// configuration before any model turn. Runtime.ProjectConfigKeys narrows
// JSON settings to the top-level keys made live by the trust grant; an empty
// list preserves the original whole-file predicate. A keyed file fails closed
// when it exists but cannot be proved to be a readable top-level JSON object.
//
// Returns "" when there is nothing to say: no such runtime surface, no
// such file, or the PID opted in with trust_project_config: true. It is a
// Degraded entry and never an Unrealized one — like the cage shortfall, it
// says what this launch gives away, not which gate went unenforced.
func ProjectConfigTrust(rt *Runtime, ag *AgentFile, dir string) string {
	if rt == nil || len(rt.ProjectConfig) == 0 || dir == "" {
		return ""
	}
	if ag != nil && ag.TrustProjectConfig {
		return ""
	}
	// Every file in the runtime's project scope, in declared order; the first
	// one with something to say is the message. One line names one file
	// because one file is enough to refuse the launch, and the operator's next
	// move (remove it, or opt the PID in) is the same either way.
	for _, rel := range rt.ProjectConfig {
		if why := projectConfigTrustFile(rt, filepath.Join(dir, rel)); why != "" {
			return why
		}
	}
	return ""
}

// projectConfigTrustFile classifies one file of the runtime's project scope.
func projectConfigTrustFile(rt *Runtime, p string) string {
	if len(rt.ProjectConfigKeys) == 0 {
		if _, err := os.Stat(p); err != nil {
			return ""
		}
		return projectConfigTrustMessage(rt, p, "project config is present")
	}

	// Lstat distinguishes a missing config from an existing path whose target
	// cannot be read (including a dangling symlink). Only the former is clean.
	if _, err := os.Lstat(p); err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		return projectConfigTrustMessage(rt, p, "project config classification failed: unreadable: "+err.Error())
	}
	// An existing path that is not a regular file can never be proved to be a
	// readable top-level JSON object, and asking by reading is not free:
	// open(2) on a FIFO with no writer never returns, so the launch used to
	// block here forever instead of degrading (ranger-base-92rt, folded into
	// ranger-base-92n5p). ADR 0002 amendment 2026-08-26 §4 wants an existing
	// file the launch cannot prove safe to DEGRADE, so this is the unreadable
	// classification the directory arm already gives, reached without the
	// open. os.Stat follows symlinks: a link to a regular file is read as
	// before, and a dangling one fails the Stat and falls through to the
	// ReadFile below, which names it unreadable as it always has.
	if fi, err := os.Stat(p); err == nil && !fi.Mode().IsRegular() {
		return projectConfigTrustMessage(rt, p, "project config classification failed: unreadable: "+fileTypeName(fi.Mode())+", not a regular file")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return projectConfigTrustMessage(rt, p, "project config classification failed: unreadable: "+err.Error())
	}
	// RawMessage values deliberately avoid interpreting either runtime's
	// schema. Only JSON validity, the top-level shape, and key presence are
	// ours to classify.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		var raw json.RawMessage
		if syntaxErr := json.Unmarshal(b, &raw); syntaxErr != nil {
			return projectConfigTrustMessage(rt, p, "project config classification failed: invalid JSON: "+syntaxErr.Error())
		}
		return projectConfigTrustMessage(rt, p, "project config classification failed: not a top-level JSON object")
	}
	if obj == nil { // JSON null unmarshals to a nil map without an error.
		return projectConfigTrustMessage(rt, p, "project config classification failed: not a top-level JSON object")
	}
	var matched []string
	for _, key := range rt.ProjectConfigKeys {
		if _, ok := obj[key]; ok {
			matched = append(matched, key)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	return projectConfigTrustMessage(rt, p, "matched top-level project config keys: "+strings.Join(matched, ", "))
}

func projectConfigTrustMessage(rt *Runtime, path, finding string) string {
	return fmt.Sprintf("trust: %s reads %s from the session dir before any turn — %s; project-owned executable configuration can run on the box with the whole session env before any model turn or PID/tool gate can mediate it; opt in with trust_project_config: true on the PID, or remove the file (or, for keyed JSON, the matching keys)",
		rt.Name, AbbrevHome(path), finding)
}

// CheckParityIn is CheckParity plus what depends on the directory the
// session starts in. CreateSession and `posse gates` use it; CheckParity
// stays the dir-independent matrix (a PID × runtime × cage × tier
// statement) so nothing that only describes a persona has to invent a cwd.
func (a *App) CheckParityIn(ag *AgentFile, rt *Runtime, cage, tier, dir string) Parity {
	p := a.CheckParity(ag, rt, cage, tier)
	a.applyL3Probe(&p, ag, rt, dir)
	applyPushEffectNote(&p, ag.Deny)
	// ADR 0013 §4: the cage half of the record stage. Directory-aware for
	// the same reason L3 is — which .beads bd opens is a fact about a
	// concrete launch dir, not about a persona — and the dir is already in
	// hand here (reachability.go, ranger-base-hxhb).
	a.applyRecordReach(&p, ag, rt, dir)
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
// directory-independent matrix. ADR 0023 amends ADR 0002 §3's doctrine: a
// marker is still never trusted to decide whether install may overwrite a
// hook — that question stays behavioral, unrelated to L3 realization — but
// full-byte identity of the dispatched file IS the L3 evidence now, paired
// with behavior of our own render (probeL3Hooks). Identity is not a marker:
// it is the whole file, checked against the whole file we would have
// written.
func (a *App) applyL3Probe(p *Parity, ag *AgentFile, rt *Runtime, dir string) {
	wantPrePush := deniesGitPush(ag.Deny)
	probe := a.probeL3Hooks(dir, wantPrePush)
	if !probe.Repo {
		return
	}
	// Why L1 is not carrying these two gates on its own, for the line a
	// git-hook realization prints when it is the ONLY layer. There are two
	// reasons now and they send the reader to different places: a runtime
	// that declared `gate_shell: false` (nothing to fix — that is the exit
	// hatch), and a template-only runtime whose shim claim is assumed until
	// somebody probes it (ADR 0032 §1). Printing the first sentence for the
	// second case would name a key the yaml does not set.
	noL1 := "L1 shim cannot hold on " + rt.Name + " (gate_shell: false)"
	if !rt.NoGateShell && a.assumedUntilProbed(rt) != "" {
		noL1 = "L1 on " + rt.Name + " is assumed, not measured — `posse runtime probe " + rt.Name + "` (ADR 0032 §1)"
	}
	for _, rule := range ag.Deny {
		switch {
		case deniesGitPush([]string{rule}):
			applyHookResult(p, rule, "L3 pre-push hook (render probed, dispatch verified)", probe.PrePush, noL1)
		case deniesUnqualifiedCommit([]string{rule}):
			applyHookResult(p, rule, "L3 prepare-commit-msg hook (render probed, dispatch verified)", probe.CommitGuard, noL1)
		}
	}
	if wantPrePush && !probe.PrePush {
		p.Degraded = append(p.Degraded, probe.PrePushDegraded)
	}
	if !probe.CommitGuard {
		p.Degraded = append(p.Degraded, probe.CommitGuardDegraded)
	}
}

// L3-observed merge (ADR 0025 §1): L3, like L1, is cooperative — held only
// in-process, by the ordinary path the hook happens to run on. Merging into
// an existing L1 row keeps that row's class (already Cooperative); a fresh
// row (L3 alone, e.g. NoGateShell runtimes) is stamped Cooperative here.
func applyHookResult(p *Parity, gate, observed string, works bool, noL1 string) {
	if !works {
		return
	}
	if g, ok := p.Realized[gate]; ok {
		g.Detail = g.Detail + " + " + observed
		p.Realized[gate] = g
		return
	}
	clearGateDegradation(p, gate)
	p.Realized[gate] = RealizedGate{Class: Cooperative, Detail: observed + " — " + noL1}
}

// applyPushEffectNote is ADR 0025 §3: at `cage: container` the verb gate for
// `git push` stays cooperative — L1/L3 do not hold any harder just because
// the process runs in a container — but the push's EFFECT can still die at
// an enforced layer, as far as this launch is configured for it: a path
// remote inside the mounts is stopped by `:ro` (granted when the PID also
// denies Edit/Write), a network remote by the egress proxy unless `egress:`
// names its host. Printed as a note beside the class, never as a computed
// claim — the launcher does not know the remote's host, so it must not
// pretend to.
func applyPushEffectNote(p *Parity, deny []string) {
	if p.Cage != CageContainer {
		return
	}
	for _, rule := range deny {
		if !deniesGitPush([]string{rule}) {
			continue
		}
		g, ok := p.Realized[rule]
		if !ok || g.Effect != "" {
			continue
		}
		g.Effect = "at cage: container the push's EFFECT still dies at an enforced layer as far as this launch is configured — a path remote inside the mounts is stopped by :ro (granted when the PID also denies Edit/Write), a network remote by the egress proxy unless egress: names its host (ADR 0025 §3); the verb gate itself stays cooperative"
		p.Realized[rule] = g
	}
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

// wholeTreeWriteWall is ADR 0002 §4's file-write row: what realizes a deny
// of writing the whole session tree. gate is the rule as the PID wrote it
// (so the matrix prints back what the operator typed) and tool is the
// Edit/Write/NotebookEdit it denies — the two differ only for ADR 0014 §1's
// long spelling, `Edit(**)`, which is this same row and not a scoped one.
func (a *App) wholeTreeWriteWall(p *Parity, gate, tool string, rt *Runtime, inner, seatbelt, container bool, enforced map[string]bool) {
	switch {
	case inner:
		p.Realized[gate] = RealizedGate{Class: Enforced, Detail: "L4 mount boundary (repo mounted :ro)"}
	case seatbelt:
		p.Realized[gate] = RealizedGate{Class: Enforced, Detail: "L2 seatbelt"}
	case enforced[tool]:
		p.Realized[gate] = RealizedGate{Class: Enforced, Detail: rt.Name + " sandbox (OS-enforced)"}
	case container:
		p.unrealized(gate, "cage container mounts the repo :ro for this deny, but image "+a.CageImage()+" is not one this posse built (`posse cage build`) — and L2 does not stack under this tier: sandbox-exec around the engine cages the client, not the container")
	default:
		p.unrealized(gate, "needs cage: seatbelt (or codex -s read-only) — native flags are politeness")
	}
}

// assumedUntilProbed is the ADR 0032 §1 rule 1 reason a `Bash(...)` deny
// does not count on this runtime yet, or "" when it does. Built-ins are
// exempt by measurement, not by privilege: their argv table was probed in
// ADR 0009 (rangerhq-e43), and a built-in has no runtimes/<name>.yaml for a
// third party to author.
//
// A probe record that FAILED reads the same way here as no record at all —
// both leave the claim unmeasured — but the sentences differ, because "we
// looked and it does not hold" and "nobody looked" want different next
// moves from the operator (ProbeState writes both).
func (a *App) assumedUntilProbed(rt *Runtime) string {
	if rt == nil || rt.Builtin {
		return ""
	}
	st := a.ProbeState(rt)
	if st.Current {
		return ""
	}
	return "assumed, not measured — run `posse runtime probe " + rt.Name + "`. " + rt.Name +
		" is template-only, so L1 rests on three behaviours nobody has measured for it: that child commands inherit the typed PATH, that a re-exec'd login shell comes from $SHELL (a CLI hardcoding /bin/zsh -l lets path_helper demote the gates dir and the shim never runs), and that its shell argv shapes are ones the gate wrapper parses. " + st.Why
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

// String renders the matrix for posse gates. Three verdicts, never spelled
// the same way (ADR 0017 §2's presentation rule): ✓ realized, ⊘ declared
// difference (safe by design — codex's SelfSandbox is the canonical
// example), ✗ degraded (a gate the wall does not hold). A declared
// difference alone never earns the header's DEGRADED — that word is
// reserved for a launch --allow-degraded actually has to waive
// (ranger-base-d17a).
func (p Parity) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s @ %s/%s:", p.Runtime, p.Cage, p.Tier)
	switch {
	case len(p.Degraded) > 0:
		b.WriteString(" DEGRADED\n")
	case len(p.DeclaredDifference) > 0:
		b.WriteString(" DECLARED DIFFERENCE\n")
	default:
		b.WriteString(" all gates realized\n")
	}
	for _, gate := range realizedOrder(p.Realized) {
		fmt.Fprintf(&b, "    ✓ %-28s %s\n", gate, p.Realized[gate])
	}
	for _, d := range p.DeclaredDifference {
		fmt.Fprintf(&b, "    ⊘ %s\n", d)
	}
	for _, u := range p.Degraded {
		fmt.Fprintf(&b, "    ✗ %s\n", u)
	}
	return b.String()
}

// realizedOrder fixes the order of the ✓ half of a block (rangerhq-epes).
// Realized is a map, so it used to print in Go's randomized iteration
// order: the content was stable but the lines moved, and the whole use of
// this matrix in review is "did this change move it?" — a question asked
// with a diff. The order chosen is the shape the ✗ half already has: the
// PID's deny rules first, then the gates that are not deny rules at all,
// each in a fixed place rather than wherever its name happens to sort.
func realizedOrder(realized map[string]RealizedGate) []string {
	gates := make([]string, 0, len(realized))
	for gate := range realized {
		gates = append(gates, gate)
	}
	sort.Slice(gates, func(i, j int) bool {
		if ri, rj := gateRank(gates[i]), gateRank(gates[j]); ri != rj {
			return ri < rj
		}
		return gates[i] < gates[j]
	})
	return gates
}

// gateRank groups the ✓ lines: 0 the PID's deny rules, then the three
// gates that are computed rather than typed, in the order CheckParity
// reaches them.
func gateRank(gate string) int {
	switch {
	case strings.HasPrefix(gate, "egress: "):
		return 1
	case strings.HasPrefix(gate, "skills: "):
		return 2
	case gate == RecordReachGate:
		return 3
	default:
		return 0
	}
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
