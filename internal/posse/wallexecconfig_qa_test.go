//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-6akzw, the fix half of ranger-base-y438o: the caged
// write set reached paths holding code or config that runs LATER, outside the
// wall — mise's wrappers under XDG_DATA_HOME, claude's executable config in
// its shared config dir, and posse's own state dir (the launch lines a pane
// sources, the session records, the dispatcher's files).
//
// Two halves, the same split the git-refs pins use. The render pins say what
// posse WRITES into the wall at both mechanisms, from any host. The live pin
// hands that render to the real bwrap and tries a write at every row of
// y438o's table, plus the writes claude needs as controls — the join between
// "the argv names it" and "the kernel refuses it".
//
//	go test ./internal/posse -run 'WallExecConfig' -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// weFixture is a box in miniature under a root outside every blanket temp
// grant: a home with claude's config dir as a live box has it, mise's
// wrappers, and a posse home whose state dir holds every member y438o named.
type weFixture struct {
	home, cfg, state, dir, gates string
	a                            *App
	ag                           *AgentFile
	rt                           *Runtime
}

func weFixtureIn(t *testing.T, root string) weFixture {
	t.Helper()
	at := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	f := weFixture{
		home:  at("home"),
		cfg:   at("home", ".claude"),
		state: at("posse", "state"),
		dir:   at("proj"),
	}
	for _, d := range []string{
		"home/.claude/skills", "home/.claude/plugins/marketplaces", "home/.claude/projects",
		"home/.local/share/mise/command-wrappers/bin",
		"posse/state/launch", "posse/state/herdr", "posse/state/backup", "posse/state/skills",
		"posse/personas/developer", "proj",
	} {
		if err := os.MkdirAll(at(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for p, body := range map[string]string{
		"home/.claude/settings.json":                         "{}\n",
		"home/.claude/history.jsonl":                         "",
		"home/.claude/.claude.json":                          "{}\n",
		"home/.local/share/mise/command-wrappers/bin/claude": "#!/bin/sh\n",
		"posse/state/launch/peer.sh":                         "# a peer seat's launch line\n",
		"posse/state/dispatch-watch.pid":                     "1\n",
	} {
		if err := os.WriteFile(at(p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", f.home)
	t.Setenv(ClaudeConfigDirVar, f.cfg)
	f.a = &App{Home: at("posse"), StateDir: f.state}
	f.ag = &AgentFile{Name: "developer", MemoryDir: at("posse", "personas", "developer")}
	f.rt = &Runtime{Name: "claude", StateDirs: []string{"~/.claude", "~/.claude.json"}}
	f.gates = f.a.GatesDir(f.ag.Name)
	if err := os.MkdirAll(f.gates, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

// The write set itself, at both mechanisms: no grant reaches the state dir's
// members or XDG_DATA_HOME, and the seat's own gates dir is still granted.
func TestQAWallExecConfigWriteSetReachesNoStateDirAndNoDataHome(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR. Under
	// wallUngrantedDir, because a t.TempDir() is under the granted /tmp and
	// every row would read "reached" for a reason that is not this bead's.
	f := weFixtureIn(t, wallUngrantedDir(t))
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, f.rt.StateDirs...)
	for _, p := range []string{
		filepath.Join(f.state, "launch", "peer.sh"),
		filepath.Join(f.state, "herdr"),
		filepath.Join(f.state, "backup"),
		filepath.Join(f.state, "skills"),
		filepath.Join(f.state, "dispatch-watch.pid"),
		filepath.Join(f.state, "dispatch-launch.lock"),
		filepath.Join(f.home, ".local", "share", "mise", "command-wrappers", "bin", "claude"),
		// The REST of the census under XDG_DATA_HOME (ranger-base-y438o's own
		// comment, 14:12: "more executes-later dirs under .local/share"),
		// which the fix covered by dropping the whole grant and no pin named.
		// Until ranger-base-1zs49 only the mise row above held that line, and a
		// re-grant NARROWER than `.local/share` put any of these back with every
		// arm green: MEASURED 2026-11-11, with `.local/share/nvim` and
		// `.local/share/systemd` appended to homeCacheGrants, this test read ok
		// with the mise row alone and reds naming both rows with these. Each one
		// runs later and outside the wall — a systemd user unit, an editor's
		// plugin code, a desktop entry's Exec= line, a keyring.
		filepath.Join(f.home, ".local", "share", "mise", "installs", "claude"),
		filepath.Join(f.home, ".local", "share", "systemd", "user"),
		filepath.Join(f.home, ".local", "share", "nvim"),
		filepath.Join(f.home, ".local", "share", "applications"),
		filepath.Join(f.home, ".local", "share", "keyrings"),
	} {
		if writeGranted(w, absResolve(p)) {
			t.Errorf("the writable set reaches %s — code or config that runs outside the wall later (ranger-base-y438o)\n set: %v", p, w)
		}
	}
	if !writeGranted(w, absResolve(f.gates)) {
		t.Errorf("the seat's own gates dir is no longer granted — its audit logs live there: %v", w)
	}
}

// The carve, at both mechanisms: every executable member of claude's config
// dir is a deny over the state grant, and the files are seeded so the bwrap
// mount has something to cover.
func TestQAWallExecConfigCarvesClaudesExecutableConfig(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, t.TempDir())
	stateDirs, err := BwrapStateDirs(f.rt)
	if err != nil {
		t.Fatal(err)
	}
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, stateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, stateDirs...)
	cfg := absResolve(f.cfg)
	for _, n := range claudeExecConfigDirNames {
		if p := filepath.Join(cfg, n); !containsString(c.Deny, p) || c.FileShaped(p) {
			t.Errorf("%s must be a directory-shaped deny: deny=%v", p, c.Deny)
		}
	}
	for _, n := range claudeExecConfigFileNames {
		p := filepath.Join(cfg, n)
		if !containsString(c.Deny, p) || !c.Seeded(p) {
			t.Errorf("%s must be a seeded file-shaped deny: deny=%v seed=%v", p, c.Deny, c.Seed)
		}
	}
	// The SBPL half: the same carve renders a deny line per member.
	prof := SeatbeltProfile(f.ag.Name, w, nil, c)
	for _, n := range []string{"settings.json", "skills", "plugins", "CLAUDE.md"} {
		if !strings.Contains(prof, sbQuote(filepath.Join(cfg, n))) {
			t.Errorf("the seatbelt profile names no deny over %s", n)
		}
	}
	// And a seat on another runtime gets none of it.
	other := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, "~/.codex")
	for _, d := range other.Deny {
		if underDir(cfg, d) {
			t.Errorf("a codex seat's carve names %s — the carve narrows claude's own state grant and nothing else", d)
		}
	}
}

// ranger-base-d0rnr: the members claude itself guards as executable, beyond
// the ones 6akzw measured, are on the carve's lists. The render pin above
// walks the lists, so it cannot notice a name dropped from them; this does.
// claudeExecConfig's comment has what each one does and what was measured.
func TestQAWallExecConfigCarvesClaudesOwnProtectedMembers(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"workflows", "routines", "daemon"} {
		if !containsString(claudeExecConfigDirNames, n) {
			t.Errorf("%s/ is not carved — claude starts or obeys what is planted there (ranger-base-d0rnr)", n)
		}
	}
	for _, n := range []string{"daemon.json", "launch.json", "scheduled_tasks.json"} {
		if !containsString(claudeExecConfigFileNames, n) {
			t.Errorf("%s is not carved — claude starts or obeys what is planted there (ranger-base-d0rnr)", n)
		}
		if seedBody(n) != "{}\n" {
			t.Errorf("%s would be seeded %q, not the empty JSON object a reader takes as nothing configured", n, seedBody(n))
		}
	}
}

// A seed never rewrites what is there, and writes the empty value of its
// format when nothing is.
func TestQAWallExecConfigSeedCreatesEmptyAndNeverOverwrites(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	have := filepath.Join(d, "settings.json")
	if err := os.WriteFile(have, []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(d, "sub", "settings.local.json")
	md := filepath.Join(d, "CLAUDE.md")
	if err := runBwrapPrep([]PrepStep{{PrepSeed, have, "t"}, {PrepSeed, fresh, "t"}, {PrepSeed, md, "t"}}); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{have: `{"theme":"dark"}`, fresh: "{}\n", md: ""} {
		if got, _ := os.ReadFile(p); string(got) != want {
			t.Errorf("%s reads %q after the seed, want %q", p, got, want)
		}
	}
}

// The launcher's own write of a launch line replaces whatever sits at the
// name — a symlink included — and never writes through it.
func TestQAWallExecConfigLaunchLineReplacesAPlantedLink(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	a := &App{Home: d, StateDir: filepath.Join(d, "state")}
	if err := os.MkdirAll(a.LaunchDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(d, "victim")
	if err := os.WriteFile(victim, []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, a.LaunchScript("s1")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PaneLine("s1", strings.Repeat("x", PaneLineMax+1)); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(victim); string(got) != "untouched\n" {
		t.Errorf("the launch line was written THROUGH a link at its name into %s: %q", victim, got)
	}
	if fi, err := os.Lstat(a.LaunchScript("s1")); err != nil || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm() != 0o600 {
		t.Errorf("the launch script is not a 0600 regular file after the write: %v %v", fi, err)
	}
}

// The live join: posse's own render, the real bwrap, a write at every row of
// y438o's table refused, and the writes claude needs still allowed.
func TestLiveWallExecConfigRefusesEveryRowUnderTheRenderedWall(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	if why := liveBwrapApplyRefusal(); why != "" {
		t.Skip("NOTHING MEASURED: " + why)
	}
	f := weFixtureIn(t, wallUngrantedDir(t))
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	try := func(script string) (bool, string) {
		cmd := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", script)...)
		out, err := cmd.CombinedOutput()
		return err == nil, strings.TrimSpace(string(out))
	}
	q := shellQuote
	cfg, st := f.cfg, f.state
	refused := map[string]string{
		"mise wrapper append":        "echo x >> " + q(filepath.Join(f.home, ".local/share/mise/command-wrappers/bin/claude")),
		"mise wrapper plant":         ": > " + q(filepath.Join(f.home, ".local/share/mise/command-wrappers/bin/git")),
		"settings.json append":       "echo x >> " + q(filepath.Join(cfg, "settings.json")),
		"settings.json replace":      "echo '{}' > " + q(filepath.Join(cfg, "s.tmp")) + " && mv " + q(filepath.Join(cfg, "s.tmp")) + " " + q(filepath.Join(cfg, "settings.json")),
		"settings.local.json create": "echo x >> " + q(filepath.Join(cfg, "settings.local.json")),
		"CLAUDE.md create":           "echo x >> " + q(filepath.Join(cfg, "CLAUDE.md")),
		"skills plant":               ": > " + q(filepath.Join(cfg, "skills", "x")),
		"plugins plant":              ": > " + q(filepath.Join(cfg, "plugins", "marketplaces", "x")),
		"agents plant":               ": > " + q(filepath.Join(cfg, "agents", "x")),
		"commands plant":             ": > " + q(filepath.Join(cfg, "commands", "x")),
		"hooks plant":                ": > " + q(filepath.Join(cfg, "hooks", "x")),
		// ranger-base-d0rnr: claude's own protected members. The dirs and
		// files are absent in the fixture, so these rows also hold the prep
		// that creates them before the mount. A dir row makes its dir first,
		// as a seat would: without the `mkdir -p` it is refused by ENOENT
		// whether or not the carve is there (MEASURED, the mutation that
		// dropped the names read these three green).
		"workflows plant":              "mkdir -p " + q(filepath.Join(cfg, "workflows")) + " && : > " + q(filepath.Join(cfg, "workflows", "x.js")),
		"routines plant":               "mkdir -p " + q(filepath.Join(cfg, "routines")) + " && : > " + q(filepath.Join(cfg, "routines", "x.md")),
		"daemon plant":                 "mkdir -p " + q(filepath.Join(cfg, "daemon")) + " && : > " + q(filepath.Join(cfg, "daemon", "x")),
		"daemon.json append":           "echo x >> " + q(filepath.Join(cfg, "daemon.json")),
		"launch.json append":           "echo x >> " + q(filepath.Join(cfg, "launch.json")),
		"scheduled_tasks.json append":  "echo x >> " + q(filepath.Join(cfg, "scheduled_tasks.json")),
		"scheduled_tasks.json replace": "echo '{}' > " + q(filepath.Join(cfg, "t.tmp")) + " && mv " + q(filepath.Join(cfg, "t.tmp")) + " " + q(filepath.Join(cfg, "scheduled_tasks.json")),
		"launch line append":           "echo x >> " + q(filepath.Join(st, "launch", "peer.sh")),
		"launch line plant":            ": > " + q(filepath.Join(st, "launch", "other.sh")),
		"herdr record plant":           ": > " + q(filepath.Join(st, "herdr", "x")),
		"backup plant":                 ": > " + q(filepath.Join(st, "backup", "x")),
		"skills tree plant":            ": > " + q(filepath.Join(st, "skills", "x")),
		"dispatch pid append":          "echo x >> " + q(filepath.Join(st, "dispatch-watch.pid")),
		"state dir new file":           ": > " + q(filepath.Join(st, "new")),
	}
	for name, script := range refused {
		if ok, out := try(script); ok {
			t.Errorf("%s: ALLOWED under the rendered wall (ranger-base-y438o row)\n script: %s\n out: %s", name, script, out)
		}
	}
	allowed := map[string]string{
		"history append":       "echo x >> " + q(filepath.Join(cfg, "history.jsonl")),
		"projects write":       ": > " + q(filepath.Join(cfg, "projects", "x")),
		".claude.json replace": "echo '{}' > " + q(filepath.Join(cfg, ".claude.json.tmp.1")) + " && mv " + q(filepath.Join(cfg, ".claude.json.tmp.1")) + " " + q(filepath.Join(cfg, ".claude.json")),
		"new state member":     "mkdir -p " + q(filepath.Join(cfg, "session-env", "s")),
		"own audit log append": "echo x >> " + q(filepath.Join(f.gates, "refusals.log")),
		"session tree write":   ": > " + q(filepath.Join(f.dir, "x")),
		"own cache write":      "mkdir -p \"$GOCACHE/x\"", // ranger-base-olr0g: the seat's own, not ~/.cache
	}
	for name, script := range allowed {
		if ok, out := try(script); !ok {
			t.Errorf("%s: REFUSED under the rendered wall — claude or the seat needs this write\n script: %s\n out: %s", name, script, out)
		}
	}
}
