//go:build posse_arm2

package posse

// QA pins for scripts/posse-land-check's ARM DERIVATION (ADR 0087 D1/D6,
// ranger-base-pn621; filed as findings 2 and 3 of the ranger-base-3fm26
// verify, ranger-base-n2yaw).
//
// WHAT WAS UNPINNED. Nothing in the suite ran the script at all: `make test`
// never invokes it, `scripts/shell-syntax.sh` only parses it, and
// `make scripts-check`'s censuses read scripts/ as text. So the one part of
// it designed to outlive its author — "a fourth arm is covered by the commit
// that declares it, with nobody remembering this file" — was witnessed only
// by four by-hand `--plan` readings on its close. A derivation that silently
// returned the WRONG dirs (as against none, which the `[ -n "$tags" ]` guard
// catches) would be found by a missed arm at a landing and never by a red.
//
// AND IT WAS WRONG: the awk matched `//go:build` only, so an arm declared in
// the legacy `// +build` spelling — which the toolchain honours, MEASURED
// 2026-11-11 on go1.27.3 linux/amd64 — went unvetted at the one gate that
// judges the tip a replay creates. `gofmt -l` flags such a file, so
// `make fmt-check` reds it, but fmt-check is not a prerequisite of this gate
// and AGENTS.md's own ranger-base-rulbl paragraph records four commits that
// reached main not gofmt-clean straight through that gap. Arm 4 below is
// that spelling, and it red before the one-pattern fix beside it.
//
// `--plan` AND NOT A REAL RUN, which is what makes this cheap enough to sit
// in an arm rather than in a door: plan mode runs no toolchain step at all,
// so the whole pin costs one `go env` per fixture. The fixtures carry a
// deliberately BROKEN .go file, and arm 1's last assertion is the witness
// that nothing compiled it — the same fixture without `--plan` reds on
// `go build ./...`, which is arm 6.
//
// OVER A PLANTED FIXTURE AND NEVER OVER THIS TREE, deliberately. A pin that
// read this repo's own arms would assert a package list every unrelated
// landing can move, and it would be a tree-wide pin living in a package
// nobody runs whole (ranger-base-rulbl's class) — reachable only behind a
// door. The subject here is the DERIVATION, so the fixture spells its own
// file set and the script is the only thing under test. The script's copy is
// the BASE checkout's and the cwd is the tree being judged (D1), which is
// exactly the shape these runs have: an absolute path to this checkout's
// script, `cmd.Dir` the fixture.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// lcpScript resolves this checkout's land check, skipping when the checkout
// does not carry it (a tarball, a worktree pruned for a build) — the same
// shape ghostrig_qa_test.go and queuecutover_qa_test.go use, and a climb to
// ONE NAMED FILE rather than an enumeration, which is what keeps this pin out
// of the tree-wide class: the climb fence's own negative case, spelled in
// TestQAOneRepoRootHelperInTheTestPackage's fixture table as "reading one
// named file through a climb is not an enumeration".
func lcpScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../scripts/posse-land-check")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("land check not present: %v", err)
	}
	return p
}

// lcpBroken is a .go file the toolchain refuses. Every fixture holds one, so
// a plan that silently RAN its steps could not read green.
const lcpBroken = "package posse\n\nfunc broken() { this is not go }\n"

// lcpTree plants a module: go.mod, the broken file, and whatever the arm
// adds. Keys are slash-separated paths relative to the tree root.
//
// The baseline declares two arms in one package, which is this repo's own
// shape (ranger-base-qp1hm) without being this repo's file list.
func lcpTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{
		"go.mod":                         "module landcheckfixture\n\ngo 1.21\n",
		"internal/posse/broken.go":       lcpBroken,
		"internal/posse/arm2_qa_test.go": "//go:build posse_arm2\n\npackage posse\n",
		"internal/posse/arm3_qa_test.go": "//go:build posse_arm3\n\npackage posse\n",
	}
	for k, v := range files {
		all[k] = v
	}
	for rel, body := range all {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

var lcpStepLine = regexp.MustCompile(`^posse-land-check: step ([0-9]+): (.*)$`)

// lcpRun runs the check over a planted tree and returns its combined output
// and exit code. args is `--plan` for every arm but the one that proves plan
// mode ran nothing.
func lcpRun(t *testing.T, tree string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(lcpScript(t), args...)
	cmd.Dir = tree
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run the land check over %s: %v\n%s", tree, err, out)
	}
	return string(out), code
}

// lcpPlan is the planned step list: the payload of each `step N:` line, with
// the numbering checked, plus the plan's own closing line. A plan that failed
// is a fatal here rather than an empty list — an arm comparing step lists
// must not read "no steps" out of a red.
func lcpPlan(t *testing.T, tree string) []string {
	t.Helper()
	out, code := lcpRun(t, tree, "--plan")
	if code != 0 {
		t.Fatalf("the plan over %s exited %d:\n%s", tree, code, out)
	}
	var steps []string
	for _, ln := range strings.Split(out, "\n") {
		if m := lcpStepLine.FindStringSubmatch(strings.TrimRight(ln, "\r")); m != nil {
			if n, _ := strconv.Atoi(m[1]); n != len(steps)+1 {
				t.Fatalf("the plan over %s numbered a step %s where %d was due — the step counter is the only thing tying a red to the tool above it:\n%s",
					tree, m[1], len(steps)+1, out)
			}
			steps = append(steps, m[2])
		}
	}
	if want := fmt.Sprintf("plan only: %d steps, nothing run", len(steps)); !strings.Contains(out, want) {
		t.Fatalf("the plan over %s printed %d step lines and does not say %q — the count it reports and the lines it prints are the same number or one of them is lying:\n%s",
			tree, len(steps), want, out)
	}
	return steps
}

// lcpArms is the planned arm vets alone, which is what every arm below
// compares: the two leading steps and the cross-GOOS tail are fixed, and the
// derivation is the middle.
func lcpArms(steps []string) []string {
	var out []string
	for _, s := range steps {
		if strings.HasPrefix(s, "go vet -tags ") {
			out = append(out, s)
		}
	}
	return out
}

func lcpWantArms(t *testing.T, tree string, want ...string) []string {
	t.Helper()
	steps := lcpPlan(t, tree)
	if got := lcpArms(steps); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the derivation plans\n  %s\nand this tree declares\n  %s\n— the whole plan was:\n  %s",
			strings.Join(got, "\n  "), strings.Join(want, "\n  "), strings.Join(steps, "\n  "))
	}
	return steps
}

// 1. THE BASELINE, every step of it, and the contract's own shape around the
// derivation: build, vet, one vet per declared arm in the package that
// declares it, and ONE cross-GOOS vet — whichever of darwin and linux is not
// this host's, which is why that step is read as a pattern rather than as a
// literal (the GOARCH is the host's except where darwin has no port for it).
func TestQALandCheckPlansBuildVetEveryArmAndOneCrossGoosVet(t *testing.T) {
	t.Parallel()
	tree := lcpTree(t, nil)
	steps := lcpWantArms(t, tree,
		"go vet -tags posse_arm2 ./internal/posse",
		"go vet -tags posse_arm3 ./internal/posse")
	if len(steps) < 2 || steps[0] != "go build ./..." || steps[1] != "go vet ./..." {
		t.Fatalf("the plan does not open with `go build ./...` and `go vet ./...`, which is ADR 0087 D6's body and the reason this gate catches a semantic conflict at all:\n  %s",
			strings.Join(steps, "\n  "))
	}
	native := strings.TrimSpace(lcpGoEnv(t, "GOOS"))
	cross := regexp.MustCompile(`^GOOS=(darwin|linux) GOARCH=[a-z0-9]+ go vet \./\.\.\.$`)
	var crossed []string
	for _, s := range steps[2:] {
		if m := cross.FindStringSubmatch(s); m != nil {
			if m[1] == native {
				t.Errorf("the plan cross-vets %s, which is this host's own GOOS — the plain `go vet ./...` above already covered it, so the other platform's files are compiled by nothing: %s", m[1], s)
			}
			crossed = append(crossed, s)
		}
	}
	if len(crossed) != 1 {
		t.Errorf("the plan holds %d cross-GOOS vet(s) %v, want exactly one (the non-native of darwin and linux — D6's body on a linux launcher, its mirror image on a darwin one):\n  %s",
			len(crossed), crossed, strings.Join(steps, "\n  "))
	}
	// And the witness that `--plan` ran none of it: the fixture holds a .go
	// file the toolchain refuses, and the plan above read green. Arm 6 is the
	// same fixture without the flag.
	if _, err := os.Stat(filepath.Join(tree, "internal", "posse", "broken.go")); err != nil {
		t.Fatalf("the fixture's broken file is gone, so a plan that RAN its steps would have read green here too: %v", err)
	}
}

// 2. THE FOURTH ARM IN AN EXISTING PACKAGE — the property the script's header
// claims and nothing checked: a commit that declares an arm is covered by
// that commit, with nobody editing the script.
func TestQALandCheckDerivesAnArmNobodyTaughtItAbout(t *testing.T) {
	t.Parallel()
	tree := lcpTree(t, map[string]string{
		"internal/posse/arm4_qa_test.go": "//go:build posse_arm4\n\npackage posse\n",
	})
	lcpWantArms(t, tree,
		"go vet -tags posse_arm2 ./internal/posse",
		"go vet -tags posse_arm3 ./internal/posse",
		"go vet -tags posse_arm4 ./internal/posse")
}

// 3. THE SAME TAG IN ANOTHER PACKAGE — vetted in THAT package rather than in
// the one the script was written beside. The derivation carries the dir, so a
// literal `./internal/posse` here would have missed it entirely.
func TestQALandCheckVetsADeclaringPackageAndNotTheOneItWasWrittenBeside(t *testing.T) {
	t.Parallel()
	tree := lcpTree(t, map[string]string{
		"internal/newthing/arm4_qa_test.go": "//go:build posse_arm4\n\npackage newthing\n",
	})
	lcpWantArms(t, tree,
		"go vet -tags posse_arm2 ./internal/posse",
		"go vet -tags posse_arm3 ./internal/posse",
		"go vet -tags posse_arm4 ./internal/newthing")
}

// 4. THE LEGACY SPELLING, which is finding 2 of the ranger-base-3fm26 verify
// and the one arm here that RED when it was written. `// +build posse_arm5`
// with no `//go:build` line is honoured by the toolchain — MEASURED
// 2026-11-11, go1.27.3 linux/amd64: `go list` names such a file in 0 files of
// the default build and 1 file under `-tags posse_arm5` — and a derivation
// reading `//go:build` alone planned nothing for it.
//
// The negative half is in the same arm because it is the same pattern: a
// `+build` naming no tag declares nothing, and neither spelling counts BELOW
// the package clause. Arm 5 is the string-literal case those two share.
func TestQALandCheckDerivesAnArmDeclaredTheLegacyWay(t *testing.T) {
	t.Parallel()
	tree := lcpTree(t, map[string]string{
		"internal/posse/arm5_qa_test.go": "// +build posse_arm5\n\npackage posse\n",
		// Declares nothing: no tag on the line, and a legacy line below the
		// package clause is as invisible as a `//go:build` one.
		"internal/posse/bare_qa_test.go":  "// +build\n\npackage posse\n",
		"internal/posse/below_qa_test.go": "package posse\n\n// +build posse_arm8\n",
	})
	lcpWantArms(t, tree,
		"go vet -tags posse_arm2 ./internal/posse",
		"go vet -tags posse_arm3 ./internal/posse",
		"go vet -tags posse_arm5 ./internal/posse")
}

// 5. A BUILD LINE AT COLUMN 0 INSIDE A RAW STRING, below the package clause —
// a mention and not a declaration, which is the `pre` guard and the reason
// the scan reads only the preamble. This tree holds the shape in both
// spellings; a derivation that scanned whole files would plan two arms that
// do not exist, and `go vet -tags posse_arm9` over a package with no such
// file is a spurious red at a gate where a spurious red blocks a landing.
func TestQALandCheckReadsADeclarationAndNotAMentionInAStringLiteral(t *testing.T) {
	t.Parallel()
	tree := lcpTree(t, map[string]string{
		"internal/posse/mention_qa_test.go": "package posse\n\nconst mention = `\n" +
			"//go:build posse_arm9\n" +
			"// +build posse_arm9\n" +
			"`\n",
	})
	lcpWantArms(t, tree,
		"go vet -tags posse_arm2 ./internal/posse",
		"go vet -tags posse_arm3 ./internal/posse")
}

// 6. FINDING NO ARMS AT ALL IS A RED, with a real exit code and the sentence
// the header promises — not a quiet skip, which would restore the exact hole
// the arm vets exist to close. The fixture renames every `posse_arm`
// occurrence away, which is the only way to reach this guard without
// breaking the scan itself.
func TestQALandCheckRedsWhenItsOwnArmDerivationFindsNothing(t *testing.T) {
	t.Parallel()
	tree := lcpTree(t, map[string]string{
		"internal/posse/arm2_qa_test.go": "//go:build other_arm2\n\npackage posse\n",
		"internal/posse/arm3_qa_test.go": "//go:build other_arm3\n\npackage posse\n",
	})
	out, code := lcpRun(t, tree, "--plan")
	if code != 1 {
		t.Errorf("a tree whose arm derivation finds nothing exited %d, want 1 — a zero here is a plan that vets no arm and still prints green:\n%s", code, out)
	}
	for _, want := range []string{"no posse_arm* build tags found", "derivation broken"} {
		if !strings.Contains(out, want) {
			t.Errorf("the red does not say %q, so the cause landCheckSaid flattens into a merge-back bead does not name it:\n%s", want, out)
		}
	}
}

// 7. The plan really is "nothing run": the SAME fixture arm 1 planned green
// over, without the flag, reds on `go build ./...` — the broken file. So
// `--plan`'s green is the absence of execution and not a tree that happens
// to compile, and D6's "it executes nothing the branch wrote" has one
// positive witness here rather than resting on reading the script.
func TestQALandCheckPlanRunsNothingAndARealRunCompilesTheTree(t *testing.T) {
	t.Parallel()
	tree := lcpTree(t, nil)
	out, code := lcpRun(t, tree)
	if code == 0 {
		t.Fatalf("a real run over a tree holding %q read green — this fixture's whole job is to be refused by the toolchain, so arm 1's plan proves nothing:\n%s", lcpBroken, out)
	}
	if !strings.Contains(out, "step 1: go build ./...") {
		t.Errorf("the red does not name the step that died, and that line is what sits above the tool's complaint in landCheckSaid's three-line tail:\n%s", out)
	}
}

// 8. The D1 contract the derivation sits inside: cwd is the tree to judge, so
// a cwd that is not a module root is a red naming it rather than a judgement
// of the base checkout the script itself lives in.
func TestQALandCheckJudgesItsCwdAndNotItsOwnCheckout(t *testing.T) {
	t.Parallel()
	out, code := lcpRun(t, t.TempDir(), "--plan")
	if code == 0 {
		t.Errorf("the check read green with no go.mod in its cwd — the copy that runs is the BASE's and the cwd is the session tree, so a check that judged its own directory would pass every replay while looking exactly like a check (ADR 0087 D1):\n%s", out)
	}
	if !strings.Contains(out, "no go.mod here") {
		t.Errorf("the red does not say which tree it could not judge:\n%s", out)
	}
}

// lcpGoEnv is one `go env` value, for the cross-GOOS arm. It asks the
// toolchain rather than reading runtime.GOOS: the script asks `go env`, and
// the arm compares what the script printed.
func lcpGoEnv(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", key, err)
	}
	return string(out)
}
