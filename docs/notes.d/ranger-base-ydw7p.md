## The wall pins' fixture root is declarable, and an undeclared abstention is not `ok` (ranger-base-ydw7p)

`wallUngrantedDir` (internal/posse/wall_helpers_test.go) is the precondition of
every test whose subject is an L2 wall REFUSING a write: a directory outside
the three write grants a rendered profile carries whatever its PID says
(`/tmp`, `/private/tmp`, `$TMPDIR` — `seatbeltTempGrants`). It cut that
directory under `/var/tmp`, which is the one path on both hosts that none of
the three covers — and which a seat's own bwrap wall makes read-only, because
`--ro-bind / /` is read-only over everything it does not bind back.

So on the box where these pins matter most, the seat that runs them cannot cut
one. 77 call sites take their root from that helper, and they are the ADR 0100
wall pins the security lane is asked to re-run when it verifies a fix.

**MEASURED 2026-11-21** (this box, linux 7.2.15-1-omarchy, go1.27.3), the
reading the bead was filed on, reproduced here in a venue shaped like a seat's
wall — `bwrap --ro-bind / / --unshare-user`, with the worktree, the build
caches and a private `/tmp` bound read-write — over ranger-base-xab99's five
pins:

| declaration | pins measured | `go test` says |
|---|---|---|
| none, before this bead | 1 of 5 | `ok`, 0.012s |
| none, after | 1 of 5 | **`FAIL`**, with the reason and the remedy |
| `POSSE_TEST_UNGRANTED_ROOT=none` | 1 of 5 | `ok` — the pre-fix reading, now something a seat has to ASK for |
| `POSSE_TEST_UNGRANTED_ROOT=<a dir the wall grants>` | **5 of 5**, all PASS | `ok`, 4.08s |

The four that abstain answer `cannot make a fixture root under /var/tmp:
mkdir …: read-only file system`. That is why ranger-base-oejcn closed on a code
read plus one executed pin.

### Two halves

**A declared root.** `envUngrantedFixtureRoot` =
`POSSE_TEST_UNGRANTED_ROOT` (internal/posse/prephostfence.go) names a
directory the seat can write; `ungrantedFixtureRoot` holds it to the same rule
the `/var/tmp` arm applies to itself, through one function both arms call
(`ungrantedGrantRefusal`) so the two cannot drift. Absolute, present, a
directory, and outside all three blanket grants — checked again on the
*resolved* path the `MkdirTemp` answers with, because a box that symlinked the
declared root into `/tmp` would hand every caller a granted root and every
refusal would be measuring the other fixture. A declaration that fails any arm
is a **failure naming the variable**, never one more silent skip: a seat that
declared a root asked to measure these pins.

It is also a prep-host fence root (`prepHostFenceRootList`), because these
fixtures' prep steps write on the host under the fixture `$HOME`, which is
*inside* this root — a seat that moved the root and not the fence would watch
every wall pin red at a refusal naming the fence instead of the wall it came to
measure. Only a valid declaration widens the fence. The list is a pure
function now so that a pin can read the rule without writing the package
globals every parallel wall pin is reading.

**An abstention is never a pass** (ADR 0097 D5), kept for the run a person
types. `scripts/posse-wall-check` keeps that rule for the venue, by reading
`go test -json` and answering exit 3 over a member that went dark — it is that
record's own `/var/tmp` row, where this very helper took 18 pins dark over a
green exit code. Nothing kept it for `go test -run <the pins> ./internal/posse`
from a seat. Every `NOTHING MEASURED` the helper can answer with now goes
through `wallUngrantedAbstain`, and `TestMain` answers the count after `m.Run`.

**It has to be the exit status, and that is measured rather than preferred.**
MEASURED 2026-11-21 (go1.27.3, linux): `go test` without `-v` DISCARDS
everything a PASSING package's binary writes to stdout and stderr, `TestMain`'s
own writes included — so a summary line printed after `m.Run` is read by nobody
in the one run shape this defect appears in. The same line is relayed in full
the moment the binary exits nonzero, in a filtered run and an unfiltered one
alike. A dedicated pin would have been worse than nothing: `-run
'TestQAKeepLeaf…'` does not select it, and that filtered verify is the case
this bead exists for.

The two counts are separate — declared and silent — and that is load-bearing
rather than bookkeeping. One count plus a "was it declared" flag reads whatever
the LAST abstention of the run happened to be, and this bead's own pin declares
a dark run for one subtest: one probe would have set that flag for every real
abstention after it, and the mechanism would have reported "declared" forever
while failing nothing.

### What the fix is not

The other shape the bead offered — grant every verifying seat such a directory
on its **launch line** and have `posse gates` print it — is still open and is
the operator's (crew guardrail 5). This variable needs no launch and no
install, which is what lets a seat behind today's wall measure these pins
today. A seat picks a root out of its own wall's write grants (`posse gates
<persona>` prints them); residue there is reachable only by
`scripts/reap-test-tmp.sh --root <that dir>`, which is why the declared arm
keeps the `posse-ungranted-` prefix and a pin holds it to it.

**The residual, stated because the rule looks stronger than it is:** the three
grants it checks are the BLANKET ones, the grants that are the same for every
render. A grant some render derives from a host input that does not move with
the fixture's `$HOME` could still cover a declared root, and no rule in the
helper can see that. The `/var/tmp` arm has the same residual and always has;
prephostfence.go's head comment names the two inputs of that kind this package
knows about.

### The temp-root census

`trGoRoots` (internal/treepins/temproot_qa_test.go) moved 17 → 19. MEASURED
2026-11-21: 19 Go roots and 8 Python over 1164 `.go` files here, against **18
Go over 1163** at the commit before this bead — read by unpacking `git archive
HEAD` into a scratch tree and running the logging pin there. So one of the two
is this bead's (the declared arm's `MkdirTemp`, whose parent name is bound to
`""` elsewhere in the package and is therefore ambiguous, which ranger-base-sxez7's
rule scores IN rather than dropping) and one was already live above a floor of
17 — exactly the one-directional silence that constant's own comment predicts.
The Python line stays at 7 against a live 8: that root is nobody's in this
bead, the floor reds nothing in that direction, and moving a number whose
before-reading was never taken here is the same drift from the other side.
