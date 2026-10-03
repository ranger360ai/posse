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
#   --check   (default) read-only: pin, patch state, the roots, watcher state
#   --apply   apply the patch; refuses unless the checkout is at the pinned
#             commit and clean of the patch already
#   --start   run the plugin's own `bob-watchctl start` under the SAME
#             environment herdr's [[startup]] hook gives it, then `status`
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
PLUGIN_ID=mloeper.herdr-bob
ROOT="${HERDR_BOB_ROOT:-$HOME/.config/herdr/plugins/github/${PLUGIN_ID}-de923c8f068d}"

die() { echo "herdr-bob-install: $*" >&2; exit 1; }

[ -e "$PATCH" ] || die "no patch at $PATCH"
[ -d "$ROOT/bin" ] || die "herdr-bob is not installed at $ROOT"

# ---------------------------------------------------------------- the roots
#
# The plugin's lib/common.sh resolves its state and config directories from
# HERDR_PLUGIN_STATE_DIR and HERDR_PLUGIN_CONFIG_DIR, which herdr sets when it
# runs a plugin command -- and falls back to ~/.local/state/herdr-bob and
# ~/.config/herdr-bob when neither is set. So a `--start` that does not export
# them starts a watcher on the FALLBACK roots: a different pidfile, a different
# log, and upstream's unproved rules.json rather than the override
# scripts/herdr-bob-rules.sh installs (ranger-base-0sa5a proved that file
# false-positive on Bob's `/` picker). MEASURED 2026-10-03, the operator's
# finding on ranger-base-knikn: the first --start on this box did exactly that,
# and because the plugin's own `stop` reads the pidfile under whichever root it
# is given, the wrong-root watcher was invisible to a stop run under the right
# one -- two watchers, one of them reading rules nobody proved.
#
# Hence: resolve the three, export them, and PRINT them. The printing is not
# decoration. This whole bead is one failure that reported nothing, and a root
# chosen silently is the same shape.

# Ask the binary; `herdr plugin config-dir` is the verb for it, and
# `herdr plugin list` reports the same path -- which is what
# scripts/herdr-bob-rules.sh reads, so the two scripts cannot disagree.
resolve_config_dir() {
  local c="${HERDR_PLUGIN_CONFIG_DIR:-}"
  [ -n "$c" ] || c="$(herdr plugin config-dir "$PLUGIN_ID" 2>/dev/null)"
  [ -n "$c" ] || c="$(herdr plugin list 2>/dev/null \
      | awk -v id="$PLUGIN_ID" '$0 ~ "^- " id " " {f=1; next} f && /config:/ {print $2; exit}')"
  [ -n "$c" ] || c="${XDG_CONFIG_HOME:-$HOME/.config}/herdr/plugins/config/$PLUGIN_ID"
  printf '%s' "$c"
}

# No herdr verb prints this one, so it is derived rather than asked.
# ASSUMED 2026-10-03, and corroborated two ways: every plugin on this box has
# its state under <state>/herdr/plugins/<plugin-id> (four of them), and the
# watcher herdr's own hook started wrote its pidfile and log exactly there.
resolve_state_dir() {
  local s="${HERDR_PLUGIN_STATE_DIR:-}"
  [ -n "$s" ] || s="${XDG_STATE_HOME:-$HOME/.local/state}/herdr/plugins/$PLUGIN_ID"
  printf '%s' "$s"
}

CFG="$(resolve_config_dir)"
STATE="$(resolve_state_dir)"
# lib/common.sh's herdr_bin() defaults to `herdr` off PATH; the hook passes an
# absolute path, and so do we, because the watcher outlives this shell.
BIN="${HERDR_BIN_PATH:-$(command -v herdr 2>/dev/null || echo herdr)}"

# The roots a --start with no environment would have used: the fallbacks in
# lib/common.sh. Reported so a watcher left over there is visible from here.
FALLBACK_STATE="${XDG_STATE_HOME:-$HOME/.local/state}/herdr-bob"
FALLBACK_CFG="${XDG_CONFIG_HOME:-$HOME/.config}/herdr-bob"

# Prints `running (pid N)` or `stopped` for the watcher under one state root.
watcher_in() { # state-dir
  local pid; pid="$(cat "$1/watch.pid" 2>/dev/null || echo)"
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then echo "running (pid $pid)"
  elif [ -n "$pid" ]; then echo "stopped (stale pidfile: $pid)"
  else echo "stopped"
  fi
}

sha="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
applied=no
git -C "$ROOT" apply --reverse --check "$PATCH" >/dev/null 2>&1 && applied=yes
appliable=no
git -C "$ROOT" apply --check "$PATCH" >/dev/null 2>&1 && appliable=yes

report() {
  echo "root:     $ROOT"
  echo "commit:   $sha$([ "$sha" = "$PINNED_SHA" ] && echo "  (the pinned one)" || echo "  (PATCH WAS CUT AGAINST $PINNED_SHA)")"
  echo "patch:    applied=$applied appliable=$appliable"
  echo "state:    $STATE"
  echo "config:   $CFG"
  echo "herdr:    $BIN"
  echo "watcher:  $(watcher_in "$STATE")"
  local f; f="$(watcher_in "$FALLBACK_STATE")"
  case "$f" in
    running*) echo "WARNING:  a watcher is also $f under the FALLBACK root $FALLBACK_STATE" >&2
              echo "          that one reads $FALLBACK_CFG/rules.json, not the installed override," >&2
              echo "          and a stop run under the plugin roots cannot see it. Stop it with:" >&2
              echo "            HERDR_PLUGIN_STATE_DIR=$FALLBACK_STATE /bin/bash $ROOT/bin/bob-watchctl stop" >&2 ;;
    *) : ;;
  esac
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
    case "$(watcher_in "$FALLBACK_STATE")" in
      running*)
        report
        echo "herdr-bob-install: refusing to start -- a watcher is already running under the" >&2
        echo "  fallback root. Stop that one first (the command is in the warning above);" >&2
        echo "  starting now would leave two watchers reporting into the same server, and" >&2
        echo "  the plugin's own stop cannot see across the two roots." >&2
        exit 1 ;;
    esac
    echo "starting under the roots herdr's [[startup]] hook uses:"
    echo "  HERDR_PLUGIN_STATE_DIR=$STATE"
    echo "  HERDR_PLUGIN_CONFIG_DIR=$CFG"
    echo "  HERDR_BIN_PATH=$BIN"
    HERDR_PLUGIN_STATE_DIR="$STATE" \
    HERDR_PLUGIN_CONFIG_DIR="$CFG" \
    HERDR_BIN_PATH="$BIN" \
      /bin/bash "$ROOT/bin/bob-watchctl" start || die "bob-watchctl start failed"
    HERDR_PLUGIN_STATE_DIR="$STATE" \
    HERDR_PLUGIN_CONFIG_DIR="$CFG" \
    HERDR_BIN_PATH="$BIN" \
      /bin/bash "$ROOT/bin/bob-watchctl" status
    ;;
  *)
    echo "usage: $0 [--check|--apply|--start]" >&2
    exit 2
    ;;
esac
