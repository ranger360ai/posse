# A launch out of a non-owning home leaves the wall and takes the redirect (ranger-base-lh9nj)

ADR 0081 D1 and D3, implemented 2026-11-07, on top of D2's stamp
(ranger-base-b87ej). The launch's unmanaged hooks arm no longer rewrites a
shared commit wall that another home rendered and this home does not own; it
renders its own wall per session instead and forwards into the shared slot, so
the session is held by BOTH walls.

## The rule is one question asked of the pre-heal probe

D3 lists four conditions under which the launch writes. They are four states of
one verdict the launch had already read, so the rule is a switch over it rather
than four file reads (`sharedCommitWallWrite`, gates.go):

| pre-heal verdict | D3 | write? |
|---|---|---|
| `l3Uninstalled`, `l3Foreign`, `l3Displaced`, `l3MemberGone` | case 1 | yes |
| `l3Held` | case 3 | yes |
| `l3Stale` (our stamp, or none at all) | cases 3, 4 | yes |
| `l3OtherHome` | case 2 | only if this home owns the repo |

That it reuses the PRE-HEAL probe (`probeL3Hooks` before the install,
ranger-base-2mogn) is the load-bearing part: the decision and the line the
launch prints about it are then the same reading of the same file. A second
read at decision time could answer differently from the warning.

The one config question — D1, ownership — is `OwnsCommitWall`: this home's
`beads_visibility:` marks `hookRepo(dir)`, meaning `BeadsVisibility` returns a
source other than `VisibilityUnmarkedSource`. `hookRepo` and not `dir`, for the
reason every other caller in that file uses it: the file lands in the COMMON
hooks dir, and a `beads_visibility:` key names a repo the operator declared, so
a worktree path could only ever fall through to unmarked — which would read
every dispatched launch on this box as a launch into a repo this home does not
own.

## What the non-owner gets instead

ADR 0052 D2's per-session dir, with the repo's own hooks dir as the forward
target. `RenderSessionHooks` was already a function of "a dir posse may not
write"; what it gained is the REASON (`l3Forward.Reason`, `l3RedirectReason`),
because two different facts now produce the same render:

- `redirect` — a managed dispatch path (ADR 0052 D1). The far side is an
  employer's directory and the operator's own effective `core.hooksPath` names
  it.
- `foreign-owner` — a shared slot stamped by the home that owns the repo
  (ADR 0081 D3). The far side is `.git/hooks`, which no `core.hooksPath` names.

The reason is the vocabulary, spelled once: the session record's `hooks_mode`
IS that string, so the launch and the record cannot disagree. `managed_hooks`
stays EMPTY for a foreign-owner redirect — there is no employer's directory
here, and that key is the record's word for one (the ADR's own Consequences
warn that a reader would otherwise treat the shared dir as an employer's). It
also keeps the flat-YAML mangling surface where it was: nothing new reaches
`writeMeta` from a path posse did not make.

Two readers needed the reason rather than the two paths:

- the parity line that says whose hooks run after ours (`applyL3Probe`). "managed
  hooks ~/repo/.git/hooks run after ours" would send an operator looking for a
  `core.hooksPath` nobody set.
- the managed-hook config comparison (`compareGitHookViews`). It faulted any
  redirect whose forward target was not the operator's effective
  `core.hooksPath`, and an unset one read as `(unset)` — so the one arrangement
  that loses the operator nothing at all would have printed RED on every
  foreign-owner launch. It now compares against git's own dispatch path for
  the repo (`DefaultHooks`) when no scope carries the setting, and says which
  of the two it compared against.

## `.sample` is not a slot (the one thing that changed for the managed arm too)

Making a repo's own `.git/hooks` a forward target put `git init`'s template
dir on the enumerator. `forwardableSlots` now counts `.sample` files out before
the stat arms, in ONE skip line rather than one per file: git's hook lookup is
`find_hook(<the exact slot name>)`, so no slot it dispatches ends in `.sample`,
and a dispatcher plus a report line for each of the dozen-odd samples would be
the launch's whole report. Counted and said, never silently dropped — what that
file may not do is step over something git WOULD run without saying so.

## At the container tier: no redirect, and the launch is refused

D2's own exclusion. The env would cross the cage boundary and the dir it names
is not on the mount list, so a `core.hooksPath` pointing at it would skip every
hook in the repo — the owner's included. So a caged launch into a repo whose
wall belongs to another home runs under the owner's wall ALONE, says so in one
line, and then the parity check reports the truth about the slot: it is not
this home's render, so this launch's L3 commit guard is not realized, so the
launch is DEGRADED and refused without `--allow-degraded`.

That refusal is the behaviour change this bead ships, and it is the ADR's own
Consequence: an ad-hoc home pointed at a real repo can no longer change that
repo's wall for anyone else, and a scratch home needs a scratch repo. Off the
container tier the redirect means there is nothing to refuse.

## What is NOT in this bead

- **D4, `install-hooks`' refusal for an explicit target** — ranger-base-hnujh,
  which `OwnsCommitWall` is exported for. LANDED 2026-11-07: the verb asks
  `sharedCommitWallWrite` through `ForeignCommitWallOwner`, so the rule has
  one implementation and the verb is no longer the remedy that undoes the
  wall. See docs/notes.d/ranger-base-hnujh.md.
- **D5, pre-push** — unchanged and still installed into the shared dir, from
  either arm. `PrePushHook` is a constant with no home input, so a cross-home
  write produces identical bytes; there is nothing for two writers to contend
  over.
- **The queue repo's own guard** (`queuejsonl.go`, `a.InstallCommitGuardHook(q)`
  before the jsonl commit) is a THIRD cross-home writer of a shared wall and
  ADR 0081 names neither it nor a rule for it: `queue_repo:` is config this
  home set, which is near-ownership but is not `beads_visibility:`. Filed
  rather than decided here: ranger-base-knb1v (`-l architecture`,
  discovered-from this bead). The predicate already exists at that call site
  as a READING — `QueueReady().Unmarked` is exactly D1's question, rendered
  as a G13 governance row — and the write happens anyway.

## Measured

- Six new pins plus the container arm (`commitwallowner_qa_test.go`), and the
  two halves of the ADR's DONE WHEN among them: a scratch-home launch into a
  repo stamped by another home leaves that file's bytes AND mtime unchanged —
  the whole hooks dir, by `mhpSnapshot` — and a real `git commit` through a
  real git, by absolute path, runs this home's wall and then the owner's
  (`ours`, `owner`, in that order), while the same commit without the redirect
  vars runs the owner's alone.
- MUTATION-CHECKED 2026-11-07, four mutants, each red in a different arm; the
  list is in the test file's header. The one worth naming here is "ownership
  dropped": it reds the OWNER's arm, which is why `l3Verdict.reStamped()` was
  left alone — narrowing the predicate instead of the call site would have
  silenced the one re-stamp D6 says must stay visible.
