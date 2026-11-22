# Two verify findings: a pin whose premise nothing witnessed, and the last unguarded read of the refusals leaf (ranger-base-jielm)

2026-11-22, Linux 7.2.15-1-omarchy x86_64, from this bead's worktree.
The two LIVE findings of the ranger-base-q2peg verify of four closes.
Neither was a defect in a landed fix's refusal path.

## F1 — the spelling pin measured spelling only because of one assignment

`TestQAProbeProfilesGrantInTheKernelsSpelling`
(internal/posse/seatbeltcarveout_qa_test.go) grades ranger-base-al4n9's
grant by its SPELLING from a box with no seatbelt kernel. It can do that
only because it PLANTS a second name for the fixture's store and feeds
`sbWritableFor` the store named through that link — one line,
`through.store = link`.

Its control stood after the measurement and compared
`filepath.Join(link, ".git")` with `want`: that witnesses the LINK EXISTS,
and is true whatever `through.store` holds. So nothing stood over the
assignment the whole test rests on, while the comment claimed "if the
fixture ever stops putting the store where a link can reach it, this says
so rather than passing."

MEASURED at d28d7c45, before the fix, `go test -count=1 -run
TestQAProbeProfilesGrantInTheKernelsSpelling ./internal/posse`:

| mutation | reading |
|---|---|
| `through.store = f.store` alone | **ok** — the pin passes and measures nothing about spelling |
| that, plus the al4n9 bug put back (`w = append(w, g...)`) | **ok** — green over the exact bug the pin was written for |

A narrowness and not a hole on the day, because `sbWritableFor`'s universal
unresolved-entry fatal is an independent detector and survives the first
mutation. It is independent only as long as that assignment stands, and
nothing stood over it.

### The fix

Assert the premise BEFORE measuring, in terms of the field `sbWritableFor`
was handed rather than in terms of the link:

- `through.store != f.store` — the set is built through the other name;
- `absResolve(filepath.Join(through.store, ".git")) != filepath.Join(through.store, ".git")`
  — and that name is not the one the kernel matches on.

`Fatalf`, not `Errorf`: a row that has stopped discriminating must not also
report on what it no longer measures. The weaker post-hoc control is gone —
keeping a duplicate of an assertion placed after the thing it is supposed to
gate is the confusion that produced the finding.

MEASURED 2026-11-22, after the fix: both mutations above are now **FAIL**,
each at the first premise arm.

## F2 — `posse gates` read the canonical leaf with a plain `os.ReadFile`

ranger-base-legou guarded the host's four uncaged opens of the refusals
trail, and hoover's census for it said "no other uncaged Go WRITER to either
leaf", which was true as stated — its subject was the writers. The fold's
spool READ was in scope because it copies a link target's bytes somewhere.

`cmd/posse/main.go`'s `gates` case did the same thing one leaf over:
`os.ReadFile(gatesDir + "/refusals.log")`, printing the last ten lines into
the operator's terminal. Same leaf, same planter outside every wall, same
following open — a much narrower prize (ten lines in a terminal, not
seat-authored bytes appended to a shell rc), and the seat does not read that
output. But that leaf is guarded at every other access in the tree.

The bead filed it from a reading of the source: "NOT EXECUTED by me — it
needs a live gates dir and I did not plant one." It is executed now, through
the built binary, and the finding is confirmed live.

MEASURED 2026-11-22, with the read back on `os.ReadFile`
(`cmd/posse/gatesrefusalread_qa_test.go`, two tests, 10.1s):

- a symlink planted at `gates/builder/refusals.log` → `…/id_ed25519.pub`:
  **the report printed the target's bytes.**
- the same plant DANGLING: the report printed `refusals.log: empty`, which
  is exactly what an absent trail prints — `os.ReadFile` of a dangling link
  answers an error `os.IsNotExist` recognises, so a caller that only asks
  "is it there" reports a plant as a clean empty trail and the plant
  survives to be followed by whatever creates the leaf next.

### The fix

`(*App).ReadRefusalsLog(persona)` — `readRefusalLeaf` from `a.StateDir`, so
the parents are walked for ranger-base-byqv0's reason too — and the call
site takes it.

A refusal prints loud (`refusals.log: ⚠️  NOT READ — <why>`) and never
prints "empty": this command's output and exit are read as "cleared to
dispatch" (INSTALL.md §7, rangerhq-qz51), and a tampered trail reported as a
clean one is the same class of lie as the unshimmed-deny line two blocks
above it.

It deliberately does NOT join the exit verdict, which is the gap between the
PID and the disk and nothing else: an unreadable trail is neither, and a
report that died here would say nothing about the wall it was asked to
describe. Worth knowing when reading that output: at `cage: seatbelt` the
same plant already reds the report's `record:` row, because the bwrap wall
render refuses a symlinked keep (ranger-base-xab99) — so the plant was
visible in the report while the read arm below it printed the planter's
bytes.

## F3 — the same open at a second host-side site, forging a probe verdict

NOT on the bead; found while fixing F2, by censusing the tree for the rest
of the class rather than taking the one call site the finding named. The
bead's own F2 text says "that leaf is now guarded at every other access in
the tree", and that sentence was not true.

`runtimeprobe.go` noted where the trail stood before launching a real seat
and read what it GAINED afterwards, through two unguarded helpers of its
own — `fileSize` (`os.Stat`) and `readFrom` (`os.ReadFile`) — and nothing
else in the tree called either.

That delta is the WHOLE evidence for the probe's observable 2, graded by
`strings.Contains` over three fixed markers (`ProbeShapeDirect`,
`ProbeShapeShellC`, `ProbeShapeScript`). So the prize here is not ten lines
in a terminal: a planter who links the leaf at a file carrying those three
strings makes the probe report

> the canary deny refused and landed in refusals.log through all three
> shapes (direct, sh -c, executable script)

over a wall that refused nothing — on the command INSTALL.md §7 sells as
the thing you read before your first dispatch.

MEASURED 2026-11-22, with the pair back on `os.Stat`/`os.ReadFile`
(`TestRefusalDeltaAfterCannotBeForgedByAPlantedLeaf`, `-tags posse_arm3`):
the size read 54 through the link, and all three markers answered from the
planter's file.

### The fix

`refusalLeafSize` and `refusalDeltaAfter` in refusalleaf.go; `fileSize` and
`readFrom` deleted rather than left beside their guarded replacements
reached by no caller. Both halves fail CLOSED — size 0, delta `""` — so
observable 2 reports "no refusal reached refusals.log", which is true of a
leaf the host would not read and is the opposite of what the unguarded pair
could be made to say.

### An arm trap worth repeating

`runtimeprobe_test.go` is `//go:build posse_arm3`, so the first `-run
TestRefusalDeltaAfter ./internal/posse` answered **`ok … [no tests to run]`**
— the exact reading AGENTS.md warns about, and it looks like a pass. Every
reading above is `-tags posse_arm3`.

## What generalises

Both findings are the same shape one layer apart: **a check whose
discriminating premise is carried by one line that nothing asserts.** In F1
it was a test fixture's assignment; in F2 it was a census whose subject
("writers") did not cover the access that escaped. The answer in both cases
was to state the premise where it is used, not near it.
