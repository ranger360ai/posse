# CI platform coverage — what the gate reaches, what it cannot, and what each
# route would cost

Status: **costed plan awaiting an operator pick** (ranger-base-4fxz).
Nothing here is built. `.github/workflows/ci.yml` is unchanged by this
document.

The ask, 2026-08-26, verbatim: *"vet on a release schedule, not every push.
begs the question, releases are bigger milestones. not ubuntu, we want macos,
omarchy and rhel/fedora."*

What was then built (ranger-base-160, 97980f2) is `go vet` + `make test` on
every push, on `ubuntu-latest` and `macos-latest`. That gate is real and it
stands. The 2026-08-26 platform list is neither delivered by it nor refused by
it, which is what this document is for.

Every number below was measured on 2026-08-28. The reproduction command is
given with each one; none of it is estimated.

---

## 1. The finding that changes the shape of the answer

**Distro variance is invisible to the Go suite, and visible in the shell posse
generates.** Both halves are measured.

*Invisible to the Go suite.* The failure set of `make test` is byte-identical
on Debian and on Fedora:

| image | posse | cmd/posse | internal/rhq | failures |
|---|---|---|---|---|
| `golang:1.26` (Debian trixie) | ok 4.5s | **FAIL** 33.0s | **FAIL** 90.8s | 5 `TestQueue*` + 2 `TestQAInstallRefusal*` |
| `probe-fedora:1` (Fedora 44) | ok 4.6s | **FAIL** 26.1s | **FAIL** 113.0s | *the same 7* |
| `probe-alma:1` (AlmaLinux 10.2) | ok 5.3s | **FAIL** 69.6s | **FAIL** 150.4s | those 7 **+ 2 `TestQAGuard*`** |

Run as `IMAGE=<image> scripts/test-linux.sh`, non-root, repo mounted `:ro` —
i.e. exactly `make test-linux` conditions.

That the first two rows agree exactly is the point. There is no cgo in this
repo (`import "C"` appears zero times), release artifacts are built
`CGO_ENABLED=0` (`scripts/release-artifacts.sh:213`), and the only
OS-conditional source is `loadavg_{darwin,linux,other}.go` and `seatbelt.go`'s
`runtime.GOOS != "darwin"` — all of it **GOOS-level, none of it
distro-level**. The compiled test binary cannot tell Fedora from Debian, so a
distro row cannot make it say anything new.

*Visible in the generated shell.* AlmaLinux's two extra failures are one
cause, and it is not a test artifact:

```
.git/hooks/prepare-commit-msg: line 193: cmp: command not found
```

`internal/rhq/gates.go:1490` renders

```sh
if [ -f "$posse_gitdir/MERGE_MSG" ] && cmp -s "$1" "$posse_gitdir/MERGE_MSG"; then
```

`cmp` ships in **diffutils**. It is installed by default on Debian, Ubuntu and
Fedora, and is *not* installed on a minimal AlmaLinux/RHEL 10 (it is in
`baseos`, available, uninstalled). Absent, that test is simply false, and the
refusal a user hits mid-`git revert` loses the paragraph that tells them git
already staged the change and names the two ways out. The wall still refuses;
its recovery instructions quietly vanish. Filed as its own bead.

Of the fourteen external commands the generated hooks call — `date tr rm
printf head grep mv mktemp dirname sort env cmp chmod cat` — `cmp` is the
**only** one missing on a minimal RHEL-family box. Verified by enumeration
against `almalinux:10`.

So the honest framing of the 2026-08-26 ask: *macos / omarchy / rhel-fedora*
is the right list for **the userland posse's own shell runs in, and for the
install story**. It is the wrong list for `go test ./...`, which cannot see it.

---

## 2. What is actually available, measured

`docker manifest inspect`, 2026-08-28:

| image | amd64 | arm64 | packaged Go vs `go.mod` (`go 1.26.5`) |
|---|---|---|---|
| `archlinux:latest` | yes | **no** | `go 2:1.27.0` ✓ |
| `fedora:latest` (F44) | yes | yes | `golang 1.26.7-1.fc44` ✓ |
| `almalinux:10` (10.2) | yes | yes | `golang 1.26.7-1.el10_2` ✓ |
| `rockylinux/rockylinux:10` | yes | yes | (not probed) |

Two consequences worth pricing:

- **Every distro named ships a Go new enough to build this repo from its own
  package manager.** No route needs a hand-installed toolchain.
- **Arch has no arm64 image.** Official Arch is x86_64-only; ARM is a separate
  project. So an Arch row runs *natively* on `ubuntu-latest` and only under
  qemu on an Apple Silicon dev box — which inverts the usual "reproduce it
  locally first" order. Under emulation `pacman` 7.1.0 additionally dies with
  `error restricting syscalls via seccomp: 22`. `--security-opt
  seccomp=unconfined` does **not** fix it; `pacman --disable-sandbox` does.
  That is a permanent local-reproduction tax on the Arch row, not a one-off.

Provision cost, warm registry, arm64: `dnf install golang git make` is 24s on
Fedora, ~15s on Alma. Baking it into an image is a 17s and 15s layer
respectively.

---

## 3. The routes, with their real cost and their real coverage

### A — distro containers on a hosted `ubuntu-latest` runner

- **Money:** none. Standard hosted runners are free for public repos, and a
  container job is an ordinary step inside one.
- **Time:** +2–3 min per distro row per push, in parallel with the existing
  jobs, so wall-clock cost is roughly zero and only the runner-count grows.
- **Maintenance:** one image spec per distro. `scripts/test-linux.sh` pins
  `golang:<go.mod minor>` on a Debian base and its pin
  `TestTestLinuxScriptIsTheReleaseGateOnLinux` (ranger-base-lsj) asserts that
  — both need a per-distro seam first. This is not a matrix edit.
- **Coverage bought:** the userland. This is where `cmp` was found, so the
  route has already paid for itself once.
- **Coverage NOT bought:** the kernel (containers share the host's), the
  install routes, systemd, the desktop. A container is not the distro.
- **The omarchy trap:** omarchy is Arch *plus* a curated desktop and dotfiles
  layer. `archlinux:latest` is Arch without it. A row labelled "omarchy" built
  this way would read as coverage and would not be. If this route is picked,
  label the row **arch**, and say in the workflow why it is not omarchy.

### B — self-hosted runners

- **Money and hardware:** the operator's, entirely. Cannot be estimated from
  here.
- **Security:** a self-hosted runner attached to a **public** repo executes
  code from fork pull requests on the operator's machine unless deliberately
  restricted. GitHub documents this as not recommended for public repos. This
  is the dominant cost of the route and it is not a money cost.
- **Coverage bought:** the real distro, real kernel, real install routes —
  everything.
- **Verdict from here:** I cannot provision, fund, or accept the risk on this
  one. Operator decision in full.

### C — hosted macOS

- Already built, already green (`macos-latest`, run 33218587437). Free for
  public repos. Nothing to decide; it is the one item of the 2026-08-26 list
  that is delivered.

### D — put the distros in the clean room, not in `ci.yml` — **recommended**

`etc/cleanroom/` already exists (ranger-base-5zh) and its entire job is the
**public install story** on a machine with a default PATH and nothing from the
dev box. That is precisely the surface distro variance actually touches, and
precisely what a `go test` row cannot reach.

- **Money:** none.
- **CI time:** none — it is run on demand, before a release, not per push.
- **Work:** add a Fedora image and an Arch image beside the Debian one, and a
  `CLEANROOM_IMAGE=` seam in `scripts/cleanroom.sh`. Roughly a day, one bead.
- **Coverage bought:** the install routes per distro — the `curl | sh` herdr
  install, the pinned `bd` tarball, `go install` and PATH, and the generated
  hooks running in that distro's userland. It would have caught `cmp`.
- **Coverage NOT bought:** still not a kernel, still not a desktop, still not
  omarchy's own layer. Honest, and much closer to the ask than a matrix row.
- **Bonus:** it composes with ranger-base-hza, the open operator bead about
  the clean room being unable to test the **macOS** install routes. Same
  instrument, same question, one decision instead of two.

---

## 4. Before any of this: the gate was red — fixed in ranger-base-rstk

`ci.yml`'s first run on main (33218587437, 2026-08-28) was **red on
`ubuntu-latest`, green on `macos-latest`** — the platform split working
exactly as designed, on its first push. Two environment dependencies, both
measured, both filed, **both fixed in the fixtures under ranger-base-rstk**
(`docs/notes.d/ranger-base-rstk.md`). What they were:

1. **Five `TestQueue*` tests assume an ambient git identity.** They commit in
   fixture repos without setting `user.email`, which succeeds wherever git can
   auto-detect one and fails where the hostname has no domain part
   (`root@…(none)`). Discriminating probe: the same tree, same image, with
   `user.name`/`user.email` supplied → **green**. This is what reds
   `ubuntu-latest` while `macos-latest` passes; it is hostname-shaped, not
   OS-shaped, so it is latent everywhere.
2. **Two `TestQAInstallRefusal*` tests write into the working directory.**
   Red under `make test-linux`, whose whole guarantee is that `/repo` is
   mounted read-only; green in CI where the checkout is writable. Ruled out as
   a `zsh` dependency by installing zsh and re-running — still red.

A third, worth recording so nobody re-finds it: `TestBackfillDoesNotFailTheListing`
fails **only when the suite runs as root** ("read-only meta was rewritten" —
root bypasses the 0444 the fixture relies on). It is not a distro failure and
not a real defect; it is why every measurement in this document was taken
non-root.

**Do not add platform rows to a red gate.** Whichever route is picked, the two
filed defects land first, or every new row inherits a failure that has nothing
to do with the platform it is supposed to be testing. That precondition is
**met**: both are fixed, `make test-linux` is green at the fix, and each
mechanism is now pinned by an arm that fails on darwin too — the identity one
through `user.useConfigOnly=true`, which switches git's hostname guessing off
so a missing identity reds every box; the working-directory one through a
scratch cwd that must be left empty, so a prescription whose `cd` does not land
is caught where the checkout is writable as well as where it is read-only.
Neither pin can go quiet on the box a persona develops on, which is how the
originals survived.

---

## 5. The pick

One line per platform is enough to unblock this.

| platform | options | recommendation |
|---|---|---|
| **macOS** | already done | nothing to decide |
| **rhel/fedora** | A (container row in `ci.yml`) · B (self-hosted) · D (clean-room image) · none | **D** |
| **omarchy / arch** | A (labelled *arch*, x86_64 only) · B · D · none | **D**, and never label a container row "omarchy" |

Reasoning in one sentence: the Go suite cannot see the distro, the userland
can, and the clean room is the instrument that already looks at the userland —
so D buys the coverage that was actually asked for, at no CI cost, without
pretending a container is a distro.

If the answer is instead *"ubuntu + macOS is enough"*, that is a complete and
defensible answer; the one thing it should not do is leave the `cmp` defect
unfixed, because that one is real on RHEL-family boxes whether or not anything
ever tests them.

---

## 6. Reproducing any of this

```sh
# the suite on a named distro, non-root, repo read-only
docker build -t probe-fedora:1 - <<'EOF'
FROM fedora:latest
RUN dnf -q -y install golang git make && dnf clean all
EOF
IMAGE=probe-fedora:1 scripts/test-linux.sh

# image architecture availability
docker manifest inspect archlinux:latest | grep '"architecture"' | sort -u

# what the generated hooks need, against a minimal RHEL base
docker run --rm almalinux:10 sh -c \
  'for c in date tr rm printf head grep mv mktemp dirname sort env cmp chmod cat; do
     command -v "$c" >/dev/null || echo "MISSING: $c"; done'
```
