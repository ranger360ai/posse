## The private /tmp replaced a grant it dropped by the producer's raw spelling (ranger-base-aurb8)

Thirteen bwrap-render pins red on macos arm 1 at b2509fba — CI run 44974829306,
job 137062263947, "test (macos-latest, 1)" — every one of them on the same
refusal, from the prep host fence:

```
cage seatbelt (bwrap) refuses to prepare /private/tmp/posse-upgrade (a deny
that is not there yet and sits inside a write grant, so the session could
create it): the bwrap prep host fence is up (this is a test binary) and
/private/tmp/posse-upgrade is under none of
/private/var/folders/.../T/posse-testhome-<n> /var/folders/.../T/ /var/tmp
```

The fence was right and the bead that filed this said so: it refused a test
binary's write into the host's own `/private/tmp`. **The defect is one step
earlier — that path should not have been inside a write grant at all.**

### What the render was doing

`bwrapRender` binds the seat's private directory over `/tmp` (ranger-base-7wdb7)
and then takes out of the write set the grants that bind replaces:
`bwrapDropTmpGrants(r.Set, $TMPDIR)`. Its drop set was

```go
drop := map[string]bool{bwrapTmpMount: true}   // the const "/tmp", RAW
if tmpdir != "" { drop[absResolve(tmpdir)] = true }
...
for _, p := range set { if !drop[absResolve(p)] { … } }
```

Every entry of that set comes from `SeatbeltWritable`, whose own `add` is
`absResolve` — "the path the sandbox will match on". So the set is in the
kernel's spelling and the mount's key was in the producer's.

- on **linux** the two are one string, `/tmp`, and the rule worked;
- on **darwin** `/tmp` IS `/private/tmp`. `seatbeltTempGrants` names both
  spellings, `add` resolves both to `/private/tmp`, and the raw key matched
  NEITHER. The blanket temp grant therefore survived a render that had just
  bound a private /tmp over it.

Downstream of that surviving grant, bwrap's deny arm does what it is supposed
to: a deny that is absent but inside a write grant is a hole the session could
open, so the launcher is asked to `mkdir -p` it first. `posse upgrade`'s
staging root is denied under every blanket temp grant (`upgradePrivateDenied`,
ranger-base-ua625 / ranger-base-18brx), so the step was
`mkdir -p /private/tmp/posse-upgrade` — on the host, in a test binary, which is
exactly what prephostfence.go exists to refuse. Hence 13 rows, one sentence.

This is ranger-base-al4n9's class one mechanism over, two commits later: *only
a resolved grant is one a kernel matches.* There it was a test arm appending a
producer's raw output to a profile; here it is production code comparing a
resolved set against a raw literal.

### The fix

- `bwrapDropTmpGrantsOver(set, mount, tmpdir)` resolves the mount —
  `drop[absResolve(mount)]`. `bwrapDropTmpGrants` is it with `bwrapTmpMount`,
  which is all production passes.
- the mount is a parameter for `seatbeltTempGrants`' reason: a pin has to be
  able to read the darwin answer on a box that is not darwin.
- `BwrapArgs`' step 1c held a SECOND copy of the same comparison
  (`absResolve(w) != bwrapTmpMount`) with the same blind spot, for direct
  callers whose set bwrapRender did not filter. It calls the one function with
  an empty TMPDIR now, so the rule and its resolution are in one place.

### MEASURED 2026-11-21 (linux 7.2.15-1-omarchy, go1.27.3)

No darwin host on this box, so the darwin behaviour is read through a PLANTED
symlink — `<root>/tmp -> <root>/private/tmp`, darwin's own shape — which is the
route ranger-base-al4n9's pin took for the same reason.
`TestQABwrapDropTmpGrantsReadsTheMountsResolvedSpelling` (bwraptmp_qa_test.go)
passes that link as the mount, a set of resolved paths as the grants, and asks
two things: the mount's grant is gone under both of its spellings, and the
staging root `<real>/posse-upgrade` is no longer `writeGranted` by what is left
(with a control that it WAS, undropped — otherwise the second arm grades
nothing).

| reading | result |
|---|---|
| the new pin, with the fix | green |
| mutant: the raw key back (`drop{mount: true}`) | **RED**, naming both spellings kept |
| the two pre-existing drop pins, under that same mutant | green — neither can see it, which is why only a macos leg could |
| the 13 rows named on the bead, linux arm 1 | green before and after |
| `-run 'Bwrap|Wall|Tmp|Prep'`, internal/posse arm 1 | ok, 133.0s |

### Not closed here

- **The darwin verdict is the next macos arm-1 leg's.** There is no darwin host
  on this box; the mechanism is established by reading the render and by the
  planted-symlink pin, and the bead's DONE WHEN ("the 13 green or
  skipped-with-reason on a macos-latest arm 1 leg") is answered by the first
  macos leg after this lands.
- **The linux half of the bead's own fix suggestion is ranger-base-2q0uy.** A
  wall fixture whose home is under `/tmp` renders no private /tmp at all, so it
  keeps the host's blanket temp grant honestly — and the prep then creates
  `<os.TempDir()>/posse-upgrade` on the host, which the fence admits because
  `os.TempDir()` has to be a fence root for `t.TempDir()`'s sake. MEASURED
  there, with the reading and the two fixture-side shapes that would close it.
  Nothing in this change affects it: the fix only moves the shape where a
  private /tmp IS rendered.
