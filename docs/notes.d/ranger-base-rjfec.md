## A rule stated for every verb and realized in one helper (ranger-base-rjfec)

Discovered from ranger-base-cse63 on 2026-10-09. `posse help`'s preamble said
"A subcommand that takes a `<name>` prints its own usage for -h/--help, and
reads a literal `--` as the end of flags". That rule was realized by
`need()`/`argLead` in `cmd/posse/main.go`, so it reached only the verbs that
call them. Everything else read `-h`/`--help` as whatever its own flag loop
made of it.

Written 2026-10-09.

### The census, which the filing bead asked for first

The bead probed eight verbs and found two that kept the rule. The whole
surface, read from `main`'s `switch cmd` rather than sampled:

MEASURED 2026-10-09, worktree `posse/gwart-posse-ranger-base-rjfec`, at
d92b4b52:

| | count |
|---|---|
| `case` clauses in `main`'s verb switch | 38 |
| verb spellings those clauses accept | 47 |
| spellings that are `help`/`version` and their flag forms | 5 |
| of the remaining 42: routed through `need()`/`argLead` | 25 |
| **never reached the rule at all** | **17** |

The seventeen: `agents` `backup` `cage` `cockpit` `cost` `dispatch` `envs`
`init` `list` `ls` `ready` `recipes` `runtime` `runtimes` `scorecard`
`skills` `worktrees`. The bead's sample of eight drew six of them, which is
why the sample read worse than the surface (6/8 against 17/42) and still
understated the kinds of failure: three of the seventeen DO WORK and report
(`cage`, `recipes`, `cockpit`), one scans bd for every persona (`scorecard`),
one reaches for herdr (`list`), and the rest print a usage and exit 1.

Sub-verbs were worse than any of them, and nothing in the bead counted them:
`posse agent new --help` scaffolded a persona called `--help` and opened it
in `$EDITOR`, which is rangerhq-qv5's original defect surviving one level
down from the fix.

### Why one shared reading and not seventeen fixes

Seventeen per-verb fixes are seventeen usage strings to keep true, beside a
preamble that was already false in sixteen places. The catalog `posse help`
prints is the only text in the binary that documents every verb's grammar,
and rangerhq-6izv already pins its shape. So the catalog is the source:

- `catalogLead` reads a header's subcommand path — the literal words of its
  grammar, up to the first that is grammar rather than a word an operator
  types. `  posse backup status` leads with `[backup status]`;
  `  posse backup [--to <dir>]` leads with `[backup]`.
- `catalogBlock(path)` is every entry whose lead begins with `path`, plus the
  continuation lines under each. A parent verb answers with all of its
  sub-verbs' entries, a sub-verb with its own.
- `catalogHelp(argv)` walks the longest path `argv` names literally and then
  asks whether the NEXT argument is `-h`/`--help`. `main` calls it before the
  switch, so the reading lands ahead of every verb's flag loop.

There is no second usage string, so nothing can go stale beside the catalog —
`TestEveryVerbAnswersForHelp` checks each verb's answer is a verbatim slice of
what `posse help` prints.

### Three things the shape had to get right

**Walk the path, never scan argv.** A scan for `-h`/`--help` anywhere would
read `posse pause "the box is on fire --help"` and `posse prompt sess --help`
as help requests and swallow an operator's text. Reading only the argument
that follows a literal subcommand path leaves free text alone, and keeps
`argLead`'s `--` escape intact: `posse kill -- --help` still kills a session
called `--help`.

**Stop a block at a `config <key>:` line.** The catalog puts a verb's config
keys between command entries at a shallower indent. Without that boundary
`posse backup --help` printed `verify_box_accepted:` — another verb's key, or
rather no verb's. The keys that ARE backup's belong in its help, and
ranger-base-cse63 is the bead that decides how; this one leaves them out
rather than printing the wrong set.

**Keep `need()`'s help arm.** It is unreachable now for every verb the
catalog names, and it stays as the backstop: the catalog is prose, and a verb
added tomorrow without an entry would otherwise take `--help` as its `<name>`
again. The census pin is what makes that case loud rather than silent.

### What the aliases cost

The catalog documents an alias in prose — `posse attach <name>  focus its
workspace in herdr (alias: focus)` — rather than giving it a header, so four
spellings `main` accepts have no entry of their own: `ls`, `local`, `focus`,
`orders`. `catalogAlias` is the one hand-kept table in the mechanism, and two
pins hold it from both ends: the census makes it total, and
`TestCatalogAliasNamesLiveVerbsAndLiveEntries` reds on an entry whose verb the
switch dropped, whose target has no catalog entry, or that the catalog has
since documented itself.

A side effect worth naming: `posse up --help` and `posse local --help` used to
print `usage: posse attach <name>`, because `up`/`local`/`focus` share
`attach`'s `need()` call. They print the `posse up` entry now — create-or-focus
— which is what they do.

### Why the preamble says "every sub-verb this catalog names"

Because three sub-verbs the switch accepts are not catalog PATHS, and the
sentence has to be true. `posse gates wrap <persona> -- <cmd>` is in
`need()`'s usage string and has no catalog entry at all; `posse skills [list]`
brackets its only sub-verb; `posse env edit|rm <name>` is one header for two,
written with a pipe. So `posse gates wrap --help` and `posse env edit --help`
exit 1, and `posse skills list --help` does the listing — filed as
ranger-base-1jmtm with the measurements, because each has a different cause
and widening `posse help` by three header lines is a decision about the
catalog's shape rather than about this reading.

The scope of the claim is what the pin measures: `catalogSubVerbs` enumerates
the catalog's own sub-verb paths and asserts each answers for itself. A
sentence scoped to "every sub-verb" would have been the same defect this bead
was filed over — a rule stated more widely than anything realizes it.
