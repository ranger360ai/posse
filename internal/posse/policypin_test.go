//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-s69ep: a policy-tier drop-in that still pins GIT_CONFIG_SYSTEM
// voids the launch pin's row. A fixture policy dir carrying the stale row
// turns the row DEGRADED; one without it reads clean.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const policyPinFixtureWant = "/etc/gitconfig"

func writePolicyFixture(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyPinStaleDropInIsDegraded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePolicyFixture(t, dir, "managed-settings.d/10-posse-inlet-pin.json",
		`{"env":{"BASH_ENV":"/dev/null","GIT_CONFIG_SYSTEM":"/dev/null"}}`)
	writePolicyFixture(t, dir, "managed-settings.d/20-posse-field-pin.json", `{"apiKeyHelper":""}`)
	fs := policyGitSystemScopeFindings(dir, policyPinFixtureWant)
	if len(fs) != 1 || !fs[0].Stale || fs[0].Value != "/dev/null" || filepath.Base(fs[0].File) != "10-posse-inlet-pin.json" {
		t.Fatalf("findings = %+v; want one stale /dev/null finding naming 10-posse-inlet-pin.json", fs)
	}
	d := policyPinDegraded(fs, policyPinFixtureWant)
	for _, w := range []string{"DEGRADED", "10-posse-inlet-pin.json", `"/dev/null"`, "shipped copy"} {
		if !strings.Contains(d, w) {
			t.Errorf("launch line %q does not contain %q", d, w)
		}
	}
	if row := policyPinRowDetail(dir, policyPinFixtureWant, fs); !strings.HasPrefix(row, "DEGRADED") {
		t.Errorf("row = %q; want it to start DEGRADED", row)
	}
}

func TestPolicyPinBaseFileIsReadToo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePolicyFixture(t, dir, "managed-settings.json", `{"env":{"GIT_CONFIG_SYSTEM":"/somewhere/else"}}`)
	fs := policyGitSystemScopeFindings(dir, policyPinFixtureWant)
	if len(fs) != 1 || !fs[0].Stale || filepath.Base(fs[0].File) != "managed-settings.json" {
		t.Fatalf("findings = %+v; want managed-settings.json named stale", fs)
	}
}

func TestPolicyPinWithoutTheRowIsClean(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"no row":          `{"env":{"BASH_ENV":"/dev/null","GIT_PAGER":""}}`,
		"same value":      `{"env":{"GIT_CONFIG_SYSTEM":"/etc/gitconfig"}}`,
		"same, uncleaned": `{"env":{"GIT_CONFIG_SYSTEM":"/etc//gitconfig"}}`,
		"no env":          `{"apiKeyHelper":""}`,
	} {
		dir := t.TempDir()
		writePolicyFixture(t, dir, "managed-settings.d/10-posse-inlet-pin.json", body)
		fs := policyGitSystemScopeFindings(dir, policyPinFixtureWant)
		if len(fs) != 0 {
			t.Errorf("%s: findings = %+v; want none", name, fs)
		}
		if d := policyPinDegraded(fs, policyPinFixtureWant); d != "" {
			t.Errorf("%s: launch line = %q; want none", name, d)
		}
		if row := policyPinRowDetail(dir, policyPinFixtureWant, fs); strings.Contains(row, "DEGRADED") {
			t.Errorf("%s: row = %q; want clean", name, row)
		}
	}
}

func TestPolicyPinMissingDirIsClean(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "absent")
	if fs := policyGitSystemScopeFindings(dir, policyPinFixtureWant); len(fs) != 0 {
		t.Errorf("findings = %+v; want none for a missing policy dir", fs)
	}
}

// A file that cannot be judged is named in the row and never reads as clean,
// but it does not earn DEGRADED: claude cannot use it either.
func TestPolicyPinUnreadableFileIsNamedNotDegraded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePolicyFixture(t, dir, "managed-settings.d/10-posse-inlet-pin.json", `{"env":`)
	fs := policyGitSystemScopeFindings(dir, policyPinFixtureWant)
	if len(fs) != 1 || fs[0].Stale || !strings.Contains(fs[0].Unread, "invalid JSON") {
		t.Fatalf("findings = %+v; want one unread invalid-JSON finding", fs)
	}
	if d := policyPinDegraded(fs, policyPinFixtureWant); d != "" {
		t.Errorf("launch line = %q; want none for an unreadable file", d)
	}
	row := policyPinRowDetail(dir, policyPinFixtureWant, fs)
	if !strings.Contains(row, "not judged") || strings.Contains(row, "DEGRADED") {
		t.Errorf("row = %q; want it to name the file as not judged, without DEGRADED", row)
	}
}

func TestPolicyPinRowOnlyForSettingsPinRuntimes(t *testing.T) {
	t.Parallel()
	p := Parity{Realized: map[string]RealizedGate{}}
	applyPolicyPin(&p, &Runtime{Name: "codex"})
	if _, ok := p.Realized[PolicyPinGate]; ok {
		t.Errorf("a runtime with no settings pin got the policy row: %v", p.Realized[PolicyPinGate])
	}
	if gateRank(PolicyPinGate) == 0 {
		t.Errorf("PolicyPinGate has no gateRank slot; it would print among the PID's own rules")
	}
}
