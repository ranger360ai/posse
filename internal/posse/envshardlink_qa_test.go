package posse

// QA's adversarial half of the link-count guard (ranger-base-9hfgb, verified
// under ranger-base-i9fda). envs_test.go pins the bead's own repro — a hard
// link from secrets/ into envs/, both surfaces. These are the shapes that
// repro does not reach, each one an argument the fix's own comment makes:
//
//   - the second name is OUTSIDE both stores. That is the case dev+ino
//     against the sibling store was rejected FOR, and nothing measured it.
//   - the extensionless spelling, which takes envFilePath's SECOND branch:
//     `envs/bare` with no `.env`. One `storeContained` call is pinned; the
//     other was reached by nobody.
//   - a symlink inside the store pointing at a hard-linked file inside it.
//     The fix uses Stat, not Lstat, on purpose, and this is the shape that
//     says so.
//   - the whole store directory being a symlink, which is a7e4's own last
//     shape carried one mechanism over.
//
// Every one is refused for the containment reason and every one hands the
// credential straight back when `storeSingleEntry` is disarmed to
// `Nlink >= 1` (mutation-checked, all four red with the secret in the
// failure line). TestEnvStoresStillResolveACopiedCredential is why that
// means anything: the same fixture DOES hand back a copy, so these are the
// guard refusing and not a fixture that cannot leak.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// refusedWithNoLeak fails unless the reader refused AND handed back nothing
// that looks like the credential — a refusal that still returns the vars is
// not a refusal.
func refusedWithNoLeak(t *testing.T, what string, vars []EnvVar, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: got %v, want refusal", what, vars)
		return
	}
	for _, v := range vars {
		if strings.Contains(v.Value, "sk-") {
			t.Errorf("%s: the refusal still carried the credential: %v", what, v)
		}
	}
}

// The second name is a file outside BOTH stores — an ssh key, a cloud
// credential file. A dev+ino comparison against the sibling store would
// certify this entry as contained; the link count is what refuses it.
func TestEnvSetVarsRefusesAHardLinkToAFileOutsideBothStores(t *testing.T) {
	a := symlinkApp(t)
	outside := filepath.Join(t.TempDir(), "cloud-credentials")
	if err := os.WriteFile(outside, []byte("CLOUD_SECRET=sk-outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(a.EnvsDir, "cloud.env")); err != nil {
		t.Skipf("the fixture needs both paths on one device: %v", err)
	}
	vars, err := a.EnvSetVars("cloud")
	refusedWithNoLeak(t, `EnvSetVars("cloud")`, vars, err)
	if got := a.ListEnvSets(); len(got) != 1 || got[0] != "default" {
		t.Errorf("ListEnvSets() = %v, want [default] — the lister must skip it too", got)
	}
}

// The same, one store over: secrets/ is posse's own store and a name of its
// own outside it is the same escape.
func TestSecretVarsRefusesAHardLinkToAFileOutsideBothStores(t *testing.T) {
	a := symlinkApp(t)
	outside := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(outside, []byte("KEY=sk-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(a.SecretsDir, "stolen.env")); err != nil {
		t.Skipf("the fixture needs both paths on one device: %v", err)
	}
	vars, err := a.SecretVars("stolen")
	refusedWithNoLeak(t, `SecretVars("stolen")`, vars, err)
}

// envFilePath resolves a name twice — `<name>.env` first, then `<name>` —
// and each spelling reaches storeContained on its own line. The bead's repro
// only ever runs the first.
func TestEnvSetVarsRefusesAHardLinkNamedWithoutTheExtension(t *testing.T) {
	a := symlinkApp(t)
	if err := os.Link(filepath.Join(a.SecretsDir, "harness.env"), filepath.Join(a.EnvsDir, "bare")); err != nil {
		t.Fatal(err)
	}
	vars, err := a.EnvSetVars("bare")
	refusedWithNoLeak(t, `EnvSetVars("bare")`, vars, err)
}

// Stat, not Lstat: a symlink that stays inside the store is legitimate
// (TestEnvSetVarsAllowsASymlinkWithinTheStore pins that), so the count has
// to be read through it or the two guards compose into a hole.
func TestEnvSetVarsRefusesASymlinkToAHardLinkedFileInTheStore(t *testing.T) {
	a := symlinkApp(t)
	if err := os.Link(filepath.Join(a.SecretsDir, "harness.env"), filepath.Join(a.EnvsDir, "inner.env")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inner.env", filepath.Join(a.EnvsDir, "alias.env")); err != nil {
		t.Fatal(err)
	}
	vars, err := a.EnvSetVars("alias")
	refusedWithNoLeak(t, `EnvSetVars("alias")`, vars, err)
}

// a7e4's last shape, one mechanism over: the store directory is itself a
// symlink, so underDir's resolution has already moved once before the count
// is read.
func TestEnvSetVarsRefusesAHardLinkUnderASymlinkedStoreDir(t *testing.T) {
	a := symlinkApp(t)
	real := filepath.Join(t.TempDir(), "realenvs")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "default.env"), []byte("SESSION_TOKEN=s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(a.SecretsDir, "harness.env"), filepath.Join(real, "hard.env")); err != nil {
		t.Skipf("the fixture needs both paths on one device: %v", err)
	}
	if err := os.RemoveAll(a.EnvsDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, a.EnvsDir); err != nil {
		t.Fatal(err)
	}
	vars, err := a.EnvSetVars("hard")
	refusedWithNoLeak(t, `EnvSetVars("hard") under a symlinked envs/`, vars, err)
	if got := a.ListEnvSets(); len(got) != 1 || got[0] != "default" {
		t.Errorf("ListEnvSets() = %v, want [default]", got)
	}
}

// The control every refusal above depends on: the rule is the link COUNT and
// not the bytes, so a COPY of the same credential still resolves and is still
// listed. Without this the four tests above pass over a fixture that could
// not have leaked anything in the first place.
func TestEnvStoresStillResolveACopiedCredential(t *testing.T) {
	a := symlinkApp(t)
	b, err := os.ReadFile(filepath.Join(a.SecretsDir, "harness.env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "copied.env"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	vars, err := a.EnvSetVars("copied")
	if err != nil || len(vars) != 1 || vars[0].Value != "sk-secret" {
		t.Fatalf("EnvSetVars(copied) = %v, %v — a copy has one name and must resolve", vars, err)
	}
	if got := a.ListEnvSets(); len(got) != 2 || got[0] != "copied" || got[1] != "default" {
		t.Fatalf("ListEnvSets() = %v, want [copied default]", got)
	}
}
