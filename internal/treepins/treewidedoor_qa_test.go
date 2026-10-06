package treepins

// QA pins for ranger-base-ik44f — fast doors to the other four tree-wide pins.
//
// THE CLASS, as ranger-base-rulbl named it: "a QA test whose subject is the
// TREE, living inside a package nobody runs whole". internal/posse is ~950s,
// past the 600s a seat can spend in one foreground call, so the standing
// advice is a focused `-run` filter — and `-run` selects by test NAME. A
// tree-wide pin is nobody's subject, so no seat's filter ever names it, and
// the pin is unreachable at exactly the moment it would have mattered.
//
// rulbl shipped the first door (`make fmt-check`) and censused the rest.
// This bead is the other four:
//
//	TestTreeIsGofmtClean                          make fmt-check   ~1.5s
//	TestShippedTreeNamesRolesNotThisCrew          make crew-check  ~2.5s
//	TestShippedStringsNameRolesNotThisCrew        make crew-check
//	TestTestCorpusHidesNoCrewNameBehindAnEscape   make crew-check
//
// and eight more the first census could not see, doored under
// ranger-base-sx2dq (see arm 2):
//
//	TestSeedSurfaceNameCountIsZero                make seed-check    ~0.2s
//	TestSeedConfigLiveKeysAreRead                 make seed-check
//	TestPublicationRootCommitOmitsExcludedPaths   make history-check ~0.5s
//	TestPublicationRootCommitADRsCarryProvenance  make history-check
//	TestPublicationHistoryNeverCarriesTheSeedScript make history-check
//	TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover
//	                                              make doc-check     ~0.1s
//	TestQACageCredDocDoesNotCallTheOnDiskCredentialStale   make doc-check
//
// (seven rows under a count of eight: the eighth was
// TestQAADR0036StatusLineDoesNotCarryTheRetractedUnbuiltStamp, deleted with
// its door entry under ranger-base-xrdb0. The numeral is sx2dq's and stays —
// it says what that bead doored, not what the class holds today, which only
// the derived sentence below may say.)
//
// and five more that took their root from `git rev-parse --show-toplevel`,
// doored under ranger-base-xndgk (FINDING 5 of the ranger-base-xtgvp verify):
//
//	TestQAIdentityLiteralsNeverAppearInATrackedPath make identity-check ~0.5s
//	TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned
//	                                              make identity-check
//	TestQAEveryOpsHitInTrackedMarkdownIsRuled     make ops-check       ~2s
//	TestQAOpsShapeTableCanStillSayNo              make ops-check
//	TestShippedExampleTableCoversEveryVersionInGitHistory
//	                                              make history-check   ~3s
//
// and four more that arrived afterwards, each doored by the bead that wrote
// it and given its own membership row in arm 2 — three rows now, the fourth
// being TestQAADR0026StatusLineDoesNotDeferTheImplementedRung
// (ranger-base-8dnuy), deleted with its door entry under ranger-base-xrdb0:
//
//	TestQAADR0035PaneModeSurfaceClaimIsBuilt      make doc-check
//	                                              (ranger-base-vwgt)
//	TestInstancePathFormNeverAppearsInTrackedContentUndispositioned
//	                                              make ops-check
//	TestQAInstancePathCensusCanStillSayNo         make ops-check
//	                                              (both ranger-base-l9ii)
//
// and one more, given its own door rather than folded into crew-check's
// (crew-check's own comment reserves that door for one question, which this
// pin does not ask):
//
//	TestQATreeGoFilesWriteExecutablesUnderTheForkLock
//	                                              make execwrite-check
//	                                              (ranger-base-rwnbd, widened
//	                                              by ranger-base-6cznr)
//
// and one more, folded into seed-check's own door rather than given a new
// one — it touches examples/config.yaml, which is exactly what that door
// already reads:
//
//	TestQAExampleConfigConstitutionBlockNamesTheWholePromotedSet
//	                                              make seed-check
//	                                              (ranger-base-nn33e)
//
// and two more, folded into seed-check's door for the same reason the row
// above was: their subject is examples/config.yaml. The pin derives the
// seed's documented duration defaults from the call sites that pair a config
// key with its Default* constant, and the second holds the register that
// silences it for a key documented at something other than its default:
//
//	TestSeedConfigDocumentedDurationDefaultsAreTheConstants
//	                                              make seed-check
//	TestSeedConfigDocumentedDefaultRegisterIsNotStale
//	                                              make seed-check
//	                                              (both ranger-base-vofbl)
//
// and one more, folded into doc-check's own door for the same reason — it
// is a prose pin over two shipped documents, which is that door's subject:
//
//	TestQAShippedLaunchLinesParseAsThePersonaLaunch
//	                                              make doc-check
//	                                              (ranger-base-qnn6j)
//
// and one more into the same door, a prose pin whose subject is a PATH: the
// door a detection refusal hands the operator has to name a filing the tree
// holds, and it was rendered from the runtime's NAME, so three of four
// built-ins named a file that is not there:
//
//	TestQADetectionDoorCitesAFilingThatExists
//	                                              make doc-check
//	                                              (ranger-base-ecchw)
//
// and one more into that same door, the SECOND prose pin whose subject is a
// path: a docs/notes.d citation in any markdown file in the tree has to
// resolve to a fragment the tree holds. v0.5.1 was cut over an ADR citing one
// that was stranded on the branch that wrote it, and nothing was red — the
// citation is prose and the fragment's absence is a missing file:
//
//	TestQAEveryNotesFragmentCitationResolves      make doc-check
//	                                              (ranger-base-nnnf1)
//
// and then a whole SECOND PACKAGE, which the register could not see at all
// until ranger-base-g6sb1. internal/treepins is 589.965s whole — the same
// wall internal/posse is, one directory over — and every rule above keys on
// a test COMPUTING the repo root, which a test in that package never does:
// its TestMain chdirs the binary to the root before anything runs, so a pin
// there reads the tree through a plain relative path. Twenty-three are
// derived from a second rule (an enumeration rooted at a relative tree path;
// see "The internal/treepins half of the class" below), and one more is
// registered by hand because its enumeration is a line of Python:
//
//	TestNotesFragmentIndexIsCurrent               make notes-check  ~0.3s
//	TestADRCitedGoFilesResolveOrAreDeclared       make adr-check    ~1.5s
//	TestADRCitationCheckCanFail                   make adr-check
//	TestADRCitationDeclarationsExemptOnlyWhatTheyDeclare
//	                                              make adr-check
//	TestADRCitationCorpusReadsTheExecutableSupplements
//	                                              make adr-check
//	TestADR0015NamesTheHookCommitPinAndItDoesWhatItSays
//	                                              make adr-check
//	TestQAFixtureRuntimeExesResolveToNothingOnThisBox
//	                                              make corpus-check   ~4s
//	TestQATheGofmtDoorReachesEveryGoFile          make corpus-check
//	TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot
//	                                              make corpus-check
//	TestQATheTolerantTempDirWrapperCompilesInEveryArm
//	                                              make corpus-check
//	TestQANoMakefilePrereqLineReadAsBytesOutsideMkPrereqs
//	                                              make corpus-check
//	TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared
//	                                              make corpus-check
//	TestNoUnswappedInternalRhqCommentsOutsideFrozenRecords
//	                                              make corpus-check
//	TestRhqLeftoverExemptionsStillNameRealLines   make corpus-check
//	TestQAEveryTreeWidePinHasADoor                make register-check ~20s
//	TestQAOneRepoRootHelperInTheTestPackage       make register-check
//	TestQATheTreeWideDoorsReportRealDrift         make register-check
//	TestQAMakeTestOpensTheTreeWideDoors           make register-check
//	TestQAEveryTrackedProgramATreePinRunsIsDispositioned
//	                                              make register-check
//	TestQATheTreepinsEnumerationRuleMatchesWhatItClaims
//	                                              make register-check
//	TestQAEveryTreepinsDoorFilterNamesItsPins     make register-check
//	TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles
//	                                              make register-check
//	TestQABoxCheckCensusCoversEveryVerifyScript   make scripts-check ~0.9s
//	TestQANoAssertionArmDecidesThroughAForkedMatcher
//	                                              make scripts-check
//	TestQABoxCheckCensusCoversEveryVerifyTarget   make scripts-check
//	                                              (ranger-base-hrf47)
//	TestShippedPIDsCarryTheNarrowedHookRows       make pid-check      ~10s
//	TestShippedPIDsLetBeadsOwnHooksRun            make pid-check
//
// (Three of those rows are exempt from the two-way check, listed with their
// reasons in twdDoorHolders, all three for the same cause: they read the
// MAKEFILE by name rather than enumerating the tree, and
// `os.ReadFile("Makefile")` is the enumeration rule's own negative case. Two
// are the last two rows of register-check — the register's own arms. The
// third is the scripts-check row ranger-base-hrf47 added, and it is the one
// worth reading, because it is not this register inspecting itself: it
// censuses the Makefile's verify-* targets against the roster and EXCLUDED
// table of scripts/verify-box.sh. Nothing about reading two files by name
// makes a pin reachable — an unrelated bead adding a verify-* target reds it
// just as a tree walk would, and for as long as it sat behind no door the
// only thing that said so was an unfiltered 589.965s run of this package.
// Its derived sibling three rows up globs scripts/, so scripts-check was
// green over it the whole time. A pin whose file set is spelled is not the
// same as a pin only its own bead can red, and the door is what closes that
// gap — not the rule.)
//
// and `make tree-check` is all of them — 46-51s on this box over three
// warm runs at fifty-three pins and fourteen doors — which is the command a
// seat types after a filtered run. (It was 14.9-16.5s over twenty-three pins
// behind eight doors, before ranger-base-g6sb1 found a second package;
// 15-43s at twenty-two, 12-27s under
// ranger-base-8dnuy, and 40-46s at a smaller class before that. Re-measured
// whenever the class changes — under ranger-base-xrdb0, and again under
// ranger-base-ecchw, and again under ranger-base-vofbl, and again under
// ranger-base-nnnf1, and again under ranger-base-yrag8 — because the
// sentence a seat prices the command from
// should not quote a run of a class it did not run; and stated without a
// second numeral, deliberately: a historical count in this comment is
// invisible to arm 4's one-claim rule, which is ranger-base-erqvh row 2. The
// seconds are NOT pinned — an elapsed-seconds red belongs to the box, per
// the `test` target's own note, and a warm build cache is most of this
// spread — but they are measured, not carried. ALL FOUR 2026-10-04 readings
// say the spread is the box and not the cache. ranger-base-g6sb1 read 46.5-77.2s
// over three warm runs at forty-nine pins, the one-minute load average going
// 8.9 -> 23.8 across them with a sibling seat holding a suite slot
// throughout; ranger-base-vofbl re-read the same command at fifty-one, three
// warm runs later the same day with no sibling suite and a load of 5.4-7.1,
// and got 44-46s. The two pins it added cost 0.06s between them, so what
// moved between the two readings was the box. ranger-base-nnnf1 read 53-75s
// at fifty-two, three warm runs with the one-minute load average falling
// 24.2 -> 8.6 across them and the slowest run the one at the top of that
// fall; the pin it added costs 0.07s. ranger-base-hrf47 read 55.34s, 99.78s
// and 69.69s at the same count on the same day — it and nnnf1 each added one
// row to this enumeration in parallel and neither saw the other's — at a
// one-minute load average of 9.11, 8.36 and 25.06, with a sibling seat
// holding one of the two suite slots throughout; its one added pin costs
// 0.03s. The live seconds are ranger-base-yrag8's, which merged those two
// rows back together and re-read the command over the class they make: 51s,
// 46s and 48s, three warm runs with BOTH suite slots free throughout and a
// one-minute load average of 4.4 before the first and 9.1, 7.5 and 6.6 after
// each. That is the quiet end of every range above, read at the largest class
// yet, which is the same conclusion from the other side: the readings that
// ran wide ran beside somebody. So a seat pricing the command from the low end
// of any of these is reading a quiet box, and the high end is what it costs
// beside someone else's suite. Cold, with internal/treepins' test binary to
// compile as well as internal/posse's, the same command read 92.6s.)
//
// THAT SENTENCE IS THE ONLY LIVE COUNT IN THIS FILE, and arm 4 holds it to
// the Makefile, both numerals and the enumeration above it. It read seventeen
// and six until ranger-base-4jogv, and arm 4 is why it is not quoted here:
// the rule matches only a sentence shaped exactly `<word> pins and <word>
// doors`; a second sentence in that shape reds it, but a historical count
// spelled some other way — like the sentence just above, on purpose — is
// invisible to it. The DOOR number entered wrong at d189b623, which wrote
// "seven doors" over an enumeration of eight and was then faithfully
// decremented by one when the selector door went; the PIN number drifted on
// its own, because the three tests listed directly above were added to a
// door variable by beads that had no reason to read this comment. A seat
// prices `make tree-check` from this sentence, in the one file whose whole
// subject is "the doors are wide enough", so nothing but a derivation may
// say how many there are.
//
// WHY THE DOOR RUNS THE PIN. fmt-check re-runs the TOOL, because gofmt is a
// tool and `gofmt -l` cannot disagree with `go/format`. These four are Go:
// their reading is an ast parse, an unquote and a case-boundary scan, and a
// shell rewrite of that would be a second implementation to keep in sync by
// hand — a door that goes NARROWER than the pin while both look green, which
// is worse than no door at all. So each door is a `-run` filter naming the
// pin, and the only thing left to hold is that the filters name ALL of them.
//
// Four arms — rulbl's three, and a fourth that holds the sentence above:
//
//  1. the doors are wired — `make test` depends on tree-check, tree-check
//     reaches every door, and each door reads only (no `-w`, no `./...`,
//     and `-count=1`, because a door that answers from cache can lie).
//  2. the doors are WIDE ENOUGH, two-way and mechanically: the class is
//     derived by parsing internal/posse/*_test.go, and every member must be
//     named by a Makefile door variable, and every name in those variables
//     must be a member. A tree-wide pin added tomorrow reds this until it is
//     given a door, which is the whole deliverable.
//
//     THAT PROMISE WAS FALSE AS FIRST SHIPPED (ranger-base-sx2dq). The
//     derivation keyed on one identifier, qibRepoRoot, and the tree carried
//     a byte-identical twin of it (qspRepoRoot) plus a hand-rolled
//     `filepath.Abs("../..")`: three tree-wide pins were spelled outside the
//     class, got no door, and this arm said nothing. The twin is folded, the
//     hand-rolled climb goes through the helper, the derivation also follows
//     a test into a helper that WALKS from the root, and
//     TestQAOneRepoRootHelperInTheTestPackage below is what keeps a third
//     copy from re-opening the same hole. Five members became thirteen.
//
//     AND WAS STILL FALSE (ranger-base-xndgk FINDING 5). Both rules key on
//     GO's filesystem calls, and five pins took their root from `git
//     rev-parse --show-toplevel` — stdout from a subprocess, which neither
//     rule can see. One of them censuses every tracked PATH in the
//     repository, one `git grep`s every tracked FILE, one scans every
//     tracked markdown file, one reads the history of every shipped
//     example. All five now take the root from the one helper, that
//     spelling is fenced in TestQAOneRepoRootHelperInTheTestPackage, and
//     thirteen members became eighteen. It has moved both ways since — ADR
//     0016's socket hints and their selector pin went, three later pins
//     arrived — and this line deliberately stops counting there: the live
//     number is the one in the `make tree-check` sentence above, which arm 4
//     derives from the Makefile. The lesson the third rule is NOT:
//     the class was never bounded by how a test spells its root, so the
//     thing that bounds it is the single helper, not another rule.
//  3. the doors can FAIL: `make -n`'s own expansion of each, run for real
//     against a scratch copy of the tree carrying the real drift — a crew
//     name in a shipped file. Clean arm first, so a door that always fails is
//     not mistaken for one that detects.
//  4. the head comment's COUNT is the Makefile's — both numerals derived,
//     and every name in a door variable named up there, so the enumeration
//     the count rests on cannot go short while the count stays green
//     (ranger-base-4jogv) — AND the package README's door table is the same
//     list in the same order (ranger-base-r5546). That table is a second
//     copy of `tree-check`'s prerequisites in a page people outside this
//     file read; it was correct the day it shipped and nothing said so
//     after. The Makefile declines to write the door COUNT twice on
//     purpose, which is the same refusal one directory over.
//
//     AND AGENTS.md's DOOR BLOCK, one page further out again
//     (ranger-base-xed72) — the same list in the same order, in the page
//     every seat reads at session start. Standing orders carried a second
//     copy of the PIN count as well, held by nobody: it said four fewer than
//     the Makefile had, through two beads that doored a pin, corrected the
//     sentence above, and had no reason to read a crew-wide page. That page
//     states no count of this class at all now and points here instead, and
//     this arm holds both halves of that — the block against `tree-check`'s
//     prerequisites, and the absence of a numeral against the same spellings
//     twdSpelled writes.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The two packages in the class. internal/posse is ~950s whole and
// internal/treepins 589.965s, so a seat runs a `-run` filter in both and a
// tree-wide pin in either is nobody's subject.
const (
	twdPossePkg    = "internal/posse"
	twdTreepinsPkg = "internal/treepins"
)

// twdDoor is one Makefile door: the variable that holds its `-run` filter,
// the target that reads it, and the package its pins live in. The package is
// not decoration — arm 2 is two-way, and "this name is not a tree-wide test"
// is only answerable against the package the door actually runs.
type twdDoor struct {
	variable string
	target   string
	pkg      string
	// tool: the door re-runs the TOOL the pin runs rather than running the
	// pin under a `-run` filter. Allowed only where the pin's whole body IS
	// that tool invocation, so the two cannot disagree — gofmt for
	// fmt-check, scripts/notes-index.py for notes-check. Anywhere else it
	// would be a second implementation that can go narrower than the pin
	// while both look green.
	tool bool
}

// The Makefile variables that hold the class. One per door, plus the pins
// whose door is a tool rather than a filter — the union is what arm 2
// measures against the two packages.
var twdDoors = []twdDoor{
	{"QA_CREW_PINS", "crew-check", twdPossePkg, false},
	{"QA_TOOL_PINS", "fmt-check", twdPossePkg, true},
	{"QA_SEED_PINS", "seed-check", twdPossePkg, false},
	{"QA_HISTORY_PINS", "history-check", twdPossePkg, false},
	{"QA_DOC_PINS", "doc-check", twdPossePkg, false},
	{"QA_IDENTITY_PINS", "identity-check", twdPossePkg, false},
	{"QA_OPS_PINS", "ops-check", twdPossePkg, false},
	{"QA_EXECWRITE_PINS", "execwrite-check", twdPossePkg, false},
	// internal/treepins (ranger-base-g6sb1).
	{"QA_NOTES_PINS", "notes-check", twdTreepinsPkg, true},
	{"QA_ADR_PINS", "adr-check", twdTreepinsPkg, false},
	{"QA_CORPUS_PINS", "corpus-check", twdTreepinsPkg, false},
	{"QA_REGISTER_PINS", "register-check", twdTreepinsPkg, false},
	{"QA_SCRIPTS_PINS", "scripts-check", twdTreepinsPkg, false},
	{"QA_PID_PINS", "pid-check", twdTreepinsPkg, false},
}

// twdPinVars is the variable names alone, derived so the two lists cannot
// drift apart.
var twdPinVars = func() []string {
	var out []string
	for _, d := range twdDoors {
		out = append(out, d.variable)
	}
	return out
}()

// twdRootHelper is the ONE repo-root helper internal/posse's tests may use.
// It is a single identifier on purpose — the class below is derived from it,
// so a second spelling is a member of the class that the derivation cannot
// see. That is not hypothetical: the tree carried qspRepoRoot, byte-for-byte
// identical, and three tree-wide pins hid behind it (ranger-base-sx2dq).
const twdRootHelper = "qibRepoRoot"

// twdVar returns the `|`-separated test names a Makefile variable holds.
func twdVar(t *testing.T, makefile, name string) []string {
	t.Helper()
	for _, line := range strings.Split(makefile, "\n") {
		if !strings.HasPrefix(line, name) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, name))
		if !strings.HasPrefix(rest, ":=") {
			continue
		}
		var out []string
		for _, f := range strings.Split(strings.TrimSpace(strings.TrimPrefix(rest, ":=")), "|") {
			if f = strings.TrimSpace(f); f != "" {
				out = append(out, f)
			}
		}
		return out
	}
	t.Fatalf("the Makefile defines no %s — a door variable the pin below reads is gone, so the class has no recorded membership", name)
	return nil
}

// twdSameSet reports whether two name lists hold the same names, order aside.
func twdSameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}

// Arm 1: the doors exist, `make test` opens them, and they only read.
func TestQAMakeTestOpensTheTreeWideDoors(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// The umbrella reaches every door. It carries no recipe of its own, so
	// `make -n tree-check` prints exactly what a seat would have to type.
	//
	// Every membership below is TOKENS, not the line's bytes: see mkPrereqs
	// (ranger-base-hna69). This arm read the bytes until ranger-base-7kwb8,
	// which is worse here than at the three sites hna69 fixed, because ONE
	// `#` in front of the tail of `tree-check:` takes six of these seven
	// doors off the umbrella at once — make stops at the comment, the old
	// reader did not, and every `strings.Contains` below still found its
	// door's name sitting on the line.
	treeLine, tree := mkPrereqs(t, src, "tree-check")
	// Derived from twdDoors, not spelled: a door added to the register and
	// forgotten here would be a door this arm never checks the wiring of.
	var doors []string
	for _, d := range twdDoors {
		doors = append(doors, d.target)
	}
	for _, door := range doors {
		if !mkRuns(tree, door) {
			t.Errorf("`make tree-check` no longer reaches `%s`, so one tree-wide pin is back to being ~950s away: %q", door, treeLine)
		}
	}

	// And `make test` reaches the umbrella, so a full run fails on the class
	// in seconds instead of at ~950 (rulbl's reason for fmt-check).
	if testLine, deps := mkPrereqs(t, src, "test"); !mkRuns(deps, "tree-check") {
		t.Errorf("`make test` no longer depends on tree-check: %q", testLine)
	}

	// All seven doors, not four of them (ranger-base-x5bgs finding 2): a file
	// of any one of their names in the tree silences it the same way, and
	// fmt-check/seed-check/history-check/doc-check were checked at wiring
	// (arm above) but not here.
	phonyLine, phony := mkPrereqs(t, src, ".PHONY")
	if !mkRuns(phony, "tree-check") {
		t.Errorf(".PHONY does not name tree-check — a file of that name in the tree would silence it: %q", phonyLine)
	}
	for _, door := range doors {
		if !mkRuns(phony, door) {
			t.Errorf(".PHONY does not name %s — a file of that name in the tree would silence it: %q", door, phonyLine)
		}
	}

	for _, door := range twdDoors {
		if door.tool {
			// A tool door runs no `go test`, so none of the shape below
			// applies to it. fmt-check is held open by
			// gofmtdoor_qa_test.go; notes-check is held open just under
			// this loop, against the pin's own argv.
			continue
		}
		rawRecipe := makeRecipe(src, door.target)
		if len(rawRecipe) == 0 {
			t.Errorf("the Makefile has no `%s` target", door.target)
			continue
		}
		// mkRecipeCode, not the lines' bytes (ranger-base-x5bgs finding 1): a
		// recipe line is handed to the SHELL verbatim, so it is the shell's
		// comment rule that governs it, not make's — a `#` in front of the
		// tail silences the line for sh while every `strings.Contains` below
		// still found its bytes sitting after the `#`.
		recipe := strings.Join(mkRecipeCode(rawRecipe), "\n")
		if recipe == "" {
			t.Errorf("`make %s`'s recipe is all comments — make hands the shell a no-op and the door is dark: %q", door.target, rawRecipe)
			continue
		}
		// The door reads its filter from the one variable arm 2 measures.
		// Spelled here, so a door quietly narrowed to a literal subset of
		// the class fails rather than passing on a shorter list.
		if !strings.Contains(recipe, "'^($("+door.variable+"))$$'") {
			t.Errorf("`make %s` no longer runs exactly $(%s), anchored — the door and the pin it stands in for can now name different tests:\n%s", door.target, door.variable, recipe)
		}
		if !strings.Contains(recipe, "./"+door.pkg) {
			t.Errorf("`make %s` no longer names ./%s — a package-tree run of it is the wall this door exists to avoid:\n%s", door.target, door.pkg, recipe)
		}
		if strings.Contains(recipe, "./...") {
			t.Errorf("`make %s` runs the package tree, which is the wall it is a door through:\n%s", door.target, recipe)
		}
		// A door that can answer from cache is a door that can lie: the
		// drift these pins are about arrives as a new FILE in a walked
		// directory, and nothing promises go's test cache key notices one.
		if !strings.Contains(recipe, "-count=1") {
			t.Errorf("`make %s` may answer from go's test cache, so it can report a tree it never read:\n%s", door.target, recipe)
		}
		if strings.Contains(recipe, " -w") || strings.Contains(recipe, "rm ") {
			t.Errorf("`make %s` writes — the point of a door is that a seat can ask the question without changing the tree:\n%s", door.target, recipe)
		}
	}

	// notes-check, the second TOOL door (ranger-base-g6sb1). It is allowed
	// to be a tool door for fmt-check's reason and no other: the pin's whole
	// body is one exec.Command, so re-running that command is not a second
	// implementation of anything. What has to hold is that the recipe runs
	// the SAME command — so the argv is read out of the pin rather than
	// spelled here, and a flag added to either side without the other reds.
	argv := twdExecArgv(t, twdTreepinsPkg, "TestNotesFragmentIndexIsCurrent")
	if len(argv) < 2 {
		t.Errorf("TestNotesFragmentIndexIsCurrent no longer runs a single literal command (%v) — notes-check is a TOOL door, which is only honest while the pin's whole body is the command the door re-runs. If the pin has grown a reading of its own, the door has to become a `-run` filter.", argv)
	} else {
		rawNotes := makeRecipe(src, "notes-check")
		notes := strings.Join(mkRecipeCode(rawNotes), "\n")
		if notes == "" {
			t.Errorf("`make notes-check`'s recipe is all comments — make hands the shell a no-op and the door is dark: %q", rawNotes)
		}
		for _, word := range argv {
			if !strings.Contains(notes, word) {
				t.Errorf("`make notes-check` does not run %q, which TestNotesFragmentIndexIsCurrent does — the door and the pin it stands in for are now asking different questions:\n  pin:  %v\n  door: %s", word, argv, notes)
			}
		}
		if !strings.Contains(notes, "--check") {
			t.Errorf("`make notes-check` no longer passes --check, so it REGENERATES the index instead of reporting a stale one — a door that fixes the drift silently is a door that never reports it:\n%s", notes)
		}
	}
}

// twdExecArgv returns the literal argv of the one exec.Command in a named
// test, or nil if the test makes none or makes more than one. Used to read a
// tool door's command out of the pin rather than spelling it twice.
func twdExecArgv(t *testing.T, pkg, test string) []string {
	t.Helper()
	paths := twdTestFiles(t, pkg)
	fset := token.NewFileSet()
	var found [][]string
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || fn.Name.Name != test {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
					return true
				}
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "exec" {
					return true
				}
				var argv []string
				for _, a := range call.Args {
					lit, ok := a.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true // not a literal argv; not this shape
					}
					v, err := strconv.Unquote(lit.Value)
					if err != nil {
						return true
					}
					argv = append(argv, v)
				}
				found = append(found, argv)
				return true
			})
		}
	}
	if len(found) != 1 {
		return nil
	}
	return found[0]
}

// mkRecipeCode returns a recipe's lines (as makeRecipe reads them) with any
// shell comment stripped, and drops lines left empty by that. makeRecipe
// hands a RECIPE line back whole, tab stripped, because that line is make's
// own text — but make then hands it to the SHELL verbatim, so what governs a
// `#` inside it is the shell's comment rule, not make's: a `#` starts a
// comment only where it begins a word (nothing, or whitespace, before it) and
// it is not inside a quote. A byte check against the raw line, uncorrected,
// still finds its bytes sitting after a `#` a seat put there to silence the
// line — or after one that only commented out the line's tail
// (ranger-base-x5bgs finding 1).
func mkRecipeCode(lines []string) []string {
	var out []string
	for _, line := range lines {
		var single, double bool
		cut := len(line)
		for i := 0; i < len(line); i++ {
			switch line[i] {
			case '\'':
				if !double {
					single = !single
				}
			case '"':
				if !single {
					double = !double
				}
			case '#':
				if !single && !double && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
					cut = i
				}
			}
			if cut != len(line) {
				break
			}
		}
		if c := strings.TrimSpace(line[:cut]); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// The reader itself, on the lines a whole-line byte check got wrong. Without
// this the only thing measuring the `#` stop is a Makefile mutant, and the
// suite never runs one.
func TestQARecipeLinesStopAtAnUnquotedShellComment(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   []string
		want []string
	}{
		{"plain line", []string{"$(GOBIN) test ./internal/posse -count=1"}, []string{"$(GOBIN) test ./internal/posse -count=1"}},
		{"whole line commented", []string{"# $(GOBIN) test ./internal/posse -count=1"}, nil},
		{"commented tail", []string{"$(GOBIN) test ./internal/posse -count=1 # -run '^($(QA_CREW_PINS))$$'"}, []string{"$(GOBIN) test ./internal/posse -count=1"}},
		{"# inside single quotes is not a comment", []string{"echo '#not-a-comment' -count=1"}, []string{"echo '#not-a-comment' -count=1"}},
		{"# inside double quotes is not a comment", []string{`echo "#not-a-comment" -count=1`}, []string{`echo "#not-a-comment" -count=1`}},
		{"# glued to a word is not a comment", []string{"echo a#b"}, []string{"echo a#b"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := mkRecipeCode(c.in)
			if !slices.Equal(got, c.want) {
				t.Errorf("mkRecipeCode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// twdJoinsTwoDotDots reports whether an argument list ENDS in two ".."
// literals — `filepath.Join(dir, "..", "..")`, the climb from internal/posse
// to the repo root written out by hand.
//
// It must be the end of the list, and that is the whole distinction between
// this class and its neighbours: `filepath.Join("..", "..", "examples",
// "agents")` climbs to the root and then goes back DOWN into one directory,
// so a walk from it reads examples/agents and reds only when that directory
// changes. Nine tests do that, none of them tree-wide.
func twdJoinsTwoDotDots(args []ast.Expr) bool {
	if len(args) < 2 {
		return false
	}
	for _, a := range args[len(args)-2:] {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		if v, err := strconv.Unquote(lit.Value); err != nil || v != ".." {
			return false
		}
	}
	return true
}

// twdWalkers are the calls that turn a repo root into a reading of the whole
// tree. A function that computes the root AND makes one of these is walking
// from the root, and every file in the tree is then its subject.
var twdWalkers = map[string]bool{"WalkDir": true, "Walk": true, "ReadDir": true, "Glob": true}

// twdFunc is one function declared in internal/posse/*_test.go: who it calls,
// and whether its own body reaches outside the package.
type twdFunc struct {
	name  string
	where string
	test  bool
	calls map[string]bool
	// root: the body calls qibRepoRoot, the package's ONE repo-root helper.
	root bool
	// rootedWalk: the body computes the root AND walks from it, so anything
	// added anywhere in the tree can red whoever calls it.
	rootedWalk bool
	// walk: the body calls one of twdWalkers.
	walk bool
	// ascent: the body spells the climb from internal/posse to the repo
	// root by hand — `"..", ".."` joined, or a `"../.."` literal.
	ascent bool
	// caller: the body asks runtime.Caller where its own source lives,
	// which is how qibRepoRoot finds the root without depending on cwd.
	caller bool
	// handRolledTreeWalk: the body walks a directory whose path IS the repo
	// root reached by a hand-rolled climb — not one reached through
	// qibRepoRoot, and not a subdirectory of it.
	handRolledTreeWalk bool
	// relEnum: the body ENUMERATES a directory at a relative tree path —
	// a WalkDir/Walk/ReadDir/Glob whose root is a literal relative path, or
	// a filepath.Join of literals starting at one. This is the
	// internal/treepins half of the class (ranger-base-g6sb1): that
	// package's TestMain chdirs the whole binary to the repo root, so a
	// relative path IS a tree path and no root helper is ever named.
	relEnum []string
	// execTree: relative literals naming a tracked regular file that this
	// body hands to exec.Command — this repo's own programs, run on the
	// tree. What they read is outside Go, so they are dispositioned rather
	// than derived (twdIndirect, twdExecDispositions).
	execTree []string
	// shellRoot: the body asks GIT for this repo's root —
	// `exec.Command("git", "rev-parse", "--show-toplevel").Output()` — and
	// keeps the answer. A third spelling of the root, invisible to every
	// rule above, which is how five tree-wide pins sat undoored
	// (ranger-base-xndgk FINDING 5).
	shellRoot bool
}

// twdAsksGitForTheRoot reports whether a call is
// `exec.Command("git", ..., "--show-toplevel", ...).Output()` (or
// CombinedOutput) — git asked for this repo's root with the answer KEPT.
//
// The value being kept is the whole distinction. Four calls in
// silentrevert_qa_test.go ask git the same question and throw the answer
// away, as a probe for "is there a checkout here at all" — as does
// qibSkipUnlessCheckout, which asks --git-dir. A probe is not a second way
// to compute the root, and it is not fenced.
//
// RESIDUAL, said out loud: this matches the exec.Command spelling, which is
// what all four offending call sites wrote and what the next one would
// write. A root taken from the package's own git(dir, ...) helper with dir
// spelled "." is not matched. Nothing here can enumerate every way to name
// this repo — what bounds the class is that there is ONE helper, and the pin
// below fences the three spellings that have actually appeared: a twin
// helper, a hand-rolled climb, and this.
func twdAsksGitForTheRoot(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Output" && sel.Sel.Name != "CombinedOutput") {
		return false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	fn, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || (fn.Sel.Name != "Command" && fn.Sel.Name != "CommandContext") {
		return false
	}
	if x, ok := fn.X.(*ast.Ident); !ok || x.Name != "exec" {
		return false
	}
	for _, a := range inner.Args {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if v, err := strconv.Unquote(lit.Value); err == nil && v == "--show-toplevel" {
			return true
		}
	}
	return false
}

// twdRootExprs finds, inside one function body, every expression that
// evaluates to the repo root reached by hand: a bare `"../.."`, a
// `filepath.Join(..., "..", "..")`, those wrapped in Abs/Clean/EvalSymlinks,
// and the identifiers assigned from any of them. It returns a predicate over
// expressions.
//
// This is a small dataflow rather than "the body mentions `..` and walks
// something" on purpose, and the difference is nine tests: detectionRig
// climbs to the root and then ReadDirs `<root>/etc/herdr/agent-detection`,
// which reds when that one directory changes. Only a walk rooted AT the root
// is tree-wide.
func twdRootExprs(body *ast.BlockStmt) func(ast.Expr) bool {
	roots := map[string]bool{}
	var isRoot func(ast.Expr) bool
	isRoot = func(e ast.Expr) bool {
		switch v := e.(type) {
		case *ast.Ident:
			return roots[v.Name]
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return false
			}
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return false
			}
			return filepath.ToSlash(s) == "../.." || filepath.ToSlash(s) == "../../"
		case *ast.CallExpr:
			sel, ok := v.Fun.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name != "filepath" {
				return false
			}
			switch sel.Sel.Name {
			case "Join":
				return twdJoinsTwoDotDots(v.Args)
			case "Abs", "Clean", "EvalSymlinks", "ToSlash":
				return len(v.Args) == 1 && isRoot(v.Args[0])
			}
		}
		return false
	}
	// Two passes, so `root := ...` before the walk and after both bind.
	for i := 0; i < 2; i++ {
		ast.Inspect(body, func(n ast.Node) bool {
			var lhs, rhs []ast.Expr
			switch st := n.(type) {
			case *ast.AssignStmt:
				lhs, rhs = st.Lhs, st.Rhs
			case *ast.ValueSpec:
				for _, id := range st.Names {
					lhs = append(lhs, id)
				}
				rhs = st.Values
			default:
				return true
			}
			// `x := expr` and `x, err := expr` both bind x.
			if len(rhs) != 1 || len(lhs) == 0 {
				if len(lhs) != len(rhs) {
					return true
				}
				for i, l := range lhs {
					if id, ok := l.(*ast.Ident); ok && isRoot(rhs[i]) {
						roots[id.Name] = true
					}
				}
				return true
			}
			if id, ok := lhs[0].(*ast.Ident); ok && isRoot(rhs[0]) {
				roots[id.Name] = true
			}
			return true
		})
	}
	return isRoot
}

// twdTestFiles lists one package's test files.
//
// THE GLOBS ARE SPELLED OUT, one literal per package, rather than composed
// from `pkg` — deliberately, and this file is the evidence. Composing the
// root from the parameter made the rule blind to its own parser:
// twdRelTreePath cannot read a variable, so every test that reaches
// twdParseDir stopped being derived as tree-wide, and arm 2 immediately red
// with two doors it could no longer justify (MEASURED 2026-10-04, on this
// file, by that red — the register caught the refactor that broke it, which
// is the only reason this comment exists). A tree reading in this package is
// written in a spelling the rule can see, the same way internal/posse's
// tests are held to ONE repo-root helper. The alternative — teaching
// twdRelTreePath to follow constants and parameters — is interprocedural
// constant propagation, a second thing to be wrong about, to buy back a
// spelling nobody needs.
func twdTestFiles(t *testing.T, pkg string) []string {
	t.Helper()
	var paths []string
	var err error
	switch pkg {
	case twdPossePkg:
		paths, err = filepath.Glob(filepath.Join("internal", "posse", "*_test.go"))
	case twdTreepinsPkg:
		paths, err = filepath.Glob(filepath.Join("internal", "treepins", "*_test.go"))
	default:
		t.Fatalf("no test-file glob for package %q — the class cannot be derived for a package this file does not name", pkg)
	}
	if err != nil || len(paths) == 0 {
		t.Fatalf("no test files found under %s: %v", pkg, err)
	}
	return paths
}

// twdParse reads every function declared in internal/posse/*_test.go.
func twdParse(t *testing.T) (map[string]*twdFunc, int) {
	t.Helper()
	return twdParseDir(t, twdPossePkg)
}

// twdParseTreepins reads every function declared in
// internal/treepins/*_test.go — this file among them, which is the point:
// the register's own arms are in the class they derive.
func twdParseTreepins(t *testing.T) (map[string]*twdFunc, int) {
	t.Helper()
	return twdParseDir(t, twdTreepinsPkg)
}

// twdParseDir reads every function declared in one package's *_test.go.
func twdParseDir(t *testing.T, pkg string) (map[string]*twdFunc, int) {
	t.Helper()
	paths := twdTestFiles(t, pkg)
	fset := token.NewFileSet()
	out := map[string]*twdFunc{}
	tests := 0
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			f := twdAnalyze(fn, fmt.Sprintf("%s:%d", path, fset.Position(fn.Pos()).Line))
			if f.test {
				tests++
			}
			out[f.name] = f
		}
	}
	return out, tests
}

// twdAnalyze reads ONE function declaration: who it calls, and every way its
// body reaches outside its own package. Every rule in this file is in here,
// so the fixture-driven mutation check below drives the same code the tree is
// derived with — a second copy of the rule, written to be easy to test, is a
// copy that can agree with the fixture and disagree with the tree.
func twdAnalyze(fn *ast.FuncDecl, where string) *twdFunc {
	f := &twdFunc{
		name:  fn.Name.Name,
		where: where,
		test:  strings.HasPrefix(fn.Name.Name, "Test"),
		calls: map[string]bool{},
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				switch filepath.ToSlash(v) {
				case "../..", "../../":
					f.ascent = true
				}
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if twdJoinsTwoDotDots(call.Args) {
			f.ascent = true
		}
		if twdAsksGitForTheRoot(call) {
			f.shellRoot = true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			f.calls[fun.Name] = true
			if fun.Name == twdRootHelper {
				f.root = true
			}
		case *ast.SelectorExpr:
			if twdWalkers[fun.Sel.Name] {
				f.walk = true
				if len(call.Args) > 0 {
					if rel, ok := twdRelTreePath(call.Args[0]); ok {
						f.relEnum = append(f.relEnum, fun.Sel.Name+"("+rel+")")
					}
				}
			}
			if x, ok := fun.X.(*ast.Ident); ok && x.Name == "runtime" && fun.Sel.Name == "Caller" {
				f.caller = true
			}
			if x, ok := fun.X.(*ast.Ident); ok && x.Name == "exec" && (fun.Sel.Name == "Command" || fun.Sel.Name == "CommandContext") {
				for _, a := range call.Args {
					if prog, ok := twdTrackedProgram(a); ok {
						f.execTree = append(f.execTree, prog)
					}
				}
			}
		}
		return true
	})
	f.rootedWalk = f.root && f.walk
	if !f.root {
		isRoot := twdRootExprs(fn.Body)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !twdWalkers[sel.Sel.Name] {
				return true
			}
			if isRoot(call.Args[0]) {
				f.handRolledTreeWalk = true
			}
			return true
		})
	}
	return f
}

// twdTreeWideTests returns the tests whose subject is the TREE. Two rules,
// unioned, and the union is deliberate — ranger-base-sx2dq is what a single
// rule cost:
//
//   - DIRECT: the test's own body calls qibRepoRoot, so it reads outside its
//     package. This was the whole rule, and it is kept because it is what
//     the five original members are doored on.
//   - WALK-REACHING: the test reaches, through this package's own test
//     helpers, a function that computes the repo root and WALKS from it.
//     TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover names no
//     root of its own — it calls coxnGoSources, which walks every shipped
//     .go file — and any file added anywhere can red it. The direct rule
//     never saw it.
//
// The reach is only propagated from a ROOTED WALK, not from any function
// that touches the root: qspSeedScript looks up one path at the root and its
// callers red only when that script changes, which is not this class.
//
// What keeps a third spelling from slipping past both rules is not a rule
// here at all — it is TestQAOneRepoRootHelperInTheTestPackage below, which
// holds internal/posse to ONE repo-root helper. That is the pin
// ranger-base-sx2dq was filed for: the tree carried a byte-identical twin of
// qibRepoRoot (qspRepoRoot) and a hand-rolled `..`/`..` ascent, and pins
// spelled either way were outside the class and got no door, silently.
func twdTreeWideTests(t *testing.T) (names []string, funcs int) {
	t.Helper()
	all, funcs := twdParse(t)

	// Seed the reach with the rooted walkers, then close it over callers.
	reaches := twdReach(all, func(f *twdFunc) bool { return f.rootedWalk })

	for name, f := range all {
		if f.test && (f.root || reaches[name]) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, funcs
}

// ---------------------------------------------------------------------------
// The internal/treepins half of the class (ranger-base-g6sb1).
//
// WHY A SECOND RULE AT ALL. Everything above keys, one way or another, on a
// test COMPUTING the repo root: it calls qibRepoRoot, or it reaches a helper
// that walks from a root, or it asks git for one. That key exists because
// `go test` runs internal/posse's binary with the PACKAGE directory as its
// working directory, so reading the tree costs an explicit climb. It does not
// exist here. internal/treepins' TestMain (configdirfence_test.go) chdirs the
// whole binary to the repo root before any test runs, so a pin in this
// package reads the tree through a plain relative path — "scripts/notes-
// index.py", "docs/adr" — and never names a root helper at all. Every rule
// above is blind to this package by construction, and this package is
// 589.965s whole: the same wall, one directory over.
//
// TestNotesFragmentIndexIsCurrent is what got out. Adding one
// docs/notes.d/<bead>.md fragment landed a commit that `make fmt-check`,
// `make tree-check` (all eight doors as it then stood) and a full
// `go test -tags posse_arm2 ./internal/posse` all called clean; the 589.965s
// arm-1 run said `notesindex_qa_test.go:13: docs/notes.d/README.md: stale
// index`. The pin was right, its remediation line was exact, and nothing a
// seat could type in under ten minutes would ever have asked it.
//
// THE KEY, and why it is not the obvious one. "A test in a package whose
// TestMain chdirs to the repo root, that reads a tree path" is the honest
// description of the blind spot, but it is not a usable key: MEASURED
// 2026-10-04, 291 of internal/treepins' 362 tests name a path the tree holds.
// Four fifths of a package is not a class. What actually separates a pin that
// an unrelated bead can red from one it cannot is whether the reading
// ENUMERATES — whether the set of files read is spelled in the test, or
// discovered. A test that reads "Makefile" reds when somebody edits the
// Makefile, which is the bead that edited it. A test that globs
// "docs/notes.d/*.md" reds when ANY bead writes its fragment there. So the
// key is a WalkDir/Walk/ReadDir/Glob rooted at a relative tree path. It found
// twenty tests in the package as it stood before this bead — and that number
// is frozen prose, like every other count in this comment: the live one is
// in the `make tree-check` sentence above, which arm 4 derives.
//
// THAT IS WIDER THAN THE POSSE RULE, deliberately. Over there a walk rooted
// at a SUBDIRECTORY is excluded — detectionRig ReadDirs
// `<root>/etc/herdr/agent-detection` and reds only when that one directory
// changes, and nine tests do that shape. The exclusion is defensible there
// and would be the bug here: docs/notes.d is a subdirectory, and it is the
// subdirectory every bead on this box writes into. Drawing the line at the
// repo root would have reproduced exactly the blind spot this bead was filed
// for — it finds nine of those twenty, and not the one that escaped.
//
// AND IT STILL DOES NOT FIND THE ONE THAT ESCAPED. TestNotesFragmentIndexIsCurrent
// enumerates nothing in Go: it runs `python3 scripts/notes-index.py --check`
// and the glob is a line of Python. No rule over Go source can see it, and a
// shell/Python source scanner would be a second implementation of "does this
// program read a directory" — the narrower-than-the-pin door this file's head
// comment refuses. So that half is REGISTERED by hand (twdIndirect), held
// honest by a pin that re-reads the program and the directory each entry
// claims, and fenced by a dispositioned census of every tracked program a
// test in this package hands to exec.Command (twdExecDispositions).

// twdRelTreePath returns the relative tree path an expression spells, if it
// spells one: a string literal, or a filepath.Join of string literals whose
// first element is one. An absolute path, a `..` climb, or anything carrying
// a variable is NOT one — a Join with a t.TempDir() in it is a scratch
// directory, and that distinction is most of this rule's precision.
func twdRelTreePath(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		str, err := strconv.Unquote(v.Value)
		if err != nil || str == "" {
			return "", false
		}
		slash := filepath.ToSlash(str)
		if filepath.IsAbs(str) || slash == ".." || strings.HasPrefix(slash, "../") {
			return "", false
		}
		return slash, true
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
			return "", false
		}
		switch sel.Sel.Name {
		case "Clean", "ToSlash", "FromSlash":
			if len(v.Args) == 1 {
				return twdRelTreePath(v.Args[0])
			}
		case "Join":
			if len(v.Args) == 0 {
				return "", false
			}
			first, ok := twdRelTreePath(v.Args[0])
			if !ok {
				return "", false
			}
			parts := []string{first}
			for _, a := range v.Args[1:] {
				lit, ok := a.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					// A variable element: the root is no longer a tree path
					// this rule can claim to know.
					return "", false
				}
				str, err := strconv.Unquote(lit.Value)
				if err != nil {
					return "", false
				}
				parts = append(parts, filepath.ToSlash(str))
			}
			return strings.Join(parts, "/"), true
		}
	}
	return "", false
}

// twdTrackedProgram reports the relative path of a tracked REGULAR file an
// expression names — one of this repo's own programs, handed to exec.Command.
//
// Regular file, not a directory, on purpose: `go run ./cmd/testparallel` and
// `go test ./internal/treepins` name package paths, not readings, and the Go
// in them is source the rules above read directly. The residual is said out
// loud in twdExecDispositions.
func twdTrackedProgram(e ast.Expr) (string, bool) {
	rel, ok := twdRelTreePath(e)
	if !ok {
		return "", false
	}
	rel = strings.TrimPrefix(rel, "./")
	if !strings.Contains(rel, "/") {
		// A bare word: `sh`, `python3`, `make`. Even if the tree happens to
		// hold a file of that name, exec resolves it on PATH.
		return "", false
	}
	info, err := os.Lstat(filepath.FromSlash(rel))
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return rel, true
}

// twdReach closes a seed set over the call graph: every function that calls a
// seeded function, transitively. Both halves of the class propagate the same
// way, so there is one closure and three seeds, not three closures.
func twdReach(all map[string]*twdFunc, seeded func(*twdFunc) bool) map[string]bool {
	reaches := map[string]bool{}
	for name, f := range all {
		if seeded(f) {
			reaches[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name, f := range all {
			if reaches[name] {
				continue
			}
			for callee := range f.calls {
				if reaches[callee] {
					reaches[name] = true
					changed = true
					break
				}
			}
		}
	}
	return reaches
}

// twdIndirect is the half of the internal/treepins class that no rule over Go
// source can derive: a pin whose enumeration happens inside one of this
// repo's own programs. Hand-registered, and held honest by
// TestQAEveryTrackedProgramATreePinRunsIsDispositioned below, which re-reads
// both the test and the program rather than trusting the row.
//
// One entry. It is the entry this bead exists for.
var twdIndirect = []struct {
	test string // the pin, in internal/treepins
	prog string // the tracked program it runs
	dir  string // the tree directory that program enumerates
	why  string // what the Go body shows, and what it does not
}{
	{
		test: "TestNotesFragmentIndexIsCurrent",
		prog: "scripts/notes-index.py",
		dir:  "docs/notes.d",
		why: "the pin's whole body is `python3 scripts/notes-index.py --check`; " +
			"the enumeration is that script's own `directory.glob(\"*.md\")` over " +
			"docs/notes.d, so every bead that writes a fragment there is in this " +
			"pin's reach and no rule over Go source can see it",
	},
}

// twdExecDispositions is the fence around the register above: every tracked
// program a test in internal/treepins hands to exec.Command must be here or
// in twdIndirect. The value says what the program reads INSTEAD of a tracked
// directory — checked by hand when the row was written, and re-asked of every
// new row by the pin that reds on an undispositioned program.
//
// RESIDUAL, said out loud. This matches a relative literal naming a tracked
// regular file in an exec.Command argv — what every call site below writes,
// and what the next one would. A program path assembled from a variable, or
// carried into exec.Command inside a []string, is not matched: the second
// arm of notesindex_qa_test.go builds its argv that way, which is correct
// here (it points the script at a t.TempDir()) and would not be correct for
// a future pin spelled the same way. Nothing in Go can bound what a
// subprocess reads. What bounds this is that the census defaults to
// INCLUSION — an enumerating program makes its callers members — and that
// the list of programs is short enough to have been read.
var twdExecDispositions = map[string]string{
	"scripts/bd-argv-gate.sh":        "parses one command line out of its argv and stdin; it opens no directory at all",
	"scripts/gotest.sh":              "`find`s only $CACHE and its own scratch tree, never a tracked path",
	"scripts/macos-install-probe.sh": "probes a scratch install root under $D; the only tracked file it reads is the formula it is handed",
	"scripts/path-warning.sh":        "reads the PATH string it is given",
	"scripts/tap-formula.sh":         "renders a formula from the --checksums file named on its command line",
	"scripts/test-linux.sh":          "assembles a docker invocation; it lists nothing",
	"scripts/test-times.sh":          "runs `go test` and parses its output; the one `ls` in it is inside an advice string it prints",
}

// twdDoorHolders are tests a door variable may name although no class rule
// derives them: the arms that hold the register itself open. They read the
// Makefile and run `make -n`, which is not a reading of the tree, so nothing
// in this file can find them — and they live in the same 589.965s package as
// the pins they guard, so left out of a door the register's own floor is as
// unreachable as the pins were.
//
// This is the ONE way into a door variable that does not go through the
// derivation, which is why it is a list with reasons rather than a flag: it
// is exactly the shape of parking spot this file keeps warning about. Arm 2
// holds each row to being a real test in the package its door runs, and two
// rows is what it should stay near. A pin put here to dodge the two-way
// check would be a pin whose membership nothing derived and whose door
// nothing justified.
var twdDoorHolders = map[string]string{
	"TestQAEveryTreepinsDoorFilterNamesItsPins":            "asks what make's expansion of each internal/treepins door filter selects; it reads the Makefile and runs `go test -list`",
	"TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles": "arm 4 — holds this file's head comment and the treepins README's door table to the Makefile's own pin and door counts; it reads the Makefile, this file and that README",
	// ranger-base-hrf47. The third exemption, and the first that is not one of
	// this register's own arms: it censuses every verify-* target in the
	// Makefile against the roster and EXCLUDED table of scripts/verify-box.sh,
	// reading those two files BY NAME. So the enumeration rule cannot see it
	// (`os.ReadFile("Makefile")` is the rule's own negative case — see
	// TestQATheTreepinsEnumerationRuleMatchesWhatItClaims), while the thing
	// that makes it tree-wide in the sense the class means is unaffected: an
	// unrelated bead adding a verify-* target reds it, and until this row
	// existed nothing but an unfiltered 589.965s package run would say so. Its
	// sibling in the same file IS derived, because that one globs scripts/ —
	// so scripts-check went green over this one for as long as both existed
	// (the escape ranger-base-hrf47 was filed for).
	"TestQABoxCheckCensusCoversEveryVerifyTarget": "censuses the Makefile's verify-* targets against the roster and EXCLUDED table of scripts/verify-box.sh; it reads those two files by name and enumerates nothing",
}

// twdTreepinsTreeWideTests returns the tests in internal/treepins whose
// subject is the TREE: the derived twenty, plus the registered indirect ones.
func twdTreepinsTreeWideTests(t *testing.T) (names []string, derived []string, funcs int) {
	t.Helper()
	all, funcs := twdParseTreepins(t)
	reaches := twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })
	for name, f := range all {
		if f.test && reaches[name] {
			derived = append(derived, name)
		}
	}
	sort.Strings(derived)
	names = append(names, derived...)
	for _, row := range twdIndirect {
		if !slices.Contains(names, row.test) {
			names = append(names, row.test)
		}
	}
	sort.Strings(names)
	return names, derived, funcs
}

// twdClassOf returns one package's tree-wide tests, for the two-way check
// below. Two packages, two rules, because the two packages reach the tree in
// ways that have no spelling in common.
func twdClassOf(t *testing.T, pkg string) []string {
	t.Helper()
	switch pkg {
	case twdPossePkg:
		names, _ := twdTreeWideTests(t)
		return names
	case twdTreepinsPkg:
		names, _, _ := twdTreepinsTreeWideTests(t)
		return names
	}
	t.Fatalf("no class rule for package %q — a door variable was given a package this file cannot derive, so its membership is checked against nothing", pkg)
	return nil
}

// The fence on the indirect register: every row still names a real pin
// running a real program over the directory it claims, and every tracked
// program a test in this package execs is dispositioned.
//
// The directory check is not decoration. If scripts/notes-index.py is pointed
// at another directory tomorrow, the row above becomes a claim about a
// reading that no longer happens — and the door would keep passing while the
// pin's subject moved out from under it.
func TestQAEveryTrackedProgramATreePinRunsIsDispositioned(t *testing.T) {
	t.Parallel()
	all, funcs := twdParseTreepins(t)
	if funcs < 200 {
		t.Fatalf("only %d test functions parsed under %s — the walk this pin reads is finding nothing", funcs, twdTreepinsPkg)
	}

	registered := map[string]bool{}
	for _, row := range twdIndirect {
		registered[row.prog] = true
		f, ok := all[row.test]
		if !ok || !f.test {
			t.Errorf("twdIndirect registers %s, which is not a test in %s — the indirect half of the class names a pin that is gone, and arm 2 is holding a door open for nothing", row.test, twdTreepinsPkg)
			continue
		}
		if !slices.Contains(f.execTree, row.prog) {
			t.Errorf("twdIndirect says %s runs %s, and its body does not (%s runs %v) — the row is a claim about a reading that no longer happens, so the pin's subject has moved out from under its door. The row's reason was: %s", row.test, row.prog, f.where, f.execTree, row.why)
			continue
		}
		body, err := os.ReadFile(filepath.FromSlash(row.prog))
		if err != nil {
			t.Errorf("twdIndirect registers %s and the tree does not hold it: %v", row.prog, err)
			continue
		}
		if !strings.Contains(string(body), row.dir) {
			t.Errorf("twdIndirect says %s enumerates %s, and that path does not appear in the program at all — either the program was pointed somewhere else, in which case this pin's reach and its door have silently diverged, or the row was wrong when it was written", row.prog, row.dir)
		}
	}

	var undispositioned []string
	for name, f := range all {
		if !f.test {
			continue
		}
		for _, prog := range f.execTree {
			if registered[prog] {
				continue
			}
			if _, ok := twdExecDispositions[prog]; ok {
				continue
			}
			undispositioned = append(undispositioned, f.where+" "+name+" runs "+prog)
		}
	}
	sort.Strings(undispositioned)
	for _, u := range undispositioned {
		t.Errorf("%s — a tracked program of this repo's own, run by a test in %s, and nothing here says what it reads. If it enumerates a tracked directory, its callers are tree-wide pins and belong in twdIndirect with a door; if it reads only what it is handed, say so in twdExecDispositions. Undispositioned is the one answer this census does not take, because a program's reading is outside Go and the default has to be inclusion.", u, twdTreepinsPkg)
	}

	// Two-way, so the disposition list cannot keep rows for programs nothing
	// runs any more — a dead row is a reader's reason to believe a question
	// was asked that nobody asks.
	execed := map[string]bool{}
	for _, f := range all {
		for _, prog := range f.execTree {
			execed[prog] = true
		}
	}
	var dead []string
	for prog := range twdExecDispositions {
		if !execed[prog] {
			dead = append(dead, prog)
		}
	}
	sort.Strings(dead)
	for _, d := range dead {
		t.Errorf("twdExecDispositions carries %s (%q), which no test in %s hands to exec.Command any more — drop the row rather than leave a disposition for a reading that does not happen", d, twdExecDispositions[d], twdTreepinsPkg)
	}
}

// The mutation check the derivation needs, because the thing being derived is
// a RULE and a rule that stops matching leaves a green register over a class
// it no longer finds. The floors in arm 2 catch a collapse; this catches the
// rule being quietly wrong at the edges, which is how both earlier widenings
// of the posse half were needed.
//
// Driven over source text rather than over the tree: a fixture says what the
// rule must and must not match, so a change to the rule reds here with the
// case it broke rather than reddening arm 2 with a number.
func TestQATheTreepinsEnumerationRuleMatchesWhatItClaims(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		src  string
		want bool
	}{
		{"glob of a tree subdirectory", `func TestX(t *testing.T) { filepath.Glob("docs/notes.d/*.md") }`, true},
		{"walk of the repo root", `func TestX(t *testing.T) { filepath.WalkDir(".", nil) }`, true},
		{"ReadDir of a tree directory", `func TestX(t *testing.T) { os.ReadDir("docs/adr") }`, true},
		{"Join of literals", `func TestX(t *testing.T) { filepath.Glob(filepath.Join("internal", "posse", "*_test.go")) }`, true},
		{"reading one named file is not enumeration", `func TestX(t *testing.T) { os.ReadFile("Makefile") }`, false},
		{"a scratch directory is not a tree path", `func TestX(t *testing.T) { dir := t.TempDir(); os.ReadDir(dir) }`, false},
		{"a Join carrying a variable is not a tree path", `func TestX(t *testing.T) { dir := t.TempDir(); filepath.Glob(filepath.Join(dir, "*.md")) }`, false},
		{"an absolute path is not a tree path", `func TestX(t *testing.T) { os.ReadDir("/etc") }`, false},
		{"a climb out of the tree is not a tree path", `func TestX(t *testing.T) { os.ReadDir("../..") }`, false},
		{"through a helper", `func helper() { filepath.WalkDir("scripts", nil) }
func TestX(t *testing.T) { helper() }`, true},
		{"through two helpers", `func inner() { os.ReadDir("examples/agents") }
func outer() { inner() }
func TestX(t *testing.T) { outer() }`, true},
		{"a helper that enumerates nothing", `func helper() { os.ReadFile("go.mod") }
func TestX(t *testing.T) { helper() }`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := twdFixtureEnumerates(t, c.src); got != c.want {
				t.Errorf("the enumeration rule says %v for this source, want %v — the key the internal/treepins half of the class is derived from no longer means what this file says it means:\n%s", got, c.want, c.src)
			}
		})
	}

	// And the rule is load-bearing over the real tree: with the enumerator
	// set emptied, nothing matches. A rule that would answer the same with
	// its key removed is not deriving anything.
	all, _ := twdParseTreepins(t)
	live := twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })
	n := 0
	for name, f := range all {
		if f.test && live[name] {
			n++
		}
	}
	if n < 14 {
		t.Fatalf("the enumeration rule finds %d tree-wide tests in %s (23 on 2026-10-04) — the key has stopped matching and the register is deriving a class that is missing members", n, twdTreepinsPkg)
	}
	dead := twdReach(all, func(f *twdFunc) bool { return false })
	if len(dead) != 0 {
		t.Fatalf("the reach closure returns %d functions from an empty seed — it is matching something other than its key, so the count above says nothing", len(dead))
	}
}

// twdFixtureEnumerates runs the internal/treepins rule over one snippet of
// source and reports whether its TestX is a member. It goes through
// twdAnalyze — the same function twdParseDir reads the tree with — so a
// fixture that passes here cannot be passing against a second, friendlier
// copy of the rule.
func twdFixtureEnumerates(t *testing.T, src string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture_test.go", "package treepins\n"+src+"\n", 0)
	if err != nil {
		t.Fatalf("parse fixture: %v\n%s", err, src)
	}
	all := map[string]*twdFunc{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		f := twdAnalyze(fn, "fixture")
		all[f.name] = f
	}
	return twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })["TestX"]
}

// Arm 2: every tree-wide pin has a door, and every door names a real one.
// Two-way, because both directions are the bug: a pin with no door is
// ranger-base-ik44f arriving again, and a door naming a test that no longer
// exists is a green `-run` filter that runs nothing.
func TestQAEveryTreeWidePinHasADoor(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	doored := map[string]twdDoor{}
	for _, d := range twdDoors {
		for _, name := range twdVar(t, src, d.variable) {
			if prev, dup := doored[name]; dup {
				t.Errorf("%s is named by both $(%s) and $(%s) — one pin, two doors, and the two can drift apart", name, prev.variable, d.variable)
			}
			doored[name] = d
		}
	}

	// QA_TOOL_PINS is the one membership this arm cannot check by wiring: a
	// tool door (`gofmt -l`) names no test, so a pin listed there is a CLAIM
	// that some tool reads it. Exactly one pin makes that claim today, and
	// its door is held open by gofmtdoor_qa_test.go. A second name arriving
	// here would be a pin declaring itself doored with nothing to show —
	// the cheapest way to silence the check below — so it fails until this
	// arm is taught what the new tool door is and who pins it.
	if tool := twdVar(t, src, "QA_TOOL_PINS"); len(tool) != 1 || tool[0] != "TestTreeIsGofmtClean" {
		t.Errorf("$(QA_TOOL_PINS) = %v, want exactly [TestTreeIsGofmtClean] — that variable records the pins whose door is a TOOL rather than a `-run` filter, which is a claim nothing here can verify. A new entry needs its own arm; a pin moved here to quiet this test has no door at all.", tool)
	}

	// QA_HISTORY_PINS is the second membership this arm cannot check by
	// wiring, for a different reason than QA_TOOL_PINS: its three pins read
	// `git log` in THIS repo, so arm 3 cannot plant drift for them in a
	// copied tree (a `git archive` scratch tree has no .git, and reds them
	// in every arm including the control). They are doored and run clean;
	// what is NOT proven for them is that their door can fail. So the
	// membership is named here, and a fourth pin moved into this variable to
	// dodge arm 3's drift plant fails until this arm is taught why.
	wantHistory := []string{
		"TestPublicationRootCommitOmitsExcludedPaths",
		"TestPublicationRootCommitADRsCarryProvenance",
		"TestPublicationHistoryNeverCarriesTheSeedScript",
		// The fourth, taught here rather than moved in quietly
		// (ranger-base-xndgk FINDING 5): it reads THIS repo's `git rev-list`
		// and `git show` for every version of every shipped example, which
		// is the same reason as the three above — a copied tree has no
		// history to plant drift in. It was undoored because its root came
		// from `git rev-parse --show-toplevel`, a spelling neither class
		// rule could see.
		"TestShippedExampleTableCoversEveryVersionInGitHistory",
	}
	if got := twdVar(t, src, "QA_HISTORY_PINS"); !twdSameSet(got, wantHistory) {
		t.Errorf("$(QA_HISTORY_PINS) = %v, want exactly %v — that variable records the tree-wide pins whose subject is this repo's git HISTORY, which is the one thing a copied tree does not have, so arm 3 runs them clean and plants no drift. A new entry needs its own arm; a pin moved here is a pin whose door nothing has shown can fail.", got, wantHistory)
	}

	// The same membership-by-name treatment for the two doors
	// ranger-base-xndgk FINDING 5 added, and for the same reason as
	// QA_HISTORY_PINS rather than a new one: all four pins READ THE TREE
	// THROUGH GIT — `git ls-files`, `git grep` — so in a copied tree they
	// find no checkout and skip, and arm 3 cannot plant drift for them
	// there. Their membership is therefore named here, so neither variable
	// can become the quiet place to park a pin whose door nothing has shown
	// can fail.
	//
	// These four were the finding: every one of them took its root from
	// `git rev-parse --show-toplevel`, which is stdout from a subprocess and
	// invisible to both class rules, so none of them was in any door
	// variable at all. TestQAIdentityLiteralsNeverAppearInATrackedPath
	// censuses EVERY TRACKED PATH in the repository.
	wantIdentity := []string{
		"TestQAIdentityLiteralsNeverAppearInATrackedPath",
		"TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned",
	}
	if got := twdVar(t, src, "QA_IDENTITY_PINS"); !twdSameSet(got, wantIdentity) {
		t.Errorf("$(QA_IDENTITY_PINS) = %v, want exactly %v — this box's identity literals asked of the tree twice, once of tracked PATHS and once of tracked CONTENT. Both read the tree with git, so arm 3 cannot plant drift for them in a copied tree; a new entry needs its own arm.", got, wantIdentity)
	}
	wantOps := []string{
		"TestQAEveryOpsHitInTrackedMarkdownIsRuled",
		"TestQAOpsShapeTableCanStillSayNo",
		// The instance path-form census and its control, taught here rather
		// than moved in quietly (ranger-base-l9ii). Same door because it is
		// the same question — ADR 0024 D1 ops residue in the public tree —
		// asked of every tracked FILE rather than of tracked markdown: this
		// deployment's constitution checkout written as a live path
		// (`~/src/…`, `$HOME/src/…`), dispositioned per file. And it belongs
		// in a named membership for the same reason the four above do: it
		// reads the tree through `git ls-files`, so in the `git archive`
		// scratch tree arm 3 works in it finds no checkout and skips, and
		// arm 3 cannot plant drift for it there. Its door is shown able to
		// fail by its own control instead, which drives the matcher on
		// planted lines.
		"TestInstancePathFormNeverAppearsInTrackedContentUndispositioned",
		"TestQAInstancePathCensusCanStillSayNo",
	}
	if got := twdVar(t, src, "QA_OPS_PINS"); !twdSameSet(got, wantOps) {
		t.Errorf("$(QA_OPS_PINS) = %v, want exactly %v — the ops-residue census over every tracked markdown file, the instance path-form census over every tracked file, and the control beside each that says it can still say no. Both censuses read the tree with git, so arm 3 cannot plant drift for them in a copied tree; a new entry needs its own arm.", got, wantOps)
	}

	// The second package's named membership, for the same reason the four
	// above are named: $(QA_NOTES_PINS) is the only door variable in the
	// register whose pin is NOT derived from source — the enumeration it
	// stands on happens inside scripts/notes-index.py, which no rule over
	// Go can read. It is carried by twdIndirect and fenced by
	// TestQAEveryTrackedProgramATreePinRunsIsDispositioned; naming it here
	// as well keeps it from becoming the quiet place to park a pin whose
	// membership nothing derived.
	wantNotes := []string{"TestNotesFragmentIndexIsCurrent"}
	if got := twdVar(t, src, "QA_NOTES_PINS"); !twdSameSet(got, wantNotes) {
		t.Errorf("$(QA_NOTES_PINS) = %v, want exactly %v — that variable holds the one pin in the class whose enumeration is a line of Python rather than a line of Go, so nothing here derives its membership. A second entry needs a row in twdIndirect and a reason; a pin moved here to quiet the check below has no door at all.", got, wantNotes)
	}

	names, funcs := twdTreeWideTests(t)
	// A pin over a derived set is satisfied by deriving nothing: say how
	// many test functions were actually parsed, and fail a walk that found
	// far fewer than internal/posse holds (1000+ on 2026-09-04).
	if funcs < 300 {
		t.Fatalf("only %d test functions parsed under internal/posse — the walk this arm derives the class from found nothing", funcs)
	}
	// Two floors, because the two rules fail apart. The union's floor
	// catches a derivation that collapsed; it is NOT what catches a member
	// going invisible one at a time — the old floor here was len < 5, which
	// fired only when qibRepoRoot was renamed and never when a pin was
	// simply spelled with the twin helper (ranger-base-sx2dq). That is
	// TestQAOneRepoRootHelperInTheTestPackage's job, below.
	if len(names) < 14 {
		t.Fatalf("only %d tree-wide tests found (18 on 2026-09-04) — %s has been renamed or wrapped, and this arm is now deriving a class that is missing members: %v", len(names), twdRootHelper, names)
	}
	// And the walk-reaching rule specifically: it is the half that catches a
	// pin reading the tree through a helper, and a rule that silently stops
	// matching leaves the union looking healthy on the direct half alone.
	if reach := twdWalkReachingTests(t); len(reach) < 1 {
		t.Errorf("no test reaches a rooted walk through a helper (at least TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover did on 2026-09-04) — half the class rule is matching nothing")
	}
	t.Logf("parsed %d test functions under internal/posse, %d of them tree-wide", funcs, len(names))

	// The second package (ranger-base-g6sb1). Its own floors, because the
	// two rules fail apart and a collapse on one side must not be covered
	// by the other side's health.
	tpNames, tpDerived, tpFuncs := twdTreepinsTreeWideTests(t)
	if tpFuncs < 200 {
		t.Fatalf("only %d test functions parsed under %s — the walk this arm derives the second half of the class from found nothing", tpFuncs, twdTreepinsPkg)
	}
	if len(tpDerived) < 14 {
		t.Fatalf("only %d tree-wide tests derived in %s (23 on 2026-10-04, of which 20 predate ranger-base-g6sb1's own arms) — the enumeration rule has stopped matching, and this arm is now deriving a class that is missing members: %v", len(tpDerived), twdTreepinsPkg, tpDerived)
	}
	t.Logf("parsed %d test functions under %s, %d of them tree-wide (%d derived, %d registered indirect)", tpFuncs, twdTreepinsPkg, len(tpNames), len(tpDerived), len(twdIndirect))

	// Both ways, per package. Both directions are the bug: a pin with no
	// door is ranger-base-ik44f arriving again, and a door naming a test
	// that is not in its package's class is a `-run` filter matching
	// nothing, which passes in silence.
	for _, class := range []struct {
		pkg   string
		names []string
	}{
		{twdPossePkg, names},
		{twdTreepinsPkg, tpNames},
	} {
		found := map[string]bool{}
		for _, name := range class.names {
			found[name] = true
			d, ok := doored[name]
			if !ok {
				t.Errorf("%s (%s) reads the tree and no Makefile door names it — it is reachable only by a `-run` filter that happens to spell it, which is ranger-base-ik44f arriving again. Give it a door, or add it to one whose subject it shares.", name, class.pkg)
				continue
			}
			if d.pkg != class.pkg {
				t.Errorf("%s is a tree-wide test in %s and $(%s) doors it with `go test ./%s` — the door runs a package the pin does not live in, so its filter matches nothing and it passes in silence", name, class.pkg, d.variable, d.pkg)
			}
		}
		members := twdTestNames(t, class.pkg)
		for name, d := range doored {
			if d.pkg != class.pkg || found[name] {
				continue
			}
			if _, holder := twdDoorHolders[name]; holder {
				if !members[name] {
					t.Errorf("twdDoorHolders registers %s and %s declares no such test — the one exemption from the derivation is holding a door open for a name that is not there", name, class.pkg)
				}
				continue
			}
			if members[name] {
				// The test is there; the DERIVATION stopped calling it
				// tree-wide. That is the more interesting of the two
				// failures and it has a usual cause, so name it: the rule
				// reads a spelling, and a reading rewritten into a
				// spelling it cannot see goes quiet without the pin
				// changing at all. This arm found exactly that on
				// 2026-10-04, when twdTestFiles' glob was briefly composed
				// from a parameter.
				t.Errorf("$(%s) names %s, which IS a test in %s and is no longer derived as tree-wide. Either it stopped reading the tree, in which case drop it from the door — or its reading was rewritten into a spelling the rule cannot see, which is the usual cause: an enumeration root composed from a variable instead of spelled as a literal. The door still passes; the pin is back to being a package-tree run away.", d.variable, name, class.pkg)
				continue
			}
			t.Errorf("$(%s) names %s, which is not a test in %s at all — the door's `-run` filter matches nothing there and passes in silence", d.variable, name, class.pkg)
		}
	}

	// And the exemption list cannot keep rows no door uses: a door-holder
	// nothing doors is a reason recorded for a decision nobody took.
	for name := range twdDoorHolders {
		if _, ok := doored[name]; !ok {
			t.Errorf("twdDoorHolders registers %s and no Makefile door names it — drop the row, or give it the door the row says it needs", name)
		}
	}
}

// twdTestNames is the set of test functions one package declares, used to
// tell "this door names a test that is no longer derived" from "this door
// names nothing at all".
func twdTestNames(t *testing.T, pkg string) map[string]bool {
	t.Helper()
	all, _ := twdParseDir(t, pkg)
	out := map[string]bool{}
	for name, f := range all {
		if f.test {
			out[name] = true
		}
	}
	return out
}

// twdWalkReachingTests is the walk-reaching half of the class on its own:
// tests that are members ONLY because they reach a rooted walk through a
// helper, with no qibRepoRoot call of their own.
func twdWalkReachingTests(t *testing.T) []string {
	t.Helper()
	all, _ := twdParse(t)
	reaches := twdReach(all, func(f *twdFunc) bool { return f.rootedWalk })
	var out []string
	for name, f := range all {
		if f.test && reaches[name] && !f.root {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// The pin ranger-base-sx2dq was filed for. The class above is DERIVED, and a
// derivation is only as wide as the thing it keys on: internal/posse carried
// two byte-identical repo-root helpers, qibRepoRoot and qspRepoRoot, and a
// pin spelled with the second one was outside the class, got no door, and
// nothing said so. Folding the twin made the rule true again; this is what
// keeps a third copy from arriving and quietly undoing it.
//
// Two shapes, both measured against the tree rather than imagined:
//
//   - a TWIN HELPER: a function that asks runtime.Caller where it lives and
//     climbs two levels, which is qibRepoRoot's body written again under
//     another name. Whether or not it walks, its callers are invisible.
//   - a HIDDEN WALKER: a function that walks a tree AND spells the climb to
//     the repo root by hand (`filepath.Abs("../..")`,
//     `filepath.Join(dir, "..", "..")`). TestSeedConfigLiveKeysAreRead was
//     exactly this: a WalkDir over every non-test .go file in the repo,
//     reached by no helper at all, so no identifier match could ever find
//     it.
//   - a SHELLED ROOT: a body that asks git — `exec.Command("git",
//     "rev-parse", "--show-toplevel").Output()` — and keeps the answer.
//     ranger-base-xndgk FINDING 5: FOUR tree-wide pins were spelled this
//     way, one of them a census of EVERY TRACKED PATH in the repository, and
//     none of them was in any door variable. Both rules above key on Go's
//     own filesystem calls, so neither could ever have seen a root that
//     arrives as the stdout of a subprocess. A probe that throws the answer
//     away is not this shape and is not fenced.
//
// Reading ONE file at the repo root is not either shape and is not fenced —
// ~48 test files do it, they red only when that one file changes, and that
// is not this class.
func TestQAOneRepoRootHelperInTheTestPackage(t *testing.T) {
	t.Parallel()
	all, funcs := twdParse(t)
	if funcs < 300 {
		t.Fatalf("only %d test functions parsed under internal/posse — the walk this pin reads is finding nothing", funcs)
	}
	if _, ok := all[twdRootHelper]; !ok {
		t.Fatalf("internal/posse's tests no longer declare %s — the class in this file is derived from it, so every derived arm above is now deriving nothing", twdRootHelper)
	}

	var twins, hidden []string
	for name, f := range all {
		if name == twdRootHelper || f.root {
			continue
		}
		if f.caller && f.ascent {
			twins = append(twins, f.where+" "+name)
		}
		if f.handRolledTreeWalk {
			hidden = append(hidden, f.where+" "+name)
		}
	}
	sort.Strings(twins)
	sort.Strings(hidden)
	for _, w := range twins {
		t.Errorf("%s asks runtime.Caller and climbs two levels — that is a second %s under another name, and every pin spelled with it is outside the tree-wide class this file derives, so it gets no door and nothing says so. Call %s instead. (ranger-base-sx2dq: the tree carried exactly this, byte for byte.)", w, twdRootHelper, twdRootHelper)
	}
	for _, h := range hidden {
		t.Errorf("%s walks a tree from a hand-rolled climb to the repo root — a tree-wide pin no identifier match can reach, which is how TestSeedConfigLiveKeysAreRead went undoored. Take the root from %s.", h, twdRootHelper)
	}

	var shelled []string
	for name, f := range all {
		if f.shellRoot {
			shelled = append(shelled, f.where+" "+name)
		}
	}
	sort.Strings(shelled)
	for _, sh := range shelled {
		t.Errorf("%s asks git for this repo's root and keeps the answer — `exec.Command(\"git\", \"rev-parse\", \"--show-toplevel\").Output()` is a root no rule in this file can see, so a pin spelled with it is outside the tree-wide class and gets no door. Take the root from %s and, if the reading needs a checkout, skip with qibSkipUnlessCheckout. (ranger-base-xndgk FINDING 5: four pins were spelled this way, one of them a census of every tracked path in the repository.)", sh, twdRootHelper)
	}
}

// twdSourceFiles lists what to copy: git's idea of the working tree, which
// leaves out bin/ and dist/ and every other build output.
//
// It falls back to a walk when git cannot answer, and the fallback is not
// decoration: a `git archive | tar -x` scratch tree is the house rig for
// mutation runs on this repo, it has no .git, and four root/internal pins
// already red on that alone in every arm including the control. A fifth
// would be a tax on every future rig, paid to save ten lines here.
func twdSourceFiles(t *testing.T) []string {
	t.Helper()
	if out, err := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output(); err == nil {
		var files []string
		for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
			if rel != "" {
				files = append(files, rel)
			}
		}
		return files
	}
	var files []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// twdSeedTree copies the working tree into a scratch directory: the door's
// recipe compiles internal/posse from there, so qibRepoRoot — which resolves
// off the compiled source's own path — answers the scratch root and the pins
// walk the copy. The WORKING tree and not HEAD, so a pin renamed in one
// commit with its door does not fail this arm against a stale HEAD.
func twdSeedTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copied := 0
	for _, rel := range twdSourceFiles(t) {
		info, err := os.Lstat(rel)
		if err != nil || !info.Mode().IsRegular() {
			// A symlink (plugin/bin/posse points at the installed binary)
			// or a path git knows and the tree does not. Neither compiles
			// and neither is read by the pins.
			continue
		}
		body, err := os.ReadFile(rel)
		if err != nil {
			continue
		}
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied < 500 {
		t.Fatalf("only %d files copied into the scratch tree — the pins below have file-count floors and would fail on the rig rather than on the drift", copied)
	}
	return dir
}

// twdExpand returns `make -n`'s own expansion of a door's recipe, so nothing
// here is a second copy of the command to drift from it.
func twdExpand(t *testing.T, target string) string {
	t.Helper()
	out, err := exec.Command("make", makeExpandFlag, "-n", target).Output()
	if err != nil {
		t.Fatalf("make -n %s: %v", target, err)
	}
	recipe := strings.TrimSpace(string(out))
	if recipe == "" {
		t.Fatalf("`make -n %s` expands to nothing", target)
	}
	if strings.Contains(recipe, "$(") {
		t.Fatalf("the expanded recipe still holds a make variable — this arm would be measuring the wrong text:\n%s", recipe)
	}
	assertRecipeIsOnlyRecipe(t, target, recipe)
	return recipe
}

// Arm 3: the doors can fail, on the drift they exist for.
func TestQATheTreeWideDoorsReportRealDrift(t *testing.T) {
	t.Parallel()
	crew := twdExpand(t, "crew-check")
	seed := twdExpand(t, "seed-check")
	doc := twdExpand(t, "doc-check")
	notes := twdExpand(t, "notes-check")
	dir := twdSeedTree(t)

	run := func(recipe string) (string, error) {
		cmd := exec.Command("sh", "-c", recipe)
		cmd.Dir = dir
		b, err := cmd.CombinedOutput()
		return string(b), err
	}

	// The clean arm first, both doors: a door that always fails detects
	// nothing, and a filter that matches nothing passes in silence.
	for _, door := range []struct{ name, recipe string }{{"crew-check", crew}, {"seed-check", seed}, {"doc-check", doc}, {"notes-check", notes}} {
		got, err := run(door.recipe)
		if err != nil {
			t.Fatalf("`make %s` failed on a clean copy of this tree — it reports drift that is not there:\n%s", door.name, got)
		}
		if strings.Contains(got, "no tests to run") {
			t.Fatalf("`make %s`'s filter matched no test at all, so its green says nothing:\n%s", door.name, got)
		}
	}

	// history-check is the one door this arm cannot copy: its three pins read
	// `git log` in THIS repo, and the scratch tree has no .git — all three
	// red there in every arm including the control, so a copied run would
	// measure the copy and not the door. Run it where it lives instead. It
	// only reads (`git log`, `git rev-list`), and this proves the two halves
	// a filter can get wrong: the recipe runs, and it names real tests.
	{
		out, err := exec.Command("sh", "-c", twdExpand(t, "history-check")).CombinedOutput()
		if err != nil {
			t.Errorf("`make history-check` failed in this repo — the door reports drift that is not there:\n%s", out)
		}
		if strings.Contains(string(out), "no tests to run") {
			t.Errorf("`make history-check`'s filter matched no test at all, so its green says nothing:\n%s", out)
		}
	}

	// The crew door's drift: a crew name in a file the shipped tree walk
	// reads. Assembled, never spelled — this file is inside that walk's
	// repo-root pass, the same reason instancebound_qa_test.go assembles.
	name := "gw" + "art"
	probe := filepath.Join("internal", "twd_drift_probe.txt")
	if err := os.WriteFile(filepath.Join(dir, probe), []byte("seat: "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := run(crew)
	if err == nil {
		t.Fatalf("`make crew-check` passed a tree naming the originating instance's crew — the door reads nothing:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(probe), name, "TestShippedTreeNamesRolesNotThisCrew"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make crew-check` failed without naming %q, so it says a seat is wrong and not where:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, probe)); err != nil {
		t.Fatal(err)
	}

	// The seed door's drift: the retired harness name on the published
	// surface. Assembled for the same reason the crew probe is — this file
	// is inside the walk that reads it.
	stale := "ranger" + "hq"
	seedProbe := filepath.Join("internal", "twd_seed_drift_probe.txt")
	if err := os.WriteFile(filepath.Join(dir, seedProbe), []byte("harness: "+stale+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(seed)
	if err == nil {
		t.Fatalf("`make seed-check` passed a tree carrying the retired harness name on the seed surface — the door reads nothing, or its members all SKIPPED, which is the same green:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(seedProbe), "TestSeedSurfaceNameCountIsZero"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make seed-check` failed without naming %q, so it says the surface is dirty and not where:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, seedProbe)); err != nil {
		t.Fatal(err)
	}

	// The doc door's drift: the ADR 0019 framing that was retired, back in a
	// shipped .go file — the exact way it walked in before, as a comment
	// nobody's test looks at. Assembled, so this file is not itself a hit
	// when a wider scanner than coxnGoSources is pointed at the tree.
	dead := "stale " + "leftover"
	docProbe := filepath.Join("internal", "twd_doc_drift_probe.go")
	body := "package internal\n\n// the on-disk credential file is a " + dead + " of a keychain login\n"
	if err := os.WriteFile(filepath.Join(dir, docProbe), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(doc)
	if err == nil {
		t.Fatalf("`make doc-check` passed a shipped source carrying the framing ADR 0019's amendment retired — the door reads nothing:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(docProbe), dead, "TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make doc-check` failed without naming %q:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, docProbe)); err != nil {
		t.Fatal(err)
	}

	// The notes door's drift, and the one this bead was filed for
	// (ranger-base-g6sb1): a docs/notes.d fragment with no entry in that
	// directory's generated index. Every bead on this box writes one of
	// these, which is why it is the drift that got out — `make fmt-check`,
	// all eight doors as they then stood and a whole
	// `-tags posse_arm2 ./internal/posse` called the commit clean, and only
	// the 589.965s arm-1 run of internal/treepins said `stale index`.
	//
	// Not assembled, unlike the three probes above: a notes fragment is
	// data in a directory nothing in this package scans for content, so
	// there is no walk for this file's own bytes to turn up in.
	notesProbe := filepath.Join("docs", "notes.d", "twd-drift-probe.md")
	if err := os.WriteFile(filepath.Join(dir, notesProbe), []byte("# A fragment no index knows about\n2026-10-04\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(notes)
	if err == nil {
		t.Fatalf("`make notes-check` passed a tree holding a notes fragment the index does not list — the door reads nothing, and the drift that filed ranger-base-g6sb1 is undetected again:\n%s", got)
	}
	for _, want := range []string{"README.md", "stale index", "scripts/notes-index.py"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make notes-check` failed without naming %q, so it says the index is wrong and not what to run:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, notesProbe)); err != nil {
		t.Fatal(err)
	}
}

// The other half of arm 3 for the internal/treepins doors: that make's own
// expansion of each `-run` filter NAMES the pins its variable holds.
//
// WHY NOT THE DRIFT PLANT. The four doors above are exercised against a
// scratch copy of the tree, which is the strongest thing this file does. It
// is not available here, for three reasons that each hold on their own: the
// copy has no .git, and several of these pins read the tree through git; a
// `go test ./internal/treepins` in the copy is a cold build of the module
// rather than a door's worth of seconds; and register-check's own pins
// include THIS test, so arm 3 running it would be arm 3 running arm 3.
//
// What is left is the half a filter actually gets wrong. `-run` is a regex,
// it is assembled by make out of a variable and a `$$`, and a filter that
// matches nothing exits 0 and prints `ok`. Arm 2 proves the NAMES are real
// tree-wide tests in the right package; this proves that make's expansion of
// them, run by go, selects exactly those tests. It uses `-list` in place of
// `-run` — one substitution on make's own text, which is why the regex under
// test is still make's and not a copy of it — so it selects without running:
// ~0.3s per door against the ~31s of running them.
func TestQAEveryTreepinsDoorFilterNamesItsPins(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	for _, door := range twdDoors {
		if door.pkg != twdTreepinsPkg || door.tool {
			continue
		}
		recipe := twdExpand(t, door.target)
		listing := strings.Replace(recipe, "-run ", "-list ", 1)
		if listing == recipe {
			t.Errorf("`make %s` has no `-run ` to read as a `-list `, so this arm cannot ask what its filter selects:\n%s", door.target, recipe)
			continue
		}
		cmd := exec.Command("sh", "-c", listing)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("listing `make %s`'s filter failed: %v\n%s", door.target, err, out)
			continue
		}
		var listed []string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Test") {
				listed = append(listed, strings.TrimSpace(line))
			}
		}
		want := twdVar(t, src, door.variable)
		if !twdSameSet(listed, want) {
			t.Errorf("`make %s`'s filter selects %v, and $(%s) names %v — the regex make assembles and the pins the variable holds are not the same set, so the door runs fewer tests than its own membership says and still prints `ok`:\n%s", door.target, listed, door.variable, want, listing)
		}
	}
}

// twdSpelled spells a non-negative integer the way this file's head comment
// does. The comment is prose and says "twenty pins", not "20 pins", so the
// pin below has to compare against a WORD; the alternative — rewriting the
// sentence in digits so a test can read it — would let the test choose how
// the file reads, which is backwards.
func twdSpelled(n int) string {
	ones := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	switch {
	case n < 0 || n > 99:
		return strconv.Itoa(n) // out of the prose range; the pin will name the mismatch
	case n < 20:
		return ones[n]
	case n%10 == 0:
		return tens[n/10]
	default:
		return tens[n/10] + "-" + ones[n%10]
	}
}

// twdHead returns this file's own head comment as flowed prose — `//` markers
// stripped, lines joined with a space. Flowed, not per-line, because the
// claim it holds WRAPS (today "... over three runs" then "at twenty pins and
// seven doors", and the break moves whenever the sentence is rewrapped), and
// a per-line scan is blind to exactly that.
func twdHead(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("internal/treepins/treewidedoor_qa_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "package treepins" || line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break // the head comment is over; `import (` is the first line here
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "//")))
	}
	if len(out) == 0 {
		t.Fatal("this file has no head comment — the sentence arm 4 holds is gone, and with it the number a seat prices `make tree-check` from")
	}
	return strings.Join(out, " ")
}

// twdCountClaim matches the ONE sentence in the head comment that says how
// big the class is. Deliberately narrow: the comment is full of correctly
// frozen history ("five pins took their root from ...", "five members became
// thirteen"), and a rule that read those as live claims would red on prose
// that is true.
var twdCountClaim = regexp.MustCompile(`([a-z]+(?:-[a-z]+)?) pins and ([a-z]+(?:-[a-z]+)?) doors`)

// Arm 4: the head comment's count is the Makefile's count (ranger-base-4jogv).
//
// WHY THIS IS A PIN AND NOT A ONE-TIME CORRECTION. Both numerals were wrong
// at main, and they went wrong by two different routes, neither of which any
// existing arm can see:
//
//   - the DOORS number entered wrong (d189b623 wrote "seven doors" over an
//     enumeration of eight) and was then faithfully decremented to six when
//     the selector door and its pin were removed — a correct edit applied to
//     a wrong number stays wrong;
//   - the PINS number drifted with no edit at all, because a bead that adds
//     a name to $(QA_DOC_PINS) has no reason to read this file's prose.
//
// Arm 2 is two-way and mechanical, so no pin is undoored and no door is
// empty — the MECHANISM was never wrong. What was wrong is the sentence a
// seat reads to decide whether `make tree-check` is worth typing, in the one
// file whose entire subject is "the doors are wide enough". So the count is
// derived here, and the enumeration it rests on is held one-way: every name
// in a door variable must be named up there. One-way on purpose — the head
// also names tests that are NOT members (TestQAOneRepoRootHelperInTheTestPackage
// is the fence, not a pin), and a two-way rule would red on those.
//
// AND THE README's DOOR TABLE, for the same reason one page further out
// (ranger-base-r5546). internal/treepins/README.md carries the doors as a
// table, one row per door with its subject — more useful than a
// bare list, and the part that can drift. It was 14/14 correct the day it
// shipped and nothing would have said so when it stopped being: a door
// renamed, added or removed edits the Makefile and this file's enumeration,
// both of which are pinned, and leaves the public page saying what used to
// be true. Order too, not just membership — the table reads as the order
// `make tree-check` runs them, and a reader pricing a partial run from it
// is owed that.
//
// AND AGENTS.md's DOOR BLOCK AND ITS ABSENT COUNT (ranger-base-xed72). The
// crew-wide page carried the door list a third time and the PIN count a
// second time, and the count is the one that had already gone wrong: four
// short of the sentence above, because a bead that doors a pin corrects the
// file it edits and standing orders are nobody's subject. The sentence's own
// failure text says why that cannot be fixed by typing the right number —
// "two places drift apart and the reader cannot tell which is stale" — so
// the page states no count at all now, names this file instead, and this arm
// holds all three of those: the block against `tree-check`'s prerequisites
// (membership and order, like the README's), the pointer, and the absence of
// a numeral.
func TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// The pins, deduped across door variables: arm 2 already fails a name
	// carried by two doors, so counting the union rather than the sum keeps
	// this arm from reporting a second, derived symptom of that one bug.
	seen := map[string]bool{}
	var pins []string
	for _, v := range twdPinVars {
		for _, name := range twdVar(t, src, v) {
			if seen[name] {
				continue
			}
			seen[name] = true
			pins = append(pins, name)
		}
	}
	// Tokens, and stopping at the `#`: a commented-out door is not a door.
	// The byte reader this replaced counted the comment marker AND the names
	// behind it, so this arm answered by accident either way — `# crew-check
	// ...` counted the `#` as an eighth door and red for a reason that has
	// nothing to do with the six it had just silenced, and `#ops-check`, no
	// space, still counted seven while `make tree-check` ran six
	// (ranger-base-7kwb8).
	_, doors := mkPrereqs(t, src, "tree-check")

	head := twdHead(t)

	// One live claim, and it is the derived one. Two matches means a second
	// sentence started counting and the two can now disagree; zero means the
	// claim moved out from under the pin that guards it.
	claims := twdCountClaim.FindAllStringSubmatch(head, -1)
	if len(claims) != 1 {
		t.Fatalf("the head comment carries %d `<n> pins and <n> doors` claims, want exactly 1 — the count of this class is said in one place on purpose, because two places drift apart and the reader cannot tell which is stale: %v", len(claims), claims)
	}
	gotPins, gotDoors := claims[0][1], claims[0][2]
	wantPins, wantDoors := twdSpelled(len(pins)), twdSpelled(len(doors))
	if gotPins != wantPins || gotDoors != wantDoors {
		t.Errorf("the head comment says %q pins and %q doors; the Makefile has %s (%d) and %s (%d).\n"+
			"  door variables: %v\n"+
			"  tree-check prerequisites: %v\n"+
			"A seat prices `make tree-check` from that sentence. Fix the sentence, not this test — and if a pin or a door really did come or go, the enumeration above the sentence needs the same edit.",
			gotPins, gotDoors, wantPins, len(pins), wantDoors, len(doors), twdPinVars, doors)
	}

	// And the enumeration the count rests on. Without this, the count stays
	// green while the list under it goes short — which is how the pins
	// number drifted in the first place: three tests were doored by beads
	// that never touched this comment.
	for _, name := range pins {
		// Word-bounded: a plain strings.Contains is a SUBSTRING test, so a
		// pin renamed to a strict prefix of a name already sitting in this
		// comment (e.g. a since-removed long name in the inventory above)
		// would satisfy it without being named here (ranger-base-erqvh row 1).
		if !regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(head) {
			t.Errorf("$(%s) door variable names %s, and this file's head comment does not — the enumeration the count sentence rests on is short by at least one, so the next reader counts a smaller class than `make tree-check` runs", twdVarOf(t, src, name), name)
		}
	}

	// And the README's table of the same doors, membership AND order.
	readme := filepath.Join(twdTreepinsPkg, "README.md")
	table := twdReadmeDoors(t, readme)
	if !slices.Equal(table, doors) {
		t.Errorf("%s lists the doors as\n  %v\nand `make tree-check`'s prerequisites are\n  %v\n"+
			"That table is a second copy of this list in a page read outside this package, and nothing but this arm would ever say it had gone stale. Fix the table, not this test.", readme, table, doors)
	}

	// And AGENTS.md's door block, held the same way — plus the count that
	// page is no longer allowed to write (ranger-base-xed72).
	agents := twdAgentsSection(t)
	block := twdAgentsDoors(t, agents)
	if !slices.Equal(block, doors) {
		t.Errorf("%s's tree-wide-pin bullet lists the doors as\n  %v\nand `make tree-check`'s prerequisites are\n  %v\n"+
			"That block is the door list every seat reads at session start, and nothing but this arm would ever say it had gone stale. Fix the block, not this test.", twdAgentsPath, block, doors)
	}
	flowed := strings.Join(agents, " ")
	if counts := twdProseCount().FindAllString(flowed, -1); len(counts) > 0 {
		t.Errorf("%s's tree-wide-pin bullet counts this class in prose (%q), and nothing holds that numeral — which is how it came to say four fewer pins than the Makefile had. Say it without the count (`all of them`, `two of them`): the one place a count of this class is written is this file's head comment, derived from the Makefile above.", twdAgentsPath, counts)
	}
	if !strings.Contains(flowed, "treewidedoor_qa_test.go") {
		t.Errorf("%s's tree-wide-pin bullet no longer names treewidedoor_qa_test.go — that page states no count of this class on purpose and points at the file that does, so a reader who loses the pointer has nowhere left to get the number.", twdAgentsPath)
	}
}

// twdReadmeDoors reads the door names out of the README's door table: the
// rows of the one table whose first column is a `make <door>` in backticks,
// in the order they appear.
//
// Shaped rather than grepped, and deliberately narrow about what a row is —
// the file names `make tree-check`, `make crew-check` and `make notes-check`
// in its PROSE as well, and a reader that took those would be reading the
// page's advice as if it were the list.
func twdReadmeDoors(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v — the package page carries the door table this arm holds to the Makefile", path, err)
	}
	row := regexp.MustCompile("^\\|\\s*`make ([a-z0-9-]+)`\\s*\\|")
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s has no door table — a row is a table line whose first cell is a backticked `make <door>`, and a page that lost them is a page that stopped saying what `make tree-check` runs", path)
	}
	return out
}

// twdAgentsPath is the crew-wide page that carries the third copy of the door
// list. Read by NAME, like the Makefile and the README: that is the
// enumeration rule's own negative case, so this arm stays a door-holder
// rather than becoming a derived member of the class (twdDoorHolders).
const twdAgentsPath = "AGENTS.md"

// twdAgentsSection returns the lines of AGENTS.md's tree-wide-pin bullet,
// trimmed: from the bullet that opens "A `-run` filter cannot reach a
// tree-wide pin" to the next top-level bullet or the end of the file.
//
// Scoped to the one bullet and not the page, because the rest of AGENTS.md
// counts its own things honestly — "121 pattern kills from 63 seats", "three
// binaries now" — and a page-wide count census would red on prose that is
// true. The bullet is found by its own sentence rather than a line number,
// and a page that has lost that sentence fails here rather than passing over
// nothing.
func twdAgentsSection(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(twdAgentsPath)
	if err != nil {
		t.Fatalf("%s: %v — the crew-wide page carries the door block this arm holds to the Makefile", twdAgentsPath, err)
	}
	lines := strings.Split(string(b), "\n")
	start := -1
	for i, l := range lines {
		if strings.Contains(l, "filter cannot reach a tree-wide pin") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s has no `-run filter cannot reach a tree-wide pin` bullet — that bullet is where a seat learns these doors exist at all, and this arm cannot hold a block it cannot find", twdAgentsPath)
	}
	out := []string{strings.TrimSpace(lines[start])}
	for _, l := range lines[start+1:] {
		if strings.HasPrefix(l, "- ") { // the next top-level bullet
			break
		}
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

// twdAgentsDoors reads the door names out of the fenced block in that bullet:
// a line whose first token is `make <door>`, inside the fence.
//
// Fenced, and shaped rather than grepped, for the reason twdReadmeDoors is:
// the bullet names `make fmt-check`, `make tree-check`, `make crew-check` and
// `make notes-check` in its PROSE as advice, and a reader that took those
// would be reading the advice as if it were the list.
func twdAgentsDoors(t *testing.T, section []string) []string {
	t.Helper()
	row := regexp.MustCompile(`^make ([a-z0-9-]+)\s`)
	var out []string
	fenced := false
	for _, l := range section {
		if strings.HasPrefix(l, "```") {
			fenced = !fenced
			continue
		}
		if !fenced {
			continue
		}
		if m := row.FindStringSubmatch(l); m != nil {
			out = append(out, m[1])
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s's tree-wide-pin bullet has no door block — a row is a fenced line whose first token is `make <door>`, and a page that lost them is a page that stopped saying what `make tree-check` runs", twdAgentsPath)
	}
	return out
}

// twdProseCount matches a count of this class written in prose: one of this
// shop's spelled numerals (or digits) before `pins` or `doors`, and the
// `all <n>` form the door count was also written in. The numerals come from
// twdSpelled rather than being retyped, so this rule and the derivation above
// cannot disagree about what a count looks like.
//
// Narrow on purpose, the same way twdCountClaim is: it catches the two
// spellings AGENTS.md actually carried — "forty-nine pins across the two,
// behind fourteen doors" and "`make tree-check` is all fourteen" — and leaves
// prose that counts something else alone.
func twdProseCount() *regexp.Regexp {
	alts := make([]string, 0, 101)
	for n := 99; n >= 0; n-- { // descending, so `forty-nine` is tried before `forty`
		alts = append(alts, regexp.QuoteMeta(twdSpelled(n)))
	}
	alts = append(alts, `\d+`)
	num := `(?:` + strings.Join(alts, "|") + `)`
	return regexp.MustCompile(`(?i)\b(?:` + num + `\s+(?:pins|doors)|all\s+` + num + `)\b`)
}

// twdVarOf names the door variable that carries a test, for the message above.
func twdVarOf(t *testing.T, makefile, name string) string {
	t.Helper()
	for _, v := range twdPinVars {
		for _, n := range twdVar(t, makefile, v) {
			if n == name {
				return v
			}
		}
	}
	return "unknown"
}
