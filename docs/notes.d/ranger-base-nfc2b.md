# A rex block has no global name, so `posse peek` could read one seat on the box (ranger-base-nfc2b)

2026-11-10. Observed by the coordinator during ADR 0083's cutover step 7:
`posse peek gilfoyle-posse-ranger-base-uwxs4 25`, while `posse list` showed
that seat working, answered

```
posse: rex capture -s  -b block:01a1c7e1-e892-7ea4-8c5d-c5cd150db4b9 --format text --trim: Error: block target "block:01a1c7e1-..." not found
```

Note the empty `-s`. The bead read the message as a stale block id, which is
what it says; the block was live and listed. The empty `-s` is the whole of
it, and the reason it looked like a stale id is that rex phrases a
wrongly-scoped lookup as a missing target.

## 1. What rex actually does with `-b`

MEASURED 2026-11-10 on the always-on host (linux, rex CLI
0.0.1794069975-g6c6689, local unix socket at
`unix:///run/user/1001/rex/server.sock`), against the live server holding
three sessions — one operator session and two posse seats. Every call passed
`--autostart=false`. All of them are reads.

| call | answer |
|---|---|
| `capture -s <monica's session> -b <monica's block>` | the screen, exit 0 |
| `capture -s "" -b <that same block>` | `Error: block target "…" not found`, exit 3 |
| `capture -b <that same block>` | the same refusal |
| `block call -s "" -b <it> program_status` | the same refusal |
| `capture -b <MY OWN seat's block>`, unscoped | the screen, exit 0 |
| `window ls` with no `-s` | `"session_id": <my own seat's session>` |

The last two rows are the one that explains the bug report. `capture --help`
documents `-s` as "scope to session SESSION", which reads as an optional
narrowing of a global lookup. It is not a narrowing — it is the lookup. With
no scope the CLI resolves the block inside ONE DEFAULT session, and `window
ls` names which one: the most recently created session on the server, which
on that box was whichever seat had launched last. So an unscoped block verb
answers for exactly one seat and refuses every other, and the seat it answers
for changes every time dispatch launches anything.

The API has no global block lookup to reach for either: `api ls` puts every
block method under `session.*` (`session.list_blocks`,
`session.describe_block`, `session.focus_block`, `session.move_block`), and
`block ls` without `-s` refuses outright — "requires one session" — which
`Rex.Blocks` already measured and works around.

## 2. Where the half-a-Seat came from

ADR 0076 §2's table spells Rex's typing and reading verbs as `rex send -b`,
`rex capture -b` and `program_status` — the block alone. `rex.go` had already
half-corrected that: every verb there passes `-s <session>`. What was left
herdr-shaped is the thing on the other side of it, `Seat`, whose doc said in
prose that a caller may build one carrying only the half it holds, "and the
verbs it then calls are the ones that need it". On herdr that is true, because
a pane id addresses a pane anywhere on the server. On Rex every block verb
needs both halves, and the pane-only `Seat` is all over the tree:
`paneSeat(target)` at some twenty sites, plus `Seat{Pane: s.PaneID}` at four
more.

## 3. The blast radius was not peek

MEASURED on the host, same session, old binary then new:

| command | before | after |
|---|---|---|
| `posse peek monica 5` | `Error: block target "…" not found` | monica's screen |
| `posse wait monica --until done --timeout 3000` | the same refusal, from `block call … program_status` | `program_status state=done app=claude-code` |
| `posse list`, monica's row | `mode:?` | `mode:auto` |

`posse wait` is the same class one verb over: it addresses the pane
`AgentTarget` resolved, and `AgentTarget` reads the session out of the meta
and then throws it away. The `mode:?` is ADR 0035 §3's permission-mode
column, which reads the pane — so on that host the column was unreadable for
every seat but the default one, and it reports an unreadable pane as a blank
rather than an error, which is why nobody had noticed.

`posse list` itself was never affected, which is what made the report
confusing: `List()` builds both halves out of the server's own listing, so
the row said `working` over a block the next verb could not address.

## 4. The fix, and the price

`RexMux.hold` fills in a `Seat` whose `Session` is empty: the session that
holds that block, out of `Sessions()` plus `Blocks()` per session. Applied at
the head of every verb that addresses a block — `Start`, `Focus`, `SendText`,
`SendKeys`, `Read`, `reading` (so `State`), `Wait`, `Prompt`,
`AwaitFirstReport` — and idempotent, so a caller holding both halves pays
nothing. `Wait`, `Prompt` and `AwaitFirstReport` resolve before their loops.

MEASURED on the same host and run: `ls` 14, 14 and 15 ms over three runs,
`block ls` 16 ms for each of the three sessions. One resolution is therefore
14 + 16N ms: 62 ms over that box's three sessions, ~340 ms over twenty.
(muxrex.go's poll-rate note quotes 50-70 ms per rex call; that was darwin
over a different transport, 2026-10-31.) No memo of block → session: it would
be sound, since Rex's UUIDs are never reused, and it would buy a tenth of a
second on verbs nothing calls in a tight loop.

Three call sites were holding the session and dropping it, and now pass
`s.Seat()`: `posse peek` (cmd/posse/main.go), the cockpit's `v` peek
(cmd/posse/cockpit.go) and the listing's mode read
(internal/posse/herdrback.go). Those spend no rex calls on resolution at all.

## 5. Why the suite was green, and what reds now

The fake rex resolved `-b` out of a flat `map[blockID]…` and ignored `-s`
entirely, for `capture`, `program_status` and `process` — and `send`,
`send-key` and `focus` returned 0 without looking at either. So a pane-only
`Seat` read exactly like a full one, and every pin over those verbs passed
over an argv the real server would have thrown away.

`fakeRexScoped` now refuses a block target that the `-s` session does not
hold, in rex's own words and exit code, and refuses an unscoped `-b`
outright. That last part is the one place the fake is deliberately STRICTER
than rex: the real CLI answers for the default session, and posse must not
depend on which seat that is, so there is nothing in the fake for it to
depend on. Four existing pins were addressing a block they had never seeded
and now seed it (`rexSeedTestSeat`).

The invariant is pinned three ways, all in `internal/posse/muxrex_qa_test.go`:
`TestQARexPaneOnlySeatIsResolvedToItsSession` (the second session's block, so
a resolution cannot be confused with a lucky default),
`TestQARexAPaneNoSessionHoldsIsRefusedAndNotAReading`, and
`TestQARexWaitResolvesThePaneOnceAndNotPerPoll` (one `ls` over three
samples).

## 6. What is left

Threading a `Seat` through dispatch's `target string` plumbing —
`awaitTarget`, `awaitDelivered`, `awaitSettled`, `AwaitPromptable`,
`PaneHolding`, `ConfirmSubmitted`, `AwaitPersonaMode` and the two `target`
struct fields, some thirty signatures across twelve files — is
ranger-base-k0hd6 and is not needed for correctness now. The one place
the resolution is paid in a loop is `AwaitPersonaMode`, which reads a pane
every 250 ms during a launch: on that host it adds ~62 ms per sample, so the
loop polls slightly slower and nothing else changes.
