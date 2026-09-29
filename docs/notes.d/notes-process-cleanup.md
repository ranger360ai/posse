## Leaked gate-shell children, and `POSSE_KEEP=` (ranger-base-apwr, -gvp2p)

Nothing on this box used to end a leaked gate-shell child. A non-interactive
zsh does not hang up its background jobs, so a Bash line that backgrounds
something and returns leaves it running with launchd as its parent; the load
guard then correctly declines to launch into the wreckage and waits forever,
because the wreckage has no living parent and nothing is looking for it.
That is teau's 2h30m freeze (sixteen spinners at ~30% of a core each), and
ranger-base-k6csq's second helping (forty, ~2h, and the session that started
them ran `pkill` and *believed it had cleaned up* — `jobs -l` and a %CPU
floor are both structurally blind to that shape).

**The predicate.** A leak is: `ppid == 1`, **and** at or over
`LoadCulpritOrphanCPU` (20% of one core), **and** at or over
`LoadOrphanMinAge` (1m — long enough to clear the second a process is
briefly ppid 1 in while the shell that forked it exits), **and** its argv
*opens with* the ADR 0009 gate-shell preamble. A forked subshell never
execs, so it carries its parent's whole `-c` string: that is what makes the
preamble a reliable "this came out of a persona's Bash line" marker, and why
"opens with" is enforced at the head — a process whose argv merely *talks
about* the preamble (a `grep`, an editor, a `ps` of the report itself) is
some other process's text about ours, and calling that a leak is the teau
misreading in a new costume.

**Arm 1 (shipped) names them and kills nothing.** It rides under the load
guard's culprit line, off the same single `ps`. Two readings reach it: a pass
the guard is skipping, and — since ranger-base-fxs60 — the **guard clock**,
one tick per `--watch` interval, on its own goroutine, **whether or not the
box is over the line**. Both of those are why: under ADR 0028 §1 a rolling
`Run` does not return while a bead is in flight, so on 2026-09-02 one loop
ran 1h40m on a single pass while eight orphans burned ~50% of a core each and
nothing evaluated the guard at all; and when that loop was restarted the load
at pass start had dipped to 44 under `load_guard: 60`, so the pass was not
skipped and the same eight went unreported again. **Load is not the
predicate. The leak is.** `go run ./cmd/checkorphans` is the same predicate
without the CPU floor and without the load-spike framing, for a persona
asking "did the thing I just backgrounded leak".

**Arm 2 (shipped OFF) ends them, and `POSSE_KEEP=` is how you keep one.**
Exactly one false positive exists and nothing in the process table separates
it: a persona that *deliberately* backgrounds a long-lived CPU-consuming
process and lets the tool call return has the identical signature. The
difference is intent, and intent is not in the process table — so the
operator's 2026-08-31 ruling put it there. **Declare or die:**

```sh
POSSE_KEEP=<reason> nohup ./long-thing.sh &     # the documented form
POSSE_KEEP=ranger-base-abcd; cd /repo && ./bench.sh &   # equally fine
```

The reason is conventionally the bead id that authorises it; the guard
prints it and does not judge it. Write it **at the head of the line** — the
guard will honour it anywhere in the argv (see below), but the head is where
the next person reads it. Undeclared is a leak and a leak is killed.

Three properties worth knowing before you rely on any of it:

- **The two anchors point opposite ways, on purpose.** The preamble
  ("is this ours") is matched at the *head*, because a loose match there
  kills a stranger's process. The marker ("was this declared") is matched
  *anywhere*, because it is the spare: a loose match there only means a leak
  survives — which arm 1 still reports and you can still kill by hand —
  while a tight match kills something somebody meant, and that is
  irreversible.
- **The signal ladder is measured, not assumed** (darwin 25.4.0, planted
  control, forked non-exec'd subshell): `TERM` ends `sh`/`zsh`/`bash`
  children in **24–31ms**. The same spinner behind `trap "" TERM` survives
  TERM indefinitely (3.0s and still going) and dies of `KILL` in 24ms. So:
  TERM, one **shared** 500ms grace for the whole batch, then KILL for
  whatever ignored it, then a shared 250ms confirm. Forty leaks cost what
  one costs.
- **It fails open on the pass and closed on the kill.** A pid that will not
  die is a word in a log line, never a pass that is late. But the reaper
  re-reads its targets' rows immediately before signalling, and a target it
  cannot re-verify — gone, recycled onto another process, no longer ppid 1,
  no longer ours, declared in between — is skipped rather than killed, with
  the reason printed. Non-positive pids are refused outright: `kill(2)`
  reads those as process *groups*.

`load_guard_kill:` in `config.yaml` arms it (`true`/`false`; absent is
false, and a typo is named in the report and leaves it off) — and since the
guard clock, that key is the ONLY thing gating the kill: `load_guard: 0`
turns off the launch gate and no longer turns off the census with it. It
ships **off**: the ruling's first bar for the live flip is arm-1 field data
showing real leaks and no deliberate process, and reading that is the
operator's, not the guard's.


### The self-check takes no fork on darwin, and why it had to (ranger-base-yxmwx)

`go run ./cmd/checkorphans` is the check AGENTS.md mandates after
backgrounding anything, and from the day it landed (2026-08-31,
ranger-base-6mhxw) to 2026-09-28 it was the one check a **caged** seat could
not run:

```
checkorphans: fork/exec /bin/ps: operation not permitted — could not read the
process table, leak status unknown
exit status 2
```

Four closes across three personas ran into it in one day — dinesh
(ranger-base-mis0i), gwart (-prjck, -rg19l), holden (-vl8zu) — and each
invented its own substitute on its own bead. The tool was never dishonest
about it: it says "leak status unknown" and exits 2 rather than reading
clean. The gap was that the seats that most need the check are the caged
ones.

**The cause is one bit.** `/bin/ps` on darwin is `-rwsr-xr-x root wheel` —
**setuid root** — and seatbelt refuses to exec a setuid binary from inside a
sandbox. It has nothing to do with this shop's profile, which is `(allow
default)` plus file-write denies and does not mention exec at all.

MEASURED 2026-09-28, darwin 25.4.0, the rendered `gilfoyle` persona profile
unless stated:

| probe | result |
| --- | --- |
| `sandbox-exec -f <persona>.sb /bin/ps -axo pid=` | `execvp() of '/bin/ps' failed: Operation not permitted` |
| `sandbox-exec -p '(version 1)(allow default)' /bin/ps …` | same refusal — nothing about the persona wall is involved |
| profile + `(allow process-exec* (literal "/bin/ps"))` | same refusal — **an explicit allow does not lift it** |
| profile + `(allow process-exec* (with no-sandbox) (literal "/bin/ps"))` | runs — i.e. only by running a setuid-root binary *outside* the wall |
| `sandbox-exec -f <persona>.sb /usr/bin/pgrep …` | runs (pgrep is not setuid — which is why every seat's substitute reached for it) |
| a non-setuid **copy** of `/bin/ps`, sandboxed or not | 0 rows, exit 0 — darwin's `ps` needs its privilege even to list |

So the carve-out was closed on both ends: the only profile line that lifts the
refusal grants a setuid-root exec outside the sandbox, which is the opposite
of what the profile exists for. **The route is the syscall.**
`unix.SysctlKinfoProcSlice("kern.proc.all")` needs no fork and no privilege,
and per-pid `kern.procargs2` gives the untruncated argv the preamble match
reads (proctable_darwin.go). Measured the same day: 452 rows via sysctl
against 451 via `ps` (ps does not list itself), argv readable for every
process of our own uid and refused for 165 of 452 that belong to another —
and a gate-shell child is always ours. Age comes from `p_starttime` instead
of an `etime` rounded to the second, and there is no second read to get past
a column width.

**What did NOT move: the load guard's `ps`.** `kinfo_proc.p_pctcpu` is **0 in
every row** on darwin 25.4.0 (all 451 comparable rows, including one at
`ps_pcpu=58.4`), so a %CPU this route reported would be a fabricated zero.
The self-check has no CPU term by design, so it loses nothing; `SysTopCPU`
keeps its fork, and does not need a cage — the load guard runs where `ps`
runs.

**The two routes agree on a real payload, not only on a row count.** The first
caged run that worked printed, for pid 47360, `source
~/.claude/shell-snapshots/snapshot-zsh-…sh 2>/d…` — the same
preamble-stripped payload the `ps` route printed for the same process from an
uncaged seat minutes earlier. `kern.procargs2` hands over the argv vector and
this code space-joins it exactly as `ps -o args=` renders it, so
`gateShellForkPayload` reads one string either way.

**The pins** are `internal/posse/proctablecage_qa_test.go`: an empty `PATH`
(cheap, hermetic, reds in milliseconds if the exec comes back) and the
reported bug itself — the check run inside `sandbox-exec -p '(version
1)(allow default)'`, with a control that skips rather than passes if a future
macOS stops refusing the setuid exec. MUTATION-CHECKED: pointing
`sysSelfProcs` back at the `ps` route reds both arms.

**If it ever answers exit 2 again**, that is "leak status unknown", never
"clean". The substitute the four seats converged on is `kill -0 <the pid the
launcher printed>`, `pgrep -fl <your worktree path>` and
`scripts/suite-lock.sh --status` — and it is weakest in exactly the case the
tool exists for: a `pgrep` of one path cannot see a fan-out of forty low-CPU
children, and a pid you already know about was never the part you were unsure
of. Say on the bead that you substituted.

One corroboration from the first caged run that worked, worth keeping because
it is the shape every cheaper check misses: pid 47360, a gate-shell child
orphaned to launchd **21 days** earlier, `0.0%` CPU, idle. No %CPU floor would
ever have named it, and no `jobs -l` could have seen it.
