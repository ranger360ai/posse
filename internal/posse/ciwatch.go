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
// A THIRD thing enforces it against ONE MORE race the first two do not
// cover (ciDupeFiled, ranger-base-jbnug): OpenLabeledAny's own OPEN-only
// promise means a bead a SEAT closed on their own commits — fixed and
// closed before this gate's next `gh run list` catches up to green — drops
// out of the dedupe's query, so a pass whose reading is still red because it
// raced the fix finds nothing open and files a second bead for the episode
// the first one already answered. ciDupeFiled reads CLOSED beads back too,
// scoped to this gate's marker AND the streak's own Since sha so an
// EARLIER, unrelated episode's closed bead never suppresses a genuinely new
// one, and gated on the closed bead lacking ciAlreadyCleared's comment so an
// episode this mechanism itself already cleared is never mistaken for one
// still racing a fix. It suppresses ONE PASS, not the rest of the episode's
// life (ranger-base-95mw3): Since does not move while a streak keeps
// failing, so a bead closed WITHOUT the gate ever recovering shares the same
// sha forever and is indistinguishable, on marker and sha alone, from the
// race this exists to catch. ciDupeFiled tells them apart over time instead
// — it marks the closed bead the first time it abstains and reads that mark
// back on the next pass, so a race (which resolves by the next read) gets
// its one free pass and a still-red episode (which does not) files again
// right after.
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
// AND A READING IS ONLY A READING IF IT IS CURRENT (ciFreshness,
// ranger-base-m46kr). `gh run list` is served from an index that hands out
// PAST pages, and nothing above asks whether the page it sorted is one of
// them: on 2026-10-05 this filed ranger-base-17jhu at P1 off a page topped
// 24 days back, naming an episode fixed at 7b35b24b the same hour it was
// found. The dedupe cannot cover it — all three arms read the STORE, never
// the clock — and the mirror failure, a stale page topped GREEN while main
// is red, is the five-day silence this file exists to end, with nothing in
// the store, stderr or `posse status` to tell it from a clean pass. So the
// newest verdict-bearing run is checked against the LOCAL
// refs/remotes/origin/<branch>, and a page more than ciFreshMaxBehind
// commits behind it is the could-not-READ abstention above rather than a
// verdict in either direction. That const carries the measurement and why a
// wall-clock bound cannot do the job.
//
// AND A RED RUN IS ONLY RED IF SOMETHING OF IT ACTUALLY FAILED
// (ciJobsSayQueue, ranger-base-rdi79). The rule two paragraphs up — a run
// GitHub STOPPED is about the queue and gets no verdict — was written at the
// run level and GitHub applies `cancelled` at the JOB level too: a run whose
// six jobs are five greens and one job that never got a runner concludes
// `failure`, and on 2026-10-05 that filed a P1 and dispatched a session over
// `bab60e51`, which nothing was wrong with (a re-run of the one job was green
// in 123s). So a `failure` about to become the verdict is checked against its
// own jobs, one extra `gh api` on a reading that is about to FILE: any job
// that failed, timed out or could not start is a real red and nothing
// changes, while a run whose only non-success job never started is set aside
// exactly as a cancelled run is and the next run down answers. Demotion takes
// positive evidence and every other answer leaves the run red, because this
// is the one guard here whose failure mode is the founding incident itself.
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

	// ciJobsVetCap bounds how many runs ONE reading will ask the jobs
	// endpoint about (ranger-base-rdi79, the job-level vet below). The call
	// is made only for a run whose conclusion is `failure` and which is
	// about to BE the verdict — 55 of this gate's last 300 runs, at most one
	// of them per pass — so a green pass pays nothing and an ordinary red
	// pass pays one child. The cap is what the pathological page costs: a
	// queue broken for everybody tops the list with phantom failure after
	// phantom failure, and walking 100 of those would put 100 children on a
	// dispatch pass. 3 because the class is 1 run in 300 and three in a row
	// is already a fact about GitHub rather than about this branch. Past the
	// cap the run stands as gh reported it, which is the reading this file
	// took before the vet existed — the same direction every other failure
	// here falls.
	ciJobsVetCap = 3

	// ciJobsReadTimeout bounds one jobs call. Shorter than ciReadTimeout
	// because it is spent on the hot path of a RED reading, after the list
	// call has already had its own 30s, and because running out is not an
	// abstention: a jobs page that did not answer leaves the run red.
	ciJobsReadTimeout = 10 * time.Second

	// ciFreshMaxBehind is how far behind the branch the newest
	// verdict-bearing run in a reading may be before the reading is unusable
	// — the freshness guard's one number (ranger-base-m46kr).
	//
	// GitHub's run-list endpoint is served from an index, and that index
	// hands out PAST pages. MEASURED 2026-10-05, 38 hand readings of this
	// gate's identical query on this box with gh 2.98.0: 1 was topped 150
	// commits behind the branch and 37 were current, and ci-watch's own
	// reading two minutes earlier had been a THIRD and deeper snapshot, 220
	// behind — an index settling, not a fixed cache, with its depth bounded
	// by nothing. The deeper one was topped 24 days back and cost a
	// dispatched session on a P1 bead with a green gate behind it
	// (ranger-base-17jhu).
	//
	// A WALL-CLOCK bound on the newest run's age cannot do this job, and that
	// is a measurement rather than a preference: the longest legitimately
	// quiet stretch between two runs here is 8.80 days (211.2h, 2026-09-11 to
	// 2026-09-20, over 689 runs), while one of the two stale pages observed
	// above was topped only 7 days back. The legitimate quiet overlaps the
	// staleness, so no age bound separates them — and a bound long enough to
	// avoid false abstention on a quiet weekend is longer than the staleness
	// it is meant to catch.
	//
	// COMMIT DISTANCE does separate them, because it reads 0 over a quiet
	// repo however long the quiet lasts. The number is how many commits can
	// legitimately sit on the branch ahead of the newest run that has a
	// VERDICT — which is not zero and is not one: a run in flight is not a
	// verdict (ciVerdict), so every commit pushed while CI is running counts,
	// and one push can carry a batch.
	//
	// MEASURED 2026-10-05 over ranger360ai/posse's whole ci.yml history on
	// main, 689 runs across 38 days (2026-08-28..10-05), reconstructed at
	// every instant a run was created or completed — 1,357 breakpoints, by
	// `git rev-list --count <newest verdict-bearing run>..origin/main` as
	// this function spells it:
	//
	//	median 0 · p95 3 · p99 5 · max 57
	//	the deepest are all 2026-08-30..09-06, the era when pushes to main
	//	shared a concurrency group and 20 runs were CANCELLED — a cancelled
	//	run carries no verdict, so a superseded batch accumulates. Since
	//	ranger-base-sfb3 keyed main's group on the commit the max is 17
	//	(524 breakpoints, 2026-09-07..10-05, 0 cancelled).
	//	the two stale pages above: 150 and 220 commits behind.
	//
	// 64 is the measured legitimate maximum (57) rounded up — zero false
	// abstentions across all 1,357 breakpoints — and 2.3x inside the
	// shallower of the two stale readings. Erring LOW is the cheap direction:
	// too low costs one loud abstention that the next completed run clears,
	// and too high costs a P1 bead filed off a page that was never about the
	// branch it names.
	//
	// It is a BOUND and not a proof. A page stale by fewer commits than this
	// reads as current, and nothing here would catch it; both pages actually
	// observed were 2.3x and 3.4x past it. And it is a bound on a distance
	// that only means something for a workflow which runs on every push to
	// the branch — ciFreshness carries what the other two workflows in this
	// repo measure at.
	ciFreshMaxBehind = 64

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

	// ciRaceAbstainPrefix opens the comment ciDupeFiled writes the ONE time
	// it suppresses filing over a closed bead, and is read back by
	// ciRaceAbstained. It is what bounds the race window jbnug's fix
	// targets to one pass instead of the still-red episode's whole life
	// (ranger-base-95mw3): a launcher restart between the grace pass and
	// the next read must still see the same "already gave this one the
	// benefit of the doubt" fact, so it goes on the bead rather than in
	// process state, the same reason ciStreakPrefix does.
	ciRaceAbstainPrefix = "ci-watch: still red, abstaining once: "
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
	// QueueOnly is what this reading SET ASIDE at the head of the page:
	// runs whose conclusion is `failure` but whose only non-success job
	// never started, which is a statement about GitHub's queue and not
	// about the branch (ciJobsSayQueue, ranger-base-rdi79). They are not
	// verdicts — they are absent from Latest, Since, Streak and
	// PriorRedRuns, exactly as a cancelled RUN is — and they are kept
	// because a reading that drops a red run has to be able to name which
	// one and say why. Empty on every ordinary pass, red or green.
	QueueOnly []CIQueueOnly
	// PriorRedRuns is the immediately preceding red streak, set only when
	// Red is false: the runs ciClear is about to say nothing went wrong in,
	// unless something reads them (ranger-base-d6zyu finding 3). Free of
	// cost — it is the same scan window ReadCI already fetched for Streak
	// and Since, just not thrown away — and bounded the same way Streak is,
	// by ciScanLimit.
	PriorRedRuns []CIRun
}

// CIQueueOnly is one run a reading set aside, with the evidence that said
// so — the Why is the sentence ciJobsSayQueue built out of the jobs page,
// kept rather than recomputed because the page it was read from is gone by
// the time anybody asks.
type CIQueueOnly struct {
	Run CIRun
	Why string
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
//
// IT IS THE RUN LEVEL AND NOTHING ELSE, which is the gap ranger-base-rdi79
// closed one level down. A run's conclusion is `failure` as soon as one job
// of it is not `success` — including a job GitHub CANCELLED because it never
// got a runner, which is the queue speaking in the exact words this function
// exists to ignore, and this function cannot see the difference because the
// difference is not in the two strings it is handed. ciJobsSayQueue is the
// other half, asked only of a `failure` about to become a verdict.
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

// ciJob is one job of one run, reduced to what the job-level rule reads.
// The field names are GitHub's own for /actions/runs/<id>/jobs.
type ciJob struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Conclusion string            `json:"conclusion"`
	RunnerName string            `json:"runner_name"`
	Steps      []json.RawMessage `json:"steps"`
}

// ciJobsPage is that endpoint's envelope. TotalCount is read rather than
// dropped: it is how a page that was PAGINATED says so, and a reading that
// cannot see every job must conclude nothing about the ones it did not get.
type ciJobsPage struct {
	TotalCount int     `json:"total_count"`
	Jobs       []ciJob `json:"jobs"`
}

// ciJobsSayQueue is the job-level half of the verdict (ranger-base-rdi79):
// the empty string unless this run's `failure` is about GitHub's QUEUE
// rather than about the code, in which case it is the sentence naming the
// evidence.
//
// THE EPISODE. ci-watch filed a P1 at 2026-10-05T19:21Z over `bab60e51` and
// dispatched a session onto it. Run 37362765576 had six jobs: five green,
// and `test (ubuntu-latest, 3)` which sat fifteen minutes in the queue and
// was then CANCELLED one second after the last running job finished —
// conclusion `cancelled`, zero steps, empty `runner_name`, no log blob at
// all (`BlobNotFound`, HTTP 404), so `gh run view --log-failed` — the first
// move the filed bead's own Reproduce block prints — answered with an empty
// string. The run-level conclusion is `failure` because one job of six is
// not `success`, ciVerdict reads that as red, and nothing was wrong with the
// commit: a re-run of that one job was green in 123s. A cancelled RUN was
// already no verdict; a cancelled JOB that reddens an otherwise-green run
// was a verdict, and should not have been. docs/notes.d/ranger-base-94grm.md
// holds the per-job table and the forensics.
//
// DEMOTION TAKES POSITIVE EVIDENCE, and every other answer leaves the run
// red. That direction is the whole safety of this: ci-watch's founding
// incident is 191 reds over five days that nobody saw (the file header), and
// a rule that suppressed a genuine red would recreate it with no trace
// anywhere — no bead, no stderr, nothing in `posse status`. A false P1 costs
// one dispatched session and leaves a bead behind saying so. So an
// unreadable page, a paginated page, an empty page, an unknown job
// conclusion, a job still running, or a `failure` no job accounts for all
// read as RED, unchanged.
//
// THE RULE, over the jobs of the run's LATEST attempt:
//
//   - `failure`, `timed_out`, `startup_failure` on any job — a real red, and
//     the answer is no. These are the job-level spellings of ciVerdict's own
//     red list, for the same reason.
//   - `success`, `skipped`, `neutral` — not the cause of the run's failure;
//     ignored.
//   - `cancelled` WITH zero steps and no `runner_name` — a job that never
//     started. The queue.
//   - `cancelled` that DID run steps or DID hold a runner — a job that was
//     stopped after doing work, which this does not claim to understand, so
//     it is no and the run stays red.
//   - anything else, including a job not `completed` — no.
//
// At least one queue-only job is required, so a run that says `failure` over
// jobs that all succeeded is not demoted: that is a disagreement with GitHub
// about its own run, and the honest reading of a disagreement is the one that
// does not suppress.
//
// ZERO STEPS is the discriminator and `runner_name` corroborates it. MEASURED
// 2026-10-05 over ci.yml on main, the 300 runs gh serves (2026-09-06T14:56:19Z
// to 2026-10-05T19:21:00Z): 244 success, 55 failure, 1 in flight. All 55
// failures carried a job with conclusion `failure` — 109 such jobs, 100 with
// 11 executed steps and 9 with 9 — so this rule demotes NONE of them and the
// red half of the gate is untouched. Across all 302 jobs of those 55 runs the
// only two conclusions in the window are `success` (193) and `failure` (109),
// every page was complete (TotalCount == len(Jobs), no pagination), and the
// episode above is the first of its class in 300 runs. Re-run it: the census
// commands are in docs/notes.d/ranger-base-rdi79.md.
func ciJobsSayQueue(p ciJobsPage) (string, bool) {
	// An empty page proves nothing, and a page gh paginated is a page whose
	// missing jobs could each be the red one.
	if len(p.Jobs) == 0 || p.TotalCount != len(p.Jobs) {
		return "", false
	}
	var queue []string
	for _, j := range p.Jobs {
		switch j.Conclusion {
		case "success", "skipped", "neutral":
			continue
		case "cancelled":
			if j.Status == "completed" && len(j.Steps) == 0 && strings.TrimSpace(j.RunnerName) == "" {
				queue = append(queue, j.Name)
				continue
			}
			return "", false
		default:
			return "", false
		}
	}
	if len(queue) == 0 {
		return "", false
	}
	return fmt.Sprintf("the run says failure but no job failed: %s never started (zero steps, no runner assigned) and the other %d of %d did not fail",
		strings.Join(queue, ", "), len(p.Jobs)-len(queue), len(p.Jobs)), true
}

// ciRunIsQueueOnly asks GitHub about one run's jobs and applies
// ciJobsSayQueue to the answer. Every failure on the way — no run id in the
// URL, a gh that did not answer, JSON that did not parse — returns no, so a
// run this cannot read stands exactly as `gh run list` reported it.
func ciRunIsQueueOnly(dir, bin, slug string, r CIRun) (string, bool) {
	id := ciRunID(r.URL)
	if id == "" {
		return "", false
	}
	out, err := ghRunJobs(dir, bin, slug, id)
	if err != nil {
		return "", false
	}
	var p ciJobsPage
	if jerr := json.Unmarshal(trimToJSON(out), &p); jerr != nil {
		return "", false
	}
	return ciJobsSayQueue(p)
}

// ghRunJobs is the second network call, and the one that is not made on a
// green pass. `gh api` rather than `gh run view --json jobs`, because the
// per-job `steps` and `runner_name` this needs are on the REST resource and
// not in gh's own run view; `per_page=100` because 100 is that endpoint's
// maximum page and a run with more jobs than that must read as paginated
// rather than as partially green.
//
// NO `filter=all`, which is deliberate and is where the forensics in
// ranger-base-94grm's notes differ from the shipped reading. The default is
// `filter=latest`, the jobs of the run's latest ATTEMPT, and the latest
// attempt is what the run's conclusion is about: MEASURED 2026-10-05, run
// 37362765576 after its re-run answers six jobs all `success` under the
// default and twelve under `filter=all`, the six cancelled-and-failed rows
// of attempt 1 among them. `filter=all` would hold a re-run run red on the
// strength of the attempt it was re-run to replace.
func ghRunJobs(dir, bin, slug, runID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ciJobsReadTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "api",
		"repos/"+slug+"/actions/runs/"+runID+"/jobs?per_page=100")
	cmd.Dir = dir
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		if ctx.Err() != nil {
			msg = "timed out after " + ciJobsReadTimeout.String() + ": " + msg
		}
		return nil, Die("%s", msg)
	}
	return []byte(out.String()), nil
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
	// THE JOB-LEVEL VET, at the HEAD of the page and nowhere else
	// (ranger-base-rdi79). A `failure` that is only a job which never
	// started is the queue speaking, so it is set aside exactly as a
	// cancelled RUN is and the next run down answers. Scoped to the head
	// because the head is the only run about to BE a verdict, and that is
	// what makes the extra call affordable: none on a green pass, one on an
	// ordinary red one, ciJobsVetCap at the worst. The cap counts CALLS and
	// not demotions, because the call is the cost — the run that ends the
	// loop by standing is one of them.
	//
	// The runs BELOW the head keep their conclusions as gh reported them,
	// so a phantom failure buried inside a red streak still counts toward
	// Streak and can still be Since. That is a number slightly too large on
	// a bead that was going to be filed anyway, against one gh child per
	// run of the streak to fix — the incident's own streak was 191. The
	// false P1 this bead is about is a reading, not a count.
	for vetted := 0; vetted < ciJobsVetCap && len(verdicts) > 0 && reds[0] && verdicts[0].Conclusion == "failure"; vetted++ {
		why, queueOnly := ciRunIsQueueOnly(dir, ghBin(q.GhBin), slug, verdicts[0])
		if !queueOnly {
			break
		}
		s.QueueOnly = append(s.QueueOnly, CIQueueOnly{Run: verdicts[0], Why: why})
		verdicts, reds = verdicts[1:], reds[1:]
	}
	if len(verdicts) == 0 {
		s.Why = "no completed run of " + q.Workflow + " on " + s.Branch + " in " + slug +
			" carries a verdict (looked at the last " + strconv.Itoa(ciScanLimit) + ")"
		if n := len(s.QueueOnly); n > 0 {
			// Which is a could-not-READ abstention and not a green pass: the
			// window held nothing but runs that never ran, and the next
			// completed run clears it.
			s.Why += "; " + strconv.Itoa(n) + " failed run(s) set aside — " + s.QueueOnly[0].Why
		}
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
	// LAST, and out of the cost order the rest of this function keeps: the
	// freshness guard is a local git read, but it needs the verdict it is
	// checking, so it cannot run before the network call that produces one.
	// Every field above stays as measured — a Why does not erase the reading
	// it rejects, because the reading is what a reader has to see to agree.
	if why := ciFreshness(dir, s.Branch, s.Workflow, s.Latest); why != "" {
		s.Why = why
	}
	return s
}

// ciFreshness is the guard on the READING ITSELF: the empty string when the
// page gh handed back can be acted on, otherwise why it cannot
// (ranger-base-m46kr). Everything else in this file asks what the runs SAY;
// this asks whether they are the current runs at all.
//
// It exists because the answer "no" is invisible in both directions, and
// only one of them leaves a trace. A stale page topped by a RED run files a
// false P1 bead — ranger-base-17jhu, filed 2026-10-05 off a page topped
// 2026-09-11, naming an episode diagnosed and fixed 24 days earlier at
// 7b35b24b. A stale page topped by a GREEN run says NOTHING while the branch
// is red, and that is this mechanism's founding incident exactly (the file
// header: 191 consecutive reds over five days, nobody looked), because the
// pass "says something only when it ACTS" and a stale green pass is
// byte-identical to a current one. So this returns a Why and ReadCI makes it
// an ABSTENTION — the "there IS a gate and it could not be READ" kind, said
// once per process — rather than a verdict in either direction. Red is left
// on the state as measured; Known() is false, so nothing reads it.
//
// THE DEDUPE CANNOT COVER THIS, which is why the guard is here and not
// there. All three of its arms (ciOpenBeads, ciDupeFiled, ciAlreadyCleared)
// read the STORE, never the clock, and a stale reading walks past every one:
// MEASURED 2026-10-05 over all 37 closed ci-red beads in this store, 20 of
// those episodes would have refiled on the FIRST stale pass and the other 17
// on the second consecutive one.
//
// THE REFERENCE POINT IS LOCAL, and it is the remote-tracking ref rather
// than the branch: refs/remotes/origin/<branch> moves when something PUSHES,
// which is the same event that creates the run, while a local `main` carries
// commits that were never pushed and so has no run to expect (this box's
// own main is routinely ahead of origin/main by the merge-backs the operator
// has not pushed yet). Nothing in a dispatch pass fetches, so that ref can
// only lag what GitHub actually has — which makes this count a LOWER bound
// on how far behind the page is, and every error here therefore falls toward
// letting the reading through rather than suppressing it. The one direction
// that matters is kept: what this can prove, it refuses to act on.
//
// Plain `rev-list --count` and NOT --first-parent, and the reason is the
// ON-chain shas rather than the off-chain ones. Plain counts the side-branch
// commits a merge brought in and --first-parent does not, so plain never
// UNDERCOUNTS how far behind the page is — and undercounting is the
// fail-open direction this guard exists to refuse, since a stale page read
// as current leaves no trace at all.
//
// MEASURED 2026-10-05 over this repo's whole first-parent chain (1,985
// commits, git 2.50.1), which needs no gh page and so is the reading to
// re-run: 731 shas answer the same under both spellings and 1,254 do not,
// plain the larger in every one and never smaller. The gaps are 2, 4, 6, 7,
// 9, 14, 19, 22, 30, 31, 42, 44, 45, 47 and 53 commits, and against this
// bound of 64 a gap of 53 is a page whose true distance is outside the bound
// and whose --first-parent distance is inside it.
//
// NO SUCH STRADDLE EXISTS TODAY, and the reason is a distance and not a
// property: the newest merge on the chain sits 730 commits back
// (2026-09-05), every sha shallower than it agrees under both spellings, and
// so no sha within 64 of the tip disagrees at all (measured: 0 shas with
// --first-parent <= 64 < plain). The fail-open is therefore CONSTRUCTIBLE
// rather than present — and it is one merge away, not one era away: 15 of
// these 1,985 commits are merges, each a `merge main into <branch>` taken so
// a bead could fast-forward, and the first one of those to land again puts
// the following 64 commits' worth of readings inside the gap. That is why
// the spelling is pinned rather than just explained
// (TestCIFreshnessCountsEveryCommitBehindAndNotTheFirstParentChainAlone,
// ranger-base-1ump9 F1).
//
// The run head shas say the same thing from the gh side, which is where F1
// measured it: over the 672 run head shas of this workflow that are on the
// chain, 409 agree and 263 do not, gaps of 4, 6, 7, 9, 42 and 53. Prefer the
// chain census above when re-checking — gh's own pagination on this repo is
// unstable in exactly the direction that costs a census its denominator
// (ranger-base-m46kr's notes fragment), and a sample only 700 commits deep
// sees zero disagreements here because the nearest merge is 730 back.
//
// The off-chain shas are NOT the reason, and a previous version of this
// paragraph had the mechanism wrong: `--first-parent A..B` limits the
// NEGATIVE traversal to first parents too, so A's own first-parent ancestry
// is still excluded and the walk is not the whole chain. Of the 19 run head
// shas here that are reachable from origin/main but off its first-parent
// chain, the two spellings agree for 17 against the tip of their own time
// and differ by 2 and by 10 for the other two, and in 0 of 19 would
// --first-parent have abstained where plain would not.
//
// IT ASSUMES THE WATCHED WORKFLOW RUNS ON EVERY PUSH to the branch, which is
// what makes a commit distance mean anything — ci.yml does (`on: push:
// branches: [main]`, no path filter), and the file header's whole argument is
// that it is the ONLY gate a commit on main passes. MEASURED 2026-10-05
// against the live repo through the shipped path, with `ci_workflow:` pointed
// at this repo's other two workflows: pages.yml abstains (newest
// verdict-bearing run 2026-09-11, 100+ commits back) and so does release.yml
// (2026-09-20), because neither runs on an ordinary push to main. That
// abstention is honest — a gate whose newest verdict is 100 commits old is
// not a statement about the tip either — but it is a CONFIGURATION answer and
// not a staleness one, it holds on every pass forever, and it is why the
// notice names both readings rather than blaming the index.
//
// A sha this checkout has never heard of is NOT stale: it is a run about a
// commit newer than anything local, so the page is ahead of the local view
// rather than behind it, and the reading stands. The cost of that reading is
// a SHALLOW clone, where the old commits a stale page is topped by are also
// absent and the guard goes quiet; the beads repos are working checkouts,
// which is why that is a documented edge and not a check.
func ciFreshness(dir, branch, workflow string, latest CIRun) string {
	ref := "refs/remotes/origin/" + branch
	cannot := "the reading cannot be checked for freshness: "
	if out, err := git(dir, "rev-parse", "--verify", "--quiet", ref); err != nil || strings.TrimSpace(out) == "" {
		// No local view at all, so nothing can be proved either way — and
		// unlike a stale page this is a FACT ABOUT THE CHECKOUT that holds on
		// every pass until somebody fetches, so saying it once per process is
		// the whole of what it costs. Letting the reading through instead
		// would leave the guard silently absent, which is the one outcome
		// this bead was filed about.
		return cannot + AbbrevHome(dir) + " has no " + ref + " to compare the newest run against (git fetch origin " + branch + ")"
	}
	if strings.TrimSpace(latest.Sha) == "" {
		return cannot + "gh's newest verdict-bearing run of " + branch + " carries no head sha"
	}
	if _, err := git(dir, "cat-file", "-e", latest.Sha+"^{commit}"); err != nil {
		return "" // ahead of the local view, not behind it
	}
	out, err := git(dir, "rev-list", "--count", latest.Sha+".."+ref)
	if err != nil {
		return cannot + "git could not count " + latest.Short() + ".." + ref + " in " + AbbrevHome(dir) + " (" + errText(err) + ")"
	}
	behind, cerr := strconv.Atoi(strings.TrimSpace(out))
	if cerr != nil {
		return cannot + "git rev-list --count " + latest.Short() + ".." + ref + " answered " + strconv.Quote(strings.TrimSpace(out))
	}
	if behind <= ciFreshMaxBehind {
		return ""
	}
	// The count itself is deliberately NOT in this sentence, and the bound
	// is. ciAbstain keys its once-per-process notice on the Why text, so a
	// moving number would re-announce the same fact on every pass that moved
	// it — a gate whose runs have stopped entirely gains a commit a day
	// forever — and that is how a visible line becomes an invisible one
	// (launcherlag.go's rule). The recipe gives the reader the exact number.
	return fmt.Sprintf("the reading is not current: the newest verdict-bearing run on %s is %s at %s (%s), more than %d commits behind %s. Either GitHub's run-list index served a PAST page (ranger-base-17jhu) or %s does not run on every push to %s — neither is a verdict about what is on the branch now, in either direction. Count it: git -C %s rev-list --count %s..%s",
		branch, latest.Short(), latest.Created.UTC().Format(time.RFC3339), latest.URL, ciFreshMaxBehind, ref, workflow, branch, AbbrevHome(dir), latest.Short(), ref)
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
	// The set-aside runs, when there are any, because the Reproduce block
	// below tells a seat to run `gh run list` and that listing is topped by
	// them: a newer `failure` than the one this bead names, which a reader
	// would otherwise take as this bead being out of date
	// (ranger-base-rdi79).
	if n := len(s.QueueOnly); n > 0 {
		fmt.Fprintf(&b, "SET ASIDE (%d), newer than the run above and not a verdict: their failure is GitHub's queue and not this branch, so `gh run list` is topped by a red run this bead is deliberately not about.\n\n", n)
		for _, qo := range s.QueueOnly {
			fmt.Fprintf(&b, "  %s  %s  %s\n      %s\n", qo.Run.Short(), qo.Run.Created.UTC().Format(time.RFC3339), qo.Run.URL, qo.Why)
		}
		b.WriteString("\n")
	}
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
		if !st.Red {
			return 0
		}
		dupe, derr := ciDupeFiled(bd, dir, st)
		if derr != nil {
			// Same rule as the ciOpenBeads read above: a store this pass
			// could not read is an unknown queue, and filing over an unknown
			// queue is the one-bead-per-push failure this exists to prevent.
			fmt.Fprintf(errw, "ci-watch: %s: %v\n", AbbrevHome(ExpandTilde(dir)), derr)
			return 0
		}
		if dupe != "" {
			// A bead already answers this exact episode — closed ahead of
			// this gate's own reading catching up to green (ciDupeFiled's
			// doc comment; ranger-base-jbnug). Filing here would be the
			// duplicate this mechanism exists to prevent, one race window
			// narrower than ciOpenBeads alone catches.
			return 0
		}
		return a.ciFile(bd, dir, st, out, errw)
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

// ciRaceAbstained is whether ciDupeFiled already gave this closed bead its
// one grace pass. Read back the same way ciAlreadyCleared is, and for the
// same reason: the fact has to survive a launcher restart, so it lives on
// the bead and not in this process.
func ciRaceAbstained(cs []BdComment) bool {
	for _, c := range cs {
		if strings.HasPrefix(strings.TrimSpace(c.Text), ciRaceAbstainPrefix) {
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

// ciSinceRe pulls the sha ciSinceSha reads back out of a filed bead's own
// streak line — streakLine's two shapes both put it right before " at "
// and a timestamp: "since %s at %s" (uncapped) and "the oldest still in the
// window is %s at %s" (capped).
var ciSinceRe = regexp.MustCompile(`(?:since|window is) ([0-9a-f]{4,40}) at `)

// ciSinceSha is the sha a filed ci-red bead's own description names as the
// start of ITS episode. It is read back out of the description rather than
// compared against a CIState this reading computed, because it is the one
// thing a filed bead never revises after the fact — unlike a live CIState's
// own Since, which moves forward while a streak is capped (streakLine).
func ciSinceSha(desc string) (string, bool) {
	m := ciSinceRe.FindStringSubmatch(desc)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// ciDupeFiled closes the one gap ciOpenBeads' OPEN-only promise leaves: a
// bead a SEAT closed — their own `bd close`, the moment they fixed it —
// drops out of ciOpenBeads before this gate's own next `gh run list` catches
// up to green. A pass whose reading is still red because it raced the fix
// then walks an empty cands list and files a SECOND bead for an episode
// that already has one and is already closed (ranger-base-jbnug: d3dgd
// closed 05:04:24, 3f4402a7 went green on GitHub at 05:07:46, x11iy filed
// 05:09:16 for the identical "since d2f13e68" episode).
//
// Marker AND the streak's own Since sha both have to match. The marker
// alone spans this gate's whole history, and a closed bead from an EARLIER,
// answered episode must not suppress a genuinely new one — a red that
// follows a green is a new red, no cooldown, on streakLine's own rule
// (TestCIWatchDoesNotRefileWhileAnEarlierEpisodesBeadIsStillOpen pins two
// episodes sharing everything but their own bead). The sha is read back off
// each candidate's own description, fixed at the moment IT was filed, so a
// later, unrelated episode that happens to start where an old one did not
// is not conflated with one that actually did.
//
// And ciAlreadyCleared has to say NO on the candidate. A bead ci-watch
// itself cleared and closed under ADR 0013 §4's exception already went
// through that comment before it closed, and treating an ALREADY-answered
// episode's own closed bead as a reason not to file its successor would be
// exactly the refile cooldown the header rejects — this only ever matches a
// close this mechanism was never told about at all, and the absence of that
// comment is the tell.
//
// ONE GRACE PASS, not the rest of the episode's life (ranger-base-95mw3).
// Marker-and-sha alone cannot tell the race jbnug's fix targets — a fixer's
// close landing seconds ahead of this gate's own read catching up to green
// — from a bead closed WITHOUT the gate ever recovering (jbnug's own d3dgd,
// closed on a local verification ahead of CI): both leave a closed bead
// sharing the live episode's Since sha while the read is still red, forever,
// because Since does not move while one streak keeps failing. But the two
// are distinguishable over TIME: the race resolves on the very next read —
// the incident jbnug's fix answers shows exactly one stale-red pass between
// the close (05:04:24) and the run actually going green (05:07:46) — while
// a genuinely still-red episode goes on matching every pass after that.
// So: the first still-red pass over a matching closed bead writes
// ciRaceAbstainPrefix on it and suppresses, same as before. Every pass after
// that reads the marker back (ciRaceAbstained) and, finding it, treats the
// episode as still red rather than still racing — the grace already had its
// one chance to turn green and did not, so this stops matching and the next
// caller files.
func ciDupeFiled(bd Bd, dir string, st CIState) (string, error) {
	issues, err := bd.AllLabeledAny(dir, CIRedLabel)
	if err != nil {
		return "", err
	}
	marker, sha := ciMarker(st), st.Since.Short()
	for _, is := range issues {
		if is.Status != "closed" || !strings.Contains(is.Description, marker) {
			continue
		}
		if since, ok := ciSinceSha(is.Description); !ok || since != sha {
			continue
		}
		cs, cerr := bd.Comments(dir, is.ID)
		if cerr != nil {
			return "", cerr
		}
		if ciAlreadyCleared(cs) || ciRaceAbstained(cs) {
			continue
		}
		note := fmt.Sprintf("%sthis pass's own read of %s/%s is still red — streak since %s at %s — over an episode %s answers with no %s comment. That is either the fix racing this gate's own read catching up to green (ranger-base-jbnug), or the fix never actually landing (ranger-base-95mw3). This pass gives it the benefit of the doubt and abstains; if the NEXT pass over this episode is still red, it files its own bead rather than trusting this one again.",
			ciRaceAbstainPrefix, st.Workflow, st.Branch, sha, st.Since.Created.UTC().Format(time.RFC3339), is.ID, ciClearedPrefix)
		if err := bd.Comment(dir, is.ID, note, VerifyActor); err != nil {
			return "", err
		}
		return is.ID, nil
	}
	return "", nil
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
