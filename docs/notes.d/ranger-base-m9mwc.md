# A default that lived in shell, and the two copies of it nothing compared (ranger-base-m9mwc)

2026-10-05. `examples/config.yaml` documents a launch-attempt cap beside
`autostart_max_beads`, and says in its own paragraph that the value on that
line IS what applies when the key is absent. That value lives in SHELL:
`plugin/autostart.sh` holds it twice, in the absent-key arm of a `case` and in
the sentence the not-a-count arm prints. There is no Go constant anywhere, so
the seed's documented-default census (ranger-base-vofbl, widened by
ranger-base-p9qve) cannot reach it — that census is an AST walk over Go
sources, and a shell default is outside it by construction.

`internal/posse/autostart_test.go` already pinned the script's BEHAVIOUR
(`TestAutostartMaxBeadsAlwaysPresent`, over absent, empty, word and negative
values), so editing the script reds the suite. What nothing held was the SEED
LINE against the script: make that pin green again by editing its `wantN` with
the script, and a fresh instance's spec names the old number. One language
over, that is the stale-for-free shape ranger-base-vofbl and
ranger-base-ghcx3 were filed for.

The values themselves are not restated here. They are in the seed, in the
script, and compared by the pins below — which is the point (ADR 0024 D3,
restate-and-cite; this fragment was refused once by the ops-class markdown
gate for quoting two of them).

## The survey first (the bead asked for it)

`plugin/` is `autostart.sh` and `herdr-plugin.toml`; the manifest declares no
defaults. The script reads seven config keys, and the class "the seed
documents a literal that IS the fallback the script applies" has exactly
**two** members:

| key | where its fallback lives | in the class? |
|---|---|---|
| `autostart_max_beads` | the script, twice | **yes** |
| `autostart_session` | the script's `${...:-}` default | **yes** |
| `autostart_interval` | nowhere — the arm switch has none to fall back on | no |
| `autostart_max_interval` | posse's own 8x, in `cmd/posse/main.go` | no |
| `autostart_dry_run` | nowhere — the seed line is the opt-IN, absent is off | no |
| `autostart_resume` | the script, but the seed line is the opt-OUT | no |
| `autostart_dir` | the script's `$HOME`; the seed line is YAML's unset | no |

So a naive "compare every seed line to its shell fallback" census would be
WRONG on four of the seven: four of those lines are worked examples or
opt-outs on purpose, and `autostart_dir`'s fallback is a path no seed line may
name (`TestSeedConfigNamesNoMachine`). The register is not paperwork — it is
most of the file.

## Why not the other two shapes

The bead weighed three. **(b) have the script ask posse for the default** was
rejected: the hook already forks posse, so the fork is not the cost — the cost
is that `--watch-status`'s own comment is "THE LINE IS THE CONTRACT, not the
exit status", and a posse too old to answer a new default query would leave
the script needing a fallback anyway. It would add a failure path and remove
no copy. **(c) widen the census to a declared shell-defaults table the script
sources** is the general answer if more shell defaults arrive; at two members
it would be a new mechanism and a generated file to keep in sync, so it stays
unbuilt and the table above is the measurement that says when to build it.

## What shipped

Three pins in `internal/posse/autostart_test.go` (arm 3), derived on BOTH
sides so no third copy of the number exists:

- `TestAutostartShellFallbacksAreTheSeedsDocumentedValues` — the documented
  text is written into a config as a LIVE key and the argv the hook builds
  from it is compared to the argv it builds with the key ABSENT. Equal means
  the fallback the script applies IS the documented one. The seed side is
  `exampleConfigDefault`, the production reader of the EMBEDDED seed that ADR
  0024 D2 check 3 already decides with; the script side is the script, run.
  It parses no shell — a grep of the `case` arm would be a second parser,
  going narrower than the script while both looked green. Each row carries a
  CONTROL value that is not the default, because two runs that never read the
  key compare equal forever.
- `TestAutostartNamedAndReplacedArmQuotesTheSeedsDocumentedValue` — the second
  copy: a value the script cannot read must fall back to the same argv AND the
  sentence it prints must quote the documented number, because that sentence
  is what the deployer reads instead of the value they typed.
- `TestAutostartShellDefaultRegisterIsTotalAndNotStale` — the total-table
  rule. The key set is derived from the script's own `cfg`/`haskey` call sites
  (an enumeration of KEYS, which are unambiguously spelled, not of defaults),
  and every key is compared or registered, never both and never neither. Each
  register row's kind is CHECKED, not believed: a "documents no value" row
  reds if the seed starts documenting one, and a "documents the non-default"
  row reds if that value ever starts arming what an absent key arms.

MEASURED 2026-10-05, this box. All seven mutations detected, tree restored
between each: the script's absent-key fallback raised; the seed's documented
value raised; the warning SENTENCE alone raised while the value it applies
stayed; the session fallback renamed; a new `cfg` call site arriving with a
hard-coded default; the resume line flipped to the default it already has; the
dir line given a real path. The three pins cost 2.8s together.

Not a tree-wide pin, and so not given a Makefile door: that class is "a test
that WALKS or enumerates the tree"
(`internal/treepins/treewidedoor_qa_test.go` — "Reading ONE file at the repo
root is not either shape and is not fenced"). These read
`plugin/autostart.sh` by name and the seed through `go:embed`, so they red
when their own two subjects change, which is what they are for.
`TestQAEveryTreeWidePinHasADoor` was run and agrees.

ranger-base-p9qve's register row for `autostart_max_beads` **stays**: the key
is still outside the Go-constant census, held here instead. The bead's
alternative — source the script's default from a Go constant, which would let
that row be dropped — was not taken, for the (b) reason above.
