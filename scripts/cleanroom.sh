#!/usr/bin/env bash
# cleanroom.sh — drive the clean-room test environment (ranger-base-5zh).
#
# A throwaway container with a DEFAULT PATH, a newcomer's Go toolchain, and
# nothing at all from this project or this dev box. It exists so the public
# install story can be tested on a machine that has never seen posse.
#
# The guarantee that matters most: ~/go/bin is NOT on PATH in there. That
# omission is the P1 under test (ranger-base-253). `cleanroom.sh verify`
# asserts it, along with every other guarantee, and is worth running before
# each test pass.
#
# FOUR DISTROS (ranger-base-5cj4). The operator's 2026-08-26 platform ask was
# "macos, omarchy and rhel/fedora", and the route picked for the two Linux
# families was this instrument rather than a ci.yml matrix row: `go test ./...`
# cannot tell one linux distro from another (measured — Debian's and Fedora's
# failure sets are byte-identical), while the userland the install commands and
# the generated hooks run in absolutely can. See
# docs/runbooks/ci-platform-coverage.md §3D.
#
#   debian   debian:trixie-slim     the default; every earlier pass ran here
#   fedora   fedora:44
#   rhel     almalinux:10           the RHEL family, where `cmp` is absent
#   arch     archlinux:latest       Arch base. NOT omarchy — see below.
#
# Usage:
#   scripts/cleanroom.sh build          build (or rebuild) the image
#   scripts/cleanroom.sh start          start a fresh container from the image
#   scripts/cleanroom.sh shell          interactive login shell as `tester`
#   scripts/cleanroom.sh run '<cmd>'    run <cmd> in a login shell as `tester`
#   scripts/cleanroom.sh reset          destroy and recreate — back to pristine
#   scripts/cleanroom.sh verify         assert every clean-room guarantee
#   scripts/cleanroom.sh hook-deps      report which commands the generated
#                                       hooks need that this distro lacks
#   scripts/cleanroom.sh cp-in  <src> [dst]   host -> ~tester (dst default: .)
#   scripts/cleanroom.sh cp-out <src> <dst>   ~tester -> host
#   scripts/cleanroom.sh status         is it up, and what is it
#   scripts/cleanroom.sh destroy        remove the container (image is kept)
#   scripts/cleanroom.sh distros        list the distros and what each is
#
# Environment:
#   CLEANROOM_DISTRO=fedora    which image to drive (default: debian)
#   CLEANROOM_IMAGE=...        override the image tag
#   CLEANROOM_NAME=...         override the container name
#   CLEANROOM_PLATFORM=...     override the platform (see the block below)
#
# Each distro gets its OWN image tag and its OWN container name, so all four
# can be built and can sit side by side without one clobbering another.
set -euo pipefail

REPO_ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
HOME_IN=/home/tester

die() { printf 'cleanroom: %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# The distro seam (ranger-base-5cj4)
# ---------------------------------------------------------------------------
# Adding a distro is four things: a Dockerfile.<name>, a row in each of the two
# case statements below, a row in cmd_distros, and a row in the README's table.
# `expected_os_id` is asserted by `verify` against /etc/os-release inside the
# container, so a stale image left under the wrong tag is caught rather than
# quietly measured.
DISTRO="${CLEANROOM_DISTRO:-debian}"
case $DISTRO in
  debian) expected_os_id=debian    ;;
  fedora) expected_os_id=fedora    ;;
  rhel)   expected_os_id=almalinux ;;
  arch)   expected_os_id=arch      ;;
  *) die "unknown CLEANROOM_DISTRO '$DISTRO' — one of: debian fedora rhel arch (try: distros)" ;;
esac

DOCKERFILE="$REPO_ROOT/etc/cleanroom/Dockerfile.$DISTRO"
[ -f "$DOCKERFILE" ] || die "no Dockerfile for distro '$DISTRO' at $DOCKERFILE"

IMAGE="${CLEANROOM_IMAGE:-posse-cleanroom-$DISTRO:1}"
NAME="${CLEANROOM_NAME:-posse-cleanroom-$DISTRO}"

# --platform IS ALWAYS PASSED, to build and to run alike. Two reasons, and the
# first is not hypothetical (ranger-base-1qm5, scripts/test-linux.sh): Docker's
# classic image store keys a tag as ONE local image, so a platform-specific
# build replaces whatever that tag pointed at, and every later DEFAULT run then
# silently emulates the override's architecture instead of the host's. Naming
# it every time makes the request explicit in both directions.
#
# The second is Arch. Official Arch is x86_64-only — `docker manifest inspect
# archlinux:latest` lists amd64 and nothing else — so that image is pinned to
# linux/amd64 whatever the host is, and on an Apple Silicon box it runs under
# qemu. Slow, and a permanent tax on this distro, not a one-off.
if [ -n "${CLEANROOM_PLATFORM:-}" ]; then
  PLATFORM=$CLEANROOM_PLATFORM
elif [ "$DISTRO" = arch ]; then
  PLATFORM=linux/amd64
else
  case "$(uname -m)" in
    arm64 | aarch64) PLATFORM=linux/arm64 ;;
    x86_64 | amd64)  PLATFORM=linux/amd64 ;;
    *) die "unknown host architecture '$(uname -m)' — set CLEANROOM_PLATFORM=linux/<arch>" ;;
  esac
fi

need_docker() {
  command -v docker >/dev/null 2>&1 || die "docker not found on PATH"
  # Do NOT tell the caller to start Docker Desktop: OPERATOR RULING 2026-08-30
  # (ranger-base-6mz7, ranger-base-uhcc) abandoned Docker on this box
  # permanently — the Virtualization.framework VM backing it held 3.8GB wired
  # on a 16GB box already swapping under six crew sessions. This instrument is
  # parked until an off-laptop cleanroom exists; see ranger-base-6mz7.
  docker info >/dev/null 2>&1 || die "docker daemon unavailable — Docker is abandoned on this box (operator ruling, ranger-base-6mz7); the clean room is parked until an off-laptop cleanroom exists, not runnable here"
}

image_exists()     { docker image inspect "$IMAGE" >/dev/null 2>&1; }
container_exists() { docker container inspect "$NAME" >/dev/null 2>&1; }
container_running() {
  [ "$(docker container inspect -f '{{.State.Running}}' "$NAME" 2>/dev/null)" = true ]
}

# Every command the test runs goes through a LOGIN shell, so PATH comes from
# /etc/profile + /etc/profile.d the way it does for a real user. Never bypass
# this with a bare `docker exec` — a non-login shell has a PATH no user has.
as_tester() { docker exec -i "$NAME" su - tester -c "$1"; }

cmd_distros() {
  cat <<'EOF'
cleanroom distros — select with CLEANROOM_DISTRO=<name>

  debian   debian:trixie-slim    DEFAULT. Every clean-room pass before
                                 ranger-base-5cj4 was run in this one.
  fedora   fedora:44             Fedora userland.
  rhel     almalinux:10          The RHEL family. This is the one that found
                                 ranger-base-rmgz: a minimal RHEL box has no
                                 `cmp`, and the generated prepare-commit-msg
                                 hook silently loses its revert-recovery
                                 paragraph without it.
  arch     archlinux:latest      Arch base. amd64 ONLY (official Arch has no
                                 arm64 image), so it runs under qemu on an
                                 Apple Silicon box.

                                 *** arch IS NOT omarchy. *** omarchy is Arch
                                 PLUS a curated desktop and dotfiles layer.
                                 This image covers the Arch base userland and
                                 covers none of that layer. Never call a pass
                                 in here omarchy coverage.

Not covered by ANY of them: the kernel (containers share the host's), an init
system, a desktop, and macOS — see ranger-base-hza for the macOS gap.
EOF
}

cmd_build() {
  need_docker
  echo "cleanroom: building $IMAGE from $(basename "$DOCKERFILE") ($PLATFORM, host $(uname -m))"
  # An `if`, not an `&&` chain: under `set -e` a standalone AND-list that
  # evaluates false IS a failing command and would abort the build here on
  # every non-arch distro.
  if [ "$DISTRO" = arch ] && [ "$(uname -m)" != x86_64 ]; then
    echo "cleanroom: arch is amd64-only — this build is EMULATED and will be slow"
  fi
  docker build --platform "$PLATFORM" -f "$DOCKERFILE" -t "$IMAGE" "$REPO_ROOT/etc/cleanroom"
  echo "cleanroom: built $IMAGE"
}

cmd_start() {
  need_docker
  image_exists || cmd_build
  if container_exists; then
    container_running && { echo "cleanroom: $NAME already running"; return 0; }
    docker rm -f "$NAME" >/dev/null
  fi
  docker run -d --platform "$PLATFORM" --name "$NAME" "$IMAGE" >/dev/null
  echo "cleanroom: $NAME started, pristine ($DISTRO, $PLATFORM)"
  echo "cleanroom: enter it with  CLEANROOM_DISTRO=$DISTRO scripts/cleanroom.sh shell"
}

cmd_shell() {
  need_docker
  container_running || cmd_start
  # -t as well as -i: this one is meant for a human/agent at a terminal.
  docker exec -it "$NAME" su - tester
}

cmd_run() {
  [ $# -ge 1 ] || die "run: needs a command, e.g. run 'go version'"
  need_docker
  container_running || cmd_start
  as_tester "$*"
}

cmd_reset() {
  need_docker
  image_exists || die "no image yet — run: CLEANROOM_DISTRO=$DISTRO scripts/cleanroom.sh build"
  container_exists && docker rm -f "$NAME" >/dev/null
  docker run -d --platform "$PLATFORM" --name "$NAME" "$IMAGE" >/dev/null
  echo "cleanroom: reset — $NAME is pristine again (image $IMAGE unchanged)"
}

cmd_destroy() {
  need_docker
  container_exists && docker rm -f "$NAME" >/dev/null
  echo "cleanroom: container removed (image $IMAGE kept; 'start' recreates)"
}

cmd_cp_in() {
  [ $# -ge 1 ] || die "cp-in: needs a source path on the host"
  need_docker
  container_running || die "not running — CLEANROOM_DISTRO=$DISTRO scripts/cleanroom.sh start"
  local src=$1 dst=${2:-.}
  case $dst in /*) ;; *) dst="$HOME_IN/$dst" ;; esac
  docker cp "$src" "$NAME:$dst"
  # docker cp lands files owned by root; hand them to tester so the test user
  # can actually read and edit what was copied in.
  docker exec -u 0 "$NAME" chown -R tester:tester "$dst" 2>/dev/null || true
  echo "cleanroom: copied $src -> $NAME:$dst"
}

cmd_cp_out() {
  [ $# -eq 2 ] || die "cp-out: needs <path-in-container> <path-on-host>"
  need_docker
  container_running || die "not running — CLEANROOM_DISTRO=$DISTRO scripts/cleanroom.sh start"
  local src=$1 dst=$2
  case $src in /*) ;; *) src="$HOME_IN/$src" ;; esac
  docker cp "$NAME:$src" "$dst"
  echo "cleanroom: copied $NAME:$src -> $dst"
}

cmd_status() {
  need_docker
  echo "distro    : $DISTRO (expect os-release ID=$expected_os_id)"
  echo "dockerfile: ${DOCKERFILE#"$REPO_ROOT"/}"
  echo "platform  : $PLATFORM"
  image_exists && echo "image     : $IMAGE (present)" || echo "image     : $IMAGE (NOT BUILT)"
  if container_running; then
    echo "container : $NAME running"
  elif container_exists; then
    echo "container : $NAME exists but is stopped"
  else
    echo "container : $NAME does not exist"
  fi
}

# ---------------------------------------------------------------------------
# hook-deps — what the generated hooks need, and what this distro has
# ---------------------------------------------------------------------------
# This is the probe that pays for the whole multi-distro route. The hooks posse
# renders (internal/posse/gates.go) are SHELL, and shell is where distro variance
# is visible — the Go suite cannot see it at all. ranger-base-rmgz is the
# finding: `cmp` ships in diffutils, a minimal RHEL box does not have it, and
# without it the prepare-commit-msg wall still refuses but silently drops the
# paragraph telling a user mid-`git revert` how to get out.
#
# THE LIST IS A CONTRACT AND IT CAN DRIFT, so it is no longer maintained by
# hand. It was hand-enumerated by READING internal/posse/gates.go for
# ranger-base-rmgz on 2026-08-28, and by 2026-09-01 it had drifted in both
# directions at once (ranger-base-lxkdi): `cut` and `sed` were called and never
# probed, and six names the hooks never call sat here looking probed. Reading
# the Go source is what drifted — the rendered bytes are what runs on the box.
#
# TestHookDepsNamesEveryCommandTheRenderedHooksCall
# (internal/posse/hookdeps_qa_test.go) now renders the three generated hooks,
# scans the shell text for command words, and fails when this line and that
# scan disagree in EITHER direction. Edit the list only in answer to that test;
# override for a one-off with HOOK_DEPS='a b c'.
#
# `cmp` dropped from this list in the ranger-base-rmgz fix itself: gates.go no
# longer calls it (the MERGE_MSG comparison is now POSIX shell — command
# substitution, not diffutils), so it is no longer a dependency to probe for.
#
# The list names what the hooks resolve THROUGH PATH, and nothing else.
#  - `git` is absent on purpose: git is what runs the hooks.
#  - Shell builtins are absent too, `printf` and `echo` included. `command -v`
#    is still the probe, and it answers a builtin as present — which is the
#    point: a builtin can never come back MISSING, so naming one here is an
#    assertion that cannot fail. This list exists to produce findings.
#  - `date` is here even though the hooks never spell it as a bare command:
#    posse_stamp walks PATH itself and runs the first non-gates `date` it
#    finds (ranger-base-l97n), which `command -v date` answers faithfully.
#  - `dirname` is the chain dispatcher's, not the two walls' — the third
#    generated hook, written when another tool already owns the slot.
HOOK_DEPS="${HOOK_DEPS:-cat cut date dirname grep head sed sort tr}"

cmd_hook_deps() {
  need_docker
  container_running || cmd_start
  echo "cleanroom: commands the generated hooks call, on $DISTRO ($IMAGE)"
  echo "cleanroom: source of the list — the RENDERED hooks, pinned by TestHookDepsNamesEveryCommandTheRenderedHooksCall"
  echo
  local out
  out=$(as_tester "for c in $HOOK_DEPS; do
           if command -v \"\$c\" >/dev/null 2>&1; then printf '  ok      %s\n' \"\$c\";
           else printf '  MISSING %s\n' \"\$c\"; fi; done") || die "hook-deps: probe failed in $NAME"
  printf '%s\n' "$out"
  echo
  if printf '%s\n' "$out" | grep -q '^  MISSING '; then
    printf 'cleanroom: %s is MISSING a command the hooks call — this is a FINDING, not a setup step.\n' "$DISTRO" >&2
    echo "cleanroom: do NOT install it here. File it (see ranger-base-rmgz for the shape)." >&2
    return 1
  fi
  echo "cleanroom: every command the hooks call is present on $DISTRO"
}

# ---------------------------------------------------------------------------
# verify — assert the guarantees ranger-base-5zh requires. Run before a pass.
# Nothing in here warms the module cache; the egress probe hits the public
# proxy over HTTP and writes nothing, so the fetch under test stays real.
# ---------------------------------------------------------------------------
fails=0
check() { # check <description> <shell-command-in-container-that-must-succeed>
  if as_tester "$2" >/dev/null 2>&1; then
    printf '  ok    %s\n' "$1"
  else
    printf '  FAIL  %s\n' "$1"; fails=$((fails + 1))
  fi
}

# The assertions below are single-quoted ON PURPOSE: they must expand inside the
# container, not on this host. Quoting them "properly" would test the dev box.
# shellcheck disable=SC2016,SC2088
cmd_verify() {
  need_docker
  container_running || cmd_start
  echo "cleanroom: verifying guarantees in $NAME ($DISTRO, $PLATFORM)"
  echo
  echo "it must be the distro it claims to be:"
  # Without this, a stale image or a hand-set CLEANROOM_IMAGE can have every
  # other check pass while measuring a distro nobody asked for.
  check "os-release ID is $expected_os_id" \
        ". /etc/os-release; [ \"\$ID\" = $expected_os_id ]"
  echo
  echo "the P1 under test must stay visible:"
  check 'PATH does NOT contain ~/go/bin'      'case ":$PATH:" in *":$HOME/go/bin:"*) exit 1;; esac'
  # /usr/local/go/bin is the toolchain itself and is expected; ANY other
  # PATH element ending in /go/bin would be a GOPATH bin dir and would hide the P1.
  check 'no GOPATH-style bin dir on PATH'     'IFS=:; for p in $PATH; do case "$p" in */go/bin) [ "$p" = /usr/local/go/bin ] || exit 1;; esac; done'
  check 'GOBIN is unset'                      '[ -z "${GOBIN:-}" ]'
  check 'GOPATH is unset in the environment'  '[ -z "${GOPATH:-}" ]'
  check 'go install target is not on PATH'    'd=$(go env GOBIN); [ -n "$d" ] || d=$(go env GOPATH)/bin; case ":$PATH:" in *":$d:"*) exit 1;; esac'
  echo
  echo "nothing from this project or this dev box:"
  check 'herdr absent'                        '! command -v herdr'
  check 'bd absent'                           '! command -v bd'
  check 'posse absent'                        '! command -v posse'
  check 'rhq absent'                          '! command -v rhq'
  check 'RHQ_HOME unset'                      '[ -z "${RHQ_HOME:-}" ]'
  check '~/.config/rhq absent'                '[ ! -e "$HOME/.config/rhq" ]'
  check 'no posse checkout in home'           '[ ! -e "$HOME/posse" ]'
  check 'Go module cache cold'                '[ ! -d "$(go env GOMODCACHE)" ] || [ -z "$(ls -A "$(go env GOMODCACHE)" 2>/dev/null)" ]'
  check 'no ~/go at all yet'                  '[ ! -e "$HOME/go" ]'
  echo
  echo "the toolchain and the public path:"
  check 'go on PATH via /usr/local/go/bin'    'command -v go | grep -q "^/usr/local/go/bin/go$"'
  check 'go >= 1.26'                          'go version'
  check 'GOPROXY is the public default'       'go env GOPROXY | grep -q "^https://proxy.golang.org"'
  check 'egress to proxy.golang.org'          'curl -fsS -o /dev/null https://proxy.golang.org/github.com/ranger360ai/posse/@v/list'
  check 'egress to github.com'                'curl -fsS -o /dev/null https://github.com/ranger360ai/posse'
  check 'running as an unprivileged user'     '[ "$(id -un)" = tester ] && [ "$(id -u)" = 1000 ]'
  echo
  if [ "$fails" -eq 0 ]; then
    echo "cleanroom: all guarantees hold — the $DISTRO clean room is honest"
  else
    echo "cleanroom: $fails GUARANTEE(S) FAILED — do not test in here until fixed" >&2
    return 1
  fi
}

case ${1:-} in
  build)     shift; cmd_build "$@" ;;
  start)     shift; cmd_start "$@" ;;
  shell)     shift; cmd_shell "$@" ;;
  run)       shift; cmd_run "$@" ;;
  reset)     shift; cmd_reset "$@" ;;
  verify)    shift; cmd_verify "$@" ;;
  hook-deps) shift; cmd_hook_deps "$@" ;;
  cp-in)     shift; cmd_cp_in "$@" ;;
  cp-out)    shift; cmd_cp_out "$@" ;;
  status)    shift; cmd_status "$@" ;;
  destroy)   shift; cmd_destroy "$@" ;;
  distros)   shift; cmd_distros "$@" ;;
  ""|-h|--help|help) awk 'NR>1 && /^#/ { sub(/^# ?/, ""); print; next } NR>1 { exit }' "$0" ;;
  *) die "unknown subcommand: $1 (try --help)" ;;
esac
