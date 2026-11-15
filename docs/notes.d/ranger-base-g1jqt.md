## A commit-guard refusal now names the posse that judged it (ranger-base-g1jqt)

2026-11-14

`ranger-base-li3up` was a P2 filed against correct code: a `prepare-commit-msg`
refusal flagged the committing branch's OWN bead, AGENTS.md "Landing the plane"
says that one id is exempt, the exemption had landed, and the binary that
judged the commit predated it by 59 commits. Nothing at the surface the
committer was reading said which binary that was — so the refusal and the
standing-orders page contradicted each other with nothing on screen to break
the tie, and a seat was dispatched to rediscover an install skew
`ranger-base-v81vo` had already measured.

Read this with `internal/posse/gates.go` (`hookJudgeFunc`, and the four
refusal sites that call it), `internal/posse/renderstamp.go` (the L3 exclusion
this is the other half of) and `docs/notes.d/ranger-base-li3up.md` §4-5.

### 1. What the refusal says now

One shell function, `posse_judged_by`, rendered once above all four walls of
the commit guard and called from every refusal site in it — 21 call sites off
one definition, MEASURED over the rendered hook. With a posse on PATH:

```
  judged by: posse 0.5.3+7d0f8c31 (herdr-native)
  worded by whichever posse last ran 'posse gates install-hooks' here, which
  may be an older one: a refusal is only as new as those two. 'posse gates
  jian-yang' counts the landed commits the judging binary is missing.
```

A commit-guard refusal is a claim made by TWO binaries and the line names
both: the WORDS are the hook file's, rendered by whichever posse last ran
`posse gates install-hooks` there, and the live arm's verdict (ADR 0024 D2
check 4's judge) is taken at commit time by the posse on PATH. Neither is
necessarily a descendant of the rule the refusal cites, which is the whole of
li3up.

### 2. The reading li3up did not have, MEASURED

MEASURED 2026-11-14, this seat, worktree
`posse/jian-yang-posse-ranger-base-g1jqt` at `005ffaed`, linux. This tree's
render of the hook, judged by the binary actually installed on this box — the
same `7d0f8c31` that refused li3up's own-bead commit:

| reading | value |
|---|---|
| this tree's build | `posse 0.5.3+005ffae-dirty` |
| installed `/home/posse/.local/bin/posse` | `posse 0.5.3+7d0f8c31` |
| this tree's HEAD | `005ffaed` |
| the refusal's own line | `judged by: posse 0.5.3+7d0f8c31 (herdr-native)` |

`7d0f8c31` against a HEAD of `005ffaed`, printed in the refusal itself. That
is one sha comparison away from "this refusal may predate the rule it cites",
at the one surface the refused committer is already reading.

Three environments, all MEASURED through a real `git commit` into a fixture
repo with this tree's hook installed (the real git by absolute path, so this
session's own `git` gate shim is not what answers):

| posse on PATH | the line |
|---|---|
| this tree's build | `judged by: posse 0.5.3+005ffae-dirty (herdr-native)` |
| none | `no posse answered on PATH — absent here, or refused by this session's own gate — so any arm that asks the binary was SKIPPED, not passed` |
| present but a shim that refuses the verb | the same absence line |

The absence case is a reading and not a degradation: it says the live arm was
skipped rather than passed, which is the one thing a reader of a clean scan
must not have to guess. It covers the gated case for free — a session whose
PID denies the verb gets stdout nothing and stderr suppressed, which is
exactly the same answer as no binary at all, and both are true in the same
way.

### 3. Three things this deliberately is not

**Not a version LITERAL in the hook body.** `renderstamp.go` excludes the L3
hook bodies from `RenderedByPosse()` because hook identity is byte-for-byte
against the binary's own render (ADR 0023): a version baked into these bytes
would read every hook on the box as degraded after any `make install`,
including installs that change nothing a hook renders. That stance holds
unchanged, and this is the other half of its sentence rather than an
exception to it — the version is read at refusal time, the bytes are
identical under every version (MEASURED: no `0.5.3+<sha>` literal anywhere in
the rendered body), and `probeL3Hooks` is untouched. renderstamp.go carries
that amendment at its own site.

**Not a control.** `wallrenderer.go`'s "A READING, NEVER A CONTROL" applied
at a third surface. `ranger-base-v81vo` §4 priced the control — a freshness
door comparing the installed binary to the tree — and refused it: red for
every seat for the whole gap between a landing and an install, over a
condition no seat can fix. A refusal is already a stop, so a line added to it
costs no committer anything they were not already paying, and it is paid only
on the refusal path.

**Not the lag reading, and that was the bead's open question.** This line
NAMES a binary and POINTS at the reading rather than taking it. `posse gates
<persona>` is where that reading lives, it is a `git log` in another repo plus
a launcher search, and the refusal path is inside git's own hook — the worst
place on the box to block. Taking it here would also have frozen today's
categories into these bytes, and the bytes outlive them: of the four commits
in v81vo's window that changed check 4's judge, exactly one appeared in the
reading at all, under "rendering a wall", beside a cost sentence about
seatbelts and cage mounts (v81vo §3). A pointer gets better on its own, and
did so during this bead — see §4.

### 4. `ranger-base-m1obc` does not own this, and it landed mid-bead

The bead asked whether m1obc's third category row should own this as a fourth
surface instead. It should not, and the two are complements. A category row
is read at `posse gates`, an operator surface typed by someone who already
suspects an install skew; a refused committer is reading a refusal, has not
yet formed that suspicion, and in li3up's case had formed the opposite one.
This line names the binary and sends the reader to m1obc's surface for the
count — it is the join, not a second copy of either.

m1obc landed on `main` as `c156eb73` while this was being written, one commit
past this branch's base. So the surface the pointer names now carries
`CommitGateSources` and says "judging a commit-time gate" in its own sentence
with its own cost — the mis-attribution above is gone at that end. **The
shipped line needed no edit for that**, which is the argument for a pointer
over a reading in one sentence: a refusal that had printed the lag itself
would have shipped v81vo's wrong category and would now need re-rendering and
re-installing on every box to stop.

### 5. What it cost the pins

Nothing structural, which is the measurement worth keeping.

- The four remedy sentences are a CLOSED SET (`remedyclosedset_test.go`,
  ranger-base-mxbq0 finding 1): a clause ADDED to a remedy reds there by
  design. This line is not in a remedy — it is provenance, rendered beside
  the visibility stamp and never inside a `wayThrough` constant — so the
  closed set is untouched, and that is the design reason to keep a stamp out
  of a remedy rather than an accident of where it landed.
- Every refusal-text pin in `internal/posse` and `internal/treepins` reads by
  `strings.Contains`, so an added line reds none of them. The NEVER lists
  were checked for collisions (`dataceiling_qa_test.go`'s four, among them
  `beads db is marked: public`): none.
- `HOOK_DEPS` (`scripts/cleanroom.sh`) leaves `posse` off the clean-room probe
  on the premise that every call in the rendered hooks sits behind
  `command -v posse`. There are two call sites now, and the premise is pinned
  by CONTAINMENT and not by call shape
  (`TestHookDepsNamesEveryCommandTheRenderedHooksCall`, ranger-base-ltfq7), so
  the second site needed no new pin — green first time. Both comments name
  both sites now, since the one that named only check 4's judge would have
  been the next reader's drift.
- In `visGuardRefusal.render` the line goes after the stamp lines and BEFORE
  the override, not last: the override is the only line in that refusal
  anyone can act on.
- **The one pin this bead could not run in its own tree**, and the reason it
  is named here: m1obc's `internal/treepins/commitgatesources_qa_test.go`
  derives its class by reading the REAL rendered commit-guard hook body for
  every `posse gates <verb>` in it — and this bead adds a `posse` call site
  to that body. It landed on main one commit past this branch's base, so no
  run in this worktree could see the interaction. MEASURED instead in a
  throwaway clone at `c156eb73` with this bead's diff applied: it applies
  cleanly, the derivation pin is green in 0.214s, and the wall-renderer
  reading arms and the `hookdeps` containment premise are green there too.
  The derivation is unharmed because `posse version` is not `posse gates
  <verb>` — `cgsInvocation` is `posse gates ([a-z0-9][a-z0-9-]*)`, and the
  one new spelling that regex CAN reach, the pointer sentence's `posse gates
  ${RHQ_PERSONA:-<persona>}`, starts with `$` and matches nothing. Reasoning
  said the same thing; the clone is why that is not all this says.

The pre-push wall gets nothing and wants nothing: its verdict comes from
`RHQ_TOOLS_DENY`, a launch input, so there is no judging binary there for a
reader to compare to anything.

### Verified

- `go build ./...`, `make fmt-check`, `make tree-check`.
- `internal/posse` filtered over `Hook|Refus|Guard|Ceiling|Remedy|Canonical|CommitGuard|VulnPrivate|Judge|Stamp|Shim` in all three arms (arm 3 adding `CheckThree`, arm 2 adding `ShellCommand`): green, 290 passes in arm 3.
- `internal/treepins` filtered over `Hook|Commit|Refus|Guard|Wall|Pid|Fixture`: green.
- `sh -n` over the rendered hook, and a real `git commit` refusal read in the
  three PATH environments of §2.
- The diff applied onto `main` at `c156eb73` in a throwaway clone: m1obc's
  `CommitGate` derivation pin, the wall-renderer reading arms and the
  `hookdeps` containment premise all green there (§5, last bullet).
