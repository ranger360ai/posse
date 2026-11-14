# A behaviour-only hook probe, and the exit code that already meant something else (ranger-base-apf87)

ADR 0092 D4 amendment 3 asked for one read-only verb — `posse gates
render-probe [dir]` — so that `posse upgrade` could judge the L3 hook wall by
BEHAVIOUR (does this binary's own render still refuse what it gates?) instead
of by IDENTITY (are the bytes at a repo's dispatch path this binary's
render?). Identity is stale by design after any install that changes the
render, which is what made the first writing of D4 refuse exactly the
landings that change the wall.

Two things this build measured, 2026-11-13, on the D1 host (linux
7.2.15-1-omarchy, posse 0.5.3 at `34bbe053`/`00a9f2a3`).

## 1. The ADR's one ASSUMED: the answer does not move with the installed hooks

MEASURED. **Seven readings, all exit 0, byte-identical verdict lines.**

Arm A asked the verb of the REAL source checkout (`~/src/posse`) and wrote
nothing there. The hook state was moved rather than the files: git's dispatch
path was redirected with `core.hooksPath` through `GIT_CONFIG_COUNT` /
`GIT_CONFIG_KEY_0` / `GIT_CONFIG_VALUE_0`, which the probe's child inherits
(`protocolEnv` replaces PATH and nothing else).

| # | the source repo's hook state | exit | verdict line |
|---|---|---|---|
| A1 | as installed (fresh) | 0 | `render-probe: ~/src/posse · pre-push refuses · prepare-commit-msg refuses` |
| A2 | stale — `exit 0` bodies | 0 | identical |
| A3 | absent — an empty dispatch dir | 0 | identical |

Arm B asked it of a scratch checkout whose hooks really were written and
removed on disk:

| # | the scratch repo's hook state | exit | verdict line |
|---|---|---|---|
| B1 | no hook installed at all | 0 | `… · pre-push refuses · prepare-commit-msg refuses` |
| B2 | freshly installed by `posse gates install-hooks` (2 slots) | 0 | identical |
| B3 | installed but overwritten with `exit 0` bodies | 0 | identical |
| B4 | both slots removed again | 0 | identical |

So the ADR's residual stands as it was accepted: the verb cannot see a render
that refuses correctly from a temp file but was INSTALLED wrong (bad mode, bad
chain). That is the operator's re-render, or the next launch's identity ∧
behaviour probe in that repo.

"The same answer every time" is only worth something if the probe can answer
differently at all, so the pins carry the other two exits beside this one:
`TestRenderProbeAnswerDoesNotMoveWithTheInstalledHooks` (the table above, as a
pin), `TestRenderProbeExitsTwoWhenItCannotAsk`,
`TestRenderProbeExitsTwoOnSomethingThatIsNotACheckout`, and
`TestExecOwnRendersAskedTellsADeadRenderFromAnUnaskableProbe` for exit 1's
arm, which through the verb alone needs a binary whose own render is broken.

## 2. The ADR said an old binary answers "neither 0 nor 1". It answers 1.

MEASURED at `34bbe053`, the commit before the verb existed:

```
$ posse gates render-probe
posse: no such agent: render-probe (looked in /home/posse/homes/ranger/agents)
$ echo $?
1
```

`posse gates`' sub-verb ladder falls through to `LoadAgent`, and an unknown
persona is a `die()` — exit 1. That is the SAME code the new verb uses for
"one of my renders does not refuse".

Read as the amendment was written, the trigger excuses a non-zero answer from
the new binary when the old binary gave "that same non-zero answer". So the
FIRST install after this verb lands — old binary exits 1 because the verb is
not there, new binary exits 1 because its render is broken — would have been
read as a box property, reported, and left installed. The missed rollback is
the expensive direction of the two: a false rollback costs one re-run of the
verb, a missed one leaves a broken binary dispatching a live fleet.

The intent is kept and the realization moved one inch: **"was it asked" is a
mark on stdout, not an exit code.** Every line the verb prints opens with
`render-probe:` (`posse.RenderProbeMark`), and `upgradeRead` records
`RenderAsked` only when a line of the output does. The three exit codes are
exactly the three the ADR names. The mark is the fourth state the ADR thought
the exit code already gave it.

The trigger, as built:

| old binary | new binary | verdict |
|---|---|---|
| asked, 0 | 0 | stands |
| asked, 0 | 1 or 2 | ROLL BACK — named in the report |
| asked, N≠0 | the same N | stands, with a note that it is a property of the box |
| not asked (no mark) | 0 | stands — the first install of the verb |
| not asked (no mark) | 1 or 2 | ROLL BACK — "the old binary did not have the verb, so nothing excuses it" |
| anything | no mark | ROLL BACK — a binary without the verb cannot be judged |

That last row is why `TestQAGatesRenderProbeVerbIsInTheBinary` exists and
runs the real binary. The OLD binary's code is what calls this verb, so the
verb is part of the interface every future binary owes the one installed
today: a release that dropped it would read as "not asked", roll back, leave
the old binary in place — and do it again on every later attempt.

## Two things worth keeping about the code

`execOwnRenders` is two-valued on purpose and stays that way: for a LAUNCH, a
probe that could not be taken and a render that does not refuse are the same
answer (degrade). The new three-valued body, `execOwnRendersAsked`, is what
this verb needs, and its error text is the reason the verb prints. While
splitting them: an exit code the probe script cannot produce (outside 0-3,
one bit per slot) now reads as "could not ask". It used to read as BOTH slots
fine — `4&1` and `4&2` are both zero — which is the wrong direction for a
number that says the script never reached its own `exit`.

The verb takes `dir` as an argument AND runs in it, and `upgradeRead` passes
the source repo for both. That is ADR 0092 D4 amendment 2's defect one verb
along: a default resolved inside the probed binary could differ from the one
resolved by the process doing the asking, and then the two sides of the diff
would be about two trees again.
