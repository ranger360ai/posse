#!/bin/sh
# ADR 0073 probe — does bd 0.50.3 print success for a write that did not land,
# and is concurrency from several worktrees of one repo the mechanism?
# Usage: sh docs/adr/0073-bd-write-readback.probe.sh <empty scratch dir> [writers] [creates-each]
# Rigs are hand-written (no `bd init`, denied at crew seats; ADR 0072's shape):
# one git repo per store mode, W linked worktrees, every writer bound to the
# MAIN rig's .beads with BEADS_DIR — which is how a posse seat reaches its
# store (ADR 0055). The live store of record is never touched.
set -u
root=${1:?usage: $0 <empty scratch dir> [writers] [creates-each]}
W=${2:-4}; K=${3:-12}
mkdir -p "$root" || exit 2

mkrig() { # mkrig <dir> [nodb]
  python3 - "$@" <<'EOF'
import json, os, subprocess, sys
d = sys.argv[1]; nodb = len(sys.argv) > 2
os.makedirs(os.path.join(d, ".beads"), exist_ok=True)
subprocess.run(["git", "init", "-q", d], check=True)
now = "2026-10-12T12:00:00Z"
with open(os.path.join(d, ".beads/issues.jsonl"), "w") as f:
    for i in (1, 2):
        f.write(json.dumps({"id": f"t-{i}", "title": f"seed {i}", "description": "seed", "status": "in_progress",
            "priority": 2, "issue_type": "task", "created_at": now, "updated_at": now, "created_by": "rig",
            "owner": "rig", "labels": [], "dependencies": [], "comments": []}, separators=(",", ":")) + "\n")
open(os.path.join(d, ".beads/config.yaml"), "w").write('issue-prefix: "t"\n' + ("no-db: true\n" if nodb else ""))
subprocess.run(["git", "-C", d, "add", "--", ".beads"], check=True)
subprocess.run(["git", "-C", d, "-c", "user.name=r", "-c", "user.email=r@r", "commit", "-qm", "rig", "--", ".beads"], check=True)
EOF
}

# writer <rig> <worktree> <tag>: K human-output creates, each line "<rc> <printed id or ->"
writer() {
  rig=$1; wt=$2; tag=$3; i=0
  while [ "$i" -lt "$K" ]; do
    i=$((i + 1))
    out=$(cd "$wt" && BEADS_DIR="$rig/.beads" bd --no-daemon create "w$tag-$i" -d "writer $tag create $i" 2>&1); rc=$?
    id=$(printf '%s\n' "$out" | sed -n 's/.*Created issue: \([^ ]*\).*/\1/p' | head -1)
    echo "$rc ${id:--} w$tag-$i" >>"$root/$(basename "$rig").printed"
  done
}

census() { # census <rig>: title -> landed? read from bd itself and from the jsonl
  rig=$1
  (cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon list --all --json --limit 0 2>/dev/null) >"$root/$(basename "$rig").list.json"
  python3 - "$root/$(basename "$rig").printed" "$root/$(basename "$rig").list.json" "$rig/.beads/issues.jsonl" <<'EOF'
import json, sys
printed = [l.split() for l in open(sys.argv[1])]
try: listed = {i["id"]: i["title"] for i in json.load(open(sys.argv[2])) or []}
except Exception as e: listed = None; print("  bd list unreadable:", e)
jl = {}
for line in open(sys.argv[3]):
    if line.strip(): r = json.loads(line); jl[r["id"]] = r["title"]
ok0 = [p for p in printed if p[0] == "0"]
noid = [p for p in ok0 if p[1] == "-"]
lost_bd = [p for p in ok0 if p[1] != "-" and listed is not None and listed.get(p[1]) != p[2]]
lost_jl = [p for p in ok0 if p[1] != "-" and jl.get(p[1]) != p[2]]
dup = len([p[1] for p in ok0 if p[1] != "-"]) - len({p[1] for p in ok0 if p[1] != "-"})
print(f"  creates {len(printed)}  exit0 {len(ok0)}  exit0-no-id {len(noid)}  printed-id-not-in-bd-list {len(lost_bd)}"
      f"  printed-id-not-in-jsonl {len(lost_jl)}  duplicate-printed-ids {dup}")
print(f"  bd list rows {None if listed is None else len(listed)}  jsonl rows {len(jl)}")
for p in (lost_bd + noid)[:5]: print("   e.g.", " ".join(p))
EOF
}

for mode in nodb db; do
  rig="$root/$mode"; mkrig "$rig" $([ "$mode" = nodb ] && echo nodb)
  (cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon list --json >/dev/null 2>&1)   # first read builds the db (db mode)
  : >"$root/$mode.printed"
  n=1; while [ "$n" -le "$W" ]; do git -C "$rig" worktree add -q "$root/$mode-wt$n" 2>/dev/null; n=$((n + 1)); done

  echo "== $mode, serial control: 1 writer x $K creates"
  writer "$rig" "$root/$mode-wt1" s; census "$rig"
  : >"$root/$mode.printed"

  echo "== $mode, concurrent: $W writers x $K creates, one per worktree, one store"
  n=1; while [ "$n" -le "$W" ]; do writer "$rig" "$root/$mode-wt$n" "$n" & n=$((n + 1)); done; wait
  census "$rig"

  echo "== $mode, close racing creates: close t-1 while $W writers create"
  : >"$root/$mode.printed"
  n=1; while [ "$n" -le "$W" ]; do writer "$rig" "$root/$mode-wt$n" "c$n" & n=$((n + 1)); done
  sleep 1; cout=$(cd "$root/$mode-wt1" && BEADS_DIR="$rig/.beads" bd --no-daemon close t-1 2>&1); crc=$?; wait
  st=$(cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon show t-1 --json 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["status"])')
  echo "  close rc=$crc printed=\"$(printf '%s' "$cout" | head -1 | cut -c1-60)\" status-after=$st"
  census "$rig"
done

echo "== long description, serial (shape 2): one create each size"
for mode in nodb db; do
  rig="$root/$mode"
  for sz in 20000 60000 70000; do
    python3 -c "import sys; print('d'*int(sys.argv[1]))" "$sz" >"$root/desc.txt"
    out=$(cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon create "long-$mode-$sz" -d "$(cat "$root/desc.txt")" 2>&1); rc=$?
    id=$(printf '%s\n' "$out" | sed -n 's/.*Created issue: \([^ ]*\).*/\1/p' | head -1)
    rb=$(cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon show "${id:-none}" --json 2>&1 | head -c 80 | tr '\n' ' ')
    echo "  $mode $sz B: rc=$rc id=${id:--} first-line=\"$(printf '%s' "$out" | head -1 | cut -c1-50)\" readback=\"$rb\""
  done
done

echo "== lost update of EXISTING records: $W writers x $K comments on t-2, and a close of in_progress t-1 mid-run"
for mode in nodb db; do
  rig="$root/$mode-u"; mkrig "$rig" $([ "$mode" = nodb ] && echo nodb)
  (cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon list --json >/dev/null 2>&1)
  n=1; while [ "$n" -le "$W" ]; do git -C "$rig" worktree add -q "$rig-wt$n" 2>/dev/null; n=$((n + 1)); done
  : >"$root/$mode-u.crc"
  n=1; while [ "$n" -le "$W" ]; do
    ( i=0; while [ "$i" -lt "$K" ]; do i=$((i + 1))
        (cd "$rig-wt$n" && BEADS_DIR="$rig/.beads" bd --no-daemon comments add t-2 "c$n-$i" >/dev/null 2>&1); echo $? >>"$root/$mode-u.crc"
      done ) & n=$((n + 1)); done
  sleep 1; (cd "$rig-wt1" && BEADS_DIR="$rig/.beads" bd --no-daemon close t-1 >/dev/null 2>&1); crc=$?; wait
  got=$(cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon comments t-2 --json 2>/dev/null | python3 -c 'import json,sys; print(len(json.load(sys.stdin) or []))')
  st=$(cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon show t-1 --json 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["status"])')
  echo "  $mode: comments exit0 $(grep -c '^0$' "$root/$mode-u.crc")/$((W * K)), on t-2 after: $got;  close t-1 rc=$crc, status after: $st"
done

echo "== mixed: a db-mode store, half the writers --no-db (a cage seat's CageBdFlags), half SQLite"
rig="$root/mixed"; mkrig "$rig"
(cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon list --json >/dev/null 2>&1)
n=1; while [ "$n" -le "$W" ]; do git -C "$rig" worktree add -q "$rig-wt$n" 2>/dev/null; n=$((n + 1)); done
: >"$root/mixed.printed"
n=1; while [ "$n" -le "$W" ]; do
  ( if [ $((n % 2)) -eq 0 ]; then fl=--no-db; else fl=; fi; i=0
    while [ "$i" -lt "$K" ]; do i=$((i + 1))
      out=$(cd "$rig-wt$n" && BEADS_DIR="$rig/.beads" bd --no-daemon $fl create "m$n-$i" 2>&1); rc=$?
      id=$(printf '%s\n' "$out" | sed -n 's/.*Created issue: \([^ ]*\).*/\1/p' | head -1)
      echo "$rc ${id:--} m$n-$i" >>"$root/mixed.printed"
    done ) & n=$((n + 1)); done; wait
echo "  read through SQLite:"; census "$rig"
echo "  read --no-db (what a cage seat sees):"
(cd "$rig" && BEADS_DIR="$rig/.beads" bd --no-daemon --no-db list --all --json --limit 0 2>/dev/null) | python3 -c 'import json,sys; print("   rows", len(json.load(sys.stdin) or []))'
