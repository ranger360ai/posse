# Verifying four closes, and the platform half nothing read (ranger-base-zm03j)

The QA verify of ranger-base-bx9wj, ranger-base-cwboz, ranger-base-rph5z and
ranger-base-blkse. All four verify on their own Done conditions. What this
fragment is for is the part that does not fit on a bead: the venue one of them
could not reach and this run did, and the mutation table that says which arms
of the box-loop census were WITNESSED rather than merely necessary.

Box, for every reading below: Linux 7.2.15-1-omarchy, go1.27.3, 16 cores,
bubblewrap 0.13.1, uid 1001, tree at 22690b7a, loadavg 4-8 with one sibling
seat holding one suite slot.

## The venue ranger-base-rph5z's fix is for, measured in it

ranger-base-rph5z's own verify line was "run the verb's box reading from a
child-userns seat with a live loop. It must NOT print 'no dispatch loop on
this box'". The fix's author could not reach that seat class — a child user
namespace was refused at their seat — so the EACCES reading stayed the
security review's, attributed, and what the close verified was the consumer
behaviour through the `boxProbe` seam.

It is reachable with `unshare -U -r`, which needs no privilege, and this is
that reading. MEASURED 2026-11-21, `uid_map` = `0 1001 1`, against both live
dispatch loops on this box (pids 1185112 and 1185185):

```
readlink /proc/1185185/exe   -> Permission denied
/proc/1185185 dir owner      -> uid 0 (this ns's mapping of 1001)
/proc/1185185/comm           -> posse
/proc/1185185/environ        -> refused
/proc dirs listed            -> 408
```

So the pid LIST opens and the links do not, which is exactly the mechanism the
ADR's amended paragraph names. Running the census itself from there, through
the real `liveBoxProbe`:

```
dispatch (box): not read — none of the 3 process(es) of this account on this
box could be read from this reader — /proc/<pid>/exe is refused, which is what
a seat in a child user namespace gets for a same-uid peer — so whether any
dispatch loop is still on the replaced image is UNKNOWN, not no. ...
```

The all-clear is not printed; the abstention is. **ranger-base-rph5z's fix
works in the venue it was written for**, and that is now the first measurement
of it from inside that venue rather than through the seam.

The same run turned up what the pin above it did. `TestLiveBoxCensusReadsThisBox`
opened with a `t.Fatalf` on `r.Err != nil` — "a linux box with an installed
posse must yield a reading, not an abstention" — so from a child-userns seat
the arm RED on the correct behaviour, and the comment four lines below it,
which says the arm "asserts the IMPLICATION in both directions" because it
"cannot choose which seat it runs on", was unreachable there: the Fatalf ran
first. The arm now branches, and in the abstention branch asserts the shape a
reading from that seat owes — that the all-clear sentence is absent and that
`boxSentenceAdmitsItDidNotCoverTheBox` holds. Green in both venues, with the
rendering logged in each.

## The mutation table: three arms were unwitnessed, all in the platform half

Each mutant applied ALONE with the subject untouched, graded against the whole
`^TestQABox|^TestLiveBox|^TestQATheVerbsBoxLine` filter (and the treepins
fence census for the last). A `[build failed]` is never a kill, so the two
that dropped an import were re-spelled and re-run.

| # | mutant | before | after |
|---|---|---|---|
| M4 | count no refusal at all (`Unreadable` never increments) | KILLED ×3 | KILLED |
| M5 | drop the `comm` narrowing in `boxRefusalWorthReporting` | KILLED | KILLED |
| M6 | drop `Lstat`+`IsRegular` on the peer's record | KILLED (10s deadline) | KILLED |
| M7 | drop the portable chokepoint fence in `readBoxLoops` | KILLED | KILLED |
| M8 | drop the PLATFORM fence in `boxHomeOf`, chokepoint left in | **SURVIVED** | KILLED |
| M9 | `boxPidOwned` stops asking the uid | **SURVIVED** | KILLED |
| M10 | `boxCommOf` answers `("", false)` always | **SURVIVED** | KILLED |
| M11 | `boxHomeOf` lifts a SECOND variable | KILLED | KILLED |

M4-M7 and M11 are the fix authors' own list, re-run and confirmed. M8-M10 are
a family and not a tail: every pin in the package drives `readBoxLoops` through
`boxProbe`, which substitutes all four platform reads, so `boxloops_linux.go`
had one reader — the live arm — and that arm asserts implications which hold
whatever those three answer.

M10 is the one worth naming. `boxCommOf` IS the second round of
ranger-base-rph5z's fix: counting every same-uid refusal made the caveat fire
on every reading on this box forever, and `comm` is what narrows it. A mutant
that stops reading `comm` restores that defect with every pin green.

`TestQABoxPlatformReadsAnswerForRealPidsOnThisBox` pins the three against the
only fixture they have — real pids on this box, plus one child of the test for
the one value a peer controls:

- `boxPidOwned` of this process, of a pid with no `/proc` entry, and of pid 1
  when another account owns it.
- `boxCommOf` against an independent read of `/proc/<pid>/comm`, then the
  narrowing in BOTH directions over ONE refused pid: counted for a target
  named after its own program, not counted for a decoy. MEASURED here on pid
  1140 (`comm` "systemd"), and on pid 101883 (`comm` "bash") from inside the
  child userns.
- `boxHomeOf`'s own fence, over a CHILD of the test exporting the value — the
  derivation the finding named, since a caged seat's own child qualifies.

**The first spelling of the `comm` arm found its fixture BY CALLING
`boxCommOf`**, so under M10 it found no fixture, logged that this box was not
one, and passed. A rig that builds its fixture from the function under test
makes every mutant of that function equivalent. The search reads `/proc`
itself now, and the comment in the test says why.

## The other three closes

- **ranger-base-bx9wj.** Its Done condition is a live wall pin that ADR 0100
  would refuse going red under an uncaged `go test` as well as in the venue.
  Reproduced: with both refusals forced to stand down (`bwrapWallRoot() == ""`
  -> `true` in `bwrapRefuseUnpinnedSource` and `bwrapRefuseUnpinnedDst`),
  `TestLiveBwrapSeatStoreWideGrantPinsTheLooseComponent` FAILS with TMPDIR
  unset — "with the in-place bind of `<common>/worktrees` cut from the argv,
  neither of ADR 0100's arms refuses the render". Pointing `bsUngrantedHome`
  back at a `t.TempDir()` reds that test at its control AND reds
  `TestQABwrapSeatStoreFixtureHomeStandsADR0100sRulesUpInBothVenues` in both
  of its TMPDIR shapes, so the home placement is itself held. Both pins green
  with TMPDIR unset and with TMPDIR under the home.
- **ranger-base-cwboz.** All four items landed and M5-M7 and M11 hold them.
  The census fence (item (d)) is in internal/treepins and reds on a second
  environ reader. One residual is open and is named below.
- **ranger-base-blkse.** `wgPack`'s vacuity guard, with the import removed,
  reds on linux with `IN THAT DIRECTORY: developer-2-probe` — the macos line
  verbatim, which is the darwin red reproduced on this box and the close's own
  claim. The control in
  `TestQAWgPackPrunesTheRefDirectoryAfterASeatStoreCommit` is not decorative:
  the fixture really does cut a sibling session's branch into the same
  `refs/heads/posse`, so "the same pack-refs prunes the sibling's loose ref
  and not this one" is the measurement that excludes the bead's stated git
  2.56.1 confound.

## Not closed here

- ranger-base-fugew

