package posse

// ADR 0049 as the operator's 2026-09-05 simplification leaves it (bead
// ranger-base-gjbdl): a local backup does not read the source queue's git
// remotes, and no shape of them prevents a recovery copy. These pins were
// the source-remote REFUSAL's pins (build bead ranger-base-ymgbo); each
// rejection arm is now the successful local backup it should always have
// been, and the arms that were never about the source — the archive's scope
// and the target refusal — are kept as they were.
//
// WHY THE REFUSAL WENT, in the one sentence these pins have to hold: the
// check was an admission test on the SOURCE's config, and a local copy's
// effects do not depend on it. Backup makes no network call and invokes no
// `git push`; it never fenced another process from pushing to that remote
// either. So its refusals could only cost the backup, never a transfer —
// MEASURED, zero prevented transfers, one instance (ranger-base-8e31g)
// that could not archive at all. What still refuses is the TARGET, and
// TestSourceRemotesDoNotSoftenTheTargetRefusal is here so this deletion
// cannot be mistaken for that one.
//
// Every URL here is an example host. No real remote is spelled in this tree.

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	sanctionedRemote = "https://example.invalid/org/queue.git"
	otherRemote      = "ssh://git@example.invalid/org/elsewhere.git"
)

// remoteAt is the fixed clock these runs archive at. A function rather
// than a package var (backup_test.go's backupAt) so the tool behind `make
// verify-parallel` sees no shared state and these keep t.Parallel.
func remoteAt() time.Time { return time.Date(2026, 9, 2, 3, 15, 0, 0, time.UTC) }

// declaredRig is backupRig with the obsolete `queue_remote:` key written
// beside `queue_repo:`. Nothing reads it any more; the rig keeps it because
// an instance that set it in the interim still has the line, and "still has
// the line" must mean "backs up exactly as if it did not".
func declaredRig(t *testing.T, u string) (*App, string) {
	t.Helper()
	a, queue := backupRig(t)
	write(t, a.ConfigPath, "queue_repo: "+queue+"\nqueue_remote: "+u+"\n")
	return a, queue
}

// tarMembers reads every member of a published archive back, name → bytes.
func tarMembers(t *testing.T, archive string) map[string][]byte {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out[h.Name] = b
	}
	return out
}

// Every shape of source remote the deleted check had an opinion about, and
// one it did not, all producing a verified local archive. The six rows are
// the refusal's own arms read back the other way round: no remote (it
// passed), one sanctioned remote (it passed), a second remote, a different
// URL, a push URL elsewhere, and the two multi-URL escapes ranger-base-m6szh
// found — each of which used to end the run before a byte was staged.
//
// Each row STAGES its fixture rather than assuming it: `git remote -v` must
// really print what the row is named for, or a row that quietly added no
// remote would pass as "a remote does not prevent a backup" while proving
// nothing. The published archive is asserted to exist WITH its sidecar,
// because the verb publishes by rename only after reading the file back —
// the name on disk is the verified verdict (ADR 0036 §6).
func TestSourceRemoteShapesAllProduceAVerifiedArchive(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		setup   [][]string
		remotes int
	}{
		{"no remote at all", nil, 0},
		{"one remote", [][]string{
			{"remote", "add", "origin", sanctionedRemote},
		}, 1},
		{"two remotes", [][]string{
			{"remote", "add", "origin", sanctionedRemote},
			{"remote", "add", "mirror", sanctionedRemote},
		}, 2},
		{"a URL nobody declared", [][]string{
			{"remote", "add", "origin", otherRemote},
		}, 1},
		{"fetch here, push elsewhere", [][]string{
			{"remote", "add", "origin", sanctionedRemote},
			{"remote", "set-url", "--push", "origin", otherRemote},
		}, 1},
		{"a second fetch URL", [][]string{
			{"remote", "add", "origin", sanctionedRemote},
			{"remote", "set-url", "--add", "origin", otherRemote},
		}, 1},
		{"a second push URL", [][]string{
			{"remote", "add", "origin", sanctionedRemote},
			{"remote", "set-url", "--add", "--push", "origin", otherRemote},
		}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, queue := declaredRig(t, sanctionedRemote)
			for _, cmd := range c.setup {
				mustGit(t, queue, cmd...)
			}
			names := strings.Fields(mustGit(t, queue, "remote"))
			if len(names) != c.remotes {
				t.Fatalf("fixture premise: the queue has %d remote(s) %v, want %d", len(names), names, c.remotes)
			}
			res := mustBackup(t, a, remoteAt())
			if res.Archive == "" {
				t.Fatalf("no archive published: %+v", res)
			}
			if _, err := os.Stat(res.Archive); err != nil {
				t.Fatalf("published archive: %v", err)
			}
			if _, err := os.Stat(res.Sidecar); err != nil {
				t.Fatalf("published sidecar: %v", err)
			}
			if got := archives(t, a); len(got) != 1 {
				t.Fatalf("archives on disk: %v, want exactly the one just published", got)
			}
		})
	}
}

// The obsolete key arms nothing, refuses nothing and prints nothing. An
// instance that wrote `queue_remote:` during the interim still carries the
// line; this is what it now buys, which is nothing at all — the reading is
// unarmed, the backup runs, and no method on App answers for the key. The
// control arm is a real backup key, without which a BackupConfigured that
// always said false would pass this.
func TestObsoleteQueueRemoteKeyArmsAndRefusesNothing(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", otherRemote)
	if a.BackupConfigured() {
		t.Error("queue_remote: alone reads as a backup key")
	}
	if f := a.BackupFreshness(time.Now(), io.Discard); f.Armed {
		t.Errorf("queue_remote: alone armed the freshness reading: %+v", f)
	}
	// It refuses nothing either: the declared URL and the queue's actual
	// remote disagree, which is precisely the arm the deleted check died on.
	mustBackup(t, a, remoteAt())
	appendConfig(t, a, "backup_keep: 2\n")
	if !a.BackupConfigured() {
		t.Fatal("control: a backup key did not arm — the assertion above measured nothing")
	}
}

// The deletion is on the SOURCE and stops there. A queue carrying every
// remote shape at once, backed up to a target that is remote-shaped, still
// refuses — and the same instance with a local target archives. Without
// this pin, a later "the remote rules all went" reading of ADR 0049 has
// nothing standing in its way.
func TestSourceRemotesDoNotSoftenTheTargetRefusal(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", sanctionedRemote)
	mustGit(t, queue, "remote", "add", "mirror", otherRemote)
	for _, target := range []string{
		"https://example.invalid/archives",
		"backup.example.invalid:/srv/archives",
		"//host/share/archives",
	} {
		_, err := a.RunBackup(BackupOpts{Out: io.Discard, Dir: target})
		if err == nil {
			t.Fatalf("target %q was accepted", target)
		}
		if !strings.Contains(err.Error(), "remote") {
			t.Errorf("target %q refused without naming the remote target: %v", target, err)
		}
	}
	// The control: the same queue, the same remotes, the instance's own
	// local dir — which is what makes the three refusals above about the
	// target and not about the source this bead stopped reading.
	if got := archives(t, a); len(got) != 0 {
		t.Fatalf("a refused run published %v", got)
	}
	mustBackup(t, a, remoteAt())
}

// The archive's scope, unchanged by the deletion and asserted from the
// other side of it: the queue half is a bundle of history and refs, so no
// remote stanza and no `.git/config` ride along even though the queue now
// really carries remotes. The URL may appear in exactly one member —
// `home/config.yaml`, the operator's own promoted file, which a restore
// must bring back byte for byte including a key nothing reads. The manifest
// is the member a "record the source's remotes for provenance" change would
// reach for, and it is a member here so that reach is a red.
func TestSourceRemoteArchiveCarriesNoRemoteStanza(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", sanctionedRemote)
	mustGit(t, queue, "remote", "add", "mirror", otherRemote)
	// The queue's own git config holds both URLs: the premise that makes
	// "not under queue/" a finding rather than a tautology.
	if got := mustGit(t, queue, "config", "--get", "remote.origin.url"); got != sanctionedRemote {
		t.Fatalf("fixture premise: remote.origin.url = %q", got)
	}
	res := mustBackup(t, a, remoteAt())
	members := tarMembers(t, res.Archive)
	if len(members) == 0 {
		t.Fatal("the archive has no members")
	}
	sawConfig := false
	for name, body := range members {
		if strings.HasPrefix(name, "queue/") && strings.HasSuffix(name, ".git/config") {
			t.Errorf("%s is a member — the queue half carries git config", name)
		}
		has := strings.Contains(string(body), sanctionedRemote) || strings.Contains(string(body), otherRemote)
		switch {
		case name == "home/config.yaml":
			sawConfig = true
			if !has {
				t.Errorf("%s does not carry the operator's own line — the restore would come back without it", name)
			}
			if want, err := os.ReadFile(a.ConfigPath); err != nil {
				t.Fatal(err)
			} else if string(body) != string(want) {
				t.Errorf("%s is not the source file byte for byte:\n got %q\nwant %q", name, body, want)
			}
		case has:
			t.Errorf("%s carries a source remote URL — only home/config.yaml may", name)
		}
	}
	if !sawConfig {
		t.Fatalf("home/config.yaml is not a member; members: %v", memberNames(members))
	}
	if _, ok := members[backupManifestName]; !ok {
		t.Fatalf("%s is not a member; members: %v", backupManifestName, memberNames(members))
	}
	queueMembers := 0
	for name := range members {
		if strings.HasPrefix(name, "queue/") {
			queueMembers++
		}
	}
	if queueMembers == 0 {
		t.Fatal("no member under queue/ — the queue half was not archived, so 'no remote stanza under queue/' is vacuous")
	}
}

func memberNames(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
