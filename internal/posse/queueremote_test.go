package posse

// ADR 0049 — the queue's sanctioned remote is an instance fact (build bead
// ranger-base-ymgbo). One pin per verification observable, numbered as the
// record numbers them; observable 1 widens the existing
// TestBackupRefusesAQueueRepoWithARemote (backup_test.go) and observable 8
// is TestBackupHasNoOverride (backuprefusal_qa_test.go), both unchanged in
// shape.
//
// Every URL here is an example host. No real remote is spelled in this
// tree: the work instance's line is the operator's, off it (0049 D7).

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

// declaredRig is backupRig with `queue_remote:` set to u beside `queue_repo:`.
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

// 2. Key = U, one remote with fetch and push both U → the archive is
// written. The remote is named `origin` on purpose: the name every clone
// mints, and the name the mutation ("compare against the remote NAME")
// would match instead of the place.
func TestQueueRemoteSanctionedRemoteIsBackedUp(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", sanctionedRemote)
	res := mustBackup(t, a, remoteAt())
	if res.Archive == "" {
		t.Fatalf("no archive published: %+v", res)
	}
	if _, err := os.Stat(res.Archive); err != nil {
		t.Fatalf("published archive: %v", err)
	}
}

// 3. Key = U, fetch URL V ≠ U → refused, and the line carries both, so the
// fix is a paste rather than a guess.
func TestQueueRemoteRefusesADifferentFetchURL(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", otherRemote)
	_, err := a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil {
		t.Fatal("a remote whose URL is not the declared one was accepted")
	}
	for _, want := range []string{sanctionedRemote, otherRemote, "queue_remote:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not carry %q", err, want)
		}
	}
	if got := archives(t, a); len(got) != 0 {
		t.Fatalf("a refused run published %v", got)
	}
}

// 4. Key = U, remote U plus a second remote → refused: the key sanctions
// ONE place. The second remote is a correct copy of U under another name,
// which is the arm a "first remote only" check passes.
func TestQueueRemoteRefusesASecondRemote(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", sanctionedRemote)
	mustGit(t, queue, "remote", "add", "mirror", sanctionedRemote)
	_, err := a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil {
		t.Fatal("two remotes were accepted under a key that sanctions one")
	}
	for _, want := range []string{"origin", "mirror", sanctionedRemote, "queue_remote:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not carry %q", err, want)
		}
	}
	// And it heals the way the single-remote refusal does: the second
	// remote gone, the same queue backs up.
	mustGit(t, queue, "remote", "remove", "mirror")
	mustBackup(t, a, remoteAt())
}

// 5. Key = U, fetch U and push V → refused. `git remote get-url --push`
// answers the pushurl when one is set, so a repo that fetches from the
// sanctioned place and pushes elsewhere is a repo with an off-box copy
// the ruling did not name.
func TestQueueRemoteRefusesAPushURLElsewhere(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", sanctionedRemote)
	mustGit(t, queue, "remote", "set-url", "--push", "origin", otherRemote)
	_, err := a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil {
		t.Fatal("a push URL pointing elsewhere was accepted")
	}
	for _, want := range []string{sanctionedRemote, otherRemote, "push"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not carry %q", err, want)
		}
	}
	// The control: the push URL restored to U, the same queue backs up,
	// which is what makes the refusal above about the push URL and not
	// about set-url having touched the repo.
	mustGit(t, queue, "remote", "set-url", "--push", "origin", sanctionedRemote)
	mustBackup(t, a, remoteAt())
}

// 6. Key = U, no remote → accepted: the key sanctions, it does not require.
func TestQueueRemoteDoesNotRequireTheRemote(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	if remotes := mustGit(t, queue, "remote"); strings.TrimSpace(remotes) != "" {
		t.Fatalf("fixture premise: the rig's queue has remotes %q", remotes)
	}
	mustBackup(t, a, remoteAt())
}

// 7. Config holding only `queue_remote:` → BackupConfigured() false and the
// freshness reading is unarmed: the key is not a backup key and starts no
// clock (0049 D3). The control arm is a real backup key, without which a
// BackupConfigured that always said false would pass.
func TestQueueRemoteIsNotABackupKey(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	write(t, a.ConfigPath, "queue_remote: "+sanctionedRemote+"\n")
	if a.BackupConfigured() {
		t.Error("queue_remote: alone reads as a backup key")
	}
	if f := a.BackupFreshness(time.Now(), io.Discard); f.Armed {
		t.Errorf("queue_remote: alone armed the freshness reading: %+v", f)
	}
	if a.BackupRemoteLine() != "  remote · "+sanctionedRemote+" (config queue_remote:) — the operator pushes; posse never does" {
		t.Errorf("declared posture line: %q", a.BackupRemoteLine())
	}
	write(t, a.ConfigPath, "backup_keep: 2\n")
	if !a.BackupConfigured() {
		t.Fatal("control: a backup key did not arm — the assertion above measured nothing")
	}
	if a.BackupRemoteLine() != "  remote · none declared (config queue_remote: unset) — any remote refuses" {
		t.Errorf("unset posture line: %q", a.BackupRemoteLine())
	}
	// `~` and `null` are the unset posture too (0049 D1): a YAML spelling
	// of nothing must not become a remote nobody declared.
	for _, none := range []string{"queue_remote:\n", "queue_remote: ~\n", "queue_remote: null\n", "queue_remote: \"\"\n"} {
		write(t, a.ConfigPath, none)
		if u := a.QueueRemote(); u != "" {
			t.Errorf("%q read as a declared remote %q", none, u)
		}
	}
}

// 9. An archive taken under posture 2, every member read back: no byte of
// U under queue/, and under home/ only in config.yaml (0049 D4). The
// bundle carries objects and refs, not config, so the queue's remote stanza
// never enters; the declaration itself rides in the promoted config.yaml,
// which is the operator's own line and the right thing for a restore to
// bring back. The manifest is the member a "record the remote for
// provenance" change would reach for, and it is a member here too: U in
// any member but home/config.yaml is the red.
func TestQueueRemoteArchiveCarriesTheURLOnlyInConfig(t *testing.T) {
	t.Parallel()
	a, queue := declaredRig(t, sanctionedRemote)
	mustGit(t, queue, "remote", "add", "origin", sanctionedRemote)
	// The queue's own git config holds the URL: the premise that makes
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
		has := strings.Contains(string(body), sanctionedRemote)
		switch {
		case name == "home/config.yaml":
			sawConfig = true
			if !has {
				t.Errorf("%s does not carry the declaration — the restore would come back without the sanction", name)
			}
		case has:
			t.Errorf("%s carries the declared URL — only home/config.yaml may", name)
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
		t.Fatal("no member under queue/ — the queue half was not archived, so 'no byte of U under queue/' is vacuous")
	}
}

func memberNames(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// checkQueueRemote alone, the seam the verb calls: the unset-key refusal
// names the way out, and a remote that fetches from U with no pushurl set
// passes on `get-url --push`'s fetch fallback (0049 D2, MEASURED).
func TestCheckQueueRemoteNamesTheWayOut(t *testing.T) {
	t.Parallel()
	_, queue := backupRig(t)
	if err := checkQueueRemote(queue, ""); err != nil {
		t.Fatalf("no remote, no key: %v", err)
	}
	if err := checkQueueRemote(queue, sanctionedRemote); err != nil {
		t.Fatalf("no remote, key set: %v", err)
	}
	mustGit(t, queue, "remote", "add", "origin", sanctionedRemote)
	err := checkQueueRemote(queue, "")
	if err == nil {
		t.Fatal("a remote under the unset key was accepted")
	}
	for _, want := range []string{"origin", "queue_remote:", "ranger-base-xhsb"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("unset-key refusal %q does not carry %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "ADR 0015") {
		t.Errorf("the refusal still cites ADR 0015, which states no such rule: %q", err)
	}
	if err := checkQueueRemote(queue, sanctionedRemote); err != nil {
		t.Fatalf("fetch U with no pushurl: %v", err)
	}
	if push := mustGit(t, queue, "remote", "get-url", "--push", "origin"); push != sanctionedRemote {
		t.Fatalf("this git does not fall back to the fetch URL for --push: %q", push)
	}
}
