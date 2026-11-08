package posse

// The queue's git history is not bookkeeping — it is the bead-loss census
// (beadloss.go). `LostBeads` IS the git log of `.beads/issues.jsonl` in
// whatever repo the redirect lands in, so an id that never reaches a commit
// is an id the alarm can never notice leaving. While the store lived in the
// constitution repo the operator's own commits carried the jsonl along (bd's
// pre-commit hook stages it into whatever commit is being made). ADR 0015 §4
// moves the store into a repo nobody commits in for any other reason, and
// takes that free ride away with it: `bd sync` exports the JSONL and does
// NOT commit (measured, `--help`), and `bd sync --full` commits AND pushes,
// which no persona may do.
//
// So the launcher commits it, at the moment it already owns a git act: the
// close it just judged, where it fast-forwards the session branch onto main
// (dispatch.go mergeBack). "Closed means it is on main" gains "and the store
// of record's projection is in a commit".
//
// Measured alternative, rejected and recorded so it is not re-discovered:
// bd 0.49.1 has `bd daemon start --auto-commit` (and a separate --auto-push),
// which would commit the jsonl with no posse code at all. It commits on a 5s
// timer with no bead to name, and its git failures land in daemon.log where
// nobody reads them — including a refusal from the visibility gate, which is
// exactly the failure this repo's hooks exist to make loud. The launcher's
// commit says one line on the pass instead.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// queueJSONLPaths are the two files the queue repo tracks that a bead can
// change: the projection bd exports, and the deletion ledger beside it. The
// database itself is gitignored by bd's own `.beads/.gitignore` and must
// stay that way — it is 9MB of binary that changes on every mutation.
var queueJSONLPaths = []string{beadsJSONL, beadsDeleted}

// QueueRepo is config `queue_repo:` — the repo ADR 0015 §4 moves the store
// of record into. Absent means the move has not happened, and absent is the
// shipped default: this whole path is inert until the operator writes the
// key, so installing a posse that knows how to commit the jsonl does not
// start committing in the constitution repo the day before the cutover.
//
// ABSOLUTIZED here, before anything reads it — the same act install-hooks'
// target takes one call site over (ranger-base-k9x8n), for the same reason
// and one rung further on (ranger-base-8kwa5). This is the ONE place that
// reaches `underDir`, `OwnsCommitWall`, `InstallCommitGuardHook` and D7's
// refusal together, and a relative `queue_repo:` resolved none of them: the
// value stayed relative all the way to the refusal, which then named a
// relative target as the repo AND prescribed that spelling as a
// `beads_visibility:` key. Such a key is resolved against the READING
// process's cwd (samePath -> resolvedPath -> filepath.Abs), so the remedy was
// inert wherever it is read from another cwd and a cwd-dependent wildcard
// over the visibility verdict that same wall enforces. MEASURED 2026-11-08,
// darwin 25.6.0, cwd at the repo's parent and `queue_repo: 001`: the refusal
// read "001's commit wall was rendered by home <abs home> ... remedy: add
// `001: <private|public>`" — the relative spelling in both places.
//
// NOT the WRITE path, and that is recorded so nobody re-measures it: the wall
// InstallCommitGuardHook renders through a relative `queue_repo:` was
// BYTE-IDENTICAL to the absolute one (MEASURED the same day, 64437 bytes).
// k9x8n's identity-derivation consequence does not reach this call site —
// because the queue repo HOLDS the store, so it carries no `.beads/redirect`
// and identityRedirectTarget answers "". Not because the dir cannot be
// relative. So this fix is the refusal and the four predicates, not the
// render.
//
// filepath.Abs Cleans, so the Clean is its; it is kept spelled only where Abs
// cannot answer. What Abs does NOT do is resolve symlinks, so two absolute
// spellings of one repo are still two repos here — measured and handed to
// architecture as ranger-base-5wq6g, and the same open end k9x8n left.
func (a *App) QueueRepo() string {
	v := a.CfgGet("queue_repo", "")
	if v == "" {
		return ""
	}
	p := ExpandTilde(v)
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return filepath.Clean(p)
}

// QueueCommit is one attempt's outcome. Skipped carries why nothing was
// committed and is "" exactly when SHA is set — a caller reporting on a
// pass needs to tell "not configured" from "configured and it did nothing",
// because the second one is a close whose record did not reach git.
type QueueCommit struct {
	Repo  string   // the queue repo the store resolved into
	Store string   // the .beads directory itself
	Paths []string // what was committed, relative to Store
	// Dropped names paths that are in the store and are NOT in Paths:
	// they had no index entry and `git add` refused to give them one.
	// A commit can succeed with a non-empty Dropped — that is the whole
	// point of dropping them — so it is reported alongside SHA rather
	// than instead of it.
	Dropped []string
	SHA     string // the commit, "" when none was made
	Skipped string // why not, "" when one was
}

// droppedNote is what a pass says about a file that was in the store and did
// not make the commit. Empty for every ordinary close, which is why it is a
// suffix rather than a line.
func (c QueueCommit) droppedNote() string {
	if len(c.Dropped) == 0 {
		return ""
	}
	return " — without " + strings.Join(c.Dropped, " ") + ", which git would not stage"
}

// CommitQueueJSONL flushes the database to its JSONL projection and commits
// that projection in the queue repo. It NEVER pushes: nothing here runs
// `git push`. On the instance scripts/queue-cutover.sh cut, the queue repo
// is created with no remote at all, and an instance may carry one — backup
// stopped reading the queue's remotes at ADR 0049 (ranger-base-gjbdl) and
// nothing else here ever did. What holds on every instance, with a remote
// or without, is that the harness never pushes: the binary invokes no `git
// push` anywhere, every shipped PID denies it (TestExampleAgentsArePIDs),
// and the push is the operator's, typed by hand. That is the guarantee.
//
// The commit is path-limited — `git commit -- <paths>` — for the reason
// every commit in this codebase is: an unqualified commit takes whatever is
// staged, and the queue repo's index is as shared as any other. It is also
// what makes the commit ignore bd's own untracked droppings (`daemon-error`,
// `daemon.log`) rather than sweeping them in.
//
// dir is any repo whose redirect resolves to the store; msg is the commit
// message, which should name the bead so `git log --grep <id>` finds it.
func (a *App) CommitQueueJSONL(bd Bd, dir, msg string) (QueueCommit, error) {
	q := a.QueueRepo()
	if q == "" {
		return QueueCommit{Skipped: "config queue_repo: is unset — the store has not moved yet (ADR 0015 §4)"}, nil
	}
	store := beadsHome(dir)
	// underSameDir and not underDir: `q` is `queue_repo:` as the operator
	// typed it and `store` is whatever the session's redirect names, so the
	// two spellings are independent and the question is only ever about the
	// directory (ADR 0082 D4 — the comparison answers containment by
	// identity, and canonicalPath never feeds a comparison). On the string
	// answer a case-changed `queue_repo:` over a store spelled as on disk
	// skipped every close, and a skip here costs the bead-loss census this
	// close's line (ranger-base-oczuq).
	if !underSameDir(q, store) {
		return QueueCommit{Repo: q, Store: store, Skipped: AbbrevHome(store) + " is not inside " + AbbrevHome(q)}, nil
	}
	c := QueueCommit{Repo: q, Store: store}

	// ranger-base-3c3's defect, one repo over (ranger-base-mp0v). The
	// prepare-commit-msg slot in THIS repo carries the beads visibility
	// stamp, and after the cutover this is the repo the jsonl commits land
	// in — so an absent, foreign or wrongly-stamped slot here is a launcher
	// that commits the store of record unguarded, silently, toward
	// disclosure. Nothing installed it but one runbook step
	// (scripts/queue-cutover.sh, performed once) and nothing ever asked it a
	// question: the launch probe (applyL3Probe) reads the SESSION dir, and
	// no session starts in the queue repo.
	//
	// So reconcile it exactly as a launch reconciles the session dir's
	// (herdrback), then probe. Reconcile is best effort for the same reason
	// it is there — a legitimate foreign chain is expected to make install
	// refuse — and the probe is what rules: ADR 0023 identity (the file at
	// the dispatch path is byte-for-byte the render config's visibility
	// calls for) plus behavior (that render, exec'd fresh, still refuses).
	// Reconciling first also re-stamps a slot config has since re-marked,
	// which is otherwise install-time-only and drifts unseen.
	//
	// A probe that does not hold refuses the commit. That costs the loss
	// census this close's line (beadloss.go) and says so on the pass, which
	// is the cheaper of the two failures: an uncommitted projection is
	// recoverable by the next close, an unguarded one is disclosed.
	//
	// ADR 0081 D7 narrows the reconcile without changing that shape. This
	// call is the THIRD writer of a repo's SHARED commit wall — the launch
	// is herdrback's, `posse gates install-hooks` is D4's — and the one with
	// no session behind it. `q` is `queue_repo:`, read from whichever home
	// ran the close, and the body this would write is a function of THAT
	// home's config: MEASURED 2026-11-06 (ranger-base-q0np6), same binary
	// and same repo with only RHQ_HOME changed renders 1897 lines against
	// 1137. Ownership stays D1 alone — `queue_repo:` does not confer it,
	// because it is one line any home can copy with the rest of a config,
	// which is exactly how a scratch home comes to name the real queue repo
	// (D7a, rejected). So the install is asked D3's rule first, of the same
	// predicate the launch and install-hooks ask and not a second copy of
	// it.
	//
	// A non-write leaves the owner's render where it is and refuses this
	// commit. That is the same cost as every other refusal here and no more:
	// the projection is a full export, so the next close from the owning
	// home commits what this one did not. It is also, in the common case,
	// the same commit a write would have lost anyway — a home that does not
	// mark `q` renders it PUBLIC, and the guard's check 0 scans a public
	// repo's jsonl as a publication and refuses (G13's queue-unmarked row).
	// D7 refuses it without taking the owner's wall down to do it.
	//
	// The refusal is worded HERE, and the probe below is SKIPPED on a
	// non-write. Both for the same reason: the rule answers no only over
	// l3OtherHome, which is not l3Held, so `probe.CommitGuard` is already
	// false whatever the behavior half would find — and that half execs a
	// render over the repo's own index for a verdict this close already
	// holds (MEASURED ~0.9s a repo, l3AskIdentity's note). What it cannot
	// supply is D7's two remedies, which is the whole of what makes this
	// refusal actionable, so the line is written where the rule is asked.
	// ForeignCommitWallOwner is the identity-only ask, so a close pays no
	// second behavior exec to reach it.
	//
	// Both paths are rendered through CanonicalAbbrevPath (ADR 0082 D4), not
	// AbbrevHome: this line prescribes the repo as a `beads_visibility:` key
	// for the operator to paste, and it prints the owning home beside it
	// from the wall's `# posse-home:` stamp — which D3 made canonical. So on
	// AbbrevHome alone one line showed two spellings of two paths, and the
	// key it prescribed depended on how this home's config happened to spell
	// `queue_repo:`. The value is untouched: `q` is QueueRepo()'s, which
	// feeds underSameDir, OwnsCommitWall and InstallCommitGuardHook above,
	// and a render never feeds a comparison.
	if foreignWallHome, wallRepo := a.ForeignCommitWallOwner(q); foreignWallHome != "" {
		return c, fmt.Errorf("%s's commit wall was rendered by home %s and this home does not own it, so it was left alone (ADR 0081 D1/D7) — and an unguarded jsonl commit of the store of record is the disclosure that wall exists to refuse; remedy: add `%s: <private|public>` under this home's beads_visibility:, which is both what confers ownership (D1) and ADR 0015 §4 cutover step 5 (G13's queue-unmarked row) — or close from home %s, which rendered what is there",
			CanonicalAbbrevPath(wallRepo), CanonicalAbbrevPath(foreignWallHome), CanonicalAbbrevPath(wallRepo), CanonicalAbbrevPath(foreignWallHome))
	}
	a.InstallCommitGuardHook(q) //nolint:errcheck // best effort, as at launch; the probe below is the verdict
	if probe := a.probeL3Hooks(q, false); !probe.CommitGuard {
		why := probe.CommitGuardDegraded
		if !probe.Repo {
			why = AbbrevHome(q) + " is not a git repository — queue_repo: must name a checkout"
		}
		return c, fmt.Errorf("its beads visibility stamp is not armed, and an unguarded jsonl commit is the disclosure this hook exists to refuse — %s", why)
	}

	// The database is the store of record and the JSONL is a projection of
	// it; committing without exporting first commits the state before the
	// close. --flush-only is the export with git left alone (measured: the
	// plain `bd sync` does not commit either, but --flush-only says so in
	// its name and cannot grow a git step in a later bd).
	if err := bd.Flush(dir); err != nil {
		return c, err
	}

	// On disk is not the same question as committable. A pathspec matches
	// only a path git already has an index entry for, and `deleted.jsonl`
	// is written by bd the first time a bead is deleted — which can be long
	// after the cutover script's one `git add -A .beads`. Naming an
	// untracked path does not merely skip it: it exits 1 with "pathspec
	// ... did not match any file(s) known to git" and leaves HEAD where it
	// was, so the TRACKED projection beside it does not land either
	// (measured, git 2.50.1 and 2.39.3 — ranger-base-d7ja). Every close
	// after that reports its projection commit as a launcher failure and
	// the loss census stops seeing ids, until a hand stages the new file.
	//
	// So a path with no index entry gets one first: `git add -- <the new
	// paths>`, scoped and never bare, which is the route rangerhq-4pbt
	// measured and AGENTS.md prescribes. It does not reintroduce
	// ranger-base-nor — a path-limited commit still takes the WORKING TREE
	// version of everything it names, so a daemon rewrite landing between
	// this add and the commit below is committed rather than lost
	// (measured: add, rewrite both files, commit — the commit holds the
	// rewrite).
	//
	// New paths ONLY. An add over a tracked path would overwrite an index
	// entry another hand staged, and the commit does not need one: the
	// entry it already has is what makes the pathspec match.
	//
	// It also has to be before the has-anything-changed question below,
	// not after: an untracked file is invisible to `git diff HEAD`, so a
	// close whose only movement is a first deletion would otherwise skip
	// as "already matches its last commit" and the ledger would never
	// reach git at all.
	var found []string
	for _, name := range queueJSONLPaths {
		if _, err := os.Stat(filepath.Join(store, name)); err == nil {
			found = append(found, name)
		}
	}
	if len(found) > 0 {
		known, err := git(store, append([]string{"ls-files", "-z", "--"}, found...)...)
		if err != nil {
			return c, err
		}
		tracked := map[string]bool{}
		for _, p := range strings.Split(known, "\x00") {
			if p != "" {
				tracked[p] = true
			}
		}
		for _, name := range found {
			// An add git refuses — the file is ignored, which is what
			// bd's own `.beads/.gitignore` and the fork protection in
			// `.git/info/exclude` do to files in this directory — drops
			// that one path. Failing here instead would be the same
			// defect one step earlier: one uncommittable file taking the
			// projection, and the close's whole record, down with it.
			if !tracked[name] {
				if _, err := git(store, "add", "--", name); err != nil {
					c.Dropped = append(c.Dropped, name)
					continue
				}
			}
			c.Paths = append(c.Paths, name)
		}
	}
	if len(c.Paths) == 0 {
		c.Skipped = "no issues.jsonl in " + AbbrevHome(store)
		if len(c.Dropped) > 0 {
			c.Skipped = AbbrevHome(store) + " holds only " + strings.Join(c.Dropped, " ") + ", which git would not stage"
		}
		return c, nil
	}

	// git is run from inside the store directory, so the pathspecs above
	// need no repo root — the same trick beadloss.go uses to walk a
	// redirect target's history without working out where its repo starts.
	//
	// The comparison is worktree-against-HEAD, because that is the only one
	// the commit below performs: a path-limited commit takes the WORKING
	// TREE version of the paths it names and ignores whatever is staged for
	// them (measured, git 2.39.3, ranger-base-nor — stage v2, write v3 to
	// the worktree, and `git commit -m x -- <path>` commits v3).
	//
	// `git status --porcelain` answers a wider question, and the extra
	// ground it covers is reachable here: an index entry differing from
	// HEAD over a tree that matches it. That is the state bd's own
	// pre-commit hook leaves in any repo where the blessed form runs
	// (rangerhq-be7k), and it is one `git add` by any hand in the queue
	// repo away otherwise. Asking the wide question there returned "dirty"
	// and sent the commit at a tree with nothing in it, which exits 1 — so
	// a close whose projection was already in git was reported to the
	// operator as a launcher failure. `git diff HEAD` is empty exactly when
	// the commit would have nothing to do.
	changed, err := git(store, append([]string{"diff", "HEAD", "--name-only", "--"}, c.Paths...)...)
	if err != nil {
		return c, err
	}
	if strings.TrimSpace(changed) == "" {
		c.Skipped = "the projection already matches its last commit"
		return c, nil
	}
	if _, err := git(store, append([]string{"commit", "-m", msg, "--"}, c.Paths...)...); err != nil {
		return c, err
	}
	sha, err := git(store, "rev-parse", "--short", "HEAD")
	if err != nil {
		return c, err
	}
	c.SHA = strings.TrimSpace(sha)
	return c, nil
}

// ─── the governance reading (ADR 0029 G13) ───────────────────────────────────
//
// WHY A ROW AND NOT A PASS LINE. The pass already says `the queue jsonl did
// NOT commit in <repo>: <err>` where the refusal happens (dispatch.go
// commitQueue), and for four weeks that was the whole of what anyone heard:
// retrospective, one line in a watch log, and `nothing needs a human` on the
// governance surface the same minute (bead ranger-base-wmaf9, from
// github.com/ranger360ai/posse/issues/3). A close whose projection never
// reached git is a bead the loss census can never notice leaving
// (beadloss.go), and every cause below is cleared by an operator editing one
// config line — which is exactly what the all-clear claimed was unnecessary.
//
// WHY NOT "the projection is dirty". A governance condition is a checkable
// fact computable twice with the same answer, and level-triggered so it
// heals. "The projection differs from HEAD" is neither a condition nor rare:
// `bd sync` re-exports the jsonl from any session, and the launcher commits
// it only at a close it judges, so MEASURED 2026-10-09 on this instance the
// queue repo's `issues.jsonl` was dirty at a moment nothing had been refused
// at all. A row keyed on that would fire between every close and say nothing
// about whether anything was wrong.
//
// So this reads the CAUSES that are facts about the configuration rather
// than about the hour: the ones that make the NEXT close's commit fail too,
// and that no amount of waiting clears. They are read from config and the
// filesystem, with no exec and no hook body — the hook's own verdict is
// G12's, swept over the same repo when `beads_visibility:` declares it.
//
// AND NOT "a configured store that does not resolve inside queue_repo",
// which was the third cause this reading shipped with for one afternoon and
// which a live `posse status` measured away (ranger-base-wmaf9). The
// launcher does skip those closes — `<store> is not inside <repo>`, a skip,
// so the pass prints `no queue commit` — but a `beads:` entry with its OWN
// store is the ordinary multi-project shape and not a misconfiguration: on
// this instance it named a client repo whose beads were never meant to live
// in posse's queue repo, and the row was a standing instruction to move
// them. ADR 0015 §4 moves THE STORE OF RECORD; it says nothing about every
// other store an instance reads.

// QueueReadiness is whether the queue repo can take the store's projection
// at all. Every field is a fact about config, so a reader can be re-run and
// agree with itself.
type QueueReadiness struct {
	// Repo is `queue_repo:` resolved, and "" when the key is unset — which
	// is the shipped default and the absence of this whole condition: an
	// instance that has not cut over commits nothing here and owes nothing.
	Repo string
	// NotARepo: `queue_repo:` names something git does not call a checkout,
	// so CommitQueueJSONL refuses with "must name a checkout" on every close.
	NotARepo bool
	// Unmarked: Repo is a checkout and no `beads_visibility:` entry names
	// it. Unmarked is PUBLIC (fail closed, visibility.go), and the commit
	// guard's check 0 — the beads-jsonl visibility scan — runs in public
	// repos only, so an unmarked queue repo is the one state in which the
	// launcher's own commit of the store of record is scanned as a
	// publication and refused. It is also ADR 0015 §4's cutover step 5,
	// performed once by hand and recorded nowhere else.
	Unmarked bool
}

// QueueReady is that reading. One `git rev-parse` for the repo question and
// one config read for the mark; nothing here execs a hook or reads a jsonl.
func (a *App) QueueReady() QueueReadiness {
	q := a.QueueRepo()
	if q == "" {
		return QueueReadiness{}
	}
	r := QueueReadiness{Repo: q}
	// The same question probeL3Hooks answers with its Repo field, asked
	// without the render and the exec behind it: a dir git will name a
	// hooks path for is a checkout, and one it will not is what
	// CommitQueueJSONL reports as "must name a checkout".
	if _, err := hooksDir(q); err != nil {
		r.NotARepo = true
		return r
	}
	// hookRepo, because that is the spelling the install and the probe
	// resolve the mark with (gates.go) — a reader that asked a different
	// one would call a marked repo unmarked whenever the two disagree.
	if _, src := a.BeadsVisibility(hookRepo(q)); src == VisibilityUnmarkedSource {
		r.Unmarked = true
	}
	return r
}

// GovRows is G13: the governance rendering of that reading, LANE on the
// backup-stale rule — a queue that will not take the projection stops
// nothing, the fleet keeps dispatching and closing, and what is lost is the
// record (ADR 0029's URGENT means the shop is stopped).
//
// One row per cause, each with its own Key, because Key is the whole
// identity a machine reader sees and the two have different remedies: one is
// a path in `queue_repo:`, one is a line in `beads_visibility:`.
func (r QueueReadiness) GovRows() []GovCondition {
	if r.Repo == "" {
		return nil
	}
	row := func(key, detail string) GovCondition {
		return GovCondition{ID: "G13", Class: GovLane, Key: key, Detail: detail}
	}
	// NotARepo short-circuits: "is it marked" is unanswerable about a path
	// that is not a checkout, and a second row there would be a second
	// remedy for one broken config line.
	//
	// AND IT KEEPS AbbrevHome WHILE THE ROW BELOW DOES NOT (ADR 0082 D4,
	// ranger-base-qm7eg). D4's subject is "a path that a refusal or remedy
	// prints for the operator to PASTE INTO CONFIG". This row prescribes
	// nothing: it QUOTES the config line that is already there — `config
	// queue_repo: names %s` — and its remedy is to edit that line. The
	// canonical spelling is by construction not the one the operator's
	// config carries, so rendering it here would quote their file back to
	// them in words their file does not contain, in the one row whose whole
	// job is to send them to that line. There is also often nothing to
	// canonicalise: a `queue_repo:` that is not a checkout is frequently a
	// path that does not exist, where canonicalPath is resolvedPath's string
	// answer anyway (ADR 0082 D1's first bullet, as amended by D5).
	if r.NotARepo {
		return []GovCondition{row("queue-not-a-repo", fmt.Sprintf(
			"config queue_repo: names %s, which is not a git checkout — the launcher's commit of the store of record's projection is refused at every close, so no closed bead reaches the queue repo's history and the bead-loss census goes blind (ADR 0015 §4)",
			AbbrevHome(r.Repo)))}
	}
	// CanonicalAbbrevPath, BOTH ARGUMENTS (ADR 0082 D4, ranger-base-qm7eg,
	// the third site of the three its Radius names). The second one is
	// prescribed as a `beads_visibility:` key for the operator to paste —
	// the class D4 rules on — and the first names the same repo one clause
	// earlier, so rendering only the key would leave one row showing two
	// spellings of one directory, which is the defect the amendment is
	// about rather than a smaller version of it.
	//
	// Being a GOVERNANCE row and not a refusal changes who reads it, not
	// what it prescribes: `posse status` and the cockpit print this Detail
	// verbatim, and the key in it is pasted from there. It is also the row
	// ADR 0081 D7's refusal CITES by name for this same remedy
	// (CommitQueueJSONL above), so the two lines an operator can reach this
	// one cutover step through now agree letter for letter.
	//
	// The VALUE is untouched, as at every other D4 site: `r.Repo` is
	// QueueReady()'s reading of `QueueRepo()`, and the same reading feeds
	// hooksDir and BeadsVisibility(hookRepo(q)) one function up — a render
	// never feeds a comparison.
	if r.Unmarked {
		return []GovCondition{row("queue-unmarked", fmt.Sprintf(
			"config beads_visibility: names no entry for the queue repo %s, and unmarked is PUBLIC (fail closed) — the commit guard's beads-jsonl scan runs in public repos only, so the launcher's own commit of the store of record is scanned as a publication and refused at every close; mark it (ADR 0015 §4 cutover step 5: `%s: private`)",
			CanonicalAbbrevPath(r.Repo), CanonicalAbbrevPath(r.Repo)))}
	}
	return nil
}
