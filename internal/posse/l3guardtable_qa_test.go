//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-awt98, verifying ranger-base-e78c4: l3CommitGuards and the
// prepare-commit-msg render have to agree in BOTH directions, and until this
// file only one was pinned.
//
// TestL3CommitGuardBannersAreInTheRender walks the TABLE and asks the render
// for each banner, which catches a banner that moved — a guard reported lost on
// every stale slot on the box. The other direction is the one e78c4's own
// commentary names as the failure it was avoiding ("a derived line that omitted
// it would under-report exactly the way the static one over-reported", which is
// why NOTES.md was added as a fifth row), and nothing asked it. MEASURED
// 2026-10-03: adding a sixth unindented guard banner to CommitGuardHook with no
// table row left the whole package green, and the derived Degraded line simply
// never names that guard — the under-report, silently, for every stale body
// that lacks it.
//
// The shape is the three parts an absence rule needs. The SCANNER takes the
// render as an argument, so the same function grades a hand-typed fixture and
// the live render. The REGISTER (qaCommitGuardArms) is the accepted exceptions:
// an unindented banner that is a further ARM of a guard the table already
// carries, not a guard of its own. Its stale half is as loud as an
// unregistered banner — a register row that excuses nothing in either live
// render reds, so the register cannot decay into sentences about arms that
// left. And the POSITIVE CONTROL is a subtest that the scanner reports a
// hand-typed sixth guard, plus a floor on how many banners the live render was
// scanned for at all, because a scanner that read nothing reports clean.
//
// A config shape this fixture does not build may introduce a top-level banner
// of its own; this pin reds then, and the register is where it gets
// dispositioned. That is the pin working.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaCommitGuardArms is the register: unindented banners in the
// prepare-commit-msg render that are a further ARM of a guard l3CommitGuards
// already names, so l3GuardGap reporting the parent is the whole truth about
// them. Nothing else may be unaccounted for.
var qaCommitGuardArms = []string{
	"─── the data ceiling, second arm:",
	"─── the data ceiling, third arm:",
	// The ceiling's fourth block and its second `git diff` (ADR 0068 D1):
	// the same classes over `.beads/*.jsonl` alone, which REPORTS and
	// continues where the other three refuse. It is an arm of the ceiling
	// and not a guard of its own — it renders only when the ceiling does,
	// it reads the same configured list, and a stale body that lost it has
	// lost the ceiling with it, which is what l3GuardGap already reports.
	"─── the data ceiling, REPORTING reader:",
}

// qaScanCommitBanners grades one render: how many unindented banners it holds,
// which register rows those needed, and which of them no table row and no
// register row accounts for.
//
// Unindented only. Every guard l3GuardGap can name opens at column 0; the
// `check N:` banners under the beads visibility guard are indented, and they
// are not guards the Degraded line is about — a body that kept the parent
// banner and lost a check is the "this render's or an older one" case
// l3GuardGap already declines to claim currency over.
func qaScanCommitBanners(render string) (seen int, usedArm map[string]bool, unregistered []string) {
	usedArm = map[string]bool{}
	for _, ln := range strings.Split(render, "\n") {
		if !strings.HasPrefix(ln, "# ─── ") {
			continue
		}
		seen++
		accounted := false
		for _, g := range l3CommitGuards {
			if strings.Contains(ln, g.banner) {
				accounted = true
				break
			}
		}
		for _, arm := range qaCommitGuardArms {
			if accounted {
				break
			}
			if strings.Contains(ln, arm) {
				usedArm[arm], accounted = true, true
			}
		}
		if !accounted {
			unregistered = append(unregistered, ln)
		}
	}
	return seen, usedArm, unregistered
}

// THE SCANNER, SHOWN ABLE TO FAIL before it is pointed at the render.
func TestQACommitBannerScannerReportsAnUnregisteredGuard(t *testing.T) {
	t.Parallel()
	clean := "#!/bin/sh\n" +
		"# ─── the shared-index guard (rangerhq-lmq9) ───\n" +
		"# ─── the data ceiling, second arm: whatever follows ───\n" +
		"  # ─── check 0: the beads db (rangerhq-hrz) ───\n"
	seen, used, bad := qaScanCommitBanners(clean)
	if seen != 2 {
		t.Errorf("the scanner read %d unindented banners in a 3-banner fixture, want 2 (the indented one is not one)", seen)
	}
	if len(bad) != 0 {
		t.Errorf("a fixture of accounted banners reported %q", bad)
	}
	if !used["─── the data ceiling, second arm:"] {
		t.Error("the register row that excused a banner was not reported as used, so its stale half cannot be graded")
	}

	sixth := clean + "# ─── the sixth guard (ranger-base-awt98) ───────────────────────\n"
	if _, _, bad = qaScanCommitBanners(sixth); len(bad) != 1 || !strings.Contains(bad[0], "the sixth guard") {
		t.Errorf("a guard in the render with no table row and no register row must be reported, got %q", bad)
	}
}

// AND THE LIVE RENDER, both halves of the ceiling fork — the same two renders
// TestL3CommitGuardBannersAreInTheRender grades in the other direction.
func TestQAEveryCommitGuardBannerInTheRenderHasATableRow(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte(qaCeilingCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: cfg}
	set := a.OpsPatternSet()
	if len(set.Ceiling) == 0 {
		t.Fatalf("fixture premise: the ceiling patterns must be accepted, got %+v/%v", set.Ceiling, set.CeilingRejected)
	}

	// The floor per render, because a scanner that read nothing reports clean.
	// The ceiling fork is the reason the two numbers differ: with no
	// data_ceiling_patterns: configured dataCeilingCheck renders nothing, so
	// the ceiling banner and its two further arms are all absent — which is
	// the in-force property l3GuardGap rests on, graded from the other side by
	// TestL3CommitGuardBannersAreInTheRender.
	renders := []struct {
		what   string
		render string
		floor  int
	}{
		{"with a data ceiling configured", CommitGuardHook(VisibilityPublic, set), len(l3CommitGuards) + len(qaCommitGuardArms)},
		{"with no data ceiling", CommitGuardHook(VisibilityPublic, (&App{ConfigPath: filepath.Join(home, "absent.yaml")}).OpsPatternSet()), len(l3CommitGuards) - 1},
	}
	armUsed := map[string]bool{}
	for _, r := range renders {
		what := r.what
		seen, used, bad := qaScanCommitBanners(r.render)
		if seen < r.floor {
			t.Errorf("%s: the scan read %d unindented banners, under the %d this render carries — the scan did not reach the render", what, seen, r.floor)
		}
		for arm := range used {
			armUsed[arm] = true
		}
		for _, ln := range bad {
			t.Errorf("%s: the render carries a guard l3CommitGuards does not name, so l3GuardGap can never report it lost and a stale body silently keeps it:\n  %s\nAdd a row to l3CommitGuards, or a row to qaCommitGuardArms if it is a further arm of a guard already named.", what, ln)
		}
	}
	// The register's stale half, as loud as an unregistered banner.
	for _, arm := range qaCommitGuardArms {
		if !armUsed[arm] {
			t.Errorf("the register excuses %q and no render carries it: an exception that pardons nothing decays into a sentence about an arm that left — drop the row", arm)
		}
	}
}
