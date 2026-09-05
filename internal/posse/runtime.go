package posse

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

	// RulesPrecedencePID: measured — a collision between the PID's own
	// rules and this runtime's native rulebooks (NativeRules) resolves in
	// the PID's favor.
	RulesPrecedencePID = "pid"
	// RulesPrecedenceNative: measured the other way — a native rulebook
	// this runtime discovers on its own outranks the PID on collision.
	RulesPrecedenceNative = "native"

	// DefaultStartupWait is the claude-shaped patience for a launch to reach
	// a promptable screen. It is a per-runtime number (`startup_wait:`)
	// because 45s is measured on claude and grok's cold start exceeds it on
	// a clean screen (ranger-base-3j8).
	DefaultStartupWait = 45 * time.Second
)

func ValidPrompt(p string) bool { return p == PromptArgv || p == PromptTyped }

func ValidRecord(r string) bool { return r == RecordTrusted || r == RecordUntrusted }

func ValidRulesPrecedence(p string) bool {
	return p == RulesPrecedencePID || p == RulesPrecedenceNative
}

// Interstitial is a first-run dialog this runtime draws that dispatch must
// not answer for the operator (ADR 0013 §2, layer 2). Posse NAMES the key
// that silences it and, with one declared exception, never writes it: one
// of these is a consent whose wrong answer donates the operator's
// private-repo prompts to training, and another's default action runs
// `brew upgrade` on their tooling.
//
// The exception is Seeded, and it is narrow on purpose (rangerhq-w4uf):
// claude's directory-trust dialog has no key the operator can set ONCE —
// it is per session directory, so a fleet that grows a new repo, worktree
// or scratch dir grows a new dialog with it, and the answer posse writes
// is the same grant it already types on codex's line. Everything else here
// stays the operator's.
//
// A dialog whose Danger is non-empty — the default action mutates the
// machine — is a launch REFUSE until the operator's own config silences it.
// Nothing blind-sends Enter.
type Interstitial struct {
	Screen  string // what the pane shows
	Where   string // the file the silencing key lives in (operator-owned unless Seeded)
	Key     string // the key itself, by name
	Silence string // what the operator does, with the SAFE choice named — or what the launch does when Seeded
	Danger  string // the default action when it mutates the machine ("" = safe default)
	// Seeded: the LAUNCH writes this key, rather than naming it and
	// refusing. True only where the key is per-session-directory and the
	// grant is one posse already makes on another runtime.
	Seeded bool
	// Probe reports whether the key is set on this machine. nil = posse
	// cannot cheaply tell, which prints as "unknown" rather than as "no".
	Probe func() Silence
}

// Silence is what a read-only probe can honestly say about one interstitial
// key on this box — three states, not two, and the third is the one that
// decides a launch.
//
// The distinction was prose in interstitial.go's header ("an unreadable or
// absent file is 'unknown', never 'no'") and a plain `false` in the code,
// which is fine while the answer only prints. It stopped being fine when
// DangerUnsilenced started refusing launches on it: refusing on "posse
// could not read the file" walls a box for something nobody measured, and
// says so in the refusal's own words (ranger-base-9r33).
//
// A probe that cannot distinguish the two never returns Unknown — grok's
// two keys are written by grok itself, so a config without them is an
// answered question, not an unreadable one.
type Silence struct {
	Silenced bool   // the key is set: this screen will not draw
	Unknown  bool   // posse could not read the file — "cannot tell", never "no"
	Why      string // the reading, in the probe's own words, for the line that prints it
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
	//
	// writable names further directories the session must be able to WRITE
	// that lie outside its workspace. It is empty for runtimes posse cages
	// itself, and load-bearing for one that sandboxes itself (codex): a path
	// nobody names is a path the persona cannot write, which is how five
	// consecutive dispatched sessions did their work and could not record it
	// (ranger-base-0fb — the store of record sat behind a .beads redirect).
	Realize   func(allow, deny []string, memory string, writable ...string) Realized
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
	// ProjectConfig names the files *in the session directory* that this
	// runtime reads as configuration at launch because posse made the session
	// directory trusted. That is a channel from the repo to the box which no
	// model and no PID sits in front of, so the launch checks them and refuses
	// unless the PID opts in — see ProjectConfigTrust in parity.go. Empty =
	// this runtime takes no project-config surface from the session dir, which
	// is the safe default for every template-only runtime.
	//
	// A list because one runtime's project scope can be more than one file:
	// claude reads `.claude/settings.json` and `.claude/settings.local.json`
	// as the same scope, from the same directory, under the same keys
	// (measured, rangerhq-9u8). `project_config:` in a runtime yaml still
	// declares exactly one path.
	ProjectConfig []string
	// ProjectConfigKeys narrows the check to the top-level JSON keys that
	// declare a repo-to-box executable channel. Empty preserves the
	// whole-file presence predicate (codex) — which is why
	// `project_config_keys:` is the one declarable key that LOOSENS a
	// safety check, and why declaring it without `project_config:`, or
	// empty, refuses at load rather than narrowing nothing quietly. A keyed file that cannot be
	// classified as a readable top-level JSON object fails closed (claude).
	// The check is on CONTENT, not on the runtime's trust state: claude's
	// trust gates its permission keys, its hooks are gated a layer down and
	// only where the dialog is a live screen, and a headless `claude -p`
	// runs an untrusted repo's hooks outright (measured, ranger-base-i0s8,
	// see trust.go). Naming a key the runtime ignores this release is the
	// conservative side of that.
	ProjectConfigKeys []string
	// Unattended is the flag that makes this runtime approve a tool call
	// with nobody watching, and it is a launch GUARANTEE, not a template
	// detail (rangerhq-qs5r): every built-in template already carries it,
	// and RenderCommandFor puts it back when the rendered line does not
	// name it — the one path that can drop it is a PID's own `command:`,
	// which is a template written by hand. A session that starts in a
	// mode where an undenied command sits unapproved forever is not an
	// unattended session; it is a dialog nobody is watching, which is what
	// the mode landing was (claude), and what every mode but auto and
	// bypassPermissions is (grok). Empty = this runtime has no such flag,
	// or nobody has measured one: posse knows no unmeasured CLI's dialect
	// and appending a guess would be worse than the gap.
	//
	// Declarable as `unattended:` in a template-only runtime's yaml
	// (ranger-base-ncxa). What makes that safe is whose measurement it is:
	// posse never guesses the flag, the operator names one they ran. The
	// value is validated as a flag posse may append to a shell line, not
	// merely as a string (runtimeFlagValue).
	//
	// Matched on the flag's own key (the first word), so a template that
	// names the flag with a different VALUE keeps its value — an explicit
	// spelling in a hand-written PID beats an implicit one from here, and
	// it is visible in `ps` where a silent override would not be.
	Unattended string
	// FleetSettings renders the settings payload {settings} carries — the
	// one flag whose VALUE posse computes at launch rather than spelling in
	// the template, because it holds the environment pin and that is a
	// property of the box, not of the line — the credential dirs
	// (credentialDirPin, ranger-base-rq83c) and the transport/exec inlets
	// (inletPin, ranger-base-rflee). Built-in only, and nil for every runtime but
	// claude: no other CLI has a measured settings surface, and {settings}
	// renders to nothing where it is nil.
	FleetSettings func() string
	// SettingsPin is the SMALLER payload — the environment pin alone, with
	// none of the fleet policy around it — that
	// EnsureSettingsPin appends to a rendered line carrying no
	// settings flag of its own. Deliberately not FleetSettings: a template
	// posse did not write is owed the security guarantee, not posse's
	// unattended policy, and the mode half of that policy already reaches
	// it through EnsureUnattended.
	SettingsPin func() string
	// PIDVoid names the flags that make this runtime IGNORE the PID
	// channel — the flag its own template delivers the persona identity
	// document on. A rendered launch line naming one is REFUSED
	// (ranger-base-64qx): what would open is not a degraded persona
	// session, it is a different session, carrying every native rulebook
	// and none of the persona.
	//
	// MEASURED on grok 1.0.5, 2026-08-28, both spellings
	// (docs/adr/0013-rules-precedence-probe.md): with
	// `--system-prompt-override` — or its compat alias `--system-prompt` —
	// on a line that also carries `--rules`, the assembled
	// `system_prompt.txt` is the override text alone (19 B), with no
	// `<human_rules>` block and no PID marker, while `prompt_context.json`
	// still carries the project rulebook in `agents_md_files` in full.
	// Vendor-documented besides ($GROK_HOME/docs/user-guide/
	// 12-project-rules.md): "Grok uses the text verbatim and skips both the
	// default system prompt and --rules."
	//
	// Why a refusal, when Unattended above gets a REPAIR: the unattended
	// flag is absent and appendable, so posse can put it back. The PID flag
	// here is present and ignored — the measured arm already had `--rules`
	// on the line — so there is nothing to restore, and appending it again
	// would buy a launch that looks fixed and is not. The only repair that
	// would work is rewriting the operator's override TEXT to carry the
	// PID, and a hand-written `command:` is the one template posse does not
	// get to edit.
	//
	// Empty = this runtime has no such flag, or nobody has measured one.
	// Built-in only, and NOT for Unattended's reason any more now that
	// `unattended:` is declarable: an unattended flag posse gets wrong is
	// appended and visible in `ps`, while a PIDVoid entry got wrong REFUSES
	// launches for a spelling nobody measured. The declarable one is the
	// one whose error is recoverable.
	PIDVoid []string
	// CageCred names the env var an authenticated *caged* session of this
	// runtime needs (`cage_cred:` in a template-only runtime's yaml). A
	// container has no keychain and posse never reads an on-disk credential
	// file there either, so the operator mints one and the launch refuses
	// without it (ADR 0002 §4, rangerhq-kiz) — cage.go. Empty falls back to
	// the built-in table; a runtime in neither is one whose container
	// credential nobody has decided yet.
	CageCred string
	// CredBin names the binary this runtime's OWN credential path execs BY
	// BARE NAME — so an L1 shim over it stands in front of the RUNTIME, not
	// only in front of the persona. That is not a hypothetical: the gates
	// dir is prepended on the TYPED line (ADR 0002 §3), so it leads the PATH
	// of the runtime process itself and of everything it spawns.
	//
	// A name-keyed fact and deliberately not declarable: which binary a CLI
	// authenticates itself with is that CLI's own state, the class ADR 0017
	// §3 names and accepts ("cage.go seeding, credential paths, trust.go's
	// claude dialog"). Empty is the honest default — a runtime whose
	// credential path nobody has measured claims nothing here, and the
	// launch says nothing rather than guessing.
	//
	// MEASURED on the darwin release artifact, claude 2.1.258
	// (ranger-base-eupf): the credential path execs `security` by bare name
	// for the read, for the delete, and for the write — the write primarily
	// through the stdin batch mode `security -i`, falling back to argv only
	// when the payload exceeds that mode's stdin limit. Two consequences,
	// and the second is the one that has never been watched: a session whose
	// PID denies that binary reports "Not logged in" at once, and at token
	// expiry the refresh has no way to write back either. The stdin mode is
	// also why this is a WARNING and not a carve-out: `security -i` takes
	// its commands on stdin, so an argv matcher cannot tell the runtime's
	// own credential write from any other keychain command, and a shim that
	// let it through would not be a narrowed deny — it would be no deny.
	CredBin string
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
	// RulesPrecedence is which channel wins when a NativeRules file
	// collides with the PID: RulesPrecedencePID or RulesPrecedenceNative.
	// Zero value is UNMEASURED, the loud default (ADR 0017 §5) — NativeRules
	// only declares what a runtime READS, and nothing else declares who
	// WINS. Display-by-design like RecordWhy below: its consumer is the
	// onboarder and the record-trust decision, never a code branch — ADR
	// 0013 §4 already decided declare-don't-suppress, and a runtime
	// measuring "native" stays record: untrusted by the existing rule.
	//
	// The first non-zero values came from a BEHAVIOURAL measurement, not
	// from placement: ranger-base-xaev's structural probe left all three
	// built-ins UNMEASURED on purpose (placement is why-string material —
	// the value is a claim about which instruction the model FOLLOWS), and
	// ranger-base-6rcv's billed turn on 2026-09-01 answered it for codex
	// and grok. claude was not authorized a turn and stays UNMEASURED.
	RulesPrecedence string
	// RulesPrecedenceWhy is the measurement behind a non-zero
	// RulesPrecedence — a probe bead id and date, so a reader can tell a
	// measured value from a guess. Ignored when RulesPrecedence is unset.
	RulesPrecedenceWhy string
	// Interstitials are the first-run dialogs this runtime draws, with the
	// operator-owned config key that silences each. Documented, never
	// written.
	Interstitials []Interstitial
	// StateDirs are the directories (and single files) this CLI keeps its
	// own state in — its config, its credentials, its per-project record.
	// They join the L2 seatbelt's writable set, because a runtime whose
	// state dir is read-only inside the sandbox is one that re-runs its
	// first-run flow every launch, or dies on a config write nobody granted
	// (seatbelt.go). Declared per runtime rather than listed centrally:
	// `~/.claude ~/.codex ~/.grok` was a literal in the profile builder, so
	// a third-party CLI's state dir could not be named at all and
	// `cage: seatbelt` broke it silently (ADR 0012 D4).
	//
	// Paths are absolute or ~-prefixed; a relative one is refused at load,
	// because the session cwd is already granted and "relative to the CLI's
	// idea of home" is not a thing this can resolve.
	//
	// An entry that names a FILE also gets its atomic-write siblings —
	// `<p>.lock`, `<p>.tmp.<pid>.<hex>` — because a CLI that replaces its
	// config by rename writes those and not the file, and a grant on the
	// file alone loses every write while the CLI reports success
	// (SeatbeltSiblings, ranger-base-cypy1). Declared runtimes get that for
	// free; it is not a claude special case.
	StateDirs []string
	// EnvRequired are the environment variable NAMES a session on this
	// runtime cannot work without — the Bedrock/Vertex shape, where the CLI
	// is installed and on PATH and every launch is a dead pane because
	// AWS_REGION was never in the session's env. Names only: posse never
	// reads, prints or forwards a value from this list (ADR 0012 D4, and
	// the envs: rule that values are never displayed anywhere).
	//
	// Checked at launch preflight (planLaunch) against the env sets the
	// session receives plus the launcher's own environment, and reported by
	// `posse runtime check`. Absent = nothing is required, which is the
	// truthful default for a CLI that authenticates from its own state dir.
	EnvRequired []string
	// TurnOutcomeAdapter names the reader behind this runtime's TURN
	// OUTCOME — whether posse can see what the CLI's own first turn did,
	// as opposed to whether herdr saw the pane settle. "" = no reader, and
	// that is a DEGRADE with a line on it (dispatch.go's settle clause):
	// an account that refused the turn settles exactly like an agent that
	// worked and skipped the bead, and until ranger-base-02zr the two were
	// the same sentence on codex and grok.
	//
	// The value is a registry key (turnfailure.go), not prose: a name no
	// reader implements is refused at load, so a runtime that declares a
	// reading gets one. Same seam as the cost adapter (ADR 0012 D4), same
	// rule that the DECLARATION is what dispatch keys on and never the
	// runtime's name (ADR 0017 §3) — but the cost side has no field here
	// at all: its registry is keyed by runtime name, so a second
	// declaration on this struct could only drift, and did (0lg6). Ask
	// CostRead/CostPriced/CostReading below.
	TurnOutcomeAdapter string
	// PaneModeAdapter names the reader behind what this runtime's PANE says
	// about the permission mode a session is in — declarable as `pane_mode:`,
	// in the registry-key shape of ADR 0017 §4 and verbatim the one
	// TurnOutcomeAdapter above wears (ADR 0057 D1).
	//
	// "" = no reader, and that is the loud default: every session on this
	// runtime lists as `mode:?` with a why saying nobody has measured what
	// its pane renders. `none` is the OPPOSITE of that — a CLI measured to
	// render no mode on any screen, whose column is a permanent `—` and whose
	// listing costs no pane read at all. The two are different facts and the
	// surfaces spell them differently, which is why `none` is a declaration
	// and not an omission (permissionmode.go).
	//
	// Keyed on the READER, never on the runtime's name: the codex branch and
	// the name-keyed reader map this replaced were an ADR 0017 §3 shadow
	// predicate, and a fourth CLI painting a claude-shaped footer can now be
	// read by declaring one line of yaml. Which reader parses a CLI's screen
	// is code measured against that CLI's captures rather than a fact about
	// this box, so it is also a MECHANISM key under ADR 0021 D2: a built-in's
	// overlay may not move it.
	PaneModeAdapter string
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

// CostRead: is any cost adapter reading this runtime at all? This is the
// question `posse cost` calls counted/uncounted (CostReport.Uncounted, the
// cockpit's `$uncounted`): false means the sessions are absent from every
// total — never a claim that the spend was $0.
//
// Resolved through the registry, like ReadsTurnOutcome below and for the
// same reason. There is deliberately no `cost_adapter:` field to declare:
// registering the adapter is the whole act, and a runtime gains and loses
// this column on the commit that adds or removes one, with nothing here to
// edit (ranger-base-0lg6).
func (rt *Runtime) CostRead() bool {
	_, ok := CostProviderFor(rt.Name)
	return ok
}

// CostPriced: do this runtime's DOLLARS reach `posse cost`? The narrower
// question, and the one ADR 0013 §5's brake keys on — a runtime that is
// read but never priced (codex: a plan seat reports no cost and no list
// rate applies) has no dollar meter either, which is what
// `uncounted_cap_<runtime>:` stands in for.
//
// False is never a claim that spend was $0; it is one of the two degrades,
// and CostReading says which.
func (rt *Runtime) CostPriced() bool {
	p, ok := CostProviderFor(rt.Name)
	return ok && p.Prices()
}

// CostReading is what the adapter reading this runtime reads, for display —
// "" when none reads it. The two degrades are told apart by exactly this:
// "" is UNCOUNTED (nothing reads it), non-empty with CostPriced false is
// UNPRICED (this reads it, and prices none of it).
func (rt *Runtime) CostReading() string {
	if p, ok := CostProviderFor(rt.Name); ok {
		return p.Reads()
	}
	return ""
}

// ReadsTurnOutcome: can posse read what this runtime's own turn did? False
// is the blind column — never a claim that the turn was healthy.
func (rt *Runtime) ReadsTurnOutcome() bool { return TurnOutcomeReaderFor(rt) != nil }

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

// TierMap is the per-tier mapping as a reader needs it: the tiers that
// render a model id here, in Tiers order and as "<tier>=<id>", and the
// tiers that render NOTHING. It exists so every surface that shows the
// dial — `posse runtimes`, the `runtime check` grid — says the same thing
// from one rendering, and so a PARTIAL map cannot be shown as a full one
// by listing only what is mapped (ranger-base-arm: a tier nobody mapped is
// the fact the reader came for).
//
// Effective, not literal: the fast → standard fallback of Model() is
// applied, so a runtime mapping only `model_standard:` reports fast as
// mapped, because fast really does render a model there.
func (rt *Runtime) TierMap() (mapped, unmapped []string) {
	for _, t := range Tiers {
		if id := rt.Model(t); id != "" {
			mapped = append(mapped, t+"="+id)
		} else {
			unmapped = append(unmapped, t)
		}
	}
	return mapped, unmapped
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

// FleetSettingsText renders {settings}: the whole flag, not just its value,
// so a runtime with no settings surface renders nothing and the placeholder
// costs its template nothing. Shell-quoted here rather than wrapped in
// literal quotes in the template, because the payload now carries PATHS —
// a home directory with an apostrophe in it would otherwise end the
// literal and hand the shell the rest of the JSON as words.
func (rt *Runtime) FleetSettingsText() string {
	if rt.FleetSettings == nil {
		return ""
	}
	return "--settings " + shellQuote(rt.FleetSettings())
}

// EnsureSettingsPin guarantees the rendered line carries the credential-dir
// pin — a launch guarantee for the same reason the unattended mode is, and
// reaching the same template posse did not write. The pin is what stops a
// persona-writable ~/.claude/settings.json from moving where this session's
// runtime reads and writes its credential, and from handing that session's
// children something to execute or a place to send its traffic
// (credentialDirPin, ranger-base-rq83c; inletPin, ranger-base-rflee).
//
// Appended only when the line names no `--settings` at all, and only when
// it starts this runtime's CLI — EnsureUnattended's two conditions, plus
// one this flag has of its own: a second `--settings` is not an addition.
// Measured on claude 2.1.259, the last occurrence REPLACES the first, so
// appending a payload beside an existing one would silently drop whatever
// that template was passing. A `command:` that spells its own --settings
// therefore owns its own pin; that is the residual, and it is a template
// nobody on this box writes (the built-in claude runtime refuses
// `command:` in its overlay, ADR 0021).
func (rt *Runtime) EnsureSettingsPin(cmd string) string {
	f := strings.Fields(cmd)
	if rt.SettingsPin == nil || len(f) == 0 || filepath.Base(f[0]) != rt.Exe() {
		return cmd
	}
	for _, w := range f {
		if w == "--settings" || strings.HasPrefix(w, "--settings=") {
			return cmd
		}
	}
	return cmd + " --settings " + shellQuote(rt.SettingsPin())
}

// PIDVoided reports which of this runtime's PIDVoid flags a rendered launch
// line names ("" = none), so the launch can be refused before a persona
// session opens without its PID (ranger-base-64qx).
//
// Matched on the flag's own token — `--flag` or `--flag=value` — the same
// way EnsureUnattended matches its key, so a longer flag that merely starts
// with one of these is a different flag and does not fire. It is also why
// grok's two spellings are both listed rather than one being a prefix of
// the other: `--system-prompt-override` is not `--system-prompt`.
//
// Unlike EnsureUnattended this does NOT first check that the line starts
// this runtime's CLI, and the asymmetry is deliberate. There, acting on a
// line posse cannot identify means typing a flag at a program that would
// fail outright, so an unrecognizable line is left alone. Here, acting
// wrongly costs a loud refusal that names the flag it saw and the PID it
// was protecting; staying silent costs a persona session that runs as
// somebody else with nothing observable to say so. The recoverable error is
// the one to make.
func (rt *Runtime) PIDVoided(cmd string) string {
	for _, w := range strings.Fields(cmd) {
		for _, f := range rt.PIDVoid {
			if w == f || strings.HasPrefix(w, f+"=") {
				return f
			}
		}
	}
	return ""
}

// ModelText is what {model} renders to for a tier: the runtime's flag with
// the id, or nothing when the tier has no mapping.
func (rt *Runtime) ModelText(tier string) string {
	return rt.ExactModelText(rt.Model(tier))
}

// ExactModelText renders one model id through this runtime's own model flag
// and the same shell quoting the tier map's id gets. It is what ADR 0053 D2
// means by "the runtime's model flag and the existing shell quoting carry
// the id": a canary launch introduces no second rendering and no raw
// command — it substitutes the id and nothing else.
//
// "" when the runtime declares no model flag, which is the launch's own
// refusal one layer up (planLaunch): there is nowhere for a typed id to go,
// and a launch that silently dropped it would open on the tier's model
// while the record said otherwise.
func (rt *Runtime) ExactModelText(id string) string {
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
// verbatim; deny: goes through L0Spellings first, because claude reads a
// whole-verb rule as EXACT (`Bash(bd)` never reaches `bd show x`) and has
// no negation at all. Realized still names the PID's own rules: the extra
// spellings are how this runtime says them, not new gates. It used to add
// option-blind spellings for a subcommand rule too — `git -C <repo> push`
// matches neither the string nor the token prefix `git push` — and those
// are gone, false positives and all (rangerhq-ky3, rangerhq-vr6j; the
// L0Spellings block says why no narrower spelling exists).
func realizeClaude(allow, deny []string, _ string, _ ...string) Realized {
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
//
// The pair itself is retired now (rangerhq-vr6j), and the answer here does
// not change with it. What L0Spellings still emits is claude's on its own
// terms: a negative rule reaches claude as one EXACT spelling,
// `Bash(git commit)`, which is what keeps it off the qualified form — and
// that same text is a PREFIX on grok, where it would refuse the very
// `git commit -- <path>` the PID allows.
func realizeGrok(allow, deny []string, _ string, _ ...string) Realized {
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
func realizeCodex(allow, deny []string, memory string, writable ...string) Realized {
	// ADR 0014 §1: `Edit(**)` is the bare rule written the long way, so the
	// mode this picks must not depend on which spelling the PID used.
	has := wholeTreeWriteDeny(deny)
	if has["Edit"] && has["Write"] {
		// read-only is a seatbelt, not a prompt: OS-enforced.
		return Realized{Deny: "-s read-only", Realized: []string{"Edit", "Write"}, Enforced: []string{"Edit", "Write", "NotebookEdit"}}
	}
	d := "-s workspace-write"
	seen := map[string]bool{}
	for _, w := range append([]string{memory}, writable...) {
		w = codexWritableRoot(w)
		if w == "" || seen[w] {
			continue
		}
		seen[w] = true
		d += " --add-dir " + shellQuote(w)
	}
	return Realized{Deny: d}
}

// codexWritableRoot resolves one writable root to its real path, because
// codex refuses a root with a symlink component — and refuses it at
// COMMAND-RUN time, not at launch. That is the whole failure mode
// (ranger-base-c02a, measured live on codex-cli 0.150.1): the session comes
// up, herdr calls it working and then idle, and every tool call in it dies
// with
//
//	Error: writable root /Users/.../.config/posse/personas/developer contains
//	symlink component /Users/.../.config/posse/personas; symlinked writable
//	roots are not supported
//
// so the persona reads its prompt and can then run nothing at all — no bd,
// no file, no commit. On this box ~/.config/posse/personas is a symlink into
// the constitution tree, which made EVERY dispatched codex session dead on
// arrival and silently so, since dispatch has no turn-outcome reader for
// codex to tell "did nothing" from "did the work" (cost.go).
//
// Resolving costs the session nothing: with the REAL path granted, a write
// through the symlinked spelling still lands (measured with `codex sandbox`,
// same bead), so RHQ_PERSONA_DIR/POSSE_PERSONA_DIR and {memory} can go on naming the path the
// operator typed. Every root is resolved, not just the persona dir — the
// others (the store of record, the git dirs) are real paths on this box
// today, which is why only codex's memory dir broke, and the next symlinked
// constitution path would kill the lane the same silent way.
//
// resolveExisting is the primitive because a root need not exist yet — the
// store's git dirs and the memory dir are named before the launch
// materializes them, and the seatbelt already resolves the longest existing
// prefix for the same reason (seatbelt.go). A root that resolves to nothing
// at all renders as itself rather than dropping: a dropped root is the
// silent cage of ranger-base-0fb, and a bad path is codex's to report out
// loud, not ours to hide.
func codexWritableRoot(p string) string {
	if p == "" {
		return ""
	}
	return resolveExisting(p)
}

// symlinkComponent returns the DEEPEST component of p that is a symlink, or
// "" when none of them is — the question codex asks of every writable root
// before it applies its sandbox, answered the way its own refusal answers it
// (the message names the COMPONENT, not the root).
//
// Deepest and not outermost, because that is the one codex names — measured
// 2026-09-05 on codex-cli 0.150.1 over three roots, l1 being a symlink to a
// real dir:
//
//	l1/sub  (sub a real dir)     → symlink component …/l1
//	l1/l2   (l2 a symlink)       → symlink component …/l1/l2
//	l1/d    (d dangling)         → symlink component …/l1/d
//
// A root posse RENDERS can only ever carry one: everything above the first
// component EvalSymlinks fails on is replaced by its real path, and nothing
// below it exists to be a link. The order is what a line posse did not
// render — a PID's own command: — is owed, and matching codex's own message
// is what makes the two reports name the same file.
//
// Lstat, never EvalSymlinks: the components this exists to catch are
// exactly the ones EvalSymlinks cannot resolve. A component that does not
// exist is not a symlink and does not end the walk — codex refuses on the
// link whether or not anything below it exists (measured, refusedWritableRoot
// below).
//
// A relative path is not judged: the roots posse renders are absolute, and
// a relative one would be resolved against THIS process's directory, which
// is not the session's — a wrong answer is worse here than no answer.
func symlinkComponent(p string) string {
	if p == "" || !filepath.IsAbs(p) {
		return ""
	}
	var parts []string
	for cur := filepath.Clean(p); ; {
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		parts = append(parts, cur)
		cur = parent
	}
	// parts is deepest-first already, so the first hit is the component
	// codex's own message names.
	for _, c := range parts {
		if st, err := os.Lstat(c); err == nil && st.Mode()&os.ModeSymlink != 0 {
			return c
		}
	}
	return ""
}

// refusedWritableRoot names the first writable root on a rendered
// self-sandboxing launch line that the runtime will refuse outright, and
// the component that makes it refuse. Two empty strings is a line whose
// roots are all acceptable.
//
// This is codexWritableRoot's RESIDUAL, and the whole of ranger-base-k62e.
// resolveExisting resolves the longest EXISTING prefix and re-joins the
// rest, so a component EvalSymlinks cannot resolve — a DANGLING link, a
// loop, a parent that cannot be read — is walked past and re-joined
// verbatim, and the rendered root still carries the symlink component codex
// refuses. The fallback itself is right: dropping the root is
// ranger-base-0fb's silent cage, and the code comment above says so. What
// it leaves behind is a line nobody may launch in silence.
//
// MEASURED 2026-09-05, codex-cli 0.150.1, over a dangling link (d/dangle ->
// d/nope, which is not there):
//
//	writable_roots=["…/dangle"]              Error: writable root …/dangle contains
//	writable_roots=["…/dangle/sub"]          symlink component …/dangle; symlinked
//	                                         writable roots are not supported
//	writable_roots=["…/real", "…/dangle"]    the same error, exit 1
//
// so a dangling root is refused exactly like the resolvable symlink
// ranger-base-c02a measured, final component or not — and ONE bad root
// refuses the whole set, before any sandbox is applied. The session that
// would open runs no command at all: not a weaker session, a dead one,
// which is why the launch sites treat this like PIDVoided (a refusal) and
// not like a degrade.
//
// Asked of the LINE and not of the roots handed to Realize, for the reason
// ADR 0053 D1's {model} check is: what reaches the line is not what was
// passed in. `-s read-only` renders no roots at all (rangerhq-5oi), and a
// PID's own command: renders whatever it was written to render.
func refusedWritableRoot(cmd string) (root, component string) {
	toks := shellTokens(cmd)
	for i := 0; i+1 < len(toks); i++ {
		if toks[i] != "--add-dir" {
			continue
		}
		if c := symlinkComponent(toks[i+1]); c != "" {
			return toks[i+1], c
		}
	}
	return "", ""
}

// writableRootRefusal is that check at a launch site: nil unless this
// runtime cages itself with the roots on its own line and one of them is a
// root it will refuse.
//
// Gated on SelfSandbox because the refusal is a property of the CLI, not of
// the flag — claude takes --add-dir too and does not care what the path is
// made of — and SelfSandbox is already the discriminator this codebase uses
// for "the roots on this line ARE this session's write wall"
// (applyRecordReach, reachability.go).
//
// A refusal and not a warning, and this is shape (b) of ranger-base-k62e's
// three: the launch that would happen can run nothing, and the report codex
// makes arrives at COMMAND-RUN time inside a session no one reads — which is
// how ranger-base-c02a cost five dispatched sessions and a P1. A warning
// would land in the dispatch log, one layer closer than the session, and
// still open a dead pane herdr calls working and then idle.
func writableRootRefusal(agent string, rt *Runtime, cmd string) error {
	if rt == nil || !rt.SelfSandbox {
		return nil
	}
	root, comp := refusedWritableRoot(cmd)
	if root == "" {
		return nil
	}
	return Die("%s: the rendered %s launch line names writable root %s, whose component %s is a SYMLINK — %s refuses a writable root with a symlink component BEFORE it applies its sandbox, so the session would open, read its prompt, and then fail every single command it runs (measured on codex-cli 0.150.1; ranger-base-k62e, and ranger-base-c02a is what that silence cost last time)\n"+
		"  posse resolves every root it can and renders the rest as typed on purpose (dropping one is the silent cage of ranger-base-0fb), so this component is one nothing could resolve — a dangling link or a loop\n"+
		"  make %s resolve — restore what it points at, or repoint it — and this launch renders the real path by itself",
		agent, rt.Name, AbbrevHome(root), AbbrevHome(comp), rt.Name, AbbrevHome(comp))
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
// CLI/API naming).
var claudeModels = map[string]string{
	TierStrong:   "claude-fable-5-1",
	TierStandard: "claude-opus-5",
	TierFast:     "claude-sonnet-5",
}

// Codex model ids per tier. This map is the fleet's cost/quality dial on
// codex, and before it existed `tier:` was INERT there: the built-in
// carried no Models at all, so rt.Model(tier) returned "", {model}
// rendered empty and the CLI picked whatever it defaults to — a PID
// saying `tier: strong` on codex got no guarantee and no warning
// (ranger-base-arm). The ModelFlag was already right (`-c model=%s`); only
// the map was missing.
//
// The two ids are what live sessions on this box show, not what a doc
// claims (measured 2026-08-25):
//
//   - gpt-5.6-sol — what a codex session here defaults to; footer
//     "gpt-5.6-sol xhigh".
//   - gpt-5.6-luna — the cheaper one codex itself offers when the account
//     approaches its limits, "Fast and affordable agentic coding model";
//     footer "gpt-5.6-luna medium". The operator authorised it as an
//     option on 2026-08-25.
//
// So: strong and standard both name sol, fast names luna.
//
//   - strong == standard is not a copy-paste. codex offers nothing above
//     sol here, and the honest mapping for "judged work" is the best id
//     that exists rather than a name with nothing behind it. Naming it on
//     both makes the launch a FACT instead of a CLI default that can move
//     under us between releases — the same argument ClaudeFleetFlags makes
//     for --permission-mode.
//   - fast is the cost lever, and only that. MEASURED the same day:
//     switching a session to luna did NOT lift an account-level usage wall
//     — the wall is on the ACCOUNT, not on the model. Nothing here may be
//     read as buying headroom; dispatch's budget step-down (standard →
//     fast, dispatch.go) now buys a cheaper model on codex and no more
//     allotment than before.
//
// Reasoning effort is NOT part of this: the footers differ (xhigh vs
// medium) because those are the two models' own defaults, and {model}
// renders a model id. A per-tier effort would be a second key nobody has
// measured yet.
var codexModels = map[string]string{
	TierStrong:   "gpt-5.6-sol",
	TierStandard: "gpt-5.6-sol",
	TierFast:     "gpt-5.6-luna",
}

// Grok model ids per tier. Same defect as codex before ranger-base-arm and
// the same fix: the grok built-in already carried ModelFlag "-m %s" and
// {model} in its template, but no Models map — so `tier:` was INERT on
// grok, {model} rendered empty, and the CLI picked its own model with no
// warning anywhere (rangerhq-jp6).
//
// The two ids are what this box serves TODAY, read from the CLI rather
// than from a doc (2026-08-29, grok 1.0.5, `grok models` and
// ~/.grok/models_cache.json fetched the same morning — both agree):
//
//   - grok-4.6 — "Default model: grok-4.6", starred `(default)` in the
//     listing; 500K context.
//   - grok-4.5 — the previous frontier model, still served; 500K context.
//
// Both come from cli-chat-proxy.grok.com under the subscription session,
// not an API key, and both advertise supports_reasoning_effort — but NOT
// the same efforts: grok-4.6 offers xhigh/high/medium/low and grok-4.5
// only high/medium/low, both defaulting to high (measured 2026-08-29,
// models_cache.json). Take the ids from the CLI when you touch this map
// again: it self-updates, and it moved 1.0.0 → 1.0.5 in the middle of one
// verification (rangerhq-vjl).
//
// So: strong and standard both name grok-4.6, fast names grok-4.5. This is
// the codex shape and it is DELIBERATE — rangerhq-jp6 asked for
// standard=grok-4.5, and the reason it is not:
//
//   - Nothing on this box prices a grok model against the WEEKLY POOL.
//     xAI publishes no usage endpoint (grokpool.go's whole header), the
//     pool is a compute allowance with no published per-model rate, and
//     grok-4.5 has never run here at all — 181 of 181 priced turns across
//     174 transcripts in ~/.grok/sessions carry "modelId":"grok-4.6"
//     (measured 2026-08-29). So "4.5 is cheaper" is not a small number, it
//     is NO number, and a map cannot be justified by it in either
//     direction. What is left is capability, and 4.6 is the frontier one.
//   - standard=grok-4.5 would also make the map's only real lever inert.
//     fast falls back to standard when unmapped, so that map renders the
//     SAME id for both, and dispatch's budget step-down (standard → fast)
//     would silently buy nothing. Naming 4.5 on `fast` is the one place
//     the step-down changes anything.
//   - It also keeps the launch a FACT rather than a CLI default that can
//     move between releases — the argument codexModels makes above, and
//     the same one ClaudeFleetFlags makes for --permission-mode. What a
//     grok session runs here today IS grok-4.6; a map that quietly moved
//     ordinary `standard` work onto the older model would be a behaviour
//     change nobody asked for.
//
// Read `fast` here as a CAPABILITY step-down, never a measured cost one.
// codex may say fast=luna is "the cost lever" because somebody measured
// luna's price; nobody has measured grok-4.5's, so nothing here claims a
// saving and NOTES.md says so in the same words.
//
// Reasoning effort is NOT in this map, and that is now RULED rather than
// deferred (ranger-base-tg7c, ADR 0003 §1 amendment 2026-08-29): nothing
// on this box can price an effort step against the weekly pool, the two
// models do not even offer the same efforts (above), and {model} renders
// one argv token via ModelFlag — so a per-tier effort is not one key but
// a tier→model × model→efforts validity matrix plus a second placeholder.
// Both models default to `high`, so leaving it unset runs each at its own
// default, and a PID or declared runtime that wants a different one can
// append --reasoning-effort to its own command: today. The revival
// condition is a MEASURED pool cost per effort step; until then, do not
// smuggle one in through this map's values — `"grok-4.6
// --reasoning-effort low"` is not a model id and would make every reader
// of this map wrong.
//
// The SHAPE above is ruled too, in the same amendment: strong = standard =
// grok-4.6 with fast = grok-4.5 stands over the map rangerhq-jp6
// originally asked for.
var grokModels = map[string]string{
	TierStrong:   "grok-4.6",
	TierStandard: "grok-4.6",
	TierFast:     "grok-4.5",
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

// ClaudeProjectConfig is the project settings file in which a repo declares
// hooks and MCP servers to claude. SeedClaudeTrust is what makes its hooks
// live for a posse LAUNCH, which is interactive; trust is not the only gate
// on them and is not one at all headless (measured, ranger-base-i0s8 — see
// the header of trust.go).
const ClaudeProjectConfig = ".claude/settings.json"

// ClaudeProjectConfigLocal is the second half of that same project scope:
// claude's local settings file, gitignored by convention and read out of the
// session dir exactly like its shared sibling.
//
// MEASURED on claude 2.1.251 (rangerhq-9u8), scratch dirs, no API turn, each
// arm with a fresh CLAUDE_CONFIG_DIR and ANTHROPIC_BASE_URL pointed at a dead
// port so nothing could reach the API: a `SessionStart` hook declared in
// .claude/settings.local.json ran — before the first turn, and before the CLI
// even resolved credentials (the run ended on "Not logged in · Please run
// /login" with the hook's witness file already written). It ran identically
// whether the fleet's own `--settings` JSON was passed or not: `--settings`
// adds a source, it does not replace the project's, so ClaudeFleetSettings
// suppresses nothing. The negative arm — the same dirs with no settings file
// — wrote no witness, so the rig discriminates.
//
// Gitignored is not a security property: an attacker who can write the repo
// can write this path too, and there it is invisible to `git status`. Same
// class, same check.
const ClaudeProjectConfigLocal = ".claude/settings.local.json"

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
// declares them and rewrites none of them: the work prompt's `guardrails:`
// line is the reconciliation — it names no source as its boundary precisely
// so a native rulebook, and bd prime's injected checklist, are both inside
// it — and whether such a file actually outranks the PID is a PROBE, not a
// patch. A runtime
// that fails that probe stays record: untrusted.
//
// Sources, so nobody re-derives them. Every line here is what the
// INSTALLED binary was measured to load, not what its docs or its own
// strings say it loads — the two came apart on claude (ranger-base-x7m1):
//   - claude 2.1.251, MEASURED: a tool-less `claude -p` turn in a fresh
//     repo carrying one token per candidate file returns CLAUDE.md,
//     CLAUDE.local.md, .claude/CLAUDE.md and .claude/rules/*.md. It does
//     NOT return AGENTS.md, which this list declared until x7m1, and with
//     AGENTS.md alone in the directory the same turn answers
//     NO-PROJECT-INSTRUCTIONS. The binary does still carry the string
//     "Claude Code hardcodes CLAUDE.md / AGENTS.md discovery" (in the codex
//     importer's unmappable reasons) — that string was the old source here,
//     and a string is not a measurement.
//   - codex-cli 0.150.1: AGENTS.md and AGENTS.override.md, project and
//     ~/.codex/; the set is widened by config project_doc_fallback_filenames.
//     AGENTS.override.md REPLACES the AGENTS.md beside it rather than adding
//     to it (ADR 0013 rules-precedence probe §4, re-measured x7m1); the line
//     comma-joins them anyway, and the replace is pinned in
//     TestQALiveNativeRulesDiscovery/codex-override rather than spelled in
//     the grid. ranger-base-s25o weighed saying it there: embedding it in
//     this slice breaks the field's contract (a bare filename — this same
//     slice is what an operator's own native_rules: fills, and what
//     TestQANativeRulesDeclarationMatchesMeasurement folds case and compares
//     verbatim against measurement); a separate declarable key is the
//     cleaner shape but is six touch points and the grid's first
//     unverifiable free-text fact, an ADR 0013 §4 grid-shape call left for
//     the architecture persona. Closed without rendering it — this comment
//     and the red-on-drift test are what holds the fact meanwhile.
//   - grok 1.0.5, docs/user-guide/12-project-rules.md, in its own order,
//     every match in a directory loaded (not first-wins), plus *.md under
//     the rules dirs at each level from repo root to cwd. .claude/CLAUDE.md
//     is MEASURED and undocumented: `grok inspect` names it (x7m1), and it
//     is not the .claude/rules/*.md entry beside it.
//
// GrokPIDVoid is grok's PIDVoid set: the flag that replaces the system
// prompt, and the compat alias its own --help names for it. Both spellings
// were measured on 1.0.5 — see Runtime.PIDVoid for the numbers. The alias
// is listed because a check that knew only the canonical spelling would
// pass a line that voids the PID exactly as thoroughly.
var GrokPIDVoid = []string{"--system-prompt-override", "--system-prompt"}

var (
	claudeNativeRules = []string{"CLAUDE.md", "CLAUDE.local.md", ".claude/CLAUDE.md", ".claude/rules/*.md"}
	codexNativeRules  = []string{"AGENTS.md", "AGENTS.override.md", "~/.codex/AGENTS.md"}
	grokNativeRules   = []string{"Agents.md", "Claude.md", "CLAUDE.md", "CLAUDE.local.md", "AGENT.md", "AGENTS.md",
		".claude/CLAUDE.md", ".grok/rules/*.md", ".claude/rules/*.md", ".cursor/rules/*.md", "~/.grok/rules/*.md"}
)

// PROMPT DELIVERY, AS THE PROBE MEASURED IT (ADR 0013 §2, ranger-base-cl7,
// full trace in docs/adr/0013-argv-prompt-probe.md). The ADR's table read
// "grok/codex argv *if the probe holds*, else typed plus a measured wait".
// It held, on both, on 2026-08-25:
//
//	codex 0.147.0  the Update-available banner draws and does not wait for
//	               a selection; the positional prompt is the first user
//	               turn and the screen goes `working`
//	               (matched_rule screen_working_fallback)
//	grok 1.0.5     the New-worktree/Resume splash clears with no keystroke
//	               from posse; the positional prompt renders as turn #1 and
//	               the screen goes `working` (matched_rule
//	               spinner_status_working)
//
// So both are PromptArgv here, and neither carries a StartupWait: a typed
// fallback is what a measured wait would be *for*, and there is no typed
// fallback to measure. On grok there could not be one — a pane that has not
// had a turn matches no rule at all, so waiting longer produces no screen
// (`agent explain`, ranger-base-3j8/cl7). Only a turn does, and
// argv is what starts one.
//
// claude stays typed: it works, and re-testing a live path for symmetry is
// a change with no measurement behind it (ADR 0013 Consequences: "argv is
// an allowed later unify"). ranger-base-dg5 is the dispatch half that reads
// this column.
var builtinRuntimes = []Runtime{
	{Name: "claude", Builtin: true, Realize: realizeClaude, Skills: skillsClaude, Models: claudeModels, ModelFlag: "--model %s", ProjectConfig: []string{ClaudeProjectConfig, ClaudeProjectConfigLocal}, ProjectConfigKeys: []string{"hooks", "mcpServers"}, Unattended: ClaudeFleetFlags,
		Egress: []string{"api.anthropic.com", "platform.claude.com"}, Interstitials: ClaudeInterstitials,
		// CredBin: the binary this CLI reads and writes its own
		// OAuth credential with on darwin — measured on the release artifact,
		// ranger-base-eupf. A PID denying it gates the runtime, not the persona.
		CredBin: "security",
		// state_dir, declared rather than listed in the seatbelt builder:
		// ~/.claude is the CLI's own tree and ~/.claude.json is a FILE, not a
		// directory — the same grant either way, and both were literals in
		// SeatbeltWritable until ADR 0012 D4 made the key declarable.
		StateDirs: []string{"~/.claude", "~/.claude.json"},
		// record: trusted — dispatched claude sessions close their beads;
		// that is the shape every other runtime is measured against.
		// account: nothing to declare — the adapter registry is the whole
		// declaration (cost_claude.go), and claude is COUNTED because that
		// adapter prices what it reads.
		Prompt: PromptTyped, Record: RecordTrusted, RecordWhy: "dispatched sessions close their beads; the baseline the contract was written from",
		FleetSettings: ClaudeFleetSettingsJSON, SettingsPin: credentialDirPinJSON,
		NativeRules: claudeNativeRules,
		// rules_precedence: UNMEASURED, deliberately. ranger-base-6rcv's
		// billed measurement was authorized for codex and grok only (no
		// claude turn), so the grid keeps saying so out loud rather than
		// borrowing their answer. The one claude datapoint on the books is
		// an incidental live collision (rangerhq-cmfj, ADR 0013 Claims),
		// which is a report, not a measurement under a controlled fixture.
		// turn_outcome: the same transcript, read for a different fact —
		// claude writes an allotment refusal as a synthetic assistant
		// message, so a pass can tell an exhausted account from a settle.
		// The only runtime with a reader today (ranger-base-02zr).
		TurnOutcomeAdapter: TurnOutcomeClaudeTranscript,
		// pane_mode: the footer names all six modes (MEASURED on claude
		// 2.1.251, permissionmodepane_qa_test.go), and a modal dialog
		// replaces it — which is the ?covered state, not a mode.
		PaneModeAdapter: PaneModeClaudeFooter,
		Command:         `claude {model} ` + ClaudeFleetFlags + ` --append-system-prompt "$(cat {file})" --add-dir {memory} {settings} {skills} {allow} {deny}`},
	{Name: "codex", Builtin: true, Realize: realizeCodex, Skills: skillsCwd, SkillsCwd: true, Models: codexModels, ModelFlag: "-c model=%s", SelfSandbox: true, ProjectConfig: []string{CodexProjectConfig}, Unattended: "-a never",
		Egress: []string{"chatgpt.com", "ab.chatgpt.com"}, StateDirs: []string{"~/.codex"},
		// record: untrusted — MEASURED the other way: 3/3 dispatched codex
		// sessions did the work and left the bead in_progress with no
		// comment, one of them on a dirty shared checkout (ranger-base-0fb).
		// Dispatch still launches; gather never ✓s a settle-without-close.
		Prompt: PromptArgv, Record: RecordUntrusted,
		// pane_mode: none — MEASURED on codex-cli 0.150.1, the footer is
		// `<model> <effort> · <cwd>` under `-a never` and `-a on-request`
		// alike and the policy is reachable in-pane only by typing /status.
		// Typing into a session is not a read, so this is a permanent `—`
		// rather than an unknown more work could close, and a listing spends
		// no herdr call per codex session to re-learn it.
		PaneModeAdapter: PaneModeNone,
		NativeRules:     codexNativeRules, Interstitials: CodexInterstitials,
		// rules_precedence: pid — MEASURED behaviourally on 2026-09-01
		// (ranger-base-6rcv, one billed turn under the operator's
		// authorization ff9pz), not inferred from xaev's placement matrix.
		// Codex's own reply named neither token, so the verdict is a
		// two-signal read of the other rules; the trace spells it out
		// (docs/adr/0013-rules-precedence-probe.md, "Behavioural
		// measurement 2026-09-01").
		RulesPrecedence:    RulesPrecedencePID,
		RulesPrecedenceWhy: "measured 2026-09-01 (ranger-base-6rcv): against a fixture AGENTS.md demanding lowercase, the word 'prepared' and its own token, codex 0.150.1 replied 'READY.' — all three AGENTS rules broken, the PID's two decidable rules obeyed and neither token emitted; both rulebooks were in the rendered prompt (ranger-base-kl58b)",
		Command:            `codex {model} {skills} {deny} -a never ` + CodexFleetFlags + ` -c developer_instructions="$(cat {file})"`},
	{Name: "grok", Builtin: true, Realize: realizeGrok, Skills: skillsCwd, SkillsCwd: true, Models: grokModels, ModelFlag: "-m %s", Unattended: GrokFleetFlags,
		Egress: []string{"cli-chat-proxy.grok.com", "grok.com"}, StateDirs: []string{"~/.grok"},
		// record: trusted — the qa lane on grok closed a bead properly on
		// 2026-08-24, which is the measurement the promotion needs and the
		// only reason grok and codex differ in this column.
		//
		// StartupWait stays UNSET, and after the probe that is an answer
		// rather than a deferral: it is the `prompt: typed` ladder's
		// patience, and grok does not use that ladder any more. Its cold
		// start exceeding 45s on a clean screen (ranger-base-3j8) was a
		// problem about reaching a composer; argv needs no composer.
		Prompt: PromptArgv, Record: RecordTrusted, RecordWhy: "the qa lane closed a dispatched bead on 2026-08-24 (ADR 0013 §4)",
		// pane_mode: the composer border names two of six modes (MEASURED on
		// grok 1.0.5); the other four and the startup splash render nothing,
		// which is ?unnamed and never "default".
		PaneModeAdapter: PaneModeGrokBorder,
		// turn_outcome: NOT the transcript — a refused turn leaves grok's
		// chat_history.jsonl silent, and the failure is typed JSON in the
		// session store beside it (turnfailure_grok.go). Promoted on the
		// captured artifact, never a guessed one: ADR 0013 §1's rule, and
		// docs/adr/0013-turn-outcome-refusal-probe.md is the capture.
		TurnOutcomeAdapter: TurnOutcomeGrokSessionStore,
		NativeRules:        grokNativeRules, Interstitials: GrokInterstitials, PIDVoid: GrokPIDVoid,
		// rules_precedence: pid — MEASURED behaviourally on 2026-09-01
		// (ranger-base-6rcv), the half xaev's structural probe could not
		// reach at all on this runtime: `agents_md_files` lands where
		// grok's own harness decides, so only a turn could answer it.
		// Grok's reply emitted the PID's own token, so it is
		// self-evidencing.
		RulesPrecedence:    RulesPrecedencePID,
		RulesPrecedenceWhy: "measured 2026-09-01 (ranger-base-6rcv): against the same fixture AGENTS.md, grok 1.0.5 replied 'READY / PID-WINS' — the PID's own token, all three AGENTS rules broken; both rulebooks were in the rendered prompt (ranger-base-kl58b)",
		Command:            `grok {model} {skills} ` + GrokFleetFlags + ` --rules="$(cat {file})" {allow} {deny}`},
}

// RuntimesDir holds template-only runtimes: RHQ_HOME/runtimes/<name>.yaml.
func (a *App) RuntimesDir() string { return filepath.Join(a.Home, "runtimes") }

// LoadRuntime returns a built-in by name — per-key overlaid from
// RHQ_HOME/runtimes/<name>.yaml if that file exists (ADR 0021) — else a
// template-only runtime from the same file.
func (a *App) LoadRuntime(name string) (*Runtime, error) {
	if name == "" {
		name = DefaultRuntime
	}
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Name == name {
			rt := builtinRuntimes[i]
			return a.overlayBuiltin(&rt, name)
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
		form, err := printfFlag("model_flag", f)
		if err != nil {
			return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
		}
		rt.ModelFlag = form
	}
	// skills_cwd: this runtime discovers skills from the session's working
	// directory instead of from a flag — the codex/grok shape, which a yaml
	// runtime could not declare at all: it was skills_flag: or no surface.
	// The launch materializes <cwd>/.agents/skills/<name> and {skills}
	// renders nothing; the links ARE the realization (skills.go).
	skillsCwdDecl, err := runtimeBool(p, "skills_cwd")
	if err != nil {
		return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
	}
	// skills_flag: the printf form {skills} renders with, given the rendered
	// skills dir ("--plugin-dir %s", or a glued "--skills=%s"). Absent →
	// this runtime has no skill surface and a PID that binds skills cannot
	// launch on it.
	flagDecl := YamlGet(p, "skills_flag")
	if flagDecl != "" && skillsCwdDecl {
		// Two half-bindings at once: the flag would point at the rendered
		// plugin tree while the links point at the session dir, and neither
		// the parity line nor `runtime check` could say which one the CLI
		// actually read. ADR 0021 rejected the same shape for built-ins.
		return nil, Die("runtime %s: %s declares both skills_flag: and skills_cwd: — a runtime has one skill surface; keep the one you measured", name, AbbrevHome(p))
	}
	switch {
	case skillsCwdDecl:
		rt.SkillsCwd = true
		rt.Skills = skillsCwd
	case flagDecl != "":
		form, err := printfFlag("skills_flag", flagDecl)
		if err != nil {
			return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
		}
		rt.Skills = func(dir string, names []string) (string, bool) {
			if len(names) == 0 {
				return "", true
			}
			return fmt.Sprintf(form, shellQuote(dir)), true
		}
	}
	// self_sandbox: this runtime wraps its own child commands in a sandbox,
	// so ours cannot wrap it — macOS refuses to nest seatbelts. Undeclarable
	// until now, which made a self-sandboxing yaml runtime broken in a way
	// nothing could say out loud: the launch wrapped it anyway (herdrback.go)
	// and the parity matrix claimed an L2 it did not have. Declaring it costs
	// the seatbelt tier honestly — CheckParity degrades cage: seatbelt here
	// and names the nesting refusal.
	if v, err := runtimeBool(p, "self_sandbox"); err != nil {
		return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
	} else if v {
		rt.SelfSandbox = true
	}
	// project_config: a file IN THE SESSION DIRECTORY this runtime reads as
	// configuration because posse made that directory trusted — a channel
	// from the repo to the box that no model and no PID sits in front of.
	// Safety-relevant and the reason this key is not cosmetic: undeclared,
	// parity's trust check (ProjectConfigTrust) silently skips it, so an
	// unguarded repo→box channel reads as a clean launch. Declared, the
	// launch degrades unless the PID sets trust_project_config: true.
	//
	if v := YamlGet(p, "project_config"); v != "" {
		rel, err := runtimeRelPath("project_config", v)
		if err != nil {
			return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
		}
		rt.ProjectConfig = []string{rel}
	}
	// project_config_keys: the JSON-key narrowing claude's built-in uses —
	// only a file naming one of these top-level keys degrades the launch,
	// instead of the file's mere presence.
	//
	// This is the one declarable key that LOOSENS a safety check, so it
	// refuses in two directions rather than one. Alone it names no file to
	// narrow, which would be a lever born inert (the ADR 0017 §3 class);
	// present-but-empty is the same declaration wearing a value. Both
	// refuse, and both refusals name removing the key as the way back to the
	// conservative whole-file predicate.
	//
	// Loosening is only honest over a config whose keys the onboarder has
	// READ, and the check keeps its own floor either way: a keyed file that
	// is not a readable top-level JSON object fails closed (parity.go
	// projectConfigTrustFile), so keys declared over a TOML config degrade
	// every launch rather than narrowing anything.
	if yamlHasKey(p, "project_config_keys") {
		keys := YamlList(p, "project_config_keys")
		switch {
		case len(rt.ProjectConfig) == 0:
			return nil, Die("runtime %s: %s declares project_config_keys: without project_config: — there is no session-dir file to narrow, so the keys are read by nothing", name, AbbrevHome(p))
		case len(keys) == 0:
			return nil, Die("runtime %s: %s declares an empty project_config_keys: — name the top-level keys that carry executable configuration, or remove the key to check the whole file's presence (the conservative side)", name, AbbrevHome(p))
		}
		rt.ProjectConfigKeys = keys
	}
	// unattended: the flag that makes this CLI approve a tool call with
	// nobody watching. Declarable separately from command: because it is a
	// launch GUARANTEE and not a template detail — EnsureUnattended puts it
	// back on a line a PID's hand-written command: rendered without it, and
	// `runtime check`'s launch row is what an onboarder reads it off.
	//
	// A built-in's flag is posse's measurement; a yaml runtime's is the
	// OPERATOR's, which is the only reason appending it is safe. Absent
	// stays loud rather than guessed: posse knows no unmeasured CLI's
	// dialect, and the launch row says "NO unattended flag known".
	if v := YamlGet(p, "unattended"); v != "" {
		flag, err := runtimeFlagValue("unattended", v)
		if err != nil {
			return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
		}
		rt.Unattended = flag
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
	//
	// Not read through runtimeBool: a misspelled value here leaves the gate
	// shell ON, which is the safe direction (more wall, not less), so
	// tightening it would refuse launches to prevent nothing. The keys that
	// DO refuse on a bad value are the ones whose wrong reading costs a gate.
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
		why := YamlGet(p, "record_why")
		if err := validateRecordDecl(v, why); err != nil {
			return nil, Die("runtime %s: %s %v", name, AbbrevHome(p), err)
		}
		rt.Record = v
		rt.RecordWhy = why
	}
	// rules_precedence: which channel wins when a native rulebook collides
	// with the PID. Absent is UNMEASURED, the loud default — a probe, not a
	// patch (ADR 0017 §5). Present-but-wrong refuses, like record: above.
	if v := YamlGet(p, "rules_precedence"); v != "" {
		if !ValidRulesPrecedence(v) {
			return nil, Die("runtime %s: %s has rules_precedence: %q (want %s or %s — ADR 0017 §5)", name, AbbrevHome(p), v, RulesPrecedencePID, RulesPrecedenceNative)
		}
		rt.RulesPrecedence = v
		rt.RulesPrecedenceWhy = YamlGet(p, "rules_precedence_why")
	}
	// turn_outcome: which registered reader sees this runtime's own first
	// turn. Absent is the loud default — the settle line says posse cannot
	// tell a refused turn from a worked one here. Present-but-unregistered
	// refuses rather than degrading quietly: a yaml that names a reading
	// nobody performs is a promise the pass would silently break.
	if v := YamlGet(p, "turn_outcome"); v != "" {
		if turnOutcomeReaders[v] == nil {
			return nil, Die("runtime %s: %s has turn_outcome: %q — no reader by that name (have: %s; ADR 0012 D4)",
				name, AbbrevHome(p), v, strings.Join(TurnOutcomeAdapters(), ", "))
		}
		rt.TurnOutcomeAdapter = v
	}
	// pane_mode: which registered reader parses this runtime's pane for the
	// mode a session is actually in. Same shape, same reason and the same two
	// arms as turn_outcome: above — absent is the loud default (`mode:?` with
	// a why naming this runtime), and a name no reader implements refuses at
	// load rather than promising a reading nothing performs. `none` is a
	// registered reader, so declaring a measured blindness goes through this
	// arm like any other (ADR 0057 D1).
	if v := YamlGet(p, "pane_mode"); v != "" {
		if _, ok := PaneModeReaderFor(v); !ok {
			return nil, Die("runtime %s: %s has pane_mode: %q — no reader by that name (have: %s; ADR 0057 D1)",
				name, AbbrevHome(p), v, strings.Join(PaneModeAdapters(), ", "))
		}
		rt.PaneModeAdapter = v
	}
	// native_rules: the rulebook files this CLI loads on its own. Posse
	// never writes them; declaring them is how `runtime check` can name the
	// other voice in the session.
	rt.NativeRules = YamlList(p, "native_rules")
	// --- the preflight keys (ADR 0012 D4, rangerhq-tr8k) ---
	//
	// state_dir: where this CLI keeps its own state. It joins the L2
	// seatbelt's writable set, which carried `~/.claude ~/.codex ~/.grok` as
	// a literal — so a third-party CLI under `cage: seatbelt` got a
	// read-only state dir and re-ran its first-run flow, or died on a config
	// write, with nothing in the launch saying why.
	if v, err := runtimeStateDirs(p); err != nil {
		return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
	} else {
		rt.StateDirs = v
	}
	// env_required: the variable NAMES a session here cannot work without.
	// The Bedrock case — claude installed, on PATH, every launch a dead pane
	// because AWS_REGION was not in the session env — was expressible only
	// as tribal knowledge in a PID's `envs:` list. Names only; the validator
	// refuses anything with a value in it.
	if v, err := runtimeEnvRequired(p); err != nil {
		return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
	} else {
		rt.EnvRequired = v
	}
	// interstitial_<slug>: the first-run screens a fresh pane of this CLI
	// opens on, and the operator-owned key that silences each. Declared, not
	// pressed: rangerhq-6723 retired the launcher's one keystroke table and
	// the rangerhq-4mzt ruling is that no drawn dialog is the
	// launcher's to answer, so what a third party can declare here is the
	// DISMISSAL — the file and the key — and never a key to send.
	if v, err := declaredInterstitials(p); err != nil {
		return nil, Die("runtime %s: %s %v", name, AbbrevHome(p), err)
	} else {
		rt.Interstitials = v
	}
	// UNKNOWN is the third reading, and until now it was silence: a key
	// nothing recognized was dropped without a word, so `skils_flag:` is a
	// persona that cannot launch and `slef_sandbox:` is a seatbelt that
	// refuses to nest — a dead wall under a config file that looks right.
	// Warned rather than refused (runtimeyaml.go).
	warnUnknownRuntimeKeys(runtimeNoticeWriter, name, p)
	return rt, nil
}

// builtinOverlayKeys are the ADR 0021 Decision 1 keys — the ONLY ones a
// runtimes/<name>.yaml may overlay onto a built-in. Each names a MEASURED
// instance fact (a model id, a wait, a promotion, where this box's CLI
// keeps its state); nothing that changes the launch MECHANISM is here
// (that split is Decision 2, refused below). Named once so the refusal
// message and the overlay code cannot drift apart.
//
// Spelled as KEYS rather than as display text, because the list now has a
// second reader: a pin walks runtimeYamlKeys() and finds every declarable
// key on exactly one side of the split, so the next key someone adds to
// the declarable surface cannot arrive unclassified — which is how the
// four below spent a release being read by nothing (ranger-base-otoq8).
// model_<tier> is expanded from Tiers for the same reason it is in
// runtimeYamlKeys(): a new tier is not a new decision.
var builtinOverlayKeys = []string{
	"model_flag", "prompt", "startup_wait", "record", "record_why",
	"native_rules", "egress", "cage_cred", "gate_shell",
	// Declarable since ADR 0012 D4, and until ranger-base-otoq8 dropped in
	// silence here: warnUnknownRuntimeKeys knows them, so an overlay
	// declaring one loaded clean and changed nothing. Each is an instance
	// fact by D1's own line — which channel wins on THIS box's CLI, where
	// that CLI keeps its state, what its session env must carry — so each
	// overlays.
	"rules_precedence", "rules_precedence_why", "state_dir", "env_required",
}

// builtinMechanismKeys are the ADR 0021 Decision 2 keys: declared in a
// built-in's overlay they REFUSE the load, naming the fact/mechanism
// split. Each changes what a launch DOES rather than naming something this
// box measured, and the built-in's own answer is one posse measured and
// pinned — a yaml that moves it is a launch nobody measured, which is the
// whole of D2.
//
// The why travels with the key so the refusal says what the key would have
// changed, not just that it is on a list.
var builtinMechanismKeys = []struct{ key, why string }{
	// A hand-written template wearing a built-in's measured realizer and
	// EnsureUnattended is a launch nobody measured. The per-persona PID's
	// own command: hatch already covers a hand-written line, visibly (ADR
	// 0002 §1); this file is not a second one.
	{"command", "a built-in's launch mechanism is not overlayable"},
	// The built-ins' skill surfaces are verified mechanisms (skillsClaude,
	// SkillsCwd materialization — rangerhq-1qd measured that codex and grok
	// have no flag at all), so either key here declares something measured
	// false and would run two half-bindings at once.
	{"skills_flag", "a built-in's skill surface is a verified mechanism, not overlayable"},
	{"skills_cwd", "a built-in's skill surface is a verified mechanism, not overlayable — declaring it here would point the links at the session dir while the built-in's flag points at the rendered tree, and no line could say which the CLI read"},
	// Whether a CLI sandboxes its own children is a property of the engine,
	// not of this box: declared here it would drop the seatbelt wrap posse
	// measured this built-in needs (or add a nesting refusal it does not).
	{"self_sandbox", "whether this CLI wraps its own children is a measured property of the engine, not an instance fact"},
	// parity's ProjectConfigTrust is built on the built-in's measured list;
	// a yaml one moves the guard rather than a number. project_config_keys:
	// is the one declarable key that LOOSENS a safety check, which is a
	// worse thing to do from a file to a guard posse measured.
	{"project_config", "the session-dir files this CLI reads as configuration are a measured property of the engine; parity's trust check is built on the built-in's list"},
	{"project_config_keys", "narrowing the project-config trust check is the one declaration that LOOSENS a guard — not something an instance fact may do to a built-in's measured list"},
	// The flag that approves a tool call with nobody watching is posse's
	// own measurement of this CLI's dialect; an overlay appends a different
	// flag to every launch line, which is mechanism by any reading.
	{"unattended", "the unattended flag is posse's measurement of this CLI's dialect, appended to every launch line here"},
	// Which registered reader parses this CLI's first turn is Go code, not
	// a reading of this box; the built-in's is the one measured against its
	// transcript, and a wrong one misreads a settle in silence.
	{"turn_outcome", "which registered reader parses this CLI's own first turn is code measured against its transcript, not an instance fact"},
	// Same split one screen over: which reader parses this CLI's SCREEN is
	// code measured against its captures. A claude release that rewords its
	// footer is fixed at the corpus the reader is checked against, not by an
	// overlay nobody measured — and an overlay declaring `none` here would
	// turn a read column into a permanent "—" with no measurement behind it
	// (ADR 0057 D2).
	{"pane_mode", "which registered reader parses this CLI's screen for its permission mode is code measured against that CLI's captures, not an instance fact"},
}

// builtinOverlayKeyList renders the overlay set for a refusal message and
// for `runtime check`'s built-in footer. Both used to spell it by hand;
// rendering it means the screen an onboarder reads and the wall they hit
// cannot disagree with the code that applies it.
func builtinOverlayKeyList() string {
	out := make([]string, 0, len(builtinOverlayKeys)+1)
	out = append(out, "model_<tier>:")
	for _, k := range builtinOverlayKeys {
		out = append(out, k+":")
	}
	return strings.Join(out, ", ")
}

// builtinMechanismKeyList renders the refused set the same way, for the
// same reason.
func builtinMechanismKeyList() string {
	out := make([]string, 0, len(builtinMechanismKeys))
	for _, m := range builtinMechanismKeys {
		out = append(out, m.key+":")
	}
	return strings.Join(out, ", ")
}

// overlayBuiltin applies runtimes/<name>.yaml as a per-key overlay onto a
// built-in (ADR 0021): absent file, today's behaviour exactly — rt comes
// back untouched; present, the yaml wins for the keys in
// builtinOverlayKeys, everything else stays the built-in's, and a key in
// builtinMechanismKeys refuses the load. List-valued keys (native_rules:,
// egress:, state_dir:, env_required:) REPLACE rather than merge, same as
// the template-only path — a merge rule would be a hidden one.
//
// rt.Models is cloned before any per-tier write: rt is a value copy of the
// shared builtinRuntimes[i] entry, but its Models field is still the SAME
// map underneath, and every LoadRuntime call for a name with no overlay
// file must keep reading the built-in's own values — an in-place write
// here would corrupt every other session's copy of that built-in.
func (a *App) overlayBuiltin(rt *Runtime, name string) (*Runtime, error) {
	p := filepath.Join(a.RuntimesDir(), name+".yaml")
	if _, err := os.Stat(p); err != nil {
		return rt, nil
	}
	// The MECHANISM keys refuse the load (ADR 0021 Decision 2). Read by
	// presence, not by value: a bare `unattended:` is still a declaration
	// that this file decides the launch mechanism, and answering it with
	// silence is the reading this contract exists to remove.
	for _, m := range builtinMechanismKeys {
		if yamlHasKey(p, m.key) {
			return nil, Die("runtime %s: %s declares %s: — %s (ADR 0021 Decision 2); a runtimes/%s.yaml may only overlay: %s",
				name, AbbrevHome(p), m.key, m.why, name, builtinOverlayKeyList())
		}
	}
	rt.Path = p
	models := make(map[string]string, len(rt.Models))
	for k, v := range rt.Models {
		models[k] = v
	}
	rt.Models = models
	for _, t := range Tiers {
		if id := YamlGet(p, "model_"+t); id != "" {
			rt.Models[t] = id
		}
	}
	if f := YamlGet(p, "model_flag"); f != "" {
		form, err := printfFlag("model_flag", f)
		if err != nil {
			return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
		}
		rt.ModelFlag = form
	}
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
		why := YamlGet(p, "record_why")
		if err := validateRecordDecl(v, why); err != nil {
			return nil, Die("runtime %s: %s %v", name, AbbrevHome(p), err)
		}
		rt.Record = v
		rt.RecordWhy = why
	}
	// native_rules:/egress: REPLACE, and only when the key is actually
	// present — yamlHasKey, not a length check, so `native_rules: []`
	// (an operator declaring "this instance's CLI reads none") is told
	// apart from the key being absent (keep the built-in's list).
	if yamlHasKey(p, "native_rules") {
		rt.NativeRules = YamlList(p, "native_rules")
	}
	if yamlHasKey(p, "egress") {
		rt.Egress = YamlList(p, "egress")
	}
	if v := YamlGet(p, "cage_cred"); v != "" {
		rt.CageCred = v
	}
	// gate_shell: false only — a misspelled or absent value leaves the gate
	// shell ON, same as the template-only path (more wall, not less).
	if YamlGet(p, "gate_shell") == "false" {
		rt.NoGateShell = true
	}
	// rules_precedence: which channel wins a native-rulebook/PID collision
	// on THIS box's CLI. The most instance-shaped key there is — it is a
	// probe's answer, not a code branch (ADR 0017 §5), and a built-in's
	// value is one release's measurement of one install.
	if v := YamlGet(p, "rules_precedence"); v != "" {
		if !ValidRulesPrecedence(v) {
			return nil, Die("runtime %s: %s has rules_precedence: %q (want %s or %s — ADR 0017 §5)", name, AbbrevHome(p), v, RulesPrecedencePID, RulesPrecedenceNative)
		}
		rt.RulesPrecedence = v
		rt.RulesPrecedenceWhy = YamlGet(p, "rules_precedence_why")
	}
	// state_dir:/env_required: REPLACE when present, keep the built-in's
	// when absent — the native_rules:/egress: rule, and for the same
	// reason: a merge would be a hidden rule, and a length check cannot
	// tell `state_dir: []` (this install keeps no state where posse thinks)
	// from the key being absent.
	//
	// Replacing state_dir: moves the seatbelt's writable grant, so an
	// overlay naming the wrong path costs a first-run flow every launch.
	// That is what an override is for and it is loud: the grid names the
	// file per key, and the file is the promoted config root (ADR 0039 D2),
	// not something a session can write.
	if yamlHasKey(p, "state_dir") {
		v, err := runtimeStateDirs(p)
		if err != nil {
			return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
		}
		rt.StateDirs = v
	}
	if yamlHasKey(p, "env_required") {
		v, err := runtimeEnvRequired(p)
		if err != nil {
			return nil, Die("runtime %s: %s has %v", name, AbbrevHome(p), err)
		}
		rt.EnvRequired = v
	}
	warnUnknownRuntimeKeys(runtimeNoticeWriter, name, p)
	return rt, nil
}

// validateRecordDecl is the record:/record_why: rule shared by the overlay
// and template-only paths (ADR 0021 Decision 4): record: trusted with no
// record_why: refuses. Promotion follows a measurement (ADR 0013 §4); a
// trusted with no measurement named is the silence the contract exists to
// remove.
func validateRecordDecl(record, why string) error {
	if !ValidRecord(record) {
		return fmt.Errorf("has record: %q (want %s or %s — ADR 0013 §4)", record, RecordTrusted, RecordUntrusted)
	}
	if record == RecordTrusted && why == "" {
		return fmt.Errorf("has record: trusted with no record_why: — promotion follows a measurement (ADR 0013 §4), never a bare edit; name what you measured")
	}
	return nil
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
	builtin := map[string]bool{}
	for _, rt := range builtinRuntimes {
		out = append(out, rt.Name)
		builtin[rt.Name] = true
	}
	ents, _ := os.ReadDir(a.RuntimesDir())
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		// A yaml naming a built-in is that built-in's overlay (ADR 0021),
		// not a second runtime — it lists once, from the loop above.
		if n := strings.TrimSuffix(e.Name(), ".yaml"); !builtin[n] {
			out = append(out, n)
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

// EnsureUnattendedLine types the unattended mode onto a launch line posse
// did not render — the `--cmd` of `posse new` and the `command:` of a
// recipe with no `agent:` (rangerhq-oaya). Those lines never reach
// RenderCommandFor, so until now they carried whatever mode the CLI
// happened to default to that release: the operator's directive is "every
// launch", not "every persona launch", and a no-persona session is still
// one of theirs to be blocked in.
//
// No runtime is named on such a launch, so it is recovered the only honest
// way there is: by the executable the line starts. A line that starts a CLI
// whose dialect posse knows gets that CLI's flag; a shell, an editor, a
// wrapper, `env FOO=1 claude`, or a template-only runtime that has no
// unattended dialect to declare gets nothing — the same refusal to guess
// EnsureUnattended already makes, and for the same reason.
//
// A mode already on the line always wins: EnsureUnattended appends only
// when the flag is absent, so `--cmd "claude --permission-mode plan"` is
// left exactly as the operator typed it. That is what keeps this a default
// rather than an imposition on the operator's own session.
func EnsureUnattendedLine(cmd string) string {
	f := strings.Fields(cmd)
	if len(f) == 0 {
		return cmd
	}
	exe := filepath.Base(f[0])
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Exe() == exe {
			return builtinRuntimes[i].EnsureUnattended(cmd)
		}
	}
	return cmd
}

// ─── Tier display (ADR 0013 §6) ──────────────────────────────────────────────

// TierUnmapped is what a tier renders as on a runtime that does not map it.
//
// The three tier names are INTENT — judged / building / mechanical (ADR
// 0003 §1) — and ADR 0013 §6 amends only their display: a runtime with no
// model id behind the name does not get to wear it. `mycli/strong` read as
// a quality guarantee on a runtime where {model} renders empty and the CLI
// picks whatever it likes; `mycli/default` says exactly that much and no
// more. Nothing about resolution moves: dispatch still resolves the tier it
// resolved before, overflow still never trades `strong` (ADR 0010 §2b), and
// an explicit --runtime the operator typed still launches.
//
// All THREE built-ins map every tier since rangerhq-jp6 gave grok its map,
// so the rule now bites only on a declared runtime that sets no
// `model_<tier>:` (or one whose map is partial), and on a runtime name
// posse has never heard of. That is a narrower blast radius, not a dead
// rule — and it is why the pins for it are written against a declared
// fixture rather than against whichever built-in happened to be unmapped
// that week.
const TierUnmapped = "default"

// RuntimeMapsTier reports whether <runtime> renders a model id for <tier>
// — the predicate the display rule above is keyed on.
//
// Deliberately cheaper than LoadRuntime, because the listing paths ask it
// once per session per redraw: a built-in answers out of its own map with
// no file touched, and a declared runtime is answered by the two keys that
// can decide it (model_<tier>:, plus model_standard: for fast's fallback).
// An unknown runtime maps nothing, which is the honest reading rather than
// a defensive one — posse cannot promise a model on a CLI it has never
// heard of.
func (a *App) RuntimeMapsTier(runtime, tier string) bool {
	if runtime == "" {
		runtime = DefaultRuntime
	}
	if tier == "" {
		tier = DefaultTier
	}
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Name == runtime {
			return builtinRuntimes[i].Model(tier) != ""
		}
	}
	// Past the built-ins this needs an instance to read RHQ_HOME/runtimes
	// from. A caller without one — a rendering fixture, a row drawn before
	// refresh — gets "unmapped" rather than a panic in a draw path: this is
	// a display predicate, and the honest answer when the map cannot be
	// read is that no mapping is known.
	if a == nil || a.Home == "" {
		return false
	}
	p := filepath.Join(a.RuntimesDir(), runtime+".yaml")
	if _, err := os.Stat(p); err != nil {
		return false
	}
	rt := Runtime{Models: map[string]string{}}
	if id := YamlGet(p, "model_"+tier); id != "" {
		rt.Models[tier] = id
	}
	if tier == TierFast {
		if id := YamlGet(p, "model_"+TierStandard); id != "" {
			rt.Models[TierStandard] = id
		}
	}
	return rt.Model(tier) != ""
}

// DisplayTier is the tier as an operator should READ it for a session on
// this runtime: the tier's own name where the runtime maps a model for it,
// else TierUnmapped. Empty in, empty out — a caller with no tier to show
// has nothing to make honest.
//
// A tier that is not one of the three names passes through UNTOUCHED. §6 is
// a rule about `strong` / `standard` / `fast` — the names that carry the
// intent — and a session meta holding `premium` is corruption, not a tier
// this runtime declined to map. Rewriting it to `default` would erase the
// one place an operator could see it.
func (a *App) DisplayTier(runtime, tier string) string {
	if tier == "" || !ValidTier(tier) {
		return tier
	}
	if a.RuntimeMapsTier(runtime, tier) {
		return tier
	}
	return TierUnmapped
}
