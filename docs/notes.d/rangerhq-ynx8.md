## Dispatch fails closed on foreign sessions (rangerhq-ynx8)

Every guard in dispatch asks a session a question. A **foreign** row — a
live herdr workspace posse holds no session meta for, which `Resolve` finds
by label — answers all of them with a zero value, and each guard reads the
zero as permission:

| guard | reads | on a foreign row |
|---|---|---|
| `crewHeld` (ADR 0008) | `s.Crew` | false — no meta, so no mark |
| working/blocked | `s.Status` | herdr's live status, `idle` for a settled pane |
| `personaActive` | `s.Agent` | `""`, so the "not this persona" skip abstains |
| `RunHolder` (ADR 0011 §3) | the run record | absent, so the name patterns decide |

So the shield ADR 0008 exists for is not false on that row — it is *absent*,
and absence read as permission is the definition of failing open. The
reachable shape is the one rangerhq-ggm8 named: a session meta is pruned
(hand cleanup of `state/herdr/`, an older binary, a scratch-server op) while
the workspace it named lives on. Crew sessions are exactly the at-risk
class, because the operator steps into a session that is *already* named
`<persona>-<repo>-<bead>` by construction — so the name match after the wipe
is guaranteed, and the moment the mark is lost the operator's own
conversation becomes fleet-promptable again.

What lands is the splice: `LaunchBead` adopts the foreign row as the bead's
holder, `launchSession` skips `CreateSession` because `Resolve` succeeded,
`AgentTarget` returns the first agent pane of the workspace, the bead is
**claimed**, and a work prompt tiered and caged for the routed persona's PID
is typed into whatever agent that pane holds — no gates, no cage, wrong
persona. Reproduced before the fix in all three legs (`--dry-run`, a plain
pass, `--resume`) plus the cockpit's `d`; the pre-fix run shows
`agent prompt wForeign:p1 Work beads issue a-1 …` in the herdr call log and
`--claim` in bd's.

**The fix is one line of policy: a wiped meta makes a session
un-promptable, not fleet-promptable.** `foreignHeld(names…)` is `crewHeld`'s
sibling and sits beside it at both launchers — `Run`'s per-bead loop and
`LaunchBead` — because it answers the same question (*is this name somebody
else's?*) and has to answer it before the holder join adopts the row.
`launchSession` carries the backstop for every route that reaches it.

Three things that had to be got right:

- **Refuse, do not treat as absent.** Reading "foreign" as "no session yet"
  would send `launchSession` into `CreateSession` under a label herdr
  already holds — the collision, not the fix — and on a `prompt: argv`
  runtime (ADR 0013 §2) it would put the work prompt on a fresh launch line
  beside the squatter. The check therefore sits *above* the argv branch.
- **`--resume` does not override it.** `--resume` overrides a holder's
  idleness; it has never overridden somebody else's ownership, and that is
  the route the in_progress case was reachable by.
- **The refusal is in dispatch, not in `Resolve`/`AgentTarget`.** The
  operator pointing `posse prompt` / `posse peek` / `posse kill` at a
  workspace they can see is the legitimate case those two exist for —
  and taking label resolution away would leave a squatting label with no
  way to clear it. Pinned as its own arm, so a later "just fix it in
  Resolve" reads as a regression.

The refusal names both ways out, because unlike a busy session it is
permanent until the operator acts: `posse kill <name> or rename it in herdr
to free the name`.

**Residual, and it is a doc line rather than code.** A meta backfilled BY
HAND defaults `crew: false`, so a by-hand recreate hands the conversation to
the fleet with a perfectly valid record. No guard can know that history;
`RelaunchSession` is the sanctioned path and preserves `Crew`
(`relaunch.go`). The destructive half — `posse kill` and cockpit `x` closing
another owner's live workspace — is rangerhq-selx, which this bead blocks.

Pinned: `internal/rhq/foreignlaunch_qa_test.go`. All four launch pins
verified red with the dispatch changes reverted; the fifth
(`TestResolveStillAnswersForeignRowsForTheOperator`) is green either way on
purpose — it pins the carve-out, not the fix.
