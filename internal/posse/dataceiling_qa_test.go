//go:build !posse_arm2 && !posse_arm3

package posse

// THE DATA CEILING IS A SECOND PATTERN KEY THAT WALLS EVERY REPO THE
// INSTANCE HOOKS, WHATEVER ITS VISIBILITY STAMP (ADR 0050, ranger-base-nfg8l).
//
// Every check in the visibility wall is inside the stamp gate, so a repo
// stamped private runs none of them — by design for visibility, and a hole
// for an instance holding someone else's data, whose bead repo is private on
// purpose. The ceiling asks a different question of a staged line — may this
// exist in a local file at all? — so it renders ABOVE the gate, over the
// same three subjects check 3 scans inside it: ADDED lines of every staged
// file and ADDED staged paths through the renderer ranger-base-uzgkz built,
// plus every line of the commit MESSAGE (the ceiling's arm since ADR 0050
// D2 as amended 2026-09-03, ranger-base-pqlxr, built in ranger-base-o2v6n;
// check 3's since ranger-base-1nbtn, built in ranger-base-qk8i9) — always
// class-only, in its own words, first in order. Same subjects, different
// gate and different remedy: that is the whole difference between them.
//
// The fixture vocabulary ("QUOKKA", "quokka-export-") is a fixture's own,
// never this box's: what these pins measure is the mechanism.
//
// EVERY PIN HERE IS MUTATION-CHECKED, per alternative — a green pin over a
// wall that never had the hole measures nothing. The mutants and what each
// one reds are recorded at each pin; the runs are on ranger-base-nfg8l.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PIN (a): a PRIVATE-stamped repo refuses an added line that trips the
// ceiling, in a .go, a .md and a .beads/*.jsonl file — the three artifact
// classes ADR 0050 D5 names, and the repo the visibility wall stands down
// in. CONTROL: the same bytes with the same ERE under
// beads_visibility_patterns: ONLY commit clean in the private repo — the
// pre-0050 shape, which is what says this pin measured the ceiling's scope
// and not the pattern.
//
// MUTATION-CHECKED (runs on ranger-base-nfg8l):
//   - M1, the ceiling block rendered INSIDE the visibility gate (after the
//     `if`): every private-repo arm here goes red — the commit lands — and
//     the control stays green. PIN (c)'s public arm stays green, which is
//     why (a) and (c) are two pins.
//   - M4, the ceiling rendered with plain posse_check calls: this pin
//     stays green (a refusal is a refusal) and PIN (d) reds.
func TestQADataCeilingRefusesAddedLinesInAPrivateRepo(t *testing.T) {
	w := qaCeilingWall(t, "")
	arms := []struct{ rel, body string }{
		{"internal/posse/notes.go", "package posse\n\n// the " + qaCeilingHit + " export from tuesday\n"},
		{"docs/notes.d/handoff.md", "# handoff\n\n" + qaCeilingHit + " — do not forward\n"},
		{".beads/issues.jsonl", `{"id":"x-1","title":"triage","description":"pasted: ` + qaCeilingHit + ` header"}` + "\n"},
	}
	for _, arm := range arms {
		t.Run(arm.rel, func(t *testing.T) {
			w.stage(t, w.priv, arm.rel, arm.body)
			out, err := w.git(w.priv, w.persona, "commit", "-m", "x", "--", arm.rel)
			if err == nil {
				t.Fatalf("the ceiling must refuse an added line in a PRIVATE repo:\n%s", out)
			}
			for _, want := range []string{
				"refused by posse gate: data-ceiling content in a staged file",
				"ADR 0050 D2",
				qaCeilingClass + ": 1 hit(s)",
				"matched in the staged additions:",
				"remove the paste from the staged file and keep the cite",
				"stamped: " + VisibilityPrivate, // the footer names the stamp it ran under
				DataCeilingConfigKey + ":",      // where the operator changes it
				VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the refusal must carry %q:\n%s", want, out)
				}
			}
			// Its own words, not a visibility refusal's: the remedy a writer
			// is sent to has to be the right one, and "re-file it in the
			// private db" is the wrong door — this IS the private db.
			for _, never := range []string{"visibility class", "re-file the bead", "identity literal", "beads db is marked: public"} {
				if strings.Contains(out, never) {
					t.Errorf("a ceiling hit must not be refused in a visibility wall's words (%q):\n%s", never, out)
				}
			}
			if !strings.Contains(w.log(t), "data ceiling scan [prepare-commit-msg hook] (stamp: "+VisibilityPrivate+")") {
				t.Errorf("the refusal must be logged under the ceiling's own label, naming the stamp:\n%s", w.log(t))
			}
			w.unstage(t, w.priv, arm.rel)
		})
	}

	// THE CONTROL: same bytes, same ERE, same private repo — the ERE under
	// beads_visibility_patterns: alone. That wall is inside the stamp gate,
	// so a private repo takes every one of these (the pre-0050 shape).
	ctl := newVisWallCfg(t, "instance", OpsPatternsConfigKey+":\n  "+qaCeilingClass+": "+qaCeilingERE+"\n")
	if set := (&App{ConfigPath: ctl.home + "/config.yaml"}).OpsPatternSet(); len(set.Rejected) > 0 || len(set.Extra) != 1 || len(set.Ceiling) != 0 {
		t.Fatalf("control premise: the visibility pattern must be accepted and no ceiling configured, got %+v %v %+v", set.Extra, set.Rejected, set.Ceiling)
	}
	for _, arm := range arms {
		ctl.stage(t, ctl.priv, arm.rel, arm.body)
		if out, err := ctl.git(ctl.priv, ctl.persona, "commit", "-m", "x", "--", arm.rel); err != nil {
			t.Errorf("control: the same ERE as a VISIBILITY pattern must commit in a private repo (%s): %v\n%s", arm.rel, err, out)
		}
	}
	if strings.Contains(ctl.log(t), "data ceiling scan") {
		t.Errorf("control: no ceiling was configured, so nothing may be logged under its label:\n%s", ctl.log(t))
	}
}

// PIN (b): the same ceiling over the ADDED staged PATHS in the private
// repo — check 3's second arm (ranger-base-dmsbu), which the ceiling
// inherits whole. An export file-name shape is the posture's third class
// and it is a PATH, not content; a pure move yields no added lines at all.
//
// MUTATION-CHECKED: M2, dropping the path arm from the ceiling's render
// (an empty posse_ipaths listing) while leaving the content arm alone,
// reds both subtests here and leaves PIN (a) GREEN — two scans of two
// subjects, pinned separately.
func TestQADataCeilingRefusesAnAddedPathInAPrivateRepo(t *testing.T) {
	w := qaCeilingWall(t, "")

	t.Run("a new file whose PATH is export-shaped, content clean", func(t *testing.T) {
		const rel = "exports/" + qaExportStem + "20250101.csv"
		// The CONTENT is spotless — the content arm must not be what
		// refuses this, or the subtest is PIN (a) wearing a path.
		w.stage(t, w.priv, rel, "a,b\n1,2\n")
		out, err := w.git(w.priv, w.persona, "commit", "-m", "x", "--", rel)
		if err == nil {
			t.Fatalf("an export-shaped PATH must be refused in a private repo:\n%s", out)
		}
		for _, want := range []string{
			"refused by posse gate: data-ceiling content in a staged PATH",
			"the FILENAME, not its content",
			qaExportClass + ": 1 hit(s)",
			rel, // the subject, the one thing that says which file
			"stamped: " + VisibilityPrivate,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the path refusal must carry %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "matched: ") {
			t.Errorf("a ceiling entry must not write posse_check's matched-text line:\n%s", out)
		}
		if !strings.Contains(w.log(t), "data ceiling scan [prepare-commit-msg hook] (stamp: "+VisibilityPrivate+", staged path)") {
			t.Errorf("the refusal must be logged, naming the stamp and that it was a path:\n%s", w.log(t))
		}
		// The path is the ONLY place the vocabulary may appear — it is the
		// writer's own staged artifact — and refusals.log never carries it.
		qaNoCeilingVocabulary(t, "the terminal refusal", out, rel)
		qaNoCeilingVocabulary(t, "refusals.log", w.log(t))
		w.unstage(t, w.priv, rel)
	})

	t.Run("a pure move to an export-shaped path", func(t *testing.T) {
		// 40 byte-identical lines: git pairs it R100 and the diff has ZERO
		// plus lines, so the content arm cannot see it at all.
		w.plant(t, w.priv, "exports/report.csv", "a,b\n"+strings.Repeat("1,2\n", 40))
		const dst = "exports/" + qaExportStem + "7.csv"
		if out, err := w.git(w.priv, nil, "mv", "exports/report.csv", dst); err != nil {
			t.Fatalf("git mv: %v %s", err, out)
		}
		if ns, _ := w.git(w.priv, nil, "diff", "--cached", "--name-status", "HEAD"); !strings.HasPrefix(ns, "R") {
			t.Fatalf("fixture premise: git must report this move as a rename, got %q", ns)
		}
		if d, _ := w.git(w.priv, nil, "diff", "--cached", "-U0", "HEAD"); strings.Contains(d, "\n+") {
			t.Fatalf("fixture premise: a pure move must yield no added lines, got:\n%s", d)
		}
		out, err := w.git(w.priv, w.persona, "commit", "-m", "move", "--", "exports/report.csv", dst)
		if err == nil {
			t.Fatalf("a move to an export-shaped path must be refused:\n%s", out)
		}
		if !strings.Contains(out, "data-ceiling content in a staged PATH") || !strings.Contains(out, dst) {
			t.Errorf("the refusal must name the move's destination:\n%s", out)
		}
	})
}

// PIN (c): a PUBLIC-stamped repo. The ceiling still refuses there, and a
// line that trips BOTH the ceiling and this instance's visibility pattern
// is refused with the CEILING's header — the order pin. First in order
// because the stricter remedy has to be the one the writer reads: a
// visibility refusal says "re-file it in the private db", and for content
// above the ceiling there is no private db to re-file into.
//
// MUTATION-CHECKED:
//   - M1 (the block inside the gate) leaves the first subtest GREEN: a
//     public repo runs the gated checks. The order subtest reds only under
//     M3, the ceiling rendered AFTER check 3 inside the gate — the line is
//     still refused, by the wrong wall.
//   - M3 leaves PIN (a) red too (the block is now gated), so M3 and M1 are
//     told apart by THIS subtest alone.
func TestQADataCeilingRefusesInAPublicRepoAndFirst(t *testing.T) {
	w := qaCeilingWall(t, qaInstanceCfg)
	if set := (&App{ConfigPath: w.home + "/config.yaml"}).OpsPatternSet(); len(set.Extra) != 1 || set.Extra[0].Class != qaInstanceClass {
		t.Fatalf("fixture premise: the instance visibility pattern must be accepted beside the ceiling, got %+v %v", set.Extra, set.Rejected)
	}

	t.Run("the ceiling refuses under a public stamp", func(t *testing.T) {
		const rel = "internal/posse/notes.go"
		w.stage(t, w.pub, rel, "package posse\n\n// the "+qaCeilingHit+" export from tuesday\n")
		out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
		if err == nil {
			t.Fatalf("the ceiling must refuse in a PUBLIC repo too:\n%s", out)
		}
		if !strings.Contains(out, "data-ceiling content in a staged file") || !strings.Contains(out, "stamped: "+VisibilityPublic) {
			t.Errorf("the refusal must be the ceiling's and name the public stamp:\n%s", out)
		}
		if !strings.Contains(w.log(t), "data ceiling scan [prepare-commit-msg hook] (stamp: "+VisibilityPublic+")") {
			t.Errorf("the log line must name the stamp it ran under:\n%s", w.log(t))
		}
		w.unstage(t, w.pub, rel)
	})

	t.Run("a line that trips both lists is refused by the ceiling", func(t *testing.T) {
		const rel = "internal/posse/both.go"
		// One line, two classes: the instance's pre-publication name AND a
		// ceiling banner. Premise: each list matches it on its own.
		line := "// " + qaInstanceName + " shipped the " + qaCeilingHit + " export"
		set := (&App{ConfigPath: w.home + "/config.yaml"}).OpsPatternSet()
		if !set.Extra[0].Match(line) || !set.Ceiling[0].Match(line) {
			t.Fatalf("fixture premise: the line must trip both lists: instance=%v ceiling=%v", set.Extra[0].Match(line), set.Ceiling[0].Match(line))
		}
		w.stage(t, w.pub, rel, "package posse\n\n"+line+"\n")
		out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
		if err == nil {
			t.Fatalf("a line tripping both lists must be refused:\n%s", out)
		}
		first := strings.Index(out, "refused by posse gate: ")
		if first < 0 {
			t.Fatalf("no refusal header at all:\n%s", out)
		}
		header := out[first:]
		if i := strings.Index(header, "\n"); i >= 0 {
			header = header[:i]
		}
		if !strings.Contains(header, "data-ceiling content in a staged file") {
			t.Errorf("the FIRST refusal must be the ceiling's (the stricter remedy), got:\n\t%s\nfull:\n%s", header, out)
		}
		if strings.Contains(out, "an instance-defined visibility class") {
			t.Errorf("the visibility wall spoke before the ceiling — the writer is sent to the wrong door:\n%s", out)
		}
		if strings.Contains(w.log(t), instanceScanLabel) {
			t.Errorf("refusals.log must carry the ceiling's line, not the instance scan's:\n%s", w.log(t))
		}
	})
}

// PIN (d): stdout/stderr and refusals.log carry the class and a COUNTED hit
// number, and contain neither the ERE nor the matched text — a refusal is
// itself a local file, so class-only is the rule's own condition here, not
// a courtesy. The override passes, says so, and logs exactly one line that
// names the ceiling scan and the stamp.
//
// MUTATION-CHECKED:
//   - M4, the ceiling's posse_check calls rendered plain (classOnly=false):
//     reds this pin — `matched: QUOKKA RESTRICTED` appears — and no other.
//   - M5, the override branch dropped from the ceiling's refusal: the
//     override arm here reds (the commit is refused under the env).
//   - M6, the OVERRIDDEN log line rendered without the stamp tail: the
//     log assertion reds.
func TestQADataCeilingRefusalWithholdsTheValueAndOverrides(t *testing.T) {
	w := qaCeilingWall(t, "")
	const rel = "internal/posse/notes.go"
	// TWO matching lines: the count is what says the number is counted
	// rather than spelled.
	w.stage(t, w.priv, rel, "package posse\n\n// the "+qaCeilingHit+" export\n// and the "+qaCeilingWord+" INTERNAL memo\n")
	out, err := w.git(w.priv, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Fatalf("fixture premise: the ceiling must refuse this file:\n%s", out)
	}
	for _, want := range []string{qaCeilingClass + ": 2 hit(s)", "never the text it matched"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "matched: ") {
		t.Errorf("a ceiling entry must not write posse_check's matched-text line:\n%s", out)
	}
	qaNoCeilingVocabulary(t, "the terminal refusal", out)
	qaNoCeilingVocabulary(t, "refusals.log", w.log(t))

	// The operator's override passes, says so, and logs exactly one line —
	// under the ceiling's label, naming the stamp.
	before := strings.Count(w.log(t), dataCeilingScanLabel)
	out, err = w.git(w.priv, append(w.persona, VisibilityOverrideEnv+"="+VisibilityOverrideValue), "commit", "-m", "x", "--", rel)
	if err != nil || !strings.Contains(out, dataCeilingScanLabel+" OVERRIDDEN") {
		t.Fatalf("the operator's override must pass, and say so: %v\n%s", err, out)
	}
	log := w.log(t)
	if after := strings.Count(log, dataCeilingScanLabel); after != before+1 {
		t.Errorf("the override must log exactly one line, got %d new:\n%s", after-before, log)
	}
	if !strings.Contains(log, dataCeilingScanLabel+" OVERRIDDEN [prepare-commit-msg hook] (stamp: "+VisibilityPrivate+")") {
		t.Errorf("the OVERRIDDEN line must name the ceiling scan and the stamp it ran under:\n%s", log)
	}
	qaNoCeilingVocabulary(t, "refusals.log after the override", log)
}

// PIN (e): ONE class namespace across the shipped list and both keys. A
// class present in both keys is refused on the visibility side and NAMED —
// by install-hooks (WriteStampReport) and in the hook comment — and the
// ceiling classes are printed for a private-stamped repo, which until ADR
// 0050 printed only its stamp. Values never.
//
// MUTATION-CHECKED:
//   - M7, the namespace check reads only the shipped list (a class taken
//     by the ceiling is accepted again under the visibility key): the
//     refusal assertions here red, and the hook carries the class twice.
//   - M8, WriteStampReport prints the ceiling line only when Extra is
//     non-empty (the pre-0050 gating, copied): the report arm reds.
func TestQADataCeilingSharesOneClassNamespace(t *testing.T) {
	cfg := DataCeilingConfigKey + ":\n" +
		"  " + qaCeilingClass + ": " + qaCeilingERE + "\n" +
		"  cost: " + qaCeilingWord + "[0-9]+\n" + // taken by the shipped list
		OpsPatternsConfigKey + ":\n" +
		"  " + qaCeilingClass + ": " + qaCeilingWord + "-twin\n" + // taken by the ceiling
		"  codename: " + qaCeilingWord + "BIRD\n"
	w := newVisWallCfg(t, "instance", cfg)
	set := (&App{ConfigPath: w.home + "/config.yaml"}).OpsPatternSet()

	if got := opsClassNames(set.Ceiling); strings.Join(got, ",") != qaCeilingClass {
		t.Errorf("ceiling accepted %v, want [%s]", got, qaCeilingClass)
	}
	if got := opsClassNames(set.Extra); strings.Join(got, ",") != "codename" {
		t.Errorf("visibility accepted %v, want [codename]", got)
	}
	if len(set.CeilingRejected) != 1 || !strings.HasPrefix(set.CeilingRejected[0], "cost: ") || !strings.Contains(set.CeilingRejected[0], "the shipped list") {
		t.Errorf("a ceiling class taken by the shipped list must be refused and say by what: %v", set.CeilingRejected)
	}
	if len(set.Rejected) != 1 || !strings.HasPrefix(set.Rejected[0], qaCeilingClass+": ") || !strings.Contains(set.Rejected[0], DataCeilingConfigKey) {
		t.Errorf("a visibility class taken by the ceiling must be refused and name the ceiling key: %v", set.Rejected)
	}
	for _, r := range append(append([]string(nil), set.Rejected...), set.CeilingRejected...) {
		if strings.Contains(r, qaCeilingWord) {
			t.Errorf("a refusal echoed a value: %q", r)
		}
	}
	// All() is still the VISIBILITY list: the ceiling is not in it.
	for _, p := range set.All() {
		if p.Class == qaCeilingClass {
			t.Errorf("All() carries the ceiling class — a visibility reader would scan the ceiling under the wrong rule")
		}
	}

	// install-hooks' words, and they do not depend on the repo's stamp.
	var report bytes.Buffer
	set.WriteStampReport(&report)
	for _, want := range []string{
		"data ceiling stamped in (config " + DataCeilingConfigKey + ":), scanned under every stamp: " + qaCeilingClass,
		"data ceiling pattern REFUSED, not in force: cost: ",
		"instance pattern REFUSED, not in force: " + qaCeilingClass + ": ",
		"instance patterns stamped in (config " + OpsPatternsConfigKey + ":): codename",
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the stamp report must carry %q:\n%s", want, report.String())
		}
	}
	if strings.Contains(report.String(), qaCeilingWord) {
		t.Errorf("the stamp report echoed a value:\n%s", report.String())
	}
	// And the ceiling line does not ride on the instance list: a set with
	// a ceiling and NO visibility patterns still prints it. This arm is
	// what kills M8 — the fixture above has both keys, so a report gated
	// on the instance list survived it (measured on ranger-base-nfg8l).
	report.Reset()
	OpsPatternSet{Ceiling: set.Ceiling}.WriteStampReport(&report)
	if !strings.Contains(report.String(), "data ceiling stamped in (config "+DataCeilingConfigKey+":), scanned under every stamp: "+qaCeilingClass) ||
		strings.Contains(report.String(), "instance pattern") {
		t.Errorf("a ceiling-only set must report the ceiling line and nothing about instance patterns:\n%q", report.String())
	}

	// The PRIVATE repo's hook file is the record: the ceiling stamped in,
	// class-only, and both refusals named in the comment — by class.
	hook := qaHookFile(t, w.priv)
	for _, want := range []string{
		"posse_check '" + qaCeilingClass + "' '" + qaCeilingERE + "' " + opsClassOnlyArg,
		"# data ceiling patterns REFUSED at stamp time (config " + DataCeilingConfigKey + ":), not in force below:\n#   cost: ",
		"# instance patterns REFUSED at stamp time (config " + OpsPatternsConfigKey + ":), not in force below:\n#   " + qaCeilingClass + ": ",
	} {
		if !strings.Contains(hook, want) {
			t.Errorf("the private repo's hook must carry %q:\n%s", want, hook)
		}
	}
	if n := strings.Count(hook, "posse_check '"+qaCeilingClass+"'"); n != 3 {
		t.Errorf("the ceiling class must be stamped exactly three times (content arm, path arm, commit-message arm), got %d", n)
	}
	if strings.Contains(hook, qaCeilingWord+"-twin") || strings.Contains(hook, qaCeilingWord+"[0-9]+") {
		t.Error("the hook recorded a REFUSED entry's value — the class is the record")
	}
}

// PIN (f): the launcher's L3 identity probe and the install see ONE list
// (ADR 0050 D3). The probe re-renders the hook from the same OpsPatternSet
// and compares byte-for-byte (ADR 0023); if the ceiling rode a value one of
// the three renderers did not pass, every launch into a hooked repo would
// read "ours but stale". Both stamps, because the ceiling renders under
// both. The sibling pin on the public-repo instance pattern is in
// TestInstanceOpsPatternGuardsAPublicRepo, which now configures a ceiling
// too.
//
// MUTATION-CHECKED: M9, the L3 probe rendering with the ceiling cleared
// from its set copy, reds both stamps here and nothing in PIN (a)-(e).
func TestQADataCeilingIsSeenIdenticallyByTheL3Probe(t *testing.T) {
	w := qaCeilingWall(t, "")
	a := &App{ConfigPath: w.home + "/config.yaml"}
	for _, repo := range []string{w.pub, w.priv} {
		if p := a.probeL3Hooks(repo, false); !p.CommitGuard {
			t.Errorf("L3 must vouch for the hook it just stamped with a ceiling (%s): %s", filepath.Base(repo), p.CommitGuardDegraded)
		}
	}
	// And the THIRD arm rides that same set (ADR 0050 D2 as amended
	// 2026-09-03): if it did not, the probe's re-render would differ from
	// the installed file by exactly this block and every launch into a
	// hooked repo would read "ours but stale".
	if !strings.Contains(qaHookFile(t, w.priv), "third arm: the commit MESSAGE") {
		t.Error("the stamped hook does not carry the ceiling's commit-message arm")
	}
	// And the file IS the render, not merely accepted by it.
	vis, _ := a.BeadsVisibility(w.priv)
	if want := CommitGuardHook(vis, a.OpsPatternSet(), testIdentity(t, w.priv)...); qaHookFile(t, w.priv) != want {
		t.Error("the private repo's stamped hook is not byte-identical to CommitGuardHook over the same set")
	}
}

// The render itself, under both stamps and with no repo at all: the ceiling
// block sits ABOVE the visibility gate, AFTER the shared helpers it calls,
// and renders nothing when the list is empty.
//
// MUTATION-CHECKED: M1 reds the order assertion; M10, the helpers left
// inside the gate, reds the helper-order assertion (a private repo would
// call an undefined posse_check); an empty-list render carrying the block
// reds the last one.
func TestQADataCeilingRendersAboveTheGateUnderEveryStamp(t *testing.T) {
	t.Parallel() // renders and reads strings; no repo, no env
	set := OpsPatternSet{Ceiling: []OpsPattern{{Class: qaCeilingClass, ERE: qaCeilingERE}}}
	for _, vis := range []string{VisibilityPublic, VisibilityPrivate} {
		hook := CommitGuardHook(vis, set)
		gate := strings.Index(hook, `if [ "$posse_beads_visibility" = `+shQuote(VisibilityPublic)+` ]; then`)
		ceiling := strings.Index(hook, "posse_check '"+qaCeilingClass+"'")
		helper := strings.Index(hook, "posse_check() {")
		base := strings.Index(hook, "posse_base=$(git hash-object")
		stamp := strings.Index(hook, "posse_beads_visibility="+shQuote(vis))
		if gate < 0 || ceiling < 0 || helper < 0 || base < 0 || stamp < 0 {
			t.Fatalf("%s: a landmark is missing (gate=%d ceiling=%d helper=%d base=%d stamp=%d)", vis, gate, ceiling, helper, base, stamp)
		}
		if !(stamp < base && base < helper && helper < ceiling && ceiling < gate) {
			t.Errorf("%s: want stamp < base < posse_check() < ceiling < gate, got stamp=%d base=%d helper=%d ceiling=%d gate=%d", vis, stamp, base, helper, ceiling, gate)
		}
		// THREE ARMS, in one order (ranger-base-o2v6n): content, path,
		// message, and all three above the gate. The order is not cosmetic
		// — each arm sends the writer to a different remedy, and the one
		// that speaks is the one whose subject they have to fix.
		path := strings.Index(hook, "second arm: the same patterns over ADDED staged PATHS")
		message := strings.Index(hook, "third arm: the commit MESSAGE")
		if path < 0 || message < 0 {
			t.Fatalf("%s: an arm's banner is missing (path=%d message=%d)", vis, path, message)
		}
		if !(ceiling < path && path < message && message < gate) {
			t.Errorf("%s: want content < path < message < gate, got content=%d path=%d message=%d gate=%d", vis, ceiling, path, message, gate)
		}
		if msgArm := strings.Index(hook, `posse_msg=$(cat "$1"`); msgArm < message || msgArm > gate {
			t.Errorf("%s: the message arm must read $1 between its own banner and the gate, got %d (banner=%d gate=%d)", vis, msgArm, message, gate)
		}
		if strings.Count(hook, "posse_check '"+qaCeilingClass+"' '"+qaCeilingERE+"' "+opsClassOnlyArg) != 3 {
			t.Errorf("%s: the ceiling must render class-only at all three arms", vis)
		}
		// The head comment's COUNT is the assertion, not the word "Five":
		// it went to four when ADR 0051's citation arm was removed
		// (ranger-base-bp0yj), and a count that is not held against the
		// walls actually rendered is a number that drifts silently — which
		// is the whole reason this cell reads it at all.
		if !strings.Contains(hook, "─── the data ceiling (ADR 0050)") || !strings.Contains(hook, "Four walls") {
			t.Errorf("%s: the block's banner and the head comment's count must name the ceiling", vis)
		}
	}
	if hook := CommitGuardHook(VisibilityPrivate, OpsPatternSet{}); strings.Contains(hook, dataCeilingScanLabel) ||
		strings.Contains(hook, "─── the data ceiling") || strings.Contains(hook, "third arm: the commit MESSAGE") ||
		strings.Contains(hook, `posse_msg=$(cat "$1"`) {
		t.Error("an empty ceiling list must render no block at all — the message arm included: an instance with no ceiling pays for no read")
	}
}

// ADR 0050 D4: the in-session warn scans the ceiling regardless of the
// repo's stamp and says "ceiling", not "visibility"; the visibility arm
// stays public-only; both can speak, ceiling first.
//
// MUTATION-CHECKED: M11, the ceiling scan gated on a public stamp: the
// private arm reds. M12, the ceiling scanned through All() (the visibility
// list): the private arm reds — All() does not carry the ceiling — and so
// does the shared-namespace pin.
func TestQAWarnOpsContentSpeaksTheCeilingUnderEveryStamp(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	pub, priv := filepath.Join(home, "pub"), filepath.Join(home, "priv")
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+priv+": private\n  "+pub+": public\n"+qaCeilingCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}

	var w bytes.Buffer
	if !a.WarnOpsContent(&w, priv, "the bead", "pasted "+qaCeilingHit+" header") {
		t.Fatal("a ceiling hit in a PRIVATE repo must warn")
	}
	if !strings.HasPrefix(w.String(), "ceiling: ") || !strings.Contains(w.String(), qaCeilingClass) || strings.Contains(w.String(), "visibility") {
		t.Errorf("the warning must be worded as the ceiling and name the class: %q", w.String())
	}
	if strings.Contains(w.String(), qaCeilingHit) {
		t.Errorf("the warning echoed the matched text: %q", w.String())
	}

	w.Reset()
	if a.WarnOpsContent(&w, priv, "the bead", "spend was $715/wk") || w.Len() > 0 {
		t.Errorf("a shipped visibility class in a private repo is still nobody's business: %q", w.String())
	}

	w.Reset()
	if !a.WarnOpsContent(&w, pub, "the bead", "pasted "+qaCeilingHit+"; spend was $715/wk") {
		t.Fatal("a public repo with both must warn")
	}
	lines := strings.Split(strings.TrimSpace(w.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "ceiling: ") || !strings.HasPrefix(lines[1], "visibility: ") || !strings.Contains(lines[1], "cost") {
		t.Errorf("want the ceiling line then the visibility line, got:\n%s", w.String())
	}
}

// ─── the THIRD arm: the commit MESSAGE (ADR 0050 D2 as amended 2026-09-03,
// ranger-base-pqlxr, built in ranger-base-o2v6n) ──────────────────────────
//
// A commit message is none of D5's exclusions: it lands in the commit
// object and replicates with the branch, and it is the most-quoted artifact
// in this shop — a persona's message cites the context it worked from,
// which is the paste shape exactly. MEASURED on ranger-base-zikpp: a
// ceiling-matching MESSAGE committed clean while the same bytes in a staged
// file were refused. That measurement is kept below as the control.

// PIN (g): the ceiling refuses a commit MESSAGE carrying the vocabulary,
// under both of the forms that reach this hook with the message already in
// hand — `-m` and the crew's `-F -` — while the staged file is SPOTLESS, so
// nothing but the message can be what refused it. Class and hit count only;
// stdout, stderr and refusals.log carry neither the ERE nor the text, and
// the log line names the stamp and the subject.
//
// CONTROL, and it is ranger-base-zikpp's measurement kept as the failing
// wrong arm: the same message with the same ERE configured under
// beads_visibility_patterns: alone commits CLEAN in the private repo and
// logs nothing under the ceiling's label. That is the pre-o2v6n world, and
// it is what says this pin measured the third arm rather than the wall.
//
// MUTATION-CHECKED (go test -overlay, runs recorded on ranger-base-o2v6n):
//   - the message arm removed: this pin reds under both forms, the control
//     stays green.
//   - the arm rendered INSIDE the stamp gate: this pin reds (private repo),
//     and the public-stamp arm below stays green — two mutants, told apart.
//   - the matched text printed (classOnly dropped from the message arm):
//     the withholding assertions red and nothing else here does.
func TestQADataCeilingRefusesTheCommitMessage(t *testing.T) {
	w := qaCeilingWall(t, "")
	// The MESSAGE carries the vocabulary; the staged file never does.
	const clean = "package posse\n\n// nothing to see\n"
	msg := "wire the export\n\nfrom the " + qaCeilingHit + " banner on the source system\n"

	for i, form := range []string{"-m", "-F -"} {
		t.Run(form, func(t *testing.T) {
			rel := "internal/posse/msg" + string(rune('a'+i)) + ".go"
			out, err := qaMsgCommit(t, w, w.priv, rel, clean, form, msg, w.persona)
			if err == nil {
				t.Fatalf("the ceiling must refuse a commit MESSAGE carrying the vocabulary (%s):\n%s", form, out)
			}
			for _, want := range []string{
				"refused by posse gate: data-ceiling content in the commit MESSAGE",
				"ADR 0050 D2",
				qaCeilingClass + ": 1 hit(s)",
				"matched in the commit message:",
				"rewrite the commit message",
				".git/COMMIT_EDITMSG",
				"stamped: " + VisibilityPrivate,
				DataCeilingConfigKey + ":",
				VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the message refusal must carry %q:\n%s", want, out)
				}
			}
			// The staged file is clean, so the FILE arm must not be what
			// spoke — or this pin is PIN (a) wearing a message.
			for _, never := range []string{
				"data-ceiling content in a staged file",
				"data-ceiling content in a staged PATH",
				"remove the paste from the staged file",
				"matched: ",
			} {
				if strings.Contains(out, never) {
					t.Errorf("a MESSAGE hit must not be refused in the staged file arm's words (%q):\n%s", never, out)
				}
			}
			if !strings.Contains(w.log(t), "data ceiling scan [prepare-commit-msg hook] (stamp: "+VisibilityPrivate+", commit message)") {
				t.Errorf("the refusal must be logged under the ceiling's label, naming the stamp and the subject:\n%s", w.log(t))
			}
			qaNoCeilingVocabulary(t, "the terminal refusal", out)
			qaNoCeilingVocabulary(t, "refusals.log", w.log(t))
			w.unstage(t, w.priv, rel)
		})
	}

	// THE CONTROL (ranger-base-zikpp, measured before this arm existed):
	// the same ERE as a VISIBILITY pattern, the same message, the same
	// private repo — it commits, and the ceiling's label is never written.
	ctl := newVisWallCfg(t, "instance", OpsPatternsConfigKey+":\n  "+qaCeilingClass+": "+qaCeilingERE+"\n")
	if set := (&App{ConfigPath: ctl.home + "/config.yaml"}).OpsPatternSet(); len(set.Rejected) > 0 || len(set.Extra) != 1 || len(set.Ceiling) != 0 {
		t.Fatalf("control premise: the visibility pattern must be accepted and no ceiling configured, got %+v %v %+v", set.Extra, set.Rejected, set.Ceiling)
	}
	for i, form := range []string{"-m", "-F -"} {
		rel := "internal/posse/ctl" + string(rune('a'+i)) + ".go"
		if out, err := qaMsgCommit(t, ctl, ctl.priv, rel, clean, form, msg, ctl.persona); err != nil {
			t.Errorf("control: the same ERE as a VISIBILITY pattern must let the message through in a private repo (%s): %v\n%s", form, err, out)
		}
	}
	if strings.Contains(ctl.log(t), dataCeilingScanLabel) {
		t.Errorf("control: no ceiling was configured, so nothing may be logged under its label:\n%s", ctl.log(t))
	}
}

// PIN (h): a message REUSED after the ceiling was configured. The
// vocabulary is committed while the hook carries no ceiling at all, then
// the operator adds the key and re-runs install-hooks, and `git commit
// --amend` — which hands this hook HEAD's message, source "commit" — is
// refused. Two things at once: the arm scans a message it did not see
// typed, and it does not case on "$2".
//
// MUTATION-CHECKED: the arm rendered under `case "$2" in message)` — i.e.
// skipping source=commit — reds THIS pin alone; PIN (g) stays green because
// -m and -F - both arrive as source "message".
func TestQADataCeilingRefusesAReusedMessage(t *testing.T) {
	w := newVisWallCfg(t, "instance", "")
	a := &App{ConfigPath: w.home + "/config.yaml"}
	if set := a.OpsPatternSet(); len(set.Ceiling) != 0 {
		t.Fatalf("fixture premise: the first commit must run under a hook with NO ceiling, got %+v", set.Ceiling)
	}
	msg := "import the " + qaCeilingHit + " extract\n"
	if out, err := qaMsgCommit(t, w, w.priv, "internal/posse/a.go", "package posse\n", "-m", msg, w.persona); err != nil {
		t.Fatalf("fixture premise: with no ceiling configured this message must commit: %v\n%s", err, out)
	}

	// The operator adds the key and re-stamps the hook — the only way the
	// list ever reaches a repo (ADR 0050 D1/D3).
	write(t, w.home+"/config.yaml", "beads_visibility:\n  "+w.pub+": public\n  "+w.priv+": private\n"+qaCeilingCfg)
	if set := a.OpsPatternSet(); len(set.Ceiling) != 2 {
		t.Fatalf("fixture premise: the ceiling must be accepted after the rewrite, got %+v", set.Ceiling)
	}
	if _, _, _, err := a.InstallCommitGuardHook(w.priv); err != nil {
		t.Fatal(err)
	}

	// A NEW, clean staged file and --no-edit: nothing this commit writes
	// carries the vocabulary. The only subject that does is the message it
	// is reusing out of HEAD.
	w.stage(t, w.priv, "internal/posse/b.go", "package posse\n\n// clean\n")
	out, err := w.git(w.priv, w.persona, "commit", "--amend", "--no-edit", "--", "internal/posse/b.go")
	if err == nil {
		t.Fatalf("a REUSED message must be scanned by the ceiling:\n%s", out)
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") || !strings.Contains(out, qaCeilingClass+": 1 hit(s)") {
		t.Errorf("the amend must be refused by the MESSAGE arm, by class:\n%s", out)
	}
	if !strings.Contains(w.log(t), "(stamp: "+VisibilityPrivate+", commit message)") {
		t.Errorf("the refusal must be logged naming the subject:\n%s", w.log(t))
	}
	qaNoCeilingVocabulary(t, "the terminal refusal", out)
	qaNoCeilingVocabulary(t, "refusals.log", w.log(t))
}

// PIN (i): a '#'-leading line is scanned, because git KEEPS it. The default
// cleanup for a message given with -m or -F and no editor is "whitespace",
// which strips nothing but blank space — so a pasted markdown heading lands
// in the commit object. The CONTROL is git's own behaviour, asserted rather
// than assumed: with no ceiling configured the same message commits and
// `git log` shows the '#' line in HEAD. If a builder ever teaches this arm
// to strip comment lines, the refusal goes away and the control still shows
// the line landing — which is the hole, stated.
//
// MUTATION-CHECKED: the arm rendered over `grep -v '^#'` reds this pin
// alone; (g), (h), (k) stay green.
func TestQADataCeilingScansCommentLookingLines(t *testing.T) {
	w := qaCeilingWall(t, "")
	msg := "wire the export\n\n# " + qaCeilingHit + "\n"
	out, err := qaMsgCommit(t, w, w.priv, "internal/posse/c.go", "package posse\n", "-F -", msg, w.persona)
	if err == nil {
		t.Fatalf("a comment-looking line in the message must still be scanned:\n%s", out)
	}
	if !strings.Contains(out, "data-ceiling content in the commit MESSAGE") || !strings.Contains(out, qaCeilingClass+": 1 hit(s)") {
		t.Errorf("the '#' line must be refused by the MESSAGE arm, by class:\n%s", out)
	}

	// THE CONTROL: the same message under a wall with no ceiling. It
	// commits, and git kept the '#' line — so the line the arm scans is a
	// line that really lands.
	ctl := newVisWallCfg(t, "instance", "")
	if out, err := qaMsgCommit(t, ctl, ctl.priv, "internal/posse/c.go", "package posse\n", "-F -", msg, ctl.persona); err != nil {
		t.Fatalf("control: with no ceiling this message must commit: %v\n%s", err, out)
	}
	body, err := ctl.git(ctl.priv, nil, "log", "-1", "--format=%B")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "# "+qaCeilingHit) {
		t.Errorf("control: git must KEEP the '#' line under the default cleanup — if it does not, this arm is scanning something that never commits:\n%q", body)
	}
}

// PIN (j): THE RESIDUAL, pinned so it cannot change by accident. A message
// typed in the EDITOR is not scanned and cannot be: prepare-commit-msg runs
// BEFORE the editor opens and is handed git's template alone (measured,
// git 2.50.1). So this commit LANDS, and the vocabulary is in HEAD — the
// residual is real, not a rig artifact, which is why the second assertion
// is here. ADR 0050 D5 states it; this pin goes red the day a commit-msg
// layer is added, so that change is deliberate rather than discovered.
func TestQADataCeilingResidualEditorTypedMessage(t *testing.T) {
	w := qaCeilingWall(t, "")
	ed := filepath.Join(t.TempDir(), "editor.sh")
	write(t, ed, "#!/bin/sh\nprintf '%s\\n' 'from the "+qaCeilingHit+" banner' > \"$1\"\n")
	if err := os.Chmod(ed, 0o755); err != nil {
		t.Fatal(err)
	}
	w.stage(t, w.priv, "internal/posse/d.go", "package posse\n")
	out, err := w.git(w.priv, append(append([]string(nil), w.persona...), "GIT_EDITOR="+ed),
		"commit", "--", "internal/posse/d.go")
	if err != nil {
		t.Fatalf("the EDITOR path is D5's stated exclusion — it must still land: %v\n%s", err, out)
	}
	body, err := w.git(w.priv, nil, "log", "-1", "--format=%B")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, qaCeilingHit) {
		t.Fatalf("fixture premise: the editor's text must actually be the commit's message, got %q", body)
	}
	if strings.Contains(w.log(t), dataCeilingScanLabel) {
		t.Errorf("nothing may be logged: the hook never saw this text:\n%s", w.log(t))
	}
}

// PIN (k): the refusal's own claims, measured. After it, .git/COMMIT_EDITMSG
// still holds the message the writer typed and HEAD is unchanged — which is
// what the remedy tells them, and a remedy that names a file that is not
// there is worse than none. Then the operator's override passes, says so,
// and logs exactly ONE line whose tail names the commit message.
//
// MUTATION-CHECKED: the override branch dropped from the message arm reds
// the second half; the OVERRIDDEN log line rendered without the subject
// tail reds the last assertion.
func TestQADataCeilingMessageRefusalLeavesTheMessageAndOverrides(t *testing.T) {
	w := qaCeilingWall(t, "")
	const rel = "internal/posse/e.go"
	// A HEAD to be unchanged: the scratch repo starts with no commit at all,
	// and "HEAD is unchanged" over an empty repo asserts nothing.
	w.plant(t, w.priv, "internal/posse/base.go", "package posse\n")
	head0, err := w.git(w.priv, nil, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("fixture premise: the scratch repo must have a HEAD to be unchanged: %v %s", err, head0)
	}
	msg := "wire it\n\nfrom the " + qaCeilingHit + " banner\n"
	out, err := qaMsgCommit(t, w, w.priv, rel, "package posse\n", "-F -", msg, w.persona)
	if err == nil {
		t.Fatalf("fixture premise: the message must be refused:\n%s", out)
	}
	// The refusal a writer reads before they reach for the override still
	// withholds the value — class-only is the rule's own condition, not a
	// courtesy, and this is the arm whose remedy points at a LOCAL file.
	qaNoCeilingVocabulary(t, "the terminal refusal", out)
	editmsg, rerr := os.ReadFile(filepath.Join(w.priv, ".git", "COMMIT_EDITMSG"))
	if rerr != nil || !strings.Contains(string(editmsg), qaCeilingHit) {
		t.Errorf("the remedy says the message is still in .git/COMMIT_EDITMSG — it must be (err=%v):\n%q", rerr, string(editmsg))
	}
	if head1, _ := w.git(w.priv, nil, "rev-parse", "HEAD"); head1 != head0 {
		t.Errorf("a refused commit must leave HEAD alone: %q -> %q", head0, head1)
	}

	before := strings.Count(w.log(t), dataCeilingScanLabel)
	out, err = w.gitIn(w.priv, append(append([]string(nil), w.persona...), VisibilityOverrideEnv+"="+VisibilityOverrideValue),
		msg, "commit", "-F", "-", "--", rel)
	if err != nil || !strings.Contains(out, dataCeilingScanLabel+" OVERRIDDEN") {
		t.Fatalf("the operator's override must pass, and say so: %v\n%s", err, out)
	}
	log := w.log(t)
	if after := strings.Count(log, dataCeilingScanLabel); after != before+1 {
		t.Errorf("the override must log exactly one line, got %d new:\n%s", after-before, log)
	}
	if !strings.Contains(log, dataCeilingScanLabel+" OVERRIDDEN [prepare-commit-msg hook] (stamp: "+VisibilityPrivate+", commit message)") {
		t.Errorf("the OVERRIDDEN line must name the stamp and the commit message:\n%s", log)
	}
	qaNoCeilingVocabulary(t, "refusals.log after the override", log)
}
