# The reading posse already had, and was spending as a concession (ranger-base-6pew3)

2026-11-11. The liveness half of ADR 0076 §3, measured on a Rex block and
then spent in `RexMux.Prompt` instead of conceded to. ADR 0076 Amendment 9
is the decision; this is the transcript behind it.

Everything below was run from a crew seat on this box, against the
operator's own rex server, in a scratch session created and killed for the
purpose (`rex new liveprobe6pew3 --cwd /tmp -- /usr/bin/env PROBE=6pew3
/usr/bin/bash -i`). No posse seat was touched.

- rex CLI `/usr/bin/rex`, version string `0.0.1794069975-g6c6689`
- claude `2.1.322` (`~/.local/share/mise/installs/claude/2.1.322/claude`)
- the scratch block: `child` = `foreground` = `bash` 1686369, cwd /tmp

## 1. The record outlives its writer, and that was already known

ADR 0076's head comment measured this on 2026-10-31: Rex drops only
`working` and `blocked` when a process exits, so an `idle` record stays.
Reproduced here, twice, because the two runs differ in a way that matters.

**A writer that exits IMMEDIATELY leaves a record with no owner at all.**

```
$ rex send -b <block> 'bash -c '"'"'printf "\033]7501;state=idle:app=probecli\033\\" > /dev/tty; echo REPORTED-PID=$$'"'"''
$ rex send-key -b <block> enter
REPORTED-PID=1796108
$ rex block call -b <block> program_status
{"records":[{"state":"idle","app":"probecli","effective_app":"probecli",
             "updated_at":"2026-11-11T01:34:18.887843552-05:00"}]}
```

No `owner` key. That is the ADR's own spike case ("Rex leaves `owner` out
when it couldn't tell the foreground process") arriving by a second route:
the writer was gone before rex resolved it. It reads `rexOwnerUnknown` and
nothing may refuse on it.

**A writer that reports and then LIVES gets an owner, and keeps it after it
dies.** Same block, `sleep 12` after the report:

```
$ rex block call -b <block> program_status          # while alive
{"records":[{"state":"idle","app":"probecli2", …,
             "owner":{"pid":1924680,"name":"sleep"}}]}
$ rex block call -b <block> process                # while alive
child      bash 1686369
foreground sleep 1924680
ancestors  [bash 1686369]

… 12 s later, the writer having exited …

$ rex block call -b <block> program_status
{"records":[{"state":"idle","app":"probecli2", …,
             "owner":{"pid":1924680,"name":"sleep"}}]}      # UNCHANGED
$ rex block call -b <block> process
child      bash 1686369
foreground bash 1686369          # back to the block's own shell
ancestors  (absent)
$ kill -0 1924680
No such process
```

`rexOwnerIsShell` false, `rexOwnerLive` false → `RuntimeOwner` answers
"the record outlived its writer". `refuseWithoutComposer` read that error
and returned nil.

## 2. What a real claude leaves, by how it ended

This is the part the bead said to measure first, and it is the part that
decides how narrow the fix is. A claude was launched in the same scratch
block (`claude --model claude-haiku-5-5`), left at its composer with
`state=idle` reported, and then ended three ways.

| how it ended | `program_status` after | `process` after | verdict |
| --- | --- | --- | --- |
| `/exit` | `{"records":[]}` | foreground back to `bash` 1686369 | unknown |
| `SIGTERM` | `{"records":[]}` | same | unknown |
| `SIGKILL` | `state=idle`, `owner {"pid":2292821,"name":"claude"}` | same | **outlived its writer** |

Two notes on the live readings, neither of them the point but both worth
keeping:

- Claude on Rex reports NATIVELY, not through posse's reporter: this very
  seat's record is `{"state":"working","app":"claude-code", …,
  "owner":{"pid":1410375,"name":"claude"}}` with `msg` carrying whatever
  the turn is doing. posse's `CodexHookReport` route is for the runtimes
  that need it.
- Neither `ctrl-c` twice nor `ctrl-d` ended a claude 2.1.322 sitting at an
  empty composer in a rex block (checked, pid still alive after each);
  `/exit` did.

So the clean exits — `/exit` and SIGTERM, which is what posse's own kill
sends — take the record with them, and the dangerous row is the one that
does not get to run its shutdown: SIGKILL, an OOM kill, a crash, or any
runtime that has no clear-on-exit at all. posse's own reporter is in that
last group for a killed seat: it clears at `SessionEnd`
(`CodexHookReport`), an event nothing fires for a process that was killed.

## 3. The consequence, through the verb's own argv

The SIGKILL row's block, typed at with exactly what `RexMux.Prompt` types
— `rex send <text>` then `rex send-key enter`:

```
/tmp ❯ make the LIVENESS refusal in RexMux.Prompt, ranger-base-6pew3
make: *** No rule to make target 'the'.  Stop.
/tmp ✗
```

The work prompt was run by the block's shell. `make` was chosen for the
demonstration because a prompt that opens with it is both realistic and
harmless in a directory with no Makefile; the hazard is that a work
prompt's first word is often a program on PATH, and the rest of the
sentence is its argv.

## 4. What was built

`internal/posse/muxrex.go`:

- `runtimeOwner` — `RuntimeOwner` with the verdict kept beside the error
  (`rexOwnerAnswers` / `rexOwnerDead` / `rexOwnerShell` /
  `rexOwnerUnknown`). `RuntimeOwner` is now a wrapper and its sentences
  are untouched, because the matching is on the value and never on the
  prose.
- `refuseAtDeadOwner` — the refusal, on `rexOwnerDead` and
  `rexOwnerShell` only, `agent_not_ready` so ADR 0084 D1's
  `isNotDelivered` hands the claim back, naming the dead owner's pid and
  `posse peek` as the recovery.
- `Prompt` takes the owner reading ONCE and hands it to both refusals;
  the liveness one runs first, because a block that has gone back to its
  shell has no composer to be judged for and the stale box the dead
  runtime left would otherwise license the prompt.

Pins: `internal/posse/promptdeadowner_qa_test.go` (the verdict table, the
two refusals, the four concessions, the order, and the one-reading cost),
plus a live arm on
`TestLiveRexPromptSeesThisSeatsOwnComposer` that reds if rex ever reads
`rexOwnerDead` over the block the suite is running in — the one failure
the hermetic arms cannot see, because they seed both halves of the
comparison.

## 5. What is NOT fixed, and why it is not a smaller bug than it looks

A bare shell block with no record at all is still typed into. That is
D2 and it is deliberate: the `/exit` and SIGTERM rows above are the same
silence as a seat in its launch window, and a refusal on silence would
refuse the whole fleet at launch. The gate for that shape is
`AwaitFirstReport` at launch on this occupant and the readiness gate on
herdr's. What this bead closes is the case where posse HAD the evidence
and spent it as a concession.
