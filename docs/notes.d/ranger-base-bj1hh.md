# ranger-base-bj1hh — the fixture path that ate the account

`ranger-base-uu303`'s reading, which is `ranger-base-mklu0`'s finding 2
repeated: `scripts/posse-wall-check`'s `evidence()` carries a failed member's
first `file.go:NN:` line truncated to 300 characters, the launcher puts that
one line on a P1 bead (ADR 0097 D3/D5), and in
`internal/posse/bwrapseatstorelive_test.go` the account argument's own `%s`
sat past character 300 in FIVE messages, with all but 13 characters of a
sixth gone too — so `bgSeatAccount`'s value, which is the seat's exit status
and its words and exists nowhere else, reached the bead as NOTHING AT ALL.
Not truncated: absent.

What blew the 300 was never the prose. It was an **interpolated fixture
path** — 49 characters at its narrowest here (`<common>`) and 123 at its
widest (`side`), and 50 more again in the venue — paid once per path:

- every path in this file descends from a `t.TempDir()` whose width is a
  function of the calling test's NAME — 69 and 71 characters under the two
  tests that carried these rows;
- ADR 0097 D1's venue (`docs/adr/0097-wall-venue.probe.sh`) sets `TMPDIR` to
  `$run/tmp` under `$XDG_CACHE_HOME/posse-0097-probe`, MEASURED at 54
  characters against `/tmp`'s 4, so **every one of those paths is +50 there**
  — and that venue is the one that files the bead.

A row interpolating two such paths pays both adjustments twice over, which is why
`:751` was the widest row in the file and `:808` the widest of all.

## The reading

MEASURED 2026-11-22 on this box (linux 7.2.15-1-omarchy, go1.27.3, bubblewrap
0.13.1, `TMPDIR` unset, `os.TempDir()` = `/tmp`) at tip `c28d67f4`, before and
after, by one harness within the same hour.

**The harness READS the file's own format strings** rather than carrying a
copy of them, which is what makes it a reading of the tree and not of a
transcription: it parses `bwrapseatstorelive_test.go` with `go/ast`, finds
every `t.Errorf`/`t.Fatalf`/`t.Logf` whose last argument is the seat account,
walks the format string spending each verb's WIDEST value from the source's
own literal tables, and stops at the verb the account is handed to. The
fixture it prices against is the real `bsLiveFixture`, so the path widths are
measured and not composed. It is a throwaway and is not in the tree; the
numbers are.

Two corrections it makes to the bead's own arithmetic, both in the worse
direction:

- **`:440` was priced with `file`, but `p.path`'s widest value is `side`**
  (`<own>/<seatStoreBaseName>/<sha[:2]>/<sha[2:]>`), 24 characters wider on
  this box. So that row carried 12 characters here and ZERO in the venue, not
  the 263 here and 313 in the venue the bead priced.
- **`:808` was left unpriced and is the worst row in the file.** Its `%q` of
  `c.sh` carries one to three absolute seat-store paths; at its widest
  (`"replace the store with a link to the shared one"`) the account began at
  **518 here and 668 in the venue**.

Where the account's `%s` began, over every account-carrying row in the file,
as a character offset into the 300 `evidence()` carries. Rows are labelled by
their PRE-fix line numbers, which are the ones this bead and
`ranger-base-uu303` name; the `paths` column is how many `TMPDIR`-derived
paths the row interpolated ahead of its account, and it is the multiplier on
both adjustments:

| row | paths | before (here / venue) | after |
|---|---|---|---|
| `:369` digest control | 1 | 338 / 388 | **178 / 178** |
| `:440` shared object changed | 1 | 288 / 338 | **151 / 151** |
| `:454` write at the real name | 1 | 374 / 424 | **173 / 173** |
| `:751` wide grant renamed it | 2 | 395 / 495 | **141 / 141** |
| `:753` refused by something else | 1 | 180 / 230 | **112 / 112** |
| `:765` NOTHING MEASURED control | 1 | 237 / 287 | **169 / 169** |
| `:808` host path reachable | 3 | 518 / 668 | **171 / 171** |

The other eleven rows were already inside the 300 and are unchanged. **After
the fix no row in the file interpolates a `TMPDIR`-derived path at all**, so
`here == venue` for every one of the eighteen and the venue's +50 has left
this file's line budget entirely. The worst account offset went from 668 to
**178** of 300, and the row that now carries LEAST carries **122** characters
of account — where before the fix five rows carried 0 and a sixth carried 13.
Re-read on the committed tree, where the header pushed every line number
down: the same eighteen rows, the same offsets, 0 paths each.

`bgSeatAccount`'s own widths, for what a carried budget buys: 216 runes for a
silent seat whose status is `permission denied` — which is `ranger-base-dxaxt`'s
199-character tail plus that 17-character status — and 40 for the shortest
spoken form. Shortening that tail is `ranger-base-dxaxt`'s and would not have
fixed any row above: they were over 300 before the account began, whatever the
account costs.

## The other two member files with accounts are clean

Checked rather than assumed, because the class is a member-set class and a
fix that stops at one file is the shape `ranger-base-uu303` was filed about.
`internal/posse/storerepogitwalllive_test.go`'s four account-carrying rows
interpolate nothing wider than `r.name` (29 characters at its widest, `plant
objects/info/alternates`), putting their accounts at roughly 60-130.
`internal/posse/bwrapgitrefslive_test.go` carries no account LOCAL at all —
it hands `bgSeatAccount(...)` to the message inline, and in all three such
messages the account is the first or near-first verb. So no row outside
`bwrapseatstorelive_test.go` spends a fixture path ahead of its account, and
this bead's five-rows-at-zero was confined to the one file.

One thing that inline spelling does mean: arm 2 of
`internal/treepins/wallmemberaccount_qa_test.go` keys on account locals bound
from a runner's last string result, so the three inline
`bgSeatAccount(...)` calls in `bwrapgitrefslive_test.go` are outside what it
reads. Not this bead's to change — recorded because the next widening of that
key will want it.

## The fix

Shorten the first line, which is the one lever ADR 0097 D3 leaves. The bead
measured the two that look like levers and are not:

- the paths cannot go on a line 2 — everything after the newline is dropped in
  transit, and `internal/treepins/wallmemberaccount_qa_test.go` arm 2 reds a
  newline in any format string interpolating an account local;
- they cannot go in a `t.Logf` beside a `t.Errorf` of the account, because
  `evidence()` takes the FIRST `file.go:NN:` line and the `Logf` would win.

So each row names its paths the way this file's prose and ADR 0100 already do
— `<common>`, `<common>/worktrees`, `its real name` — and names an object by
`sha[:12]`, which is what the read-backs in the same file were already written
in. Three values came out because they were a SECOND SPELLING of something
already in the line:

- `:369`'s `got %q` of `out` — `bgSeatAccount(err, out)` *is* the seat's
  words, so the row was spending 20 characters saying twice what reaches the
  bead once;
- `:440`'s `p.path` — `p.where` already names which path the row typed;
- `:808`'s `%q` of `c.sh` — `c.name` is the same claim in words, and the shell
  line is a literal in the table two lines above it.

Two explanatory clauses moved into the comment above their row rather than
being deleted: `:751`'s consequence (a leftover plants at
`<common>/worktrees` and the next launch binds it read-write, no race to win)
and `:454`'s (a failure there means the real-name rows above refused the
fixture rather than reading the wall). The second was already in that `if`'s
own paragraph nearly verbatim. A path is in the source at the line number the
bead carries; an account is not — that is the whole ordering principle, and
the file header now says so.

## Verified

- `go test ./internal/posse -run TestLiveBwrapSeatStore` — 5 tests green,
  `wall venue: measuring (ADR 0097)` emitted, so they measured rather than
  skipped.
- **One live mutation, end to end**, because a width computed is not a width
  rendered: `:751`'s `case ok:` forced to fire against the real seat renders
  402 characters, the account beginning at 141, so `evidence()` carries
  `exit status 1, and the seat said "mv: cannot move '…/worktrees' to …` —
  the status, `mv`'s own error and the path the seat named. Before the fix the
  same row carried none of it.
- `internal/treepins` arms 1, 2 and 3 of the wall-member account census green
  (24 member files, 33 launching functions, 40 child errors graded, 2 wrapper
  reads), and the nest-gate census beside them. No account interpolation was
  removed, so arm 2's floor reads what it read.
- `make fmt-check`, `make tree-check`, `make test`.

## Not closed here

- `ranger-base-dxaxt`
