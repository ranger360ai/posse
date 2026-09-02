# ADR 0025 — Enforcement class: "realized" stops meaning two things, and the refusals trail gets one writer

*Status: accepted 2026-08-27 · owner: richard · amends ADR 0002 §3/§4
(bead ranger-base-6uq6, measured in rangerhq-pafo) · amended 2026-09-01:
§4's second bullet, its residual and Verification 2/3 corrected — a spool
cut back to its cursor, above it, or before its first fold is NOT
detected; what the fold guarantees is that the canonical log only grows
(ranger-base-j3r6z, measured under ranger-base-w7h58) · amended
2026-09-01: the Verification list now says which items were executed and
which were only written — Verification 2 has never been run and cannot be
run in this shop (ranger-base-dmrfh)*

## Context

Live verification of the container tier (rangerhq-pafo, posse-cage:latest
on Docker 29.0.1, pinned in
`internal/posse/cageinnerliveqa_test.go` `TestQALiveCageEscapeAttemptsOnAWritableRepo`)
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
- A truncation or rewrite of bytes the canonical log *already holds*
  shows as `size < offset` or a hash mismatch over `bytes[0:offset]`; the
  fold then appends a tamper line to the canonical log
  (`<ts> refusals spool tampered [fold] session=<id>`) and re-folds the
  spool from zero, marked suspect. That is the whole reach of detection:
  the cursor covers the folded prefix and nothing past it. A spool cut
  back to exactly the cursor, to any length above it, or before its first
  fold ever ran (no cursor, so no comparison at all) folds as "no new
  lines" — no tamper line, no marker (MEASURED 2026-09-01 at 718ebf0,
  ranger-base-j3r6z; pinned green as the documented residual in
  `internal/posse/refusalfold_qa_test.go`, which goes red the day
  detection grows so this paragraph gets rewritten with it). **What
  decides this design is that the canonical log can only grow from inside
  a cage** — held by the mount that is not there — and that everything
  already folded is beyond the reach of the process it records. The
  tamper line is a bonus against the clumsy erasure, not a promise that
  erasure leaves evidence.
- Folds run where a host loop already runs: dispatch pass start /
  auto-reap sweep, session close, relaunch. No new daemon.

Residual, stated rather than wished away: lines appended and then
truncated *between* two folds are unrecoverable **and the loss is not
detected**. An inside process cannot read the cursor and does not need
to: it stats the spool, does the thing it does not want recorded, and
truncates back to the size it read — the folded prefix is byte-identical
and the fold sees nothing. That is the default arm; the detected arm is
the race where a fold landed inside the window. The window is the sweep
cadence. Closing it means one of the two mechanisms rejected below (an
append-only mount the inside cannot shorten, a live tail) — re-open when
the threat model does, not before (ranger-base-j3r6z: the trail is a
single operator's record of operator-authored beads, ADR 0002; nothing
scores a persona on it). The host tier is unchanged: an unconfined persona can edit the canonical
file no matter who appends, so a spool buys nothing there. In class
vocabulary: the trail is itself cooperative at `shims` and
enforced-append at `container` once this lands — enforced that the
canonical file only grows, cooperative that a given line reaches it
before the next fold.

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

**Execution status, measured 2026-09-01 (ranger-base-dmrfh).** Items 2
and 4's *pins* live in `internal/posse/cageinnerliveqa_test.go`, which
runs only against a real engine and a built `posse-cage:latest`. This box
has neither, and by the operator's ruling of 2026-08-30
(ranger-base-6mz7) it never will until an off-laptop cleanroom exists —
Docker was removed that day (3.8GB wired on a 16GB box) and no engine
replaced it: no `docker` daemon, no colima/podman/lima/OrbStack/nerdctl,
no Apple `container`. The `docker` CLI is still on PATH (Homebrew 29.1.1,
client only), which is why the tier does not announce itself as absent —
see the residual below. Run here, the whole file skips in ~0.1s per test —
all five tests, re-run 2026-09-02 under ranger-base-uuj27:

    RHQ_LIVE_DOCKER=1 go test -timeout 25m ./internal/posse/ \
      -run 'TestQALive(Cage|Parity|UnknownSocket|RootInside)' -v
    --- SKIP: TestQALiveUnknownSocketRefusesTheLaunchItself (0.13s)
    --- SKIP: TestQALiveParityCatchesARealImageWithNoInnerPosse (0.13s)
    --- SKIP: TestQALiveCageEscapeAttemptsOnAWritableRepo (0.13s)
    --- SKIP: TestQALiveCageMountBoundaryIsDeepAndTheNotebookSurvives (0.13s)
    --- SKIP: TestQALiveRootInsideCannotRemountTheBoundaryWritable (0.13s)

The `-run TestQALiveCage` this paragraph carried on 2026-09-01 reaches two
of those five; the other three are named for what they attack rather than
for the cage. The conclusion is unchanged — every one of them skips — but
the earlier command was never evidence about the whole file.

So read the list as: 3 and 5 hold and were run; 1 was run at `shims` and
not at `container`; 4's *measurement* stands but was made outside a cage;
2 has never been run by anyone, on any box, and is written from the code.

1. `posse gates <p>` prints the class per realized gate: at `shims`,
   `Bash(git push:*)` → cooperative; at `container` with a built image,
   `Edit` → enforced (L4 mount) while `Bash(git push:*)` stays
   cooperative with the effect note. **The `shims` arm is executed. The
   `container` arm is not executable here**: with no engine, `posse gates`
   for a PID that declares `cage: container` prints no container row at
   all — it renders `shims` and `seatbelt` and says `✗ cage: PID demands
   container, launching at shims`.
2. **NEVER EXECUTED — this is the claim, not a measurement.** In the live
   cage QA: `: > "$RHQ_GATES_DIR/refusals.log"` inside no longer shrinks
   the host canonical; the next fold appends a tamper line naming the
   session — because the QA folded once before the truncate. The same
   gesture before a spool's first fold folds as empty and says nothing
   (ranger-base-j3r6z). Item 3's hermetic pins exercise
   `FoldRefusalsSpool` directly and carry the fold half of this; what they
   cannot reach is the half only a cage can answer — that the inner shims
   write where `CageMounts` now points them, which is what §4 actually
   claims. Settling it needs the off-laptop cleanroom of
   ranger-base-6mz7 and then the command above; until then §4's
   truncate behaviour is a design intention with hermetic support, not a
   verified property, and nothing in this tree should be read as saying
   otherwise. The QA file said otherwise until 2026-09-02: its header
   claimed one 2026-08-27 measurement for the whole file, over a §4 arm
   written four days later by d67cad9. Fixed under ranger-base-uuj27 —
   that header now qualifies per arm, and the truncate arm, the escape
   probes (044bed2) and the staleness guard (d695fa4) each carry a NEVER
   RUN line where they sit.
3. Two folds over an unchanged spool append zero new lines; a spool
   truncated below its cursor and refilled to the same size folds as
   tampered (the hash, not the offset, is what catches it). A spool
   truncated to or above its cursor and refilled folds as new lines,
   untampered — the hash covers only the folded prefix. **Executed** —
   hermetic, no engine needed; mutation-checked under ranger-base-w7h58.
4. `/usr/bin/git push --no-verify` and `git -c core.hooksPath= push`
   are measured and pinned either way in the same QA file. **The
   measurement is executed, the pin is not**: it was made in an ungated
   non-container scratch (ranger-base-3csb, git 2.39.3), and that stands
   on its own; the QA file that would re-measure it inside a cage skips
   here with the rest.
5. ADR 0002 §3's L3 row no longer claims `env -i`; a grep for the claim
   finds only this history. **Executed, re-run 2026-09-01** — zero hits
   for the struck wording, and every surviving `env -i` in ADR 0002 either
   says it *defeats* the arm or records the history.

### Residual (fixed, ranger-base-1mu9r): the skip named the wrong reason

`App.ContainerAvailable` is `exec.LookPath("docker")` — a PATH check, not
a liveness check — and `App.CageImageBuilt` runs `docker image inspect
{image}`, which against a dead daemon exits 1 with a socket-connect error
rather than "no such image". A control arm settles it: `docker image
inspect definitely-absent-xyzzy:latest` prints byte-identical output to
the same call for `posse-cage:latest`. So on this box every reader of that
pair reports the engine's absence as a missing image and gives advice that
cannot be followed. Both the QA file's skip and `posse cage`'s status line
read *image not built — run `posse cage build`*, and a build needs the
same engine the box does not have. Neither hangs and both are honest about
not running, so ranger-base-6mz7's done-when holds; what neither did is
name the ruling.

Fixed under ranger-base-1mu9r by splitting the question in two. The engine
gains a `live:` template (`docker version` for the built-in) and
`App.CageEngineLive` runs it; `App.CageNotReady` asks binary, then image,
then — only once the image probe has said no — whether anything was there
to answer it, and returns the first reason that is true. So the surfaces
above now say that the engine is on PATH and nothing answers it, and that
`posse cage build` is unavailable for the same reason rather than being
the advice. The liveness probe deliberately cannot change *whether* a
launch or a test proceeds, only *why* it did not: it runs after the verdict
is already settled, so an engine that answers `live:` badly cannot refuse a
host that works.
