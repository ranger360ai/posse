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

// The remote posture line is GONE from `posse backup status` (ADR 0049 as
// the operator's 2026-09-05 ruling simplifies it, ranger-base-gjbdl). It
// used to print what the instance declared as its queue's sanctioned
// remote; with nothing reading `queue_remote:` any more, a line about it
// would tell an operator that a key still sanctions or refuses something,
// which is the one thing the simplification says not to do.
//
// Both postures are run because the line had two spellings and either one
// coming back is the red. The instance still SAYS things — the freshness
// line, the schedule line, the unarmed line — asserted here so that "the
// remote line is absent" is a finding about that line and not about a
// status verb that printed nothing at all.
func TestBackupStatusPrintsNoRemotePosture(t *testing.T) {
	bin := buildRhq(t)
	const u = "https://example.invalid/org/queue.git"
	for _, c := range []struct{ name, cfg string }{
		{"no key at all", "runtime: claude\n"},
		{"the obsolete key, still in config", "runtime: claude\nqueue_remote: " + u + "\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, out := runBackupPosse(t, bin, backupHome(t, c.cfg), "backup", "status")
			if code != 0 {
				t.Errorf("exit %d, output %q; want exit 0", code, out)
			}
			for _, absent := range []string{"queue_remote", "remote · ", u} {
				if strings.Contains(out, absent) {
					t.Errorf("output %q still carries the remote posture %q", out, absent)
				}
			}
			// The control: the verb did report, so the absences above are
			// about the deleted line and not about a silent status.
			for _, want := range []string{"backup ·", "nothing is armed"} {
				if !strings.Contains(out, want) {
					t.Errorf("output %q; want it to carry %q", out, want)
				}
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

// plantArchive writes one archive into a scratch instance's directory and
// hands back its NAME, which is the whole of what every reader here dates it
// by. A negative ago is a stamp in the future, which is the state this pin
// exists for; the byte of content is deliberate — `backup verify` opening
// the file and refusing it as gzip is the proof that it CHOSE this one.
func plantArchive(t *testing.T, home string, ago time.Duration) string {
	t.Helper()
	name := "posse-backup-" + time.Now().UTC().Add(-ago).Format("20060102T150405Z") + ".tar.gz"
	if err := os.WriteFile(filepath.Join(home, "state", "backup", name), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

// `posse backup verify` with no --archive, filling the gap ranger-base-wxvd5
// measured while verifying the close of ranger-base-rgv61: the fix taught
// the verb that a stamp ahead of the clock is not a reading (cmd/posse
// main.go, `if f.Future > 0`), the close measured it CLI to CLI by hand, and
// nothing in the tree held it — the whole branch could be deleted, or its
// condition inverted, with `go test ./...` green. Everything else about a
// future stamp is pinned inside the package (internal/posse/
// backupfuture_test.go); this is that fix where an operator meets it.
//
// The three rows are the states the verb can be handed, and the third is the
// control arm without which the other two pass for the wrong reason: a
// `verify` that refused EVERY directory would satisfy both refusals.
func TestBackupVerifyNamesWhyItCannotPickAnArchive(t *testing.T) {
	bin := buildRhq(t)
	const armed = "runtime: claude\nbackup_interval: 1h\n"

	t.Run("an empty directory has no archive at all", func(t *testing.T) {
		code, out := runBackupPosse(t, bin, backupHome(t, armed), "backup", "verify")
		if code != 1 || !strings.Contains(out, "no archive to verify") {
			t.Errorf("exit %d, output %q; want exit 1 saying there is no archive to verify", code, out)
		}
		// The empty directory must not borrow the future-stamp wording:
		// it has no files, so there is nothing to be undatable about.
		if strings.Contains(out, "no datable archive") {
			t.Errorf("an empty directory answered with the undatable wording: %q", out)
		}
	})

	t.Run("a directory of future stamps has files but no reading", func(t *testing.T) {
		home := backupHome(t, armed)
		future := plantArchive(t, home, -72*time.Hour)
		code, out := runBackupPosse(t, bin, home, "backup", "verify")
		for _, want := range []string{"no datable archive to verify", future, "AHEAD of this box's clock", "name one with --archive"} {
			if !strings.Contains(out, want) {
				t.Errorf("output %q; want it to contain %q", out, want)
			}
		}
		if code != 1 {
			t.Errorf("exit %d; want 1", code)
		}
		// "no archive to verify" would be the second lie next to a
		// directory that is not empty — and it is the arm this branch
		// falls through to when the fix is removed.
		if strings.Contains(out, "no archive to verify in") {
			t.Errorf("a directory holding %s was reported as empty: %q", future, out)
		}
	})

	t.Run("control: it follows through to the newest archive it CAN date", func(t *testing.T) {
		home := backupHome(t, armed)
		future := plantArchive(t, home, -72*time.Hour)
		datable := plantArchive(t, home, 3*time.Hour)
		_, out := runBackupPosse(t, bin, home, "backup", "verify")
		if !strings.Contains(out, datable) {
			t.Errorf("output %q; want it to open %s, the newest archive it can date", out, datable)
		}
		if strings.Contains(out, future) || strings.Contains(out, "no datable archive") {
			t.Errorf("a datable archive was on the box and the verb still refused: %q", out)
		}
	})
}
