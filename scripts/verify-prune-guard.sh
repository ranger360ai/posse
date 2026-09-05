#!/usr/bin/env bash
# Verify a posse binary's session-meta prune guard against the LIVE herdr,
# without risking a single fleet meta (rangerhq-m15, the promote gate for
# rangerhq-8fq / abb2716).
#
# Usage: scripts/verify-prune-guard.sh [path-to-posse]      (default: $(command -v posse))
#        scripts/verify-prune-guard.sh bin/posse-release    # the promote candidate
#
# The rig is one scratch RHQ_HOME under $TMPDIR holding four planted metas:
#   ghost-foreign     records a socket this server is not     -> must be KEPT
#   ghost-socketless  records no socket at all                -> must be KEPT (abb2716)
#   <label>           names a workspace this server DOES hold, under that
#                     workspace's own label                   -> must be LISTED,
#                     and gets socket: and gen: stamped on it (the abb2716
#                     backfill, plus rangerhq-yt1p's generation fence;
#                     gen: is N:N:N = dev:ino:mtime after ranger-base-fjj)
#   ghost-stranger    names that SAME live workspace under a name that is not
#                     its label                               -> must be KEPT
#                     and NOT listed (rangerhq-yt1p): the id answers alive for
#                     somebody else's workspace, which proves nothing about
#                     this session, and listing it would put the name on a
#                     stranger's pane.
# Both ghosts name workspace ids herdr does not have, and every planted
# launched: stamp is two hours old, so they are past PruneGrace and clear of
# the rangerhq-9nso grace arm — what is left under test is the socket guard
# and the identity check.
#
# Safe direction, and only this direction: RHQ_HOME is scratch on the process
# WE run, so every write lands in the scratch home; herdr is only ever read
# (workspace list + a workspace-alive query per absent meta). The rangerhq-snd
# wipe was the reverse — a real RHQ_HOME against a scratch socket.
#
# A pre-abb2716 binary (the fleet's 757e13d) fails this: it prunes
# ghost-socketless, because its different-socket arm was gated on
# m.Socket != "" and so could never fire for a meta that records none.
set -uo pipefail

RHQ=${1:-$(command -v posse)}
[ -x "$RHQ" ] || { echo "verify-prune-guard: not executable: ${RHQ:-<none>}"; exit 2; }
command -v herdr >/dev/null || { echo "verify-prune-guard: herdr not on PATH"; exit 2; }

sock=${HERDR_SOCKET_PATH:-$HOME/.config/herdr/herdr.sock}
[ -S "$sock" ] || { echo "verify-prune-guard: no herdr socket at $sock"; exit 2; }

# A workspace this server really holds, for the backfill arm — and its LABEL,
# because since rangerhq-yt1p a meta is only that workspace's if it wears its
# name (a workspace id alone is re-issued across a server restart or handoff).
# The label has to be usable as a session name, since that is what the meta's
# filename is.
read -r live label < <(herdr workspace list 2>/dev/null | python3 -c '
import json, re, sys
for w in json.load(sys.stdin)["result"]["workspaces"]:
    if re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_-]*", w.get("label") or ""):
        print(w["workspace_id"], w["label"])
        break
' 2>/dev/null)
[ -n "${live:-}" ] || { echo "verify-prune-guard: herdr holds no workspace with a session-shaped label; nothing to test the backfill against"; exit 2; }

home=$(mktemp -d "${TMPDIR:-/tmp}/posse-prune-guard.XXXXXX")
trap 'rm -rf "$home"' EXIT
metas=$home/state/herdr
mkdir -p "$metas"

old=$(python3 -c 'import datetime as d; print((d.datetime.now(d.timezone.utc)-d.timedelta(hours=2)).strftime("%Y-%m-%dT%H:%M:%SZ"))')
printf 'name: ghost-foreign\nworkspace: w404\npane: w404:p1\nemoji: G\nagent: developer\nruntime: claude\nlaunched: %s\nsocket: /tmp/not-this-server/herdr.sock\n' "$old" > "$metas/ghost-foreign.yaml"
printf 'name: ghost-socketless\nworkspace: w405\npane: w405:p1\nemoji: G\nagent: qa\nruntime: claude\nlaunched: %s\n' "$old" > "$metas/ghost-socketless.yaml"
printf 'name: %s\nworkspace: %s\npane: %s:p1\nemoji: G\nagent: architect\nruntime: claude\nlaunched: %s\n' "$label" "$live" "$live" "$old" > "$metas/$label.yaml"
printf 'name: ghost-stranger\nworkspace: %s\npane: %s:p1\nemoji: G\nagent: devops\nruntime: claude\nlaunched: %s\nsocket: %s\n' "$live" "$live" "$old" "$sock" > "$metas/ghost-stranger.yaml"

echo "verify-prune-guard: $RHQ ($("$RHQ" version 2>/dev/null))"
echo "  herdr socket : $sock"
echo "  live space   : $live ($label)"
echo "  scratch home : $home"
echo

out=$(RHQ_HOME=$home HERDR_SOCKET_PATH=$sock "$RHQ" list 2>&1)
printf '%s\n\n' "$out"

fail=0
check() { # check <label> <condition-result> <detail>
  if [ "$2" = 0 ]; then printf '  OK   %s\n' "$1"
  else printf '  FAIL %s — %s\n' "$1" "$3"; fail=1; fi
}

# Every arm below is decided by bash, not by a forked matcher
# (ranger-base-s8b4g, ranger-base-7hx87). A `grep`/`awk` that is signalled,
# that cannot be exec'd under load, or that takes EPIPE returns non-zero over
# text that carries the line, and every arm here reads non-zero as "the guard
# did not fire" — a FAIL against a binary that is correct. The absolute
# `/usr/bin/grep` the gen: arms used is the same defect: an absolute path
# survives a PATH shim, not a signal or a failed fork.
#
# has_text <text> <literal> — `grep -qF` over $out or $listing.
has_text() { case $1 in *"$2"*) return 0 ;; esac; return 1; }

# line_eq <file> <line> — `grep -q "^<line>$"`.
line_eq() {
  local l
  [ -r "$1" ] || return 1
  while IFS= read -r l || [ -n "$l" ]; do
    [ "$l" = "$2" ] && return 0
  done <"$1"
  return 1
}

# line_starts <file> <prefix> — `grep -q "^<prefix>"`.
line_starts() {
  local l
  [ -r "$1" ] || return 1
  while IFS= read -r l || [ -n "$l" ]; do
    case $l in "$2"*) return 0 ;; esac
  done <"$1"
  return 1
}

# meta_field <file> <key> — the value after `<key>: ` on the first line that
# starts with `<key>:`, non-zero when there is no such line.
meta_field() {
  local l
  [ -r "$1" ] || return 1
  while IFS= read -r l || [ -n "$l" ]; do
    case $l in
    "$2:"*)
      l=${l#"$2:"}
      printf '%s' "${l# }"
      return 0
      ;;
    esac
  done <"$1"
  return 1
}

# digits <s> — non-empty and all digits.
digits() { case ${1:-} in '' | *[!0-9]*) return 1 ;; esac; return 0; }

# after_marker <text> <line-prefix> — every line after the FIRST line that
# starts with <line-prefix>, which is `awk '/^<prefix>/{on=1;next} on'`.
after_marker() {
  local l on=0 buf=
  while IFS= read -r l || [ -n "$l" ]; do
    if [ "$on" = 1 ]; then
      buf=$buf$l$'\n'
      continue
    fi
    case $l in "$2"*) on=1 ;; esac
  done <<<"$1"
  printf '%s' "${buf%$'\n'}"
}

# listed <listing> <label> — a line carrying ` <label>` followed by a space or
# the end of the line, which is `grep -q " <label>\( \|$\)"`.
listed() {
  local l
  while IFS= read -r l || [ -n "$l" ]; do
    case $l in
    *" $2 "* | *" $2") return 0 ;;
    esac
  done <<<"$1"
  return 1
}

[ -f "$metas/ghost-socketless.yaml" ]; check "socket-less meta kept (abb2716)" $? "PRUNED — this binary predates abb2716"
[ -f "$metas/ghost-foreign.yaml" ];    check "foreign-socket meta kept (9ac4a16)" $? "PRUNED — the different-socket arm is not firing"
has_text "$out" '2 session meta file(s) kept, not listed'
check "both refusals reported on stderr" $? "expected a '2 ... kept, not listed' warning"
listing=$(after_marker "$out" "HERDR SESSIONS")
listed "$listing" "$label"
check "live-workspace meta listed" $? "$label missing from the listing itself"
line_eq "$metas/$label.yaml" "socket: $sock"
check "socket: backfilled onto the live meta (abb2716)" $? "no backfill — the meta would stay unprunable forever"
# ranger-base-fjj: ServerGen is dev:ino:mtime (three fields). The two-field
# shape '^gen: N:N$' is a false FAIL against a correct stamp — it is the
# pre-fjj token, and matching it would sign off on the linux inode-reuse hole.
gen_label="gen: backfilled onto the live meta (rangerhq-yt1p / ranger-base-fjj)"
if gen=$(meta_field "$metas/$label.yaml" gen); then
  gen_a=${gen%%:*}
  gen_r=${gen#*:}
  gen_b=${gen_r%%:*}
  gen_c=${gen_r#*:}
  if [ "$gen_r" != "$gen" ] && [ "$gen_c" != "$gen_r" ] &&
    digits "$gen_a" && digits "$gen_b" && digits "$gen_c"; then
    check "$gen_label" 0
  elif [ "$gen_r" != "$gen" ] && [ "$gen_c" = "$gen_r" ] &&
    digits "$gen_a" && digits "$gen_b"; then
    check "$gen_label" 1 "two-field gen: (dev:ino) — ranger-base-fjj requires bind time as a third field"
  else
    check "$gen_label" 1 "gen: present but not N:N:N — gen: $gen"
  fi
else
  check "$gen_label" 1 "no generation stamped — the next restart cannot tell a rename from a re-issued id"
fi
line_eq "$metas/$label.yaml" "launched: $old"
check "backfill preserved launched:" $? "the rewrite dropped fields it should have kept"

# rangerhq-yt1p: the same live workspace, claimed by a meta whose name is not
# its label. Liveness is proven and identity is not, so the file stays and the
# session does not appear — a listing there would address a stranger's pane.
[ -f "$metas/ghost-stranger.yaml" ]
check "stranger-id meta kept (rangerhq-yt1p)" $? "PRUNED — a live workspace's id is not a licence to delete another meta"
has_text "$listing" ghost-stranger
if [ $? = 0 ]; then check "stranger-id meta not listed (rangerhq-yt1p)" 1 "listed under a workspace labelled '$label' — a prompt would land in somebody else's pane"; else check "stranger-id meta not listed (rangerhq-yt1p)" 0; fi
has_text "$out" 'another workspace holds the id they recorded'
check "the re-issued id is reported with its repair" $? "no warning naming the identity mismatch"
if [ -f "$metas/ghost-socketless.yaml" ]; then
  ! line_starts "$metas/ghost-socketless.yaml" "socket:"
  check "no socket guessed for an absent workspace" $? "stamped a socket it had no proof of"
else
  printf '  SKIP %s\n' "no socket guessed for an absent workspace (meta was pruned)"
fi

echo
if [ $fail = 0 ]; then echo "verify-prune-guard: PASS"; else echo "verify-prune-guard: FAIL"; fi
exit $fail
