# The same discard one layer up: a recipe's `command:` beside its `agent:` (ranger-base-g27p4)

*2026-11-22 · dinesh · the arm ranger-base-k2olp filed rather than took — why
the recipe refusal is its own message in its own place, why it is asked at
LaunchRecipe and deliberately not at LoadRecipe, and the one live recipe on
this box that has to change.*

## The claim that was not true

`internal/posse/recipes.go` reads `command:` and `agent:` independently, and
`LaunchRecipe` hands both on:

```go
err = b.CreateSession(NewSessionOpts{
    Name: r.Name, Dir: r.Dir, Cmd: r.Command,
    Emoji: r.Emoji, Envs: r.EnvFiles, Agent: r.Agent, …
})
```

`planLaunch` then does what ranger-base-k2olp's fragment describes: `cmd :=
o.Cmd`, and the `o.Agent != ""` branch overwrites it with
`RenderCommandForModel`. So the operator's declared `command:` never runs,
the session record does not keep it either, and nothing says a word. It is
the identical failure to `posse new --cmd --agent`, one layer up, and it
failed safe for the identical reason: what runs is the PID's rendered line
with every launch guarantee the render appends.

The rule was already written down — `examples/recipes/codex-projA.yaml`'s
`agent:` form says `command: null   # the runtime's template renders it` —
and enforced nowhere.

## Why it is a separate refusal and not `CheckCmdAgent` over the opts

The reader is editing a YAML file. `CheckCmdAgent`'s message names `--cmd`
and `--agent`, two flags this operator never typed, and prescribes "launch it
without `--agent`" — a flag that is not in front of them. So
`CheckRecipeCmdAgent` is a sibling in the same file, pure in the same way,
and its message names the two KEYS, the file to open, the line being thrown
away, and the three ways out (`command: null`, drop `agent:`, change the
PID). What it is over the built binary:

```
$ posse recipe grok-fam
posse: recipe ~/.config/posse/recipes/grok-fam.yaml declares command: beside agent: grok — a
persona's line is rendered from its PID and its runtime's template, so the command: you
declared (grok) would be discarded in silence (ADR 0002 §1)
  to launch the persona, write `command: null` — the runtime's template renders the line
  to run that line yourself, drop `agent:` — the session gets no PID, no gates and none of the rendered launch flags
  to change what the persona runs, change its PID's command: or its runtime file
```

With both boundaries now armed — `parseNewFlags` for the flags,
`LaunchRecipe` for the keys — no `NewSessionOpts` carrying both can reach
`planLaunch`, and `planLaunch` still asks neither. That is the whole of
`CheckCmdAgent`'s table from ranger-base-k2olp's fragment, closed.

## It reads the LOADED value, not the key

The one way this check could be wrong while looking right. `YamlGet` reads
`null`, `~` and an empty value as unset (`yamlflat.go`), so the three
spellings below are NOT the pair and all three launch exactly as before:

```yaml
command: null        # what the seed recipe prescribes
command: ~
                     # the key absent altogether
```

A check written over the key's PRESENCE would refuse the form this repo
documents — and the form `~/homes/ranger/recipes/codex-src.yaml` on this box
already carries. `TestRecipeWithNullCommandBesideAgentStillLaunches` exists
for that mutant and nothing else.

## Asked at LaunchRecipe, NOT at LoadRecipe

The decision in this bead worth recording, because the other way looks
tidier. `posse recipes` lists by loading every file and **dropping the ones
that error**:

```go
for _, n := range a.ListRecipes() {
    if r, err := a.LoadRecipe(n); err == nil {
        fmt.Fprintf(out, "%s %s  %s\n", r.Emoji, n, r.Purpose)
    }
}
```

So a refusal moved into `LoadRecipe` would make a recipe carrying the pair
vanish from the listing — a silent discard of exactly the class this bead
fixes, traded for the one it fixes. `LaunchRecipe` is also where the discard
actually happens, which is where the operator is standing when they are told.
`TestRecipeCarryingThePairStillLoadsAndLists` holds that trade: the file
still loads, both values read back, and `posse recipes` still prints the row
the operator has to go fix.

## The live break, which is the operator's call and not a seat's

MEASURED 2026-11-22, census of `~/homes/ranger/recipes/*.y*ml` on this box
(six recipes), by what each one's `command:` and `agent:` READ BACK through
`YamlGet` rather than by which keys are present:

| recipe | `command:` | `agent:` | after this lands |
|---|---|---|---|
| `grok-fam.yaml` | `grok` | `grok` | **refused** — needs `command: null` |
| `codex-src.yaml` | `null` → unset | `codex` | launches, unchanged |
| `claude-projA.yaml` | a line | — | launches, unchanged |
| `claude-weller.yaml` | a line | — | launches, unchanged |
| `scratch.yaml` | a line | — | launches, unchanged |
| `web-dev.yaml` | a line | — | launches, unchanged |

One recipe, one line. `codex-src.yaml` is the interesting row: it declares
`command: null` beside `agent: codex` and is already in the prescribed form,
which is the positive control for the loaded-value reading above. The seat
did not edit either file — `~/homes/ranger/recipes/` is the operator's live
config, not this repo — so `posse recipe grok-fam` refuses until he writes
`command: null` there, and the refusal names the file and the fix so he need
not come back to this fragment for it.

## Grading the pins

`internal/posse/recipecmdagent_qa_test.go`, over `LaunchRecipe` rather than
over the built binary — which is where this diverges from
`cmd/posse/cmdagent_qa_test.go`'s twin. That refusal's only path out is
`die`, so "exit 1, nothing created" had to be a claim about a process. This
one RETURNS its error, and `LaunchRecipe` is the whole of `posse recipe` (the
one entry point whose opts are built inside the package), so the boundary and
the test sit in the same place and the fixture reads the host back directly.

Six mutants, MEASURED 2026-11-22 (this box, linux 7.2.15 x86_64, go1.27.3),
each applied alone and each red at an ASSERTION rather than at build or vet:

| mutant | verdict |
|---|---|
| reads `yamlHasKey(r.Path, "command")` instead of the loaded value | RED — the `null`/`~`/absent row |
| refusal moved into `LoadRecipe` | RED — the listing row |
| refusal downgraded to a line on the launcher's writer | RED — the error row and the writer row |
| asked after `CreateSession` instead of before | RED — the session exists |
| message drops the discarded line and the file path | RED — the message rows |
| message drops `command: null`, the fix | RED — the message rows |

The third is ranger-base-k2olp's own graded mutant one layer up, and it is
cheaper to catch here than it was there: `LaunchRecipe` returns an error, so
`err == nil` is a direct read and needed no `no such agent` proxy. The fourth
is the one the flag boundary could not have: `posse new`'s refusal is at
parse time and its pin walks the home for leftover state, while here
`b.HasSession(r.Name)` answers the same question outright.

The operator-visible half is `cmd/posse/recipecmdagent_qa_test.go`, for
`TestCmdHelpNamesTheAgentRefusal`'s reason: the refusal reaches the operator
only after they wrote the recipe and ran it, and the catalog reaches them
before. It goes through `helpcatalog_test.go`'s own `commandOf` rather than
a substring scan, because the catalog's continuation lines carry no key of
their own (rangerhq-6izv) — and that choice is what its third mutant pays
for. Three mutants, same box and date, each red at an assertion:

| mutant | verdict |
|---|---|
| the entry loses the rule (the line as it read before) | RED — the catalog has no `command: null` |
| the rule becomes a prohibition with no reason and no fix | RED — same |
| the rule drifts under `posse relaunch` | RED — and it NAMES the command it drifted to |

The ordering half still needs its own row — the pair must be refused before
posse resolves the persona at all, or the reader is told about a missing
persona instead of their two keys — so one row launches a pair naming a
persona that is not installed and asserts the error is not `no such agent`.
