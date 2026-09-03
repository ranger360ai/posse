# ADR 0023 — The L3 probe certifies only bytes it wrote: identity at the dispatch path, behavior of our own render

*Status: accepted 2026-08-27 · owner: architect · amends ADR 0002 §3's
probe doctrine ("behavior, not the ownership marker, is what parity
counts") · from ranger-base-vqvl (escape B of ranger-base-flz7)*

> ```sh
> #!/bin/sh
> [ "$RHQ_PERSONA" = probe ] && exit 1
> exit 0
> ```
> Five lines at `.git/hooks/pre-push`. The probe certifies the wall, the
> reconcile refuses to touch a foreign hook, and every real push sails
> through. One write, forever. Pinned green today:
> `TestL3ProbeIsDefeatedByItsOwnSignature` (MEASURED in this tree).

## Context

`probeL3Hooks` (gates.go:1206) execs the hook file at the dispatch path
with a fixed, source-readable signature (`RHQ_PERSONA=probe
RHQ_TOOLS_DENY='Bash(git push:*)' RHQ_GATES_DIR= <hook> origin probe`)
and takes exit 1 as "the wall holds". That is a question the hook's
author answers. A planted hook that refuses only the probe's shape lies
its way to a green parity, and no secret signature fixes it: the probe
must not perform the real operation, so it differs from the real
operation in *some* observable — env spelling, argv, unset
`GIT_INDEX_FILE`, the `posse-prepare-commit-msg-probe-*` temp name — and
every observable is readable by whoever wrote the hook. Behavioral
black-box probing cannot certify adversary-chosen code. This is not
CWE-367; nothing changes between check and use. The check itself is
answered by the adversary.

The same line is also a quiet escalation: the probe execs
session-writable files **in the launcher's unsandboxed context** at
every launch (`CheckParityIn` ← `CreateSession`, `posse gates`). At L2
the cage grants `cwd/.git` (seatbelt.go:98, index refresh) and
`LinkedGitDirs` hands a worktree session the common git dir — MEASURED
writable from a caged session (ranger-base-flz7). A caged session that
writes a hook gets its code run outside the cage on a one-launch fuse.

What is already closed and stays closed: escape A (dispatch directory —
`hooksDir` asks `git rev-parse --git-path hooks`, b065c07) and the
ranger-base-3c3 tamper forms (delete / chmod 644 / truncate /
marker-bearing `exit 0`).

## Decision

Stop asking the file a question. Ask two questions whose answers the
planter cannot forge, and never run bytes we did not just write.

**1. Split the L3 claim: identity ∧ behavior.**

- *Dispatch is ours (identity).* The file at git's own dispatch path
  (`git rev-parse --git-path hooks`, the flz7 fix) is byte-for-byte one
  of the two shapes we prescribe, with the execute bit:
  - our current render (`PrePushHook` / `CommitGuardHook(visibility)`),
    or
  - our prescribed chain dispatcher (`isChainHookDispatcher` — already
    byte-exact modulo a metacharacter-free neighbor filename) **and**
    `posse-<slot>` byte-equal to our render, both `+x`. The dispatcher
    runs our member first and propagates its exit, so the neighbor
    cannot wave through what our member refuses.
- *Our render refuses (behavior).* The probe writes the current render
  to a private temp file and execs *that* under the existing gated env
  script. This half catches what identity cannot: a renderer regression,
  a broken `/bin/sh` — the render itself failing to be a wall.

The wall is Realized iff both hold. Parity wording changes from
"(behavior probed)" to "(render probed, dispatch verified)".

**2. The launcher never execs the on-disk hook.** The only bytes it runs
are bytes it rendered this launch. The planted hook is never consulted,
so there is no signature to discriminate on — the escape closes by
construction, and the escalation in Context closes with it.

**3. Foreign is degraded, said out loud.** Identity mismatch degrades
the launch regardless of what the file would do:

> *Amended 2026-09-02 (ADR 0052, ranger-base-yt6m0): a MANAGED hooks
> path — absolute, outside the repo, unwritable by the uid — is not
> foreign but untouchable. It is not degraded: L3 is realized by a
> per-session hooks dir the session env aims git at, chained into the
> managed one, and the launch line names it either way.*

- no ownership marker → "foreign hook at `<path>` — posse cannot vouch
  for a hook it did not write", plus the chain prescription
  (`chainDispatcher`) as the remedy. This includes a foreign hook that
  genuinely refuses everything: honest, because we cannot tell it from
  the liar above without running it, and we no longer run it.
- marker present, bytes stale (reachable when parity runs without the
  reconcile, e.g. `posse gates`) → "ours but stale — run `posse gates
  install-hooks`". The launch path (herdrback.go install-then-probe)
  makes this unreachable there.

**4. The pins invert — that failure is the signal.**
`TestL3ProbeIsDefeatedByItsOwnSignature` must FAIL when this lands.
Rewrite as two contracts: the discriminating hook is reported down on
both arms with a degraded line naming the foreign file; and the probe
never execs foreign bytes — plant a hook whose body drops a canary file,
assert no canary after the probe. `TestL3ProbeCatchesTamperingAtTheKnownPath`
keeps its meaning (all four forms are identity misses).
ADR 0002 §3's doctrine sentence and the applyL3Probe comment
(parity.go:351) are amended: marker text is still not evidence; full-byte
identity is not a marker — it is the degenerate check that *determines*
behavior instead of hinting at it.

## Named non-goals (each has its own bead)

- **Post-probe TOCTOU** — unchanged; the probe type doc already
  disclaims it. Mid-session tampering is the boundary's problem, not the
  probe's.
- **Use-time redirect** — `/usr/bin/git -c core.hooksPath=<empty> push`
  needs no file write, dodges L1 (absolute path) and moves L3's dispatch
  in the same breath. ASSUMED from flz7's measured precedence, not yet
  run (this session's own L1 deny refused the probe push — the wall
  working is not the wall measured). No probe fixes a use-time fact.
  Filed for measurement + design.
- **Planting containment (flz7's arm a)** — deliberately not decided
  here. A hooks-dir write-deny alone is unsound while `.git/config` is
  writable (redirect to a writable dir, the escape A move), and a config
  write-deny has unpriced legitimate costs (bd sync, remote/config
  edits). Spike first: measure what sessions legitimately write under
  `.git`, then design the carve-out from data.

## Alternatives rejected

- **Probe through real git against a scratch remote** (flz7's arm b as
  written). Rejected on soundness, not just cost: the probe still must
  not perform the real operation, so it is distinguishable — scratch
  remote URL in the pre-push argv, temp repo paths, synthetic refs — and
  each discriminator is readable in the hook's own source. It also runs
  the hostile file (via git) in the launcher context, and pays real git
  operations per launch (cost ASSUMED, never priced — it never got far
  enough to price).
- **Randomized / secret probe signature.** The hook body is readable by
  its planter; randomizing `RHQ_PERSONA` moves the tell to the next
  observable. A secret does not survive an adversary who reads the
  checker's source. (Established in the bead; kept here because it is
  the first thing the next reader will propose.)
- **Trust the ownership marker.** Relitigates ranger-base-3c3: a marker
  is a substring anyone can include. Rejected then, still rejected —
  identity is the whole file, and the whole file is the behavior.
- **Containment as the fix.** `cage: shims` has no file-write boundary
  at all (ADR 0002) and is what most claude sessions run; a perfect
  carve-out would still leave the probe falsely certifying a hook
  planted before the cage existed or by any other tool. Containment is
  defense-in-depth (spiked above), not the answer to a lying witness.

## Consequences

- A legitimately refusing foreign hook that used to certify now
  degrades. The remedy is the chain prescription, which install already
  prints; after chaining, identity certifies again. This is the honest
  trade and it is deliberate.
- The commit-guard's expected render depends on
  `BeadsVisibility(dir)`, so the probe moves onto `App` (or takes
  expected bodies as arguments) — `CheckParityIn` already has one.
- The probe stops being a pure exec of unknown code and becomes two file
  reads, a byte compare, and one exec of our own temp render per slot
  (cost ASSUMED negligible; the exec was already being paid).
- 3c3's doctrine survives refined: *behavior of our bytes, identity of
  the dispatched bytes.* What died is only the belief that exec-ing an
  unknown file tells you anything an adversary didn't want you to hear.

## Verification (runnable; the flz7 shapes)

1. `TestL3ProbeIsDefeatedByItsOwnSignature` FAILS on the fix branch,
   then is inverted per Decision 4.
2. Canary pin: planted hook body `: > <canary>; exit 1` — after the
   probe, both arms report down AND the canary does not exist.
3. Chain pin: dispatcher built byte-exact from `chainHookDispatcher` +
   `posse-<slot>` copied from the render → certifies; flip one byte of
   the member → degrades naming the member.
4. Foreign-refuser pin: unconditional `exit 1` hook, no marker →
   DEGRADED naming the foreign file and the chain remedy.
5. Existing pins stay green: the four tamper forms, escape A's
   `TestL3ProbeFollowsCoreHooksPath` / `TestParityFollowsCoreHooksPath`
   (on a tree carrying b065c07).
