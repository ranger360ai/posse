package posse

// ADR 0019 D1's one-hand rule, as tests: everything under envs/ may reach a
// session; nothing under secrets/ ever does. The rule is only worth the
// paper if the two directories cannot be reached through each other, so
// every pin here is written to go red when they can be.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// secretsApp is a home with one of each class in it: an env set a PID may
// name, and a harness secret no PID may.
func secretsApp(t *testing.T) *App {
	t.Helper()
	a := NewAppAt(t.TempDir())
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.SecretsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "default.env"), []byte("SESSION_TOKEN=s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.SecretsDir, "meter.env"), []byte("# a harness credential\nexport METER_TOKEN=h\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return a
}

// P3: `posse envs` never contains a name from secrets/, and the property is
// structural — the lister and the loader share no directory. Both directions
// are pinned, because a fallback added to either resolver breaks the rule.
func TestSecretsAndEnvSetsShareNoDirectory(t *testing.T) {
	t.Parallel()
	a := secretsApp(t)

	if got := a.ListEnvSets(); len(got) != 1 || got[0] != "default" {
		t.Errorf("ListEnvSets() = %v — the lister reads envs/ and nothing else", got)
	}
	if _, err := a.EnvSetVars("meter"); err == nil {
		t.Error("EnvSetVars(\"meter\") resolved a harness secret — a PID's envs: must not reach secrets/")
	}
	if _, err := a.SecretVars("default"); err == nil {
		t.Error("SecretVars(\"default\") resolved an env set — the harness loader must not reach envs/")
	}

	// And the loader does load: same grammar as an env set, shared parser,
	// comments and `export ` alike.
	vars, err := a.SecretVars("meter")
	if err != nil {
		t.Fatalf("SecretVars(meter): %v", err)
	}
	if len(vars) != 1 || vars[0].Key != "METER_TOKEN" || vars[0].Value != "h" {
		t.Errorf("SecretVars(meter) = %+v, want one METER_TOKEN=h", vars)
	}
}

// The injection risk the rule is actually about: a PID's `envs:` is prose,
// and prose can spell a path. A name is a file stem in both stores.
func TestNoNameCanCrossBetweenTheTwoStores(t *testing.T) {
	t.Parallel()
	a := secretsApp(t)
	for _, name := range []string{"../secrets/meter", "../secrets/meter.env", "..", ".", "", "sub/meter"} {
		if _, err := a.EnvSetVars(name); err == nil {
			t.Errorf("EnvSetVars(%q) resolved — an env set name must be a file stem, not a path", name)
		}
	}
	for _, name := range []string{"../envs/default", "..", "", "sub/default"} {
		if _, err := a.SecretVars(name); err == nil {
			t.Errorf("SecretVars(%q) resolved — a harness secret name must be a file stem, not a path", name)
		}
	}
	// The control: the traversal names a file that really is there, so the
	// refusals above are the guard and not a missing fixture.
	if _, err := os.Stat(filepath.Join(a.SecretsDir, "meter.env")); err != nil {
		t.Fatalf("fixture: %v", err)
	}
}

// D1: init seeds the EMPTY directory. Not a seed file, and above all not
// plan-guard.env — the plan guard is not a consumer.
func TestInitSeedsAnEmptySecretsDir(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(a.SecretsDir)
	if err != nil {
		t.Fatalf("secrets/ not seeded: %v", err)
	}
	if st.Mode().Perm() != 0o700 {
		t.Errorf("secrets/ mode %04o, want 0700", st.Mode().Perm())
	}
	ents, err := os.ReadDir(a.SecretsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("secrets/ seeded %v — init seeds the empty dir and no credential file", names)
	}

	// Re-running init leaves an operator's own secret alone, the way it
	// leaves an env set alone.
	f := filepath.Join(a.SecretsDir, "mine.env")
	if err := os.WriteFile(f, []byte("K=v\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f); string(b) != "K=v\n" {
		t.Errorf("init rewrote a harness secret: %q", string(b))
	}
}

// TightenEnvPerms parity: same belt, same silence, same names-never-contents
// notice — and a store that is not there is not a drift (P6).
func TestTightenSecretPermsIsEnvParity(t *testing.T) {
	t.Parallel()
	a := secretsApp(t)
	f := filepath.Join(a.SecretsDir, "meter.env")
	os.Chmod(f, 0o644)
	os.Chmod(a.SecretsDir, 0o755)

	var notes strings.Builder
	a.TightenSecretPerms(&notes)
	if st, _ := os.Stat(a.SecretsDir); st.Mode().Perm() != 0o700 {
		t.Errorf("secrets dir mode %04o, want 0700", st.Mode().Perm())
	}
	if st, _ := os.Stat(f); st.Mode().Perm() != 0o600 {
		t.Errorf("secret file mode %04o, want 0600", st.Mode().Perm())
	}
	out := notes.String()
	if !strings.Contains(out, "meter.env was 0644") || strings.Contains(out, "METER_TOKEN") {
		t.Errorf("notice must name the file and never its contents:\n%s", out)
	}
	notes.Reset()
	a.TightenSecretPerms(&notes)
	if notes.Len() != 0 {
		t.Errorf("second pass must be silent, got:\n%s", notes.String())
	}
	// The read path tightens too, like EnvSetVars.
	os.Chmod(f, 0o644)
	if _, err := a.SecretVars("meter"); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(f); st.Mode().Perm() != 0o600 {
		t.Errorf("SecretVars left %04o, want 0600", st.Mode().Perm())
	}
	// Tightening one store never touches the other's directory.
	os.Chmod(a.EnvsDir, 0o755)
	notes.Reset()
	a.TightenSecretPerms(&notes)
	if st, _ := os.Stat(a.EnvsDir); st.Mode().Perm() != 0o755 {
		t.Errorf("TightenSecretPerms touched envs/ (mode %04o) — the stores tighten separately", st.Mode().Perm())
	}
}

// P6: absent or empty secrets/ is the shipped state. Nothing crashes and
// nothing is said.
func TestAbsentSecretsDirIsNotAnError(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	var notes strings.Builder
	a.TightenSecretPerms(&notes)
	if notes.Len() != 0 {
		t.Errorf("a store that is not there is not a drift, got:\n%s", notes.String())
	}
	if _, err := a.SecretVars("meter"); err == nil {
		t.Error("SecretVars on an absent store must say not-found, not succeed")
	}
	if err := os.MkdirAll(a.SecretsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	a.TightenSecretPerms(&notes)
	if notes.Len() != 0 {
		t.Errorf("an empty store is not a drift, got:\n%s", notes.String())
	}
}

// A store a session may not be handed is not a store a session may edit:
// secrets/ joins envs/ in the constitution area no writable set may reach
// (ADR 0015 §2/§7, ADR 0019 D1).
func TestSecretsAreInNoSessionsWritableSet(t *testing.T) {
	t.Parallel()
	a := secretsApp(t)
	var named bool
	for _, p := range a.HomeConstitutionPaths() {
		if p == a.SecretsDir {
			named = true
		}
	}
	if !named {
		t.Fatalf("HomeConstitutionPaths() = %v — secrets/ must be in it", a.HomeConstitutionPaths())
	}
	if bad := a.ConstitutionGrants([]string{a.SecretsDir}); len(bad) != 1 || bad[0] != a.SecretsDir {
		t.Errorf("ConstitutionGrants([secrets]) = %v, want it flagged", bad)
	}
	if bad := a.ConstitutionGrants([]string{filepath.Join(a.SecretsDir, "meter.env")}); len(bad) != 1 {
		t.Errorf("a grant INSIDE the store is the same breach spelled smaller: %v", bad)
	}
	if bad := a.ConstitutionGrants([]string{a.StateDir}); len(bad) != 0 {
		t.Errorf("state/ is granted by design and must not be flagged: %v", bad)
	}
}

// The wiring, not the function: every launch re-asserts the modes on BOTH
// credential stores. Pinned at planLaunch because deleting the call site is
// invisible to a test that only calls TightenSecretPerms itself — the belt
// existing and the belt being worn are two different claims.
func TestEveryLaunchReAssertsBothCredentialStores(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	writeVoidPID(t, a, "dev", "claude", "")
	for _, d := range []string{a.EnvsDir, a.SecretsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	envFile := filepath.Join(a.EnvsDir, "default.env")
	secretFile := filepath.Join(a.SecretsDir, "meter.env")
	for _, f := range []string{envFile, secretFile} {
		if err := os.WriteFile(f, []byte("K=v\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chmod(f, 0o644)
	}
	os.Chmod(a.EnvsDir, 0o755)
	os.Chmod(a.SecretsDir, 0o755)

	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "dev"}); err != nil {
		t.Fatalf("planLaunch: %v", err)
	}
	for _, c := range []struct {
		path string
		want os.FileMode
	}{
		{a.EnvsDir, 0o700}, {envFile, 0o600},
		{a.SecretsDir, 0o700}, {secretFile, 0o600},
	} {
		st, err := os.Stat(c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if st.Mode().Perm() != c.want {
			t.Errorf("after a launch %s is %04o, want %04o", AbbrevHome(c.path), st.Mode().Perm(), c.want)
		}
	}
}
