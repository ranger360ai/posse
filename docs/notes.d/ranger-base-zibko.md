# The red that was a commit old, and the test that seeds the shared home (ranger-base-zibko)

*2026-11-11 · jian-yang · verifies ranger-base-zvgql's fix; sharpens its note*

## What this bead found

Nothing to fix. `TestPersonaLaunchRuntime`'s whole-package failure was
already closed on `main` when this bead was dispatched, and the whole of the
work was verifying that and naming the cause precisely enough that the next
reader does not re-derive it.

The bead's repro was taken in a fresh clone at `b9129561`. The fix is
`65436282` (ranger-base-zvgql), one commit later: it gave that test's grok
recipe a session dir of its own. The recipe had named no `dir:`, so it
launched in `$HOME` — the test binary's one shared temp home (TestMain,
ranger-base-32bdv) — and read another test's residue there.

**MEASURED 2026-11-11, linux, go1.27, in a worktree of `main` at
`43af67ac` with nothing local in the tree: `make test` exits 0, zero `FAIL`
lines.** Arm 1 `./internal/posse` 180.4s, arm 2 65.9s, arm 3 103.9s,
`./internal/treepins` 196.4s, `cmd/posse` 172.3s; `tree-check` and the
silent-revert audit (2527 commits, 0 untriaged) green ahead of them. Arm 1's
unfiltered `./internal/posse` is the run that was red, so the reading is of
the failure's own venue and not of a filter beside it.

## The chain

`CheckParityIn` → `applyWallWitness` (wallwitness.go:94) → `RenderWall` →
`runBwrapPrep` seeds grok's executable home members (`runtimeExecMembers`,
ranger-base-zvgql) under `$HOME/.grok`. `config.toml` is seeded **zero
bytes**. `.grok/config.toml` is on grok's `ProjectConfig` list, so a later
launch in that same directory reaches `projectConfigTrustFile`
(parity.go:762), which parses it as JSON, gets `unexpected end of JSON
input`, and refuses:

```
trust: grok reads ~/.grok/config.toml from the session dir before any turn —
project config classification failed: invalid JSON: unexpected end of JSON input
```

That is the herdr_test.go:4481 refusal, letter for letter. It is invisible to
`-run` because neither half is this test's: one test writes the file, a
different test reads it, and a filter that selects either one alone selects
away the other.

## Which test writes it

The bead could say only "a sibling test that clears or races HOME". It is one
test, it needs no race, and it reproduces alone in under a second:

```
RHQ_TEST_HOME=<an empty dir> \
  go test -count=1 -run '^TestQATheAbsoluteWritableRowDegradesNoLaunch$' ./internal/posse
```

`<that dir>/.grok/` afterwards: `config.toml`, `AGENT.md`, `AGENTS.md`,
`Agents.md`, `CLAUDE.md`, `Claude.md`, `CLAUDE.local.md`,
`managed_config.toml`, `requirements.toml`, `trusted_folders.toml` — all zero
bytes — `lsp.json` and `settings.json` at 3 bytes (`{}`), and `agents/`
`commands/` `hooks/` `installed-plugins/` `personas/` `plugins/` `rules/`
`skills/` `workflows/`.

The test is bwrapprepauthority_qa_test.go:592. It takes its fixture from
`bwlFixture`, which — unlike `wrcFixtureIn` and `weFixtureIn` — does not
`t.Setenv("HOME")`, so the `~/.grok` its grok runtime declares resolves
against the shared home. Six sibling bwrap-prep tests in the same file were
probed the same way and seed nothing of grok's;
`TestQABwrapLaunchDoesNotMaterializeTheStoreOfRecord` seeds `~/.mycli-bpa`,
a name unique to itself that collides with nothing.

The probe that names a seeder is worth keeping: plant a regular **file** at
`$RHQ_TEST_HOME/.grok` and run the suspects. Every test whose prep tries to
`MkdirAll` under it fails by name instead of silently succeeding.

## The correction

docs/notes.d/ranger-base-zvgql.md attributes the seed to "a parallel test's
real bwrap prep", and says the same thing happens outside tests "once any
caged grok seat has **run**". Both understate when the condition appears.

It is not a launch that writes those files. It is a **read-only parity
check**. `applyWallWitness` is reached from `CheckParityIn`, whose other
callers are `posse gates` in a cwd and the preflight — neither of which
launches anything — and wallwitness.go:94 says so in its own comment:
"rendering the bwrap arm writes files and makes host-side prep". So a box
where `posse gates` has been typed once, against a grok PID at the seatbelt
tier, already has the seeded `~/.grok` that degrades a grok session launched
in `$HOME`.

What that note decided is unchanged and is not reopened here: the downstream
verdict is the old verdict for any home whose `config.toml` exists, and
ranger-base-zvgql ruled it not worth filing. This fragment corrects only the
*when*, because a reader who believes a seat must have run will look for a
launch that never happened.

## Not closed here

Nothing. The red is gone and the suite is green at `43af67ac`.
