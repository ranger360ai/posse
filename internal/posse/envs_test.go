//go:build posse_arm3

package posse

// storeContained is storeName's other half: the guard on the NAME does not
// see where a symlinked name RESOLVES. Pinned both ways round — a relative
// and an absolute symlink out of the store refused, an ordinary set still
// resolving — and the lister skips a symlink instead of naming it a set
// (ranger-base-a7e4).

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func symlinkApp(t *testing.T) *App {
	t.Helper()
	a := NewAppAt(t.TempDir())
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.SecretsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.SecretsDir, "harness.env"), []byte("HARNESS_TOKEN=sk-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "default.env"), []byte("SESSION_TOKEN=s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return a
}

// The repro from ranger-base-a7e4: a relative symlink inside envs/ that
// resolves into secrets/ must not hand a harness credential back through
// EnvSetVars — the reader herdrback.go uses to build a launch's env.
func TestEnvSetVarsRefusesASymlinkOutOfTheStore(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	target := filepath.Join(a.SecretsDir, "harness.env")

	rel := filepath.Join(a.EnvsDir, "leak-relative.env")
	if err := os.Symlink("../secrets/harness.env", rel); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(a.EnvsDir, "leak-absolute.env")
	if err := os.Symlink(target, abs); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"leak-relative", "leak-absolute"} {
		if vars, err := a.EnvSetVars(name); err == nil {
			t.Errorf("EnvSetVars(%q) = %v, want refusal — the symlink resolves into secrets/", name, vars)
		}
	}

	// The control: an ordinary set with no symlink involved still resolves.
	vars, err := a.EnvSetVars("default")
	if err != nil || len(vars) != 1 || vars[0].Key != "SESSION_TOKEN" {
		t.Errorf("EnvSetVars(default) = %v, %v — an ordinary set must still resolve", vars, err)
	}
}

// The same shape one store over: secrets/ is never reached by a PID's
// envs:, but the resolver that reads it must equally refuse a symlink
// pointed elsewhere outside secrets/ — same guard, same reason.
func TestSecretVarsRefusesASymlinkOutOfTheStore(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	link := filepath.Join(a.SecretsDir, "escape.env")
	if err := os.Symlink(filepath.Join(a.EnvsDir, "default.env"), link); err != nil {
		t.Fatal(err)
	}
	if vars, err := a.SecretVars("escape"); err == nil {
		t.Errorf("SecretVars(\"escape\") = %v, want refusal — the symlink resolves outside secrets/", vars)
	}
	vars, err := a.SecretVars("harness")
	if err != nil || len(vars) != 1 || vars[0].Key != "HARNESS_TOKEN" {
		t.Errorf("SecretVars(harness) = %v, %v — an ordinary secret must still resolve", vars, err)
	}
}

// ListEnvSets() must not name the symlink as a set `posse envs` can offer
// the operator — the same containment question one directory up.
func TestListEnvSetsSkipsASymlink(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	if err := os.Symlink(filepath.Join(a.SecretsDir, "harness.env"), filepath.Join(a.EnvsDir, "leak.env")); err != nil {
		t.Fatal(err)
	}
	got := a.ListEnvSets()
	if len(got) != 1 || got[0] != "default" {
		t.Errorf("ListEnvSets() = %v, want [default] — a symlink is not a set the operator wrote", got)
	}
}

// A symlink that resolves to a file INSIDE the store (envs/alias.env ->
// envs/default.env) is not the injection case ADR 0019 D1 is about — it
// never crosses stores — so it must keep resolving.
func TestEnvSetVarsAllowsASymlinkWithinTheStore(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	a := symlinkApp(t)
	if err := os.Symlink(filepath.Join(a.EnvsDir, "default.env"), filepath.Join(a.EnvsDir, "alias.env")); err != nil {
		t.Fatal(err)
	}
	vars, err := a.EnvSetVars("alias")
	if err != nil || len(vars) != 1 || vars[0].Key != "SESSION_TOKEN" {
		t.Errorf("EnvSetVars(alias) = %v, %v — a symlink that stays inside envs/ is not the injection case", vars, err)
	}
}

// A HARD link is the same invariant one mechanism over (ranger-base-9hfgb).
// envs/hard.env IS a directory entry inside envs/, so it resolves inside
// envs/ and underDir is right to say so — the file left the store when the
// link was made, not when the name was resolved. Same repro as the symlink
// above, same reader, and it must refuse the same way.
func TestEnvSetVarsRefusesAHardLinkOutOfTheStore(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	if err := os.Link(filepath.Join(a.SecretsDir, "harness.env"), filepath.Join(a.EnvsDir, "hard.env")); err != nil {
		t.Fatal(err)
	}
	if vars, err := a.EnvSetVars("hard"); err == nil {
		t.Errorf("EnvSetVars(\"hard\") = %v, want refusal — the entry shares its inode with secrets/harness.env", vars)
	}
	// The control: an ordinary set, one name and one inode, still resolves.
	vars, err := a.EnvSetVars("default")
	if err != nil || len(vars) != 1 || vars[0].Key != "SESSION_TOKEN" {
		t.Errorf("EnvSetVars(default) = %v, %v — an ordinary set must still resolve", vars, err)
	}
}

// The same shape one store over, for the same reason the symlink case is
// pinned both ways: secrets/ is read by posse's own processes, and a second
// name for a file outside it is the same escape whichever store it is in.
func TestSecretVarsRefusesAHardLinkOutOfTheStore(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	if err := os.Link(filepath.Join(a.EnvsDir, "default.env"), filepath.Join(a.SecretsDir, "escape.env")); err != nil {
		t.Fatal(err)
	}
	if vars, err := a.SecretVars("escape"); err == nil {
		t.Errorf("SecretVars(\"escape\") = %v, want refusal — the entry shares its inode with envs/default.env", vars)
	}
	vars, err := a.SecretVars("harness")
	if err != nil || len(vars) != 1 || vars[0].Key != "HARNESS_TOKEN" {
		t.Errorf("SecretVars(harness) = %v, %v — an ordinary secret must still resolve", vars, err)
	}
}

// The lister is the second surface, exactly as it was for the symlink: a
// hard link is a regular file, so ReadDir's type bits say yes and `posse
// envs` would name it a set the operator wrote.
func TestListEnvSetsSkipsAHardLink(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	if err := os.Link(filepath.Join(a.SecretsDir, "harness.env"), filepath.Join(a.EnvsDir, "hard.env")); err != nil {
		t.Fatal(err)
	}
	got := a.ListEnvSets()
	if len(got) != 1 || got[0] != "default" {
		t.Errorf("ListEnvSets() = %v, want [default] — a hard link is not a set the operator wrote", got)
	}
}

// The deliberate cost, pinned so it is a decision and not an accident: the
// rule is the LINK COUNT, not where the other name is. A second name inside
// envs/ is refused too, because nothing short of walking the filesystem can
// tell it from a second name in secrets/ — and a dev+ino comparison against
// the sibling store, the only cheaper alternative, would leave every other
// file on the box linkable into a set a session is handed.
func TestEnvSetVarsRefusesAHardLinkWithinTheStore(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	if err := os.Link(filepath.Join(a.EnvsDir, "default.env"), filepath.Join(a.EnvsDir, "alias.env")); err != nil {
		t.Fatal(err)
	}
	if vars, err := a.EnvSetVars("alias"); err == nil {
		t.Errorf("EnvSetVars(\"alias\") = %v, want refusal — the link count, not the target, is the rule", vars)
	}
}

// WriteEnvSet's own guard (ranger-base-wafvm): its siblings all resolve the
// name through storeName before it ever reaches EnvsDir. A name holding a
// path separator must not write outside envs/.
func TestWriteEnvSetRefusesAPathSeparatorInTheName(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	outside := t.TempDir()
	name := filepath.Join(outside, "escape") + string(filepath.Separator) + "x"
	if err := a.WriteEnvSet(name, []EnvVar{{Key: "K", Value: "v"}}); err == nil {
		t.Errorf("WriteEnvSet(%q) = nil, want refusal — the name holds a path separator", name)
	}
	if _, err := os.Stat(filepath.Join(outside, "escape")); err == nil {
		t.Errorf("WriteEnvSet(%q) wrote outside envs/ despite refusing", name)
	}
}

// WriteEnvSet must not write THROUGH a hard link (ranger-base-wafvm): the
// readers' guard (storeContained, ListEnvSets) is on link count at read
// time, but os.WriteFile truncates the inode it finds regardless of how
// many names point at it — so a name hard-linked to secrets/harness.env
// would clobber the harness credential in place. The fix breaks the link
// with a temp-file rename instead, the same shape setEnvVarStamped uses.
func TestWriteEnvSetBreaksAHardLinkInsteadOfWritingThroughIt(t *testing.T) {
	t.Parallel()
	a := symlinkApp(t)
	secret := filepath.Join(a.SecretsDir, "harness.env")
	linked := filepath.Join(a.EnvsDir, "hard.env")
	if err := os.Link(secret, linked); err != nil {
		t.Skipf("the fixture needs both paths on one device: %v", err)
	}

	if err := a.WriteEnvSet("hard", []EnvVar{{Key: "SESSION_TOKEN", Value: "s"}}); err != nil {
		t.Fatalf("WriteEnvSet(hard) = %v, want success", err)
	}

	b, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "HARNESS_TOKEN=sk-secret") {
		t.Errorf("secrets/harness.env = %q, want the harness credential intact — the write went through the link", b)
	}

	b, err = os.ReadFile(linked)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "SESSION_TOKEN=s") {
		t.Errorf("envs/hard.env = %q, want the new vars — the rename should have replaced the directory entry", b)
	}
}
