# Two pins that were green over the hole they were written for (ranger-base-mxbq0)

The verify at ranger-base-6rj8w filed two findings against two closes from
the day before, and they are the same defect in two places: a pin that
reads a thing it can only see when somebody has already thought of it.
2026-11-13, this box (linux 7.2.15 x86_64, go1.27.3, git 2.56.0, 16 cores),
branched at 34bbe053. Every mutant below was run through `go test -overlay`
with the tracked tree never written.

## 1. A remedy sentence is a closed set, or it has no reader at all

`ranger-base-2nj35` gave check 3's two remedy constants their first test
readers, and `ranger-base-ckcly` had done the same for check 4's. All four
readers asked one question — is this phrase STILL in the sentence — over a
row list covering only the nouns the sentence already named. So the
drop-a-promise direction reds and the ADD-a-promise direction has no reader:
a clause appended to a remedy is a way through nobody measured, and a new
promise has by construction no phrase for such a pin to be missing.

MEASURED, two mutants, each coherent and each displacing no phrase any pin
greps for:

- `IdentityWayThrough` (`internal/posse/visibility.go`) += "Either arm also
  clears if you stage the file under docs/, which this check does not scan."
- `IdentityMessageWayThrough` += "If you would rather keep the wording, put
  the identifier in the commit TRAILER instead: trailers are not scanned."

Both PASSED every check-3 and check-4 pin in the tree, in all three arms.
And the second is false, measured rather than argued: a `visWall` commit
whose only identity literal is in a trailer line, over a clean staged file,
is refused with the header "an operator identity literal in the commit
MESSAGE" — `IdentityRule`'s own "every line of the COMMIT MESSAGE". A
refused writer sent there is refused again, which is ranger-base-68631
finding 6 exactly: a way through that does not go through.

The defence in `checkthreeremedy_qa_test.go`'s head comment was that a
reword re-promising what the rule refuses "has to displace one of them to be
coherent". That is the sentence the two mutants falsify, and it is now
replaced by what they measured.

`internal/posse/remedyclosedset_test.go` is the third direction, one pattern
over both checks: `TestQARemedySentencesAreClosedSets` holds all four remedy
constants to their whole text. What it buys is not the catch but the STEP —
an edit to any of them reds until its author has gone back to the file that
holds the behavioural rows and measured the clause they wrote. The file is
untagged, so an edit to a check-3 remedy reds in a bare `go test ./...` and
not only in arm 3, where the rows live.

Both halves collapse whitespace first, which is `remedyPromises`'s own
reason: these sentences are hard-wrapped at the width a terminal refusal
prints, so a phrase worth naming straddles a line break. Re-wrapping a
refusal is not drift; re-wording it is. `remedyPromises` moved to the new
file and check 4's two readers now use it instead of a plain
`strings.Contains` — the house precedent is the fixed one, since
ranger-base-2nj35 matched the broken one in good faith.

### What the mutants said after

| mutant | arm 1 before | arm 3 before | after |
|---|---|---|---|
| append a clause to `IdentityWayThrough` | GREEN | GREEN | RED |
| append a clause to `IdentityMessageWayThrough` | GREEN | GREEN | RED |
| append a clause to `VulnPrivateWayThrough` | GREEN | GREEN | RED |
| append a clause to `VulnPrivateMessageWayThrough` | GREEN | GREEN | RED |
| reverse the rename clause (gwart's M6, the drop direction) | GREEN | RED | RED, both arms |
| a pure RE-WRAP of `IdentityWayThrough`, no word changed | GREEN | **RED** | GREEN |

That last row is a regression fixed in passing and in the same class. The
row test asserted the refusal OUTPUT with a raw
`strings.Contains(out, "generalize whatever")`, and the refusal prints the
sentence hard-wrapped with the break falling inside that phrase — so a pin
whose subject is the wording was a pin on the wrapping. It reads
`remedyPromises` now, and the stated cost of the closed-set reader ("a
re-wrap is not drift") is true of the tree and not only of the new file.

## 2. A render pin that could not see a wall kind its fixture omitted

`ranger-base-ihnnl` shipped `TestWallWatchRenderRecordsEveryWallAndNoOpening`
as a `reflect.DeepEqual` against a four-element literal, over a fixture
carrying only the four kinds already recorded. MEASURED: a render whose
watch list came back EMPTY passed the pin named "records every wall"
unchanged, because the fixture asked for none of the kinds that render had.
A pin that can only see what somebody has already thought of is why
ranger-base-w6ip0 was found by hand; a literal of six — which
ranger-base-y4cm6 has since made it — leaves the sixth-to-seventh step the
same silent one.

`bwrapArgsWatched`'s doc comment was the other half: it states a taxonomy
that reads complete while a kind added with no `watch()` call would leave it
reading complete still.

The fix is the shape this tree already uses where a class must stay total
(`make corpus-check`, pkgcensus, treewidedoor): derive the expectation from
the ARGV. `TestWallWatchEveryRenderedMountIsAWallOrANamedOpening` walks the
rendered argv and holds every destination the render mounts over to being
either in the watch list or in an opening set computed from the render's own
INPUTS, and holds every watched path to being a destination the argv really
carries. The render cannot do this itself and the comment is right that it
cannot — a destination does not say whether it is a wall or an opening — but
the pin was handed the lists the destinations came from, so it can.

Three doors, and each one reds on a different mistake:

- a mount with no `watch()` call: unclassified, so it reds. A new INPUT list
  that renders an opening reds too, until its author says so in `wwOpenings`
  in as many words.
- a `watch()` call with no mount: `posse wall-watch` would read the HOST's
  mount at that name as the seat's wall, and file a P1 naming a wall that
  was never put up.
- an argv OPTION the pin does not classify: a fatal, never a skip. A mount
  flag nobody classified is the hole this replaces.

`TestWallWatchFixturesCarryEveryCarveOutField` holds the other end, because
a derivation over an argv says nothing about a shape the inputs never asked
for: every field of `SeatbeltCarveOut` must have a value in one of the
renders here, so a field added to that struct reds until somebody gives it
one and says which side of the rule its mount falls on. `carve.Tmp` gets a
render of its own — it mounts over `/tmp`, and every path a `t.TempDir`
fixture has is under `/tmp`, so in one fixture `reopens` would skip the
denies and the ssh tmpfs that no write grant re-opens and the render would
be measuring almost nothing.

The old literal stays, now eight long: the derivation holds the SET, the
literal holds what each member is called and when it is mounted. A wall
recorded under the wrong kind is a bead that names the wrong wall.

### What the mutants said

| mutant | before | after |
|---|---|---|
| drop `watch(WallWatchReadMask, f)` (the credential masks) | RED | RED |
| drop `watch(WallWatchReadMask, d)` (the ssh tmpfs) | RED | RED |
| a new wall mount with no `watch()` call | **GREEN** | RED |
| an argv option nobody classified (`--size 4096`) | **GREEN** | RED |
| a `watch()` call with no mount | RED | RED |
| a new field on `SeatbeltCarveOut` | **GREEN** | RED |

The two GREEN-to-RED rows in the middle are the class the bead was filed
over; the field row is the fixture held total from the input end.

## What this did not change

Neither finding is a regression, and both closes they came from met their
DONE WHEN. ranger-base-2nj35 really did give two unread constants a reader
and really does red the drop direction (gwart's M6 reproduced
independently). ranger-base-ihnnl's render really does record the four kinds
it was written for. What was wrong in both cases was what the pin PROVES
against what its name claims — which is the thing that makes a finding live
by ADR 0006 §6, and the reason neither needed a `-t bug` bead of its own.
