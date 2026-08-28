# RUNBOOK — cutting a posse release

The maintainer's procedure for `vX.Y.Z`: four tarballs, a GitHub Release,
and the Homebrew formula that `brew install ranger360ai/tap/posse` resolves.
Deployers do not need this file — they need `INSTALL.md`. This is the other
side of INSTALL.md step 2.

Five things in this repo point here (`Makefile`, `INSTALL.md` §15,
`scripts/tap-formula.sh`, and `.github/workflows/release.yml` twice), because
the machinery deliberately stops short of publishing anything.

**What is automated:** pushing a `vX.Y.Z` tag fires
`.github/workflows/release.yml`, which vets, runs `make test`, builds the four
tarballs, renders the formula, and stages a **draft** release.

If that run needs to be retried, dispatch the workflow manually with the
existing tag. `workflow_dispatch` checks out the requested tag before vetting
and testing it, and the artifacts are built from that same commit.

**What is not, on purpose:** publishing that draft and pushing the formula to
the tap are outward-facing acts (crew guardrail 4) and stay in the operator's
hands. Nothing in CI touches `ranger360ai/homebrew-tap`.

---

## The one irreversible step

**The tag.** Everything else here can be redone; a version number cannot be
reused. The Go module proxy caches `vX.Y.Z` immutably on first fetch, so a tag
that shipped a bad build is spent even if you delete it. A tag that fires a
workflow which dies before `build artifacts` burns the number and produces no
release at all.

So the preconditions below are not a formality. Check them; the delay costs
nothing and a burned version is permanent.

---

## Preconditions — all of them, before the tag

**1. The version in the source already equals the tag.**

```sh
$ grep 'Version ' internal/rhq/app.go
```
**Verify:** `Version = "X.Y.Z"` matches the `vX.Y.Z` you are about to cut.
`internal/rhq.Version` is a `const` — it cannot be stamped from outside, and
`scripts/release-artifacts.sh` refuses a build where the two disagree, because
a binary whose `posse version` contradicts its own download URL is worse than
no release. **Bumping a release is therefore: edit `app.go`, merge, then tag —
in that order.**

**2. The tag lands on a commit that is on `origin/main`.**

```sh
$ git fetch origin && git log --oneline -1 origin/main
$ git rev-list --left-right --count origin/main...HEAD
```
**Verify:** the right-hand number is `0`. A local commit ahead of `origin/main`
is not a taggable commit — CI checks out what GitHub has, not what your working
tree has.

**3. `make test-linux` is green, at that exact commit.**

CI is `ubuntu-latest`. Darwin-green proves nothing about it: two real bugs
(`ranger-base-fjj`, `ranger-base-gaf`) survived the macOS suite and were found
only during a release rehearsal. Run the repository's Linux gate before the
tag, while the version number is still free:

```sh
$ make test-linux
```
**Verify:** exit 0, both packages `ok`, and `silent-revert audit: N commits, 0
untriaged`. Run it against a **clean clone of the commit being tagged**, not a
dirty working tree.

`make test-linux` derives the toolchain from `go.mod`, mounts the repository
read-only, and runs the container as `$(id -u):$(id -g)`. Its writable build and
module caches live outside the repository, so the rehearsal cannot leave a
root-owned artifact or rewrite the tree.

The target runs `go vet ./...` followed by `make test`, matching the release
workflow. `make test` includes the silent-revert audit, which detects the
failure class a green Go suite does not report (`rangerhq-8rtf`).

**4. Doc fixes to `README.md` and `INSTALL.md` are merged BEFORE the tag.**

The tarball ships `posse`, `LICENSE`, `README.md` and `INSTALL.md`, and the
formula installs the last two into `doc/` — its caveats send the reader there
("The cold-start runbook is #{doc}/INSTALL.md"). **The documentation a brew
user reads is frozen at the tag, not at `main`,** and a published tarball
cannot be corrected in place: the fix rides the next release.

v0.3.0 is the worked example. It was cut at `8595b8a`; the brew route was
corrected to its three-command form (the `brew trust` line, `ranger-base-4mg`)
one commit later in `adf9637`. So every v0.3.0 tarball ships the one-line
sequence that stops at Homebrew 6.x's trust refusal, and its INSTALL.md still
explains an `Error: Failure while executing tap` that now means something else.
Nothing is wrong with the release; the doc simply predates the fix. Check
before tagging rather than after:

```sh
$ tar -xzOf dist/posse_X.Y.Z_darwin_arm64.tar.gz INSTALL.md | sed -n '/^## 2\./,/^## 3\./p'
```
**Verify:** the install sequence in the *extracted* file is the one you want a
stranger to follow.

**5. Optionally, rehearse the build itself** — the workflow's remaining steps,
on Linux, at the same commit:

```sh
$ scripts/release-artifacts.sh --rev <sha> --version vX.Y.Z
$ scripts/tap-formula.sh --version vX.Y.Z --checksums dist/checksums.txt --out dist/posse.rb
$ (cd dist && sha256sum -c checksums.txt)
```
Neither script tags, publishes, or talks to GitHub; both write `dist/` and stop.

`--out` is **emptied** before the build, so `release-artifacts.sh` refuses an
`--out` holding anything it did not write — and refuses `/`, `$HOME` and the
repo root by name (`ranger-base-9hyc`). There is no `--force`: if a directory
of yours is in the way, type the `rm` yourself, so the blast radius is on a
command line you wrote rather than on a mistyped flag at the step next to the
irreversible one.

---

## Step 0 — push the tag

```sh
$ git tag vX.Y.Z <the sha from precondition 2>
$ git push origin vX.Y.Z
```

This fires the workflow. **Watch it.** It must reach `Draft release vX.Y.Z
staged.` If it dies before `build artifacts`, the version number is spent — fix
forward to the next patch, do not re-cut.

`vX.Y.Z` is the only shape `resolve the tag` accepts — `v`, digits and dots,
nothing else — because that string is what the five steps after it hand to a
shell, and git will happily let you tag `v1.0$(id)` (ranger-base-qqxm). A
suffixed tag such as `v0.4.0-rc1` is refused there, loudly, before anything is
built; widening the guard is a deliberate edit to `.github/workflows/release.yml`.

## Step 1 — publish the draft *(operator)*

`github.com/ranger360ai/posse/releases` → the `vX.Y.Z` **draft**.

Six assets must be present before you press Publish:

```
posse_X.Y.Z_darwin_arm64.tar.gz    posse_X.Y.Z_darwin_amd64.tar.gz
posse_X.Y.Z_linux_arm64.tar.gz     posse_X.Y.Z_linux_amd64.tar.gz
checksums.txt                      posse.rb
```

Nothing is downloadable until you publish — and the formula's URLs 404 until
you do, which makes step 3 fail in a way that looks like a bad formula.

## Step 2 — create the tap *(operator, once ever)*

A new **public** repo, owner `ranger360ai`, named **exactly**:

```
homebrew-tap        ->  github.com/ranger360ai/homebrew-tap
```

The name is not a preference. `brew install <owner>/tap/<formula>` expands
`<owner>/tap` to `<owner>/homebrew-tap`. Any other name — `tap`,
`homebrew-posse`, `posse-tap` — and INSTALL.md step 2 keeps failing with
`Error: Failure while executing tap`, which never says the tap does not exist.

## Step 3 — commit the formula *(operator)*

**Download `posse.rb` from the published release. Do not regenerate it
locally.** The tarball sha256s are not reproducible — gzip stamps a timestamp,
so two runs of the same commit differ — and the only `posse.rb` whose hashes
match the *uploaded* tarballs is the one built in that same workflow run. A
regenerated formula installs fine on the machine that made it and fails
checksum verification everywhere else.

```sh
$ git clone https://github.com/ranger360ai/homebrew-tap
$ cd homebrew-tap && mkdir -p Formula
$ cp ~/Downloads/posse.rb Formula/posse.rb
$ git add Formula/posse.rb && git commit -m "posse X.Y.Z" && git push
```

The directory must be `Formula/` and the file `posse.rb` — brew resolves the
formula name from the filename.

## Step 4 — prove it, on a machine that is not the one that built it

```sh
$ brew tap ranger360ai/tap
$ brew tap-info ranger360ai/tap                  # expect: Untrusted, on a machine that has not trusted us
$ brew trust --formula ranger360ai/tap/posse     # this one formula, not the tap
$ brew install ranger360ai/tap/posse
$ posse version
$ which posse
```
**Verify:** `posse X.Y.Z+<sha>`, and `which posse` resolves inside the brew
prefix — **not** `~/.local/bin/posse`. A machine with a `make install` binary
on `$PATH` will answer `posse version` from that binary and hide a broken brew
install, so a box with one is the wrong instrument for this step. This is the
same sequence INSTALL.md step 2 gives a deployer; until it passes here, that
step is advertising something that does not work.

The trust line is not decoration. Homebrew 6.x ignores third-party taps until
they are trusted, and the narrow `--formula` form grants this one formula
rather than everything the tap will ever carry. It is non-interactive — no
prompt, exit 0 — so it is safe inside a dispatched session; on some brew
versions the fully-qualified `brew install` grants it for you and the line
prints `Already trusted`. Run it anyway: it costs nothing when it is redundant.
INSTALL.md step 2 carries the reader-facing version of this (ranger-base-4mg).

**If it fails: fix forward to the next patch version. Do not delete and re-cut
the tag.** The Go proxy's cache is immutable and not ours to purge.

## Step 5 — verify the published chain, from anywhere

Step 4 needs a macOS machine that is not ours, which we do not reliably have
(`ranger-base-hza`) — and it only ever proves the *one* architecture the person
running it happens to have. This check needs no brew, no macOS and no trust
grant, runs in the Linux clean room, and covers all four:

```sh
$ base=https://github.com/ranger360ai/posse/releases/download/vX.Y.Z
$ curl -sL -O "$base/checksums.txt" -O "$base/posse.rb"
$ for a in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
>   curl -sLO "$base/posse_X.Y.Z_$a.tar.gz"; done
$ shasum -a 256 -c checksums.txt          # GitHub's bytes match the manifest
$ curl -sL -o tap.rb https://raw.githubusercontent.com/ranger360ai/homebrew-tap/main/Formula/posse.rb
$ diff posse.rb tap.rb                     # the tap serves the release's own formula
$ for h in $(grep -o 'sha256 "[a-f0-9]*"' tap.rb | cut -d'"' -f2); do
>   grep -q "$h" checksums.txt && echo "OK $h" || echo "MISS $h"; done
```
**Verify:** four `OK` lines, `diff` silent, all four tarballs `OK`.
(`shasum -a 256 -c` is the macOS spelling; `sha256sum -c` is GNU's. The
`golang` image carries both, a bare Debian may carry only the second.)

This is the check that catches the failure `scripts/tap-formula.sh` exists to
prevent — one stale sha256 in the formula, which fails `brew install` for
exactly one architecture: the one the person who cut the release does not use.
A successful install on the maintainer's laptop cannot see it.

---

## What is still unproven from a developer machine

Stated plainly, so nobody mistakes this runbook for a guarantee:

- **`gh release create`** is GitHub state and is never exercised locally.
  `permissions: contents: write` covers it, `gh` is preinstalled on
  `ubuntu-latest`, and `--generate-notes` works with no prior release — read,
  not run.
- **linux/amd64 unless explicitly requested.** On an Apple-silicon machine,
  plain `make test-linux` runs linux/arm64 while GitHub's `ubuntu-latest` is
  amd64. Run `PLATFORM=linux/amd64 make test-linux` to cover the CI architecture
  under emulation; the default rehearsal alone does not prove it. Running it is
  safe to repeat: the script passes `--platform` on every run, defaulting to the
  host, so an amd64 rehearsal no longer leaves the next default run emulated
  (ranger-base-1qm5).
