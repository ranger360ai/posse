package posse

// ADR 0042 D3, and ranger-base-f5fkk: the rendered CredBin shim's refusal
// EXIT CODE is load-bearing, and until now nothing measured it as such.
//
// WHAT THE CODE DECIDES. ADR 0019 D2 read the darwin 2.1.258 release binary's
// credential composite (`keychain-with-plaintext-fallback`) and wrote down its
// rules. The read tries the keychain first — `security`'s find verb on the
// runtime's item, the same read keychainCmd makes — and falls through to the
// plaintext credentials file under the runtime's config dir ONLY when that
// answered NULL: exit 0 with no output, exit 36 (user interaction not
// allowed), exit 44 (item not found). Any other exit is a read failure and
// the strict read does not fall through.
//
// Every crew PID denies the binary the runtime authenticates with (ADR 0042
// D1), so the runtime's own read resolves to this persona's refusal shim.
// Which side of that line the shim lands on is therefore a design decision,
// and D3 takes it: the shim exits a READ FAILURE. A null exit would send the
// runtime to the fallback file — a stale S2 file on a healthy box, the
// 2026-08-24 misdiagnosis class with a new sentence — and at refresh time the
// composite's update rule turns the refused write into a plaintext write of
// the fresh token.
//
// WHY IT NEEDED ITS OWN PIN. The exit is 1 today only because EVERY shim's
// refusal is (gates.go, posse_refuse). Nothing asked it of THIS shim, so a
// change to the shim template that picked 44 for some other reason would be
// silent here — the suite would stay green while every crew runtime quietly
// started reading a plaintext file. The pin below renders the real gates for
// a PID that shims the default runtime's declared CredBin, execs the rendered
// shim with the runtime's own read argv, and grades the code the way the
// composite does. Its wrong arm is the mutant D3's own verification checklist
// names: the same shim rendered to exit 44 must be red.
//
// The launch-time half of ADR 0042 (D2, the mint precondition) is pinned in
// credgatecollision_qa_test.go and does not reach the rendered shim at all.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// credNullExits is the composite's NULL set — the codes that are not a
// failure to the runtime but an invitation to read the plaintext file. Copied
// here from the measurement (ADR 0019 D2, restated at 0042 D3); the copy is
// held to its source by TestQACredNullExitSetIsTheOneTheDecisionNames.
var credNullExits = []int{0, 36, 44}

// credExitVerdict grades one exit code the way the composite does. "" is a
// read failure, which is what D3 requires of the shim; a non-empty string is
// the reason the code is null, and is the sentence the mutant must earn.
func credExitVerdict(code int) string {
	for _, n := range credNullExits {
		if code == n {
			return fmt.Sprintf("exit %d, one of the composite's null codes %v — the runtime falls through to the plaintext credentials file instead of failing the read (ADR 0042 D3)", code, credNullExits)
		}
	}
	return ""
}

// credRefusalExitRe finds the exit of the rendered shim's refusal branch —
// the last line of posse_refuse, whatever code it carries.
var credRefusalExitRe = regexp.MustCompile(`(?m)^(  exit )(\d+)(\n\}$)`)

// credShimRig is the rendered gate this is all about: a real persona gate,
// rendered from a PID that denies the DEFAULT RUNTIME's own declared
// credential binary, plus the argv that runtime reads its credential with.
type credShimRig struct {
	rt   *Runtime
	rule string
	shim string
	argv []string
	leak string // written iff some arm walked past the refusal
}

func newCredShimRig(t *testing.T) credShimRig {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	b, _ := newTestBackend(t)
	rt := rt0(t, b)
	if rt.CredBin == "" {
		t.Skipf("%s declares no credential binary, so no shim ever stands in front of its own read", rt.Name)
	}

	// A stub under the credential binary's name, first on PATH, doing two
	// jobs. It gives the shim a real binary to resolve OUTSIDE the gates dir
	// on any box — so this runs and means the same under `make test-linux`,
	// where the runtime's darwin binary does not exist — and it is the
	// witness: a `find-generic-password` that reached the operator's actual
	// keychain is the live read this package refuses to make in a test, a
	// prompt on their screen included (gatedkeychain_test.go).
	leak := filepath.Join(t.TempDir(), "leaked")
	stubDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubDir, rt.CredBin), []byte("#!/bin/sh\necho LEAK \"$@\" >>'"+leak+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	rule := "Bash(" + rt.CredBin + ":*)"
	deny := []string{"Edit", rule}
	_, binDir, _, err := b.App.RenderGates("ranger", deny)
	if err != nil {
		t.Fatal(err)
	}
	// The pair under test is the one D2 refuses a launch over, asked through
	// the predicate that decides it rather than assumed from the deny string:
	// if this PID does not actually shim the runtime's credential read, every
	// exit code measured below is some other shim's.
	if got := CredGateCollision(rt, deny, binDir); got != rule {
		t.Fatalf("this PID must shim %s's own credential binary %q — CredGateCollision says %q", rt.Name, rt.CredBin, got)
	}
	// The read's argv comes from the one place production builds it, so the
	// pin cannot drift from the read it claims to be about.
	return credShimRig{rt: rt, rule: rule, shim: filepath.Join(binDir, rt.CredBin), argv: keychainCmd(rt.CredBin).Args[1:], leak: leak}
}

// credRun execs bin and hands back its exit code and combined output.
func credRun(t *testing.T, bin string, argv []string) (int, string) {
	t.Helper()
	out, err := exec.Command(bin, argv...).CombinedOutput()
	switch e := err.(type) {
	case nil:
		return 0, string(out)
	case *exec.ExitError:
		return e.ExitCode(), string(out)
	default:
		t.Fatalf("exec %s %v: %v", bin, argv, err)
		return 0, ""
	}
}

// ADR 0042 D3, verification checklist item 3, first half.
func TestQACredBinShimRefusalExitIsOutsideTheCompositesNullCodes(t *testing.T) {
	r := newCredShimRig(t)

	code, out := credRun(t, r.shim, r.argv)

	// A code is only evidence about the refusal if the refusal is what
	// produced it: exit 127 from a shim that found no binary, or the stub's
	// own 0 from one that fell through, would both read as "outside the null
	// set" and pin nothing.
	if !strings.Contains(out, "refused by posse gate: "+r.rt.CredBin) || !strings.Contains(out, "deny: "+r.rule) {
		t.Fatalf("%s's own credential read must be REFUSED by the shim, not answered (exit %d):\n%s", r.rt.Name, code, out)
	}
	if _, err := os.Stat(r.leak); err == nil {
		t.Fatalf("the shim exec'd the real %s instead of refusing", r.rt.CredBin)
	}

	if why := credExitVerdict(code); why != "" {
		t.Errorf("the rendered %s shim refuses %s's credential read with %s", r.rt.CredBin, r.rt.Name, why)
	}
}

// The wrong arm, named by D3 itself: a shim rendered to exit 44 must turn the
// pin above red. The mutant is the RENDERED bytes with one line edited, so it
// differs from the shipped shim in nothing but the code — which is what makes
// the red above D3's red and not a broken-shim red.
func TestQACredBinShimNullExitTurnsThePinRed(t *testing.T) {
	r := newCredShimRig(t)
	body, err := os.ReadFile(r.shim)
	if err != nil {
		t.Fatal(err)
	}
	// The refusal's own exit line, LOCATED rather than assumed: writing the
	// shipped 1 in here would re-pin the incidental value this file exists to
	// stop depending on, and would red this arm on a move to some other
	// read-failure code that D3 is perfectly happy with. What the arm needs
	// is that the branch has exactly one exit to edit — an arm that mutates
	// nothing passes for free.
	loc := credRefusalExitRe.FindAllStringSubmatchIndex(string(body), -1)
	if len(loc) != 1 {
		t.Fatalf("the refusal branch's exit is no longer a single line this arm can edit (%d matches) — re-derive it from renderShim:\n%s", len(loc), body)
	}

	for _, code := range credNullExits {
		t.Run(fmt.Sprintf("exit%d", code), func(t *testing.T) {
			mutant := filepath.Join(t.TempDir(), r.rt.CredBin)
			mutated := credRefusalExitRe.ReplaceAllString(string(body), fmt.Sprintf("${1}%d${3}", code))
			if err := os.WriteFile(mutant, []byte(mutated), 0o755); err != nil {
				t.Fatal(err)
			}

			got, out := credRun(t, mutant, r.argv)
			if !strings.Contains(out, "refused by posse gate: "+r.rt.CredBin) || !strings.Contains(out, "deny: "+r.rule) {
				t.Fatalf("the mutant must still refuse — only its exit code may differ:\n%s", out)
			}
			if got != code {
				t.Fatalf("the mutant must exit %d, got %d:\n%s", code, got, out)
			}
			if why := credExitVerdict(got); why == "" {
				t.Errorf("a shim exiting %d sends %s to its plaintext fallback file, and the grader passed it", code, r.rt.Name)
			}
		})
	}
}

// credNullExits is a copy of a measurement made elsewhere, and a copy that
// nothing holds to its source is a constant that goes quietly wrong. The
// decision this file pins states the set in one place; render the set from
// the constant and require the page to still carry it.
func TestQACredNullExitSetIsTheOneTheDecisionNames(t *testing.T) {
	t.Parallel()
	const adr = "0042-runtime-credential-binary-under-the-wall.md"
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "adr", adr))
	if err != nil {
		t.Fatal(err)
	}
	parts := make([]string, 0, len(credNullExits))
	for _, n := range credNullExits {
		parts = append(parts, fmt.Sprint(n))
	}
	want := "(" + strings.Join(parts, ", ") + ")"
	if !strings.Contains(string(body), want) {
		t.Errorf("ADR 0042 D3 no longer names the null set %s that credNullExits copies — the grader in this file is measuring against a set the decision has moved off", want)
	}
}
