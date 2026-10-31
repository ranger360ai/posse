# grok's cwd skills root has a precondition on the DIRECTORY: declared, probed, not seeded (ranger-base-kwbh1)

ranger-base-9f2o6 re-measured grok's skills discovery for the §6c drift clock
and found a gate nobody had declared: the cwd root is read only in a folder
grok considers trusted. It handed the code lane three options — seed the trust,
declare the precondition, or rule it the operator's. This bead did the second
and third, and refused the first on a measurement.

## MEASURED 2026-10-31, grok 1.0.52 (0920c8169e27), darwin 25.6.0, this box

Two scratch git repos with IDENTICAL content under ONE fixture `$HOME` whose
`trusted_folders.toml` named only the first — so the two arms differ in the
trust entry and in nothing else. Both carried
`.agents/skills/qm-alpha/SKILL.md` and this `.claude/settings.json`:

    {"permissions":{"allow":["Bash(qm-probe-allow:*)"],
                    "deny":["Bash(qm-probe-deny:*)"]},
     "hooks":{"SessionStart":[{"hooks":[{"type":"command",
                                        "command":"echo qm-probe-hook"}]}]}}

| `grok inspect` reports | trusted repo | untrusted repo |
|---|---|---|
| `Project trusted:` | yes | no |
| `Skills` | `(1)  qm-alpha  project` | `(None)` |
| Permissions `Source` | `<repo>/.claude/settings.json` | `(none)` |
| Permissions loaded | 2 | 0 |

The skills half is 9f2o6's finding, confirmed on a cleaner rig (one HOME, two
repos, one variable). **The permissions half is new** and is the reason the
seed was refused — see below.

Three more readings, each of which changed the design:

- **`grok inspect` is cheap enough for the launch path.** 0.061s wall, exit 0,
  no network, and `~/.grok` byte-identical before and after (a stat census over
  every file under it). It is documented as "Show the configuration Grok
  discovers for this directory" and reaches no model.
- **Trust resolves through the GIT ROOT to a linked worktree's main checkout.**
  In `<this worktree>/.tmp-trustprobe/deep` — a path with no
  `trusted_folders.toml` entry of its own and no trusted ancestor anywhere in
  the table (the table names six `~/src/*` repos and nothing under
  `~/.posse/worktrees`) — grok answers `Project trusted: yes` and reports the
  WORKTREE as its git root. So one acceptance per repo covers every seat posse
  opens in it, and a reimplementation of that rule in posse would be a second
  implementation that disagrees silently.
- **`grok --version`'s channel suffix is a function of `$HOME`, not of the
  binary.** One binary, three homes: `grok 1.0.52 (0920c8169e27) [stable]`
  under the operator's, `grok 1.0.52 (0920c8169e27)` under a fixture and under
  an empty one. `~/.grok/config.toml` has no channel key (`[cli] auto_update`,
  `maximum_version`, `required_maximum_version`), so some other state under
  `~/.grok` supplies it. That makes grok's `skills_verified:` declaration —
  which §6c compares VERBATIM — permanently drifted on any fresh-`$HOME` seat.
  Filed as **ranger-base-3a5z7**; not fixed here, because the string is
  9f2o6's landing and the fix is a choice between two spellings of §6c.

## The decision: declare it, probe it by asking grok, do not seed it

ADR 0007 §6f. The shape, and why each half is what it is:

**Declared** — `SkillsDirTrust` on the Runtime (What · Where · Grant · Probe),
built-in only. No `runtimes/*.yaml` key, deliberately: a declaration with no
probe is a permanent UNKNOWN, an UNKNOWN degrades nothing, so the key would
buy an onboarder a sentence that reads like a guarantee and changes no launch.

**Probed by asking the CLI** — `grok inspect`'s own `Project trusted:` line,
never posse's model of `trusted_folders.toml`. The worktree reading above is
the whole argument: the rule is a multi-step resolution (resolve the path,
find the git root, follow a linked worktree to its main checkout, match
ancestors in the table), and getting it wrong in the "reads trusted when it is
not" direction reproduces exactly the silence this bead exists to end.

**Not met degrades; unknown never does** — an untrusted directory takes the
skills row OUT of `Realized` and adds a `Degraded` line. The removal is the
point: a Realized row over a root the CLI will not read IS ranger-base-65rc's
clean launch with an empty binding. UNKNOWN leaves the verdict alone and
prints the reading, because posse refuses on a reading and never on ignorance
(ranger-base-9r33). §6b outranks §6f: an UNPROVEN root has already degraded
the launch with the louder sentence, and a second line about a precondition on
a root nobody has measured reading is noise over the top of it.

**Not seeded.** The three sibling grants are the contrast — codex's
`trust_level` and bob's `--trust` are ARGV (per directory, session-scoped,
visible in `ps`, nothing written to the operator's box), and claude's
`hasTrustDialogAccepted` IS written because without it the session parks on a
modal and never becomes promptable, so that seed buys the LAUNCH and claude
offers no line to type it on. grok has neither shape: no flag and no overlay,
so the only channel is a persistent, box-wide entry in the operator's own
file; the launch it would buy is not dead, only skill-less. And then the third
reason, which is the one that decides it: the grant is **not scoped to
skills**. The table above measures folder trust also deciding whether grok
reads the repo's OWN `.claude/settings.json` as a permissions source — a
repo→box authority channel grok's built-in declares nothing about
(`ProjectConfig` is nil, which `projectConfigRow`'s own `missing:` line calls
"the silent one"), and one posse's claude built-in declares and degrades over.
Writing a grant on the operator's box that opens a channel nobody has measured
is not posse's to do. Filed as **ranger-base-1g8gf** (P1, security lane), with
the repro and the three things that are still unmeasured — chiefly whether
grok EXECUTES the project's `hooks` from that file, which `grok inspect`
cannot answer.

So the remedy is the operator's, once per checkout, and the degrade line says
so in those words.

## Where it is read, and where it deliberately is not

The reading is per DIRECTORY, so it sits at the dir-aware parity layer beside
`ProjectConfigTrust` — the launch, and `posse gates` in the session dir.

- `posse runtime check grok` prints the DECLARATION and no reading. That grid
  is a runtime × box statement and never invents a cwd (CheckParity's own
  rule); the row names the dir-aware surfaces that do read it.
- `posse runtime drift` is untouched. Its subject is the CLI VERSION and its
  exit codes are a per-box roster row (ranger-base-f5jbs); a directory is not
  a property of the box, the walk takes no dir, and giving it one would make
  `0 of N measured` mean two things.

## Changes

- `internal/posse/skillsdirtrust.go` (new): `SkillsDirTrust` (the
  declaration), `DirTrust` (the three-valued reading, Silence's shape and
  Silence's reason), grok's probe over `grok inspect` split into an exec seam
  and a parse, `applySkillsDirTrust`, the degrade sentence, and
  `skillsGateKey` — one spelling of the skills row's Realized key, because two
  layers write it now and two spellings would make the removal a silent no-op.
- `internal/posse/runtime.go`: the `SkillsDirTrust` field; grok declares
  `GrokSkillsDirTrust`; the grok comment block says the precondition is
  declared now rather than only filed.
- `internal/posse/parity.go`: one call in `checkParityIn`, and the two skills
  Realized writes go through `skillsGateKey`.
- `internal/posse/runtimecheck.go`: the declaration in the skills row.
- `internal/posse/trust_test.go`: the assertion's sentence said "posse types
  codex's trust on the line and grok needs none", and the second half was
  false. It now says the true thing the test actually proves — codex's trust is
  TYPED and grok's is the operator's to grant, never written from here.
- `internal/posse/runtimefields_qa_test.go`: the ADR 0017 §3 field audit entry.
- `internal/posse/skillsdirtrust_qa_test.go` (new): the parse over verbatim
  captures of both arms, the three verdicts, the two deliberate silences, every
  declaration's readability, and a census that reds if a later bead writes
  grok's trust table.
- `internal/posse/skillsrules_parity_qa_test.go`: the live arm now also asserts
  that POSSE's parse agrees with grok, in both directions, over the bytes grok
  actually printed — a probe pinned against a fixture the test wrote is pinned
  against nothing.

## Verified

- `go test -tags posse_arm3 -run '<the five new pins + TestSeedTrust + TestCheckParity>'
  ./internal/posse/` — green, each test named in a `-v` run.
- **MUTATION-CHECKED, six, each red:** folding UNKNOWN into untrusted (1
  failure); keeping the Realized row on an untrusted dir (1); reading an
  unrecognised value as untrusted (2); dropping grok's declaration (3); adding
  a `SeedGrokTrust` that writes the table (1); and, on the LIVE arm, a parse
  that reads `Project trusted: no` as trusted (the untrusted half reds by
  name).
- `RHQ_PARITY_LIVE=1 go test -tags posse_arm3 -run
  'TestQALiveSkillDiscoveryPerRuntime/cwd-runtimes' ./internal/posse/` — green
  against the installed grok 1.0.52, both the trusted and untrusted fixtures.
- End-to-end with the REAL probe through `CheckParityIn`: an untrusted scratch
  repo degrades with the full sentence and the skills row is gone from
  Realized.
- **The first full `make test` was RED, and it was the mechanism working.**
  `TestQASkillSurfacePerRuntimeDeclaration/grok` creates a real grok session
  with a `skills:` PID in a `t.TempDir()`, and the production probe — which
  runs `grok inspect` with the process environment exactly as a launch would
  — read `Project trusted: no` and refused the launch. A test binary here runs
  under a redirected `$HOME` (ADR 0047) with no grok config in it, so EVERY
  directory inside the suite is untrusted. Two consequences worth keeping:
  a hermetic test that drives the success path has to GRANT the precondition
  (`qmTrustGrokDir`, which merges into the shared test home under a mutex
  because these tests are `t.Parallel()` and there is no per-test env to set),
  and a hermetic PIN has to SWAP the probe rather than trust the box — which
  is what the new pins do. Keyed on `rt.SkillsDirTrust != nil` and not on
  grok by name, so the next declaring runtime arrives driven.
  MUTATION-CHECKED: dropping that seed reds the row again.
- `make fmt-check`, `make notes-check`, `make crew-check`, `make tree-check`
  (1:38 wall, every door ok), and `make test` green over the final tree —
  three arms, `cmd/posse` 256s, `internal/posse` 417s/344s/383s,
  `internal/treepins` 463s.
