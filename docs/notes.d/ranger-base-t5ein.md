# Promoting codex's record, and the fixtures that were pinning a table row (ranger-base-t5ein)

2026-11-07. `Runtime.Record` for codex is `RecordTrusted` now, with a
`RecordWhy` naming the measurement. The decision is one line of the built-in
table (`internal/posse/runtime.go`); everything else in this bead is what else
had been written against the OLD value without saying so.

## 1. The measurement, read off the store and not off a pass

ADR 0076 Amendment 2's live acceptance (ranger-base-8gh58 §7) spent the two
codex turns the operator authorized in ranger-base-x7ojx, one per
multiplexer, on two small REAL beads. Read back here before deciding
anything:

| | control | acceptance |
|---|---|---|
| bead | ranger-base-w4enh | ranger-base-7vh7y |
| instance | `8gh58h`, no `multiplexer:` key | `8gh58r`, `multiplexer: rex` |
| seat | `codexherdr-posse-ranger-base-w4enh` | `codexrex-posse-ranger-base-7vh7y` |
| `bd show` | CLOSED, comment by the seat | CLOSED, comment by the seat |
| commit on `main` | `3bc1a6db` (rebased from `3396bc02`) | `b14420ef` (fast-forward) |

Both comments name what changed and how it was verified — the census seat
reads out four immune files and three guarded ones, the comment seat names
four corrected tokens and `make tree-check` green. Both commits are ancestors
of `main` (`git merge-base --is-ancestor`), and `git log --grep` finds each
under its own bead id. codex-cli 0.153.4, tier standard. Neither seat needed a
second leg.

## 2. Why two closes on one day is enough here

Not because two is a large *n*. Three reasons, in the order they decided it:

1. **The comparison is against what the other two promotions rest on.** grok
   is `record: trusted` on ONE dispatched close (the qa lane, 2026-08-24);
   claude's is the baseline the contract was written from and names no count
   at all. A rule that admits grok on one and refuses codex on two is not a
   rule about evidence.
2. **The two differ in the dimension that could have faked it.** Same
   runtime, same version, same repo, PIDs byte-identical but for the comment
   naming which is which — and two different multiplexers. A rig artifact
   would have to be present in both halves of the one comparison the
   acceptance was built to make.
3. **The reading it replaces has a known harness component.** 2026-08-24:
   3/3 dispatched codex sessions did the work and left the bead
   `in_progress` with no comment (ranger-base-0fb), codex 0.147.0. That
   reading is not retracted — it was true of that day — but the silence
   class 0fb names also includes five consecutive sessions whose `bd close`
   was DENIED, because a self-sandboxing runtime was never told the store of
   record was writable (`launchWritableRoots`, and the reachability half of
   ADR 0013 §4 that ranger-base-hxhb added for it). That half is MEASURED.
   `prompt: argv` delivery had not landed either, which is a difference
   between the two days and not a cause anyone measured. What 2026-11-07 has
   that 2026-08-24 lacked is a seat that could reach the store at all.

**The caution the bead asked for is in `RecordWhy` itself**, not only here:
both PIDs carried a deny list and a `bd close` instruction in the body, so
what was measured is a runtime that acts on the contract it is handed. That
is the same thing claude's and grok's trust was measured on, and a reader of
the grid should not have to assume it.

## 3. What keyed off the old value, and what it was really keying off

`RecordTrust()` has one consumer that branches (`recordClause` in
`dispatch.go`, and the ✓ suppression through it) and one that displays
(`recordRow` in `runtimecheck.go`). Both are name-free and needed no change.
Four things named codex instead:

- `recordskip_qa_test.go` — the ADR 0013 §4 pins. Its fixture helper was
  `codexPersona`, "a PID on the runtime that is `record: untrusted` for a
  measured reason", and one arm asserted the clause `codex is record:
  untrusted` by name. So three of the four §4 pins were pinning a ROW OF THE
  BUILT-IN TABLE, and the promotion would have reddened them over a
  declaration that had not changed.
- `turnoutcome_qa_test.go`'s untrusted-and-blind arm — same shape, and the
  mirror image of a lesson the same file had already learned once: its
  trusted-and-blind arm says in its own comment that grok stopped being that
  fixture the day it got a reader (ranger-base-fc8go), so it was rewritten
  over a template runtime and "survives codex getting a reader too". It did
  not survive codex getting a RECORD.
- `runtimecheck_test.go`'s built-in contract table — the one place the change
  belongs, and its comment said so: *"If this test ever has to change, a
  measurement changed."* It changed.
- prose in `dispatch.go`, `runtimewalk_live_test.go`, ADR 0013 §4, ADR 0017
  and `notes-personas.md`, all of which offered codex as the example of a
  declared degrade.

Both test fixtures are now a template-only `runtimes/unmeasured.yaml` that
declares nothing but `command:`, which is `record: untrusted` by default
(ADR 0013 §4). That is not a convenience: it is the only shape that can
still print the clause at all.

## 4. The consequence nobody had to decide: no built-in wears this clause now

claude, codex and grok are `record: trusted`. bob is `record: untrusted` for
the absence of a measurement rather than one that went the other way — no bob
session has been DISPATCHED at all, because its launch row refuses — so the
degrade branch has no dispatchable built-in occupant. Its live occupant is a
runtime you DECLARE, which is exactly the population ADR 0013 §4 wrote the
default for. The branch is live, pinned and reachable; what changed is that
you now have to declare a runtime to reach it.

## 5. Verified

`make fmt-check`, `make tree-check`, the four §4 pins and the arm-2
turn-outcome pins by name, and `TestBuiltinContractDeclarations`. Readings
and commands are on the bead.
