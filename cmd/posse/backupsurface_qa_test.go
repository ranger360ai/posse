package main

// QA pin — ranger-base-inomb, verifying the closes of
// ranger-base-yt88 ("status BUILT, sweep and backup_dest CUT") and
// ranger-base-ymec (§4's ticker) against the user-facing surface.
//
// Everything ADR 0036 §6 says about freshness is pinned inside the package
// (internal/posse/backupfresh_qa_test.go). What no test in the tree touched
// before this file — `backup` appears in no other cmd/posse test — is the
// verb itself: the EXIT CODE `posse backup status` hands a caller, and the
// CUT measured where an operator would meet it rather than by the absence
// of a symbol. A cut that is only true in the source is a cut that comes
// back as a flag.
//
// The states below are the four an operator can be in, and the exit rule is
// ADR 0036 §6's: nonzero for on-box staleness alone.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// backupHome is a scratch instance: a config with the given keys and an
// archive directory holding one file per age in ago. The archives are
// written rather than produced — the directory listing IS the freshness
// store (an archive is published only after it verifies), and the age is
// the name's stamp, so a file of one byte is a backup of the right age for
// every reader this test drives.
func backupHome(t *testing.T, cfg string, ago ...time.Duration) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "state", "backup")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range ago {
		name := "posse-backup-" + time.Now().UTC().Add(-d).Format("20060102T150405Z") + ".tar.gz"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func runBackupPosse(t *testing.T, bin, home string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse %s: %v", strings.Join(args, " "), err)
	}
	return code, string(out)
}

// ADR 0036 §6: `posse backup status` exits nonzero for on-box staleness and
// for nothing else. The unarmed instance is the row that makes the other
// three mean something — installing posse arms nothing, so "no archive" is
// only a failure once somebody has asked for backups.
func TestBackupStatusExitsForOnBoxStalenessOnly(t *testing.T) {
	bin := buildRhq(t)
	for _, c := range []struct {
		name string
		cfg  string
		ago  []time.Duration
		code int
		want string
	}{
		{"unarmed and empty says nothing is armed", "runtime: claude\n", nil, 0, "nothing is armed"},
		{"armed and empty is stale", "runtime: claude\nbackup_interval: 1h\n", nil, 1, "NONE on box"},
		{"fresh is clear", "runtime: claude\nbackup_interval: 1h\n", []time.Duration{20 * time.Minute}, 0, "on box"},
		{"past 2x the interval is stale", "runtime: claude\nbackup_interval: 1h\n", []time.Duration{5 * time.Hour}, 1, "STALE, older than 2h00m"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, out := runBackupPosse(t, bin, backupHome(t, c.cfg, c.ago...), "backup", "status")
			if code != c.code || !strings.Contains(out, c.want) {
				t.Errorf("exit %d, output %q; want exit %d containing %q", code, out, c.code, c.want)
			}
			// The fresh row is the one that can pass for the wrong
			// reason — a status that never says STALE would satisfy
			// "exit 0" here — so it asserts the word is absent too.
			if c.code == 0 && strings.Contains(out, "STALE") {
				t.Errorf("a clear instance said STALE: %q", out)
			}
		})
	}
}

// The sub-ruling CUT `sweep`, `init`, `drill` and `restore`, and this is
// that cut where an operator meets it: each one is refused, and the refusal
// names the whole surviving surface. A verb that came back would land here
// before it landed in a release note.
func TestCutBackupVerbsAreRefusedAtTheSurface(t *testing.T) {
	bin := buildRhq(t)
	home := backupHome(t, "runtime: claude\n")
	const usage = "posse backup [--to <dir>] | posse backup status | posse backup verify [--archive <path>]"
	for _, verb := range []string{"sweep", "init", "drill", "restore"} {
		code, out := runBackupPosse(t, bin, home, "backup", verb)
		if code != 1 || !strings.Contains(out, usage) {
			t.Errorf("posse backup %s: exit %d, output %q; want exit 1 naming the three surviving verbs", verb, code, out)
		}
	}
	// The control: the surviving verb is not refused by the same reading.
	// Without it, a `backup` that refused everything would pass above.
	if code, out := runBackupPosse(t, bin, home, "backup", "status"); code != 0 || strings.Contains(out, usage) {
		t.Errorf("posse backup status: exit %d, output %q; want the status the cut verbs are refused in favour of", code, out)
	}
}

// `backup_dest:` and `backup_keep_dest:` are CUT, and cut means unread: an
// instance carrying both must render exactly what the same instance without
// them renders. The control arm is what makes that comparison evidence —
// `backup_max_age:`, a key that IS read, moves the same output — so a
// status that had stopped reading config at all could not pass this.
func TestCutDestinationKeysChangeNothing(t *testing.T) {
	bin := buildRhq(t)
	const armed = "runtime: claude\nbackup_interval: 1h\n"
	// say runs one instance and takes its home OUT of the rendering: every
	// line names the archive directory, and three scratch homes would
	// differ by that alone — which would make the comparison below pass
	// whatever the code did, and the control arm pass with it.
	say := func(cfg string) string {
		home := backupHome(t, cfg)
		_, out := runBackupPosse(t, bin, home, "backup", "status")
		return strings.ReplaceAll(out, home, "<home>")
	}
	base := say(armed)
	withDest := say(armed + "backup_dest: /Volumes/NoSuchDisk\nbackup_keep_dest: 4\n")
	if withDest != base {
		t.Errorf("a destination key changed the surface:\n with: %q\nwithout: %q", withDest, base)
	}
	if strings.Contains(strings.ToLower(base), "sweep") || strings.Contains(base, "dest") {
		t.Errorf("status still speaks of a destination: %q", base)
	}
	withMaxAge := say(armed + "backup_max_age: 30m\n")
	if withMaxAge == base {
		t.Fatalf("the control did not move the output, so the comparison above measured nothing: %q", base)
	}
	if !strings.Contains(withMaxAge, "30m") {
		t.Errorf("backup_max_age: 30m is read and rendered, got %q", withMaxAge)
	}
}
