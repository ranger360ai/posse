# A seat's `git config --global` cannot reach the operator's global file (ranger-base-161t2)

2026-11-15. OBSERVED that morning: `~/.config/git/config`, the operator's
global git config, was rewritten to `user.name=probe`,
`user.email=probe@invalid` in the same second a probe finished on a
shims-tier seat, and an earlier event did the same with `p`/`p@x`. Commits
on `main` carried both identities.

## How `--global` escaped the HOME override

It never went through HOME. Since ranger-base-obdnk every seat carries
`GIT_CONFIG_GLOBAL=<the operator's resolved global file>` in its settings
pin, and that variable replaces git's whole global-file lookup — which is
why it closes the HOME and XDG_CONFIG_HOME inlets. The probe ran
`export HOME=<mktemp>/home; git config --global user.name probe`, expecting
a scratch HOME to divert the write, and git wrote the file the pin named.
Not XDG resolution, not a wrapper, not a hook: the probe called the real git
by absolute path, so no L1 shim saw it either.

## The fix

The row now names a read-only SNAPSHOT of that file
(`gitglobalscope.go`, `gitGlobalSnapshot`/`gitGlobalPinValue`): a byte copy,
content-addressed under `~/.cache/posse-wall/git-global/<hash>/`, with a
directory standing at `<snapshot>.lock`. git takes a config write's lock with
`O_CREAT|O_EXCL` on that exact name, so every write — `--global`,
`--file "$GIT_CONFIG_GLOBAL"`, `--unset` — refuses with
`could not lock config file …: File exists`, rc 255. Reads, `git var
GIT_AUTHOR_IDENT` and the identity wall see the same bytes. MEASURED
2026-11-15, git 2.56.0, linux. The settings pin and the git shim's hook-skip
reference (ADR 0079 D1) both take the value from `gitGlobalPinValue`, so they
still agree.

Two choices in it worth knowing:

- **A lock directory, not a 0555 parent.** Both refuse the write; a 0555
  directory is one `os.RemoveAll` cannot empty, and every test HOME in this
  package is a `t.TempDir` that the snapshot would land under.
- **Under the wall root, not `~/.cache/posse`.** The first draft used
  `posseCacheDir()`. That dir is write-granted to every caged seat (it is
  the box's lock dir), so one caged seat could rewrite a snapshot every
  sibling with the same global config then reads, `core.hooksPath` included:
  the exec inlet obdnk closed, reopened between seats.
  `~/.cache/posse-wall` is the directory ADR 0100 D2(a) keeps every render's
  grant off, and it reads through the seat's read-only `/`.

## Per tier

- **seatbelt / bwrap:** no render grants `~`, `~/.config/git` or the wall
  root (`TestQAGitGlobalSnapshotIsUnderNoCagedWriteGrant`), so a caged seat
  can't write the operator's file by any route, and can't write the
  snapshot either.
- **shims:** no kernel wall exists at this tier by definition (ADR 0025's
  cooperative class). What closes is the route both incidents took: a
  `--global` write from a process that thought it had moved HOME. Removing
  the lock directory on purpose, or naming `~/.config/git/config` by path,
  still writes. That is the tier, not this fix.
- **container:** unchanged. The row names a host path the container does
  not mount, as it did before.

## Degraded paths, said on the launch line

A global file with an `[include]` or `[includeIf]` section is not
snapshotted, because a relative include path resolves against the including
file's own directory. The row then names the original file and
`GitGlobalScopeDegraded` prints that a `--global` write in the session
WRITES it. Any failure to write the snapshot goes the same way: keeping
obdnk's inlet closed matters more, and the line says what's left open.

## Pins

`gitglobalsnapshot_qa_test.go` (arm 3) runs the incident's own lines under
the rendered row. It checks that the operator's file stays byte-identical,
that each write refuses at the config lock, and that the author ident
doesn't move. A control proves the old row (the operator's file) is
rewritten by the same line. Also pinned: no global file snapshots as an
empty file and creates nothing, includes degrade and say so, and a
tampered snapshot is rewritten on the next resolve. Five mutants (row back
to the file, no lock dir, includes ignored, no rewrite on tamper, snapshot
root back to `.cache/posse`) each turn a pin red.
