#!/bin/bash
# ADR 0059 probe — packed-refs.lock as a WRITABLE subpath (create + unlink) of a
# session worktree's L2 grant, packed-refs and packed-refs.new still denied.
#
# Every arm runs on a FRESH fixture in the fleet's shape (refs packed, branch
# posse/x two directories deep, a sibling worktree), under sandbox-exec with a
# hand-rendered profile that matches SeatbeltProfile's shape for a dispatched
# tree. CANDIDATE=0 is the shipped grant (the control); CANDIDATE=1 adds the one
# line this ADR decides. Numbers in the ADR's MEASURED table come from here
# (2026-09-11, darwin 25.4.0, git 2.50.1).
#
#   ./0059-packed-refs-lock.probe.sh            # both arms, every probe
#   CANDIDATE=1 ./0059-packed-refs-lock.probe.sh kill   # one arm, one probe
#
# Fixture commits are path-limited because a crew PID denies the bare form
# even in a scratch repo (ORDERS, lwd29).
set -u
SP="${PROBE_ROOT:-$(mktemp -d "${TMPDIR:-/tmp}/adr0059.XXXXXX")}"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
unset GIT_EXTERNAL_DIFF GIT_DIR GIT_WORK_TREE
N=0
fixture() {   # $1 = arm name ; sets ROOT REPO TREE OWN COMMON LOCK PROF CLEAN CONF
  N=$((N+1)); ROOT="$SP/fx/$N-$1"; rm -rf "$ROOT"; mkdir -p "$ROOT/home" "$ROOT/tmp"
  export HOME="$ROOT/home" TMPDIR="$ROOT/tmp"
  REPO="$ROOT/repo"; mkdir -p "$REPO"
  git -C "$REPO" init -q -b main . ; git -C "$REPO" config user.email t@example.com; git -C "$REPO" config user.name t
  echo seed > "$REPO/README.md"; git -C "$REPO" add README.md; git -C "$REPO" commit -q -m seed -- README.md
  echo "main line" > "$REPO/clean.txt"; git -C "$REPO" add clean.txt; git -C "$REPO" commit -q -m clean-on-main -- clean.txt
  echo "main says A" > "$REPO/conflict.txt"; git -C "$REPO" add conflict.txt; git -C "$REPO" commit -q -m conflict-on-main -- conflict.txt
  git -C "$REPO" pack-refs --all
  TREE="$ROOT/trees/x"
  git -C "$REPO" worktree add -q -b posse/x "$TREE" main~2
  git -C "$REPO" worktree add -q -b posse/other "$ROOT/trees/other" main~2
  echo "session says B" > "$TREE/conflict.txt"; git -C "$TREE" add conflict.txt; git -C "$TREE" commit -q -m session-conflict -- conflict.txt
  OWN="$(cd "$TREE" && git rev-parse --git-dir)"; OWN="$(cd "$OWN" && pwd -P)"
  COMMON="$(cd "$TREE" && git rev-parse --git-common-dir)"; COMMON="$(cd "$COMMON" && pwd -P)"
  LOCK="$COMMON/packed-refs.lock"
  CLEAN="$(git -C "$REPO" rev-parse main~1)"; CONF="$(git -C "$REPO" rev-parse main)"
  PROF="$ROOT/prof.sb"
  { echo '(version 1)'; echo '(allow default)'; echo '(deny file-write*)'; echo '(allow file-write*'
    for p in "$TREE" "$OWN" "$COMMON/objects" "$COMMON/logs" "$COMMON/refs/heads/posse/x" "$COMMON/refs/heads/posse/x.lock" "$ROOT/home" "$ROOT/tmp"; do echo "  (subpath \"$p\")"; done
    [ "${CANDIDATE:-0}" = 1 ] && echo "  (subpath \"$LOCK\")   ; THE CANDIDATE (ADR 0059 D1)"
    echo '  (regex #"^/dev/")'; echo '  (literal "/dev/null")'; echo ')'
    echo '(allow file-write-create'; echo "  (literal \"$COMMON/refs/heads/posse\")"; echo ')'
  } > "$PROF"
}
sb() { sandbox-exec -f "$PROF" /bin/sh -c "$1" 2>&1; echo "rc=$?"; }
state() {
  local m=""; for f in CHERRY_PICK_HEAD REVERT_HEAD MERGE_HEAD sequencer rebase-merge rebase-apply AUTO_MERGE; do [ -e "$OWN/$f" ] && m="$m $f"; done
  echo "   markers:[${m# }] lock:$([ -e "$LOCK" ] && echo PRESENT || echo absent) packed-refs:$([ -e "$COMMON/packed-refs" ] && echo present || echo ABSENT)"
}
hdr() { echo; echo "=== $* (CANDIDATE=${CANDIDATE:-0})"; }
ind() { sed 's|^|   |'; }
hook_install() { # $1 = lock path to watch; the hook logs every event and holds the window when that lock is held
  mkdir -p "$COMMON/hooks"; cat > "$COMMON/hooks/reference-transaction" <<H
#!/bin/sh
echo "\$(date +%s.%N | cut -c1-17) \$1 pid=\$PPID lock=\$([ -e $1 ] && echo P || echo -)" >> $ROOT/tmp/hook.log
[ "\$1" = prepared ] && [ -e $1 ] && [ -n "\${HOLD:-}" ] && sleep 3
exit 0
H
  chmod +x "$COMMON/hooks/reference-transaction"
}

probe_sequencer() {
  hdr "D2.1a clean cherry-pick (never conflicts)"; fixture seq-clean
  sb "git -C $TREE cherry-pick $CLEAN" | ind; state
  hdr "D2.1b conflicted cherry-pick, --abort, then the path-limited commit the PID allows"; fixture seq-abort
  sb "git -C $TREE cherry-pick $CONF" | tail -1 | ind; state
  sb "git -C $TREE cherry-pick --abort" | ind; state
  sb "echo after >> $TREE/clean2.txt && git -C $TREE add clean2.txt && git -C $TREE commit -q -m after -- clean2.txt" | ind; state
  hdr "D2.1c conflicted cherry-pick then --quit"; fixture seq-quit
  sb "git -C $TREE cherry-pick $CONF" >/dev/null; sb "git -C $TREE cherry-pick --quit" | ind; state
  hdr "D2.1d conflicted cherry-pick, resolve, --continue"; fixture seq-cont
  sb "git -C $TREE cherry-pick $CONF" >/dev/null
  sb "echo resolved > $TREE/conflict.txt && git -C $TREE add conflict.txt && GIT_EDITOR=true git -C $TREE cherry-pick --continue" | ind; state
  hdr "D2.1e conflicted revert then --abort"; fixture seq-revert
  sb "echo later >> $TREE/conflict.txt && git -C $TREE commit -q -m later -- conflict.txt" >/dev/null
  sb "git -C $TREE revert --no-edit HEAD~1" | tail -1 | ind; state
  sb "git -C $TREE revert --abort" | ind; state
  hdr "D2.1f conflicted rebase then --abort"; fixture seq-rebase
  sb "git -C $TREE rebase $CONF" | tail -1 | ind; state
  sb "git -C $TREE rebase --abort" | ind; state
}
probe_commit() {
  hdr "D2.2 ordinary path-limited commits, one after an operator pack-refs"; fixture commit
  sb "echo one >> $TREE/conflict.txt && git -C $TREE commit -q -m one -- conflict.txt" | ind; state
  git -C "$REPO" pack-refs --all
  sb "echo two >> $TREE/conflict.txt && git -C $TREE commit -q -m two -- conflict.txt" | ind; state
}
probe_rewrite() {
  hdr "D2.3 git WANTS to rewrite packed-refs: delete a PACKED ref via the main checkout"; fixture rewrite
  git -C "$REPO" pack-refs --all; before="$(shasum < "$COMMON/packed-refs")"
  sb "git -C $REPO update-ref -d refs/heads/posse/x" | ind; state
  echo "   packed-refs unchanged: $([ "$before" = "$(shasum < "$COMMON/packed-refs")" ] && echo yes || echo NO); loose copy exists: $([ -e "$COMMON/refs/heads/posse/x" ] && echo yes || echo no)"
}
probe_plant() {
  hdr "Q4 session plants the lock and sits on it; operator pack-refs / gc; session rm"; fixture plant
  sb "touch $LOCK" | ind; state
  if [ -e "$LOCK" ]; then
    git -C "$REPO" pack-refs --all 2>&1 | head -1 | sed 's|^|   op pack-refs: |'; echo "   op pack-refs rc=${PIPESTATUS[0]}"
    git -C "$REPO" gc -q 2>&1 | tail -1 | sed 's|^|   op gc: |'; echo "   op gc rc=${PIPESTATUS[0]}"; state
    sb "rm $LOCK" | sed 's|^|   session rm: |'; state
  fi
  hdr "Q4 control: the hand a session ALREADY has — its own ref .lock planted"; fixture plant-ref
  sb "touch $COMMON/refs/heads/posse/x.lock" | ind
  git -C "$REPO" pack-refs --all 2>&1 | head -1 | sed 's|^|   op pack-refs: |'; echo "   op pack-refs rc=${PIPESTATUS[0]}"
  git -C "$REPO" gc -q 2>&1 | tail -1 | sed 's|^|   op gc: |'; echo "   op gc rc=${PIPESTATUS[0]}"
}
probe_aliases() {
  hdr "D2.4 the lock name as a route to anything else"; fixture alias
  before="$(shasum < "$COMMON/packed-refs")"
  sb "ln $COMMON/packed-refs $LOCK && echo HARDLINK-MADE" | sed 's|^|   hardlink packed-refs -> lock: |'; rm -f "$LOCK"
  sb "ln -s $COMMON/packed-refs $LOCK && echo symlink-made" | sed 's|^|   symlink at lock: |'
  sb "echo junk >> $LOCK" | sed 's|^|   write through the symlink: |'
  echo "   packed-refs unchanged: $([ "$before" = "$(shasum < "$COMMON/packed-refs")" ] && echo yes || echo NO-CORRUPTED)"; rm -f "$LOCK"
  sb "touch $LOCK && mv $LOCK $COMMON/packed-refs && echo RENAMED" | sed 's|^|   rename lock -> packed-refs: |'; rm -f "$LOCK"
  sb "touch $LOCK && mv $LOCK $COMMON/refs/heads/main && echo RENAMED" | sed 's|^|   rename lock -> refs/heads/main: |'; rm -f "$LOCK"
  sb "mv $COMMON/packed-refs $LOCK && echo MOVED" | sed 's|^|   rename packed-refs -> lock: |'
  sb "echo x > $COMMON/packed-refs.new" | sed 's|^|   write packed-refs.new: |'
  echo "   packed-refs still present: $([ -e "$COMMON/packed-refs" ] && echo yes || echo NO)"
}
probe_kill() { # signal the git process that HOLDS packed-refs.lock, mid-abort
  for sig in TERM HUP KILL; do
    hdr "Q2 [$sig] to the git process holding packed-refs.lock during cherry-pick --abort"; fixture "kill-$sig"
    hook_install "$LOCK"; export HOLD=1
    sb "git -C $TREE cherry-pick $CONF" >/dev/null; rm -f "$ROOT/tmp/hook.log"
    sandbox-exec -f "$PROF" /bin/sh -c "git -C $TREE cherry-pick --abort & echo \$! > $ROOT/tmp/pid; wait" >/dev/null 2>&1 &
    bg=$!; for i in $(seq 1 60); do grep -q "prepared.*lock=P" "$ROOT/tmp/hook.log" 2>/dev/null && break; sleep 0.1; done
    holder="$(grep 'prepared.*lock=P' "$ROOT/tmp/hook.log" | head -1 | sed 's/.*pid=\([0-9]*\).*/\1/')"
    if [ -z "$holder" ]; then echo "   no transaction ever held the lock (expected on the shipped grant)"; wait $bg 2>/dev/null; state; unset HOLD; continue; fi
    kill -$sig "$holder"; echo "   kill -$sig holder $holder rc=$?"; wait $bg 2>/dev/null; sleep 4
    echo "   holder alive after settle: $(ps -p "$holder" -o pid= >/dev/null && echo YES || echo no)"; state; unset HOLD
  done
  hdr "Q2 control: killing the cherry-pick FRONTEND leaves the worker to finish"; fixture kill-frontend
  hook_install "$LOCK"; export HOLD=1
  sb "git -C $TREE cherry-pick $CONF" >/dev/null; rm -f "$ROOT/tmp/hook.log"
  sandbox-exec -f "$PROF" /bin/sh -c "git -C $TREE cherry-pick --abort & echo \$! > $ROOT/tmp/pid; wait" >/dev/null 2>&1 &
  bg=$!; for i in $(seq 1 60); do grep -q "prepared.*lock=P" "$ROOT/tmp/hook.log" 2>/dev/null && break; sleep 0.1; done
  if grep -q "prepared.*lock=P" "$ROOT/tmp/hook.log" 2>/dev/null; then
    kill -KILL "$(cat "$ROOT/tmp/pid")"; echo "   kill -KILL frontend rc=$?"; wait $bg 2>/dev/null; sleep 12; state
  else wait $bg 2>/dev/null; echo "   (shipped grant: no lock window to kill inside)"; state; fi
  unset HOLD
}
probe_window() {
  hdr "the window: how long one cherry-pick --abort holds packed-refs.lock (hook only logs)"; fixture window
  hook_install "$LOCK"; unset HOLD
  for i in 1 2 3 4 5; do
    sb "git -C $TREE cherry-pick $CONF" >/dev/null; rm -f "$ROOT/tmp/hook.log"
    sb "git -C $TREE cherry-pick --abort" >/dev/null
    first=$(grep -m1 "lock=P" "$ROOT/tmp/hook.log" | cut -d' ' -f1); last=$(grep "lock=P" "$ROOT/tmp/hook.log" | tail -1 | cut -d' ' -f1)
    [ -n "$first" ] && echo "   run $i: held span $(echo "$last - $first" | bc)s over $(grep -c 'lock=P' "$ROOT/tmp/hook.log") events" || echo "   run $i: never held (shipped grant)"
  done
}
probe_reflock_control() {
  hdr "Q2 control on the SHIPPED class: SIGKILL a plain commit while it holds refs/heads/posse/x.lock"; fixture reflock
  RL="$COMMON/refs/heads/posse/x.lock"; hook_install "$RL"; export HOLD=1
  rm -f "$ROOT/tmp/hook.log"
  sandbox-exec -f "$PROF" /bin/sh -c "echo x >> $TREE/conflict.txt; git -C $TREE commit -q -m k -- conflict.txt 2>/dev/null & echo \$! > $ROOT/tmp/pid; wait" >/dev/null 2>&1 &
  bg=$!; for i in $(seq 1 60); do grep -q "prepared.*lock=P" "$ROOT/tmp/hook.log" 2>/dev/null && break; sleep 0.1; done
  holder="$(grep 'prepared.*lock=P' "$ROOT/tmp/hook.log" | head -1 | sed 's/.*pid=\([0-9]*\).*/\1/')"
  kill -KILL "$holder"; echo "   kill -KILL holder rc=$?"; wait $bg 2>/dev/null; sleep 1; unset HOLD
  echo "   ref lock after SIGKILL: $([ -e "$RL" ] && echo STRANDED || echo absent); index.lock: $([ -e "$OWN/index.lock" ] && echo STRANDED || echo absent)"
  rm -f "$COMMON/hooks/reference-transaction"
  git -C "$REPO" gc -q 2>&1 | tail -1 | sed 's|^|   op gc: |'; echo "   op gc rc=${PIPESTATUS[0]}"
}
probe_stray() {
  hdr "a stray lock from ANYONE, then the session's --abort (71g2f's arm is the floor here)"; fixture stray
  sb "git -C $TREE cherry-pick $CONF" >/dev/null; touch "$LOCK"
  s=$(date +%s.%N); sb "git -C $TREE cherry-pick --abort" | grep -c "File exists" | sed 's|^|   "File exists" lines: |'; e=$(date +%s.%N)
  echo "   abort wall: $(echo "$e - $s" | bc)s"; state; rm -f "$LOCK"
}

want="${1:-all}"
run() { case "$want" in all|"$1") "probe_$1" ;; esac; }
for c in ${ARMS:-0 1}; do
  export CANDIDATE=$c
  run sequencer; run commit; run rewrite; run plant; run aliases; run kill; run window; run reflock_control; run stray
done
echo; echo "fixtures under $SP"
