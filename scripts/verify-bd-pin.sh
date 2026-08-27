#!/usr/bin/env bash
# Verify the fleet's bd version pin (rangerhq-f49) against the live box — at
# the COMMAND layer and at the PROCESS layer.
#
# The pin has taken the fleet down twice, both times through the same door.
# bd 0.49.1 auto-spawns a per-repo daemon on any call; that daemon keeps
# running when its binary is replaced or deleted underneath it. `brew upgrade
# beads` on 2026-08-16 did exactly that, and the 08-16 rollback checked only
# `bd version` / `bd ready` / `dispatch --dry-run` — all green, all at the
# command layer — while the orphan from the reverted artifact kept running for
# 12d21h and then degraded every bd write on the box for ~40 minutes on 08-26.
#
# So this asserts four things (etc/bd/version-pin.toml is the declaration):
#   1. `bd version` is the pinned version, exactly.
#   2. `bd` on PATH resolves to the pinned binary. /opt/homebrew/bin precedes
#      ~/.local/bin, so anything linked in front of the pin wins silently.
#   3. homebrew's beads keg is unlinked, or brew-pinned. Either holds; LINKED
#      is the 08-16 outage re-armed.
#   4. every live `bd daemon` process is running the pinned binary, and started
#      AFTER that binary was written. A daemon older than its own binary is by
#      definition running an artifact that no longer exists on disk — the exact
#      shape the 08-16 rollback missed.
#
# READ-ONLY, AND IT CANNOT ACT. It kills nothing and starts nothing:
# `Bash(bd daemon:*)` is denied fleet-wide and killing a pid is in no PID.
# Remediation is the operator's hand; this prints the commands and stops.
# A detector that cannot act still turns 12d21h of not looking into one
# dispatch pass, which is the number this incident actually cost.
set -uo pipefail

cd "$(dirname "$0")/.."
pin=etc/bd/version-pin.toml
fail=0

command -v bd >/dev/null || { echo "verify-bd-pin: bd not on PATH"; exit 2; }
[ -r "$pin" ] || { echo "verify-bd-pin: missing $pin"; exit 2; }

val() { sed -n "s/^$1 *= *\"\{0,1\}\([^\"#]*\)\"\{0,1\}.*/\1/p" "$2" | head -1 | tr -d ' '; }

want_ver=$(val posse_pinned_version "$pin")
want_bin=$(val pinned_binary "$pin")
formula=$(val formula "$pin")
case $want_bin in "~"/*) want_bin=$HOME${want_bin#\~} ;; esac

# `--no-daemon` so the check itself never spawns the thing it is checking. It
# is a 0.49.x flag (0.51.0 deleted the daemon and this flag with it), so an
# unpinned 1.x on PATH rejects it — fall back, and let the version row fail.
live_ver=$(bd --no-daemon version 2>/dev/null || bd version 2>/dev/null)
live_ver=$(printf '%s' "$live_ver" | tr ' ' '\n' | grep -m1 -E '^[0-9]+\.[0-9]+\.[0-9]+$')
live_bin=$(command -v bd)

chk_row() { # label want got
	if [ "$2" = "$3" ]; then printf '  %-24s %-34s ok\n' "$1" "$3"
	else printf '  %-24s %-34s <-- FAIL (want %s)\n' "$1" "${3:-?}" "$2"; fail=$((fail + 1)); fi
}

echo "bd version pin — $pin"
chk_row "bd version" "$want_ver" "$live_ver"
chk_row "command -v bd" "$want_bin" "$live_bin"

# ------------------------------------------------------- homebrew keg state
# brew's own answer, not our reading of a symlink. Compact JSON either way, but
# parse it properly — verify-grok-pin's sed cannot survive pretty-printing
# (ranger-base-ocfh) and that defect is not worth reproducing here.
keg_state=?; keg_ver=
if command -v brew >/dev/null && command -v python3 >/dev/null; then
	keg=$(brew info --json=v2 "$formula" 2>/dev/null | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    print("?\t"); raise SystemExit
f = (d.get("formulae") or [None])[0]
if not f or not f.get("installed"):
    print("not-installed\t"); raise SystemExit
ver = (f["installed"][-1] or {}).get("version", "?")
if f.get("pinned"):
    print("pinned\t" + ver)
elif f.get("linked_keg"):
    print("LINKED\t" + str(f["linked_keg"]))
else:
    print("unlinked\t" + ver)
' 2>/dev/null)
	keg_state=${keg%%	*}; keg_ver=${keg#*	}
elif ! command -v brew >/dev/null; then
	keg_state=no-brew
fi

case $keg_state in
unlinked | pinned | not-installed | no-brew)
	printf '  %-24s %-34s ok\n' "homebrew $formula" "$keg_state${keg_ver:+ $keg_ver}"
	;;
LINKED)
	printf '  %-24s %-34s <-- FAIL (want unlinked or brew-pinned)\n' "homebrew $formula" "LINKED $keg_ver"
	fail=$((fail + 1))
	;;
*)
	printf '  %-24s %-34s (cannot determine — no brew/python3 answer)\n' "homebrew $formula" "—"
	;;
esac

# ------------------------------------------------------------ process layer
bin_mtime=$(stat -f %m "$want_bin" 2>/dev/null || stat -c %Y "$want_bin" 2>/dev/null)

proc_start() { # pid -> epoch seconds, empty when unparseable
	local ls
	ls=$(ps -wwo lstart= -p "$1" 2>/dev/null | sed 's/[[:space:]]*$//')
	[ -n "$ls" ] || return 0
	date -j -f '%a %b %e %H:%M:%S %Y' "$ls" +%s 2>/dev/null ||
		date -d "$ls" +%s 2>/dev/null || true
}

age() { # epoch -> "12d21h"
	local s=$(( $(date +%s) - $1 ))
	[ "$s" -lt 0 ] && s=0
	printf '%dd%dh' $((s / 86400)) $(((s % 86400) / 3600))
}

# argv[0] basename `bd` with argv[1] `daemon` — not a substring match, so this
# script's own command line and any grep for it are not daemons.
pids=$(ps -Awwo pid=,args= 2>/dev/null |
	awk '{ c = $2; sub(/.*\//, "", c); if (c == "bd" && $3 == "daemon") print $1 }')

echo
echo "live bd daemons — the layer the 08-16 rollback never checked"
if [ -z "$pids" ]; then
	echo "  none running"
else
	for pid in $pids; do
		path=$(ps -wwo comm= -p "$pid" 2>/dev/null | sed 's/[[:space:]]*$//')
		start=$(proc_start "$pid")
		when=${start:+$(age "$start")}
		verdict=ok
		if [ -z "$path" ] || [ ! -e "$path" ]; then
			# macOS empties `comm` once the executable is unlinked; either way
			# the binary this process is running is not on disk any more.
			verdict="<-- ORPHAN: its binary is gone from disk"
			fail=$((fail + 1))
			path=${path:-"(unlinked — no path)"}
		elif [ "$path" != "$want_bin" ]; then
			verdict="<-- FOREIGN: not the pinned binary"
			fail=$((fail + 1))
		elif [ -n "$start" ] && [ -n "$bin_mtime" ] && [ "$start" -lt "$bin_mtime" ]; then
			verdict="<-- STALE: started before the pinned binary was written"
			fail=$((fail + 1))
		elif [ -z "$start" ] || [ -z "$bin_mtime" ]; then
			verdict="(age unverified — could not read start time or binary mtime)"
		fi
		printf '  pid %-7s age %-8s %s\n' "$pid" "${when:-?}" "$path"
		printf '    %s\n' "$verdict"
	done
fi

echo
if [ "$keg_state" = unlinked ]; then
	cat <<EOF
NOT BELT-AND-BRACES: homebrew $formula $keg_ver is installed and UNLINKED, but
not brew-pinned. \`brew upgrade\`, \`brew link $formula\`, or a reinstall of the
formula that depends on it relinks the keg — and /opt/homebrew/bin precedes
$(dirname "$want_bin") on PATH, so 1.x would win silently. The pin holds today
by the absence of a symlink, not by policy.

The belt is the operator's hand, not this lane's:
    brew pin $formula
  or install the operator's own formula: brew install davidstacy/local/$formula@$want_ver

EOF
fi

if [ "$fail" -ne 0 ]; then
	cat <<EOF
verify-bd-pin: $fail check(s) FAILED — the live box is not what $pin says.

REMEDIATION IS THE OPERATOR'S. This script kills nothing and starts nothing:
\`Bash(bd daemon:*)\` is denied fleet-wide. For a flagged daemon, the runbook is
NOTES.md "beads (bd) substrate" — freeze writers, \`kill -TERM <pid>\`, then
re-run this and confirm the replacement daemon is on $want_bin.
EOF
	exit 1
fi
echo "verify-bd-pin: pin intact at $want_ver — command layer and process layer agree"
