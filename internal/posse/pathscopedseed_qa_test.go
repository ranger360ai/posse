package posse

// ranger-base-ccd: ADR 0014's grammar, applied to the example PIDs.
//
// ADR 0014 §1 gave path-scoped writes two shapes and a Consequences bullet
// saying the shipped skeletons would carry one each once the renderers
// landed. They land in ranger-base-nuu (L2) and ranger-base-yu5 (L4); this
// is the seed side, and this is its pin.
//
// Four things could go wrong and only one of them is about YAML:
//
//  1. A PID loses the rule, or gains one it should not have. Reviewer and
//     security are already the bare wall and must stay path-scoped-free;
//     qa must NOT be the reviewer shape (`harden-suite` commits tests), and
//     is the one row where "stricter" would be wrong.
//  2. A PID keeps the rule and drops `cage:`. At `shims` a path-scoped
//     write is not a tool-name deny, nothing realizes it, and the PID reads
//     as a wall while being prose. That is the exact failure ADR 0014
//     exists to prevent, so it is asserted through ResolveCage — the
//     function the launcher itself asks.
//  3. The rule survives, parses, and names nothing the wall can express.
//     `Edit(docs/adr/**/*.md)` is a legal-looking rule that no tier
//     realizes. So the last arm goes through pidDeniedSubtrees /
//     SeatbeltWritable — production's own renderers — and asks what the
//     PROFILE would say, not what the frontmatter says.
//
//  4. The rule lands and the persona is never told. ADR 0001: a guardrail
//     that can be a tool rule appears TWICE, prose in the body and a rule
//     in `deny:` — the rule refuses, the prose explains, and a refusal a
//     persona cannot account for is the one it works around. So the body
//     of every PID carrying the wall has to name the subtree.
//
// The table is exhaustive over the seed and a name missing from it is a
// failure: a corpus pin that skips the file it never heard of says "clean"
// about a corpus it did not read (ranger-base-h6fx). Every field is
// compared exactly, per PID, so deleting one rule from one file names that
// file and that field.

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// seedWriteShape is what one seeded PID must say about file writes.
type seedWriteShape struct {
	Bare     []string // Edit/Write/NotebookEdit denied over the whole tree
	Scoped   []string // path-scoped rules, in the order the PID writes them
	Writable []string // writable: extras
	Cage     string   // what ResolveCage("", ag) answers — the launch tier
}

// seedWriteShapes is ADR 0014's Consequences bullet as a table.
var seedWriteShapes = map[string]seedWriteShape{
	// The allow-list shape: the repo is not writable except docs/adr.
	// "You write ADRs, not the code the ADR constrains."
	"architect": {Bare: []string{"Edit", "Write"}, Writable: []string{"docs/adr"}, Cage: CageSeatbelt},
	// The deny-list shape: the repo is writable except docs/adr.
	// "You do not edit the ADR that constrains you."
	"developer": {Scoped: []string{"Edit(docs/adr/**)", "Write(docs/adr/**)"}, Cage: CageSeatbelt},
	// Same shape, and deliberately NOT the reviewer's: qa is mixed-intent
	// (`harden-suite` writes tests), so the bare wall would be wrong here.
	"qa": {Scoped: []string{"Edit(docs/adr/**)", "Write(docs/adr/**)"}, Cage: CageSeatbelt},
	// Already stricter. Nothing path-scoped goes on these two — a scoped
	// rule beside the bare one is redundant, and `posse agent check` warns.
	"reviewer": {Bare: []string{"Edit", "Write"}, Cage: CageSeatbelt},
	"security": {Bare: []string{"Edit", "Write"}, Cage: CageSeatbelt},
	// Not a code lane at all: business-manager reads and reports, so it
	// carries the same bare wall the two review lanes do, and nothing
	// path-scoped for the same reason they do not.
	"business-manager": {Bare: []string{"Edit", "Write"}, Cage: CageSeatbelt},
	// The rest write files and are walled by nothing here; they must stay
	// at the default tier, because a `cage:` they do not need is a cost
	// (and, on codex, an incompatibility) nobody asked for.
	"devops":  {Cage: CageShims},
	"ops":     {Cage: CageShims},
	"product": {Cage: CageShims},
}

// TestSeededPIDsCarryTheADR0014Shapes is the corpus pin over the embed —
// posse.Seed is what a release binary ships and `posse init` copies, which
// is the surface ADR 0014's Consequences bullet is about.
func TestSeededPIDsCarryTheADR0014Shapes(t *testing.T) {
	names := exampleAgentNames(posse.Seed)
	if len(names) < 9 {
		t.Fatalf("the seed ships %d example PIDs (%v) — a corpus pin over a corpus this small is measuring nothing", len(names), names)
	}
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents")}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, n := range names {
		rel := path.Join("agents", n+".md")
		b, err := fs.ReadFile(posse.Seed, rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		ag, err := a.LoadAgent(n)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		want, ok := seedWriteShapes[n]
		if !ok {
			t.Errorf("examples/%s ships and this table has never heard of it — say what it does about file writes (ADR 0014 §1: allow-list, deny-list, or nothing) before it seeds an instance", rel)
			continue
		}
		checked++

		var bare []string
		whole := wholeTreeWriteDeny(ag.Deny)
		for _, tool := range []string{"Edit", "Write", "NotebookEdit"} {
			if whole[tool] {
				bare = append(bare, tool)
			}
		}
		if !reflect.DeepEqual(bare, nilIfEmpty(want.Bare)) {
			t.Errorf("examples/%s denies the whole tree for %v, table says %v — deny: %v", rel, bare, want.Bare, ag.Deny)
		}
		var scoped []string
		for _, d := range pathScopedWrites(ag.Deny) {
			if d.Bare {
				t.Errorf("examples/%s: %s is the bare rule written the long way — write %s (ADR 0014 §1)", rel, d.Rule, d.Tool)
				continue
			}
			if !d.Subtree {
				t.Errorf("examples/%s: %s is not a directory-prefix glob — no tier realizes it, so it is a wall in prose only (ADR 0014 §1)", rel, d.Rule)
				continue
			}
			scoped = append(scoped, d.Rule)
		}
		if !reflect.DeepEqual(scoped, nilIfEmpty(want.Scoped)) {
			t.Errorf("examples/%s path-scoped writes are %v, table says %v — deny: %v", rel, scoped, want.Scoped, ag.Deny)
		}
		if !reflect.DeepEqual(nilIfEmpty(ag.Writable), nilIfEmpty(want.Writable)) {
			t.Errorf("examples/%s writable: is %v, table says %v", rel, ag.Writable, want.Writable)
		}
		// The one that turns a rule into a wall. A scoped deny at `shims`
		// is unrealized on every runtime, and the same is true of the bare
		// pair on claude and grok.
		if got := ResolveCage("", ag); got != want.Cage {
			t.Errorf("examples/%s launches at cage %s, table says %s — a file-write deny below seatbelt is politeness (ADR 0014 §2)", rel, got, want.Cage)
		}
		if (len(want.Scoped) > 0 || len(want.Bare) > 0) && ResolveCage("", ag) == CageShims {
			t.Errorf("examples/%s denies file writes and launches at shims — nothing realizes that", rel)
		}
		// ADR 0001's "twice": the deny is the rule, the body is the reason.
		// Scoped only — the two bare-wall review lanes say "read-only by
		// construction" and name no path, which is the whole truth there.
		if len(want.Scoped) > 0 || len(want.Writable) > 0 {
			if !strings.Contains(ag.Body, "docs/adr") {
				t.Errorf("examples/%s walls docs/adr and its body never says so — ADR 0001: the rule refuses and the prose explains, and a persona that cannot account for a refusal works around it", rel)
			}
		}
	}
	if checked != len(names) {
		t.Errorf("only %d of %d seeded PIDs were checked", checked, len(names))
	}
}

// TestSeededPIDShapesReachTheRenderedProfile asks the two L2 renderers what
// they would actually emit for a session in a repo, rather than trusting
// that a well-formed rule reaches the wall. It is the arm that would catch
// a rule that parses, passes the table, and names a path the profile never
// mentions.
func TestSeededPIDShapesReachTheRenderedProfile(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents")}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	adr := filepath.Join(repo, "docs", "adr")
	if err := os.MkdirAll(adr, 0o755); err != nil {
		t.Fatal(err)
	}
	load := func(t *testing.T, n string) *AgentFile {
		t.Helper()
		b, err := fs.ReadFile(posse.Seed, path.Join("agents", n+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		ag, err := a.LoadAgent(n)
		if err != nil {
			t.Fatal(err)
		}
		return ag
	}
	wantAdr := absResolve(adr)

	// Deny-list: docs/adr is in the trailing deny block, and the repo is
	// still granted. The negative control is the SIBLING — `docs` must stay
	// writable, or what is being pinned is a deny of the parent.
	for _, n := range []string{"developer", "qa"} {
		ag := load(t, n)
		// One entry per RULE, so the pair Edit(…)+Write(…) resolves twice
		// to one directory; SeatbeltCarveOut dedupes them into the profile's
		// single subpath. Both halves matter: two rules must be parsed, and
		// one directory must come out.
		den := pidDeniedSubtrees(ag, repo)
		if len(den) != 2 {
			t.Errorf("%s: %d rules reached the renderer, want the Edit/Write pair", n, len(den))
		}
		var got []string
		for _, d := range den {
			got = append(got, d.Path)
		}
		if !reflect.DeepEqual(dedupeStrings(got), []string{wantAdr}) {
			t.Errorf("%s: the profile's trailing deny would carry %v, want [%s]", n, got, wantAdr)
		}
		w := a.SeatbeltWritable(ag, repo, filepath.Join(home, "gates"))
		if !containsPath(w, absResolve(repo)) {
			t.Errorf("%s: the deny-list shape must leave the repo writable; grants were %v", n, w)
		}
		if containsPath(w, wantAdr) {
			t.Errorf("%s: docs/adr appears as a GRANT — the deny below it is what holds, but a grant here means the PID was read as the allow-list shape", n)
		}
	}

	// Allow-list: nothing path-scoped to deny, the repo is not granted, and
	// docs/adr is. `.beads` is the carve-out that keeps claim/comment/close
	// working — without it the wall would buy the file gate with the record.
	ag := load(t, "architect")
	if d := pidDeniedSubtrees(ag, repo); len(d) != 0 {
		t.Errorf("architect: the allow-list shape has no path-scoped denies to render, got %v", d)
	}
	w := a.SeatbeltWritable(ag, repo, filepath.Join(home, "gates"))
	if containsPath(w, absResolve(repo)) {
		t.Errorf("architect: the repo is granted whole — bare Edit/Write must take it away; grants were %v", w)
	}
	if !containsPath(w, wantAdr) {
		t.Errorf("architect: writable: [docs/adr] did not reach the profile; grants were %v", w)
	}
	if !containsPath(w, absResolve(filepath.Join(repo, ".beads"))) {
		t.Errorf("architect: the .beads carve-out is missing — the wall would cost the record stage; grants were %v", w)
	}
	if containsPath(w, absResolve(filepath.Join(repo, "docs"))) {
		t.Errorf("architect: docs/ is granted whole — the extra is the ADR directory, not its parent; grants were %v", w)
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
