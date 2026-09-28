## A fixture CLI is named for nothing real (ranger-base-mis0i)

The rule, in one line: **a test whose subject is "a runtime that is not
installed here" spells that runtime's exe `posse-<bead>-no-such-exe`.**

### What happened

On 2026-09-28 14:31 an `npm install` of an unrelated tool called **bobshell
2.0.5** put `/opt/homebrew/bin/bob` on this box — a symlink to
`../lib/node_modules/bobshell/dist/bob.js`. Five arm3 tests in
`internal/posse` went red at a HEAD nobody had touched:

```
TestProbeGapIsNamedAndNonBlocking
TestAssumedProbeIsNotWaivableAtTierFast
TestRuntimeCheckPrintsTheProbeRowBothWays
TestTemplateBashDenyIsAssumedUntilProbed
TestProbeSessionExeRefusesWhatItCannotName/resolves_nothing
```

`bob` had been the tree's placeholder for "a CLI the harness has never seen"
since ADR 0017. A second seat isolated the cause to exactly one variable —
same tree, same command, PATH with `/opt/homebrew/bin` FAIL (1.9s), PATH
without it ok (1.1s).

### Why it is a class and not an incident

The fixture's premise is not a property of the fixture. Three production
paths resolve a runtime's exe on the **box's** PATH, none of which a
`t.Parallel()` test can redirect:

- `ProbeState`'s drift check (`internal/posse/runtimeprobe.go`) —
  `resolve(rt.Exe())` compared against the record's `launcher_cli_path`;
- `RuntimeGaps`' `exe` row (`runtimepreflight.go`), an `exec.LookPath`;
- `probeSessionExe`, which types `command -v <exe>` into a real subprocess
  that inherits the ambient PATH.

So "this runtime does not exist here" is a **bet that the name stays free on
every box the suite ever runs on**. alice/bob/carol are exactly the names a
package manager ships.

**Why not "give the test a PATH it controls" instead** — the other remedy the
bead names. `PathOutsideGates` reads `os.Getenv("PATH")`, so controlling it
means `t.Setenv`, which panics beside `t.Parallel()`; all five tests are
parallel. `resolveOutside` was deliberately made to read the environment
without swapping it, precisely so these tests could go parallel
(ranger-base-i7fa), and `probePaths` memoizes per process, so a swap would
leak across tests. Redirecting instead means a new seam through `CheckParity`
/ `RuntimeCheck` / `RuntimeGaps` — a test seam on the launch path — to buy
what one word at the fixture buys for nothing.

### What the collision costs when it is not pinned

None of the five failures named a collision. They said:

```
the probe measured /usr/local/bin/bob and bob now resolves to
/opt/homebrew/bin/bob — a different binary was measured; re-run
`posse runtime probe bob`
```

which reads as a defect in the probe's own drift logic — and the fifth, a
test asserting that a probe which *cannot name its binary must refuse*, read
as posse failing to refuse. Two seats reproduced it before the cause turned
out to be a package manager.

### The convention, and why it needed a pin

The spelling already existed in the tree — `posse-tr8k-no-such-exe` and
`posse-8vys9-no-such-exe`, both in `runtimepreflight_test.go` — beside two
other spellings of the same intention: `definitely-not-installed-anywhere-385x`,
and the alice/bob/carol placeholder line, which is the one that lost. A
convention that three files keep and nothing checks is not a convention.

`internal/treepins/fixtureexe_qa_test.go` is the check. It walks the
repository for every `*_test.go` (walked rather than listed, so a package
that grows runtime fixtures tomorrow is scanned without anyone remembering)
and parses each with `parser.ParseFile` — which applies no build
constraints, so the untagged, `posse_arm2` and `posse_arm3` files are all
read in one run. It collects the first word of every `command:` template
line and every `Command:` struct field, resolving package-level string
constants so that a fixture which already obeys the rule is read correctly,
and fails if any of those words resolves on **this** box.

It is deliberately box-dependent. The defect is box-dependent; a pin that
were not could only check a naming habit. This one reds on the day the
collision arrives, wherever it arrives, in a package that runs in seconds,
and names the exe, the file:line, the binary it collided with, and the
remedy.

MEASURED 2026-09-28, darwin arm64, go1.26.5, on the tree this bead lands
(82773c2e plus this change): the scan collects **55** fixture exes. Six are exempt in `fxeReal`, each with its reason — the four
built-in runtimes (`claude`, `codex`, `grok`, `bob`), the two cage-template
engines (`env`, `echo`), and `sh` (`cmd/posse`'s `shell` fixture, whose
subject is routing and flag parsing and which refuses on a missing herdr long
before anything resolves the exe).

### Verification

Two-way, with a real `carol` placed on PATH:

| tree | PATH | result |
|---|---|---|
| at 82773c2e | + fake `carol` | the same five FAIL, same messages as the bead |
| with this change | + fake `carol` | ok 1.05s |
| with this change | + fake `carol` + fake `mycli` | the pin FAILs, naming both and their file:line |

Note the last row: `carol` is still the command word at four *hermetic* sites
in `runtimeprobe_test.go` (they inject `resolve`, so no PATH lookup can reach
them) and in `promoteruntimes_qa_test.go`. Those were left alone deliberately
— renaming them buys nothing today — and the pin covers them anyway, which is
the point of pinning the property rather than the spelling.
