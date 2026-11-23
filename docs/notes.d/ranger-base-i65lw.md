# ranger-base-i65lw — verifying four closes, and the conjunct census that found the gap

Batched verify of ranger-base-q5fee, ranger-base-4uiwo, ranger-base-vyz61 and
ranger-base-4yefx. Everything below was measured on this box (linux
7.2.15-1-omarchy, go1.27.3, 16 cores) on 2026-11-23, at `dceb3642`, which was
`main` and this branch's base throughout.

All three closes whose commits were named by a pre-rebase sha landed
byte-identically: `git patch-id --stable` is equal across each pair
(579ac3c8/ead3fe9a, f840f1bc/204ba900, 2b82445e/d13842d4).

## ranger-base-4yefx — the conjunct census, and the two clauses nothing read

`mayReapWorkingSeat` (internal/posse/reapworking.go) is a ten-clause
conjunction in front of a sweep that KILLS SESSIONS. Each clause was
neutralised separately (`&& false` on its own line) and graded against every
reap, landing and kill test in arm 2 (`-run
'AutoReap|Reap|reap|Landing|Land|Kill'`, control green):

| clause | line | verdict |
|---|---|---|
| `class == reapFleetClosed` | 132 | survived — duplicate of the sweep's pre-filter, which IS pinned |
| `status == "working"` | 143 | survived — same pre-filter |
| the run record is readable | 147 | survived — fail-safe; its drop is a nil meta, not a wrong reap |
| the screen named background work | 151 | killed, two arms |
| no turn in flight | 168 | killed |
| the composer is empty | 177 | killed |
| the dial is armed | 192 | killed |
| the record is dated | 198 | survived — subsumed by the grace below it; the drop changes only which line prints |
| the grace has elapsed | 203 | killed |
| `residueHolds` is empty | 208 | **SURVIVED, and neither duplicated nor subsumed** |
| widen `workingSeatWorthAStoreRead` to `true` | — | killed (the pre-gate arm) |

The residue clause is the only residue question this population is ever
asked: `reapWhy`'s general grace-and-residue block is guarded by `if class !=
reapFleetClosed` (internal/posse/autoreap.go), so a `working` seat over a
closed bead never reaches it. Dropping it does not lose the work — the reap
lands the plane — but it makes the arm a reap that DECIDES, which is the one
thing `residueHolds`' own doc says the widened arms may not do.

The same shape one layer out, in ask 2's renderer: neutralising
`landRefusalKillLines`' `a kill WOULD take something` branch
(internal/posse/landrefusal.go:213) reds NOTHING in that whole set, while the
clean branch beside it is read by the route arm. That is the branch standing
between a refusal over a checkout holding work and the sentence `a kill would
take nothing uncommitted … so --no-land skips only the agent's own wrap-up`.

Both gaps are closed by `internal/posse/verify_i65lw_qa_test.go`, two arms,
each mutation-checked: with its clause neutralised each is the ONLY red in
that set, and the reap arm also reds at its own in-pass control when the arm
is made to refuse everything (so the control is live rather than decorative).

One fixture fact the new arm had to find: `residueSession`'s `bd show` answer
is a FILE in the session's own dir, so in a real git checkout the fixture makes
its own tree dirty and both halves of a held/clean pair read as holding work.
Commit it. This is why the residue arms next door all pass an empty bead
status — this is the first arm in the tree that needs a CLOSED bead and a git
tree at once.

The rest of the close verified as claimed: `LiveTurn` is set on BOTH reading
routes (panework.go:426 the explain previews, :576 the rex capture) and each
route's pin kills its own mutant, one arm each; `off` parses to 0 through
`graceAfter` and keeps the line; `reap_working_after:` is on every surface the
close names (INSTALL.md, CHANGELOG.md, `posse help`, examples/config.yaml, the
reap table); the dated correction beside `capturedHold`'s own 2026-11-10
measurement landed.

## ranger-base-4uiwo — the flake reproduced on demand, and the fix measured against it

The close's diagnosis (a sibling fork inherits a duplicate of the write
descriptor, and O_CLOEXEC shuts it only at `execve`) was re-measured from
scratch with a probe that runs the fold over a planted FIFO and peeks exactly
where the helper peeks, 400 rounds per arm:

| arm | helper | fork traffic | `leafPipeOpen` seen | result |
|---|---|---|---|---|
| A | the landed one (settle 10s) | 8 goroutines exec'ing `/bin/true` | 46/400 | PASS, 0 failing rounds, 0 bytes |
| B | the one it replaced (settle 0) | the same | 3 of the first 19 | FAIL — the incident's own message |
| C | the one it replaced (settle 0) | none | 0/400 | PASS |

So the original red reproduces on demand in seconds (B), the cause is the
sibling traffic and not the fold (B vs C), and the landed helper absorbed 46
real occurrences without answering for any of them (A) — non-vacuous, which
is the half a green suite cannot show. Six mutants of the helper were graded
and every row of the close's own table reproduced, including the two that used
to be "this pin ALONE" and are now a pair, and the `default` arm a panic shows
is reached by nothing.

The figure the close deliberately did not move —
`internal/treepins/refusaltrail_qa_test.go`'s `refusalleaf_test.go (8)` — is
right under either derivation: the file now holds 11 tests of which 3 are the
`leafReachedThePlanter` pins, and that pin's own head comment says test
sources are not read, so the figure is prose asserted by nothing.

## ranger-base-vyz61 — re-derived rather than read

At `dceb3642`, from arm 6's own `Logf`: **66** account messages graded, **9**
rendered inside a `t.Helper()`, the decision charging **44**. Under the
hermetic mutant the head comment now prescribes (drop `wmaFuncSpans`' FuncLit
span and its FuncLit bail) the sloppier reading is **10 of 66**, adding
`wallwatchlive_test.go:614` and nothing else — adds one, drops none — with arm
6 GREEN under the mutant and arm 7 red. The nine charge 27 (five), 30 (two)
and 41 (two) at their own positions, derived from the basenames and line
numbers arm 6 prints, so the range 27–41 is unchanged and the parameter
route's own message (the narrowest head, 62) charges 30, inside it. `:614` is
typed in exactly one place in the tree.

## ranger-base-q5fee — five mutants, five killed

Control green; each killed by the intended arm: the readiness predicate given
the settle gate's state half (arm 7 + arm 3a); the door fired on any explain
error (arm 8, by its call census); `muxVerbMissing` by `err != nil` instead of
`errors.As`, which is the mutant aimed at arm 9's own subject (arm 9 + arm 8);
the `muxSeen` arm removed from `AwaitPromptable` (arm 3a); and removed from
`awaitDelivered` (arm 5a). Arm 9 stays green under the door mutant and that is
not decoration — the herdr blast radius is pinned by arm 8's no-call census,
which reads the error and not the occupant.

Recorded, not filed: `promptready.go:290` and `:308`. The note the gate
renders when it opens on the occupant's reading after more than one poll reads
`waited 300ms for <session> for its multiplexer to report a runtime holding
the keyboard (…)` — a doubled preposition, because the other two `waitedFor`
branches are infinitive phrases that complete `waited X for <session> ___` and
this one is not. Rendered for real, twice, by a probe driving the first poll
onto a record the block's own shell wrote and the second onto the runtime's.
Neither new branch of `waitedFor`/`seenBy` is read by any test. It decides
nothing and no pin is wrong about it, so it is one line on the verify close
rather than a bead.
