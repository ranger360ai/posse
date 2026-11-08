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
*bob is the exception (ADR 0076 Amendment 4, D14, ranger-base-778y0): `bob chat` takes no argv prompt and writes no record before a typed one, so bob is herdr-only on Rex, and a bob crew on the D2 host uses herdr beside it.*
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
  operator reads them before a codex or grok crew runs unattended.

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

The rename landed: `seatbeltWallRendered` is `wallRendered`, in
`internal/posse/seatbelt.go`, and D21's text below should be read against
that name. It still carries `!rt.SelfSandbox` unconditionally — D21's flip
waits for probe T6 — and `credentialDirEnvSetRefusal` (credential.go) follows
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
   empty directory first. Otherwise nothing can create it and no mount is
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
`internal/posse/bwrapargs_qa_test.go` over fixture trees on darwin. No launch
wiring; nothing calls it. Five shapes as listed, with three things the text
did not settle and one residual for D18. Full reasoning in
`docs/notes.d/ranger-base-qoamy.md`.

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

**D21. codex is walled on Linux.** On darwin, `seatbeltWallRendered` skips
SelfSandbox runtimes because nested `sandbox_apply` is refused. On Linux, B7
measured bwrap nesting inside bwrap. So `wallRendered` drops `!rt.SelfSandbox`
when the mechanism is `bwrap`, and parity's "cannot wrap codex" declared
difference becomes darwin-only. codex's own sandbox then runs inside ours,
and a write needs both walls to allow it. That is stronger than darwin, where
codex's own sandbox is the only file wall. Gated on probe T6 (`codex sandbox`,
no turn). Until T6 reads as expected, the flip stays off and Linux codex
keeps darwin's posture.

**D22. The probe for this design is `docs/adr/0080-bwrap-tier.probe.sh`.**
T1 is the D19 save under the shipped wall, T2-T5 are D20's git grants against
an outside pack, T6 is D21's nesting, and T7 is D18's `--args` fd. It writes
only under one mktemp dir and calls no model. D19 and D20 can be built and
pinned on darwin, because the renderer is pure. D18's live arm and D21 wait
for the run.

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
  way on Linux (T1); D20's keep file, git ignoring it, and the post-pack
  commit (T4-T4e); the packed-refs residual (T5); codex nesting (T6); bwrap
  closing the `--args` fd, though a leaked fd would be read-only and hold
  nothing secret (T7c); claude's own `/sandbox` nesting in a real session,
  which no row tests without a turn.
- STATIC, this tree: the render inputs and call sites named above.
