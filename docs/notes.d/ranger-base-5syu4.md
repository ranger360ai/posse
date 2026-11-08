# The 38 `#!/bin/bash` literals were never in a render (ranger-base-5syu4)

This is the audit ADR 0080's own parenthesis asked for, taken 2026-11-08. Its
Consequences recorded "the 38 `#!/bin/bash` literals in posse's Go renders (a
grep, not an audit) are a portability debt, not a blocker" — because that host
has `/bin/bash` and a NixOS or minimal-image host would not.

## What the grep was right and wrong about

Right about the count. Wrong about the place, and the place is the whole of
what a reader does next.

**MEASURED 2026-11-08, this tree at 8503a6ea.** `grep -ro '#!/bin/bash'
--include='*.go'` over the repository: 38 occurrences, every one of them in
`internal/treepins/*_test.go`.

| file | occurrences |
|---|---|
| `internal/treepins/brewguard_qa_test.go` | 22 |
| `internal/treepins/bdpin_qa_test.go` | 9 |
| `internal/treepins/grokpin_qa_test.go` | 4 |
| `internal/treepins/codexpin_qa_test.go` | 2 |
| `internal/treepins/selftestforkarm_qa_test.go` | 1 |

Non-test Go under `cmd/` and `internal/` — the class the bead names — holds
**zero**. The same grep restricted to it returns nothing, and never did
otherwise in this tree: posse's renders were `#!/bin/sh` already, all of them,
in `internal/posse/gates.go` (the gate shim, the gate shell, `PrePushHook`,
`commitGuardHead`), `cageinner.go`, `hooksredirect.go`, `adrcensus.go` and
`runtimeprobe.go`, with `egress.go`'s proxy the one non-shell render
(`#!/usr/bin/env node`). So the ADR's "portability debt in the renders" names
a debt the renders do not carry.

Nor do they carry it one level down, where it would be easy to miss. The one
place posse looks for bash at all is `realShell` (internal/posse/gates.go),
which resolves `zsh` then `bash` on PATH and falls through to `/bin/sh` with
the comment "Every image has /bin/sh" — so the gate shell on a bash-less host
is already decided and already correct. The only other `/bin/bash` in non-test
Go is a row in a measured signal table in `loadguardkill.go`'s head comment,
which is prose about a reading taken on darwin.

What the 38 are is the SUITE, which is a different dependency with the same
cause: eighteen of them were executed by a test through `WriteExecutable`, so
on a host without `/bin/bash` those arms fail at exec and nothing else in the
tree does. Not a blocker for ADR 0080's host either, and now not a debt at
all.

## The classification, all 38

Two kinds, and the kind decides the answer. A shim WriteExecutable makes
executable and a test then runs is a script, and a bashism in it is real. A
body written with `os.WriteFile(…, 0o644)` for `bgRead` to parse is TEXT: it
is never exec'd, and `bgRead` skips any line starting `#` before it reads
anything, so the shebang's content is inert to it and only its being one line
matters.

**Executed shims — 18 occurrences, 17 → `#!/bin/sh`, 1 → `#!/usr/bin/env bash`.**

| site | body | → |
|---|---|---|
| `bdpin_qa_test.go:110` `bpStubBD` | `[ … ] && shift`, `if`, `echo`, `exit` | `/bin/sh` |
| `bdpin_qa_test.go:134` `bpStubBrew` | quoted heredoc | `/bin/sh` |
| `bdpin_qa_test.go:177` `bpStubPS` | **`pid=${@: -1}`** | `/usr/bin/env bash` |
| `bdpin_qa_test.go:195` lsof stub | `while [ $# -gt 0 ]`, `case`, `awk` | `/bin/sh` |
| `bdpin_qa_test.go:287` `stat` gnu | `for f in "$@"`, `printf` | `/bin/sh` |
| `bdpin_qa_test.go:308` `stat` bsd | `${1#-}` (POSIX) | `/bin/sh` |
| `bdpin_qa_test.go:318` `stat` broken | `echo`, `exit` | `/bin/sh` |
| `bdpin_qa_test.go:324` ×2 | the prefix splice and its `TrimPrefix` | `/bin/sh` |
| `brewguard_qa_test.go:761` subject binary | `echo`, `exit` | `/bin/sh` |
| `brewguard_qa_test.go:770` brew recorder | `{ … } >>"$F"`, `${NAME-default}` | `/bin/sh` |
| `codexpin_qa_test.go:110` | `if [ … ]` | `/bin/sh` |
| `codexpin_qa_test.go:175` | `case … esac` | `/bin/sh` |
| `grokpin_qa_test.go:104`, `:109` | `if`, quoted heredoc | `/bin/sh` |
| `grokpin_qa_test.go:227` brew stub | `if`, `printf`, heredoc | `/bin/sh` |
| `grokpin_qa_test.go:623` | `if`, heredoc | `/bin/sh` |
| `selftestforkarm_qa_test.go:91` | `kill -TERM $$` | `/bin/sh` |

`bpStubPS` is the only real bashism in the set: `${@: -1}` is bash's offset
expansion over the positional parameters and POSIX has no spelling for it. It
takes `#!/usr/bin/env bash` and a comment naming the bashism — bash through
PATH rather than at a fixed path, which is the right trust level for a test
fixture and emphatically not for a gate shim, where the fixed path is the
point. (A POSIX rewrite exists — `for pid in "$@"; do :; done` leaves the last
positional in `pid` — and was not taken: the bead's rule is to reclassify the
shebang, not to rewrite the apparatus the arms are measured against. If a
bash-less host ever has to run this suite, that one line is the whole of what
is left.)

**Fixture text, never exec'd — 20 occurrences, all → `#!/usr/bin/env bash`.**

`brewguard_qa_test.go:352-428` (nineteen table rows of
`TestQATheBrewGuardReaderCanStillSayNo`) and `:504`
(`TestQATheBrewGuardReaderRefusesAStructureItCannotPlace`). These are input to
a line-based reader, so the change is not a portability fix; it is fidelity.
Every file these rows stand in for is a real script under `scripts/`, and not
one of those says `#!/bin/bash` — the tree runs `#!/usr/bin/env bash` (32 of
them) and `#!/bin/sh` (10), with 13 more on `#!/usr/bin/env python3`. A fixture spelling a shebang the corpus does not
use was misleading about what the reader reads. The line count is unchanged,
one line either way, and every assertion in those rows is a line NUMBER, so
nothing in them moved.

## Out of scope, named

`docs/adr/0059-packed-refs-lock.probe.sh` carries `#!/bin/bash` and keeps it:
it is not Go, and it drives `sandbox-exec` with a hand-rendered seatbelt
profile, so it is darwin-only by construction and darwin always has
`/bin/bash`. `scripts/shell-syntax.sh` and `docs/notes.d/ranger-base-mz8ud.md`
name the string in prose — `interp_of`'s doc comment listing the shebang
shapes it answers for, and the record of what herdr's own generated hook
launcher has baked in. Neither is a shebang.

## The pin

`internal/treepins/bashshebang_qa_test.go`,
`TestQANoGoSourceWritesABinBashShebang`, doored in `$(QA_CORPUS_PINS)` —
`make corpus-check`. It parses every `.go` file under `cmd/` and `internal/`,
test sources included (excluding them would exclude the entire subject), and
refuses the string in a STRING LITERAL. Parsed literals and not file bytes for
the reason `bddbflag_qa_test.go` gives: prose about a shebang is not a
shebang, and a byte census reds on the paragraph that explains the rule — this
fragment included.

Floors, so that absence is not satisfied by a walk that read nothing: 906 `.go`
files parsed and 352 literals carrying a shebang of some flavor, MEASURED
2026-11-08, with the floors at 400 and 150. One roster row, the pin's own file,
which holds `shbFixedBash` and plants it; the negative witness scans a planted
file in both literal spellings, interpreted and raw, because every live carrier
is an exemption and a census of exemptions passing says nothing about whether
it can still fail.

What it cannot see, said out loud: a shebang assembled across a concatenation
(`"#!" + "/bin/bash"`). That is the byte census's job and the byte census is
refused above; typing the portable spelling is less work than hiding the other
one.
