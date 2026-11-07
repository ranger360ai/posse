# What codex echoes in `hooks/list`'s `cwd`, and two argv the tables had no row for (ranger-base-5x8xi)

2026-11-07. The three live findings from `ranger-base-rucsu`'s verify of four
closes, all fixed here: D7's trust ask never read the directory its answer was
about, `posse hook-report --ask` with no app name answered exit 0 over an empty
stdout, and the app-name pin could not fail on the first byte its own doc
names. Findings and repros are on `ranger-base-5x8xi`;
`docs/notes.d/ranger-base-rucsu.md` has the verify tables that produced them.

## Finding 1 — the ask now vets the directory its answer is about

`internal/posse/codextrust.go`, `codexAskOver`. The ask sends `hooks/list
{cwds:[<the seat's cwd>]}` and decodes `result.data[]`, one entry per asked
cwd. posse asks about exactly one directory, so an entry naming another — or
naming none — is an inventory of a different seat: the hooks *this* seat's
project declares are not in it, so they stay untrusted, codex draws the
startup review dialog, and the seat submits nothing and reports nothing. That
is what every other refusal in the file exists to prevent, and the
duplicate-key refusal already said the reasoning in words ("posse asks about
ONE cwd") — it was the one place the reasoning was written and the `cwd` field
was not read.

The guard is `inv.Cwd != cwd` in the decode loop, and it refuses naming both
paths, because that sentence is the whole diagnosis for a launch that
otherwise looks like nothing at all.

### Why a BYTE compare, which is the reading the pin could not take

The finding said to measure this first and not to prescribe the compare: posse
hands over an absolute, symlink-resolved path (`resolveLinks`, in both
`runCodexHooksAsk` and `RexReportLine`), and nothing on record said whether
codex echoes it byte for byte — so a strict equality compare could have
refused a launch that was fine.

**MEASURED 2026-11-07**, codex-cli 0.153.4, darwin 25.6.0, from this session
worktree, through a direct `codex app-server` client (`initialize` +
`initialized` + `hooks/list`, no prompt submitted) in a scratch `CODEX_HOME`
whose only provider is a dead loopback port with an `env_key` that is never
set. Six readings:

| asked | echoed | byte-identical |
|---|---|---|
| `<W>/seat` | `<W>/seat` | yes |
| `<W>/seat/` — trailing slash | `<W>/seat/` | yes |
| `<W>/seat/sub/..` — uncollapsed | `<W>/seat/sub/..` | yes |
| `<W>/link` — a symlink to `<W>/seat` | `<W>/link`, NOT resolved | yes |
| `<W>/broken` — a `hooks.json` codex cannot parse | `<W>/broken` | yes |
| `<W>/seat` with project trust on the line, one hook discovered | `<W>/seat` | yes |

The reading did NOT come from `docs/adr/0076-trust-ask.probe.sh`'s `state`
arm, which the finding pointed at: that arm prints the inventory posse's ask
logs on stderr, and `codexAskNotes` — the only thing that reads `inv.Cwd` —
prints a line per `error` and per `warning`, so an answer with neither says
nothing about the echo at all. A direct client was needed to see the field.

`hooks/list` echoes the asked path **verbatim**, with no normalization of any
kind, and the echo does not vary with what the answer carries. So equality
cannot refuse a launch that was fine, and anything weaker admits a
neighbouring path — MEASURED: a `strings.HasPrefix(cwd, inv.Cwd)` compare in
place of equality accepts the empty-`cwd` answer, and reddens only that arm.

### The pin, inverted

`internal/posse/verify_rucsu_qa_test.go`,
`TestQACodexAskVetsTheDirectoryItsAnswerIsAbout` — `ranger-base-rucsu` shipped
it GREEN asserting the hole, with the inversion written into each failure
message; this is that inversion. Five arms, and the two controls are
untouched, because they are the half that says the compare landed in the right
place rather than refusing everything:

| arm | asserts |
|---|---|
| control: the answer is about the seat's own cwd | renders a trust line |
| an answer about another directory | refuses, naming that directory |
| an answer naming no directory (`data[]` non-empty) | refuses |
| control: two entries both about the seat's OWN cwd | still refuses on the KEY, not the cwd |
| two entries, one of them another directory, the extra declaring no hook | refuses on the DIRECTORY |

The fourth arm is what bounds the change: it is the shape the duplicate-key
guard already covered, and it must still be refused by that guard, so the new
compare has not widened past what it was scoped to. The fifth is the shape the
finding was about that nothing covered — before this, a two-entry answer was
refused only *incidentally*, by a duplicate key the second entry's
session-flags layer happened to repeat, so an extra entry declaring no posse
hook at all went through.

## Finding 2 — the `--ask` arm owns its own arity

`internal/posse/codexreport.go`. The route was `len(args) >= 2 && args[0] ==
HookReportAskFlag`, so a bare `--ask` fell past it into the shared wrong-argv
arm, whose `return 0` is the HOOK path's rule (codex reads 2 from a hook as
"block this prompt"). Exit 0 over an empty stdout is the one answer this arm
must never give: `runCodexHooksAsk`'s own doc says "a refusal that exited 0
would be a launch script's green light over an empty trust line".

**MEASURED 2026-11-07**, through a `go build`'d `cmd/posse`:

| argv | before | after |
|---|---|---|
| `hook-report --ask` | exit 0, empty stdout, the SHARED usage | exit 1, the ask arm's usage |
| `hook-report --ask codex -c` | exit 1, the ask arm's usage | unchanged |
| `hook-report --ask codex model=x` | exit 1 | unchanged |
| `hook-report --ask codex -c a=1 -x b=2` | exit 1 | unchanged |

It is two edits and the second is not cosmetic: the route is now `len(args) >=
1`, and `runCodexHooksAsk` grew its own `len(args) == 0` check, because
`args[0]` on an empty slice panics and `RunHookReport`'s recovery turns a panic
into **exit 0** — the very answer the arity change exists to stop. MEASURED:
with the route widened and the inner check dropped, `--ask` exits 0 with an
empty *stderr* too. One pin row kills both halves.

The pin is one row — `{"posse", HookReportVerb, HookReportAskFlag}` — at the
head of `TestHookReportAskArmRefusesAnUnpairedConfigValue`
(`internal/posse/codextrust_test.go`), beside the dangling `-c` it belongs
with. `hook-report` with no args at all is unchanged (shared usage, exit 0,
pinned by `TestHookReportCannotExitTwoByCrashing`), and no hook ever types
`--ask`, so nothing on the hook path moved.

## Finding 3 — the app-name pin can fail on a lone colon

`internal/posse/codexreport_test.go`,
`TestHookReportRefusesAnAppNameThatIsNotOne`. Its doc names the colon first —
"`:` separates pairs and `=` separates one" — and all seven of its rows carried
a `=`, a space, an escape, a newline or a slash BESIDE the colon, so every row
was refused for the other byte and the colon was never isolated. **MEASURED
2026-11-07**: admitting `:` in `plainStatusToken` left the test `ok`.

One row, `"codex:x"`. It is a table row and not a defect: the byte class is
pinned by `TestCodexHooksAskRefusesBeforeStartingAnythingItCannotAsk`'s
`codex:evil` row. With the row in place the same mutant reddens and names what
it cost — `\x1b]7501;state=idle:app=codex:x\x1b\\`, which is an extra pair in
somebody else's report.

## Mutants

Five, each killed, each reddening only its own arm:

| mutant | killed by |
|---|---|
| drop `inv.Cwd != cwd` | the three refusal arms of the inverted pin; both controls stay green |
| `strings.HasPrefix(cwd, inv.Cwd)` instead of equality | the empty-`cwd` arm alone |
| revert the ask route to `len(args) >= 2` | the new `--ask` row (exit 0, shared usage) |
| drop `runCodexHooksAsk`'s own `len(args) == 0` | the same row (exit 0, empty stderr — the panic path) |
| admit `:` in `plainStatusToken` | the new `"codex:x"` row |
