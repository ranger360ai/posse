#!/usr/bin/env bash
# Verify the fleet's grok version pin (rangerhq-y7jr) against the live install,
# and report when upstream has moved past it.
#
# The pin exists because grok's `[cli] auto_update = true` default lets the
# binary — and the long-lived leader process, mid-life — roll forward with no
# review. Several things the fleet depends on are version-VERIFIED, not
# contractual (see the re-audit list below and NOTES.md "grok substrate"), so an
# unreviewed roll-forward silently retires those findings.
#
# Two ceilings, and they are not the same gate. maximum_version is soft: the
# updater will not INSTALL above it, but a binary that got there some other way
# still starts. required_maximum_version is hard — grok refuses to start above
# it (measured, rangerhq-iy3y: the config key gates, not only the env var), so
# an unreviewed upgrade is a loud fleet-wide stop rather than a silent run.
# Both are declared in the pin file and both are checked below; an UNSET hard
# ceiling reads empty and FAILs, because that is the state this pin exists to
# make impossible.
#
# This script is also the gate: when upstream stable moves past the pinned
# version it prints the re-audit list. Lifting the pin is an operator action and
# is gated on the security lane re-running that list against the new build.
set -uo pipefail

cd "$(dirname "$0")/.."
pin=etc/grok/version-pin.toml
cfg=${GROK_HOME:-$HOME/.grok}/config.toml
fail=0

command -v grok >/dev/null || { echo "verify-grok-pin: grok not on PATH"; exit 2; }
[ -r "$pin" ] || { echo "verify-grok-pin: missing $pin"; exit 2; }
[ -r "$cfg" ] || { echo "verify-grok-pin: missing $cfg"; exit 2; }

val() { sed -n "s/^$1 *= *\"\{0,1\}\([^\"#]*\)\"\{0,1\}.*/\1/p" "$2" | head -1 | tr -d ' '; }

want_ver=$(val posse_pinned_version "$pin")
want_max=$(val maximum_version "$pin")
want_req=$(val required_maximum_version "$pin")
live_ver=$(grok --version 2>/dev/null | awk '{print $2}')
cfg_auto=$(val auto_update "$cfg")
cfg_max=$(val maximum_version "$cfg")
cfg_req=$(val required_maximum_version "$cfg")

# The authority on what the updater will actually do — grok's own answer, not
# our reading of the config file. Network call; empty when offline.
#
# Parse tolerantly: live 1.0.5 emits compact JSON, but a space after the colon
# or a pretty-printed payload is the same answer and must read the same
# (ranger-base-ocfh — the old extractor required the value to sit immediately
# after the colon, so `"autoUpdate": true` captured nothing and fell into the
# offline arm: config false, updater true, exit 0 "pin intact"). Only an EMPTY
# payload is offline. A payload we cannot parse is a FAIL, not silence — if the
# field is ever renamed or restyled this script says so instead of passing.
# That rule is BOTH fields' (ranger-base-phxj): `latestVersion` gets the same
# three arms below, because losing it is the worse of the two — it turns the
# re-audit gate off without a word.
#
# jsplit puts every `"key": value` pair at the start of its own line, and both
# extractors then anchor to `^`. Two things follow. A key spelled inside a
# STRING VALUE can no longer match. And the FIRST occurrence wins on a compact
# payload exactly as `head -1` already made it win on a pretty one — before
# this the leading `.*` was greedy, so on one line the LAST `"autoUpdate"` won
# and a true answer could be masked by a nested false one, while on many lines
# the first won: the two shapes disagreed about which answer was authoritative.
# The captures require at least one character on purpose, so an unreadable
# shape (`"true"`, `null`, `1.9`, `""`) yields NOTHING rather than an empty
# match that would both look like an answer and hide any later line.
# POSIX BRE only, no alternation: BSD sed has no `\|`.
jsplit() { printf '%s' "$1" | tr ',{}[]' '\n\n\n\n\n'; }
jstr() { jsplit "$2" | sed -n "s/^[[:space:]]*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"][^\"]*\)\".*/\1/p" | head -1; }
jword() { jsplit "$2" | sed -n "s/^[[:space:]]*\"$1\"[[:space:]]*:[[:space:]]*\([a-z][a-z]*\).*/\1/p" | head -1; }

chk=$(grok update --check --json 2>/dev/null)
live_auto=$(jword autoUpdate "$chk")
upstream=$(jstr latestVersion "$chk")

chk_row() { # label want got
  if [ "$2" = "$3" ]; then printf '  %-28s %-10s ok\n' "$1" "$3"
  else printf '  %-28s %-10s <-- FAIL (want %s)\n' "$1" "${3:-?}" "$2"; fail=$((fail + 1)); fi
}

echo "grok version pin — $pin"
chk_row "grok --version"            "$want_ver"  "$live_ver"
chk_row "config auto_update"        "false"      "$cfg_auto"
chk_row "config maximum_version"    "$want_max"  "$cfg_max"
chk_row "config required_max_ver"   "$want_req"  "$cfg_req"
case "$live_auto" in
  true | false)
    chk_row "grok update: autoUpdate"  "false"     "$live_auto" ;;
  *)
    if [ -n "$chk" ]; then
      printf '  %-28s %-10s <-- FAIL (grok answered; no autoUpdate boolean in it)\n' "grok update: autoUpdate" "${live_auto:-?}"
      fail=$((fail + 1))
    else
      printf '  %-28s %-10s (offline? `grok update --check --json` returned nothing)\n' "grok update: autoUpdate" "—"
    fi ;;
esac

# latestVersion, same three arms (ranger-base-phxj). Its readability is not a
# detail of the block below: an empty `$upstream` skips UPSTREAM MOVED in
# silence, so `null`, an unquoted number and a rename all read as "nothing to
# re-audit" and the gate on lifting the pin is simply off. The row says out
# loud whether the gate is live. `read` rather than `ok` because reading the
# field is all this row claims — whether upstream MOVED is the block below, and
# a move is not a failure.
if [ -n "$upstream" ]; then
  printf '  %-28s %-10s read\n' "grok update: latestVersion" "$upstream"
elif [ -n "$chk" ]; then
  printf '  %-28s %-10s <-- FAIL (grok answered; no latestVersion in it)\n' "grok update: latestVersion" "?"
  fail=$((fail + 1))
else
  printf '  %-28s %-10s (offline? `grok update --check --json` returned nothing)\n' "grok update: latestVersion" "—"
fi

ver_gt() { [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)" = "$1" ]; }

echo
if [ -n "$upstream" ] && ver_gt "$upstream" "$want_ver"; then
  cat <<EOF
UPSTREAM MOVED: grok stable is $upstream; the fleet is pinned at $want_ver.

The pin is holding — nothing has changed on this machine. Lifting it is the
operator's call and is gated on a security re-audit of the NEW build, because
each item below was verified against $want_ver only and none of it is contractual:

  1. Coding-data consent (rangerhq-sz7u, security). In 1.0.5 the consent-record
     RPC x.ai/consent/record has NO server handler, so even an accidental
     [Opt in] on the startup splash cannot persist. Re-check whether $upstream
     ships the handler; if it does, the accidental-opt-in question is live again
     and a dispatched Enter must be proven not to land on [Opt in].
  2. Permission-mode precedence (rangerhq-vjl, ranger-base-ejd, security). CLI
     --permission-mode beats [ui] permission_mode in config.toml
     (runtime.go:361-369). This is the ONLY thing keeping fleet grok launches
     off this machine's always-approve config fallback. Re-verify before the
     fleet dispatches into $upstream.
  3. The rule dialect and launcher contract (NOTES.md "Grok specifics"):
     --rules="\$(cat ...)" in the = form, --allow/--deny matching on
     shell-parsed segments, git push on grok's own dangerous list, and the
     login-shell capture that L1 rides on.
  4. The startup-splash detection override (etc/herdr/agent-detection/grok.toml,
     rangerhq-37c/1xsj) — re-run \`make verify-detection\` against the new splash.

  Runbook to lift the pin: NOTES.md, "grok substrate: pinned at $want_ver".
EOF
elif [ -n "$upstream" ]; then
  echo "upstream stable: $upstream (== the pin; nothing to re-audit)"
fi

echo
if [ "$fail" -ne 0 ]; then
  echo "verify-grok-pin: $fail check(s) FAILED — the pin is not what $pin says"
  exit 1
fi
echo "verify-grok-pin: pin intact at $want_ver"
