# The in-force BIT, spelled by hand beside the one read (ranger-base-uz4cs)

2026-10-31. ADR 0077 D1's rule is "one read of the runtime-home variable
names", because since the typed pin those names travel in two directions:
posse resolves its own view of a CLI's state root through them and posse
TYPES them onto the seat it hires. `runtimeHomeOverride` (runtimehome.go) is
that read, and it answers two things — the value, made absolute, and whether
the variable is in force at all.

`claudeHistoryPath` (sentline.go) took one of the two. The VALUE came from
`ClaudeConfigDirIn`; the BIT was spelled on the line above it:

```go
if (err != nil || home == "") && os.Getenv("CLAUDE_CONFIG_DIR") == "" {
```

Found while fixing ranger-base-sczjy. Measured on **go1.26.5 darwin/arm64**,
in this bead's session worktree.

## It was not a wrong answer, and that is the point

`runtimeHomeOverride`'s bit is `v != ""`, which is exactly what that line
computed, so the two agreed for every value — a relative one included, since
the value half already went through `ClaudeConfigDirIn`. What a second
spelling costs is not the answer, it is the **coupling**. MEASURED here, with
the `v == ""` bail deleted from `runtimeHomeOverride`:

| the line | the mutant |
|---|---|
| pre-fix, `os.Getenv(…) == ""` | **`ok`** — survives |
| asking `runtimeHomeOverride` | **reds**, both no-variable arms |

The pre-fix line never called the function the mutant broke, so it could not
be reddened by breaking it. That is the same survival ranger-base-api7c
closed inside `ClaudeConfigFile` one site over, and the same class as
ranger-base-x5f6p and ranger-base-wd4be before it — the fourth occurrence,
surviving inside the function whose own doc already recorded the third (an
earlier version `TrimSpace`d `CLAUDE_CONFIG_DIR` where `ClaudeConfigDirIn`
does not, so a whitespace-only value made the two disagree).

Under that mutant the function answers `<cwd>/history.jsonl`, not the bare
relative `history.jsonl`: the empty value the mutant now calls "in force" is
not absolute either, so ranger-base-sczjy's `filepath.Abs` resolves it against
the launcher's cwd on the way through. Same store of record in a directory
nobody named (ranger-base-58b5's shape), one `filepath.Abs` further along —
and `lastSubmitted` stats that path from inside govern's G2 tick and
dispatch's settle judgment.

## The fix, and why the pin asserts a literal

```go
if _, named := runtimeHomeOverride(ClaudeConfigDirVar); (err != nil || home == "") && !named {
```

The pin is `TestClaudeHistoryPathTakesItsInForceBitFromTheOneRead`
(runtimehome_qa_test.go, arm 1), and its `want` is a **literal `""`** rather
than `ClaudeConfigDirIn`'s answer. That is the whole difference from
`TestClaudeHistoryPathAgreesWithClaudeConfigDirIn` (ghostcomposer_qa_test.go,
arm 2), which is the AGREEMENT half and is green under the mutant by
construction: it derives its want from the same function it tests through, so
a mutant moves both sides together. Four arms — no variable, present but
empty, absolute, relative — and each also asserts the answer is `""` or
ABSOLUTE and never anything between.

## The census, and its scope

Prose has held "one read" three times and lost three times, so
`TestNoSecondReadOfARuntimeHomeVariable` derives it: every non-test `.go` file
in internal/posse is parsed (mode 0, so comments are not retained) and an
`os`/`syscall` `Getenv`/`LookupEnv` whose single argument NAMES one of the
three variables — as a literal or as the exported constant — is a finding.

Two things keep it honest:

- It matches the **read**, not the name. This package's prose says
  `CLAUDE_CONFIG_DIR` some seventy times; `noHomeSilence("CLAUDE_CONFIG_DIR",
  …)` passes it to a message, and `credentialDirVars` (credential.go) carries
  it in a list of names to ask ABOUT — a second spelling that is deliberate,
  documented and pinned, because it reads no value at all. None of those is a
  second answer to "which file is in play".
- Its expected answer is **zero sites**, including inside
  `runtimeHomeOverride`, which takes the name as a parameter. A census that
  must find nothing passes just as well broken, so the matcher is pinned
  against ten synthetic sources
  (`TestTheRuntimeHomeReadCensusMatchesTheReadAndNotTheName`), the first of
  which is the line this bead removed, verbatim. MEASURED: restoring that line
  reds the census at `sentline.go:145`.

SCOPE, stated rather than implied: this package's own directory. That is where
every posse-side reader of these variables lives — `cmd/` carries none, by the
grep this census replaces. Reaching `cmd/` would mean climbing to the repo
root, which puts a pin in the tree-wide class and wants a Makefile door
(AGENTS.md's register); this one reads `.` and stays out of it.
