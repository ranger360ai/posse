#!/bin/sh
# Linux cage tier (ADR 0080, amendment "Landlock as the Linux cage",
# ranger-base-20r29). Written 2026-11-06 on a darwin laptop with no Linux
# kernel to hand, so NOTHING below has run yet: every expectation is from the
# kernel's Landlock doc (7.4.0-rc1) and landlock(7) (man-pages 6.19). The
# first run on the D1 host is the measurement; paste its output on the bead
# that runs it.
#
# FIRST RUN 2026-11-08, on the D1 host as the service account
# (ranger-base-hft2q): Linux 7.2, bubblewrap 0.13.1, Landlock ABI 10, LSMs
# lockdown,capability,landlock,yama,bpf,ima,evm. Every L- and B-row read
# GOT == EXPECT; the one finding was B5's cell, which this file's B5 comment
# now explains. So the expectations below are no longer all unread — what each
# row is worth after that run is ADR 0080's "Measured versus assumed" block,
# which reads that run row by row and is the current record; the "ASSUMED"
# notes inside D14 and D15 are the pre-run state it supersedes
# (ranger-base-vdmm9).
#
#   sh docs/adr/0080-linux-cage.probe.sh
#
# Needs: Linux, python3 (Landlock via ctypes, no compiler), bwrap, unshare.
# Run as the `posse` user. Everything it makes is under one mktemp dir and is
# removed on exit; nothing here writes to the repo, a home, or a persona.
#
# Each line prints  PROBE  EXPECT  GOT. A row whose GOT differs from EXPECT
# is the finding, not a failed run.
set -u
W=$(mktemp -d)
trap 'chmod -R u+w "$W" 2>/dev/null; rm -rf "$W"' EXIT
mkdir -p "$W/tree/deny" "$W/tree/ok" "$W/home"
echo secret > "$W/home/cred"
echo '{}' > "$W/home/cfg.json"

row() { printf '%-44s %-10s %s\n' "$1" "$2" "$3"; }
try() { if "$@" >/dev/null 2>&1; then echo allowed; else echo refused; fi; }

echo "== host =="
uname -sr
command -v bwrap >/dev/null && bwrap --version || echo "bwrap: absent"
cat /sys/kernel/security/lsm 2>/dev/null; echo
echo "kernel.unprivileged_userns_clone=$(sysctl -n kernel.unprivileged_userns_clone 2>/dev/null || echo n/a)"
echo "user.max_user_namespaces=$(sysctl -n user.max_user_namespaces 2>/dev/null || echo n/a)"
for b in claude codex grok git bd; do
  printf '%s: ' "$b"; command -v "$b" >/dev/null && "$b" --version 2>&1 | head -1 || echo absent
done

# ---------------------------------------------------------------- Landlock
# ll.py <mode> <tree> <home> -- cmd...   applies one ruleset, then execs cmd.
# Syscall numbers 444-446 are the generic table (x86_64 and arm64 alike).
cat > "$W/ll.py" <<'PY'
import ctypes, os, sys
libc = ctypes.CDLL(None, use_errno=True)
CREATE, ADD, RESTRICT = 444, 445, 446
abi = libc.syscall(CREATE, None, 0, 1)  # LANDLOCK_CREATE_RULESET_VERSION
if sys.argv[1] == "abi":
    print(abi if abi > 0 else "unavailable errno=%d" % ctypes.get_errno()); sys.exit(0)
EXEC, WF, RF, RD, RMD, RMF = 1, 2, 4, 8, 16, 32
MK = 64 | 128 | 256 | 512 | 1024 | 2048 | 4096   # MAKE_CHAR..MAKE_SYM
REFER, TRUNC = 8192, 16384
ALL = EXEC | WF | RF | RD | RMD | RMF | MK | REFER | TRUNC
READ = EXEC | RF | RD
FILE_ONLY = EXEC | WF | RF | TRUNC
class Attr(ctypes.Structure):
    _fields_ = [("fs", ctypes.c_uint64), ("net", ctypes.c_uint64), ("scoped", ctypes.c_uint64)]
class Beneath(ctypes.Structure):
    _pack_ = 1
    _fields_ = [("allowed", ctypes.c_uint64), ("fd", ctypes.c_int32)]
mode, tree, home = sys.argv[1], sys.argv[2], sys.argv[3]
cmd = sys.argv[sys.argv.index("--") + 1:]
fd = libc.syscall(CREATE, ctypes.byref(Attr(ALL, 0, 0)), ctypes.sizeof(Attr), 0)
if fd < 0: sys.exit("create_ruleset errno=%d" % ctypes.get_errno())
def allow(path, rights):
    p = os.open(path, os.O_PATH)
    r = libc.syscall(ADD, fd, 1, ctypes.byref(Beneath(rights, p)), 0)
    if r != 0: sys.exit("add_rule %s errno=%d" % (path, ctypes.get_errno()))
allow("/", READ)
allow("/dev", ALL)
allow("/tmp", ALL) if not tree.startswith("/tmp") else None
if mode == "union":        # tree writable, tree/deny granted READ only
    allow(tree, ALL); allow(tree + "/deny", READ)
elif mode == "fileonly":   # cfg.json writable as a FILE rule, home read-only
    allow(home + "/cfg.json", FILE_ONLY & ~EXEC)
elif mode == "plain":
    allow(tree, ALL)
libc.prctl(38, 1, 0, 0, 0)                          # PR_SET_NO_NEW_PRIVS
if libc.syscall(RESTRICT, fd, 0) != 0: sys.exit("restrict errno=%d" % ctypes.get_errno())
os.execvp(cmd[0], cmd)
PY
LL="python3 $W/ll.py"
echo "== Landlock =="
row "L0 ABI version" ">=3" "$($LL abi x x)"
# L1: the seatbelt trailing deny. Landlock rules are a UNION within a layer,
# so the READ-only rule on tree/deny cannot take back tree's write.
row "L1 write under a 'denied' subtree" allowed \
  "$(try $LL union "$W/tree" "$W/home" -- sh -c "echo x > $W/tree/deny/x")"
# L2: claude's atomic config write: temp beside the file, then rename.
# MAKE_REG/REMOVE_FILE live on the DIRECTORY, so a file rule cannot allow it.
row "L2 atomic replace of a file-ruled file" refused \
  "$($LL fileonly "$W/tree" "$W/home" -- sh -c "echo n > $W/home/cfg.json.tmp && mv $W/home/cfg.json.tmp $W/home/cfg.json" >/dev/null 2>&1 && echo allowed || echo refused)"
row "L2b in-place write of the same file" allowed \
  "$(try $LL fileonly "$W/tree" "$W/home" -- sh -c "echo n > $W/home/cfg.json")"
# L3: topology. A landlocked process may not mount, so a runtime's own
# bwrap sandbox (claude's) cannot start inside a Landlock cage.
row "L3 unshare+mount inside Landlock" refused \
  "$(try $LL plain "$W/tree" "$W/home" -- unshare -rm mount -t tmpfs none "$W/tree/ok")"
row "L3b bwrap inside Landlock" refused \
  "$(try $LL plain "$W/tree" "$W/home" -- bwrap --ro-bind / / true)"

# --------------------------------------------------------- mount namespace
# The trailing deny, expressed as a read-only bind on top of a writable one.
BW="bwrap --die-with-parent --ro-bind / / --dev /dev --proc /proc --tmpfs /tmp
  --bind $W/tree $W/tree --ro-bind $W/tree/deny $W/tree/deny
  --ro-bind /dev/null $W/home/cred"
echo "== bubblewrap =="
row "B0 bwrap starts at all" allowed "$(try $BW true)"
row "B1 write in the granted tree" allowed "$(try $BW sh -c "echo x > $W/tree/ok/x")"
row "B2 write under the denied subtree" refused "$(try $BW sh -c "echo x > $W/tree/deny/x")"
row "B3 rename the denied subtree away" refused "$(try $BW mv "$W/tree/deny" "$W/tree/d2")"
# B4: the seal question. Renaming an ANCESTOR of the mountpoint is allowed;
# the mount should travel with the dentry, so the moved tree stays read-only.
mkdir -p "$W/tree/a/deep"
BW4="bwrap --die-with-parent --ro-bind / / --dev /dev --proc /proc --tmpfs /tmp
  --bind $W/tree $W/tree --ro-bind $W/tree/a/deep $W/tree/a/deep"
row "B4 rename ancestor, then write moved deny" refused \
  "$(try $BW4 sh -c "mv $W/tree/a $W/tree/b && echo x > $W/tree/b/deep/x")"
# B5: shape 5, the mask. The GOT cell is the BYTES the read returned, and
# nothing else: "empty" when it returned none, the first 20 of the secret if
# the mask leaked. cat's own stderr used to be concatenated in FRONT of that
# word, so a read that failed printed "cat: <path>: ...empty" and the row read
# as a mismatch when nothing had leaked (ranger-base-yca4g). It is not thrown
# away either. No bytes is no leak whatever stopped them, so the cell matching
# is right — but shape 5 is a claim about WHY they were absent, and its
# documented reading is EOF at exit 0, not an error. So the reason prints UNDER
# the row as prose, and D15 says what it is worth: not a D14 reopen, and not a
# pass of shape 5 either.
b5=$($BW cat "$W/home/cred" 2>"$W/b5.err"); b5rc=$?
row "B5 read the masked credential" empty \
  "$(if [ -n "$b5" ]; then printf '%s' "$b5" | head -c 20; else echo empty; fi)"
if [ "$b5rc" -ne 0 ] || [ -s "$W/b5.err" ]; then
  echo "   B5 detail (not a row): exit=$b5rc stderr=$(head -c 200 "$W/b5.err" | tr '\n' ' ')"
fi
# B6: can the seat undo its own wall? A new userns gets caps, but mounts it
# inherits are MNT_LOCKED: no umount, no clearing the read-only flag.
row "B6 umount the deny from a nested userns" refused \
  "$(try $BW unshare -rm umount "$W/tree/deny")"
row "B6b remount the deny rw from a nested userns" refused \
  "$(try $BW unshare -rm mount -o remount,bind,rw "$W/tree/deny")"
# B7: a runtime's OWN sandbox nested inside ours (claude uses bwrap).
#
# THIS $BW IS NOT THE SHIPPED WALL, and that is what B7 is a row about (ADR
# 0096). $BW carries `--proc /proc`; D17's render deliberately does not, so
# inside a shipped seat /proc is the host's procfs under the read-only bind, a
# nested user namespace cannot write its own uid_map, and nothing nests. B7
# stays a true row about this $BW and about bwrap as a MECHANISM, and it was
# never a property of the tier — reading it as one is what ADR 0096 D1
# corrected, after the wall's own live pins were found nesting a bwrap and
# then failing, or abstaining for the wrong reason, inside a caged seat
# (eleven test files; docs/notes.d/ranger-base-pvkhn.md has the census). The
# row for the shipped wall is `T-nest` in 0080-bwrap-tier.probe.sh, whose
# $BASE is D17's, and it EXPECTS refused.
row "B7 bwrap inside bwrap" allowed "$(try $BW bwrap --ro-bind / / --dev /dev true)"
# B8: the atomic config write against a bind-mounted FILE: rename onto a
# mountpoint is EBUSY. This is why the seat's config must live in a DIR grant.
BW8="bwrap --die-with-parent --ro-bind / / --dev /dev --proc /proc --tmpfs /tmp
  --bind $W/home/cfg.json $W/home/cfg.json"
row "B8 atomic replace of a bound file" refused \
  "$(try $BW8 sh -c "echo n > $W/home/cfg.json.tmp && mv $W/home/cfg.json.tmp $W/home/cfg.json")"
# B9: the runtimes start under a wall (version only: no model is called).
for b in claude codex grok; do
  command -v "$b" >/dev/null || { row "B9 $b --version under bwrap" allowed absent; continue; }
  row "B9 $b --version under bwrap" allowed "$(try $BW "$b" --version)"
done
echo "== done; GOT != EXPECT rows are the findings =="
