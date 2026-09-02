// QA pin — ADR 0036's 2026-09-01 sub-ruling CUT `restore` on a single
// premise, in the sub-ruling table's own words: "`restore` is `tar -xzf`,
// which is the exit hatch the record already promised".
//
// That premise is a claim about the box's tar and about what falls out of a
// real published archive, and every other pin in this package reads archives
// with Go's archive/tar — the same encoder that wrote them, so they cannot
// catch a shape only an external reader would trip on. This one uses the
// box's own tar and then USES the result: the db it extracts must answer a
// query and the bundle it extracts must clone. Filed while verifying the
// close of ranger-base-x3de (ranger-base-37zah).
package posse

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQABackupExitHatchIsPlainTar(t *testing.T) {
	a, _ := backupRig(t)
	res := mustBackup(t, a, backupAt)

	out := t.TempDir()
	if b, err := exec.Command("tar", "-xzf", res.Archive, "-C", out).CombinedOutput(); err != nil {
		t.Fatalf("the exit hatch the sub-ruling cut `restore` for does not work: tar -xzf: %v\n%s", err, b)
	}

	var members []string
	if err := filepath.Walk(out, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			r, _ := filepath.Rel(out, p)
			members = append(members, r)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}

	var db, bundle string
	for _, m := range members {
		switch {
		case strings.HasSuffix(m, ".db"):
			db = filepath.Join(out, m)
		case strings.HasSuffix(m, ".bundle"):
			bundle = filepath.Join(out, m)
		}
	}
	if db == "" || bundle == "" {
		t.Fatalf("tar -xzf produced no db and/or no bundle, so it is not a restore; members: %v", members)
	}

	// The store of record actually comes back: the db answers for the bead
	// the rig seeded. Extracting bytes is not restoring a store.
	got, err := exec.Command("sqlite3", db, "select id from issues;").CombinedOutput()
	if err != nil || strings.TrimSpace(string(got)) != "x-1" {
		t.Fatalf("the extracted db does not hold the seeded bead: %q (err %v)", got, err)
	}
	// And the history: the bundle clones without the archive being unpacked
	// by anything but tar.
	clone := filepath.Join(t.TempDir(), "c")
	if b, err := exec.Command("git", "clone", "-q", bundle, clone).CombinedOutput(); err != nil {
		t.Fatalf("the extracted bundle does not clone, so the history did not survive: %v\n%s", err, b)
	}

	// Shown able to fail: the same pipeline over the same archive with one
	// byte flipped must not hand back a store that answers. The exit code is
	// deliberately NOT the assertion — this box's tar (bsdtar) prints
	// "Damaged tar archive / Retrying..." and still exits 0, so an operator
	// taking the sub-ruling's exit hatch on a corrupted archive is told
	// nothing by tar itself. What stands between them and a silent bad
	// restore is `posse backup verify` and the sha256 sidecar, not tar.
	// Without this arm every assertion above would read green over a tar
	// that tolerates anything, or over no tar at all.
	body, err := os.ReadFile(res.Archive)
	if err != nil {
		t.Fatal(err)
	}
	body[len(body)/2] ^= 0xff
	broken := filepath.Join(t.TempDir(), filepath.Base(res.Archive))
	if err := os.WriteFile(broken, body, 0o600); err != nil {
		t.Fatal(err)
	}
	into := t.TempDir()
	tarOut, tarErr := exec.Command("tar", "-xzf", broken, "-C", into).CombinedOutput()
	t.Logf("tar -xzf over a flipped byte: err=%v\n%s", tarErr, tarOut)
	brokenDB := filepath.Join(into, filepath.Base(filepath.Dir(db)), filepath.Base(db))
	q, qErr := exec.Command("sqlite3", brokenDB, "select id from issues;").CombinedOutput()
	if tarErr == nil && qErr == nil && strings.TrimSpace(string(q)) == "x-1" {
		t.Fatal("a flipped byte extracted into a fully working store — this pin measures nothing")
	}
}
