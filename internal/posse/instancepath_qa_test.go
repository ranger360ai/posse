//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-l9ii (rule revised on ranger-base-nhvr, routing
// ruled by the operator on ranger-base-292z): THIS deployment's constitution
// checkout, written as a live path, must not appear in a tracked file of the
// public tree — except a small, explicitly dispositioned set.
//
// WHY THE PATH FORM AND NOT THE NAME. The retired seed preflight's check 4
// carried a prose arm — the private repo's NAME in public text — and the
// security reviewer retired it as a rule on nhvr: the bare name grants no
// capability (the rangerhq-yv11 ruling), and it is irrecoverably public and
// load-bearing here, because every bead marker and every commit subject
// carries it. At the sweep that was 1679 inert markers against 60 unexcused
// occurrences, so a check keyed on the name would be reading 1679 legitimate
// lines to find a handful of real ones — the wrong instrument, said in as
// many words on nhvr. What remains in force is the CONTENT classes of
// NOTES.md's "Privacy model", and a live checkout path is a member of them:
// `~/src/<the instance repo>` is this deployment's topology, not any
// deployer's.
//
// THE PATTERN, and the noise measured before it was fixed (the rangerhq-hrz
// method — narrow against this repo rather than argue). At HEAD before the
// sweep, over tracked files:
//
//	`ranger-base` anywhere            1737 hits, 1679 of them bead markers
//	`src/ranger-base`                 36 lines / 19 files
//	the three live spellings below    34 lines / 17 files
//
// The three-line difference between the last two is the whole ruling: two of
// them are fixture paths on hosts that do not exist (`/Users/x/src/…` in
// runtimewalk_qa_test.go, `/Users/t/src/…-gk6e` in visibility_test.go), which
// are not this box's topology and are not the class. So the pattern is the
// LIVE spellings and not the bare substring, and the fixtures pass by shape
// rather than by disposition.
//
// THE ABSOLUTE SPELLING IS NOT THIS PIN'S. `/Users/<you>/src/…` carries the
// box's username, so it is an ADR 0024 D2 check 3 identity literal and is
// held tree-wide by TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned
// with its own dispositioned set. Two censuses of one string would be two
// implementations to keep in sync; this one owns the `~`/`$HOME` half, that
// one owns the absolute half, and neither guesses about the other.
//
// NOR IS THE QUEUE REPO'S PATH. `~/src/<the queue repo>` is the DERIVED
// instance-path literal of check 3 — the same identity census already holds
// it, and the security reviewer already ruled it (on ranger-base-gk6e, kept
// at ranger-base-d3fn1): it is the software's own shipped, documented
// location for the shared store (ADR 0015 §4), not an operator secret.
// Censusing it again here would be a hardcoded twin of a derived pin.
//
// WHY A CENSUS AND NOT A ONE-SHOT SWEEP. The class regenerated from 58 to 60
// occurrences in a single day while nhvr was open (the ranger-base-j2io
// errata added two path lines to a runbook), so a sweep with nothing holding
// it is a measurement with a shelf life. This runs from `make ops-check`,
// which `make tree-check` reaches and `make test` depends on.
//
// THE SPELLINGS ARE ASSEMBLED, NOT WRITTEN WHOLE, and the join is
// load-bearing — do not "simplify" it back into one literal. This file is a
// tracked file of the tree its own census reads, so written whole its
// fixtures would be indistinguishable from the thing it is looking for, the
// way visibility.go's assembled plan brands already are.
// TestQAInstancePathCensusCanStillSayNo pins that.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// instanceRepoDir is the constitution repo's directory name, assembled — see
// the file header.
const instanceRepoDir = "ranger" + "-base"

// instancePathSpellings is the class: this deployment's constitution
// checkout written as a live path, in the three spellings that reach it
// without naming the box's user. The absolute spelling is check 3's
// (header); a path assembled from segments in code — `filepath.Join(home,
// "src", …)` — is not a path form in prose and is not matched, which is why
// the pattern is a literal and not a regex over the name.
//
// A func and not a package-level var, here and for the disposition below,
// so neither is a written package var that every parallel reader of it
// needs a cmd/testparallel clearance for.
func instancePathSpellings() []string {
	return []string{
		"~/src/" + instanceRepoDir,
		"$HOME/src/" + instanceRepoDir,
		"${HOME}/src/" + instanceRepoDir,
	}
}

// instancePathDisposition is the ruling, in code: a tracked file that may
// keep its live path forms, and why. Keyed by path and nothing else, so the
// list is short and readable — and every entry is a licence for the WHOLE
// file, which is why adding one is a reviewed edit and not a convenience.
// The way through a red is to write `$CONSTITUTION`, not to add a line here.
func instancePathDisposition() map[string]string {
	return map[string]string{
		// The window step's script, whose defaults ARE the live paths on
		// purpose: `CONSTITUTION=${CONSTITUTION:-…}`, overridable by flag and by
		// environment, so the operator's line has no arguments and a rehearsal
		// on a copy has all of them (the script's own usage note, and
		// ranger-base-tjfw's rehearsal ran it exactly that way). A generic
		// default here is a default that does nothing.
		"scripts/queue-cutover.sh": "the cutover script's own overridable defaults",

		// The pins for that script. ADR 0024 D4 moved the runbook OUT of this
		// tree (commit 92e67bd, triaged on ranger-base-yheoa), and reading it
		// there by path is what these pins stopped doing on ranger-base-l1vej —
		// the rollback block they RUN is now printed by the script itself
		// (`--print-rollback`) and nothing here reaches outside the checkout.
		// What is left is `qcRollbackBefore`: that block quoted VERBATIM as it
		// read at posse 43f0ec5, the control that proves the rig can see the
		// defect. Its live paths are also the substitution keys `qcRollbackRun`
		// rewrites onto the fixture, so they have to match the text the operator
		// actually had. De-instanced, the quote stops being a quote and the
		// substitution stops matching.
		"internal/posse/queuecutover_qa_test.go": "verbatim historic quote of the block the script now prints",
	}
}

// instancePathHitsIn returns the 1-based line numbers of body that carry a
// live path form. Split out from the census so the control below can feed it
// planted lines: the census reads the real tree, and a pin whose only arm is
// the tree it is standing in cannot be shown able to fail.
func instancePathHitsIn(body string) []int {
	var out []int
	for i, line := range strings.Split(body, "\n") {
		for _, s := range instancePathSpellings() {
			if strings.Contains(line, s) {
				out = append(out, i+1)
				break
			}
		}
	}
	return out
}

// instancePathHasCheckout is qibSkipUnlessCheckout's question asked without
// skipping,
// for the one arm below that must run its other rows either way. It throws
// the answer away in the same sense: it is not a second way to compute the
// root (ranger-base-sx2dq's fence), only a probe for whether one is here.
func instancePathHasCheckout(root string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	return exec.Command("git", "-C", root, "rev-parse", "--git-dir").Run() == nil
}

// The census. Every tracked file, read whole — not `git grep`, because the
// matcher above is then the ONE reader and the control below is measuring
// the same code the tree is measured by.
func TestInstancePathFormNeverAppearsInTrackedContentUndispositioned(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not `git rev-parse --show-toplevel`: any file added
	// anywhere can red this, which is the tree-wide class, and that class is
	// derived from calls to the one root helper (ranger-base-xndgk FINDING
	// 5). Spelled with git it would have no Makefile door.
	root := qibRepoRoot(t)
	qibSkipUnlessCheckout(t, root)

	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files did not run: %v", err)
	}
	paths := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	// A census over a derived set is satisfied by deriving nothing.
	if len(paths) < 100 {
		t.Fatalf("census premise: git ls-files returned %d paths, which is not this repo", len(paths))
	}

	covered := map[string]int{}
	scanned := 0
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			// A tracked path with no readable file is a checkout mid-something,
			// not a finding; skipping it silently would let a census read
			// clean over a tree it never opened, so say so.
			t.Logf("tracked but unreadable, not censused: %s (%v)", rel, err)
			continue
		}
		scanned++
		lines := instancePathHitsIn(string(body))
		if len(lines) == 0 {
			continue
		}
		if why, ok := instancePathDisposition()[rel]; ok {
			covered[rel] = len(lines)
			t.Logf("dispositioned: %s, %d line(s) — %s", rel, len(lines), why)
			continue
		}
		t.Errorf("%s carries this deployment's constitution checkout as a live path on line(s) %v — undispositioned. Write the generic form ($CONSTITUTION) instead, per NOTES.md \"Privacy model\"; a disposition in instancePathDisposition is for a file that CANNOT be generic, and is a reviewed edit.", rel, lines)
	}
	if scanned < 100 {
		t.Fatalf("census premise: only %d tracked files were readable, which is not this repo", scanned)
	}

	// The other direction, and it is the half a census cannot get by looking
	// harder: a disposition whose file no longer carries a hit is a standing
	// licence for a file nobody is watching any more, which is exactly how
	// the next one walks back in (ranger-base-r00pq took an identity entry
	// off that list for this reason rather than keeping it as a courtesy).
	for rel := range instancePathDisposition() {
		if covered[rel] == 0 {
			t.Errorf("instancePathDisposition rules %s, which no longer carries a live path form — a spent licence. Delete the entry.", rel)
		}
	}
	t.Logf("censused %d tracked files; %d dispositioned files carry hits", scanned, len(covered))
}

// The control: the matcher can still say no, the disposition can still say
// no, and this file is not quietly exempting itself.
//
// It takes the repo root for the last of those three — the file it reads is
// its own source, at the path the census walks — which also puts it in the
// tree-wide class beside the census, so both go through `make ops-check`
// rather than only the one.
func TestQAInstancePathCensusCanStillSayNo(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)

	for _, c := range []struct {
		name string
		body string
		want bool
	}{
		// The three live spellings, each on its own.
		{"the tilde spelling", "the store is in `~/src/" + instanceRepoDir + "/.beads`\n", true},
		{"the $HOME spelling", "CONSTITUTION=${CONSTITUTION:-$HOME/src/" + instanceRepoDir + "}\n", true},
		{"the braced $HOME spelling", "cd ${HOME}/src/" + instanceRepoDir + " && bd sync\n", true},
		// The two fixture hosts the pattern is narrow enough to leave alone.
		// These are the reason the pattern is not the bare `src/<name>`
		// substring; widening it to that greens nothing and reds these.
		{"a fixture path on a host that does not exist", "\"11\": \"/Users/x/src/" + instanceRepoDir + "/.beads\",\n", false},
		{"a fixture path whose last segment is a bead id", "const path = \"/Users/t/src/" + instanceRepoDir + "-gk6e\"\n", false},
		// The bare name, in its 1679-strong legitimate form. A check keyed
		// on the name reds every one of these, which is why there is not one.
		{"a bead marker in prose", "measured on " + instanceRepoDir + "-l9ii, bd 0.50.3\n", false},
		{"the prefix alone", "the db mixes the harness prefix and `" + instanceRepoDir + "-`\n", false},
		{"the repo named without a path", "cut as " + instanceRepoDir + " implementation beads from\n", false},
		// The generic form the sweep wrote, and the public repo's own path,
		// which is not private and never was.
		{"the generic form", "`--apply` from `$CONSTITUTION` → `bd sync --flush-only`\n", false},
		{"the public checkout's own path", "in ~/src/posse, the repo that carries class paths\n", false},
		// Line numbering, so a report can be acted on: the hit is reported
		// where it is, not at the top of the file.
		{"a hit below clean lines", "one\ntwo\nthree ~/src/" + instanceRepoDir + "/x\n", true},
	} {
		got := instancePathHitsIn(c.body)
		if (len(got) > 0) != c.want {
			t.Errorf("control %q: hits=%v, want any=%v", c.name, got, c.want)
		}
	}
	if got := instancePathHitsIn("one\ntwo\nthree ~/src/" + instanceRepoDir + "/x\n"); len(got) != 1 || got[0] != 3 {
		t.Errorf("the census must report the line the hit is ON, got %v, want [3]", got)
	}

	// Every disposition names a file git tracks. An entry for a path that
	// moved is a licence with no subject, and it would sit here reading like
	// a ruling while the file it was written for is censused under its new
	// name — or not at all.
	//
	// Guarded by the same checkout probe the census uses rather than by
	// `if err == nil`: a listing that did not RUN would otherwise read as a
	// listing that found everything, which is a setup refusal passing as a
	// result. Where there is no checkout the arm says so and the planted
	// rows above still ran.
	if !instancePathHasCheckout(root) {
		t.Logf("no checkout at %s — the planted rows above ran, the tracked-path arm did not", root)
		return
	}
	tracked, err := exec.Command("git", "-C", root, "ls-files", "-z", "--").Output()
	if err != nil {
		t.Fatalf("git ls-files did not run: %v", err)
	}
	have := map[string]bool{}
	for _, p := range strings.Split(strings.TrimSuffix(string(tracked), "\x00"), "\x00") {
		have[p] = true
	}
	var missing []string
	for rel := range instancePathDisposition() {
		if !have[rel] {
			missing = append(missing, rel)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("instancePathDisposition rules paths git does not track: %v", missing)
	}

	// And this file does not exempt itself by being written the way its
	// subject is. The spellings are assembled at run time (file header); a
	// later "simplification" back to one literal would make this file's own
	// source a hit, and the only ways out of that are a self-disposition or
	// a narrower pattern — both of them holes.
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	body, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range instancePathSpellings() {
		if strings.Contains(string(body), s) {
			t.Errorf("this file carries %q verbatim — assemble it (see the header), or the census's own source is a hit in the census", s)
		}
	}
}
