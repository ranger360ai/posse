# The donor dispatched on argv[0], and the darwin half was the make line (ranger-base-fol4d)

2026-11-21, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree. From ranger-base-zcj0z, which put the fixture's stderr in a file
instead of /dev/null and then stopped, deliberately, so that the next reading
would name the cause rather than another seat guessing at it from a linux box.

One CI reading was enough. Nine days of reading the log line were not.

## The reading

Run **44901378525** (2026-11-21 11:55Z, push of 964e6f9a) is the first CI job
to carry the postmortem. Its ubuntu-latest arm-1 job (136837807692) printed,
for both finding-4 arms:

```
FAIL the '-p' fixture (pid 10845) was already gone when the census ran …
     postmortem: it exited 1 (126 could not exec, 127 not found, 0 stdin EOF,
     else it rejected its argv); donor /usr/bin/tee; the copy at
     /tmp/posse-reap-selftest.iBsJBj/claude is executable; the fifo is a fifo
     its stderr: coreutils: unknown program 'claude'
```

Every one of the discriminators the bead named was answered at once. Not 126,
so not a noexec mount and not a loader error. The copy executable, the fifo a
fifo, so not the apparatus. Exit 1 with `coreutils: unknown program 'claude'`
is GNU coreutils' own message from `src/coreutils.c`, printed when a binary
built `--enable-single-binary` looks up `last_component(argv[0])` in its
program table and misses. That runner's `/usr/bin/tee` IS the multi-call
binary, so `cp`ing it to `.../claude` and running it as `.../claude` asks
coreutils to be a program it has never heard of.

This box's `/usr/bin/tee` is a standalone 51512-byte PIE (GNU coreutils 9.12),
which is why the rig was 8-of-8 green here and 0-of-N on every ubuntu job
since it landed.

## The fix, and why it is one line

The exe link and argv[0] are independent facts, and `scripts/reap-test-tmp.sh`'s
production counter reads only the first: identity is
`case "$exetarget" in */claude)` over `/proc/<pid>/exe`, and the `-p` exclusion
is `claude_could_register` scanning the cmdline's TOKENS. So the FILE has to be
at `.../claude` and argv[0] does not have to be anything at all.

```diff
-    "$fakeclaude" "$@" < "$fifo" >/dev/null 2>"$fakeerr" & pfake=$!
+    ( exec -a "$teename" "$fakeclaude" "$@" ) < "$fifo" >/dev/null 2>"$fakeerr" & pfake=$!
```

`teename=${teebin##*/}`, derived from the donor rather than spelled, so a
multi-call donor gets the name it dispatches on and a standalone one ignores
argv[0] as it always did. One spelling, both donors, darwin included.

**MEASURED against a faithful mutant of that donor**, since no ubuntu 24.04
venue is reachable from a crew seat: a binary that prints
`coreutils: unknown program '<basename argv[0]>'` to stderr and exits 1 unless
that basename is `tee`, and otherwise idles on stdin.

| donor | spelling | argv | result |
|---|---|---|---|
| mutant | argv[0] = copy path (pre-fix) | `-p` | GONE, exit 1, `coreutils: unknown program 'claude'` |
| mutant | argv[0] = copy path (pre-fix) | none | GONE, exit 1, same |
| mutant | `exec -a tee` | `-p` | LIVE, exe `…/claude`, cmdline `tee -p` |
| mutant | `exec -a tee` | none | LIVE, exe `…/claude`, cmdline `tee` |
| real (this box) | argv[0] = copy path | `-p` / none | LIVE either way |
| real (this box) | `exec -a tee` | `-p` / none | LIVE either way |

And end to end, with that mutant shadowing `tee` on PATH: the tracked script
`all arms pass`; a mutant of it reverted to the pre-fix spelling reproduces
the CI failure exactly, two arms FAILED with that same stderr. The pid in
`$!` is the fixture's own, because `exec` replaces the subshell — asserted by
reading `/proc/$pfake/exe` back as `…/claude` in every LIVE row above.

## The darwin half was never the script

The same run's macos-latest arm-1 job (136837807737) died at the SAME ninth
prerequisite, for an unrelated reason, and this is the first run where it was
visible: ranger-base-zcj0z fixed the apostrophe in `verify-shell-syntax`, two
prerequisites earlier, which is what had been masking it.

```
reap-test-tmp.sh: no /proc — this reads process state and will not guess without it
make: *** [verify-reap-test-tmp] Error 2
```

That refusal is correct and stays. The script reads `/proc` and needs
`flock(1)`, and ADR 0089 D2 is explicit that it must refuse rather than fall
back to the two weaker gates. What was wrong is making a linux-only self-test
an unconditional prerequisite of `make test` on a platform with no `/proc` at
all. The Makefile recipe now skips on the PLATFORM, loudly, with exit 0:

```
verify-reap-test-tmp: SKIPPED on Darwin -- scripts/reap-test-tmp.sh reads /proc
and needs flock(1), so its subject does not exist here (ranger-base-fol4d)
```

**On the platform and not on the precondition**, which is the only decision
here worth defending: a linux box whose cage hides `/proc`, or whose PATH has
no `flock`, still gets exit 2 and still reds, because there the refusal is
news. On darwin it is a fact about darwin. MEASURED both arms with a `uname`
shim on PATH: `Darwin` skips and exits 0, this box runs the self-test and
passes. `scripts/verify-box.sh`'s roster row for the target carries the same
sentence, which is where `boxcheck_qa_test.go` makes a reader meet it.

## The apostrophe class, closed out

The bead's second half, no CI reading needed. `scripts/reap-timer-unit.sh`
(two systemd unit bodies) and `docs/adr/0096-proc-spellings.probe.sh` (`$inner`,
a block of shell handed to `sh -c` inside bwrap) were the last two tracked
files building a body as a heredoc inside a command substitution — the shape
that took every macos arm-1 job twice (ranger-base-g4z8m, then
ranger-base-zcj0z, nine days unread the second time). Both had an EVEN
apostrophe count, which is to say they parsed by arithmetic rather than by
construction, and the probe's shebang is `#!/bin/bash`, which on darwin IS
bash 3.2.

Both are `IFS= read -r -d '' … <<EOF || true` now, with unquoted delimiters
because every path, root and interval in them is expanded on purpose. `read`
is a builtin and takes no substitution, so the hazard has nowhere left to
live. Equivalence proved rather than assumed:

- `reap-timer-unit.sh`: the rendered service unit, timer unit, stdout and
  stderr are **byte-identical** to the pre-change script's, diffed against a
  copy of HEAD's version run from the same directory. `$(cat <<EOF)` strips
  trailing newlines and `read -d ''` does not, so each body is assigned
  `${body%$'\n'}` and the four `printf '%s\n'` sites are untouched.
- `0096-proc-spellings.probe.sh`: `$inner` is identical but for one trailing
  newline (670 bytes against 671), which is nothing to the `sh -c "$inner"`
  that consumes it.

No tracked file carries the shape as of this date. The CLASS stays
`verify-shell-syntax`'s, because nothing stops the next one being written and
only a parser knows — and that sweep's own conversion idiom is proved on
darwin by this very run, whose macos leg got PAST `verify-shell-syntax` for
the first time in nine days.

## Readings worth keeping

- `make verify-reap-test-tmp` on this box, after the fix: 11.1s, all arms
  pass, both finding-4 arms LIVE at the census. (The Makefile comment above
  `register-check` prices this target at 7.3-7.5s; this reading was taken at a
  one-minute load average of 7.)
- `make tree-check`: green, **2m4.5s** at a one-minute load average of 7.05
  with neither suite slot held. Above the 83-92s band ranger-base-3cpcg
  measured at a load of 7-12.5 on the same door list, which is one more
  reading for the column in `treewidedoor_qa_test.go` and not a claim about
  this change — it added no pin.
- `go run ./cmd/checkorphans`: clean.

## Not closed here

Nothing. Whether the macos arm-1 job now reaches a Go test is the next run's
reading: `verify-wall-check` and `tree-check` sit between this prerequisite
and the first `go test`, and neither has been read on that leg since
2026-11-11.
