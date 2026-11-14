#!/bin/sh
# LANDLOCK_ACCESS_FS_RESOLVE_UNIX as a layer INSIDE the bwrap cage
# (ADR 0080, amendment "RESOLVE_UNIX closes the connect hole and takes the
# nested mount with it", ranger-base-6rndn).
#
# 0080-linux-cage.probe.sh's L-rows measured Landlock as a REPLACEMENT for
# the bwrap tier and rejected it (D13). This measures the one shape D13's
# "Alternatives rejected" left open — "bwrap plus a Landlock layer for the
# scopes bwrap lacks ... A scope-only ruleset may not block [the nested
# bwrap], but that is ASSUMED and unread" — because ABI 9 added a right that
# refuses connect(2) to a host pathname socket, which is the one thing no
# launch-time bwrap mount can do (ranger-base-l2gk9).
#
# FIRST RUN 2026-11-13 on the D1 host (ranger-base-6rndn): Linux
# 7.2.15-1-omarchy, bubblewrap 0.13.1, Landlock ABI 10, linux-api-headers
# 7.3-1. Every U-row read GOT == EXPECT. The two rows that decide the design
# are U9 and U10, and their EXPECT below is what the amendment MEASURED, not
# what the design wanted: a domain handling RESOLVE_UNIX **and nothing else**
# is still a filesystem domain for the topology hooks, so mount(2) is refused
# and a nested bwrap cannot start. That is ADR 0080 D14's probe B7, the one
# property D13 says Landlock fails, and it fails here too.
#
#   sh docs/adr/0080-resolve-unix.probe.sh
#
# Needs: Linux, python3 (Landlock via ctypes, no compiler), bwrap, unshare.
# Run as the posse user. Everything it makes is under one mktemp dir and is
# removed on exit; it binds scratch sockets only, writes nothing to the repo,
# a home or a persona, and waits on every child it forks.
#
# Each line prints  PROBE  EXPECT  GOT. A row whose GOT differs from EXPECT
# is the finding, not a failed run. Do not rerun to make it match.
set -u
W=$(mktemp -d "${TMPDIR:-/tmp}/posse-ru.XXXXXX")
trap 'chmod -R u+w "$W" 2>/dev/null; rm -rf -- "${W:?}"' EXIT

echo "== host =="
uname -sr
command -v bwrap >/dev/null && bwrap --version || echo "bwrap: absent"
cat /sys/kernel/security/lsm 2>/dev/null; echo
printf 'RESOLVE_UNIX in the headers: '
grep -c LANDLOCK_ACCESS_FS_RESOLVE_UNIX /usr/include/linux/landlock.h 2>/dev/null \
  || echo "linux/landlock.h: absent"

# ru.py is the whole measurement. One file, because every row needs the same
# three things — an unrestricted parent that binds the sockets, a forked child
# that applies the domain, and a wait on that child — and a shell row per case
# would fork a new unrestricted parent each time and lose the ordering that
# U2, U12 and U13 are entirely about: who bound the socket, and whether it was
# before or after restrict_self.
#
# RESOLVE_UNIX is (1ULL << 16), ABI 9+. Syscall numbers 444-446 are the
# generic table (x86_64 and arm64 alike), as 0080-linux-cage.probe.sh's ll.py
# already uses them.
cat > "$W/ru.py" <<'PY'
import ctypes, os, socket, subprocess, sys, time

libc = ctypes.CDLL(None, use_errno=True)
CREATE, ADD, RESTRICT = 444, 445, 446
RESOLVE_UNIX = 1 << 16
ERRNAME = {1: "EPERM", 2: "ENOENT", 13: "EACCES", 111: "ECONNREFUSED"}
W = sys.argv[1]


class Attr(ctypes.Structure):
    _fields_ = [("fs", ctypes.c_uint64), ("net", ctypes.c_uint64),
                ("scoped", ctypes.c_uint64)]


class Beneath(ctypes.Structure):
    _pack_ = 1
    _fields_ = [("allowed", ctypes.c_uint64), ("fd", ctypes.c_int32)]


def abi():
    v = libc.syscall(CREATE, None, 0, 1)
    return v if v > 0 else "unavailable errno=%d" % ctypes.get_errno()


def restrict(allows=()):
    """One layer handling RESOLVE_UNIX only; allows = [(path, rights)]."""
    fd = libc.syscall(CREATE, ctypes.byref(Attr(RESOLVE_UNIX, 0, 0)),
                      ctypes.sizeof(Attr), 0)
    if fd < 0:
        raise OSError(ctypes.get_errno(), "create_ruleset")
    for path, rights in allows:
        p = os.open(path, os.O_PATH)
        if libc.syscall(ADD, fd, 1, ctypes.byref(Beneath(rights, p)), 0) != 0:
            raise OSError(ctypes.get_errno(), "add_rule " + path)
        os.close(p)
    libc.prctl(38, 1, 0, 0, 0)                       # PR_SET_NO_NEW_PRIVS
    if libc.syscall(RESTRICT, fd, 0) != 0:
        raise OSError(ctypes.get_errno(), "restrict_self")
    os.close(fd)


def serve(path):
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    s.bind(path)
    s.listen(8)
    return s


def connect(path):
    c = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    try:
        c.connect(path)
        return "ok"
    except OSError as e:
        return ERRNAME.get(e.errno, "errno=%d" % e.errno)
    finally:
        c.close()


def sendto(path):
    c = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
    try:
        c.sendto(b"x", path)
        return "ok"
    except OSError as e:
        return ERRNAME.get(e.errno, "errno=%d" % e.errno)
    finally:
        c.close()


def run(argv):
    p = subprocess.run(argv, capture_output=True, text=True)
    if p.returncode == 0:
        return "allowed"
    # The stderr line, not just "refused": a cell that throws the stream away
    # cannot tell a sandbox denying a thing from a tool that never started,
    # which is the whole lesson of ADR 0080 D21's T6 (ranger-base-bjk4k).
    return "refused rc=%d %s" % (p.returncode,
                                 (p.stderr.strip().splitlines() or [""])[0][:70])


def wait_for(path, secs=5.0):
    end = time.time() + secs
    while time.time() < end:
        if os.path.exists(path):
            return True
        time.sleep(0.02)
    return False


def child(fn):
    """fn() in a fork, so the parent stays unrestricted. -> fn's string."""
    r, w = os.pipe()
    pid = os.fork()
    if pid == 0:
        os.close(r)
        try:
            out = fn()
        except BaseException as e:                       # noqa: BLE001
            out = "ERR " + repr(e)[:140]
        try:
            os.write(w, str(out).encode()[:600])
        finally:
            os._exit(0)
    os.close(w)
    buf = b""
    while True:
        chunk = os.read(r, 4096)
        if not chunk:
            break
        buf += chunk
    os.close(r)
    os.waitpid(pid, 0)
    return buf.decode() or "(no output)"


def handshake(restricted_child, then):
    """Run restricted_child() in a fork; when it says it is restricted, run
    then() HERE (unrestricted), and let the child proceed. -> child's string.

    The ordering is the measurement: `then` is a bind, an unlink or a rebind
    performed by a process the domain does not cover, after the domain is up.
    """
    flag, go = os.path.join(W, "restricted"), os.path.join(W, "go")
    for p in (flag, go):
        if os.path.exists(p):
            os.unlink(p)
    r, w = os.pipe()
    pid = os.fork()
    if pid == 0:
        os.close(r)
        try:
            out = restricted_child(flag, go)
        except BaseException as e:                       # noqa: BLE001
            out = "ERR " + repr(e)[:140]
        try:
            os.write(w, str(out).encode()[:600])
        finally:
            os._exit(0)
    os.close(w)
    wait_for(flag)
    keep = then()
    open(go, "w").close()
    got = os.read(r, 4096).decode()
    os.close(r)
    os.waitpid(pid, 0)
    if hasattr(keep, "close"):
        keep.close()
    return got or "(no output)"


# ── the nested entry point: run under a bwrap wall, by the rows at the end ──
if len(sys.argv) > 2 and sys.argv[2] == "--inner":
    target = sys.argv[3]
    print("  N1 inner, no domain, connect host sock      ", connect(target))
    print("  N2 inner, domain, connect host sock         ",
          child(lambda: (restrict(), connect(target))[1]))
    print("  N3 inner, domain, nested bwrap              ",
          child(lambda: (restrict(),
                         run(["bwrap", "--ro-bind", "/", "/", "true"]))[1]))
    sys.exit(0)

if len(sys.argv) > 2 and sys.argv[2] == "--inner-wait":
    # Under a --tmpfs wall: wait until the host has bound a NEW socket at this
    # path in ITS view, then try it. No domain at all — the mount is the wall.
    ready, target = sys.argv[3], sys.argv[4]
    wait_for(ready)
    print("  N4 inner, tmpfs wall, socket bound after    ", connect(target))
    sys.exit(0)


ROWS = []


def row(name, expect, got):
    ROWS.append((name, expect, got))
    print("%-52s %-12s %s" % (name, expect, got), flush=True)


print("landlock abi: %s   (RESOLVE_UNIX needs >= 9)" % abi())
print()

pre = serve(os.path.join(W, "pre.sock"))          # bound unrestricted, BEFORE
other = serve(os.path.join(W, "other.sock"))
dg = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
dg.bind(os.path.join(W, "dgram.sock"))

print("== RESOLVE_UNIX handled, no allow rule ==")
def restricted_connect(target, allows=()):
    """The ordinary shape of a row: domain first, then one connect."""
    def fn():
        restrict(allows)
        return connect(target)
    return child(fn)


def waiting_connect(target, allows=()):
    """The restricted half of a handshake: domain, say so, wait, connect."""
    def fn(flag, go):
        restrict(allows)
        open(flag, "w").close()
        wait_for(go)
        return connect(target)
    return fn


row("U0 control: no domain, connect pre.sock", "ok",
    child(lambda: connect(pre.getsockname())))
row("U1 domain, socket bound BEFORE restrict_self", "EACCES",
    restricted_connect(pre.getsockname()))
# U2 is the row no bwrap mount can answer (ranger-base-l2gk9): an UNRESTRICTED
# process binds the socket after the seat's wall is already up.
post = os.path.join(W, "post.sock")
row("U2 domain, socket bound AFTER restrict_self", "EACCES",
    handshake(waiting_connect(post), lambda: serve(post)))


def u3():
    """The domain first, and only then the socket — the order IS the row."""
    restrict()
    s = serve(os.path.join(W, "inside.sock"))
    out = connect(s.getsockname())
    s.close()
    return out


row("U3 domain, socket made INSIDE the domain", "ok", child(u3))


def u4():
    restrict()
    return sendto(dg.getsockname())


row("U4 domain, sendmsg(DGRAM) to an explicit address", "EACCES", child(u4))


def u5():
    restrict()
    p = subprocess.run([sys.executable, "-I", __file__, W,
                        "--inner", pre.getsockname()],
                       capture_output=True, text=True)
    return (p.stdout.splitlines() or ["(no output)"])[0].split()[-1]


row("U5 the domain survives execve", "EACCES", child(u5))

print()
print("== allow back by path ==")
row("U6 allow rule on the socket FILE", "ok",
    restricted_connect(pre.getsockname(),
                       [(pre.getsockname(), RESOLVE_UNIX)]))
row("U7 allow rule on its DIRECTORY", "ok",
    restricted_connect(pre.getsockname(), [(W, RESOLVE_UNIX)]))
row("U8 a file allow does not reach a SIBLING socket", "EACCES",
    restricted_connect(other.getsockname(),
                       [(pre.getsockname(), RESOLVE_UNIX)]))

print()
print("== does a RESOLVE_UNIX-only domain restrict anything ELSE? ==")
# The two rows that decide the design. ADR 0080 D14's B7 — a runtime's own
# bwrap nests inside ours — is the property D13 says Landlock fails, and
# "Alternatives rejected" left open whether a SCOPE-only layer fails it too.
# RESOLVE_UNIX is not a scope: it is handled_access_fs, so the question is
# whether the topology hooks read the right or merely the presence of a
# filesystem domain. They read the presence.
os.makedirs(os.path.join(W, "mnt"), exist_ok=True)


def restricted_run(argv):
    def fn():
        restrict()
        return run(argv)
    return child(fn)


nest = restricted_run(["bwrap", "--ro-bind", "/", "/", "true"])
mnt = restricted_run(["unshare", "-rm", "mount", "-t", "tmpfs", "none",
                      os.path.join(W, "mnt")])
row("U9 bwrap inside the domain (ADR 0080 B7)", "refused", nest.split(" ")[0])
row("U10 unshare -rm mount tmpfs inside the domain", "refused",
    mnt.split(" ")[0])
# The detail lines B5 taught this file to print (ranger-base-yca4g): the cell
# above is one word, and one word cannot say whether mount was refused or the
# tool was never there.
print("   U9/U10 detail (not a row), the stderr those two cells drop:")
print("     bwrap:   ", nest)
print("     unshare: ", mnt)


def u11():
    restrict()
    p = os.path.join(W, "wrote")
    with open(p, "w") as f:
        f.write("x")
    with open(p) as f:
        f.read()
    os.unlink(p)
    d = os.path.join(W, "d")
    os.mkdir(d)
    os.rename(d, d + "2")
    os.rmdir(d + "2")
    os.stat("/etc/hostname")
    return "allowed"


row("U11 ordinary file read/write/mkdir/rename in it", "allowed", child(u11))

print()
print("== what an allow rule is worth after the host re-binds ==")
# ranger-base-l2gk9's defeat of the bwrap hide was an unlink plus a re-bind by
# the host: the seat's mount detaches and the NEW socket connects. The same
# move against an allow RULE is the mirror question, and it decides whether an
# allow list can name a socket a daemon restarts.
reb = os.path.join(W, "reb.sock")
rs = serve(reb)


def rebind():
    rs.close()
    os.unlink(reb)
    return serve(reb)


row("U12 allow on the FILE, then host unlink + re-bind", "EACCES",
    handshake(waiting_connect(reb, [(reb, RESOLVE_UNIX)]), rebind))
# U13 is U12's answer: a DIRECTORY rule is a statement about the subtree, so a
# socket that does not exist yet is already covered — which is the only way an
# allow list can survive a restart, and the only way to allow a socket that is
# absent at launch (an O_PATH open of it would be ENOENT).
late = os.path.join(W, "late.sock")
row("U13 allow on the DIR, socket bound in it after", "ok",
    handshake(waiting_connect(late, [(W, RESOLVE_UNIX)]),
              lambda: serve(late)))

print()
print("== the mount answer, for comparison (no domain at all) ==")
# The control for the whole design: ludpp's existing `--tmpfs` over a host
# socket DIRECTORY, measured against the same unlink-and-re-bind that beat the
# per-socket hide. It holds — and it costs no topology — so the mount answer
# is intact for every directory a seat can afford to have emptied, and the
# residual RESOLVE_UNIX exists for is a socket inside a WRITE GRANT.
tm = os.path.join(W, "tmpfsdir")
os.makedirs(tm, exist_ok=True)
hidden_sock = os.path.join(tm, "appears-later.sock")
ready = os.path.join(W, "tmpfs-ready")
if os.path.exists(ready):
    os.unlink(ready)
p = subprocess.Popen(
    ["bwrap", "--die-with-parent", "--ro-bind", "/", "/", "--dev", "/dev",
     "--bind", W, W, "--tmpfs", tm,
     sys.executable, "-I", __file__, W, "--inner-wait", ready, hidden_sock],
    stdout=subprocess.PIPE, text=True)
time.sleep(0.4)
hs = serve(hidden_sock)                  # the host binds it AFTER the wall
open(ready, "w").close()
out, _ = p.communicate(timeout=30)
hs.close()
print(out.rstrip() or "(no stdout)")
print("     EXPECT N4 ENOENT — the seat's tmpfs has no such name, and the")
print("     host's unlink/re-bind happens in a directory it cannot see.")

print()
print("== the real shape: the domain applied INSIDE a bwrap wall ==")
p = subprocess.run(
    ["bwrap", "--die-with-parent", "--ro-bind", "/", "/", "--dev", "/dev",
     "--bind", W, W,
     sys.executable, "-I", __file__, W, "--inner", pre.getsockname()],
    capture_output=True, text=True)
print(p.stdout.rstrip() or "(no stdout)")
if p.returncode != 0:
    print("  bwrap rc=%d stderr=%s" % (p.returncode, p.stderr.strip()[:300]))
print("     EXPECT N1 ok, N2 EACCES, N3 refused — the hole closes inside the")
print("     cage, and the nested bwrap goes with it.")

print()
bad = [t for t in ROWS if t[1] != t[2]]
print("rows: %d, mismatched: %d" % (len(ROWS), len(bad)))
for n, e, g in bad:
    print("  MISMATCH %-48s expect=%-10s got=%s" % (n, e, g))
PY

echo
python3 -I "$W/ru.py" "$W"
