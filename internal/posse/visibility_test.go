package posse

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The patterns are read twice — by Go's regexp and by POSIX grep -E — so
// they may use only what both understand, and neither may carry a quote
// that would break out of the shell word they are rendered into.
func TestOpsPatternsArePortable(t *testing.T) {
	if len(OpsPatterns) == 0 {
		t.Fatal("no patterns")
	}
	seen := map[string]bool{}
	for _, p := range OpsPatterns {
		if p.Class == "" || p.Why == "" {
			t.Errorf("%q: a class and a reason are what makes a refusal readable", p.Class)
		}
		if seen[p.Class] {
			t.Errorf("duplicate class %q — the refusal names the class, so it has to be one", p.Class)
		}
		seen[p.Class] = true
		if strings.Contains(p.ERE, "'") {
			t.Errorf("%s: a single quote cannot be escaped inside a single-quoted sh word", p.Class)
		}
		for _, bad := range []string{`\t`, `\s`, `\d`, `\b`, `\w`} {
			if strings.Contains(p.ERE, bad) {
				t.Errorf("%s: %s is a GNU/Go escape POSIX ERE does not share — use a [[:class:]]", p.Class, bad)
			}
		}
		if _, err := regexp.Compile(p.ERE); err != nil {
			t.Errorf("%s: %v", p.Class, err)
		}
	}
}

// The acceptance cases from rangerhq-hrz, plus the false positives that
// made the start set unusable in this repo — a lint that fires on a quarter
// of commits is a lint that gets uninstalled, so the misses are pinned as
// hard as the hits.
func TestScanOps(t *testing.T) {
	for _, c := range []struct {
		text  string
		class string // "" = must not match anything
	}{
		{"the pass ran $715/wk against the ceiling", "cost"},
		{"today $160.26 api-equiv across 32 beads", "cost"},
		{"the pass summed $999.99 median $9.99 over 22 beads", "cost"},
		{"security find-generic-password -s 'Claude Code-credentials' -w", "credential"},
		{"~/.codex/auth.json is OAuth with a refresh token", "credential"},
		{"no XAI_API_KEY in the session env", "credential"},
		{"operator set plan_guard_5h: 70 / plan_guard_7d: 85", "guard"},
		{"autostart is ARMED (autostart_interval: 5m, autostart_dry_run: true)", "guard"},
		{"my own budget_day: 250 revised to 400/300", "guard"},
		// dispatch_epoch: is the same class of fact and arrived with ADR
		// 0028 §2 — what THIS shop's spend and launch windows are set to.
		{"we run dispatch_epoch: 30m here, so budget_pass buys half as much", "guard"},
		// The two keys ADR 0003's 2026-08-25 amendment §4 put beside
		// plan_usage_ttl:
		// `model_preflight: false` says ONE deployment has its availability
		// check switched off — the same fact as `autostart_dry_run: false`.
		// `model_probe_ttl: 0` says that deployment reads its account
		// credential at every launch, a cadence fact about a credential read.
		{"we run model_preflight: false on this box", "guard"},
		{"model_probe_ttl: 0 here, so every launch asks", "guard"},
		{"on Max 5x the fleet's marginal cost is inside the plan", "plan"},
		{"the operator is on the SuperGrok plan this month", "plan"},
		// Shell, quoted in beads about these very hooks: 22 of the 37 beads
		// the bead's `\$[0-9]` start pattern hit in this repo's own db.
		{`d=$(dirname "$0"); "$d/posse-pre-push" "$@" || exit $?`, ""},
		{`if [ "$1" = 'push' ]; then rhq_refuse; fi`, ""},
		{`while [ $# -gt 0 ]; do case "$1" in -C) dir="$2"; shift 2;; esac; done`, ""},
		// The harness's own public vocabulary: the KEY is documentation,
		// the key with a live value is the instance's.
		{"Config budget_pass: / budget_day: (API-equivalent dollars)", ""},
		{"dispatch: plan-guard overflow — plan_guard_overflow:/plan_guard_overflow_cap:", ""},
		{"config model_preflight: / model_probe_ttl: are documented in examples/config.yaml", ""},
		{"the plan-usage adapter reads the token from the macOS keychain", ""},
		{"env sets, personas and skills are config under ~/.config/rhq/", ""},
	} {
		hits := ScanOps(c.text, OpsPatternSet{})
		var got []string
		for _, h := range hits {
			got = append(got, h.Class)
		}
		switch {
		case c.class == "" && len(hits) > 0:
			t.Errorf("false positive %v on %q", got, c.text)
		case c.class != "" && !contains(got, c.class):
			t.Errorf("missed %s in %q (got %v)", c.class, c.text, got)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// Every way of being unsure has to fail the same way: closed.
func TestBeadsVisibilityFailsClosed(t *testing.T) {
	home := t.TempDir()
	pub, priv, other := filepath.Join(home, "pub"), filepath.Join(home, "priv"), filepath.Join(home, "other")
	for _, d := range []string{pub, priv, other} {
		os.MkdirAll(d, 0o755)
	}
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("beads_visibility:\n  "+pub+": public\n  "+priv+": PRIVATE\n  "+other+": secret\n"), 0o644)
	a := &App{ConfigPath: cfg}

	for _, c := range []struct{ dir, want, src string }{
		{pub, VisibilityPublic, "config beads_visibility:"},
		{priv, VisibilityPrivate, "config beads_visibility:"},
		// A value that is neither word is public, and says so — a typo must
		// not quietly buy the exemption.
		{other, VisibilityPublic, "neither public nor private"},
		// Unlisted, and the trailing-slash spelling of a listed one, which
		// is the same repo.
		{filepath.Join(home, "nope"), VisibilityPublic, "unmarked"},
		{priv + "/", VisibilityPrivate, "config beads_visibility:"},
	} {
		got, src := a.BeadsVisibility(c.dir)
		if got != c.want || !strings.Contains(src, c.src) {
			t.Errorf("%s: got %s (%s), want %s (%s)", c.dir, got, src, c.want, c.src)
		}
	}
	// No config at all is the state a fresh instance is in, and it is public.
	if got, _ := (&App{ConfigPath: filepath.Join(home, "missing.yaml")}).BeadsVisibility(pub); got != VisibilityPublic {
		t.Errorf("no config must be public, got %s", got)
	}
}

// The override is the operator's to type. Nothing the harness puts in a
// session's environment may carry it, or dispatch would be handing every
// persona the way past the wall.
func TestVisibilityOverrideIsNeverDispatched(t *testing.T) {
	roots := []string{"..", "../../cmd"}
	for _, root := range roots {
		filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			for _, form := range []string{
				`EnvVar{"` + VisibilityOverrideEnv,
				`EnvVar{VisibilityOverrideEnv`,
				VisibilityOverrideEnv + `=` + VisibilityOverrideValue,
				`Setenv("` + VisibilityOverrideEnv,
			} {
				if bytes.Contains(b, []byte(form)) {
					t.Errorf("%s puts %s into an environment (%q) — it is the operator's to type", p, VisibilityOverrideEnv, form)
				}
			}
			return nil
		})
	}
}

// rangerhq-hrz's acceptance, driven by real git: a bead carrying ops-class
// content is refused on the way into a public-marked repo's db, commits
// clean in a private-marked one, and an unmarked repo behaves as public.
func TestBeadsVisibilityGuardHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	gates := t.TempDir()
	pub, priv, unmarked := filepath.Join(home, "pub"), filepath.Join(home, "priv"), filepath.Join(home, "unmarked")
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("beads_visibility:\n  "+pub+": public\n  "+priv+": private\n"), 0o644)
	a := &App{ConfigPath: cfg}

	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(repo string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	// One bead line carrying two classes at once, in the shape bd writes.
	const dirty = `{"id":"x-1","title":"calibration","description":"the week ran $715/wk; keys via security find-generic-password -s x"}`
	const clean = `{"id":"x-2","title":"flat-YAML keeps a lone trailing quote in command:","description":"rangerhq-nvq"}`

	setup := func(repo string) {
		os.MkdirAll(filepath.Join(repo, ".beads"), 0o755)
		if out, err := git(repo, nil, "init", "-q", "-b", "main"); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
			t.Fatal(err)
		}
	}
	stage := func(repo, body string) {
		os.WriteFile(filepath.Join(repo, ".beads", "issues.jsonl"), []byte(body+"\n"), 0o644)
		git(repo, nil, "add", ".beads/issues.jsonl")
	}
	persona := []string{"RHQ_PERSONA=tester", "RHQ_GATES_DIR=" + gates}

	for _, repo := range []string{pub, priv, unmarked} {
		setup(repo)
	}

	// Public: refused, and the refusal names the class, the matched text and
	// the rule. This is also the FIRST commit in the repo — the empty-tree
	// arm, which is exactly when a db arrives whole.
	stage(pub, dirty)
	out, err := git(pub, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl")
	if err == nil {
		t.Fatalf("public repo must refuse ops-class content:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: ops-class content in a public repo's beads db",
		"cost:", "$715/wk", "credential:", "find-generic-password",
		`NOTES.md "Privacy model"`,
		"re-file the bead in the instance's PRIVATE db",
		VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("refusal must carry %q:\n%s", want, out)
		}
	}
	if o, _ := git(pub, nil, "log", "--oneline"); !strings.Contains(o, "does not have any commits") {
		t.Errorf("nothing may have landed: %s", o)
	}

	// Unmarked behaves as public — fail closed.
	stage(unmarked, dirty)
	if out, err := git(unmarked, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl"); err == nil ||
		!strings.Contains(out, "ops-class content in a public repo's beads db") {
		t.Errorf("an unmarked repo must be treated as public: %v\n%s", err, out)
	}

	// Private: the same content commits clean.
	stage(priv, dirty)
	if out, err := git(priv, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl"); err != nil {
		t.Errorf("private repo must take the same content: %v\n%s", err, out)
	}

	// Public, clean bead: through.
	stage(pub, clean)
	if out, err := git(pub, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl"); err != nil {
		t.Fatalf("a clean bead must commit in a public repo: %v\n%s", err, out)
	}
	// ADDED lines only: content already committed is not re-scanned on the
	// next sync, or one dirty bead would wall the repo shut forever.
	stage(pub, clean+"\n"+`{"id":"x-3","title":"cage engine stays on docker","description":"test the route"}`)
	if out, err := git(pub, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl"); err != nil {
		t.Errorf("appending a clean bead must not re-scan history: %v\n%s", err, out)
	}

	// The override is a decision, not a flag: only the exact word.
	stage(pub, clean+"\n"+dirty)
	if out, err := git(pub, append(persona, VisibilityOverrideEnv+"=1"), "commit", "-m", "x", "--", ".beads/issues.jsonl"); err == nil ||
		!strings.Contains(out, "refused by posse gate") {
		t.Errorf("a truthy value must not be an override: %v\n%s", err, out)
	}
	if out, err := git(pub, append(persona, VisibilityOverrideEnv+"="+VisibilityOverrideValue), "commit", "-m", "x", "--", ".beads/issues.jsonl"); err != nil ||
		!strings.Contains(out, "OVERRIDDEN") {
		t.Errorf("the operator's override must pass, and say so: %v\n%s", err, out)
	}

	// A non-markdown file outside .beads is not this guard's business,
	// whatever it says — check 2 (ADR 0024 D2) scans staged MARKDOWN
	// (MarkdownPathspecs), not arbitrary text; TestDocsGenreAndProseGuardHook
	// covers the markdown case and TestQAMarkdownScanOwnsEveryMarkdownSpelling
	// the spellings.
	os.WriteFile(filepath.Join(pub, "notes.txt"), []byte("plan_guard_5h: 70 and $715/wk\n"), 0o644)
	git(pub, nil, "add", "notes.txt")
	if out, err := git(pub, persona, "commit", "-m", "docs", "--", "notes.txt"); err != nil {
		t.Errorf("only the beads db and staged markdown are scanned: %v\n%s", err, out)
	}

	// Both refusals and the override are evidence, so all three are logged.
	logb, _ := os.ReadFile(filepath.Join(gates, "refusals.log"))
	if n := strings.Count(string(logb), "beads visibility guard"); n != 4 {
		t.Errorf("refusals.log: want 4 visibility lines, got %d:\n%s", n, logb)
	}
	if !strings.Contains(string(logb), "OVERRIDDEN") {
		t.Errorf("an override that leaves no trace is worse than a refusal:\n%s", logb)
	}
}

// ADR 0024 D2 checks 1 and 2, in the same arm and the same slot as the
// beads-jsonl guard above: a docs-genre allowlist over staged NEW files
// under docs/, and an OpsPatterns scan over the ADDED lines of every staged
// *.md, any path. Both run only in a public-stamped repo, fail closed, and
// share the beads visibility guard's override and refusals.log shape.
func TestDocsGenreAndProseGuardHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	gates := t.TempDir()
	pub, priv := filepath.Join(home, "pub"), filepath.Join(home, "priv")
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("beads_visibility:\n  "+pub+": public\n  "+priv+": private\n"), 0o644)
	a := &App{ConfigPath: cfg}

	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(repo string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	persona := []string{"RHQ_PERSONA=tester", "RHQ_GATES_DIR=" + gates}

	for _, repo := range []string{pub, priv} {
		os.MkdirAll(filepath.Join(repo, ".beads"), 0o755)
		if out, err := git(repo, nil, "init", "-q", "-b", "main"); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
			t.Fatal(err)
		}
	}
	writeAndAdd := func(repo, rel, body string) {
		p := filepath.Join(repo, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
		git(repo, nil, "add", rel)
	}

	// Check 1 — an unlisted docs/ genre is refused, naming the rule and
	// both ways through: the instance tree, or a deliberate constant edit.
	// This is also the DONE WHEN example verbatim: "committing docs/rca/x.md
	// is refused (genre)".
	writeAndAdd(pub, "docs/rca/x.md", "just an rca, no ops content\n")
	out, err := git(pub, persona, "commit", "-m", "x", "--", "docs/rca/x.md")
	if err == nil {
		t.Fatalf("an unlisted docs/ genre must be refused: %s", out)
	}
	for _, want := range []string{
		"refused by posse gate: a new docs/ file outside the public genre allowlist",
		"docs/rca/x.md",
		"(genre: rca)",
		"ADR 0024 D2 check 1",
		"write it in the instance tree instead",
		"add it to PublicDocsGenres in internal/posse/visibility.go",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("genre refusal must carry %q:\n%s", want, out)
		}
	}

	// A file staged directly under docs/, with no subdirectory, has no
	// genre and is refused the same way — the DONE WHEN's "no genre" case.
	writeAndAdd(pub, "docs/loose.md", "no subdir\n")
	if out, err := git(pub, persona, "commit", "-m", "x", "--", "docs/loose.md"); err == nil ||
		!strings.Contains(out, "none — staged directly under docs/") {
		t.Errorf("a file directly under docs/ must be refused as having no genre: %v\n%s", err, out)
	}

	// An allowlisted genre, clean content: through.
	writeAndAdd(pub, "docs/adr/0099-example.md", "# an ADR\n\nno ops content here.\n")
	if out, err := git(pub, persona, "commit", "-m", "x", "--", "docs/adr/0099-example.md"); err != nil {
		t.Fatalf("an allowlisted genre with clean content must commit: %v\n%s", err, out)
	}

	// Check 2 — an allowlisted genre is still scanned for ops content: a
	// dollar figure in an ADR is refused, showing the matched text. This is
	// the DONE WHEN's other half verbatim: "a docs/adr/x.md carrying a
	// dollar figure is refused (content)".
	writeAndAdd(pub, "docs/adr/0100-example.md", "# an ADR\n\nthe pilot cost $715/wk to run.\n")
	out, err = git(pub, persona, "commit", "-m", "x", "--", "docs/adr/0100-example.md")
	if err == nil {
		t.Fatalf("ops content in a staged .md must be refused: %s", out)
	}
	for _, want := range []string{
		"refused by posse gate: ops-class content in staged markdown in a public repo",
		"cost:", "$715/wk",
		`NOTES.md "Privacy model"`,
		"ADR 0024 D3, restate-and-cite",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("md-scan refusal must carry %q:\n%s", want, out)
		}
	}

	// The override passes the same content, and says so.
	if out, err := git(pub, append(persona, VisibilityOverrideEnv+"="+VisibilityOverrideValue), "commit", "-m", "x", "--", "docs/adr/0100-example.md"); err != nil ||
		!strings.Contains(out, "OVERRIDDEN") {
		t.Errorf("the operator's override must pass, and say so: %v\n%s", err, out)
	}

	// Private: the same content — an unlisted genre AND ops-class prose —
	// commits clean, the DONE WHEN's "same content commits clean
	// private-stamped".
	writeAndAdd(priv, "docs/rca/x.md", "the pilot cost $715/wk to run.\n")
	if out, err := git(priv, persona, "commit", "-m", "x", "--", "docs/rca/x.md"); err != nil {
		t.Errorf("a private repo must take any docs genre and any content: %v\n%s", err, out)
	}

	logb, _ := os.ReadFile(filepath.Join(gates, "refusals.log"))
	if !strings.Contains(string(logb), "docs-genre allowlist [prepare-commit-msg hook] (public repo)") {
		t.Errorf("a genre refusal must be logged:\n%s", logb)
	}
	if !strings.Contains(string(logb), "markdown ops-content scan [prepare-commit-msg hook] (public repo)") {
		t.Errorf("a md-scan refusal must be logged:\n%s", logb)
	}
	if !strings.Contains(string(logb), "markdown ops-content scan OVERRIDDEN") {
		t.Errorf("the override must be logged:\n%s", logb)
	}
}

// The stamp is the hook's own record of what config said, and a private
// repo's hook must still carry the block — so a human reading the file can
// see which way it was stamped.
func TestCommitGuardHookCarriesBothWalls(t *testing.T) {
	for _, vis := range []string{VisibilityPublic, VisibilityPrivate} {
		h := CommitGuardHook(vis, OpsPatternSet{})
		if !strings.Contains(h, "posse_beads_visibility='"+vis+"'") {
			t.Errorf("%s: the hook must record the stamp it was written with", vis)
		}
		if !strings.Contains(h, "the shared-index guard (rangerhq-lmq9)") ||
			!strings.Contains(h, "an unqualified git commit") {
			t.Errorf("%s: the shared-index wall must survive", vis)
		}
		// And it must survive UNKEYED: the operator carve-out is gone
		// (rangerhq-lt2w), so no arm of this hook may stand down on an
		// empty RHQ_PERSONA.
		if strings.Contains(h, `[ -n "$RHQ_PERSONA" ] || exit 0`) {
			t.Errorf("%s: the operator carve-out is back in the rendered wall", vis)
		}
		for _, p := range OpsPatterns {
			if !strings.Contains(h, p.ERE) {
				t.Errorf("%s: pattern %s is not in the hook — one list, both readers", vis, p.Class)
			}
		}
	}
}

// The in-session warn: fast feedback where the harness files a bead itself,
// and silent everywhere the answer is private.
func TestWarnOpsContent(t *testing.T) {
	home := t.TempDir()
	pub, priv := filepath.Join(home, "pub"), filepath.Join(home, "priv")
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("beads_visibility:\n  "+priv+": private\n"), 0o644)
	a := &App{ConfigPath: cfg}

	var w bytes.Buffer
	if !a.WarnOpsContent(&w, pub, "the verify bead for x-1", "spend was $715/wk") ||
		!strings.Contains(w.String(), "cost") || !strings.Contains(w.String(), "the verify bead for x-1") {
		t.Errorf("public repo, ops content: want a named warning, got %q", w.String())
	}
	w.Reset()
	if a.WarnOpsContent(&w, priv, "x", "spend was $715/wk") || w.Len() > 0 {
		t.Errorf("private repo: want silence, got %q", w.String())
	}
	w.Reset()
	if a.WarnOpsContent(&w, pub, "x", "flat-YAML keeps a trailing quote") || w.Len() > 0 {
		t.Errorf("clean text: want silence, got %q", w.String())
	}
}

// The plan class's brand names are ASSEMBLED from fragments in visibility.go,
// and this is what keeps them that way. Both files ship in the public repo,
// and the seed publication preflight's plan-brand check (check 2 of
// docs/runbooks/0012-seed-publication.sh — two brand phrases, case-
// insensitive, zero tolerance; they are written there, deliberately not
// here) cannot tell a detector's target from a deployment stating its own
// plan: written whole they are the same bytes, which is what re-reddened
// the final preflight (rangerhq-vm43).
//
// The needles below are assembled for the same reason, and so is every
// comment in both files — a prose quotation of the phrase is the same
// leak as a constant, which is the mistake this test caught first.
func TestPlanBrandsAreNotShippedVerbatim(t *testing.T) {
	needles := []string{"claude" + " max", "max" + " plan"}
	for _, f := range []string{"visibility.go", "visibility_test.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		low := strings.ToLower(string(b))
		for _, n := range needles {
			if strings.Contains(low, n) {
				t.Errorf("%s carries %q verbatim — seed preflight check 2 reads that as a leak, not a detector; assemble it from fragments", f, n)
			}
		}
	}
	// And the assembling must not have cost the guard a branch. This is the
	// one brand no prose fixture can spell out, so it is exercised here from
	// the same kind of fragments; the other three are prose in TestScanOps.
	if s := "on " + "Claude" + " Max" + " the fleet is inside the plan"; !contains(classesOf(ScanOps(s, OpsPatternSet{})), "plan") {
		t.Errorf("assembling the fragments lost a branch: %q no longer matches", s)
	}
}

func classesOf(ps []OpsPattern) []string {
	var cs []string
	for _, p := range ps {
		cs = append(cs, p.Class)
	}
	return cs
}

// The other half of the same lesson, and the one that bit twice: this file
// and visibility.go must not carry a crew or operator name either — check 1
// of the same preflight, zero tolerance. Rather than keep a second copy of
// the name list here (which would BE the leak), the test reads the ERE out
// of the script and applies it — one list, two readers, the same idiom the
// hook and the harness already share for OpsPatterns.
//
// The script lives in docs/runbooks/, which does not ship, so this skips in
// the published tree and guards in the one where the fix has to land.
func TestGuardFilesCarryNoCrewNames(t *testing.T) {
	const script = "../../docs/runbooks/0012-seed-publication.sh"
	b, err := os.ReadFile(script)
	if err != nil {
		t.Skip("no seed runbook here (published tree): ", err)
	}
	m := regexp.MustCompile(`grep -rniE '([^']*)'`).FindSubmatch(b)
	if m == nil {
		t.Fatalf("%s: cannot find check 1's ERE — if the check moved, move this test with it", script)
	}
	names, err := regexp.Compile("(?i)" + string(m[1]))
	if err != nil {
		t.Fatalf("check 1 ERE does not compile in Go: %v", err)
	}
	for _, f := range []string{"visibility.go", "visibility_test.go"} {
		src, rerr := os.ReadFile(f)
		if rerr != nil {
			t.Fatalf("%s: %v", f, rerr)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if hit := names.FindString(line); hit != "" {
				t.Errorf("%s:%d names %q — a person, not a mechanism; say the role instead (preflight check 1)\n\t%s",
					f, i+1, hit, strings.TrimSpace(line))
			}
		}
	}
}

// ─── an instance's own vocabulary (ranger-base-4rbs) ─────────────────────────

// What config adds, what it is refused for, and the one thing a refusal may
// never do: echo the pattern. An instance's pattern IS its confidential
// vocabulary — a client name, a codename — so the reason line carries the
// class and nothing else, in a message that goes to a terminal and into a
// generated hook file.
func TestOpsPatternSetFromConfig(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, "config.yaml")
	const secret = "Zephyrine"
	os.WriteFile(cfg, []byte(strings.Join([]string{
		OpsPatternsConfigKey + ":",
		"  client-acme: " + secret + "[[:space:]]*(Corp|Holdings)",
		"  quoted: \"(" + secret + "|Northwind)\"",
		"  bad-escape: " + secret + `\d+`,
		"  bad-quote: " + secret + "'s",
		"  bad-regexp: " + secret + "(",
		"  empty:",
		"  bad name!: " + secret,
		"  cost: " + secret,
		"",
		"beads_visibility:",
		"  " + home + ": public",
	}, "\n")), 0o644)
	a := &App{ConfigPath: cfg}
	set := a.OpsPatternSet()

	var got []string
	for _, p := range set.Extra {
		got = append(got, p.Class)
		if p.Why == "" {
			t.Errorf("%s: an instance pattern still owes the refusal a reason", p.Class)
		}
	}
	if want := []string{"client-acme", "quoted"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("accepted %v, want %v", got, want)
	}
	// The wrapping pair of double quotes is flat-YAML's, not the pattern's.
	if set.Extra[1].ERE != "("+secret+"|Northwind)" {
		t.Errorf("quoted value: got %q", set.Extra[1].ERE)
	}
	// Every rejection is NAMED — a pattern the operator believes in and that
	// is not there is worse than no pattern.
	for _, want := range []string{"bad-escape", "bad-quote", "bad-regexp", "empty: empty pattern", "bad name!", "cost"} {
		if !containsSubstr(set.Rejected, want) {
			t.Errorf("refused entry %q is not named in %v", want, set.Rejected)
		}
	}
	if len(set.Rejected) != 6 {
		t.Errorf("want 6 refusals, got %d: %v", len(set.Rejected), set.Rejected)
	}
	for _, r := range set.Rejected {
		if strings.Contains(r, secret) {
			t.Errorf("a refusal echoed the pattern — that IS the confidential vocabulary: %q", r)
		}
	}
	// The shipped list stays first and stays whole: the set is an addition,
	// never a replacement, and the zero value is the shipped list alone.
	all := set.All()
	if len(all) != len(OpsPatterns)+2 {
		t.Fatalf("All(): got %d, want shipped+2", len(all))
	}
	for i, p := range OpsPatterns {
		if all[i].Class != p.Class {
			t.Errorf("All()[%d] = %s, want the shipped %s first", i, all[i].Class, p.Class)
		}
	}
	if n := len(OpsPatternSet{}.All()); n != len(OpsPatterns) {
		t.Errorf("the zero set must be the shipped list alone, got %d", n)
	}
	// Both readers see it: the in-session warn is the Go one.
	if !contains(classesOf(ScanOps(secret+" Holdings signed", set)), "client-acme") {
		t.Error("an accepted instance pattern must match in Go")
	}
	if contains(classesOf(ScanOps(secret+" Holdings signed", OpsPatternSet{})), "client-acme") {
		t.Error("the shipped list alone must not carry an instance's class")
	}
	var w bytes.Buffer
	if !a.WarnOpsContent(&w, home, "a bead", secret+" Holdings signed") || !strings.Contains(w.String(), "client-acme") {
		t.Errorf("WarnOpsContent must read config patterns too: %q", w.String())
	}
}

func containsSubstr(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// ranger-base-4rbs's acceptance, driven by real git: a pattern from config
// refuses a matching added jsonl line in a public-marked repo, the same line
// goes through where the pattern is not configured (the control), a private
// repo is untouched, and a REFUSED config entry neither guards nor hides —
// it is named in the hook file and its vocabulary is not.
func TestInstanceOpsPatternGuardsAPublicRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	gates := t.TempDir()
	pub, priv, plain := filepath.Join(home, "pub"), filepath.Join(home, "priv"), filepath.Join(home, "plain")
	const secret = "Zephyrine"

	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte(strings.Join([]string{
		"beads_visibility:",
		"  " + pub + ": public",
		"  " + priv + ": private",
		OpsPatternsConfigKey + ":",
		"  client-acme: " + secret + "[[:space:]]*(Corp|Holdings)",
		"  bad-escape: Northwind" + `\d+`,
	}, "\n")), 0o644)
	// The control is a SEPARATE config with no patterns key at all: the same
	// repo, the same line, the shipped list alone.
	plainCfg := filepath.Join(home, "plain.yaml")
	os.WriteFile(plainCfg, []byte("beads_visibility:\n  "+plain+": public\n"), 0o644)

	a := &App{ConfigPath: cfg}
	plainApp := &App{ConfigPath: plainCfg}

	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(repo string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	persona := []string{"RHQ_PERSONA=tester", "RHQ_GATES_DIR=" + gates}
	line := `{"id":"x-1","title":"onboarding","description":"the ` + secret + ` Holdings engagement starts monday; Northwind 4 too"}`
	stage := func(repo string) {
		os.WriteFile(filepath.Join(repo, ".beads", "issues.jsonl"), []byte(line+"\n"), 0o644)
		git(repo, nil, "add", ".beads/issues.jsonl")
	}
	for _, r := range []struct {
		dir string
		app *App
	}{{pub, a}, {priv, a}, {plain, plainApp}} {
		os.MkdirAll(filepath.Join(r.dir, ".beads"), 0o755)
		if out, err := git(r.dir, nil, "init", "-q", "-b", "main"); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, _, _, err := r.app.InstallCommitGuardHook(r.dir); err != nil {
			t.Fatal(err)
		}
	}

	// Public + configured pattern: refused, naming the instance's class and
	// the text it tripped on.
	stage(pub)
	out, err := git(pub, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl")
	if err == nil {
		t.Fatalf("an instance pattern must refuse in a public repo:\n%s", out)
	}
	for _, want := range []string{"ops-class content in a public repo's beads db", "client-acme:", secret + " Holdings"} {
		if !strings.Contains(out, want) {
			t.Errorf("refusal must carry %q:\n%s", want, out)
		}
	}
	// The REFUSED config entry guards nothing — that is what being refused
	// means, and the test that says so is this line going through in the
	// same commit that the accepted one stops.
	if strings.Contains(out, "bad-escape:") && !strings.Contains(out, "REFUSED") {
		t.Errorf("a refused pattern must not be in force:\n%s", out)
	}

	// THE CONTROL: same line, same public marking, no pattern in config.
	stage(plain)
	if out, err := git(plain, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl"); err != nil {
		t.Fatalf("without the config pattern this line is clean — the refusal above measured the pattern, not the line: %v\n%s", err, out)
	}

	// Private: an instance pattern is still only a public repo's business.
	stage(priv)
	if out, err := git(priv, persona, "commit", "-m", "bd sync", "--", ".beads/issues.jsonl"); err != nil {
		t.Errorf("private repo must take the same content: %v\n%s", err, out)
	}

	// The hook FILE is the record: what is in force, and what was asked for
	// and refused — by class, never by value.
	hooks, err := hooksDir(pub)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(hooks, "prepare-commit-msg"))
	if err != nil {
		t.Fatal(err)
	}
	hook := string(b)
	if !strings.Contains(hook, "posse_check 'client-acme'") {
		t.Error("the accepted pattern is not stamped into the hook")
	}
	if !strings.Contains(hook, "REFUSED at stamp time") || !strings.Contains(hook, "bad-escape:") {
		t.Errorf("the hook must record what it was asked for and did not stamp:\n%s", hook)
	}
	if strings.Contains(hook, "Northwind") {
		t.Error("the hook recorded a REFUSED entry's value — the class is the record, the value is the secret")
	}
	// The list is stamped twice: once for the beads-jsonl arm (check 0), once
	// for the markdown-prose arm (ADR 0024 D2 check 2) — same list, two
	// call sites (visibilityGuardBody). The identity literals are stamped
	// twice as well, once per check-3 arm: the ADDED LINES of every staged
	// text file, and the ADDED staged PATHS (ranger-base-dmsbu). Same
	// rendered literal set, same posse_check, two call sites — the path arm
	// renders inside a per-path loop because posse_check keeps the class and
	// the matched text but not the subject, and the refusal has to name the
	// offending path.
	identityCalls := 2 * len(testIdentity(t, pub))
	if want, got := 2*(len(OpsPatterns)+1)+identityCalls, strings.Count(hook, "posse_check "); got != want {
		t.Errorf("want shipped+1 checks stamped twice plus %d identity checks (%d), got %d", identityCalls, want, got)
	}

	// And the launch-time identity probe must agree with the install, or an
	// instance that adds a pattern reads as "ours but stale" forever
	// (ADR 0023 identity is byte-for-byte).
	if p := a.probeL3Hooks(pub, false); !p.CommitGuard {
		t.Errorf("L3 must vouch for the hook it just stamped: %s", p.CommitGuardDegraded)
	}
}

// ─── ADR 0024 D2 check 3: identity literals ──────────────────────────────

// DeriveIdentityLiterals's three sources, each pinned against a fixture that
// controls it: username always comes back (os/user basically never fails),
// email is repo-config (git's own repo-then-global priority needs no help
// from this code), and the instance path is the redirect target's DIRNAME,
// resolved the same way beadsHome and seedBeadsRedirect resolve it — both
// forms, deduplicated when they coincide.
func TestDeriveIdentityLiterals(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	// HOME has to be a real ancestor of the redirect target, or AbbrevHome
	// has nothing to abbreviate and instance-path/instance-path-abs
	// coincide — which is a real, DEDUPED case (seen in TestDeriveIdentity
	// LiteralsSkipsAbsentSourcesSilently's kin below), not this test's.
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	os.MkdirAll(repo, 0o755)
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")

	// No .beads/redirect yet: username and email only.
	lits, err := DeriveIdentityLiterals(repo)
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]string{}
	for _, l := range lits {
		classes[l.Class] = l.Value
	}
	if classes["email"] != "t@example.com" {
		t.Errorf("email = %q, want t@example.com", classes["email"])
	}
	if classes["username"] == "" {
		t.Error("username must always be derived")
	}
	if _, ok := classes["instance-path"]; ok {
		t.Error("no .beads/redirect yet — instance-path must not appear")
	}

	// A redirect, written RELATIVE — bd's own spelling — resolves against
	// the repo root, not against .beads/ itself (identityRedirectTarget's
	// doc comment; the same arithmetic beadsHome and seedBeadsRedirect use).
	os.MkdirAll(filepath.Join(repo, ".beads"), 0o755)
	write(t, filepath.Join(repo, ".beads", "redirect"), "../instance/.beads\n")
	instance := filepath.Clean(filepath.Join(repo, "..", "instance"))

	lits, err = DeriveIdentityLiterals(repo)
	if err != nil {
		t.Fatal(err)
	}
	classes = map[string]string{}
	for _, l := range lits {
		classes[l.Class] = l.Value
	}
	if classes["instance-path-abs"] != instance {
		t.Errorf("instance-path-abs = %q, want %q", classes["instance-path-abs"], instance)
	}
	if want := AbbrevHome(instance); classes["instance-path"] != want || want == instance {
		t.Errorf("instance-path = %q, want the ~-abbreviated %q (and the fixture must make them differ)", classes["instance-path"], want)
	}
}

// A box with no git email configured anywhere derives none — `git config
// user.email` reads the global layer even outside a repo, which is exactly
// what "repo-then-global" means, so a bare non-repo dir is not by itself
// what makes email absent; an empty ~/.gitconfig is. A repo with no
// .beads/redirect at all — or one naming no target — derives nothing from
// THAT source, and does so silently: no error, just an absent class.
// Skipped sources are check 3's normal case, not a failure of it.
func TestDeriveIdentityLiteralsSkipsAbsentSourcesSilently(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.gitconfig: no global email to fall through to
	dir := t.TempDir()
	lits, err := DeriveIdentityLiterals(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lits {
		if l.Class == "instance-path" || l.Class == "instance-path-abs" || l.Class == "email" {
			t.Errorf("no configured email, no .beads at all — must not derive %s", l.Class)
		}
	}
	os.MkdirAll(filepath.Join(dir, ".beads"), 0o755)
	write(t, filepath.Join(dir, ".beads", "redirect"), "\n") // present, empty
	lits, err = DeriveIdentityLiterals(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lits {
		if strings.HasPrefix(l.Class, "instance-path") {
			t.Error("an empty redirect must derive nothing, not a path built from garbage")
		}
	}
}

// A literal containing a single quote cannot render into the single-quoted
// sh word the hook uses — the same init-panic class validateOpsERE holds
// the shipped OpsPatterns list to, except this is caught at install time
// (the value is only known on the box, never at compile time) and REFUSES
// the whole install rather than shipping a hook that would break its own
// quoting.
func TestIdentityLiteralSingleQuoteRefusesInstall(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "config", "user.email", "o'brien@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config user.email: %v %s", err, out)
	}
	if _, err := DeriveIdentityLiterals(repo); err == nil || !strings.Contains(err.Error(), "single quote") {
		t.Fatalf("DeriveIdentityLiterals must refuse a single-quote literal, got: %v", err)
	}
	a := &App{}
	if _, _, _, err := a.InstallCommitGuardHook(repo); err == nil || !strings.Contains(err.Error(), "single quote") {
		t.Fatalf("InstallCommitGuardHook must refuse rather than render broken quoting, got: %v", err)
	}
	// And nothing was written: a refused install must not leave a foreign
	// or half-rendered hook behind for the next call to trip over.
	if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")); err == nil {
		t.Error("a refused install must not write the slot")
	}
}

// The constraint CommitGuardHook's own doc comment states in the ADR's
// words: a bead id is a hyphenated word, and the rendered ERE is the WHOLE
// literal, slashes included — so a bare bead id with none of the
// surrounding path can never satisfy it. Pinned at both layers: the ERE
// itself (Go's regexp, the same dialect the shell reads) and the rendered
// hook run for real.
func TestIdentityLiteralDoesNotTripOnABareBeadID(t *testing.T) {
	const path = "/Users/t/src/ranger-base-gk6e" // a path whose LAST segment IS a bead id
	ere := identityLiteralERE(path)
	re, err := regexp.Compile(ere)
	if err != nil {
		t.Fatalf("identityLiteralERE(%q) does not compile: %v", path, err)
	}
	if !re.MatchString("see " + path + " for detail") {
		t.Fatalf("the full path must still match itself")
	}
	if re.MatchString("filed as ranger-base-gk6e, no relation") {
		t.Errorf("a bare hyphenated bead id must not match a path literal — the ERE requires the slashes too")
	}

	// And for real, through the rendered hook: staging a file that mentions
	// only the bead id (no path) must commit clean even though the box's
	// derived instance path happens to END in the same id.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	repo := filepath.Join(home, "pub")
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("beads_visibility:\n  "+repo+": public\n"), 0o644)
	a := &App{ConfigPath: cfg}
	os.MkdirAll(filepath.Join(repo, ".beads"), 0o755)
	write(t, filepath.Join(repo, ".beads", "redirect"), filepath.Join(home, "instance-ranger-base-gk6e", ".beads")+"\n")
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := git(nil, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo, "notes.md"), "discussed ranger-base-gk6e in standup\n")
	git(nil, "add", "notes.md")
	if out, err := git([]string{"RHQ_PERSONA=tester"}, "commit", "-m", "x", "--", "notes.md"); err != nil {
		t.Errorf("a bare bead id must not trip the instance-path literal: %v\n%s", err, out)
	}
}

// The end-to-end acceptance, DONE WHEN (a): a scratch public repo refuses a
// commit whose added lines carry this box's rendered username or its
// redirect-derived instance path, naming the identity class; the same
// content commits clean in a private-stamped repo. Any staged file, not
// only markdown — check 3 is not check 2.
func TestIdentityLiteralGuardHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	// Install runs IN this Go process, not through the git subprocess
	// helper below — DeriveIdentityLiterals reads $HOME directly (AbbrevHome),
	// so it has to be THIS home too, or instance and instance-abs never
	// differ and dedupe drops one of them.
	t.Setenv("HOME", home)
	gates := t.TempDir()
	pub, priv := filepath.Join(home, "pub"), filepath.Join(home, "priv")
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("beads_visibility:\n  "+pub+": public\n  "+priv+": private\n"), 0o644)
	a := &App{ConfigPath: cfg}

	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(repo string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	persona := []string{"RHQ_PERSONA=tester", "RHQ_GATES_DIR=" + gates}

	instance := filepath.Join(home, "instance")
	for _, repo := range []string{pub, priv} {
		os.MkdirAll(filepath.Join(repo, ".beads"), 0o755)
		write(t, filepath.Join(repo, ".beads", "redirect"), filepath.Join(instance, ".beads")+"\n")
		if out, err := git(repo, nil, "init", "-q", "-b", "main"); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
			t.Fatal(err)
		}
	}
	identity := testIdentity(t, pub)
	var username, instancePath string
	for _, l := range identity {
		switch l.Class {
		case "username":
			username = l.Value
		case "instance-path-abs":
			instancePath = l.Value
		}
	}
	if username == "" || instancePath == "" {
		t.Fatalf("fixture premise: both username and instance-path-abs must derive, got %+v", identity)
	}

	writeAndAdd := func(repo, rel, body string) {
		p := filepath.Join(repo, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
		git(repo, nil, "add", rel)
	}

	// The username, in a code file — check 3 is not markdown-only.
	writeAndAdd(pub, "cmd/example.go", "// owned by "+username+"\npackage main\n")
	out, err := git(pub, persona, "commit", "-m", "x", "--", "cmd/example.go")
	if err == nil {
		t.Fatalf("the box's own username in a staged file must be refused: %s", out)
	}
	for _, want := range []string{
		"refused by posse gate: an operator identity literal in a staged file",
		"username:", username,
		"ADR 0024 D2 check 3",
		"ADR 0024 D3, restate-and-cite",
		VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("identity refusal must carry %q:\n%s", want, out)
		}
	}

	// The redirect-derived instance path, in a fresh file.
	writeAndAdd(pub, "notes.txt", "the instance lives at "+instancePath+"\n")
	out, err = git(pub, persona, "commit", "-m", "x", "--", "notes.txt")
	if err == nil {
		t.Fatalf("the box's own instance path must be refused: %s", out)
	}
	if !strings.Contains(out, "instance-path-abs:") {
		t.Errorf("refusal must name the instance-path-abs class:\n%s", out)
	}

	// The override passes it through, and says so.
	if out, err := git(pub, append(persona, VisibilityOverrideEnv+"="+VisibilityOverrideValue), "commit", "-m", "x", "--", "notes.txt"); err != nil ||
		!strings.Contains(out, "OVERRIDDEN") {
		t.Errorf("the operator's override must pass, and say so: %v\n%s", err, out)
	}

	// Private: the identical content commits clean.
	writeAndAdd(priv, "notes.txt", "the instance lives at "+instancePath+", owned by "+username+"\n")
	if out, err := git(priv, persona, "commit", "-m", "x", "--", "notes.txt"); err != nil {
		t.Errorf("a private repo must take the same content: %v\n%s", err, out)
	}

	// Content with NEITHER literal commits clean in the public repo too —
	// the wall is not simply refusing everything.
	writeAndAdd(pub, "clean.txt", "nothing identifying here\n")
	if out, err := git(pub, persona, "commit", "-m", "x", "--", "clean.txt"); err != nil {
		t.Errorf("clean content must commit in the public repo: %v\n%s", err, out)
	}

	logb, _ := os.ReadFile(filepath.Join(gates, "refusals.log"))
	if !strings.Contains(string(logb), "identity literal scan [prepare-commit-msg hook] (public repo)") {
		t.Errorf("an identity refusal must be logged:\n%s", logb)
	}
	if !strings.Contains(string(logb), "identity literal scan OVERRIDDEN") {
		t.Errorf("the override must be logged:\n%s", logb)
	}
}

// DONE WHEN (b)+(c): this box's own identity literals, derived exactly as
// InstallCommitGuardHook would for THIS repo, must not appear in any
// git-tracked file — the never-committed property — except a small,
// explicitly dispositioned set (ranger-base-gk6e's own close comment
// carries the full measurement and disposition of each). Anything past
// that set is a hit nobody has judged, which is worth failing loud over.
func TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not inside a git checkout")
	}
	repo := strings.TrimSpace(string(out))
	identity, err := DeriveIdentityLiterals(hookRepo(repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(identity) == 0 {
		t.Skip("this box derives no identity literals — nothing to measure")
	}
	// Dispositioned by CLASS and PATH, not by path alone (ranger-base-d3fn1):
	// a file on this list has one judged hit, and must not become a free pass
	// for a DIFFERENT identity class landing in it later.
	//
	// NOTES.md, docs/adr/0015-constitution-promotion.md and
	// internal/posse/queuecutover_qa_test.go all name, in full, the shared
	// queue repo's conventional path — the software's OWN shipped,
	// documented location for it (ADR 0015 §4), not an operator secret. A
	// residual of deriving "the instance path" from THIS repo's own
	// .beads/redirect, which after the queue cutover names that shared
	// repo rather than a private one. (Spelled out here only in prose, on
	// purpose — the literal value itself would be a fourth hit.)
	//
	// scripts/verify-bd-pin.sh carried the username hit this check was
	// written to catch; ranger-base-r00pq scrubbed it, and re-measuring at
	// ranger-base-d3fn1 found zero username hits, so it is off the list
	// rather than kept as a standing licence.
	known := map[string]bool{
		"instance-path\x00NOTES.md":                                true,
		"instance-path\x00docs/adr/0015-constitution-promotion.md": true,
		"instance-path\x00internal/posse/queuecutover_qa_test.go":  true,
	}
	for _, lit := range identity {
		out, err := exec.Command("git", "-C", repo, "grep", "-lF", "--", lit.Value).Output()
		if err != nil {
			// grep exits 1 on no match, which is the property holding. Any
			// other exit is the census failing to run, and a census that did
			// not run must not read as a clean one (ranger-base-d3fn1).
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() == 1 {
				continue
			}
			t.Fatalf("git grep for the %s literal did not run: %v", lit.Class, err)
		}
		for _, file := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if file == "" || known[lit.Class+"\x00"+file] {
				continue
			}
			t.Errorf("identity literal (%s = %q) appears in tracked %s — undispositioned hit; either scrub it or disposition it here", lit.Class, lit.Value, file)
		}
	}
}
