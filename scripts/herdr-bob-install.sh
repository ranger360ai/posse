#!/usr/bin/env bash
# Apply etc/herdr/herdr-bob/darwin-portability.patch to the INSTALLED herdr-bob
# checkout, and start its watcher (ranger-base-mz8ud).
#
# Why a patch against a managed checkout rather than a fork: `herdr plugin
# install` pins a commit and there is no `plugin update`, so nothing moves this
# tree behind our back -- only an explicit re-install at a new commit does, and
# `--check` says so when it happens. `scripts/verify-herdr-bob.sh` proves the
# patch against COPIES of this tree and never touches the live one; this script
# is the only thing that writes to it.
#
#   --check   (default) read-only: pin, patch state, watcher state
#   --apply   apply the patch; refuses unless the checkout is at the pinned
#             commit and clean of the patch already
#   --start   run the plugin's own `bob-watchctl start`, then `status`
#
# --start writes to the LIVE herdr server: from then on the watcher labels any
# unclaimed pane whose foreground process is Bob, every 3rd tick, for as long
# as it runs. That is the point of it, and it is also its blast radius -- a
# mis-claimed pane does not recover on its own (bob-watch's own comment: the
# label stays until `herdr server reload-agent-manifests`). Operator-gated.
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PATCH="$REPO/etc/herdr/herdr-bob/darwin-portability.patch"
PINNED_SHA=e53bd475dbe362c8a9195b582b88ef2c0857590b
ROOT="${HERDR_BOB_ROOT:-$HOME/.config/herdr/plugins/github/mloeper.herdr-bob-de923c8f068d}"

die() { echo "herdr-bob-install: $*" >&2; exit 1; }

[ -e "$PATCH" ] || die "no patch at $PATCH"
[ -d "$ROOT/bin" ] || die "herdr-bob is not installed at $ROOT"

sha="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
applied=no
git -C "$ROOT" apply --reverse --check "$PATCH" >/dev/null 2>&1 && applied=yes
appliable=no
git -C "$ROOT" apply --check "$PATCH" >/dev/null 2>&1 && appliable=yes

watcher_pid="$(cat "$HOME/.local/state/herdr/plugins/mloeper.herdr-bob/watch.pid" 2>/dev/null || echo)"
watcher=stopped
[ -n "$watcher_pid" ] && kill -0 "$watcher_pid" 2>/dev/null && watcher="running (pid $watcher_pid)"

report() {
  echo "root:     $ROOT"
  echo "commit:   $sha$([ "$sha" = "$PINNED_SHA" ] && echo "  (the pinned one)" || echo "  (PATCH WAS CUT AGAINST $PINNED_SHA)")"
  echo "patch:    applied=$applied appliable=$appliable"
  echo "watcher:  $watcher"
}

case "${1:---check}" in
  --check)
    report
    ;;
  --apply)
    [ "$applied" = no ] || { report; echo "nothing to do: already applied"; exit 0; }
    [ "$sha" = "$PINNED_SHA" ] ||
      die "checkout is at $sha, the patch was cut against $PINNED_SHA -- re-cut it before applying"
    [ "$appliable" = yes ] || die "the patch does not apply cleanly to $ROOT"
    git -C "$ROOT" apply "$PATCH" || die "git apply failed"
    echo "applied: $PATCH -> $ROOT"
    # The generated hook launcher bakes in a plugin root and a bash; the
    # plugin refreshes it on every startup, and so does `bob-watchctl start`.
    echo "next: $0 --start   (writes to the live herdr -- see the header)"
    ;;
  --start)
    [ "$applied" = yes ] || die "patch is not applied; run $0 --apply first"
    /bin/bash "$ROOT/bin/bob-watchctl" start || die "bob-watchctl start failed"
    /bin/bash "$ROOT/bin/bob-watchctl" status
    ;;
  *)
    echo "usage: $0 [--check|--apply|--start]" >&2
    exit 2
    ;;
esac
