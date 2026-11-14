# A fence that forbade the page from naming the fence (ranger-base-y0qvh)

`pushMentionFaults` in `internal/treepins/pushmandate_qa_test.go` faults every
line of AGENTS.md that names push at a position no letter precedes, unless the
line also carries one of four exempting spellings. Every identifier in that
file opens with the word — `pushmandate_qa_test.go`, `pushMentionFaults`,
`internal/treepins/pushmandate_qa_test.go` — so the one page that has to
document this pin was the one page the pin forbade from naming it, and the
workaround under a red suite is to drop the filename, which costs the reader
the pointer.

2026-11-14, this box (linux 7.2.15 x86_64, go1.27.3), branched at fcfcdbf6.

## The defect, reproduced

The scanner run verbatim over the nine lines the bead names, before the fix:

    let by  TestTestCorpusHidesNoCrewNameBehindAnEscape  no fast door yet
    FAULT   the four spellings are in `pushmandate_qa_test.go`'s own scanner
    FAULT   `make orientation-check` runs internal/treepins/pushmandate_qa_test.go
    FAULT   QA_ORIENTATION_PINS is the door; pushMentionFaults is the scanner
    let by  see TestRepoAgentsMdCarriesNoPushMandate for the AGENTS.md half
    FAULT   - Work is NOT complete until `git push` succeeds
    FAULT   you must push it yourself when the branch is ready
    let by  - **Never push.** The operator pushes.
    let by  every crew PID denies `Bash(git push:*)`

The two orders land and the four exempting spellings are let by, so the pin's
live job was intact. Three of four lines that merely NAME code faulted, and
the fourth was let by only by accident — the word sits mid-identifier in
`NoPushMandate`, where a letter precedes it.

This is ranger-base-rulbl's class one step out. That bead narrowed a substring
test to "no letter before the match" because the substring form fired on
`TestTestCorpusHidesNoCrewNameBehindAnEscape`, and its own comment states the
reason: "a fence that reds on correct documentation is one people learn to
route around." The narrowing fixed push BURIED in a longer word and left push
OPENING one.

## The discriminator, and the one that was rejected

Position cannot do it: `pushes`, `pushed` and `pushing` are all push followed
by letters and all have to keep landing. What separates them from
`pushMentionFaults` is that the latter is CODE. So the match is rejected when
the TOKEN around it carries a character English cannot — `_`, `/`, an interior
`.` — or a camelCase hump.

The bead listed a second candidate, "or it sits inside a backticked span", and
it is wrong for a reason worth keeping: the mandate's central line spells its
order in backticks (``Work is NOT complete until `git push` succeeds``), so
that reading lets the push mandate itself through. It is `mentionsPushBacktick
Exempt` in the file now, a mutant that group 1 kills.

Three definitions are load-bearing and each has a group-1 case:

- A hump is a LOWERCASE letter followed by an uppercase one, not any capital
  past position zero — otherwise `PUSH TO REMOTE`, the mandate's own shouting,
  is camelCase.
- The token's `.`, `/` and `_` are trimmed at the EDGES: a sentence's full
  stop is not a path's extension dot ("The operator pushes.") and markdown's
  emphasis underscore is not an identifier's ("you must _push_ it yourself").
  Both found attacking the widening, not writing it.
- The token question is asked of the MATCH, not of the line, so a page may
  name this pin's file and still be held to the mandate in the same sentence.

## What attacking it turned up

**A panic I was about to ship, and an order that was already escaping.** Both
rules read the line by byte offset at an index taken from the folded copy, and
`strings.ToLower` is not length-preserving. MEASURED over all of Unicode: 27
runes change byte width under it, 25 narrower and 2 wider.

- narrower slides the index left and the ORDER ESCAPES. `İpush it yourself`
  (U+0130) and `Kpush it yourself` (U+212A Kelvin) both read as faultless,
  because the byte the letter rule inspects is the `i`/`k` the fold produced
  rather than the line's own. That half is ranger-base-rulbl's, live since the
  letter rule landed.
- wider slides it right and the token reader walks off the end. `Ⱥpush`
  (U+023A) panicked: `slice bounds out of range [:7] with length 6`. That half
  is this bead's own, introduced by the token rule and never shipped.

A panic is the worse of the two — it takes the package BINARY down, so every
other test in the run is lost and the reader meant to name one line of
AGENTS.md names nothing (ranger-base-ietka's class, four days old).

Fixed with `asciiLowered`, an A-Z-only fold that is byte-for-byte as long as
its input. The needle is ASCII, so nothing is lost: no non-ASCII rune
lowercases to any of `p`, `u`, `s`, `h`.
`TestPushMandateScannerFoldIsByteLengthPreserving` holds all three — that
`asciiLowered` preserves length over every valid rune, that `strings.ToLower`
still does not (a floor of one each way, so the arm reds if it is ever pinning
a hazard that has gone away), and that each of the 27 in front of an order
faults, with the panic recovered so a failure names the rune instead of
killing the binary.

## The mutation check

The bead asked for a case in each group and a mutation check per group,
because ranger-base-3nyqf's finding was that nine arms passed a scanner with
the loop replaced by a single `strings.Index` — found by a verifier running
that mutant BY HAND. Running it by hand is what
`TestPushMandateScannerMutantsDieInTheirOwnGroup` stops being necessary. The
three groups are package-level tables now, each the named killer of at least
one wrong scanner, and each mutant shares everything with the real scanner but
the one decision it is named for:

| mutant | killed by | of |
|---|---|---|
| the pre-widening scanner (letter rule only) | 3 of 8 cases in the lines that must not fault | group 2 |
| a backticked-span exemption | 1 of 8 in the orders that must fault | group 1 |
| the token's edges left untrimmed | 2 of 8 in the orders that must fault | group 1 |
| the token question asked of the line | 1 of 3 in both-on-one-line | group 3 |
| no loop: one `strings.Index` (3nyqf's) | 3 of 3 in both-on-one-line | group 3 |

Each mutant's arm first asserts the REAL scanner's verdict on every line of
the group and `t.Fatal`s if it is wrong, so a mutant reading can never be an
artifact of a group that is already broken.

`TestPushMandateScannerReadsTheFileBdPlants` closes a gap the mutation work
exposed: the control arm has always read `bdPlantedAgentsMd` with
`readerDirectedPushOrders`, which matches six exact phrases, and NOTHING read
that fixture with the line scanner. A widening could have let the real mandate
through while every phrase arm stayed green, and the only reader of the line
scanner was seven lines somebody typed. Eight of the planted file's lines
fault, all eight reader-directed, pinned as a floor; the arm also asserts the
fixture carries none of the four exempting spellings, so the faults are the
scanner's judgement and not an accident of the skip list.

## Verified against the real file

The bead's repro, run against the tracked AGENTS.md and restored
(`git diff --no-ext-diff HEAD -- AGENTS.md` empty afterwards):

- nothing planted: PASS
- the four lines that merely NAME this pin's code: **PASS** (FAIL before)
- a line that names the file AND orders the reader
  (``` `pushmandate_qa_test.go` is the pin; push the branch yourself ```):
  **FAIL at `AGENTS.md:568`, naming the line** — the hole did not open

`make tree-check` green, 66.5s, at a one-minute load average of 7-9 with one
suite slot held by a sibling seat. `make fmt-check` green, 0.5s.
`go test ./internal/treepins -count=1 -run PushMandate` is 0.037-0.059s for
all five pins.

## What the scanner still cannot see

An order spelled ONLY as code — "run `scripts/push.sh` to finish" — is let by,
because by shape that is the same token as
`internal/treepins/pushmandate_qa_test.go` and nothing short of asking the
filesystem tells them apart. Two things cover it: `readerDirectedPushOrders`
matches bd's six exact phrases whatever the line scanner thinks, and INSTALL.md
§9's human `grep -n -i "push"` is wider than both on purpose. Stated in the
file so the next reader does not have to find it: the line scanner catches an
order written in English and says nothing about one written as a command.

## Naming

The three new pins are `TestPushMandateScanner…`, so the filter this file's
notes prescribe — `-run PushMandate`, which ranger-base-n7hsa measured at
0.089s — selects every pin in the file. A pin here whose name does not carry
that substring is a pin nothing a seat types will reach, which is n7hsa's
finding one level in. Whoever lands that bead's `make orientation-check` door
should name all five.

## One pointer swept

`internal/posse/pushmandate_qa_test.go`'s head comment sent a reader to
`../../pushmandate_qa_test.go` for "the rest of the pins for cmfj". From
internal/posse that resolves to the repo root, where no such file exists — the
pins moved into internal/treepins and the pointer did not follow. Comment
only, so nothing a process does and no pin's subject changed; corrected in
place while its target was open, and recorded here rather than filed.

## Not closed here

ranger-base-n7hsa — the door. Its AGENTS.md bullet works around this defect by
reciting the four exempting spellings instead of naming the file that holds
them; that workaround is no longer needed and the bullet is free to name the
file, `make orientation-check` and `pushMentionFaults` directly.
