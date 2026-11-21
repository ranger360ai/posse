# A derivation swap trades one escape class for two (ranger-base-a7y67)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, go1.27.3, 16 cores, from this
bead's worktree at `acdaf81f`. The verify lane over four closes —
ranger-base-3weac, ranger-base-fugew, ranger-base-cso5z, ranger-base-xtg6z.
All four verified; two carry a live finding, filed as one bundle. This
fragment keeps the two measurements that generalize.

## A derivation keyed on one thing, re-keyed on another, is two escape classes

`internal/posse/seedkeycensus_test.go`'s part 1 derives the flat-YAML readers
out of `internal/posse/yamlflat.go`, and that set is what gives the whole
config-key census its reach: a function not in it has no call sites for part 2
to walk, so every key read through it is invisible to all three seed pins and
`make seed-check` is green and silent.

ranger-base-3weac re-keyed the rule from the first parameter being NAMED
`path` to the body — a reader is a function that hands its own first
parameter, as an Ident, to arg 0 of `os.ReadFile` or of another function in
that file already derived as opening. That closed the escape it was filed on.
It also opened two.

Each arm below appends one reader to `yamlflat.go` plus a non-test
`.ConfigPath` caller of it reading an undocumented key, runs
`make seed-check`, restores from git, and asserts `yamlflat.go` back
byte-identical (sha256 `c30d6b95…`) with the tree clean. The second table runs
each arm twice, the second time with `seedkeycensus_test.go` restored to its
pre-fix version (`git show 51dc5a6f^:…`) and nothing else changed.

| arm | the reader's shape | at `acdaf81f` |
|---|---|---|
| A1 | `(path, key string)` → `readLines(path)` | rc=2, key named (control) |
| A2 | `(file, key string)` → `readLines(file)` | rc=2, key named — the escape the close fixed |
| A3 | `(file, key string)` → an opener spelled `os.Open` | **rc=0, key named nowhere** |
| A4 | `(file, key string)` → `readLines(p)`, `p := file` | **rc=0, key named nowhere** |

| arm | the same two shapes spelled `path` | pre-fix rule | at `acdaf81f` |
|---|---|---|---|
| A3p | `(path, key)` + `os.Open` | rc=2, key named | **rc=0, named nowhere** |
| A4p | `(path, key)` + a local alias | rc=2, key named | **rc=0, named nowhere** |

So the close's "reach did not move" is right about today's tree — the same six
readers and the same 112 sites over 79 distinct keys, re-confirmed by a green
baseline here — and the head comment's claim about tomorrow's seventh ("in the
census the day it is written, whatever its parameters are called") is false in
BOTH directions. The name rule admitted A3p and A4p and refused A2; the body
rule does the opposite.

**The rule for the next derivation swap.** A key change is not a widening
unless both key sets are measured. Write the arm that the OLD key accepted and
the new one does not, as well as the escape the swap was filed on; a pre-fix
run of the same battery is one `git show <fix>^:<path>` away and it is the only
thing that distinguishes a widening from a trade. `skcWitness`'s floor
(`len(c.flat) < 6`) can see neither: every one of these escapes is a SEVENTH
reader, so the count never moves.

The fix seam is already in the file. `TestConfigKeyCensusCanStillSayNo` plants
its own `yamlflat.go` and asserts the derived set EXACTLY against `wantFlat`,
with `YamlGetAlt(file, key string)` in it as A2's fixture; A3's and A4's shapes
go in the same plant beside it.

## A refused shape is verified by running the shipped verb, not the function

ranger-base-cso5z reported that check 4's vuln-private content scan lost the
file boundary at `-U0` and refused a correctly written "Not closed here"
fragment. Its close found the defect already fixed in the tree
(ranger-base-4og5l `bfc068fa`, corrected by ranger-base-pdcf6 `eb3f34fc`,
both ancestors of this tree), landed the five rows the bead asked for, and
graded them through `RunGatesVulnPrivateScan`.

Graded again here one level out — through `posse gates vuln-private-scan` on a
binary built from this tree, fed the diff stream the hook's own pipeline
builds, with an open `vuln-private` id so the judge has something to find.
Eight readings, every one as the close said:

| shape | with boundaries | without (the pre-fix stream) |
|---|---|---|
| the bead's repro: bare id last in its file, a Go import line next | clean | **hit** |
| the same escape read upwards | clean | **hit** |
| a bare id alone | clean | clean |
| a described id in ONE file (control) | hit | — |
| a `diff --git`-shaped content line between id and description | hit | — |
| a `+++ b/`-shaped content line in the same place | hit | — |
| a file header with no `+++` line does not eat the next file | hit | — |

The close's structural argument for there being no false NEGATIVE holds, and
holds more strongly than it was stated: the id set is computed over the whole
text and only the GROUPING changes, so a boundary can only narrow a unit,
bareness is "contains a letter after the ids are stripped", and that predicate
is monotone under subset. A unit that was bare when wider is bare when
narrower. No measurement can find what the predicate forbids.

**And the skew it describes was still open seven hours later.** The binary at
`~/.local/bin/posse` is `0.5.3+00601dfe`, built 06:26, two hours before the
fix landed; the hook in the shared checkout was re-rendered at 16:07 and still
carries the pre-fix `grep -av '^+++'` five times over, because a render comes
from that binary and not from the tree. So the blank-line workaround is still
the way through at every seat, and the close was right not to re-render the
shared checkout's hooks under live seats. A tree-built binary renders the
fixed pipeline.

## Holding a CALL rather than the function it calls

ranger-base-xtg6z closed the gap at the call by driving the verb to its
report rather than by weakening the subject to a census. Graded at seven
mutants over `internal/posse/upgrade.go`, each alone, the file restored
byte-identical after every arm, under the filter its own header records (108
top-level tests here, 4.1s clean):

| mutant | result |
|---|---|
| the `upgradeLoopReaders(&plan, a)` call removed | rc=1, the verb pin and only it |
| the readers installed on a COPY of the plan the verb discards | rc=1, the verb pin and only it |
| `plan.Loop = nil` on the line after the call | rc=1, the verb pin and only it |

The second and third are the shapes the pin's own header claims a census
could not have held — "a different sentence the moment anything between the
call and `upgradeRunPlan` rebuilds or copies the plan" — and they are the
reason the stronger subject was worth the fixture. A census of the symbol
survives both.

## Not closed here

- ranger-base-m4w7d

