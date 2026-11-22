# The caged BEADS_DIR is a translation, not a resolve (ranger-base-ayubz)

2026-11-22, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores). **No
container ran**: this box's `posse` user is not in the `docker` group, so the
one done-condition the bead asked for could not be met here. What is measured
below is the namespace fact the fix rests on, and it was measured on this box
rather than read.

## the pair that disagreed

`beadsHome` answers bd's own SPELLING and not a resolved path, deliberately
(beadloss.go's contract, pinned by
`TestQABeadsHomeAnswersBdsOwnSpellingAndNotAResolvedPath`). At the container
tier two consumers read it, and one of them resolved:

| consumer | what it did | spelling |
|---|---|---|
| the MOUNT — `CageMounts` → `cageOverlay` | `src := absResolve(p)` | resolved |
| the NAME — `planLaunch`'s env, forwarded by `CageEnvNames` | `beadsHome(dir)` | raw |

So where the store is reached through a symlink — a `.beads/redirect` naming a
path with a link component, or any tree under a linked prefix, which is every
darwin box under `/var` — the caged bd was handed a path the container does
not have. The inner bd is always `--no-db --no-daemon` (`CageBdFlags`) and a
no-db bd resolves `$BEADS_DIR` else `$cwd/.beads` and reads no redirect at all
(ADR 0055 D1), so the caged claim/comment/close either failed outright or
landed in the session worktree's own `issues.jsonl` and died with the
container. ADR 0013 §4's reachability row read it **Enforced** throughout,
because `containerReachRow` resolves: the row judged the mount and the session
used the name.

## the namespace fact, MEASURED

`scripts`-free, under bwrap, which stands in for the engine to put the
question in its smallest form: *when only the resolved path is bound, does
the symlink spelling of that directory exist inside?* Fixture is
`<d>/link -> <d>/real` with the store at `<d>/real/store`.

**Arm A — only the resolved path bound (the L4 shape).** MEASURED 2026-11-22,
bwrap 0.13.1 on this box:

```
<d>/real/store   stat -> is there
<d>/link/store   stat -> No such file or directory
<d>/real/store   touch issues.jsonl -> wrote
<d>/link/store   touch issues.jsonl -> No such file or directory
```

That ENOENT is what a caged `bd comments add` hits, and it is why the row
reading Enforced was a false pass of the same class as ranger-base-0fb and
ranger-base-k62e.

**Arm B — the host root bound read-only too, which is posse's own L3 render
(`bwrap.go`: `--ro-bind / /`).** Same fixture, same instant:

```
<d>/real/store   touch l3.probe -> wrote
<d>/link/store   touch l3.probe -> wrote
```

Both spellings write, and both land in the one host directory. That is the
bead's claim about L3, measured rather than assumed: the link is still in the
seat's namespace, so the raw spelling resolves to the bound directory. L2 is
unaffected for a different reason — a seatbelt rule matches the vnode's real
path, so a write through the link spelling is matched against the resolved
grant.

**Live L4: NOT MEASURED, and it cannot be from this seat.** MEASURED
2026-11-22: `docker info` answers `permission denied while trying to connect
to the docker API at unix:///var/run/docker.sock`, with `uid=1001(posse)
groups=1001(posse)` against `srw-rw---- 1 root docker /var/run/docker.sock`.
That is group membership and not the cage, so no crew seat on this box can run
the before/after the bead asked for; it needs the operator's own hands or a
box where the engine is reachable.

## why one absResolve at the env site is the WRONG fix

The bead's own "THE LIKELY FIX" was one `absResolve` where the value crosses.
It is wrong, and arm 2 of `cagebeadsdir_qa_test.go` is the measurement rather
than the argument.

At L4 the filesystem IS the mount set. `CageMount.Dst` is what the container
has, and the repo bind is `{Src: dir, Dst: dir}` with dir exactly as the
launch spells it — `cageCovering`'s own comment says why ("an overlay resolved
for the host would land at a path nothing mounts"). So where **dir** is the
thing behind the link — a home under darwin's `/home` →
`/System/Volumes/Data/home`, every `t.TempDir()` under `/var` →
`/private/var` — the store crosses at the RAW spelling, and resolving the name
would have broken the case that works today. Two shapes, opposite answers:

| shape | the mount that covers the store | the container's spelling |
|---|---|---|
| `.beads/redirect` out of the tree, behind a link | the carve-out, at `absResolve(home)` | RESOLVED |
| no redirect, session dir behind a link | the repo bind, at `dir` as spelled | RAW |

So the answer is neither spelling by rule: it is **the destination the
deepest bind covering the store gives it**, which is `cageCovering`'s answer
and not a second one. `CageBeadsDir` (cage.go) is that, and `planLaunch` takes
it only when `caged` — the same predicate the cage wrap is keyed on. Below L4
the env keeps `beadsHome`'s spelling, because bd takes its own
`BEADS_DIR points to unsafe location` verdict on the path the operator
configured (ADR 0055 Consequences) and no wall below L4 needs the name
resolved at all.

`""` from `cageBeadsDirIn` means no bind covers the store, and the fallback is
`beadsHome`'s answer — what shipped. Not a second refusal: that session cannot
write the store at any spelling, and the thing that says so is the
reachability row over the same mount set, waivable with `--allow-degraded`
exactly as every other gate is.

## mutants, and the one that is equivalent

Each patches the exact declaration under test; both batteries ran the three
pre-existing files over the same subject (`beadshomespelling_qa_test.go`,
`beadsdirenv_qa_test.go`'s five arms under `-tags posse_arm3`) beside the new
ones.

| mutant | site | verdict |
|---|---|---|
| `return home` (what shipped) | `CageBeadsDir` | KILLED — arms 1, 3 |
| `return absResolve(home)` (the bead's fix) | `CageBeadsDir` | KILLED — arm 2 |
| `return over.Src` | `CageBeadsDir` | **SURVIVED, equivalent** |
| `return over.Src` | `cageBeadsDirIn` | KILLED — arm 4 |
| drop the `if caged` branch | `planLaunch` | KILLED — arm 5 |
| `absResolve(home)` for every tier | `planLaunch` | KILLED — arm 5 |
| `CageBeadsDir` for every tier | `planLaunch` | KILLED — arm 5, and `TestQACrewAndPersonalessLaunchesCarryBeadsDirToo` under `-tags posse_arm3` |

The survivor is equivalent **in every mount set a render can produce**, and
that is a fact about the domain rather than a weak pin: every bind
`CageMounts` puts over a store is same-path in and out, so `Src` and `Dst` are
one string in each of them. The one bind whose halves differ is the runtime
HOME's (`{Src: a.CageHome(persona), Dst: e.Home}`), which no store is under —
so the choice is only observable one layer down, where arm 4 asks
`cageBeadsDirIn` over a set written by hand and the same mutant dies.

Worth knowing while reading `cageBeadsDirIn`: containment is judged on `Dst`,
which is `cageCovering`'s rule and not this function's to widen ("every mount
a path rule can name is same-path in and out"). A store reachable only through
some bind's SOURCE is therefore not found at all, and arm 4's third assertion
pins that the honest answer there is `""`.
