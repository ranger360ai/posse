# ranger-base-uu303 — the account's place inside the 300, measured

`ranger-base-momix`'s finding, filed off `ranger-base-mklu0`'s finding 2.
`internal/treepins/wallmemberaccount_qa_test.go` arm 2 pins that a seat
account is rendered BEFORE the first newline. It does not pin where inside
`scripts/posse-wall-check`'s `evidence()` 300 the account lands, and mklu0's
finding 2 was exactly that: 492 rendered, 300 carried, 261 of the 300 spent
before the account.

This bead's stopping condition was a reading: **what fraction of the member
set's account messages would be cut at 300, and on which boxes.** The
alternative it named — "none, and the headroom is wide, so the order fix
closed the class" — is refuted. Every number below is MEASURED 2026-11-22 on
this box (linux 7.2.15-1-omarchy, go1.27.3, bubblewrap 0.13.1, 16 cores,
`TMPDIR` unset) at tip `e5221de6`, or DERIVED from the source by arithmetic
the entry spells out.

## The unit is CHARACTERS, not bytes

`evidence()` is Python and slices `text[:300]` over a `str` decoded from `go
test -json`, so the budget is 300 Unicode code points. The member set's prose
is em-dash heavy — one to three per message, 3 bytes each — so a Go-side rule
counting `len(string)` over-reads every message it grades. The first pass of
this reading counted bytes and put the widest message 2 too high. Whoever
writes the pin counts runes.

## What a message spends before the account, read statically

The arm-2 keys, with the format string sliced at the account argument's own
verb: `prefix` is go test's `file.go:NN: ` (the basename, the line and two
characters, which `evidence()` keeps because it strips only indentation), and
`preamble` is the literal text before that verb with the earlier verbs
removed. No interpolated character is counted, so each figure is a FLOOR.

| | member files | with accounts | account messages |
| --- | --- | --- | --- |
| 2026-11-22 | 24 | 3 | **32** |

The three are `bwrapseatstorelive_test.go` (18), `bwrapgitrefslive_test.go`
(10) and `storerepogitwalllive_test.go` (4). `prefix + preamble` runs
**52 to 233 characters, median 105**:

```
233 220 209 154 151 142 141 138 133 132 129 122 117 110 108 105
104  99  97  94  93  83  83  79  78  76  75  75  74  69  68  52
```

## What the account itself costs

`bgSeatAccount` (`bwrapgitrefslive_test.go:255`) returns `<status>` plus one
of two tails. The silence tail is a **fixed 199-character literal** — no box,
no interpolation, no distribution to measure:

| account | characters | leaves a message |
| --- | --- | --- |
| `exit 0` + silence tail | 205 | 95 |
| `signal: killed` + silence tail | 213 | 87 |
| `exit status 128` + silence tail | 214 | **86** |

The words branch is `<status>, and the seat said ` + `wwOneLine(out)`, so it
costs 35 characters plus the quoted fold of whatever the seat said. Eight real
refusals, MEASURED by running them under bwrap on this box in the shapes the
member set measures (`bwrap --dev-bind / /` with a `--ro-bind` over the
fixture's git dir, objects dir or an in-place `--bind`; the fixture repo built
with `mktree`/`commit-tree` because this PID denies the bare `git commit`):

| the seat's words | account | leaves a message |
| --- | --- | --- |
| `fatal: update_ref failed … cannot lock ref … Read-only file system` | 207 | 93 |
| `fatal: Unable to create '…/.git/index.lock': Read-only file system` (ungranted home) | 153 | 147 |
| `bwrap: Can't find source path /var/tmp/posse-ungranted-…/home/plain/.git: …` | 145 | 155 |
| `error: unable to create temporary file … fatal: Unable to add (null) to database` | 138 | 162 |
| `mv: cannot move '…/wt' to '…/wt.stolen': Device or resource busy` | 137 | 163 |
| `bwrap: Can't find source path /tmp/…/nope/deeper/still: …` | 131 | 169 |
| `fatal: Unable to create '…/index.lock': Read-only file system` | 126 | 174 |
| `bwrap: execvp /no/such/binary: No such file or directory` | 93 | 207 |

`cannot lock ref` is the longest, and it is the one that matters: it is git's
own answer at `exit status 128`, the row `ranger-base-axt1f` exists to keep
apart from `signal: killed`.

## THE FRACTION

Against the governing budget of 86 (the silence account at `exit status 128`),
**21 of 32 (66%) cut the account** — on the literal preamble alone, before one
interpolated character. Against the longest measured words account, 20 of 32
(63%). Against the *shortest* account this class can produce (93 characters,
`execvp`, budget 207), 3 of 32 are still cut.

## THREE MESSAGES WHERE THE ACCOUNT ARRIVES AS ZERO CHARACTERS

The preamble's floor is not what breaks them; their own interpolated fixture
paths are, which is the half the bead said no static bound could see. The
components, each MEASURED or DERIVED:

- MEASURED, `t.TempDir()` under the real test names on this box:
  `…WideGrantPinsTheLooseComponent` 71, `…RefusesRewritingASharedObject` 69.
- MEASURED, the venue's `TMPDIR`: `docs/adr/0097-wall-venue.probe.sh` sets it
  to `$run/tmp` under `$XDG_CACHE_HOME/posse-0097-probe`, which is **54
  characters** here against `/tmp`'s 4 — so every `t.TempDir()` path is **+50
  in ADR 0097 D1's venue**, which is the venue the member set is judged in.
- DERIVED from the source: `f.repo` = `wtRepo` = `gitTempDir` = `t.TempDir()`;
  `f.common` = `<f.repo>/.git` (+5); `wt` = `<f.common>/worktrees` (+10);
  `file` = `<f.common>/objects/<sha[:2]>/<sha[2:]>` (+50); `want.Sum` is a
  sha256 hex (64); `shape.name` is `as rendered` (11) or `with <common>
  granted whole` (27).

| site | arithmetic | this box | the venue |
| --- | --- | --- | --- |
| `bwrapseatstorelive_test.go:751` | 233 + `f.common` + `wt` | **395** | **495** |
| `bwrapseatstorelive_test.go:454` | 220 + 11 + `file` + 2 | **357** | **407** |
| `bwrapseatstorelive_test.go:369` | 129 + `file` + 64 + 2 | **319** | **369** |

All three put the account's own `%s` past character 300, so the account
reaches the bead as nothing at all — not truncated, absent. Row 454 is priced
with the SHORTER `shape.name` and row 369 with an empty `%q` (its `!ok`
branch), so both are floors. A fourth, `bwrapseatstorelive_test.go:440`, is
263 here and **313 in the venue** at its longest literals — under on this box
and over in the one that files the bead. `bwrapseatstorelive_test.go:765` is
287 in the venue: 13 characters of account arrive, which is the status and
nothing after it. `:808` interpolates a `%q` of a shell line carrying two
absolute seat-store paths and is left unpriced rather than claimed.

## WHY A STATIC PIN CANNOT CLOSE THIS, and what can

- **A derived static rule is writable today and is the right NECESSARY
  condition**: `prefix + preamble + the account's own floor ≤ 300`, where the
  floor is read out of `bgSeatAccount`'s source rather than typed. It needs no
  distribution and no runtime reading. At today's 199-character tail it reds
  **21 of 32**, including every `CONTROL:` message whose preamble is a
  complete claim ADR 0097 D3 wants on line 1 — correct, and unlandable without
  21 prose edits.
- **It is not SUFFICIENT.** The three rows above pass any rule over literals:
  what blows the 300 is an interpolated path of 124 to 174 characters. No
  static census can see that, and the three are not a tail — they are the
  file's widest messages.
- **A fitted bound is worse than either.** A floor of 128 reds 11 and catches
  all three — precision 3/11, recall 3/4, missing `:440` in the venue — and it
  has no derivation at all, which is the number nobody measured that the bead
  warned against.
- **The lever is the account, not the 32 messages.** The 199-character tail is
  the single largest consumer in the class, larger than any message's
  preamble, and it is ONE function. At a 60-character tail the derived rule's
  budget becomes 225 and it reds **1 of 32** — at which point the necessary
  condition is landable and the residual is exactly the rows above.

Its content is reasoning, not evidence: the status and `said NOTHING` are the
finding, and "which is the shape of a seat ended from outside rather than one
the wall turned away" is the prose ADR 0097 D3 says costs nothing on line 2.
Shortening it is a decision about what every member reports, and
`bgSeatEndedFromOutside`'s own comment quotes the phrase, so it is filed and
not taken here.

## Reproducing it

The static half is the arm-2 keys of
`internal/treepins/wallmemberaccount_qa_test.go` — `wmaMemberPattern`,
`wmaMemberFiles`, `wmaAccountLocals`, `wmaFormat` — with the format string cut
at the account argument's verb and both halves counted in runes. The account
half is `len([]rune(bgSeatAccount(err, out)))` over the eight refusals above.
The `t.TempDir()` widths are a test whose function names are the real members'.

## Not closed here

- ranger-base-bj1hh — the three sites whose account arrives as zero
  characters.
- ranger-base-dxaxt — the account's own 199-character budget, and the
  necessary condition that shortening it would make landable.
- Sufficiency needs a runtime render venue (each member's messages rendered
  under its fixture and sliced as `evidence()` does). Recorded here rather
  than filed, so the next attempt does not re-try a static rule.
