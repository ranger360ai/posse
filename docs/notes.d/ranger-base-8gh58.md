# Admitting codex on Rex: the line, the version pin, and two ASSUMEDs that came back half-true (ranger-base-8gh58)

2026-11-06. The admission slice of ADR 0076 Amendment 2 (design bead
ranger-base-dok0a), built on ranger-base-z7kle's reporter and
ranger-base-fwwl8's trust ask. `ReportsOnRex` is true for codex now; the line
that earns it is assembled in `internal/posse/codexrex.go`, the version it was
measured on is pinned in the built-in table, and the two readings are taken at
the same two sites D1 is taken at (`dispatch.go` above the claim,
`planLaunch` as every other path's backstop).

Everything below was measured on this box on 2026-11-06 from a dispatched
`cage: seatbelt` seat: **Rex CLI 0.0.1793377904-ge6592f**, **codex-cli
0.153.4**, posse `0.5.3+dev`, darwin 25.6.0. **No model turn was spent.** The
live acceptance the bead's done-when names — a codex bead dispatched on a
herdr instance and one on a `multiplexer: rex` instance — spends billed codex
turns and is the operator's to authorize, so it is NOT in here.

## 1. `--disable hooks` has to come off WHOLE-WORD, or the guard goes blind

The Rex line is three edits to an already-rendered line: `--disable hooks`
off, the D6 declaration on, the D7 trust on after it. The first edit is the
one with a trap in it.

A plain `strings.ReplaceAll(cmd, " --disable hooks", "")` takes the same bytes
out of a line that reads `--disable hooks,mcp` and leaves `,mcp` behind. The
hook layer is still off; the refusal that is supposed to catch exactly that
cannot see it, **because the string it looks for is the string that was just
removed**. The result is a line carrying a declaration and a trust for hooks
that will never run: a seat that starts, reports nothing, and reads downstream
as a runtime that went silent — and on `prompt: argv` that is a claim, a
worktree and a billed turn.

So the removal matches only posse's own spelling, ` --disable hooks` followed
by a space or the end of the line, and **anything else is left in place** for
the refusal to find. A PID's own `command:` is the one template posse did not
write, which is why this is a refusal and not a cleverer parser.

Caught by mutation: with the whole-word rule dropped,
`TestQARexReportLineRefusesAFlagThatWouldSwitchTheHooksOffAgain` reds.

## 2. Both values are shell-quoted, and a real `/bin/sh` is what says so

The launch line is TYPED — into a pane's shell, or into the
`state/launch/<session>.sh` a line over `PaneLineMax` (1023) spills to. The
declaration contains single quotes **by construction**: codex splits a hook
`command` on whitespace with no shell and honours single quotes, so the
handler path is quoted inside the TOML string (MEASURED under
ranger-base-z7kle, probe arm `space`). A naive `'…'` wrapper would end the
shell word in the middle of the handler path.

`shellQuote`'s `'\''` idiom carries it, and single quotes also make a `$` or a
backtick in either value inert, which a double-quoted spelling would not.
What the pin asks is a real shell:

```
sh -c "set -- <the line posse would type>; for w; do printf '%s\n' \"$w\"; done"
```

and compares the words to the argv codex must receive. MEASURED: the typed
line splits to exactly `codex -a never -c <declaration> -c <trust>`, both
values byte-identical to what the ask asked about.

**What it costs the line: about 1.6 KB** (roughly 800 bytes of declaration for
eight events, and about as much again once codex has reported a sha256 per
hook). That spends headroom that was already spent — every persona line has
spilled to a launch script since 2026-09-01, 1867-2058 bytes against the 1023
limit — so the pair lands in the spill and no behaviour changes.

## 3. ADR 0076 D7's ASSUMED: the claim is released, the worktree is not

D7 said "the worktree and the claim are released by the ordinary
launch-failure path (ASSUMED that it covers this stage; the slice checks)".
MEASURED by driving a whole dispatch pass with a failing ask
(`TestQACodexRexAskRefusalUnclaimsTheBeadAndLeavesTheTreeForTheSweep`):

| what | what happens | why |
|---|---|---|
| the bead's claim | **handed back** (`--status open`) | `createSession` returns the ask's error, `launchWithPrompt` wraps it in `unclaimAfterLaunchFailure` |
| the persona/runtime slot | **benched for the pass** | a plain error, so fireLoop's busy-key split reads it as the slot's — every bead routed here meets the same refusal |
| the session worktree | **still on disk** | `EnsureSessionTree` cuts it near the TOP of `planLaunch`, above every refusal in that function, and nothing unwinds it |
| a turn | **never spent** | the refusal is on the near side of the launch |

The worktree half is true of every refusal in `planLaunch` — danger,
detection, parity, PID channel, the version pin — and for the D7 ask it cannot
be otherwise: **the ask is about the tree.** codex discovers hooks from the
cwd's project files and keys project trust by the resolved path, so there is
nothing to ask before there is a directory to ask about. The tree has no
commits, so `posse worktrees --land` removes it ("had no commits"). The cost
of a refused ask is one launch and a tree waiting for a sweep.

## 4. The darwin `/dev/tty` ASSUMED: the premise was wrong, the answer is yes

Amendment 2's Consequences: "On darwin, the hook runs as codex's child under
posse's seatbelt when the PID is caged. That the hook may open `/dev/tty`
there is ASSUMED, and the slice tests it."

**posse renders no seatbelt for codex at all.** codex declares `SelfSandbox`,
macOS refuses to nest `sandbox-exec`, and `seatbeltWallRendered` is false for
such a runtime whatever tier the PID asked for — posse reports it as a
DeclaredDifference (ADR 0017 §2) instead of wrapping. So at `cage: seatbelt`
there is no posse profile in front of a codex seat, and the bullet is moot at
that tier. `cage: container` is refused for codex today anyway, for want of a
`CageCred`.

The counterfactual is still worth the measurement, because posse's profile is
the one wall shape that COULD have refused the open: it is `(deny
file-write*)` with an allow list, unlike the probes' `(allow default)(deny
network*)`. MEASURED, with a profile `RenderSeatbelt` really rendered:

| run | result |
|---|---|
| `sandbox-exec -f <profile> /bin/sh -c 'exec 3>/dev/tty'` | **succeeds** |
| `sandbox-exec -f <profile> posse hook-report codex` (unknown event) | **exit 0** |

The grant is the profile's own `(allow file-write* (regex #"^/dev/"))`, which
outvotes the deny above it for `/dev/tty`. The pin measures the OPEN and not a
write, deliberately: a record written onto the operator's own Rex block by a
test is a side effect nobody asked for.

**Where it had to be measured is the finding underneath the finding.**
`openTTY` opens the CONTROLLING TERMINAL, and a dispatched seat's `go test`
has none — `open /dev/tty: device not configured`. So the pin
(`TestQATheHookTTYUnderACagedCodexPID`) skips its live arms there and would
have been green forever without asking the kernel anything. A Rex block has a
tty, and Rex spawns it outside this seat's cage, where `sandbox_apply` is not
refused (this box's own earlier measurement, ranger-base-z7kle). Hence
`docs/adr/0076-reporter.probe.sh` arm `tty`: compile the package test binary
out here, run it in there, read the verdict back. It printed `PASS` (not
`SKIP`), twice.

Triangulated by the two earlier slices, whose codex ran under both
`sandbox-exec (deny network*)` and codex's own `-s workspace-write` with the
hook writing records all the way through.

## 5. The version pin, and the three things D9 did not settle

`RexVerified` is built-in only, `codex-cli 0.153.4`, read as
`RexVersionDrift`.

- **An unreadable version is refused too.** D9's words are "a codex that
  reports another version"; a codex that will not answer `--version` reports
  none, and the pinned claim is "this route was measured on THIS version".
  ADR 0013 §1's "unknown is never read as no". Refusing costs one launch;
  admitting costs the stall below.
- **A native report carries no pin and that is not an omission.** claude's
  record is claude's own output: a regression shows up as D2's first-report
  wait missing, with nothing typed. codex's record exists because posse typed
  two `-c` values derived from an `[experimental]` RPC, and the likely failure
  of a moved codex is **not an error** — it is a line codex ACCEPTS, a launch
  that completes, a seat on the startup review dialog, and `prompt: argv`'s
  HOLD keeping the claim over a turn nobody can read.
- **The reader is parity.go's `--version` seam** (`skillsDriftVersion` →
  `ProbeCLIVersion`), compared through `skillsVersionRelease` (ADR 0007 §6c).
  One reader, one per-process memo: the dispatch loop pays one exec per
  process, and a codex upgraded under a running `posse dispatch --watch` is
  not noticed until the next process — the property `posse runtime drift`
  exists for.

MEASURED by mutation: with `dispatch.go`'s version arm deleted, **every pin
that called `RexVersionDrift` directly stayed green.** The pin that reds it
drives a whole pass and asserts that bd is never asked to CLAIM, which is D9's
placement rather than its message
(`TestQACodexRexVersionRefusalLandsAboveTheClaim`). The same shape as
ranger-base-metqp's finding about D2's own wait, one reading over.

## 6. Two observations this bead did not act on

- **`planLaunch` asks herdr's detection manifest on a Rex instance too.** ADR
  0076 §3 says there is no herdr detection there at all, so that reading
  describes a surface the launch will not use — and it REFUSES a bead-carrying
  launch when the manifest is missing. It costs nothing today because
  `etc/herdr/agent-detection/codex.toml` exists, which is why this is an
  observation and not a bead: an admitted runtime without a herdr manifest
  would be refused on Rex for a reason that does not apply there.
- **`posse runtime probe` renders a persona line and does not get the route.**
  Deliberate: the probe's job is to measure a runtime, not to report as one,
  and it types no work prompt. Worth knowing if a future probe arm ever wants
  to read a record.
