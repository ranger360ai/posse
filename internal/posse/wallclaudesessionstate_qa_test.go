//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-5632o: claude's shell-snapshots/ and session-env/ are
// run by every claude seat ahead of its Bash commands, and the shared config
// dir grant let one seat write another's. Each seat now gets a private copy
// of both, bound over the shared one (wallclaudesessionstate.go).
//
// Same split as the 6akzw pins: render pins from any host, and a live pin
// that hands the render to the real bwrap and tries the plant.
//
//	go test ./internal/posse -run 'WallClaudeSessionState' -v

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// cssFixture is weFixture plus another seat's live session state in the
// shared config dir, as a box with two claude seats has it.
func cssFixture(t *testing.T) (f weFixture, peerSnap, peerHook string) {
	t.Helper()
	f = weFixtureIn(t, wallUngrantedDir(t))
	peerSnap = filepath.Join(f.cfg, "shell-snapshots", "snapshot-bash-1-peer.sh")
	peerHook = filepath.Join(f.cfg, "session-env", "peer-session", "sessionstart-hook-0.sh")
	for p, body := range map[string]string{
		peerSnap: "# the peer seat's shell snapshot\n",
		peerHook: "# the peer seat's hook env\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f, peerSnap, peerHook
}

// bindIndex is the argv index of `--bind src dst`, or -1.
func bindIndex(argv []string, src, dst string) int {
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == "--bind" && argv[i+1] == src && argv[i+2] == dst {
			return i
		}
	}
	return -1
}

// The render: a claude seat binds its own dir over each member, AFTER the
// config-dir grant it narrows, and a seat on another runtime binds nothing.
func TestQAWallClaudeSessionStateRendersPrivateBindsOverTheGrant(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f, _, _ := cssFixture(t)
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	cfg := absResolve(f.cfg)
	grant := bindIndex(r.Argv, cfg, cfg)
	if grant < 0 {
		t.Fatalf("the fixture's config dir is not granted, so nothing here is measured: %q", r.Argv)
	}
	own := SeatCacheDir(f.home, f.dir)
	for _, n := range claudeSessionStateDirNames {
		src, dst := filepath.Join(own, claudePrivateSessionStateRel, n), filepath.Join(cfg, n)
		i := bindIndex(r.Argv, src, dst)
		switch {
		case i < 0:
			t.Errorf("no private bind over %s — every claude seat can write what the next one runs ahead of Bash (ranger-base-5632o)\n argv: %q", dst, r.Argv)
		case i < grant:
			t.Errorf("the private bind over %s comes before the config-dir grant at %d, so the grant covers it again", dst, grant)
		}
	}
	// debug/ is the one member SeatbeltCarveOut also DENIES, because seatbelt
	// cannot bind (claudeDebugDir, ranger-base-ntt1w). At this mechanism the
	// deny has to come back out: the deny step mounts after the private one,
	// so a read-only bind from the host would put the shared directory back
	// over the seat's own copy and undo the bind the loop above just checked.
	dbg := filepath.Join(cfg, claudeDebugDirName)
	if containsString(r.Carve.Deny, dbg) {
		t.Errorf("%s is still in the bwrap carve-out's Deny, so the deny step re-binds the SHARED dir over the seat's private copy of it (ranger-base-ntt1w)\n deny: %v", dbg, r.Carve.Deny)
	}

	other := &Runtime{Name: "codex", StateDirs: []string{"~/.codex"}}
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	ro, err := f.a.bwrapRender(f.ag, other, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	// A codex seat has private mounts of its own since ranger-base-6ob20
	// (codexEphemeralDirs), in its own home; none may land in claude's.
	for _, pb := range ro.Carve.Private {
		if underDir(cfg, pb.Dst) {
			t.Errorf("a codex seat gets a private bind into claude's config dir, which nothing granted it: %v", pb)
		}
	}
}

// A link at a member's name refuses the render. Every seat can write the
// parent, so the link may be another seat's, and bwrap would mount where it
// points.
func TestQAWallClaudeSessionStateRefusesALinkAtTheMember(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	grant := filepath.Join(d, "cfg")
	elsewhere := filepath.Join(d, "elsewhere")
	for _, p := range []string{grant, elsewhere} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(grant, "shell-snapshots")
	if err := os.Symlink(elsewhere, dst); err != nil {
		t.Fatal(err)
	}
	carve := SeatbeltCarveOut{Private: []PrivateBind{{Src: filepath.Join(d, "seat"), Dst: dst}}}
	if _, _, err := BwrapArgs(d, []string{grant}, carve); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("a link at the member rendered (err %v), so bwrap would mount where another seat pointed it", err)
	}
	// And an absent member and source become prep, in that order.
	carve.Private[0].Dst = filepath.Join(grant, "session-env")
	_, prep, err := BwrapArgs(d, []string{grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	var made []string
	for _, s := range prep {
		if s.Kind == PrepMkdirAll {
			made = append(made, s.Path)
		}
	}
	if !containsString(made, carve.Private[0].Dst) || !containsString(made, carve.Private[0].Src) {
		t.Errorf("prep %v does not make both the absent member and its source", made)
	}
}

// The live join: the rendered wall, the real bwrap, another seat's snapshot
// and hook env planted at, and the seat's own snapshot written and sourced.
func TestLiveWallClaudeSessionStateRefusesPeerPlants(t *testing.T) {
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
	f, peerSnap, peerHook := cssFixture(t)
	// The ranger-base-fswhs members: another seat's .claude.json backup with
	// a far-future stamp, and a checkpoint in its file-history tree.
	backups, hist := filepath.Join(f.cfg, "backups"), filepath.Join(f.cfg, "file-history")
	peerBackup := filepath.Join(backups, ".claude.json.backup.99999999999999")
	peerHist := filepath.Join(hist, "00000000-0000-4000-8000-00000000peer", "0123456789abcdef@v1")
	// The ranger-base-ntt1w member: an uncaged claude's live debug log, and
	// the file outside the wall a planted link would redirect it into.
	dbg := filepath.Join(f.cfg, claudeDebugDirName)
	peerDebug := filepath.Join(dbg, "00000000-0000-4000-8000-00000000peer.txt")
	target := filepath.Join(wallUngrantedDir(t), "redirect-target")
	for p, body := range map[string]string{
		peerBackup: "{\"peer\": \"backup\"}\n",
		peerHist:   "the peer seat's checkpoint\n",
		peerDebug:  "the uncaged seat's debug log\n",
		target:     "a file outside every wall\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
	snaps, envs := filepath.Join(f.cfg, "shell-snapshots"), filepath.Join(f.cfg, "session-env")
	before := map[string][]byte{}
	for _, p := range []string{peerSnap, peerHook, peerBackup, peerHist, peerDebug, target} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[p] = b
	}

	// Another seat's state is not even visible. Asked FIRST, because a plant
	// below creates the peer's name in this seat's own private dir.
	for name, script := range map[string]string{
		"peer snapshot listed": "ls " + q(snaps) + " | grep -q peer",
		"peer hook env listed": "test -e " + q(filepath.Dir(peerHook)),
		"peer backup listed":   "ls -a " + q(backups) + " | grep -q 99999999999999",
		"peer checkpoint seen": "test -e " + q(filepath.Dir(peerHist)),
		"peer debug log seen":  "test -e " + q(peerDebug),
	} {
		if ok, out := try(script); ok {
			t.Errorf("%s: another seat's session state is visible to this one\n script: %s\n out: %s", name, script, out)
		}
	}

	// Each plant, tried whether or not the shell reports an error: an append
	// to a name the seat cannot see CREATES a file in its private dir, so the
	// result that counts is the peer's file on the host, read below.
	for name, script := range map[string]string{
		"peer snapshot append":   "echo PLANT >> " + q(peerSnap),
		"peer snapshot replace":  "echo PLANT > " + q(snaps+"/x.tmp") + " && mv " + q(snaps+"/x.tmp") + " " + q(peerSnap),
		"peer hook env append":   "mkdir -p " + q(filepath.Dir(peerHook)) + " && echo PLANT >> " + q(peerHook),
		"snapshots dir moved":    "mv " + q(snaps) + " " + q(snaps+".old") + " && mkdir " + q(snaps) + " && echo PLANT > " + q(peerSnap),
		"session-env dir moved":  "mv " + q(envs) + " " + q(envs+".old"),
		"config dir moved":       "mv " + q(f.cfg) + " " + q(f.cfg+".old"),
		"peer backup replace":    "echo PLANT > " + q(peerBackup),
		"peer checkpoint append": "mkdir -p " + q(filepath.Dir(peerHist)) + " && echo PLANT >> " + q(peerHist),
		"backups dir moved":      "mv " + q(backups) + " " + q(backups+".old"),
		"file-history dir moved": "mv " + q(hist) + " " + q(hist+".old"),
		// ranger-base-ntt1w: an uncaged claude reopens its debug log BY PATH
		// on every write and follows a link there, so a link left at the
		// live log's name redirects the rest of it; a link at <cfg>/debug
		// itself redirects every seat's logs at once.
		"peer debug log append":  "echo PLANT >> " + q(peerDebug),
		"peer debug log relink":  "rm -f " + q(peerDebug) + " && ln -s " + q(target) + " " + q(peerDebug),
		"debug log name created": "ln -s " + q(target) + " " + q(filepath.Join(dbg, "00000000-0000-4000-8000-0000000000ab.txt")),
		"debug dir moved":        "mv " + q(dbg) + " " + q(dbg+".old"),
		"debug dir relinked":     "rm -rf " + q(dbg) + " && ln -s " + q(filepath.Dir(target)) + " " + q(dbg),
		"snapshots dir via link": "ln -s " + q(snaps) + " " + q(filepath.Join(f.dir, "s")) + " && echo PLANT >> " + q(filepath.Join(f.dir, "s", filepath.Base(peerSnap))),
	} {
		_, out := try(script)
		for p, b := range before {
			if got, err := os.ReadFile(p); err != nil || string(got) != string(b) {
				t.Errorf("%s: the peer's %s changed on the host (%q, %v) — a snapshot or hook env runs ahead of the peer's next Bash command (ranger-base-5632o); a backup or checkpoint is what the peer restores or rewinds to (ranger-base-fswhs)\n script: %s\n out: %s", name, filepath.Base(p), got, err, script, out)
				_ = os.WriteFile(p, b, 0o644)
			}
		}
		if _, err := os.Stat(snaps + ".old"); err == nil {
			t.Errorf("%s: the shared shell-snapshots dir was renamed out from under the mount", name)
		}
		// ranger-base-ntt1w, the shape the bytes above cannot state: what an
		// uncaged claude opens must still be a FILE and a DIRECTORY, and no
		// new name may appear under the shared debug dir for its next
		// session to open. A link that nothing has been written through yet
		// leaves every byte on the host as it was.
		if fi, err := os.Lstat(peerDebug); err != nil || fi.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s: the shared debug log is now %v (%v) — claude reopens it by path and follows a link, so the rest of that session's log lands wherever this points (ranger-base-ntt1w)\n script: %s\n out: %s", name, fi, err, script, out)
			_ = os.Remove(peerDebug)
			_ = os.WriteFile(peerDebug, before[peerDebug], 0o644)
		}
		if fi, err := os.Lstat(dbg); err != nil || !fi.IsDir() {
			t.Errorf("%s: the shared debug dir is now %v (%v) — every debug-on claude on the box writes its log through it (ranger-base-ntt1w)\n script: %s\n out: %s", name, fi, err, script, out)
		}
		for _, p := range []string{dbg + ".old", filepath.Join(dbg, "00000000-0000-4000-8000-0000000000ab.txt")} {
			if _, err := os.Lstat(p); err == nil {
				t.Errorf("%s: %s exists on the host — the seat reached the SHARED debug dir (ranger-base-ntt1w)\n script: %s\n out: %s", name, p, script, out)
				_ = os.Remove(p)
			}
		}
	}

	// The seat's own: claude's writes, then a LATER shell sourcing the
	// snapshot, which is what the Bash tool does on every command. Both land
	// in the seat's own dir on the host and never in the shared one.
	//
	// A SLICE, not a map: "own snapshot sourced" reads the file "own snapshot
	// write" creates, and map iteration order is random — as a map this block
	// sourced an absent file in 6/40, 2/20 and 3/27 runs, and reddened arm 1
	// four times (ranger-base-l1d9i). The plant loops above are maps because
	// each of their steps is independent.
	own := filepath.Join(SeatCacheDir(f.home, f.dir), claudePrivateSessionStateRel)
	mine := filepath.Join(snaps, "snapshot-bash-2-mine.sh")
	for _, step := range []struct{ name, script string }{
		{"own snapshot write", "echo 'MARK=sourced' > " + q(mine)},
		{"own snapshot sourced", ". " + q(mine) + " && test \"$MARK\" = sourced"},
		{"own backup write", "echo '{}' > " + q(filepath.Join(backups, ".claude.json.backup.1"))},
		{"own checkpoint write", "mkdir -p " + q(filepath.Join(hist, "mine")) + " && : > " + q(filepath.Join(hist, "mine", "0123456789abcdef@v1"))},
		{"own hook env write", "mkdir -p " + q(filepath.Join(envs, "mine")) + " && : > " + q(filepath.Join(envs, "mine", "sessionstart-hook-0.sh"))},
		// The seat's own claude keeps its debug log, which is why debug/ is a
		// private bind here and a flat write deny at seatbelt (where the log
		// is lost and the turn still completes — claudeDebugDir's MEASURED).
		{"own debug log write", "echo 'debug line' > " + q(filepath.Join(dbg, "11111111-1111-4111-8111-111111111111.txt"))},
		{"own debug latest relink", "ln -sf " + q(filepath.Join(dbg, "11111111-1111-4111-8111-111111111111.txt")) + " " + q(filepath.Join(dbg, "latest"))},
		// A sibling member outside both private binds still writes. Not
		// projects/ any more: that is read-only but for the own key since
		// ranger-base-ajkqx, and this fixture's cwd is no session worktree.
		{"config dir still writes", ": > " + q(filepath.Join(f.cfg, "x"))},
	} {
		name, script := step.name, step.script
		if ok, out := try(script); !ok {
			t.Errorf("%s: REFUSED under the rendered wall — claude needs this\n script: %s\n out: %s", name, script, out)
		}
	}
	if _, err := os.Stat(filepath.Join(own, "shell-snapshots", filepath.Base(mine))); err != nil {
		t.Errorf("the seat's own snapshot is not in its private dir on the host: %v", err)
	}
	if _, err := os.Stat(mine); err == nil {
		t.Errorf("the seat's own snapshot landed in the SHARED dir %s", snaps)
	}
	for _, rel := range []string{
		filepath.Join("backups", ".claude.json.backup.1"),
		filepath.Join("file-history", "mine"),
		filepath.Join(claudeDebugDirName, "11111111-1111-4111-8111-111111111111.txt"),
		filepath.Join(claudeDebugDirName, "latest"),
	} {
		if _, err := os.Lstat(filepath.Join(own, rel)); err != nil {
			t.Errorf("the seat's own %s is not in its private dir on the host: %v", rel, err)
		}
		if _, err := os.Lstat(filepath.Join(f.cfg, rel)); err == nil {
			t.Errorf("the seat's own %s landed in the SHARED config dir", rel)
		}
	}
}

// The seatbelt half of the debug/ carve (ranger-base-ntt1w). A profile cannot
// bind, so what bwrap says with a private copy this says with a write deny
// over the whole directory: the seat may neither write a log there nor create
// or unlink a NAME under it, which is what a planted link would be. Rendered
// from any host; whether sandbox-exec honours it is the verify lane's, the
// same split the projects pins take.
//
//	go test ./internal/posse -run 'WallClaudeSessionStateSeatbelt' -v
func TestQAWallClaudeSessionStateSeatbeltDeniesTheSharedDebugDir(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := cpFixtureIn(t, wallUngrantedDir(t))
	cfg := absResolve(f.cfg)
	want := "(subpath " + sbQuote(filepath.Join(cfg, claudeDebugDirName)) + ")"

	prof, c := sbRender(t, f, f.rt, f.dir)
	if sbLine(prof, "(subpath "+sbQuote(cfg)+")") < 0 {
		t.Fatalf("the fixture's config dir is not in the allow block, so nothing here is measured:\n%s", prof)
	}
	carve := sbLine(prof, "(deny file-write*")
	line := sbLine(prof, want)
	switch {
	case carve < 0:
		t.Errorf("the fixture rendered no carve-out deny block, so nothing here is measured\n%s", prof)
	case line < 0:
		t.Errorf("no write deny over %s — an uncaged claude opens <cfg>/debug/<session id>.txt by path and follows a link, so any caged seat can redirect its log (ranger-base-ntt1w)\n%s", want, prof)
	case line < carve:
		t.Errorf("%s renders at line %d, before the carve-out's deny block at %d, so it is an ALLOW and not the deny this bead needs\n%s", want, line, carve, prof)
	}
	if !containsString(c.Deny, filepath.Join(cfg, claudeDebugDirName)) {
		t.Errorf("the carve-out's Deny does not name the shared debug dir: %v", c.Deny)
	}

	// A seat on another runtime is granted no claude config dir, so it gets
	// no line for one — the guard claudeDebugDir shares with every other
	// member of this carve.
	other := &Runtime{Name: "codex", StateDirs: []string{"~/.codex"}}
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if oprof, _ := sbRender(t, f, other, f.dir); sbLine(oprof, want) >= 0 {
		t.Errorf("a codex seat's profile names %s, a directory nothing granted it\n%s", want, oprof)
	}
}

// The live sessions/ pin (ranger-base-lx5jk): the rendered wall, the real
// bwrap, a bystander's row and peer-token key planted in the SHARED
// sessions/, and the seat's own registration.
//
// THE PROTECTED PROCESSES. An uncaged claude's peer discovery reads every row
// in the shared sessions/ and connects to its messagingSocketPath, sending
// the key file's token; a seat that can repoint a row or write a key gets the
// uncaged session's messages and answers into its context. And sentline's
// echo verdict reads the sessionId posse's rex join takes from the row, so a
// rewritten row misdirects it. Both are protected by `sessions` being in
// claudeSessionStateDirNames: drop the name and every plant below lands on
// the host, and the seat's own row lands in the shared dir.
func TestLiveWallClaudeSessionStateKeepsThePeerRegistryPrivate(t *testing.T) {
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
	f, _, _ := cssFixture(t)
	sess := filepath.Join(f.cfg, claudeSessionsDir)
	peerRow := filepath.Join(sess, "99999.json")
	peerKey := filepath.Join(sess, "99999."+strings.Repeat("ab", 32)+".key")
	elsewhere := wallUngrantedDir(t)
	for p, body := range map[string]string{
		peerRow: `{"pid":99999,"sessionId":"00000000-0000-4000-8000-00000000peer","procStart":"1","messagingSocketPath":"/run/user/1/peer.sock"}` + "\n",
		peerKey: "the uncaged peer's token\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
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
	before := map[string][]byte{}
	for _, p := range []string{peerRow, peerKey} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[p] = b
	}
	names := func() []string {
		ents, _ := os.ReadDir(sess)
		var out []string
		for _, e := range ents {
			out = append(out, e.Name())
		}
		return out
	}
	shared := names()
	const protects = "an uncaged claude's peer discovery connects to a row's socket with its key's token, and sentline's echo verdict reads posse's join of the row (ranger-base-lx5jk)"

	// The peer is not visible. Asked FIRST: a plant below creates the
	// peer's name in this seat's own private dir.
	if ok, out := try("ls -a " + q(sess) + " | grep -q 99999"); ok {
		t.Errorf("the bystander's row is visible to the seat, so it can read a peer's token or socket — %s\n out: %s", protects, out)
	}

	for name, script := range map[string]string{
		"peer row repointed":      "echo '{\"pid\":99999,\"messagingSocketPath\":\"" + f.dir + "/evil.sock\"}' > " + q(peerRow),
		"peer row replaced":       "echo '{}' > " + q(sess+"/x.tmp") + " && mv " + q(sess+"/x.tmp") + " " + q(peerRow),
		"peer key rewritten":      "echo FORGED > " + q(peerKey),
		"live-looking row added":  "echo '{\"pid\":1,\"sessionId\":\"x\",\"pidDomain\":\"elsewhere\"}' > " + q(filepath.Join(sess, "1.json")),
		"sessions dir moved":      "mv " + q(sess) + " " + q(sess+".old") + " && mkdir " + q(sess),
		"sessions dir relinked":   "rm -rf " + q(sess) + " && ln -s " + q(elsewhere) + " " + q(sess),
		"sessions dir via a link": "ln -s " + q(sess) + " " + q(filepath.Join(f.dir, "s")) + " && echo FORGED > " + q(filepath.Join(f.dir, "s", filepath.Base(peerKey))),
	} {
		_, out := try(script)
		for p, b := range before {
			if got, err := os.ReadFile(p); err != nil || string(got) != string(b) {
				t.Errorf("%s: the shared %s changed on the host (%q, %v) — %s\n script: %s\n out: %s", name, filepath.Base(p), got, err, protects, script, out)
				_ = os.WriteFile(p, b, 0o600)
			}
		}
		if fi, err := os.Lstat(sess); err != nil || !fi.IsDir() {
			t.Fatalf("%s: the shared sessions/ is now %v (%v) — %s\n script: %s\n out: %s", name, fi, err, protects, script, out)
		}
		if got := names(); strings.Join(got, ",") != strings.Join(shared, ",") {
			t.Errorf("%s: the shared sessions/ holds %v, was %v — a new row there reads as a live peer to every claude on the box; %s\n script: %s\n out: %s", name, got, shared, protects, script, out)
		}
		_ = os.RemoveAll(sess + ".old")
		_ = os.Remove(filepath.Join(f.dir, "s"))
	}

	// The seat's own registration: a process under the wall writes its row
	// as claude does — its own pid (the host's: no --unshare-pid) and the
	// kernel's start time — and stays alive so the fence can be taken.
	own := filepath.Join(SeatCacheDir(f.home, f.dir), claudePrivateSessionStateRel, claudeSessionsDir)
	const ownSession = "22222222-2222-4222-8222-222222222222"
	reg := `st=$(cut -d' ' -f22 /proc/$$/stat) && printf '{"pid":%s,"sessionId":"` + ownSession + `","procStart":"%s"}' "$$" "$st" > ` + q(sess) + `/$$.json && exec sleep 60`
	cmd := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", reg)...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	var row string
	for deadline := time.Now().Add(10 * time.Second); row == "" && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		ents, _ := os.ReadDir(own)
		for _, e := range ents {
			if b, err := os.ReadFile(filepath.Join(own, e.Name())); err == nil && strings.Contains(string(b), ownSession) && strings.HasSuffix(string(b), "}") {
				row = e.Name()
			}
		}
	}
	if row == "" {
		t.Fatalf("the seat's own row never landed in its private dir %s — %s; shared dir holds %v", own, protects, names())
	}
	pid, err := strconv.Atoi(strings.TrimSuffix(row, ".json"))
	if err != nil {
		t.Fatalf("own row %q is not named by a pid", row)
	}
	if _, err := os.Stat(filepath.Join(sess, row)); err == nil {
		t.Errorf("the seat's own row landed in the SHARED sessions/ — %s", protects)
	}
	// A host-side row in the shared dir for the same pid, fenced right but
	// naming another conversation: the join must still answer the seat's own.
	start, err := procStartOf(pid)
	if err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(sess, row)
	if err := os.WriteFile(decoy, []byte(`{"pid":`+strconv.Itoa(pid)+`,"sessionId":"33333333-3333-4333-8333-333333333333","procStart":"`+start+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(decoy)
	got, err := claudeSessionOfPID(filepath.Join(f.cfg, claudeHistoryFile), f.home, pid)
	if err != nil || got != ownSession {
		t.Errorf("the rex join answered %q, %v for the caged seat's pid %d, want its own %q from the private row — sentline's echo verdict reads whichever conversation this names", got, err, pid, ownSession)
	}
}

// ─── history.jsonl and paste-cache/ (ranger-base-a544g) ──────────────────────

// The render: a claude seat's history.jsonl is a FILE bind of its own private
// log over the shared one, after the config-dir grant, and the launcher makes
// both ends as empty 0600 files when they are absent (a mount needs a name to
// cover). paste-cache/ is a member of claudeSessionStateDirNames, which the
// dir-bind pin above walks.
func TestQAWallClaudeSessionStateRendersTheHistoryFileBind(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f, _, _ := cssFixture(t)
	if !containsString(claudeSessionStateDirNames, "paste-cache") {
		t.Errorf("paste-cache is not a private member %v — every seat can rewrite a paste the operator's prompt recall fills in (ranger-base-a544g)", claudeSessionStateDirNames)
	}
	shared := filepath.Join(f.cfg, claudeHistoryFile)
	if err := os.Remove(shared); err != nil {
		t.Fatal(err)
	}
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	cfg := absResolve(f.cfg)
	grant := bindIndex(r.Argv, cfg, cfg)
	if grant < 0 {
		t.Fatalf("the fixture's config dir is not granted, so nothing here is measured: %q", r.Argv)
	}
	src, dst := claudePrivateHistoryPath(f.home, f.dir), filepath.Join(cfg, claudeHistoryFile)
	switch i := bindIndex(r.Argv, src, dst); {
	case i < 0:
		t.Fatalf("no private file bind of %s over %s — every seat can plant a row the operator's prompt recall submits (ranger-base-a544g)\n argv: %q", src, dst, r.Argv)
	case i < grant:
		t.Errorf("the history bind comes before the config-dir grant at %d, so the grant covers it again", grant)
	}
	var points []string
	for _, s := range r.Prep {
		if s.Kind == PrepMaskPoint {
			points = append(points, s.Path)
		}
	}
	if !containsString(points, dst) || !containsString(points, src) {
		t.Fatalf("prep %v does not make both the absent shared log and the private one as empty files", points)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dst, src} {
		fi, err := os.Lstat(p)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() != 0 || fi.Mode().Perm() != 0o600 {
			t.Errorf("after prep %s is %v (%v), want an empty regular 0600 file — claude's own mode for its history", p, fi, err)
		}
	}
}

// A link, a dir or a fifo at <cfg>/history.jsonl refuses the render, and so
// does one at the seat's own private log. Every seat can write the config
// dir, and a seat can write its own cache dir; bwrap would mount on wherever
// a link points, and an uncaged claude's append follows a link there
// (MEASURED, ranger-base-gab5i). Protects: the mount from landing on a target
// a seat chose.
func TestQAWallClaudeSessionStateRefusesALinkAtHistory(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f, _, _ := cssFixture(t)
	shared := filepath.Join(f.cfg, claudeHistoryFile)
	priv := claudePrivateHistoryPath(f.home, f.dir)
	target := filepath.Join(wallUngrantedDir(t), "victim")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(priv), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		at    string
		plant func(string) error
	}{
		{"a link at the shared log", shared, func(p string) error { return os.Symlink(target, p) }},
		{"a dir at the shared log", shared, func(p string) error { return os.Mkdir(p, 0o700) }},
		{"a fifo at the shared log", shared, func(p string) error { return syscall.Mkfifo(p, 0o600) }},
		{"a link at the seat's private log", priv, func(p string) error { return os.Symlink(target, p) }},
	} {
		_ = os.RemoveAll(shared)
		_ = os.RemoveAll(priv)
		if err := os.WriteFile(shared, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(c.at)
		if err := c.plant(c.at); err != nil {
			t.Fatal(err)
		}
		if _, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates); err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("%s rendered (err %v), so bwrap would mount where a seat pointed it", c.name, err)
		}
	}
}

// The live pin: the rendered wall, the real bwrap, a row planted in the SHARED
// history.jsonl and a file in the shared paste-cache/, and the seat's own
// submit.
//
// THE PROTECTED PROCESSES. The operator's prompt recall in an uncaged claude
// loads the shared log's rows for its cwd and fills pastes from the row or
// the cache without re-hashing, so a seat that writes either changes what
// the operator submits under their own name. And sentline's echo verdict
// reads the last row for a seat's session, so a planted row turns a held
// draft into an "echo" posse types over. Both are protected by the file bind
// and by paste-cache being a private member: drop either and the plants
// below land on the host.
//
// THE DRIVER is a shell appending a row by path, as claude does (a locked
// O_APPEND of one JSON line; design ranger-base-tgc4b, static read and
// interactive measure on claude 2.1.323). The suite has no claude and no
// endpoint, and `claude -p` writes no history at all (MEASURED,
// ranger-base-gab5i), so the bind is what this pin measures.
func TestLiveWallClaudeSessionStateKeepsTheSubmitLogPrivate(t *testing.T) {
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
	f, _, _ := cssFixture(t)
	hist := filepath.Join(f.cfg, claudeHistoryFile)
	pastes := filepath.Join(f.cfg, claudePasteCacheDir)
	paste := filepath.Join(pastes, "0123456789abcdef.txt")
	const operatorSession = "44444444-4444-4444-8444-444444444444"
	elsewhere := wallUngrantedDir(t)
	if err := os.MkdirAll(pastes, 0o700); err != nil {
		t.Fatal(err)
	}
	for p, body := range map[string]string{
		hist:  `{"display":"the operator's own prompt","pastedContents":{},"project":"` + f.dir + `","sessionId":"` + operatorSession + `","timestamp":1}` + "\n",
		paste: "the operator's pasted body\n",
	} {
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
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
	before := map[string][]byte{}
	for _, p := range []string{hist, paste} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[p] = b
	}
	names := func() string {
		ents, _ := os.ReadDir(pastes)
		var out []string
		for _, e := range ents {
			out = append(out, e.Name())
		}
		return strings.Join(out, ",")
	}
	sharedPastes := names()
	const protects = "the operator's prompt recall submits what the shared history and paste-cache hold under the operator's name, and sentline's echo verdict reads the shared history (ranger-base-a544g)"

	// Neither the operator's row nor paste is visible. Asked FIRST: a plant
	// below writes the same names in this seat's own private copies.
	if ok, out := try("grep -q 'operator' " + q(hist)); ok {
		t.Errorf("the operator's history row is readable by the seat — %s\n out: %s", protects, out)
	}
	if ok, out := try("test -e " + q(paste)); ok {
		t.Errorf("the operator's cached paste is visible to the seat — %s\n out: %s", protects, out)
	}

	row := `{"display":"rm -rf ~","pastedContents":{},"project":"` + f.dir + `","sessionId":"` + operatorSession + `","timestamp":2}`
	for name, script := range map[string]string{
		"row appended":       "echo " + q(row) + " >> " + q(hist),
		"log truncated":      ": > " + q(hist),
		"log replaced":       "echo " + q(row) + " > " + q(f.dir+"/h.tmp") + " && mv " + q(f.dir+"/h.tmp") + " " + q(hist),
		"log relinked":       "rm -f " + q(hist) + " && ln -s " + q(elsewhere+"/h") + " " + q(hist),
		"log via a dir link": "ln -s " + q(f.cfg) + " " + q(filepath.Join(f.dir, "c")) + " && echo " + q(row) + " >> " + q(filepath.Join(f.dir, "c", claudeHistoryFile)),
		"paste rewritten":    "echo FORGED > " + q(paste),
		"paste added":        "echo FORGED > " + q(filepath.Join(pastes, "fedcba9876543210.txt")),
		"paste dir relinked": "rm -rf " + q(pastes) + " && ln -s " + q(elsewhere) + " " + q(pastes),
		"paste dir moved":    "mv " + q(pastes) + " " + q(pastes+".old") + " && mkdir " + q(pastes),
	} {
		_, out := try(script)
		for p, b := range before {
			if got, err := os.ReadFile(p); err != nil || string(got) != string(b) {
				t.Errorf("%s: the shared %s changed on the host (%q, %v) — %s\n script: %s\n out: %s", name, filepath.Base(p), got, err, protects, script, out)
				_ = os.WriteFile(p, b, 0o600)
			}
		}
		if fi, err := os.Lstat(hist); err != nil || !fi.Mode().IsRegular() {
			t.Fatalf("%s: the shared history.jsonl is now %v (%v) — %s\n script: %s\n out: %s", name, fi, err, protects, script, out)
		}
		if got := names(); got != sharedPastes {
			t.Errorf("%s: the shared paste-cache holds %s, was %s — %s\n script: %s\n out: %s", name, got, sharedPastes, protects, script, out)
		}
		_ = os.RemoveAll(pastes + ".old")
		_ = os.Remove(filepath.Join(f.dir, "c"))
	}

	// The seat's own submit: a process under the wall appends its row as
	// claude does, then stays alive so posse's read can take its pid's cwd
	// (the host's pid: the render has no --unshare-pid).
	priv := claudePrivateHistoryPath(f.home, f.dir)
	const ownSession = "55555555-5555-4555-8555-555555555555"
	const ownLine = "the seat's own submit"
	sub := `echo ` + q(`{"display":"`+ownLine+`","pastedContents":{},"project":"x","sessionId":"`+ownSession+`","timestamp":3}`) + ` >> ` + q(hist) + ` && echo $$ && exec sleep 60`
	cmd := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", sub)...)
	var errb strings.Builder
	cmd.Stderr = &errb
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	got := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		got <- line
	}()
	pid := 0
	select {
	case line := <-got:
		pid, _ = strconv.Atoi(strings.TrimSpace(line))
	case <-time.After(10 * time.Second):
	}
	if pid == 0 {
		t.Fatalf("the seat's submit never ran under the wall: %s", errb.String())
	}
	if b, err := os.ReadFile(priv); err != nil || !strings.Contains(string(b), ownLine) {
		t.Fatalf("the seat's own submit is not in its private log %s (%q, %v) — %s", priv, b, err, protects)
	}
	if b, _ := os.ReadFile(hist); strings.Contains(string(b), ownLine) {
		t.Errorf("the seat's own submit landed in the SHARED history.jsonl — %s", protects)
	}
	// A host-side row in the shared log for the seat's session, naming another
	// line: the echo read must still answer the seat's own.
	decoy := `{"display":"a host-side decoy","sessionId":"` + ownSession + `","timestamp":4}` + "\n"
	if err := os.WriteFile(hist, append(before[hist], decoy...), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := lastSubmittedOfPID(hist, f.home, pid, ownSession); !ok || got != ownLine {
		t.Errorf("the echo read answered %q, %v for the caged seat's pid %d, want its own %q from the private log — sentline's echo verdict is this line", got, ok, pid, ownLine)
	}
}
