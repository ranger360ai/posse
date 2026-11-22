# The re-plant hazard cannot reach a `#!` script, so 361 call sites are not exposed (ranger-base-7mfnf)

ranger-base-7mfnf, 2026-11-22, an audit filed by ranger-base-nywsk's close and
not a defect. `WriteExecutable` plants through `os.WriteFile`, which opens
`O_TRUNC` in place, so the question was: which call sites re-plant a path they
already exec'd, and are they exposed to the ETXTBSY race
docs/notes.d/ranger-base-nywsk.md measured?

**The answer is none, and the reason is better than "nothing re-plants".**
Sixteen sites do re-plant one path. Every one of them plants a `#!` shell
script, and the hazard does not reach a shebang plant at all.

Environment for every reading here: Linux 7.2.15-1-omarchy, 16 cores, go
1.27.3, one-minute load average given per table. **These are linux readings.**
execwrite.go's own comment records that darwin does not enforce the exec-side
check at all (MEASURED 2026-09-06), so neither arm below can fire there; the
finding is at worst the same on darwin, and it carries to ci.yml's ubuntu legs,
which run this kernel's behaviour.

## The mechanism: binfmt_script hands the deny back before the interpreter runs

ETXTBSY on a **write** means some process still holds that inode as its
executable *text image*. A `#!` file is never that. `execve` on a script is
handled by `binfmt_script`, which reads the interpreter line and then releases
its write access to the script before handing off — the file the kernel goes on
to hold as text is the **interpreter** (`/bin/sh`, an ELF), not the plant. So
the deferred `fput` that ranger-base-nywsk measured has no script to be
deferred about.

MEASURED 2026-11-22, loadavg 2.98-3.73. Each iteration plants, execs, `Wait`s
(so the child is reaped), then writes over the same path immediately and
records the errno — the same write in both arms, so the only variable is the
file's format:

| arm | n | ETXTBSY | rate |
|---|---|---|---|
| ELF (positive control) | 400 | 348 | 87.00% |
| `#!/bin/sh` | 400 | 0 | 0% |
| ELF (positive control) | 2000 | 1894 | 94.70% |
| `#!/bin/sh` | 2000 | 0 | 0% |

0 of 2400 for scripts, against 93-95% for a real Go ELF through the identical
write. The control is the load-bearing half: without it the script arm's zero
would measure the probe's insensitivity rather than the kernel's behaviour.

The control's rate is far above ranger-base-nywsk's 1.5-2.3%, and that is the
same hazard and not a different one — that probe went through bash and paid a
fork (~1.7ms) between the reap and the write, where this one issues the write
from Go with nothing in between. The rate is a property of how soon the write
follows, which is why "retried flat out it cleared in 8-22 attempts" there and
why a tight loop here nearly always loses the first attempt.

## The census

Over the whole tree, by AST (`go/ast`, not source text — see "What a regex got
wrong" below): **361 `WriteExecutable` call sites in 148 files**, up from the
345 MEASURED 2026-11-11 under ranger-base-in8dr.

A path written once in a process's life is not exposed whatever execs it, so
the census key is a path written **twice**. Two shapes can do that:

- **LOOP** — a plant inside a `for` body whose path is loop-invariant.
  52 of the 361 plants sit inside a loop; **49 write a distinct path per
  iteration** (the path names a loop variable, or is rooted at a `t.TempDir()`
  minted inside the body — AGENTS.md's "prefer a fresh `mktemp -d` per
  iteration", kept by construction). **3 are loop-invariant.**
- **SEQUENCE** — two plants in one function naming the same path. **13.**

Sixteen candidates, all hand-read. Three of them genuinely write a path after a
**reaped exec of that same path** — the exact measured shape:

| site | the sequence |
|---|---|
| `internal/posse/execwrite_test.go:171` | plant `p` (152) → `exec.Command(p).Output()` → re-plant `p` → exec again |
| `internal/posse/gates_test.go` `TestPrePushHook` | `InstallPrePushHook` plants `p` → 5× `exec.Command(p,…).CombinedOutput()` → re-install re-plants `p` (1096) → plants again (1099, 1112) → a real `git push` execs it |
| `internal/treepins/bdargvgate_qa_test.go:1147` | per arm: plant `dir/mutant.sh` → `run(script,…)` execs and reaps it, several times → next arm re-plants the same path |

The other thirteen are unexposed for a reason that is not the shebang one, and
it is worth recording because it is the tree's prevailing habit: **their two
writes are separated only by an in-process READ.** `probeL3Hooks` /
`probeL3HooksIn` compare identity through `l3IdentityIn`, `ReportHookWall`
reports, `planLaunch` plans, `RunGatesRenderProbe` execs its **own** renders out
of a per-call `probeScratchDir()` and never the repo's installed hooks. Nothing
execs the path between the plants. Three more of the thirteen are table-driven
closures that never both run against the same path, and two mint a fresh
`t.TempDir()` per call so the repeated path *expression* is a different path
each time.

And then the shebang finding closes all sixteen at once: `PrePushHook`
(`gates.go:2897`), `commitGuardHead` (`4283`, which `CommitGuardHook` builds
from), `chainRender`'s dispatcher and `scripts/bd-argv-gate.sh` all begin
`#!/bin/sh`.

## Nothing in this tree plants a real ELF

The complement matters as much as the intersection: **no `WriteExecutable` call
site in the tree writes genuinely compiled bytes.** Every "binary" plant is a
synthetic pattern that the kernel will not load as a text image —
`bytes.Repeat([]byte("O"), swapProbeSize)` (upgrade_test.go's swap probe),
`"\x7fELF not a script\n"` (coldexecinject_test.go:540, deliberately, so the
pass-through counter reads non-zero either way), `"OLD BINARY\n"`,
`"HALF A BINARY"`, `"stub binary\n"`. The one production plant whose bytes do
not come from a template is `copyPromotedSet`, and `PromotedPaths` is
`agents, config.yaml, recipes, runtimes, skills` — prose, config and scripts,
never a compiled binary — written once per path per run.

So the hazard is **structural and cheap to leave**, which is the branch the bead
named for this answer. No patch landed.

## Why no pin

A static matcher over this class is what ADR 0085 argued against and the
measurement now makes moot: the hazard cannot fire on a shebang plant, so a pin
would guard a race that no call site can lose. The thing a future reader needs
is not a gate but this page's first section — and the one rule worth carrying
forward is the habit the 49 cleared loop plants already keep: **mint the path
inside the loop.** If this tree ever plants a real compiled binary through
`WriteExecutable` and re-plants it after exec'ing it, the fix is
plant-to-temp-and-rename (a rename over a busy inode is never refused, which is
why gotest.sh's production `reuse()` path was never exposed), not a retry.

## What a regex got wrong

Three passes of text matching preceded the AST one, and each was wrong in a way
worth naming, because the shape of the error is the shape of the class:

1. **A loop whose path names the loop variable is safe** — 36 of the first 47
   hits. Obvious in hindsight, and it is most of the population.
2. **The path is routinely derived inside the loop BODY**, not in the header, so
   testing the path against the header's variables called three safe production
   sites unsafe (`copyPromotedSet`, `upgradeGateArm`, `RenderSessionHooks`).
3. **Fixing that by asking "does the body rebind an identifier the path names"
   is unsound in the direction that matters.** `script := filepath.Join(dir,
   "mutant.sh")` rebinds `script` every iteration to the *same value*, so the
   test cleared the one real hit in the tree. Substituting the RHS as *text*
   then corrupted the expressions it was reading (`"prepare-commit-msg"` became
   `"prepare-commit-(err.Error())"` when an identifier named `msg` was
   substituted inside a string literal).

The AST pass asks the question by value: resolve the path's root identifiers to
their nearest binding, and the path varies iff some root is the loop's own
key/value variable or is bound **inside** the loop body to a per-iteration
value. Two sub-rules earned their keep. A root bound *outside* the loop is
invariant **even when its own RHS is a `t.TempDir()`** — that call is evaluated
once, which is the whole difference between `swapControl` (fresh dir per
attempt, safe) and `bdargvgate` (one dir, re-planted). And a pure path
constructor is *transparent*: `filepath.Join`, `Dir`, `Base`, `Sprintf` pass the
question through to their arguments, while any other call is opaque and presumed
per-iteration — the fixture helpers here (`gitTempDir`, `hsSetup`,
`qaHookRepo`) each mint their own directory per call.

Two residuals the AST pass does not see, both hand-cleared here:

- **Range key/value variables are not resolved as bindings.** This left
  `gates_test.go:1988` (`for path, body := range map[string]string{…}`) reading
  as invariant with respect to its *outer* loop; `repo := gitTempDir(t)` is
  inside the outer `t.Run` body, so the paths differ and the row is a false
  positive.
- **The SEQUENCE arm matches by path EXPRESSION**, so two writes to one path
  spelled two different ways would be missed. The 54 functions with more than
  one plant were read for this; the pairs plant visibly different basenames.

Both residuals only ever add or drop rows inside a 16-row hand-read set, and the
shebang measurement decides all sixteen the same way, so neither changes the
answer.

The census tool and the two-arm probe stayed in the session scratchpad: the
tables and the mechanism above are the part worth keeping, and a tool pinned to
a hazard that cannot fire would be a third thing to maintain. The probe is
twenty lines — plant, exec, `Wait`, write, record errno, once per format — and
re-deriving it from this page is cheaper than keeping it green.
