## codex 0.153.4 re-measured: cwd root and symlinks hold, and it now reads `~/.agents/skills` as a global root (ranger-base-bdias)

ADR 0007 §6c flagged codex as in drift the day it shipped: `SkillsVerified`
named `codex-cli 0.147.0`, the installed binary on this box was `0.153.4`
(ranger-base-q1w0h's Evidence, 2026-10-12). §6's Evidence also left one
question ASSUMED: whether codex now reads `~/.agents/skills` as a GLOBAL
root — the shipped binary's strings were inconclusive. This bead re-runs
the live discovery measurement against the installed CLI and answers both.

### Method

MEASURED 2026-10-12, `codex-cli 0.153.4`, darwin 25.4.0, this box.

**cwd root + symlink-out** (re-verifying §2's 2026-08-18 measurement against
the newly installed version): `TestQALiveSkillDiscoveryPerRuntime/cwd-runtimes`
(`internal/posse/skillsrules_parity_qa_test.go`, tag `posse_arm3`), run with
`RHQ_PARITY_LIVE=1`. The fixture renders a PID's bound skills into
`<repo>/.agents/skills/` as symlinks whose targets live outside the repo
entirely (`a.SkillsDir()`, a sibling temp tree), then runs
`codex debug prompt-input hi` with `cmd.Dir` set to the repo and checks the
dumped prompt for the bound skills' names and the absence of an unbound one.
PASS — codex-cli 0.153.4 still reads `<cwd>/.agents/skills/<name>/SKILL.md`
only, still follows the symlink out, still ignores the unbound skill.

**`~/.agents/skills` as a global root** (the ASSUMED question): no existing
pin asks this, so it was probed by hand with `codex debug prompt-input`,
which dumps the rendered prompt — including codex's own `### Skill roots`
table — without spending a turn. Three runs, each a fresh `$HOME` and a
fresh, unrelated git-initialized cwd (never nested under that `$HOME`):

1. `$HOME/.agents/skills/qm-global/SKILL.md` present, cwd has no
   `.agents/skills` at all. The dumped prompt's skill-roots table carries an
   `r0` entry equal to `$HOME/.agents/skills` (ahead of `r1`, codex's own
   bundled `$HOME/.codex/skills/.system`), and `qm-global` is listed as
   `file: r0/qm-global/SKILL.md`.
2. Control: identical setup minus the skill — no `.agents/skills` anywhere.
   The skill-roots table carries only `$HOME/.codex/skills/.system`; no
   `.agents/skills` root is listed at all. So entry `r0` in run 1 is not a
   root codex always lists — it appeared because the directory exists at
   `$HOME`, not because of anything at the cwd.
3. Union check: a skill at `$HOME/.agents/skills/qm-homeonly/` AND a
   different skill at `<cwd>/.agents/skills/qm-cwdonly/`, cwd and `$HOME`
   unrelated siblings. Both roots are listed (`r0` = home, `r2` = cwd) and
   both skills appear in `### Available skills`. Not override — union, the
   same shape bob already declares for this exact path.

Recipe kept at `scratchpad/codex-global-probe/` for this session; not
committed (a throwaway fixture, not evidence anyone downstream reads a path
for).

### Changes

- `internal/posse/runtime.go`: codex's built-in `SkillsVerified` moves
  `"codex-cli 0.147.0"` → `"codex-cli 0.153.4"` (§6c's remedy for drift is a
  re-measurement, not a refusal) and gains
  `SkillsGlobalRoots: []string{AgentsSkillsPath}` (§6e) — codex joins bob as
  a declared owner of `~/.agents/skills`. grok is untouched: this bead's
  scope is codex only, and grok's own live probe stays ASSUMED.
- `internal/posse/skillshomedir_qa_test.go`, ARM 1: `builtinRuntimes` is
  swept in declaration order (claude, codex, grok, bob) and
  `refuseSkillsGlobalWrite` returns on the first match, so with codex now
  declaring `.agents/skills` *before* bob in that order, the refusal a codex
  or grok render at `$HOME` gets now names `codex`, not `bob`. Updated the
  arm's comment and its expected-substring assertion to match — this is
  exactly the "a CLI declaring `.agents` as its own config root" case ADR
  0007's old Consequences paragraph said would red an arm here rather than
  drift quietly.

### Verified

`go build ./...`; `go test -run
'TestQASkillsTreeAtASessionDirThatIsTheHomeRefusesUnderADeclaredGlobalRoot|TestQABobsSkillsTreeAtTheHomeRefusesUnderItsOwnGlobalRoot|TestQAEveryDeclaredSkillsGlobalRootRefusesTheCwdRender'
./internal/posse/` — all three green with the new declaration in place, and
confirmed the ARM 1 assertion reds against the pre-fix message (names
`codex`, not `bob`) before the test file was updated, then goes green after.
`RHQ_PARITY_LIVE=1 go test -tags posse_arm3 -run
TestQALiveSkillDiscoveryPerRuntime ./internal/posse/` — green, re-verifying
the cwd+symlink surface on 0.153.4. `go run ./cmd/checkorphans` — clean.
