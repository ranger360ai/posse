# ranger-base-c75wa — the floor on n is the run subtracting from itself

`scripts/posse-wall-check` reported `wall check: green, <n> pins measured` and
compared n to nothing. Membership is read off ADR 0097 D4's marker line, whose
only emitter is the nest gate, so a member that stops reaching that gate stops
existing as far as the body is concerned: the run is green with a smaller n,
and nothing in the tree is a second reader of n. Abstentions D3 counts;
disappearances it did not. This is judgement 3 of `ranger-base-dudu4`, which
declined to decide it, and the shape is D5's own `/var/tmp` row one mechanism
over — there a pin printed the marker and THEN skipped, which the abstention
arm counts; here it never printed the marker at all.

## The reading that decided it

MEASURED 2026-11-22 in ADR 0097 D1's venue on `bd7891ac` (linux
7.2.15-1-omarchy, bwrap 0.13.1, go1.27.3, 16 cores, cold GOCACHE,
`--unshare-user --unshare-pid --proc /proc`, private `/tmp` and `/var/tmp`,
`POSSE_WALL_VENUE=1`, load 6.2): **118s, `go test` exit 0, 57 tests selected,
54 members, 3 dark, all 57 passing.** The three dark are exactly the three
`ranger-base-dudu4` names as deliberate:
`TestLiveWallWitnessProbeDoesNotWriteTheLiveWall`,
`TestLiveWallWitnessHoldsUnderTheRenderedWall` and
`TestLiveWallUnixStepReArmsAClearedDieWithParent`. None of them execs a bwrap,
so none reaches D4's gate and none can print the marker.

57/54 is the same pair `ranger-base-dudu4` measured off the SOURCE — in flight
in another lane as this landed, so the agreement is between two readings and
not yet between two landed pins. It is the only cross-check this number has,
and the two readings are of different things on purpose: a source census
cannot see a pin that reaches the gate in source and skips before it at
runtime, which is the whole of what a runtime floor buys.

## The floor, and why it is not a typed n

The bead's own tradeoff was that a typed floor goes stale on every landing
that adds or retires a pin, and a stale floor in a shipped script is a red on
a tree nobody changed. The body does not type n. It subtracts:

    n >= <tests the filter selected> - nonmember_ceiling

Both sides move together when a pin is added, retired or renamed, so a landing
that touches the member set reds nothing. The one number typed is
`nonmember_ceiling=3` — the size of the deliberate set, not of the member set.

**Its staleness is one shape and only one.** A NEW deliberate non-member reds
a tree that is fine, once, printing every dark name, and the fix is to raise
the ceiling in the commit that adds it. A deliberate non-member RETIRED leaves
the ceiling slack by one — a pin's worth of floor lost rather than a red — so
it is lowered in the commit that retires one. That is the entire maintenance
burden, and it falls on somebody who is already editing the wall pins.

The number is held to the record by
`TestQATheWallCheckNonMemberCeilingIsTheRecords` (arm 6 of
`internal/treepins/wallcheckselftest_qa_test.go`), because it is the one
number in that body with no other side in `internal/posse`: D4's three
literals are a handshake with `wallvenue_test.go` and are held to ADR 0097 by
`TestQAWallVenueContractLiteralsAreADR0097s`, and this one cannot get there by
that route. Raised without the record it is a floor quietly lowered, with
every signal still green.

## The two floors not taken

| option | what it costs | what it buys over the subtraction |
|---|---|---|
| DERIVE the member count in the venue | a step that compiles against `internal/posse`, which is what D4's three contract literals exist to avoid (ADR 0088 D1: the body resolves no path out of `$0`, so it cannot even reach a sibling parser) | nothing. What it would derive is the SOURCE count, which the censuses in `internal/treepins` already read off the tree — and which is blind to a pin that reaches the gate in source and skips before it at runtime |
| RATCHET on the last green n | a new git-config record (`posse.wallCheckQuiet`'s `<n>` is D5's consecutive-silence counter, not a member count), a runner that reads and writes it, and a further D5 amendment | drift detection one landing LATER than the subtraction gives it, and nothing at all on the first run of a fresh clone |

Neither is refused on principle; both are more machinery for a weaker answer
than one subtraction the run was already in a position to do.

## What landed

- `scripts/posse-wall-check`: `nonmember_ceiling=3`, a `selected` set in the
  parser, and a fourth exit-3 arm after the member arms — `wall pins WENT
  DARK: <k> of the <m> test(s) … printed no marker`, naming every dark test
  with its verdict and its first evidence line. It goes after the member arms
  because a member that FAILed is still this check's red and a member that
  abstained is still the better diagnosis; this arm is what is left over.
- Two `--self-test` arms, both sides of the one arithmetic: `green: the
  deliberate non-members printed no marker and n is the members alone` (three
  dark, at the ceiling, green) and `abstention: a selected pin went dark,
  putting n under the derived floor` (four dark, exit 3). Twelve arms clean,
  0.2s.
- `internal/treepins/wallcheckselftest_qa_test.go`: the two labels in
  `wcsArms`, arm 6 above, and arm 5's mutant counts re-read against the body
  (`sys.exit(3)` sites 2 → 3, `st_arm` calls asserting 3 four → five). That
  arm `t.Fatalf`s rather than passing when the counts disagree, which is what
  brought the update with it.
- ADR 0097: the amendment, which is where the three deliberate non-members are
  named. The body names no test at all, deliberately — nothing in it may read
  membership off a name.

## One gotcha worth keeping

The parser is a `python3 -I -c '…'` body inside a single-quoted shell word, so
**an apostrophe anywhere in it — comments included — ends the quote** and bash
reads the rest of the parser as shell: `syntax error near unexpected token`,
exit 2, before any arm runs. A `run's own arithmetic` in a comment cost two
rounds here. The existing body has no apostrophe in it either, which is why
nothing had hit it before.
