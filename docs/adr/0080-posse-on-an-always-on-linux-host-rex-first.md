# ADR 0080 — posse leaves the laptop: one always-on Linux host, one OS account, one Rex server, a home per crew

*Status: accepted 2026-11-06 (ranger-base-3v8gh, operator direction in an
architecture session the same day) · owner: architect · scope: the
operator's PERSONAL instances only; the work instance is out of scope and
unchanged · applies ADR 0075 §2 (the multiplexer slot) and ADR 0076 (Rex
as its second occupant) to a host that is not macOS · reverses ADR 0076 D4's
price judgement on operator direction (see D3)*

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
- The account is a dedicated `posse` user.
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
  operator reads them before a codex or grok crew runs unattended.

**D7. Exits, named now.** Each of these is a later bead, not this one.
- **One OS account per crew** via Rex's per-account login server, when one
  crew's walls stop being enough.
- **Landlock** as the Linux enforced tier (D4).
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
- Two never-measured Linux paths get measured by the shakedown, with no
  code first: the meter's credentials-file store (ADR 0019 V1) and
  `checkorphans` over `ps`.
- Arch's current git reads config-based hooks (`hook.*`), which the
  laptop's git 2.50.1 does not. The ranger-base-zwkgb arms that skip below
  2.54 can run there for the first time. This is a bonus, not a gate.
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
