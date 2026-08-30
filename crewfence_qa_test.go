package posse

// QA pin for ranger-base-i312, verifying the close of ranger-base-az93 — the
// P1 that a persona can destroy the fleet's beads store by typing an unfenced
// spelling of a bd verb (`bd daemons stop`, `bd admin reset`, `bd doctor
// --fix`, `bd hook post-merge`, `bd rename`, `bd config unset`, `bd repo add`,
// and the two egress verbs `bd jira sync` / `bd linear sync`).
//
// az93 was closed on the operator applying ten deny rows to this repo's
// .claude/settings.json by hand (commit b100b60, "operator's hand"). Verified:
// the applied file is byte-identical to the staged draft, and every spelling
// in az93's table is refused today.
//
// THE GAP THIS PIN CLOSES. Nothing in the suite read those rows. ADR 0015 §3
// moved the authoritative fence into the PIDs — where it travels with the
// session rather than with the repo — and scripts/verify-pid-deny-set.sh plus
// piddenyset_qa_test.go pin THAT half. But .claude/settings.json is still the
// crew fence for this checkout, it is the artifact az93 actually delivered,
// and it is edited by hand (personas deny Edit(.claude/**), so only the
// operator touches it). A hand edit that drops a row reopens a P1 silently:
// a Bash rule matching neither allow nor deny RUNS in a dispatched persona's
// mode, so the deny list is the only fence at this layer.
//
// It asks the question against scripts/verify-pid-deny-set.sh's own REQUIRED
// list rather than a list spelled here, so the two fences cannot drift apart
// while both look green — a pin written from its own copy of the answer
// measures the copy. The `posse promote` row in REQUIRED is a PID rule and is
// deliberately not required of this file; only the bd half is az93's.
//
// Mutation-checked (ranger-base-i312): drop any one bd row from
// .claude/settings.json and this names that row; empty the deny list and it
// names all 23; break the REQUIRED parse and it fails on having measured
// nothing rather than passing over an empty want-list.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const crewFenceFile = ".claude/settings.json"

// adr0015RequiredRules reads the REQUIRED array out of the script that owns
// it. Parsing shell is unlovely, but the alternative is a second copy of the
// list, which is the thing being guarded against.
func adr0015RequiredRules(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(pdsScript)
	if err != nil {
		t.Fatalf("%s: %v", pdsScript, err)
	}
	_, rest, ok := strings.Cut(string(b), "REQUIRED=(")
	if !ok {
		t.Fatalf("%s: no REQUIRED=( array — this pin can no longer read the list it checks against", pdsScript)
	}
	body, _, ok := strings.Cut(rest, "\n)")
	if !ok {
		t.Fatalf("%s: REQUIRED=( is never closed", pdsScript)
	}
	var out []string
	for _, part := range strings.Split(body, "'") {
		if strings.HasPrefix(part, "Bash(") && strings.HasSuffix(part, ")") {
			out = append(out, part)
		}
	}
	return out
}

// crewFenceDeny is the deny list this repo's Claude Code sessions run under.
func crewFenceDeny(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(crewFenceFile)
	if err != nil {
		t.Fatalf("%s: %v", crewFenceFile, err)
	}
	var f struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("%s: %v", crewFenceFile, err)
	}
	if len(f.Permissions.Deny) == 0 {
		t.Fatalf("%s: permissions.deny is empty or unparsed — at this layer the deny list is the ONLY fence, so an empty one is not a pass", crewFenceFile)
	}
	return f.Permissions.Deny
}

func TestCrewFenceCarriesEveryADR0015BdRule(t *testing.T) {
	var want []string
	for _, r := range adr0015RequiredRules(t) {
		if strings.HasPrefix(r, "Bash(bd ") {
			want = append(want, r)
		}
	}
	// Nothing measured is not a pass: if the parse above ever returns a short
	// list, every row below is trivially satisfied and the fence goes
	// unchecked while this test prints ok.
	if len(want) < 20 {
		t.Fatalf("only %d bd rules parsed out of %s (%v) — a fence check over a want-list this short is measuring nothing", len(want), pdsScript, want)
	}
	have := map[string]bool{}
	for _, d := range crewFenceDeny(t) {
		have[d] = true
	}
	missing := 0
	for _, r := range want {
		if !have[r] {
			missing++
			t.Errorf("%s does not deny %s.\n\n"+
				"That is one of ADR 0015 §3's bd rules (scripts/verify-pid-deny-set.sh REQUIRED), applied\n"+
				"to this file for ranger-base-az93. In a dispatched persona's mode a Bash command matching\n"+
				"neither allow nor deny RUNS, so removing the row does not add a prompt — it grants the verb.",
				crewFenceFile, r)
		}
	}
	if missing == 0 {
		t.Logf("all %d bd rules of ADR 0015 §3 are denied in %s", len(want), crewFenceFile)
	}
}
