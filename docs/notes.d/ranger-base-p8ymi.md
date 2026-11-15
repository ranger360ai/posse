# posse upgrade tests and builds from a private, re-hashed copy (ranger-base-p8ymi)

ADR 0092 D3.2-D3.6, as amended by this bead. Found by ranger-base-8wi4p's
review of the verb as built.

## The hole this closes

Every D3 anchor in `internal/posse/upgrade.go` is a commit sha, and every one
of them was read out of the source checkout's object store: D3.1's ancestry
checks, D3.2's `git worktree add --detach <sha>` that `make test` ran in, and
the epilogue's scripts. git trusts an object's bytes by the name it is filed
under and does not re-hash a loose or packed object on a local read, and
every walled worktree seat is granted `<common>/objects`. So a seat could
rewrite one blob origin/main reaches, every sha would stay the same, and the
verb's caller — the install delegate, uncaged — would check out and run that
code as `make test`. ranger-base-la61c had already closed the build half
(clean-build.sh fetches through index-pack), so what was left was the suite
and the epilogue running unverified bytes, and D3.1 answering from commit
objects that had not been re-hashed.

## What changed

`upgradeWorktree` is gone. `upgradePrivateCopy` takes its place:

1. `git init` a new repository under a fresh `mktemp -d`.
2. `git fetch --no-tags <repo> <tip>` — the tip of `refs/remotes/origin/main`
   that `upgradeResolve` read, by full sha, now carried on the plan as `Tip`.
   The receiving side hashes every object; one whose bytes do not hash to its
   name leaves the tree incomplete and the fetch fails.
3. `FETCH_HEAD` must equal the tip.
4. D3.1's two ancestry answers are taken AGAIN in the new store: `to` is
   reachable from the tip, and `from` from `to`. A forged parent in the shared
   store cannot survive this, because the new store holds only re-hashed
   history.
5. `refs/remotes/origin/main` is set to the tip in the copy (the worktree the
   suite used to run in could see it), then `checkout --detach <to>`.

`make test`, `clean-build.sh` (whose own fetch then reads the private store)
and the epilogue all run from that copy. Every git call carries
`--no-replace-objects`, which sets `GIT_NO_REPLACE_OBJECTS` for the spawned
upload-pack too — a belt, since refs/replace is not seat-writable today.
`POSSE_BUILD_SOURCE` still names the real source checkout.

A refusal happens before `upgradeTest`, writes nothing, and names `git fsck`.

## Measurements

MEASURED 2026-11-14, git 2.56.0, linux 7.2.15, scratch repos only:

- fetch by full sha from a local path into an empty repository works with no
  `uploadpack.allow*` setting.
- a substituted loose blob: `cat-file` serves the seat's bytes under the
  original sha, a worktree add checks them out under an unchanged HEAD, and
  the fetch refuses ("did not send all necessary objects").
- a commit object rewritten to claim another parent: the fetch refuses.
- a `checkout` after a refused fetch EXITS 0 over an incomplete tree, so the
  fetch's own exit is what has to refuse — and does.

Cost, this repo's origin/main tip (312f823c, 2,773 commits), three runs at a
load average of 2.1-2.4: init+fetch 1.04s, 1.08s, 1.05s; checkout 0.09s each;
a 21M store. Against a 15-25 minute suite, nothing.

## Pins

`internal/posse/upgradeprivatecopy_test.go`:

- `TestUpgradeRefusesASubstitutedBlobBeforeTheSuite` plants a substituted
  blob reachable from origin/main, shows (as its premise) a worktree add
  checking out the seat's bytes, and asks the verb to refuse before the suite.
- `TestUpgradeRefusesAnAncestryTheReHashedHistoryDoesNotBack` hands the run a
  plan whose `to` (then `from`) the shared store vouched for and the real
  history does not.

`TestUpgradeGreenPathSwapsAndEndsWithTheRecordLine` now asserts the suite's
checkout has its own object store, carries origin/main at the tip, and is the
same directory the build and the epilogue are given.

Negative controls, run: putting a worktree add back reds all three pins on
their own messages; dropping the two ancestry re-checks reds the ancestry pin.

## Not closed here

- ranger-base-21i98
