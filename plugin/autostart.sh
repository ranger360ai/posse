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
# the workspace: `posse dispatch --watch` stamps $RHQ_HOME/state/dispatch-watch.pid
# and a husk's stamp is stale. Kill-and-replace only when no live loop
# answers (rangerhq-ct9). Run by hand without the flag it is conservative
# and leaves a live session alone either way.
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
# Beside it, dispatch-watch.pid — written by the loop, removed when it ends
# cleanly, left stale when its pane is killed.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# ./bin/posse is the symlink `make link-plugin` points at the *promoted*
# binary — never a persona's unfinished `make build` (ORDERS: rangerhq-8te).
RHQ=${RHQ_BIN:-$here/bin/posse}
RHQ_HOME=${RHQ_HOME:-$HOME/.config/rhq}
CONFIG=$RHQ_HOME/config.yaml
LOG=$RHQ_HOME/state/dispatch-watch.log
PIDFILE=$RHQ_HOME/state/dispatch-watch.pid
MAXLOG=${AUTOSTART_MAXLOG:-5242880} # 5 MiB, then one .1 generation

startup=false
[ "${1:-}" = "--startup" ] && startup=true

say() { echo "dispatch autostart: $*"; }

# Is a dispatch loop actually running? Existence of the pidfile proves
# nothing — a loop killed with its pane never gets to remove it — so this
# asks the process. Signal 0 is the question, not an action.
livepid=
argvchecked=
loop_alive() {
	local pid argv
	[ -f "$PIDFILE" ] || return 1
	pid=$(sed -n 's/^pid:[[:space:]]*//p' "$PIDFILE" | head -1)
	case "$pid" in '' | *[!0-9]*) return 1 ;; esac
	kill -0 "$pid" 2>/dev/null || return 1
	livepid=$pid
	# Pid reuse: a stale file whose number the kernel handed to something
	# else would read as live and leave the fleet silently unarmed. A watch
	# loop's argv always carries the subcommand.
	#
	# The argv probe can only REFUTE liveness, never assert death: `ps` is
	# not on every box this hook runs on (a slim image, a sandbox that
	# denies process inspection, a restricted PATH), and a probe that could
	# not run answers nothing. Reading that silence as "no loop" is
	# fail-open on the invariant the pidfile exists to hold — it kills the
	# workspace and starts a SECOND loop against the queue the first one is
	# still claiming beads from (rangerhq-ct9). An alive pid we cannot
	# identify stays alive: unarmed is visible and recoverable, double
	# dispatch is neither (rangerhq-mugy).
	if ! argv=$(ps -p "$pid" -o command= 2>/dev/null) || [ -z "$argv" ]; then
		return 0
	fi
	argvchecked=yes
	case "$argv" in
	*dispatch*) return 0 ;;
	esac
	livepid=
	return 1
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
	# standing down on an unidentifiable pid is a different fact from
	# standing down on a confirmed loop, and only one of them wants looking
	# at.
	if [ -n "$argvchecked" ]; then
		say "loop already running (pid $livepid) — left alone"
	else
		say "pid $livepid is alive but ps could not identify it — left alone (one loop per queue)" >&2
		say "if no dispatch loop is running, remove $PIDFILE and restart" >&2
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
# A loop started by a posse old enough not to stamp the pidfile reads as dead
# here and is replaced once at the next server start; after that every loop
# leaves a record.
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
