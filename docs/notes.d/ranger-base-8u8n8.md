## Two pinned spellings of one seam, and the gate between them (ranger-base-8u8n8)

`scripts/verify-bd-pin.sh` FAILed on every persona seat on this box, against a
box whose pin was intact:

```
$ bash scripts/verify-bd-pin.sh            # MEASURED 2026-10-31, gilfoyle seat
  bd version               0.50.3                             ok
  command -v bd            ~/.config/posse/state/gates/gilfoyle/bin/bd <-- FAIL (gate shim, exec target unparseable)
  homebrew beads           pinned 1.2.2                       ok
verify-bd-pin: 1 check(s) FAILED — the live box is not what etc/bd/version-pin.toml says.
exit=1
```

Nothing on the box was wrong. `verify-bd-pin.sh` is on `verify-box.sh`'s
roster, so `verify-box.sh` exited 1 from any seat, and a gate that cries wolf
is a gate people stop reading.

### What the two sides said

Row 2 does not compare `command -v bd` against the pinned path — a persona's
gates dir leads PATH ahead of `~/.local/bin` by design (ADR 0009 §1), so that
comparison failed in every session once already (ranger-base-43v1). The fix
then was to read what the shim itself execs, out of the shim file. Its reader
matched exactly one line shape:

```sh
"exec '"*"' \"\$@\"")        # renderShim's generic tail
```

The bd shim has no such line. ADR 0073 D2 gave bd its own tail
(`renderBdWriteTail`, internal/posse/gates.go): a write verb execs
`posse gates bd-write --real "$posse_bd_real"` instead of bd, so the write-lock
arm has to be what picks the exec target — the path became an **assignment**
near the top of the shim and the tail became a variable:

```sh
posse_bd_real='~/.local/bin/bd'      # line 162 of the live shim, the path spelled out
exec "$posse_bd_real" "$@"                          # line 200
```

Both spellings were pinned. `internal/posse/dataceilingargv_qa_test.go:261`
pins `exec "$posse_bd_real" "$@"` on the renderer, with a comment that says in
so many words that `cmd=="bd"` no longer ends on a bare `exec '<real>' "$@"`.
`internal/treepins/bdpin_qa_test.go` pinned the literal form on the reader —
through `bpStubGateShim`, a hand-written fixture of a shim posse had stopped
rendering. Each pin was green about its own side. **Nothing compared them**,
so the renderer moved and the reader did not, and the only thing that noticed
was a persona seat reading a FAIL it could not act on.

### The three shapes

posse renders a bd shim in three, all of them in gates.go, and the reader now
knows all three by name:

| shape | what carries the target | renderer |
|---|---|---|
| generic | `exec '<path>' "$@"` | `renderShim` tail — git, pkill, posse, security on this box today |
| bd | `posse_bd_real='<path>'` | `renderBdWriteTail`, ADR 0073 D2 — **the only shape a bd shim has** |
| deferred | nothing: a `for d in $PATH` search at run time | either renderer's `real == ""` branch — rendered on a box where bd was absent |

The deferred shape is the one most easily misread, because it *writes the very
variable* the bd shape's target is read out of — twice, and neither write is a
frozen target: the column-0 `posse_bd_real=` is empty and unquoted, the in-loop
`posse_bd_real="$d/bd"` is indented and double-quoted. Anchoring the match at
column 0 and requiring the single quote excludes both.

Deferred and unreadable are now separate rows, because they are different
facts with different remedies: deferred means the gates were rendered where bd
was absent and the fix is a re-render; unreadable means posse renders a fourth
shape and the fix is in `shim_target`. One message for both is how the live
shape went unnamed through a release.

### The pin that would have caught it

A fourth hand-written fixture would have had the same defect as the first
three, so the pin does not write a shim: it renders one through posse's own
`RenderGates` and hands it to the live script.

- `TestQABdPinReadsTheShimPosseActuallyRenders` — render with the fixture's
  pinned bd on PATH, assert row 2 reads GATED and names that path.
- `TestQABdPinNamesTheDeferredShimPosseRendersWithNoBdOnPath` — render with no
  bd on PATH at all, assert the deferred row by name.
- `TestQABdPinReadsEveryShimShapePosseRenders` — the control, in
  `brewguard_qa_test.go`'s sense: a fixture per shape that can be read and per
  shape that cannot, each asserted against the `command -v bd` row alone and
  not against "the output contains FAIL", which is also true when some other
  row failed.

No spelling is written down on both sides of the seam any more, so the next
render change reds in the suite instead of on a seat.

VERIFIED 2026-10-31: against the pre-fix `shim_target` (`git show HEAD:`), the
bd arm, the deferred arm, the unknown-shape arm and both seam arms all FAIL —
the seam arm printing the exact row the seat saw — and the generic-literal arm
stays green. With the fix: `verify-bd-pin.sh` reads
`GATED (execs ~/.local/bin/bd)` — the pin file's own `pinned_binary`,
spelled out — and exits 0 from the gilfoyle
seat, and `scripts/verify-box.sh` reads 9 ok of 9.
