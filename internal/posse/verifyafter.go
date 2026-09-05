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
// and the closer PID's "done when" row where one matches — otherwise that
// PID's whole `## Intents` table, marked unmatched (ADR 0006 §3, amended
// 2026-09-01: a bead carries no intent, so the row is a word match, and bd's
// default type `task` names none) — then comment
// `verify filed: <qid>` on the closed bead. A closer who filed the verify
// bead first is seen (the qa dependent) and not duplicated.
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

// verifyPolicy is the config one sweep runs under, read once at the head of
// the pass: every repo in it is answered by the same rule, and a config typo
// is named once rather than once per repo.
type verifyPolicy struct {
	Labels []string
	Batch  int
	MaxAge time.Duration
	Now    time.Time
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
// `bd list --all` per repo plus a create per new close, not the fire loop's.
func (a *App) VerifyAfter(bd Bd, dirs []string, out, errw io.Writer) int {
	pol := verifyPolicy{Labels: a.verifyLabels()}
	if len(pol.Labels) == 0 {
		return 0 // config says off
	}
	pol.Batch, pol.MaxAge, pol.Now = a.verifyBatch(errw), a.verifyBatchAge(errw), time.Now()
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

	// Classify every candidate BEFORE grouping. A close that already has its
	// verify bead must not consume a slot in a batch, and a close bd could
	// not answer for must not silently join one — so both are decided per
	// candidate first, and only what is genuinely pending is batched.
	const (
		vaHandled = iota // answered already, or answered by this pass
		vaPending        // needs a verify bead
		vaFailed         // bd could not say, or the bead is unusable
	)
	state := make([]int, len(cands))
	var pending []int
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
			state[i] = vaPending
			pending = append(pending, i)
		}
	}

	// One verify bead per pol.Batch closes, in close order. The trailing
	// partial batch is HELD rather than filed short — filing it would make N
	// a ceiling and reproduce the 1:1 gate under a config key — until its
	// oldest close reaches pol.MaxAge, which is what keeps a shop that goes
	// quiet mid-batch from leaving its last closes unverified forever
	// (DefaultVerifyBatchAge).
	filed := 0
	for off := 0; off < len(pending); off += pol.Batch {
		end := off + pol.Batch
		if end > len(pending) {
			end = len(pending)
			age := pol.Now.Sub(*cands[pending[off]].ClosedAt)
			if age < pol.MaxAge {
				fmt.Fprintf(out, "~ %-14s %d close(s) held for a verify batch of %d (oldest %s)\n",
					AbbrevHome(dir), end-off, pol.Batch, age.Round(time.Minute))
				break
			}
		}
		group := make([]BdIssue, 0, end-off)
		for _, i := range pending[off:end] {
			group = append(group, cands[i])
		}
		qid, err := a.fileVerifyBead(bd, dir, group, already, errw)
		if err != nil {
			fmt.Fprintf(errw, "verify-after: %s: %v\n", verifyIDList(group), err)
			for _, i := range pending[off:end] {
				state[i] = vaFailed
			}
			continue
		}
		filed++
		for _, i := range pending[off:end] {
			state[i] = vaHandled
			fmt.Fprintf(out, "+ %-14s verify filed: %s\n", cands[i].ID, qid)
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

// fileVerifyBead files ONE verify bead for a group of closes — one close at
// `verify_batch: 1`, N at N — and comments its id back on every close it
// covers. The comment is the provenance that survives: the `discovered-from`
// edges are the write that may not land at all (verifyMarkerPrefix), and
// with N of them the walk that fails to terminate has N chances to start.
func (a *App) fileVerifyBead(bd Bd, dir string, group []BdIssue, already map[string]bool, errw io.Writer) (string, error) {
	title, desc := verifyGroupTitle(group), a.verifyGroupDescription(dir, group)
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

// ─── the findings bundle and the class (ADR 0006 §1/§3, amended 2026-09-02) ──

// verifyTrailer is the closing instruction on every verify bead this rule
// mints, single or batched. The ruling it spells: a verify close files ONE
// findings bead carrying every finding, labelled with the close's own lane
// and `debt`; only a LIVE defect in money, the constitution, or dispatch
// correctness earns a bead of its own, and the bundle then names it by id.
// It is one sentence for a batch as for a single close — the bundle is per
// VERIFY close, not per close verified, which is the amplification the
// ruling cut (a day that filed 111 beads against 86 closes, with QA's
// per-finding filing the largest line).
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
	return "For any close that does NOT verify, file ONE findings bead `-l " + lane +
		" -l debt --deps discovered-from:<this bead's id>` — one line per finding: file:line · what fails · the bead it escaped from · the repro or failing test. " +
		"A LIVE money / constitution / dispatch-correctness defect alone gets its own `-t bug` bead at P1/P2 with the domain in the title, named in the bundle by id (ADR 0006 §1). " +
		"Then close this one `escape` (ADR 0006 §2). No findings, no bead. The closed bead is never reopened by a persona — that is the operator's call.\n"
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
	if intent, done := a.closerDoneWhen(closer, is); done != "" {
		fmt.Fprintf(&b, "- done when (%s · %s): %s\n", verifyOneLine(closer), verifyOneLine(intent), verifyOneLine(done))
	} else if rows := a.closerIntentRows(closer); len(rows) > 0 {
		// No match, but the closer's table exists: quote the whole thing
		// rather than nothing, so §2's promise — the verifier's checklist
		// without opening the PID — holds for a close whose type names no
		// intent too (ADR 0006 §3, amended 2026-09-01; `task` is bd's
		// default type and 0 of 27 task closes on 2026-09-01 carried the
		// row against 21 of 21 bug closes). It interprets nothing: the
		// table is quoted in table order, not chosen from.
		fmt.Fprintf(&b, "- done when (%s · unmatched; every intent):\n", verifyOneLine(closer))
		for _, r := range rows {
			fmt.Fprintf(&b, "    %s: %s\n", verifyOneLine(r.Intent), verifyOneLine(r.DoneWhen))
		}
	}
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

// closerDoneWhen is best effort in both directions: the closer may not be a
// persona on this box, and its PID may have no intent matching the bead.
//
// The match candidates are the bead's labels PLUS its bd issue type
// (`bug`, `feature`, ...). Labels alone are structurally unreachable for
// most closes: a persona's `verify_labels` default is its own catch-all
// routing label (`code`, `devops`, ...), which by design names no specific
// intent, and a production close rarely carries a second, more specific
// label alongside it (ranger-base-wogo — 0/30 live verify beads carried
// this row). The issue type is set on every bead and is exactly the kind
// of word `intentMatchesLabel` already expects (`bug` -> `fix-bugs`), so it
// recovers the match without inventing a label vocabulary.
func (a *App) closerDoneWhen(closer string, is BdIssue) (intent, doneWhen string) {
	if closer == "" {
		return "", ""
	}
	ag, err := a.LoadAgent(closer)
	if err != nil {
		return "", ""
	}
	cands := is.Labels
	if is.IssueType != "" {
		cands = append(append([]string{}, is.Labels...), is.IssueType)
	}
	return ag.IntentDoneWhen(cands)
}

// closerIntentRows is the other half of best effort: the whole `## Intents`
// table of a closer who is a persona on this box, for the section to quote
// when closerDoneWhen matched nothing. Same two silences as closerDoneWhen —
// a closer who is not a persona here, and a PID whose table is missing or
// empty, are both zero rows, and zero rows is no line at all.
//
// The rows are rendered INDENTED, and that is load-bearing, not cosmetic:
// verifySourceID matches verifyMarkerPrefix at the start of a line with no
// trimming, so a line beginning with spaces can never be read as a per-close
// marker however a PID's cells are written. Keep the indent. Cells and slugs
// still pass through verifyOneLine like every other field, so one cell can
// never become two lines either.
func (a *App) closerIntentRows(closer string) []IntentRow {
	if closer == "" {
		return nil
	}
	ag, err := a.LoadAgent(closer)
	if err != nil {
		return nil
	}
	return ag.IntentRows()
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
	out, err := exec.Command("git", "-C", dir, "log", "--grep", id,
		"--format=%h %s", "-n", strconv.Itoa(verifyCommitLimit)).Output()
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
