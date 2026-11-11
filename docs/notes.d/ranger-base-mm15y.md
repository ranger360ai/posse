# ranger-base-mm15y — scrubbing a beads store of record, and the export history it has already been committed into

`scripts/queue-scrub.sh`, `scripts/queue-scrub-sub.py`.

The occasion is ranger-base-ochbf and the operator ruling recorded on it. What
is written here is the MECHANISM only: what has to be removed, from where, and
how you prove it is gone. The content being removed is named by bead id on
ranger-base-mm15y and nowhere else, because it is a hole that is still open at
the time this page lands.

## The shape of the problem

A beads store is two artifacts that have to agree: a SQLite database
(`.beads/beads.db`, untracked) and a generated export (`.beads/issues.jsonl`,
tracked and committed by every seat that closes a bead). Text that should not
travel can be in either, and removing it from one is not removing it:

- scrub the database alone and every commit that already carries the export
  still holds the text — and those commits are what a push sends;
- rewrite the history alone and the database re-exports the text at the next
  `bd sync`, in the next seat's ordinary beads commit, within the hour.

So it is one operation in a fixed order: **database, re-export, history.** A
scrub that stops after the first step is the common failure, because the first
step is the one that makes `bd show` look clean.

## bd cannot do the first step

`bd` has no verb that edits or deletes a comment. `bd comments` has exactly one
subcommand, `add`; `bd edit` edits an ISSUE field. There is no route through
the CLI, so the database half is a SQL `UPDATE` against `comments.text` or
`issues.description`, and that is an operator action on the store of record —
not something a seat does, whatever its PID happens to allow.

Two bookkeeping rows have to move with it or the re-export is a no-op: bd skips
a bead whose content hash is unchanged, so the owning bead needs a row in
`dirty_issues` and its row in `export_hashes` dropped. (On this instance
`export_hashes` is empty and the state lives in `.beads/export-state/*.json`;
the delete is harmless either way and the `dirty_issues` insert is what
actually does it.)

MEASURED 2026-11-11, on a `.backup` copy of a 3,536-issue store: `UPDATE` on
three containers, the two bookkeeping rows per owning bead, then `bd sync` —
`✓ Exported 3537 issues`, and the regenerated export differed from the original
in exactly those three records plus nine others that other seats had written in
the forty minutes since the snapshot was taken. Which is the operational note:
**the apply wants a quiet db**, or the beads commit that carries the scrub
carries half the shop's afternoon as well.

## The second step has a trap with no error message

The export is written by Go's `encoding/json`, which HTML-escapes `<`, `>` and
`&` to `<`, `>`, `&` and leaves non-ASCII as raw UTF-8. Python's
`json.dumps` does neither by default. So the obvious way to find the text in
the `.jsonl` — re-encode the value you read out of SQLite and search for it —
finds NOTHING, and finding nothing looks exactly like having nothing to do.

MEASURED on this tree's export: 2,324 `<` and 663 `&`. The first run
of this scrub reported `0 of 34 commits carry a target` and was wrong about
every one of them.

`queue-scrub-sub.py` re-escapes the way Go does and the caller's hit count is
the check: a count of zero on a container you know carries the text means the
escaping is wrong, not that the text is absent. Say that out loud in any tool
of this kind, because both readings produce a green run.

Matching on the ESCAPED form, rather than parsing each record and
re-serialising it, is also what keeps the edit surgical: the record is never
rebuilt, so a generated file stays byte-identical everywhere the text is not.

## The third step is refused at a seat, by design

Rewriting the unpushed commits means writing commit objects that no
`pre-commit` or `commit-msg` hook ever sees. posse's git shim refuses
`git commit-tree` for exactly that reason (`hookskip:plumbing-commit`,
ranger-base-r3pri, ADR 0052), and `git filter-branch` is refused through the
same door because it calls `commit-tree` underneath.

That is the gate working. A seat can prove the whole rewrite without it:
transform each commit's blob, check the result, and check that nothing but the
target records moved. `queue-scrub.sh` does that as its "blob proof" and then
stops at the gate with a sentence saying so, exiting 0 on a dry run. The chain
itself is built by the operator, with `--apply`.

The loop that builds it preserves author, committer, both dates and the message
verbatim, and rewrites one path. It is a `commit-tree` loop rather than
`filter-branch` because it is twenty lines you can read, and because
`filter-branch` is deprecated and `git-filter-repo` is not installed here.

## What "verified" has to mean

Two independent checks, because each misses what the other catches:

1. **zero target hits** — the old value of each scrubbed container is absent.
   Complete for the containers you named, and blind to one you did not.
2. **zero detector hits** — a fixed-string sweep (`grep -F -f`) for the
   distinguishing string itself, over every commit in the range AND over a
   freshly regenerated export. Catches a container you did not name.

Run both, over `origin/main..main` and over the working tree. Check (2) is what
turned a ruling naming one container into a measured list of three: scrubbing
the named one left the detector reading 2 hits per commit, in all seven commits
that carried it, and the blob proof said `DETECTOR-STILL-HITS` rather than
passing. A verification that only checks what you decided to remove will agree
with you.

## And the durable finding, which is ochbf's and not this bead's

A wall exists for ops-class prose entering a public TREE — `prepare-commit-msg`
refuses the commit by class and by matched token — and none for the same prose
entering a beads DB, which reaches the same audience by a different door. One
seat put the same sentences through both doors inside an hour and was stopped
at exactly one of them. Whether `bd create` and `bd comments add` should be
gated the same way is open on ranger-base-ochbf; costing it is not this bead's
work.
