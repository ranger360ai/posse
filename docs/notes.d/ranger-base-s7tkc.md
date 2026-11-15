## The bwrap wall hides the host's crash stores (ranger-base-s7tkc)

2026-11-14. A bwrap seat runs at the same uid as every host process. So it
could read what systemd-coredump keeps about a same-uid crash: the core
files under `/var/lib/systemd/coredump` (each carries an ACL that gives the
crashed uid read access), and the crash's `COREDUMP_ENVIRON` in the journal.
`coredumpctl` and `journalctl` read the journal FILES under
`/var/log/journal` and `/run/log/journal` directly. Cores already stored on a
host could be read, so nobody had to provoke a crash first.

The fix puts the three directories on the carve-out's Hide list
(`bwrapCrashStoreDirs`, appended in `liveBwrapRuntimeDirs`). The 1b loop
mounts an empty tmpfs over each one, the same way it hides the user runtime
dir. The landed judge's cage gets them too, because `landedJudgeHides` uses
the same producer. A directory a host does not have is skipped, not refused,
so a host without systemd-coredump or with a volatile-only journal renders
cleanly. Logging from a seat still works, because journald's input socket is
`/run/systemd/journal/socket`, which is not under any of the three.

MEASURED, D1 host, bubblewrap 0.13.1, base argv plus the three tmpfs:
- Inside the wall: 0 entries in each directory. `coredumpctl list` printed
  "No coredumps found". `journalctl -o export` showed 0 `COREDUMP_` fields.
  `logger` from inside still reached the host journal.
- Control, without the wall: 2 cores, and 1 `COREDUMP_ENVIRON` that could be
  read.

The pins are in `internal/posse/bwrapcrashstore_qa_test.go`. They spell out
the names rather than reading them off the list. As a negative control I
reverted the append in `liveBwrapRuntimeDirs`, and the render pin went red
once for each store.

Not closed here:
- ranger-base-al0d0
