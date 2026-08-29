## A verb-prefix deny is not a fence: resolve the argv (ranger-base-3bqn)

A Claude Code `Bash(<cmd> <verb>:*)` permission rule matches a **token
prefix of the typed line**. Any global option in front of the verb moves the
verb out of that prefix and the rule misses. The discriminating pair, one
token apart (measured on ranger-base-az93, bd 0.49.1 / claude 2.1.251):

    bd daemon             --help   ->  refused
    bd --no-daemon daemon --help   ->  ran, exit 0

`--no-daemon` is the spelling the fleet is *taught* to type (ranger-base-42mv),
so this is not an exotic evasion — it is the ordinary one. The same hole is
in every rule of that shape, in a PID's `deny:` and in a project
`.claude/settings.json` alike. Two consequences worth stating plainly:

- **An enumerated deny list closes the accident, not the class.** It also
  cannot close a verb nobody enumerated — bd has at least one command its
  own `--help` does not list (`daemons`, az93).
- **Flipping to an allow-list posture in a settings file does not work.**
  Deny beats allow (`Hook returned '<allow>', but deny rule overrides:` in
  the 2.1.251 bundle), so `deny: Bash(bd:*)` plus per-verb allows kills
  `bd show`/`bd ready`/`bd close` for everyone.

**The fence that survives is a parser, and posse already had one.** The L1
shim renders `posse_verb_match`, which skips the command's leading global
options — consuming in pairs the ones that take a separate value — and
matches the first non-option token (rangerhq-2zm). `globalValueOpts` is that
per-command list, and until this bead it knew only `git` and `posse`. bd's
entry is `--actor --db --dolt-auto-commit --lock-timeout`: **eighteen** global
options on 0.49.1 (not the seven the first write-up counted), of which exactly
those four eat the next word. Without the entry, `bd --db /tmp/x daemon stop`
resolves to the verb `/tmp/x` and walks through; with it, the shim refuses
every reordering and still passes `bd --actor daemon show x`, where `daemon`
is an option's *value* and not the verb. `matcherFor` upgrades bd rules from
"subcommand, best-effort" to "subcommand, option-aware", which is what parity
is allowed to claim.

**Where the typed line still beats the shim, and the reverse.** A PATH shim
never sees `/abs/path/to/bd daemon stop`; a string matcher never sees an
alias. So `scripts/bd-argv-gate.sh` is the other half — a PreToolUse hook
that parses the typed line, resolves bd's verb the same way, and refuses
unless the verb is on an allow-list (which is how the hidden-verb class
closes: a hook decides alone, so allow-list posture is available there and
not in a settings file). Both layers are **cooperative** in ADR 0025's sense,
never `enforced`: the wall against a process that means it is the L2 cage.
ADR 0014 §5 stands — posse renders no hook; this one is a script the operator
installs, and its default is to say nothing so it can never widen a grant.

Three properties of the hook were measured against the real binary, each with
a failing wrong arm, because a fence in the Bash path has fleet-wide blast
radius (run record in the instance tree):

1. **It sees compound commands.** A PreToolUse matcher is a *tool-name*
   regex, and `tool_input.command` carries the whole line, `cd x && …`
   included. Control: with no hook, all four probe commands ran.
2. **It fails closed only because it exits 2.** Claude's own contract:
   "Exit code 2 - show stderr to model and block tool call / Other exit codes
   - show stderr to user only but continue with tool call". Control: a hook
   that exits 1 let the refused command run. So the wrapper maps every
   could-not-decide path to exit 2 — but *only for lines that name bd*, or a
   mistyped path in settings.json would deny every Bash call in the fleet.
   `python3 <missing file>` exits 2 by itself, so the block cannot key on the
   exit status alone; it keys on the parser's own stderr marker.
3. **Path spellings yes, shell indirection no.** Refused: an absolute path,
   `./bd`, `env bd`, `FOO=bar bd`, a quoted `sh -c`, `xargs bd`, `eval`, a
   substitution, and a `$VAR` command word on a line that names bd (that last
   one *ran* against the first cut — measured, then closed). Not refusable by
   construction: an alias, or any name that reaches bd without spelling it.
   Those are what the cage is for.

The pins: `bdargvgate_qa_test.go` (decisions and the failure path),
`internal/rhq/bdshim_test.go` (the shim entry, pinned from both sides so
deleting it fails twice). Installing either fence is the operator's — the
hook because `Edit(.claude/**)` is denied to personas, the PID rules because
the shipped deny set is ADR 0015 §3's.
