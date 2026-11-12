//go:build !posse_arm2 && !posse_arm3

package posse

// QA arms added while verifying ranger-base-ajkqx and ranger-base-r2uxm
// (ranger-base-9xt70). Three gaps, each found by a mutant the closes' own
// pins left green:
//
//  1. claudeOwnSessionTree's OVER-GRANT arm. Its three refusals are pinned
//     with a main checkout, a plain dir in $HOME and a subdir of the tree —
//     all three of which the PATH SHAPE test (two components under the
//     worktree root) already refuses on its own. Dropping the
//     `LinkedGitDirs(real) == nil` guard therefore left every pin green,
//     although it hands a projects/ reopen to any directory sitting two
//     levels under the worktree root. The fixture here is that directory:
//     right shape, not a worktree.
//
//  2. The REPORT. `posse gates` prints the narrow rules (the r2uxm close
//     says so), and nothing asked it to: emptying the loop in SeatbeltReport
//     left the whole Wall|Seatbelt|Gates|Claude filter green.
//
//  3. The hostile shapes the live pin does not try. Its twelve are append,
//     replace-by-rename, directory move and a symlink in the TREE. These are
//     the rest of the write verbs a shell has over a file it can see:
//     unlink, in-place truncate, chmod-then-write, touch (posse's own cost
//     and turn-outcome readers key on a transcript's mtime), a hardlink out
//     of the read-only mount into the one writable dir inside it — the
//     classic read-only-bind escape, which Linux answers EXDEV because the
//     two paths are different MOUNTS of one filesystem — a symlink the other
//     way round, and a /proc/self/fd reopen of a descriptor obtained for
//     read. MEASURED 2026-11-12: all refused, peer bytes unchanged.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// A directory two levels under the worktree root that is NOT a linked git
// worktree is not the seat's own session tree, so its key dir is not
// reopened. Without this the only thing refusing it is a path-shape test
// every other fixture already fails.
func TestQAWallClaudeProjectsOwnTreeNeedsALinkedWorktreeAndNotJustThePathShape(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	// gitTempDir and not t.TempDir: this arm runs `git init` itself, for the
	// standalone-repository impostor, and a bare root under an init is what
	// TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot refuses.
	f := cpFixtureIn(t, gitTempDir(t))
	root, err := f.a.WorktreeRoot()
	if err != nil {
		t.Fatal(err)
	}

	// Three impostors at the one path shape claudeOwnSessionTree accepts.
	// The real tree is <root>/repo/developer-repo-x1, so each of these is
	// two components under the root and differs only in not being a linked
	// worktree.
	plain := filepath.Join(root, "repo", "not-a-worktree")
	standalone := filepath.Join(root, "repo", "own-repo-x2")
	stale := filepath.Join(root, "repo", "stale-x3")
	for _, d := range []string{plain, standalone, stale} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A repository of its own: git answers --git-dir, so only the
	// git-dir/common-dir DIFFERENCE tells it from a worktree.
	if out, err := exec.Command("git", "init", "-q", standalone).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// A tree whose worktree registration is gone: a `.git` FILE naming a
	// git dir that is not there. This is the shape a removed-but-not-cleaned
	// session leaves behind.
	if err := os.WriteFile(filepath.Join(stale, ".git"), []byte("gitdir: "+filepath.Join(root, "gone", ".git", "worktrees", "stale-x3")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := f.a.claudeOwnSessionTree(f.dir); got != absResolve(f.dir) {
		t.Fatalf("the fixture's real session worktree is not recognised (%q), so the refusals below are not measured against anything", got)
	}
	for name, cwd := range map[string]string{
		"a plain dir at the session-tree path shape":    plain,
		"a standalone repository at that path shape":    standalone,
		"a tree whose worktree registration is missing": stale,
	} {
		if got := f.a.claudeOwnSessionTree(cwd); got != "" {
			t.Errorf("%s (%s) is read as the seat's own session worktree (%q) — only a LINKED worktree is (ranger-base-ajkqx)", name, cwd, got)
		}
		// And the consequence, at the render: projects/ read-only whole,
		// with nothing reopened inside it.
		cfg := absResolve(f.cfg)
		projects := filepath.Join(cfg, "projects")
		binds := claudeProjectsBinds(f.home, runtimeStateGrants(SeatbeltStateDirs(f.rt)), f.a.claudeOwnSessionTree(cwd))
		if len(binds) != 1 || binds[0].Dst != projects || !binds[0].ReadOnly {
			t.Errorf("%s: projects/ is not read-only whole for this cwd: %+v", name, binds)
		}
		for _, n := range seatbeltProjectsNarrow(binds) {
			if n.Allow {
				t.Errorf("%s: the seatbelt render reopens %s for a cwd that is not the seat's own session worktree", name, n.Path)
			}
		}
	}
}

// `posse gates` prints the narrow rules, in the order the profile renders
// them, with the reopen marked w and the denies x. The r2uxm close claims
// this; nothing was asking for it.
func TestQAWallClaudeProjectsSeatbeltReportPrintsTheNarrowRules(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	projects := filepath.Join(cfg, "projects")
	own := filepath.Join(projects, claudeProjectDir(absResolve(f.dir)))
	mem := filepath.Join(own, "memory")

	var b bytes.Buffer
	if err := f.a.SeatbeltReport(f.ag, f.dir, &b, SeatbeltStateDirs(f.rt)...); err != nil {
		t.Fatal(err)
	}
	report := b.String()
	// The report abbreviates $HOME, so ask it the way it writes.
	wantRows := []struct{ mark, path string }{
		{"x", AbbrevHome(projects)},
		{"w", AbbrevHome(own)},
		{"x", AbbrevHome(mem)},
	}
	at := make([]int, len(wantRows))
	lines := strings.Split(report, "\n")
	for i, want := range wantRows {
		at[i] = -1
		for j, l := range lines {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, want.mark+" "+want.path+" ") || t == want.mark+" "+want.path {
				at[i] = j
				break
			}
		}
		if at[i] < 0 {
			t.Errorf("the gates report names no %q row for %s (ranger-base-r2uxm) — the narrow rules are in the profile and not in the report the operator reads:\n%s", want.mark, want.path, report)
		}
	}
	if at[0] >= 0 && at[1] >= 0 && at[2] >= 0 && !(at[0] < at[1] && at[1] < at[2]) {
		t.Errorf("the gates report lists the narrow rules out of order (%v) — the order IS the rule, so a report that reorders them describes a different wall:\n%s", at, report)
	}
	// Shown able to fail for the right reason: a seat whose cwd is not its
	// own session worktree has no reopen to print.
	var nb bytes.Buffer
	if err := f.a.SeatbeltReport(f.ag, f.main, &nb, SeatbeltStateDirs(f.rt)...); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(nb.String(), "w "+AbbrevHome(own)+" ") {
		t.Errorf("the gates report reopens the own key dir for the main checkout:\n%s", nb.String())
	}
}

// The rest of the write verbs, under the rendered wall and the real bwrap.
// Every one of these is a shape the ranger-base-ajkqx live pin does not try,
// and the hardlink is the read-only-bind escape: a link made from inside the
// read-only projects/ mount into the read-write own-key mount would leave
// the peer's inode writable under a name the wall allows.
func TestLiveWallClaudeProjectsRefusesTheWriteVerbsThePinDoesNotTry(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	wrcNeedBwrap(t)
	f := cpFixtureIn(t, wallUngrantedDir(t))
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
	projects := filepath.Join(f.cfg, "projects")
	own := filepath.Join(projects, claudeProjectDir(absResolve(f.dir)))
	peerDir := filepath.Dir(f.peerLog)

	// The own key dir must really be writable, or every refusal below is
	// the refusal of a wall that reopened nothing and the arm is decorative.
	if ok, out := try("echo '{}' > " + q(filepath.Join(own, "cccccccc-dddd-eeee-ffff-000000000000.jsonl"))); !ok {
		t.Fatalf("the seat cannot write its OWN key dir, so nothing below measures a narrowing: %s", out)
	}

	before := map[string][]byte{}
	for _, p := range []string{f.peerMem, f.peerLog} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[p] = b
	}
	peerStat, err := os.Stat(f.peerLog)
	if err != nil {
		t.Fatal(err)
	}

	for _, step := range []struct{ name, script string }{
		{"unlink a peer transcript", "rm -f " + q(f.peerLog)},
		{"truncate a peer transcript in place", ": > " + q(f.peerLog)},
		{"chmod a peer transcript, then write", "chmod 0666 " + q(f.peerLog) + " && echo PLANT >> " + q(f.peerLog)},
		{"touch a peer transcript", "touch " + q(f.peerLog)},
		{"unlink a peer memory file", "rm -f " + q(f.peerMem)},
		{"truncate a peer memory file in place", ": > " + q(f.peerMem)},
		{"mkdir a memory dir in a peer key", "mkdir -p " + q(filepath.Join(peerDir, "memory", "sub"))},
		{"hardlink a peer transcript into the own key dir, then write it",
			"ln " + q(f.peerLog) + " " + q(filepath.Join(own, "stolen.jsonl")) + " && echo PLANT >> " + q(filepath.Join(own, "stolen.jsonl"))},
		{"symlink from the own key dir to a peer transcript, then write it",
			"ln -s " + q(f.peerLog) + " " + q(filepath.Join(own, "s.jsonl")) + " && echo PLANT >> " + q(filepath.Join(own, "s.jsonl"))},
		{"reopen a read descriptor of a peer transcript for write",
			"exec 3< " + q(f.peerLog) + " && echo PLANT > /proc/self/fd/3"},
		{"rename the own key dir over a peer's", "mv " + q(own) + " " + q(peerDir)},
		{"write a new file at the root of projects/", "echo PLANT > " + q(filepath.Join(projects, "x"))},
		{"write inside the own key's memory under a new name", "mkdir -p " + q(filepath.Join(own, "memory")) + " && echo PLANT > " + q(filepath.Join(own, "memory", "notes.md"))},
	} {
		ok, out := try(step.script)
		if ok {
			t.Errorf("%s: the write SUCCEEDED under the rendered wall (ranger-base-ajkqx)\n script: %s\n out: %s", step.name, step.script, out)
		}
		t.Logf("%s: refused — %s", step.name, out)
		for p, b := range before {
			got, err := os.ReadFile(p)
			if err != nil || !bytes.Equal(got, b) {
				t.Errorf("%s: %s changed on the host (%q, %v)", step.name, p, got, err)
				_ = os.MkdirAll(filepath.Dir(p), 0o755)
				_ = os.WriteFile(p, b, 0o644)
			}
		}
		if fi, err := os.Stat(f.peerLog); err != nil || !fi.ModTime().Equal(peerStat.ModTime()) {
			t.Errorf("%s: the peer transcript's mtime moved (%v, %v) — posse's cost and turn-outcome readers key on it", step.name, fi, err)
		}
		if _, err := os.Lstat(filepath.Join(own, "stolen.jsonl")); err == nil {
			t.Errorf("%s: a hardlink to a peer transcript exists inside the seat's own key dir on the host", step.name)
		}
	}
}

// D3's PREMISE, as a pin rather than as a scratch rig. The design's third
// decision — a seat whose cwd is not its own session worktree gets projects/
// read-only WHOLE — rests on one measurement: that the real claude runs with
// its transcript dir unwritable instead of dying on it. If it died, D3 would
// take the `--session-id` exit hatch, and every probe seat and every seat in
// the shared checkout would be launching a CLI that cannot start.
//
// It was MEASURED before the build (ranger-base-ajkqx's step-0 comment, claude
// 2.1.322) in a rig that lived in a session scratchpad and is gone, so nothing
// in the tree re-asks it — and it is a claim about the INSTALLED CLI, which
// moves under us. This arm asks the real binary both ways in one run: under a
// wall rendered for a cwd that is not the seat's own it ends by itself and
// leaves projects/ byte-for-byte as it found it, and under the own-cwd wall the
// same binary in the same minute writes its transcript. The second half is what
// stops the first from passing over a claude that writes nothing anywhere.
//
// Unbilled: a dead loopback endpoint, as the ranger-base-ajkqx live pin uses.
// NOT re-measured here, and left as the close's own reading: that the run exits
// 0 and prints an answer, which needs a serving endpoint.
func TestLiveWallClaudeProjectsANonOwnCwdRunsTheRealClaudeAndWritesNoTranscript(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	wrcNeedBwrap(t)
	bin := wrcRealBinary("claude")
	if bin == "" {
		t.Skip("NOTHING MEASURED: no claude binary on this box")
	}
	f := cpFixtureIn(t, wallUngrantedDir(t))
	projects := filepath.Join(f.cfg, "projects")

	run := func(t *testing.T, cwd string) string {
		t.Helper()
		r, err := f.a.bwrapRender(f.ag, f.rt, cwd, f.gates)
		if err != nil {
			t.Fatal(err)
		}
		if err := runBwrapPrep(r.Prep); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bwrap", append(append([]string{}, r.Argv...), bin, "-p", "say hi")...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "HOME="+f.home, ClaudeConfigDirVar+"="+f.cfg,
			"ANTHROPIC_BASE_URL=http://127.0.0.1:1", "ANTHROPIC_API_KEY=sk-dead-9xt70",
			"DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "CLAUDE_CODE_MAX_RETRIES=0")
		done := make(chan struct{})
		var out []byte
		go func() { out, _ = cmd.CombinedOutput(); close(done) }()
		select {
		case <-done:
		case <-time.After(90 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Fatalf("claude -p under the wall for cwd %s did not end in 90s — D3 assumes an unwritable transcript dir is survivable, and a seat that hangs is the failure that assumption rules out:\n%s", cwd, out)
		}
		return strings.TrimSpace(string(out))
	}

	// The premise of the premise: for this cwd projects/ really is read-only
	// whole, with nothing reopened inside it.
	if got := f.a.claudeOwnSessionTree(f.main); got != "" {
		t.Fatalf("the fixture's main checkout is read as the seat's own session worktree (%q), so the run below is not the D3 case", got)
	}
	binds := claudeProjectsBinds(f.home, runtimeStateGrants(SeatbeltStateDirs(f.rt)), f.a.claudeOwnSessionTree(f.main))
	if len(binds) != 1 || !binds[0].ReadOnly {
		t.Fatalf("projects/ is not read-only whole for the main checkout: %+v", binds)
	}

	beforeTree := map[string]string{}
	walk := func() map[string]string {
		seen := map[string]string{}
		_ = filepath.Walk(projects, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			seen[p] = string(b)
			return nil
		})
		return seen
	}
	beforeTree = walk()
	if len(beforeTree) == 0 {
		t.Fatal("the fixture planted no file under projects/, so an unchanged tree below proves nothing")
	}

	out := run(t, f.main)
	t.Logf("real claude -p with projects/ read-only whole (cwd %s): %s", f.main, out)
	if after := walk(); !reflect.DeepEqual(after, beforeTree) {
		t.Errorf("the real claude changed projects/ from a cwd that is not the seat's own session worktree\n before: %v\n after:  %v\n out: %s", beforeTree, after, out)
	}

	// The control, same binary, same minute: with its own key dir reopened
	// it does write a transcript. Without this the arm above would pass over
	// a claude that writes no transcript under any wall at all.
	own := filepath.Join(projects, claudeProjectDir(absResolve(f.dir)))
	before, _ := filepath.Glob(filepath.Join(own, "*.jsonl"))
	ctl := run(t, f.dir)
	after, _ := filepath.Glob(filepath.Join(own, "*.jsonl"))
	if len(after) <= len(before) {
		t.Errorf("the control wrote no transcript in its own key dir %s, so the reading above is not a measurement of the NARROWING\n out: %s", own, ctl)
	}
}
