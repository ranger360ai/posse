//go:build posse_arm3

package posse

// CHECK 3 HAS A FOURTH LITERAL SOURCE, AND IT SCANS ONE SUBJECT: THE CREW
// NAMES OVER ADDED STAGED PATHS (ADR 0012 D2 / App.A 5 through ADR 0024 D2
// check 3 — ranger-base-cdxpf, from ranger-base-o3g6a).
//
// THE HOLE. check 3 has had a staged-PATH arm since ranger-base-wlsv1, and
// what it matches is DeriveIdentityLiterals: whoami, every scope's git
// e-mail, the instance repo path. A file named after a SEAT carries none of
// those. So a probe named for a QA seat was added, committed clean, and rode
// main for a day until ranger-base-o3g6a taught the two shipped pins to read
// a file's NAME as well as its lines. That fix is the suite catching it after
// the fact; this is the commit refusing it.
//
// THE THREE DECISIONS ranger-base-cdxpf was filed with, and where each one is
// pinned here:
//
//  1. WHICH NAMES — derived from the box, never a shipped list, because a
//     hardcoded crew list in this tree would BE what ADR 0012 App.A 5
//     forbids. ListAgents, less every name posse has ever shipped an example
//     PID under. PIN (a) and PIN (d).
//  2. PATH OR CONTENT — path only. A crew name in a staged LINE is legitimate
//     where ADR 0012 D2 leaves it (docs/, the root narrative, a
//     D6-grandfathered id) and a commit message names the persona who wrote
//     it; a content arm would refuse what the constitution allows. PIN (c)
//     holds the decision so widening it later is a deliberate edit.
//  3. BOUNDARIES — none, because the separator in Go's own file names is an
//     underscore and no word boundary fires beside one (measured in
//     ranger-base-o3g6a: the line pattern does not match the file that got
//     through). PIN (b) stages exactly that shape.
//
// A FOURTH DECISION, and cdxpf did not make it (ranger-base-p7e0z, found by
// the four-close verify ranger-base-krjdz):
//
//  4. WHICH TREE — the ones ADR 0012 D6 puts INSIDE App.A 5. D6's edge is
//     "the tree, not the syntax", and it names the other side in as many
//     words: "docs/ and the root narrative files are the development
//     record, where the crew are historical actors." cdxpf's arm had no
//     root filter at all, so it refused a path the constitution lets stand
//     and sent the writer to rename it. PIN (f) is that decision, and PIN
//     (b)'s capitalized case moved INTO the governed tree to make room for
//     it.
//
// THE MEASUREMENT BEHIND DECISION 1, and it is what makes the exclusion the
// load-bearing half rather than a nicety (censused at the fix over this
// repo's 830 tracked paths): the 11 PID names this instance staffs hit ONE
// path. The 9 names the seed ships hit 285, one of them 273 on its own:
// every *_qa_test.go in the tree. A wall built from the seed's roles would
// refuse a new test file on a fresh install, which is the "refuses honest
// commits" failure the bead was filed against. PIN (d) is that census as a
// pin.
//
// THAT ONE PATH IS THE MEASUREMENT BEHIND DECISION 4. cdxpf's close called
// it "the real hit"; it is an ADR under docs/, which D6 excludes, so it is
// the ONLY thing the census found and the only thing the rule does not
// reach. Re-censused at p7e0z over 841 tracked paths: same one path, and
// ZERO under the trees App.A 5 governs. A wall whose entire live catch is
// outside its own rule is a wall that has only ever been wrong.
//
// The fixture crew is invented here and is neither this instance's nor the
// seed's — this file ships, and a pin that named a real seat would be the
// defect it exists to prevent.

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// The fixture PIDs. Not a crew, not a role: two names nothing in any tree
// carries, one of them hyphenated so the joined spelling has something to be
// derived from.
const (
	qaCrewName   = "zephyrina"
	qaCrewHyphen = "quint-us"
)

// qaCrewCap is the capitalized spelling, which two pins stage: the match is
// case-insensitive, and a capitalized file name is where a name survives a
// lower-case convention. Built here rather than written out so the fixture
// name has exactly one spelling in this file.
var qaCrewCap = strings.ToUpper(qaCrewName[:1]) + qaCrewName[1:]

// crewWall is a visWall whose home staffs PIDs, with the hook re-stamped from
// an App that can see them. Re-stamped rather than built that way because the
// wall makes its own home: newVisWallCfg's App has no AgentsDir, so every
// other pin over this harness derives no crew names at all and is untouched
// by this file.
func crewWall(t *testing.T, names ...string) (*visWall, *App) {
	t.Helper()
	w := newVisWall(t)
	a := crewApp(t, w, names...)
	for _, repo := range []string{w.pub, w.priv} {
		if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
			t.Fatal(err)
		}
	}
	return w, a
}

// crewApp staffs the wall's home and returns the App that sees it. The PID
// bodies are irrelevant — ListAgents reads the directory, which is the whole
// point of deriving from the box.
func crewApp(t *testing.T, w *visWall, names ...string) *App {
	t.Helper()
	for _, n := range names {
		write(t, filepath.Join(w.home, "agents", n+".md"), "---\nname: "+n+"\n---\n\nfixture PID.\n")
	}
	return &App{ConfigPath: filepath.Join(w.home, "config.yaml"), AgentsDir: filepath.Join(w.home, "agents")}
}

// PIN (a): the derived set is the staffed PIDs, the hyphenated one's joined
// spelling with it, and nothing else — every literal flagged PathsOnly, which
// is what keeps it out of the content and message arms two pins down.
//
// The seed's own roles are the CONTROL and they are staffed here on purpose:
// a home that carries both must derive the operator's names and none of
// posse's. The fixture premise is asserted rather than assumed — a seed that
// shipped no example PIDs would make this pin green over nothing.
//
// MUTATION-CHECKED (go test -overlay, tree untouched, runs on
// ranger-base-cdxpf):
//   - the shipped-role exclusion dropped from DeriveCrewLiterals: this pin
//     and PIN (d) go red, nothing else moves.
//   - the joined spelling dropped: this pin and PIN (b)'s joined-spelling
//     case go red, and only those two.
func TestQACrewLiteralsAreTheStaffedPIDsLessTheShippedRoles(t *testing.T) {
	w := newVisWall(t)
	roles := shippedRoleNamesForTest(t)
	a := crewApp(t, w, append([]string{qaCrewName, qaCrewHyphen}, roles...)...)

	got := map[string]bool{}
	for _, lit := range a.DeriveCrewLiterals() {
		if lit.Class != CrewLiteralClass {
			t.Errorf("a crew literal must carry the class a refusal names (%q), got %q", CrewLiteralClass, lit.Class)
		}
		if !lit.PathsOnly {
			t.Errorf("crew literal %q must be PathsOnly — the content and message arms are not this rule's", lit.Value)
		}
		got[lit.Value] = true
	}

	for _, want := range []string{qaCrewName, qaCrewHyphen, "quintus"} {
		if !got[want] {
			t.Errorf("the crew literals must carry %q (the staffed PIDs and the joined spelling of a hyphenated one), got %v", want, crewKeys(got))
		}
	}
	for _, role := range roles {
		if got[role] {
			t.Errorf(`the wall must not be built from a name posse itself ships (%q): a role name is what ADR 0012 D2 tells a
writer to rename TO, so refusing it would refuse the remedy — and the seed's own
roles hit 285 of this repo's paths where this instance's crew hit 1.`, role)
		}
	}
	if len(got) != 3 {
		t.Errorf("the set is the staffed PIDs and nothing else, got %v", crewKeys(got))
	}
}

// PIN (b): a staged PATH naming a crew seat is refused in the public repo,
// in the crew's own words, naming the path AND the name — the reader is the
// instance the name belongs to and the remedy is a rename they have to be
// able to make. Three shapes, all of them the ones that got through
// something: the underscore-separated Go test file (no word boundary ever
// fires beside `_`), a capitalized spelling (the match is case-insensitive),
// and the joined spelling of a hyphenated PID.
//
// ALL THREE ARE UNDER internal/, and that is load-bearing since
// ranger-base-p7e0z: the capitalized case used to be an ADR under docs/,
// which is outside the tree App.A 5 governs — it now COMMITS, and PIN (f)
// is where it went. What survives here is the property it was staged for,
// the case fold, tested where the rule actually reaches.
//
// THE CONTROL: the same path in the PRIVATE-stamped repo of the same wall
// commits clean and logs nothing. This arm is inside the visibility gate with
// the rest of check 3 — a crew name is about what may go PUBLIC.
//
// MUTATION-CHECKED:
//   - the crew source never rendered: all three cases here go red and so
//     does PIN (e); PIN (c) stays green, because it asserts commits.
//   - crewLiteralERE replaced by identityLiteralERE (no case fold): the
//     capitalized case alone goes red.
//   - the whole check-3 block rendered ABOVE the visibility gate: the
//     private CONTROL reds and all three public cases stay green — two
//     mutants, told apart.
func TestQACrewNameInAStagedPathIsRefused(t *testing.T) {
	w, _ := crewWall(t, qaCrewName, qaCrewHyphen)
	const clean = "package posse\n\n// nothing here names anybody.\n"

	for _, c := range []struct{ rel, matched string }{
		{"internal/posse/" + qaCrewName + "_as19_probe_test.go", qaCrewName},
		{"internal/posse/" + qaCrewCap + "_pulse_test.go", qaCrewCap},
		{"internal/posse/quintus_probe_test.go", "quintus"},
	} {
		t.Run(c.rel, func(t *testing.T) {
			w.stage(t, w.pub, c.rel, clean)
			out, err := w.git(w.pub, w.persona, "commit", "-m", "a message that names nobody", "--", c.rel)
			if err == nil {
				t.Fatalf("a crew name in a staged PATH must be refused:\n%s", out)
			}
			for _, want := range []string{
				"refused by posse gate: a crew persona name in a staged PATH",
				"ADR 0012 D2 and App.A 5",
				"the FILENAME, not its content:",
				"  " + c.rel,
				CrewLiteralClass + ":",
				"matched: " + c.matched,
				"name the file for the ROLE, not the seat",
				"this repo's beads db is marked: public",
				VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the refusal must carry %q:\n%s", want, out)
				}
			}
			// Another source's words on this hit would mean this pin is one
			// of theirs wearing a persona name.
			for _, never := range []string{
				"an operator identity literal",
				"an instance-defined visibility class",
				"matched in the staged additions:",
				"matched in the commit message:",
			} {
				if strings.Contains(out, never) {
					t.Errorf("a crew-name hit must not be refused in another source's words (%q):\n%s", never, out)
				}
			}
			if !strings.Contains(w.log(t), crewScanLabel+" [prepare-commit-msg hook] (public repo, staged path)") {
				t.Errorf("the refusal must be logged under the crew label, naming the subject:\n%s", w.log(t))
			}
			w.unstage(t, w.pub, c.rel)
		})
	}

	// THE CONTROL: the same name, the same path shape, the PRIVATE repo.
	before := w.log(t)
	rel := "internal/posse/" + qaCrewName + "_ctl_test.go"
	w.stage(t, w.priv, rel, clean)
	if out, err := w.git(w.priv, w.persona, "commit", "-m", "private", "--", rel); err != nil {
		t.Errorf("control: a PRIVATE repo runs no check 3 at all — this path must commit: %v\n%s", err, out)
	}
	if w.log(t) != before {
		t.Errorf("control: nothing may be logged for a private repo:\n%s", strings.TrimPrefix(w.log(t), before))
	}
}

// PIN (c): ONE SUBJECT. The same crew name in a staged LINE, and in the
// commit MESSAGE, commits clean in the PUBLIC repo — which is the decision
// ranger-base-cdxpf turned on, not an accident of where the code went. ADR
// 0012 D2 leaves the crew standing in docs/ and the root narrative as
// historical actors and D6 grandfathers ids, and a commit message names the
// persona who wrote it; a content arm here would refuse text the constitution
// allows. The path stays spotless in both halves, so nothing but the subject
// under test can be what decided it.
//
// MUTATION-CHECKED: a content arm added to the crew source (its own words
// over stagedLineMatched) reds this pin; a message arm added reds it too.
// Neither moves PIN (b) or any other pin in this file.
func TestQACrewNameIsRefusedInAPathOnly(t *testing.T) {
	w, _ := crewWall(t, qaCrewName)

	line := "package posse\n\n// " + qaCrewName + " renamed this the day it landed.\n"
	if out, err := qaMsgCommit(t, w, w.pub, "internal/posse/clean_line_test.go", line, "-m", "clean message", w.persona); err != nil {
		t.Errorf(`a crew name in a staged LINE is not this arm's subject and must commit: %v
%s`, err, out)
	}
	msg := "wire the reaper\n\npicked up where " + qaCrewName + " left off\n"
	if out, err := qaMsgCommit(t, w, w.pub, "internal/posse/clean_msg_test.go", "package posse\n", "-F -", msg, w.persona); err != nil {
		t.Errorf(`a crew name in the commit MESSAGE is not this arm's subject and must commit: %v
%s`, err, out)
	}
	if l := w.log(t); l != "" {
		t.Errorf("nothing may be refused or logged for either subject:\n%s", l)
	}
}

// PIN (d): a home staffed ONLY with the names posse ships arms no crew source
// at all — no checks, no refusal, no variable — and a fresh *_qa_test.go
// commits clean under it. This is the census as a pin (the seed's roles hit
// 285 of this repo's 830 tracked paths, one of them 273 on its own), and it
// is the half that decides whether the wall is usable on a seeded home.
//
// MUTATION-CHECKED: the shipped-role exclusion dropped from
// DeriveCrewLiterals — this pin and PIN (a) go red together, and nothing
// else in the file moves.
func TestQAShippedRoleNamesDoNotArmTheCrewArm(t *testing.T) {
	w, a := crewWall(t, shippedRoleNamesForTest(t)...)

	if lits := a.DeriveCrewLiterals(); len(lits) != 0 {
		t.Fatalf("a home staffed with posse's own example roles must derive no crew literals, got %+v", lits)
	}
	hook := qaHookFile(t, w.pub)
	for _, never := range []string{crewScanLabel, "posse_nbad", "a crew persona name in a staged PATH"} {
		if strings.Contains(hook, never) {
			t.Errorf("an unarmed crew source must render nothing at all, found %q in the hook", never)
		}
	}
	rel := "internal/posse/newthing_qa_test.go"
	if out, err := qaMsgCommit(t, w, w.pub, rel, "package posse\n", "-m", "a new qa pin", w.persona); err != nil {
		t.Errorf(`a role name is what ADR 0012 D2 tells a writer to rename TO, so the wall may not be
built from one: this file must commit on a seeded home: %v
%s`, err, out)
	}
}

// PIN (e): every renderer of this hook derives the SAME literals, crew names
// included. The installed file is compared against the render a caller makes
// with a.commitGuardLiterals — the one derivation all four sites share — and
// against the render made with the identity literals ALONE, which must
// DIFFER. Without the second half this pin is green over a wall that derives
// no crew names at all.
//
// MUTATION-CHECKED: the crew source never rendered reds this pin along with
// PIN (b) — the installed file and the commitGuardLiterals render agree
// again only because neither carries the names.
//
// It is the ADR 0023 property that makes this load-bearing rather than tidy:
// the L3 probe decides "ours" by rendering the hook again and comparing bytes
// (l3HookProbe), and the session-hooks redirect writes the same bytes into a
// dir of its own. A site deriving a different set reads as "ours but stale"
// on every launch — ranger-base-up22's defect, waiting behind a second
// literal source.
func TestQAEveryCommitGuardRendererDerivesTheCrewNames(t *testing.T) {
	w, a := crewWall(t, qaCrewName)

	lits, err := a.commitGuardLiterals(w.pub)
	if err != nil {
		t.Fatal(err)
	}
	installed := qaHookFile(t, w.pub)
	if want := CommitGuardHook(VisibilityPublic, a.OpsPatternSet(), lits...); installed != want {
		t.Errorf("the installed hook must be what commitGuardLiterals renders — a second derivation reads as stale on every launch")
	}
	identity, err := DeriveIdentityLiterals(hookRepo(w.pub))
	if err != nil {
		t.Fatal(err)
	}
	if idOnly := CommitGuardHook(VisibilityPublic, a.OpsPatternSet(), identity...); installed == idOnly {
		t.Error("the wrong arm: a render from the identity literals ALONE must differ from the installed hook, or this pin measured nothing")
	}
}

// PIN (f): the TREE filter (ranger-base-p7e0z). A staged path App.A 5 does
// not reach COMMITS with the crew name in it, and logs nothing; a path the
// rule does reach is still refused. Both halves in one pin, because either
// alone is green over a wall that has stopped working: an all-commit wall
// passes the first, the wall cdxpf shipped passes the second.
//
// THE DEFECT. cdxpf's crew arm had no root filter, and ADR 0012 D6 says the
// edge is the TREE: "docs/ and the root narrative files are the development
// record, where the crew are historical actors." Measured on the box the
// bead was filed from — the staffed PIDs matched exactly one tracked path,
// docs/adr/00NN-<seat>-pulse.md, standing on main — so adding, renaming or
// re-adding that ADR was refused with only the override as the way through,
// and the refusal cited the very rule that permits it. Worse, it was PINNED
// as intended: PIN (b)'s second case staged that shape and asserted the
// refusal, so a later reader would have read the defect as the decision.
//
// THE CASES. Two committed shapes, three refused ones, and every refused
// one is a boundary of the filter rather than a repeat of PIN (b):
//
//   - www/<name>.md — markdown, but NOT at the repo root. This is what
//     makes the `*/*` arm in crewPathSkip load-bearing: drop it and `*.md`
//     matches markdown anywhere in the tree.
//   - docsomething/<name>.go — `docs/*` is a component and not a prefix.
//   - internal/posse/<name>_probe_test.go — the shape ranger-base-o3g6a
//     found riding main, and the control the bead names.
//
// MUTATION-CHECKED (go test -overlay, tree untouched):
//   - pathSkip dropped from the crew source (the cdxpf wall): the two
//     committed cases red, the three refused ones stay green, and PIN (b)
//     does not move.
//   - the filter applied to every source instead of the crew one:
//     TestQAIdentityLiteralGuardScansAddedPaths reds on its
//     docs/runbooks/<username>.md subtest — an identity literal is refused
//     in EVERY tree, which is the "this source only" half of the claim, and
//     it is pinned there rather than restaged here.
//   - the `*/*` arm removed from the case: the www/ case alone reds.
//   - `docs/*` written as `docs*`: the docsomething/ case alone reds.
func TestQACrewArmSkipsTheTreesAppA5DoesNotReach(t *testing.T) {
	w, _ := crewWall(t, qaCrewName)
	const clean = "package posse\n\n// nothing here names anybody.\n"

	// COMMITS: the development record ADR 0012 D6 names. The ADR shape is
	// the live path the census found; the root file is the "root narrative
	// files" half of the same sentence.
	for _, rel := range []string{
		"docs/adr/0099-" + qaCrewCap + "-pulse.md",
		qaCrewName + "-handover.md",
	} {
		t.Run("commits/"+rel, func(t *testing.T) {
			if out, err := qaMsgCommit(t, w, w.pub, rel, "# the development record\n\nnothing else.\n", "-m", "the record names who wrote it", w.persona); err != nil {
				t.Fatalf(`ADR 0012 D6 puts this path OUTSIDE App.A 5 — "the edge is the tree, not the
syntax" — so the crew arm must let it stand: %v
%s`, err, out)
			}
			if l := w.log(t); l != "" {
				t.Errorf("nothing may be refused or logged for a path outside the rule:\n%s", l)
			}
		})
	}

	// STILL REFUSED: inside the tree, and the two shapes that say the
	// filter reads a path COMPONENT rather than a prefix or a suffix.
	for _, rel := range []string{
		"www/" + qaCrewName + ".md",
		"docsomething/" + qaCrewName + ".go",
		"internal/posse/" + qaCrewName + "_probe_test.go",
	} {
		t.Run("refused/"+rel, func(t *testing.T) {
			w.stage(t, w.pub, rel, clean)
			out, err := w.git(w.pub, w.persona, "commit", "-m", "a message that names nobody", "--", rel)
			if err == nil {
				t.Fatalf("this path is inside the tree App.A 5 reaches and must still be refused:\n%s", out)
			}
			for _, want := range []string{
				"refused by posse gate: a crew persona name in a staged PATH",
				"  " + rel,
				"matched: " + qaCrewName,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the refusal must carry %q:\n%s", want, out)
				}
			}
			w.unstage(t, w.pub, rel)
		})
	}
}

// shippedRoleNamesForTest is every name posse has ever shipped an example PID
// under, asserted non-empty: the exclusion in DeriveCrewLiterals is only
// meaningful if the seed actually ships roles.
func shippedRoleNamesForTest(t *testing.T) []string {
	t.Helper()
	names := retirableExampleNames(posse.Seed)
	if len(names) < 5 {
		t.Fatalf("fixture premise: posse ships example role PIDs, got %v", names)
	}
	return names
}

// crewKeys names the derived set in a failure message. keysOf (trust_test.go)
// is the same shape over map[string]any and stays there; one helper for two
// map types would be a generic nobody reads for.
func crewKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
