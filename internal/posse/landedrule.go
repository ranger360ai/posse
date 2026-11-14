package posse

// ONE RATCHET AND ONE WATCHDOG, READ BY TWO CHECKS (ADR 0097 D5, from
// ranger-base-wplhv). landedfile.go is ADR 0088 D5's ratchet — one attributed
// bead per green→red stretch — and landedhealth.go is D6.3's watchdog on the
// detector itself. ADR 0097 D5 gives the wall check the SAME two rules over
// its own two records, and says how: "Parameterise the existing code rather
// than copying it. Dedupe markers must differ from the judge's."
//
// SO THIS FILE IS THE PARAMETER, and it is a value and not an interface. What
// differs between the two checks is a dozen LITERALS — two git config keys,
// two marker prefixes, a script path, a bead title's subject, and the ADR
// lines a reader of the bead is sent to. Nothing about the tables differs: the
// transition rows, the three answers a store can give, the gating on a record
// that could not be written, the dedupe by a whole line in a description. A
// second implementation of any of that is the thing this file exists instead
// of — the two would drift, and the one that drifted would be the one nobody
// was reading when it mattered.
//
// WHY THE MARKERS MUST DIFFER, which is the one line of D5 that is a
// requirement and not a convention: the dedupe is "the newest OPEN bead in
// this lane whose description carries this line" (openMarkedBead). The two
// ratchets file into the same lane (`-l code`), so a marker either could match
// would make a red wall comment on a red main's bead — one stretch's bead
// carrying another stretch's verdicts, and neither stretch filing its own. The
// prefixes are therefore distinct STRINGS and not a repo plus a base, and
// TestQATheTwoLandedRulesShareNoRecordOrMarker holds every pair of them apart.
//
// THE PROSE FIELDS CARRY FORMAT VERBS, and the comment on each one names them
// in order. That is a sharper edge than the rest of this struct — `go vet`
// cannot check a format string it reads out of a variable — so each is held
// by the same QA pin, which renders both rules' beads and asserts no `%!`
// verb error survives in either.

import (
	"path/filepath"
	"time"
)

// landedRule is everything one of these two checks spells differently. Every
// field is a literal a human READS: in a git config line, in a bead's title,
// or in the description a fixer opens first.
type landedRule struct {
	// Name is the check in prose, lower case, as the health bead's title and
	// the silent description open with it: "landed check", "wall check".
	Name string
	// Agent is what the watchdog's prose CALLS the thing that answers or does
	// not: "judge" for ADR 0088's, "wall check" for ADR 0097's. A second noun
	// beside Name and not a reuse of it, because the judge is called the
	// judge everywhere a human has ever read about it, and "the landed check
	// did not answer" is a sentence about a different subject from "the judge
	// did not answer" — the check is the rule, the judge is the child.
	Agent string
	// Script is the body ADR 0088 D1's contract names, and the path a tree
	// opts in by shipping.
	Script string
	// Extract is the basename the copy that RUNS is written as, inside the
	// run dir and beside the worktree — never inside it (extract's header).
	// Distinct per rule because one run extracts both.
	Extract string
	// Key and QuietKey are the two records: `<tree> <green|red>` and
	// `<tree> <n>`. Separate keys and not one with a third field, which is
	// the alternative ADR 0088 D6.3 rejects by name.
	Key, QuietKey string
	// FileMarkerPrefix and HealthMarkerPrefix open the one line of a filed
	// bead's description that says which rule filed it. See the header: these
	// are the dedupe of record and they may not collide.
	FileMarkerPrefix, HealthMarkerPrefix string
	// RedSubject is what the ratchet's bead is ABOUT, in its title and in its
	// first sentence. EMPTY means the branch that landed, which is ADR 0088
	// D5's "<base> red at <tree>" — the landed check's subject is main
	// itself. The wall check's subject is the pins, whatever branch they were
	// landed on.
	RedSubject string
	// RedWhen says when the check ran, closing the ratchet bead's first
	// sentence: the clause after "judged by `<script>` ".
	RedWhen string
	// RedAdvice is the ratchet bead's "What to do" paragraph. VERBS, in
	// order: the script, the subject. It ends with a space, because the
	// sentence about who closes the bead follows it.
	RedAdvice string
	// CeilingTitle is the price's name in the over-ceiling bead's title:
	// "<name> over <this> at <repo>".
	CeilingTitle string
	// SilentAdvice is the watchdog bead's "What to do" paragraph. VERBS, in
	// order: the script, the repo, the hang bound.
	SilentAdvice string
	// CeilingAdvice is the over-ceiling bead's "What to do" paragraph.
	// VERBS, in order: the script.
	CeilingAdvice string
	// SilentCost is what the silence COSTS, as one clause: it closes "While
	// the <agent> is not answering, …". The judge's silence is a landing
	// nobody judged; the wall check's is a landing whose pins nobody
	// measured. Both end with the ratchet that therefore files nothing,
	// because that is why the watchdog bead exists.
	SilentCost string
	// The four citations a reader of one of these beads is sent to. Named
	// rather than spelled at each site, because a bead carries two or three
	// of them and the ADRs they name are different documents for the two
	// rules.
	// CeilingADR is the citation on the one line that is about an answered
	// run OVER the price — the comment on an open health bead — where the
	// judge's rule cites two D-numbers at once and the wall's cites one.
	RunADR, FileADR, HealthADR, PriceADR, BoundsADR, CeilingADR string
}

// fileMarker is the ratchet's dedupe line for one repo's base. It carries
// both, because a beads store can serve more than one repo here — two of this
// instance's three stores are reached through a `.beads/redirect` — so "the
// open bead" has to be the open bead for THIS branch of THIS repo.
func (r landedRule) fileMarker(repo, base string) string {
	return r.FileMarkerPrefix + AbbrevHome(repo) + " " + base + " (" + r.FileADR + ")"
}

// healthMarker is the watchdog's, per REPO and not per base: the counter is a
// repo-level git config record and the bead says "at <repo>", because a check
// that cannot answer cannot answer about any branch.
func (r landedRule) healthMarker(repo string) string {
	return r.HealthMarkerPrefix + AbbrevHome(repo) + " (" + r.HealthADR + ")"
}

// subject is RedSubject with its empty case resolved: the branch that landed.
func (r landedRule) subject(base string) string {
	if r.RedSubject == "" {
		return base
	}
	return r.RedSubject
}

// redTitle is the ratchet bead's title.
func (r landedRule) redTitle(base, tree string) string {
	return r.subject(base) + " red at " + abbrevSHA(tree)
}

// silentTitle and ceilingTitle are the watchdog's two.
func (r landedRule) silentTitle(repo string) string {
	return r.Name + " silent at " + AbbrevHome(repo)
}

func (r landedRule) ceilingTitle(repo string) string {
	return r.Name + " over " + r.CeilingTitle + " at " + AbbrevHome(repo)
}

// memKey is how one repo's in-memory watchdog state is keyed. BOTH the rule
// and the repo, because one launcher judges one repo under both rules and a
// map keyed on the repo alone would have the wall check's last sentence
// reported as the judge's (quietSaid, quietUnreset, saidOwnCopy).
func (r landedRule) memKey(repo string) string { return r.QuietKey + " " + repo }

// landedJudgeRule is ADR 0088's: the landed tree's own judge. Every literal
// here is the one the code carried before ADR 0097 D5 parameterised it, and
// that is load-bearing — the suite pins these sentences by their words
// (landedfile_test.go, landedhealth_test.go), so a reword here is a reword of
// a bead a persona is holding.
var landedJudgeRule = landedRule{
	Name:               "landed check",
	Agent:              "judge",
	Script:             landedCheckScript,
	Extract:            "judge",
	Key:                landedCheckKey,
	QuietKey:           landedQuietKey,
	FileMarkerPrefix:   landedFileMarkerPrefix,
	HealthMarkerPrefix: landedHealthMarkerPrefix,
	RedSubject:         "", // main itself
	RedWhen:            "after a landing, outside the lock (ADR 0088 D1/D2)",
	RedAdvice:          "What to do: reproduce with `%s` on a fresh %s — it is the prerequisites of `make test`, so a seat hits the same red before any suite arm runs. Fix it on a branch and land it as usual. ",
	CeilingTitle:       "its D6 ceiling",
	SilentAdvice:       "What to do: run `%s` by hand on a fresh checkout of %s and time it. If it answers, the silence is the cage or the worktree and the sentences above name which; if it does not, D6's derived body has outgrown %s and the bound is the thing to review. ",
	CeilingAdvice:      "What to do: price the doors (`%s` is the prerequisites of `make test`, derived from the Makefile, so the body grows with every door somebody adds) and decide whether the price is worth paying. ADR 0088 D6's alternative (b′) — `make -j --output-sync` with a serial re-run of the failed door — is deferred with \"revisit if D6.3 files an over-ceiling bead\" written on it, and this is that bead. Alternative (b), narrowing the body by hand, costs D6's derivation and is refused there. ",
	SilentCost:         "no landing is judged, nothing is recorded, and ADR 0088 D5 files nothing",
	RunADR:             "ADR 0088 D1",
	FileADR:            "ADR 0088 D5",
	HealthADR:          "ADR 0088 D6.3",
	PriceADR:           "ADR 0088 D6.2",
	BoundsADR:          "ADR 0088 D6.1/D6.2",
	CeilingADR:         "ADR 0088 D6.2/D6.3",
}

// landedRules is both of them, for the pins that hold the pair apart and for
// a reader who wants to know how many there are. Two, and a third would need
// a record and two markers of its own.
func landedRules() []landedRule { return []landedRule{landedJudgeRule, wallCheckRule} }

// landedRunEnv is the environment one run hands whichever check it runs:
// TMPDIR and GOCACHE inside the run dir. ONE spelling, because the judge and
// the wall check run over the same worktree in the same run dir and ADR 0097
// D1's argv pin reads the difference between their two cages — two spellings
// of this would show up there as a difference the venue did not ask for.
//
// The directories themselves are made by judge(), which runs first by
// construction (ADR 0097 D2: the wall check runs after a judgement).
func landedRunEnv(run string) []string {
	return []string{"TMPDIR=" + landedCheckTmpDir(run), "GOCACHE=" + landedCheckGoCache(run)}
}

// landedCheckTmpDir is that TMPDIR. Named because three places spell it and
// one of them is a cage render (landedCheckTmp, which binds it over /tmp).
func landedCheckTmpDir(run string) string { return filepath.Join(run, "tmp") }

// ceilingOf is D6.2's price as this checker reads it, and it is the same
// number for both rules: a field with a zero default rather than the constant
// at the call site, for landedChecker.cage's reason — a 240s wall is
// unreachable inside a test, so without it the arm where an answered run IS
// over the price would have no spelling here either.
func (c *landedChecker) ceilingOf() time.Duration {
	if c.ceiling <= 0 {
		return landedCheckCeiling
	}
	return c.ceiling
}
