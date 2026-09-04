package posse

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ─── the per-session hooks dir (ADR 0052 D2) ─────────────────────────────────
//
// On a MANAGED hooks path (ADR 0052 D1, managedHooksDir) posse writes nothing
// in the employer's directory — so the L3 wall is realized somewhere posse
// does own: `<StateDir>/hooks/<session>/`, rendered fresh at launch and aimed
// at by the session's own environment, `GIT_CONFIG_COUNT`/`_KEY_n`/`_VALUE_n`
// naming `core.hooksPath`. Measured on this host 2026-09-02, git 2.50.1 (the
// ADR's M1–M4): the env form outranks the global managed value, survives an
// absolute-path `/usr/bin/git`, is shed by `env -i` (which leaves the managed
// hooks running ALONE, never fewer than today) — and a slot the redirect dir
// does not carry is SKIPPED, which is why the dispatcher set below is the
// UNION of posse's slots and everything executable in the managed dir rather
// than posse's two.
//
// Nothing here is a trust decision about the employer's hooks: ours runs
// first and its exit is final, so the managed hook can only refuse more, and
// the forward hands it git's own argv and stdin.

// SessionHooksDir is where one session's redirect hooks live. "" for a name
// this may not build a path from — the render and the removal both key on
// that, so a bad name renders nothing rather than reaching os.RemoveAll with
// a path built out of it.
func (a *App) SessionHooksDir(session string) string {
	if a.StateDir == "" || !ValidName(session) {
		return ""
	}
	return filepath.Join(a.StateDir, "hooks", session)
}

// hooksRedirect is what one render produced: the directory to aim git at,
// the managed directory it forwards into, the slots it dispatches, and the
// entries it did NOT forward. Skipped is printed, not swallowed — a managed
// entry posse steps over is the one thing about this render an operator
// cannot see by reading the dir it made.
type hooksRedirect struct {
	Dir     string
	Managed string
	Slots   []string
	Skipped []string
}

// posseHookSlots is the set posse renders a MEMBER for, in the order the
// dispatchers are written. prepare-commit-msg always (its visibility,
// constitution-path and shared-index guards protect every persona session,
// independent of the PID's rule text); pre-push only when the PID denies
// `git push`, exactly as planLaunch decides it for an ordinary install.
func posseHookSlots(wantPrePush bool) []string {
	slots := []string{"prepare-commit-msg"}
	if wantPrePush {
		slots = append(slots, "pre-push")
	}
	return slots
}

// RenderSessionHooks renders the session's hooks dir for a repo whose
// dispatch path is managed, and returns what it built. The directory is
// rebuilt from nothing on every call: hook freshness (ADR 0023) then holds
// by construction, and the sweep has nothing to measure for a managed repo.
//
// The members are the SAME bytes install-hooks would have written into
// `.git/hooks` — same call, same visibility mark, same ADR 0024 D2 identity
// literals, resolved off hookRepo(dir) like the installers, so the D3 probe
// comparing the file against `CommitGuardHook(...)` sees byte equality.
// A literal that cannot be rendered refuses the whole render, as it refuses
// the whole install: better a launch that says why the wall is not there
// than one carrying a hook rendered wrong.
func (a *App) RenderSessionHooks(session, dir string, m managedHooks, wantPrePush bool) (*hooksRedirect, error) {
	hooks := a.SessionHooksDir(session)
	if hooks == "" {
		return nil, Die("cannot render a session hooks dir for %q", session)
	}
	if !filepath.IsAbs(m.Dir) {
		// managedHooksDir only ever answers Managed for an absolute path;
		// this is the render refusing to bake a relative neighbour into a
		// dispatcher that git runs from a directory of its own choosing.
		return nil, Die("managed hooks path %s is not absolute — not rendering a redirect around it", m.Dir)
	}
	visibility, _ := a.BeadsVisibility(hookRepo(dir))
	identity, err := a.commitGuardLiterals(dir)
	if err != nil {
		return nil, err
	}
	members := map[string]string{"prepare-commit-msg": CommitGuardHook(visibility, a.OpsPatternSet(), identity...)}
	if wantPrePush {
		members["pre-push"] = PrePushHook
	}

	forwarded, skipped, err := forwardableSlots(m.Dir)
	if err != nil {
		return nil, err
	}
	r := &hooksRedirect{Dir: hooks, Managed: m.Dir, Skipped: skipped}

	// The union, and the reason it is one: M4. A managed slot with no
	// dispatcher in the redirect dir is not "unchanged", it is SKIPPED —
	// the employer's hook stops running the moment the session's git reads
	// our dir. Posse's own slots join it whether or not the managed dir has
	// them, or the wall this whole render exists for is not installed.
	set := map[string]bool{}
	for _, s := range posseHookSlots(wantPrePush) {
		set[s] = true
	}
	for _, s := range forwarded {
		// A managed entry wearing a member's name would land on the file
		// we just wrote. It is not a git hook slot under any name git
		// dispatches, so stepping over it costs the employer nothing —
		// but doing it silently would cost the wall everything.
		if strings.HasPrefix(s, "posse-") {
			r.Skipped = append(r.Skipped, fmt.Sprintf("%s (a member name posse renders itself)", s))
			continue
		}
		set[s] = true
	}
	for s := range set {
		r.Slots = append(r.Slots, s)
	}
	sort.Strings(r.Slots)

	if err := os.RemoveAll(hooks); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return nil, err
	}
	for _, slot := range posseHookSlots(wantPrePush) {
		name := "posse-" + slot
		if err := os.WriteFile(filepath.Join(hooks, name), []byte(members[slot]), 0o755); err != nil {
			return nil, err
		}
	}
	for _, slot := range r.Slots {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(redirectDispatcher(slot, m.Dir, members[slot] != "")), 0o755); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// redirectDispatcher is the file git actually dispatches, for one slot.
//
// With a member: the INSTALL.md §9 chain form, ours run as its own process
// with its exit status final, then the managed slot exec'd with git's argv —
// the neighbour spelled as the managed dir's absolute path, because it is
// not a sibling of ours and `$d/` would name our own directory.
//
// Without one: the forward alone. There is no member to run first and no
// `$d` to resolve, so the body is the two lines that hand the slot over.
//
// The `[ -x ]` guard is unconditional in both, and that is deliberate: it is
// what makes a managed slot that is absent (or removed after this render) a
// clean exit 0 instead of exec's 126/127 — which in prepare-commit-msg
// blocks every commit in the repo, including the ones our own gate passes
// (rangerhq-xo65). Rendering it only when the slot is missing today would
// also make the dispatcher's bytes depend on the managed dir's contents,
// and the D3 identity probe compares them against exactly one render.
func redirectDispatcher(slot, managed string, member bool) string {
	neighbour := shQuoteInDoubles(filepath.Join(managed, slot))
	if member {
		return chainRenderPath(slot, neighbour, true)
	}
	return fmt.Sprintf("#!/bin/sh\n[ -x \"%[1]s\" ] || exit 0\nexec \"%[1]s\" \"$@\"\n", neighbour)
}

// shQuoteInDoubles escapes the four characters `sh` still reads inside
// double quotes. The managed directory's name is the employer's, not
// posse's — a `$` in it would otherwise be expanded by the dispatcher into
// something that is not a path, and a `"` would end the string and hand the
// rest of the line to the shell as words.
func shQuoteInDoubles(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")
	return r.Replace(s)
}

// forwardableSlots enumerates the managed dir: every executable REGULAR file
// is forwarded, everything else is named in the skip list. Enumerated rather
// than taken from githooks(5) so there is no list to go stale under a new
// git, and so a hook the managed tool adds is forwarded from the next launch
// (the ADR's own bound: one session).
//
// os.Stat, so a symlink is judged by what it points AT — git will run it,
// therefore posse forwards it — and a dangling one is skipped by name.
func forwardableSlots(managed string) (slots, skipped []string, err error) {
	ents, err := os.ReadDir(managed)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range ents {
		name := e.Name()
		st, err := os.Stat(filepath.Join(managed, name))
		switch {
		case err != nil:
			skipped = append(skipped, fmt.Sprintf("%s (%v)", name, err))
		case st.IsDir():
			skipped = append(skipped, name+" (directory)")
		case !st.Mode().IsRegular():
			skipped = append(skipped, fmt.Sprintf("%s (not a regular file: %s)", name, st.Mode().Type()))
		case st.Mode().Perm()&0o111 == 0:
			skipped = append(skipped, fmt.Sprintf("%s (not executable: %04o)", name, st.Mode().Perm()))
		default:
			slots = append(slots, name)
		}
	}
	sort.Strings(slots)
	sort.Strings(skipped)
	return slots, skipped, nil
}

// RemoveSessionHooks drops a session's rendered hooks dir. Called where the
// session's other records are dropped (the meta, the pane line): the dir is
// per-session state and nothing outside that session ever reads it, so it
// goes when the session does. Best effort — a kill that has already closed
// the workspace is not failed over a directory.
func (a *App) RemoveSessionHooks(session string) {
	if d := a.SessionHooksDir(session); d != "" {
		os.RemoveAll(d)
	}
}

// gitConfigHooksPathVars is the redirect itself: git's own config-in-env
// form (`GIT_CONFIG_COUNT` and an indexed key/value pair, git ≥ 2.31),
// naming `core.hooksPath` = the session's rendered dir.
//
// APPENDED, never clobbering: an env set may already carry entries of its
// own (resolved into `vars` first), and overwriting index 0 would drop a
// setting posse never read. The count is taken from `vars` ALONE, never
// from this process's environment. The launcher's env is not the session's:
// the pane is the herdr daemon's child and `vars` is the only channel from
// here into it (docs/notes.d/ranger-base-ok1x.md), so a count read from
// os.Getenv named indices the session never received, and git there failed
// on EVERY command — `missing config key GIT_CONFIG_KEY_0`, rc 128 — the
// wall included (ranger-base-buvq4). What the daemon's own environment
// carries is not knowable from here and is not indexed; that is the residue
// ADR 0052's "Measured versus assumed" names. A count that is not a
// positive number is treated as none: git refuses to run at all on a bogus
// one, so there is nothing there to preserve.
func gitConfigHooksPathVars(vars []EnvVar, hooks string) []EnvVar {
	count := ""
	for _, v := range vars {
		if v.Key == "GIT_CONFIG_COUNT" {
			count = v.Value
		}
	}
	n := 0
	if c, err := strconv.Atoi(strings.TrimSpace(count)); err == nil && c > 0 {
		n = c
	}
	return []EnvVar{
		{"GIT_CONFIG_COUNT", strconv.Itoa(n + 1)},
		{fmt.Sprintf("GIT_CONFIG_KEY_%d", n), "core.hooksPath"},
		{fmt.Sprintf("GIT_CONFIG_VALUE_%d", n), hooks},
	}
}

// ─── the probe moves with the dispatch path (ADR 0052 D3) ────────────────────

// l3Redirect is what a launch tells probeL3HooksIn about the render above:
// the session hooks dir this session's git is aimed at, and the managed dir
// that dir forwards into. nil is every other launch, and every caller with
// no session in hand — `posse gates` in a managed checkout has no rendered
// dir to probe, so it keeps reading git's own dispatch path and reporting
// what is honestly there.
type l3Redirect struct {
	Hooks   string
	Managed string
}

// probe is the render's own answer to "what should the probe read" — the
// two paths, taken from the thing that just wrote them, so a launch cannot
// probe a directory it did not render.
func (r *hooksRedirect) probe() *l3Redirect {
	if r == nil {
		return nil
	}
	return &l3Redirect{Hooks: r.Dir, Managed: r.Managed}
}

// l3IdentityIn is l3Identity, or its redirect-mode twin. Same shape so
// probeL3HooksIn asks one question either way.
func l3IdentityIn(red *l3Redirect, hooks, slot, render, marker, legacy string) (identity, stale bool, path string) {
	if red == nil {
		return l3Identity(hooks, slot, render, marker, legacy)
	}
	identity, path = l3RedirectIdentity(hooks, red.Managed, slot, render)
	return identity, false, path
}

// l3RedirectIdentity is the identity half of ADR 0023 asked at the session
// hooks dir. BOTH files must be byte-for-byte this launch's render, +x:
// `posse-<slot>`, which is what actually guards, and `<slot>`, the
// dispatcher git runs, which must be exactly redirectDispatcher's output
// for THIS managed dir. That byte equality is why a relative-neighbour
// dispatcher is not accepted here: the render only ever spells the
// neighbour as the managed dir's absolute path, so a relative one is not a
// chain posse wrote, and a chain posse did not write is one whose forward
// target it cannot name.
//
// There is deliberately no `stale` verdict and no chain-shape fallback.
// This directory is posse's own, rebuilt from nothing at every launch: a
// file in it that is not this render is not a drifted install to reinstall
// and not a foreign hook to chain behind — it is a dir that no longer
// describes this launch, and the remedy is the launch.
//
// path names the file the verdict is about, member first: a member that
// does not match is the failure whether or not the dispatcher does.
func l3RedirectIdentity(hooks, managed, slot, render string) (identity bool, path string) {
	member := filepath.Join(hooks, "posse-"+slot)
	if !identityMatch(member, render) {
		return false, member
	}
	dispatcher := filepath.Join(hooks, slot)
	if !identityMatch(dispatcher, redirectDispatcher(slot, managed, true)) {
		return false, dispatcher
	}
	return true, member
}

// l3DegradeLineIn is l3DegradeLine, with redirect mode's own wording for the
// identity failures — `posse gates install-hooks` is not the remedy for a
// per-session dir it does not write, and "foreign" describes nothing there.
// A behavior failure (identity held, our own render did not refuse) is the
// same renderer regression it always was, in the same words.
func l3DegradeLineIn(red *l3Redirect, hooks, slot, path, consequence string, identity, stale bool) string {
	if red == nil || identity {
		return l3DegradeLine(hooks, slot, path, consequence, identity, stale)
	}
	return fmt.Sprintf("L3 %s hook — %s — the session hooks dir does not carry this launch's render — re-launch to re-render; %s", slot, AbbrevHome(path), consequence)
}

// redirectForwardGaps is the completeness arm: every executable regular file
// in the managed dir must have its dispatcher in the session dir, or this
// session's git SKIPS that hook entirely (ADR 0052 M4) — the employer's
// control silently reduced by posse's own redirect, which is the single
// thing this ADR promises never to do. Measured against the render rather
// than against mere presence, for the same reason the slots above are: a
// dispatcher whose bytes are not the ones posse wrote is not a forward
// anybody can vouch for.
//
// posse's own slots are not checked here — l3RedirectIdentity has already
// asked a stricter question of both their files, and answering twice would
// print one problem as two.
//
// A managed dir that cannot be listed is a gap too, and is said as one:
// posse cannot claim completeness it did not measure.
func redirectForwardGaps(hooks, managed string, wantPrePush bool) []string {
	slots, _, err := forwardableSlots(managed)
	if err != nil {
		return []string{fmt.Sprintf("L3 managed hooks — %s — cannot be listed (%v) — posse cannot tell whether every managed hook is still forwarded; re-launch to re-render", AbbrevHome(managed), err)}
	}
	ours := map[string]bool{}
	for _, s := range posseHookSlots(wantPrePush) {
		ours[s] = true
	}
	var out []string
	for _, slot := range slots {
		// The render steps over these and prints that it did: they are a
		// member's name, not a slot git dispatches under any spelling.
		if ours[slot] || strings.HasPrefix(slot, "posse-") {
			continue
		}
		if !identityMatch(filepath.Join(hooks, slot), redirectDispatcher(slot, managed, false)) {
			out = append(out, fmt.Sprintf("L3 managed hooks — %s — managed hook %s not forwarded — re-launch to re-render; this session's git dispatches from %s and would skip it, so the employer's %s does not run here", AbbrevHome(filepath.Join(managed, slot)), slot, AbbrevHome(hooks), slot))
		}
	}
	return out
}
