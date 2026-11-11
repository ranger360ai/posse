#!/usr/bin/env bash
# reap-timer-unit.sh — render the systemd user timer that runs
# scripts/reap-test-tmp.sh (ranger-base-y9t65, the operator's accepted-risk
# mitigation on ranger-base-l23nb).
#
# Usage:
#       scripts/reap-timer-unit.sh            print both units to stdout
#       scripts/reap-timer-unit.sh --write    put them in the user unit dir
#       scripts/reap-timer-unit.sh --interval 15min
#       scripts/reap-timer-unit.sh --script PATH   the reaper to run
#       scripts/reap-timer-unit.sh --root DIR      repeatable, default /tmp
#       scripts/reap-timer-unit.sh --name NAME     unit basename
#
# IT RENDERS AND IT NEVER ENABLES, which is `posse systemd`'s contract and
# ADR 0080 D5's rule, kept here because the reason is the same: enabling a
# unit is a change to a deployed system, so this prints the `systemctl
# --user` lines instead of running them. It is a shell script and not a
# `posse` verb because nothing about it tracks a home: `posse systemd`
# refuses outright unless the home says `autostart_via: systemd`, and this
# timer has nothing to do with autostart or with any home — it is one
# per BOX, because the thing it sweeps is one shared filesystem.
#
# ─── WHY 15 MINUTES ─────────────────────────────────────────────────────────
#
# Not from taste. The reap costs 2.1s wall over 9689 top-level entries and 61
# processes (MEASURED 2026-11-11 on this box), so the cadence is bounded by
# how fast a BURST has to be noticed, not by what a run costs.
#
#   - The steady leak rate does not set it. MEASURED 2026-11-11 over
#     /tmp's own mtimes: 465-928 `tmp.*` dirs per hour, ~4.6 inodes each,
#     so 2100-4300 inodes/h against 945k free — 220 to 450 hours of
#     headroom. A daily sweep would hold that.
#   - A BURST sets it. ranger-base-b27pn priced ONE arm-2 run at 310 dirs
#     and 12200 inodes, and five seats run suites concurrently here, so the
#     worst case is ~183000 inodes/h. From the pressure line (70%, 734k) to
#     a full table (1048576) is 314k inodes: 103 minutes.
#   - The reap can only take what is past its class grace, so during a
#     burst what actually matters is how soon the PRESSURE arm notices and
#     drops every grace to 60m. 15 minutes puts 6-7 checks inside that
#     103-minute window, at 2.1s each — a duty cycle of 0.23%.
#
# ─── WHY IT IS OFF THE DISPATCH LOOP ────────────────────────────────────────
#
# The bead's requirement, and it takes four things, not one:
#   - It is its OWN unit. It is not called from `dispatch --watch` and it
#     carries no After=/Requires= on the dispatch units, so neither a
#     dispatch restart nor a reap failure can reach the other. (A reap also
#     needs no multiplexer, which is the whole reason the dispatch unit is
#     ordered behind one.)
#   - RandomizedDelaySec is LOAD-BEARING here, not decoration. The two homes
#     on this box pass at 5m (ranger) and 3m (weller), and 15 is a multiple
#     of both — so an unrandomized 15-minute timer would start on a dispatch
#     tick every single time it fired, which is the one thing the bead asked
#     against.
#   - Nice=10 and IOSchedulingClass=idle, so the overlaps that do happen
#     yield to the pass and to anybody's suite.
#   - Type=oneshot with no Restart=. A failed reap has failed; the next one
#     is 15 minutes away. A restarting sweep on a shared filesystem is how
#     a bad run becomes a bad hour.
#
# ─── WHAT THE UNIT RUNS, AND WHEN THAT CHANGES ──────────────────────────────
#
# ExecStart names the script IN THE CHECKOUT, not a promoted copy. So a
# landing that changes scripts/reap-test-tmp.sh changes what this timer runs,
# with no promote step in between — unlike the dispatch unit, which names a
# promoted `~/.local/bin/posse`. That is deliberate and it is the trade: a
# frozen copy under ~/.local/libexec would need a promote verb nobody has,
# and would go stale silently, which on a sweep tool means the box quietly
# keeps running the version whose gates were wrong. The gate on this script
# is therefore the gate every landing has — review and `make tree-check` —
# plus the script's own dry-run default. `--script PATH` renders against a
# frozen copy instead for an operator who wants the other trade.
set -euo pipefail

self=$(basename "$0")
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)

write=0 interval=15min name=posse-reap-tmp script="$here/reap-test-tmp.sh" roots=()
unitdir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

die2() { echo "$self: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --write)    write=1; shift ;;
    --interval) interval="${2:?--interval needs a systemd time span}"; shift 2 ;;
    --script)   script="${2:?--script needs a path}"; shift 2 ;;
    --root)     roots+=("${2:?--root needs a directory}"); shift 2 ;;
    --name)     name="${2:?--name needs a unit basename}"; shift 2 ;;
    --unit-dir) unitdir="${2:?--unit-dir needs a directory}"; shift 2 ;;
    -h|--help)  awk 'NR>1 && /^#/ {sub(/^# ?/, ""); print; next} NR>1 {exit}' "$0"; exit 0 ;;
    *) die2 "unknown argument: $1" ;;
  esac
done
[ ${#roots[@]} -eq 0 ] && roots=(/tmp)

case "$name" in *[!a-zA-Z0-9_.-]*|'') die2 "unit basename '$name' is not a unit name" ;; esac
case "$interval" in *[!a-zA-Z0-9]*|'') die2 "--interval '$interval' is not a systemd time span" ;; esac
[ -x "$script" ] || die2 "$script is not an executable file — name the reaper with --script"
script=$(cd "$(dirname "$script")" && pwd)/$(basename "$script")
for r in "${roots[@]}"; do
  case "$r" in /*) ;; *) die2 "--root $r is not absolute; systemd expands nothing" ;; esac
  case "$r" in *[[:space:]%]*) die2 "--root $r carries whitespace or a %, which systemd would re-read" ;; esac
done

# A USER SERVICE DOES NOT INHERIT YOUR SHELL'S PATH — a user manager reads no
# profile, and its minimal PATH does not include ~/.local/bin, which is where
# INSTALL puts the tracker CLI the bead arm needs (ranger-base-ib61s is the
# same lesson on the dispatch unit: without this the loop exited 'not found
# on PATH' every RestartSec, forever). So this is THE PATH THIS RENDER RAN
# WITH, with every posse gates shim dir dropped — those are per-seat walls
# (ADR 0017) and a unit is not a seat — and the system dirs appended. Read at
# render, so re-render after anything moves.
render_path() {
  local out="" d
  while IFS= read -r d; do
    [ -n "$d" ] || continue
    case "$d" in */state/gates/*) continue ;; esac
    case ":$out:" in *":$d:"*) continue ;; esac
    out="${out:+$out:}$d"
  done < <(printf '%s\n' "$PATH" | tr ':' '\n')
  for d in /usr/local/sbin /usr/local/bin /usr/sbin /usr/bin /sbin /bin; do
    case ":$out:" in *":$d:"*) continue ;; esac
    out="${out:+$out:}$d"
  done
  printf '%s' "$out"
}
unitpath=$(render_path)
case "$unitpath" in *%*) die2 "the rendered PATH carries a %, which systemd expands where it lands: $unitpath" ;; esac

rootargs=""
for r in "${roots[@]}"; do rootargs="$rootargs --root $r"; done

# A SESSION WORKTREE IS NOT A PLACE TO POINT A TIMER AT. Every path below is
# resolved from where THIS script sits, which is right for an operator
# running it out of the shared checkout and wrong for a dispatched session,
# whose tree is fast-forwarded onto main and then retired — the unit would
# survive the directory it names and fail every interval with 'No such file
# or directory'. Named and not refused, because rendering it to stdout from
# a worktree is exactly how it gets reviewed before anyone writes it.
notes=""
case "$repo" in
  */.posse/worktrees/*) notes="$repo is a session worktree, which is retired once its bead closes. The paths below point into it. Re-render from the shared checkout before writing or enabling anything." ;;
esac
[ -n "$notes" ] && echo "$self: $notes" >&2
[ -n "$notes" ] && [ "$write" -eq 1 ] && die2 "refusing --write from a session worktree — re-render from the shared checkout"

service_body=$(cat <<EOF
# posse: reap the scratch dirs a killed run leaves behind in a shared /tmp.
#
# RENDERED BY \`scripts/reap-timer-unit.sh\` (ranger-base-y9t65) — do not
# hand-edit: re-render it, so the paths and the PATH keep tracking the
# checkout. Whoever rendered this did NOT enable it; that is the operator's,
# and the render printed the systemctl lines.
#
# BLAST RADIUS, every ${interval}: an \`rm -rf\` of top-level directories under
# ${roots[*]} that match one of the reaper's class globs, are owned by this
# uid, are named by no live process, and are older than their class's grace.
# It never descends, it never touches a file, and the two classes it refuses
# on purpose (the harness's claude-* scratchpad roots and node-compile-cache)
# are named with their reasons in the script's header.
#
# IT RUNS THE SCRIPT IN THE CHECKOUT, not a promoted copy — a landing that
# changes ${script#$repo/} changes what this timer does, with no promote
# step. See the renderer's header for why that is the trade.
#
# The unit carries no After=/Requires= on the dispatch units on purpose: a
# reap needs no multiplexer, and coupling them would let either one's
# restart reach the other.
[Unit]
Description=posse: reap leaked scratch dirs from ${roots[*]} (ranger-base-y9t65)
Documentation=file://$script

[Service]
Type=oneshot
WorkingDirectory=$repo
Environment=PATH=$unitpath
ExecStart=$script$rootargs --apply --quiet --bead-repo $repo
# A sweep yields to everything: it is never the urgent process on this box.
Nice=10
IOSchedulingClass=idle
# No Restart=. A oneshot that fails has failed and the next one is one
# interval away; a sweep that retries on a shared filesystem is how a bad
# run becomes a bad hour. The journal is the record — the script prints its
# before/after df and every class's count on every run.
EOF
)

timer_body=$(cat <<EOF
# posse: the timer for ${name}.service (ranger-base-y9t65).
#
# RENDERED BY \`scripts/reap-timer-unit.sh\` — re-render, do not hand-edit.
#
# WHY ${interval}: the reap costs ~2s, so the cadence is set by how fast a
# BURST has to be noticed. A concurrent-suite burst was priced at ~183000
# inodes/h, which is 103 minutes from the 70% pressure line to a full inode
# table; ${interval} puts six or seven checks inside that window. The
# renderer's header has the measurements.
#
# RandomizedDelaySec IS LOAD-BEARING, not politeness: this box's two homes
# dispatch at 5m and 3m, and 15 is a multiple of both, so a fixed 15-minute
# timer would fire on a dispatch tick every time. The jitter is what keeps
# the reap off the pass.
#
# No Persistent=: that key only applies to OnCalendar= timers. This one is
# monotonic, so after a suspend or a reboot OnBootSec= brings the first run
# and OnUnitActiveSec= takes over — there is no missed run to catch up.
[Unit]
Description=posse: reap leaked scratch dirs every ${interval} (ranger-base-y9t65)

[Timer]
Unit=${name}.service
OnBootSec=10min
OnUnitActiveSec=${interval}
RandomizedDelaySec=2min
AccuracySec=30s

[Install]
WantedBy=timers.target
EOF
)

if [ "$write" -eq 0 ]; then
  echo "# ===== $unitdir/${name}.service"
  printf '%s\n' "$service_body"
  echo
  echo "# ===== $unitdir/${name}.timer"
  printf '%s\n' "$timer_body"
  echo
  echo "# ===== not written. \`$self --write\` puts both files there." >&2
fi

if [ "$write" -eq 1 ]; then
  mkdir -p "$unitdir"
  printf '%s\n' "$service_body" > "$unitdir/${name}.service"
  printf '%s\n' "$timer_body"   > "$unitdir/${name}.timer"
  echo "wrote $unitdir/${name}.service"
  echo "wrote $unitdir/${name}.timer"
fi

# Printed on BOTH paths, because the lines are the deliverable: a rendered
# unit nobody enables does nothing, and this script will not enable it.
cat <<EOF

this renders the unit and does NOT enable it — that is the operator's:

  # read what it would take first; this is the exact command the unit runs,
  # minus the --apply, so it deletes nothing:
  $script$rootargs --bead-repo $repo

  # then arm it:
  systemctl --user daemon-reload
  systemctl --user enable --now ${name}.timer
  systemctl --user list-timers ${name}.timer

  # and read the first run back:
  systemctl --user start ${name}.service
  journalctl --user -u ${name}.service -n 40 --no-pager

  # to stand it down again:
  systemctl --user disable --now ${name}.timer
EOF
