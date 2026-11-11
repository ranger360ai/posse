## The bwrap wall's hole was connect(2), not a write (ranger-base-ludpp)

2026-11-11. A read-only bind of `/` lets a seat connect to every unix socket
it can see, and lets it write-open every FIFO it can see. The kernel's EROFS
check covers neither. So the bwrap tier was walled against writes while the
user runtime dir's control endpoints stayed open. Those include the session
bus, the user service manager, the multiplexer's server, the agents and the
secret store, and several of them act outside the cage.

The fix is in ADR 0080's amendment of this date. It mounts an empty tmpfs
over the runtime dir (both `$XDG_RUNTIME_DIR` and `/run/user/<uid>`, resolved)
right after the base, so a later write grant is the only way back in. It
also unsets the bus and agent addresses.

What to remember when you port a wall to another mechanism: **list the
syscalls that bypass the check, not only the paths it covers.** A path rule
answers "may this file be written". A socket and a FIFO are reached by
connect and open, which a mount flag never sees. A mount over the directory
holding them, or a network namespace, does reach them.

MEASURED, D1 host, bubblewrap 0.13.1: under the old base the real
`/run/user/1001` had 21 entries and five sockets connected. Under the new
base it had 0 entries and the bus variable was unset. The pins are in
`internal/posse/bwraphide_qa_test.go`. The live pin uses a fixture runtime
dir and a control arm, so it does not depend on this box's desktop session.
Each of the three mutations (no tmpfs, no unset, a deny re-bound into the
hidden dir) reds its own pin.

ASSUMED, not measured: that a claude seat runs normally with a private,
empty runtime dir. Claude could not create its messaging socket under the
old read-only base either, so nothing it could do before is lost. The
re-verify from a live bwrap seat on ranger-base-x9f3s is where this
gets measured.
