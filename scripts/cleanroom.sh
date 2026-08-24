#!/usr/bin/env bash
# cleanroom.sh — drive the clean-room test environment (ranger-base-5zh).
#
# A throwaway Debian container with a DEFAULT PATH, a newcomer's Go toolchain,
# and nothing at all from this project or this dev box. It exists so the public
# install story can be tested on a machine that has never seen posse.
#
# The guarantee that matters most: ~/go/bin is NOT on PATH in there. That
# omission is the P1 under test (ranger-base-253). `cleanroom.sh verify`
# asserts it, along with every other guarantee, and is worth running before
# each test pass.
#
# Usage:
#   scripts/cleanroom.sh build          build (or rebuild) the image
#   scripts/cleanroom.sh start          start a fresh container from the image
#   scripts/cleanroom.sh shell          interactive login shell as `tester`
#   scripts/cleanroom.sh run '<cmd>'    run <cmd> in a login shell as `tester`
#   scripts/cleanroom.sh reset          destroy and recreate — back to pristine
#   scripts/cleanroom.sh verify         assert every clean-room guarantee
#   scripts/cleanroom.sh cp-in  <src> [dst]   host -> ~tester (dst default: .)
#   scripts/cleanroom.sh cp-out <src> <dst>   ~tester -> host
#   scripts/cleanroom.sh status         is it up, and what is it
#   scripts/cleanroom.sh destroy        remove the container (image is kept)
set -euo pipefail

IMAGE=posse-cleanroom:1
NAME=posse-cleanroom
HOME_IN=/home/tester
REPO_ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)

die() { printf 'cleanroom: %s\n' "$*" >&2; exit 1; }

need_docker() {
  command -v docker >/dev/null 2>&1 || die "docker not found on PATH"
  docker info >/dev/null 2>&1 || die "docker daemon is not running — start Docker Desktop"
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

cmd_build() {
  need_docker
  echo "cleanroom: building $IMAGE (arch: $(uname -m))"
  docker build -t "$IMAGE" "$REPO_ROOT/etc/cleanroom"
  echo "cleanroom: built $IMAGE"
}

cmd_start() {
  need_docker
  image_exists || cmd_build
  if container_exists; then
    container_running && { echo "cleanroom: $NAME already running"; return 0; }
    docker rm -f "$NAME" >/dev/null
  fi
  docker run -d --name "$NAME" "$IMAGE" >/dev/null
  echo "cleanroom: $NAME started, pristine"
  echo "cleanroom: enter it with  scripts/cleanroom.sh shell"
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
  image_exists || die "no image yet — run: scripts/cleanroom.sh build"
  container_exists && docker rm -f "$NAME" >/dev/null
  docker run -d --name "$NAME" "$IMAGE" >/dev/null
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
  container_running || die "not running — scripts/cleanroom.sh start"
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
  container_running || die "not running — scripts/cleanroom.sh start"
  local src=$1 dst=$2
  case $src in /*) ;; *) src="$HOME_IN/$src" ;; esac
  docker cp "$NAME:$src" "$dst"
  echo "cleanroom: copied $NAME:$src -> $dst"
}

cmd_status() {
  need_docker
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
  echo "cleanroom: verifying guarantees in $NAME"
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
    echo "cleanroom: all guarantees hold — the clean room is honest"
  else
    echo "cleanroom: $fails GUARANTEE(S) FAILED — do not test in here until fixed" >&2
    return 1
  fi
}

case ${1:-} in
  build)   shift; cmd_build "$@" ;;
  start)   shift; cmd_start "$@" ;;
  shell)   shift; cmd_shell "$@" ;;
  run)     shift; cmd_run "$@" ;;
  reset)   shift; cmd_reset "$@" ;;
  verify)  shift; cmd_verify "$@" ;;
  cp-in)   shift; cmd_cp_in "$@" ;;
  cp-out)  shift; cmd_cp_out "$@" ;;
  status)  shift; cmd_status "$@" ;;
  destroy) shift; cmd_destroy "$@" ;;
  ""|-h|--help|help) awk 'NR>1 && /^#/ { sub(/^# ?/, ""); print; next } NR>1 { exit }' "$0" ;;
  *) die "unknown subcommand: $1 (try --help)" ;;
esac
