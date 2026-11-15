package posse

// Whether the binary that RENDERS this box's walls carries the walls that
// landed (ranger-base-vso72, from ranger-base-mmhhc) — and, since
// ranger-base-atvd4, whether it carries the launch-time CHECKS that landed
// too, and since ranger-base-m1obc whether it carries the COMMIT-time gates
// it JUDGES — and since ranger-base-7fjmv the WRITE-time one it judges for
// every bd write a persona types. Four categories, counted and named
// separately in one sentence at the two surfaces that make the claim: `posse
// gates` and the promote epilogue. LaunchCheckSources', CommitGateSources'
// and BdWriteSources' own comments hold those halves' reasons and
// measurements.
//
// THE GAP. `posse promote` records the renderer and never reads it.
// promote.go stamps `m.Posse = VersionString()` beside the source SHA it
// promoted, and the only reader of that field compares promoted SETS
// ("the manifest predates this binary's promoted set") — never the binary's
// own commit against the source it just promoted from. So on 2026-09-28 a
// binary 40 commits behind the sha it had that moment promoted, one of those
// commits a seatbelt.go change, printed nothing at all. The walls a seat
// launches behind are renders from the INSTALLED binary (renderstamp.go), so
// those 40 commits were 40 commits of gate fixes that no launch could reach,
// and the surface whose whole job is "what is in force here" was silent.
//
// WHY THIS IS NOT LauncherLag, AND WHY IT USES IT. launcherlag.go already
// counts "how many landed commits is the running binary missing" and prints
// it on `posse status` and in the watch pass — that reading is about the
// LAUNCHER's own defects (a merge-back block filed a fourth time). This one
// asks the narrower question the operator acts on differently: of those
// missing commits, how many change a wall this binary RENDERS — because a
// lagging launcher files a duplicate bead, while a lagging renderer puts a
// superseded seatbelt in front of every caged seat on the box and says
// "rendered from the PID at launch" while doing it. Same measurement, so the
// count cannot disagree with `posse status`; different consequence, so it is
// said at the operator's promote and at `posse gates`, where the claim being
// made is precisely "this is the wall".
//
// A READING, NEVER A CONTROL, on possebinary.go's rule: it prints, it warns
// and it decides nothing. Installing over a binary that is dispatching a
// live fleet stays the operator's (guardrail 3). And an unreadable answer is
// UNKNOWN, never OK — a dev build that names no commit cannot say it carries
// anything, and rendering that abstention as silence is how the gap above
// stayed open through three closes that said "reaches a seat at its next
// launch".
//
// WITH ONE ANSWER THAT IS NEITHER (ranger-base-ctath). A RELEASE install has
// no checkout to compare against and never will, so the UNKNOWN above was
// permanent there rather than unreadable: `posse gates | head -1` said it on
// every release, forever. A release binary carries the tag it was cut from
// now (releasestamp.go), and the walls it renders are that tag's by
// construction — a third state, said in its own sentence, which claims
// nothing about whether a newer release exists because reading that would be
// a network call on a launch path. Everything else in this file is unchanged:
// a known lag is still counted, and a dirty or unnamed build still reads
// UNKNOWN.

import (
	"fmt"
	"io"
	"strings"
)

// WallRenderSources are the tracked paths whose commits can change a wall
// this binary renders: the seatbelt profile and, on Linux, the same tier's
// bwrap args file (ADR 0080 D18), the gate shims and gate shell
// (with the git shim's hook-skip arm),
// the L3 hook bodies, the cage's mounts and launcher, the egress allowlist,
// the pane line, the session skills binding, the persona-mode channel that
// carries the PID on a CLI with no launch-time system flag, and the renderer
// stamp itself.
//
// A FLOOR, and read as one. internal/treepins' pin holds the derived half —
// every internal/posse file that renders a `do not edit` header must be
// named here, and so must every file declaring a `render*` func reached from
// one through `render*` calls (the BODY renderers: hookskip.go renders the
// shim's hook-skip arm and gates.go writes its header, ranger-base-jk380) —
// and the rest are named by hand, so a render site that carries
// no such header could still be missing. That is why nothing keys OFF this
// list: the line below always reports the WHOLE lag and always names `make
// install`, and the wall count only says how loud to read it. An empty wall
// count is never an all-clear about the install.
//
// hookfresh.go is deliberately NOT here, against the bead's own sketch: it
// READS walls (the sweep) and renders none, so a commit to it changes what
// posse reports and nothing a seat launches behind. Counting a reader would
// inflate the one number whose value is that it is specific.
var WallRenderSources = []string{
	"internal/posse/seatbelt.go",
	"internal/posse/bwrap.go",
	"internal/posse/wall.go",
	"internal/posse/gates.go",
	"internal/posse/hookskip.go",
	"internal/posse/cage.go",
	"internal/posse/cagelauncher.go",
	"internal/posse/cageinner.go",
	"internal/posse/egress.go",
	"internal/posse/paneline.go",
	"internal/posse/hooksredirect.go",
	"internal/posse/skills.go",
	"internal/posse/personamode.go",
	"internal/posse/renderstamp.go",
}

// LaunchCheckSources are the tracked paths whose commits can change a
// launch-time CHECK this binary DECIDES: the parity matrix and the four
// arms that write verdicts into it, and the Runtime declarations those
// verdicts read.
//
// THE THIRD CATEGORY, and why the wall count could not cover it
// (ranger-base-atvd4, from ranger-base-qr7me). A wall is install-gated
// because the installed binary RENDERS it. A launch-time check is
// install-gated for exactly the same reason — the installed binary decides
// it — but it renders no file, so WallRenderSources' derived class cannot
// see it: that class is keyed on a `do not edit` header a render writes
// about itself (internal/treepins' arm 1), and a check writes no file to
// put a header on. runtime.go is therefore CORRECTLY outside
// WallRenderSources, and hand-adding it would red that pin.
//
// MEASURED 2026-10-31 over ranger-base-qr7me's own exposure window —
// installed fe3b8036 against main at d4d105d2, 29 commits:
//
//	wall          2  (skills.go, personamode.go)
//	launch-check  8  (two of them f083e140 and d4d105d2, the grok
//	                 project-config detector itself)
//
// So the operator read "29 behind, 2 of them rendering a wall" over eight
// hours in which a landed project-config detector guarded no grok launch on
// this box, and nothing in the reading named the category it was in. The
// consequence is the wall's consequence exactly — a landed fix that does not
// guard live seats — so it is said in the same sentence, at the same two
// surfaces (`posse gates` and the promote epilogue).
//
// A FLOOR, read as one, for WallRenderSources' reason and with the same
// derivation discipline: internal/treepins' launchchecksources pin holds the
// derived half (every non-test internal/posse file that declares a func
// naming *Parity, or constructs a Runtime), and nothing keys OFF this list —
// the line always reports the WHOLE lag and always names `make install`.
//
// NOT DISJOINT from WallRenderSources, and not pinned to be: a file can both
// render a wall and decide a check, and a commit to one is honestly counted
// in both categories. The two counts are per-category and do not sum; the
// headline is the whole lag either way.
var LaunchCheckSources = []string{
	"internal/posse/agents.go",
	"internal/posse/bwrap.go",
	"internal/posse/githookparity.go",
	"internal/posse/parity.go",
	"internal/posse/policypin.go",
	"internal/posse/reachability.go",
	"internal/posse/runtime.go",
	"internal/posse/skillsdirtrust.go",
	"internal/posse/suitelock.go",
	"internal/posse/wallwitness.go",
}

// CommitGateSources are the tracked paths whose commits can change a
// COMMIT-time gate this binary JUDGES: the judge behind every `posse gates
// <verb>` the commit-guard hook body invokes. Today that is ADR 0024 D2
// check 4 and vulnprivate.go alone.
//
// THE THIRD CATEGORY, and why neither existing list can see it
// (ranger-base-m1obc, from ranger-base-v81vo). A wall is install-gated
// because the installed binary RENDERS it; a launch-time check because the
// installed binary DECIDES it at launch; this one because the installed
// binary JUDGES it at COMMIT time. The hook BODY is a render — gates.go
// writes it and is already in WallRenderSources — but the judge it pipes
// through is a SUBPROCESS of whatever `posse` is on PATH, resolved fresh at
// commit time on purpose (vulnPrivateChecks: "an id's open/closed status is
// exactly the live fact a render-time stamp cannot hold"). So the body can
// be perfectly current while the verdict is a binary behind, and the whole
// install-gated consequence lands on a file in neither list:
// vulnprivate.go writes no `do not edit` header, which is the wall class's
// key (internal/treepins/wallrendersources_qa_test.go arm 1), and it
// declares no func naming *Parity and constructs no Runtime, which are the
// launch-check class's two keys (launchchecksources_qa_test.go). It is
// CORRECTLY outside both, exactly as runtime.go was correctly outside
// WallRenderSources, and hand-adding it to either would red that list's pin
// AND attach the wrong cost sentence.
//
// MEASURED 2026-11-14 on this box over its own exposure window — installed
// 0.5.3+7d0f8c31 against origin/main at 1d03f9d1, 59 commits:
//
//	rendering a wall             8
//	deciding a launch-time check 7
//	judging a commit-time gate   4  (e3b41b30, 6067b1d7, e208dba9, 72fe7738)
//
// Of those four, exactly ONE appeared in the printed reading at all —
// 6067b1d7, which also touched gates.go and so was counted under "rendering
// a wall", beside a cost sentence about seatbelts, gate shims, cage mounts
// and hooks. That sentence is true of gates.go and says nothing about why
// the seat is being refused. The other three were inside the headline 59 and
// named by no category. What the staleness cost, reproduced twice from the
// reading seat: ranger-base-ed07e's own-bead exemption is one of the four,
// so until it was installed an open vuln-private bead's own fix could carry
// no descriptive commit subject and its notes fragment could not land at all
// — and the gate's "rewrite it so the id stands alone" remedy is
// unachievable for a fragment, both its `## … (<id>)` heading and
// scripts/notes-index.py's generated index line being flagged.
// docs/notes.d/ranger-base-v81vo.md section 3 has the whole reading.
//
// A FLOOR, read as one, for WallRenderSources' reason and with the same
// derivation discipline: internal/treepins' commitgatesources pin holds the
// derived half (the verb must be named in the rendered commit-guard hook
// body AND dispatched by cmd/posse to a `RunGates*` entry point, and the
// internal/posse file declaring that entry point is the member), and nothing
// keys OFF this list — the line always reports the WHOLE lag and always
// names `make install`.
//
// NOT bd-write, which is a FOURTH category and not this one — BdWriteSources
// below, filed as ranger-base-m1obc done-when 4 and landed and priced by
// ranger-base-7fjmv.
// bdwritelock.go is install-gated through the same callback shape — the bd
// gate shim is a render, and `posse gates bd-write` resolves its judge fresh
// — but it is taken at WRITE time, not commit time, its consequence is a
// lost or duplicated bead write (ADR 0073) and not a refused commit, and its
// invoking render is the bd shim rather than a hook body. By this file's own
// rule — two categories that differ in prose and in pathspecs are two rows —
// that is another row, and internal/treepins' derivation excludes it by
// construction because the bd shim is not a hook body — asserted both ways
// round, so the two rows cannot merge because a clause widened.
// ranger-base-7fjmv carries it, and prices it: this bead did not.
//
// NOT DISJOINT from either sibling, and not pinned to be, for the reason
// LaunchCheckSources gives: gates.go renders the hook body and would be
// honestly counted in two categories if it ever also held a judge. The
// per-category counts do not sum; the headline is the whole lag either way.
var CommitGateSources = []string{
	"internal/posse/vulnprivate.go",
}

// BdWriteSources are the tracked paths whose commits can change the
// WRITE-time gate this binary JUDGES: the judge behind the one `posse gates
// <verb>` the bd shim's rendered write-verb tail EXECS, and the helpers that
// judge calls on its way to a verdict. Today that is ADR 0073 D2's bd-write
// arm — bdwriteshim.go's entry point, bdwritelock.go's per-store flock and
// bdline.go's line-cliff projection.
//
// THE FOURTH CATEGORY, and why the commit-time row is not it
// (ranger-base-7fjmv, from ranger-base-m1obc done-when 4). It is
// install-gated through the same callback shape its sibling is: the bd gate
// SHIM is a render — gates.go writes it and is already in WallRenderSources
// — and `exec "$posse_bw_bin" gates bd-write --real <bd> -- "$@"` resolves
// its judge fresh from whatever posse is on PATH, on purpose (ADR 0073 D2:
// the lock must be held by the process that becomes bd). By this file's own
// rule — two categories that differ in prose and in pathspecs are two rows —
// it is nevertheless not the commit-time row:
//
//   - the decision is taken at WRITE time, not commit time;
//   - the consequence is a lost or duplicated bead write, which nobody is
//     refused and nobody is told about (bd prints `Created issue: <id>` and
//     exits 0 either way: ADR 0073's rig, 34-36 of 48 concurrent creates
//     lost, a `close` reading back `in_progress` in 1 of 3 trials), so it
//     needs its own cost sentence;
//   - the invoking render is the bd shim, not a hook body, so
//     internal/treepins/commitgatesources_qa_test.go's derivation excludes
//     it by construction — asserted from both rows, so the two cannot merge
//     because a clause widened.
//
// MEASURED 2026-11-14 on this box. In its own exposure window — installed
// 0.5.3+7d0f8c31 against main at 35e8cf41, 75 commits — the counts are 12
// rendering a wall, 9 deciding a launch-time check, 4 judging a commit-time
// gate and ZERO judging a write-time gate. That zero is the whole reason the
// row is a row: it is said out loud, and an operator can tell it from a
// reading that never ran.
//
// WHAT A STALE WRITE-TIME JUDGE COST, read off this box's own history.
// Exactly three commits in main's 2,756 have ever touched these paths, and
// what the reading would have said about two of them is the mis-attribution
// ranger-base-m1obc measured, one row over:
//
//	2a9f4e06 (2026-10-12 14:12, ADR 0073 B1) touched bdwritelock.go and
//	         beads.go and NO path in any of the three lists that existed —
//	         so it sat inside the headline, named by no category at all.
//	21508a1a (2026-10-12 16:19, ADR 0073 B2) touched gates.go, so it was
//	         counted under "rendering a wall", beside a cost sentence about
//	         seatbelts, gate shims, cage mounts and hooks; its judge half
//	         (bdwriteshim.go, bdwritelock.go, cmd/posse) was named by
//	         nothing, and a seat reading that line had no way to know that
//	         every bd write it typed was going unlocked.
//	6b4cd64d (ADR 0072 B1) created bdline.go, the projection the judge
//	         refuses on.
//
// How long that window was is NOT MEASURED and cannot be from here: of 101
// wall-renderer readings across the 490 session transcripts this box retains
// (mtimes 2026-11-07..2026-11-14), 10 name a distinct installed stamp, 8 of
// those resolve in this tree and all 8 already carry both ADR 0073 commits —
// so this row reads 0 in every retained reading, and the window in which it
// would not predates the corpus. The other 2 are 0.5.0 stamps this tree does
// not hold: their lag over these paths is UNKNOWN here, never zero.
//
// What the row is worth is the traffic it guards, and that IS measured:
// 3,264 bd WRITE calls typed in Bash tool calls across those transcripts in
// the week to 2026-11-14 (3,286 regex matches less 22 that are not bd verbs),
// peaking at 1,074 in one day across 123 seats — every one of them judged and
// serialized by whichever posse was installed, not by whichever posse had
// landed. docs/notes.d/ranger-base-7fjmv.md has the commands.
//
// A FLOOR, read as one, for WallRenderSources' reason and with the same
// derivation discipline: internal/treepins' bdwritesources pin holds the
// derived half (the verb the rendered bd shim EXECS, dispatched by cmd/posse
// to a `RunGates*` entry point, plus that entry point's own call closure
// over the helpers sharing its name stem), and nothing keys OFF this list —
// the line always reports the WHOLE lag and always names `make install`. The
// residual the closure cannot see is a helper whose name does not carry the
// stem: bdlineshim.go's refusal wording and gate log are reached from this
// judge and are not here, which is a floor and not a hand-added path.
//
// NOT DISJOINT from any sibling, and not pinned to be, for the reason
// LaunchCheckSources gives. The per-category counts do not sum; the headline
// is the whole lag either way.
var BdWriteSources = []string{
	"internal/posse/bdline.go",
	"internal/posse/bdwritelock.go",
	"internal/posse/bdwriteshim.go",
}

// wallCommitsShown is how many of a category's lagging commits the line
// names before deferring to the git command it prints. Three, for the reason
// PromoteVerdict.Line() caps at three: a promote epilogue has to stay
// readable, and "which ones" is the operator's next command either way.
const wallCommitsShown = 3

// WallRenderer is one reading of "does the installed binary carry the walls
// it renders, the launch-time checks it decides, the commit-time gates it
// judges and the write-time gate it judges for every bd write?".
//
// Lag is the measurement, taken by launcherlag.go against the checkout the
// binary's own stamp names. Commits is the wall-rendering subset of that
// lag, newest first, CheckCommits the launch-check subset, GateCommits the
// commit-gate subset and WriteCommits the bd-write subset; Why, CheckWhy,
// GateWhy and WriteWhy are why that subset could not be read — empty when it
// was, including when it was legitimately empty.
//
// Four pairs and no map, because categories() is the one place the
// narrowings are composed and a row there is cheaper to read than a key
// nothing enumerates (ranger-base-m1obc, taking lagCategory's own rule:
// a third category gets a row, not a branch; ranger-base-7fjmv's fourth got
// one too, and still no branch anywhere that renders).
type WallRenderer struct {
	Lag          LauncherLag
	Commits      []string
	Why          string
	CheckCommits []string
	CheckWhy     string
	GateCommits  []string
	GateWhy      string
	WriteCommits []string
	WriteWhy     string
	// Release is what the binary knows about itself when the lag could not
	// be read at all: on a release install there is no checkout to compare
	// against and never will be, and the walls are that tag's
	// (releasestamp.go, ranger-base-ctath). Read in every case so one
	// reading carries both answers, and consulted only where the lag
	// abstains — a release binary on a box that DOES hold its commit is
	// counted like any other, because the lag is the richer answer and the
	// release state must never mask one.
	Release ReleaseIdentity
}

// ReadWallRenderer narrows a lag reading to the wall-rendering commits in
// it, the launch-check commits in it, the commit-gate commits in it and the
// bd-write commits in it. Four `git log`s over four pathspec lists, and only
// when there is a lag to narrow: a binary at the tip has nothing to list,
// and asking anyway would put four forks in the watch pass's budget for
// answers that are always empty.
func ReadWallRenderer(l LauncherLag) WallRenderer {
	w := WallRenderer{Lag: l, Release: Release()}
	if !l.Known() || l.Behind == 0 {
		return w
	}
	w.Commits, w.Why = narrowLag(l, WallRenderSources, "wall-rendering")
	w.CheckCommits, w.CheckWhy = narrowLag(l, LaunchCheckSources, "launch-check")
	w.GateCommits, w.GateWhy = narrowLag(l, CommitGateSources, "commit-gate")
	w.WriteCommits, w.WriteWhy = narrowLag(l, BdWriteSources, "bd-write")
	return w
}

// narrowLag lists the commits of a lag that touch one pathspec list, newest
// first. noun names the category in the abstention, so a reader who gets
// UNKNOWN knows WHICH question went unanswered.
func narrowLag(l LauncherLag, sources []string, noun string) (commits []string, why string) {
	args := append([]string{"log", "--oneline", "--no-decorate", l.Rev + ".." + l.Base, "--"}, sources...)
	out, err := git(l.Repo, args...)
	if err != nil {
		return nil, "cannot list the " + noun + " commits in " + l.Rev + ".." + l.Base + " (" + errText(err) + ")"
	}
	for _, ln := range strings.Split(out, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			commits = append(commits, ln)
		}
	}
	return commits, ""
}

// WallRenderer is the instance's own reading. extra are directories to look
// for the binary's own checkout in BEFORE the configured ones — `posse
// promote` passes the constitution source, so a box whose constitution is
// itself a posse checkout is counted against the very tree it is promoting
// from rather than against whichever repo `beads:` happens to name first.
//
// Through launcherCandidates, which is the ONE list (ranger-base-eiwhk): this
// function used to compose its own copy of it, and `posse gates` — the
// surface that bead's symptom was read off — is this one, not the status
// line's. A candidate source added to one copy and not the other is a fix
// that looks landed and is not.
func (a *App) WallRenderer(extra ...string) WallRenderer {
	return ReadWallRenderer(FindLauncher(VersionString(), a.launcherCandidates(true, extra...)))
}

// lagCategory is one narrowing of a lag rendered for a human. The
// categories differ in prose and in pathspecs and in nothing else, so they
// are one type: a third category added later gets a row, not a branch. One
// was (ranger-base-m1obc, the commit-time gate) and it did — three rows in
// categories(), two fields on WallRenderer, one `narrowLag` call, and not a
// line of new branching anywhere that renders. So did the fourth
// (ranger-base-7fjmv, the write-time bd gate), at the same price.
type lagCategory struct {
	commits []string
	why     string
	// gerund completes "<n> of them <gerund>" in the summary clause.
	gerund string
	// question completes "UNKNOWN <question>" when the subset is unreadable.
	question string
	// cost is the consequence sentence, printed only when the count is
	// nonzero: what the box is giving away until `make install`.
	cost string
	// list is the identifier an operator reads to see what the floor covers.
	list string
	// sources are the pathspecs, for the `git log` the block prints.
	sources []string
}

// categories is the four narrowings, wall first — the order
// ranger-base-vso72 shipped and the order the bead's example sentence reads
// in — then launch-check, then commit-gate, then bd-write, each appended in
// the order it landed. The order is the sentence's order at both surfaces, so
// a reader who learned the line once does not have to re-learn it when a row
// is added.
func (w WallRenderer) categories() []lagCategory {
	return []lagCategory{{
		commits:  w.Commits,
		why:      w.Why,
		gerund:   "rendering a wall",
		question: "how many of them render a wall",
		cost: "every launch on this box renders the OLD seatbelt, gate shims, gate shell, cage mounts and hooks, and says \"rendered from the PID at launch\" while doing it (ranger-base-mmhhc: 12 caged launches rendered a superseded credential deny)." +
			" `make install`:",
		list:    "WallRenderSources",
		sources: WallRenderSources,
	}, {
		commits:  w.CheckCommits,
		why:      w.CheckWhy,
		gerund:   "deciding a launch-time check",
		question: "how many of them decide a launch-time check",
		cost: "every launch on this box is judged by the OLD parity matrix and the OLD runtime declarations; a launch-time check RENDERS nothing, so the seat reads a clean verdict and the landed fix guards nobody (ranger-base-qr7me: grok's project-config detector landed at 07:15 and again at 08:47, and no grok launch here checked a single entry until `make install` at 09:40)." +
			" `make install`:",
		list:    "LaunchCheckSources",
		sources: LaunchCheckSources,
	}, {
		commits:  w.GateCommits,
		why:      w.GateWhy,
		gerund:   "judging a commit-time gate",
		question: "how many of them judge a commit-time gate",
		cost: "every commit in every repo whose hooks this box installed is judged by the OLD judge, and the hook BODY that pipes to it is a render that can be perfectly current while the judge behind it is a binary behind — so the gate refuses what the landed judge would pass and the committer has no remedy to type (ranger-base-v81vo: a posse 59 commits behind ran ADR 0024 D2 check 4 without ranger-base-ed07e's own-bead exemption, and an open vuln-private bead's own fix could carry no descriptive commit subject and its notes fragment could not land at all — reproduced twice from the reading seat)." +
			" `make install`:",
		list:    "CommitGateSources",
		sources: CommitGateSources,
	}, {
		commits:  w.WriteCommits,
		why:      w.WriteWhy,
		gerund:   "judging a write-time gate",
		question: "how many of them judge a write-time gate",
		cost: "every bd WRITE any persona on this box types is serialized by the OLD lock and judged by the OLD line-cliff projection, and the bd SHIM that execs it is a render that can be perfectly current while the judge behind it is a binary behind — and this one refuses nobody: a write that loses the race prints `Created issue: <id>` and exits 0 like any other, so the cost lands on whoever reads the graph next (ADR 0073's rig: 34-36 of 48 concurrent creates lost, a `close` reading back `in_progress` in 1 of 3 trials; 3,264 bd writes typed here in the week to 2026-11-14, 1,074 of them in one day across 123 seats)." +
			" `make install`:",
		list:    "BdWriteSources",
		sources: BdWriteSources,
	}}
}

// summary is this category's clause in the one-line count: "2 of them
// rendering a wall", or an UNKNOWN that names its own question.
func (c lagCategory) summary() string {
	if c.why != "" {
		return "UNKNOWN " + c.question
	}
	return fmt.Sprintf("%d of them %s", len(c.commits), c.gerund)
}

// said reports whether this category has a block to print: a count to act
// on, or a question it could not answer.
func (c lagCategory) said() bool { return c.why != "" || len(c.commits) > 0 }

// cmd is the git command that lists this category's commits, printed beside
// them so the operator's next move needs no recall of the pathspec list.
func (c lagCategory) cmd(l LauncherLag) string {
	return fmt.Sprintf("git -C %s log --oneline %s..%s -- %s",
		AbbrevHome(l.Repo), l.Rev, l.Base, strings.Join(c.sources, " "))
}

// lines is this category's block, under the shared count line. Empty when
// there is nothing to say: a zero count is said once, in the summary, and
// repeating it as a block is how a surface teaches an operator to skim.
func (c lagCategory) lines(l LauncherLag) []string {
	switch {
	case c.why != "":
		// The lead already carries "UNKNOWN <question>"; what it cannot
		// carry is the reason, which is the whole of what a reader does next.
		return []string{"  " + c.question + " is UNKNOWN: " + c.why + " — `make install` is the remedy either way"}
	case len(c.commits) == 0:
		return nil
	}
	out := []string{"  " + fmt.Sprintf("%d %s — until a descendant of %s is installed, ", len(c.commits), c.gerund, l.Base) + c.cost}
	for i, cm := range c.commits {
		if i == wallCommitsShown {
			out = append(out, fmt.Sprintf("    … and %d more", len(c.commits)-wallCommitsShown))
			break
		}
		out = append(out, "    "+cm)
	}
	return append(out, "    "+c.cmd(l))
}

// Lines is the reading rendered for a human, one string per output line and
// never empty: an operator who typed a command is owed a sentence, and an
// abstention that renders as silence reads as an all-clear. where names the
// caller, the same way ReportHookWall's does — a reader who cannot tell the
// check ran is back to trusting that it did.
func (w WallRenderer) Lines(where string) []string {
	l := w.Lag
	head := fmt.Sprintf("wall renderer (%s) · %s", where, l.Version)
	switch {
	case !l.Known() && w.Release.IsRelease():
		// THE RELEASE STATE (ranger-base-ctath). A third answer, not a
		// rebranded OK: it says which release's walls are in force, names
		// the abstention that is still true underneath it, and does not
		// claim to be the latest, because that is a question about
		// github.com and a launch path takes no network call. Reachable
		// only from the release script's own stamp — see releasestamp.go on
		// why no dev build can get here, and on the three refusals that
		// keep a dirty, mis-tagged or commitless build out.
		return []string{
			head + " is release " + w.Release.Tag + " — the walls it renders, the launch-time checks it decides and the commit-time and write-time gates it judges are the ones tagged " + w.Release.Tag + ", which is what a release install carries",
			"  no local comparison was possible — " + l.Why + " — and whether a newer release has been published is not read here: that is a network call, and this line is printed at every launch. So this names which release is in force, never that it is the newest one — `brew upgrade ranger360ai/tap/posse` is what moves this box forward, and `make install` from a checkout is what a fleet box does instead",
		}
	case !l.Known():
		// UNKNOWN and not OK, explicitly. A dev build carries no
		// vcs.revision (cagestale.go documents the shape: a `go build` from
		// a linked worktree carries neither a pseudo-version nor vcs.*), so
		// there is no commit to compare — and the walls it renders and the
		// checks it decides are still the ones every seat on this box
		// launches behind.
		out := []string{head + " · UNKNOWN whether this binary carries the wall renders and the launch-time checks that landed: " + l.Why +
			" — every seatbelt, gate shim, gate shell, cage mount set and hook here is rendered by THIS binary, every parity verdict a launch reports is decided by it, and every commit-time gate verdict a hook body pipes out to it and every write-time verdict the bd shim execs into it are judged by it, so read this as UNKNOWN and never as OK"}
		// A stamp that was there and could not be believed is said out
		// loud under it. Dropping it would make a mis-stamped or forged
		// release identity render exactly like the ordinary dev build
		// above, which is the one thing this reading must not do.
		if w.Release.Disagreement != "" {
			out = append(out, "  "+w.Release.Disagreement+" — the reading above stands and `make install` is the remedy either way")
		}
		return out
	case l.Behind == 0:
		return []string{head + fmt.Sprintf(" is the tip of %s in %s — the walls it renders, the launch-time checks it decides at every launch, the commit-time gates it judges at every commit and the write-time gate it judges at every bd write are current%s",
			l.Base, AbbrevHome(l.Repo), w.dirtyClause())}
	}
	// Behind > 0. The whole lag leads in every case, because `make install`
	// is the remedy for all of it and the per-category counts only say how
	// loud to read it. EVERY count is named whatever it is: a category
	// that is said only when it is nonzero is a category whose zero cannot
	// be told from a reading that never ran, which is the silence
	// ranger-base-atvd4 is about — and the silence ranger-base-m1obc found
	// again, for the commit-time gate, inside a headline of 59.
	cats := w.categories()
	sums := make([]string, 0, len(cats))
	for _, c := range cats {
		sums = append(sums, c.summary())
	}
	lead := head + fmt.Sprintf(" is %d commit(s) behind %s in %s%s: ",
		l.Behind, l.Base, AbbrevHome(l.Repo), w.dirtyClause()) + strings.Join(sums, ", ")

	said := false
	for _, c := range cats {
		said = said || c.said()
	}
	if !said {
		// Not an all-clear, and worded so it cannot be read as one: every
		// pathspec list is a floor (see their declarations), and the lag is
		// still a lag. No git command: each would list nothing, and a
		// command an operator runs for an empty answer teaches them to
		// mistrust the surface.
		names := make([]string, 0, len(cats))
		for _, c := range cats {
			names = append(names, c.list)
		}
		// "all N lists" counts the rows rather than spelling a number, so a
		// fourth category cannot leave this sentence claiming three
		// (ranger-base-m1obc: a row, not a branch, and not a literal either).
		return []string{lead + " — none of them touching a path posse renders a wall from, decides a launch-time check in, judges a commit-time gate in or judges a bd write in; those paths are current as far as they go, and " +
			fmt.Sprintf("all %d lists (", len(names)) +
			strings.Join(names, ", ") + ") are a FLOOR, so `make install` closes the rest"}
	}
	out := []string{lead}
	for _, c := range cats {
		out = append(out, c.lines(l)...)
	}
	return out
}

// dirtyClause says what the "-dirty" suffix in a version string does not:
// the stamp names the last COMMIT of a tree that also had uncommitted edits,
// so the wall this binary renders is in no commit at all and the count is a
// floor. behindSentence() says the same thing for the launcher reading.
func (w WallRenderer) dirtyClause() string {
	if !w.Lag.Dirty {
		return ""
	}
	return ", and its tree had uncommitted edits, so the wall it renders is in no commit at all and this is a floor"
}

// Actionable is whether the reading found something for a human: a lag, an
// answer it could not read, or a release stamp it could not believe.
//
// A believed release identity is NOT actionable here, and that is the whole
// behavioural difference ranger-base-ctath buys. On a release install the
// walls are the tag's by construction, nothing local can be installed to
// change that, and `brew upgrade` is a decision about a newer release this
// binary deliberately does not read — so reporting it as "something to act
// on" would be the permanent UNKNOWN again, wearing a bool.
func (w WallRenderer) Actionable() bool {
	switch {
	case w.Release.Disagreement != "":
		return true
	case !w.Lag.Known():
		return !w.Release.IsRelease()
	default:
		return w.Lag.Behind > 0
	}
}

// ReportWallRenderer prints the reading and reports whether it found
// something to act on — a lag, or an answer it could not read. The bool is
// for a caller that wants to say more, never a refusal: `posse promote` is
// the operator's command and this is a reading (see the file header).
func (a *App) ReportWallRenderer(out io.Writer, where string, extra ...string) bool {
	w := a.WallRenderer(extra...)
	for _, ln := range w.Lines(where) {
		fmt.Fprintln(out, ln)
	}
	return w.Actionable()
}
