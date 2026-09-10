# NOTES — source and operating map

Start with [README.md](README.md) for the product and
[INSTALL.md](INSTALL.md) for installation. This page maps the implementation
and its operating rules. Decisions live in [docs/adr/](docs/adr/);
procedures live in [docs/runbooks/](docs/runbooks/).

The public sections formerly on this page are preserved unchanged in
[docs/notes.d/](docs/notes.d/). Sections carrying instance-ops content are
preserved separately for the private instance tree, as cited in the table below.
They are historical records, not a second source of current configuration.
Their source paths and commands are relative to the repository root.
Per-bead follow-ups have a [month-by-month index](docs/notes.d/README.md).
The tmux-era implementation remains on the `tmux-reference` branch.

## Authoritative stores and writers

| Store | Authority and writer |
| --- | --- |
| Repository source, examples and ADRs | Maintainers change the product through reviewed commits; `embed.go` carries the generic seed. |
| The instance home (`RHQ_HOME`) | The operator owns configuration, env sets, PIDs, runtime declarations and recipes. The harness reads them to plan launches. |
| Persona directories and `ORDERS.md` | Agents record durable lessons; the operator owns persona policy and promotion. These belong to the instance. |
| The configured beads stores | `bd` records work, claims, dependencies and completion. Dispatch reads these stores; a session's `BEADS_DIR` identifies its store of record. |
| `state/herdr/<name>.yaml` | The launcher records a session's recipe and workspace identity. Backend state is observed from herdr, not reconstructed from labels alone. |
| Dispatch run records, ledgers and pause state | The harness records its actions and accounting; operator controls such as pause change whether new work may launch. |
| Session worktree and Git commits | The agent edits its isolated tree and commits its work. Landing moves commits; uncommitted files remain in that tree. |

See [state](docs/notes.d/notes-state.md) and
[workspace identity](docs/notes.d/notes-workspace-identity.md) for ownership
checks. The table below also cites the private dispatch and beads records.

## Launch, settle and landing

1. **Select and claim work.** Dispatch reads the queue, routes a bead to a
   lane and seat, and checks existing holders before launching another worker.
2. **Plan the launch.** Resolve the persona, runtime, tier, environment,
   working directory and cage; check the required enforcement before creating
   a workspace. The launcher records the resulting session identity and recipe.
3. **Run and observe.** Submit work after the runtime has a detected screen.
   Herdr reports activity; the bead records whether the work actually settled.
   A terminal state alone does not prove that a commit or close happened.
4. **Record the result.** The agent verifies its changes, commits named paths,
   and records the outcome or blocker on the bead. A dispatched worktree has
   its own index; the main checkout may have concurrent writers.
5. **Land committed work.** The launcher checks and integrates session commits
   into the repository's branch. Conflicts and blocked landing need attention;
   a bead close is not evidence that every byte reached that branch.
6. **Retire or relaunch deliberately.** A relaunch plans the replacement before
   closing the old workspace. Settlement and landing precede destruction;
   timeout or uncertain ownership is reported instead of guessed away.

See [state and relaunch](docs/notes.d/notes-state.md). The dispatch and
shared-working-tree history is cited in the table below as private records.

## Build and test entry points

| Command | Purpose |
| --- | --- |
| `make build` | Build the working tree into `bin/posse-go` for development. |
| `make release` | Build committed HEAD in a temporary worktree. |
| `make test` | Run prerequisites and all three suite arms, with suite-lock slots and per-package timeouts. |
| `make test-arm1` | Run the default package tree, tree checks and silent-revert audit. |
| `make test-arm2` / `make test-arm3` | Type-check the partition and run the corresponding tagged `internal/posse` tests. |
| `make tree-check` | Run formatting and the tree-wide QA checks; use it after a focused test run. |
| `go test -timeout 25m -run '<name>' ./internal/treepins` | Run selected tests whose subject is the repository tree. |
| `make verify-box` | Check the local installation; this is distinct from hermetic fixture tests. |

A bare `go test ./...` compiles only the default arm of `internal/posse`.
It omits the other two build-tag partitions, the suite wrapper and its
box-wide queue, and the Makefile's additional checks. For a focused test in
another arm, pass `-tags posse_arm2` or `-tags posse_arm3` explicitly.
[Testing history](docs/notes.d/notes-testing.md) records why these entry points exist.
The [Makefile](Makefile) is the executable source of their current recipes.

## Enforcement classes

Instructions and runtime permission prompts are cooperative controls. They
help an agent follow policy but do not turn arbitrary code execution into a
restricted capability. Shell shims and Git hooks enforce particular command
and repository paths; their coverage depends on the launch and installed hook.
They are not an operating-system boundary against arbitrary programs.

A cage or seatbelt supplies the OS or container boundary for the declared
filesystem and process access. Its guarantees depend on the actual engine,
mounts and launch configuration. Parity checks report realized controls and
missing coverage; a rendered rule alone is not proof of enforcement.
See [write overlays](docs/notes.d/notes-write-overlays.md) and
[ADR 0025](docs/adr/0025-enforcement-class.md). The table below cites the
private security-posture and container-tier records.

## Privacy model

A public artifact belongs here only when any deployer of this software could
have written it. Instance configuration, operational measurements, accounts,
credentials and persona memory belong in the operator's private instance.
Beads inherit their repository's visibility. A local aggregation of stores
does not authorize copying their contents between audiences.

This rule applies to prose and examples as well as code and beads. The
visibility lint helps catch mistakes; the routing rule and repository audience
remain the boundary. See [ADR 0024](docs/adr/0024-work-product-routing.md)
for the routing decision and the private privacy-model record cited below.

## Decisions still made by hand

The operator approves spending, publication and promotion to a live install;
chooses deployment policy and accepted degradation; resolves merge conflicts;
and decides cleanup where ownership or retained work is ambiguous. Agent instructions do not
authorize those decisions by themselves. The local
[installation checks](docs/notes.d/notes-live-box-checks.md) report what needs attention;
a green fixture suite does not prove the live installation is current.

## Historical sections

Each row names one former section, in its original order. Public links and
private citations identify where its unchanged text and bead IDs are retained.

| Section | Record |
| --- | --- |
| The mapping | [mapping](docs/notes.d/notes-mapping.md) |
| State lives in files, never in the multiplexer | [state](docs/notes.d/notes-state.md) |
| Dispatch primitives | `dispatch`: instance notes (private tree), not in this repo |
| The cockpit is a herdr plugin | [cockpit](docs/notes.d/notes-cockpit.md) |
| The YAML subset | [yaml](docs/notes.d/notes-yaml.md) |
| Env sets and secrets | `envs-and-secrets`: instance notes (private tree), not in this repo |
| Personas | [personas](docs/notes.d/notes-personas.md) |
| Leaked gate-shell children, and `POSSE_KEEP=` (ranger-base-apwr, -gvp2p) | [process-cleanup](docs/notes.d/notes-process-cleanup.md) |
| Fleet security posture (read this honestly) | `security-posture`: instance notes (private tree), not in this repo |
| Container tier (L4): what the spike measured (rangerhq-89a) | `container-tier`: instance notes (private tree), not in this repo |
| Path-scoped writes at L4: the overlays, measured (ranger-base-yu5) | [write-overlays](docs/notes.d/notes-write-overlays.md) |
| Cage engine re-evaluation: still Docker (rangerhq-rli) | [cage-engine](docs/notes.d/notes-cage-engine.md) |
| Privacy model | `privacy-model`: instance notes (private tree), not in this repo |
| beads (bd) substrate: pinned at 0.50.3, 0.51+ is a migration (rangerhq-f49) | `beads`: instance notes (private tree), not in this repo |
| grok substrate: pinned at 1.0.5, upgrades are a security re-audit (rangerhq-y7jr) | [grok](docs/notes.d/notes-grok.md) |
| Instance interstitials: the keys posse names, and the one it writes (ADR 0013 §2) | [interstitials](docs/notes.d/notes-interstitials.md) |
| herdr substrate: upgrading the fleet's herdr | [herdr](docs/notes.d/notes-herdr.md) |
| Workspace ids recycle across a server process boundary (rangerhq-6bg7) | [workspace-identity](docs/notes.d/notes-workspace-identity.md) |
| The shared working tree: one index, seven personas (rangerhq-nyqj) | `shared-working-tree`: instance notes (private tree), not in this repo |
| The live-box checks, and the one command that runs them (ranger-base-51z8j) | [live-box-checks](docs/notes.d/notes-live-box-checks.md) |
| Testing | [testing](docs/notes.d/notes-testing.md) |
| The fleet writes no auto-memory (ranger-base-7uhip) | [auto-memory](docs/notes.d/notes-auto-memory.md) |
