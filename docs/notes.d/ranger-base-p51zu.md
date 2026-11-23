# ranger-base-p51zu — a seat with no work prompt is a fact the records already held

A dispatched seat sat at its CLI's welcome banner with an empty composer, a
worktree, a branch and a bead pointer, and nobody told it what to do. The
operator's delegate found it by looking at the pane. This is what the records
said while that was happening, and the one reading nothing was taking.

## What happened, MEASURED

2026-11-23, the originating box, rex occupant, claude/standard, posse at
`7dc3313e`. From `state/dispatch-watch.log`, one pass, four consecutive lines
(persona and repo elided — ADR 0012 App. A):

```
· ranger-base-72z36 creating session <persona>-<repo>-ranger-base-72z36 (… claude/standard)
· ranger-base-72z36 own tree …/<persona>-<repo>-ranger-base-72z36 on posse/… (merges to main at close)
✗ ranger-base-72z36 herdr: no idle|done|blocked state in block:01a205f7-… within 16.446s (last reading: working) (timeout)
– ranger-base-72z36 <persona>-<repo>-ranger-base-72z36 did not take the launch — <persona> keeps its slot; the next bead gets a fresh session
```

The bead asked which of two things happened: the prompt raced the TUI, or it
was never sent. **It was never sent.** `awaitAgent`'s error is raised above
the claim and above the keystroke (`launchSession`, dispatch.go), so the pass
created the seat and left before typing: the bead stayed unclaimed, the
worktree stayed, and a live CLI stayed sitting on its first screen. That is
the settle gate working — a pane reading `working` is exactly what it refuses
to type at (`awaitSettled`'s own header) — and the cost was not the refusal.

The two run records, both read at 01:21Z (`state/herdr/*.yaml`), are the whole
of the evidence. The abandoned seat:

```
bead: ranger-base-72z36
launched: 2026-11-23T01:13:51.934876903Z
```

and the seat launched 54 seconds later, which took its prompt:

```
bead: ranger-base-p51zu
launched: 2026-11-23T01:14:45.45136428Z
prompted: 2026-11-23T01:15:32.668652986Z
```

`bead:` set and `prompted:` absent. The fleet had the fact; nothing read it.

## Why nothing was ever going to say so

Four surfaces could have and none can:

- **the reaper** wants a bead the store of record calls CLOSED
  (`autoreap.go`); this bead was open, so no arm of it — narrow, crew-marked
  or stampless — can reach the seat.
- **the lane lines** read occupancy through `personaActive`, whose
  idle-seat-still-holding test requires the bead to be `in_progress` AND
  assigned to the persona. An unclaimed bead fails both, so the seat is not
  even reported busy; it is reported as nothing at all.
- **`posse ls`, G1 and G15** read a pane that is idle and (bar the host's
  ratified cage map) undegraded. That is what a seat which FINISHED looks
  like.
- **the launch's own `✗`** is one line in a log the fleet writes thousands of
  lines a day to, and it names a rex block id rather than the session.

So the seat was invisible in exactly the way ADR 0029 exists to end: every
process could compute the condition and none was asked to.

## G19

`unprompted.go`. ADR 0029's bar for a row outside the table is a documented
predicate, owner, scope and class:

| | |
|---|---|
| predicate | a live, non-foreign, non-crew session whose meta carries a `bead:` pointer, carries NO `prompted:` stamp, and whose `launched:` stamp is older than `unpromptedGrace` |
| owner | the session's own run record (ADR 0011 §3) — the launcher writes both stamps and nothing else does |
| scope | one seat; the row names the session, the persona and the bead |
| class | LANE — one bead is stopped and the shop flows |

It heals three ordinary ways and nothing has to be closed: the next pass
prompts the seat it already resolves by name (`launchSession`'s
existing-session path), the operator prompts it by hand, or the seat is
killed. That is a condition rather than a bead, which is the split ADR 0029
is about.

The walk sits beside G15's in `ShopCheck` and opens a record only for a
session the listing already shows as a dispatched per-bead seat — eight of
those on a full box, at pass cadence.

### The grace is a bound, not a preference

`unpromptedGrace` is 15 minutes and there is deliberately no config key for
it, because the number is not an opinion about how long an operator should
wait. It is the bound on how long a launch can still be mid-flight, and it
clears two floors:

- **the launch's own worst case.** A typed launch spends the runtime's
  patience up to three times before a key is pressed — `awaitTarget`, then
  `awaitSettled`, then `AwaitPersonaMode` — at `DefaultStartupWait` 45s each.
  135s, plus the work prompt's own bd read. 15m is more than six times it.
- **the pass that heals it.** A failed launch is retried by the next pass,
  which resolves the same per-bead session by name and prompts the live seat,
  so the grace has to outlast more than one pass or the row beats the fleet's
  own repair to the operator. The cadence is the instance's own
  (`autostart_interval:` — there is no shipped default, the key IS the arm
  switch), so the reading is on the bead and not here (ADR 0024 D3). 15m is
  several passes at the cadences this shop is run at; a seat that reds has had
  its chances and taken none.

MEASURED against the incident: the launch line was typed at 01:13:51.9Z and
the record still carried no stamp at 01:21Z — 7m later, with the next tick
still ahead of it. A 15m horizon is a row a tick or two after the failure,
not a race with the first one. Verified by execution the other way too: `posse status`
run from this build against the live fleet at 01:27:51Z — 14m00s after that
launch — raised nothing for the seat, and the same command at 01:30:0xZ
raised it, LANE, naming the session, the persona, the bead, `16m ago` and the
two remedies. One row, on the seat the operator had already found by eye.

No suspend discount, and that is a reading of ADR 0064 D1 rather than an
omission of it: the grace exists to exclude a launch still IN PROGRESS, and a
launch cannot be in progress across a suspend — the process that was going to
type is gone with it. The awake/wall distinction cannot move this answer in
the direction that matters.

## The stamp a hand prompt never wrote

`posse prompt` is the measured recovery for this row, and it did not clear it,
because that path never wrote `prompted:` at all. It wrote the bead pointer
(`NoteBeadFromPrompt`, ranger-base-v674) and the crew mark and stopped — so
the operator's own prompt was invisible to the one record introduced to make a
prompt legible across processes.

It writes the stamp now (`cmd/posse/main.go`, `case "prompt"`), after the
prompt took, like the crew mark beside it. That is load-bearing twice:

- G19's remedy clears G19.
- `promptedRecently` — the guard that stops a running pass typing a second
  instruction into a seat mid-turn — could not see a hand prompt either. It is
  the persisted half of PromptGrace for exactly that reason
  (`rangerhq-tzdf`'s remaining half), and the one prompt it was blind to was
  the operator's.

It is the same read-modify-write on the same file `NoteBeadFromPrompt` does
one line above, without the launcher lock dispatch holds — the risk class this
call site already carries, not a new one.

## Not closed here

- `ranger-base-hcig1` — the settle gate returns the Wait verb's `timeout`
  error raw, so the window closing is spelled two ways: one of them takes
  `awaitSettled`'s deadline ladder (a logged `Reading`, the last screen named,
  `check the session (posse peek …)`) and one bypasses it entirely. The three
  abandonments of this shape on the box's current logs all took the second
  spelling. Which arm a timeout should take is a decision about whether to
  type at a pane whose last reading was `working`, which is ADR 0061/0066
  territory and not this bead's.
