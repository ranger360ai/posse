## A fence in a settings file does not travel; a fence in the PID does (ranger-base-d866)

Three things can refuse a command in a dispatched session, and only two of
them follow the session into another repo:

| layer | where it is written | travels with |
|---|---|---|
| project permissions | `<repo>/.claude/settings.json` | **the repo** |
| L0 `--disallowedTools` + L1 `PATH` shim | the PID's `deny:` | **the session** |
| PreToolUse hook | the operator's user settings | **the box** |

The PID's `deny:` is rendered into `$RHQ_GATES_DIR/bin/<cmd>` and into
claude's `--disallowedTools` argv at launch, so it is in the environment the
session carries; neither layer asks which repo the session is standing in. A
project settings file is read *because of where the session is*. That is the
whole finding: a rule that exists only in one repo's settings is absent one
`cd` or one cross-repo dispatch away, and this is the same shape as every
other "the fence does not travel" report.

**So a rule that must hold everywhere belongs in the PID** — which is what
ADR 0015 §3 says for `posse promote` and, as amended 2026-08-29
(ranger-base-u9ud), for bd's destructive and egress verbs.

**And then nothing was checking the PIDs.** The nine shipped example PIDs
carried the amended set within hours; the PIDs that actually dispatch carried
none of it for the rest of the day, and no command on the box could say so.
`posse gates <persona>` answers a different question — *is the rule this PID
carries realized on this runtime* — and answers it beautifully about a rule
that is not there. The missing check is the flat one: does the PID carry the
rule at all. `scripts/verify-pid-deny-set.sh <home>` (`make verify-pid-deny-set`)
is it; `piddenyset_qa_test.go` pins it over `examples/`.

**Measured on the rendered shim** (bd 0.49.1, 45 spellings through a copy of
the real `$RHQ_GATES_DIR/bin/bd` whose `exec` was replaced by an echo, so no
denied verb ever ran): 26 refused, 19 ordinary verbs untouched — `dep list`
survives the `dep relate` rule, `sync --flush-only` survives `sync --full` —
and the option-aware match holds both ways: `bd --db /tmp/x daemon stop` is
refused while `bd --actor daemon show x`, where `daemon` is an option's
*value*, is not.

**One residual, measured rather than assumed.** A `<verb> --flag` rule is
position-sensitive and the tool's flag is not: the shim matches `--full` only
as the token immediately after `sync`, so `bd sync --push --full` walks past
`Bash(bd sync --full:*)`. The L2 argv gate refuses that spelling (its
`SUBDENY` scans the whole segment), so on claude it is covered and on
grok/codex it is not. Filed as ranger-base-vct2 rather than widened here.
