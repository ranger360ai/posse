# Outside-in review of posse — 2026-09-09 (grok)

Landing path: `docs/notes.d/ranger-base-vuosd.md`. The bead named
`docs/reviews/2026-09-09-outside-in-grok.md`; ADR 0024 D2 check 1 refuses a
new file under `docs/reviews/` (`PublicDocsGenres` is `adr`, `runbooks`,
`notes.d`, `probes`). This session does not edit `visibility.go` (the bead
is read-only on code) and does not pass `RHQ_VISIBILITY_OVERRIDE` (that
override is operator-typed, never a dispatched seat). Adding `reviews` to
the allowlist is a separate, reviewed code change.

Reviewed at `f97cf1fc` (session worktree tracking `origin/main`). Stance: first look at what is on disk and in git, not at what the beads or ADRs promise. Optimism, tooling claims, and prior verdicts get the same look. Twin review unread.

## Verdicts

This is not a thin harness. README and DIRECTION still sell a small binder over herdr (panes) and beads (the work graph). On disk it is one Go module with two `package posse` names, **76,138 lines of product Go in 123 files**, **199,309 lines of test Go in 571 files**, and essentially one importable engine: `internal/posse` (117 product files, 69,306 lines). `dispatch.go` opens "Small on purpose: the substrates do the hard parts" and then runs 5,489 lines / 127 functions. `gates.go` is 5,796 lines of PATH-shim generation and argv matching. A newcomer who believes the architecture diagram will not recognise the tree.

The last fourteen days are almost the whole public life of this repo (published 2026-08-23; 1,744 commits total; **1,629 of them since 2026-08-26**). Since `v0.4.0` (2026-08-29, `feaf3010`) product Go went 40,111 → 76,138 (+90%), tests 81,995 → 199,309 (+143%), QA tests 34,353 → 111,826 (+226%). Version is still `0.4.0`. CHANGELOG `## Unreleased` is ~1,970 lines. The shop can delete when it measures — paid overflow, the herdr event subscriber, and the pulse doubling ladder are actually gone — and those three cuts are the exception. Keyword-sorted, the 1,629 subjects are ~407 docs/ADR, ~333 test/arm/suite, ~279 pin/QA, ~36 simplify/retire. **The codebase is getting busier.** Some of that is compounding (worktree, gates, credentials, cage: each incident added a case to a wall that has a caller). Some of it is racing (pins that pin comments, Makefile recipes, other pins, ADR status lines). Both are happening. Racing is winning on volume.

Test discipline is the most serious engineering in the tree and also the load-bearing cost. Three build-tag arms, mutation-checked partition pins, Makefile doors for tree-wide pins, `verify-parallel`, a silent-revert audit, a box-wide suite lock: all of these answer measured failures (a bare `go test ./...` is not the suite; `go test`'s 10-minute default has already timed out this package; five concurrent full suites stalled the shop; a revert that also skipped its pin is invisible to a green run). That is KEEP. What grew around it is a second product: **1,120 `TestQA*` functions in 245 files**; 328 of 818 files added in fourteen days are `*_qa_test.go`; the Makefile went 422 → 986 lines, 23 `verify-*` targets and 9 `*-check` doors, so that a pin whose subject is the tree remains reachable. A stranger cannot contribute through CONTRIBUTING's `make test` without a 25-minute three-arm run and a working herdr/bd install.

A newcomer cannot understand this tree from the tree. NOTES.md is 7,473 lines (5,046 at `v0.4.0`). `docs/notes.d/` is 137 fragments, all from this window. 58 ADR markdown files, many still carrying full superseded bodies under a pointer. The repo root is one `embed.go` plus 55 test files. Every large product file opens with a multi-screen incident novel. HISTORY.md says `ranger-base-*` ids are inert provenance that will not resolve here; they are the only `why is this line here` the git log has (1,421 of 1,629 subjects). DIRECTION.md still talks about a 2×2 grid and a small dispatch loop. `posse help` still tells you `--land` never removes a tree, then documents `--retire` which does.

Risk, in three lines. **I would not ship current `main` as a release** to anyone who is not already this shop: still stamped 0.4.0, Unreleased is a book, beads is pinned to 0.50.3 exactly, herdr is mandatory, the house suite is a box-sized machine. **I would not ship L1/L3 as a security boundary** — the code already says L1 is a cooperative PATH shim and L3 fails open on an empty environment; L2/L4 are the actual fences. **I would delete the comment-and-pin amplification before deleting product.** The one thing I would fix first is a write freeze on new QA pins whose subject is prose, a Makefile comment, or another pin. ADR 0006 §6 already ruled that a finding whose fix runs nothing is a comment on the verify bead. The tree did not listen. Until that loop stops, splitting `dispatch.go` is rearranging furniture in a house that is still being extended.

## What I ran

| command | result |
|---|---|
| `make fmt-check` | exit 0 |
| `make tree-check` | exit 0 (seven door runs, 0.3–2.5s each) |
| `go build -o /tmp/posse-vuosd-review ./cmd/posse` | exit 0; `posse version` → `0.4.0+dev` (unstamped `go build`, as `version.go` documents) |
| `go test -count=1 -timeout 3m -run 'TestQAEveryPosseTestFileIsSharedOrInANamedArm\|TestQAEverySuiteArmTypeChecks' .` | `ok` 4.003s |
| `make verify-parallel` | `2189 eligible tests` all carry `t.Parallel` |
| `scripts/suite-lock.sh --status` | both slots free (I did not take one) |

I did **not** run `make test`, `test-arm2`, `test-arm3`, or `test-linux`. Each arm takes a box-wide suite slot and a 25-minute timeout; this review changes no product code. Last simplification landings on this history already recorded full-suite greens (e.g. `953f0be4`: internal/posse 1192.9s). I did not run `posse dispatch`, `posse new`, `bd daemon`, or anything that hires. I did not live-exercise the container cage.

Counts below are MEASURED from this tree and `git` unless tagged ASSUMED.

---

## Findings

### 1. KEEP — The substrate split at the edges is real

`internal/posse/herdr.go:3` and `beads.go:3` are what DIRECTION.md claims: posse shells out to `herdr` and `bd`, and does not reimplement panes or the issue graph. `go.mod` has two dependencies (`golang.org/x/sys`, `golang.org/x/term`). Runtime is a named profile (`runtime.go:3–14`), not a hardcoded CLI. Promote-from-commit, constitution hashing, and human-only `make install` are real product (README; `promote.go`). `App` injects clocks, load, CI, and reaping (`app.go:47–102`) so the suite is not the operator's box. That shape is the right one. The problem is everything that grew *between* those edges.

### 2. FIX — The "small dispatcher" sentence is false on disk

```3:4:internal/posse/dispatch.go
// The dispatch loop — the harness core (DIRECTION.md). Small on purpose:
// the substrates do the hard parts.
```

`DIRECTION.md:122–123` repeats it. The file is 5,489 lines, 127 functions, at HEAD. At `v0.4.0` (`internal/rhq/dispatch.go`) it was already 3,910. `cmd/posse/main.go` is a 2,646-line switch over ~40 verbs (sessions, dispatch, backup, cost, scorecard, gates, cage, promote, pause, skills, …). A first-time reader who starts at DIRECTION will design as if the loop were a page of pseudocode (`DIRECTION.md:125–133`). It is not. Fix the sentence to match the file, or cut the file to match the sentence. Leaving both is how the next incident becomes another 400 lines in the same SCC.

### 3. KEEP, with a cost — Isolation is the load-bearing product decision

`worktree.go:1–32` is the honest architecture: N personas cannot share one index and one HEAD. Session worktrees, `posse/<session>` branches, launcher merge-back, path-limited commits, deny-wins PIDs — this is why the shop functions at all. It is also why the harness is 76k lines. The file grew 1,125 → 3,208 since `v0.4.0` because isolation has a long tail (redirect, FIFO, `BEADS_DIR`, merge-back, retire, dirty close). That is compounding, not vanity. Keep it. Do not add a second isolation mechanism.

### 4. SIMPLIFY — One package, one strongly-connected component

`docs/notes.d/ranger-base-qp1hm.md` measured at `6f94a99c`: **101 of 112 product files in one SCC**; six files peel; nothing else does. HEAD has 117 product files in the same package. I did not re-run Tarjan (ASSUMED the graph has not grown a seam in three days). The three-arm test split exists *because* a sub-package split is closed at this price, and moving `*_qa_test.go` out is closed too (same note: QA files consume unexported identifiers and test-only helpers). Anyone proposing "just split the package" is proposing the bead that note already priced. The simplification that still pays is fewer things *in* the component, not a new directory.

### 5. KEEP — The suite's doors are answering real lies

`armtags_qa_test.go:12–35` names four silent failures a green run will not report, including a bare `go test ./internal/posse` that compiles no tests and still says `ok`. Current partition, MEASURED by build line on this tree:

| arm | test files | `Test*` funcs |
|---|---|---|
| 1 (default, `!posse_arm2 && !posse_arm3`) | 124 | 823 |
| 2 | 126 | 847 |
| 3 | 146 | 812 |
| shared (no tag) | 71 | 523 |

`verify-parallel` at HEAD: 2,189 eligible tests all carry `t.Parallel`. `Makefile:272–277` is the house command: three arms, 25-minute timeout, silent-revert audit, tree-check. CONTRIBUTING.md:9–23 is honest about this. Keep the doors. They are why `go test ./...` is documented as insufficient rather than quietly wrong.

### 6. DELETE — The pin-on-pin loop

1,120 `TestQA*` functions. 328 new `*_qa_test.go` files in fourteen days. Root package: 1 product file (`embed.go`) and 55 tests. `treewidedoor_qa_test.go` is a catalogue of doors to pins whose subject is the tree, living in a package nobody runs whole, because `-run` selects by name. `armtags_qa_test.go:216–221` exists so six *other* pins that read `make test`'s literal recipe stay true. `absencerules_qa_test.go:1–24` is the good form of the class (a syntactic census of a stated rule, shown able to fail). `herdrhintsgone_qa_test.go:8–15` is the good form of a removal pin (identifiers that must not grow back). The bad form is a pin whose subject is a comment, a status line, or another pin's recipe. ADR 0006 §6 (commit `cc3979a4`, 2026-09-06) already said a finding whose fix runs nothing is recorded on the verify bead, not filed. Fourteen days later the dominant added file type is still `*_qa_test.go`.

Delete, as a class, new pins that do not change what a process does. Existing ones: do not mass-delete in one sweep (the silent-revert audit exists because that sweep is how pins die). Stop minting them. A census that cannot fail except by editing the comment it holds is not a test.

### 7. KEEP — Measured deletion, when it actually happens

Three real cuts, all 2026-09-06, all measured *before* the delete:

- `a3852e2f` — `overflow.go` gone; plan guard parks, does not pick a paid provider.
- `953f0be4` — `herdrevents.go` gone; 0 settle-hint wakes in 142 passes / 15h31m on this shop's watch log.
- `d6185d09` — pulse record two fields wide; doubling ladder never bound in 117.3h of log.

Each landed with mutation pins and a CHANGELOG upgrading note. That is the shop's best habit. It is also rare: 36 of 1,629 subjects even *mention* retire/remove/simplify. `herdrevents.go` and `overflow.go` show as +0 net in a 14-day numstat because they were born and died inside the window — they were not a reduction of the `v0.4.0` surface, they were a feature that arrived after the tag and then left. Still: the deletes are real and the remaining truth tables (plan guard, watch timer, pulse delivery) look like they kept their jobs. Keep doing this. Do it to the pin loop next.

### 8. FIX — Last fourteen days bought a thicker wall, not a simpler product

Core file line counts, `v0.4.0` (`internal/rhq/`) → HEAD (`internal/posse/`):

| file | v0.4.0 | HEAD |
|---|---|---|
| gates.go | 2,011 | 5,796 |
| dispatch.go | 3,910 | 5,489 |
| herdrback.go | 2,424 | 3,583 |
| worktree.go | 1,125 | 3,208 |
| cockpit.go | 2,277 | 3,042 |
| main.go | 1,916 | 2,646 |
| runtime.go | 1,508 | 2,145 |
| credential.go | 928 | 1,733 |
| seatbelt.go | 715 | 1,585 |
| cage.go | 828 | 1,450 |

Net Go in the window: +54,271 product, +165,834 test. ADRs 37 → 58 markdown files. Makefile 422 → 986. NOTES 5,046 → 7,473. 1,236 commits since the tag, all under one human author name (the identity the launcher commits as; bead id in the subject is the actual provenance; HISTORY.md's "inert" claim does not describe how the tree is read). Commit rate 2026-08-30..09-01: 302; 2026-09-06..09-08: 191. The 09-05 ADR simplification (`b96a1080` and the fold commits) is the one day the *record* got smaller. The code did not. 09-08 is a single commit (`f97cf1fc`, arm type-check door). That pause is the interesting data point, not the 150-commit days.

What the two weeks bought that a deployer can feel: worktree isolation matured, overflow/fallback/event-hint/pulse-ladder complexity left, settings/hooks pinning, backup verb, CI-red bead, retire of dead trees (ADR 0058). What they bought that only this shop feels: the immune system around all of the above. Racing is the immune system treating every verify finding as a new organ.

### 9. FIX — A newcomer cannot learn this from the tree

- Two packages named `posse` (`embed.go:10`, `internal/posse/app.go:1`). Grep `package posse` and you are in both.
- Root of the module is a test dump. `embed.go` exists for a real reason (`go:embed` cannot leave its directory). The 55 tests beside it exist because tree-wide pins were homeless.
- `NOTES.md` is not a map; it is a memoir. `docs/notes.d/` is the overflow valve (ADR 0022), 137 files, not an index.
- 58 ADRs, no `docs/adr/README.md` on purpose (ADR 0040: `grep -m1 '^\*Status'` is the index). That is elegant for the author and hostile to a first open of `docs/adr/`.
- `DIRECTION.md:117` still says "crew = the 2×2 grid slots". Main is herdr-native; tmux-reference is retired in the same file (`DIRECTION.md:63–65`).
- `cmd/posse/main.go:2232–2235` (`posse help`): `--land` "never removes a tree — it cannot tell a dead session's from a live one's", immediately followed by `--retire` which removes one under ADR 0058. The first sentence is the comment ADR 0058 overturned; the help kept it.
- `HISTORY.md:9–18` vs AGENTS.md and 1,421 bead-id subjects: the public tree's provenance *is* the private tracker ids. Calling them inert is a licence statement, not a description of the artefact.
- `examples/config.yaml` is 875 lines, mostly comments, which is the right seed shape — and still a wall of instance knowledge a fresh deployer has to finish.

### 10. SIMPLIFY — Gates are an argv parser, not a security boundary

`gates.go:1–22` is clear: L1 is a POSIX shim on PATH, rendered from PID `deny:`, matching after global options. `gates.go:169–180` is a per-command table of value-taking global flags (`git`, `bd`, `posse`). The rest of 5,796 lines is the matcher, the renderer, hook text, parity claims, and the cases that escaped last month. ADR 0002 §3's own table says L1 is cooperative and L3 is a git hook; L2 (seatbelt) and L4 (container) are the enforced filesystem/egress. Historical ADR 0025 (superseded, still on disk) already measured `env -i` defeating L1/L3. Keep deny-wins PIDs and the shim *idea*. Do not let the next `git -C` cousin add another 400 lines to this file. I would not describe this stack as "the wall" to a security reviewer without leading with "cooperative, fail-open on empty env, L4 is the fence."

### 11. SIMPLIFY — Several verbs are a full subsystem around one idea

Not "delete tomorrow"; "ask whether the next 400 lines should exist":

- `backup.go` (1,249) + `backuploop.go` — on-box `tar.gz` of the store, remote refused. A cron plus tar would have been the do-nothing option; the file's own header (`backup.go:7–30`) says the off-box half was cut. What remains is a harness verb with a ticker.
- `ciwatch.go` (1,202) — one bead when `main`'s CI is red, close it when green. The invariant "one bead, not one per push" is right. 1,202 lines is the immune system around that invariant.
- `govern.go` (1,101) + pulse — "facts get computed, decisions get beads" (`govern.go:12–21`) is the right slogan. The rendering surfaces (status, cockpit, watch log, pulse prompt) are why it is a thousand lines.
- `cost.go` (1,091) plus three runtime readers — justified if spend is a brake. It is (plan guard). Keep the seam; watch the per-runtime readers for a fourth copy.
- `visibility.go` (1,342) — a lint, and it says so (`visibility.go:13–16`). A lint that is 1,342 lines is a policy engine.

### 12. FIX — `v0.4.0` is no longer the product

`internal/posse/app.go:20`: `Version = "0.4.0"`. `git describe`: `v0.4.0-1236-gf97cf1fc`. CHANGELOG `## Unreleased` starts at line 12, `## v0.4.0` at 1983. A stranger running `go install …@latest` gets the tag (README:86–91 says so). A stranger reading `main` is 1,236 commits further on, including the overflow removal, the event-hint removal, worktree retire, and the constitution/runtime promotion change that **refuses every dispatched launch until `posse promote`**. Shipping that as Unreleased on a 0.4.0 stamp is how a deployer reads the wrong contract. Either tag, or stop calling the tag the product.

### 13. KEEP — Author identity is a shop convention, not a bug in the log

Every commit in the window carries a single human author name (the identity the launcher commits as). The bead id in the subject is the provenance the tree actually uses (AGENTS.md; ADR 0051). That is consistent with "personas draft, operator identity commits." It does mean `git log` does not name a persona or a runtime. Outside reviewers (this one) have to read subjects, not authors. Do not "fix" this with trailers; nothing enforces them (AGENTS.md already measured that).

---

## What I would not ship, what I would delete, what I would fix first

**Would not ship.** Current `main` as `v0.4.x` to a deployer who is not this instance. L1/L3 described as enforcement. CONTRIBUTING's implication that a first-time contributor runs `make test` the way they would in a 10k-line module.

**Would delete (class, not a Saturday massacre).** New QA pins whose subject is a comment, a Makefile recipe, an ADR status line, or another pin. Inert config keys can stay inert (the upgrading notes are honest). `docs/adr/history/` already left; do not bring dated copies back. The tmux-reference branch is already declared dead (`DIRECTION.md:63–65`); I did not inspect it.

**Would not delete.** Worktree isolation, deny-wins PIDs, promote-from-commit, the three-arm doors, the silent-revert audit, the substrate runners, the measured-deletion habit.

**Fix first.** A write freeze on the pin-on-pin class, enforced the cheap way: the next architect close that files a `*_qa_test.go` whose scanner reads comments or Makefile text has to answer "what process behaves differently if this pin is deleted." Second, in the same week and without a design bead: strike `dispatch.go:3–4` and `DIRECTION.md:122–123` so the next design is not licensed by a sentence the file has not earned since before `v0.4.0`. Third: tag what is actually running, or accept that `0.4.0` is a museum label.

---

## What I could not verify

- Full suite colour at this SHA (`make test` / arm 2 / arm 3 / `test-linux`). Doors I ran were green; that is not the suite.
- Live dispatch, cockpit, cage L4, herdr detection, bd 0.50.3 behaviour on this box. I read the runners and the tests; I did not hire.
- Whether the 101-file SCC at `6f94a99c` is still 101. Product files are 117 now. ASSUMED still one component.
- Per-arm *wall clock* today. File/test counts above are MEASURED; seconds per arm are cited from older notes (qp1hm: 1091.8s one-binary; later landings: ~1140–1190s for `internal/posse` after the split — those runs were not mine).
- Commit-theme buckets are keyword matches on subjects, not a hand taxonomy. A "fix:" that is actually a pin will have been double-counted or missed.
- Cleanroom images, Homebrew bottles, macOS install routes: unread beyond the fact that the scripts and QA files exist.
- The twin review on the other runtime.
- Whether anyone outside this shop runs `main`.
