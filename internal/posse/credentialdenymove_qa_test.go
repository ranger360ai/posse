//go:build posse_arm2

package posse

// ranger-base-x5f6p — the seatbelt credential read-deny follows the file.
//
// The defect the security review found at ranger-base-7pf1h:
// credentialReadDenyLiterals
// spelled one darwin literal, `~/.claude/.credentials.json`, and read no
// environment at all. The runtime's secure storage does not: it writes to
// CLAUDE_SECURESTORAGE_CONFIG_DIR when that is present and non-empty, else
// CLAUDE_CONFIG_DIR, else `~/.claude` (credentialDir, measured off the
// 2.1.258 bundle on ranger-base-wd4be). So on a box with either variable
// set the wall named a path the runtime no longer writes and left the one
// it does open — and every layer above reported a healthy caged launch.
//
// The old pin stayed GREEN through all of that, which is the point of the
// arms below: one per variable, and each one is a spelling of the same
// question — does the deny MOVE when the write moves.
//
// Three properties, three levels:
//
//   - the selection (credentialReadDenyLiterals): pure, every arm provable
//     from one box, goos a parameter as it already was.
//   - the render (SeatbeltCarveOut → SeatbeltProfile): the added path is
//     absResolve'd, because an SBPL literal filter matches the CANONICAL
//     path and a config dir reached through a symlink would otherwise be
//     walked straight past.
//   - the kernel (sandbox-exec): the moved file is actually refused, with a
//     control that allows it, because a deny nothing was ever shown to
//     enforce is a comment.
//
// Every arm t.Setenv's BOTH variable names, so the operator's own box
// cannot leak into an expectation.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// cdmEnv puts HOME and both config-dir variables in a known state: a value
// of cdmUnset means the variable is not there at all, which is a THIRD
// state and not the same as empty — an empty CLAUDE_SECURESTORAGE_CONFIG_DIR
// resolves to the home and shadows CLAUDE_CONFIG_DIR, and that arm is the
// one nobody would guess.
const cdmUnset = "\x00unset"

func cdmEnv(t *testing.T, home, sec, cfg string) {
	t.Helper()
	t.Setenv("HOME", home)
	// The sibling half of the deny follows CODEX_HOME / GROK_HOME since
	// ranger-base-x5cbz, so the codex and grok expectations below are the
	// HOME spelling only while neither is set — the operator's own box must
	// not leak into them any more than the two claude names do.
	unsetenvForTest(t, "CODEX_HOME")
	unsetenvForTest(t, "GROK_HOME")
	for k, v := range map[string]string{
		"CLAUDE_SECURESTORAGE_CONFIG_DIR": sec,
		"CLAUDE_CONFIG_DIR":               cfg,
	} {
		if v == cdmUnset {
			unsetenvForTest(t, k)
			continue
		}
		t.Setenv(k, v)
	}
}

func cdmCreds(dir string) string { return filepath.Join(dir, ".credentials.json") }

// The selection, one arm per variable. stateDirs is claude's own throughout
// (`~/.claude` — the launching runtime), so every arm measures the darwin
// exception rather than the "not this session's business" path: claude's own
// credentials file is denied on darwin whatever the variables say, and the
// question here is only WHICH file that is.
func TestQACredentialReadDenyFollowsTheConfigDirVariables(t *testing.T) {
	home := t.TempDir()
	sec := t.TempDir()
	cfg := t.TempDir()
	homeCreds := cdmCreds(filepath.Join(home, ".claude"))
	codexFile := filepath.Join(home, ".codex", "auth.json")
	grokFile := filepath.Join(home, ".grok", "auth.json")
	claudeState := []string{"~/.claude", "~/.claude.json"}

	for _, tc := range []struct {
		name     string
		goos     string
		sec, cfg string
		want     []string // the claude half; codex and grok are appended below
		absent   string   // a path this arm must NOT deny, "" for none
	}{
		{
			name: "neither variable set: the home, as it always was",
			goos: "darwin", sec: cdmUnset, cfg: cdmUnset,
			want: []string{homeCreds},
		},
		{
			name: "CLAUDE_CONFIG_DIR alone: the config dir's file joins the home's",
			goos: "darwin", sec: cdmUnset, cfg: cfg,
			want: []string{homeCreds, cdmCreds(cfg)},
		},
		{
			name: "CLAUDE_SECURESTORAGE_CONFIG_DIR alone: the secure-storage dir's joins it",
			goos: "darwin", sec: sec, cfg: cdmUnset,
			want: []string{homeCreds, cdmCreds(sec)},
		},
		{
			// The arm the resolver's own doc calls the one nobody would
			// guess: `n!==void 0` is a PRESENCE test, so an empty value
			// enters the branch and falls to the home — shadowing
			// CLAUDE_CONFIG_DIR rather than deferring to it. A deny that
			// deferred would wall a directory the runtime never writes and
			// leave the home open, which is the original defect wearing a
			// different variable.
			name: "CLAUDE_SECURESTORAGE_CONFIG_DIR present but EMPTY shadows CLAUDE_CONFIG_DIR: the home only",
			goos: "darwin", sec: "", cfg: cfg,
			want: []string{homeCreds}, absent: cdmCreds(cfg),
		},
		{
			// Both set and differing: secure storage wins outright, so the
			// config dir is a directory the runtime will not write into and
			// the deny does not name it. The home is still named — a file
			// left there before the variables moved does not leave with the
			// write (ADR 0019 D2, ranger-base-xjj9).
			name: "both set: secure storage wins, the config dir is not denied, the home still is",
			goos: "darwin", sec: sec, cfg: cfg,
			want: []string{homeCreds, cdmCreds(sec)}, absent: cdmCreds(cfg),
		},
		{
			name: "both naming the SAME directory is one deny, not two",
			goos: "darwin", sec: sec, cfg: sec,
			want: []string{homeCreds, cdmCreds(sec)},
		},
		{
			// A variable naming the home is the home. Without the dedupe
			// the profile carries the same literal twice, which reads to an
			// operator as two walls and is one.
			name: "a variable naming ~/.claude itself dedupes against the home",
			goos: "darwin", sec: filepath.Join(home, ".claude"), cfg: cdmUnset,
			want: []string{homeCreds},
		},
		{
			// GOOS shape survives the change: on linux that file is the
			// store of record (ADR 0019 D2) and denying the RELOCATED one
			// strands the session exactly the way denying the home one
			// would. Moving where a file is does not move which platform
			// owns it.
			name: "linux with the variable set: still no claude deny at all",
			goos: "linux", sec: sec, cfg: cfg,
			want: nil, absent: cdmCreds(sec),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cdmEnv(t, home, tc.sec, tc.cfg)
			want := append(append([]string(nil), tc.want...), codexFile, grokFile)
			got := credentialReadDenyLiterals(tc.goos, claudeState)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("goos=%s sec=%q cfg=%q:\n got  %v\n want %v", tc.goos, tc.sec, tc.cfg, got, want)
			}
			if tc.absent == "" {
				return
			}
			for _, p := range got {
				if p == tc.absent {
					t.Errorf("denied %s — the resolver does not send the runtime's write there on this arm, and a deny that names it is a claim about the wrong directory", p)
				}
			}
		})
	}
}

// The home is not merely first in the list, it is UNCONDITIONAL — and that
// is the half a "follow the resolver" fix would quietly drop. Asked of the
// candidates directly so the claim is about the union and not about the
// deny that consumes it.
func TestQACredentialCandidatesAlwaysKeepTheHomeSpelling(t *testing.T) {
	home := t.TempDir()
	sec := t.TempDir()
	cdmEnv(t, home, sec, t.TempDir())
	got := credentialFileCandidates()
	want := []string{cdmCreds(filepath.Join(home, ".claude")), cdmCreds(sec)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("credentialFileCandidates() = %v, want %v — the home spelling must survive a variable that moves the write, because the file already sitting there does", got, want)
	}
}

// The render half: an SBPL literal filter matches the canonical path
// (underDir's own note in seatbelt.go), so a config dir reached through a
// symlink must reach the profile resolved or the deny is over a spelling
// the kernel never compares against. The sweep already has this arm for the
// detective control (credentialpaths_qa_test.go); this is the same trap on
// the preventive one.
func TestQACredentialReadDenyRendersTheResolvedSpellingForASymlinkedConfigDir(t *testing.T) {
	root := sbRoot(t) // fixture HOME
	real := sbMkdir(t, filepath.Join(root, "real-config"))
	link := filepath.Join(root, "link-config")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", link)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")

	resolved := cdmCreds(absResolve(real))
	viaLink := cdmCreds(link)
	if resolved == viaLink {
		t.Skip("NOTHING MEASURED: the link and its target resolve to the same string, so this arm cannot tell an absResolve from its absence")
	}

	work := sbMkdir(t, filepath.Join(root, "work"))
	a := NewAppAt(filepath.Join(root, "posse-home"))
	gates := sbMkdir(t, a.GatesDir("developer"))
	ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}
	w := a.SeatbeltWritable(ag, work, gates)
	prof := SeatbeltProfile("developer", w, nil, a.SeatbeltCarveOut(ag, work, gates, w))

	if !strings.Contains(prof, "(literal "+sbQuote(resolved)) {
		t.Errorf("rendered profile does not name the RESOLVED spelling %s:\n%s", resolved, prof)
	}
	if strings.Contains(prof, "(literal "+sbQuote(viaLink)+")") {
		t.Errorf("rendered profile names the config dir through its symlink (%s) — the kernel compares the canonical path, so that literal walls nothing:\n%s", viaLink, prof)
	}
}

// The rendered profile, one arm per variable — the bead's DONE WHEN read at
// the level this box can actually measure. The sandbox arm below is the
// stronger claim and it SKIPS inside a caged session (ranger-base-xjw9: a
// seatbelt session may not nest a sandbox_apply), which is exactly where a
// posse session runs. This one has no such gate: it renders the real App's
// profile and reads the text, so the deny is witnessed on every box the
// suite runs on and not only on an uncaged one.
func TestQATheRenderedProfileNamesTheMovedFileForEitherVariable(t *testing.T) {
	for _, name := range credentialDirVars {
		t.Run(name, func(t *testing.T) {
			root := sbRoot(t) // fixture HOME
			moved := sbMkdir(t, filepath.Join(root, "moved-config"))
			cdmEnv(t, os.Getenv("HOME"), cdmUnset, cdmUnset)
			t.Setenv(name, moved)

			work := sbMkdir(t, filepath.Join(root, "work"))
			a := NewAppAt(filepath.Join(root, "posse-home"))
			gates := sbMkdir(t, a.GatesDir("developer"))
			ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}
			stateDirs := []string{"~/.claude", "~/.claude.json"}
			w := a.SeatbeltWritable(ag, work, gates, stateDirs...)
			prof := SeatbeltProfile("developer", w, nil, a.SeatbeltCarveOut(ag, work, gates, w, stateDirs...))

			// The moved file, and the home's beside it: a fix that merely
			// SWAPPED the literal would pass the first check and leave a
			// file already sitting in the home outside the wall.
			for _, want := range []string{
				cdmCreds(absResolve(moved)),
				absResolve(ExpandTilde("~/.claude/.credentials.json")),
			} {
				if !strings.Contains(prof, "(literal "+sbQuote(want)+")") {
					t.Errorf("%s=%s: rendered profile does not deny %s:\n%s", name, moved, want, prof)
				}
			}
		})
	}
}

// The kernel half, and the bead's own DONE WHEN: with the variable set, the
// file the runtime WOULD write is refused under the rendered profile and
// allowed under the control, the home's is still refused, and a sibling in
// the same moved directory stays readable — the deny must move, not spread.
func TestQACredentialReadDenyRefusesTheMovedFileUnderSandboxExec(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	root := sbRoot(t)
	home := os.Getenv("HOME")

	claudeDir := sbMkdir(t, filepath.Join(home, ".claude"))
	sbWrite(t, cdmCreds(claudeDir), `{"claudeAiOauth":"stale-home-copy"}`)

	moved := sbMkdir(t, filepath.Join(root, "moved-config"))
	sbWrite(t, cdmCreds(moved), `{"claudeAiOauth":"the one the runtime would write"}`)
	sibling := filepath.Join(moved, "settings.json")
	sbWrite(t, sibling, `{}`)
	t.Setenv("CLAUDE_CONFIG_DIR", moved)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")

	work := sbMkdir(t, filepath.Join(root, "work"))
	a := NewAppAt(filepath.Join(root, "posse-home"))
	gates := sbMkdir(t, a.GatesDir("developer"))
	ag := &AgentFile{Name: "developer", MemoryDir: sbMkdir(t, filepath.Join(root, "memory"))}
	stateDirs := []string{"~/.claude", "~/.claude.json"}
	w := a.SeatbeltWritable(ag, work, gates, stateDirs...)
	withCarve := a.SeatbeltCarveOut(ag, work, gates, w, stateDirs...)

	// Same isolation as the hw18 probe: stripping DenyRead by hand is what
	// makes the write carve-out the only other variable between the two
	// profiles. Omitting stateDirs would not produce a DenyRead-free
	// carve-out, it would deny the same files for a different reason.
	control := withCarve
	control.DenyRead = nil
	walled := sbRenderProfile(t, "walled.sb", SeatbeltProfile("developer", w, nil, withCarve))
	open := sbRenderProfile(t, "control.sb", SeatbeltProfile("developer", w, nil, control))

	for _, denied := range []string{cdmCreds(moved), cdmCreds(claudeDir)} {
		if crdRun(t, walled, denied) {
			t.Errorf("reading %s was ALLOWED under the carve-out", denied)
		}
		if !crdRun(t, open, denied) {
			t.Fatalf("the CONTROL refused %s too — the probe proves nothing about the deny", denied)
		}
	}
	if !crdRun(t, walled, sibling) {
		t.Errorf("reading %s was refused — the deny widened into the moved directory instead of naming one file in it", sibling)
	}
}

// ─── the residual: an env set that carries either name ────────────────────

// credentialDirVars is a hand-copied list, so it is pinned against the
// resolver's BEHAVIOR rather than against itself: each name, set alone,
// must actually move credentialDir. A name that moves nothing is a name the
// launcher's refusal below would spend an operator's launch on for no
// reason; a name the resolver honors and this list omits is the hole the
// refusal exists to close.
func TestQACredentialDirVarsAreTheNamesTheResolverActuallyHonors(t *testing.T) {
	home := t.TempDir()
	if len(credentialDirVars) == 0 {
		t.Fatal("credentialDirVars is empty — the launcher's refusal then scans for nothing and passes on every env set")
	}
	for _, name := range credentialDirVars {
		t.Run(name, func(t *testing.T) {
			cdmEnv(t, home, cdmUnset, cdmUnset)
			base, err := credentialDir()
			if err != nil {
				t.Fatal(err)
			}
			moved := t.TempDir()
			t.Setenv(name, moved)
			got, err := credentialDir()
			if err != nil {
				t.Fatal(err)
			}
			if got == base {
				t.Fatalf("setting %s left credentialDir at %s — this name is in the list but the resolver does not honor it", name, base)
			}
			if got != moved {
				t.Fatalf("setting %s resolved to %s, want %s", name, got, moved)
			}
		})
	}
}

func TestQACredentialDirVarsInFindsThemInAnEnvSetAndNothingElse(t *testing.T) {
	t.Parallel()
	vars := []EnvVar{
		{Key: "ANTHROPIC_API_KEY", Value: "sk-must-not-be-read"},
		{Key: "CLAUDE_CONFIG_DIR", Value: "/somewhere/else"},
		{Key: "CLAUDE_CONFIG_DIR_SUFFIXED", Value: "/not/this/one"},
		{Key: "CLAUDE_SECURESTORAGE_CONFIG_DIR", Value: "/elsewhere"},
	}
	got := credentialDirVarsIn(vars)
	want := []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("credentialDirVarsIn = %v, want %v (credentialDirVars order, exact keys only)", got, want)
	}
	if n := len(credentialDirVarsIn(vars[:1])); n != 0 {
		t.Errorf("an env set with neither name reported %d — every launch carrying an API key would refuse", n)
	}
	// The exactness has to be measured where nothing else can answer for
	// it. In the slice above, a prefix match on CLAUDE_CONFIG_DIR_SUFFIXED
	// is indistinguishable from the exact hit on CLAUDE_CONFIG_DIR sitting
	// beside it — the real key is never returned, only the name looked for
	// — so the near-miss must be measured ALONE. A launch is refused over
	// this answer, and refusing one over a variable the runtime's resolver
	// has never read is a wall spent on nothing.
	near := []EnvVar{
		{Key: "CLAUDE_CONFIG_DIR_SUFFIXED", Value: "/not/this/one"},
		{Key: "MY_CLAUDE_CONFIG_DIR", Value: "/nor/this"},
	}
	if got := credentialDirVarsIn(near); len(got) != 0 {
		t.Errorf("credentialDirVarsIn(%v) = %v, want none — neither name is the variable credentialDir reads, and a launch is refused over this answer", near, got)
	}
	// The refusal message names variables; it must never be a path for a
	// value to reach a log. Nothing above returns one, and this is the
	// assertion that keeps it that way.
	for _, name := range got {
		if strings.Contains(name, "/") {
			t.Errorf("credentialDirVarsIn returned %q — that is a value, not a name", name)
		}
	}
}

// The predicate the two call sites share. A launch with no seatbelt wall
// must not be refused over an env set — there is no read-deny for the
// variable to walk past at the shims tier, and the container tier renders
// its own profile inside the cage.
//
// Serial, deliberately: it reads AvailableCages, a package var another test
// is free to write, and cmd/testparallel flags exactly that (`pkgvar …
// UNCLEARED`). The predicate is three comparisons — there is nothing here
// worth a parallel slot and a race over the cage table.
func TestQASeatbeltWallRenderedIsTheTierAndNotTheName(t *testing.T) {
	plain := &Runtime{Name: "claude"}
	self := &Runtime{Name: "selfcaged", SelfSandbox: true}
	if !AvailableCages[CageSeatbelt] {
		t.Skip("NOTHING MEASURED: no sandbox-exec on this host, so every arm below is false for a reason that is not the tier")
	}
	for _, tc := range []struct {
		name string
		cage string
		rt   *Runtime
		want bool
	}{
		{"the seatbelt tier on a runtime posse wraps", CageSeatbelt, plain, true},
		{"the shims tier: no file-read wall exists to be moved past", CageShims, plain, false},
		{"the container tier: the cage renders its own profile inside", CageContainer, plain, false},
		{"a self-sandboxing runtime: posse renders no profile for it", CageSeatbelt, self, false},
		{"no runtime at all (a launch with no persona)", CageSeatbelt, nil, false},
		{"no cage resolved", "", plain, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := seatbeltWallRendered(tc.cage, tc.rt); got != tc.want {
				t.Errorf("seatbeltWallRendered(%q, %v) = %v, want %v", tc.cage, tc.rt, got, tc.want)
			}
		})
	}
}

// The premise the refusal rests on, pinned in the files that would break
// it: the seatbelt profile is rendered BEFORE the session's env sets are
// resolved, so the deny cannot have seen them. If a later change moves the
// env-set resolution above the render, this pin reds — and the right answer
// then is not to re-order this test but to delete the refusal and add the
// set's directory to the deny, which is the fix the ordering had made
// expensive.
//
// Per LAUNCH PATH and package-wide (ranger-base-179hy, widened by
// ranger-base-qg6q5). The scope has been too small twice, the same way both
// times. The pin e241b14 replaced took first(RenderSeatbelt) /
// first(EnvSetVars) over the whole of herdrback.go, so it read the three
// lines of planLaunch and stayed green while RelaunchAgent — the second path
// that renders a persona line, and the UNATTENDED one — rendered a profile,
// re-read the env sets by name, and refused nothing: a whole launch path
// invisible to it. Its replacement went per function but parsed the one
// FILE, so the same shape one file over was invisible again (measured
// 2026-09-05: a third path in a new file left all four pins green). Then the
// package was the boundary, and a path assembled one IMPORT over — both
// halves are exported — was invisible again (measured 2026-09-05,
// ranger-base-o05yg). So the sweep is every non-test source in the MODULE,
// and a path outside this package fails on the one thing it cannot do:
// ask a wall predicate and a refusal that are both unexported.
//
// A launch path here is a function that does BOTH — renders a profile and
// resolves this session's env sets — which is a property and not a list of
// names: SeatbeltReport and the reachability probe render profiles too, and
// they are out of scope because they resolve no env sets, not because they
// are spelled somewhere as exceptions — and that is what lets the scope be
// the whole module without a per-package exemption list. The cost of keying on both is that a
// launch path which stops resolving env sets drops out of the sweep instead
// of failing it, and the census floor at the bottom is what catches that —
// it is also the wall against the honest limit of an AST sweep, that a path
// reaching EnvSetVars through a helper is not seen as one.
//
// The mutation each arm is for, in order: move the env-set loop above the
// render and the refusal is asking about a wall that could have seen them;
// drop the refusal (the ranger-base-179hy defect itself) and the sets are
// resolved and never scanned; spell the wall predicate inline at the render
// and it can drift from the one the refusal asks — which is the drift
// e241b14 extracted seatbeltWallRendered to end, and which
// TestQANoLaunchPathSpellsTheSeatbeltWallPredicateItself finishes.
func TestQAEveryLaunchPathThatRendersASeatbeltRefusesACredentialDirEnvSet(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	paths := 0
	for _, src := range cdmModuleSources(t) {
		f, err := parser.ParseFile(fset, src.abs, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		inPkg := strings.HasPrefix(src.rel, "internal/posse/")
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			lines := map[string][]int{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch g := call.Fun.(type) {
				case *ast.SelectorExpr:
					name = g.Sel.Name
				case *ast.Ident:
					name = g.Name
				}
				if name != "" {
					lines[name] = append(lines[name], fset.Position(call.Pos()).Line)
				}
				return true
			})
			if len(lines["RenderSeatbelt"]) == 0 || len(lines["EnvSetVars"]) == 0 {
				continue
			}
			paths++
			who := fn.Name.Name
			callAt := func(name string) (int, bool) {
				if len(lines[name]) == 0 {
					return 0, false
				}
				return lines[name][0], true
			}
			last := func(name string) int { return lines[name][len(lines[name])-1] }
			render, _ := callAt("RenderSeatbelt")
			envs, _ := callAt("EnvSetVars")

			if !inPkg {
				t.Errorf("%s.%s renders a seatbelt profile at line %d and resolves this session's env sets at line %d — a launch path OUTSIDE internal/posse, where neither half of the wall can be asked: seatbeltWallRendered and credentialDirEnvSetRefusal are both unexported, so this path cannot ask whether it rendered a wall and cannot refuse an env set that moves the credential write past one. The launch belongs in the package that owns the wall (ranger-base-179hy, ranger-base-o05yg)", src.rel, who, render, envs)
				continue
			}
			guard, ok := callAt("seatbeltWallRendered")
			if !ok || guard > render {
				t.Errorf("%s.%s renders a seatbelt profile at line %d without asking seatbeltWallRendered first (asked at %v) — the render is one of that predicate's two call sites, and a site that spells the question itself is free to drift from the one the refusal below asks", src.rel, who, render, lines["seatbeltWallRendered"])
				continue
			}
			if envs <= render {
				t.Errorf("%s.%s resolves env sets at line %d, at or before the seatbelt render at line %d — the credential read-deny CAN see the session's env sets now, so the launch should add their directory to the deny rather than refusing (ranger-base-x5f6p)", src.rel, who, envs, render)
			}
			scan, ok := callAt("credentialDirEnvSetRefusal")
			if !ok {
				t.Errorf("%s.%s renders a seatbelt profile (line %d) and resolves env sets (line %d) but never calls credentialDirEnvSetRefusal — an env set exporting CLAUDE_CONFIG_DIR or CLAUDE_SECURESTORAGE_CONFIG_DIR moves this session's credential write past a wall already rendered, and nothing here says so (ranger-base-179hy)", src.rel, who, render, envs)
				continue
			}
			if scan <= last("EnvSetVars") {
				t.Errorf("%s.%s refuses at line %d, before the last env set is resolved at line %d — the scan then reads a short list and the sets after it pass unexamined", src.rel, who, scan, last("EnvSetVars"))
			}
		}
	}
	if paths < 2 {
		t.Fatalf("%d function(s) in this MODULE both render a seatbelt profile and resolve the session's env sets; there are two — planLaunch and RelaunchAgent — and a pin that finds fewer than two is watching a path that has left, or one that no longer resolves its env sets by name and so is checked by nothing above (ranger-base-179hy, ranger-base-qg6q5). The floor is still only the wall against a path LEAVING; a path ADDED is caught by the sweep now that the sweep is the module and not this package (ranger-base-o05yg)", paths)
	}
}

// cdmPackageSources is the scope the WALL-SPELLING sweep takes: every
// non-test source in this package. Not one file — a second spelling of the
// wall predicate added one file over is exactly what a per-file pin cannot
// see, and that has now been the defect twice (ranger-base-qg6q5).
//
// The package IS the honest boundary for that sweep, unlike the launch-path
// one above, which is why the two no longer share a scope
// (ranger-base-o05yg): the predicate it protects, seatbeltWallRendered, is
// unexported, and so is the table read it is spelled from where another
// package would have to reach it through `posse.AvailableCages`. That
// spelling exists on main today at cmd/posse's parity report, where it is
// the availability question and not the wall question — one asks whether to
// PRINT a seatbelt row, the other whether THIS launch renders a profile —
// so widening this sweep would flag a legitimate reader and say nothing
// true. What is not covered, and is stated here rather than left silent: a
// second spelling of the wall question in another package, which nothing
// measures.
func cdmPackageSources(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		t.Fatal("no non-test .go source in this package — a sweep over nothing passes for a reason that is not the property")
	}
	return out
}

// cdmSource is one file a sweep parses: the path to open, and the
// module-relative name a failure prints, which is what says WHICH package a
// hit is in once the scope is wider than one.
type cdmSource struct{ abs, rel string }

// cdmModuleSources is the scope the launch-path sweep takes: every non-test
// Go source in this MODULE.
//
// The package was the previous scope, and it was one import away from being
// walked past: RenderSeatbelt and EnvSetVars are both EXPORTED and cmd/posse
// already calls EnvSetVars, so a third launch path assembled in another
// package was invisible to the sweep — and invisible to the census floor
// under it too, because a path ADDED elsewhere leaves the count at two and
// that floor only catches a path that has LEFT (measured 2026-09-05: a
// launch path in cmd/posse calling RenderSeatbelt then EnvSetVars built
// clean and left both pins green — ranger-base-o05yg, the next rung on
// ranger-base-qg6q5's first() -> file -> package ladder).
//
// The matcher itself was already scope-free — it keys on the call names, so
// `a.RenderSeatbelt` reads the same from any package — which is what makes
// the widening a walk and not a second sweep.
func cdmModuleSources(t *testing.T) []cdmSource {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Asked of go.mod rather than derived as "../..", so a package that
	// moves retargets the walk instead of silently sweeping a subtree.
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("no go.mod above this package — the module root the sweep walks cannot be found, so the sweep would measure whatever subtree it happened to start in")
		}
		root = parent
	}
	var out []cdmSource
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if p != root && (strings.HasPrefix(n, ".") || n == "vendor" || n == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		n := d.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, cdmSource{abs: p, rel: filepath.ToSlash(rel)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Both counts positive, which is the witness that the widening is real:
	// zero here and the walk missed the package the launch paths are in,
	// zero elsewhere and it is the package sweep under a longer name.
	here, elsewhere := 0, 0
	for _, src := range out {
		if strings.HasPrefix(src.rel, "internal/posse/") {
			here++
		} else {
			elsewhere++
		}
	}
	if here == 0 || elsewhere == 0 {
		t.Fatalf("the module walk from %s found %d non-test source(s) in this package and %d outside it — a sweep reaching only one of those is measuring its own scope and not the property", root, here, elsewhere)
	}
	return out
}

// cdmWallLookup reports whether n asks the seatbelt-wall question itself,
// in either of the two spellings this package already makes available: the
// table read `AvailableCages[CageSeatbelt]`, and `cageAvailable(CageSeatbelt)`,
// the helper that returns that same read verbatim for every cage but the
// container (cage.go). The key by its constant or by the value that constant
// holds, because a second spelling is free to pick either one.
//
// The helper arm is ranger-base-o05yg. The matcher saw the INDEX EXPRESSION
// only, so a copy of the predicate that reached the table through
// cageAvailable was not swept at all — measured 2026-09-05, the identical
// function spelled with the table read reds and the one spelled through the
// helper stood green. cageAvailable's own body is not a hit and should not
// be: its key is the parameter, not the seatbelt, so it asks about whatever
// cage it was handed rather than about this wall.
//
// The honest limit after this, unchanged in kind: a spelling routed through
// a NEW helper — some third function that reads the table and returns it —
// is not seen either, and the two floors below do not catch that shape.
// What they catch is the exempt spellings MOVING, not a third arriving.
func cdmWallLookup(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.IndexExpr:
		tbl, ok := v.X.(*ast.Ident)
		return ok && tbl.Name == "AvailableCages" && cdmSeatbeltKey(v.Index)
	case *ast.CallExpr:
		name := ""
		switch f := v.Fun.(type) {
		case *ast.SelectorExpr:
			name = f.Sel.Name
		case *ast.Ident:
			name = f.Name
		}
		return name == "cageAvailable" && len(v.Args) == 1 && cdmSeatbeltKey(v.Args[0])
	}
	return false
}

// cdmSeatbeltKey reports whether e names the seatbelt cage: the constant, or
// a string literal holding the value that constant holds.
func cdmSeatbeltKey(e ast.Expr) bool {
	switch k := e.(type) {
	case *ast.Ident:
		return k.Name == "CageSeatbelt"
	case *ast.BasicLit:
		return k.Kind == token.STRING && strings.Trim(k.Value, "\"`") == CageSeatbelt
	}
	return false
}

// The other half, and the one that keeps the pin above honest: the wall
// predicate is spelled ONCE. e241b14 extracted seatbeltWallRendered so the
// render site and the question about the render could not drift apart, and
// RelaunchAgent kept its inline copy for two months — a copy that had lost
// the `rt != nil` arm and was tied to no refusal at all (ranger-base-179hy).
//
// The exemption is what the two legitimate spellings DO, not the file they
// sit in (ranger-base-qg6q5, which is what the filename exemption this
// replaces let through: a second spelling appended to seatbelt.go itself was
// not swept, measured 2026-09-05). One fills the table — an assignment is
// not a question. One is the body of seatbeltWallRendered, which is the
// place the question is answered. Anywhere else, including the rest of the
// file that owns the predicate, the lookup is a second spelling by
// definition.
func TestQANoLaunchPathSpellsTheSeatbeltWallPredicateItself(t *testing.T) {
	t.Parallel()
	const owner = "seatbeltWallRendered"
	fset := token.NewFileSet()
	sawAnswer, sawFill := "", ""
	for _, src := range cdmPackageSources(t) {
		f, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, isFn := d.(*ast.FuncDecl)
			answers := isFn && fn.Name.Name == owner
			// An AssignStmt is visited before its own Lhs, so a lookup
			// being written to is already marked by the time it is judged.
			fills := map[ast.Node]bool{}
			ast.Inspect(d, func(n ast.Node) bool {
				if as, ok := n.(*ast.AssignStmt); ok {
					for _, lhs := range as.Lhs {
						if cdmWallLookup(lhs) {
							fills[lhs] = true
						}
					}
					return true
				}
				if !cdmWallLookup(n) {
					return true
				}
				at := fmt.Sprintf("%s:%d", src, fset.Position(n.Pos()).Line)
				switch {
				case fills[n]:
					sawFill = at
				case answers:
					sawAnswer = at
				default:
					t.Errorf("%s spells the seatbelt-wall lookup itself, inside %s — that is the wall question, and seatbeltWallRendered is the one place it is answered (ranger-base-179hy, e241b14). A copy is free to drift from the spelling the credential refusal asks, and both directions are silent: a refusal for a session with no wall, or no refusal for one that has it", at, cdmDeclName(d))
				}
				return true
			})
		}
	}
	if sawAnswer == "" {
		t.Fatalf("no %s in this package reads the seatbelt-wall table — the predicate this pin exempts has moved or is spelled another way, so the sweep is measuring an absence that means nothing", owner)
	}
	if sawFill == "" {
		t.Fatalf("nothing in this package assigns the seatbelt-wall table entry — the availability fill this pin exempts has moved, so the exemption is granted to nobody and the sweep is measuring an absence")
	}
}

// cdmDeclName names the declaration a sweep hit landed in, so the failure
// above says WHERE the second spelling is and not only which file.
func cdmDeclName(d ast.Decl) string {
	switch v := d.(type) {
	case *ast.FuncDecl:
		return "func " + v.Name.Name
	case *ast.GenDecl:
		return v.Tok.String() + " (file scope)"
	}
	return "this declaration"
}

// The hoisted pair itself (ranger-base-179hy): the guard and the scan
// together, which is the unit both launch paths now call. The arms are the
// product of the two questions — is there a wall, does a set name a
// config dir — because the defect this replaces was a caller that had the
// first and not the second.
//
// Serial for the reason TestQASeatbeltWallRenderedIsTheTierAndNotTheName is:
// it reads AvailableCages, a package var another test is free to write.
func TestQACredentialDirEnvSetRefusalIsTheGuardAndTheScanTogether(t *testing.T) {
	if !AvailableCages[CageSeatbelt] {
		t.Skip("NOTHING MEASURED: no sandbox-exec on this host, so every arm below is nil for a reason that is not the scan")
	}
	plain := &Runtime{Name: "claude"}
	self := &Runtime{Name: "selfcaged", SelfSandbox: true}
	moves := []EnvVar{{Key: "FOO", Value: "bar"}, {Key: "CLAUDE_CONFIG_DIR", Value: "/elsewhere"}}
	clean := []EnvVar{{Key: "FOO", Value: "bar"}}
	both := []EnvVar{{Key: "CLAUDE_SECURESTORAGE_CONFIG_DIR", Value: "/a"}, {Key: "CLAUDE_CONFIG_DIR", Value: "/b"}}
	for _, tc := range []struct {
		name    string
		cage    string
		rt      *Runtime
		vars    []EnvVar
		refused []string // the variable names the message must carry, nil = no refusal
	}{
		{"a wall and a set that moves the write past it", CageSeatbelt, plain, moves, []string{"CLAUDE_CONFIG_DIR"}},
		{"a wall and both names: the message says both", CageSeatbelt, plain, both, []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR"}},
		{"a wall and an ordinary set", CageSeatbelt, plain, clean, nil},
		{"a wall and no env sets at all", CageSeatbelt, plain, nil, nil},
		{"the shims tier: no file-read wall for the variable to walk past", CageShims, plain, moves, nil},
		{"the container tier: the cage renders its own profile inside", CageContainer, plain, moves, nil},
		{"a self-sandboxing runtime: posse renders no profile to move past", CageSeatbelt, self, moves, nil},
		{"no runtime at all", CageSeatbelt, nil, moves, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := credentialDirEnvSetRefusal(tc.cage, tc.rt, tc.vars)
			if len(tc.refused) == 0 {
				if err != nil {
					t.Fatalf("credentialDirEnvSetRefusal = %v, want nil — a refusal here is a wall over nothing", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("credentialDirEnvSetRefusal = nil, want a refusal naming %v", tc.refused)
			}
			for _, want := range tc.refused {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal never names %s:\n%v", want, err)
				}
			}
			// NAMES only, never the value: the refusal is made ON a launch
			// path that must not learn what an env set holds.
			for _, v := range tc.vars {
				if v.Value != "" && strings.Contains(err.Error(), v.Value) {
					t.Errorf("the refusal carries %q — that is an env set's VALUE, and no launch path reads one (credentialDirVarsIn returns names):\n%v", v.Value, err)
				}
			}
		})
	}
}
