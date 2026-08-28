package rhq

// posse relaunch — session refresh as routine hygiene (rangerhq-dxq).
//
// A long-lived persona session accumulates context nobody can see and
// nothing durable comes out of it when it is finally reaped. Relaunch is
// the maintained way to start one over without losing what it learned:
// land the plane (one bounded turn in which the agent writes its lessons
// down and commits what it may), close the workspace, then recreate it
// from the meta the first launch wrote — same persona, dir, env sets,
// runtime, tier, cage. Everything the recreate needs already lives in
// state/herdr/<name>.yaml, which is why this is a rearrangement of
// CreateSession and not a second launch path.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// DefaultLandTimeout bounds the landing turn. Landing is wrap-up, not work
// — but it can run a suite and a commit, so the bound is generous; the
// session is only closed once the turn has actually settled.
const DefaultLandTimeout = 10 * time.Minute

type RelaunchOpts struct {
	Name    string
	NoLand  bool          // skip the landing turn (a dead or wedged session)
	Timeout time.Duration // bound on the landing turn (0 → DefaultLandTimeout)
	// Force stands the ADR 0013 §4 reap guard down: refresh a session that
	// still holds an open bead over an uncommitted tree. The operator has
	// read the refusal and said do it anyway.
	Force bool
}

// RelaunchSession lands, kills, and recreates one session by name.
//
// The kill is irreversible and the meta it deletes is the only record of
// what to recreate, so the order is: prove the replacement is buildable,
// *then* destroy the original. planLaunch resolves the whole recreate —
// persona, runtime, tier, cage, parity, skills, seatbelt, gates, env sets,
// working directory — without touching herdr, and the plan it returns is
// the one the recreate is built from, so preflight and create cannot
// disagree. A relaunch that cannot be completed is refused with the session
// still running (rangerhq-v52t).
//
// The kill has a twin, and the preflight cannot help there either: a
// session this pass cannot see is not a session that is gone, so the unlink
// beside the kill proves death before it destroys the record — see
// clearDeadMeta (rangerhq-i2g9).
//
// Every step here is aimed by the session's RECORD and not by its name.
// Resolve falls back to foreign workspaces by label, so a workspace merely
// wearing the name could answer for a session whose own workspace this
// listing did not show, and the kill closed the stranger while the session
// it was asked to refresh kept running (rangerhq-9jk1, closeRecorded). The
// preflight has the other half of that board: a workspace already wearing
// the name will still be wearing it after the kill, and the recreate could
// never take the name back (nameWornElsewhere).
//
// What preflight cannot cover is herdr itself: the workspace create is on
// the far side of the kill by construction, and no ordering fixes that. If
// it fails there, the session's recipe is written back (keepRecipe) and the
// error carries both ways out — the retry and the hand-rebuilt command —
// so a failure costs a restart, never the session's identity.
//
// The preflight and the landing turn run unserialized; everything from the
// kill down runs under the launcher lock (replace, ranger-base-w4h5).
func (b *HerdrBackend) RelaunchSession(w io.Writer, o RelaunchOpts) error {
	// Read the meta before anything calls Sessions(): a meta whose
	// workspace has already died is pruned on read, and that file is the
	// only record of what to recreate.
	m, ok := b.readMeta(o.Name)
	if !ok {
		return Die("no session meta for %s (not created by posse — nothing to recreate from)", o.Name)
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = DefaultLandTimeout
	}

	if m.Dir == "" {
		fmt.Fprintf(w, "warning: %s recorded no dir — recreating in the configured default\n", o.Name)
	}
	recreate := RecreateOpts(m)
	plan, err := b.planLaunch(recreate)
	if err != nil {
		// Nothing has been destroyed yet, and this is the whole reason for
		// planning first: the session the operator asked to refresh is still
		// running, and still theirs.
		return Die("%s cannot be recreated as it stands, so it was NOT closed: %v", o.Name, err)
	}
	fmt.Fprintf(w, "checked %s: %s\n", o.Name, describePlan(recreate, plan))

	// The one obstacle to the recreate that does not live in the plan, and
	// the only one the preflight used to walk past.
	if other, label, err := b.nameWornElsewhere(m); err != nil {
		return Die("%s was NOT closed: this herdr did not list its workspaces (%v)", o.Name, err)
	} else if other != "" {
		// The obstacle's OWN label, not the session name: that is the string
		// `herdr workspace list` prints, and under an instance tag the two
		// differ (rangerhq-ouf9).
		return Die("%s cannot be recreated as it stands, so it was NOT closed: herdr workspace %s is also labelled '%s' and posse did not create it, so the recreate could not take the name back.\n"+
			"  rename or close %s in herdr (herdr workspace list), then relaunch again",
			o.Name, other, label, other)
	}

	if !o.NoLand {
		settled, err := b.landThePlane(w, m, timeout)
		if err != nil {
			return err
		}
		if !settled {
			return Die("%s is still working after %s — let it finish and relaunch again (or --timeout %s, or --no-land to close it anyway)",
				o.Name, timeout, (2 * timeout).String())
		}
	}

	// The reap guard (ADR 0013 §4), asked AFTER the landing turn, because
	// the landing turn is the fix: an agent that wrote its lessons down and
	// committed leaves a clean tree and refreshes exactly as before. What is
	// refused is the other shape — a session whose bead nobody recorded
	// finishing, over a tree nobody committed, about to lose the only agent
	// that knows what is in it. `--no-land` reaches this with the tree
	// untouched, which is the reap the ADR names.
	if !o.Force {
		if why := b.ReapRefusal(m); why != "" {
			return Die("%s was NOT closed: %s — look first (posse attach %s), or `posse relaunch %s --force`",
				o.Name, why, o.Name, o.Name)
		}
	}

	// Everything destructive runs under ONE launcher lock (ADR 0011 §1),
	// taken here rather than at the top of the call.
	//
	// The whole RelaunchSession cannot take it: landThePlane waits up to
	// DefaultLandTimeout (10m) for an agent's turn to settle, and a launcher
	// lock held for ten minutes is every dispatch pass, every `posse new`
	// and every listing's prune queued behind one operator's refresh. The
	// preflight is read-only and the landing turn destroys nothing, so
	// neither needs the serialization; from here down every step writes the
	// meta dir.
	//
	// Nor is it one lock per destructive step. The kill and the recreate are
	// one critical section or they are none: between them the name is free,
	// and a launcher that takes it there leaves this pass with a session
	// killed and a recreate that can never succeed — the rangerhq-v52t loss,
	// arrived at from the other side. Under one lock a create for this name
	// is either finished before the kill (holdsRecorded and mustNotOrphan
	// both read it) or has not begun.
	//
	// underLaunchLock and not lockLaunches: a relaunch reached from inside a
	// pass that already holds the lock must run, not deadlock on its own
	// open file description.
	return underLaunchLock(b.App, w, func() error { return b.replace(w, m, recreate, plan) })
}

// replace is relaunch's destructive half — kill, recreate, and the rollback
// between them — and it runs under the launcher lock its caller took.
func (b *HerdrBackend) replace(w io.Writer, m *HerdrMeta, recreate NewSessionOpts, plan *launchPlan) error {
	name := m.Name
	// The kill's own unlink (closeRecorded: CloseWorkspace, then remove the
	// meta) is a check-then-act over the meta dir too, and it is covered by
	// being here: no create for this name can land between the two.
	if err := b.closeRecorded(m); err != nil {
		return err
	}
	fmt.Fprintf(w, "killed %s\n", name)

	if ws, err := b.recreateSession(recreate, plan); err != nil {
		if ws != "" {
			// A replacement workspace is up; only its start-up did not
			// finish. Writing the recipe back would blank the meta of a live
			// workspace and orphan it — the cpeh harm, self-inflicted. Name
			// it and leave the record naming it.
			return Die("%s was closed and its replacement workspace %s came up, but did not finish starting: %v\n"+
				"  retry with:\n    posse relaunch %s\n"+
				"  or rebuild it by hand:\n    %s",
				name, ws, err, name, RecoverCommand(m))
		}
		if kept := b.keepRecipe(m); kept != "" {
			// The record on disk still names a workspace this pass could not
			// prove dead, so it was left as it stands rather than blanked —
			// and the workspace id is the one thing the operator needs.
			return Die("%s could not be recreated: %v\n"+
				"  its record in %s still names workspace %s, which this pass could not prove dead — nothing was blanked\n"+
				"  look at it first:\n    posse list\n"+
				"  or rebuild it by hand:\n    %s",
				name, err, b.metaPath(name), kept, RecoverCommand(m))
		}
		return Die("%s was closed but could not be recreated: %v\n"+
			"  its recipe was kept in %s — retry with:\n    posse relaunch %s\n"+
			"  or rebuild it by hand:\n    %s",
			name, err, b.metaPath(name), name, RecoverCommand(m))
	}
	fmt.Fprintf(w, "ready: posse attach %s\n", name)
	return nil
}

// RecreateOpts is the launch the meta describes — the session's own recipe,
// read back as the options that would create it again.
func RecreateOpts(m *HerdrMeta) NewSessionOpts {
	return NewSessionOpts{
		Name: m.Name, Dir: m.Dir, Cmd: m.Cmd, Emoji: m.Emoji,
		Envs: splitEnvNames(m.Envs), Agent: m.Agent,
		Runtime: m.Runtime, Tier: m.Tier, Cage: m.Cage,
		// The operator consented to this session's degradation when it was
		// created; relaunching the same session is not a new decision.
		AllowDegraded: m.Degraded != "",
		// Nor is it a change of hands: a session the operator was talking
		// to is still theirs on the other side of the refresh (ADR 0008).
		Crew: m.Crew,
		// The refreshed session is the same session, so it is still the one
		// that bead was dispatched to: the reap guard must still find it
		// (ADR 0013 §4).
		Bead: m.Bead,
		// A session that had its own tree keeps it. Dir already IS the
		// worktree, so EnsureSessionTree resolves the same main checkout
		// through it and finds the tree standing; the flag is what keeps the
		// meta's `repo:`/`branch:` on the recreated session rather than
		// silently demoting it to a shared-checkout session (rangerhq-09o2).
		// A tree the operator removed by hand is not self-healed here — the
		// launch's own "directory not found" is the honest answer, and
		// re-cutting a branch from today's main would not restore what was
		// in it anyway.
		Worktree: m.Branch != "",
	}
}

// SessionTreeOf reads a session's worktree out of the run record it was
// written into (ADR 0011 §3). nil means the session shares the checkout —
// including every session created before per-session worktrees landed,
// whose meta has no `branch:` and so has nothing to merge or prune.
func SessionTreeOf(m *HerdrMeta) *SessionTree {
	if m == nil || m.Branch == "" || m.Repo == "" || m.Dir == "" {
		return nil
	}
	// Base is the branch this session was CUT from, recorded on the branch
	// when it was cut. It used to be read fresh out of the repo's HEAD, and
	// that was ranger-base-5s2o: an operator who switches branches while a
	// persona works is not moving the merge target, and reading HEAD made
	// the close land the persona's commits on the operator's own branch.
	// A branch older than the recording falls back to the repo's branch,
	// and a detached HEAD then answers "" — MergeSessionWork says so in
	// words either way.
	return &SessionTree{Repo: m.Repo, Path: m.Dir, Branch: m.Branch, Base: baseOf(m.Repo, m.Branch, repoBranch(m.Repo))}
}

// recreateSession is the create half, minus the planning the caller already
// did: the name is only free once the kill has happened, so its guards run
// here and not before.
func (b *HerdrBackend) recreateSession(o NewSessionOpts, p *launchPlan) (string, error) {
	if err := b.nameFree(o.Name); err != nil {
		return "", err
	}
	return b.startPlanned(o, p)
}

// clearDeadMeta unlinks the meta of a session this pass could not find. It
// is relaunch's other destructive step: the kill removes the meta of a
// session whose recorded workspace this listing holds as its own, this
// removes the meta of one it does not (closeRecorded).
//
// It used to be a bare os.Remove commented "workspace already gone; the
// meta is ours to clear", and the comment was the bug. HasSession answers
// out of Sessions(), where a meta whose workspace is missing from the
// listing snapshot is spared but deliberately left OUT of the listing
// (rangerhq-9nso's own scope note). So HasSession == false is not "gone" —
// it is exactly the 9nso condition: alive on the server, absent from this
// read. The unlink destroyed the only record of a live session, and the
// recreate then walked straight past mustNotOrphan, which read no meta,
// rightly concluded there was nothing to orphan, and built a SECOND
// workspace under the same label while the first kept running its agent
// with nothing on disk naming it. Silent, err == nil, and the default
// `posse relaunch <name>` walks it: landThePlane resolves through Sessions()
// too, so it reports "nothing to land" and waves it through (rangerhq-i2g9).
//
// A delete this unrecoverable proves death like every other one (ADR 0011
// §2): mustNotOrphan is that proof, and asking it HERE — one line before
// the evidence it reads would be gone — is the whole fix. It is the same
// predicate the create and the prune ask through, so the three cannot drift
// apart, and it brings the socket guards with it: a meta this herdr cannot
// answer for is not cleared by a relaunch either (rangerhq-jeu2, rangerhq-8fq).
//
// A workspace that answers alive is refused, not closed by id. This pass
// cannot land it — AgentTarget resolves through the same listing, which is
// why the landing turn just reported "nothing to land" — and closing a live
// agent that never got its landing turn is precisely the loss relaunch
// exists to prevent. Refusing costs a retry and nothing else: the session is
// left running with its record intact, a stale snapshot clears on the next
// pass, and a meta another herdr holds says so and says which.
//
// Proof and act were two steps over a file another actor writes, which is
// rangerhq-3a5t's shape one caller over: between mustNotOrphan answering
// and os.Remove landing, a create for this name can pass the same guard
// legitimately (the old workspace really is dead) and writeMeta a fresh
// meta at this very path. The unlink then deletes the NEW record — a live
// session with nothing on disk naming it, i2g9's damage reached through the
// write/delete interleave instead of a stale snapshot.
//
// So the act is taken under the launcher lock (ADR 0011 §1) and the proof
// is taken AGAIN inside it, reclaim's pattern verbatim: locking the unlink
// alone would still act on evidence read before the lock, which is the same
// race one step over. Under the lock a create for this name is either
// finished — and the re-read sees its meta, naming a workspace herdr
// answers for, so the guard refuses — or has not begun, because every
// create takes this lock (CreateSession, and RelaunchSession's own tail).
//
// underLaunchLock and not tryLockLaunches, which is where this parts from
// the prune: contention there means "spare the file", a safe answer a
// listing can give. Here the caller is mid-relaunch and its own tail holds
// the lock, so treating this process's lock as contention would refuse
// every relaunch. Nesting runs the body directly.
//
// The outer proof is kept, and is not the guard: it is the fail-fast, so a
// relaunch reached with the lock free is refused without first queueing
// behind a firing pass. What decides is the one inside.
func (b *HerdrBackend) clearDeadMeta(name string) error {
	if err := b.provenClearable(name); err != nil {
		return err
	}
	return underLaunchLock(b.App, b.warnWriter(), func() error {
		if err := b.provenClearable(name); err != nil {
			return err
		}
		os.Remove(b.metaPath(name))
		return nil
	})
}

// provenClearable is clearDeadMeta's proof and its refusal, in the words the
// caller prints — asked twice, once outside the lock and once inside it.
func (b *HerdrBackend) provenClearable(name string) error {
	if err := b.mustNotOrphan(name); err != nil {
		return Die("%s was NOT closed and its record was left in place: %v", name, err)
	}
	return nil
}

// keepRecipe puts a session's meta back after a recreate that failed on the
// far side of the kill. The workspace really is gone, so it is recorded as
// gone: a meta naming no workspace can orphan nothing (mustNotOrphan lets
// the next create through) and cannot be inferred dead and reaped
// (prunable refuses without an id to ask about), so the recipe outlives the
// failure and `posse relaunch <name>` is itself the retry. Best effort — a
// bookkeeping write must not replace the error the operator needs to read.
//
// "The workspace really is gone" was an assumption about the caller, and it
// is the fourth meta-destroying step in this file — the one that asked
// nothing. Blanking workspace: is a meta WRITE, and a write over a meta is
// as unrecoverable as the delete: state/ is outside git, so the id is not
// recoverable from anywhere else, and mustNotOrphan exists for exactly that
// (rangerhq-cpeh). On the rangerhq-9jk1 board this ran on the way OUT of
// that guard's own refusal and destroyed the record the refusal had just
// declined to overwrite. So it proves death like the other three, through
// the same predicate (ADR 0011 §2), and a record it may not blank is left
// standing and named to the caller instead.
//
// mustNotOrphan is asked of the FILE and not of m: after an ordinary kill
// there is no file, and a name with no record has nothing to protect — so
// the ordinary rollback still pays nothing and still writes the recipe.
//
// And the write is the act of a check-then-act, exactly as the unlink is
// (rangerhq-3a5t): between mustNotOrphan and writeMeta a create for this
// name can write a fresh meta, and blanking workspace:/pane: over it is the
// cpeh damage — a LIVE session's workspace id gone from the only place it
// is recorded. So it goes under the launcher lock with the proof re-asked
// inside, the same shape as clearDeadMeta, and what it may not blank it
// names to the caller instead.
func (b *HerdrBackend) keepRecipe(m *HerdrMeta) (kept string) {
	if err := b.mustNotOrphan(m.Name); err != nil {
		return b.recordedWorkspace(m)
	}
	// Best effort to the end: a lock this pass cannot take is not a reason to
	// replace the error the operator needs to read, and a recipe that was not
	// written costs the retry `posse relaunch` already is.
	_ = underLaunchLock(b.App, b.warnWriter(), func() error {
		if err := b.mustNotOrphan(m.Name); err != nil {
			kept = b.recordedWorkspace(m)
			return nil
		}
		r := *m
		r.Workspace, r.Pane, r.Launched = "", "", time.Time{}
		_ = b.writeMeta(&r)
		return nil
	})
	return kept
}

// recordedWorkspace is the workspace id a refused blanking is protecting:
// whatever the file names NOW, since a create may have rewritten it under
// this pass, and only failing that the id this pass came in holding.
func (b *HerdrBackend) recordedWorkspace(m *HerdrMeta) string {
	if cur, ok := b.readMeta(m.Name); ok && cur.Workspace != "" {
		return cur.Workspace
	}
	return m.Workspace
}

// closeRecorded is relaunch's destructive half, and it is aimed by the
// session's own record rather than by its name.
//
// It used to ask HasSession(name) and then KillSession(name), and both
// answer out of Resolve — which falls back to FOREIGN workspaces by label.
// So a workspace merely WEARING the session's name answered for a session
// whose own workspace was missing from this listing snapshot (the
// rangerhq-9nso condition, i2g9's own board): the kill arm closed that
// stranger, never matched to this meta and with no landing turn, while the
// session the operator asked to refresh kept running. The create then
// refused, correctly, over a record the same call stack blanked one line
// later, and the retry the refusal advises finished the orphaning
// (rangerhq-9jk1). Label resolution is right for `posse kill`, where the
// operator is pointing at a row they can see; it is wrong here, where the
// whole job is to rebuild the session THIS meta describes.
//
// So the listing is consulted for one thing: does it hold the workspace the
// meta NAMES, under an identity this pass may call ours (notOurWorkspace,
// the rangerhq-yt1p fence)? If it does, that workspace is closed and its
// record cleared — the ordinary path, which still pays no per-id query. If
// it does not — genuinely gone, hidden behind the snapshot, held by a
// stranger, or unlistable — nothing is closed by id, and what may be done
// to the record is clearDeadMeta's call, which proves death or refuses.
func (b *HerdrBackend) closeRecorded(m *HerdrMeta) error {
	ours, err := b.holdsRecorded(m)
	if err != nil || !ours {
		// A listing this pass could not read decides nothing either: the
		// guard asks again and turns it into the refusal that says so.
		return b.clearDeadMeta(m.Name)
	}
	if err := b.H.CloseWorkspace(m.Workspace); err != nil {
		return err
	}
	os.Remove(b.metaPath(m.Name))
	return nil
}

// holdsRecorded reports whether this herdr's listing holds the workspace m
// names, as m's own. It is the aiming question — which workspace is this
// session's — and never a liveness proof: absence from a listing is a
// snapshot, and what that permits is mustNotOrphan's to say.
func (b *HerdrBackend) holdsRecorded(m *HerdrMeta) (bool, error) {
	if m.Workspace == "" {
		return false, nil // a kept recipe names none (rangerhq-v52t)
	}
	wss, err := b.H.Workspaces()
	if err != nil {
		return false, err
	}
	gen := ServerGen()
	for _, ws := range wss {
		if ws.WorkspaceID == m.Workspace {
			return b.notOurWorkspace(m, ws, gen) == "", nil
		}
	}
	return false, nil
}

// nameWornElsewhere finds a workspace other than this session's own that
// already wears its name.
//
// The recreate needs the name free (nameFree), and a herdr workspace
// labelled <name> that this session's record does not point at is still
// there on the far side of the kill — so it is a refusal the preflight can
// raise while it costs nothing, which is rangerhq-v52t's rule applied to the
// one obstacle that lives in herdr rather than in the plan. Without it the
// kill happens, the create fails on a name it can never take, and a session
// is destroyed for a reason that was knowable before anything was touched.
//
// It is also what keeps the rest of relaunch pointed at its own session on
// the rangerhq-9jk1 board. Resolve falls back to a foreign row only when
// none of this session's own is listed, so once no other workspace wears
// the name, landThePlane's AgentTarget can address this session or nothing
// — never a stranger's agent. And the refusal names the workspace in the
// way, which the orphan guard's own message cannot: it would send the
// operator to `posse attach <name>`, which resolves to that same stranger.
func (b *HerdrBackend) nameWornElsewhere(m *HerdrMeta) (id, label string, err error) {
	wss, err := b.H.Workspaces()
	if err != nil {
		return "", "", err
	}
	for _, ws := range wss {
		// "Wears its name" is this home's rendering of it (rangerhq-ouf9):
		// a workspace another instance labelled <their tag>/<name> is not
		// in the way of a create this instance labels <our tag>/<name>, and
		// refusing over it would block a relaunch nothing can obstruct.
		if b.App.labelWearsName(ws.Label, m.Name) && ws.WorkspaceID != m.Workspace {
			return ws.WorkspaceID, ws.Label, nil
		}
	}
	return "", "", nil
}

// describePlan is the preflight's receipt: what the recreate resolved to,
// printed before anything is destroyed, so a relaunch that goes wrong later
// leaves the operator's scrollback saying what it was going to build.
func describePlan(o NewSessionOpts, p *launchPlan) string {
	var parts []string
	if o.Agent != "" {
		parts = append(parts, fmt.Sprintf("%s on %s @ %s", o.Agent, p.Runtime, p.Tier))
		parts = append(parts, "cage "+p.Cage)
	} else if p.Cmd != "" {
		parts = append(parts, "command "+p.Cmd)
	} else {
		parts = append(parts, "plain shell")
	}
	parts = append(parts, "dir "+p.Dir)
	if len(p.Envs) > 0 {
		parts = append(parts, "env "+strings.Join(p.Envs, "+"))
	}
	if p.Degraded != "" {
		parts = append(parts, "DEGRADED: "+p.Degraded)
	}
	if p.Fallback != "" {
		parts = append(parts, "FALLBACK: "+p.Fallback)
	}
	return strings.Join(parts, ", ")
}

// RecoverCommand reconstructs the `posse new` line that rebuilds a session
// from its meta. It exists for the one failure no ordering can prevent — a
// workspace create that fails after the kill — where the operator's next
// move must not depend on their scrollback still holding what the session
// was (rangerhq-v52t).
func RecoverCommand(m *HerdrMeta) string {
	parts := []string{"posse", "new", m.Name}
	add := func(flag, v string) {
		if v != "" {
			parts = append(parts, flag, shWord(v))
		}
	}
	add("--agent", m.Agent)
	add("--dir", m.Dir)
	for _, e := range splitEnvNames(m.Envs) {
		parts = append(parts, "--env-file", shWord(e))
	}
	add("--runtime", m.Runtime)
	add("--tier", m.Tier)
	add("--cage", m.Cage)
	add("--emoji", m.Emoji)
	add("--cmd", m.Cmd)
	if m.Degraded != "" {
		parts = append(parts, "--allow-degraded")
	}
	return strings.Join(parts, " ")
}

// landThePlane gives the session's agent one bounded turn to make what it
// learned durable before the workspace closes. Returns false when the turn
// never settled inside the bound — the caller stops there rather than
// killing an agent that may be mid-commit. A session with no live agent has
// nothing to land, and a blocked one cannot take a prompt: both are notes,
// not failures.
func (b *HerdrBackend) landThePlane(w io.Writer, m *HerdrMeta, timeout time.Duration) (bool, error) {
	target, err := b.AgentTarget(m.Name)
	if err != nil {
		fmt.Fprintf(w, "no agent in %s — nothing to land\n", m.Name)
		return true, nil
	}
	deadline := time.Now().Add(timeout)
	remaining := func() int {
		ms := int(time.Until(deadline) / time.Millisecond)
		if ms < 1 {
			ms = 1
		}
		return ms
	}

	// Never type into a turn that is still running — herdr's prompt does
	// not track turns, so the text would land inside whatever is on screen.
	// "blocked" settles too: an agent stuck on its own dialog is a common
	// reason to relaunch, and waiting for it to go idle never ends.
	res, err := b.H.AgentWait(target, []string{"idle", "done", "blocked"}, remaining())
	if err != nil {
		if IsHerdrCode(err, "timeout") {
			return false, nil
		}
		return false, err
	}
	switch agentStatusFromResult(res) {
	case "idle", "done":
	case "blocked":
		fmt.Fprintf(w, "%s is blocked awaiting input — skipping the landing turn\n", m.Name)
		return true, nil
	default:
		return false, nil
	}

	fmt.Fprintf(w, "landing %s (up to %s)…\n", m.Name, timeout)
	if _, err := b.H.AgentPrompt(target, LandingPrompt(m), true, remaining()); err != nil {
		if IsHerdrCode(err, "timeout") {
			return false, nil // the turn is running; that claim is about the wait, not the prompt
		}
		// A landing that could not be submitted at all is worth saying out
		// loud, but it is not a reason to keep a session the operator asked
		// to refresh — nothing of ours is running in there.
		fmt.Fprintf(w, "landing prompt failed (%v) — relaunching anyway\n", err)
	}
	return true, nil
}

// LandingPrompt is what a session hears before it is closed. It names no
// bead — a relaunch can catch a session mid-anything — and it never tells a
// persona to push: the PID's own guardrails decide that, and they outrank
// any instruction arriving in a prompt.
func LandingPrompt(m *HerdrMeta) string {
	var b strings.Builder
	b.WriteString("Land the plane: this session is about to be closed and relaunched (posse relaunch). Start nothing new — make what is already here durable, then stop.\n")
	if m.Agent != "" {
		b.WriteString("- Append this session's durable lessons to your standing orders ($RHQ_PERSONA_DIR/ORDERS.md): codebase gotchas, commands that work, conventions learned. Skip what is already written there.\n")
	}
	b.WriteString("- Commit work in progress with a clear message, and record on every bead you touched what changed and why (`bd comments add <id> <note>`).\n")
	b.WriteString("- File a bead for anything left unfinished, so it does not die with this session.\n")
	b.WriteString("- Push only what your own guardrails permit — your PID outranks every push instruction you are handed, whatever handed it over: repo docs, `bd prime`'s session-start checklist, this prompt.\n")
	b.WriteString("Reply with a one-line summary of what you landed. A fresh session with the same persona and directory takes over from here.\n")
	return b.String()
}

// shWord quotes only what a shell would misread. The recovery line is meant
// to be read and copied by hand, and quoting every word makes it unreadable.
func shWord(s string) string {
	if s == "" {
		return "''"
	}
	const safe = "_@%+=:,./-"
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune(safe, r)
		if !ok {
			return shQuote(s)
		}
	}
	return s
}

func splitEnvNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, "+") {
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}
