## Linux distro variance is invisible to the Go suite and visible in the shell we generate (ranger-base-4fxz)

Costing CI rows for the platforms `ci.yml` does not reach — omarchy (Arch) and
rhel/fedora — turned on one question: *would a distro row report anything the
existing ubuntu + macOS rows do not?* Measured both ways, 2026-08-28. Full plan
and reproduction commands: `docs/runbooks/ci-platform-coverage.md`.

**No, for `go test`.** `make test` produces a byte-identical failure set on
Debian and on Fedora — non-root, `/repo` read-only, i.e. `make test-linux`
conditions:

| image | failures |
|---|---|
| `golang:1.26` (Debian trixie) | 5 `TestQueue*` + 2 `TestQAInstallRefusal*` |
| Fedora 44 + its own `golang 1.26.7` | *the same 7* |
| AlmaLinux 10.2 + its own `golang 1.26.7` | those 7 **+ 2 `TestQAGuard*`** |

There is no cgo in the repo (`import "C"` count: zero), release artifacts build
`CGO_ENABLED=0`, and the only OS-conditional source is `loadavg_{darwin,linux,
other}.go` and `seatbelt.go` — every branch keyed on **GOOS, never on distro**.
The compiled test binary cannot tell Fedora from Debian, so no matrix row can
make it say something new. Every distro named also ships a Go ≥ `go.mod`'s
1.26.5 in its own package manager, so no route needs a hand-installed toolchain.

**Yes, for the userland.** AlmaLinux's two extra failures are one cause, and it
is a product defect rather than a test artifact (ranger-base-rmgz):

```
.git/hooks/prepare-commit-msg: line 193: cmp: command not found
```

`gates.go:1490` renders `cmp -s "$1" "$posse_gitdir/MERGE_MSG"` to detect a
git-prepared message. `cmp` ships in **diffutils**: installed by default on
Debian, Ubuntu and Fedora, *not* installed on a minimal AlmaLinux/RHEL 10.
Absent, the condition is just false, the wall still refuses, and the paragraph
telling a user mid-`git revert` that git already staged the change — and naming
both ways out — silently disappears. Of the fourteen external commands the
generated hooks call, `cmp` is the only one missing there.

So the useful question is not "which distros should run the suite" but "which
userlands does our generated shell assume". Enumerating the second is cheap and
found a real bug; the first cannot find one by construction.

**Two traps worth keeping.** Official Arch has **no arm64 image** — an Arch row
runs natively on `ubuntu-latest` and only under qemu on an Apple Silicon dev
box, where `pacman` 7.1.0 dies with `error restricting syscalls via seccomp:
22`; `--security-opt seccomp=unconfined` does *not* help, `pacman
--disable-sandbox` does. And omarchy is Arch **plus** a curated desktop layer,
so a container row labelled "omarchy" would read as coverage and not be one.

**Move one variable at a time.** The first pass of this ran the containers as
root and produced a Fedora-only red — `TestBackfillDoesNotFailTheListing`,
"read-only meta was rewritten", deterministic 3/3 — that evaporated non-root:
root bypasses the 0444 the fixture depends on. Distro *and* uid had both moved.
Re-run as the host uid via `IMAGE=<image> scripts/test-linux.sh` and the two
distros agree exactly. A cross-platform probe that also changes the user
measures the user.
