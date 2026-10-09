package posse

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ranger360ai/posse"
)

// The upstream detection filing, as a thing a RELEASE BINARY can hand over
// (ranger-base-mhv7j).
//
// ADR 0060 D2 is one sentence long: posse ships the filing, the operator
// sends it. Every door a detection refusal prints said so — "posse ships the
// filing at etc/herdr/agent-detection/upstream-bob.md and the operator sends
// it" — and on the box the sentence is read on, that path did not exist.
// scripts/release-artifacts.sh tars `posse`, README.md and INSTALL.md; the
// bottle's keg is bin/posse plus those two docs. So a `brew install` posse
// refused a dispatched bob seat (agent_not_found, `posse prompt` refuses an
// unlabelled pane) and then named, as the only way out, a file nobody on that
// box had. `go install` is the same story for the same reason: it fetches a
// binary, not repo files.
//
// WHY THE VERB AND NOT THE OTHER TWO OPTIONS the operator left open on
// issue #4. "Ship etc/ in the bottle" moves the file onto the box but not to
// the path the sentence names — it would land under
// `$(brew --prefix)/share/posse/`, so the door becomes environment-dependent
// prose, and the keg would have to disagree with the formula's own
// `def install` (which tapformula_qa_test.go pins). "Say the file lives in
// the source tree at <url>" is honest and leaves the operator a clone away
// from the material. Embedding is the route posse already took for the one
// other repo tree a release binary needs (ADR 0012 D5, examples/ and
// `posse init`), it is the only one that works identically on a checkout, a
// bottle, a tarball and a `go install`, and it makes the door's claim —
// posse CARRIES this — true of the binary rather than of the repo.
//
// THE PIN IS WHAT KEEPS IT TRUE: detectiondoor_qa_test.go now holds a
// declared DetectionFiling against the EMBEDDED tree as well as the checked
// out one, because the embedded copy is the one every reader of this door
// has.

// FilingRoot is the repo-relative directory the embedded filings come from,
// with the trailing slash, so a repo-relative DetectionFiling is turned into
// a path inside posse.Filings by trimming exactly this prefix and nothing
// else. detectionFilingDoc (detection.go) is a path under the same directory;
// both are spelled out in full where they are read, because a reader chasing
// a door sentence greps for the whole path.
const FilingRoot = "etc/herdr/agent-detection/"

// FilingPackageDir is where the material that goes WITH a filing lives,
// inside posse.Filings: `upstream/<runtime>/`, the draft manifest and the
// pane snapshots (etc/herdr/agent-detection/README.md, "upstream/ — filing
// packages, not overrides").
//
// Rendered from the runtime's name, and that is not the ranger-base-ecchw
// mistake wearing a new hat: what that bead cost was a door PROMISING a
// path, in a sentence an operator was told to act on, that nothing had
// checked. Here the name only decides where to LOOK, and a filing whose
// package directory does not exist simply has no attachments — which is the
// true answer for three of the five filings this binary carries
// (upstream-report.md and its two herdr-bob siblings are covering notes with
// nothing to attach). Nothing is promised that was not first read.
func FilingPackageDir(name string) string { return path.Join("upstream", name) }

// FilingFor resolves a runtime's filing package out of the binary: the
// covering note's path inside posse.Filings, and the attachment paths beside
// it, sorted. It reads the EMBEDDED tree and never the working one — a
// release binary has no working one, and a sentence that is true only inside
// a checkout is the defect this whole file closes.
func FilingFor(rt *Runtime) (filing string, attach []string, err error) {
	if rt.DetectionFiling == "" {
		// Said as the door says it, because this is the same fact one verb
		// over: a built-in posse carries no filing for is not a filing
		// waiting to be printed, it is a herdr upgrade.
		return "", nil, Die("posse carries no upstream detection filing for %s: %s", rt.Name, DetectionDoor(rt))
	}
	rel := strings.TrimPrefix(rt.DetectionFiling, FilingRoot)
	if rel == rt.DetectionFiling {
		// A declaration that is not under the filings directory cannot be
		// in the embedded tree, whatever it is in the checkout. Loud, not
		// swallowed: the declaration is what makes the door true.
		return "", nil, Die("%s declares DetectionFiling %q, which is not under %s — the embedded filings are rooted there, so this binary cannot hand that file to anybody (ranger-base-mhv7j)",
			rt.Name, rt.DetectionFiling, FilingRoot)
	}
	if _, err := fs.Stat(posse.Filings, rel); err != nil {
		return "", nil, Die("%s declares DetectionFiling %q and this binary does not carry it — that is the ranger-base-mhv7j defect, not a bad argument: rebuild from a tree that has the file, or `git -C <checkout> show HEAD:%s`",
			rt.Name, rt.DetectionFiling, rt.DetectionFiling)
	}
	dir := FilingPackageDir(rt.Name)
	ents, dirErr := fs.ReadDir(posse.Filings, dir)
	if dirErr != nil {
		// No package directory: the filing stands alone. Not an error.
		return rel, nil, nil
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		attach = append(attach, path.Join(dir, e.Name()))
	}
	sort.Strings(attach)
	return rel, attach, nil
}

// CmdRuntimeFiling is `posse runtime filing <name> [--out <dir>]`.
//
// With no --out the covering note goes to out ALONE, so the verb composes —
// `posse runtime filing bob > bob.md`, or straight into a pager — and
// everything about it that is not the filing goes to note. That split is the
// point: the operator's next move is to send this text, and a verb that
// mixed its own prose into stdout would make him edit it out first.
//
// --out writes the whole package, because the filing is not only prose: ADR
// 0060 D2's draft manifest and its redacted pane snapshots are what make the
// ask actionable, and they are .txt screens nobody can send down a pipe.
func (a *App) CmdRuntimeFiling(rt *Runtime, outDir string, out, note io.Writer) error {
	filing, attach, err := FilingFor(rt)
	if err != nil {
		return err
	}
	if outDir == "" {
		b, err := fs.ReadFile(posse.Filings, filing)
		if err != nil {
			return err
		}
		if _, err := out.Write(b); err != nil {
			return err
		}
		fmt.Fprintf(note, "\n%s — the upstream detection filing this binary carries (%s in a checkout, ADR 0060 D2)\n",
			path.Base(filing), rt.DetectionFiling)
		switch len(attach) {
		case 0:
			fmt.Fprintf(note, "  nothing to attach: this filing stands alone\n")
		default:
			fmt.Fprintf(note, "  %d file(s) go WITH it — the draft manifest and the pane snapshots:\n", len(attach))
			for _, p := range attach {
				fmt.Fprintf(note, "    %s\n", path.Join(FilingRoot, p))
			}
			fmt.Fprintf(note, "  to write the whole package out: posse runtime filing %s --out <dir>\n", rt.Name)
		}
		return nil
	}
	return a.writeFilingPackage(rt, filing, attach, outDir, out)
}

// filingOutRefusal names what --out may be handed, in the one place both the
// guard and its own message read it from.
const filingOutRefusal = "--out may name only a directory that is absent, empty, or holds nothing but a previous run's own output"

// writeFilingPackage lays the package down under outDir.
//
// THE GUARD IS THE SAME ONE scripts/release-artifacts.sh AND
// scripts/tap-formula.sh CARRY, for the same reason and with the same
// deliberate absence of a --force: --out is typed by hand, it is one flag
// away from a path that is somebody's work, and the way to overwrite a
// directory of yours is to empty it yourself. What differs is how small it
// can be here — this verb writes a known set of file NAMES and never
// removes anything, so "absent, empty, or holding nothing but those names"
// is the whole of it, and a stray file is named rather than counted.
func (a *App) writeFilingPackage(rt *Runtime, filing string, attach []string, outDir string, out io.Writer) error {
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	// Every path this run would write, keyed by its basename — the package
	// is flattened on purpose: what the operator does with this directory is
	// attach its contents to one message, and `upstream/bob/idle-composer.txt`
	// arrives as `idle-composer.txt` either way.
	want := map[string]string{path.Base(filing): filing}
	for _, p := range attach {
		if other, dup := want[path.Base(p)]; dup {
			// Unreachable today (one note, one flat package dir) and worth
			// refusing anyway: flattening two files onto one name would
			// silently send one of them twice.
			return Die("the %s filing package holds two files named %s (%s and %s) — flattening them into one directory would drop one, so this verb refuses rather than choose",
				rt.Name, path.Base(p), other, p)
		}
		want[path.Base(p)] = p
	}
	if st, err := os.Stat(abs); err == nil {
		if !st.IsDir() {
			return Die("posse runtime filing refuses --out %s: it exists and is not a directory\n  %s", AbbrevHome(abs), filingOutRefusal)
		}
		ents, err := os.ReadDir(abs)
		if err != nil {
			return err
		}
		for _, e := range ents {
			if _, ours := want[e.Name()]; ours && !e.IsDir() {
				continue
			}
			return Die("posse runtime filing refuses to write into %s: it holds %q, which this verb did not put there\n  %s\n  if that directory is yours to lose, empty it yourself and re-run",
				AbbrevHome(abs), e.Name(), filingOutRefusal)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		b, err := fs.ReadFile(posse.Filings, want[n])
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(abs, n), b, 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "wrote the %s upstream detection filing to %s (%d files, ADR 0060 D2)\n", rt.Name, AbbrevHome(abs), len(names))
	for _, n := range names {
		fmt.Fprintf(out, "  %-40s %s\n", n, path.Join(FilingRoot, want[n]))
	}
	fmt.Fprintf(out, "send %s with the rest attached; nothing here has been published (%sREADME.md)\n", path.Base(filing), FilingRoot)
	return nil
}
