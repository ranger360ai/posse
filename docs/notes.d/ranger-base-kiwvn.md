# The first full run under injection: the price, and the one class every red belonged to (ranger-base-kiwvn)

2026-11-10, Linux 7.2.15 x86_64, go1.27.3, 16 cores. ADR 0085 D4-D6's slice
2: run `make coldexec-check` unfiltered, triage every red, and put a number
on what the door costs. From ranger-base-gdzu5 (the design) and
ranger-base-pbo1q (the injector, whose fragment holds the mechanism).

**LANDED LATE, AFTER THE DOOR IT MEASURED WAS REPLACED** (ranger-base-mbkmz,
2026-11-11). This bead's commit would not replay onto main: ranger-base-97w45
had meanwhile landed the execve supervisor (`cmd/coldexec`), which is what
the triage below bought, and `make coldexec-check` runs that instead of the
content rewrite every number here was taken through. The fragment is left as
it was measured. What the replay changed is said on ranger-base-mbkmz: the
Makefile keeps the supervisor's recipe, so `POSSE_COLD_EXEC_REPORT` is set by
no door any more — the supervisor counts in the PARENT of `go test` and
nothing eats its report — and the pass-through rule and its counters survive
in internal/posse for a by-hand run of the rewrite until ranger-base-7zkxw
retires both.

**MEASURED AT 10d046f9, NOT AT MAIN'S TIP, and that is not a shortcut.** main
at a2327230 does not compile: `go build ./...` answers two errors in
internal/posse/panework.go and `go test -c` fails the same way for all three
arms, so the door could not start at all. Filed P1 as ranger-base-17puk with
the repro and the cause (a2327230 is a non-merge reland of ranger-base-k0hd6
onto 10d046f9, and ranger-base-r5131 had meanwhile added three users of the
shape k0hd6 renamed). 10d046f9 is the newest commit that builds — `go build
./...` clean, all three arms compile — and every number here is that tree.

The first runs were the Makefile recipe's own three lines rather than `make
coldexec-check`, for one reason: make stops at the first failing arm, and a
red arm 1 would have hidden arms 2 and 3 for the whole bead. Same commands,
same `scripts/test-times.sh` wrapper, same suite slots. The last run was the
door itself.

## The price

Package seconds as `go test` reports them, which is the honest number: the
suite-slot wait is outside it. One-minute loadavg before → after each arm.

| arm | uninjected | loadavg | injected, D=2.5s | loadavg | factor |
|---|---|---|---|---|---|
| 1 (`./internal/posse/...`) | 114.643s | 4.67 → 6.33 | 413.342s | 4.24 → 11.23 | 3.61x |
| 2 (`-tags posse_arm2`) | 65.424s | 6.33 → 11.06 | 177.847s | 11.23 → 7.66 | 2.72x |
| 3 (`-tags posse_arm3`) | 100.222s | 11.06 → 8.51 | 260.219s | 7.66 → 4.67 | 2.60x |
| **all three** | **280.289s** | | **851.408s** | | **3.04x** |

**+571.1s over the same three arms uninjected, with the box in the same 4-11
loadavg band for both phases.** Three arms take three of the box's two slots
in turn, so a busy box serializes them behind other suites exactly as `make
test` does. Price the door at a quarter of an hour of box.

**After the fix below the door is CHEAPER, because a pass-through is a sleep
not slept.** Run unfiltered as a seat types it, `make coldexec-check` came
back green twice: **790.097s** (402.710 + 132.211 + 255.176) at a loadavg of
7-26, and 813.953s (418.019 + 135.318 + 260.616) at 14-37 — both under the
851.4s above, which was measured at a quarter of that load. Different load
bands, so read the fall as a floor rather than as a delta. **Its counters
were identical across the two runs**, every one of the fifteen numbers, so
what the gate reaches is not a reading that moves with the box.

**And the delay does not add up the way a reader expects.** 443 first execs
were paid in that run against a 814s wall, where 443 x 2.5s is 1107s of
sleeping. The sleeps overlap: the plants are exec'd from tests that run in
parallel. Nobody should price this door as D x plants.

## Every red, with its kind

**28 red rows over 26 distinct tests, and all three uninjected arms were
green, so every red is the injector's doing and not a pre-existing failure.**
(`TestRenderClaudeSkillsKeepsTheExecuteBit` is untagged, so it appears in all
three arms: 26 tests, 28 rows.)

**Not one of them was IN CLASS.** No test in this package holds a deadline
over a cold first exec today — ranger-base-aru4c's two sites are fixed and
nothing since has added one. Every red is an INJECTOR ARTIFACT, and they are
all one shape: **the plant's own BYTES are the test's subject.** ADR 0085's
consequences priced this family as a tail ("a test that changes `$0`, or
reads its own line numbers, sees one extra line"); measured, it is the whole
of the red set.

| # | the shape | tests (file:line of the fatal) | what the fatal says |
|---|---|---|---|
| 23 | a git hook slot | hooksredirectprobe_qa_test.go:148,183,505 · gates_test.go:2006,2419,2594 · launchhookpreheal_qa_test.go:208,305,342 · chainrelpath_qa_test.go:172 · chainrepair_qa_test.go:183,232,270 · verify_nx85_qa_test.go:282 · verify_37buk_qa_test.go:258 · parity_test.go:828 · l3_hookspath_qa_test.go:292 · gateschain_qa_test.go:645,831,880,951 · chainposseslot_qa_test.go:90,120 | posse decides ours/foreign/stale/chained by comparing an installed hook to the render it would write, so a rewritten plant is "a foreign hook, posse cannot vouch for a hook it did not write", "the pasted chain is not recognized as installed", or "exists and is not a posse hook — not overwriting" |
| 2 | a gate shim header | renderstamp_qa_test.go:92 · shadowedposse_qa_test.go:286 | `gateShimTarget` reads `gateShimMarker` out of the FIRST TWO LINES (possebinary.go) and the injected line IS line 2, so the shim reads as not a shim. renderstamp's own pin says it: "the first two lines are %d bytes; gateShimTarget only reads 512" |
| 1 | a content compare | skills_test.go:205 | the test compared a rendered COPY against the literal it planted, and the copy faithfully carries the injector's shebang |

## What was done about it

**The injector grew a second pass-through class; the content compare was
fixed in the test.**

`coldExecProtocolReason` (internal/posse/coldexec_test.go) passes a plant
through unchanged, and counts it, when its bytes are a PROTOCOL rather than a
fixture — a file some production reader identifies by content. This is not a
new rule: `execwrite.go`'s `callerIsTestFile` already keeps a shim PRODUCTION
code writes byte-exact, in its own words "so the content read-back pins over
rendered gates, hooks and probes cannot red for the injector's reason rather
than their own". A fixture planting the same protocol file is the same claim
about the same reader; the writer is the only thing that differs.

Three keys, and **each of the last two was bought by a run.** The first
spelling read the immediate parent directory only; the next full run left 4
reds, and they named exactly the two shapes it could not see.

| key | what it catches | why the obvious spelling missed it |
|---|---|---|
| the basename is a slot posse installs into, or a `-` chain spelling of one | `pre-push`, `posse-prepare-commit-msg`, `bd-prepare-commit-msg` | chainrelpath_qa_test.go plants `pre-push` RELATIVE, from a cwd inside the hooks dir — its subject is a prescription surviving its own cd, so the path carries no directory at all |
| the parent **or grandparent** dir is `hooks`/`managed-hooks` | a slot whose name is not one of the three | posse RENDERS into a per-session dir, `state/hooks/<session>/<slot>`, one level deeper than git's own |
| `gateShimMarker` inside the first two lines | gate shims, wherever they are planted | the header is the subject, so only the content can answer |

Five mutations, five distinct witnesses, each applied alone and reverted:

| mutation | witness that fired |
|---|---|
| the hook-slot key never answers | `a plant into a hook slot was rewritten, so posse's probe reads its own render as foreign` |
| the gate-shim key never answers | `a gate shim was rewritten …` **and** `gateShimTarget(…) = "", false after a pass-through` |
| every plant is a protocol | `an ordinary script plant was passed through, so this run measures nothing` |
| the basename key never answers | `a relative hook plant was rewritten — the basename is the only signal this one has` |
| the grandparent is not read | `a plant in the rendered per-session hooks dir was rewritten` |

The second one is the one worth keeping: the witness is the PRODUCTION
reader refusing the shim, not a counter.

`skills_test.go:205` was fixed in the test instead, and the new spelling is
the stronger one: it compares the rendered copy against **the source as it
lies on disk** rather than against the literal that was planted. That is
`copySkillFile`'s actual claim — the copy is the source — and a copier that
faithfully copied a source something else had touched now fails where the old
spelling passed.

## What the door reaches, which is the number to argue with

**MEASURED over the committed door's own green runs, by its own counters —
the same fifteen numbers twice.**
A pass-through is a plant whose first exec this gate does not make slow.

| | arm 1 | arm 2 | arm 3 | total | share |
|---|---|---|---|---|---|
| rewritten | 177 | 62 | 315 | 554 | 32.1% |
| …of those, exec'd at least once | 152 | 46 | 245 | **443** | **25.7%** |
| passed through: no `#!` (D5) | 325 | 205 | 308 | 838 | 48.5% |
| passed through: a git hook slot | 154 | 46 | 112 | 312 | 18.1% |
| passed through: a gate shim header | 18 | 5 | 0 | 23 | 1.3% |
| plant events | 674 | 318 | 735 | 1727 | |

So **the gate's real reach is 443 of 1727 plant events, 26%** — a plant that
is rewritten and never exec'd was rewritten for nothing, and two thirds are
passed through before that. The pre-fix run counted 1711 plant events with
871 rewritten and 840 without a `#!`; the small drift in the total between
runs is the suite's own (parallel fixtures do not plant the same number of
times every run).

**D5's "ASSUMED small" is the one to retire: 838 of 1727, 49%.** D5 wrote
"ASSUMED small: 124 of the 322 test call sites have `#!` in their literal
text within three lines … Slice 2 measures the real count." This is that
count. Nothing here says WHAT those 838 are — a copied binary cannot be
rewritten and the count was all this bead owed.

## The counts were invisible on a green run, which is where they are needed

`go test` shows a test binary's stderr only when the package FAILS. MEASURED
over six arms: the injector's install line and its count report are in every
red arm's log and in **none** of the green ones. So D5's "the gap is a number
and not a silence" held only while the door was red — the one state nobody
needs a coverage number in.

`POSSE_COLD_EXEC_REPORT` now names a file each arm appends its line to, and
the recipe prints it and keeps go test's own exit status (verified: with one
key mutated, `make coldexec-check` exits 2, prints `Error 1`, and still
prints the count). The variable is **consumed** at install — read, then
`os.Unsetenv` — because a child that inherits it installs an injector of its
own and appends a line of zeroes: MEASURED one such line in arm 1's run and
EIGHT in arm 3's, from pins that re-exec the test binary. The child half of
arm 6 now pins its absence.

## What went to the architecture lane

ranger-base-dacil (`-l architecture`, discovered-from this bead), because
both questions are about what the door is worth and not how it works: the
artifact class is a family rather than a tail, and the pass-through rule that
keeps the door green narrows it by every hook and shim in the tree, which D2
and D5 do not describe; and the gate reaches a quarter of the plants. The
price above is what that call needs. ADR 0085 is untouched here.
