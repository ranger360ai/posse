## `brew info --cask` has a third exit arm, and a caged seat can land in it (ranger-base-zf8pf)

`scripts/verify-grok-pin.sh` and `scripts/verify-codex-pin.sh` read the tap
version out of `brew info --cask` and required exit 0 to use it:

```sh
info=$(brew info --cask "$want_cask" 2>/dev/null); brc=$?
upstream=""
[ "$brc" -eq 0 ] && upstream=$(lastver "$info")
```

The comment above it said `brew info --cask` "exits 0 on success and 1 on an
unknown cask, so this one requires 0". There is a third case, and it is the
one six of the eleven live PIDs land in: **brew prints the header out of its
API cache and THEN exits non-zero because it could not refresh that cache.**
The version was read and the row threw it away.

Found by ranger-base-j1eeh against the close of ranger-base-7qey3, which is
the bead that introduced the grok row. The codex twin was byte-identical and
pre-existing.

### MEASURED 2026-10-31 — gilfoyle seat, Homebrew 7.0.11-75-gfcdfa04, macOS 26.7.1 arm64

A seatbelt profile of the shipped shape, rendered by hand —
`(version 1)(allow default)(deny file-write*)` over an allowlist naming the
scratchpad and TMPDIR and **no `/opt/homebrew`**, which is
`internal/posse/seatbelt.go`'s shape — and `sandbox-exec -f` around the real
brew:

```
brew info --cask grok-build   ->  exit 1
  stdout, first line:  ==> grok-build (Grok Build): 1.0.52 → 1.0.70
  stderr, only line:   Error: Operation not permitted @ rb_sysopen -
                       /opt/homebrew/var/homebrew/locks/grok-build.json.download.lock
```

The lock is the API-cache re-download, which is the mechanism
`internal/treepins/brewguard_qa_test.go`'s head comment already names (and
**not** auto-update — that correction is ranger-base-et7bs's). Confirmed by
elimination: `HOMEBREW_API_AUTO_UPDATE_SECS=0` does not produce it, and
`HOMEBREW_NO_AUTO_UPDATE=1` alone does not suppress it.

**The lock file already exists** (`/opt/homebrew/var/homebrew/locks/` is full
of 0-byte `*.lock` files, owned by this uid, dir 0755). Its existence is not
the variable: brew opens it for **write** to flock it, and that open is what
the cage denies.

**The switch, on this box at this minute, was analytics.** Same cage, same
minute, four probes:

| env | exit | header on stdout |
|---|---|---|
| none | 1 | yes |
| `HOMEBREW_NO_AUTO_UPDATE=1` | 1 | yes |
| `HOMEBREW_NO_ANALYTICS=1` | 0 | yes |
| all three guards the scripts set | 0 | yes |

So the scripts' own `HOMEBREW_NO_ANALYTICS=1` (ranger-base-esz16) happened to
be suppressing the re-download here. **This refines holden's reading rather
than contradicting it**: a caged seat *can* land in the third arm, not *always*
does, and which way it falls is brew-internal state nobody on this box
controls. The fix does not depend on which — a version that was read must be
used either way.

### The script-level before/after, same seat, same minute

Real brew throughout, reached through a one-line shim that lifts the scripts'
own analytics suppression so the re-download is attempted:
`exec env -u HOMEBREW_NO_ANALYTICS /opt/homebrew/bin/brew "$@"`.

```
BEFORE  tap version   ?         <-- FAIL (brew info --cask grok-build exited 1)
        exit 1, every other row green          [identical for the codex twin]

AFTER   tap version   1.0.70    read (brew exited 1; header read from its API cache)
        exit 0, UPSTREAM MOVED printing, the four-point re-audit list printing
```

Uncaged, both scripts print `tap version 1.0.70 read` and exit 0, before and
after. `scripts/verify-bd-pin.sh` is **not** affected and is the control: it
pipes `brew info --json=v2` through python3 and never keys on brew's exit code
at all (exit 0 caged, before and after).

### Why it mattered more than one red row

`verify-grok-pin` and `verify-codex-pin` ARE the loud lift-gate the 2026-10-31
operator ruling asked for, and `cage: seatbelt` is declared by dinesh, gwart,
holden, hoover, jian-yang and laurie — six of eleven. gilfoyle, erlich, jared,
monica and richard declare no cage, which is exactly why ranger-base-7qey3
closed on a measured exit 0 and eleven green rows while a caged seat measured
exit 1 on the same tree. A gate that cries wolf on the majority of the seats
that would type it teaches them to skim it.

### The fixture made the real case unreachable

`internal/treepins/grokpin_qa_test.go`'s `gpStubBrew` rendered a non-zero exit
as `exit 1` with **no header**:

```go
info := "cat <<'HDR'\n" + header + "\nHDR\nexit " + itoa(b.infoExit)
if b.infoExit != 0 {
    info = "exit " + itoa(b.infoExit)
}
```

So "brew failed" and "there was nothing to read" were one arm, and the case
that happens on a real box — non-zero exit WITH a readable header — had no arm
at all. `TestQAGrokPinUnreadableTapVersionFailsInsteadOfSilence` asserted the
row went red for `gpBrew{infoExit: 1}` and was satisfied by a stub that printed
nothing.

The codex fixture's knobs were already independent — `brewRaw` always printed
the header — and its `TestQACodexPinEachRowFailsAlone` arm named "brew could
not answer" handed the script a readable header with exit 2 and **demanded the
row go red**. That arm was pinning the defect in place.

### The fix

Read the header whatever the exit code is; fail only when there is no version
in it. The unknown-cask arm still fails, because an unknown cask prints no
header. The exit code keeps the one job it can do — it says WHY there was
nothing to read, and it annotates a row read off a cache brew could not
refresh:

```
tap version   1.0.70   read (brew exited 1; header read from its API cache)
```

Green, and not silent about it. The word `FAIL` stays off that line on purpose:
it is how every reader of these tables, `gpRowFailed`/`cpRowFailed` included,
tells a red row from a green one.

Fixtures, in both twins: the exit code and the header are independent knobs
(`gpBrew.noInfo` is now the only way to ask for silence, and it is the
unknown-cask shape), the mislabelled arms say what they measure, and each twin
gains `TestQA{Grok,Codex}PinNonZeroBrewWithAReadableHeaderStillReadsTheTap`.
Both new tests were run against the **unfixed** scripts and red there, naming
the discarded header; `gpRow` is new beside `gpRowFailed` (the codex twin has
carried `cpRow` since ranger-base-k4lza) because "the row is not red" and "the
row holds the version brew named" are different assertions, and a gate that
discards a reading satisfies the first.

### Left standing, and filed separately

`scripts/verify-box.sh:446` calls `write_state` unconditionally, so a caged
seat typing `make verify-box` by hand records its own verdict over the
operator's scheduled one in `$RHQ_HOME/state/verify-box.yaml`, which
governance row G10 reads. The write is reachable: `SeatbeltWritable` grants
`a.StateDir` to every caged seat (`internal/posse/seatbelt.go:432`), which
corroborates holden's `touch` exit 0 in code. This bead removes one cage
artifact that could poison that file; it does not remove the class. The
launchd schedule (`com.posse.verify-box.plist`) runs uncaged and is
unaffected. NOT EXECUTED — `verify-box` was not run, and the operator's
verdict (`rc: 0`, 2026-10-31T07:17:20Z) was not touched.
