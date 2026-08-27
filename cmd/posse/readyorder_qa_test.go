package main

// Adversarial pins for the ready queue's ORDER, written verifying
// ranger-base-xotg under ranger-base-hi2r.
//
// The fix landed three sorts — ReadyAll's, `posse ready`'s, and the
// dispatch pass's — and mutation testing found two of the three already
// pinned and one not: deleting `rhq.OrderBeads` from the `posse ready` arm
// (main.go, the line whose only unique job is the --dir path) left the
// whole suite green. These tests close that hole and pin the surface the
// bug was actually reported on: what an operator READS.
//
// Self-contained on purpose — its own fake bd, so it keeps compiling as
// main_test.go and cockpit_test.go grow fixtures of their own.

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
)

// orderBd answers `ready` from the repo's own fake-ready.json and
// `list --status in_progress` from fake-inprog.json, so a test can give each
// source its own order — bd's order is the query's, not a queue's.
const orderBd = `#!/bin/sh
cmd=""
for a in "$@"; do
  case "$a" in
    -*) ;;
    *) cmd=$a; break ;;
  esac
done
case "$cmd" in
  ready)
    if [ -f fake-ready.json ]; then cat fake-ready.json; exit 0; fi ;;
  list)
    for a in "$@"; do
      if [ "$a" = "in_progress" ] && [ -f fake-inprog.json ]; then cat fake-inprog.json; exit 0; fi
    done ;;
esac
echo '[]'
exit 0
`

var orderLine = regexp.MustCompile(`(?m)^(\S+)\s+p(\d)`)

// listedIDs is the id column of a `posse ready` listing, in printed order.
func listedIDs(out string) []string {
	var ids []string
	for _, m := range orderLine.FindAllStringSubmatch(out, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// oneSource is deliberately in bd's order and not a queue's: priorities out
// of order, and two P1s whose created_at decides which is first.
const oneSource = `[{"id":"d-p3","title":"third","priority":3,"created_at":"2026-01-01T00:00:00Z"},
  {"id":"d-p1-new","title":"newer p1","priority":1,"created_at":"2026-03-01T00:00:00Z"},
  {"id":"d-p4","title":"fourth","priority":4,"created_at":"2026-01-01T00:00:00Z"},
  {"id":"d-p2","title":"second","priority":2,"created_at":"2026-01-01T00:00:00Z"},
  {"id":"d-p1-old","title":"older p1","priority":1,"created_at":"2026-02-01T00:00:00Z"}]`

// ranger-base-xotg, the --dir arm: one repo is still a queue. ReadyAll is
// not on this path — `posse ready --dir` calls bd.Ready once and prints what
// comes back — so the sort in the command is the only thing between bd's
// query order and the operator's eye. Removing it left every other test in
// the repo green (measured under ranger-base-hi2r), which is what this pins.
func TestReadyDirOrdersOneSourceByPriority(t *testing.T) {
	bin := buildRhq(t)
	home, repo := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"),
		[]byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(oneSource), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := writeExec(t, t.TempDir(), "bd", orderBd)

	cmd := exec.Command(bin, "ready", "--dir", repo)
	cmd.Env = readyEnv(t, home, "RHQ_BD_BIN="+bd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("a readable repo must list: %v\n%s", err, out)
	}
	got := string(out)
	ids := listedIDs(got)
	// Priority first, then the oldest bead of a priority — waiting work is
	// not overtaken by work filed after it (OrderBeads' stated contract).
	want := []string{"d-p1-old", "d-p1-new", "d-p2", "d-p3", "d-p4"}
	if strings.Join(ids, " ") != strings.Join(want, " ") {
		t.Errorf("--dir must print a queue, not bd's query order:\nwant %v\ngot  %v\n%s", want, ids, got)
	}
}

// ranger-base-xotg on the surface it was reported from: the cockpit's READY
// WORK. The operator does not read ReadyAll, they read the rows — and the
// list travels ReadyAll -> readyOnly -> buildRows before it reaches them.
// Any of those three keeping sources apart, or re-grouping, brings the bug
// back with the sort still in place.
func TestCockpitReadyWorkRowsAreInQueueOrder(t *testing.T) {
	home := t.TempDir()
	first, second := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"),
		[]byte("beads:\n  - "+first+"\n  - "+second+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The reported shape: the second source holds a P1 and the first source
	// is full of lower-priority work. Each source hands back its own order.
	if err := os.WriteFile(filepath.Join(first, "fake-ready.json"), []byte(
		`[{"id":"one-p3","title":"first p3","priority":3,"created_at":"2026-01-01T00:00:00Z"},
		  {"id":"one-p2","title":"first p2","priority":2,"created_at":"2026-01-01T00:00:00Z"},
		  {"id":"one-p1","title":"first p1","priority":1,"created_at":"2026-01-01T00:00:00Z"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "fake-ready.json"), []byte(
		`[{"id":"two-p3","title":"second p3","priority":3,"created_at":"2026-01-02T00:00:00Z"},
		  {"id":"two-p1","title":"second p1","priority":1,"created_at":"2026-01-02T00:00:00Z"},
		  {"id":"two-held","title":"second p2, already claimed","priority":2,"status":"in_progress","created_at":"2026-01-02T00:00:00Z"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	// A held bead belongs to IN PROGRESS only (ADR 0004 §2). Dropping it out
	// of the middle must not disturb what is left.
	if err := os.WriteFile(filepath.Join(second, "fake-inprog.json"), []byte(
		`[{"id":"two-held","title":"second p2, already claimed","priority":2,"status":"in_progress","created_at":"2026-01-02T00:00:00Z"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	herdr := writeExec(t, binDir, "herdr", `#!/bin/sh
if [ "$1" = "workspace" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"workspaces":[]}}'
  exit 0
fi
if [ "$1" = "agent" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"agents":[]}}'
  exit 0
fi
printf '%s\n' '{"error":{"code":"no","message":"unexpected"}}'
exit 1
`)
	bd := writeExec(t, binDir, "bd", orderBd)

	a := &rhq.App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
	}
	c := &cockpit{
		app: a,
		hb:  &rhq.HerdrBackend{App: a, H: rhq.Herdr{Bin: herdr}, Warn: io.Discard},
		bd:  rhq.Bd{Bin: bd},
	}
	c.refresh()

	var ids []string
	for _, is := range c.issues {
		ids = append(ids, is.ID)
	}
	want := []string{"one-p1", "two-p1", "one-p2", "one-p3", "two-p3"}
	if strings.Join(ids, " ") != strings.Join(want, " ") {
		t.Errorf("READY WORK must be one queue across sources:\nwant %v\ngot  %v", want, ids)
	}
	// The reported regression, exactly: the second source's P1 ahead of the
	// first source's lower-priority beads, not behind them.
	at := func(id string) int {
		for i, g := range ids {
			if g == id {
				return i
			}
		}
		t.Fatalf("%s missing from READY WORK: %v", id, ids)
		return -1
	}
	if at("two-p1") > at("one-p2") || at("two-p1") > at("one-p3") {
		t.Errorf("a second-source P1 must outrank the first source's P2/P3: %v", ids)
	}
	// …and the rows the operator actually sees carry that same order.
	c.buildRows()
	var rowOrder []string
	for _, r := range c.rows {
		if r.kind != rowItem || r.sec != secIssues {
			continue
		}
		for _, cl := range r.cols {
			if id := strings.TrimSpace(qaPlain(cl.text)); strings.HasPrefix(id, "one-") || strings.HasPrefix(id, "two-") {
				rowOrder = append(rowOrder, id)
				break
			}
		}
	}
	if strings.Join(rowOrder, " ") != strings.Join(want, " ") {
		t.Errorf("the rendered rows must keep the queue order:\nwant %v\ngot  %v", want, rowOrder)
	}
}
