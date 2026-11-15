# A runtime that dies before it reports reads as "that seat ENDED" (ranger-base-ybrk6)

ranger-base-ybrk6, 2026-11-15. Discovered from ranger-base-1l1en: every launch
line is exec'd now, so a seat's block closes when its runtime ends, and ADR
0076 §3's launch confirmation started reading a server that answered perfectly
as a server that would not answer. ADR 0076's Amendment 10 of the same date
carries the decision; this fragment is the working record beside it — the error
surface the predicate is derived from, the mutation table, and the two things
that cost the most to get right.

Environment for every reading here: Linux 7.2.15-1-omarchy, 16 cores, go1.27.3,
rex CLI 0.0.1794069975-g6c6689, posse built from this worktree.

## Where the readings were taken, and the incident that is not this fragment's

Every reading below comes from `probe.sh`, run at 08:07 against a SCRATCH Rex
server on a private socket (`server run --listen
unix:///run/user/1001/ybrk6/rex.sock --restore=false --max-sessions 6`), with
`rex -S <that socket> --autostart=false` on every call, `whoami` reading
`session_count: 0` before anything was created, and a cleanup that killed the
two session ids the script itself had created. No fleet seat was addressed and
no model ran.

A LATER script in the same session (`live.sh`, 08:29) did none of that and
destroyed eight live fleet seats. That is ranger-base-uq81v, which owns the
incident, the evidence and the fix direction for scratch-server harnesses; it
is named here only so a reader of this fragment does not attribute the readings
above to it. The two differ in exactly the three things uq81v's fix direction
names: `live.sh` resolved its endpoint from the environment instead of passing
it, did not abort when its own sanity line printed the fleet socket, and killed
everything in `rex ls` rather than what it had made.

## rex's refusal for a gone BLOCK, per verb

The gone block is the one `rex new deadseat -- /bin/sh -c 'exit 0'` leaves
behind: the program ended, so the block closed and the session stayed as a husk
(`block ls` → `{"blocks": []}`, `window ls` → `"windows": []`, the session
still in `rex ls`). Every verb posse calls with a block id, against that block,
answered on stderr with exit 3 and nothing on stdout:

| verb | rex's refusal |
| --- | --- |
| `block call -s S -b B program_status` | `block target "B" not found` |
| `block call -s S -b B process` | `block target "B" not found` |
| `capture -s S -b B --format text` | `block target "B" not found` |
| `send -s S -b B hello` | `block target "B" not found` |
| `send-key -s S -b B enter` | `block target "B" not found` |
| `focus -s S B` | `block **or window** target "B" not found` |

**The `focus` row is why the predicate is two constants**, and it is the whole
value of taking the measurement the bead asked for rather than reading the one
call site that needed it. `rex focus` takes a block OR a window in one
positional, so its refusal names both — and a predicate matched on `block
target "` alone answers `false` for it while answering `true` for the five
verbs beside it. `confirmReports` calls `program_status`, so a predicate
written from the failure in hand would have been correct, shipped, and wrong
for the next caller.

Three more readings, all the same sentence:

- **a block that never existed** (`-b block:0000…`) → `block target "…" not
  found`, in a LIVE session.
- **a word that is no id at all** (`-b notablock`) → `block target "notablock"
  not found`.
- **the block's session destroyed under it** → the same sentence with a reason
  clause appended, which a prefix match carries unchanged: `block target "B"
  not found: session scope: session "S" not found`.

And the neighbour that is NOT this sentence: a SESSION-addressed verb names the
session. `block ls -s <destroyed>` → `session target "S" not found`, which is
what `rexSessionGone` already read. So the two predicates read the two
sentences rex actually prints, and `RexMux.SeatSaysGone` is where a caller that
wants either asks for both.

The contrast rows, for completeness: the live block answered
`{"records":[]}` for `program_status` and a full `process` payload, both exit
0.

## The mutation table

Eleven mutants, one exact-string edit each over an otherwise unmodified tree,
the tree restored from a byte copy between them, the pins run as a `-run`
filter over `./internal/posse` (this bead's four new tests plus the eight
neighbours they are the other side of). The harness is in this session's
scratchpad; it is throwaway, but one thing about it is worth keeping: **the
first harness planted none of its mutants.** Its `sed -i 's|…|…|'` patterns
carried Go's `||` operator, which sed read as the delimiter, so every mutant
silently no-op'd and every run came back green. A mutation harness that cannot
plant reads exactly like a tree with no holes. The replacement asserts the
anchor occurs exactly once before it writes.

| # | mutant | result |
| --- | --- | --- |
| — | baseline | green |
| 1 | `rexBlockGone` loses the `block or window target` spelling | KILLED by 2 |
| 2 | `rexBlockGone` loses the `block target` spelling | KILLED by 7 |
| 3 | `rexBlockGone` drops its `IsHerdrHang` guard | **SURVIVED** |
| 4 | `rexBlockGone` drops the `" not found` suffix test | **SURVIVED**, then killed |
| 5 | `RexMux.SeatSaysGone` always false | KILLED by 6 |
| 6 | `RexMux.SeatSaysGone` drops `rexSessionGone` | KILLED by 1 |
| 7 | `RexMux.CloseSaysGone` widened to the seat question | KILLED by 1 |
| 8 | `confirmReports` loses the ENDED arm (the defect restored) | KILLED by 4 |
| 9 | the ENDED arm built with the no-record sentences | KILLED by 4 |
| 10 | `launchVerdict` loses the dispatched/interactive asymmetry | KILLED by 4 |
| 11 | `launchVerdict` loses the argv arm | KILLED by 2 |

**Mutant 3 survives on purpose and the code says so.** The `IsHerdrHang` guard
is belt: a hang's text is posse's own argv and its wait, which carries the
block id but not rex's sentence, so the substring test refuses it unaided. It
is kept for the reason `rexSessionGone`'s identical guard is kept — read as
"the seat is gone", a blown deadline would refuse a launch on the one answer
that proves nothing about the seat — and the pin holds the OUTCOME rather than
the line, which is what would catch a rewrite that keyed on the exit code,
where a killed child and a refusal are the same nonzero.

**Mutant 4 survived and should not have.** Dropping the `" not found` test left
`block target "` matched alone, and no row in the table distinguished them —
every sentence rex prints that names a block happens to be a `not found`, so
the suffix test was correct, load-bearing in principle and held by nothing. It
has a row now (a refusal that names the block without losing it), written
against the shape rather than a reading, because the reading that matters is
that "rex mentioned the block" is not "rex does not have the block". Re-run
with the row in place: KILLED. That is the one defect this exercise found in
work that was otherwise already green.

## The fake had to learn all three spellings

`fakeRexScoped` printed one sentence — `block target "<id>" not found` — for
every verb, which is exactly the fake a one-message predicate passes. It now
takes the verb and spells `focus`'s refusal as rex does, and appends the
session-scope clause when the `-s` session is not held either; `block ls -s
<unheld session>` stopped answering `{"blocks": []}` and answers `session
target "<id>" not found`, exit 3, because the empty listing is also what a live
session holding no block answers — the husk — and a fake that spells a gone
session the way it spells a husk cannot tell apart the two readings that turn
on the difference.

That is why `TestQAEveryRexVerbsOwnRefusalForAGoneBlockReadsAsGone` drives all
six verbs through `RexMux` rather than asserting against lines a test wrote:
the assertion is against what the fake prints, and the fake prints what the
server printed.

## One ladder, two reasons

The reason is new; what a miss COSTS is not. ADR 0076 D2 keys the cost on
`prompt:` and on whether anybody is watching, both facts about the launch and
neither a fact about why it failed — so `HerdrBackend.launchVerdict` is that
ladder lifted out and applied to a `rexLaunchVerdict` of three sentences, and
the two reasons differ only in which three. Mutants 10 and 11 are the ladder's
two arms and each reds pins from BOTH reasons, which is the property the lift
was for: one place for D2's key, so a change to it cannot land for the absence
and miss the end.

All three sentences are built before the ladder chooses, which costs three
formatted strings on a path that has already spent a bounded wait and buys the
one property worth having here — a reason cannot pick its own verdict.

## What the sentences had to drop

Both no-record arms advise `posse peek` and keep the session deliberately,
"the only evidence of why no record came". Neither is true of a closed block:
MEASURED, `rex capture` of one is the same `not found`, so the scrollback went
with the block and the session is a husk holding no evidence. The ENDED
sentences name the session's log and, on a caged seat, `posse wall-watch`'s
filed bead instead, and they offer `posse kill` as what frees the server's
session slot before ADR 0080 D10's prune gets to it. A pin reds on `posse peek`
appearing in the refusal, because that advice is the kind that survives a
copy-paste.

## Not closed here

- ranger-base-uq81v
