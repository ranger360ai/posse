# ranger-base-p9qve — the seed documented-default census gets a NUMBER half

The seed documented-default census (`internal/posse/seedconfig_test.go`,
property 4) held **durations only**, and by construction rather than by
omission: its derivation keys on readers whose result is a `time.Duration`,
so a key read as an `int`, a `float64` or a `uint64` was outside it however
many bare numbers `examples/config.yaml` documented.

## MEASURED 2026-10-05 — what the seed actually documents

Ten commented bare-number lines ship in `examples/config.yaml`:

| line | key | value | what holds it, before |
|---|---|---|---|
| 304 | `verify_batch` | 4 | nothing |
| 368 | `plan_guard_5h` | 70 | nothing |
| 369 | `plan_guard_7d` | 85 | nothing |
| 576 | `budget_pass` | 30 | nothing |
| 577 | `budget_day` | 250 | nothing |
| 630 | `uncounted_cap_codex` | 20 | nothing |
| 664 | `grok_guard_week` | 85 | nothing |
| 666 | `grok_pool_usd_per_point` | 0.50 | nothing |
| 733 | `load_guard` | 25 | nothing |
| 846 | `autostart_max_beads` | 3 | nothing |

And three of the tree's number keys pair a key literal with one
`Default`-shaped constant through a reader of a derived shape:

| key | reader | constant | value |
|---|---|---|---|
| `backup_keep` | `BackupKeep` (`int`) | `DefaultBackupKeep` | 3 |
| `load_guard` | `LoadGuard` (`float64`) | `LoadGuardDefault` | 25.0 |
| `verify_batch` | `verifyBatch` (`int`) | `DefaultVerifyBatch` | 1 |

Zero numeric CALL-SITE readers: no method on `App` has the shape
`(key string, def <numeric>, errw io.Writer) <numeric>`. `budgetDollars` and
`planPercent` are the near misses and both are
`(key string, errw io.Writer) float64` — the shape with no default parameter
at all, because unset is no cap and no guard rather than a number.

**So only `load_guard` is a documented number that agrees with a constant,
and `verify_batch: 4` is documented at four times its default on purpose.**

## Why seven of the ten have no constant to drift from

The house vocabulary for a number key is overwhelmingly "unset means OFF":
`budget_pass:`/`budget_day:` unset is no cap, `plan_guard_<window>:` unset is
no guard, `grok_guard_week:` unset is the pool meter off,
`uncounted_cap_<runtime>:` unset is unlimited. A line beside such a key is a
SUGGESTED SETTING and not a claim about a default — and the seed's own prose
says so: "suggested:", "shapes, not recommendations", "ARBITRARY SHAPES
chosen to be obviously not anyone's".

That is why this half's third register (`seedDocumentedNumberOutsideTheCensus`,
seven rows) is larger than the duration half's (two rows), and why it is not a
hole: each row names the reason there is no constant for its line to drift
from, and each row's staleness half fails when it stops describing anything —
including a row whose key the derivation has since learned to REACH, which is
what keeps that register and `seedNumberDefaultNotAConstant` from both
claiming one key. The discriminator between the two is mechanical (does the
derivation reach it) and not prose.

## One mechanism, two predicates

The widening is a widening and not a second census beside the first, which is
the distinction the bead was filed on: a one-off pin for a seventh key would
be a second implementation going narrower than the derived one while both
looked green. Everything structural is said once:

- `seedReaderDecl(fd, isValue)` derives both reader shapes, the key-literal
  rule, the `Default`-shaped-constant rule, the four near-misses and the
  reporting of an unpairable key. The duration half passes
  `seedIsTimeDuration`, the number half `seedIsNumeric`.
- `seedDocumentedLines(cfgText, looksLike)` enumerates the seed's own
  commented `key: value` lines. The duration half passes
  `seedLooksLikeDuration`, the number half `seedLooksLikeNumber`.
- `seedDocumentedValue`, `seedDefaultShaped`, `seedConfigKeyArg`,
  `seedUniqSorted` and the census walk are not restated at all.

A reader shape that stops matching stops matching for both halves at once,
which is the property a parallel copy would not have.
`TestSeedNumberDerivationCanStillSayNo` plants a `time.Duration` reader in
the number half's planted tree and asserts each half sees only its own,
so a leak in either direction reds.

### The comparison type

float64, because the tree's number readers answer in three different types
and one carrier keeps the drift check a single code path. float64 holds every
integer below 2^53 exactly, so every count, percent, dollar figure and
megabyte ceiling this config can carry resolves exactly and the comparison is
`==` rather than a tolerance nobody measured.

The hand-written conversion in `seedNumberBodyReaders` is the one place a
mistake could hide — wiring a key to the wrong reader would feed every
comparison the wrong number — so the pin hands each body-form reader a config
with its own key ABSENT and requires the answer to be `seedNumberDefaults`'
value for the constant the derivation paired it with. Mutant M8 below is that
check firing.

It is still a read through the PRODUCTION reader and never a text compare:
`25`, `25.0` and ` 25 ` are the same load average, `1.5` is a number that
`verifyBatch`'s own grammar refuses, and a value the reader refuses makes it
name the value on errw and return the default — which would otherwise let a
line that proves nothing pass by landing on the very number it was supposed
to prove.

### The one difference from the duration half, stated

`seedDurationDefaultNotAConstant` carries a STRONGER half — a key in it must
also stay UNDOCUMENTED, because its one member (`backup_max_age`) has a
default that is a rule with no number at all, so a documented line for it
would be a claim about nothing. `seedNumberDefaultNotAConstant` carries no
such half. Its two members both have something a reader can state —
`BackupMinFree`'s default is `DefaultBackupMinFreeMB` scaled to bytes,
`GrokGuardWeek`'s is an explicit "off" — so a documented line beside either is
a suggestion rather than a false claim, and refusing it would mean deleting a
live seed line the operator uses to see what shape the setting takes. What
keeps the drop on the record instead is the row's reason plus the seed-side
accounting, which logs the line by name and by file position every run.

## MEASURED 2026-10-05 — ten mutants, ten deaths

One mutant per thing the pin claims, run against
`TestSeedConfigDocumentedNumber*` in this worktree:

| # | mutant | what died |
|---|---|---|
| M1 | seed `load_guard: 25` → `50` | `documents 50 (= 50), the constant is 25` |
| M2 | `LoadGuardDefault` 25.0 → 30.0 | `documents 25 (= 25), the constant is 30` |
| M3 | the `load_guard:` line deleted | `compared 1 documented number defaults, want at least 2`, and the duration/number floor |
| M4 | a new documented number line with no reader | `documents planted_new_cap as 9, and this census never reached that key` |
| M5 | seed `verify_batch: 4` → `1` | the on-purpose register's live half: `which IS DefaultVerifyBatch (1) — but … says it is deliberately not the default` |
| M6 | `seedDocumentedNumberOutsideTheCensus["budget_pass"]` dropped | `documents budget_pass as 30, and this census never reached that key` |
| M7 | `LoadGuard` dropped from `seedNumberBodyReaders` | `the tree has a body-form number reader seedNumberBodyReaders does not name: LoadGuard` |
| M8 | the `BackupKeep` closure wired to `BackupMinFree` | `BackupKeep over a config with backup_keep absent returns 4.02653184e+08, but seedNumberDefaults says DefaultBackupKeep is 3` |
| M9 | a NEW int key with a derived-shape reader, documented at 10x | first `falls back to DefaultLauncherBehindMax, which seedNumberDefaults does not name — add "DefaultLauncherBehindMax": …` and `never reached that key`; then, once the tables name it, `documents 160 (= 160), the constant is 16` |
| M10 | a numeric CALL-SITE resolver arrives | `the tree has a call-site number reader seedNumberResolvers does not name: plantedCount` |

M9 is the bead's own motivating case — `launcher_behind_max`, the seventh key,
which `ranger-base-y13h7` adds. It is not in the tree yet
(`grep -rn launcher_behind_max` finds nothing at this HEAD), so the census is
written to reach it prospectively rather than to name it: when a reader of
either derived shape lands, the census reports the key, names the exact table
edit, and then compares the seed line against the constant. **A seat landing
`launcher_behind_max` will see `make seed-check` fail until
`seedNumberBodyReaders` and `seedNumberDefaults` name its reader and its
constant** — that failure is the mechanism working, and its message says what
to add.

M10 is the only exercise the number half's call-site arm gets outside
`TestSeedNumberDerivationCanStillSayNo`'s planted tree, since
`seedNumberResolvers` is empty. The map stays because the ARM stays: the first
numeric call-site reader anyone adds is a named failure instead of a key that
quietly resolves nothing.

## Out of scope, filed

`autostart_max_beads`'s default `3` is a SHELL literal:
`plugin/autostart.sh` reads the key and falls back in a `case`, twice. There
is no Go constant, so an AST census over Go sources cannot reach it by
construction. The seed's `3` and the script's two `3`s are three copies with
no edge between them, and `internal/posse/autostart_test.go` pins the
script's BEHAVIOUR rather than the seed line against it. Registered with that
reason and filed as **ranger-base-m9mwc**.

## The door

Two new tree-wide pins, both folded into `make seed-check` beside the
duration half's two, for the same reason: their subject is
`examples/config.yaml`. `make tree-check` is 48.4-51.5s over three warm runs
at fifty-five pins and fourteen doors (MEASURED 2026-10-05, load average 9.6
to 36.5 across the three with a sibling seat holding a suite slot; it was
46-51s at fifty-three).

The disjointness claim — no commented line is enumerated as both a duration
and a number — lives INSIDE
`TestSeedConfigDocumentedNumberDefaultsAreTheConstants` rather than in a test
of its own. A pin over this one tracked file is not the tree-wide class
(`treewidedoor_qa_test.go` is explicit that a reading of one path at the root
is not that class), so a test of its own would get no door and would be a
~950s package run away — which is the gap the doors exist to close.
