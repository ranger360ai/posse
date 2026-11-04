## The wall that refused the reads it was never built to stop (ranger-base-ap4p6)

*2026-11-04 · from ranger-base-b1ttk, raised by gilfoyle · code: `internal/posse/hookskip.go`, `internal/posse/hookskip_qa_test.go`*

The L1 git shim's hook-skip arm (ranger-base-r3pri, ADR 0052) refused
`-c core.hooksPath=…` / `-c hook.*` / `-c include*` on **every** verb. The
preamble said why: judging the flag alone costs nothing and closes
`git -c core.hooksPath=x <alias>`, whose verb no list can name. Both halves of
that were true. What it did not account for is that the Claude Code harness
types that exact prefix in front of the git calls it makes for its own
context — `-c core.hooksPath=/dev/null` plus
`-c hook.<every-hook>.enabled=false`, on `status`, `rev-parse`,
`remote get-url`, `config --get-regexp`, `ls-files`, `update-index` and
`write-tree`. So the first seat on the new binary lost its branch/status
snapshot, and so did every seat after it.

**MEASURED 2026-11-03/04**, `refusals.log` for seven personas under
`<StateDir>/gates`, every `hookskip:config-override` line: 51
`remote get-url origin`, 51 bare `remote`, 43 `status --porcelain`, 43
`rev-parse --abbrev-ref HEAD`, 43 `config -z --get-regexp ^(remote\..+…)$`, 8
`ls-files --error-unmatch`, 7 each of `write-tree`,
`update-index --untracked-cache`, `ls-files -z --stage` and
`config --get --path core.excludesFile`, plus `log`,
`worktree list --porcelain -z` and a bare `--version`. **46 distinct argv
lines, 0 writes, 0 commits, 0 pushes.** Not one of them could skip a hook,
because not one of them runs one.

### What changed

The `-c` scan still records on the flag; the **refusal now waits for the
verb**, because whether an override skips a hook is a fact about the verb.
`hookSkipReadVerbs` is the allowlist, `hookSkipReadSubverbs` narrows two
verbs to their read subcommands, and `posse_hs_readverb` is the rendered sh,
built in sorted key order so the shim is byte-identical run to run. Everything not on the list — **including an unknown verb, which is what
an alias looks like from inside the shim** — is still refused, so the hole
the "any verb" arm was built for stays shut. `git` with no verb at all
(`git --version`, `git -c core.hooksPath=x`) runs nothing and is allowed.

### The membership rule, so the next verb is judged and not guessed

A verb belongs on the allowlist when it **(a) performs no reference update**
and **(b) runs no hook whose nonzero exit vetoes anything.**

Rule (b) needed measuring, because three hooks do fire on verbs now on the
list. **MEASURED, githooks(5), git 2.50.1:**

- `fsmonitor-watchman` — a query helper git reads an answer *from*. Not a gate.
- `post-index-change` — "invoked when the index is written in read-cache.c
  `do_write_locked_index`". It runs after the write. Not a gate.
- `reference-transaction` — the one that **can** veto: "in the 'prepared'
  state, a non-zero exit status will cause the transaction to be aborted."
  It needs a reference update, which rule (a) already excludes.

That is why `checkout`, `switch`, `reset`, `fetch`, `tag`, `branch`,
`update-ref`, `symbolic-ref`, `stash`, `reflog`, `gc` and `submodule` are
absent beside the eight `hookSkipVerbs`: each updates a ref, runs a vetoing
hook, or both. It is also why two members are split by subcommand —
`remote prune`/`remote update` move refs where `remote get-url` does not, and
`worktree add` writes a HEAD and runs `post-checkout` where `worktree list`
does not.

### Two things the package already had, and one it must keep apart

The verb-plus-next-word read the subcommand split needs is the one
`githang.go`'s `gitVerbPair` already does, for the same `worktree add` /
`worktree list` distinction — there to size a deadline, here to judge an
override. The sh walk reaches the pair for free, having already consumed the
global options to find the verb, so this is that precedent and not a second
mechanism.

Both tables are `map[string]bool` / `map[string][]string`, like both of
`githang.go`'s verb tables. A verb table is a set: duplicate keys are a
compile error rather than a pin's job, and `extdiff_qa_test.go`'s ARM 4
census reads a `"diff"` inside an argv-shaped **slice** as a bare `git diff`
reader. The first draft of this fix was a slice and the census red it at
`hookskip.go:135:14`, printing the whole verb list back as "argv here" —
which is the census working, not misfiring: ranger-base-zfza8 added its
map-key exemption for `gitReadOnlyVerbs`, the same class of table, for
exactly this reason.

**`gitReadOnlyVerbs` is NOT this table and must stay separate.** It proves a
signalled git child moved no ref, HEAD or working tree, so it excludes
`status` deliberately — status rewrites the index when the stat cache is
racy. This one proves a verb runs no hook to skip, which `status`,
`write-tree` and `update-index` all satisfy while writing the index freely.
Folding them would have to drop one claim or the other. The overlap is real
and harmless: `merge-tree` was added here because that table already proved
it a read.

The direction of the remaining error is unchanged from r3pri: an
over-refusal is one respelling away and acceptable, an under-refusal is not.
A verb left off the list is refused.

### Verified

- The 46 real argv lines above, replayed through a freshly rendered seat
  shim (claude runtime, the crew deny list): **46 of 46 pass the arm**, where
  all 46 were refused before.
- `TestQAHookSkipRefusedOnEveryRuntime` on claude and codex: every r3pri
  refusal still refuses and the hook still does not fire, with six legs added
  for the verbs the allowlist must not admit — an unknown verb (the alias
  case), `remote prune`, `remote update`, `worktree add`, `checkout` and
  `reset --hard` — and seventeen added for the harness's shape, which must
  not be refused.
- `TestQAHookSkipReadVerbsAreNotHookVerbs`, new: the two tables are
  disjoint, every allowlisted verb reaches the rendered sh, the render is
  deterministic (two renders byte-equal — a map iterated unsorted would not
  be), and the arm still consults `posse_hs_readverb` with the verb AND its
  next word. This is the
  pin on the one mistake the execution pins cannot see — they assert that a
  refusal happened, never which list produced it, so a `commit` added to the
  allowlist would un-do r3pri with a green suite. Verified by breaking it:
  `commit` on the allowlist reds it with both messages.
- Two r3pri legs moved from refused to allowed, deliberately and not as
  collateral: `-c core.hooksPath status` (respelled as
  `-c core.hooksPath commit` so the no-`=` key stays pinned) and
  `-C . -c user.name=x -c core.hooksPath=/dev/null log -1` (respelled onto
  `merge` so the two-`-c` walk stays pinned). Those two legs **were** the
  claim "on any verb"; the claim is now "on any verb that runs a hook".
