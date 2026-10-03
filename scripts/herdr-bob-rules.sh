#!/usr/bin/env bash
# Installs posse's herdr-bob screen-rule override (ranger-base-0sa5a):
#   etc/herdr/herdr-bob/rules.json  ->  <plugin config dir>/rules.json
#
# This is the whole fix. bob-watch seeds the plugin's own rules.json into
# config_root() once, never refreshes it, and prefers it thereafter -- so the
# override needs NO write to the managed checkout and no patch to upstream's
# code. It is a different file and a different blast radius from
# scripts/herdr-bob-install.sh (ranger-base-mz8ud), which patches the checkout.
#
#   --check    read-only. Prints the plugin root, the config dir, what is
#              installed there and how it differs from upstream's and from
#              this repo's. Changes nothing.
#   --diff     the diff between what is installed and what this repo holds.
#   --install  writes the override, after scripts/verify-herdr-bob-rules.sh
#              passes. Keeps a timestamped backup and prints the restore
#              command. GATED: see ranger-base-knikn -- the herdr config tree
#              is the live server every dispatched seat runs in.
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OVERRIDE="$REPO/etc/herdr/herdr-bob/rules.json"
PLUGIN_ID=mloeper.herdr-bob
SRC="${HERDR_BOB_ROOT:-$HOME/.config/herdr/plugins/github/${PLUGIN_ID}-de923c8f068d}"
# config_root() in the plugin's lib/common.sh, which is HERDR_PLUGIN_CONFIG_DIR
# as herdr sets it -- `herdr plugin list` reports it, so ask rather than guess.
CFG="${HERDR_PLUGIN_CONFIG_DIR:-}"
if [ -z "$CFG" ]; then
  CFG="$(herdr plugin list 2>/dev/null \
         | awk -v id="$PLUGIN_ID" '$0 ~ "^- " id " " {f=1; next} f && /config:/ {print $2; exit}')"
fi
[ -n "$CFG" ] || CFG="$HOME/.config/herdr/plugins/config/$PLUGIN_ID"
LIVE="$CFG/rules.json"

die() { echo "herdr-bob-rules: $*" >&2; exit 1; }
[ -e "$OVERRIDE" ] || die "no override at $OVERRIDE"

# The rule ids in a file, newline separated.
ids() { jq -r '.rules[].id' "$1" 2>/dev/null; }

report() {
  echo "plugin root:  $SRC"
  echo "config dir:   $CFG"
  echo "override:     $OVERRIDE  ($(ids "$OVERRIDE" | tr '\n' ' '))"
  if [ ! -e "$LIVE" ]; then
    echo "installed:    NOTHING -- bob-watch will seed upstream's copy on its first run"
    echo "state:        not installed"
    return
  fi
  echo "installed:    $LIVE  ($(ids "$LIVE" | tr '\n' ' '))"
  if cmp -s "$LIVE" "$OVERRIDE"; then
    echo "state:        INSTALLED (byte-identical to this repo's override)"
  elif cmp -s "$LIVE" "$SRC/rules.json"; then
    echo "state:        not installed -- what is there is upstream's copy, unmodified"
  else
    # The case this script exists to make visible: a hand-edit. On this box the
    # seeded copy was edited by hand on 2026-09-28, five minutes after the
    # install, adding a rule that appears in no commit, no plugin file and no
    # bead (MEASURED 2026-10-03, ranger-base-0sa5a). An unversioned change is
    # not a fix; it is a thing that works until someone wonders why.
    echo "state:        HAND-EDITED -- matches neither upstream's copy nor this repo's override"
    local extra
    extra="$(comm -23 <(ids "$LIVE" | sort) <(ids "$SRC/rules.json" | sort) | tr '\n' ' ')"
    [ -n "$extra" ] && echo "              rule ids upstream does not ship: $extra"
    echo "              mtime $(stat -f '%Sm' "$LIVE" 2>/dev/null)"
  fi
}

case "${1:---check}" in
  --check)
    report
    ;;
  --diff)
    [ -e "$LIVE" ] || die "nothing installed at $LIVE"
    diff -u "$LIVE" "$OVERRIDE"
    ;;
  --install)
    command -v jq >/dev/null 2>&1 || die "jq is not on PATH"
    jq -e . "$OVERRIDE" >/dev/null 2>&1 || die "the override is not valid JSON"
    # An unproved rules file is not installable. A false `blocked` stops every
    # wait on the pane, so the corpus replay is a precondition, not a courtesy.
    echo "verifying the override before installing it:"
    "$REPO/scripts/verify-herdr-bob-rules.sh" || die "verify-herdr-bob-rules failed -- not installing"
    mkdir -p "$CFG" || die "cannot create $CFG"
    if [ -e "$LIVE" ]; then
      if cmp -s "$LIVE" "$OVERRIDE"; then
        echo "already installed and identical; nothing to do"
        exit 0
      fi
      bak="$LIVE.bak.$(date -u +%Y%m%dT%H%M%SZ)"
      cp "$LIVE" "$bak" || die "cannot back up $LIVE"
      echo "backed up:    $bak"
      echo "restore with: cp '$bak' '$LIVE'"
    fi
    cp "$OVERRIDE" "$LIVE" || die "cannot write $LIVE"
    echo "installed:    $LIVE"
    echo
    report
    echo
    echo "NOTE: bob-watch re-reads the rules file on every tick, so a running"
    echo "      watcher picks this up within one poll interval. On this box the"
    echo "      watcher cannot run at all until ranger-base-mz8ud's patch is"
    echo "      applied (ranger-base-knikn), so until then this is inert."
    ;;
  -h|--help)
    sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
    ;;
  *)
    die "unknown argument '$1' -- try --check, --diff, --install or --help"
    ;;
esac
