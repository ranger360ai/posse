# grok 1.0.52 re-measured: the cwd root is still read, and it has a precondition nobody declared (ranger-base-9f2o6)

The first run of `posse runtime drift` (ranger-base-f5jbs) found grok's
`skills_verified:` reading `grok 1.0.5` against an installed
`grok 1.0.52 (0920c8169e27) [stable]`. ADR 0007 §6c's remedy for a drift is a
re-measurement and never a version bump in the declaration, so this bead
re-ran the live discovery measurement. The root is still read. It is also
gated on something no declaration here mentions.

## It cost no turn, and the bead was wrong to say it would

The bead's THE WORK §1 said the measurement "SPENDS A REAL TURN on grok, so
it needs the operator's per-run authorization", citing ranger-base-6rcv as the
shape. It does not, and 6rcv is a different instrument: that was a rules-
precedence probe that launched codex with a prompt and got a reply.
`TestQALiveSkillDiscoveryPerRuntime` launches no session on any runtime. Its
grok probe is `grok inspect` — "Show the configuration Grok discovers for this
directory" (`grok inspect --help`) — which ADR 0007 §4 already names as the
instrument that "asks `grok inspect` what the persona would have had", and
which `docs/notes.d/notes-personas.md` calls "the zero-cost probe here". The
whole run is 1.5-3.1s wall and reaches no model. No authorization was asked
for and none was needed; a bead that asks for one it does not need buys a
round trip and leaves a clock red while it waits.

## MEASURED 2026-10-31, `grok 1.0.52 (0920c8169e27) [stable]`, darwin 25.6.0, this box

grok was installed 2026-10-11 and is pinned there — `~/.grok/config.toml` has
`auto_update = false`, `maximum_version = "1.0.52"` and one version in the
Caskroom — so the binary under every reading below is the one every launch on
this box uses.

**The cwd root holds.** `<cwd>/.agents/skills/<name>/SKILL.md` is read as
`project` scope, the symlink out of the repo is still followed, and a skill
the PID never bound is still not read. Both a real `SKILL.md` and a symlink
whose target is a sibling tree outside the repo were read, separately.

**And it is gated on folder trust.** The same fixture in a folder that
`~/.grok/trusted_folders.toml` does not mark trusted reads
`Project trusted: no` and `Skills (None)` — silently, with nothing said about
why, and with the binding fully realized on disk. Four readings:

| cwd | `~/.grok/trusted_folders.toml` | `grok inspect` |
|---|---|---|
| fixture repo, real `SKILL.md` | no entry | `Skills (None)` |
| fixture repo, symlink out | no entry | `Skills (None)` |
| fixture repo, real `SKILL.md` | entry, `trusted = true` | `Skills (1)  qm-alpha  project` |
| fixture repo, symlink out | entry, `trusted = true` | `Skills (1)  qm-alpha  project` |

Three things that are easy to get wrong about that table:

- **The key must be the path grok RESOLVES.** A `t.TempDir()` repo is
  `/var/folders/...` and grok reports it as `/private/var/folders/...`. An
  entry written with the unresolved spelling matches nothing and reads
  **exactly** like no entry at all — `Project trusted: no`, no warning. That
  cost the first patched run of the pin, which failed identically to the
  unpatched one.
- **It is session setup, not an `inspect` display rule.** `session/new` over
  `grok agent stdio` emits its own setup trace, and the phases are
  `auth`, `resolve_workspace`, **`folder_trust`**, `plugin_registry`,
  `mcp_merge`, `persistence_init`, `spawn_session_actor`. (That run used a
  fresh `$HOME`, so it stopped at `Authentication required` before any skill
  list; with the operator's real `$HOME` a caged seat cannot run it at all —
  `Managed policy could not be verified: the policy lock file under
  ~/.grok is not accessible`, the same seatbelt wall ranger-base-6r1rf hit on
  bob. So the session-level skill list is UNMEASURED; the phase name and
  `inspect`'s own answer are what there is.)
- **`permission_mode = "always-approve"` does not bypass it.** The
  operator's `~/.grok/config.toml` carries it, and the pin's grok probe ran
  with that real `$HOME` and still read `Skills (None)`.

**Trust is inherited by a git worktree.** `grok inspect` from
`~/.posse/worktrees/posse/holden-posse-ranger-base-9f2o6` reads
`Project trusted: yes` although `trusted_folders.toml` holds no entry for that
path and no trusted ancestor of it — `~/src/posse` is trusted and the worktree
is a worktree of it. Confirmed under a fresh `$HOME` trusting only a fixture
parent repo: a `git worktree add` checkout at an unrelated path read
`Project trusted: yes` there too. A plain subdirectory of a trusted folder is
trusted as well, and discovery there reads the SUBDIRECTORY's
`.agents/skills`, not the git root's.

So every grok launch on this box gets its bound skills today, and the reason is
one entry in the operator's trust table that posse does not write, does not
read and does not declare.

## Changes

- `internal/posse/runtime.go`: grok's `SkillsVerified` moves
  `"grok 1.0.5"` → `"grok 1.0.52 (0920c8169e27) [stable]"`, the VERBATIM first
  stdout line of `grok --version`, which is what `ProbeCLIVersion` compares.
  The bead's note on the declared form resolves the only way it can: `grok
  1.0.5` could never have matched a 1.0.52 that prints a build hash and a
  channel, so whatever it was abbreviated from, it would have held the §6c
  clock red forever. Whether 1.0.5 printed a bare line or the declaration was
  shortened by hand is UNRESOLVED and now unfalsifiable — the binary is gone
  and `grok update` to a pinned-out version is not a thing a QA seat does to
  the operator's box.
- `internal/posse/skillsrules_parity_qa_test.go`:
  `TestQALiveSkillDiscoveryPerRuntime/cwd-runtimes` redirects grok's `$HOME` to
  a fixture that trusts the fixture repo (`qmTrustedGrokHome`, both path
  spellings), and a new arm pins the precondition itself — same realized
  binding, untrusted `$HOME`, nothing read. Mutation-tested: hand that arm a
  trusted `$HOME` and it reds on both bound skills.
- Handoff: **ranger-base-kwbh1** (`-l code -l debt`,
  `discovered-from:ranger-base-9f2o6`) — posse neither seeds nor declares the
  trust precondition, and `internal/posse/trust_test.go:713` asserts the
  opposite in prose ("grok needs none"). What to do about it is the code
  lane's call, not QA's.

## The pin was red, and a close said it was green

`TestQALiveSkillDiscoveryPerRuntime/cwd-runtimes` fails for grok on this box
without the trust seed — measured here before the fixture change. ranger-base-
bdias's Verified section (2026-10-12) records
`RHQ_PARITY_LIVE=1 go test -tags posse_arm3 -run TestQALiveSkillDiscoveryPerRuntime
./internal/posse/` as "green, re-verifying the cwd+symlink surface on 0.153.4",
and grok was already 1.0.52 on that date (installed 2026-10-11, pinned). The
tag is right, so the arm did compile and grok was on `$PATH`.

Two readings and this bead cannot separate them: the box's trust table held an
entry covering the temp root then (its `trusted_folders.toml` was itself
rewritten 2026-10-12 23:30, and what it held before that is not recoverable),
or the grok half of that run was not read before the line was written. Either
way the sentence is wrong about this box now. It is RECORDED rather than filed:
bdias's scope was codex, its codex readings stand, and correcting a Verified
sentence in a closed bead's notes changes nothing a process does or a pin
proves (ADR 0006 §6). The pin itself is the fix, and it is in this bead.

## Not fixed here

bob's half of the same drift (`2.0.5` declared, `2.0.7` installed) is the
operator's **ranger-base-zneyv**. `posse runtime drift` still exits 1 for it,
which is the clock working.
