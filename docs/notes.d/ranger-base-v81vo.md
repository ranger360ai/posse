## A commit-time gate is install-gated exactly like a wall, and the lag reading has no category for it (ranger-base-v81vo)

2026-11-14

Findings and measurements behind ranger-base-v81vo. The bead's subject is an
installed binary that predates `ranger-base-ed07e`, so check 4's own-bead
exemption is not in force on this box. The install is not this seat's to do
(see §4). What IS here is the reason nothing on the box said so, which is a
category gap one file over.

Read this with `internal/posse/wallrenderer.go` (where the two pathspec lists
live), `internal/treepins/launchchecksources_qa_test.go` (where the second
list's membership is derived) and `docs/notes.d/ranger-base-atvd4.md` (the
same shape, one category earlier).

### 1. The box state, MEASURED

MEASURED 2026-11-14, this seat, in the worktree
`posse/gilfoyle-posse-ranger-base-v81vo` at `1d03f9d1`:

| reading | value |
|---|---|
| `/home/posse/.local/bin/posse` mtime | 2026-11-13 20:58 |
| `posse version` | `0.5.3+7d0f8c31 (herdr-native)` |
| `7d0f8c31..origin/main` | 59 commits |
| `origin/main` tip | `1d03f9d1` |
| `7d0f8c31` is an ancestor of `e3b41b30` (the `ranger-base-ed07e` landing) | yes |

The exemption's code is absent from the installed artifact, not merely
untriggered. Two string literals that `e3b41b30` added to
`internal/posse/vulnprivate.go` — `this tree answers for no bead` and
`One id is never flagged here` — are both ABSENT from
`/home/posse/.local/bin/posse` (`grep -a`). The source is correct in the
tree; the artifact is stale.

The branch record the exemption reads is present and correct:
`git config --get branch.posse/gilfoyle-posse-ranger-base-v81vo.posseBead`
answers `ranger-base-v81vo`. Nothing in the installed binary reads it.

### 2. What that costs at commit time, reproduced

Both halves reproduce against the installed binary. The scanned text is a
line naming one open `vuln-private` bead with more than its bare id; the id
is withheld here and the shapes are given as written:

- a house commit subject — `<id>: <summary>` — exits 1 with
  `a line here, or a line continuing it, carries more than the bare id — the
  text is withheld (AGENTS.md "Landing the plane")`. A bare `<id>` alone
  exits 0. That is the hole `ranger-base-ed07e` was filed over and measured,
  still open here.
- a notes fragment cannot land AT ALL, and the gate's remedy sentence has
  nothing to offer. A fragment's `## … (<id>)` heading is flagged, and so is
  `scripts/notes-index.py`'s generated index line, whose link TEXT is the
  bead id followed by the title and date. Both shapes exit 1, so "rewrite it
  so the id stands alone" is unachievable for a fragment that must name its
  own bead twice.

A second drift, same cause: the installed `prepare-commit-msg` in
`~/src/posse/.git/hooks/` calls `posse gates vuln-private-scan` with no
`--message` at both of its call sites (lines 1470 and 1512). No flag is
`VulnPrivateContentArm`, the judge's own zero, so a commit MESSAGE is
currently scanned through the content arm. The tree renders two arms and
spells `--message` on the message one (`gates.go`'s `vulnPrivateChecks`).
Hook bodies are compiled into the binary, so the hook is a copy of the old
render however recently it was written — this one is dated 2026-11-14 16:09
and is still the `7d0f8c31` render. This is exactly the window
`VulnPrivateArm`'s zero was chosen for, and it behaves as documented; it is
recorded here because it is a second thing the install moves, not a second
defect.

### 3. The category gap, which is the finding

`posse gates <persona>` already takes this reading and prints it first. On
this box it reads:

```
wall renderer (gates) · 0.5.3+7d0f8c31 is 59 commit(s) behind main in ~/src/posse:
  8 of them rendering a wall, 7 of them deciding a launch-time check
```

Two categories, `WallRenderSources` and `LaunchCheckSources`. Neither covers
a decision the installed binary takes at COMMIT time, and check 4 is one:
the hook body is a render, but the JUDGE is a subprocess of the installed
binary, resolved fresh at commit time on purpose — `vulnPrivateChecks`'
comment says why ("an id's open/closed status is exactly the live fact a
render-time stamp cannot hold").

MEASURED 2026-11-14 over this box's own exposure window
(`7d0f8c31..origin/main`, 59 commits):

| category | count |
|---|---|
| rendering a wall | 8 |
| deciding a launch-time check | 7 |
| changing check 4's judge (`internal/posse/vulnprivate.go`) | 4 |

The four are `e3b41b30` (ranger-base-ed07e), `6067b1d7` (ranger-base-gi6ge),
`e208dba9` (ranger-base-crwxs) and `72fe7738` (ranger-base-n2veh). Of those
four, exactly ONE appears in the printed reading at all — `6067b1d7`, which
also touched `gates.go` and is therefore counted under "rendering a wall",
beside a cost sentence about seatbelts, gate shims, cage mounts and hooks.
That sentence is true of `gates.go` and says nothing about the commit a seat
is being refused. The other three are inside the headline 59 and named by no
category.

So the gap is `ranger-base-atvd4`'s gap exactly, one category further on. A
wall is install-gated because the binary RENDERS it; a launch-time check
because the binary DECIDES it at launch; check 4 because the binary JUDGES
it at commit. Same consequence shape — a landed fix that reaches nobody —
and the same reason neither existing derivation can see it: `vulnprivate.go`
writes no `do not edit` header (the wall class's key) and declares no
`*Parity` signature and constructs no `Runtime` (the launch-check class's two
keys). It is CORRECTLY outside both lists, and hand-adding it to either would
red that list's pin and attach the wrong cost sentence.

`wallrenderer.go`'s `lagCategory` already anticipates this: "The two
categories differ in prose and in pathspecs and in nothing else, so they are
one type: a third category added later gets a row, not a branch." Handed off
as such.

### 4. Why the install is not here, and what a freshness door would be

Installing a posse binary on this box changes what every seat's gates do. It
is an operator key (ADR 0015 §1), and ADR 0092 delegates it to one persona
through `posse upgrade` under five guardrails — but D7 holds the
`install_delegate:` config key and the PID for the operator's promote, after
D1–D5 land and after the security persona's whole read of the verb.

MEASURED 2026-11-14: the verb IS in the installed binary (`posse upgrade
--help` answers). `install_delegate:` is absent from this instance's
`config.yaml`, and `upgradeFence` refuses EVERY persona on an absent key, by
design and before any other step reads anything ("a verb that installed the
fleet's binary for anybody the day somebody deleted a config line is a fence
whose failure mode is wide open"). So the fence is working, and this seat is
on the refusing side of it.

The install itself is already filed and already the operator's:
`ranger-base-yj80h` (P1, open) asks for an install at `2839ad8e` or later.
Note the floor: `2839ad8e` PREDATES `e3b41b30`, so that bead's literal
minimum would not carry the exemption. `make install` builds the HEAD of
`~/src/posse`, which today is `1d03f9d1` and does carry it, so in practice
one install cures both — but the floor for THIS bead is `e3b41b30`.

**On the bead's own open question — whether a seat should have a freshness
door before it commits — the answer is no, and the reading is why.** Neither
`make verify-gate-freshness` nor `make verify-hook-freshness` is a
prerequisite of `make test` or of `make tree-check` (MEASURED: the `test:`
and `tree-check:` lines name neither), and neither is named by AGENTS.md at
all (MEASURED: `grep -n freshness AGENTS.md` is empty — the bead believed
otherwise). That division is intended. A door that compared the installed
binary to the tree would be red for every seat on the box for the whole gap
between a landing and an install — 59 commits wide right now — over a
condition the seat typing it cannot fix, because installing is the operator's
or the delegate's. `wallrenderer.go` states that stance for itself in the
same words: "A READING, NEVER A CONTROL … it prints, it warns and it decides
nothing."

The freshness surfaces are therefore already the right shape; what is wrong
is their coverage. The fix is §3's third category, not a door.

### Not closed here

- ranger-base-a9gbq
