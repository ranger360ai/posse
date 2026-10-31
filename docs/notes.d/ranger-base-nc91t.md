# Relaunch's two readings are the slot's listing, and the second one is why holdsRecorded alone was not the fix (ranger-base-nc91t)

2026-10-31. `posse relaunch` is the one command that was still herdr-only
after ADR 0076 §5 landed: two of its readings asked herdr's raw `workspace
list` through a §4 door, so on a non-herdr instance neither answered and the
whole command refused. Nothing was damaged and nothing was refreshed — the
one place a Rex instance behaved worse than herdr for a reason nobody chose
(ranger-base-q5gc2's notes §5 named it and left it open).

Three things are worth a reader's time: the bead named one reader and there
were two, the refusal that actually fired was the other one, and the fence
the conversion had to keep.

## 1. Two readers, not one — and the one the bead named fires SECOND

`ranger-base-q5gc2` §5 named `holdsRecorded`: relaunch's aiming question,
"which seat is this session's", asked one line before `closeRecorded` closes
it. That is the reader whose conversion the bead was filed for, and on its
own it would have changed nothing an operator could see.

`RelaunchSession`'s order is the preflight first, and the preflight asks the
OTHER reader:

```
readMeta → CallerRunsInside → planLaunch → provenNameTakeable → landThePlane → ReapRefusal → replace
                                              └── nameWornElsewhere → b.workspaces()   ← refused HERE
                                                                      replace → closeRecorded → holdsRecorded → b.workspaces()
```

`nameWornElsewhere` is the name question — is some other seat already wearing
the label the recreate is about to ask for — and `provenNameTakeable` turns a
listing it could not read into `Die("… was NOT closed: this herdr did not
list its workspaces")`. So on a Rex instance that was the sentence an
operator met, before the landing turn and before anything was spent;
`holdsRecorded` was unreachable. Converting only the reader the bead named
would have moved the identical refusal four call frames down the same
command.

Both convert to the same two lines, because both read the two fields
`SeatInfo` and `HerdrWorkspace` both carry — the session id and the label:

```go
seats, err := b.Mux().List()      // was b.workspaces()
…
if s.Session == m.Workspace {      // was ws.WorkspaceID
if s.Label == want && …            // was ws.Label
```

That is `listSessions`' and `sessionGone`'s reading verbatim, which is what
makes it free rather than a judgement: on herdr the slot's `List()` IS
`workspace list` joined to `agent list`, so the same rows come back. What the
join adds on herdr is `agent list`'s failure as a second way to get no
listing at all — and no listing REFUSES on both of these paths, which is the
direction they already go for the other half of the same listing.

With both converted, `b.workspaces()` had no readers and is gone, and so is
`HerdrMux.Workspaces()`, its only caller. `HerdrMux.List` still calls the
herdr client's own `Workspaces()`; nothing reaches it through the slot.
**ADR 0076 §4's list did not move**: `workspace list` was never on it. The
one §4 door of this shape left is `workspaceGet`, herdr's per-id query, which
is the prune's identity evidence and which `idEvidence` already branches away
from for a non-herdr record (`seatEvidence`).

## 2. The fence stays, and stays herdr's

`notOurWorkspace` is the rangerhq-yt1p fence: herdr's allocator is
`max(live id)+1` recomputed at every server process start, so the ids stale
metas hold are exactly the ones it hands out again. The conversion kept the
call — a relaunch that aimed at a stranger wearing the recorded id would
close somebody else's workspace with no landing turn, which is
rangerhq-9jk1's damage — and the predicate answers `""` for a non-herdr
record, because Rex's ids are UUIDs and are never reissued: a present id IS
the seat the record meant, and the label half alone reads a rename as a
recycle (ADR 0076 §5, q5gc2 §3).

One thing did change in construction and not in behaviour. `holdsRecorded`
read the generation from `ServerGen()`, the package function that resolves a
**herdr** socket, so on a Rex instance it stamped a herdr path's
device/inode/bind-time onto a question about a Rex seat — q5gc2 §1's bug one
reader over. It is `b.Mux().Server().Gen` now. It cannot change an answer
today, because the fence returns `""` for a non-herdr record before it looks
at either anchor; it is there so it stays true when an occupant that DOES
recycle ids arrives and has to write its fence in its own terms.

`provenNameTakeable`'s two refusals name the occupant now (`this rex did not
list its seats`) rather than saying "herdr" to an operator who has none.

## 3. What this does not reach

- **A relaunch end to end on a Rex instance is still unverified against the
  real rex.** Everything here is pinned against a stub occupant and against
  herdr's fake; the live verify is ranger-base-phq39's, where the
  multiplexer key gets its first real instance. What these pins hold is the
  pair that no green relaunch can show: a listing that WAS taken aims the
  kill at the seat the record names, and a listing that could NOT be taken
  refuses at both call sites with nothing closed and the record standing.
- **`HerdrMeta.Workspace`/`.Pane` still carry herdr's names**, Go-side and on
  disk. Unchanged, and still the mechanical rename ADR 0076 §1 defers with
  `HerdrBackend` → `Backend`.
- **`hermeticGen` moved out of `instancelabel_qa_test.go` into
  `relaunch_helpers_test.go`.** It was arm 2's and the fence has an arm 1
  reader now; a file with a build tag is absent from the arms it does not
  name (ranger-base-qp1hm). No test's behaviour changed.

## Pins

`internal/posse/relaunchaim_qa_test.go`, arm 1. Six tests, MUTATION-CHECKED
2026-10-31 — each mutation reds exactly the tests it is about and nothing
else:

| mutation | reds |
|---|---|
| `holdsRecorded` reads herdr's raw listing again | aims-at-the-recorded-seat, absent-seat-not-closed-by-id |
| `holdsRecorded` loses the `notOurWorkspace` call | does-not-aim-at-a-stranger |
| `holdsRecorded` reads an unreadable listing as "not ours" | refuses-when-the-listing-cannot-be-taken |
| `nameWornElsewhere` reads herdr's raw listing again | name-worn-elsewhere-reads-the-occupant |
| `notOurWorkspace` fences a non-herdr seat | no-identity-fence, aims-at-the-recorded-seat |

The stub occupant embeds `Mux` as a nil value, so a verb these paths have no
business calling panics by method name instead of answering a zero value —
muxmeta_qa_test.go's rule, for its reason.
