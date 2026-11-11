# ADR 0080 — posse leaves the laptop: one always-on Linux host, one OS account, one Rex server, a home per crew

*Status: accepted 2026-11-06 (ranger-base-3v8gh, operator direction in an
architecture session the same day) · owner: architect · scope: the
operator's PERSONAL instances only; the work instance is out of scope and
unchanged · applies ADR 0075 §2 (the multiplexer slot) and ADR 0076 (Rex
as its second occupant) to a host that is not macOS · reverses ADR 0076 D4's
price judgement on operator direction (see D3) · D3 amended for the D1 host
and D21 retired, 2026-11-09 (ranger-base-r2fz9, operator ruling
ranger-base-0ka8a (a)): see the last amendment*

## Context

posse runs on the operator's laptop. The laptop sleeps, it is shared with
the operator's own work, and on 2026-08-30 six crew sessions plus a container
VM had it swapping (ranger-base-6mz7, MEASURED there). The goal is to take
posse off the laptop and onto infrastructure that is always on, layered as:

    always-on infrastructure  >  posse crews (several)  >  projects

The host is a headless x86_64 mini PC the operator owns: 8 cores / 16
threads, 24 GB RAM (likely soldered, so a ceiling: ASSUMED), one 1 TB NVMe,
wired 2.5 GbE. It runs an Arch-based distribution today and stays on it.

What posse needs from a host, read from the tree on 2026-11-06:

- **Substrates:** a multiplexer server (herdr ≥ 0.8, or Rex per ADR 0076),
  bd at 0.50.3 exactly, git, and the runtimes' CLIs with their logins.
- **An always-on loop.** `posse dispatch --watch` arms backups and the
  coordinator pulse. Today it is autostarted ONLY by herdr's `[[startup]]`
  hook (`plugin/autostart.sh`, examples/config.yaml "scheduled dispatch").
  A Rex-only host has no autostart at all.
- **A cage.** `cage: seatbelt` is darwin-only (`SeatbeltAvailable`,
  seatbelt.go). Off darwin, the only enforced tier posse has is the container
  tier. Everything else is cooperative (ADR 0002 §4, formerly ADR 0025): it
  holds the ordinary path and nothing that empties its environment.
- **Credentials.** The Claude meter reads the macOS keychain on darwin and
  a credentials file elsewhere (`meterStoreAt`, credential.go). The file
  path is built but "not yet confirmed against a live login" (ADR 0019 V1).
- **Rex's binary.** The default `rex_bin` is the macOS app bundle path
  (`rexDefaultBinRel`, rex.go). The Linux package installs `/usr/bin/rex`.
- **Process table.** Off darwin, `checkorphans` reads `ps`
  (proctable_other.go). Whether that works on Linux is ASSUMED there, never
  measured.

What the multiplexers offer on Linux (vendor docs read 2026-11-06, not yet
measured here):

- **Rex:** a Linux server is available to testers on x86-64 and arm64, with
  an Arch package. The server and the CLI are one binary. Its modes are
  personal (one user, a systemd user service), shared, and a per-account
  login server. It listens on a Unix socket by default. Remote use is
  either its built-in Tailscale listener (`rex server start --tailscale`) or
  an SSH-forwarded socket. Rex's own docs say it "trusts a caller based only
  on how they connect", and recommend Tailscale until its auth system ships.
  The macOS app attaches with "Add Remote Host…".
- **herdr:** Linux x86_64 and arm64. `herdr --remote <ssh target>` draws
  locally while the remote server owns the panes. Authentication is plain
  OpenSSH.

## Decision

**D1. The host is the operator's always-on Linux box, run headless.** It
boots to `multi-user.target`, with no desktop session. Every long-lived
process is a systemd *user* service under one account, with lingering
enabled so the services outlive a logout. The operator updates the
distribution by hand, never on a timer. A rolling release moves git, the
kernel and Rex in one step, and posse's git-scope pins (ADR 0079,
gitsystemscope.go) are measured against a git version. If an update breaks
those pins, hold git back with the package manager's ignore list; never
patch the pins to suit.

**D2. One OS account, one Rex server, one posse home per crew.**
- The account is a dedicated `posse` user. That name passes the commit
  wall only through ADR 0024 D2 check 3's 2026-11-09 amendment
  (ranger-base-87jw1): a username equal to a shipped name is not derived
  as an identity literal. Until that lands, an account with this name
  cannot commit into posse.
- That user runs one Rex server in personal mode, plus herdr's server
  beside it (D3).
- Each crew is its own `RHQ_HOME` under that account, with its own
  config, beads stores and repos.
- The laptop is a client only: the Rex macOS app, `herdr --remote`, or
  SSH. Runtimes report OSC 7501 to a server on the SAME host, so no status
  sequence has to survive an SSH hop.
- Access is over the operator's tailnet: Rex's built-in Tailscale listener
  and SSH. Nothing listens on the LAN or the internet unauthenticated.
  Rex's `--allow-anonymous` HTTP mode is not used.

Crews are separated by posse's own walls, not by Unix accounts. That is the
cost of "one server", and the exit is D7.

**D3. Rex is first-class, and herdr and Rex are interchangeable for every
runtime.** This is operator direction (2026-11-06), and it reverses ADR 0076
D4. D4 refused codex on Rex because of what admitting it costs posse; that
cost is now accepted. The mechanism is designed in its own bead (cut below)
as an amendment to ADR 0076. Until it lands, the D2 host runs herdr beside
Rex, and a crew whose runtime Rex refuses (codex, grok, bob) uses herdr.
`posse list` already says which multiplexer a session is on.
*bob is the exception (ADR 0076 Amendment 4, D14, ranger-base-778y0): `bob chat` takes no argv prompt and writes no record before a typed one, so bob is herdr-only on Rex, and a bob crew on the D2 host uses herdr beside it.*
*On the D1 host, claude only (operator ruling, ranger-base-0ka8a (a),
2026-11-09).* D3 stays posse's design: the codex line on Rex (ranger-base-8gh58)
is built and stays built. What the host RUNS is narrower, for D6's reason:
codex, grok and bob crews stay on the laptop. See the last amendment.
ADR 0076 D3 (grok: no record in 131 s, MEASURED, cause upstream) is not
reversed by direction. A mechanism can route around a runtime that never
reports, but it cannot turn that runtime's silence into evidence.

**D4. Cage: cooperative only, to start.** No container tier on this host.
- On a shared account, docker-group membership is root-equivalent, and it
  would undo the separation D7 is for.
- The memory spent on containers is the memory this move is buying back.

A PID whose rules need an enforced cage keeps refusing at launch, as it
does today on any host without one. That refusal is honest, and the first
crews here use PIDs that do not need a cage. Landlock (kernel-enforced,
unprivileged, path-scoped, no daemon) is the candidate enforced tier for
Linux. It is ASSUMED a fit, and spiked before anything is built on it.
*(Spiked: Landlock is rejected and a mount namespace is the candidate. See
the amendment "The Linux cage is a mount namespace, not Landlock".)*

**D5. On this host the watch loop is a systemd user unit, not a
multiplexer hook.** It runs one `posse dispatch --watch` per crew home,
ordered `After=`/`Requires=` the multiplexer server's unit, so it cannot
start in a world with no socket (the herdr hook's existing property, kept).
The unit is rendered by posse, so its argv tracks the binary. The
autostart-key arm grammar (`autostart_interval:` presence) is unchanged.
The loop is visible through `posse status`, its log, and `systemctl --user
status`. The herdr hook stays the autostart on hosts that run herdr only.

**D6. Models: stock CLIs, each signed in directly, no proxy.**
- **Claude:** one subscription, signed in to the unmodified Claude Code
  binary once per account on the host. Every crew draws on that one usage
  limit. posse's meter and plan guard (ADR 0012) are the budget across
  crews.
- **codex and grok:** each uses its own CLI's own login, as needed.
- Anthropic's Claude Code legal page permits "an end user [signing] in to
  the unmodified Claude Code binary with their own Claude subscription,
  including where a platform hosts Claude Code". The same page forbids
  collecting, storing or intermediating Claude.ai credentials, and the Usage
  Policy (effective 2026-11-12) forbids "services that route requests
  through consumer subscriptions".
- Whether one login may be used on more than one of the operator's own
  machines is not addressed in the terms. ASSUMED permitted.
- OpenAI's and xAI's terms for a plan login on a server are unread. The
  operator reads them before a codex or grok crew runs unattended. Until
  then the D1 host is claude-only (last amendment).

**D7. Exits, named now.** Each of these is a later bead, not this one.
- **One OS account per crew** via Rex's per-account login server, when one
  crew's walls stop being enough.
- **An enforced Linux tier** (D4): a bubblewrap mount namespace, not
  Landlock (amendment below, ranger-base-20r29).
- **A VM per crew** (overalls or similar) once a platform installs on
  owned hardware.

The host holds no state hostage: every crew's state is files under its
`RHQ_HOME` and its repos, and `posse backup` covers the stores as it does
today.

## Consequences

- The laptop stops being a dependency: crews run while it sleeps or
  travels. The cockpit is a herdr plugin pane, so on Rex the operator reads
  the fleet through `posse status`/`posse list` and Rex's own UI.
- Six-plus seats fit in 24 GB without a VM or desktop holding memory
  (ASSUMED, read in the shakedown). The core count equals the laptop's, so
  `make test` costs what it costs today.
- `/bin/bash` exists on this host, so the 38 `#!/bin/bash` literals in
  posse's Go renders (a grep, not an audit) are a portability debt, not a
  blocker. Filed P3.
  **Read, 2026-11-08** (ranger-base-5syu4, docs/notes.d/ranger-base-5syu4.md):
  the grep had the count right and the place wrong. Not one of the 38 was a
  render — posse's renders were `#!/bin/sh` already, every one of them — and
  all 38 were test-fixture shims and fixture text under internal/treepins.
  So the debt was never in the renders; it was in the SUITE, which eighteen
  of them made unrunnable on a host without `/bin/bash`. They are
  `#!/bin/sh` now bar one bashism that takes `#!/usr/bin/env bash`, and
  `TestQANoGoSourceWritesABinBashShebang` (`make corpus-check`) keeps the
  literal out. Nothing here needed the host to have `/bin/bash` after all,
  and nothing was held back.
- Two never-measured Linux paths get measured by the shakedown, with no
  code first: the meter's credentials-file store (ADR 0019 V1) and
  `checkorphans` over `ps`.
- Arch's current git reads config-based hooks (`hook.*`), which the
  laptop's git 2.50.1 does not. The ranger-base-zwkgb arms that skip below
  2.54 can run there for the first time. This is a bonus, not a gate.
  **Read, 2026-11-07** (ranger-base-jep08, docs/notes.d/ranger-base-jep08.md):
  the host arrived at git 2.56.0, and D1's hold-back clause was exercised
  before any crew ran there. All 51 hook-skip, parity and git-scope arms pass
  on it — two for the first time anywhere — so nothing is held back and no
  pin was patched. The arms were cross-compiled (`GOOS=linux go test -c`) and
  run from a copy of the tree, which needs no toolchain on the host and is
  not a substitute for step 6 of the bring-up. What 2.56 does add is a hide
  direction that is inert below 2.54: `hook.<name>.enabled` and
  `hook.<event>.enabled` skip a managed scanner at rc 0. The hook-skip arm
  refuses all three spellings already, by key prefix; the gap was that
  nothing had measured ADR 0052's rule against the `hook.*` shape at all.
  `docs/adr/0052-config-based-hooks.probe.sh` is the instrument, and the
  reading to re-take when this host's git moves again.
- Several crew homes under one account share one multiplexer server. The
  amendment below (ranger-base-oz8nc) answers whether session names collide
  and whether D5 dissolves herdr's "only the default herdr server may arm
  the fleet loop" (D8-D12).

## Alternatives rejected

- **overalls.dev as the infrastructure layer** (the one wanted). It gives a
  persistent Firecracker microVM per crew, clone and snapshot, which fits
  "infrastructure > crews" exactly. Deferred, not dismissed. As of
  2026-11-06:
  - it is a research preview launching 2026-11-12;
  - its installer knows Hetzner, EC2 and Hetzner Cloud only;
  - it needs ≥ 2 public IPs on one on-link subnet (single routed extras
    fail its uplink check), public DNS on port 53, an SMTP relay and a
    domain;
  - its guests reach Claude through its proxy on API keys, the opposite of
    D6.

  Revisit when it installs on owned hardware.
- **underclass for subscription pooling.** It intermediates Claude
  subscription credentials. Its own README records that this "violates the
  Consumer Terms of Service" and that "Anthropic … can invalidate a
  credential without notice". A fleet built on a revocable credential fails
  at the moment it is depended on.
- **NixOS on the host.** It is declarative and was the operator's learning
  goal. Rejected for now because it stacks a second unknown on the Linux
  port: Rex, Claude Code, codex and grok ship dynamic binaries that need
  nix-ld, and NixOS has no `/bin/bash`. It remains a fine destination once
  posse-on-Linux is measured.
- **Ubuntu Server reinstall.** It is reasonable and boring. Rejected
  because the current Arch-based install already runs Rex's Arch package and
  `/bin/bash`, so a reinstall buys nothing D1 needs.
- **One OS account per crew from day one.** Rex then means a login server
  and PAM, and five more things on the first day. D7 keeps it as the exit.
- **Containers as the Linux cage now.** See D4.
- **Keep posse on the laptop with sleep disabled.** It keeps the memory
  ceiling and the shared-machine problem that started this.

## Measured versus assumed

MEASURED, this tree, 2026-11-06: the darwin-only seatbelt and keychain
paths, the macOS `rex_bin` default, the herdr-only autostart, the
`#!/bin/bash` count (a grep), and ADR 0076's refusal set.

REPORTED, from vendor docs read the same day: Rex's and herdr's Linux and
remote modes, overalls' constraints, and the Anthropic terms quoted in D6.

ASSUMED, until the shakedown reads them: seat capacity in 24 GB, soldered
RAM, `ps`-based `checkorphans`, the Linux credentials-file meter, and that
one Claude login across the operator's own machines is within terms.

## Beads

All are `discovered-from` ranger-base-3v8gh.

| bead | who | what |
|---|---|---|
| ranger-base-yyq64 | operator | host bring-up (D1, D2, D6) |
| ranger-base-izgm6 | dinesh | `rex_bin` default off darwin |
| ranger-base-136b5 | dinesh | watch loop as a systemd user unit (D5) |
| ranger-base-dok0a | architect | Rex admits every runtime: ADR 0076 amendment (D3) |
| ranger-base-oz8nc | architect | spike: several homes on one account and one server (D2) |
| ranger-base-20r29 | architect | spike: Landlock as the Linux cage (D4, D7) |
| ranger-base-5syu4 | dinesh | `#!/bin/bash` portability (P3) |
| ranger-base-offid | operator | shakedown; blocked on yyq64, izgm6, 136b5 |

## Amendment: several crew homes on one account and one server (ranger-base-oz8nc, 2026-11-06)

**Context.** D2 puts N `RHQ_HOME`s under one OS account, sharing one Rex
server and one herdr server. The Consequences above left three questions:
(a) do session names collide across homes, and what does each occupant's
`List()` (ADR 0076 §5 correction) show of the other homes' sessions;
(b) does D5 dissolve "only the default herdr server may arm the fleet loop";
(c) what else is keyed per user rather than per home.

The host is not up yet (ranger-base-yyq64), so the two-home run was taken
on the laptop against a **scratch** Rex server (`rex server run --listen
<private socket> --restore=false --max-sessions 3`), the operator's own
server untouched. Three scratch homes: `crewa` with `instance: crewa`,
`crewb` and `crewc` untagged, all `multiplexer: rex`, `rex_bin:` a wrapper
adding `-S <socket>`, seats running `sleep 600`. No model ran.
MEASURED 2026-11-06, laptop (darwin/arm64), rex 0.0.1793377904-ge6592f,
posse built from this tree:

| probe | result |
|---|---|
| `rex new 'crewa/smoke'` | label with `/` round-trips through `ls` and `session list --json` |
| `rex new dup` twice | **both accepted**: Rex allows duplicate labels (herdr refuses the second, rangerhq-ouf9) |
| a 4th `rex new` at `--max-sessions 3` | refused, `409 conflict` |
| block's program and shell ended (`windows: 0`) | **the husk still counts toward the cap**; `session_count` includes it |
| A `posse new smoke`, B `posse new smoke` | labels `crewa/smoke` and `smoke`; both created |
| A `posse list` | its own `smoke 👤` **and** B's as a second `smoke (rex)` row |
| B `posse list` | A's row reads `crewa/smoke (rex)` |
| B `posse kill crewa/smoke` | refused: "NOT killed: … is a foreign workspace" |
| B `posse new crewa/smoke` | refused: `/` is not a legal session name |
| C (untagged) `posse new smoke` | refused "already exists": posse's own foreign-label check stands in for the server refusal Rex lacks |
| runtime exits, shell stays | seat stays listed (block open) — no husk |
| shell exits too | seat drops out of A's `List()`; after `PruneGrace` A unlinks the meta and **the Rex session stays on the server** (see D10) |

Read from the tree, same day:
- `RexMux.List()` and `HerdrMux.List()` return every seat on the server,
  every home's. Ownership is the meta file, never the listing:
  `listSessions` claims rows by recorded id and lists the rest as
  `Foreign`. Kill, dispatch, autoreap, govern and permission reads all
  skip or refuse foreign rows. This is rangerhq-ouf9's design, and it is
  occupant-agnostic, so it holds on Rex as measured above.
- `plugin/autostart.sh` resolves ONE home (`RHQ_HOME`, else
  `~/.config/posse`) per herdr server start. "Only the default herdr
  server" is a per-USER rule, because herdr's plugin registry is per user.
- `dispatch --watch` that cannot take `state/dispatch-watch.lock` exits
  non-zero (watch.go). The lock keeps one loop per home whoever starts it.
- The plan-usage snapshot that ended the 2026-08-22 429 storm
  (plancache.go, rangerhq-tdy8) is per HOME (`$StateDir/plan-usage.json`).
  The meter it shares is per CREDENTIAL, and D6 puts every home on one
  login.
- `reclaim` (herdrback.go) unlinks a proven-dead meta and closes nothing.

**Answers.**
- **(a)** Names collide only when a home is untagged. With `instance:` set
  on every home, labels are disjoint by construction, and `/` cannot
  appear in a typed name. Untagged, every verb is still safe (refusals
  above), but a listing shows two rows with one name. Each occupant's
  `List()` shows every home's seats. That is what makes the foreign
  refusals possible, so it stays.
- **(b)** Yes for the rule; no for the hook. With D5's unit owning the
  loop, no home needs the hook. But the hook still runs whenever the
  posse user's herdr server starts with the plugin linked. If it finds an
  armed home, it starts a herdr-hosted loop first (herdr is up before the
  unit that is `After=` it), and the unit's own loop then loses the lock
  and fails. That is one loop, not two, but it lives where D5 said it
  must not.
- **(c)** The table in D12.

**D8. Every home on the host sets `instance:`, and no crew home lives at
`~/.config/posse`.** Config and procedure, not code. The tag removes the
ambiguous rows. Having no default home means a hand-typed `posse` must
name its crew with `RHQ_HOME`, and the herdr hook has nothing to arm. A
command that runs against the wrong home is rangerhq-snd's and
ranger-base-g7lt's shape, and a default home on a multi-home account
invites it.

**D9. Loop ownership is a config key: `autostart_via: herdr | systemd`,
default `herdr`.** The hook stands down by name on `systemd`. The unit
render (ranger-base-136b5) refuses unless the home says `systemd`.
`posse status` G7 reads the key the same way. One key with two readers,
the house pattern for this file (plugin/autostart.sh `cfg()`). D8 makes
the race unreachable on the host, and D9 makes ownership explicit
wherever both mechanisms exist.

*As built (ranger-base-136b5, 2026-11-06).* Three readers, not two: the
hook, the unit render (`posse systemd`) and G7, pinned on one input table
(`internal/posse/systemdunit_test.go`). Absent, bare, `null` and `~` all
read as `herdr` — the key has a default, so `""` is the one reading
`yamlClean` and `cfg()` cannot disagree about — and a value that is
NEITHER NAME is refused by all three rather than defaulted, because a typo
read as the default is how this race gets armed. D5's ordering needed a
second key, and it is `autostart_multiplexer_unit:` with **no default**:
"name from config, not hardcoded" has no fallback that is honest, since a
name no unit wears refuses the start and a name another unit wears orders
the loop behind the wrong process. The unit is
`posse-dispatch-<instance>.service` (D8's tag is what keeps two homes'
units apart in the one directory an account has, D12), and the render
refuses a broken arm — absent, bare or unparseable `autostart_interval:` —
rather than write a unit that dies in the journal.

*Amended ranger-base-hmet1, 2026-11-06.* "Refused by all three" is a claim
about a home that ARMS. On one with no `autostart_interval:` at all the
three readers diverge, and the divergence is the honest one: `AutostartVia`
reads a value and refuses it wherever it appears; the render refuses too,
for the arm's reason; the hook — the only one of the three that runs on
such a home — names the value on stderr and exits 0, because it would arm
nothing there whatever the key said and a refusal needs an arm to refuse;
and G7 says nothing at all, its row being gated on the arm switch on
purpose (ranger-base-i6h). What this cost before the amendment was a herdr
`[[startup]]` hook that exited 1 at every server start with no governance
row naming why. The split is recorded per row in `autostartViaTable`'s
`disarmed` column.

**D10. A Rex seat proven dead is destroyed, not only forgotten.** On a
non-herdr occupant, `reclaim` calls `Close` (`session.destroy`) on the
recorded session id under the launcher lock, before it unlinks the meta.
If the destroy fails, the meta is kept and the next pass retries. An id
the server no longer knows counts as already destroyed. This is not the
reaper ADR 0076's correction rejected. That reaper destroyed sessions
when their blocks exited, so that `session.list` would mean liveness. This
destroy runs after liveness has been proven by the occupant's `List()`,
by the one home whose meta names the id. Without it, every seat that ends
by its shell exiting leaves a session that no home lists (no terminal
block) and no meta names (pruned), and that still holds one of the
server's `--max-sessions` (default 64) slots. Those slots are shared by
every home on the server. When they run out, every home's `Create` fails
with 409. herdr needs nothing here: a dead herdr workspace is gone.

**D11. The plan-usage snapshot is keyed by the Claude login, not the home.**
It moves to `~/.cache/posse/plan-usage/<sha256 of the resolved Claude
config dir>.json`, with its `.log` beside it. That is the same cache root
as the suite slots and the bd-write locks. Every home on one login then
shares one reading, one 429 cooldown and one request log. Without this,
N homes are N pollers on one rate limit, and a cooldown that home A earned
is not honoured by home B. That is the rangerhq-tdy8 incident with the
fix undone. `Shared` still gates the store, so a test home cannot write
the per-user file. There is no migration: the first read misses, and one
fetch fills the file.

**D12. What is per user, and stays that way.**

| thing | keyed by | verdict |
|---|---|---|
| suite slots, bd-write locks (`~/.cache/posse`) | user (bd lock: store realpath) | correct: the cores and stores are the box's |
| load guard, `checkorphans` | box | correct: home A's guard ends home B's *undeclared* orphan, same rule as between seats (`POSSE_KEEP=`) |
| `~/.claude`, `~/.codex`, `~/.grok` | user | D6, by design. Trust seeding locks `<cfg>.posse-lock`, so concurrent homes are safe |
| worktree root `~/.posse/worktrees/<repo>/<session>` | user | enumerated per repo through git, so there is no cross-home sweep. Collides only on same repo basename and session name; D8's procedure sets `worktrees:` per home anyway |
| herdr socket and plugin registry; Rex default socket | user | D2 and D9. posse has no Rex server selector, which D2 wants |
| metas, gates, hooks, launch and watch locks | home (`state/`) | unchanged |
| plan-usage snapshot | home | **moves to the login (D11)** |

**Alternatives rejected.**
- **A Rex server per home** (posse grows a server selector; `rexGlobals`
  already passes `--server`). Each listing would then be private, but
  that isolation buys nothing: ownership is the meta, not the listing.
  It also costs N Tailscale listeners and N hosts in the Rex app, which is
  D2 undone. If the walls stop being enough, the exit is D7's account
  split.
- **Filter other homes' rows out of `List()`.** The foreign refusals
  (kill, create, dispatch) are built on seeing those rows. Hiding them
  turns "refused, belongs to another instance" into a silent duplicate.
- **A server-wide husk sweep** (destroy every zero-window session). It
  would destroy sessions no meta of ours names: other homes', and the
  operator's own. Ownership is the meta, so only the owner destroys (D10).
- **`--max-sessions 0` as the husk fix.** That hides the leak instead of
  closing it. The cap is Rex's memory guard, and it stays.
- **A config key pointing homes at one shared plan file.** Every home
  would have to agree by hand. Deriving the key from the login makes
  sharing automatic and correct when one home uses its own
  `CLAUDE_CONFIG_DIR`.
- **The hook detects a rendered systemd unit.** That reads another
  system's files to guess ownership. The key states it.

**Measured versus assumed.**
MEASURED (laptop, scratch server, above): Rex label and duplicate
behaviour, the husk counting toward the cap, foreign rows and refusals on
Rex, and the husk surviving the prune.
STATIC (code read): (b)'s hook and lock behaviour, D11's keying, D12's table.
ASSUMED until the host's two-home run: that Linux Rex behaves like
darwin Rex on all of the above; whether `--restore` brings husks back
after a server restart; and the cost of Rex `List()` when it walks every
home's seats (1 + N + M calls at 50-70 ms each, MEASURED 2026-10-31 for
one home). Each home pays for the whole server's fleet on every pass.
That is unpriced until a second crew is running, so it is read then and
not built now.

**Beads.** ranger-base-136b5 gains D9. The D10 and D11 builds are cut below
for dinesh. D8 is added to ranger-base-yyq64 and ranger-base-offid as
procedure.

## Amendment: the Linux cage is a mount namespace, not Landlock (ranger-base-20r29, 2026-11-06)

**Context.** D4 assumed Landlock is the Linux analogue of the seatbelt
profile's path-scoped trailing deny. That profile (`SeatbeltProfile`,
seatbelt.go) has five shapes, and it is the shapes that have to carry over:

1. a write grant on whole subtrees (cwd, state dirs, TMPDIR, gates);
2. a **trailing deny under a grant**: the gates and hooks dirs inside the
   granted state dir, `.git/hooks` and the git config chain inside the
   worktree, and a PID's own `Edit(docs/adr/**)` inside cwd (ADR 0014 §3);
3. atomic-write **siblings of a granted file**, such as
   `~/.claude.json.lock` and `~/.claude.json.tmp.<pid>.<hex>`, as regexes;
4. **create-only** literals: git may make a ref dir, but write nothing in it;
5. a **read deny** on another runtime's credential file.

The host is not up (ranger-base-yyq64), and this laptop has no Linux kernel,
so nothing here is MEASURED on Linux. The facts are from primary sources,
read 2026-11-06: the kernel's Landlock doc (7.4.0-rc1), landlock(7)
(man-pages 6.19), `security/landlock/syscalls.c` at each tag, Arch's
`linux` package and config, Claude Code's sandboxing docs and issues, and
codex's `linux-sandbox` README.

**Findings.**
- **Landlock is allow-only.** "One policy layer grants access to a file path
  if at least one of its rules encountered on the path grants the access."
  Within a layer, rules combine as a union. Across stacked layers, they
  combine as an intersection, and every layer must allow. No rule can take
  back a subtree that an enclosing rule grants. So shape 2 has no Landlock
  spelling, and shape 2 is the whole of what the profile defends.
- **The directory rights cover the whole subtree.** `MAKE_*`, `REMOVE_*` and
  `REFER` "only apply to the content of a directory", and to everything
  beneath it. A file rule carries only EXECUTE, WRITE_FILE, READ_FILE,
  TRUNCATE and IOCTL_DEV. So shape 3 means granting creation in all of
  `$HOME`, and shape 4 means granting all of the parent dir. Claude Code's
  `~/.claude.json` write is a lock, then a temp file, then a rename, all in
  `$HOME`. It has crashed Claude Code under a Landlock sandbox with EACCES on
  the first config save (anthropics/claude-code#30372, on ABI 5). A maintainer
  reproduced the `$HOME`-root temp files again on 2.1.234 (#87677, open).
- **A Landlock domain forbids mounts.** "Threads sandboxed with filesystem
  restrictions cannot modify filesystem topology, whether via mount(2) or
  pivot_root(2)." Claude Code's own sandbox is bubblewrap plus socat on
  Linux. Codex's is bubblewrap plus seccomp: "Bubblewrap is the default
  filesystem sandbox". Codex keeps Landlock only as a legacy mode, which it
  rejects for filesystem-restricted policies because Landlock "cannot
  isolate app-server Unix sockets". So under a Landlock cage, codex's
  SelfSandbox (runtime.go) stops working, and so does claude's /sandbox.
- **The field's standard answer is a mount namespace.** A read-only bind on
  top of a writable bind is exactly a trailing deny. Binding `/dev/null` over
  a file is a read deny. Flatpak, Claude Code and codex all build on
  bubblewrap. It is unprivileged, needs no daemon and no group, and holds no
  memory, so D4's two objections to containers do not apply to it.
- **The host can run both.** Arch `linux` 7.2.14 sets
  `CONFIG_SECURITY_LANDLOCK=y` with
  `CONFIG_LSM="landlock,lockdown,yama,integrity,bpf"`, and 7.2 is Landlock
  ABI 10. The Arch wiki says unprivileged user namespaces are "enabled by
  default in all officially supported kernels except for linux-hardened",
  which is what bubblewrap needs.

**Decision.**
- **D13. Landlock is rejected as posse's Linux cage tier.** This holds at
  every ABI read (1 through 12): none of them adds a deny rule.
- **D14. The candidate is a bubblewrap mount namespace**, built only after
  the probe passes on the host. Each shape maps to a mount:
  - shape 1: `--ro-bind / /`, plus `--bind` on each grant;
  - shape 2: `--ro-bind` on each denied subtree, layered over the grant.
    The seal is free: renaming a mountpoint is EBUSY, and renaming an
    ancestor carries the mount with it (both ASSUMED, probes B3 and B4).
  - shape 5: `--ro-bind /dev/null <file>`.
  - Shapes 3 and 4 have no narrow bwrap spelling either. Renaming onto a
    bind-mounted file is EBUSY (probe B8). The design bead answers them,
    most likely by giving each seat a `CLAUDE_CONFIG_DIR`, which moves
    `.claude.json` and its siblings into a dir grant. The docs confirm the
    move; that the siblings follow it is ASSUMED. The answer has to sit
    with ADR 0077's typed environment and the credential-dir check in
    credentialDirVarsIn.
  - A runtime's own bwrap must still nest inside ours (probe B7). That is
    the property Landlock fails.
- **D15. The probe is the gate:** `docs/adr/0080-linux-cage.probe.sh`. It
  needs Linux, python3 and bwrap, and makes no repo writes. Its L-rows
  confirm the three Landlock refusals above. Its B-rows check every
  bwrap expectation, including that a nested user namespace cannot unmount
  or remount the deny (MNT_LOCKED, ASSUMED). Any B-row that reads
  differently from its EXPECT column reopens D14 before any design.
  **B5 is the one row whose cell cannot say that on its own**
  (ranger-base-yca4g). Its cell is the BYTES the read returned, so a read
  that FAILED returns none and reads `empty` — a match, and rightly: no
  bytes is no leak, whatever stopped them. What it does not show is WHY,
  and shape 5 is a claim about why — `--ro-bind /dev/null`, whose
  documented reading is EOF at exit 0, not an error. So a B5 that errored
  leaves shape 5 ASSUMED rather than measured, and the row prints a
  `B5 detail (not a row)` line under itself carrying the exit status and
  cat's stderr for the bead. **A detail line under a matching B5 is not a
  D14 reopen and not a pass of shape 5**: it is the one row to re-read
  before shape 5 is written into a cage, because whatever errored there
  may not be present when the cage is real.
  MEASURED 2026-11-08 on the host (ranger-base-hft2q): B5 read
  `cat: <tmpdir>...empty`, cat's error concatenated in front of the word
  by the pre-fix cell, over a credential that was not readable. The
  operator's verdict stands — the arm holds and the tier is buildable —
  but the 20 characters the old cell kept did not include the errno text,
  so which error it was is unread. The next host run's detail line answers
  it. The amendment's "Measured versus assumed" block below is the current
  reading of every row, B5 included (ranger-base-vdmm9).

**Alternatives rejected.**
- **Landlock alone**, which D4 wanted. Shape 2 is inexpressible, and it
  breaks both runtimes' own sandboxes.
- **Landlock by enumerating leaves**: grant every child of cwd except the
  denied one. A new top-level file in cwd still needs `MAKE_REG` on cwd,
  which reaches the denied child. The other choice is to freeze the tree as
  it was at launch.
- **bwrap plus a Landlock layer** for the scopes bwrap lacks (ABI 6 signal
  and abstract-socket scoping). The scoping is attractive, because it would
  make a cross-seat kill impossible rather than merely denied by the PID.
  But a filesystem domain blocks the nested bwrap. A scope-only ruleset may
  not block it, but that is ASSUMED and unread. The PID-level `pkill` deny
  already covers this. Revisit only if that deny is measured leaking.
- **landrun or go-landlock as the helper.** Both are maintained (go-landlock
  v0.11.0, 2026-10-23; landrun v0.1.17, 2026-07-22). Both inherit Landlock's
  shape problem, and go-landlock's BestEffort "will succeed without error
  even when Landlock is not available". That is a fail-open, which posse's
  "refuse at launch" rule (D4) forbids.
- **Containers.** D4's reasons stand.

**Exit hatch.** bubblewrap is one binary that posse types onto the launch
line, as it types `sandbox-exec` today. It holds no state. Removing it
leaves the cooperative tier D4 already runs.

**Measured versus assumed.** The amendment above was written before the
probe ran, so the "ASSUMED" notes inside D14 and D15 are that first state.
This block is the current one (updated 2026-11-08, ranger-base-vdmm9).
- MEASURED 2026-11-08 on the D1 host (ranger-base-hft2q, run as the
  service account, checkout 7d3446ef; Linux 7.2, bubblewrap 0.13.1,
  Landlock ABI 10, `kernel.unprivileged_userns_clone=1`):
  - Landlock, D13: L0 ABI 10; L1 allowed (the trailing deny cannot be
    spelled); L2 refused and L2b allowed (a file rule allows an in-place
    write, not the atomic replace); L3 and L3b refused (a Landlock domain cannot start
    bwrap). Every row read as its EXPECT.
  - bubblewrap, D14: B0 and B1 allowed; B2 refused (shape 2); B3 and B4
    refused (the seal, both halves); B6 and B6b refused (a nested user
    namespace cannot unmount the deny or remount it rw: MNT_LOCKED holds);
    B7 allowed (a runtime's own bwrap nests inside ours); B8 refused (an
    atomic replace of a bound file is refused, so shapes 3 and 4 need a dir
    grant); B9 claude, codex and grok `--version` start under the wall.
    Every row read as its EXPECT. These retire the B3/B4, B6, B7 and B8
    assumptions above.
  - B5, shape 5, measured only in part. No byte of the credential reached
    the seat, so the read deny holds. But the read FAILED: the cell printed
    cat's error where `/dev/null`'s silent EOF was expected. So shape 5 was
    shown by an error, not by its stated mechanism, and which error is
    unread. The old cell kept 20 characters, and the tmpdir path used them
    up. ranger-base-yca4g makes the next run print the exit status and the
    errno text.
- MEASURED, this tree: the five shapes are read from seatbelt.go.
- REPORTED (primary sources, 2026-11-06): every quoted fact above.
- ASSUMED, still:
  - that `--ro-bind /dev/null <file>` masks a file through a clean EOF (B5,
    above). The design bead may only rely on "the read gets no bytes";
  - that the `.claude.json` siblings (`.lock`, `.tmp.<pid>.<hex>`) follow
    `CLAUDE_CONFIG_DIR`. No probe row tests this: B9 runs `--version` only,
    which writes no config. ranger-base-rwcqg must measure it before its
    shape 3/4 answer is buildable;
  - that the runtimes work under the wall past `--version`: a turn, a
    config save, codex's SelfSandbox nested in a real session. B7 shows
    only that bwrap nests in bwrap;
  - that a scope-only Landlock ruleset leaves nested bwrap alone
    (Alternatives rejected). No row tests it.

## Amendment: the bwrap tier, as built (ranger-base-rwcqg, 2026-11-08)

**Context.** D14 named bubblewrap and the host probe passed (ranger-base-hft2q).
This section is the buildable design: what the tier is called, how each of
the five shapes renders, what shapes 3 and 4 become, how codex nests, and the
launch line. Every renderer input already exists. `SeatbeltWritable`,
`SeatbeltCarveOut`, `credentialReadDenyLiterals` and `sessionRefDirs` produce
the same lists for both mechanisms. Only the last step, list to wall, differs.

**D16. The tier keeps its name, `seatbelt`; the mechanism is a host fact.**
`cage: seatbelt` means "the L2 path-scoped write wall" (ADR 0002 §3). It is
realized by `sandbox-exec` on darwin and by `bwrap` on Linux, the way `cage:
container` is realized by whichever engine the home names. `WallMechanism()`
returns `sandbox-exec`, `bwrap` or `""`. On Linux, `AvailableCages[CageSeatbelt]`
is true when `bwrap` resolves and an apply probe passes:
`bwrap --ro-bind / / /bin/true` through `protocolCmd`, the twin of
`sandboxApplyProbe`. The probe catches a kernel or LSM that refuses
unprivileged user namespaces (linux-hardened, Ubuntu's AppArmor sysctl).
Parity's cage row and `posse gates` print `seatbelt (bwrap)` or
`seatbelt (sandbox-exec)`. A PID never names the mechanism.

*As built (ranger-base-eoxo1, 2026-11-08).* The decision is unchanged. What
the build had to settle is that **`WallMechanism()` answers a NAMING question
and not an availability one**, and the two are now kept apart on purpose.
It reads nothing but `GOOS`: `sandbox-exec`, `bwrap`, and `""` only on a GOOS
with neither. `AvailableCages[CageSeatbelt]` stays the measured half — on
Linux, `bwrap` resolving plus the apply probe passing, filled by one
`init` in `internal/posse/wall.go` that asks the question for both
mechanisms rather than one init per mechanism. Folded together, a Linux host
whose kernel refuses unprivileged user namespaces would print bare
`seatbelt`, which reads as a tier with no mechanism; apart, it prints
`seatbelt (bwrap)` over the block of ✗ rows that say the wall did not go up,
which is the honest reading and the one with a remedy in it. The label is
`CageLabel(cage)`, a label and never a value: `Parity.Cage` stays the bare
tier, because it is compared against the constant and written into session
records.

The probe is the twin D16 names with **one deliberate difference, which is
the direction it fails in**. `sandboxApplyProbe` drops a failure that is not
an apply refusal, because its `""` is read as "this process may apply a
profile" — a permission — and a host problem belongs to whatever check meets
it. Here `""` is read as "this host provides the tier", so every failure is a
refusal: a missing `/bin/true`, a `bwrap` too old for these flags, a seccomp
refusal and the resolver's own `could not ask` all leave the tier
unavailable. "Could not ask" is not "the wall is up".

*A THIRD question, and the window between D16 and D18 (ranger-base-pl3ck,
2026-11-08).* D16 as built answers "which mechanism" and "can this host", and
nothing between the availability table and the launch line asked **can posse
type it**. `wallRendered` reads the table, and both of its call sites then
type `SeatbeltPrefix` — `sandbox-exec -f <prof>` — unconditionally. So with
D16 landed and D18 not, a `cage: seatbelt` launch on the Linux host would have
reported its L2 rules REALIZED (parity's `seatbelt :=` reads only
`cageAvailable`) and typed a binary that host does not have, where before D16
the same launch was REFUSED with "cage seatbelt is not available on this
host/build". Not hypothetical: the host probe measured bwrap 0.13.1 present
with B0 allowed (ranger-base-hft2q), so both of D16's questions answer yes
there. The table fill therefore reads all three —
`seatbeltTierProvidedOn` = `wallMechanismRendered(WallMechanism())` **and**
the measured availability — and `wallMechanismRendered` admits
`sandbox-exec` alone. The question is about TYPING the wall: D17's `BwrapArgs`
is a pure argv render with no call site on a launch line, so it does not lift
this. **D18 lifted it in one line** (ranger-base-djl3c), in the commit that
replaced both `SeatbeltPrefix` call sites, and the pin beside it
(`TestQAWallTableClaimsNoTierThisBuildCannotType`) reds if either lands
without the other — it now holds the BICONDITIONAL instead of the hold:
every mechanism `wallMechanismOn` can name is one `RenderWall` has an arm
for, so the hazard it watches has moved from "bwrap is available and
untyped" to "the next mechanism is named and untyped". The third question is
kept apart from the other two for
the same reason they are kept apart from each other: folded into
availability it would make the apply probe answer for a render, and folded
into the NAME it would print bare `seatbelt` on that Linux host again.

The rename landed: `seatbeltWallRendered` is `wallRendered`, in
`internal/posse/seatbelt.go`, and D21's text below should be read against
that name. It still carries `!rt.SelfSandbox` unconditionally — D21's flip
was GATED on probe T6 and that gate answered refused (ranger-base-bjk4k;
see D21's as-measured note), so the unconditional reading is now the settled
one and not a hold — and `credentialDirEnvSetRefusal` (credential.go) follows
the rename unchanged, because it always asked about the wall and never about
the binary.

**D17. The render.** `BwrapArgs` is a pure function from those lists to an
argv. The order is fixed, and a later mount covers an earlier one:
1. base: `--die-with-parent --ro-bind / / --dev-bind /dev /dev --chdir <cwd>`.
   No `--unshare-pid`, `--unshare-net`, `--proc`, `--tmpfs /tmp` or
   `--new-session` (see Alternatives).
2. shape 1: `--bind p p` for each writable DIRECTORY, shallowest first. A
   writable path that does not exist yet is `mkdir -p`'d by the launcher
   first. The launcher is outside the wall, and seatbelt granted such paths
   before they existed (`resolveExisting`).
3. shape 2: `--ro-bind d d` for each `Deny`. If `d` is missing and its nearest
   existing ancestor is inside a write grant, the launcher makes it as an
   empty directory first — unless the carve-out says the producer means that
   path as a FILE, in which case nothing is rendered and nothing is created
   (ranger-base-ppnds, below). Otherwise nothing can create it and no mount is
   rendered. `Seal` renders nothing: renaming a mountpoint, or carrying it
   away by renaming an ancestor, is refused (B3, B4).
4. `Keep`: the launcher creates each file, then `--bind f f`. That is an
   in-place append on a bound file, which B8 does not touch (B8 is a rename).
5. shape 5: `--ro-bind /dev/null f` for each `DenyRead` that exists at launch.
   A missing one renders nothing, and parity prints the residual (Consequences).
A writable entry that is a regular FILE has no bwrap spelling (B8). The
render refuses with the file named, except for the git entries D20
replaces and claude's `~/.claude.json`, which D19 removes.

*As built (ranger-base-qoamy, 2026-11-08).* `BwrapArgs` in
`internal/posse/bwrap.go`, beside the availability half, pinned by
`internal/posse/bwrapargs_qa_test.go` over fixture trees on darwin. Five
shapes as listed, with three things the text did not settle and one residual
for D18. Full reasoning in `docs/notes.d/ranger-base-qoamy.md`. The launch
wiring it was written without is D18's and has landed (ranger-base-djl3c):
`RenderBwrap` calls this, runs its prep and writes its argv.

- **The order is SBPL's last-match-wins, and the gates dir is the proof.**
  The persona's own gates dir is in three lists at once — a write grant,
  inside the `state/gates` deny, holding two Keep literals — so rendering
  grant, then deny, then Keep reproduces exactly what SBPL resolves by taking
  the last match: no writing its own shims, and both audit logs appendable.
  That is the arm driven from `SeatbeltWritable`/`SeatbeltCarveOut` rather
  than from a fixture list.
- **DIVERGED on shape 2's missing-deny test, to this item's own stated
  reason.** The letter asks whether the deny's nearest EXISTING ancestor is
  inside a write grant; the reason given is "otherwise nothing can create it".
  They disagree when the GRANT does not exist yet, which is a fresh home:
  `a.StateDir` is a grant, `a.StateDir/gates` is a deny, the deny's nearest
  existing ancestor is the home, and the letter renders no mount over a grant
  that is about to be created and bound rw. The build asks whether the deny is
  inside a write grant, which answers the reason, is never narrower, and
  covers the prep-created grants.
- **Two refusals this item does not name.** A stat that fails for any reason
  other than "not there" refuses the render naming the path and the list, so
  that an unreadable path cannot become a missing one (for a deny, missing is
  no mount at all). And a DIRECTORY at a Keep path refuses, because binding it
  would hand back the tree the deny just closed.
- **Residual for D18: a missing grant is not free here.** `SeatbeltWritable`
  carries four darwin-only literals unconditionally — `/private/tmp` and three
  under the home. At L2 a grant for an absent path costs nothing. Here the
  launcher must create it, outside the namespace, as the seat's user — so on
  Linux every seat's prep would ask for a `mkdir -p` at the filesystem root
  that an unprivileged launcher cannot make, plus three junk directories in
  the home. Narrowing those four to darwin in `SeatbeltWritable` is the
  smaller fix and serves both mechanisms; a launcher that merely refuses the
  step leaves the wall announcing grants it does not have.

  *That residual took on the right class and then enumerated the wrong
  members* (ranger-base-ppnds, holden's finding 1 off ranger-base-vnp01). It
  named four darwin-only literals under `/private/tmp` and the home, and
  missed every GIT LOCK PATH — none of which is darwin-only, and two of which
  damage the SHARED repo rather than the session. The class is not "a path
  that cannot be made" but "a path made as the WRONG KIND", and item 3's
  letter is faithful about it: it says `mkdir -p` with no kind distinction,
  because SBPL never needed one. Several `Deny` entries are files that are
  absent on every healthy repo — `<gitdir>/config.lock`,
  `<gitdir>/config.worktree`, `config.worktree.lock`, from
  `sessionGitConfigFiles` and `sessionWorktreeIdentityFiles`, and the beads
  store's own through `beadsGitDirs` — and all of them sit inside a write
  grant, so the missing-deny arm fired on each. MEASURED by hand against a
  real repo at git 2.50.1, each with a green control first: a directory at
  `config.lock` makes `git config` die at `could not lock config file` while
  commit and status go on working; at `config.worktree` it is inert until
  `extensions.worktreeConfig` is set (git sets it itself for
  `sparse-checkout`) and then `status`, `config --list` and `commit` all exit
  128 at `fatal: unknown error occurred while reading the configuration
  files`; at `config.worktree.lock` it refuses `git config --worktree`. Shape
  1's absent-GRANT arm has the same hole — a directory at `<ref>.lock` is
  `fatal: cannot lock ref 'HEAD'`, at `packed-refs.lock` it is a thing
  AGENTS.md makes the operator's to remove and no persona's — and D20 has
  since removed all three from the set before the render sees it, leaving
  claude's `~/.claude.json` as D19's.

- **DIVERGED again on shape 2's missing-deny arm, and this one needs a
  channel** (ranger-base-ppnds). `SeatbeltCarveOut` gains `DenyFile`: the
  subset of `Deny` its PRODUCER means as a file, with a `FileShaped(p)`
  predicate for the render and "not marked" meaning a directory. The render
  cannot work the kind out — an absent path's shape is exactly what cannot be
  stat'd — and must not guess, so the producers say: both git producers above
  (files by construction), plus `HomeConstitutionFiles` (`config.yaml` and the
  promote manifest). An absent file-shaped deny then renders **no mount and no
  prep**, and `touch` is not the other half of the fix: an empty file at
  `config.lock` is the same `File exists` refusal the directory buys. Stated
  residual, because a mount cannot say "this name may not come into being":
  the session can create those files inside the grant that contains them, and
  nothing is over them when it does. It still cannot make one take effect —
  the file each lock renames onto is itself bound read-only and B8 measured
  that a rename cannot cross a mount point — so the refusal lands one step
  later than under SBPL (`could not write config file` where seatbelt says
  `could not lock`, both arms measured in `seatbeltgitidentity_qa_test.go`),
  and what remains is a stray lockfile and an inert `config.worktree`. Both
  are smaller than a directory the launcher makes at every launch.
  **Shape 1 still guesses**, and that is a difference in what producers can
  say: a file-shaped GRANT arrives from a `state_dir:` declaration, and ADR
  0012 D4's key says "directories (and single files)" without saying which.
  D20 answered the same question one step earlier for git's three, by taking
  them out of the set in `BwrapWritable`; `~/.claude.json` is D19's, and the
  next file-shaped grant after that reopens this, with its own producer the
  thing that closes it.

- **The producer-driven arm's cwd was not a git repo** (ranger-base-ppnds,
  finding 3 — and it is why the above escaped).
  `TestBwrapArgsRendersTheTiersOwnLists`
  is the one arm driven from `SeatbeltWritable`/`SeatbeltCarveOut` rather than
  a hand-built list, and its cwd was a fixture DIRECTORY, so `gitPath` failed,
  `LinkedGitDirs` answered nothing, and not one file-shaped deny or git grant
  reached the render in ANY of the file's arms. Its prep assertion was
  `writeGranted(writable, s.Path)` — inside a grant, never the right KIND —
  which stayed green over all five directories. It is a `git init` plus a `git
  worktree add` now, with the kind of every step asserted against the
  carve-out and a positive witness that a file-shaped deny under the session's
  git dirs really is absent and inside a grant. It drives `wgNewFixture` and
  D20's own `bgSwap`, so it hand-assembles nothing and is the call D18 will
  make — `SeatbeltWritable`, `BwrapWritable`, then `BwrapArgs` with the
  carve-out and the keep file. D20's arms drive the same fixture with an
  EMPTY carve-out; what is only here is a real one and the KIND of every prep
  step it produces.

**D18. The launch line, identical on herdr and Rex panes.** `RenderBwrap`
writes the argv NUL-separated to `state/gates/<persona>/bwrap.args`, beside
`seatbelt.sb`, and writes a readable copy to `bwrap.txt` headed by the
`RenderedByPosse()` stamp. The prefix goes in `SeatbeltPrefix`'s slot,
between the PATH assignment and the runtime command:

    3<'<state>/gates/<p>/bwrap.args' '<abs bwrap>' --args 3 -- <runtime cmd>

A redirection before the command name is POSIX. The line already assumes a
POSIX pane shell (`PATH=…` prefix, `"$(cat …)"`), and Rex's `rexShell` is
`realShell`. `bwrap` is typed as the absolute path resolved at render, not
bare, so a mise shim or a gates-dir shim cannot stand in for it. Both call
sites that prefix today (herdrback.go, the create and relaunch paths) change
together, through one `WallPrefix(rt)` function.

*As built (ranger-base-djl3c, 2026-11-08).* The line is D18's letter
unchanged. `RenderBwrap` in `internal/posse/bwrap.go` (the D18 section),
`BwrapPrefix` beside it, and **one mechanism switch for the whole tier** in
`internal/posse/wall.go`: `RenderWall` returns a `WallRender`
(mechanism · artifact · prefix) and `WallPrefix` is what the two launch call
sites type. `wallMechanismRendered` admits `WallBwrap` in the same commit, as
D16's note above demands. Pinned by
`internal/posse/bwraplaunchline_qa_test.go` on darwin. Full reasoning in
`docs/notes.d/ranger-base-djl3c.md`.

- **The three things the prefix is typed in front of, and the three that
  moved with it.** The letter named two call sites; the build found four
  surfaces that decide a wall and had to answer for all of them.
  `herdrback.go`'s create and relaunch paths are the two. The third is
  `reachability.go`, whose seatbelt arm would have returned early on Linux
  (`!SeatbeltAvailable()`) and left ADR 0013 §4's record-reach row ABSENT for
  every bwrap seat — a check that measured nothing printing the same as no
  check at all, which is the one shape that file exists to refuse. It gets a
  bwrap arm that judges by TYPING the prefix under `/bin/sh`, so the
  redirection and the quoting are measured at a launch and not only by probe
  T7. The fourth is `posse gates`, now `WallReport`.
- **`WallRender` is one struct because the three cannot be allowed to
  disagree.** The prefix NAMES the artifact — `-f <prof>` on one mechanism,
  `3<<args>` on the other — so a caller that rendered one and reported the
  other would print a wall nobody is behind. Parity's reach row is exactly
  such a caller, and its pin is that the prefix contains the artifact.
- **D17's residual is closed the way qoamy §4 recommended**, in
  `SeatbeltWritable`: `homeCacheGrants(goos)` and a darwin guard on
  `/private/tmp`. Unnarrowed, every Linux seat's prep list carried a
  `mkdir -p /private/tmp` at the filesystem root (a refused launch) plus
  three junk directories in the home. The darwin render is byte-identical —
  `add` absResolves and `/tmp` IS `/private/tmp` there, so the fourth literal
  was already redundant — and the Linux list gains the Go toolchain's
  telemetry counters at their `os.UserConfigDir()` spelling, `local` and never
  its parent, because `telemetry/mode` beside it is what decides whether the
  box UPLOADS them.
- **A prep `touch` never truncates, and that is a pin rather than a note.**
  The launcher creates the session's two audit logs so the Keep binds can go
  back over the deny. `os.Create` would empty a live seat's `refusals.log` at
  every relaunch, and the only reader who would notice is whoever goes
  looking for a refusal that was recorded and is gone.
- **`bwrapBin` is a package variable**, the third host measurement to become
  one (beside `bwrapResolves` and `bwrapApplyRefusal`) and for their reason:
  without it the bwrap arm of `RenderWall` is unreachable on the box that
  builds it, and an untested arm that puts up an L2 write wall is the one
  nobody may be handed on a guess.
- **What is still untyped is the mechanism that does not exist yet.** The
  third question stays a separate predicate after D18 because `RenderWall`
  refuses the mechanism it has no arm for, and the predicate is what keeps
  that refusal from being where a host first hears about it.
- **`wallMechanismHost` is the fourth such variable, and it is the one the
  other three needed** (ranger-base-temhf, 2026-11-09). Those three make the
  bwrap RENDER reachable from darwin and `renderWallOn`'s mechanism parameter
  makes the switch reachable — but a LAUNCH asks `WallMechanism()`, so
  nothing could put `planLaunch` or `CreateSession` behind the other
  mechanism, and the launch paths are exactly where the two mechanisms differ
  in what they REFUSE: D19 below is a refusal on one host and silence on the
  other. D18 landed `make test` green on darwin and red on all six ubuntu
  jobs of ci.yml — seven tests over all three arms, none of them about the
  wall, on main's only gate. `TestQALaunchAtTheOtherHostsWallMechanism` and
  `TestQAClaudeAtTheBwrapMechanismIsRefusedAtTheLaunchAndNotOnDarwin`
  (`internal/posse/walllaunchmechanism_qa_test.go`, arm 1) drive a real launch
  at both mechanisms from either host now. **Whoever builds D21 wants this
  door** (D21 is retired; the door stays for the next launch-path difference): the `!rt.SelfSandbox` flip is a launch-path difference between the
  two mechanisms, which is this class exactly. Full reasoning in
  `docs/notes.d/ranger-base-temhf.md`, including the two side effects the red
  surfaced — a `writable:` in a PID naming a path outside the session tree is
  a directory posse CREATES on the host at this mechanism, and a bwrap launch
  materializes `<cwd>/.beads`,
  which moves ADR 0013 §4's record-reach row for a LATER launch in the same
  tree.
- **`seatbelt (bwrap)` is this label and NOT a `cage_realize_<goos>:` entry,
  so the host's cutover map carries no `seatbelt` line at all**
  (ranger-base-ybz6u, 2026-11-09 — asked because the label alone cannot tell
  the two apart, and the host cutover rests on the answer). D16 above already
  rules it: the tier keeps its name and the mechanism is a host fact, the way
  `cage: container` is realized by whichever engine the home names, and a PID
  never names the mechanism. `seatbelt (bwrap)` is `CageLabel`'s output — a
  label and never a value, and `Parity.Cage` stays the bare tier. ADR 0083 D7
  is a different thing in every respect that matters: it maps one TIER to
  another, it is an explicit promoted line that fails closed when absent, and
  it marks every session it lowers with a ratified waiver. A mechanism maps
  nothing and waives nothing. D7's own second bullet is "a tier maps to itself
  where `AvailableCages` has it", and D6 option C names this tier as what ENDS
  the `seatbelt → shims` entry — "the D7 map flips from `shims` to the bwrap
  tier, one promoted line" — so once the Linux host realizes `seatbelt`, the
  correct `cage_realize_linux:` has NO `seatbelt` key in it. Nothing to
  change here; pinned rather than reasoned, in the last block of
  `TestQALaunchAtTheOtherHostsWallMechanism`
  (`internal/posse/walllaunchmechanism_qa_test.go`, arm 1): over a launch at
  either mechanism in a home that declares no `cage_realize_*` key, the
  demanded tier is the realized tier — the bare constant, an empty `Ratified`
  and an empty `Degraded`. MEASURED: the three red together under a mapping
  that IS present.

**D19. Shape 3: claude's config lives in a dir grant because the seat's
`CLAUDE_CONFIG_DIR` points at one. There is no per-seat dir.**
MEASURED today (below): claude writes `.claude.json`'s lock and temp siblings
inside `$CLAUDE_CONFIG_DIR`. Every claude seat on this box already runs with
it set to `~/.claude`, which ADR 0077 D1 types onto the line and which is a
whole-dir grant. So on the bwrap mechanism:
- the launcher's resolved `CLAUDE_CONFIG_DIR` must be set and absolute. The
  value typed on the line (ADR 0077 D1) is the value the grant uses, so the
  wall and the runtime cannot disagree about it;
- claude's state grant is that directory. The `~/.claude.json` file entry and
  its `SeatbeltSiblings` drop out;
- unset, the bwrap render refuses: `claude on cage seatbelt (bwrap) needs
  CLAUDE_CONFIG_DIR set in the launcher; set it in the home's unit
  Environment=`. That is host procedure for ranger-base-yyq64/offid, one line;
- `credentialDirVarsIn`'s env-set refusal is unchanged and now covers bwrap
  too, because `seatbeltWallRendered` becomes the mechanism-neutral
  `wallRendered`. An env set that moves the config dir would move the
  runtime's write out of its grant (and, for non-claude seats, past the read
  mask) on either mechanism.
- On Linux, claude's credential is a file inside the config dir (the path
  `credentialFileCandidates` already resolves). It is inside a claude seat's
  own grant, so it is not masked there. For every OTHER runtime's seat it
  joins `credentialReadDenyLiterals` as a sibling, the way codex's
  credential file does for claude. That function currently adds claude's
  file on darwin only.

*As built (ranger-base-tc41o, 2026-11-08).* `BwrapStateDirs` in
`internal/posse/bwrap.go` — `rt.StateDirs` as this mechanism can grant them
— plus one branch in `credentialReadDenyLiterals` (`internal/posse/seatbelt.go`).
Pinned by `internal/posse/bwrapstatedirs_qa_test.go` on darwin, for the
reason D17's pins run there. Full reasoning in
`docs/notes.d/ranger-base-tc41o.md`. D18 wired it (ranger-base-djl3c):
`RenderBwrap` takes its state dirs from here, so the unset refusal is now a
refused LAUNCH and is pinned as one.

- **Three refusing arms, not one.** Unset, set-and-EMPTY (posse's rule is
  that empty is out of force, and the runtime resolves its config DIRECTORY
  with `??` and its config FILE with `||`, so an empty value is a
  cwd-relative directory to one and the home to the other), and in-force but
  not absolute — which `runtimeHomeOverride` makes unreachable on any box
  with a working cwd, and which stays spelled because the demand above is
  "set AND absolute". The pure half, `bwrapClaudeStateDirs`, takes the
  resolved directory and the in-force bit as arguments so that last arm has a
  pin.
- **`~/.claude` STAYS; only the file entry drops.** The letter says so, and
  there is a second reason outside this item: `credentialReadDenyLiterals`'
  `own()` is keyed to the DECLARED `state_dir` literal, so a list that had
  lost the tree would read there as "not a claude seat" and mask a Linux
  claude seat off the credential this item's own last bullet spares it. The
  coupling is pinned from one function's real output into the other.

  *The second reason is no longer load-bearing, and the declaration still
  stays* (ranger-base-ppnds, holden's finding 2 off ranger-base-vnp01).
  `own()` asked string equality against the declared literal, which is a
  different question wearing the same name: `state_dir:` REPLACES the built-in
  list (ADR 0021 D1), so what arrives is whatever this box's operator typed,
  and the arm below newly subjects claude to that test off darwin — where the
  file is the store of record and not a keychain fallback. MEASURED on the
  linux arm, `credentialReadDenyLiterals("linux", <state_dir>)`:
  `["~/.claude" "~/.claude.json"]` spared, `BwrapStateDirs`' own output
  spared, and then `[<home>/.claude]` DENIED (the same directory, spelled
  absolutely), `["~/.claude/"]` DENIED (one trailing slash),
  `[/srv/claude-state]` DENIED (an overlay). A seat denied its own credential
  cannot authenticate at all. `own()` asks containment now — does any file of
  this store lie under a directory the launch declared, through `underDir`,
  which is what every other containment question in that file uses — so the
  three sibling runtimes' inherited half of the same coupling closes with it,
  whole-store as it has always been (a codex seat declaring `~/.codex` on a
  box exporting a moved `CODEX_HOME` is still spared both spellings, which is
  ranger-base-b52r3's residual kept where it was). The overlay row stays
  denied and that is the answer rather than a residual: `SeatbeltWritable`
  grants the declared state dirs and nothing else, so on that declaration
  nothing grants `~/.claude` either, and the wall and the grant agree about
  which directory is this seat's state. The `~/.claude` entry stays for the
  letter's own reason and for ADR 0019 D2 store 3's — whatever the CLI wrote
  in the home before the variable moved the write is still sitting there.
- **The last bullet's `sibling()` arm went in as `own()` and not as a GOOS
  branch.** "On Linux that file is the store of record" is a reason to spare
  the seat that owns it and was read for two beads as a reason to open it to
  every seat, so before this a Linux codex, grok or bob seat could read
  claude's credential. The arm went in as
  `goos == "darwin" || !own("~/.claude")` and is
  `goos == "darwin" || !own(credentialFileCandidates()...)` since
  ranger-base-ppnds (the bullet above), through `credentialFileCandidates`
  on both halves so the Linux wall follows the variables the way the darwin
  one has since ranger-base-x5f6p. Two assertions of ABSENCE reversed, each
  given its counterpart row in the same table rather than an edited
  expectation.
- **The darwin render is untouched and pinned so.** The cheap version of this
  item would have dropped `~/.claude.json` from the declaration, which would
  take the `(subpath …)` grant and its sibling regex out of the darwin
  profile too and put ranger-base-cypy1's lost config write back on every
  seatbelt seat on this box. So: the declaration still carries the file,
  `SeatbeltSiblings` still grants its scratch namespace, and
  `BwrapStateDirs` is the only thing that knows otherwise.

**D20. Shape 4: git's ref writes need a dir grant, and the dir is
`refs/heads/<branch parent>`.** Git writes a ref as `<ref>.lock` and then a
rename onto `<ref>`. Both need write on the ref's directory, so the leaf and
`.lock` pair in `sessionGitGrants` has no bwrap spelling. The wall instead
binds the ref's parent directory, `<common>/refs/heads/posse` for the fleet's
`posse/<session>` branches. The launcher creates it, and puts an empty
`.posse-keep` in it, before the wall goes up. The keep file matters: an
outside `git pack-refs --all --prune` (any gc) otherwise empties and removes
the bound directory under a live seat, and its next commit fails. Git skips
dot-named entries in a loose ref directory (ASSUMED, probe T4c/T4d). A
slashless branch in a linked worktree would bind `refs/heads` itself, which
exposes `main`. So it gets no ref grant: the commit fails loudly, and parity
says why. `<common>/packed-refs.lock` is dropped. Granting it means a
writable `<common>`, which can create a missing `packed-refs` or rewrite
`HEAD`. The cost is ADR 0059's residual, the one the container tier already
has: a sequencer that deletes a pseudo-ref exits 0 and leaves it, and
AGENTS.md's git shim refuses that run (probe T5).

*As built (ranger-base-gi1gi, 2026-11-08).* `BwrapWritable` in
`internal/posse/bwrap.go`, beside D19's half, plus `applyBwrapRefGrant`'s
parity row and one line in `checkParityIn`. Pinned by
`internal/posse/bwrapgitrefs_qa_test.go` over real linked worktrees on
darwin, and by `internal/posse/bwrapgitrefslive_test.go`, which skips off
Linux or without `bwrap` and mirrors probe T2-T4e against the RENDERED argv
rather than a hand-typed one. Full reasoning in
`docs/notes.d/ranger-base-gi1gi.md`. D18 wired it (ranger-base-djl3c):
`RenderBwrap` renders from `BwrapWritable`'s set and the launcher touches the
keep file, so `applyBwrapRefGrant`'s parity row now prints on a bwrap host.

- **The swap is a filter over the writable SET, not a change to
  `sessionGitGrants`.** Same rule D19 kept for `~/.claude.json` and for the
  same reason: narrowing the shared producer would take the leaf pair out of
  the darwin profile too, and put ranger-base-uuze's lost commit back on
  every seatbelt seat on this box.
- **The keep file is the one render input that renders NO mount** — a
  `PrepStep` and nothing else, which is why it is `BwrapArgs`' own trailing
  argument and not a sixth list. It is emitted with the write grants and not
  with `Keep`, because the prep list's order is a promise to the launcher and
  the keep file's directory is a grant one loop up.
- **T4b-T4d are MEASURED now, on darwin, and T4d's expectation was wrong.**
  Three of that row's claims are git's behaviour rather than the kernel's, so
  they did not need the host: with the keep file `pack-refs --all --prune`
  leaves the directory and the next commit lands; dot-named,
  `for-each-ref`, `show-ref`, `branch`, `log --all` and `gc` are all clean at
  exit 0; **`fsck` is not** — exit 8, `badRefName: invalid refname format`
  and `badRefContent`. The probe's T4d row read `clean` and now reads
  `flagged`. What the DOT buys is larger than "git skips it": named without
  it, the same empty file makes `git gc` and `git log --all` exit 128 in the
  OPERATOR's checkout (`fatal: bad object`, `failed to run repack`) — a
  worse failure than the prune, and one that lands on the operator. All of
  it MEASURED 2026-11-08, darwin 25.6.0, git 2.50.1 (Apple Git-155).
- **What is left for the host is one question, and it is narrower than T4e.**
  Whether the prune can remove a directory that is a MOUNTPOINT in a live
  seat's namespace at all: `vfs_rmdir` answers EBUSY for a dentry anything is
  mounted on, and bwrap's bind target is the same dentry, so the prune may
  simply fail and the keep file be belt-and-braces. The design holds either
  way — the file costs an empty inode — so the live pin asserts the
  COHERENCE instead of the outcome: what must never happen is the seat's
  commit exiting 0 while the operator's checkout does not see the branch
  move, which is a ref write into an unlinked directory with nothing red.
  The first run on the D1 host logs which answer it got.

**D21. codex is walled on Linux.** *RETIRED 2026-11-09 (ranger-base-r2fz9):
the D1 host is claude-only, so there is no codex seat to wall. The text and
both notes below are kept as the record; the last amendment says what
replaces them.* On darwin, `seatbeltWallRendered` skips
SelfSandbox runtimes because nested `sandbox_apply` is refused. On Linux, B7
measured bwrap nesting inside bwrap. So `wallRendered` drops `!rt.SelfSandbox`
when the mechanism is `bwrap`, and parity's "cannot wrap codex" declared
difference becomes darwin-only. codex's own sandbox then runs inside ours,
and a write needs both walls to allow it. That is stronger than darwin, where
codex's own sandbox is the only file wall. Gated on probe T6 (`codex sandbox`,
no turn). Until T6 reads as expected, the flip stays off and Linux codex
keeps darwin's posture.

*THE GATE ANSWERED, AND IT ANSWERED REFUSED — SO THE FLIP STAYS OFF
(ranger-base-bjk4k, 2026-11-08).* The host run of T6 is in: MEASURED on the
D1 host, Linux 7.2, bubblewrap 0.13.1, codex 0.167.0, **T6 EXPECT allowed GOT
refused**, with T6b and T6c refused as expected (ranger-base-r5m98's comment
carries the whole table). So the condition this item was gated on is not met
and `wallRendered` keeps `!rt.SelfSandbox` unconditionally: Linux codex keeps
darwin's posture, parity's "cannot wrap codex" stays mechanism-neutral, and
nothing in D21 is built.

**What the reading does NOT establish, and this is the part worth keeping.**
A refused T6 is consistent with D21's premise being wrong, and it is equally
consistent with two defects in the probe — and the probe cannot tell them
apart, because `try()` reports `refused` for any non-zero exit and discards
the stream that would say which:

1. *The argv may never have entered a sandbox.* `codex sandbox` takes a
   `--permission-profile <NAME>` that T6 does not pass. On the one codex
   available to measure — **MEASURED 2026-11-08, darwin 25.6.0, codex-cli
   0.153.4** — T6's exact argv shape is a usage error, `exit 2`, "the
   following required arguments were not provided: --permission-profile
   <NAME>", before any sandbox is applied; supplying the flag then wants a
   `[permissions]` table in the config. That version is not the host's
   0.167.0 and this is therefore not a measurement OF the finding — but a
   usage error produces exactly the row pattern the host returned, all three
   refused, which is why the reading cannot be read as nesting.
2. *T6's wall is not the wall posse renders.* The prefix is
   `$BASE --bind $WT $WT --bind $W/out $W/out`, so under `--ro-bind / /` the
   seat gets no writable `~/.codex` and no writable `/tmp`. The shipped wall
   grants both — codex declares `StateDirs: ["~/.codex"]` and
   `SeatbeltWritable` adds TMPDIR — and codex writes there at startup before
   it does anything else: it materializes `apply_patch`, `applypatch` and
   `codex-execve-wrapper` plus a `.lock` under `$CODEX_HOME/tmp/arg0/`
   (MEASURED, same box and version; on darwin an unwritable `CODEX_HOME`
   only warns, "could not create PATH aliases: Operation not permitted").

**Why T6b and T6c do not rescue it.** Both expect `refused` and both got it —
but a codex that refuses every write, or never started, satisfies them too.
Once T6 is refused those two rows distinguish nothing, so the composition
claim ("a write needs both walls to allow it") has no positive evidence in
this run at all. A probe whose expected-pass row is the only one carrying
information needs that row to fail LOUDLY, which is what the amendment below
is for.

**The probe is amended, not re-read** (`docs/adr/0080-bwrap-tier.probe.sh`,
ranger-base-bjk4k): T6 carries codex's own stderr into the GOT cell instead of
a bare `refused`, binds a writable `CODEX_HOME` so the wall under test is the
one posse renders, and gains **T6z, an unwrapped control** — the same codex
argv with no bwrap at all, expecting `allowed`. T6z is the row that decides
how to read the other three: refused there and T6/T6b/T6c say nothing about
nesting, because the argv or the environment never reached the sandbox. The
amended argv is deliberately NOT guessed into shape for 0.167.0 — nobody here
can run that version — so the probe reports what codex says rather than
asserting what it ought to accept.

Whether D21 is worth another host run at all was not this bead's call. It
was filed to the architecture lane with the reading (ranger-base-unsqg,
discovered-from ranger-base-bjk4k) and the note below is that lane's answer:
PARKED. One correction to this bead's own filing travels with it — the
"claude-only by operator ruling" in the run's comment is in no record, so
it was never a reason to retire anything, and this bead should not have
offered it as one.

*D21 is parked, not pending and not retired (ranger-base-unsqg,
2026-11-09).* The first host run read T6 refused (ranger-base-r5m98,
MEASURED: Linux 7.2, bubblewrap 0.13.1, codex 0.167.0), and that reading
says nothing about nesting. The probe counted every non-zero exit as
`refused`, it bound no writable `CODEX_HOME`, and its argv omits the
`--permission-profile` that `codex sandbox` requires on 0.153.4. That version
answers a usage error with exactly the three refused rows the host returned
(darwin, demonstrative only: not the host's version). ranger-base-bjk4k
amended the probe and added T6z, an unwrapped control that decides how to
read the other three.
- **Why parked.** D21 improves on darwin's posture; it is not a
  precondition for codex on Linux. With the flip off, Linux codex is walled
  exactly as darwin codex is, by its own sandbox alone, and that posture is
  already accepted. Nothing shipped waits on D21. The tripwire pin from
  ranger-base-bjk4k keeps the flip from landing by accident.
- **Why not retired.** The run's comment calls the host "claude-only by
  operator ruling". No ADR, bead or ORDERS file records that ruling
  (searched 2026-11-09), and D3 above, which is operator direction, says the
  opposite: Rex and herdr serve every runtime, and codex was admitted on Rex
  (ranger-base-8gh58). A design is not retired on a ruling the record does
  not contain. That conflict is the operator's to settle, and it is filed as
  one question.
- **What wakes it.** If codex is wanted on the D1 host, the amended T6/T6z
  run comes before the first codex crew there. It costs no turn. If the
  host is claude-only, D21 is retired, and so are D3's "every runtime" for
  that host and the bjk4k tripwire's instructions.

**D22. The probe for this design is `docs/adr/0080-bwrap-tier.probe.sh`.**
T1 is the D19 save under the shipped wall, T2-T5 are D20's git grants against
an outside pack, T6 is D21's nesting (retired with D21; its rows print
`absent` on a host without codex), and T7 is D18's `--args` fd. It writes
only under one mktemp dir and calls no model. D19 and D20 can be built and
pinned on darwin, because the renderer is pure. D18 is built and its SHELL
half — the redirection, the quoting of both paths, the `--` separator — is
measured on darwin against a stand-in binary that copies fd 3
(`bwraplaunchline_qa_test.go`); what T7 adds is the real binary's own
acceptance of the argv. D21's run is IN and T6 read refused against an
expected allowed (ranger-base-bjk4k): the reading, what it does not
establish, and the T6/T6z amendment this bead made to the probe are in D21's
as-measured note above.

**Alternatives rejected.**
- **A per-seat `CLAUDE_CONFIG_DIR`**, the bead's likely answer and the one I
  wanted, because it isolates seats from each other's config. On Linux the
  credential lives in that directory. N seats would then mean N credential
  copies, and if the refresh token rotates (ASSUMED) one seat's refresh logs
  the others out. It also splits D11's plan-usage key into N pollers on one
  rate limit, and it hides the shared transcripts, settings, plugins and
  skills. It buys nothing over what was measured: the shared dir already
  holds the siblings.
- **A new tier name** (`bwrap`, `namespace`). PIDs are shared across darwin
  and Linux homes. A rung that exists on only one OS would degrade every PID
  that moves, and the PID would carry a host fact.
- **Bind `~/.claude.json` as a file.** B8 refuses the atomic replace.
- **`$HOME` writable, read-only binds over its children.** This is
  enumerating leaves. A dotfile that does not exist yet (`~/.gitconfig`,
  `~/.bashrc`) is creatable, and it is the git config chain the deny exists
  for. It fails open.
- **An overlay or tmpfs on `$HOME`.** Config writes would vanish at exit,
  and a seat's trust and MCP state would silently diverge from the file
  posse seeds.
- **Granting `<common>` for `packed-refs.lock`.** It fails open, as above.
- **A narrower ref grant by renaming the fleet's branches** to
  `posse/<session>/head`, so that each session owns a directory. It is
  narrower, but every reader of `posse/<session>` changes. If D20's
  widening is ever measured being used, this is the exit.
- **`--unshare-pid`.** A seat must `kill` its own pids and `checkorphans`
  must read the box's `/proc`. **`--unshare-net`**: egress belongs to the
  container tier. **`--tmpfs /tmp`**: a seat's posse needs the multiplexer
  socket and the shared TMPDIR paths (`posse-test-signal.log`).
  **`--new-session`**: setsid detaches the TUI from its controlling terminal
  (ASSUMED to break ^C and job control). TIOCSTI injection, which it guards
  against, is off by default since kernel 6.2 (`dev.tty.legacy_tiocsti`,
  ASSUMED, not read on the host).
- **The argv inline on the line** instead of `--args`. It runs to hundreds
  of characters per seat, and the relaunch path would re-quote them.

**Consequences.**
- **A widening against seatbelt:** a bwrap seat can move a PEER session's
  `posse/*` ref (probe T3d). It still cannot move `main` or any top-level
  branch, or touch the shared hooks or config. That is narrower than codex
  and grok on darwin, which get `refs/heads` whole through `--add-dir`.
- **A credential mask is a snapshot at launch.** Another runtime's
  credential file created after the seat started is readable by it. Parity
  names each missing file. The seat still cannot WRITE there.
- The packed-refs residual of the container tier now applies to bwrap seats.
- **`git fsck` is no longer clean in a repo that has hosted a bwrap seat**
  (MEASURED, ranger-base-gi1gi): the keep file is an invalid refname holding
  no sha, so fsck exits 8 and names it twice. Every other git verb is clean,
  `gc` included, and nothing removes the file — it is one empty inode per ref
  directory, permanently. The exit, if the noise is ever measured costing
  anything, is a `.posse-keep` DIRECTORY rather than a file: a directory
  keeps the parent non-empty, is no ref-shaped entry at all, and the render
  refuses one today precisely because the letter asks for a file.
- Claude seats on the host need `CLAUDE_CONFIG_DIR` in the unit
  environment, which D6's one login already implies.
- Exit hatch unchanged: `bwrap` is one binary on the line, the args file is
  re-rendered each launch, and no state is held.

**Measured versus assumed.**
- MEASURED 2026-11-08, this laptop (darwin 25.6.0, claude 2.1.319, the host's
  version): `CLAUDE_CONFIG_DIR=<scratch> claude mcp add -s user …` under a
  `sandbox-exec` profile that allowed writes ONLY to the scratch dir and
  `/dev/` saved twice, exit 0, the entry in `<scratch>/.claude.json`, and the
  real `~/.claude.json` inode and mtime unchanged. The same with a trailing
  `deny file-write*` on `^<scratch>/\.claude\.json\..+` failed: exit 1, `MCP
  server "p2" was not saved`. So the siblings are named `.claude.json.<x>`
  inside the config dir and nowhere else. The CLI also READ the real
  `~/.claude/backups/` to print a restore hint for the fresh dir. That was a
  read, and nothing failed.
- ASSUMED, read by the probe: that the same build places siblings the same
  way on Linux (T1); whether a prune can remove a ref directory that is a
  live seat's MOUNTPOINT, which is what is left of D20's T4e once T4b-T4d
  were measured on darwin (above); the packed-refs residual (T5); codex
  nesting (T6); bwrap
  closing the `--args` fd, though a leaked fd would be read-only and hold
  nothing secret (T7c); claude's own `/sandbox` nesting in a real session,
  which no row tests without a turn.
- STATIC, this tree: the render inputs and call sites named above.

## Amendment: prep creates the session's own furniture, nothing else (ranger-base-sg7a7, 2026-11-09)

**Context.** D17/D18's prep shape 1 exists for a real reason — bwrap binds
only a source that exists, where sandbox-exec granted paths that did not — so
a write grant that is not there becomes a `mkdir -p` outside the namespace, in
the seat's user. It landed with no perimeter, and two things that cost nothing
at the other mechanism became host writes. Both were found by driving a launch
at this mechanism from darwin (ranger-base-temhf's `wallMechanismHost`), and
neither was reported by anything:

1. **An absolute `writable:` in a PID was a directory posse created.** One
   line — `writable: [scratch, /opt/shared]` in `parity_test`'s fixture — was
   a clean launch on darwin (SBPL grants a path whether or not it is there and
   nothing writes it), a REFUSED launch on this laptop
   (`mkdir /opt/shared: permission denied`, MEASURED with the mechanism
   pinned) and a new host directory on ci.yml's ubuntu runner, created
   silently on every red run.
2. **A launch materialized `<cwd>/.beads`, which moved a LATER launch's parity
   row.** The store of record is in the writable set, so prep made it; ADR
   0013 §4's reachability row probes only a store that EXISTS. In
   `TestSeatbeltProfileAndLaunch` a grok seatbelt launch therefore handed a
   codex seat three arms later — same tree, `-s read-only` — a real store
   behind a read-only sandbox where it had none before, so the row went from
   "nothing to reach" to unrealized and REFUSED that launch. Same tree, same
   PIDs, a different answer depending on which seat launched there first.

**D23. The launcher creates the session's own furniture and nothing else.**
Prep's authority is posse's own perimeter: the session tree, the gates dir,
posse's state, the persona's memory dir, the launching runtime's declared
state dirs, the generic caches, `TMPDIR`. Two classes of write grant are
outside it and are **bound where the host has them, render no mount where it
does not, and are never created**:

- **a `writable:` extra that RESOLVES OUTSIDE THE SESSION TREE** (`~` counts
  as outside, for `homeCacheGrants`' reason one list over). It is a host path
  the operator declared, not one posse derived, and nothing inside the session
  can bring it into being either. An extra that resolves INSIDE the tree keeps
  its prep: posse made that tree and retires it with the session, and for a
  persona that denies file writes cwd itself is not granted, so without the
  mkdir the grant would be dead.

  **The question is LOCATION, and until ranger-base-hoh3q the code asked a
  SPELLING** (`pidWritableIsHostPath` was `filepath.IsAbs(ExpandTilde(w))`,
  found by the verify ranger-base-41g0y). `pidWritableExtras` resolves a
  relative entry with `filepath.Join(cwd, w)`, which CLEANS `..`, so
  `writable: ["../../shared"]` was a path outside the session tree that this
  decision classed as relative — and prep therefore created it, the render
  bound it, the parity row filed nothing and `posse agent check` warned about
  nothing, all four at once (MEASURED 2026-11-09 at
  `TestQABwrapPrepDoesNotCreateAPathOutsideTheSessionTree`). At the real
  spelling cwd is `~/.posse/worktrees/posse/<session>`, so `../shared` is a
  junk directory in the operator's worktrees dir that no session retires —
  `bwrapNoCreate`'s own stated reason for counting `~` as outside — and enough
  `..` reaches `/opt/shared` itself, the path this decision was written about.
  It asks `underDir(cwd, filepath.Join(cwd, w))` now, so a `..` that resolves
  back inside the tree (`scratch/../scratch`) stays session furniture and an
  entry under a symlink that leaves the tree does not — the latter is the half
  a string test cannot reach at all, and it needs no `..` and no absolute path
  in the PID. Where there is no session dir to resolve against — `posse agent
  check`, which stats nothing and answers the same on every box — the string
  is the whole of what is known and `filepath.IsLocal` is that half: one rule,
  two vocabularies, the cwd-less arm strictly the weaker one.
- **the store of record.** Its EXISTENCE is itself read as a fact — ADR 0013
  §4's row says "bd creates it on first write" in its own words, and prep made
  that sentence false. An empty `.beads` is not inert either: in no-db mode bd
  resolves `$BEADS_DIR` else `$cwd/.beads` and reads no redirect at all (ADR
  0055), so a directory posse makes there is a second store of record that can
  take writes nothing reads back. `bd init` is denied to every persona because
  creating a store is a decision; it is not the launcher's either.

Each absence is said out loud at a surface read BEFORE the launch, because a
grant that silently disappears is the wall-with-a-note shape this tier
refuses:

- `posse gates` and the readable `bwrap.txt` print it with `Seal`'s marker —
  the one that already means "this entry is in a list and renders no mount" —
  below the prep list, so the two together are the whole of what the launcher
  did and did not do on the host.
- parity carries the PID half as a **DeclaredDifference, never degradation**
  (ADR 0017 §2), for `applyBwrapRefGrant`'s reason in its own words: nothing
  here is a wall posse asked for and did not get. It is a grant this mechanism
  renders narrower than darwin's, what it costs is a function the seat loses
  loudly — a write into that path is refused by the base `--ro-bind / /` — and
  degrading it would put a safe-by-design narrowing behind `--allow-degraded`
  and refuse dispatch for a path that is nobody's gate (ranger-base-d17a).
- `posse agent check` warns on a `writable:` outside the session tree, which
  is where the operator writing that PID is standing. A warning and
  box-INDEPENDENT: the PID is coherent and may well launch on a host that has
  the path, and a lint that stats would answer differently per box where the
  launch row already answers for the box it is on. Two sentences since
  ranger-base-hoh3q, because the remedy differs — an absolute path gets
  created outside posse, a `..` gets re-spelled — and because the one sentence
  it used to print ended "A path relative to the session dir is created with
  the session tree", which was the defect stated to the operator as a rule.
- the store half gets no second row: ADR 0013 §4's no-store row is already the
  reader for "there is no store at this launch dir", and it gains one clause
  naming the launcher at this mechanism only.

*As built (ranger-base-sg7a7, 2026-11-09).* `bwrapNoCreate` and
`bwrapBindExisting` in `internal/posse/bwrap.go`, applied in `bwrapRender`
after D20's substitution and before `BwrapArgs` — so the renderer never learns
this question exists, there is no sixth arm in it to get wrong, and its
signature does not grow a parameter at thirty call sites. That is D20's own
move (`bwrapGitSwap`), and the producer-says rule D19, D20 and
`SeatbeltCarveOut.DenyFile` all follow. Four things the text did not settle:

- **The store has ONE spelling that can be absent**, and it is `<cwd>/.beads`.
  `SeatbeltWritable` grants the store under two names — that one, and
  `beadsHome`'s resolved target where a redirect moves it out of the tree (ADR
  0012 D3-C) — but a redirect bd FOLLOWS is one whose target
  `beadsRedirectHop` has already stat'd as a directory. The resolved target is
  therefore there by construction, and an entry for it would be a control with
  no failing arm.
- **A FILE at a no-create grant stays in the set**, so B8's refusal is still
  `BwrapArgs`' and still names the path and the remedy. Answering it in the
  drop would be a second refusal for one fact, and a quieter one.
- **`SeatbeltWritable` is untouched**, the rule D19 and D20 both keep:
  narrowing the shared producer would take these entries out of the darwin
  profile, where a grant for an absent path costs nothing and the session can
  still create it.
- **Stated residual, not closed.** Where a redirect names a store in a
  directory that is not a git repository, `SeatbeltWritable` grants
  `<root>/.git` and prep still makes it. That path is not in the list because
  in the common case `<root>` IS the session tree, where `.git` is the
  session's own and prep creating it is exactly the furniture this decision
  keeps. The cost is a `.git` directory in a non-repo, which the next `git` run
  there reports; closing it needs a producer that knows which of the two it is
  naming, the way D20 answered the ref pair one step earlier.

**Pins** (`internal/posse/bwrapprepauthority_qa_test.go`, arm 1, every arm on
darwin): the launcher creates neither member and binds both where the host has
them; an extra that resolves inside the tree is still created, which is the
control that keeps the fix from being "drop every relative grant"; a relative
extra that CLIMBS OUT is created by nobody and reported at all four surfaces,
and an entry under a symlink that leaves the tree is the arm that holds the
predicate to a path rather than a spelling (ranger-base-hoh3q); a launch in a
tree with no store
leaves ADR 0013 §4's row byte-identical for the next launch in the same tree,
measured before and after rather than asserted; the declared difference
reaches a real matrix and degrades nothing; `bwrap.txt` names what was not
created and `bwrap.args` does not name it at all; `agent check` warns and does
not fail. `TestSeatbeltProfileAndLaunch` keeps `/opt/shared` in its fixture on
purpose — ci.yml's ubuntu arms are the only reader on the planet that runs
that test at this mechanism, so the declaration is now the live witness on the
exact line that was creating the directory.

*Measured versus assumed.* MEASURED 2026-11-09, darwin/arm64, with
`wallMechanismHost` pinned to `linux` and `bwrapBin` stubbed: before the fix
the launch creates both members and the reachability row moves; after it,
neither. Five mutations, each red in the pin whose name is the behaviour: the
render ignoring the producer (3 red), the store dropped from the list (3), the
host-path predicate answering false (6), the no-store clause silenced (2), and
the drop stopping asking whether the path is there (3) — that last one is the
other direction the fix can be wrong, and it reds the two controls and B8's
refusal. ASSUMED, unchanged by this bead: that a Linux render is this render,
which it is not — `SeatbeltWritable` still reads `runtime.GOOS` for the
home-cache grants, so which PATHS the wall covers is `homeCacheGrants`' own
pin and what the launcher MAKES is this one.

## Amendment: the D1 host is claude-only; D21 is retired (ranger-base-r2fz9, 2026-11-09)

*Operator ruling, ranger-base-0ka8a option (a), 2026-11-09. First said
2026-11-08 in answer to ADR 0083's prep questions ("claude for now",
ranger-base-9nxdt) and restated when the operator declined ranger-base-uzb3s.*

**Context.** D21's parked note left one question open: is the D1 host
claude-only? The probe-run comment said so, and no record did. The operator
has now answered on the record, so the conflict with D3 is settled.

**D24. The D1 host runs claude crews only.** The reason is D6: the operator has
not read OpenAI's and xAI's terms for a plan login on a server. codex, grok and
bob crews stay on the laptop. Two things are unchanged:
- D3 as a property of posse: herdr and Rex are interchangeable, and the codex
  line on Rex (ranger-base-8gh58) stays built and tested. Only what the D1
  host RUNS is narrowed.
- How to reopen: ask the operator. Do not infer it from a new runtime version
  or a probe result. The ruling says "for now", and its condition is the D6
  terms read, not a measurement.

**D25. D21 is retired, and so is the parity change it would have made.** With
no codex seat on the host, walling codex under bwrap protects nothing.
- `wallRendered` keeps `!rt.SelfSandbox` on both mechanisms. That is the code
  as it stands, so nothing is reverted.
- parity's "cannot wrap codex" declared difference (`seatbeltIncompatible`,
  parity.go) stays mechanism-neutral. The darwin-only version D21 described
  is never built.
- reachability.go's SelfSandbox arm stays where it is.
- The T6 rows in `0080-bwrap-tier.probe.sh` stay. They print `absent` on a host
  without codex. If codex is ever admitted, the amended T6/T6z run is the first
  step, and ranger-base-1jhgv already describes it.

**D26. The bjk4k pin stays, without its instructions.**
`TestQANoPosseWallForASelfSandboxRuntimeOnEitherMechanism` was written as a
tripwire "meant to be deleted by the commit that lands D21". That commit will
not happen now, so the deletion instructions are wrong and are removed. The
pin itself keeps its job.

What the pin protects: a commit that adds a host term (the mechanism, or the
GOOS) to `wallRendered`. On darwin that commit passes every behavioral pin, and
it gives codex on any Linux host an L2 wall that posse never measured. posse is
shipped software, so "any Linux host" is not only the D1 host. If the pin is
deleted, that commit lands green.

The pin's message now says three things: D21 is retired by operator ruling;
reopening it is the operator's call; and a reopen starts with the T6/T6z run.
It also drops the "land it WITH its reading … this pin is meant to be deleted"
list. Both arms and the allow-list are unchanged. The code lane makes the
change (cut below), together with the matching comment in seatbelt.go
(`wallRendered`'s head: "D21's as-measured note carries both halves").

**Alternatives rejected.**
- **Delete the pin with its instructions.** The bead's wording allows this
  reading. It is rejected because the hole the pin closes (a flip that darwin
  cannot see) does not depend on D21 being pending. It depends on the
  predicate reading no host term, and that is still the design.
- **Keep D21 parked.** That was option (b), and the operator chose (a). Parked
  would also keep a pending-looking item in an ADR whose host will never run
  it.
- **Remove the T6 rows from the probe.** They cost nothing on a claude-only
  host and they are the reopen run. Removing them would turn a reopen into a
  rewrite.

**Measured versus assumed.** Nothing here is newly MEASURED; this is a ruling
applied to text. The T6 reading (refused, uninterpretable) stands as D21's
notes record it. ASSUMED: that no codex binary is installed on the D1 host, so
T6 prints `absent`. If one is installed, the rows run and their result is
ignored, because no decision reads them.

**Beads.** ranger-base-r2fz9 (this). The code slice is the one cut from this
bead for dinesh: the pin message and the seatbelt.go comment.

## Amendment: the bwrap wall hides the user runtime dir (ranger-base-ludpp, 2026-11-11)

**Context.** D17's base is `--ro-bind / /` with no network namespace. A
read-only mount stops writes. It does not stop connect(2) on a unix socket,
and it does not stop a write-open of a FIFO: the kernel's read-only check
covers neither. The user runtime dir (`$XDG_RUNTIME_DIR`, normally
`/run/user/<uid>`) is where a desktop session keeps exactly those endpoints.
Several of them act outside the cage when a client connects: the session
bus and the user service manager, the multiplexer's server, the gpg and ssh
agents, the secret store, and other seats' sockets. ranger-base-x9f3s
measured all of them reachable from a live seat. That bead holds the census.

**Decision.**
1. **An empty tmpfs over the runtime dir**, rendered right after the base
   and before every write grant. It is an allowlist, not a denylist: an
   endpoint the host adds later is hidden without anyone naming it. A seat
   gets something back only through a write grant, which mounts later and
   so lands on top of the tmpfs. Today nothing is granted back. MEASURED for
   this bead: the Rex reporter writes its OSC 7501 records to `/dev/tty` and
   opens no socket. Claude's own messaging socket under `cc-socks/` could
   never be created under the read-only base. On the tmpfs it can be, and it
   stays private to the seat.
2. **Both spellings are hidden.** `$XDG_RUNTIME_DIR` is the one the seat's
   programs read. `/run/user/<uid>` is hidden as well, because a launcher
   started outside the desktop session may carry no XDG variable at all.
   Both are resolved, and on a normal host they render as one mount. The
   producer is `bwrapRuntimeDirs`. It fills `SeatbeltCarveOut.Hide`, a field
   only `bwrapRender` sets, the same way `Seed` is bwrap-only. It refuses a
   relative value, `/`, a top-level directory, or a directory that holds the
   home. An empty tmpfs over any of those leaves a seat with no tree.
3. **A deny or Keep inside a hidden dir that no grant re-opened renders no
   mount.** Both are binds from the host, and a read-only bind of a socket is
   still a socket a seat can connect to.
4. **`--unsetenv DBUS_SESSION_BUS_ADDRESS SSH_AUTH_SOCK GPG_AGENT_INFO`**,
   unconditionally. An address can name an endpoint outside the hidden dir,
   such as an abstract socket or an agent socket under `/tmp`.

D17 item 1 now reads: base, then the unsets, then `--tmpfs` for each hidden
dir, then the grants.

**Residuals, stated rather than closed.**
- **Abstract unix sockets** are not files, so a tmpfs cannot hide them. Only
  `--unshare-net` closes them, and D17 rejected that flag because egress
  belongs to the container tier. ranger-base-x9f3s found no listening
  abstract socket reachable on the D1 host. That is a census of one host on
  one date, not a guarantee.
- **System-scope sockets** (`/run/systemd/*`, `/run/<daemon>`) give a seat
  the rights the user already has outside the cage. They are out of scope
  here.
- **The multiplexer socket under `/tmp`** stays reachable, as D17's
  Alternatives require.
- **Darwin.** The seatbelt profile is `(allow default)` with only file writes
  denied, so unix-socket connects and launchd lookups are not walled. Closing
  that needs the darwin box. It is its own bead.

**Measured.** 2026-11-11, D1 host, bubblewrap 0.13.1, Linux 7.2.15.
Connect-only probes, nothing sent. Under the old base, `bus`,
`systemd/private`, `rex/server.sock`, `gnupg/S.gpg-agent.ssh` and
`keyring/control` all connected, the uwsm FIFO was present, and
`DBUS_SESSION_BUS_ADDRESS` reached the seat. Under the new base the runtime
dir is empty and the variable is unset. The pins are in
`internal/posse/bwraphide_qa_test.go`. The live pin builds its own runtime
dir with listening sockets and a FIFO, and it carries a control arm that
reaches them without `Hide`.

**Beads.** ranger-base-ludpp (this), from ranger-base-x9f3s (the finding).
