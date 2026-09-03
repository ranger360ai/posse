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
	"go/ast"
	"go/parser"
	"go/token"
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

// The premise the refusal rests on, pinned in the file that would break it:
// the seatbelt profile is rendered BEFORE the session's env sets are
// resolved, so the deny cannot have seen them. If a later change moves the
// env-set resolution above the render, this pin reds — and the right answer
// then is not to re-order this test but to delete the refusal and add the
// set's directory to the deny, which is the fix the ordering had made
// expensive.
//
// It also pins the two ENDS of that gap: seatbeltWallRendered is asked once
// where the profile is written and once again after the env sets exist. Drop
// the second call — the guard around credentialDirVarsIn — and the scan
// still compiles, still runs on every launch, and refuses nothing that
// matters; that is the mutation this arm exists to catch.
func TestQATheSeatbeltProfileIsRenderedBeforeTheEnvSetsAreResolved(t *testing.T) {
	t.Parallel()
	const src = "herdrback.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	lines := map[string][]int{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		case *ast.Ident:
			name = fn.Name
		}
		if name != "" {
			lines[name] = append(lines[name], fset.Position(call.Pos()).Line)
		}
		return true
	})
	first := func(name string) int {
		t.Helper()
		if len(lines[name]) == 0 {
			t.Fatalf("%s calls %s nowhere — this pin is watching a launch path that has moved, so its ordering claim measures nothing", src, name)
		}
		return lines[name][0]
	}
	last := func(name string) int {
		t.Helper()
		return lines[name][len(lines[name])-1]
	}
	render, envs, scan := first("RenderSeatbelt"), first("EnvSetVars"), first("credentialDirVarsIn")
	if render >= envs {
		t.Errorf("%s resolves env sets at line %d, at or before the seatbelt render at line %d — the credential read-deny CAN see the session's env sets now, so the launch should add their directory to the deny rather than refusing (ranger-base-x5f6p)", src, envs, render)
	}
	if scan <= envs {
		t.Errorf("%s scans for the config-dir variables at line %d, before the env sets are resolved at line %d — the scan then reads an empty list and every env set passes", src, scan, envs)
	}
	if guard := first("seatbeltWallRendered"); guard > render {
		t.Errorf("%s asks seatbeltWallRendered first at line %d, after the render at line %d — the render is meant to be one of its call sites", src, guard, render)
	}
	if guard := last("seatbeltWallRendered"); guard <= envs || guard > scan {
		t.Errorf("%s does not ask seatbeltWallRendered between the env sets (line %d) and the config-dir scan (line %d) — last asked at line %d. Unguarded, the scan refuses launches that have no seatbelt wall for an env set to move past; guarded by something else, it stops answering for the wall this launch rendered", src, envs, scan, guard)
	}
}
