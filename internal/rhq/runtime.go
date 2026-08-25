package rhq

// Runtimes (ADR 0002 §1): a persona launches on a named launch profile,
// not a command string. Built-ins claude/codex/grok each carry a command
// template and a native realizer that turns the PID's allow/deny lists
// into that CLI's own flags — politeness (L0), never the wall; the wall
// is gates.go (§3, rangerhq-9ha). RHQ_HOME/runtimes/<name>.yaml
// (flat-YAML: `command:`, optional `model_<tier>:`/`model_flag:` and
// `gate_shell:`) adds a template-only runtime with no realizer:
// {allow}/{deny} render to nothing there and every gate goes to the wall,
// which is safe by construction.
//
// Precedence for which runtime a session gets: CLI --runtime > recipe
// runtime: > PID runtime: > config default_runtime: > claude.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultRuntime = "claude"

// Tiers (ADR 0003 §1): a name, not a model id — mapped per runtime.
const (
	TierStrong   = "strong"
	TierStandard = "standard"
	TierFast     = "fast"
	DefaultTier  = TierStrong
)

var Tiers = []string{TierStrong, TierStandard, TierFast}

func ValidTier(t string) bool {
	for _, x := range Tiers {
		if x == t {
			return true
		}
	}
	return false
}

// The dispatch contract (ADR 0013 §1). Six stages —
//
//	launch → promptable → work → record → settle → account
//
// — of which four are observed (herdr, the bead, the cost adapter) and two
// are DECLARED here, per runtime: how the work prompt is delivered, and
// whether a dispatched session of this runtime has been measured to write
// the store of record. `posse runtime check <name>` prints the grid.
//
// The expensive column to get wrong is the unknown one, so the zero value
// of every declaration is the noisy-and-honest reading, not the convenient
// one: a template-only runtimes/<name>.yaml that declares nothing is
// `prompt: typed`, `record: untrusted`, uncounted and tier-unmapped. It is
// dispatchable — ADR 0013 rejected "refuse until a conformance suite is
// green" — and it is loud.
const (
	// PromptArgv: the work prompt rides on the launch line as a positional
	// argument, so no screen is the delivery channel (ADR 0013 §2).
	PromptArgv = "argv"
	// PromptTyped: create → await promptable → claim → type, today's path,
	// with StartupWait as the patience.
	PromptTyped = "typed"

	// RecordTrusted: a *dispatched* session of this runtime has been
	// measured to close its bead. Promotion is an edit here (or a yaml key)
	// after that measurement — never a derived, auto-updating store, which
	// could disagree with the bead (ADR 0011's class).
	RecordTrusted = "trusted"
	// RecordUntrusted: the default everywhere else. Dispatch still launches;
	// gather never prints ✓ on settle-without-close, and unattended
	// --resume re-prompts.
	RecordUntrusted = "untrusted"

	// DefaultStartupWait is the claude-shaped patience for a launch to reach
	// a promptable screen. It is a per-runtime number (`startup_wait:`)
	// because 45s is measured on claude and grok's cold start exceeds it on
	// a clean screen (ranger-base-3j8).
	DefaultStartupWait = 45 * time.Second
)

func ValidPrompt(p string) bool { return p == PromptArgv || p == PromptTyped }

func ValidRecord(r string) bool { return r == RecordTrusted || r == RecordUntrusted }

// Interstitial is a first-run dialog this runtime draws that dispatch must
// not answer for the operator (ADR 0013 §2, layer 2). Posse NAMES the key
// that silences it and never writes it: one of these is a consent whose
// wrong answer donates the operator's private-repo prompts to training, and
// another's default action runs `brew upgrade` on their tooling.
//
// A dialog whose Danger is non-empty — the default action mutates the
// machine — is a launch REFUSE until the operator's own config silences it.
// Nothing blind-sends Enter.
type Interstitial struct {
	Screen  string // what the pane shows
	Where   string // the file the silencing key lives in (operator-owned)
	Key     string // the key itself, by name
	Silence string // what the operator does, with the SAFE choice named
	Danger  string // the default action when it mutates the machine ("" = safe default)
	// Probe reports whether the key is set on this machine. nil = posse
	// cannot cheaply tell, which prints as "unknown" rather than as "no".
	Probe func() (bool, string)
}

// Realized is what a native realizer produced: the two placeholder
// expansions and the rules it could express natively.
type Realized struct {
	Allow    string   // text for {allow} ("" = nothing)
	Deny     string   // text for {deny}
	Realized []string // rules expressed by the runtime's own surface (L0 — politeness)
	Enforced []string // of those, rules the runtime enforces at OS level (counts as wall in the parity check)
}

type Runtime struct {
	Name string
	// Path is the runtimes/<name>.yaml this runtime was loaded from ("" for
	// a built-in). It is what lets `runtime check` say WHO declared each
	// stage — a key read from the file and a key that fell back to the
	// built-in default are different facts to an onboarder.
	Path    string
	Command string // template: {file} {memory} {allow} {deny} {model}
	// Realize turns the PID's rule lists into this CLI's own flags. memory is
	// the persona's memory dir (unquoted): a runtime whose writable-dir flag is
	// only legal in some of the modes it picks has to name it here rather than
	// in the template (codex --add-dir, below).
	Realize   func(allow, deny []string, memory string) Realized
	Builtin   bool
	Models    map[string]string // tier → model id; unset tier → runtime default (fast falls back to standard)
	ModelFlag string            // printf form for {model}: "--model %s", "-c model=%s", "-m %s"
	// NoGateShell: do not point SHELL/GROK_SHELL at the gate shell on the
	// typed line for this runtime (ADR 0009 §2 exit hatch, `gate_shell:
	// false` in a template-only runtimes/<name>.yaml). The wrapper is what
	// keeps L1 alive on a runtime that re-execs a *login* shell — on macOS
	// that hands PATH to path_helper, which re-orders it so /usr/bin comes
	// before anything the launcher prepended, and the shim never runs (grok
	// 1.0.5, verified rangerhq-vjl). Setting this for a runtime that chokes
	// on a wrapper is honest but costly: the parity check then falls back to
	// unrealized for every Bash(...) deny there but `git push`, which L3
	// catches as a git hook rather than a PATH lookup.
	NoGateShell bool
	// Skills points this runtime at the persona's bound skills (ADR 0007 §2).
	// dir is the rendered skills tree (skills.go), names the PID's `skills:`
	// list; the return is what {skills} renders to, and whether this runtime
	// has a per-session skill surface at all. ok is a property of the
	// runtime, not of dir: false means the binding cannot be realized here
	// and the parity check refuses the launch. nil reads as "no surface"
	// (noSkills) — a template-only runtime without `skills_flag:`.
	Skills func(dir string, names []string) (flag string, ok bool)
	// SkillsCwd: this runtime discovers skills from the session's *working
	// directory* rather than from a flag, so the launch materializes
	// <cwd>/.agents/skills/<name> and {skills} renders nothing (codex and
	// grok — rangerhq-1qd). The binding is realized by those links being
	// there, not by anything typed on the line.
	SkillsCwd bool
	// SelfSandbox: the runtime wraps its own child commands in sandbox-exec
	// (codex). macOS refuses to nest seatbelts (sandbox_apply: Operation not
	// permitted — verified 2026-08-17), so our seatbelt tier cannot wrap it;
	// its own sandbox is what enforces Edit/Write there.
	SelfSandbox bool
	// ProjectConfig names a file *in the session directory* that this
	// runtime reads as configuration at launch, because posse typed the flag
	// that makes it do so (codex's directory trust). That is a channel from
	// the repo to the box which no model and no PID sits in front of, so the
	// launch checks for the file and refuses unless the PID opts in — see
	// ProjectConfigTrust in parity.go. Empty = this runtime takes nothing
	// from the session dir, which is the safe default for every
	// template-only runtime (posse types no trust flag there).
	ProjectConfig string
	// Unattended is the flag that makes this runtime approve a tool call
	// with nobody watching, and it is a launch GUARANTEE, not a template
	// detail (rangerhq-qs5r): every built-in template already carries it,
	// and RenderCommandFor puts it back when the rendered line does not
	// name it — the one path that can drop it is a PID's own `command:`,
	// which is a template written by hand. A session that starts in a
	// mode where an undenied command sits unapproved forever is not an
	// unattended session; it is a dialog nobody is watching, which is what
	// the mode landing was (claude), and what every mode but auto and
	// bypassPermissions is (grok). Empty = this runtime has no such flag:
	// every template-only runtime, where posse knows no CLI's dialect and
	// appending a guess would be worse than the gap.
	//
	// Matched on the flag's own key (the first word), so a template that
	// names the flag with a different VALUE keeps its value — an explicit
	// spelling in a hand-written PID beats an implicit one from here, and
	// it is visible in `ps` where a silent override would not be.
	Unattended string
	// CageCred names the env var an authenticated *caged* session of this
	// runtime needs (`cage_cred:` in a template-only runtime's yaml). A
	// container has no keychain, and the on-disk credential files are
	// stale there or unrefreshable read-only, so the operator mints one
	// and the launch refuses without it (ADR 0002 §4, rangerhq-kiz) —
	// cage.go. Empty falls back to the built-in table; a runtime in
	// neither is one whose container credential nobody has decided yet.
	CageCred string
	// Egress is what this runtime itself must reach for a session on it to
	// be a session at all — its API, and the host its OAuth refresh goes
	// to. ADR 0002 §4: the launcher ALWAYS adds these to the PID's
	// `egress:` list, because a cage that cannot reach its own model is not
	// an isolated persona, it is an offline one. Measured live in
	// rangerhq-89a, and the shape of the failure differs per runtime:
	// claude degrades quietly when a host it wanted is denied, codex
	// retries ~70 times in 35s and then errors hard. Telemetry hosts are
	// deliberately NOT here (claude's datadog, grok's mixpanel): a caged
	// persona's traffic is the operator's business and both degrade
	// quietly. `egress:` in a template-only runtime's yaml names its own.
	Egress []string

	// --- ADR 0013, the declared half of the dispatch contract ---

	// Prompt is how DISPATCH delivers the work prompt: PromptArgv (the
	// prompt file appended to the already-rendered launch line as
	// "$(cat <file>)", so no screen is the delivery channel) or PromptTyped
	// (create → await promptable → claim → type). Empty = PromptTyped, the
	// honest default for a runtime nobody has probed. Interactive `posse
	// new` is unaffected either way: it appends nothing.
	Prompt string
	// StartupWait is this runtime's patience for reaching a promptable
	// screen. Zero = DefaultStartupWait. A number here is MEASURED on the
	// runtime, never guessed — that is the whole reason it is per-runtime.
	StartupWait time.Duration
	// Record is whether a dispatched session of this runtime has been
	// measured to write the store of record (the bead — ADR 0011, never the
	// runtime's own settle). Empty = RecordUntrusted.
	Record string
	// RecordWhy is the measurement behind a RecordTrusted, named so a reader
	// can tell a promotion from an assumption. Ignored when untrusted.
	RecordWhy string
	// NativeRules are the rulebook files this runtime discovers and loads on
	// its own, ahead of anything posse types. Posse does NOT rewrite them —
	// the session cwd is the operator's shared checkout (ADR 0013 §4) — it
	// declares them so `runtime check` can say what else is talking to the
	// model. Whether a native file outranks the PID is a probe, not a patch.
	NativeRules []string
	// Interstitials are the first-run dialogs this runtime draws, with the
	// operator-owned config key that silences each. Documented, never
	// written.
	Interstitials []Interstitial
	// CostAdapter names the reading behind this runtime's `account` stage
	// ("" = no adapter → account-degraded, ADR 0013 §5). Filling the
	// cost-adapter seam (ADR 0012 D4) is how a runtime leaves that column;
	// until then `uncounted_cap_<runtime>:` is the brake and unset means
	// unlimited and loud.
	CostAdapter string
}

// PromptMode is how dispatch delivers the work prompt here. The zero value
// reads as typed: an unprobed runtime does not get argv delivery by
// default, because argv-skips-the-interstitial is an ASSUMED claim in ADR
// 0013 §2 and a wrong guess is a prompt typed into a hole.
func (rt *Runtime) PromptMode() string {
	if ValidPrompt(rt.Prompt) {
		return rt.Prompt
	}
	return PromptTyped
}

// Wait is this runtime's promptable patience.
func (rt *Runtime) Wait() time.Duration {
	if rt.StartupWait > 0 {
		return rt.StartupWait
	}
	return DefaultStartupWait
}

// RecordTrust is whether this runtime has been measured to close its bead.
// Unknown reads as untrusted — the direction that costs a re-prompt rather
// than a ✓ on work nobody recorded.
func (rt *Runtime) RecordTrust() string {
	if rt.Record == RecordTrusted {
		return RecordTrusted
	}
	return RecordUntrusted
}

// Counted: does a cost adapter read this runtime's spend? False is the
// account-degraded column (ADR 0013 §5) — never a claim that spend was $0.
func (rt *Runtime) Counted() bool { return rt.CostAdapter != "" }

// Model returns the model id for a tier on this runtime ("" = leave the
// runtime to its default). fast falls back to standard when unmapped.
func (rt *Runtime) Model(tier string) string {
	if id := rt.Models[tier]; id != "" {
		return id
	}
	if tier == TierFast {
		return rt.Models[TierStandard]
	}
	return ""
}

// Exe is the runtime's canonical executable name — the first word of its
// command template. It is the name herdr matches its agent manifests on,
// so it is also what the container tier's argv0 launcher is called and
// what that launcher sets argv[0] to (cagelauncher.go, rangerhq-1k1). The
// *runtime's* template and not the PID's own command:, deliberately — a
// PID free to write `env FOO=1 claude …` would otherwise name its agent
// `env` and disappear from `herdr agent list`.
func (rt *Runtime) Exe() string {
	f := strings.Fields(rt.Command)
	if len(f) == 0 {
		return ""
	}
	return filepath.Base(f[0])
}

// EnsureUnattended guarantees the rendered launch line names this
// runtime's unattended flag (rangerhq-qs5r). Appended at the end, which is
// safe on all three built-in CLIs: each parses options anywhere on the
// line, and each ends a variadic list at the next option token.
//
// Only when the line actually starts this runtime's CLI. A PID's command:
// is a template posse did not write and may not even be the runtime's
// executable — Exe() reads the runtime's own template for the same reason
// (`env FOO=1 claude …` names its agent `env`). Where posse cannot see the
// CLI it knows the dialect of, it appends nothing: a flag typed at the
// wrong program is a launch that fails outright, which is worse than the
// mode it was fixing.
func (rt *Runtime) EnsureUnattended(cmd string) string {
	f := strings.Fields(cmd)
	if rt.Unattended == "" || len(f) == 0 || filepath.Base(f[0]) != rt.Exe() {
		return cmd
	}
	key := strings.Fields(rt.Unattended)[0]
	for _, w := range f {
		if w == key || strings.HasPrefix(w, key+"=") {
			return cmd
		}
	}
	return cmd + " " + rt.Unattended
}

// ModelText is what {model} renders to for a tier: the runtime's flag with
// the id, or nothing when the tier has no mapping.
func (rt *Runtime) ModelText(tier string) string {
	id := rt.Model(tier)
	if id == "" || rt.ModelFlag == "" {
		return ""
	}
	return fmt.Sprintf(rt.ModelFlag, shellQuote(id))
}

func quoteEach(rules []string) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = shellQuote(r)
	}
	return out
}

// claude: rule syntax, variadic flags, deny wins. allow: is the PID's list
// verbatim; deny: goes through L0Spellings first, because claude's prefix
// match is literal and `git -C <repo> push` does not start with `git push`
// — the polite refusal was missing on exactly the spellings the L1 shim
// has to catch (rangerhq-3mc). Realized still names the PID's own rules:
// the extra spellings are how this runtime says them, not new gates.
func realizeClaude(allow, deny []string, _ string) Realized {
	var r Realized
	if len(allow) > 0 {
		r.Allow = "--allowedTools " + strings.Join(quoteEach(allow), " ")
	}
	if len(deny) > 0 {
		r.Deny = "--disallowedTools " + strings.Join(quoteEach(L0Spellings(deny)), " ")
	}
	r.Realized = append(append(r.Realized, allow...), deny...)
	return r
}

// grok: --allow/--deny take one rule each (compat aliases of claude's).
// Verified on grok 1.0.x (rangerhq-vjl, dialect re-probed in rangerhq-625):
// the rules really are claude's — `Bash(git push:*)` refuses `git push
// --dry-run` and leaves `git status` alone, bare tool names (`Edit`,
// `Write`) match every invocation, and deny beats allow even under
// --permission-mode bypassPermissions. Claude's dialect, not its matcher:
// `:*` is a prefix with no word boundary here, so that same rule also
// refuses `git pushy --help` (claude does not), and a rule with no
// wildcard is a prefix rather than an exact match. Still L0: the model is
// asked, not the kernel — a shell one-liner that spells the command
// differently walks past it, which is what the L1 shims are for.
//
// Deliberately NOT widened by L0Spellings the way claude's is, and the
// reason is not the one rangerhq-625 was filed on: grok's `*` IS a
// wildcard, and the option-blind pair would refuse every option spelling
// there too (all ten verified live on 1.0.5). What stops it is that grok
// splits a command like a shell before matching, so the matcher sees a
// re-rendered segment with the quotes taken off — and then `Bash(git -*
// push *)` also refuses `git -C <r> log --author "push me"` and `git -c
// user.name=t commit -m "push it upstream"`, both of which claude runs.
// At L0 a false positive is a hard block the model cannot ask its way
// past, which is the ground rangerhq-3mc rejected a single `Bash(git -*
// push*)` on; the same standard says no here. The true positive is not
// lost — L1 holds on grok since ADR 0009 (rangerhq-e43), and the shim
// refuses every one of those spellings. The whole-verb half would be a
// no-op besides: a plain `Bash(<cmd>)` is already a prefix on grok, not
// claude's exact match. TestGrokDialectIsWhyGrokIsNotWidened models it.
func realizeGrok(allow, deny []string, _ string) Realized {
	var r Realized
	var a, d []string
	for _, x := range allow {
		a = append(a, "--allow "+shellQuote(x))
	}
	for _, x := range deny {
		d = append(d, "--deny "+shellQuote(x))
	}
	r.Allow, r.Deny = strings.Join(a, " "), strings.Join(d, " ")
	r.Realized = append(append(r.Realized, allow...), deny...)
	return r
}

// codex: no per-verb rules; its sandbox modes are what it can express.
// -s read-only when the deny list covers Edit and Write, else
// -s workspace-write — always emitted. Allow rules have no surface.
//
// --add-dir rides with the mode, not with the template: under -s read-only
// codex *exits* on it ("Ignoring --add-dir (…) because the effective
// permissions do not allow additional writable roots") — verified on
// codex-cli 0.147.0, rangerhq-5oi. Under read-only the memory dir is
// readable anyway (codex reads the whole disk); only workspace-write needs
// it named, so the persona can append to its ORDERS.md.
func realizeCodex(allow, deny []string, memory string) Realized {
	has := map[string]bool{}
	for _, x := range deny {
		has[x] = true
	}
	if has["Edit"] && has["Write"] {
		// read-only is a seatbelt, not a prompt: OS-enforced.
		return Realized{Deny: "-s read-only", Realized: []string{"Edit", "Write"}, Enforced: []string{"Edit", "Write", "NotebookEdit"}}
	}
	d := "-s workspace-write"
	if memory != "" {
		d += " --add-dir " + shellQuote(memory)
	}
	return Realized{Deny: d}
}

// claude: --plugin-dir points a session at one rendered plugin dir, whose
// skills/ the CLI loads on top of the global ones — session-only, additive,
// and repeatable (posse binds one dir per persona, so once). Verified
// 2026-08-18, ADR 0007's table.
func skillsClaude(dir string, names []string) (string, bool) {
	if len(names) == 0 {
		return "", true
	}
	return "--plugin-dir " + shellQuote(dir), true
}

// skillsCwd is codex's and grok's surface: neither takes a flag, both read
// <cwd>/.agents/skills/<name>/SKILL.md at the directory the session starts
// in. Verified 2026-08-18 (rangerhq-1qd) on codex-cli 0.147.0 and grok
// 1.0.5 — symlinks are followed out of the repo, git's ignore rules do not
// hide a skill from either CLI, and codex reads that dir under every flag
// the fleet types (--disable hooks, allow_login_shell=false, the trust
// table, -s read-only). Neither climbs from a subdirectory to the repo
// root, so the link goes in the session dir itself. {skills} renders
// nothing: the materialization *is* the realization.
//
// What was checked and is not there, so nobody re-checks it:
//   - codex has no config key naming extra skill roots. `skills` is a real
//     config table (`-c skills=1` errors with "expected struct
//     SkillsConfig"), but its only field is `bundled.path`, which does not
//     add roots; the app-server's skills/extraRootsSet JSON-RPC method is
//     not reachable from the CLI, and unknown -c keys are silently ignored.
//     codex also reads <cwd>/.codex/skills — .agents is the neutral one.
//   - grok's `[skills] paths` exists but only in config.toml: the
//     GROK_CONFIG / GROK_CONFIG_PATH overlay is allowlisted to models,
//     features, toolset and shell_environment_policy and drops every other
//     table, "cannot add a discovery source" by design. Verified: the
//     overlay leaves grok's skill list unchanged.
//   - grok's `--agent <definition>` carries no skill-path field. A
//     definition with `skills:`/`skill_paths:` parses and binds nothing —
//     verified against a headless session's init line.
func skillsCwd(_ string, names []string) (string, bool) { return "", true }

// noSkills is a runtime with no per-session skill surface at all: a
// template-only runtime whose yaml names no skills_flag:. Binding nothing
// is realizable (there is nothing to point at); binding anything is not.
func noSkills(_ string, names []string) (string, bool) { return "", len(names) == 0 }

// SkillsText renders {skills} for this runtime and reports whether the
// binding is realizable here.
func (rt *Runtime) SkillsText(dir string, names []string) (string, bool) {
	if rt.Skills == nil {
		return noSkills(dir, names)
	}
	return rt.Skills(dir, names)
}

// RealizesSkills: can this runtime be pointed at these skills at all?
func (rt *Runtime) RealizesSkills(names []string) bool {
	_, ok := rt.SkillsText("", names)
	return ok
}

// Claude model ids per tier (ADR 0003 table; exact ids from the current
// CLI/API naming). codex/grok have no per-tier mapping yet: every tier
// leaves the runtime to its default and {model} renders empty.
var claudeModels = map[string]string{
	TierStrong:   "claude-fable-5",
	TierStandard: "claude-opus-5",
	TierFast:     "claude-sonnet-5",
}

// ClaudeFleetFlags is what a claude persona session needs to run
// unattended — the mode the OPERATOR DIRECTIVE of 2026-08-22 requires of
// every agent (rangerhq-qs5r):
//
//   - --permission-mode auto. Nothing on the claude line used to name a
//     mode at all, so the session got whatever the CLI defaulted to that
//     week — and a CLI default is not a launch fact: dispatched sessions
//     have landed in `manual`, blocked on approval dialogs nobody was
//     watching, until the operator cleared them by hand. Of the
//     six modes (acceptEdits, auto, bypassPermissions, manual, dontAsk,
//     plan) only `auto` and `bypassPermissions` approve a tool call with
//     nobody there; `auto` is the lower-privilege of the two and still
//     honours --disallowedTools, so it is the one the fleet types.
//
// Verified live on claude 2.1.239 (2026-08-22), on a herdr pane running
// the full fleet-shaped line (--model, --append-system-prompt, --add-dir,
// --settings, --disallowedTools): with `--permission-mode manual` the
// footer reads "⏸ manual mode on" — the reported symptom exactly — and with
// `--permission-mode auto` it reads "⏵⏵ auto mode on". The flag takes in
// both directions, which is the point: the mode is now a launch fact and
// not a CLI default that can move under us.
//
// It rides ahead of {allow}/{deny} deliberately: those render to claude's
// variadic --allowedTools/--disallowedTools, and a flag after a variadic
// list is a spelling nobody needs to think about twice.
const ClaudeFleetFlags = `--permission-mode auto`

// CodexFleetFlags is what a codex persona session needs beyond its sandbox
// mode and -a never — all three verified on codex-cli 0.147.0 (rangerhq-5oi),
// and without them a dispatched session either sits on a dialog nobody is
// watching or runs with the wall switched off:
//
//   - allow_login_shell=false. Codex otherwise runs every shell command
//     through a login shell that re-sources the operator's rc files, which
//     re-prepends the login PATH *ahead* of the one codex inherited: the L1
//     gates dir loses and `git push` reaches /usr/bin/git. This is the macOS
//     path_helper trap of ADR 0002 §3 biting one level further in. With the
//     flag off the shim wins again (`command -v git` → gates/<p>/bin/git).
//   - directory trust. "Do you trust the contents of this directory?" fires
//     per exact path — a trusted parent does not cover a repo underneath it —
//     and -a never does not suppress it. The TOML inline-table form is the one
//     codex's -c parser accepts; the dotted form (projects."/p".trust_level=…)
//     is silently ignored, as unknown -c keys are generally. $PWD is the pane
//     shell's cwd, i.e. the session dir, so the trust posse grants is scoped to
//     the one directory it launched in and RelaunchAgent re-types the same.
//     What trust costs, and why the launch checks for it (rangerhq-b7m,
//     verified again here on 0.147.0 with a scratch repo and `codex mcp
//     list`, no API turn): a trusted session also loads $PWD/.codex/config.toml
//     ("settings for a trusted repository", codex's own words). Every key posse
//     types on the line beats the project's value — -s, -a and
//     developer_instructions are ours whatever the repo says — but the keys
//     posse does *not* type are taken from the file: [mcp_servers.*] (a probe
//     server with command=/bin/sh shows up in `codex mcp list` only under
//     trust), notify, model_provider(s) (env_key names any session env var as
//     the bearer sent to base_url) and shell_environment_policy. mcp_servers
//     and notify are spawned by codex itself — outside its per-command
//     sandbox, with the whole session env, before any model turn. Hence
//     Runtime.ProjectConfig below and the launch-time check.
//   - hooks. A new or changed ~/.codex/hooks.json opens a "Hooks need review"
//     dialog. posse disables the feature for persona sessions rather than trusting
//     hooks blind — the cage is ours (ADR 0002 §3), not the runtime's plugins'.
//     Stock herdr reads that dialog as *idle*, so a prompt lands in the dialog;
//     etc/herdr/agent-detection/codex.toml fixes the detection for panes the
//     fleet did not launch (rangerhq-7ia). The flag stays either way: it is a
//     posture, not a workaround for the detection gap.
const CodexFleetFlags = `--disable hooks -c allow_login_shell=false -c "projects={\"$PWD\"={trust_level=\"trusted\"}}"`

// CodexProjectConfig is the file a trusted session dir hands codex —
// codex's own doc string for it is "Project `.codex/config.toml`: settings
// for a trusted repository, including sandbox, MCP, hooks, model, and
// reasoning defaults."
const CodexProjectConfig = ".codex/config.toml"

// GrokFleetFlags is what a grok persona session needs to run unattended —
// verified in rangerhq-vjl on grok 1.0.x (the headless mode matrix on
// 1.0.0, the live fleet sessions on 1.0.5 after the CLI self-updated
// mid-verification; nothing here changed across the two):
//
//   - --permission-mode auto. Of the six modes, only `auto` and
//     `bypassPermissions` approve a tool call with nobody watching; under
//     `default`, `acceptEdits` and `dontAsk` even an undenied command sits
//     unapproved forever. Both working modes still honour --deny (a denied
//     command is refused under bypassPermissions too), so `auto` is the
//     lower-privilege of the two and the one the fleet types. The flag also
//     beats the operator's ~/.grok/config.toml `[ui] permission_mode`,
//     which on this machine is "always-approve": the launch is the same
//     whatever the operator left in their config.
//
// Not passed, and why: grok's cross-session memory is off unless
// --experimental-memory is given, so the persona's memory stays the
// fleet's ORDERS.md; project hooks need explicit folder trust before they
// run, so an untrusted session dir cannot be hijacked by a repo's hooks
// and there is no trust dialog to clear (unlike codex).
//
// There is, however, a STARTUP SPLASH on every fresh pane (rangerhq-37c):
// the New worktree / Resume session / Quit menu, a changelog line and the
// "Help improve Grok" consent banner. etc/herdr/agent-detection/grok.toml
// reports that screen `blocked`, which is why nothing on this line tries
// to suppress it — and dispatch clears it, in awaitAgent, rather than any
// flag here doing so (rangerhq-7sbo).
//
// What the splash actually does was measured on 1.0.5, and it is milder
// than 37c recorded: the composer under the splash is LIVE. Text sent to
// that screen appears in the composer, Enter submits it, and the pane goes
// idle → working → done on the turn with the menu still drawn. Esc moves
// the focus into the composer but does not undraw anything. So the splash
// is a rendering fact, not a keyboard lock; see awaitAgent for what the
// launcher does with that, and rangerhq-7sbo for the measurements.
//
// `grok --minimal` renders no splash at all, which is why it was a
// candidate. It is not on this line: it is an experimental rendering mode,
// a fleet-wide look-and-feel decision, and every grok detection rule we
// have is anchored on the boxed composer it would stop drawing.
const GrokFleetFlags = `--permission-mode auto`

// Native rulebooks (ADR 0013 §4). What each CLI discovers and loads by
// itself, before anything posse types — a second instruction channel into
// the same session, living in the operator's shared checkout. Posse
// declares them and rewrites none of them: the work prompt's "PID
// guardrails override repo docs" line is the reconciliation, and whether a
// native file actually outranks the PID is a PROBE, not a patch. A runtime
// that fails that probe stays record: untrusted.
//
// Sources, so nobody re-derives them:
//   - claude 2.1.243, from the binary: "Claude Code hardcodes CLAUDE.md /
//     AGENTS.md discovery" (it says so while explaining why codex's
//     project_doc_fallback_filenames has no equivalent).
//   - codex-cli 0.147.0: AGENTS.md and AGENTS.override.md, project and
//     ~/.codex/; the set is widened by config project_doc_fallback_filenames.
//   - grok 1.0.5, docs/user-guide/12-project-rules.md, in its own order,
//     every match in a directory loaded (not first-wins), plus *.md under
//     the rules dirs at each level from repo root to cwd.
var (
	claudeNativeRules = []string{"CLAUDE.md", "CLAUDE.local.md", "AGENTS.md", ".claude/rules/*.md"}
	codexNativeRules  = []string{"AGENTS.md", "AGENTS.override.md", "~/.codex/AGENTS.md"}
	grokNativeRules   = []string{"Agents.md", "Claude.md", "CLAUDE.md", "CLAUDE.local.md", "AGENT.md", "AGENTS.md",
		".grok/rules/*.md", ".claude/rules/*.md", ".cursor/rules/*.md", "~/.grok/rules/*.md"}
)

// PROMPT DELIVERY IS `typed` ON ALL THREE, AND THAT IS THE PROBE'S DOING,
// NOT AN OVERSIGHT (ADR 0013 §2). The ADR's own table reads "grok/codex
// argv *if the probe holds*, else typed plus a measured wait" — and the
// probe (ranger-base-cl7: does an argv prompt skip the interstitial and
// still trip herdr `working`?) has not run. ASSUMED is not MEASURED, and
// the cost of guessing wrong here is a work prompt delivered into a screen
// nobody read. So argv is declarable today and declared nowhere.
//
// WHEN ranger-base-cl7 LANDS, this is the edit site: flip Prompt to
// PromptArgv for whichever runtimes the probe held for, or leave them typed
// and set a MEASURED StartupWait for the ones it falsified. Do not do both
// from a guess. ranger-base-dg5 is the dispatch half that reads it.
var builtinRuntimes = []Runtime{
	{Name: "claude", Builtin: true, Realize: realizeClaude, Skills: skillsClaude, Models: claudeModels, ModelFlag: "--model %s", Unattended: ClaudeFleetFlags,
		Egress: []string{"api.anthropic.com", "platform.claude.com"},
		// record: trusted — dispatched claude sessions close their beads;
		// that is the shape every other runtime is measured against.
		// account: the transcript scanner (cost.go) is the one adapter that
		// exists, and it reads ~/.claude/projects/*.jsonl.
		Prompt: PromptTyped, Record: RecordTrusted, RecordWhy: "dispatched sessions close their beads; the baseline the contract was written from",
		NativeRules: claudeNativeRules, CostAdapter: "transcript scanner (~/.claude/projects/*.jsonl, ADR 0003 §4)",
		Command: `claude {model} ` + ClaudeFleetFlags + ` --append-system-prompt "$(cat {file})" --add-dir {memory} --settings '` + ClaudeFleetSettings + `' {skills} {allow} {deny}`},
	{Name: "codex", Builtin: true, Realize: realizeCodex, Skills: skillsCwd, SkillsCwd: true, ModelFlag: "-c model=%s", SelfSandbox: true, ProjectConfig: CodexProjectConfig, Unattended: "-a never",
		Egress: []string{"chatgpt.com", "ab.chatgpt.com"},
		// record: untrusted — MEASURED the other way: 3/3 dispatched codex
		// sessions did the work and left the bead in_progress with no
		// comment, one of them on a dirty shared checkout (ranger-base-0fb).
		// Dispatch still launches; gather never ✓s a settle-without-close.
		Prompt: PromptTyped, Record: RecordUntrusted,
		NativeRules: codexNativeRules, Interstitials: CodexInterstitials,
		Command: `codex {model} {skills} {deny} -a never ` + CodexFleetFlags + ` -c developer_instructions="$(cat {file})"`},
	{Name: "grok", Builtin: true, Realize: realizeGrok, Skills: skillsCwd, SkillsCwd: true, ModelFlag: "-m %s", Unattended: GrokFleetFlags,
		Egress: []string{"cli-chat-proxy.grok.com", "grok.com"},
		// record: trusted — the qa lane on grok closed a bead properly on
		// 2026-08-24, which is the measurement the promotion needs and the
		// only reason grok and codex differ in this column.
		//
		// StartupWait is deliberately UNSET (= 45s) even though grok's cold
		// start is known to exceed it on a clean screen (ranger-base-3j8):
		// the replacement number is measured by ranger-base-cl7, and a
		// guessed one here would be the same class of mistake with a longer
		// timeout on it.
		Prompt: PromptTyped, Record: RecordTrusted, RecordWhy: "the qa lane closed a dispatched bead on 2026-08-24 (ADR 0013 §4)",
		NativeRules: grokNativeRules, Interstitials: GrokInterstitials,
		Command: `grok {model} {skills} ` + GrokFleetFlags + ` --rules="$(cat {file})" {allow} {deny}`},
}

// RuntimesDir holds template-only runtimes: RHQ_HOME/runtimes/<name>.yaml.
func (a *App) RuntimesDir() string { return filepath.Join(a.Home, "runtimes") }

// LoadRuntime returns a built-in by name, else a template-only runtime
// from RHQ_HOME/runtimes/<name>.yaml.
func (a *App) LoadRuntime(name string) (*Runtime, error) {
	if name == "" {
		name = DefaultRuntime
	}
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Name == name {
			rt := builtinRuntimes[i]
			return &rt, nil
		}
	}
	p := filepath.Join(a.RuntimesDir(), name+".yaml")
	if _, err := os.Stat(p); err != nil {
		return nil, Die("unknown runtime %q (built-ins: claude, codex, grok; or %s)", name, AbbrevHome(p))
	}
	cmd := YamlGet(p, "command")
	if cmd == "" {
		return nil, Die("runtime %s: %s has no command:", name, AbbrevHome(p))
	}
	// Optional per-tier models (model_strong: …) and the flag {model} renders
	// with (model_flag:, default --model).
	rt := &Runtime{Name: name, Path: p, Command: cmd, Models: map[string]string{}, ModelFlag: "--model %s"}
	for _, t := range Tiers {
		if id := YamlGet(p, "model_"+t); id != "" {
			rt.Models[t] = id
		}
	}
	if f := YamlGet(p, "model_flag"); f != "" {
		rt.ModelFlag = f + " %s"
	}
	// skills_flag: the printf form {skills} renders with, given the rendered
	// skills dir ("--plugin-dir %s"). Absent → this runtime has no skill
	// surface and a PID that binds skills cannot launch on it.
	if f := YamlGet(p, "skills_flag"); f != "" {
		flag := f
		rt.Skills = func(dir string, names []string) (string, bool) {
			if len(names) == 0 {
				return "", true
			}
			return fmt.Sprintf(flag+" %s", shellQuote(dir)), true
		}
	}
	// cage_cred: the env var this runtime authenticates with inside a
	// container (cage.go). Absent = undecided, and `cage: container`
	// refuses on this runtime rather than starting a session that cannot
	// reach its API.
	if v := YamlGet(p, "cage_cred"); v != "" {
		rt.CageCred = v
	}
	// egress: the hosts this runtime itself needs, always added to a caged
	// PID's allowlist. Absent = none known, and a caged session on it
	// reaches only what its PID names — which for an unknown CLI is the
	// honest default: posse has no business guessing an API host.
	rt.Egress = YamlList(p, "egress")
	// gate_shell: false — this runtime chokes on a wrapper named as the
	// shell, so the typed line leaves SHELL/GROK_SHELL alone (ADR 0009 §2).
	if YamlGet(p, "gate_shell") == "false" {
		rt.NoGateShell = true
	}
	// --- the ADR 0013 dispatch-contract keys ---
	//
	// ABSENT is the loud reading, and it is reached by doing nothing: this
	// runtime stays prompt: typed, record: untrusted, uncounted and
	// tier-unmapped. A template-only yaml with no declarations at all is a
	// dispatchable runtime that says so on every line of `runtime check`.
	//
	// PRESENT-BUT-WRONG is a different animal and refuses. `record: trused`
	// would otherwise be silently demoted to untrusted, and `prompt: arvg`
	// silently kept typed — a typo that reads as a declaration is exactly
	// the silence this contract exists to remove.
	if v := YamlGet(p, "prompt"); v != "" {
		if !ValidPrompt(v) {
			return nil, Die("runtime %s: %s has prompt: %q (want %s or %s — ADR 0013 §2)", name, AbbrevHome(p), v, PromptArgv, PromptTyped)
		}
		rt.Prompt = v
	}
	if v := YamlGet(p, "startup_wait"); v != "" {
		d, err := ParseInterval(v)
		if err != nil {
			return nil, Die("runtime %s: %s has startup_wait: %q — %v", name, AbbrevHome(p), v, err)
		}
		rt.StartupWait = d
	}
	if v := YamlGet(p, "record"); v != "" {
		if !ValidRecord(v) {
			return nil, Die("runtime %s: %s has record: %q (want %s or %s — ADR 0013 §4)", name, AbbrevHome(p), v, RecordTrusted, RecordUntrusted)
		}
		rt.Record = v
		rt.RecordWhy = YamlGet(p, "record_why")
	}
	// native_rules: the rulebook files this CLI loads on its own. Posse
	// never writes them; declaring them is how `runtime check` can name the
	// other voice in the session.
	rt.NativeRules = YamlList(p, "native_rules")
	return rt, nil
}

// ResolveTier applies the launch-site precedence available here: explicit
// (CLI/recipe/dispatch) > PID tier: > config default_tier > strong. Bead
// labels and tier_by_label are dispatch's business (ADR 0003 §2).
func (a *App) ResolveTier(explicit string, ag *AgentFile) string {
	if explicit != "" {
		return explicit
	}
	if ag != nil && ag.Tier != "" {
		return ag.Tier
	}
	return a.CfgGet("default_tier", DefaultTier)
}

// ListRuntimes returns built-in names plus any runtimes/*.yaml, sorted-ish
// (built-ins first).
func (a *App) ListRuntimes() []string {
	var out []string
	for _, rt := range builtinRuntimes {
		out = append(out, rt.Name)
	}
	ents, _ := os.ReadDir(a.RuntimesDir())
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			out = append(out, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	return out
}

// ResolveRuntime applies the precedence: explicit (CLI or recipe) > PID >
// config default_runtime > claude. ag may be nil (no persona).
func (a *App) ResolveRuntime(explicit string, ag *AgentFile) string {
	if explicit != "" {
		return explicit
	}
	if ag != nil && ag.Runtime != "" {
		return ag.Runtime
	}
	return a.CfgGet("default_runtime", DefaultRuntime)
}

// EmojiExact returns the config emoji: entry for exactly name ("" if none).
func (a *App) EmojiExact(name string) string {
	for _, kv := range YamlMapPairs(a.ConfigPath, "emoji") {
		if kv[0] == name {
			return kv[1]
		}
	}
	return ""
}
