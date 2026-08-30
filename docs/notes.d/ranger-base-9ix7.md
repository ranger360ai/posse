## The fence has three carriers and they do not refresh alike (ranger-base-9ix7)

ranger-base-d866 established where a rule must be written so it travels with
the session. This is the next question, and nothing was asking it: once it is
written there, **when does an edit to it actually reach a session that is
already running?**

| carrier | scope | refreshed by |
|---|---|---|
| L1 `PATH` shim, `state/gates/<persona>/bin/*` | per **persona** | `RenderGates` removes the bin dir and writes it fresh at **every dispatch** — a session already running picks the change up at its next `exec`, because it looks the shim up on `PATH` each time |
| L0 `--disallowedTools` argv | per **session** | **nothing.** Rendered at launch and frozen for the life of the process; no one can rewrite a running process's argv |
| `<repo>/.claude/settings.json` | per **repo**, crew-wide | **no renderer at all.** It is a tracked file; `git worktree add` checks it out and the operator's hand is what changes it |

So an edited PID reaches a live session by one carrier and not the other, and
the two disagree for as long as that session lives. Both directions are real:

- **Tighten** a PID and every already-open session keeps running on the looser
  argv it launched with. Its shim tightens; its L0 does not.
- **Loosen** one — which is what a *narrowing* is — and every already-open
  session keeps refusing what the constitution now permits. That is the
  ranger-base-c7ek shape one layer up: c7ek was the broad `Bash(bd hook:*)` row
  refusing the verb beads' own git hooks `exec`, fixed in the PIDs and picked
  up by the shims within the hour. The sessions that were already running kept
  the broad row in argv regardless.

**Measured 2026-08-29**, reading `ps` for every live persona session and
diffing its argv deny region against the persona's PID as it stood at that
moment: six of ten sessions were running on a fence their PID no longer
spelled, four of them still carrying the two rows ranger-base-y5g7 had
superseded hours earlier.

**The third carrier had drifted too, and in the direction nobody looks.** This
repo's own `.claude/settings.json` still carried both superseded rows. It is
harmless in the way that hides it — claude matches only the top-level command,
so a verb reached through a git hook is untouched and commits kept landing —
and it is invisible to the d866 audit, which reads PIDs and this file is not
one.

### What was built

`scripts/verify-pid-deny-set.sh` gains two readers beside the PID one, sharing
its single copy of the rule list:

- `--live [--live-from <ps capture>]` — every live session's argv fence against
  its PID, **both directions**, naming which sessions are LOOSER and which are
  STRICTER than the constitution. The fix it prescribes is `posse relaunch`,
  which lands the work first.
- `--settings <repo>` — that repo's committed settings file against the rules
  the ruling superseded. Those are a finding; rules merely *absent* from it are
  a note, because ADR 0015 §3 makes the PID the carrier and a shorter copy is
  not a hole. Asserting otherwise would invent a requirement no ADR wrote.

**Why a detective control and not a re-render.** There is nothing to
re-render: argv is not writable, and `.claude/settings.json` is constitution
class (ADR 0015 §3, fourth spelling) — a persona session is refused at the
commit if it edits one. And a session mid-bead is not something to restart
behind the operator's back.

**The comparison is not a string match**, and that is the part worth keeping.
`L0Spellings` decorates each rule on its way to claude: it adds an
option-blind twin, it renders a wordless rule twice (bare and `:*`), and it
rewrites a negative rule entirely — claude's dialect has no negation, so
`Bash(git commit unless --)` reaches argv as the bare `Bash(git commit)` plus
its twin. A literal comparison therefore called that rule missing from *every*
session, all of which were current in it. `norm_rule` erases the three
decorations and compares what is left; it never synthesizes a spelling, so it
cannot drift into disagreeing with the renderer about what a rule expands to.
One self-test arm per decoration, and all twelve mutants of the new code die
to them.
