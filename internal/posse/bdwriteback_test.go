package posse

// ADR 0073 D3/B3: Create, Close, Comment, SetDescription, DepAdd and Unclaim
// each read their own write back now, rather than trusting bd's exit code
// and printed answer — the rig this ADR is built on measured 34-36 of 48
// concurrent no-db creates lost, exit 0 and a printed id every time. These
// pins are the landed / not-landed / unverified arm for each of the six,
// over the fake bd (herdr_test.go), whose "lost" and "-read-fail" markers
// exist for exactly this file.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFake(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func touchFake(t *testing.T, dir, name string) {
	t.Helper()
	writeFake(t, dir, name, "")
}

// ─── Create ──────────────────────────────────────────────────────────────────

func TestBdCreateLandedWhenShowAgrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	id, err := bd.Create(dir, BdNew{Title: "a real title"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("Create returned no id")
	}
	issue, err := bd.Show(dir, id)
	if err != nil || issue.Title != "a real title" {
		t.Fatalf("Show(%s) = %+v, %v — the create did not really land", id, issue, err)
	}
}

// TestBdCreateNotLandedWhenShowDisagrees models the rig's own shape: the id
// bd just printed resolves to a DIFFERENT title, because another writer's
// create clobbered it (ADR 0073, "t-15 printed by three writers"). The fake
// seeds a stale row at the id Create is about to be handed (q-1, the first
// id fakeBdNextID hands out in a fresh fake dir) BEFORE the real create
// appends its own row after it — fakeBdShowByID returns the first match in
// the list, so the stale row is what Show answers with.
func TestBdCreateNotLandedWhenShowDisagrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"q-1","title":"somebody else's bead","status":"open"}]`)

	id, err := bd.Create(dir, BdNew{Title: "mine"})
	if id != "q-1" {
		t.Fatalf("expected the fake's first id q-1, got %q", id)
	}
	if !IsBdWriteNotLanded(err) {
		t.Fatalf("expected BdWriteNotLandedError, got %v", err)
	}
	if !strings.Contains(err.Error(), "mine") || !strings.Contains(err.Error(), "somebody else's bead") {
		t.Errorf("error must name both what was wanted and what the store holds: %q", err.Error())
	}
}

func TestBdCreateUnverifiedWhenShowFails(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	touchFake(t, dir, "fake-show-fail")

	id, err := bd.Create(dir, BdNew{Title: "mine"})
	if id == "" {
		t.Fatal("Create must still report the id bd printed, even unverified")
	}
	if !IsBdWriteUnverified(err) {
		t.Fatalf("expected BdWriteUnverifiedError, got %v", err)
	}
}

// ─── Close ───────────────────────────────────────────────────────────────────

func TestBdCloseLandedWhenShowAgrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"a-1","title":"t","status":"open"}]`)

	if err := bd.Close(dir, "a-1", ""); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestBdCloseNotLandedWhenShowDisagrees is ADR 0073's own rig shape for this
// verb: `✓ Closed t-1: Closed`, exit 0, and the read-back found it still
// in_progress in 1 of 3 runs. fake-close-lost makes the fake print the same
// success without actually moving the row.
func TestBdCloseNotLandedWhenShowDisagrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"a-1","title":"t","status":"open"}]`)
	touchFake(t, dir, "fake-close-lost")

	err := bd.Close(dir, "a-1", "")
	if !IsBdWriteNotLanded(err) {
		t.Fatalf("expected BdWriteNotLandedError, got %v", err)
	}
	if !strings.Contains(err.Error(), "closed") || !strings.Contains(err.Error(), "open") {
		t.Errorf("error must name both the wanted and actual status: %q", err.Error())
	}
}

func TestBdCloseUnverifiedWhenShowFails(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"a-1","title":"t","status":"open"}]`)
	touchFake(t, dir, "fake-show-fail")

	err := bd.Close(dir, "a-1", "")
	if !IsBdWriteUnverified(err) {
		t.Fatalf("expected BdWriteUnverifiedError, got %v", err)
	}
}

// ─── Comment ─────────────────────────────────────────────────────────────────

func TestBdCommentLandedWhenReadBackAgrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := bd.Comment(dir, "a-1", "hello there", "ranger"); err != nil {
		t.Fatalf("Comment: %v", err)
	}
	cs, err := bd.Comments(dir, "a-1")
	if err != nil || len(cs) != 1 || cs[0].Text != "hello there" {
		t.Fatalf("Comments(a-1) = %+v, %v — the comment did not really land", cs, err)
	}
}

// TestBdCommentNotLandedWhenReadBackDisagrees is the storm's own shape: exit
// 0 on `comments add`, present afterwards in 17, 23, 12 of 48 (ADR 0073).
// fake-comment-lost prints the same success without appending the comment.
func TestBdCommentNotLandedWhenReadBackDisagrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	touchFake(t, dir, "fake-comment-lost")

	err := bd.Comment(dir, "a-1", "hello there", "ranger")
	if !IsBdWriteNotLanded(err) {
		t.Fatalf("expected BdWriteNotLandedError, got %v", err)
	}
}

// TestBdCommentNotLandedOnAuthorMismatch pins the match rule's second half:
// a comment with the right text but the wrong author is not a landed write
// of THIS call's — it is somebody else's comment that happens to agree on
// text, and a dedupe that cannot tell those apart over-counts.
func TestBdCommentNotLandedOnAuthorMismatch(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	// Seed a comment with the same text under a DIFFERENT author, then make
	// the add itself a no-op (fake-comment-lost) so the only candidate match
	// is the seeded one.
	writeFake(t, dir, "fake-comments.json",
		`[{"id":1,"issue_id":"a-1","text":"hello there","author":"someone-else","created_at":"2026-10-12T00:00:00Z"}]`)
	touchFake(t, dir, "fake-comment-lost")

	err := bd.Comment(dir, "a-1", "hello there", "ranger")
	if !IsBdWriteNotLanded(err) {
		t.Fatalf("expected BdWriteNotLandedError (author mismatch must not count as landed), got %v", err)
	}
}

func TestBdCommentUnverifiedWhenReadBackFails(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	touchFake(t, dir, "fake-comments-read-fail")

	err := bd.Comment(dir, "a-1", "hello there", "ranger")
	if !IsBdWriteUnverified(err) {
		t.Fatalf("expected BdWriteUnverifiedError, got %v", err)
	}
}

// ─── SetDescription ──────────────────────────────────────────────────────────

func TestBdSetDescriptionLandedWhenShowAgrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"a-1","title":"t","status":"open","description":"old"}]`)

	if err := bd.SetDescription(dir, "a-1", "new body", ""); err != nil {
		t.Fatalf("SetDescription: %v", err)
	}
	issue, err := bd.Show(dir, "a-1")
	if err != nil || issue.Description != "new body" {
		t.Fatalf("Show(a-1) = %+v, %v — the description did not really land", issue, err)
	}
}

func TestBdSetDescriptionNotLandedWhenShowDisagrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"a-1","title":"t","status":"open","description":"old"}]`)
	touchFake(t, dir, "fake-setdesc-lost")

	err := bd.SetDescription(dir, "a-1", "new body", "")
	if !IsBdWriteNotLanded(err) {
		t.Fatalf("expected BdWriteNotLandedError, got %v", err)
	}
}

func TestBdSetDescriptionUnverifiedWhenShowFails(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"a-1","title":"t","status":"open","description":"old"}]`)
	touchFake(t, dir, "fake-show-fail")

	err := bd.SetDescription(dir, "a-1", "new body", "")
	if !IsBdWriteUnverified(err) {
		t.Fatalf("expected BdWriteUnverifiedError, got %v", err)
	}
}

// ─── DepAdd ──────────────────────────────────────────────────────────────────

func TestBdDepAddLandedWhenGraphAgrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := bd.DepAdd(dir, "a-1", "b-1", ""); err != nil {
		t.Fatalf("DepAdd: %v", err)
	}
	deps, err := bd.DepList(dir, "a-1")
	if err != nil || len(deps) != 1 || deps[0].ID != "b-1" {
		t.Fatalf("DepList(a-1) = %+v, %v — the edge did not really land", deps, err)
	}
}

// TestBdDepAddNotLandedWhenGraphDisagrees is the muoo class already modeled
// for this verb (fake-dep-add-fail): exit 0 with nothing wrong on the wire
// and no edge in the graph.
func TestBdDepAddNotLandedWhenGraphDisagrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	touchFake(t, dir, "fake-dep-add-fail")

	err := bd.DepAdd(dir, "a-1", "b-1", "")
	if !IsBdWriteNotLanded(err) {
		t.Fatalf("expected BdWriteNotLandedError, got %v", err)
	}
}

func TestBdDepAddUnverifiedWhenGraphReadFails(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	touchFake(t, dir, "fake-dep-list-fail")

	err := bd.DepAdd(dir, "a-1", "b-1", "")
	if !IsBdWriteUnverified(err) {
		t.Fatalf("expected BdWriteUnverifiedError, got %v", err)
	}
}

// ─── Unclaim ─────────────────────────────────────────────────────────────────

func TestBdUnclaimLandedWhenShowAgrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := bd.Unclaim(dir, "a-1", "ranger", false); err != nil {
		t.Fatalf("Unclaim: %v", err)
	}
	issue, err := bd.Show(dir, "a-1")
	if err != nil || issue.Status != "open" {
		t.Fatalf("Show(a-1) = %+v, %v — the unclaim did not really land", issue, err)
	}
}

// TestBdUnclaimNotLandedWhenShowDisagrees pins the fallback order fake-show.json
// takes over Show (ADR 0073 B3's own gap, found building this): a fixture
// that pins an explicit `show` answer is never updated by `update --status
// open` (only fakeBdState and fake-list.json are), so a stale fake-show.json
// row is a faithful stand-in for the lost update itself.
func TestBdUnclaimNotLandedWhenShowDisagrees(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	writeFake(t, dir, "fake-show.json", `[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)

	err := bd.Unclaim(dir, "a-1", "ranger", false)
	if !IsBdWriteNotLanded(err) {
		t.Fatalf("expected BdWriteNotLandedError, got %v", err)
	}
}

func TestBdUnclaimUnverifiedWhenShowFails(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	touchFake(t, dir, "fake-show-fail")

	err := bd.Unclaim(dir, "a-1", "ranger", false)
	if !IsBdWriteUnverified(err) {
		t.Fatalf("expected BdWriteUnverifiedError, got %v", err)
	}
}
