# Historical snapshot — not current policy

Archived 2026-09-05. Current decision: [ADR 0035](../0035-mode-second-layer-typed-line-only.md).

# ADR 0035 — Permission-mode second layers ride the typed line; posse writes no foreign config home

*Status: accepted 2026-08-29 · owner: architect · relates 0002 §3 (the cage
is ours), 0017 (grok's cell is a DECLARED DIFFERENCE), 0022 (single writer,
extended here to homes posse does not own) · source bead ranger-base-p4aa,
ask 2 of ranger-base-0emp, baseline rangerhq-slq6*

## Context

rangerhq-slq6 gave claude a second mode layer: `--settings
'{"permissions":{"defaultMode":"auto"}}'` beside `--permission-mode auto`
on the same rendered line, so a launch that drops one flag still lands the
mode from the other. The threat it answers is one-flag drift — template
edits, a CLI arg rename, an argv-building path that forgets a token
(dispatch has two such paths; ranger-base-unzn) — not whole-line loss.

The bead asks whether codex and grok get an equivalent, and its
measurements (laurie, 2026-08-29, codex-cli 0.150.1 / grok 1.0.5 / claude
2.1.251) frame the cost: codex's `-p posse` layers
`$CODEX_HOME/posse.config.toml` — a posse-written file in the operator's
`~/.codex`, silently ignored when missing or misspelled; grok has no
scoped file at all, only the operator's global `~/.grok/config.toml`.
What is at stake on codex is real: its empty-config default approval
policy is OnRequest — a dispatched session sitting on dialogs nobody
watches.

The structural fact that decides this: claude's second layer is not "a
settings file", it is an **inline payload on the line posse already
types**. If the flag survives, the payload survives — nothing on disk can
dangle, go stale, or be edited out from under it.

## Decision

**1. Class ruling: no posse-written file in a runtime config home posse
does not own.** `~/.codex` and `~/.grok` are the runtimes' and the
operator's. A mode layer that lives there is a cross-artifact invariant
(flag in the template, file on disk, two writers, three release
schedules), its absence or staleness is exactly the silent drift a second
layer exists to catch, and posse already audits runtime config files as
attack surface (`Runtime.ProjectConfig`, the trusted-repo check) — it
does not also mint them. A second layer is in scope only when it rides
the same typed command line as layer 1.

**2. codex gets its second layer, inline: append `-c
approval_policy=never` to the built-in template beside `-a never`.**
Measured this session (richard, scratch `CODEX_HOME`, no credentials,
codex-cli 0.150.1):

- the `-c` spelling lands: `codex doctor --json -c approval_policy=never`
  reports `"approval policy": "Never"` where the base config reports
  `"OnRequest"`;
- the wrong arm is loud: `approval_policy=bogus-not-a-policy` via `-c`
  dies exit 1 naming the valid variants, on `codex exec` and on the root
  command — unlike `-p <missing>`, this layer cannot be absent silently;
- the pair coexists: root `codex -a never -c approval_policy=bogus` still
  dies on the `-c` parse (so `-a` does not short-circuit it), and the
  agreeing pair loads clean to the TTY check;
- `codex exec` takes no `-a` at all, so the `-c` spelling is the more
  portable of the two across codex's own subcommands.

Precedence on disagreement is unmeasured and moot by construction: both
spellings render `never` from one constant on one line.

**3. grok gets nothing, and its control is detection.** grok 1.0.5 has no
per-invocation config channel (re-verified: `--help` offers no settings
payload or scoped config file; `grok setup` fetches *managed global*
config; `GROK_HOME` relocation lands a fresh device-code login). The only
file layer is the operator's global `~/.grok/config.toml`, which governs
every grok session on the box including the operator's own interactive
ones — a blast radius wider than the fleet, so it stays out of scope even
with permission; and it would invert the layer order, since argv already
beats it. The compensating control already shipped as the other half of
ranger-base-0emp: the actual pane mode is read from the composer border
and surfaced in `rhq list`/gates, so a flag-lost grok session is
*visible*, not prevented. Under ADR 0017 this is a DECLARED DIFFERENCE,
not an UNKNOWN.

**4. Vocabulary guard: a mode is a default disposition, not a
non-blocking promise.** Measured the same morning: a claude session in
auto mode blocked on "Auto mode classifier requires confirmation for this
command." No surface may render mode=auto/never as "will not block"; the
mode column and the blocked/queue state stay separate facts.

## Consequences

- One `-l code` bead to dinesh: the one-token template edit plus pins.
  Count the declarations (ranger-base-unzn): assert the rendered line
  carries both spellings on *both* dispatch launch paths, and
  mutation-check by deleting `-a never` — the pin must go red on the
  remaining spelling's absence, not on prose.
- Re-verification recipe when codex moves: the bogus-variant probe (exit
  1, variant list) is the discriminating arm; run it before trusting the
  layer on a new codex release. codex's `--strict-config` (errors on
  unrecognized keys) exists as a future loudness option; not adopted —
  it hardens a file posse now deliberately does not write.
- If codex ever removes `-c` or renames `approval_policy`, the layer
  degrades to layer 1 exactly as claude would on losing `--settings`;
  the class ruling stands and the fix is a new inline spelling, not a
  file.
- If grok ever grows an inline settings payload (it already carries
  claude-compat aliases), the probe-then-append path here is the recipe;
  until then its parity-grid cell reads declared, with this ADR as the
  why.

## Alternatives rejected

- **`-p posse` profile in `~/.codex`** (the bead's headline option):
  posse writes a directory it does not own, and the layer's absence is
  silent — `-p posse` with no such file runs the base config with no
  error (measured). A defense whose own failure is silent adds a false
  sense of coverage, not coverage. And it only fires if `-p` itself
  survives argv drift, so it defends a narrower loss than the inline
  spelling at strictly higher cost.
- **`CODEX_HOME=<posse dir>`** (own the whole config home): auth lives
  there too — a scratch home reports "no Codex credentials were found"
  (measured). Buys config ownership at the price of the operator's
  login; the exit hatch would hold credentials hostage.
- **Writing `~/.grok/config.toml [ui] permission_mode`** with operator
  permission: changes the operator's own sessions (this box:
  always-approve, their choice), and argv beats it anyway — the file
  would matter only to a launch that lost argv, i.e. posse's config
  would govern exactly the panes posse did not launch.
- **`GROK_HOME` relocation**: fresh device-code login screen (measured,
  aborted). Same auth hostage as CODEX_HOME.
- **Doing nothing for codex**: leaves the mode single-spelling against
  slq6's own threat model, when the fix is one argv token with a loud
  wrong arm. The asymmetry with grok is honest: codex has an owned
  inline channel, grok does not.

## Claims

MEASURED (richard, this bead, codex-cli 0.150.1, scratch CODEX_HOME):
`-c approval_policy=never` → doctor "Never" vs base "OnRequest"; bogus
variant via `-c` exits 1 loudly on root and exec; `-a`+`-c` coexist;
exec has no `-a`; grok 1.0.5 `--help` surface re-checked. MEASURED
(laurie, ranger-base-p4aa): `-p` layering and its silent-absence miss;
CODEX_HOME/GROK_HOME auth cost; claude `--settings` behavioral baseline;
grok config-file precedence and pane-border mode rendering; the
auto-mode classifier block. ASSUMED: `-a` vs `-c` precedence on
disagreement (moot — one constant renders both, agreeing); `codex
doctor`'s reported policy is the root TUI's effective one (both read the
same config loader; the bead's OnRequest baseline came from doctor too).
