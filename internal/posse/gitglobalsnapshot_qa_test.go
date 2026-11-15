//go:build posse_arm3

package posse

// The GIT_CONFIG_GLOBAL row names a read-only snapshot (ranger-base-161t2).
//
// OBSERVED 2026-11-15: a probe on a shims-tier seat ran `export
// HOME=<mktemp>/home; git config --global user.name probe` meaning a scratch
// HOME. obdnk's row named the operator's own ~/.config/git/config, the row
// replaces git's whole global lookup, and so the write landed in the
// operator's file and every later commit on the box carried probe/probe@invalid.
// These pins run that shape against a fixture HOME and hold that no seat
// env the pin renders can change the operator's global identity.
//
// Not parallel: inletPin() reads this process's HOME (globalScopeFixture).

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// seatGit runs git under the env a seat gets: this process's env with every
// GIT_CONFIG* shed, the whole inlet pin applied, then extra (last wins).
func seatGit(t *testing.T, row string, extra []string, args ...string) (string, int) {
	t.Helper()
	env := gitConfigScrubbedEnv()
	for _, v := range inletPin() {
		if v.Key == "GIT_CONFIG_GLOBAL" {
			v.Value = row
		}
		env = append(env, v.Key+"="+v.Value)
	}
	c := exec.Command("git", args...)
	c.Env, c.Dir = append(env, extra...), t.TempDir()
	out, err := c.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode()
		}
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out), 0
}

// TestQAASeatCannotRewriteTheOperatorsGlobalGitIdentity is the incident:
// the probe's exact lines, under the rendered row, leave the operator's file
// byte-identical and refuse at a nonzero rc — and under the row as it stood
// (the operator's own file) the same lines DO rewrite it, so the pinned arm
// measures something.
func TestQAASeatCannotRewriteTheOperatorsGlobalGitIdentity(t *testing.T) {
	for _, c := range []struct {
		name          string
		homeF, xdgF   bool
		operatorsFile func(homeCfg, xdgCfg string) string
	}{
		{"only ~/.gitconfig", true, false, func(h, _ string) string { return h }},
		{"only the XDG file (this box's shape)", false, true, func(_, x string) string { return x }},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, homeCfg, xdgCfg := globalScopeFixture(t, c.homeF, c.xdgF)
			op := c.operatorsFile(homeCfg, xdgCfg)
			before, err := os.ReadFile(op)
			if err != nil {
				t.Fatal(err)
			}
			row, ok := pinRow("GIT_CONFIG_GLOBAL")
			if !ok {
				t.Fatal("CONTROL FAILED: a one-file fixture produced no GIT_CONFIG_GLOBAL row")
			}
			identBefore, _ := seatGit(t, row, nil, "var", "GIT_AUTHOR_IDENT")

			scratch := []string{"HOME=" + filepath.Join(t.TempDir(), "home")}
			probe := [][]string{
				{"config", "--global", "user.name", "probe"},
				{"config", "--global", "user.email", "probe@invalid"},
				{"config", "--file", row, "user.email", "p@x"},
				{"config", "--global", "--unset", "user.email"},
			}
			for _, args := range probe {
				out, rc := seatGit(t, row, scratch, args...)
				if rc == 0 {
					t.Errorf("`git %s` under the rendered row exited 0 — a seat's global write must refuse, not land somewhere quietly", strings.Join(args, " "))
				} else if !strings.Contains(out, "could not lock config file") {
					t.Errorf("`git %s` refused, but not at the config lock: %s", strings.Join(args, " "), out)
				}
			}
			if after, _ := os.ReadFile(op); !bytes.Equal(after, before) {
				t.Fatalf("the operator's %s changed under the rendered row:\n before %q\n after  %q", op, before, after)
			}
			if after, _ := os.ReadFile(row); !bytes.Equal(after, before) {
				t.Errorf("the snapshot %s no longer reads as the operator's file", row)
			}
			if _, err := os.Stat(homeCfg); c.homeF == false && err == nil {
				t.Errorf("a --global write created %s", homeCfg)
			}
			if got, _ := seatGit(t, row, nil, "var", "GIT_AUTHOR_IDENT"); identStrip(got) != identStrip(identBefore) || !strings.Contains(got, "@example.invalid") {
				t.Errorf("the seat's author identity moved: before %q, after %q", identBefore, got)
			}

			// CONTROL: the row as it stood names the operator's file, and the
			// same first line rewrites it. If this stops holding, the arm above
			// measures nothing.
			if _, rc := seatGit(t, op, scratch, probe[0]...); rc != 0 {
				t.Fatalf("CONTROL FAILED: `git config --global` under GIT_CONFIG_GLOBAL=%s exited %d", op, rc)
			}
			if after, _ := os.ReadFile(op); bytes.Equal(after, before) {
				t.Fatal("CONTROL FAILED: the old row's write did not reach the operator's file, so the pinned arm measures nothing")
			}
		})
	}
}

// identStrip drops git var's trailing timestamp, which moves per call.
func identStrip(ident string) string {
	if i := strings.Index(ident, ">"); i >= 0 {
		return ident[:i+1]
	}
	return ident
}

// TestQAGitGlobalNoFileSnapshotsEmptyAndCreatesNothing: with no global file
// the old row named ~/.gitconfig, so the first --global write CREATED the
// operator's file. The snapshot is empty and the write refuses.
func TestQAGitGlobalNoFileSnapshotsEmptyAndCreatesNothing(t *testing.T) {
	_, homeCfg, xdgCfg := globalScopeFixture(t, false, false)
	row, ok := pinRow("GIT_CONFIG_GLOBAL")
	if !ok {
		t.Fatal("no GIT_CONFIG_GLOBAL row with no global file")
	}
	if b, err := os.ReadFile(row); err != nil || len(b) != 0 {
		t.Fatalf("the snapshot of no file is not an empty file: %q, %v", b, err)
	}
	if _, rc := seatGit(t, row, nil, "config", "--global", "user.name", "probe"); rc == 0 {
		t.Error("a --global write exited 0 under the rendered row")
	}
	for _, f := range []string{homeCfg, xdgCfg} {
		if _, err := os.Stat(f); err == nil {
			t.Errorf("a --global write created %s", f)
		}
	}
}

// TestQAGitGlobalSnapshotWithIncludesDegradesAndSaysSo: a relative include
// resolves against the including file's own directory, so a copy elsewhere
// would read different config. No snapshot; the row keeps the original and
// the launch names the write route as open.
func TestQAGitGlobalSnapshotWithIncludesDegradesAndSaysSo(t *testing.T) {
	_, homeCfg, _ := globalScopeFixture(t, true, false)
	for _, hdr := range []string{"[include]\n\tpath = extra\n", "[IncludeIf \"gitdir:~/x/\"]\n\tpath = extra\n"} {
		if err := os.WriteFile(homeCfg, []byte("[user]\n\temail = home@example.invalid\n"+hdr), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _ := pinRow("GIT_CONFIG_GLOBAL"); got != homeCfg {
			t.Errorf("with %q the row is %q, want the original %s (a snapshot would move its relative include)", hdr, got, homeCfg)
		}
		d := GitGlobalScopeDegraded()
		for _, want := range []string{"DEGRADED", homeCfg, "include", "WRITES", "ranger-base-161t2"} {
			if !strings.Contains(d, want) {
				t.Errorf("with %q the degraded line does not say %q:\n%s", hdr, want, d)
			}
		}
	}
}

// TestQAGitGlobalSnapshotIsRewrittenWhenTampered: a seat that removed the
// lock and edited the snapshot does not hand that to the next launch over the
// same bytes.
func TestQAGitGlobalSnapshotIsRewrittenWhenTampered(t *testing.T) {
	_, homeCfg, _ := globalScopeFixture(t, true, false)
	row := gitGlobalPinOf(t, homeCfg)
	if err := os.Remove(row + ".lock"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(row, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(row, []byte("[user]\n\temail = probe@invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if again := gitGlobalPinOf(t, homeCfg); again != row {
		t.Fatalf("the same bytes snapshot to a new path: %s then %s", row, again)
	}
	want, _ := os.ReadFile(homeCfg)
	if got, _ := os.ReadFile(row); !bytes.Equal(got, want) {
		t.Errorf("the tampered snapshot was not rewritten: %q", got)
	}
	if fi, err := os.Lstat(row + ".lock"); err != nil || !fi.IsDir() {
		t.Errorf("the lock directory was not restored: %v", err)
	}
}
