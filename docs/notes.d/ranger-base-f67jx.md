# The wall's own prep is a project-config producer (ranger-base-f67jx)

*2026-11-11 · gwart · follows ranger-base-zmk4q (ADR 0090 D3) and
ranger-base-zvgql; decision bead ranger-base-mkrsv*

## The red, and why it is not what this bead closes

`TestPersonaLaunchRuntime` red in internal/posse's full-package run and
passed under `-run`. holden measured the cause out of ranger-base-riqwp: a
parallel test's real bwrap prep seeded a zero-byte `.grok/config.toml` into
the test binary's shared `$HOME`, and the test's grok recipe named no `dir:`,
so it launched there and the project-config classifier refused it.

That red is already fixed on main. ranger-base-zvgql (65436282) gave the
recipe its own session dir. MEASURED 2026-11-11, HEAD 43af67ac, linux
7.2.15, go1.27.3:

    go test -count=1 ./internal/posse     ok  157.644s

So nothing in this bead is about the test. What is left is the claim holden's
comment 4 named: seatbelt.go's `Seeded` says a seeded file is "exactly as
inert as none (seedBody)", and for `.grok/config.toml` it is not.

## What was measured

The launcher creates paths in the operator's home before it execs bwrap —
`PrepSeed` for file-shaped denies, `PrepMkdirAll` for directory ones
(`BwrapArgs`, `runBwrapPrep`). Some of those names are entries on a runtime's
`project_config:` list, and `ProjectConfigTrust` resolves those entries
against the SESSION DIR. For a session whose dir is the home — which is what
a recipe with no `dir:` gets — the two meet.

MEASURED 2026-11-11, linux 7.2.15, go1.27.3, by the pins below, in a home
where all three runtimes' prep has run. Ten (runtime, entry) rows collide:

| listed by | entry | made by | step | finding |
|---|---|---|---|---|
| claude | `.claude/settings.json` | claude | seed | *clean* |
| claude | `.claude/settings.local.json` | claude | seed | *clean* |
| codex | `.codex/config.toml` | codex | seed | project config is present |
| grok | `.claude/plugins` | claude | mkdir -p | unreadable: a directory, not a regular file |
| grok | `.claude/settings.json` | claude | seed | *clean* |
| grok | `.claude/settings.local.json` | claude | seed | *clean* |
| grok | `.grok/config.toml` | grok | seed | invalid JSON: unexpected end of JSON input |
| grok | `.grok/hooks` | grok | mkdir -p | unreadable: a directory, not a regular file |
| grok | `.grok/lsp.json` | grok | seed | project config is present |
| grok | `.grok/plugins` | grok | mkdir -p | unreadable: a directory, not a regular file |

Three classes, and only the first is inert:

- a `{}` seed under a KEYED list declares no key, so it reads clean. That is
  the case ranger-base-6akzw's claim was written for, and the only one where
  it holds.
- a `{}` seed under a WHOLE-FILE entry degrades on presence alone, so the
  body never gets a say (`.grok/lsp.json`, `ProjectConfigWholeFile`).
- a ZERO-BYTE seed is invalid JSON, and a directory the prep made is not a
  regular file. `seedBody` returns `""` for every name that is not `.json`,
  so every `config.toml` the wall seeds is in this class.

The verdict a launch in that home actually gets — the FIRST entry with
something to say, in declared order:

    claude   clean
    codex    project config is present
    grok     unreadable: a directory, not a regular file

So the answers to what the bead asked to check alongside are both yes, and
one of them is wider than expected. **A real launch is refused**, not only a
test one: a recipe with no `dir:` launches in `$HOME`. **codex has the same
exposure**, through the whole-file predicate rather than the JSON parse.
And it is **cross-runtime**: what refuses the grok launch is `.claude/plugins`,
a directory a CLAUDE seat's prep made, which wins the declared order before
`.grok/config.toml` is reached.

ranger-base-zvgql said of this "that is the old verdict for any home whose
`config.toml` exists, so it is not filed". True for a home that already had
one. A home that had none launched clean before ranger-base-zmk4q and does
not now.

## The remedy the refusal names does not work

`projectConfigTrustMessage` tells the operator to "remove the file (or, for
keyed JSON, the matching keys)". MEASURED: remove `~/.grok/config.toml`,
run the next launch's prep, the file is back and so is the finding. On a box
that keeps launching caged seats there is no operator action that clears it
short of `trust_project_config: true` or `--allow-degraded`, and both are the
wrong instrument for a file posse made.

## What changed here

Nothing in behaviour. Two comments that asserted the claim now say where it
holds and where it does not (`SeatbeltCarveOut.Seeded` in seatbelt.go, the
Seeded arm in bwrap.go), the classifier's doc says posse is one of the
producers it reads (`ProjectConfigTrust` in parity.go), and the census and
verdicts above are pinned so neither half can move without the other seeing
it.

`internal/posse/wallseedprojectconfig_qa_test.go`, `-run WallSeedProjectConfig`,
arm 1, 0.3s, no suite slot:

- the census: every (runtime, entry) the prep creates, with the step that
  makes it and the classifier's finding, against the recorded table. A new
  wall member or a new `project_config:` entry that collides reds and names
  itself; a row that disappears reds too and says the decision bead goes with
  it. Control: the same home classifies clean for all three runtimes before
  the prep runs.
- the verdict: what each runtime's launch in that home is told, plus the
  remove-and-relaunch arm above.

Negative control, both pins: with `.grok/config.toml`'s finding recorded as
clean and codex's verdict recorded as clean, both red and print the real
values.

bwrap-only, and it skips on darwin with the reason: the seed is the bwrap
mechanism's, because an SBPL subpath deny needs no name to exist.

## Not closed here

- ranger-base-mkrsv
