## bd daemons leak, and there are two vectors (ranger-base-42mv)

bd 0.49.1 **auto-starts a per-database daemon on first use and stops none**.
Any bd call in a throwaway directory therefore leaves a process behind that
outlives its directory: a sqlite handle to a database nothing can reach
again. It also holds the directory well enough to defeat `t.TempDir`'s own
`RemoveAll` — so one call leaks a *process* and a *directory*.

**Vector 1: our own calls.** Fixed by `--no-daemon`, which is a 0.49.x global
flag and goes before the subcommand. Measured 2026-08-28 in a throwaway repo,
`init` + `create` + `list`:

| arm | daemon after | `.beads/daemon.pid` | rows read back |
|---|---|---|---|
| plain | 1 | written | 3 |
| `--no-daemon` | 0 | absent | 3 |

Same data, no process. The flag is the answer whenever a *running* daemon is
not part of the claim — resolution of which database a call lands in happens
before bd reaches for a socket, so a redirect/worktree pin loses nothing by
it. When a running daemon **is** the claim (it imports a newer JSONL before
answering — `liveCageBeadStore`, internal/rhq/cageinnerlive_test.go), keep it
and stop it in cleanup instead.

**Vector 2: bd's own git hook, and `--no-daemon` does not cover it.** `bd
init` installs a pre-commit hook that runs a bare `bd sync --flush-only` with
no such flag. So a `git commit` in any bd-initialized repo starts a daemon
whatever *we* pass — measured 2026-08-28: with every direct call carrying
`--no-daemon`, `TestLiveWorktreeSharesOneGraph` still produced one daemon and
one `daemon.pid`, written by the hook during its own fixture commit. The hook
is bd's, not ours; cleanup is the only lever. `bd init --skip-hooks` avoids it
where the hook is not part of what the fixture reproduces.

**Cleanup, and the blast-radius rule.** The handle is `.beads/daemon.pid`
beside the fixture's own database — a file written seconds ago in a directory
nothing else has ever seen, so SIGTERMing it is not a guess about somebody
else's process. SIGTERM, not KILL: it flushes the WAL. Wait for the process to
go before the directory is removed, or `RemoveAll` fails with *directory not
empty*. **Never `bd daemon stop-all`** — it would take the canonical queue's
daemon with it. `bd daemon stop <dir>` is not sufficient on its own: it exits
0 and leaves the process running (measured 2026-08-27).

**Where the two halves live.**

- *Prevention*, `bddaemonleak_qa_test.go`: every `_test.go` that shells the
  real bd must either pass `--no-daemon` or stop the daemon via
  `.beads/daemon.pid`. Per-file, because that is the granularity at which the
  two honest answers are legible.
- *Detection*, `scripts/verify-bd-pin.sh` assertion 5: every live `bd daemon`
  is classified by **cwd** — `LEAKED` when the directory is gone, `EPHEMERAL`
  when it is under a temp root — reported separately from the binary verdict,
  because a daemon can be running the right binary in a directory that no
  longer exists and folding the verdicts prints `ok` for exactly that process.
  A cwd the probe cannot read is `unverified`, never `ok`. The script still
  kills nothing: it prints `kill -TERM <the leaked pids>`, naming those and
  only those.

Classification by cwd, and by cwd alone, is what makes the remedy safe: the
canonical queue's daemon sits in a real repo and is never named.

posse's own bd calls are not a vector — `Bd.run` takes a directory and every
caller passes a configured repo (`BeadsDirs`), so posse never invokes bd
somewhere throwaway. The reachable sources are test fixtures, ad-hoc calls in
session scratchpads, and the hook above.
