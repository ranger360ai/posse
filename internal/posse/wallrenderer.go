package posse

// Whether the binary that RENDERS this box's walls carries the walls that
// landed (ranger-base-vso72, from ranger-base-mmhhc).
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

// wallCommitsShown is how many of the lagging wall commits the line names
// before deferring to the git command it prints. Three, for the reason
// PromoteVerdict.Line() caps at three: a promote epilogue has to stay
// readable, and "which ones" is the operator's next command either way.
const wallCommitsShown = 3

// WallRenderer is one reading of "does the binary rendering this box's walls
// carry the wall renders that landed?".
//
// Lag is the measurement, taken by launcherlag.go against the checkout the
// binary's own stamp names. Commits is the wall-rendering subset of that
// lag, newest first. Why is why the subset could not be read — empty when it
// was, including when it was legitimately empty.
type WallRenderer struct {
	Lag     LauncherLag
	Commits []string
	Why     string
}

// ReadWallRenderer narrows a lag reading to the wall-rendering commits in
// it. One `git log` over the pathspecs, and only when there is a lag to
// narrow: a binary at the tip has nothing to list, and asking anyway would
// put a fork in the watch pass's budget for an answer that is always empty.
func ReadWallRenderer(l LauncherLag) WallRenderer {
	w := WallRenderer{Lag: l}
	if !l.Known() || l.Behind == 0 {
		return w
	}
	args := append([]string{"log", "--oneline", "--no-decorate", l.Rev + ".." + l.Base, "--"}, WallRenderSources...)
	out, err := git(l.Repo, args...)
	if err != nil {
		w.Why = "cannot list the wall-rendering commits in " + l.Rev + ".." + l.Base + " (" + errText(err) + ")"
		return w
	}
	for _, ln := range strings.Split(out, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			w.Commits = append(w.Commits, ln)
		}
	}
	return w
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

// Lines is the reading rendered for a human, one string per output line and
// never empty: an operator who typed a command is owed a sentence, and an
// abstention that renders as silence reads as an all-clear. where names the
// caller, the same way ReportHookWall's does — a reader who cannot tell the
// check ran is back to trusting that it did.
func (w WallRenderer) Lines(where string) []string {
	l := w.Lag
	head := fmt.Sprintf("wall renderer (%s) · %s", where, l.Version)
	switch {
	case !l.Known():
		// UNKNOWN and not OK, explicitly. A dev build carries no
		// vcs.revision (cagestale.go documents the shape: a `go build` from
		// a linked worktree carries neither a pseudo-version nor vcs.*), so
		// there is no commit to compare — and the walls it renders are still
		// the walls every seat on this box launches behind.
		return []string{head + " · UNKNOWN whether this binary carries the wall renders that landed: " + l.Why +
			" — every seatbelt, gate shim, gate shell, cage mount set and hook here is rendered by THIS binary, so read this as UNKNOWN and never as OK"}
	case l.Behind == 0:
		return []string{head + fmt.Sprintf(" is the tip of %s in %s — the walls it renders at every launch are current%s",
			l.Base, AbbrevHome(l.Repo), w.dirtyClause())}
	}
	// Behind > 0. The whole lag leads in every case, because `make install`
	// is the remedy for all of it and the wall count only says how loud.
	behind := head + fmt.Sprintf(" is %d commit(s) behind %s in %s%s",
		l.Behind, l.Base, AbbrevHome(l.Repo), w.dirtyClause())
	cmd := fmt.Sprintf("git -C %s log --oneline %s..%s -- %s",
		AbbrevHome(l.Repo), l.Rev, l.Base, strings.Join(WallRenderSources, " "))
	switch {
	case w.Why != "":
		return []string{behind + ", and how many of them render a wall is UNKNOWN: " + w.Why +
			" — `make install` is the remedy either way"}
	case len(w.Commits) == 0:
		// Not an all-clear, and worded so it cannot be read as one: the
		// pathspec list is a floor (see WallRenderSources), and the lag is
		// still a lag.
		return []string{behind + ", none of them touching a path posse renders a wall from — the walls are current as far as those paths go; `make install` closes the rest (" + cmd + ")"}
	}
	out := []string{behind + fmt.Sprintf(", %d of them rendering a wall — until a descendant of %s is installed, every launch on this box renders the OLD seatbelt, gate shims, gate shell, cage mounts and hooks, and says \"rendered from the PID at launch\" while doing it (ranger-base-mmhhc: 12 caged launches rendered a superseded credential deny). `make install`:",
		len(w.Commits), l.Base)}
	for i, c := range w.Commits {
		if i == wallCommitsShown {
			out = append(out, fmt.Sprintf("    … and %d more", len(w.Commits)-wallCommitsShown))
			break
		}
		out = append(out, "    "+c)
	}
	return append(out, "    "+cmd)
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

// ReportWallRenderer prints the reading and reports whether it found
// something to act on — a lag, or an answer it could not read. The bool is
// for a caller that wants to say more, never a refusal: `posse promote` is
// the operator's command and this is a reading (see the file header).
func (a *App) ReportWallRenderer(out io.Writer, where string, extra ...string) bool {
	w := a.WallRenderer(extra...)
	for _, ln := range w.Lines(where) {
		fmt.Fprintln(out, ln)
	}
	return !w.Lag.Known() || w.Lag.Behind > 0
}
