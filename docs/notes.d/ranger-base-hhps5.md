# Two arms nothing witnessed, closed: a launch key that read one spelling, and a drop key with no fixture that could grade it (ranger-base-hhps5)

2026-11-22, this box (linux 7.2.15-1-omarchy x86_64, go1.27.3, 16 cores). The
two findings ranger-base-t0ha1's verify left live, both of the same shape — an
arm nothing witnesses, inside a probe whose other arms are well held. Neither
was a wrong answer: each was a question the tree could not ask.

## 1. The census key read one spelling of a launch

`internal/treepins/wallmemberaccount_qa_test.go`'s arm 1 claims to read "every
function in a member file that launches a bwrap seat". It read the functions
that hand `exec.Command`/`CommandContext` a LITERAL `"bwrap"` ARGUMENT, and
the file's prose stated no such limit — so a seat launched through a shell or
a pty wrapper, with the word inside the command STRING, was invisible. The
`launchers >= 25` floor could not say so, which is the whole reason this
escaped: an invisible launcher does not lower the count of the visible ones.

The planted member file and its control — the same probe twice, the only
difference the spelling of the argv, RED for `exec.Command("bwrap", …)` and
GREEN for `exec.Command("sh", "-c", "bwrap …")` — are ranger-base-t0ha1's
reading at 2866fdf4 and not re-taken here. They are arm 3 fixtures now, which
is where a control of a rule belongs: hermetic, named, and re-run every time
the package does.

The key is now the bwrap command WORD wherever it reaches the call's
arguments, one hop included (`wmaSeatLaunches`, with `wmaSeatWordNames` for
the hop). A command word and not a substring: `bwrap` and `/usr/bin/bwrap`
match, `bwraptest`, `bwrap-wrapper`, `bwrap.sh` and `wrcNeedBwrap` do not.
The hop is the depth arm 2's account read already uses, and it is what reads
an argv joined into a line before the call.

### What it found, MEASURED 2026-11-22 (24 member files, this tree)

| key | launching functions |
|---|---|
| a literal `bwrap` argv entry (the old one) | 31 |
| the command word anywhere in the arguments, one hop | **32** |

Two functions take the wrapper route, and both were already in the member set
before this landed:

| member | route |
|---|---|
| `grokleader_qa_test.go:198` `TestLiveWallGrokLeaderUseLeaderTUIStartsNoLeader` | builds `q = ["bwrap", …render argv…]`, joins it, runs it under `script -qfec` |
| `wallunixlive_test.go:374` `TestLiveWallUnixScopesSignalsToTheSeat` | `sh -c 'bwrap "$@"; :'`, the trailing `:` keeping the shell from exec'ing bwrap in place |

The second was already visible by a DIFFERENT launch in the same function (a
direct `exec.Command("bwrap", argv...)` above it), so the widening adds one
function the census could not see at all. Both render their seat's error, so
the widening reds nothing: 33 child errors graded, all four census arms green.
What it closes is the route the next member would have escaped by.

### The mutants, one at a time

| mutant | killed by |
|---|---|
| `wmaSeatLaunches` returns `direct, false` — the old narrow key | arm 3's three planted wrapper cases AND arm 1's wrapper floor (`found 0 … want at least 1`) |
| `wmaSeatWord` is `strings.Contains(s, "bwrap")` | arm 3's `bwraptest && bwrap-wrapper && echo bwrap.sh` case. The tree arms stay GREEN — no member hands an exec call a command string naming a neighbour — so the hermetic negative case is the only witness, which is why it is there |
| the one-hop name read dropped (`wmaExprMentions` gone) | arm 3's pty case only. The wrapper FLOOR stays green, because wallunixlive's literal is still a wrapper launch — so the hop is held hermetically and not by the tree count, and this row is the reading that says so |

Every restore sha256-identical to the pre-mutant file, re-read green after.

### Why a floor AND a ceiling

The floor asks the one question the `launchers` floor cannot — does the tree
still carry a launch the old argv-entry key could not see — so a later
narrowing of the key reds on the tree and not only on the plants. The ceiling
(`wrapped > 8`) is the other direction, because widening a key is how a census
starts grading functions that launch nothing: 2 of 32 take this route, and a
jump is the one-hop read matching prose or a neighbour's name. Arm 3's
negative cases are the precise half of the same claim.

## 2. The drop set's second key had no fixture that could grade it

`bwrapDropTmpGrantsOver` (internal/posse/wallseatcache.go) builds its drop set
from two keys and both must be resolved:

```go
drop := map[string]bool{absResolve(mount): true}
if tmpdir != "" {
        drop[absResolve(tmpdir)] = true
}
```

Only the mount's was witnessed. MEASURED 2026-11-22, `drop[tmpdir]` raw:
`TestQABwrapDropTmpGrantsReadsTheMountsResolvedSpelling`,
`TestBwrapDropTmpGrantsTakesTmpAndATmpdirUnderIt`,
`TestBwrapDropTmpGrantsTakesATmpdirOutsideTmp` and
`TestBwrapArgsBindsTheSeatTmpOverTheHosts` — ALL FOUR GREEN. Nothing in the
tree noticed the loss.

It is not that the mount pin was careless: its fixture plants a symlinked
MOUNT and hands it a tmpdir that needs no resolution, so it physically cannot
produce the shape that grades the second key. ranger-base-aurb8 wrote that
pin, and the `absResolve(tmpdir)` line predates it — the landing changed the
mount key and nothing else.

`TestQABwrapDropTmpGrantsReadsTheTmpdirsResolvedSpelling` is that fixture one
key over: the TMPDIR is the planted link (`var/folders/zz/T` →
`private/var/folders/zz/T`, darwin's own shape) and the mount is a plain
directory, so the only resolution under grading is the one the arm is for.
Three pre-drop controls, because a fixture that cannot produce the shape is
how this key went unwitnessed in the first place: the grant is in the set, it
is in the set under the RESOLVED spelling only, and the undropped set reaches
it under both spellings. Then the drop, asserted as a path set and under both
spellings, with the name-prefix sibling kept.

| mutant | result |
|---|---|
| `drop[tmpdir] = true` (the raw TMPDIR) | the new arm RED, naming both spellings; the other four GREEN |
| `drop := map[string]bool{mount: true}` (the raw mount) | `…ReadsTheMountsResolvedSpelling` RED; the new arm GREEN |

So the two arms grade distinct keys, which is the point of adding one rather
than widening the other. Restore sha256-identical both times.

What the missing witness was worth, and it is ranger-base-aurb8's own sentence
one key over: on darwin a launcher TMPDIR of `/var/folders/…/T` resolves to
`/private/var/folders/…/T`, and `SeatbeltWritable`'s `add` puts the resolved
spelling in the set. With the key raw the drop misses, the box's shared temp
grant survives a render that has just bound a private `/tmp` over it, and the
seat keeps a write grant on a directory every seat on the box shares — the
condition ranger-base-1c91x created this drop to remove. The line was correct;
one `absResolve` held it and no run would have noticed its loss.

## What this did not do

Neither finding is a live defect, so there is no darwin reading to take: both
fixes are instruments, and both plant the darwin shape on a box that is not
darwin, which is the route ranger-base-al4n9 and ranger-base-aurb8 already
established for this class. The first macos-latest arm 1 leg after this lands
reads them on the platform they are about.

No new door: `wallmemberaccount_qa_test.go` still enumerates its file set with
a Glob, so it stays in internal/treepins' one-package-census class and is
doored by `make treepins-check`'s complement (ADR 0098 D1/D4), and the new
internal/posse arm is an ordinary arm-1 test.
