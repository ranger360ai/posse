#!/usr/bin/env bash
# ranger-base-evx07 — the killer surface of a box that SIGKILLs a walled seat,
# and the two kernel facts the wall-watch census rests on.
#
# Run it from the repo root. Everything here is READ-ONLY except the bwrap
# starts in arm B, which run `stat` and exit.
#
#   bash docs/notes.d/ranger-base-evx07.probe.sh          # arms A, C, D
#   bash docs/notes.d/ranger-base-evx07.probe.sh nsino    # arm B too (~20s)
#
# Each arm prints what it read. The readings this probe took on 2026-11-21 are
# in docs/notes.d/ranger-base-evx07.md; re-run it before quoting them.
set -u
want_ns=${1:-}

echo "=== A. the cgroup/OOM surface: can anything here kill one pid of a seat? ==="
echo "-- this process's cgroup"
cat /proc/self/cgroup
echo "-- memory.max and memory.events up the tree (oom_kill is CUMULATIVE since boot:"
echo "   a zero here rules the kernel/memcg OOM family out far harder than a dmesg grep)"
c=/sys/fs/cgroup$(cut -d: -f3 < /proc/self/cgroup)
while [ "$c" != "/sys/fs/cgroup" ] && [ -n "$c" ]; do
  printf '%-72s max=%-10s %s\n' "$c" "$(cat "$c/memory.max" 2>/dev/null)" \
    "$(tr '\n' ' ' < "$c/memory.events" 2>/dev/null)"
  c=$(dirname "$c")
done
printf '%-72s max=%-10s %s\n' /sys/fs/cgroup "$(cat /sys/fs/cgroup/memory.max 2>/dev/null)" \
  "$(tr '\n' ' ' < /sys/fs/cgroup/memory.events 2>/dev/null)"

echo
echo "-- systemd-oomd: active? and at what thresholds?"
systemctl is-active systemd-oomd 2>&1
systemctl is-enabled systemd-oomd 2>&1
for f in /etc/systemd/oomd.conf /etc/systemd/oomd.conf.d/*.conf; do
  [ -r "$f" ] && { echo "   $f:"; grep -v '^\s*\(#\|$\)' "$f" | sed 's/^/     /'; }
done
systemctl show user.slice -p ManagedOOMMemoryPressure -p ManagedOOMSwap \
  -p ManagedOOMMemoryPressureLimit 2>&1 | sed 's/^/   user.slice /'
echo "-- oomd's two arms read these. Swap first (it kills over 90% used):"
swapon --show 2>&1
echo "-- then memory PSI (its limit is DefaultMemoryPressureLimit over DurationSec):"
cat /proc/pressure/memory 2>&1 | sed 's/^/   /'
echo "NOTE: oomd kills a whole cgroup (cgroup.kill). bwrap makes no cgroup of its"
echo "      own, so a seat shares its go test's cgroup — an oomd kill takes the"
echo "      REPORTER too, which is the shape ranger-base-evx07 did NOT see."
echo "NOTE: oomd's own kill log is in the system journal, which a seat cannot read"
echo "      unless its user is in adm/systemd-journal/wheel. Check, and say which:"
id -nG | tr ' ' '\n' | grep -x -e adm -e systemd-journal -e wheel || echo "   (in none of them: oomd is unnameable from here)"

echo
echo "=== C. pid reuse: can a stale pid in a kill census name a live stranger? ==="
echo "pid_max = $(cat /proc/sys/kernel/pid_max)"
echo "-- last_pid, sampled over 10s. Pids are handed out SEQUENTIALLY, so a"
echo "   FREED pid is only reissued once this counter wraps all the way past"
echo "   pid_max. The rate is therefore the whole of the price:"
p0=$(awk '{print $5}' /proc/loadavg)
for _ in 1 2 3 4 5; do
  sleep 2
  printf '     last_pid = %s\n' "$(awk '{print $5}' /proc/loadavg)"
done
p1=$(awk '{print $5}' /proc/loadavg)
d=$((p1 - p0))
if [ "$d" -gt 0 ]; then
  rate=$((d / 10))
  echo "     ~$rate pid/s over that window"
  [ "$rate" -gt 0 ] && echo "     => the counter wraps every ~$(( $(cat /proc/sys/kernel/pid_max) / rate / 60 )) min at this rate"
else
  echo "     (the counter moved $d: it wrapped during the window, or the box is idle)"
fi
echo "   A VENUE RUN IS THE CASE THAT MATTERS and it is bounded differently:"
echo "   ADR 0097 D1's venue is --unshare-pid, so its go test and every nested"
echo "   seat live in a FRESH pid namespace whose counter starts at 1. A ~120s"
echo "   run cannot consume pid_max, so inside one venue run a freed pid is"
echo "   never reissued. wallWatchKillWith's bwrapPid 0 arm is for the other"
echo "   case — a record outliving its watcher on the host, where this rate says"
echo "   reuse is a question of tens of minutes and not of days."

echo
echo "=== D. bwrap's signal propagation: what does a member actually READ? ==="
echo "bwrap propagates its SANDBOX CHILD's status and nothing deeper, so what"
echo "reaches the member depends on what that child did with the death below it."
k=$(mktemp); printf 'kill -9 $$\n' > "$k"
bwrap --ro-bind / / --dev-bind /dev /dev --unshare-user sh -c "exec sh $k" >/dev/null 2>&1
echo "   exec of a self-killing script         bwrap rc=$?  (signalled child: 128+9)"
bwrap --ro-bind / / --dev-bind /dev /dev --unshare-user sh -c "sh $k" >/dev/null 2>&1
echo "   no exec, killed child goes LAST       bwrap rc=$?  (the shell reports 128+9 itself)"
bwrap --ro-bind / / --dev-bind /dev /dev --unshare-user sh -c "sh $k; exit 7" >/dev/null 2>&1
echo "   no exec, the shell CARRIES ON         bwrap rc=$?  (the kill is MASKED)"
bwrap --ro-bind / / --dev-bind /dev /dev --unshare-user sh -c 'exit 128' >/dev/null 2>&1
echo "   git's own answer at 'cannot lock ref' bwrap rc=$?  (one below the range; never abstained)"
rm -f -- "${k:?}"
echo "None of the four printed anything. The first two are both 137 by different"
echo "routes — a signal death and a normal exit carrying the same number — which"
echo "is why bgSeatEndedFromOutside reads ExitCode and not Signaled alone. The"
echo "third is why packUnderALiveSeat's inner line execs."

if [ "$want_ns" = "nsino" ]; then
  echo
  echo "=== B. nsfs mount-namespace inode reuse ==="
  echo "wallWatchSeatPids(0, mntns) picks pids to SIGKILL by comparing"
  echo "/proc/<pid>/ns/mnt's INODE to a number read from a record. The code"
  echo "reasons about recycled PIDS and says nothing about recycled namespace"
  echo "INODES, so this is the reading that closes that gap."
  seen=$(mktemp)
  dupes=0
  for _ in $(seq 1 200); do
    ino=$(bwrap --ro-bind / / --unshare-all stat -c %i /proc/self/ns/mnt 2>/dev/null)
    [ -z "$ino" ] && continue
    grep -qx "$ino" "$seen" && { dupes=$((dupes + 1)); echo "   ino $ino REUSED"; }
    echo "$ino" >> "$seen"
  done
  echo "   namespaces: $(wc -l < "$seen")   distinct inodes: $(sort -u "$seen" | wc -l)   reuses: $dupes"
  echo "   range: $(sort -n "$seen" | head -1) .. $(sort -n "$seen" | tail -1)"
  rm -f -- "${seen:?}"
fi

if [ "$want_ns" = "pdeathsig" ]; then
  echo
  echo "=== E. --die-with-parent + per-task PDEATHSIG + Go thread churn ==="
  echo "PR_SET_PDEATHSIG is per-TASK — this repo says so in terms (wallunix.go:"
  echo "\"PR_SET_NO_NEW_PRIVS, landlock_restrict_self and bwrap's"
  echo "--die-with-parent pdeathsig are all per-task\"). It fires when the parent"
  echo "THREAD exits, not when the parent process does. Go forks exec.Command"
  echo "children from whatever OS thread the goroutine is on, and a goroutine"
  echo "holding runtime.LockOSThread that RETURNS has its thread terminated"
  echo "(documented). Every seat in the ADR 0097 member set is a"
  echo "--die-with-parent bwrap forked by a multi-threaded, heavily parallel Go"
  echo "test binary — so a healthy test binary killing its OWN seats, with no"
  echo "external killer and no output, was a complete candidate for 137. It also"
  echo "predicted the one thing the bead found strange: that the victim was an"
  echo "IDLE seat parked at a ready file, which is what a thread-exit race"
  echo "selects and what no CPU-burning-leak heuristic would."
  echo
  echo "It is NOT what happens. MEASURED 2026-11-21: all three arms ALIVE."
  echo "bwrap sets its pdeathsig on the SANDBOX CHILD relative to bwrap, not on"
  echo "itself relative to the forking thread. Kept because the premise is sound"
  echo "and the next reader of a 137 will reach for it again."
  echo
  # The source lives here rather than in a tracked .go file on purpose: a .go
  # outside $(FMT_ROOTS) is reported clean by `make fmt-check` and not fixed by
  # `make fmt` (ranger-base-rulbl one directory over), and it also trips the
  # protocolCmd door. A heredoc into a temp dir is neither.
  d=$(mktemp -d)
  cat > "$d/main.go" <<'GOEOF'
package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// Arms differ by ONE variable each, so a future kernel or bwrap that does
// propagate says which half moved:
//
//	locked   — forked from a LockOSThread goroutine that then returns
//	unlocked — forked from an ordinary goroutine that then returns (control)
//	nodwp    — locked, with --die-with-parent taken OFF (control)
func arm(name string, lock, dieWithParent bool) {
	argv := []string{}
	if dieWithParent {
		argv = append(argv, "--die-with-parent")
	}
	argv = append(argv, "--ro-bind", "/", "/", "--dev-bind", "/dev", "/dev",
		"--unshare-user", "sh", "-c", "sleep 60")

	var cmd *exec.Cmd
	var wg sync.WaitGroup
	started := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if lock {
			runtime.LockOSThread() // no Unlock: the thread dies with this goroutine
		}
		cmd = exec.Command("bwrap", argv...)
		started <- cmd.Start()
	}()
	if err := <-started; err != nil {
		fmt.Printf("   %-9s start failed: %v\n", name, err)
		return
	}
	wg.Wait() // the forking goroutine has returned; its thread is gone if locked

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t0 := time.Now()
	select {
	case err := <-done:
		fmt.Printf("   %-9s seat DIED after %dms: err=%v (ExitCode=%d)\n",
			name, time.Since(t0).Milliseconds(), err, cmd.ProcessState.ExitCode())
	case <-time.After(3 * time.Second):
		fmt.Printf("   %-9s seat ALIVE after 3s (the thread exit did not reach it)\n", name)
		_ = cmd.Process.Kill()
		<-done
	}
}

func main() {
	fmt.Printf("   GOMAXPROCS=%d go=%s\n", runtime.GOMAXPROCS(0), runtime.Version())
	arm("locked", true, true)
	arm("unlocked", false, true)
	arm("nodwp", true, false)
}
GOEOF
  (cd "$d" && go mod init pdeathsig >/dev/null 2>&1 && go run .)
  rm -rf -- "${d:?}"
fi
