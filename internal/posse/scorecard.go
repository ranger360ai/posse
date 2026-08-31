package posse

// posse scorecard — per-persona outcome metrics from bd data, read-only
// (rangerhq-h2c). The PIDs name the metrics (ADR 0001 catalog); this
// makes the ones bd can see observable, and says plainly which it cannot.
//
// Sources: `bd list --all --json` per configured repo (assignee, status,
// created_by, closed_at, close_reason) and, for reopens, the git history
// of .beads/issues.jsonl — bd's own snapshot has no status history, but
// the repo tracks every sync commit, so a closed→open transition between
// two commits is a reopen.

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Score struct {
	Persona  string
	Closed   int // beads assigned to the persona, status closed
	Reopened int // of those, later reopened (git history of issues.jsonl)
	// ReposScored is how many beads repos this score sums, and
	// ReposWithHistory how many of those had a readable git history for the
	// census. Reopened is exact only when the two agree: at zero it is an
	// absence of evidence rather than a zero, and in between it is a floor —
	// the repos with no history could hold any number of reopens. Every
	// rendering has to say which of the three it is holding, rather than
	// print a partial count as if it were the whole (ranger-base-0tc for the
	// none case, ranger-base-od6g for the floor).
	ReposScored      int
	ReposWithHistory int
	Open             int           // assigned, status open
	Held             int           // assigned, status in_progress
	Blocked          int           // assigned, status blocked
	AgeAtClose       time.Duration // median closed_at − created_at over Closed (0 if none)
	Filed            int           // created_by the persona
	Rejected         int           // filed and closed with a reason reading invalid/duplicate/wontfix
}

// ReopensKnown reports whether Reopened counts every repo this score sums.
func (s Score) ReopensKnown() bool {
	return s.ReposWithHistory > 0 && s.ReposWithHistory == s.ReposScored
}

// ReopensPartial reports whether Reopened is a floor: read from some of the
// scored repos and unreadable in the rest.
func (s Score) ReopensPartial() bool {
	return s.ReposWithHistory > 0 && s.ReposWithHistory < s.ReposScored
}

// NotYetComputable prefixes every metric line the scorecard cannot answer
// yet. The catalog is derived from the PIDs (ADR 0001 amendment), so a
// declared id is never "unknown" — it is either computed here or waiting
// for an answerer, and the line says which.
const NotYetComputable = "declared, not yet computable: "

// metricNeeds says what bd would have to show for one of the ADR's own
// catalog ids to become computable. Deliberately short: the crew's ids are
// the *instance's* vocabulary, not the product's, so anything this map
// does not name gets the honest general answer rather than a hardcoded
// guess about someone else's persona.
var metricNeeds = map[string]string{
	"blocked-honestly":              "a dispatch-side outcome bd does not record — blocked with a stated need vs silently idle",
	"designs-implemented-unchanged": "a comment scan of the implementation beads for design-divergence markers",
	"spec-clarity":                  `a comment scan of the specced beads for "clarify" markers`,
}

// MetricNeeds is what bd would need for an id the scorecard cannot answer.
func MetricNeeds(id string) string {
	if n := metricNeeds[id]; n != "" {
		return n
	}
	return "an answerer over what bd shows — status, assignee, created_by, close reason, comments, timestamps"
}

// Metric renders one metric id for this persona: the number when the
// scorecard can compute it, otherwise what bd would need.
func (s Score) Metric(id string) string {
	switch id {
	case "closed-no-reopen":
		// An unknown must not wear a checkmark: with no history to read
		// transitions from, the score is a ceiling, not a number.
		if !s.ReopensKnown() {
			// A partial read is a floor, and the score it licenses is still a
			// ceiling: the repos with no history could hold any number more.
			if s.ReopensPartial() {
				return fmt.Sprintf("%d closed, ≥%d reopened (git history for %d of %d beads repos) → ≤%d",
					s.Closed, s.Reopened, s.ReposWithHistory, s.ReposScored, s.Closed-s.Reopened)
			}
			return fmt.Sprintf("%d closed, reopens unknown (no git history for %s) → ≤%d", s.Closed, beadsJSONL, s.Closed)
		}
		return fmt.Sprintf("%d closed, %d reopened → %d", s.Closed, s.Reopened, s.Closed-s.Reopened)
	// The PIDs' spelling is canonical (ADR 0001 amendment); the ADR's
	// original `findings-survive-triage` stays a computed alias so an
	// older PID keeps its number.
	case "findings-surviving-triage", "findings-survive-triage":
		if s.Filed == 0 {
			return "nothing filed yet"
		}
		return fmt.Sprintf("%d filed, %d rejected → %d", s.Filed, s.Rejected, s.Filed-s.Rejected)
	case "cost-per-closed-bead":
		return "see posse cost — API-equiv $ per bead by tier from the transcripts of every runtime with a cost adapter (ADR 0003 §4, ADR 0012 D4), joined to closes by bead id"
	}
	return NotYetComputable + MetricNeeds(id)
}

// MetricComputed reports whether the scorecard answers an id from data.
// It asks Metric rather than keeping a second list, so the two can never
// disagree about what is computable.
func MetricComputed(id string) bool {
	return !strings.HasPrefix(Score{}.Metric(id), NotYetComputable)
}

// MetricCatalogReport writes the derived catalog: every id the PIDs (or
// config) declare, whether the scorecard computes it, and who declares it.
// `posse scorecard --catalog`. Reads no bd data — it is a vocabulary check.
func (a *App) MetricCatalogReport(w io.Writer) error {
	cat := a.MetricCatalog()
	if len(cat) == 0 {
		return Die("no metrics: declared by any PID in %s, and no metric_ids: in config", a.AgentsDir)
	}
	fmt.Fprintf(w, "metric catalog — the union of every PID's metrics: and config metric_ids: (ADR 0001 amendment)\n\n")
	fmt.Fprintf(w, "  %-34s %-9s %s\n", "id", "state", "declared by")
	for _, id := range MetricCatalogIDs(cat) {
		state := "declared"
		if MetricComputed(id) {
			state = "computed"
		}
		fmt.Fprintf(w, "  %-34s %-9s %s\n", id, state, MetricDeclaredBy(cat, id))
	}
	fmt.Fprintf(w, "\ncomputed = posse scorecard answers it from bd data; declared = a PID names it\nand the answerer is not written yet (posse scorecard says what bd would need)\n")
	return nil
}

var rejectWords = []string{"invalid", "duplicate", "dup", "wontfix", "won't fix", "not a bug"}

// rejectRe matches rejectWords as WORDS. It was strings.Contains until
// ranger-base-5fyg, and a substring test over a free-text field reads this
// shop's own engineering vocabulary as a rejection: "dup" is inside dedupes,
// deduplicated, duplicated; "invalid" is inside invalidate, invalidation,
// invalidates. Measured through this function over the live store's 520
// closed -l code / -l devops beads carrying a reason: 5 match the words,
// and the one that is not a rejection at all is ranger-base-muoo — a P1
// fix whose close reason opens "verify-after dedupes on the description
// marker" and says "15 duplicates closed" further in.
//
// The trailing s? keeps the plurals the substring test caught: real
// rejections read "closed as duplicates of x". It deliberately does NOT
// reach the -d/-tion forms, which is the whole point.
var rejectRe = regexp.MustCompile(`\b(?:` + strings.Join(rejectWordsQuoted(), "|") + `)s?\b`)

func rejectWordsQuoted() []string {
	q := make([]string, len(rejectWords))
	for i, w := range rejectWords {
		q[i] = regexp.QuoteMeta(w)
	}
	return q
}

// isRejectedClose reports whether a close_reason says the bead was rejected
// rather than done — the scorecard's `Rejected` column, and one half of
// verify-after's reason to file no QA session for it (verifyafter.go). One
// vocabulary, read from the field bd writes when a closer passes
// `bd close -r <reason>`; a close with no reason at all is not rejected, it
// is unexplained.
//
// Word-matching is a narrowing, not a fix, and this function is not one
// either: "the retry no longer files a duplicate bead" describes a shipped
// fix in whole words. Free text cannot carry a machine verdict on its own,
// so the caller that pays for a wrong answer — verify-after, where a false
// rejection SUPPRESSES a control rather than miscounting a cell — reads a
// structured signal alongside it (verifyafter.go: no commit names the id).
func isRejectedClose(reason string) bool {
	return rejectRe.MatchString(strings.ToLower(reason))
}

// ScoreIssues computes one persona's score over a repo's issues; reopens
// maps issue id → reopen count for that repo.
func ScoreIssues(persona string, issues []BdIssue, reopens map[string]int) Score {
	s := Score{Persona: persona, ReposScored: 1}
	if reopens != nil {
		s.ReposWithHistory = 1
	}
	var ages []time.Duration
	for _, is := range issues {
		if is.CreatedBy == persona {
			s.Filed++
			if is.Status == "closed" && isRejectedClose(is.CloseReason) {
				s.Rejected++
			}
		}
		if is.Assignee != persona {
			continue
		}
		switch is.Status {
		case "closed":
			s.Closed++
			if reopens[is.ID] > 0 {
				s.Reopened++
			}
			if is.ClosedAt != nil && !is.Created.IsZero() {
				ages = append(ages, is.ClosedAt.Sub(is.Created))
			}
		case "open":
			s.Open++
		case "in_progress":
			s.Held++
		case "blocked":
			s.Blocked++
		}
	}
	if len(ages) > 0 {
		sort.Slice(ages, func(i, j int) bool { return ages[i] < ages[j] })
		s.AgeAtClose = ages[len(ages)/2]
	}
	return s
}

// addScore sums two repos' scores. The median age is not summable; keep
// the one from the larger closed sample (a's count is pre-addition).
func addScore(a, b Score) Score {
	if b.Closed > a.Closed || a.AgeAtClose == 0 {
		a.AgeAtClose = b.AgeAtClose
	}
	a.Closed += b.Closed
	a.Reopened += b.Reopened
	a.ReposScored += b.ReposScored
	a.ReposWithHistory += b.ReposWithHistory
	a.Open += b.Open
	a.Held += b.Held
	a.Blocked += b.Blocked
	a.Filed += b.Filed
	a.Rejected += b.Rejected
	return a
}

// ReopensFromGit counts closed→not-closed transitions per issue id across
// the git history of the census JSONL bd actually reads for dir — the
// redirect target's when .beads/redirect names one, dir's own otherwise.
// Returns nil when there is no git or no history — callers treat that as
// "unknown", not zero.
//
// The redirect hop is beadsHome's (beadloss.go), and for the same reason:
// under ADR 0012 D3-C the one `beads:` entry tracks no jsonl at all, so
// walking its own history finds fewer than two commits forever and the
// scorecard prints "reopened: ?" for the life of the instance. That is
// unknown rather than a lie, but closed-no-reopen is a crew metric and it
// stops being measurable at cut-over.
//
// Both commands run from inside the resolved .beads directory, so no repo
// root has to be derived from the redirect target — `git -C` finds
// whichever checkout the directory belongs to. A `log` pathspec is
// cwd-relative already; `git show <rev>:<path>` is repo-root-relative
// unless the path begins with "./", which is what makes the blob read
// follow the same cwd (gitrevisions, "<rev>:./<path>").
func ReopensFromGit(dir string) map[string]int {
	home := beadsHome(dir)
	revs, err := gitBead(home, "log", "--format=%H", "--reverse", "--", beadsJSONL)
	if err != nil {
		return nil
	}
	shas := strings.Fields(string(revs))
	if len(shas) < 2 {
		return nil
	}
	out := map[string]int{}
	prev := map[string]string{}
	for _, sha := range shas {
		blob, err := gitBead(home, "show", sha+":./"+beadsJSONL)
		if err != nil {
			continue
		}
		cur := map[string]string{}
		for _, ln := range strings.Split(string(blob), "\n") {
			var rec struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			}
			if json.Unmarshal([]byte(ln), &rec) == nil && rec.ID != "" {
				cur[rec.ID] = rec.Status
			}
		}
		for id, st := range cur {
			if prev[id] == "closed" && st != "closed" {
				out[id]++
			}
		}
		prev = cur
	}
	return out
}

// Scorecard aggregates scores across the configured repos for every
// persona (or one), and prints them with the metric lines each PID names.
func (a *App) Scorecard(bd Bd, w io.Writer, personaFilter string, now time.Time) error {
	personas := a.ListAgents()
	if personaFilter != "" {
		personas = []string{personaFilter}
	}
	if len(personas) == 0 {
		return Die("no personas in %s", a.AgentsDir)
	}
	totals := map[string]Score{}
	repos := 0
	var failed []error
	var allIssues []BdIssue
	for _, dir := range a.BeadsDirs() {
		issues, err := bd.ListAll(dir)
		if err != nil {
			// A repo the scan could not read has an UNKNOWN history, not an
			// empty one — the rule ReadyAll already keeps for a queue
			// (rangerhq-llse, ranger-base-vlrp). Dropping it here is the
			// quieter hazard: the table still renders, every column is a
			// plausible number, and each persona's closed/open/filed count
			// is short by whatever that repo held, with nothing on the page
			// saying so. UnresolvedDirs is the helper for a caller that
			// never reaches bd; this one does, so the failure it must name
			// is bd's own — a locked database or a repo with no bd init as
			// much as a path that is not there.
			failed = append(failed, ScanError{Dir: dir, Err: err})
			continue
		}
		repos++
		allIssues = append(allIssues, issues...)
		reopens := ReopensFromGit(dir)
		for _, p := range personas {
			totals[p] = addScore(totals[p], ScoreIssues(p, issues, reopens))
		}
	}
	if repos == 0 {
		// A present-but-empty `beads:` names no repos to fail, so there is
		// nothing to list; anything else must say which repo and why, which
		// is the whole of what an operator can act on here.
		if len(failed) == 0 {
			return Die("no beads repos readable (config beads: list)")
		}
		named := make([]string, 0, len(failed))
		for _, err := range failed {
			named = append(named, err.Error())
		}
		return Die("no beads repos readable (config beads: list): %s", strings.Join(named, "; "))
	}
	// Above the table, and on w rather than stderr. These numbers ARE the
	// output of this command: a caveat the operator reads after the count
	// has already been read, and one that goes to stderr is gone the moment
	// the card is piped into a file or pasted into a report — which is what
	// a scorecard is for.
	for _, err := range failed {
		fmt.Fprintf(w, "scorecard scan failed: %v\n", err)
	}
	if len(failed) > 0 {
		fmt.Fprintf(w, "scored %d of %d configured beads repo(s) — every number below counts only those; the rest is unknown, not zero\n\n",
			repos, repos+len(failed))
	}
	// The column, the metric line and the trailer all read one fact, carried
	// on the scores themselves (ScoreIssues: reopens != nil, counted per
	// repo), so they cannot disagree. Every persona is scored over the same
	// repos, so any total answers the repo counts for the card.
	coverage := totals[personas[0]]
	fmt.Fprintf(w, "%-16s %6s %8s %5s %5s %7s %10s %6s %8s\n", "persona", "closed", "reopened", "open", "held", "blocked", "age@close", "filed", "rejected")
	for _, p := range personas {
		s := totals[p]
		s.Persona = p
		re := fmt.Sprint(s.Reopened)
		switch {
		case s.ReopensPartial():
			// A floor wears its sign: some scored repo's history was
			// unreadable, so this count is at least, never exactly.
			re = "≥" + re
		case !s.ReopensKnown():
			re = "?"
		}
		fmt.Fprintf(w, "%-16s %6d %8s %5d %5d %7d %10s %6d %8d\n", p, s.Closed, re, s.Open, s.Held, s.Blocked, fmtAge(s.AgeAtClose), s.Filed, s.Rejected)
	}
	fmt.Fprintln(w)
	for _, p := range personas {
		ag, err := a.LoadAgent(p)
		if err != nil || len(ag.Metrics) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s\n", p)
		for _, id := range ag.Metrics {
			fmt.Fprintf(w, "  %-32s %s\n", id, totals[p].Metric(id))
		}
	}
	switch {
	case coverage.ReopensPartial():
		fmt.Fprintf(w, "\nreopened: ≥ — git history of %s read for %d of the %d scored beads repo(s); the rest holds an unknown number of reopens, not zero\n",
			beadsJSONL, coverage.ReposWithHistory, coverage.ReposScored)
	case !coverage.ReopensKnown():
		fmt.Fprintln(w, "\nreopened: ? — no git history of .beads/issues.jsonl to read transitions from")
	}
	writeHarnessRatios(w, allIssues, personas, now)
	return nil
}

// ─── harness-upkeep ratio (rangerhq-ndi) ─────────────────────────────────────
//
// DIRECTION.md's caution: Gas Town died of harness self-refinement, and
// Yegge budgets ~20-25% of all work going to harness upkeep. This makes the
// number visible so the budget is a fact, not a feeling: per window, the
// share of closed beads whose OWN id names the harness repo versus every
// other repo the instance's beads: config aggregates.
//
// A bead's repo is read from its id, not from which configured `beads:` dir
// happened to list it: a redirect can serve several repos' issues out of one
// physical store (queuejsonl.go, ADR 0015 §4 — this instance's own
// beads: list holds a single entry whose redirect chain lands on the shared
// queue db, and that db carries both the harness prefix and ranger-base-
// ids), so the dir is not a reliable repo boundary and the id prefix is the
// only fact bd hands back that is. bd assigns ids "<repo>-<slug>" with the
// slug itself never containing a hyphen (observed across every id in this
// instance's live store), so `<prefix>-` is an exact match, not a heuristic.
//
// The harness's own bd project is named at the top of this section
// (rangerhq-ndi) — the literal name survives this repo's later rename to
// posse, so the prefix on issues filed against posse's own code and process
// is still that same prefix, unchanged. Everything else (this instance:
// ranger-base-) is product/ops work, which is what the budget is measured
// against: in a single-repo instance whose one bd project IS the harness,
// the ratio is trivially 100% — the metric only means something once an
// instance's beads: aggregation actually spans a harness project and
// something else.
const HarnessRepoPrefix = "ranger" + "hq"

// IsHarnessBead reports whether id belongs to the harness repo.
func IsHarnessBead(id string) bool {
	return strings.HasPrefix(id, HarnessRepoPrefix+"-")
}

// HarnessWindows are the report windows for the ratio: 7d and 30d, in that
// order, so a slower creep (30d) never hides behind a quiet week (7d) or
// vice versa — both windows print, always.
var HarnessWindows = []time.Duration{7 * 24 * time.Hour, 30 * 24 * time.Hour}

func fmtHarnessWindow(d time.Duration) string {
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// HarnessCounts is closed beads and how many of those are harness, for one
// persona (or the instance total) over one window.
type HarnessCounts struct {
	Harness int
	Closed  int
}

// String renders the ratio next to the counts — no threshold, no verdict,
// just the two numbers and the percent (rangerhq-ndi: "no thresholds or
// alerts, just the ratio next to the counts").
func (c HarnessCounts) String() string {
	if c.Closed == 0 {
		return "no closes"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", c.Harness, c.Closed, 100*float64(c.Harness)/float64(c.Closed))
}

// HarnessRatios buckets every closed issue into each window it falls in
// (now − ClosedAt ≤ window), split harness vs everything else, by persona
// (assignee) and total ("" key). now is a parameter rather than time.Now()
// so a hermetic test can hold the clock still against fixed closed_at
// stamps; a bead closed after now (clock skew) counts in no window rather
// than in every one.
func HarnessRatios(issues []BdIssue, now time.Time) map[time.Duration]map[string]HarnessCounts {
	out := make(map[time.Duration]map[string]HarnessCounts, len(HarnessWindows))
	for _, w := range HarnessWindows {
		out[w] = map[string]HarnessCounts{}
	}
	for _, is := range issues {
		if is.Status != "closed" || is.ClosedAt == nil {
			continue
		}
		age := now.Sub(*is.ClosedAt)
		if age < 0 {
			continue
		}
		harness := IsHarnessBead(is.ID)
		for _, w := range HarnessWindows {
			if age > w {
				continue
			}
			bucket := out[w]
			add := func(key string) {
				c := bucket[key]
				c.Closed++
				if harness {
					c.Harness++
				}
				bucket[key] = c
			}
			add("")
			if is.Assignee != "" {
				add(is.Assignee)
			}
		}
	}
	return out
}

// writeHarnessRatios prints the ratio table: one column per window, one row
// per persona plus the instance total, read from every issue Scorecard
// already scanned — the same repos, the same failed-repo caveat above.
func writeHarnessRatios(w io.Writer, issues []BdIssue, personas []string, now time.Time) {
	ratios := HarnessRatios(issues, now)
	fmt.Fprintf(w, "\nharness-upkeep ratio (%s-* beads vs everything else the beads: config aggregates; DIRECTION.md budgets ~20-25%%, no threshold enforced here)\n", HarnessRepoPrefix)
	fmt.Fprintf(w, "%-16s", "persona")
	for _, win := range HarnessWindows {
		fmt.Fprintf(w, " %18s", fmtHarnessWindow(win))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-16s", "total")
	for _, win := range HarnessWindows {
		fmt.Fprintf(w, " %18s", ratios[win][""].String())
	}
	fmt.Fprintln(w)
	for _, p := range personas {
		fmt.Fprintf(w, "%-16s", p)
		for _, win := range HarnessWindows {
			fmt.Fprintf(w, " %18s", ratios[win][p].String())
		}
		fmt.Fprintln(w)
	}
}

func fmtAge(d time.Duration) string {
	switch {
	case d == 0:
		return "-"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}
