# A derivation's seed is a convention too, and a value cap is not a line cap (ranger-base-m4w7d)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, go1.27.3, 16 cores, from this
bead's worktree at `2779f80f`. The two live findings ranger-base-a7y67's
verify lane filed, fixed: one in the config-key census's flat-reader
derivation, one in the box-wide loop census's rendering.

## 1. Re-keying a derivation from a name to a body keeps a convention; it moves it

`internal/posse/seedkeycensus_test.go`'s part 1 derives the flat-YAML readers
out of `internal/posse/yamlflat.go`, and that set is the whole census's reach:
a function not in it has no call sites for part 2 to walk, so every key read
through it is named NOWHERE and `make seed-check` is green and silent.

ranger-base-3weac re-keyed the rule off the first parameter being NAMED `path`
and onto the body — a reader hands its own first parameter, as an Ident, to
arg 0 of `os.ReadFile` or of another function in that file already derived as
opening. ranger-base-a7y67 measured what that traded: the new rule's seed was
one stdlib call and its argument test was a bare Ident, so a reader whose
opener is `os.Open` and a reader that writes `p := file` first were each
derived as nothing — and both shapes, spelled with `path`, had been in the
census under the rule it replaced.

Both halves are conventions. A seed has to be spelled somewhere; what is
fixable is how much ordinary Go walks past it. So the seed is now the three
stdlib opens that turn a path at argument 0 into the file's bytes
(`os.ReadFile`, `os.Open`, `os.OpenFile` — MEASURED over the tracked non-test
Go: 109, 30 and 39 call sites, which is every way this tree opens a file;
`os.Create` is excluded, because a function reaching it writes its first
parameter), and arg 0 is matched through local aliases and through wrapping —
an alias set seeded with the parameter, closed over assignments whose
right-hand side mentions a member, and an arg 0 that MENTIONS any member.
Mentioning rather than being, because `readLines(filepath.Clean(file))` is the
same reader and a rule reading only a bare Ident is one assignment from
blindness.

REGRESSION, each arm appending one reader to `yamlflat.go` plus a non-test
`.ConfigPath` caller of it reading an undocumented key, running
`make seed-check`, restoring `yamlflat.go` from a copy and asserting it back
byte-identical (sha256 `c30d6b95…`). Run twice: once with this fix, once with
`seedkeycensus_test.go` restored to `git HEAD`'s pre-fix version and nothing
else changed (`scripts` not needed; the battery is this bead's
`skc-probe.sh`).

| arm | the reader's shape | pre-fix | with this fix |
|---|---|---|---|
| plain | `(file, key)` -> `readLines(file)` | rc=2, key named | rc=2, key named |
| open | `(file, key)` -> `os.Open(file)` | rc=0, NAMED NOWHERE | rc=2, key named |
| alias | `(file, key)` -> `readLines(p)`, `p := file` | rc=0, NAMED NOWHERE | rc=2, key named |

REACH DID NOT MOVE over the real tree: 230 non-test `.go` files, 6 flat
readers, 5 config readers, 112 sites over 79 distinct keys, 4 dynamic sites —
the same numbers the close being verified reported.

The residue is named where the seed is spelled (`skcIsFileOpener`): an open
this set does not name, or a reader reaching the file through a helper in some
other file of the package, is still derived as nothing. The fixpoint's
transitive half closes over `yamlflat.go`'s own declarations and no further.

## 2. A cap on one value is not a cap on the line the value is rendered into

`internal/posse/boxloops.go`'s `boxHomeMaxLen` (256) is justified by a line a
reader can still read: the box census's rendering is ONE line an install
delegate pastes onto a bead, and a peer that can make it four thousand bytes
long has pushed the reader's own sentence — the scope clause, the counts — off
the end of it.

The cap bounds the VALUE. Nothing bounded the LIST: `boxProcList` joins one
row per process, so four thousand bytes cost a peer sixteen forks instead of
one long value, with the cap in place and every value it admits well-formed.
MEASURED through the package's fixture seam, N peers each declaring a distinct
256-byte home the fence accepts: 1 peer 612 bytes, 2 peers 882, 16 peers 4649,
64 peers 17561 (ranger-base-a7y67's series; the last re-measured here).

Now bounded by `boxRowsMaxBytes` (512 per list), whole rows, with a clause
saying how many rows were dropped — and both per-process lists go through the
one join, because the unattributed list is reachable more cheaply than the
loop list: a row there needs no record naming the pid back, only a fork.

| reading | before | after |
|---|---|---|
| 64 peers, all confirmed loops | 17561 bytes | 748 |
| 64 peers, all unattributed | 6789 bytes | 1082 |
| 64 of each, both clauses full | — | 1435, the line's worst case |
| this box, live (2 loops, 2 others) | 754 bytes | 754, nothing dropped |

512 is three times the longest list this box renders (the loop list 78 bytes
over two rows, the unattributed list 161 over two), so no reading this box
produces is truncated. The count an operator acts on is rendered AHEAD of
each list by `boxCount`, so truncation costs which rows and never how many —
which is what the new pin asserts alongside the bound.

## Mutants, each applied alone and graded against the fix's own filter

| mutant | graded | result |
|---|---|---|
| seed back to `os.ReadFile` alone | `^TestConfigKeyCensus\|^TestEveryConfigKey\|^TestSeed` | red, `TestConfigKeyCensusCanStillSayNo` |
| alias resolution disabled | same | red, same pin |
| `boxRowsMaxBytes` = 1 GiB | `^TestQABox\|^TestLiveBox\|^TestQATheVerbsBoxLine` | red, the new pin, at 17561 bytes |
| only `boxProcList` bounded | same | red, the new pin, at 6789 bytes |
| truncation clause says nothing | same | red, the new pin |

The battery is `mutants.sh` in this bead's scratchpad: one mutant at a time
over a restored tree, refusing to grade a mutant whose text did not apply
exactly once — a mutant that did not apply grades green and lies.
