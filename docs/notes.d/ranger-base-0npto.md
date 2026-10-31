# Two pins that were green over the code they named, and what made each one green (ranger-base-0npto)

2026-10-31. ranger-base-72kew verified four closes and found two pins of the
same class: a test that PASSES over a tree with the judgement it is named
after deleted. One was green because its discriminating signal was a race;
the other because the mutant that proved it was built out of the pin's own
string list. Neither finding disputes the code — both judgements are correct
and both mechanisms work. What was missing was anything that would notice if
they stopped.

## 1. The deadline arms: a race as a discriminator

`readCLIVersionWithin` (internal/posse/runtimeprobe.go) makes one judgement
past its deadline: the version is `""` **even when bytes arrived**, because a
killed child's stdout can hold a truncated first line and a truncated version
line is not unknown to ADR 0007 §6c — it is a release move, so returning it
would file a false drift finding.

`TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer` asserted that judgement
off a live kill. MEASURED by holden, five runs per mutant: deleting the
`ctx.Err()` guard was caught **2 runs in 5**, and deleting
`cmd.WaitDelay = limit` **4 in 5**.

The reason is in os/exec's own documentation: when `WaitDelay` elapses, a
`Stdout` that is not an `*os.File` "may be written to concurrently with the
return from `Wait`", and `cmd.Output()` captures into a `bytes.Buffer`. So
whether the truncated line is in that buffer when `Output` returns is a race.
MEASURED on this box (darwin 25.6.0), a 200ms limit over `#!/bin/sh` + `echo
<line>` + `sleep 25`, five runs with `WaitDelay` set: `len(out)` was **0 on
the first run and 16 on the other four**. With the guard deleted, the run
that read 0 bytes still returns `""` — from the empty buffer, for the wrong
reason — and the arm passes.

**The fix is a split, not a better fixture.** The run stays where it was and
the judgement moved to `cliVersionFrom(out, err, ctxErr)`, which is pure.
`TestCLIVersionFromIsUnknownPastTheDeadline` is a table over the three things
one run produces, including the row that is the whole judgement (deadline
blown, buffer holds `sleepycli 9.9`, want `""`) and its two-way half (killed
for another reason with the line already out, want the line — so the `""`
comes from the DEADLINE and not from the kill). The deadline arms keep the
wall-time bound, which is the thing only a real child can show.

RE-MEASURED after the split, five runs each: the `ctxErr` guard deleted dies
**5/5** (pure test), `cmd.WaitDelay = limit` deleted dies **5/5** (live arms,
each run reporting a 25.15-25.20s wait under a 200ms deadline).

## 2. The fixture note was also wrong about which arm is which

Both the test's comment and `docs/notes.d/ranger-base-2cttm.md` said a
ONE-command body (`sleep N` alone) is bounded with or without `WaitDelay`,
"because `sh -c` execs a lone command and the process the context kills IS
the sleeper". The fixture is a script FILE, not `sh -c`, and darwin's
`/bin/sh` forks the lone command.

MEASURED on this box, a 200ms limit over a 25s sleep, five runs each of five
bodies — `sleep` alone; echo-then-sleep; echo-then-background-sleep-then-sleep;
background-sleep-then-wait; echo-then-background-sleep-then-wait:

| | WaitDelay set | WaitDelay dropped |
|---|---|---|
| all five bodies, 25 runs each way | 0.20-0.41s | **25.01-25.12s, every run** |

So no body behind a shell is bounded by the context alone, and the recorded
"arm 1 passed in 0.20s, arm 2 waited 120.16s" does not reproduce as a
property of the arms. Corrected in all three places that carried it
(the test comment, `internal/treepins/execwaitdelay_qa_test.go`'s header, and
the 2cttm fragment), each marked as a correction rather than rewritten,
because the next WaitDelay fixture gets built from those notes.

The `ps` exemption in ranger-base-2cttm is NOT affected — it rests on `ps`
forking nothing and on there being no shell between posse and it, which is a
different claim. Its appeal to "the fixture's passing arm" is withdrawn; the
ruling stands on its own reasoning.

Arm 2 is now `echo <line>; sleep 25 & wait`. The `&` cannot be exec'd away by
any shell, so the grandchild on the pipes is there by construction instead of
by the shell's choice. That is the shape to copy.

## 3. The trust-table census: a mutant built from the pin's own list

`TestQAPosseNeverWritesGroksTrustTable` is the teeth on ADR 0007 §6f's one
decision — posse names grok's trust table and never writes it, because the
grant is box-wide, outlives the session, and also decides whether grok reads
the repo's own `.claude/settings.json` as a permissions source
(ranger-base-1g8gf, P1).

It required `trusted_folders` and a write verb **on the same source line**,
and its write list contained the string `"SeedGrokTrust"` — a function NAME.
The close's mutation table named "a SeedGrokTrust that writes the table" as
one of its reds, so the one mutant that died was the one built from a literal
in the list under test. MEASURED, three seeders appended to
`skillsdirtrust.go` one at a time:

| mutant | per-line census | per-scope census |
|---|---|---|
| `SeedGrokTrust` with path and write on one line | KILLED | KILLED |
| path on one line, `os.WriteFile` on the next | **SURVIVED** | KILLED |
| `home + "/.grok/" + "trusted_" + "folders.toml"` | **SURVIVED** | KILLED |
| the path hoisted into a package-level const | not tried | KILLED |

The census now decides per SCOPE — one function, or a file's package-level
declarations taken together — over `go/ast` with comments not parsed, so the
prose that must name the table is invisible to it rather than filtered by a
`//` prefix test. All string literals in a scope are joined before matching,
which catches the `+` chain; a package-level string whose own value names the
table makes every scope referencing it a namer, which closes the hoist. The
write list is standard-library and in-package write CALLS only — **no seeder
name is in it**, which is the part that made the old rig circular.

`TestQATheTrustTableCensusCanStillSayNo` plants the fixtures above as
sources, so the mutation run is a test and not a transcript. Its other half
matters as much: a census that answered yes to every file mentioning the
table would pass every positive row and hold nothing, so reading the table,
writing something else under `.grok/`, and naming the table in one func while
writing in another are all pinned green.

## What is NOT here

No tree-wide reach. Both pins read their own package — the posse census
through `os.ReadDir(".")`, as before — so neither becomes a tree-wide pin
owing a Makefile door (AGENTS.md, "the DOORS are held here instead").

No change to either judgement. `readCLIVersionWithin` decides exactly what it
decided, and `skillsdirtrust.go` is byte-identical. This bead is only about
whether anything would notice.
