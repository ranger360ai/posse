# ADR 0081 — a repo's shared commit wall has one owning home; a launch from any other home takes its own wall instead of rewriting the shared one

*Status: accepted 2026-11-06 (ranger-base-n6dsk, from ranger-base-q0np6); D7 added 2026-11-07 (ranger-base-knb1v, from ranger-base-lh9nj) · owner: architect · amends ADR 0023 (one writer per slot is now a rule with an owner, not an assumption) and extends ADR 0052 D2 (the per-session redirect is also taken when posse MAY write the slot but this home SHOULD NOT) · builds in the code beads named on ranger-base-n6dsk (dinesh)*

## Context

A session launch ends, on an unmanaged hooks path, with
`InstallPrePushHook(dir)` + `a.InstallCommitGuardHook(dir)`
(internal/posse/herdrback.go, planLaunch). `dir` is the session worktree, whose
`--git-path hooks` is the repo's COMMON hooks dir by design (gates.go
`hooksDir`). `a` is the LAUNCHING process's home, and the commit guard is a
function of that home's config (`OpsPatternSet`, `DeriveGuardValueLiterals`,
`DeriveCrewLiterals`, `BeadsVisibility`). So any home on the box can rewrite
any repo's shared wall, and every reader of that wall (the launch probe, G12's
sweep, `scripts/verify-hook-freshness.sh`, `install-hooks`) renders it from its
own home. That is two writers on one file, with different inputs and no
ordering, so the last writer wins.

MEASURED 2026-11-06 (ranger-base-q0np6; its notes fragment lands with that
bead's branch): same binary 0.5.3+474337e8, same repo, only `RHQ_HOME` changed.
The box's home renders 1897 lines and a scratch home renders 1137. The
760-line delta is the four data-ceiling arms. Two codex seats launched from
scratch homes did this to `~/src/posse` at 22:06 and 22:11.

ADR 0023's identity-then-probe assumes one writer per slot. Nothing enforces
that. The field's answer is the **single-writer principle**: give the resource
one owner, and have everyone else route around it instead of contending on it
(Thompson, "Single Writer Principle", 2011; Helland, "Life beyond
Distributed Transactions", CIDR 2007). The shop already has a store of record for
"which repos is this home responsible for": `beads_visibility:`, which the
config itself calls "the hook wall's repo list". `BeadsVisibility` already
reports a repo the home does not list as `VisibilityUnmarkedSource`.

## Decision

**D1 — Owner.** Home H owns repo R's shared commit wall iff H's
`beads_visibility:` marks R, meaning `BeadsVisibility(hookRepo(dir))` returns a
source other than `VisibilityUnmarkedSource`. No new key and no new state.

**D2 — Stamp.** The commit-guard body (the `posse-prepare-commit-msg` member,
or the slot itself when it is unchained) carries one comment line,
`# posse-home: <absolute a.Home>`. This is how a home that does NOT own R can
tell that another home wrote the wall. It cannot read the other home's config,
but it can read the file. Readers keep comparing bytes. When the bytes differ
and the stamp names a different home, the verdict says
`rendered by home <path>`, not `ours but stale`. That is a different finding
with a different fix.

**D3 — The launch's write rule** (unmanaged arm only; the managed arm is
ADR 0052 and does not change). The launch writes the shared commit-guard slot
iff any of these holds:
1. the slot holds no posse commit guard (absent; a foreign hook is still
   refused by installHook, ADR 0002 §3);
2. H owns R (D1);
3. the installed body's stamp names H;
4. the installed body has no stamp. This is the one-shot migration of bodies
   written before this ADR.

Otherwise the launch does not touch the shared slot. On the seatbelt/shims
tiers it takes ADR 0052 D2's per-session dir, with the shared hooks dir as the
forward target. `RenderSessionHooks` already accepts any absolute dir. Its
dispatcher runs this home's wall first and then execs the shared slot, so the
session is held by BOTH walls and never by fewer than the owner's. `env -i`
sheds the redirect and leaves the owner's wall, as D2 already documents. At
the container tier there is no redirect (D2's own exclusion), so the session
runs under the owner's wall alone. Every non-write prints one line:
`posse: <session> — <repo>'s commit wall belongs to home <stamp>; not
rewritten, L3 rendered at <dir>` (or `… not rewritten; the session runs under
it`).

**D4 — install-hooks follows the same rule** for an explicit target. A repo on
the home's own list is owned (D1), so the default sweep is unaffected. A
refusal names the stamping home and both remedies: mark R in this home's
`beads_visibility:`, or install from the home that stamped it. No `--force`.
The remedy is a config line, and a flag would recreate the second writer.

**D5 — pre-push is out of scope.** `PrePushHook` is a constant with no home
input, so a cross-home write produces identical bytes. It is left as is.

**D6 — Two owners is misconfiguration, made visible, not refused.** If two
homes both mark R, both write and the last one wins. That is today's behaviour,
except that D2's stamp now makes every reader name the other home.

**D7 — The queue repo's guard write follows D3. When D3 says no, the
projection commit is refused.** `CommitQueueJSONL`
(internal/posse/queuejsonl.go) is a third writer of a shared wall. Before
every jsonl commit it calls `a.InstallCommitGuardHook(q)` with
`q = a.QueueRepo()`, from whichever home ran the close. Ownership is still D1
alone. `queue_repo:` does not confer it, because it is one line any home can
copy. The call site does what the launch does: probe first, ask D3's rule
(the same predicate the launch asks, not a second copy of it), and install
only if the rule allows. When it does not, there is no session to redirect.
The probe that already rules this commit then reads the other home's render,
and the commit is refused. The error names the stamping home and both
remedies: mark `q` in this home's `beads_visibility:` (G13's remedy and
ADR 0015 §4 cutover step 5), or close from the home that stamped it.

This costs no line the census does not already lose. A home that does not
mark `q` renders it public. In a public repo check 0 scans the projection's
added lines against the effective ops patterns, and G13 already reports that
commit as refused at every close (QueueReady's `Unmarked`). Under rule (a)
below, a non-owner would rewrite the owner's wall with its public body, and
in that common case its own commit would still be refused. D7 refuses the
same commit and leaves the owner's wall intact. The projection is a full
export, so the next close from the owning home commits what this one did not.
That is the "recoverable by the next close" the call site already relies on.

## Consequences

- Every body installed before this ADR has no stamp. Each reader's fresh
  render now differs from it, so the first launch per repo re-stamps it and
  prints the existing "found WRONG … re-stamped" line once. One
  `posse gates install-hooks` clears every listed repo.
- An ad-hoc home pointed at a real repo can no longer change that repo's wall
  for anyone else. A scratch home still needs a scratch repo to test its own
  wall in the shared slot. That is the q0np6 note's operator advice, now
  enforced.
- The session record records a redirect that has no managed path behind it.
  The meta field must say why it was taken (a `foreign-owner` reason, not
  `managed`), or the cleanup and probe readers will treat the shared dir as an
  employer's.
- Tests that pin exact commit-guard bytes gain the stamp line from their own
  scratch home.
- The stamp writes the home's absolute path into an untracked file under
  `.git/hooks`. It never enters the tree. ASSUMED: no reader copies a hook
  body into a tracked file (no such path exists today).

## Alternatives rejected

- **(a) Monotone write: refuse when the new body drops a guard the installed
  one carries.** This was the one I wanted, because it states the invariant
  directly. It covers only one direction and only one kind of difference. A
  scratch home with the same arms but different identity literals or ops
  patterns drops no guard, but it still leaves every reader of the owner's
  home reading `stale`. A home with MORE guards passes the check and does the
  same. It also needs a guard lattice: banner parsing, a registry every new
  arm must join, and an order over values. That is machinery to manage
  contention between writers when the fix is to have one writer.
- **(b) Redirect every launch whose slot is shared, owner included.** The
  owner's own launches would stop refreshing the shared wall, so only
  `install-hooks` would keep it fresh. Every ordinary seat would carry an env
  redirect and its D2 caveats (container tier, `env -i`) to fix a problem
  that only exists for non-owners. D3 takes (b) only where it is needed.
- **(c) without the stamp: refuse every shared write from an unmarked home.**
  A non-owner cannot see the owner's config, so "unmarked" is the only signal
  it has. Then a launch from the box's real home into an unlisted repo
  (`~/code/...`, `dirs:` entries) would never write or refresh a wall there.
  That loses L3 in every repo nobody listed. The stamp is what separates
  "nobody's" from "somebody else's".
- **Write the launch's hooks under a temp dir.** This disarms L3 in the repo
  every session commits into (q0np6).
- **A lock or lease file beside the slot.** It orders the two writers but
  still lets either one win. That is coordination for a fact that only needs
  an owner.
- **Stamp a hash of the config instead of the home.** That says the bodies
  differ, which the bytes already say. It cannot say whose config it is or
  what the remedy is.
- **(D7a) `queue_repo:` implies ownership of the queue repo's wall.** D1
  would gain a second source, and the second one is a key that copies
  between homes with the rest of a config. That is how a scratch home comes
  to name the real repo in the first place. The write it permits is the
  public body over the owner's private one, and check 0 then refuses the
  commit anyway, so it does damage and saves nothing.
- **(D7b) Run the queue commit under both walls, the way D3 holds a
  session.** That means a per-commit `core.hooksPath` redirect to a fresh
  render of this home's wall, which forwards to the shared slot. This home's
  wall is the public body, so wherever check 0 fires it refuses exactly as
  D7 does. It differs only when no added line matches a pattern, and there it
  lets a non-owning home write history into another home's queue repo. That
  costs a render and a temp dir per close to rescue a commit the owner's next
  close makes anyway. ASSUMED rare; not MEASURED.
- **(D7c) Accept the owner's render as armed (commit under the owner's wall
  alone).** The launcher would be acting on this home's config, which calls
  `q` public, while skipping the scan that config demands. By this home's own
  reading that is a disclosure.
