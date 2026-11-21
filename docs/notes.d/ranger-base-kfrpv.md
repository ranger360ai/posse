# ranger-base-kfrpv — the store repo's `.git` is read-only to every seat

A seat whose store of record lives in ANOTHER repository — `.beads/redirect`,
ADR 0012 D3-C, the shape every store on this instance has — was granted that
repo's git dirs WHOLE. `beadsGitDirs(home)` answers `<root>/.git` plus
`--git-dir` and `--git-common-dir`, unnarrowed, and three producers put every
one of them in a write set: `SeatbeltWritable` (L2, and L3 through
`BwrapWritable`), `launchWritableRoots` (the self-caging runtimes' `--add-dir`
set) and `recordGitTargets` (the ADR 0013 §4 gate that probes for them).

VERIFIED on a live bwrap seat 2026-11-15 (the security lane, on
ranger-base-julaf, main 955667e8): an rw bind on `<store>/.git` whole, ro binds only on its `config`
and `hooks`, NO deeper mount on `objects`, `objects/info`, `refs/heads`,
`packed-refs`, `refs/replace`, `info/`, `index` or `logs`, and access(W_OK)
true on `.git`, `objects` and `refs/heads`. That is ADR 0099 D1's hole for the
store repo, whose own answer — the per-seat object store — is scoped to the
SESSION's repository. A prompt-injected seat could overwrite or unlink the
object files every uncaged reader trusts (git names an object by the hash of
its bytes and never re-hashes on read), move `refs/heads/main` for the
operator's next push, or plant `objects/info/alternates` and
`refs/replace/*` so an uncaged `git log` serves content of the seat's
choosing.

2026-11-20.

## Why the grant went to zero rather than being narrowed

The bead prescribed ADR 0099 D1's shape for the store repo — a per-seat
object store bound rw over `<store common>/objects`, the shared store ro at a
side path its `info/alternates` names, the re-hashing import run for it — plus
a narrowing of the rest of the `.git` grant to "what bd sync's commit writes".
Both halves rest on one premise: that a seat commits in the store repo. It
does not.

| the premise | what it reads today |
| --- | --- |
| "`bd sync` COMMITS the JSONL there" (the grant's own comment, from 2026-08-26) | bd 0.50.3 `bd sync --help`: "Does NOT stage or commit - that's the user's job" |
| the committing spelling | `bd sync --full`, denied by every crew PID |
| who commits the projection | the LAUNCHER, uncaged, under the launcher lock — `Dispatcher.commitQueue` → `App.CommitQueueJSONL`, since ADR 0015 §4 moved the store into a repo of its own |
| what the crew actually types | MEASURED below: zero seat writes in 622 transcripts |

The reason was written when the store still lived in the CONSTITUTION repo and
a persona committed its own ORDERS.md beside it (ranger-base-rhw). ADR 0015 §4
moved the store out and took the seat's reason to commit with it; the grant
stayed.

L4 had already ruled this way and says so in `CageMounts`: "The target alone,
not its `.git`: the inner wrapper is `--no-db --no-daemon`, appends JSONL and
never commits, so the grant L2 needs for a sync buys nothing here and would
mount a second repo's history read-write." The container tier grants the store
DIRECTORY and nothing of that repo's git. This is that ruling at the three
tiers that never followed it, so all four producers now answer one sentence:
**the store directory is granted; its repo's git is not.**

A per-seat object store for the store repo would also have introduced a defect
the session-repo case cannot have. One store repo has ONE branch and MANY seat
writers: seat A's commit would be readable only out of A's own store until an
import ran, so the operator's `git log` and the NEXT seat's commit — which has
to read the previous tip's tree — would both fail in the window. The session
repo has one branch per seat and the launcher lands it, which is why D1's
shape fits there and not here.

## The measurements

**MEASURED 2026-11-20, this box (linux 7.2.15-1-omarchy, git 2.56.0, bd
0.50.3).**

1. **No seat has ever written it.** `scripts/store-git-write-census.py
   --store <the store repo's dir name>` — 631 session transcripts under
   `~/.claude/projects`, every Bash command on the box. 190 seat-session
   commands name the store repo; every git verb among them is a read
   (status 19, log 11, rev-parse 8, remote 4, rev-list 4, branch 3, show 3,
   ls-files 1) and the WRITE set is **empty**. The operator's own session —
   uncaged, shared checkout — carries all 10 writes (fetch 5, push 3,
   config 1, merge 1), which is the design. A snapshot, not a constant:
   re-run it, the corpus grows, and the seat half of it grew by 18 commands
   while this bead was being worked. Its `--self-test` holds the classifier to nine lines this corpus
   really carries, including the two `git -C "$SP/dry/<the store
   repo>" update-ref` against a scratchpad COPY that a substring matcher scores as
   seat writes, and `git branch --show-current`, which a verb-only matcher
   scores as a write three times.
2. **Nothing a seat runs needs it.** `bwrap --bind / / --ro-bind <store>/.git
   <store>/.git`. Control arm: `: > <store>/.git/kfrpv-probe` → `Read-only
   file system`, so the probe is over the defect. Under it `bd show --json`,
   `bd list --status open`, `bd ready`, `bd sync --status` and `bd comments
   add` all exit 0 — the comment recording this measurement on the bead was
   itself written through that wall.
3. **The live render, before and after, for a real persona on this box**
   (`posse gates <persona>`, the report copy, same shell and same cwd; the
   installed binary against one built from this tree). The mount set's lines
   for the store repo:

   | | pre-fix | fixed |
   | --- | --- | --- |
   | the store of record | `w <store>/.beads` | `w <store>/.beads` |
   | its repo's git dir | `w <store>/.git` | *(absent)* |
   | read-only binds inside that grant | `x <store>/.git/config`, `x <store>/.git/config.lock`, `x <store>/.git/hooks` | *(absent)* |
   | the record-reach gate | `enforced (… 2 targets)` | `enforced (… 1 targets)` |

   The wall-watch list (`bwrap.mounts`, NUL-separated) goes from 142 names to
   138, and the four that leave are exactly `deny`, `deny`,
   `<store>/.git/config` and `<store>/.git/hooks` — nothing is added. That is
   the adjacent cost below, measured: a host rename at either of those two
   paths detached a mount and killed every live bwrap seat on the box.

4. **The wall itself, under a real bwrap**
   (`internal/posse/storerepogitwalllive_test.go`, 8 rows, this box). A seat
   appends to the store's JSONL and touches its database; it cannot unlink an
   object the store's history names, rewrite it in place, write a new one,
   move `refs/heads/main`, plant `objects/info/alternates`, plant a
   `refs/replace` entry, or commit. Each refusal is read back from OUTSIDE
   the wall — the object's bytes, HEAD, the alternates file, `refs/replace`,
   and what the operator's own `cat-file` serves — and each row runs a second
   time under the pre-fix render, where it must SUCCEED, so a refusal can
   never be an unwritable fixture or a malformed command. With the grant
   reinstated all 7 wall rows red and the record-stage row stays green, which
   is the signature the fix has to have.

**THE LIVE ARM HAS TO RUN ON AN `sbRoot` FIXTURE, and finding that out is
also why this hole was found by a seat probe rather than a test.** The
property is an ABSENT grant, and the bwrap render grants `/tmp` and TMPDIR
wholesale. MEASURED with a first draft over a plain `t.TempDir()` store:
under the FIXED render a seat unlinked an object of the store repo's history,
rewrote it in place, wrote a new one and planted `objects/info/alternates` —
four for four, with every producer answering correctly, through the blanket
temp grant. `sbRoot` takes that grant out of the render for the duration of
the test (`sbNoBlanketTempGrants`), which is what makes an absent grant
measurable at all; the file's head comment carries the reading so the next
author does not pay for it twice.

## What landed

| file | what |
| --- | --- |
| `internal/posse/seatbelt.go` | `SeatbeltWritable` drops the `beadsGitDirs` loop; `sessionHooksDirs` and `sessionGitConfigFiles` drop their store-repo arm; `beadsGitDirs`' doc says it grants nothing now and names its one caller |
| `internal/posse/worktree.go` | `launchWritableRoots` drops the same grant, and the `--add-dir` trade it stated is back to the session's own repo |
| `internal/posse/reachability.go` | `recordGitTargets` answers nothing for a store in another repo — the gate asks for exactly what the wall grants, or one of them is wrong |
| `internal/posse/storerepogitwall_qa_test.go` | the four render-level pins: the producers, the L3 render (bwrap argv), the deny list, the darwin profile |
| `internal/posse/storerepogitwalllive_test.go` | the 8 live rows under a real bwrap wall, with the pre-fix render as each row's control |
| `internal/posse/seatbeltgitidentity_qa_test.go` | arm 3's `redirect` shape: the store repo's config and hooks in neither list, and the record stage is the JSONL write with the store-repo commit asserted REFUSED (darwin-only; it SKIPS on this host) |
| `internal/posse/seatbeltcarveout_qa_test.go`, `internal/posse/cageoverlay_test.go`, `internal/posse/seatbeltdenygitgrant_qa_test.go`, `internal/posse/seatbeltredirect_test.go`, `internal/posse/codexwritable_test.go`, `internal/posse/recordreachgrant_qa_test.go`, `internal/posse/reachability_qa_test.go` | the pins that asserted the grant, inverted with their controls kept non-vacuous |
| `scripts/store-git-write-census.py` | the census above, with its self-test |

Each new pin was run against the defect reinstated: the producer, argv and
profile arms red on the grant, the deny-list arm reds on the deny producers
(the second half of the fix, and a different function), and the live rows red
7 of 8 — the record-stage row stays green there, as it must, since the grant
never bought the record stage anything.

One inverted pin needed a new fixture rather than a new assertion.
`TestCodexLaunchRefusesAWritableRootCodexWouldRefuse` (ranger-base-k62e) put
its dangling symlink at the STORE repo's `.git`, because that was the one
root on a codex launch line that nothing materialized. Of what
`launchWritableRoots` still names, the store directory and the tree's own git
dirs must all Stat as directories before they are named at all, so the
reachable route is the suite slot dir: `suiteLockRoot` answers
`POSSE_SUITE_LOCK_DIR` for any tree carrying `scripts/suite-lock.sh`, and
nothing creates it. The fixture commits that script into the repo the session
worktree is cut from and points the variable at a path behind a link to
nothing; the arms and the control are otherwise the bead's own.

Three changed arms cannot RUN on this host — `sbSkipUnlessSandboxable` and
`rchSkip` need sandbox-exec: arm 3's record-stage arm (the JSONL write
granted, the store-repo commit refused, with HEAD read back and an
unsandboxed control that the same commit lands), the oyta control (one
store-of-record grant to delete now, not two), and the trailing-deny arm
(whose "other target" is `<work>/.beads`, since the store repo's `.git` is
no longer a reachable one). An unrun arm is a claim, so every premise under
them was measured here by a throwaway pin: exactly one store grant line in
the rendered profile and no mention of the store repo's git anywhere in it,
`<work>/.beads` granted and present as a subpath line, `recordTargets` of
length 1, the JSONL granted, the three store-git paths ungranted, and the
unsandboxed control commit landing in that fixture. What stays unexercised
is sandbox-exec's own refusal, which this host cannot apply at all.

## The adjacent cost it also ends

`<store>/.git/config` and `<store>/.git/hooks` were DENIED because that `.git`
was granted — the ADR 0095 shape, a read-only bind inside a writable one. At
L3 an EXISTING deny renders a read-only bind whatever the grants say
(`bwrapPrivateArgs`' loop skips only an ABSENT one), `posse wall-watch` lists
that bind, and a host rename of the path under it detaches the mount in every
namespace and kills the seat. So any operator `git config` or `branch -D` in
the store repo ended EVERY live bwrap seat on this box. The denies go with the
grant, and arm 3 is what holds them gone.

## What it costs

A seat cannot commit in the store repo at all, by any route. Nothing asks it
to. An instance whose store is in another repo and whose `queue_repo:` is
UNSET now has no committer for its JSONL — the launcher's commit is silent
there by design ("every instance that has not cut over") — which is a
configuration to fix by setting `queue_repo:`, not a write to hand a walled
seat.

The store's `.beads` stays read-write, which is the whole of what the record
stage needs: the database, the JSONL, the socket and the lock. What this
closes is the integrity and availability of the store repo's git HISTORY, not
of the bead data, which a seat writes by design.

## Not closed here

- ranger-base-gdgbb — the ADR text that still states this grant as a
  decision. Handed to the architecture lane; the code and its pins are the
  record until it lands.
