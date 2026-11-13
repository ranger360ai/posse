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
#   autostart_via:           herdr | systemd — WHICH MECHANISM OWNS THIS
#                            HOME'S LOOP (ADR 0080 D9). Default herdr, which
#                            is this hook. On `systemd` this hook stands
#                            down BY NAME and arms nothing: the loop is a
#                            systemd user unit rendered by `posse systemd`,
#                            and both mechanisms arming is not two loops but
#                            a race the unit loses — a dispatch --watch that
#                            cannot take state/dispatch-watch.lock exits
#                            non-zero, and herdr's server is up before the
#                            unit that is After= it, so a linked hook that
#                            found an armed home would take the lock and the
#                            unit would fail on every start. Absent, bare,
#                            null and ~ all read as herdr (the key has a
#                            default, unlike the arm switch below); a value
#                            that is NEITHER NAME is refused by name, because
#                            reading an owner posse cannot parse as the
#                            default is exactly how that race gets armed —
#                            except on a home that arms nothing at all (no
#                            autostart_interval: below), where the value is
#                            named on stderr and the hook stands down exit 0:
#                            a refusal needs an arm to refuse, and G7's row
#                            is gated on the same switch (ranger-base-hmet1)
#   autostart_multiplexer_unit:
#                            read by `posse systemd` only, never here: the
#                            multiplexer server's own user unit, which the
#                            rendered unit is ordered After=/Requires=
#   autostart_interval:      base pass interval (30s, 5m, or bare seconds)
#                            — PRESENCE OF THIS KEY IS THE ARM SWITCH, and a
#                            value it cannot read is a broken arm, not a
#                            disarm: refused by name, nothing armed
#   autostart_max_interval:  backoff cap for quiet passes (default: posse's 8x)
#                            — a value it cannot read is NAMED AND DROPPED,
#                            not refused: unlike the arm switch above, this
#                            key has a default to fall back to, so a typo in
#                            a backoff cap must not stand a fleet down
#   autostart_max_beads:     -n, launch attempts per dispatch_epoch: (default
#                            1h, ADR 0028 §2) — RAISES OR LOWERS
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
# answers. Run by hand without the flag it is conservative and kills nothing:
# a workspace already wearing the name is left alone whether or not a loop is
# in it — but when no loop is, the by-hand run says so and exits nonzero,
# because it armed nothing (ranger-base-oej).
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

startup=false
[ "${1:-}" = "--startup" ] && startup=true

say() { echo "dispatch autostart: $*"; }

# Which home. This is the SECOND decision site for one fact — newApp
# (internal/posse/app.go) is the first — so it reads the same way or the hook
# arms out of one instance while the loop it starts runs out of another
# (ranger-base-g7lt). RHQ_HOME wins, then POSSE_HOME (both-names window,
# ranger-base-mlc Q2), then ~/.config/posse, unless it does not exist and
# ~/.config/rhq does. `-e` follows symlinks and is true for a plain file,
# matching os.Stat; `-d` matches st.IsDir() on the legacy side, so a dangling
# posse symlink falls back and a posse *file* does not.
#
# Not a bare default either way: ~/.config/rhq alone would leave a fresh
# `posse init` disarmed forever, and ~/.config/posse alone would disarm every
# instance that still has only the old home — this operator's included — at
# the next herdr start.
if [ -z "${RHQ_HOME:-}" ]; then
	if [ -n "${POSSE_HOME:-}" ]; then
		RHQ_HOME=$POSSE_HOME
	else
		preferred=$HOME/.config/posse
		legacy=$HOME/.config/rhq
		RHQ_HOME=$preferred
		if [ ! -e "$preferred" ] && [ -d "$legacy" ]; then
			RHQ_HOME=$legacy
			say "$preferred does not exist; using existing home $legacy (nothing moved)" >&2
		fi
	fi
fi
# Exported, not just assigned: the probe, `posse new`, and the dispatch loop
# that session runs all inherit it, so every one of them resolves the home
# the arm decision was actually made from. Unexported, a legacy-home arm
# handed `posse new` a bare environment and the loop wrote its session into
# ~/.config/posse — the arm and the queue in different instances.
export RHQ_HOME

CONFIG=$RHQ_HOME/config.yaml
LOG=$RHQ_HOME/state/dispatch-watch.log
MAXLOG=${AUTOSTART_MAXLOG:-5242880} # 5 MiB, then one .1 generation

# The plugin registry is global, so herdr runs this hook for named session
# servers too. Only the default server owns the fleet queue and its one
# dispatch-watch.pid. A scratch/named server inheriting the fleet RHQ_HOME
# must fail closed before it reads that pidfile or invokes posse
# (ranger-base-87q).
#
# HERDR_SOCKET_PATH is authoritative when present, matching herdr itself. If
# it is absent, HERDR_SESSION still proves this is a named server. The fixed
# default socket layout is measured and shared with herdrSocketPath in
# internal/posse/herdrback.go; herdr has no config-dir override.
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
# unrelated notices there — the config-home transition notice used to fire on
# every invocation of an instance that still has only the old home, and does
# not for these children only because RHQ_HOME is exported above — and a line
# glued in front of the answer would read as "could not ask" and stand the
# hook down for good. The next such notice will not announce itself either.
# stderr is kept, but only to quote when the answer did not arrive.
loopstate=
loopwho=
loopsaid=
loopwhy=
loop_alive() {
	local errd errf
	# A DIR WITH THE FILE INSIDE IT, never a bare `mktemp`
	# (ranger-base-u1z89). This hook runs at every herdr startup, and a bare
	# one leaves a FILE in /tmp when the hook is killed past its traps —
	# every candidate walk in scripts/reap-test-tmp.sh is `-type d`, so such
	# a file is collected by nothing, ever. A `posse-*` dir is taken by that
	# script's posse-scratch class at 120m.
	errd=$(mktemp -d "${TMPDIR:-/tmp}/posse-autostart.XXXXXX" 2>/dev/null) || errd=
	errf=${errd:+$errd/stderr}
	loopsaid=$("$RHQ" dispatch --watch-status 2>"${errf:-/dev/null}")
	if [ -n "$errf" ]; then
		loopwhy=$(cat "$errf")
		rm -rf -- "${errd:?}"
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

# Flat-YAML scalar read: `key: value`, a " #" comment stripped, a wrapping
# pair of double quotes dropped, and null/~ read as unset.
#
# None of that is decoration. This file has TWO readers: posse reads it
# with yamlClean (internal/posse/yamlflat.go, behind CfgGet/YamlGet) and this
# hook reads it here, and they must not disagree about what the operator
# wrote. yamlClean drops a matched pair of double quotes, so YAML's own way
# of writing a string — `autostart_interval: "5m"` — is 5m to posse, which
# reports the loop armed, while cfg() handed the hook `"5m"` and the hook
# refused it as not an interval. Same split on every key here: a quoted
# session name was created WITH the quotes in it, a quoted dir did not
# exist, a quoted count hit the "is not a count" arm.
#
# So the rule is yamlClean's rule, in yamlClean's order, and not a shell
# reader invented here (ranger-base-k3yd, ranger-base-fqfw). In order:
#
#   1. THE COMMENT. yamlClean's findComment is awk's rule — a comment starts
#      at WHITESPACE followed by '#' — and it scans the value from index 1,
#      so a '#' with nothing before it is data, not a comment. Hence the
#      leading-whitespace strip moved DOWN here from the first sed: eating
#      it first made `dir:#tmp` and `dir: #tmp` the same input, and posse
#      reads them as `#tmp` and empty. `[[:blank:]]` is space and tab,
#      exactly what findComment accepts.
#   2. WHITESPACE, both ends (strings.TrimSpace).
#   3. THE QUOTES: double only (single quotes are not in the house subset
#      and stay part of the value), both ends or neither.
#   4. NULL. yamlGetLines maps a cleaned `null` or `~` to the empty string,
#      so those are YAML's spellings of "unset" and this hook must read them
#      as the key not being there — for `autostart_dir:` that is $HOME and
#      not a directory called null. AFTER the unquote, because that is where
#      yamlGetLines checks: `"null"` is unset to posse too.
#
# The arm switch is the one key where unset-by-value is still not absent:
# `autostart_interval: null` reads empty here and `haskey` still sees the
# key, which is the broken-arm stand-down below and the EMPTY arm of G7
# (internal/posse/govern.go) — the same arm, on both surfaces, with the same
# sentence.
cfg() {
	[ -f "$CONFIG" ] || return 0
	local v
	v=$(sed -n "s/^$1://p" "$CONFIG" | head -1 |
		sed -e 's/[[:blank:]]#.*$//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' \
			-e 's/^"\(.*\)"$/\1/')
	case "$v" in
	null | '~') return 0 ;;
	esac
	printf '%s\n' "$v"
}

# Is the key THERE, regardless of what it says? `cfg` answers with the value,
# and an empty value is indistinguishable from an absent key in that answer —
# which for the one key whose PRESENCE is the arm switch is the difference
# between "you have not armed this" and "your arm is broken". The match
# mirrors cfg's own so the two can never disagree about what counts as the
# key (ranger-base-cxyk).
haskey() {
	[ -f "$CONFIG" ] || return 1
	grep -q "^$1:" "$CONFIG"
}

# Does posse accept this interval? Mirrors internal/posse.ParseInterval
# (internal/posse/watch.go): bare seconds, or a Go duration — one or more
# number+unit groups, units ns/us/µs/μs/ms/s/m/h. Positive, too, which for
# this grammar is exactly "some magnitude is nonzero", there being no sign to
# write: `0`, `0s` and `0h0m` all parse and are all refused by ParseInterval.
# Asked here so the refusal lands in the deployer's terminal instead of in a
# session log nobody opens (ranger-base-7rt5).
valid_interval() {
	local num='([0-9]+(\.[0-9]*)?|\.[0-9]+)' unit='(ns|us|µs|μs|ms|s|m|h)'
	[[ $1 =~ ^\+?[0-9]+$ || $1 =~ ^\+?($num$unit)+$ ]] || return 1
	[[ $1 =~ [1-9] ]]
}

# WHO OWNS THIS HOME'S LOOP (ADR 0080 D9). Read before anything else about
# the arm, because it decides whether this hook has any business here at all:
# a `systemd` home is never armed by this hook whatever the rest of its
# config says, which is what makes D9's race unreachable rather than merely
# unlikely.
#
# The grammar is cfg()'s and AutostartVia's (internal/posse/systemdunit.go),
# and the two are pinned on one input table (systemdunit_test.go) exactly as
# this file's interval check is pinned against ParseInterval. Absent, bare,
# `null` and `~` all read as herdr: this key HAS a default, unlike the arm
# switch below, and "" is the one reading cfg() and yamlClean cannot disagree
# about. A value that is neither name is refused rather than defaulted —
# defaulting it to herdr is how a systemd host's hook takes the lock the unit
# needs, and standing down on it would be the same mistake in the other
# direction on a herdr host. On a home with no arm switch the refusal has
# nothing to refuse and the value is named instead; the arm's own `case`
# below says why.
via=$(cfg autostart_via)
case "$via" in
'' | herdr) ;;
systemd)
	say "autostart_via: systemd in $CONFIG — systemd owns this home's loop (ADR 0080 D5), nothing armed here"
	say "the unit: posse systemd --write, then systemctl --user enable --now <unit>; this hook arms nothing while the key says systemd"
	exit 0
	;;
*)
	# A VALUE NEITHER NAME IS A REFUSAL, and a refusal needs an arm to
	# refuse. On a home with no `autostart_interval:` at all this hook would
	# arm nothing whatever the owner said, so exiting 1 over the owner is a
	# [[startup]] hook that fails at every server start about a loop nobody
	# asked for — and `posse status`'s G7 cannot name why, because its whole
	# row is gated on the arm switch (ranger-base-hmet1, from
	# ranger-base-zml8t). So the value is still NAMED, in the same words and
	# the same place, and the exit code is the one the disarm below uses: the
	# typo goes loud at the moment it starts mattering, which is the moment
	# the home is armed.
	#
	# The arm SWITCH and not the arm's value: a present-but-broken
	# `autostart_interval:` is a home that means to arm, so an unreadable
	# owner there is refused as before — the more total of the two failures
	# (no mechanism will arm this home) and the one that speaks.
	if ! haskey autostart_interval; then
		say "autostart_via: '$via' in $CONFIG is not a mechanism — use herdr or systemd; nothing to arm here anyway (no autostart_interval:)" >&2
		exit 0
	fi
	say "autostart_via: '$via' in $CONFIG is not a mechanism — use herdr or systemd; nothing armed" >&2
	exit 1
	;;
esac

interval=$(cfg autostart_interval)
if [ -z "$interval" ]; then
	# A bare `autostart_interval:` is a BROKEN ARM, not a disarm. The seed
	# config promises there is no off-value for this key — disarming is done
	# by commenting it out — so reading an empty one as off would invent the
	# very off-value that paragraph says does not exist, and say "no
	# autostart_interval:" about a key the deployer can see in the file. It
	# cannot be defaulted either: `posse dispatch --watch` has no default
	# interval and dies on the empty argument, so the only choice is where
	# that failure lands — here, or inside the herdr session's log. Same
	# stand-down as a missing binary below: named, loud, nothing armed.
	if haskey autostart_interval; then
		say "autostart_interval: in $CONFIG is present but empty — give it an interval (30s, 5m, or bare seconds), or comment the key out to disarm" >&2
		exit 1
	fi
	say "disarmed (no autostart_interval: in $CONFIG)"
	exit 0
fi
# ...and a value it cannot read is the same broken arm, one input shape over.
# `posse dispatch --watch banana` dies with `bad interval "banana"` — inside
# the herdr session, minutes later, under a hook that already said "dispatch
# started". Every other value key here names what it cannot read
# (autostart_max_beads, autostart_resume); this one cannot fall back on a
# default because there is none to fall back on, so it stands down exactly the
# way the empty key above does (ranger-base-7rt5).
if ! valid_interval "$interval"; then
	say "autostart_interval: '$interval' in $CONFIG is not an interval — use 30s, 5m, or bare seconds; nothing armed" >&2
	exit 1
fi
# There used to be a fallback here that armed through the transition alias
# bin/rhq when bin/posse was missing (rangerhq-tyay). `make link-plugin` no
# longer writes that name (ranger-base-igup), so the arm could only ever fire
# on a plugin dir left over from before the retirement — and arming a fleet
# loop off a stale link is worse than the loud failure below.
if [ ! -x "$RHQ" ]; then
	say "no posse at $RHQ — run 'make link-plugin'" >&2
	exit 1
fi

session=$(cfg autostart_session); session=${session:-dispatch}
maxint=$(cfg autostart_max_interval)
# Same defect as the interval above, and deliberately NOT the same answer.
# `posse dispatch --watch 5m --max-interval banana` dies exactly the way
# `--watch banana` does, inside the session, under a hook that already said
# "dispatch started" — but what to do about it is a real choice, because this
# key has a default and that one does not: absent, the flag is omitted and
# posse supplies 8x the base interval (cmd/posse/main.go). So it takes the
# autostart_max_beads / autostart_resume precedent — name the value on stderr
# and use the default — rather than the autostart_interval one, which refuses
# only because there is no default to refuse in favour of. Standing a whole
# fleet down over an unreadable backoff cap, on a key that is not the arm
# switch, would be the more expensive failure of the two (ranger-base-x8y8).
#
# An EMPTY value stays silent: absent and empty both mean "no cap given, use
# the default", which is what they already meant here and what an empty
# autostart_max_beads: means one block down. Only a value that says something
# posse cannot read gets a line.
if [ -n "$maxint" ] && ! valid_interval "$maxint"; then
	say "autostart_max_interval: '$maxint' in $CONFIG is not an interval — use 30s, 5m, or bare seconds; using posse's default cap (8x $interval)" >&2
	maxint=
fi
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

# THE LOG, and who writes it (ranger-base-n00wn). Built here rather than
# above because it reads `$loopsaid` — the answer the probe just gave.
#
# A posse whose `--watch-status` line NAMES its log writes that log itself:
# it opens $RHQ_HOME/state/dispatch-watch.log at the top of the loop, tees
# its own stdout and stderr into it, and rotates it at 5 MiB. Then this hook
# must not tee, or every line lands twice.
#
# A posse too old to name one keeps the tee it has always had, rotation
# included. That is the whole reason this is a probe and not a version
# check: the hook is read fresh from the checkout at every herdr start while
# the binary is whatever was last promoted, so the two are routinely out of
# step in both directions. The probe gets each of them right — an old binary
# keeps its record, a new one is not written twice.
#
# WHAT IT DOES NOT FIX is a by-hand relaunch that still types the old
# `| tee -a $LOG` pipeline at a new binary: that doubles lines until the
# habit changes. Doubling is a readable record; the failure this replaces
# was no record at all.
case "$loopsaid" in
*" · log "*) selflog=true ;;
*) selflog=false ;;
esac
mkdir -p "$(dirname "$LOG")"
if $selflog; then
	# The loop owns the file: no pipe, no banner, no rotation here. Its argv
	# is then the posse command itself rather than the left half of a
	# pipeline, which is what every reader of the process table wanted anyway.
	cmd="$watch"
else
	if [ -f "$LOG" ] && [ "$(wc -c <"$LOG")" -gt "$MAXLOG" ]; then
		mv -f "$LOG" "$LOG.1"
	fi
	banner="printf '\\n== dispatch --watch armed %s ==\\n' \"\$(date '+%Y-%m-%d %H:%M:%S')\" | tee -a $LOG"
	cmd="$banner; $watch 2>&1 | tee -a $LOG"
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
		# What the kill SAID is kept, because it is the only thing that can
		# explain the failure below. A kill can refuse — the ADR 0013 §4 reap
		# guard, the foreign-workspace refusal — and a refusal swallowed into
		# /dev/null leaves "still present after kill" naming no lever
		# (ranger-base-oej). Newlines are squashed: this is quoted on one line.
		if ! killsaid=$("$RHQ" kill "$session" 2>&1 | tr '\n' ' '); then
			say "posse kill $session refused: ${killsaid:-nothing}" >&2
		fi
		start; rc=$?
		[ "$rc" = 3 ] && { say "$session still present after kill — not started${killsaid:+ (kill said: $killsaid)}" >&2; rc=1; }
	else
		# NOT "already running". The lock was asked two blocks up and answered
		# NONE, so whatever wears this name, it is not a dispatch loop — and
		# reporting a live loop off a name refusal contradicts the hook's own
		# measurement. That is the bug: during F8 an operator read
		# "dispatch already running — left alone" twice with no loop and no
		# pidfile, off a workspace herdr had restored without its command, and
		# the line named neither the husk nor the lever (ranger-base-oej).
		#
		# The crew mark is why nothing clears it on its own: this session is
		# created by `posse new`, which stamps `crew: true` (ADR 0008), so it
		# shows as the operator's 👤 in `posse list` and every sweep steps over
		# it. `posse kill` is the lever, and it is the operator's to pull — a
		# by-hand run still never kills, because the name may be worn by a
		# workspace they are sitting in.
		#
		# Nonzero, unlike the old rc=0: this run armed nothing, the same
		# failure the broken-arm and missing-binary stand-downs above exit 1
		# on. An arm that did not arm must not report success.
		say "$session exists but no dispatch loop holds the lock — left alone, nothing armed" >&2
		say "a herdr workspace outlives its command; this one wears the operator's crew mark 👤 (posse new stamps it), so no sweep clears it — run 'posse kill $session' and re-run this hook, or restart the herdr server, whose --startup run replaces a husk itself" >&2
		rc=1
	fi
fi
exit "$rc"
