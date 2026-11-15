## The anchor's class, and where `posse upgrade` stages its copy (ranger-base-ua625)

Two findings from the batched verify at ranger-base-pxprm, both LIVE, neither in
the QA lane. They share nothing but their shape: a wall whose premise was written
down as a LIST of names, where the thing that made those names dangerous was a
property any number of other names also had.

## Finding 1 — the anchor was seven names; the class is "a mount point this launcher leaves empty"

`SeatbeltCarveOut.Anchor` was filled from `claudeSweptConfigDirNames` and from
nothing else, so the only mount points bwrap kept non-empty were claude's seven
swept config dirs. The list is not mis-scoped — it matches the host remover, and
ranger-base-pxprm re-measured that within the verify: across a sweep that took
all seven, the other fourteen EMPTY members of `claudeExecConfigDirNames` were
untouched, and those are dirs claude loads code from, where an empty dotfile is
not measured inert. That exclusion stands and is now priced in
`Anchored`'s own doc.

What was missing was the sentence above the list. The property is emptiness:

> A read-only bind, or a tmpfs, over an EMPTY host directory is a mount over a
> name any host process can `rmdir`, and on Linux an `rmdir` on the host
> detaches the mount in every other namespace.

MEASURED 2026-11-14 (bwrap 0.13.1, Linux 7.2.15-1-omarchy, a live bwrap child
waiting on a signal file while the host acts):

| mount over                   | host removal | seat's mountinfo | then                              |
|------------------------------|--------------|------------------|-----------------------------------|
| `--ro-bind` over an EMPTY dir| rc=0         | 1 → 0            | the seat's write lands in the host's dir |
| the same, one empty file in  | ENOTEMPTY    | 1 → 1            | write refused                     |
| `--tmpfs` over an EMPTY dir  | rc=0         | 1 → 0            | the seat READS a file the host wrote there after launch |

So the mechanism already generalized; only the list was scoped. Two arms of the
render were in the shape and anchored by nothing:

- **The credential-store mask (step 5c).** `~/.bob` exists on this box, holds
  zero entries, and `credentialStoreReadMaskDirs` returns it for every claude
  seat. The arm MAKES the directory when it is absent, deliberately, so that a
  login during the seat's life lands inside the mask — which puts it squarely in
  the class. `~/.codex` and `~/.grok` hold one entry each, which is all that
  keeps them out of it.
- **The private binds (step 2).** Evidence on ranger-base-dtzgt: the
  six host dirs this render binds private copies over were all ABSENT before a
  launch and recreated empty by the prep in the same millisecond as the anchored
  deny dirs (`stat %w`, birth 00:01:38.0331–.0334). The seat's content goes to
  `~/.cache/posse-seat/<session>/claude/*`, never to the host dir, so the
  launcher leaves them empty for the seat's whole life. (That source path is
  the one measured; ADR 0100 D1 landed on `main` while this bead's branch was
  blocked and moved every bind source that differs from its destination to
  `~/.cache/posse-wall/<session>/`, which no render grants — the DESTINATIONS
  this bullet is about, and the emptiness that makes them removable, are
  unchanged.) The same `rmdir` that
  takes `mcp-skill-archives` off would take one of these off, and the seat's
  session, history and file-history writes would then land in the shared host
  dir every other seat and the operator's own claude read.

Both are anchored now, through one `bwrapAnchorStep`. The dir path of
`bwrapPrivateArgs` was collapsed to a single tail so its four arms share one
anchor call the way they already share one `watch` call — a per-arm line is a
line the fifth arm forgets, which is how `watch` learned the same lesson under
ranger-base-4tsa7.

**And the anchor no longer follows a link.** The deny arm read its `dir` answer
out of `bwrapStat`, which is `os.Stat`, so a link standing at an anchored deny
name answered for its target and the seed wrote
`<link target>/.posse-wall-anchor`. `bwrapAnchorStep` lstats. A link or a file at
the name gets no anchor and no refusal: nothing `rmdir`s either, so there is no
mount to lose.

**The edge of the class is "what this launcher leaves empty".** Three arms mount
over a host directory and are NOT anchored, each for the same reason stated at
its own site:

- **Steps 5a (`~/.ssh`) and 5b (`DenyReadDir`).** Both skip an absent directory
  outright, on the stated ground that nothing grants its parent, so posse
  neither makes nor empties them, and a seed there would be posse writing a file
  into the operator's ssh dir. Residual: ranger-base-lin9y.
- **The private bind's in-place arm** (`Src == Dst`), which is claude's
  `projects/`. A directory bound over itself is the host's live directory and
  the seat writes *through* to that name, so the launcher never leaves it empty.

The first draft anchored all three, and both halves were caught by pins rather
than by reading. 5a/5b: `TestQAWallAnchorKeepsClaudesSweptConfigDirsFromAHost
Rmdir` runs the prep over a render whose `SSHDir` comes from `liveSSHHomes()` —
which includes `user.Current().HomeDir`, the REAL home, not the fixture's — so
the run wrote `.posse-wall-anchor` into `/home/posse/.ssh` and
`/home/posse/.local/state/rex/tsnet`. Both files were removed by hand.
In-place: `TestLiveWallClaudeProjectsANonOwnCwdRunsTheRealClaudeAndWritesNo
Transcript` reported "the real claude changed projects/", and the change was
posse's own prep.

That `~/.ssh` hazard outlives this fix, so it is worth naming for whoever picks
up ranger-base-lin9y: the mutation row below that anchors 5a/5b REPRODUCES it,
and wrote `/home/posse/.ssh/.posse-wall-anchor` a second time when the battery
was re-run. Remove the file after running that row, or fix the pin's fixture
first.

**The anchor refuses a rename as well as an rmdir,** which is more than
ranger-base-v84cn claimed for it. `rename(2)` over a directory needs the target
EMPTY, so the one empty file answers ENOTEMPTY to both callers. MEASURED here,
not reasoned: `TestLiveWallWatchKillsASeatWhoseWallComesOffAndFilesOneBead`
detaches a private bind by renaming a fresh directory over its mount point, and
that rename began failing. Its helper now removes the anchor first — which is
exactly the remover shape `Anchored` already names as the residual the anchor
does not close — and `TestQAWallAnchorKeepsClaudesSweptConfigDirsFromAHostRmdir`
holds the refusal itself beside the rmdir one.

`TestQAWallAnchorCoversEveryWatchedDirectoryMountOrPricesIt` is what keeps the
class from going back to a list: it walks the render's WATCH list — the mounts
this render calls walls — and requires each directory mount to be anchored or
priced by its producer. A fifth arm that mounts a directory and watches it falls
into the switch's `default` and reds. It is the anchor half of what
`TestWallWatchEveryRenderedMountIsAWallOrANamedOpening` does for the watch list,
and it holds both ends the same way: the argv says what was mounted, the
carve-out says what each path is.

Its first draft asked the DISK which mounts were directories, by lstatting each
bind's source — and a private copy's source is a path the PREP makes, which has
not run at render time. All seven copy binds lstat'd ENOENT, were scored as file
mounts, and the arm the pin exists for graded nothing while the pin read green.
What caught it was a floor on how many mounts the pin REQUIRED an anchor of, as
against how many it priced out; the classification reads the carve-out now, and
the floors stayed.

## Finding 2 — the upgrade's private copy sat inside a blanket seat write grant

`upgradePrivateCopy`'s own comment claimed "the new repository's store is under
this run's mktemp -d, which no seat is granted". At bwrap that is true and is
stated where it is true (step 1c: the seat's `/tmp` is a private bind, "so no
seat writes where uncaged builds stage what they execute"). At the SEATBELT
mechanism it was false both ways round. MEASURED 2026-11-14 through the
production functions:

```
seatbeltTempGrants("darwin", "/var/folders/ab/cd1234/T")
  = [/var/folders/ab/cd1234/T  /private/tmp  /tmp]
seatbeltTempGrants("darwin", "")
  = [/private/tmp  /tmp]
os.MkdirTemp("", "posse-upgrade.") with TMPDIR unset
  -> /tmp/posse-upgrade.453727933, mode 0700, inside the grant /tmp
```

With `TMPDIR` set, `MkdirTemp` uses it and `TMPDIR` is grant 1; with it unset the
copy is in `/tmp` and `/tmp` is grant 3. There is no setting of `TMPDIR` that
puts the copy outside the three, and `seatbelt.go:1275`'s own comment calls them
"the BLANKET temp grants every rendered profile carries … every seat on the box
gets the same three, whatever its PID, its cwd or its persona says". Seats and
the uncaged `install_delegate` run as one uid, so mode 0700 separates nothing.
So a seat could write the private copy's working tree and object store for the
whole of the `make test` window — which that bead priced at more than twenty
minutes — and `make test` runs out of that copy, UNCAGED, as the verb's caller.
Re-hashing on the way in closed what the copy is MADE of (ranger-base-la61c);
it said nothing about who can write it afterwards.

**The copy did not move; the grant was narrowed.** The obvious fix is a copy
under posse's state dir, and it is the wrong one here: this tree's suite moves
with `TMPDIR` in three ways that all read as "the commit is red" (MEASURED
2026-11-13 at 983ae5d, ranger-base-6ca29, quoted in `upgradeRunPlan`), one of
them being "under `$HOME`", where `AbbrevHome` renders the path as `~/…` and
every test asserting its own absolute temp path reds. So the copy stays in the
temp root where the suite was measured green, one fixed level down —
`<os.TempDir()>/posse-upgrade/run.XXXX` — and `upgradePrivateDenied` puts that
one name under a deny in every rendered profile, one per blanket grant. It is
rendered whether or not an upgrade is running, which it has to be: a seat's
profile is fixed at the seat's launch, and the verb picks its directory later.
A per-run random name could not have been denied at all.

MEASURED 2026-11-15, this box, linux, TMPDIR unset: the rendered profile carries
`  (subpath "/tmp/posse-upgrade")` in its deny block, and the pin reads that line
rather than only the carve-out.

A fixed name under a world-of-one-uid temp root is also a name something could
plant a link at, so the verb lstats it and refuses rather than following — which
is the one thing the random name did have, and the pin's last row.

## Four fixtures carried a reduced copy of the launcher's prep loop

`bwrapseatstorelive_test.go`, `bwrapgitrefslive_test.go`, `wallwatchlive_test.go`
and `wallwatchbackstopwiring_linux_test.go` each ran the launcher's half of the
render themselves: two by a hand-written switch over `PrepMkdirAll` and
`PrepTouch` with `default: t.Fatalf("the render asked for a prep step this test
cannot run")`, two by asserting `len(prep) != 0` was impossible. Both shapes
predate D18, which made `runBwrapPrep` production code. The new anchor is a
`PrepSeed`, so all four refused a step a real launch makes without comment.

They call `runBwrapPrep` now. A reduced launcher measures a wall the launcher
does not put up, and the tripwire was asking each test to re-learn the step list
by hand — which is the same defect one level up as the anchor's own.

## Mutation-checked

Each mutation applied to the working tree, the pins run, the files restored
byte-identical:

| mutation | reds |
|---|---|
| the credential-store mask (5c) loses its anchor | the class pin, the unit pin |
| the private-bind anchor dropped | the class pin, the unit pin |
| the in-place arm is anchored again | the class pin, the unit pin |
| 5a/5b are anchored | the class pin |
| the deny arm stops anchoring the swept dirs | all three anchor pins |
| `bwrapAnchorStep` back on `os.Stat` | the link pin |
| the carve-out stops denying the staging root | the finding-2 pin |
| the copy goes straight into the temp root again | the finding-2 pin |
| the staging root's lstat guard removed | the finding-2 pin |

Three of those were green against a first draft. "The copy goes straight into
the temp root again" passed a finding-2 pin that asserted where the root IS and
never that the verb uses it. "The in-place arm is anchored again" and "5a/5b are
anchored" passed a class pin that had not yet drawn the edge — and the edge was
not reasoned out, it was handed over by a live pin and by two stray files in the
operator's home.

## Not closed here

- ranger-base-lin9y
