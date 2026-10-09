# The one launch precondition the PID lint could read and did not (ranger-base-sl5sg)

From github.com/ranger360ai/posse issue #6, filed by the operator out of the
work-box shakedown (ranger-base-x4g3h), MEASURED on a real install 2026-10-08.

## The state

A claude PID scaffolded with the crew's deny set — `Bash(security:*)`, the
keychain-CLI tripwire every crew PID carries (ranger-base-khu) — and no
`envs:` is refused at launch, correctly and unwaivably: the refusal names the
runtime's session-credential key — the one `cage.go`'s `cageCredential` table
declares, which `posse runtime check <name>`'s `cage_cred` row prints — and
says it is in none of this session's env sets (ADR 0042 D2). Restated rather
than quoted: a key name is schema, but the refusal's own line is the one place
it sits beside where the value lives, and ADR 0024 D3 is restate-and-cite.

That refusal is the design. The gates dir leads the PATH of the runtime
PROCESS, not only of the persona's shells (ADR 0002 §3), and claude's
credential path execs `security` by bare name — so the rule shims the
runtime's own read, which is what keeps the operator's rotating pair
single-writer (ADR 0042 D1). A crew runtime authenticates with the session
mint posse injects instead, and without the mint the session opens logged out
and cannot refresh at expiry, so the launch is refused rather than spent.

## The defect

Nothing said so before the launch. `posse agent check` read every other
launch precondition off the PID file — `sockets:` a cage tier cannot mount, a
path-scoped write at `shims`, a `tier:` under the PID's own `tier_floor`, a
`command:` with no `{model}` or no PID channel — and had no rule for this
one. So a PID that could not open a session at all linted clean, and the
operator met the refusal at its first dispatch. The scaffold was silent in
the same way: `envs: [gh]  # env sets this persona's sessions receive` says
what the key does and not that a claude PID with the rule cannot launch
without it.

## What landed

`CredGateLint` (gates.go) beside `CredGateRefusal`, and the lint that
calls it (pidcheck.go). The pair is the `PIDChannelFinding` /
`PIDChannelRefusal` shape one precondition over: both derive the rule, the
binary and the key from the same functions — `CredGateCollision`,
`CageCredential` — and differ only in voice, because a linter's line and a
launch refusal are read in different rooms. The reader under it is
`EnvSetKeyNames` (envs.go), which returns the KEY NAMES of the sets the PID
names and never a value, so no caller can print one it did not receive
(rangerhq-f2b). A set the PID names that this box does not have comes back
separately: that is a launch refusal of its own ("env set not found") and
minting fixes none of it, so the warning says so instead of claiming a key is
absent from a file it never read.

Two decisions worth keeping:

- **A warning, never a finding.** The env store is machine-local
  (`RHQ_HOME/envs`, mode 700) and never in the repo, so `posse agent check`
  run in an instance repo's CI has no sets to read — a finding would red
  every build over a PID that is correct. The same launch is also reachable
  with `--env-file`, which no PID file can see. Pinned
  (`pidcredgate_qa_test.go`): a finding carrying the key name reds the arm.
- **The bin dir is named, never rendered.** `CredGateCollision`'s only use
  for it is excluding it from the PATH it resolves the binary on — which is
  what keeps a `security` deny from warning on a box that has no such binary
  — and a lint must write nothing: it runs in CI and inside a cage, where the
  gates dir is not writable (the shape `GateShellOn` exists for,
  ranger-base-zbg8o).

## The pin that caught the first cut

`TestNoNewShadowPredicate` (internal/treepins, ADR 0017 §3) reds a branch
keyed on a runtime's NAME where the behaviour is a dimension rather than that
CLI's own state. The lint's first cut had one — the recipe clause said
`` `claude setup-token` `` only `if rt.Name == DefaultRuntime` — and the pin
was right: how a session credential is minted is a dimension, and the place a
second runtime's recipe belongs is the declaration that names its key
(`cage_cred:`, printed by `cageCredRow`), not a name comparison in a sentence
builder. The fix made the clause unconditional and moved it into
`credMintDoor()`, which the launch refusal now reads too — so the drift the
two copies would have had is gone as a side effect of satisfying the pin. The
refusal's own text is byte-identical to before.

Worth noting for the next hand: `CredGateRefusal` has carried the literal
"on claude: `claude setup-token`" in a string since ADR 0042 and the pin has
never objected, because it reads BRANCHES and not prose. A name in a sentence
is a fact about one CLI; a name in an `if` is a dimension pretending not to
be one.

## Measured

| claim | status | source |
|---|---|---|
| the bead's exact state now warns, naming the rule, the binary, the key and the recipe | MEASURED 2026-10-09 | a crew PID copied into a scratch `RHQ_HOME` with its `envs:` line removed |
| all eleven crew PIDs on the reference box stay silent | MEASURED 2026-10-09 | `posse agent check --all`: 11 clean, no credential line on any |
| the lint renders no gates dir | MEASURED 2026-10-09 | `TestQAAgentCheckRendersNoGatesDir` |

The second row re-confirms ADR 0042's own table ("all eleven crew PIDs name
an env set with the mint") from the other side: the check that would have
caught issue #6 adds no noise to a box that is correct.
