# ADR 0031 — `posse init` joins the operator fence, keyed on the target home, not the persona

*Status: accepted 2026-08-28 · owner: richard · extends ADR 0015 §3
(bead ranger-base-x26u, measured in ranger-base-h7cd) · amended
2026-09-02: §6 — a PROMOTED home is nobody's to init, the operator's own
hands included (ranger-base-39jnl)*

## Context

MEASURED (laurie, on h7cd's repro): `posse init` honours `RHQ_HOME` over
`HOME`, and every session the launcher creates exports
`RHQ_HOME=<the operator's live home>` (herdrback.go:1570) so that every
rhq/bd tool inside addresses the right instance. The combination: a
persona session running `posse init` — even with `HOME` sandboxed, even
from a release tarball extracted outside any checkout — seeds recipes,
skills and example PIDs into the live home, and `retireExamplePIDs`
moves PIDs out of `agents/` — a routing change to the live crew — with
no operator in the loop. h7cd (5fbb28c) removed the manifest half: init
no longer arms §3's launch verify over a home nobody promoted. The
writes themselves remain, and they are this ADR.

The two commands already fenced (promote.go:336, refresh.go:152) refuse
blanket under `RHQ_PERSONA`. That shape is wrong for init, because the
harm profiles differ categorically: promote and refresh are
*ratification acts* — the operator's regardless of what they point at —
while init is a *seeding act* whose harm is entirely a property of WHICH
home it writes. `RHQ_HOME=<scratch> posse init` from a persona session
is how QA seeds fixtures and how h7cd itself was measured; a blanket
refusal kills that, and a PID deny line (`Bash(posse init:*)`) kills it
too, since the L1 shim sees the argv and cannot see what `RHQ_HOME`
resolves to. The distributed-systems canon's load-bearing line applies:
*the resource is the last line of defense, not the lock* — the check
belongs in the binary at the write site, where the target home is a
resolved path and not a guess.

What was missing was not policy but a fact: at init time the process
cannot tell the live home from a scratch one, because the only carrier
of "the home this session belongs to" is `RHQ_HOME` itself, and
overriding it for a scratch run erases the evidence.

## Decision

**1. The launcher stamps every session with its origin home.**
Alongside `RHQ_HOME`, the session env gets
`RHQ_LAUNCH_HOME=<a.Home>` (const `EnvLaunchHome`, beside `EnvPersona`).
Unlike `RHQ_HOME` it is not an address, it is a *record*: nothing
resolves paths through it, and a persona overriding `RHQ_HOME` for a
scratch run leaves it standing. It is added to `CageEnvNames`
(cage.go:386) so caged sessions carry it like the rest of the identity
stamp. This is not a new mechanism class — it is one more line in the
identity stamp the launcher already writes (`BD_ACTOR`, `RHQ_PERSONA`,
`RHQ_PERSONA_DIR`).

**2. init refuses to write the home it was launched from.** At the top
of `initFrom` (the single entry every init write passes through), when
`EnvPersona` is set:

- Resolve the target (`a.Home`) and the origin (`RHQ_LAUNCH_HOME`);
  refuse when the target *is* the origin or lies *inside* it. The
  resolution must survive a symlinked home (`~/.config/rhq` onto the
  instance repo, the pre-cutover shape ADR 0015 §2 names) and a target
  that does not exist yet (the ordinary throwaway case): resolve the
  longest existing prefix of each side through symlinks, then compare
  cleaned paths.
- Origin absent while `EnvPersona` is set: refuse — **fail closed**. A
  session that cannot prove where it came from does not get to write
  anywhere it might have come from. The refusal names both exits:
  relaunch the session (a post-0031 launcher stamps it), or the
  operator runs init.
- Both refusals name this ADR and print the working form:
  `RHQ_HOME=<scratch> posse init`.

**3. No PID deny line — deliberately.** The divergence from
promote/refresh's "fenced the same way twice" is the point of this ADR:
the second fence (a deny in every crew PID) cannot express "this target,
not that one", so adding it would re-impose the blanket cost the binary
fence exists to avoid. The fence list in prose stays at promote and
refresh; init's fence lives only where the target is knowable.

**4. Enforcement class: `cooperative`** (ADR 0025). Env-carried and
in-process, defeated by env surgery — the same class promote and
refresh already are, and the right class for a guardrail against
*accidents of precedence*, which is what was measured. Caged runtimes
already hold the enforced-class version of this fence for free:
`RHQ_HOME` is not mounted into the cage (cageinner.go), so the live
home is unreachable from inside regardless. This ADR covers the uncaged
runtimes, which is where the leak was measured.

**5. Promote and refresh are not narrowed.** Their blanket refusals
stand: a ratification act is the operator's whatever it points at, and
"promote a scratch home from a persona" is a use case nobody has
presented. If one arrives, the origin stamp is sitting there; that is a
one-line amendment, not a design.

**6. A promoted home is refused whoever is typing** *(amended
2026-09-02, ranger-base-39jnl)*. §2 keys on `EnvPersona`, so it is a
fence around SESSIONS — and the leak it was written for arrived again
through the door it does not cover. MEASURED 2026-09-02: the operator
ran the work-laptop install steps on the fleet box by hand; `posse init`
re-seeded `examples/` and `secrets/` under a constitution `posse
promote` owns, in the same hour a shadowed binary took dispatch down.

So `initFrom` refuses, before its first write, when the target home
carries a manifest that is not `seeded`. The line is `seeded` and not
"has a manifest" on purpose:

- A **seeded** manifest is a fresh install's anchor with no commit
  behind it, and re-running init on one is the generics upgrade
  INSTALL.md §7 advertises: it fills gaps and re-stamps for exactly what
  it wrote. Unchanged.
- A **promoted** manifest is a claim about a commit that only `posse
  promote` may restate. Init has no re-stamp available to it there, so
  even a successful run leaves the launch verify refusing every
  dispatched launch — which is `ranger-base-pith`, reached from inside
  init and unfixable from there. Refusing is the fix pith could not
  reach: the copy simply does not happen.
- An **unreadable** manifest keeps its old behaviour. Posse cannot say
  whether that home was promoted or seeded, and the existing arm on the
  way out already names it.

Same enforcement class as §2 (`cooperative`, ADR 0025) and the same
shape of refusal: it names the manifest, the commit behind it, `posse
promote`, and the working form `RHQ_HOME=<scratch> posse init`.

## Consequences

- A persona's bare `posse init` becomes a loud refusal instead of a
  silent live-home write; scratch seeding is unchanged; the operator's
  own init is unchanged — except on a promoted home, which §6 refuses
  for every caller (amended 2026-09-02, ranger-base-39jnl).
- A session launched by a pre-0031 launcher and running a post-0031
  binary refuses *all* init until relaunch (origin absent, fail
  closed). The window is the hours between promoting the binary and the
  fleet's next relaunch, the refusal says exactly what to do, and the
  conservative side of that trade is the correct one here.
- QA tests that set `RHQ_PERSONA` and drive init must now spell their
  origin (set `RHQ_LAUNCH_HOME` to a path that is not the target, or
  leave the persona unset) — per test, hermetically.
- The origin stamp is a general primitive: any future command whose
  harm is target-relative ("write the live X from a session born of X")
  compares against it instead of growing a blanket refusal.

## Alternatives rejected

- **Blanket refusal under `RHQ_PERSONA` + PID deny (the promote
  shape).** The clever-simple one. Kills throwaway seeding — the
  measured QA path and the way h7cd was diagnosed — to stop a write
  that only matters for one specific target. Fence shape should follow
  harm shape.
- **Refuse when the target is the *default* home path.** No new env
  var, but leaky exactly where it matters: an operator with a custom
  `RHQ_HOME` gets no fence at all, and a QA run whose sandboxed `HOME`
  makes the scratch target *look* default gets refused. Wrong on both
  sides of the line.
- **Derive the origin from `RHQ_GATES_DIR` / `RHQ_SKILLS_DIR` path
  shape** (both already in the env, both `<home>/...`). Free, and
  wrong the way the L3 hooks-dir derivation was wrong (flz7): a
  derivation encodes today's layout, and the day the layout moves, the
  fence silently unfences. Ask; don't derive. The record costs one env
  line.
- **Ask the target home whether it is live** (scan `state/` for
  sessions, look for a lock). Heavier, TOCTOU-shaped, and answers the
  wrong question: a live home between dispatch passes has no session
  running, and it is still the operator's.
- **Fail open when the origin stamp is absent.** Preserves init in
  stale sessions through the upgrade window, at the price of making the
  fence's whole guarantee conditional on a var nobody can see missing.
  The failure this ADR exists for was silent; its fence does not get to
  fail silently.
