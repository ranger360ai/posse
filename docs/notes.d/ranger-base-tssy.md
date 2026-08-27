## A BSD/GNU `||` chain only discriminates if the WRONG arm FAILS (ranger-base-tssy)

`scripts/verify-bd-pin.sh` read the pinned binary's mtime with the shape that
looks obviously portable and is not:

    bin_mtime=$(stat -f %m "$want_bin" 2>/dev/null || stat -c %Y "$want_bin" 2>/dev/null)

`-f` is not the same flag in the two `stat`s. On BSD/macOS it is the FORMAT
flag. On GNU coreutils it means DISPLAY FILESYSTEM STATUS and takes **no**
format, so `stat -f %m FILE` reads `%m` and FILE as two *file operands*: it
prints FILE's filesystem block on **stdout**, and only then exits non-zero on
the missing `%m` — so the fallback ran as well and appended the real epoch.
`bin_mtime` came out as blob + number. Measured in `golang:1.26`:

    $ stat -f /repo/go.mod | head -2
        File: "/repo/go.mod"
          ID: 0        Namelen: 255     Type: UNKNOWN (0x6a656a63)

The failure was not a red check. It was a **green** one. The STALE arm —
`[ "$start" -lt "$bin_mtime" ]` — errors and goes false, and the
"age unverified" arm below it *also* goes false because `bin_mtime` is
non-empty. The verdict falls through to `ok`. On linux the script printed
`ok` for a daemon it had never checked, and ended "pin intact … command layer
and process layer agree": exactly the 08-16 command-layer-only verdict
`ranger-base-tdwy` exists to prevent, reintroduced on the other platform.

Two properties, both worth keeping:

- **Probe order must be chosen so the wrong arm fails.** GNU first is safe
  here because BSD `stat` rejects `-c` outright (`stat: illegal option -- c`,
  nothing on stdout). `-f` first is not safe, because GNU `-f` succeeds at
  printing something. Exit status is not a platform detector unless you have
  checked that the losing side actually loses.
- **Validate the value, not the exit status.** Every epoch now goes through
  `epoch()`, which drops anything that is not all digits. A third `stat` that
  answers wrong lands in "age unverified" — the honest arm — and can no longer
  reach `ok`. `proc_start`'s `date -j … || date -d …` chain (which *does*
  discriminate: GNU `date` rejects `-j`) is validated the same way, because
  its result feeds the same `-lt`.

`scripts/verify-id-recycle.sh`'s `sock_ino` had the identical inverted probe
and is fixed the same way. That is the whole of the smell in `scripts/`; the
other `||` chains there are between commands, not between flag dialects.

**The pin now runs on both hosts.** The live bug was reachable only from
linux, so only `make test-linux` ever saw it — a QA test that is red on one
platform is a test the mac never runs. `bpStubStat` puts a `gnu`, `bsd` or
`broken` `stat` on the stub PATH, so darwin exercises the GNU case too:
`TestQABdPinReadsBinaryMtimeWhicheverStatIsInstalled` (stale *and* young per
flavor — young catches a "fix" that just returns empty) and
`TestQABdPinCallsAnUnreadableMtimeUnverifiedNotOk`. All three go red against
the old one-liner on darwin.
