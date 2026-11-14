## A COMMIT-time gate is install-gated exactly like a wall and a launch-time check (ranger-base-m1obc)

`lagCategory` already said the answer: "The two categories differ in prose
and in pathspecs and in nothing else, so they are one type: a third category
added later gets a row, not a branch." This is that third row, and it is a
row: three entries in `categories()`, two fields on `WallRenderer`, one more
`narrowLag` call, and not a line of new branching anywhere that renders.

### 1. The category

A wall is install-gated because the installed binary RENDERS it
(`WallRenderSources`). A launch-time check is install-gated because the
installed binary DECIDES it at launch (`LaunchCheckSources`). ADR 0024 D2
check 4 is install-gated because the installed binary JUDGES it at COMMIT
time: the `prepare-commit-msg` hook body is a render — `gates.go` writes it
and is already in `WallRenderSources` — but the judge the body pipes through
is a subprocess of whatever `posse` is on PATH, resolved fresh at commit time
on purpose (`vulnPrivateChecks`: "an id's open/closed status is exactly the
live fact a render-time stamp cannot hold"). So a perfectly current hook body
can carry a verdict from a binary 59 commits behind, and the consequence is
the other two's exactly: a landed fix that reaches nobody.

### 2. Why neither existing list could see it, and why hand-adding is wrong

`internal/posse/vulnprivate.go` writes no `do not edit` header, which is the
wall class's derivation key (`internal/treepins/wallrendersources_qa_test.go`
arm 1). It declares no func naming `*posse.Parity` and constructs no
`posse.Runtime`, which are the launch-check class's two keys
(`launchchecksources_qa_test.go`). It is CORRECTLY outside both, exactly as
`runtime.go` was correctly outside `WallRenderSources`, and adding it to
either would red that list's pin AND attach the wrong cost sentence.

### 3. The reading, MEASURED

MEASURED 2026-11-14 on this box over its own exposure window — installed
`0.5.3+7d0f8c31`, `origin/main` at `1d03f9d1`, 59 commits behind:

| category | commits |
|---|---|
| rendering a wall | 8 |
| deciding a launch-time check | 7 |
| judging a commit-time gate | 4 |

The four: `e3b41b30`, `6067b1d7`, `e208dba9`, `72fe7738`. Exactly ONE of them
appeared in the printed reading at all — `6067b1d7`, which also touched
`gates.go` and so was counted under "rendering a wall", beside a cost
sentence about seatbelts, gate shims, cage mounts and hooks. That sentence is
true of `gates.go` and says nothing about why the seat is being refused. The
other three were inside the headline 59 and named by no category.

What the staleness cost, reproduced twice from the reading seat
(ranger-base-v81vo, section 3): ranger-base-ed07e's own-bead exemption is one
of the four, so until it was installed an open `vuln-private` bead's own fix
could carry no descriptive commit subject and its notes fragment could not
land at all — both the `## … (<id>)` heading and `scripts/notes-index.py`'s
generated index line are flagged, so the gate's "rewrite it so the id stands
alone" remedy is unachievable for a fragment.

### 4. The derived half, and why it is an intersection

`internal/treepins/commitgatesources_qa_test.go`, in the discipline its two
siblings keep and with a third spelling, because this class shares none with
them:

- **THE INVOCATION, read off the REAL RENDER and not off an AST.** The hook
  body is shell, so the clause calls `(*App).CommitGuardHook` and reads the
  bytes for every `posse gates <verb>` in them. An AST walk over `gates.go`
  could not do this: the body is assembled from a dozen consts and helpers,
  and the verb a commit actually pipes to is a property of the assembled
  text.
- **THE JUDGE, read off `cmd/posse`'s own dispatch.** For each verb, the
  `if <x> == "<verb>"` arm in `cmd/posse/main.go` and the `RunGates*` entry
  point called inside it; the non-test `internal/posse` file declaring that
  entry point is the member.

**Why an intersection and not a regex over the render.** The rendered hook
body NAMES three verbs and invokes one. MEASURED on this tree: 10
occurrences, 8 of them mentions (seven `install-hooks`, one `adr-census`),
and SIX of those eight inside `echo` refusal text — so a
drop-the-comment-lines filter would still leave six. The dispatch half is
what sorts them: `install-hooks` goes to an installer and `adr-census` to
`RunAdrCensus`, neither a `RunGates*` judge, and only `vuln-private-scan` is
both named by the body and dispatched to one. Over-derivation (a refusal text
that one day quotes `posse gates bd-write`) reds the pin and asks a human,
which is the safe direction and the same trade the wall pin's call closure
makes.

**PRICED**: 0.196s, 0.197s and 0.216s for all four arms over three runs on
this box, 2026-11-14, at a one-minute load average of 1.98 with neither suite
slot held (`go test -count=1 -run CommitGate ./internal/treepins`, warm build
cache). No suite slot, no git, no exec, no tempdir — one hook render in
memory, one `go/parser` pass over `cmd/posse/main.go`, one `os.ReadDir` plus a
parse per non-test file under `internal/posse`. `make treepins-check` is the
door that runs it: since ADR 0098 named that door by what it does NOT run, a
pin added here is doored the moment it is written, with no Makefile edit.

A **FLOOR**, read as one, with nothing keying off the list — the shipped
answer for both siblings and for this one. The line always reports the WHOLE
lag and always names `make install`; the per-category counts only say how
loud. "A READING, NEVER A CONTROL" is unchanged, and no freshness door was
added (ranger-base-v81vo section 4 prices and rejects one: it would be red
for every seat for the whole gap between a landing and an install, over a
condition the seat cannot fix).

### 5. bd-write is a FOURTH category, not this one

`internal/posse/bdwritelock.go` (ADR 0069's write-time layer of the bd shim,
ADR 0073 D2's `posse gates bd-write`) is install-gated through exactly this
callback shape: the bd gate shim is a render, and the judge resolves fresh. By
`wallrenderer.go`'s own rule — two categories that differ in prose and in
pathspecs are two rows — it is another row and not this one:

- the decision is taken at WRITE time, not commit time;
- the consequence is a lost or duplicated bead write (ADR 0073), not a
  refused commit, so it needs its own cost sentence;
- its invoking render is the bd shim and not a hook body, so the derivation
  above excludes it by construction rather than by hand — asserted, in the
  pin's arm 4, both ways round.

Filed as ranger-base-7fjmv with the pricing this bead did not do. The pin's
arm 4 also holds the other boundary: `PrePushHook` names no `posse gates`
judge, and if it ever does, that is a PUSH-time row and not this one.
