# The rendered unit carries a PATH, and it is the render's own (ranger-base-ib61s)

2026-11-08. `posse systemd`'s unit (ADR 0080 D5) rendered
`Environment=RHQ_HOME`, an absolute `ExecStart` and no `PATH`. On a host
where `bd` and `posse` live in `~/.local/bin` that is a crash-loop, and the
shakedown read it.

## What was measured

MEASURED 2026-11-08 on the ADR 0080 host (the offid shakedown, leg 3):

- the rendered unit started, `posse dispatch --watch` exited 1 **at once**
  with `posse: bd not found on PATH outside posse's gates dirs`;
- `Restart=on-failure` + `RestartSec=5s` looped it until systemd's start
  limit;
- the workaround applied on the host was a drop-in
  `Environment=PATH=%h/.local/bin:/usr/local/bin:/usr/bin:/bin`.

The cause is one sentence: a systemd **user** manager reads no shell profile,
so its `PATH` does not include `~/.local/bin` — and INSTALL's own `bd` step
installs `bd` exactly there. Every Linux host following INSTALL hits it. It
is not a posse resolution bug: `protocolBin`/`protocolAvailable` were
answering correctly about the `PATH` they were given (ADR 0077 D3), and that
`PATH` had no `bd` in it.

ranger-base-136b5's own notes called this out and chose the other way —
"baking the render-time `PATH` into the file would go stale silently", a
comment in the unit and a drop-in in INSTALL.md, ASSUMED sufficient, "whether
that is enough is a shakedown reading". That reading came back no.

## The fix, and why it is the whole PATH

`renderWatchUnitOn` writes `Environment=PATH=` from a `PATH` the caller
names; `RenderWatchUnit` names this process's, through
`PathOutsideGates("")`. The value is:

1. the render's own `PATH`, in its own order — gates bin dirs dropped
   (`gatesSplitPath`, applied inside the render rather than trusted to the
   caller), relative and empty elements dropped, duplicates dropped;
2. then `systemdFallbackPath` for the dirs step 1 did not already name.

**Step 2 exists because `Environment=PATH=` REPLACES the manager's default
rather than adding to it.** A unit naming only the render host's dirs would
have taken away every binary in `/usr/bin` in order to fix one in
`~/.local/bin`. The value written there is REPORTED from systemd.exec(5),
not MEASURED: this shop has no systemd (darwin), and systemd's `DEFAULT_PATH`
is a build option a distribution may set differently.

**The whole `PATH` and not just `dirname(bd)` + `dirname(git)`**, which is
the other shape the bead offered. The loop execs more than those two —
`sqlite3`, `python3`, `gh`, `ps`, `sh`, `go`, the multiplexer, a runtime CLI
— and it also TYPES a `PATH` onto every seat it hires (`SeatBasePath`), so
the unit's `PATH` is the floor under everything the loop does, seats
included. A list of binaries is an enumeration that goes stale one landing
later; "the `PATH` posse itself was working on when the deployer rendered
this" does not. It is also what makes the unit's posse behave like the
posse the deployer just ran `posse status` with by hand.

**Gates dirs are dropped, and the render owns that rule.** A unit is nobody's
seat (ADR 0077 D3): a persona's shim dir in front of the loop's own `bd` is
ranger-base-c33j6's deadlock, in a process no persona typed in. A relative
element is the hazard this change would otherwise INTRODUCE — the unit's
`WorkingDirectory=` is a crew's own repo, so `.` or `bin` on the loop's
`PATH` would let whatever that tree holds answer to `git`. Both are dropped
and named on stderr, never swallowed.

**An unresolvable `bd` is a NOTE and not a refusal.** `unitSearchPathMisses`
reads the rendered dirs for `bd` and `git` — the two bare names a dispatch
pass execs unconditionally — and the `bd` sentence quotes `NoBdLine`
verbatim, so the render's warning and the journal's error are one searchable
string. It is a note because the render is a render: this file already
declines to stat the `posse` in `ExecStart=` ("a unit written before the
binary is promoted is still the right unit"), and `posse systemd` printing a
unit for review on a box that is not the host is a legitimate call. Not rex
(an absolute path by construction) and not herdr (whose own unit is what this
one is `Requires=`); not the runtime CLIs, whose dirs ARE carried but whose
absence does not crash-loop the loop.

## What this costs

The unit's bytes now depend on the render's `PATH`, so an operator whose
`PATH` changed will have `--write` refuse to replace the unit ("says
something else") until they pass `--force`. That is the existing guard
working: the unit really does say something else. INSTALL.md's "re-run after
any `autostart_*` change" now also says to re-run after `bd`, `git` or a
runtime CLI moves.

## Pins

All in `internal/posse/systemdunit_test.go` (`-tags posse_arm3`):

- `TestSystemdUnitPathFindsBd` — the bead's done condition, asked through the
  FILE: a fake `bd`/`git` in a dir no default `PATH` names, and
  `pathDirsLookup` over what systemd will read resolves `bd` to that one. It
  also asserts every `systemdFallbackPath` dir survives.
- `TestSystemdUnitPathNamesTheChildrenItCannotResolve` — the note, over
  `unitSearchPathMisses` directly rather than a render. A render always
  appends `systemdFallbackPath`, so a box with a real `/usr/bin/bd` would
  pass the negative arm for the wrong reason, and a box without one would
  stop testing anything the day `bd` is packaged.
- `TestSystemdUnitPathDropsWhatAUnitMustNotCarry` — a gates dir, `bin`, `.`
  and an empty element in, none of them out, each of the first three named in
  the notes, and `/opt/crew/bin` still there.
- `TestSystemdUnitPathIsEscapedLikeADirective` — `%` doubled, `$` not
  (Environment= is not a command line), and the whole list quoted once.
- `TestSystemdUnitRenderIsPinned` — now calls `renderWatchUnitOn` with a
  named `PATH` and pins the directive, including the order and the dedupe.

Nothing here ran on Linux and no unit was loaded by a real systemd; this box
has neither. The render is pinned as a render. The host reading is the
shakedown's.
