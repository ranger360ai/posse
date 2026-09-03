package posse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ─── the hook wall, swept across every repo config declares ──────────────────
//
// ranger-base-ixv4. The L3 hook bodies are compiled into this binary, so every
// hook on the box is a COPY that was correct when it was written, and only two
// things re-render one: `posse gates install-hooks`, typed by hand, and a
// session create — which refreshes the COMMON hooks dir of the repo the
// session was cut from and no other. A repo that never holds a session is
// re-rendered by nothing at all.
//
// probeL3Hooks already asks the identity∧behavior question of ADR 0023, but it
// asks it only of the repo a session is launching into. That is the wall
// existing exactly where sessions launch, which is the hole this sweep closes:
// ~/src/ranger-base holds the constitution and holds no session, and its
// prepare-commit-msg waved a promoted-class commit through hours after the
// constitution arm shipped because that copy predated it.
//
// The sweep asks the same question of every repo `beads_visibility:` names —
// the operator's own declaration of which repos posse guards, and the same
// list `scripts/verify-hook-freshness.sh` walks. It reads; it repairs nothing.
// A hook rewrite in a shared checkout is a change someone should type, and the
// finding prints the command that types it.
//
// It runs from the two places that fire without anyone deciding to check:
// `posse promote`'s epilogue (the operator's constitution touch point) and the
// `dispatch --watch` preamble (once per loop — the loop IS a binary, so loop
// start is the first moment a newly installed render could disagree with what
// is on disk).

// HookWallRepo is one configured repo's verdict.
type HookWallRepo struct {
	// Config is the key exactly as config spelled it; Dir is that path
	// expanded. Findings quote Config, so the operator can find the line.
	Config string
	Dir    string
	// Skip names why nothing was measured here ("absent", "not a git
	// repository", or the managed-hooks line), and is empty when the repo
	// was measured.
	Skip string
	// Managed is the one Skip the operator is meant to read rather than
	// ignore: git dispatches this repo's hooks from a path posse may not
	// write (ADR 0052 D1), so there is no copy here to be stale.
	Managed bool
	// Degraded holds ready-to-display lines, one per slot that does not
	// count. Empty for a repo whose wall is this binary's render.
	Degraded []string
}

// HookWallSweep is what one pass over the configured repos found.
type HookWallSweep struct {
	Repos    []HookWallRepo
	Declared int // entries in beads_visibility:
	Measured int // of those, present and git
	Findings int // of those measured, carrying at least one degraded slot
	Managed  int // of those present and git, dispatching from a managed path
}

// SweepHookWall asks the ADR 0023 question — identity at the dispatch path,
// behavior of our own render — of every repo `beads_visibility:` names.
//
// Repos are deduplicated by resolved path (two spellings of one checkout are
// one wall) and reported in config order. A repo that is absent or is not a
// git repository is recorded as skipped, never as a finding: config outliving
// a checkout is an ordinary thing and not evidence about any wall.
func (a *App) SweepHookWall() HookWallSweep {
	var s HookWallSweep
	seen := map[string]bool{}
	for _, kv := range YamlMapPairs(a.ConfigPath, "beads_visibility") {
		key := strings.TrimSpace(kv[0])
		if key == "" {
			continue
		}
		// Two spellings of one checkout are one wall, and one entry in
		// the counts: a duplicate key that inflated Declared would make
		// "0 of N present" name a repo that was never a separate repo.
		dir := absResolve(ExpandTilde(key))
		real := resolvedPath(dir)
		if seen[real] {
			continue
		}
		seen[real] = true
		s.Declared++
		r := HookWallRepo{Config: key, Dir: dir}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			r.Skip = "absent — no such directory"
			s.Repos = append(s.Repos, r)
			continue
		}
		// ADR 0052 D1, asked before the probe rather than after it: on a
		// managed hooks path the two slots hold the employer's hooks, which
		// the probe reads — correctly — as foreign, and would report as two
		// degraded slots with `posse gates install-hooks` as the remedy. That
		// remedy is a write posse refuses to attempt there, so the finding
		// would be a standing instruction to do the one thing this ADR says
		// not to do. A managed repo is a SKIP with the same line every other
		// caller prints, and is not a stale wall: nothing of posse's is there
		// to go stale.
		if m, err := managedHooksDir(dir); err == nil && m.Managed {
			r.Skip, r.Managed = m.line(), true
			s.Managed++
			s.Repos = append(s.Repos, r)
			continue
		}
		p := a.probeL3Hooks(dir, true)
		if !p.Repo {
			r.Skip = "not a git repository"
			s.Repos = append(s.Repos, r)
			continue
		}
		s.Measured++
		if !p.CommitGuard {
			r.Degraded = append(r.Degraded, hookWallLine(p.HooksDir, "prepare-commit-msg", key, p.CommitGuardDegraded))
		}
		if !p.PrePush {
			r.Degraded = append(r.Degraded, hookWallLine(p.HooksDir, "pre-push", key, p.PrePushDegraded))
		}
		if len(r.Degraded) > 0 {
			s.Findings++
		}
		s.Repos = append(s.Repos, r)
	}
	return s
}

// hookWallLine keeps l3DegradeLine's wording for every case it describes
// correctly, and replaces the one it does not. A slot with no file at all
// reaches l3Identity as "neither ours nor a chain dispatcher" and so prints
// as "foreign hook — posse cannot vouch for a hook it did not write", which
// is true of a hook that exists. Here the common case is a repo the operator
// declared and never installed into, where naming a foreign hook that is not
// there sends the reader looking for a file to inspect.
func hookWallLine(hooks, slot, repo, degraded string) string {
	if hooks != "" {
		_, topErr := os.Stat(filepath.Join(hooks, slot))
		_, memberErr := os.Stat(filepath.Join(hooks, "posse-"+slot))
		if topErr != nil && memberErr != nil {
			return fmt.Sprintf("L3 %s hook — %s — no hook installed at all; run `posse gates install-hooks %s`",
				slot, AbbrevHome(filepath.Join(hooks, slot)), repo)
		}
	}
	return degraded
}

// ReportHookWall prints the sweep for the operator and reports whether it
// printed a finding. Silent when config declares no repo at all: an instance
// with no `beads_visibility:` block has made no claim for this to check, and a
// line about it in every promote and every watch loop is noise about nothing.
//
// where names the caller in the clean line, because the whole point of this
// bead is that the check now fires somewhere; a reader who cannot tell it ran
// is back to trusting that it did.
func (a *App) ReportHookWall(w io.Writer, where string) bool {
	s := a.SweepHookWall()
	if s.Declared == 0 {
		return false
	}
	if s.Measured == 0 && s.Managed == 0 {
		fmt.Fprintf(w, "hook wall (%s): 0 of %d repo(s) in config beads_visibility: are present and git — nothing measured\n", where, s.Declared)
		return false
	}
	if s.Measured > 0 {
		if s.Findings == 0 {
			fmt.Fprintf(w, "hook wall (%s): %d repo(s) carry this binary's render\n", where, s.Measured)
		} else {
			fmt.Fprintf(w, "hook wall (%s): %d of %d repo(s) do NOT carry this binary's render\n", where, s.Findings, s.Measured)
			for _, r := range s.Repos {
				if len(r.Degraded) == 0 {
					continue
				}
				fmt.Fprintf(w, "  %s\n", r.Config)
				for _, d := range r.Degraded {
					fmt.Fprintf(w, "    %s\n", d)
				}
			}
			fmt.Fprint(w, "  nothing re-renders a repo that holds no session, so a stale wall there stays stale:\n"+
				"  re-render each from this binary and re-run `make verify-hook-freshness` in the posse checkout.\n")
		}
	}
	// Printed rather than counted silently: a repo the operator declared and
	// posse deliberately does not install into is exactly the thing a reader
	// would otherwise go looking for a missing finding about.
	if s.Managed > 0 {
		fmt.Fprintf(w, "hook wall (%s): %d repo(s) dispatch from a managed hooks path — posse writes nothing there (ADR 0052)\n", where, s.Managed)
		for _, r := range s.Repos {
			if r.Managed {
				fmt.Fprintf(w, "  %s\n    %s\n", r.Config, r.Skip)
			}
		}
	}
	return s.Findings > 0
}
