package posse

// Whether the binary that RENDERS this box's walls carries the walls that
// landed (ranger-base-vso72, from ranger-base-mmhhc) — and, since
// ranger-base-atvd4, whether it carries the launch-time CHECKS that landed
// too. Two categories, counted and named separately in one sentence at the
// two surfaces that make the claim: `posse gates` and the promote epilogue.
// LaunchCheckSources' own comment holds that half's reason and measurement.
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
	"os"
	"strings"
)

// WallRenderSources are the tracked paths whose commits can change a wall
// this binary renders: the seatbelt profile, the gate shims and gate shell,
// the L3 hook bodies, the cage's mounts and launcher, the egress allowlist,
// the pane line, the session skills binding, the persona-mode channel that
// carries the PID on a CLI with no launch-time system flag, and the renderer
// stamp itself.
//
// A FLOOR, and read as one. internal/treepins' pin holds the derived half —
// every internal/posse file that renders a `do not edit` header must be
// named here — and the rest are named by hand, so a render site that carries
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
	"internal/posse/gates.go",
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
	"internal/posse/githookparity.go",
	"internal/posse/parity.go",
	"internal/posse/reachability.go",
	"internal/posse/runtime.go",
	"internal/posse/skillsdirtrust.go",
	"internal/posse/suitelock.go",
}

// wallCommitsShown is how many of a category's lagging commits the line
// names before deferring to the git command it prints. Three, for the reason
// PromoteVerdict.Line() caps at three: a promote epilogue has to stay
// readable, and "which ones" is the operator's next command either way.
const wallCommitsShown = 3

// WallRenderer is one reading of "does the installed binary carry the walls
// it renders and the launch-time checks it decides?".
//
// Lag is the measurement, taken by launcherlag.go against the checkout the
// binary's own stamp names. Commits is the wall-rendering subset of that
// lag, newest first, and CheckCommits the launch-check subset; Why and
// CheckWhy are why that subset could not be read — empty when it was,
// including when it was legitimately empty.
type WallRenderer struct {
	Lag          LauncherLag
	Commits      []string
	Why          string
	CheckCommits []string
	CheckWhy     string
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

// ReadWallRenderer narrows a lag reading to the wall-rendering commits in it
// and to the launch-check commits in it. Two `git log`s over two pathspec
// lists, and only when there is a lag to narrow: a binary at the tip has
// nothing to list, and asking anyway would put two forks in the watch pass's
// budget for answers that are always empty.
func ReadWallRenderer(l LauncherLag) WallRenderer {
	w := WallRenderer{Lag: l, Release: Release()}
	if !l.Known() || l.Behind == 0 {
		return w
	}
	w.Commits, w.Why = narrowLag(l, WallRenderSources, "wall-rendering")
	w.CheckCommits, w.CheckWhy = narrowLag(l, LaunchCheckSources, "launch-check")
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
func (a *App) WallRenderer(extra ...string) WallRenderer {
	c := append([]string{}, extra...)
	c = append(c, a.BeadsDirs()...)
	if cwd, err := os.Getwd(); err == nil {
		c = append(c, cwd)
	}
	return ReadWallRenderer(FindLauncher(VersionString(), c))
}

// lagCategory is one narrowing of a lag rendered for a human. The two
// categories differ in prose and in pathspecs and in nothing else, so they
// are one type: a third category added later gets a row, not a branch.
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

// categories is the two narrowings, wall first — the order ranger-base-vso72
// shipped and the order the bead's example sentence reads in.
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
			head + " is release " + w.Release.Tag + " — the walls it renders and the launch-time checks it decides are the ones tagged " + w.Release.Tag + ", which is what a release install carries",
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
			" — every seatbelt, gate shim, gate shell, cage mount set and hook here is rendered by THIS binary, and every parity verdict a launch reports is decided by it, so read this as UNKNOWN and never as OK"}
		// A stamp that was there and could not be believed is said out
		// loud under it. Dropping it would make a mis-stamped or forged
		// release identity render exactly like the ordinary dev build
		// above, which is the one thing this reading must not do.
		if w.Release.Disagreement != "" {
			out = append(out, "  "+w.Release.Disagreement+" — the reading above stands and `make install` is the remedy either way")
		}
		return out
	case l.Behind == 0:
		return []string{head + fmt.Sprintf(" is the tip of %s in %s — the walls it renders and the launch-time checks it decides at every launch are current%s",
			l.Base, AbbrevHome(l.Repo), w.dirtyClause())}
	}
	// Behind > 0. The whole lag leads in every case, because `make install`
	// is the remedy for all of it and the per-category counts only say how
	// loud to read it. Both counts are named whatever they are: a category
	// that is said only when it is nonzero is a category whose zero cannot
	// be told from a reading that never ran, which is the silence
	// ranger-base-atvd4 is about.
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
		// Not an all-clear, and worded so it cannot be read as one: both
		// pathspec lists are floors (see their declarations), and the lag is
		// still a lag. No git command: each would list nothing, and a
		// command an operator runs for an empty answer teaches them to
		// mistrust the surface.
		names := make([]string, 0, len(cats))
		for _, c := range cats {
			names = append(names, c.list)
		}
		return []string{lead + " — none of them touching a path posse renders a wall from or decides a launch-time check in; those paths are current as far as they go, and both lists (" +
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
