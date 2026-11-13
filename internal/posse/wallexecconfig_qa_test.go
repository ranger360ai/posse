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
	"fmt"
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
	//
	// HAND-TYPED AND COMPLETE, where the two loops above read the tables
	// (ranger-base-nxus6). Those loops are the thing under test supplying
	// its own fixture: take a name out of claudeExecConfigDirNames and the
	// loop asks about one member fewer, so every per-entry mutant of either
	// table is equivalent to them. This list is the other reader, and it is
	// written out rather than derived for exactly that reason. It is also
	// the half that runs on EVERY host: the live pin below grades the same
	// mutation on the kernel's errno, and it skips where bwrap is not the
	// mechanism.
	//
	// Both tables, in full, as of 2026-11-12. A member added to a table and
	// not added here reds with the member named — which is the direction
	// that keeps the two lists one list.
	prof := SeatbeltProfile(f.ag.Name, w, nil, c)
	declared := []string{
		"agents", "commands", "daemon", "hooks", "output-styles",
		"plugins", "routines", "rules", "skills", "workflows",
		"CLAUDE.md", "daemon.json", "launch.json", "scheduled_tasks.json",
		"settings.json", "settings.local.json",
	}
	for _, n := range declared {
		if !strings.Contains(prof, sbQuote(filepath.Join(cfg, n))) {
			t.Errorf("the seatbelt profile names no deny over %s — claude runs or obeys what is planted there, and the carve is the only thing standing between a caged seat and that write (ranger-base-y438o, ranger-base-d0rnr)", n)
		}
	}
	for _, n := range append(append([]string{}, claudeExecConfigDirNames...), claudeExecConfigFileNames...) {
		if !containsString(declared, n) {
			t.Errorf("%s is a declared member of claude's config-dir carve and this pin's hand-typed list does not name it — add it. The list is hand-typed so that a name taken OUT of a table reds; a name added to a table and not here is the same drift running the other way (ranger-base-nxus6).", n)
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

// ADR 0091 D2: ONE unfiltered member list, and the three carves are that list
// filtered by the state grant. Every pin above walks a carve's own name
// lists, so none of them can see a member the list declares and no carve
// emits, or a carve emitting a path the list does not name — which is the
// drift D2 exists to make impossible, because D1's classifier reads the
// unfiltered list to decide what is not project scope. This reads both
// directions at once, over a grant that reaches all three homes.
func TestQAWallExecConfigCarvesAreTheOneMemberListFiltered(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	var grants []string
	for _, h := range []string{".claude", ".codex", ".grok"} {
		grants = append(grants, absResolve(filepath.Join(f.home, h)))
	}
	cdirs, cfiles := claudeExecConfig(f.home, grants)
	xdirs, xfiles := runtimeExecMembers(f.home, grants)
	carvedDirs := append(append([]string{}, cdirs...), xdirs...)
	carvedFiles := append(append(append([]string{}, cfiles...), runtimeConfigFiles(f.home, grants)...), xfiles...)

	want := len(claudeExecConfigDirNames) + len(claudeExecConfigFileNames) +
		2 + // codex's config.toml and grok's
		len(codexExecDirNames) + len(codexExecFileNames) +
		len(grokExecDirNames) + len(grokExecFileNames)
	members := cliHomeExecMembers(f.home)
	if len(members) != want {
		t.Errorf("cliHomeExecMembers names %d members, want %d — one row per declared name of all three homes", len(members), want)
	}
	if n := len(carvedDirs) + len(carvedFiles); n != len(members) {
		t.Errorf("the three carves emit %d paths over a grant reaching every home, the list names %d", n, len(members))
	}
	shape := map[string]bool{} // member path -> directory-shaped
	for _, m := range members {
		if _, dup := shape[m.path]; dup {
			t.Errorf("cliHomeExecMembers declares %s twice", m.path)
		}
		shape[m.path] = m.dir
	}
	for _, c := range []struct {
		paths []string
		dir   bool
	}{{carvedDirs, true}, {carvedFiles, false}} {
		for _, p := range c.paths {
			d, ok := shape[p]
			switch {
			case !ok:
				t.Errorf("a carve denies %s, which cliHomeExecMembers does not name — D1's exemption would miss a path the wall carves", p)
			case d != c.dir:
				t.Errorf("%s is carved dir=%v and declared dir=%v", p, c.dir, d)
			default:
				delete(shape, p)
			}
		}
	}
	for p := range shape {
		t.Errorf("cliHomeExecMembers names %s, which no carve emits over a grant reaching every home — D1 would exempt a path nothing walls", p)
	}

	// The half D1 rests on: the list takes no grant and filters nothing, so
	// it still names every member for a seat whose grant reaches no home at
	// all — the seat that cannot write there by any route.
	if d, fl := claudeExecConfig(f.home, nil); len(d)+len(fl) != 0 {
		t.Errorf("claudeExecConfig over no grant: %v %v", d, fl)
	}
	if fl := runtimeConfigFiles(f.home, nil); len(fl) != 0 {
		t.Errorf("runtimeConfigFiles over no grant: %v", fl)
	}
	if d, fl := runtimeExecMembers(f.home, nil); len(d)+len(fl) != 0 {
		t.Errorf("runtimeExecMembers over no grant: %v %v", d, fl)
	}
	if got := cliHomeExecMembers(f.home); len(got) != want {
		t.Errorf("cliHomeExecMembers names %d members over no grant, want %d", len(got), want)
	}
	if got := cliHomeExecMembers(""); len(got) != 0 {
		t.Errorf("cliHomeExecMembers(\"\") names %v — no home, no members", got)
	}

	// And the guards are not interchangeable: a grant naming only a
	// config.toml admits that carve, whose guard is the file, and not the
	// exec-member carve, whose guard is the home.
	only := []string{absResolve(filepath.Join(f.home, ".codex", "config.toml"))}
	if fl := runtimeConfigFiles(f.home, only); len(fl) != 1 || fl[0] != only[0] {
		t.Errorf("a grant over %s carves %v, want that file alone", only[0], fl)
	}
	if d, fl := runtimeExecMembers(f.home, only); len(d)+len(fl) != 0 {
		t.Errorf("a grant over %s alone carves %v %v — each carve narrows the grant its own guard names", only[0], d, fl)
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

// wallRefusalWords are the kernel's own words for a write the WALL refused.
// A refused row is graded on these and not on the shell's exit status, which
// is the whole of ranger-base-nxus6's fix below.
//
// MEASURED 2026-11-11 (ranger-base-2smyx) and re-measured 2026-11-12 over
// every row of the table below, on this box (linux 7.2.15 x86_64, bwrap):
// 26 of the 28 rows read `Read-only file system` — the ro-bind over the path
// or over its parent — and the two `mv` rows read `Device or resource busy`,
// because their destination IS a bind mount and a rename onto a mount point
// is refused by the mount rather than by the mode. Nothing else was
// observed, and in particular no row reads `No such file or directory`.
//
// Deliberately only the two that were measured. A row added later that the
// wall refuses some other way — a seccomp verdict, a write through a
// /dev/null mask — reds here with the shell's words on the screen, and the
// fix is to measure it and add it with its date, never to widen this list on
// a guess. The list being tight is what makes it a witness.
var wallRefusalWords = []string{
	"Read-only file system",
	"Device or resource busy",
}

// wallRefusedBy returns the words that grade `out` as a wall refusal, or "".
func wallRefusedBy(out string) string {
	for _, w := range wallRefusalWords {
		if strings.Contains(out, w) {
			return w
		}
	}
	return ""
}

// The live join: posse's own render, the real bwrap, a write at every row of
// y438o's table refused, and the writes claude needs still allowed.
//
// EVERY REFUSED ROW IS GRADED ON THE ERRNO, not on the exit status
// (ranger-base-nxus6, from ranger-base-2smyx's verify of ranger-base-6akzw).
// A nonzero exit is not evidence of a wall. Five of claudeExecConfigDirNames'
// ten members — agents, commands, hooks, output-styles, rules — have no
// directory in weFixtureIn and exist at run time ONLY because they are
// denies, since a deny inside a write grant makes BwrapArgs emit a
// PrepMkdirAll. So taking a name out of the table took the deny and the
// directory together, and the row passed on a missing parent instead of on a
// read-only one. MEASURED 2026-11-11:
//
//	for n in agents commands hooks output-styles rules; do
//	  sed -i "s/\"$n\", //" internal/posse/wallexecconfig.go
//	  go test ./internal/posse -run WallExecConfig -count=1   # ok, every time
//	done
//
// The render pins' own table loops could not grade it either — they read the
// table under test, so every per-entry mutant is equivalent to them — which
// is why TestQAWallExecConfigCarvesClaudesExecutableConfig now holds a
// hand-typed list of all sixteen members as well. That list is the witness
// on a host where bwrap is not the mechanism; here it is the shell's own
// words:
//
//	agents  fixture-dir-exists=true   .../agents/x: Read-only file system
//	hooks   fixture-dir-exists=false  .../hooks/x:  No such file or directory
//
// The second half of the same gap is a member with no row at all: dropping
// `output-styles` or `rules` changed nothing here because neither was tried.
// So the rows below are held to covering every declared member, by a loop at
// the end of this test that reads the tables to DEMAND a row and never to
// supply one. A mutant that drops a name drops its demand and keeps its
// hand-typed row, which is the asymmetry that makes a derived coverage check
// safe and the rows themselves worth hand-typing.
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
		"output-styles plant":        ": > " + q(filepath.Join(cfg, "output-styles", "x")),
		"rules plant":                ": > " + q(filepath.Join(cfg, "rules", "x")),
		// ranger-base-d0rnr: claude's own protected members. The dirs and
		// files are absent in the fixture, so these rows also hold the prep
		// that creates them before the mount. A dir row makes its dir first,
		// as a seat would: without the `mkdir -p` it is refused by ENOENT
		// whether or not the carve is there (MEASURED, the mutation that
		// dropped the names read these three green). That is the same gap
		// the errno grading closes for the other ten; the `mkdir -p` stays
		// because it is also what a seat does, and the two witnesses are
		// independent.
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
		// ranger-base-ajkqx: projects/ is read-only but for a seat's own key
		// dir, and this fixture's cwd is no session worktree, so it has none.
		// wallclaudeprojects_qa_test.go has the own-key half.
		"projects write": ": > " + q(filepath.Join(cfg, "projects", "x")),
	}
	for name, script := range refused {
		ok, out := try(script)
		switch {
		case ok:
			t.Errorf("%s: ALLOWED under the rendered wall (ranger-base-y438o row)\n script: %s\n out: %s", name, script, out)
		case wallRefusedBy(out) == "":
			t.Errorf("%s: refused, and NOT BY THE WALL — the shell's words are none of %v.\n"+
				" script: %s\n out: %s\n"+
				"A nonzero exit is not evidence of a deny. `No such file or directory` here means the path's parent is absent, which is what a member of claudeExecConfigDirNames looks like when its name has been taken OUT of the table: the deny and the PrepMkdirAll that creates the directory go together, so the row would pass on the missing parent and the carve could be dropped silently (ranger-base-nxus6). Either the fixture stopped creating what this row writes into, or the carve stopped being there.",
				name, wallRefusalWords, script, out)
		}
	}
	// Every declared member of claude's config dir is tried above. The rows
	// are hand-typed on purpose — they are the only reader of those tables
	// that a per-entry mutant cannot move — so this reads the tables to
	// DEMAND a row and never to supply one.
	for _, m := range []struct {
		names []string
		dir   bool
	}{{claudeExecConfigDirNames, true}, {claudeExecConfigFileNames, false}} {
		for _, n := range m.names {
			want := filepath.Join(cfg, n)
			if m.dir {
				want += string(filepath.Separator)
			}
			tried := false
			for _, script := range refused {
				if strings.Contains(script, want) {
					tried = true
					break
				}
			}
			if tried {
				continue
			}
			t.Errorf("no row of this table writes to %s, a declared member of claude's config-dir carve — so nothing here can tell whether the wall refuses it, and taking %q out of the table would change nothing in the only pin that reads the errno (ranger-base-nxus6). Add a row.", want, n)
		}
	}
	allowed := map[string]string{
		"history append":       "echo x >> " + q(filepath.Join(cfg, "history.jsonl")),
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

// ranger-base-l751c: claude's server-policy caches are a deny with NO seed at
// both mechanisms, and a /dev/null mask at bwrap whether or not the file is
// there. claudeManagedCache has the measurement this rests on.
func TestQAWallExecConfigMasksClaudesManagedCacheWithoutSeeding(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	planted := filepath.Join(f.cfg, "remote-settings.json")
	if err := os.WriteFile(planted, []byte(`{"env":{"X":"planted"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"remote-settings.json", "policy-limits.json", "remote-settings-consent.json", "remote-settings-helper-consent"} {
		if !containsString(claudeManagedCacheFileNames, n) {
			t.Errorf("%s is not on the list — claude obeys or trusts what is cached there (ranger-base-l751c)", n)
		}
	}
	stateDirs, err := BwrapStateDirs(f.rt)
	if err != nil {
		t.Fatal(err)
	}
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, stateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, stateDirs...)
	prof := SeatbeltProfile(f.ag.Name, w, nil, c)
	cfg := absResolve(f.cfg)
	for _, n := range claudeManagedCacheFileNames {
		p := filepath.Join(cfg, n)
		if !containsString(c.Deny, p) || !c.Masked(p) {
			t.Errorf("%s must be a masked file-shaped deny: deny=%v mask=%v", p, c.Deny, c.Mask)
		}
		if c.Seeded(p) {
			t.Errorf("%s is SEEDED — a seed is a stored answer of posse's making that every claude on the box reads", p)
		}
		if !strings.Contains(prof, sbQuote(p)) {
			t.Errorf("the seatbelt profile names no deny over %s", n)
		}
	}
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(r.Argv, "\x00")
	for _, n := range claudeManagedCacheFileNames {
		p := filepath.Join(cfg, n)
		if !strings.Contains(argv, "\x00--ro-bind\x00/dev/null\x00"+p+"\x00") {
			t.Errorf("bwrap does not mask %s with /dev/null", n)
		}
		if strings.Contains(argv, "\x00--ro-bind\x00"+p+"\x00"+p+"\x00") {
			t.Errorf("bwrap binds %s over itself — the seat would read what is planted there", n)
		}
		var kinds []PrepKind
		for _, s := range r.Prep {
			if s.Path == p {
				kinds = append(kinds, s.Kind)
			}
		}
		want := []PrepKind{PrepMaskPoint}
		if p == absResolve(planted) {
			want = nil // there already: the mask covers it, nothing is made
		}
		if fmt.Sprint(kinds) != fmt.Sprint(want) {
			t.Errorf("%s: prep %v, want %v", n, kinds, want)
		}
	}
	// A seat on another runtime gets none of it.
	other := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, "~/.codex")
	if len(other.Mask) != 0 {
		t.Errorf("a codex seat's carve masks %v — the mask narrows claude's own state grant and nothing else", other.Mask)
	}
}

// A planted link or a directory at a masked name refuses the render: a bwrap
// mount follows a link to its target and leaves the name open to a rename.
func TestQAWallExecConfigMaskRefusesALinkOrADirAtTheName(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	for _, shape := range []string{"link", "dir"} {
		t.Run(shape, func(t *testing.T) {
			f := weFixtureIn(t, wallUngrantedDir(t))
			p := filepath.Join(f.cfg, "remote-settings.json")
			var err error
			if shape == "link" {
				err = os.Symlink(filepath.Join(f.cfg, "elsewhere.json"), p)
			} else {
				err = os.Mkdir(p, 0o755)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates); err == nil || !strings.Contains(err.Error(), "cannot mask") {
				t.Errorf("a %s at %s rendered (err=%v) — the mask would not cover the name", shape, p, err)
			}
		})
	}
}

// The mask point is empty and 0600, and never touches what is already there.
func TestQAWallExecConfigMaskPointIsEmpty0600AndNeverOverwrites(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	have := filepath.Join(d, "policy-limits.json")
	if err := os.WriteFile(have, []byte(`{"restrictions":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(d, "remote-settings.json")
	if err := runBwrapPrep([]PrepStep{{PrepMaskPoint, have, "t"}, {PrepMaskPoint, fresh, "t"}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(have); string(got) != `{"restrictions":{}}` {
		t.Errorf("the mask point rewrote a file that was there: %q", got)
	}
	fi, err := os.Stat(fresh)
	if err != nil || fi.Size() != 0 || fi.Mode().Perm() != 0o600 {
		t.Errorf("the mask point is not an empty 0600 file (claude rewrites these in place, so 0444 would lock it out): %v %v", fi, err)
	}
}

// The live join: posse's own render under the real bwrap. Every masked name
// refuses a write, an append, a replace and a removal; a copy planted on the
// host before the launch reads as nothing inside; and the host is left with
// the planted copy byte-identical and every absent member an empty 0600 file.
func TestLiveWallExecConfigMasksClaudesManagedCache(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	wrcNeedBwrap(t)
	f := weFixtureIn(t, wallUngrantedDir(t))
	planted := filepath.Join(f.cfg, "remote-settings.json")
	const plant = `{"env":{"L751C":"planted"}}`
	if err := os.WriteFile(planted, []byte(plant), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	try := func(script string) (bool, string) {
		out, err := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", script)...).CombinedOutput()
		return err == nil, strings.TrimSpace(string(out))
	}
	q := shellQuote
	for _, n := range claudeManagedCacheFileNames {
		p := filepath.Join(f.cfg, n)
		tmp := filepath.Join(f.cfg, n+".l751c.tmp")
		for name, script := range map[string]string{
			"write":   "echo '{}' > " + q(p),
			"append":  "echo x >> " + q(p),
			"replace": "echo '{}' > " + q(tmp) + " && mv " + q(tmp) + " " + q(p),
			"remove":  "rm -f " + q(p),
		} {
			if ok, out := try(script); ok {
				t.Errorf("%s %s: ALLOWED under the rendered wall (ranger-base-l751c)\n out: %s", name, n, out)
			}
		}
	}
	if ok, out := try("cat " + q(planted)); strings.Contains(out, "planted") {
		t.Errorf("the seat reads the planted copy through the mask (ok=%v): %s", ok, out)
	}
	// Control: the config dir itself is still the seat's to write.
	if ok, out := try("echo x >> " + q(filepath.Join(f.cfg, "history.jsonl"))); !ok {
		t.Errorf("history append REFUSED — claude needs its config dir: %s", out)
	}
	for _, n := range claudeManagedCacheFileNames {
		p := filepath.Join(f.cfg, n)
		if p == planted {
			if b, _ := os.ReadFile(p); string(b) != plant {
				t.Errorf("the planted copy changed on the host: %q", b)
			}
			continue
		}
		fi, err := os.Lstat(p)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() != 0 || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s on the host after the run is not an empty 0600 file: %v %v", n, fi, err)
		}
	}
}
