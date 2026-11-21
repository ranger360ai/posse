# A write over a file you just exec'd is ETXTBSY under load, and no fd of yours is why (ranger-base-nywsk)

ranger-base-nywsk, 2026-11-20, from a red seen once on 2026-11-15. `make
tree-check` reddened in a session worktree at a one-minute load average of 14:

```
--- FAIL: TestQATheWrapperStillProvesItReusesTheBinary (0.43s)
    gotestreuse_qa_test.go:109: scripts/gotest.sh --self-test failed: exit status 1
        ok    reuse: unchanged package keeps one inode
        ok    reuse: no second binary was written
        ok    control: the plant runs when exec'd
        scripts/gotest.sh: line 490: /tmp/posse-gotest-selftest.FgX9qq/cache/gotestselftest-cb3b3b155d561616.test: Text file busy
FAIL  github.com/ranger360ai/posse/internal/treepins  92.304s
```

Line 490 was `cat "$plant" >"$cached"` — the self-test's poison arm, writing a
planted file IN PLACE over the cached test binary arm 1 had just run twice. A
re-run of that one test in isolation, same tree, same commit, same load, was
`ok` in 1.266s.

Environment for every reading here: Linux 7.2.15-1-omarchy, 16 cores, bash 5,
go test's own temp dirs under `/tmp`, one-minute load average given per table.

## The mechanism is the kernel's, not the script's

The bead's first reading was the obvious one: a writer's fd the script left
open, which is ADR 0085's shape from the other side. It is the wrong one, and
the difference decides the fix.

ETXTBSY from an **open for write** means some process still holds that inode as
its executable text; ETXTBSY from an **exec** means someone holds it open for
write. The failure above is the first kind — it is a redirection, reported by
bash with its own line number. The arm's child was exec'd, waited for and
reaped before that write, and the script holds no descriptor on the cached file
at any point. There is nothing to close.

What is left is the kernel: the final `fput` of an exec'd file — the thing that
releases the write deny — is not synchronous with the exit of the process that
held it. A write issued immediately after a reaped exec can therefore lose the
race, and does, at a rate that rises with load. That is why one sighting at a
load average of 14 and a green re-run on a quiet box is exactly the signature
to expect, and why no amount of descriptor hygiene would have helped.

## MEASURED: the race is reproducible, and the window is short

A tiny Go ELF, exec'd through bash, waited for and reaped, then written over
immediately (`scripts/gotest.sh`-shaped: `cat pristine > bin`). Two runs:

| iterations | loadavg (1m) | ETXTBSY | other errors |
|---|---|---|---|
| 300 | 5.24 | 7 (2.3%) | 0 |
| 400 | 3.74 | 6 (1.5%) | 0 |

Control, same binary, same write, while a process really is executing it:
refused every time. So the 1.5-2.3% above is the deferred release and not a
filesystem quirk, and ETXTBSY is enforced on this box — which the fix's new arm
then relies on.

Retried flat out with no delay at all, every hit cleared:

| hit | attempts | elapsed |
|---|---|---|
| 1 | 22 | 38.4ms |
| 2 | 11 | 18.9ms |
| 3 | 16 | 34.6ms |
| 4 | 18 | 31.5ms |
| 5 | 8 | 13.9ms |
| 6 | 19 | 32.3ms |

6/6 cleared, worst 22 attempts and 38.4ms. Each attempt is a fork, ~1.7ms on
this box, which is why a flat-out retry paces itself.

Probes: `etxtbsy-probe.sh` and `etxtbsy-drain.sh`, run from a scratchpad and
not kept — both are twenty lines of the table above, and the numbers are the
part worth keeping.

## What landed

`scripts/gotest.sh`, self-test only. `plant_over <src> <dst>` writes bytes over
a destination in place, waiting out a destination the kernel still holds:

- Eight immediate attempts, then a nap between attempts. The budget is two
  orders of magnitude over the worst drain measured.
- The nap is **probed**, like `stat` and the sha256 spelling in the same
  script: BSD `sleep` has rejected a fractional argument, so a box that will
  only sleep in whole seconds gets fewer, longer attempts rather than a
  tighter spin.
- It retries on **any** failure and reads no `strerror` text. The message is
  localized, and the right answer to a genuinely unwritable destination is the
  same bounded wait followed by the same named failure.
- It prints the reason once, with the bead id, the first time it has to wait,
  so a loaded box leaves a trace instead of a silent pause.
- The write is not believed on its exit status: the bytes are read back with
  `cmp`. That is what catches a plant that reports success and lands nothing.
- A plant that cannot land **reds one arm** with the reason named, instead of
  killing the script under `set -e` with the arms after it never run. That was
  the whole cost of the original: three oks and a dead script.

The production path needs none of this and did not get it — `reuse()` renames a
freshly built file into place, and a rename over a busy inode is not refused.

## The fix carries an arm that can fail

The flake showed once and would not come back on demand, so "it did not happen
again" is not evidence. Arm 6 is the deterministic half of the same refusal: a
real ELF (a `#!` script's own file is never held as text, so the self-test's
existing plant cannot stand in for it) is exec'd in the background and holds
its own inode until the arm removes the marker it wrote — not for a fixed
interval, because a sleep would make the arm a race about timing, which is the
defect and not a test for it. The arm then asserts both halves:

- `control: a write over a live executable is refused` — the naive write, what
  the script used to do. If this passes, ETXTBSY is not enforced and the other
  half proves nothing.
- `plant: the in-place plant waits out a live executable` — the retrying plant
  gets through.

Both labels are required by `internal/treepins/gotestreuse_qa_test.go`'s arm 2,
so deleting either is a red.

Mutation battery over a COPY of the script, 2026-11-20:

| mutant | result |
|---|---|
| plant_over gives up after one attempt (the pre-fix behaviour) | **caught** — arm 6 reds, naming ETXTBSY after 1 try |
| plant_over reports success and writes nothing | **caught** — arm 1b and arm 6 both red on the read-back |
| the `cmp` read-back deleted | survives |

The third survives by construction: deleting an assertion is observable only
through a behaviour that assertion was the only thing watching, and with the
write intact there is none. It is the reason the read-back and the
different-bytes source in arm 6 both exist — together they are what caught the
second mutant.

Cost: `scripts/gotest.sh --self-test` goes from 1.266s to 1.52s, one extra
`go test -c` of a four-line test file. `make verify-gotest` and
`go test -run TestQATheWrapper ./internal/treepins` (10.2s) are green.

## Not closed here

The same class has a second site in this tree, measured once, unaudited and
nothing observed. Named by bead id and nothing else, with the detail on the
bead where the audience is already the right one (AGENTS.md, "Landing the
plane"):

- ranger-base-7mfnf
