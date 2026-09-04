package posse

// AN INSTANCE'S OWN VISIBILITY PATTERNS GET CHECK 3'S SCOPE (ADR 0048 D2,
// ranger-base-uzgkz, from ranger-base-9ubk6/ranger-base-n8shu).
//
// The shipped OpsPatterns are markdown-only in check 2 for a reason ADR 0024
// D2 states and this file does not touch: the shipped list's OWN source and
// tests are byte-identical to hits, and a wall carrying an allowlist of its
// own files is a wall with a hole list. That argument is about the SHIPPED
// list. A config pattern is never in source — it lives in the operator's
// config and in the rendered hook, both untracked — so it has check 3's
// property, no legitimate public use anywhere, and ADR 0048 gives it check
// 3's reach: the ADDED lines of every staged file, code included, and
// the ADDED staged paths.
//
// WHY IT MATTERS, measured on ranger-base-9ubk6: seven recurrences of a bare
// pre-publication name reaching public main, FIVE of them in .go files —
// which check 2 does not read. The only reader of that invariant was
// TestSeedSurfaceNameCountIsZero, which walks the working tree after the
// commit has landed.
//
// EVERY PIN HERE IS MUTATION-CHECKED, per alternative — a green pin over a
// wall that never had the hole measures nothing. What each mutant is and
// what it reds is recorded at each pin; the runs are on ranger-base-uzgkz.

import (
	"strings"
	"testing"
)

// qaInstancePatternWall is the visibility wall with ONE instance pattern
// configured, in the shape of the one ADR 0048 D1 gives the operator: a
// class name and an ERE whose exception is the marker form. The name is a
// fixture's own ("zephyr"), never this box's, so what these pins measure is
// the mechanism and not the deployment.
const (
	qaInstanceClass = "pre-publication-name"
	qaInstanceName  = "zephyr"
	qaInstanceERE   = qaInstanceName + "([^-]|-[^0-9a-z]|-?$)"
	qaInstanceCfg   = OpsPatternsConfigKey + ":\n  " + qaInstanceClass + ": " + qaInstanceERE + "\n"
)

func qaInstanceWall(t *testing.T) *visWall {
	t.Helper()
	w := newVisWallCfg(t, "instance", qaInstanceCfg)
	// FIXTURE PREMISE: the pattern was ACCEPTED, not refused at stamp time.
	// A pin over a pattern the parser threw away is green against any wall
	// at all — the hook records refusals in a comment and guards nothing.
	set := (&App{ConfigPath: w.home + "/config.yaml"}).OpsPatternSet()
	if len(set.Rejected) > 0 || len(set.Extra) != 1 || set.Extra[0].Class != qaInstanceClass {
		t.Fatalf("fixture premise: the config pattern must be accepted, got extra=%+v rejected=%v", set.Extra, set.Rejected)
	}
	return w
}

// PIN (a): an instance pattern refuses an ADDED LINE in a .go file under a
// public-stamped repo. Code, not markdown — which is the whole of ADR 0048
// D2 and the shape five of the seven recurrences had.
//
// MUTATION-CHECKED, per alternative (runs on ranger-base-uzgkz):
//   - M1, the pre-0048 scope: drop both `len(extra) > 0` sections from
//     identityGuardCheck, so a config pattern rides check 0 and check 2 only.
//     Reds this pin, the PATH pin, the empty-identity pin, and the stamped
//     count in TestInstanceOpsPatternGuardsAPublicRepo.
//   - M5, the wrong renderer: put the instance EREs through
//     identityLiteralERE, which escapes every metacharacter. Reds this pin
//     and the PATH pin — an escaped ERE matches no spelling at all.
//   - M6, left in check 2 as well: nothing reds here (the line is refused
//     either way, by the other check), and the count in
//     TestInstanceOpsPatternGuardsAPublicRepo reds instead. That is what
//     makes the count pin the one that owns "scanned once, not twice".
func TestQAInstancePatternRefusesAnAddedLineInCode(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = "internal/posse/notes.go"
	body := "package posse\n\n// the " + qaInstanceName + " harness shipped this in 2025.\n"

	w.stage(t, w.pub, rel, body)
	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Fatalf("an instance pattern must refuse an added line in a .go file:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: an instance-defined visibility class in a staged file",
		"ADR 0048 D2",
		qaInstanceClass + ":", // the class, which is what a refusal names
		"matched in the staged additions:",
		OpsPatternsConfigKey + ":", // where the operator changes it
		VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
	// It is NOT check 3's identity arm wearing a different hat: the rule and
	// the remedy a writer is sent to have to be the right ones.
	if strings.Contains(out, "an operator identity literal") {
		t.Errorf("an instance-pattern hit must not be refused as an identity literal:\n%s", out)
	}
	// refusals.log outlives the terminal it printed to, so the VALUE never
	// goes in it — the class is the record (visibility.go,
	// OpsPatternsConfigKey).
	if !strings.Contains(w.log(t), "instance pattern scan [prepare-commit-msg hook] (public repo)") {
		t.Errorf("the refusal must be logged under its own name:\n%s", w.log(t))
	}
	if strings.Contains(w.log(t), qaInstanceName) {
		t.Errorf("refusals.log carried the pattern's value — that IS the confidential vocabulary:\n%s", w.log(t))
	}

	// The operator's override passes, says so, and logs exactly one line.
	before := strings.Count(w.log(t), "instance pattern scan")
	if out, err := w.git(w.pub, append(w.persona, VisibilityOverrideEnv+"="+VisibilityOverrideValue),
		"commit", "-m", "x", "--", rel); err != nil || !strings.Contains(out, "OVERRIDDEN") {
		t.Fatalf("the operator's override must pass, and say so: %v\n%s", err, out)
	}
	if after := strings.Count(w.log(t), "instance pattern scan"); after != before+1 {
		t.Errorf("the override must log exactly one line, got %d new:\n%s", after-before, w.log(t))
	}

	// THE CONTROL, and it is the one that says this pin measured the pattern
	// rather than the line: the same bytes, the same public marking, a wall
	// stamped from a config with no patterns key at all.
	plain := newVisWall(t)
	plain.stage(t, plain.pub, rel, body)
	if out, err := plain.git(plain.pub, plain.persona, "commit", "-m", "x", "--", rel); err != nil {
		t.Errorf("without the config pattern this line is clean: %v\n%s", err, out)
	}

	// And an instance pattern is still only a PUBLIC repo's business.
	w.stage(t, w.priv, rel, body)
	if out, err := w.git(w.priv, w.persona, "commit", "-m", "x", "--", rel); err != nil {
		t.Errorf("a private repo must take the same content: %v\n%s", err, out)
	}
}

// The ERE is used AS an ERE — passed through, never regexp-escaped the way
// check 3's identity literals are (identityLiteralERE). The marker form
// `<name>-<id>` is the exception ADR 0048 D1's pattern is written to make,
// and it is the reason the value is an ERE and not a fixed string.
//
// It is also pin (a)'s NEGATIVE arm: without it, a renderer that mangled the
// ERE into a plain substring would keep (a) green while refusing every
// legitimate marker in the tree.
//
// MUTATION-CHECKED: M7, a renderer that truncates the value at its first
// '(' — the shape a shell-quoting slip makes — reds THIS pin and nothing
// else in the package. (M5, escaping the ERE, does not red it: an escaped
// pattern matches neither spelling, so the marker line commits for the
// wrong reason and pin (a) is what catches that.)
func TestQAInstancePatternKeepsItsMarkerException(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = "internal/posse/markers.go"
	w.stage(t, w.pub, rel, "package posse\n\n// see "+qaInstanceName+"-7xpn and "+qaInstanceName+"-qm6c.\n")
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel); err != nil {
		t.Errorf("the marker form is legitimate and must commit: %v\n%s", err, out)
	}
}

// PIN (b): the same pattern over the ADDED staged PATHS — check 3's second
// arm (ranger-base-dmsbu), which the instance patterns inherit whole. A
// filename is exactly where a name that must not be public ends up, and a
// pure move yields no added lines at all.
//
// MUTATION-CHECKED: M2, dropping the instance section from the path loop
// and its refusal block while leaving the content arm alone, reds both
// subtests here (and the empty-identity pin, which reads the PATH arm's
// refusal out of the render) and leaves pin (a) GREEN — which is why the
// two arms are pinned separately: they are two scans of two subjects.
func TestQAInstancePatternRefusesAnAddedPath(t *testing.T) {
	w := qaInstanceWall(t)

	t.Run("a new file whose PATH carries the name, content clean", func(t *testing.T) {
		const rel = "internal/" + qaInstanceName + "/doc.go"
		// The CONTENT is spotless — the content arm above must not be what
		// refuses this, or the subtest is pin (a) wearing a path.
		w.stage(t, w.pub, rel, "package doc\n")
		out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
		if err == nil {
			t.Fatalf("an instance pattern in a staged PATH must be refused:\n%s", out)
		}
		for _, want := range []string{
			"refused by posse gate: an instance-defined visibility class in a staged PATH",
			"the FILENAME, not its content",
			qaInstanceClass + ":",
			rel,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the path refusal must carry %q:\n%s", want, out)
			}
		}
		if !strings.Contains(w.log(t), "instance pattern scan [prepare-commit-msg hook] (public repo, staged path)") {
			t.Errorf("the refusal must be logged, and say it was a path:\n%s", w.log(t))
		}
		if strings.Contains(w.log(t), qaInstanceName) {
			t.Errorf("refusals.log carried the pattern's value:\n%s", w.log(t))
		}
		// Unstage: the next subtest asserts that its move yields NO added
		// lines, and a refusal leaves its own subject in the index.
		if out, err := w.git(w.pub, nil, "rm", "-q", "--cached", "--", rel); err != nil {
			t.Fatalf("git rm --cached: %v %s", err, out)
		}
	})

	t.Run("a pure move to a path carrying the name", func(t *testing.T) {
		// 40 byte-identical lines: git pairs it R100 and the diff has ZERO
		// plus lines, so the content arm cannot see it at all.
		w.plant(t, w.pub, "internal/posse/deploy.go", "package posse\n"+strings.Repeat("// a public line\n", 40))
		const dst = "internal/posse/" + qaInstanceName + "_deploy.go"
		if out, err := w.git(w.pub, nil, "mv", "internal/posse/deploy.go", dst); err != nil {
			t.Fatalf("git mv: %v %s", err, out)
		}
		// Both fixture premises, asserted, or this subtest is pin (a) again.
		if ns, _ := w.git(w.pub, nil, "diff", "--cached", "--name-status", "HEAD"); !strings.HasPrefix(ns, "R") {
			t.Fatalf("fixture premise: git must report this move as a rename, got %q", ns)
		}
		if d, _ := w.git(w.pub, nil, "diff", "--cached", "-U0", "HEAD"); strings.Contains(d, "\n+") {
			t.Fatalf("fixture premise: a pure move must yield no added lines, got:\n%s", d)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "move", "--", "internal/posse/deploy.go", dst)
		if err == nil {
			t.Fatalf("a move to a path carrying the name must be refused:\n%s", out)
		}
		if !strings.Contains(out, "an instance-defined visibility class in a staged PATH") || !strings.Contains(out, dst) {
			t.Errorf("the refusal must name the move's destination:\n%s", out)
		}
	})
}

// PIN (c): the SHIPPED list did not move. Its scope is check 2's, markdown
// only, exactly as ADR 0024 D2 left it — a cost figure in a .go file still
// commits, and the same bytes in a .md file are still refused. Two-way,
// because "the shipped list is unchanged" is a claim about both directions
// and a one-way assertion is satisfied by a wall that scans nothing.
//
// MUTATION-CHECKED: M4, rendering OpsPatterns into check 3's block alongside
// the instance patterns, reds the .go half here and leaves the .md half
// green (and reds the stamped count) — ADR 0024 D2's residual widened by
// accident, which is the thing this pin exists to catch.
func TestQAShippedPatternsStayMarkdownOnly(t *testing.T) {
	w := qaInstanceWall(t)
	// A shipped 'cost' hit, and nothing this instance's pattern matches.
	const opsLine = "// last window's spend was $715/wk\n"

	w.stage(t, w.pub, "internal/posse/budget.go", "package posse\n\n"+opsLine)
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", "internal/posse/budget.go"); err != nil {
		t.Errorf("check 2 is markdown-only for the SHIPPED list: a .go file must commit: %v\n%s", err, out)
	}

	w.stage(t, w.pub, "docs/notes.d/budget.md", "# notes\n\n"+opsLine)
	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", "docs/notes.d/budget.md")
	if err == nil {
		t.Fatalf("the same bytes in markdown must still be refused by check 2:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: ops-class content in staged markdown in a public repo",
		"cost:", "$715/wk",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("check 2's refusal must carry %q:\n%s", want, out)
		}
	}
}

// PIN (d): the block renders when the identity derivation came back EMPTY
// and the instance patterns did not. The two sources are independent — a
// box with no git email, no .beads/redirect or an unset $HOME derives
// nothing, and that is not a reason to stand the operator's own patterns
// down. Before ADR 0048 the block returned "" on an empty identity, so a
// pattern would have been rendered nowhere but check 0 and check 2.
//
// MUTATION-CHECKED: M3, restoring `if len(identity) == 0 { return "" }`,
// reds this pin ALONE — every other pin in this file runs on a box that
// derives literals, so this is the only one that measures the guard.
func TestQAInstancePatternRendersWithoutAnyIdentity(t *testing.T) {
	t.Parallel() // renders and reads strings; no repo, no env (ranger-base-pj87l)
	extra := []OpsPattern{{Class: qaInstanceClass, ERE: qaInstanceERE}}

	block := identityGuardCheck(nil, extra)
	if block == "" {
		t.Fatal("an empty identity with a non-empty pattern list must still render check 3 (ADR 0048 D2)")
	}
	for _, want := range []string{
		"check 3: identity literals", // the banner names its sources and gained a third (ranger-base-cdxpf); this is its stable head
		"posse_check '" + qaInstanceClass + "'",
		"an instance-defined visibility class in a staged file",
		"an instance-defined visibility class in a staged PATH",
		"posse_ipaths=$(git -c core.quotePath=false diff --cached --name-only --no-renames --diff-filter=A",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("the rendered block must carry %q:\n%s", want, block)
		}
	}
	// Nothing derived and nothing configured is still nothing: the block is
	// skipped whole rather than paying for a git diff that cannot match.
	if s := identityGuardCheck(nil, nil); s != "" {
		t.Errorf("no literals and no patterns must render nothing, got:\n%s", s)
	}

	// And it reaches the real render through the real caller, which is what
	// InstallCommitGuardHook stamps — the unit above would be green over a
	// block nothing calls.
	hook := CommitGuardHook(VisibilityPublic, OpsPatternSet{Extra: extra})
	if !strings.Contains(hook, "posse_check '"+qaInstanceClass+"'") ||
		!strings.Contains(hook, "an instance-defined visibility class in a staged PATH") {
		t.Error("CommitGuardHook must carry check 3's block with no identity literals at all")
	}
}
