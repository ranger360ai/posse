## The step-7 canary, and the genre wall that moved it (ranger-base-uwxs4)

2026-11-10 — first dispatch on the always-on host (ADR 0083 step 7).

That line is the canary ADR 0083 D5 asks for: the host's watch resumes only
after one bead has gone filed → hired → closed → landed, and this fragment
is the landed half of it.

### It was asked for as `docs/canary.md`, and that path cannot land here

The bead named `docs/canary.md`. The `prepare-commit-msg` gate refused the
commit:

```
refused by posse gate: a new docs/ file outside the public genre allowlist
today's allowlist: adr|runbooks|notes.d|probes
staged new file(s):
  docs/canary.md    (genre: (none — staged directly under docs/, no subdirectory))
```

ADR 0024 D2 check 1: this repo's beads db is stamped public, and a public
repo's `docs/` tree carries only the genres any deployer of this software
could have written — an allowlisted subdirectory, shipped as
`PublicDocsGenres` in `internal/posse/visibility.go`. A file staged directly
under `docs/` has no genre at all and is refused the same as an unlisted one.

The refusal names two ways through: write it in the instance tree, or admit
the genre by editing that constant and re-running `posse gates
install-hooks`. Neither was taken. The instance tree is not where a canary
can land, because *landed* in D5 means a commit the launcher fast-forwards
onto this repo's `main` — that round trip is the whole thing being tested,
and a commit in the private tree does not make it. Admitting a new public
`docs/` genre is a reviewed code change about where the public/private wall
sits, which is not a one-line canary's to make, and `canary` is a genre no
other deployer of this software has any use for.

So the line landed in `notes.d`, which is allowlisted and is already the
per-bead record genre. Nothing about the canary's shape changed: one dated
line, committed, landed. Only the path did.

Verified: `make tree-check` green at 50.3s, one-minute load 0.46.
