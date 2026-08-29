package rhq

import (
	"bytes"
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
		{"the plan-usage adapter reads the token from the macOS keychain", ""},
		{"env sets, personas and skills are config under ~/.config/rhq/", ""},
	} {
		hits := ScanOps(c.text)
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

	// A file outside .beads is not this guard's business, whatever it says.
	os.WriteFile(filepath.Join(pub, "NOTES.md"), []byte("plan_guard_5h: 70 and $715/wk\n"), 0o644)
	git(pub, nil, "add", "NOTES.md")
	if out, err := git(pub, persona, "commit", "-m", "docs", "--", "NOTES.md"); err != nil {
		t.Errorf("only the beads db is scanned: %v\n%s", err, out)
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

// The stamp is the hook's own record of what config said, and a private
// repo's hook must still carry the block — so a human reading the file can
// see which way it was stamped.
func TestCommitGuardHookCarriesBothWalls(t *testing.T) {
	for _, vis := range []string{VisibilityPublic, VisibilityPrivate} {
		h := CommitGuardHook(vis)
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
	if s := "on " + "Claude" + " Max" + " the fleet is inside the plan"; !contains(classesOf(ScanOps(s)), "plan") {
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
