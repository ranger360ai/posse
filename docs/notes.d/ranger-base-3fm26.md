# Four closes, and the three guards their own mutants could not reach (ranger-base-3fm26)

2026-11-11. Verify bead for ranger-base-6pew3, ranger-base-dfgte,
ranger-base-pn621 and ranger-base-6hjm5. All four VERIFIED. Three LIVE findings
went to the code lane as one bundle (ranger-base-n2yaw) and six are recorded on
the close comment. What is worth keeping past the close is the METHOD in each
case, because all three live findings are the same shape seen three times: a
guard or a derivation whose subject no shipped arm can address.

## Where the evidence was taken

D1 host, linux 7.2.15, 16 cores, go1.27.3 linux/amd64, bubblewrap 0.13.1,
bd 0.50.3, a live claude 2.1.322 seat on a `multiplexer: rex` box. All readings
at `d68b0179` (main at the time this bead opened; main moved twice under it),
from this bead's own session worktree. `make tree-check` green over that tip in
44.9s at a 1-minute load average of 3.21 — below the 55-62s floor the register's
own head comment states for a reading under a load of 16, which is one warm
reading and not a claim.

Landing: three of the four closes named a pre-rebase sha. `git patch-id --stable`
paired only one of the three; the other two differ because `docs/notes.d/README.md`
grows with every other bead's fragment and because a commit that landed between
the branch and the merge arrived under the rebase. Per-path md5 over the
commit's own paths is what settles it — nine files byte-identical for the cage
map close, one README hunk for the liveness close. Nothing stranded.

## 1. An order claim is graded by SWAPPING, not by removing

ranger-base-6pew3 put a liveness refusal ahead of the screen refusal in
`RexMux.Prompt` and said, in the code, in the ADR (0076 Amendment 9 D7) and in
the pin's own name, that the order is load-bearing: "a block that has gone back
to its shell has no composer to be judged for, and the stale box the dead
runtime left on the screen would otherwise license the prompt."

Removing the new refusal reds five pins, the order-named one among them. That
reads like a kill for the order and is not one. The SWAP is:

| mutant | focused 21 pins | whole arm-1 `./internal/posse` |
|---|---|---|
| `refuseAtDeadOwner` removed from `Prompt` | 5 red | — |
| the two refusals exchanged | **green** | **green, 118.9s** |

The reason is structural and worth naming as a class: `rexOwnerDead` and
`rexOwnerShell` both return a ZERO owner value, and the screen refusal's first
line concedes on any owner that is not the one runtime its reading is written
for. So refusal B is satisfied by exactly the zero value refusal A's path
produces, and no input can tell the two orders apart. The pin still holds
something true and load-bearing — a dead owner behind a drawn composer is
refused, and the liveness refusal pays for no screen capture — but not the
sentence it is named for.

**The question to ask of any "A runs before B" claim: does B's guard clause
accept the zero value A's branch hands it?** If it does, the order is a comment
and not a behaviour.

## 2. Ask what a fix changed the SHAPE of, not only the value of

ranger-base-6hjm5 moved the record-reach gate's git targets from `beadsGitDirs`
(git directories) to `sessionGitGrants` (what the wall actually grants a session
worktree). Three of that function's six entries are not directories at all:
`<common>/packed-refs.lock`, `<common>/refs/heads/<branch>` and that path plus
`.lock`. The filter that keeps them out of the probe — `isDirPath`, present
since the package rename for an entirely different reason, a `.git` that does
not exist — became the only thing standing between the gate and a
`: > <regular-file>/probe`, which is ENOTDIR.

Removing the filter is green over the focused 44-test selection AND over the
whole arm-1 package (100.5s). The four shipped arms cannot see it by
construction: they assert that every target is a SUBSET of the grants, and the
three lock paths ARE granted. **A set-comparison pin is blind to the KIND of
its members.** The ref file is not hypothetical — `<common>/refs/heads/posse/<branch>`
is a 41-byte regular file in the shared git dir for every live session.

The same reading produced the one behavioural fact that close could not claim:
the gate was exercised live under a real bwrap wall from a real linked worktree
with the real store (`record: store of record → enforced (… 2 targets created
and removed under bwrap)`). That is the another-repo arm, not the exposed shape,
but it is a wall reading where the pins are producer-level by necessity.

## 3. A derivation keyed on one spelling, and a shipped script nothing runs

`scripts/posse-land-check` derives the build-tag arms it vets from the tree's own
`//go:build` lines rather than a literal list — deliberately, so a fourth arm is
covered by the commit that declares it. Two gaps, both measured:

* A bare `// +build posse_arm5` line with no `//go:build` beside it is still
  honoured by the toolchain — `go list -f '{{.GoFiles}}'` names the file in 0
  files of the default build and 1 under `-tags posse_arm5` — and the derivation
  does not see it, so that arm goes unvetted at the one gate that judges a
  replayed tip. `gofmt -l` flags such a file, so `make fmt-check` reds it; but
  that is a different gate, and the AGENTS.md paragraph on tree-wide pins
  records four commits that reached main not gofmt-clean straight through it.
* Nothing in the suite runs the script. `grep -rl ranger-base-pn621 --include=*.go`
  returns zero files, test or not; `scripts/shell-syntax.sh` parses it and the
  scripts censuses read it as text. **Zero `.go` hits for the bead id of a
  shipped deliverable is a finding, not a coincidence.** The four by-hand
  `--plan` witnesses on the close all reproduce — a planted `posse_arm4` in an
  existing package, the same tag in a new one, a `//go:build` at column 0 inside
  a raw string BELOW the package clause (correctly ignored), and every
  `posse_arm` occurrence renamed away (exit 1, read outside the pipeline) — which
  is exactly what a `--plan` pin would assert mechanically.

Attacked and survived, on the same script's "executes nothing the branch wrote":
a branch that ADDS a cgo file does not get to choose the C compiler's argv,
because Go's own `#cgo` flag allow-list refuses the compiler-plugin shape
outright (`invalid flag in #cgo CFLAGS: -fplugin=…`). The useful part of that
negative result is that the header's "this repo has no cgo" clause — a statement
about the judged tree, which is the untrusted input — is not load-bearing.

## 4. Grading a cage-map close without writing a deployed home

ADR 0086 D2's close deferred its live half in writing, because `posse gates`
RENDERS the persona's gates dir and the wall artifact and a deployed home is not
a code seat's to write. It is not a verify seat's either. Two surfaces split the
job:

* `posse status`'s G17 lane row is READ-ONLY and names the declined PIDs and
  their count against the real home: "declined for 1 PID(s): …, which launch at
  their demanded tier because the mapped one leaves a rule of theirs
  unrealized". That reproduces the ADR's own census — one of eleven host PIDs —
  through the resolver and against the live config, with no render.
* `posse gates <persona>` over a `cp -a` copy of the home, with a build of the
  landed tip, carries the rest: the declined PID's `Edit`/`Write` rows
  `enforced (L2 seatbelt (bwrap), witnessed at launch)` and the
  `⊘ cage_realize: … declined for this PID` line.

Two traps there. `posse gates`' FIRST parity block is `posse.DefaultCage` as a
literal for every persona, so a PID whose demand the map declines is read off
the SECOND block — reading the first as the PID's wall is the mistake. And the
report reads no cage from the environment at all (measured both ways, identical
output), so shedding `RHQ_CAGE` buys nothing; the only `os.Getenv("RHQ_CAGE")`
readers in the non-test tree are a message and a trust reading, neither on that
path.

The remaining half of such a DONE WHEN — "dispatch launches it" — is bounded by
the INSTALLED build, which was 30 commits behind main and carried none of the
four fixes. `git merge-base --is-ancestor <fix> <build stamp>` says so in one
line, and that makes the live-fleet reading unmeasurable rather than unmet.

## Mutation readings, for whoever extends these arms

Every mutant below was applied by a single-occurrence python replace (assert
count == 1; the sources are TAB-indented), compiled, and the file restored from
a golden `cp` afterwards with md5 confirming the restore.

| close | mutant | what red |
|---|---|---|
| liveness | the new refusal removed from `Prompt` | 5 pins: both refusals, the hand-back, the not-scoped-to-a-runtime arm, the order arm |
| liveness | refuse on the UNKNOWN verdict too | both concession pins and a sibling no-wait pin |
| liveness | the shell verdict's arm dropped | the shell refusal alone |
| liveness | the refusal's code changed off the hand-back allow-list | the hand-back pin alone |
| liveness | the verdict scoped to one runtime | the not-scoped arm (two subtests), two verdict-table rows, the shell refusal |
| liveness | **the two refusals exchanged** | **nothing, package-wide** |
| cage map | parity resolving the raw map itself | the decline table, the declined launch, BOTH census arms |
| cage map | the launch door dropping its resolution | the door census and the launch arm |
| cage map | the "realized at the demanded tier" half dropped | the subsumption pin |
| cage map | never declining | three decline rows, the launch arm, the governance row |
| cage map | the availability clause deleted | nothing — disclosed on the close, and the SUBSUMPTION is what is pinned |
| cage map | a decline keeping its ratified waiver | the decline table and the launch arm |
| cage map | the declined line's from/to exchanged | the decline table |
| cage map | the standing's declined walk cut | the governance row |
| cage map | **the map-free parity variant allowed to re-read the map** | `fatal error: stack overflow` — the recursion is real and the flag is load-bearing |
| record reach | the wall-producer branch dropped (the pre-fix derivation) | the worktree arm and the agree matrix's two worktree/open cells |
| record reach | the main-checkout fallback dropped | the main-checkout arm AND its predecessor bead's own control |
| record reach | the whole-tree-deny branch dropped | the agree matrix's four deny cells and the D3 pin |
| record reach | the another-repo branch dropped | the store-repo arm and the D3 pin |
| record reach | **the directory filter dropped** | **nothing, package-wide (100.5s)** |

Three survivors, three different reasons: one disclosed and pinned as a
subsumption, one unobservable by construction, one a guard whose subject the
shipped arms cannot express. Only the last two are findings.
