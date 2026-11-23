# ranger-base-thrn2 — the line rule read the format's own newlines, and the newline can arrive inside an argument

Landed 2026-11-23, linux 7.2.15-1-omarchy, go1.27.3, 16 cores, at tip
`ebfc4869`, one-minute load average 1.7-3.4. Found by `ranger-base-whpuk`'s
verify of `ranger-base-momix`'s close.

`internal/treepins/wallmemberaccount_qa_test.go`'s arm 2 holds every wall-check
member message to "the seat's account is rendered before the first newline",
because `scripts/posse-wall-check`'s `evidence()` carries the first
`file.go:NN:` line and nothing after it (ADR 0097 D3) and the launcher puts
that one line on a P1 bead. `ranger-base-momix` made that rule POSITIONAL —
the account in front, the supporting prose behind, which is the shape D3
prescribes — and the rule it wrote read `strings.IndexAny(text, "\n\r")` over
the FORMAT STRING and nothing else:

```go
func wmaAccountBeforeNewline(text string, argIdx int) (before, readable bool, firstLine string) {
	i := strings.IndexAny(text, "\n\r")
	if i < 0 {
		return true, true, text   // <- 60 of the member set's 66 account messages
	}
	...
```

go test ends the line it prints at the first newline of the RENDERED message,
so an argument that brings its own ends it exactly as a literal `\n` does. A
`%s` over a value carrying a newline therefore puts the account on line 2, the
bead gets a symptom, a colon and nothing — `ranger-base-a688i`'s own shape, by
the one route the rule could not see — and the rule answered
`(before=true, readable=true)` without looking. Its doc comment made the
opposite claim in that place ("the whole of what reaches a bead", "firstLine is
what `evidence()` would carry"), which is what made this live rather than
recorded: a pin wrong about what it measures is live (ADR 0006 §6).

## The reading that decided the shape

Two probes at `ebfc4869`, neither committed and neither needed to re-run this
(`laurie`'s is `ranger-base-whpuk-preaccount-args-probe_test.go.txt`; mine
walked the same key and printed each pre-account argument's VERB beside its
expression):

| | |
|---|---|
| 66 | account messages the key reads |
| 60 | of them carry no literal newline — the branch above, returned unread |
| 29 | of those 60 render something before the account |
| 1 | of the 29 is the seat's own unfolded output: `bwrapseatstorelive_test.go:506`, `bsLive.in`'s raw middle result, under a `%q` |
| 38 | pre-account arguments in all: 23 `%s` over a case label, a digest slice, a fixture path or a wait token; 11 `%d` over an exit code; 4 `%q` |

So there was no dark member — and the one row in the keyed class passes on its
VERB, which is the whole discriminator: `%q` escapes a newline, `%s` carries
it.

## Both readings land, split by subject

`ranger-base-whpuk` filed it as a two-reading decision (widen the rule, or say
in the prose that the reach is narrower than the claim). Both are right, about
different classes, and this file's own doctrine says which is which.

**Widened, by SUBJECT.** The seat's UNFOLDED words are the one argument in this
census's subject that carry newlines by construction — a child's stdout and
stderr, which is what `wwOneLine` exists to fold. `wmaUnfoldedLocals` keys
them, two routes: a seat runner's non-last string results (`bsLive.in` returns
`(bool, string, string)` and its own comment says why — "the raw output stays,
because callers READ it") and a local bound from a `Cmd` waiter anywhere in the
file. Rendered in front of the account by a verb that does not escape them, the
message is held to the stricter rule. Nothing today pays for it, which is what
every widening of this census has cost: it closes the route the next one would
have escaped by.

**Priced as a FLOOR, for everything else.** A newline arriving inside any OTHER
argument is out of reach, and that is the newline-direction twin of arm 6's
"every interpolated verb counted as ZERO, so this is a FLOOR". Deciding whether
a `%s` over `c.name` or a fixture path can break a line means reading a runtime
value, and the file's ORDER WITHIN THE FIRST LINE paragraph already records the
house answer to that class: sufficiency needs each member's messages rendered
under its fixture, "a venue `ranger-base-uu303` records rather than files, so
the next attempt does not re-try a static rule". A static census grading those
38 would be judging someone else's prose by variable name — the register this
file refuses when it declines an exemption table.

The rule returns a named `wmaLineCause` — `wmaLineOK`, `wmaLineBehind`,
`wmaLineUnreadable`, `wmaLineUnfolded` — rather than the `(before, readable)`
pair it carried before, because arm 2 reports a different thing for each cause
and the first draft of this landing had the arm re-deriving "it was the
unfolded words" from the format text in its own `case` guard. That is a second
spelling of the rule's own reading, which is the register this file refuses
everywhere else; the cause is now decided once and the arm branches on it.
Arm 3's rows pin the cause, which is a strictly better witness than the two
booleans: they say *why*, and arm 2's branch is keyed on the same value.

Two deliberate over-reads, both in the floor's direction:

- `wmaVerbEscapes` holds only `%q`, `%x` and `%X`. `%d` over a `[]byte` is
  newline-free too and is left out, because a set reading "cannot carry a
  newline" per argument TYPE needs types this census does not have.
- the waiter route is **not** scoped to a seat-launching `FuncDecl`, because
  the file where this class has been found twice launches its seat from a
  CLOSURE and `wmaLaunchesSeat` reads `FuncDecl`s — which is
  `ranger-base-momix`'s own lesson. A fixture child's output is read as
  unfolded too, and that red is one this census is content to make: a `git`
  dump in front of an account ends the line just as a seat's words do.

## The battery, eight mutants one at a time plus the pair

`scripts` under my scratchpad, not committed: a golden copy, one exact-string
mutation asserted to apply exactly once, the four arms under a `-run` filter
(so no suite slot), then a restore verified sha256-identical. ARM2 is the line
census, ARM3 its hermetic rules, ARM6 the 300-character half, ARM7 arm 6's
readers.

| mutant | ARM2 | ARM3 | ARM6 | ARM7 | what it says |
|---|---|---|---|---|---|
| none | GREEN | GREEN | GREEN | GREEN | the tree as it stands |
| `:506`'s `%q` → `%s` | **RED** | GREEN | GREEN | GREEN | the widening REACHES the member set: the green is its reach, not an unreached key |
| the raw reading dropped from `wmaAccountBeforeNewline` | GREEN | **RED** | GREEN | GREEN | the pre-fix rule, and arm 3's new row is what catches it |
| the key's runner arm dropped | GREEN | **RED** | GREEN | GREEN | held hermetically; no member message witnesses the route's loss |
| the key's waiter arm dropped | GREEN | **RED** | GREEN | GREEN | the same, and the route has no member witness at all today |
| every verb reads as escaping | GREEN | **RED** | GREEN | GREEN | the discriminator gone one way |
| no verb reads as escaping | **RED** | **RED** | GREEN | GREEN | gone the other way, and ARM2 reds at `:506` UNMUTATED — the `%q`-against-`%s` reading is witnessed live |
| the account subtraction dropped | GREEN | GREEN | GREEN | GREEN | reds nowhere over this tree; arm 3's row is the whole of it |
| the two wrappers no longer read through | GREEN | **RED** | GREEN | GREEN | `string(out)` / `strings.TrimSpace(…)`, neither of which removes an interior newline |
| **the pair**: `:506` unquoted WITH the raw reading dropped | GREEN | **RED** | GREEN | GREEN | the state `ranger-base-whpuk` read: the account dropped in transit and arm 2 saying nothing |

The pair is the finding. ARM3's red in that row is this landing's own rows
refusing the removal — in the true pre-fix tree those rows did not exist, and
nothing anywhere was red.

`drop-subtraction` is the one mutant with no tree witness, and it is recorded
rather than fixed: no member file renders a name that is an account elsewhere
in it in front of another account, so the under-read (a name an account key
holds anywhere in a file is not read as unfolded output anywhere in it) is
arm 3's `bwrapgitrefslive`-shaped row and nothing else.

## What arms 2 and 6 print now

Unchanged everywhere the counts were measured before — 24 member files, 8
carrying an account, 66 messages read by four routes, 6 deferring prose, 66
graded against 300 characters, 9 rendered inside a `t.Helper()`, the widest
head still 209 at `bwrapgitrefslive_test.go:624` — plus arm 2's new clause,
which LISTS the set rather than counting it, for `ranger-base-yem73`'s reason:

```
1 render the seat's unfolded words in front of the account:
  [internal/posse/bwrapseatstorelive_test.go:506 out as %q (escapes=true)]
```

That list is the reading to take; no paragraph in the file is the record of
one.

## One thing about running a mutate-and-restore battery

Never pipe it into `head`. The battery's restore is the last thing it does per
mutant, and `head` exiting early SIGPIPEs the script between the mutation and
the restore — which left `drop-waiter-arm` in the tree here, and the next edit
landed on top of it. Redirect to a log file and read the file.
