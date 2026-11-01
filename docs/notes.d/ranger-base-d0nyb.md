# A probe answers on the PATH the exec it predicts resolves on (ranger-base-d0nyb)

2026-11-01. Four probes in posse's shipped tree asked `exec.LookPath` — THIS
process's `$PATH`, the gated one — about a binary the matching call then
resolved through `protocolBin`, against ADR 0077 D3's gates-free PATH. On a
gated seat those are different questions, so the probe could answer **true**
where the resolution **refuses**: a wrong yes, a plan made on it, and the right
no arriving later. `internal/posse/herdr.go`'s `Herdr.Available` and
`internal/posse/cage.go`'s three engine probes (`ContainerAvailable`,
`cageNoBinary`, `BuildCageImage`) are the four; `ranger-base-g1xnt` is the same
defect on `Bd.Available`, which is where the class was first seen.

All four are one expression now: `resolveOutside(<name>, "") != ""`, which is
the judgement `protocolBin` itself makes for a bare name — and the *stricter*
of the two for a name that spells a path, because `resolveOutside` hands it to
`absLookPath` and so still requires the file to be there, while `protocolBin`
hands an explicit binary straight back unchecked. The three cage probes share
one helper, `engineOnPath`, so the engine question cannot drift from the engine
answer again.

Pinned in `internal/posse/probegatesfreepath_test.go`, both directions, each arm
with its own positive control on a PATH whose non-gates element *does* hold the
binary — because "unavailable" is also what a probe reading a PATH it can
resolve nothing on reports.

## The soft degrade was the casualty, not just the timing

The herdr half is worth more than "a wrong answer followed by a right one".
`KnownAgentKinds`, `AgentManifest` and `HasReportAgent` each document that an
unavailable herdr is **UNKNOWN and never a no** — a launch must not be refused
because a diagnostic verb went missing. A `protocolBin` refusal reaching
`capture()` is not a `*HerdrHangError`, so `HasReportAgent`'s one exemption
misses it and the verb reads as absent.

MEASURED 2026-11-01 (darwin 25.6.0, this worktree at main `0861d88d`), with
`exec.LookPath` restored as the mutant and PATH = one gates bin dir holding a
`herdr` shim plus one ordinary dir holding none:

```
Available()         = true          (it found the shim)
HerdrMux.Available()= true          (runtimeprobe.go's gate reads this one)
HasReportAgent()    = (false, true) <- present=false, ok=TRUE
```

`ok=true` is a confident wrong **no** on exactly the question whose doc says a
launch must not be refused for it. Answering false up front gives those callers
the abstention they were written for.

## The class is derivable, and it is measured

`ranger-base-d0nyb` asked for a census arm rather than three hand fixes. The
derivation works. The class is "a probe for a name that a shipped
`protocolCmd`/`protocolBin` site execs", keyed on the rendered **expression**
of the name and matched **per file**:

```
probe     = exec.LookPath(X)                            non-test .go under cmd/, internal/
exec site = protocolCmd(exec.Command(Y, …))              first arg
          | protocolCmd(exec.CommandContext(ctx, Y, …))  SECOND arg
          | protocolBin(Y)
in class  = some exec site in the SAME FILE renders Y identically to X
```

MEASURED 2026-11-01 by `go/ast` over 159 non-test `.go` files under `cmd/` and
`internal/`, **after** this bead's four fixes: 9 probes, 18 distinct exec'd
spellings.

| rule | result |
|---|---|
| per-FILE spelling match | 3 of 9 probes |
| TREE-WIDE match | the same 3, plus 2 **false positives** |
| matching nothing | 4 |

The three: `beads.go:51` (`b.Bin` ← `protocolBin` at `:298`),
`cageinner.go:151` (`argv[0]` ← `:356`), `seatbelt.go:45`
(`"sandbox-exec"` ← `:104`). The two extra rows tree-wide are
`cagelauncher.go:210` and `gates.go:1159`, both `LookPath(path)`, colliding with
`runtimeprobe.go:442` and `skillsdirtrust.go:168` — `path` is a generic
identifier, and a cross-file spelling match says nothing about the binary. The
four that match nothing are `backup.go:491` (`tool`), `gates.go:1149`
(`"true"`), `possebinary.go:51` (`"posse"`), `runtimepreflight.go:160` (`exe`).

So **per file** is the rule: it catches every member of today's class and
nothing else. Run against the tree *before* these fixes it also finds
`herdr.go:59` and `cage.go:266/:1277/:1430` — the four fixed here — which is the
test of a derivation, that it finds the class it was derived from.

The arm is filed and not landed (`ranger-base-372iq`, blocked), for one reason:
`beads.go:51` is a live member and is `ranger-base-g1xnt`'s deliverable, in
flight in another worktree today. An arm that reds on another lane's line is not
landable, and a roster row excusing a live defect is the door-held-open that
`protocolchildpath_qa_test.go`'s own ruling warns about.

Of the three remaining rows, exactly **one** is a false positive and belongs in
a roster: `cageinner.go:PrepareGatesWrap`. Its `argv[0]` and
`CageInnerGatesReady`'s are two different argvs — the first is what follows `--`
*inside* the container, resolved on the caller's PATH before the shims go in
front of it, which is the same ruling `pcExemptFuncs`' two `cagelauncher.go`
rows carry: that line runs in the seat, not in a launcher.

## And a fourth surface, whose wrong answer is permissive

`seatbelt.go:45` came out of the same derivation and is filed separately
(`ranger-base-npqem`) because its direction is its own decision:
`SeatbeltAvailable`'s doc deliberately asks a HOST question about a session
whose `sandbox-exec` is typed into a herdr pane. It is
`liveSandboxApplyRefusal` at `:104` that execs the same binary through
`protocolCmd` and must agree with it.

MEASURED 2026-11-01, same fixture shape:

```
SeatbeltAvailable()                = true   (it found the shim)
protocolCmd(…).CombinedOutput()    = out "", err "sandbox-exec: executable file
                                       not found outside posse's gates dirs …"
isSandboxApplyRefusal("")          = false  (it needs "sandbox_apply")
=> sandboxApplyWhy stays ""        = "THIS PROCESS may apply a profile"
```

Unlike the four above, that is the **permissive** wrong answer and it is silent:
the abstention `ranger-base-heur` added does not fire, so `reachability.go`
believes a measurement that never happened — the exact failure that bead was
filed for. The out-of-band `err` is discarded by design at that call site, since
only an apply refusal is theirs to abstain on.

## Why none of this fires on this box

The same margin `ranger-base-cwsi8` measured for `bd`, and exactly as thin: a
real `herdr`, a real `docker` and `/usr/bin/sandbox-exec` all sit on PATH
outside every gates dir, and no PID here denies any of them — so nothing renders
a shim for a probe to find, and the two readings agree by accident.
`ranger-base-zfljf` (open, P1) is the kind of change that removes such an entry.
