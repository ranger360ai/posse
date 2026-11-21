## Shape 5's directory masks cover the name before the credential exists (ranger-base-bncbk)

bwrap's step 5a (`~/.ssh`) and step 5b (the rex tsnet node's state dir) mount an
empty tmpfs over a host directory. Both used to `continue` on a directory that
was not there: no tmpfs, no watch, no prep. The comment's stated reason was "a
seat cannot create it either — nothing grants its parent", which is true and
beside the point, because the **host** creates it.

Fresh box, no `~/.ssh`, seats running. The operator runs `ssh-keygen` to set up a
push key; it mkdirs `~/.ssh` and writes `id_ed25519`. Every live bwrap seat then
reads that private key through `--ro-bind / /`. No mount ever existed at the
name, so nothing detached and `posse wall-watch` said nothing — the exposure
lasted until each seat ended. 5b is the same story one runtime along: a `rex`
that first starts during a seat's life writes a tailnet identity, and a copy of
that identity outlives the seat and the box.

This is ADR 0080's snapshot-at-launch residual at the two arms 5c's mkdir
(ranger-base-a8qfg) never reached. The seatbelt render never had it: a subpath
read deny is a statement about a PATH and covers a directory created later.

**MEASURED 2026-11-21**, in the wall venue on this box (linux 7.2.15-1-omarchy,
bwrap 0.13.1), by `TestLiveBwrapAbsentSSHDirMasksAKeyTheHostWritesAfterLaunch`:
a seat launched with no `~/.ssh` at all, told to wait until its namespace was up
before the host acted, then handed a key the host wrote at `~/.ssh/id_ed25519`
*after* that. With 5a reverted to the skip, the seat reads
`PRIVATE-KEY-WRITTEN-AFTER-LAUNCH` in full; with the fix it reads nothing, and
the pin's unmasked control arm reads the key, so the green is a wall and not a
probe that could never read. The bead was filed by code reading on
ranger-base-lin9y's verify, on the grounds that nested bwrap cannot run from a
caged seat (ADR 0096 D3) — true behind the wall, and this seat happened to be
in the venue, where the gate reads "measuring".

### What landed

Both arms now do what 5c does, and the shape of shape 5 is one shape again:

| step | absent at launch | present and empty | present and full |
|------|------------------|-------------------|------------------|
| 5a `~/.ssh` | `mkdir -p -m 700`, anchor, tmpfs, watch | anchor, tmpfs, watch | tmpfs, watch (+ the allowlist bound back) |
| 5b masked state dir | the same, **only** at `rexNodeStateDirNative` | anchor, tmpfs, watch | tmpfs, watch |
| 5c credential store | `mkdir -p`, anchor, tmpfs, watch | anchor, tmpfs, watch | tmpfs, watch (+ allowlist) |

`0700` is not a hardening choice, it is the mode the directory would have had:
`ssh-keygen` makes `~/.ssh` that way, and rex's tsnet node does
`os.MkdirAll(rootPath, 0700)` — tailscale.com v1.104.4 `tsnet/tsnet.go:863`, the
version `/usr/bin/rex` links, read 2026-11-21 — which is also how its missing
parents are made. So a posse-made directory at either name is indistinguishable
from the one it preempts, parents included, and `PrepMkdirPrivate` is a second
PrepKind rather than a mode field because these steps **print**: `mkdir -p -m
700 ~/.ssh` is a line an operator can type and compare.

The same reading settles the bead's other question: tsnet `MkdirAll`s its root,
`Stat`s it for `IsDir`, and reads no directory listing of it at all
(`tsnet.go:863-869`), so a pre-existing empty `0700` dir with an anchor beside
it is inert to the node that fills it. The state file is a named
`tailscaled.state` under it (`tsnet.go:966`), not an enumeration.

**5b creates only the native path**, and that asymmetry is the point.
`rexNodeStateDirs` deliberately lists both OSes' XDG state defaults under both
homes, because a mask wider than its runtime's own resolution is the safe
direction for a mask. A *create* has to be exactly that resolution, or posse
invents `~/Library/Application Support/rex/tsnet` on linux and then masks and
anchors a directory nothing will ever write. So the producer says which one it
is (`SeatbeltCarveOut.DenyReadDirMake`, `rexNodeStateDirNative`), which is
`DenyFile`/`Seed`/`Anchor`'s rule at step 3 applied one shape along.

It resolves from `$HOME` and never from passwd, which is the one place it
differs from the mask's two-home list: the xdg library rex links reads
`os.Getenv("HOME")` else `"/"` and touches the passwd database not at all
(github.com/adrg/xdg v0.5.3, `internal/pathutil/pathutil_unix.go:14`, read
2026-11-21). That reading also gives rexnode.go's ASSUMED darwin default a second source —
`paths_darwin.go:32` defaults `stateHome` to `~/Library/Application Support` —
but it is a reading of the library and not of a darwin box, so the label there
stays ASSUMED and the open bead under "Not closed here" keeps the measurement
ask.

### The anchor class has no edge left

`bwrapAnchorStep`'s doc used to carry "THIS LAUNCHER IS THE WHOLE OF THE EDGE",
with 5a and 5b named as the arms outside the class because posse neither created
nor emptied their directories. That edge did not survive its own residual: the
skip it justified was the hole above. Both arms make their directory now, which
puts them in the class on the old definition and leaves the definition nothing
to exclude. Every arm that mounts a tmpfs or a bind over a host **directory**
calls that one function; the only paths that get no anchor are the ones nothing
can rmdir (a file) and the ones the seat writes through (a dir bound over
itself, the Hide mounts).

`TestQAWallAnchorCoversEveryWatchedDirectoryMountOrPricesIt` is where the old
edge was spelled as a branch, and that branch is gone. Its read-mask floor is
now **per producer** rather than one total, because one total was exactly how
the pin passed while grading neither 5a nor 5b: three credential stores satisfied
a floor of 2 on their own. Five read-mask tmpfs mounts and eight private copies
in that fixture, MEASURED 2026-11-21 (three and seven on 2026-11-15).

A `bwrapAnchorStep` bug fell out of putting a real anchor in that render's way:
it emitted its `PrepSeed` for any directory, including one whose anchor an
earlier launch had already left there. That breaks PrepStep's own rule — a step
names a path that is NOT there, and `prepRequire` reads the list's emptiness as
"this host already has the whole wall" — so a reader's probe would have abstained
over an anchored host forever. Nothing measured it before, because the one pin
that holds the rule runs over a fixture whose every credential store sits inside
the blanket `/tmp` write grant and so renders no anchor at all.

### The precondition: a test's prep stays inside the fixtures

`runBwrapPrep` is the one part of the wall that happens OUTSIDE it — the
launcher is not in the namespace, so every PrepStep is a write on the host, in
the seat's name, at a path the **render** chose. Right for a launch, wrong for a
test, and two of the render's inputs do not move when a fixture sets `$HOME`:

- **`liveSSHHomes`' passwd half.** `user.Current().HomeDir` comes from the passwd
  database and ignores `$HOME`, so a test whose fixture home is a `t.TempDir()`
  rendered `SSHDir: [<the operator's real ~/.ssh>, <the fixture's>]`. MEASURED
  2026-11-21 by dumping readerprobeprep_qa_test.go's own fixture: both there,
  and the real one is the one the watch list named.
- **`XDG_STATE_HOME`**, which `rexNodeStateDirs` reads straight out of the
  environment and no test fence unset.

With 5a and 5b skipping, neither mattered. With them creating, every test in
internal/posse that renders a whole wall would plant `.posse-wall-anchor` in the
operator's real `~/.ssh` and in rex's live tsnet dir — ranger-base-lin9y
measured that happening and the files were removed by hand.

Two halves, and clihomefence.go's pattern because three test binaries need this
and test code cannot be shared across packages:

1. **The fix** rides in `FenceCLIHomes`: the passwd home is a package var now
   (`sshPasswdHome`) and the fence points it at `$HOME`, so a fixture that moves
   `$HOME` moves it; `XDG_STATE_HOME` is unset. A var and not an environment
   variable, deliberately — a seat can set an environment variable and shed its
   own mask.
2. **The census pin** is `prepHostFenceRefusal`, in `runBwrapPrep`: a step whose
   path is under none of the declared roots refuses, naming the step and the
   roots. It holds the fact rather than a list of the tests that render, so a
   test added tomorrow, or a producer that starts reading a third host input,
   reds here instead of writing into somebody's live area. Off until a binary
   declares it — a launch's prep must write where the render says — with
   `TestQATheBwrapPrepHostFenceIsUpAndRefusesAStepOutsideIt` as the hard arm
   that stops it becoming a no-op.

The fence itself has two arms, which is `sbAssertHomeIsAFixture`'s shape at this
mechanism. The **roots** are the hard arm — the binary's own temp home plus
`os.TempDir()` and `/var/tmp`, the last because `wallUngrantedDir` cuts its
fixtures there on purpose (a `t.TempDir()` is inside a blanket temp grant, where
`reopens` skips every one of these masks). They need no passwd database and no
privileges, so they cannot quietly become a no-op. Their blind spot is a box
whose real home is itself under a temp root, where "under a fixture root" would
be true of the one directory the fence exists to protect — so the second arm is
a **keep out** at the passwd home, read at fence time and never through
`sshPasswdHome`, the seam the fence replaces, so the arm cannot be moved by the
thing it is checking. It refuses whatever the roots say, and the pin drives it
at the real `~/.ssh/.posse-wall-anchor` — the write lin9y measured.

The two controls are independent, which the mutation battery shows: with the
passwd-home fix reverted and the fence left up, the fence refuses
`/home/posse/.ssh/.posse-wall-anchor` by name and the pins red on the refusal.

### Mutation-checked

MEASURED 2026-11-21, one mutation at a time, each red in the pin that owns it:

| mutation | red |
|---|---|
| 5a skips an absent `~/.ssh` again | the ssh rows of `TestQAWallAbsentReadMaskDirIsMadeAnchoredAndWatched` |
| 5b skips every absent masked state dir again | its rex rows |
| 5b creates the OTHER OS's XDG default too | its other-OS rows |
| the anchor step is re-emitted over an anchor already there | `TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere` + the re-render row |
| the prep host fence is never put up | `TestQATheBwrapPrepHostFenceIsUpAndRefusesAStepOutsideIt` |
| the passwd home is left reading the operator's real home | the fence's own refusal, in two pins |
| 5a skips an absent `~/.ssh` again (the LIVE arm) | `TestLiveBwrapAbsentSSHDirMasksAKeyTheHostWritesAfterLaunch` reads the key in full |

### Not closed here

- ranger-base-1mxhx
- ranger-base-3cjzs

