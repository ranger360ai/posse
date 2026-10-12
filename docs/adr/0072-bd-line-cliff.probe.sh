#!/bin/sh
# ADR 0072 probe — bd 0.50.3's 64 KB line cliff, reproduced in throwaway rigs.
# Usage: sh docs/adr/0072-bd-line-cliff.probe.sh <empty scratch dir>
# No `bd init` (denied at crew seats): each rig is a git repo with a
# hand-written .beads/issues.jsonl, committed before the first read (ADR 0071).
# BEADS_DIR is bound per rig so the live store of record is never touched.
set -u
root=${1:?usage: $0 <empty scratch dir>}
mkdir -p "$root" || exit 2

mkrig() { # mkrig <dir> <row-2 description bytes> [nodb]
  python3 - "$@" <<'EOF'
import json, os, subprocess, sys
d, size = sys.argv[1], int(sys.argv[2]); nodb = len(sys.argv) > 3
os.makedirs(os.path.join(d, ".beads"), exist_ok=True)
subprocess.run(["git", "init", "-q", d], check=True)
now = "2026-10-12T12:00:00Z"
def row(i, desc):
    return {"id": f"t-{i}", "title": f"row {i}", "description": desc, "status": "open",
            "priority": 2, "issue_type": "task", "created_at": now, "updated_at": now,
            "created_by": "rig", "owner": "rig", "labels": [], "dependencies": [], "comments": []}
with open(os.path.join(d, ".beads/issues.jsonl"), "w") as f:
    for r in (row(1, "small"), row(2, "x" * size), row(3, "small too")):
        f.write(json.dumps(r, separators=(",", ":")) + "\n")
open(os.path.join(d, ".beads/config.yaml"), "w").write('issue-prefix: "t"\n' + ("no-db: true\n" if nodb else ""))
subprocess.run(["git", "-C", d, "add", "--", ".beads"], check=True)
subprocess.run(["git", "-C", d, "-c", "user.name=r", "-c", "user.email=r@r", "commit", "-qm", "rig", "--", ".beads"], check=True)
EOF
}
linelen() { python3 -c "import sys; print(len(open(sys.argv[1],'rb').readlines()[int(sys.argv[2])-1]))" "$1/.beads/issues.jsonl" "$2"; }
bdin() { d=$1; shift; (cd "$d" && BEADS_DIR="$d/.beads" bd --no-daemon "$@" >/dev/null 2>"$d/err.txt"; echo "exit=$? $(grep -v "Dolt\|SQLite data" "$d/err.txt" | head -c 160 | tr "\n" " ")"); }

echo "== 1. the boundary (no-db): line bytes incl. newline -> list --json"
for n in 65291 65292 65293; do
  d="$root/edge-$n"; mkrig "$d" "$n" nodb
  echo "$(linelen "$d" 2) B: $(bdin "$d" list --json)"
done

echo "== 2. crossing write (no-db): row 2 at 60 KB, 6000-byte comment"
d="$root/cross"; mkrig "$d" 60000 nodb
python3 -c "print('y'*6000)" >"$d/body.txt"
echo "comments add: $(bdin "$d" comments add t-2 -f "$d/body.txt")  line now $(linelen "$d" 2) B"
echo "list:        $(bdin "$d" list --json)"
echo "show t-1:    $(bdin "$d" show t-1 --json)"
echo "update -d:   $(bdin "$d" update t-2 -d short)"

echo "== 3. SQLite mode: initial build over 70 KB, then a census append"
for n in 60000 70000; do
  d="$root/db-$n"; mkrig "$d" "$n"
  bdin "$d" list --json >/dev/null
  python3 - "$d/.beads/issues.jsonl" <<'EOF'
import json, sys
p = sys.argv[1]; r = json.loads(open(p).readline()); r["id"] = "t-9"; r["title"] = "appended after the build"
open(p, "a").write(json.dumps(r, separators=(",", ":")) + "\n")
EOF
  sleep 2; touch "$d/.beads/issues.jsonl"   # a plain touch, past the build second: a FUTURE mtime (touch -t 2030…) does not trip the check
  echo "row2=$n list: $(bdin "$d" list --json)"
  (cd "$d" && BEADS_DIR="$d/.beads" bd --no-daemon info --json 2>/dev/null | grep issue_count)
  echo "  sync --import-only: $(bdin "$d" sync --import-only)"
done

echo "== 4. bd escaping: 6000 B (1000 each of <, newline, quote, e-acute, tab) -> line growth"
d="$root/esc"; mkrig "$d" 10 nodb
python3 -c "import sys; sys.stdout.write('<'*1000+'\n'*1000+'\"'*1000+'é'*1000+'\t'*1000)" >"$d/body.txt"
before=$(linelen "$d" 2); bdin "$d" comments add t-2 -f "$d/body.txt" >/dev/null
echo "grew $(( $(linelen "$d" 2) - before )) B; raw '<' kept: $(grep -c '<<<' "$d/.beads/issues.jsonl")"
