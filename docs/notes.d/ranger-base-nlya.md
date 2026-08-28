## The runtime contract walk: a live gate that stands up its own pane (ranger-base-nlya)

`make verify-runtime-walk RUNTIME=grok|codex|claude` walks ADR 0013's six
stages — launch → promptable → work → record → settle → account — on one
real runtime, and scores each with ADR 0017 §2's verdicts (MEASURED
WORKING / MEASURED BROKEN / DECLARED DIFFERENCE / UNKNOWN(failing)). It is
`internal/rhq/runtimewalk_live_test.go`, gated `RHQ_LIVE_RUNTIME=`, and it
is never in `make test`: **it spends a real turn on the runtime under
test.**

**Why it exists.** The seven `RHQ_LIVE_*` pins in `internal/rhq` all need a
human to launch a session first and point an env var at the pane
(`livesplash_test.go`'s header says so). Each is therefore a pin on one
stage of somebody else's session, and nothing in the repo ever *launched* a
grok or codex session and asked whether it still works. Both precedents for
that gap are silent ones: ranger-base-z6n (the grok splash rule matched
only a narrow pane; `verify-detection` was green because the fixtures were
narrow) and ranger-base-ocfh (`verify-grok-pin`'s `autoUpdate` sed reads
pretty-printed JSON as "offline" and passes blind). Green check, broken
thing — the class a fixture test cannot catch.

**It drives production, not a copy.** `Dispatcher.fire` → `launchSession` →
`CreateSession{Worktree, PromptFile}` → `awaitAgent` → `claim` →
`Dispatcher.gather`, with the real `Bd` and a real herdr. If dispatch
breaks, the walk breaks; that property is the only thing that makes the
tokens worth spending.

### The fixture recipe, which is the reusable part

| piece | how, and why not the obvious way |
|---|---|
| herdr | its **own** scratch server (`qa-walk-<pid>-<stamp>`), started with a cleaned env and torn down after. Not the fleet server: a fixture session must not appear on the operator's board, and a teardown that reached the wrong one would close a real workspace. |
| RHQ_HOME | `t.TempDir()`. Nothing in `~/.config/rhq` is read or written, so ADR 0015's promotion gate is not in the way. |
| PID | a throwaway `walkfixture` written into that scratch home. No operator PID is borrowed. |
| bd store | a **copy** of a real `.beads`, reached through a real `<repo>/.beads/redirect`. |
| worktrees | config `worktrees:` moved to `~/.posse/qa-runtimewalk/<stamp>/` and removed after — never the live `~/.posse/worktrees` (ranger-base-gvrh). |

The store deserves its own paragraph, because it was the bead's open
question. A plain scratch db does not exercise the `.beads` redirect, which
is where codex broke before; the live queue must not be dirtied by a
fixture; and `bd init` is denied to fleet personas, which a test may not
launder by shelling out to it. Copying a real store — by name, never
wholesale, because a socket cannot be copied and a `daemon.pid` copied out
of a live store would point the fixture at the operator's daemon — gives a
real bd on the real schema behind a real redirect, and writes nothing the
fleet reads. Measured 2026-08-28: `bd` refuses a `BEADS_DIR` under `/tmp`
but accepts one under `/var/folders` (what `t.TempDir()` hands out), so the
whole fixture fits in a temp dir.

### Four things that each cost a run before the gate measured anything

1. **`herdr … server` is a foreground process, not a daemon.** `cmd.Run()`
   never returns and `cmd.CombinedOutput()` is worse (it blocks on a pipe
   that never sees EOF). `Start()`, poll for the socket, kill in cleanup.
2. **`prompt: argv` and `prompt: typed` have different observables.** Only
   the argv path writes `state/prompts/<session>.txt`; the typed path's
   observable is that `awaitAgent` reached a screen herdr could NAME before
   anything was typed. Asserting the file on both scored the claude
   baseline MEASURED BROKEN for a property claude never claimed.
3. **The launching persona's environment reaches the session under test.**
   The scratch herdr server is the seam: every pane it opens inherits its
   env. `HERDR_*`, `CLAUDECODE`/`CLAUDE_CODE_*`, the persona's `RHQ_*`
   (gates, deny list, skills, home) and every `/state/gates/` entry on
   `PATH` are stripped before the server starts.
4. **…and the env set it needs does NOT reach it by itself.** A fleet PID
   carries `envs: [default]`, and that set is where
   `CLAUDE_CODE_OAUTH_TOKEN` comes from. A fixture PID without it launches
   a claude that authenticates from `~/.claude` alone: measured twice on
   2026-08-28, `401 OAuth access token has expired · Please run /login`,
   then a settle to idle having done nothing — which is exactly the shape
   a finished turn has, and is invisible unless the pane is read. The walk
   now symlinks the operator's `envs/` into its scratch home (never a
   copy — ADR 0019) and gives the fixture PID `envs: [default]`.

Two of those generalise past this gate. **A live test launched from inside
a persona pane measures a session wearing that persona's enforcement**
unless it says otherwise — and **a launched session that cannot
authenticate settles to idle exactly like one that finished**, so any gate
that judges a session by its settle alone will read "could not log in" as
"did the work".

### The account cell, and why it is its own dimension

The architect's scoping (ranger-base-il14): an exhausted allotment must be
distinguishable from a broken runtime, because reading one as the other
cost the shop a morning. The walk therefore buys the cheapest turn the CLI
sells (`grok -p`, `codex exec`, `claude -p`) *before* it launches anything,
and stops on a provider's refusal with UNKNOWN(failing) rather than
reporting six broken stages that are really one unpaid bill.

Two directions, both pinned hermetically in `runtimewalk_qa_test.go`:

- a probe that did not come back is read for the provider's own exhaustion
  words;
- **a probe that DID come back is alive even if its answer contains those
  words** — checking the words first would report a healthy account as an
  unpaid bill and refuse to run on a working box.

And the probe is not enough on its own: it authenticates from the shell's
environment, and a *launched* session does not always get the same
credentials. So a record miss also reads the pane for an auth failure and
scores UNKNOWN(failing) — the session could not log in, so nothing about
the runtime was measured.

### What a run leaves behind

A few `projects[<a fixture temp dir>]` entries in the operator's
`~/.claude.json` on the claude arm — measured 2026-08-28 at 2–3 per run,
one per directory the launch touches. `SeedClaudeTrust` is what makes a
fresh directory promptable, and diverting it to a scratch file turns the
baseline into a trust dialog nobody answers. The walk deliberately does NOT
delete them again: that file is a lost-update race with every live claude
session (ranger-base-5qnt), and a fixture must not take that risk with the
operator's config to tidy up after itself. codex and grok never reach the
seed at all.

Everything else is removed, and the teardown says so on the sheet: the
scratch herdr session is stopped and deleted, the worktree root is removed,
and no bd daemon is left — asserted both by the fixture store's own
`daemon.pid` and by a `pgrep` delta, since either alone is blind
(ranger-base-42mv, where a live test leaked two).

### Sheets, 2026-08-28

- **claude** — 9/9 MEASURED WORKING, 64s. The baseline arm is what makes
  the rest legible: the session took the typed prompt, worked, and CLOSED
  its bead with a comment through the redirect. Without it, `record`'s
  working arm is a branch nothing has ever taken.
- **codex** — launch, promptable (argv, 3238-byte prompt on the launch
  line), work and settle MEASURED WORKING in 50s; `record` DECLARED
  DIFFERENCE (it commented and left the bead `in_progress`, which is the
  fourth measurement of ranger-base-0fb's degrade and the reason
  `record: untrusted` is the honest declaration); `account` DECLARED
  DIFFERENCE (uncounted).
- **grok** — UNKNOWN(failing) at the account cell in 2.9s, on the
  provider's own `402 Payment Required`, and the walk stopped there: no
  session launched, nothing spent, and no cell blamed on the runtime.

An hour later a re-run of the codex arm stopped at `preflight`
UNKNOWN(failing): codex 0.150.1 had been published and its update menu came
back (`dismissed_version 0.149.1` vs `latest_version 0.150.1`), which is a
launch the operator has to silence and not a runtime finding. That is the
gate doing the job it was written for, on a degrade that arrived while it
was being written.

### What it does not cover

Only one runtime per run, and only the dispatch shape: the cage tiers,
egress, skills materialisation and the gate wall are other lanes'
measurements (ranger-base-25fj et al.). `record` on a `record: untrusted`
runtime is scored DECLARED DIFFERENCE and never fails — a codex session
that *closes* its bead would show up as MEASURED WORKING, which is the
measurement that would promote it.
