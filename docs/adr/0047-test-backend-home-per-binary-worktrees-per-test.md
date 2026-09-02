# ADR 0047 — The test backend takes one $HOME per binary and one worktrees root per test, through an App field; internal/posse does not split

*Status: accepted 2026-09-02 · owner: architect · answers the open
question in docs/notes.d/ranger-base-i7fa.md §6 · from ranger-base-54zaz,
builds in ranger-base-aupee*

> `newTestBackend` calls `t.Setenv` four times and thereby holds 1431 of
> internal/posse's 1975 tests — 75% of a 991s package — out of
> `t.Parallel`. Three of the four calls are constants and move to
> `TestMain`. The fourth, `HOME`, is the one with design content: a
> per-binary HOME keeps every test out of the operator's live `~/.posse`
> (ranger-base-gvrh), but `DefaultWorktreeRoot()` is `$HOME/.posse/
> worktrees`, and two parallel tests that cut a session tree under one
> root can land on one path.

## 1. Isolation shape

### Context

MEASURED 2026-09-02 at this branch's HEAD:

- `SessionTreePath` is `<root>/<basename of repo>/<session>`
  (worktree.go). Every test repo is a `t.TempDir()`, and the first
  `t.TempDir()` in every test has basename `001` (run: a two-test
  package logs `001 002` and `001`). codexwritable_test.go launches three
  tests with `Name: "crew", Worktree: true`. Under one shared root those
  three are `<root>/001/crew` — three repos, one worktree path, and
  `existingTree` answers "have" for whichever comes second.
- The surface is small but real: `Worktree: true` appears 15 times in 7
  test files; 5 of those files go through `newTestBackend` and so join
  the parallel set the moment aupee lands (autoreap_qa 12 sites,
  worktree_qa 13, codexwritable 3, settleopen_qa 1, warncapture_qa 1).
  `wtApp` sets its own HOME and stays serial by the runtime's rule.
- `App.WorktreeRoot()` reads config `worktrees:` first and enforces one
  invariant: the root is under `$HOME`. It is the feature's only
  placement rule and it has its own pins (worktree_test.go:93-112,
  worktree_qa_test.go:1079-1103).
- 116 test sites rewrite `ConfigPath` wholesale. A `worktrees:` key
  written by the helper survives none of them.
- The other `$HOME` readers outside the App (17 sites: cost.go,
  cost_grok.go, cost_codex.go, credential.go, interstitial.go, trust.go,
  planhint_codex.go, seatbelt.go, herdrback.go ×2, app.go ×3,
  worktree.go ×2) are reads of an empty temp home today and stay reads
  of an empty temp home. One WRITES: `SeedClaudeTrust` puts
  `projects[<dir>].hasTrustDialogAccepted` into `ClaudeConfigFile()`
  under a flock. The key is the per-test repo path, so concurrent seeds
  serialize and do not clobber; only a test asserting that file whole,
  or absent, sees another test.

### Decision

**D1 — one HOME per binary.** `TestMain` makes one temp home before
`m.Run()` and removes it after; `newTestBackend` stops calling
`t.Setenv` entirely. This is aupee as filed and keeps what gvrh bought.

**D2 — one worktrees root per test, as an App field.** `App` grows a
fourth fake-by-construction field beside `ModelLister`/`Load1`/`TopCPU`:
a default worktrees root, empty on a real App, meaning
`DefaultWorktreeRoot()`. `WorktreeRoot()` passes it to `CfgGet` as the
default and changes in no other way: config still wins, the
under-`$HOME` rule still runs on the result. `hermetic` takes the `*T`
and sets it to `<binary HOME>/worktrees/<t.Name()>` — under the shared
home, so the rule holds by construction and its pins keep measuring the
one predicate the product has. `hermetic` takes `t` rather than
`newTestBackend` setting the field because the second call site
(promotelaunch_qa_test.go:318, the w4fb class: a test that builds its
own App and swaps it in) must get the same root or it is the collision
above with a different spelling.

**D3 — a third filter before `t.Parallel`.** The i7fa filters catch
env and package-level vars; neither catches a shared filesystem. Before
adding `t.Parallel` to the newly env-clean set, run the same call-graph
taint with a new root set — the 17 `$HOME` readers above, `SeedClaudeTrust`
and `lockClaudeConfig` in particular — and read each tainted test for
what it asserts: a per-test key (its own repo's trust entry, its own
socket path) is safe; the file whole, or its absence, is not, and that
test either stays serial or calls `t.Setenv("HOME", t.TempDir())` itself,
which makes it serial by the runtime's rule with no list to maintain.
ASSUMED: the list is short; the builder counts it and puts the number
on the bead.

### Alternatives rejected

- **Per-binary HOME alone** (aupee's default if unanswered). Measured
  collision: `001/crew` ×3. It would land as a flake with a one-in-N
  ordering, the class this crew has paid for before.
- **`worktrees:` written into config by the helper.** Killed by the 116
  wholesale config rewrites: the key vanishes silently and the failure
  is the same collision, one step further from its cause.
- **A `UserHome()` indirection on the App, replacing all 17 `$HOME`
  reads.** The one I wanted. It costs a field threaded through 17
  sites, 8 of them free functions with no App in hand (`ExpandTilde`,
  `AbbrevHome`, `ClaudeConfigFile`, the grok/codex dirs,
  `DefaultWorktreeRoot`, the herdr socket path); two of those carry a
  documented invariant, "the path printed is the path read", against
  `os.UserHomeDir`, and a third source of the same fact is exactly what
  that comment forbids. And it isolates nothing a child process does:
  the fake herdr and bd are exec'd, and read the environment.
- **A per-test root under `$TMPDIR` with the rule waived for the
  field.** The rule is the feature's only placement invariant; a
  test-only bypass makes its pins measure a path the product never
  takes. Under the binary home costs one `Join` and waives nothing.
- **A process per test.** Loses every in-process assertion; a rewrite,
  not a change.

### Consequences

- App gains one field; `hermetic(t, a)` changes signature at 2 call
  sites; `WorktreeRoot` changes one argument. Nothing product-facing
  moves: a real App has the field empty and resolves exactly as today.
- Exit hatch: delete the field and `WorktreeRoot` reads as before; the
  field holds no state hostage.
- The `default_dir` fallback to `$HOME` (herdrback.go:1409) now lands a
  dir-less launch in the shared home. It is not a git repo, so
  `EnsureSessionTree` returns nil there; a read, not a collision.

## 2. Package split: no split

MEASURED 2026-09-02, a DIRECTED file-level reference graph over the 93
non-test files (`go/ast`, top-level idents; the i7fa note measured the
undirected one): 27 strongly-connected components. One is a 67-file
cycle holding app.go, herdrback.go, worktree.go, gates.go, seatbelt.go,
cage*.go, dispatch.go, relaunch.go and everything the launch path
touches. Two files sit below it (yamlflat.go, credpin.go — the core
uses them, they use nothing), 23 sit above it as leaves (init.go,
autoreap.go, settleopen.go, cost_grok.go, … — each uses the core,
nothing uses it), and exampledigests.go is isolated.

So a seam does exist, twice, and neither carries wall clock. The 45
test files that hold the serial remainder after aupee — the i7fa tail:
`wtApp` 48s, `wtqaPassWithWork` 26s, `hwsFixture` 20s, direct `PATH`
16s, `HERDR_SOCKET_PATH` 12s — overlap the 30 test files of the leaf
and below-core files in exactly one file (instancelabel_qa_test.go).
The serial time is core-bound. And once aupee lands, in-package
`t.Parallel` already uses the same eight cores that package-level
concurrency would; a second package can only overlap the serial
remainder, which is 25% of the package and lives in the cycle. Moving
the 23 leaves out would also mean exporting `newTestBackend`, the fake
herdr and the fake bd from a package they are unexported in today, to
run tests that get no faster.

**Decision: no split.** Re-price only when a bead moves the subject of
a serial-tail test out of the 67-file cycle; until then the number to
quote is this section's, not a new graph.

The lever that does what a split would do without a seam is sharding
the serial remainder across two `go test -run` processes, each with its
own environment. Priced, not cut: the model that predicted the first
half to 0.4% says aupee alone lands at 0.344 of serial — 164s on a
quiet box, under the 300s line for any serial at or below 873s. Measure
that first. If the loaded-box number is the one that has to pass, file
the shard then, against this section.

## Claims

MEASURED: `001` basenames, `crew` ×3, 15/7/5 worktree sites, 116 config
rewrites, 17 `$HOME` readers, the flock on the trust seed, 27 SCCs /
67 / 2 / 23 / 1, the 1-file overlap. ASSUMED: D3's list is short; the
0.344 prediction (i7fa's model, validated on the first half only).
