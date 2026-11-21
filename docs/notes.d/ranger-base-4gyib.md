# ranger-base-4gyib — the seat store pin's wide-grant shape is unrenderable under ADR 0100, and the venue is the only place that says so

`wall pins` was red in ADR 0097 D1's venue from 2026-11-21 02:45 onward. This
is what the red was, why an uncaged `make test` and every seat's `make test`
read green over it, and what the window's own bisect says about when it
started.

## The red

One member, `TestLiveBwrapSeatStoreRefusesRewritingASharedObject`, at the
shape its own head comment calls `with <common> granted whole` — the
store-of-record case ADR 0099 argues about, written as the writable set plus
`<common>`:

    BwrapArgs refused the seat store render: bwrap cannot bind
    <repo>/.git/worktrees/s-1 in place: <repo>/.git/worktrees lies inside a
    write grant of this render and is no mount of it … (ADR 0100 D1/D2(b))

The refusal is right and the shape was wrong. A grant covering `<common>`
whole puts `<common>/worktrees` strictly inside it with nothing mounted
between, and the seat's own per-worktree git dir — `sessionGitGrants`' `own`,
`<common>/worktrees/<name>` — is a grant under THAT, bound `--bind p p`.
bwrap re-resolves a bind path at `mount(2)`, so a leftover of this seat
renames the loose component, plants a link at the name, and the next launch
mounts whatever it points at, read-write, with no race to win. The same
component is loose for the store's SIDE bind destination
(`<common>/worktrees/<name>/seat-objects-base`), which is ADR 0100 D4(a)'s
arm over the same path.

## The fix

The shape carries `<common>/worktrees` as well. Granting it is how a test
spells "bound in place": shape 1 binds every write grant `--bind p p`, so the
grant IS the mount that pins what is under it — the fix ADR 0100 prescribes
for a miss, and never an exemption. `bsWideGrantRefusedUnpinned` is the same
set WITHOUT it, asserted refused and asserted to name that component, so the
entry cannot be read later as a widening somebody added to make a test pass.

Nothing in the shipped renders changed and nothing needed to:
`TestQAWallPinnedSourceEveryRenderPinsItsInPlaceBindsToo`'s census still says
no render this package makes has a grant nested two or more components inside
another with a non-mount component between them. The wide grant is the live
pin's own synthesis of a shape ADR 0099 reasons about, and under ADR 0100 that
shape renders only with the component between pinned.

## MEASURED 2026-11-21 — the stretch, bounded

This box: Linux 7.2.15-1-omarchy, bubblewrap 0.13.1, go1.27.3, 16 cores.
Each reading is `git archive <sha> | tar -x` into a scratch tree and
`TMPDIR=$HOME/tv go test -count=1 -run
'^TestLiveBwrapSeatStoreRefusesRewritingASharedObject$' ./internal/posse` —
the venue's shape, which is a TMPDIR that is not under `/tmp` (see below),
and the cheap way back to it.

| landing | verdict | first failure |
|---|---|---|
| 4b3013cd ranger-base-ltmyh (2026-11-15) | ok | — |
| dc1d70cc ranger-base-1qi7c | ok | — |
| da79a784 ranger-base-bncbk | FAIL | `prep: … refuses to prepare <…>/seat-objects-base/.posse-wall-anchor` |
| dfe4c9ad ranger-base-gn050 | FAIL | the same prep refusal, which masks the next row |
| 7d72371a ranger-base-dw2du | FAIL | the same |
| 3c16c00c (HEAD of the window) | FAIL | the unpinned wide grant, above |

So the stretch began at `bncbk`, four landings before the one it was
attributed to, and it is TWO reds in a row in one pin: `bncbk`'s prep host
fence first — closed at 464015e4 by ranger-base-cmedf, which is what
uncovered the second — and then the grant refusal, which arrives with
`gn050`'s D4(a) destination arm and is joined by `dw2du`'s D2(b) in-place arm.
ADR 0088 D5's attribution rule worked exactly as its own text says it does:
every seat in the window ran the doors on its own base and was green there,
and the newest landing is only the one whose tree first held them all.

**Which arm refuses it, MEASURED by mutation:** putting the `m.Src == m.Dst`
skip back into `bwrapRefuseUnpinnedSource` — dw2du's whole change — leaves
the same set refused, by D4(a)'s destination arm over
`<common>/worktrees/<name>/seat-objects-base`. So `bsWideGrantRefusedUnpinned`
measures "the unpinned wide grant is refused and names the loose component",
which is the claim worth holding, and not one arm of ADR 0100 over the other.
The mutation that reds the pin as filed is dropping `<common>/worktrees` from
the shape's grants.

## Why nothing outside the venue could see it

`bwrapRefuseUnpinnedSource` and `bwrapRefuseUnpinnedDst` both stand down when
`bwrapWallRoot()` is empty, which it is when this uid's home lies under
`/tmp`: on such a box `/tmp` is granted unconditionally, every private source
is already in the shared directory, and there is no seat for the wall root to
protect. Every fixture in internal/posse is that shape — `TestMain`'s home is
a temp dir — so ADR 0100's two refusals are **dormant for the live pins under
an operator's uncaged `make test`, and live in the venue**, whose TMPDIR is
`<state>/landed-check/run-<n>/tmp` under the posse state dir. MEASURED, same
tree, same command: `TMPDIR` unset → `ok 0.13s`; `TMPDIR=$HOME/tv` → the
refusal above.

That is why ADR 0097 D6's control is not equivalent to the venue for this
family of rules, and it is the third venue-only red in two days whose cause is
"the venue's TMPDIR is not `/tmp`" (ranger-base-cmedf and ranger-base-18brx
are the other two). The pins written FOR these rules put their home under
`wallUngrantedDir` for exactly this reason; the live pins do not, and a
fixture home placement that makes the two venues agree is
ranger-base-bx9wj's.

## The two notes the green verdict carries

    note: FAIL TestLiveWallUnixLeavesCrossDirectoryRename … bind: invalid argument
    note: FAIL TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched … bind: invalid argument

Both are a socket path over `sun_path`'s 108 bytes: those two pins bind inside
`t.TempDir()`, whose name is the test's, and the venue's TMPDIR spends 52-62
of the 108 before the name starts. Pre-existing, not a member (they print no
ADR 0097 D4 marker), and the body reports them in a `note:` and exits 0 — "a
red for `make test` and not for this check". `wallchecklive_test.go`'s own
byte budget already names this class for the two gnupg-socket members;
`shortTempDir` is the package's answer. ranger-base-vbp5a.

## Verification

    bash docs/adr/0097-wall-venue.probe.sh venue+check

before: `rc=1 wall=100s load=6.78`, `wall pins RED: 1 of 46 member(s)
FAILed`. after: `rc=0 wall=101s load=10.49`, `wall check: green, 46 pins
measured`, with the two notes above. The filtered pin set in the venue's
shape (`TMPDIR=$HOME/tv`, a root short enough for the sockets) is `ok 79.6s`,
and the uncaged control is `ok`.
