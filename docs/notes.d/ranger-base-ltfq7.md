# The HOOK_DEPS posse exemption, pinned by containment instead of by a count of one shape (ranger-base-ltfq7)

`scripts/cleanroom.sh`'s `HOOK_DEPS` is the list the clean-room probe walks
inside a distro image, and `posse` is deliberately absent from it: no distro
ships this repo's own binary, so naming it there would report every distro
`MISSING` forever. The exemption is sound only while every call to the binary
in a rendered hook fails open on its own — each one behind a `command -v posse`
test — and `internal/posse/hookdeps_qa_test.go` is where that premise is
measured rather than asserted (ranger-base-zby5f).

It was measured for exactly one call shape.

## What was wrong, in both directions at once

The premise block counted `strings.Count(body, "| posse ")` against
`strings.Count(body, "command -v posse >/dev/null 2>&1")` and demanded the two
be equal.

**Silent on the thing the exemption's own comment says the block exists to
prevent.** MEASURED 2026-11-12 (holden, this box, linux 7.2.15 x86_64,
go1.27.3, tree 9aaef0d4, `go test -overlay`, tracked tree untouched): one line
appended to `vulnPrivateChecks` AFTER its `fi`, outside the guard, rendering as

```sh
posse_vp_extra=$(posse gates vuln-private-scan </dev/null 2>/dev/null)
```

passed `internal/posse` WHOLE in all three arms — arm 1 ok 168.9s, arm 2 ok
54.1s, arm 3 ok 85.5s, 0 FAIL. `runs` and `guards` both stayed at 2, because
the new call is not in the counted shape, and `posse` is exempt from
`hookCensus` BY NAME so the census could not see it either. The pin's message
did tell a reader to "count that shape here too" — but only on an UNEQUAL
count, and a shape the counter cannot see at all gives 0 behind 0, so nobody
was told anything.

**Red on safe code.** Two counted calls under ONE guard — guarded, by
inspection — read `4 run(s) ... behind 2` (4 and not 3 because
`vulnPrivateChecks` renders twice, at check 4's content arm and its message
arm).

A count cannot fix the second direction, whatever it counts: two runs under one
guard are two runs and one guard. The property is CONTAINMENT.

## The fix

The scanner already saw the call. With the pipeline dropped, `hookCensus`
reported the command at `prepare-commit-msg:366` by name — so the measurement
belongs to `shellCommandWords`' command positions, the thing the exemption
suppresses, and not to a hand-spelled substring of today's render.

`shellCommandWords` is now the deduping face of a new `shellCommandSites`,
which is the scanner itself and returns EVERY command position with two
lexical facts a name alone cannot carry:

- **`Prefix`** — the words walked past while the command position stayed open.
  `command -v posse` arrives as Prefix `["command", "-v"]`, Name `posse`, which
  is how a PROBE of the binary is told from a RUN of it. A substring cannot:
  `command -v` leaves the name in a command position, so the name alone says
  only that the binary is mentioned.
- **`Guards`** — the names tested by every `command -v <name>` guard whose
  BODY the site sits inside. An `ifFrame` stack carries it: `then` promotes the
  condition's probes to the body's guards, `else` drops them (that branch is
  what runs when the probe FAILED), `elif` starts the frame over, `fi` pops.

The premise block then reads: every `posse` command position is a probe, or a
run with `posse` among its guards. No list of call shapes to maintain.

## Mutation-checked

Seven mutants of `vulnPrivateChecks`, each through `go test -overlay` against
the focused pin, tracked tree never modified (MEASURED 2026-11-12, this box,
same environment as above):

| mutant | want | got |
|---|---|---|
| the unguarded line appended after `fi` (the escape above) | red | red, at `prepare-commit-msg:369` and `:412` |
| the guard replaced by `if true` | red | red, and the vacuity arm fires too |
| the run moved into the guard's `else` branch | red | red |
| the guard inverted to `if ! command -v posse` | red | red |
| two runs under one guard | green | green |
| the judge reached without its pipeline, still guarded | green | green |
| baseline | green | green |

The fourth row is the one false GREEN the first cut of this had, and it is why
`!` joins the prefix: an inverted probe is a probe whose BODY runs when the
binary is absent, so it must establish no guard. It was caught by writing the
mechanism's rows, not by the live render, which has no inversion in it — the
`else` and `elif` arms were written from the same reading.

The fifth row is the previous version's second mutant, and it is green here on
purpose: the premise is that the call fails open, not that it is written as a
pipeline. A guarded call without a pipeline is safe code, and the old block red
on it for the wrong reason — it detected "a counted call disappeared", never
"an unguarded call appeared".

`TestShellCommandSitesTellsAProbeFromARunAndNamesItsGuards` holds the
mechanism's own rows, fourteen of them, including a nested `if` under the
guard, an `elif` as a fresh frame, an inverted probe, and another binary's
guard covering nothing.

## Not vacuous, and still red on two shapes

A render that calls the binary nowhere would satisfy every assertion and leave
the exemption covering nothing — the block would go quiet exactly when it
stopped being needed. So runs and probes are each asserted non-zero across the
three hooks together, with the message naming the `notPathLookups` entry to
delete if no hook calls the binary any more.

Only an `if` BODY counts as containment. A guard written as an AND-list
(`command -v posse >/dev/null 2>&1 && posse gates ...`) or as a `while`/`until`
condition reads as no guard, so a safe hook written either way would red. No
hook is, both shapes have a row in the mechanism test so the residue is
measured rather than denied, and the instruction when one grows is to teach
`ifStack` that compound rather than to widen the premise. This file has twice
proved that an absolute in it is a claim the next reader stops checking
(ranger-base-h6k2r, ranger-base-xwepd); it gets a named residue instead.

## Not closed here

- ranger-base-wzo36
