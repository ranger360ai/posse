# ADR 0025 — Enforcement class: "realized" stops meaning two things, and the refusals trail gets one writer

*Status: accepted 2026-08-27 · owner: richard · amends ADR 0002 §3/§4
(bead ranger-base-6uq6, measured in rangerhq-pafo)*

## Context

Live verification of the container tier (rangerhq-pafo, posse-cage:latest
on Docker 29.0.1, pinned in
`internal/rhq/cageinnerliveqa_test.go` `TestQALiveCageEscapeAttemptsOnAWritableRepo`)
measured two things ADR 0002's parity table promises and the gates do not
hold. Neither is an implementation defect — the code does what §3 says —
but an operator reads that table as a guarantee.

1. **An emptied environment defeats L1 and L3** (MEASURED). Both layers
   carry the deny list in the environment: L1 is a PATH shim and `env -i`
   drops PATH; the pre-push hook opens with
   `[ -n "$RHQ_TOOLS_DENY" ] || exit 0` (gates.go:683) and fails open by
   design — it is installed per REPO, one file shared by every persona
   that checks the repo out, so the env is its only per-persona carrier,
   and an empty env is also exactly what the operator's own shell looks
   like. Yet §3's L3 row claimed `git push` is refused "even via absolute
   path / `env -i` / a subprocess" — the one place in the tree that says
   so (NOTES.md's L3 section and the hook's own doc comment already say
   the opposite). The claim was an assumption; it is now measured false.
   Cheaper bypasses were always in reach: `git push --no-verify` skips
   pre-push entirely and `git -c core.hooksPath=` points hooks nowhere
   (both ASSUMED from git's documentation; QA pins them below).

2. **The audit trail is writable by the thing it audits** (MEASURED).
   `gates/<p>/refusals.log` is bind-mounted into the cage read-write
   because the inner shims append to it (cage.go:340), and a bind mount
   cannot distinguish an append from a rewrite: `: > refusals.log` inside
   shrinks the HOST file — a caged persona can erase the record of its
   own refusals. It holds with the repo `:ro` too.

What held, so this is scoped and not an alarm (both MEASURED, same file):
root inside the container cannot remount the `:ro` boundary writable
(EPERM), and `env -i /bin/touch` through a `:ro` mount is refused — the
kernel does not read the environment.

The diagnosis in one sentence: **the parity table uses one word,
"realized", for a gate the kernel holds against a process that evades and
a gate a process holds against itself.**

## Decision

**1. Every realized gate carries an enforcement class.**

- `enforced` — held outside the gated process, survives an adversarial
  one: L2 seatbelt, L4 mount boundary, the egress `--internal` network +
  proxy, codex `-s read-only` (OS sandbox).
- `cooperative` — held in-process, holds the runtime's ordinary path and
  nothing stronger: L1 shims, L3 hooks, the gate shell. Defeated by an
  emptied environment, `--no-verify`, `core.hooksPath`, or editing the
  slot (the shims-tier TOCTOU residual NOTES.md already names).

`Parity.Realized` values carry the class; `posse gates <p>`, the
refusal/degraded listing, and session meta print it. **No launch behavior
changes**: class never refuses a launch and `--allow-degraded` is not
involved — `cage:` remains the knob for demanding the stronger tier. The
threat model ADR 0002 already states (a single operator running
operator-authored beads) prices cooperative as the fleet floor for shell
verbs; what changes is that the floor now says its name where the
operator reads the guarantee.

**2. ADR 0002 §3's L3 row is corrected in place.** "even via absolute
path / `env -i` / a subprocess" becomes "via absolute path or a
subprocess that keeps its environment; an emptied environment,
`--no-verify`, or `core.hooksPath` defeat it — cooperative class". The
row was self-inconsistent with the hook's own first line.

**3. The push *effect* has an enforced realization; say where it comes
from.** At `cage: container` the verb gate stays cooperative, but the
effect of a push dies at an enforced layer when the launch is configured
for it: a path remote inside the mounts is stopped by `:ro` (which the
tier grants only when the PID denies Edit/Write), and a network remote is
stopped by the proxy unless `egress:` names its host. A PID that denies
`git push`, mounts the repo read-write, and can reach its remote has only
the cooperative gate — that is the measured escape. Parity prints this as
a note on the class line, not as a computed claim: the launcher does not
know the remote's host, so it must not pretend to.

**4. Single writer for the refusals trail.** The canonical
`gates/<p>/refusals.log` is never mounted into a cage; its only writers
are host-side processes. The inner render appends to a per-session spool
(`state/cages/<p>/<session>/refusals.log`, the one file mounted rw), and
the host **folds** spool → canonical:

- A cursor per spool (byte offset + SHA-256 of `bytes[0:offset]`) lives
  under `state/gates/<p>/spool-cursors/<session>` — on the host, never
  mounted. Fold = read from offset, append lines to the canonical log
  prefixed `session=<id>`, advance the cursor. At-least-once with an
  idempotent cursor: re-running a fold appends nothing twice.
- Truncation or rewrite from inside shows as `size < offset` or a hash
  mismatch; the fold then appends a tamper line to the canonical log
  (`<ts> refusals spool tampered [fold] session=<id>`) and re-folds the
  spool from zero, marked suspect. **The erasure attempt becomes
  evidence** — the property that decides this design.
- Folds run where a host loop already runs: dispatch pass start /
  auto-reap sweep, session close, relaunch. No new daemon.

Residual, stated rather than wished away: lines appended and then
truncated *between* two folds are unrecoverable — the truncation is
detected, the content is not restored; the window is the sweep cadence.
The host tier is unchanged: an unconfined persona can edit the canonical
file no matter who appends, so a spool buys nothing there. In class
vocabulary: the trail is itself cooperative at `shims` and
enforced-append at `container` once this lands.

## Consequences

- `parity.go`: `Realized` gains the class (spelling is the builder's —
  a small struct or a tagged string — but the class must be typed enough
  that `posse gates` and meta print it without parsing prose).
- `cage.go` stops mounting the canonical log (the :340 mount entry);
  `cageinner.go`'s refusal destination points at the spool.
- The fold, with cursor + hash, wired into the existing sweep, session
  close, and relaunch paths.
- QA: `TestQALiveCageEscapeAttemptsOnAWritableRepo`'s truncate probe
  flips — the host canonical must not shrink, and the next fold's tamper
  line is asserted; new pins for `git push --no-verify` and
  `git -c core.hooksPath= push` (today ASSUMED to escape, from git docs).
- NOTES.md's posture section adopts the class vocabulary, and the two
  "it cannot see through `env -i` … that is the container tier's job"
  sentences (NOTES.md L3 section, gates.go PrePushHook comment) sharpen
  to: the container stops the *effect* via mount/egress where configured;
  the verb gate stays cooperative at every tier.
- Implementation beads: parity class + docs + QA pins; refusal spool +
  fold + QA flip. Both `-a dinesh -l code`, ids on ranger-base-6uq6.

## Alternatives rejected

- **Render the deny list to a file the hook reads when the env is empty**
  (the bead's option b, the clever one). Without the env the shared
  per-repo hook cannot tell whose session it is — `RHQ_PERSONA` rides
  the same env `env -i` strips — and every non-env signal it could read
  (worktree cwd → marker file, a per-worktree gitdir file) is inside the
  persona's own write reach at the shims tier, so the fallback is
  cooperative again, one file further away. And a perfect env-independent
  hook still loses to `--no-verify` and `core.hooksPath=`, both cheaper
  than `env -i`. Hardening one door in an open field buys back the false
  claim, not a wall. (Cost priced, not built: per-worktree deny files +
  a hook that resolves them ≈ a rendered state machine in POSIX sh,
  ASSUMED ~100 lines, all of it defeasible by its own audience.)
- **Fail closed on an empty env.** An empty env is what the operator's
  own shell looks like — ADR 0002 verification 13's dual — so this
  refuses the human to inconvenience the persona.
- **`chattr +a` / an append-only mount for the log.** Needs
  CAP_LINUX_IMMUTABLE inside and driver support nobody has measured here
  (VirtioFS behavior unverified) — the rangerhq-3nxk rule: no unrun
  mechanism inside a decision. Re-open if someone measures it.
- **A live `tail -F` folder (window → ~0).** A per-session host daemon
  to own, restart, and reap, to shrink a window the threat model prices
  low. The upgrade path if `refusals.log` ever feeds an adversarial
  scorecard; not now.
- **Log via a host service reached from inside (herdr socket).** §3 of
  ADR 0002 keeps that socket out of the cage on purpose; widening a
  fleet-wide capability to protect a log inverts the trade.
- **Keep the rw mount and document the hole.** The table would still say
  "realized" about an audit trail its subject can erase, and the fold
  costs roughly a page of code in loops that already run.

## Verification

1. `posse gates <p>` prints the class per realized gate: at `shims`,
   `Bash(git push:*)` → cooperative; at `container` with a built image,
   `Edit` → enforced (L4 mount) while `Bash(git push:*)` stays
   cooperative with the effect note.
2. In the live cage QA: `: > "$RHQ_GATES_DIR/refusals.log"` inside no
   longer shrinks the host canonical; the next fold appends a tamper
   line naming the session.
3. Two folds over an unchanged spool append zero new lines; a spool
   truncated and refilled to the same size folds as tampered (the hash,
   not the offset, is what catches it).
4. `/usr/bin/git push --no-verify` and `git -c core.hooksPath= push`
   are measured and pinned either way in the same QA file.
5. ADR 0002 §3's L3 row no longer claims `env -i`; a grep for the claim
   finds only this history.
