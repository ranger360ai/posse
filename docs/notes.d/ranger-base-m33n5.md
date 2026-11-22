# ranger-base-m33n5 — the door for the bwrap fixture-home class is a RUN

The class is `ranger-base-jhmm7`'s. `BwrapArgs` judges every write grant
against the per-seat wall root composed from `$HOME` and refuses a grant that
contains it (`bwrapRefuseWallRootGrant`, ADR 0100 D1/D2(a)); a blanket temp
grant contains any fixture home cut under the temp root, and the refusal
stands down only where the home is under `/tmp` (`bwrapWallRoot`). So a
fixture home from a bare `t.TempDir()` is green wherever `t.TempDir()` lands
under `/tmp` — this box, and ci.yml's ubuntu legs — and red where it does not:
darwin, and any linux seat whose `TMPDIR` points elsewhere. The class has had
five fixes (`ranger-base-shf97`'s count) and every one of them was found by
reading a macos arm-1 log. The sixth, `ranger-base-mlw6q`, was found by this
door — which is the only argument for it that is not a price.

`ranger-base-shf97` measured every widening of the static pin that could reach
the renders it cannot see, took none of them, and measured a RUN instead. This
bead builds that run, prices all three arms, and hands where it should live to
the architecture lane (`ranger-base-o065a`).

## The door

`make tmpdir-check` — internal/posse's three arms with `$TMPDIR` pointed at a
fresh directory under `/var/tmp`, each through `scripts/test-times.sh` so each
takes a suite slot. `RUN=<regex>` is the filtered form, with `-v`, for
`coldexec-check`'s measured reason. `TMPDIR_CHECK_ROOT` moves the root.

Three things in it are load-bearing and none of them is the `go test` line:

- **`-count=1`, and the reason is not the one it looks like.** The failure
  this venue must not have is a cached PASS from the run with the other temp
  root: green having measured nothing, which is what ADR 0097 D5 refuses.
  go's cache already refuses it. MEASURED 2026-11-22 (go1.27.3) on
  `TestQAWallSeedProjectConfigCensusOfWhatThePrepCreates`: `ok 0.445s`, then
  `ok (cached)` on a second run under this box's TMPDIR, then — with no
  `-count=1` anywhere — a RE-RUN and a `FAIL` in 0.118s under a `/var/tmp`
  root. The test reaches TMPDIR through `t.TempDir`, the cache records that
  read, and the key moves with it. So `-count=1` is not what makes the door
  sound; it is what keeps the door from DEPENDING on that, cache-key
  behaviour being a property of the toolchain rather than of this tree. The
  first draft of this door claimed the opposite and the measurement is why it
  does not.
- **The resolved path is what is exported**, not the `mktemp -d` spelling. The
  raw/resolved split is part of this class: on darwin `/tmp` is a link to
  `/private/tmp` and `/var/tmp` resolves under `/private/var`, so a venue that
  handed the arms an unresolved root would plant fixtures at one spelling and
  have the render answer at the other (`wallStandDownDir` carries the same
  argument for the same reason).
- **A root it cannot use is a REFUSAL and never a pass.** Both arms were
  measured, 2026-11-22, this box:

      make tmpdir-check TMPDIR_CHECK_ROOT=/tmp RUN=…       exit 2
        "…/tmp resolves to /tmp/posse-tmpdir-check.HZ2T16, which is inside a
         blanket temp grant — this venue would run the exact shape the class
         is invisible in."
      make tmpdir-check TMPDIR_CHECK_ROOT=/proc/nope RUN=…  exit 2
        "…cannot create a fixture root under /proc/nope — inside the bwrap
         wall of a caged seat that directory is read-only, so this seat cannot
         be the venue."

  The second is not hypothetical: inside a seat's own bwrap wall `/var/tmp` is
  read-only, which `wallUngrantedDir`'s comment carries as its own reading. A
  caged seat is not this venue and now says so instead of exiting 0.

`/var/tmp` is the root for `wallUngrantedDir`'s reason — granted by nothing on
either host, because `/tmp` and `/private/tmp` are different directories — and
it is checked the way that helper checks its own: against the resolved answer,
not against the spelling, because a box that symlinked it into `/tmp` would
hand this door the shape the class is invisible in.

## The prices

MEASURED 2026-11-22 (linux 7.2.15 x86_64, go1.27.3, 16 cores, warm build
cache, at 144605f2, one-minute load 10.29 → 16.44 with one sibling seat
holding a suite slot throughout), `TMPDIR` at a fresh `/var/tmp` root per arm:

| arm | command | `go test` | wall | verdict |
|---|---|---|---|---|
| 1 | `./internal/posse/...` | 292.0s | 293s | RED — exactly 4 members |
| 2 | `-tags posse_arm2 ./internal/posse` | 70.5s | 76s | green |
| 3 | `-tags posse_arm3 ./internal/posse` | 127.0s | 158s | green |
| | | **489.5s** | **527s** | |

25s of arm 3's wall was a queued suite slot, not test time; all three arms
took one (`suite-lock: slot 1 of 2`, and arm 3 `acquired after 25s`).

**Arms 2 and 3 were UNMEASURED under this venue before this bead and are
green.** `ranger-base-shf97` had priced arm 1 alone, so "price them before
committing to a shape" was the bead's own condition. The answer is that the
door's entire live yield is in arm 1 and its entire cost is 527s — which is
the number the placement decision turns on, and it is 5.7x what the live half
costs on its own.

shf97 read arm 1 at 263.0s; this reading is 292.0s at a one-minute load of
10-16. Same class, different box weather — which is what every wall reading in
this repo says about which variable tracks.

## That it is the discriminator, measured in both directions

The filtered form over the four members, 9.8s, no suite slot:

    make tmpdir-check RUN='TestQAWallSeedProjectConfig|TestQAWallWitnessTakesTheRowAway'
      --- FAIL: TestQAWallSeedProjectConfigCensusOfWhatThePrepCreates
      --- FAIL: TestQAWallSeedProjectConfigLaunchInTheHomeItSeededIsClean
      --- FAIL: TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate
                 (a_linked_worktree, a_main_checkout)
      --- FAIL: TestQAWallWitnessTakesTheRowAwayWhenTheWallCannotBeRenderedAtAll

and the control, the same filter with `TMPDIR` unset — this box's own shape
and ci.yml's ubuntu legs' — `ok` in 1.267s, all four PASS. The unfiltered arm
1 named those four and nothing else.

Two of the four are `ranger-base-shf97`'s (`wallseedprojectconfig_qa_test.go`)
and two are `ranger-base-mlw6q`'s (`wallwitness_qa_test.go`). Both fixes were
in flight in the code lane and neither had landed on main at 144605f2, so this
run reproduces their reds independently of the readings that filed them — and
no door can go on anyone's prerequisite line until they land.

## Why a run and not a scan, in one table

From `ranger-base-shf97`, MEASURED 2026-11-22 at 648a32d6 over the live corpus
with the pre-fix source overlaid. Reproduced here only in its conclusion; that
bead's record is the derivation.

| variant | reach | flagged | real | false |
|---|---|---|---|---|
| the pin as it stands | 106 | 0 | 0 | 0 |
| + a method edge where the name is UNIQUE | 109 | 0 | 0 | 0 |
| + a method edge keyed by NAME ALONE (unsound) | 142 | 2 | 0 | 2 |
| the 10 production seeds, identifier edges | 189 | 13 | 0 | 13 |
| the 10 + unique-name method edges | 192 | 13 | 0 | 13 |
| the 10 + name-alone method edges (unsound) | 230 | 17 | 2 | 15 |
| **the run** | — | **4** | **4** | **0** |

The best static variant flags 2 of arm 1's 4 at a cost of 15 false reads; the
run finds 4 of 4 at a cost of none. It needs no call graph, no method-edge
guess and no model of which grants reach the home, because the class's whole
content is that a render's refusal turns on where the box puts its temp files
— so a second run with the temp root moved IS the discriminator. The two
members in `wallwitness_qa_test.go` are ones no variant in that table flags at
all; the run found them (`ranger-base-mlw6q`).

## One abstention, and it is declared

Arm 1's tail:

    posse wall fixtures: 1 test(s) that need a fixture root outside every
    blanket temp grant ABSTAINED and measured NOTHING (a dark run is declared
    by POSSE_TEST_UNGRANTED_ROOT=none …) — all 1 declared (ranger-base-ydw7p)

That is `wallungrantedroot_qa_test.go`'s own subtest, which sets
`envUngrantedFixtureRoot` to `ungrantedRootDark` itself (`:117`, `:258`) to
drive the mechanism it pins. It is a property of that test and not of this
venue. Nothing else abstained: `wallUngrantedVerdict` makes the COUNT decide
the exit status, so arms 2 and 3 exiting 0 is the statement that neither had a
silent abstention — which matters here, because `go test` without `-v`
discards everything a passing package's binary writes and the line itself is
unreadable on a green arm.

Moving `TMPDIR` to `/var/tmp` does not take `wallUngrantedDir` away: its root
is a SIBLING of the venue root under `/var/tmp`, outside both the venue's
`TMPDIR` grant and `/tmp`, so the pins whose subject is the refusal still
measure it.

## What was not taken, and why

- **A treepins pin that the venue's arms equal internal/posse's build-tag
  partition.** The three recipe lines are hand-written, as `make test`'s,
  `vet`'s and `coldexec-check`'s are. Filed as `ranger-base-c3zb8` rather than
  built here, which is `coldexec-check`'s own history: the door landed under
  ADR 0085 D6 and the derivation came later under `ranger-base-s64a5`.
- **A sentence in `internal/treepins/bwraphomestanddown_qa_test.go`'s head
  comment naming this door.** It belongs there — that comment is where a
  reader meets the half the pin cannot see — but the file is named by
  `ranger-base-shf97`'s own DONE WHEN and that bead is open, so this is said
  on the bead instead of raced for (AGENTS.md).
- **Any widening of the static pin.** shf97 measured all of them and the table
  above is why.

## Not claimed

That a darwin run was made. That arm 1 is green under this venue once
`ranger-base-shf97` and `ranger-base-mlw6q` land — it reds exactly their four
members and nothing else at the reading above, which is a statement about
those four and not a prediction about the arm. That the arm list is derived;
it is not (`ranger-base-c3zb8`). That 527s is a budget: it is one reading at
one load, on a box where a sibling seat held a slot the whole time.
