# The transcript ceiling: three levers that lost, one that held (ranger-base-ac22n)

*2026-11-08 · code · implements ADR 0050's third channel (the operator's fact
(b): "restricted-class content never enters the bead db, transcripts or
memory") as a per-repo switch on the launch line. Every number here is
MEASURED on claude 2.1.319, darwin 25.6.0, 2026-11-08, from a dispatched seat;
every arm that spends is marked.*

A fleet seat dispatched into a repo whose content may not exist in a local
file wrote the whole conversation — every file it read, every tool result — to
`$CLAUDE_CONFIG_DIR/projects/<key>/<session>.jsonl`, where it sat until the
30-day cleanup swept it. Two of ADR 0050's three channels had a mechanism (the
bead db has the data-ceiling lint; auto-memory is off fleet-wide). The
transcript had none.

## The lever that held

`CLAUDE_CODE_SKIP_PROMPT_HISTORY=1`, one process's environment, nothing
deleted and no other seat touched. It is the environment arm of the runtime's
own persistence resolver, read out of the shipped bundle:

```js
Lpr(){ if(production!=="test" ... ) ...
       if(AD())                                return "explicit_disable";   // --no-session-persistence
       if(i.CLAUDE_CODE_SKIP_PROMPT_HISTORY)   return "skip_prompt_history";
       if(R_t())                               return "nested_marker";      // CLAUDE_CODE_CHILD_SESSION
       return null }
```

`isTranscriptPersistenceDisabled()` is `Lpr()!==null`, and the transcript
writer's own methods are gated on it — the appender, the torn-tail seal, the
session-alias record, `canRecord`. **There is no print or TTY condition
anywhere in that resolver**, which is the fact that makes the flag's
`--print` restriction the flag's and not the mechanism's.

MEASURED, two `claude -p` runs differing in this one name, same cwd shape,
`ANTHROPIC_BASE_URL` at a dead loopback port so neither spends:

| arm | result |
|---|---|
| without the variable | `<root>/projects/<key>/<uuid>.jsonl` appeared |
| with `=1` | the project directory was created holding an empty `memory/`, and **no transcript file** |

Nothing else moved: no existing transcript was touched. The project
*directory* is still created — an empty directory, which the ruling's pin
("no new file under the transcript dir") is satisfied by and which is worth
knowing before someone greps for the directory as evidence.

**The value is load-bearing and `0` is the trap.** The runtime reads this one
name under two rules: the transcript resolver tests plain JS truthiness, under
which the string `"0"` is TRUE, and the prompt-history writer parses it as a
boolean (`Pe(process.env…)`), under which `"0"` is FALSE. So `0` suppresses
the transcript and keeps writing prompt history — a half-closed channel that
reads, from a config line, exactly like a closed one. Nothing takes the value
from config for that reason: the config line says WHETHER, posse says WITH
WHAT.

## The three that lost, and why each one looks like the answer

**1. `cleanupPeriodDays: 0`.** The lever the opening bead was written around,
and the reason it said MEASURE FIRST: it was documented as disabling
persistence AND deleting every existing transcript at startup, user-wide, so
one seat's `--settings` would have wiped the coordinator's pile and every
sibling's with it.

It is not that any more, and it is not a lever at all. The shipped validator
types the key `.int().positive()`, so `0` does not pass validation — and the
SIBLING key says so in the vendor's own words: *"Unlike cleanupPeriodDays, 0
is allowed because this setting never disables writes."* Its own description
now points elsewhere for this job: *"use --no-session-persistence to disable
transcript writes entirely."*

READ FROM THE VALIDATOR, NOT RUN, and that is a decision rather than a
shortcut: lever 3 below says this box has no scratch transcript root, so the
only venue for the destructive arm was the operator's live pile. What would be
needed is a box whose policy file does not pin the config dir, or a scratch
account — the operator's to run, never a seat's.

A payload carrying it is also worse than inert. A single row the validator
rejects makes the runtime discard the WHOLE `--settings` object in silence
(fieldpin.go, MEASURED 2026-09-05), which would take the credential-dir pin,
all 21 inlet rows and the fleet's permission mode off the line while it was
busy failing to stop a transcript.

**2. `--no-session-persistence`.** The flag the vendor's own doc string now
names, and it does disable the writes. It is REFUSED on an interactive launch.
MEASURED through a pty:

```
$ script -q /dev/null claude --no-session-persistence < /dev/null
Error: --no-session-persistence can only be used with --print mode.
```

Every seat posse hires is interactive — `DefaultAgentCommand` carries no `-p`,
and it cannot: ADR 0013 §2's typed prompt needs a promptable screen. So the
flag is a launch that does not start.

**3. A per-launch `CLAUDE_CONFIG_DIR` scoped to the session.** The shape the
bead offered first, the one to be loudest about, because it half works and the
half that works is the half you can see.

MEASURED with the variable exported to a scratch directory, one `claude -p`
run: the `.claude.json` landed in the scratch dir, and the **transcript landed
in the live pile anyway**. This box's root-owned `managed-settings.json` pins
the config dir in its `env` block, at policy scope — the LAST source in the
runtime's settings fold — so an exported value loses to it, the same
precedence ranger-base-rq83c measured for every other name in that block.

The two resolvers disagreeing under the pin is the worst available outcome: a
seat reading a config file posse never seeded (no trust grant, no interstitial
answers) whose transcript is exactly where it was. It would also rename the
keychain item out from under the operator's login, which is what
`credentialDirPin` exists to prevent and what makes a config-dir move a
credential change rather than a path change.

This is also why there is no scratch transcript root on this box, which is
what made lever 1's arm undecidable and what makes the live pin ask the binary
where its projects directory is rather than deriving it from a sandboxed
`HOME`.

## What was built

`session_transcripts:` — a one-level map repo → `on|off` in the operator's
config, same shape as `beads_visibility:` and compared the same way (by path
IDENTITY, `os.SameFile`, never string normalizing: on the APFS default two
spellings of one directory differ only in case, the class ranger-base-99gww
paid for). `SessionTranscripts` is the reader; `TranscriptPin` renders the
fragment; `WrapWithGates` puts it beside `RuntimeHomePin`, so the same three
sites carry it — `planLaunch`, `RelaunchAgent` and the runtime probe.

**UNMARKED KEEPS THE TRANSCRIPT**, which is the opposite direction from the
other two walls in this family, and the ledger below is why.

**TWO ARMS, and the unset arm is the point.** A seat is a child of the
multiplexer SERVER, whose environment was frozen whenever something happened
to autostart it; posse cannot read that layer, so no launch decision may rest
on it (ADR 0077 D1's argument). Here the leak direction is the dangerous one:
a `CLAUDE_CODE_SKIP_PROMPT_HISTORY` that reached that server's environment by
any route would silence EVERY seat's transcript on the box, and the loss
arrives as **no error at all**. So an `on` repo does not merely decline to set
the name — it renders `/usr/bin/env -u` and unsets it. 47 bytes on every
claude launch, on a line that already spills past MAX_CANON to
`state/launch/<session>.sh`.

**The dir is resolved through `hookRepo`**, and that is the defect this was
one line away from shipping. A dispatched seat's dir is its own linked
worktree under `~/.posse/worktrees`; `SessionTranscripts` compares by
identity; a worktree path matches no config key. Handing `dir` straight to the
reader would have read `on` for every dispatched seat in the marked repo and
`off` only for a crew session in the shared checkout — closed where nobody
needed it, open where the ruling was — and a green unit test, a completing
launch and a config line saying `off` all look right while it is wrong. The
pin for it (`transcriptceiling_qa_test.go` arm 3) is built on a real
`git worktree add` for exactly that reason.

## What it costs, named because it is not nothing

- **Turn outcome.** claude is the only runtime with a reader
  (`TurnOutcomeClaudeTranscript`) and the transcript IS the reader: an
  allotment refusal arrives as a synthetic assistant message, which is how a
  pass tells an exhausted account from an agent that settled without closing
  its bead (ranger-base-02zr). In a marked repo that distinction is gone, and
  dispatch says so on the settle line exactly as it does for a runtime that
  declares no adapter.
- **Cost.** `posse cost` and the cockpit footer price what they can read under
  the transcript root; a seat that writes nothing is spend the ledger does not
  count. ADR 0018 §3's language is right for it — a floor, not a total —
  except that here it is deliberate, so it does not arrive as an
  unreadable-transcript warning. It arrives as nothing at all.
- **Resume** goes too, which costs a dispatched seat nothing: a fresh session
  per bead is the norm (ADR 0003 Dial F).

That ledger is why unmarked keeps the transcript, and why a marked launch says
one line on the warn stream: a channel closed in silence is indistinguishable
from a channel nobody closed, and both readings above are ones an operator
reads later and would otherwise read as broken.

## Gaps, stated

- **Claude only.** `Runtime.TranscriptOffVar` is empty for every other
  built-in, and `TranscriptPin` then renders nothing in either arm — no
  ceiling and no pretence of one. Nobody has measured a suppression surface
  for codex's or grok's own session stores. It costs this instance nothing
  today: all eleven PIDs on this box run claude. It is the first thing to
  measure for a marked repo ever dispatched on another runtime.
- **The container tier is out of scope**, the same way ADR 0077 D1's home pin
  is: that tier takes `WrapInCage` and its own cage HOME, and this box has no
  `cage: container` PID. ADR 0083 D6 leaves the host's cage open for an
  operator ruling, so if that ruling lands on containers this is the line to
  revisit.
- **The interactive arm is not in the live pin.** An interactive session needs
  a live credential to reach a turn at all; with none, claude opens the login
  screen and materializes nothing, so the arm cannot discriminate (MEASURED
  through a pty — no transcript in either arm, a rig that would pass over a
  broken fix). The resolver quoted above is what stands in for it, and a
  credentialed interactive arm is the operator's to run.
