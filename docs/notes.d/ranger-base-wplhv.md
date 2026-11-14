# The wall venue on boxLandedCage, the step behind the judge, and three fixture reds that were not the tree's (ranger-base-wplhv)

ADR 0097 D1/D2/D5: the venue as a flag on the judge's own cage renderer, the
wall check as a second check after each green judgement, and ADR 0088 D5's
ratchet plus D6.3's watchdog over two records of its own. This is the ONE
place this bead's readings are said; the code quotes none of them.

## What landed

- **D1** — `boxLandedCage` takes a `landedCageKind`
  (`internal/posse/landedcheck.go`). On the bwrap arm the venue adds exactly
  `--unshare-pid --proc /proc`, `--bind <run>/vartmp /var/tmp` (with a prep
  mkdir, like the judge's private `/tmp`, standing down when that one does),
  and `POSSE_WALL_VENUE=1` on the run's env after `landedJudgeEnvKeys`. On
  sandbox-exec it refuses. The three additions and the prep are
  `wallVenueArgv` in `internal/posse/wallcheck.go`; nothing renders a second
  cage.
- **D2** — `landedChecker.wall` runs `scripts/posse-wall-check` on the judged
  tree, in the judge's own run dir and detached worktree, after a judgement
  that answered GREEN. Exit 3 is read as NO ANSWER (`wallChildVerdict`).
- **D5** — `landedRule` (`internal/posse/landedrule.go`) is the parameter:
  the ratchet and the watchdog are one implementation over two rules, with
  `posse.wallCheck` / `posse.wallCheckQuiet` and the `wall-check: ` /
  `wall-check health: ` markers. `POSSE_WALL_VENUE` moved from
  `wallvenue_test.go` into shipped code, because the render is what writes it.

The pins: `wallvenueargv_qa_test.go` (D1's argv difference, both directions,
and the call sites), `wallcheck_test.go` (both tables, driven through the real
runner with a fake script), `landedrule_qa_test.go` (the two rules share no
record or marker; every field filled; both rules' four descriptions render),
`wallchecklive_test.go` (the live measurement below).

## MEASURED 2026-11-14

D1 host, Linux 7.2.15, bubblewrap 0.13.1, go1.27.3, 16 cores, from this seat
(cage `shims`, so a bwrap starts from it). The run is the whole production
path — one `Schedule` of this branch, the judge behind ADR 0088 D3's cage,
then the wall check behind ADR 0097 D1's venue — over tree `5782830dce21` at
tip `9dc0284f2f47`:

    RHQ_LIVE_WALLCHECK=$HOME/.cache/posse go test -tags posse_arm2 \
      ./internal/posse -run TestLiveTheWallCheckMeasuresInTheRealVenue -v -timeout 30m

| run | judge | wall check | pair | verdict |
|---|---|---|---|---|
| 5 (the bytes that landed) | 98.3s | **81.0s** | 179.4s | **green, 32 pins measured** |
| 4 | 117.0s | 78.3s | 195.4s | green, 32 pins measured (count not yet on the line) |
| 3 | 106.5s | 78.5s | 185.3s | RED — fixture, reading 3 below |
| 2 | 94.7s | 79.3s | 174.2s | RED — fixture, reading 3 below |
| 1 | 96.7s | 65.0s | 162.0s | RED — fixture, reading 1 below |

The wall check is 78.3-81.0s in the venue against ADR 0097's own 96-104s band
for the pins alone, and the difference is the GOCACHE: the judge has just
built this tree in the same run dir, so the venue's `go test` is warm where
the ADR's probe gave each arm a fresh cache. The pair is 174-195s against the
ADR's 199.8s for judge+venue cold. Nothing here is over ADR 0088 D6.2's 240s
ceiling, which both rules read off answered runs — but the PAIR is the thing a
landing now waits for, and 195s of it is one landing's worth of box.

### Reading 1 — a fixture HOME reds two members, and they are codex's

Run 1 was RED on `TestLiveWallRuntimeConfigCodexRunsWithEphemeralDirsReadOnly`
and `TestLiveWallRuntimeConfigEachCLIRunsWithItsConfigReadOnly`, both with
`codex … err=exit status 1, reply seen=false`. Not the tree: this package's
TestMain moves `HOME` to a temp dir under `/tmp` for every test in the binary
(ranger-base-6vvgs), `codex` on this box is a mise command-wrapper that
resolves its installs under `HOME`, and the venue has `--unshare-net` — so it
found no toolchain and had no network to fetch one. The fixture takes the home
out of the passwd database now, which is the one source `$HOME` cannot have
moved, and those two members pass. **In production the launcher's HOME is the
operator's**, so this arises nowhere but in a test binary.

### Reading 2 and 3 — `sun_path` is 108 bytes, and the run dir spends it

Runs 2 and 3 were RED on `TestLiveBwrapHidesTheRuntimeDirsControlEndpoints`
and `TestLiveBwrapWithoutHideReachesTheRuntimeDir`, both
`listen unix …: bind: invalid argument`. That errno is AF_UNIX's path limit,
not the wall: the longest socket this tree binds under a run's TMPDIR is

    <state>/landed-check/run-<10 digits>/tmp/posse-bwh<10 digits>/gnupg/S.gpg-agent.ssh

which is 74 bytes after the state dir. Production's state dir is `~/.posse`
(17 bytes here) and leaves 17 to spare. The fixture's first two attempts spent
51 and then 35 on the state dir and came to 113 and 112 bytes. It now uses the
root as the state dir with no per-test directory inside it, and REFUSES with
the arithmetic in the message when the root is too long. **Stated as a
residual, not fixed here:** a box whose posse state dir is more than 34 bytes
long would red these two members in a production venue run, for this reason
and with this message. Nothing in the tree bounds the state dir's length.

### What the venue rendered, live

    /usr/bin/bwrap --die-with-parent --ro-bind / / --dev-bind /dev /dev --unshare-user
      --chdir <run>/tree --unsetenv DBUS_SESSION_BUS_ADDRESS --unsetenv SSH_AUTH_SOCK
      --unsetenv GPG_AGENT_INFO --tmpfs ~/.bob --tmpfs ~/.claude --tmpfs ~/.codex
      --tmpfs ~/.grok --tmpfs /run --tmpfs /run/user/1001 --bind <run>/tmp /tmp
      --bind <run> <run> --bind <repo>/.git/worktrees/tree <repo>/.git/worktrees/tree
      --tmpfs ~/.ssh --ro-bind ~/.ssh/config ~/.ssh/config
      --ro-bind ~/.ssh/known_hosts ~/.ssh/known_hosts --tmpfs <the rex node's state>
      <and one --ro-bind of /dev/null over each runtime credential this host has,
       which is credentialReadDenyLiterals' list and is not quoted here>
      --unshare-pid --proc /proc --bind <run>/vartmp /var/tmp
      --unshare-net --setenv TMPDIR <run>/tmp --setenv GOCACHE <run>/gocache
      --setenv POSSE_WALL_VENUE 1 -- <run>/wall-check

Every layer before `--unshare-pid` is the judge's, unchanged, which is D1's
"the same renderer through a flag" as the argv reads it. The three additions
are the three groups after it.

## The argv pin's mutants

`TestQAWallVenueArgvIsTheJudgesPlusADR0097sThree` and
`TestQATheVenueCallSiteDiffersFromTheJudgesInTheKindAndTheChildOnly`, driven
one change at a time (the battery is reproducible:
`scripts`-less by design — it lived in the seat's scratchpad as
`mutate-venue-argv.py` and is reproduced from this list). Every row RED, and
the tree restored green after each:

| mutant | which pin |
|---|---|
| drop `--unshare-pid` | argv (missing addition) |
| drop `--proc /proc` | argv |
| drop the private `/var/tmp` bind | argv |
| drop `POSSE_WALL_VENUE=1` | argv (env half) |
| set `POSSE_WALL_VENUE=0` | argv (env half) |
| add a fourth addition (`--unshare-uts`) | argv (unexpected addition) |
| venue weaker than the judge (no `--unshare-net` on the venue arm) | argv (the REMOVED direction) |
| no prep mkdir for the `/var/tmp` source | argv (the prep arm) |
| the wall check runs in the judge's cage | call site (the kind) |
| the venue gets a wider write set | call site |
| the call site sets the venue variable instead of the render | call site |

The removal row is the one worth keeping: D1's first bullet is "it is never
weaker than the judge", and nothing but a two-directional diff reads it.

## Residuals, stated

- A coalesced run whose tree is already on the JUDGE's record returns before
  judging (ADR 0088 D4's cache), so the wall check does not run for that tree
  either — there was no judgement, which is what D2 keys on. The cached case
  is a landing that moved no bytes, so the wall check's own record already
  names that tree or the next landing re-asks.
- The `/var/tmp` bind stands down when the private `/tmp` does, which is what
  D1 says. `landedCheckTmp` stands down for a home, run or tree under `/tmp`
  and says nothing about `/var/tmp` — so a run dir under `/var/tmp` would
  render a private `/var/tmp` over its own tree. No production run is that
  shape (the run dir is under the state dir); this package's fixtures are,
  and they only compare argv.
- The state dir's length, reading 3 above.
- `make test` on a caged seat is unchanged: the members abstain (ADR 0096 D3)
  and the venue measures. The live pin is env-gated, in arm 2, and a
  prerequisite of nothing.
