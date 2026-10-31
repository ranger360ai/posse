# The typed line pins the runtime-home var, and the refused write was never the lock (ranger-base-49v2h)

2026-10-31. ADR 0077 D1, surface 1, built: the typed runtime line now carries
the runtime-home variable posse resolved, in both arms, on every path that
types one. Three things are worth keeping that the ADR could not have said in
advance.

Measured against **claude 2.1.316, codex-cli 0.153.4, grok 1.0.52, bob 2.0.9,
go1.26.5 darwin/arm64, this box**, in this bead's own session worktree.

## 1. The variable is DECLARED, so the pin cannot name one posse does not read

`Runtime.HomeVar` (runtime.go) is the declaration — `CLAUDE_CONFIG_DIR`,
`CODEX_HOME`, `GROK_HOME`, and nothing for bob — and `RuntimeHomePin`
(runtimehome.go) is the one renderer. It goes in `WrapWithGates` beside
`GatePrefix` rather than at the call sites, because there are three of those
and they must not drift: `planLaunch`, `RelaunchAgent` and the runtime probe.

The reason it is a field and not a `switch rt.Name` is ADR 0017 §3, and the
reason it is BUILT-IN ONLY is one axis over from `ReportsOnRex`'s: the pin's
whole guarantee is "the seat reads what posse read", and posse reads a CLI's
home through a resolver it ships (one per CLI, ranger-base-x5cbz). A declared
runtime could name a variable and could not supply that resolver, so the yaml
key would let a profile pin a name on the line while every posse-side reading
of that CLI still went to `$HOME` — the p84 shape, a lever that reads as
connected and is not.

`runtimeHomeOverride` is the single read of those variables, and all four
posse-side readings now take it: `ClaudeConfigDirIn` (trust.go),
`codexHomeIn` / `grokHomeIn` (interstitial.go), `ClaudeConfigFile`'s
no-home early return, and `credentialDirNamed`'s named-ness bit
(credential.go). That is what makes "one resolver, two consumers" true rather
than asserted — before it, "in force" was spelled five times.

## 2. In the measured cage the refused write was the TEMP FILE, and the lock was granted

The ADR records c33j6's first attempt failing on a `.posse-seed-*` temp file
in `$HOME`, and the obvious fixture for that — chmod the config's directory
0500 — stages the **wrong failure**. MEASURED while building the pin:
`SeedClaudeTrust` takes `lockClaudeConfig` BEFORE it writes, and on an empty
unwritable directory the lock's `O_CREATE` of `<cfg>.posse-lock` is what
fails first, so the test read the lock's error and never reached the write at
all.

In the real cage the ordering is the other way round, and it is not an
accident. `SeatbeltSiblings` grants `<p>.` plus anything for every declared
state **file** (ranger-base-cypy1), and `~/.claude.json` is one — so
`~/.claude.json.posse-lock` is inside the seat's writable set by
construction, while `$HOME/.posse-seed-*` is a name in the config's
*directory* rather than beside its file and is in nobody's grant. The lock
takes; the temp file is refused. That is exactly what c33j6 saw.

So the fixture plants the config and its lock sidecar first and chmods the
directory 0500 afterwards: opening an existing file for write needs no write
permission on its directory, so the lock still takes and `os.CreateTemp` is
the one call that fails. The arm skips rather than passes for a user who can
write a mode-0500 directory (root in a container), because a test that
silently measured the success path is worse than one that says so.

Consequence: `claudeTrustWriteRefusal` is scoped to the write, which is where
the bead put it, and the lock-open error is left as it was. There is no gap
behind that choice — a posse-launched caged seat cannot have the lock be its
refused write, for the grant reason above.

## 3. The cage sentence is conditional on both facts it claims

The refusal names the cage from `RHQ_CAGE` in posse's **own** environment,
and only when the error is `fs.ErrPermission`. Reading the launcher's
environment is right here and is not the mistake ADR 0077 names: that mistake
is reading the launcher's environment to decide something about the SEAT; this
decides which process just had a write refused, and the launcher's
environment is the only place that fact lives. A disk-full or a read-only
volume gets the paths and the two ways out without being told it is a cage
(pinned both ways).

## 4. What it costs on a line with a cliff

The fragment is 34 bytes in the unset arm for claude (27 for codex, 26 for
grok) and `len(value) + 21` in the set arm — 46 bytes for this box's own
`CLAUDE_CONFIG_DIR`, whose value is a 25-byte path. `PaneLineMax` is 1023, and
every persona's line on this box has rendered past it since 2026-09-01
(paneline.go), so the cost is one more line over the cliff and into
`state/launch/<session>.sh`, never a line that is lost. Rex blocks are the
same tty and the same spill.

## What this does NOT prove

The bead's live clause — a dispatch from a claude seat opening with no trust
dialog — needs a seat, and the pins here are posse's half of it: the line
carries the variable in posse's arm, and a process whose environment is that
line's resolves the file posse seeded (asked by running the fragment under
`sh -c` against a decoy value, which is the only way to tell an effective
assignment from a string that looks like one). Whether the seat's claude then
opens promptable is ranger-base-c33j6's own verify to re-run.
