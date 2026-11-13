# The narrowing made one PID's frontmatter order decide the lane, and ADR 0011 §4 made it a key (ranger-base-5793u)

2026-11-13 · gwart · linux 7.2.15 x86_64, go1.27.3, this box, at a6dfd084

`labelMatches` (internal/posse/dispatch.go:1805) takes the lane's winning
label from `out[0].label`, which is `matched[0]` (dispatch.go:1796) — the
first of the PREFERRED PID's own labels that the bead carries, in that PID's
frontmatter order. Before ranger-base-equhp's narrowing that value only NAMED
the lane in the route report. Since the narrowing it also decides membership
(dispatch.go:1808, `hasLabel(m.matched, win)`), so which seats back a bead up
turns on the order two words are written in inside one PID's frontmatter.

equhp removed exactly that ordering for a FOLLOWER — a PID is a seat wherever
it lists the winning label, pinned by
`TestQALaneKeepsAPIDThatCarriesTheWinningLabelSecond` — and left it
load-bearing for the HEAD, where its own pin called it "an ordering accident
inside a PID's frontmatter, which is nobody's decision". The question this
bead asked was whether that should be true; ADR 0011 §4 answered it, and the
answer is that at the head it is a KEY (see "What was decided" below).

## Re-measured, and now a pin

RE-MEASURED 2026-11-13 at a6dfd084, independently of the run on
ranger-base-5793u, through the package's own helpers (`laneOf`,
`writePersona`). One input differs between the arms — the head PID's own label
order. Bead and roster are otherwise identical:

| arm | head PID | roster | bead | lane | label |
|---|---|---|---|---|---|
| A | `[code, qa]` | `developer-2 [code]`, `tester [qa]` | `[code, qa]` | developer, developer-2 | `code` |
| B | `[qa, code]` | same | `[code, qa]` | developer, **tester** | `qa` |
| C | `[team, code, feature, bug]` | `developer-2 [code, feature, bug]`, `watcher [security, audit, team]` | `[code, team]` | developer, **watcher** | `team` |

`team` stands for the real label several of this instance's PIDs share: that is
the instance's own vocabulary, so it is named on the bead and not here (ADR
0024 D3, restate-and-cite). It is the spelling ADR 0011 §4 uses for the same
label, so the pin and the ADR read as one thing. Nothing in the arm turns on
it; the first commit of this work was refused by the visibility gate (ADR 0048
D2) for the real literal, as richard's first commit of the ADR was.

Arm B drops a code-only seat from a code bead and seats a qa-only seat in its
place. Arm C is equhp's own incident shape reached the other way: a code
deliverable whose lane holds the findings-only security PID and not the second
code seat. Against the pre-fix body (`return out` for `return lane`) all three
arms give every seat and only the printed label moves, so the consequence is
NEW with the narrowing.

The three arms are pinned as
`TestQALaneWinningLabelTurnsOnTheHeadPIDsFrontmatterOrder`
(`internal/posse/lanewinninglabel_qa_test.go`, arm 2). They went in as a
CHARACTERIZATION pin, holding the behaviour while the decision was out, and
are now a DECISION pin on the same table (see "What was decided"): A and B are
meant to differ, because that difference is the dial, and C is the stated
configuration error.

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
(`code` sorts before `team`, `qa` and `security`) and it keeps `route_order:`
able to force a lane by changing WHO is preferred. But by the finding's own
criterion — the defect is "an input no key controls, no ADR dials" — the
alphabet fails that test exactly as the frontmatter order does, and unlike the
frontmatter it cannot be dialled per instance at all. That is the fact the
decision was missing, and it is why this bead did not just ship (a).

## Where the decision went, and what was decided

ADR 0011 §4 is where the answer had to be written, so the decision was the
architecture lane's (ADR 0006 §2): **ranger-base-svoux**, carrying the
options, this measurement and gwart's recommendation, with ranger-base-5793u
dep-blocked on it.

DECIDED 2026-11-13, richard, ADR 0011 §4 amended at commit 70ab74b7: **option
(b)**. A PID's label order is the SECOND routing key — a PID lists its primary
lane first, `route_order:` decides who heads the candidates, and the head's
own label order decides which of its labels is the lane. A follower's order
decides nothing. No change to `labelMatches`'s logic; what changed is that the
input is named as a dial instead of being called an accident, and that the
consequence is stated: a PID listing a cross-cutting label first makes that
label its lane and every PID carrying it a seat. That is a configuration error
whose remedy is the order, not code — no static rule can refuse it, because a
cross-cutting label and a legitimately shared lane label are spelled alike.
§4 adds the measurement that made it safe (20 of 20 PIDs on this instance
already list their lane first, and over 3,694 beads this rule and the
alphabetical one seat the same personas) and keeps an explicit `lane:` key as
the exit hatch for the day a route report shows the misordering.

What this bead then carried, all of it comment and prose over unchanged logic:
the pin turned from characterization into a decision pin on the same table,
the follower pin's comment narrowed to FOLLOWERS, `routeMatch` and
`labelMatches`'s doc comments restated with the §4 citation, and the equhp
pointer below aimed at the amendment rather than at an open bead.

## Correction landed here

`docs/notes.d/ranger-base-equhp.md`'s residual paragraph said the residual
"was there before this bead and is unchanged by it ... the fix neither creates
nor widens it". That is right about the persona-order half (which lane's
alphabetically-first persona heads the sort) and wrong about this half: before
the narrowing the head's label order moved only the printed label, and after
it the same input moves the lane. A dated correction is appended to that
fragment rather than rewritten into it.
