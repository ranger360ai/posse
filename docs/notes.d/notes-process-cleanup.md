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

