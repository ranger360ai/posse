## The ordered matrix cannot see a shadowed rule, so five of the override's stated properties were unasserted (ranger-base-b96nx)

*bead: ranger-base-b96nx (P1 bug), discovered-from ranger-base-ksfk6 · relates
ranger-base-0sa5a (the override), ranger-base-vf3pf (the harness sweep),
docs/notes.d/ranger-base-0sa5a.md · 2026-10-03, this box (darwin 25.4.0,
herdr-bob @e53bd475, bob 2.0.5, jq 1.8.1) · everything below is MEASURED by
`scripts/verify-herdr-bob-rules.sh` unless marked ASSUMED*

Six findings came out of verifying the ranger-base-0sa5a close. Three were
already fixed when this bead was picked up and three were not.

## What was already fixed

Findings 1, 2 and 3 all landed in 7147dfde (ranger-base-vf3pf), which was
already on `main` at 69ee64d5 — the EXCLUDED row for `verify-herdr-bob-rules`
in `scripts/verify-box.sh`, the sixteen forked assertion arms, and the dead
multibyte-bracket assertion, which that commit moved out of the pipe subshell
to `multi="$(jq_ask multibyte-brackets)"` decided at top level.

**main is not red.** `TestQABoxCheckCensusCoversEveryVerifyTarget` and
`TestQANoAssertionArmDecidesThroughAForkedMatcher` both pass at 69ee64d5,
verified with `-v` so that `ok` is a test that ran and not a filter that
matched nothing. Finding 3 was re-measured rather than assumed: with
upstream's three row marks put back into one bracket expression, the harness
now prints `FAIL bracket expression with a multibyte character` **and exits
1**, where before the finding it printed the FAIL and exited 0.

## What the ordered matrix cannot see

The override orders `bob_execute_command` and `bob_signin` first, so the
matrix — which asserts only *which rule id wins* — is answered by the first
rule that matches and every property of a rule behind it is stated and
unasserted. Four such properties, each one the close states deliberately:

| property | ordered | alone | derived screen |
|---|---|---|---|
| `sso_login_prompt`'s `sign[- ]?in` (the hyphen) | survives | **KILLED** | — |
| `tool_approval_prompt`'s added U+2192 | survives | **KILLED** | — |
| `bob_execute_command`'s row marker optional (`?`) | survives | survives | **KILLED** |
| `bob_execute_command`'s heading anchored | survives | survives | survives |

The bead's proposed fix shape — replay each rule in isolation — was recorded
as separating all four. MEASURED: it separates two. The other two are not
about ordering at all.

## Isolation is for ordering; a screen is for everything else

`ISOLATION` in the verify is a second matrix: one rule at a time, built with
`jq --arg id '{rules: [.rules[] | select(.id==$id)]}'` through the script's
existing jq rig, replayed through the same `replay.sh` over the same six
captures in both windows. Its negative half is the point for the two
UNFIXTURED rules — `trust_folder_prompt` and `license_prompt` must match none
of the six **alone**, which is the whole of what the override claims about
them and was previously carried by rules ordered in front of them.

The table is cross-checked against the file's own rule ids in both
directions. A rule added to the override and not to the table is replayed by
nobody; a table row naming a rule that is gone replays an empty one-rule file
and reads as six clean misses. Both are green silence, which is the failure
this arm exists to end.

**Derived screens, and why they are not checked in.** Two properties need a
screen the corpus has not got. Each is a stated transformation of one real
capture, applied at run time:

- *cursor-moved* — `blocked-execute-command.txt` with ↓ pressed once: the
  marker on `Always Allow Command for task`, `Approve Once` indented like the
  other unselected rows, the counter at `(2/3)`. Three substitutions, no
  invention; it is a screen Bob draws and nobody captured.
  `bob_execute_command` must still fire on it.
- *completed-only* — `blocked-execute-command-scrollback.txt` cut just above
  the live dialog, which is that same pane one dialog earlier: it carries
  ` Execute Command (completed)` and no dialog at all. Nothing may fire on
  it. A stopped wait on an idle pane is the expensive half of a wrong verdict.

They are derived rather than added to `etc/herdr/agent-detection/upstream/bob`
because that directory holds what a real Bob really drew, and a synthetic
screen in there is a capture the next reader will trust as one. Derived at run
time it cannot drift from its source, and the derivation is refused loudly —
never reported as a verdict — when the source stops having the lines it reads.

## The heading anchor is not pinnable, and that is the answer

Unanchoring `^[[:space:]]*Execute Command[[:space:]]*$` to a bare
`Execute Command` moves no verdict on any of the six captures, in either
window, ordered or alone, **nor on the completed-only screen**. The reason is
in the rule: `all` also requires an `Approve Once` row, and a finished call
leaves none behind, so no screen carrying only the completed spelling can
reach the heading test at all. The anchor is belt to that brace.

`docs/notes.d/ranger-base-0sa5a.md` and the rule's own `_why` both said
`blocked-execute-command-scrollback.txt` is the capture that pins the anchor.
It is not. Both are corrected rather than the anchor removed — it costs
nothing and it is right — and both now say so, so that the next reader does
not spend a session hunting the mutant that kills it. What is missing is a
capture, not a mutant.

## Vacuity, because an arm that cannot fail is worse than no arm

Each new arm was shown to bite:

- `bob_execute_command`'s `all` reduced to a bare `Execute Command` → the
  completed-only arm reports a false blocked, exit 1.
- a seventh rule appended to the override → the table cross-check names it,
  exit 1.
- and the three kills in the table above, each from an unmutated green run.

## Cost

`make verify-herdr-bob-rules` goes from ~6.4s to ~14.5s on this box: 72 more
replays (6 rules x 6 captures x 2 windows), 4 for the derived screens, and 6
more `jq` forks. The Makefile comment is updated. It is still not a
prerequisite of `make test` — it needs the plugin installed, which is an
instance fact.
