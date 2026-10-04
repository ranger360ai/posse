# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the plane

- **Know which tree you are in.** A dispatched session works in its OWN git
  worktree of this repo (under `~/.posse/worktrees/`), on a branch
  `posse/<session>` — its own index, its own HEAD, nobody else's. The work
  prompt names it. An operator session, and any session in the checkout at
  `~/src/posse` itself, shares that checkout with everyone.
- Close the bead, and commit **naming your own paths** (`git commit -F - --
  <paths>`). That form is unconditional: every crew PID carries
  `deny: Bash(git commit unless --)`, a PID-level deny realized as a PATH
  shim that reads argv and never the tree, so it refuses an unqualified
  commit in your own worktree too. What differs between the trees is the
  reason and the hook: in the shared checkout the index is shared, an
  unqualified commit takes whatever another persona has staged, and the
  `prepare-commit-msg` gate refuses it as well; in a session worktree
  nothing is shared and that gate stands down — the PID does not.
- **A NEW file needs two steps** (rangerhq-4pbt): `git add -- <the new
  paths>`, then `git commit -F - -- <all your paths>`. A pathspec only matches
  a file git already has an index entry for, so the path-limited form on its
  own answers `did not match any file(s) known to git`. Scope that add with
  `--`; never `git add -A` or `git add .`, which stage every persona's file
  into the shared index.
- **`MM .beads/issues.jsonl` after a clean commit is not work** (rangerhq-be7k).
  bd's `pre-commit` hook flushes the beads db and stages it, but a path-limited
  commit only refreshes the index for the paths you NAMED — so the commit takes
  the flushed file and the index is left holding the version before it.
  `git status` then reads `MM` over a tree that already matches HEAD.
  Check with `git diff --no-ext-diff HEAD -- <paths>`, which compares the tree
  to the COMMIT: empty means there is nothing there, and
  `git restore --staged -- <paths>` clears it. Never unstage without that check
  first — if the diff is not empty, the entry is someone's real work.
- **In the shared checkout a revert is two steps**: `git revert --no-commit
  <sha>`, then `git commit -F - -- <the paths it touched>`. A plain
  `git revert` names no paths, so the same gate refuses it — after git has
  already staged it (rangerhq-lrnp); undo that path-limited with
  `git restore --source=HEAD --staged --worktree -- <those paths>`, never
  `git reset --hard`.
- **In a session worktree, a `cherry-pick`/`revert`/`rebase` that exits 0 has
  not necessarily ended** (ranger-base-71g2f). Ending one deletes a pseudo-ref
  (`CHERRY_PICK_HEAD`), and that delete needs the SHARED repo's
  `.git/packed-refs.lock`. Your sandbox may create and remove that lock
  (ADR 0059), so the ordinary run is clean and silent. What still bites is a
  lock git cannot HAVE: one someone else left stranded there — an operator
  `gc`, a git killed mid-transaction — or an L4 cage, which binds none. git
  then waits a second per ref transaction, three of them, gives up the delete
  and **exits 0 anyway**: a few seconds of silence and a zero exit. The tree is
  restored and HEAD is right, so every signal says it worked; the marker
  survives, and your NEXT path-limited commit dies at `fatal: cannot do a
  partial commit during a cherry-pick`, in a later Bash call. **That fatal is
  this, not your PID.** MEASURED 2026-09-11, with the lock unavailable:
  `--abort`, `--quit`, `--continue` and `--skip` all leak, and so does a plain
  `git cherry-pick <sha>` that never conflicted; `git merge --abort` and
  `git am --abort` are clean. The L1 git shim refuses such a run (exit 1) and
  prints the `rm -rf` that ends it — read its recipe, run it, then re-read
  `git status`. Doing it by hand: remove `CHERRY_PICK_HEAD`, `REVERT_HEAD`,
  `AUTO_MERGE` and `MERGE_MSG`, and `rm -rf sequencer`, all under the dir
  `git rev-parse --git-dir` names — and **nothing** under
  `git rev-parse --git-common-dir`. A stray `packed-refs.lock` there is the
  operator's to remove, never yours: git's lock files carry no holder
  identity, so you cannot tell a stranded one from a live `gc`'s
  (ADR 0059 D3). Say so on the bead and stop.
- **In the shared checkout, never `--amend`, `rebase` or `reset`.** HEAD there
  moves under you between any two of your own commands — another persona's
  commit, or posse landing a persona's memory at a kill nobody scheduled —
  and an amend rebuilds whatever HEAD is NOW, taking that commit as its base
  and reissuing it under your subject line. Path-limiting does not save you:
  a pathspec governs what is ADDED from the working tree, never what the base
  tree already holds. Nothing of the content is lost — the blob is identical
  either way — but the commit that said who landed those lines is, and
  `git log` then names the wrong persona and the wrong bead
  (ranger-base-4bdo). Correct a bad commit with a NEW one.
- **The bead id in the subject is this shop's provenance; the
  `Co-Authored-By` runtime trailer is the harness's** (ranger-base-5aks).
  599 of 608 commits on `main` name a bead, and everything that asks "why is
  this here" reads that. The trailer is typed by the model and enforced by
  nothing: 60% of the same commits carry one, in runs, since the repo's first
  week. No gate adds it and no gate removes it — `prepare-commit-msg` can
  refuse a commit but never opens its message file for write, so a
  trailer-less commit is a message that was written without one, never a
  route that ate one. Type it; do not rewrite history that lacks it.
  **The `git log --grep <id>` promise is scoped the same way it is earned**
  (ADR 0022): it holds unconditionally in a session worktree; in the shared
  checkout it holds only for a file with one in-flight writer, and NOTES.md
  is never that — personas write a `docs/notes.d/<bead-id>.md` fragment
  instead, or edit NOTES.md from a worktree (see NOTES.md, "The shared
  working tree").
- **Commit everything you want kept.** Only commits move: the launcher
  fast-forwards your branch onto `main` when the bead closes, and uncommitted
  files stay behind in a tree that is eventually retired.
- `bd sync`, so `.beads/issues.jsonl` matches the database. All worktrees
  share one beads database — the graph does not fork.
- **`BEADS_DIR` is set in your session, and it is why the graph does not
  fork** (ADR 0055). Every session posse launches carries
  `BEADS_DIR=<the store of record for the directory you were launched into>`
  — the same `.beads` the seatbelt grants you, the cage mounts read-write,
  and `.beads/redirect` names. It is there because bd does not always find
  that store on its own: in no-db mode — which a CONTAINER session's `bd`
  always is, and which any store can be — bd resolves `$BEADS_DIR` else
  `$cwd/.beads` and reads no redirect at all, so without it a bead you file
  from a session worktree lands in that worktree's own `issues.jsonl`,
  invisibly, while `bd where` still names the main store. Leave it alone. If
  you genuinely need ANOTHER repo's graph for one call, shed it for that
  call — `env -u BEADS_DIR bd <...>` — rather than exporting a new value,
  which would silently move every later bd in the session.
- **Never push, and never merge to `main` yourself. The operator pushes and
  the launcher merges.** Every persona's PID denies `Bash(git push:*)` and
  this repo's `pre-push` gate refuses it, so a push is a refused turn, not a
  landing. Work is complete when it is committed locally and the bead is
  closed; `posse worktrees` shows anything that has not landed.
- **Checking that a background process actually died: not `jobs -l`, not a
  %CPU threshold** (ranger-base-6mhxw). Both look like a clean check and
  both can read empty over a real leak. `jobs -l` only sees the CURRENT
  shell process — a gate session's Bash tool calls each fork their own shell
  (ADR 0009 preamble), so anything backgrounded in an earlier call is
  already invisible to a later call's `jobs -l`, alive or dead. A
  per-process %CPU threshold is blind the other way: a leak that fans out
  into many low-CPU children (forty spinners at ~1% each was the incident)
  sits under any floor worth setting. After backgrounding anything, in a
  LATER Bash call, run `go run ./cmd/checkorphans` from the repo root
  instead — it reads the real process table (ppid 1, old enough not to be a
  fork/exec teardown window, argv matched against the ADR 0009 gate-shell
  preamble) and exits nonzero if anything of yours is still there.
  **It runs from a caged seat, and until 2026-09-28 it did not**
  (ranger-base-yxmwx). `/bin/ps` on darwin is setuid root, and seatbelt
  refuses to exec a setuid binary from inside a sandbox whatever the profile
  says — `(allow default)` does not lift it and neither does an explicit
  `(allow process-exec* (literal "/bin/ps"))`, both MEASURED — so the
  mandated check was the one check a caged seat could not run: `fork/exec
  /bin/ps: operation not permitted`, exit 2, leak status unknown — four
  closes across three personas in one day, each inventing a different
  substitute. It takes the table from `sysctl kern.proc.all` now, with no
  fork at all, and a caged run prints what an uncaged one prints. If it ever
  answers **exit 2** anyway, that is "leak status unknown" and never
  "clean": fall back to `kill -0 <the pid the launcher printed>`,
  `pgrep -fl <your worktree path>` and
  `scripts/suite-lock.sh --status`, and say on the bead that you did —
  because that substitute is weakest in exactly the case the tool exists
  for. A `pgrep` of one path cannot see the fan-out of forty low-CPU
  children above, and a pid you already know about was never the part you
  were unsure of.
- **Something you MEANT to leave running: declare it** (ranger-base-gvp2p).
  Write `POSSE_KEEP=<reason>` at the head of the Bash line that backgrounds
  it — `POSSE_KEEP=ranger-base-abcd nohup ./bench.sh &` — with the reason
  conventionally the bead id that authorises it. It is a plain shell
  assignment, so it is inert in every shell and lands in the argv either
  way, which is all the load guard can read about a process it did not
  start. **Undeclared is a leak**: the guard's kill arm ends an orphaned,
  CPU-burning, gate-shell child that carries no marker — on any tick of a
  `--watch` loop, not only when the box is already saturated
  (ranger-base-fxs60) — and it cannot tell your deliberate process from
  teau's sixteen spinners by any other means
  (NOTES.md, "Leaked gate-shell children"). A deliberate long-lived CPU
  process on this box is meant to be rare and loud — the standing ruling is
  still no load testing here.
- **Ending anything: `kill` one pid, never a `pkill -f` pattern**
  (ranger-base-6nx72, ranger-base-y2esu). `pkill -f` and `pgrep -f` match
  argv across EVERY process on the box, and every session here runs
  byte-identical argv from a different worktree — same Makefile line, same
  script, same interpreter. So `pkill -f test-times.sh`, `pkill -f "go test
  -timeout 25m"` and `pkill -f "make test"` match all of them, and so do
  `killall yes`, `pkill -f "python3 -"`, `pkill -f "sleep 300"` and `pkill
  -f "dispatch --watch"`. Reading pids off a `pgrep` of the same pattern is
  the same mistake one step later.
  MEASURED 2026-09-11 over every session transcript on the box, 83 days,
  by `scripts/pattern-kill-census.py` (re-run it; the corpus grows): 121
  pattern kills from 63 seats named a target that was not unique to the
  typing session, and **17 of them were followed within ten seconds by
  another seat's run ending** — against 3.7 expected when the same kills
  are displaced in time (p < 0.0025). Of the 31 whose pattern could match a
  sibling's suite argv, **11 landed**: eleven seats, five personas, victim
  runs up to 1,373s. One pair two and a half minutes apart is a kill and a
  counter-kill, each ending the other seat's suite. This is not one
  persona's footgun and prose has not stopped it.
  (These replace the 2026-09-03 figures — 138/18/35 — which were the output
  of a matcher that read quoted and heredoc TEXT as kills that ran, fixed
  under ranger-base-zbg8o. The 39 lines it dropped were all text, so what
  moved is the denominator: the eleven confirmed suite kills are the same
  eleven. Never quote the pre-fix numbers.)
  A run that dies this way prints no red and names no test — it looks like
  a green suite with a short tail — so the cost lands on whoever reads it
  next. `scripts/test-times.sh` prints its own pid at the start (`kill
  THAT`) and, if it is signalled anyway, writes the process table at that
  instant to `$TMPDIR/posse-test-signal.log`, where the sender's gate shell
  still names its seat. A pattern is only safe when it can match nothing
  but your own session — your scratchpad path, your worktree path, `-P $$`
  — never a tool name.
  **Enforced, not advised** (operator ruling 2026-09-03, ranger-base-jjx19):
  every crew PID denies `Bash(pkill:*)` and `Bash(killall:*)` beside
  `Bash(git push:*)`, realized as PATH shims that refuse the whole verb, so a
  `pkill` or `killall` typed at a crew seat is refused by the gate whatever
  its pattern — the session-unique spellings above included, `-P $$` among
  them. Keep the pid the launcher printed and `kill` that, or `kill -- -$$`
  for your own process group; `kill`, `kill -0` and `pgrep` still run.
- **The full suite: `make test`, never a bare `go test ./...`**
  (ranger-base-uvzjk). Five crew worktrees ran `go test ./...` at the same
  moment on 2026-09-04, on a box with eight cores. `go run
  ./cmd/checkorphans` was clean, so none of it was a leak — five legitimate
  suites, each running at 2-3x its solo time, and the aggregate load tripped
  the fleet load guard's ceiling for as long as they overlapped, so the shop
  stopped hiring at exactly the moment five seats were about to free.
  `make test`, `scripts/test-times.sh` and `scripts/gotest.sh` now take one
  of two box-wide slots (`scripts/suite-lock.sh`, an flock under
  `~/.cache/posse`) before they start: a third full run waits, and its first
  line names the worktree it is waiting on. A `-run` filter or a named
  package takes no slot — type those as freely as you ever did, with ONE
  measured exception: an unfiltered `./internal/treepins` is a full suite by
  cost, and **`make treepins` is the spelling that queues it**
  (ranger-base-7zng1, ranger-base-1a0hi). That package's pins have the tree as
  their subject and one of them vets all three arms, so an unfiltered run of
  it costs what a queued arm costs — 363.2s with a warm build cache, 679.1s
  cold, a median of 393.5s over 327 wall readings at or above 60s — and
  unqueued it is invisible to the runs that queue and makes nobody else
  queue. One beside a `make test` on 2026-10-03 put arm 3 at 1459.6s against
  424.1s unloaded and reddened two watch tests that need 0.9s of the 30s they
  blew through. **A bare `go test ./internal/treepins` takes no slot**, for
  the reason the `./...` paragraph below gives and not a different one: the
  rule is a shell function `scripts/gotest.sh` and `scripts/test-times.sh`
  source, and nothing on this box shims `go`. MEASURED 2026-10-04 over 1,125
  session transcripts: 122 unfiltered `go test ./internal/treepins` segments
  from 48 sessions, plus 65 `go vet` over the same package, and NONE of the
  187 through a wrapper — so until `make treepins` existed the rule queued
  nothing it was written for. A `-run` filter over the same package takes no
  slot by design, so `make tree-check` and every focused pin run are
  unchanged.
  `scripts/suite-lock.sh --status` says who holds the slots;
  `POSSE_SUITE_SLOTS` changes how many there are and `POSSE_SUITE_LOCK=0`
  opts a run out loudly. A seat whose cage cannot OPEN the slot files —
  `suite-slot.*.lock: Operation not permitted`, which is what a codex seat
  saw before `~/.cache/posse` was named on its launch line — says so and runs
  unserialized rather than queueing against a box it cannot see
  (ranger-base-r3czg). That run is not held back by anything and holds
  nothing back; `posse gates` prints the row that says whether this session's
  wall names the dir. A held slot whose line also reads **`that pid is
  GONE`** is not a live suite: the wrapper that took it has died and something
  it forked inherited the lock (fd 9 is inherited on purpose — a test tree
  that outlives its wrapper is still spending the box). It frees itself when
  the last of that tree exits; nothing reaps it, and until this line existed a
  slot in that state printed exactly what a running suite printed
  (ranger-base-2fgu4).
  **A bare `go test ./...` takes no slot and is invisible to the runs that
  do** — it is not queued, and it does not make anyone else queue.
  MEASURED 2026-09-04 by `scripts/suite-entry-census.py` over every session
  transcript on the box (re-run it; the corpus grows): of 1,851 unfiltered
  package-tree runs, 78% were typed as a bare `go test` — and over the last
  three days, since `make test` became the house command, 25% still are.
  Nothing enforces this and nothing can: no argv deny distinguishes
  `go test ./...` from `go test -run TestFoo ./...`, and the seats that most
  often type the bare form strip the gates dir out of `PATH` first. It is a
  rule you keep, and the only one of these whose cost lands entirely on
  other people.
  **And since ranger-base-qp1hm a bare `go test ./...` is no longer even the
  whole suite.** internal/posse's tests are three binaries now, split by
  build tag because half its wall was a serial stream no `-parallel` value
  can widen (747 tests that cannot take `t.Parallel`, 537.6s inside a 1091.8s
  run). `go test ./...` builds arm 1 — roughly a third of the package's
  tests, the exact fraction drifting with every landing that adds, removes,
  or retags a test file (`armtags_qa_test.go`'s pin logs the current split)
  — and says `ok` over the two thirds it did not compile. `make test` is the
  three:
  `make test-arm1` (which is also `./...`, the gates and the silent-revert
  audit), `make test-arm2`, `make test-arm3`. Each arm takes a suite slot, so
  a `make test` queues three times and cannot run three suites at once behind
  the guard's back. A focused run of one test still needs no tag *if that
  test is in arm 1 or shared*; if `-run` comes back `ok` with no test named,
  the test is in another arm — `go test -tags posse_arm2 -run … ./internal/posse`.
  `armtags_qa_test.go` (repo root) keeps the partition total and keeps every
  `-run` door's pin in arm 1; docs/notes.d/ranger-base-qp1hm.md has the
  measurement and the file lists.
- **A `-run` filter cannot reach a tree-wide pin — run `make fmt-check` by
  hand** (ranger-base-rulbl). `TestTreeIsGofmtClean` lives in internal/posse,
  a ~950s package past the 600s a seat can spend in one foreground call, so
  the standing advice is a focused `-run` filter. `-run` selects by test
  NAME, and no filter has ever named `TreeIsGofmtClean`, because gofmt is
  nobody's subject. Four commits reached main not gofmt-clean straight
  through that gap — ranger-base-ig1o, -d4ya, -edg8, -4v4r6 — and the last
  drew three concurrent P1 beads, three worktrees and three suite runs for
  one whitespace character, because a red internal/posse fails the whole
  package and `make test` then exits 2 for every seat on the box. Nobody was
  careless: the close that shipped it said `-run
  '(Pin|PlanUsage|Override|Credential|Loopback)'`, ok 45.7s, and none of
  those five matches.
  **`make fmt-check` is the door** — ~1.5s, read-only, no `go test`. It is a
  prerequisite of `make test`, so a full run fails on it in seconds instead
  of at ~950; **when your run was a `-run` filter, type it yourself before
  you commit.** `make fmt` is the fix it names, and the two now read one
  `$(FMT_ROOTS)`, so the advice works on every file the check reports.
  The class is "a QA test whose subject is the TREE, living inside a package
  nobody runs whole", and there are two such packages: internal/posse at
  ~950s and **internal/treepins at 589.965s** (ranger-base-g6sb1). Forty-nine
  pins across the two, behind fourteen doors:

  ```
  make fmt-check       ~1.5s   gofmt over the whole tree (a TOOL, not a filter)
  make crew-check      ~2.5s   does the shipped tree name this instance's crew
  make seed-check      ~0.2s   the published seed surface and examples/config.yaml
  make history-check   ~3s     this repo's publication history
  make doc-check       ~0.1s   prose pins over shipped code and docs
  make identity-check  ~0.5s   this box's identity literals, in paths and content
  make ops-check       ~2s     ops residue and live instance paths in tracked files
  make execwrite-check ~0.2s   executable writes routed through WriteExecutable
  make notes-check     ~0.3s   docs/notes.d/README.md lists every fragment (a TOOL)
  make adr-check       ~1.5s   ADR citations resolve, exemptions name real files
  make corpus-check      ~4s   censuses over this repo's own .go and test sources
  make register-check   ~20s   the register itself: every tree-wide pin has a door
  make scripts-check   ~0.9s   censuses over scripts/
  make pid-check        ~10s   the shipped PIDs under examples/agents/
  ```

  **`make tree-check` is all fourteen, 46.5-77.2s warm over three runs and
  92.6s cold** (MEASURED 2026-10-04, ranger-base-g6sb1; the spread is the box,
  not the cache — the load average went 8.9 to 23.8 across the three with a
  sibling seat holding a suite slot. It was 14.9-16.5s before
  internal/treepins joined, and 5.1s at four pins when ranger-base-ik44f wrote
  this line) — that is the one
  command to type after a filtered run, and it is a prerequisite of `make
  test` for the same reason `fmt-check` is. Two doors are worth typing on
  their own: `make crew-check` when your change touched `cmd/`, `internal/`,
  `etc/`, `examples/` or any `*_test.go`, and **`make notes-check` whenever
  you add a `docs/notes.d/<bead-id>.md` fragment** — which is most beads, and
  which is why that one escaped for as long as it did. `crew-check` replaces
  the hand-composed `grep -rn '<every crew name>' cmd etc examples internal
  *_test.go` that standing orders used to carry: it prints path, line and the
  offending name, and it IS the pin, so it cannot disagree with the suite.

  Most of these doors run the pin under a `-run` filter rather than
  reimplementing it in shell, which is the difference from `fmt-check` and
  `notes-check` (gofmt is a tool and a `gofmt -l` cannot disagree with
  `go/format`; the notes pin's whole body is `python3 scripts/notes-index.py
  --check`, so the door re-runs that same command). A shell rewrite of an ast
  parse would be a second implementation to keep in sync by hand — a door
  that goes narrower than the pin while both look green.
  **A tree-wide pin added tomorrow needs a door here**, and the class is
  derived mechanically — but by TWO rules, because the two packages reach the
  tree in ways with no spelling in common. In internal/posse `go test` runs
  with the PACKAGE directory as cwd, so reading the tree costs a climb and
  the key is the one repo-root helper (`qibRepoRoot`). In internal/treepins
  the `TestMain` chdirs the binary to the repo root first, so a pin there
  reads the tree through a plain relative path and names no helper at all —
  the key there is a directory ENUMERATION (`WalkDir`/`Walk`/`ReadDir`/`Glob`)
  rooted at a relative path, because a reading whose file set is not spelled
  in the test is a reading an unrelated bead can red. (Not "reads a tree
  path": MEASURED 2026-10-04, 291 of that package's 362 tests do that.) One
  member is registered by hand rather than derived — the notes pin, whose
  enumeration is a line of Python inside `scripts/notes-index.py` — and
  `treewidedoor_qa_test.go` reds until every member of either half is named
  by a Makefile door variable.
