//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-b22vq. ADR 0039 D2 (ranger-base-ight8) added `runtimes` to
// PromotedPaths and swept the docs; what it could not sweep is the shipped
// OUTPUT that enumerates the promoted set by hand. Four sentences did, and
// all four went quietly false in the same commit — nothing in the suite
// asserted any of them, which is why the sweep and the suite both stayed
// green:
//
//   - the commit wall's own refusal (gates.go), the sharpest, because it is
//     reachable and self-contradicting: it refused `rhq/runtimes/claude.yaml`,
//     printed the class line naming `rhq/runtimes`, and explained itself one
//     line earlier with a list that omitted it;
//   - the promote verb's help (cmd/posse/main.go), where an operator reads
//     what promote copies;
//   - the init stamp (init.go), the sentence that tells a new operator what
//     the launch verify covers;
//   - the seatbelt all-clear (seatbelt.go), read on a clean live home.
//
// All four now render PromotedProse, so the next member of the set widens
// them in the same edit. These pins are what stops the NEXT one going stale:
// each reads the shipped text BACK and measures it against a written-out
// spec.
//
// THE SPEC IS WRITTEN OUT, for the reason constitutionClassSpec is
// (ranger-base-ak3e): cases generated from PromotedPaths shrink with it.
// Drop `runtimes` from the set and a derived case list deletes its own
// subtest and passes; the spec below reds instead, so widening the promoted
// set stays a deliberate two-line edit.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// promotedProseSpec is the promoted set as an operator-facing sentence
// spells it — the HOME spelling, trees with a trailing slash. Its repo-side
// twin is constitutionClassSpec.
var promotedProseSpec = []string{
	"agents/",
	"config.yaml",
	"recipes/",
	"runtimes/", // ADR 0039 D2, ranger-base-ight8
	"skills/",
}

// The join between the written set and the rendered one, failing in both
// directions: a member dropped is every sentence below narrowing, a member
// added is a widening nobody wrote down.
func TestQAPromotedProseIsTheSpecifiedSet(t *testing.T) {
	t.Parallel()
	if got, want := PromotedProse(""), strings.Join(promotedProseSpec, ", "); got != want {
		t.Errorf("PromotedProse(\"\") = %q, ranger-base-b22vq specifies %q\n"+
			"— if the promoted set legitimately grew, widen promotedProseSpec in the same edit", got, want)
	}
	// The conjunction form is the same list with one word moved, so it is
	// pinned as such rather than as a second literal: a bug that dropped a
	// member from one form and not the other would otherwise hide here.
	and := PromotedProse("and")
	for _, m := range promotedProseSpec {
		if !strings.Contains(and, m) {
			t.Errorf("PromotedProse(\"and\") = %q does not name %q", and, m)
		}
	}
	if n := strings.Count(and, ","); n != len(promotedProseSpec)-2 {
		t.Errorf("PromotedProse(\"and\") = %q has %d commas, want %d — the last member takes the conjunction, not a comma",
			and, n, len(promotedProseSpec)-2)
	}
	if !strings.HasSuffix(and, " and "+promotedProseSpec[len(promotedProseSpec)-1]) {
		t.Errorf("PromotedProse(\"and\") = %q does not end in a conjunction", and)
	}
	// `config.yaml` is a file and the rest are trees, and the split is taken
	// from the path rather than from a second list. A member that stopped
	// being distinguished would read as `config.yaml/` in four sentences.
	if strings.Contains(PromotedProse(""), "config.yaml/") {
		t.Error("PromotedProse renders config.yaml as a tree — it is the one member with an extension")
	}
}

// SITE 1, the bead's own repro, run: a persona commit staging
// `<ConstitutionSourceDir>/runtimes/claude.yaml` in the constitution repo is
// refused, and the refusal's REASON names every promoted path — including
// the one it just refused. The class line and the reason line disagreeing in
// the same refusal is the defect; this reads the reason span and measures
// it.
func TestQAConstitutionWallRefusalReasonNamesTheWholePromotedSet(t *testing.T) {
	t.Parallel()
	repo, git, persona := constitutionWallRepo(t, true)
	rel := ConstitutionSourceDir + "/runtimes/claude.yaml"
	stageAt(t, repo, git, persona, rel, "model_strong: claude-fable-5-1\n")
	out, err := git(persona, "commit", "-m", "widen the overlay", "--", rel)
	assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))

	// The REASON span, not the whole refusal: the class line below it prints
	// the staged path verbatim, so a whole-message Contains would be
	// satisfied by the very line the reason contradicts.
	const from, to = "ADR 0015 §2/§3:", "staged now, and in the class:"
	i := strings.Index(out, from)
	j := strings.Index(out, to)
	if i < 0 || j <= i {
		t.Fatalf("the refusal has no reason span between %q and %q — this pin is reading the wrong text:\n%s", from, to, out)
	}
	reason := out[i:j]
	for _, m := range promotedProseSpec {
		if !strings.Contains(reason, m) {
			t.Errorf("the wall refused %s and explained itself without naming %q:\n%s", rel, m, reason)
		}
	}
	// The two non-promoted halves of the class are spoken for in the same
	// sentence, and they are not in PromotedPaths — so they are named here
	// rather than derived.
	for _, want := range []string{"env files", "settings file"} {
		if !strings.Contains(reason, want) {
			t.Errorf("the refusal's reason no longer accounts for the %s half of the class:\n%s", want, reason)
		}
	}
}

// SITE 3: the init stamp. A fresh home's stamp line is the sentence that
// tells a new operator what the launch verify covers, and the launch verify
// covers the whole promoted set.
func TestQAInitStampNamesTheWholePromotedSet(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var b strings.Builder
	if err := a.initFrom(&b, os.DirFS(foreignSeedDir(t)), "stale"); err != nil {
		t.Fatalf("the seed: %v\n%s", err, b.String())
	}
	var stamp string
	for _, line := range strings.Split(b.String(), "\n") {
		if strings.HasPrefix(line, "stamped ") && strings.Contains(line, "(seeded)") {
			stamp = line
			break
		}
	}
	if stamp == "" {
		t.Fatalf("a fresh init printed no `stamped ... (seeded)` line — this pin has nothing to judge:\n%s", b.String())
	}
	if !strings.Contains(stamp, "every launch now hashes") {
		t.Fatalf("the stamp line no longer claims to say what the verify hashes:\n  %s", stamp)
	}
	for _, m := range promotedProseSpec {
		if !strings.Contains(stamp, m) {
			t.Errorf("init's stamp tells a new operator the launch verify covers a set without %q:\n  %s", m, stamp)
		}
	}
}

// SITE 4: the gates all-clear on a clean home. The bead could not reach this
// branch from a scratch home with the built binary — every scratch home sits
// under a granted temp tree, so the bad-grant branch answers instead, and
// THAT branch is correct (it names the reaching grants by path). What went
// stale is the summary an operator sees when nothing reaches. In-package the
// branch is reachable, so it is measured rather than left to the one shape
// that could not be produced.
func TestQAGatesAllClearNamesTheWholePromotedSet(t *testing.T) {
	root := sbRoot(t)
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	cwd := sbMkdir(t, filepath.Join(root, "work"))
	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(a.PersonasDir(), "developer")}

	var b strings.Builder
	if err := a.SeatbeltReport(ag, cwd, &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	var line string
	for _, ln := range strings.Split(got, "\n") {
		if strings.Contains(ln, "in no grant above") {
			line = ln
			break
		}
	}
	if line == "" {
		t.Fatalf("this home did not reach the all-clear branch — the pin is measuring nothing:\n%s", got)
	}
	for _, m := range promotedProseSpec {
		if !strings.Contains(line, m) {
			t.Errorf("the gates all-clear does not name %q among the constitution it declares ungranted:\n  %s", m, line)
		}
	}
	// envs/ and the manifest are constitution and not promoted; the line
	// carries them on its own terms.
	for _, want := range []string{ConstitutionEnvsDir + "/", PromoteManifestFile} {
		if !strings.Contains(line, want) {
			t.Errorf("the gates all-clear no longer names %q:\n  %s", want, line)
		}
	}
}
