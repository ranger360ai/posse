## The clean room grew three distros, and its first run widened a defect (ranger-base-5cj4)

The operator picked **route D for both platforms** on 2026-08-28 — cover
rhel/fedora and omarchy/arch with clean-room images, not with rows in
`ci.yml`. `ci.yml` and `scripts/test-linux.sh` are unchanged; the work is
`etc/cleanroom/Dockerfile.{debian,fedora,rhel,arch}` and a `CLEANROOM_DISTRO=`
seam in `scripts/cleanroom.sh`. Costing and correction:
`docs/runbooks/ci-platform-coverage.md`.

**The instrument found something on its first run, and it was not what was
filed.** `ranger-base-rmgz` says the generated `prepare-commit-msg` hook calls
`cmp`, that `cmp` ships in diffutils, and that a minimal AlmaLinux/RHEL box
lacks it while "Debian, Ubuntu and Fedora" have it. Measured against the four
finished images, each carrying only the newcomer baseline `ca-certificates curl
git less make`:

| distro | `cmp` |
|---|---|
| `debian:trixie-slim` | ok |
| `fedora:44` | **MISSING** |
| `almalinux:10` | **MISSING** |
| `archlinux:latest` | **MISSING** |

So it is three distros of four, not one family, and the Fedora half of the
original claim was wrong.

**Why the original number was wrong is the transferable part.** It came from
`probe-fedora:1`, an image built as `FROM fedora:latest` + `dnf -y install
golang git make` so that the Go suite could be run in it. `dnf` pulls diffutils
in transitively to satisfy that, so the probe measured **its own scaffolding**
and reported it as what Fedora ships. *An image built to run the suite is not
an image for measuring what the distro gives a user.* Anything asking "what
does this platform have out of the box" has to be asked of a box with only the
baseline on it, which is exactly what the clean room is for and exactly why a
`ci.yml` container row — which must install a toolchain before it can do
anything — could not have answered it.

Reproduce:

```sh
make cleanroom-verify-all                        # all four
CLEANROOM_DISTRO=fedora make cleanroom-hook-deps # one
```

Three things worth keeping from building it:

- **`--platform` goes on `docker build` as well as `docker run`.**
  `scripts/test-linux.sh` learned this on `run` (ranger-base-1qm5, a tag left
  holding the wrong architecture's blob). A per-distro build has the same
  exposure and now names the platform in both directions. Arch is pinned to
  `linux/amd64` unconditionally — official Arch publishes **no** arm64 image —
  so on this box it is emulated, and `pacman` 7.1.0 needs `--disable-sandbox`
  under emulation (`--security-opt seccomp=unconfined` does not fix it).
- **A multi-distro instrument needs an identity assertion.** `verify` now reads
  `/etc/os-release` inside the container and asserts `ID`. The control matters:
  pointed at the debian container while claiming fedora, that one check fails
  and *every other check still passes* — a stale image or a hand-set
  `CLEANROOM_IMAGE` would otherwise produce a full green page about a distro
  nobody asked for.
- **`hook-deps`'s command list is a contract that can drift.** It was
  enumerated from the rendered hooks for ranger-base-rmgz. If `gates.go` grows
  a call to a new external command, the list in `scripts/cleanroom.sh` must
  grow with it or the probe goes quiet about it. Written down there and in
  `etc/cleanroom/README.md` rather than left to be rediscovered.

And one honest limit, because it is the easiest sentence to get wrong in a
verdict: **`arch` is the Arch base, not omarchy.** omarchy is Arch plus a
curated desktop and dotfiles layer, and no container route reaches it. Nor does
any of this reach a kernel, an init system, or the macOS install routes — those
stay open on `ranger-base-hza`.
