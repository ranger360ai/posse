# ADR 0094 — A caged claude seat writes only its own projects/ key, and no one's memory

*Status: accepted 2026-11-13 (ranger-base-l60zr, a restatement under
[ADR 0024](0024-work-product-routing.md) D3 of a design written first in the
instance tree, cited inert as ranger-base-w95hl; built by ranger-base-ajkqx,
ranger-base-r2uxm and ranger-base-fswhs, verified by ranger-base-9xt70) ·
owner: architect · builds on [ADR 0080](0080-posse-on-an-always-on-linux-host-rex-first.md)
D19 (claude's config lives in a dir grant) and
[ADR 0090](0090-a-config-file-a-caged-seat-must-rewrite-is-made-inert-by-removing-its-readers-not-its-writer.md)
(no per-seat config dir)*

## Context

Every posse-launched claude seat shares one `CLAUDE_CONFIG_DIR`, and the
caged write set grants that whole dir (ADR 0080 D19). ranger-base-5632o
already made two members private per seat on bwrap (`shell-snapshots/`,
`session-env/`, in `claudePrivateSessionState`). The CLI's own list of
per-session members of that dir (MEASURED 2026-11-11, claude 2.1.322, static
read of the binary) is `shell-snapshots`, `session-env`, `projects`,
`file-history`, `backups`. This record decides the last three.

| member | who reads it back |
|---|---|
| `projects/<key>/memory/` | every interactive claude whose working-directory key is `<key>`, at session start, with no flag |
| `projects/<key>/<uuid>.jsonl` and `<uuid>/` | claude on `--resume`/`--continue`; posse's cost, turn-outcome and lag readers, as data |
| `file-history/<session>/` | `/rewind` in that session |
| `backups/` | nothing automatic; the CLI prints a restore command naming one of them when its config file is unreadable |

A seat that can write another key's `memory/` or transcripts writes
instructions into a session that is not its own, the operator's included.
The key is not a mystery at render: posse already computes it to find
transcripts (`claudeProjectDir`).

## Decision

**D1. `projects/` is read-only inside a caged claude seat, except the seat's
own key dir, which stays writable in place.** In place, not private: every
reader keeps its path. bwrap renders `--ro-bind projects`, then `--bind` the
own key over it, in the private-members slot after the config-dir grant
(`claudeProjectsBinds`). Seatbelt renders the same list as ordered write
rules: deny `projects/`, allow the own key, deny its `memory/`
(`seatbeltProjectsNarrow`, read off the same list, so the two mechanisms
cannot name different dirs). A link at any of those names refuses the
render.

**D2. `memory/` is read-only at every key, the seat's own included.** The
fleet writes no auto-memory, so nothing is lost, and a seat's own key can
equal the operator's.

**D3. A seat whose cwd is not its own session worktree gets `projects/`
read-only whole.** "Own" is `claudeOwnSessionTree`: a linked worktree at
exactly the launch's session path. The shared checkout, a scratch dir and a
subdirectory all miss it, and the shared checkout's key is the operator's.
MEASURED 2026-11-12, claude 2.1.322: `claude -p` with `projects/` unwritable
exits 0 and answers, writing no transcript. That is a claim about the
installed CLI, so it is a live pin, not a note.

**D4. `backups/` and `file-history/` join the private per-seat members on
bwrap.** Nothing posse runs reads either one, so a seat's own copies live in
its seat cache and leave with it.

**D5. The pins are live, under posse's rendered wall, and each names the
process it protects.** D1–D3 protect the operator's next session start and
next resume: plants into a peer key's `memory/MEMORY.md` and `*.jsonl`, a new
key, the own `memory/` and the mount points are refused with host bytes
unchanged, and the seat's own transcript is written by a shell and by the
real claude against a dead loopback endpoint (no billed turn). D3's premise
runs both ways in one test, so it cannot pass over a claude that writes
nothing anywhere. D4 protects restore and rewind: a peer's members are
absent and unchanged. Each pin was mutated by overlay and went red.

## Consequences

- A claude started inside a seat with another cwd (a probe) writes no
  transcript. Probes take a scratch `CLAUDE_CONFIG_DIR`, as ADR 0090 already
  requires.
- The CLI's periodic transcript cleanup cannot delete outside the own key
  from inside a seat. ASSUMED to be swallowed; uncaged claudes still clean.
- A seat can still rewrite its own transcript. That is its own output, which
  posse already parses as untrusted data.
- Seatbelt's allow-inside-a-deny relies on SBPL's last match winning.
  ASSUMED from the render; the live darwin half belongs to the verify lane.
- Holding a running seat to these binds after launch is `posse wall-watch`'s
  job (ranger-base-0qpsw), not this record's.

## Alternatives rejected

- **A private `projects/` per seat, as 5632o did for its members.** It moves
  every transcript out of the readers' path into a seat cache that worktree
  removal deletes, and cost history goes with it. D1 needs only the key, and
  the key is already computed.
- **A `--session-id` file grant for every seat** (one rule, and the one I
  wanted). It also covers D3's seats and hands posse the exact transcript
  path. Rejected because it needs a pre-created file bind and a replace
  behaviour that was only half measured, while D1 reuses an existing dir
  bind. It was D3's exit hatch, and D3's measurement made it unnecessary.
- **A per-seat `CLAUDE_CONFIG_DIR`, or an overlay.** Rejected in ADR 0090;
  nothing here changes those prices.

## Not decided here

The design this restates priced two more rows. Both were ruled on as risks
by the operator and stay with their beads:

- ranger-base-9xqme
- ranger-base-f30f6

## Not closed here

- ranger-base-zd4d4
- ranger-base-drimd
