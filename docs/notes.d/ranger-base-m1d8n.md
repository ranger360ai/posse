# A closed-dirty handoff that resolved itself two minutes after it was filed (ranger-base-m1d8n)

ADR 0041 §2 routes a dirty close back to the closer as a P1 bead. This is the
first one in this repo's history that was already resolved when the closer
read it, and the first whose worktree no longer existed — so the two
resolutions the handoff's own description offers ("COMMIT them in that
worktree" / "DISCARD them") both named a tree that was gone.

## MEASURED 2026-10-05 — the timeline

All times EDT, from `git log --format=%cI` on `main` and from the bead's own
comments.

| time | what |
|---|---|
| 21:33:25 | `3f8f025e` ranger-base-p9qve, the census itself |
| 21:38:33 | `5953bc09` the generated notes-index row held OUT, so the merge could land under a PID that refuses a conflicted commit |
| 21:38:34 | `0161e4bd` merge main (4faa5c62) |
| 21:39 | `bd close ranger-base-p9qve`; a launcher reading of the tree posts `closed dirty [1 path(s)]: docs/notes.d/README.md` and files ranger-base-m1d8n |
| 21:41:51 | `e4328fbf` ranger-base-p9qve — the row put BACK from the generator, `docs/notes.d/README.md │ 1 +` |
| 21:45:52 | `e54338b4` the push lane's silent-revert triage of 5953bc09 |

So the dirty path was committed under ranger-base-p9qve by the session that
left it, two minutes after the handoff was filed, and it landed: `e4328fbf` is
on `main`, the p9qve branch and worktree are both gone (`git worktree list`,
`git branch --list '*p9qve*'` — empty), and the tree was therefore retired
through the ordinary route rather than discarded with work in it.

## Why nothing could have been lost, provably, with the tree gone

The dirty path was `docs/notes.d/README.md`, which `scripts/notes-index.py`
renders **whole** — one `output.write_text(render(directory))` over
`docs/notes.d/*.md`, no hand-maintained section anywhere in the file. So its
content is a pure function of the fragments on disk, and
`python3 scripts/notes-index.py --check` green at `e54338b4` (exit 0; also
`make notes-check`, exit 0) says HEAD's copy IS that function at HEAD. A lost
copy of a wholly generated file whose inputs all landed can hold nothing the
tree does not already have. That is the argument that closes this bead; it
does not generalise to a dirty path anyone authored by hand, where the content
is gone with the tree and the only honest answer is that it is gone.

`make seed-check` green over all seven of the pins ranger-base-p9qve ships,
the NUMBER half included, so the deliverable that bead was filed for is whole
on `main` too.

## The gap, stated and not filed

ADR 0041 has no retraction. §2's dedupe (ranger-base-a3zvb) stops a second
handoff being filed once the closer has committed *some* of the paths, but
nothing closes the handoff already on the board when the condition stops
holding, so the resolved case still costs a dispatched session to read, verify
and close — this one. §4's reasoning does not forbid a retraction the way it
forbids reopening or auto-committing: the handoff is posse's own bead, not a
persona's write in the store of record.

Not filed as a bead, because the number is too small to size anything by:
**two** `closed dirty: ` beads exist in this graph over its whole history
(`bd list --all --limit 3000`, 2026-10-05) — ranger-base-9rsia, closed by its
closer in the ordinary way, and this one. A retraction arm would be a
mechanism built on n=1. The second self-resolving one should be filed against
this paragraph.

## For the closer, which is the part that is reusable

**Commit, then comment, then close — and stop.** §3's cooperative gate refuses
`bd close` from a dirty session worktree, so a close is only ever clean at the
instant it is typed; dirt that appears *after* it is outside every fence ADR
0041 has, and a launcher reading in that window files a true P1 bead about a
tree that is about to be fine. The p9qve session closed, then kept working,
then committed — which is why this page exists.
