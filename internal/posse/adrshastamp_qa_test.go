package posse

// The commit wall's ADR sha-stamp arm (ADR 0051 D4 as amended by D5,
// ranger-base-glewr), measured through the rendered hook and real git rather
// than read off the source string.
//
// WHAT IT IS FOR. A persona closes a build bead and writes "landed c067486
// (ranger-base-uzw11)" into the ADR that asked for it. The only sha it can
// know is its own session tree's, and the launcher rebases a third of those
// trees before it lands them — MEASURED, 48 of 134 landings — which mints a
// new sha. 12 of the 32 resolving shas in this repo's own docs/adr were
// ancestors of nothing when ADR 0051 was written. The citation of record is
// the bead id; a sha is a measurement against the base branch, or it sits
// beside its landed twin in a record whose subject IS the staleness (D5).
//
// FOUR TRAPS THESE PINS ARE SHAPED AROUND, none hypothetical:
//
//   - THE WRONG ARM PASSES THE TEST. This slot carries four walls, and three
//     of them will happily refuse a commit for reasons of their own — an
//     unqualified commit (shared-index), a class path (constitution), a
//     staged NOTES.md. Every commit below is PATH-LIMITED and every refusal
//     assertion is on THIS arm's own words, with the other three asserted
//     ABSENT. That is also why the out-of-scope vehicle in the main checkout
//     is README.md and docs/notes.d/ and not NOTES.md: NOTES.md has an arm of
//     its own there and would prove nothing about scope. NOTES.md gets its
//     cell in the worktree fixture, where that arm stands down.
//
//   - A GREEN PIN THAT MEASURED NOTHING. Passing is this arm's default: it
//     passes a token that does not resolve, a token that is an ancestor, a
//     token whose twin is in the file, a path outside docs/adr, and
//     everything at all when the base cannot be read. So no cell here asserts
//     only "exit 0" — each pass cell is a minimal EDIT of a cell that refuses
//     (same file, same line, one token or one path changed), so the arm is
//     proved live in the same fixture that proves it quiet.
//
//   - THE BASE THAT IS NOT THE BRANCH. In the writer's own worktree the sha
//     IS an ancestor of HEAD, which is exactly why ADR 0051 refused a suite
//     pin and put the check in the commit hook. The production shape is a
//     LINKED WORKTREE whose HEAD is a session branch and whose base is the
//     main checkout's branch, and TestQAAdrShaStampRefusesTheWritersOwnSha
//     is that shape end to end — a fixture on the main checkout alone would
//     be green against a hook that read HEAD instead of the common dir.
//
//   - A TWIN THAT IS NOT A TWIN. D5's admission is the only way through the
//     arm, so its edges are the arm's attack surface: an unrelated ancestor
//     beside a stale sha, a twin in a DIFFERENT staged file, and — the one
//     that is free to type once known — a stale commit with an EMPTY
//     patch-id beside an empty ancestor, which a repo's own root commit
//     always is. Each has a cell.
//
// MUTATION-CHECKED (rig-must-be-shown-able-to-fail): the mutant table is
// re-run and quoted on ranger-base-glewr rather than carried in a comment
// here, where nothing re-measures it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// adrShaRefusalMarks are the words this arm's refusal must carry: what it
// is, the rule, the resolver that survives a rebase, both remedies, and the
// promise that the hook left the tree alone.
var adrShaRefusalMarks = []string{
	"refused by posse gate: a staged docs/adr line names a sha that is not on",
	"has no landed twin in the record",
	"ADR 0051 D1/D2",
	"the citation of record for a landed change is the BEAD ID",
	"git log --grep",
	"cite the bead id",
	"put the landed twin beside it in the same file",
	"Nothing here has been reset, unstaged or cleaned up",
}

// adrOtherArmMarks are the three other walls in this slot. A cell that
// refuses with any of these measured the wrong wall.
var adrOtherArmMarks = []string{
	"This working tree's .git/index is shared",
	"a persona commit touching the constitution",
	"ops-class content",
	"a commit changing NOTES.md in the shared checkout",
}

// adrStamp is what a writer actually types: a status line with a sha and a
// bead id in it. One line, so a cell can swap the token and change nothing
// else.
func adrStamp(tok string) string {
	return "# ADR 0999 — a fixture\n\n*Status: accepted (ranger-base-glewr) · landed " + tok + "*\n"
}

// adrRepo is the fixture: a walled repo whose object store holds one of
// every shape the predicate has to tell apart.
type adrRepo struct {
	dir     string
	git     func(env []string, args ...string) (string, error)
	persona []string
	gates   string

	root        string // the repo's ROOT commit: an ancestor whose patch-id is EMPTY
	landed      string // an ordinary ancestor, non-empty patch-id, twin of nothing stale
	stale       string // a non-ancestor with no twin anywhere on the base branch
	twinStale   string // a non-ancestor whose patch-id twin IS on the base branch
	twinLanded  string // that twin
	twin2Stale  string // a second pair, so a table has two rows and a
	twin2Landed string // first-match-only bug has somewhere to show
	emptyStale  string // a non-ancestor with an EMPTY patch-id
}

// newAdrRepo builds the fixture. The twin pairs are made the way the
// launcher makes them for real: the same content change committed twice from
// the same parent tree, once on a side branch and once on the base branch.
// Nothing here is asserted from the shape — every claim the pins rest on is
// re-measured with git at the end.
func newAdrRepo(t *testing.T) *adrRepo {
	t.Helper()
	dir, git, persona := commitWallRepo(t)
	r := &adrRepo{dir: dir, git: git, persona: persona, gates: adrGatesDir(persona)}

	r.root = r.commit(t, "seed.txt", "seed\n", "fixture seed")
	r.landed = r.commit(t, "base.txt", "base\n", "fixture base")

	// A non-ancestor with no twin: content nothing on the base branch has.
	r.branch(t, "side")
	r.stale = r.commit(t, "side.txt", "side\n", "fixture side")
	r.checkout(t, "main")

	r.twinStale, r.twinLanded = r.twinPair(t, "t1", "twin one\n", "side-t1")
	r.twin2Stale, r.twin2Landed = r.twinPair(t, "t2", "twin two\n", "side-t2")

	// A non-ancestor with an EMPTY patch-id: a commit whose tree is its
	// parent's. commit-tree rather than `git commit --allow-empty`, which
	// this session's PID denies in every tree.
	tree, err := git(nil, "rev-parse", "side^{tree}")
	if err != nil {
		t.Fatalf("rev-parse side^{tree}: %v %s", err, tree)
	}
	head, err := git(nil, "rev-parse", "side")
	if err != nil {
		t.Fatalf("rev-parse side: %v %s", err, head)
	}
	empty, err := git(nil, "commit-tree", strings.TrimSpace(tree), "-p", strings.TrimSpace(head), "-m", "no diff of its own")
	if err != nil {
		t.Fatalf("commit-tree: %v %s", err, empty)
	}
	r.emptyStale = r.short(t, strings.TrimSpace(empty))

	// Every claim the cells below rest on, measured here rather than assumed
	// from how the fixture was built.
	for _, a := range []string{r.root, r.landed, r.twinLanded, r.twin2Landed} {
		if _, err := git(nil, "merge-base", "--is-ancestor", a, "refs/heads/main"); err != nil {
			t.Fatalf("fixture: %s must be an ancestor of main", a)
		}
	}
	for _, n := range []string{r.stale, r.twinStale, r.twin2Stale, r.emptyStale} {
		if _, err := git(nil, "merge-base", "--is-ancestor", n, "refs/heads/main"); err == nil {
			t.Fatalf("fixture: %s must NOT be an ancestor of main", n)
		}
	}
	if r.patchID(t, r.twinStale) != r.patchID(t, r.twinLanded) || r.patchID(t, r.twinStale) == "" {
		t.Fatalf("fixture: %s and %s must be a non-empty patch-id pair", r.twinStale, r.twinLanded)
	}
	if r.patchID(t, r.twin2Stale) != r.patchID(t, r.twin2Landed) {
		t.Fatalf("fixture: %s and %s must be a patch-id pair", r.twin2Stale, r.twin2Landed)
	}
	if r.patchID(t, r.stale) == r.patchID(t, r.landed) {
		t.Fatalf("fixture: %s must NOT be a twin of %s", r.stale, r.landed)
	}
	// The trap the empty guard exists for: a root commit and a commit with
	// no diff of its own have the SAME (empty) patch-id, so an unguarded
	// reader admits the second beside the first.
	if r.patchID(t, r.root) != "" || r.patchID(t, r.emptyStale) != "" {
		t.Fatalf("fixture: %s and %s must both have an EMPTY patch-id — that is the escape under test", r.root, r.emptyStale)
	}
	return r
}

func (r *adrRepo) short(t *testing.T, rev string) string {
	t.Helper()
	out, err := r.git(nil, "rev-parse", "--short", rev)
	if err != nil {
		t.Fatalf("rev-parse --short %s: %v %s", rev, err, out)
	}
	return strings.TrimSpace(out)
}

// patchID is the reference predicate's own pair, run out of process the way
// the hook and the census both run it — a fixture that computed this any
// other way would be measuring a different question.
func (r *adrRepo) patchID(t *testing.T, rev string) string {
	t.Helper()
	patch, err := r.git(nil, "diff-tree", "-p", rev)
	if err != nil {
		t.Fatalf("diff-tree -p %s: %v %s", rev, err, patch)
	}
	cmd := exec.Command("git", "-C", r.dir, "patch-id", "--stable")
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("patch-id: %v", err)
	}
	return strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(out)), " ", 2)[0])
}

// commit stages one path and commits it with the message given. THE MESSAGE
// IS A PARAMETER AND NOT DERIVED FROM THE PATH, and that is load-bearing: a
// commit object is content-addressed, so the twin pair below — same tree,
// same parent, same author and committer, same second — collapsed to ONE
// SHA when both halves also carried the same message, and the fixture's
// "stale" and "landed" were literally the same commit (measured; the
// ancestry assertion caught it).
func (r *adrRepo) commit(t *testing.T, rel, body, msg string) string {
	t.Helper()
	adrStage(t, r.dir, r.git, nil, rel, body)
	if out, err := r.git(nil, "commit", "-qm", msg, "--", rel); err != nil {
		t.Fatalf("fixture commit %s: %v %s", rel, err, out)
	}
	return r.short(t, "HEAD")
}

func (r *adrRepo) branch(t *testing.T, name string) {
	t.Helper()
	if out, err := r.git(nil, "checkout", "-q", "-b", name); err != nil {
		t.Fatalf("git checkout -b %s: %v %s", name, err, out)
	}
}

func (r *adrRepo) checkout(t *testing.T, name string) {
	t.Helper()
	if out, err := r.git(nil, "checkout", "-q", name); err != nil {
		t.Fatalf("git checkout %s: %v %s", name, err, out)
	}
}

// twinPair makes the launcher's own shape: one content change committed from
// the same parent tree twice, once on a side branch (the sha a persona would
// read out of its session tree) and once on main (the sha the rebase minted).
func (r *adrRepo) twinPair(t *testing.T, file, body, branch string) (stale, landed string) {
	t.Helper()
	r.branch(t, branch)
	stale = r.commit(t, file+".txt", body, "in a session tree ("+file+")")
	r.checkout(t, "main")
	landed = r.commit(t, file+".txt", body, "rebased and landed ("+file+")")
	return stale, landed
}

func (r *adrRepo) stage(t *testing.T, rel, body string) {
	t.Helper()
	adrStage(t, r.dir, r.git, r.persona, rel, body)
}

func (r *adrRepo) commitPath(t *testing.T, rel string) (string, error) {
	t.Helper()
	return r.git(r.persona, "commit", "-m", "adr", "--", rel)
}

// adrStage writes body at rel (creating parents) and stages exactly that
// path — a new file needs the add before a path-limited commit can match it
// (rangerhq-4pbt), and the add is scoped to the one path.
func adrStage(t *testing.T, repo string, git func(env []string, args ...string) (string, error), env []string, rel, body string) {
	t.Helper()
	full := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(env, "add", "--", rel); err != nil {
		t.Fatalf("git add %s: %v %s", rel, err, out)
	}
}

func adrGatesDir(persona []string) string {
	for _, kv := range persona {
		if strings.HasPrefix(kv, "RHQ_GATES_DIR=") {
			return strings.TrimPrefix(kv, "RHQ_GATES_DIR=")
		}
	}
	return ""
}

// assertAdrShaRefusal is the whole verdict in one place: refused, by THIS
// arm and not one of the other three, naming the token, and logged.
func assertAdrShaRefusal(t *testing.T, out string, err error, tok, gatesDir string) {
	t.Helper()
	if err == nil {
		t.Fatalf("a staged docs/adr line naming %s must be refused:\n%s", tok, out)
	}
	for _, wrong := range adrOtherArmMarks {
		if strings.Contains(out, wrong) {
			t.Fatalf("refused by a DIFFERENT arm (%q) — this pin is measuring the wrong wall:\n%s", wrong, out)
		}
	}
	for _, want := range append(append([]string(nil), adrShaRefusalMarks...), tok) {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
	log, _ := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if !strings.Contains(string(log), "ADR sha-stamp guard [prepare-commit-msg hook]") {
		t.Errorf("the refusal must be logged to refusals.log, got %q", string(log))
	}
}

// PINS 1, 2 and 3, one fixture and one staged line, because the whole claim
// is about which token is on it: the stale sha is refused, a landed sha
// passes, and a token that resolves to nothing here passes. Three cells that
// differ by seven characters is what makes the middle two evidence rather
// than a default.
func TestQAAdrShaStampJudgesTheTokenAndNothingElse(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	// PIN 1 — the stale sha: resolves here, ancestor of no branch, no twin.
	r.stage(t, "docs/adr/0999-fixture.md", adrStamp(r.stale))
	out, err := r.commitPath(t, "docs/adr/0999-fixture.md")
	assertAdrShaRefusal(t, out, err, r.stale, r.gates)

	// PIN 2 — the same line with a landed sha. This is the edit ADR 0051 D3
	// prescribes, and it must commit.
	r.stage(t, "docs/adr/0999-fixture.md", adrStamp(r.landed))
	if out, err := r.commitPath(t, "docs/adr/0999-fixture.md"); err != nil {
		t.Fatalf("a docs/adr line naming a LANDED sha must commit: %v\n%s", err, out)
	}

	// PIN 3 — a 7-hex token that resolves to nothing in this clone. Prose,
	// or another repo's; this hook cannot judge it and does not.
	const unresolvable = "deadbee"
	if _, err := r.git(nil, "cat-file", "-e", unresolvable+"^{commit}"); err == nil {
		t.Fatalf("fixture: %s must not resolve in this repo", unresolvable)
	}
	r.stage(t, "docs/adr/0999-fixture.md", adrStamp(unresolvable))
	if out, err := r.commitPath(t, "docs/adr/0999-fixture.md"); err != nil {
		t.Fatalf("a token that resolves to nothing here must commit: %v\n%s", err, out)
	}
}

// PINS 7, 8 and 9 — D5's admission and its edges. A stale sha is admitted
// only by a patch-id twin, only in the SAME file, and never by an empty
// patch-id.
func TestQAAdrShaStampAdmitsAStaleShaOnlyBesideItsTwin(t *testing.T) {
	t.Parallel()

	t.Run("pin 7: twin elsewhere in the same staged file passes", func(t *testing.T) {
		t.Parallel()
		r := newAdrRepo(t)
		// The twin is two paragraphs away, not on the line — the radius is
		// the record, because prose wraps a bracket onto the next line.
		body := "# ADR 0999 — a census\n\nThe stamp `" + r.twinStale + "` named a commit that never landed.\n\n" +
			"Its landed twin is `" + r.twinLanded + "`.\n"
		r.stage(t, "docs/adr/0999-census.md", body)
		if out, err := r.commitPath(t, "docs/adr/0999-census.md"); err != nil {
			t.Fatalf("a stale sha whose twin is in the same file must commit (D5): %v\n%s", err, out)
		}
	})

	t.Run("pin 7b: the twin is read from the whole staged blob, not the added lines", func(t *testing.T) {
		t.Parallel()
		r := newAdrRepo(t)
		// D5's everyday shape: the record already carries the twin and the
		// commit adds only the stale half — re-flowing one row of a
		// stale-to-landed table adds the stale sha and nothing else. A
		// reader that looked for candidate twins in the ADDED LINES would
		// see only the stale sha and refuse the record it is written for.
		const rel = "docs/adr/0999-reflow.md"
		r.stage(t, rel, "# ADR 0999 — the record\n\nThe landed commit is `"+r.twinLanded+"`.\n")
		if out, err := r.commitPath(t, rel); err != nil {
			t.Fatalf("fixture: a file naming only a landed sha must commit: %v\n%s", err, out)
		}
		r.stage(t, rel, "# ADR 0999 — the record\n\nThe landed commit is `"+r.twinLanded+"`.\n\n"+
			"It was stamped `"+r.twinStale+"` before the launcher rebased it.\n")
		// The added lines carry the stale sha and no twin; the blob carries
		// both. Measured here rather than argued, so a fixture that stopped
		// having this shape says so instead of passing quietly.
		added, err := r.git(nil, "diff", "--cached", "-U0", "HEAD", "--", rel)
		if err != nil {
			t.Fatalf("reading the staged diff: %v %s", err, added)
		}
		// The hook's own reader: lines starting with + and not +++. The raw
		// diff is not that — git's hunk header quotes the preceding context
		// line, twin included, and reading the diff whole would make this
		// fixture assert the opposite of what the hook sees.
		var plus []string
		for _, line := range strings.Split(added, "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				plus = append(plus, line)
			}
		}
		addedLines := strings.Join(plus, "\n")
		if !strings.Contains(addedLines, r.twinStale) || strings.Contains(addedLines, r.twinLanded) {
			t.Fatalf("fixture: the ADDED lines must carry %s and NOT %s:\n%s", r.twinStale, r.twinLanded, addedLines)
		}
		if out, err := r.commitPath(t, rel); err != nil {
			t.Fatalf("the twin already in the record must admit the stale sha the commit adds (D5): %v\n%s", err, out)
		}
	})

	t.Run("pin 8: an ancestor that is not the twin does not admit", func(t *testing.T) {
		t.Parallel()
		r := newAdrRepo(t)
		body := "# ADR 0999 — decoration\n\nThe stamp `" + r.stale + "` never landed.\n\n" +
			"Here is an unrelated landed commit: `" + r.landed + "`.\n"
		r.stage(t, "docs/adr/0999-decor.md", body)
		out, err := r.commitPath(t, "docs/adr/0999-decor.md")
		assertAdrShaRefusal(t, out, err, r.stale, r.gates)
	})

	t.Run("pin 8b: an EMPTY patch-id is not a twin", func(t *testing.T) {
		t.Parallel()
		r := newAdrRepo(t)
		// A repo's own root commit is an ancestor with an empty patch-id,
		// and so is any commit with no diff of its own. Two empties compare
		// equal, so an unguarded reader admits this pair.
		body := "# ADR 0999 — the empty escape\n\nStale: `" + r.emptyStale + "`.\n\n" +
			"Root, which is landed: `" + r.root + "`.\n"
		r.stage(t, "docs/adr/0999-empty.md", body)
		out, err := r.commitPath(t, "docs/adr/0999-empty.md")
		assertAdrShaRefusal(t, out, err, r.emptyStale, r.gates)
	})

	t.Run("pin 8c: the twin in another staged file does not admit", func(t *testing.T) {
		t.Parallel()
		r := newAdrRepo(t)
		r.stage(t, "docs/adr/0999-here.md", "# ADR 0999\n\nStale: `"+r.twinStale+"`.\n")
		r.stage(t, "docs/adr/0998-there.md", "# ADR 0998\n\nLanded: `"+r.twinLanded+"`.\n")
		out, err := r.git(r.persona, "commit", "-m", "adr", "--", "docs/adr/0999-here.md", "docs/adr/0998-there.md")
		assertAdrShaRefusal(t, out, err, r.twinStale, r.gates)
	})

	t.Run("pin 9: a two-row stale-to-landed table passes", func(t *testing.T) {
		t.Parallel()
		r := newAdrRepo(t)
		// ADR 0051's own shape: the stale column IS the content, and every
		// row carries its twin. Two rows, so a reader that admitted only the
		// first pair has somewhere to fail.
		body := "# ADR 0999 — the stale stamps\n\n| stale | landed |\n|---|---|\n" +
			"| " + r.twinStale + " | " + r.twinLanded + " |\n" +
			"| " + r.twin2Stale + " | " + r.twin2Landed + " |\n"
		r.stage(t, "docs/adr/0999-table.md", body)
		if out, err := r.commitPath(t, "docs/adr/0999-table.md"); err != nil {
			t.Fatalf("a stale-to-landed table must commit (D5, ADR 0051's own shape): %v\n%s", err, out)
		}
	})
}

// PIN 5 — the scope is docs/adr and only docs/adr. The SAME stale token on
// the SAME line, in three other files, commits clean. Not NOTES.md: that file
// has an arm of its own in a shared checkout and would refuse for a reason
// this pin is not about (the worktree fixture below takes that cell, where
// the NOTES.md arm stands down).
func TestQAAdrShaStampScopeIsDocsAdrOnly(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	for _, rel := range []string{"README.md", "docs/notes.d/ranger-base-glewr.md", "notes.txt"} {
		r.stage(t, rel, adrStamp(r.stale))
		if out, err := r.git(r.persona, "commit", "-m", "x", "--", rel); err != nil {
			t.Errorf("%s is outside this arm's scope and must commit: %v\n%s", rel, err, out)
		}
	}
	// And the arm is live in this very repo, on this very token: the same
	// body under docs/adr is refused. Without this cell the three passes
	// above are consistent with no arm at all.
	r.stage(t, "docs/adr/0999-fixture.md", adrStamp(r.stale))
	out, err := r.commitPath(t, "docs/adr/0999-fixture.md")
	assertAdrShaRefusal(t, out, err, r.stale, r.gates)
}

// PIN 4 — the main checkout's HEAD is detached, so there is no base branch
// to measure ancestry against. ADR 0019's composite rule and ADR 0051 D4
// both say the same thing here: a gate that cannot find its base judges
// nothing and says so. It must NOT fall back to a guessed `main` — a guess
// would be a refusal the hook cannot justify, and in a repo whose branch is
// not called main, a refusal of everything.
func TestQAAdrShaStampJudgesNothingWhenTheBaseIsDetached(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	// The control: on a branch, this exact commit is refused.
	r.stage(t, "docs/adr/0999-fixture.md", adrStamp(r.stale))
	out, err := r.commitPath(t, "docs/adr/0999-fixture.md")
	assertAdrShaRefusal(t, out, err, r.stale, r.gates)

	if out, err := r.git(nil, "checkout", "-q", "--detach", "main"); err != nil {
		t.Fatalf("git checkout --detach: %v %s", err, out)
	}
	// refs/heads/main still exists and still would not have this sha on it,
	// so a hook that guessed the name would refuse here.
	if _, err := r.git(nil, "merge-base", "--is-ancestor", r.stale, "refs/heads/main"); err == nil {
		t.Fatalf("fixture: %s must still not be an ancestor of refs/heads/main", r.stale)
	}
	r.stage(t, "docs/adr/0999-fixture.md", adrStamp(r.stale))
	out, err = r.commitPath(t, "docs/adr/0999-fixture.md")
	if err != nil {
		t.Fatalf("a detached main checkout must judge nothing, not refuse: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ADR sha-stamp check judged nothing") || !strings.Contains(out, "detached") {
		t.Errorf("the arm must say on stderr that it judged nothing and why:\n%s", out)
	}
}

// PIN 6 — the production shape, end to end: a session worktree cut from
// main, a commit made in it, and that commit's own sha pasted into an ADR.
// This is the sha a persona can actually read (`git rev-parse HEAD` in its
// own tree) and the one 36% of landings rewrite. It is an ancestor of the
// worktree's HEAD and of nothing else, so a hook that measured against HEAD
// would be green here — which is why this cell exists beside the others.
func TestQAAdrShaStampRefusesTheWritersOwnSha(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)
	wt := filepath.Join(t.TempDir(), "session")
	if out, err := r.git(nil, "worktree", "add", "-q", "-b", "sess", wt, "main"); err != nil {
		t.Skipf("git worktree add unavailable here: %v %s", err, out)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + r.dir,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	wtgit := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// The persona's own commit in its own tree, the ordinary path-limited
	// form. Nothing refuses it; this is the work.
	adrStage(t, wt, wtgit, r.persona, "work.txt", "work\n")
	if out, err := wtgit(r.persona, "commit", "-m", "work (ranger-base-glewr)", "--", "work.txt"); err != nil {
		t.Fatalf("a path-limited commit in a session worktree must land: %v\n%s", err, out)
	}
	mineOut, err := wtgit(nil, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse in the worktree: %v %s", err, mineOut)
	}
	mine := strings.TrimSpace(mineOut)
	if _, err := r.git(nil, "merge-base", "--is-ancestor", mine, "refs/heads/main"); err == nil {
		t.Fatalf("fixture: the session commit %s must not be on main yet", mine)
	}
	if _, err := wtgit(nil, "merge-base", "--is-ancestor", mine, "HEAD"); err != nil {
		t.Fatalf("fixture: %s must be an ancestor of the worktree's own HEAD — that is the trap", mine)
	}

	// And now the stamp the writer would type.
	adrStage(t, wt, wtgit, r.persona, "docs/adr/0999-fixture.md", adrStamp(mine))
	out, err := wtgit(r.persona, "commit", "-m", "adr", "--", "docs/adr/0999-fixture.md")
	assertAdrShaRefusal(t, out, err, mine, r.gates)
	if !strings.Contains(out, "refs/heads/main") {
		t.Errorf("the refusal must name the base it measured against:\n%s", out)
	}

	// PIN 5's NOTES.md cell, taken here because this is the tree where the
	// NOTES.md arm stands down: same token, outside docs/adr, commits.
	adrStage(t, wt, wtgit, r.persona, "NOTES.md", adrStamp(mine))
	if out, err := wtgit(r.persona, "commit", "-m", "notes", "--", "NOTES.md"); err != nil {
		t.Errorf("NOTES.md is outside this arm's scope and must commit here: %v\n%s", err, out)
	}
}

// PIN 10 — one predicate, two line sources (ADR 0051 D4 as amended). The
// hook judges a commit's ADDED lines; `scripts/adr-sha-census.sh` judges
// every line of a whole record. They must not disagree about what is exempt,
// or a writer passes one and fails the other — which is the second copy of
// the rule the amendment exists to prevent.
//
// Every fixture below is a WHOLE NEW FILE, so the two line sources are the
// same lines and a disagreement is a disagreement about the PREDICATE and
// nothing else. The census reads the working tree and the hook reads the
// staged blob; here they are byte-identical, which is what makes the
// comparison fair.
func TestQAAdrShaStampAgreesWithTheCensusScript(t *testing.T) {
	t.Parallel()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "adr-sha-census.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("the reference implementation must be in the tree: %v", err)
	}
	r := newAdrRepo(t)

	cases := []struct {
		name string
		body func(*adrRepo) string
		pass bool
	}{
		{"landed", func(r *adrRepo) string { return adrStamp(r.landed) }, true},
		{"unresolvable", func(r *adrRepo) string { return adrStamp("deadbee") }, true},
		{"stale-alone", func(r *adrRepo) string { return adrStamp(r.stale) }, false},
		{"stale-beside-a-non-twin", func(r *adrRepo) string {
			return "# x\n\nStale `" + r.stale + "`, landed `" + r.landed + "`.\n"
		}, false},
		{"stale-beside-its-twin", func(r *adrRepo) string {
			return "# x\n\nStale `" + r.twinStale + "`.\n\nTwin `" + r.twinLanded + "`.\n"
		}, true},
		{"empty-beside-empty", func(r *adrRepo) string {
			return "# x\n\nStale `" + r.emptyStale + "`, root `" + r.root + "`.\n"
		}, false},
		{"two-row-table", func(r *adrRepo) string {
			return "| " + r.twinStale + " | " + r.twinLanded + " |\n| " + r.twin2Stale + " | " + r.twin2Landed + " |\n"
		}, true},
	}
	for i, c := range cases {
		rel := "docs/adr/09" + string(rune('a'+i)) + "-two-way.md"
		r.stage(t, rel, c.body(r))
		_, hookErr := r.commitPath(t, rel)
		hookPassed := hookErr == nil

		cmd := exec.Command("sh", script, rel)
		cmd.Dir = r.dir
		cmd.Env = []string{"PATH=" + PathOutsideGates(""), "HOME=" + r.dir}
		censusOut, censusErr := cmd.CombinedOutput()
		censusPassed := censusErr == nil

		if hookPassed != censusPassed {
			t.Errorf("%s: the hook %s and the census %s — one predicate, two line sources (ADR 0051 D4)\ncensus said:\n%s",
				c.name, adrVerdict(hookPassed), adrVerdict(censusPassed), censusOut)
		}
		if hookPassed != c.pass {
			t.Errorf("%s: the hook %s and this fixture must be %s", c.name, adrVerdict(hookPassed), adrVerdict(c.pass))
		}
		if !hookPassed {
			// A refused file is still staged; unstage it so the next case's
			// path-limited commit carries only its own path.
			if out, err := r.git(nil, "rm", "-q", "--cached", "--", rel); err != nil {
				t.Fatalf("git rm --cached %s: %v %s", rel, err, out)
			}
		}
	}
}

func adrVerdict(pass bool) string {
	if pass {
		return "passed"
	}
	return "refused"
}

// The rendered hook must carry the scope from the Go const and nothing
// hand-spelled beside it — one place, the way MarkdownPathspecs is
// (ranger-base-4b1z4's own ask), and case-insensitively for the same
// measured reason: git pathspec matching is case-sensitive and one
// character of it walked content past this hook once already.
func TestQAAdrPathspecIsRenderedFromTheGoConst(t *testing.T) {
	t.Parallel()
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if !strings.Contains(render, "'"+AdrPathspec+"'") {
		t.Errorf("the rendered hook must carry the pathspec %q from AdrPathspec", AdrPathspec)
	}
	if strings.Contains(render, "-- 'docs/adr/*'") {
		t.Errorf("a bare case-sensitive 'docs/adr/*' pathspec is back in the hook — ranger-base-4b1z4")
	}
	// The predicate is git object identity, not a regex over the token. A
	// render that lost any of these is a wall that says yes to everything or
	// no to everything.
	for _, want := range []string{
		`git cat-file -e "$posse_adr_t^{commit}"`,
		`git merge-base --is-ancestor "$posse_adr_t" "$posse_adr_base"`,
		`posse_adr_common=$(git rev-parse --git-common-dir`,
		`git --git-dir="$posse_adr_common" symbolic-ref -q HEAD`,
		`git show ":$posse_adr_f"`,
		`git patch-id --stable`,
	} {
		if !strings.Contains(render, want) {
			t.Errorf("the rendered hook must carry %q", want)
		}
	}
	// It never guesses the base name.
	if strings.Contains(render, `posse_adr_base=refs/heads/main`) ||
		strings.Contains(render, `posse_adr_base="refs/heads/main"`) {
		t.Errorf("the arm must never fall back to a guessed base branch (ADR 0051 D4)")
	}
	// And it carries no override env. ranger-base-mlfie priced an explicit
	// marker and rejected it: free to type, and it teaches the override. D5
	// is the way through, and it is one nothing minted in a session tree can
	// take.
	for _, forbidden := range []string{"ADR_SHA_OVERRIDE", "ADR_STAMP_OVERRIDE"} {
		if strings.Contains(render, forbidden) {
			t.Errorf("this arm has no override env (ranger-base-mlfie): %q is in the render", forbidden)
		}
	}
}
