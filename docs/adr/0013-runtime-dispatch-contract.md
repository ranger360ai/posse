# ADR 0013 — Runtime dispatch contract: what a runtime must guarantee to take work

*Status: accepted 2026-08-24 · owner: architect · amends 0003 §1
display, 0010 §1/§5, 0012 D4.1 · amended 2026-08-26: cl7 probe results
folded into §2/§4/Claims; codex rule-suppression decided in §4
(ranger-base-cl7, ranger-base-00f); §2 busy key gains a per-pass
ceiling (ranger-base-8h5p); §2/Claims narrowed: grok's typed fallback
has no `startup_wait:` to measure — pre-turn chrome is a race
(ranger-base-wjze) · amended 2026-08-27: §4 gains the reachability
half of record — the cage, not only the runtime, decides whether the
store of record can be written (ranger-base-hxhb, measured in
ranger-base-rhw/oyta); §2 layer 3's only instance retired — dispatch
presses no key at any screen, the layer stands empty as last resort
(rangerhq-6723, ranger-base-xqft) · amended 2026-08-28: xaev
placement measurements folded into §4/Claims — where each channel
lands is now measured, point 4's byte-cap hedge resolved false, the
suppression decision strengthened, not reopened (ranger-base-xaev,
ranger-base-93u0; trace in `0013-rules-precedence-probe.md`) ·
amended 2026-08-28 (later): §1 settle row gains its declared half —
`turn_outcome:` names the registered reader of the runtime's own first
turn, and a runtime declaring none wears a per-bead blindness clause
(ranger-base-02zr, folded by ranger-base-ivf0) · amended 2026-08-29:
§5's predicate ratified as `Runtime.CostPriced()` — do the runtime's
DOLLARS reach `posse cost` — not "is there an adapter"; read-but-unpriced
(codex) keeps the brake and the degrade's reason clause branches; §1
account row updated (ranger-base-0lg6, ratified in ranger-base-mykq)*

> ADR 0002 answered "can a persona *launch* safely on any runtime." ADR
> 0012 D4 answered "can a third engine be *added* without patching the
> harness." Neither asked whether a launched persona can *reliably take
> work*. One evening of two non-claude runtimes in production produced a
> new failure roughly every hour, each patched ad hoc (ranger-base-9fq).
> This is the dispatch-lifecycle contract. It does not re-open the cage.

## Context

Dispatch assumes a claude-shaped life: the CLI starts, herdr *sees* a
screen, a prompt is typed, the model follows `Done:`, `posse cost` can
price the turn, and the plan guard's meter is the meter the pass spends.
Each clause is an assumption wearing a universal name.

Measured 2026-08-24, on beads:

| # | assumption | what broke |
|---|---|---|
| 1 | a launched runtime is promptable | three first-run interstitials (privacy banner, version splash, update menu whose default is `brew upgrade`) made herdr report only `idle` / `default_known_agent_idle_fallback` (ranger-base-3j8) |
| 2 | 45s is enough, and a failed launch benches the persona | grok's cold start exceeds 45s on a *clean* screen; one miss burns the persona's busy key for the whole pass |
| 3 | `Done:` is acted on | 3/3 codex sessions did the work and skipped the bead; one left a dirty shared checkout (ranger-base-0fb). Claude and grok complied the same evening |
| 4 | spend is visible | `posse cost` reports codex/grok **uncounted** (ADR 0003 §4 already requires this honesty; it does not put a brake on the channel) |
| 5 | the plan guard gates the meter being spent | it gates the *pass* on one provider's windows, including when blind, so an off-meter drain was parked by an unreadable on-meter credential (ranger-base-ri4) |
| 6 | `tier: strong` means a strong model | `{model}` rendered empty on grok/codex (both mapped since — codex ranger-base-arm, grok rangerhq-jp6; a declared runtime with no `model_<tier>:` still renders empty); the catalog preflight is Anthropic-only and cannot see allotment exhaustion (ranger-base-1cc) |
| 7 | the PID is the instruction | grok and codex natively load `AGENTS.md` (and kin). That file already collided with a PID deny (rangerhq-cmfj). Whether that is the bookkeeping defect is untested |

ADR 0012 D4.1 currently says a runtime "starts idle awaiting a typed
prompt." That sentence is a *delivery method*. It became the dispatch
path. The idle-fallback is a liveness guess over an unrecognized screen
(`liveness-and-identity`); a prompt typed into it is at-most-once
delivery into a hole (`delivery-and-idempotency`).

Concepts in play: **liveness vs identity** (herdr `idle` is not "promptable"),
**single-writer / store of record** (the bead, not agent settle, is work's
truth — ADR 0011), **at-least-once + claim-as-fence** (the work prompt is a
message; the claim is what makes a retry safe), **TOCTOU** (a failed pane
read as "this persona is busy").

## Decision

A runtime is **dispatchable** when it can be declared against six stages.
Missing a stage is not a patch: it is a named degrade or a named refuse.
A new runtime is onboarded by filling the grid, not by discovering each
quirk in production.

```
launch → promptable → work → record → settle → account
```

Interactive `posse new` is unchanged: idle, awaiting a typed prompt.
This contract is what **dispatch** requires of that same process.

### 1. The grid — observable, who declares, missing →

| stage | observable | declared by | missing → |
|---|---|---|---|
| **launch** | argv0 herdr recognizes; PID delivered; unattended flag on the line; cage grants reach `beadsHome(cwd)` (§4 Reachability) | runtime template + herdr manifest (ADR 0002 / 0012 D4.1–3,6) | **refuse** the launch |
| **promptable** | the work prompt is the first user turn, *without* posse answering a dialog | runtime `prompt: argv` (preferred) or `prompt: typed` + `startup_wait:` | **refuse this launch**, loudly; see §2 |
| **work** | herdr `working` then a settled state | herdr detection (already) | wait ladder as today (NOTES §6–7); a timeout is a check-in, never an unclaim |
| **record** | bead `closed`, or a comment plus an ASK/question that takes it out of `bd ready` | runtime `record: trusted\|untrusted` (§4) | settle-without-record is **incomplete**, never ✓; unattended `--resume` re-prompts; see §4 |
| **settle** | herdr `idle`/`done`/`blocked` *Seen()* — a matched rule, not the idle-fallback — plus, where readable, the runtime's own record of what the first turn did | pane half: herdr (already); turn half: runtime `turn_outcome:`, a registry key naming the reader (today: `claude-transcript`, `grok-session-store`) | pane half: existing ignorance path, claim kept. Turn half: **turn-blind** — an exhausted account and a settle-without-close are the same line; the per-bead blindness clause names the missing fact, and the per-pass account-degraded report (§5) is the roll-up |
| **account** | dollars in `posse cost` — a cost-adapter reading that *prices* (`CostPriced()`), or an explicit uncounted cap | adapter registry (ADR 0012 D4; registering IS the declaration — no `cost_adapter:` field, a second hand-kept declaration drifted, ranger-base-0lg6) or config `uncounted_cap_<runtime>:` | **account-degraded**, two ways — UNCOUNTED (nothing reads it) or UNPRICED (read, never priced): loud every pass; dispatchable; the cap is the brake (§5) |

`posse runtime check <name>` prints this grid for a runtime. Unknown is
the expensive column to get wrong: a template-only yaml with no
declarations is `prompt: typed`, `record: untrusted`, uncounted, unmapped
tiers — dispatchable and noisy, not silent.

**Settle's declared half (added 2026-08-28, ranger-base-02zr).** herdr's
settle says the pane went quiet; it cannot say whether a model ever
handled the prompt. Claude writes an allotment refusal as a synthetic
assistant message, so the same pane-idle covers "worked and skipped the
bead" and "no work ran" — and the read that separates them used to be
keyed `p.runtime == DefaultRuntime`, ADR 0017 §3's shadow-predicate
class, measured to cost exactly what that section predicts: the same
stubbed refusal driven through production `Run` stops the pass with ⛔
on claude and is never even asked for on codex/grok, whose line then
names the *record* degrade — the wrong explanation — for a session that
never ran a turn (pin: `TestQAParityAccountRefusalIsNamedOnEveryRuntime`).
Now the read keys on the declaration: `turn_outcome:` names a reader in
the turnfailure.go registry (present-but-unregistered refuses at load,
listing what is on offer), the ⛔ line names the runtime whose account
refused, and a runtime declaring no reader is **turn-blind**: its
settle-without-close line says posse reads no turn outcome there — an
exhausted account settles exactly like this, `posse peek` before reading
it as work. A test-injected reader is the *reader*, never the
permission: blindness stays the declaration's to say.

The ⛔ line has **two arms**, because a refusal is not always a refusal to
*start* (ranger-base-qcu4c). "no work ran" was written for claude, whose
synthetic refusal IS the whole turn — the reader only reports one when it is
the first assistant record after the bead prompt, so nothing can have happened
ahead of it. It is a false claim on grok: 1 of the 7 refusals in this box's
history landed six model calls and 190,817 tokens into a turn that had been
running for a minute and a half, and the session on the other side of that
line may have edited files and commented on the bead. So the runtime's own
record says which arm prints — grok's `usage` object, read for *how much of
the turn ran* and never for whether it was refused — and a refusal with work
behind it prints `refused the turn mid-flight … the turn had already run (6
model calls, 5571 output tokens), so work may exist: posse peek <session> and
check the worktree`. A reader that cannot tell must say so in its docstring:
the line reads "no work" as *nothing ran*, which is a claim, and claude and
grok are each able to make it (claude by construction, grok off a `usage`
object present on all 186 turns this box has that served anything). Reader
promotion
follows `record:`'s pattern — after a measured artifact, not on
plausibility. codex (`~/.codex/sessions/*.jsonl`) and grok
(`$GROK_HOME/sessions/<cwd>/<id>/`) are reachable in principle (xaev),
but nobody has captured what either store records on an account refusal;
capturing that artifact is ranger-base-e123, and a reader bead follows
it iff the artifact discriminates. Known open edge: claude cannot see a
refusal that lands *after* a first answer — the reader stops at the first
assistant record and reads that turn as healthy, clearing a marker
(ranger-base-4ldma, capture-when-it-happens like codex's half).

No new template placeholder. `{prompt}` is rejected: an unrendered token
is a literal argv (ADR 0001/0002 lesson). Dispatch that has a prompt file
appends `"$(cat <file>)"` to the already-rendered line when `prompt:
argv`. Interactive launches append nothing.

### 2. Promptable — argv-first; never keystroke a machine-mutating menu

**Delivery.** All three built-in CLIs accept a positional prompt that
starts an *interactive* session (MEASURED via `--help` 2026-08-24:
`claude [prompt]`, `codex [PROMPT]`, `grok [PROMPT]`; each also has a
headless flag, `-p`/`exec`/`-p --single`, which this is not). Dispatch
uses that surface:

1. **Claim first** (the fence). A lost claim creates nothing.
2. Write the assembled work prompt (ADR 0005) to a per-bead file under
   `$RHQ_HOME/state/`.
3. Create the session with `"$(cat <file>)"` on the launch line.
4. Await a *Seen()* state, not "idle enough to type at." The prompt is
   already delivered; we are waiting for work to start, not for a
   composer to appear.
5. Create-fails-after-claim → unclaim (same cleanup as today's prompt
   failure).

Resume into a live session stays `agent prompt` (typed). `prompt: typed`
runtimes keep today's create → await promptable → claim → prompt, with a
per-runtime `startup_wait:` (default 45s).

**MEASURED 2026-08-25 (ranger-base-cl7, trace in
`0013-argv-prompt-probe.md`): argv held on both.** The positional prompt
became the first user turn with no harness keystroke, and both post-argv
screens matched a herdr `working` rule. Precisely: argv *sidesteps* the
interstitial as a delivery blocker; it does not suppress the banner
(codex's update banner still draws, grok's splash clears itself). The
probe also showed the typed fallback is **not a measurement task on
grok — there is no `startup_wait:` to measure**. Typed delivery needs a
named-rule idle before the first turn, and grok's pre-turn chrome is a
race, not a latency: one fresh pane emits OSC title `grok` from 0.41s
(ranger-base-z6n), another emits zero OSC bytes and no composer footer
until it has received a turn (ranger-base-3j8, monica's `agent
explain`). When the race goes the wrong way, no wait at any value
produces a match — waiting longer is waiting for chrome that only a
delivered prompt creates. So `prompt: typed` plus a measured
`startup_wait:` is a branch for runtimes *measured to deliver typed*
(claude), not the recipe when the argv probe fails: first measure
whether a fresh pane reliably reaches a named idle at all; only then is
there a number worth taking. Moot for grok today — its built-in is
`prompt: argv` — but the next runtime onboarded against this grid reads
this branch as the recipe, so it says so.

**Interstitials, three layers, cheapest first:**

1. **Sidestep** — argv (above). The screen is not the delivery channel.
2. **Instance config** — operator-owned facts posse documents and never
   writes: grok coding-data consent (already an operator click;
   `[Opt in]` donates private-repo prompts — a visibility line), grok's
   version pin (`etc/grok/version-pin.toml`, `[cli] auto_update = false`),
   codex skip-until-next-version (`~/.codex/version.json`). A first-run
   dialog whose **default action mutates the machine** is a launch
   **refuse** until that config silences it. Nothing blind-sends Enter.
   The coordinator's string-match Escape watchdog is a stopgap and is
   not the architecture.

   **Amended 2026-08-29 (ranger-base-9r33, from ranger-base-a9y9 and the
   ranger-base-gu9z verify), because the refuse above was documentation
   and nothing else — `Interstitial.Danger` reached `runtime check` and no
   launch path, so codex dispatched onto the very menu this paragraph
   gates.** The refuse now runs, and wiring it settled the two questions
   a9y9 left open:

   - **Who refuses.** Anything DISPATCHED refuses, and it refuses **above
     the claim** — `launchSession`, not `planLaunch`, because the argv
     ladder above claims the bead first and a refusal any later would
     hand back a bead it had already taken. `planLaunch` carries the same
     rule for every other bead-carrying path. An **interactive** launch
     warns `DEGRADED` and proceeds. That is §3 of ADR 0015's asymmetry,
     and here it is load-bearing rather than analogous: the operator's
     remedy for codex's menu is to ANSWER it, in a codex session, so a
     posse that refused interactive launches would have walled off the
     only way to clear its own refusal.
   - **The escape hatch is that asymmetry**, and there is no config key
     for one. An operator who has decided to live with the screen opens
     the session themselves; a launch nobody is watching does not get to
     make that decision.

   And the refusal is a **reading, never ignorance**: probes are
   tri-state (silenced / not silenced / unknown) and only the middle one
   refuses. An unreadable `~/.codex/version.json` — a box codex has never
   checked a release on — is unknown, and refusing there would refuse in
   the probe's own words ("cannot tell whether the update menu is
   silenced"). The screen is still not unguarded: herdr's `update_menu`
   rule names it `blocked`, so a launch that meets it fails by name.

   **Amended 2026-08-30 (ranger-base-vbp3, from the ranger-base-il14
   parity sweep).** 9r33 also excluded a screen with **no probe** —
   every `runtimes/<name>.yaml` interstitial — on the reasoning that
   declaring a screen documents it and never walls the declarer's own
   launches. That exclusion made this whole rule unreachable for the only
   runtimes that can newly meet it: the three built-ins are measured,
   claude's screen is `Seeded` and codex/grok deliver by argv, so the
   **first typed-delivery runtime with a machine-mutating dialog is by
   construction a declared one**. It was dispatched onto that dialog while
   `runtime check` printed LAUNCH REFUSE about it — the same "printed
   sentence, no launch path" defect 9r33 was filed for, one layer in.
   So: **a declared screen with `danger:` refuses too**, and it is still
   a reading rather than ignorance — `danger:` is not posse guessing at a
   config it cannot parse, it is the operator's own written statement that
   this screen's default action mutates their machine. Declaring it is
   choosing the wall. A declared screen **without** `danger:` still walls
   nothing, which is the documentation case 9r33 meant to protect. The
   refusal does not lift by silencing, because posse cannot read the key;
   what lifts it is dropping `danger:` from the profile, and both the
   refusal line and the `runtime check` grid say so.

   **Ratified 2026-08-30 (ranger-base-mzmv)**, with one ground the
   reversal itself did not state: 9r33's unknown-exclusion was safe
   *because of a backstop* — herdr's `update_menu` rule, posse's own
   manifest, still names codex's screen `blocked`, so an unknown reading
   left the screen guarded by name. A declared yaml runtime has no herdr
   rule unless the operator writes one, so the case 9r33 exempted was the
   *less* guarded one, not the more — MEASURED on the fixture: the work
   prompt was typed onto the danger screen while `runtime check` printed
   LAUNCH REFUSE about it. The residual cost is accepted knowingly: an
   operator who silences the screen in the CLI's own config must also
   drop `danger:`, losing the yaml's documentation of the screen for the
   next unsilenced box. Alternative priced and rejected: a declared
   silence predicate in the yaml (file + key + expected value), which
   would give the screen a probe and let silencing lift the refusal.
   Rejected for now because zero shipped or known yamls declare
   `danger:` — the mechanism would be invented ahead of its first user,
   and the first declarer who hits the wall with a genuinely silenced
   screen is the bead that specifies it.
3. **Declared keystrokes** — last resort, keyed on a herdr *rule id*
   (today: none — grok's splash was the only entry, retired in
   rangerhq-6723 once detection stopped calling it a blocker
   (rangerhq-1xsj) and the branch was measured never firing in a launch
   (rangerhq-3hb5)). The layer stays: any future table presses each key
   once, never Enter, and carries rangerhq-4mzt's two ratified
   assertions (only Esc; only rule ids from posse's own manifests) —
   `TestDispatchPathPressesNoKeys` fails until they return, and is
   edited in the same commit that revives the table. A speed bump, not
   a ban. The next candidate (codex's update menu, rangerhq-9py0) is a
   layer-2 case by this section's own rule: its default action mutates
   the machine, so it is a launch refuse until instance config silences
   it.

**Busy key.** Dial F already gives each bead its own session. A failure
of *this pane* is not a fact about the persona. Split:

- **session failure** (un-promptable, splash, this startup timed out) →
  slot stays free; the next bead gets a fresh Dial F session.
- **persona/runtime failure** (exe missing, auth, wall refuse) → bench
  the slot for the rest of the pass, as today.

Tonight's "one miss sterilises the queue" is the first arm wearing the
second arm's consequence.

**Ceiling (added 2026-08-26, ranger-base-8h5p).** The split's claim is
"*one* pane failing says nothing about the persona" — and one is the
quantifier, not a flourish. A slot's session failures within a pass are
consecutive attempts by construction (a success sets the busy key and
ends them), and each is a fresh Dial F pane sharing everything with the
last except the pane itself. Two identical failures on two independent
panes make the shared cause — the persona on this runtime — the better
explanation than two coincident pane-local ones. So the pane-local
explanation gets exactly one retry: the **second session failure of a
slot in one pass benches the slot** for the rest of the pass, wearing
the persona arm's consequence, with a line saying it was the second,
not the first. Both panes stay stranded so the working/blocked guard
ignores them, exactly as today.

Two is derived, not tuned. The floor is two: a ceiling of one is the
3j8 sterilise under a new name. And counts above two buy nothing,
because the argv delivery above already drained the benign case out of
this arm — a slow cold start now lands in `awaitDelivered`'s
seen=false outcome (launched, claim kept, judged next pass), not in
session failure, so what the arm still contains is dominated by
deterministic persona faults (exe missing, instant crash, auth exit)
that repeat identically per pane at ~`startup_wait:` each. A runtime
that legitimately fails detection twice on fresh panes has a wrong
`startup_wait:`, and that declared key — not this ceiling — is where
slowness belongs. The bench is pass-local like the rest of the busy
map: the next pass starts at zero, so a machine-load fluke costs at
most the tail of one pass.

### 3. Plan guard — the meter gates the beads that spend it

Amends ADR 0010 §1 and §5. The pass **always runs**. The reading is
still taken (shared snapshot, same TTL). The skip moves per bead:

```
off the guarded meter                    → launch, even if the guard is blind
on-meter and blind                       → skip this bead (park; never overflow)
on-meter and over threshold              → ADR 0010 ladder (overflow / skip)
```

A grok-only drain is no longer parked by an unreadable Anthropic
credential. On-meter beads still fail closed when unattended and blind.
Overflow remains a judgement *on a reading*; §5's "blind never overflows"
stands. A pass whose every bead skipped is a quiet pass; `--watch`
backs off on that, not on a whole-pass skip at the top of `Run`.

The `plan_guard_blind_max: 0` escape (ranger-base-ri4) is unnecessary
once this lands and must not be the way off-meter work is kept alive.

### 4. Record — the bead is the store of record; the runtime is not

The observable is the bead (ADR 0011, NOTES step 6). Agent settle is a
hint. That was already true; what was missing is what dispatch does when
the runtime does not write the store.

- `record: trusted` — a dispatched session of this runtime has been
  *measured* to close (claude; grok via the qa lane tonight). Built-in
  default only where measured. Promotion is a yaml/built-in edit after a
  measured close, not a new store.
- `record: untrusted` — default for every other runtime, including
  today's codex. Dispatch still launches. Gather never prints ✓ on
  settle-without-close. Unattended `--resume` stays on (ranger-base-f0g)
  so a skip is retried, not parked behind a busy key. This is not
  "monica closes the bead."

**One exception (RULED by the operator 2026-09-05, ranger-base-8fr2j;
built under ranger-base-4gy4i).** The harness may close a bead **it
filed** that **no session ever claimed** — status still `open`, no
assignee — because a bead nothing was ever dispatched onto grades
nobody's record, which is the harm this section names; a bead a seat
holds stays the seat's, and the close stays the persona's everywhere
else. The only caller is ci-watch's green half (`ciwatch.go`,
`ciHolder`), and it is registered by name in the arm-2 register of
`absencerules_qa_test.go` so that a second one has to be written down
before it compiles green.

**Reachability (added 2026-08-27, ranger-base-hxhb).** The record
stage has two inputs, not one. `record:` grades the runtime's
*willingness* to write the store; the **cage** decides its
*reachability*, and a session whose sandbox cannot open the store of
record cannot do the record stage under any runtime grade. MEASURED
(ranger-base-rhw, verified both directions in oyta): hoover — runtime
claude, `record: trusted`, the best grade this section gives — under
the pre-23c4e54 seatbelt profile had `bd sync`, `bd export` and the
path-limited commit all denied at the db file / index.lock, while
`bd create`/`close`/`comments add` "worked" only because the daemon
socket crosses the cage and exports on a timer nobody watches; with
the daemon absent, direct mode fails at the same db open, and the
trusted session records nothing at all. No existing observable sees
this half: parity grades *denies*, so a cage that denies too much
prints "all gates realized"; settle looks normal; and the bead —
the store this contract nominates as truth — shows nothing, which
is the one signal that cannot be watched for.

So reachability is a **launch observable**, not a post-hoc one:
before dispatch, the cage's grants must reach `beadsHome(cwd)` and
its git dirs, judged against the **rendered artifact** — for seatbelt
the profile's last-match-wins semantics, not the writable list that
fed it, because a trailing deny naming the target re-denies it
silently (measured against ranger-base-h15's proposed carve-out) —
and a miss refuses the launch through the existing parity path,
`--allow-degraded` and all. The class recurs by construction: every
future narrowing of the writable set reviews as strictly safer, so
the check, not the review, is what keeps the record stage loud at
the next launch instead of quiet until the daemon dies.

**Third answer (added 2026-08-29, ranger-base-heur).** The two answers
above stand, but the row has a third. The probe *is* a `sandbox-exec`,
and the kernel refuses a nested `sandbox_apply` — so a posse command
rendering this row from inside a caged persona session cannot run the
probe at all, for a reason that says nothing about the profile
(MEASURED, heur: any outer profile carrying a deny refuses every nested
apply, `sandbox_apply: Operation not permitted`, exit 71). That refusal
is the third answer: not granted, not denied — **not applied**. It
lands in `Realized` as an unmeasured note rather than through
`unrealized()`, because a launch must not be degraded by a check that
did not run; before the fix the kernel's words printed as a wall
verdict about the store of record, and every reach row rendered from
inside a cage degraded the launch on a measurement that never happened.

Cage *availability* stays a **host** question on purpose (developer's
call, recorded rather than assumed): `SeatbeltAvailable()` asks about
the session posse is about to launch, whose `sandbox-exec` is typed
into a herdr pane outside this process's sandbox — not whether this
process can apply a profile ("is the binary there" and "can I apply
one" are different questions with different answers in the same caged
session: `sandbox-exec` stays on PATH inside the cage, measured in
heur). So `AvailableCages` keeps seatbelt even when posse itself runs
caged. Dropping it there would fall this row through to the default
arm, which claims "cage <x> has no file wall — every path this session
can write, it can write" — a stronger claim than "unmeasured," and
false.

**Reap guard.** A session whose bead is still `in_progress` and whose
cwd is dirty is **not killed**. The 353-line near-miss is a shared
checkout plus a reap, not a missing `Done:` line. L3's pathspec rule
already stops an unqualified commit; it does not stop `posse kill`.

**Native rulebooks.** A runtime declares `native_rules: [AGENTS.md, …]`
(grok's list is longer — `Agents.md`, `CLAUDE.md`, …). Posse does not
rewrite the operator's `AGENTS.md` (shared checkout, operator's file).
The work-prompt line "PID guardrails override repo docs" stays. Whether
a native file outranks the PID is a **probe**, not a patch; a runtime
that fails it stays `record: untrusted`.

The probe said (MEASURED 2026-08-25, cl7): codex `-c
project_doc_max_bytes=0` removes the project `AGENTS.md` from the
model-visible prompt while the argv prompt survives;
`project_doc_fallback_filenames=[]` does not; no tested grok flag
silences its discovery. So the original "no flag on either CLI silences
project rules" was false for codex, and dispatch has a choice it must
not make silently (dinesh's DIVERGED on cl7).

**Placement (MEASURED 2026-08-28, ranger-base-xaev; trace in
`0013-rules-precedence-probe.md`; codex 0.147.0, grok 1.0.5, no billed
turn).** Where each channel lands in the assembled prompt. Codex: the
PID (`developer_instructions`) is prepended verbatim to codex's own
first `developer` message, index 0 of the model-visible input list;
the native rulebooks are one later `user` message — `~/.codex/AGENTS.md`
first, a literal `--- project-doc ---`, then the project doc —
immediately before the argv work prompt, and `AGENTS.override.md`
*replaces* `AGENTS.md` when both exist. Grok: the PID (`--rules`) is a
`<human_rules>` block at the tail of the locally-assembled system
prompt; the native rulebooks are not in the system prompt at all —
they ride a separate structured `agents_md_files` field whose
model-visible placement is decided by grok's own harness downstream
and is not observable from any local artifact. So the structure
*supports* the PID-wins prompt line on codex (stronger role, earlier
position — though codex's own harness text endorses `AGENTS.md` as an
authority twice) and leaves it structurally unmeasurable on grok,
whose shipped docs declare later-in-context wins on conflict. The
revisit trigger below did not fire. Structure is not behavior: the
billed half is parked on the operator money question ranger-base-6rcv.

One hazard from the same probe: grok's `--system-prompt-override`
silently discards `--rules` — the entire PID channel — while leaving
every native rulebook in place (vendor-documented). No built-in
template uses the flag; a PID's own hand-written `command:` is the
reachable path (ranger-base-64qx).

**Decided (ranger-base-64qx): a launch line that voids the PID channel
is REFUSED, not repaired.** `Runtime.PIDVoid` declares the flags that
make a runtime ignore the PID its own template delivers — grok's
`--system-prompt-override` and the compat alias `--system-prompt`, both
measured on 1.0.5 — and every path that renders a persona line
(`planLaunch`, `RelaunchAgent`) refuses when the rendered line names
one. The unattended flag next door gets a *repair* because it is absent
and appendable; this one is present and ignored, so there is nothing to
restore: the measured arm already carried `--rules`, and appending it
again buys a launch that looks fixed and is not. The repair that would
work — rewriting the operator's override text to carry the PID — is an
edit to a hand-written `command:`, which posse does not make. A refusal
rather than a DEGRADED launch, because `degraded` is for a gate no wall
layer could realize, and a persona that is not in the session at all is
not a weaker persona. Built-in declaration only, like `Unattended` and
for the same reason: on a template-only runtime posse knows no CLI's
dialect, and a guessed flag name would refuse launches for a spelling
nobody measured.

**Decided (ranger-base-00f): dispatch does not use the codex override.**
`native_rules:` stays a declaration, not a switch, on every runtime:

1. *It buys no invariant.* Grok cannot suppress, so "a dispatched
   session may carry a second rulebook" stays true fleet-wide and every
   existing mitigation stays anyway — the PID-wins prompt line, the
   precedence probe, `record:`, the cage owning the hard lines.
   Suppression on one runtime removes no mechanism; it adds variance:
   the same bead gets different effective instructions depending on
   which runtime claimed it, chosen silently by dispatch.
2. *It drifts silently.* The key is measured on codex 0.147.0 only. A
   later codex that renames or re-scopes it reloads `AGENTS.md` with no
   observable in dispatch — the ADR 0009 lesson (the per-runtime flag
   is what drifts) in rulebook costume. The declared posture keeps an
   observable: `codex debug prompt-input`, the probe's own instrument.
3. *It splits the operator's brain.* Native discovery is the CLI
   behavior the operator installed; suppressing it only under dispatch
   yields "codex honors my `AGENTS.md` by hand and ignores it under
   posse." This ADR already rejected the session-copy variant on the
   operator's-file ground; a flag differs in mechanism, not effect.
4. *It cannot deliver what it promised.* MEASURED (ranger-base-xaev,
   2026-08-28, previously the ASSUMED half): `project_doc_max_bytes`
   is a *project-doc* byte cap, not a rulebook switch. At `=0` the
   project doc goes while `~/.codex/AGENTS.md` survives at full
   length in the same user message; at `=40` the project doc
   truncates mid-marker with the home doc untouched. The flag never
   yields a single rulebook while a home `AGENTS.md` exists — so the
   suppression on offer was always partial, wearing a total name.

The key joins §2 layer 2 as an operator-owned fact posse **documents
and never writes**: an operator who wants codex doc-free sets
`project_doc_max_bytes = 0` in `~/.codex/config.toml` (instance-wide,
no dispatch/interactive split), or carries the `-c` override in their
own `runtimes/<name>.yaml` `command:` if they insist on dispatch-only.
Zero new posse surface either way. Revisit trigger: if the precedence
probe ever measures codex's native `AGENTS.md` overriding the PID
guardrails line, suppression returns as a mitigation applied to a
measurement — not as a default.

### 5. Account — no dollar meter is a degrade, not a zero

Uncounted-never-$0 stands (ADR 0003 §4). A live spend channel with a
human eyeballing it is the hole. The predicate is `Runtime.CostPriced()`
— do this runtime's DOLLARS reach `posse cost` — and NOT "is there an
adapter", because those came apart (ranger-base-0lg6; this section
originally said "a runtime with no cost adapter", ratified as amended in
ranger-base-mykq). Two ways to fail it, both **account-degraded**, and
they are different facts the degrade line must not print the same
sentence about (`accountDegrade`, uncounted.go):

- **UNCOUNTED** — nothing reads the runtime; its sessions are absent
  from every total. The state this section was written for.
- **UNPRICED** — an adapter reads it (turns, tokens, per-bead
  attribution) and prices none of what it reads. codex is this: a plan
  seat reports no cost and no list rate applies to one. "No cost
  adapter reads codex" is a false sentence, and was printed every pass.

The vocabulary is not new — the cockpit has printed `$uncounted` /
`$unpriced` / a figure since codex's adapter landed (`sessionCost`,
cmd/posse/cockpit.go); this amendment brings the account stage and the
dispatch degrade into line with it. Both states keep the obligations,
because what the cap stands in for is a missing DOLLAR meter, equally
missing whether nothing reads the pool or something reads it and cannot
price it:

- every pass names how many launches it sent there;
- config `uncounted_cap_<runtime>:` (beads / rolling 7 days, same shape
  as ADR 0010's overflow cap) is the brake; **unset = unlimited and
  loud**, the budget_* dormancy pattern;
- registering an adapter that *prices* (0012 D4) is how a runtime
  *leaves* this column; registering one that only reads is not — keying
  the brake on "has an adapter" would have removed the only brake on a
  subscription seat on the strength of a token count. Numbers and which
  meters to read are the operator's; the mechanism does not invent a
  price table.

There is no `cost_adapter:` declaration to write anywhere: the registry
is the only answer, resolved per call. The hand-kept string this ADR
originally implied drifted within two days of the fleet gaining a second
adapter (ranger-base-0lg6), which is ADR 0017 §3's second-declaration
class doing exactly what that register predicts.

No autonomous spending: the cap is a count of beads posse itself
launched, not a bill.

*(2026-09-02, ranger-base-ws09)* The ledger's WRITABILITY is half of the
cap's reading, the rule ADR 0010 §3 already took for `overflow.log`
(ranger-base-2y96). `uncounted.log` asked only whether it could be READ,
so a readable-but-unwritable one counted every pass at whatever it
already said — cap 1 over an empty `0444` ledger admitted one launch per
pass forever, recorded none, and warned about each only after the launch.
With a cap set, `uncountedFor` now probes `UncountedAppendable()` and
`uncountedSkip` fails closed on it beside the unreadable case; an append
that fails anyway (a full disk, which no open can see in advance) arms
the same brake for the rest of the pass and the account line names how
many of the pass's launches the file will never hold. An UNSET cap is
untouched: unlimited stays unlimited, loud by the report.
`docs/notes.d/ranger-base-ws09.md`.

### 6. Tier — the name is intent; the mapping is declared

Amends ADR 0003 §1 display, not the three names. `strong` / `standard` /
`fast` remain "judged / building / mechanical." A runtime that does not
map a tier **does not wear it**:

- `{model}` empty stays the render;
- `posse list` / cockpit / prompt header show `<runtime>/default`, never
  `<runtime>/strong`;
- PID `tier: strong` on an unmapped runtime is a `posse agent check` /
  `runtime check` warning, not a quality guarantee;
- dispatch's own overflow still never moves `strong` (ADR 0010 §2b);
- an explicit `--runtime` the operator typed is their decision and
  launches.

*(2026-08-29, rangerhq-jp6.)* All three built-ins map every tier now, so
this rule bites on a declared `runtimes/<name>.yaml` that sets no
`model_<tier>:`, on a partial map, and on a runtime name posse has never
heard of. Narrower, not dead — and the pins for it are fixtured on a
declared runtime rather than on whichever built-in happened to be blank,
which is how a rule about the map stops being tested the day somebody
fills that map in.

Availability preflight is per cost/plan adapter — as shipped, the
predicate is the runtime's `egress:` naming the catalog host
(`anthropicAPI`, `modelavail.go`; ranger-base-lzx), not the adapter
table. No catalog → no preflight. Dead-on-arrival (allotment message, one assistant turn, idle)
is a **turn outcome**, not a catalog miss (ranger-base-1cc shipped the
detection half; ranger-base-02zr keyed it on `turn_outcome:` instead of
the runtime's name). A runtime without that probe launches what it was
asked — and "must say so when the first turn is a limit" is now
realized: with a reader the ⛔ line names the refusing runtime, without
one the per-bead turn-blind clause says posse cannot tell (§1 settle
row).

## Consequences

- `runtime.go` / yaml: `prompt: argv|typed`, `startup_wait:`,
  `record: trusted|untrusted`, `native_rules:`, existing `model_<tier>:`;
  since 2026-08-28 also `turn_outcome: <registry key>` (claude
  `claude-transcript` — ranger-base-02zr; grok `grok-session-store` since
  2026-09-05, promoted on the captured artifact rather than a guessed
  shape — ranger-base-e123's probe, then ranger-base-fc8go; codex none,
  its refusal artifact still uncaptured).
  Built-in defaults after the probe (it held, 2026-08-25): claude
  `typed` (works; argv is an allowed later unify), grok/codex `argv`
  with no `startup_wait:`; claude+grok `record: trusted`, codex
  `untrusted`. Shipped in ranger-base-dg5.
- `dispatch.go`: claim-then-argv path; busy-key split; plan-guard skip
  is per-bead including blind; gather never ✓ on untrusted
  settle-without-close.
- `posse kill` / refresh land: dirty+open-bead reap refuse.
- `posse runtime check <name>`: the grid, one screen.
- Config: `uncounted_cap_<runtime>:`. NOTES/INSTALL: instance
  interstitial keys (document, don't write).
- ADR 0010 "Fixes in passing: off-meter launches are no longer skipped"
  was true only of a *threshold trip*. Blind is the other stop. This
  ADR is that remainder.
- Metric: `record-skip-rate` by runtime (settled-open / dispatched) —
  how a runtime earns `trusted`.
- §4 Reachability (2026-08-27): parity gains a reachability row — at
  launch, the cage about to be used must reach `beadsHome(cwd)` and
  its git dirs (shims: no file wall, trivially passes; seatbelt: the
  rendered profile judged behaviorally; codex cage: membership of the
  `--add-dir` set realizeCodex emits). A miss is the existing
  degradedError refusal. Cut as ranger-base implementation beads from
  ranger-base-hxhb.
- §4 Reachability, the launch half (2026-08-28, ranger-base-xqwr): the
  row found the codex launch line one grant short — it named
  `beadsHome(dir)` and the SESSION's git dirs, never the STORE repo's,
  so `bd sync`'s commit of the JSONL died on its `index.lock` exactly
  as the pre-23c4e54 seatbelt did (ranger-base-rhw). `launchWritableRoots`
  is now the one list both the launch and this row read, so they cannot
  disagree about what "writable" meant. THE TRADE: `--add-dir` is
  directory-granular, so the store repo's refs, hooks and config are
  granted whole — the same gap `sessionGitGrants` already accepts and
  states for the session's own repo, extended to the store's. Nothing
  narrower exists at that wall; narrowing it needs a different wall.

## Alternatives rejected

- **Declared keystroke lists as the architecture** (the clever one).
  Runtime yaml names screens, posse types keys. Tonight's existence
  proof is the coordinator's Escape watchdog. It races a splash
  (rangerhq-5cj/7sbo), cannot tell "Update now" from "Skip" without
  peeking, and makes Enter on an unknown menu a `brew upgrade` of the
  operator's tooling. Argv retires the screen as the delivery channel.
  Keystrokes stay last-resort and Esc-only.
- **Headless (`grok -p` / `codex exec` / `claude -p`).** No splash by
  construction. Also no herdr agent, no peek, no wait ladder. Oversight
  lives in herdr (DIRECTION). A headless runtime is a different product.
- **Refuse to dispatch any runtime that is not claude-shaped until a
  full conformance suite is green.** Purity. The qa lane on grok already
  closed work properly tonight; parking that is the opposite of
  degrade-loudly. Unknown starts noisy and untrusted, not forbidden.
- **Pre-answer interstitials from the harness** (write
  `privacy_banner_acked`, write `version.json`). Coding-data consent is
  a visibility line; update-skip is the operator's pin. Posse documents
  the keys. Posse does not write the operator's CLI config.
- **Rename tier to a claude-only word.** The three names are intent;
  the lie was displaying an unmapped mapping. Honest display is cheaper
  than a vocabulary split.
- **Per-pool budget model** (ADR 0010 already rejected). Still no
  meters to feed it. `uncounted_cap_` is the substitute in beads, same
  as overflow's cap, dormant when unset.
- **`{prompt}` in the closed placeholder set.** Unrendered, it is a
  literal argument to the CLI. Append-on-dispatch does not touch PID
  `command:` templates.
- **Harness closes the bead on the agent's behalf.** Hides the defect
  and puts a human (or a heuristic) in the loop dispatch exists to
  replace. Resume-until-record is the harness's job; `bd close` is the
  persona's. (Narrowed 2026-09-05 by the §4 exception above: a bead the
  harness filed that no session ever claimed is on no agent's behalf,
  and closing it hides no defect — ranger-base-8fr2j.)
- **Silence `AGENTS.md` by writing a session copy.** The session cwd
  *is* the repo. That file is the operator's. A competing rulebook is
  declared and probed, not overwritten.
- **Codex single-rulebook via `-c project_doc_max_bytes=0` on the
  dispatch line** (the clever one, and measured to work on 0.147.0 —
  added 2026-08-26 after cl7 falsified "no such flag exists"). Rejected
  in §4: no fleet invariant gained (grok cannot join), silent re-enable
  on a key rename with no observable, a dispatched-vs-interactive split
  over the operator's own file — and, measured after the rejection
  (xaev, 2026-08-28), the `~/.codex/AGENTS.md` channel survives the
  cap at full length, so the flag never produced a single rulebook in
  the first place. Priced honestly: the saving is one argv token and
  the *partial* retirement of the cmfj collision class on one runtime
  (the home doc still rides); the cost is a `native_rules:` surface
  that means different things per runtime. The saving's ceiling is now
  MEASURED; that no incident has yet been caused by codex reading
  `AGENTS.md` under dispatch stays ASSUMED.
- **Fold reachability into `record:`** (a third value, or a
  `trusted-unreachable` grade — added 2026-08-27, ranger-base-hxhb).
  `record:` is a per-runtime declaration promoted after a measured
  close; reachability is a per-launch fact of cwd × cage × rendered
  profile. The same runtime is reachable in one cage and not another
  on the same evening, so a launch fact wearing a declaration's
  clothes is wrong the first time they differ — and it drifts
  silently, the declaration never re-measured against the cage.
- **Detect unreachability post-hoc** (gather flags
  settle-without-sync; a jsonl-drift monitor). The daemon papers over
  the write verbs on a timer it does not publish, so the defect
  surfaces only at daemon-down or commit time — after the spend, in
  the exact stage the store of record can no longer report. rhw ran
  for a full session doing everything the work prompt asked before
  anything showed. Post-hoc watching of a store the session cannot
  write is watching for an absence; the launch check watches a
  presence.
- **Check the cage's allow list instead of the rendered artifact**
  (the cheap version of the launch check). Measured wrong the same
  way a code review of it would be: SBPL is last-match-wins, and a
  trailing `(deny file-write*)` naming the beads target re-denies a
  target the allow list still grants (h15 interaction, measured in
  oyta). The list is the input; the profile is the semantics; judge
  the thing that runs.
- **A fifth store for "this runtime is trusted."** Trust is a built-in
  / yaml default updated after a measured close. Derived auto-promotion
  is a store that can disagree with the bead (ADR 0011's class).
- **Leaving the session-failure arm unbounded** (what ranger-base-dg5
  shipped, implementing this ADR's original §2 literally — added
  2026-08-26, ranger-base-8h5p). Its defense was "a pass is already
  bounded by `-n`, and `--watch` backs off on a quiet pass." Both
  halves fail on reading: `-n` defaults to 0, unlimited, and each
  failed launch consumes an attempt *and* a serialized
  `startup_wait:`, so a persona whose CLI is broken in any
  non-loadable way turns the pass into its own wait loop — thirty
  ready beads is thirty waits, ~22 minutes, delaying every persona
  after it in the same pass; and a mixed fleet's pass is never
  zero-dispatch, so `--watch` never backs off on it.
- **An exec preflight instead of a ceiling** (the arm that "does not
  invent policy": LookPath the runtime's argv0 before launching; a
  miss is the persona failure the grid already names). Rejected as a
  gate because it answers in the wrong environment: posse's process,
  the herdr daemon, and the pane's login shell hold three different
  PATHs — INSTALL already warns about exactly this split — and a
  false miss (scheduled dispatch's lean PATH, the pane's full one)
  benches a healthy fleet on a guess. A detector that fails in the
  dangerous direction is worse than none. It also covers only one
  inhabitant of the arm: auth-exit and instant-crash resolve on PATH
  fine and drain identically. Allowed as *diagnosis only*, the NOTES
  precedent: a session-failure line may add that the exe is absent
  from posse's own PATH, worded as posse's observation and never the
  pane's, and the stranded pane's own tail (`posse peek`) remains the
  honest instrument. Diagnosis decides nothing.

## Claims

**MEASURED**

- Three interstitials, 45s miss on a clean grok screen, busy-key
  sterilise, 3/3 codex bookkeeping skips, one dirty checkout, uncounted
  cost, plan-guard-gates-the-pass including blind, `{model}` empty on
  grok/codex, oay preflight Anthropic-only, `AGENTS.md` collision on
  claude resolved by PID (ranger-base-3j8, 0fb, ri4, 1cc, cmfj).
- All three CLIs accept a positional interactive prompt (`--help`
  2026-08-24). Grok `-p` / codex `exec` / claude `-p` are the headless
  cousins and are not this path.
- Grok splash: composer is live under the menu; Esc does not undraw;
  Enter on grok's menu is dangerous; Enter on codex's menu runs `brew
  upgrade` (rangerhq-7sbo, ranger-base-3j8).
- Grok pin already kills auto-update (`etc/grok/version-pin.toml`).
- Qa on grok closed a bead properly; that is why grok is `record:
  trusted` and codex is not.
- ADR 0010 already launches off-meter beads through a *tripped*
  threshold. Blind still whole-pass-skips. `OnGuardedMeter` already
  knows the built-ins.
- **The cl7 probe, 2026-08-25** (codex 0.147.0, grok 1.0.5, herdr
  0.8.0; trace `0013-argv-prompt-probe.md`): argv is the first user
  turn on both with no harness keystroke; both post-argv screens match
  a herdr `working` rule (offline manifest replay); the interstitial is
  sidestepped, not visually suppressed. Codex `-c
  project_doc_max_bytes=0` removes the project `AGENTS.md` from the
  model-visible prompt; `project_doc_fallback_filenames=[]` does not;
  no tested grok flag silences its discovery.
- The measured pre-turn grok pane (wJ9) matched **no** herdr rule —
  zero OSC bytes, no composer footer — and the same pane after one turn
  matched three (ranger-base-3j8, monica's `agent explain`). Not
  universal: another fresh pane carried OSC title `grok` from 0.41s
  (ranger-base-z6n, laurie). Pre-turn detectability on grok is a race
  posse does not control, and when it goes the wrong way no
  `startup_wait:` value produces a match — the typed fallback has no
  number to measure there.
- Codex's update persist-skip is an instance-config write
  (`dismissed_version` in `~/.codex/version.json`), documented in
  NOTES/INSTALL; `runtime check` prints it against `latest_version`
  because the dismissal expires per release. Amended
  **2026-09-02 (ranger-base-cohw)**: those two fields alone are not a
  reading about the screen. The menu draws only when a release newer
  than the RUNNING one exists, so a box that updated instead of
  dismissing read un-silenced forever and this section's refusal walled
  the most up-to-date box (measured on codex-cli 0.150.1 against
  `latest_version` 0.150.1, with two live witnesses that no menu drew).
  The probe reads the installed `codex --version` as a third arm, and
  reads UNKNOWN — which refuses nothing — when it cannot.

- **The xaev placement probe, 2026-08-28** (codex 0.147.0, grok 1.0.5,
  no billed turn; trace `0013-rules-precedence-probe.md`): codex PID =
  `developer` role at index 0, prepended to codex's own developer
  message; native rulebooks = one `user` message (home doc,
  `--- project-doc ---`, project doc) immediately before the argv
  turn; `AGENTS.override.md` replaces `AGENTS.md`.
  `project_doc_max_bytes` caps the project doc only — at `=0`,
  `~/.codex/AGENTS.md` survives at full length, so the flag never
  yields a single rulebook. Grok PID = `<human_rules>` at the tail of
  the locally-assembled system prompt; native rulebooks = a separate
  `agents_md_files` structured field whose model-visible placement is
  decided downstream and is not locally observable; and
  `--system-prompt-override` silently discards `--rules` — the PID
  channel — while leaving every native rulebook (ranger-base-64qx).
  Re-measured 2026-08-28 for that bead: the compat alias
  `--system-prompt` voids it identically (19 B of override text, no
  `<human_rules>`, no PID marker, `agents_md_files` intact), so the
  refusal lists both spellings.

- The cage half of record, 2026-08-26 (ranger-base-rhw, A/B'd
  control-vs-fix in oyta): under the pre-23c4e54 seatbelt profile,
  `bd sync` / `bd export` / the path-limited commit all fail at the
  redirect target (ADR 0012 D3-C) with "operation not permitted",
  while daemon-socket verbs succeed; `bd --sandbox show` fails at the
  db open, so daemon-down means every bd write verb fails the same
  way; parity printed "all gates realized" throughout, because it
  grades denies. And a trailing `(deny file-write*)` naming the beads
  target re-denies it under last-match-wins — the recurrence path §4
  Reachability exists to catch.

- The nested-apply half, 2026-08-29 (ranger-base-heur, darwin 25.4.0):
  any outer seatbelt profile carrying a deny refuses every nested
  `sandbox_apply` (exit 71), while `sandbox-exec` stays on PATH inside
  the cage — the reach row's third answer (§4, "not applied") and the
  on-purpose host scope of `AvailableCages` both come from that
  measurement. Background: docs/notes.d/ranger-base-heur.md.

- The turn-outcome parity fixture, 2026-08-28 (ranger-base-unzn →
  ranger-base-02zr): the same stubbed allotment refusal driven through
  production `Dispatcher.Run` on all three built-ins — claude asked the
  turn outcome once and printed ⛔; codex and grok asked zero times and
  printed the ordinary ◑ settle with the record clause. The same day, by
  hand (runtimewalk probes, ranger-base-nlya): grok's account was
  returning `402 Payment Required` while a pass would have called it an
  ordinary settle; codex's account served a turn. Pin:
  `TestQAParityAccountRefusalIsNamedOnEveryRuntime`
  (internal/posse/dispatchparity_qa_test.go).

- Read from source, 2026-08-26 (ranger-base-8h5p): `-n` defaults to 0
  = unlimited (`cmd/posse/main.go`), and `fireLoop` counts a failed
  launch as an attempt, so the unbounded-arm cost above is the
  default, not an edge; `awaitDelivered` returns seen=false — a
  successful launch, claim kept — when a pane exists but no rule
  matched, so on `prompt: argv` runtimes a slow cold start does not
  reach the session-failure arm at all; session failures of one slot
  in one pass are consecutive attempts by construction (any success
  sets the busy key).

**ASSUMED** (still, after both probes)

- Whether any native rulebook outranks the PID channel in live model
  behavior. The structural half is now measured (xaev): placement
  supports PID-wins on codex and is unmeasurable locally on grok. The
  behavioral half needs a billed turn and is parked on the operator
  money question ranger-base-6rcv; until it is measured, the PID-wins
  prompt line is the reconciliation and codex stays
  `record: untrusted`.
- Cost-adapter internals for grok/codex exist behind 0012 D4 and are
  not designed here; `uncounted_cap_` is the brake until they do.
  (Resolved 2026-08-29: both exist — grok's carries provider-reported
  dollars and left the column, ranger-base-k7nb/0lg6; codex's reads and
  prices nothing, so its cap stands — §5 as amended in ranger-base-mykq.)
- That deterministic persona faults dominate what the session-failure
  arm still catches after argv. Read from the mechanism (the benign
  case migrated to seen=false), not tallied from a fleet; the
  `record-skip-rate` cousin for this arm would be a
  second-failure-benches count per runtime, worth eyeballing before
  anyone proposes raising the ceiling.
- That claude's first-run interstitials recur on every fresh dispatch
  pane until the operator dismisses one by hand (nothing under
  dispatch presses Enter). If true, bench-on-second is the right
  outcome there too — panes cannot fix an instance-config fact (§2
  layer 2). Unmeasured since the 3j8 evening.
