## Workspace ids recycle across a server process boundary (rangerhq-6bg7)

`w5V` is not a name, it is a **cursor**. herdr's workspace-id allocator is
`max(live workspace id) + 1`, recomputed from the live set every time a server
process starts — including a `--handoff` import. Nothing on disk carries a
high-water mark: `~/.config/herdr/session.json` (schema `version: 3`) persists
`id` per workspace plus `next_public_pane_number` / `next_public_tab_number`
*inside* each workspace, and no counter at the top level. Ids are base-36
after the `w` (`w5`=5, `w57`=187, `w5V`=211), monotonic **within one server
process** and only there.

Measured on a scratch session server (`herdr --session idprobe server`, its
own socket under `~/.config/herdr/sessions/idprobe/`, deleted after; the live
fleet server was never touched):

| step | result |
|---|---|
| fresh server, create ×4 | `w1 w2 w3 w4` — a new server starts at `w1` |
| close `w3` `w4`, create ×2 | `w5 w6` — **no reuse within a lifetime** |
| stop + start (live `w1 w2 w5 w6`), create ×2 | `w7 w8` — resumes past the live max |
| close `w7` `w8`, stop + start, create | **`w7` — recycled** |
| close everything, stop + start, create | **`w1` — counter fully reset** |
| close `w3`, `herdr server live-handoff --import-exe …`, create | **`w3` — recycled** |
| control: close `w3`, create in the *same* process | `w4` — still no reuse |

A live handoff does preserve pane and workspace ids — exactly, and more
narrowly than that reads: ids survive for workspaces that are *live* at the
handoff. Every id **above the live high-water is free real estate** on the far
side, and that set is precisely the set of ids stale metas hold — a meta whose
workspace died before the restart names an id the next server will hand to
somebody else. Worst case is the quiet one: bring the fleet down, restart
herdr, and the allocator restarts at `w1` and climbs back through the whole
range every old meta points into.

**What this costs ADR 0011 §2.** `WorkspaceAlive(id)` proves a workspace holds
that id, never that it is the one the meta recorded — the pid-recycling trap
with a different counter. And `workspace_not_found` is a **snapshot, not a
stable fact**: an id that answers not-found today can answer alive tomorrow,
for a stranger. The consequences are the ones §2 exists to prevent — the meta
is spared forever, `Sessions()` lists the wrong workspace under the old name,
and a prompting pass types into somebody else's pane.

**The socket guard does not catch this.** `cannotAnswerFor` compares the meta's
`socket:` against `SocketID()`, and the path is unchanged across both a restart
and a handoff (`~/.config/herdr/herdr.sock` either way) — same string, different
generation, different id space.

**What herdr will and will not tell you about identity.** `workspace get`
returns only `workspace_id`, `label`, `number`, `tab_count`, `pane_count`,
`active_tab_id`, `agent_status`, `focused`: **no creation time**, and
`api snapshot` carries no server pid, boot id or start time either — there is
no generation token in the API. Two anchors are real:

- **`label`** — posse already creates every workspace with `--label <session
  name>` (`herdr.go:231`) and the meta's own filename is that name, so the
  identity check needs no new field: a workspace whose `label` is not the
  meta's name is not the meta's workspace. (`terminal_id` and the root pane's
  `shell_pid` are *not* usable: both are regenerated when the pane's terminal
  is rebuilt, so they mark a legitimately restarted workspace as a stranger.)
- **the api socket's inode** — the socket file is recreated (new inode, new
  btime) by *both* a restart and a live handoff, verified. `stat(2)` on
  `socket:` is therefore an exact "same server generation" fence, purely local,
  no herdr call: same path + different inode ⇒ this server did not issue that
  id, so the id is evidence about nothing.

**What landed** (rangerhq-yt1p, `internal/posse/herdrback.go`). `gen:` in the
session meta — `dev:inode` of the api socket, stamped at create, backfilled
on positive identity — plus one predicate, `notOurWorkspace`, asked by all
three destructive paths through `idEvidence`: the prune (`Sessions`), the
create (`mustNotOrphan`, which `posse relaunch`'s unlink also calls), and the
listing itself. The listing is where the damage actually was: a stale meta
whose id a stranger now holds used to be listed *over* that workspace, and
every addressing path (`Resolve`, `AgentTarget`, `KillSession`,
`RelaunchAgent`) reads that listing — so the name prompted into somebody
else's pane and `posse kill` closed it. Such a meta is now kept, left out of
the listing, and reported with the repair recipe (the instance runbook's
post-flight carries it).

Two decisions worth keeping. The fence is **not** a third arm of
`cannotAnswerFor`: a generation mismatch there would keep every meta forever
after any restart, and it is not even true — `workspace_not_found` is still
proof of death in any generation, because a workspace never changes its id
while it exists (the ids that survive a restart or a handoff are the live
ones, unchanged). And the create **refuses** rather than repairing itself
onto the label-matched workspace: a repair is only ever right if the session
were alive under a *different* id, which cannot happen, so the post-flight
repair stays the operator's deliberate act.

One migration cost, and it is one-time and visible: a meta written before
`gen:` existed whose workspace was renamed in herdr reads as a stranger until
it is repaired or the session is recreated — kept, not listed, warned about
by name.

