# The sandbox grant pins did not pass on a bare /tmp — they skipped, 123 of them (ranger-base-u8xk4)

*2026-11-11 · dinesh · from ranger-base-w5z7u, whose fix had to route around
this · touches internal/posse's seatbelt grant pins and one production
seam (`seatbeltTempGrants`)*

## What the bead said, and what was actually there

The bead's reading was that several grant pins were satisfied by the blanket
`/tmp` every rendered profile carries, so they could not fail for the reason
they exist. That is the right diagnosis of the mechanism and the wrong
reading of the symptom. On this box the pins did not pass: **they skipped.**

`sbRoot` (internal/posse/seatbeltconstitution_qa_test.go) builds every
sandbox grant fixture — its own HOME, its own posse home, its own state dir
— and its contract is that nothing in that fixture is reached by a grant the
test is not about. The profile carries three blanket temp grants (TMPDIR,
`/tmp`, `/private/tmp`), so the helper pointed TMPDIR at a SIBLING of the
root. Then it checked whether the root had nevertheless landed under `/tmp`,
and if so:

    t.Skip("TMPDIR resolves under /tmp; the profile's blanket temp grant
            would cover the fixture")

The root comes from `t.TempDir()`. With TMPDIR unset — this box, and
ci.yml's ubuntu legs — `t.TempDir()` is under `/tmp`, so that line fired
every time.

MEASURED 2026-11-11 (linux 7.2.15 x86_64, go1.27.3, TMPDIR unset), counting
the skip message over a `-v` run of each arm:

| arm | tests/subtests skipped on that line |
|---|---|
| arm 1 | 53 |
| arm 2 | 22 |
| arm 3 | 48 |
| **total** | **123** |

Across the 14 QA files that call `sbRoot`: credentialdenymove, promotedprose,
promoteruntimes, reachability, reachclassifier, seatbeltatomicsiblings,
seatbeltcarveout, seatbeltconstitution, seatbeltcredentialread,
seatbeltgitidentity, seatbeltinstancestate, seatbeltpathscoped,
seatbelttelemetry, seatbeltworktreegit. Every sandbox grant pin in
internal/posse, measuring nothing, in silence, on the only box that runs
them. A skip reads as a pass in `make test`'s summary line, which is why
three beads in a row had looked at this class and seen green.

## The fix: where the box cannot move the fixture, move the GRANTS

`seatbeltTempGrants(goos, tmpdir)` (internal/posse/seatbelt.go) is new and is
a pure refactor of the four lines that added those three grants — same
members, same order, same profile. It is a **var**, for the reason
`bwrapResolves`, `bwrapApplyRefusal`, `bwrapBin` and `wallMechanismHost` are
vars: the arm a pin needs is one the box will not provide. `sbRoot` now calls
`sbNoBlanketTempGrants`, which stubs it to nil for the test **when and only
when the fixture root is inside one of those grants**. The fixtures below
then measure the profile a launch outside a temp root would have had — the
same premise reached the other way round, needing nothing of the box.

On darwin the stub is not installed: TMPDIR is under `/var/folders`, the
placement holds, and the grants stay the real ones, because that premise is
stronger and it is the one a kernel reads. The one host whose RENDERED
profile this changes is a darwin box whose operator exported `TMPDIR=/tmp`,
which before this was a host where every execution pin here graded a wall
the blanket grant had already opened.

The subtraction the old comment rejected ("filtering the blanket grants out
of the set afterwards would have been the same thing said less carefully")
was built first and then thrown away: it needs every call site to remember
it — 22 `sbCovers` sites, plus `ConstitutionGrants`, `writeGranted`,
`renameSeal` and `SeatbeltCarveOut`, each taking the set as an argument —
and it cannot reach a pin whose subject is a production REPORT that derives
the set internally (`SeatbeltReport`, two pins). One knob at the source
reaches all of them and cannot be forgotten.

`sbRoot`'s contract is now a test of its own,
`TestQASbRootHandsOutARootNoGrantReaches`: a path under the root it returns
is reached by nothing in a rendered set, and on a host that grants a
directory the root is inside, the set carries no blanket grant at all.

## What 123 restored assertions turned up

Every red below was green before only because the pin did not run. None is a
regression; each is a claim that had stopped being true, or had never been
true on this GOOS.

- **`TestQAStateStaysWritableAndIsDerivedFromTheHome` asserted a grant
  ranger-base-6akzw deliberately removed.** It required `a.StateDir` and
  `state/skills` to be writable — which is hoover's finding
  ranger-base-y438o stated as a requirement. 6akzw took posse's state dir
  out of the write set (launch lines a pane sources, session records
  carrying a seat's `cage:`, rendered skills trees, the dispatcher's files,
  work prompts: all run or trusted later by something outside every wall)
  and kept the seat's own gates dir and its two audit logs. The pin now pins
  that, both ways, and the security claim 6akzw landed has a pin in this
  file for the first time.
- **`TestQAGrantStillReachesTheGitConfigInEveryShape` declared the
  `deniesFiles` shape a deny-over-a-grant row on the strength of a
  `<cwd>/.git` grant ADR 0086 D3 had already dropped.** Which wall each of
  the four shapes grades is now DATA on `giShape` (`wallIsOmission`) rather
  than a hardcoded shape name, so a grant that widens or narrows reds the
  premise pin by shape name instead of silently moving what the execution
  table measures. The execution table itself was already correct — it asks
  `reachesConfig` per row.
- **Two pins asserted darwin-only spellings of GOOS-parameterized
  decisions.** `seatbelttelemetry_qa_test.go` wanted
  `~/Library/Application Support/go/telemetry/local` covered on a box whose
  set holds `.config/go/telemetry/local`; `goTelemetryLocal` and its
  unrelated-neighbour paths are per-GOOS now. And
  `TestQATheRenderedProfileNamesTheMovedFileForEitherVariable` rendered for a
  seat declaring `~/.claude`, which ADR 0080 D19 SPARES off darwin — it
  renders for a codex seat there, mirroring the two linux rows of the table
  above it, and asserts its own premise so a declaration that stopped naming
  a non-claude store cannot turn it back into the spared arm.
- **Two reachability pins drove the wrong mechanism.** ADR 0080 D18 gave
  `applyRecordReach` a bwrap arm, and `rchFakeApplyRefusal` stubbed only
  `sandboxApplyRefusal` — so on a Linux box the abstention under test never
  happened and `Detail` was `""`. It stubs both readers now, which is
  `wallwitness_qa_test.go`'s own spelling for the same pair, and
  `rchBlockedRender` asks for the render this host would really do. The
  sandbox-exec-only tail stays behind `SeatbeltAvailable()`.
- **`TestQAConfigDenyAddsNoNewRenameSeal` compared two empty lists in two of
  its four shapes** (a worktree and a whole-tree-deny PID seal nothing,
  which is the claim). It now asserts out loud that at least one shape
  produced a seal.
- Four more came back green on the restored premise with no change at all:
  `TestQACarveOutSealsTheAncestorsAGrantMadeRenamable` and
  `TestQACarveOutIsSmallForAnOrdinaryProject` (`renameSeal` walks up while a
  grant covers the parent, so under `/tmp` the walk never stopped),
  `TestQAOnlySubtreeGlobsReachTheProfile/bare_spelling` (its claim is that
  cwd is OUT of the allow block), and the three `ConstitutionGrants` pins
  plus the two `SeatbeltReport` ones, whose fixture home was reached by
  `/tmp`. `TestQAGatesAllClearNamesTheWholePromotedSet`'s own comment had
  already named this — "every scratch home sits under a granted temp tree,
  so the bad-grant branch answers instead" — and said the branch was
  reachable in-package. It was reachable on darwin.

## Mutations

Both MEASURED 2026-11-11 on this box, over the arm-1 `sbRoot` families:

- `sbNoBlanketTempGrants` turned into a no-op: **14 reds**. The premise is
  load-bearing and the restored assertions bite.
- `add(gatesDir)` removed from `SeatbeltWritable`:
  `TestQAStateStaysWritableAndIsDerivedFromTheHome` reds on four lines. The
  same mutation was invisible before, because the pin did not run.

## What this buys ranger-base-w5z7u's lane

w5z7u needed `make coldexec-check`'s arms to run somewhere with room and
could not simply give them a TMPDIR with room, because this class red
whenever TMPDIR left `/tmp`; its fix nests the arms' temp dir UNDER `/tmp`
instead, which buys no space. With the premise now arranged by the test
rather than by the box, a seat whose `/tmp` share is spent can move TMPDIR
off it and still read a true verdict from this class.

MEASURED 2026-11-11, `TMPDIR=/var/tmp/…` (a filesystem that is neither
`/tmp` nor under `$HOME`, so neither of ranger-base-akk9f's two workaround
classes applies): all three arms of internal/posse green — arm 1 151.0s,
arm 2 58.4s, arm 3 87.5s. On the house default the same three read 155.2s,
55.1s and 83.1s inside a green `make test` (10m36s, exit 0, zero failures).
That is this class and not the whole suite — see "Not closed here".

## Not closed here

A moved TMPDIR is not yet a clean full suite, and the rest of what reds
there is not this class. ranger-base-akk9f measured 8 reds in 4 packages
under `TMPDIR` on `$HOME`; the ones outside internal/posse are a different
mechanism — a surface that abbreviates `$HOME` to `~` while the assertion
wants the absolute path (`AbbrevHome`, not a grant), and two cmd/posse pins
that shell out to `go build` and so cannot be read on a box whose `/tmp` is
full in EITHER root. They live in cmd/posse and cmd/coldexec, not under
`sbRoot`, and nothing here touches them.
