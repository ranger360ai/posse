//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-ighvi: a bwrap seat that is not the pusher reads none
// of the ssh keys this box is configured to push with.
//
// THE CLASS. The bwrap wall is a write-scope wall, so every file the account
// can read was readable from a seat, deploy keys included, and an ssh
// transport that does not spell `git push` goes around every push gate. The
// fix masks every IdentityFile ssh's own configs name (and ssh's default
// names) with /dev/null, for every seat except the one whose PID grants push.
//
// The parse and render arms run on every host against fixture trees. The
// live arm skips without Linux and bwrap, and renders the wall the launcher
// renders, so it measures the argv a seat gets and not a hand-built one.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sshFixture is a home with a config that uses every shape the parser reads.
type sshFixture struct {
	home string
	keys []string // every private key the configs name, absolute
	odd  string   // a key on disk that nothing names
}

func writeFile(t *testing.T, p, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func newSSHFixture(t *testing.T, root string) sshFixture {
	t.Helper()
	home := filepath.Join(root, "home")
	ssh := filepath.Join(home, ".ssh")
	f := sshFixture{home: home}
	key := func(rel string) string {
		p := filepath.Join(home, rel)
		writeFile(t, p, "PRIVATE "+rel+"\n", 0o600)
		return p
	}
	// Host-keyed: one file per host, named with %h, reached by a glob.
	f.keys = append(f.keys,
		key(".ssh/posse_deploy"),
		key(".ssh/with space_deploy"),
		key(".ssh/env_deploy"),
		key(".ssh/pd_deploy"),
		key(".ssh/inc_deploy"),
		key(".ssh/hosts/github-a.key"),
		key(".ssh/hosts/github-b.key"),
		key(".ssh/agentpick"),
		// OUTSIDE ~/.ssh: the allowlist cannot reach it, so the named-key
		// mask still must (ranger-base-12ggv keeps the /dev/null binds).
		key("keys/outside_deploy"),
	)
	f.odd = key(".ssh/nobody_named_me")
	writeFile(t, filepath.Join(ssh, "known_hosts"), "KNOWN github.com ssh-ed25519 AAAA\n", 0o644)
	writeFile(t, filepath.Join(ssh, "config"), `# a comment line
Host github-posse
  HostName github.com
  IdentityFile ~/.ssh/posse_deploy
  IdentitiesOnly yes
Host spaced
  IdentityFile "~/.ssh/with space_deploy"   # trailing comment
Host env
  IdentityFile=${SSHFIX}/.ssh/env_deploy
Host pd
  identityfile %d/.ssh/pd_deploy
Host none
  IdentityFile none
Host perhost-*
  IdentityFile ~/.ssh/hosts/%h.key
Host agent
  IdentityFile ~/.ssh/agentpick.pub
Host outside
  IdentityFile ~/keys/outside_deploy
Include conf.d/*.conf
`, 0o600)
	writeFile(t, filepath.Join(ssh, "conf.d", "10-inc.conf"), "Host inc\n  IdentityFile ~/.ssh/inc_deploy\n", 0o600)
	if err := os.MkdirAll(filepath.Join(ssh, "conf.d", "a-dir.conf"), 0o700); err != nil {
		t.Fatal(err) // a directory a glob matches is skipped, as ssh does
	}
	return f
}

func (f sshFixture) env() sshEnv {
	return sshEnv{
		Home: f.home, User: "seat", UID: 1001, Host: "box.example",
		Getenv: func(k string) string {
			if k == "SSHFIX" {
				return f.home
			}
			return ""
		},
	}
}

func TestSSHIdentityFilesReadsWhatSSHReads(t *testing.T) {
	t.Parallel()
	f := newSSHFixture(t, t.TempDir())
	got, err := sshIdentityFiles(f.env())
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range f.keys {
		if !containsString(got, k) {
			t.Errorf("the list misses %s, which the config names — a seat could read it:\n  %v", k, got)
		}
	}
	for _, n := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		if p := filepath.Join(f.home, ".ssh", n); !containsString(got, p) {
			t.Errorf("the list misses ssh's default identity %s", p)
		}
	}
	if containsString(got, f.odd) {
		t.Errorf("the list names %s, which no config names: the list is a glob, not ssh's own resolution (7pf1h)", f.odd)
	}
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Errorf("relative entry %q: the render would mask a path against its own cwd", p)
		}
		if strings.HasSuffix(p, "/none") {
			t.Errorf("`IdentityFile none` read as a file: %q", p)
		}
	}
}

func TestSSHIdentityFilesReadsTheSystemConfigAndARelativeKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sys := filepath.Join(root, "etc", "ssh", "ssh_config")
	writeFile(t, sys, "Include ssh_config.d/*.conf\n", 0o644)
	writeFile(t, filepath.Join(root, "etc", "ssh", "ssh_config.d", "50.conf"), "IdentityFile /srv/keys/sys_deploy\nIdentityFile rel_deploy\n", 0o644)
	got, err := sshIdentityFiles(sshEnv{System: sys, Cwd: "/work/tree"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/srv/keys/sys_deploy", "/work/tree/rel_deploy"} {
		if !containsString(got, want) {
			t.Errorf("the list misses %s: %v", want, got)
		}
	}
}

func TestSSHIdentityFilesRefusesAConfigItCannotRead(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("NOTHING MEASURED: root reads a 0000 file")
	}
	f := newSSHFixture(t, t.TempDir())
	cfg := filepath.Join(f.home, ".ssh", "config")
	if err := os.Chmod(cfg, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := sshIdentityFiles(f.env()); err == nil || !strings.Contains(err.Error(), cfg) {
		t.Errorf("an unreadable ssh config rendered a wall anyway (err=%v): the keys it names would go unmasked", err)
	}
	// Absent is not an error: that is a box without ssh.
	if _, err := sshIdentityFiles(sshEnv{Home: filepath.Join(t.TempDir(), "nohome")}); err != nil {
		t.Errorf("a home with no ssh config refused: %v", err)
	}
}

func TestSSHIdentityIncludeLoopRefuses(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".ssh", "config"), "Include config\n", 0o600)
	if _, err := sshIdentityFiles(sshEnv{Home: home}); err == nil {
		t.Error("a self-including config was read to the end — the depth limit is gone")
	}
}

func TestBwrapMasksSSHKeysForEverySeatButThePusher(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		ag    *AgentFile
		masks bool
	}{
		{"push denied", &AgentFile{Deny: []string{"Bash(git push:*)"}}, true},
		{"push neither granted nor denied", &AgentFile{}, true},
		{"the pusher", &AgentFile{Allow: []string{"Bash(git push:*)"}}, false},
		{"granted and denied: deny wins", &AgentFile{Allow: []string{"Bash(git push:*)"}, Deny: []string{"Bash(git push:*)"}}, true},
		{"no PID", nil, true},
	} {
		if got := bwrapMasksSSHKeys(c.ag); got != c.masks {
			t.Errorf("%s: masks=%v, want %v", c.name, got, c.masks)
		}
	}
}

// The render: a push-denied PID's wall carries `--ro-bind /dev/null <key>`
// for every key the fixture config names, after every grant; the pusher's
// carries none of them.
func TestBwrapRenderMasksTheConfiguredSSHKeys(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	sf := newSSHFixture(t, filepath.Dir(f.home))
	t.Setenv("SSHFIX", sf.home)

	f.ag.Deny = []string{"Bash(git push:*)"}
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(r.Argv)
	lastBind := -1
	for i, a := range r.Argv {
		if a == "--bind" {
			lastBind = i
		}
	}
	// ~/.ssh is an allowlist (ranger-base-12ggv): an empty tmpfs after every
	// grant, then config and known_hosts bound back over it, in that order.
	sshDir := absResolve(filepath.Join(sf.home, ".ssh"))
	tmpfs := bwIndex(r.Argv, "--tmpfs", sshDir)
	if tmpfs < 0 {
		t.Errorf("a push-denied seat's wall mounts no tmpfs over %s, so a key no config names is readable (ranger-base-12ggv):\n  %s", sshDir, line)
	} else if tmpfs < lastBind {
		t.Errorf("the tmpfs over %s is mounted before a write grant, which can bind the host's back:\n  %s", sshDir, line)
	}
	for _, n := range sshDirAllowlist {
		p := filepath.Join(sshDir, n)
		if i := bwIndex(r.Argv, "--ro-bind", p, p); i < 0 || i < tmpfs {
			t.Errorf("%s is not bound back read-only AFTER the tmpfs over %s (at %d, tmpfs at %d):\n  %s", p, sshDir, i, tmpfs, line)
		}
	}
	for _, k := range sf.keys {
		k = absResolve(k)
		i := bwIndex(r.Argv, "--ro-bind", "/dev/null", k)
		if underDir(sshDir, k) {
			// Under the tmpfs: a /dev/null mount would only make its name.
			if i >= 0 {
				t.Errorf("%s is masked with /dev/null under the tmpfs that already hides it:\n  %s", k, line)
			}
			continue
		}
		if i < 0 {
			t.Errorf("a push-denied seat's wall does not mask %s, so it holds a key that pushes (ranger-base-ighvi):\n  %s", k, line)
		} else if i < lastBind {
			t.Errorf("the mask over %s is mounted before a write grant, which can bind it back:\n  %s", k, line)
		}
	}

	f.ag.Deny, f.ag.Allow = nil, []string{"Bash(git push:*)"}
	r, err = f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range sf.keys {
		if bwIndex(r.Argv, "--ro-bind", "/dev/null", absResolve(k)) >= 0 {
			t.Errorf("the pusher's wall masks %s: the one seat that pushes cannot", k)
		}
	}
	if i := bwIndex(r.Argv, "--tmpfs", sshDir); i >= 0 {
		t.Errorf("the pusher's wall hides %s: the one seat that pushes cannot read its keys", sshDir)
	}
	if len(r.Carve.SSHDir) != 0 {
		t.Errorf("the pusher's carve-out names ssh dirs %v", r.Carve.SSHDir)
	}
}

// ranger-base-12ggv: a file at ~/.ssh refuses the render — an empty tmpfs
// covers only a directory, and a hide that renders nothing is the silence the
// tier refuses. A ~/.ssh that is not there renders nothing and is no error.
func TestBwrapArgsRefusesAnSSHDirThatIsNotADirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	f := filepath.Join(root, "file", ".ssh")
	writeFile(t, f, "not a dir\n", 0o600)
	if _, _, err := BwrapArgs("", nil, SeatbeltCarveOut{SSHDir: []string{f}}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("a FILE at %s rendered a wall (err=%v): the keys beside it would stay readable", f, err)
	}
	absent := filepath.Join(root, "nohome", ".ssh")
	argv, _, err := BwrapArgs("", nil, SeatbeltCarveOut{SSHDir: []string{absent}})
	if err != nil {
		t.Errorf("an absent ~/.ssh refused: %v", err)
	}
	if bwIndex(argv, "--tmpfs", absent) >= 0 {
		t.Errorf("an absent ~/.ssh got a tmpfs, which bwrap would have to create: %v", argv)
	}
	// A config the carve-out read-denies by name is not bound back.
	dir := filepath.Join(root, "home", ".ssh")
	cfg := filepath.Join(dir, "config")
	writeFile(t, cfg, "Host x\n", 0o600)
	argv, _, err = BwrapArgs("", nil, SeatbeltCarveOut{SSHDir: []string{dir}, DenyRead: []string{cfg}})
	if err != nil {
		t.Fatal(err)
	}
	if bwIndex(argv, "--ro-bind", cfg, cfg) >= 0 {
		t.Errorf("the allowlist bound back %s, which DenyRead names — deny wins: %v", cfg, argv)
	}
}

// ranger-base-12ggv, the seatbelt half: the profile denies file-read* on the
// ~/.ssh subpath and only THEN allows config and known_hosts — SBPL is last
// match wins, so the order is the rule. The pusher's profile carries neither.
func TestRenderSeatbeltSSHDirIsADenyThenAllow(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	sf := newSSHFixture(t, filepath.Dir(f.home))
	t.Setenv("SSHFIX", sf.home)
	sshDir := absResolve(filepath.Join(sf.home, ".ssh"))
	profile := func(ag *AgentFile) string {
		t.Helper()
		p, err := f.a.RenderSeatbelt(ag, f.dir)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	f.ag.Deny = []string{"Bash(git push:*)"}
	prof := profile(f.ag)
	// The subpath line, inside a deny block (the passwd home's ~/.ssh may
	// precede it in the same block).
	deny := strings.Index(prof, "  (subpath "+sbQuote(sshDir)+")\n")
	if deny >= 0 {
		open := strings.LastIndex(prof[:deny], "(deny file-read*\n")
		if open < 0 || strings.Contains(prof[open:deny], ")\n(") {
			deny = -1 // not inside a file-read* deny block
		} else {
			deny = open
		}
	}
	if deny < 0 {
		t.Fatalf("a push-denied seat's profile does not read-deny the subpath %s (ranger-base-12ggv):\n%s", sshDir, prof)
	}
	allow := strings.Index(prof[deny:], "(allow file-read*\n")
	if allow < 0 {
		t.Fatalf("no file-read* allow follows the %s deny, so ssh's config is unreadable too:\n%s", sshDir, prof[deny:])
	}
	tail := prof[deny+allow:]
	for _, n := range sshDirAllowlist {
		if lit := "(literal " + sbQuote(filepath.Join(sshDir, n)) + ")"; !strings.Contains(tail, lit) {
			t.Errorf("%s is not re-allowed AFTER the subpath deny:\n%s", lit, prof[deny:])
		}
	}
	if strings.Contains(prof[:deny], "(allow file-read*") {
		t.Errorf("a file-read* allow precedes the %s deny, which would outvote nothing:\n%s", sshDir, prof)
	}
	var rep strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.dir, &rep); err != nil {
		t.Fatal(err)
	}
	if want := "r " + AbbrevHome(sshDir) + "/ (file-read* subpath deny"; !strings.Contains(rep.String(), want) {
		t.Errorf("the wall report does not name %s as read-denied:\n%s", sshDir, rep.String())
	}

	f.ag.Deny, f.ag.Allow = nil, []string{"Bash(git push:*)"}
	if prof := profile(f.ag); strings.Contains(prof, "(subpath "+sbQuote(sshDir)+")") {
		t.Errorf("the pusher's profile names %s: the one seat that pushes cannot read its keys", sshDir)
	}
}

// Live: the wall the launcher renders for a push-denied PID, run under bwrap,
// reads no byte of any configured key — and the same probe without the mask
// reads them all, so a green is not a probe that could never read.
func TestLiveBwrapPushDeniedSeatReadsNoConfiguredSSHKey(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	sf := newSSHFixture(t, filepath.Dir(f.home))
	t.Setenv("SSHFIX", sf.home)
	f.ag.Deny = []string{"Bash(git push:*)"}
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(sf.home, ".ssh", "config")
	known := filepath.Join(sf.home, ".ssh", "known_hosts")
	read := func(argv []string) map[string]string {
		t.Helper()
		script := `for p in "$@"; do printf '%s=' "$p"; cat -- "$p" 2>/dev/null | head -c 64 | tr -d '\n'; echo; done`
		args := append(append([]string{}, argv...), "sh", "-c", script, "sh")
		args = append(args, sf.keys...)
		args = append(args, sf.odd, cfg, known)
		out, err := exec.Command("bwrap", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("bwrap: %v\n%s", err, out)
		}
		res := map[string]string{}
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if k, v, ok := strings.Cut(l, "="); ok {
				res[k] = v
			}
		}
		return res
	}
	got := read(r.Argv)
	for _, k := range sf.keys {
		if got[k] != "" {
			t.Errorf("inside a push-denied seat's wall, %s read %q — want no bytes (ranger-base-ighvi)", k, got[k])
		}
	}
	// A key no config names, at no default name (ranger-base-nl2ha): the
	// allowlist over ~/.ssh covers it (ranger-base-12ggv). This arm was the
	// residual — it read PRIVATE — until the tmpfs landed.
	if got[sf.odd] != "" {
		t.Errorf("inside a push-denied seat's wall, %s (named by no config) read %q — want no bytes (ranger-base-12ggv)", sf.odd, got[sf.odd])
	}
	// The allowlist is an allowlist: ssh's config and known_hosts still read.
	if !strings.HasPrefix(got[cfg], "# a comment line") {
		t.Errorf("inside a push-denied seat's wall, %s read %q — the allowlist lost ssh's config", cfg, got[cfg])
	}
	if !strings.HasPrefix(got[known], "KNOWN") {
		t.Errorf("inside a push-denied seat's wall, %s read %q — the allowlist lost known_hosts", known, got[known])
	}

	// The control: the same probe, a wall with no read masks.
	ctl, _, err := BwrapArgs("", nil, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	got = read(ctl)
	for _, k := range append(append([]string{}, sf.keys...), sf.odd) {
		if !strings.HasPrefix(got[k], "PRIVATE") {
			t.Errorf("control: %s inside an unmasked wall read %q — the probe cannot read, so the arm above proves nothing", k, got[k])
		}
	}
}

// ranger-base-7m3up: the darwin arm of the same wall. ighvi masked the keys in
// the bwrap render only, so a seatbelt seat that was not the pusher read
// every one of them. The profile RenderSeatbelt writes for a push-denied PID
// names every configured key in its `(deny file-read*` block, the pusher's
// names none, and the report a seat's operator reads says which lines are ssh
// keys. Runs on every host: it pins the render, which is what a caged seat
// (which cannot nest sandbox-exec, ranger-base-xjw9) can still witness.
func TestRenderSeatbeltDeniesTheConfiguredSSHKeys(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	sf := newSSHFixture(t, filepath.Dir(f.home))
	t.Setenv("SSHFIX", sf.home)
	readBlock := func(ag *AgentFile) string {
		t.Helper()
		p, err := f.a.RenderSeatbelt(ag, f.dir)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		prof := string(b)
		i := strings.Index(prof, "(deny file-read*")
		if i < 0 {
			return ""
		}
		end := strings.Index(prof[i:], "\n)")
		if end < 0 {
			t.Fatalf("unterminated file-read* deny block:\n%s", prof[i:])
		}
		return prof[i : i+end]
	}

	f.ag.Deny = []string{"Bash(git push:*)"}
	block := readBlock(f.ag)
	for _, k := range sf.keys {
		if lit := "(literal " + sbQuote(absResolve(k)) + ")"; !strings.Contains(block, lit) {
			t.Errorf("a push-denied seat's seatbelt profile does not read-deny %s, so it holds a key that pushes (ranger-base-7m3up):\n%s", k, block)
		}
	}
	// Scope of THIS block: the literals are ssh's own resolution, so a file no
	// config names is not one of them — the ~/.ssh subpath deny after it is
	// what covers that file (ranger-base-12ggv, TestRenderSeatbeltSSHDirIsADenyThenAllow).
	if strings.Contains(block, sbQuote(absResolve(sf.odd))) {
		t.Errorf("scope: %s (named by no config) is a read-deny literal; the list is no longer ssh's own resolution", sf.odd)
	}
	var rep strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.dir, &rep); err != nil {
		t.Fatal(err)
	}
	for _, k := range sf.keys {
		if want := "r " + AbbrevHome(absResolve(k)) + " (file-read* deny — an ssh key"; !strings.Contains(rep.String(), want) {
			t.Errorf("the wall report does not name %s as a read-denied ssh key:\n%s", k, rep.String())
		}
	}

	f.ag.Deny, f.ag.Allow = nil, []string{"Bash(git push:*)"}
	block = readBlock(f.ag)
	for _, k := range sf.keys {
		if strings.Contains(block, sbQuote(absResolve(k))) {
			t.Errorf("the pusher's seatbelt profile read-denies %s: the one seat that pushes cannot", k)
		}
	}
}

// Live on darwin: under the profile RenderSeatbelt writes for a push-denied
// PID, sandbox-exec reads no configured key, and the same profile with its
// read deny stripped reads them all — so a green is not a probe that could
// never read. Skips where sandbox-exec cannot apply (Linux, a caged seat).
func TestLiveSeatbeltPushDeniedSeatReadsNoConfiguredSSHKey(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	sbSkipUnlessSandboxable(t)
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	sf := newSSHFixture(t, filepath.Dir(f.home))
	t.Setenv("SSHFIX", sf.home)
	f.ag.Deny = []string{"Bash(git push:*)"}
	walled, err := f.a.RenderSeatbelt(f.ag, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range sf.keys {
		if crdRun(t, walled, k) {
			t.Errorf("inside a push-denied seat's seatbelt profile, %s was READABLE (ranger-base-7m3up)", k)
		}
	}
	// A key no config names (ranger-base-nl2ha): the ~/.ssh subpath deny
	// covers it (ranger-base-12ggv), and the allows after it keep the config.
	if crdRun(t, walled, sf.odd) {
		t.Errorf("inside a push-denied seat's seatbelt profile, %s (named by no config) was READABLE (ranger-base-12ggv)", sf.odd)
	}
	if cfg := filepath.Join(sf.home, ".ssh", "config"); !crdRun(t, walled, cfg) {
		t.Errorf("inside a push-denied seat's seatbelt profile, %s was refused — the allow after the subpath deny is not outvoting it", cfg)
	}
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w)
	c.DenyRead, c.SSHDir = nil, nil
	control := sbRenderProfile(t, "control.sb", SeatbeltProfile(f.ag.Name, w, nil, c))
	for _, k := range append(append([]string{}, sf.keys...), sf.odd) {
		if !crdRun(t, control, k) {
			t.Errorf("control: %s was refused with no read deny — the probe cannot read, so the arm above proves nothing", k)
		}
	}
}
