package posse

// THE LANDED TREE IS READ AFTER EVERY LANDING, OUTSIDE THE LOCK (ADR 0088
// D1-D4, from ranger-base-oe9ja). This file is the runner; filing is
// ranger-base-ddgfr's and this slice PRINTS the verdict.
//
// WHAT IT IS FOR. ADR 0087's land check judges the REPLAY arm, inside the
// lock, before the swap. It cannot see the other half of the problem: two
// seats whose halves are each green on their own base, landed one after the
// other, combining on main into a tree neither seat ever had. MEASURED
// 2026-11-11 — ranger-base-s64a5 shipped a stale-register pin,
// ranger-base-in8dr added the arms that fire it, the replay type-checks so
// 0087's gate passed it, and `make test` then exited 2 in 31.9s for every
// seat on the box. The verify lane found it about five hours later, because
// nothing reads main's exit code: ci.yml fires on the operator's push.
//
// SO THE READER IS THE LAUNCHER, AFTER THE FACT. Five differences from
// landcheck.go, and every one of them is a decision in ADR 0088 rather than
// an implementation detail:
//
//  1. It runs on BOTH landing arms, because the fast-forward arm is the one
//     0087 never judges (D1, 0087 residual (1)).
//  2. It runs OUTSIDE the lock and never blocks (D2). The landing has
//     already happened; this buys detection with attribution, in minutes
//     instead of hours, and it cannot strand the fleet.
//  3. It runs CAGED, in a detached worktree of the landed commit (D3).
//     Landing adds no human review, so the code is no more trusted after it
//     lands than before — and this one runs `make`, which is the whole point
//     (the thing that went red was a door, and a door is `go test`). If no
//     cage renders, the answer is "no answer": ⚠ and nothing recorded. It
//     NEVER falls back to running uncaged.
//  4. One run at a time, and runs COALESCE (D4). When a run ends and main
//     has moved, the next run judges the newest tip only.
//  5. The judge is the copy from the last tree recorded GREEN, not the copy
//     in the tree being judged (D1). A landing that weakens the script does
//     not change what judges it.
//
// WHAT IS NOT HERE. D5's attribution — one bead per green-to-red stretch —
// is ranger-base-ddgfr. This records the verdict and prints it, which is
// what makes that bead a reader of a record rather than a second runner.
//
// THE RESIDUALS, STATED. (1) A semantic conflict only a suite arm reaches is
// still found by the next seat's `make test`: the arms are fifteen minutes
// and three suite slots per landing and are deliberately not run (D6).
// (2) Between the landing and the verdict a dispatch can branch from a red
// main. (3) A repo with no `scripts/posse-landed-check` gets nothing.
// (4) `posse kill`'s landing arm schedules nothing — a kill must stay
// instant (herdrback.go says so where it takes the lock without waiting),
// and this runner is joined by the pass that starts it, so a kill that
// scheduled one would either block the cockpit for a minute or leak a caged
// child into a process that is about to exit. A tree landed by a kill is
// read by the next landing into that repo, one tip later. (5) The child
// inherits the launcher's environment with TMPDIR redirected and nothing
// else, which is ADR 0087 D1's rule and the contract's: what a repo's check
// needs out of the environment is the repo's to decide, and this repo's
// scrubs make's own variables for itself after its author measured a green
// over an empty room (docs/notes.d/ranger-base-r3pqf.md). Posse scrubs
// nothing on a repo's behalf.
//
// WHAT IT COSTS, and the one reading this file quotes, because it is the
// reading that decided whether D3 was affordable at all: **the cage is
// free**. MEASURED 2026-11-11 on the D1 host, judging 436f467c behind
// bwrap — 70s, 71s, 71s at a one-minute load of 4.3-6.4, against 72s for
// the same script over the same shape of detached worktree with no wall at
// all. Every other reading, and the three-entry write set that makes it
// work, is in docs/notes.d/ranger-base-5fond.md, which is the one place they
// are said.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// landedCheckScript is D1's contract, and it is ADR 0087 D1's verbatim with
// one word changed: no arguments, cwd is the tree to judge, exit 0 is green,
// and the last lines of combined output are the cause. A separate path from
// landCheckScript because the two run under different rules — the land check
// executes nothing the branch wrote and this one runs the tree's own `make`
// — so a repo may ship either, both, or neither.
const landedCheckScript = "scripts/posse-landed-check"

// landedCheckKey is D4's whole record: one repo-level git config entry
// holding `<tree-id> <green|red>`.
//
// KEYED ON THE TREE for landCheckKey's reason — the check reads BYTES, so
// two commits with one tree are one question — and it is what makes a
// coalesced run free when nothing moved: the record already names the tip's
// tree and there is nothing to ask.
//
// ONE KEY AND NOT ONE PER TREE, also landCheckKey's reason: this is a cache
// with one useful entry, and a config section that grew an entry per landing
// would be a store nothing ever prunes.
const landedCheckKey = "posse.landedCheck"

// landedCheckLimit bounds one run. generatorTimeout, the same bound every
// other non-git child the landing path starts carries, and it sits above
// D6's own ceiling with room: this repo's check is MEASURED at 49-64s warm
// and 74s cold (docs/notes.d/ranger-base-r3pqf.md), against a D6 ceiling of
// 120s at which the price goes back to architecture.
//
// A deadline here is NOT a red. It is spelled as silence by
// landChildVerdict, which is the whole doctrine this runner shares with the
// land check.
const landedCheckLimit = generatorTimeout

// landedCheckWorkDir is where the launcher puts one run's scratch: the
// detached worktree, the extracted judge, and the TMPDIR the check's own
// children write into. Under the posse state dir rather than the host's
// /tmp, because every one of them has to be inside the cage's write grant
// and the host's /tmp is the one directory every uncaged process on this box
// also writes (ranger-base-7wdb7's reason, one tier down).
func landedCheckWorkDir(a *App) string { return filepath.Join(a.StateDir, "landed-check") }

// ─── D3: THE CAGE ───────────────────────────────────────────────────────────

// landedCage is one rendered cage: the argv that RUNS and the environment it
// changes. A struct and not an *exec.Cmd because the caller owns the
// deadline — landChildVerdict reads `ctx.Err()`, so the child has to be
// built with the context, and a renderer handing back a started-shaped Cmd
// would be the one place a bound could be forgotten.
type landedCage struct {
	// Argv is the whole command, cage binary first. The judge is its last
	// element, so nothing here can exec the judge without the wall in front
	// of it — which is D3's "never falls back to running uncaged" made
	// structural rather than remembered.
	Argv []string
	// Env is the pairs applied OVER the launcher's own environment.
	Env []string
}

// landedCageFn renders that cage around one judge over one tree. It is a
// field on the checker for the reason Dispatcher.CIRead is one: the thing
// most worth pinning here is what happens when the cage REFUSES, and the
// only other way to reach that arm is to run the suite on a box with no
// bwrap and no sandbox-exec — which is a test that passes for the wrong
// reason on this box and cannot be written at all on the other.
//
// Production has exactly one implementation and it is boxLandedCage.
type landedCageFn func(a *App, judge, tree string, writable []string, env []string) (landedCage, error)

// boxLandedCage is D3: the box's own cage tier, bwrap on Linux and
// sandbox-exec on darwin, with the landed worktree and the shared build
// cache writable and nothing else.
//
// IT GOES THROUGH THE SAME RENDERERS A SEAT'S WALL GOES THROUGH — BwrapArgs
// and SeatbeltProfile — and not through RenderWall, which is a different
// question with the same shape. RenderWall renders a PERSONA's wall: it
// needs an AgentFile for the PID's denies, a Runtime for the state dirs, and
// a gates dir to write the shims into. None of those exist here. What is
// being caged is not a seat and has no persona; it is one program with one
// write grant, and composing a fake AgentFile to reach the persona renderer
// would be a seat posse announces and does not have.
//
// AN UNAVAILABLE MECHANISM REFUSES rather than returning an empty argv. An
// empty prefix is a run with no wall and nothing said, which is the one
// outcome D3 names by hand.
func boxLandedCage(a *App, judge, tree string, writable, env []string) (landedCage, error) {
	switch m := WallMechanism(); m {
	case WallBwrap:
		if !bwrapAvailableOn(runtime.GOOS) {
			return landedCage{}, Die("the landed check cannot run: this host realizes cage %s with %s and %s (ADR 0088 D3), so there is no wall to put it behind",
				CageSeatbelt, m, orElse(bwrapApplyRefusal(), "it does not resolve on the gates-free PATH"))
		}
		// The carve-out is empty on purpose, and it is the one place this
		// render is DELIBERATELY poorer than a seat's. A seat's carve-out is
		// about what a PERSONA may reach inside its grants — credential
		// denies, the rename seal, the runtime dir's control endpoints. This
		// child's grant is a worktree the launcher just created and a build
		// cache; there is nothing inside either for a deny to narrow, and a
		// carve-out composed to look like a seat's would be a wall whose
		// shape nobody could derive from what it protects.
		argv, prep, err := BwrapArgs(tree, writable, SeatbeltCarveOut{})
		if err != nil {
			return landedCage{}, err
		}
		// The prep is not housekeeping: bwrap binds only a source that
		// exists, so a write grant that is not there yet (the run's TMPDIR,
		// a build cache this box has never filled) is a mkdir the launcher
		// makes OUTSIDE the namespace, in the order BwrapArgs emitted it.
		if err := runBwrapPrep(prep); err != nil {
			return landedCage{}, err
		}
		bin, err := bwrapBin()
		if err != nil {
			return landedCage{}, err
		}
		for _, kv := range env {
			if k, v, ok := strings.Cut(kv, "="); ok {
				argv = append(argv, "--setenv", k, v)
			}
		}
		return landedCage{Argv: append(append([]string{bin}, argv...), "--", judge), Env: env}, nil
	case WallSandboxExec:
		if !SeatbeltAvailable() {
			return landedCage{}, Die("the landed check cannot run: this host realizes cage %s with %s and %s (ADR 0088 D3), so there is no wall to put it behind",
				CageSeatbelt, m, orElse(sandboxApplyRefusal(), "it does not resolve"))
		}
		// "landed-check" where a seat's profile names its persona: the
		// header line is the only thing that reads it, and an operator who
		// opens this file needs it to say what the wall is for rather than
		// to name a persona that has nothing to do with it.
		prof := filepath.Join(filepath.Dir(judge), "landed-check.sb")
		if err := os.WriteFile(prof, []byte(SeatbeltProfile("landed-check", writable, nil, SeatbeltCarveOut{})), 0o644); err != nil {
			return landedCage{}, err
		}
		return landedCage{Argv: []string{"sandbox-exec", "-f", prof, judge}, Env: env}, nil
	}
	return landedCage{}, Die("the landed check cannot run: posse has no wall render for this host (%s), and ADR 0088 D3 never runs it uncaged", runtime.GOOS)
}

// landedCheckWritable is D3's write set, and the whole of it: the run's own
// directory — which holds the detached worktree, the extracted judge and the
// TMPDIR its children write into — the worktree's private git directory, and
// the box's shared build cache.
//
// THE WORKTREE'S GIT DIR IS PART OF THE WORKTREE, not part of the repo. A
// linked worktree keeps its own HEAD, index and logs under
// `<repo>/.git/worktrees/<name>`, and a `git status` inside the tree
// refreshes that index. The check reads git history by construction
// (`history-check`, the silent-revert audit — which is WHY D3 says a
// worktree and not a `git archive` export), so a grant that stopped at the
// working tree would leave a check failing on its own plumbing rather than
// on the tree. The bind is that one directory and not the repo's `.git`:
// what stays out is the COMMON dir, which is where `packed-refs.lock` lives
// and where a stranded lock is the operator's to remove and never a seat's
// (AGENTS.md, ADR 0059 D3).
//
// THE BUILD CACHE IS SHARED ON PURPOSE (D3: "the worktree is writable, and
// so is the shared build cache"). A private cache would make every landed
// check a cold build — MEASURED 74s cold against 49-64s warm for this repo's
// check — and the cache is content-addressed, so a landed tree's entries are
// the same entries the next seat's build wants.
func landedCheckWritable(run, tree string) []string {
	w := []string{run}
	// ASKED OF GIT AND NOT SPELLED. A linked worktree's admin directory is
	// `<repo>/.git/worktrees/<name>`, which is neither inside the worktree
	// nor somewhere this package may compute: the repo's own `.git` can be a
	// file, a `commondir` can redirect it, and the name git chose is git's.
	// Spelling it by hand got a directory that was not there, which BwrapArgs
	// then created and bound — a grant for a path nothing reads, and the real
	// one still read-only, both silently (MEASURED 2026-11-11, before this
	// line; the check passed anyway, which is exactly why it was silent).
	if g, err := git(tree, "rev-parse", "--absolute-git-dir"); err == nil && g != "" {
		w = append(w, g)
	}
	if c := goBuildCache(); c != "" {
		w = append(w, c)
	}
	return dedupeStrings(w)
}

// goBuildCache is the box's shared build cache, resolved WITHOUT forking
// `go env`. The documented resolution is `$GOCACHE`, else
// `$XDG_CACHE_HOME/go-build`, else `$HOME/.cache/go-build` (and
// `~/Library/Caches/go-build` on darwin), and a fork here would be a second
// child on a path whose whole subject is that exactly one child runs and
// that it runs caged.
//
// "" is a box whose cache cannot be named, which costs a cold build and
// never a wrong answer: the grant is simply not rendered and the check's own
// `go` says what it could not write.
func goBuildCache() string {
	if c := os.Getenv("GOCACHE"); c != "" {
		return c
	}
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "go-build")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "go-build")
	}
	return filepath.Join(home, ".cache", "go-build")
}

// ─── D4: ONE AT A TIME, COALESCED ───────────────────────────────────────────

// landedTreeName is the detached worktree's directory inside a run dir, and
// `<it>.git` is the private git dir beside it. Named rather than spelled
// twice because the write grant and the `worktree add` have to agree.
const landedTreeName = "tree"

// landedChecker is D2 and D4: the landings say what moved, this runs the
// check, and the two are not the same goroutine.
//
// WHY A SCHEDULER AT ALL, rather than a call at the end of the landing. D2
// says the check runs outside the lock and never blocks, and D4 says at most
// one is in flight and that runs coalesce. A landing sweep lands five trees
// into one repo under one lock; five checks would be five minutes of box for
// one question, and the first four would each judge a tip that was stale
// before the run finished. So a landing SAYS which repo moved and this reads
// the newest tip when it gets there — which is the same answer, once.
//
// IT IS JOINED, and that is not optional on this box. A caged `make` that
// outlives the process that started it is an orphan with no POSSE_KEEP
// marker, which the fleet load guard's kill arm ends on any tick
// (AGENTS.md, "Something you MEANT to leave running"). So the pass that
// schedules is the pass that waits: `Wait` at the end of a dispatch pass,
// and at the end of `posse worktrees --land`.
type landedChecker struct {
	a    *App
	out  io.Writer
	cage landedCageFn

	mu sync.Mutex
	// pending is the coalescing set: repo path → the branch that moved.
	// A map, so N landings into one repo are one entry however many passes
	// made them, and the run reads the TIP for itself rather than taking a
	// sha captured at schedule time — which is what makes "judge the newest
	// tip only" true rather than hopeful.
	pending map[string]string
	running bool
	// done is closed when the run loop drains and stops. Remade on each
	// start, so a Wait that arrives between two cycles blocks on the live
	// one and never on a channel that is already closed.
	done chan struct{}
	// saidOwnCopy is D1's "said once": the first run with no green tree on
	// record judges with the landed tree's own copy, and says so. Per
	// checker and not per run, because the record stays empty until the
	// first green and the sentence is about the configuration, not the tip.
	saidOwnCopy bool
}

// newLandedChecker builds one. `out` must be a writer the caller is willing
// to have written to from another goroutine — a Dispatcher hands its
// serialized `outWriter()`, and the one command path that hands a bare
// writer schedules after its own last line (LandSessionTrees).
func newLandedChecker(a *App, out io.Writer) *landedChecker {
	return &landedChecker{a: a, out: out, cage: boxLandedCage, pending: map[string]string{}}
}

// Schedule says a landing moved `base` in `repo`. It starts the run loop if
// nothing is running and returns immediately — which is D2's "the landing
// has already happened", and the reason the landing path can call it under
// the lock it is about to drop.
func (c *landedChecker) Schedule(repo, base string) {
	if c == nil || repo == "" || base == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[repo] = base
	if c.running {
		// D4: a run is in flight. It re-drains this map when it finishes and
		// reads the tip then, so this landing is judged by that run's
		// successor and never by a second concurrent one.
		return
	}
	c.running, c.done = true, make(chan struct{})
	go c.loop()
}

// Wait blocks until nothing is in flight. The loop below may start a new
// cycle while this one waits — a landing can arrive from another goroutine —
// so it re-asks rather than waiting on one channel.
func (c *landedChecker) Wait() {
	if c == nil {
		return
	}
	for {
		c.mu.Lock()
		if !c.running {
			c.mu.Unlock()
			return
		}
		done := c.done
		c.mu.Unlock()
		<-done
	}
}

// loop is the one goroutine. It takes one repo at a time out of the pending
// set and judges it, and stops only when the set is empty — so a landing
// that arrives while a check runs is picked up by the same goroutine rather
// than by a second one.
func (c *landedChecker) loop() {
	for {
		c.mu.Lock()
		var repo, base string
		for r, b := range c.pending {
			repo, base = r, b
			break
		}
		if repo == "" {
			c.running = false
			close(c.done)
			c.mu.Unlock()
			return
		}
		delete(c.pending, repo)
		c.mu.Unlock()
		c.once(repo, base)
	}
}

// once is one landed check: read the newest tip, ask whether that tree is
// already on record, and if it is not, judge it caged and record what it
// said.
//
// EVERY REFUSAL IS A ⚠ AND A RECORD THAT IS NOT WRITTEN (D3's bottom row and
// D5's "no answer | any"). Nothing here returns red for a question it could
// not ask, and nothing here records a verdict it did not get — a cache entry
// written over silence would be read by the next run as a judgement.
func (c *landedChecker) once(repo, base string) {
	tip := refSHA(repo, base)
	if tip == "" {
		return // the branch is gone; the next landing into this repo re-asks
	}
	tree := revTreeID(repo, tip)
	if tree == "" {
		c.warn("%s could not be judged: git would not read %s's own tree", AbbrevHome(repo), abbrevSHA(tip))
		return
	}
	// D4's cache, and the common case of a coalesced run: the record already
	// names this tree, so the question has been asked and the answer has not
	// changed — the check reads BYTES and the bytes are the same bytes.
	//
	// Asked BEFORE the opt-in, which is the cheaper order for the case that
	// happens most: a repo that opted in and has not moved costs one git
	// config read, and a repo that never opted in pays the same two reads
	// whichever way round they go.
	if had, v := landedCheckRecorded(repo); had == tree && v != landUnanswered {
		return
	}
	if !landedCheckShipped(repo, tree) {
		return // the repo has not opted in, which is every repo until it does
	}
	run, err := os.MkdirTemp(landedCheckWorkDirEnsured(c.a), "run-")
	if err != nil {
		c.warn("%s could not be judged: the landed check has nowhere to work (%v)", AbbrevHome(repo), err)
		return
	}
	// The worktree and everything beside it are the launcher's to remove.
	// `git worktree remove` first, so the repo's administrative record goes
	// with the directory rather than being left pointing at a path that is
	// gone — and best effort, because a removal that fails is a directory to
	// sweep and never a reason to lose the verdict.
	defer func() {
		_, _ = git(repo, "worktree", "remove", "--force", filepath.Join(run, landedTreeName))
		_ = os.RemoveAll(run)
	}()
	v, said := c.judge(repo, run, tip, tree)
	switch v {
	case landUnanswered:
		c.warn("%s landed at %s and was NOT judged — %s — nothing recorded (ADR 0088 D3: a check that cannot answer never answers red)", AbbrevHome(repo), abbrevSHA(tip), said)
		return
	case landGreen:
		c.say("✓ %s green at %s (%s, ADR 0088 D1)", AbbrevHome(repo), abbrevSHA(tip), landedCheckScript)
	default:
		// The verdict and the cause, which is the sentence ranger-base-ddgfr
		// turns into a bead. It is printed here whole rather than summarized,
		// because until that bead lands this line IS the record a human reads.
		c.warn("%s is RED at %s — %s — %s (ADR 0088 D1)", AbbrevHome(repo), abbrevSHA(tip), landedCheckScript, said)
	}
	if err := recordLandedCheck(repo, tree, v); err != nil {
		// Best effort by contract, for recordLandCheck's reason: a config
		// write that fails leaves the next landing judging this tree again,
		// which is the behaviour before any record existed.
		c.warn("%s judged %s %s and the verdict could not be recorded (%v)", AbbrevHome(repo), abbrevSHA(tip), v, err)
	}
}

// judge creates the detached worktree, extracts the judge, and runs it
// caged. The three returns of landChildVerdict are its returns: green, red,
// or a sentence saying why there is no answer.
//
// THE LAUNCHER RUNS ONLY GIT HERE (D3). `worktree add` and `cat-file` are
// the whole of what this process executes outside the cage; the judge's own
// first instruction runs behind the wall.
func (c *landedChecker) judge(repo, run, tip, tree string) (landVerdict, string) {
	path := filepath.Join(run, landedTreeName)
	// `--no-checkout` is deliberately NOT passed: the check's subject is the
	// tree's bytes. Detached, so this creates no branch and the repo's own
	// refs are untouched.
	if _, err := git(repo, "worktree", "add", "--detach", path, tip); err != nil {
		return landUnanswered, fmt.Sprintf("a detached worktree of %s could not be created (%s)", abbrevSHA(tip), gitSaid(err))
	}
	judge, from, err := c.extractJudge(repo, run, tree)
	if err != nil {
		return landUnanswered, err.Error()
	}
	tmp := filepath.Join(run, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return landUnanswered, fmt.Sprintf("the landed check's own temp dir could not be made (%v)", err)
	}
	// TMPDIR inside the run dir rather than the host's /tmp, because the run
	// dir is the grant: the check's children stage and link there, and under
	// D3 nothing outside this directory and the build cache is writable.
	env := []string{"TMPDIR=" + tmp}
	cage, err := c.cageFn()(c.a, judge, path, landedCheckWritable(run, path), env)
	if err != nil {
		// D3's one hard rule, and the reason the argv comes out of a
		// renderer instead of being built here: a cage that will not render
		// is the end of the run. There is no uncaged arm to fall through to.
		return landUnanswered, oneLine(err.Error())
	}
	if from == tree && !c.saidOwnCopy {
		// D1's "said once", and said HERE rather than where the judge is
		// extracted: a run whose cage refused never judged anything, and
		// burning the once on it would mean the first run that really ran
		// said nothing about judging itself. The fallback is the
		// configuration speaking, not this tip — there is no green tree on
		// record to take the judge from, so the tree being judged is judging
		// itself, and a landing that weakened the script would have weakened
		// its own gate. It stays true until the first green lands, which is
		// why once per process is the right number.
		c.saidOwnCopy = true
		c.say("· %s is judged by its OWN %s — no tree is recorded green in %s yet (ADR 0088 D1)", AbbrevHome(repo), landedCheckScript, landedCheckKey)
	}
	ctx, cancel := context.WithTimeout(context.Background(), landedCheckLimit)
	defer cancel()
	// protocolCmd around the construction and protocolEnv around the
	// environment, which are two halves of ONE rule (ADR 0077 D3): a posse
	// process that dispatches can have another persona's gates bin leading
	// its PATH, so the cage binary is resolved outside every gates dir — and
	// since cmd.Env REPLACES the environment, handing it a list that did not
	// go through protocolEnv would throw that resolution away on the next
	// line while the call site still read as though it had one.
	//
	// THE CHILD'S ENVIRONMENT IS THE LAUNCHER'S, with TMPDIR redirected and
	// nothing else — the land check's rule, and the contract's. What a
	// repo's check needs out of the environment is the repo's to decide:
	// this one scrubs make's own variables for itself, because the caller's
	// environment deciding what a check RUNS is a green over an empty room
	// (docs/notes.d/ranger-base-r3pqf.md). Posse scrubbing on its behalf
	// would be posse legislating about a contract whose point is that the
	// repo owns what "green" means. Stated as a residual in
	// docs/notes.d/ranger-base-5fond.md rather than fixed here.
	cmd := protocolCmd(exec.CommandContext(ctx, cage.Argv[0], cage.Argv[1:]...))
	cmd.Dir = path
	cmd.Env = protocolEnv(append(os.Environ(), cage.Env...))
	// landChildVerdict is the binder: it sets the reaping grace beside this
	// deadline, because a deadline alone bounds the CHILD and not this call
	// — and this child has grandchildren by construction (`make`, and `go`
	// under it).
	return landChildVerdict(ctx, cmd, landedCheckLimit)
}

// cageFn is the renderer this checker uses, defaulting to the box's. A
// method rather than a nil-check at the call site so a checker built as a
// zero value still runs the real thing.
func (c *landedChecker) cageFn() landedCageFn {
	if c.cage == nil {
		return boxLandedCage
	}
	return c.cage
}

// extractJudge is D1's "the copy that runs comes from the last tree recorded
// green, not from the tree being judged". It reads the blob out of git and
// writes it where the cage can exec it — beside the worktree and not INSIDE
// it, because a judge written into the tree it judges would be a file the
// check can see in its own `git status` and a byte the next run's tree id
// does not account for.
//
// THE RECORD IS ONE ENTRY, SO "LAST GREEN" IS THE RECORD WHEN IT IS GREEN.
// D4 spells the record `<tree-id> <green|red>`, which holds the last tree
// JUDGED and not a history, so during a red stretch there is no green tree
// on record to take the judge from. That is read as D1's documented absent
// case — the landed tree's own copy, said once — rather than as a reason to
// coin a second config key: the alternative is a judge that gets older every
// landing until somebody fixes main, and the red stretch is exactly when the
// script is most likely to be what is being fixed.
//
// A recorded tree that no longer has the script (it was added after that
// tree, or the record predates the opt-in) falls back the same way, because
// "the judge is not there" is not a statement about the tree being judged.
func (c *landedChecker) extractJudge(repo, run, tree string) (path, from string, err error) {
	from = tree
	if had, v := landedCheckRecorded(repo); v == landGreen && had != "" {
		from = had
	}
	// gitRaw and not git: a judge is BYTES, and git() trims its output. A
	// trimmed shell script usually still runs, which is exactly what makes
	// that the wrong tool here — the one thing this gate may not do is judge
	// with a copy that is not the copy on record.
	body, gerr := gitRaw(repo, "cat-file", "blob", from+":"+landedCheckScript)
	if gerr != nil && from != tree {
		from = tree
		body, gerr = gitRaw(repo, "cat-file", "blob", from+":"+landedCheckScript)
	}
	if gerr != nil {
		return "", "", fmt.Errorf("%s could not be read out of %s (%s)", landedCheckScript, abbrevSHA(from), gitSaid(gerr))
	}
	path = filepath.Join(run, "judge")
	// 0o755: the blob's own mode is not asked. landCheckShipped's reasoning
	// one file over applies here with the sides swapped — there the execute
	// bit is left to exec so a missing one is loud; here the bytes come out
	// of a git blob into a file posse creates, so posse is the one deciding
	// the mode and a mode that cannot exec would be posse refusing its own
	// write.
	if werr := WriteExecutable(path, body, 0o755); werr != nil {
		return "", "", fmt.Errorf("%s could not be written for the cage to exec (%v)", landedCheckScript, werr)
	}
	return path, from, nil
}

// landedCheckShipped reports whether a TREE opts in, which is D1's "a repo
// opts in by shipping an executable at scripts/posse-landed-check" asked of
// the thing being judged.
//
// ASKED OF THE TREE AND NOT OF THE CHECKOUT, which is the one place this
// differs from landCheckShipped. The land check runs the BASE CHECKOUT's
// copy, so a stat of the working tree is the same question; this one runs
// out of git and judges a worktree the launcher is about to create, and the
// operator's checkout can be mid-rebase, dirty, or on another branch
// entirely. `cat-file -e` is the question git answers about a tree.
func landedCheckShipped(repo, tree string) bool {
	_, err := git(repo, "cat-file", "-e", tree+":"+landedCheckScript)
	return err == nil
}

// recordLandedCheck writes the verdict for one tree. Best effort by
// contract, and silence is never written: landUnanswered has no spelling in
// the record because the next run must read "nothing on record" and ask
// again, not "unanswered" and wonder.
func recordLandedCheck(repo, tree string, v landVerdict) error {
	if tree == "" || v == landUnanswered {
		return nil
	}
	_, err := git(repo, "config", landedCheckKey, tree+" "+v.String())
	return err
}

// landedCheckRecorded reads it back. A record in any other shape is no
// record, which is landCheckRecorded's rule and for its reason: this is one
// line of git config a human can edit, and a half-parsed verdict is the one
// thing that must not be read as green.
func landedCheckRecorded(repo string) (string, landVerdict) {
	out, err := git(repo, "config", "--get", landedCheckKey)
	if err != nil {
		return "", landUnanswered
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return "", landUnanswered
	}
	switch f[1] {
	case landGreen.String():
		return f[0], landGreen
	case landRed.String():
		return f[0], landRed
	}
	return "", landUnanswered
}

// landedCheckWorkDirEnsured is the work dir, made if it is not there. A
// failure to make it is not reported here: MkdirTemp inside it fails next
// with a sentence naming the path, and two sentences for one missing
// directory is the same finding twice.
func landedCheckWorkDirEnsured(a *App) string {
	d := landedCheckWorkDir(a)
	_ = os.MkdirAll(d, 0o755)
	return d
}

// say and warn are the only two writers. Both go through the checker's own
// writer, which the constructor's doc says must tolerate a write from this
// goroutine.
func (c *landedChecker) say(format string, a ...any) {
	fmt.Fprintf(c.out, strings.TrimRight(format, "\n")+"\n", a...)
}

func (c *landedChecker) warn(format string, a ...any) {
	c.say("⚠ "+strings.TrimPrefix(format, "⚠ "), a...)
}
