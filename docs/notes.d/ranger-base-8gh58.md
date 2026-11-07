# Admitting codex on Rex: the line, the version pin, and two ASSUMEDs that came back half-true (ranger-base-8gh58)

2026-11-06. The admission slice of ADR 0076 Amendment 2 (design bead
ranger-base-dok0a), built on ranger-base-z7kle's reporter and
ranger-base-fwwl8's trust ask. `ReportsOnRex` is true for codex now; the line
that earns it is assembled in `internal/posse/codexrex.go`, the version it was
measured on is pinned in the built-in table, and the two readings are taken at
the same two sites D1 is taken at (`dispatch.go` above the claim,
`planLaunch` as every other path's backstop).

Sections 1-6 were measured on this box on 2026-11-06 from a dispatched
`cage: seatbelt` seat: **Rex CLI 0.0.1793377904-ge6592f**, **codex-cli
0.153.4**, posse `0.5.3+dev`, darwin 25.6.0, and **no model turn was spent**
on any of them. Section 7 is the live acceptance the bead's done-when names —
two billed codex turns, which the operator authorized on ranger-base-x7ojx
and which ran on 2026-11-07 against the same box and the same versions.

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

## 7. The live acceptance: two turns, two multiplexers, both settled

Authorized on **ranger-base-x7ojx** (operator, 2026-11-06: two billed codex
turns, one per multiplexer, "routed as two small REAL beads per the suggested
shape"). Run 2026-11-07, two dispatch passes minutes apart against the same
repo and the same runtime, differing only in the instance's `multiplexer:`
key — which is the comparison D8 and ADR 0080 D3 are about, and the only one
that can tell a Rex finding from a codex one.

**The rig.** Two scratch homes, each holding exactly ONE codex PID whose
`labels:` match no bead in this queue, so the only thing that can route work
to either seat is an explicit assignee (`laneFor`: an assignee that
canon-resolves is a lane of one and never falls through). `instance: 8gh58h`
with no `multiplexer:` key, `instance: 8gh58r` with it; byte-identical
otherwise but for the two comments naming which is which. Both beads read
`unroutable (no assignee/label match and no default_persona)` on the LIVE
instance before and after — no live persona carries `debt`, and the live
`--watch` loop had been printing that line for both of them for days — so
nothing raced the operator's loop for either one. Passes ran `-n 2 --no-reap
--allow-degraded`.

**Both passes had to run from a Rex BLOCK and not from this seat**, for the
reason now filed as **ranger-base-m928w**: this seat is `cage: seatbelt`, its
profile does not grant the codex runtime's own home, and posse's
credential-presence probe (ADR 0019 — presence, mtime, path) reads the
resulting EPERM as ABSENT. So from inside the cage every dispatched codex
launch refuses on the signin interstitial, which is a LAUNCH REFUSE for
anything dispatched (ADR 0013 §2), saying codex has nothing to authenticate
with — while a live login is in fact present, MEASURED from a Rex block the
same minute. The caged read fails `Operation not permitted` and not `No such
file`, and that is the whole of the bug: the probe takes the failure for an
answer without looking at the errno, which is the one thing ADR 0013 §1's
"unknown is never read as no" forbids. A Rex block is spawned by the Rex
server, outside this seat's cage, which is where the reading is true; it is
the same door the `tty` arm of the probe needed, for the same reason. The
paths and their readings are instance-ops and stay in the instance tree
(ADR 0024 D3).

|  | control | acceptance |
|---|---|---|
| instance | `8gh58h`, no key | `8gh58r`, `multiplexer: rex` |
| bead | ranger-base-w4enh (P3 census) | ranger-base-7vh7y (P2, three comment words) |
| seat | `codexherdr-posse-ranger-base-w4enh` | `codexrex-posse-ranger-base-7vh7y` |
| line | `CodexFleetFlags` byte-identical, 1952 B | the three D8 edits, 4858 B |
| settle | herdr detection: `idle` | `program_status`: `idle`, `owner {pid 40297, codex}` |
| closed by | the seat | the seat |
| landed on `main` | `3bc1a6db`, REBASED | `b14420ef`, fast-forward |
| turn | 1 of 2 | 2 of 2 |

**D8's byte-identical promise, measured and not only pinned.** The control's
rendered line carries `-a never --disable hooks -c allow_login_shell=false -c
"projects={\"$PWD\"={trust_level=\"trusted\"}}"` and holds no `hooks=` and
no `hooks.state` anywhere in its 1952 bytes. The Rex line, at 4858 bytes, has
`--disable hooks` gone, `-c 'hooks={...}'` typed BEFORE `-c
'hooks.state={...}'`, and both values shell-quoted with the `'\''` spelling
section 2 measured through `/bin/sh`. Four times `PaneLineMax` (1023), so it
rode `state/launch/<session>.sh` exactly as section 2 said it would, and the
pane sourced it.

posse said so on the way past, which is the warn `RexReportLine` prints:

    posse: codexrex-posse-ranger-base-7vh7y — codex reports on Rex through
    posse's own hooks: the declaration and the trust codex itself reported are
    on the launch line and `--disable hooks` is off it (ADR 0076 D6/D7)

**The first-report gate passed, and I do not have the number.** Both logs hold
ZERO HOLD lines, zero `could not be confirmed` lines and zero refusals, so
`confirmReports` took its success path — which returns nil SILENTLY. codex
declares no `startup_wait:`, so `rt.Wait()` is `DefaultStartupWait` = 45s, and
the line was written at +0.067s after Rex created the session (uuidv7, which
carries its own millisecond stamp). A record therefore existed before ~+45s or
the argv HOLD would have printed. What I can state exactly is the record's
later life, read off Rex: `working` at **+80.4s** and `idle` at **+285.1s**
after session creation, the same root record updated in place, owner
`{pid 40297, name codex}` throughout — which is D9's `ReportsOnRex` in its
widened sense, a record whose owner is the RUNTIME, written by posse's route
on the runtime's own events.

The exact arrival time is NOT recovered, and the reason is a rig error of
mine, not a posse one: the root record is updated IN PLACE, so `updated_at`
holds the latest write and the arrival stamp is gone, and my external
stopwatch missed the seat because **posse labels a Rex session
`<instance>/<session>`** — `8gh58r/codexrex-posse-ranger-base-7vh7y` — while
the watcher matched the bare session name. Anyone re-measuring this should
match on the suffix, and should poll `block call ... program_status` from
before the seat exists; one more turn would buy the number, and this bead did
not spend one on it after the gate had already answered the question the
done-when was asking.

**What the two turns bought besides evidence.** Both seats did the work and
closed their own beads. The census seat read all SEVEN files its bead's own
scope correction names (not the five in the description), classified four as
immune, found and guarded three real cross-clock fixtures in
`internal/posse/verifybox_qa_test.go`, `cmd/posse/runtimeavail_qa_test.go` and
`cmd/posse/backupsurface_qa_test.go`, wrote its own notes fragment and
regenerated the index (ranger-base-w4enh — named as a bead and not as a path,
because that fragment landed on `main` and this branch does not carry it);
`make tree-check` green. The comment seat corrected
four stale tokens — the bead said three — and ran `make tree-check` green.
Neither needed a second prompt.

### Three things this pass found that are not about codex

- **`--no-reap` does not cover the land sweep** (observation, not filed). The
  rex pass printed `⌫ ranger-base-onwi1 ... retired` for ANOTHER persona's
  tree while `--no-reap` was on the command line. The flag names the end-of-
  pass reaper; the worktree landing sweep is a different thing and ran anyway.
  Nothing was lost — the predicate is bead-closed plus nothing-unlanded plus
  session-gone plus untouched-for-1h, and the live loop's own next pass would
  have taken the same tree — but a second instance pointed at the same repo
  does the first one's housekeeping, and the flag's name does not say so.
- **The Rex wait leg does not wait**, filed as **ranger-base-74tkp** (P2):
  295 `still working after Ns — waiting again` lines in ~3m30s over 210
  distinct one-second values, about 1.4 legs per second, against ZERO such
  lines from the herdr pass over a longer wait on the same bead class. On
  herdr a leg blocks inside herdr's own wait verb; on Rex there is no blocking
  wait verb to call, so the leg returns at once and the loop busy-waits — and
  polls `program_status` once per iteration — for the whole length of every
  dispatched turn.
- **`idle-to-next` has nothing to say on a first launch**, both halves:
  `first launch into this seat on record — no settle to measure from`. The
  same gap ranger-base-c33j6 recorded, and for the same reason: the interval
  needs a second dispatch into a seat the reaper retires with its bead.

### Where the trees went

Both seats were killed from this seat and both worktrees were KEPT, because
`git worktree remove` under `~/.posse/worktrees/<other session>` is
`Operation not permitted` from inside this cage — the same wall as the
credential probe, one directory over. Both hold nothing unlanded (their
commits are on `main`, verified by `git log --grep`), so the live landing
sweep takes them after `retire_tree_after:` (1h), which is precisely what it
did to gilfoyle's tree during my own pass. `rex ls` is back to the one
pre-existing foreign session; `go run ./cmd/checkorphans` is clean and both
suite slots are free.
