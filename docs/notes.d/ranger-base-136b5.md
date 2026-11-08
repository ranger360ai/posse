# The watch loop as a systemd user unit, and ownership as a key (ranger-base-136b5)

ADR 0080 D5 and its amendment D9, built 2026-11-06.

## What landed

- `posse systemd [--write] [--force]` renders this home's
  `posse dispatch --watch` loop as a systemd **user** unit, one per
  `RHQ_HOME`, ordered `After=`/`Requires=` the unit named by
  `autostart_multiplexer_unit:`, with `Restart=on-failure`. The unit goes to
  stdout and everything about it to stderr, so a redirect is a file. posse
  prints the `systemctl --user` lines and never runs them.
- `autostart_via: herdr | systemd` (default `herdr`) is loop ownership,
  stated. `plugin/autostart.sh` stands down by name on `systemd`; the render
  refuses unless the key says `systemd`; `posse status`'s G7 reads the key
  and names the owner's lever.
- `internal/posse/systemdunit.go` (the readers and the render),
  `internal/posse/systemdunit_test.go` (arm 3, beside `autostart_test.go`).

## Why ownership is a key and not a detection

D9's reasoning, restated because the code reads as over-careful without it.
A `posse dispatch --watch` that cannot take `state/dispatch-watch.lock`
exits **non-zero** (`watch.go`), and herdr's server comes up *before* the
unit that is `After=` it. So on a host running both, a linked `[[startup]]`
hook that finds an armed home takes the lock first and the unit's own loop
fails on every start. That is one loop, not two — but it is the loop living
where D5 said it must not, and the only visible symptom is a unit in
`failed`.

Detection was rejected in the ADR ("the hook detects a rendered systemd
unit") because it reads another system's files to guess. The key states it.

## The grammar, and why a malformed value is refused rather than defaulted

`autostart_via:` has a default, so absent / bare / `null` / `~` all read as
`herdr` — `""` is the one reading `yamlClean` and the hook's `cfg()` cannot
disagree about, which is the whole lesson of ranger-base-k3yd and
ranger-base-fqfw. That is the `autostart_max_interval:` precedent, not the
`autostart_interval:` one: presence is not the arm switch here.

A value that is **neither name** departs from that precedent and is refused
by all three readers. A typo in a backoff cap costs one backoff; a typo in
*who arms* silently read as `herdr` arms the hook on a systemd host and
takes the lock the unit needs. Safe failure direction: refuse loudly, arm
nothing.

## `autostart_multiplexer_unit:` has no default, on purpose

D5 says "name from config, not hardcoded". posse has measured neither
multiplexer's Linux unit name, and a guess fails in two directions: a name
no unit wears makes `Requires=` refuse the start, and a name some *other*
unit wears orders the loop behind the wrong process. The render refuses and
names `systemctl --user list-units --type=service`.

## One argv, two builders

The hook builds the `dispatch --watch` argv in bash and the render builds it
in Go. This shop's rule is one renderer or a check grades a row nobody
launches — and it cannot be kept here: herdr reads the hook fresh from the
checkout at server start, so it cannot call into Go, and the render cannot
shell out to bash and stay deterministic.

What replaces the rule is `TestSystemdUnitExecStartIsTheHooksArgv`: one
table of 17 configs — every key, its absence, its default, and every value
one of them names and drops — run through the **real script in a real
process** and through the render, compared argv to argv. A `case` arm or a
fallback that moves in either language names the config it moved on.
`TestSystemdUnitNamesTheValuesItDrops` holds the same pair in words.

## Two unit-file facts the render has to know

systemd re-reads what it is handed, so every interpolated value goes through
one guard:

- `%` is the specifier escape, so a literal one is doubled. A home under
  `/srv/100%/crew` would otherwise render a path systemd expands.
- an unquoted argument splits on whitespace, so a value carrying any is
  quoted.
- a control character — a newline above all — is **refused**, not escaped: in
  a unit file a newline ends the directive and opens the next one. Flat-YAML
  cannot produce one (the reader splits on it), which is why refusing is
  honest rather than lossy.

## What is NOT measured

Nothing here ran on Linux and no unit was loaded by a real systemd: this box
has neither. The render is pinned as a render (`TestSystemdUnitRenderIsPinned`
compares the directive lines, not the prose). The unit runs for real in the
shakedown (ranger-base-offid), which is where D5 said it would.

Two render decisions the shakedown should read, both ASSUMED 2026-11-06:

- `RestartSec=5s` beside `Restart=on-failure`. systemd's start limit
  (5 starts / 10s by default) is what turns the lock-loser's non-zero exit
  into a `failed` unit rather than a restart storm; 5s keeps the retries off
  the box while that plays out. Nobody measured the number.
- **No `Environment=PATH=`.** A user service gets systemd's minimal `PATH`
  and posse shells out to `bd` and `git`. Baking the render-time `PATH` into
  the file would go stale silently, so the unit carries a comment and
  INSTALL.md says to check `systemctl --user show-environment` and add a
  drop-in. Whether that is enough is a shakedown reading.
  **SUPERSEDED 2026-11-08 (ranger-base-ib61s).** It was not enough: the
  shakedown read a 5s crash-loop on `bd not found on PATH outside posse's
  gates dirs`, because `~/.local/bin` is where INSTALL installs `bd` and a
  user manager reads no profile. The render writes the `PATH` it ran with
  now — see `docs/notes.d/ranger-base-ib61s.md`.
