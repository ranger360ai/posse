#!/usr/bin/env bash
# Verify the fleet's grok version pin (rangerhq-y7jr) against the live install,
# and report when upstream has moved past it.
#
# THE PIN GOVERNS THE HOMEBREW CASK `grok-build` (operator ruling 2026-10-31,
# ranger-base-yq1ar, applied under ranger-base-7qey3). It used to govern the
# `~/.grok` managed install, and this script used to read only grok itself; the
# box moved to the cask on 2026-10-12 and the ruling is to keep it. So there are
# now two halves to assert, and neither can stand in for the other:
#
#   grok's own config  — ~/.grok/config.toml's two ceilings, which is where grok
#                        reads its policy from whatever installed the binary,
#                        plus the ONE key that is not a ceiling at all:
#                        `[ui.notifications] program_status`, which ADR 0076
#                        D12/D13 requires for a grok seat on a
#                        `multiplexer: rex` instance. It is here because it is
#                        the same mechanism — posse declares what it needs in
#                        the operator's config and never writes that file —
#                        and because the measurement behind it was taken on
#                        the pinned release only (re-audit item 5).
#   brew               — the installed cask version, the cask pin, and the
#                        Caskroom directory, because `brew upgrade` is not
#                        grok's updater and obeys none of grok's config.
#
# The pin exists because grok's `[cli] auto_update = true` default lets the
# binary — and the long-lived leader process, mid-life — roll forward with no
# review. Several things the fleet depends on are version-VERIFIED, not
# contractual (see the re-audit list below and docs/notes.d/notes-grok.md), so an
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
# UPSTREAM IS READ FROM THE CASK TAP, NOT FROM GROK. This used to read grok's
# own `latestVersion`, and that made the re-audit gate unreachable by
# construction. MEASURED 2026-10-31 (ranger-base-7qey3), four `grok update
# --check --json` runs against a scratch $GROK_HOME differing only in
# maximum_version: with no ceiling grok answered `"latestVersion":"1.0.70"`;
# with the ceiling at 1.0.52 it answered `"1.0.52"`; with it at 1.0.5 it
# answered `"1.0.5"` — BELOW the installed binary. `latestVersion` is clamped to
# the soft ceiling, which the pin always sets to the pinned version, so
# `latestVersion == want_ver` whenever the pin is intact and "UPSTREAM MOVED"
# could never fire. It only ever fired on this box because the live config and
# the pin file disagreed. `brew info --cask` names the tap version whatever is
# installed and no grok config can clamp it, so that is the gate's input now —
# and `latestVersion` is kept as a CHECKED row against the soft ceiling, a
# positive control that the updater is obeying it.
#
# This script is also the gate: when upstream stable moves past the pinned
# version it prints the re-audit list. Lifting the pin is an operator action and
# is gated on the security lane re-running that list against the new build.
#
# Exit 2 = nothing measured (grok or brew absent, pin or config unreadable).
# Exit 1 = a check failed. Exit 0 = the pin is intact.
set -uo pipefail

# A gate must not change the box it is reading, and a `brew` read has a
# footprint unless it is told not to: with the variable unset, an answer that
# comes from Homebrew's cached package JSON re-downloads that file (~16 MB)
# when the cached copy is over 7 days old, mid-gate. scripts/verify-box.sh
# runs verify-grok-pin.sh, verify-codex-pin.sh and verify-bd-pin.sh back to
# back off that ONE cache, so an unguarded read in the first can move the data
# the next two read.
#
# `brew info` and `brew list` do NOT auto-update Homebrew and do NOT re-fetch
# taps — they are not in Homebrew's auto-update command set, MEASURED, and the
# banner that first suggested otherwise belonged to a `brew upgrade --cask`
# typed beside them. Both arms, and the mechanism that IS real, are in
# docs/notes.d/ranger-base-et7bs.md.
#
# The ANALYTICS kill is the second footprint, and it leaves the box
# (ranger-base-w2bha, ranger-base-esz16): analytics are ON for brew on this box
# — `homebrew.analyticsdisabled` is unset in HOMEBREW_REPOSITORY's git config,
# MEASURED 2026-10-31, which is brew's default — so with the variable unset
# every brew read below POSTs a command_run event to Homebrew's InfluxDB. The
# source audience of a crew-run check is this box; that endpoint is wider, and
# nothing here rules the wider one acceptable, so it does not move (crew hard
# risk line 4). The box's own `brew analytics` setting is the operator's, for
# his own brew use, and no gate changes it.
#
# The ENV_HINTS kill is cosmetic: it keeps brew's advice block out of the rows.
export HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1

cd "$(dirname "$0")/.."
pin=etc/grok/version-pin.toml
grok_home=${GROK_HOME:-$HOME/.grok}
cfg=$grok_home/config.toml
fail=0

command -v grok >/dev/null || { echo "verify-grok-pin: grok not on PATH"; exit 2; }
command -v brew >/dev/null || { echo "verify-grok-pin: brew not on PATH"; exit 2; }
[ -r "$pin" ] || { echo "verify-grok-pin: missing $pin"; exit 2; }
[ -r "$cfg" ] || { echo "verify-grok-pin: missing $cfg"; exit 2; }

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

# tomlpath <file> <dotted.key.path> — the scalar at that FULL key path, read
# SECTION-AWARE, for the one key in this file whose name is not unique to one
# table (ADR 0076 D13, ranger-base-nhxdx). `val` above is deliberately
# section-blind — it is tomlFlag's rule in shell, and it is right for keys that
# occur once in grok's config. `program_status` is not one of those: it is a
# plausible name under more than one table, and the one table that counts is
# `[ui.notifications]`. A section-blind read would take `program_status = "on"`
# out of any table and call the precondition met where posse REFUSES it, which
# is a gate reporting green over a launch posse will not make.
#
# It is the shell twin of grokrex.go's tomlPathValue and reads the same three
# spellings, which is the contract and not a sample:
#
#   [ui.notifications]  + program_status = "on"
#   [ui]                + notifications.program_status = "on"
#   (root)                ui.notifications.program_status = "on"
#
# and nothing else — an inline table, a multi-line value, a quoted bare key
# carrying a dot all read as absent, which FAILS the row rather than passing
# it. Same direction as the Go reader, same reason: the cost is one line
# re-spelled and the cost of guessing the other way is a gate that lies.
#
# No forks, per the extractor note at the top of this file: this decides an
# arm. `[[array.of.tables]]` sets a table nothing can match rather than being
# skipped, because skipping it would leave the previous header in force and
# read that array's keys as the enclosing table's.
tomlpath() {
  local file=$1 want=$2 line t rest table= key v path
  [ -r "$file" ] || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    t=${line#"${line%%[![:space:]]*}"}
    t=${t%"${t##*[![:space:]]}"}
    case $t in
      '' | '#'*) continue ;;
      '[['*)
        table=$'\n'
        continue
        ;;
      '['*)
        rest=${t#\[}
        case $rest in
          *\]*) table=${rest%%\]*} ;;
          *) table=$'\n' ;;
        esac
        continue
        ;;
    esac
    case $t in *=*) ;; *) continue ;; esac
    key=${t%%=*}
    v=${t#*=}
    # The composed path, with every space and quote removed: a bare TOML key
    # cannot contain either, and a quoted one that does is a spelling this
    # refuses by design (above).
    path=$table.$key
    [ -z "$table" ] && path=$key
    path=${path//[\"\' ]/}
    path=${path//$'\t'/}
    [ "$path" = "$want" ] || continue
    v=${v#"${v%%[![:space:]]*}"}
    case $v in
      '"'*)
        v=${v#\"}
        printf '%s' "${v%%\"*}"
        return 0
        ;;
      "'"*)
        v=${v#\'}
        printf '%s' "${v%%\'*}"
        return 0
        ;;
    esac
    v=${v%%#*}
    printf '%s' "${v%"${v##*[![:space:]]}"}"
    return 0
  done <"$file"
}

# field2 <text> — the second whitespace-separated field of the FIRST line,
# replacing `| awk '{print $2}'`. Deliberately stricter than the awk it
# replaces, which printed field 2 of EVERY line: the version answer is one
# line, and a second line is a surprise the row should show rather than
# silently concatenate. Both `grok --version` ("grok 1.0.52 (sha) [stable]")
# and `brew list --cask --versions grok-build` ("grok-build 1.0.52") put their
# version in field 2.
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
# token. This reads the TAP version out of `brew info --cask`, whose header
# comes in three shapes, all measured (Homebrew 7.0.11):
#
#   installed == tap     ==> codex (Codex): 0.153.4
#   installed <  tap     ==> grok-build (Grok Build): 1.0.52 → 1.0.70
#   auto_updates cask    ==> thaw (Thaw): 1.2.0 → 2.0.1 (auto_updates)
#
# The TAP version is the last one on the line in all three, so reading it as
# "last digit-initial token" is blind both to how the arrow is encoded and to
# any trailing parenthesised flag. A header with no version-shaped token
# (a `version :latest` cask) yields nothing and the caller fails the row.
# The split is written out rather than `for tok in $s`, whose word splitting
# also globs. Shared with verify-codex-pin.sh, same reasoning.
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
want_max=$(val maximum_version "$pin")
want_req=$(val required_maximum_version "$pin")
want_cask=$(val formula "$pin")
want_pin=$(val pin_state "$pin")
want_room=$(val caskroom_dir "$pin")
# The third key posse REQUIRES in the operator's config (ADR 0076 D13), and
# the only one read section-aware — see tomlpath above for why.
grok_ps_path=ui.notifications.program_status
want_ps=$(tomlpath "$pin" "$grok_ps_path")

# The REACHED binary, not the one a path says should be there: grok is also
# installable by its own internal installer into $GROK_HOME/bin (which is what
# a rollback recreates — see the runbook), and either one linked ahead of the
# other leaves the pin asserting a binary nothing runs.
live_ver=$(field2 "$(grok --version 2>/dev/null)")
cfg_auto=$(val auto_update "$cfg")
cfg_max=$(val maximum_version "$cfg")
cfg_req=$(val required_maximum_version "$cfg")
cfg_ps=$(tomlpath "$cfg" "$grok_ps_path")

# What brew RECORDS as installed. This is the row the operator's lift-gate asks
# for: it reds the moment a `brew upgrade` moves the cask, whether or not the
# binary on PATH is the one that moved. Nonzero brew, or an answer with no
# version in it, both read empty and fail the row rather than passing.
cask_installed=$(field2 "$(brew list --cask --versions "$want_cask" 2>/dev/null)")

# `grep -qx` without the fork (ranger-base-s8b4g): a dying grep would report
# the cask unpinned — a FAIL row against a box where the pin is holding.
pinned=unpinned
pinned_list=$(brew list --pinned 2>/dev/null)
while IFS= read -r line || [ -n "$line" ]; do
  if [ "$line" = "$want_cask" ]; then
    pinned=pinned
    break
  fi
done <<<"$pinned_list"

# The authority on what the updater will actually do — grok's own answer, not
# our reading of the config file. Network call; empty when offline.
#
# Parse tolerantly: live grok emits compact JSON, but a space after the colon
# or a pretty-printed payload is the same answer and must read the same
# (ranger-base-ocfh — the old extractor required the value to sit immediately
# after the colon, so `"autoUpdate": true` captured nothing and fell into the
# offline arm: config false, updater true, exit 0 "pin intact"). Only an EMPTY
# payload is offline. A payload we cannot parse is a FAIL, not silence — if the
# field is ever renamed or restyled this script says so instead of passing.
# That rule is BOTH fields' (ranger-base-phxj): `latestVersion` gets the same
# three arms below. What CHANGED under ranger-base-7qey3 is what the readable
# answer is compared against, not whether an unreadable one fails — see the
# header: latestVersion is clamped to the soft ceiling, so it is a control on
# that ceiling and no longer the upstream read.
#
# jsplit puts every `"key": value` pair at the start of its own line, and both
# extractors then anchor to `^`. Two things follow. The FIRST occurrence wins
# on a compact payload exactly as `head -1` already made it win on a pretty one
# — before this the leading `.*` was greedy, so on one line the LAST
# `"autoUpdate"` won and a true answer could be masked by a nested false one,
# while on many lines the first won: the two shapes disagreed about which
# answer was authoritative. And only a line that IS a key/value pair can
# answer. A key cannot literally appear inside a well-formed JSON string value
# — the quotes there are escaped (measured) — so the reachable case is grok
# printing a plain line alongside the payload; unanchored, `head -1` read
# `false` out of a warning that merely mentioned the key.
# The captures require at least one character on purpose, so an unreadable
# shape (`"true"`, `null`, `1.9`, `""`) yields NOTHING rather than an empty
# match that would both look like an answer and hide any later line.
JSPLIT=
jsplit() { JSPLIT=${1//[],\{\}[]/$'\n'}; }
jstr() {
  local line rest v
  jsplit "$2"
  while IFS= read -r line || [ -n "$line" ]; do
    rest=${line#"${line%%[![:space:]]*}"}
    case $rest in "\"$1\""*) ;; *) continue ;; esac
    rest=${rest#"\"$1\""}
    rest=${rest#"${rest%%[![:space:]]*}"}
    case $rest in :*) ;; *) continue ;; esac
    rest=${rest#:}
    rest=${rest#"${rest%%[![:space:]]*}"}
    case $rest in '"'*) ;; *) continue ;; esac
    rest=${rest#\"}
    case $rest in *\"*) ;; *) continue ;; esac
    v=${rest%%\"*}
    [ -n "$v" ] || continue
    printf '%s' "$v"
    return 0
  done <<<"$JSPLIT"
}
jword() {
  local line rest v
  jsplit "$2"
  while IFS= read -r line || [ -n "$line" ]; do
    rest=${line#"${line%%[![:space:]]*}"}
    case $rest in "\"$1\""*) ;; *) continue ;; esac
    rest=${rest#"\"$1\""}
    rest=${rest#"${rest%%[![:space:]]*}"}
    case $rest in :*) ;; *) continue ;; esac
    rest=${rest#:}
    rest=${rest#"${rest%%[![:space:]]*}"}
    case $rest in [a-z]*) ;; *) continue ;; esac
    v=${rest%%[!a-z]*}
    printf '%s' "$v"
    return 0
  done <<<"$JSPLIT"
}

chk=$(grok update --check --json 2>/dev/null)
live_auto=$(jword autoUpdate "$chk")
clamped=$(jstr latestVersion "$chk")

chk_row() { # label want got
  if [ "$2" = "$3" ]; then printf '  %-30s %-12s ok\n' "$1" "$3"
  else printf '  %-30s %-12s <-- FAIL (want %s)\n' "$1" "${3:-?}" "$2"; fail=$((fail + 1)); fi
}

echo "grok version pin — $pin"
chk_row "grok --version"            "$want_ver"  "$live_ver"
chk_row "installed cask version"    "$want_ver"  "$cask_installed"
chk_row "brew cask pin"             "$want_pin"  "$pinned"
chk_row "config auto_update"        "false"      "$cfg_auto"
chk_row "config maximum_version"    "$want_max"  "$cfg_max"
chk_row "config required_max_ver"   "$want_req"  "$cfg_req"
# The precondition ADR 0076 D12 puts in front of every grok seat on a
# `multiplexer: rex` instance, asserted in both directions:
#
#   the PIN row says the declaration still names the one admitting value. "on"
#   is hardcoded here rather than taken from the pin file, because that is the
#   whole assertion — posse admits `on` and nothing else (MEASURED: `auto`, the
#   shipped default, is silent on Rex AND leaks the `n,auth` tail of Rex's
#   reply into grok's composer; `off` is silent), so a pin file edited to
#   declare `auto` would otherwise agree with itself.
#   the CONFIG row says the operator's own file carries it. posse READS that
#   file and never writes it (D13): config.toml holds the operator's
#   preferences and grok rewrites it itself, so a copy over it would clobber
#   both. This row is the whole mechanism by which the requirement is kept.
#
# An absent key reads empty and FAILs, which is the state this gate exists to
# make visible: until the line is there, a dispatched grok on Rex is refused
# before the claim and the refusal names this file.
chk_row "pin program_status"        "on"         "$want_ps"
chk_row "config program_status"     "on"         "$cfg_ps"
case "$live_auto" in
  true | false)
    chk_row "grok update: autoUpdate"  "false"     "$live_auto" ;;
  *)
    if [ -n "$chk" ]; then
      printf '  %-30s %-12s <-- FAIL (grok answered; no autoUpdate boolean in it)\n' "grok update: autoUpdate" "${live_auto:-?}"
      fail=$((fail + 1))
    else
      printf '  %-30s %-12s (offline? `grok update --check --json` returned nothing)\n' "grok update: autoUpdate" "—"
    fi ;;
esac

# latestVersion, same three arms (ranger-base-phxj), against a different
# subject (ranger-base-7qey3). It is NOT the upstream read — it is clamped to
# maximum_version, measured, see the header — so what it proves is that the
# soft ceiling is being obeyed by the updater itself, which is the one thing
# no amount of reading config.toml can establish. A readable value that is not
# the soft ceiling means the clamp is not holding: either the updater stopped
# honouring the key, or it is reading a different config than the one checked
# above. Both are the pin failing, loudly, which is the point.
#
# The field still has to be READABLE for any of that. An empty `$clamped` on a
# non-empty payload is a rename or a restyle, and before ranger-base-phxj it
# passed in silence; it fails here for the same reason it did then.
if [ -n "$clamped" ]; then
  chk_row "grok update: latestVersion" "$want_max" "$clamped"
elif [ -n "$chk" ]; then
  printf '  %-30s %-12s <-- FAIL (grok answered; no latestVersion in it)\n' "grok update: latestVersion" "?"
  fail=$((fail + 1))
else
  printf '  %-30s %-12s (offline? `grok update --check --json` returned nothing)\n' "grok update: latestVersion" "—"
fi

# The rollback target, and the identity of the running binary in one reading.
# `command -v` names a path; only resolving it says which tree answers. A
# Caskroom directory that is present but is NOT what grok runs from is a
# rollback target for a binary nobody uses, so both halves are one row each.
# Shape and reasoning taken from verify-codex-pin.sh, including why
# `caskroom_dir` is asserted to NAME the pinned version rather than merely to
# exist: Homebrew never removes the parent `Caskroom/grok-build`, and any
# installed version's binary sits underneath it, so a prefix compare against
# that parent reads "grok resolves into the pin" ok whatever is running
# (ranger-base-g29az finding 3).
room="$(brew --prefix 2>/dev/null)/$want_room"
room_state=gone
if [ -d "$room" ] && [ "${want_room##*/}" = "$want_ver" ]; then
  room_state=present
  # Both sides get resolved before they are compared. `readlink -f` follows
  # the cask's symlink to a REAL path, so an unresolved prefix on the other
  # side fails a box that is in fact correct — /var vs /private/var is enough,
  # and any Homebrew prefix reached through a symlink is the same bug.
  # `cd && pwd -P` is the portable resolver; there is no `realpath` on stock
  # macOS.
  room=$(cd "$room" && pwd -P)
  printf '  %-30s %-12s ok\n' "rollback target on disk" "present"
  real=$(readlink -f "$(command -v grok)" 2>/dev/null || command -v grok)
  realdir=$(cd "$(dirname "$real")" 2>/dev/null && pwd -P)
  case "$realdir/" in
    "$room"/*) printf '  %-30s %-12s ok\n' "grok resolves into the pin" "yes" ;;
    *)         printf '  %-30s %-12s <-- FAIL (%s)\n' "grok resolves into the pin" "no" "${real:-?}"; fail=$((fail + 1)) ;;
  esac
elif [ -d "$room" ]; then
  printf '  %-30s %-12s <-- FAIL (%s)\n' "rollback target on disk" "unversioned" "$want_room names no version ($want_ver)"
  fail=$((fail + 1))
  # Not measured, and said so rather than counted: a declared path that names
  # no version is not a rollback target for THIS pin, whatever happens to be
  # sitting under it.
  printf '  %-30s %-12s (declared path names no version; nothing to resolve into)\n' "grok resolves into the pin" "—"
else
  printf '  %-30s %-12s <-- FAIL (%s)\n' "rollback target on disk" "GONE" "$room"; fail=$((fail + 1))
  # Not measured, and said so rather than counted: "does grok resolve into a
  # directory that does not exist" has no answer, and a second red row for one
  # broken thing is how a reader learns to skim the list.
  printf '  %-30s %-12s (rollback target gone; nothing to resolve into)\n' "grok resolves into the pin" "—"
fi

# Upstream, read from the TAP and not from grok and not from what happens to be
# installed. Three arms, the same three the JSON fields get, and for the same
# reason: a reader that answers nothing must not be indistinguishable from
# "nothing to re-audit". `brew info --cask` names the tap version whatever is
# installed, so there is nothing to fall back TO, and the one way of not
# getting an answer fails the row: a header with no version in it. Unlike the
# per-package `brew outdated --cask grok-build` form (which exits 1 when that
# cask IS outdated), `brew info --cask grok-build` exits 0 on success and 1 on
# an unknown cask -- and non-zero in a third case, below.
#
# THE EXIT CODE IS NOT THE QUESTION, and requiring 0 here cost this gate the
# caged seats (ranger-base-zf8pf). `brew info --cask` has a THIRD arm beside
# "success" and "unknown cask": it prints the header out of the API cache and
# THEN exits non-zero because it could not REFRESH that cache. That re-download
# is the mechanism brewguard_qa_test.go's head comment names, and it opens
# /opt/homebrew/var/homebrew/locks/<cask>.json.download.lock for write. Six of
# the eleven live PIDs declare `cage: seatbelt` (dinesh, gwart, holden, hoover,
# jian-yang, laurie) and seatbelt.go's allowlist names no /opt/homebrew, so
# from those seats that write is DENIED and the gate reds on a cage artifact —
# on the seats most likely to type it, and it is itself the loud lift-gate the
# 2026-10-31 operator ruling asked for. Found by ranger-base-j1eeh.
#
# CAN, not always: whether brew ATTEMPTS the refresh on a given call is
# brew-internal state nobody here controls, and the guards above happened to
# suppress it in four same-minute probes (the table is in
# docs/notes.d/ranger-base-zf8pf.md). The fix does not depend on which way it
# falls — a version that was read must be used either way — which is the only
# reason that uncertainty is tolerable in a gate.
#
#   MEASURED 2026-10-31, gilfoyle seat under a hand-rendered profile of the
#   same shape ((allow default)(deny file-write*) with no /opt/homebrew),
#   Homebrew 7.0.11-75-gfcdfa04, macOS 26.7.1 arm64:
#
#     brew info --cask grok-build  ->  exit 1, and on STDOUT, first line:
#       ==> grok-build (Grok Build): 1.0.52 -> 1.0.70
#     ...with only this on stderr:
#       Error: Operation not permitted @ rb_sysopen -
#       /opt/homebrew/var/homebrew/locks/grok-build.json.download.lock
#
# So the version was there to be read and the row threw it away. Read the
# header whatever the exit code is and fail only when there is no version in
# it; the unknown-cask arm still fails, because an unknown cask prints no
# header at all. The exit code keeps one job, the one it can do: it tells the
# reader WHY there was nothing to read, and it annotates a row that was read
# off a cache brew could not refresh.
info=$(brew info --cask "$want_cask" 2>/dev/null); brc=$?
upstream=$(lastver "$info")
if [ -n "$upstream" ] && [ "$brc" -eq 0 ]; then
  printf '  %-30s %-12s read\n' "tap version" "$upstream"
elif [ -n "$upstream" ]; then
  # Green, and annotated rather than silent: the version is the tap's, read
  # out of brew's API cache, and brew was unhappy about something downstream
  # of printing it. The word FAIL stays off this line on purpose -- it is how
  # every reader of this table, the QA row matchers included, tell a red row
  # from a green one.
  printf '  %-30s %-12s read (brew exited %s; header read from its API cache)\n' "tap version" "$upstream" "$brc"
elif [ "$brc" -ne 0 ]; then
  printf '  %-30s %-12s <-- FAIL (brew info --cask %s exited %s and printed no version)\n' "tap version" "?" "$want_cask" "$brc"
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
# 12 pairs (ranger-base-s8b4g). Numeric per field is not cosmetic here: grok's
# own versions went 1.0.5 → 1.0.52 → 1.0.70, and a string compare puts 1.0.6
# above 1.0.52.
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
# The tap moving and the BOX moving are two different questions, and the
# paragraph under this heading is true of only one of them. Gated on
# `$upstream` alone, this block said "the pin is holding — nothing has changed
# on this machine" about a box it had never asked. On codex that sentence
# printed, wrong, over two FAILING rows on 2026-09-06 (ranger-base-9ycqa
# finding 1, fixed under ranger-base-xd78m); here it was latent — the pin was
# whole, so the block was unreachable — and this is that twin,
# ranger-base-fed4n. The gate stays the tap's — a move upstream is what makes
# the re-audit list worth printing — and the TEXT branches on live_ver.
#
# grok's mechanism is not codex's and the remedy is not copied from it. codex
# has no version ceiling, so a box past the pin runs happily and the question
# is only which build is being trusted. grok declares two ceilings, and above
# the hard one it refuses to START (rangerhq-iy3y, and etc/grok/version-pin.toml
# names the cost: every dispatched pane and the operator's own grok stop at
# once). So a moved grok box has two shapes worth telling apart, and the second
# is a fleet outage the operator is standing in right now. The ceiling that
# actually gates startup is the one in the operator's CONFIG, so that is what
# is read here — its row above says whether it matches the declaration.
#
# The rollback arm is MEASURED, twice over, and it changed direction between
# those measurements. ranger-base-bttb1 replaced a flat promise that putting
# the pin back "needs no re-fetch: grok keeps the old binaries in
# ~/.grok/downloads/" — a statement about a directory the script had never
# looked at, and false on this box — with a count of that directory. What
# ranger-base-7qey3 adds is the half bttb1 could only call unverified: the
# re-fetch route is real. MEASURED 2026-10-31, from this cask-run binary,
# into a scratch $GROK_HOME: `grok update --version 1.0.5` fetched and
# installed, 134,349,648 bytes matching x.ai's content-length exactly, and
# `https://x.ai/cli/grok-<v>-macos-aarch64` answers 200 for 1.0.5, 1.0.52 and
# 1.0.70 alike. The catch, and it is why this is not simply good news: the
# fetch lands in `$GROK_HOME/downloads/` and recreates `$GROK_HOME/bin/grok`,
# i.e. the MANAGED install — a second install the cask does not know about,
# which the PATH does not prefer, and which can self-update again. The
# rollback COMMANDS are not restated here: docs/notes.d/notes-grok.md carries
# them beside the lift, and one copy of a command that must be right is
# enough.
#
# rb_n — how many `grok-*` binaries $grok_home/downloads/ holds. On the cask
# route that is 0 and the directory does not exist; it becomes non-zero only
# after someone runs the re-fetch above, which is exactly when the operator
# needs to be told that a managed install now exists beside the cask. A glob
# and a `[ -f ]`, no fork: this decides an arm, and per the extractor note at
# the top of this file a matcher that is signalled or cannot be exec'd must
# not read as "the directory is empty". An unmatched glob stays literal under
# bash's default nullglob and fails `[ -f ]`, so a missing directory and an
# empty one both land on 0 — the same answer for the operator either way.
rb_n=0
for rb_f in "$grok_home"/downloads/grok-*; do
  [ -f "$rb_f" ] && rb_n=$((rb_n + 1))
done
if [ -n "$upstream" ] && ver_gt "$upstream" "$want_ver"; then
  cat <<EOF
UPSTREAM MOVED: grok stable is $upstream; the fleet is pinned at $want_ver.
EOF
  if [ "$live_ver" = "$want_ver" ] && [ "$cask_installed" = "$want_ver" ]; then
    cat <<EOF

The pin is holding — nothing has changed on this machine. Lifting it is the
operator's call and is gated on a security re-audit of the NEW build, because
each item below was verified against $want_ver only and none of it is contractual:
EOF
  else
    cat <<EOF

THE PIN IS NOT HOLDING: this box runs ${live_ver:-an unreadable grok}, not $want_ver.

The failing version row above is that measurement, and this is no longer a
question of whether to LIFT the pin — the box is already off it. The choice is
to put $want_ver back, or to re-audit what is installed and move the pin
deliberately.
EOF
    if [ "$rb_n" -gt 0 ]; then
      rb_word=binaries
      [ "$rb_n" -eq 1 ] && rb_word=binary
      cat <<EOF

A MANAGED INSTALL EXISTS BESIDE THE CASK: $grok_home/downloads/ holds
$rb_n grok $rb_word, one file per version, so a rollback target is already on
disk and needs no re-fetch. Two cautions the cask route did not have. That
tree is what grok's own updater is allowed to replace, so whatever runs out of
it can self-update again — the pin's [cli] ceilings are the only thing holding
it. And \`which grok\` may still resolve to the cask, in which case swapping
the managed symlink changes nothing about what the fleet runs; the rows above
say which tree answers. The commands sit beside the lift runbook cited at the
end of this list, under the same heading.
EOF
    else
      cat <<EOF

THERE IS NO LOCAL ROLLBACK ARTIFACT: $grok_home/downloads/ holds no grok
binary, and on the cask route it does not exist at all — the Caskroom keeps
only the version that is installed, old cask versions are not installable
(there is no \`$want_cask@<version>\` cask and homebrew-cask is not cloned on
this box), so there is nothing here to swap to. Putting $want_ver back is a
RE-FETCH over the network, and that route is MEASURED rather than hoped for:
\`grok update --version $want_ver\` works from a cask-run binary, and
https://x.ai/cli/grok-$want_ver-macos-aarch64 still serves. What it does NOT
do is restore the cask — it fetches into $grok_home and recreates the managed
install, a second install brew does not track and which can self-update again.
Read the rollback section beside the lift runbook cited at the end of this
list before running it; the sha to verify against is in $pin.
EOF
    fi
    if [ -n "$live_ver" ] && [ -n "$cfg_req" ] && ver_gt "$live_ver" "$cfg_req"; then
      cat <<EOF

AND THE FLEET IS STOPPED RIGHT NOW: $live_ver is above the hard ceiling this
box declares (config required_maximum_version = $cfg_req), and grok refuses to
START above it — not just to update. Every dispatched grok pane and the
operator's own interactive grok is failing to launch until $want_ver is back.
That is the pin working as designed: the loud stop it was chosen over a silent
run of an un-re-audited build (rangerhq-iy3y).
EOF
    fi
    cat <<EOF

Each item below was verified against $want_ver only and none of it is
contractual, so none of it describes what this box is running:
EOF
  fi
  cat <<EOF

  1. Coding-data consent (rangerhq-sz7u, ranger-base-g4294, security). The
     sz7u defense is VOID from 1.0.52 on: consent is server-persisted. The
     x.ai/consent/record answer now goes out over HTTP ("/consent/accept"),
     and a second, account-level path sets retention server-side
     ("/privacy/coding-data-retention", field codingDataRetentionOptOut).
     Re-audit $upstream by grepping for those strings, not for the 1.0.5
     "not sent" message. Splash detection is now the ONLY layer between a
     dispatched Enter and [Opt in]. SEAT GATE at or above 1.0.52, and it is
     not met today: prove that a dispatched Enter cannot select [Opt in].
     That means detection holds AND the splash's default focus is not [Opt in],
     read from a pane capture with no key sent, on an account the operator
     designates. Until that is proven, no grok seat is declared —
     and no PID declares \`runtime: grok\` today, which is why moving this
     pin carries no live exposure either way.
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
  5. The Rex program_status precondition (ADR 0076 Amendment 3 D12/D13,
     ranger-base-1vgrw). \`[ui.notifications] program_status = "on"\` is what
     makes grok write its own OSC 7501 record on a \`multiplexer: rex\`
     instance, and grok will not confirm the key: \`grok inspect --json\`
     reports no value for it and grok's own validator calls the table an
     "unrecognized config key", while the TUI obeys it. So the measurement is
     all there is, and it was taken on $want_ver only — which is why the grok
     built-in carries \`rex_verified: grok $want_ver (<build hash>)\` and
     refuses a grok on any other release BEFORE the claim. Re-measure
     $upstream with \`sh docs/adr/0076-grok-hook.probe.sh auto on off onidle\`
     (about two minutes, no model turn) and move \`rex_verified:\`, or grok
     stops launching on every Rex instance the moment the pin moves.

  Runbook to lift the pin: docs/notes.d/notes-grok.md, "grok substrate".
EOF
elif [ -n "$upstream" ]; then
  echo "tap version: $upstream (== the pin; nothing to re-audit)"
fi

echo
if [ "$fail" -ne 0 ]; then
  echo "verify-grok-pin: $fail check(s) FAILED — the pin is not what $pin says"
  exit 1
fi
echo "verify-grok-pin: pin intact at $want_ver"
