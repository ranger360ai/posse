# ranger-base-mklu0 — a member the marker never counted, and an account the truncation ate

Two findings of `ranger-base-2yt8s`'s verify (4 closes: `ranger-base-8ck6d`,
`-x5ydb`, `-eheo3`, `-mhl7h`), both about what reaches a bead when a wall pin
reds — which is ADR 0097 D3's subject and the subject of the closes they
escaped. Every reading below is MEASURED 2026-11-22 on this box (linux
7.2.15-1-omarchy, bwrap 0.13.1, go1.27.3, 16 cores, cage shims), before and
after, within the same hour.

## Finding 1 — three live pins exec a bwrap and were members of nothing

`internal/posse/wallunixlive_test.go` has four live pins. Three of them exec a
bwrap (`TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched`,
`…ScopesSignalsToTheSeat`, `…LeavesCrossDirectoryRename`) and all three gated
on `wuLiveSkip`/`wuSigLiveSkip`, which reached `bwrapNestSkip` through no
route at all. That gate's `nestMeasure` arm is the ONLY emitter of
`wall venue: measuring (ADR 0097)`, and membership is read off that marker and
never off the name (ADR 0097 D4, `scripts/posse-wall-check`'s
`member_pattern` selects the run and the marker decides the set) — so the name
these three already had bought them nothing.

Three of the four rows `ranger-base-8ck6d` folded sat in that file, and its
description asserts the opposite: "Both files are members, so both spend one
line on a bead when they red". The fold itself is delivered and verified; the
premise was false.

**What it cost, and the reading that decides it.** A `bwrap` shim on PATH that
prints `bwrap: setting up uid map: Permission denied` and exits 1 is the
walled-seat shape (ADR 0096 D1); the test binary is built first with the real
bwrap, so the build is not what the shim changes.

| | the three | two gated controls | the fourth pin |
| --- | --- | --- | --- |
| before | **FAIL / FAIL / FAIL**, binary rc=1 | SKIP | PASS |
| after | SKIP / SKIP / SKIP, binary rc=0 | SKIP | PASS |

The controls are `TestLiveWallClaudeProjectsRefusesPeerPlants` and
`TestLiveBwrapSeatTmpKeepsTheSeatOutOfTheHostsTmp`; the gated SKIP reads
`NOTHING MEASURED: this process is inside a wall that forbids nesting`. ADR
0096 D3 forbids that red in words — "IT NEVER FAILS FOR THE WALL, outside the
venue" — and ADR 0100 D5's SCOPE note counted 2 walled seats on this instance
on 2026-11-15, so it was latent and not hypothetical. That is why the bundle
is P2 and not the P3 diagnostic class `ranger-base-a688i` named.

**The counted set went 51 → 54**, which is the whole-venue reading of the
same fix: `bash docs/adr/0097-wall-venue.probe.sh venue+check` answered
`wall check: green, 54 pins measured`, rc=0, 125s wall at a load of 8.96
(the venue's own `go test` took 126s of it). It read 51 before, and 51 was
also the package census of marker emitters against the 57 real tests inside
D3's filter — the 6 outside it were wallunixlive's four and the two
wall-witness pins, and only the witness pair was recorded anywhere.

The other two consequences, both of them the same silence one arm apart: a red
at one of the three printed as `note: FAIL <test>` under
`wall check: green, N pins measured` at EXIT 0 — the script's own self-test
says so, `green: a NON-member failed and every member passed` — so D3's
member-FAIL exit 1, which ADR 0088 D5's ratchet files as one P1 carrying that
line, never fired for them; and a run in which they went DARK was counted by
neither D5 arm, which is the /var/tmp shape D5 exists for and which
`ranger-base-vbp5a` already measured once for these very pins.

**THE FOURTH PIN IS NOW RECORDED AS A DELIBERATE NON-MEMBER.**
`TestLiveWallUnixStepReArmsAClearedDieWithParent` execs no bwrap at all — its
launcher is `sh -c '"$@"; :'` and what it drives is posse's own wall-unix
step — so it needs no nest to run and passes under the shim above. It takes
`wuSigStepSkip`, a sibling of `wuSigLiveSkip` without the gate, whose comment
is the record; `scripts/posse-wall-check`'s own prose went from two such tests
inside D3's filter to three and names all of them (the two wall-witness pins
are the others).

**The one ordering judgement.** The gate goes BEFORE the Landlock-ABI floor
and after GOOS and `LookPath("bwrap")`. GOOS and LookPath have to come first
because the gate cannot probe a nest without them. The ABI floor does not, and
the gate logs D4's marker the moment it lets a pin proceed — so a skip after
it is an abstention the venue COUNTS (exit 3) and a skip before it is a pin
the venue never knew it had. `scripts/posse-wall-check` carries no floor on
the member count, so pins going dark one at a time read as `green, N pins
measured` with a smaller N and nothing to compare it to. A kernel whose ABI
fell below RESOLVE_UNIX is a venue with no socket wall left to measure, which
is a finding and not a quiet absence.

## Finding 2 — the deadline's member line was cut before the seat's account

`internal/posse/wallclaudeprojectsattack_qa_test.go`'s 90s deadline branch was
ALREADY one line — which is why `ranger-base-w6kke`'s `:\n%s` census could not
see it. What it spent its budget on was the ORDER.
`scripts/posse-wall-check`'s `evidence()` carries the first `file.go:NN:` line
truncated to 300 characters with that prefix inside the 300, and the account
was LAST, behind the cwd and a D3 reasoning preamble.

Both readings come from planting `0 * time.Second` for that file's 90s
deadline and slicing the first `.go:`-bearing output line to 300 as
`evidence()` does. The reported line is the CALLER's — the closure calls
`t.Helper()`, so the position is `out := run(t, nonOwn)` and not the `Fatalf`.

| | before | after |
| --- | --- | --- |
| rendered | 492 chars | 182 chars |
| reaches the bead | 300, cut mid-word | 182, whole |
| ends | `… the seat answered signal: killed, and t` | `… and the deadline is the whole finding` |

192 characters lost, and 261 of the 300 were spent before "the seat answered":
the cwd (`/var/tmp/posse-ungranted-3023754429/home/plain`, 46 bytes here and
more under a longer TMPDIR) and the preamble. Verdict fail and marker present
in both. The sibling fold at `wallclaudeprojects_qa_test.go`'s own 90s branch
reads 176 and fits, which is the shape this one now has.

`ranger-base-eheo3`'s close quotes the untruncated `go test` line as "the
Fatalf shape scripts/posse-wall-check carries" — true of the line, not of what
the check carries.

**THE STATUS IS NOW DELIBERATELY NOT CARRIED**, which is the half of this the
bead's own reasoning forced. THIS process does the killing one line above, so
`runErr` reads `signal: killed` whatever happened: the deadline naming itself
and not the seat. For a HANG there is no status to read and the seat's words
are the whole account, so the fold is `wwOneLine` with its own silence case —
`bgSeatAccount`'s NOTHING sentence answers a seat that ENDED without a word,
the opposite of a seat still running at 90s. That is `bgSeatAccount`'s own
rule (each caller keeps its own account of silence and shares the fold) and
the spelling the sibling already had. The cwd and the preamble keep line 2,
where D3 says supporting prose costs nothing.

## Not closed here

- ranger-base-dudu4
