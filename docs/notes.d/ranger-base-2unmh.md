# Four closes verified, and four arms nothing witnessed (ranger-base-2unmh)

2026-11-22, linux 7.2.15 x86_64, go1.27.3, bwrap 0.13.1, 16 cores. The batch
was ranger-base-evx07, -zbmxc, -byqv0, -ayubz, all four landed on `main`
between 63975006 and 7caf9a48 and all four ancestors of the tree this was
measured on.

All four closes VERIFY. Every mutant their closers recorded was re-run
un-combined and every one reproduced, three of them to the exact row count.
What the verification added is the other question — not "is this arm
necessary" but **"is this arm WITNESSED"** — and there the answer was no in
four places, three of which are pinned here.

## the closers' own mutants, re-run un-combined

| close | the claim | re-measured |
|---|---|---|
| byqv0 | drop the directory open's `O_NOFOLLOW` reds all 28 link rows | KILLED, **exactly 28** |
| byqv0 | drop its `O_DIRECTORY` reds 5 deepest-component rows | KILLED, **exactly 5** |
| byqv0 | truncating in the cursor's open reds the hardlink row | KILLED, **exactly 1** |
| byqv0 | dropping the cursor's `Truncate` reds the round trip | KILLED |
| ayubz | `CageBeadsDir` returns `home` | KILLED (arms 1, 3 — the behind-a-link and reach-row arms) |
| ayubz | `cageBeadsDirIn` returns `absResolve(home)` (the bead's own proposed fix) | KILLED (arms 2, 4) |
| ayubz | `cageBeadsDirIn` returns `over.Src` | KILLED (arm 4) — see the correction below |
| ayubz | `planLaunch` drops / widens / absResolves the `caged` branch | KILLED (arm 5; the widening also reds an arm-3 pin) |
| evx07 | the criterion over real `*exec.ExitError` values | KILLED |

Three mutants of my own, all killed, so the parts no one had mutated are
witnessed too: asking `cageCovering` with the RAW home (killed, 3 arms);
dropping the covering bind's `Join(Dst, rel)` extension (killed, all 5 arms);
and the error classification dropping `ENOTDIR`, which the `errRefusalParent`
sentinel catches in 42 rows.

## the four arms nothing witnessed

Each was MEASURED by patching exactly one declaration and reading what red.
The first three are pinned in this landing; each new pin was then graded
against the mutant it was written for, and **the fourth is left unpinned on
purpose, as the control that the three are narrow**.

| arm | before | pin added | after |
|---|---|---|---|
| the abstention WIRING in both seat runners (evx07) | SURVIVED `./internal/posse` arm 1 **unfiltered** and `make treepins`, both green | the account census, per runner | KILLED, each runner graded separately |
| `openRefusalLeaf`'s not-below-root containment (byqv0) | SURVIVED | `TestOpenRefusalLeafRefusesItsCallersTwoMistakes` | KILLED |
| its `O_TRUNC` refusal, un-combined (byqv0) | SURVIVED | the same pin's second arm | KILLED |
| `CageBeadsDir`'s LoadEngine-error fallback (ayubz) | SURVIVED | none | SURVIVED |

The wiring one is the one worth reading. `bgSeatEndedFromOutside` decides
whether a member REDS or ABSTAINS, and its own pin calls the function
DIRECTLY — so that pin grades the criterion and says nothing about whether
the two runners ask it. Neither can any live run: the branch is reached only
by a real silent supervisor kill, which no fixture manufactures inside a
runner. Deleting both call sites is the pre-fix behaviour, and it left a full
unfiltered arm 1 green. A filtered sweep could not have licensed that
sentence, which is why the reading was taken unfiltered.

`CageBeadsDir`'s other fallback — `""` from `cageBeadsDirIn`, meaning no bind
covers the store — IS pinned now, and it is the load-bearing one: an empty
`BEADS_DIR` is forwarded by `-e BEADS_DIR`, and a `--no-db` bd with nothing
there resolves `$cwd/.beads` and writes the container's throwaway
`issues.jsonl`, which is the silent record loss ADR 0055 D1 exists to prevent
and ranger-base-ayubz's whole subject. Arm 5 already carries the assertion
that would catch it (`if got == ""`), but its fixture's store IS covered, so
that branch was never reached.

## zbmxc's plantable shapes, re-run rather than read

The notes fragment for that bead claims all 63 arms of its five touched files
are green natively, under `TMPDIR=<a link>`, under `TMPDIR=/var/tmp/<dir>`,
and under a TMPDIR that is both at once. Re-measured with the filter read off
the five files rather than typed:

```
native                                     63 pass  0 fail  0 skip
TMPDIR behind a symlink                    63 pass  0 fail  0 skip
TMPDIR granted but not /tmp                63 pass  0 fail  0 skip
both at once (darwin's actual shape)       63 pass  0 fail  0 skip
```

0 SKIP matters as much as 0 FAIL: a planted shape that quietly fired an
escape-hatch skip would read like a pass in a suite summary.

The RED-BEFORE half reproduces too. With `upgrade_test.go` reverted to `$PWD`
against the raw `t.TempDir()` and `TMPDIR` behind a link, the message is the
macos log's own shape — `want cwd=<link>/…, got cwd=<real>/…`. And the venue
row, driven the other way with `c.wallMechFn` answering `sandbox-exec`, reads
`rendered 1 cage(s), want 2` over a judge that answered GREEN, with the kept
`strings.Builder` now printing the one line that says why:

```
· …: wall check not run — this host realizes cage seatbelt with sandbox-exec,
  and ADR 0097 D1's venue is a bwrap (ADR 0096: no darwin change), so the step
  does not exist here
```

That is both halves of that row's fix in one reading: the seam, and the
writer that was being thrown away.

Each of the five rows was also re-graded for TEETH — a production mutant per
row, since four of the five fixes were fixture-side and a darwin-tolerant
fixture could have bought its green by giving up discrimination. None did:
the build cwd, the wall-root containment, the codex refusal's rendered token,
D19's config-file drop and the venue's cage kind each still red when the
production behaviour behind them is broken. The one mutant that survived is
`SeatWallRoot` composed from the raw home, which is the identity on linux and
is exactly the darwin-only arm the close recorded as unwitnessed.

## what the consumers actually do with an abstention

evx07 rests on "a member that abstains is exit 3, which ADR 0097 D5 routes to
the watchdog". Read at the consumer rather than taken from the close:
`scripts/posse-wall-check` collects `skip` members and answers exit 3 with
"An abstention is never a pass (D5)", and it carries its own self-test arm for
exactly that case (`'abstention: a member SKIPped after the marker' 0 3`),
which is a prerequisite of `make test`. ADR 0097 D5 and its Consequences agree
in terms. Every member that reaches `walled` also calls `bgLiveSkip` first, so
D4's marker is always printed before the abstention — which is what makes the
check count the member at all rather than reading it as a filter miss.

The committed probe runs, and arm D re-reads the bead's central kernel fact
unchanged on a second day: `exec` of a self-killing script → 137, no-exec with
the killed child last → 137, a shell that carries on → **7, the kill masked
entirely**, git at `cannot lock ref` → 128. Nothing printed on any of them.

## corrections to the record

- **ayubz's mutation table reads as though a mutant shipped unkilled.** Its
  two `return over.Src` rows are ONE mutant graded by the entry point each arm
  reaches it through, and the paragraph below the table says so — but the row
  labelled **SURVIVED, equivalent** names `CageBeadsDir` as its site, and
  `CageBeadsDir` has no `over` to return. Measured as one mutant it is KILLED,
  by the arm that asks `cageBeadsDirIn` over a hand-written mount set. The
  grading is right and the rendering invites the wrong reading.
- **evx07 overclaims the ADR 0086 D5 discharge.** Its code comment says the
  wiring "DISCHARGES ADR 0086 D5's OBLIGATION, which ADR 0097's own
  Consequences records as OPEN". That obligation is "a pin whose own exec
  fails for some OTHER reason still reads as a refusal", and the criterion is
  deliberately narrow — silence AND a supervisor's signal. A bwrap that fails
  for any other reason still leaves `walled` answering `ok == false`, which
  the three refusal arms (`if ok { fail }`) read as the wall refusing. One
  reason of many is discharged; the obligation stands, and ADR 0097's text
  still states it correctly.
- **ADR 0097 says the member set is 32.** The set is **55** today, in 24
  arm-1 files, read with the check's own `member_pattern`. The bead itself
  quotes 48 from the day before. The count drifts with every landing, which is
  why no number should sit in prose that nothing derives.

## what the next reader inherits

ranger-base-ayubz asked for a before/after on a **live L4 seat** with a store
behind a symlink. Its close declined that honestly, and this verification
cannot meet it either: `docker info` from a crew seat answers `permission
denied while trying to connect to the docker API at
unix:///var/run/docker.sock`, rc=1, against a socket owned by a group this uid
is not in. (A first reading here said rc=0 and was wrong: the zero was `head`'s
through a pipeline, which is the one failure mode ORDERS names for exactly
this shape. Read without the pipe it is 1.) Everything else about that bead is
held by the suite; that one row needs a box where the engine is reachable.

The abstention reaches 8 of the 55 members. The other 47, in 23 files, keep
their own seat runners and still read a silent outside kill as an answer about
the wall — which is the cost the bead was filed to stop. That is the one live
finding here and it is filed.
