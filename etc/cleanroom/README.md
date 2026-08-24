# The clean room — testing the public install story on a machine that has never seen posse

`ranger-base-5zh`. Built for `ranger-base-33k` (laurie's install QA) and for
verifying any fix to `ranger-base-253`.

A throwaway Debian 13 container with a **default PATH**, a newcomer's Go
toolchain, and nothing whatsoever from this project or from the dev box.

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

## Drive it

From the repo root. `make` targets exist for the common three; the script has
the rest.

| what | command |
|---|---|
| build the image (once) | `scripts/cleanroom.sh build` |
| start it | `make cleanroom` |
| **check it is still honest** | `make cleanroom-verify` |
| get a shell in it | `make cleanroom-shell` |
| **reset to pristine** | `make cleanroom-reset` |
| run one command in it | `scripts/cleanroom.sh run 'go version'` |
| copy a file in | `scripts/cleanroom.sh cp-in ./notes.md` |
| copy a file out | `scripts/cleanroom.sh cp-out transcript.txt ./transcript.txt` |
| is it up? | `scripts/cleanroom.sh status` |
| remove the container | `scripts/cleanroom.sh destroy` |

`build` is needed once; `start` and `reset` recreate the container from the
image and take about a second. Requires Docker Desktop to be running.

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
| Fresh OS, no snapshot of the dev box | `debian:trixie-slim` pulled from Docker Hub, built from `etc/cleanroom/Dockerfile`. Nothing is copied from this machine. |
| No shared home directory | No bind mounts at all. `/home/tester` is created empty by `useradd`. |
| **DEFAULT PATH** | `/usr/local/bin:/usr/bin:/bin:/usr/local/games:/usr/games:/usr/local/go/bin` — Debian's default plus the single line `https://go.dev/doc/install` tells a newcomer to add. `~/go/bin` is absent; verify asserts no GOPATH-style bin dir is on PATH and that `go install`'s target dir specifically is not. |
| Go installed the newcomer way | **go1.27.0**, the official `go1.27.0.linux-arm64.tar.gz` from `https://go.dev/dl/`, sha256-verified in the Dockerfile, unpacked to `/usr/local/go` — `go.dev/doc/install` verbatim. |
| Nothing from this project | No herdr, no bd, no posse, no rhq, no `RHQ_HOME`, no `~/.config/rhq`, no checkout, no warmed module cache. Each asserted individually. |
| Real public egress | `GOPROXY` left at its `https://proxy.golang.org` default; `GOBIN`/`GOPATH`/`GOFLAGS`/`GOPRIVATE` unset. Verify reaches `proxy.golang.org` and `github.com` over HTTP without writing to the module cache, so the fetch under test stays real and is not a local replay. |
| Resettable to pristine in one step | `make cleanroom-reset` — container destroyed and recreated from the image. |
| Drivable without gilfoyle at the keyboard | `cleanroom.sh shell` / `run` / `cp-in` / `cp-out`. |

### What is preinstalled, and why you need to know

`ca-certificates`, `curl`, `git`, `less`, `make` — a generic newcomer's baseline,
nothing project-specific. This is listed because **an undocumented prerequisite
is a finding**, and you cannot judge that fairly without knowing what the image
handed you for free. `git` and `make` in particular are named nowhere in the
landing page's quickstart; the image supplies them so the repo instructions can
be *attempted* at all. If a step needs something not on this list — a compiler,
a package — that is a real finding, so record it rather than installing it.

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
4. **arm64 only.** No amd64 coverage. The Dockerfile carries the amd64 checksum
   in a comment-adjacent `ARG` pair — flip `GO_ARCH`/`GO_SHA256` if that is ever
   needed (amd64 sha256:
   `675c26c449cbb18fc24b74650de1eabbae6e16f64326fd85a283fb3b58280685`).
5. **Docker NAT networking with the host's DNS.** Egress to the public proxy is
   genuine, but a stranger behind a corporate proxy or a captive portal is not
   simulated.

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
