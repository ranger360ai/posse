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
    *Amended 2026-11-14 by [ADR 0096](0096-nesting-is-not-a-property-of-the-bwrap-seat-wall.md)
    (ranger-base-rwet4): B7 was measured on the probe's `$BW`, which carries
    `--proc /proc`. D17's render does not carry it, and nothing nests inside
    a shipped seat. Nesting is not a property of this tier.*
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
- **A credential mask was a snapshot at launch**, and for the three SIBLING
  runtimes it is not one any more (2026-11-13). A /dev/null bind is a mount
  over a NAME, so a credential file created after the seat started got no
  mount at all and was readable by it, with nothing for `posse wall-watch`
  to see detach; a refresh by atomic rename detached one that HAD been
  rendered. Step 5c masks each sibling credential store as a DIRECTORY
  instead — an empty tmpfs, made if absent, with the three members posse's
  own in-seat readers ask for bound back read-only
  (`credentialStoreAllowlist`) — so a later login writes inside the mask and
  a refresh inside it is not a mount event. A store a write grant
  reaches is not masked at all, because an empty tmpfs over a grant loses
  the seat's writes — the breach there is the grant. The report names every
  one of these states, the ones that render no mount with the `·` marker.
  The seat still cannot WRITE at any of them. The bead that landed this, and
  its open residual, are:

  - ranger-base-a8qfg
  - ranger-base-1c0r6
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

## Amendment: the bwrap wall gives each seat its own /tmp (ranger-base-7wdb7, 2026-11-11)

**Context.** D17's write set granted the host's `/tmp` to every seat, and
D17's Alternatives rejected `--tmpfs /tmp` because a seat needed the
multiplexer socket and the shared TMPDIR paths there. `/tmp` is also where
uncaged processes at the same uid stage content they then run. An uncaged
`go build` compiles and links in `/tmp/go-build<N>`, and node keeps its
compile cache in `/tmp/node-compile-cache`. A seat that can write `/tmp` can
reach those. This is the same class as ranger-base-olr0g's shared caches,
over a different directory. Since ludpp, the multiplexer's socket is in the
user runtime dir, which the wall hides. MEASURED on the D1 host for this
bead: no socket under `/tmp` is one a seat needs. The ones there are X11,
ICE and a browser's, and a seat should not reach any of them.

**Decision.**
1. **Each seat gets its own directory bound over `/tmp`**:
   `<SeatCacheDir>/tmp` (`SeatTmpDir`), rendered after the hides and before
   the grants. The host's `/tmp` is then not in the seat's view at all,
   neither to write nor to read. It is a bind, not a tmpfs, so a dead
   seat's temp files, including `posse-test-signal.log`, stay on the host
   for the operator to read. It sits under the seat cache dir, so the
   worktree remove that takes the cache takes it too.
2. **The grant of `/tmp` itself is dropped**, and so is a launcher `TMPDIR`
   under `/tmp` (`bwrapDropTmpGrants`). A host path under `/tmp`, bound back
   over the seat's `/tmp`, would be the shared directory again. The seat
   gets `TMPDIR=/tmp` instead, which is its own. A PID `writable:` extra
   under `/tmp` is a named directory and is still bound.
3. **`/tmp` joins the hidden list for the rest of the render.** A deny or
   Keep under `/tmp` that no grant re-opened renders nothing, the same as
   inside the runtime dir.
4. **It stands down when the home or the session dir is under `/tmp`**
   (`bwrapSeatTmp`). A private `/tmp` would hide the seat's own tree, and a
   home under `/tmp` is in the shared directory already. No production
   launch has that shape. This package's wall fixtures do, so every
   existing pin renders the wall it did before, and the new pins pass the
   mount directly.

`--tmpfs /tmp` stays rejected, for the reason in point 1. The multiplexer
half of that rejection no longer holds on this host.

**Measured.** 2026-11-11, D1 host, bubblewrap 0.13.1, Linux 7.2.15.
- The live pin puts a file in the host's `/tmp`, shaped like a build's WORK
  object. Under the new wall the seat sees it absent and cannot overwrite
  it. The seat's own write to `/tmp` lands in its own dir and not on the
  host. The control arm grants `/tmp` as before, and there the same probe
  reads the plant and overwrites it.
- `bwrapRender` was run once end to end with HOME and the session dir
  outside `/tmp` and the launcher's `TMPDIR` under it. The argv binds the
  seat dir at `/tmp` and sets `TMPDIR=/tmp`. The set holds nothing under
  `/tmp`. Inside, `/tmp` starts empty and is writable, and the host's
  `/tmp/claude-<uid>` is not there.
- A user namespace does not reach the host's `/tmp` through
  `/proc/<host pid>/root` or `/cwd` either: both are refused (EACCES), so
  the mount cannot be walked around through `/proc`.

The pins are in `internal/posse/bwraptmp_qa_test.go`.

**Residuals, stated rather than closed.**
- **Darwin.** The seatbelt profile still grants `/private/tmp`. Closing
  that needs the darwin box, and it is its own bead.
- **Uncaged processes still share `/tmp` with each other.** This amendment
  takes walled seats out of it. It does not change where the operator's
  own builds stage their work.

**Beads.** ranger-base-7wdb7 (this), from ranger-base-olr0g.

## Amendment: the seat gets a socket wall, and B7 was never the shipped wall's property (2026-11-13)

*This amendment's bead, and the finding it answers, are `vuln-private`. They
are named by bare id in the Beads block below and nowhere else, which is
AGENTS.md "Landing the plane" — a hole still open is named by its id and
nothing else. What is described here is what this change SHUTS, which the
same ruling allows and which is why the code looks the way it does.*

**Context.** `--ro-bind / /` does not stop `connect(2)`: the kernel's
read-only check does not cover a socket, which D17's own render already says
(`bwrapArgsInto` 1b, and the reason the user runtime dir gets a tmpfs —
ranger-base-ludpp). What that tmpfs cannot reach is a socket inside a **write
grant**, which the seat must be able to write and therefore cannot have
emptied. Both halves of the mount answer fail there: a `--ro-bind /dev/null`
over a live socket is detached by the host's own unlink and re-bind, and a
socket absent at launch has nothing to bind over at all. So a process in a
caged seat could reach a host daemon listening on such a path and hand it
work, whatever the seat's runtime.

D13 rejected Landlock as the Linux cage TIER and every word of that holds:
Landlock is allow-only, so the trailing deny under a grant — the whole of
what the wall defends — has no Landlock spelling. What D13's "Alternatives
rejected" left open was the LAYER: "bwrap plus a Landlock layer for the
scopes bwrap lacks ... A scope-only ruleset may not block [the nested
bwrap], but that is ASSUMED and unread." ABI 9 added
`LANDLOCK_ACCESS_FS_RESOLVE_UNIX`, which refuses exactly the resolution no
mount can, so that line is what this amendment reads.

**Findings.** MEASURED 2026-11-13 on the D1 host (Linux 7.2.15-1-omarchy,
bubblewrap 0.13.1, Landlock ABI 10, linux-api-headers 7.3-1) by
`docs/adr/0080-resolve-unix.probe.sh`, 14 rows, every one as its EXPECT.

- **The right does what the fix needs.** Handled with no allow rule,
  `connect(2)` is EACCES to a socket bound before `restrict_self` (U1) **and
  to one an unrestricted process binds after** (U2) — the case no mount
  covers; `sendmsg(2)` with an explicit address likewise (U4); the domain
  survives `execve` (U5); a socket created inside the domain still connects
  (U3); and ordinary file read, write, mkdir and rename are untouched (U11),
  which is what "the bwrap wall stays the file wall" means in practice.
- **An allow rule, if one is ever needed, names a DIRECTORY and never a
  socket** (U12, U13). A rule on the socket FILE is keyed on the inode it was
  added for, so the host's unlink-and-re-bind — the same move that defeats a
  mount over a socket — leaves it covering nothing (EACCES: a daemon restart
  would silently cut the seat off). A DIRECTORY rule covers a socket bound in
  it afterwards, and is also the only way to allow one that is ABSENT at
  launch, since an `O_PATH` open of it would be ENOENT and there would be no
  rule to add.
- **It costs the nested mount namespace** (U9, U10). A domain handling any
  filesystem right — this one included — forbids `mount(2)`, so a nested
  bwrap cannot start inside it: `bwrap: Failed to make / slave: Operation not
  permitted`. That is D14's probe B7, "a runtime's own bwrap must still nest
  inside ours", which D13 names as the property Landlock fails. It fails here
  too. RESOLVE_UNIX is not a scope — it is `handled_access_fs` — and the
  topology hooks read the PRESENCE of a filesystem domain, not which right it
  handles.
- **And that cost is already paid, because B7 does not hold for the wall
  posse ships.** MEASURED the same day against a rendered `bwrap.args` and
  against the installed wall of a live caged seat, with no Landlock anywhere:
  a nested bwrap inside the real wall fails with `bwrap: setting up uid map:
  Read-only file system`, and `unshare -rm` with `cannot open
  /proc/self/uid_map: Read-only file system`. The cause is in D17's own text:
  `--proc` is **deliberately absent** from the render, so `/proc` inside a
  seat is the host's, bound read-only by `--ro-bind / /`, and a nested user
  namespace cannot write its own uid map — long before it reaches a mount. B7
  was measured on `0080-linux-cage.probe.sh`'s `$BW`, which carries `--proc
  /proc`. That is not the wall `bwrapArgsInto` renders.
  The same reading has a consequence nobody had written down: inside a real
  caged seat `TestLiveBwrapGitRefsCommitsOnTheSessionBranch` FAILS, and
  `TestLiveWallClaudeProjectsRefusesPeerPlants` and
  `TestLiveWallStrictMCPStartsOnlyTheLineServers` (which was
  `TestLiveStrictMCPStartsOnlyTheLineServersUnderTheRenderedWall` when this
  amendment was written — ranger-base-q749c renamed it into
  [ADR 0097](0097-the-walls-live-pins-measure-in-a-nesting-venue-after-each-landing.md)
  D4's filter) go to `SKIP: NOTHING MEASURED` — so three of the wall's own
  live pins cannot measure the wall from inside it, and one of them reds
  `make test` there.
  That is ranger-base-rwet4, for richard, and not this amendment's to decide:
  nothing below depends on how it goes, because this layer adds no refusal
  the shipped wall does not already impose.
- **The census, so the allow list can be empty and say so.** MEASURED
  2026-11-13 on D1 over every host pathname listener. Every socket a seat
  could otherwise resolve is either inside the runtime dir the render already
  hides (the session bus, the gpg and ssh agents, the keyring, p11-kit,
  pulse, pipewire, the user systemd, rex's own `server.sock` and
  `login-server.sock`, and the other seats' `cc-socks/*.sock`) or belongs to
  the system and is of no use to a seat (`/run/systemd/*`, `/run/dbus`,
  `/run/udev`, `/run/polkit`, `/run/cups`, `/run/avahi-daemon`,
  `/run/tailscale`, `/run/wpa_supplicant`, `/run/ssh-unix-local`,
  `/etc/pacman.d/gnupg`). **No socket exists anywhere under the home.**
  ranger-base-ludpp had already measured the same answer from the other side:
  what a seat needs bound back out of the hidden runtime dir is nothing. The
  one host socket posse ever hands a session is herdr's, through the
  container tier's opt-in `sockets:` (`CageSocketMounts`), which no shipped
  PID declares.

**Decision.**
- **D27. A bwrap seat runs behind a Landlock domain that handles
  RESOLVE_UNIX and no other right, applied inside the cage.** *(Amended
  2026-11-15, ranger-base-p5i2c: it handles REFER as well and grants it on
  `/`, which is the only spelling Landlock has for leaving rename(2) and
  link(2) alone. "No other right" was not achievable as written — see the
  amendment "the socket wall's domain must hand REFER back" below.)* The file
  wall stays the bwrap wall, byte for byte. The layer is one word on the launch
  line — `posse wall-unix --` — typed after bwrap's own `--`, so it wraps
  exactly the runtime command bwrap would otherwise have exec'd, and the
  domain therefore covers the runtime and everything the seat forks from it.
- **D28. No allow rule, on the census above.** If one is ever needed it names
  a DIRECTORY, on U12/U13. A per-socket allow is forbidden rather than
  discouraged: it is defeated by the very move this class of bug is about.
  *(Amended 2026-11-15, ranger-base-p5i2c: no allow rule for RESOLVE_UNIX,
  which is what this decision is about and what still stands. There is one
  rule, and it grants REFER on `/` — a directory, as this decision requires,
  and the root because the grant has to reach everywhere the seat can already
  write. It confers no access: see the amendment below.)*
- **D29. The step applies the domain and then BECOMES the runtime**
  (`syscall.Exec`), so the seat's pid, its controlling terminal and its exit
  status are the runtime's own, exactly as they are when bwrap execs the
  runtime itself. It resolves the runtime on the seat's own PATH
  (`exec.LookPath`, gates dir first) and not gates-free: the L1 shims are
  aimed at the runtime process itself, and resolving past them would take the
  shell-verb denies off the one process they most have to cover.
- **D30. It refuses rather than execs when the domain cannot be applied.**
  The render only types the step on a host whose ABI it has already read as
  sufficient, so a failure there is a wall posse reported and does not have —
  D4's rule. A refusal costs one seat; a silent exec would put every host
  socket back in reach with the wall report still saying the layer is up.
- **D31. Below ABI 9 the step is not rendered, and `posse gates` says
  DEGRADED — never silence.** `WallUnixDegraded` carries the sentence, and
  the report prints both arms, because an operator reading a wall has to be
  able to tell the half that is up from the half that is not.
- **D32. The step is on the LAUNCH spelling only, not on `Prefix`.** That is
  the rule the wall-watch sidecar already follows and for its reason:
  `Launch` is the line a seat is opened with (`WallPrefix` prefers it), and
  `Prefix` is what the reachability and witness PROBES type — "probes type
  it, and they run no watcher". Those probes' subject is the FILE wall, which
  this layer does not touch, so a step on their line buys nothing and costs
  something real: a second binary to resolve, and so a new way for the probe
  to abstain. It cost exactly that when it was first written onto both — the
  witness read `NOT WITNESSED … never printed its marker`, which under ADR
  0086 D5 is an abstention and not a pass.
- **D33. The probe is `docs/adr/0080-resolve-unix.probe.sh`**, and its U9/U10
  EXPECT cells say `refused`: they record what the kernel does, not what the
  design wanted. Each prints a detail line with the stderr the one-word cell
  drops, which is B5's lesson (ranger-base-yca4g) and D21's T6's
  (ranger-base-bjk4k) — a cell that throws the stream away cannot tell a wall
  refusing a thing from a tool that never started.

**Alternatives rejected.**
- **A `--tmpfs` over the write grant.** It is the right answer for every
  directory a seat can afford to have emptied, and it holds across the
  unlink-and-re-bind (MEASURED: the probe's N4 row reads ENOENT) — which is
  why ranger-base-ludpp's hide stays exactly as it is and this layer is only
  for the residual. Over a write grant it would empty the grant.
- **An enumerated deny of the known host sockets.** Default-allow by
  omission: the directory nobody listed is the hole, and the whole point here
  is a socket that was not there when the list was written.
- **`--unshare-net`.** Pathname sockets resolve through the filesystem and
  not the net namespace (the bind-mounted `docker.sock` pattern). D17 rejects
  it for its own reasons as well.
- **seccomp.** It cannot read a connect path.
- **An operator rule that no such daemon runs beside caged seats.** A
  detective control, and it covers one daemon out of the class.

**Measured versus assumed.**
- MEASURED 2026-11-13, D1 host, as above: the probe's 14 rows; the nested
  bwrap failing inside a real rendered wall and inside a live caged seat's
  installed wall; the socket census; and, end to end behind the step on
  D17's base, `claude --version` and `git --version` starting, argv and exit
  status passing through, and a host socket reading EACCES inside the started
  runtime while the control arm connects.
- MEASURED, this tree: the render carries the step on the launch line, in
  that one position, and not on the probe line or on a seatbelt line
  (`internal/posse/wallunixrender_qa_test.go`); and the live pin — an uncaged
  process binding a socket after the seat launched, against a control arm
  that connects (`internal/posse/wallunixlive_test.go`).
- ASSUMED, still: that no runtime posse launches needs a host pathname socket
  it has not declared. The census is of the HOST's listeners and of what
  ranger-base-ludpp measured a seat asking for; it is not a reading of every
  runtime's future. A runtime that grows one fails visibly — EACCES at
  connect, and `posse gates` names the layer — and the answer is a directory
  rule under D28, never a socket.
- Out of scope: the darwin half, which is unmeasured and unrendered. Its
  bead is in the block below.

**Two bugs this change's own pins caught**, both in the arm that exists to
refuse, and both kept here because the next person to write a wrapper like
this will write them again:
1. **The step defaulted its command to its own argv** when the line carried
   no `--`. The arm meant to refuse an empty line instead restricted the
   calling process and exec'd `posse` — it took down the test binary. The
   command is now only what follows `--`, and nothing when there is no `--`.
2. **The live pin's seat inherited `RHQ_FAKE_HERDR`.** The seat is the test
   binary re-exec'd, and internal/posse's `TestMain` turns any posse test
   binary carrying that marker into a fake bd: both arms answered `fake bd:
   unhandled -test.run=…` and exited 1, so the control arm would have
   reported a connect that never happened. Shed for the child now — the same
   cause `docs/notes.d/ranger-base-4cmqw.md`'s neighbours ran into, and the
   reason `landedchecklive_test.go` sheds it too.

**Beads.**

- ranger-base-6rndn
- ranger-base-l2gk9
- ranger-base-q9bqu
- ranger-base-rwet4

## Amendment: the socket wall's domain also scopes signals (2026-11-15)

*Same convention as the amendment above: this amendment's bead, and the
finding it answers, are `vuln-private` and are named by bare id in the Beads
block below and nowhere else. What is described here is what the landed
change SHUTS.*

**Context.** D17 keeps the host pid namespace on purpose, and its
Alternatives say why: a seat must `kill` its own pids and `checkorphans` must
read the box's `/proc`. What that left unwritten is that sharing the pid
namespace also shared signal REACH. Without a pid namespace, a process in a
seat could send any signal, signal 0 included, to any same-uid process on the
host: the launcher, every other seat, and every operator process. The only
cover was the PID-level `pkill`/`killall` deny, a shim on two verbs that a
plain `kill <pid>`, or any runtime calling `kill(2)` itself, never meets.
D13's Alternatives had already named the remedy ("bwrap plus a Landlock
layer ... the scoping would make a cross-seat kill impossible rather than
merely denied by the PID") and parked it on "a filesystem domain blocks the
nested bwrap". D27 then put a Landlock domain on every seat, and ADR 0096 D1
ruled that nesting is not a property of the seat wall. So the reason for
parking it is gone, and the layer to carry it already exists.

**Findings.** MEASURED 2026-11-14 from inside a live bwrap seat on D1
(Linux 7.2.15-1-omarchy, Landlock ABI 10), Python ctypes, a domain with
`handled_access_fs = 0` and `scoped = SIGNAL`, signal 0 only, to pids the
probe did not start:
- Before `restrict_self`: `kill -0` of the parent shell and of a child
  forked before the restriction both succeed.
- After it: both are EPERM, and SIGTERM to the pre-restriction child is
  EPERM. `kill -0` of itself succeeds.
- Inside the domain: a child can SIGTERM its own grandchild and `kill -0`
  its restricted parent. An exec'd `sh` can `kill -TERM $!` its own
  background job and `kill -0 $PPID`. The domain survives fork and execve.
- The errno still says whether the pid exists: a live pid outside the domain
  gives EPERM, an absent pid gives ESRCH. A shell's `kill -0` exits 1 for
  both, so only the errno carries liveness.
- `/proc` inside the seat is `ro,nosuid,nodev,noexec`, and `W_OK` on
  `/proc/self/coredump_filter` is false. The knobs ADR 0096 D2 keeps closed
  are still closed from a live seat. This amendment opens none of them.

MEASURED 2026-11-15, this tree, on D1:
`TestLiveWallUnixScopesSignalsToTheSeat` ran (it did not skip) and passed
all three arms. Behind the rendered step, a host pid is EPERM while the
control seat without the step reaches it, an absent pid is ESRCH, the seat
can SIGTERM its own fork+exec'd child, and the seat still dies with its
parent. `TestQAWallUnixRulesetScopesSignalAtEveryABITheStepIsTypedFor`
passed. The landing commit records that dropping `Scoped` from the plan
reds both pins, and dropping it from the syscall attr reds the live pin.

**Decision.**
- **D34. The wall-unix domain sets `Scoped = LANDLOCK_SCOPE_SIGNAL`.** It
  goes in the same ruleset D27 applies, in the same step and at the same
  moment, before `syscall.Exec` into the runtime (D29). A seat's processes
  can signal only processes in their own domain: the runtime and everything
  it forks. The launcher, the other seats and every host process are EPERM,
  signal 0 included. Signals from the launcher to the seat are unaffected,
  because the launcher is outside every domain. `--die-with-parent`
  (pdeathsig) and SIGCHLD are kernel deliveries that do not pass the
  `task_kill` hook, and the live pin shows the seat still dies with its
  parent. The one plan is `wallUnixRuleset(abi)` in
  `internal/posse/wallunix.go`, and the Linux file converts it into the attr
  the kernel gets. There is no second spelling to drift.
- **D35. This amends D17's rationale, not its render.** The host pid
  namespace stays, for VISIBILITY: a seat still sees host pids, `checkorphans`
  still reads the whole box's table, and the `/proc/<pid>/root` premise
  (ranger-base-di8da) is untouched. It no longer means REACH. D17's
  `--unshare-pid` rejection stands for every reason it gives. ADR 0096 D2 is
  unchanged, but one sentence of 0096's reasoning is now out of date:
  "weaker than `kill(2)`, which the seat already has against the same set".
  The seat no longer has it. That only strengthens 0096's rejection of
  `--bind /proc /proc`, because the knobs it would open are no longer
  dominated by a reach the seat holds anyway.
- **D36. The step is typed from ABI 6, not 9.** SCOPE_SIGNAL is ABI 6
  (Linux 6.12) and RESOLVE_UNIX is ABI 9, so this adds no new kernel floor.
  On an ABI 6-8 host the step applies the signal scope alone, and
  `WallUnixDegraded` says the socket half is missing and the signal half is
  up. Below ABI 6 it says neither is up, and the PID-level `pkill`/`killall`
  deny is the only cover left. This replaces D31's "below ABI 9 the step is
  not rendered", which was written when the step carried one right. D30
  still holds for each half: the render types only what the ABI it read
  supports, and `RunWallUnix` re-reads the ABI and refuses below 6 rather
  than exec without the domain.
- **D37. What it costs, and who pays.** A shell `kill -0` from a seat now
  exits 1 for a LIVE host pid. So AGENTS.md's `checkorphans` fallback reads
  the errno instead (EPERM means alive, ESRCH means gone), and that fallback
  landed with the code. posse's own signal call sites were censused when it
  landed: `pidAlive` and `procAlive` already read EPERM as alive, the load
  guard's kill arm and wall-watch run on the host, and `beads.go` and
  `githang.go` signal only their own children. ASSUMED: no runtime posse
  launches needs to signal a process it did not start. A runtime that does
  fails visibly, with EPERM at `kill(2)`.

**Alternatives rejected.**
- **`--unshare-pid` (b1).** It cuts reach by cutting visibility, so it costs
  every reason D17 gives, plus ADR 0096 arm C as measured: a seat no longer
  sees or signals its host-side pids, and `checkorphans` reads only the
  seat's namespace.
- **Accept it, on the PID-level deny (b2).** That deny covers two verbs, and
  the fix here is one struct field in a ruleset already applied. A cheap cut
  exists, so accepting the reach buys nothing.
- **seccomp on `kill`, `tgkill` and `rt_sigqueueinfo`.** A filter sees the
  argument values, not the pid's ancestry, so it cannot tell "my child" from
  "the launcher". The best it can do is refuse every signal, and that breaks
  job control and every test harness that ends its own children.
- **A separate Landlock step for the scope.** A second domain is a second
  thing to render, report and pin. The scope belongs in the ruleset that is
  already there.

**Open, and not decided here: `LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET`.** It
is the same field and the same ABI. ranger-base-x9f3s found no reachable
host abstract listener. MEASURED 2026-11-15 from a seat on D1 (`ss -xl`,
which the seat sees whole because it shares the host's net namespace): no
abstract STREAM listener at all, and one unconnected abstract datagram
socket. Scoping it would turn that observation into a property. A desktop
host can grow such a listener on demand, Xwayland's `@/tmp/.X11-unix/X0`
being the common one, which is the D28 lesson in a different namespace. The
unread half is what a seat's runtime and its test suites connect to. Its
bead in the block below carries that measurement as its gate.

**Beads.**

- ranger-base-al0d0
- ranger-base-e5un1
- ranger-base-sv89t
- ranger-base-uxubn
- ranger-base-zqtoi

## Amendment: a seat's launch line is exec'd, so no shell outlives its runtime (ranger-base-1l1en, 2026-11-15)

*This amendment's design bead is `vuln-private` and is named by bare id in
the Beads block below and nowhere else. What is described here is what this
change SHUTS, which AGENTS.md "Landing the plane" allows and which is why
the code looks the way it does.*

**Context.** D9's measured table has the row *runtime exits, shell stays →
seat stays listed (block open) — no husk*. That row is a STATE, and it is
the state a seat's exposure lived in. A pane whose shell has come back to
its prompt is a command interpreter reading the seat's tty, and it is
outside every wall and outside the gate PATH — `GatePrefix` rides the typed
LINE, not the shell, and on Rex the block's shell is `rexShell`'s real-shell
answer. What posse types at a seat is prompt text carrying bead titles and
comments, which are inputs.

Two readers reach a seat in that state. One is a prompt racing a wall
watcher's own exit: the owner check reads the watcher live, the text is
sent, and the watcher has returned by the time it lands. The other is wider
and is why the scope here is EVERY seat and not only a caged one — on Rex a
runtime that exits normally CLEARS its `program_status` record, so
`runtimeOwner` answers unknown, `refuseAtDeadOwner` has nothing to refuse,
and `Prompt` types. Dispatch relaunches before it prompts; `posse prompt`,
the cockpit and any other caller that prompts between a seat's exit and the
next dispatch pass do not.

**Decision.** Every launch line posse types is exec'd, so the pane's shell
is REPLACED by the launch and nothing that reads that tty as a command
interpreter is left behind when the runtime (or, on a bwrap seat, the
watcher) ends. It is applied in one place — `App.PaneLine`
(internal/posse/paneline.go), which all three launch sites pass through —
and to both of that function's forms, because `exec` inside a SOURCED file
replaces the sourcing shell exactly as the typed word does.

**A trailing `; exit` is not enough, and that is the whole reason this is
exec.** MEASURED 2026-11-14 (the design bead) and again 2026-11-15 as a
repeatable pin (`internal/posse/launchexeclive_linux_test.go`), GNU bash
5.3.20 on a pty, the line typed as a seat's is:

| line | the shell afterwards |
|---|---|
| `sh -c '…; exit 3'; exit` | gone |
| `sh -c '…; kill -INT $$'; exit` | **SURVIVED**, and ran the next line typed |
| `exec sh -c '…; kill -INT $$'` | gone |

An interactive bash abandons the rest of a command list when its foreground
job dies of SIGINT — so a trailing `exit` is skipped in exactly the case the
seat was killed by a key or a signal, and the watcher's drain runs with ISIG
still on. The middle row is kept as a live pin's negative case rather than
deleted: a renderer that swapped exec for `; exit` would pass every static
pin in the tree and red that one.

**Where the word goes.** Every launch line this tree renders is a simple
command behind a run of `NAME=VALUE` assignments — the gate prefix's
PATH/SHELL/GROK_SHELL, D1's runtime-home pin, the transcript ceiling,
CLAUDE_CONFIG_DIR. MEASURED 2026-11-15, bash 5.3.20 and sh on this box:
`FOO=bar exec /usr/bin/env` prints `FOO=bar` and runs in the shell's own pid
(replaced, and carrying the assignment), while `exec FOO=bar /usr/bin/env`
is `exec: FOO=bar: not found`. So `exec` lands ON the command word, after
the assignments — which is also where it reads correctly in a pane's
scrollback. A line that is NOT one simple command cannot be exec'd in place
at all; nothing posse renders today is one, and such a line is exec'd as
`exec /bin/sh -c '<line>'` and always spilled to the launch script. That
fallback costs the pane's foreground process being a shell rather than the
runtime — the property `. <script>` was chosen to keep (rangerhq-ybec) and
the one herdr's argv0 detection and every kill path read — so it is a
fallback with a pin of its own, and not the rule.

**The D9 row changes.** The 2026-11-06 table stays as measured; this is what
it reads now:

| probe | result |
|---|---|
| runtime exits, or is killed | the block CLOSES; the seat drops out of `List()`; after `PruneGrace` the record is pruned and the Rex session destroyed (D10) |

There is one way a seat ends now, where there were two. **D10 is therefore
no longer the rare row — it is the only one**, and that is the load-bearing
consequence of this change: without the destroy, every seat that ever ran
would leave a session holding one of the server's `--max-sessions` slots,
shared by every home (D2).

**MEASURED 2026-11-15, this box** (linux/amd64, rex 0.0.1794069975-g6c6689,
herdr installed, posse built from this tree). A SCRATCH Rex server on a
private socket (`--restore=false --max-sessions 6`) and a scratch herdr
server on its own session socket; no fleet seat was touched and no model ran:

| probe | result |
|---|---|
| rex, exec'd line, program ended | `block ls` → `{"blocks": []}`, `window ls` → `"windows": []`, the session still listed: a husk |
| rex, the same line NOT exec'd | the block stays open and its shell is at a prompt |
| `rex capture` of that closed block | `block target "<id>" not found` — the scrollback went with it |
| server session count across it | `sessions: 3` with the husk |
| D10, end to end | a scratch home, `posse new husk --cmd …` whose command ended, the meta backdated past `PruneGrace`, ONE `posse list` → the seat left the listing, `rex ls` lost the label, `sessions: 3` → `2`, and the meta was unlinked |
| a fresh create on the same name after that prune | create path ran clean, a new session, no orphan and no name refusal; the pane read `exec /bin/sh -c 'sleep 60'` |
| herdr, exec'd line, program ended | the pane is GONE (`pane_not_found`) and the one-pane workspace is gone from `workspace list` |
| herdr, the same line NOT exec'd | the workspace is still listed, its shell at a prompt |

So herdr needs nothing for the HUSK, for the reason D10's first line already
gives: a dead herdr workspace is gone, there is no husk, and posse's prune
reads it through the same empty-listing arm it always did.

**One thing had to change with the exec, and it is the part of this that
could have taken the fleet down.** A seat's runtime now REPLACES the block's
shell, which means it inherits the shell's pid and is itself the block's
`child`. MEASURED above:

|  | child | foreground | ancestors |
|---|---|---|---|
| shell at its prompt | `{P,"bash"}` | `{P,"bash"}` | — |
| exec'd runtime | `{P,"sleep"}` | `{P,"sleep"}` | — |
| runtime the shell started | `{P,"bash"}` | `{Q,"sleep"}` | `[{P,"bash"}]` |

The first two rows are the same SHAPE. ADR 0076 §3's "owner must not be the
pane's shell" guard was `owner == process.child` and nothing else, so with
the exec in place every reporting runtime would have read as *its owner is
the block's own shell* → `MuxUnknown`: no launch would confirm (§3), every
seat would list as unreadable, and dispatch could never prompt one. The
guard asks a second question now — is that child the shell posse STARTS a
block with (`rexShell`, the path it handed `rex new`) — and `RexProc` reads
`path` for exactly that. The pins are
`TestQARexStateReadsARuntimeThatReplacedTheBlocksShell` beside the one it is
the other side of, and the fixtures stopped spelling the block's shell as the
literal `zsh`, which was only ever true of the box the shape was first
measured on.

**The same class is open on the herdr route and is NOT fixed here**:
`PaneProcesses.AtShellPrompt` reads "every foreground pid IS the pane's own
shell pid", and MEASURED above the exec'd row is byte-identical to the
shell-at-its-prompt row in the two fields posse parses (`shell_pid` and the
foreground pids). It is reached only by the reported-detection prompt route
(ADR 0061 / §4) and no built-in runtime declares `detection: reported`, so
what it costs is a declared runtime on a herdr instance refusing every typed
prompt. It is ranger-base-m5rdy, with the three rows and the candidates on
it: posse does not choose herdr's pane shell, so the Rex answer does not
transfer and that one needs its own call.

**What is lost, said out loud.** `posse peek` of a seat that died. Before
this change the D9 row left the block open, so an operator could read the
scrollback of a seat whose runtime had exited — including, on a caged seat,
`posse wall-watch`'s own `KILLED seat …` line. That block closes now and the
scrollback is gone with it, measured above. On the happy path nothing is
lost: the watcher FILES a bead and the bead is the record. What is lost is
the double-failure path the watcher's third line exists for — the wall came
off AND bd could not be written, where `⚠ the record is incomplete … this
line is the record` was the only remaining copy. That is ranger-base-pfcci,
filed with this change and not fixed in it.

**And one diagnosis moves, which is worth naming beside the loss above.**
§3's launch confirmation (`confirmReports`) has three arms: reported; a READ
ERROR, which warns and keeps the launch ("unknown is not no"); and no record
inside `rt.Wait()`, which is the arm that refuses or declares DEGRADED. A
runtime that died at once used to leave its shell, so the block stayed open
and the poll answered *no record* — the third arm. It closes with the runtime
now, so the read errors with rex's `block target "<id>" not found` and the
SECOND arm fires. MEASURED above, on the scratch server, in both directions:
a `--cmd` that ended printed *could not be confirmed — rex would not answer
the program_status read*, and one that stayed up printed the proper
*DEGRADED — no program_status record … within 45s*. The GATE's property
survives by another route — nothing is prompted, because `awaitAgent` then
fails and benches the slot — but the sentence now blames a server that
answered correctly. That is ranger-base-ybrk6: a predicate for a block that
is gone (the sibling of `MuxCloseGone`, which covers a session and not a
block) and a third reason that names the seat. Filed with this change and
not fixed in it, because it wants rex's error surface measured per verb
before anything matches on one message.

**The relaunch arm, re-read.** `dispatch.go`'s `s.Status == ""` arm and
`RelaunchAgent` were written for this exact row: a persona's CLI exited and
left a bare shell still carrying the launch env, so re-typing the line there
is a full restart without a new workspace (rangerhq-vk2). After this change
that premise is gone on both occupants — the pane closes, the seat drops out
of `List()`, `Resolve` fails, and the bead takes the CREATE path. Both are
kept, and neither is now reached for the reason it was written: what can
still reach them is a seat launched by a posse from before this change, or
an occupant that keeps a dead pane open. They are correct there and they
refuse everywhere else (`AgentTarget` finds an agent, or `Resolve` finds no
session), so retiring them is a later, measurable call and not this bead's.

**Measured versus assumed.**
- MEASURED, this box, above: the bash/pty rows, the exec placement rows, the
  rex block and husk behaviour, `rex capture` of a closed block, D10's
  destroy end to end, the create path after it, and herdr's pane.
- MEASURED, this tree: every built-in runtime's create line, the re-typed
  line and the spilled script's command line are exec'd
  (`internal/posse/launchexec_qa_test.go`); the roster of every
  `Start(seat, line)` in the tree, and that a launch's line comes from
  `PaneLine` while the probe's `command -v` lookup does not
  (`internal/treepins/launchexec_qa_test.go`); the live pty rows, the
  sourced form, and that the assignment prefix still reaches the runtime
  across the exec (`internal/posse/launchexeclive_linux_test.go`); and §3's
  reading over a runtime that is the block's own child
  (`internal/posse/muxrex_qa_test.go`).
- ASSUMED, not measured: **zsh**. `rexShell` prefers zsh and zsh is not
  installed on this box, so the SIGINT row above is a bash reading. `exec`
  itself is POSIX and the placement rule is too; what is unmeasured is only
  the negative row, which is the row posse does not render.
- ASSUMED, not measured: darwin. The pty pin is Linux-only (the ioctls), and
  herdr's pane behaviour was read on the Linux build here.
- Out of scope: making the watcher's verdict durable (ranger-base-pfcci),
  `posse peek` saying where a gone seat's reason went, the confirmation's
  third reason (ranger-base-ybrk6), and the herdr half of the shell-pid
  class (ranger-base-m5rdy).

**Beads.**

- ranger-base-ie4mx
- ranger-base-pfcci
- ranger-base-ybrk6
- ranger-base-m5rdy

## Amendment: the socket wall's domain must hand REFER back (ranger-base-p5i2c, 2026-11-15)

**The finding.** A Landlock domain that handles ANY filesystem access right
and does not handle `LANDLOCK_ACCESS_FS_REFER` denies every `rename(2)` and
`link(2)` whose source and destination directories differ, with `EXDEV`, for
every process in the domain, everywhere in its view, whatever the mounts say.
D27's domain handles exactly one filesystem right, so it inherited that
default-deny — and D27's own words, and the sentence `posse gates` prints from
them ("The mounts above are unchanged by it"), said the opposite.

**What it cost.** `git branch -m` fails inside the step: `unable to move
logfile logs/refs/heads/main to logs/refs/.tmp-renamed-log: Invalid
cross-device link`, exit 128. ranger-base-i92tj put the step on the
landed-check judge's line, so behind it four `internal/treepins`
`bdargvgate_qa_test.go` pins reddened on their fixture's `git branch -M`,
`make treepins-check` reddened, and ADR 0088's judge reported main RED on
every tree — including the tree last RECORDED green, which is the control
that proved no landing had caused it (ranger-base-ufd7z).

**Why U11 read green over it.** The 2026-11-13 probe's U11 row renamed
`W/d -> W/d2`: one directory, which this domain never touched. The class is
cross-directory refer and nothing else. `mv` reads green over it too, because
coreutils falls back to copy+unlink on `EXDEV`, so a shell probe cannot see
the class at all — it takes `python3`/`os.rename`, `ln`, or git.

**Decision.**
- **D33. The domain handles `LANDLOCK_ACCESS_FS_REFER` and adds one rule
  granting REFER on `/`.** Handled-and-granted is a no-op, and it is the only
  spelling Landlock has for "leave refer alone": the right has to be handled,
  or the kernel's refer default-deny applies, and it has to be granted, or
  Landlock's allow-only rule denies it everywhere. The grant confers no access
  a process did not have — `rename(2)` and `link(2)` still need a writable
  mount and a permitted uid — so it restores exactly the status quo this layer
  was never meant to change, and nothing else.
- **D34. `/` and not a list of the seat's grants.** The grant has to reach
  everywhere the seat can already write, which is what the bwrap mounts
  decide and not this layer; a narrower rule would make the same bug arrive
  again at whatever path the next PID grants. It is also the one path in a
  seat's view that cannot be unlinked and re-bound under the rule, which D28
  requires of any rule for U12/U13's reason.
- **D35. REFER rides on the RESOLVE_UNIX arm and only on it.** The ABI 6-8
  arm handles no filesystem right at all, so there is no refer default-deny
  there and nothing to give back. The grant is derived from the ruleset's own
  REFER bit (`wallUnixReferGrant`), never from a second list, so an arm cannot
  handle the right and forget to grant it.
- **D36. The step still refuses rather than execs when the rule cannot be
  added** (D30, unchanged). A ruleset created, not granted REFER, and enforced
  anyway is a seat whose `rename(2)` is silently `EXDEV` — this bug arriving
  as a mystery instead of a refusal.

**Measured versus assumed.**
- MEASURED 2026-11-15, D1 host (Linux 7.2.15-1-omarchy, Landlock ABI 10, git
  2.56.0), `docs/adr/0080-resolve-unix.probe.sh` rows U14-U16, every row as
  its EXPECT, 17 rows and 0 mismatched: under RESOLVE_UNIX alone, rename
  within one directory is ok while rename across directories, link across
  directories and rename of a DIRECTORY across directories are all `EXDEV`;
  under RESOLVE_UNIX|REFER with a REFER rule on `/`, all four are ok; and the
  socket wall still answers `EACCES` under that grant, because a rule grants
  only the rights it names and nothing anywhere grants RESOLVE_UNIX.
- MEASURED 2026-11-15, same box, by `posse wall-unix -- <cmd>` against an
  unwalled control: before the fix, `git branch -m` exit 128 inside and 0
  outside; after it, 0 both sides, with `connect(2)` to a host pathname
  socket still `EACCES` and `kill(2)` to a host process outside the seat still
  `EPERM` inside while both are ok outside. The step alone reproduces and
  resolves it; no bwrap is needed.
- MEASURED, this tree: the ruleset pin reads REFER on exactly the arms that
  handle RESOLVE_UNIX and reads the grant as REFER alone, from every GOOS
  (`internal/posse/wallunixrender_qa_test.go`); the live pin renames and links
  ACROSS directories inside a real bwrap seat behind the step, against a
  control seat that differs by the step alone
  (`internal/posse/wallunixlive_test.go`); and the Linux file carries a
  compile-time assertion that the hand-spelled REFER bit is x/sys's.
- ASSUMED, not measured: that no other right this domain might handle later
  carries a default-deny of its own. REFER is the only `LANDLOCK_ACCESS_FS_*`
  bit documented to work this way, but the lesson generalizes — a right added
  to `handled_access_fs` restricts everything it covers, and the probe is
  where that is read before it ships.
- Out of scope: the darwin half, which is unmeasured and unrendered, and
  whether `LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET` belongs here (still open).

**Beads.**

- ranger-base-p5i2c
- ranger-base-ufd7z
