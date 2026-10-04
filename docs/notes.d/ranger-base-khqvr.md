# A derived census narrowed by a hand list of two names reached 4 of 10 keys (ranger-base-khqvr)

2026-10-04. ranger-base-vofbl landed "every documented default matches its
constant" over `examples/config.yaml` and its head comment said the derivation
"makes it total" — "a new duration key reds this pin until it is in it". The
verify lane measured it at 4 of the 10 duration keys the seed documents with a
value (ranger-base-2vynj finding 1), with six mutants surviving against two
controls that died. This file is what the generalisation had to become, and
the two rules it was missing.

Environment for everything below: this worktree, go1.26.5, darwin 25.4.0,
`go test -count=1` filtered runs. Mutants applied in-tree with the work
committed first, restored from a golden copy and checksummed back, with
`git status --porcelain` asserted empty before and after each.

## Why a derivation went stale anyway

The pin derived its key/constant pairings from the tree instead of a hand map,
for a stated reason: "a key->constant map would be a hand list with a silent
hole for the next key". It then **filtered call sites through a hand list of
two reader NAMES** — the same shape, one level up, and with the same hole. Two
independent narrowings, both invisible:

1. **The resolver rule.** `attnAge` and `graceAfter` are the only readers that
   take the key and the default as ARGUMENTS. Six keys have a reader of their
   own that spells both in its BODY (`YamlGet(a.ConfigPath, "plan_usage_ttl")`
   one line, `return PlanUsageTTLDefault` the next). A reader not in the hand
   list contributed no keys, and nothing said so.
2. **The constant-name rule.** `strings.HasPrefix(id.Name, "Default")` misses
   `ModelProbeTTLDefault`, `PlanUsageTTLDefault`, `PlanUsageStaleAfterDefault`
   and `PlanGuardBlindMaxDefault`. Both spellings are live in this tree.

Four of the six keys failed both halves at once, which is why the census's own
log read plausible: it named the keys it compared and the keys it found
undocumented, and the six it never met appeared in neither list.

**The lesson is not "derive it".** It is that a derivation is only total up to
its own hand-written inputs, and each one needs a check in the direction that
hurts: *the tree has a member this table does not name.* The old pin had that
check for one of its two tables (`seedDurationDefaults` fails on a constant it
cannot resolve) and not for the other, and the gap was exactly the size of the
table that had no such check.

## The shape that landed

Both ways the harness states a pairing are now derived structurally, and all
three hand tables are made total by the derivation:

| rule | derived from | members today |
| --- | --- | --- |
| call-site form | a `(string, time.Duration, io.Writer) time.Duration` method on `*App` that passes its key PARAMETER to `YamlGet` | `attnAge`, `graceAfter` |
| body form | an `(io.Writer) time.Duration` method on `*App` with exactly one `YamlGet`/`CfgGet` key literal and exactly one `Default`-shaped returned identifier | 6 readers |
| a constant | `Default*` **or** `*Default` | 13 rows |

Each table is checked from both ends: a reader the tree has and the table does
not name is a failure that names the reader, and an entry nothing resolves any
more is a failure that says to drop it. For a body-form pairing the constant's
VALUE is cross-checked too — the reader handed a config with its own key
ABSENT must return the table's number — so a pairing the parse got wrong (right
key, wrong constant) cannot sit there green. A call-site reader cannot be
checked that way: its default is the argument, so it answers whatever a test
hands it.

The pairing count floor sits BELOW the totality checks, which mutation
decided. Above them, disabling either reader rule printed "derived 6 pairings,
want at least 10" and stopped — a symptom — and never reached
"seedDurationResolvers names attnAge, which is no longer a call-site duration
reader in the tree", which is the cause.

## The third way a key leaves the census, and the register for it

`backup_max_age` is read through `attnAge` with `a.defaultBackupMaxAge()` — a
CALL, not an identifier — so it was invisible for a reason neither rule above
touches, and it left in silence. ADR 0036 §6 makes its default a rule rather
than a number (2x the scheduled interval), so there is no constant for a
documented line to agree with and it should NOT be forced into one.

`seedDurationDefaultNotAConstant` is the register: key -> why, failing on an
unregistered unpairable key and on an entry nothing is unpairable about any
more. It carries the stronger half as well — **a key in it must stay
UNDOCUMENTED in the seed**, because a documented line for it would be a number
this pin cannot hold, which is the stale-for-free shape the whole block exists
to prevent.

## What stays outside, and why that is not a hole

Twelve commented `key: value` lines in the seed carry a duration. Ten have a
constant default and are the subject. The other two are correctly outside:
`autostart_interval` has NO default (its presence is the arm switch, absent
means off) and `autostart_max_interval` defaults to 8x the base rather than to
a constant. Neither is read through a reader of either derived shape, so
neither is derived. A register row for a key with no constant to drift from
would be the wrong answer.

MEASURED: the body-form rule scans `CfgGet` key literals beside `YamlGet`
ones — the two disagree about which argument is the key — and that adds no
member today. It is there so a future `CfgGet`-based duration reader does not
escape the way these six did.

## Mutants

The bead's own six, against
`TestSeedConfigDocumentedDurationDefaultsAreTheConstants`: one mutant per
documented line, each replacing that line's documented duration with a
different one, applied to `examples/config.yaml` alone. All six SURVIVED
before this change. The last two rows are the verify lane's controls, which
killed then and must still — they are what make the six evidence rather than a
reading, since the rig is shown able to fail on the same file in the same run,
one line away. The before/after pairs are on ranger-base-2vynj and
ranger-base-khqvr; this table restates the outcome, not the values.

| documented key mutated | the reader the failure names | before | now |
| --- | --- | --- | --- |
| model probe TTL | modelavail.go | SURVIVED | KILLED |
| verify batch age | verifyafter.go | SURVIVED | KILLED |
| plan guard blind max | planusage.go | SURVIVED | KILLED |
| plan usage TTL | plancache.go | SURVIVED | KILLED |
| plan usage stale after | planstale.go | SURVIVED | KILLED |
| dispatch epoch | epoch.go | SURVIVED | KILLED |
| `attn_parked_age` (control) | govern.go | KILLED | KILLED |
| `verify_box_max_age` (control) | verifybox.go | KILLED | KILLED |

And the new rules themselves, each mutated to the narrowing it replaced:

| mutant | killed by |
| --- | --- |
| the constant rule back to prefix-only | the body-reader staleness arm, and `compared 6`, want 10 |
| the body-form rule never matches | `seedDurationBodyReaders names …`, then the pairing floor at 8 |
| the call-site rule never matches | `seedDurationResolvers names attnAge`/`graceAfter`, then the floor at 6 |
| an unpairable call site dropped silently | the register's stale half, naming `backup_max_age` |
| one constant row pointed at the wrong value | the absent-key cross-check, naming the reader and both numbers |
| `CfgGet`'s key argument index moved to `YamlGet`'s | the planted-tree control, on the `CfgGet` row |
| one documented line deleted from the seed | `compared 9 documented duration defaults, want at least 10` |

`TestSeedDurationDerivationCanStillSayNo` drives the whole census — both
passes — over a PLANTED tree rather than the repo, so it is not itself a
tree-wide pin and a change to the harness's own readers cannot move its
subject. It holds all four near-misses (a default that is a call, two keys in
one reader, a fallback that is not `Default`-shaped, a call site whose default
is a call) and the four things that only LOOK like a reader (a free function,
a non-duration reader, a `_test.go` file, a file that does not parse).

## The trust key's compatibility fixture (finding 2)

`TestTrustKeyIsNFCWhereClaudesIs` had seven arms and every non-ASCII fixture
in them was canonical — `re`+U+0301 and U+00E9 — on which a compatibility form
agrees with NFC. So `norm.NFKC` in `claudeProjectKey` passed the whole test,
and no compatibility-character fixture existed anywhere in the package.

One arm fixes it: a repo directory named with U+FB01 LATIN SMALL LIGATURE FI,
which NFC leaves as one rune and NFKC folds to two letters. MEASURED: with
`norm.NFKC` in place, that arm is the ONLY one of the eight that fails; with
`norm.NFKD`, six of the eight fail. This is the twin of the keychain row
landed under ranger-base-2vynj (42ac3a9e), one normalization axis over —
ranger-base-d88rp's call site rather than ranger-base-snrur's.
