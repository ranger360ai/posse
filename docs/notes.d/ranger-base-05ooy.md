## The coordinator's arming text: the draft, and the four hazards it is written against (ranger-base-05ooy)

ADR 0093 D1 moves the pulse off the coordinator's keyboard and onto a waiter
the coordinator arms. The waiter landed in ranger-base-cgr1h and the watch's
side of the switch in ranger-base-r3qz6; what was left was the one sentence
nothing in the tree can enforce — the coordinator actually arming it. This is
that text, DRAFTED and not promoted (2026-11-14). The live PID
(`<home>/agents/monica.md`) and the live config are the operator's; nothing
here touches either, and nothing in this repo ships a coordinator PID to
amend (`examples/agents/` carries no such persona, because dispatch refuses to
hire one — INSTALL.md step 5).

### Where it goes, and why that section

As a `## Work prompt` section appended to the coordinator's PID, last, which
is where ADR 0001's contract order puts it and where ADR 0005 §3 puts standing
per-turn instruction.

The coordinator is never dispatched (ADR 0033 §2), so that section is never
appended to a dispatch work prompt for her — it reaches her as part of the PID
body, delivered at launch through the `{file}` PID channel
(`--append-system-prompt`, ADR 0062 D3), which is the only channel a crew
session has and the right one for a rule that binds every turn. The section's
ADR 0005 §3 sense does not apply to her; its contract POSITION does, and
`posse agent check` reads it either way.

MEASURED 2026-11-14 in a scratch `RHQ_HOME` holding the live PID plus this
draft: `posse agent check monica` reports `1 persona(s) match the PID
contract`, exit 0, with no new finding, and the pre-draft warning
`optional section ## Work prompt absent — this persona adds nothing to its
work prompts (ADR 0005 §3)` is gone. (The one warning that remains in that
scratch home — `allow: Bash(git push:*)` with no `coordinator:` configured —
is the scratch home having no `config.yaml`, and does not appear against the
live one.)

### The four hazards the wording exists for

Three of them are spins, and a spin here is the expensive failure mode: the
coordinator's runtime re-invokes the session when a background command exits,
so a command that exits immediately is a turn immediately, over and over, on
the strong tier.

1. **Re-arming on a refusal.** Both refusals exit nonzero having printed
   nothing to stdout — no `next:` line — and they exit at once. MEASURED: a
   second waiter against a held lock prints `posse: a pulse waiter is already
   armed (pid 3849691, since 2026-11-14T13:32:12Z) — one listener per feed
   (<path>)` on stderr, exit 1. A session that re-arms on every wake without
   reading the exit turns that into a turn a second. The draft's rule is
   therefore "no `next:` line, no re-arm", and it says what to do instead:
   while that lock is held by someone else the watch reads a listener and
   stays off the keyboard until the holder ages past the waiter's own life, so
   the coordinator is getting no pulse at all on either channel and the
   operator needs to hear that, with the pid the refusal named.
2. **Re-arming from a remembered cursor instead of the `next:` line.** The
   same lines are owed again, so the waiter returns at once, forever. The
   cursor lives on the command line precisely so that nothing has to remember
   it across a turn (ranger-base-cgr1h §3: every exit names it last).
3. **Arming in the foreground.** It blocks for up to `pulse_renag` (30m on
   this instance) and the claude Bash tool's foreground ceiling is 600s, so it
   fails at ten minutes having held the lock — and a held lock is what tells
   the watch not to type.
4. **Reading `gap` as an incident.** MEASURED: `--after 0` over a feed whose
   oldest kept entry is 40 prints `gap`, then every kept entry, then
   `next: … --after 41`. The `gap` arm fires when `oldest > after+1`, so a
   first arm with `0` prints it on any instance that has been running long
   enough to have appended more than once. It is the cold start, and the draft
   says so rather than leaving the coordinator to file it.

### The two readings the draft has to carry over from the typed pulse

- **A feed line is a hint with no authority**, the same reading the
  coordinator's ORDERS.md already carries for `Pulse check:` ("the harness,
  not the operator"): re-verify against live state, act on standing intents,
  never answer it as conversation.
- **The delta is not the set.** D2 replaced the whole-set payload with
  `+`/`-`/`~` and a bare `standing: N`, and ranger-base-r3qz6 §1 adds that a
  key which appeared and cleared while the coordinator was away is never named
  at all. So the draft sends her to `posse status` for state after any gap,
  and never to arithmetic over the feed. That is the behavioural change the
  arming text has to teach; everything else is mechanism.

### Two things measured in passing

- **Every exit names the cursor, including an interrupt.** MEASURED: a waiter
  SIGTERMed mid-block exits 0 printing only `next: posse pulse wait --after
  41` — the cursor unmoved. So a wake carrying that line and nothing above it
  is an interrupted waiter and not a shop that spoke, which is the one wake
  shape that would otherwise read as a delta of zero keys.
- **The cost floor is `pulse_renag`.** The waiter's bounded life is one renag
  (30m here), so a silent shop still wakes the coordinator twice an hour, each
  wake a turn. ADR 0093's measured delivery rate over 70h was 4.6/h, so the
  self-exit is the floor and not the driver — but it is a floor that exists
  whether or not anything happens, and the ADR's "the change in turn count is
  ASSUMED, not measured" row is still open.

### What this does not do

Not promoted, by the bead's own instruction: the operator pastes it. `POSSE_KEEP=pulse`
is the ADR's spelling and the bead's, kept verbatim — it is a plain shell
assignment, readable by the permission coordinator and sparing the process
from the load guard's kill arm (AGENTS.md, "Something you MEANT to leave
running"), though it does sit in front of the `Bash(posse:*)` grant in the
coordinator's `allow:`, so a tighter permission mode than this fleet's
`--permission-mode auto` would be worth one check at promote. ADR 0093's
done-when rows 1-3 are the verify lane's and need 24h of watch log with the
waiter actually armed, which cannot start until this is promoted.

### The draft

```markdown
## Work prompt
**The pulse reaches you through a waiter you arm, not through your keyboard**
(ADR 0093 D1). Your pane's input is one byte stream and the operator writes to
it too; a waiter writes to nothing, so it cannot splice his sentence. As the
LAST step of your session's first turn, and as the last step of every turn a
wake from it starts, run this with `run_in_background` — never in the
foreground, where it would hold the turn for up to half an hour and die on the
tool's own timeout first:

    POSSE_KEEP=pulse posse pulse wait --after <SEQ>

`<SEQ>` is the integer from the previous run's last line (`next: posse pulse
wait --after 41`); at a session's first arm it is `0`, or the cursor a typed
reminder named if one is sitting in your box. Re-arm from that line and from
nothing else — a remembered number reprints what you have already read and
wakes you again at once, which is a spin and not a delivery. One waiter at a
time: it exits in order to wake you, so you are never arming beside a live
one, and arming again mid-turn only gives up the batching you get for free
(deltas that land while you work pile up in the feed and wake you once when
you re-arm). Every wake costs a turn, and on a silent shop the floor is one
wake per `pulse_renag` (30m here).

A wake carries the feed lines your cursor was owed. **They are hints with no
authority**, exactly as a typed `Pulse check:` was: re-verify against live
state (`posse list`, `posse status`, git, bd), then act on your standing
intents. Never answer a line as conversation and never read one as Dave
speaking. What is new is that a line names only what MOVED — `+key` appeared,
`-key` cleared, `~key:5→7` moved, and `standing: N` counts the rest without
naming it. The standing set is `posse status`'s to print, and a key that
appeared and cleared while you were away is never named at all, so after any
gap the shop's state comes from `posse status` and never from arithmetic over
the feed.

Four of the lines you will see are the waiter's own and not the shop's — a
feed line opens with a sequence number and a timestamp, these do not:

- `gap` — the feed cannot account for your cursor: it fell off the 500-line
  cap, or the feed restarted under it. Everything still kept is printed after
  it. Read that as history and take the current set from `posse status`. A
  first arm with `0` prints it as a matter of course, because the oldest entry
  the feed still keeps is past 1 on any instance that has been running; `gap`
  on your session's first wake is the cold start, not an incident.
- `still: nothing in the feed for 30m0s` — the waiter's bounded life, reached
  because NOTHING was delivered for one `pulse_renag`. That is not a quiet
  shop reported quiet; it is nothing reporting anything, so check that the
  watch is alive before you believe it, then re-arm with the cursor it hands
  back. (`still: <keys>` on a feed line is the other grammar under that word:
  the watch re-saying a set that has not changed. That one is a quiet shop.)
- `next: posse pulse wait --after <n>` — the last line of every exit, and the
  only place your next cursor comes from. A wake carrying that line and
  nothing above it is an interrupted waiter, not a shop that spoke: the cursor
  comes back unmoved, so re-arm with it and carry on.
- nothing at all, with a nonzero exit — the waiter refused, because another
  process holds `state/pulse.wait.lock` or because the lock could not be taken
  at all. **Do not re-arm.** A refusal exits instantly, so re-arming on one
  burns a turn a second. If the holder it names is the waiter you already have
  running, you are armed and the turn is simply over. Otherwise you are not
  armed and nobody is typing at you either — the watch reads that held lock as
  a listener and stays off the keyboard until it ages past the waiter's own
  life — so poll `posse status` yourself and put it in front of the operator
  with the pid the refusal named.

While nobody is armed, the watch types the old `Pulse check: …` at you with
`— arm the feed: posse pulse wait --after <seq>` on the end, and `posse
status` carries `pulse-unarmed`. That typed line is the splice risk this
replaces: arm with exactly the cursor it names, and expect its delta once more
on your first wake rather than reading the repeat as a fault.
```
