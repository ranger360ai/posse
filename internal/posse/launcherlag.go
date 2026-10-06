package posse

// How far behind its own repo the RUNNING launcher is (ranger-base-z3hx6).
//
// THE INCIDENT. On 2026-09-04 ~/.local/bin/posse was built at 23:54 from
// c592683 and stayed that way until 07:16 — eight hours, 34 commits behind
// main by the end. Two of those 34 each independently stop a merge-back
// block from being re-filed against one branch (67effd0 ranger-base-emgdb,
// c3ab918 ranger-base-j8qmj); both landed inside the window, and the
// 23:54 binary had neither. So the block was filed a FOURTH time at 07:08,
// 77 minutes after the second fix landed, and a seat spent a whole session
// re-deriving a do-not-land verdict two commits on main already held.
//
// WHY NOTHING CAUGHT IT. A stale launcher does not fail. It dispatches,
// merges back and files beads exactly as designed — it just does so with
// the defect its own repo fixed hours ago, and the beads it files are
// indistinguishable from real ones. `git log` shows nothing: main is
// perfect. The tell is only ever the version stamp, and until this file
// nothing read it.
//
// WHY THIS IS NOT IN THE WATCH PREAMBLE, where the binary's identity is
// already printed once at loop start (possebinary.go). A start-of-loop
// reading is blind to this by construction, measured both times it
// happened: c592683 was committed 23:52:01 and built at 23:54, and
// `rev-list --count --before=23:54 c592683..main` is 0; 9920e75 was
// installed at 07:16:28 and the same count before that instant is 0. A
// binary is BUILT from the tip and the loop is started right after, so the
// one moment a start-of-loop line speaks is the one moment the number is
// always zero. The gap is created entirely afterwards — the next commit
// after c592683 landed at 23:55:47, 1m47s after the build. So the reading
// lives in the PASS, which is the only clock that keeps ticking while the
// binary does not.
//
// A READING, NEVER A CONTROL, on possebinary.go's rule: it prints, it warns
// and it decides nothing. Installing over a binary that is dispatching a
// live fleet is a live change and stays the operator's (guardrail 3); this
// file is the signal, not the remedy.
//
// AND, SINCE ranger-base-y13h7, A GOVERNANCE CONDITION — which is not a
// control, and the two hold at once. A condition gates nothing: it stops no
// pass, declines no launch and installs nothing. It puts the fact on the one
// surface whose whole question is "does anything need a human", and the
// absence of it was a four-day silence of a particular kind: `posse status`
// printed the sentence below — ending "only installing closes it" — and
// `nothing needs a human` two lines under it, in the same view, on every
// pass, while ~/.local/bin/posse went from current to 101 commits behind.
// The remedy is unchanged and still the operator's. What changed is that the
// surface which answers "is anybody needed" now reads this one. The row is
// G11, LANE, and it is the last section of this file.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// posseModule is the module path a candidate checkout must declare before
// its commits are counted as this binary's. The stamp alone nearly settles
// it — a 7-hex prefix naming a commit in an unrelated repo is a one-in-tens-
// of-thousands accident — but "nearly" here renders a number into an
// operator's log, and a wrong number is worse than none. Spelled out rather
// than read from the running binary's build info because a TEST binary's
// build info names the package under test, not the module (see
// cagestale.go's header on what go's own stamps do and do not carry).
const posseModule = "github.com/ranger360ai/posse"

// LauncherLag is one answer to "how many landed commits is the binary that
// is running this fleet missing?".
//
// Behind is meaningful only when Why is empty. Every field that is a path or
// a rev is kept as measured, so a line can name what it counted and a reader
// can re-run it by hand — the whole value of this reading is that it is
// reproducible from the log it printed into.
type LauncherLag struct {
	Version string // VersionString() of the process taking the reading
	Rev     string // the commit that version claims, as a rev git resolved
	Dirty   bool   // the tree that built it had uncommitted edits
	Repo    string // the main checkout Rev was found in
	Base    string // the branch Rev was counted against
	Behind  int    // commits on Base that Rev does not have
	Why     string // why there is no number; "" when Behind is one
}

// Known is whether Behind means anything.
func (l LauncherLag) Known() bool { return l.Why == "" }

// stampRev is the rev a version string claims, and whether the tree that
// built it was dirty.
//
// Three shapes, and VersionString() produces all three: "0.4.0+9920e75" from
// the Makefile's ldflags stamp or go's own build info, the same with a
// "-dirty" suffix, and a bare "0.4.0" from an install of the tag itself —
// which a brew keg is, and a stale keg ahead of ~/.local/bin on PATH is how
// ranger-base-39jnl happened, so the tag shape is counted too rather than
// abstained on. "+dev" is the one shape that names no commit at all.
func stampRev(version string) (rev string, dirty bool, ok bool) {
	ident, ok := cutLast(version, "+")
	if !ok {
		// Installed from the tag this source is. versionString() collapses
		// "0.4.0+v0.4.0" to the bare version, so the tag is the only thing
		// left to count from.
		if version == "" {
			return "", false, false
		}
		return "v" + version, false, true
	}
	ident, dirty = strings.CutSuffix(ident, "-dirty")
	if ident == "" || ident == "dev" {
		return "", dirty, false
	}
	return ident, dirty, true
}

// cutLast splits on the LAST sep, because Version itself may carry one and
// the stamp is always the tail.
func cutLast(s, sep string) (string, bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return "", false
	}
	return s[i+len(sep):], true
}

// FindLauncher resolves which checkout the running binary came out of and
// takes the first count. Give it every directory this instance knows about;
// the STAMP picks the repo, not the caller, which is the half of the bead's
// "measured against the installed binary's own build stamp rather than
// anything a worktree can fake" that a config key would give away.
//
// Each candidate is resolved to its MAIN checkout first (MainCheckout): a
// linked session worktree shares its refs with the checkout it hangs off, so
// counting there would answer the same question with a `main` that a persona
// could move, and the count belongs to the repo, not to whoever's tree the
// launcher happened to be looking at.
func FindLauncher(version string, candidates []string) LauncherLag {
	l := LauncherLag{Version: version}
	rev, dirty, ok := stampRev(version)
	l.Dirty = dirty
	if !ok {
		l.Why = "the running binary names no commit (" + version + ") — a build with neither the Makefile's stamp nor go's own build info"
		return l
	}
	l.Rev = rev
	seen := map[string]bool{}
	var looked []string
	for _, c := range candidates {
		if c == "" {
			continue
		}
		dir, isRepo := MainCheckout(ExpandTilde(c))
		if !isRepo || seen[dir] {
			continue
		}
		seen[dir] = true
		looked = append(looked, AbbrevHome(dir))
		if !isPosseCheckout(dir) {
			continue
		}
		// The stamp has to be IN it. A posse checkout that has never
		// fetched the commit this binary was built from cannot say how far
		// behind it is, and a count against a rev git resolved to something
		// else is the wrong-number case this whole function is careful about.
		//
		// WHAT THIS DOES NOT SETTLE, said here so nobody re-derives it: two
		// real CLONES of posse both hold the stamp, and the first candidate
		// wins even if its `main` is the staler of the two. The stamp
		// separates unrelated histories, not two copies of one — for that
		// the order is the operator's, and it is the order of `beads:`.
		if _, err := git(dir, "cat-file", "-e", rev+"^{commit}"); err != nil {
			continue
		}
		l.Repo = dir
		break
	}
	if l.Repo == "" {
		where := "no directory this instance knows about"
		if len(looked) > 0 {
			where = strings.Join(looked, ", ")
		}
		l.Why = "no checkout of " + posseModule + " here holds " + rev + " (looked in " + where + ")"
		return l
	}
	base, err := git(l.Repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || base == "" {
		l.Why = "cannot read what branch " + AbbrevHome(l.Repo) + " is on (" + errText(err) + ")"
		return l
	}
	l.Base = base
	return l.Count()
}

// Count re-reads the number in the repo FindLauncher already resolved, and
// is the whole per-pass cost: one `git rev-list --count`, no candidate scan,
// no second look at go.mod. Cheap on purpose — the resolution cannot change
// under a running loop (a loop keeps the binary it started with, and the
// checkout it was built from does not move), while the NUMBER changes every
// time somebody lands a commit, which on this box is about eleven times an
// hour.
func (l LauncherLag) Count() LauncherLag {
	if l.Repo == "" || l.Rev == "" || l.Base == "" {
		return l
	}
	out, err := git(l.Repo, "rev-list", "--count", l.Rev+".."+l.Base)
	if err != nil {
		l.Behind, l.Why = 0, "cannot count "+l.Rev+".."+l.Base+" in "+AbbrevHome(l.Repo)+" ("+errText(err)+")"
		return l
	}
	n, cerr := strconv.Atoi(strings.TrimSpace(out))
	if cerr != nil {
		l.Behind, l.Why = 0, "git rev-list --count "+l.Rev+".."+l.Base+" answered "+strconv.Quote(out)
		return l
	}
	l.Behind, l.Why = n, ""
	return l
}

// isPosseCheckout is the module declaration, read off the working tree
// rather than out of git: one file read instead of a fork, and the working
// tree is what a build in that directory would have compiled.
func isPosseCheckout(dir string) bool {
	body, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	for _, ln := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(ln) == "module "+posseModule {
			return true
		}
	}
	return false
}

func errText(err error) string {
	if err == nil {
		return "no error reported"
	}
	return err.Error()
}

// Line is what `posse status` prints: an answer in every case, including the
// case where there is no number. An operator who typed a command is owed a
// sentence, and an abstention that renders as silence reads as an all-clear.
func (l LauncherLag) Line() string {
	switch {
	case !l.Known():
		return fmt.Sprintf("launcher · %s · not counted: %s", l.Version, l.Why)
	case l.Behind == 0:
		return fmt.Sprintf("launcher · %s is the tip of %s in %s", l.Version, l.Base, AbbrevHome(l.Repo))
	default:
		return "launcher · " + l.behindSentence()
	}
}

// BehindLine is what a PASS prints, and it is empty in every case but the
// one the bead is about. A pass reports conditions; "the launcher is fine"
// is not one, and a loop that says it every pass for ten hours is how a
// visible line becomes an invisible one (the rule watch.go's preamble
// already keeps for the hook wall and the stale-plan typo).
func (l LauncherLag) BehindLine() string {
	if !l.Known() || l.Behind == 0 {
		return ""
	}
	return "launcher behind · " + l.behindSentence()
}

// behindSentence names the number, both ends of the comparison, and the
// command that lists what is missing — because the next thing anyone asks is
// "which fixes?", and the answer must not require re-deriving the rev.
func (l LauncherLag) behindSentence() string {
	// The dirty clause says what the suffix in the version string does NOT:
	// the stamp names the last COMMIT of a tree that also had uncommitted
	// edits, so whatever those edits were is missing from the base as well
	// and the number is a floor rather than the whole distance.
	dirty := ""
	if l.Dirty {
		dirty = ", and its tree had uncommitted edits that are in no commit either, so this is a floor"
	}
	return fmt.Sprintf(
		"%s is %d commit(s) behind %s in %s%s — every one of them is a fix this fleet is not getting, and it keeps running the defects they fixed; git -C %s log --oneline %s..%s names them, and only installing closes it",
		l.Version, l.Behind, l.Base, AbbrevHome(l.Repo), dirty, AbbrevHome(l.Repo), l.Rev, l.Base)
}

// Launcher is the instance's own reading: the running binary, counted
// against every repo this instance is configured for, plus the process's
// working directory — the second so `posse status` typed inside a posse
// checkout answers even on a box whose `beads:` never names one.
func (a *App) Launcher() LauncherLag {
	return FindLauncher(VersionString(), a.launcherCandidates(true))
}

// LauncherOverFleet is the same reading taken over the instance's CONFIGURED
// checkouts alone, and it is what G11 is computed from (govern.go lag()).
//
// The difference from Launcher is one candidate and it is the difference
// between a line and a condition. A line is owed to whoever typed the
// command, so it answers about the checkout they are standing in; a condition
// is owed to the fleet, and the fleet is `beads:`. The second half of the
// reason is in govern.go's lag(), where the row is raised.
func (a *App) LauncherOverFleet() LauncherLag {
	return FindLauncher(VersionString(), a.launcherCandidates(false))
}

// launcherCandidates is the list a reading is taken over, with withCwd the
// only difference between the two readings above.
//
// Its own function because that difference is the whole of what distinguishes
// them and is otherwise unpinnable: both readings go through VersionString(),
// a test binary carries no vcs stamp at all (cagestale.go's header), so every
// end-to-end reading in this package abstains before the candidate scan is
// even reached — which is why dispatch.Lag and GovInputs.Lag are seams. The
// candidate list is the part a pin can hold.
func (a *App) launcherCandidates(withCwd bool) []string {
	c := a.BeadsDirs()
	if !withCwd {
		return c
	}
	if cwd, err := os.Getwd(); err == nil {
		c = append(c, cwd)
	}
	return c
}

// lagDrumbeat is the cadence rule: say the number when it has DOUBLED since
// it was last said. Nothing else about the reading was in doubt; how often
// to repeat it was, and the two obvious answers are both wrong here.
//
// Every pass is ~90 lines over the eight-hour window this came from — a
// number an operator stops seeing by the third hour. ONCE is the
// start-of-loop banner the file header rejects, and it is worse: it is
// always zero. Doubling prints at 1, 2, 4, 8, 16, 32 — six lines over those
// eight hours, and the last of them is the one that reads as an alarm rather
// than as weather. It is also this loop's own idiom: a quiet pass doubles
// its sleep (watch.go), for the same reason — a signal that recurs must get
// rarer as the thing it is about stays the same.
//
// A loop that STARTS behind says so on its first pass whatever the number,
// because the first say() is against a floor of 1.
type lagDrumbeat struct{ nextAt int }

func (d *lagDrumbeat) say(behind int) bool {
	at := d.nextAt
	if at < 1 {
		at = 1
	}
	if behind < at {
		return false
	}
	d.nextAt = behind * 2
	return true
}

// ─── G11 · past a measured depth the reading is a condition ──────────────────

// DefaultLauncherBehindMax is the depth past which a lagging launcher stops
// being a reading and becomes a governance condition (ranger-base-y13h7).
//
// 0 IS WRONG, and that is the one end of this that was already measured: a
// binary is built from the tip and the next commit lands minutes later — this
// file's header has it twice, 1m47s and 0 commits behind at the instant of
// the build — so a threshold of zero fires on every install within the hour
// and says nothing about whether anyone is minding it.
//
// WHAT WAS MEASURED FOR THE OTHER END. MEASURED 2026-10-05, darwin 25.4.0, in
// ~/src/posse. Every install of ~/.local/bin/posse this box can still account
// for: the loop-start preambles in `$StateDir/dispatch-watch.log{,.1}`, which
// print the running binary at every loop start (possebinary.go), plus the
// `posse binary ·` lines in `~/.claude/projects/*/*.jsonl`. 14 installs,
// 2026-09-03..10-03, so 13 CLOSED episodes, each one's depth being
// `git rev-list --count <stamp_i>..<stamp_i+1>` — the lag the running binary
// had reached when the next install replaced it:
//
//	9 · 20 · 26 · 34 · 42 · 46 · 48 · 51 · 52 · 79 · 105 · 165 · 180
//	median 48. The open episode when this bead was filed was 101.
//	reach >= 8: 13/13 · >= 16: 12/13 · >= 32: 10/13 · >= 64: 4/13
//	EXCLUDED, as fixtures and not installs: the three readings of
//	0.5.0+02dd122 at 40, 42 and 40 behind on three separate days — a
//	non-monotonic depth for one stamp, and the string is quoted verbatim in
//	docs/notes.d/ranger-base-vso72.md.
//	The install set is a FLOOR: an install whose stamp reached neither a
//	preamble nor a transcript is invisible, which makes every distance above
//	an upper bound and cuts toward a SHALLOWER threshold, not a deeper one.
//
// So the box's own cadence is not the bound. Its median episode (48) is
// already past the depth at which this lag has twice cost a session, which
// means no threshold can be derived from "deeper than normal" — normal is the
// defect. The number has to come off the COST, and both costs are traceable
// commit by commit:
//
//	ranger-base-z3hx6/tvorm · c592683, installed 2026-09-03 23:54. The first
//	fix the fleet was then missing (67effd0, ranger-base-emgdb) landed at
//	depth 21; the second (c3ab918, ranger-base-j8qmj) at depth 31; the cost —
//	a merge-back block filed a FOURTH time, one dispatched session re-deriving
//	a do-not-land verdict two commits on main already held — at depth 34.
//	ranger-base-6pnab/xxgp3 · 193c8782, installed 2026-10-03 15:28. The first
//	of the four ci-watch fixes it was missing (540cc22c, ranger-base-m46kr)
//	landed at depth 90; the false ci-red P1 was filed at depth 93; the reading
//	in the bead said 101.
//
// 16 IS THE LARGEST DRUMBEAT STEP STRICTLY BELOW 21 — the shallowest depth at
// which a fix the fleet needed was already on main. That is the property
// being bought: at 16 the row is up BEFORE the fleet is missing anything, so
// an operator who acts on it is installing a current binary rather than
// closing a hole. The steps are the drumbeat's own (lagDrumbeat), because the
// key below re-prompts on them.
//
// REJECTED, with what each costs:
//
//	8 · 13/13 episodes, and it buys nothing — no incident's first missing fix
//	  was shallower than 21 — while spending the one episode (9) where the
//	  operator installed promptly, which is the behaviour the row wants.
//	32 · 10/13. In the shallower incident the row would first rise after BOTH
//	  missing fixes were already on main (21 and 31), two commits before the
//	  cost landed: a report of an existing hole, not a warning.
//	64 · 4/13, and SILENT through the whole of that incident, which reached 34.
//	  Excluded by measurement rather than by taste.
//
// AND IT IS A FLOOR, NOT A PROOF. 13 episodes, 2 with a cost traced to them;
// nothing here establishes that the other 11 cost nothing, because a stale
// launcher does not fail (this file's header) and the absence of a filed bead
// is not the absence of a cost. Erring LOW is the cheap direction: a row that
// rises early costs one install the shop wanted anyway, and it clears itself
// the moment the install lands; a row that rises late costs a dispatched
// session, twice measured. `launcher_behind_max:` is the dial, and 0 means
// "any lag at all".
const DefaultLauncherBehindMax = 16

// LauncherBehindMax is the configured depth (`launcher_behind_max:`). Same
// typo handling as every other threshold key: a value that is not a count is
// named on stderr and the default stands, because a threshold nobody can see
// is worse than a wrong one. Zero is meaningful and kept — it means every
// non-zero lag is a condition, which is the shape an instance that installs
// from the tip every time would want.
func (a *App) LauncherBehindMax(errw io.Writer) int {
	raw := strings.TrimSpace(a.CfgGet("launcher_behind_max", ""))
	if raw == "" {
		return DefaultLauncherBehindMax
	}
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return n
	}
	fmt.Fprintf(errw, "governance: config launcher_behind_max: %q is not a commit count — using %d\n",
		raw, DefaultLauncherBehindMax)
	return DefaultLauncherBehindMax
}

// GovRows is G11: the governance rendering of this same reading, past max.
//
// It takes the threshold rather than reading it, on VerifyBoxReading.GovRows's
// rule one file over: a reading carries what it was judged against, so two
// renderings of one reading cannot disagree about where the line was.
//
// An ABSTENTION raises nothing. "Cannot be counted" is the ordinary shape of
// a checkout build — a `go run`, or a plain `go build` from a linked worktree,
// names no commit at all — and it is already said out loud and unconditionally
// by the two surfaces that own this reading (`posse status` prints Line() in
// every case; the watch preamble says it once). It is not an unreadable store
// either, and ShopCheck says why where it declines to report one.
func (l LauncherLag) GovRows(max int) []GovCondition {
	if !l.Known() || l.Behind <= 0 || l.Behind < max {
		return nil
	}
	return []GovCondition{{
		ID:    "G11",
		Class: GovLane,
		Key:   fmt.Sprintf("launcher-behind:%d", lagBucket(l.Behind, max)),
		// "the launcher" first, because a GOVERNANCE row is read in a list of
		// other rows and a detail that opens with a version string makes the
		// reader work out what it is about. Line() has the same word in the
		// same place, so one grep finds the row and the reading.
		Detail: "the launcher " + l.behindSentence() + fmt.Sprintf(
			" · past launcher_behind_max: %d, which is what makes this a condition and not just a reading — the install itself is still the operator's, because installing over a binary that is dispatching a live fleet is a live change (guardrail 3)",
			max),
	}}
}

// lagBucket is the drumbeat step a depth falls in: the threshold, doubled
// while another doubling would still fit under the number. It is what the Key
// carries, and the Key is what the pulse fingerprints — so a lag that keeps
// deepening re-prompts at 16, 32, 64, 128 and nowhere in between, which is
// log2 prompts over an episode rather than one per tick.
//
// The same rule lagDrumbeat uses for the watch log, and deliberately the same
// numbers: the pass line and the governance row escalate together, so an
// operator reading the log back sees one cadence and not two.
func lagBucket(behind, max int) int {
	b := max
	if b < 1 {
		b = 1
	}
	for b*2 <= behind {
		b *= 2
	}
	return b
}
