//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-ecchw — the door a detection refusal hands the
// operator has to name a filing the tree actually holds.
//
// THE ESCAPE. DetectionDoor's built-in arm rendered its path from the
// runtime's NAME: `etc/herdr/agent-detection/upstream-<rt.Name>.md`, for
// every built-in. The tree ships upstream-bob.md and nothing else, so on
// claude, codex and grok all four surfaces that print this door — the launch
// refusal, the re-type refusal, `posse runtime check`'s ✗ detection gap and
// the interactive DEGRADED warn — named a path that is not there. It escaped
// ranger-base-d8riq (the arm is new in cfc877f5) and ranger-base-enmu2
// inherited it; ranger-base-3hf2r's verify caught it.
//
// WHY IT COULD SIT THERE GREEN. This door is unreachable on a box whose
// herdr has a manifest for the argv0, which on 2026-09-28 is claude, codex
// and grok — so only bob took the branch and only bob's filing was ever
// named. It becomes reachable on an older or trimmed herdr, which is exactly
// the box where the sentence is the whole remedy. A path inside a string
// constant is checked by nothing.
//
// It is the same class ranger-base-vh2o8 closed a bead over — copy that
// hands a seat a path, with a pin that the path exists
// (internal/treepins/mergeblockedrunbook_qa_test.go). This is that pin for
// this door, and it reads the REAL functions over the REAL built-ins rather
// than re-deriving the sentence: what a seat is handed is what is checked.
//
// THE SECOND ESCAPE, and why this pin grew a second reading
// (ranger-base-mhv7j, github issue #4). "Is the filing in the tree" is the
// right question in a checkout and the wrong one everywhere else: the
// release tarball and the Homebrew bottle carry `posse` plus README.md and
// INSTALL.md, so a brew-installed posse refused a dispatched bob seat and
// named, as the whole remedy, a repo path that did not exist on that
// machine. Green here the entire time — the file IS in the tree, and the
// tree is not what the reader of this door has. So the filing is embedded
// now and this pin reads BOTH: the checkout, because a persona standing in
// one follows the path, and posse.Filings, because that is the copy every
// release has. A filing in one and not the other is the defect.
//
// Doored as `make doc-check` ($(QA_DOC_PINS)) — a tree-wide pin living in a
// package nobody runs whole, so no seat's `-run` filter would ever name it
// (ranger-base-rulbl, ranger-base-ik44f).

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ranger360ai/posse"
)

// dddFilingCite matches an agent-detection filing path as one of these
// sentences writes it. Anchored on the directory, because that is the part
// the door promises and the part a rename moves.
var dddFilingCite = regexp.MustCompile(`etc/herdr/agent-detection/[A-Za-z0-9._-]+\.md`)

// dddMissing returns the cited paths that are not files under root. Split
// from the test so the control below can point it at a tree where the answer
// is known — and it takes the root as an argument for the same reason: a
// helper that found the repo root itself would be unusable from the control.
func dddMissing(root string, cites []string) []string {
	var out []string
	for _, c := range cites {
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(c))); err != nil || st.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

// dddUncarried returns the cited paths the BINARY does not carry — the
// reading the tree cannot answer. It takes the filesystem as an argument for
// the reason dddMissing takes a root: the control below points it at a tree
// where the answer is known.
func dddUncarried(filings fs.FS, cites []string) []string {
	var out []string
	for _, c := range cites {
		rel := strings.TrimPrefix(c, FilingRoot)
		if rel == c {
			// Not under the filings root at all, so no embed directive
			// could be carrying it however it reads on disk.
			out = append(out, c)
			continue
		}
		if st, err := fs.Stat(filings, rel); err != nil || st.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

func TestQADetectionDoorCitesAFilingThatExists(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)

	for i := range builtinRuntimes {
		rt := builtinRuntimes[i]
		// Every surface that prints the door, so none of them can carry a
		// path the others do not. The reading is the one that refuses: no
		// manifest for the argv0, nothing declared to label the pane.
		r := ManifestReading{Argv0: rt.Exe(), State: ManifestUnknownAgent, Declared: rt.DetectionMode()}
		for surface, line := range map[string]string{
			"DetectionRefusal":       DetectionRefusal(&rt, r).Error(),
			"DetectionRetypeRefusal": DetectionRetypeRefusal(&rt, r, "s1").Error(),
			"DetectionGapLine":       DetectionGapLine(&rt, r),
			"DetectionDegraded":      DetectionDegraded(&rt, r),
		} {
			cites := dddFilingCite.FindAllString(line, -1)
			for _, c := range dddMissing(root, cites) {
				t.Errorf("%s: %s hands the operator %s, which is NOT in the tree — and this door is only ever read on a box whose herdr lacks the manifest, where the sentence is the whole remedy (ranger-base-ecchw):\n%s",
					rt.Name, surface, c, line)
			}
			// And the same path against the BINARY. A release has no
			// checkout: the reader of this line typed `brew install posse`
			// (ranger-base-mhv7j). README.md is not a filing and is not
			// embedded — the no-filing branch cites it as the authoring
			// page, so it is excluded by name rather than by letting the
			// check go quiet on everything.
			var carried []string
			for _, c := range cites {
				if path.Base(c) != "README.md" {
					carried = append(carried, c)
				}
			}
			for _, c := range dddUncarried(posse.Filings, carried) {
				t.Errorf("%s: %s hands the operator %s, which THIS BINARY does not carry — on a brew install there is no checkout and that sentence is the whole remedy, which is the ranger-base-mhv7j defect:\n%s",
					rt.Name, surface, c, line)
			}
		}

		door := DetectionDoor(&rt)
		// The declaration and the door are one fact: a filing posse says it
		// carries must be named as the way out, and a built-in posse carries
		// none for must not be handed a path at all.
		switch {
		case rt.DetectionFiling != "":
			if !strings.Contains(door, rt.DetectionFiling) {
				t.Errorf("%s declares DetectionFiling %q and its door does not name it, so the filing posse carries is one no refusal sends anybody to:\n%s", rt.Name, rt.DetectionFiling, door)
			}
			if m := dddMissing(root, []string{rt.DetectionFiling}); len(m) != 0 {
				t.Errorf("%s declares DetectionFiling %q, which is not a file in the tree — the declaration is what makes the door true, so it is the thing that has to be real", rt.Name, rt.DetectionFiling)
			}
			// Carried, and reachable through the verb the door now names —
			// the two halves of what a release binary can actually hand
			// over (ranger-base-mhv7j). FilingFor is the production reader,
			// so this is the real route and not a second implementation of
			// it: a filing that is declared, in the tree, and absent from
			// the embed directives fails HERE rather than on the operator's
			// laptop.
			if m := dddUncarried(posse.Filings, []string{rt.DetectionFiling}); len(m) != 0 {
				t.Errorf("%s declares DetectionFiling %q and the binary does not carry it — add it to embed.go's filings directives, or the release that prints this door has no filing behind it (ranger-base-mhv7j)", rt.Name, rt.DetectionFiling)
			}
			if _, _, err := FilingFor(&rt); err != nil {
				t.Errorf("%s declares DetectionFiling %q and `posse runtime filing %s` refuses it: %v", rt.Name, rt.DetectionFiling, rt.Name, err)
			}
			if verb := "posse runtime filing " + rt.Name; !strings.Contains(door, verb) {
				t.Errorf("%s carries a filing and its door does not name %q — the PATH is only true in a checkout, so the command is the part a brew install can act on (ranger-base-mhv7j):\n%s", rt.Name, verb, door)
			}
		default:
			// The escape itself, stated as the property rather than as the
			// symptom: no built-in's door may be built out of its NAME.
			if strings.Contains(door, "upstream-") {
				t.Errorf("%s carries no filing and its door names an upstream-* path anyway — that is the name-rendered sentence ranger-base-ecchw was filed for:\n%s", rt.Name, door)
			}
			// And the reader still gets somewhere: the version check and the
			// page that says how a filing is written.
			for _, want := range []string{"herdr --version", detectionFilingDoc} {
				if !strings.Contains(door, want) {
					t.Errorf("%s carries no filing, so its door is the only route left, and it does not name %q:\n%s", rt.Name, want, door)
				}
			}
		}
	}
}

// The control: the scanner and the existence check must both be able to say
// no. Without it this is a green function nobody has seen refuse anything —
// a citation regexp that matches nothing, or a stat that never fails, reads
// exactly like a tree whose doors are all true.
//
// It takes its own scratch root rather than the repo's, so it is not a
// tree-wide pin and needs no door of its own.
func TestQADetectionDoorFilingCheckCanFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const gone = "etc/herdr/agent-detection/upstream-no-such-runtime.md"

	if got := dddFilingCite.FindAllString("posse ships the filing at "+gone+" and the operator sends it.", -1); len(got) != 1 || got[0] != gone {
		t.Errorf("the citation scanner does not read a filing path out of a door sentence: %q", got)
	}
	if got := dddFilingCite.FindAllString("check `herdr --version` and upgrade", -1); len(got) != 0 {
		t.Errorf("the citation scanner invents a citation: %q", got)
	}
	if got := dddMissing(root, []string{gone}); len(got) != 1 {
		t.Errorf("a filing that is not in the tree reads as present: %q", got)
	}
	// And a path that IS there passes, so the check is not simply always red.
	real := filepath.Join(root, "etc", "herdr", "agent-detection")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "upstream-no-such-runtime.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dddMissing(root, []string{gone}); len(got) != 0 {
		t.Errorf("a filing that IS in the tree reads as missing: %q", got)
	}
	// The directory it lives in is not a filing: a check that accepted one
	// would pass a tree where every upstream-*.md had been replaced by a dir.
	if got := dddMissing(root, []string{"etc/herdr/agent-detection"}); len(got) != 1 {
		t.Errorf("a directory reads as a filing: %q", got)
	}

	// The EMBEDDED reading needs its own control, and more than the other
	// one does: it is the check whose green was the ranger-base-mhv7j
	// defect. A fs.Stat that never fails, or a prefix trim that silently
	// matched everything, reads exactly like a binary that carries the lot.
	scratch := fstest.MapFS{
		"upstream-no-such-runtime.md": {Data: []byte("x\n")},
		"upstream/x/bob.toml":         {Data: []byte("x\n")},
	}
	if got := dddUncarried(scratch, []string{gone}); len(got) != 0 {
		t.Errorf("a filing the filesystem DOES carry reads as uncarried: %q", got)
	}
	if got := dddUncarried(scratch, []string{"etc/herdr/agent-detection/upstream-absent.md"}); len(got) != 1 {
		t.Errorf("a filing nothing carries reads as carried: %q", got)
	}
	// A path outside the filings root cannot be carried by any directive,
	// however it reads on disk — that is the refusal FilingFor makes too.
	if got := dddUncarried(scratch, []string{"docs/runbooks/agent-detection-manifest.md"}); len(got) != 1 {
		t.Errorf("a path outside %s reads as carried: %q", FilingRoot, got)
	}
	// And a directory is not a filing here either.
	if got := dddUncarried(scratch, []string{"etc/herdr/agent-detection/upstream"}); len(got) != 1 {
		t.Errorf("a directory reads as a carried filing: %q", got)
	}
}
