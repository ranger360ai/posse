## The reach probe's third answer (ranger-base-heur)

ADR 0013 §4's reachability row is judged **behaviourally**: create and
remove a probe file inside each target under `sandbox-exec -f <the rendered
profile>`. That makes the row's own instrument a `sandbox_apply`, and a
posse command run inside a **caged persona session** may not perform one:

```
$ sandbox-exec -p '(version 1)(allow default)' /usr/bin/true
sandbox-exec: sandbox_apply: Operation not permitted        # exit 71
```

Before this bead the probe's failure was reported as a wall verdict —
*"`<store>/.beads` is not writable under the profile this launch runs"* —
with the kernel's own words carried in the middle of a sentence that is a
finding **about the grant**. `unrealized()` feeds `Degraded`, so every
posse command that rendered a reach row from inside a cage degraded the
launch on a measurement that never happened. Uncaged the same command
printed nothing at all, which is why it read as a cage-shaped mystery
rather than as a bug.

**The three answers.** `seatbeltReachRow` returns `(why, unmeasured)`:
granted (`"", ""`), refused (`why, ""`), or *not applied* (`"", why`). The
third goes into `Realized` as a `NOT MEASURED` row and never through
`unrealized()`. A check that did not run must not degrade a launch, and it
must not read like a pass either (ranger-base-fm4p).

**The two "Operation not permitted"s.** They are different events and the
whole bug is reading one as the other:

| output | who said it | means |
|---|---|---|
| `sandbox-exec: sandbox_apply: Operation not permitted` | the kernel, to sandbox-exec | the profile was never applied — nothing measured |
| `/bin/sh: <target>/.posse-reach-probe.N: Operation not permitted` | the sandboxed shell | the profile was applied and denies the target — the finding |

`isSandboxApplyRefusal` keys on `sandbox_apply`, not on the three words
they share. Widen it to `Operation not permitted` and the row abstains on
exactly the denial it exists to report.

**"Is the binary there" is the wrong question, and it has two right
answers, not one.**

- `SeatbeltAvailable()` — a PATH lookup — is about the **host**: may a
  session *launched* from here be caged? Yes, even inside a cage, because
  the launch line is typed into a herdr pane (`startPlanned`'s `PaneRun`)
  and herdr is not in this process's sandbox. So `AvailableCages` keeps
  seatbelt in a caged session, deliberately. Dropping it would have fallen
  the reach row through to the `default` arm, which says *"cage <x> has no
  file wall — every path this session can write, it can write"*: a worse
  sentence than the one this bead removed.
- `sandboxApplyRefusal()` — an actual apply — is about **this process**:
  may the checks posse runs *in* it apply a profile? Every check whose
  instrument is a `sandbox-exec` has to ask this one first.

**The probe carries a deny**, and that is load-bearing: measured over all
four corners in ranger-base-xjw9, a deny in *either* profile refuses the
nested apply, so under a lenient allow-default wrapper a lenient probe
answers "sandboxable" while every profile `SeatbeltProfile` renders — it
emits `(deny file-write*)` unconditionally — is refused. A probe has to be
shaped like the thing it predicts.

**Testing this where you cannot nest.** Inside the cage no arm that needs a
real apply can run at all, so the pins drive the **reader**
(`sandboxApplyRefusal`, a package var) and not the kernel's permission to
read. The wrong arm still needs to run somewhere: `sandbox-exec -f <a
profile that is not there>` fails at *open* —
`sandbox-exec: /nope/x.sb: No such file or directory`, exit 65, no
`sandbox_apply` — inside the cage as well as outside, which is what lets
"a probe failure that is not an apply refusal is still a finding" be
measured in both worlds.
