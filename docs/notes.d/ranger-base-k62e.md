## The root codexWritableRoot cannot resolve (ranger-base-k62e)

The residual `ranger-base-c02a`'s fallback leaves, found verifying that close
(`ranger-base-a5ug`) and not an escape from it: c02a's FIX section prescribed
the fallback, and all four of its pins still hold.

`codexWritableRoot` → `resolveExisting` resolves the longest EXISTING prefix
and re-joins the rest. A component that does not exist cannot be a symlink, so
the re-joined tail is normally symlink-free — but a component `EvalSymlinks`
CANNOT resolve is walked past and re-joined verbatim, and it can still be a
symlink:

- a **dangling** link (the reachable one: move the tree it points into),
- a **loop** (`a -> b`, `b -> a`),
- a component whose parent cannot be read.

The rendered root then still has the symlink component codex refuses, and
codex reports it at COMMAND-RUN time inside a session nobody reads — which is
the whole of c02a: a session that comes up, reads its prompt, and then fails
every tool call, while herdr calls it working and then idle.

**MEASURED 2026-09-05, codex-cli 0.150.1** (`codex sandbox`, no model turn, no
money — `CODEX_HOME` honored, so the probe reads no live state):

```
writable_roots=["…/dangle"]            Error: writable root …/dangle contains
writable_roots=["…/dangle/sub"]        symlink component …/dangle; symlinked
                                       writable roots are not supported
writable_roots=["…/real","…/dangle"]   the same error, exit 1
```

So a dangling root is refused exactly like the resolvable symlink c02a
measured, final component or not, and **one bad root refuses the whole set**,
before any sandbox is applied. The session runs no command at all.

**Which component codex names**, measured in the same run (`l1` a symlink to a
real dir): `l1/sub` → `…/l1`, `l1/l2` → `…/l1/l2`, `l1/d` → `…/l1/d`. It names
the DEEPEST symlink component, so `symlinkComponent` does too — a root posse
renders can only carry one (everything above the failing link is replaced by
its real path, and nothing below it exists), but a PID's own `command:` can
carry two, and naming `/var` at an operator who has to repair `…/l1/d` helps
nobody.

**The shape chosen: (b), refuse the launch.** The bead offered warn / refuse /
leave-and-pin. What would open can run nothing, so it is PIDVoided's case, not
a degrade: "not a weaker persona session, a different one". A warning would
land in the dispatch log — one layer closer than the session, still a dead
pane herdr calls working. Dropping the root instead stays wrong for the reason
the code comment already says: that is ranger-base-0fb's silent cage.

`writableRootRefusal` is asked of the rendered LINE at both sites that
render a persona line (`planLaunch`, `RelaunchAgent`), like `PIDVoided` and
ADR 0053 D1's `{model}` check and for the same reason: the roots that reach
the line are not the roots handed to `Realize` (`-s read-only` renders none of
them). Gated on `SelfSandbox`, because the refusal is the CLI's and not the
flag's — claude takes `--add-dir` too and does not care, and on macOS its line
routinely carries one (`/var`).

**`codexReachRow` had a false pass over the same fact** and now refuses first:
it asked membership ("is the store under some root") and read a root codex
will refuse as an ordinary grant, so `posse gates` printed "reachable" over a
line that runs no command. At launch the row speaks first and
`--allow-degraded` waives it; the launch refusal holds past the waiver, which
is the point of having both.

**What is already loud, so nobody re-files it:** the memory dir. The bead's
"one `mv` away" route — `~/.config/posse/personas` dangling — never reaches
the render, because `EnsureMemoryDir` runs first and `MkdirAll` refuses a
dangling component (`mkdir …: file exists`, measured both for a dangling
parent and a dangling final component). The roots that DO reach the line
unchecked are the git dirs: `beadsGitDirs` names `<store>/.git`
unconditionally and `LinkedGitDirs` passes git's own answers through, and that
is the shape the launch pin uses.

**Pins** (`internal/posse/codexwritable_test.go`, beside c02a's):
`TestCodexRefusesADanglingSymlinkWritableRoot` (the root still renders AND the
line is readable as refused — dangling, under-a-dangling-link, a loop, and the
two-component ordering), `TestCodexAcceptsTheRootsItResolves` (the control:
c02a's resolvable link, an unborn root, `-s read-only`),
`TestOnlyASelfSandboxingRuntimeRefusesASymlinkedRoot` (claude is not refused),
and `TestCodexLaunchRefusesAWritableRootCodexWouldRefuse` — the LINE, with the
row's refusal, the refusal past `--allow-degraded`, and the repaired-link
control that launches. Seven mutants, six killed: the refusal no-op, `Stat`
for `Lstat`, the dropped `SelfSandbox` gate, the row guard off, "every root is
refused", and outermost-for-deepest. The survivor is the `IsAbs` guard in
`symlinkComponent` — a relative root is not judged, because it would be
resolved against THIS process's directory and not the session's; pinning it
needs a `t.Chdir`, which in a package this parallel buys a flake for a
defensive guard.
