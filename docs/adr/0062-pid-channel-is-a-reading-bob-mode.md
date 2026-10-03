# ADR 0062 — The PID reaches Bob as a posse-rendered custom mode in the session tree, and a launch line with no PID channel refuses dispatched work

*Status: accepted 2026-10-03 · owner: architect · source bead
ranger-base-f1ytb (from ranger-base-5jjtn) · supersedes ADR 0060 D3 ·
amends ADR 0013 §1's launch row ("PID delivered" becomes a reading, not a
sentence) · sits under ADR 0012 D4 (adapter seams), ADR 0007 (a
session-local tree in the cwd, excluded from git) and ADR 0017 §3 (declare
the dimension, never key on the name) · measurements in
`docs/notes.d/ranger-base-f1ytb.md` · amended 2026-10-03 (ranger-base-er6mt):
D1 ruled to STAND with `hidden` measured inert — see "Ruling" below*

## Context

ADR 0060 D3 put the PID on `bob -p "$(cat {file})" chat …` as Bob's first
user message. MEASURED FALSE 2026-10-01 (ranger-base-5jjtn): `bob chat`
parses the program-level `-p` and ignores it, top-level `-p` is headless
and key-gated, so **no argv shape on bob 2.0.5 opens the TUI with a prompt
already submitted**. 5jjtn removed `-p` and `{file}` from `BobCommand`, so
today the built-in delivers no PID at all, and a dispatched Bob seat opens
carrying every native rulebook and no persona — PIDVoided's exact harm,
reached by a template that names no flag to refuse on.

Two things were true that 0060 did not weigh. First, `bob chat --mode
<slug>` selects a *custom mode*, and a custom mode's `roleDefinition` is
the `role_definition` section of Bob's own system prompt — a launch-time
system channel, which 0060 said Bob had none of. 0060 priced only the
global-file variant (`~/.bob/settings/custom_modes.yaml`, the operator's
own file) and rejected it on single-writer grounds. Second, Bob loads
modes from the **workspace** too: `<workspace>/.bob/{custom_modes.yaml,
plugins/*/custom_modes.yaml}`, where workspace is `-w`, which posse already
renders as `.` — the session directory. posse already writes a
session-local tree there for this very runtime (`.agents/skills`, ADR
0007, hidden from `git status` by `.git/info/exclude`).

MEASURED 2026-10-03, this box, bob 2.0.5, scratch workspace, no turn and
no spend (recipe in the notes fragment):

- `<ws>/.bob/plugins/posse/custom_modes.yaml` declaring one mode (slug
  `posse-f1ytb`, `hidden: true`, the agent mode's ten tool groups) plus
  `bob chat --accept-license --trust --auto-approve -w . --mode
  posse-f1ytb` → the footer reads **`Posse f1ytb probe Mode
  (auto-approve)`**. The mode loaded from the plugins glob and `--mode`
  selected it.
- The same line with `--mode nosuch-f1ytb` → `(ℹ) Unknown mode
  "nosuch-f1ytb". Falling back to "agent" mode.` and the footer reads
  `Agent Mode (auto-approve)`. **An unknown slug is a fallback, not a
  refusal** — one grey line on screen and a session with no persona in
  it. That is the `.allowUnknownOption()` class 5jjtn paid for, one layer
  up, and it is why D2 below reads the footer before it types.

And the grid: `runtime check`'s launch row prints `PID delivered by the
template` unconditionally (`launchRow`, runtimecheck.go) — a sentence
with no reading behind it, the fourth of its class (9r33 danger, vbp3
declared screen, i3q6g detection, now the PID). MEASURED by reading the
call sites 2026-10-03: the only PID-delivery check anywhere is
`PIDVoided`, which asks whether a *flag* voids the PID and never whether
the template *carries* one.

## Decision

**D1 — The PID reaches Bob as a custom mode posse renders into the
session tree, selected on the line by a `{mode}` placeholder.** Properties,
not helper names:

1. Before the launch line is typed, on every path that renders a persona
   line (the create, the relaunch, the in-place retype, the probe), posse
   writes `<session dir>/.bob/plugins/posse/custom_modes.yaml` holding
   exactly one mode: slug `posse-<persona>` (the persona name reduced to
   `[a-zA-Z0-9-]`, Bob's slug alphabet), `name` the persona's name,
   `roleDefinition` the PID body verbatim, `groups` the built-in agent
   mode's ten LESS `mode` (`read edit command browser mcp skill todo
   artifact subagent` — MEASURED by reading the bundle that an omitted
   `groups` is `[]`, a mode with no tools; `mode` is the `switch_mode`
   tool and granting it would let the model leave the PID mid-session,
   claim 9 and the amendment below), `hidden: true` — a declared
   intent and NOT a boundary: MEASURED FALSE, claim 6, and the Ruling
   below is why the choice stands without it. The directory
   is excluded the way `.agents/skills` is (ADR 0007: `.git/info/exclude`,
   session-local, never the repo's). It is the skills tree's sibling: same
   paths call it, same lifecycle, same cage behaviour (the session dir is
   mounted; nothing under the home is written).
2. `{mode}` renders to `--mode posse-<persona>` on a runtime that
   declares a persona-mode materializer, and to nothing on one that does
   not — the `{skills}`/`{allow}` shape (ADR 0012 D4: a seam on the
   Runtime, no `rt.Name` branch anywhere; ADR 0017 §3). `BobCommand`
   becomes `bob chat {mode} --accept-license --trust --auto-approve -w .
   {allow} {deny}`. The pin 5jjtn inverted still holds: no `-p`,
   `--prompt` or `$(cat …)` on the line.
3. posse never overwrites a file it did not write: a `.bob/plugins/posse/`
   path the repo tracks, or that holds content posse cannot recognise as
   its own, refuses by name (the `.agents/skills` rule, "not written by
   posse — not overwriting").
4. The project-config trust check (`bobProjectConfig`, parity.go) is
   widened to the plugins glob Bob actually reads — `.bob/plugins` as a
   whole-directory presence, the same predicate as `.bob/settings` —
   **except** posse's own `plugins/posse/` entry. A repo shipping
   `.bob/plugins/<anything else>/custom_modes.yaml` is handing Bob a
   system prompt and a tool-group grant, which is the repo→box channel
   that check exists to refuse on; posse's own file is not the repo's.
   (Found here, unmeasured before: the declared list missed this glob.)

**D2 — The mode taking is an observable, read before the first keystroke.**
Because an unknown slug falls back in silence (Context), the launch does
not trust that the file it wrote was read. Before the work prompt is typed
into a Bob pane, the launch reads the pane's own text for the footer that
names the mode (`<name> Mode`, the string the probe matched) and refuses
to type — by name, quoting Bob's fallback line when it is on screen — if
the footer names `Agent Mode` instead. A pane read, not a keystroke, and
not a new reader: `PaneRead` (herdr.go) and `pane wait-output` already
exist and were what measured this. Bounded by `startup_wait`, like the
label it already waits for (ADR 0061 D2). Where the gate lives is the
builder's; what it must not do is type a work prompt at a Bob that fell
back.

**D3 — "PID delivered" is a reading, and a bead-carrying launch whose
rendered template consumed no PID refuses.** One function answers, for a
rendered-from template, *which channel delivered the PID*: `{file}`,
`{mode}`, or none. Three surfaces read it and cannot disagree (9r33's
one-rule-three-surfaces shape):

1. `runtime check`'s launch row says which placeholder delivers the PID,
   or that the template carries none; the preflight carries a blocking
   `pid` gap in the second case.
2. `agent check` (pidcheck.go) adds the `{model}` finding's sibling: a
   PID's own `command:` that names no PID channel.
3. The launch: **dispatched refuses, interactive warns** (ADR 0013 §1
   property 5, ADR 0015 §3), beside `PIDVoided` on both paths that render
   a persona line, before anything is spent. The line says the template
   carries neither `{file}` nor `{mode}` so no PID text reaches the CLI,
   and that the session would open carrying every native rulebook and no
   persona at all — PIDVoided's words, because it is PIDVoided's harm.
   Interactive warns because the operator at the keyboard can paste the
   PID, and because `posse runtime probe` renders a persona line and is
   the surface the instance side measures with; a probe that refused on
   this could never measure the channel that lifts it.

Force, stated: the day D3 lands and 5jjtn has landed, **Bob dispatch
refuses** until D1 lands — correctly, since today it spends a worktree, a
pane and a `startup_wait` on a session with no persona. ADR 0061 D1
lifted the detection refusal and this does not re-impose it; it is a
different row of the same grid, and it lifts the moment the template has
a channel. `PIDVoided` keeps refusing interactive too — it asks about a
flag the operator wrote into their own PID, where the remedy is editing
that line; this check asks about a template the operator may not own.

## Ruling 2026-10-03 — D1 stands without `hidden` (ranger-base-er6mt)

The question (ADR 0006 §1, exclusive ruling): D1 was written with `hidden:
true` among the rendered file's properties and the global file was rejected
partly on "every persona in their `/mode` menu". Claim 6 measured the field
inert. Does the session tree still win, and what did the field's loss cost?

**It stands, and the reasoning is re-priced rather than the Decision.** The
do-nothing option — leave D1 as written — is the ruling's shape, because
D1's weight never rested on the field. Each ground, re-read with `hidden`
removed, session tree against the two home-glob alternatives:

| ground | session tree | `~/.bob/settings/custom_modes.yaml` | `~/.bob/plugins/posse-<p>/` |
|---|---|---|---|
| where the mode is listed and selectable | one directory, the session's; the last persona launched there (MEASURED: the file holds one mode, `PersonaModeFile` does not vary by persona) | every workspace on the box, every persona ever launched (MEASURED: the home glob, notes f1ytb §1) | every workspace, every persona (same glob) |
| lifetime | dispatch: dies with the worktree (MEASURED: the tree is retired). shared checkout: the file stays until the next launch there rewrites it (MEASURED: no reader removes it) | outlives every seat; a sweep joins the build | same |
| inside the cage | rides in with the mounted session dir (MEASURED, qllt8) | invisible — the cage has its own home | same |
| write under the operator's home | refused by name (se81d) | is the write | is the write |
| posse rewriting a file the operator edits (ADR 0022) | no: a namespace bob gives away, never-clobber reads the marker | yes: it is their file | no |

Every row that chose the session tree is intact; the one cell `hidden`
touched is the first, and it moved on both sides as the bead said: the
session tree's exposure went from "none" to "one persona, in one
directory", the global file's from "every persona, hidden" to "every
persona, in every workspace". Strictly narrower on both axes. The margin
shrank; the sign did not change.

**What D1 gains, written down as a property and not a "should" (the bead's
option 2, rejected).** Wherever the operator opens `bob` in a directory a
seat used as its session dir, the persona's mode is the fourth Shift+Tab
stop and `Tab` in the `/mode` picker prints the PID. For dispatch that
directory is `~/.posse/worktrees/<repo>/<session>`, which the operator does
not open. For a crew session it is the shared checkout — by design, not
by accident: "a crew session is the operator's own conversation in the
operator's own checkout" (ADR 0008, quoted from `herdrback.go`'s
`Worktree` field), and the skills tree, the hooks and the gates already
land there for the same reason. So the exposure is: the operator, in their
own checkout, can read their own persona file from their own CLI's picker.
No visibility line moves — the PID's audience was the operator and the
seat, and it still is. The cost is one extra mode stop and the chance of
selecting it by hand, which the footer names and Shift+Tab reverses. A
`posse new --worktree` would remove even that, but it changes the crew
model ADR 0008 chose, nothing measured demands it, and ranger-base-q114b
already carries it as the root-cause note for the `$HOME` sibling; the
`$HOME` case itself — the only one where the exposure is box-wide — is
refused by name (Consequences, se81d). Not filed.

**Found on the way, the shared checkout with two bob personas.** D1's
materializer says "one session directory, one persona" and rewrites one
file; ADR 0007 settled the same directory as the REPO's, hosting several
personas with union semantics; ADR 0022 is the record of two writers in one
checkout. So in a shared checkout the second bob persona's launch replaces
the first's mode file (MEASURED by reading `RenderPersonaModeFor`: same
path for every persona, both files carry the marker, so never-clobber
passes). Is a live seat hurt? **No, MEASURED 2026-10-03 by reading the
2.0.5 bundle:** the modes manager watches the workspace glob and reloads
the list (`_workspaceWatchers`, `updateFromFiles` → `triggerModeChange`),
but `onModesUpdate` has NO subscriber anywhere in the bundle — the three
occurrences of `_modeChangeEvents` are its declaration, its push and its
forEach — and a session's mode is the object resolved at start (`n.mode`),
re-resolved only by `setSessionMode` when the user switches. The running
seat keeps its persona; what changes is the picker in that directory, which
names the last persona launched there, and a by-hand switch back to a slug
no longer in the file fails with bob's own "Mode with id … not found".
ASSUMED 4 below carries the half that needs a turn. The fix priced and
rejected: one plugin dir per persona (`plugins/posse-<slug>/`, the glob's
own namespace, one writer per file by construction). It costs Dir becoming
a template, the trust exemption narrowing to this launch's dir, an exclude
pattern and the pins — small — and it BUYS the operator's picker in that
checkout every persona ever launched there, which is the global file's
rejected cost confined to one directory, for no seat-side gain. ADR 0022
does not bite: this file is never committed, so there is no sweep to
mis-attribute. Last-writer-wins is recorded, not fixed; `rm` the file or
launch the persona again is the remedy, and `.git/info/exclude` keeps it
out of every diff either way.

**Not decided here:** whether bob should implement `hidden` (upstream, the
`create-mode` skill documents the field); whether `~/.agents/skills` under
the home wants a guard — ruled the same day, the other way: no guard
(ranger-base-q114b, Consequences below); the two billed turns
(ranger-base-x5xt6).

## Amendment 2026-10-03 — D1.1 grants nine groups, not ten (ranger-base-mkcsy)

**`mode` is the `switch_mode` tool, and the mode IS the PID, so that one
group was a door out of this channel.** Claim 9 has the measurement; the
decision is here, because D1.1 chose its group list by matching bob's
built-in agent mode and that list was copied before anybody asked what each
name grants.

**D1.1 now reads: the nine, `mode` dropped.** `groups` is a free string
array in bob's schema, so nine is as valid a mode as ten; the
`command`→`execute` aliasing is untouched; nothing else about the channel
moves.

**The cost, stated: a bob persona cannot switch modes.** No PID asks for
that, and the capability is the one thing this record exists to prevent —
D1 is a mechanism for making the PID *arrive*, and D2 is a reading that
refuses to type at a session where it did not. A grant that lets the model
undo both after the gate has passed is not a feature of the channel with a
price; it is the channel being optional at the model's discretion.

**Why a drop and not a reading.** D2's shape — do not trust the flag, read
the footer — does not generalize here. D2 reads once, before the first
keystroke, because that is when the launch can still refuse to spend; a
switch can happen at any later turn, so the equivalent reading would be a
poll of every bob pane's footer for the life of the session, and its
remedy at the moment it fired would be a session already running on the
wrong role. `hidden: true` is not a reading either: claim 6 measured that
it hides nothing, and claim 9 adds that `validate` searches the unfiltered
list, so a hidden mode is a legal `switch_mode` target. Removing the tool
is the only one of the three that is a gate.

**What this does NOT claim.** Nobody measured a model electing to switch
(claim 9's NOT MEASURED), and bob's own prompt discourages it. The drop is
not priced against an observed loss — it is priced against ADR 0013 §1:
the captured artifact says the door is open, and a disposition in a system
prompt is not a gate. Closing it costs a capability no PID asked for.

**Pinned**, because this list drifts back by being helpful: the group's
absence is asserted of the declaration *and* of the rendered `groups:`
line, in `TestQAPersonaModeGrantsNoToolForLeavingTheMode`, which names
`switch_mode` in its failure so the next reader who re-copies the built-in's
ten is argued with rather than merely reddened. The other nine are
deliberately not pinned: which tools a persona gets is a product choice
that may move, while this one is a property of the channel.

## Consequences

- Built: one placeholder, one materializer seam with one member, one
  file written where the skills tree already is, one exclude line, one
  trust-list entry and its exemption, one footer read before the first
  keystroke, one PID-channel reading behind three surfaces. No new config
  key, no new actor, no runtime name in code, no write under the home.
- **"No write under the home" is a GUARD and not a property of the paths**
  (ranger-base-se81d, from claim 7). posse's workspace path and bob's home
  glob are the same relative path, so the channel turns global by itself
  whenever the session directory is the operator's home — which is what a
  `posse new` with no `--dir` names on any install that has not pointed
  `default_dir` elsewhere, since `$HOME` is the fallback. The channel
  therefore declares the CLI's own config root (`GlobalRoot`, `.bob`) beside
  the Dir it writes, and the materializer **refuses** — before the
  never-clobber read, which under that root answers the wrong question
  twice — when the file it would write lies under `<home>/<GlobalRoot>`.
  Refuse and not warn: proceeding IS the harm, it lands on the operator's
  files rather than on the seat, and `posse new` is interactive, so a
  warning would not stop the one path that reaches it. Not sweep: deleting
  under the operator's CLI home a file posse cannot prove it wrote is a
  larger liberty than the write being undone. The cost, stated: on a box
  whose `default_dir` is the home, an interactive `posse new --runtime bob`
  refuses until the operator names a `--dir`. That is this record's own
  position — the channel is the SESSION tree — and the message names both
  remedies. The class is older than this channel and has landed once
  already down the same `dir` default: `~/.agents/skills/` exists (ADR
  0007's tree, created 2026-09-05), which `GlobalRoot` does not cover,
  because `.agents/skills` in the home is read by a cwd-at-home session
  while `.bob/plugins/posse` there is read by every bob session anywhere.
  That sibling is ranger-base-q114b, **and it was decided the other way,
  2026-10-03: no guard.** The skills tree at the home means what the skills
  tree in any other directory means — ADR 0007 §4's contract is already
  additive, dir-owned and seat-outliving, and no CLI reads `~/.agents` as a
  global root (each of the three has its own, and it is a different path:
  `~/.codex/skills`, `~/.grok/skills`, `~/.bob/skills`) — so the home is a
  directory under that contract rather than an escalation of it. The
  asymmetry is pinned from both sides in
  `internal/posse/skillshomedir_qa_test.go`, including the ORDER: a bob
  launch at the home renders the skills tree and *then* refuses here, so the
  already-created `~/.agents/skills` is a correct tree for a cwd-at-home
  session and not residue of a refused write. Nothing to sweep, for a
  stronger reason than this bead's — not "posse cannot prove it wrote it"
  but "the write was right". The root cause both share — `posse new`
  having no worktree option, so an interactive session never gets a tree of
  its own — was raised as ranger-base-er6mt's option 2 and ruled not a
  consequence of this record (Ruling, above). It is still where a refusal of
  a home-as-session-dir would belong if one is ever wanted: one rule about
  the dir, not a guard in one of the four writes that land inside it (the
  pre-push and commit-guard hooks, the skills tree, this file), which stops
  one and lets the rest run.
- Not built, on purpose: the typed-PID route (below); the realizer
  (`groups` with `fileRegex` restrictions is where `{deny}` could render
  one day — ADR 0060 already said so; the trigger is unchanged, a measured
  per-verb surface); `customInstructions` and `rules-posse-<persona>/`,
  which Bob also reads beside the modes file and posse has no second
  text to put in.
- Blast radius of D3, MEASURED 2026-10-03: 19 CLI-shaped `command:`
  fixtures in `internal/posse` tests carry no `{file}` (10 of them in the
  three cage test files); those that reach a bead-carrying launch gain a
  `{file}`, which is what a fixture standing in for a persona launch
  should have carried. The bead's "~92" counted every `command:` string,
  prose included.
- `rules_precedence` on Bob comes back from MOOT to UNMEASURED: the PID
  arrives again, and whether `roleDefinition` outranks the workspace's
  AGENTS.md on a collision is a billed turn on the instance side
  (ranger-base-6rcv's shape).
- Still blocked by something this record does not touch: the typed work
  prompt needs a labelled pane, and on darwin nothing labels one until the
  herdr-bob watcher can start (ranger-base-mz8ud, upstream). D1 delivers
  the PID without a keystroke, so it is not behind that; the work prompt
  is.

## Alternatives rejected

- **PID typed as the first user message, work prompt second** (ADR 0060's
  other candidate). A user turn, not a system channel: precedence against
  AGENTS.md/CLAUDE.md is the model's mood; one billed turn per launch
  whose reply is noise; a sequencing rule nobody has (type → wait Seen()
  idle → type), which coins a runtime dimension the Prompt key has no
  vocabulary for; and it needs the pane labelled *first*, so on this box
  it is dead until mz8ud lands. D1 needs none of that and is the channel
  Bob documents for personas (`create-mode` skill: "a mode or persona").
- **The operator's global `~/.bob/settings/custom_modes.yaml`** (0060's
  rejection, reasons intact): posse rewriting a file the operator edits
  (ADR 0022), every persona in their `/mode` menu — in every workspace,
  re-priced in the Ruling against the session tree's one persona in one
  directory, since `hidden` hides neither — a stale mode outliving
  the session.
- **`~/.bob/plugins/posse-<persona>/custom_modes.yaml`** — the clever one:
  Bob reads the plugins glob under the home too, so one file per persona
  outside the repo, no git, no trust collision. Priced: a write under the
  operator's CLI home on the host tier (the seatbelt grants `~/.bob` to
  the *runtime*, not to the launcher); invisible inside a cage whose home
  is its own, so a mount or a seed joins the build; stale across persona
  deletion, so a sweep joins it; visible to the operator's own Bob
  sessions in every workspace. The session tree already exists, is
  already excluded, already rides into the cage, and dies with the seat.
- **`.bob/custom_modes.yaml` at the top of the session tree.** It is on
  `bobProjectConfig` by name, so posse's write would trip posse's own
  trust refusal; and it is the file a repo would ship, so posse's write
  could shadow the repo's. The plugins subdirectory is a namespace Bob
  gives away for exactly this.
- **Trust the slug** (skip D2). The fallback is one grey line and a
  persona-less session — measured, not feared. 5jjtn is what trusting a
  parsed flag cost.
- **Refuse on any Blocking gap**, or **refuse interactive too** — rejected
  in ADR 0013 §1 already (rows 2 and 5); here they would wall off the
  probe.
- **Do nothing on the refusal** (a non-blocking line only). Honest, and
  it stops nothing: the P1 this came from was a spent seat with every
  line printed naming a slow start.

## Claims

**MEASURED 2026-10-03, this box (darwin 25.4.0, bob 2.0.5 / node 25.2.1,
herdr 0.9.1), scratch workspace, no turn, no spend, both panes closed and
`checkorphans` clean** — recipe in `docs/notes.d/ranger-base-f1ytb.md`:
1. A workspace `.bob/plugins/posse/custom_modes.yaml` mode is loaded and
   selected by `--mode <slug>`; the footer names it.
2. `--mode <unknown>` prints `Unknown mode "…". Falling back to "agent"
   mode.` and opens in Agent Mode.
3. By reading the 2.0.5 bundle: the workspace glob is
   `{custom_modes.yaml,plugins/*/custom_modes.yaml}` under
   `<workspace>/.bob`; the global one is `{settings/…,plugins/*/…}` under
   the home; `roleDefinition` is the `role_definition` section of the
   prompt; slug alphabet `^[a-zA-Z0-9-]+$`; `groups` defaults to `[]`;
   `hidden` is a schema field; `rules-<slug>/` beside the modes file is
   read too.
4. `launchRow` prints "PID delivered by the template" with no reading;
   `PIDVoided` is the only PID-delivery check on any launch path.
5. 19 CLI-shaped test `command:` templates carry no `{file}`.

**MEASURED FALSE 2026-10-03** (ranger-base-4mrmc, discharging ASSUMED 2
below; evidence in `docs/notes.d/ranger-base-4mrmc.md`, fixes in
ranger-base-se81d):

6. **`hidden: true` hides NOTHING on bob 2.0.5.** The mode is the fourth
   Shift+Tab stop (Agent → Plan → Ask → `<persona>`), `/mode` lists it
   4/4, and the picker's `Tab to view mode` prints the whole PID —
   frontmatter and `deny:` list included. By reading the bundle: the YAML
   `hidden` is parsed and carried onto the mode object and nothing reads
   it; both consumers take `runtime.getModes({workspace})`, which filters
   only tool groups and duplicate ids. (`hiddenFromUser` is a different
   field, filtered in `isModeEnabled`, and belongs to provider/builtin
   modes — which a workspace modes file never becomes.) 5jjtn's
   `.allowUnknownOption()` class a third time, in the schema this time.
   There is no "loaded but hidden" state to reach for either: the only
   switch that takes a workspace mode out of the list is workspace trust,
   and that same switch stops the mode being SELECTED, which is the
   channel. posse keeps emitting the field — it is the honest declaration
   of intent, it costs nothing on a CLI that ignores it, and it is the
   behaviour for free the day bob implements it — but nothing may read it
   as a confidentiality boundary. **D1's premise that this field keeps the
   PID out of the operator's own picker is gone; D1's choice was ruled to
   STAND without it (Ruling, ranger-base-er6mt).**
7. **The session dir is not always a session tree, and `.bob/plugins/posse`
   under the home is bob's GLOBAL modes glob.** `posse new` has no worktree
   option, so `o.Worktree` is false and its session dir is `--dir`, else
   `default_dir`, else **`$HOME`** — the last being the fallback `CfgGet` is
   handed, so it needs no misconfiguration to reach. One `posse new
   --runtime bob -a <persona>` with no `--dir` would therefore write the PID
   to `~/.bob/plugins/posse/custom_modes.yaml`, which is the home glob
   (`{settings/…,plugins/*/…}`, notes `ranger-base-f1ytb.md` §1) and global
   scope: a mode in every bob session on that box, in every workspace,
   returned even for an UNTRUSTED one, and outliving the seat, since nothing
   in the tree removes the file. That is this record's own rejected
   alternative (`~/.bob/plugins/posse-<persona>/…`) reached by accident.
   Caught before the fact — ranger-base-4mrmc looked and found nothing had
   landed — and guarded since, see Consequences.

**MEASURED 2026-10-03** (ranger-base-er6mt, by reading the 2.0.5 bundle at
`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js`, 15.5 MB, no turn):

8. **A live bob session keeps the mode it opened with when the modes file
   changes under it.** The modes manager watches the workspace glob and
   reloads its list, but nothing subscribes to the change (`onModesUpdate`
   has no caller; `_modeChangeEvents` occurs three times: declared, pushed,
   iterated), and a session's mode is the object `resolveMode` returned at
   start, re-resolved only by `setSessionMode` on a by-hand switch. The
   Ruling's shared-checkout reading rests on this.

**MEASURED 2026-10-03** (ranger-base-mkcsy, read off ranger-base-4mrmc's
one authorized billed turn — its stored prompt in `~/.bob/db/bob.db`, task
`82388f73d09c74e1344b8ced0cd57d5f` — no further spend):

9. **The `mode` tool group is `switch_mode`, so D1's own group list let the
   model leave the PID mid-session.** `switch_mode`'s call is `changeMode`
   and the prompt's `role_definition` section renders from the CURRENT
   mode's `roleDefinition`, so one tool call replaces the persona with
   Agent Mode's "You are Bob, a highly skilled software engineer…" for the
   rest of the session — no refusal, no pane signal, and a footer that then
   reads `Agent Mode`, the exact string D2's gate treats as a PID that
   never arrived. In that turn's own artifact: `switch_mode` was in
   `availableTools`; the `available_modes` block (byte 15410) named agent,
   plan and ask with their tool sets, under "You can switch between
   different modes using the switch_mode tool"; `hidden: true` removed only
   posse's OWN row from that block (claim 6's field, its one consumer) and
   removed neither the built-ins nor the tool; `validate` searches the
   UNFILTERED list (`this.availableModes.find(o => o.id === n)`), so even a
   hidden mode is a legal target, and a wrong guess enumerates every id in
   its error text; and the current mode id reaches the model anyway as
   `environment_details/current_mode`. NOT MEASURED: whether a model elects
   to switch. bob's prompt discourages it ("Stay in your current mode unless
   there is an explicit, compelling reason to switch") — a disposition, not
   a gate, and ADR 0013 §1 promotes the reader on the captured artifact,
   which says the door is open and unlatched. **Acted on: see the
   Amendment above, which drops the group.**

**ASSUMED** (each a line for the instance side, ranger-base-6wqe's lane):
1. The `roleDefinition` reaches the model as the persona — the footer
   says the mode is selected; that the model answers as the PID is one
   billed turn nobody has spent.
2. ~~`hidden: true` keeps the mode out of Shift+Tab and the `/mode`
   picker~~ — MEASURED FALSE, claim 6 above. Kept numbered so the three
   ASSUMED lines this record shipped with stay countable.
3. Precedence of `roleDefinition` over the workspace's AGENTS.md on a
   collision (rules_precedence, UNMEASURED).
4. The per-turn system prompt is built from the session's held mode object
   and not re-read from the modes file (claim 8 shows no re-resolution;
   that the prompt builder reads `n.mode` rather than the file is one turn
   in a shared checkout after a second persona's launch — ranger-base-4mrmc's
   lane, if the operator wants it).

## Verification (the closer's observables)

1. `posse new --runtime bob -a <persona>` renders `--mode posse-<persona>`
   on the line and writes `<dir>/.bob/plugins/posse/custom_modes.yaml`
   with the PID body as `roleDefinition` and NINE groups — bob's agent mode
   less `mode`, asked of the declaration and of the rendered `groups:` line
   both, so a reader who re-copies the built-in's ten reds (the pin is
   `TestQAPersonaModeGrantsNoToolForLeavingTheMode`; ranger-base-mkcsy);
   `git status` in the session dir does not list it.
2. A dispatch onto a runtime whose template carries neither `{file}` nor
   `{mode}` creates nothing, claims nothing, and names both placeholders
   and the native-rulebook consequence; `posse new` on the same profile
   warns and proceeds; `runtime check` prints a blocking `pid` gap and
   `agent check` on a PID whose `command:` lacks both prints the finding —
   all three off one fake.
3. A Bob pane whose footer reads `Agent Mode` after `--mode posse-x` gets
   no work prompt, and the line quotes Bob's fallback sentence.
4. A session dir whose repo tracks `.bob/plugins/posse/custom_modes.yaml`
   refuses by name; a repo shipping `.bob/plugins/other/custom_modes.yaml`
   trips the project-config trust refusal; posse's own file does not.
5. A session dir that is `$HOME`, or anything under `$HOME/.bob`, refuses
   by name — naming the path it would have written, that the directory is
   the CLI's GLOBAL modes root and not a workspace, and both remedies
   (`--dir`, `default_dir`) — and creates neither the file nor `.bob/` on
   the way. **Including when a symlink inside `$HOME/.bob` would carry the
   file out of the root**: the containment asks both where the write lands
   and how the path is spelled, because the spelling is what bob's glob
   matches and a symlink at `~/.bob/plugins/posse` makes the two differ
   (ranger-base-ie68e F1; before it the guard returned nil there and the PID
   was written, readable at the in-root path all the same — so the sentence
   this row used to carry, "anything under `$HOME/.bob` refuses", was
   MEASURED FALSE from the day it landed). **And the mirror of it**: a
   session dir OUTSIDE the root whose own `.bob/plugins` is a symlink to
   `~/.bob/plugins` is refused too — there the spelling is outside the root
   and the inode inside, so it is the resolved question and not the spelled
   one that answers, and the bytes would otherwise land on bob's global
   modes glob exactly (ranger-base-4w7rk, verifying ranger-base-ie68e: the
   two questions were both asked from the day F1 landed, but only one of
   them had an arm, so the other could be deleted with the package green).
   `$HOME/.bobbish` and an ordinary dir under the
   home do not refuse, and a runtime with no persona-mode channel does not
   refuse at `$HOME` either, because it writes nothing there to refuse.
   (ranger-base-se81d, ranger-base-ie68e, ranger-base-4w7rk; the pin is
   `TestQAPersonaModeRefusesAWriteIntoTheCLIsGlobalConfigRoot`, and it reds
   on all seven of: the guard call deleted, `underDir` downgraded to a
   string prefix, `Dir` moved out from under `GlobalRoot`, the spelling arm
   deleted, the root left unresolved for the spelling test, the spelling
   test resolving the leaf too, and the resolved arm deleted.)
