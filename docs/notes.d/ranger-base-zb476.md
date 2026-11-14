# A launch-line assertion reads both places the line lands in (ranger-base-zb476)

Filed from ranger-base-6ca29, whose fix was one site of a class nobody had
counted. The class: a pin that asserts on a persona LAUNCH LINE and reads the
fake herdr's log through `calls`, which sees only what was typed.

Over `PaneLineMax` (1023 bytes, the tty's `MAX_CANON` — `internal/posse/paneline.go`)
a rendered line is not typed into the pane at all. It is written to
`state/launch/<session>.sh` and the pane types `. '<script>'` instead, so
`calls.log` holds the dot-source and not the line. Which of the two places a
line lands in is a fact about its LENGTH — about the fixture's path lengths,
about `TMPDIR`, and about any unrelated landing that adds bytes to a rendered
line — and never about the launch. `launchLog` reads both.

**Why it is a live bug.** ranger-base-6ca29 measured `internal/posse/parity_test.go`'s
grok line at 959 bytes with `TMPDIR` unset (typed, 64 bytes of headroom) and
1325 bytes with `TMPDIR` under the home (spilled, and the pin red with
`bwrap wrap missing` over a launch that was correctly walled).
ranger-base-rq83c walked six fixtures over the same cliff when ~110 bytes of
an unrelated credential-dir pin were added; ranger-base-90y3c is a third
landing. And in production spilling is the NORMAL case — paneline.go records
every persona's line at 1867-2058 bytes against the 1023 limit, all seven
spilled — so a fixture short enough to be typed is measuring the EXCEPTIONAL
path, and an assertion satisfied by the typed form can be **green over a
spilled line it never read**.

## The census

`internal/treepins/launchlineread_qa_test.go`, behind `make corpus-check`
(whose sentence is "censuses over this repo's own .go, test and script
sources"). Two arms: the census over `internal/posse`, and a control arm over
planted sources.

**Scope is decided by the asserted TEXT.** internal/posse has 395 calls of
`calls` and almost none are about a launch line: the log holds one herdr argv
per line and nearly every pin reading it asks about herdr's own argv —
`workspace create`, `agent prompt`, `agent wait --until working`. None of that
can spill, because `PaneLine` is reached by the persona COMMAND alone, so
`calls` is the right reader for all of them. `--env RHQ_TIER=fast` is the one
that LOOKS like a launch-line assertion and is not: it is herdr's own
`workspace create --env K=V` (`Herdr.CreateWorkspace`), and eleven assertions
name one.

**The marker vocabulary is derived in the half that can be.** From
`builtinRuntimes` in internal/posse/runtime.go: each runtime's CLI name (with
its trailing space, so `--env RHQ_RUNTIME=claude` is not read as one), every
`-`flag in its resolved `Command` and `Unattended`, its `ModelFlag` printf
head, and every `-`flag literal in the bodies of the `Realize` and `Skills`
functions it names — `-s read-only`, `--allowedTools`, `--plugin-dir` and
three more live in neither template, and three of the six sites below were
found by one of them. A runtime added to the built-ins, or a flag added to a
template, widens the census the same day.

Two readings the derivation needed:

- **The resolution is a FIXPOINT, not one pass.** `CodexFleetFlags` is a chain
  of other constants (`CodexDisableHooks + " -c " + codexConfLoginShell + …`),
  so a single pass bound it to nothing, codex's whole `Command` template
  resolved to nothing with it, and the census read NO flag off the one runtime
  whose template is three constants deep. `--disable` was missing from the
  vocabulary. Each pass re-resolves every binding against what the last one
  knew; the one-distinct-value rule is re-applied every pass and never
  relaxed.
- **A sweep of the `*Runtime` METHODS was measured and rejected.** It would
  derive `--settings` and six more worth having, and also `--cmd`, `--flag`
  and `--flag=value` — posse's own CLI flag and a plain session's command.
  `internal/posse/relaunch_test.go` asserts `run-me --flag` off calls.log for
  a `--cmd` session that has no launch line at all. So the sweep is out and
  `--settings` is one of four LISTED markers, each carrying its renderer. The
  other three are `GATES ` (the marker both readers collapse the gate prefix
  to, which only ever prefixes a persona command), `bwrap` and `sandbox-exec`.

**The association is per CALL, not per statement**, and it was statement-level
first. `internal/posse/dispatch_qa_test.go:427` is the shape that settled it:
`strings.Count(c, "pane run") != 2 || !strings.Contains(launchLog(…), "GATES claude")`
— one `if` whose author had already made exactly this distinction, the COUNT
off calls.log and the CONTENT through launchLog, with a comment saying so. A
statement-level reader attributed `GATES ` to `c` and reported the one site in
the tree that had got it right.

**A name is resolved at the position it is USED.** `TestPersonaLaunchRuntime`
binds `got` to `launchLog` once and to `calls` twice over four assertions; a
map of name → reader holding only the last blamed `calls` for the assertion
that had used `launchLog`. Lexical order, which is exact for a straight-line
test body.

**What is loose, and what covers it.** An assertion whose text carries no
marker — a bare model id, a quoted fixture path — is out of scope and this
reader will not find it. There is no closed vocabulary in the other direction
either: posse renders herdr's argv and the launch line from the same package,
so "a flag literal in posse's sources" separates nothing, and a default-IN
rule would have to exempt pane ids, line counts and `--cmd` text one at a
time. What covers that direction is the SITES FLOOR — exact, not a margin — so
a site leaving the census reds the door whatever took it out.

## What the census found

MEASURED 2026-11-14 (linux, this box): 885 .go files read, 23 derived markers
over 4 runtimes beside 4 listed, **22 in-scope launch-line assertions, 7 of
them through `calls`**. 0.37s. One of the seven is an exemption; the other six
are the class:

| site | markers |
|---|---|
| `herdr_test.go` `TestPersonaLaunchRuntime` | `GATES ` `codex ` `grok ` `-s read-only` `-c model=` `--rules=` `--no-leader` `--permission-mode` |
| `planguardpark_test.go` `TestTripChoosesNoProviderForTheBead` | `GATES ` `grok ` |
| `skills_test.go` `TestSkillsParity` | `--plugin-dir` |
| `skills_test.go` `TestSkillsOnCodexAcceptance` | `--plugin-dir` `-s read-only` |
| `unattended_test.go` `TestNoPersonaLaunchPathsTypeTheMode` | `claude ` `--permission-mode` `--strict-mcp-config` |
| `verify_nx85_qa_test.go` `TestQANoPersonaRelaunchTypesTheModeExactlyOnce` | `claude ` `--permission-mode` `--strict-mcp-config` |

**Four of the six were ABSENCES** — `--plugin-dir` must not be on a plain or
codex line, `GATES grok ` must not be anywhere after a parked bead — and an
absence is the direction where a calls()-only reader is worst: a line it cannot
see reads as a line that was never typed, so the pin passes over the launch it
was written to forbid.

**The spill was measured, not argued.** With `TMPDIR` at a 138-byte path under
`/tmp`, and with `TestPersonaLaunchRuntime`'s grok recipe assertion put back to
`calls` and nothing else changed, that test reds `recipe runtime: not applied`
under the long TMPDIR and passes under the default — the same pair
ranger-base-6ca29 measured at 959 against 1325 bytes, over a different fixture.
Through `launchLog` it is green under both.

That is the whole claim, over the one fixture it was measured on. The rest of
the set is green under both TMPDIRs across all three arms after the fixes,
which says the fixes break nothing at either length; it does NOT say each of
those lines crosses the cliff. Which of them does is a fact about their fixture
paths, and the point of the rule is that nobody has to know.

The live count is 21 after them rather than 22, because
`TestPersonaLaunchRuntime` was the one function asserting through BOTH readers
and a site is one (file, function, reader).

## The one exemption

`internal/posse/bwraplaunchline_qa_test.go` `TestQABothMultiplexersTypeTheWallLineVerbatim`,
on `llExempt` with its reason, keyed on (file, function) rather than on a line
number. Nothing can spill there: the pin calls `HerdrMux.Start` directly with a
line it rendered itself, and `PaneLine` is never on that path — the spill
decision is made one layer up, in `planLaunch` and `RelaunchAgent`. Its
assertion is `pane run w1:p1 ` + the line, the herdr argv INCLUDING the
command, which is the subject: that the multiplexer slot's occupant does not
rewrite, re-quote or re-split what posse gave it.

The exemption list is checked both ways — a stale row reds — and a row whose
reason is too short to be worth reading reds too.

**`internal/posse/paneline_test.go` needs no exemption**, which is worth saying
because it is the file a reader will look for. Its whole subject IS which of the
two places a line landed in, and it reads the two halves APART: `paneRunLines`
for the typed bytes, `os.ReadFile(a.LaunchScript(…))` for the spilled ones,
compared against each other. A pin whose subject is the spill reads them apart,
which is exactly what keeps it out of scope.

## Four readers, one per question

ranger-base-6ca29's scope note left open whether the tree should have one
reader. It should not: there are four questions. They are all in
`internal/posse/herdr_test.go` now, which is the SHARED file, so every build
arm can ask any of them.

| reader | reads | right for |
|---|---|---|
| `calls` | every herdr argv, gate prefixes collapsed | herdr's OWN argv, none of which can spill |
| `launchLog` | `calls` plus every spilled script | "does any launch carry this" |
| `paneRunLines` | the RAW typed `pane run` commands, in order | a subject that IS the typed bytes: length, and whether this line spilled |
| `launchLines` | `paneRunLines` resolved and collapsed | "how many launches carried this, and in what order" |

Only the first two are this census's business. Everything else is a different
reader and therefore out of scope by construction, which is the right answer
and not an omission: `paneRunLines` is deliberately raw, and `launchLines` is
already spill-aware.

`paneRunLines` was in `paneline_test.go` and `resolvedTypedLine` in
`codexwritable_test.go`, both arm 1 only — which is why ranger-base-6ca29 added
a third whole-log reader rather than reach for them from a shared file. Both
declarations MOVED (not copied) into herdr_test.go, so there is still one
reader per question. Moving a function between files changes nothing
`armtags_qa_test.go` pins: that partition is over FILES, and every file stayed
in its arm.

`launchLines` is the two of them composed and nothing else, so no third copy of
either rule exists. **It is why `TestPersonaLaunchRuntime`'s relaunch count
could be fixed at all**, and that site is the one place in the class where
`launchLog` is not the answer either: the claim is that TWO launches carried the
codex line, h2's spilled script is REWRITTEN at the relaunch, so a spilled pair
leaves ONE copy and a count over launchLog reads 1 where a count over typed
lines reads 2. Counting resolved `pane run` lines reads 2 either way.

`unattended_test.go`'s count did NOT need it, and the difference is worth
keeping: its two launches are two different SESSIONS (`bare` and the `qa-bare`
recipe), so they spill to two different scripts and the total is two whichever
place each one landed in.

## Verified

- `make tree-check` — 41.5s, green, 148 pins over 17 doors.
- `make register-check` — green after the head-comment enumeration and the
  pin count in `treewidedoor_qa_test.go` were updated (both reds were the
  register doing its job).
- The six fixed tests plus `TestPaneLine*`, `TestCodexRelaunchLineNamesTheStoreOfRecord`,
  `TestEveryLaunchPathTypesTheMode`, `TestDispatchRelaunchesDeadAgent`,
  `TestDispatchTierReachesSession`, `TestSeatbeltProfileAndLaunch` and
  `TestDispatchRefusesDegradedUnlessAllowed` — green in all three arms, under
  the default TMPDIR and under the 138-byte one.
- `make fmt-check`, `go vet` in all three arms.
