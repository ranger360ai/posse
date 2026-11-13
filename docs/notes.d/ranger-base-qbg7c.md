# The residual fixture-root class, and the census that ends the hand count (ranger-base-qbg7c)

*2026-11-12 · from ranger-base-4dw7b, which fixed the last two roots with no
remover · beside ranger-base-7hyn8 (the third site) and ranger-base-w8n6w (the
shell half of this census) · the reaper is scripts/reap-test-tmp.sh and ADR
0089 says why its gates are shaped the way they are*

Four beads in five days walked this tree by hand looking for temp roots
outside posse's namespace, and each found some. 7hyn8 fixed `cmd/coldexec`'s
door build dir and filed the two it saw while it was there. 4dw7b fixed those
two — `cmd/testparallel`'s tool dir and `scripts/pattern-kill-census.py`'s
fixture root — and filed this bead for the residual. w8n6w wrote the first
census, over `mktemp -d` under `scripts/`, and said in its own head comment
that a root created by Go's `os.MkdirTemp` was "pinned nowhere here".

This bead is the other two languages and the end of the hand count.

## What the prefix buys, since it is easy to read as decoration

Every root in the residual HAS a remover: a `t.Cleanup`, a `TestMain`, a
`try`/`finally`, a `with` block. None of them was accumulating — 0 dirs of
every prefix on `/tmp`, MEASURED 2026-11-12, which is the point rather than a
reassurance.

What no remover covers is the half that matters on this box. A SIGKILL and a
`-timeout` panic run no `finally`, no `defer` and no `t.Cleanup`, and this box
kills suites — AGENTS.md's "Ending anything" counts eleven confirmed suite
kills across five personas. The root such a run leaves is then collectable
only by `scripts/reap-test-tmp.sh`, whose `posse-scratch` class is the glob
`posse-*` at a 120m grace; every other class in that table names Go's
`$WORK`, `t.TempDir`'s `Test*[0-9]`, the harness's own per-session
scratchpads, or a dead fixture name kept to drain a pre-fix backlog. A root
outside the namespace is matched by no row, so it is unreapable at any age, by
any spelling of that script, forever — against a `/tmp` that is a 14G tmpfs
with 1,048,576 inodes shared by every seat as one uid, and that hit 100% of
its INODES on 2026-11-11 with the byte gauge reading 29%.

## The eleven sites

The bead named ten. Nine were real, one was not, and the reader found a
eleventh the bead's hand census could not have seen.

| site | was | is |
|---|---|---|
| `internal/posse/bwraphide_qa_test.go` | `os.MkdirTemp("", "bwh")` | `"posse-bwh"` |
| `internal/posse/cageinnerlive_test.go` | `os.MkdirTemp("/tmp", "posse6so")` | `"posse-6so"` |
| `internal/posse/bwraptmp_qa_test.go` | `os.MkdirTemp(bwrapTmpMount, "bwt7wdb7-")` | `"posse-bwt7wdb7-"` |
| `scripts/nodb-set-defer.py:253` | `"nodb-set-defer-selftest-"` | `"posse-nodb-set-defer-selftest-"` |
| `scripts/verify-nodb-defer.py:136` | `"nodb-defer-"` | `"posse-nodb-defer-"` |
| `scripts/verify-nodb-defer.py:298` | `"nodb-defer-selftest-"` | `"posse-nodb-defer-selftest-"` |
| `scripts/page-currency-census.py:471` | `"page-currency-selftest-"` | `"posse-page-currency-selftest-"` |
| `scripts/d3-reader-eval.py:519` | `"d3-reader-eval."` | `"posse-d3-reader-eval."` |
| `scripts/verify-self-close.sh:62` | `/private/tmp/pselfclose.XXXXXX` | `/private/tmp/posse-selfclose.XXXXXX` |
| `scripts/verify-ghost-composer.sh:225` | `/private/tmp/pghost.XXXXXX` | `/private/tmp/posse-ghost.XXXXXX` |

`posse6so` is the one worth reading twice: it looks like it is inside the
namespace and is matched by no class at all, because the glob is `posse-*`
with the hyphen. One byte.

**The eleventh.** `internal/posse/bwraptmp_qa_test.go`'s `bwt7wdb7-` holds its
parent in the package const `bwrapTmpMount` — `"/tmp"`, declared in a
different file of that package (`wallseatcache.go`). No grep for
`MkdirTemp("/tmp"` can see that line and neither can a grep for
`MkdirTemp(""`. It is in the table above because the census found it, which
is the whole argument of this bead in one row.

**The non-site.** `internal/treepins/gittempdirroot_qa_test.go:373`'s
`os.MkdirTemp("/tmp", "x")` is TEXT inside a raw string literal — one of that
pin's planted Go fixtures — and creates nothing. A `go/parser` census is
immune to it by construction, since a string literal yields no `CallExpr`,
which is the second reason the Go half is an AST read and not a grep.

**The two darwin roots keep their hardcoded `/private/tmp`.** That path is
not negotiable there: these gates put a unix socket under the root, `sun_path`
caps the whole path at 104 bytes, and darwin's `TMPDIR` is a per-user
`/var/folders/…` path that spends most of them. `/private/tmp` is also what
darwin's `/tmp` resolves TO, so the reaper's default root already looks there
and the `posse-` prefix is the half of the house template that applies. Their
rows in `mktemproot_qa_test.go`'s `mtExempt` now say that the PATH is the
exemption rather than the whole template. `verify-self-close.sh`'s line is
also pinned by name in `internal/posse/selfclose_qa_test.go`, so this rename
was the two-file change across the arm boundary that ranger-base-tbukc
declined to make.

## The census

`internal/treepins/temproot_qa_test.go`,
`TestQAEveryTempRootInTheTreeIsInPossesTempNamespace`, behind
`make corpus-check` — whose sentence is already "a census over this repo's own
.go sources" — and in `treewidedoor_qa_test.go`'s register, which took the
class to one hundred and forty-two pins behind the same seventeen doors.

THE RULE: a live `os.MkdirTemp`, `tempfile.mkdtemp` or
`tempfile.TemporaryDirectory` whose PARENT is a shared temp directory carries
a prefix beginning `posse-`. No exemption list: complying is six characters,
the namespace constrains nothing but the name, and an exemption mechanism here
would only ever be used to re-open the class.

A SHARED TEMP PARENT is the TMPDIR-defaulting spelling — Go's
`os.MkdirTemp("", …)` and `os.TempDir()`, Python's `dir=` omitted — plus
`/tmp`, `/private/tmp` and `/var/tmp`. The reaper's default roots are `/tmp`
and this shell's `$TMPDIR` when that is elsewhere, so the first four land
somewhere it looks; `/var/tmp` is in the list anyway, because a leak there is
box-wide the same way and the prefix is the whole of what would let
`reap-test-tmp.sh --root /var/tmp` collect one. The tree's single root there
already conformed, so including it cost nothing and closed the hole rather
than waiting for a bead to find it.

THE GO HALF IS A `go/parser` CENSUS over every `.go` file in the tree,
grouped by package directory, with `parser.ParseFile` applying no build
constraints — so internal/posse's untagged, `posse_arm2` and `posse_arm3`
files are all read by one run. That is 2115efb1's escape closed mechanically:
a fix verified under one tag while the sites under another were never
compiled. `trStrings` resolves a name to its string literal across the whole
package and DROPS a name the package binds to two different literals rather
than guessing. The cross-file resolution is what found site eleven.

THE PYTHON HALF IS A LINE READER, the way w8n6w's shell half is, and loose in
the same direction: comment lines create nothing and everything else that
names one of the two calls counts, a line inside a string literal included.
Over-counting costs the author six characters; under-counting is the next bead
in this family.

## The one direction it is loose, and what that cost on the first run

A parent the reader cannot resolve is scored OUT of scope. That is the only
loose direction in the census and it is said in the head comment rather than
half-handled — a PREFIX it cannot read is scored RED, because there the strict
answer is the safe one and the remedy is one line at the call site.

It cost a live site immediately. `internal/posse/wall_helpers_test.go`'s
`/var/tmp` root held its parent in a local `const base`, and internal/posse
binds `base` to a seven-line yaml literal seven times over in
`systemdunit_test.go`, so the name was dropped as ambiguous and the root fell
out of scope: 14 Go roots where a hand count says 15. The fix is at the call
site and not in the reader — that const is `varTmpRoot` now — because
resolving per function instead of per package would have cost the cross-file
resolution that found site eleven.

## What is out of scope, said once

ranger-base-qbg7c's third deliverable was a decision about the roots that are
not under a temp dir, rather than an exemption row per site.

- **A root whose parent is not a shared temp directory** is covered by its
  parent. Three live roots are in that position and each was audited rather
  than assumed: `app_test.go`'s `abbrev-probe-` and
  `readingslog_qa_test.go`'s `rl-repo-` both sit under this test binary's
  temp HOME, which IS `posse-testhome-*`, and `landedcheck.go`'s `run-` sits
  under the launcher's state dir, which is in no temp root at all. Collecting
  the parent takes the child with it; naming the parent's namespace again in
  the child would be a second place for it to drift.
- **A temp FILE**: `os.CreateTemp`, `tempfile.mkstemp`, `mktemp` with no
  `-d`. Every class in the reaper's table finds candidates with `-type d`, so
  a leaked file is collected by no row whatever it is called and a `posse-`
  prefix on one would claim coverage that does not exist. Same line
  `mktemproot_qa_test.go` draws, same reason.
- **`t.TempDir()`**, which the `Test*[0-9]` class already globs, and which is
  the right answer wherever the length of the path does not matter. Three of
  the roots renamed here are `MkdirTemp` precisely because a unix socket under
  them has to fit in 104 or 108 bytes.
- **Shell outside `scripts/`.** `mktemp -d` under `scripts/` is w8n6w's pin
  behind `make scripts-check`; the two censuses are deliberately not one
  reader, since folding them would mean a shell parse or an AST over bash, and
  each says in prose which languages it owns.

## Controls

- **Each of the eight renamed Go and Python prefixes was planted back and
  re-run**, one at a time, from a byte copy rather than from git so the rest
  of the bead's edits to the same file stayed in place: the census reds naming
  the site and the line, eight for eight, and the arm is green over the tree
  as it stands.
- **The control arm** (`TestQATheTempRootCensusCanStillSayNo`) drives both
  readers over planted Go and Python fixtures under `t.TempDir()`: the four
  parent spellings, the head-resolved concatenation, the unreadable prefix,
  and the six readings that must stay OUT — an unresolved parent, an unbound
  name, a name the package binds twice, `os.CreateTemp`/`mkstemp`, a call
  inside a string literal, and `t.TempDir()`. It walks nothing, so it is not
  in the tree-wide class and needs no door.
- **One arm of that control was itself mutated**, because an arm that passes
  either way is a survivor. The Python reader's argument scanner first skipped
  an escaped character with a bare `continue`, which advances to the escaped
  character and then judges it — so `\'` inside a single-quoted literal closed
  the quote, the argument text ran to the end of the line, and a trailing
  comment naming `dir="/opt"` was read as that call's parent. The fixture line
  carries exactly that shape: with the bug restored the root drops OUT of the
  census silently (eleven readings where the arm wants twelve), and the first
  spelling of the fixture — an escaped quote with nothing after it — passed
  both ways, since the prefix regexp handles the escape correctly on its own.
- **Every touched script still passes its own self-test**:
  `verify-nodb-defer.py --self-test` (all arms ok), `nodb-set-defer.py
  --self-test` (all arms ok), `page-currency-census.py --self-test` (40 ok, 0
  BAD), `verify-ghost-composer.sh --self-test` (4 PASS), `d3-reader-eval.py`
  compiles, and both shell gates pass `bash -n`. `verify-self-close.sh` has no
  `--self-test` and is darwin-only; on linux it stops at the `mktemp` for want
  of `/private/tmp`, exactly as it did before the rename.

## Numbers

MEASURED 2026-11-12, linux, this box:

| | |
|---|---|
| `.go` files walked | 1026, in 8 package directories |
| `.py` files walked | 15 |
| in-scope Go roots | 15 |
| in-scope Python roots | 7 |
| outside the namespace before this bead | 8 (3 Go, 5 Python) + 2 darwin shell |
| census wall | 0.30s (the cost of parsing 1026 files) |
| control arm wall | 0.00s |
| `make corpus-check` with it | 5.9s |

## Not closed here

- ranger-base-ym8pl
- ranger-base-m0j9n
