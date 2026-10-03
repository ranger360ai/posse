## The skills tree at a session dir of `$HOME` is accepted, and the mode file beside it is not (ranger-base-q114b)

2026-10-03. ranger-base-se81d guarded ADR 0062's persona-mode file against a
session dir that is the operator's home, and named this bead as the sibling
down the same `dir` default — the one that had **already landed**. The answer
is **no guard**, and this is the reasoning, because "either answer is a
landing" means the one that was taken has to be readable later.

### 1. The input, and why nothing has to be misconfigured to reach it

`posse new` has no worktree option at all (`grep -n Worktree
cmd/posse/main.go` → nothing; only `dispatch.go` and `relaunch.go` ever set
`o.Worktree`). So the session dir is `--dir`, else `default_dir`, else
`$HOME` — `herdrback.go:1973`, `a.CfgGet("default_dir", os.Getenv("HOME"))`.
One interactive `posse new --runtime codex -a <persona>` with no `--dir`, on
an install that never pointed `default_dir` anywhere, names the home as the
workspace.

`~/.agents/skills` exists on this box, created 2026-09-05 (ranger-base-4mrmc
F2). That is this path, taken.

### 2. Why it is accepted

The mode file's hazard is that **one relative path means two scopes**:
`.bob/plugins/posse/custom_modes.yaml` is a workspace file under a workspace,
and a member of bob's GLOBAL modes glob under the home. The write means
something at the home that it means in no other directory, so the
materializer refuses before it reads anything (`refusePersonaModeGlobalWrite`,
personamode.go).

`.agents/skills` has no such second scope. MEASURED 2026-08-18
(rangerhq-1qd, ADR 0007 §2, codex-cli 0.147.0 and grok 1.0.5): *"Discovery is
at the cwd only: it does not climb to the repo root"*; grok reads
`<cwd>/.agents/skills/` as `project` scope. bob's own row is cwd-discovery
too — *"reads `.agents/skills` from cwd"*, ADR 0060's MEASURED table — and
the no-climb half of that is not separately measured for bob, which does not
change the reading below because every reader of this path is a cwd reader.

What matters is that **each of the three CLIs has its own global skills root
and none of them is this path**: `~/.codex/skills`, `~/.grok/skills`, and
`~/.bob/skills` under bob's state dir (ADR 0060's `state_dir` row: "settings,
db, logs, skills"). Nothing reads `~/.agents`. So `~/.agents/skills` is read
by a session whose cwd is exactly `$HOME`, which is the session that wrote
it, plus later cwd-at-home sessions of those three CLIs.

And "plus later sessions" is not an escalation either, because it is this
surface's declared contract at **every** dir. ADR 0007 §4 and
`RenderAgentsSkills`'s own rules: the dir belongs to the repo and not to the
persona, binding is additive, a launch leaves another persona's links alone,
and a name dropped from a PID keeps its link where another persona still
binds it. `~/src/posse/.agents/skills` outlives its seat in exactly the same
way. `$HOME` is a directory under that contract.

What is bound there is also the operator's own `RHQ_HOME/skills/<name>`, by
name, as an additive capability — not a system prompt the session is
guaranteed to load, which is what a `roleDefinition` is.

### 3. The seam: this is a fact about the DIR, not about this writer

Four writes land INSIDE the session dir, all from `CreateSession`: the
pre-push hook and the commit-guard hook under its `.git/hooks`
(`InstallPrePushHook(dir)`, `InstallCommitGuardHook(dir)` — `installHook`,
gates.go), the skills tree plus its `.git/info/exclude` line (2344), and the
mode file (2355). Two more are keyed on the dir without writing into it: the
L3 hooks redirect, which renders under the state dir and aims
`core.hooksPath` at it, and claude's directory trust, which writes a
dir-keyed entry in claude's own config. The gates dir is per-AGENT under the
state dir and is not one of these. A guard inside one of the four refuses
that one and lets the rest run.

That is observable rather than hypothetical, because the skills render comes
**first**: a `posse new --runtime bob` with no `--dir` writes
`~/.agents/skills/` and *then* refuses at the mode file. ranger-base-se81d's
refusal does not mean "wrote nothing"; it means "wrote everything up to the
mode file". Pinned as
`TestQAAtTheHomeTheSkillsTreeIsWrittenAndTheModeFileRefuses`.

So if a session whose dir is the home should be refused, that is **one rule
where the dir is resolved**, and under it is the root cause both beads share:
give `posse new` a tree of its own (ranger-base-er6mt option 2). Cheap as a
patch, not cheap as a decision — ranger-base-er6mt priced it the same day and
did not file it: it changes the crew model ADR 0008 chose (a crew session is
the operator's own conversation in the operator's own checkout), nothing
MEASURED demands it, and the `$HOME` case the exposure would be box-wide in is
already refused by name for the mode file (ranger-base-se81d). So this
fragment is the root-cause note, not a proposal.

### 4. The already-created tree is not the operator's to remove

For a stronger reason than the mode file's.

ranger-base-se81d declined to sweep `~/.bob` on the grounds that deleting a
file posse cannot prove it wrote is a larger liberty than the write being
undone. Here the reason is simpler: **the write was right.**
`~/.agents/skills` is a correct skills tree for a session whose cwd was the
home. There is nothing to undo. It is the operator's to keep or remove as
they would any other session dir's tree, and posse will keep writing it for
any session launched at the home.

### 5. The pins, and the one fact that would flip this

`internal/posse/skillshomedir_qa_test.go`, in suite arm 1 (the default
build, so a bare `go test ./internal/posse` runs it too):

| arm | claim |
|---|---|
| `TestQASkillsTreeAtASessionDirThatIsTheHomeIsWrittenOnPurpose` | the writer AND `RenderSkillsFor` (codex, grok) render `$HOME/.agents/skills`, the skill is reachable through the link, and the tree stays out of `git status` in a home that is a git repo |
| `TestQAAtTheHomeTheSkillsTreeIsWrittenAndTheModeFileRefuses` | bob at one session dir: skills written, mode file refused — the asymmetry and the order, in one test |
| `TestQANoRuntimeDeclaresAGlobalRootOverTheAgentsSkillsTree` | no runtime's `PersonaModeChannel.GlobalRoot` covers `AgentsSkillsPath`'s first segment |

The third arm is the conditional. The decision rests on `~/.agents` having no
global reader; the day a CLI declares `.agents` as its own config root, the
write at the home becomes the mode file's case and wants the mode file's
refusal. Inside the tree that shows up as a `GlobalRoot` declaration, and
that arm reds on it rather than letting the blessing above go quiet.

Every other skills pin hands the writer its own temp dir as the session dir,
which is the one input under which all of this is invisible — so these three
set the process `$HOME` and pass it as the dir, the way
`TestQAPersonaModeRefusesAWriteIntoTheCLIsGlobalConfigRoot` does.

MUTATION-CHECKED 2026-10-03, this tree:

- a `dir == $HOME` refusal added to `RenderAgentsSkills` reds arms 1 and 2.
- `refusePersonaModeGlobalWrite` handed an empty home (its own disable
  hatch) reds arm 2's mode half alone.
- `bobPersonaMode.GlobalRoot` changed to `.agents` reds arm 3 alone.
