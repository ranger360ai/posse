## A chdir that bound nothing, a path pasted into a pattern, and five arguments nothing could fail (ranger-base-7ebv6)

ranger-base-751ha verified four closes and found eight things. Three were
defects in shipped behaviour; five were claims a close argued at its own site
with no test able to fail them. This is what each one turned out to be, and
what now holds it.

MEASURED 2026-10-09, darwin 25.4.0 / APFS, git 2.50.1 (Apple Git-155), bd
0.50.3 pinned.

### 1. `BEADS_DIR` outranks a chdir, so `store_owner` bound nothing

ranger-base-9mjxb's answer to github issue 9 was to stop handing bd a
store-selecting flag: name the repo that OWNS the store, `cd` into it, and let
bd resolve its own. The audit, the census pin and the ORDERS bullet were all
right about the flag, and `scripts/prune-bd-relates-to.sh` did the chdir — and
then ran bd with the one environment variable that is read BEFORE the cwd
still in its environment. In no-db mode bd resolves `$BEADS_DIR` and reads no
redirect at all (ADR 0055), and posse sets it in **every** session it
launches, so the three writes went to whatever store the environment named.
Measured live in the session that wrote this fix: `BEADS_DIR` named the
`.beads` of the store of record for the directory the session was launched
into, which on this instance is a third repository and not the one the script
was aimed at.

The remedy was already written down twice — AGENTS.md's bullet
(`env -u BEADS_DIR bd …`) and `internal/posse/beads.go`'s `bdStoreEnvShed`,
measured a month earlier under ranger-base-k45gn — which is why the Go runner
was clean and the script was not.

**Why the existing pin could not see it.** `TestQAPruneRelatesToAppliesFromThe`
`StoresOwnRepo`'s fake bd resolved `$PWD/.beads` and nothing else, while
`prunRun` passed the runner's own `BEADS_DIR` straight through. So the pin
asserted a property the real bd does not have, under the environment the pin
itself was handing the script. The fake now resolves `$BEADS_DIR` first, as
the real bd does; `prunRun` strips the inherited one so an arm that does not
name the variable measures the script and not the box; and the pin's fixture
aims it at the *other* repo, so every assertion about "the repo we stood in"
is also one about the store the environment named.

The lesson is general and worth more than the fix: **a fake that models only
the mechanism the bead was about is a fake that can certify the bug beside
it.** Dropping the shed now reds four assertions, one of which prints the
other repository's store as the one bd resolved.

### 2. A declared path is not a gitignore pattern

`seedScaffoldExcludes` pasted a `worktree_link:` path into a pattern body
unescaped. A declared `cfg[1]` wrote `/cfg[1]`, a character class: it matches
`cfg1` and does not match `cfg[1]`. Both directions wrong from one line — the
scaffolding it was written to hide still read `?? cfg[1]`, so the spurious
closed-dirty P1 survived for that path, and a collateral `cfg1` the seat wrote
went invisible to git, which is the silent-loss direction `seedScaffoldExcludes`'
own comment calls the worse of the two.

`gitIgnoreLiteral` (internal/posse/skills.go) spells a path as a pattern for
that path and nothing else. Four bytes carry meaning in a pattern body —
`*`, `?`, `[` and the escape `\` — and two more only at the start of a line,
`#` and `!`, which cannot be reached because every pattern is anchored with a
leading `/`. A trailing space is escaped (git strips an unescaped one). A path
holding a newline is REFUSED rather than written: the file is line-oriented
with no quoting, so the line it would write is a pattern for something else, in
the operator's own file.

| pattern written | ignores | leaves visible |
|---|---|---|
| `/cfg\[1]` | `cfg[1]` | `cfg1` |
| `/a\*b` | `a*b` | `aXb` |
| `/q\?z` | `q?z` | `qYz` |
| `/back\\slash` | `back\slash` | `backXslash` |

Two things found while pinning it, both worth keeping:

- **An operator who ignores `cfg[1]` has to escape it too.** A `.gitignore`
  line reading `cfg[1]/` ignores `cfg1/` and leaves `cfg[1]` dirty. The
  fixture spells the operator's own ignore `cfg\[1]/` for that reason, which
  is also what makes finding 3's reading admit the pattern at all.
- **`git status --porcelain` C-QUOTES a path holding a backslash**
  (`?? "back\\slash"`), so a substring search for the path's own spelling
  reads a dirty file as a clean one. A mutant that escaped only `[` went
  GREEN through that hole until the per-path assertions moved to
  `git check-ignore`, which is git's own answer and carries no quoting.

Dedup is over the file's own lines, so a tree that already carries an
unescaped line for a metacharacter path keeps it and gains the escaped one
beside it. Nothing rewrites that file — two launchers append to it
concurrently and the operator may be editing it — and no path on this instance
holds a metacharacter, so the repair is the operator's one-line delete.

### 3. The exclude file the operator's own checkout reads

`info/exclude` is the COMMON one; git reads no other (a `/.bob` written into
`.git/worktrees/<name>/info/exclude` left `?? .bob` in that worktree, measured
under ranger-base-e01op). So every line written for a session tree is read by
the main checkout. A declared path that checkout held untracked and UN-ignored
was therefore silently ignored there from the first session tree onward:
`?? .secrets/env` before seeding, nothing after, `!! .secrets/env` under
`--ignored` — so a `git add -A` salvage in the main checkout would have
skipped it.

That is posse deciding an ignore for the operator's own file, which is exactly
what the same commit declined to do for `.beads/` and for a tracked declared
path. `ignoredInMainCheckout` now admits a pattern only where it costs that
checkout nothing: the main checkout does not have the path at all, or git
there already ignores it.

`git check-ignore -q` is the reading, measured rather than reimplemented: over
a repo whose `.gitignore` says `.bob/` it exits 0 for the bare directory name
as well as for `.bob/` and `.bob/x`, and exits 1 for a tracked path and for an
untracked un-ignored one. A tracked path is thus withheld, which is right
twice over — an ignore pattern does not reach a tracked file, so the line
would buy nothing and only widen what the operator's checkout hides.

**The disclosed cost, pinned beside the rule:** in a repo whose main checkout
shows the scaffolding path as ordinary dirt, the session tree keeps its `??`
line and ADR 0041's closed-dirty check can still raise it. That is the
under-exclude direction — a visible file, not a hidden one — and it is the
half of the trade that loses nobody's work. It costs this instance nothing:
`worktree_link:` is documented for gitignored paths, and both live repos
ignore `.beads/` in the checkout.

### 4-8. Five arguments nothing could fail, and what each one was

The bead called all five "argued conditions nothing can fail". Measuring them
one at a time put them in three different classes, and the difference matters
more than the count.

| # | site | what it turned out to be |
|---|---|---|
| 5 | `memoryland.go` `sort.Strings` | a real property, argued for the WRONG reason |
| 7 | `queuejsonl.go` `hookRepo(q)` | a real property, pinnable, now pinned |
| 8 | `memoryland.go` `ValidName` | a real property, pinnable, now pinned |
| 6 | `govern.go` `SweepHookWallIdentity` | a real decision with NO behavioural discriminator |
| 4 | `pidcheck.go` `key == ""` | an UNREACHABLE arm: the mutant is a no-op |

**5. The sort's own reason was not the right one.** The site said persona order
holds the pulse's fingerprint stable. It cannot: `ShopCheck` sorts the whole
governance set by `Key` before returning it, and a G14 key is
`memory-unlanded:<persona>`, so that order is settled downstream whatever the
reading does. What the sort actually decides is the order of the STRANDS — a
slice that is printed, and that `MemoryDirtyPaths` is compared against row by
row. The comment now says that, and the pin asserts the order.

Pinning it needed a number, not an assertion: Go randomizes the START of a map
range, so a small map iterates as a random rotation of insertion order — which
is `git status`'s own, already sorted — and a single read of the existing
two-persona fixture comes back sorted by luck half the time. Six dirty
personas in one bucket leave three admissible starts that are sorted, so
twenty-five reads put a mutant's survival at about 2e-11. The mutant's first
unsorted read landed on draw 1.

**6. A decision with nothing to discriminate it.** G12 reads
`SweepHookWallIdentity` because it runs on a ticker: the ADR 0023 pair costs
3.41/3.54/3.62s a sweep against 375/396/393ms for identity alone, paid every
30s by the cockpit and every two minutes by the pulse. Swapping the pair back
in survives all sixteen G12-G15 pins — and no fixture can red it, because the
two readings disagree only where the behavior half FAILS, and it fails for a
renderer regression (a property of this binary, identical in every repo), a
missing `sh`, or a scratch dir that cannot be made. One binary-wide and two
process-global: none of them a thing a fixture can arrange for one repo
without reaching into the environment every other test in internal/posse
shares. Over every repo state a fixture CAN build, both sweeps return the same
verdict, so a pin that called the governance set would be green under either
call.

So this one is held by reading the source, in internal/treepins, with the
measurement asserted beside the choice — the next hand to touch that line has
to walk past the numbers that decided it. It is the exception and not the
habit, and the reason is written in the pin.

**4. An arm nothing can reach.** The credential lint's "no session credential
is decided for `<runtime>`" arm is selected by
`if key := CageCredential(own); key == ""`, and the bead reported it reachable
through an operator runtime yaml declaring `cred_bin:` with no `cage_cred:`.
There is no `cred_bin:` yaml key. `CredGateCollision` returns "" unless the
runtime declares a `CredBin`; exactly one runtime does (the claude built-in,
`security`); and its session credential is a const in `cageCredential`. So
`key` is never empty where the guard is asked, the two branches are one
branch, and mutating the guard to `false` is a NO-OP mutant — a different
thing from an unheld property, and not a defect.

Two things can fail instead, and both now do:

- the sentence itself, asked of `CredGateLint` with a synthetic runtime in
  that state (the function had no direct caller in any test), including that
  it does not print the other sentence — the one with a hole where the key
  name goes, which is what the mutated guard produces; and
- the reachability premise, so "unreachable today" cannot rot in silence: no
  loadable runtime declares a credential binary without a decided credential,
  and a runtime yaml cannot declare one. When either stops holding, the arm
  goes live and the test's failure message says what the next hand owes.

### What this bead changed

| file | change |
|---|---|
| `scripts/prune-bd-relates-to.sh` | `env -u BEADS_DIR` at the three writes and in the printed recipe |
| `internal/posse/skills.go` | `gitIgnoreLiteral`, applied to every pattern |
| `internal/posse/worktree.go` | `ignoredInMainCheckout` gates every scaffolding pattern |
| `internal/posse/memoryland.go` | the sort's reason, corrected |
| `internal/treepins/bdrelatesprune_qa_test.go` | the fake bd resolves `BEADS_DIR`; the pin's fixture sets it |
| `internal/posse/skills_test.go` | `gitIgnoreLiteral` asked of git, four byte classes |
| `internal/posse/worktree_test.go` | a metacharacter in the porcelain fixture; the main-checkout rule |
| `internal/posse/govern_test.go` | G13's linked worktree, G14's order, G14's non-persona dir |
| `internal/posse/pidcredgate_qa_test.go` | the undecided sentence, and its reachability premise |
| `internal/treepins/govtickreading_qa_test.go` | G12's reading, and the measurement beside it |

Two live-defect pins committed under ranger-base-751ha are deleted, their
behaviour folded into the pins their own failure messages named:
`internal/treepins/bdstoreenvhole_qa_test.go` and
`internal/posse/scaffoldexcludehole_qa_test.go`. That is the inversion working
as designed — a pin that ships green over a hole and tells its reader where
the behaviour belongs once the hole is closed.
