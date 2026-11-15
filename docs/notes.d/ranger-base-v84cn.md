# An empty deny directory gets an anchor file, so a host rmdir cannot take its mount off (ranger-base-v84cn)

2026-11-14. wall-watch killed bwrap seats five times between 00:52Z and
02:25Z over `~/.claude/mcp-skill-archives` (this bead and its duplicates
ranger-base-1kavg and ranger-base-5xwad). It was the same bwrap persona every time. The cause
is not a rename. Something on the host deletes empty directories in the
claude config dir. These seven were deleted together and recreated together
by the next bwrap launch's prep: `access-audit`, `logs`,
`mcp-discovery-cache`, `mcp-skill-archives`, `shares`, `statsig`, `todos`. The
recreation had one birth time to the millisecond and ran in alphabetical
order, which is the render's `mkdir -p` deny step. All seven are on
`claudeExecConfigDirNames`. The launcher makes each one empty and binds it
read-only over itself.

**MEASURED** (bwrap 0.13.1, Linux 7.2.15, scratch dirs):

- A read-only bind over an EMPTY dir: a host `rmdir` succeeds, the mount
  leaves the seat's mountinfo, and a write from the seat then lands in the
  host's directory. So the kills were correct: after the detach the seat
  could write there.
- The same bind with one empty file inside the dir: the host `rmdir` fails
  with ENOTEMPTY, the mount stays, and the seat's write is refused.

The seven are claude 2.1.325's own list: the
`["mcp-skill-archives","mcp-discovery-cache","shares"]` loop plus the `mPo`
four in its sandbox deny builder. Its Linux sandbox cleanup rmdirs a mount
point it finds empty, and its scrub rmdirs a "planted empty dir" at a deny
path (read from the binary). Both use `rmdirSync`.

**ASSUMED, not measured: which process does the removing.** No claude seat
on this box runs claude's sandbox (none has `sandbox` in its `--settings`,
and its Bash is not under bwrap), so the binary does not prove it is claude.

**The rule now.** `SeatbeltCarveOut.Anchor` lists the deny directories bwrap
keeps non-empty. The carve fills it with claudeExecConfig's dirs that are on
`claudeSweptConfigDirNames`. In `bwrapArgsInto`, an anchored deny that is a
directory (or is about to be one) gets a `seed <dir>/.posse-wall-anchor`
step before its bind. The step is O_EXCL and empty, so a second launch
leaves the anchor alone. Seatbelt renders the same entries as plain denies.

**What stays a kill.** A remover that empties the directory first, or
renames it, still takes the mount off, and wall-watch still kills the seat
(ranger-base-0qpsw D2). The anchor goes only on the seven measured names.
The directories claude loads code from (skills, commands, agents, hooks,
daemon, jobs, ...) get none, because an extra file there is not measured to
be inert. None of those directories was removed.

Pinned by `TestBwrapArgsAnchorsAnAnchoredDenyDirectoryAndNothingElse` and
`TestQAWallAnchorKeepsClaudesSweptConfigDirsFromAHostRmdir`
(internal/posse/wallanchor_qa_test.go). Mutation-checked: dropping the step
reds both, and dropping the directory guard reds the file row.
`wwEveryShape` carries an anchored deny, so wall-watch's render pins see it.
