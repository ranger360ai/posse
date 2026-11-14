# The two scripts that could not be signalled, signalled (ranger-base-kbaah)

discovered-from: ranger-base-kkp5g, whose DONE WHEN had one clause left that
the box which wrote it could not reach.

ranger-base-kkp5g split the INT/TERM arms of five trap handlers under
`scripts/`. Three were measured on the real script with real signals;
`scripts/verify-herdr-bob.sh` and `scripts/verify-herdr-bob-rules.sh` were
not, because both SKIP here — herdr-bob is not installed at the pinned path
and `PINNED_SHA` means the rig cannot be faked up — so the bead asked the
operator for the mac.

**The SKIP is upstream of the trap, which is why the controls said nothing.**
Both scripts register their traps *after* the skip gates
(`verify-herdr-bob.sh`: gate :128, traps :153-155; `-rules.sh`: gate :125,
traps :185-187). A run that skips exits at the gate having never armed a
handler, so `exit 0, SKIP: herdr-bob is not installed at …` is not a weak
reading of the split — it is no reading of it at all.

MEASURED 2026-11-13, linux 7.2.15 x86_64, bash 5.3.20 (the Linux host). Both
controls re-run on this box: exit 0, `SKIP: herdr-bob is not installed at
/home/posse/.config/herdr/plugins/github/mloeper.herdr-bob-de923c8f068d`. That
path's *parent* does not exist either.

## What this box CAN measure, and the one claim it cannot

What the split is, is **script text**: three `trap` lines, and a `cleanup` the
EXIT arm runs. The plugin is the subject of the arms *below* them. So the
signal arm can be measured here if the run can be got past the skip gate and
held somewhere long enough for a signal to land — and that needs no plugin.

The rig is **environment only; neither tracked file was modified for any
reading below** (md5 asserted identical before and after the mutations, and
`git status -- scripts/` clean):

- `HERDR_BOB_ROOT` at a stand-in checkout — the override hook the scripts
  already carry (`verify-herdr-bob.sh:31`, `-rules.sh:25`). It holds a
  `bin/` with a stand-in `bob-watch`/`bob-watchctl` and a `rules.json`.
- PATH shims that answer the three questions a checkout this box does not
  have would answer: `git rev-parse HEAD` → the pinned sha, `git checkout
  --force` → success, `git apply` → success. Nothing else is intercepted;
  the shim `exec`s the real `git`, which on this box is the gate shim, so the
  gate still sees every call the rig does not fake.
- **A hold, which is the whole device.** Both files run a top-level `chmod +x`
  a few lines past their trap registration and before any arm can print a
  verdict (`-rules.sh:236`, `.sh:195`), so a `chmod` shim that sleeps opens a
  window in the PARENT shell the instant the trap is armed, with every arm
  still ahead of it. Same device as ranger-base-kkp5g's `--wrapper <a script
  that sleeps>` for verify-gate-freshness.sh.

**What it is not.** It is not the plugin, so it is not the run
ranger-base-5b3lq asks for, and nothing here is a verdict about herdr-bob. It
is a reading of the two files' trap lines, which is what this bead was for.

`set -m` throughout, for ranger-base-kkp5g's reason: a background child of a
non-interactive shell has INT set to `SIG_IGN`, bash cannot trap a signal
ignored on entry, and an INT arm without it measures nothing and looks like a
pass. Seen again here — a probe run in the FOREGROUND of a gate shell also had
INT ignored on entry (exit 0, handler never reached), because an ignored
disposition is inherited across `exec`. Only the `set -m` readings below are
real INT readings.

## The signal arm

`kill` of one pid — the one the runner launched — never a `pkill -f` pattern:
every seat on this box runs byte-identical argv.

| # | script | signal | at | EXIT | the line on stderr | FAIL lines | `N check(s) failed` | scratch | watcher |
|---|---|---|---|---|---|---|---|---|---|
| S1 | `-rules.sh` | TERM | 1s | **143** | `interrupted (SIGTERM) -- that is the apparatus and not the property; no verdicts` | **0** | **0** | gone | — |
| S2 | `-rules.sh` | INT | 1s | **130** | `interrupted (SIGINT) -- …` | **0** | **0** | gone | — |
| S3 | `.sh` | TERM | 1s | **143** | `interrupted (SIGTERM) -- the apparatus is gone; no verdicts` | **0** | **0** | gone | — |
| S4 | `.sh` | INT | 1s | **130** | `interrupted (SIGINT) -- …` | **0** | **0** | gone | — |
| W | `.sh` | TERM | 4s | **143** | same | **0** | **0** | gone | **killed** |
| D1 | `-rules.sh` | TERM | 1s | **143** | same | **0** | **0** | gone | — |
| D2 | `.sh` | TERM | 1s | **143** | same | **0** | **0** | gone | — |

S1-S4 print **nothing at all on stdout**: the window is before the first arm.

**W is the clause the other rows cannot reach** — "no bob-watch left running",
which is `cleanup`'s `kill` loop and not its `rm -rf`. The stand-in's
`bob-watchctl start` leaves a live detached watcher and a pidfile under the
PLUGIN state root, so `verify-herdr-bob.sh:355-358` records that pid in
`STARTED_PIDS` and prints `ok bug 1: start left a live watcher, pid 479512
(pidfile agrees)`. TERM at 4s: exit 143, and pid 479512 gone afterwards. The
`exit` in the signal arm runs the EXIT trap, cleanup kills what it registered.
There is no equivalent row for `-rules.sh`, whose watcher lives in the two
seeding arms at the very end of the file: those need the real `match_rules`
and are not reachable here at all. For that one function of that one file the
evidence is still ranger-base-myjcz's stand-in reading plus the mac.

**D1/D2 are the same signal landing inside a COMMAND SUBSTITUTION** (held in
`-rules.sh`'s first jq question, and in `verify-herdr-bob.sh`'s
`D="$(prepare fix patch)"`). bash cannot run a trap while blocked there, so
delivery is deferred until the child returns — ranger-base-myjcz's finding,
from the other side. The status and the line are unchanged; only the latency
is. **This matters to whoever runs the mac pass**: a `sleep 4; kill -TERM` can
land in a substitution and appear to do nothing for seconds.

## The readings can say no

ranger-base-kkp5g's PRE-FIX spelling planted back — `trap cleanup EXIT INT
TERM`, one line for the split three — read back before each run, restored from
a byte copy after, md5 identical before and after:

| # | mutant | EXIT | what it printed instead |
|---|---|---|---|
| M1 | `-rules.sh`, TERM @1s | **1** | no interrupted line, and **2 FAILs** — one of them `FAIL the override is not valid JSON, or has no .rules array (jq status 1)`, a red verdict about **posse's own rules.json**, which is valid. `cleanup` had removed `$T/jq.sh` under the run. |
| M2 | `.sh`, TERM @1s | **1** | no interrupted line, **10 FAILs** and the `10 check(s) failed` tail — *and six `ok`s*, over an apparatus that was already gone |
| M3 | `.sh` + recorded watcher, TERM @4s | **1** | carried on through ten more arms; the watcher WAS still killed, so resumption's cost here is the false verdicts and not a leak |

M2's six greens are the thing worth keeping. ranger-base-kkp5g's description
is that a resuming run prints FAILs it never measured; it also prints **`ok`s
it never measured**, which is the worse half of the same sentence and the one
a reader skims past.

Against those, the unsignalled controls under the same rig: `-rules.sh` exit 1
with 1 FAIL (its extraction arm refusing, correctly, because the stand-in has
no `match_rules`), `.sh` exit 1 with 13 FAILs and the tail. So the
discriminator is not subtle — 143 and silence against 1 and two dozen lines.

## bash runs the EXIT trap on an UNTRAPPED terminating signal

Found by asking what HUP does, since HUP is in `bd-argv-gate.sh`'s and
`verify-gate-freshness.sh`'s trap lists and in neither of these two.

MEASURED 2026-11-13, bash 5.3.20, with only `trap cleanup EXIT` set and the
signal delivered 1s into a 6s hold — twice over, once with the subject as a
`set -m` background job of a runner and once as the foreground child of a
shell, signalled by its own detached child, so neither backgrounding nor the
runner can be what produced it:

| signal | exit | EXIT trap | resumed |
|---|---|---|---|
| HUP | 129 | **RAN** | no |
| TERM | 143 | **RAN** | no |
| USR1 | 138 | **RAN** | no |

So for these two scripts a HUP today is: `cleanup` runs, scratch removed,
watchers killed, exit 129 — and **no explanatory line**, because no handler
printed one. That is a smaller gap than INT and TERM were, and a real one.

It also makes one clause of ranger-base-kkp5g's bd-argv-gate exemption false,
in all three places that carry it — `docs/notes.d/ranger-base-kkp5g.md:83`,
`scripts/bd-argv-gate.sh:106`, and the `tsgExempt` row's `Reason` in
`internal/treepins/trapsignalexit_qa_test.go:203`. Each says dropping the
signals from that trap "kills the gate before it can answer (fail open) **AND
leaks `$errd`**". The fail-open half is true and is the decisive one, so **the
exemption's verdict is unchanged and bd-argv-gate.sh is correct as it
stands**. The leak half is not: the EXIT trap would still run and still
`rm -rf -- "${errd:?}"`.

Not corrected in this bead, deliberately. The three sites have to move
together or the next reader cannot tell which is stale, and one of them is
`bd-argv-gate.sh` — a file `verify-gate-freshness.sh` compares the INSTALLED
gate against HEAD, so editing even its comment makes every installed gate on
this box read `stale` until they are re-rendered. That is a verb with a blast
radius, for a prose fix, and it belongs to its own bead with its own
re-install and its own freshness check rather than riding along here.

## Not closed here

- ranger-base-5b3lq
- ranger-base-5owgm

The rig (`shim/{git,jq,chmod}`, the stand-in checkout, `run-one.sh`,
`battery.sh`, `watcher-arm.sh`, `mutate.sh`, the two HUP probes and every
transcript) stayed in the session scratchpad, for ranger-base-myjcz's reason:
shipping it under `scripts/` would put a stand-in `bob-watch` where the verify
scripts can find one, which is the single thing the `PINNED_SHA` check exists
to prevent.

`go run ./cmd/checkorphans` clean after every batch.
