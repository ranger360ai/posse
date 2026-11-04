# The read arm judged remote on its next word, and git takes an option first (ranger-base-c1pqo)

Fix for hoover's finding ranger-base-jvwcb, landed with the ranger-base-hogeb
replay of ranger-base-ap4p6 (72f5fa69), so main never carries the hole.

## The defect

ap4p6 defers the `-c core.hooksPath` / `hook.*` / `include*` refusal until
the verb is known, and lets it through on a read. `remote` reads or writes
by subcommand, so `posse_hs_readverb` was handed `"$1" "${2:-}"` and the
table listed `""`, `-v`, `--verbose`, `get-url` and `show`. git takes
remote's `-v` BEFORE the subcommand, so `git -c core.hooksPath=/dev/null
remote -v prune origin` was read as a read and deleted refs with hooks off.
That breaks the table's own rule (a): a read performs no reference update.

MEASURED 2026-11-04 by hoover, git 2.50.1 (Apple Git-155), plain git in a
scratch repo: `remote -v prune`, `-v -v prune`, `-vv prune`, `--verb prune`
and `-v update --prune` each pruned the stale tracking ref. `remote -- prune`
is git's "unknown subcommand", and `worktree -v list` is "unknown switch".

## The fix

`posse_hs_readverb` now gets the verb and every word after it. For a
`hookSkipReadSubverbs` verb it skips each leading word that starts with `-`,
except a lone `--`, and judges the first word that is left. A read is only:
no such word, `get-url` or `show` (and `list` for worktree). The table holds
subcommands only; `-v` and `--verbose` are gone from it. Skipping cannot
swallow a subcommand, because remote's one top-level option is a flag with
no value and worktree takes none.

Rendered output: one `while` that skips options, per subverb verb, still in
sorted key order. It gave the same answer on 16 argv shapes under `/bin/sh`
(bash 3.2.57) and `/bin/dash` (Apple dash-16), MEASURED 2026-11-04.

## The alias choice (ranger-base-1agm6 §5)

Merging ap4p6 with z8kym's alias resolver needed a decision neither close
made: does the deferred `-c` refusal fire before the alias is resolved, or
after? It fires on the FIRST walk. An alias name is never a read verb (git
runs a builtin before it looks up an alias, and every read verb is a
builtin), so `git -c core.hooksPath=x <alias>` is refused whatever the alias
expands to, a read included. That over-refusal is one respelling away. An
alias that carries its own `-c` is judged on the re-walk, against the verb
it expands to. Legs for both directions are in `hsRefused`/`hsAllowed`:
`-c … pcm` and `-c … pstat` refused, `prm` (`-c … remote -v prune`) refused,
`pst` (`-c … status`) allowed.

## Pins

`TestQAHookSkipRefusedOnEveryRuntime` (arm 1, claude and codex, through
`WrapWithGates`) carries hoover's 7 refused and 5 allowed legs plus
`remote -v -- prune`, `worktree -v add` and the four alias legs.
`TestQAHookSkipReadVerbsAreNotHookVerbs` refuses an option in the subverb
table and checks the call site passes `"$@"`. Mutant check: putting `-v`/
`--verbose` back and judging only the next word turns both pins red, with
`-v prune`, `--verbose prune`, `-v -v prune`, `-v update` and `prm` exiting
0 on both runtimes.

## Still open

ranger-base-e09l0 (160d6bd9, `posse_hs_split`, the alias unquoting) is
merge-blocked on its own branch and will conflict with this file again when
it lands. Its splice feeds the same re-walk, so the alias legs above apply
to it unchanged.
