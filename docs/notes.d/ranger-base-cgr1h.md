## The pulse waiter: what the ADR settled, and the four things it did not (ranger-base-cgr1h)

`posse pulse wait --after <seq>` (ADR 0093 D1/D3) landed here: it arms
`flock(state/pulse.wait.lock)` with its pid and start time inside, blocks on
`state/pulse.feed`, prints the entries past the cursor, ends with the line to
come back with, and exits 0. The watch's own arm — switching delivery on the
lock, the `pulse-unarmed` condition, D4's composer hold — is
ranger-base-r3qz6 and is not here; the coordinator PID that arms this is
ranger-base-05ooy. Dated 2026-11-14.

The mechanism was not invented. "Is a listener alive?" answered by an flock,
with identity kept as a separate check, is the field's standard answer to the
pidfile's two conflated questions (`liveness-and-identity.md`: the lock is
held by the open file description and the kernel drops it at process death,
so release IS death and there is no staleness class), and the same answer
this shop already reached for `state/dispatch-launch.lock` in ADR 0011 §1 and
`state/dispatch-watch.lock` in rangerhq-gir5. This file is about the four
decisions the ADR left open, each of which is in the code with a comment and
is MEASURED by a mutant below.

### 1. GAP has two shapes, and the ADR's prose names one

D2 says `--after` asking for a seq older than the oldest kept line prints
`gap`. Read literally — `oldest > after+1` — a feed that **restarted** under
the cursor is not a gap: lose `state/pulse.feed` and the next delivery starts
again at 1, so a waiter holding cursor 400 sees entries 1 and 2, nothing
`> 400`, and blocks until its renag on a shop that is in fact delivering.
Silent on both sides, which is exactly the failure the writer's own next-seq
rule (max parseable plus one, never the last line's) exists to avoid at the
other end.

The ADR expects this case to cost "one `gap`" — its exit-hatch paragraph says
so about losing the file — so the reading here is "the feed cannot account
for the cursor", with both arms: `oldest > after+1 || newest < after`. On
either, every kept entry is printed, because on both the question "what did I
miss" has no answer narrower than "what is left".

MEASURED: dropping the second arm reds
`TestPulseWaitTreatsARestartedFeedAsAGap` and the reading's own table. With
it dropped, the waiter self-exits with `still:` over a live feed.

### 2. The self-exit says `still:` and does NOT name keys

D3's consequence asks the waiter to exit after `pulse_renag` even with no
delta, "printing `still:`". The feed's own `still: <keys>` line is the one
body that names keys, and it is the **watch's**, written on the one tick
where the watch is re-saying an unchanged set it computed.

The waiter computes nothing (ADR 0029: the watch is the only component that
computes conditions) and remembers nothing (D1: the cursor lives on the
command line). So it has no keys to name, and in the healthy case it never
needs any: a live watch appends its keys-naming `still:` and the waiter wakes
on that line and prints it verbatim, which satisfies the ADR's sentence
through the ordinary delta path. The self-exit arm is reached only when **no
line arrived at all** — the watch is dead, the pulse is disarmed, or the feed
cannot be written — and there the honest subject for the word is the feed's
silence: `still: nothing in the feed for 30m0s`. A line naming keys there
would be naming them from nowhere.

So the coordinator sees two grammars under one token. That is deliberate and
the token is shared because the coordinator's reading of it is the same —
nothing has changed — and the two are told apart by their prefix anyway: a
feed line opens with `<seq> <RFC3339>` and the waiter's own lines do not.

### 3. Every exit names the cursor, including the interrupt

Not in the ADR, and load-bearing for ranger-base-05ooy: a delta, a gap, a
renag self-exit and a SIGINT/SIGTERM all end by printing
`next: posse pulse wait --after <n>` and nothing after it. The coordinator
re-arms off that line and off nothing else, so a waiter that woke a session
without naming the cursor has woken one that must guess. The two **refusals**
print nothing at all, which is the other half of the same rule: there is no
cursor to hand back when nothing was ever armed, and a `next:` line there
would tell the coordinator it had been listening.

An interrupt is therefore exit 0, not an error. The runtime takes its
background commands down with the session, and that is not a failure of the
waiter's.

MEASURED: replacing the self-exit's cursor line with a bare `return nil` reds
`TestPulseWaitEveryExitNamesTheCursorLast`.

### 4. A waiter that cannot take the lock refuses, where the watch loop runs

`lockWatch`'s third answer — the lock file could not be opened at all — is
**degraded but running**: a watch loop that cannot record itself still
dispatches the queue, which is its whole job. The waiter's third answer is a
refusal, and the asymmetry is the point: a waiter's whole job IS to be
visible to D3's probe. One that blocked without the lock would read as
unarmed, so the watch would type into the pane of a session that is in fact
listening — a listener nobody can see, silent for up to `pulse_renag`.
Refusing costs the coordinator one turn and leaves it on the typed channel
knowingly.

Same reasoning for a **second** waiter: refused, naming the holder from the
lock file's body, because D3 switches on "the lock is held" and two holders
would be two coordinators woken by one delta.

### Two smaller decisions

- **A poll, not inotify.** The bead offered either. inotify is Linux-only
  and this tree builds on darwin (ci.yml's macos-latest leg), so the
  watch-descriptor version needs a second implementation beside it or a new
  dependency — to buy latency against a feed that moves **4.6 times an
  hour** (ADR 0093's measured 70h window). `PulseWaitPoll` is 2s, one
  stat-and-read of a file capped at 500 lines, identical on both platforms.
- **A disarmed pulse gets `DefaultPulseRenag`, not zero.**
  `LoadPulseConfig` answers a config with no `pulse_interval:` with a zero
  `PulseConfig`, and reading that zero as the waiter's life would exit it the
  instant it armed — which, re-invoked by the coordinator's runtime, is a hot
  spin rather than a wait. The waiter's contract does not depend on the pulse
  being armed: it blocks on a feed, and a feed nobody writes is a quiet shop,
  said once per interval. (`pulse_renag` IS the waiter's life, so a renag
  short enough to spin the coordinator is a config that already spins the
  pulse at the same rate. No floor was invented for a number the operator
  already owns.)

### The lock file carries its own identity, so there is one file and not two

`dispatch-watch.lock` plus `dispatch-watch.pid` are a pair because the pid
record predates the lock and kept the identity half when the lock took the
liveness half. The waiter had no such history, so D3's "with its pid and
start time written inside" is taken literally: the body goes in the lock
file, written through the fd that holds the lock, truncate-then-write. It
decides nothing — a stale body survives a SIGKILLed holder and the probe
never reads it — and the only caller is a refusal's "who holds it" clause,
which forms a sentence (`holder unrecorded`) when the body is missing.

### Where the pins are, and why they are in the arms they are in

- `internal/posse/pulsewait_test.go` — **arm 2**, with the rest of the flock
  acquisition family beside `watchlock_test.go`, under the serial discipline
  that family needs (ranger-base-9l77f: two acquisition tests side by side
  read a released lock as still held, 3-6 failures in 60 at `-parallel 8`).
  Eleven entries added to `cmd/testparallel`'s serial map; the two tests here
  that arm nothing carry `t.Parallel` instead. Note for a seat hunting the
  pulse: the feed's own line format is next door in **arm 3**
  (`pulsefeed_test.go`), so `-run Pulse` needs to know which tag it wants.
- `internal/posse/pulsewaitpane_qa_test.go` — **arm 1**, an AST census of
  one file in its own directory (no repo-root climb, so it is not a
  tree-wide pin and needs no Makefile door). ADR 0093 D1's claim is that the
  waiter writes nothing to a pane *by construction*, and a claim of
  construction has to be read off the construction — no behavioural test can
  prove a pane write did not happen on a path it did not take. Two arms: the
  file names no pane-reaching identifier, and `PulseWait`'s signature gives
  it nothing to reach one with.
- `cmd/posse/pulseverb_qa_test.go` — the same claim where a verb actually
  acquires a pane. Every arm of main's switch has `hb` in scope, so "writes
  to no pane" is one careless `hb.` away at that layer whatever
  internal/posse's signature says.

### Every pin measured the other way

Seven mutants, all seven red the pin that claims them (2026-11-14, this
worktree). The AST censuses read their subject from **disk** at test runtime,
so `go test -overlay` cannot reach them — the overlay rewrites what the
compiler sees and nothing else — and those three were measured by planting
the mutation in the real file and restoring from a copy, with a checksum
taken first as the proof the tree came back (`git diff` would have read clean
over any of it: `pulsewait.go` was untracked at the time, and git reports
nothing for an untracked path).

| mutant | pin it reds |
|---|---|
| the waiter names `hb.Mux().Prompt` | `TestQAThePulseWaiterReachesNoPane` |
| `PulseWait` grows a `*HerdrBackend` parameter | `TestQAPulseWaitTakesNoBackend` |
| the cmd arm names `hb` | `TestPulseVerbNeverReachesTheHerdrBackend` |
| the restarted-feed gap arm is dropped | `TestPulseWaitTreatsARestartedFeedAsAGap` |
| `defer lock.Release()` becomes an immediate release | `TestPulseWaitHoldsTheLockWhileItBlocks` |
| the self-exit stops printing the cursor | `TestPulseWaitEveryExitNamesTheCursorLast` |
| `PulseWaiterArmed` answers from the lock file's body | `TestPulseWaitLockDiesWithItsProcess` |

The last is the regression the SIGKILL child test exists for, and the reason
that test is worth its fork: liveness read from the record instead of from
the kernel reads a killed waiter as armed, which silences the typed fallback
for as long as the stale body sits there. It is the pidfile mistake by
another name, and it is the one a code review cannot see — the mutant is four
lines and every other pin in this bead stays green through it.

### What this slice does not do

The watch does not read the lock yet: it still types on every due tick and
appends the feed line after the prompt lands (ranger-base-hkqpq's §1), so
until ranger-base-r3qz6 an armed waiter is a SECOND channel rather than the
primary one — the coordinator gets the feed line and the typed pulse. D4's
composer hold, the `pulse-unarmed` condition and `pulse_feed: off` are all in
that bead too. ADR 0093's done-when rows 1-3 are the verify lane's and need
24h of watch log, which nothing here can produce.
