# The verdict file has one subject, and not every run can speak for it (ranger-base-4dhcw)

2026-10-31. Discovered on ranger-base-zf8pf, which removed one cage artifact
that could poison this file and did not remove the class.

## The defect

`scripts/verify-box.sh` called `write_state` unconditionally, at the end of
every run, whoever typed it and from wherever. `write_state` replaces
`$RHQ_HOME/state/verify-box.yaml` **whole** — its own header says so — and
posse's governance row G10 reads that file. So the last run to finish owned
the box's recorded verdict, and a seat that typed `make verify-box` by hand
overwrote the operator's scheduled one with its own, refreshing the freshness
clock along with it.

Two things make that worse than a tie-break nobody will notice.

**The write is reachable from exactly the seats whose answer is least
trustworthy.** `App.SeatbeltWritable` adds `a.StateDir` to the writable list
for every caged seat (`internal/posse/seatbelt.go`, "posse's own state,
derived from the home"), and `verify-box.yaml` lives directly under it. Six of
the eleven live PIDs declare `cage: seatbelt` (dinesh, gwart, holden, hoover,
jian-yang, laurie). Holden measured the `touch` into that directory on
ranger-base-j1eeh; the grant in `seatbelt.go` is the same fact read off the
code.

**A caged check's red can be about the cage.** ranger-base-zf8pf is the worked
example: `brew info --cask` exits non-zero under a seatbelt profile with a
readable header, and `verify-grok-pin.sh` scored that as a finding about the
box. That red would have become the operator's recorded verdict, and G10 would
then have reported a failing box because a seat was sandboxed.

There is a third reason, independent of cages and true of every seat:
`verify-policy-pins`, `verify-gate-freshness` and `verify-hook-freshness` all
compare the live box against **HEAD of the checkout they are run from**. A
dispatched seat runs in its own worktree on its own branch, so its red can be
an unlanded branch and its green can be a branch that has not landed the thing
the box is missing.

## The fix

The bead listed three honest shapes and asked for one to be picked rather than
inherited from the description's ordering:

- **(a)** only the recording run writes a verdict; anything else says so on
  stdout.
- **(b)** `write_state` records which seat and cage produced the verdict, so
  G10 can discount a hand run.
- **(c)** a hand run writes to a separate path.

**(a) was taken.** (b) leaves one file that the last writer still replaces
whole, so the scheduled verdict is gone either way and G10 gains a field whose
only use is to disbelieve the record it sits in — and `verifybox.go` already
argues the case against that shape ("a field nothing reads is a second store
that can drift from the first in silence"). (c) preserves the scheduled
verdict, but the second file is read by nobody: the person who typed the
command is reading stdout, which is where the answer already is. Both collapse
toward (a) with extra surface.

So: a dispatched seat **runs every check and prints every verdict** — seeing
the answer is the whole point of typing it — and records nothing, saying so on
stdout beside the verdict it just printed. The gate is inside `write_state`
rather than at its one call site, so there stays exactly one place that knows
whether a verdict was recorded.

## The discriminator, and which way it fails

`RHQ_PERSONA`. posse sets it on every dispatched seat and nothing else sets it
(`const EnvPersona = "RHQ_PERSONA"`, `internal/posse/herdrback.go:975`); the
git shim that `scripts/audit-silent-reverts.sh` sheds with `env -u
RHQ_PERSONA` already reads it the same way.

MEASURED 2026-10-31, `plutil -p ~/Library/LaunchAgents/com.posse.verify-box.plist`
as installed: the scheduled job's `EnvironmentVariables` are exactly `PATH` and
`RHQ_HOME`, its `ProgramArguments` are the instance checkout's
`scripts/verify-box.sh --quiet`, and it fires at 06:15 daily.
**`RHQ_PERSONA` is not among them**, so
the scheduled run is unchanged and so is the operator's own `make verify-box`.
The plist was read and not touched; nothing here needs it changed, which is
why this landed without an operator gate.

The gate therefore fails *open toward the schedule*: if the discriminator were
somehow wrong on the recording run, the file stops being refreshed, ages past
`verify_box_max_age:` (26h) and G10 raises `verify-box-stale` within the day.
A judgement about who may write has to fail in the direction that makes the
surface louder.

An alternative discriminator was considered and dropped: detecting the
scheduled run positively (ppid, `XPC_SERVICE_NAME`, a `--record` flag in the
plist). Each needs either a measurement of launchd internals or a change to a
live installed plist, and each fails *closed* — a wrong answer silently stops
the only recording run there is.

## What is pinned, and the mutant that proves it

- `scripts/verify-box.sh --self-test` arm 8, three properties with a control:
  the file is not written, the run still exits with its real verdict (the gate
  silences the write, not the verdict), and the run says where the verdict
  went. The control is the same roster with `RHQ_PERSONA` shed, which must
  write — without it the arm would pass on a script that stopped writing at
  all.
- `self_test()` sheds `RHQ_PERSONA` for all its other arms. Without that every
  state arm would fail from a dispatched seat and pass from the operator's
  terminal.
- `internal/treepins/boxcheck_qa_test.go` names `a dispatched seat records no
  verdict` in `TestQABoxCheckSelfTestStillFires`, so the arm cannot be deleted
  under the ten-arm floor.
- `internal/posse/verifybox_qa_test.go`'s
  `TestTheRecordingGateNamesTheVariablePosseActuallySets` holds the **join**:
  the script spells the variable as a literal, so a rename of `EnvPersona` on
  the Go side would leave the gate reading an environment posse no longer sets
  — open, with every arm above still green.
- `TestTheScriptWritesWhatThisReaderParses` now passes `EnvPersona+"="`
  explicitly. It is the format join and wants the recording path; inherited,
  it would fail from a seat and pass from a terminal.

MEASURED 2026-10-31, this worktree: with the gate's condition replaced by
`if false`, arm 8 reports `FAIL a dispatched seat recorded a verdict over the
box's` and `FAIL a dispatched seat wrote nothing and said nothing`, and the
control still passes. The arm is not vacuous.

## The defect is in the live log, today

MEASURED 2026-10-31 11:17Z, reading `~/.config/posse/state/verify-box.log`
read-only. The plist points `StandardOutPath` and `StandardErrorPath` at that
file, so **every scheduled run leaves a `run started <stamp>` line there and a
hand run leaves none** — a hand run prints to the typist's terminal and
overwrites the state file in silence.

- 55 start lines in the log; 51 of them at hour `10Z`, which is 06:15
  America/New_York — the schedule. (The box is `America/New_York`; the plist's
  `StartCalendarInterval` is 06:15 local, so the scheduled stamp is `10:15Z`
  under EDT.)
- Today's scheduled run is in the log at `2026-10-31T10:15:06Z`, and the state
  file now carries that stamp with `rc: 1` (`verify-skills-drift: finding`).
- **ranger-base-4dhcw's own description records the state file reading `rc: 0`
  at `2026-10-31T07:17:20Z` a few hours earlier, and the log has no start line
  for that run.** The zf8pf session that read it says in as many words that it
  did not run `verify-box`. So a run that was not the schedule and was not that
  session wrote the box's governance verdict at 03:17 local, and it stood as
  G10's answer for the three hours until the schedule replaced it.

The limit of that reading, stated because the artifact is gone: the state file
has since been overwritten, so what survives is the bead's contemporaneous
quotation of the stamp plus the log's silence at that minute. It is evidence
that a non-scheduled run recorded a verdict, not evidence of which seat did or
of whether its `rc: 0` was right.

## Not executed

`verify-box` was not run against the real `$RHQ_HOME` and nothing here wrote
to `$RHQ_HOME/state`. Every arm above ran against a scratch `RHQ_HOME` under
`mktemp -d`. No `launchctl` was run;
`~/Library/LaunchAgents/com.posse.verify-box.plist` and the state file and log
were read and not written.
