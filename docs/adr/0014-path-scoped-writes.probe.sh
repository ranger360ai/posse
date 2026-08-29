#!/bin/sh
# Path-scoped writes at the container tier (ADR 0014 §4) — the measurement
# that ADR left open. §4 shipped with "Overlapping binds (later mount wins)
# are **ASSUMED** — the Docker probe was not run this session", and named
# this probe as the L4 bead's done-when: if a shape fails, that bead files
# DIVERGED: and path-scoped rules stay unrealized at `container` rather than
# claiming a wall the engine does not hold.
#
#   sh docs/adr/0014-path-scoped-writes.probe.sh
#
# Run from anywhere; every artifact it makes is under one mktemp -d and it
# removes them. It writes nothing to the repo, needs no image build (busybox
# is enough — none of this is about the runtime) and spends no API turn.
#
# MEASURED 2026-08-29, macOS 26.4.1, Docker Desktop engine 29.0.1 (VirtioFS),
# bead ranger-base-yu5: all seven probes as written below.
#
# Read the VALUES, not the exit status. Every probe carries its own CONTROL —
# the same tree with the overlay taken away — because a refusal proves
# nothing unless the arm without the wall succeeds. Two of the seven are
# there only as controls and one is a negative: a probe with no failing wrong
# arm measures the fixture, not the engine.
set -e

ENGINE=${ENGINE:-docker}
IMG=${IMG:-busybox}
command -v "$ENGINE" >/dev/null 2>&1 || {
  echo "no $ENGINE on PATH — nothing measured (that is not a pass)"; exit 2; }

echo "== engine =="
"$ENGINE" version --format '{{.Server.Version}}' 2>/dev/null || "$ENGINE" --version

WORK=$(mktemp -d)
# Resolved: mounts are same-path in and out, and on macOS mktemp answers
# under /var/folders while the real path is /private/var/folders. A fixture
# that mixed the two spellings would be measuring the symlink. (This is the
# same trap `cageCovering` exists for on the posse side — an overlay spelled
# for the host lands at a path the container never mounted.)
WORK=$(cd "$WORK" && pwd -P)
trap 'chmod -R u+w "$WORK" 2>/dev/null || true; rm -rf "$WORK"' EXIT

# A FRESH tree per probe. A control arm that mutates the shared fixture makes
# every later probe measure the leftovers instead of the engine (measured the
# hard way on ranger-base-h15).
tree() {
  t=$WORK/$1
  rm -rf "$t"
  mkdir -p "$t/internal" "$t/docs/adr" "$t/.beads" "$t/.git"
  echo x > "$t/internal/a.txt"
  echo y > "$t/docs/adr/b.md"
  echo '{"id":"probe"}' > "$t/.beads/issues.jsonl"
  echo "$t"
}

# probe <label> <tree> <mount args...> -- <sh line>
# The sh line prints `key=value` per claim; this prints them under the label.
probe() {
  label=$1; shift
  printf '%s\n' "-- $label"
  "$ENGINE" run --rm "$@" 2>&1 | sed 's/^/   /'
}

# The three questions every shape has to answer, asked the same way each
# time so the arms are comparable: is the denied/overlaid path writable, is
# the rest of the tree writable, is the repo root writable.
LINE='cd "$0"
w() { touch "$1/.probe" 2>/dev/null && echo "$2=writable" || echo "$2=refused"; }
w "$0/docs/adr" subtree
w "$0/internal" rest
touch "$0/.probe" 2>/dev/null && echo "root=writable" || echo "root=refused"'

echo
echo "== 1. CONTROL: repo read-write, no overlay =="
echo "   (if the subtree is not writable HERE, probe 2 measures nothing)"
T=$(tree c1); probe "expect subtree=writable rest=writable root=writable" \
  -v "${T}:${T}" "$IMG" sh -c "$LINE" "$T"

echo
echo "== 2. DENY-LIST shape: repo read-write + :ro overlay of docs/adr =="
echo "   (ADR 0014 §4 bullet 1 — the developer PID's Edit(docs/adr/**))"
T=$(tree c2); probe "expect subtree=refused rest=writable root=writable" \
  -v "${T}:${T}" -v "${T}/docs/adr:${T}/docs/adr:ro" "$IMG" sh -c "$LINE" "$T"

echo
echo "== 3. CONTROL: repo :ro, no overlay =="
echo "   (if the subtree is writable HERE, probe 4 measures nothing)"
T=$(tree c3); probe "expect subtree=refused rest=refused root=refused" \
  -v "${T}:${T}:ro" "$IMG" sh -c "$LINE" "$T"

echo
echo "== 4. ALLOW-LIST shape: repo :ro + read-write overlay of docs/adr =="
echo "   (ADR 0014 §4 bullet 2 — deny: [Edit, Write] plus writable: [docs/adr])"
T=$(tree c4); probe "expect subtree=writable rest=refused root=refused" \
  -v "${T}:${T}:ro" -v "${T}/docs/adr:${T}/docs/adr" "$IMG" sh -c "$LINE" "$T"

echo
echo "== 5. the :ro carve-outs: .beads and .git read-write over a :ro repo =="
echo "   (ADR 0014 §4 — the tier whose point is the personas who may not edit"
echo "    the work is not the tier where they cannot report what they found)"
T=$(tree c5); probe "expect beads=writable git=writable root=refused" \
  -v "${T}:${T}:ro" -v "${T}/.beads:${T}/.beads" -v "${T}/.git:${T}/.git" "$IMG" sh -c '
cd "$0"
echo more >> .beads/issues.jsonl 2>/dev/null && echo "beads=writable" || echo "beads=refused"
touch .git/x 2>/dev/null && echo "git=writable" || echo "git=refused"
touch .probe 2>/dev/null && echo "root=writable" || echo "root=refused"
grep -c . .beads/issues.jsonl | sed "s/^/lines-in-jsonl=/"' "$T"

echo
echo "== 6. mount ORDER: the overlay listed BEFORE the repo it sits on =="
echo "   (if this differs from probe 4, the mount list is order-sensitive and"
echo "    cage.go must sort it; measured: the engine sorts by destination"
echo "    depth, so the deeper bind wins whatever order it was given in)"
T=$(tree c6); probe "expect the same answers as probe 4" \
  -v "${T}/docs/adr:${T}/docs/adr" -v "${T}:${T}:ro" "$IMG" sh -c "$LINE" "$T"

echo
echo "== 7. a :ro overlay whose SOURCE does not exist =="
echo "   (a denied subtree nobody has made yet: the rule is about a path, and"
echo "    mkdir docs/future is exactly what a persona does next. The engine"
echo "    creates the source in the writable parent and the deny holds — which"
echo "    is why cage.go Stat-guards only the read-write direction, where a"
echo "    created source would be a grant of a path nobody wrote.)"
T=$(tree c7); probe "expect future=refused rest=writable" \
  -v "${T}:${T}" -v "${T}/docs/future:${T}/docs/future:ro" "$IMG" sh -c '
cd "$0"
touch docs/future/.probe 2>/dev/null && echo "future=writable" || echo "future=refused"
touch internal/.probe 2>/dev/null && echo "rest=writable" || echo "rest=refused"' "$T"
echo "   host afterwards: $(ls -d "${T}/docs/future" 2>/dev/null || echo 'not created')"

echo
echo "== verdict =="
echo "Probes 2, 4, 5, 6 and 7 are ADR 0014 §4's overlapping-bind assumption."
echo "If any of them disagrees with its expect line on YOUR engine, the L4"
echo "overlays are not realized there: file DIVERGED: on the bead naming what"
echo "the engine did, and leave path-scoped rules unrealized at container."
echo
echo "One trap that is not the engine's, for anyone re-running these lines by"
echo "hand: in ZSH, \"\$R:\$R:ro\" is not what it looks like. \`:r\` is a zsh"
echo "modifier (strip the extension), so the word becomes \$R:\${R}o — docker"
echo "then binds an EMPTY auto-created directory read-WRITE at a destination"
echo "one character off, and the probe reads as 'the engine ignored :ro'."
echo "Spell it \"\${R}:\${R}:ro\". This script is /bin/sh and is not affected."
