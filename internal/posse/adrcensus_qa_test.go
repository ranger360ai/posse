package posse

// `posse gates adr-census` (ADR 0051 D3's verify, D4's "one predicate, two
// line sources", ranger-base-gyrko): the prepare-commit-msg hook's own
// sha-stamp predicate, rendered from the same Go function, run over every
// line of every docs/adr record. These pins measure the census the way
// adrshastamp_qa_test.go measures the hook — through the rendered shell and
// real git, over the same fixture repo — and every pass cell sits beside a
// refuse cell that differs by one token or one file, so the mode is proved
// live in the fixture that proves it quiet.
//
// MUTATION-CHECKED (rig-must-be-shown-able-to-fail): the mutant table is
// quoted on ranger-base-gyrko rather than carried here.

import (
	"bytes"
	"strings"
	"testing"
)

// census runs the mode in-process over rel paths in the fixture repo, with
// the fixture's own walled env (PathOutsideGates, HOME=repo) so the git it
// drives reads the fixture's config and not the box's. refused is the
// script's own exit 1; anything else the script could not do is fatal here.
func (r *adrRepo) census(t *testing.T, files ...string) (out, errOut string, refused bool) {
	t.Helper()
	var so, se bytes.Buffer
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + r.dir}
	refused, err := runAdrCensus(r.dir, files, env, &so, &se)
	if err != nil {
		t.Fatalf("adr-census over %v: %v\nstdout:\n%s\nstderr:\n%s", files, err, so.String(), se.String())
	}
	return so.String(), se.String(), refused
}

// adrSummary is the census's last line, the one a reader keys on.
func adrSummary(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return lines[len(lines)-1]
}

// PIN 1 — ADR 0051's own table shape: stale→landed rows whose halves are
// patch-id twins, and an ordinary ancestor and a prose token beside them.
// 0 refused, and T is the number of stale shas — every row was JUDGED and
// admitted, not skipped. The second cell is the same file with the table
// taken out: the summary must then say it judged NOTHING, which is what a
// census over a pruned object store prints, and that reading is the whole
// reason the summary carries a judged count (ADR 0051 Consequences).
func TestQAAdrCensusAdmitsATwinTableAndCountsIt(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	const rel = "docs/adr/0051-like.md"
	table := "| stale | landed |\n|---|---|\n| " + r.twinStale + " | " + r.twinLanded + " |\n| " + r.twin2Stale + " | " + r.twin2Landed + " |\n"
	r.stage(t, rel, "# a record about stale shas\n\n"+table+"\nBuilt on `"+r.landed+"`; `deadbee` is prose.\n")
	out, errOut, refused := r.census(t, rel)
	if refused {
		t.Fatalf("a twin table must not be refused:\n%s%s", out, errOut)
	}
	for _, want := range []string{
		"ADMITTED " + rel + " " + r.twinStale + " twin " + r.twinLanded,
		"ADMITTED " + rel + " " + r.twin2Stale + " twin " + r.twin2Landed,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the census must name each admitted pair: want %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "REFUSE") {
		t.Errorf("nothing here is refusable:\n%s", out)
	}
	if got, want := adrSummary(out), "posse gates adr-census: base main judged 5 distinct tokens: 3 ancestors, 2 admitted by twin, 0 refused"; got != want {
		t.Errorf("summary:\n got %q\nwant %q", got, want)
	}

	// The control that the count is a measurement: no resolvable token, and
	// the summary says so instead of saying clean.
	const prose = "docs/adr/0998-prose.md"
	r.stage(t, prose, "# nothing to judge\n\n`deadbee` and `cafef00d` resolve to nothing here.\n")
	out, _, refused = r.census(t, prose)
	if refused {
		t.Fatalf("prose must not be refused:\n%s", out)
	}
	if got, want := adrSummary(out), "posse gates adr-census: base main judged 0 distinct tokens: 0 ancestors, 0 admitted by twin, 0 refused"; got != want {
		t.Errorf("a census that judged nothing must say 0 judged, never clean:\n got %q\nwant %q", got, want)
	}
}

// PIN 2 — a stale sha beside an unrelated ancestor and no twin: 1 refused,
// the REFUSE line names the file, the LINE, the sha and the remedy. The
// control is the same file with the ancestor swapped for the twin — one
// token changed, and the verdict flips.
func TestQAAdrCensusRefusesAStaleShaBesideANonTwin(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	const rel = "docs/adr/0997-stale.md"
	r.stage(t, rel, "# x\n\nStale `"+r.twinStale+"`, landed `"+r.landed+"`.\n")
	out, _, refused := r.census(t, rel)
	if !refused {
		t.Fatalf("a stale sha beside a non-twin must be refused:\n%s", out)
	}
	for _, want := range []string{
		"REFUSE " + rel + ":3 " + r.twinStale + " resolves here but is not on main",
		"no landed twin is in the record",
		"cite the bead id (git log --grep)",
		"ADR 0051 D2/D5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
	if got, want := adrSummary(out), "posse gates adr-census: base main judged 2 distinct tokens: 1 ancestors, 0 admitted by twin, 1 refused"; got != want {
		t.Errorf("summary:\n got %q\nwant %q", got, want)
	}

	// The control: the twin in place of the non-twin, and nothing else moved.
	r.stage(t, rel, "# x\n\nStale `"+r.twinStale+"`, landed `"+r.twinLanded+"`.\n")
	out, _, refused = r.census(t, rel)
	if refused {
		t.Fatalf("the same line with the twin beside it must pass:\n%s", out)
	}
	if !strings.Contains(out, "ADMITTED "+rel+" "+r.twinStale+" twin "+r.twinLanded) {
		t.Errorf("the pass must be an admission by twin, not a skip:\n%s", out)
	}
}

// PIN 3 — the radius is the RECORD: a twin in another file of the same
// census admits nothing. The control is the two files' contents in one
// file, over which the same census admits the pair.
func TestQAAdrCensusRadiusIsTheRecordNotTheCensus(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	const a, b, both = "docs/adr/0996-a.md", "docs/adr/0996-b.md", "docs/adr/0996-both.md"
	r.stage(t, a, "# a\n\nStale `"+r.twinStale+"`.\n")
	r.stage(t, b, "# b\n\nTwin `"+r.twinLanded+"`.\n")
	out, _, refused := r.census(t, a, b)
	if !refused {
		t.Fatalf("a twin in ANOTHER file must not admit the stale sha:\n%s", out)
	}
	if !strings.Contains(out, "REFUSE "+a+":3 "+r.twinStale) {
		t.Errorf("the refusal must name the stale sha in its own file:\n%s", out)
	}
	if strings.Contains(out, "ADMITTED") {
		t.Errorf("nothing was admitted here:\n%s", out)
	}
	if got, want := adrSummary(out), "posse gates adr-census: base main judged 2 distinct tokens: 1 ancestors, 0 admitted by twin, 1 refused"; got != want {
		t.Errorf("summary:\n got %q\nwant %q", got, want)
	}

	r.stage(t, both, "# both\n\nStale `"+r.twinStale+"`.\n\nTwin `"+r.twinLanded+"`.\n")
	out, _, refused = r.census(t, both)
	if refused || !strings.Contains(out, "ADMITTED "+both+" "+r.twinStale+" twin "+r.twinLanded) {
		t.Errorf("the same two lines in ONE record must be admitted:\n%s", out)
	}
}

// PIN 4 — a detached main checkout: exit 0, stderr says it judged nothing
// and why, and stdout carries no verdict and no summary — a gate that
// cannot find its base does not guess (ADR 0019's composite, 0051 D4). The
// control is the same file on the branch, refused.
func TestQAAdrCensusJudgesNothingWhenTheBaseIsDetached(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	const rel = "docs/adr/0995-detached.md"
	r.stage(t, rel, adrStamp(r.stale))
	out, _, refused := r.census(t, rel)
	if !refused || !strings.Contains(out, "REFUSE "+rel+":3 "+r.stale) {
		t.Fatalf("control: on a branch this file is refused:\n%s", out)
	}

	if out, err := r.git(nil, "checkout", "-q", "--detach", "main"); err != nil {
		t.Fatalf("git checkout --detach: %v %s", err, out)
	}
	// refs/heads/main still exists and still would not have this sha on it,
	// so a census that guessed the name would refuse here.
	if _, err := r.git(nil, "merge-base", "--is-ancestor", r.stale, "refs/heads/main"); err == nil {
		t.Fatalf("fixture: %s must still not be an ancestor of refs/heads/main", r.stale)
	}
	out, errOut, refused := r.census(t, rel)
	if refused {
		t.Fatalf("a detached main checkout must judge nothing, not refuse:\n%s%s", out, errOut)
	}
	if !strings.Contains(errOut, "judged nothing") || !strings.Contains(errOut, "detached") {
		t.Errorf("stderr must say it judged nothing and why:\n%s", errOut)
	}
	if strings.Contains(out, "ADMITTED") || strings.Contains(out, "REFUSE") || strings.Contains(out, "judged ") {
		t.Errorf("no verdict and no summary when nothing was judged:\n%s", out)
	}
}

// ONE TEXT. The census and the hook must carry adrShaPredicate byte for
// byte — a copy edited in one place is the drift the mode exists to make
// impossible, and TestQAAdrShaStampAgreesWithTheCensus would only catch it
// where a fixture happens to reach the edited line.
func TestQAAdrCensusAndTheHookRenderTheSamePredicate(t *testing.T) {
	t.Parallel()
	pred := adrShaPredicate()
	if !strings.Contains(pred, "posse_adr_judge() {") || !strings.Contains(pred, "posse_adr_judged \"$posse_adr_f\"") || !strings.Contains(pred, "posse_adr_record \"$posse_adr_f\"") {
		t.Fatalf("the predicate must read both line sources through the caller's two functions:\n%s", pred)
	}
	if !strings.Contains(AdrCensusScript(), pred) {
		t.Errorf("posse gates adr-census must render adrShaPredicate verbatim")
	}
	if !strings.Contains(CommitGuardHook(VisibilityPublic, OpsPatternSet{}), pred) {
		t.Errorf("the prepare-commit-msg hook must render adrShaPredicate verbatim")
	}
	// The two modes differ in their line sources and nowhere else.
	for _, want := range []string{"posse_adr_judged() {\n  cat \"$1\"\n}", "posse_adr_record() {\n  cat \"$1\"\n}"} {
		if !strings.Contains(AdrCensusScript(), want) {
			t.Errorf("the census reads every line of the file as both sources: want %q", want)
		}
	}
}
