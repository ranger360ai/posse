# ranger-base-wjqeu — verifying ADR 0071's check against the real bd, and the one arm it had not pinned

Verification of four closes (ranger-base-lw0s5, -f9g3c, -i00xh, -xqtn0). Two
readings are worth keeping past the bead: a live-bd measurement ADR 0071's
offline pins could not make, and the one mutant that survived them.

## The check fires over the real state, and refuses nothing healthy

ADR 0071 D6 split the pins: twelve offline arms land with the code
(`internal/posse/bdemptyunread_qa_test.go`), and the LIVE arm — the real
pinned bd over a real zero-row store — is deferred to its own bead. So at
the close of the code bead, nothing had asked the pinned bd 0.50.3 whether
the shipped check actually fires on the state it exists for. It does.

MEASURED 2026-10-09, darwin 25.4.0, bd 0.50.3 (bd25acbc), a seatbelt crew
seat, through a throwaway in-package probe calling `Bd{Bin: "bd"}` against
scratch git repos (each its own git root, because bd resolves `.beads` from
the git root), census records copied from the store of record:

| rig | `.beads` state | ListAll | Ready / InProgress / OpenLabeledAny |
|---|---|---|---|
| fresh clone, `no-db` unset | 3-record census, no `beads.db` | `BdStoreUnreadError` | all refused, same error |
| the same rig, second call | the empty `beads.db` the first list materialised | `BdStoreUnreadError` | refused |
| healthy, `no-db: true` | 3-record census, JSONL is the store | 3 rows | `nil, nil` — the empty reads are honest |
| no census at all | `.beads` holding only `config.yaml` | `nil, nil` | — |

Two things the offline arms cannot say, and this does:

- the FIRST list over a JSONL-only repo with `no-db` unset builds the empty
  `beads.db` itself and then answers `[]` over it, so the defect state is one
  read away from any fresh clone — and `errors.As` reaches
  `*BdStoreUnreadError` through it, with the store, the census and the
  operator's repair named.
- a healthy `no-db: true` store reports its count — `issue_count: 3`, read
  from the JSONL, no database in the directory at all — so a legitimately
  empty FILTERED read there is served and not refused. That was the false
  positive worth pricing: every store `bd init` writes today is `no-db:
  true` (internal/posse/beads.go, `Ready`'s block), and had `info` answered
  0 over such a store, every quiet pass over one would have died "the queue
  is unknown, not empty".

`bd info --json` against the store of record: `issue_count` present, 0.24s /
0.25s / 0.39s over three runs — the same order as the 0.49-0.63s the code
bead measured under load.

## The mutant the twelve arms did not kill

ADR 0071 D2a is two claims: a census that is **absent**, or one with **no
line beginning `{`**, is no census to contradict bd with. The arms covered
the first and covered a file of blank lines; nothing planted a line that is
non-blank and not a record. So `censusHasRecord`'s `t[0] == '{'` was
unpinned — MEASURED: replacing the predicate with a bare `len(t) > 0` left
all twelve arms GREEN.

Under that mutant a census holding a line that is not a record is weighed
against bd's count and a HEALTHY empty read is refused — the false positive
the paragraph above exists to rule out. The two rows added to
`TestQAAnEmptyCensusFileIsNotACensusWithRows` are the ways that file comes
about here: bd's own error text landing in it, and an empty `--json` answer
(`[` `]`) redirected into it, which is the very shape the record is about.
Re-measured after: the mutant reds on exactly those two rows and the
blank-line row stays green, so neither new row is re-pinning what was
already pinned.

The general shape, for the next reader: an "absence" fixture that is EMPTY
tests the emptiness, not the predicate. A predicate over line CONTENT needs
a fixture with content.
