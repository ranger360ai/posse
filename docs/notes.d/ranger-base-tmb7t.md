# ranger-base-tmb7t · verifying the four hook-skip closes

QA verification of ranger-base-tpvkc, ranger-base-7yvo4, ranger-base-obdnk
and ranger-base-r3pri. All four verified. One suite gap found and closed.

## The gap: a content comparison cannot be pinned on a box with nothing to hide

ranger-base-r3pri's env arm judges `GIT_CONFIG_SYSTEM` / `GIT_CONFIG_GLOBAL` /
`GIT_CONFIG_NOSYSTEM` by CONTENT — the hook-relevant keys git sees under the
override against the ones it sees with the override unset — which is the right
design and makes the arm untestable in the direction that matters, on this box,
with real git.

ranger-base-zwkgb's attack is `GIT_CONFIG_SYSTEM=/dev/null` on a box whose
SYSTEM scope carries the employer's managed hook. Here `/etc/gitconfig` does
not exist, so the real system scope is empty, every system override compares
empty-to-empty, and the arm correctly allows it. `hookskip_qa_test.go` puts its
managed hook in GLOBAL scope for exactly that reason, so its one refused system
leg (`GIT_CONFIG_SYSTEM=<planted file>`) is the ADD direction — an override that
ADDS hook config git did not have. The HIDE direction had no leg at all, and
`GIT_CONFIG_NOSYSTEM` appeared in no test in the tree.

MEASURED 2026-11-03/04: deleting the `GIT_CONFIG_NOSYSTEM` trigger from the env
arm, and separately making the scope comparison always return equal, each left
every shipped hook-skip pin GREEN.

**The route is a fake git at render time.** `renderHookSkipArm(real, …)` takes
the git the arm execs, so a fake that reports a non-empty system scope when the
scope is not overridden — what a real git does on a managed box — makes the hide
direction reachable without writing `/etc/gitconfig`.
`hookskipnosystem_qa_test.go` is that pin: four hide spellings refused by name
with a refusals.log line, three controls that must NOT be refused (no override,
`NOSYSTEM=0`, `NOSYSTEM=`), and both mutations above red it.

The arm itself was never broken — it refuses all four spellings by execution.
What was missing was anything that would notice if it stopped.

## Worth keeping from the other three

- **tpvkc** ships both halves the bead asked for, and `posse runtime check
  <name>` needs the NAME: a bare `posse runtime check` is a usage error (exit 1,
  one line), which reads at a glance like a missing row. The row is there for
  claude and codex, and prints both resolved system paths plus `system scope
  /etc/gitconfig absent: nothing to carry`. On this box it passes vacuously over
  0 entries, as the bead predicted; the fixture arms are what prove it. The seat
  system path now reads `/etc/gitconfig`, not `/dev/null` — t301k's arm b live.
- **obdnk**'s three-inlet pin is the best-built thing in this batch: it fires
  each of `GIT_CONFIG_GLOBAL`, `XDG_CONFIG_HOME` and `HOME` WITHOUT the row and
  fails with `CONTROL FAILED` if an attack did not fire, so its green cannot be
  vacuous. Mutating the two-file case to return `/dev/null` reds two pins.
- **7yvo4**'s landing pin discriminates both ways: restoring `--no-verify` reds
  it on the unfired marker, and dropping `clearHookLeftovers` reds it on
  `MM .beads/issues.jsonl`.
- git settles two spellings itself on 2.50.1: it accepts NO abbreviation of a
  global option (`--config-en`, `--config-en=…`) and no attached `-c` value
  (`-ccore.hooksPath=…`), all three `unknown option`. And `cherry-pick`/`revert`
  have no `--no-verify` at all (`-n` is `--no-commit`), so the arm is right to
  scan neither.
