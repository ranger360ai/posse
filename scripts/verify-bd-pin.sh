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
#   2. `bd` on PATH resolves to the pinned binary — or, inside a persona
#      session, to a posse gate shim (internal/rhq/gates.go) whose own exec
#      line targets it. The gate shim dir leads every persona's PATH ahead
#      of ~/.local/bin by design (ADR 0009 §1), so row 2 comparing raw paths
#      failed in EVERY persona session on a box whose pin was intact
#      (ranger-base-43v1) — the shim is recognised by the header renderShim
#      stamps on it, and what it actually execs is read out of the shim
#      file itself, not re-derived from today's PATH.
#   3. homebrew's beads keg is unlinked, or brew-pinned. Either holds; LINKED
#      is the 08-16 outage re-armed.
#   4. every live `bd daemon` process is running the pinned binary, and started
#      AFTER that binary was written. A daemon older than its own binary is by
#      definition running an artifact that no longer exists on disk — the exact
#      shape the 08-16 rollback missed.
#   5. every live `bd daemon` process still has a WORKING DIRECTORY, and it is
#      not a throwaway one (ranger-base-42mv). bd auto-starts a daemon on first
#      use against a database and nothing ever stops it, so any bd call in a
#      temp dir — a test fixture, a session scratchpad — leaves a process
#      holding a sqlite handle to a database nobody can reach any more. Ten
#      accumulated over twelve days that way, two of them in one evening once a
#      live test started exercising bd. Same failure class as 4, one level
#      down: 4 is a process whose BINARY is gone, 5 is one whose DIRECTORY is.
#      Classification is by cwd and by cwd alone, which is what makes it safe:
#      the canonical queue's daemon sits in a real repo and is never named, and
#      the remedy is never `bd daemon stop-all`, which would take it with it.
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
leaked=

command -v bd >/dev/null || { echo "verify-bd-pin: bd not on PATH"; exit 2; }
[ -r "$pin" ] || { echo "verify-bd-pin: missing $pin"; exit 2; }

val() { sed -n "s/^$1 *= *\"\{0,1\}\([^\"#]*\)\"\{0,1\}.*/\1/p" "$2" | head -1 | tr -d ' '; }

want_ver=$(val posse_pinned_version "$pin")
want_bin=$(val pinned_binary "$pin")
formula=$(val formula "$pin")
case $want_bin in "~"/*) want_bin=$HOME${want_bin#\~} ;; esac

# `--no-daemon` so the check itself never spawns the thing it is checking. The
# daemon it spawns went at 0.50.0 and the flag outlived it as a deprecated
# no-op until 0.51.0 deleted that too (ranger-base-db04), so the flag is
# accepted by 0.49.x and 0.50.x and rejected by an unpinned 1.x on PATH — fall
# back, and let the version row fail.
live_ver=$(bd --no-daemon version 2>/dev/null || bd version 2>/dev/null)
live_ver=$(printf '%s' "$live_ver" | tr ' ' '\n' | grep -m1 -E '^[0-9]+\.[0-9]+\.[0-9]+$')
live_bin=$(command -v bd)

chk_row() { # label want got
	if [ "$2" = "$3" ]; then printf '  %-24s %-34s ok\n' "$1" "$3"
	else printf '  %-24s %-34s <-- FAIL (want %s)\n' "$1" "${3:-?}" "$2"; fail=$((fail + 1)); fi
}

# A posse gate shim (internal/rhq/gates.go renderShim) is the header it
# stamps on itself, not the path it lives at — a persona name in the path
# would need reproducing here and drift the day gates move. rangerhq-9ha
# is the fixed half of that header, present regardless of persona.
is_gate_shim() {
	head -2 "$1" 2>/dev/null | grep -q 'posse gate for .*rangerhq-9ha'
}

# What the shim actually execs, read out of its OWN last line — the target
# renderShim froze in at render time — never re-derived from today's PATH,
# which can drift out from under a shim nobody re-rendered since.
shim_target() {
	sed -n "s/^exec '\\(.*\\)' \"\\\$@\"\$/\\1/p" "$1" | head -1
}

echo "bd version pin — $pin"
chk_row "bd version" "$want_ver" "$live_ver"
if is_gate_shim "$live_bin"; then
	gate_target=$(shim_target "$live_bin")
	case $gate_target in
	"$want_bin")
		printf '  %-24s %-34s GATED (execs %s)\n' "command -v bd" "$live_bin" "$want_bin"
		;;
	"")
		printf '  %-24s %-34s <-- FAIL (gate shim, exec target unparseable)\n' "command -v bd" "$live_bin"
		fail=$((fail + 1))
		;;
	*)
		printf '  %-24s %-34s <-- FAIL (gate shim execs %s, want %s)\n' "command -v bd" "$live_bin" "$gate_target" "$want_bin"
		fail=$((fail + 1))
		;;
	esac
else
	chk_row "command -v bd" "$want_bin" "$live_bin"
fi

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

# A BSD/GNU `||` chain only discriminates if the WRONG arm FAILS, and
# `stat -f` does not fail on GNU: `-f` is a FORMAT flag on BSD but
# DISPLAY-FILESYSTEM-STATUS on GNU, where it takes no format, so
# `stat -f %m FILE` reads `%m` and FILE as two operands, prints FILE's
# multi-line filesystem block on STDOUT, and only then exits non-zero on the
# missing `%m` — so the fallback ran too and appended the real epoch.
# bin_mtime became that blob plus a number: `-lt` errored, the STALE arm went
# false, and so did the "age unverified" arm below it because bin_mtime was
# non-empty. Linux printed `ok` for a daemon it never checked, which is the
# 08-16 command-layer-only verdict reintroduced on the other platform
# (ranger-base-tssy). GNU first, because BSD stat rejects `-c` outright and so
# that order does discriminate; and every epoch is digit-checked on the way
# out, so a probe that comes back wrong on some third stat lands in "age
# unverified" — the honest arm — instead of `ok`.
epoch() { # stdin -> epoch seconds, empty unless it is all digits
	local v
	v=$(cat)
	case $v in '' | *[!0-9]*) return 0 ;; esac
	printf '%s' "$v"
}

file_mtime() { # path -> epoch seconds, empty when unreadable
	{ stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null || true; } | epoch
}

bin_mtime=$(file_mtime "$want_bin")

proc_start() { # pid -> epoch seconds, empty when unparseable
	local ls
	ls=$(ps -wwo lstart= -p "$1" 2>/dev/null | sed 's/[[:space:]]*$//')
	[ -n "$ls" ] || return 0
	{
		date -j -f '%a %b %e %H:%M:%S %Y' "$ls" +%s 2>/dev/null ||
			date -d "$ls" +%s 2>/dev/null || true
	} | epoch
}

proc_cwd() { # pid -> working directory, empty when unreadable
	# `lsof` first and PATH-resolved, so the arm is chosen by a `command -v`
	# — a file test on a name, not an exit status — and the wrong arm is
	# never entered at all (the BSD/GNU `stat` lesson, ranger-base-tssy).
	# `-Fn` is lsof's machine-readable form: one `n<path>` line, no columns
	# to lose a path with a space in it to.
	if command -v lsof >/dev/null 2>&1; then
		lsof -p "$1" -a -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' | head -1
	elif [ -r "/proc/$1/cwd" ]; then
		readlink "/proc/$1/cwd" 2>/dev/null
	fi
}

# `ps -o comm=` reports the path AS INVOKED, not a resolved one, so a daemon
# started with a relative argv0 reports a RELATIVE path — and this script has
# already cd'd to the repo root, so testing that with `-e` asks whether the
# REPO holds a file by that name. It does not, and a binary that is on disk
# got the ORPHAN verdict and with it the wrong runbook (ranger-base-zk8v). A
# relative comm means something only against the PROCESS's own cwd, which the
# cwd layer reads anyway — so read it first and resolve against it, and when
# there is no cwd to resolve against say so rather than guess. Only the common
# `./x` form is tidied: a `../` comm still joins to a path `-e` answers
# correctly, and simply will not string-equal $want_bin, which fails toward
# FOREIGN — the safe direction, since the alarm still fires.
resolve_comm() { # comm cwd -> path to test, empty when it cannot be resolved
	case $1 in
	'') : ;;
	/*) printf '%s' "$1" ;;
	*) [ -z "$2" ] || printf '%s/%s' "${2%/}" "${1#./}" ;;
	esac
	return 0
}

# The throwaway roots a daemon has no business living under. macOS resolves
# both /tmp and $TMPDIR through /private, and lsof reports the resolved form,
# so both spellings are listed rather than resolved at read time.
is_ephemeral() { # path -> 0 when it is under a temp root
	case $1 in
	/tmp/* | /private/tmp/* | /var/tmp/* | /private/var/tmp/* | /var/folders/* | /private/var/folders/*) return 0 ;;
	esac
	case ${TMPDIR:-} in
	'' | /) return 1 ;;
	esac
	case $1 in "${TMPDIR%/}"/*) return 0 ;; esac
	return 1
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
		# The cwd is read HERE, ahead of the binary layer, because a relative
		# comm can only be resolved against it. The two layers stay REPORTED
		# separately below; only the reading order moved.
		cwd=$(proc_cwd "$pid")
		real=$(resolve_comm "$path" "$cwd")
		verdict=ok
		if [ -z "$path" ] || { [ -n "$real" ] && [ ! -e "$real" ]; }; then
			# macOS empties `comm` once the executable is unlinked; either way
			# the binary this process is running is not on disk any more.
			verdict="<-- ORPHAN: its binary is gone from disk"
			fail=$((fail + 1))
			path=${path:-"(unlinked — no path)"}
		elif [ -z "$real" ]; then
			# Relative comm, unreadable cwd: nothing to resolve it against, so
			# its existence is unknowable. What still holds is that a relative
			# path is not the absolute pinned binary — the honest half.
			verdict="<-- FOREIGN: not the pinned binary (relative path, cwd unreadable — cannot tell whether it is still on disk)"
			fail=$((fail + 1))
		elif [ "$real" != "$want_bin" ]; then
			verdict="<-- FOREIGN: not the pinned binary"
			fail=$((fail + 1))
		elif [ -n "$start" ] && [ -n "$bin_mtime" ] && [ "$start" -lt "$bin_mtime" ]; then
			verdict="<-- STALE: started before the pinned binary was written"
			fail=$((fail + 1))
		elif [ -z "$start" ] || [ -z "$bin_mtime" ]; then
			verdict="(age unverified — could not read start time or binary mtime)"
		fi
		# The cwd layer is REPORTED SEPARATELY from the binary layer above,
		# never folded into it: a daemon can be running the right binary in a
		# directory that no longer exists, and collapsing the two verdicts
		# into one would print `ok` for exactly that process.
		if [ -z "$cwd" ]; then
			cwd_verdict="(working directory unverified — could not read it)"
			cwd="?"
		elif [ ! -d "$cwd" ]; then
			cwd_verdict="<-- LEAKED: its working directory is gone"
			leaked="$leaked $pid"
			fail=$((fail + 1))
		elif is_ephemeral "$cwd"; then
			cwd_verdict="<-- EPHEMERAL: a throwaway directory, so nothing will ever reach this database again"
			leaked="$leaked $pid"
			fail=$((fail + 1))
		else
			cwd_verdict=ok
		fi
		printf '  pid %-7s age %-8s %s\n' "$pid" "${when:-?}" "$path"
		printf '    binary %s\n' "$verdict"
		printf '    cwd    %s  %s\n' "$cwd" "$cwd_verdict"
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
	if [ -n "$leaked" ]; then
		cat <<EOF

For the LEAKED/EPHEMERAL rows there is nothing to restart — their databases
are unreachable, so the reap is the whole remedy. SIGTERM only, one pid at a
time (it flushes the WAL), and NEVER \`bd daemon stop-all\`, which would take
the canonical queue's daemon with it:
   kill -TERM$leaked
EOF
	fi
	exit 1
fi
echo "verify-bd-pin: pin intact at $want_ver — command layer and process layer agree"
