#!/usr/bin/env bash
# scratch-rex.sh — run a live probe against a SCRATCH Rex server, with the
# fleet's own server unreachable from inside it (ranger-base-uq81v).
#
# THE INCIDENT THIS IS. 2026-11-15 08:31:19-22, a scratch-server live test
# destroyed eight live fleet seats. The script started a scratch rex server
# under a scratch XDG_RUNTIME_DIR and expected the rex CLI and posse to
# resolve that socket. Neither does. Its own sanity line printed the FLEET
# endpoint and a session count of 12, it did not stop, it ran `posse new`
# against the fleet server, and its cleanup loop ran `rex kill` on every
# session in `rex ls`. Eight seats, five personas, one of them the seat
# running the loop. ranger-base-uq81v has the journal, the transcript and the
# fix direction; docs/notes.d/ranger-base-uq81v.md has the measurements below.
#
# WHY THE EXPECTATION WAS WRONG, MEASURED 2026-11-20 on this box
# (linux 7.2.15-1-omarchy, rex CLI 0.0.1794069975-g6c6689, read-only
# `whoami --json` calls under `--autostart=false`):
#
#   plain                                   the local default socket
#   XDG_RUNTIME_DIR=<scratch>               the local default socket  ← ignored
#   XDG_RUNTIME_DIR=<scratch> HOME=<fake>   the local default socket  ← ignored
#   REX_RUNTIME_DIR=<scratch>               <scratch>/server.sock
#   REX_SERVER=unix://<scratch>/…           the local default socket  ← ignored
#   -S unix://<scratch>/…                   <scratch>/… (the flag works)
#
# So rex's runtime dir is REX_RUNTIME_DIR else a /run/user/<uid> path it
# builds from getuid(); XDG_RUNTIME_DIR is not read for it. `REX_SERVER` is a
# variable rex EXPORTS into a pane beside REX_SESSION and REX_BLOCK, not one
# the CLI reads — a bogus value, a bogus host label included, changes nothing
# and raises nothing, which makes it the most dangerous of the three: a
# harness that "passes the endpoint by environment" that way addresses the
# fleet and says it did not.
#
# AND POSSE HAS NO SUCH DOOR AT ALL. `NewRexMux` builds `Rex{Bin: a.RexBin()}`
# with an empty `Server`, so every posse rex call goes to the box's default
# socket (internal/posse/muxrex.go, internal/posse/rex.go's rexGlobals). There
# is no config key and no environment variable that moves posse to another
# server. The only seam is the BINARY — `rex_bin:` / RHQ_REX_BIN — which is
# why this harness interposes on it rather than hoping.
#
# THE THREE GUARDS, in the order they run:
#
#  1. IDENTITY. After the scratch server is up, `whoami --json` is read back
#     through `-S <the socket this script started>` and the run ABORTS unless
#     its `endpoint` is byte-equal to that socket and its `session_count` is
#     0. An empty read is a mismatch. Exit 70, before the body runs at all.
#  2. INTERPOSITION. The body runs with `$SCRATCH_REX_DIR/bin/rex` at the head
#     of PATH and named in RHQ_REX_BIN, which is this same script in shim
#     mode: it prepends `-S <the scratch socket> --autostart=false` to every
#     call, refuses a `-S`/`--server` of the body's own, and refuses the
#     `server` verb. A body that types a bare `rex`, and posse called from
#     that body, cannot reach the fleet server by any spelling.
#  3. THE KILL FENCE. The shim refuses `kill` with no session id, and refuses
#     an id that is not in the record this harness keeps — which it refreshes
#     from the scratch server's own `ls -q` after every successful `new`. So
#     `for s in $(rex ls); do rex kill "$s"; done` kills what this run made
#     and nothing else even if guard 1 somehow passed, and cleanup kills
#     through the same fence rather than beside it.
#
# `--autostart=false` is injected on EVERY shimmed call, including `new`,
# where posse itself lets the default stand (rex.go's head comment, item 3).
# The scratch server is already up, so autostart never fires and the
# observable behaviour is posse's; what it buys is that a scratch server that
# DIED is an honest failure rather than an empty server silently started in
# its place, which is the reading hazard that whole comment is about.
#
# Usage:
#   scripts/scratch-rex.sh [--sessions N] -- COMMAND [ARG...]
#   scripts/scratch-rex.sh --self-test
#   SCRATCH_REX_BIN=/path/to/rex scripts/scratch-rex.sh -- COMMAND ...
#
# --self-test drives the three guards' own predicates over a table and opens
# nothing: no server, no socket, no rex binary, no fleet. It is what
# internal/treepins/scratchrex_qa_test.go runs, and it is the arm that says
# the fence can still refuse.
#
# WHAT THE BODY GETS, all exported:
#   SCRATCH_REX_ENDPOINT  unix:///…/s.sock — pass it to anything not shimmed
#   SCRATCH_REX_DIR       the scratch root: bin/, state/, sessions, server.log
#   SCRATCH_REX_RECORD    the session-id record the fence reads
#   RHQ_REX_BIN, PATH     pointed at the shim (guard 2)
#
# WHAT IT DOES NOT DO. It does not give the body a scratch RHQ_HOME, HOME or
# cwd: a posse probe wants all three and they are the caller's, because what
# belongs in them is the caller's question. A scratch SOCKET is not isolation
# on its own (scripts/verify-ghost-composer.sh's header says the same thing
# about herdr), so a probe that writes posse records sets RHQ_HOME too.
#
# RESIDUAL, said out loud: the fence matches a session id EXACTLY as `rex ls
# -q` prints it (`session:<uuid>`, and that CLI prints the same string for
# --short and --short=false). A unique prefix, which the rex CLI itself
# accepts, is refused here with the record's path in the refusal. Widening it
# to prefixes would be a matcher to get subtly right in the one place whose
# whole value is that it cannot be.
set -euo pipefail

me=scratch-rex

die() {
	echo "$me: $*" >&2
	exit 2
}

# ─── the three guards' predicates, shared by the shim and the self-test ──────

# endpoint_is <reported> <wanted> — guard 1's judge. Byte equality, and never
# a pass on an empty read: a `whoami` that printed nothing is a server that
# did not answer, which is the one reading that must not be taken for a match.
endpoint_is() {
	[ -n "${1:-}" ] || return 1
	[ "${1:-}" = "${2:-}" ]
}

# recorded <id> <record-file> — guard 3's judge. The id is a whole line of the
# record, which holds what the scratch server's own `ls -q` printed.
recorded() {
	[ -n "${1:-}" ] || return 1
	[ -s "${2:-/dev/null}" ] || return 1
	grep -qxF -- "$1" "$2"
}

# first_positional <argv...> — the rex verb, option-aware. The value-taking
# globals are skipped so that `rex --timeout 5s ls` reads as `ls` and not as
# `5s`; a rule that read the first non-dash word would call the VALUE the verb
# and let `kill` past the fence behind one global flag.
first_positional() {
	local skip=0 a
	for a in "$@"; do
		if [ "$skip" = 1 ]; then
			skip=0
			continue
		fi
		case $a in
		-C | --client | --color | --timeout | -S | --server) skip=1 ;;
		-*) ;;
		*)
			printf '%s' "$a"
			return 0
			;;
		esac
	done
	printf ''
}

# positional_ids <argv...> — every positional after the verb, which for `kill`
# is the session ids. Same option-awareness as first_positional.
positional_ids() {
	local skip=0 seen=0 a
	for a in "$@"; do
		if [ "$skip" = 1 ]; then
			skip=0
			continue
		fi
		case $a in
		-C | --client | --color | --timeout | -S | --server) skip=1 ;;
		-*) ;;
		*)
			if [ "$seen" = 1 ]; then
				printf '%s\n' "$a"
			else
				seen=1
			fi
			;;
		esac
	done
}

# names_a_server <argv...> — does this argv carry an endpoint of its own?
# Refused in the shim: this harness owns the endpoint, and a `-S` late in an
# argv wins over the one the shim prepends.
#
# IT READS THE SHORT CLUSTER CHARACTER BY CHARACTER, because an exact match on
# `-S` is not enough. MEASURED 2026-11-20: rex takes `-Sunix:///…` with the
# value ATTACHED, identically to `-S unix:///…` and `--server unix:///…`, and
# pflag clusters shorthands, so `-vSunix:///…` is the same flag again. A test
# on the whole word would pass all three straight through to a call that then
# addresses whatever they name. The scan stops at `-C`, whose own value
# follows it in the same word and may contain any letter.
names_a_server() {
	local a rest c
	for a in "$@"; do
		case $a in
		--server | --server=*) return 0 ;;
		--*) continue ;;
		-?*)
			rest=${a#-}
			while [ -n "$rest" ]; do
				c=${rest:0:1}
				case $c in
				S) return 0 ;;
				C) break ;;
				*) rest=${rest:1} ;;
				esac
			done
			;;
		esac
	done
	return 1
}

# ─── shim mode: invoked as `rex`, through the symlink the launcher makes ─────

shim_die() {
	echo "$me shim: $*" >&2
	exit 70
}

shim_record() {
	local rec=$SCRATCH_REX_RECORD
	"$SCRATCH_REX_BIN" --color never -S "$SCRATCH_REX_ENDPOINT" --autostart=false \
		ls -q >>"$rec" 2>/dev/null || true
	grep -v '^[[:space:]]*$' "$rec" | sort -u >"$rec.new" || true
	mv -f "$rec.new" "$rec"
}

shim_main() {
	local verb rc id
	if names_a_server "$@"; then
		shim_die "a body may not name a server: this run owns $SCRATCH_REX_ENDPOINT (guard 2)"
	fi
	verb=$(first_positional "$@")
	case $verb in
	server)
		shim_die "the harness owns the server; a body does not start or stop one (guard 2)"
		;;
	kill)
		local ids=()
		while IFS= read -r id; do
			[ -n "$id" ] && ids+=("$id")
		done < <(positional_ids "$@")
		if [ "${#ids[@]}" = 0 ]; then
			shim_die "refusing a kill that names no session (guard 3)"
		fi
		for id in "${ids[@]}"; do
			recorded "$id" "$SCRATCH_REX_RECORD" ||
				shim_die "refusing to kill $id: this run did not create it. The record of what it did is $SCRATCH_REX_RECORD, and an id is matched exactly as \`rex ls -q\` prints it (guard 3)"
		done
		;;
	esac
	rc=0
	"$SCRATCH_REX_BIN" --color never -S "$SCRATCH_REX_ENDPOINT" --autostart=false "$@" || rc=$?
	if [ "$verb" = new ] && [ "$rc" = 0 ]; then
		shim_record
	fi
	exit "$rc"
}

if [ "$(basename -- "$0")" = rex ]; then
	: "${SCRATCH_REX_BIN:?the shim is launched by scratch-rex.sh, which exports this}"
	: "${SCRATCH_REX_ENDPOINT:?the shim is launched by scratch-rex.sh, which exports this}"
	: "${SCRATCH_REX_RECORD:?the shim is launched by scratch-rex.sh, which exports this}"
	shim_main "$@"
fi

# ─── self-test: the guards' predicates, over a table, opening nothing ────────

selftest() {
	local fails=0 rec recdir runroot localsock ids
	check() { # check <name> <got> <want>
		if [ "$2" = "$3" ]; then
			echo "PASS  $1"
		else
			echo "FAIL  $1  got $2, want $3"
			fails=1
		fi
	}
	yn() { if "$@"; then echo yes; else echo no; fi; }

	# Guard 1. The scratch endpoint this table is written against, and the
	# box's DEFAULT local socket — built here rather than spelled, because a
	# literal uid path in a tracked file is what `make identity-check` is for.
	runroot=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}
	localsock="unix://$runroot/rex/server.sock"
	check "endpoint: the socket this run started is a match" \
		"$(yn endpoint_is "unix://$runroot/posse-scratch-rex.aaaaaa/s.sock" "unix://$runroot/posse-scratch-rex.aaaaaa/s.sock")" yes
	check "endpoint: THE INCIDENT — the box's default socket is not it" \
		"$(yn endpoint_is "$localsock" "unix://$runroot/posse-scratch-rex.aaaaaa/s.sock")" no
	check "endpoint: a read that answered nothing is not a match" \
		"$(yn endpoint_is "" "unix://$runroot/posse-scratch-rex.aaaaaa/s.sock")" no
	check "endpoint: a prefix of the wanted socket is not a match" \
		"$(yn endpoint_is "unix://$runroot/posse-scratch-rex.aaaaaa" "unix://$runroot/posse-scratch-rex.aaaaaa/s.sock")" no
	check "endpoint: the same path without the scheme is not a match" \
		"$(yn endpoint_is "$runroot/posse-scratch-rex.aaaaaa/s.sock" "unix://$runroot/posse-scratch-rex.aaaaaa/s.sock")" no

	# Guard 3, over a record of two ids this "run" made.
	# A dir with a fixed-name file in it, which is the house answer for a
	# scratch file: scripts/reap-test-tmp.sh's candidate walks are all
	# `-type d`, so a leaked scratch FILE is reachable by no class it has
	# (internal/treepins/mktemproot_qa_test.go).
	recdir=$(mktemp -d "${TMPDIR:-/tmp}/posse-scratch-rex-selftest.XXXXXX")
	rec="$recdir/sessions"
	printf '%s\n' "session:01a1fc0c-a948-7af1-928a-80649bfc1bb0" \
		"session:01a1fc0d-7245-755c-8bf3-69077b738c4c" >"$rec"
	check "fence: an id this run created is killable" \
		"$(yn recorded "session:01a1fc0c-a948-7af1-928a-80649bfc1bb0" "$rec")" yes
	check "fence: an id this run did not create is refused" \
		"$(yn recorded "session:01a1df74-2a1d-783d-9892-443b63e30bd0" "$rec")" no
	check "fence: an empty id is refused" "$(yn recorded "" "$rec")" no
	check "fence: a prefix of a created id is refused, with the record named" \
		"$(yn recorded "session:01a1fc0c" "$rec")" no
	check "fence: an empty record kills nothing" \
		"$(yn recorded "session:01a1fc0c-a948-7af1-928a-80649bfc1bb0" /dev/null)" no
	rm -rf -- "${recdir:?}"

	# The verb reader, which is what decides whether the fence is consulted.
	check "verb: a plain call" "$(first_positional ls -q)" ls
	check "verb: behind the harness's own globals" \
		"$(first_positional --color never -S unix:///x/s.sock --autostart=false kill session:a)" kill
	check "verb: a value-taking global does not become the verb" \
		"$(first_positional --timeout 5s kill session:a)" kill
	check "verb: an argv of flags alone has none" "$(first_positional --help)" ""
	# The newline-joined forms are flattened by parameter expansion and not by
	# a `| tr`: in an assertion arm the matcher must not fork
	# (internal/treepins/selftestforkarm_qa_test.go), because a matcher that
	# cannot be exec'd under load reports the property false when the
	# apparatus is what failed.
	ids=$(positional_ids --color never kill session:a session:b)
	check "ids: every positional after the verb" "${ids//$'\n'/ }" "session:a session:b"
	check "ids: a kill with no id names none" "$(positional_ids kill)" ""
	ids=$(positional_ids --timeout 5s kill session:a)
	check "ids: a global's value is not an id" "${ids//$'\n'/ }" "session:a"
	check "server: a body's own -S is seen" "$(yn names_a_server ls -S unix:///x)" yes
	check "server: a body's own --server= is seen" "$(yn names_a_server --server=lab ls)" yes
	check "server: --server with a separate value is seen" "$(yn names_a_server --server lab ls)" yes
	check "server: -S with the value ATTACHED is seen" "$(yn names_a_server ls -Sunix:///x)" yes
	check "server: -S inside a short cluster is seen" "$(yn names_a_server ls -vSunix:///x)" yes
	check "server: an argv with none" "$(yn names_a_server ls -q)" no
	check "server: a client label that happens to carry an S is not one" \
		"$(yn names_a_server -CmySeat ls)" no
	check "server: a bare dash is not a flag" "$(yn names_a_server send -)" no

	if [ "$fails" = 0 ]; then
		echo "$me --self-test: all guards can still say no"
		return 0
	fi
	echo "$me --self-test: FAILED"
	return 1
}

# ─── launcher mode ──────────────────────────────────────────────────────────

sessions=6
case ${1:-} in
--self-test)
	[ $# = 1 ] || die "--self-test takes no other argument"
	selftest
	exit $?
	;;
esac
while [ $# -gt 0 ]; do
	case $1 in
	--sessions)
		[ $# -ge 2 ] || die "--sessions needs a count"
		sessions=$2
		shift 2
		;;
	--)
		shift
		break
		;;
	*) die "usage: $me [--sessions N] -- COMMAND [ARG...]   |   $me --self-test" ;;
	esac
done
[ $# -gt 0 ] || die "usage: $me [--sessions N] -- COMMAND [ARG...]   |   $me --self-test"
case $sessions in
'' | *[!0-9]*) die "--sessions wants a number, got '$sessions'" ;;
esac

REXBIN=${SCRATCH_REX_BIN:-/usr/bin/rex}
[ -x "$REXBIN" ] || die "not executable: $REXBIN — name the rex CLI in SCRATCH_REX_BIN (there is no PATH lookup for it: on a Debian-family host \`rex\` is an unrelated Perl tool, ADR 0076)"

SELF=$(cd -- "$(dirname -- "$0")" && pwd)/$(basename -- "$0")
[ -x "$SELF" ] || die "cannot find myself at $SELF"

# The scratch root is in the RUNTIME dir and not under ${TMPDIR:-/tmp}, which
# is what every other root under scripts/ names: a unix socket path is capped
# (108 bytes on linux, 104 on darwin) and a dispatched seat's TMPDIR is its
# session scratchpad, ~130 characters before the socket's own name. The
# `posse-` prefix is kept so that `scripts/reap-test-tmp.sh --root <runtime
# dir>` reaches a root a SIGKILLed run left behind; the EXIT trap below is
# what collects it in the ordinary case. internal/treepins/mktemproot_qa_test.go
# carries the exemption row and this reason.
RUNROOT=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}
[ -d "$RUNROOT" ] || die "no runtime directory at $RUNROOT"
SD=$(mktemp -d "$RUNROOT/posse-scratch-rex.XXXXXX")
SOCK="$SD/s.sock"
ENDPOINT="unix://$SOCK"
REC="$SD/sessions"
SERVER_PID=""
mkdir -p "$SD/bin" "$SD/state"
: >"$REC"
ln -s "$SELF" "$SD/bin/rex"

# Every variable the handler reads is assigned in THIS shell, never inside a
# `$( … )` (internal/treepins/trapstatepick_qa_test.go), and the handler for a
# deadly signal exits rather than returning into a torn-down apparatus
# (internal/treepins/trapsignalexit_qa_test.go).
cleanup() {
	local id
	if [ -n "$SERVER_PID" ] && [ -s "$REC" ]; then
		while IFS= read -r id; do
			[ -n "$id" ] || continue
			SCRATCH_REX_BIN="$REXBIN" SCRATCH_REX_ENDPOINT="$ENDPOINT" \
				SCRATCH_REX_RECORD="$REC" "$SD/bin/rex" kill "$id" >/dev/null 2>&1 || true
		done <"$REC"
	fi
	if [ -n "$SERVER_PID" ]; then
		kill "$SERVER_PID" 2>/dev/null || true
		wait "$SERVER_PID" 2>/dev/null || true
	fi
	rm -rf -- "${SD:?}"
}
trap cleanup EXIT
trap 'echo "$me: interrupted (SIGINT) — the scratch server and its sessions are gone; no verdicts" >&2; exit 130' INT
trap 'echo "$me: interrupted (SIGTERM) — the scratch server and its sessions are gone; no verdicts" >&2; exit 143' TERM
trap 'echo "$me: interrupted (SIGHUP) — the scratch server and its sessions are gone; no verdicts" >&2; exit 129' HUP

"$REXBIN" server run --listen "$ENDPOINT" --restore=false \
	--state-dir "$SD/state" --max-sessions "$sessions" >"$SD/server.log" 2>&1 &
SERVER_PID=$!

n=0
while [ "$n" -lt 40 ]; do
	[ -S "$SOCK" ] && break
	kill -0 "$SERVER_PID" 2>/dev/null || die "the scratch server died before it listened; its log:
$(cat "$SD/server.log" 2>/dev/null)"
	n=$((n + 1))
	sleep 0.25
done
[ -S "$SOCK" ] || die "no socket at $SOCK after 10s; the server log:
$(cat "$SD/server.log" 2>/dev/null)"

# GUARD 1. Read the server back through the socket this script started, and
# abort unless it is the one that answers. This is the line the incident
# printed, read instead of printed.
whoami_json=$("$REXBIN" --color never -S "$ENDPOINT" --autostart=false whoami --json 2>/dev/null || true)
reported=$(printf '%s' "$whoami_json" | python3 -c 'import json,sys
try: d = json.load(sys.stdin)
except Exception: print(""); raise SystemExit(0)
print(d.get("endpoint", ""))' 2>/dev/null || true)
held=$(printf '%s' "$whoami_json" | python3 -c 'import json,sys
try: d = json.load(sys.stdin)
except Exception: print(-1); raise SystemExit(0)
print(d.get("session_count", -1))' 2>/dev/null || true)

echo "$me: scratch server pid $SERVER_PID, max $sessions sessions"
echo "  endpoint : $ENDPOINT"
echo "  answered : ${reported:-<nothing>} (session_count ${held:-<nothing>})"
echo "  root     : $SD"
echo "  rex      : $REXBIN"

if ! endpoint_is "$reported" "$ENDPOINT"; then
	echo "REFUSING: the server that answered is ${reported:-<nothing>}, not $ENDPOINT." >&2
	echo "          That is ranger-base-uq81v's incident exactly: rex does not read" >&2
	echo "          XDG_RUNTIME_DIR or REX_SERVER for its endpoint, so a scratch socket" >&2
	echo "          is reached by -S and by nothing else. Nothing ran." >&2
	exit 70
fi
if [ "${held:-x}" != 0 ]; then
	echo "REFUSING: $ENDPOINT answered with session_count ${held:-<nothing>}." >&2
	echo "          This server was started --restore=false a moment ago and must hold" >&2
	echo "          nothing; a count means it is not the server this run made. Nothing ran." >&2
	exit 70
fi

export SCRATCH_REX_BIN="$REXBIN"
export SCRATCH_REX_ENDPOINT="$ENDPOINT"
export SCRATCH_REX_RECORD="$REC"
export SCRATCH_REX_DIR="$SD"
export RHQ_REX_BIN="$SD/bin/rex"
export PATH="$SD/bin:$PATH"

echo
rc=0
"$@" || rc=$?
echo
echo "$me: body exit $rc; killing $(grep -c . "$REC" 2>/dev/null || echo 0) session(s) this run created, then pid $SERVER_PID"
exit "$rc"
