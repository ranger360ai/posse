# claude's cache/ and .last-cleanup, private per seat at bwrap (ranger-base-7x3bj)

*2026-11-20 · gwart · census ranger-base-drimd*

## What changed

`internal/posse/wallclaudesessionstate.go`, two lines of render and the
reasons above them:

- **`cache` joins `claudeSessionStateDirNames`**, so each caged claude seat
  gets a private `<SeatWallDir>/claude/cache` bound over
  `<CLAUDE_CONFIG_DIR>/cache` after the config-dir grant. The seat caches
  the served model catalog, the masked ids, the changelog and the
  closed-issues notice in its own copy, reads no peer's and writes none.
- **`.last-cleanup` gets a private FILE bind**, the shape `history.jsonl`
  already had (`PrivateBind.File`): `<SeatWallDir>/claude/.last-cleanup`
  over `<cfg>/.last-cleanup`. Its source is composed in
  `claudePrivateSessionState` rather than behind a helper, which is the one
  difference from `history.jsonl`: nothing outside that file reads the
  marker back, so there is no second reader to recompose from one helper
  (ADR 0100 D1).

Both are bwrap-only, like every other member of that list:
`SeatbeltCarveOut.Private` is filled by `bwrapRender`, and a profile cannot
bind.

## The hole this closes

These two were the members of the shared config dir that the four in
ranger-base-agnp1 left behind — on claude's own protected list nowhere, and
named by no carve (census ranger-base-drimd). MEASURED 2026-11-15 from a
bwrap seat, claude 2.1.325: no bind covered either name, a file created
under `cache/` landed in the shared host dir, and `.last-cleanup` passed
`W_OK`. Uncaged seats and the operator's own claude read the same files.

What a peer's write reached, read out of the 2.1.325 binary's own JS and not
exercised:

- `cache/model-catalog/<account>-<hash>-cc.json` is the **served** catalog
  cache — models, provider alias targets, per-model thinking and effort
  defaults, `state.model`, `min_claude_code_version`. On read claude checks
  the schema and the surface and nothing else; the file carries no signature
  field, the signing covers the separate published catalog. So a planted
  copy was what the next claude on the box believed about models.
  `cache/model-catalog/masked-ids.json` is persisted masked model ids,
  hydrated at every later start.
- `.last-cleanup` is read for its **mtime**, as a 24h throttle: inside the
  window the background housekeeping and retention sweep is skipped, and the
  marker is only rewritten after a sweep runs. One peer's future mtime
  therefore switched off box-wide transcript retention indefinitely. (A
  symlink at the marker was never a redirect: claude treats ELOOP/EISDIR as
  not-regular, deletes it and rewrites.)
- `cache/my-closed-issues.json` and `cache/changelog.md` are UI display
  only. Low on their own; the same carve covers them.

A posse seat's `--model` is an exact id, so the alias route did not move a
seat. The effort defaults, the masks and the operator's own alias use are
UNMEASURED, and no code execution or credential exposure is known — P2
hardening.

## Pins

`internal/posse/wallclaudesessionstate_qa_test.go`:

- `TestQAWallClaudeSessionStateRendersTheCacheAndCleanupBinds`: `cache` is a
  member **by literal**, so renaming the constant alone cannot satisfy it;
  the `.last-cleanup` file bind is present and lands after the config-dir
  grant; prep makes both ends as empty 0600 files; and prep leaves an
  EXISTING shared marker's mtime alone — the one host write this carve makes
  is bounded to the absent case, because a rewrite there would be the
  launcher doing the very thing the carve stops.
- `TestQAWallClaudeSessionStateRefusesALinkAtTheCleanupMarker`: a link, a
  dir or a fifo at the shared marker and a link at the seat's private one
  refuse the render and hand the launcher no prep step; a link at
  `<cfg>/cache` refuses it with the dir arm's wording.
- `TestLiveWallClaudeSessionStateKeepsTheCatalogCacheAndCleanupMarkerPrivate`,
  under the real bwrap render: the box's served catalog and masked ids are
  invisible to the seat and stay byte-identical through a rewrite, an
  append, a new catalog file, relinking and moving both `model-catalog/` and
  `cache/`, and writing through a link to the config dir; the shared marker
  stays a regular file at its stale mtime through a `touch -d` into the
  future, a truncate, a relink and a write through a dir link; the shared
  `model-catalog/` names do not change. Then the seat's own writes land in
  its own copies, which is what says this is a private copy and not a
  refusal.
- `TestQAWallSeatWallHoldsEveryPrivateSourceAndEveryReader`
  (`wallseatwall_qa_test.go`) counts one bind per member plus **two** file
  binds now, and every source stays under the seat wall dir with the wall
  dir as its `SrcRoot`.

Control, MEASURED 2026-11-20 on this host (`internal/posse`, `-run
'WallClaudeSessionState|WallSeatWall'`):

| mutation | reds |
|---|---|
| `claudeCacheDir` out of `claudeSessionStateDirNames` | the membership arm; the link-at-`cache/` arm renders; the live pin's catalog and masked ids are readable and every plant reaches the host |
| the `.last-cleanup` bind deleted | the file-bind arm; every marker arm of the live pin; and the seat-wall count pin |

## Costs

Both of them are the carve working:

- The seat refetches the served catalog and the changelog instead of reading
  the box's copy, and caches them privately.
- It runs its own housekeeping against its own binds — and with a private
  marker created empty at launch, that means a fresh mtime and no sweep for
  the seat's life. The conservative direction: a seat's own sweep is the
  suspected remover in ranger-base-jkpxq.
- The one place this carve writes the SHARED dir is the mount point: an
  absent `<cfg>/.last-cleanup` is created there as an empty 0600 file, whose
  mtime then throttles the box's next sweep by up to 24h, once, on a box
  where claude has never swept. On this box the marker already exists, and
  prep leaves what is there alone.
- Like every private member, a link left at either name stops the next caged
  launch rather than being mounted over.

## Not closed here

`ranger-base-mbcbv`
