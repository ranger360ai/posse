# A reflection over FIELDS cannot hold an arm a zero value selects (ranger-base-zkuwa)

*2026-11-14 · holden · verifying ranger-base-ao8fs, ranger-base-99mvt,
ranger-base-73752 and ranger-base-n2veh*

All four closes verify on the deliverable each was filed for. The fragment is
about the one shape among the escapes that is structural rather than local,
and about the two cheap measurements that found two of the others.

## The class: "widen the reflection" closes the class only upwards

`ranger-base-99mvt` fixed a real hole — `ranger-base-a544g` had added a FILE
private bind to `bwrapArgsInto` and no wall-watch fixture rendered one, so the
`watch(WallWatchPrivate, pb.Dst)` on that arm was held by no pin. The fix gave
`wwEveryShape` entries for the arms and then went one level deeper, which is
the right instinct: `TestWallWatchFixturesCarryEveryCarveOutField` now
reflects over `PrivateBind`'s own fields and demands each be **non-zero in
some render**, so a field added to that struct tomorrow cannot be a mount
shape no fixture renders.

MEASURED 2026-11-14 at 72fe7738, that half works, in both directions: drop
`File: true` from the fixture and it reds naming `PrivateBind.File`; drop
`ReadOnly: true` and it reds naming `PrivateBind.ReadOnly`; add a field to the
struct and it reds naming that field. Three kills, each with the right
message.

**And the loop still has an arm it cannot reach.** The private loop selects
four arms, each with its own `watch` call:

| arm | selected by | watch at |
|---|---|---|
| FILE bind | `pb.File` | bwrap.go:617 |
| in place | `pb.Src == pb.Dst` | bwrap.go:641 |
| empty tmpfs | `pb.Src == ""` | bwrap.go:652 |
| dir copy | neither | bwrap.go:666 |

Comment out one `watch(WallWatchPrivate, pb.Dst)` at a time, `-run
TestWallWatch` (1.8s an arm, golden copy, md5 either side):

```
bwrap.go:617 (FILE)      2 FAIL
bwrap.go:641 (in place)  2 FAIL
bwrap.go:652 (TMPFS)     ok 1.789s      <- the survivor
bwrap.go:666 (dir copy)  2 FAIL
```

The survivor against the whole of arm 1, unfiltered, is `ok 174.865s`, and of
the 330 `internal/posse` test files tagged `posse_arm2` or `posse_arm3` none
mentions `WallWatch`, `wallwatch`, `bwrapArgs` or `PrivateBind` — so arm 1 is
the whole of the coverage and that watch is held by nothing.

The reason is the generalisation's own shape: **the tmpfs arm is selected by a
field's ZERO value**, and a reflection that asks for a non-zero value can
never demand it. `Src: ""` is both "no source" and "this arm", so the axis the
pin walks is blind to it by construction. A pin over a struct's FIELDS holds
the arms a value selects; it says nothing about the arms an absence selects,
and the fixture comment that calls them "the three arms" is how that went
unnoticed. What wants holding is the ARM count — the number of `watch(` sites
in the loop against the number of fixture entries that select distinct arms —
and that is a different pin, not a wider version of this one.

The instance is live (a codex or grok seat's private tmpfs over
`shell_snapshots/` and `daemon/run/` is rendered by `bwrap.go:2069` and
watched by no pin, so a host-side detach goes unnoticed) and is filed with its
repro: **ranger-base-4tsa7** finding 1.

## Two cheap measurements that earned their turn

**Gut the rig, not only the code.** `ranger-base-73752`'s close added a
read-only fake that refuses a mutating statement inside a read transaction —
good mechanism, and MEASURED to buy a real extra kill. Nothing proves it still
works: make its clause regex match nothing, or make its read transaction not
read-only, and all 829 tests pass. A fake's refusal is a branch like any
other, and "the mutant I ran reds" is not the same as "a committed test holds
it" (ranger-base-4tsa7 finding 2).

**Re-aim a sed that changed nothing.** One arm of mine read as a survivor and
was not: the sed had hit the line above the one carrying the token and the md5
was unchanged. Printing the md5 and the touched line per arm is what caught
it, and it is why the finding list says which arms were re-aimed. A no-op edit
scores as a survivor every time.

## Not closed here

- ranger-base-4tsa7
