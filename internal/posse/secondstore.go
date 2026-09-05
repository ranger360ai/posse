package posse

// ─── a second store beside the redirect, named while it is still latent ──────
//
// ADR 0012 D3, the September 2026 adherence audit's finding 6
// (ranger-base-dj3k2, from ranger-base-4wxko).
//
// D3 bought ONE store of record: the bd database lives in the instance repo,
// and the public working copy's `.beads/` holds a redirect to it — "a second
// mount point, not a second store". The ADR names the shape it rejected by
// name too: "a gitignored *local* `.beads/` inside the public tree (a second,
// unversioned queue store)".
//
// The audit found that shape sitting in the public checkout anyway: a
// beads.db from 2026-08-24 beside the redirect, its shared-memory file
// touched that morning. Nothing was wrong that day, because bd resolves the
// redirect first (beadsHome, beadloss.go) — and nothing anywhere would have
// said so on the day the redirect file was lost, when a three-week-old graph
// would have answered every `bd ready` at exit 0. A wrong graph that exits 0
// is the whole failure: the loop dispatches, the beads it names are stale,
// and every surface reads as a working shop.
//
// So this file computes one fact from two stats and a directory read, and
// renders it as a line the `dispatch --watch` pass preamble and `posse
// status` print byte-for-byte, so one grep finds both.
//
// **It reports.** It refuses nothing, deletes nothing, and moves no exit
// code — stated here so a later edit has to argue with it. A store this
// names may be the one thing standing between the operator and a lost graph
// (the redirect target could be the half that went away), and posse deleting
// a database nobody asked it to delete is a worse incident than the one this
// prevents. The remedy is one `rm` an operator types.
//
// The redirect half is load-bearing in BOTH directions:
//
//   - No redirect at all is not a finding. A repo whose `.beads/` holds its
//     own database is every ordinary bd repo, this instance's queue included.
//   - A redirect bd will NOT follow — absent target, empty file, a special
//     file — is a finding with a different sentence, because there bd is
//     already reading the local store. That is not the latent case; it is the
//     case the audit was worried about, happening now.

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// SecondStore is one configured `beads:` directory that holds a bd store of
// its own beside a redirect. Values, not prose, so a test can assert what was
// found without reading it back out of the sentence.
type SecondStore struct {
	// Dir is the `beads:` entry, expanded; Home is its `.beads`, the
	// directory the second store is IN and the path the line names.
	Dir  string
	Home string
	// Target is where bd follows the redirect to, and "" when bd will not
	// follow it — in which case Why says what stopped it. Exactly one of the
	// two is set, and which one it is chooses the sentence.
	Target string
	Why    string
	// Files are the basenames in Home that would answer a bd query: any
	// `*.db`, and `issues.jsonl` when it carries a byte. In directory order,
	// which os.ReadDir already sorts.
	Files []string
}

// SweepSecondStores walks every configured `beads:` directory and reports the
// ones holding a store beside a redirect. Read-only: three stats, one read of
// the redirect and one directory listing per entry.
//
// Deduplicated by resolved `.beads` path, because two spellings of one
// checkout are one store and would otherwise be two findings naming the same
// file. Reported in config order.
func (a *App) SweepSecondStores() []SecondStore {
	var out []SecondStore
	seen := map[string]bool{}
	for _, dir := range a.BeadsDirs() {
		home, target, why := beadsRedirectHop(dir)
		if target == "" && why == "" {
			// No redirect file: an ordinary bd repo, whose database is
			// the store of record and not a second one.
			continue
		}
		real := resolvedPath(home)
		if seen[real] {
			continue
		}
		seen[real] = true
		// A redirect that resolves back to the directory holding it is one
		// store reached by a pointless hop, not two. bd reads exactly the
		// files below, so naming them as a store to delete would be an
		// instruction to delete the store of record.
		if target != "" && resolvedPath(target) == real {
			continue
		}
		files := storeFilesIn(home)
		if len(files) == 0 {
			continue
		}
		out = append(out, SecondStore{Dir: dir, Home: home, Target: target, Why: why, Files: files})
	}
	return out
}

// storeFilesIn is what in a `.beads` directory would answer a bd query.
//
// A database counts by PRESENCE, whatever its size: an empty beads.db is
// still the file bd opens, and it answers "no beads" at exit 0, which is the
// same lie as a stale graph told more quietly. An `issues.jsonl` counts only
// when it carries a byte, because an empty projection is what an export into
// a fresh directory leaves behind and it carries no graph to be wrong about.
//
// `*.db` rather than the literal `beads.db` the rest of this package joins:
// the literal is what bd names its own store here, and a differently named
// database in this directory is a store this check exists to see. os.ReadDir
// returns entries sorted by filename, so the order is stable without a sort.
func storeFilesIn(home string) []string {
	ents, err := os.ReadDir(home)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".db"):
			out = append(out, name)
		case name == beadsJSONL:
			if fi, err := e.Info(); err == nil && fi.Size() > 0 {
				out = append(out, name)
			}
		}
	}
	return out
}

// Line is the one rendering — the watch pass preamble and `posse status`
// print these same bytes.
//
// Two sentences, because there are two states and the remedy differs. The
// latent one says the redirect is holding today and this store answers the
// day it stops; the live one says bd is reading this store NOW, and names
// fixing the redirect first — deleting the local store under a redirect that
// resolves to nothing leaves bd with no graph at all.
func (s SecondStore) Line() string {
	files := strings.Join(s.Files, ", ")
	if s.Target != "" {
		return fmt.Sprintf("second store: %s holds %s beside a redirect to %s — bd follows the redirect today and this store answers the day it is lost; delete it (ADR 0012 D3)",
			AbbrevHome(s.Home), files, AbbrevHome(s.Target))
	}
	return fmt.Sprintf("second store: %s holds %s beside a redirect bd will NOT follow (%s) — bd is reading THIS store now; repair the redirect, then delete it (ADR 0012 D3)",
		AbbrevHome(s.Home), files, s.Why)
}

// SecondStoreLines renders a sweep for a surface to print. nil for a clean
// sweep, and every caller is silent on nil: a shop with one store of record
// is the designed state, and a standing "no second store" line on every pass
// of a loop that runs for ten hours is furniture, which is how the lines
// beside it stop being read.
func SecondStoreLines(ss []SecondStore) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Line())
	}
	return out
}

// ReportSecondStores prints the sweep for this instance and reports whether
// it printed anything. The bool is for a test and for a caller that wants to
// know it said something — deliberately NOT a verdict: no caller of this may
// branch its behaviour on it (TestQASecondStoreReportsAndRefusesNothing).
func (a *App) ReportSecondStores(w io.Writer) bool {
	lines := SecondStoreLines(a.SweepSecondStores())
	for _, ln := range lines {
		fmt.Fprintln(w, ln)
	}
	return len(lines) > 0
}
