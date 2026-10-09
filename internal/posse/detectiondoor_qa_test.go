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
// THE THIRD ESCAPE, and the reason the second reading no longer excuses a
// BASENAME (ranger-base-x8bv0, github issue #4's other half). mhv7j fixed
// the half that is material to SEND; the doors also hand the reader PAGES,
// which need no bytes, and those stayed repo-relative. README.md was
// excused here by name — correctly, since it is not a filing and must not be
// embedded — but the exclusion was spelled as "this one file is not checked"
// and so it excused the defect along with the file. The rule is spelled by
// how the READER reaches the citation now: a path under publicDocBase
// (publicdoc.go) is a page to read and need not be carried, a bare path must
// be, and both must be in the tree. Nothing is excluded by name, so the next
// page a door cites has to pick one of the two and cannot be quiet.
//
// WHAT THIS PIN DOES NOT REACH: a bare path written into a sentence that is
// not a DetectionDoor surface. The citation would have to be a string
// literal somewhere in the package, and a census over shipped literals
// naming `docs/` is not precise enough to be a pin — MEASURED
// 2026-10-09, an ast walk over every non-test literal under internal/ and
// cmd/ returns 32KB of hook bodies, globs and regexps, nearly all of them
// legitimate. What holds instead is that each page is spelled ONCE, as the
// path const this pin stats, and reaches every sentence through publicDoc —
// so a page that moves reds here whichever surface prints it. The probe's
// own agent_not_found line is pinned where it is rendered
// (runtimeprobe_test.go, arm 3).
//
// Doored as `make doc-check` ($(QA_DOC_PINS)) — a tree-wide pin living in a
// package nobody runs whole, so no seat's `-run` filter would ever name it
// (ranger-base-rulbl, ranger-base-ik44f).

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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

// dddDocCite matches a repo-relative path to one of this repo's own doc
// families as a door sentence writes it — wider than dddFilingCite, which
// is anchored on the filings directory and cannot see the authoring page the
// declared-runtime arm cites.
var dddDocCite = regexp.MustCompile(`(?:docs/runbooks|etc/herdr/agent-detection)/[A-Za-z0-9._/-]+\.md`)

// dddSplitCites classifies every doc citation in one door sentence by how
// the READER of that sentence reaches it: urls are spelled under
// publicDocBase and are pages to read, bare ones are paths and are only
// honest when the binary carries the bytes. Split from the test, and taking
// the line rather than finding it, for the reason dddMissing takes a root —
// the control below points it at sentences whose answer is known.
func dddSplitCites(line string) (urls, bare []string) {
	for _, m := range dddDocCite.FindAllStringIndex(line, -1) {
		c := line[m[0]:m[1]]
		if m[0] >= len(publicDocBase) && line[m[0]-len(publicDocBase):m[0]] == publicDocBase {
			urls = append(urls, c)
			continue
		}
		bare = append(bare, c)
	}
	return urls, bare
}

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

	// The pages these doors cite, read as CONSTS before any sentence is
	// rendered. Each is spelled once as a repo-relative path and reaches
	// every surface through publicDoc, so this one reading is what makes a
	// renamed page red on surfaces this pin cannot render — the probe's
	// agent_not_found line among them (ranger-base-x8bv0).
	for _, rel := range []string{detectionFilingDoc, detectionDocPath} {
		if m := dddMissing(root, []string{rel}); len(m) != 0 {
			t.Errorf("a door cites %q, which is not a file in the tree — every sentence that names it is composed from this const, so the page moved and nothing else can tell you", rel)
		}
		if got, want := publicDoc(rel), publicDocBase+rel; got != want {
			t.Errorf("publicDoc(%q) = %q, want %q — the URL has to carry the path this pin just stat'd or the two readings are of different files", rel, got, want)
		}
		if urls, bare := dddSplitCites(publicDoc(rel)); len(urls) != 1 || urls[0] != rel || len(bare) != 0 {
			t.Errorf("publicDoc(%q) does not read back as a URL citation: urls=%q bare=%q — the classifier below would then call every door's link a bare path, or none of them", rel, urls, bare)
		}
	}

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
			// And the same paths against what the READER has. A release has
			// no checkout: the reader of this line typed `brew install
			// posse` (ranger-base-mhv7j). Two ways to be reachable from
			// there and the sentence has to pick one (ranger-base-x8bv0) —
			// material the binary CARRIES, or a page spelled as a URL. A
			// bare path is neither, and that was the defect both beads were
			// filed for.
			urls, bare := dddSplitCites(line)
			for _, c := range dddUncarried(posse.Filings, bare) {
				t.Errorf("%s: %s hands the operator %s as a bare path, which THIS BINARY does not carry — on a brew install there is no checkout, so that sentence is a dead end: carry it (embed.go) or cite it as %s (ranger-base-mhv7j, ranger-base-x8bv0):\n%s",
					rt.Name, surface, c, publicDoc(c), line)
			}
			// A URL is excused the carried reading and nothing else: the
			// page it names still has to be in the tree, or the link 404s on
			// the operator's laptop and reds nowhere.
			for _, c := range dddMissing(root, urls) {
				t.Errorf("%s: %s links the operator to %s, which is NOT in the tree — %s is composed from this path, so a page that was renamed or moved is a 404 the reader meets and no pin does (ranger-base-x8bv0):\n%s",
					rt.Name, surface, c, publicDoc(c), line)
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
			for _, want := range []string{"herdr --version", publicDoc(detectionFilingDoc)} {
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

	// THE CLASSIFIER, and it is the piece that replaced an exclusion by name
	// (ranger-base-x8bv0). It decides which of the two readings each
	// citation answers to, so a classifier that called everything a URL
	// would excuse every bare path in the tree — the mhv7j defect, restored
	// and green — and one that called everything bare would red on the two
	// pages that are correctly links. Both directions, and both families.
	const relDoc = "docs/runbooks/agent-detection-manifest.md"
	const relFiling = "etc/herdr/agent-detection/README.md"
	for _, c := range []struct {
		name string
		line string
		urls []string
		bare []string
	}{
		{"a bare path in a sentence", "the filing has to be written and sent (" + relFiling + ", ADR 0060 D2)", nil, []string{relFiling}},
		{"the same path as a link", "sent (" + publicDoc(relFiling) + ", ADR 0060 D2)", []string{relFiling}, nil},
		{"the other family, bare", "alias it onto one that exists: " + relDoc, nil, []string{relDoc}},
		{"the other family, linked", "alias it onto one that exists: " + publicDoc(relDoc), []string{relDoc}, nil},
		{"one of each in one sentence", publicDoc(relDoc) + " and " + relFiling, []string{relDoc}, []string{relFiling}},
		{"no citation at all", "check `herdr --version` and upgrade", nil, nil},
		// A URL to somebody ELSE's copy is not this base and is not excused:
		// it is a path this repo's pin cannot stat, dressed as a link.
		{"a fork's blob URL", "https://github.com/someone/posse/blob/main/" + relDoc, nil, []string{relDoc}},
		// And the base alone, with the path broken off, matches nothing —
		// so a truncated link cannot read as a satisfied citation.
		{"the base with no path", publicDocBase, nil, nil},
	} {
		urls, bare := dddSplitCites(c.line)
		if !reflect.DeepEqual(urls, c.urls) || !reflect.DeepEqual(bare, c.bare) {
			t.Errorf("%s: dddSplitCites urls=%q bare=%q, want urls=%q bare=%q\n%s", c.name, urls, bare, c.urls, c.bare, c.line)
		}
	}
}
