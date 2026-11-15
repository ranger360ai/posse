# release-artifacts.sh fetches its build source, as clean-build.sh does (ranger-base-9lsg2)

2026-11-15. `scripts/release-artifacts.sh` read its build source with
`git worktree add --detach` out of the repo it runs in — the route
ranger-base-la61c removed from `scripts/clean-build.sh`. git never re-hashes an
object on a local read, so the tarballs, bottles, `checksums.txt` and the
Homebrew formula could be minted over bytes substituted in the object store,
and the checksums would then certify them. Exposure was the box-local
`make release-artifacts` / `make tap-formula` route; `.github/workflows/release.yml`
runs over a fresh GitHub clone.

Fixed the same way: `git init` a fresh repo, `fetch --no-tags` the FULL commit
id `$REV^{commit}` resolves to (so a tag or a relative `--rev` still works),
refuse on a failed fetch, compare `FETCH_HEAD` to that id, then check out.
Pinned by `internal/treepins/releaseartifactsrehash_qa_test.go`, whose code-line
scan reds on a `worktree add` coming back (MEASURED by mutation: it does).

MEASURED 2026-11-15 (git 2.56.0, linux 7.2.15, scratch clones only):

- A fresh repo fetching a raw commit id over the local transport succeeds for a
  non-tip commit and for an unreachable one — no `uploadpack.allow*SHA1InWant`
  needed.
- Honest control, the fixed script over a `--no-local` clone of this tree at
  36cb7229: rc 0, all eight tarballs and bottles plus `checksums.txt`.
- Forged arm, the same clone with one commit's loose README.md blob rewritten
  under its own SHA (`git cat-file` serves the payload; `git fsck` reports
  "hash-path mismatch"): the fetch fails "did not send all necessary objects",
  the script prints "did not verify — refusing to build", rc 1, `--out` empty.

The version preflight still reads `app.go` with `git show` from the shared
store. It decides nothing that ships: a substituted `app.go` blob is one the
fetch then refuses, so the run stops either way.

Not closed here:
- ranger-base-21i98
