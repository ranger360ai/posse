#!/usr/bin/env bash
# herdr [[startup]] hook — arm the fleet's dispatch loop (rangerhq-snd).
#
# The runner for scheduled dispatch is a herdr workspace running
# `posse dispatch --watch`: the loop lives where the operator can see it
# (cockpit row, `posse peek`, one `x` to kill), it cannot start in a world
# with no herdr socket, and it dies when herdr does — which is the right
# lifetime for a fleet. This hook is only the arming step: herdr runs it
# once per server start, so "survives reboot" means "comes back when herdr
# does".
#
# DISARMED unless the instance config says otherwise. Keys in
# $RHQ_HOME/config.yaml (flat-YAML scalars, house subset):
#
#   autostart_interval:      base pass interval (30s, 5m, or bare seconds)
#                            — PRESENCE OF THIS KEY IS THE ARM SWITCH
#   autostart_max_interval:  backoff cap for quiet passes (default: posse's 8x)
#   autostart_max_beads:     -n, launch attempts per pass — RAISES OR LOWERS
#                            A CAP THAT IS ALWAYS PRESENT (default: 3).
#                            0 means unbounded, and only ever by saying so:
#                            an armed loop must never fire the whole ready
#                            queue in one pass by omission (rangerhq-v83)
#   autostart_dry_run:       true → passes route and report, dispatch nothing
#   autostart_resume:        false → the loop only WARNS about a bead whose
#                            persona settled without closing it. DEFAULTS ON,
#                            unlike every other key here, because the warning
#                            ("◑ … settled but open — review") is addressed to
#                            an operator who is by definition not watching this
#                            loop: three measured sessions in a row went idle
#                            on finished work and the beads sat open until a
#                            human re-prompted by hand (ranger-base-f0g).
#                            Only bd-ready beads are reachable, so a persona
#                            that filed a question and depended its bead on it
#                            is left alone; one that settles open with nothing
#                            filed is re-prompted every pass until it closes
#                            or somebody looks. Set false to get the warning
#                            back and nothing else.
#   autostart_session:       session name (default: dispatch)
#   autostart_dir:           session cwd (default: $HOME)
#
# Called with --startup by the manifest: a session of this name at server
# start is usually a husk herdr restored from its last layout (the workspace
# comes back, the command does not), and is replaced. Usually, not always —
# herdr runs [[startup]] hooks on a LIVE HANDOFF too (`herdr update
# --handoff`), where the workspace comes back *with* its command still
# running, same pid, PTY fd passed across. So the hook asks the loop, not
# the workspace: `posse dispatch --watch` holds flock(2) on
# $RHQ_HOME/state/dispatch-watch.lock for its whole life and
# `posse dispatch --watch-status` reports it, so a husk's lock is free and a
# carry-over's is held — kernel-owned, with no stale state to reason about
# (rangerhq-ct9, rangerhq-gir5). Kill-and-replace only when no live loop
# answers. Run by hand without the flag it is conservative and leaves a live
# session alone either way.
#
# The plan-utilization guard (plan_guard_5h / plan_guard_7d) is what keeps an
# unattended loop off the operator's plan window; dispatch runs it at the top
# of every pass. Arming without it is arming a token loop nobody is watching.
# This loop is *the* unattended one, so the guard fails closed here after
# plan_guard_blind_max: (10m default, 0 = never) without a successful
# reading — the passes park, the beads stay ready, and the first good
# reading resumes them (rangerhq-6h1).
#
# Log: $RHQ_HOME/state/dispatch-watch.log (tee'd from the pane; state/ is
# machine-local and gitignored). The pane's own scrollback is the live view.
# Beside it, dispatch-watch.lock — held by the loop, released by the kernel
# when it dies — and dispatch-watch.pid, which says which pid and since
# when. The lock is the evidence; the pidfile is the name on it.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# ./bin/posse is the symlink `make link-plugin` points at the *promoted*
# binary — never a persona's unfinished `make build` (ORDERS: rangerhq-8te).
RHQ=${RHQ_BIN:-$here/bin/posse}
RHQ_HOME=${RHQ_HOME:-$HOME/.config/rhq}
CONFIG=$RHQ_HOME/config.yaml
LOG=$RHQ_HOME/state/dispatch-watch.log
MAXLOG=${AUTOSTART_MAXLOG:-5242880} # 5 MiB, then one .1 generation

startup=false
[ "${1:-}" = "--startup" ] && startup=true

say() { echo "dispatch autostart: $*"; }

# The plugin registry is global, so herdr runs this hook for named session
# servers too. Only the default server owns the fleet queue and its one
# dispatch-watch.pid. A scratch/named server inheriting the fleet RHQ_HOME
# must fail closed before it reads that pidfile or invokes posse
# (ranger-base-87q).
#
# HERDR_SOCKET_PATH is authoritative when present, matching herdr itself. If
# it is absent, HERDR_SESSION still proves this is a named server. The fixed
# default socket layout is measured and shared with herdrSocketPath in
# internal/rhq/herdrback.go; herdr has no config-dir override.
if $startup; then
	default_socket=$HOME/.config/herdr/herdr.sock
	case "${HERDR_SOCKET_PATH:-}" in
	'')
		if [ -n "${HERDR_SESSION:-}" ]; then
			say "not the default herdr server (HERDR_SESSION=$HERDR_SESSION) — not arming the fleet loop"
			exit 0
		fi
		;;
	"$default_socket") ;;
	*)
		say "not the default herdr server (HERDR_SOCKET_PATH=$HERDR_SOCKET_PATH) — not arming the fleet loop"
		exit 0
		;;
	esac
fi

# Is a dispatch loop actually running? The loop holds flock(2) on
# $RHQ_HOME/state/dispatch-watch.lock for its whole life, so this asks the
# kernel: held means running, free means not, and process death releases the
# lock — crash, kill -9, closed pane alike. There is no stale state to
# detect and nothing here to infer (rangerhq-gir5, ADR 0011 §1).
#
# What that retires: `kill -0` on the pidfile plus a grep of `ps -o
# command=`. Reconstructing liveness from a file whose truth decays needed
# three patches and still leaked — a recycled pid read as alive, a one-shot
# `posse dispatch --persona` whose argv merely contains the word read as the
# watch loop (rangerhq-ppy9), and a `ps` that could not answer read as
# alive or dead depending on which arm you wrote (rangerhq-mugy,
# ranger-base-rmc). The pidfile stays for the identity half — which pid,
# since when — which is what the probe quotes once the lock has answered.
#
# flock(1) is not the probe: it is util-linux and absent on macOS, where
# this hook runs. posse asks the kernel itself and reports on one line.
#
# THE LINE IS THE CONTRACT, not the exit status. A posse too old to know the
# subcommand fails its flag parse and prints nothing matching, which is the
# same "could not ask" as a state dir it cannot open — and an unanswerable
# probe stands the hook down rather than replacing a loop it cannot see.
# Unarmed is visible and recoverable; double dispatch is neither
# (rangerhq-ct9/mugy).
#
# stdout is the answer and stderr is never folded into it. posse writes
# unrelated notices there — the config-home transition notice fires on every
# invocation of an instance that still has only the old home — and a line
# glued in front of the answer would read as "could not ask" and stand the
# hook down for good. stderr is kept, but only to quote when the answer did
# not arrive.
loopstate=
loopwho=
loopsaid=
loopwhy=
loop_alive() {
	local errf
	errf=$(mktemp 2>/dev/null) || errf=
	loopsaid=$("$RHQ" dispatch --watch-status 2>"${errf:-/dev/null}")
	if [ -n "$errf" ]; then
		loopwhy=$(cat "$errf")
		rm -f "$errf"
	fi
	case "$loopsaid" in
	"watch-loop: running"*)
		loopstate=running
		loopwho=${loopsaid#watch-loop: running}
		return 0
		;;
	"watch-loop: none"*)
		loopstate=none
		return 1
		;;
	esac
	loopstate=unknown
	return 0
}

# Flat-YAML scalar read: `key: value`, trailing " #" comment stripped.
cfg() {
	[ -f "$CONFIG" ] || return 0
	sed -n "s/^$1:[[:space:]]*//p" "$CONFIG" | head -1 |
		sed -e 's/[[:space:]]*#.*$//' -e 's/[[:space:]]*$//'
}

interval=$(cfg autostart_interval)
if [ -z "$interval" ]; then
	say "disarmed (no autostart_interval: in $CONFIG)"
	exit 0
fi
# The transition alias (rangerhq-tyay). `make link-plugin` writes both names,
# but an operator who ran `make install` and stopped has a plugin dir holding
# only the old one — and the cost of being strict here is that nothing gets
# dispatched until somebody notices. So fall back, and SAY so: this arm exists
# to be seen and removed, not to make the two names interchangeable.
if [ ! -x "$RHQ" ] && [ -z "${RHQ_BIN:-}" ] && [ -x "$here/bin/rhq" ]; then
	say "no $RHQ — arming through the transition alias bin/rhq; run 'make link-plugin'" >&2
	RHQ=$here/bin/rhq
fi
if [ ! -x "$RHQ" ]; then
	say "no posse at $RHQ — run 'make link-plugin'" >&2
	exit 1
fi

session=$(cfg autostart_session); session=${session:-dispatch}
maxint=$(cfg autostart_max_interval)
maxbeads=$(cfg autostart_max_beads)
# The cap is always passed. Absent key → 3; a malformed value would reach
# `-n` as Atoi's 0 — unbounded, the one thing this default exists to
# prevent — so it is named on stderr and replaced, not silently obeyed.
case "$maxbeads" in
'') maxbeads=3 ;;
*[!0-9]*) say "autostart_max_beads: '$maxbeads' is not a count — using 3" >&2; maxbeads=3 ;;
esac
dry=$(cfg autostart_dry_run)
# Inverted default: absent key → --resume. See the header — a settled-but-open
# bead's only other outcome under an unattended loop is sitting there. A value
# that is not a boolean is named and replaced rather than read as "off": the
# one direction a typo must not silently take this is back to the broken shape.
resume=$(cfg autostart_resume)
case "$resume" in
'' | true | yes | 1) resume=true ;;
false | no | 0) resume=false ;;
*) say "autostart_resume: '$resume' is not true/false — using true" >&2; resume=true ;;
esac
dir=$(cfg autostart_dir); dir=${dir:-$HOME}

watch="$RHQ dispatch --watch $interval"
[ -n "$maxint" ] && watch="$watch --max-interval $maxint"
watch="$watch -n $maxbeads"
[ "$resume" = true ] && watch="$watch --resume"
case "$dry" in true|yes|1) watch="$watch --dry-run" ;; esac

mkdir -p "$(dirname "$LOG")"
if [ -f "$LOG" ] && [ "$(wc -c <"$LOG")" -gt "$MAXLOG" ]; then
	mv -f "$LOG" "$LOG.1"
fi
banner="printf '\\n== dispatch --watch armed %s ==\\n' \"\$(date '+%Y-%m-%d %H:%M:%S')\" | tee -a $LOG"
cmd="$banner; $watch 2>&1 | tee -a $LOG"

start() {
	if out=$("$RHQ" new "$session" --dir "$dir" --emoji 🛰️ --cmd "$cmd" 2>&1); then
		say "$session started — $watch"
		say "log: $LOG"
		return 0
	fi
	case "$out" in
	*"already exists"*) return 3 ;;
	*) echo "$out" >&2; return 1 ;;
	esac
}

# A live loop is never replaced, whatever the workspace looks like and
# whatever it is called — one loop per queue is the invariant, and a handoff
# carry-over is a live loop that merely *looks* restored (rangerhq-ct9).
if loop_alive; then
	# A failed scan says so, the same way a gated pass does (rangerhq-llse):
	# standing down because the probe could not answer is a different fact
	# from standing down on a confirmed loop, and only one of them wants
	# looking at.
	if [ "$loopstate" = running ]; then
		say "loop already running$loopwho — left alone"
	else
		say "posse could not answer whether a dispatch loop is running — left alone (one loop per queue)" >&2
		say "it said: ${loopsaid:-nothing} ${loopwhy:+(stderr: $loopwhy)}" >&2
		say "if this posse predates 'dispatch --watch-status', run 'make link-plugin'" >&2
	fi
	exit 0
fi

# No live loop. `posse new` refuses a name that already resolves to a live
# workspace, which is the idempotency we want by hand: a second run reports
# rather than starting a second loop double-dispatching the same queue. At
# server start the same refusal means the workspace came back from herdr's
# saved layout without its command — a husk, and a husk holds nothing worth
# keeping.
#
# A loop started by a posse old enough not to hold the watch lock reads as
# dead here and is replaced once at the next server start; after that every
# loop holds one. Both halves come from the same promoted ./bin/posse, so the
# window is the one start that follows an upgrade.
start; rc=$?
if [ "$rc" = 3 ]; then
	if $startup; then
		say "$session restored by herdr without its loop — replacing"
		"$RHQ" kill "$session" >/dev/null 2>&1 || true
		start; rc=$?
		[ "$rc" = 3 ] && { say "$session still present after kill — not started" >&2; rc=1; }
	else
		say "$session already running — left alone"
		rc=0
	fi
fi
exit "$rc"
