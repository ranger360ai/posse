# A fix could not describe the hole it closed, and its bead could not close first (ranger-base-ed07e)

ADR 0024 D2 check 4 refuses a staged markdown line or a commit message that
names an OPEN `vuln-private` bead beside any letter. It judges by LIVE bead
status. A dispatched fix must commit BEFORE its bead closes, because the
close is what fast-forwards the session branch onto `main` — so for the whole
of the window in which such a fix is committable, its own bead is open, and
the wall read the fix's own subject line as a disclosure of the hole it was
closing. AGENTS.md "Landing the plane" says the opposite in its first clause:
a public commit message or notes fragment MAY describe the hole its fix
CLOSES.

## What it cost, MEASURED 2026-11-14 on this repo's main

Seven commits whose ENTIRE message is a bare bead id — nothing in the
subject but the id, no body, no fragment:

| commit | bead | landed |
|---|---|---|
| `a733f674` | ranger-base-76mcd | 2026-11-13 16:48 |
| `a981f75f` | ranger-base-g8fmb | 2026-11-13 19:28 |
| `679389f1` | ranger-base-yaha3 | 2026-11-13 20:17 |
| `83157b70` | ranger-base-a544g | 2026-11-13 21:32 |
| `9e478611` | ranger-base-wj93u | 2026-11-13 22:33 |
| `205ccdfa` | ranger-base-k06gw | 2026-11-13 23:25 |
| `f91d4ba0` | ranger-base-heypq | 2026-11-14 00:18 |

Every one is a `vuln-private` bead's own fix. Zero such commits in the 2,590
up to and including check 4's own landing (`e5b3d160`, 2026-11-12 12:30);
seven of the 105 since. That is this shop's provenance rule — AGENTS.md's own
"599 of 608 commits on `main` name a bead, and everything that asks 'why is
this here' reads that" — answered with an id and nothing else. Three of those
fixes' notes fragments are still held on their beads' comments, unlanded:
ranger-base-g8fmb, ranger-base-yaha3 and ranger-base-wj93u, the three
instances this bead was filed over. Landing them is ranger-base-dsysq; their
beads are closed now, so nothing refuses the text any more.

## Why not the other option

The bead named two: an exemption, or a documented landing order. There is no
order to document. Close-then-commit races the launcher's fast-forward, which
retires the tree the commit would be made in; a follow-up commit after the
close is a commit nobody lands. The only moment a fix can commit is a moment
its bead is open.

## The signal

`branch.<branch>.posseBead` (`beadKey`, `worktree.go`), written at every
launch into a session tree, says which bead that tree was cut FOR. The scan
drops that one id before it reads, and nothing else. It is the record and
never a parse of a name: `unaccountedFor` already refuses the same parse,
because `SessionForBead` joins persona, repo and bead with `-` and all three
may contain one. A tree with no record gets no exemption — the shared
checkout's `main`, where the committer is one of eleven personas and git
cannot say which — so both remedy sentences now send that writer to their
worktree instead of guessing. A detached HEAD still has a record: a
container-tier seat is launched that way on purpose (`PrepareSessionHead`,
ranger-base-t4f1), `symbolic-ref` answers nothing, and the branch comes from
the linked worktree's own admin dir name, which IS the session.

The residual is stated in ADR 0024 rather than closed: a commit that
describes its own bead's hole WITHOUT carrying the fix — a WIP commit, or a
session that lands a fragment and is then blocked — publishes that
description. No commit-time wall can tell a fix from a WIP, and the one
signal that could is the bead closing, which is the signal that is
unavailable by construction.

## What the one escape was

ranger-base-lx5jk landed a full subject line AND a fragment on `983ae5d3`,
2026-11-13 11:47:36, while its own bead was `in_progress` and labelled
`vuln-private` — the queue's own jsonl at 11:44:37, three minutes earlier,
says so. The bead asked what differed. Not the text and not the rule: the
same text through the shipped scan today is refused, MEASURED. The arm fails
open when `posse` on PATH has no `vuln-private-scan` verb (stderr is
discarded, the hook's own designed fail-open, ADR 0024 check 4), and the
pattern fits a binary predating check 4 being replaced that afternoon:
descriptive subject at 11:47, then bare-id subjects from 16:48 onward,
unbroken. Not provable now — the binary in place at 11:47 has been
overwritten (`~/.local/bin/posse` is 2026-11-13 20:58) — so this is the
probable cause and labelled as one. Nothing to fix either way: the fail-open
is ADR 0068's lockout rule working as decided.

## Verified

- `internal/posse/vulnprivate_test.go`: five pins. Four mutants, each
  through `go test -overlay` with the tracked tree unmodified — drop the
  skip, pin the own bead to `""`, drop the detached-HEAD fallback, and
  exempt by `strings.Split(<worktree name>, "-")[2:]` instead of the record
  — all four RED. The last one survived the first version of the parse pin,
  which measured the ATTACHED path where the admin dir name is never read at
  all; the pin reads a detached tree now.
- Live end-to-end, the built binary against the real store, a scratch repo
  carrying a `.beads/redirect` and a session worktree whose record names a
  real open `vuln-private` bead: the control (another residual, with text)
  FLAGGED, so the arm was live; own-bead subject, own-bead fragment and
  own-bead-on-a-detached-HEAD all CLEAN; a "Not closed here" line naming
  another residual FLAGGED with the own bead absent from the refusal; the
  same text from the main checkout FLAGGED; a bare id CLEAN. Store record
  count 3,835 before and after.
- The same four questions again with git's own hook environment set —
  `GIT_DIR` at the worktree's admin dir, `GIT_INDEX_FILE` beside it, which
  is what a `prepare-commit-msg` run actually has and what the check above
  did NOT have: same answers, and the same answers again from a
  subdirectory, each with its own control line.
- A scratch repo with NO redirect reads its own empty `.beads` and the arm
  fails open — which is why every live check above carries a control, and
  worth knowing before writing any other live check of this arm.

## Not closed here

- ranger-base-ietka
- ranger-base-dsysq
