## A caged persona cannot measure codex's sandbox (ranger-base-ejva)

`TestQACodexAcceptsOnlyAResolvedWritableRoot` (the live half of
ranger-base-c02a) shells out to `codex sandbox`, which ends in `sandbox_apply`.
A posse persona whose PID says `cage: seatbelt` is **already inside** a
sandbox-exec, and macOS refuses to nest one:

```
$ sandbox-exec -f /tmp/p.sb /bin/echo nested
sandbox-exec: sandbox_apply: Operation not permitted   # exit 71
```

So arm 2 Fatal'd for every caged persona and read as posse being broken. It was
the only failure across all three packages.

**Why arm 1 still passed, which is what made the red look real.** codex
validates its writable roots *before* it applies a sandbox, so the control arm
gets codex's own verdict even inside a cage:

```
$ codex sandbox … writable_roots=["<symlinked>"] -- /bin/sh -c true
Error: writable root … contains symlink component …          # exit 1, codex prose
$ codex sandbox … writable_roots=["<resolved>"]  -- /bin/sh -c true
sandbox-exec: sandbox_apply: Operation not permitted          # exit 71, the box
```

(measured 2026-08-29, codex-cli 0.150.1). The two failures are different facts
wearing the same red.

**The fix** is a preflight arm 0: one `codex sandbox` run over a plainly-legal
root — `real` through `filepath.EvalSymlinks`, deliberately *not* through
`codexWritableRoot`, so a broken renderer cannot make the preflight lie — and
the whole test skips when it comes back with sandbox-exec's own line. Any other
preflight failure Fatals, with a positive witness (`echo applied`) so a run that
"succeeded" by printing nothing cannot pass it.

**The skip has to stay narrow** or it swallows the bug the test exists for. A
write the sandbox denies says `Operation not permitted` too —
`/bin/sh: /System/nope: Operation not permitted` — and that is exactly arm 3's
failure, so the discriminating half is the `sandbox_apply:` prefix, never the
errno text. `codexSandboxUnappliable` is that one predicate;
`TestCodexSandboxUnappliableSkipsOnlyANestedCage` pins it over four measured
transcripts (nested cage, codex's root refusal, a denied write, a successful
run). All four mutations die naming the right arm: dropping `err != nil` kills
the success arm, matching the bare errno kills the denied-write arm, `false`
kills the cage arm, `err != nil` kills both refusal arms.

**What is still unmeasured from inside a cage:** the preflight's success path.
That it does not skip on a healthy box rests on the predicate's pin plus the
measurement above that codex's root refusal never prints `sandbox_apply:`.
