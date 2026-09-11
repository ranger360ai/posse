# Verbs that grew a subsystem: a responsibility census (ranger-base-zv1cu)

Tree `9ed6b79e`, 2026-09-11. Raised by two outside reviews of `f97cf1fc`:
[grok §10–11](ranger-base-vuosd.md) ("several verbs are a full subsystem
around one idea"; "gates are an argv parser, not a security boundary") and
[codex §5](ranger-base-b0fsz.md) ("remove unused policy and duplicated
readings first, then consolidate one state transition at a time behind its
existing owner"). The question this page answers is the bead's: **per file,
what would be lost if the next 400 lines were not written.**

No code changed under this bead. The deliverable is this page, the
instrument that produced its numbers (`scripts/responsibility-census.py`),
and three cut beads with done-when rows.

## Method

Every count below is MEASURED at `9ed6b79e` on 2026-09-11, macOS 25.4.0,
and re-derivable:

- **Line composition** — `python3 scripts/responsibility-census.py <file.go>`,
  optionally `<file.go>:START-END` for one region. It separates Go code from
  comment, from the POSIX shell that gates.go and visibility.go render out of
  multi-line string literals, and from blank. One line gets one class
  (comment > shell > blank > code), so the columns sum to the range. The
  numbers were cross-checked against an independent `go/ast` implementation
  that agreed on all five files.
- **Reachability** — for every exported name declared in the five files, a
  repo-wide reference count with `_test.go` excluded, then a second pass
  counting non-comment uses inside the declaring file. Name-occurrence
  matching, so a generic method name (`Line`, `Row`, `Match`) is a floor;
  the four readers named in cut 1 were each confirmed by reading every hit.
- **Round-trip probe** — one throwaway test in `internal/posse`, run and
  deleted, cited where it is used.
- **Test surface** — lines in test files whose name begins with the product
  file's stem. A FLOOR, not a total: `gates.go` is also pinned by
  `adrcensus_qa_test.go`, `chainrepair_qa_test.go`, `l3_hookspath_qa_test.go`,
  `managedhookspath_qa_test.go` and others that do not carry the stem.

## The answer in one table

| file | total | comment | shell | blank | **Go code** | tests (floor) |
|---|---:|---:|---:|---:|---:|---:|
| `gates.go` | 5,889 | 2,837 | 1,214 | 165 | **1,673** | 5,779 |
| `visibility.go` | 1,342 | 670 | 185 | 77 | **410** | 1,495 |
| `backup.go` | 1,249 | 385 | 0 | 77 | **787** | 2,403 |
| `ciwatch.go` | 1,202 | 598 | 0 | 58 | **546** | 2,189 |
| `govern.go` | 1,141 | 524 | 0 | 63 | **554** | 1,190 |
| **total** | **10,823** | **5,014** | **1,399** | **440** | **3,970** | 13,056 |

**The five "thousand-line files" are 3,970 lines of Go.** Forty-six percent
of their bulk is comment — the record of the incident each invariant answers
— and thirteen percent is a second language: a POSIX shell program written
inside Go string literals, all but 185 lines of it in `gates.go`. `wc -l` was
measuring three different things at once, and the three have different
verdicts.

Three claims in the source reviews move on these numbers:

- **"A lint that is 1,342 lines is a policy engine"** (grok §11) — half
  right. `visibility.go` is 410 lines of Go. What is a thousand lines is the
  *text*: 670 comment, 185 lines of rule-and-remedy prose that the refusals
  print verbatim. It is not a policy engine, it is a policy *phrasebook*,
  and its growth rate is one class = one more set of constants (below).
- **"A cron plus tar would have been the do-nothing option"** (grok §11) —
  no. `backup.go:394–780` does not tar the store: it reads `beads.db`
  through sqlite's online-backup API via `pairCheckReader` (the WAL case
  `mode=ro` cannot open at all), bundles the queue repo with
  `git bundle create --all`, and writes a sha256 manifest that
  `VerifyBackup` walks back. `tar` over a live WAL database produces a file
  that restores to a corrupt db, silently. The 384 code lines that are not
  "tar" are the reason the verb exists.
- **"Gates are an argv parser"** (grok §10) — the argv matcher is 635 of
  `gates.go`'s 1,673 code lines. The other 1,038 are three subsystems that
  are not an argv parser and do not belong in the same file (below).

And codex's order is the right one, because there *is* unused policy —
three exported readers nothing in production reads (cut 1).

## Per file

Each block table is: line span · Go code lines · what class the block is ·
verdict · the measurement that would justify the verdict.

Classes: **IDEA** = the thing the file's header says it exists for.
**IMMUNE** = what protects that idea from a failure the tree has already
had. **SURFACE** = renderings and vocabulary. **OTHER** = a different idea
living in this file.

---

### `backup.go` — 1,249 lines, 787 Go

**The one idea** (`backup.go:3–30`): the store of record's backup is a
harness verb, on-box, and a remote target is REFUSED — the refusal is the
design, not a flag. ADR 0036's off-box half (sweep, age identity, the second
clock) was cut by the 2026-09-01 sub-ruling and the header carries the
record of what went with it.

| block | code | class | verdict | measurement that would justify it |
|---|---:|---|---|---|
| `1–90` header, consts | 21 | IDEA | KEEP | the header is the only place the three cut decisions survive; deleting it re-opens them by amnesia |
| `91–238` config keys, arming | 66 | SURFACE | KEEP | 5 keys; `BackupConfigured` is the inertness rule (an unarmed instance reports nothing) |
| `239–308` `CheckBackupTarget` | 53 | IDEA | KEEP | this is the whole of "refuses any remote target", and it runs at every use, not at config time |
| `309–393` types, `BackupHomePaths` | 40 | IDEA | KEEP | the copy path walks a named list, never a tree; `BackupExcluded` is what keeps `secrets/` out |
| `394–780` stage + archive | 295 | IDEA | KEEP | 37% of the file's code and the only part a `tar` could not replace — sqlite online backup, `git bundle --all`, sha256 manifest |
| `781–877` `VerifyBackup` | 89 | IMMUNE | KEEP | an archive nobody has opened is a belief; this opens it against its own manifest at write time (`:494`) |
| `878–1025` freshness + 3 renderings | 96 | IMMUNE | KEEP | the future-stamp clause: `time.Sub` is negative for a stamp ahead of the clock and `BlindFor` rendered that as "0s", i.e. a dead backup reading as fresh (`:921–935`, `splitBackupsAt:1092`) |
| `1026–1249` list, prune, lock | 127 | IMMUNE | KEEP | `pruneBackups` + `lockBackupDir`: two backups racing the same directory, and `backup_keep: 0` refused because "keep none" deletes the copy it just made (`:178–190`) |

**What would be lost if the next 400 lines were not written: nothing.** This
verb is finished. Every remaining feature in ADR 0036 belongs to the
destination the operator cut, and the file says so in as many words
(`:7–30`): §5's age identity comes back WITH an off-box destination, in that
order, or not at all.

**Cap, measurable:** a new line in `backup.go` that is not a bug fix needs an
ADR 0036 amendment naming the destination first. Nothing to consolidate; the
one duplicated shape is small and named in the watch list.

---

### `ciwatch.go` — 1,202 lines, 546 Go

**The one idea** (`ciwatch.go:3–33`): a red `ci.yml` run on `main` files ONE
bead and says on that bead when main is green again. The incident it answers
is in its own header: 191 consecutive failed runs over five days and ~120
commits, nobody noticed, and a genuine break was indistinguishable from the
standing noise.

| block | code | class | verdict | measurement that would justify it |
|---|---:|---|---|---|
| `1–167` header, the incident | 16 | IDEA | KEEP | 148 comment lines; this is the file's specification and the reason the dedupe is a store read |
| `168–253` markers, labels, caps | 13 | IDEA | KEEP | `ciMarker` is the dedupe of record — repo + workflow + branch, one line |
| `254–543` the reading (`ReadCI`, `gh`) | 171 | IDEA | KEEP | "unknown" never renders as "clear": `CIState.Why`/`NoGate` carry why no reading was taken |
| `544–628` `ciCauses` | 63 | SURFACE | KEEP, capped | the only block that forks `gh` per red run (cap 10, `ciCauseScanCap:208`); it buys attribution — "TestX (3 runs)" on the bead — which is precisely what the five-day red spent |
| `629–710` bead text renderings | 40 | SURFACE | KEEP, but see below | four of these strings are also the state schema |
| `711–828` abstain, workflow, `CIWatch` | 53 | IMMUNE | KEEP | a reading that cannot be taken must not look like silence, and must not print every pass (`ciAbstained`, once per process) |
| `829–1202` act, dedupe, file, clear | 190 | IMMUNE | KEEP | the invariant itself: `ciOpenBeads` newest-first, `ciDupeFiled`'s one grace pass (ranger-base-jbnug: d3dgd closed 05:04:24, green at 05:07:46, a second bead filed 05:09:16), `ciHolder`'s ADR 0013 §4 exception |

**What would be lost if the next 400 lines were not written: nothing the
invariant needs.** 190 code lines already hold it and each one names the
race it closed. The file's real exposure is not its length:

> **The bead's rendered English IS the state store.** `ciSinceRe`
> (`:971`) parses the sha back out of `streakLine`'s two sentences
> ("…since %s at %s" / "…the oldest still in the window is %s at %s",
> `:678–689`); `ciStreakRe` parses the count out of the same line;
> `ciAlreadyCleared` and `ciRaceAbstained` prefix-match comment text. If a
> reword breaks the sha read-back, `ciDupeFiled`'s candidate loop
> `continue`s — silently — and the mechanism files the duplicate bead it
> exists to prevent.
>
> MEASURED 2026-09-11 (throwaway test, run and deleted): both shapes round-trip
> today, capped and uncapped, `ciSinceSha(Description()) == Since.Short()`.
> `ciwatch_test.go` pins `ciLastStreak` against both shapes but pins no
> round-trip for the *sha*. Cheapest honest fix is four lines of test, not a
> schema (watch list).

**Cap, measurable:** the next 400 lines here would be more attribution or
another race class. A new race class needs the incident id and the bead
comment that proves the store read; attribution past `ciCauseScanCap` needs
a measurement that a capped scan lost a cause anyone wanted.

---

### `govern.go` — 1,141 lines, 554 Go

**The one idea** (`govern.go:3–31`): facts get computed, decisions get beads.
Conditions are level-triggered, computed live from the store that owns each,
and never written down; one function (`ShopCheck`), three renderings.

| block | code | class | verdict | measurement that would justify it |
|---|---:|---|---|---|
| `1–45` header | 11 | IDEA | KEEP | the "no attention file, no attention bead" decision lives only here |
| `46–305` vocabulary (`GovSet`, `GovInputs`, `Pause`) | 114 | SURFACE | KEEP, minus 2 methods | `GovSet.Urgent` has ZERO callers; `GovSet.Has` has one test caller while `cmd/posse/cockpit.go:2331` open-codes it (cut 1) |
| `306–593` `ShopCheck` G1/G4/G5/G6/G7/G8 | 133 | IDEA | KEEP | 9 conditions, 12 `add` sites, ~60 code lines per condition including its comment |
| `594–1026` the bd half, plan reading, dial E | 217 | IDEA+IMMUNE | KEEP | G7's four keys (`arm-broken` twice, `loop-dead`, `loop-mute`) are all one fact with different causes — a second row name would say the shop has two problems where it has one (`:418–455`) |
| `1027–1141` four renderings | 79 | SURFACE | KEEP | all four have live non-test callers: `GovReport` (`cmd/posse/main.go:982`), `GovSummary` (`cockpit.go:2296,2329`), `GovOrdered` (`cockpit.go:2444`), `GovLines` (`pulse.go:260`) |

**What would be lost if the next 400 lines were not written: six or seven
governance conditions** — that is the honest answer, and it is the only file
of the five where the next 400 lines buy new facts rather than new defences.
The three renderings are already paid for: a new row costs `add(...)` plus
its comment, and `Keys`/`Row`/`GovOrdered` render it without being touched.
That is the design working.

**Cap, measurable:** the ADR 0029 test is already the cap and it is
checkable — a new row must be a fact any process can compute twice with the
same answer, and must not need a rendering of its own. A proposed G-row that
needs new code in `GovReport`, `GovSummary` *or* the cockpit is a decision
wearing a condition's clothes, and decisions are already bead-shaped.

Note the two days since the reviews: both `govern.go` commits (`f07b073d`,
`8a4ef0fe`) corrected the *reading* behind an existing row. Neither added
one.

---

### `visibility.go` — 1,342 lines, 410 Go, 185 lines of refusal prose

**The one idea** (`visibility.go:3–22`): a lint, and it says so. One pattern
list, two readers — the `prepare-commit-msg` hook greps added lines with
these EREs, and the harness warns with the same ones before it files a bead
itself. The EREs live in the intersection of POSIX ERE and Go's `regexp`
because both readers must accept them.

| block | code | prose | class | verdict | measurement that would justify it |
|---|---:|---:|---|---|---|
| `1–136` header, visibility + docs-genre + ops-prose rules | 25 | 25 | IDEA | KEEP | the rule text IS the refusal's output; a refusal that names no rule is a regex saying no |
| `137–372` pattern type, shipped list, `ScanOps`, `WarnOpsContent` | 115 | 0 | IDEA | KEEP, minus 2 | `ScanOps` and `OpsPattern.MatchedText` are exported and read by TESTS ONLY — production scans through the unexported `opsClassesOf` (cut 1) |
| `373–822` instance classes, data ceiling, message notes | 105 | 86 | IDEA+SURFACE | KEEP | ADR 0048/0050; `OpsPatternSet` reads both config keys through one class namespace, ceiling first, so a class in both is refused on the visibility side |
| `823–1208` identity + crew literals | 110 | 44 | IDEA | KEEP | `DeriveIdentityLiterals` is the only class derived from the BOX rather than shipped; `identityLiteralMaxLen` is the guard on a pathological value |
| `1209–1342` guard-value literals | 55 | 30 | IDEA | KEEP | the sentinel skips ("0", "true", "false") are a re-measurement (ranger-base-ei046), not a taste call |

**What would be lost if the next 400 lines were not written: roughly four
more classes.** Measured cost of a class today: a `Rule` constant, a
`WayThrough` constant, a `MessageWayThrough` constant, a derive function
where the class is box-derived, and a shell arm in `gates.go` — where a
check costs 268 lines (`visibilityGuardBody`) to 275 (`identityGuardCheck`),
plus whatever shared renderer it has to bring with it (the ceiling brought
`messageArm`: 440 more).

Two findings this file's own header no longer covers:

1. **"One pattern list, two readers" is now one list and one-and-a-half
   readers.** The Go reader (`WarnOpsContent:319`) scans the ceiling and the
   visibility patterns. The identity, crew, guard-value and docs-genre
   classes are enforced in the shell hook ONLY. That is defensible — the hook
   catches every entry path and the harness warning is a courtesy — but the
   header reads as if both readers see the same policy, and they do not.
2. **The refusal prose is four copies of three sentences.** "Nothing was
   committed: HEAD is unchanged…" appears 3 times, "rewrite the commit
   message" 4 times, `.git/COMMIT_EDITMSG` 7 times, across
   `OpsInstanceMessageWayThrough`, `DataCeilingMessageWayThrough`,
   `IdentityMessageWayThrough` and `GuardValueMessageWayThrough` (watch
   list). The shop's own rule for this is one text, one reader.

**Cap, measurable:** a new class should cost one registry row plus its rule
text. When the next class is added, if it costs four new constants and a
hand-written shell arm again, that is the measurement that says the
phrasebook needs a table.

---

### `gates.go` — 5,889 lines, 1,673 Go, 1,214 lines of rendered shell

**The one idea** (`gates.go:3–33`): L1 is a POSIX sh shim on PATH, rendered
from the PID's `deny:` rules, that matches the first non-option token after
the command's own global options — because `git -C <repo> push` walked
straight past a positional matcher (rangerhq-2zm).

That idea is 635 code lines. The file is four subsystems:

| region | total | shell | code | what it is | verdict |
|---|---:|---:|---:|---|---|
| `1–78` | 78 | 0 | 13 | the header, and the only place L1's cooperative scope is stated | KEEP |
| `79–1645` | 1,567 | 48 | 635 | **L1**: the argv matcher, the option tables, the shim renderer, the gate shell, the credential-gate collision check | KEEP — this is the file |
| `1646–2571` | 926 | 78 | 346 | **L3 plumbing**: hooks dir resolution, `core.hooksPath`, chaining, `installHook`, the push-deny/allow parity readers | MOVE |
| `2572–5392` | 2,821 | 1,071 | 433 | **the `prepare-commit-msg` body**: shared-index arm, visibility guard, data ceiling, identity check 3, constitution guard, and the ADR census verb | MOVE, in pieces |
| `5393–5889` | 497 | 17 | 246 | **L3 probe + install**: `CommitGuardHook`, the hook-identity probe, degrade lines | MOVE |

Inside the `prepare-commit-msg` region, measured per check:

| check | span | total | shell | code |
|---|---|---:|---:|---:|
| shared-index arm | `2572–3024` | 453 | 276 | 1 (two consts, 174 comment) |
| `visibilityGuardBody` (checks 0 and 2) | `3135–3402` | 268 | 218 | 23 |
| `dataCeilingCheck` | `3724–3802` | 79 | 20 | 59 |
| `messageArm`, shared by ceiling and check 3 | `3803–4242` | 440 | 52 | 13 (373 comment) |
| `identityGuardCheck` (check 3) | `4333–4607` | 275 | 53 | 182 |
| constitution guard | `4822–4933` | 112 | 94 | 11 |
| `adrShaPredicate` | `4934–5120` | 187 | 104 | 2 |
| ADR census verb | `5210–5354` | 145 | 51 | 47 |

The `messageArm` row is the file in one line: 440 lines to render one shell
arm, 373 of them a single comment explaining what git writes below a
verbose commit's cut line and why only some of it is message
(ranger-base-gyrnp, ranger-base-d94zl). That comment is the most expensive
thing in `gates.go` per line of behaviour, and it is also the reason the arm
is right.

**What would be lost if the next 400 lines were not written** — this is the
one file where the answer differs by region:

- **L1 (`79–1645`)**: real holes. Every recent addition here closed one —
  `33860743` two days ago taught the push alarm to look for `git` at every
  word of a rule rather than only the first, +83/−10. Grok §10 is right that
  L1 is cooperative and wrong that this makes its growth optional: a
  cooperative wall with a hole is worse than one without, because the PID
  claims the deny.
- **The hook body (`2572–5392`)**: whatever the next ADR asks for, at the
  measured price of ~300 lines a class. Nothing is lost by not writing them
  *in this file*.
- **The ADR census (`5210–5354` + `4934–5120`)**: nothing. It stopped being a
  gate at the 2026-09-05 operator ruling (ranger-base-bp0yj) and is now an
  on-demand verb, `posse gates adr-census`, already pinned by its own
  `adrcensus_qa_test.go` (cut 2).

**Cap, measurable:** this file does not need a line cap, it needs a split,
and the split is already drawn by the tests — six stem-matched test files,
plus five more that name only one subsystem each. The measurement that
justifies moving a block is that no symbol crosses in both directions: the
ADR census's only tie to the rest of the file is `diffReaderShape`, one
const (cut 2).

## Growth, measured

`f97cf1fc` (the reviews' tree, 2026-09-09) → `9ed6b79e` (today): the five
files took **148 insertions and 15 deletions in four commits**, net +133.

| commit | file | ± | what it was |
|---|---|---|---|
| `33860743` | gates.go | +83/−10 | the push alarm reads every word of a rule |
| `ed169b79` | gates.go | +20/−0 | read-only gates verbs for a census |
| `8a4ef0fe` | govern.go | +23/−5 | G2 drops for live work |
| `f07b073d` | govern.go | +22/−0 | a reading that flattened FAINT text |

**Every one of the four is a correction to an existing invariant. None adds
a capability.** At that cadence "the next 400 lines" is about six days —
and on this evidence they will be immune system, which is the answer to the
bead's question in the general case: what would be lost is not features, it
is the tree's accumulated knowledge of how each of these mechanisms has
already failed. That knowledge is worth its length. What is NOT worth its
length is keeping four subsystems in one file, and keeping exported readers
that only tests read.

## The first three cuts

Filed as separate beads, in codex's order — unused policy and duplicated
readings first, then one move at a time behind its existing owner.

1. **ranger-base-zyou5** — the four exported readers nothing in production
   reads: `GovSet.Urgent` (zero callers anywhere), `GovSet.Has`, `ScanOps`,
   `OpsPattern.MatchedText`.
2. **ranger-base-tn3u2** — the ADR census is not a gate: move it out of
   `gates.go` (−332 lines, pure move).
3. **ranger-base-xfo6d** — one refusal shape: fold `visibilityGuardBody`'s
   two hand-rolled arms (`gates.go:3244–3355`, `:3356–3402`) onto
   `visGuardRefusal.render`, which the ceiling and check 3 already use.

## Watch list (measured, not filed)

- `ciwatch.go` — pin the sha round-trip `ciSinceSha(Description())` for both
  streak shapes. Four lines of test; today it passes (MEASURED above) and
  nothing holds it there.
- `visibility.go` — the three sentences every MESSAGE refusal shares belong
  in one constant. Counts above.
- `gates.go` — the L3 plumbing (`1646–2571`) and the L3 probe (`5393–5889`)
  are 1,423 lines of git-hook mechanics in the argv-matcher's file. The next
  move after cut 2, and bigger than it; not filed because two file moves at
  once is a review nobody can do.
- `backup.go:178–207` — `BackupKeep` and `BackupMinFree` are two copies of
  "read an int key, warn once, take the default". Package-wide there are 8
  such readers (backup 2, govern 2, pulse 2, backuploop 1, uncounted 1).
  Too small to file; the number to watch is 8.

## What I could not verify

- Full suite colour at this SHA. This bead changed no product code; the one
  probe test I ran passed and was deleted.
- Whether any block is dead at *statement* level. The reachability sweep is
  name-occurrence with the hits read by hand, not a call graph.
- The test-surface column is a floor (stem-matched files only) and
  understates `gates.go` most.
- Grok's and codex's own numbers were not re-derived except where this page
  says so; `gates.go` has grown 93 lines since the tree they read.
