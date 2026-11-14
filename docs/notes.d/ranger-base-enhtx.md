# A corpus keyed on the extension, over a directory that holds three entry points without one (ranger-base-enhtx)

`TestQANoEntryPointRunsGoTestOnTheDefaultTimeout`
(`internal/treepins/suitetimeout_qa_test.go`, doored by `make scripts-check`)
is the arm that says no second entry point gets `go test`'s default 10m
timeout back. Its own prose says why it exists: "a second entry point that
runs `go test` itself gets the 10m default back, and it would be green here
while `make test` stayed correct."

It built its corpus from `Makefile` + `scripts/*.sh` +
`.github/workflows/*.yml`, and `scripts/` holds three extensionless entry
points — `posse-land-check`, `posse-landed-check` and, since ADR 0097 D3,
`posse-wall-check`. The glob saw none of them.

## Why nothing was red

The first two name `go test` only in comments, and the arm's `isComment`
skips those, so for most of the hole's life the population it missed was
empty. ADR 0097 D3 then gave `scripts/posse-wall-check` a LIVE run —
`go test -count=1 -json -timeout 15m -run '^Test(QA)?Live(Bwrap|Wall)'
./internal/posse`. It carries the flag and it is at the floor, so nothing was
wrong with the tree; what was wrong was that nothing held it there. Dropping
the `-timeout 15m` from that line reddened nothing before this bead, and the
one check whose whole subject is that question was green over it.

## The corpus rule

`scripts/shell-syntax.sh`'s two rules, which `tsIsShell` in
`internal/treepins/trapstatepick_qa_test.go` already spells for this package:
a tracked `.sh`, or a file whose first line is a shell shebang. The arm walks
`scripts/` with `filepath.WalkDir` and `mtWalkInto` — the same shape
`mktemproot_qa_test.go` and `trapstatepick_qa_test.go` use, and the WalkDir
call stays inline in the arm because `treewidedoor_qa_test.go` derives this
class from an enumeration rooted at a literal relative path.

Not "every file under `scripts/`": the `.py` censuses there
(`pattern-kill-census.py`, `suite-entry-census.py`, `bd-argv-gate.py`) carry
`go test` inside string literals and regexes, and the arm's `isComment` only
understands a leading `#`, so a blanket widening reds them. `tsIsShell`
excludes a python shebang, and `scripts/silent-reverts.allow` — whose first
line is `#` and not `#!` — with it.

MEASURED 2026-11-14, this box, over the widened corpus: 53 files, 49 of them
shell under `scripts/` (of 64 files there), 3 of those 49 extensionless, 3
workflows, 31 `go test` invocations seen. The floors are 45 files, 40 shell
and 1 extensionless — under the reading, so an ordinary landing does not move
them, and a reader that goes back to keying on the extension reads 0
extensionless and fails on that line rather than on an absence.

## The two things widening it turned up

Both are in `scripts/posse-wall-check`, both were found by running the
widened corpus before trusting it, and neither is in the bead's own
description of the fix.

**The live invocation is an ARRAY.** It is written

    pins_argv=(go test -count=1 -json -timeout 15m -run "$member_pattern" ./internal/posse)

so that `--plan` can print the argv the run would exec rather than a second
spelling of it. `adjacentGoTest` looked for a word that was exactly `go`,
`$(GOBIN)`, `${GOBIN}` or `$GOBIN`, and `pins_argv=(go` is none of them — so
the widened corpus on its own would have added the file and still seen
nothing in it. `goWord` now strips a leading `(` or `NAME=(` first. An argv
recorded on one line and exec'd on another is as much a run as a bare one.

**The same file hands python a 130-line program.** Line 246 is

    printf '%s\n' "$pins_json" | python3 -I -c '

and one line of the parser's docstring reads

    not go test framing. One rule for a failure line and for a skip line,

which a line-oriented reader over shell reports as an entry point on the
default timeout. That was the only false positive in the whole widened
corpus — 31 invocations, one line of prose among them.

Two changes answer it, and the split between them is the point:

* `reparsesItsArgument` already opened a quoted word handed to `-c`, because
  `sh -c "go test ./..."` really is an entry point. `-c` is not a shell's
  flag alone, though, so it now counts only when the command it belongs to
  IS a shell. `commandBefore` walks back past flags to find that command,
  which is what `python3 -I -c` needs. This is ranger-base-quqn's rule — a
  quoted word is data, not argv — applied to a quoted PROGRAM.
* the body spans lines, so the reader keeps one piece of cross-line state:
  `embeddedProgramQuote` arms a skip when a quote opens after a non-shell
  `-c` and does not close on that line, and `afterEmbeddedProgram` hands back
  the remainder of the line that closes it (`' "$go_rc" … || rc=$?` is shell
  again).

**Why armed rather than tracked.** Carrying quote state across every line of
every script would desync on the first apostrophe in a heredoc body — the
English in `scripts/verify-box.sh`'s ROSTER/EXCLUDED/UNTARGETED tables is
full of them, and that shape is the whole reason `scripts/shell-syntax.sh`
exists — and a desynced tracker SKIPS, which is blindness with no symptom. So
the default stays "read this line for an invocation" and only an explicit
non-shell `-c` opener suspends it. A shell's `-c` is deliberately not armed:
those lines are shell, and hiding an entry point in one is exactly the
route-around the arm exists to catch. A single-quoted body cannot contain a
single quote — the shell would end the word — so for the live spelling the
skip's extent is exact; a double-quoted body carrying an escaped `\"` ends
the skip early, which is the sensitive direction and is left alone.

The per-file scan is one function, `scanEntryPoints`, used by the sweep and by
the detector's own arm, for the reason `trapstatepick` and `trapsignalexit`
share `tsRead`: two readers are two answers to "what is on this line", and
they can come to disagree.

## Verified

`gofmt -l` clean, `make tree-check` green, and the arm was run against two
mutants of `scripts/posse-wall-check:186`, both restored afterwards:

* `-timeout 15m` removed → `scripts/posse-wall-check:186 runs `go test` on
  the default 10m timeout`
* `-timeout 15m` → `-timeout 8m` → `-timeout 8m0s is below the 15m0s floor`

Before this bead both mutants were green.
