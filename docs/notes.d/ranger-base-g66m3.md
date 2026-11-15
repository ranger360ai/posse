# The test-binary cache was writable by every walled seat, and nothing compared what it exec'd (ranger-base-g66m3)

`scripts/gotest.sh` (`make test-reuse`) builds a package's test binary, hashes
it, and execs `$CACHE/<slug>-<sha16>.test` if a file of that NAME is already
there — dropping the fresh build. Nothing compared the file to the build. The
default cache was `~/.cache/posse/testbin`, and `~/.cache/posse` is the one
home cache dir every walled seat is granted write on (`homeCacheGrants`,
`internal/posse/seatbelt.go`: the box's lock dir). So a seat could overwrite the
cached binary of an unchanged package, and the next run of the wrapper by anyone,
the operator uncaged included, exec'd the seat's file. `make test` was never on
this path (it runs `scripts/test-times.sh`).

## The fix, both halves of the bead's option list

1. **Byte equality before reuse.** `reuse()` now `cmp -s`es the fresh build
   against the cached file and reuses only on equality; any difference, or a
   `cmp` that cannot answer, replaces the cached file with the fresh build by
   rename. The good path is still one inode, so the Gatekeeper saving holds.
2. **The default cache left the grant.** It is `~/.cache/posse-testbin` now.
   `~/.cache` left every wall's grant under ranger-base-olr0g, so a sibling of
   `posse` under it is read-only to seats. Half 1 alone leaves a window: the
   cache dir is writable, so a writer can swap the file between `cmp` and exec,
   or swap the fresh build itself before it lands. Half 2 shuts that window for
   the default. A seat that cannot write the default gets a refusal naming
   `POSSE_TESTBIN_CACHE`. It never falls back to a shared dir.

## Pins

- `--self-test`, three new arms: a control proving the plant really runs when
  exec'd; `poison: a planted file under the cached name is not exec'd` (the
  plant is written in place over the cached inode, as a writer of the dir would);
  `poison: the plant is replaced by the fresh build`. MEASURED 2026-11-15
  (Linux 7.2, x86_64): a mutant with the `cmp` replaced by `true` reds both
  poison arms and nothing else.
- `internal/treepins/gotestreuse_qa_test.go`
  `TestQATheWrapperCacheDefaultIsOutsideEverySeatWriteGrant` reads the script's
  `CACHE=` default and `homeCacheGrants`' literals (every GOOS arm), and reds on
  any overlap. Blindness control: the old default must be caught. MEASURED: put
  back, it reds naming `.cache/posse`.

## Not done here

- The old `~/.cache/posse/testbin` is still on the box. Nothing execs from it
  after this lands; removing it is the operator's call.
- An operator who points `POSSE_TESTBIN_CACHE` at a seat-writable dir reopens
  the cmp-to-exec window. The header comment says so.
