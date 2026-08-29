# The clean room — testing the public install story on a machine that has never seen posse

`ranger-base-5zh`. Built for `ranger-base-33k` (the install QA) and for
verifying any fix to `ranger-base-253`. Four distros since `ranger-base-5cj4`.

A throwaway container with a **default PATH**, a newcomer's Go toolchain, and
nothing whatsoever from this project or from the dev box.

**Its value is in what it does not contain.** The dev box has herdr, bd, a
seeded `RHQ_HOME`, a warm module cache and a PATH curated over weeks — every
one of those hides a defect from a new user. A test that inherits any of them
passes while the bug is still there.

> **Do not add `~/go/bin` to PATH in this image.** That omission *is* the P1
> under test. `go install` writes there, it is not on a default PATH, and the
> next advertised command dies with `command not found`. Put it on PATH and the
> P1 becomes invisible and the whole exercise is silently defeated.
>
> This is also why the image does not use the official `golang` base images:
> they ship `ENV PATH=/go/bin:...` and would defeat it for you.

## Four distros — and why they are here rather than in `ci.yml`

The operator's platform ask, 2026-08-26, was *"not ubuntu, we want macos,
omarchy and rhel/fedora."* The route picked to cover the two Linux families
(`ranger-base-5cj4`) was **this instrument**, not matrix rows in
`.github/workflows/ci.yml`. The reason is measured and it is the whole design:

- **`go test ./...` cannot tell one linux distro from another.** The failure
  set of `make test` is byte-identical on Debian and Fedora. There is no cgo,
  release artifacts build `CGO_ENABLED=0`, and the only OS-conditional source
  is GOOS-level. A distro matrix row cannot make the test binary say anything
  it does not already say on `ubuntu-latest`.
- **The userland can.** The hooks posse generates are *shell*, and the install
  story is *commands*. That is where distro variance lives, and it is exactly
  what the clean room already looks at. `ranger-base-rmgz` — the
  `prepare-commit-msg` hook losing its revert-recovery paragraph on a box with
  no `cmp` — was found this way and could not have been found the other way.

**The first run of the finished set widened that defect.** `cmp` is absent on
**three of the four** — `fedora`, `rhel` and `arch`; only `debian` ships it in
the newcomer baseline. `ranger-base-rmgz` and the runbook had both recorded
Fedora as shipping diffutils by default, because the image those numbers came
from had run `dnf install golang git make` first and picked diffutils up
transitively. Measured against a clean baseline it does not. That correction
cost one command and is the clearest argument for the route there is.

Full costing, with every reproduction command: `docs/runbooks/ci-platform-coverage.md`.

| `CLEANROOM_DISTRO=` | base image | notes |
|---|---|---|
| `debian` *(default)* | `debian:trixie-slim` | Every clean-room pass before `ranger-base-5cj4` ran in this one. |
| `fedora` | `fedora:44` | Fedora userland. Reproduces `ranger-base-rmgz`. |
| `rhel` | `almalinux:10` | The RHEL family. Reproduces `ranger-base-rmgz`. |
| `arch` | `archlinux:latest` | Arch base. **amd64 only** — see below. Reproduces `ranger-base-rmgz`. |

Each distro has its own image tag (`posse-cleanroom-<distro>:1`) and its own
container name, so all four can sit side by side without clobbering each other.

> The pre-`ranger-base-5cj4` image was tagged `posse-cleanroom:1` with a
> container named `posse-cleanroom`. Nothing uses those names any more. If one
> is still on your box it is inert — `docker rmi posse-cleanroom:1` and
> `docker rm posse-cleanroom` when you want the space back.

> ### `arch` is not omarchy. Never write it up as though it were.
> omarchy is Arch **plus** a curated desktop and a dotfiles layer.
> `archlinux:latest` is Arch without it. This image covers the Arch base
> userland — pacman's packages, the filesystem layout, the default PATH — and
> covers none of omarchy's own layer, no desktop, no kernel. A row labelled
> "omarchy" would *read* as coverage and would not be one.

> ### `arch` has no arm64 image and runs under emulation here.
> Official Arch is x86_64-only; `docker manifest inspect archlinux:latest`
> lists amd64 and nothing else. `cleanroom.sh` therefore pins that distro to
> `linux/amd64` whatever the host is, so on an Apple Silicon box it is
> qemu-emulated and slow. That is a permanent tax on this distro. Under
> emulation `pacman` 7.1.0 also dies with `error restricting syscalls via
> seccomp: 22` — `--security-opt seccomp=unconfined` does **not** fix it and
> `pacman --disable-sandbox` does, which is why `Dockerfile.arch` passes it.

## Drive it

From the repo root. `make` targets exist for the common ones; the script has
the rest.

**Every command below drives `debian` unless you say otherwise.** Prefix with
`CLEANROOM_DISTRO=<fedora|rhel|arch>` to drive another — it works on the `make`
targets too, since make passes the environment through to the recipe.

| what | command |
|---|---|
| list the distros | `make cleanroom-distros` |
| build the image (once, per distro) | `scripts/cleanroom.sh build` |
| start it | `make cleanroom` |
| **check it is still honest** | `make cleanroom-verify` |
| **…on every distro** | `make cleanroom-verify-all` |
| what the hooks need vs what this distro has | `make cleanroom-hook-deps` |
| get a shell in it | `make cleanroom-shell` |
| **reset to pristine** | `make cleanroom-reset` |
| run one command in it | `scripts/cleanroom.sh run 'go version'` |
| copy a file in | `scripts/cleanroom.sh cp-in ./notes.md` |
| copy a file out | `scripts/cleanroom.sh cp-out transcript.txt ./transcript.txt` |
| is it up, and what is it? | `scripts/cleanroom.sh status` |
| remove the container | `scripts/cleanroom.sh destroy` |

`build` is needed once per distro; `start` and `reset` recreate the container
from the image and take about a second. Requires Docker Desktop to be running.

```sh
CLEANROOM_DISTRO=rhel make cleanroom-verify     # one distro
make cleanroom-verify-all                       # all four, keeps going after a red one
```

`cleanroom-verify-all` does not stop at the first failure — it walks all four
and names the bad ones at the end. Stopping early would leave the rest
unmeasured and the run would still read as a verdict.

### Start
```sh
make cleanroom          # start, pristine
make cleanroom-verify   # assert every guarantee before you begin
make cleanroom-shell    # you are `tester` in a login shell, type as a user would
```

### Reset — one step, back to pristine
```sh
make cleanroom-reset
```
Destroys the container and recreates it from the unchanged image: home
directory empty, no `~/go`, no module cache, no `~/.config/rhq`. **Reset
between every pass.** A run that starts dirty is worthless — the second
`go install` would hit a warm cache and stop exercising the public fetch.

To rebuild the image itself (e.g. a newer Go), `scripts/cleanroom.sh build`.

### Files in and out
```sh
scripts/cleanroom.sh cp-in  ./scratch/notes.md      # -> /home/tester/notes.md
scripts/cleanroom.sh cp-out transcript.txt ./out/   # <- /home/tester/transcript.txt
```
Paths without a leading `/` are relative to `/home/tester`. Copied-in files are
chowned to `tester`. There is **no shared mount with the host** — that is
deliberate, a shared home would leak the dev box straight into the clean room.

Get the transcript out this way rather than by scrolling: the bead asks for
verbatim commands, exit statuses and output.

### `hook-deps` — what the generated hooks need, per distro

```sh
CLEANROOM_DISTRO=rhel make cleanroom-hook-deps
```

The probe that pays for the whole multi-distro route. The hooks posse renders
(`internal/rhq/gates.go`) are **shell**, and shell is where distro variance is
visible at all. It reports each of the external commands those hooks call
against this distro's userland, and exits non-zero on any `MISSING`.

**A `MISSING` line is a FINDING. Do not install the package to make it go
away** — that would hide exactly what the image exists to surface. File it;
`ranger-base-rmgz` is the shape.

The command list is a **contract that can drift**. It was enumerated from the
rendered hooks on 2026-08-28. If `gates.go` learns to call a new external
command, the `HOOK_DEPS` default in `scripts/cleanroom.sh` must learn it too,
or the probe goes quiet about it. Override for a one-off with
`HOOK_DEPS='a b c'`.

## Rules for a test pass

- **Always enter through a login shell.** `make cleanroom-shell` and
  `cleanroom.sh run` both do. A bare `docker exec ... bash` gets Docker's own
  PATH, which no real user has, and `go` will not be found. Never bypass them.
- **Type the commands as written.** Never by absolute path, never with `GOBIN`
  or `PATH` helpfully pre-set. The moment you fix the environment to get past a
  step, the test is over — that fix is a **finding**, not a workaround.
- **`make cleanroom-verify` before each pass.** It re-proves the guarantees.
  If it fails, do not test in here until it is fixed.

## What is guaranteed, and how

`make cleanroom-verify` asserts all of these by execution:

| guarantee | how it is met |
|---|---|
| Fresh OS, no snapshot of the dev box | The distro's official base image pulled from Docker Hub, built from `etc/cleanroom/Dockerfile.<distro>`. Nothing is copied from this machine. |
| **It is the distro it claims to be** | `verify` reads `/etc/os-release` inside the container and asserts `ID` matches the selected distro. Without this a stale image, or a hand-set `CLEANROOM_IMAGE`, passes every other check while measuring a distro nobody asked for. |
| No shared home directory | No bind mounts at all. `/home/tester` is created empty by `useradd`. |
| **DEFAULT PATH** | The distro's own default plus the single line `https://go.dev/doc/install` tells a newcomer to add — on Debian that is `/usr/local/bin:/usr/bin:/bin:/usr/local/games:/usr/games:/usr/local/go/bin`. `~/go/bin` is absent; verify asserts no GOPATH-style bin dir is on PATH and that `go install`'s target dir specifically is not. The addition goes in `/etc/profile.d/go.sh`, never an `ENV PATH=`, so it reaches login shells the ordinary way and does not leak into a non-login `docker exec`. |
| Go installed the newcomer way | **go1.27.0**, the official `go1.27.0.linux-<arch>.tar.gz` from `https://go.dev/dl/`, sha256-verified in the Dockerfile, unpacked to `/usr/local/go` — `go.dev/doc/install` verbatim. Both linux checksums are carried in every Dockerfile and selected by BuildKit's `TARGETARCH`, so one file serves amd64 and arm64. **Identical on all four distros on purpose:** see the packaged-Go gap below. |
| Nothing from this project | No herdr, no bd, no posse, no rhq, no `RHQ_HOME`, no `~/.config/rhq`, no checkout, no warmed module cache. Each asserted individually. |
| Real public egress | `GOPROXY` left at its `https://proxy.golang.org` default; `GOBIN`/`GOPATH`/`GOFLAGS`/`GOPRIVATE` unset. Verify reaches `proxy.golang.org` and `github.com` over HTTP without writing to the module cache, so the fetch under test stays real and is not a local replay. |
| Resettable to pristine in one step | `make cleanroom-reset` — container destroyed and recreated from the image. |
| Drivable without a human at the keyboard | `cleanroom.sh shell` / `run` / `cp-in` / `cp-out`. |

### What is preinstalled, and why you need to know

`ca-certificates`, `curl`, `git`, `less`, `make` — a generic newcomer's baseline,
nothing project-specific, **the same five on all four distros**. This is listed
because **an undocumented prerequisite is a finding**, and you cannot judge that
fairly without knowing what the image handed you for free. `git` and `make` in
particular are named nowhere in the landing page's quickstart; the image
supplies them so the repo instructions can be *attempted* at all. If a step
needs something not on this list — a compiler, a package — that is a real
finding, so record it rather than installing it.

One exception, and it is a cost of the harness rather than of posse:
`Dockerfile.fedora` and `Dockerfile.rhel` also install **`util-linux`, for
`su`**. Those base images ship no `su` at all, and `scripts/cleanroom.sh` enters
every container through `su - tester` so the test runs in a real login shell
with a real user's PATH. Count it as instrument, not as a posse prerequisite.

**Nothing is ever installed to make a posse failure go away.** `diffutils` in
particular is installed in none of the four: `ranger-base-rmgz` is exactly the
defect a userland without `cmp` produces, and adding it here would hide the
class of finding these images exist to surface. Whatever each distro happens to
have is that distro's business — `make cleanroom-hook-deps` reports the truth
rather than assuming it.

## Fidelity given up — read before writing a verdict

VirtualBox was the operator's stated preference for isolation fidelity. It is
installed on this box and works, but standing up a VM needed a multi-GB OS ISO
download onto a boot volume with **24 GiB free of 460 GiB (95% full)**, plus a
VM disk on top. That is a change to the operator's machine and a real risk of
filling his disk, so under this bead's boundary the container was taken instead.
What that costs:

1. **The guest OS is Debian 13 / bash, not macOS / zsh.** This is the gap that
   matters. The operator's failure was on macOS. This box reproduces the *same
   defect* — `go install` lands outside a default PATH — but not his exact
   environment. Anything macOS-specific is **out of scope and unverified**:
   Homebrew, `brew install ranger360ai/tap/posse`, `.zprofile`/`.zshrc`, zsh's
   `command not found` handler, Gatekeeper on a downloaded binary. Say so in the
   verdict rather than implying macOS was covered. Note that VirtualBox would
   *not* have closed this gap either — it would also have been a Linux guest.
2. **Shared kernel, not a separate one.** The container shares the Docker
   Desktop Linux VM's kernel. Irrelevant to an install-story test; it would
   matter for anything kernel- or driver-level.
3. **No init system.** Minimal rootfs, no systemd. If the install story ever
   grows a service to start, that path is not exercised here.
4. **One architecture per run, and `arch` cannot have this one.** Every image
   builds for the host architecture by default; `CLEANROOM_PLATFORM=linux/amd64`
   asks for the other, and both Go checksums are carried so it just works. The
   exception is `arch`, which is pinned to `linux/amd64` because official Arch
   publishes no arm64 image at all — on this box that means qemu, which is slow
   and which is a difference from a real Arch machine in its own right.
5. **Docker NAT networking with the host's DNS.** Egress to the public proxy is
   genuine, but a stranger behind a corporate proxy or a captive portal is not
   simulated.
6. **The distro's own packaged Go is never exercised.** All four images take
   the `go.dev/dl` tarball, because this instrument exists to isolate the
   *distro* and taking each distro's package would vary the toolchain at the
   same time. But `dnf install golang` and `pacman -S go` are plausible
   newcomer routes on Fedora, RHEL and Arch — each ships a Go new enough to
   build this repo — and **no clean room covers them.** Named, uncovered, and
   cheap to add later as a variant if it is ever worth it.
7. **`arch` is the Arch base, not omarchy**, and no container route reaches
   omarchy's curated desktop and dotfiles layer. Repeated here because it is
   the easiest sentence in a verdict to get wrong.

## Proof it reproduces the bug

Run on the finished image before handing it over — the point being that a clean
room that does not reproduce the defect is not a clean room:

```
$ go install github.com/ranger360ai/posse/cmd/posse@latest
go: downloading github.com/ranger360ai/posse v0.0.0-20260824022158-2d454c50b4bf
go: downloading golang.org/x/term v0.45.0
go: downloading golang.org/x/sys v0.47.0
exit=0

$ posse new myproj --dir ~/code/myproj --cmd claude
-bash: line 1: posse: command not found
exit=127

$ ls -l ~/go/bin/
-rwxrwxr-x 1 tester tester 10925248 Aug 24 12:33 posse
```

Command one succeeds, command two is `command not found`, and the binary is
sitting in `~/go/bin` where nothing can reach it. That is the operator's report,
reproduced — `ranger-base-253` exactly. The container was reset to pristine
afterwards, so the module cache is cold for the first real pass.

This was a smoke test of the instrument, not the QA pass. The pass is
`ranger-base-33k`.
