# ADR 0014 — Path-scoped writes: a parametrized Edit/Write deny is a subtree file-write deny

*Status: accepted 2026-08-25 · owner: architect · amends 0002 §3 L2/L4,
§4 matrix, §5 `writable:`*

> The write cage was binary. An external reader of the public surface
> asked whether a persona can be allowed some paths and refused others.
> This is that layer, classified honestly per runtime. A hook is not it.

## Context

ADR 0001 took Claude Code's permission-rule syntax as the PID's `deny:`
grammar. ADR 0002 realized the *bare* names `Edit` / `Write` /
`NotebookEdit` as a file-write wall: L2 omits the repo from the seatbelt
allow-list (leaving `.beads/` and `.git/` so bd still works), L4 mounts
the repo `:ro`, and codex `-s read-only` counts because it is
OS-enforced. Anything else — `Edit(docs/adr/**)` included — falls to
parity's default case (`parity.go`), classified as a tool-name deny:
runtime-native only, unrealized below `container`, and at `container`
mis-labelled as a stdio MCP server.

So the cases that matter are held by prose:

- a verifier editing the code it judges
- a builder editing the ADR that constrains him

`writable:` already carries per-path *allows* at L2 (extras on a
write-denied repo). Nothing carries per-path *denies*. L4 does not honour
`writable:` at all. A Claude PreToolUse hook was offered as the missing
layer; ADRs 0001/0002 mention settings hooks only as a trust channel and
an explicit follow-on, never as a priced enforcement layer.

L0 already forwards the parametrized rule (`--disallowedTools
'Edit(docs/adr/**)'`). That is politeness. `sed -i` walks past it.

## Decision

**1. Grammar.** A parametrized `Edit(<glob>)` / `Write(<glob>)` /
`NotebookEdit(<glob>)` deny is a **subtree file-write deny**, not a
tool-name deny. Any one of the three targeting a subtree is the OS deny
of that subtree — the same "any of the three" union the bare rule
already uses. Relative globs join the session dir; `~` expands as
`writable:` does.

A glob is a **subtree glob** when, after stripping a trailing `/**` or
`/`, the remainder contains no `*`, `?`, or `[`. Those are the only
globs L2/L4 can express (SBPL `subpath`, a bind-mount). `Edit(**/*.md)`
and `Edit(docs/adr/**/*.md)` are **unrealized**, named: *not a
directory-prefix glob; the wall realizes subtrees
(`Edit(docs/adr/**)`), not file filters*. `Edit(**)` / `Edit(*)` /
`Edit(.)` are the bare rule; `posse agent check` says so.

Two shapes, existing keys, deny-wins:

| shape | PID | meaning |
|---|---|---|
| deny-list | `deny: [Edit(docs/adr/**), Write(docs/adr/**)]` | repo writable except that subtree |
| allow-list | `deny: [Edit, Write]` plus `writable: [docs/adr]` | repo not writable except that subtree (and the `.beads`/`.git` carve-out) |

`writable:` extras overlapping a path-scoped deny lose (deny-wins, ADR
0001). `posse agent check` warns. Bare `Edit`/`Write` plus a
parametrized rule for a subtree is redundant; warn, do not fail.

No new PID key. No glob compiler.

**2. Realization.** L0 never counts. A hook never counts.

| gate | shims | seatbelt (L2) | container (L4) | codex `-s read-only` |
|---|---|---|---|---|
| bare `Edit`/`Write`/`NotebookEdit` | unrealized (claude/grok) | whole-repo (cwd omitted; `.beads`/`.git`/`writable:` stay) | repo `:ro` plus the carve-outs in §4 | **counts** — equivalent to the bare deny |
| subtree glob | unrealized | trailing `subpath` deny of that directory | `:ro` overlay of that directory | **does not count** — it over-enforces (the persona cannot write the paths the PID left open) |
| non-subtree glob | unrealized | unrealized | unrealized | unrealized |

Concretely: the developer `Edit(docs/adr/**)` on claude/grok needs
`cage: seatbelt` (or `container`); on codex it needs `container`. Codex
cannot nest under our seatbelt (`SelfSandbox`), and its sandbox has no
per-path deny. Dispatch still never passes `--allow-degraded`.

**3. L2 trailing deny.** SBPL last-matching-wins is **MEASURED**
(2026-08-25, macOS 26.4 `sandbox-exec`, realpath'd tree): allow cwd,
then deny a subpath → `touch` / `sed -i` / `python open().write` on the
subpath all `EPERM`; the rest of cwd writes. Deny the subpath *before*
allowing cwd → the allow leaks. So `SeatbeltProfile` appends, after the
allow block:

```
(deny file-write*
  (subpath "<resolved subtree>")
  …
)
```

Same slot as ranger-base-h15 (constitution / gates / hooks carve-out):
one trailing deny block, both lists in it. Paths through
`resolveExisting`, same as today's allow entries.

**4. L4 submounts, and the `:ro` carve-out.** `writable:` is promoted
from "seatbelt extras" to **allow-list paths at L2 and L4**. L4:

- deny-list: repo read-write, then a `:ro` overlay of each denied subtree
- allow-list / bare `Edit`/`Write`: repo `:ro`, then a read-write overlay
  of each `writable:` extra

Overlapping binds (later mount wins) are **ASSUMED** — the Docker probe
was not run this session. The L4 bead's done-when *is* that probe on
this host's engine (Docker 29 / VirtioFS). If it fails, that bead files
`DIVERGED:` and path-scoped rules stay unrealized at `container` rather
than claiming a wall the engine does not hold.

Bare `:ro` currently also blocks bd: SQLite cannot open `.beads` on a
read-only mount, and `.beads/bd.sock` is not a route (VirtioFS
`ENOTSUP`, measured rangerhq-6so). That was left as a design question
in NOTES. **Answered here:** L4's `:ro` repo always carries L2's
carve-outs as read-write overlays — `.beads/` and `.git/` — so a
read-only-repo persona can still claim, comment and close. The
redirect-target extra (store of record outside the session dir) is
ranger-base-rhw at L2; the L4 bead applies the same path as a read-write
submount when it is outside the repo. `.git/hooks` as a further `:ro`
overlay is ranger-base-3c3 / h15, not this ADR.

**5. A hook is not a cage.** A posse-rendered Claude PreToolUse matcher
is deterministic, claude-only, and sees tool-mediated writes only. It
is L0, and we will not even render one: `{deny}` already forwards
`Edit(<glob>)` to `--disallowedTools`, ADR 0001 rejected a per-persona
`settings.json`, and calling a hook a wall would be the classification
this ADR exists to prevent.

## Consequences

- **`parity.go`** grows a subtree-glob parser and a third arm next to
  the bare `Edit`/`Write` case. Today's default arm stops claiming
  `Edit(docs/adr/**)` is an MCP server.
- **`seatbelt.go` / `cage.go`** consume the same parsed list. L2
  trailing deny; L4 overlays; `writable:` honoured at both.
- **Example PIDs** wait until the renderer lands (a parametrized rule
  on today's binary already refuses the launch). Then: architect
  allow-list `writable: [docs/adr]` plus bare `Edit`/`Write` and
  `cage: seatbelt`; developer deny-list `Edit(docs/adr/**)` /
  `Write(docs/adr/**)` and `cage: seatbelt`; qa the same deny-list (it
  is mixed-intent — `harden-suite` writes tests — so it is not the
  verifier shape; reviewer/security already are). Instance PIDs follow
  or don't.
- **`posse agent check`** names non-subtree globs, `Edit(**)` spelled
  as a glob, `cage: shims` plus a path-scoped deny, and a `writable:`
  extra inside a denied subtree.
- **Codex** is the worst runtime for this feature. That is a fact to
  print, not a compiler to write.

## Alternatives rejected

- **Claude PreToolUse as the wall** (the clever one, and the one that
  was offered). Deterministic, small, and already in the CLI. It sees
  `Edit`/`Write` and not `sed -i` / `python` / `tee`; it is claude-only;
  codex fleet-disables hooks; presenting it as a cage would violate ADR
  0002 §4 in the one place an external reader asked us to be honest.
  Native flags stay politeness; the wall is OS-enforced or it is not a
  wall.
- **Render the hook as L0 courtesy anyway.** `{deny}` already ships the
  glob to `--disallowedTools`. A second settings channel re-opens ADR
  0001's rejected sidecar and the project-hooks trust hazard (0002
  amendment). Nothing to render.
- **A new PID key (`unwritable:` / `writes:`).** Two vocabularies for
  one fact. The syntax is already Claude's; `writable:` already exists
  for the allow-list shape.
- **Count codex `-s read-only` as realizing a path-scoped deny.** Safer
  in the deny direction, unusable in the allow direction: the developer
  could not write code. Over-enforcement is not realization of a
  scoped rule.
- **File-extension globs as wall-realizable.** Seatbelt and bind-mounts
  are subtrees. Realizing `**/*.md` would be a matcher in a hook, which
  is the first rejected alternative in costume.
- **Translate globs into each runtime's native surface.** ADR 0002
  already rejected the rule→flag compiler. Path-scoped writes are why.
- **Leave L4 `:ro` without a `.beads` overlay** (today's behaviour,
  "the boundary doing what was asked"). It realizes Edit/Write by also
  breaking claim/comment/close — extra, not the gate. L2 does not do
  that. Match L2.

## Verification (QA's checklist)

1. `posse gates developer` for a PID with `deny: [Edit(docs/adr/**),
   Write(docs/adr/**)]`, `cage: seatbelt`: claude/grok at seatbelt
   `✓ Edit(docs/adr/**) L2 trailing deny`; at shims `✗` *needs cage:
   seatbelt (or container) — a path-scoped write is not a tool-name
   deny*; codex at shims the same `✗` (read-only does not count); at
   container, `✓ L4 :ro overlay` once the probe in (3) holds.
2. Seatbelt, live: `touch docs/adr/x` and `sed -i` on an ADR →
   Operation not permitted; `touch internal/x` succeeds; a `python`
   write to the denied subtree fails too. Profile has the trailing
   deny *after* the cwd allow.
3. Container, live (`RHQ_LIVE_DOCKER=1`): overlapping binds hold in
   both shapes — denied subtree not writable, the rest of a rw repo
   is; `writable:` extra writable on a `:ro` repo; `bd comments add`
   succeeds on that `:ro` repo through the `.beads` overlay; `touch`
   at repo root fails.
4. `posse agent check` on `Edit(**/*.md)` names the glob as
   unrealized-by-construction; on `Edit(**)` tells you to write `Edit`;
   on a `writable:` extra inside a denied subtree, warns deny-wins.
5. A PID with only bare `Edit`/`Write` is unchanged: same matrix, same
   L2 allow-list, same L4 `:ro` (plus the new `.beads`/`.git` overlays).

## Measured versus assumed

| claim | status |
|---|---|
| SBPL last match wins; trailing deny of a subpath after allowing cwd holds against `touch` / `sed -i` / `python` | **MEASURED** 2026-08-25, macOS 26.4 `sandbox-exec` |
| Parametrized `Edit(glob)` currently hits `parity.go`'s default arm | **MEASURED** (the source) |
| Codex `-s read-only` has no per-path surface; `--add-dir` is refused under it | **MEASURED** (rangerhq-5oi, `runtime.go`) |
| L0 forwards the glob via `--disallowedTools` / `--deny` | **MEASURED** (`realizeClaude` / `realizeGrok` pass the PID rule through) |
| Docker overlapping `:ro`/`:rw` binds on VirtioFS do what later-mount-wins says | **ASSUMED** — L4 bead measures before claiming |
| `.beads` rw overlay on a `:ro` repo lets SQLite open | **ASSUMED** (follows from overlapping binds); the *need* is measured (NOTES, rangerhq-6so) |
| Claude PreToolUse does not see `sed -i` | **ASSUMED** from the tool-hook contract (stdin is the tool call); not re-probed; rejected on that contract, not on a live miss |
