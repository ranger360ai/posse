# An untrapped terminating signal runs bash's EXIT trap, so one clause of the bd-argv-gate exemption was false in three places — and HUP now speaks in the five splits

ranger-base-5owgm, 2026-11-14. Discovered from ranger-base-kbaah, which
measured the fact while building a signal rig for the two verify-herdr-bob
scripts and deliberately left both halves here.

## The fact it all turns on

MEASURED 2026-11-13 (linux 7.2.15 x86_64, bash 5.3.20; the readings and the
probes are ranger-base-kbaah's, under "bash runs the EXIT trap on an UNTRAPPED
terminating signal"), with only `trap cleanup EXIT` set and the signal
delivered 1s into a 6s hold, taken twice over — once with the subject as a
`set -m` background job of a runner, once as the foreground child of a shell
signalled by its own detached child, so neither backgrounding nor the runner
is what produced it:

| signal | exit | EXIT trap | resumed |
|---|---|---|---|
| HUP | 129 | **RAN** | no |
| TERM | 143 | **RAN** | no |
| USR1 | 138 | **RAN** | no |

bash runs the EXIT trap on an untrapped terminating signal, exits 128+signo,
and does not resume. Both halves below follow from that one line.

## Half one: the clause that was false, corrected in all three places

ranger-base-kkp5g's exemption for `scripts/bd-argv-gate.sh` gives three
reasons not to drop INT/TERM/HUP from that trap. The third read, verbatim and
in three places:

> Dropping the signals from the trap is worse again: bash's default
> disposition then kills the gate before it can answer (fail open) **AND
> leaks `$errd`**.

The fail-open half is true and is the decisive one — exit 129/143 is nonzero
but not 2, and ranger-base-3bqn MEASURED that any other nonzero shows stderr
and **runs** the call. The leak half is false: the EXIT trap would still run
and still `rm -rf -- "${errd:?}"`.

So **the exemption's verdict is unchanged and `bd-argv-gate.sh`'s code is
correct as it stands.** What changed is one sentence, in the three places that
carried it and in the same words, because a partial correction is worse than
none:

- `docs/notes.d/ranger-base-kkp5g.md` — the "Why bd-argv-gate.sh is the
  exemption" bullet.
- `scripts/bd-argv-gate.sh` — the third bullet of the trap's head comment.
- `internal/treepins/trapsignalexit_qa_test.go` — the `tsgExempt` row's
  `Reason`, which is the one the pin prints.

Each now says fail-open is the whole of the reason and enough on its own, that
the row used to claim a leak, and what was measured instead.

## Half two: HUP speaks in the four splits that had no arm for it

ranger-base-kkp5g split five scripts on INT and TERM.
`verify-gate-freshness.sh` already had a HUP arm (it answers all three through
`nothing`); the other four did not. Given the fact above, a HUP was never a
leak in any of them — cleanup ran, scratch went, exit 129 — it was a
**silence**, and the explanatory line is the entire thing the split bought.

MEASURED 2026-11-14 (linux 7.2.15 x86_64, bash 5.3.20, this box), one SIGHUP
to the script's own pid as a `set -m` background job, after the change. Every
run: exit 129, the new line on stderr, and no scratch dir of any of these
families left behind.

| script | HUP at | exit | the line | partial work reported? |
|---|---|---|---|---|
| `clean-build.sh` | 1s | 129 | `clean-build: interrupted (SIGHUP) — this run did not finish` | no `built` line |
| `release-artifacts.sh` (`--version v0.5.3`, warm) | 2s | 129 | `release-artifacts: interrupted (SIGHUP) — <out> is incomplete` | `--out` empty, no checksums.txt |
| `verify-herdr-bob.sh` | 2s | 129 | `verify-herdr-bob: interrupted (SIGHUP) -- the apparatus is gone; no verdicts` | no verdicts |
| `verify-herdr-bob-rules.sh` | 2s | 129 | `verify-herdr-bob-rules: interrupted (SIGHUP) -- that is the apparatus and not the property; no verdicts` | no verdicts |

129 is ≥ 128, so `verify-herdr-bob-rules.sh`'s own `apparatus()` already read
it as NOT AN ANSWER; what it could not do was say so.

**The rig stayed in the session scratchpad**, for ranger-base-kbaah's reason:
the two verify-herdr-bob scripts both SKIP on this box (the plugin is not
installed at the pinned path), and the only way to reach their traps is a
stand-in — which is the single thing their `PINNED_SHA` check exists to
prevent. Shipping one under `scripts/` would put a stand-in `bob-watch` where
a verify script could find one. What the rig was: a `git` shim answering
`rev-parse HEAD` for the stand-in root with the pinned sha, an otherwise empty
stand-in root with a `bob-watch` carrying a `match_rules()` of the right shape
and a hold, and a probe that backgrounds the real script under `set -m`, sends
one HUP to the pid it launched, and `wait`s. `go run ./cmd/checkorphans` clean
after every batch.

**`verify-herdr-bob.sh` needed a hold and got the one ranger-base-kkp5g
used.** It reaches its trap and then dies in milliseconds on this box's empty
stand-in, so the shim sleeps 5s on the pinned-sha `checkout --force` the
patched arm runs *after* arming the trap — the same device as kkp5g's
`--wrapper <a script that sleeps 10s>` for `verify-gate-freshness.sh`. Worth
recording for whoever next lands a signal on a fast script: the handler then
ran when that child exited and not when the signal arrived (wall 5.0s for a
HUP at 2s), which is bash deferring a trap until the running foreground
command completes — the same deferral `verify-herdr-bob-rules.sh:665` already
records at 9s.

## What this did to the pin's census

`internal/treepins/trapsignalexit_qa_test.go`'s census moves by exactly the
four new registrations. MEASURED 2026-11-14 by the pin itself: 48 shell files
of the 64 under `scripts/`, **53** trap registrations, **22** on a deadly
signal, **21** of those ending the shell, one exempted. The 2026-11-13
reading, before these four arms, was 49/18/17. The floors (40 / 42 / 15 / 14)
did not move and still sit under the older reading, which is what lets an
ordinary landing past them.

One number in that file was replaced rather than updated. A looseness bullet
read "all **nine** registrations on a deadly signal under scripts/ reach their
`exit` or their re-raise unconditionally". Nine was already stale when
kkp5g's five splits landed — the same file's own census said 18 and 17 in the
same breath — and the predicate that counted nine cannot now be recovered, so
the sentence was rewritten against the census's own `ending` (21,
MEASURED 2026-11-14 by reading all of them: the twelve `echo …; exit N` arms
of the four splits, `verify-gate-freshness.sh`'s three calls to `nothing`,
`queue-cutover.sh`'s three bare exits, `reap-selftest-ambient.sh`'s
`cleanup; exit 130`, `test-times.sh`'s two re-raises). That figure is the one
the floor below it reads, so the prose and the pin can no longer drift apart
unnoticed.

Left as it stands, deliberately: the coverage arm's floor provenance says
"MEASURED 2026-11-13: 48 files, 11313 lines". It reads **11,380** today —
63 of those 67 lines are other people's landings, 4 are this bead's. It is a
dated reading of an incidental metric under a floor of 9000, it drifts with
every landing that adds a shell line, and re-dating it would imply somebody
maintains it.

## The blast radius this bead was filed to carry, and what it found

`scripts/bd-argv-gate.sh` is the one file here that
`scripts/verify-gate-freshness.sh` compares the INSTALLED gate against, so the
worry was that editing even its comment makes this box's gate read `stale`.

MEASURED 2026-11-14, before any of this landed: **it already does.** The
installed copy at `~/.config/posse/gate/bd-argv-gate.sh` differs from the main
checkout's HEAD (96052c97) by exactly one hunk, and that hunk is pure
comment — ranger-base-kkp5g's own trap block, which landed after the copy was
installed (2026-11-13 12:56). `bd-argv-gate.py` matches HEAD, and all three
behaviour arms are green: `bd list` allowed, `bd daemon stop` denied **by the
parser**, `go test` untouched. So the fence on this box is functionally
HEAD's, the detective control is doing exactly its job, and this bead adds no
new staleness — it extends the same comment block, and one `install` line
refreshes both.

That line is the operator's to type and nobody else's (ADR 0015 §3: posse
renders no box-wide hook), so it is filed as ranger-base-lkju4 with the
command, the blast radius and what verified will mean. `posse upgrade`
installs this box's gate copy from the commit it installs (ADR 0092 D5), so
the next upgrade also closes it.

## Not closed here

- ranger-base-lkju4 — the operator's one `install` line for this box's gate
  copy, which was already owed before this bead and is behaviour-green either
  way.
