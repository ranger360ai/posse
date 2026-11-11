# Verifying four closes: what the mutants said, and the sixth door (ranger-base-md7ag)

2026-11-11. Verify bead for ranger-base-8t6j1, ranger-base-hlnyx,
ranger-base-mbkmz and ranger-base-xdaw9. All four VERIFIED. Two things are
worth keeping past the close comment: one survivor, and one hole the
five-door fix did not reach.

## Where the evidence was taken

D1 host, linux 7.2.15, bubblewrap 0.13.1, git 2.56.0, bd 0.50.3, a live
claude 2.1.322 seat on a `multiplexer: rex` box. All readings at
`142840f3` (main), from this bead's own session worktree.

Each close landed as a REBASE, so the sha in the close comment is not on
main. Content checked per path rather than by `patch-id`, which differs for
all three of the rebased commits because `docs/notes.d/README.md` grows with
every other bead's fragment:

| bead | close named | on main | content difference over the commit's own paths |
|---|---|---|---|
| ranger-base-8t6j1 | `95453159` | `3c53f3a5` | README index rows; `parity.go`'s hook-skip residual sentence, which is `ranger-base-crlty`'s (`f445f129`) and not this bead's |
| ranger-base-hlnyx | `c60059d5` | `e3f0290f` | README index rows only |
| ranger-base-xdaw9 | `76acbcec` | `142840f3` | README index rows only |
| ranger-base-mbkmz | `83ef17ea` | `ee09594e` | none — identical `patch-id` |

ranger-base-mbkmz is a REPLAY of `d5f9e6ed`, so its own `patch-id` differs
from the original by design. 1 original : 1 replay, and `(author email,
author date, subject)` is carried verbatim, so `equivalentOnBase` pairs it:
the key appears exactly once in `main`. The branch and the
`refs/posse/merge-blocked/...` pin are both gone, which is the launcher's
pass.

## 14 mutants, 13 killed, one semantically equivalent

One mutant per door, applied by exact byte substitution with an md5 check
either side so a no-op could not score as a kill
(`scratchpad/mutate.py`, golden copies restored between). Every arm ran in
BOTH the arm-1 and the arm-3 filter, because ranger-base-xdaw9's pins are
`//go:build posse_arm3` and an arm-1-only run reports `ok` over them.

| mutant | verdict |
|---|---|
| `SeatbeltWritable`'s deny arm re-adds `<cwd>/.git` | killed, 7 arms across both arms incl. the live bwrap arm |
| `sessionGitGrantsFor` loses its guard | **SURVIVED** — see below |
| `sessionRefDirsFor` loses its guard | killed (the profile's create-only block) |
| `BwrapWritable` loses its early return | killed (the render arm + the live arm) |
| the parity ref-grant row stops being silent | killed |
| `recordTargets` loses its narrowing | killed |
| `RexMux.Prompt` loses the refusal call | killed, 4 arms |
| `captureComposerSight` answers unreadable for covered | killed, 5 arms + 7 on the herdr route |
| the Rex refusal stops being owner-scoped | killed (the codex concession arm) |
| the Rex refusal moves AFTER the two keystrokes | killed (the "types nothing" arms) |
| the Rex second look is removed | killed |
| `sendTextPrompt` loses the refusal call | killed, 6 arms |
| the pane refusal stops being label-scoped | killed (both concession arms) |
| the pane leading-'/' message reverted | killed (the substring pin) |

### The survivor is dead code, not a missing pin

`sessionGitGrantsFor`'s ADR 0086 D3 guard is UNREACHABLE. `SeatbeltWritable`
calls it in the `else` arm of `if deniesFiles`, where `deniesFileWrite(ag.Deny)`
is false by construction, and that is its only caller in the tree. MEASURED:
a `panic("guard WAS reached")` planted in that branch does not fire across
`-run 'Seatbelt|Bwrap|Parity|RecordReach|Cage|WholeTree|Redirect|Grant'` in
arm 1 or arm 3 — both `ok`.

So the rule in that arm is enforced entirely by the `if deniesFiles` branch
itself (the mutant that re-adds `<cwd>/.git` kills seven arms), while the
function whose forty-line comment says "sessionGitGrantsFor is where the
whole reason is written" and "IT ANSWERS NOTHING" is inert at its only call
site. ADR 0086 D3's as-built bullet counts it as one of the five doors. The
other four are live and each is killed by its own mutant.

Nothing a process does changes if the dead guard is deleted, so this is
RECORDED rather than filed. It matters because of the next section: the
five-door count includes a door that cannot fire, and excludes one that can.

## The sixth door: `beadsGitDirs`, and a store of record inside the session's own repo

ADR 0086 D3's headline is "a PID that denies the whole tree gets no write
grant into any git dir of its session", with one exception — the store of
record's own git dirs, "because bd commits there, and it is another repo"
(seatbelt.go's own words).

The code does not test for another repo. It tests whether the store is
already covered by what this PID was granted:

    grantedBeadsBase := cwd
    if deniesFiles { grantedBeadsBase = filepath.Join(cwd, ".beads") }
    if home := beadsHome(cwd); !underDir(grantedBeadsBase, home) {
            add(home)
            for _, g := range beadsGitDirs(home) { add(g) }
    }

So a `.beads/redirect` that stays INSIDE the session's repository but outside
`<cwd>/.beads` — `<cwd>/inner/.beads`, which is ranger-base-f5dg's REPRO 2
and the shape ranger-base-8t6j1's own `recordTargets` narrowing deliberately
keeps "granted and probed" — makes the store of record's git dirs and the
SESSION's git dirs the same directories, and they are granted WRITE.

MEASURED 2026-11-11 through the shipped producer, in a real linked worktree:

| PID | store | repo-local write grants |
|---|---|---|
| whole-tree deny | no redirect (control) | `<cwd>/.beads` and nothing else |
| whole-tree deny | `<cwd>/.beads/redirect` → `<cwd>/inner/.beads` | `<cwd>/.beads`, `<cwd>/inner/.beads`, `<cwd>/inner/.git`, **`<repo>/.git/worktrees/<session>`**, **`<repo>/.git`** |

The last two are W14/W15 as D3 states them, and WIDER than the grant D3
removed: `<repo>/.git` is the common dir WHOLE, not narrowed below it the way
`sessionGitGrants` was (ranger-base-m2wf), so `objects` and `refs/heads` are
both writable — a seat that cannot edit a file mints a commit by plumbing and
moves `posse/<session>`, which the launcher fast-forwards onto `main`. In a
main checkout the same grant reaches `refs/heads/main`.

Both the other mechanisms agree with it rather than catching it:
`BwrapWritable` returns early for this PID class ("no git grant to swap"), so
the set it hands `BwrapArgs` is this one unnarrowed — MEASURED, both entries
survive — and `recordTargets` keeps the git dirs as targets in this shape, so
the record-reach gate PASSES over the grant.

NOT EXPOSED ON THIS BOX. All three stores here are reached through a redirect
into another repository, so every dispatched launch
takes the branch where the exception is what D3 says it is. ADR 0038's own
Context paragraph shows the assumption: it names the `beadsGitDirs` clause as
"when `.beads/redirect` points at another repo".

Held open by `internal/posse/seatbeltdenyinrepostore_qa_test.go`, a live-defect
pin that ships GREEN with the inversion in its failure message. Shown able to
fail: with the candidate fix planted — skip `beadsGitDirs` when `deniesFiles`
and the store resolves under `cwd` — the pin reds with that message and
NOTHING ELSE in `-run 'TestQAWholeTreeDeny|TestSeatbeltFollowsTheBeadsRedirect|TestSeatbeltWithoutARedirect'`
moves, so the fix is one branch wide.

## What else was checked and held

- The live Rex pin run against a SECOND seat (this one, not the closing
  seat's): `RHQ_LIVE_REX=1 ... TestLiveRexPromptSeesThisSeatsOwnComposer` →
  PASS, `live sight = 2 (open=2), last line = "⏵⏵ auto mode on (shift+tab to
  cycle)"`. That is the guard on the one hazard an absence rule buys, and it
  now holds on two independently captured live screens.
- `TestLiveBwrapWholeTreeDenyGetsNoGitWriteInItsSession` runs here (real
  bwrap, real linked worktree) and is killed by two of the mutants above, so
  it is not decoration.
- Every posse prompt route goes through `Mux().Prompt` — seven call sites
  (dispatch's two, the pulse, relaunch, the runtime probe, the cockpit,
  `posse prompt`) — so ranger-base-hlnyx's claim (b) holds and `--now`, which
  is handled before `Prompt`, cannot skip the refusal. `RexMux.Start`,
  `SendText` and `SendKeys` press keys outside it and are the shell launch
  and the operator's own hands; `ConfirmSubmitted` deliberately presses
  nothing (ranger-base-6o7wm).
- `agent_not_ready` was already in `isNotDelivered` before both closes —
  neither widened dispatch's hand-back allow-list.
- ranger-base-mbkmz's citations resolve: `exitNoneDelayed = 3`
  (cmd/coldexec/main_linux.go:40), `COLDEXEC_REPORT` gone from the tree,
  `POSSE_COLD_EXEC_REPORT` only in `coldexec_test.go` and the dated fragment,
  97w45's `$(COLDEXEC_BIN)` recipe byte-identical, the notes index
  regenerated with both rows, and `ranger-base-xpxo6` / `ranger-base-7zkxw`
  both open and named correctly. `coldexecinject_test.go` is byte-identical
  to the original.

## Prose left stale by ranger-base-8t6j1 (recorded, nothing filed)

- `internal/posse/seatbelt.go`, the redirect block's comment: "for a
  deniesFiles persona cwd itself is never granted (only cwd/.beads and
  cwd/.git, above)". The `cwd/.git` half is what D3 removed, twenty lines up.
- `docs/adr/0038-git-identity-write-deny.md`, Context: "a PID that denies
  Edit/Write is granted `cwd/.git` whole (seatbelt.go, SeatbeltWritable)".
- `docs/notes.d/notes-write-overlays.md`: "L2 grants a write-denying PID
  `cwd/.git` for index refresh and git's own locks, and a tier that took it
  away would be enforcing more than the gate" — the premise for the L4
  carve-out, now removed at L2 by D3 and at L4 by ADR 0086 D8
  (ranger-base-56w67; the code is ranger-base-sk13s, open).
- `internal/posse/muxrex_qa_test.go`, the leading-'/' arm's comment: "the
  picker eats the Enter and the text sits in the composer while the call
  reports success", with no "on an older CLI" qualifier — twenty lines from
  the message ranger-base-hlnyx corrected for exactly that reason. The
  production comment above `Prompt` states the same thing and then corrects
  itself in the next paragraph, so only the test comment is left saying it
  flatly.
