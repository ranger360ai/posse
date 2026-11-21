package posse

// THE LAUNCHER THAT JUDGED IS NAMED, AND A WINDOW IS NOT ATTRIBUTED TO A WALL
// THAT MOVED (ADR 0088 D7/D8, from ranger-base-7or9p). landedcheck.go judges
// the landed tree and landedfile.go turns the verdict into a bead; this file
// is the one input to both that does NOT come from the tree under judgement.
//
// WHAT WENT WRONG. ADR 0088 D1 is careful about where the judge's bytes come
// from — the last tree recorded green, never the tree being judged — and D5's
// bead is written as though that were the whole of it: "The newest landing is
// simply the one whose tree first held them all" (landedWindowSection). It is
// not the whole of it. The judge's own wall is rendered by the RUNNING
// launcher: the socket-wall step's binary is `wallWatchBin()`'s answer
// (landedcheck.go's `THE BINARY IS THE LAUNCHER'S`), the seatbelt profile is
// rendered by this process's code, and ADR 0097's venue is rendered by the
// same function. So a landing that changes the wall does not change what
// judges until somebody reinstalls the launcher — and on the install, one
// tree's verdict flips with no landing at all.
//
// MEASURED 2026-11-20 (ranger-base-ufd7z, re-read under ranger-base-q8j8x;
// this box, 16 cores): the TREE held constant at main's `edfe7a79` and only
// the step's binary changed. A posse built at `b29c95b5` judged it exit 2 in
// 139s, with four bdargvgate pins failing on `branch -M` with `Invalid
// cross-device link`; the installed `0.5.3+edfe7a79` judged the same tree
// exit 0 in 132s. Production said it from its side, commenting `still red at
// 853242ca2dc2` over a tree that already held the fix. What it cost: one P1
// filed and dispatched against a window of eleven innocent landings and the
// persona of the newest one, and a seat's session to establish that no tree
// in the window could clear it.
//
// SO THERE ARE TWO RULES HERE, and they are the cheap one and the sharp one.
//
// D7, THE CHEAP ONE: every verdict line, and the bead D5 files, NAMES the
// launcher that ran the check — version, path, mtime. Before this, nothing in
// either said which binary judged, so a reader could not tell a tree red from
// a judge red; with it, the two readings above are two different sentences in
// the log. NOT D6.3's health beads, which is a stated residual and not an
// omission: their subject is two or three RUNS, and a launcher restart can
// split those between two binaries (docs/notes.d/ranger-base-4jroi.md), so
// this run's reading would be a claim about runs it did not take. Version AND mtime because they fail differently: the version is
// the RUNNING code, which renders the cage and the profile, and the mtime is
// the FILE at the step's path, which is what the socket-wall step execs — a
// reinstall under a running launcher moves the second and not the first.
//
// D8, THE SHARP ONE: a green→red window is not attributed to a landing when
// the launcher is not the one that judged the green tree. The record is this
// rule's own `git config` line (landedRule.LauncherKey), written beside a
// GREEN verdict and only there, so what it holds is "the launcher that judged
// the tree currently on record green" — which is exactly the far edge of
// every window D5 attributes. Three answers and not a bool, for
// landedStretchState's reason: `same` attributes as before, `moved` files
// UNASSIGNED and says the wall moved, and `unknown` — no token on record, or
// a launcher that cannot name itself — attributes as before with one sentence
// saying the comparison could not be made. Silence would be the one answer
// that cannot be read.
//
// WHY NOT A THIRD FIELD ON D4'S RECORD. `landedRecorded` reads a record in
// any other shape as NO record, which is deliberate (a half-parsed verdict
// must never read green) — and a record read as absent makes every red a
// green→red transition again, which is one P1 per landing, the
// ranger-base-4v4r6 incident D5 exists to end. A separate key is also the
// house pattern for exactly this: D6.3's counter is its own key for the same
// reason (landedrule.go's `Key, QuietKey`).
//
// WHY A TOKEN AND NOT A PARSED STRUCT. The record is written by this file and
// compared by this file, as one string, and nothing ever reads a field out of
// it. An operator reads it — `git config --get posse.landedCheckBy` — so it
// is spelled for a human; a hand-edit reads as `moved`, which withholds an
// attribution and files nothing extra.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// landedLauncher is the identity of the launcher a run judges with: the
// RUNNING code's build, and the file the cage's own step execs.
//
// `Why` carries the refusal when the file half could not be read, because
// that case has to be SAID: a launcher that cannot stat its own binary names
// no token, and a verdict whose launcher cannot be named is one no other
// verdict can be compared with. A reader who is told that reaches for the box;
// a reader shown a bead with a silent gap in it reaches for the window.
type landedLauncher struct {
	Version string    // VersionString(): the code that rendered the cage
	Path    string    // wallWatchBin(): the file the socket-wall step execs
	Mod     time.Time // that file's mtime, which a reinstall moves
	Size    int64
	Why     string // why the file half (Path, Mod, Size) is missing
}

// landedLauncherNow reads it. A plain function and not a var, because the
// seam the suite needs is on the CHECKER (landedChecker.launcherFn, beside
// the cage renderer's): these tests run in parallel, and a package-level var
// one of them swapped would be the launcher every other one read.
func landedLauncherNow() landedLauncher {
	l := landedLauncher{Version: VersionString()}
	self, err := wallWatchBin()
	if err != nil {
		l.Why = oneLine(err.Error())
		return l
	}
	l.Path = self
	fi, serr := os.Stat(self)
	if serr != nil {
		l.Why = oneLine(serr.Error())
		return l
	}
	l.Mod, l.Size = fi.ModTime(), fi.Size()
	return l
}

// String is the launcher as a verdict line and a bead name it. The path is
// abbreviated for WarnOpsContent's reason — this string reaches a bead other
// seats read — and the mtime is UTC so two boxes' lines compare.
func (l landedLauncher) String() string {
	if l.Path == "" {
		return fmt.Sprintf("posse %s, whose own binary could not be read (%s)", l.Version, orElse(l.Why, "it does not resolve"))
	}
	if l.Mod.IsZero() {
		return fmt.Sprintf("posse %s at %s, which could not be stat'd (%s)", l.Version, AbbrevHome(l.Path), orElse(l.Why, "it does not resolve"))
	}
	return fmt.Sprintf("posse %s at %s (mtime %s, %d bytes)", l.Version, AbbrevHome(l.Path), l.Mod.UTC().Format(time.RFC3339), l.Size)
}

// token is the record's value, and "" when this launcher cannot name itself —
// which is read as `unknown` and never as a difference (landedLauncherDrift).
//
// EVERY FIELD THAT CAN MOVE WITHOUT THE OTHERS IS IN IT. The version is the
// running code; the mtime is the file a reinstall replaces under it; the size
// is what an `install -p` that preserved the mtime still moves. The path is
// last because it is the one field that can carry a space, and nothing parses
// this string — it is compared whole.
func (l landedLauncher) token() string {
	if l.Path == "" || l.Mod.IsZero() {
		return ""
	}
	return strings.Join([]string{
		l.Version,
		l.Mod.UTC().Format(time.RFC3339),
		strconv.FormatInt(l.Size, 10),
		AbbrevHome(l.Path),
	}, " ")
}

// landedLauncherDrift is what the record can say about the launcher that
// judged the tree on record green. Three answers and not a bool, because
// exactly one of them may withhold an attribution: see this file's header.
type landedLauncherDrift int

const (
	// landedLauncherUnknown: one side names no launcher — nothing on record
	// (a green judged before D8, or a write that lost the lock), or a
	// launcher that could not stat its own binary. Never a reason to withhold.
	landedLauncherUnknown landedLauncherDrift = iota
	// landedLauncherSame: the record names this launcher. D5's attribution
	// means what it says.
	landedLauncherSame
	// landedLauncherMoved: the record names another one. The wall moved under
	// the window, so no landing in it is answerable for the red.
	landedLauncherMoved
)

// landedLauncherDriftOf compares a reading with a record.
func landedLauncherDriftOf(now landedLauncher, was string) landedLauncherDrift {
	t := now.token()
	switch {
	case t == "" || strings.TrimSpace(was) == "":
		return landedLauncherUnknown
	case t == strings.TrimSpace(was):
		return landedLauncherSame
	}
	return landedLauncherMoved
}

// recordLandedLauncher writes the token for a tree just recorded GREEN.
//
// GREEN ONLY, which is the whole of why nothing else here needs a parameter:
// the question D8 asks is always "who judged the tree on record green", and
// leaving the record alone across a red stretch is what keeps that answer
// readable from the red run itself. Best effort, like the verdict's own
// write, and the caller SAYS when it failed — a stale token reads as `moved`
// and withholds an attribution, which is the safe direction and not a silent
// one.
func recordLandedLauncher(r landedRule, repo string, l landedLauncher) error {
	if l.token() == "" {
		return nil // nothing to record; the next red reads `unknown` and says so
	}
	_, err := git(repo, "config", r.LauncherKey, l.token())
	return err
}

// landedLauncherRecorded reads it back: the token for the tree on record
// green, or "" when there is none. Unlike D4's record there is nothing to
// parse, so any value is the token it is and a hand-edited one reads as
// another launcher.
func landedLauncherRecorded(r landedRule, repo string) string {
	out, err := git(repo, "config", "--get", r.LauncherKey)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// launcher is this run's reading, taken ONCE per run: once() clears the memo
// at the top of each one, so the verdict line, the record and the filed bead
// cannot name three different launchers for one judgement. A fresh read when
// the memo is empty, which is what makes a zero-value checker — and a test
// that renders a description without running anything — read the real thing.
//
// No lock: the memo is written and read on the one run goroutine (loop), and
// a test that calls a description renderer directly has a checker of its own.
func (c *landedChecker) launcher() landedLauncher {
	if c.launcherAt == nil {
		read := landedLauncherNow
		if c.launcherFn != nil {
			read = c.launcherFn
		}
		l := read()
		c.launcherAt = &l
	}
	return *c.launcherAt
}

// launcherNote is D7 at a verdict line: every one of them carries it, the ⚠
// that says there is no verdict included, because "which binary judged" is a
// question about a cage that would not render too.
func (c *landedChecker) launcherNote() string {
	return " — launcher: " + c.launcher().String()
}

// landedJudgedBy is what one filed bead says about the launcher: the reading
// this verdict was reached with, the token on record for the tree last
// recorded green, and what the two of them make.
//
// A VALUE AND NOT THREE PARAMETERS, because the three are read together in
// two places (the first paragraph of the bead and its attribution paragraph)
// and a drift computed twice is a drift that can disagree with itself.
type landedJudgedBy struct {
	Now   landedLauncher
	Was   string
	Drift landedLauncherDrift
}

// judgedBy reads it, and the order is the one thing to keep: it must be asked
// BEFORE anything writes this rule's launcher record. On the only path that
// asks (fileRed, a red verdict) nothing has — D8's token is written beside a
// GREEN verdict and only there.
func (c *landedChecker) judgedBy(r landedRule, repo string) landedJudgedBy {
	now, was := c.launcher(), landedLauncherRecorded(r, repo)
	return landedJudgedBy{Now: now, Was: was, Drift: landedLauncherDriftOf(now, was)}
}
