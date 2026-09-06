#!/usr/bin/env bash
# Verify the fleet's codex version pin (ranger-base-poj5) against the live
# install, and report when upstream has moved past it.
#
# THIS SCRIPT HAS A DIFFERENT SHAPE FROM verify-grok-pin.sh, on purpose.
# grok is pinned by two config keys — a soft ceiling the updater obeys and a
# hard one grok refuses to start above. codex has neither: measured on the
# 0.150.1 binary, `required_maximum_version`, `maximum_version`,
# `minimum_version` and `auto_update` all appear ZERO times, against a
# positive control (`model_reasoning_effort`, 27) proving the read reaches
# codex's config-key strings at all. So there is no hard ceiling to assert and
# no version of etc/codex/version-pin.toml could add one. The pin is:
#
#   1. `brew pin --cask codex` — refuses to UPGRADE, never to START. This is
#      maximum_version's shape. It also covers codex's own in-TUI updater,
#      because on a brew install codex's update action IS `brew upgrade --cask
#      codex` (codex says so: `codex doctor --json`, updates.status details),
#      and brew exits 1 on a pinned cask.
#   2. `check_for_update_on_startup = false` in the operator's config — the
#      startup menu whose default-selected option is "1. Update now" is never
#      drawn. Permanent, where `dismissed_version` silences one release.
#   3. A rollback target that still exists. A cask keeps exactly one version
#      and `brew cleanup` takes the old one; grok's ~/.grok/downloads/ has no
#      equivalent here, so the Caskroom directory is asserted rather than
#      assumed.
#
# ACCEPTED RISK, stated every run: a codex above the pin still STARTS. Nothing
# available on this runtime refuses to run an un-re-audited build.
#
# Exit 2 = nothing measured (codex or brew absent, pin or config unreadable).
# Exit 1 = a check failed. Exit 0 = the pin is intact.
set -uo pipefail

cd "$(dirname "$0")/.."
pin=etc/codex/version-pin.toml
cfg=${CODEX_HOME:-$HOME/.codex}/config.toml
fail=0

command -v codex >/dev/null || { echo "verify-codex-pin: codex not on PATH"; exit 2; }
command -v brew  >/dev/null || { echo "verify-codex-pin: brew not on PATH"; exit 2; }
[ -r "$pin" ] || { echo "verify-codex-pin: missing $pin"; exit 2; }
[ -r "$cfg" ] || { echo "verify-codex-pin: missing $cfg"; exit 2; }

# Every extractor below decides an arm, so none of them forks a matcher
# (ranger-base-s8b4g, ranger-base-7hx87): a `sed`/`awk`/`tr`/`head`/`sort`
# that is signalled, that cannot be exec'd under load, or that takes EPIPE
# returns nothing, and nothing here is indistinguishable from "the key is not
# in the file" or "the tool answered nothing" — which reads as offline,
# prints "pin intact" and exits 0. The fork that IS the measurement stays;
# only the readers of its output changed.
#
# val <key> <file> — the value on the first line of <file> that starts with
# <key>, optional spaces, `=`, optional spaces and an optional opening quote,
# cut at the first `"` or `#` and with every space removed. That is exactly
# what `sed -n | head -1 | tr -d ' '` did, anchored at ^ for the same reason:
# every explanatory line in the pin file and in the operator's config starts
# with '#', so a commented example of a key can never be read as the key.
val() {
  local line rest v
  [ -r "$2" ] || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    rest=${line#"$1"}
    [ "$rest" = "$line" ] && continue
    while [ "${rest# }" != "$rest" ]; do rest=${rest# }; done
    case $rest in '='*) ;; *) continue ;; esac
    rest=${rest#=}
    while [ "${rest# }" != "$rest" ]; do rest=${rest# }; done
    rest=${rest#\"}
    v=${rest%%[\"#]*}
    printf '%s' "${v// /}"
    return 0
  done <"$2"
}

# field2 <text> — the second whitespace-separated field of the FIRST line,
# replacing `| awk '{print $2}'`. Deliberately stricter than the awk it
# replaces, which printed field 2 of EVERY line: the version answer is one
# line, and a second line is a surprise the row should show rather than
# silently concatenate.
field2() {
  local s=${1%%$'\n'*} rest
  s=${s#"${s%%[![:space:]]*}"}
  rest=${s#*[[:space:]]}
  [ "$rest" = "$s" ] && return 0
  rest=${rest#"${rest%%[![:space:]]*}"}
  printf '%s' "${rest%%[[:space:]]*}"
}

# lastver <text> — the LAST whitespace-separated token of the first line
# that starts with a digit, and nothing at all if the line carries no such
# token. This reads the tap version out of `brew info --cask`, whose header
# comes in three shapes, all measured on Homebrew 6.0.22:
#
#   installed == tap     ==> codex (Codex): 0.153.4
#   installed <  tap     ==> tigervnc (TigerVNC): 1.15.0 → 1.16.2
#   auto_updates cask    ==> thaw (Thaw): 1.2.0 → 2.0.1 (auto_updates)
#
# The TAP version is the last one on the line in all three, so reading it as
# "last digit-initial token" is blind both to how the arrow is encoded and to
# any trailing parenthesised flag. A header with no version-shaped token
# (a `version :latest` cask) yields nothing and the caller fails the row.
# The split is written out rather than `for tok in $s`, whose word splitting
# also globs.
lastver() {
  local s=${1%%$'\n'*} tok out=
  while [ -n "$s" ]; do
    s=${s#"${s%%[![:space:]]*}"}
    [ -n "$s" ] || break
    tok=${s%%[[:space:]]*}
    s=${s#"$tok"}
    case $tok in [0-9]*) out=$tok ;; esac
  done
  printf '%s' "$out"
}

want_ver=$(val posse_pinned_version "$pin")
want_cask=$(val formula "$pin")
want_pin=$(val pin_state "$pin")
want_cfuos=$(val check_for_update_on_startup "$pin")
want_room=$(val caskroom_dir "$pin")

# The REACHED binary, not the one a path says should be there: codex is also
# installable by npm, bun, pnpm and a standalone installer, each its own
# update channel, and any of them linked ahead of the cask leaves the pin
# asserting a binary nothing runs.
live_ver=$(field2 "$(codex --version 2>/dev/null)")
cfg_cfuos=$(val check_for_update_on_startup "$cfg")

pinned=unpinned
# `grep -qx` without the fork (ranger-base-s8b4g): a dying grep would report
# the cask unpinned — a FAIL row against a box where the pin is holding.
pinned_list=$(brew list --pinned 2>/dev/null)
while IFS= read -r line || [ -n "$line" ]; do
  if [ "$line" = "$want_cask" ]; then
    pinned=pinned
    break
  fi
done <<<"$pinned_list"

chk_row() { # label want got
  if [ "$2" = "$3" ]; then printf '  %-30s %-12s ok\n' "$1" "$3"
  else printf '  %-30s %-12s <-- FAIL (want %s)\n' "$1" "${3:-?}" "$2"; fail=$((fail + 1)); fi
}

echo "codex version pin — $pin"
chk_row "codex --version"          "$want_ver"    "$live_ver"
chk_row "brew cask pin"            "$want_pin"    "$pinned"
chk_row "config check_for_update"  "$want_cfuos"  "$cfg_cfuos"

# The rollback target, and the identity of the running binary in one reading.
# `command -v` names a path; only resolving it says which tree answers. A
# Caskroom directory that is present but is NOT what codex runs from is a
# rollback target for a binary nobody uses, so both halves are one row each.
room="$(brew --prefix 2>/dev/null)/$want_room"
# Named rather than re-tested: the UPSTREAM MOVED block below tells an
# operator whether the rollback artifact is still fetchable, and asking the
# disk a second time could answer differently from the row above it.
room_state=gone
if [ -d "$room" ]; then
  room_state=present
  # Both sides get resolved before they are compared. `readlink -f` follows
  # the cask's symlink to a REAL path, so an unresolved prefix on the other
  # side fails a box that is in fact correct — /var vs /private/var is enough,
  # and any Homebrew prefix reached through a symlink is the same bug.
  # `cd && pwd -P` is the portable resolver; there is no `realpath` on stock
  # macOS.
  room=$(cd "$room" && pwd -P)
  printf '  %-30s %-12s ok\n' "rollback target on disk" "present"
  real=$(readlink -f "$(command -v codex)" 2>/dev/null || command -v codex)
  realdir=$(cd "$(dirname "$real")" 2>/dev/null && pwd -P)
  case "$realdir/" in
    "$room"/*) printf '  %-30s %-12s ok\n' "codex resolves into the pin" "yes" ;;
    *)         printf '  %-30s %-12s <-- FAIL (%s)\n' "codex resolves into the pin" "no" "${real:-?}"; fail=$((fail + 1)) ;;
  esac
else
  printf '  %-30s %-12s <-- FAIL (%s)\n' "rollback target on disk" "GONE" "$room"; fail=$((fail + 1))
  # Not measured, and said so rather than counted: "does codex resolve into a
  # directory that does not exist" has no answer, and a second red row for one
  # broken thing is how a reader learns to skim the list.
  printf '  %-30s %-12s (rollback target gone; nothing to resolve into)\n' "codex resolves into the pin" "—"
fi

# Upstream, read from the TAP and not from what happens to be installed.
# This used to read `brew outdated --cask --verbose`, which is SILENT about a
# cask whose installed version is not BEHIND the tap — including the case this
# row exists to catch. On the box of ranger-base-k4lza the cask had already
# been upgraded past the pin, so installed == tap == 0.153.4, `brew outdated`
# named no codex, and the old `[ -n "$upstream" ] || upstream=$want_ver`
# fallback substituted the pin itself: the run printed "tap version 0.150.1"
# and "== the pin; nothing to re-audit" over a tap three minor versions past
# it, and suppressed the whole re-audit list. Same shape as the forked
# matchers of ranger-base-s8b4g — a reader that answered nothing was
# indistinguishable from "nothing to re-audit" — with a fallback in place of
# the fork.
#
# `brew info --cask` names the tap version whatever is installed, so there is
# nothing left to fall back TO, and both ways of not getting an answer fail
# the row: a non-zero brew, and a header with no version in it. Unlike the
# per-package `brew outdated --cask codex` form (which exits 1 when that cask
# IS outdated), `brew info --cask codex` exits 0 on success and 1 on an
# unknown cask, so this one requires 0.
info=$(brew info --cask "$want_cask" 2>/dev/null); brc=$?
upstream=""
[ "$brc" -eq 0 ] && upstream=$(lastver "$info")
if [ -n "$upstream" ]; then
  printf '  %-30s %-12s read\n' "tap version" "$upstream"
elif [ "$brc" -ne 0 ]; then
  printf '  %-30s %-12s <-- FAIL (brew info --cask %s exited %s)\n' "tap version" "?" "$want_cask" "$brc"
  fail=$((fail + 1))
else
  printf '  %-30s %-12s <-- FAIL (no version in: %s)\n' "tap version" "?" "${info%%$'\n'*}"
  fail=$((fail + 1))
fi

# ver_gt <a> <b> — true when <a> sorts after <b>, replacing
# `sort -t. -k1,1n -k2,2n -k3,3n | tail -1`. Three dotted fields compared as
# numbers, each read the way `sort -n` reads one (leading digits; missing or
# non-numeric is 0), and the whole string as the last resort where all three
# tie — which is `sort`'s own last-resort comparison, and the only reason
# `ver_gt 1.0.0 1.0` is true. Measured against the pipeline it replaces over
# 12 pairs (ranger-base-s8b4g).
ver_gt() {
  local a=$1 b=$2 i ax bx
  [ "$a" = "$b" ] && return 1
  for i in 1 2 3; do
    ax=${a%%.*}
    bx=${b%%.*}
    ax=${ax%%[!0-9]*}
    bx=${bx%%[!0-9]*}
    [ -n "$ax" ] || ax=0
    [ -n "$bx" ] || bx=0
    [ "$ax" -gt "$bx" ] && return 0
    [ "$ax" -lt "$bx" ] && return 1
    case $a in *.*) a=${a#*.} ;; *) a= ;; esac
    case $b in *.*) b=${b#*.} ;; *) b= ;; esac
  done
  [[ $1 > $2 ]] && return 0
  return 1
}

echo
echo "ACCEPTED RISK (ranger-base-poj5): codex has no required_maximum_version"
echo "equivalent. Everything above refuses to MOVE the binary; nothing refuses"
echo "to RUN one that got past the pin by another route."

echo
# The tap moving and the BOX moving are two different questions, and the
# paragraph under this heading is true of only one of them. Read from the
# tap alone, this block used to tell every box "the pin is holding — nothing
# has changed on this machine" and to hurry it into fetching a rollback
# artifact before $upstream "lands" — on a box where $upstream had already
# landed, the rows above said so twice, and `brew cleanup` had already taken
# the rollback target. ranger-base-k4lza unsuppressed this block correctly
# and left the prose underneath it unread (ranger-base-9ycqa finding 1).
#
# So the gate stays the tap's and the TEXT branches on live_ver: the pin
# holds only where the binary is still $want_ver. Everything from item 2 on
# is the re-audit list, which is the same work either way.
if [ -n "$upstream" ] && ver_gt "$upstream" "$want_ver"; then
  cat <<EOF
UPSTREAM MOVED: the codex cask is $upstream; the fleet is pinned at $want_ver.
EOF
  if [ "$live_ver" = "$want_ver" ]; then
    cat <<EOF

The pin is holding — nothing has changed on this machine. Lifting it is the
operator's call, and each item below was verified against $want_ver only:

  1. FETCH THE ROLLBACK ARTIFACT FIRST. A cask keeps one version. The moment
     $upstream lands, \`brew cleanup\` deletes the only $want_ver copy on this
     box and homebrew-cask carries no version history. The URL and sha256 are
     in $pin, [rollback].
EOF
  else
    cat <<EOF

THE PIN IS NOT HOLDING: this box runs ${live_ver:-an unreadable codex}, not $want_ver.

The failing rows above are that measurement, and this is no longer a question
of whether to LIFT the pin: the box is already past it. The choice now is to
roll back to $want_ver, or to re-audit what is installed and move the pin.
Items 2-4 are that re-audit, and none of it has been done for any version
but $want_ver.

EOF
    if [ "$room_state" = present ]; then
      cat <<EOF
  1. THE ROLLBACK ARTIFACT IS STILL HERE, and it is the last copy. $want_ver
     is on disk and the next \`brew cleanup\` takes it, so copy it off the box
     now if a rollback is on the table. The URL and sha256 in $pin,
     [rollback], are the only other way back.
     $room
EOF
    else
      cat <<EOF
  1. THE ROLLBACK ARTIFACT IS ALREADY GONE. \`brew cleanup\` has taken the
     only $want_ver copy on this box and homebrew-cask carries no version
     history, so a rollback means re-fetching it and verifying that sha
     before it is run. The URL and sha256 are in $pin, [rollback].
EOF
    fi
  fi
  cat <<EOF
  2. The dispatch contract (ADR 0013 §1, NOTES.md "Personas"). Every flag on
     the codex launch line is version-verified, not contractual: \`-a never\`,
     \`--disable hooks\`, \`-c allow_login_shell=false\`, the \`projects\`
     trust grant, and \`-c developer_instructions="\$(cat ...)"\` — which is
     how the work prompt is delivered at all. \`posse runtime check codex\`.
  3. The startup-update key itself. \`check_for_update_on_startup\` exists at
     $want_ver; a rename retires the affordance kill silently. Re-run the
     four-arm rig (key absent / true / false / an unrelated key) against a
     CODEX_HOME whose version.json is due a menu — the unrelated-key arm is
     what separates "this key works" from "any key works".
  4. Interstitial detection (etc/herdr/agent-detection/codex.toml and its
     testdata: update_menu, model_picker, hooks_review, idle_composer).
     \`make verify-detection\` against the new build's screens.

  Runbook: docs/notes.d/ranger-base-poj5.md.
EOF
elif [ -n "$upstream" ]; then
  echo "tap: $upstream (== the pin; nothing to re-audit)"
fi

echo
if [ "$fail" -ne 0 ]; then
  echo "verify-codex-pin: $fail check(s) FAILED — the pin is not what $pin says"
  exit 1
fi
echo "verify-codex-pin: pin intact at $want_ver"
