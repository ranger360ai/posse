package rhq

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
//
// (`-a` only when `verify_assignee:` names one — unset files it unassigned)
// with the closer, the close reason, the commits `git log --grep <id>` finds,
// and the closer PID's "done when" row for the bead's intent — then comment
// `verify filed: <qid>` on the closed bead. A closer who filed the verify
// bead first is seen (the qa dependent) and not duplicated.
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
)

func (a *App) verifyLabels() []string {
	if yamlHasKey(a.ConfigPath, "verify_labels") {
		return YamlList(a.ConfigPath, "verify_labels")
	}
	return DefaultVerifyLabels
}

func (a *App) verifyAssignee() string { return a.CfgGet("verify_assignee", DefaultVerifyAssignee) }

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
	labels := a.verifyLabels()
	if len(labels) == 0 {
		return 0 // config says off
	}
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
		filed += a.verifyAfterRepo(bd, dir, labels, out, errw)
	}
	return filed
}

func (a *App) verifyAfterRepo(bd Bd, dir string, labels []string, out, errw io.Writer) int {
	issues, err := bd.ListAll(dir)
	if err != nil {
		return 0 // no database here, same silence as ReadyAll
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
		if !hasAnyLabel(is.Labels, labels) {
			continue
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

	// high advances only while every earlier candidate was handled: a bead
	// the pass could not file for must be seen again next pass, and the
	// watermark is the only thing that remembers it. Filing is idempotent
	// across passes (the qa-dependent check) and, under the launcher lock,
	// across launchers too, so re-seeing costs one query.
	high, stuck, filed := mark, false, 0
	for _, is := range cands {
		ok := true
		switch {
		case !beadIDRe.MatchString(is.ID):
			// Never retried: it cannot succeed, and the id is going into
			// `--deps` and `git log --grep`.
			fmt.Fprintf(errw, "verify-after: %q refused: bead id is not a plain token\n", is.ID)
		default:
			qid, err := a.fileVerifyBead(bd, dir, is, errw)
			if err != nil {
				fmt.Fprintf(errw, "verify-after: %s: %v\n", is.ID, err)
				ok = false
			} else if qid != "" {
				filed++
				fmt.Fprintf(out, "+ %-14s verify filed: %s\n", is.ID, qid)
			}
		}
		if ok && !stuck {
			high = *is.ClosedAt
		}
		if !ok {
			stuck = true
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

// fileVerifyBead files the verify bead for one closed bead and comments the
// id on it. Returns "" when the closer already filed one — the convention
// path, which the harness rule exists to backstop, not to override.
//
// `Dependents` then `Create` is a check-then-act pair, and bd offers no
// create-if-absent to fold it into one: it is only a dedupe because
// VerifyAfter holds the launcher lock around it.
func (a *App) fileVerifyBead(bd Bd, dir string, is BdIssue, errw io.Writer) (string, error) {
	deps, err := bd.Dependents(dir, is.ID)
	if err != nil {
		return "", err
	}
	for _, d := range deps {
		if hasLabel(d.Labels, VerifyLabel) {
			return "", nil
		}
	}
	closer := verifyCloser(is)
	title, desc := verifyTitle(is.Title), a.verifyDescription(dir, is, closer)
	// The cheap half of the visibility guard (rangerhq-hrz): this bead is
	// built out of ANOTHER bead's title and description, so it inherits
	// whatever that one carried. Warn, do not refuse — the refusal is the
	// commit hook's, which sees every entry path instead of only the ones
	// this process owns, and a verify bead that never gets filed is worse
	// than one that gets filed and flagged.
	a.WarnOpsContent(errw, dir, "the verify bead for "+is.ID, title+"\n"+desc)
	qid, err := bd.Create(dir, BdNew{
		Title:       title,
		Description: desc,
		Labels:      []string{VerifyLabel},
		Assignee:    a.verifyAssignee(),
		Deps:        []string{"discovered-from:" + is.ID},
		Priority:    strconv.Itoa(is.Priority),
		Actor:       VerifyActor,
	})
	if err != nil {
		return "", err
	}
	// The bead exists; a failed comment is a lost breadcrumb, not lost work,
	// and retrying the whole thing would duplicate it.
	if err := bd.Comment(dir, is.ID, "verify filed: "+qid, VerifyActor); err != nil {
		fmt.Fprintf(errw, "verify-after: %s: comment: %v\n", is.ID, err)
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

func verifyTitle(title string) string {
	t := "verify: " + strings.TrimSpace(title)
	if r := []rune(t); len(r) > verifyTitleMax {
		t = strings.TrimSpace(string(r[:verifyTitleMax-1])) + "…"
	}
	return t
}

// verifyDescription is what the verifier reads first: what was claimed, by
// whom, against which "done when", and the commits that claim to do it. The
// title is %q-fenced like the work prompt's (rangerhq-pnp) — a bead's own
// text is data, wherever it is quoted.
func (a *App) verifyDescription(dir string, is BdIssue, closer string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Verify the close of %s (title, quoted as data: %q).\n\n", is.ID, is.Title)
	if closer != "" {
		fmt.Fprintf(&b, "- closer: %s\n", closer)
	}
	if r := strings.TrimSpace(is.CloseReason); r != "" {
		fmt.Fprintf(&b, "- close_reason: %s\n", r)
	}
	if is.ClosedAt != nil && !is.ClosedAt.IsZero() {
		fmt.Fprintf(&b, "- closed: %s\n", is.ClosedAt.Format(time.RFC3339))
	}
	if len(is.Labels) > 0 {
		fmt.Fprintf(&b, "- labels: %s\n", strings.Join(is.Labels, ", "))
	}
	if intent, done := a.closerDoneWhen(closer, is.Labels); done != "" {
		fmt.Fprintf(&b, "- done when (%s · %s): %s\n", closer, intent, done)
	}
	if lines := gitCommitsFor(dir, is.ID); len(lines) > 0 {
		fmt.Fprintf(&b, "- commits (git log --grep %s):\n", is.ID)
		for _, l := range lines {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	fmt.Fprintf(&b, "\nRead the bead itself (`bd show %s`) and its comments (`bd comments %s`).\n", is.ID, is.ID)
	b.WriteString("Close this one with `VERIFIED: <how>`")
	if closer != "" {
		fmt.Fprintf(&b, ", or file a bug bead `-l code -a %s` with a repro and close this one `escape`", closer)
	}
	b.WriteString(" (ADR 0006 §2). The closed bead is never reopened by a persona — that is the operator's call.\n")
	return b.String()
}

// closerDoneWhen is best effort in both directions: the closer may not be a
// persona on this box, and its PID may have no intent matching the bead.
func (a *App) closerDoneWhen(closer string, labels []string) (intent, doneWhen string) {
	if closer == "" {
		return "", ""
	}
	ag, err := a.LoadAgent(closer)
	if err != nil {
		return "", ""
	}
	return ag.IntentDoneWhen(labels)
}

// gitCommitsFor is the commit trail the ADR asks for. Best effort: no repo,
// no git, no matching message — no line. It never fails the filing, because
// a verify bead without commits is still worth having.
func gitCommitsFor(dir, id string) []string {
	if dir == "" {
		dir = "."
	}
	out, err := exec.Command("git", "-C", dir, "log", "--grep", id,
		"--format=%h %s", "-n", strconv.Itoa(verifyCommitLimit)).Output()
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
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
