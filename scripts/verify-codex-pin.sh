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

# Same extractor as verify-grok-pin.sh, and it is anchored at ^ for the same
# reason: every explanatory line in both the pin file and the operator's
# config starts with '#', so a commented example of a key can never be read
# as the key. POSIX BRE only — BSD sed has no \|.
val() { sed -n "s/^$1 *= *\"\{0,1\}\([^\"#]*\)\"\{0,1\}.*/\1/p" "$2" | head -1 | tr -d ' '; }

want_ver=$(val posse_pinned_version "$pin")
want_cask=$(val formula "$pin")
want_pin=$(val pin_state "$pin")
want_cfuos=$(val check_for_update_on_startup "$pin")
want_room=$(val caskroom_dir "$pin")

# The REACHED binary, not the one a path says should be there: codex is also
# installable by npm, bun, pnpm and a standalone installer, each its own
# update channel, and any of them linked ahead of the cask leaves the pin
# asserting a binary nothing runs.
live_ver=$(codex --version 2>/dev/null | awk '{print $2}')
cfg_cfuos=$(val check_for_update_on_startup "$cfg")

pinned=unpinned
brew list --pinned 2>/dev/null | grep -qx "$want_cask" && pinned=pinned

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
if [ -d "$room" ]; then
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

# Upstream. `brew outdated --cask --verbose` with no package named exits 0
# whether or not anything is outdated (measured, Homebrew 6.0.20 — the
# per-package form exits 1 when that package IS outdated, which is why this
# reads the unqualified listing and greps). An empty listing is a legitimate
# answer, so "brew answered" is distinguished from "brew failed" by the exit
# status, not by the output being empty: a failed brew must not read as
# "nothing to re-audit" and quietly switch the gate off.
outd=$(brew outdated --cask --verbose 2>/dev/null); brc=$?
upstream=""
if [ "$brc" -le 1 ]; then
  upstream=$(printf '%s\n' "$outd" | sed -n "s/^$want_cask (.*) != \([^ ]*\).*/\1/p" | head -1)
  [ -n "$upstream" ] || upstream=$want_ver
  printf '  %-30s %-12s read\n' "tap version" "$upstream"
else
  printf '  %-30s %-12s <-- FAIL (brew outdated exited %s)\n' "tap version" "?" "$brc"
  fail=$((fail + 1))
fi

ver_gt() { [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)" = "$1" ]; }

echo
echo "ACCEPTED RISK (ranger-base-poj5): codex has no required_maximum_version"
echo "equivalent. Everything above refuses to MOVE the binary; nothing refuses"
echo "to RUN one that got past the pin by another route."

echo
if [ -n "$upstream" ] && ver_gt "$upstream" "$want_ver"; then
  cat <<EOF
UPSTREAM MOVED: the codex cask is $upstream; the fleet is pinned at $want_ver.

The pin is holding — nothing has changed on this machine. Lifting it is the
operator's call, and each item below was verified against $want_ver only:

  1. FETCH THE ROLLBACK ARTIFACT FIRST. A cask keeps one version. The moment
     $upstream lands, \`brew cleanup\` deletes the only $want_ver copy on this
     box and homebrew-cask carries no version history. The URL and sha256 are
     in $pin, [rollback].
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
