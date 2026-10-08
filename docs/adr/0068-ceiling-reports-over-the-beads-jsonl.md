# ADR 0068 — The data ceiling REPORTS over `.beads/*.jsonl` and refuses everywhere else: a wall whose remedy the committer cannot perform is a lockout, not a gate

*Status: accepted 2026-10-08 (ranger-base-6pisf, from github.com/ranger360ai/posse/issues/1 — operator, work-box shakedown ranger-base-x4g3h) · owner: architect · amends ADR 0050 D2 and D5 (the ceiling's content arm over the beads jsonl) · builds in the code beads named on ranger-base-6pisf (dinesh); the write-time layer ADR 0050 deferred is filed as its own design bead, not decided here*

> One bead record carrying a data-ceiling class refused EVERY commit in the
> repo — persona memory landings at reap, the loop's queue commit, the
> operator's own — until an override committed it once. Nobody who was
> refused could remove the content, and the override changes nothing about
> whether it lands: it only decides who types it. This record makes the
> ceiling refuse where a committer can act and report where they cannot.

## Context (MEASURED 2026-10-08, git 2.50.1, bd 0.50.3, unless marked)

- **bd's pre-commit stages the queue into a path-limited commit.** For
  `git commit -m x -- a.txt`, git hands every hook `GIT_INDEX_FILE` =
  `.git/next-index-<pid>.lock`, a false index holding HEAD plus the named
  paths. bd's shim runs `git add .beads/issues.jsonl` against THAT index, so
  the commit's tree carries the flushed jsonl beside the one path the
  committer named. The real `.git/index` is untouched during the hooks and
  gains only the named paths afterwards — which is the `MM .beads/issues.jsonl`
  residue AGENTS.md already calls "not work" (rangerhq-be7k).
- **The hook cannot tell a named path from a hook-staged one by the index.**
  In the false index both differ from HEAD and both differ from the real
  index (the named path because the real index still holds its old blob; the
  jsonl because bd rewrote it). The only distinguishing fact is ORDER — what
  the index held before bd ran — and no hook posse owns runs before bd's.
- **Nothing a committer can do clears a bead line.** `bd comments` has one
  subcommand, `add`. `bd delete` is denied to every persona and, for the
  operator, moves the whole record into `.beads/deletions.jsonl` — which is
  the same wall's subject (check 0's comment, rangerhq-fuom). bd's pre-commit
  re-exports and re-stages the file into every later commit. So from the
  moment the record is written, the line WILL enter git history; the refusal
  decides only whether a human types `RHQ_VISIBILITY_OVERRIDE=i-mean-it`
  first, and in which of the N refused processes.
- **The two automated victims are path-limited commits nobody is typing at**
  (`memoryland.go` LandPersonaMemory, `queuejsonl.go` the queue commit). A
  refusal there is a reap that does not land and a queue that does not
  project, until an operator notices — the fleet-stall shape, measured once
  on the work box (issue #1).
- **A prepare-commit-msg hook cannot un-stage what bd staged.** git re-reads
  the index after `pre-commit` and not after `prepare-commit-msg`: a
  `git reset -- q.jsonl` in the latter left the false index clean and the
  commit still carried bd's blob (scenario B, scratch repo).
- **ADR 0050 priced this day.** Its "runtime-side gate on the paste itself"
  was rejected as the FIRST wall and named a legitimate SECOND layer, "the
  trigger for filing it is the first ceiling refusal in refusals.log on the
  work box". Issue #1 is that refusal.
- ADR 0050 D5 as written: the wall "guards the durable, replicated copy: the
  beads jsonl that syncs". For a bead record it does not: the db is already
  durable, the jsonl already written, and the refusal removes neither.

## Decision

**D1 — split the ceiling's content arm by subject, same classes, two
exits.** The reader that REFUSES scans the ADDED lines of every staged path
except `.beads/*.jsonl` (pathspec `':(top,exclude).beads/*.jsonl'`; `(top)`
because the hook's cwd is the worktree root and must not be assumed — both
measured). A second reader over `':(top).beads/*.jsonl'` alone — the db and
the deletion ledger — runs the same `posse_check` classes, class-only as
always, and on a hit REPORTS and continues: a stderr block, a refusals.log
line labelled `data ceiling scan REPORTED [prepare-commit-msg hook] (stamp:
N, beads jsonl)`, exit untouched. The report names the class, the hit count,
and the `"id"` of every matching record (`sed -n 's/.*"id":"\([^"]*\)".*/\1/p'`
over the matching `+` lines): the id is the sanctioned citation (ADR 0050
Context), the one thing about the record a refusal may print, and it is
exactly what the remedy needs. The paths arm and the message arm are
unchanged — a path and a message are the committer's own. Check 0 (the
visibility guard over the same files, public repos only) is unchanged; see
Consequences.

**D2 — the report's remedy IS the freeze-and-succeed procedure, printed
where it is needed and written once in the docs.** Rule of the text: the
content is already in the db and will be in history; what stops it
re-entering every diff is that the record never changes again. Steps, as
printed: (1) `bd close <id> -r 'data ceiling: content above the ceiling;
succeeded by <new-id>'` — one last change, one last report; (2) file the
successor citing `<id>` and carrying the system of record's id, never the
content; (3) never comment on `<id>` again — `bd comments` cannot edit or
delete, so every comment re-lands the line (reported, not refused). The same
three steps go into INSTALL.md's ceiling paragraph (~line 1661) and
NOTES.md "Privacy model" (§ at line 114; INSTALL.md cites a NOTES.md heading
"When an instance holds someone else's data" that does not grep in this
tree — the docs bead fixes the cite or adds the heading, whichever is true).
`DataCeilingRule`'s sentence "scanned over the ADDED lines of every staged
file" gains the exception in the same breath, because a rule text that
disagrees with the wall is the drift ADR 0050's Consequences already paid
for once (ranger-base-2bijx).

**D3 — ADR 0050's second layer is triggered, and it is a design bead, not a
paragraph here.** The write path is the only place a bead's ceiling content
can be REFUSED with a remedy the writer can perform (retype the comment with
the cite). The host is already in the tree — `scripts/bd-argv-gate.py`
parses bd argv per segment and resolves the verb — and ADR 0050 wrote down
why it was not the first wall (tool-path only, per-runtime, the ERE in a
gate script that hook-gate logging can echo). Which verbs, how `-f <file>`
payloads are read, where the ERE renders from, what the log may carry: one
page of its own. Filed `-l architecture` from ranger-base-6pisf; this ADR does
not wait on it, because D1 ends the lockout on its own.

**What this is not.** Not a hole: a refusal with no remedy keeps nothing out,
so nothing is let in that was kept out before. Not a change to bd's contract:
the flush still rides into every commit, as bd designed it. Not a new wall:
same matcher, same classes, same log file, one new label.

## Consequences

- The rendered hook changes; every hooked repo reads "ours but stale" on the
  L3 probe until `posse gates install-hooks` re-runs — the price ADR 0048
  and 0050 each paid once.
- Cost per commit: the whole-tree reader now excludes `.beads/*.jsonl` and a
  second reader covers it. ASSUMED same byte count read, so the same cost
  class as today's one reader (ADR 0050 Consequences: ~0.55 s/MB, linear).
  One extra `git diff --cached` process, ~0.1 s (ASSUMED from the ADR 0050
  figure "the reader itself is 0.12s").
- refusals.log gains a label; `posse gates` readers that count ceiling
  REFUSALS do not count REPORTS. A report line per commit whose jsonl
  additions carry a hit — naturally rate-limited to "the record changed",
  which is what D2's step (3) ends.
- Check 0 refuses over the same files for the visibility classes, and a
  COMMENT line in a public repo has the same no-remedy shape (a description
  or title can be rewritten with `bd update`; a comment cannot). Deferred,
  the ADR 0050 way: no lockout has been filed for it in the public repo's
  history of running check 0 since ADR 0024 D2; file it on the first, and
  the same split fits.
- `memoryland.go`'s landing still takes bd's flush into a "memory: land"
  commit and leaves the be7k residue; `generatedindex.go` already passes
  `--no-verify` for exactly that reason (MEASURED there 2026-10-03: pre-commit
  skipped, prepare-commit-msg still runs, so posse's walls hold). D1 cures
  the landing's lockout without it, and no bead demands the residue fix, so
  it is not cut here — the precedent is named so the next reader does not
  re-derive it.
- `bd hook --help` and `bd config list` are refused by the harness's bd argv
  gate for a persona (MEASURED today), so whether bd 0.50.3 has a no-stage
  knob — the issue's exit (a) — was not probed from this seat and is the
  operator's to check upstream.

## Alternatives rejected

- **Do nothing; document freeze-and-succeed alone (the issue's (c)).** Price:
  zero code. Cost: the lockout stands from the write until a human override,
  across every committer and both automated commits; the procedure cannot be
  run from inside a refusal, because closing the record is itself a change
  to the line the wall refuses. The only exit is still the override. MEASURED
  once; the shape is total by construction.
- **Scan only what the committer named (the issue's (b), literally).**
  Feasible: both hooks see the same `$PPID` (git's pid, MEASURED), so a posse
  `pre-commit` chained BEFORE bd's shim could snapshot `git diff --cached
  --name-only HEAD` to `$GIT_DIR/posse-named.$PPID` and prepare-commit-msg
  could read it. Price: a second hook slot (today posse owns
  prepare-commit-msg and pre-push and leaves pre-commit to bd), a temp-file
  protocol between two hooks, a stale-file sweep, and the `--no-verify` and
  foreign-chained-hook cases to reason about — three mechanisms. What they
  buy: the refusal survives on commits that NAME the jsonl, which is the
  loop's queue commit and the operator's `bd sync`-then-commit — the same
  no-remedy refusal, one commit narrower, still stalling the loop until the
  same override. Three mechanisms for one keystroke.
- **Drop bd's staged entry from the commit.** In prepare-commit-msg: not
  honoured (MEASURED, Context). From a second pre-commit after bd's: it would
  work, and it would make posse decide what bd's flush lands, which is bd's
  contract and the shared-tree residue rangerhq-be7k lives on — a write to
  the index where a read-only split suffices.
- **Exclude `.beads/*.jsonl` from the ceiling silently (pathspec only).** One
  token cheaper than D1. Rejected: the operator never learns a record above
  the ceiling exists, nothing tells them to freeze it, and ADR 0050's trigger
  for the second layer — a line in refusals.log — never fires. The report is
  the difference between a hole and a decision.
- **`--no-verify` on posse's own commits.** Cures the two automated victims
  only; a persona's `git commit -F - -- <paths>` and the operator's hand
  commit are refused exactly as before. Kept as the generatedindex precedent,
  not adopted as the fix.
- **The clever one: refuse unless the record is already frozen.** Read the
  record's `status` from the matching line and refuse only an OPEN record,
  so a closed-and-succeeded one lands. Rejected: it moves bd's status
  vocabulary into a shell wall, it refuses the one change (the close) that
  the remedy requires, and it still buys only the keystroke.

## Verification (the pin reproduces the scratch repo; run before close)

1. Repo with a pre-commit that rewrites `.beads/issues.jsonl` to carry a
   classed line and `git add`s it; `git commit -m x -- docs/a.md` → exit 0,
   stderr names the class, `1 hit(s)` and the record's id, never the text;
   refusals.log gains one `REPORTED … beads jsonl` line. (MEASURED today with
   the two readers by hand: refusing reader 0, reporting reader 1, id `rb-2`.)
2. Same class in the NAMED `docs/a.md` → exit 1, the existing refusal, no
   report line. (MEASURED: refusing reader 1.)
3. Same class in the commit message → exit 1 (arm unchanged).
4. `git commit -m x -- .beads/issues.jsonl` (the queue commit's shape) with
   a classed line → exit 0 with the report.
5. Committed from a subdirectory → identical (`(top)` pathspec; MEASURED).
6. `RHQ_VISIBILITY_OVERRIDE=i-mean-it` is not consulted by the reporting
   reader (there is nothing to override).
7. `grep -m1 '^\*Status' docs/adr/0050*.md` names this record; `make adr-check`
   green.
