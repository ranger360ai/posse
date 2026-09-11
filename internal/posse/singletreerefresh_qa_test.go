//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-qnn6j, M2 deviation 3 (operator, 2026-09-02): the SINGLE-TREE
// home, and every sentence posse printed at it.
//
// THE SHAPE. `posse init` seeds a home and stamps a `seeded` manifest, so
// ADR 0015 §3's launch verify is armed from the first launch. The operator
// then adds and renames PIDs IN PLACE — no constitution repo, no
// `constitution:` — and every launch warns DEGRADED about the crew they just
// wrote. Four shipped sentences told them what to do about it and all four
// named `posse promote`, which REFUSES on that home before it writes
// anything ("the same tree — nothing to promote", CmdPromote): there is no
// second tree to promote from, and `init` will not re-stamp a home it did
// not seed (ranger-base-h7cd, correctly). The only refresh that shape has is
// removing the manifest, which turns the verify OFF — and that IS the
// posture for a home nobody ratifies, because arming §3 is a ratification
// and this shape has no ratification step to re-run. Nothing said so
// anywhere: not the four sentences, not INSTALL.md §4, not §7.
//
// THE RULE THIS RESTORES is already written down at the two sites that keep
// it — LoadGuardEscape and constitutionLandRefusal both name `posse promote`
// only where it applies, because "prescribing a command that would do
// nothing about what was just read is what teaches people to skim refusals"
// (ranger-base-6s00n). The launch verify's own four sentences were outside
// that rule; SingleTreeRefresh puts them inside it.
//
// FOUR SITES, and the bead named two of them. The two it named are `init`'s
// fresh-stamp line and the interactive DEGRADED warning. The other two are
// the sibling arms of those same two if-blocks — `init`'s closing verify and
// the DISPATCH refusal — which carry the identical false remedy to a reader
// who is in more trouble, not less: their launches are already refusing. A
// fix that left those two saying `posse promote` and nothing else would be a
// fix an operator meets from the wrong side.
//
// The controls matter as much as the arms: a home `posse promote` CAN serve
// must never be told to delete its manifest, which would disarm a check that
// works. Both halves of the gate get a control below.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// singleTreeHome is the shape: a seeded home with no `constitution:` and a
// PID the test can then edit in place, the way the operator did.
func singleTreeHome(t *testing.T, a *App) string {
	t.Helper()
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: /tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pid := filepath.Join(a.AgentsDir, "ranger.md")
	if err := os.WriteFile(pid, []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The seeded stamp `posse init` leaves, and the whole reason the verify
	// is armed on a home nobody ratified.
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("the fixture does not start verifiable: %s", v.Line())
	}
	return pid
}

// editInPlace is the operator's move — the one this whole shape is about.
func editInPlace(t *testing.T, pid string) {
	t.Helper()
	body, _ := os.ReadFile(pid)
	if err := os.WriteFile(pid, append(body, []byte("\ncage: shims\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The gate itself, both ways round. Two facts, and either one alone leaves a
// home `posse promote` can still serve.
func TestSingleTreeRefreshIsNamedOnlyWhereThereIsNoPromote(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name         string
		seeded       bool
		constitution string
		want         bool
	}{
		{"seeded, no constitution: the shape", true, "", true},
		{"seeded, but a constitution names a tree to promote from", true, "~/src/rhq-constitution", false},
		{"promoted: the manifest is a claim about a commit", false, "", false},
		{"promoted and configured", false, "~/src/rhq-constitution", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := initTestApp(t)
			if err := os.MkdirAll(a.Home, 0o755); err != nil {
				t.Fatal(err)
			}
			cfg := ""
			if c.constitution != "" {
				cfg = "constitution: " + c.constitution + "\n"
			}
			if err := os.WriteFile(a.ConfigPath, []byte(cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			got := a.SingleTreeRefresh(c.seeded)
			if (got != "") != c.want {
				t.Errorf("SingleTreeRefresh(seeded=%v) with constitution:%q = %q, want named=%v",
					c.seeded, c.constitution, got, c.want)
			}
			if c.want && !strings.Contains(got, PromoteManifestFile) {
				t.Errorf("the sentence does not name the file it is about: %q", got)
			}
		})
	}
}

// SITE 1 — `init`'s fresh stamp, the sentence the operator read first. It is
// the arm that ARMS the verify, so it is also the only place the difference
// can be said before anything has gone wrong.
func TestInitsFreshStampNamesTheRefreshASingleTreeHomeActuallyHas(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v\n%s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "stamped") || !strings.Contains(got, "posse promote") {
		t.Fatalf("this is not the fresh-stamp arm any more; the pin below is measuring the wrong output:\n%s", got)
	}
	if !strings.Contains(got, SingleTreeRefreshFile) {
		t.Errorf("init armed the launch verify and named only `posse promote`, which a single-tree home cannot run:\n%s", got)
	}
}

// …and its control: a home whose config names a constitution is a home
// promote serves, and telling that operator to delete their manifest would
// disarm a check that works.
func TestInitsFreshStampSaysNothingAboutRemovingAManifestPromoteCanRestamp(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	if err := os.MkdirAll(a.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("constitution: ~/src/rhq-constitution\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), SingleTreeRefreshFile) {
		t.Errorf("init told a home with a constitution to remove its manifest:\n%s", out.String())
	}
}

// SITE 2 — `init`'s closing verify, read by an operator whose dispatched
// launches are already refusing. This is the re-run case: a home already
// seeded, edited since, `init` typed again because §7 advertises it.
func TestInitsClosingVerifyNamesTheRefreshASingleTreeHomeActuallyHas(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var first strings.Builder
	if err := a.initFrom(&first, posse.Seed, "embedded"); err != nil {
		t.Fatalf("the first init: %v\n%s", err, first.String())
	}
	// The operator's own edits, in place: a PID written straight into
	// agents/ (which is what §7 tells them to do) and a routine config
	// change. Unpromoted and changed, the two classes the verify reports.
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"), []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: /tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); v.OK() {
		t.Fatal("the in-place edit did not drift the home; this pin would pass on a clean verify")
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("the re-run: %v\n%s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "every dispatched launch will refuse") {
		t.Fatalf("this is not the closing-verify arm any more:\n%s", got)
	}
	if !strings.Contains(got, SingleTreeRefreshFile) {
		t.Errorf("init told a single-tree home its launches will refuse until it runs the one command that refuses:\n%s", got)
	}
}

// SITE 3 — the dispatch refusal. Nobody is watching this launch, so the
// sentence is read later, out of a log, by someone deciding what to type.
func TestDispatchRefusalOnASingleTreeHomeNamesTheRefreshItActuallyHas(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	pid := singleTreeHome(t, b.App)
	dir := t.TempDir()

	// Clean first, so a refusal that always fires is not mistaken for one
	// that read the manifest.
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "x-1"}); err != nil {
		t.Fatalf("control: a matching constitution refused a dispatch: %v", err)
	}
	editInPlace(t, pid)

	_, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger", Bead: "x-1"})
	if err == nil {
		t.Fatal("dispatch launched on a home that does not match its manifest")
	}
	if !strings.Contains(err.Error(), "posse promote") {
		t.Fatalf("this is not the §3 refusal any more: %v", err)
	}
	if !strings.Contains(err.Error(), SingleTreeRefreshFile) {
		t.Errorf("the refusal prescribes only a command this home cannot run: %v", err)
	}
}

// SITE 4 — the interactive DEGRADED warning, the one the bead was filed off.
func TestInteractiveDegradedOnASingleTreeHomeNamesTheRefreshItActuallyHas(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	pid := singleTreeHome(t, b.App)
	var warn bytes.Buffer
	b.Warn = &warn
	editInPlace(t, pid)

	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("an interactive launch was refused: %v", err)
	}
	got := warn.String()
	if !strings.Contains(got, "DEGRADED") || !strings.Contains(got, "posse promote") {
		t.Fatalf("this is not the §3 warning any more:\n%s", got)
	}
	if !strings.Contains(got, SingleTreeRefreshFile) {
		t.Errorf("the DEGRADED line prescribes only a command this home cannot run:\n%s", got)
	}
}

// The control for both launch sites, and the one that would matter most if
// the gate were ever loosened: a PROMOTED home is told to promote and
// nothing else. `rm promoted.json` there throws away a manifest that is a
// claim about a commit — a real check, disarmed on advice posse gave.
func TestAPromotedHomeIsNeverToldToDeleteItsManifest(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	pid := promotedTestHome(t, b)
	var warn bytes.Buffer
	b.Warn = &warn
	editInPlace(t, pid)

	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("an interactive launch was refused: %v", err)
	}
	if got := warn.String(); strings.Contains(got, PromoteManifestFile) {
		t.Errorf("the DEGRADED line on a promoted home names its manifest as a thing to remove:\n%s", got)
	}
	_, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: t.TempDir(), Agent: "ranger", Bead: "x-1"})
	if err == nil {
		t.Fatal("dispatch launched on a home that does not match its manifest")
	}
	if strings.Contains(err.Error(), PromoteManifestFile) {
		t.Errorf("the dispatch refusal on a promoted home names its manifest as a thing to remove: %v", err)
	}
}
