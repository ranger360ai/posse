# ADR 0042 — An L1 deny may stand in front of the binary a runtime authenticates itself with: a crew runtime authenticates with posse's mint, and the shim is what keeps the operator's store single-writer

*Status: accepted 2026-09-02 · owner: architect · from ranger-base-eje6d
(handoff of ranger-base-eupf, which measured the collision and shipped
the launch warning) · amends 0002 §3 (the L1 row: the typed-line prepend
is a decision about the runtime process, not only a `path_helper`
workaround), 0009 §3 (arm 2 named as the per-runtime exit hatch, not
built) and §4 (a third reader distinction for `refusals.log`), 0019 D1
(a crew runtime's session credential is the mint; the store of record
keeps one writer) · relates 0017 §3 (CLI-own-state), 0025 (cooperative
class), 0002 §4 (the caged credential precondition, generalized here)*

> ranger-base-eupf, 2026-09-02: "May an L1 deny cover the binary a
> runtime reads and writes its OWN credential with? Today it does, by
> construction: the gates dir is prepended on the typed line, so it leads
> the PATH of the runtime PROCESS, not only of the persona's shells."
> Three arms were offered — accept it, move L1's enforcement point onto
> the gate shell, or carve the binary out by service name. This page
> takes the first, and says why it was never the accident it looked like.

## Context

**What the wall does today.** ADR 0002 §3 types `PATH=<gates>/bin:$PATH`
in front of the runtime command. Every process in the pane inherits it —
the persona's tool shells, and the runtime itself. Claude Code on darwin
authenticates through the keychain CLI, exec'd by bare name (measured on
the 2.1.258 release artifact, ranger-base-eupf: the find verb for the
read, the item removal, an argv write, and the CLI's own stdin batch mode
as the primary write path). `Runtime.CredBin` declares that binary;
every crew PID denies it; so every crew launch shims the runtime's own
credential read and refuses it.

**What the refusals actually are.** MEASURED 2026-09-02 over the
thirteen gate dirs on the reference box: 10,279 refusal lines, of which
eleven launched personas account for all but two. The runtime's own read
forms — the find verb on the runtime's item, its legacy-named twin, and
the config-dir-hashed variants — are 4,600 of them, plus 9 refused item
removals. Fewer than thirty lines are persona-shaped (`help`, a keychain
info query, one bare invocation, and the three batch-mode spellings
ranger-base-eupf counted). The log is 98% the runtime asking for a token,
about twice an hour per session.

**What the runtime holds instead.** The bead framed the session's
credential as "frozen at whatever it started with". It is not frozen; it
was never the keychain's. MEASURED 2026-09-02 on this session's own
runtime process (`ps -E` on the pane's `claude`): the process environment
carries the session mint under the name `CageCredential` returns for
claude — the same key the caged precondition of 0002 §4 requires — and
the Bash tool's child environment does not (the runtime scrubs it, along
with every other `*_KEY`/`*_TOKEN`-shaped name, before spawning a tool
shell). All eleven crew PIDs name an env set (`envs:`), and
ranger-base-wkai3 records the operator's own account: a permanent
session token minted by hand into the default env set "ran the whole
crew all day" on 2026-08-31 while the meter, which reads the rotating
keychain token, went blind for nineteen hours and no session noticed.

**What a refused read does inside the runtime.** ADR 0019 D2 measured
the composite store's rules in the release binary: the find verb is
tried first; exit 0-with-nothing, 36 and 44 are *null* and fall through
to the plaintext file under the runtime's config dir (`CredentialsFile()`);
*any other exit is a read failure and the strict read does not fall
through*. The shim exits 1. So a shimmed runtime never reads the file
either: with a mint in its environment it runs on the mint, and with no
mint it reports itself logged out — laurie's nested `claude -p` on
2026-08-29, exactly. The two halves the bead named ("logged out" and
"cannot refresh at expiry") are one fact: a crew runtime has no path to
the operator's rotating pair, in either direction.

**What that pair is protected by.** ADR 0019 (Context and "the trade,
plainly") is unambiguous: the keychain was never a wall against a
same-user read (MEASURED 2026-08-24, the non-interactive read succeeds),
and every second copy of a rotating credential is a snapshot that
disagrees with the source exactly when it matters — `default.env` held
such a copy and 401'd the fleet twice. The rotating pair is safe because
it has **one writer**: the operator's own claude, whose login loop
refreshes it. Eleven crew runtimes reading that item would be eleven
more processes entitled to refresh it — every one of them a writer the
moment its bearer token ages out. The L1 shim in front of the runtime's
read is, today, the only thing at `shims` tier standing between the crew
and that pair.

**What the deny was for.** `Bash(<keychain CLI>:*)` is the persona-facing
spelling of the visibility line: a persona's tool shell does not read the
operator's keychain. That is cooperative (ADR 0025): `/usr/bin/<bin>`
walks past every L1 shim (0009 §3), and the seatbelt's file-read deny
(ranger-base-hw18) reaches only `cage: seatbelt` PIDs (ranger-base-4lj05,
open). The rule stops the accidental and the curious, and says the line
out loud. Nothing in that purpose is about the runtime — which is why the
runtime-facing consequence read as an accident.

## Decision

**1. Yes. The deny stands over `Runtime.CredBin`, and it is the design.**
The invariant it enforces, stated for the first time: *a crew runtime
authenticates with the session mint posse injects (ADR 0019 D1, purpose
`session`, the key `CageCredential` names) and never with the operator's
store of record; the rotating pair has one writer, and it is not a crew
process.* At `shims` tier the L1 shim in front of the runtime's own
credential read is the enforcement of that invariant; at `container` the
absence of a keychain enforces it for free. ADR 0002 §3's "PATH is
prepended on the typed line" is therefore a decision about the runtime
process as much as about `path_helper` (stamped there in this commit).
Every crew PID keeps the rule.

**2. The caged precondition becomes the shimmed precondition.** ADR 0002
§4: "Auth is not a gate but it is a precondition: a caged claude launches
only with an operator-minted credential in the env set." Generalized: a
launch whose PID shims the runtime's `CredBin` — `CredGateCollision`
non-empty, which already requires the binary to resolve outside the
gates dir, so a platform where the runtime reads a file stays untouched —
launches only with that runtime's session mint (`CageCredential`) among
its env-set names, else **refuses** with the same `CheckCageCredential`
sentence family: the rule, the binary, the missing key, and the mint
recipe. Not waivable by `--allow-degraded`: this is not a degraded cage,
it is a session that cannot authenticate at all (measured: "Not logged
in"), and refusing beats spending the launch. With the mint present the
launch **says nothing** — the ranger-base-eupf warning is retired, because
both of its sentences are now false: nothing is frozen, and "drop the
rule from this PID" is the one move this page forbids.

**3. The shim's refusal exit is load-bearing and is pinned.** The
rendered `CredBin` shim must exit a code the runtime's composite treats
as a *read failure*, never one of its *null* codes (0, 36, 44). A null
exit would send the runtime to the plaintext fallback file — a stale S2
file on a healthy box is the 2026-08-24 misdiagnosis class with a new
sentence — and at refresh time the composite's update rule turns a
refused keychain write into a plaintext write of the fresh token
(ASSUMED from the measured rule: non-transient write failure → write the
file). Today the exit is 1 because every shim's is; after this page it is
1 because a test execs the rendered shim with the runtime's read argv and
reads the code.

**4. A third reader distinction for `refusals.log` (0009 §4).** A line
whose argv is the runtime's own credential read is *the runtime asking
for a credential this page says it may not have* — not the model's
intent, not the operator's rc. No mechanism: the shim cannot tell the
runtime from a persona typing the same read (below, alternatives 3 and
7), so the distinction is a rule for readers, and any census that counts
refusals as intent subtracts `CredBin`'s read forms first. The lines keep
landing (about two an hour per session; ten thousand in ten days is not
a size problem) because a second file would misfile a persona's keychain
use under "runtime".

**5. Nested CLIs authenticate with the session's mint, never by taking
the wall down.** The crew's working recipe for a nested `claude -p` in a
probe is to strip the gates dir from PATH for the child — which removes
every shim from that subtree, not one. The sanctioned shape is to hand
the child the mint the session itself runs on, explicitly on its line
(the runtime scrubbed it from the tool shell's env; the persona can read
its own env set, same uid, mode 600 — 0019: "the secrecy wall was never
file mode"). No new export: a second env name the runtime does not scrub
would put the mint into every child's `env` output, and the scrub is
there on purpose. A NOTE in the memory that taught the strip, not a
bead.

**6. The exit hatch, named and not built.** A runtime with no
env-injected credential path — one that can *only* authenticate by
reading a host store — gets the bead's arm 2 as a per-runtime
declaration in the shape of `NoGateShell`: the typed line carries
`SHELL=<gate shell>` and no PATH prepend, so the wrapper's guard (0009 §1)
walls every command the runtime runs through a shell and the runtime's
own execs walk free. Its price is stated at 0009 §3: it holds only for a
runtime that honours `SHELL` for every command it runs on the persona's
behalf (claude's Bash tool does, MEASURED today: the tool shell is a
direct child of the runtime running the wrapper's guard), and the
runtime's non-shell execs leave the wall — hooks were observed spawning
as direct `bash` children of the runtime; MCP stdio servers ASSUMED the
same. No runtime in the table needs it; claude has the mint.

## Consequences

- **Launch**: `planLaunch` swaps the warning for the precondition (D2);
  `CredGateWarning` goes; `CheckCageCredential` gains a caller at every
  tier. Two crew PIDs on the reference box carry `cage: seatbelt`; all
  eleven carry `envs:` with the mint, so no live launch changes outcome —
  the change is that a PID that drops its `envs:` now refuses instead of
  starting a logged-out session.
- **Accepted cost, stated**: inside a crew session every keychain-keyed
  feature of the runtime is off — the nine refused item removals are the
  runtime's legacy-item cleanup, and anything else the runtime keeps in
  that store (MCP OAuth persistence, ASSUMED) does not persist. MEASURED
  against it: sessions run for days with every read refused.
- **The session's only credential clock is the mint's.** 0019 D5's timer
  surfaces carry session mints only where the env set carries stamps; the
  default set carries none today, so posse answers "cannot tell" — that
  is ranger-base-wkai3's thread (monica), not reopened here. At expiry
  the read is refused, nothing falls through, and the 401 is loud: the
  actuator 0019 D5 chose.
- **The file guard is unchanged.** hw18's seatbelt file-read deny and its
  reach (ranger-base-4lj05) are the operator's open call; this page
  removes one *producer* of that file at `shims` (a crew runtime never
  refreshes, so never writes it) and leaves the guard where it is.
- **hoover's stake**: no exposure widens. Arm 3 is rejected; the
  persona's cooperative bypass of the shim is unchanged and already named
  in 0009 §3.
- **Docs**: 0002 §3, 0009 §3–4 and 0019 D1 are stamped in this commit;
  the memory that taught the PATH strip is rewritten to D5.

## Alternatives rejected

1. **Arm 2 — move L1's enforcement point onto the gate shell for claude**
   (the clever one, and the one I wanted). It does not fix the symptom
   that motivated it: the persona's subtree stays walled by the wrapper
   — correctly — so a nested `claude` is still refused. It puts eleven
   runtimes onto the operator's rotating pair at the moment the mint
   expires or the runtime's precedence changes (env-first is ASSUMED,
   unmeasured, and the store is unreadable today so it never arose). It
   leaves codex and every `NoGateShell` runtime needing the prepend
   regardless. And it opens a fourth exec channel (hooks, MCP servers) —
   dearer than the `/usr/bin/<verb>` hole that exists, so not a new class,
   but a channel for nothing. Thirty-six test files touch the typed line.
   Kept as the hatch (D6) for a runtime that has no mint path.
2. **Arm 3 — carve the binary out by service name.** ranger-base-eupf's
   measurement ends it: the write goes through the stdin batch mode,
   which an argv matcher cannot see, so any carve-out that keeps refresh
   working is no deny. A read-only carve-out hands the persona the
   operator's live token on one typed line and buys the session nothing
   it uses.
3. **Discriminate by parent process in the shim.** `exec <bin> …` from
   the tool shell makes the shim's parent the runtime; defeated for free.
   The env-marker variant is the same shape (the persona owns its env).
4. **Drop the rule from crew PIDs** — the retired warning's own advice.
   It frees the persona's typed read of the operator's store, the one
   thing the rule is for, and puts runtimes on the rotating pair (1).
5. **Exit a null code from the shim so the runtime "falls through".** It
   falls through to a stale plaintext file (S2) on a healthy box — the
   2026-08-24 misdiagnosis with a new sentence — and makes every refused
   refresh write a fresh token to that file. D3 pins the opposite.
6. **Re-key the runtime's item by config dir so the read misses.** The
   read still execs the binary and is still refused; and on a genuine
   miss (44) the composite reads the file. Nothing gained.
7. **A second log file for the `CredBin` shim.** The shim cannot tell the
   runtime from a persona typing the same read, so a split by verb would
   file a persona's keychain use under "runtime" — a worse audit than a
   subtraction rule (D4).
8. **Refuse mint-less launches fleet-wide, operator sessions included.**
   No persona, no deny, no collision; the operator's own claude is the
   pair's one writer and must stay able to read it.

## Verification (QA's checklist)

1. A PID carrying the deny, launched with an env set carrying the mint:
   the launch's warn stream carries no credential-collision line (the
   existing launch pin's polarity flips; its control arm becomes the
   refusal below).
2. The same PID with an env set lacking the mint: the launch refuses,
   naming the rule, the binary and the key, and creates no session.
3. The rendered `CredBin` shim, exec'd with the runtime's read argv, exits
   a code outside {0, 36, 44}; the mutant that exits 44 is red.
4. Live, on any crew session: `ps -E` on the pane's runtime process shows
   the mint's name; the same persona's `refusals.log` carries the read
   refusals; the session is authenticated. The three together are the
   state this page describes.
5. A refusals census that subtracts `CredBin`'s read forms leaves only
   persona-shaped lines (fewer than thirty on the reference box today).

## Measured versus assumed

| claim | status | source |
|---|---|---|
| the gates dir leads the runtime process's PATH | MEASURED | 0002 §3 typed line; `ps -E` on this session's runtime, 2026-09-02 |
| the runtime execs the keychain CLI by bare name, four forms, stdin batch as the primary write | MEASURED | ranger-base-eupf on the 2.1.258 release artifact |
| 10,279 refusal lines; ~4,600 runtime reads; <30 persona-shaped | MEASURED | thirteen gate dirs, 2026-09-02 |
| the session runtime carries the mint; the tool shell does not | MEASURED | `ps -E` versus the Bash tool's `env`, this session, 2026-09-02 |
| all eleven crew PIDs name an env set with the mint | MEASURED | `envs:` present in 11/11; ranger-base-wkai3 for the key's provenance |
| a refused read is a read failure, not null; no fallthrough | MEASURED rule (0019 D2) + shim exit 1 read from the render; consequence observed as "Not logged in" (2026-08-29) | ADR 0019, ranger-base-eupf |
| a refused keychain write relocates the token to the plaintext file | ASSUMED from the measured update rule; not observed | ADR 0019 D2 |
| env mint wins over a readable keychain item | ASSUMED, unmeasured; irrelevant while the item is unreadable | this page, alt. 1 |
| claude's Bash tool spawns `$SHELL` for every tool command | MEASURED | process ancestry of this tool shell, 2026-09-02 |
| hooks and MCP servers spawn without `$SHELL` | hooks OBSERVED as direct `bash` children; MCP ASSUMED | 120 s process sampler, 2026-09-02 |
| the mint is a permanent token, not a copy of the rotating one | ASSUMED from the operator's account on ranger-base-wkai3 and from days of authenticated sessions; the value was not read | ranger-base-wkai3 |
