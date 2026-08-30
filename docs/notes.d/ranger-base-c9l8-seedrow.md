# ranger-base-g4cm — INSTALL.md §14's seeding row reads a silent re-run as a promoted home

Found verifying the close of **ranger-base-1vpf** (ranger-base-c9l8, laurie,
2026-08-30, at posse `e3c5cc3`). The detail lives here because the bd payload
gate refused the sentences twice; the bead points at this file.

**Not an escape.** 1vpf's DONE WHEN is met and I verified it by execution: the
row names the line to read back, the promoted-home caveat carries both
`posse promote` and `promoted.json`, and both pins in
`internal/rhq/installseedrow_test.go` are green at HEAD with neither skipped
(`TestInstallSeedingRowLeavesAHomeADispatchWillLaunchOn` 0.04s,
`TestInstallSeedingRowNamesPromoteForAPromotedHome` 0.04s). This is a defect in
the *replacement sentence* that close introduced — the same shape 1vpf itself
was for the close of ranger-base-n0d.

## Defect 1 — the inference is unsound

The row now ends:

> If the re-run says nothing about `promoted.json`, this home was **promoted**,
> not seeded — the filled-in files are outside `promoted.json` and every
> *dispatched* launch now refuses (ADR 0015 §3); `posse promote` again is what
> clears it (ranger-base-pith).

The inference is *silence ⇒ promoted*. `initFrom` prints its manifest line from
a four-way switch (`internal/rhq/init.go:266-283`) whose repaired arm is guarded
by a count of what this run wrote:

```go
repaired := !fresh && wrote > 0 && man != nil && man.Seeded
```

A **seeded** home whose re-run filled no gap has `wrote == 0`. It matches no arm
of the switch, so init says nothing about `promoted.json` there either — and the
row diagnoses it as promoted. All three of the sentence's claims are false for
that home: it is seeded, nothing is outside `promoted.json`, and no dispatched
launch refuses. What the reader is sent to do is `posse promote`, a ratification
and the operator's own act under ADR 0015 §3, performed on a home that needed
nothing — over a tree that (defect 2) still holds the wrong seed's content.

Reachable from the row's own stated cause, not a contrived shape. The cause cell
says a real seed tree sits one level above the binary; the ordinary instance of
that is another posse checkout, whose `examples/` carries the same **filenames**
as the embed (`TestEmbeddedSeedMatchesExamplesDir` pins the embed as
`examples/` byte for byte). Same filenames means `copyIfMissing` writes nothing
on the re-run, which is exactly `wrote == 0`.

## Defect 2 — in that case the procedure is a no-op that reads as a fix

The stale content stays: `copyIfMissing` never overwrites, which the row says
honestly. But the row gives the reader no way to notice that nothing was
repaired. They moved the directory aside, re-ran init, got exit 0, and still
have the wrong `config.yaml` and the wrong PIDs.

## Defect 3 — "the re-run's second line" is a position, not a line

`retireExamplePIDs` writes two lines of its own to the same writer **before** the
`initialized` line (`internal/rhq/init.go`, `retired %d example PID(s) …`). On
any home that still has retirable example PIDs in `agents/` — an install from an
older posse, which is exactly who re-runs init — line two is a retirement line
and the manifest line is further down. Name the line, not its position.

## Repro — measured 2026-08-30, posse `e3c5cc3`, in-package, throwaway `RHQ_HOME`

1. Build a foreign seed dir by walking `posse.Seed` and writing **every** file it
   carries with junk content: same filenames, wrong bytes.
2. Init a fresh test home from that directory as the seed.
3. **The row's fix:** init the same home again, from the embed.
4. Read the home back with `VerifyPromoted` and `ReadPromoteManifest`.

EXPECTED, which is the row's own promise: a reader who reads the re-run's output
back can tell which home they have.

ACTUAL — step 3 printed exactly two lines, neither about the manifest:

```
initialized <home> (seed: embedded)
no personas installed — <home>/examples/agents holds 9 example PID(s) to copy from; `posse agent new <name>` scaffolds one
```

and step 4 answered `OK=true`, `constitution matches its manifest`, over a
manifest marked `seeded` with 14 files. A seeded, verifiable home that the row
calls promoted and broken.

## What would fix it (jian-yang's; QA verifies, it does not edit production text)

Either give the `wrote == 0` case a line of its own in `initFrom`, so silence
stops being ambiguous — or make the row diagnose on something the reader can
actually see (the manifest's own `seeded` flag, or the presence of
`promoted.json`) rather than on the absence of a line.

Pinned, green on both sides of that fix:
`internal/rhq/installseedsilence_qa_test.go`.

Related: ranger-base-1vpf (the row's own bead), ranger-base-pith (the promoted
half, open, dinesh), ranger-base-e6y (which added the repaired arm).
