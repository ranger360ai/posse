package posse

// ci-watch — a red ci.yml run on `main` files ONE bead, and says on that
// bead when main is green again (ranger-base-x9e34).
//
// THE INCIDENT. ci.yml went red on main at 2026-08-30T01:53Z (8d50fed5) and
// stayed red: 191 consecutive failed runs over five days and ~120 commits,
// last green 0c0607b0 at 01:23Z. Nobody noticed, because nothing anywhere a
// person or a seat looks says so — `gh run list` is a command someone has to
// REMEMBER, which is the same argument ci.yml's own header makes about
// `make test-linux`.
//
// WHAT A RED GATE COSTS is not the reds, it is the ATTRIBUTION. Merge-back
// fast-forwards a bead branch onto main and opens no pull request, so ci.yml
// is the ONLY gate a commit on main ever passes and nothing runs before it
// lands. During those five days the standing reds had nothing to do with the
// commits they were attached to, so a genuine break — internal/posse not
// building at all, twice, for over an hour each — was indistinguishable from
// the noise.
//
// THE SHAPE, from the design bead (ranger-base-90y3c): a dispatch-loop check
// that files a bead on a red main run and closes it when main is green
// again. A GitHub notification setting was rejected there and is not
// reconsidered here — it is not versioned and it reaches one account.
//
// ONE BEAD, NOT ONE PER PUSH. This is the invariant the whole thing lives or
// dies on: a mechanism that files a bead per red push during the next
// five-day red is worse than the silence it replaces. Two things enforce it
// and they are independent:
//
//   - The dedupe is an OPEN BEAD carrying CIRedLabel whose description holds
//     this gate's marker line (ciMarker). It is a store read, not process
//     state, so a launcher restart and a second launcher see the same one
//     bead — and it is the marker rather than the label alone because every
//     repo here redirects `.beads` to one queue, so one gate's bead is in
//     the listing another gate's dedupe reads.
//   - A `cancelled` run is NOT A VERDICT (ciVerdict). Measured over this
//     workflow's whole 300-run history on 2026-09-04 — 262 completed runs,
//     242 of them verdict-bearing, 20 cancelled — this mechanism would have
//     filed 7 beads in 6.6 days. Counting cancelled as red files 16;
//     counting it as green files 13. A run GitHub stopped says nothing
//     about main, and saying nothing is the only honest reading of it.
//
// 7 beads over 6.6 days against 196 red runs is the blast radius, measured
// rather than argued. The shortest episode would have been open 15 minutes
// (20:29:09Z red, 20:44:10Z green on 2026-08-29), so no refile cooldown is
// carried: a red that follows a green is a NEW red, and the store already
// shows the previous episode closed beside it.
//
// WHERE IT PRINTS. The pass says something only when it ACTS — on the pass
// that files and the pass that closes. A condition that recurs must not be
// re-announced every pass; that is how a visible line becomes an invisible
// one (launcherlag.go's rule, and watch.go's own backoff). The BEAD is the
// standing surface, and it is the surface the bead's DONE WHEN asks for:
// "reaches the crew without anyone remembering to look".
//
// NOT IN `posse status`, and that is a measurement rather than taste. One
// `gh run list` costs 2.8-4.2s here (three samples, 2026-09-04) against a
// `posse status` that costs 3.87s in total — so the reading would roughly
// double the latency of the one command an operator waits on, to answer a
// question the filed bead already carries. The launcher-lag reading takes
// the opposite decision for the opposite reason: it is a local
// `git rev-list --count` and it has no bead.
//
// TWO KINDS OF ABSTENTION, and the difference is the whole of what reaches
// stderr. A repo that has NO GATE — not a git checkout, no such workflow
// file, no github.com origin — is silent: nothing is hidden, there is no
// all-clear to mistake, and the fact is true on every pass forever. A repo
// that HAS a gate this pass could not READ — no gh, an unauthenticated gh, a
// network that did not answer, no verdict-bearing run — says so once per
// process, because that one does render as an all-clear if it renders as
// silence. The suite taught this file the difference the hard way: the first
// cut said both, and 22 dispatch and plan-guard pins went red asserting that
// a clean pass writes nothing to stderr (CIState.NoGate).
//
// WHAT RIDES INTO THE BEAD: shas, run URLs, conclusions and timestamps.
// Deliberately NOT the run's displayTitle, which is the commit message —
// nothing this mechanism writes into a bead comes from anywhere but
// GitHub's own run metadata for a repo whose CI is already public, so
// guardrail 4 is satisfied by construction rather than by a filter.
//
// WHILE IT STAYS RED the bead's number is re-said when it has DOUBLED — 1,
// 2, 4, 8, 16 — so the incident's 191 failures earn eight comments and not
// 191 (ciDrumbeat). The "last said" number is read back off the bead rather
// than kept in this process, because a launcher restart is the ordinary case
// here: the incident outlived several.
//
// IT CLOSES ONE BEAD AND ONLY ONE: the bead IT FILED that NO SESSION EVER
// CLAIMED. That is the single exception ADR 0013 §4 admits, and it is a
// RULING (ranger-base-8fr2j, 2026-09-05) rather than this file's reading of
// the section. §4 rejects "harness closes the bead on the agent's behalf" in
// as many words — "resume-until-record is the harness's job; `bd close` is
// the persona's" — and the harm it names is a record graded by the thing
// that writes it. A bead nobody was ever dispatched onto grades nobody, so
// closing it hides no defect and replaces no human in a loop: there is no
// agent here whose behalf this could be on.
//
// The predicate is ciHolder, and it is read off the bead rather than
// remembered: status still `open` AND no assignee. Anything else — a seat
// holding it in_progress, a bead the operator routed by assignee, a blocked
// or deferred one — is somebody's, and stays somebody's; the green half for
// those is the comment that shipped first (ranger-base-x9e34) saying CLOSE
// IT and why the harness did not. The guard errs toward NOT closing on every
// shape it does not recognise, which is the direction that costs a minute
// rather than a record.
//
// The one shape the row cannot show is a bead that WAS claimed and was put
// back (Bd.Unclaim: status open, assignee cleared). That is rangerhq-81d's
// case — dispatch claimed on a persona's behalf and the prompt never reached
// the agent — so no session ever worked it either, and the observable and
// the ruling agree there rather than merely coinciding.
//
// absencerules_qa_test.go's TestNoBdCloseVerbReachableFromDispatch is what
// makes this narrow rather than merely stated: the first cut of this file
// closed on green and that pin caught it, when there was no register to add
// a row to. There is one now — arm 1's caller register and arm 2's
// reachability register, one row each, naming ciClear and why it is not the
// agent's-behalf case. A second harness close has to be written down before
// it compiles green.
//
// A cleared bead that ci-watch did NOT close must not suppress the NEXT red,
// so ciAlreadyCleared reads that comment back and the dedupe steps over it.
// One bead per episode still holds; what changes for a held bead is that it
// outlives its episode.
//
// A READING AND THREE WRITES, and no more: it files, it comments, and it
// closes the bead nobody claimed. It never reruns a workflow, never pushes,
// never touches the gate itself, and never closes a bead a seat holds.
// Whoever fixes CI is a dispatched seat with a bead, which is the point.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultCIWorkflow is config `ci_workflow:` when the key is absent —
	// the workflow FILE NAME as `gh run list --workflow` takes it. A present
	// but empty key turns ci-watch off, the way an empty `verify_labels:`
	// turns verify-after off.
	DefaultCIWorkflow = "ci.yml"

	// CIRedLabel is the label every bead this files carries, and therefore
	// the dedupe query. It is not a routing label: nothing dispatches on it,
	// and it exists so one `bd list --label-any` answers "is there already
	// an open one" without reading the whole store.
	CIRedLabel = "ci-red"

	// CIRedLane is the label that ROUTES the bead to a persona (agents.go
	// `labels:`). It is `devops` because that is the harness's own shipped
	// name for this lane — DefaultVerifyLabels already carries it — so a
	// bead about a broken build lands where build breakage lands, with no
	// new config key and no compiled-in persona name.
	CIRedLane = "devops"

	// ciScanLimit is how many runs one reading pulls. 100 is `gh`'s single
	// page, so it is the largest window that costs one API call — and the
	// streak it can measure (the incident's own was 191) is reported as a
	// floor past that rather than as a wrong number.
	ciScanLimit = 100

	// ciReadTimeout bounds the child. A pass must not hang on a network,
	// and an unreachable GitHub is an abstention like any other.
	ciReadTimeout = 30 * time.Second

	// ciCauseScanCap bounds how many of a cleared streak's own red runs
	// ciCauses will fork a `gh run view --log-failed` for — a second,
	// independent cost from ciScanLimit's one `run list` call, one child
	// per run rather than one for the whole window. ci-watch's own reason
	// to exist is that nothing bounded a red streak before it (the incident
	// this file's header names ran 191 runs with nobody looking), and since
	// it files a bead on the FIRST red, the ordinary streak it clears is
	// this bead's own findings 1 and 2 — five runs and one. 10 covers those
	// with room, and a streak long enough to hit it says so rather than
	// pretending it read the rest.
	ciCauseScanCap = 10

	// ciCauseReadTimeout is per run, not per clear: ciCauseScanCap runs at
	// this bound is the most one clear can cost, and a clear is not read on
	// the hot path drumbeat and dedupe are.
	ciCauseReadTimeout = 10 * time.Second

	// ciMarkerPrefix opens every ci-red bead's description and is the dedupe
	// of record: which repo, which workflow and which branch this bead is
	// about, written by bd in the same breath as the issue. The label alone
	// is not enough — an instance with two `beads:` repos would otherwise
	// let one repo's red suppress the other's.
	ciMarkerPrefix = "ci-red-gate: "

	// ciClearedPrefix opens the comment ci-watch writes when the gate goes
	// green, and is read back by ciAlreadyCleared. It carries the whole of
	// the green half's state for a bead the harness may NOT close — one a
	// seat holds (ciHolder) stays open, so without this the cleared bead
	// would go on matching the dedupe and the NEXT red would never be filed.
	// On the bead it does close, the same comment is the close comment: it
	// is written first and names the run that cleared the gate, so a closed
	// ci-red bead says which run answered it.
	ciClearedPrefix = "ci-red cleared: "

	// ciStreakPrefix is the machine-readable half of the streak, written
	// into the description at filing and into every drumbeat comment after
	// it. It is parsed back out of those same two places, which is why this
	// needs no process state: a restarted launcher reads the number off the
	// bead it already filed.
	ciStreakPrefix = "ci-red streak: "
)

// CIQuery is everything a reading needs that is not the answer. GhBin is a
// field rather than an env read at the call site so a pin can drive the real
// argv path with a fake `gh` without mutating an environment this package's
// parallel suite shares.
type CIQuery struct {
	Dir      string // any checkout of the repo; resolved to its MAIN checkout
	Workflow string // the workflow file name, e.g. "ci.yml"
	Branch   string // "" = origin/HEAD's branch, then "main"
	GhBin    string // "" = $RHQ_GH_BIN, then "gh"
}

// CIRun is one workflow run, reduced to the four facts a bead needs. No
// commit message: see the file header on what rides into a bead.
type CIRun struct {
	Sha        string    `json:"headSha"`
	URL        string    `json:"url"`
	Conclusion string    `json:"conclusion"`
	Status     string    `json:"status"`
	Created    time.Time `json:"createdAt"`
}

// Short is the sha as a person reads it back.
func (r CIRun) Short() string {
	if len(r.Sha) > 8 {
		return r.Sha[:8]
	}
	return r.Sha
}

// CIState is the gate's state on one branch of one repo.
//
// Red means anything only when Why is empty. Every field that is a path, a
// slug, a sha or a URL is kept as measured, so a line can name what it
// counted and a reader can re-run it by hand — the whole value of a reading
// that files beads is that the bead it filed is reproducible.
type CIState struct {
	Repo     string // the main checkout the reading was taken in
	Slug     string // owner/name on github.com
	Workflow string
	Branch   string
	Red      bool
	Latest   CIRun // newest verdict-bearing run: the verdict itself
	Since    CIRun // oldest run of the current streak: when it started
	Streak   int   // consecutive runs of the same verdict, Latest included
	Capped   bool  // the streak filled the scan window, so Streak is a floor
	Why      string
	// NoGate separates the two kinds of Why, and the suite is what taught
	// this file the difference (22 reds across the dispatch and plan-guard
	// tests, every one of them a temp dir with no CI in it).
	//
	//   NoGate      there is no gate to read HERE — not a git checkout, no
	//               such workflow file, no github.com origin. A configuration
	//               FACT about the repo, true on every pass forever, and
	//               therefore silent. A launcher whose beads repos have no CI
	//               must not print a line about it, ever.
	//   otherwise   there IS a gate and it could not be READ — no gh, an
	//               unauthenticated gh, a network that did not answer,
	//               unparseable JSON, no verdict-bearing run in the window.
	//               Said once per process, because that one renders as an
	//               all-clear if it renders as silence.
	//
	// Both are still abstentions: neither files, neither closes, and neither
	// is ever read as green.
	NoGate bool
	// PriorRedRuns is the immediately preceding red streak, set only when
	// Red is false: the runs ciClear is about to say nothing went wrong in,
	// unless something reads them (ranger-base-d6zyu finding 3). Free of
	// cost — it is the same scan window ReadCI already fetched for Streak
	// and Since, just not thrown away — and bounded the same way Streak is,
	// by ciScanLimit.
	PriorRedRuns []CIRun
}

// Known is whether Red means anything.
func (s CIState) Known() bool { return s.Why == "" }

// ciVerdict says whether a run is a statement about the branch, and which
// one.
//
// `cancelled` is the case this function exists for. GitHub stops a run for
// reasons that are about the QUEUE and not about the code — a superseding
// push under a shared concurrency group, an operator cancelling, a runner
// going away — and ci.yml has 20 of them in its history from the era when
// pushes to main shared a group. A stopped run has no verdict, so it gets
// none: it is skipped and the next run down answers instead. Measured over
// that history, skipping files 7 beads where counting cancelled as red files
// 16 and counting it as green files 13.
//
// `timed_out` and `startup_failure` ARE red: the gate did not pass, and a
// gate that cannot start is exactly as failed as one that ran and failed.
// Everything else — `neutral`, `skipped`, `action_required`, `stale` — is no
// verdict, on the same rule as cancelled.
func ciVerdict(status, conclusion string) (red, ok bool) {
	if status != "completed" {
		return false, false
	}
	switch conclusion {
	case "success":
		return false, true
	case "failure", "timed_out", "startup_failure":
		return true, true
	default:
		return false, false
	}
}

// ghSlugRe pulls owner/name out of the spellings git remotes come in:
// https://github.com/o/n(.git), git@github.com:o/n(.git),
// ssh://git@github.com/o/n(.git). A remote that is not github.com does not
// match, and a repo that is not on GitHub is an abstention, not an error —
// `gh` is the only client here and there is nothing for it to ask.
var ghSlugRe = regexp.MustCompile(`(?:^|@|//)github\.com[:/]+([^/:]+)/([^/]+?)(?:\.git)?/?$`)

func githubSlug(remote string) (string, bool) {
	m := ghSlugRe.FindStringSubmatch(strings.TrimSpace(remote))
	if m == nil {
		return "", false
	}
	return m[1] + "/" + m[2], true
}

// ciBranch is which branch the gate is read on: the configured one, else the
// branch origin/HEAD names, else "main".
//
// Derived rather than assumed because "main" is a guess about somebody
// else's repo, and a guess here does not fail — it reads a branch with no
// runs on it and abstains, which renders as an all-clear over a repo whose
// default branch is called something else.
func ciBranch(dir, configured string) string {
	if b := strings.TrimSpace(configured); b != "" {
		return b
	}
	if ref, err := git(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if _, b, ok := strings.Cut(strings.TrimSpace(ref), "/"); ok && b != "" {
			return b
		}
	}
	return "main"
}

func ghBin(configured string) string {
	if configured != "" {
		return configured
	}
	if b := os.Getenv("RHQ_GH_BIN"); b != "" {
		return b
	}
	return "gh"
}

// ReadCI takes one reading. Every failure is a Why and never an error: this
// runs inside a dispatch pass, where a repo that cannot be read must cost a
// sentence and not the pass.
//
// The order of the checks is the order of their cost — a local file, a local
// git read, then the network — so the ordinary case of a repo with no CI at
// all never forks `gh`.
func ReadCI(q CIQuery) CIState {
	s := CIState{Workflow: q.Workflow}
	if strings.TrimSpace(q.Workflow) == "" {
		s.Why, s.NoGate = "no workflow configured (ci_workflow: is empty)", true
		return s
	}
	dir, isRepo := MainCheckout(ExpandTilde(q.Dir))
	if !isRepo {
		s.Why, s.NoGate = AbbrevHome(ExpandTilde(q.Dir))+" is not a git checkout", true
		return s
	}
	s.Repo = dir
	// The workflow file has to be IN the checkout. This is the abstention
	// that keeps ci-watch silent in every repo that has no such gate, and it
	// is one stat rather than an API call that answers "no runs" — which is
	// the same answer a typo'd workflow name gives, and must not read the
	// same way.
	wfPath := filepath.Join(dir, ".github", "workflows", q.Workflow)
	if _, err := os.Stat(wfPath); err != nil {
		s.Why, s.NoGate = AbbrevHome(dir)+" has no .github/workflows/"+q.Workflow, true
		return s
	}
	remote, err := git(dir, "remote", "get-url", "origin")
	if err != nil {
		s.Why, s.NoGate = AbbrevHome(dir)+" has no origin remote ("+errText(err)+")", true
		return s
	}
	slug, ok := githubSlug(remote)
	if !ok {
		s.Why, s.NoGate = AbbrevHome(dir)+"'s origin is not on github.com ("+remote+")", true
		return s
	}
	s.Slug = slug
	s.Branch = ciBranch(dir, q.Branch)

	out, err := ghRunList(dir, ghBin(q.GhBin), slug, q.Workflow, s.Branch)
	if err != nil {
		s.Why = "gh could not list " + slug + " runs of " + q.Workflow + " on " + s.Branch + " (" + errText(err) + ")"
		return s
	}
	var runs []CIRun
	if jerr := json.Unmarshal(trimToJSON(out), &runs); jerr != nil {
		s.Why = "gh run list answered something that is not the JSON asked for (" + jerr.Error() + ")"
		return s
	}
	// gh documents no order it will keep, and the whole reading is "the
	// newest verdict", so it is sorted here rather than trusted.
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].Created.After(runs[j].Created) })

	var verdicts []CIRun
	var reds []bool
	for _, r := range runs {
		red, ok := ciVerdict(r.Status, r.Conclusion)
		if !ok {
			continue
		}
		verdicts = append(verdicts, r)
		reds = append(reds, red)
	}
	if len(verdicts) == 0 {
		s.Why = "no completed run of " + q.Workflow + " on " + s.Branch + " in " + slug +
			" carries a verdict (looked at the last " + strconv.Itoa(ciScanLimit) + ")"
		return s
	}
	s.Red, s.Latest = reds[0], verdicts[0]
	s.Streak = 1
	for i := 1; i < len(reds) && reds[i] == reds[0]; i++ {
		s.Streak++
	}
	s.Since = verdicts[s.Streak-1]
	s.Capped = s.Streak == len(verdicts) && len(runs) >= ciScanLimit
	// No `if !s.Red` needed: verdicts[s.Streak], if it exists, is by
	// construction the first run whose verdict DIFFERS from reds[0] — so
	// when the current streak is itself red, reds[s.Streak] is green and
	// this appends nothing, and PriorRedRuns stays what a red reading
	// wants it to be: empty.
	for i := s.Streak; i < len(reds) && reds[i]; i++ {
		s.PriorRedRuns = append(s.PriorRedRuns, verdicts[i])
	}
	return s
}

// ghRunList is the one network call. `--repo` is passed explicitly and is
// not left to gh's cwd resolution: a dispatch pass runs from wherever the
// launcher was started, which is routinely not the repo being read.
func ghRunList(dir, bin, slug, workflow, branch string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ciReadTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "run", "list",
		"--repo", slug,
		"--workflow", workflow,
		"--branch", branch,
		"--limit", strconv.Itoa(ciScanLimit),
		"--json", "conclusion,status,createdAt,headSha,url")
	cmd.Dir = dir
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		if ctx.Err() != nil {
			msg = "timed out after " + ciReadTimeout.String() + ": " + msg
		}
		return nil, Die("%s", msg)
	}
	return []byte(out.String()), nil
}

// trimToJSON drops anything gh printed before the payload, for the reason
// Bd.Create does the same to bd: an advisory note that lands on stdout must
// not cost us the answer.
func trimToJSON(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexAny(s, "[{"); i > 0 {
		s = s[i:]
	}
	return []byte(s)
}

// ciRunIDRe pulls the numeric run id off a run's URL — `gh run view` takes
// an id, not a sha, and CIRun keeps only the four fields a bead needs (the
// file header's own rule), which does not include a separate id field.
var ciRunIDRe = regexp.MustCompile(`/runs/(\d+)`)

func ciRunID(url string) string {
	m := ciRunIDRe.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}

// ciFailRe finds a Go test's own `--- FAIL: Name` wherever `--log-failed`
// put it on the line — gh prefixes every line with the job and step it came
// from, and that prefix is not this file's to depend on the shape of.
var ciFailRe = regexp.MustCompile(`--- FAIL: (\S+)`)

// ciCauses is the real reading behind App.CICauses: for up to
// ciCauseScanCap of runs (newest first, so a capped streak still reports
// its most recent causes), fork `gh run view --log-failed` and collect the
// distinct failing test names it printed, each with how many of the
// (scanned) runs carried it — "TestX (3 runs)" is a record a next reader
// can check against an open bead before this happens again unnoticed
// (ranger-base-d6zyu finding 3: the reason this file's own header names —
// attribution — was being spent rather than banked).
//
// Best-effort and silent about it: a run gh could not be asked about (no
// network, no auth, a run too old for GitHub to still hold logs for) is
// left out rather than guessed at, on the same rule NoGate and Why already
// follow in this file — an attribution that might be wrong is worse than
// none, because the next reader trusts it.
func ciCauses(dir, bin, slug string, runs []CIRun) []string {
	scan := runs
	if len(scan) > ciCauseScanCap {
		scan = scan[:ciCauseScanCap]
	}
	counts := map[string]int{}
	for _, r := range scan {
		id := ciRunID(r.URL)
		if id == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), ciCauseReadTimeout)
		cmd := exec.CommandContext(ctx, bin, "run", "view", id, "--repo", slug, "--log-failed")
		cmd.Dir = dir
		out, err := cmd.Output()
		cancel()
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, m := range ciFailRe.FindAllStringSubmatch(string(out), -1) {
			seen[m[1]] = true
		}
		for name := range seen {
			counts[name]++
		}
	}
	if len(counts) == 0 {
		return nil
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	lines := make([]string, len(names))
	for i, name := range names {
		word := "run"
		if counts[name] != 1 {
			word = "runs"
		}
		lines[i] = fmt.Sprintf("%s (%d %s)", name, counts[name], word)
	}
	if len(runs) > ciCauseScanCap {
		lines = append(lines, fmt.Sprintf("— scanned the newest %d of %d red runs", ciCauseScanCap, len(runs)))
	}
	return lines
}

// ciCausesFor is ciClear's one call site: nothing to ask when the streak it
// is clearing left no red runs behind it (the ordinary case — most clears
// follow a streak of one), App.CICauses when a test set the seam, otherwise
// the real reading.
func (a *App) ciCausesFor(dir string, st CIState) []string {
	if len(st.PriorRedRuns) == 0 {
		return nil
	}
	if a.CICauses != nil {
		return a.CICauses(dir, st.Slug, st.PriorRedRuns)
	}
	return ciCauses(dir, ghBin(""), st.Slug, st.PriorRedRuns)
}

// ciMarker is the dedupe of record — repo, workflow and branch, one line.
func ciMarker(s CIState) string {
	return ciMarkerPrefix + s.Slug + " " + s.Workflow + " " + s.Branch
}

// Title is what the bead is called. It names the gate and the branch and
// nothing that moves: the streak, the sha and the run URL all change while
// the bead is open, and a title that changed under a bead would break every
// human's memory of which bead this is.
func (s CIState) Title() string {
	return "ci is red on " + s.Branch + ": " + s.Workflow + " is failing in " + s.Slug
}

// Description is the filed bead's body: the marker, the streak, both ends of
// the episode, and the two commands that reproduce the reading. The commands
// are there because the first thing a seat asks is "which runs?", and the
// answer must not require re-deriving the query.
func (s CIState) Description() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n\n", ciMarker(s), s.streakLine())
	fmt.Fprintf(&b, "%s is the ONLY gate a commit on %s ever passes: merge-back fast-forwards a bead branch onto %s and opens no pull request, so nothing runs before it lands. While this is red, a red run says nothing about the commit it is attached to — what a red gate costs is not the reds, it is the ATTRIBUTION.\n\n", s.Workflow, s.Branch, s.Branch)
	since := "red since  "
	if s.Capped {
		// Not a start: the oldest run still inside the window (streakLine).
		since = "red at least"
	}
	fmt.Fprintf(&b, "  red now      %s  %s  %s\n", s.Latest.Short(), s.Latest.Created.UTC().Format(time.RFC3339), s.Latest.URL)
	fmt.Fprintf(&b, "  %s %s  %s  %s\n\n", since, s.Since.Short(), s.Since.Created.UTC().Format(time.RFC3339), s.Since.URL)
	fmt.Fprintf(&b, "Reproduce:\n\n  gh run list --repo %s --workflow=%s --branch %s --limit %d --json conclusion,status,createdAt,headSha,url\n  gh run view %s --repo %s --log-failed\n\n",
		s.Slug, s.Workflow, s.Branch, ciScanLimit, s.Latest.Short(), s.Slug)
	b.WriteString("DONE WHEN: " + s.Workflow + " is green on " + s.Branch + " again. Filed by the dispatch pass (ci-watch, ranger-base-x9e34), which will COMMENT here naming the run that clears the gate — including where the fix lands under some other bead. If NOBODY HAS CLAIMED this bead by then it closes it too, which is the one exception ADR 0013 §4 admits (ranger-base-8fr2j): nobody's record is graded by a bead nobody was dispatched onto. Once you claim it the close is yours again, and finding that comment already here means the work is done and closing this bead is the whole of what is left.\n")
	return b.String()
}

// streakLine is the number, said the same way in the description and in
// every drumbeat comment so one parser reads both.
//
// A CAPPED streak says so in both halves, because both are floors. The
// window is one `gh` page and the incident's own streak was 191, so at the
// cap Since is not the first red — it is the oldest run still inside the
// window, and it moves FORWARD in time as more reds pile up behind it.
// Rendering that as "since" would date a five-day red to two days ago,
// which is worse than saying nothing about the start. Uncapped — which is
// every bead filed on the pass that turns the gate red, the ordinary case —
// Since is the real first red and says so plainly.
//
// It is also where the drumbeat runs out, deliberately: at the cap the
// number stops moving, so nothing doubles and nothing more is said. By then
// the bead has said 1, 2, 4, 8, 16, 32, 64 and the cap, and the next thing
// that will change about this gate is the close.
func (s CIState) streakLine() string {
	if s.Capped {
		return fmt.Sprintf("%s%d+ consecutive failed run(s) — every run in the %d-run window this reads is red, so both the count and the start are floors; the oldest still in the window is %s at %s",
			ciStreakPrefix, s.Streak, ciScanLimit, s.Since.Short(), s.Since.Created.UTC().Format(time.RFC3339))
	}
	return fmt.Sprintf("%s%d consecutive failed run(s) since %s at %s",
		ciStreakPrefix, s.Streak, s.Since.Short(), s.Since.Created.UTC().Format(time.RFC3339))
}

// RedLine is what the pass prints when it FILES, and GreenLine when it
// closes. A pass reports what it did; the standing condition is the bead's
// to carry.
func (s CIState) RedLine(id string) string {
	return fmt.Sprintf("ci red · %s %s on %s · %s · filed %s", s.Slug, s.Workflow, s.Branch, s.streakLine(), id)
}

// GreenLine's two shapes are the two outcomes of one pass, and the line says
// which one happened: held is ciHolder's answer, so an empty one means the
// harness closed the bead itself under §4's exception and a non-empty one
// names who it left it to.
func (s CIState) GreenLine(id, held string) string {
	what := "said on " + id + " and CLOSED it — no session ever claimed it (ADR 0013 §4's one exception, ranger-base-8fr2j)"
	if held != "" {
		what = "said on " + id + ", which is now somebody's to close (ADR 0013 §4: " + held + ")"
	}
	return fmt.Sprintf("ci green · %s %s on %s · %s at %s · %s",
		s.Slug, s.Workflow, s.Branch, s.Latest.Short(), s.Latest.Created.UTC().Format(time.RFC3339), what)
}

// ciAbstained keys the once-per-process abstention notice. A reading that
// cannot be taken must not render as silence — silence is what an all-clear
// looks like — but it must also not print every pass for the whole life of a
// loop.
var ciAbstained sync.Map

// ciAbstain says why no reading was taken, ONCE per process, and only for
// the readings that could have been taken (CIState.NoGate).
//
// The NoGate half is silent and that is not a softening. A repo with no CI
// is not a repo whose CI could not be read: nothing is being hidden, there
// is no all-clear to mistake, and the fact is true on every pass forever.
// Saying it made 22 tests red — every dispatch and plan-guard pin that
// asserts a clean pass writes nothing to stderr, run over a temp dir — and
// they were right: a line about the absence of a thing the operator never
// configured is noise in the one stream that must stay signal.
func ciAbstain(errw io.Writer, dir string, st CIState) {
	if errw == nil || st.NoGate {
		return
	}
	if _, said := ciAbstained.LoadOrStore(dir+"\x00"+st.Why, struct{}{}); said {
		return
	}
	fmt.Fprintf(errw, "ci-watch: %s: not read: %s — no bead will be filed or closed for this repo while that holds\n", AbbrevHome(dir), st.Why)
}

// ciWorkflow is config `ci_workflow:`, and it is read through yamlHasKey
// rather than CfgGet for the reason verifyLabels is: CfgGet cannot tell a
// key that is ABSENT from one that is present and empty, and present-and-
// empty is how this shop spells "off" (verify_labels:). Without the
// distinction the off switch silently reads as the default and the
// mechanism runs anyway.
func (a *App) ciWorkflow() string {
	if yamlHasKey(a.ConfigPath, "ci_workflow") {
		return strings.TrimSpace(YamlGet(a.ConfigPath, "ci_workflow"))
	}
	return DefaultCIWorkflow
}

// CIWatch is the whole mechanism, once per dispatch pass: read the gate in
// every configured repo, file one bead where it is red and has none, and
// COMMENT on the one it filed where it is green again — naming the run that
// cleared the gate — and closing it where NO SESSION EVER CLAIMED it, which
// is the one exception ADR 0013 §4 admits (ranger-base-8fr2j). A bead a seat
// holds keeps the close the persona's; this file's header says why at
// length, and absencerules_qa_test.go's
// TestNoBdCloseVerbReachableFromDispatch holds the register that keeps
// ciClear the only Bd close verb the dispatch path reaches.
//
// The PASS and not `posse ready`, which is the other place verify-after runs
// from. Two reasons, and they are the same two that keep this out of `posse
// status`: the reading is a 2.8-4.2s network call and `posse ready` is a
// command an operator waits on, and the answer is not what `ready` is for —
// a listing that pauses for three seconds to file a bead about something
// else is a listing nobody types twice. The loop is the clock this belongs
// on; it is the one already running when a gate goes red at 01:53.
//
// Returns how many beads it filed or closed, which is what a caller counts
// when it wants to know whether the pass acted.
//
// THE READING IS OUTSIDE THE LOCK and the writes are inside it, which is the
// one thing about this function's shape that is not obvious. The launcher
// lock serializes launches; ci-watch needs it because its dedupe is a READ
// followed by a CREATE, and unserialized two launchers file two beads for
// one red (verify-after's rungerhq-th7l reason). But the reading is a `gh`
// child over the network — measured at 2.8-4.2s here on 2026-09-04 — and
// holding the launcher lock across that would park the fire loop and freeze
// the cockpit for those seconds on EVERY pass, including every pass where
// the gate is green and there is nothing to do. So: read first, take the
// lock only if some repo actually has a gate to act on, drop it before
// returning. An instance whose repos all abstain never touches the lock.
func (a *App) CIWatch(bd Bd, dirs []string, out, errw io.Writer) int {
	wf := a.ciWorkflow()
	if wf == "" {
		return 0 // config says off
	}
	read := a.CIRead
	if read == nil {
		read = ReadCI
	}
	branch := a.CfgGet("ci_branch", "")
	type gate struct {
		dir string
		st  CIState
	}
	var gates []gate
	for _, dir := range dirs {
		st := read(CIQuery{Dir: dir, Workflow: wf, Branch: branch})
		if !st.Known() {
			ciAbstain(errw, ExpandTilde(dir), st)
			continue
		}
		gates = append(gates, gate{dir, st})
	}
	if len(gates) == 0 {
		return 0
	}

	lock, err := lockLaunches(a, out)
	if err != nil {
		// One line, and not acting is the safe outcome: nothing here keeps
		// a watermark, so the next pass reads the same gate and reaches the
		// same verdict.
		fmt.Fprintf(errw, "ci-watch: %v\n", err)
		return 0
	}
	defer lock.Release()

	acted := 0
	for _, g := range gates {
		acted += a.ciActOnGate(bd, g.dir, g.st, out, errw)
	}
	return acted
}

// ciActOnGate is the state machine, and it is the whole of it.
//
// The comments are read ONCE, here, because both remaining branches need
// them: the drumbeat reads its own last beat out of them, and the dedupe
// needs to know whether this bead has already been told its gate went green
// (ciAlreadyCleared) — a bead that has is answered, and the next red gets
// its own.
func (a *App) ciActOnGate(bd Bd, dir string, st CIState, out, errw io.Writer) int {
	cands, err := ciOpenBeads(bd, dir, st)
	if err != nil {
		// A store this pass could not read is an unknown queue, not an empty
		// one (BeadsDirs' rule): abstaining is right, and filing on a failed
		// dedupe read is precisely the one-bead-per-push failure.
		fmt.Fprintf(errw, "ci-watch: %s: %v\n", AbbrevHome(ExpandTilde(dir)), err)
		return 0
	}
	// The first candidate, newest first, that has not already been told its
	// gate cleared. Normally that is the first one and this is one bd call.
	var open *BdIssue
	var cs []BdComment
	for i := range cands {
		got, cerr := bd.Comments(dir, cands[i].ID)
		if cerr != nil {
			// Acting blind here is the expensive mistake in both directions:
			// a drumbeat with no last beat says its number every pass, and a
			// dedupe that cannot tell a cleared bead from a live one either
			// files a second bead for one red or never files the next one.
			fmt.Fprintf(errw, "ci-watch: %s: comments: %v\n", cands[i].ID, cerr)
			return 0
		}
		if !ciAlreadyCleared(got) {
			open, cs = &cands[i], got
			break
		}
		if !st.Red {
			// A GREEN gate whose newest bead is already cleared is settled,
			// and every older bead is older than a cleared one. Stopping
			// here keeps the ordinary green pass at one bd call however many
			// episodes are waiting to be closed; the RED pass still walks,
			// because it has to reach a live bead or conclude there is none
			// — and it pays that walk once, on the pass that opens the new
			// episode, since the bead it then files is the newest.
			break
		}
	}
	switch {
	case open == nil:
		// No live bead for this gate: either none was ever filed, or every
		// one of them has been told its episode is over.
		if st.Red {
			return a.ciFile(bd, dir, st, out, errw)
		}
		return 0
	case st.Red:
		a.ciDrumbeat(bd, dir, st, *open, cs, errw)
		return 0
	default:
		return a.ciClear(bd, dir, st, *open, out, errw)
	}
}

// ciAlreadyCleared is whether ci-watch has already said on this bead that
// its gate went green. It is the durable half of the green branch: the bead
// stays open (ADR 0013 §4), so nothing else distinguishes a bead whose
// episode is over from one whose episode is running.
func ciAlreadyCleared(cs []BdComment) bool {
	for _, c := range cs {
		if strings.HasPrefix(strings.TrimSpace(c.Text), ciClearedPrefix) {
			return true
		}
	}
	return false
}

// ciOpenBeads is the dedupe's candidate set: every OPEN bead for THIS
// gate, NEWEST FIRST. Marker-matched rather than label-matched alone, so an
// instance watching two repos does not let one repo's red suppress the
// other's.
//
// A SET and not one bead, and newest first, because a cleared bead can
// OUTLIVE its episode: ci-watch closes only the one nobody claimed (ciHolder,
// ADR 0013 §4's exception), so a bead a seat holds sits in this listing until
// that seat closes it, and a second episode legitimately has two beads here.
// The claimed ones are the case this walk exists for — before the exception
// it was every one of them, which is the same walk over a bigger set. The
// current episode's bead is the newest, and ciActOnGate walks from there to
// the first one that has not been told its gate cleared. Picking the OLDEST — which the first cut did, to leave a
// double-file visible — is the same bug this whole mechanism exists to
// prevent, one layer in: the oldest is always the CLEARED one, so every
// pass after the second episode began would have filed another bead.
func ciOpenBeads(bd Bd, dir string, st CIState) ([]BdIssue, error) {
	issues, err := bd.OpenLabeledAny(dir, CIRedLabel)
	if err != nil {
		return nil, err
	}
	marker := ciMarker(st)
	var found []BdIssue
	for i := range issues {
		// No `Status != "closed"` guard here: OpenLabeledAny drops closed
		// rows itself, on both of bd 0.50.3's store classes and not just
		// the one that happens to be underneath (ranger-base-bwrp8). This
		// mechanism needs it more sharply than most — a dedupe that adopted
		// a closed bead would never file again, so the gate would go red for
		// five days while this sat holding a bead that says the last episode
		// is over — and a duplicated guard here is exactly what would hide a
		// regression of the general one from ciwatch_live_test.go, the arm
		// that found it.
		if !strings.Contains(issues[i].Description, marker) {
			continue
		}
		found = append(found, issues[i])
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Created.After(found[j].Created) })
	return found, nil
}

func (a *App) ciFile(bd Bd, dir string, st CIState, out, errw io.Writer) int {
	title, desc := st.Title(), st.Description()
	a.WarnOpsContent(errw, dir, "the ci-red bead for "+st.Slug, title+"\n"+desc)
	id, err := bd.Create(dir, BdNew{
		Title:       title,
		Description: desc,
		Labels:      []string{CIRedLabel, CIRedLane},
		// P1: while this is red, every bead branch's only gate is not a
		// gate, so it is not one team's problem — it is the reason nothing
		// else that lands today is verified.
		Priority: "1",
		Type:     "bug",
		// The same actor verify-after files under: `created_by` is the one
		// field that separates a bead the harness filed from a persona's.
		Actor: VerifyActor,
	})
	if err != nil {
		fmt.Fprintf(errw, "ci-watch: %s: create: %v\n", AbbrevHome(ExpandTilde(dir)), err)
		return 0
	}
	fmt.Fprintln(out, st.RedLine(id))
	return 1
}

// ciHolder is ADR 0013 §4's exception, in one predicate read off the bead:
// the empty string when NO SESSION EVER CLAIMED this bead and the harness may
// close it itself, otherwise the reason it may not, in words that go on the
// bead and on stdout.
//
// The ruling (ranger-base-8fr2j, 2026-09-05) names the state exactly —
// "status still open, never in_progress" — and the row carries two fields
// that answer it: a claim sets BOTH status and assignee (Bd.Claim), so a
// bead that is `open` with no assignee is one nothing was ever dispatched
// onto. An assignee on an `open` bead is the operator's own routing, which
// is somebody's decision about who this belongs to and not the harness's to
// overrule; every other status is a seat's.
//
// Errs toward NOT closing, always: an unrecognised status is somebody's, and
// the cost of that mistake is the minute the shipped comment already asks
// for. The opposite mistake closes a bead out from under a seat mid-fix.
func ciHolder(open BdIssue) string {
	if open.Assignee != "" {
		return open.Assignee + " is assigned it (" + open.Status + ")"
	}
	if open.Status != "open" {
		return "its status is " + open.Status + ", so a session claimed it"
	}
	return ""
}

// ciClear says on the bead that the gate is green again, naming the run that
// cleared it — and then CLOSES it if ciHolder says nobody ever claimed it
// (ADR 0013 §4's one exception, ruled on ranger-base-8fr2j; the file header
// carries the argument, and TestNoBdCloseVerbReachableFromDispatch holds the
// register that keeps this the only such caller).
//
// THE COMMENT COMES FIRST, and on both arms, which is the whole of what
// makes a failed close honest. bd's close is a child process that can fail
// on a locked store, and the order here decides what the bead says when it
// does: comment-then-close leaves a bead that is OPEN and carries the run
// that cleared it, which is exactly the state the shipped mechanism left
// behind and which any seat can finish in a minute. Close-then-comment would
// leave a closed bead with no record of what answered it — the one thing the
// bead's own DONE WHEN asks for — so the comment says "if you are reading
// this on an open bead, the close did not take" rather than asserting an
// outcome that has not happened yet.
//
// On the arm it may not close, the comment has to do the whole job the close
// would have done for the reader: say the condition is over, say which run
// says so, say that there is nothing left to build, and say why the harness
// left the close to them.
func (a *App) ciClear(bd Bd, dir string, st CIState, open BdIssue, out, errw io.Writer) int {
	held := ciHolder(open)
	why := "No session ever claimed this bead — status open, unassigned — so ci-watch closes it itself: that is the one exception ADR 0013 §4 admits (ruled on ranger-base-8fr2j, built under ranger-base-4gy4i), because a bead nobody was dispatched onto grades nobody's record. If you are reading this on an OPEN bead, the close did not take and closing it is the whole of what is left."
	if held != "" {
		why = "CLOSE IT. The harness does not: " + held + ", and a bead a seat holds stays the seat's (ADR 0013 §4: the bead is the store of record and `bd close` is the persona's; its one exception is a bead the harness filed that no session ever claimed, which this is not). If you were mid-fix, your own commits naming this bead are the record and this comment does not contradict them."
	}
	note := fmt.Sprintf("%s%s is green again on %s — %s at %s, %s.\n\nNothing is left to build under this bead: ci-watch filed it when the gate went red and the gate is no longer red. %s",
		ciClearedPrefix, st.Workflow, st.Branch, st.Latest.Short(), st.Latest.Created.UTC().Format(time.RFC3339), st.Latest.URL, why)
	if causes := a.ciCausesFor(dir, st); len(causes) > 0 {
		note += "\n\nCAUSES this streak's own runs carried, read off their failed steps rather than left for the next reader to rediscover: " + strings.Join(causes, "; ") + "."
	}
	if err := bd.Comment(dir, open.ID, note, VerifyActor); err != nil {
		fmt.Fprintf(errw, "ci-watch: %s: clear comment: %v\n", open.ID, err)
		return 0
	}
	if held == "" {
		if err := bd.Close(dir, open.ID, VerifyActor); err != nil {
			// The comment stands, so the bead is in the state the shipped
			// mechanism left it in and a seat can finish it. Said out loud
			// because a close that silently did not happen is how six
			// beads a week come back.
			fmt.Fprintf(errw, "ci-watch: %s: close: %v — the clearing comment stands and the bead is a seat's to close\n", open.ID, err)
			held = "the harness's own close failed"
		}
	}
	fmt.Fprintln(out, st.GreenLine(open.ID, held))
	return 1
}

// ciDrumbeat is what a five-day red says on the bead it already filed: the
// number, when it has DOUBLED since the last time it was said. 1, 2, 4, 8,
// 16, 32 — the incident's 191 failures earn eight comments over five days
// instead of 191, and the last of them reads as an alarm rather than as
// weather. It is launcherlag.go's cadence and watch.go's backoff, for the
// same reason both have it: a signal that recurs must get rarer as the thing
// it is about stays the same.
//
// The "last said" number is read back OFF THE BEAD — out of the description
// this filed and the comments it added, through one parser — rather than
// kept in this process. A launcher restart is the ordinary case here (the
// incident outlived several), and process state would re-say the number from
// 1 on every one of them.
func (a *App) ciDrumbeat(bd Bd, dir string, st CIState, open BdIssue, cs []BdComment, errw io.Writer) {
	said := ciLastStreak(open.Description)
	for _, c := range cs {
		if n := ciLastStreak(c.Text); n > said {
			said = n
		}
	}
	if said < 1 {
		said = 1
	}
	if st.Streak < said*2 {
		return
	}
	if err := bd.Comment(dir, open.ID, st.streakLine()+" — still red, and this is the first say since "+strconv.Itoa(said)+". Latest: "+st.Latest.Short()+" "+st.Latest.URL, VerifyActor); err != nil {
		fmt.Fprintf(errw, "ci-watch: %s: drumbeat: %v\n", open.ID, err)
	}
}

var ciStreakRe = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(ciStreakPrefix) + `(\d+)\b`)

// ciLastStreak is the largest streak number a text states, or 0.
func ciLastStreak(text string) int {
	best := 0
	for _, m := range ciStreakRe.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > best {
			best = n
		}
	}
	return best
}
