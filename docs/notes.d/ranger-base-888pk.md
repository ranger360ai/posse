# The ledger read $0 for every dispatched seat, because the CLI wrapped the prompt (ranger-base-888pk)

2026-11-11. `posse cost` printed **no bead rows at all** on this instance and
a day total of zero under armed caps, while its interactive line carried every
turn the fleet had taken. The plan guard had been blind 26m+ on 429s at the
same moment, so ADR 0018 §1's degrade — on-meter beads keep launching *under
the ledger brake* — was running with a brake that measured nothing. The only
live limit on spend was the launch cap. (The figures are instance-ops content
under ADR 0024 D1 and stay in the private record: ranger-base-888pk.)

## The cause is one anchor and one wrapper

posse hands a dispatched session its work prompt as a positional argv
argument (ADR 0013 §2, `argvprompt.go`). What the CLI then WRITES into its
transcript is the CLI's business. claude 2.1.322 wraps a multi-line argv
prompt before recording it, and the user record's text reads

```
"\n\n<pasted_content id=\"f384\">\nWork beads issue ranger-base-888pk (…"
```

against a segmenter anchored at `^Work beads issue (\S+?)[: (]`.

**MEASURED 2026-11-11, Rex host, every transcript under `~/.claude/projects`**
(142 project dirs, oldest record 2026-11-08T12:38Z): 136 dispatched sessions,
**136 wrapped, 0 bare**. The prefix is byte-identical in all 136 — two
newlines, then the opening tag alone on its line; the id is the only part
that varies. Not a tail: every dispatched seat on the box, from the first
record the box retains.

## Why nothing looked broken

The failure is a MIS-count, not an undercount, and that is the whole reason it
survived 62 closes in a day. With no segment cut, each seat's first turn fell
through `ScanTranscript`'s interactive fallback — which skips text starting
with `<`, and the wrapped text starts with `\n\n` — so the seat's entire
session was attributed to `interactive`, a line the report prints with "(not
gated — shown so the ratio is visible)". The dollars were all there, in the
one row nothing gates on. A total that had gone to zero would have been
noticed; a total that moved one row over was not.

ADR 0018 §3 anticipated the adjacent failure exactly — "an unreadable
transcript root today reads as $0 spent" — and hardened the locator against
it. The locator was fine. Every file was read, every token was counted, and
the arithmetic put the result where no brake looks.

## The fix, and what it is allowed to relax

One shared predicate, `BeadFromWorkPrompt` (cost.go), replacing five direct
applications of the raw regex: the claude, grok and codex cost scanners, the
turn-failure reader (claude and grok) and herdr's `NoteBeadFromPrompt`
session→bead pointer. It skips leading whitespace, then up to four lines that
are **nothing but an opening tag**, then applies the same anchored match.

The strictness is the safety argument. A wrapper line carries no prose, so
skipping one cannot move the header off the start of a line, and the anchor
keeps meaning what it meant: a bead id named mid-sentence is a mention, and
cutting a segment on one would bill a quoted bead for this session's spend.
Four refusals are pinned for that — a mention mid-sentence, prose before the
header on the wrapper's own line, the header quoted inside a wrapped paste,
and a *closing* tag, which is not a wrapper.

## The check that says the fix is a re-attribution

Run the report over the same transcripts in the same minute, before and after.
**Conservation is the test.** Total turns must be identical, and the total must
agree to within per-row rounding, while the bead side goes from nothing to a
row per dispatched session and the interactive side falls to what a human
actually typed. That held here, measured both ways on the same files.

Keep that check, not the figures: **a cost fix that moves the total is
inventing or dropping spend.** This one moves only the attribution. The
figures themselves — the two sides, the day total against the armed caps, the
caps — are instance-ops content under ADR 0024 D1 and are recorded in
ranger-base-888pk's comments, in the private store. Per-pass spend is measured
live by `posse dispatch` and is unaffected either way.

## What rots this again, twice

The wrapper belongs to the CLI, so it can change under an upgrade with no
commit in this tree — one pin holds the shape measured today. And the header
is read by five call sites across three runtimes, so the next reader added can
re-break one while four stay green — a second pin (an arm-2 pkgcensus, with
its Makefile door) refuses any application of `workPromptRe` outside the
helper. Both are in `internal/posse/workpromptwrapper_qa_test.go`;
mutation-checked 3/3 (drop the wrapper skip, drop the anchor, add a bypassing
reader).

## Left open

The 429s themselves are ranger-base-zs686 (filed from here, `-l devops`). The read
log says this is not a storm: **MEASURED over `~/.cache/posse/plan-usage`'s
read log, 742 reads 2026-11-08T05:07Z → 2026-11-11T18:24Z, 13 of them 429
(1.8%)** — dispatch 494, cockpit 214, pulse 29, cost 4, status 1 — with the
2026-11-11 16:27–17:11Z spell the only one that went past a single retry
(backoff 2m→5m→10m→20m, recovered on its own). What is plainly wasted is
**48 same-second duplicate reads**, two callers asking the endpoint the same
question in the same second; cockpit's median gap is 300s and its minimum is
0s. That is the cadence question, and it is not this bead's.

The suite red this bead landed beside is ranger-base-zibko: `TestPersonaLaunchRuntime`
fails in the whole-package run of `internal/posse` and passes under a `-run`
filter, reproduced in a fresh clone of main at b9129561 with nothing of this
bead's in the tree. Its own pins and `make tree-check` are green.
