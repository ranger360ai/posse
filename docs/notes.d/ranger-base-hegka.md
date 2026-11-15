# ranger-base-hegka — three findings of the ranger-base-eclrz verify

Three live findings that escaped four verified closes (ranger-base-m1obc,
-np54s, -g1jqt, -yj80h). Each is fixed here, each has a pin, and each pin was
run against its own defect planted back before it was trusted.

## Finding 1 — an unlocked watch-stream read at two backuploop sites

`internal/posse/backuploop_test.go:511` and `:629` read the Dispatcher's
`*strings.Builder` through `dispatcherOut` while a `d.Watch` goroutine was
still writing it. That is the class ranger-base-np54s closed, at two sites its
census could not see.

**Why the census missed them, which is the reusable part.** np54s keyed it
"a `for` loop POLLING dispatcherOut" and was right about what it asked:
exactly one loop in the package polls it. These two loops poll the
FILESYSTEM (`archives(...)`) and read the stream only in their deadline arm.
The hazard is "a read taken while a goroutine that writes `d.Out` has not been
joined", and keyed that way the same package held two more.
`TestQANoUnlockedWatchStreamReadBeforeItsJoin`
(`internal/posse/watchoutlive_qa_test.go`, door `make pkgcensus-check`) is
that key as a pin.

**The fix was not a one-word swap**, and the measurement is why.

MEASURED 2026-11-15, this box (Linux 7.2.15-1-omarchy, go1.27.3, 16 cores,
warm build cache), worktree `posse/jian-yang-posse-ranger-base-hegka`, with
the deadline at line 507 aimed into the loop's first milliseconds so the arm
actually executes — `go test -race -tags posse_arm3 ./internal/posse
-count=20 -run '^TestWatchBackupClockWritesAnArchive$'`:

| tree | deadline | WARNING: DATA RACE | racing writer |
|---|---|---|---|
| before | 1ns | 2 per run | `teeWatchLog` → `d.rawOut`, `d.Out` (watch.go:553/554) vs `baseOut` |
| before | 100µs / 1ms | 1 and 3 over 20 runs | `watch.go:232/241/259/283` Fprintln vs `Builder.String` |
| Builder half only | 1ms | 8 over one 20-run batch | `watch.go:232` etc. — a read under `outMu` against a write that takes none |
| both halves | 1ns … 10ms, 6 deadlines × 20 | 0 | — |

So three writers had to come under the lock, not one:

- `teeWatchLog`'s swap of `d.rawOut`/`d.Out`/`d.Err` (`watch.go`) — it runs on
  the WATCH goroutine at every one of the 26 test sites that launch `d.Watch`
  on a goroutine, so "it runs in Watch's preamble before any writer exists"
  was false there. (26, MEASURED 2026-11-15 over `internal/posse/*_test.go`:
  18 one-line `go func() { … }()` and 8 in block form, listed by the scan in
  this bead's transcript. The bead that filed this said 17, which is the one
  number of its own I could not reproduce; nothing downstream turns on the
  count, and the comments that cite it say 26.)
- Watch's five header Out writes, through `d.quietf` and a new
  `d.quietOutWriter()` — `dispatch.go`'s writer family had no quiet Out cell
  and its doc said so because nothing took one. The backup clock's goroutine
  is started ABOVE that block, so it was not alone on the stream even in
  production.
- the two reads themselves, now `dispatcherOutLive`.

`TestQAWatchStreamWritesGoThroughTheDispatcher`'s allowlist is five entries
shorter, and the premise it stated for the whole header block ("before the
first clock goroutine and the first gather exist") is corrected in place: the
half about being no sign of life is true and is why these lines stay QUIET.

Not ranger-base-87p6i: that bead adds these files to `scripts/test-race.sh`'s
CANDIDATE_FILES, and the racing read is in a 60s deadline arm that never
executes in a green run, so the `-race` arm would have been green over these
two sites even with the file in its list.

## Finding 2 — a refusal's matched block ran into the way-through

`visGuardRefusal.render` printed the matched lines with `printf '%s'
"$badVar"` and no trailing newline, so the last matched line and the
way-through sentence rendered as ONE line — at the commit-guard refusal this
chain (ranger-base-li3up → -g1jqt) exists to make readable.

The arms that build a block line by line embed a trailing newline and
rendered correctly; the arms whose badVar comes out of a `$(command
substitution)` cannot, because that strips every trailing newline whatever
the Go side writes. `posse_matched_block` (rendered once into the commit
guard) gives exactly one newline whatever it is handed — a `$( )` strip and a
`printf '%s\n'` — so both shapes render right and neither is double-spaced.
Four matched-block sites print through it now.

The bd shim's own data-ceiling refusal is deliberately out of scope: it is a
different render with no helper in it, and its block is built line by line,
so it already ends its own line. The census arm reads the commit guard alone
and says so.

**Why no pin caught it:** every refusal-text pin reads by
`strings.Contains`, so none of them can see a missing newline BETWEEN two
substrings that are each present. `TestQAEveryRefusalMatchedBlockEndsItsOwnLine`
executes the rendered refusal under `sh` and asserts the way-through STARTS a
line; with the defect planted back it reds on the command-substitution shape
and on the two-line shape, and stays green on the embedded-newline shape
(MEASURED 2026-11-15).

## Finding 3 — check 4 read "could not look" as "clean"

Pre-existing since ranger-base-ihp7y introduced ADR 0024 D2 check 4. The
judge's call discarded stderr and never consulted the exit code, so an empty
stdout from a FAILURE — no posse on PATH, a session gate refusing the verb, a
store error, a panic — was byte-identical to an empty stdout from a clean
scan: exit 0, zero bytes, nothing said. "A check whose only signal is output
passes by producing none", failing in the direction that lets a disclosure
land.

It still fails OPEN — a scan that could not run is not a finding and the
committer has no remedy for a missing binary (ADR 0068) — and it now SAYS it
was skipped (`posse_vp_skipped`, once per commit for both arms). The three
answers are told apart by the judge's exit code, which this is the first shell
to read:

| answer | exit | stdout |
|---|---|---|
| a hit | nonzero | the hit lines |
| a clean scan | 0 | nothing |
| could not look | nonzero | nothing |

That makes the exit code part of the contract, and `cmd/posse/main.go`'s
comment (which said the shell never reads it) says so now.
`TestQACheckFourSaysWhenItCouldNotLook` drives all four cases through the real
rendered arm with a shell function standing in for the binary, and
`TestQAVulnPrivateScanPrintsExactlyWhenItAnswersAHit` holds the Go half —
a hit prints and a clean scan does not, which is what makes "nonzero with
nothing on stdout" mean one thing. With the defect planted back the pin reds
on all three could-not-look legs and stays green on the hit and the clean scan
(MEASURED 2026-11-15).

Reach, stated as the verify stated it: every seat has posse via its gate shim
and no crew PID denies `posse gates`, so no seat was in this state. What makes
it worth fixing is that the guard could not tell a refusing shim, a store
error, a panic or a timeout from a clean message.

## Not closed here

Nothing. All three findings are fixed here. The verify's own suite caveat —
`make test` red at doc-check, ranger-base-0e5zd — was closed on main before
this work started, and `make tree-check`'s doc-check door is green on this
tree.
