# claude's sessions/ registry, private per seat at bwrap, and the rex join reads the private row first (ranger-base-lx5jk)

*2026-11-13 · dinesh · design ranger-base-tii0u (D1, D2, D4 bwrap half)*

## What changed

- **bwrap (D1)**: `sessions` joins `claudeSessionStateDirNames`
  (`internal/posse/wallclaudesessionstate.go`), so each caged claude seat
  gets a private `<seat cache>/claude/sessions`, bound over
  `<CLAUDE_CONFIG_DIR>/sessions` after the config-dir grant. The seat
  registers into its own copy, sees no peer's row or key, and cannot write
  one. The render's existing lstat refusal covers a link at the member's
  name, as it does for every other private member.
- **posse's reader (D2)**: the rex join (`claudeSessionOfPID`,
  `internal/posse/runtimesession.go`) now reads
  `SeatCacheDir(home, readlink /proc/<owner pid>/cwd)/claude/sessions/<pid>.json`
  first and the shared row second, with the procStart fence on each. The
  cwd reading is `procCwdOf`, beside `procStartOf` in `procstart_linux.go`;
  off linux it answers "not available", and the join already stops at
  `procStartOf` there. The home is `HerdrBackend.ClaudeSeatHome`, filled by
  `NewHerdrBackend` from `claudeSeatHome()` — the one home read
  `claudeHistoryPath` now goes through too — and left empty by every test
  backend, the same discipline as `ClaudeHistory`. No home or no history
  store means no private read.

Without D2, D1 would have turned off sentline's echo reading for every caged
seat, because their rows would no longer be in the shared dir.

## The hole this closes

`<cfg>/sessions/<pid>.json` is claude's peer registry. Every claude on the
box — uncaged ones included — reads it for peer discovery and connects to a
row's `messagingSocketPath`, sending the token from the matching
`<pid>.<hash>.key`. Before this, a caged bwrap seat could write that dir, so
it could repoint a peer's row (or add a row that reads as live) at a socket
it binds, and write the matching key. An uncaged session that then messaged
that peer would send to the seat, and the reply would land in its context.
The same write could hand posse's rex join a wrong `sessionId` for any seat,
which misdirects sentline's echo verdict.

After this, at bwrap, the seat's writes land in its private copy, and only
the host can write the shared dir. Either row the join reads is honest about
the pid it names: a seat that lies in its own private row misleads only its
own echo reading.

## Pins

- `TestQAARexJoinReadsTheSeatsPrivateRowBeforeTheSharedOne`
  (`runtimesession_qa_test.go`): private row answers before the shared one;
  absent, fence-failing or wrong-pid private rows fall through to the shared
  one; no home reads no private row; no history store reads nothing; a test
  backend carries no `ClaudeSeatHome`; and the backend door hands the field
  to the join.
- `TestLiveWallClaudeSessionStateKeepsThePeerRegistryPrivate`
  (`wallclaudesessionstate_qa_test.go`), under the real bwrap render: a
  bystander row and key in the shared dir are invisible to the seat and
  stay byte-identical through a repoint, a rename-over, a key rewrite, a new
  live-looking row, moving the dir, relinking it, and writing through a
  link to it; the shared dir's names do not change. A process under the
  wall registers as claude does (its pid, its kernel start time); the row
  lands in the private dir and not the shared one; and with a host-side
  fenced decoy row for the same pid in the shared dir, the join still
  answers the seat's own sessionId.

Control, MEASURED 2026-11-13 on this host:

| mutation | reds |
|---|---|
| `sessions` out of `claudeSessionStateDirNames` | live pin: bystander visible, every plant changes the host's row or key, the new row appears in the shared dir, the shared dir is moved and relinked |
| the private-first read skipped in `claudeSessionOfPID` | the join pin, and the live pin's join answers the decoy |

## Costs

- A caged seat's own row lives under its seat cache and goes with it. Peer
  messaging between a caged seat and any other session is gone at bwrap: it
  sees no peers, and none see it.
- Like every private member, a link left at `<cfg>/sessions` stops the next
  caged launch rather than being mounted over.
- ASSUMED (from the design): claude never changes its own process cwd, and
  the render's home equals `os.UserHomeDir()`.
- Live seats get this only once the pending install lands
  (ranger-base-wi08c).

## Not closed here

- ranger-base-yaha3
- ranger-base-a544g
