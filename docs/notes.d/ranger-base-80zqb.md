# The two brackets were backwards, and the reader was never going to close the class (ranger-base-80zqb)

2026-10-31. Both live findings of ranger-base-3fhkg, which verified
ranger-base-mk6gw's close. Both are about
`internal/treepins/brewguard_qa_test.go`, the pin that holds the two Homebrew
guards every brew-reading gate must export — `HOMEBREW_NO_ANALYTICS=1`
(ranger-base-w2bha, ruled SUPPRESS: an unguarded read POSTs a `command_run`
event to Homebrew's InfluxDB, which is crew hard risk line 4) and
`HOMEBREW_NO_AUTO_UPDATE=1` (ranger-base-et7bs: a stale cache re-downloads
~16 MB of package JSON that the next two gates answer out of).

## Finding 1 — the reachability reading did not count parentheses

ranger-base-mk6gw gave `bgRead` a notion of reachability: an export counts only
at column 0, block depth 0, outside a heredoc. Depth counted the block words
and `{`/`}`. It counted `(` and `)` nowhere, so an export in a top-level
subshell, or in a function whose body is a subshell, sat at column 0 with depth
0 and was recorded as a top-level guard — the ranger-base-f5lu1 survivor
re-spelled with the other bracket, against the fix filed to close it.

The two brackets ended up treated exactly backwards from what they do.
MEASURED 2026-10-31, `scripts/verify-grok-pin.sh:80` mutated four ways and run
with a stub `brew` that printed its own environment:

| shape at :80 | guards in the brew child |
|---|---|
| plain top-level `export …` (golden) | `ANALYTICS=1 AUTOUPDATE=1` |
| `(\nexport …\n)` | `ANALYTICS=<unset> AUTOUPDATE=<unset>` |
| `never_called() (\nexport …\n)` | `ANALYTICS=<unset> AUTOUPDATE=<unset>` |
| `{\nexport …\n}` | `ANALYTICS=1 AUTOUPDATE=1` |

So `( … )`, which exports into no later child, was counted as a guard, while a
top-level `{ … }` group, which really does export into the current shell, was
refused as shadowed. The permissive bracket was the one that sets nothing.

### Counting the parens is the obvious repair and it is the wrong one

A `case` pattern ends in a bare `)`, so a paren count goes negative on the
first pattern of every argument parser in the tree. MEASURED 2026-10-31 over
all 56 files under `scripts/`, with the counting rule mirrored exactly:

| rule | files that LOSE the block count |
|---|---|
| block words + `{`/`}` (as shipped) | 3 of 56 |
| the same plus `(`/`)` | **38 of 56** |

All four of `bgRoster`'s brew callers were among the 38. The count would still
be *lost* rather than wrong — the strict direction — but a reader that can
place nothing below line ~120 of every gate has a strictness that no longer
means anything. This is the measurement behind diverging from the fix shape
ranger-base-3fhkg spelled for option (a).

### What shipped instead: a paren is a structure the reader cannot place

A line carrying `(` or `)` is recorded in `lost` and nothing from that line on
is placed — that line included, because `( export X=1 )` and
`export X=$(date)` are the same text to a line-based scan and only one of them
runs. In effect the export must sit in the run of lines above the first
bracket of any kind, which is where all four callers already put it.
MEASURED 2026-10-31:

| script | export at | first parenthesis at |
|---|---|---|
| `scripts/verify-grok-pin.sh` | :80 | :82 (`cd "$(dirname "$0")/.."`) |
| `scripts/verify-codex-pin.sh` | :62 | :64 |
| `scripts/verify-bd-pin.sh` | :75 | :77 |
| `scripts/macos-install-probe.sh` | :87 | :100 |

It costs a real shape — `export A=1 B=1 NOW=$(date)` does run and is refused —
and `TestQATheBrewGuardReaderCanStillSayNo` now carries that row so the cost is
written down rather than discovered.

### And the static reading is no longer the only one

A rule about brackets is still a parser learning one more bracket, so
`TestQABrewGuardsReachTheBrewChild` proves the claim BY CONSTRUCTION for every
gate that can be run: it runs the gate with a stub `brew` that records its own
environment and reads the guards out of the CHILD, where an export the shell
never executed simply is not there. That is option (b) of ranger-base-mk6gw's
own finding 1, which its close took option (a) over without saying why.

Its child environment is built from a literal four-entry list and deliberately
not from `os.Environ()`, and the arm asserts that list carries neither guard —
otherwise a variable the parent `go test` already held would arrive in the
child and read as the script's own export.

The fixtures are the minimum that REACHES brew, and finding that minimum was
the measurement: MEASURED 2026-10-31, with a stub `brew` on PATH and nothing
else, all three gates exited 2 before calling brew once — `grok`/`codex`/`bd`
not on PATH — and two of them then wanted a `config.toml`. A stub subject
binary and an empty config file are enough; the gates' own rows fail against a
fixture that thin and their exit codes are not asserted. "Called brew zero
times" is a fatal, because that is what this arm measuring nothing looks like.

### One rostered caller cannot join, and that is why the reader was repaired

`scripts/macos-install-probe.sh` reads brew at an ABSOLUTE path in `paths`
mode — `/opt/homebrew/bin/brew --prefix`, falling back to
`/usr/local/bin/brew` (:206) — because that mode's whole subject is the
OPERATOR's own brew and whether its prefix is on the default PATH. An absolute
path consults neither `$PATH` nor the shell's functions, so nothing can stand
in for that child and its environment cannot be observed hermetically; the
alternative is running the real brew from the suite, which is the footprint
this pin exists to refuse. `bgNoDynamic` holds that reason, two-way against
`bgRoster`, so the dynamic half cannot cover three of four while reading as
though it covered the roster — and that caller is why the static reader is
repaired rather than deleted.

### Mutation-checked with the paren spellings

MEASURED 2026-10-31 in this bead's own worktree, each mutant md5-checked
changed before the run and the golden restored after, control re-run first:

| mutant of `verify-grok-pin.sh:80` | before | after |
|---|---|---|
| golden | ok | ok |
| `(\nexport …\n)` | ok — **survivor** | FAIL: roster arm, grok arm, AND the dynamic arm |
| `never_called() (\nexport …\n)` | ok — **survivor** | FAIL: the same three |
| `never_called() {\nexport …\n}` | FAIL | FAIL (no regression) |
| drop `HOMEBREW_NO_ANALYTICS=1` | FAIL | FAIL |

The two halves red independently, which is the point of having both.

## Finding 2 — the caller census read one level of a directory it claimed all of

`TestQAEveryBrewNamingScriptIsRosteredOrExempt` was an `os.ReadDir("scripts")`
with `if e.IsDir() { continue }`, under two comments claiming "every file under
scripts/". A brew-calling script one directory down was on neither list and red
nothing. MEASURED 2026-10-31: an unguarded caller planted at
`scripts/zzprobe/newgate.sh` — which `bgRead` reports as one runnable brew
mention with no exports, so the roster assertion would refuse it — left the
census green. With `filepath.WalkDir` it fails and names the path. The same
plant against a `ReadDir` reading of the fixed file passes, which is the
mutation check that the walk is what catches it.

`scripts/` is flat today (56 entries, 0 directories, MEASURED the same day), so
nothing was escaping: this is the claim being made good, not a file being
caught. It is also what makes a roster row's path meaningful below the top
level.

**Walking rather than asserting flatness** was deliberate. ranger-base-3fhkg
offered either, noting that `boxcheck_qa_test.go:405` and
`selftestforkarm_qa_test.go:516` are top-level too, so one flatness assertion
would serve all three. But an assertion that `scripts/` must stay flat is a
layout ruling this pin has no business making, and it reds on the first
legitimate `scripts/lib/`. The walk is correct either way. The other two
censuses are the same latent blindness in other pins' claims and are filed
separately rather than fixed here.

## Lesson that is not about brew

A pin whose subject is "is this line reached" cannot be closed by counting
brackets: the next construct is always unenumerated (`&&`, a preceding `exit`,
a pipeline). What closes it is running the thing and reading the answer out of
the child. Where that is impossible — an absolute path, a live-box subject —
say so by name in a two-way list beside the roster, so the gap is a row
somebody can read rather than a silence.
