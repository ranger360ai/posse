#!/bin/bash
# ADR 0096 (ranger-base-rwet4): which /proc spelling lets a runtime's bwrap
# nest inside D17's base, and what each spelling costs. Run UNCAGED on Linux
# as the posse user; inside a caged seat every arm's own bwrap is the nested
# one and the table reads nothing. No repo writes; one mktemp dir, one
# throwaway `sleep` as the same-uid "host peer" whose knobs arm D writes.
#
#   bash docs/adr/0096-proc-spellings.probe.sh
#
# MEASURED 2026-11-14, D1 host, Linux 7.2.15, bubblewrap 0.13.1: see the ADR's
# table. A cell that reads differently from the ADR reopens D1/D2.
set -u
command -v bwrap >/dev/null || { echo "bwrap: absent"; exit 1; }
T=$(mktemp -d)
mkdir -p "$T/w/deny"
sleep 300 &
HP=$!
trap 'kill "$HP" 2>/dev/null; rm -rf -- "${T:?}"' EXIT
BASE=(--die-with-parent --ro-bind / / --dev-bind /dev /dev --unshare-user
  --bind "$T/w" "$T/w" --ro-bind "$T/w/deny" "$T/w/deny")

inner=$(cat <<EOF
r() { label=\$1; shift; out=\$("\$@" 2>&1); rc=\$?; printf '  %-30s rc=%s %s\n' "\$label" "\$rc" "\$(echo "\$out" | head -1 | cut -c1-80)"; }
r nested-bwrap bwrap --ro-bind / / /bin/true
r nested-bwrap-pidns-proc bwrap --ro-bind / / --dev /dev --unshare-pid --proc /proc /bin/true
r unshare-rm unshare -rm /bin/true
r host-pids-visible sh -c 'ls -d /proc/[0-9]* 2>/dev/null | wc -l'
r kill0-host-peer kill -0 $HP
r proc-root-of-host-peer ls /proc/$HP/root/
r B6-umount-deny-nested unshare -rm umount $T/w/deny
r B6-remount-rw-nested unshare -rm mount -o remount,bind,rw $T/w/deny
r proc-mount sh -c "grep ' /proc ' /proc/self/mountinfo | tail -1 | cut -d' ' -f3,5,6"
EOF
)
arm() { echo "ARM $1"; shift; bwrap "${BASE[@]}" "$@" sh -c "$inner" 2>&1; }
arm "A shipped (no /proc flag)"
arm "B --proc /proc" --proc /proc
arm "C --unshare-pid --proc /proc" --unshare-pid --proc /proc
arm "D --bind /proc /proc" --bind /proc /proc

echo "ARM D knobs of the same-uid host peer (WRITTEN = the kernel allowed it)"
bwrap "${BASE[@]}" --bind /proc /proc sh -c "
w() { if printf '%s' \"\$2\" > /proc/$HP/\$1 2>/dev/null; then echo \"  \$1=WRITTEN\"; else echo \"  \$1=refused\"; fi; }
w oom_score_adj 900; w oom_adj 5; w comm x; w coredump_filter 0x7f; w clear_refs 1
w sched 0; w timerslack_ns 1000; w autogroup 5; w loginuid 0; w mem x
w uid_map '0 1001 1'; w setgroups deny; w timens_offsets 'monotonic 1 0'
for f in /proc/sys/kernel/ns_last_pid /proc/sys/kernel/hostname /proc/sys/fs/binfmt_misc/register /proc/sysrq-trigger; do
  v=\$(head -c 64 \"\$f\" 2>/dev/null); [ -n \"\$v\" ] || v=x
  if printf '%s' \"\$v\" > \"\$f\" 2>/dev/null; then echo \"  \$f=WRITTEN\"; else echo \"  \$f=refused\"; fi
done"
