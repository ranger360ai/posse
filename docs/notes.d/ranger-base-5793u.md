# The narrowing removed the frontmatter accident for a follower and left it deciding the lane for the head (ranger-base-5793u)

2026-11-13 · gwart · linux 7.2.15 x86_64, go1.27.3, this box, at a6dfd084

`labelMatches` (internal/posse/dispatch.go:1805) takes the lane's winning
label from `out[0].label`, which is `matched[0]` (dispatch.go:1796) — the
first of the PREFERRED PID's own labels that the bead carries, in that PID's
frontmatter order. Before ranger-base-equhp's narrowing that value only NAMED
the lane in the route report. Since the narrowing it also decides membership
(dispatch.go:1808, `hasLabel(m.matched, win)`), so which seats back a bead up
turns on the order two words are written in inside one PID's frontmatter.

equhp removed exactly that accident for a FOLLOWER — a PID is a seat wherever
it lists the winning label, pinned by
`TestQALaneKeepsAPIDThatCarriesTheWinningLabelSecond` — and left it
load-bearing for the HEAD.

## Re-measured, and now a pin

RE-MEASURED 2026-11-13 at a6dfd084, independently of the run on
ranger-base-5793u, through the package's own helpers (`laneOf`,
`writePersona`). One input differs between the arms — the head PID's own label
order. Bead and roster are otherwise identical:

| arm | head PID | roster | bead | lane | label |
|---|---|---|---|---|---|
| A | `[code, qa]` | `developer-2 [code]`, `tester [qa]` | `[code, qa]` | developer, developer-2 | `code` |
| B | `[qa, code]` | same | `[code, qa]` | developer, **tester** | `qa` |
| C | `[crosscut, code, feature, bug]` | `developer-2 [code, feature, bug]`, `watcher [security, audit, crosscut]` | `[code, crosscut]` | developer, **watcher** | `crosscut` |

`crosscut` stands for the real label several of this instance's PIDs share:
that is the instance's own vocabulary, so it is named on the bead and not here
(ADR 0024 D3, restate-and-cite). Nothing in the arm turns on its spelling.

Arm B drops a code-only seat from a code bead and seats a qa-only seat in its
place. Arm C is equhp's own incident shape reached the other way: a code
deliverable whose lane holds the findings-only security PID and not the second
code seat. Against the pre-fix body (`return out` for `return lane`) all three
arms give every seat and only the printed label moves, so the consequence is
NEW with the narrowing.

The three arms are committed as
`TestQALaneWinningLabelTurnsOnTheHeadPIDsFrontmatterOrder`
(`internal/posse/lanewinninglabel_qa_test.go`, arm 2). It is a
CHARACTERIZATION pin, not a decision: it asserts what the code does today, so
that the answer, whichever it is, has to come through it and read the bead.

## What a bead's label order is, measured — and what it does to option (a)

ranger-base-5793u's fix option (a) was "take the winning label from the BEAD's
label order rather than the PID's". That option's premise is that a bead's
label order is the filer's. It is not.

MEASURED 2026-11-13 over this instance's store: **689 of 689 beads carrying
two or more labels are in alphabetical order**, and the typed order is not.
Two positive controls, from the `bd create` lines in the session transcripts
under `~/.claude/projects/*/*.jsonl`:

| typed | bead | read back |
|---|---|---|
| `-l risk -l question` | ranger-base-z5kgj | `['question', 'risk']` |
| `-l devops -l debt` | ranger-base-4arlo | `['debt', 'devops']` |

So bd normalizes labels to alphabetical order at create. `BdIssue.Labels` is
decoded straight from that JSON (`internal/posse/beads.go:492`), so routing
never sees a filer's order at all: there is no path by which it could.

Option (a) therefore means "the alphabetically first of the preferred PID's
matched labels". It happens to give the wanted lane on all three arms above
(`code` sorts before `crosscut`, `qa` and `security`) and it keeps `route_order:`
able to force a lane by changing WHO is preferred. But by the finding's own
criterion — the defect is "an input no key controls, no ADR dials" — the
alphabet fails that test exactly as the frontmatter order does, and unlike the
frontmatter it cannot be dialled per instance at all. That is the fact the
decision was missing, and it is why this bead did not just ship (a).

## Where the decision went

ADR 0011 §4 is where the answer has to be written, whichever it is, so the
decision is the architecture lane's: **ranger-base-svoux**, with the options,
the measurement and gwart's recommendation. ranger-base-5793u is dep-blocked
on it and carries the code change once it lands. ADR 0006 §2: a divergence is
handed to architecture if it changes the design.

## Correction landed here

`docs/notes.d/ranger-base-equhp.md`'s residual paragraph said the residual
"was there before this bead and is unchanged by it ... the fix neither creates
nor widens it". That is right about the persona-order half (which lane's
alphabetically-first persona heads the sort) and wrong about this half: before
the narrowing the head's label order moved only the printed label, and after
it the same input moves the lane. A dated correction is appended to that
fragment rather than rewritten into it.
