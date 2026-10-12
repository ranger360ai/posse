#!/bin/sh
# ADR 0074 probe — does a SQLite-mode bd 0.50.3 (--no-daemon, as posse pins it)
# ever take in a no-db write it did not race, and what does its next flush do?
# Usage: sh docs/adr/0074-cage-nodb-into-sqlite.probe.sh <empty scratch dir>
# One writer, no concurrency, a sleep between steps. The no-db side is the
# cage seat's argv (CageBdFlags); the SQLite side is the host's. The rig is
# hand-written in ADR 0072/0073's shape (no `bd init`); the live store of
# record is never touched.
set -u
root=${1:?usage: $0 <empty scratch dir>}
rig=$root/rig
mkdir -p "$rig/.beads" || exit 2
python3 - "$rig" <<'EOF'
import json, os, subprocess, sys
d = sys.argv[1]
subprocess.run(["git", "init", "-q", d], check=True)
now = "2026-10-12T12:00:00Z"
with open(os.path.join(d, ".beads/issues.jsonl"), "w") as f:
    for i in (1, 2):
        f.write(json.dumps({"id": f"t-{i}", "title": f"seed {i}", "description": "seed", "status": "in_progress",
            "priority": 2, "issue_type": "task", "created_at": now, "updated_at": now, "created_by": "rig",
            "owner": "rig", "labels": [], "dependencies": [], "comments": []}, separators=(",", ":")) + "\n")
open(os.path.join(d, ".beads/config.yaml"), "w").write('issue-prefix: "t"\n')
# Tracked, like a real store: with an untracked .beads, `bd sync` exits 1 and
# neither flushes nor imports, and steps D-F then measure that instead.
subprocess.run(["git", "-C", d, "add", "--", ".beads"], check=True)
subprocess.run(["git", "-C", d, "-c", "user.name=r", "-c", "user.email=r@r", "commit", "-qm", "rig", "--", ".beads"], check=True)
EOF
cd "$rig" || exit 2
export BEADS_DIR="$rig/.beads"
has() { grep -c "\"title\":\"$1\"" .beads/issues.jsonl; }
sees() { bd --no-daemon list --all --json --limit 0 2>/dev/null |
  python3 -c 'import json,sys; t=sys.argv[1]; print(sum(1 for i in json.load(sys.stdin) or [] if i["title"]==t))' "$1"; }
bd --no-daemon list --json >/dev/null 2>&1; sleep 1   # first read builds beads.db
bd --no-daemon --no-db create n1 -d x >/dev/null 2>&1
echo "A cage-side --no-db create n1, rc=$?: in jsonl=$(has n1)"
sleep 2
echo "B host SQLite read 2s later: SQLite sees n1=$(sees n1), jsonl still has n1=$(has n1)"
bd --no-daemon create s1 -d x >/dev/null 2>&1
echo "C host SQLite create s1: jsonl n1=$(has n1) s1=$(has s1) (export not yet run), SQLite sees n1=$(sees n1)"
bd --no-daemon sync --flush-only >/dev/null 2>&1
echo "D host sync --flush-only (a flush; bd's pre-commit hook flushes too, rangerhq-be7k): jsonl n1=$(has n1) s1=$(has s1)"
bd --no-daemon --no-db create n2 -d x >/dev/null 2>&1; sleep 2
bd --no-daemon sync --import-only >/dev/null 2>&1
echo "E --no-db create n2, then host sync --import-only rc=$?: SQLite sees n2=$(sees n2)"
bd --no-daemon create s2 -d x >/dev/null 2>&1; bd --no-daemon sync --flush-only >/dev/null 2>&1
echo "F host create s2 + flush after that import: jsonl n2=$(has n2) s2=$(has s2)"
