# The store that was not Rex's: the unsent-prompt half of the settled-pane hold (ranger-base-qzujz)

2026-11-10. From ranger-base-r5131, which realized the WORK half of
ranger-base-htafy's `PaneHold` on an occupant with no `agent explain` and
refused the COMPOSER half outright. This is that half, and it opened with a
spike, because the bead's own question was "what is the Rex-side store of
record for *was this text submitted*" and nobody had measured one.

## Why the composer half had to wait

`PaneHold.Typed` is "text sitting unsent in the prompt box", and on its own
it is not a reading — it is a screen-state matcher that cannot go false
(ranger-base-wr624: the pulse skipped ~586 times over ten hours on lines the
operator had already sent). What makes it honest is the ECHO check: claude's
own submit log says which text this pane last submitted, so a box previewing
that line is echoing it, not holding it (sentline.go, ranger-base-2hvtv).
That log is keyed on claude's SESSION ID, and on herdr the id comes from the
multiplexer — `pane get`'s `agent_session.value`, ADR 0076 §4, which answers
"not available" on a `multiplexer: rex` instance.

So a `Typed` reading taken on the capture route would have had no way to ask,
every echo would have read as a hold, and an invented hold keeps a genuinely
stuck bead claimed forever — the one direction panework.go refuses. r5131 said
so and stopped (ADR 0076 Amendment 5, "The composer half stays §4's").

## SPIKE: what carries "was this submitted" on Rex → claude does, keyed on the pid

MEASURED 2026-11-10 on this host (linux, rex CLI 0.0.1794069975, claude
2.1.322, `multiplexer: rex` at `~/homes/ranger/config.yaml:306`), over the
five live claude seats on the box plus one scratch Rex session created and
killed for the arm. The three candidates the bead named, and what each
answered:

1. **`program_status` carries nothing about the conversation.** The root
   record is `state`, `app`, `effective_app`, `title`, `msg`, `updated_at`
   and an `owner` — a pid and a process name. No session id, no transcript
   path. (`msg`/`updated_at` were the bead's own second candidate; `msg` is
   whatever the program last said about itself, which on this box was the
   title of the shell command that wrote the record.)
2. **Rex has no send history.** `rex capture` answers a screen; `rex send`
   answers nothing. A block holds no record of what was typed into it.
3. **claude keeps one file per LIVE session**, which is the answer:
   `$CLAUDE_CONFIG_DIR/sessions/<pid>.json`, carrying `pid`, `sessionId`,
   `cwd`, `startedAt`, `procStart`, `version`, `peerProtocol`, `kind`,
   `entrypoint`, `pidDomain`, `messagingSocketPath`, `name`, `status`,
   `updatedAt`. **Keyed by the pid**, which is exactly what `program_status`
   does carry.

So the Rex-side store of record is not Rex's. It is the runtime's, and Rex's
contribution is the pid.

The chain end to end, on a live posse seat (the one that wrote this):

| link | value |
|---|---|
| block | `block:01a1c83d-544b-7981-a1b6-492d43a87c2e` |
| `program_status` owner | `{pid:2069699,name:"claude"}` |
| `sessions/2069699.json` | `sessionId 0226278e-a243-4f86-9200-5cd648eac89f` |
| `history.jsonl` row | that sessionId, `project` = that seat's worktree |

Also measured on the scratch seat, which proves the join does not depend on
posse having created the seat: owner `{pid:3188672,name:"claude"}` →
`sessions/3188672.json` → `sessionId e3f03c2c-57a5-4ef6-b3c6-a12c7015524a`,
`cwd` the scratch dir.

### The fence

A file NAMED by a pid is a pid-reuse hazard one layer past the one Rex warns
about for its own `owner` ("Compare it as well as pid, because the system
reuses process IDs"). claude writes `procStart` into the row, and it is the
kernel's own start time for that process — the `starttime` field of
`/proc/<pid>/stat`, field 22 counting from one, byte for byte. MEASURED over
all six processes:

```
pid      row's procStart   kernel's starttime
2069699  24545899          24545899
258895   24413296          24413296
3184881  24296903          24296903
3401316  23912539          23912539
900467   24499783          24499783
3188672  24585257          24585257
```

Six for six, exact string matches. So the row is accepted only when the
kernel still agrees about which process that pid is now.

The rows are also REMOVED when the session ends, measured twice in the same
run: 49 `session-env` entries against 5 session rows, and two processes that
exited mid-measurement (pid 3184881, a sibling seat, and 3188672, the scratch
seat after `rex kill`) each had their row gone at the next listing. That is
not relied on — the fence is what makes a stale row harmless, and the cleanup
only makes one rare.

`cwd` is in the row and is deliberately not read: Rex's `process` answer
carries a cwd too, so the two could be compared, but posse does not parse it
(`RexProc` is a pid and a name) and the `procStart` fence already answers
exactly what a cwd check would be approximating. A second, weaker guard
beside an exact one is a second thing to keep true.

## MEASURED: the composer on a captured screen

`rex capture --format text` of a live claude 2.1.322 block, four screens
taken across the states the reading has to tell apart. The composer sits
**between the screen's last two horizontal rules** — one rule up from where
`captureFooter` finds the footer:

```
────────────────────────────────────────────────────  (rule)
❯<U+00A0>                                             an empty box
────────────────────────────────────────────────────  (rule)
                                                      (blank)
  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents (footer)
```

The four composer bodies, verbatim:

```
❯<U+00A0>
❯<U+00A0>Try "create a util logging.py that..."
❯<U+00A0>this prompt is sitting unsent in the box
❯<U+00A0>this prompt is sitting unsent in the box and now it gets long enough to wrap
  across more than one screen line because the terminal is eighty columns
  wide and this sentence is not
```

The mark is `❯` + U+00A0 (`E2 9D AF C2 A0`), the same two characters
panework.go's `composerMark` reads on herdr.

### A capture is whole and wrapped; a preview is truncated and not

That fourth screen is the whole finding. herdr's region preview is one string
cut at 243 characters, so `submittedEcho` compares for equality and accepts a
prefix only when herdr says it cut the region. A capture is never cut and is
always wrapped, at the terminal's width, at spaces that are IN the submitted
text. One prompt, three screen lines, continuations indented two.

So the capture route rejoins the lines and collapses whitespace runs on both
sides of the comparison, which makes a screen wrap and a real space
indistinguishable in the way they are indistinguishable on screen. The rule
itself is still `submittedEcho`'s — one answer to "is this the line that was
submitted", spelled once — and what is named separately is the artefact being
removed before it is asked.

What that cannot see, said out loud: a line claude broke MID-WORD, which
needs an unbroken token wider than the terminal. The collapse then leaves a
space the submitted text does not have, the echo is missed, and the text
stays a hold — a withheld settle rather than an invented one. Un-wrapping
properly would need the terminal's width, and the width is not in the
capture.

## MEASURED: half of ghostbox.go's missing half

`ghostbox.go` tells claude's own next-prompt suggestion from typed text by
the SGR 2 (faint) run claude draws it under, and its corpus has an explicit
hole: 60 dim suggestions, 17 empty boxes, and **a typed unsent line measured
zero times**, because typing into a live pane is typing into somebody's seat
and posse's own sends submit. In a scratch Rex session that arm is cheap, and
it was taken — `rex capture --format vt`, same session, minutes apart:

```
❯<U+00A0>\e[0m\e[2mTry "create a util logging.py that..."\e[0m   the suggestion: dim
❯<U+00A0>this prompt is sitting unsent in the box                 typed: NO SGR at all
```

One observation, one runtime, one occupant — so the reading still only
RETIRES a claim and still licenses no keystroke, and
`scripts/verify-ghost-composer.sh` is still what closes this on herdr. But
the direction the corpus could not sample now has a sample in it, and it is
the direction the mechanism assumed.

### And one thing herdr's read does not do

`rex capture --format vt` spells a REDRAWN line's gaps as cursor moves, not
as spaces. The wrapped prompt's first line came back as:

```
❯<U+00A0>this\e[1Cprompt\e[1Cis\e[1Csitting\e[1Cunsent\e[1Cin\e[1Cthe\e[1Cbox…
```

where the short form of the same prompt came back with literal spaces, and
where herdr's `agent read --format ansi` was measured with literal spaces
throughout. `scanSGRDim` drops every escape it is not reading, so without a
normalization the words run together, the join against the plain reading
fails, and a box that IS claude's own suggestion reads as a hold nobody can
clear — ranger-base-6o7wm's defect, back on this occupant. The capture route
expands `CSI n C` to n spaces before handing the screen to that rule. The
normalization is this route's and not ghostbox's: the dim rule is a measured
corpus, and widening its escape scan would change what that corpus was
measured through.

## What landed

- `internal/posse/runtimesession.go` — the join: `sessions/<pid>.json`, the
  fence, and `rexAgentSession` off `RexMux.RuntimeOwner`.
- `internal/posse/procstart_linux.go`, `procstart_other.go` — the kernel's
  start time, and the honest refusal on a platform with no `/proc`. A
  refusal means the echo reading stays off and the composer is judged
  exactly as it was before this bead.
- `internal/posse/muxrex.go` — `RexMux.RuntimeOwner`, the one new Rex-only
  verb: the root record's owner once ADR 0076 §3's three guards have passed
  on it, an error otherwise and never a zero owner.
- `internal/posse/herdrback.go` — `paneAgentSession` grows the second route
  behind the same door, so its caller never learns which multiplexer
  answered.
- `internal/posse/capturedcomposer.go` — the reading: the region between the
  last two rules, the wrap-insensitive echo, the cursor-move expansion, and
  the three fields assembled off ONE capture.
- `internal/posse/panework.go` — `capturedHold` fills the composer half too.
- ADR 0076, Amendment 6 — §4 gains one sentence: a §4 verb that names a fact
  the RUNTIME owns may be answered on another occupant by asking the runtime,
  through the door that already spells the verb.

The two consequences the bead named needed no change at all: dispatch's
`--resume` skip and govern's G2 `-unsent` row both consume `PaneHold`, so
they came live the moment the reading did.

## What the scratch venue cost, and why there was one

One `rex new` in a scratch cwd, `claude --model claude-haiku-5-5`, text typed
and never submitted, then `rex kill`. Nothing was ever sent to the model, so
the arm spent no inference; the process and its session row were both gone at
the next listing. It was a venue and not a sibling's seat on purpose: typing
into a live pane's composer is typing into somebody's work, which is exactly
why ranger-base-6o7wm's missing arm stayed missing for two months.
