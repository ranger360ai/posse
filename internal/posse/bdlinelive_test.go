//go:build posse_arm3

package posse

// Live pins for ranger-base-292wj / ADR 0072 B1 — verification items 4 and
// 5 of the ADR, run against the real bd rather than the package's fake:
//
//	RHQ_LIVE_BD=1 go test -tags posse_arm3 ./internal/posse -run TestLiveBdLine -v
//
// Every offline arm (bdline_qa_test.go) drives a hand-written census
// directly or a fake bd that never actually hits the 64 KB scanner — the
// right place to pin ProjectBdWrite's own arithmetic and the D3/D4 wiring,
// but it leaves unpinned whether the real bd 0.50.3 still refuses a
// 65,537-byte line exactly where ADR 0072's probe measured it. So this
// shells out, like the other live pins (bdemptyunreadlive_test.go).
//
// Env-gated and skipped by default: it shells out to the operator's bd.
// Both rigs are no-db, committed, throwaway repos — never `bd init`, which
// is denied at every persona seat; the fixture writes the census by hand
// the way the ADR's own probe does.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// liveLineRig is a committed, no-db, three-row rig whose row 2 carries
// desc — ADR 0072's own probe shape (docs/adr/0072-bd-line-cliff.probe.sh),
// built with bdRow (bdline_qa_test.go) so the offline and live arms agree
// on what a census row looks like.
func liveLineRig(t *testing.T, row2Desc string) string {
	t.Helper()
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}
	repo := wtRepo(t)
	t.Cleanup(func() { stopLeakedDaemons(t, repo) })
	write(t, filepath.Join(repo, beadsDirName, "config.yaml"), "issue-prefix: \"t\"\nno-db: true\n")
	write(t, filepath.Join(repo, beadsDirName, beadsJSONL), bdRow("t-1", "small")+bdRow("t-2", row2Desc)+bdRow("t-3", "small too"))
	mustGit(t, repo, "add", "--", ".beads")
	mustGit(t, repo, "commit", "-q", "-m", "rig", "--", ".beads")
	return repo
}

// Verification item 5: Bd.Ready over a 70 KB row in a no-db rig returns
// BdStoreLineTooLongError naming line 2, t-2 and the row's real byte count
// — read from bd's own failure, never matched against its wording
// (ADR 0056, ADR 0072 D4).
func TestLiveBdLineReadyRefusesAnOversizedLine(t *testing.T) {
	t.Parallel()
	desc := strings.Repeat("x", 70000)
	repo := liveLineRig(t, desc)
	wantLine := bdRow("t-2", desc)

	_, err := Bd{Bin: "bd"}.Ready(repo, "")
	if err == nil {
		t.Fatal("want a refusal over the 70 KB row, got nil — re-measure docs/adr/0072-bd-line-cliff.probe.sh before trusting this pin")
	}
	var le *BdStoreLineTooLongError
	if !errors.As(err, &le) {
		t.Fatalf("want a typed BdStoreLineTooLongError, got %T: %v", err, err)
	}
	if !IsBdStoreLineTooLong(err) {
		t.Error("IsBdStoreLineTooLong must agree with errors.As")
	}
	if len(le.Lines) != 1 {
		t.Fatalf("Lines = %+v, want exactly the one oversized row", le.Lines)
	}
	got := le.Lines[0]
	if got.Line != 2 || got.ID != "t-2" || got.Bytes != len(wantLine) {
		t.Errorf("got %+v, want Line=2 ID=t-2 Bytes=%d", got, len(wantLine))
	}
	if !strings.Contains(err.Error(), "ADR 0072") || !strings.Contains(err.Error(), "t-2") {
		t.Errorf("the error must cite ADR 0072 and name the row: %v", err)
	}
}

// Verification item 4: Bd.Comment refuses, in Go, a write that would cross
// the cliff — before bd ever runs, and leaving the census untouched. The
// row is sized like item 2's no-db rig (60 KB), just under
// BdLineLimit-BdLineHeadroom, so bd's own write would still have succeeded
// had posse not judged it first.
func TestLiveBdLineCommentRefusesACrossingWrite(t *testing.T) {
	t.Parallel()
	desc := strings.Repeat("x", BdLineLimit-BdLineHeadroom-200)
	repo := liveLineRig(t, desc)
	before, err := os.ReadFile(filepath.Join(repo, beadsDirName, beadsJSONL))
	if err != nil {
		t.Fatal(err)
	}

	cerr := Bd{Bin: "bd"}.Comment(repo, "t-2", "a short comment that still tips it over", "tester")
	if cerr == nil {
		t.Fatal("want a refusal, got nil — re-measure before trusting this pin")
	}
	var le *BdLineWouldCrossError
	if !errors.As(cerr, &le) {
		t.Fatalf("want a typed BdLineWouldCrossError, got %T: %v", cerr, cerr)
	}
	if !IsBdLineWouldCross(cerr) {
		t.Error("IsBdLineWouldCross must agree with errors.As")
	}
	after, err := os.ReadFile(filepath.Join(repo, beadsDirName, beadsJSONL))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("a refused write must leave the census byte-for-byte unchanged — bd must never have run")
	}
}
