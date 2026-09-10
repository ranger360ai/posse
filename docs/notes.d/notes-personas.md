## Personas

`agents/<name>.md`: flat-YAML frontmatter (`name`, `description`,
`runtime`, `labels`; PID keys `intents`, `allow`, `deny`, `metrics` —
see `docs/adr/0001-persona-intent-documents.md`; `envs`, the env sets its
sessions receive; `tier`/`tier_floor` (ADR 0003); `cage`, `writable`,
`egress` (ADR 0002 §5 — `cage:` is the minimum wall tier, `egress:`
implies `container`; the example PIDs that deny Edit/Write declare
`cage: seatbelt`); `skills` (ADR 0007 — see **Skills** below);
`command`, the escape hatch) + a markdown body that
*is* the prompt. Body sections are the ADR 0001 contract order plus an
optional `## Work prompt` (ADR 0005): the persona's standing per-bead
instruction, appended verbatim to every dispatched work prompt.

**Runtimes (ADR 0002 §1–2, `internal/posse/runtime.go`).** A persona
launches on a named launch profile, not a command string. Built-ins
`claude`, `codex`, `grok` each carry a template and a *native realizer*
for `{allow}`/`{deny}`: claude → `--allowedTools r…`/`--disallowedTools
r…`; grok → `--allow r`/`--deny r` per rule; codex → `-s read-only` when
the deny list covers `Edit`+`Write`, else `-s workspace-write` (always
emitted; codex has no verb rules). Native flags are politeness (L0), not
the wall — the wall is the gates bead (§3). `RHQ_HOME/runtimes/<name>.yaml`
(`command:` only) adds a template-only runtime with no realizer:
`{allow}`/`{deny}` render to nothing there and every gate goes to the
wall.

Claude's `{deny}` is *widened* on the way out (`L0Spellings`,
rangerhq-3mc). Claude matches a `Bash(...)` rule three ways — exact, `:*`
as a prefix of the argv **tokens** (not of the command string:
`Bash(sed -n:*)` does not reach `sed -ni 1p f.txt` and does reach
`sed -n -i.bak …`, measured on 2.1.241, ranger-base-g8e), and `*` as a
wildcard (`.*`, anchored, whitespace collapsed). A whole-verb deny
(`Bash(bd)`, which claude reads as *exact*) therefore ships `Bash(bd:*)`
alongside, so `bd show x` is refused too, and a negative rule — which the
dialect cannot say at all — ships as one exact spelling (below). `allow:`
is never widened (that would grant more than the PID says),
`RHQ_TOOLS_DENY` still carries the PID's own rules, and parity is
unchanged: L0 is politeness, never the wall.

A *subcommand* deny gets nothing but itself, and that is the end of a
two-step retreat. It shipped as an option-blind **pair** —
`Bash(git -* push)` for the bare spelling and `Bash(git -* push *)` for
the same verb with further arguments — because `git -C <repo> push`
matches neither the string nor the token prefix `git push`. `*` is `.*`,
so `-*` is a `-` and then *anything*, not an option run, and each half
proved it:

- **`Bash(git -* push)`** matched any `git -…` command whose last word was
  one of them — `git -C <r> log --grep push` and `git -C <r> stash push`
  refused, `--grep=push` running. Gone in **rangerhq-ky3**.
- **`Bash(git -* push *)`** matched any ` push ` token after a leading
  global option, subcommand or not — `git -C <r> stash push -m wip` and
  `git -C push status -s` (where `push` is `-C`'s value) refused on claude
  2.1.239 and grok 1.0.5, with `git stash push` (no leading option)
  running as the control. Gone in **rangerhq-vr6j**.

The L1 shim refuses none of those four: it skips the global options and
matches the first non-option token, so the wildcard half was blocking argv
the wall deliberately lets through. And nothing in the dialect separates
them from a real push — no negation, no way to say "option tokens only" —
so the false positives go and their coverage with them: `git <globals>
push …`, arguments or not, draws no polite refusal now, only L1's hard one
(`TestShimSkipsGlobalOptionsBeforeSubcommand` holds every spelling). The
same shape had already been dropped from the negative rule's widening for
want of a fallback half (ranger-base-xll2). Grok's realizer deliberately
does **not** widen at all: its dialect is verified (rangerhq-625) and the
wildcard is real, but grok matches a shell-parsed segment with the quotes
off, which turned the pair into a false-positive generator there — and
with the pair retired the reason is still live, because a rule with no
wildcard is a *prefix* on grok, so the negative rule's `Bash(git commit)`
would refuse the qualified `git commit -- <path>` the PID allows. See
*Grok specifics*. Templates:

```
claude: claude {model} --append-system-prompt "$(cat {file})" --add-dir {memory} {settings} {skills} {allow} {deny}
codex:  codex {model} {skills} {deny} -a never --disable hooks -c allow_login_shell=false -c "projects={\"$PWD\"={trust_level=\"trusted\"}}" -c developer_instructions="$(cat {file})"
grok:   grok {model} {skills} --permission-mode auto --rules="$(cat {file})" {allow} {deny}
```

**The dispatch contract (ADR 0013, `internal/posse/runtimecheck.go`).** A
runtime that *launches* safely is not the same claim as a runtime that can
*take work*, and one evening of two non-claude runtimes in production broke
the second claim about once an hour. So dispatch names six stages —

```
launch → promptable → work → record → settle → account
```

— of which four are observed (herdr, the bead, the cost adapter) and two
are **declared** per runtime, in the built-in table or in
`runtimes/<name>.yaml`. The settle stage has a declared half too
(`turn_outcome:`): herdr sees the pane settle either way, and only the
runtime's own record says whether a turn actually ran.

| key | values | what it says |
|---|---|---|
| `prompt:` | `typed` (default) / `argv` | how dispatch delivers the work prompt: type it into a promptable screen, or append the prompt file to the launch line so no screen is the delivery channel |
| `startup_wait:` | duration, default 45s | how long a launch may take to reach a promptable screen. Measured per runtime — 45s is a *claude* number |
| `record:` (+ `record_why:`) | `untrusted` (default) / `trusted` | whether a **dispatched** session of this runtime has been MEASURED to close its bead |
| `native_rules:` | file names | the rulebooks this CLI discovers by itself, ahead of anything posse types |
| `turn_outcome:` | a reader name (`claude-transcript`, `grok-session-store`), default none | whether posse can read what this runtime's own first turn DID — the fact that separates an exhausted account from an agent that worked and skipped the bead |

`posse runtime check <name>` prints the grid: each stage's observable, who
declared it, and what a missing one costs — always a named degrade or a
named refuse, never a patch. **This is how a runtime is onboarded**: fill
the grid rather than discover each quirk in production.

The unknown runtime is the case the command exists for, and the zero value
of every declaration is the expensive-to-be-wrong-about direction. A
`runtimes/<name>.yaml` with nothing but `command:` is `prompt: typed`,
`record: untrusted`, uncounted and tier-unmapped — **dispatchable and
loud**, every row naming the key that would change it. A *present but
misspelled* value is the opposite case and refuses at load: `record:
trused` silently reading as untrusted is exactly the silence this contract
removes.

`turn_outcome:` is a **registry key, not prose**: the value names a reader
that exists in `internal/posse/turnfailure.go` (today `claude-transcript`
and `grok-session-store`), and a value no reader implements refuses at load for
the same reason `record: trused` does — a declaration that promises a
reading nothing performs is worse than no declaration.

`native_rules:` is a declaration, not a switch — and codex 0.147.0 *has*
a switch (`-c project_doc_max_bytes=0` drops the project `AGENTS.md` from
the model-visible prompt; measured, `ranger-base-cl7`) that dispatch
deliberately does not use (ADR 0013 §4, decided on `ranger-base-00f`):
grok has no equivalent so nothing fleet-wide would be gained, the key
could rename and silently re-enable discovery, and the file is the
operator's — suppressing it only under dispatch would make posse sessions
disagree with hand-run ones about which of the operator's documents
apply. An operator who wants codex doc-free owns that choice:
`project_doc_max_bytes = 0` in `~/.codex/config.toml` (instance-wide), or
the `-c` override in their own `runtimes/<name>.yaml` `command:`. Posse
documents the key and never writes it.

### Argv delivery: the work prompt rides in on the launch line

The probe (`ranger-base-cl7`, trace in `docs/adr/0013-argv-prompt-probe.md`)
held on both non-claude runtimes on 2026-08-25, so **grok and codex are
`prompt: argv`** and claude stays `prompt: typed` (it works; ADR 0013 calls
argv there an allowed later unify). Neither argv runtime carries a
`startup_wait:` — that number is the *typed* ladder's patience, and they do
not use that ladder.

Why it is not a nicety. A grok pane that has not had a turn emits no OSC
title, no OSC progress and no composer footer, so it matches **no** herdr
rule and herdr answers with its idle guess; the same pane after one turn
matches three rules at once. Detectability is a property of *having been
prompted* — so there is no value of `startup_wait:` that turns waiting into
a promptable screen there, and the typed fallback ADR 0013 §2 describes
could not have been made to work with a bigger number.

The path, in `dispatch.go` (`launchWithPrompt`) and `argvprompt.go`:

1. **Claim first** — the fence. A lost claim creates nothing: no workspace,
   no prompt file, no persona sitting in a repo working someone else's bead.
2. **Write** the assembled ADR 0005 work prompt to
   `$RHQ_HOME/state/prompts/<session>.txt` (0600). The prompt does not go on
   the line itself: it is a page of text and a fresh pane's shell is a tty
   with a line limit (rangerhq-ybec).
3. **Create** the session with `"$(cat <file>)"` appended to the *runtime's*
   rendered command — before the seatbelt prefix, the gates prefix and the
   cage wrap, because it is an argument to the CLI and not to `sandbox-exec`
   or `docker run`. There is no `{prompt}` placeholder: an unrendered token
   would be a literal argv (ADR 0001/0002's lesson, and ADR 0013 rejects it
   by name). At the container tier the file is mounted same-path read-only,
   since that `$(cat …)` is expanded by the container's own shell.
4. **Await a state herdr has SEEN** — a matched rule, not the idle fallback.
   This is not a readiness gate; nothing is waiting to be typed. It is the
   evidence that the launch line ran and a turn started.

Three failures, three different cleanups, and the differences are the point:

- **create fails after the claim** → unclaim. The claim went first so a race
  loses cleanly; the price of that is handing the bead back.
- **no agent pane ever appears** → the CLI never started, nothing read the
  prompt file, nobody is working the bead → unclaim.
- **a pane, but no rule matched in time** → the prompt IS with the CLI.
  Claim kept, bead not judged this pass, and **no settle-wait is started**:
  a wait over herdr's idle guess returns instantly and would read a session
  that never worked as one that settled.

The prompt files are left behind on purpose. A typed prompt at least echoes
into the pane's scrollback; an argv one is consumed by the exec before any
screen exists, so the file is the only record of what a session was asked.
One small file per bead, rewritten on re-dispatch.

**Resume is still typed.** `--resume` into a live session, and the cockpit's
`d` on a live holder, prompt the composer: the launch line has already been
typed, and the only way into a running CLI is through its screen.

### When a launch is not promptable, say what herdr was looking at

Argv retires the screen as the *delivery* channel; it does not retire the
screen. A launch can still end with herdr recognizing nothing, and until
`ranger-base-3j8` the only thing dispatch said about that was:

```
herdr never saw a screen it recognizes there, only "idle"
(default_known_agent_idle_fallback)
```

Honest, and useless. Three different screens produced that identical
sentence in one evening — a consent banner, a version splash, and a pane
whose OSC chrome grok had simply not emitted yet — and two of them needed
opposite fixes. Telling them apart cost a hand-launch and a `posse peek`
each time.

herdr already had the answer. `agent explain --json` carries
`evaluated_rules`: every rule it tried, the screen **region** that rule
reads, and how many bytes were in that region with a preview. Both
promptability failures (typed `awaitSettled`, argv `awaitDelivered`) now
append it, grouped by region — rules outnumber regions three to one and
share their evidence, so twelve rows of which eleven repeat is a
diagnostic nobody reads:

```
herdr evaluated 15 rules there and matched none. What it was reading:
  osc_title                      0 bytes  ""  — osc_title_blocked, osc_title_idle, osc_title_working
  bottom_non_empty_lines(2)    601 bytes  "╰── Grok 4.6 (high) ─╯ ..."  — permission_hints_blocked, +3 more
  whole_recent                3969 bytes  "\ue0a0 main ~/src/posse ╭──╮ │ ..."  — startup_splash, +4 more
```

Read it as: **an empty region means the CLI has not spoken yet**; a region
full of text means it is up and parked on a screen posse cannot name. On
grok those are the two halves of this bead. The row above is the real one,
and `╰── Grok 4.6 (high) ─╯` with no `· auto` is the tell the coordinator
found by hand.

Runs of four or more identical non-alphanumeric runes collapse to two
before the preview is cut: on the wide grok splash, 70 of the first 72
characters were one repeated box rule, so the row previewed nothing.
Letters and digits are never collapsed.

This is **diagnosis only**. Nothing decides on it and no key is pressed
because of it — interstitials stay on the argv sidestep and the operator's
own config (ADR 0013 §2, and the interstitial section below). A herdr that
does not emit `evaluated_rules` leaves the old sentence exactly as it was.

### The busy key: session failure vs persona failure (ADR 0013 §2)

Dial F gives every bead its own session, so a *pane* failing is not a fact
about the *persona*. Dispatch splits the three outcomes of a failed launch:

| outcome | what it is | slot |
|---|---|---|
| claim lost | the **bead**'s — somebody else holds it | free |
| session failure — no agent, never promptable, unknown screen | this **pane**'s | **free once**; the next bead gets its own fresh session, and the pass's *second* such failure benches the slot (the ceiling below) |
| anything else — runtime will not load, exe missing, cage credential, gates the wall cannot realize | the **persona** on this runtime | benched for the pass |

A pane the pass gave up on is also remembered for the pass, so the
working/blocked guard does not read a session left sitting on a splash
(which herdr reports `blocked`) as the persona being busy — that would put
the sterilise back one guard further down.

**The cost, and its ceiling.** Before the split, one failed launch benched
the persona and the pass paid one startup wait; the split let every bead's
own launch pay its own — deliberately, because one grok cold start taking a
persona's whole queue out of the pass was the failure being fixed
(`ranger-base-3j8`), superseding rangerhq-vk2's "one detection timeout per
pass" rule from the one-session-per-repo era. Unbounded, though, a persona
whose CLI is broken (exe missing, instant crash, auth exit — all of which
look exactly like "no agent detected" from herdr's side) spends a
serialized startup wait per ready bead, and `-n` defaults to unlimited. So
the pane-local explanation gets exactly one retry: the **second** session
failure of a slot in one pass benches the slot for the rest of the pass
(ADR 0013 §2 "Ceiling", `ranger-base-8h5p`). Two is the floor that keeps
the 3j8 fix and the ceiling that caps the drain; the benign slow-start case
lands in `awaitDelivered`'s seen=false outcome on argv runtimes, not here.
An exec-preflight gate was rejected in the same amendment — posse's PATH is
not the pane's — see the ADR's alternatives.

`record:` is where grok and codex differ, and only for a measured reason:
the qa lane on grok closed a dispatched bead properly, and 3/3 dispatched
codex sessions did the work and left the bead `in_progress` with no comment
(`ranger-base-0fb`). Trust is a built-in/yaml edit after a measurement —
never a derived store that could disagree with the bead (ADR 0011).

### What a settle-without-close costs, and what it does not (ADR 0013 §4)

The tick is the **bead's** to give. A pass prints `✓` only when `bd show`
says `closed`, on every runtime, trusted or not — an agent going idle is a
hint (ADR 0011), and on codex that hint was wrong three times out of three.
Everything else settles as `◑ <id> settled "done" but issue is
"in_progress" — review <session>`, and on a `record: untrusted` runtime the
line adds whose declared degrade it is:

```
◑ a-1   settled "done" but issue is "in_progress" — review ranger-posse-a-1
        (codex is record: untrusted — the claim is kept and --resume re-prompts it)
```

Three things that clause is telling the operator, and each of them is a
decision made elsewhere in this file: the bead **keeps its claim** (nothing
is handed back on a settle a pass could not judge), unattended `--resume`
**re-prompts the same session** next pass rather than parking it behind a
busy key (`autostart_resume:` above), and the harness **does not close the
bead** on the agent's behalf — that hides the defect and puts a human back
in the loop dispatch exists to replace. A `record: trusted` runtime gets the
same honest `◑` and no clause: a runtime measured to close its beads that
has stopped closing them is a signal, not a footnote.
(`internal/posse/recordskip_qa_test.go`.)

### The other half of that line: what posse cannot see (ADR 0013 §1 settle)

A settled pane is not a turn. Claude writes an allotment refusal as a
synthetic assistant message, so a pass that reads the transcript can stop
the bead with `⛔ … refused the first turn … no work ran` and tag the
session — but that read is a per-runtime **declaration** (`turn_outcome:`),
not a property of the name `claude`. Until `ranger-base-02zr` it was keyed
on the name, so the reader was never even asked on codex or grok, and an
exhausted account there printed as an ordinary settle-without-close.
MEASURED the same day: grok's account was answering `402 Payment Required`
while a pass called it a settle.

A runtime that declares no reader now says so on the bead's own line:

```
◑ a-1   settled "idle" but issue is "in_progress" — review ranger-posse-a-1
        (codex is record: untrusted — the claim is kept and --resume re-prompts it;
         posse reads no turn outcome on codex — an account that refused the turn
         settles exactly like this, so posse peek ranger-posse-a-1 before reading
         it as work that ran)
```

Both clauses can be true at once and they say different things: the first
is a **declared degrade** (nothing was lost, `--resume` retries), the
second is a **fact posse does not have**. Neither is a verdict — the two
causes are still one `posse peek` apart, and a harness that guessed here
would be guessing exactly where it just admitted it cannot see. The
per-pass half of the same honesty is the account-degraded report (ADR 0013
§5); this is its per-bead half.

grok's reader was built once the artifact behind it was captured
(`ranger-base-e123`'s probe, then `ranger-base-fc8go`): grok does NOT write
its refusal into a transcript — a refused turn leaves `chat_history.jsonl`
silent — so `turn_outcome: grok-session-store` reads the typed record in
`$GROK_HOME/sessions/<cwd>/<id>/updates.jsonl` instead
(`internal/posse/turnfailure_grok.go`). codex writes
`~/.codex/sessions/*.jsonl` (`ranger-base-xaev`) and is reachable the same
way, but its refusal artifact has never been captured — its account was
alive when the probe ran, and forcing one means spending until a quota
trips — so it declares none, and the order matters: a reader written over a
shape nobody measured is worse than the blindness it replaces, which is why
codex's line above is still the honest one. (`internal/posse/turnoutcome_qa_test.go`,
`internal/posse/turnoutcomegrok_qa_test.go`.)

Reading grok's record is also what made the ⛔ line's `no work ran` a lie
worth fixing (`ranger-base-qcu4c`). That phrase was written for claude,
where the refusal IS the whole turn — a synthetic message in place of a
first answer, nothing before it. On grok it is not always true: 1 of the 7
refusals in this box's history carries a full `usage` object, six model
calls and 190,817 tokens into a turn that had been running for ninety
seconds when the account went out from under it, and that session may well
have edited files and commented on the bead. The line has two arms now, and
which one prints is the runtime's own record's to say:

```
⛔ a-1   grok refused the turn mid-flight: API error (status 402 Payment
         Required): … — the turn had already run (6 model calls, 5571 output
         tokens), so work may exist: posse peek ranger-posse-a-1 and check
         the worktree before relaunching at another tier
```

The flat `refused the first turn … no work ran` stays for a refusal with
nothing behind it, which is claude's shape and 6 of grok's 7. Absence of
`usage` is read as "nothing ran" because grok writes one for every turn that
served anything (186/186 on this box, all nonzero in every field, censused
2026-09-05); a reader that cannot tell must say so, since the line reads a
zero as a claim and not as ignorance. The claude mirror was filed as open and
capture-shaped — a refusal landing *after* a first answer being invisible to
`FindClaudeTurnOutcome`, which stopped at the first assistant record — and it
turned out not to need a capture at all (`ranger-base-4ldma`). Censusing all
1755 claude transcripts on this box on 2026-09-05 found the artifact already
on disk, eleven times over: of the 13 allotment refusals inside a dispatch
turn, **11 land after real work** and only 2 are the first answer. Six
dispatched beads — `vtyst` (33 model calls, 24,740 output tokens before the
refusal), `frqmn` (27 / 18,417), `felmj` (27 / 20,230), `oujxl` (25 /
11,415), `2dzsm` (17 / 28,070), `pwtix` (15 / 6,250) — each settled with the
reader reporting a healthy turn, so each printed an ordinary
settle-without-close and cleared any marker a previous pass had set. The two
first-answer refusals are the records the reader was built from, which is how
their shape became the rule. It reads the whole turn now: the turn is closed
by the next work prompt or by end of file, never by the tool_result records
its own tool calls put in the user channel, and claude's work fields are the
transcript's `usage` objects deduped by message id — one model call arrives
as several records repeating one growing usage object, so summing the records
would report three calls where one ran
(`internal/posse/turnoutcomeclaude_qa_test.go`).

### The reap guard: dirty tree + open bead is not killed (ADR 0013 §4)

`posse kill <name>` — and the kill inside `posse relaunch` — **refuses** a
session whose `bead:` is still `in_progress` and whose working directory
holds uncommitted work:

```
NOT killed: ranger-posse-a-1 still holds a-1 (in_progress) and ~/src/posse has
uncommitted work (dispatch.go NOTES.md) — look first (posse attach ranger-posse-a-1),
or `posse kill ranger-posse-a-1 --force`
```

The near-miss it is built from is `ranger-base-0fb`: a dispatched session
that skipped its bookkeeping *and* left 353 uncommitted lines in the
**shared** checkout, one reap away from gone. Per-session worktrees do not
cover that board — a shared-checkout session has no branch, so the landing's
existing "a tree still holding work is kept" refusal never fires — and L3's
pathspec rule stops an unqualified commit, not a kill.

Both arms must fire, and neither is a new store:

- the **bead**, read from bd every time. `bead:` in the session meta is only
  a *pointer* to which one, stamped by dispatch at launch (and on the
  session it resumes into), so the meta can never disagree with the store
  about whether the work is done. A session with no pointer — `posse new`,
  a crew session, a recipe, anything from before this landed — is unguarded,
  exactly as it was.
- **git**, `status --porcelain` in the session's own cwd. Uncommitted is the
  only shape of loss a kill can cause: committed work survives on a branch,
  and a clean tree has nothing to lose however open its bead.

An open bead over a clean tree is a bookkeeping skip — gather's line to
print and `--resume`'s to retry, not a reason to keep a workspace alive. A
dirty tree under a closed bead is the operator's own scratch. Ignorance
*inside* the pair fails **closed**: a pointer, a dirty tree and a bd that
will not answer is refused too, on `RemoveSessionTree`'s rule — the safe
answer to an unanswerable question about destroying work is no, and the
costs are asymmetric (a wrong refusal is one `--force`; a wrong kill is
work with no other copy). `--force` on either command stands the guard
down and nothing else — the landing still refuses to *remove* a tree that
holds work. (`internal/posse/reapguard.go`, `reapguard_qa_test.go`.)

### What the auto-reap takes, and what it will never take (ranger-base-f6lk)

The end-of-pass sweep (`internal/posse/autoreap.go`) has three arms. All three
first ask the same two questions — herdr says nobody is working in there
(`idle`/`done`, or no agent at all past `RelaunchGrace`), and no launcher
prompted it inside `PromptGrace` — and then differ in what evidence they need
that the session is *finished*:

| arm | population | needs |
|---|---|---|
| pointer | dispatch's own per-bead session | its bead reads **closed** |
| crew | a crew mark on a session **dispatch made** | closed bead + `reap_crew_after:` (4h) + a tree holding nothing |
| unpointed | a per-bead-named session with **no** `bead:` pointer | `reap_unpointed_after:` (1h) + a tree holding nothing, and only at a sweep **past routing** |

Never taken, at any age: a conversation the operator MADE (`posse new` — the
crew arm takes only the name `SessionForBead` renders from the session's own
`agent:`/`dir:`/`bead:` record, so an operator-chosen name is out of reach);
`pulse_persona:`'s session (ADR 0027 has nowhere else to deliver); the
persona's reusable `<persona>-<repo>` slot; a foreign row. `off` / `never` on
either grace restores the permanent skip those two arms used to be.

**The unpointed arm waits for routing**, and that is the one place the two
widened arms differ in kind. Dispatch reaches a session by NAME —
`SessionForBead` for the bead it is about to work — pointer or no pointer, so
a stampless session at a live bead's Dial F name is a seat this pass is about
to relaunch into and reuse (rangerhq-vk2), not a dead shell. The pass-start
sweep cannot tell those apart; a sweep past routing does not have to, because
anything the pass used was either prompted (`promptedRecently`) or resumed
into, and a resume stamps the pointer (`NoteBead`) and takes the session out
of the population. The cost is that a pass which dies in gather sweeps no
stampless residue — but a quiet pass, which is the steady state this residue
accumulates across, reaches its epilogue in seconds. The crew arm keeps both
sites: its bead is closed, and a closed bead is never dispatched again.

**"A tree holding nothing" is `RemoveSessionTree`'s refusal asked as a
question** (`residueHolds`): no uncommitted paths in the session's cwd, and —
for a worktree session — no commits the base does not hold, measured by
patch-id AND by content (ranger-base-as19/x8jp; git's `-x` trailer is
somebody's decision, not a measurement). Every unanswerable question fails
closed. It is stricter than the kill that follows needs to be: a crew-arm
session's bead is closed, so the kill would land the branch itself, but a reap
that lands is a reap that decides. Deferring costs one pass —
`landClosedTrees` lands it at the head of the next, a DETACHED tree included
(ranger-base-vavx2: that sweep asked `<base>..<branch>` alone too, and skipped
such a tree in silence before its bead was read — `nothingToLand` asks both
tips now) — and the refusal prints
`◑ <session> idle <d> over <why> and NOT reaped: <what it holds>` **every
pass**, because the silence is what read as a broken reaper and cost the
hand-reaps in the first place (ranger-base-kftx).

**Asked of BOTH of a tree's tips** (`removalTips`, ranger-base-v2rj7): the
branch, which is what `branch -D` deletes, and the tree's own HEAD, which
is what `worktree remove` drops. On a **detached** HEAD those are different
commits — a commit made there writes no ref at all, which is exactly why a
container-tier session is launched detached (`PrepareSessionHead`,
ranger-base-t4f1) — so `<base>..<branch>` is ZERO over a tree holding a whole
session's work. Both guards asked only that, and MEASURED 2026-09-05 before
the fix: `RemoveSessionTree(t, false)` over a stamped, clean, detached tree
with one commit on it returned nil, removed the worktree, deleted the branch
and left the commit referenced by nothing. Not a live loss the day it was
found — the settle path runs `MergeSessionWork` first and that splices — but
a guard that holds on its caller's evidence is not a guard, and ADR 0058 makes
the sweep a second caller. The same wrong question is fixed at the listing and
the merge report in ranger-base-d8o6 (`treeState`, `landed`). Asking the head
is a no-op for a tree whose HEAD is on its own branch, so this is the detached
case and only it; the branch is still asked in its own right, because a branch
holding a commit its worktree walked away from is `branch -D`'s to lose.

The two graces are policy dials, not measurements of anything: nothing posse
records says how long a conversation's gaps are (typing in a pane leaves no
stamp — ADR 0008 §1 accepted that when it refused a timer), so the crew grace
is set long and the unpointed one, which protects much less, short. The age
itself is the later of `launched:` and `prompted:`; a record with neither has
no age, and no age is not old enough.

### The ownership refusal: a foreign row is not this home's to kill

The kill's second refusal, and a different question from the first
(rangerhq-selx). `posse kill <name>` resolves by *label*, and `Resolve`
falls through to **foreign** rows — live herdr workspaces this `RHQ_HOME`
holds no session meta for. That fallback is deliberate: `posse peek`,
focus and the listings exist to show the whole herd, including rows posse
did not make. Following it into a *destructive* path is the bug. Measured
across two homes on one herdr server: instance A's `posse kill m1-collide`
closed instance B's live workspace — exit 0, no warning, no ownership check
— while the *create* one command earlier had refused the same name
correctly. The resolver could always see the row was not A's; only the
destructive path declined to act on it.

So `posse kill` and the cockpit's `x` ask `ForeignKillRefusal` before the
close, and name the **workspace id** in the refusal — the id is what an
operator can carry to `herdr workspace list` or to the other home, because
the *name* is precisely what is not unique across instances:

```
NOT killed: dispatch is a foreign workspace (w7) — this posse home holds no
session meta for it, so it belongs to another instance or was made in herdr by
hand; close it where it lives, or `posse kill dispatch --foreign` to close it
from here anyway
```

Two flags, because they are two facts and reading one refusal is no
evidence about the other: `--force` says "I have looked at *my* session's
unfinished work" (the reap guard above), `--foreign` says "I mean the row
that is not this home's". A foreign row carries no meta and so never
reaches the reap guard at all, which is exactly why `--force` must not
carry it — a flag typed from habit about one's own dirty tree would
otherwise close another instance's live agent. The cockpit has no override
key; `x` on a foreign row refuses at the keypress rather than asking to
confirm a kill the backend will refuse anyway, and points at the CLI.

`plugin/autostart.sh` is the reachable caller: its `--startup` husk
replacement kills the autostart session by name, so on a shared server it
was one restart away from closing the other instance's dispatch loop. It
already ignores a failed kill and re-checks, so the refusal surfaces as
`<session> still present after kill — not started` — a hook that does not
start beats a hook that kills the wrong loop.

What no override can repair is the other side's bookkeeping: the owning
home's `state/herdr/<name>.yaml` still points at the workspace that was
closed, and that file is outside this home. Its own next listing prunes it
(ADR 0011 §2). One more reason the refusal is the default and the flag is
the exception. (`ForeignKillRefusal` in `internal/posse/herdrback.go`,
`internal/posse/foreignkill_qa_test.go`; the launch half is rangerhq-ynx8's
`foreignHeld`.)

Which runtime a session gets: `posse new --runtime` / `posse dispatch
--runtime` > recipe `runtime:` > PID `runtime:` > config
`default_runtime:` > `claude`. A PID's `command:` is the template for
*its own* runtime only; an override to another runtime uses that
runtime's built-in template (a claude-shaped `command:` on codex would be
nonsense). PIDs should say `runtime:` and drop `command:` — the scaffold
and `examples/agents/*` do. Persona sessions get `RHQ_RUNTIME` in the
env, `runtime:` in the session meta (`RelaunchAgent` re-renders for the
same runtime after a crash), and the runtime's emoji from config `emoji:`
so the cockpit shows what the persona runs on (`🎭name@codex` when not
claude). `posse runtimes` lists profiles. Verified flags on this machine:
codex — see **Codex specifics** below; grok — see **Grok specifics**.

**Skills (ADR 0007, `internal/posse/skills.go`).** `skills: [dataviz,
code-review]` on a PID binds those skills to the persona on whatever it
launches as — the cross-agent binding posse owns, as against the
per-user-per-runtime accident of "whatever this machine installed into
this CLI". A name resolves to `RHQ_HOME/skills/<name>/SKILL.md` or it is
unknown; that directory *is* the registry (real Agent-Skills dirs or
symlinks to `~/.claude/skills/x`, a plugin's `skills/x`, a repo — posse
indexes nothing and copies nothing INTO it, so `posse skills` is `ls` and
`posse agent check` is `stat`).

At launch the binding is materialized fresh, exactly as the gates are, in
one of two shapes — which one is a property of the *runtime*, not of the
PID:

- **A flag at a rendered tree (claude).** `RHQ_HOME/state/skills/<persona>/claude/`
  gets `.claude-plugin/plugin.json` plus `skills/<name>` — a real directory
  of files, COPIED out of the registry at every launch — and `{skills}`
  renders `--plugin-dir <that dir>` (session-only, additive, verified:
  `claude --plugin-dir <tree> plugin details posse-<persona>` lists the
  bound skills). `--add-dir` is CLAUDE.md dirs and does **not** load skills.
  A `runtimes/<name>.yaml` opts into the same shape with `skills_flag:` (a
  printf form, as `model_flag:` — `--foo` renders separated, `--foo=%s`
  glued) and is handed the same dir — the layout inside it is the universal
  Agent-Skills shape and the plugin.json is inert to anything that does not
  read it.
  **Copied, not symlinked, and that is the whole of what a second runtime
  needs from this shape** (ranger-base-65rc). The entries used to be
  symlinks into `RHQ_HOME/skills`, which made the "universal layout" promise
  a claim about the READER: grok 1.0.5 validated the tree, installed it, and
  reported `Skills (0)` — a `skills_flag:` runtime whose loader behaves that
  way launches clean, with `skills:` in Realized and the persona holding
  nothing, which is exactly the failure ADR 0007 §3 spends a refusal on,
  arriving through the accepted path. The same tree with real files reports
  both. claude dereferences, so the one CLI the surface had been exercised
  on was the one that hides it. The copy keeps each file's mode (a skill may
  ship a script), refuses a skill dir that links back into itself, and costs
  one directory copy per launch, beside the gates render.
- **No flag, symlinks in the session dir (codex, grok).** Both CLIs
  discover skills from their *working directory*, so the launch links
  `<session dir>/.agents/skills/<name>` → `RHQ_HOME/skills/<name>` and
  adds `/.agents/skills/` to the repo's `.git/info/exclude` — never the
  repo's own `.gitignore`, which is the operator's file. `{skills}` renders
  nothing there: the links *are* the realization
  (`Runtime.SkillsCwd`, `App.RenderAgentsSkills`). A `runtimes/<name>.yaml`
  declares this shape with `skills_cwd: true`; a profile naming both keys
  refuses, because a runtime has one skill surface and two half-bindings
  are not a binding (ADR 0012 D4). See **Skill surfaces** below for what
  else was tried.

An empty `skills:` renders nothing, placeholder and space alike.

**Declared means required.** `skills:` is a statement that the persona's
work depends on them, so it goes through the same parity gate as a wall
rule: a runtime with no surface adds `skills: <names> — <runtime> has no
per-session skill surface` to `Degraded` — today only a template-only
`runtimes/*.yaml` that names neither `skills_flag:` nor `skills_cwd:`, the
three built-ins all materialize — and the launch refuses unless
`--allow-degraded` (the session is then marked in meta and cockpit, like
any degraded launch). It is *not* filed under `Unrealized` — nothing is
being enforced here. A name that resolves to nothing refuses the launch
outright rather than binding a dangling symlink; `posse agent check` finds
it first, along with a PID whose own `command:` forgot `{skills}` while
`skills:` is non-empty (the `{model}` rule again: never leave a token
unrendered, never silently skip one) and a bound `SKILL.md` carrying no
`description:` — the third of the same kind, because codex drops such a
skill in silence and the persona launches believing it has one
(rangerhq-3zr; `App.SkillDescription`). Binding is additive — the
runtime's global skills still load; posse guarantees presence, not absence.
Isolation is a cage question, named out of scope by the ADR.

**Skill surfaces (rangerhq-1qd, verified 2026-08-18 on codex-cli 0.147.0
and grok 1.0.5).** The cwd shape is not a fallback — it is the only
per-session surface either CLI has, and it happens to be the same one, so
one realizer serves both. What was checked and is *not* there, so nobody
re-checks it:

- **codex has no config key for extra skill roots.** `skills` is a real
  config table (`-c skills=1` errors with "expected struct `SkillsConfig`",
  which is how you tell a recognized key from a silently-dropped one), but
  its only field is `bundled.path` — where the shipped system skills live,
  not an added root. The app-server's `skills/extraRootsSet` JSON-RPC
  method exists and is not reachable from the CLI. `codex debug
  prompt-input` renders the `<skills_instructions>` block the model
  actually sees, which makes every one of these a zero-cost check.
- **codex reads `<cwd>/.agents/skills/`** (and `<cwd>/.codex/skills/`;
  `.agents` is the vendor-neutral one), follows symlinks out of the repo,
  and keeps reading them under the full fleet line (`--disable hooks`,
  `allow_login_shell=false`, the trust table, `-s read-only`). It does
  **not** climb from a subdirectory to the repo root — the links have to go
  in the directory the session starts in.
- **grok's `[skills] paths` cannot be injected per session.** It is a
  `config.toml` key, and the one config layer a launcher can inject —
  `GROK_CONFIG` / `GROK_CONFIG_PATH` — is allowlisted to `models`,
  `features`, `toolset` and `shell_environment_policy`, dropping every
  other table by design ("cannot add a discovery source"). Verified: the
  overlay leaves `grok inspect`'s skill list unchanged.
- **grok's `--agent <definition>` has no skill-path field.** A definition
  carrying `skills:` / `skill_paths:` parses fine and binds nothing
  (checked against a headless session's `init` line, which advertises the
  session's skills). `grok inspect [--json]` is the zero-cost probe here.
- **codex silently skips a `SKILL.md` with no `description:`.** It never
  reaches the prompt and nothing says so — grok and claude fall back to the
  body's first paragraph, codex does not. A bound skill missing that one
  frontmatter line is bound to nothing on codex (rangerhq-1qd). `posse
  agent check` reports it (rangerhq-3zr), and the E2E arm below re-measures
  it against the installed CLIs.
- **grok reads `<cwd>/.agents/skills/`** as `project` scope, and its skill
  discovery deliberately ignores git's ignore rules — so `.git/info/exclude`
  hides the dir from `git status` without hiding it from the CLI. (Both
  CLIs behave the same way about this; it is what makes the exclude honest
  rather than a trick.) `internal/posse/skills_e2e_test.go` re-runs the whole
check against the installed CLIs (`RHQ_E2E=1 go test ./internal/posse/ -run
E2ESkillSurfaces`) — worth doing when either updates, since this is a
discovery convention, not a documented flag.

The cwd dir belongs to the **repo**, not to the persona, which is the one
asymmetry with claude's tree worth remembering: a launch adds its own links
and leaves other personas' alone (binding is additive — presence, not
absence), sweeps only its own links whose skill has left `RHQ_HOME/skills`,
and *refuses* rather than overwrite an entry posse did not write. A name
dropped from a PID therefore keeps its link in a repo where another
persona still binds it — harmless by ADR 0007 §4, and the price of a
shared directory.

Every persona session carries `RHQ_SKILLS_DIR` (`RHQ_HOME/skills`) and,
when the PID binds any, `RHQ_SKILLS` (newline-separated names): the exit
hatch, so a runtime posse cannot point can still be *told* where they are,
by the PID's body or by a wrapper. `posse skills` lists the names with the
PIDs that bind each, and flags names a PID declares that nothing answers.

**Authoring a skill** (`distributed-systems` is the pattern to copy —
rangerhq-gsy3). `SKILL.md` frontmatter is `name:` + `description:`, and
the description is mandatory — codex silently drops a skill without one
(rangerhq-1qd) — and is also the *entire* per-session cost (~110 tokens:
the always-loaded advertisement), so write it trigger-shaped. The body is
a short index (one line per concept, a "how to use in a bead" note), and
the content lives in `references/<concept>.md` files loaded one at a
time on demand — never paste a textbook into the front matter; the whole
corpus should only ever load a file at a time (~0.8–1k tokens each).
Honesty rules from the first skill: every claim carries a primary source
verified *live* on the authoring date (the model's recall is not a
source), labeled [paper]/[docs]/[blog]; where the shop diverges from the
field's answer, the file says so and cites the ADR. A literature spike
precedes the writing (rangerhq-dfz8).

**Codex specifics (codex-cli 0.147.0, verified end-to-end in rangerhq-5oi).**
Everything in codex's template above is a fix, not decoration:

- **The PID rides as `developer_instructions:`, not as the prompt arg.**
  ADR 0002 assumed the first user turn; that never worked — a PID file
  starts with `---`, which codex's arg parser reads as a flag, so
  `codex "$(cat <pid>.md)"` dies with `error: unexpected argument '---…'`.
  `-c developer_instructions=…` is a real config key here: `codex debug
  prompt-input` (free, no API turn) shows the text prepended to the base
  developer message — the closest thing codex has to
  `--append-system-prompt` — and it survives multi-line markdown with
  quotes, brackets, braces and `=`. It also leaves the first *user* turn
  free, so a dispatched codex session starts idle and the work prompt is
  its first turn, exactly as on claude. **Silent-failure warning:** codex
  ignores unknown `-c` keys without a word, so a future rename would
  launch personas with no PID at all. Probe before trusting a new codex:
  `codex debug prompt-input x -c developer_instructions=MARK | grep MARK`.
- **`--add-dir` rides with the sandbox mode, not with the template.**
  Under `-s read-only` codex *exits* on it ("the effective permissions do
  not allow additional writable roots"), so `realizeCodex` emits it only
  alongside `-s workspace-write` — where it is what makes the persona's
  memory dir writable (`touch $RHQ_PERSONA_DIR/x` → WRITABLE, verified).
  Read-only needs none: codex reads the whole disk regardless. **Every root
  is rendered resolved** (`codexWritableRoot`): codex refuses a writable root
  with a symlink *component*, and refuses it when a command runs rather than
  at launch, so a symlinked `personas/` dir made every dispatched codex
  session come up and then fail every tool call, silently — measured on
  codex-cli 0.150.1, `docs/notes.d/ranger-base-c02a.md`. **Every root but the
  one that cannot be resolved**: a DANGLING link (or a loop) is walked past
  and re-joined verbatim, so the rendered root keeps its symlink component —
  and the launch refuses on it rather than opening a session in which no
  command can run, because codex validates its roots before it applies the
  sandbox and one bad root refuses the whole set (`writableRootRefusal`,
  `docs/notes.d/ranger-base-k62e.md`).
- **`allow_login_shell=false` is a wall flag, not a nicety.** Codex
  otherwise runs each shell command through a login shell that re-sources
  the operator's rc files, and that re-prepends the login PATH *ahead* of
  the one codex inherited — so `PATH='<gates>/bin':"$PATH" codex …` still
  resolves `git` to `/usr/bin/git` and the L1 shim is **silently
  bypassed** (observed: `git push --dry-run` reached real git, no
  `refusals.log` line). `--disable shell_snapshot` does not help; the
  login shell is the cause. With the flag off, `command -v git` →
  `gates/<persona>/bin/git` and the refusal lands. Any runtime that
  re-sources rc files deserves the same audit before its shims are
  believed — **and every runtime now gets the gate shell besides** (ADR
  0009): the typed line points `SHELL`/`GROK_SHELL` at
  `gates/<persona>/shell/<basename>`, so the flag is the belt and the
  wrapper the braces. Codex never invokes it while
  `allow_login_shell=false` stands — verified on 0.147 with the wrapper
  live: the session still runs `/bin/bash -c` directly and `command -v
  git` still answers the shim (rangerhq-e43).
- **Two dialogs stand between an unattended codex and idle.** Directory
  trust ("Do you trust the contents of this directory?") fires per exact
  path — a trusted parent does *not* cover a repo underneath it — and
  `-a never` does not suppress it; the TOML inline-table `-c` form above
  does, scoped to `$PWD`, i.e. only the directory posse launched in (the
  dotted form `projects."/p".trust_level=…` is silently ignored). A new
  or changed `~/.codex/hooks.json` then opens "Hooks need review", which
  **stock herdr detection reads as `idle`** — a prompt sent there goes
  into the dialog, silently. `--disable hooks` removes the dialog for
  fleet panes, and posse keeps the flag regardless: not trusting hooks
  blind is the posture, because the cage is ours, not the runtime's
  plugins'. The detection gap itself is fixed by a local herdr manifest
  override, `etc/herdr/agent-detection/codex.toml` (`make
  install-detection`), which also covers every other codex modal footed
  `esc to go back` — `/model` read as idle too. That protects panes the
  fleet did not launch. Upstream's fallback for a known agent whose
  screen matches no rule is `idle`, i.e. it fails toward "safe to
  prompt"; the override is a fork of herdr's manifest, so
  `make verify-detection` warns when upstream moves past our fork point
  (rangerhq-7ia).
- **What directory trust actually loads, and the launch check for it
  (rangerhq-pmz → rangerhq-b7m, verified twice on 0.147.0 with a scratch
  repo and `codex mcp list`, no API turn).** Trust is not only about the
  dialog: a trusted session also reads `$PWD/.codex/config.toml` —
  codex's own words for it are "settings for a trusted repository,
  including sandbox, MCP, hooks, model, and reasoning defaults". The
  split that matters:
  - **Keys posse types on the line win.** `-s`, `-a never` and
    `developer_instructions` are ours whatever the file says, so the
    sandbox mode and the PID cannot be overridden by repo content.
  - **Keys posse does not type are the repo's.** `[mcp_servers.*]` (a probe
    server with `command = "/bin/sh"` appears in `codex mcp list` **only**
    under trust), `notify`, `model_provider(s)` — whose `env_key` can name
    *any* session env var as the bearer sent to its `base_url` — and
    `shell_environment_policy`. `mcp_servers` and `notify` are spawned by
    codex itself, **outside its per-command sandbox, with the whole
    session env, before any model turn**: a file in the repo gets exec on
    the box with no model and no PID in front of it.

  Kept as the fleet default anyway (the security persona's verdict on rangerhq-pmz):
  opt-in would mean codex dispatch never works, the grant is one exact
  path and is not persisted, the attacker must already be able to write
  the repo, and claude has the same class of channel in
  `.claude/settings.json` project hooks — trust is parity, not a new
  floor. What posse does instead is **check at launch**:
  `Runtime.ProjectConfig` names the file and `ProjectConfigKeys` optionally
  narrows it to top-level JSON keys (`parity.go`). Codex keeps the original
  whole-file predicate: any `.codex/config.toml` is a hit. Claude names
  **both** `.claude/settings.json` and `.claude/settings.local.json` — one
  scope, two files, checked in that order (rangerhq-9u8) — with `hooks` and
  `mcpServers`: either top-level key in either file is a hit regardless of
  value, while a readable object carrying only `permissions` is clean. An existing keyed file that is unreadable,
  malformed, or not an object fails closed because the launch cannot prove
  the channel absent. A hit is a `Degraded` entry, so `posse new`/`posse
  dispatch`/the cockpit **refuse** and name the file plus the matched keys or
  classification failure unless the operator passes `--allow-degraded`
  (session then marked, as any degraded launch) or the PID carries
  `trust_project_config: true`. It is `Degraded` and never `Unrealized`:
  nothing here is an unenforced gate, it is what the launch gives away.
  `posse gates <persona>` computes the matrix for the cwd, so the line shows
  there too. `posse relaunch` preflights the same launch plan before replacing
  a session, so a repo that grows one of these surfaces after a clean launch
  refuses on relaunch.
- **Env: codex strips nothing by default.** ADR 0002 assumed
  `shell_environment_policy` drops `*KEY*`/`*TOKEN*`/`*SECRET*` names, so
  the template might need `ignore_default_excludes=true`. It does not: on
  0.147.0 the model's shell inherits the session env whole — `BD_ACTOR`
  and `RHQ_*` arrive intact (`bd` inside codex commented and closed a
  bead under the dispatched persona's own actor name), and so does every
  key the operator exports. No flag
  is needed and no naming rule applies; the honest statement is the one
  in the security posture below — env sets are inherited, not contained,
  on every runtime alike.

**Grok specifics (grok 1.0.x, verified end-to-end in rangerhq-vjl —
headless probes on 1.0.0, live fleet sessions on 1.0.5 after the CLI
self-updated mid-verification; no behaviour below differed).**

- **`--rules=`, never `--rules `.** A PID file starts with `---`, which
  grok's arg parser reads as a flag in the separated form: `grok --rules
  "$(cat <pid>.md)"` dies with `Usage: grok [OPTIONS] [PROMPT]` and the
  pane falls back to a shell prompt. The `=` form binds the value and the
  PID lands in the system prompt (`--rules` appends to it; asked its name,
  the session answers with the persona's). This is codex's `---` trap in a
  second parser — assume the next runtime has it too, and probe for it.
- **`--permission-mode auto` is what unattended means here.** Of the six
  modes, only `auto` and `bypassPermissions` approve a tool call with
  nobody watching; under `default`, `acceptEdits` and `dontAsk` even an
  *undenied* command sits unapproved forever. Both working modes still
  honour `--deny` — a denied command is refused under `bypassPermissions`
  too — so the fleet types `auto`, the lower-privilege of the two. The
  flag also beats the operator's `~/.grok/config.toml` `[ui]
  permission_mode`, which on this machine is `always-approve`: the launch
  is the same whatever the operator left in their config.
- **`--allow`/`--deny` are claude's rule *spellings*, on a different
  matcher.** `Bash(git push:*)` refuses `git push --dry-run` and leaves
  `git status` alone; bare tool names (`Edit`, `Write`) match every
  invocation; deny beats allow. The match is not anchored at argv[0]
  either: `env git push --dry-run origin HEAD` was refused too (`Denied by
  permission policy: deny rule on bash matching "git push"`). Still L0:
  `/usr/bin/git push` walked straight past the rule in a live session (the
  pre-push hook is what stopped it), and a denied verb reached from a
  Makefile recipe or a subprocess never enters the string grok matches —
  which is L1's job.
- **The dialect, probed rule by rule (grok 1.0.5, rangerhq-625).** `*` IS
  a wildcard mid-string, not a literal: with `Bash(git -* push)` +
  `Bash(git -* push *)` typed (the pair as it stood then; neither half is
  emitted any more — rangerhq-ky3, rangerhq-vr6j), all ten option
  spellings of a push —
  `-C`, `-c`, `--git-dir=`/`--work-tree=` and their separated forms, `-p`,
  `--no-pager`, `--literal-pathspecs`, a stacked `--namespace x -C <r>
  --no-optional-locks` — were refused, and nothing reached the throwaway
  bare repo. `?` and `[…]` work too. Three divergences from claude, each
  verified rather than read off grok's own shipped docs
  (`~/.grok/docs/user-guide/22-permissions-and-safety.md` describes the
  no-word-boundary prefix and the `:*` strip below, but on the shell-parse
  one — the expensive half — says the opposite, "nothing else is
  normalized" beyond leading whitespace; a matcher detail grok has not
  committed to its own doc, so it can move under us):
  - **A segment reaches the matcher shell-parsed** — quotes off, runs of
    whitespace collapsed to one. So `git -C <r>  push …` (two spaces)
    matches a rule written with one, and, the expensive half, a *quoted
    argument* carrying the word `push` matches as if it were the
    subcommand: `git -C <r> log --author "push me"` and `git -c
    user.name=t commit -m "push it upstream"` are both refused under the
    pair. Claude matches the raw string and runs them.
  - **`:*` is a prefix with no word boundary.** `Bash(git push:*)` refuses
    `git pushy --help`. Claude requires the boundary. That is the PID's
    own rule being read more broadly than we write it, not something the
    fleet spells around.
  - **A rule with no wildcard is a prefix, not exact.** `Bash(sha1sum)`
    refuses `sha1sum --version` unaided — the miss `L0Spellings` adds
    `Bash(<cmd>:*)` for on claude does not exist here.
  This is why **`realizeGrok` does not call `L0Spellings`**: the widening
  is not a no-op there, it is a false-positive generator, and at L0 a
  false positive is a hard block the model cannot ask its way past (the
  ground rangerhq-3mc rejected a single `Bash(git -* push*)` on). The true
  positive is not lost — L1 holds on grok (next bullet) and the shim
  refuses every one of those spellings. Two of the false positives were
  *not* grok's: an unquoted trailing `push` word (`git -C <r> log --grep
  push`, rangerhq-ky3) and a ` push ` token that is not the subcommand
  (`git -C <r> stash push -m wip`, `git -C push status -s`,
  rangerhq-vr6j) were refused by the pair on claude too, which is why
  claude emits neither half any more. The quoted-`push` divergence in the
  bullet above is measured against that retired rule text, and it is what
  keeps a future widening from being shared; what keeps grok unwidened
  *today* is the bullet below it — a rule with no wildcard is a prefix
  here, so the negative rule's lone claude spelling, `Bash(git commit)`,
  would refuse the qualified `git commit -- <path>` the PID allows.
- **`git push` is on grok's own dangerous list.** Under `--permission-mode
  auto` it is cancelled outright with nobody there to approve it
  (`User cancelled the execution for tool run_terminal_command`), rule or
  no rule — so the fleet's launch mode already refuses it a second way,
  and a probe that needs a push to actually *run* has to be driven under
  `bypassPermissions` to isolate what the rules did.
- **L1 holds on grok — because the shell is ours now (ADR 0009,
  rangerhq-e43).** Grok runs every shell command through a *login* shell:
  it captures the login shell's state (rc-exported vars, aliases,
  functions) and replays it into each command. On macOS that hands PATH to
  `path_helper`, which re-orders it so `/usr/bin` precedes anything the
  launcher prepended — in a live developer-persona grok session `command -v git`
  answered `/usr/bin/git`, not the gates shim (rangerhq-vjl). This is
  codex's login-shell trap, but codex had `allow_login_shell=false` and
  grok has no equivalent knob: `GROK_LOGIN_ENV`, `GROK_SHELL` and a
  `[toolset] login_shell_capture = false` overlay via `GROK_CONFIG_PATH`
  were all read and ignored. So the fleet supplies the shell instead —
  `SHELL`/`GROK_SHELL` on the typed line point at the rendered **gate
  shell** (see *Gates L1* below), which re-prepends the gates dir inside
  the `-c` string and again in grok's user-command slot, after the replay.
  Verified live on grok 1.0.5, same box: `command -v git` →
  `gates/<persona>/bin/git`; a `git push --dry-run` reached from a Makefile
  recipe → `refused by posse gate: git push --dry-run origin HEAD` + a
  `refusals.log` line; `/usr/bin/git push origin HEAD` → the L3 pre-push
  refusal, nothing pushed; `rm -rf …` on a PID that denies it → refused,
  so a **non-`git push` shell deny is clean on grok, not degraded**.
  `Runtime.LoginShellPATH` and its parity special case are gone.
- **Auto mode will not run an unknown local script.** `sh probe.sh` and
  `sh check.sh` — the second one sitting in the session's own cwd — both
  came back "blocked by auto mode: … treated as an untrusted/unknown local
  script", nothing executed. A live gate probe therefore has to reach the
  shell through a *known binary* (`make <target>` works), not through a
  script the model is asked to run. Grok's own shell-command policy is
  narrower than claude's on this point, and it is why the shim's own
  refusal is easiest to demonstrate from a Makefile recipe.
- **No `--add-dir` equivalent, and none needed.** Grok's sandbox is off
  unless `--sandbox <profile>` is passed, so the persona reads and writes
  the whole disk; the memory dir rides on `RHQ_PERSONA_DIR` alone and the
  template has no `{memory}`. Under `cage: seatbelt` the memory dir is
  writable because *our* profile allows it — verified in a live
  security-persona grok seatbelt session: `touch $HOME/x` → `Operation not
  permitted`, `touch $RHQ_PERSONA_DIR/x` → `rc=0`. Grok's own
  `--sandbox` profiles (`workspace`/`read-only`/`strict`, custom ones in
  `~/.grok/sandbox.toml`) are real Seatbelt, and grok refuses to start if
  a named profile cannot be applied — but they are runtime-specific by
  definition, so ADR 0002 keeps them as L0 inside our L2, not as the tier.
- **The startup splash stands between an unattended grok and idle
  (rangerhq-37c).** No trust dialog: grok's folder trust gates *hooks*,
  not startup, so an untrusted session dir simply runs without the repo's
  hooks — the posture we want. Cross-session memory is off unless
  `--experimental-memory` is given, so the persona's memory stays the
  fleet's ORDERS.md. grok's `SessionStart` hook
  (`~/.grok/hooks/herdr.json`, installed by herdr) reports the session id.
  But a fresh `grok` pane opens on a **startup screen** — the New worktree
  / Resume session / Quit menu, a "<version> is here!" changelog line and
  the "Help improve Grok" consent banner — and that screen holds the
  keyboard before the composer does. Text sent to it is *buffered*, not
  delivered, and the submitting Enter is eaten by the splash: the work
  prompt never runs, it sits unsent in the composer. Stock herdr detection
  matched **no rule** there and fell through to
  `default_known_agent_idle_fallback` = `idle`, so `dispatch`'s
  wait-for-settled gate was satisfied by a screen that could not take the
  prompt. `etc/herdr/agent-detection/grok.toml` reports it `blocked`
  instead; the launch then fails loudly ("never settled idle") rather than
  silently losing the bead's prompt. This was foreseen right here — the
  note used to end "it still reads correctly, but that is luck, not a
  contract", and the luck ran out at grok 1.0.5. The earlier
  `posse dispatch --runtime grok` run that reached idle → prompted → done
  (and closed its bead under the dispatched persona's actor name, so
  `BD_ACTOR` does reach `bd` through
  grok's shell) was on a pane whose splash had already been consumed.
  Note also that the override only makes the failure *loud*: the splash
  never self-dismisses, so grok is not dispatchable again until the
  launcher dismisses it or the fleet moves to a startup mode that has no
  splash (`grok --minimal` has none and takes input immediately) —
  rangerhq-7sbo.
- **…and re-measured in rangerhq-7sbo, where the "holds the keyboard" half
  did not reproduce.** On live 1.0.5 panes today, launched both plain and
  with the fleet's flags: text sent to an untouched splash appears in the
  composer *immediately* (with the `Enter:send │ …` hint footer), one
  `agent send-keys <pane> enter` after it submits the turn (idle →
  `osc_progress_working` → done, menu still drawn), and **Esc undraws
  nothing** — the menu, changelog and banner stay, so detection keeps
  reporting `startup_splash`/`blocked` and `agent wait --until idle`
  simply times out. The splash is decoration over a live composer. Which
  means (a) the "send Esc and wait for idle" fix does not work, and ~~the
  launcher instead presses Esc once and prompts past *the same* screen if
  it is still reported (`clearStartupScreen` in dispatch.go)~~ **the
  launcher presses nothing at any screen — the special case was retired in
  rangerhq-6723: `clearStartupScreen`, `startupScreenDismissals` and
  `startupScreen` are gone, and `awaitAgent` waits for a settled state and
  refuses anything that is not `idle`/`done`
  (`docs/notes.d/rangerhq-6723.md`)**, and (b) the
  first-run state 37c measured is either already consumed on this machine
  or was the boot race all along — a prompt typed before grok's TUI took
  input, which fits the coordinator's incident too. Whether `startup_splash` should
  report `blocked` at all is rangerhq-1xsj (ops). Two lessons, both
  the same one: a screen that *looks* like it holds the keyboard is not
  evidence that it does — press a key and read the pane; and a detection
  rule's anchor must not include an optional element (37c's required the
  `[stable]` channel tag, which today's panes do not render, so the rule
  matched nothing at all until 7sbo widened it).

**Gates L1 (ADR 0002 §3, `internal/posse/gates.go`).** Native flags are
politeness; the wall for shell-verb denies is ours and runtime-agnostic.
At every persona launch (create and crash-relaunch alike) the PID's
`deny:` rules of the shape `Bash(<cmd> <prefix>:*)`, `Bash(<cmd> <args>)`
(exact) and `Bash(<cmd>)` (whole verb) are rendered into
`RHQ_HOME/state/gates/<persona>/bin/<cmd>`: a POSIX sh shim that refuses
when argv matches — `refused by posse gate: git push --force (deny:
Bash(git push:*))`, exit 1, a line in `gates/<persona>/refusals.log` —
and otherwise `exec`s the real binary resolved at render time from PATH
minus any gates dir. Rendered fresh each launch (a rule dropped from the
PID stops being enforced; the log survives). The shim dir is prepended
**on the typed command line** — `PATH='<bin>':"$PATH" SHELL='<gate
shell>' GROK_SHELL='<gate shell>' <rendered command>` — not in the
workspace env, because macOS `path_helper` reorders PATH when the pane
shell starts. `RHQ_GATES_DIR` names the dir in the env. `posse gates <persona>` shows the shims and the log tail.
Known holes — `/usr/bin/git`, `command -p`, **git aliases** — are why L3
exists for the one verb that is a hard risk line; the parity check
(rangerhq-1po) says which denies the wall realizes.
`Edit`/`Write`/`WebFetch`/`mcp__*` denies are not shims.

- **Git aliases dodge L1 entirely** (rangerhq-3mc). `git p` where the
  operator's gitconfig says `alias.p = push` reaches the shim as the token
  `p`; matching it would mean running `git config --get alias.<token>` per
  invocation, in POSIX sh, against whatever repo the global options point
  at — a fork and a config read on every gated command, and still wrong
  for an alias defined in the *repo's* config. So it stays a documented
  hole: L3's pre-push hook catches it in hooked repos (the alias expands
  to a real push, which runs the hook), nothing catches it elsewhere, and
  the tier that closes it properly is `seatbelt`/`container`, where the
  gate is not a name lookup.

**The gate shell (ADR 0009, `renderGateShell`, verified live in
rangerhq-e43).** A PATH prefix on the typed line survives only while the
runtime uses the shell it inherited. Grok 1.0.5 does not: it re-execs the
operator's *login* shell, so `/etc/zprofile` runs `path_helper` and the
gates dir is demoted below `/usr/bin` before any command runs. So the
shell is ours as well: every launch renders
`RHQ_HOME/state/gates/<persona>/shell/<basename>`, a POSIX sh wrapper that
walks argv the way a shell does, prepends a PATH guard to the command
string after a `-c` and — behind a `--` argv0 — to the runtime's
user-command slot (which runs *after* the snapshot replay), then `exec`s
the real shell: `$SHELL` when its basename is `bash` or `zsh`, else
`/bin/zsh`. The wrapper is *named* with that basename, because grok picks
its snapshot dialect from the name. The typed line points `SHELL=` and
`GROK_SHELL=` at it on **every** runtime, so a runtime that starts
snapshotting after a self-update inherits the fix instead of a silent
regression; `Runtime.NoGateShell` (`gate_shell: false` in
`runtimes/<name>.yaml`) is the exit hatch and drops parity back to
unrealized for `Bash(…)` denies on that runtime. A mis-parse breaks the
persona's shell loudly; it never falls back to a silent bypass.

- **The guard tests precedence, not presence.** `case "$PATH:" in
  "<bin>":*) ;; *) PATH="<bin>:$PATH";; esac; export PATH;` — the obvious
  "idempotent" spelling (is the dir *anywhere* on PATH) is a no-op exactly
  when it is needed, because the typed line already put the gates dir on
  PATH and `path_helper` **re-orders** it rather than dropping it. With
  the presence test the wrapper rendered, ran on the right argv, and still
  left `command -v git` answering `/usr/bin/git` in a live grok session.
  Cost of the precedence test: PATH can carry the dir twice; lookup takes
  the first. The same trap waits for any "already set?" guard that runs
  after a reorder.
- **`gates/<persona>/shell.log`** gets a line only when the *user-command
  slot* guard had to re-prepend — the replayed snapshot had lost the gates
  dir entirely. A normal session leaves no file at all (verified across
  claude, codex and grok sessions); a line means a runtime's snapshot
  shape moved and is worth reading.
- **Consequence: the persona's own rc files now run gated.** The guard is
  prepended ahead of the login capture's `source ~/.zshrc`, so a denied
  verb *inside* an rc file is refused like any other — observed on a probe
  PID denying `rm -rf`: oh-my-zsh's `rm -rf ~/.oh-my-zsh/log/update.lock`
  landed in `refusals.log`. Honest, and worth knowing before reading a
  refusals log as "the model tried something".

Shell shapes verified 2026-08-18 by installing an argv logger as `$SHELL`
(repeat after any runtime self-update — ADR 0009 verification item 7):

| runtime | how it invokes the gate shell |
|---|---|
| grok 1.0.5 | login capture, twice: `-lc 'source "$HOME/.zshrc"; printf …"$PATH"; command env -0'` and `-lc '… builtin alias -L; builtin typeset -f'`. Per command: `-c '<replay: snap=$(command cat <&3); builtin eval "$snap"; …; __grok_user_cmd="$1"; builtin set --; builtin eval "$__grok_user_cmd" 2>&1>' -- '<user cmd>'` (bash dialect: `-O extglob -c … -- …`). The replay also shadows `find`→`bfs` and `grep`→`ugrep` as functions. |
| claude | `-c -l '<snapshot; eval cmd>'`; its snapshot restores the process PATH, so the typed prefix already won — the wrapper changes nothing (verified: `command -v git` → the shim). |
| codex 0.147 | never — `allow_login_shell=false` runs `/bin/bash -c` directly (verified: the shim still wins). |

**Gates L3 (rangerhq-8s4).** `.git/hooks/pre-push` (`# posse-gate`
marker) refuses when `RHQ_TOOLS_DENY` — newline-separated, exported into
every persona session by CreateSession — carries a rule matching git
push (`Bash(git push:*)`, `Bash(git push --force:*)`, `Bash(git:*)`,
`Bash(git)`), with the same message shape and a `[pre-push hook]` line in
`refusals.log`; without the variable (an interactive operator) it passes.
Catches `/usr/bin/git push` and pushes from subprocesses that kept the
env — cooperative class at every tier (ADR 0025 §1): it cannot see
through `env -i` (nothing in-process can), `--no-verify` skips it
outright, and `-c core.hooksPath=` redirects past it with zero writes
(measured, ranger-base-3csb). At `cage: container` the push's *effect*
can still die at an enforced layer (mount `:ro` / egress proxy) where the
launch is configured for it (ADR 0025 §3); the verb gate itself stays
cooperative. Install: `posse gates install-hooks [repo]`
(replaces its own hook, refuses to overwrite a foreign one — chain by
hand). Persona launch reconciles the pre-push slot when the PID denies git
push and always reconciles `prepare-commit-msg`, then decides each slot on
**byte identity** against its own current render — that render, or the
prescribed chain dispatcher with `posse-<slot>` byte-equal to it (ADR 0023).
The file at the dispatch path is never exec'd; the behavioral half runs
posse's own render from a private temp file, so it catches a renderer
regression and nothing about what is planted. A foreign chain therefore does
not count however well it behaves, and a planted pass-through body does not
count even if it kept a marker: markers gate replacement, never realization.
Failure is `DEGRADED`, visible before herdr is touched, and L3 disappears
from concrete parity. The probe is launch-time
evidence, not a permanent lock: at `cage: shims` the session can still edit
the slot after the probe (the TOCTOU residual); the L2/L4 hook carve-out is
what removes that capability. Chain foreign slots per INSTALL.md §9.
`posse init` does not touch repos. And because a launch reconciles only the
repo its worktree was cut from, a hooked repo that never holds a session is
re-rendered by nothing — `SweepHookWall` asks the same identity question of
every `beads_visibility:` key from `posse promote`'s epilogue and the
`dispatch --watch` preamble, and names the ones to re-render by hand
(ranger-base-ixv4).

**Gates L3 on a managed hooks path (ADR 0052).** An employer-managed
`core.hooksPath` — absolute, outside every repo, and unwritable by this uid,
all three measured before any write — is not foreign, it is a wall posse does
not touch: `install-hooks`, session create and the hook-wall sweep write
nothing there and print one line saying so. L3 is realized instead by a
per-session hooks dir, `state/hooks/<session>/`, that the session env aims
git at through `GIT_CONFIG_COUNT`/`_KEY_n`/`_VALUE_n` naming `core.hooksPath`:
posse's members plus one dispatcher for every executable in the managed dir
(the union, because a slot the redirect dir lacks is an employer hook git
skips), ours first with its exit final, then the employer's with git's own
argv and stdin. The identity probe moves to that dir and gains a
forward-completeness arm; parity says so (`session hooks dir, redirected by
env; managed hooks <dir> run after ours`); meta records `hooks_mode:
redirect`. Env-borne, so the same class as the rest of L3: survives an
absolute-path git, shed by `env -i`, which leaves the employer's hooks
running alone — nothing a persona or posse does weakens the employer's
control. The staleness sweep and `scripts/verify-hook-freshness.sh` classify
each configured repo before reading it and skip a managed one by name: the
box is CLEAN, not unmeasured, because nothing of posse's is installed there
to go stale. The script's reference render — a real `install-hooks` into a
throwaway repo — is taken with that same config-in-env redirect aimed at a
scratch dir of its own, and refuses to measure unless git confirms the
redirect took; without it the throwaway repo inherits the managed global and
the whole control goes dark (ranger-base-1se2l). `posse gates managed-hooks
[dir]` is the read-only form of the classification, exit 0 managed / 1 not.
Recipe and the two residuals: INSTALL.md §9, "A managed hooks path".

**Tiers (ADR 0003 §1–2).** A tier is a name — `strong` / `standard` /
`fast` — mapped to a model per runtime in the built-in table: claude
`claude-fable-5-1` / `claude-opus-5` / `claude-sonnet-5`; codex
`gpt-5.6-sol` / `gpt-5.6-sol` / `gpt-5.6-luna`; grok `grok-4.6` /
`grok-4.6` / `grok-4.5` (`fast` falls back to `standard` when only that is
mapped). Codex
maps `strong` and `standard` to the same id on purpose: sol is what a
codex session here defaults to and codex offers nothing above it, so
naming it makes the launch a fact rather than a CLI default that can move
between releases, while `fast` = luna is the **cost** lever only —
MEASURED 2026-08-25, switching to luna did not lift an account-level usage
wall, because the wall is on the account and not on the model
(ranger-base-arm). Until that map existed `tier:` was inert on codex: no
`Models` at all, `{model}` empty, no warning. **grok had the same defect
until rangerhq-jp6** — its template already carried `{model}` and
`ModelFlag: -m %s`, so only the map was missing — and it is filled the
same shape and for the same reason: grok-4.6 is what a grok session here
defaults to (`grok models`, 1.0.5, 2026-08-29: "Default model: grok-4.6")
and grok offers nothing above it, so `strong` and `standard` both name it.
`fast` = grok-4.5 is a **capability** step-down and, unlike codex's, NOT a
measured cost one: xAI publishes no per-model rate against the weekly pool
(the reason grokpool.go estimates the meter at all), and grok-4.5 has
never run on this box — 181 of 181 priced turns across 174 transcripts in
`~/.grok/sessions` carry `"modelId":"grok-4.6"` — so "4.5 is cheaper" is
not a small number, it is **no number**, and nothing may read a saving
into that row. `fast` is named explicitly rather than left to the
fallback for the reason the fallback would defeat: a `fast` rendering the
same id as `standard` leaves dispatch's budget step-down buying nothing.
`--reasoning-effort` (alias `--effort`) is arguably the bigger spend dial
on grok and is deliberately NOT in the map — **ruled out**, not deferred
(ranger-base-tg7c, ADR 0003 §1 amendment 2026-08-29): nothing here can
price an effort step against the weekly pool, and the two models do not
offer the same efforts — grok-4.6 has xhigh/high/medium/low, grok-4.5 only
high/medium/low, both defaulting to `high` (measured). So a per-tier
effort is not one key but a tier→model × model→efforts validity matrix
plus a second placeholder, and an unrendered placeholder is a literal
argv. A PID or declared runtime that wants one appends
`--reasoning-effort` to its own `command:` today. The same ruling ratified
the SHAPE of the column over the map rangerhq-jp6 originally asked for
(`standard` = grok-4.5).
A runtime that maps nothing
now says so where it is read — `posse runtimes` prints `tiers: UNMAPPED —
ignores tier:` and `posse runtime check <name>` names the tiers that
render nothing, both off one rendering (`Runtime.TierMap`). No built-in
reads `UNMAPPED` any more, so that rendering is now about a declared
`runtimes/<name>.yaml` with no `model_<tier>:`, a partial map, or a
runtime name posse has never heard of — and its pins are fixtured on a
declared runtime for exactly that reason. A
`runtimes/*.yaml` still cannot override a **built-in**: `LoadRuntime`
returns the built-in as soon as the name matches, before it stats
`RHQ_HOME/runtimes/<name>.yaml`, so `model_<tier>:` / `model_flag:` reach
template-only runtimes only (undecided, ranger-base-arm). Built-in
templates carry `{model}` → `--model <id>` (claude),
`-c model=<id>` (codex), `-m <id>` (grok), rendered to nothing when
unmapped; a `runtimes/<name>.yaml` may set `model_<tier>:` and
`model_flag:`. A PID that keeps its own `command:` gets no model unless
it adds `{model}` — `posse agent check` warns. Precedence at a launch site:
`--tier` (new/dispatch) > PID `tier:` > config `default_tier:` >
`strong`; bead-label and `tier_by_label` resolution is dispatch's
(rangerhq-6eb), and `fast` is gated on full enforcement parity with
`tier_floor:` refusing anything cheaper (rangerhq-2uq — see the security
posture below). Dial D ("floor `standard` fleet-wide, `fast` only on an
explicit signal *and* full parity") needs no config key of its own:
nothing resolves to `fast` unless a bead label or `tier_by_label` says
so, and the parity rule is what makes it honest when one does. Sessions carry `RHQ_TIER` in the env and `tier:` in meta; listings
show `🎭name@runtime/tier` when either differs from claude/strong. Dial
A in `examples/agents`: architect/security/product `strong`, the rest
`standard`.
The claude ids above are a restatement of `claudeModels`, and the
fallback line quoted below is a quotation of what the preflight prints:
`internal/posse/notestier_qa_test.go` holds both against the code, in
BOTH directions, after three days in which this paragraph named a
superseded strong id that ADR 0003 had already flipped. ADR 0003 §1 no
longer names an id at all — "the current built-in model ids and exact
price rows live in runtime.go/cost.go" — so this paragraph is now the
ONLY prose in the tree restating that table, and a pin is the only thing
that can keep it true (ranger-base-1kvfr).

**Tier availability preflight (rangerhq-oay).** A tier is a name and the
launch turns it into a model id — but until this landed, nothing asked
whether the account can run that id. It came from a real morning: access
to the strongest model disappeared from the operator's own session, and a
persona resolving `tier: strong` would have gone on launching while the
CLI quietly served something else, with `posse cost` filing the spend
under whatever tier the substitute belongs to and no line anywhere saying
why. So `planLaunch` now checks, once per launch, on the pair it has just
resolved: `App.TierPreflightFrom` (modelavail.go — `TierPreflight` is the
same check for a caller that names no env sets), before the parity check.
Unavailable prints one line — `richard: tier strong wants claude-fable-5-1
— unavailable on this account; launching as asked, and only an explicit
--runtime/--tier/--model or a PID change moves it` — and that line is the
whole of what it does. `posse cost` needed no change and that is the point:
`TierForModel` reads the model out of the transcript, so the spend was
always counted honestly — what was missing was anyone knowing.

**What it used to do, and no longer does (ADR 0003 §3, ranger-base-hv2zr).**
Until 2026-09-06 an unavailable model was SUBSTITUTED: the launch walked
config `tier_fallback:` (default `strong` → `standard`), opened on whatever
it landed on, wrote a `fallback:` mark into the session meta, and `posse
list`, the cockpit, the relaunch receipt and dispatch all read that mark
back. Dial H is struck: availability is advisory, the runtime may refuse an
unavailable choice on its own, and choosing another pair is the operator's.
The measurement behind the removal is on the bead — over 2026-08-25 →
2026-09-06 (509 catalog reads, 137 of them successful) the mechanism
performed no substitution at all, and the 336 lines mentioning 401 are
UNKNOWN reads, which by construction never substituted anything. What
replaced the mark is a comparison: dispatch's `effectiveTier` reads the
meta's own `runtime:`/`tier:` against the pair the bead resolved, so a
session that opened on another pair — an operator's explicit `--tier`, or a
session created before the removal — still gets a work prompt naming what
it really runs.

The probe is `GET api.anthropic.com/v1/models`, zero tokens, shared through
`$RHQ_HOME/state/model-catalog.json` behind `model_probe_ttl:` (default
1h) exactly as `plan-usage.json` is — a successful reading is reused for
the TTL, and rate-limit cooldowns are shared across processes. Other failed
attempts remain UNKNOWN and may be retried by the next launch.

WHICH CREDENTIAL it presents changed twice on 2026-09-05 and the sentence
above used to answer "the same one the plan guard reads". It now PREFERS the
session mint of the env sets the launch being judged realizes — read out of
`envs/*.env` under the home by the ADR 0019 seam, selected by that launch's
own set list and taking the last assignment of the name across it (ADR 0039
D3d as amended, ranger-base-q3n4e) — because the meter credential rots in
hours and had left this probe failing since 2026-08-31. The plan guard's
credential is the FALLBACK, for the two ways the preference does not answer:
none of the named sets carries the variable (no request is spent), or the
endpoint refuses the one that did (exactly one more read per catalog read,
never a loop). A read no launch asked for — `posse runtimes`, `posse gates` —
uses the persona-less list, which is config `default_env` and nothing else; a
persona that names no env set gets no session credential rather than the
cockpit's, because an env set is an explicit choice and never a silent
default (rangerhq-f2b). Verified live
2026-08-23: the OAuth credential is
accepted there (the route exists and answers 401 unauthenticated;
`/api/oauth/models`, the shape the plan guard's endpoint would suggest, is
a 404), and the catalog it returns is the ten ids this account can use,
including all three the claude tier table names. A later installed-binary
launch on the same account produced no snapshot at all; before the request
log below, that launch left no evidence distinguishing credential context,
HTTP response, transport failure, or an empty answer. **Answered
2026-08-26 (ranger-base-r64): that launch was from a gated pane** — posse's
own `security` read resolved to the persona's `Bash(security:*)` shim and
was refused. The read now returns that as its own error instead of
"keychain item unreadable", and the preflight says so once per process on
stderr rather than only in the log. Since ranger-base-ypf5 it is not refused
at all: the read execs `/usr/bin/security` absolutely, so a launch from a
gated pane probes normally.

Every cache miss that attempts a probe appends a generic outcome to
`$RHQ_HOME/state/model-catalog.log` (`ok models=N`, HTTP failure, empty
catalog, or cooldown); cache hits append nothing, and the log is bounded on
the same policy as `plan-usage.log`. It never records the credential or a
header. This is the evidence for UNKNOWN: a launch still fails open, but a
missing `model-catalog.json` no longer leaves "model available" and "probe
could not authenticate" observationally identical.

Three rules make it safe to leave on by default. **It fails open in one
direction only**: a catalog that was actually read and does not contain
the model is the ONLY thing that reaches a verdict at all — an unreadable
credential, an unreachable endpoint, a 429, an empty answer or a runtime
with no model mapping are all *unknown*, and unknown launches exactly what
it was asked to launch without a launch warning (the request outcome remains
in `model-catalog.log`). A preflight that guessed "unavailable"
would once have silently downgraded the whole shop; since dial H went it
would only shout, which is milder and still wrong. **It never refuses**:
rule (3) from the operator — "a degraded model is worse than nothing" is
their judgement, and the place they record it in advance is `tier_floor:`,
which still bites, now on the asked-for pair, because that is the only pair
a launch can open on. **And it never chooses**: `tier_fallback:` is gone
(ADR 0003 §3) and a config that still carries the key is inert — no walk,
no default `strong` → `standard`, no hops, no marks. `model_preflight:
false` turns the whole check off; `posse gates <persona>` prints the verdict
per runtime, which is how you tell "the strong model is gone" from "the
probe never answers on this box" without launching anything.

Two consequences worth knowing. A session's meta records the pair it
*actually* launched at, the way `cage:` records the cage it got — so
`posse relaunch` and `RelaunchAgent` replay THAT pair rather than
re-resolving it, which is what keeps a session the operator opened at
another tier where they put it. And the test seam is `App.ModelLister` (nil =
`NewModelLister`), the twin of `Dispatcher.Plan`: `newTestBackend` hands
every test an unconfigured one, which reads no credential and reaches no
network and is therefore also the fail-open path. That seam is the *only*
one: the catalog URL is compiled in, with no env override at all, because
the probe sends the account's OAuth token and a second way to point that
token is the same argument as a second way to hand it one (credpin.go).
Tests that want the preflight to *do* something seed
`state/model-catalog.json` — a reading off a seeded snapshot with an
unconfigured lister proves it never asked anyone.

Catalog membership and plan allotment are different facts. On 2026-08-24 a
strong-tier Claude session returned a synthetic assistant message saying
its Fable allotment was exhausted, then settled `idle` without doing work;
the model can remain in `/v1/models` throughout that condition. Dispatch now
checks the matching Claude transcript after a turn settles. That exact
provider refusal writes `turn_failure:` into the session meta, prints a loud
⛔ line, marks `posse list` with `🛑turn-failed`, and renders the
cockpit row red as `failed` instead of healthy `idle`. That line says "no
work ran" only where the runtime's own record says so — on a refusal that
landed mid-turn it names what had already run and sends the operator to the
worktree first (`ranger-base-qcu4c`, §"The other half of that line"). It does not guess a
fallback or replay the prompt: changing tiers remains an operator decision,
and the claimed bead stays attached to the failed session until that
decision is made. A later dispatched turn whose first assistant answer is
healthy clears the marker. The matching keys on the transcript's assistant
record after `Work beads issue <id>`, not on pane text, because bead data
may quote the provider message verbatim.
`posse scorecard [<persona>]` makes the PID metrics observable from bd
data, read-only (rangerhq-h2c): per persona, closed / reopened / open /
held (in_progress) / blocked, median age at close, and beads filed
(`created_by` = the persona's BD_ACTOR) vs rejected (close reason
reading invalid/duplicate/wontfix); then each PID's `metrics:` ids in
words. bd's snapshot has no status history, so *reopened* is read from
the git history of `.beads/issues.jsonl` (closed→open between two sync
commits) and shown as `?` when there is none. Honest gaps, printed as
such: `blocked-honestly` is a dispatch-side outcome, and
`designs-implemented-unchanged` / `spec-clarity` need a comment scan —
neither is computed yet.

The scorecard also prints the **harness-upkeep ratio** (rangerhq-ndi):
DIRECTION.md's caution that Gas Town died of harness self-refinement budgets
~20-25% of all work at harness upkeep, and this makes the number a fact
rather than a feeling — closed beads, harness vs everything else, over 7d
and 30d, per persona and total. **What counts as "harness"**: a bead whose
own id carries the same prefix as this bead's own id above (rangerhq-ndi)
— the harness's bd project, unchanged since this repo's prior name, so
issues filed against posse's own code and process still carry that prefix
regardless of the directory's current name. Everything else counts as
product/ops work, which is the bucket the budget is measured against. The
classifier reads the id (`IsHarnessBead`, scorecard.go), never the
configured `beads:` dir: a `.beads/redirect` can serve several repos'
issues out of one shared store (ADR 0015 §4) — this instance's own
`beads:` list holds a single entry whose redirect chain lands on the
shared queue db, and that db mixes the harness prefix and `ranger-base-`
ids — so the dir is not a repo boundary and the id prefix is the only
fact bd hands back that is. `IsHarnessBead` costs
nothing extra to compute: the ratio buckets the same `bd list --all --json`
rows the rest of the card already scanned, across the same repos and with
the same "scored N of M" caveat when one fails to read. In a single-repo
instance whose one bd project IS the harness, the ratio reads a trivial
100% — the number only means something once an instance's aggregation
spans a harness project and something else.

`posse cost [--since <date>] [--project <substr>] | --plan` is ADR 0003 §4's
accounting: the analyst's `bead-cost.py` method in Go. Every runtime with a
**cost adapter** (ADR 0012 D4, `internal/posse/costseam.go`) is segmented by the
dispatcher's "Work beads issue <id>" prompts; three ship, and a runtime with
no adapter is reported as *uncounted*, never $0.

- **claude** — `$CLAUDE_CONFIG_DIR/projects/*/*.jsonl`, `~/.claude`'s when the
  override is unset — the same config dir the trust file, `history.jsonl` and
  the credentials file resolve, because a walk rooted at `~/.claude`
  regardless (which this was until `ranger-base-yqdov`) lands on an absent
  root under an override, and an absent root is *never ran the CLI*: $0 with
  no error and no uncounted line, on the one runtime that carries dollars.
  Measured on 2.1.261 before the walk moved — the shipped bundle joins
  `projects` onto its config home in three places, and a headless run with
  `$HOME` moved wrote its transcript under the config dir, not the moved
  home. `FindClaudeTurnOutcome` reads the same locator, so it followed the
  override in the same commit. Assistant records are deduped by message id
  (streamed chunks repeat it — max per usage field) and priced
  at list rates per MTok for the model each record names (fable 10/50, opus
  5/25, sonnet 3/15, haiku 1/5; cache write 1.25× input for 5m TTL and 2× for
  1h when the breakdown is present, else 1.25× flat as the script did; cache
  read 0.1×). An id matching no family is **unpriced, not guessed**: the
  report says the total is a floor rather than putting an invented number in
  the same column as real money.
- **grok** — `$GROK_HOME/sessions/<url-encoded cwd>/<uuid>/updates.jsonl`,
  `~/.grok` when the override is unset — the same home the version probe and
  the turn-outcome reader resolve, because a walk rooted at `~/.grok`
  regardless (which this was until `ranger-base-z65xu`) lands on an absent
  root under an override, and an absent root is *never ran grok*: $0 with no
  error and no uncounted line. grok reports its own dollars (`costUsdTicks`,
  nano-dollars) per turn, so there is no rate card to keep current; the `modelUsage` breakdown restates the same
  spend and is deliberately not read (reading both is exactly 2×).
- **codex** — `$CODEX_HOME/sessions/YYYY/MM/DD/rollout-*.jsonl` (`~/.codex`
  unset, same rule and same fix as grok's above), decoded from the
  `token_count` events' **cumulative** `total_token_usage` by charging each
  segment the delta since the previous snapshot. Not by summing
  `last_token_usage`: codex re-emits a token_count with an identical snapshot
  a fraction of a second later, so summing the per-turn field reports ~2× —
  measured 2026-08-28 over the whole local history, 100 of 163 rollouts carry
  such duplicates and 15 of those were written by 0.147.0, the version running
  here, so it is not a version to wait out. Not by maxing the snapshot either:
  that is dedupe-proof but cannot attribute, so a session working two beads
  would charge the second one the first one's spend. Delta-from-cumulative is
  both; verified on all 159 local rollouts that carry token counts, the deltas
  sum to the file's final `total_token_usage` on 159 of 159 with no snapshot
  ever going backwards. Two field facts the shape depends on: `input_tokens`
  INCLUDES `cached_input_tokens`, and `reasoning_output_tokens` is a SUBSET of
  `output_tokens`.

**What is priced and what is not.** claude beads carry API-equivalent dollars.
grok beads carry the provider's own dollars. **codex beads carry no dollars at
all** and print a `—` in the `api$` column with a legend saying why: codex runs
on the operator's ChatGPT subscription, which reports no per-turn cost, and no
list rate applies to a plan seat. Their turns and tokens ARE the measurement.
A blank is deliberate — an invented figure at another provider's rates would
land in the same total as real money with nothing marking which dollars were
guessed, and would then move a budget window (ADR 0003 Dial E) on a number
nobody measured. Unpriced rows are excluded from the summary statistics and
from every group sum, and each group says so — a mixed group appends
`(3 of 12 unpriced — sum is a floor, median and per-bead are over the 9 priced)`,
and a group where `no bead here has a rate` prints a bare `—` for its sum.

Output: per bead (start, persona from the bead's assignee, **runtime**, tier
from the model that did the work, turns, tokens, api$), then by runtime /
tier / persona / day with median and per-bead, the interactive total (never
gated, shown for the ratio, and it names its own unpriced turns), and the
honest gaps — per pass is not attributable until dispatch records a pass id
(rangerhq-25p). The runtime column is what makes a mixed day readable: two
beads with the same tier and persona can have come out of two different pools,
and only one of them has a dollar figure. **`strong`/`standard` are `?` on
codex and grok**, and that is not an oversight: tier is re-derived from the
model id that did the work by `TierForModel`, and codex and grok both name
one id on two tiers (`gpt-5.6-sol` for codex strong and standard, `grok-4.6`
for grok's since rangerhq-jp6) — so the id does not identify a tier there
and the report says `?` rather than picking one, whichever tier a map
iteration happens to hit (ranger-base-3st5). `fast` DOES resolve on both:
`gpt-5.6-luna` and `grok-4.5` are each named by exactly one tier in their
runtime's built-in map, so a fast-tier turn on either identifies its own
tier unambiguously, same as claude. The rule stays symmetric across all
three runtimes — only an id ONE tier names may resolve — so a future
runtime whose ids collide the other way (`fast` shared, `strong`/`standard`
distinct) reports the shared one as `?` too. The cockpit shows
each per-bead session's running cost and the day total in the footer (rescanned
every 30s off the event loop).

**codex has a plan meter, and it is a hint, not a guard.** Every codex
`token_count` event also carries `payload.rate_limits`: `limit_id`,
`plan_type`, `primary`/`secondary` each `{used_percent, window_minutes,
resets_at}`, and `credits {has_credits, unlimited, balance}` plus
`spend_control_reached` — the same reading `planusage.go` gets from Claude's
endpoint, on disk, no network and no keychain. rangerhq-0va item 4 asked for
it in the cockpit header beside Claude's; **ADR 0034** decided the shape, and
it is not a second guard: the plan-window seam stays singular and codex enters
as a typed `PlanHint` that is DISPLAY only — D4, the decision that would
have let it refuse anything, is withdrawn, and every launch/brake decision
belongs to ADR 0010 —
because the reading is a snapshot outside its store of record whose staleness
is unbounded in the dangerous direction — the pool is account-wide, the
rollouts are box-local, so codex on another device drains it without this
file moving. Windows are named by duration (`codex_5h`, `codex_7d`), never by
slot: primary was the 5h window Jan–Jun 2026 and the weekly one in Aug, so a
slot-named threshold changes meaning under you — and `plan_type` moves too
(team → plus). Implementation is ranger-base-xb5f (the reader) and -ormb
(display, always with the reading's age); -3o10, the overflow advisory, went
with D4 and with the mechanism it advised.

The metric `cost-per-closed-bead` has a scorecard answerer for
h2c — `posse cost` by bead id against closes — so a PID that declares it
reads as `computed`. `--plan` skips all of the above and prints only the
plan's own rate windows (the plan-guard section has the reading); it takes
no other flags, because there is nothing for a date or a project to select.

`posse agent new <name>` scaffolds the PID shape — every frontmatter key
present (lists empty and commented, with one exception: `deny:` ships
`Bash(git commit unless --)` as a real entry, because a scaffolded persona
with no deny at all got the commit wall's L3 half and none of its L1 half,
ranger-base-09b7), every body heading in contract order with a one-line
hint, the four hard risk lines verbatim (`HardRiskLines`) — and opens it in
$EDITOR; `posse agent edit <name>`
reopens it. The scaffold parses with `LoadAgent` untouched. `posse agent
check <name>|--all` lints PIDs against the contract (identity line,
sections present and ordered, Intents table header, hard risk lines
verbatim, `{allow}`/`{deny}` last in `command:`, permission rules whose
parentheses an inline list split on a comma, metric ids that near-duplicate
another PID's spelling) and exits 1 on findings, so an instance repo can
run it in CI.

The metric catalog is **derived, not declared** (ADR 0001 amendment
2026-08-18): `App.MetricCatalog()` is the union of every loaded PID's
`metrics:` plus config `metric_ids:`, mapped to the personas that declare
each id. A persona naming how it is judged is the source of truth, so the
linter never rejects an id as "unknown"; what it does check is that the
vocabulary stays *one* spelling — `metricKey` lowercases, splits on
non-alphanumerics and stems each word loosely (-ing/-s/-e), so
`findings-survive-triage` and `findings-surviving-triage` collide and get a
"one spelling" finding naming the other PID. `posse scorecard --catalog`
lists the union with `computed` (the scorecard has an answerer) or
`declared` per id; per-persona lines read `declared, not yet computable:
<what bd would need>` for the rest, never "not in the catalog". Answerers
follow the PIDs' spelling (`findings-surviving-triage`; the ADR's original
stays a computed alias). The hint for *what bd would need* is specific only
for the ADR's own generic ids — the crew's ids are the instance's
vocabulary, not the product's, so anything else gets the honest general
answer.
`examples/agents/*.md` are all in PID shape (architect is the reference).
Runtime variants (codex, grok, …) are a `command:` choice, not an agent
file: pick the runtime by editing a PID's `command:` (or a recipe
`--cmd`); one persona, two runtimes = one PID and two recipes.

**The permission mode is a launch fact, never a default (rangerhq-qs5r).**
OPERATOR DIRECTIVE, 2026-08-22: all agents always start in auto mode.
Nothing on the claude line used to name a mode, so a persona session got
whatever the CLI defaulted to that week — and that moved under us:
two live persona sessions both landed in `manual` (footer `⏸
manual mode on`), blocked on consecutive approval dialogs nobody was
watching, and the operator cleared them by hand. The fleet's claude line
now types **`--permission-mode auto`** (`ClaudeFleetFlags`), the same mode
and the same reasoning as grok's: of the six modes (`acceptEdits`, `auto`,
`bypassPermissions`, `manual` — the old `default`, renamed — `dontAsk`,
`plan`) only `auto` and `bypassPermissions` approve a tool call with nobody
there, both still honour the deny list, and `auto` is the lower-privilege
of the two.

Verified live on claude 2.1.239, on a herdr pane running the full
fleet-shaped line (`--model --append-system-prompt --add-dir --settings
--disallowedTools`): `--permission-mode manual` → `⏸ manual mode on`,
`--permission-mode auto` → `⏵⏵ auto mode on`. The flag takes in both
directions, which is the whole point — the mode is now typed, not
inherited. Two things this is *not*: it is not typed keystrokes (a mode
cycled with shift+tab lands in whatever screen the pane is drawing —
cf. grok's splash, rangerhq-7sbo), and it is not a `.claude/**` permission
file (those are the operator's keys). Nothing in the launch reads a CLI
default any more, so a session cannot be one CLI release away from
blocking.

The guarantee is a *runtime* property, not a template detail:
`Runtime.Unattended` names the flag (claude `--permission-mode auto`, grok
the same, codex `-a never`) and `RenderCommandFor` appends it when the
rendered line does not name it. That covers the one template posse did not
write — a PID's own `command:` — and stops short of guessing: it appends
only when the line actually starts that runtime's executable, and a
template-only runtime (`Unattended` empty) is left alone, because a flag
typed at a CLI whose dialect nobody probed is a launch that does not start
at all. A `command:` that names a mode explicitly keeps the mode it named,
where `ps` can see it.

**Unattended sessions and Claude Code dialogs (rangerhq-4e5).** Claude
Code opens its "Set up auto mode for your environment?" dialog *after a
turn ends* whenever the session is in auto mode (which every fleet session
now is, by the flag above), the operator has never answered it, and it is
not otherwise
suppressed — with "Set it up" preselected. In a fleet pane nobody is
looking, so the next dispatched prompt (text + Enter) selects "Set it up"
and subsequent keystrokes operate a wizard that configures shell-history
and repo scanning. Reproduced on demand in a tmux pane; the gate (read
from the 2.1.233 binary) is: skill `auto-mode-setup` not disabled via
`skillOverrides` AND no `autoMode.environment` configured AND
`~/.claude.json` `autoModeEnvSetup` not `dismissed` (and no "Not now" in
the last 7 days — which the accidental "Set it up" path *clears*, hence
the recurrence on fresh sessions).

The fix posse ships: every claude persona command carries
`--settings '{"skillOverrides":{"auto-mode-setup":"off"}}'` (the
`ClaudeFleetSettings` constant; the default command and
`examples/agents/*` include it). Verified: same tmux repro, dialog gone
across turns, auth untouched. Alternatives, for the record: choosing
"Don't show again" once in any interactive session (writes
`autoModeEnvSetup.dismissed` to `~/.claude.json`), or the same
`skillOverrides` entry in `~/.claude/settings.json` — both are per-user
config outside the repo, so they are not what the harness relies on.
Rejected: `CLAUDE_CODE_SIMPLE=1` / `--bare` does suppress onboarding but
never reads OAuth/keychain credentials — a subscription-authenticated
fleet lands on "Not logged in" (verified). Also rejected: dispatch
sending Esc on a stalled prompt and retrying — that is more automated
keystrokes into an unknown modal; dispatch instead unclaims and benches
the session (rangerhq-81d), and the root cause is suppressed here.

**Identity.** A persona's durable identity is its beads assignee name.
Launching a persona session injects `BD_ACTOR=<persona>` into the workspace
environment, so every bd command the agent runs — claims, closes, mail —
is attributed to the persona automatically. `posse claim/done --as <persona>`
does the same from outside. herdr agent names are never load-bearing.

**BD_ACTOR is attribution, not authorization — accepted (rangerhq-pnp).**
bd has no authentication: the actor is an env var or `--actor` flag, and
any process in any session can run `BD_ACTOR=other bd close X`. That is
fine *at this scale* because all personas run as one OS user under one
operator — the OS user is the real principal, the actor is an audit
label, and the commit author on `.beads/issues.jsonl` is a second,
harder-to-forge signal for forensics. Nothing in posse grants privilege by
actor: `Route` honouring a bead's assignee and `launchSession` treating
"already claimed by this persona" as a resume are routing conveniences.
The line — any one of these flips the verdict, and the fix past it is
OS-level separation (per-persona user/container), not a smarter
BD_ACTOR: (1) beads start gating outward actions on assignee ("only
devops may close deploy beads", merge slots, `bd gate`); (2) personas
ingest untrusted input, because prompt injection → identity spoof →
close/reroute other personas' work; (3) more than one human or org
shares the database. Related guard already in place: `workPrompt` fences
bead-sourced text — the title is `%q`-quoted and labelled as data, and a
bead id that is not a plain token is refused before it is embedded in a
`bd show` command — so a title written by one persona (or a public
repo's `issues.jsonl`) cannot be another persona's first instruction.

**Memory.** `personas/<name>/` (under RHQ_HOME) is the persona-private
memory dir — the one memory kind posse owns (project memory belongs to
beads: `bd remember` / `bd prime`; runtime memory belongs to the agent
CLI). It is materialized at launch, seeded with `ORDERS.md` and a
`.gitignore`, exposed to the process as `RHQ_PERSONA_DIR` and to the command
template as `{memory}` (e.g. `--add-dir {memory}` for claude).
`posse memory <persona>` opens the standing orders in $EDITOR.

The `.gitignore` is there because the landing below sweeps the WHOLE dir and
that dir is also where a persona works: five `myai-suite*.out` captures of
test stdout were committed as one persona's standing orders (ranger-base-c9m7).
The filter is git's own rather than a list of blessed filenames, because an
allowlist stops landing a persona's real notes silently — four of the 29 files
tracked under this instance's `personas/` are deliberate work that an
ORDERS.md-and-`pending/` allowlist would have dropped. Seeded with `*.out` and
`*.log`; it is the persona's to grow and nothing rewrites it.

