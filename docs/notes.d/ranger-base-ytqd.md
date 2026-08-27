## `bd dep add -t relates-to`, twice, is a symmetric pair (ranger-base-ytqd)

QA correction to the durability claim in the *ranger-base-nusr* section of
NOTES.md, and to the header comments of `scripts/prune-bd-relates-to.sh` and
`scripts/verify-bd-dep-safety.sh`, all of which read:

> Exactly one verb plants a pair: `bd dep relate` (and its deprecated alias
> `bd relate`). `bd dep add -t relates-to` writes a single row and is harmless.

The first sentence of the measurement is right: one `bd dep add -t relates-to`
writes one row. The inference drawn from it is not. **Two of them, in opposite
directions, plant a full symmetric pair** — bd 0.49.1's cycle check does not
refuse the reverse `relates-to` edge. Measured 2026-08-27 on a `VACUUM INTO`
snapshot of the live store, `--no-daemon`, using only `bd dep add`:

    bd dep add ranger-base-s2bq ranger-base-il14 -t relates-to   -> ✓ 0s
    bd dep add ranger-base-il14 ranger-base-s2bq -t relates-to   -> ✓ 0s
    sqlite3 … "select … where type='relates-to'"                 -> both rows

Replaying all ten of the pruned pairs that way — twenty `bd dep add` calls, no
`bd dep relate` anywhere — restores the store to the exact pre-prune shape
(`--gate` names the same 13 nodes) and brings the defect back verbatim:

    bd --no-daemon create "…" --deps discovered-from:ranger-base-okbr
      -> killed at 90s; issue committed (ranger-base-e4l2), dependencies []

So `Bash(bd dep relate:*)` / `Bash(bd relate:*)` in `.claude/settings.json` do
not, on their own, make the prune hold: the reachable path runs through a verb
that is allowed and that the docs describe as harmless. What holds the store at
zero is `scripts/verify-bd-dep-safety.sh --gate` **being run** — which is
`ranger-base-z3s3`, still open. Fix is tracked on **ranger-base-uw8g**.

Two things that are confirmed, and are not part of this correction:

- **The prune itself is good.** Live store is 0 pair-nodes / 0 unsafe targets,
  `relates-to` rows are gone (0 of 786 edges), the twenty provenance comments
  are on both sides of all ten pairs, and `create --deps` lands its edge in
  well under a second.
- **The pair really is the cause.** Restoring the ten pairs to a pruned
  snapshot reproduces the 90s hang; removing them removes it. Note that *one*
  isolated pair is not enough — a single `okbr <-> r64` pair left `create
  --deps discovered-from:ranger-base-okbr` at 0.4s with the edge present, while
  `--gate` still (correctly, conservatively) flagged it.

A mixed-type 2-cycle — e.g. `A -> B discovered-from` plus `B -> A blocks` —
would also make the walk diverge and is *not* what the gate's
`a.type = b.type` join looks for, but it is unreachable: bd refuses the second
edge with `would create a cycle`. Measured on the same snapshot. The gate's
blind spot there is theoretical; the `relates-to` one is not.
