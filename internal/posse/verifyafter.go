package posse

// verify-after — the one dispatch affordance ADR 0006 §3 adds to the
// substrate. Collaboration otherwise lives entirely in beads that personas
// file themselves; this is the single handoff the harness files, because the
// alternative (convention only) fails silently: a builder who forgets leaves
// QA idle forever and nothing in the queue says so.
//
// The rule, once, per pass and per `posse ready`: for every bead in every
// `beads:` repo that carries a `verify_labels:` label and closed after this
// repo's watermark, if it has no `qa` dependent, file
//
//	verify: <title>   -l qa  [-a <verify_assignee>]  --deps discovered-from:<id>
//	                  [-t feature|-t bug | -l debt]
//
// (`-a` only when `verify_assignee:` names one — unset files it unassigned;
// the class is the CLOSE's own, inherited through the one class helper —
// ADR 0006 §1/§3, amended 2026-09-02, and verifyClassRank below)
//
// One close is exempt: one whose `close_reason` says it was REJECTED rather
// than done (duplicate, invalid, wontfix — scorecard.go's rejectWords) AND
// which no commit names. Such a close builds nothing, so the QA session it
// would file has one reachable verdict; it is skipped and named on stdout.
// A close with no reason at all is not exempt — unexplained is not rejected
// — and neither is one that shipped commits whatever its reason says
// (ranger-base-5fyg: "15 duplicates closed" is a fix, not a rejection).
//
// with the closer, the close reason, the commits `git log --grep <id>` finds,
// and a pointer at the closed bead's OWN description as the checklist (ADR
// 0006 §4, simplified 2026-09-05: acceptance is explicit, never inferred) —
// then comment `verify filed: <qid>` on the closed bead. A closer who filed
// the verify bead first is seen (the qa dependent) and not duplicated.
//
// Config `verify_batch: N` makes that one bead per N closes rather than one
// per close (N=1, the default, is the rule exactly as written above). The
// batched bead carries all N closers, close reasons and commit lists, one
// section apiece, and `verify filed: <qid>` goes back on every close in it.
// It exists because the 1:1 ratio is an amplifier: the same work is verified
// either way, but the verify bead's OWN follow-up work fires once per batch
// instead of once per close, and that is what took the queue's branching
// factor above 1.0 (ranger-base-1t7r, and DefaultVerifyBatch below).
//
// Config `verify_chain_max: N` bounds the DEPTH of that rule. Verify files
// a bead, the verifier files findings from it, the fix closes, and verify
// files a bead about THAT close — a cycle whose every round is locally
// reasonable and which nothing in this file used to stop. One project crew
// on another instance ran ~20 rounds on a single config-test pin before a
// human paused it (ranger-base-ewnas). Above N rounds on one lineage the
// next close files ONE `-l question` bead for the operator instead of
// another verify bead, which ends the cycle where the harness owns it: no
// verify bead, no verifier session, no findings bundle, nothing to fix.
// `verify_chain_max: 0` turns the bound off, and nothing else does: it binds
// at every `verify_batch:`, out of two halves that only work together (ADR
// 0006 §3, amended 2026-11-09 by ranger-base-rffxq, from the verify of
// ranger-base-ewnas via ranger-base-bu8rz). A batch is filed LIKE WITH LIKE
// — the pending closes are grouped by the rounds their lineages have already
// spent, and `verify_batch:` applies within each group (verifyRoundGroups) —
// and a batched hop CONTINUES through the shallowest of the verify bead's
// closes rather than stopping there (verifyChainRounds). The minimum is a
// floor on the true count, which is the direction every count here errs, and
// over a batch filed by that grouping every source has the same count, so
// the floor is exact.
//
// It is one query per repo per pass and one rule. It is not a workflow
// engine: the verify bead never holds the close, and nothing here reopens
// anything.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultVerifyLabels is config `verify_labels:` when the key is absent —
// the two labels whose closes are claims about working software. A present
// but empty key turns verify-after off.
var DefaultVerifyLabels = []string{"code", "devops"}

// DefaultVerifyBatch is config `verify_batch:` when unset: one verify bead
// per close, the 1:1 gate ADR 0006 §3 describes.
//
// N is a quantum, not a coverage cut. N closes earn ONE verify bead carrying
// all N, so the same work is verified — in one session instead of N. What is
// divided is the FILING amplification: the verify bead's own follow-up work
// fires once per batch instead of once per close. The staffing review
// (ranger-base-1t7r) measured this queue's branching factor at rho = 1.14
// successor beads of work per bead closed, 90% CI [1.02, 1.25], of which the
// 1:1 gate is the code -> qa leg at 0.86. Above 1.0 the queue grows without
// bound AT ANY HEADCOUNT; N=4 puts rho at 0.875 and the shop at ~8.4 seats.
const DefaultVerifyBatch = 1

// DefaultVerifyBatchAge is config `verify_batch_age:` when unset: how long a
// PARTIAL batch waits for the closes that would fill it before it is filed
// short.
//
// This is the one question batching poses, and neither obvious answer is
// right. Filing every leftover on the pass that sees it makes N a ceiling
// rather than a quantum — passes are frequent and most see a single close,
// so `verify_batch: 4` would still file 1:1 and buy nothing. Holding until N
// arrives is worse in the tail: a shop that goes quiet three closes into a
// batch of four never verifies those three, and nothing in the queue says
// so. So: hold, bounded. A partial batch is held until its OLDEST close
// reaches this age, then filed as it stands. Neither named failure survives.
const DefaultVerifyBatchAge = 24 * time.Hour

// DefaultVerifyChainMax is config `verify_chain_max:` when unset: how many
// verify rounds ONE LINEAGE earns before the next close in it goes to the
// operator instead of to a verifier. 0 turns the bound off.
//
// ADR 0006 §6 already bounds the FAN-OUT of one verify close — a finding
// whose fix runs nothing is a comment and files no bead — and the 2026-09-02
// rule before it bounded beads per verify close. Neither bounds DEPTH: a
// chain of live findings, each one genuinely live, each filed as one bundle,
// walks as far as the two seats will carry it.
//
// MEASURED 2026-11-09 by scripts/chain-depth-census.py over one shop's whole
// bead store (3,353 beads, 2026-08-12 .. 2026-11-09, 2,634 of them not
// verify beads), counting for each close the rounds already spent on its
// lineage by exactly the walk verifyChainRounds makes:
//
//	0 rounds 2055 (78.0%) · 1 round 527 (20.0%) · 2 rounds 41 (1.6%)
//	3 rounds 8 (0.3%) · 4 rounds 3 (0.1%) · deepest observed 4
//
// So 98.0% of closes are inside one round and that shop's own deepest chain
// is four. Three is therefore a bound no converging piece of work on that
// queue has ever needed, and it would have escalated 11 closes in 89 days —
// about one a week for the operator, against ~20 sessions spent on one
// artifact in the incident that filed this. The per-artifact key the bead
// proposed was measured and rejected: see verifyChainRounds.
//
// That census is the walk as it was: it stopped at a batched verify bead, so
// it priced the bound at the default batch and at no other (ranger-base-bu8rz
// measured what that cost — nothing on that queue had been reachable since
// 2026-09-06, every one of the 11 having closed between 2026-08-22 and
// 2026-09-06, while the shop still filed single-close verify beads).
//
// THE WALK THAT SHIPS NOW CONTINUES THROUGH THE SHALLOWEST SOURCE and this
// default does not move, because over the same store the two keys agree on
// exactly the closes that matter. MEASURED 2026-11-09 by minkey.py
// (docs/notes.d/ranger-base-rffxq.md; folded into the census as its MIN key),
// over the same 2,639 non-verify beads:
//
//	0 rounds 2058 · 1 round 521 · 2 rounds 49 · 3 rounds 8 · 4 rounds 3
//
// Eleven at or above 3 — the same eleven, and the same deepest of 4. The new
// walk reads DEEPER than the old one for 8 closes and shallower for none,
// which is the only direction continuing through a batch can move a count.
const DefaultVerifyChainMax = 3

const (
	// DefaultVerifyAssignee is config `verify_assignee:` when unset: nobody.
	// The harness ships no crew, so it cannot know which persona verifies —
	// a compiled-in name files at a persona a fresh instance does not have.
	// Unassigned is the honest default: the bead is filed, it is ready, and
	// whoever verifies claims it. `verify_assignee:` is the only source.
	DefaultVerifyAssignee = ""
	// VerifyLabel is the label a verify bead carries — and therefore the
	// label whose presence on a dependent means one already exists.
	VerifyLabel = "qa"
	// VerifyActor is the bd audit actor for beads the harness itself files,
	// so `created_by` distinguishes them from a persona's own work.
	VerifyActor = "posse"

	verifyCommitLimit = 10
	verifyTitleMax    = 200
	// verifyChainMaxBound caps `verify_chain_max:`, so that an operator typo
	// of 10000 is not a pass that spends the launcher lock on a graph walk.
	// Well above the deepest chain this queue has produced
	// (DefaultVerifyChainMax's census: four).
	verifyChainMaxBound = 50
	// verifyChainReadMax caps the work ONE candidate's walk may do: how many
	// dependency lists it reads, cache hits counted. Past it the count is a
	// FLOOR (verifyChainRounds), and a floor never escalates.
	//
	// The bound itself was that cap until the walk began to branch
	// (ranger-base-rffxq): a batched hop continues through the SHALLOWEST of
	// its verify bead's closes, so it reads all of them, and the worst case
	// is a tree rather than a line — 1 + 4 + 16 = 21 at the shipped shape
	// (bound 3, `verify_batch: 4`), the depth cap ending the level below.
	// 50 leaves that shape half its budget again, and is what the old
	// one-read-per-round walk could already spend (verifyChainMaxBound).
	// The ordinary case is far under: a source with no round above it ends
	// the minimum early, and 78% of closes are such a source
	// (DefaultVerifyChainMax's census).
	//
	// Cache hits count because this bounds the WORK and not the bd bill —
	// verifyChainWalk's cache already makes that bill one read per bead per
	// pass, and re-walking cached edges without bound would spend the
	// launcher lock just as surely as reading them would.
	verifyChainReadMax = 50
	// verifyBranchLimit caps how many session branches one close's landing
	// block names, for the reason verifyCommitLimit exists: this is a
	// description bd has to store. A bead is normally cut for one session,
	// so more than one is already the interesting case (a relaunch under a
	// second persona, a hand-made branch) and the first few carry it.
	verifyBranchLimit = 5

	// verifyMarkerPrefix opens every verify bead's description, and is the
	// dedupe of record: which close this bead answers, written by bd in the
	// same breath as the issue itself.
	//
	// The `discovered-from` edge is not that, however much it looks like it.
	// Measured 2026-08-27 against bd 0.49.1 (ranger-base-muoo): `bd create
	// --deps discovered-from:<id>` is NOT atomic. When the parent's
	// dependency closure is tangled — this graph holds ten cycles, every one
	// a symmetric `relates-to` pair — the daemon's validation outruns the
	// client's 30s socket read timeout: bd exits 1 with "failed to read
	// response: … i/o timeout" after 30.9s, the issue IS committed, and the
	// edge is NOT. A dedupe that reads the edge therefore sees nothing and
	// files again, every pass, forever: 33 duplicate P1 verify beads between
	// 16:36 and 21:13 that day, all of them edgeless, against three parents
	// that time out deterministically while every other parent's verify bead
	// carries its edge and was filed exactly once.
	//
	// ranger-base-pkqn named the mechanism behind that: bd's cycle check is a
	// UNION ALL recursive CTE that enumerates WALKS, not nodes, to depth 100
	// over every edge type, and it starts at the `--deps` target. A symmetric
	// `relates-to` pair is a 2-cycle it bounces across ~7x per level, so the
	// query does not terminate — the "timeout" is bd waiting on itself. The
	// three deterministic parents are exactly the parents that can reach such
	// a pair; `scripts/verify-bd-dep-safety.sh <id>` says which those are.
	// This is not fixable by upgrading: the SQLite line ends at 0.50.3 with
	// the same query, and 0.51+ is the Dolt migration. The marker stays.
	verifyMarkerPrefix = "Verify the close of "
	verifyMarkerAfter  = " ("

	// verifyChainStuckPrefix opens a chain-stop escalation's title and is
	// its dedupe of record, for the reason the marker above is the verify
	// bead's: it is written by bd in the same breath as the issue, so an
	// orphaned create — the muoo class, one paragraph up — still dedupes.
	// The stopped close's id follows it immediately, so a title scan needs
	// no parser.
	//
	// Unlike settleopen.go's escalation this one dedupes against CLOSED
	// question beads too, and the difference is not an oversight: a settle
	// that recurs after a human looked at it is news again, because the
	// bead is still in the queue. Here the subject is a close that has
	// already happened, the watermark walks past it, and nothing will ever
	// re-examine it — so an answered escalation must not re-arm into a
	// second one if a later pass ever sees that close again.
	verifyChainStuckPrefix = "verify chain stopped: "

	// discoveredFromDepType is bd's own name for the handoff edge (ADR 0006
	// §1), as `bd dep list --json` spells it in `dependency_type`. The walk
	// counts THIS edge and no other: a `blocks` or `relates-to` parent is
	// not a bead somebody filed from this one.
	discoveredFromDepType = "discovered-from"
)

// verifiedSources is the set of closes this repo already has a verify bead
// for, read out of the listing the pass already made — no extra bd call, and
// closed verify beads count: one that has been answered must not be re-filed.
//
// It reads back a marker the harness itself wrote (verifySection), which is
// a contract with ourselves and is documented as one. That is the point: it
// survives a create whose second write never landed. A batched verify bead
// carries one marker per close it answers, so all N are indexed here — the
// orphan a timed-out create leaves behind still dedupes every close in it.
func verifiedSources(issues []BdIssue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		if !hasLabel(is.Labels, VerifyLabel) {
			continue
		}
		for _, id := range verifySourceIDs(is.Description) {
			out[id] = true
		}
	}
	return out
}

// verifySourceIDs recovers every close a verify description answers: one for
// a 1:1 filing, N for a batched one. Each close's section opens with the
// marker on its own line, so the scan is by line — and every field that
// could carry a newline into the description is flattened first
// (verifyOneLine), or a crafted close_reason could forge a marker and
// suppress another close's handoff forever.
func verifySourceIDs(desc string) []string {
	var out []string
	for _, line := range strings.Split(desc, "\n") {
		if id := verifySourceID(line); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// verifySourceID recovers one closed bead's id from the head of a verify
// description or of one of its sections, or "" when the text is not one the
// harness wrote. The id is a plain token (beadIDRe), so the first " (" after
// the prefix ends it.
func verifySourceID(desc string) string {
	if !strings.HasPrefix(desc, verifyMarkerPrefix) {
		return ""
	}
	rest := desc[len(verifyMarkerPrefix):]
	i := strings.Index(rest, verifyMarkerAfter)
	if i <= 0 {
		return ""
	}
	if id := rest[:i]; beadIDRe.MatchString(id) {
		return id
	}
	return ""
}

func (a *App) verifyLabels() []string {
	if yamlHasKey(a.ConfigPath, "verify_labels") {
		return YamlList(a.ConfigPath, "verify_labels")
	}
	return DefaultVerifyLabels
}

func (a *App) verifyAssignee() string { return a.CfgGet("verify_assignee", DefaultVerifyAssignee) }

// verifyBatch is config `verify_batch:`: how many closes one verify bead
// answers. A value that is not a positive whole number is named on errw and
// treated as unset — a typo must be visible, not a silently changed gate.
func (a *App) verifyBatch(errw io.Writer) int {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "verify_batch"))
	if raw == "" {
		return DefaultVerifyBatch
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		fmt.Fprintf(errw, "verify-after: config verify_batch: %q is not a positive whole number — one verify bead per close\n", raw)
		return DefaultVerifyBatch
	}
	return n
}

// verifyBatchAge is config `verify_batch_age:` as a Go duration ("24h",
// "90m"). Same rule: an unreadable value is named and the default stands,
// because this bound is the only thing between a held batch and a lost one.
func (a *App) verifyBatchAge(errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "verify_batch_age"))
	if raw == "" {
		return DefaultVerifyBatchAge
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		fmt.Fprintf(errw, "verify-after: config verify_batch_age: %q is not a positive duration — using %s\n", raw, DefaultVerifyBatchAge)
		return DefaultVerifyBatchAge
	}
	return d
}

// verifyChainMax is config `verify_chain_max:` as a round count. Same rule
// as verifyBatch: an unreadable value is named on errw and the default
// stands, because a typo must be visible rather than silently change a gate.
//
// 0 IS A VALUE here and not an error — it turns the bound off, the
// `verify_labels:`-empty convention — so the refusal is of a negative or
// non-numeric one, and of a bound past verifyChainMaxBound.
func (a *App) verifyChainMax(errw io.Writer) int {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "verify_chain_max"))
	if raw == "" {
		return DefaultVerifyChainMax
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > verifyChainMaxBound {
		fmt.Fprintf(errw, "verify-after: config verify_chain_max: %q is not a round count in 0..%d — the bound stands at %d\n",
			raw, verifyChainMaxBound, DefaultVerifyChainMax)
		return DefaultVerifyChainMax
	}
	return n
}

// verifyPolicy is the config one sweep runs under, read once at the head of
// the pass: every repo in it is answered by the same rule, and a config typo
// is named once rather than once per repo.
type verifyPolicy struct {
	Labels   []string
	Batch    int
	MaxAge   time.Duration
	ChainMax int
	Now      time.Time
}

// verifyWatermarkPath is RHQ_HOME/state/verify-after.<repo> — per repo,
// because closes are per repo and one unreadable database must not stall
// the others.
func (a *App) verifyWatermarkPath(dir string) string {
	key := dir
	if key == "" {
		if wd, err := os.Getwd(); err == nil {
			key = wd
		}
	}
	key = strings.Trim(sessionSanitizeRe.ReplaceAllString(key, "-"), "-")
	if key == "" {
		key = "cwd"
	}
	return filepath.Join(a.StateDir, "verify-after."+key)
}

func readVerifyWatermark(p string) (time.Time, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(b)))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func writeVerifyWatermark(p string, t time.Time) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(t.Format(time.RFC3339Nano)+"\n"), 0o644)
}

// VerifyAfter runs the rule over the given repos and returns how many verify
// beads it filed. Every failure is a line on errw and nothing more: this
// runs at the head of a dispatch pass, and a repo without a bd database (or
// a bd that answered badly) must not stop the fleet.
//
// It runs under the launcher lock (ADR 0011 §1), because it *acts*: filing a
// bead is the only write a pass makes before its fire loop, and its dedupe is
// two check-then-act pairs neither bd nor the filesystem makes atomic — read
// the watermark … write it, and `Dependents` … `Create`. Two launchers that
// start together therefore both see one close as new and both file for it
// (rangerhq-th7l): a duplicate verify bead is a duplicate dispatch, at full
// token price, and closing one leaves the other ready. bd has no
// create-if-absent to lean on, so the serialization has to come from here.
//
// The lock is taken and dropped inside this call, so callers must not already
// hold it: `Run` calls this before fireLoop takes its own, and `posse ready` —
// which files by the same rule (ADR 0006 §3) — holds nothing. Hold time is a
// `bd list --all` per repo, a create per new close, and — since the depth
// bound — up to verifyChainReadMax dependency reads per new close, not the
// fire loop's. In practice that is ONE such read per close: the walk's first
// call is also its last for any close that was not filed from a verify bead,
// which is 78% of them (DefaultVerifyChainMax's census), and it makes none
// at all when the bound is off or when the close already has its verify
// bead. Per REPO the bill is smaller again: one pass reads any one bead's
// edges once however many candidates' walks reach it (verifyChainWalk).
func (a *App) VerifyAfter(bd Bd, dirs []string, out, errw io.Writer) int {
	pol := verifyPolicy{Labels: a.verifyLabels()}
	if len(pol.Labels) == 0 {
		return 0 // config says off
	}
	pol.Batch, pol.MaxAge, pol.Now = a.verifyBatch(errw), a.verifyBatchAge(errw), time.Now()
	pol.ChainMax = a.verifyChainMax(errw)
	lock, err := lockLaunches(a, out)
	if err != nil {
		// One line, like every other failure here. Not acting is the safe
		// outcome: no watermark moves, so the next pass sees these closes
		// again. A dispatch pass that needs the lock still fails at fireLoop,
		// where the failure is the pass's; `posse ready` still lists.
		fmt.Fprintf(errw, "verify-after: %v\n", err)
		return 0
	}
	defer lock.Release()

	filed := 0
	for _, dir := range dirs {
		filed += a.verifyAfterRepo(bd, dir, pol, out, errw)
	}
	return filed
}

func (a *App) verifyAfterRepo(bd Bd, dir string, pol verifyPolicy, out, errw io.Writer) int {
	issues, err := bd.ListAll(dir)
	if err != nil {
		return 0 // no database here; ReadyAll names it during the queue scan
	}
	wmPath := a.verifyWatermarkPath(dir)
	mark, seeded := readVerifyWatermark(wmPath)

	newest := mark
	var cands []BdIssue
	for _, is := range issues {
		if is.ClosedAt == nil || is.ClosedAt.IsZero() {
			continue
		}
		if is.ClosedAt.After(newest) {
			newest = *is.ClosedAt
		}
		if !seeded || !is.ClosedAt.After(mark) {
			continue
		}
		if hasLabel(is.Labels, VerifyLabel) {
			continue // a verify bead is not itself verified — that is a loop
		}
		if !hasAnyLabel(is.Labels, pol.Labels) {
			continue
		}
		// A rejected close is not a claim about working software, and the
		// header above says verify_labels are the labels whose closes are.
		// `bd close -r "duplicate of x"` builds nothing, so the QA session
		// this would file has one reachable verdict — "nothing was built" —
		// at a full session's price (ranger-base-skgs: a duplicate re-cut in
		// the 08-26 lock storm cost exactly that). The vocabulary is the
		// scorecard's rejectWords, already trusted to mean the same thing.
		//
		// The words alone are not enough to decide it (ranger-base-5fyg).
		// The scorecard can afford an over-match — one metric cell is off.
		// Here an over-match SUPPRESSES A CONTROL, and it is unrecoverable:
		// the watermark advances past an exempted close, so no later pass
		// re-examines it. So the exemption needs a second signal, and the
		// only one that is not the closer's prose is what the close SHIPPED:
		// a rejection builds nothing, so no commit names the bead.
		//
		// Both, in this order — the git call is one exec, and only a close
		// whose reason already reads as a rejection pays for it:
		//
		//	words match + no commit names it  → exempt, named on stdout
		//	words match + commits             → candidate, named on stdout
		//	git cannot answer                 → candidate (doubt files the bead)
		//
		// The two not-exempt lines say what was measured, not what will
		// happen: whether the bead is then filed, adopted or batched is
		// decided below, and stdout must not claim a filing that a failed
		// `bd create` did not do.
		//
		// Its limit, stated: only a close that CARRIES the reason is caught.
		// `bd close` with no -r writes the bare "Closed", and a rationale
		// left in a comment is invisible here. The rest is process, not code.
		// Emptiness alone is not the test either — a doc-only or
		// already-working close has no commits and still earns verification;
		// it is only exempt when the reason ALSO says it was rejected.

		// THE OTHER CLOSE THAT BUILT NOTHING (ranger-base-x9e34,
		// ciwatch.go): a `ci-red` bead — the harness files those when the
		// repo's gate goes red on main — closed with no commit naming it.
		// ci-watch comments on that bead when the gate goes green and never
		// closes it (ADR 0013 §4), so such a close is a persona reading
		// "the gate is green, close this" and doing so. The CONDITION
		// ended; nobody wrote anything for a QA session to look at. Sending
		// one anyway costs a seat per red episode, and 7 of them fell in
		// the 6.6 days ci-watch was measured over.
		//
		// The second signal is the SAME ONE the rejection exemption below
		// uses, and for the same reason — a label alone must never suppress
		// a control. It is also what makes this correct in the case that
		// matters: a persona who actually FIXED ci under this bead leaves
		// commits naming it, and the verify bead is filed exactly as for
		// any other close. Label first, then the git call, so only a ci-red
		// bead pays for it.
		if hasLabel(is.Labels, CIRedLabel) {
			trail, err := gitCommitsFor(dir, is.ID)
			switch {
			case err != nil:
				fmt.Fprintf(out, "- %-14s ci-red, not exempt: git could not say what this close shipped\n", is.ID)
			case len(trail) == 0:
				fmt.Fprintf(out, "- %-14s no verify bead: a ci-red close no commit names (the gate cleared, nobody built anything)\n", is.ID)
				continue
			default:
				fmt.Fprintf(out, "- %-14s ci-red, not exempt: %d commit(s) name it\n", is.ID, len(trail))
			}
		}

		if isRejectedClose(is.CloseReason) {
			trail, err := gitCommitsFor(dir, is.ID)
			switch {
			case err != nil:
				fmt.Fprintf(out, "- %-14s rejection words, not exempt: git could not say what this close shipped\n", is.ID)
			case len(trail) == 0:
				fmt.Fprintf(out, "- %-14s no verify bead: close reason is a rejection and no commit names it (%s)\n",
					is.ID, verifyTruncate(verifyOneLine(is.CloseReason)))
				continue
			default:
				fmt.Fprintf(out, "- %-14s rejection words, not exempt: %d commit(s) name it (%s)\n",
					is.ID, len(trail), verifyTruncate(verifyOneLine(is.CloseReason)))
			}
		}
		cands = append(cands, is)
	}

	// First sight of a repo: the watermark starts at its newest close and
	// nothing is filed. The ADR says "closed since the last pass", and before
	// the first pass there is none — treating that as "since the epoch" would
	// answer a repo's whole closed history with verify beads, which is not a
	// handoff, it is a flood.
	if !seeded {
		if err := writeVerifyWatermark(wmPath, newest); err != nil {
			fmt.Fprintf(errw, "verify-after: %s: %v\n", AbbrevHome(dir), err)
		}
		return 0
	}

	sort.SliceStable(cands, func(i, j int) bool { return cands[i].ClosedAt.Before(*cands[j].ClosedAt) })

	// Every close this repo already has a verify bead for, from the listing
	// above. A create that timed out after committing leaves an orphan with
	// no edge and no comment; this is what adopts it next pass instead of
	// filing a second one — and adopting it is a handled candidate, so the
	// watermark that the timeout froze thaws on its own.
	already := verifiedSources(issues)
	// And every close a chain-stop escalation already answers: the harness
	// files one INSTEAD of a verify bead, so without this the next pass
	// sees a close with no verify bead and no qa dependent and files the
	// escalation again (verifyChainStopped).
	for id := range verifyChainStopped(issues) {
		already[id] = true
	}
	// The listing by id, for the chain walk: every bead above a close but
	// the close's own parent edge is in here already, so the walk pays bd
	// for the edges and nothing else.
	byID := make(map[string]BdIssue, len(issues))
	for _, is := range issues {
		byID[is.ID] = is
	}

	// Classify every candidate BEFORE grouping. A close that already has its
	// verify bead must not consume a slot in a batch, and a close bd could
	// not answer for must not silently join one — so both are decided per
	// candidate first, and only what is genuinely pending is batched.
	const (
		vaHandled   = iota // answered already, or answered by this pass
		vaPending          // needs a verify bead
		vaChainStop        // its lineage has spent its rounds: the operator's
		vaFailed           // bd could not say, or the bead is unusable
	)
	state := make([]int, len(cands))
	var pending, stops []int
	// The rounds already spent above each pending close, by close id: read
	// once here, then read three times — by the escalation that stops a
	// chain, by the grouping that files like with like, and by the head line
	// that tells a verifier which round it is on.
	chain := map[string][]verifyChainHop{}
	// One walk per repo pass, so no bead's edges are read twice however many
	// candidates' lineages run through it.
	walk := newVerifyChainWalk(bd, dir, byID, pol.ChainMax)
	for i, is := range cands {
		if !beadIDRe.MatchString(is.ID) {
			// Never retried: it cannot succeed, and the id is going into
			// `--deps` and `git log --grep`.
			fmt.Fprintf(errw, "verify-after: %q refused: bead id is not a plain token\n", is.ID)
			state[i] = vaFailed
			continue
		}
		done, err := a.verifyAlreadyFiled(bd, dir, is, already)
		switch {
		case err != nil:
			fmt.Fprintf(errw, "verify-after: %s: %v\n", is.ID, err)
			state[i] = vaFailed
		case done:
			state[i] = vaHandled
		default:
			// How deep is this close's lineage? The bound is on the count,
			// and a walk bd could not finish gives a FLOOR — named, and
			// never read as a reason to escalate (verifyChainRounds).
			hops, whole := verifyChainRounds(walk, is.ID)
			chain[is.ID] = hops
			if !whole {
				fmt.Fprintf(errw, "verify-after: %s: bd could not finish the verify-chain walk — %d round(s) counted, which is a floor, so the verify bead is filed\n",
					is.ID, len(hops))
			}
			// A FLOOR NEVER ESCALATES (ADR 0006 §3), said here rather than
			// left to the walk's shape. It is belt and braces today: a
			// floored path gave up with depth still to spend, so it is
			// strictly shorter than the bound (verifyChainRounds' proof),
			// and no fixture can red this `whole` on its own. It is the
			// ADR's sentence, and the next change to the walk is what it
			// is for.
			if pol.ChainMax > 0 && whole && len(hops) >= pol.ChainMax {
				state[i] = vaChainStop
				stops = append(stops, i)
				continue
			}
			state[i] = vaPending
			pending = append(pending, i)
		}
	}

	// The bound, before the batching: a close whose lineage has spent its
	// rounds must not consume a slot in a batch either, so it is decided
	// here and never reaches the loop below. The escalation IS the handling
	// — the watermark walks past this close and nothing re-examines it —
	// so a create that fails leaves it vaFailed and the watermark holds.
	for _, i := range stops {
		is := cands[i]
		qid, err := a.fileVerifyChainStop(bd, dir, is, chain[is.ID], byID, pol, errw)
		if err != nil {
			fmt.Fprintf(errw, "verify-after: %s: %v\n", is.ID, err)
			state[i] = vaFailed
			continue
		}
		already[is.ID] = true
		state[i] = vaHandled
		fmt.Fprintf(out, "+ %-14s no verify bead: %d verify round(s) on one lineage is the bound — escalated to %s, the operator decides the next one\n",
			is.ID, len(chain[is.ID]), qid)
	}

	// One verify bead per pol.Batch closes, in close order, LIKE WITH LIKE:
	// the pending closes are grouped by the rounds their lineages have
	// already spent and the batch loop runs inside each group (ADR 0006 §3,
	// amended ranger-base-rffxq; verifyRoundGroups says why the groups are
	// the other half of the bound). The trailing partial batch of EACH group
	// is HELD rather than filed short — filing it would make N a ceiling and
	// reproduce the 1:1 gate under a config key — until its oldest close
	// reaches pol.MaxAge, which is what keeps a shop that goes quiet
	// mid-batch from leaving its last closes unverified forever
	// (DefaultVerifyBatchAge). With the bound off every close has counted
	// zero rounds, so there is exactly one group and every byte of this is
	// what it was before the grouping existed.
	filed := 0
	for _, g := range verifyRoundGroups(cands, pending, chain) {
		for off := 0; off < len(g.Pending); off += pol.Batch {
			end := off + pol.Batch
			if end > len(g.Pending) {
				end = len(g.Pending)
				age := pol.Now.Sub(*cands[g.Pending[off]].ClosedAt)
				if age < pol.MaxAge {
					fmt.Fprintf(out, "~ %-14s %d close(s) held for a verify batch of %d (oldest %s)%s\n",
						AbbrevHome(dir), end-off, pol.Batch, age.Round(time.Minute), verifyRoundNote(g.Round))
					break
				}
			}
			group := make([]BdIssue, 0, end-off)
			for _, i := range g.Pending[off:end] {
				group = append(group, cands[i])
			}
			qid, err := a.fileVerifyBead(bd, dir, group, verifyChainHead(group, chain, pol.ChainMax), already, errw)
			if err != nil {
				fmt.Fprintf(errw, "verify-after: %s: %v\n", verifyIDList(group), err)
				for _, i := range g.Pending[off:end] {
					state[i] = vaFailed
				}
				continue
			}
			filed++
			for _, i := range g.Pending[off:end] {
				state[i] = vaHandled
				fmt.Fprintf(out, "+ %-14s verify filed: %s\n", cands[i].ID, qid)
			}
		}
	}

	// high advances only while every earlier candidate was handled: a bead
	// this pass did not answer for — a bd failure, or a close held for the
	// next batch — must be seen again next pass, and the watermark is the
	// only thing that remembers it. That is the whole of the batch's memory:
	// the pending set is not a new store, it is the closes the watermark has
	// not passed yet. Filing is idempotent across passes (`already` above,
	// then the qa-dependent check) and, under the launcher lock, across
	// launchers too, so re-seeing costs no bd call it did not already make.
	high, stuck := mark, false
	for i, is := range cands {
		if state[i] != vaHandled {
			stuck = true
		}
		if !stuck {
			high = *is.ClosedAt
		}
	}
	if !stuck {
		high = newest // closes that were never candidates are past too
	}
	if high.After(mark) {
		if err := writeVerifyWatermark(wmPath, high); err != nil {
			fmt.Fprintf(errw, "verify-after: %s: %v\n", AbbrevHome(dir), err)
		}
	}
	return filed
}

// verifyAlreadyFiled reports whether this close already has its verify bead.
// `already` is the harness's own filings, including the orphans a timed-out
// create left behind (verifyMarkerPrefix); the qa dependent is the
// convention path — a closer who filed the verify bead itself, which this
// rule exists to backstop, not to override.
//
// This and the Create below are a check-then-act pair, and bd offers no
// create-if-absent to fold them into one: it is only a dedupe because
// VerifyAfter holds the launcher lock around both. `already` needs no lock —
// it is a read of state bd committed atomically with the issue.
func (a *App) verifyAlreadyFiled(bd Bd, dir string, is BdIssue, already map[string]bool) (bool, error) {
	if already[is.ID] {
		return true, nil
	}
	deps, err := bd.Dependents(dir, is.ID)
	if err != nil {
		return false, err
	}
	for _, d := range deps {
		if hasLabel(d.Labels, VerifyLabel) {
			return true, nil
		}
	}
	return false, nil
}

// ─── the depth bound (ranger-base-ewnas), at any batch size
//     (ranger-base-rffxq) ───────────────────────────────────────────────────

// verifyRoundGroup is one round's worth of pending closes: the indices into
// `cands` of the closes whose lineages have spent the same number of verify
// rounds, still in close order.
type verifyRoundGroup struct {
	Round   int
	Pending []int
}

// verifyRoundGroups splits the pending closes by the rounds already counted
// on their lineages — 0 up to ChainMax-1, since a close AT the bound left as
// an escalation before this — in ascending round order.
//
// THIS IS HALF OF THE BOUND AND NOT A TIDINESS (ADR 0006 §3, amended
// 2026-11-09 by ranger-base-rffxq). The other half is verifyChainRounds
// continuing through a batched verify bead's SHALLOWEST close, which is a
// floor on the true count because every one of those closes is one the
// findings bundle could have been about. A floor is safe but it is not a
// bound: over a batch that MIXES depths it reads the shallowest member's
// count for all of them, and the deep lineage keeps its rounds hidden
// forever. Grouping is what closes that: over a batch filed like with like
// every source has spent the same rounds, so the minimum is not a floor but
// the answer, and the next close's count is exact.
//
// MEASURED 2026-11-09 (docs/notes.d/ranger-base-rffxq.md) over one shop's
// 3,353-bead store: 263 of its 351 batched verify beads mix depths, so
// without the grouping the shallowest-source walk stays shallow — it reads a
// deepest of 2 on the closes since that shop moved to `verify_batch: 4`, and
// a bound of 3 still never fires. With it, the same replay files 3.5% more
// verify beads over the whole history and 6.9% more since 2026-09-15: that
// is the price of the bound, and it is the cheapest shape measured (filing
// each deep close on its own costs 93% and 64%).
//
// Each group's tail is held on its own (DefaultVerifyBatchAge), which is the
// one cost a reader feels: a lone round-1 close waits for another round-1
// close, not for the next close of any depth. The replay's p90 wait goes
// from 1.5h to 3.9h over the whole history at `verify_batch: 4`.
//
// When the bound is off nothing has counted a round, so there is one group
// and the filing is byte for byte what it was before this existed.
func verifyRoundGroups(cands []BdIssue, pending []int, chain map[string][]verifyChainHop) []verifyRoundGroup {
	byRound := map[int][]int{}
	for _, i := range pending {
		r := len(chain[cands[i].ID])
		byRound[r] = append(byRound[r], i)
	}
	rounds := make([]int, 0, len(byRound))
	for r := range byRound {
		rounds = append(rounds, r)
	}
	sort.Ints(rounds)
	out := make([]verifyRoundGroup, 0, len(rounds))
	for _, r := range rounds {
		out = append(out, verifyRoundGroup{Round: r, Pending: byRound[r]})
	}
	return out
}

// verifyRoundNote says which group a held batch is, and is "" for the first
// round — which is almost every batch, so the ordinary hold line is the one
// this rule has always printed.
func verifyRoundNote(round int) string {
	if round <= 0 {
		return ""
	}
	return fmt.Sprintf(", each %d verify round(s) deep", round)
}

// verifyChainHop is one verify round already spent above a close: the verify
// bead, the close the walk continued through, and how many closes that
// verify bead answers — 1 at `verify_batch: 1`, N for a batched one, which
// is what the round line renders as "(one of N)".
type verifyChainHop struct {
	Verify  string // the verify bead this close was filed from
	Source  string // the close the walk continued through, "" when there was none
	Sources int    // how many closes that verify bead answers
}

// verifyChainWalk is one repo pass's chain walk: the bd seam, the listing,
// the bound, and the cache that makes the bill one `bd dep list` per BEAD
// per pass however many candidates' lineages run through it.
//
// The cache is the walk's state and the read budget is the candidate's, so
// the two live here together: verifyChainRounds resets `spent` per candidate
// and nothing resets `edges`.
type verifyChainWalk struct {
	bd    Bd
	dir   string
	byID  map[string]BdIssue
	limit int
	edges map[string][]BdDep // id -> its dependencies, read at most once
	spent int                // dependency lists this candidate has read
}

func newVerifyChainWalk(bd Bd, dir string, byID map[string]BdIssue, limit int) *verifyChainWalk {
	return &verifyChainWalk{bd: bd, dir: dir, byID: byID, limit: limit, edges: map[string][]BdDep{}}
}

// deps is one bead's dependency list, from the cache or from bd, and false
// when it cannot be had — a bd that would not answer, or this candidate's
// read budget spent (verifyChainReadMax). Both make the count a FLOOR, and
// they are one return because a floor is a floor whichever produced it.
//
// A failed read is not cached: it is a fact about this moment, not about the
// bead, and the budget already bounds what re-asking can cost.
func (w *verifyChainWalk) deps(id string) ([]BdDep, bool) {
	if w.spent >= verifyChainReadMax {
		return nil, false
	}
	w.spent++
	if d, ok := w.edges[id]; ok {
		return d, true
	}
	d, err := w.bd.DepList(w.dir, id)
	if err != nil {
		return nil, false
	}
	w.edges[id] = d
	return d, true
}

// verifyChainRounds walks the lineage above one close and returns the verify
// rounds already spent on it, newest first, plus whether the count is whole
// rather than a floor.
//
// One hop is: this close was filed `--deps discovered-from:<Q>` where Q is a
// verify bead (ADR 0006 §6's findings bundle, which is the only thing that
// edge means here), and Q answered the closes S1..SN. Continue from the
// SHALLOWEST of them: the count is 1 + the smallest count any Si carries.
// The walk ends at a close with no such parent, at a parent the listing does
// not hold or does not call a verify bead, at a verify bead whose closes are
// all already on this path, or at `limit` hops.
//
// WHY THE MINIMUM. Which of Q's N closes a findings bundle answered is
// written nowhere the harness can read (ranger-base-bu8rz measured what that
// cost: the walk used to STOP there, and above `verify_batch: 1` every
// verify bead is batched, so every walk counted exactly one round however
// deep the lineage ran and the bound could not fire at all). Every one of
// the N is a close the bundle could have been about, so the smallest of
// their counts is a LOWER BOUND on the true one — and short is the direction
// every count in this file errs in, because an undercount costs one more
// round while an overcount files an escalation the shop did not earn. Over a
// batch filed like with like (verifyRoundGroups) all N have spent the same
// rounds, so the minimum is not merely safe, it is exact.
//
// NOT THE DEEPEST, which is the obvious repair and was measured worse than
// the hole it fills (ranger-base-bu8rz): taking the deepest chain through
// all N reads 17 rounds where the lineage reads 1, over hops that are
// visibly more than a dozen unrelated subjects — a backup-lock CI-red close,
// a seatbelt credential close, a FIFO-at-redirect close — chained only
// because batching put each consecutive pair in one verify bead; at
// `verify_chain_max: 3` it escalates 202 of 2,639 closes (7.65%) against the
// minimum's 11 (0.42%). A batch is no more an artifact than a hot file is.
//
// A FLOOR CANNOT ESCALATE, and what used to make that obvious no longer
// does. While the walk was a line, a bd failure left strictly fewer than
// `limit` hops because the loop ran only while `len(hops) < limit`, so
// `len(hops) >= limit` was itself proof that every edge had been read. With
// branches that reading is gone, so the second return says it in terms and
// the caller escalates only on a whole count (it names a floor on stderr and
// files the verify bead instead).
//
// The rule for whole is one line: the minimum is whole when some branch that
// ATTAINS it is whole, because then that branch's true count IS the minimum
// — every other branch's true count is at least what it reported, which is
// at least the minimum. A tie between a whole branch and a floored one is
// therefore whole, and only a minimum every one of whose branches gave up is
// a floor.
//
// It still holds structurally, by a longer route, and that is worth writing
// down because it is what the caller's `whole` check is insuring: a path is
// floored only where it GAVE UP — a read that failed or the budget spent —
// and both can only happen with depth still to spend, so a floored path is
// strictly shorter than `limit`. A floor therefore cannot reach the bound by
// construction either, and no fixture can make it; the explicit check is for
// the change that breaks that argument without noticing.
//
// THE KEY IS THE LINEAGE, NOT THE PATH, and that is a measured choice rather
// than the obvious one. ranger-base-ewnas proposed keying the chain on the
// ARTIFACT — "a verify of a bead that was itself a verify finding on the
// same path" — so a chain that moved to a different file would start a fresh
// count. MEASURED 2026-11-09 by scripts/chain-depth-census.py, which measures
// every key over the same store and prints the deepest chains of each to be
// read by eye (3,353 beads; the artifact read as the path set of every
// commit naming the bead, two rounds on one artifact when those sets
// intersect, and a batched hop disambiguated BY THE PATH, which is that
// key's whole appeal): the path key reads 137 closes at 3 rounds or more, 10
// of them at 8 or more and the deepest at 13 — and those are visibly NOT one
// artifact. One 11-deep chain runs a cost-provider bead into a silent-revert
// bead into three `posse worktrees` beads into four common-dir-pin beads
// into a seatbelt bead, chained because every one of them touches
// internal/posse/dispatch.go and NOTES.md. A hot file is not an artifact,
// and in this tree most files are hot.
//
// The lineage key reads the same store at a deepest 4 (see
// DefaultVerifyChainMax), and every chain it reads at 3 or more is one
// subject by eye: the GIT_CONFIG inlet pin four rounds running, the
// common-dir pin three.
func verifyChainRounds(w *verifyChainWalk, id string) ([]verifyChainHop, bool) {
	if w.limit <= 0 {
		return nil, true // the bound is off: no walk, no bd calls
	}
	w.spent = 0
	return w.rounds(id, w.limit, map[string]bool{id: true})
}

// rounds is verifyChainRounds' recursion: the rounds above `id`, with
// `depth` hops left to spend and `seen` the ids already on this path.
//
// `seen` is per PATH, so a count is not memoized across candidates — only
// the READS are (verifyChainWalk.deps), which is where the cost is. A
// path-blind memo would have to drop the repeat check, and then a lineage
// that loops back on itself would count to the bound and escalate: an
// overcount, in the one direction this file refuses.
func (w *verifyChainWalk) rounds(id string, depth int, seen map[string]bool) ([]verifyChainHop, bool) {
	if depth <= 0 {
		// The depth cap, not a floor: the caller already holds `limit` hops
		// and every one of them was read. Nothing above can change what it
		// does with them.
		return nil, true
	}
	deps, read := w.deps(id)
	if !read {
		// Not knowing how deep the chain is is not evidence that it is deep.
		return nil, false
	}
	// The first `discovered-from` parent that IS a verify bead, rather than
	// the first one full stop. The two answer the same for all but one bead
	// in the census store — MEASURED 2026-11-09: of its 2,634 non-verify
	// beads, 2,005 carry exactly one `discovered-from` parent, 628 carry
	// none, and ONE carries two — but "the first" makes the count depend on
	// the order bd happens to list edges in, and in that one measured case
	// the verify bead is the SECOND. (Four parents is the ordinary shape of
	// a `verify_batch: 4` verify bead, 342 of them there; a verify bead is
	// never a bead this walk starts from or continues through.)
	var q BdIssue
	parent := ""
	for _, d := range deps {
		if d.DependencyType != discoveredFromDepType || !beadIDRe.MatchString(d.ID) || seen[d.ID] {
			continue
		}
		if is, known := w.byID[d.ID]; known && hasLabel(is.Labels, VerifyLabel) {
			parent, q = d.ID, is
			break
		}
	}
	if parent == "" {
		return nil, true
	}
	srcs := verifySourceIDs(q.Description)
	hop := verifyChainHop{Verify: parent, Sources: len(srcs)}
	best, bestWhole, bestHops, bestSrc := -1, false, []verifyChainHop(nil), ""
	for _, s := range srcs {
		if s == parent || seen[s] {
			continue
		}
		sub, whole := w.rounds(s, depth-1, verifyChainSeen(seen, parent, s))
		switch {
		case best < 0 || len(sub) < best:
			best, bestWhole, bestHops, bestSrc = len(sub), whole, sub, s
		case len(sub) == best && whole && !bestWhole:
			// A branch that TIES the minimum and is whole makes the minimum
			// whole, and the path shown is then one we can vouch for.
			bestWhole, bestHops, bestSrc = true, sub, s
		}
		if best == 0 && bestWhole {
			break // a source with no round above it: nothing can go lower
		}
	}
	if best < 0 {
		// A verify bead with no close this walk can continue through: one
		// filed by hand with no marker, or every source already on this
		// path. The round counts and the walk ends here.
		return []verifyChainHop{hop}, true
	}
	hop.Source = bestSrc
	return append([]verifyChainHop{hop}, bestHops...), bestWhole
}

// verifyChainSeen is `seen` plus `add`, copied: a branch must not mark a
// sibling branch's beads as visited, or the order bd lists a batch's closes
// in would decide which of them the walk may continue through.
func verifyChainSeen(seen map[string]bool, add ...string) map[string]bool {
	out := make(map[string]bool, len(seen)+len(add))
	for k := range seen {
		out[k] = true
	}
	for _, a := range add {
		out[a] = true
	}
	return out
}

// verifyChainStopped is every close this repo already has a chain-stop
// escalation for, read out of the listing the pass already made — the
// escalation's half of `already`, and the reason it is a TITLE scan rather
// than an edge read is verifyChainStuckPrefix's.
//
// Closed escalations count. See that constant for why this differs from
// settleopen.go, which deliberately re-arms.
func verifyChainStopped(issues []BdIssue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		if id := verifyChainStuckSource(is.Title); id != "" {
			out[id] = true
		}
	}
	return out
}

// verifyChainStuckTitle is the escalation's title — the dedupe key, so the
// stopped close's id sits at a fixed offset and nothing else may precede it.
func verifyChainStuckTitle(id string, rounds int) string {
	return verifyTruncate(fmt.Sprintf("%s%s — %d verify rounds on one lineage, the operator decides the next one",
		verifyChainStuckPrefix, verifyOneLine(id), rounds))
}

// verifyChainStuckSource recovers the stopped close's id from an escalation
// title, or "" when the text is not one the harness wrote.
func verifyChainStuckSource(title string) string {
	if !strings.HasPrefix(title, verifyChainStuckPrefix) {
		return ""
	}
	rest := title[len(verifyChainStuckPrefix):]
	if i := strings.IndexByte(rest, ' '); i > 0 {
		rest = rest[:i]
	}
	if beadIDRe.MatchString(rest) {
		return rest
	}
	return ""
}

// verifyChainStopComment is the breadcrumb left on the stopped close naming
// the escalation filed instead of its verify bead. It is the pointer the
// `discovered-from` edge may not carry (verifyMarkerPrefix's muoo note), and
// it must never read as `verify filed:` — a reader who finds that line on a
// close believes a verifier is coming.
func verifyChainStopComment(qid string, rounds, max int) string {
	return fmt.Sprintf("verify chain stopped: %s — this close's lineage has already spent %d verify round(s) and "+
		"`verify_chain_max:` is %d, so NO verify bead was filed for it and the operator decides whether there is "+
		"another round (ranger-base-ewnas). Nothing is blocked and nothing reopens; %s says what to decide.",
		qid, rounds, max, qid)
}

// verifyChainStopBody is what the operator reads. It is built from the same
// facts a verify bead's section carries — closer, close reason, commits —
// and deliberately NOT from verifySection, which opens with the dedupe
// marker: this bead is not a verify bead, and a marker saying it verifies
// this close would be both a lie to the reader and a dedupe waiting to
// happen if verifiedSources ever stops checking the label.
func (a *App) verifyChainStopBody(dir string, is BdIssue, hops []verifyChainHop, byID map[string]BdIssue, max int) string {
	id := verifyOneLine(is.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "%s closed, and its lineage has already spent %d verify round(s): every close below was filed\n"+
		"from the verify of the close before it. `verify_chain_max:` is %d, so no verify bead was filed for this\n"+
		"close — the next round is the operator's call and not another seat's (ADR 0006 §3, ranger-base-ewnas).\n\n"+
		"A verify/fix pair can iterate on one artifact without bound: every round is locally reasonable, each finding\n"+
		"is real, and nothing in the queue says the loop is running. One project crew on another instance ran ~20\n"+
		"rounds on a single config-test pin that way before a human paused it.\n\n", id, len(hops), max)
	fmt.Fprintf(&b, "close:    %s (title, quoted as data: %q)\n", id, is.Title)
	if c := verifyOneLine(verifyCloser(is)); c != "" {
		fmt.Fprintf(&b, "closer:   %s\n", c)
	}
	if r := verifyOneLine(is.CloseReason); r != "" {
		fmt.Fprintf(&b, "reason:   %s\n", r)
	}
	fmt.Fprintf(&b, "repo:     %s\n%s%s\n\n", AbbrevHome(dir), discoveredFromMarkerPrefix, id)
	b.WriteString("the chain, newest first:\n")
	for i, h := range hops {
		src := "no close it answers that this walk could continue through — the lineage ends here"
		if h.Source != "" {
			src = "close " + h.Source
			if s, ok := byID[h.Source]; ok {
				src += fmt.Sprintf(" (title, quoted as data: %q)", s.Title)
			}
			if h.Sources > 1 {
				src += fmt.Sprintf(" — one of the %d closes that verify bead answers, and the SHALLOWEST of them, so this round's count is a floor on the true one (ADR 0006 §3)", h.Sources)
			}
		}
		fmt.Fprintf(&b, "  round %d  verify %s of %s\n", len(hops)-i, h.Verify, src)
	}
	if lines, _ := gitCommitsFor(dir, is.ID); len(lines) > 0 {
		fmt.Fprintf(&b, "\ncommits naming %s (git log --grep; a commit may merely CITE the bead):\n", id)
		for _, l := range lines {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	fmt.Fprintf(&b, "\nWhat to decide: is the artifact converging? If it is, one more round is cheap and this bead is how you\n"+
		"buy it — `bd create \"verify: <title>\" -l %s --deps discovered-from:%s` files it by hand, and then close this\n"+
		"bead. If it is not, another round is not the answer: rewrite the thing, hand the design to architecture, or\n"+
		"accept it as it stands and say so on %s. Closing THIS bead files nothing and blocks nothing — the close it\n"+
		"names is already closed, and verify-after will not see it again.\n", VerifyLabel, id, id)
	return b.String()
}

// fileVerifyChainStop files ONE question bead for a close whose lineage has
// spent its rounds, and comments it back on the close. It is the whole stop:
// with no verify bead there is no verifier session, no findings bundle and
// no next close, so the cycle ends at the one hop the harness owns.
//
// Nothing is blocked and nothing is reopened — there is nothing in the queue
// to block, because the subject is a close. That is the difference from
// settleopen.go's escalation, whose stuck bead is live and leaves `bd ready`
// on an edge.
func (a *App) fileVerifyChainStop(bd Bd, dir string, is BdIssue, hops []verifyChainHop, byID map[string]BdIssue, pol verifyPolicy, errw io.Writer) (string, error) {
	title := verifyChainStuckTitle(is.ID, len(hops))
	desc := a.verifyChainStopBody(dir, is, hops, byID, pol.ChainMax)
	// The same cheap half of the visibility guard fileVerifyBead takes, for
	// the same reason: this bead is built out of other beads' titles.
	a.WarnOpsContent(errw, dir, "the verify-chain escalation for "+is.ID, title+"\n"+desc)
	qid, err := bd.Create(dir, BdNew{
		Title:       title,
		Description: desc,
		Labels:      []string{SettleQuestionLabel},
		Assignee:    a.CfgGet("operator", ""),
		// As urgent as the close it stands in for, which is how
		// fileVerifyBead prices a verify bead: the escalation replaces that
		// bead, so it inherits that bead's claim on attention rather than
		// inventing one.
		Priority: strconv.Itoa(is.Priority),
		// The handoff edge, which settleopen.go's escalation cannot carry
		// and this one can: there is no `blocks` edge here for it to close a
		// cycle with (ranger-base-23oo is about that PAIR), so the ordinary
		// provenance is available. It is still the write bd can lose while
		// committing the issue, so the title above is the dedupe of record
		// and the body carries discoveredFromMarkerPrefix as well.
		Deps:  []string{"discovered-from:" + is.ID},
		Actor: VerifyActor,
	})
	if err != nil {
		// As fileVerifyBead: bd may have committed the issue and failed on
		// the edge. Say so, retry next pass, and let the title scan
		// (verifyChainStopped) adopt the orphan rather than file a second.
		return "", err
	}
	if err := bd.Comment(dir, is.ID, verifyChainStopComment(qid, len(hops), pol.ChainMax), VerifyActor); err != nil {
		fmt.Fprintf(errw, "verify-after: %s: comment: %v\n", is.ID, err)
	}
	return qid, nil
}

// verifyChainHead is the line the VERIFIER reads first when the filing is
// not the first round on its close's lineage: which round this is, of how
// many, and what the last one means. It is the bound's other half and the
// cheaper one — the harness can only stop the round after next, and a
// verifier who knows it is on round 3 of 3 can stop this one.
//
// "" when no close in the group has spent a round, which is almost all of
// them (78.0%, DefaultVerifyChainMax's census) — so a first-round filing's
// description stays byte for byte what this rule has always written, and no
// fixture of one moves.
func verifyChainHead(group []BdIssue, chain map[string][]verifyChainHop, max int) string {
	var b strings.Builder
	for _, is := range group {
		hops := chain[is.ID]
		if len(hops) == 0 {
			continue
		}
		round := len(hops) + 1
		fmt.Fprintf(&b, "VERIFY ROUND %d of %d on %s's lineage (config `verify_chain_max:`): each close below it was filed from the verify of the one before. ",
			round, max, verifyOneLine(is.ID))
		var rounds []string
		for i, h := range hops {
			switch {
			case h.Source == "":
				rounds = append(rounds, fmt.Sprintf("round %d: verify %s (no close of its own to continue through)", len(hops)-i, h.Verify))
			case h.Sources > 1:
				// A batched round: which of its closes the bundle answered
				// is not written anywhere, so the walk took the shallowest
				// and the count below is a floor (verifyChainRounds).
				rounds = append(rounds, fmt.Sprintf("round %d: verify %s of %s (one of %d)", len(hops)-i, h.Verify, h.Source, h.Sources))
			default:
				rounds = append(rounds, fmt.Sprintf("round %d: verify %s of %s", len(hops)-i, h.Verify, h.Source))
			}
		}
		fmt.Fprintf(&b, "Newest first — %s.", strings.Join(rounds, "; "))
		if round >= max {
			b.WriteString(" THIS IS THE LAST ROUND this lineage gets: a close filed from this bead files a question for the operator instead of another verify bead. " +
				"So if the artifact is not converging, say THAT on this bead's close — the finding a fourth round would make is worth less than the sentence that says the loop is not ending.")
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

// fileVerifyBead files ONE verify bead for a group of closes — one close at
// `verify_batch: 1`, N at N — and comments its id back on every close it
// covers. `head` is verifyChainHead's depth line, prepended rather than
// threaded into the sections: it is "" for a first-round filing, so a
// description this rule has always written is unchanged byte for byte, and
// the marker the dedupe reads is still found BY LINE below it. The comment is the provenance that survives: the `discovered-from`
// edges are the write that may not land at all (verifyMarkerPrefix), and
// with N of them the walk that fails to terminate has N chances to start.
func (a *App) fileVerifyBead(bd Bd, dir string, group []BdIssue, head string, already map[string]bool, errw io.Writer) (string, error) {
	title, desc := verifyGroupTitle(group), head+a.verifyGroupDescription(dir, group)
	// The cheap half of the visibility guard (rangerhq-hrz): this bead is
	// built out of OTHER beads' titles and descriptions, so it inherits
	// whatever those carried. Warn, do not refuse — the refusal is the
	// commit hook's, which sees every entry path instead of only the ones
	// this process owns, and a verify bead that never gets filed is worse
	// than one that gets filed and flagged.
	a.WarnOpsContent(errw, dir, "the verify bead for "+verifyIDList(group), title+"\n"+desc)
	deps := make([]string, 0, len(group))
	prio := group[0].Priority
	class := BeadClass(group[0])
	for _, is := range group {
		deps = append(deps, "discovered-from:"+is.ID)
		if is.Priority < prio {
			prio = is.Priority // a batch is as urgent as its most urgent close
		}
		if c := BeadClass(is); verifyClassRank(c) < verifyClassRank(class) {
			class = c // and takes as urgent a CLASS as its most urgent close
		}
	}
	labels, beadType := verifyClassFields(class)
	qid, err := bd.Create(dir, BdNew{
		Title:       title,
		Description: desc,
		Labels:      labels,
		Assignee:    a.verifyAssignee(),
		Deps:        deps,
		Priority:    strconv.Itoa(prio),
		Type:        beadType,
		Actor:       VerifyActor,
	})
	if err != nil {
		// bd may have committed the issue anyway and failed on the edges
		// (see verifyMarkerPrefix). Say so, retry next pass, and let
		// `already` find the orphan then rather than filing a second one —
		// which works for a batch only because the description carries a
		// marker for EVERY close in it, not just the first.
		return "", err
	}
	for _, is := range group {
		already[is.ID] = true
		// The bead exists; a failed comment is a lost breadcrumb, not lost
		// work, and retrying the whole thing would duplicate it.
		if err := bd.Comment(dir, is.ID, "verify filed: "+qid, VerifyActor); err != nil {
			fmt.Fprintf(errw, "verify-after: %s: comment: %v\n", is.ID, err)
		}
	}
	return qid, nil
}

// verifyCloser is who the close is attributed to. bd records no close actor,
// so the honest answer is the assignee that held the bead, then whoever
// filed it. "" when bd knows neither.
func verifyCloser(is BdIssue) string {
	if is.Assignee != "" {
		return is.Assignee
	}
	return is.CreatedBy
}

// verifyIDList is the closes a group covers, in close order — the batched
// bead's title, its error lines, and its ops-content label all name them.
func verifyIDList(group []BdIssue) string {
	ids := make([]string, len(group))
	for i, is := range group {
		ids[i] = is.ID
	}
	return strings.Join(ids, ", ")
}

// verifyGroupTitle: a single close keeps `verify: <title>`, byte for byte
// what it always was. A batch names its closes instead — N titles do not fit
// in one and the ids are what the verifier types next anyway; the titles are
// one line down, in the description.
func verifyGroupTitle(group []BdIssue) string {
	if len(group) == 1 {
		return verifyTitle(group[0].Title)
	}
	return verifyTruncate(fmt.Sprintf("verify %d closes: %s", len(group), verifyIDList(group)))
}

func verifyTitle(title string) string { return verifyTruncate("verify: " + strings.TrimSpace(title)) }

func verifyTruncate(t string) string {
	if r := []rune(t); len(r) > verifyTitleMax {
		t = strings.TrimSpace(string(r[:verifyTitleMax-1])) + "…"
	}
	return t
}

// verifyGroupDescription is what the verifier reads first. A single close is
// byte for byte the description this rule has always written; a batch is N of
// the same sections under one trailer — so the marker, which is the dedupe
// of record, appears once per close the bead covers and verifiedSources
// finds all N.
func (a *App) verifyGroupDescription(dir string, group []BdIssue) string {
	if len(group) == 1 {
		return a.verifyDescription(dir, group[0], verifyCloser(group[0]))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d closes, batched (config `verify_batch:`). Each section below is one close: verify every one of them, then close this bead once.\n\n", len(group))
	for _, is := range group {
		b.WriteString(a.verifySection(dir, is, verifyCloser(is)))
		b.WriteString("\n")
	}
	b.WriteString("Read each bead itself (`bd show <id>`) and its comments (`bd comments <id>`).\n")
	b.WriteString("Close this one with `VERIFIED: <how>` naming every close above. ")
	b.WriteString(verifyTrailer(a.verifyLane(group)))
	return b.String()
}

// verifyDescription is one close's verify bead: its section, then the
// trailer that says how to answer it.
func (a *App) verifyDescription(dir string, is BdIssue, closer string) string {
	var b strings.Builder
	id := verifyOneLine(is.ID)
	b.WriteString(a.verifySection(dir, is, closer))
	fmt.Fprintf(&b, "\nRead the bead itself (`bd show %s`) and its comments (`bd comments %s`).\n", id, id)
	b.WriteString("Close this one with `VERIFIED: <how>`. ")
	b.WriteString(verifyTrailer(a.verifyLane([]BdIssue{is})))
	return b.String()
}

// ─── the findings bundle and the class (ADR 0006 §6, amended 2026-09-06) ───

// verifyTrailer is the closing instruction on every verify bead this rule
// mints, single or batched. The ruling it spells, in the order the verifier
// meets it: each finding takes one of TWO channels, decided by one question
// — would the fix change what a process does or what a pin proves? No, and
// the finding is RECORDED, one line on the verify bead's own close comment
// and no bead filed at all (comment claims, doc sentences, NOTES/CHANGELOG
// wording, a measurement record's prose). Yes, and it is LIVE and filed as
// before: ONE findings bead carrying every live finding, labelled with the
// close's own lane and `debt`; only a LIVE defect in money, the
// constitution, or dispatch correctness earns a bead of its own, and the
// bundle then names it by id. It is one sentence for a batch as for a
// single close — the bundle is per VERIFY close, not per close verified,
// which is the amplification the 2026-09-02 ruling cut (a day that filed
// 111 beads against 86 closes, with QA's per-finding filing the largest
// line). The recorded channel is the 2026-09-06 amendment (ranger-base-0f2zy)
// and it is stated FIRST because it is the answer for most findings: a
// trailer that says what to file before it says what not to file has
// already asked for the bead.
//
// Two things it deliberately no longer says. The closer's NAME: the fix is
// lane work and the closer is not on §1's five-item `-a` allowlist (§1
// amendment of 2026-09-01, restated 2026-09-02) — this text read "file a
// bug bead `-l code -a <closer>` with a repro" until the cut, with a pin
// asserting that spelling, and both were retired together. And "a bug
// bead": the bundle is `debt`, because that is what audit findings are.
//
// The `<this bead's id>` is written as a placeholder rather than filled in
// because there is no id yet: the description is bd `create`'s argument, so
// the bead it describes does not exist while this string is built.
func verifyTrailer(lane string) string {
	return "For any close that does NOT verify, ask of each finding: would the fix change what a process does or what a pin proves? " +
		"NO — the finding is RECORDED and NO bead is filed for it: one line on THIS bead's close comment, `file:line · what · the close it escaped from`, which is the whole class of comment claims, doc sentences, NOTES/CHANGELOG wording and a measurement record's prose (ADR 0006 §6). " +
		"YES — the finding is LIVE, and for LIVE findings only, file ONE findings bead `-l " + lane +
		" -l debt --deps discovered-from:<this bead's id>` — one line per finding: file:line · what fails · the bead it escaped from · the repro or failing test; a pin that is wrong about what it measures is live. " +
		"A LIVE money / constitution / dispatch-correctness defect alone gets its own `-t bug` bead at P1/P2 with the domain in the title, named in the bundle by id (ADR 0006 §6). " +
		"Then close this one `escape` (ADR 0006 §6) either way — a close carrying recorded lines and no bead is a complete close. No live findings, no bead. The closed bead is never reopened by a persona — that is the operator's call.\n"
}

// verifyLane is the lane that bundle is filed in: the close's own. The ADR
// says `code`, or `devops` when the close was `-l devops`, and `code` for a
// batch spanning both — which is one rule once `verify_labels:` is allowed
// to be anything: the FIRST configured verify label any close in the group
// carries. Under the default `code, devops` that is the ADR's sentence
// exactly, including the tie, and an instance that verifies `-l infra`
// closes gets `-l infra` instead of a compiled-in `code` that names no lane
// it has.
//
// A group always carries one — hasAnyLabel is what made it a candidate — so
// the fallbacks below are for a direct caller only, and they answer with
// the configured lane rather than inventing a name.
func (a *App) verifyLane(group []BdIssue) string {
	labels := a.verifyLabels()
	for _, l := range labels {
		for _, is := range group {
			if hasLabel(is.Labels, l) {
				return l
			}
		}
	}
	if len(labels) > 0 {
		return labels[0]
	}
	return DefaultVerifyLabels[0]
}

// verifyClassRank orders the four classes by urgency, which is what a batch
// inherits: bug › feature › debt › unclassified. An unverified feature is
// still open feature work, so the verify bead is the class of the work it
// answers for and never a fourth "verify" bucket of its own.
//
// It is NOT BeadClasses (beads.go). That is the REPORTING order the pulse
// line spells, and a census printing in one order while a batch inherits in
// another is not an inconsistency — they answer different questions, and
// writing this order there would silently change the pulse line's columns.
func verifyClassRank(class string) int {
	switch class {
	case ClassBug:
		return 0
	case ClassFeature:
		return 1
	case ClassDebt:
		return 2
	}
	return 3
}

// verifyClassFields renders a class into the two fields bd actually stores
// (ADR 0006 §1's table, whole and in one place): the `issue_type` for
// feature and bug, and the `debt` LABEL for debt, because bd has no debt
// type — which is the whole reason the class is two fields rather than one.
//
// Unclassified in, unclassified out: an empty type passes no `-t`, so bd's
// own default (`task`) stands and the bead lands in the bucket the
// scorecard REPORTS rather than one this filer manufactured. A guess here
// would make the operator's numbers lie in exactly the direction they were
// lying before.
func verifyClassFields(class string) (labels []string, beadType string) {
	labels = []string{VerifyLabel}
	switch class {
	case ClassFeature, ClassBug:
		beadType = class
	case ClassDebt:
		labels = append(labels, ClassDebt)
	}
	return labels, beadType
}

// verifySection is one close's block, whether it stands alone or is one of N
// in a batch: the marker line that identifies it, then what was claimed, by
// whom, against which "done when", and the commits that claim to do it. The
// title is %q-fenced like the work prompt's (rangerhq-pnp) — a bead's own
// text is data, wherever it is quoted — and every other field bd hands back
// is flattened to one line, because in a batched description a newline is a
// forged marker (verifyOneLine).
func (a *App) verifySection(dir string, is BdIssue, closer string) string {
	var b strings.Builder
	// The id is flattened like every other field, and it is the one whose
	// flattening COSTS something: a poisoned id no longer parses as its own
	// marker, so that close is re-filed rather than adopted. That is the
	// trade this whole file makes — a duplicate is loud and recoverable, a
	// suppressed handoff is neither (ranger-base-j8qk).
	fmt.Fprintf(&b, "%s%s%stitle, quoted as data: %q).\n\n", verifyMarkerPrefix, verifyOneLine(is.ID), verifyMarkerAfter, is.Title)
	if closer != "" {
		fmt.Fprintf(&b, "- closer: %s\n", verifyOneLine(closer))
	}
	if r := verifyOneLine(is.CloseReason); r != "" {
		fmt.Fprintf(&b, "- close_reason: %s\n", r)
	}
	if is.ClosedAt != nil && !is.ClosedAt.IsZero() {
		fmt.Fprintf(&b, "- closed: %s\n", is.ClosedAt.Format(time.RFC3339))
	}
	if len(is.Labels) > 0 {
		fmt.Fprintf(&b, "- labels: %s\n", verifyOneLine(strings.Join(is.Labels, ", ")))
	}
	b.WriteString(verifyAcceptanceLine(is))
	if lines, _ := gitCommitsFor(dir, is.ID); len(lines) > 0 {
		fmt.Fprintf(&b, "- commits naming %s (git log --grep; a commit may merely CITE the bead):\n", is.ID)
		for _, l := range lines {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	// Beside the trail, never instead of it: the trail says what mentions
	// the bead, this says whether the session cut for it got its work home
	// (ranger-base-hl0sp). Silence here means no branch record named this
	// bead, which is not a claim either way — gitBranchLandingFor's comment
	// says why that is the honest shape.
	if lines := gitBranchLandingFor(dir, is.ID); len(lines) > 0 {
		fmt.Fprintf(&b, "- session branches cut for %s (branch.<b>.posseBead):\n", is.ID)
		for _, l := range lines {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	return b.String()
}

// verifyOneLine flattens a field bd hands back to a single line. A batched
// description carries one marker per close and finds them BY LINE, so a
// newline inside a close_reason is an injection point: it could forge a
// marker for a close that has no verify bead and suppress that close's
// handoff forever, which is the one failure this whole file exists to
// prevent. Nothing legitimate in these fields is multi-line.
var verifyLineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

func verifyOneLine(s string) string { return strings.TrimSpace(verifyLineBreaks.Replace(s)) }

// verifyAcceptanceLine is the section's checklist line, and ADR 0006 §4
// (simplified 2026-09-05) makes it a POINTER rather than a criterion: the
// closed bead's own description is the acceptance, the verifier reads it
// from the bead, and where the bead states none the section says so and
// names the limit instead of supplying one.
//
// It replaces the row this file used to mine out of the CLOSER's PID: its
// `## Intents` "done when" cell, picked by word-matching the bead's labels
// and issue type, with that PID's whole table quoted when nothing matched.
// (The deleted helpers are named in ADR 0006 §4 and in git history; this
// package may not spell them again — acceptanceexplicit_qa_test.go.)
//
// MEASURED 2026-09-06 over one shop's whole queue (2229 beads, 2100 closed,
// 1048 of them carrying a `verify_labels:` label, closed 2026-08-12 ..
// 2026-09-06):
//
//	203 verify beads filed by this rule over that window carried the row
//	ZERO times, and no close of one cites it. (The reader was shown able
//	to see it: a section rendered by this file for a matched close
//	contains the line it counted.)
//
//	Re-run against the matcher as it stood, 379 of the 1048 closes would have
//	earned a row — and those 379 rows are TWO distinct sentences, 359 of
//	them the identical "root cause named in the commit, regression test
//	added, suite green". The cell is a property of the CLOSER's PID and
//	the bead's TYPE, never of the task, so it cannot carry a criterion
//	this bead's acceptance does not already own: of those 379 sources,
//	279 descriptions already say "test", 153 "cause", 140 "suite"/"green".
//
// So the row supplied no criterion absent from the task's own acceptance —
// it supplied one of two constants — and it cost the one injection surface
// this file most cares about: operator-written PID prose interpolated into
// a description whose per-close markers are found BY LINE. Nothing but the
// bead's own id reaches this line now.
func verifyAcceptanceLine(is BdIssue) string {
	id := verifyOneLine(is.ID)
	if strings.TrimSpace(is.Description) == "" {
		// The ADR's "say it is missing": an absent description states no
		// completion criteria at all, and the verifier is told to REPORT
		// that rather than fall back on what a bead of this shape usually
		// means. This is a presence check on the bead's own field, not an
		// acceptance parser — §4 forbids adding one.
		return "- acceptance: MISSING — " + id + " carries no description, so it states no completion criteria." +
			" Verify what the commits and comments below actually show, and record that gap as this verification's limit; do not supply criteria of your own.\n"
	}
	return "- acceptance: the closed bead's own description is this section's checklist — read it there (`bd show " + id + "`)." +
		" If it states no completion criteria, say so as this verification's limit rather than inventing them.\n"
}

// gitCommitsFor is the commit trail the ADR asks for, and the structured
// half of the rejection exemption above. Best effort for the trail — no
// repo, no git, no matching message, no line; it never fails the filing,
// because a verify bead without commits is still worth having.
//
// The error is returned because the exemption cannot treat "git said no"
// and "git could not say" alike: an unanswerable repo (not a checkout, no
// commits yet, no git) would otherwise exempt every close whose reason
// happens to hold a reject word. A repo with no matching commit is exit 0
// and empty output, not an error, so the two really are distinguishable.
func gitCommitsFor(dir, id string) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	out, err := protocolCmd(exec.Command("git", "-C", dir, "log", "--grep", id,
		"--format=%h %s", "-n", strconv.Itoa(verifyCommitLimit))).Output()
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// gitBranchLandingFor answers the question `git log --grep` cannot ask, and
// whose absence let a stranded close read as a landed one (ranger-base-hl0sp,
// found by the sweep on ranger-base-2dzsm). --grep names every commit in the
// checkout's ancestry whose MESSAGE mentions the id, and there a commit that
// merely CITES a bead is indistinguishable from the commit that shipped it.
// The instance: ranger-base-5jdzh's verify bead listed d309e2b — which is
// ranger-base-wd4be's commit, whose message happens to name 5jdzh — while
// 5jdzh's own work sat on a session branch whose tip (411e54f) was not an
// ancestor of main, and no line in the bead said so.
//
// The record that does know is branch.<b>.posseBead (beadKey), written at
// every launch into the tree and outliving the session meta by design. So:
// find the branches cut FOR this bead and say, for each, whether its tip has
// reached the base it was cut from (baseKey, with the same fallback baseOf
// gives a branch cut before that stamp existed).
//
// A MISSING record is not evidence of a landing, and nothing here may be
// read as one: `git branch -d` takes the branch's config with it, so a
// session that landed and was tidied up leaves nothing behind, exactly like
// a branch cut before the stamp. That is why this returns lines only for the
// records it FOUND and states a strand positively ("has NOT reached main")
// rather than inferring one from an empty list — the empty case is the
// silence gitCommitsFor already gives, not a verdict.
//
// What it writes is a READING TAKEN AT FILING TIME and the description says
// so by naming the record, not a status the bead then owns: a strand that is
// re-landed an hour later leaves this line stale in a stored document, the
// same way the commit trail goes stale. That is the right trade here — the
// verifier's first act is to re-run the two commands the line names, and a
// line that was true when written is what sends them to look. A live status
// would mean the filer re-writing descriptions after the fact, which is the
// one thing the dedupe-of-record marker must be able to rely on not happening.
//
// Best effort in the same shape as gitCommitsFor: no repo, no git, no record
// for this id, no lines. It never fails the filing.
func gitBranchLandingFor(dir, id string) []string {
	if dir == "" {
		dir = "."
	}
	// The whole branch section, filtered in Go rather than by a tighter
	// regexp, because git config is only half case-preserving and the half
	// that matters here is the wrong one: it lowercases the section and
	// VARIABLE names it prints (the subsection — the branch — keeps its
	// case), so the key comes back as `branch.<b>.possebead` however it was
	// written. Matching that in the pattern would mean encoding git's
	// case-folding rules for `--get-regexp` in a string literal; EqualFold
	// below says the same thing where it can be read.
	out, err := git(dir, "config", "--get-regexp", `^branch\.`)
	if err != nil {
		return nil
	}
	// The spelling comes from beadKey itself (beadKey("") is
	// "branch.<>.posseBead"), so renaming the key cannot leave this reader
	// silently matching nothing.
	const branchPrefix = "branch."
	beadSuffix := strings.TrimPrefix(beadKey(""), branchPrefix)
	var branches []string
	for _, l := range strings.Split(out, "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(l), " ")
		if !ok || strings.TrimSpace(val) != id {
			continue
		}
		if !strings.HasPrefix(key, branchPrefix) || len(key) <= len(branchPrefix)+len(beadSuffix) {
			continue
		}
		if !strings.EqualFold(key[len(key)-len(beadSuffix):], beadSuffix) {
			continue
		}
		branches = append(branches, key[len(branchPrefix):len(key)-len(beadSuffix)])
	}
	// git prints config in file order; the description is a stored document
	// and a re-file must not shuffle its lines.
	sort.Strings(branches)
	fallback := repoBranch(dir)
	var lines []string
	for _, b := range branches {
		if len(lines) == verifyBranchLimit {
			lines = append(lines, fmt.Sprintf("(%d more branch record(s) name this bead)", len(branches)-len(lines)))
			break
		}
		tip, err := git(dir, "rev-parse", "--short", "refs/heads/"+b)
		if err != nil {
			// The record outlives the branch only until someone prunes the
			// config by hand; say which, rather than dropping the row.
			lines = append(lines, fmt.Sprintf("%s: no such branch here (the record outlived it)", verifyOneLine(b)))
			continue
		}
		base := baseOf(dir, b, fallback)
		switch {
		case base == "":
			lines = append(lines, fmt.Sprintf("%s tip %s — no base recorded and the checkout is detached, so nothing here can say where it lands",
				verifyOneLine(b), verifyOneLine(tip)))
		case b == base:
			// The bead was worked in the shared checkout, on the base itself
			// (a crew session, or dispatch onto a detached HEAD). There is no
			// merge-back to be waiting on and no strand to report.
			lines = append(lines, fmt.Sprintf("%s tip %s IS %s — worked in the checkout, no merge-back",
				verifyOneLine(b), verifyOneLine(tip), verifyOneLine(base)))
		case !branchExists(dir, base):
			lines = append(lines, fmt.Sprintf("%s tip %s — %s is not a branch here, so nothing can say whether it reached",
				verifyOneLine(b), verifyOneLine(tip), verifyOneLine(base)))
		default:
			reached := "has NOT reached"
			if _, err := git(dir, "merge-base", "--is-ancestor", "refs/heads/"+b, "refs/heads/"+base); err == nil {
				reached = "has reached"
			}
			lines = append(lines, fmt.Sprintf("%s tip %s %s %s", verifyOneLine(b), verifyOneLine(tip), reached, verifyOneLine(base)))
		}
	}
	return lines
}

func hasAnyLabel(labels, want []string) bool {
	for _, w := range want {
		if hasLabel(labels, w) {
			return true
		}
	}
	return false
}
