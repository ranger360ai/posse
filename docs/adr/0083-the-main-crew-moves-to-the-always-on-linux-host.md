# ADR 0083 — the main crew moves to the always-on Linux host: phase 0, prep, one cutover window, one writer per store, the laptop kept as the rollback home

*Status: accepted 2026-11-08 (ranger-base-9nxdt, operator direction the same
day, after the shakedown crew ran end to end on the ADR 0080 host) · owner:
architect · scope: the operator's PERSONAL main crew and the stores it
dispatches · applies ADR 0080 (the host), ADR 0081 (hook ownership) and
ADR 0015 (promotion) · the cage itself is OPEN for an operator ruling (D6);
everything else here is decided · no step here is executed by this ADR*

## Context

ADR 0080 put posse on an always-on Linux host, and a one-repo shakedown crew
has run there end to end (ranger-base-offid legs 0-4: Rex seat, systemd
watch, merge-back, verify loop). The main crew still runs on the laptop. It
dispatches two bead stores:

- **the store of record**, a sqlite db in the queue repo (`queue_repo:`),
  reached from the posse repo and the constitution repo through
  `.beads/redirect` files. MEASURED 2026-11-08: both redirects are
  untracked and hold an ABSOLUTE laptop path, and the queue repo has **no
  git remote at all** — its only off-box copy today is nothing;
- **the private project repo's own store** (`beads:` lists it beside the posse repo),
  under its existing rules (ADR 0050), pane history off.

Three facts read from the tree decide most of the shape:

1. **The promoted copy cannot be hand-edited.** Every launch re-hashes the
   promoted set against `promoted.json`, and dispatch refuses on a mismatch
   (ADR 0015 §3). The shakedown crew's PID copies were hand-edited to
   `cage: shims` (offid leg 2), which works only in a home that was never
   promoted. The main crew's home is promoted.
2. **Six of the main crew's eleven PIDs demand `cage: seatbelt`** (MEASURED,
   the live home, 2026-11-08): the three code-writing personas by the
   2026-08-28 risk ruling (ranger-base-8hd), the two verifiers, and the
   security reviewer. The security reviewer also carries a bare
   `Edit`/`Write` deny, which nothing realizes at shims, so that persona
   refuses on shims whatever its `cage:` line says (ADR 0002 §4).
3. **Off darwin, the Claude login is a file.** `credentialReadDenyLiterals`
   (seatbelt.go) read-denies Claude's on-disk login file even to a claude
   seat. On the laptop the login is in the keychain behind a cooperative
   `Bash(security:*)` deny. On the host at shims, any seat can read the
   subscription credential with `cat`.

The field's names for the two hard parts: moving a store with **one writer
at all times** is a primary handover with a fence (the old primary is made
unable to write before the new one is allowed to: "fencing", Kleppmann,
*Designing Data-Intensive Applications* ch. 8). Letting one workload
definition ask for a capability that each machine provides differently is
**a class with a per-node handler** (Kubernetes RuntimeClass: the pod names
a class, the node's config names the handler that realizes it).

## Decision

**D1. Operator rulings, recorded.**
- **GitHub: one deploy key per project**, generated on the host as the
  service account. Write access only on the repos the crew lands into and
  the operator pushes from: the posse repo, the constitution repo, the queue
  repo, and the private project repo. No account-wide token. GitHub refuses one key on
  two repos, so each clone's remote uses a per-repo SSH host alias.
  The queue repo gains a **private** remote for its key. That is its first
  off-box copy, and its audience is unchanged (the operator alone).
- **The coordinator runs ON the host as a crew seat.** The operator reaches
  it through the Rex macOS app first, `herdr --remote` second.
- **The private project repo and its store move too**, under their existing rules.
- **claude is the only runtime on the host for now** (ADR 0080 D6).

**D2. Phase 0 is the shakedown's legs 5-8, read on the host before any
prep.** Status (a dispatched claude seat never reads `unknown` while live),
the Linux meter (ADR 0019 V1, first live reading), leaks (`checkorphans`'
first Linux reading, exit 2 is "unknown" and never "clean"), and capacity.
Capacity becomes the host's seat ceiling, and it is set as two numbers: the
`load_guard:` value from the host's quiet baseline, and the largest number of
claude seats the leg measured fitting in RAM with every home's other seats
up. posse has no live-seat cap today, only `-n` per epoch and the load
guard. If the measured ceiling is below the number of seats the host's homes
can field at once, a cap is a code bead, filed from that reading, and the
cutover waits for it.

**D3. One writer per store, at every instant.** A store is written by the
laptop home until the fence, and by the host home after the restore. Never
both, never neither-then-both. The cutover is:

    pause → drain → push → stop + FENCE → transport → verify → host writes

- **The fence is the store file's write bit.** After the last laptop write,
  the laptop's db files and their `.beads` dir lose write permission. A
  paused loop is not enough: pause stops spend, not oversight, and the
  epilogue still files verify beads (`posse pause` help). The fence also
  catches the operator's own hand-typed `bd` on the laptop. It is reversible
  with one `chmod`, which is what rollback needs. That bd 0.50.3 fails LOUD
  against a read-only db is ASSUMED, and it is measured on a copy before the
  window (bead below).
- **Transport is the archive posse already verifies.** `posse backup`
  stages the queue db through sqlite's online backup API, bundles the queue
  repo's git history and the jsonl, and reads the archive back before
  naming it. The operator copies it to the host and runs `posse backup
  verify --archive` there. The private project's store is not in that archive; it moves
  as `sqlite3 <db> ".backup <out>"`, the same API. Never `cp` of a live db.
- **Verify is equality, not a smoke test.** The host's `bd export` of each
  store equals the laptop's last export, byte for byte after sorting, before
  the host's watch is enabled.
- **Redirects are rewritten on the host by hand.** They are untracked, so
  nothing carries them, and they hold absolute paths.

**D4. Prep happens before the window and changes nothing the laptop
reads.**
- **Clones** at the same `~`-relative paths as the laptop's. `beads:`,
  `beads_visibility:` and `queue_repo:` are all `~`-relative, so the
  constitution's config needs no per-host path.
- **The host home** (ADR 0080 D8): not `~/.config/posse`, `instance:` set,
  `worktrees:` set, env sets recreated by hand (envs/ is machine-local
  state, never promoted, ADR 0015 §7), backup armed with a local
  `backup_dir:`. It is **born paused**: `state/pause.yaml` is written
  before the watch unit is enabled, so the unit's first pass declines.
- **Constitution promote on the host** is a dry run during prep
  (`posse promote --dry-run`) and real in the window, at the cutover SHA.
- **The host-fact keys** (`multiplexer: rex`, `autostart_via: systemd`,
  `autostart_multiplexer_unit:`, `instance:`, `worktrees:`) are committed
  to the constitution IN the window, not before. From that commit on, nobody
  runs `posse promote` on the laptop: the laptop's promoted copy is the
  rollback image, frozen at the last pre-cutover promote.
- **Hook ownership (ADR 0081)** is per clone, and the host's clones are new
  files, so there is no contended slot to hand over. On the host the main
  home marks the four repos in `beads_visibility:` and owns their walls; the
  shakedown home must not mark them. `posse gates install-hooks` from the
  main home stamps all four. The laptop's clones keep the laptop home's
  stamp and take no crew commits after the fence.
- **Pane history off for the private project repo.** One Rex server serves every
  home (ADR 0080 D2), and Rex's history switch is server-wide: `--restore=false`
  "starts without them and saves nothing" (MEASURED, `rex server run --help`,
  laptop build). The host's Rex runs with it. Whether Rex's saved state holds
  scrollback is ASSUMED; the prep bead reads the state dir after one session.
  Claude's own transcripts under `~/.claude/projects` follow whatever the
  laptop does for the private project repo today; the same bead reads that and repeats
  it, it does not invent a new rule.

**D5. The coordinator moves last, with its memory, through git.** Persona
memory is the `personas/` symlink into the constitution repo (ADR 0015 §5).
The laptop coordinator is killed with its landing turn (which commits its
standing orders), the constitution repo is pushed and pulled on the host,
and the coordinator is launched there as a crew seat (`posse crew`). The
operator attaches through the Rex app. The host's watch resumes only after
the coordinator is up and one canary bead has gone filed → hired → closed →
landed.

**D6. The cage — OPEN, for the operator (ranger-base-nllw7).** Three options,
priced:

| option | what the host runs | costs | ends when |
|---|---|---|---|
| **A. move on shims** | the six seatbelt PIDs at shims, by a ratified D7 map | the code personas lose the file wall the 8hd ruling bought; the security reviewer cannot run at all (bare Edit/Write); the Claude login and the four write deploy keys are readable by any seat (cooperative `push` deny only, and the posse repo is public) | never, by itself |
| **B. wait for bwrap** | nothing moves until the bubblewrap tier lands | the laptop keeps swapping and sleeping; bwrap is a probe (ranger-base-hft2q, operator, minutes), a design (ranger-base-rwcqg) and its build slices, so weeks | the tier lands |
| **C. move, then add bwrap** | A, with a known end | A's costs, for the window between the move and the tier | the D7 map flips from `shims` to the bwrap tier, one promoted line |

**Recommendation: C, gated on the probe.** Move only after
ranger-base-hft2q's output is in and every B-row reads as expected. The
probe costs minutes, and it turns A's open-ended weakness into a window with
a known end. If a B-row fails, the recommendation becomes B. *The gate is
met:* ranger-base-hft2q closed 2026-11-08 with the probe passing on the
host, only a cosmetic B5 print issue (READ from the coordinator's comment on
ranger-base-9nxdt, not re-read here), so C stands as recommended. Two
mitigations ride with C, and the operator can take either: the deploy keys
carry a passphrase and are unlocked only for a push in the operator's own
shell (no agent socket a seat could find); and the security reviewer's
lane stays empty on the host until the tier lands, rather than its PID being
rewritten weaker.

The laptop crew's posture today is not stronger on the keys: the seatbelt
profile denies no `~/.ssh` read (MEASURED, a grep of seatbelt.go), and the
laptop's key is the operator's account key. Per-repo deploy keys are
narrower even at shims.

**D7. A PID's `cage:` is the class it needs; `cage_realize:` names, per OS,
the tier that realizes it.** This is the per-host answer the shared
constitution was missing.

    cage_realize:
      linux:
        seatbelt: shims     # D6 option A/C; becomes the bwrap tier later

- The key is in `config.yaml`, so it is in the promoted set: ratified,
  hashed by the manifest, and shown in promote's diff. Keyed by GOOS, so
  no host name enters the constitution, and the laptop and the host read
  one file and get different answers.
- Built in: a tier maps to itself where `AvailableCages` has it. A demanded
  tier that is unavailable and unmapped refuses, as today.
- Mapping to a tier that realizes every rule of the PID is not degradation.
  Mapping to a lower tier is **ratified degradation**: the launch proceeds,
  dispatch included, because the waiver was ratified at promote and not
  invented by dispatch (ADR 0002 §4's rule is about who invents it, and it
  stands). The session meta is marked degraded with the reason
  `cage_realize: <goos> <from>→<to> (promoted <sha>)`, and `posse status`
  carries one standing row while any mapping lowers a tier.
- A map changes the cage and nothing else. Parity is recomputed at the
  realized tier, and a rule that tier cannot realize still refuses. The
  map cannot waive a rule.
- `container` cannot be mapped from or to. Its engine is per home already.

**D8. Rollback: the laptop home stays intact until the operator retires it.**
Nothing on the laptop is deleted or re-promoted by the cutover. Rollback is
the window run backwards:
- **before the host's first write** (anything up to the host's resume):
  stop the host unit, `chmod` the laptop stores writable, `posse resume` on
  the laptop. Nothing is lost.
- **after it:** pause the host, drain, fence the host's stores, transport
  them back the same way (D3), unfence and resume the laptop. The host's
  writes come back with the store.
- **Trigger:** the operator's call. The ADR names no automatic trigger,
  because a watch that rolls itself back is a second writer.

**D9. Afterwards the laptop is a client only.** The Rex app and
`herdr --remote` to the host, SSH for the operator's own pushes. The
laptop home is kept, fenced and paused, as the rollback image until the
operator retires it (a bead, not a date). Its herdr plugin link is removed
so a herdr restart starts no loop. The work instance is out of scope and
unchanged.

## Consequences

- The crew runs while the laptop sleeps, and the store of record gets its
  first off-box copy (the queue repo's private remote).
- Under D6 A or C, the host crew is weaker than the laptop crew on file
  writes and the Claude login until the bwrap tier lands, and the security
  reviewer's lane is empty. `posse status` says so on every read.
- The constitution gains one key with one reader (parity). The broader
  host-fact keys are not overlaid per host (rejected below): the
  constitution describes the one host that runs it, and the laptop's
  promoted copy is frozen as the rollback image.
- Each cutover step is a bead in the operator's hands, so the window's
  progress is in the graph, not in a transcript.

## Alternatives rejected

- **Hand-edited PID copies on the host** (what the shakedown did). Dispatch
  refuses a home whose promoted bytes differ from its manifest, and a wall
  lowered outside promote is the 89c54c3 route the 8hd ruling objected to.
- **Rewrite the six PIDs to `cage: shims` in the constitution.** It works on
  the host and lies everywhere else: a re-promote on the laptop would
  silently drop seatbelt where seatbelt exists, and the PID would stop
  saying what its rules need.
- **A per-PID map (`cage: {darwin: seatbelt, linux: shims}`).** It changes
  the PID grammar and its readers, and it takes six edits again when the
  tier lands. The need is the PID's; how a host meets it is the host's.
- **A general per-OS overlay of `config.yaml`** (`os/<goos>.yaml` for
  `multiplexer:`, `autostart_via:` and the rest) — the clever one. It would
  let one constitution run on two hosts at once. That is exactly what D3
  forbids for one store, and each overlaid key has a shell reader too
  (`plugin/autostart.sh cfg()`): ADR 0080 D9 showed what keeping three
  readers in step costs for ONE key. Priced, not built; it is the exit if a
  second host ever runs this constitution with its own store.
- **Pause as the fence.** A paused loop's epilogue still writes (verify
  beads), and pause does not stop the operator's own `bd`.
- **Transport by `bd import` from the jsonl.** `bd import` is a fenced verb,
  the jsonl is a projection of the db rather than the db, and the archive
  is already verified end to end.
- **A second Rex server for the private project's home**, to keep history on for the
  rest. ADR 0080 D12 rejected per-home servers; the cost of losing session
  restore on a host whose seats posse relaunches anyway is small.
- **Cut over one store at a time.** One home dispatches both stores in one
  queue (`beads:`); splitting them puts two homes on one crew's personas.

## Measured versus assumed

MEASURED, 2026-11-08, laptop and this tree: the six seatbelt PIDs and the
one bare Edit/Write deny (the live home); the redirects untracked and
absolute; the queue repo without a remote; dispatch refusing a manifest
mismatch (`posse promote` help); pause leaving the epilogue running
(`posse pause` help); Rex's `--restore=false` text; no live-seat cap key;
seatbelt.go's credential deny and absence of an `~/.ssh` deny.

READ from the shakedown (operator's comments on ranger-base-offid): legs 0-4,
including the hand-edited `cage: shims` and the unit's missing PATH.

ASSUMED, each measured by a bead below before the window: bd fails loud on
a read-only db; a restored archive's export equals the source's; Rex's saved
state holds scrollback; seat capacity (phase 0 leg 8); the bwrap B-rows
(ranger-base-hft2q).

## Beads

Phase 0 is ranger-base-offid (operator, legs 5-8). The rest are
`discovered-from` ranger-base-9nxdt. Operator-hands beads carry the
`operator-hands` label. The window's beads chain in the order below.

| bead | who | what | waits on |
|---|---|---|---|
| ranger-base-offid | operator | phase 0: shakedown legs 5-8 (D2) | — |
| ranger-base-nllw7 | operator | the cage ruling (D6) | — |
| ranger-base-1155n | developer | `cage_realize:` (D7) | — |
| ranger-base-i0yuw | developer | probe: the fence and the transport, on copies (D3) | — |
| ranger-base-zrja9 | operator | deploy keys; the queue repo's private remote (D1) | offid |
| ranger-base-cywju | operator | clones at `~`-relative paths; redirects (D3, D4) | offid, zrja9 |
| ranger-base-4s8a0 | operator | the host home, born paused; seat ceiling; promote dry run (D2, D4) | offid |
| ranger-base-jpgxo | operator | private project pane history off, measured (D4) | offid |
| ranger-base-19bt7 | operator | hook ownership on the host clones (D4) | cywju, 4s8a0 |
| ranger-base-j3r7t | operator | window 1: pause and drain the laptop | all of the above |
| ranger-base-68i8n | operator | window 2: push the four repos | j3r7t |
| ranger-base-l9c69 | operator | window 3: stop the laptop loop, fence | 68i8n |
| ranger-base-i1occ | operator | window 4: transport, prove equal | l9c69 |
| ranger-base-yuqhv | operator | window 5: host keys + map committed, promoted on the host | i1occ |
| ranger-base-d55jh | operator | window 6: the coordinator moves (D5) | yuqhv |
| ranger-base-c3tv0 | operator | window 7: host watch, resume, canary — the first host write | d55jh |
| ranger-base-hct28 | operator | the laptop becomes a client (D9) | c3tv0 |
