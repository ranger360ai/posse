# A launcher TMPDIR outside /tmp leaves the bwrap write set too (ranger-base-1c91x)

*2026-11-12 · dinesh · follows ranger-base-7wdb7*

## What changed

ranger-base-7wdb7 gave every bwrap seat its own `/tmp` and handed it
`TMPDIR=/tmp`. `bwrapDropTmpGrants` then took the launcher's `TMPDIR` out of
the write set, but only when that TMPDIR lay under `/tmp`. A launcher TMPDIR
anywhere else, under the home for example, was still bound read-write into
every seat on the box. The seat was no longer pointed at it, but it could
still write there, and it was a directory every seat shared with the
launcher. Now the TMPDIR is dropped wherever it is, whenever the seat has a
private `/tmp` (`internal/posse/wallseatcache.go`). A PID's named
`writable:` extra under `/tmp` still stays, as before.

## Pins

In `internal/posse/bwraptmp_qa_test.go`:

- `TestBwrapDropTmpGrantsTakesATmpdirOutsideTmp` covers the producer. It
  checks that a TMPDIR under the home goes, that a sibling whose name has it
  as a prefix stays, and that an empty TMPDIR drops only `/tmp`.
- `TestQABwrapRenderGrantsNoLauncherTmpdirOutsideTmp` covers posse's own
  `bwrapRender` over a fixture outside every temp grant, with TMPDIR set
  under the fixture home. It first runs a control: `SeatbeltWritable` does
  grant that TMPDIR, so a pass means the grant was dropped and not that it
  was never there. The rendered set and argv must not carry the TMPDIR, and
  the argv must still say `--setenv TMPDIR /tmp`.

MEASURED 2026-11-12, D1 host. Restoring the old `underDir(/tmp, TMPDIR)`
guard turns both new pins red, and removing it again turns them green.

## Not closed here

- ranger-base-gzl2w
