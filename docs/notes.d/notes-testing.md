## Testing

**Fixtures are detector-safe: all-A bodies; a realistic secret shape blocks
the push of the whole branch** (ranger-base-m0ce1). Commit 34a27b4 shaped its
Slack test fixtures xoxb-\<12 digits\>-\<12 digits\>-\<16 letters\> — exactly
GitHub's own push-protection detector (GH013) — and GitHub refused every push
of main until the commit was rewritten (6a230eb) with xox?-AAAA…-AAAA-…
bodies. Any credential-shaped fixture (Slack, OpenAI, Anthropic, GitHub,
AWS, Linear) must use a letter-only body; `detectorshapes_qa_test.go` scans
the tracked tree for a digit inside one of those shapes and fails the
commit locally instead of GitHub failing the push.

**The suite command is `make test`, not a bare `go test ./...`**
(ranger-base-2ggb, with gilfoyle's ranger-base-2ad3 and 7xla on the same
invariant). The target adds `-timeout 25m` and the flag is load-bearing: go's
default is 10m PER PACKAGE, and `internal/posse` has been measured on darwin
between 484.6s and 623.2s standalone — the worst reading already past the
default — and at 600.8s / 601.0s / 601.1s under `./...`, which is not an
assertion but the ceiling arriving as a timeout panic, because `./...` runs
the three packages concurrently and starves the long one. That red belongs to
the box, lands on whoever ran the suite to verify an unrelated diff, and names
NO TEST through the house filter (`| grep -E '^(---|ok|FAIL)'` prints a bare
`FAIL … 601.010s`). There is no long pole to cut instead — 1442 tests, none
over 10.3s, the top ten only 14% of the run — and the one structural lever
(`t.Parallel`; the package is a single serial stream at two-thirds of one
core) is blocked by 55 test files calling `t.Setenv`/`t.Chdir`. (Since
lifted in two halves: ranger-base-i7fa put `t.Parallel` on 758 tests; ADR 0047
decides the `newTestBackend` half — one HOME per binary, one worktrees root
per test via an App field — and stamps the package split NO: the directed
graph has a 67-file cycle and the serial time lives inside it.) 25m rather
than 20m because `make test-linux` runs this same target and its
`PLATFORM=linux/amd64` arm — emulated — is over 600s every time, while native
linux/arm64 is 112.3s and nowhere near. `suitetimeout_qa_test.go` keeps the
flag on, keeps its value above the measurement, and sweeps for new entry
points that route around the target. Numbers and the distribution:
`docs/notes.d/ranger-base-2ggb.md`.

**The clock is not the only thing this box runs out of** (ranger-base-krra).
On 2026-08-29 `make test` came back exit 2 with ~80 reds in `internal/rhq`,
every one of them `TempDir: mkdir …: no space left on device`, with 231Mi
free, a 41G go build cache and 670 leaked `Test*` dirs going back two days.
`t.TempDir()` calls `t.Fatal` on ENOSPC, so ONE full filesystem is reported
ONCE PER TEST that wanted a temp dir: through the house filter it is a list of
unrelated test names — worktree, watch, dispatch, merge — and reads exactly
like a broken change. `scripts/test-times.sh` now says it in words at both
moments a reader is present: a `DISK: <n> MB free on <fs>` line BEFORE the
packages run (a warning, never a failure, below a measured floor), and a block
AFTER any run whose log carries ENOSPC, naming the cause and the three places
the space went. A second block covers the same hour's other face — `package
iter is not in std` with a working toolchain, which is what a build cache
emptied under a running build looks like. It deletes nothing: `go clean
-cache` slows every concurrent session and deleting from `$TMPDIR` can take a
live test's TempDir away, so what to clear on a shared box stays the
operator's call. `suitedisk_qa_test.go` pins that `make test` still runs
THROUGH the wrapper — without that the explainers are dead code — and runs the
script's own `--self-test` inside the suite. Numbers:
`docs/notes.d/ranger-base-krra.md`.

`go test ./...` (also `make test`) is hermetic: the test binary re-execs as
a fake `herdr` when `RHQ_FAKE_HERDR=1` (state under `RHQ_FAKE_DIR`), so no
real herdr server is involved. `RHQ_HERDR_BIN` / `RHQ_BD_BIN` point the
runners at any binary — that's how the fakes are injected.

Hermetic against *our own wall*, too: the suite usually runs inside a
persona pane, whose PATH leads with that session's gates bin. Its `git`
shim was answering `TestPrePushHook`'s real `git -C <repo> push` before
the hook under test ever ran — one assertion failed, and the one that
"passed" was reading the shim's refusal, not the hook's (rangerhq-8sd).
`TestMain` now sets PATH to `PathOutsideGates("")` for the whole test
binary; a test that wants a shim on PATH renders one and prepends it.

**A pin that names a clock-derived token cannot hold it with a margin**
(ranger-base-nmab1). `cmd/posse`'s `TestCostPlanAndTheCostFooterAreOneRendering`
seeded a plan reading 3m30s old — "3m30s sits in the middle of the '3m' bucket,
so the two runs below cannot straddle its edge" — and then compared two
`runPosse` launches of the BUILT binary to each other. That is a 30-second
margin against an apparatus nothing bounds the runtime of: on 2026-09-05 the
pair took 48.18s on a box running two other seats' suites, `--plan` rendered
`read 3m ago`, the footer rendered `read 4m ago`, and `make test` was red for
every seat with nothing wrong in the rendering it was pinning. A wider constant
is not the fix — the wall grew 2.4x in four days (ranger-base-pj87l) and would
eat it again. Two shapes work. Freeze the clock IN PROCESS, which is what
`PlanCache.Now` is for and where the age's exact bytes are in fact already
pinned (`TestPlanCacheLineSaysHowOldTheReadingIs`). Or, when the subject really
is the built binary, BRACKET the run: seed immediately before the launch and
accept every value the formatter could honestly have produced between that seed
and the process exiting — `runAgedPlan` in `cmd/posse/costplan_test.go`, which
has one accepted answer on an idle box and two on a loaded one, with every other
byte of both renderings still exact. The four single-launch fixtures of the
same shape (`cmd/posse/planquiet_qa_test.go`'s
`TestQACostPlanServesTheSnapshotWhileQuiet` and three codex-hint tests in
`costplan_test.go`) took the same bracket — `agedAges` and `wantOneOf`, factored
out of `runAgedPlan`/`oneRendering` for a single launch with nothing to compare
against — under ranger-base-boafa.

### The suite on Linux — `make test-linux`

The suite ran on darwin and only on darwin until 2026-08-24, and two defects
lived in that gap until a release rehearsal found them: `ServerGen` fenced herdr
generations on an inode number, which ext4 and overlayfs recycle and APFS does
not (ranger-base-fjj — a live bug in the linux tarballs, not merely nine red
tests), and one gate test asserted macOS's `/bin/zsh` where the contract is a
PATH search (ranger-base-gaf). Neither is exotic: this code reads filesystem
identity and resolves shells, so it is platform-sensitive by nature and darwin
hides one whole half of it.

`make test-linux` closes that gap without CI: `go vet ./... && make test` — the
same two commands `.github/workflows/release.yml` runs — inside a throwaway
`golang:<go.mod>` container. ~35s cold, ~2s warm. The repo is mounted
**read-only** and the container runs as *you* rather than root, so a run cannot
leave a root-owned artifact or a rewritten `go.sum` behind; a test that needs to
write must use `t.TempDir()`, which is what CI requires of it anyway. The build
and module caches are the one writable thing and they live in
`~/.cache/posse/test-linux`, outside the tree.

**It works from a session worktree** (ranger-base-v0gm). It did not, at first:
the script mounted `$REPO_ROOT` and nothing else, and in a linked worktree
`.git` is a *file* reading `gitdir: /Users/…/src/posse/.git/worktrees/<session>`
— a path outside the mount, so git in the container resolved nothing and the
three seedpub publication-boundary pins (ADR 0012) failed with `fatal: not a
git repository` 40s in, looking exactly like product failures. Since every
dispatched session works in a worktree, the gate ORDERS tells personas to run
was unrunnable-clean for every persona, and the honest report was "green except
three known env failures" — one step from "green enough". The script now asks
`git rev-parse --path-format=absolute --git-common-dir --git-dir` and mounts
whatever those name outside `$REPO_ROOT` **at the same absolute path**, because
that pointer is baked into the `.git` file and cannot be relocated. Those
mounts are `:ro` like the repo, and are added to `safe.directory`; an ordinary
checkout has its git dir inside `/repo` already and gets no extra mount.
`release_qa_test.go` pins all of it against a fake git, so both branches are
exercised wherever the suite runs. Skipping the seedpub tests was never the
fix: they *are* the publication boundary.

It tests the host's architecture (arm64 on an Apple-Silicon box).
`PLATFORM=linux/amd64 make test-linux` crosses that under emulation, slowly.
`IMAGE=` overrides the toolchain image; `scripts/test-linux.sh --shell` drops
you in there, and `scripts/test-linux.sh '<cmd>'` runs one command.

**`--platform` is passed on every run, not only when `PLATFORM=` is set**
(ranger-base-1qm5). Docker's classic image store keys `golang:<minor>` as one
local image, so the documented amd64 override *replaced* what that tag pointed
at: one `PLATFORM=linux/amd64` run, and every later default run qemu-emulated
amd64 — announced by nothing but a platform WARNING — while NOTES.md, this
paragraph, said it tested the host arch. A one-shot override was a persistent
retarget. The default is now `linux/arm64` or `linux/amd64` read from
`uname -m`, so the request is explicit in both directions and the run after an
override resolves back to the host instead of inheriting it. Docker's
containerd image store (`Storage Driver: overlayfs`,
`io.containerd.snapshotter.v1`) keeps both platforms under the tag and never
poisoned — measured 2026-08-27 on Docker 29.0.1, where the repro no longer
reproduces — which is exactly why naming the platform beats relying on the
store: the fix is in the `docker run` argv, and `release_qa_test.go` pins it
there against a fake docker, on any store and any host.

The release workflow is still the last gate, not the first one: run this before
you push, because on a tag is the worst place to learn that Linux disagrees.

### The two runners are an ENVIRONMENT split, and it reaches the pins (ranger-base-tiidc)

`ci.yml` runs `make test` on ubuntu-latest and macos-latest, and ten
consecutive pushes to main were red there for three reasons that had nothing
to do with the commits carrying them. All three are one shape: a pin that
stood in for something the ENVIRONMENT supplies — a make, a git, a shell — and
was right about the one this box has. Worth knowing before writing the next
one.

**GNU make 4.x prints `make[1]: Entering directory` on STDOUT, 3.81 does not.**
`TestQATheGofmtDoorReportsRealDrift` and `TestQATheTreeWideDoorsReportRealDrift`
capture `make -n <door>` and hand the result to `sh -c`, which is what keeps
them from carrying a second copy of the recipe. `make test` exports MAKELEVEL,
every `make` the suite spawns therefore believes it is a sub-make, and 4.x
turns on `--print-directory` there — so the captured "recipe" came back wrapped
in two lines of make's own chatter and the shell ran them: `sh: 1: make[1]::
not found`, `sh: 8:`, and a door that had found no drift at all failed its
clean arm. ubuntu-latest carries 4.x; macOS ships 3.81, which is why this was
red on exactly one runner. Reproduced on this box against a GNU make 4.4.1
bottle with `MAKELEVEL=1` — byte for byte, including the line numbers. The fix
is `--no-print-directory` on the command line (`makeExpandFlag`), which beats
an inherited `MAKEFLAGS=w` as well and which 3.81 accepts; `assertRecipeIsOnlyRecipe`
makes its absence a named failure instead of a garbled shell.

**The runners moved to git 2.55.0, and `git commit` grew three options.**
`-U`/`--unified` and `--inter-hunk-context` (the context width of the `-v`
diff) do not exist on this box's Apple Git-155 / 2.50.1. They take a value as a
separate word, so `qualifierSpoilers["git commit"].ValueOpts` has to carry
them or the L1 wall reads the value as an option and refuses a safe
path-limited commit — which is what `TestQAValueOptsAreGitsRequiredValueOptions`
said, on both runners, in the words of the fix.

That makes the table the UNION of two gits, and two pins then name a spelling
the local git cannot be asked about. `qaCommitOptsSince` is that fact on the
pin side: option → the version that has it, exempting it from the "this git
never showed us that spelling" arms and from nothing else, and only after
making git call the option unknown (`git commit -h` hides some it still
parses — `--ahead-behind` on 2.50.1).

**Do not ask `qaGitResolves` whether this git has an option.** It reports that
a prefix resolved, never WHAT it resolved to. On 2.50.1 `--u` and `--un`
resolve cleanly — to `--untracked-files`, which is a different option in
neither list — so a `len(got) == 0` test reads them as abbreviations of
`--unified` and demands wall arms for them, which is the hole the pin exists to
prevent, rendered by the pin itself. Ask `qaCommitOptions` (git's own `-h`
list) instead.

**A path an operator PASTES is a shell's input, not argv.**
`TestTheOffBranchPrescriptionIsRunnableAndRescuesTheWork` takes the `git -C
<tree> branch -f <branch> HEAD` out of `posse worktrees`' listing and runs it,
which is the arm that catches a prescription naming the wrong directory. It
ran it with `exec.Command`, splitting the words itself — a shell in every
respect but the one that mattered. The listing `AbbrevHome`s the path, so on
any box whose worktree root is under `$HOME` the line reads `git -C
~/worktrees/…` and only a shell turns that `~` back into a directory: ubuntu
red with `fatal: cannot change to '~/worktrees/…'`, macOS green, same commit.
macOS was the accident — the test binary's temp `$HOME` is `/var/folders/…`
and the tree path resolves to `/private/var/folders/…`, so `AbbrevHome` finds
no prefix and prints the absolute path — and ubuntu was reproducing the real
operator's shape. Reproduced here by resolving the temp `$HOME` in TestMain,
which reds it on macOS too. It runs through `sh -c` now.

**Measure their version, do not infer it.** `brew fetch git make` DOWNLOADS
the bottles and does not install, link, or touch PATH, so `/usr/bin/git` stays
what it was; untar into a scratch dir and put a shim in front. Two things make
an extracted bottle actually usable and both fail silently otherwise: `DYLD_*`
must be exported INSIDE the shim script (macOS strips it entering any
SIP-protected binary, `/bin/sh` included), and a bottle outside its prefix
finds none of its own data files, so git needs `GIT_TEMPLATE_DIR` or every
`git init` makes a repo with no `hooks/`. Both root causes here were then
reproduced on this box, and each fix has a mutant that reds without it.

And the `toolchain identity` step in ci.yml prints `git --version` and
`make --version` on both runners now, for the same reason it prints awk's:
the next userland split should be a log line rather than an expedition.

**Making one half of git fail: `git status` and `git diff` break on different
things** (ranger-base-2asm5, owed by ranger-base-xw51s). Both cagestale.go
readers swallow a failed git call into a wrong verdict, so both need a fixture
that FAILS a specific call — and a fixture that breaks everything cannot say
which read a pin caught. Measured on git 2.50.1 / darwin 25.4.0:

- **An invalid `status.*` config value** — `git config status.showUntrackedFiles
  <not-a-mode>` — kills every `git status` at config-parse time (rc 128, and
  an explicit `--untracked-files=all` on the command line does NOT rescue it),
  while `git diff HEAD` still renders a full patch. This is the status-only
  fixture, and the one that pins a status branch on its own.
- **A garbage `.git/index`** kills status AND `git diff HEAD` (both `fatal:
  index file smaller than expected`) while `git rev-parse HEAD` still answers,
  since it reads refs and not the index. Use it for "nothing but HEAD can be
  read", never to pin one call.
- **An unreadable tracked file** (`chmod 000`) fails the diff and NOT the
  status, which only stats — extdiff_qa_test.go's arm 2.
- **An unreadable untracked DIRECTORY** (`chmod 000`) is not a fixture at all:
  status WARNS and exits 0.

The fixture helper witnesses both directions — the call it means to break, and
the call that must still work — because a plant that broke both would leave
the arm measuring the other reader.

**A "shipped surfaces" population must be DERIVED from the embed, and it has
been missed twice by a hand-written list** (ranger-base-kox69, verifying
ranger-base-3ersc). `embed.go:17` is `//go:embed all:examples`, so everything
under `examples/` is inside every release binary and `posse init` lays it down
in a fresh instance. That makes a seed PID *more* shipped than AGENTS.md, not
less — AGENTS.md is a file in this repository, the seed is what a deployer
gets — and it is exactly the surface a sweep forgets, because the PIDs a
persona reads all day live in `RHQ_HOME/agents/`:

- ranger-base-09b7: the L1 commit wall stood on the crew's own files and on no
  PID anyone created from what the binary ships.
- ranger-base-l1ix2 hardened a broken command everywhere it was PRESCRIBED,
  listed the hand-sweep as "README.md, INSTALL.md and the ADRs … docs/notes.d/
  is out too", and closed with "that is the whole rest of the tree".
  `examples/agents/reviewer.md:69` still told a reviewer to run the broken
  form, on exactly the change they were sent to read.

So build the population with `exampleAgentNames(posse.Seed)` and
`fs.ReadFile`, not a literal list: a PID added tomorrow is then graded with
nobody remembering the test file. Read the EMBED rather than `../../examples`
— in a checkout they are the same bytes, and the embed is the artifact both
gaps were in. Put a corpus floor on it, because `fs.ReadDir` over a subtree
that moved returns nothing AND no error, so a census over zero surfaces is
green. `extdiff_qa_test.go`'s `extDiffSeedSurfaces` and
`commitwallseed_qa_test.go` are the two worked examples.

Editing a seed PID is two edits, never one: `exampledigests.go`'s contract is
APPEND the new sha256 for that path and never replace one, or a home holding
the old bytes stops being a home posse recognises its own file in.
`TestEveryEmbeddedExamplePIDIsInTheShippedTable` reds and prints the line.

**A count FLOOR is not a liveness check — measure how slack yours is.**
`extdiff_qa_test.go`'s ARM 7 ran seven prescriptions against a floor of three,
so four of them could stop being found without a word. Ask the question
per NAMED surface instead (`ran[name] == 0`), so a prescription that
disappears reds by file rather than shrinking a total nobody reads. That is
ranger-base-3ersc FINDING 2 one level up: there, three exempt spans satisfied
`seen[name] == 0` on their own while every real NOTES.md prescription could
vanish. Before writing any floor, count the real population once with a
throwaway `t.Logf` census; a floor set below what is there measures nothing.

**A tree-walking pin must skip what git skips.** `TestSeedSurfaceNameCountIsZero`
walked the repo root skipping only `.git` and `.beads`, so `make build` — which
writes the gitignored `bin/posse-go` — put a 13MB Mach-O on the "seed surface"
and the pin found the banned token in its string table. `make build && make
test` was red for everyone, the printed "line" was a binary offset, and the
failure text sent that reader after a commit-time wall that was working. Two
arms of the same file had disagreed about it since both were written:
`TestPublicationRootCommitOmitsExcludedPaths` already excludes `bin/` from the
surface it checks. The scan takes its ignore set from one `git ls-files -z
--others --ignored --exclude-standard --directory` — one call, not one `git
check-ignore` per path — and git reports BOTH shapes, a wholly-ignored
directory collapsed to one entry and a single ignored file inside a directory
that is otherwise on the surface. They are skipped by different lines in the
walk, so a fixture carrying only the first leaves the second unpinned (measured:
that mutant survived).

**And it takes no ignore set at all unless the root is that checkout's
toplevel.** An export unpacked INSIDE some other repo — the `git archive`
scratch tree the house mutation rig runs in, when it lands under a checkout
rather than in /tmp — would otherwise be scanned against rules written about
that repo's paths, and the failure is a false SKIP, not a false hit: the parent
ignoring `notes/` silently takes the export's `notes/` off the surface. Empty is
the right answer for a tarball or a scratch tree because an export carries
tracked files only, so nothing under it is ignored and the walk loses no
coverage. (ranger-base-n0v6o)

**Amended 2026-09-06 (ranger-base-chd6w): that last sentence was true of a
PRISTINE export and false the moment anything writes into one.** Empty is the
right ignore set there; what it is not is protection. Measured under
ranger-base-5htxx and again here: `git archive main | tar -x` gives 951 files
and zero hits — and one `go build -o bin/posse-go ./cmd/posse` inside that tree
puts the same 13MB Mach-O back on the surface — a hit reported as
`bin/posse-go:8189:`, a string-table offset with the banned name run together
between `reopenedrejected` and `verify: username`, which is ranger-base-n0v6o
byte for byte in the tree whose safety the paragraph above asserts. (The name
itself is not quoted here for the same reason the pin exists.) `make build` is
exactly what writes into it and `git archive | tar -x` is the house mutation
rig. So the scan now skips **two** things, as a union: what git ignores, and
what is not text (a NUL byte anywhere in the body — git's own test, and 0 of
951 tracked files carry one). The ignore set still earns its place: it collapses
whole directories and it keeps text-shaped build output off the surface. Both
arms are pinned and each reds alone —
`TestSeedSurfaceScanSkipsBuildOutputWhereThereIsNoIgnoreSet` for the second.
ranger-base-n0v6o offered both shapes ("skip paths git itself ignores … or skip
non-text files") and only the first shipped; this is the other half.

