# ADR 0052 — A managed hooks path is a wall posse does not touch: L3 is realized by a per-session hooks dir the session env aims git at, chained into the managed one

*Status: accepted 2026-09-02 · owner: architect · amends ADR 0023 §3
("foreign is degraded") · from ranger-base-yt6m0 (M2 criterion 8, the
operator's finding on the cold install, ranger-base-26cd)*

## Authority

[ADR 0002 §3](0002-runtimes-and-gates.md) is the sole definition of L3 realization (identity and own-render behavior) and §4 defines enforcement class. This record owns only managed-path classification, session redirection and complete forwarding. Its proof obligations below apply that contract.

## Context

A managed box sets `core.hooksPath` to an absolute, root-owned directory
outside every repo, and every git on the box dispatches its hooks from
there. `posse gates install-hooks` fails on that box with `open
<dir>/pre-push: permission denied` — not while fingerprinting anything:
`installHook` swallows the read error on the missing slot and falls through
to `os.WriteFile`, which is the create that is refused (gates.go:1804–1900,
read). Session create makes the same call best-effort, installs nothing, and
the identity probe (ADR 0023) then reads the managed slot as foreign — or
absent — and degrades both L3 lines. A dispatched launch refuses on that
(`herdrback.go` install-then-probe, `!o.AllowDegraded`), and a standing
`--allow-degraded` is the habitual waiver criterion 5 forbids. So on that
box nothing dispatches.

The constraint the bead sets, kept verbatim: the employer's control is not
fought, bypassed, or chained behind silently. Posse must not write in that
directory, must never cause the managed hook to be skipped, and must say
what it did on every launch.

Which L3 guards apply there (the bead's question c): all of them. The
instance has two seats, so the shared-index arm applies; its repos are
`private`, so the visibility arm applies; the constitution lives in a git
repo, so the constitution-path arm applies; and the PIDs deny `git push`,
so the pre-push wall applies. The minimum realizer is the whole render —
which is the argument for reusing the render rather than rewriting the
guards somewhere else.

Three facts measured 2026-09-02 on this host (git 2.50.1, scratch repo, a
mode-0555 "managed" dir named by a global `core.hooksPath`), the whole
design rests on them:

| # | measurement | result |
|---|---|---|
| M1 | `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=<mine> git rev-parse --git-path hooks` | `<mine>` — the env wins over the global managed value; `-c` the same |
| M2 | `/usr/bin/git commit -- f` under that env, `<mine>/prepare-commit-msg` exec-ing the managed one | ours ran, then the managed one ran — the redirect survives an absolute-path git, and the forward hands the managed hook git's argv |
| M3 | same commit under `env -i` | managed hooks ran alone — the redirect is shed with the environment, the managed control is not |
| M4 | `<mine>` with no `pre-commit`, managed dir with one | the managed `pre-commit` did **not** run — a slot the redirect dir lacks is skipped, so the forward set must be complete |

## Decision

**1. Classify before touching. A hooks path is MANAGED when all three
hold:** git's dispatch path (`hooksDir`, the `--git-path hooks` answer) is
absolute; it is not under the repo's common git dir; and the uid cannot
create a file in it — measured by one create probe of a dot-file in that
directory, never by opening a slot. `install-hooks`, session create's
install step, and the `hookfresh` sweep each ask this first. Managed means:
write nothing, chain nothing there, and print one line —
`L3: managed hooks path <dir> (owner <uid>, mode <perm>) — posse's wall is
not installed there; realized by session redirect (ADR 0052)`. Any subset
of the three (a writable foreign hook, a relative `core.hooksPath`, a
hooks dir inside the repo) keeps today's behaviour: the chain prescription
and a degraded launch.

**2. The realizer is a hooks dir posse owns, per session, aimed by the
session env.** At launch, when the session's repo is managed, the launcher
renders `<RHQ_HOME>/state/hooks/<session>/`:

- `posse-prepare-commit-msg` — the current `CommitGuardHook` render for the
  session's repo; `posse-pre-push` when the PID denies `git push`. The same
  bytes `install-hooks` would have written into `.git/hooks`.
- one dispatcher per slot in {posse's slots} ∪ {every executable regular
  file in the managed dir at render time}. A slot with a posse member is the
  INSTALL.md §9 chain form (`chainRender`) with the neighbour spelled as the
  managed dir's absolute path: ours runs first, its exit is final, then
  `exec <managed>/<slot> "$@"` with git's own argv and stdin. A slot with
  no posse member is the forward alone. M4 is why the set is the union: a
  missing dispatcher is a skipped employer hook.

The session env carries the redirect as git's own config-in-env form:
`GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`/`GIT_CONFIG_VALUE_n` naming
`core.hooksPath=<that dir>`, appended after any count already in the
operator's env, never clobbering it. Set beside `RHQ_PERSONA` in the launch
vars (`herdrback.go` ~1803). The dir is rendered fresh every launch and
removed when the session is retired, so hook freshness holds by
construction and the sweep has nothing to measure for a managed repo.

**3. The probe moves with the dispatch path.** In redirect mode
`probeL3Hooks` checks identity at the session dir — members byte-equal to
the renders, dispatchers byte-equal to the absolute-neighbour chain form,
all `+x` — plus one new arm, forward completeness: every executable in the
managed dir has a dispatcher, else the slot degrades as
`managed hook <slot> not forwarded — re-launch to re-render`. The behaviour
half (`execOwnRenders`) is unchanged. Parity prints
`(render probed, dispatch verified — session hooks dir, redirected by env;
managed hooks <dir> run after ours)`; the launch is not degraded; class
stays Cooperative (ADR 0025). Session meta records `hooks_mode: redirect`
and `managed_hooks: <dir>`.

*(Amended 2026-09-03, ranger-base-buvq4, and 2026-09-04, ranger-base-m6szh:
a managed dir the RECORD cannot carry refuses the launch, before a
workspace, a render or a record exists. The meta is flat YAML whose reader
stops at the first newline — a path's tail would read back as meta lines of
its own, `crew: true` among them — and also cuts the value at `" #"`,
strips a wrapping pair of double quotes, trims it, and reads `~`/`null` as
unset. git accepts every one of those in `core.hooksPath`; posse cannot
record them, and no encoding fixes it, because the comment cut runs before
the quotes come off. So the guard asks the reader itself
(`flatScalarRoundTrip`) and refuses what would read back as a different
path, naming both. Only `" #"` is reachable through a real dir today —
gitPathRaw trims git's output, so a trailing blank never arrives — and the
guard is written against the reader rather than that list, because
reachability is two other files' property. **MEASURED** 2026-09-04, git
2.50.1.)*

**4. Reach, stated honestly.** Env-borne means the wall keeps the
absolute-path reach a file install had (M2) and loses to an emptied
environment (M3) — the class ADR 0025 already assigned L3, since the hooks
read `RHQ_PERSONA` from that same environment. The one thing a persona can
do here that it could not before is drop posse's hook by dropping the env;
doing so leaves the employer's hook running (M3). Nothing a persona or
posse does weakens the employer's control — that is the property, and it
is the reason env was chosen over the repo-local override below.

**5. Apply ADR 0002 §3:** foreign identity degrades, named — except a
MANAGED path, which is not foreign but unwritable, and is realized by this
redirect, named on the launch line either way. The bd shim needs no
answer: `bd hooks install` fails on the same box the same way, bd 0.50.3
flushes per write (measured 2026-09-02, on the bead), and the launcher's
jsonl commit runs in the launcher's env — managed hooks only, as the
operator's own commits do.

## Consequences

- Criterion 7 unblocks without a waiver: a managed repo launches with L3
  realized and says how. Criterion 5 runs without `--allow-degraded`.
- The employer's hooks run on every commit and push a posse session makes,
  after ours, with the argv and stdin git gave the slot. Never fewer of
  them than today; the audit trail is the launch line, session meta, and
  the rendered dir, all readable.
- One render per session, keyed to the session's repo: the member runs in
  every repo the session touches (as the L1 shim does), with the session
  repo's visibility class. On a private instance that is exact. On a
  mixed-class instance a commit into a wider-class repo is over-refused
  (never under); the exit is per-repo member subdirs, filed only if it
  bites.
- A slot the managed tool adds mid-session is unforwarded until the next
  launch. Bounded by one session; named in the probe's completeness arm.
- Non-managed repos are untouched: every existing pin over `.git/hooks`
  keeps its meaning.

## Alternatives rejected

- **Repo-local `core.hooksPath` override** (the clever one). Local config
  outranks global, so writing `core.hooksPath=<posse chain dir>` into the
  repo's `.git/config` would reach every git in that repo, the operator's
  own included, absolute path and `env -i` alike. Rejected: it rewrites the
  effective value of an employer-managed setting, which a compliance check
  reading `git config core.hooksPath` cannot tell from tampering, and it
  changes the operator's own commits on a work box they did not ask posse
  to guard. The env form is scoped to posse's sessions and leaves the
  operator's git exactly as the employer configured it. Precedence
  ASSUMED from git-config(1); cheap to measure, deliberately not built.
- **Reimplement the guards in the gate shell's git shim** (the bead's (b)
  as sketched). The arms read what git hands the hook — `GIT_INDEX_FILE`,
  the staged tree, the message file — which a PATH shim running before git
  must re-derive; that is a second implementation of one invariant, and it
  sits behind the layer an absolute path walks past, which is the reach
  the hook existed to add. The redirect keeps the render, the identity
  probe, and the freshness doctrine, and adds one config fact.
- **Fingerprint the managed hook before chaining** (the bead's (a)). Not a
  trust decision: ours runs first and its exit is final, so the managed
  hook can only refuse more, and it runs in the operator's context today
  with or without posse. Its identity is the employer's business; posse
  logs its path and forwards.
- **Chain in the managed dir by privilege, or ask for posse's hook to be
  added there.** Fights the control (the bead's line) and produces an
  operator-owned copy nothing re-renders.
- **A standing `--allow-degraded`.** The habitual waiver criterion 5
  forbids; also unavailable at tier fast (ADR 0003 §3).
- **Hardcode githooks(5) for the forward set.** Replaced by enumerating the
  managed dir at render: no list to go stale under a new git, and a new
  managed hook is forwarded from the next launch.

## Verification (laurie's checklist; each is a pin)

1. Classification: a mode-0555 dir named by `GIT_CONFIG_GLOBAL`
   `core.hooksPath` → `install-hooks` writes nothing, prints the managed
   line, and no `permission denied` appears. The dir's listing and mtimes
   are identical before and after every step below.
2. Redirect reach: under the session env, `/usr/bin/git commit -- f` runs
   `posse-prepare-commit-msg` then the managed `prepare-commit-msg` (M2).
3. Forward completeness: a managed `pre-commit` dropping a canary runs
   under the redirect; delete its dispatcher → the probe degrades that slot
   and names it.
4. Ours first, exit final: posse member refuses → managed hook does not
   run, git reports the refusal.
5. Identity: edit one byte of a session-dir member → degraded; the
   dispatcher with a relative neighbour is not accepted in redirect mode.
6. `env -i` commit: managed hooks run alone (M3); no posse line.
7. Existing hook pins (3c3, flz7, xo65, q32o, hd56) green unchanged.

## Measured versus assumed

MEASURED (this host, 2026-09-02): M1–M4 above; the failing call is the
create in `installHook`, by reading. ASSUMED: local-over-global precedence
(rejected alternative, from docs); `GIT_CONFIG_COUNT` appending when the
operator's env already carries one (git ≥ 2.31 documents the form; the
append is pinned by the build); the managed box's dir holds only executable
regular files worth forwarding (the render forwards exactly those and
prints what it skipped).
