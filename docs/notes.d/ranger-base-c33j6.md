# The Rex positive pass, and the three readings that take the launcher's environment for the seat's (ranger-base-c33j6)

2026-10-31. ranger-base-phq39 measured five of the six done-whens of
`multiplexer: rex` and left the positive half — a claude seat read from
`idle` through `working` to settled, its bead closed and its branch landed —
saying it needed "a seat outside this box's wall". It did not. What it needed
was a different diagnosis, and the defect phq39 filed as "one thing to check
while there" turned out to be the whole of the blockage.

Everything below is against **Rex CLI 0.0.1793377904 (API revision 8), Rex
Beta 0.1.0, claude 2.1.316, posse at a4afaa5a, this box**, on a scratch
`multiplexer: rex` home with a throwaway repo, one ready bead and a
`worktrees:` under `$HOME`.

## 1. "Not logged in" was never the cause

phq39 read the positive pass as unreachable from a gated seat because
ranger-base-eupf's wall — the launching persona's gates dir leads PATH, so a
hired claude reads its own OAuth credential through that persona's
`Bash(security:*)` shim — made the seat answer `Not logged in`. The reading
was of a hand-built shell, and the inference does not hold:

- MEASURED: `claude config list` inside **a working dispatched jian-yang
  seat** answers `Not logged in · Please run /login`. That seat is plainly
  running. A child shell's view is not the seat's state.
- The reason is ADR 0042 D1 working exactly as written. A crew runtime never
  held the operator's rotating pair: it runs on the session mint posse
  injects from an env set the PID declares (`cage_cred:` names which env var
  that is, per runtime — runtimecheck.go's `cage_cred` stage row prints it
  for this instance). The runtime consumes that var and does not hand it to
  the Bash children it spawns, so a child has never held the credential and
  says so.
- posse agrees, in its own surface: `posse gates builder` on the scratch home
  prints `✓ Bash(security:*) → cooperative (L1 shim (whole verb))` and no
  credential refusal at all. `CheckCredGate` (gates.go, ADR 0042 D2) is
  satisfied by the mint being among the env-set names — "with the mint
  present it is the design and the launch says NOTHING", which is that
  function's own head comment.

So the credential wall is not in this path, and `--allow-degraded` over the
one remaining parity row removes nothing: posse prepends the HIRED persona's
gates dir, so the seat gets its own shims first and the launcher's only for
verbs its own PID does not shim. That is strictly more refusal, never less.

## 2. A Rex seat's environment is the SERVER's, fixed at server start

This is the fact both of this bead's findings come out of, and it is not
written down anywhere in ADR 0076.

`GatePrefix` types `PATH=<hired bin>:"$PATH"` in front of the runtime's
command, and that `$PATH` expands **in the Rex shell**, whose environment
comes from the Rex server process. MEASURED, by probing a
`rex new … -- /bin/sh -c 'printf "%s" "$PATH"'` session from inside this
seat:

| var | in the launching seat | in the Rex session |
|---|---|---|
| `CLAUDE_CONFIG_DIR` | `~/.claude` | **UNSET** |
| `RHQ_HOME` | the live home | **UNSET** |
| gates dir on `PATH` | `…/gates/jian-yang/bin`, leading | `…/gates/jian-yang/bin`, mid-PATH |

The gates dir in that third row did not arrive from this process. `rex server
status` reports the server `started: 2026-10-31T04:08:03Z`, hours before this
session existed — so some earlier jian-yang seat autostarted it and **donated
its PATH to every Rex seat the server will ever open**. Nothing resets that
but a server restart.

Two consequences:

- **Parity reads the wrong PATH on a Rex home.** `ForeignGatesElements`
  (parity.go) resolves the LAUNCHER's PATH, which on Rex is not the PATH the
  seat will have; the remedy the refusal prints — "relaunch with a clean
  PATH" — is advice about the wrong process. On this box it happens to be
  *accidentally* right, because the server's PATH carries a gates dir too, and
  for a reason nothing in the message names. Filed as ranger-base-u6c3u.
- **The trust seed targets a file the seat does not read.** §3.
- **And posse's own children resolve against that donated PATH too**, which
  is how a dispatch from a gated seat deadlocks its own bd claim — §4.

## 3. The trust dialog, which is what actually refused the launch

`SeedClaudeTrust` writes into `ClaudeConfigFile()`, which honours the
LAUNCHER's `CLAUDE_CONFIG_DIR`. The seat's claude resolves in the server's
environment, where that var is unset, so it reads `~/.claude.json`. For an
operator shell the two agree (neither sets the var). For a dispatch run from
inside a claude seat they do not, because the runtime exports
`CLAUDE_CONFIG_DIR` to its children.

MEASURED, in order, on a repo neither file had ever seen:

1. `posse dispatch -n 1 --allow-degraded` created the Rex seat and typed the
   runtime's line. The key
   `projects["<repo root>"].hasTrustDialogAccepted = true` landed in
   `$CLAUDE_CONFIG_DIR/.claude.json` and was **absent** from
   `~/.claude.json`.
2. The launch was refused by ADR 0076 D2's first-report gate: `no
   program_status record from the runtime within 45s`, `NOTHING WAS TYPED
   into it`, bead untouched, session left running.
3. `posse peek` on that session showed the reason on the screen, in full:
   claude's **"Quick safety check: Is this a project you created or one you
   trust?"** trust dialog. A seat sitting on that dialog writes no
   `program_status` record, so the gate was reading the seat correctly and
   phq39's refusal was never about credentials.
4. The key itself was right throughout: `scripts/claude-trust-key.sh` over
   the session tree answers the repo root, which is what posse wrote. This is
   the FILE, not the key.

**And from a caged launcher the correct file cannot be written at all.** With
`CLAUDE_CONFIG_DIR` shed for the posse process alone — so that
`ClaudeConfigFile()` answers `~/.claude.json`, the file the seat reads — the
same dispatch refused one step earlier: `open ~/.posse-seed-2815198631:
operation not permitted`. `writeJSONInPlace` renames through a temp file in
the target's own directory, that directory is `$HOME`, and a `cage: seatbelt`
seat cannot write there. So a dispatch from a caged seat has two ways to lose
this and no way to win it: seed the file the seat will not read, or fail to
seed the file it will.

Not Rex-specific — a herdr pane is a child of the herdr server the same way.
Filed as **ranger-base-u6c3u** with the other two surfaces of the same root;
trust.go carries this measurement at `SeedClaudeTrust`.

## 4. The positive pass

Accepting that one dialog by hand — `rex send-key … Down`, then `Enter`, the
same remedy trust.go's own errors name ("run `claude` in <dir> once and
accept the dialog by hand") — made the seat trusted in the file it actually
reads, and `posse list` immediately read it `idle` off the runtime's own
report.

Two mechanical obstacles stood between that `idle` and the prompt, and both
are the SAME foreign gates dir the parity row names — which is the row
`--allow-degraded` waives, so this is what that waiver actually costs:

- **posse deadlocked its own claim on ADR 0073's bd write lock.** MEASURED,
  twice: `bd --no-daemon --actor builder update c33j6-1 --claim --json:
  refused by posse gate: <store>'s bd write lock was held for 30s`. Nothing
  was holding it — the lock file read FREE on a non-blocking `flock` straight
  after, and its own stamp named the waiting call's pid. posse takes the
  per-store lock and then execs `bd`, `bd` resolves against the launcher's
  PATH, the launcher is a persona seat whose gates dir leads it, and that
  dir's `bd` shim takes the same lock before exec'ing the real binary (the
  gates bd-write arm even clears FD_CLOEXEC so bd inherits it). posse waits
  30 s on itself and refuses. `RHQ_BD_BIN` pointed at the real binary is what
  got past it, which is the surgical knob and not a wall removal: the PID's
  own `bd` denies still bind every bd typed in the launching seat.
- **A caged launcher cannot write `~/.claude.json` at all** — §3's last
  paragraph.

With both out of the way, in one pass:

```
· c33j6-1  herdr cannot explain builder-repo-c33j6-1 (… ADR 0076 §4) — prompting on its "idle" anyway
· c33j6-1  → builder-repo-c33j6-1  (prompted, standard via PID)
… 1 prompt(s) in flight, gathering
✓ c33j6-1  closed by builder
⤴ c33j6-1  1 commit(s) fast-forwarded from posse/builder-repo-c33j6-1 onto main in <repo>
reaped builder-repo-c33j6-1 (bead c33j6-1 closed)
  <session tree> had no commits — worktree and posse/builder-repo-c33j6-1 removed
1 bead(s) dispatched
```

Read back, every one of them off the store and the tree rather than off that
output: the bead is `closed`, assignee `builder`, carrying the seat's own
comment; the repo's `main` holds `ae421fe c33j6-1: add HELLO.md` with the one
line the bead asked for; `rex ls` is back to the one pre-existing foreign
session; the session tree and `posse/builder-repo-c33j6-1` are gone. And
`~/.claude.json` now carries the repo's trust key, written by claude itself
when the dialog was answered — which is the last confirmation that it is the
file the seat reads.

The first-report confirmation also stood down correctly rather than silently:
the `herdr cannot explain … prompting on its "idle" anyway` line is ADR 0076
§4's reading-not-available arm, and the prompt went in on the runtime's own
report, which is the whole of §3.

One number this run cannot give: `idle-to-next` printed `first launch into
this seat on record — no settle to measure from`. A settle-to-settle interval
needs a second dispatch into the same seat, and the reaper retires the seat
with its bead.

## 5. What this leaves

- `multiplexer: rex` is confirmed end to end for claude: occupant selected by
  the key alone, admission refused for the non-reporting runtimes before any
  claim, a Rex seat created and typed into, the first-report gate refusing
  with nothing typed when no record comes, the seat read from its own
  `program_status` report across a turn, and `posse kill` destroying the Rex
  session and retiring the tree.
- What is filed is not Rex's. Three surfaces — the trust file, parity's
  foreign-gates reading, and posse's own bd claim deadlocking on the
  launcher's `bd` shim — are one undecided question: which environment posse
  should treat as the seat's, and what it should hand its OWN children when
  the launcher is a gated seat. ADR 0076 §2 settles that the slot is the
  client and not the backend; it says nothing about the environment, and that
  is the gap. Filed as **ranger-base-u6c3u** (P1, `-l architecture`).
- The positive pass needed `--allow-degraded` and one hand-answered trust
  dialog. Neither is a wall coming off: the waived row puts the launcher's
  shims BEHIND the hired persona's, which is more refusal and not less, and
  answering a trust dialog for a repo this bead created is the remedy
  trust.go's own errors name. Nothing stripped a gates dir from anything, and
  `Bash(security:*)` was never in the path — §1.
