//go:build posse_arm3

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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
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

	// Shown able to fail: the same pipeline over an archive carrying one
	// flipped byte must not hand back a store that answers. The byte is
	// CHOSEN, not arbitrary, and that is the whole of ranger-base-ehllm.
	// This arm used to flip body[len(body)/2] of the .tar.gz, but gzip is a
	// stream: a flip corrupts what comes after it and nothing before, and
	// where the midpoint lands among the members moves with the pack bytes
	// of the bundle, which are not byte-stable. When it landed past
	// queue/beads.db the db extracted and answered, and the guard below —
	// whose whole job is to notice exactly that — fired: 9 runs in 100 on
	// this box, so about one full-package run in eleven went red on where
	// the deflate stream happened to fall and not on anything about posse.
	//
	// So the flip goes into the db member's own sqlite header, found by
	// walking the tar index, and the tar is re-gzipped around it. Every tar
	// header, size and checksum is untouched — tar does not checksum member
	// data — so what the exit hatch reads is a well-formed archive that
	// extracts whole at exit 0, carrying a db that is not a database. That
	// is the point of putting the byte here rather than in the gzip header:
	// the exit code is deliberately NOT the assertion. This box's tar
	// (bsdtar) prints "Damaged tar archive / Retrying..." and still exits 0
	// on a mangled stream, and over this archive it has nothing to print at
	// all. What stands between an operator taking the sub-ruling's exit
	// hatch and a silent bad restore is `posse backup verify` and the
	// sha256 sidecar, not tar. Without this arm every assertion above would
	// read green over a tar that tolerates anything, or over no tar at all.
	rel, err := filepath.Rel(out, db)
	if err != nil {
		t.Fatal(err)
	}
	raw := gunzipFile(t, res.Archive)
	off := tarDataOffset(t, raw, filepath.ToSlash(rel))
	if !bytes.HasPrefix(raw[off:], []byte("SQLite format 3\x00")) {
		t.Fatalf("offset %d is not where the %s member's bytes start, so the flip below would land somewhere unknown", off, rel)
	}
	raw[off] ^= 0xff
	broken := filepath.Join(t.TempDir(), filepath.Base(res.Archive))
	writeGzipFile(t, broken, raw)

	into := t.TempDir()
	tarOut, tarErr := exec.Command("tar", "-xzf", broken, "-C", into).CombinedOutput()
	if tarErr != nil {
		t.Fatalf("only one member byte moved, so the archive is still well formed and tar must extract it: %v\n%s", tarErr, tarOut)
	}
	brokenDB := filepath.Join(into, rel)
	q, qErr := exec.Command("sqlite3", brokenDB, "select id from issues;").CombinedOutput()
	if qErr == nil && strings.TrimSpace(string(q)) == "x-1" {
		t.Fatalf("a flipped byte in the db member extracted into a fully working store — this pin measures nothing: %q", q)
	}
	t.Logf("tar said %q at exit 0; sqlite3 over the extracted db said %q (err %v)",
		strings.TrimSpace(string(tarOut)), strings.TrimSpace(string(q)), qErr)
}

// gunzipFile is an archive's uncompressed tar bytes, and writeGzipFile is
// its inverse: together they let the arm above move one byte of a member
// without disturbing the tar around it.
func gunzipFile(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeGzipFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// tarDataOffset is where the named member's bytes start in an uncompressed
// tar stream. archive/tar reads its 512-byte blocks straight from the
// reader it was handed and buffers nothing ahead, so the count at the
// moment Next returns a header is that member's data offset. The caller
// checks what it finds there rather than trusting this.
func tarDataOffset(t *testing.T, raw []byte, name string) int64 {
	t.Helper()
	c := &countReader{r: bytes.NewReader(raw)}
	tr := tar.NewReader(c)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Name == name {
			return c.n
		}
	}
	t.Fatalf("no %s member in the archive", name)
	return 0
}

type countReader struct {
	r io.Reader
	n int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
