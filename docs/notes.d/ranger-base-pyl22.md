## The sixth spelling of a walk root: one hop at the call site, not a third hand register (ranger-base-pyl22)

2026-10-31

`internal/treepins`' half of the tree-wide door class is derived from a
directory ENUMERATION rooted at a relative tree path (ranger-base-g6sb1,
widened by ranger-base-8wxbn). ranger-base-0ckgm censused six spellings of
that root; 8wxbn taught the derivation four of them and doored the twelve
pins behind them, leaving one live: a root that arrives as a PARAMETER.

Five tree-wide pins sat behind it, over two helpers:

```
TestOneCredentialAcquirerAndNoOther       arParse(t, ".", arRoots)
TestNoNewShadowPredicate                  arParse(t, ".", arRoots)
TestNoBdCloseVerbReachableFromDispatch    arParse(t, ".", arRoots)
TestPaneModeReadingDecidesNothing         arParse(t, ".", arRoots)
TestNoShippedScriptAcquiresACredential    cgScan(t, ".", cgScannedDirs...)
```

`arParse` walks every non-test `.go` under `internal/` and `cmd/`; `cgScan`
walks every runnable file under `scripts/`, `plugin/`, `etc/` and `docs/`.
Both are `filepath.Join(root, d)` over a package-level list of literals, and
both are genuinely polymorphic — eight more tests hand `arParse` a fixture
root — so neither root could be respelled at the walk the way
`scanRealBdCallSites`' was. Their only route was an unfiltered 589.965s run
of the package.

### The decision, which is the whole of this bead

The bead was filed with two options and the instruction to put the refusal on
the record rather than work around it.

**(a) a hand register with reasons and a door**, the shape `twdIndirect` and
`twdDoorHolders` already use. Rejected, and the reason is not taste: neither
of those registers' rationales reaches these five. `twdIndirect` is for an
enumeration **no rule over Go source can derive** — and one derives this
spelling today, one file over, in `rootparam_qa_test.go`'s `rpScan`, under a
fixture arm, and has since ranger-base-0ckgm landed that morning.
`twdDoorHolders` is for the arms that hold the register itself open, which
these are not. So (a) meant a THIRD hand list, with a new rationale, for pins
a shipped rule already finds — a second thing to keep in sync by hand, in the
one file whose head comment warns about exactly that parking spot.

**(b) read the callee's walked PARAMETER POSITION and resolve the argument at
the call site.** Taken. The refusal it runs into is `twdTestFiles`' and
`twdRelEnv`', and read precisely, it does not cover this: what those refuse
is **interprocedural constant propagation** — a value followed across
boundaries. One argument resolved at one call site is not propagation, and
the package had already shipped that analysis, with a fixture arm, when the
refusal was quoted against it. Five pins sat undoored in the gap between what
the refusal said and what it was read to say.

### What landed

`twdParamHops` (`treewidedoor_qa_test.go`) builds the walker table by calling
`rpParamRootedWalkers` — the loop extracted out of `rpScan`, so there is ONE
implementation of "a walk rooted at one of this function's own parameters"
and the pin and the derivation cannot drift. `twdAnalyze` then resolves each
call site's argument at that parameter's own index with `twdRelTreePaths`, and
records the hop in the caller's `relEnum` as `Walk(.)@arParse`. That feeds the
existing seed and the existing reach closure — the same question ("does this
body enumerate the tree"), so not a second closure.

Entirely intraprocedural plus one hop; a CHAIN (`outer(root)` handing its own
parameter to `inner`, with the literal two call sites away) is still unread,
by the hop and by `rpScan`, identically. That one would be the propagation.

`$(QA_CORPUS_PINS)` doors all five: four walk `internal/` and `cmd/`, which is
that door's subject exactly, and the fifth is there rather than behind
`scripts-check` because that door's subject is `scripts/` alone while `cgScan`
reads four shipped directories — and because its own file's other pin
(`TestKeychainReadIsExecdFromExactlyOneFile`) is already there. One file, one
door. 0.43s between the five.

### What happened to ranger-base-0ckgm's pin

It was a PROHIBITION — a param-rooted walker may not be handed a tree path —
whose stated reason was that the derivation cannot see such a pair. The
derivation sees it now, so the reason is gone and the pin had to be
dispositioned rather than left asserting something false.

It is a COUPLING now, renamed
`TestQAEveryTreePathHandedThroughAParameterIsDerived`: every pair `rpScan`
finds must be a pair the hop READ, as evidence in the caller's `relEnum`.
`rpScan` resolves the call-site argument with `twdRelTreePath` and the hop
with `twdRelTreePaths`, a superset today, so the live answer is that all six
handoffs are read — and the day somebody narrows the hop (to one walker, to
argument zero, to a bare identifier) this reds with the pair it dropped
instead of five pins quietly leaving the class the way they quietly sat
outside it. Both floors are explicit: no param-rooted walk at all is a rule
matching nothing, and no handoff at all is a coupling comparing two empty
sets.

`rpWaived` is dropped. **This is the correction ranger-base-8wxbn's own
description owed `ranger-base-0ckgm`**: 0ckgm predicted those two rows would
come out when 8wxbn landed, 8wxbn landed with both helpers still
parameter-rooted and said so on its bead, and dropping them waited on this
bead. It is earned now — not because the pairs are gone (all six are still
there) but because the hole they recorded is closed.

### Numbers

MEASURED 2026-10-31, this worktree, go 1.26.10, darwin.

- The derivation: 43 tree-wide tests in `internal/treepins` before the hop
  (42 derived + 1 registered indirect), 48 after (47 + 1). The five are
  exactly the delta — arm 2 named those five and nothing else.
- The enumeration rule alone: 42 tree-wide tests with the hop, 35 without,
  23 on 2026-10-04.
- `rpScan` over the package: 6 param-rooted walks, 6 handoffs (`arParse` at
  four call sites, `cgScan` at two).
- The five pins: 0.07s, 0.07s, 0.08s, 0.06s and 0.15s — 0.43s together.
- `make tree-check`: **74.5-172.1s** warm over three runs at eighty-one pins
  and fourteen doors — 172.1s, 164.7s and 74.5s in that order, at a
  one-minute load average of 57.34, 57.89 and 94.79 before the three, with
  one suite slot held by a sibling seat for the first two and BOTH held for
  the third. That is a reading of THIS BRANCH's class: `ranger-base-bm6j8`
  doored two more pins on `main` while it was being taken, so the class these
  five landed into is eighty-three and the live reading is the merge-back's
  (`docs/notes.d/ranger-base-3e68k.md`, 81-89s).

That last reading is the widest this command has produced and it is ordered
by nothing in the load: the FASTEST run is the one at the highest load
average and with both suite slots gone. The same command read 57.0-57.7s
three hours earlier at a load of 10-31 (ranger-base-v3tt4), over a class five
pins smaller that cost 0.43s. So the spread is the box, for the fourth wide
reading in a row — price the command from the quiet end of these ranges and
read the high end as what it costs beside other people's suites.

### Files

- `internal/treepins/treewidedoor_qa_test.go` — `twdParamHops`,
  `twdHopEntry`, the hop in `twdAnalyze`, eight fixture cases (one of them
  flipped from `false` to `true`, which is the deliberate change), the head
  comment's five rows and re-measured count sentence.
- `internal/treepins/rootparam_qa_test.go` — `rpDecls` and
  `rpParamRootedWalkers` split out of `rpScan`, `walk` on `rpHandoff`,
  `rpWaived` dropped, the live arm turned into the coupling and renamed.
- `internal/treepins/bddaemonleak_qa_test.go` — the comment on
  `scanBdCallSitesUnder` named the old pin and the old claim.
- `Makefile` — five names into `$(QA_CORPUS_PINS)`, the rename in
  `$(QA_REGISTER_PINS)`, `corpus-check`'s comment.
- `AGENTS.md`, `internal/treepins/README.md` — the re-measured reading and
  the door's description.
