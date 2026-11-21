# The third delegated `posse upgrade` lands 12 of 33 commits: the D4 trip is the crossing, not the range (ranger-base-78k0w)

The first two delegated installs landed and the third rolled itself back
(ranger-base-i9fq8). The rollback was correct and is deterministic: the target
added a condition row the judging binary's class table cannot name, so ADR 0092
D4's unknown-code rule classed it binary-rendered and undid the swap. The generic
defect was answered in the ADR the same day (ranger-base-v2xj9, D4 amendment 4-6:
the new binary describes its own rows), and that answer cannot be installed through
the verb either — every target holding it is judged by the table that predates it,
which is the bootstrap ADR 0092 leaves open and ranger-base-rfsy4 asks the operator
to close.

What is closed here is the part of that stall that never needed either of them.
**The trip is CROSSING the commit that adds the row, not installing a long range**,
and which commit that is can be read before any box is spent.

## The derivation, in three reads

ADR 0092 D4's partition lives in one map, `upgradeConditionSource`
(internal/posse/upgradeverify.go). MEASURED 2026-11-21, read out of that map at
three shas with `git show <sha>:internal/posse/upgradeverify.go`:

| sha | rows the table names |
|---|---|
| `0e1cb6c0` — the installed binary, the judge | G1..G17 |
| `00601dfe` — ranger-base-7or9p | G1..G17 |
| `02412ade` — ranger-base-rrgnc | G1..G18 |

`02412ade` is where G18 arrives. Every target at or after it is judged by a table
without it. Every target *before* it is judged by a table identical to the judge's
own, so there is no unknown code for D4 to trip on — and `posse upgrade [<sha>]`
has taken an explicit target since the D1-D3 build (ADR 0092 D3.1: an ancestor of
`refs/remotes/origin/main`, a descendant of the installed stamp). So the delegate
can land everything up to the commit before the crossing, unaided, and the
operator's `make install` is needed only for the crossing itself.

One commit in the range touched the condition machinery at all — `dc1d70cc`
(ranger-base-1qi7c), internal/posse/govern.go — and its change there is a
signature only, `LoadPulseConfig(a)` to `LoadPulseConfig(a, errw)`: no new row, no
new code, which is what the identical table above already says from the other side.

`00601dfe` was the last commit before the crossing, so that was the target: 12
commits of 33, carrying five vuln-private fixes (ranger-base-7x3bj,
ranger-base-kfrpv, ranger-base-bncbk, ranger-base-gn050, ranger-base-dw2du). The
two security ids above the line (ranger-base-rrgnc, ranger-base-18brx) are not
installed and wait on ranger-base-rfsy4.

## The reading

MEASURED 2026-11-21, 06:10:00.79 to 06:27:26.15 EST (America/New_York), this box
(16 cores), by gilfoyle at a claude/standard seat realized at shims, from the
worktree `~/.posse/worktrees/posse/gilfoyle-posse-ranger-base-78k0w`. The verb was
backgrounded and waited on by pid, per ADR 0092 D3 and AGENTS.md.

```
posse upgrade 00601dfe
  0e1cb6c -> 00601df (12 commits) · source ~/src/posse · target ~/.local/bin/posse
exit 0 · wall 1045.4s (17m25s) · zero suite-slot queue
loadavg 3.22 1.69 2.12 at pre-flight · 7.52 5.01 3.34 at launch · 13.52 8.82 7.69 at end
```

| phase | this run | 76esp | jnf2z | i9fq8 (rolled back) |
|---|---|---|---|---|
| suite-slot queue, arm 1 | **0s** | 10s | 146s | 0s |
| arm 1 · internal/posse | 223.1s | 244.9s | 214.0s | 232.7s |
| arm 1 · internal/treepins | 168.2s | 156.9s | 154.5s | 172.9s |
| arm 1 · cmd/posse | 136.9s | 122.3s | 119.1s | 139.1s |
| arm 1 · cmd/coldexec | 1.8s | 1.8s | 2.0s | — |
| arm 1 · cmd/testparallel | 0.2s | 0.2s | 0.2s | — |
| arm 2 (`-tags posse_arm2`) | 62.4s | 62.6s | 81.5s | 66.3s |
| arm 3 (`-tags posse_arm3`) | 111.7s | 103.7s | 119.1s | 172.6s |
| silent-revert audit | 2812, 0 | 2800, 0 | 2798, 0 | 2817, 0 |
| wall | 1045.4s | 1017s | 1148s | 1111s |
| wall minus queue | 1045.4s | 1007s | 1002s | 1111s |

Nothing newly red, so no D4 trigger fired:

```
  before: status 17 condition(s), 0 unreadable store(s) · gates 11 persona(s), 36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  after:  status 17 condition(s), 0 unreadable store(s) · gates 11 persona(s), 36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  ✓ nothing the binary renders is newly red — the install stands
```

The D6 line, verbatim:

```
upgraded 0e1cb6c..00601df (12 commits) · gate unchanged · rollback: install -m 0755 ~/.local/bin/posse.prev ~/.local/bin/posse
```

Verified by execution afterwards, independently of the verb's own counts:
`~/.local/bin/posse` is `0.5.3+00601dfe` and `posse.prev` is `0.5.3+0e1cb6c0`, so
one level of rollback is in place (D3.4); `posse.new` is gone, which is D3.5's
`rename(2)` having happened, and the two files hold different inodes;
`posse gates gilfoyle` exits 0; `posse gates render-probe` exits 0 with its mark;
`go run ./cmd/checkorphans` reads clean; the verb's private copy under
`/tmp/posse-upgrade` cleaned itself up.

The gate word was read before the run, as ranger-base-76esp's fragment prescribes:
no commit in the whole 33-commit range touches `scripts/bd-argv-gate.{sh,py}`, and
both installed copies were byte-identical to the target's
(`a47bebed848ec3e1…`, `24ef126210550fa2…`). The verb then reported `unchanged`
with all three behaviour arms green — a third `unchanged`, so D5's `installed`,
`refused` and `unknown` words are still unexercised in anger.

## The install's own signal was the thing it stopped short of

An install that stops before `02412ade` installs a binary with no G18 row, so
`posse status` cannot say the dispatch loops are stale, and the verb printed no
`dispatch:` line either — both are ranger-base-rrgnc's code. MEASURED 06:31 EST,
four minutes after the swap: both `posse dispatch --watch` loops, started 00:07:31
and 00:08:52, read `/proc/<pid>/exe -> ~/.local/bin/posse (deleted)` against a
live binary written at 06:26. Every one of the five fixes in this range is a wall
fix, so until the loops restart a seat they launch renders the pre-install walls
and a live check of any of the five reads red over a correct install.

That is the price of the partial install, and it is worth naming because it
inverts the usual reading: here the absence of a G18 row is not an all-clear, it
is the row not being installed yet. The remedy is unchanged and is the
coordinator's — restart the loops, then relaunch seats (ADR 0092 D6 as amended by
ranger-base-rrgnc).

## What this does not measure

- **A quiet-box wall, still.** Four launches now. Both suite slots were free at
  launch for the first time, and all three arms took slot 2 with no wait — but
  `scripts/suite-lock.sh --status` right after the run read slot 1 held by a
  sibling worktree's suite, which is also what the loadavg going 3.22 to 13.52
  says. So this is the second zero-queue reading and not a quiet-box one.
  What three installs that swapped and verified do now say: wall minus queue is
  1002s, 1007s and 1045.4s, a 4.3% spread across launches at a loadavg of ~11,
  ~11 and 3-to-13. The queue is what moves the reading; sharing the box *within*
  the slot discipline moves it by about 4%.
- **The crossing.** Nothing here installs `02412ade` or anything above it, and
  after this install the judge's table is still G1..G17, so the next delegated
  attempt at the tip rolls back exactly as ranger-base-i9fq8's did.
- **Whether a partial install is ever the wrong call.** It was right here because
  the range held five vuln-private fixes and the alternative was waiting on an
  open operator ask. A range whose commits are one bead's series would be a
  different question, and this reading says nothing about it.

## Not closed here

- ranger-base-rfsy4
- ranger-base-i9fq8
