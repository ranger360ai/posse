# The flat-reader derivation was keyed on a parameter's NAME (ranger-base-3weac)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, go1.27.3, 16 cores, from this
bead's worktree. Row 1 of two findings laurie raised in ranger-base-msir3,
verifying ranger-base-w9t63's close. Row 2 is upgrade.go's box line, filed
separately to the devops lane.

## What was wrong

`internal/posse/seedkeycensus_test.go`'s part 1 derives the flat-YAML readers
out of `internal/posse/yamlflat.go`, and that is what gives the whole config-key
census its reach: a function not in that set has no call sites to walk, so
every key read through it is invisible to all three seed pins. The rule was

```go
if len(names) >= 2 && names[0] == "path" && skcIsString(types[0]) && skcIsString(types[1]) {
```

while the head comment said the discriminator was the TYPE shape — "A reader
there takes the FILE first and the key second, and the two `(lines []string,
key string)` helpers beside them do not" — and promised "A seventh added
tomorrow is in the census the day it is written". Both were false for a reader
whose first parameter is called anything else, and no rule, pin or convention
in this tree requires the name `path`. `skcWitness`'s floor (`len(c.flat) < 6`)
could not see it either: the escaping reader is a seventh, so the count stays 6.

## MEASURED

Each arm appended one reader to `yamlflat.go` plus a non-test `.ConfigPath`
caller of it reading an undocumented key, ran `make seed-check`, restored, and
asserted `yamlflat.go` back byte-identical (sha256 `c30d6b95…`) with the tree
at its baseline. Battery: `mutant.sh`, in this bead's scratchpad.

| arm | the reader's first parameter | before this bead | after |
|---|---|---|---|
| A1 (control) | `path` | rc=2, the key named in the failure | rc=2 |
| A2 (the escape) | `file` | **rc=0, the key named nowhere** | rc=2, the key named in the failure |

A2 was also green and silent under `make pkgcensus-check`, `make doc-check`,
`make fmt-check` and `go vet ./internal/posse`, and no other `*_test.go` in the
tree derives `yamlflat.go`'s reader set — grepping the filename, this census is
the only deriver. So the escape was the whole of what stood there.

**The name test was load-bearing for EXCLUSION, which is why deleting it alone
was not the fix.** `yamlflat.go:94` is `func flatFieldRefusal(key, v string)
string`: two strings, no file argument, so without the name test it reads as a
reader with `v` in the key slot. MEASURED with that one conjunct removed and
nothing else changed: 7 flat readers instead of 6 — `flatFieldRefusal` joins —
but **112 sites over 79 distinct keys, unchanged**. Part 2's scope rule already
filters it: a flat reader's call site only counts when its first argument is a
`.ConfigPath` selector, and `flatFieldRefusal`'s call sites pass a key string
there. So the exclusion held the derived SET honest and added no key to the
census on today's tree — a distinction worth keeping, because the next
`(string, string)` non-reader in that file may well be called with a config
path first.

## The fix

Part 1 now derives over BODIES. A function in `yamlflat.go` is a flat reader
when it hands its own first parameter to something that opens it as a file and
takes a string key second. Opening is transitive and the fixpoint is seeded
with `os.ReadFile`:

- `readLines(path string)` hands its parameter to `os.ReadFile` — an opener,
  but one parameter, so no key slot and not a reader;
- `YamlGet`, `YamlList`, `YamlMapPairsRaw`, `yamlHasKey`,
  `YamlKeysWithPrefix` hand theirs to `readLines`;
- `YamlMapPairs` hands its to `YamlMapPairsRaw`, one hop further out — the
  member the transitive half of the rule exists for;
- `yamlGetLines`/`yamlListLines` take `[]string` and reach no file;
  `flatScalarRoundTrip` takes one parameter; `flatFieldRefusal` takes two
  strings and opens nothing. All out, and out for the reason the head comment
  always gave.

The seed is stdlib on purpose. A seed spelled as a local identifier —
"whatever calls `readLines`" — would be the same convention-by-name the body
rule replaced, one layer down.

The key index stays a hardcoded 1, which is a shape and not a derivation: a
`(path, section, key string)` reader would be censused at `section`. That reds
loudly — `section`'s literal is reported as a config key the seed does not
document — so it is left as a shape, said in the comment rather than guessed
at.

After the change the real tree derives the same six readers and the same 112
sites over 79 distinct keys as before, so the census's reach did not move; what
moved is what holds it.

## Pinned, not believed

`TestConfigKeyCensusCanStillSayNo` drives the census over a planted tree, and
its `yamlflat.go` fixture now carries real bodies with the real file's hops.
Four non-readers sit beside the readers, one for each way part 1 must say no —
the two `(lines, key)` helpers, a one-argument function, and
`flatFieldRefusal`'s `(string, string)` shape — and a seventh reader,
`YamlGetAlt(file, key string)`, which is A2 as a fixture. The arm asserts the
derived set EXACTLY rather than counting it, because each name in it is a
different way the rule can go wrong and each absence a different way it can go
wide; `planted_alt_named`, read through `YamlGetAlt`, is in the wanted-key
list.

## The four copies of the claim

laurie's row named four places the "derived from the signatures" wording lives.
`internal/posse/seedkeycensus_test.go`'s head comment is **the record** — both
it and the inline comment at part 1 now state the body rule and what the name
rule cost. `CHANGELOG.md`'s "readers' own signatures" is corrected to "bodies".
`internal/treepins/treewidedoor_qa_test.go`'s summary never said "signatures"
and was not false, so the only change there is to say it IS a summary and to
name the census's head comment as the record when the two disagree. A copy
that claims less cannot go stale; four copies of a rule is three places for
the next correction to miss, which is what laurie's row asked to be settled.
