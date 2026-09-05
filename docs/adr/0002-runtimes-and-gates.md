# ADR 0002 — Runtimes and gates: launch boundaries and what they prove

*Status: accepted 2026-08-17; consolidated 2026-09-05 under the operator ruling · owner: architect.*

## Context

Doing nothing leaves the same enforcement result defined in five records
and successive amendments. A persona is its declared policy; a runtime is
replaceable labor. Keep model-independent enforcement and refuse a launch
that cannot realize its requested posture. This consolidation preserves the
running boundaries; it does not build a stronger threat model.

## Decision

**1. A runtime is a named launch profile.** CLI runtime selection overrides
recipe, then PID, then fleet default. A PID's `command:` applies only to
its own runtime; switching runtime selects the target's template. Native
flags reduce friction; they are not automatically an enforcement boundary.
The current declarations, template obligations, built-in overlays and
onboarding checklist have one home in [0013](0013-runtime-dispatch-contract.md).
Template-only runtimes must demonstrate their capabilities; being loadable
does not prove parity. Tier precedence belongs to 0003. Posse does not
write runtime-owned config homes (0035).

**2. Carry policy across the launch.** The launch carries `BD_ACTOR`,
`RHQ_PERSONA`, `RHQ_PERSONA_DIR`, `RHQ_TOOLS_ALLOW`, `RHQ_TOOLS_DENY`,
`RHQ_RUNTIME`, `RHQ_GATES_DIR` and `RHQ_CAGE`. Session metadata records
runtime, cage and degradation. [0055](0055-store-of-record-rides-the-session-env.md)
owns explicit `BEADS_DIR`; it is the same resolved store granted by the cage.
Do not infer session facts from names or queue facts from a second store.

**3. Render the wall from the PID at every launch.** `allow:` remains
runtime-native friction; `deny:` selects the available boundaries below.
`cage:` is the minimum requested tier. Generated artifacts are disposable.

| Layer | Mechanism and carrier | Reach |
|---|---|---|
| L0 | Native runtime flags | Runtime promises; count an OS sandbox only for the scope it actually enforces |
| L1 | Argv-aware command shims plus the typed PATH and gate shell | Cooperative shell-verb denies on the normal command path |
| L2 | Seatbelt wrapping the runtime, with declared write grants and trailing subtree denies | Enforced filesystem boundary; launching runtime's state only, including required atomic-write siblings |
| L3 | Git hook render, identity check and own-render behavior probe | Cooperative commit/push checks; managed realization in 0052 |
| L4 | Container mount boundary and internal network with CONNECT proxy | Enforced filesystem and configured egress effects; L1/L3 must also be realized inside |

L4 replaces L2; wrapping the engine client in seatbelt does not cage the
container. The inner wrapper renders shims and shell against image binaries,
not host paths. The runtime launcher preserves foreground identity for
herdr. Forward the required session environment to the image. Mount only
declared capabilities; the herdr socket is fleet-wide power and is absent
unless explicitly requested. Runtime service hosts join the proxy allowlist.
An allowed host can still carry unwanted traffic; deny-list prose is not a
second network boundary. Image/engine availability must be checked honestly.

[0014](0014-path-scoped-writes.md) owns bare/scoped write semantics,
`writable:` grants and container overlay ordering;
[0038](0038-git-identity-write-deny.md) owns persistent git identity protection.
Do not weaken those boundaries by copying an older mount table here.

### L1 gate shell (from 0009)

The typed command sets the persona's shim PATH and `SHELL`/`GROK_SHELL`
to its rendered gate shell. Resolve the real shell outside **every** gates
directory; refuse a wrapper-to-wrapper target. Name the wrapper for the
real shell's dialect. Parse shell options and their operands; prepend the
guard to the command string and, when present, the runtime's user-command
slot after snapshot replay. Rebuild PATH: remove every gates path element
and put only this persona's bin first. Presence anywhere in PATH is not
precedence. Apply the same guard to the wrapper's environment before exec;
pass interactive/script forms through without inventing a command string.

The operator's rc commands execute under this same policy. A refusal proves
the session attempted a command, not that the model typed it. Do not exempt
rc files or infer model intent from a log. `gate_shell: false` is the
existing escape hatch and degrades shell denies honestly. A SHELL-only
launch is a rejected, unbuilt alternative unless an actual runtime requires
it: non-shell execs would escape. Credential mint ownership and launch
preconditions live in [0019](0019-credential-architecture.md).

### L3 identity and behavior

Install only owned bytes or the prescribed chain; never overwrite a foreign
hook. The shared-checkout pathspec requirement (0022), constitution fence
(0015), public visibility (0024), data ceiling (0050), and citation policy
(0051) retain their separate decisions. `prepare-commit-msg` survives git's
`--no-verify`; `pre-push` does not. The shared-index arm reads tree shape,
not persona identity; actor-specific arms carry identity in the environment.
Linked-worktree exemptions in a hook do not waive a PID's argv deny.

Realization is **dispatch identity AND behavior of our own render**.
At git's actual dispatch path, require executable byte-exact current members
and prescribed dispatchers. Execute only a fresh private temporary copy of
the render, never on-disk hook bytes chosen by another writer. The behavior
probe must refuse; marker text alone is no identity proof. A foreign hook
degrades with its path and chaining remedy even if it would refuse; an owned
stale hook names reinstallation. This proves a launch-time observation, not
immunity to later tampering or use-time redirect.

[0052](0052-managed-hooks-path-session-redirect.md) owns the managed path
exception: the per-session redirect must carry the same identity/behavior
proof and complete forwarding of managed slots. The managed directory is
never modified; a failed proof degrades. Persistent file protection (0038)
does not close command-local hook redirection.

**4. Enforcement parity: refuse or degrade explicitly.** Compute parity
for PID, runtime, cage and tier; `CheckParityIn` additionally observes the
launch directory and L3 result. Any unrealized or degraded requirement
refuses unless the operator explicitly selects `--allow-degraded`; automatic
dispatch never invents that waiver, and tier `fast` cannot waive it.
Render the reason and record degradation in session metadata/cockpit.
Recheck on relaunch. Bare write, scoped write, web, tool-name and egress
rules each need their actual realization; native tool flags do not imply a
kernel boundary. Equivalent runtime self-sandbox posture may be a declared
difference under 0013 rather than a fictitious loss of protection.

Every realized gate carries a class; `posse gates`, rendered parity and
session metadata must preserve that distinction wherever they report
realization. The metadata obligation is retained from 0025; this fold does
not assert that an unobserved metadata field has shipped.

| Class | Meaning |
|---|---|
| `enforced` | Outside the process: L2, L4 mounts/network/proxy and an actually active runtime OS sandbox |
| `cooperative` | Ordinary execution path: L1 shims, gate shell and L3 hooks at every tier |
| empty | A realized row that is not an adversarial gate, such as model, skill or metering information |

Class changes no verdict and requires no waiver. Absolute-path commands,
environment removal and hook redirects expose different cooperative holes.
At L4 a push's **effect** is blocked only when the configured read-only
mount or network boundary actually stops its destination; the launcher
does not know enough to certify every remote. Print this as a qualified
note, never upgrade the verb gate's class.

### Refusal trail: one host-owned canonical record

Never mount the canonical refusal log into the cage. Inner shims append
to a per-session writable spool; existing host pass/reap, close and relaunch
paths fold it into the canonical log. A host-only cursor stores the byte
offset and hash of the folded prefix. Append new lines with the session id,
then advance the cursor; unchanged repeat folds add nothing. A size below
the cursor or changed folded prefix adds a tamper line and refolds from
zero as suspect. This is deduplication during ordinary retry, not a
transactional exactly-once promise across a crash between append and cursor.

Truncating unconsumed bytes, truncating exactly to the cursor, or erasing
before the first fold can lose lines without detection. Only already-folded
history is beyond the cage's reach; the canonical log cannot be shortened
from inside. At shims tier the filesystem is still cooperative. No new
daemon is needed. MEASURED: the prefix/truncation cases in 0025's hermetic
evidence. UNVERIFIED: its full live-container wiring probe; this docs run
does not turn skipped container tests into measurements. Rejected: a live
tail daemon or wider socket capability to close a window the current
threat model does not require closing (extra actor/capability cost ASSUMED).

### Project configuration is a launch input

Keep directory trust for unattended launches. Posse names the executable
configuration channel and requires an explicit opt-in, rather than trying
to implement each runtime's settings engine. Runtime `ProjectConfig` is an
ordered path list; YAML `project_config:` still declares one path. Codex's
project file triggers on presence. Claude's project and local settings files
both trigger on top-level `hooks` or `mcpServers`, regardless of value;
missing files or a valid object without the keys are clean. An unreadable,
nonregular, malformed or non-object keyed file degrades. Name the first
offending file in declaration order. `.mcp.json` retains its separate
runtime approval boundary and is outside this detector. A PID's
`trust_project_config: true` opts in; otherwise §4 applies on every launch.
The retained key list describes a declared channel, not a claim that every
runtime version executes every key. Headless and interactive trust differ;
the historical probes do not establish behavior of a new binary.

**5. PID additions.** `runtime:`, `cage:`, `writable:`, `egress:`,
`sockets:` and `trust_project_config:` retain their current meanings above
and in their owning contracts. Unsupported requirements are visible. No
new key, flag, state, actor or runtime mechanism is added by this fold.

## Consequences and alternatives

MEASURED historical evidence: login-shell PATH reordering and wrapper cycles
(0009); a hook discriminating the probe signature (0023); emptied-env and
hook-redirect escapes and erased spools (0025); managed-hook forwarding
(0052). Probes keep their original paths, including
[the gate-shell probe](0009-gate-shell.probe.sh) and
[the container probe](0002-container-tier.probe.sh). No live runtime or
container probe was run in this documentation execution. The current
no-daemon rationale is [0056](0056-no-daemon-is-the-pin-tripwire.md), a
compatibility tripwire; historical daemon-import/performance explanations
are version-specific.

Rejected: doing nothing (competing claims of realization); treating native
permission prose as a boundary; trusting a marker or executing a foreign
hook to certify it (a chosen witness can lie); hardening only one hook
bypass (the others remain); replacing the shared gate shell with per-runtime
login knobs. ASSUMED maintenance gain: fewer policy homes. Re-rendered
artifacts hold no irreplaceable state; changing runtimes or container engines
requires the same parity checks. Refusal history remains host-owned.

## Lineage

| Was | Here |
|---|---|
| 0002 §§1–5 and project-config/escape amendments | §§1–5 above; dated detail in the historical snapshot |
| 0009 §§1–4, including REAL resolution, PATH position, rc and credential distinctions | §3 L1; credentials in 0019; executable probe path unchanged |
| 0023 §§1–4 identity, own-render probe and foreign classification | §3 L3; 0052 supplies the managed realization under this same proof |
| 0025 §§1–4 classes, push-effect qualification, host-owned refusal fold; verification limits | §4 and Refusal trail; dated probe outcomes remain in 0025 |

[Historical evidence and prior alternatives](history/0002-runtimes-and-gates-before-2026-09-05.md)
are retained separately and are not a second active contract.
