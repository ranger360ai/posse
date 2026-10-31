## A launch-time CHECK is install-gated exactly like a wall, and nothing counted that category (ranger-base-atvd4)

2026-10-31

Findings and measurements behind ranger-base-atvd4, filed off
ranger-base-qr7me ("installed posse predates both grok project-config
fixes"). Read this with `internal/posse/wallrenderer.go` (where both pathspec
lists live) and `internal/treepins/launchchecksources_qa_test.go` (where the
second list's membership is derived).

### 1. The shape of the gap

ranger-base-vso72 closed one silence: the binary that **renders** this box's
walls can be behind the walls that landed, and `posse promote` recorded the
renderer without ever reading it. Its fix narrows the launcher lag through
`WallRenderSources` and says so at `posse gates` and the promote epilogue.

A launch-time **check** is install-gated for exactly the same reason — the
installed binary decides it — and has exactly the same consequence: a landed
fix that does not guard live seats. But it renders no file. `WallRenderSources`
derives its membership from a `do not edit` header a render writes **about
itself** (`internal/treepins/wallrendersources_qa_test.go`, arm 1), and a
check has no file to put a header on. So `internal/posse/runtime.go` is
**correctly** outside `WallRenderSources`, hand-adding it would red that pin,
and the category went uncounted.

That is the whole finding: not a stale list, a missing category.

### 2. MEASURED, and a correction to the bead's own numbers

Over ranger-base-qr7me's exposure window — installed `fe3b8036` (00:24)
against `main` at `d4d105d2` (08:47), 29 commits — measured 2026-10-31 in
this worktree, `git log fe3b8036..d4d105d2 -- <list>` with the pathspecs
passed as separate argv words:

| category | count | commits |
|---|---|---|
| total lag | 29 | — |
| `WallRenderSources` | **2** | `700cc4c1` (personamode.go), `2dce8da3` (skills.go) |
| `LaunchCheckSources` | **8** | incl. `f083e140` (ranger-base-l12j9) and `d4d105d2` (ranger-base-2zkcy) — the grok project-config detector itself |
| union | 9 | — |

**ranger-base-atvd4's own title and description say the wall count read 0 over
that window. It read 2.** The reproduction is in the shell, not in the code:
in `zsh`, an unquoted `$LIST` holding space-separated paths expands to **one**
argv word, so `git log <range> -- $LIST` hands git a single pathspec
containing spaces, which matches nothing and exits 0. The same footgun was hit
and caught once more while writing this fragment. Measure a pathspec list with
a bash array and `"${ARR[@]}"`, or with the paths typed out.

The correction does not weaken the finding, it sharpens it. What the operator
actually read over those eight hours was *"29 behind, 2 of them rendering a
wall"* — a true sentence about a category the grok detector was **not** in,
printed while the detector guarded no seat. Two commits were named and the
eight that mattered were not; a zero would at least have been a silence.

### 3. Steady-state rate — is the second count specific enough?

The wall count's whole value is that it is specific. Measured over this
repo's history on 2026-10-31, commits touching each list:

| window | total | wall | launch-check |
|---|---|---|---|
| `HEAD~100..HEAD` | 100 | 11 | 17 |
| `HEAD~300..HEAD` | 304 | 29 | 26 |

So ~9-17% per window for the new count against ~10-11% for the wall count:
the same order, and far from "every commit". Per file over `HEAD~300..HEAD`:
`runtime.go` 20, `parity.go` 7, `agents.go` 5, `skillsdirtrust.go` 3,
`reachability.go` 0, `suitelock.go` 0.

### 4. The derived class, in two clauses

`LaunchCheckSources` is derived, for the reason the wall list derives its own
half: a hand list goes stale in the direction that prints reassurance. A
launch-time check has two halves that share no spelling, so there are two
clauses, both purely syntactic (no `go/types` load, no call-graph walk):

1. **The deciders** — a non-test `internal/posse` source declaring a func or
   method whose signature names `Parity` or `*Parity`. `Parity` **is** the
   registry every launch-time verdict is reported from, so a signature naming
   it is the file's own claim to write one. Finds `parity.go`,
   `reachability.go`, `skillsdirtrust.go`, `suitelock.go`.
2. **The declarations those verdicts read** — a non-test `internal/posse`
   source constructing a `Runtime` composite literal. The verdicts read a
   `Runtime` (`ProjectConfig`, `ProjectConfigKeys`, `SkillsRoot`,
   `SkillsVerified`), so a commit changing what a `Runtime` **says** changes
   what a launch checks as surely as one changing the checking code. Finds
   `runtime.go` and `agents.go`.

Clause 2 is why `f083e140` counts at all: it touched `runtime.go` and nothing
else outside tests.

**`hookfresh.go` is deliberately out**, the same judgement
`WallRenderSources` made about it: the hook-freshness sweep *reads* walls and
decides nothing at launch. Counting a reader inflates the one number whose
value is that it is specific. Its `r.Degraded` is a `HookWallRepo` field, not
a `Parity`, so neither clause reaches it — and arm 3 below reds if anyone
adds it by hand.

**The list is a FLOOR and nothing keys off it.** The line always reports the
WHOLE lag and always names `make install`; the per-category counts only say
how loud to read it.

**The two lists are NOT pinned disjoint.** A file can render a wall and decide
a check, and a commit to it is honestly counted in both categories. The two
counts are per-category and do not sum — the headline is the whole lag either
way. They happen not to overlap today; that is a fact about the tree, not an
invariant worth a pin.

### 5. Three arms, and what each catches

`internal/treepins/launchchecksources_qa_test.go`:

- **arm 1** — every derived source is in `LaunchCheckSources`. Reds (verified
  by deletion) with: *"internal/posse/runtime.go constructs a Runtime … and
  posse.LaunchCheckSources does not name it"*.
- **arm 2** — every listed path is a real file. A pathspec matching nothing
  renders as `0 of them deciding a launch-time check`, and `git log --
  <nothing>` says so to nobody.
- **arm 3** — the list holds nothing the derivation cannot see. This is the
  one arm the wall pin does **not** have, and the asymmetry is deliberate:
  the wall list has a hand half by design (a render site carrying no header),
  while both clauses here are mechanical. Verified by substituting
  `hookfresh.go` for `agents.go`: arm 1 reds on the missing `agents.go` and
  arm 3 reds on the hand-held `hookfresh.go`, which is exactly the pair of
  findings that mistake should produce.

Shape matched to its sibling `wallrendersources_qa_test.go`: `repoRoot()` plus
an absolute `filepath.Join`, so neither pin enters internal/treepins' derived
tree-wide class (whose key is an enumeration rooted at a **relative** path —
`treewidedoor_qa_test.go`) and neither needs a Makefile door. Arm 1 of `./...`
runs them whole.

### 6. What was NOT changed, and why

- **The surface is still called `wall renderer`.** Two call sites
  (`cmd/posse/main.go` `posse gates`, `promote.go`'s epilogue) and the
  `TestPromoteReadsTheRendererItRecords` pin key on that prefix, and the bead
  asked for the category to be *counted and named separately*, not for a
  rename. The prose inside now names both categories; the heading is an
  operator's grep target and stays.
- **A reading, never a control** (possebinary.go's rule, kept verbatim from
  `WallRenderSources`): it prints, it warns, it decides nothing. Installing
  over a binary dispatching a live fleet stays the operator's (guardrail 3).
- **UNKNOWN, never OK**, when the binary carries no `vcs.revision` — now said
  for both categories in one sentence.
- **No new surface.** Both counts ride the existing `WallRenderer` reading, so
  they cannot disagree with each other or with `posse status`' lag, and both
  reach `posse gates` and the promote epilogue for free. One extra `git log`,
  taken only when there is a lag to narrow.

### 7. The rendering

Over the window in §2, `WallRenderer.Lines("gates")` now prints:

```
wall renderer (gates) · 0.5.3+fe3b8036 is 29 commit(s) behind d4d105d2 in ~/src/posse: 2 of them rendering a wall, 8 of them deciding a launch-time check
  2 rendering a wall — until a descendant of d4d105d2 is installed, every launch on this box renders the OLD seatbelt, … `make install`:
    700cc4c1 ranger-base-csnzn: …
    2dce8da3 ranger-base-6r1rf: …
    git -C ~/src/posse log --oneline fe3b8036..d4d105d2 -- internal/posse/seatbelt.go …
  8 deciding a launch-time check — until a descendant of d4d105d2 is installed, every launch on this box is judged by the OLD parity matrix and the OLD runtime declarations; a launch-time check RENDERS nothing, so the seat reads a clean verdict and the landed fix guards nobody (ranger-base-qr7me: …). `make install`:
    d4d105d2 ranger-base-2zkcy: …
    f5c00378 ranger-base-2cttm: …
    f083e140 ranger-base-l12j9: …
    … and 5 more
    git -C ~/src/posse log --oneline fe3b8036..d4d105d2 -- internal/posse/agents.go …
```

Both counts are named whatever they are, including zero: a category said only
when it is nonzero is a category whose zero cannot be told from a reading that
never ran, which is the silence this bead is about. A category with nothing to
list prints no block and no `git log` — a command an operator runs for an
empty answer teaches them to mistrust the surface. When **neither** list is
touched the whole thing is one sentence that names both lists and the word
FLOOR, and still names `make install`.
