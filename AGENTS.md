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

## Shapes that stall the seat

A dispatched seat runs behind a permission coordinator that reads the
command it is about to approve, and it will not approve one it cannot read
whole. A command it cannot read does not fail — the seat **hangs** on a
dialog nobody is there to answer, for about two minutes until that dialog
auto-denies, and the work is then redone. A wait loop that matches itself
does not fail either: dispatch reads it, correctly, as "waiting on own
background work" and never re-prompts, so the seat sits dead and holds its
lane. MEASURED 2026-11-11 (ranger-base-r5w0p): six approval stalls and two
spinning waits across six seats in one day — the crew's biggest avoidable
stall that day, and every one of them a typing habit.

- **Edit a tracked file with the Edit tool, never a heredoc'd
  interpreter.** `python3 - <<'EOF' … EOF` hands the coordinator an argv of
  `python3 -` and a body it has no way to read, so it raises `this shell -c
  script … could not be checked` — twice on 2026-11-11, both editing the
  Makefile (ranger-base-in8dr, ranger-base-w5z7u). Edit and Write take a
  path and the change, and are read whole.
- **A loop that deletes, or a battery that mutates, goes in a script file
  under your scratchpad and is run by path.** An inline `for … do rm -rf
  …; done` raises `dangerous rm operation not ruled out`
  (ranger-base-j0riv), and a compound command of `sed -i` plants plus
  restores raises the unreadable-script dialog (ranger-base-2smyx). Write
  the script with the Write tool, then `bash <path>`: one argv, one
  readable file, one approval.
- **Prefer a fresh `mktemp -d` per iteration over rm-and-recreate.** A
  directory that is new each time round needs no `rm -rf` at all, so the
  shape that prompts never appears in the loop.
- **`rm` on a variable path prompts even when the variable is set on the
  same line** (ranger-base-zmk4q) — the checker cannot prove the expansion
  is non-empty. Guard it: `rm -rf -- "${d:?}"`. The `:?` is what says the
  path cannot be empty, and it is also what makes the command safe to run.
- **Never wait on `until ! pgrep -f <string>` when your own loop's command
  line carries `<string>`.** `pgrep -f` matches argv, your loop's argv
  included, so the loop sees itself and spins forever: ranger-base-vw1cp
  spun 2h30m after its `make test` had finished at 06:48, and
  ranger-base-w5z7u about 50 min after its suite finished at 09:37, both on
  2026-11-11. Wait on the pid instead — `$!`, `wait`, or `kill -0 <pid>` —
  and when the question is whether a background process really died, the
  answer is `go run ./cmd/checkorphans` (see "Checking that a background
  process actually died" below), never a `pgrep` of a tool name.

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
- **A public commit message or notes fragment may describe the hole its fix
  CLOSES; a hole still OPEN is named by bead id and nothing else** (operator
  ruling 2026-11-11). This repo is public and the bead graph is not. What a
  fix shuts is shut, and saying so is how a reader knows why the code
  changed — but a residual still open under a `vuln-private` bead gets its
  id alone: no description, no path, no mechanism, no reproduction, until
  that residual's own fix lands. Keep the "Not closed here" section; write
  it as a bare `- ranger-base-xxxxx` and leave the detail on the bead, where
  the audience is already the right one. WHY: on 2026-11-11 three sandbox
  fixes landed with that section describing the next hole, so each was held
  back from publication and the hold never converged — a tree that publishes
  its open holes cannot be published until every one of them is closed, and
  the next fix's section names the one after it. This is ADR 0024 D1's
  "when in doubt, instance" at the one place its genre arm already reads
  public: the fix is public work, the unfixed hole is not.
  **YOUR OWN bead is exempt, and in your worktree the wall knows it**
  (ranger-base-ed07e): the commit-time check drops the one id your branch is
  recorded for — `branch.<branch>.posseBead`, written at launch — because a
  dispatched fix must commit BEFORE its bead closes, the close being what
  fast-forwards the branch, so under a live-status judge a fix could never
  describe the hole it was closing. Write the subject line and the fragment
  you would write for any other bead. A SHARED checkout carries no such
  record and gets no exemption: write a vuln-private fix's own fragment from
  your worktree. MEASURED 2026-11-14: before this landed, seven commits on
  `main` whose entire message was a bare bead id, and three fixes' fragments
  held on their beads unlanded.
- **Commit everything you want kept.** Only commits move: the launcher
  fast-forwards your branch onto `main` when the bead closes, and uncommitted
  files stay behind in a tree that is eventually retired.
- `bd sync`, so `.beads/issues.jsonl` matches the database. All worktrees
  share one beads database — the graph does not fork.
- **A bd write's exit 0 and its printed line are not the record — read every
  write back** (ADR 0073). In no-db mode a concurrent writer can exit 0,
  print `Created issue: <id>` or `Closed`, and still lose the write: MEASURED
  34-36 of 48 concurrent creates lost, duplicate ids printed, and a `close`
  read back `in_progress` in 1 of 3 trials under a comment storm. `bd show
  <id>` after a create, close or update; `bd comments <id>` after a comment;
  `bd dep list <id>` after an edge. If the read-back is missing the write,
  look for it by title in `bd list` before you file again. Re-file only when
  both reads say it is absent, and never on an exit code alone.
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
- **Reading ANOTHER repo's queue: with the cwd inside that repo, never with a
  store-selecting flag from here** (github issue 9, ranger-base-9mjxb). On the
  pinned bd, a call made from a cwd inside repo A that names repo B's database
  with bd's explicit store flag was MEASURED by the operator auto-importing
  A's `.beads/issues.jsonl` — discovered from the cwd's git repo — into B's
  store. A READ verb, writing one repository's records into another's
  database, silently and past the import line. We are pinned at that version
  and will not file upstream (standing ruling 2026-10-08), so the only control
  is to never hand bd that flag: `cd` into the repo that owns the store (or
  `git -C` nothing — bd reads the DIRECTORY, so the chdir is the whole of it),
  shed `BEADS_DIR` for the call as the bullet above says, and let bd resolve
  its own store. The same rule binds posse's own code and is pinned there:
  `Bd.runOnce` binds a store with `cmd.Dir` plus `BEADS_DIR` and
  `bdGlobalFlags` carries nothing else (internal/posse/beads.go), and
  `TestQANoShippedPosseCallHandsBdAStoreSelectingFlag` censuses every shipped
  script and every non-test `.go` under `cmd/` and `internal/` for a call that
  reintroduces it (`make corpus-check`).
  **This instance runs three stores**, two of them reached through a
  `.beads/redirect`, so a cross-repository read here is one flag away from
  writing the wrong one. `scripts/prune-bd-relates-to.sh`'s `store_owner` is
  the worked example of doing it right: it names the repo that owns the store,
  refuses when it cannot, and never guesses.
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
- **A red `make test` on a fresh `main`: look for the open "main red at"
  bead before you file your own** (ADR 0088 D5, ranger-base-ddgfr) — the
  launcher judges main's tree after every landing and files ONE P1 per
  green-to-red stretch, naming every landing in the window, so the bead you
  were about to write is probably already there with the window in it.
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
  ~950s and **internal/treepins at 589.965s** (ranger-base-g6sb1). This page
  names no count of that class, on purpose: the number is said in ONE place —
  `internal/treepins/treewidedoor_qa_test.go`'s head comment, derived from the
  Makefile by its arm 4 — because two places drift and the reader cannot tell
  which is stale. The copy that used to sit here did: it went four short
  through two beads that added a pin, updated the pinned sentence and left
  this one (ranger-base-xed72). The DOORS are held here instead, membership
  and order, by that same arm reading the block below against what
  `tree-check` actually depends on:

  ```
  make fmt-check       ~1.5s   gofmt over the whole tree (a TOOL, not a filter)
  make crew-check      ~2.5s   does the shipped tree name this instance's crew
  make seed-check      ~0.2s   the published seed surface and examples/config.yaml
  make history-check   ~3s     this repo's publication history
  make doc-check       ~1.2s   prose pins over shipped code and docs
  make identity-check  ~0.5s   this box's identity literals, in paths and content
  make ops-check       ~2s     ops residue and live instance paths in tracked files
  make execwrite-check ~0.2s   executable writes routed through WriteExecutable
  make pkgcensus-check ~2.9s   censuses over internal/posse's own sources (arm 1)
  make pkgcensus-arm2-check    the same, for what only -tags posse_arm2 builds
  make pkgcensus-arm3-check    the same, for what only -tags posse_arm3 builds
  make treepins-check   ~70s   every tree-wide pin in internal/treepins, minus a priced table
  make pid-check        ~10s   the shipped PIDs under examples/agents/
  make land-check      ~1.5s   the launcher's own land gate, cross-GOOS vet and all
  ```

  **The treepins door is named by what it does NOT run** (ADR 0098,
  ranger-base-3cpcg), and it is the one door on that list spelled that way.
  Every other door is a `-run` filter, so a door is the pins it NAMES — and
  in internal/treepins that key stopped paying: a pin there reads a tracked
  file it names as often as it walks a directory, a named read is the
  derivation's own negative case, and so the pins that most needed a door
  were the ones nothing could find. `treepins-check` is that package minus
  `$(QA_TREEPINS_EXCLUDED)`, twelve priced rows in
  `treewidedoor_qa_test.go`'s `twdTreepinsExcluded` — each with its measured
  seconds, box, date and the route that still runs it — so a pin written
  tomorrow is doored the moment it is written. Six targets came off
  `tree-check`'s line when it landed and every one of them is still there to
  type, each a subset of the one door that replaced them: `make notes-check`
  (the notes index, a TOOL), `make orientation-check` (neither orientation
  page orders you to the remote), `make adr-check`, `make corpus-check`,
  `make register-check` and `make scripts-check`.

  The last of those is the odd one out and the one worth knowing about: it is
  not a tree-wide pin, it is `scripts/posse-land-check`, the gate that judges
  a REPLAYED landing — and its fifth step vets the NOT-native GOOS, which is
  the only thing on this box that does. GOOS appeared nowhere in the Makefile
  until this door, so a file that compiles here and not on darwin sat behind a
  green `make test`, a green `make tree-check` and a green `go vet ./...`, and
  the first thing to say so was the coordinator's pre-push gate: that is how
  ranger-base-zd4d4 was held and re-filed as ranger-base-tea4e. ADR 0087's
  residual 1 — "the unreplayed arm can still land a tip the seat never built;
  that is seat discipline" — says why the door has to exist: an UNREPLAYED
  fast-forward runs that gate nowhere, so nobody read its verdict until that
  pre-push gate did. 1.5s warm, 8.2s with the darwin build cache cold, is what
  that discipline costs now it is typed for you.

  **`make tree-check` is all of them, 83-92s warm over three runs with the
  complement door on the line** (MEASURED 2026-11-14, ranger-base-3cpcg:
  91.5s, 83.1s then 91.0s in that order, at a one-minute load average of
  12.53, 11.76 and 7.01 with NEITHER suite slot held by anyone throughout and
  one warm-up discarded — against 42.4s, 43.0s and 42.0s for the door list it
  replaced, measured in the same window at a load of 6.4-7.9. That is the one
  reading in this chain not comparable to the one before it: the line lost six
  targets and gained one that runs a whole package, so +40 to +49s is what a
  door that cannot go stale costs here.
  `make treepins-check` is 67.7-85.3s of it, 2.0-2.25 cores of sixteen, and
  it takes no suite slot — docs/notes.d/ranger-base-3cpcg.md has every
  reading). Before that door it read **50-61s warm over three runs with the
  land-check door on the line** (MEASURED 2026-11-14 on the landing that added
  it: 55s, 61s then 50s in that order, at a one-minute load average of 14.01,
  11.93 and 15.02 with one suite slot held by a sibling seat throughout —
  and 51s, 41s then 41s earlier the same day on the same work at a load of
  1.71 to 4.51 with nothing held, which is the same thing every entry here
  says about which variable tracks). Before that door it read
  **55-56s warm over three runs on a quiet
  box and 92.6s cold** (MEASURED 2026-10-31, ranger-base-h1q5o: 56s, 55s and
  55s in that order, at a one-minute load average of 4.56, 3.70 and 3.25 with
  no suite slot held by anyone, and 72s on the run before those three with
  two of the test binaries cold — the merge-back that landed
  ranger-base-na4rm's work onto a main that had meanwhile landed
  ranger-base-8yv7z's, so the first reading of the class as it LANDS rather
  than as either side of that merge had it. A 1s spread, the tightest this
  command has produced, on the quietest box it has been read on and over the
  largest class it has run — the class is the one thing the wall does not
  track. An hour earlier the same command read 68-106s at a load of 9-11
  (ranger-base-8yv7z) and 60-62s at a load of 6-15 (ranger-base-na4rm), over
  that merge's two sides. It read 81-89s warm at a load of 25-28
  (MEASURED 2026-10-31, ranger-base-3e68k: 87s, 81s and 89s in
  that order, at a one-minute load average of 25.12, 27.90 and 26.08 with
  both suite slots held by sibling seats for the first two runs and one for
  the third — the merge-back that landed ranger-base-pyl22's new work onto a
  main that had meanwhile landed ranger-base-bm6j8's, the first reading of
  that class at its real size. The same command, same code, an hour earlier
  read 62s, 127s and 146s at a load of 23.45, 13.98 and 91.60 — an 84s
  spread against this 8s one, so what the spread tracks is how much the load
  MOVES and not how high it is. Where the LEVEL shows is the floor it moves
  around: every reading taken under a load of 16 is 55-62s and the steady
  25-28 one is 81-89s, so price the command at a minute on a quiet box and at
  ninety seconds on a busy but steady one, and expect two and a half minutes
  when the load is moving. The work those two beads added cost under a second
  between them, so neither spread is theirs. On 2026-11-04 it read 95-122s at
  a load of 52-99 (ranger-base-rviq5, after a discarded warm-up of 153.2s at
  172.55) — the same band as pyl22 below, over a much larger class, and both
  faster and slower than pyl22 was. Up there the box swamps the class:
  across the two, a saturated reading is anything from 75s to 172s, so
  price the command from the quiet entries and read the high end as what it
  costs beside other people's suites. Earlier the same day the command read
  74.5-172.1s at a load of 57-95 (ranger-base-pyl22) and 57.0-57.7s at a load
  of 10-31 (ranger-base-v3tt4), over classes smaller by what those two beads
  added. The cold reading is
  ranger-base-g6sb1's, 2026-10-04, when the same command read 46.5-77.2s
  warm; it was 14.9-16.5s before internal/treepins joined, and 5.1s over a
  small fraction of today's class when ranger-base-ik44f wrote this line.
  Every reading of this command, and what the class was when it was taken,
  is in treewidedoor_qa_test.go's head comment, which is also the one place
  that may say how many pins there are) — that is the one
  command to type after a filtered run, and it is a prerequisite of `make
  test` for the same reason `fmt-check` is. Three of them are worth typing on
  their own: `make crew-check` when your change touched `cmd/`, `internal/`,
  `etc/`, `examples/` or any `*_test.go`, **`make notes-check` whenever
  you add a `docs/notes.d/<bead-id>.md` fragment** — which is most beads, and
  which is why that one escaped for as long as it did — and
  **`make orientation-check` whenever you edit THIS page or `INSTALL.md`**,
  because a `bd init` in the checkout, a second `bd onboard` or a bd upgrade
  puts back a "Landing the Plane" section whose every step the `pre-push`
  gate refuses, and the readers who edit a standing-orders page are the
  readers least likely to be running a suite (ranger-base-n7hsa). It reds on
  any line of this page that names that verb without also saying who does it
  — `Never push`, `operator pushes`, `pre-push` and `git push:*` are the four
  spellings it lets by, which is the whole vocabulary this page has for
  discussing the mandate, and the first draft of this bullet reddened the
  door it was documenting. `crew-check` replaces
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
- **A test that plants an executable and then execs it under a deadline:
  `make coldexec-check RUN=<your tests>`** (ADR 0085, ranger-base-pbo1q).
  macOS assesses a freshly written executable on its FIRST exec and charges
  156-714ms for it, worst 2.17s, against 6.3-10.9ms warm; this box charges
  1.07ms, so an unwarmed fixture written here passes where it is written and
  flakes on ci.yml's macos-latest leg and on the laptop. The door reruns your
  tests inside a supervisor (`cmd/coldexec`) that holds the first `execve` of
  every file created after it started, for every process under it at any
  depth, and the fix when it reds is a `--warm` pre-flight that pays the
  first exec OUTSIDE the deadline, with both halves asserted. (It used to
  inject at `WriteExecutable` instead, rewriting the plant's `#!` line; ADR
  0085 D2/D3 as amended moved the cost into the `execve` because 26 tests
  whose subject is a planted file's own bytes could see the rewrite. The
  rewrite's code survives in internal/posse, reached by no door, until
  ranger-base-7zkxw retires it.) Filtered it takes seconds and no suite slot;
  unfiltered it is five arms, five slots and is nobody's prerequisite —
  `tree-check` and `make test` do not run it, and the class had been
  rediscovered by hand three times before it existed. **Five, because the
  arms are the packages that PLANT**: internal/posse's three tagged arms,
  cmd/posse and internal/treepins, which are every package in this tree with
  a `WriteExecutable` call site at all — 345 of them, MEASURED 2026-11-11,
  274/54/17 (ranger-base-in8dr). The last two cost two Makefile lines and no
  injector of their own, because the door is a process around `go test`.
  **Priced at three times the uninjected arms, and the unfiltered form is
  green** (MEASURED 2026-11-10, ranger-base-kiwvn, at a loadavg of 4-11):
  851.4s over the three arms against 280.3s for the same arms uninjected —
  3.04x, +571.1s — so budget a quarter of an hour of box for the unfiltered
  form and do not run it out of curiosity. That reading was taken through the
  door kiwvn ran, the content rewrite, and not through the execve supervisor
  that runs it now; ranger-base-xpxo6 prices the supervisor and nothing here
  claims the two cost the same. What the run bought was the supervisor. It
  red 26 tests and **not one of them was in the class** — every red was a
  test whose subject is a planted file's own BYTES, a git hook posse
  certifies by comparing it to its own render, or a gate shim identified by a
  marker on line 2 — a family and not a tail, and the answer to a family is
  to stop touching the bytes (ADR 0085 D2/D3 as amended by
  ranger-base-dacil). The rewrite that could not reach them, and the 49% of
  plants it could never reach at all (840 of 1711 have no `#!`), survive in
  internal/posse behind a counted pass-through until ranger-base-7zkxw
  retires both.
  docs/notes.d/ranger-base-kiwvn.md has the triage, the counters and what
  went to the architecture lane.
  **The supervisor's first prices, for the two arms it was extended to**
  (MEASURED 2026-11-11, ranger-base-in8dr, warm cache, loadavg 5-15):
  cmd/posse 674.8s against 103.5s undoored, 6.5x; internal/treepins 1581.8s
  against 131.8s, 12.0x; both green, no `--warm` pre-flight needed anywhere.
  The cost is additive — each arm's doored wall is its undoored wall plus
  2.5s once per cold exec, within 5% — so budget from the cold-exec count
  the report prints, and expect an unfiltered five-arm run to cost most of
  an hour. Every arm therefore carries `-timeout 60m` and not `make test`'s
  25m: internal/treepins at 25m dies on `panic: test timed out after 25m0s`
  with tests still running, which is the door's own cost and not a hang.
  docs/notes.d/ranger-base-in8dr.md has those readings and the one coverage
  hole they turned up (an orphaned process's execs are un-adjudicated —
  ranger-base-orgvt).
