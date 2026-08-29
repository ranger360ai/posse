## codex refuses a writable root with a symlink component (ranger-base-c02a)

Measured live 2026-08-29 on codex-cli 0.150.1, during the live half of
ranger-base-i0qp. Every codex session posse launched on the operator's box
could run **no shell command at all**, and did so silently: the session came
up, herdr called it working and then idle, dispatch reported "review", and the
bead sat `in_progress` with zero files, zero commits and zero comments. The
claude arm of the same bead in the same minute did the whole task — so it was
the runtime, not the fixture and not the prompt.

**The mechanism.** `realizeCodex` turns the persona's memory dir into a
writable root:

```
codex … -s workspace-write --add-dir '~/.config/posse/personas/<persona>' …
```

`~/.config/posse/personas` is a symlink into the constitution tree, and codex
refuses a writable root that has a symlink **component**:

```
Error: writable root …/.config/posse/personas/<persona> contains symlink
component …/.config/posse/personas; symlinked writable roots are not supported
```

Two properties make it lethal rather than loud:

- it is refused at **command-run time, not at launch**, so the model starts,
  reads its PID and its work prompt, and then every single tool call dies;
- dispatch has no turn-outcome reader for codex (`Record: RecordUntrusted`,
  cost.go), so "did nothing" and "did the work" look the same from outside.

**The repro is free and deterministic** — `codex sandbox` runs one command
under a session's sandbox, no model turn, no money:

```
codex sandbox -c 'sandbox_mode="workspace-write"' \
  -c 'sandbox_workspace_write.writable_roots=["<symlinked path>"]' -- /bin/echo hi
# Error: … contains symlink component …
codex sandbox -c 'sandbox_mode="workspace-write"' \
  -c 'sandbox_workspace_write.writable_roots=["<resolved path>"]' -- /bin/echo hi
# hi
```

`CODEX_HOME=<tmp>` is honored, so a probe reads and writes no live state.

**The fix** is one place: `realizeCodex` puts every root through
`codexWritableRoot` → `resolveExisting` (the seatbelt's own primitive, which
resolves the longest existing prefix and re-joins the rest, so a root named
before the launch materializes it still lands). All roots, not just the memory
dir: the store of record and the git dirs happen to be real paths on this box,
which is the only reason nothing else broke, and the next symlinked
constitution path would kill the lane the same silent way.

**Resolving costs the session nothing** (measured, same bead): with the REAL
path granted, a write through the symlinked spelling still lands — the sandbox
matches real paths. So `{memory}` and `RHQ_PERSONA_DIR` go on naming the path
the operator typed; only the sandbox root moves. `underDir` already resolves
both sides, so ADR 0013 §4's reachability row judges the resolved line against
literal targets unchanged.

**Pins.** `TestQACodexAcceptsOnlyAResolvedWritableRoot` is the live one: it
runs `codex sandbox` over all three arms (the refusal, the acceptance, the
write through the link) and skips loudly if a later codex stops refusing.
`TestCodexResolvesASymlinkedWritableRoot` pins the renderer,
`TestCodexRendersAWritableRootThatDoesNotExistYet` its wrong arm (an
unresolvable root renders, never drops — dropping is the silent cage of
ranger-base-0fb), and `TestCodexLaunchLineResolvesASymlinkedPersonasDir` pins
the LINE over the shape the box was in.
