package posse

// QA pin for ranger-base-2yaud — the parallel gate's fakeDirs exemption is
// argued in prose, and the prose has to name the key the code actually uses.
//
// cmd/testparallel waives three package-level vars out of its taint filter.
// fakeDirs is the expensive one: left in, it taints 663 of the tests the
// parallel work exists to free. The waiver is safe only because the map is
// partitioned per test, so the comment carrying that argument IS the safety
// case a reader checks the exemption against.
//
// MEASURED, and this is why the pin exists rather than a one-time edit: the
// comment said the key was `t.Name()` for as long as the key was NOT t.Name().
// internal/posse/herdr_test.go moved it to the *testing.T itself in dfb55b3 —
// the same commit that wrote the second copy of the stale sentence — because
// `go test -count=N` gives N live copies of one test the same name and
// t.Parallel resumes them together, measured as five FAILs at -count=3 that
// were green at -count=1 (ranger-base-pj87l). So the exemption was justified,
// in writing, by the exact key that had just been measured to fail. The waiver
// was still SOUND (a pointer is strictly stronger than a name); only the
// argument was wrong, which is the failure mode a reader cannot catch by
// reading, because reading is what it defeats.
//
// The pin is two-way on purpose: it derives the key from the code and requires
// the prose to name THAT key, so it stays correct if the key ever moves back
// rather than pinning today's answer as a literal. Each arm is paired with a
// control over the real text mutated at one site, which must come out the
// other way — a checker that cannot refuse pins nothing.

import (
	"os"
	"strings"
	"testing"
)

const (
	fakeDirsKeyT       = "*testing.T"
	fakeDirsKeyName    = "t.Name()"
	fakeDirsKeyUnknown = "unknown"
)

// fakeDirsActualKey answers what internal/posse/herdr_test.go actually keys
// the map on, by reading the three call sites rather than the comment above
// them. Store/Delete/Load must agree or the answer is unknown: a split key is
// not a partition and must not read as either one.
func fakeDirsActualKey(src string) string {
	calls := []string{"fakeDirs.Store(", "fakeDirs.Delete(", "fakeDirs.Load("}
	seen := map[string]bool{}
	for _, call := range calls {
		i := strings.Index(src, call)
		if i < 0 {
			return fakeDirsKeyUnknown
		}
		switch strings.TrimSpace(firstCallArg(src[i+len(call):])) {
		case "t":
			seen[fakeDirsKeyT] = true
		case "t.Name()":
			seen[fakeDirsKeyName] = true
		default:
			return fakeDirsKeyUnknown
		}
	}
	if len(seen) != 1 {
		return fakeDirsKeyUnknown
	}
	for k := range seen {
		return k
	}
	return fakeDirsKeyUnknown
}

// firstCallArg answers the first argument of a call, given everything after
// its open paren. It counts parens so that `t.Name()` comes back whole —
// stopping at the first `)` reads it as `t.Name(`, which is the one way this
// extractor could answer "unknown" for a key that is perfectly well defined.
func firstCallArg(rest string) string {
	depth := 0
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return rest[:i]
			}
			depth--
		case ',':
			if depth == 0 {
				return rest[:i]
			}
		}
	}
	return rest
}

// fakeDirsExemptionProse is the comment block in cmd/testparallel/main.go that
// argues the waiver — from the sentence that opens it to the one that closes
// it. Everything a reader would check the exemption against is inside.
func fakeDirsExemptionProse(src string) string {
	start := strings.Index(src, "// The one exemption")
	if start < 0 {
		return ""
	}
	end := strings.Index(src[start:], "waived silently.")
	if end < 0 {
		return ""
	}
	return src[start : start+end+len("waived silently.")]
}

// fakeDirsProseNamesKey answers which key a stretch of prose CLAIMS, so the
// claim can be compared with the code. A stretch naming both, or neither, is
// not an argument a reader can check.
func fakeDirsProseNamesKey(prose string) string {
	// "the key is the T and NOT t.Name()" names one key and denies the
	// other; a bare mention of t.Name() is not a claim that it IS the key.
	claimsName := strings.Contains(prose, "Store(t.Name()") ||
		strings.Contains(prose, "Load(t.Name())") ||
		strings.Contains(prose, "partitioned by the\n\t// test's own name") ||
		strings.Contains(prose, "IS partitioned by t.Name()")
	claimsT := strings.Contains(prose, "*testing.T")
	switch {
	case claimsT && !claimsName:
		return fakeDirsKeyT
	case claimsName && !claimsT:
		return fakeDirsKeyName
	default:
		return fakeDirsKeyUnknown
	}
}

func fakeDirsSources(t *testing.T) (harness, gate string) {
	t.Helper()
	h, err := os.ReadFile("internal/posse/herdr_test.go")
	if err != nil {
		t.Fatalf("read herdr_test.go: %v", err)
	}
	g, err := os.ReadFile("cmd/testparallel/main.go")
	if err != nil {
		t.Fatalf("read cmd/testparallel/main.go: %v", err)
	}
	return string(h), string(g)
}

// Arm 1. The prose that justifies waiving 663 tests names the key the code
// uses. Two-way: derived from the code, not pinned as a literal.
func TestQAParallelGateExemptionProseNamesTheRealKey(t *testing.T) {
	harness, gate := fakeDirsSources(t)

	actual := fakeDirsActualKey(harness)
	if actual == fakeDirsKeyUnknown {
		t.Fatalf("cannot read the fakeDirs key off its call sites in internal/posse/herdr_test.go; the extractor stopped matching and this pin is measuring nothing")
	}

	prose := fakeDirsExemptionProse(gate)
	if prose == "" {
		t.Fatalf("the fakeDirs exemption comment is not in cmd/testparallel/main.go; either the waiver went silent or the extractor stopped matching")
	}
	if claimed := fakeDirsProseNamesKey(prose); claimed != actual {
		t.Errorf("cmd/testparallel/main.go argues the fakeDirs exemption by the %s key; internal/posse/herdr_test.go keys the map on %s.\nThe waiver may still be sound, but the argument a reader checks it against is wrong (ranger-base-2yaud, ranger-base-pj87l).", claimed, actual)
	}
}

// Arm 2. The second copy of the argument, at filter 2b, is the one a reader
// reaches from the fakeDirOf side; it went stale in the same commit as the
// first and must not name a key the code does not use.
func TestQAParallelGateSecondExemptionNoteIsNotStale(t *testing.T) {
	harness, gate := fakeDirsSources(t)

	actual := fakeDirsActualKey(harness)
	if actual == fakeDirsKeyUnknown {
		t.Fatalf("cannot read the fakeDirs key off its call sites; this pin is measuring nothing")
	}
	i := strings.Index(gate, "This is not the fakeDirs exemption")
	if i < 0 {
		t.Fatalf("the filter 2b note that distinguishes fakeDir() from the fakeDirs exemption is gone from cmd/testparallel/main.go")
	}
	note := gate[i:]
	if j := strings.Index(note, "MEASURED"); j > 0 {
		note = note[:j]
	}
	if actual != fakeDirsKeyName && strings.Contains(note, "partitioned by t.Name()") {
		t.Errorf("the filter 2b note still says the map is partitioned by t.Name(); it is partitioned by %s (ranger-base-2yaud)", actual)
	}
}

// Arm 3, the controls. Each checker is run over the real text with one site
// mutated, and must come out the other way. Without these, arms 1 and 2 are
// green over a checker that answers the same thing for every input.
func TestQAParallelGateExemptionCheckCanFail(t *testing.T) {
	harness, gate := fakeDirsSources(t)

	// Control A: the key moves back to t.Name() in the harness and the gate's
	// (now correct-again) prose must be read as agreeing, not as stale. This
	// is the arm that proves the pin is two-way rather than a literal.
	backToName := strings.NewReplacer(
		"fakeDirs.Store(t, dir)", "fakeDirs.Store(t.Name(), dir)",
		"fakeDirs.Delete(t)", "fakeDirs.Delete(t.Name())",
		"fakeDirs.Load(t)", "fakeDirs.Load(t.Name())",
	).Replace(harness)
	if got := fakeDirsActualKey(backToName); got != fakeDirsKeyName {
		t.Fatalf("with every call site keyed on t.Name() the extractor answered %q; it is not reading the call sites", got)
	}
	if got := fakeDirsActualKey(harness); got != fakeDirsKeyT {
		t.Fatalf("the real harness answered %q; the extractor separates nothing", got)
	}

	// Control B: a split key — Store on the T, Load on the name — is not a
	// partition, and must not read as either key.
	split := strings.Replace(harness, "fakeDirs.Load(t)", "fakeDirs.Load(t.Name())", 1)
	if got := fakeDirsActualKey(split); got != fakeDirsKeyUnknown {
		t.Fatalf("a Store/Load key mismatch answered %q; the extractor reads one call site and calls it the key", got)
	}

	// Control C: the exact stale prose this bead removed must be refused
	// against the real code. This is the defect as it shipped, verbatim in
	// shape: the sentence that said the key was the test's own name.
	stale := strings.Replace(
		gate,
		"a key space partitioned by the *testing.T POINTER over a",
		"a key space partitioned by the\n\t// test's own name over a",
		1)
	if stale == gate {
		t.Fatalf("the exemption sentence this control mutates is not in cmd/testparallel/main.go in the shape the control expects; the control is not exercising the real text")
	}
	if got := fakeDirsProseNamesKey(fakeDirsExemptionProse(stale)); got == fakeDirsKeyT {
		t.Fatalf("prose saying the partition is the test's own NAME was read as claiming %s; the check cannot see the defect it exists for", got)
	}
	if got := fakeDirsProseNamesKey(fakeDirsExemptionProse(gate)); got != fakeDirsKeyT {
		t.Fatalf("the real exemption prose was read as claiming %q, not %s; the check refuses everything and separates nothing", got, fakeDirsKeyT)
	}

	// Control D: a deleted exemption block reads as absent, not as agreeing.
	if got := fakeDirsExemptionProse("package main\n\nfunc main() {}\n"); got != "" {
		t.Fatalf("prose was extracted from a file that has none: %q", got)
	}
}
