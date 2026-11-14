#!/bin/bash
# ADR 0097 (ranger-base-m2ii4): do the wall's live pins measure inside the
# venue ADR 0097 D1 chooses, and what does one run cost? Run UNCAGED on Linux
# from inside a checkout of this repo. Each arm gets a fresh run dir under
# the user cache (not /tmp: the venue binds a private /tmp over it) and a
# fresh GOCACHE, so every venue reading is a COLD build, as the judge's are
# (ADR 0088 D3.2). Writes only that run dir and the go test cache inside it.
#
#   bash docs/adr/0097-wall-venue.probe.sh [uncaged|venue|seat-proc|judge+venue]
#
# Arms:
#   uncaged      the pins as an operator's `make test` runs them (the control)
#   venue        D1's wall: the D17 base, --unshare-user, --unshare-pid
#                --proc /proc, --unshare-net, private /tmp and /var/tmp
#   seat-proc    the same wall WITHOUT the pid namespace and procfs, i.e. the
#                host /proc a seat has (ADR 0096 arm A): the pins cannot nest
#   judge+venue  scripts/posse-landed-check, then the pins, in one venue run:
#                the price of folding them into the judge (rejected, D2)
#
# MEASURED 2026-11-14, Linux 7.2.15, bubblewrap 0.13.1, go1.27.3, 16 cores:
# see the ADR's table. A cell that reads differently reopens D1.
set -u
command -v bwrap >/dev/null || { echo "bwrap: absent"; exit 1; }
TREE=$(git rev-parse --show-toplevel) || exit 1
RE='^Test(QA)?Live(Bwrap|Wall|StrictMCP|GrokLeader|GrokLine)'
PINS="go test -count=1 -json -run '$RE' ./internal/posse"
mkdir -p "${XDG_CACHE_HOME:-$HOME/.cache}/posse-0097-probe"

arm() {
  local mode=$1 body=$2 run start end rc
  run=$(mktemp -d -p "${XDG_CACHE_HOME:-$HOME/.cache}/posse-0097-probe")
  mkdir -p "$run/tmp" "$run/vartmp" "$run/gocache"
  local wall=(--die-with-parent --ro-bind / / --dev-bind /dev /dev --unshare-user
    --unshare-net --bind "$TREE" "$TREE" --bind "$run" "$run"
    --bind "$run/tmp" /tmp --bind "$run/vartmp" /var/tmp --chdir "$TREE"
    --setenv TMPDIR "$run/tmp" --setenv GOCACHE "$run/gocache")
  start=$(date +%s%N)
  case $mode in
  uncaged) (cd "$TREE" && bash -c "$body") > "$run/out.json" 2> "$run/err" ;;
  seat-proc) bwrap "${wall[@]}" bash -c "$body" > "$run/out.json" 2> "$run/err" ;;
  *) bwrap "${wall[@]}" --unshare-pid --proc /proc bash -c "$body" > "$run/out.json" 2> "$run/err" ;;
  esac
  rc=$?
  end=$(date +%s%N)
  printf 'ARM %-12s rc=%s wall=%ss load=%s\n' "$mode" "$rc" "$(( (end - start) / 1000000000 ))" "$(cut -d' ' -f1 /proc/loadavg)"
  python3 -I -c '
import json, sys, collections
c = collections.Counter(); first = {}
for l in open(sys.argv[1]):
    try: e = json.loads(l)
    except ValueError: continue
    t = e.get("Test")
    if not t or "/" in t: continue
    a = e.get("Action")
    if a in ("pass", "fail", "skip"): c[a] += 1; first[t] = (a, first.get(t, (None, ""))[1])
    elif a == "output" and t not in first and ("NOTHING MEASURED" in e["Output"] or "uid map" in e["Output"]):
        first[t] = (None, e["Output"].strip()[:120])
print("  ", dict(c))
for t, (a, why) in sorted(first.items()):
    if a and a != "pass": print("  ", a, t, "|", why)
' "$run/out.json"
  tail -1 "$run/err" | sed 's/^/   stderr: /'
}

[ $# -gt 0 ] || set -- uncaged venue seat-proc
for m in "$@"; do
  case $m in
  judge+venue) arm "$m" "scripts/posse-landed-check && $PINS" ;;
  *) arm "$m" "$PINS" ;;
  esac
done
