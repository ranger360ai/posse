package posse

// QA pin — "REFUSES any remote target ... the refusal is the design, not a
// flag" (the operator's 2026-09-01 sub-ruling on ranger-base-ay3dr,
// narrowing ADR 0036 §3; build bead ranger-base-a0ln0).
//
// Two things need pinning and they are different claims. That every remote
// SPELLING is refused is a table. That the refusal cannot be turned off is
// structural, and a table can never say it — so the last test here asserts
// the option surface itself.
//
// The volume arm is measured twice on purpose. This box mounts exactly one
// non-local filesystem (autofs at /System/Volumes/Data/home, MEASURED
// 2026-09-01), which is a real arm but a sample of one on one laptop; the
// fake supplies the nfs share and the unreadable volume that no developer's
// machine is going to have mounted when the suite runs.

import (
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// localVolume and remoteVolume are the two readings CheckBackupTarget can
// get from the kernel, as fakes.
func localVolume(string) (volume, error) {
	return volume{FSType: "apfs", Local: true, FreeBytes: 1 << 40}, nil
}

func remoteVolume(fs string) func(string) (volume, error) {
	return func(string) (volume, error) { return volume{FSType: fs, Local: false, FreeBytes: 1 << 40}, nil }
}

func unreadableVolume(string) (volume, error) { return volume{}, Die("statfs: permission denied") }

// Every spelling of "not on this box", and the reason each is refused for.
// The reason is asserted, not just the refusal: a target refused for the
// wrong reason is a refusal that will stop matching the day the shape it was
// actually caught by changes.
func TestBackupRefusesEveryRemoteSpelling(t *testing.T) {
	local := t.TempDir()
	for _, tc := range []struct {
		target string
		reason string
	}{
		{"https://backups.example.com/posse", "it is a URL"},
		{"s3://bucket/posse", "it is a URL"},
		{"smb://nas.local/backups", "it is a URL"},
		{"file:///Volumes/nas/backups", "it is a URL"},
		{"nas.local:/vol/backups", "it is an scp-style host:path"},
		{"backups@nas:/vol", "it is an scp-style host:path"},
		{"//nas/share/backups", "it is a UNC //host/share path"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			// Read with the LOCAL fake, so nothing here can pass by
			// accident of what this box happens to mount: if the shape
			// check did not catch it, the volume reading would say yes.
			err := checkBackupTarget(tc.target, localVolume)
			if err == nil {
				t.Fatalf("%s was accepted as an on-box path", tc.target)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("%s refused as %q, want the reason %q", tc.target, err, tc.reason)
			}
			if !strings.Contains(err.Error(), "ADR 0036") {
				t.Errorf("the refusal does not cite the ruling: %v", err)
			}
		})
	}
	// The control, and it is the arm that makes the table mean anything: a
	// real local directory is ACCEPTED. Without it every assertion above is
	// satisfied by a function that returns an error unconditionally.
	if err := checkBackupTarget(local, localVolume); err != nil {
		t.Fatalf("a local directory was refused (%v) — the table above measures nothing", err)
	}
}

// The volume arm: a path with no remote SHAPE at all, on a filesystem that
// is not this box's. Nothing in the string gives it away — only the kernel
// does — which is why the check does not stop at parsing.
func TestBackupRefusesAMountedRemoteVolume(t *testing.T) {
	dir := t.TempDir()
	for _, fs := range []string{"nfs", "smbfs", "afpfs", "webdav", "fuse"} {
		err := checkBackupTarget(dir, remoteVolume(fs))
		if err == nil {
			t.Fatalf("a %s volume was accepted", fs)
		}
		if !strings.Contains(err.Error(), fs) {
			t.Errorf("the %s refusal does not name the filesystem: %v", fs, err)
		}
	}
	// And a reading that could not be taken is refused too. This is the one
	// direction a guard usually gets wrong: the load guard fails OPEN when
	// it cannot read the load, because a guard that blocks on its own
	// blindness stops the shop. A refusal has the opposite duty — it is the
	// wall itself, and a wall that opens when it cannot see is not a wall.
	err := checkBackupTarget(dir, unreadableVolume)
	if err == nil {
		t.Fatal("a target whose volume could not be read was accepted — the refusal fails OPEN")
	}
	if !strings.Contains(err.Error(), "cannot certify") {
		t.Errorf("the unreadable-volume refusal reads %q", err)
	}
}

// The live arm, on the one non-local mount this box actually has. It is
// skipped rather than faked where it is absent: a test that invents the
// mount is the fake arm above, and this one exists to prove the real
// statfs reading and the fake agree.
func TestBackupRefusesTheBoxesOwnNonLocalMount(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the autofs mount this arm reads is a darwin path")
	}
	const autofs = "/System/Volumes/Data/home"
	v, err := volumeOf(autofs)
	if err != nil || v.Local {
		t.Skipf("%s is not a non-local mount here (local=%v, fs=%q, err=%v)", autofs, v.Local, v.FSType, err)
	}
	if err := CheckBackupTarget(autofs); err == nil {
		t.Fatalf("%s (%s, not local) was accepted as a backup target", autofs, v.FSType)
	}
	// The control on the real reader: the same exported function, no fake,
	// accepts a real local directory.
	if err := CheckBackupTarget(t.TempDir()); err != nil {
		t.Fatalf("the real reader refused a local temp dir: %v", err)
	}
}

// "The refusal is the design, not a flag." A table of spellings cannot say
// that — a single `--anywhere` flag would leave every row above green. So
// this pins the option surface: BackupOpts is the whole of what a caller may
// ask for, and a field added to lift the refusal reds here before it can
// ship.
func TestBackupHasNoOverride(t *testing.T) {
	var got []string
	rt := reflect.TypeOf(BackupOpts{})
	for i := 0; i < rt.NumField(); i++ {
		got = append(got, rt.Field(i).Name)
	}
	sort.Strings(got)
	if want := "Dir,Now,Out"; strings.Join(got, ",") != want {
		t.Errorf("BackupOpts fields = %v, want exactly %s — a new option on the one struct that reaches the refusal is a way around it, and there is no way around it (ADR 0036 §3, the 2026-09-01 sub-ruling)", got, want)
	}
	// Dir is the field a caller CAN set, so the refusal has to run on it —
	// not only on the configured default. A run pointed at a remote target
	// by argument is refused with the same words.
	a, _ := backupRig(t)
	_, err := a.RunBackup(BackupOpts{Dir: "nas.local:/vol/backups"})
	if err == nil || !strings.Contains(err.Error(), "is not an on-box path") {
		t.Fatalf("--to a remote target refused as %v, want the on-box refusal", err)
	}
}
