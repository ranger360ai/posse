# A launch out of a scratch posse home disarms the real repo's commit wall (ranger-base-q0np6)

G12 kept flipping to `L3 prepare-commit-msg hook … ours but stale — the data
ceiling guard is not realized` in `~/src/posse`, minutes after the operator's
`posse gates install-hooks ~/src/posse` had cleared it. The bead was filed
against the push lane's own gauntlet, narrowed to `make treepins`, and asked
for the test or make target "reaching the shared common dir instead of a temp
repo" to be made to write under a temp dir.

It is not a test and not a make target. It is a **session launch**, writing
where it is designed to write, from a home nobody meant it to read.

## The writer

`HerdrBackend.planLaunch` (internal/posse/herdrback.go, the unmanaged arm) ends
every launch with

```go
if deniesGitPush(ag.Deny) {
    InstallPrePushHook(dir)
}
a.InstallCommitGuardHook(dir)
```

Two things meet there, each correct on its own:

- `dir` is the session's worktree, and `installHook` resolves the slot through
  `hooksDir` → `git rev-parse --git-path hooks`, which for a **linked worktree
  is the COMMON hooks dir** — deliberately, so a worktree keeps getting the
  hooks of the repo it belongs to (gates.go's `hooksDir` doctrine). A session
  worktree of `~/src/posse` therefore writes `~/src/posse/.git/hooks`, which
  every other worktree and the main checkout read. `InstallCommitGuardHook`'s
  own doc says so: "writes the guard into the repo at dir (its common git dir,
  so worktrees share it)", and CHANGELOG names the launch as one of the two
  things that re-render a hook at all.
- `a` is `b.App` — the **launching process's** home. The render it produces is
  a function of that home's `config.yaml`: `a.OpsPatternSet()`,
  `a.DeriveGuardValueLiterals()`, `a.DeriveCrewLiterals()`,
  `a.BeadsVisibility(hookRepo(dir))`.

So a launch from home A into a worktree of repo R arms R's shared wall with
A's config, and every reader of that wall — G12's sweep, the launch probe,
`scripts/verify-hook-freshness.sh`, the operator's `install-hooks` — renders it
from the box's real home. One file, two writers, different configs, last writer
wins, silently.

## MEASURED 2026-11-06, darwin 25.6.0, posse 0.5.3+474337e8

The same installed binary, the same repo, **only `RHQ_HOME` different**, each
render taken under a `core.hooksPath` redirect into a scratch dir:

| `RHQ_HOME` | lines | sha256 (prefix) | data-ceiling banners |
|---|---|---|---|
| `~/.config/posse` (this box) | 1897 | `745b3537` | 4 |
| `…/8gh58/…/scratchpad/home-rex` | 1137 | `6e8bdc03` | 0 |

The 760-line delta is exactly the four data-ceiling arms — the ADR 0050
patterns block, its staged-PATHS arm, its commit-MESSAGE arm and the ADR 0068
D1 `.beads/*.jsonl` reporting reader. That is G12's sentence word for word:
`l3guardgap_qa_test.go` pins that the ceiling arm renders **nothing** for a home
that configured no patterns, and neither scratch home has a
`data_ceiling_patterns:` key.

### Which launches did it

Two ad-hoc codex seats, launched by dinesh's session ranger-base-8gh58 out of
scratch homes in its own scratchpad — `home-herdr/agents/codexherdr.md` and
`home-rex/agents/codexrex.md`. Neither persona exists in
`~/.config/posse/agents`, and neither has a gate dir in
`~/.config/posse/state/gates`, because their gates were rendered into the
scratch homes' own `state/gates`. Both seats did real work and landed real
commits on `main`.

The launch→write lag is 1–2s, and it is the whole correlation the bead turned
on:

| worktree created | hook written | seat |
|---|---|---|
| 22:06:57 | 22:06 (as reported) | `codexherdr-posse-ranger-base-w4enh` |
| 22:11:18 | 22:11:20 | `codexrex-posse-ranger-base-7vh7y` |
| 22:16:56 | 22:16:57 (pre-push), 22:16:58 (prepare-commit-msg) | `jian-yang-posse-ranger-base-q0np6` |

The third row is this bead's own seat, reproducing the mechanism five minutes
after the second — and it wrote a **fresh** body, because a claude seat launches
from the box's real home. That is why the wall is fresh now while two launches
earlier in the same ten minutes staled it.

### `make treepins` is not the writer

The bead's two narrowing comments read `make treepins rc=0` in the lane log at
22:11:26 against a hook mtime of 22:11:20 and concluded the write was inside it.
The six seconds are a coincidence: codexrex's launch is at 22:11:18.

MEASURED: the full unfiltered `scripts/test-times.sh go test ./internal/treepins
-timeout 25m -count=1 -v`, 22:24:56 → 22:37:07, **860 tests, `ok … 723.438s`**,
with two watchers on `~/src/posse/.git/hooks/posse-prepare-commit-msg` and
`posse-pre-push` at 0.1s — one on sha256, one on mtime, because a re-render of
identical bytes moves the mtime and not the sha and the operator's evidence was
an mtime. **Neither watcher fired.** Not a byte, not an mtime, across the whole
run.

Statically it agrees: the only `install-hooks` call in the package is
`hookfreshness_qa_test.go`'s rig, aimed at `$HOME/<name>` temp repos under a
scratch `HOME`/`RHQ_HOME`, and `scripts/verify-hook-freshness.sh`'s per-repo
reference render is aimed at the operator's own repo but taken under a
`core.hooksPath` redirect that it **refuses to measure without** — it asks git
`--git-path hooks` and exits 2 unless the answer is its own directory. That
redirect does govern posse's lookup: `protocolEnv` rewrites `PATH` and nothing
else, so `GIT_CONFIG_*` reaches the child.

## Why the named fix is the wrong one

"Make it write only under a temp dir" would disarm the L3 wall in the repo every
session commits into, which is the one repo it exists to guard. The write is
not a leak out of a fixture; it is the launch doing its job with the wrong
config in hand.

What is actually wrong is that the launch's write is **not monotone**: a launch
that finds the wall armed and is about to render a body with FEWER guards
overwrites it without a word. The detection for the other direction already
exists one line below the install — the launch warns when
`preHeal.CommitGuardVerdict.reStamped()`, i.e. when it found the wall WRONG and
silently re-stamped it. The missing half is the launch that finds it RIGHT and
makes it wrong. Deciding what a launch should do there — refuse, warn, or take
ADR 0052 D2's per-session hooks dir for a slot shared with sessions this launch
does not own — is a change to the design of the wall's write path, so it is
filed rather than taken here.

## The operator-facing half

Launching a session from a scratch posse home into a worktree of a **real**
repo re-renders that repo's shared commit guard from the scratch home's config,
for every other worktree and the main checkout, until someone types
`posse gates install-hooks <repo>`. A scratch home for a runtime experiment
wants a scratch repo too.
