## `posse version` names its build without the Makefile (ranger-base-bzu)

`rhq.Build` is set only by `-ldflags`, which only `scripts/clean-build.sh`
and `scripts/release-artifacts.sh` pass. Every other binary — `go install
...@latest`, `go install ...@v0.3.0`, a plain `go build ./cmd/posse` —
therefore called itself `0.3.0+dev` while carrying its own commit in its
build info. `internal/rhq/version.go` now falls back to
`runtime/debug.ReadBuildInfo` when the stamp is missing:

| build | reports |
|---|---|
| `make install` / brew release | `0.3.0+<sha7>[-dirty]` (the stamp; unchanged) |
| `go install …@v0.3.0` | `0.3.0` — the tag this source is, not `0.3.0+v0.3.0` |
| `go install …@latest` off a later commit | `0.3.0+<sha7>` |
| `go build` from a checkout | `0.3.0+<sha7>[-dirty]` |
| `-buildvcs=false`, no module version | `0.3.0+dev` — nothing to name |

Two measurements behind that, both go1.26.5:

- **Go stamps the main module's version itself now** (go 1.24+). A checkout
  build is not `(devel)`: it is a pseudo-version, `v0.0.0-<ts>-<sha12>`, or
  `v0.3.1-0.<ts>-<sha12>` when a tag is behind it, with `+dirty` appended
  when the tree had edits. `vcs.revision` is still read for older
  toolchains.
- **Go stamps no vcs info at all in a linked git worktree.** `cmd/go`'s vcs
  root test is `{filename: ".git", isDir: true}` (`vcs.go`), and a worktree's
  `.git` is a *file*. It fails silently: even `-buildvcs=true` stamps nothing
  and exits 0. Every persona runs the suite from a worktree, so
  `cmd/posse/version_test.go` builds its fixture in a throwaway repo of its
  own (copy the tree, `git init`, commit via `write-tree`/`commit-tree` —
  plumbing, because `git commit` is denied to fleet personas) rather than
  from the checkout under test. A version test that built in place would
  assert `+dev` and pass against the bug.
