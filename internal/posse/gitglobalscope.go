package posse

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The GIT_CONFIG_GLOBAL row's value (ranger-base-obdnk, from
// ranger-base-3m6xy).
//
// Three env names steer git's GLOBAL scope — GIT_CONFIG_GLOBAL itself,
// XDG_CONFIG_HOME (git reads $XDG_CONFIG_HOME/git/config) and HOME (git reads
// $HOME/.gitconfig) — and a lower-scope settings `env` naming any of them at
// an attacker's config ran that config's core.hooksPath. MEASURED 2026-11-03
// (git 2.50.1 Apple Git-155): all three FIRED under the pin as it stood, and
// all three went QUIET with GIT_CONFIG_GLOBAL set to an absolute path,
// because that variable replaces git's whole global-file lookup.
//
// ranger-base-37y0z left the row out because every CLOSING constant
// (/dev/null, "") drops ~/.gitconfig — where user.name and user.email live —
// and git then commits under a gecos ident at rc 0 while the identity wall
// (DeriveIdentityLiterals) comes out with no e-mail in it. That ruling
// rested on there being no neutral value a constant could carry. Since
// ranger-base-t301k the launcher resolves a row per launch, so the value can
// be the file git itself would read with the variable unset:
//
//   - exactly one global file exists: that file.
//   - none exists: the LAST candidate, $HOME/.gitconfig, which is where
//     `git config --global` writes when the variable is unset.
//   - BOTH exist: NO row. git reads both, XDG first, and no single value
//     reads both (MEASURED, fixture HOME: pinning either loses the other's
//     keys; an [include] file breaks `git config --global --get-all` and
//     moves --global writes). The inlet stays open, as it was under
//     ranger-base-37y0z, and the launch says so (GitGlobalScopeDegraded).
//
// THE FAIL DIRECTION IS THE OPPOSITE OF THE SYSTEM ROW'S, on purpose. An
// unresolved GIT_CONFIG_SYSTEM falls back to /dev/null: what that loses is
// system scope, said on the launch line. An unresolved GIT_CONFIG_GLOBAL
// falls back to NO ROW, never /dev/null or "": what those lose is the
// operator's identity, so every commit is misattributed and the identity
// wall is emptied, at rc 0, with nothing said by git. Leaving an inlet open
// and naming it is the lesser cost.
//
// Stated, not fixed:
//   - The value is resolved from the LAUNCHER's HOME and XDG_CONFIG_HOME. A
//     launcher whose own env is attacker-steered is out of scope here, as it
//     is for every row.
//   - A second global file created after launch is not read by that session;
//     the next launch degrades.
//   - HOME's reach beyond git (a shell's rc files, other tools' dotfiles) is
//     not covered by this row.
//   - The container tier renders the same pin, and a container seat's HOME is
//     its cage HOME, not the launcher's: there the row names a host path the
//     container does not mount. Neither file exists inside either way (the
//     cage HOME is seeded with no git config), so what git READS is the same;
//     what moves is where a `git config --global` write lands.
//
// THE ROW NAMES A READ-ONLY SNAPSHOT, NOT THE OPERATOR'S FILE
// (ranger-base-161t2). Naming the operator's own file made every seat's
// `git config --global` a write to it, and a moved HOME no longer diverts
// that write — the row replaces git's whole lookup, which is the point of
// the row. OBSERVED 2026-11-15: a probe on a shims-tier seat ran
// `export HOME=<mktemp>/home; git config --global user.name probe` meaning
// a scratch HOME, and rewrote ~/.config/git/config, so every later commit on
// the box (public posse history included) carried probe/probe@invalid. A
// shims seat has no kernel wall over ~/.config; the caged tiers grant no
// write there, but the row is the one thing every tier shares.
//
// So the row's value is gitGlobalSnapshot's file: a byte copy of the
// resolved file, content-addressed under SeatWallRoot()/git-global — the
// directory no seat tier grants a write to (gitGlobalPinValue says why not
// `.cache/posse`) — with
// a DIRECTORY standing at `<file>.lock`. git takes a config write's lock
// with O_CREAT|O_EXCL on exactly that name, so every LOCK-TAKING write —
// `--global`, `--file "$GIT_CONFIG_GLOBAL"`, `--unset`, and the five other
// spellings gitglobalsnapshotmode_qa_test.go lists — refuses at rc 255 with
// `could not lock config file …: File exists`, and reads, `git var
// GIT_AUTHOR_IDENT` and DeriveIdentityLiterals see the same bytes they
// would see in the original (MEASURED 2026-11-15, git 2.56.0, linux). A
// lock directory rather than a 0555 parent, because a 0555 dir is one
// os.RemoveAll cannot empty, and every test HOME is a t.TempDir.
//
// `git config --global --edit` is the one write that does NOT go through that
// lock: it hands the path to $GIT_EDITOR and the editor writes in place, so
// the snapshot's 0444 MODE is the whole of what refuses it (MEASURED
// 2026-11-15, ranger-base-k2cbh — a snapshot with its lock directory in place
// at mode 0644 is rewritten by `--edit` at rc 0). "Every write refuses at rc
// 255" was this block's own sentence until ranger-base-r9slg, and it was not
// true of `--edit`; the mode is held by gitGlobalSnapshotVerify and healed by
// gitGlobalSnapshot, both of which say so where they do it.
//
//   - A file carrying an [include] or [includeIf] section is NOT snapshotted:
//     a relative include path resolves against the including file's own
//     directory, and the copy lives in another one. The row then names the
//     original file as before, and GitGlobalScopeDegraded says the write
//     route is open. So does any failure to write the snapshot.
//   - The snapshot is as of the launch: an edit to the operator's file is
//     seen by the next launch, not by a running session.
//   - Cooperative on the shims tier, as everything there is: a shims seat
//     that removes the lock directory can write the snapshot (and another
//     seat launched over the same bytes reads it until the next launch
//     rewrites it), and one that names ~/.config/git/config by path writes
//     it. A caged seat can do neither: no render grants the wall root, the
//     home itself, or ~/.config/git.
//     What closes is the route both incidents took — `--global` through the
//     pin, by a process that believed it had moved HOME.

// gitGlobalScopeCandidates asks gitBin which files it reads as global scope
// with GIT_CONFIG_GLOBAL unset, in the order git reads them.
//
// `git var GIT_CONFIG_GLOBAL` (git 2.42+) names every candidate whether or
// not it exists — MEASURED: always two lines, XDG first, when HOME is set.
// An older git answers it with an error, and so does any git with HOME unset;
// either way the answer falls back to git's documented rule over the same
// environment, which is the whole of what git does.
func gitGlobalScopeCandidates(gitBin string, env []string) ([]string, error) {
	if gitBin != "" {
		ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
		defer cancel()
		cmd := protocolCmd(exec.CommandContext(ctx, gitBin, "var", "GIT_CONFIG_GLOBAL"))
		bindGitDeadline(ctx, cmd)
		cmd.Env = protocolEnv(env)
		var out bytes.Buffer
		cmd.Stdout = &out
		if cmd.Run() == nil {
			var paths []string
			for _, l := range strings.Split(out.String(), "\n") {
				if l = strings.TrimRight(l, "\r"); l != "" {
					paths = append(paths, l)
				}
			}
			if len(paths) > 0 {
				return paths, nil
			}
		}
	}
	return gitGlobalScopeDocumented(env)
}

// gitGlobalScopeDocumented is git's documented global-file rule (git-config
// FILES): ${XDG_CONFIG_HOME:-$HOME/.config}/git/config, then $HOME/.gitconfig.
// An empty XDG_CONFIG_HOME counts as unset, as it does in git. With no HOME
// and no XDG_CONFIG_HOME git reads no global file at all, and there is
// nothing to pin.
func gitGlobalScopeDocumented(env []string) ([]string, error) {
	get := func(k string) (string, bool) {
		for i := len(env) - 1; i >= 0; i-- {
			if v, ok := strings.CutPrefix(env[i], k+"="); ok {
				return v, true
			}
		}
		return "", false
	}
	home, hasHome := get("HOME")
	var out []string
	if xdg, _ := get("XDG_CONFIG_HOME"); xdg != "" {
		out = append(out, xdg+"/git/config")
	} else if hasHome {
		out = append(out, home+"/.config/git/config")
	}
	if hasHome {
		out = append(out, home+"/.gitconfig")
	}
	if len(out) == 0 {
		return nil, errors.New("neither HOME nor XDG_CONFIG_HOME is set, so git reads no global file")
	}
	return out, nil
}

// errGitGlobalScopeTwoFiles is the one resolution that has no neutral value.
var errGitGlobalScopeTwoFiles = errors.New("git global scope has two files")

// chooseGitGlobalScope is the row's decision over the candidates, with
// existence asked fresh: one existing file is that file, none is the last
// candidate (git's write target), and more than one is no value at all.
func chooseGitGlobalScope(cands []string, exists func(string) bool) (string, error) {
	if len(cands) == 0 {
		return "", errors.New("git named no global-scope file")
	}
	var have []string
	for _, c := range cands {
		if exists(c) {
			have = append(have, c)
		}
	}
	switch len(have) {
	case 0:
		return cands[len(cands)-1], nil
	case 1:
		return have[0], nil
	}
	return "", fmt.Errorf("%w: %s", errGitGlobalScopeTwoFiles, strings.Join(have, " and "))
}

// gitGlobalScopeCache keys the candidate list on the git binary and the two
// env values git derives it from. Existence is NOT cached — it is two stats,
// and a file that appears between launches must move the next answer.
var gitGlobalScopeCache struct {
	sync.Mutex
	m map[string]gitGlobalScopeEntry
}

type gitGlobalScopeEntry struct {
	size  int64
	mtime time.Time
	cands []string
}

// gitGlobalScope is the GIT_CONFIG_GLOBAL row's value for this launch, or
// the error that says why there is no row.
func gitGlobalScope() (string, error) {
	env := gitSystemScopeEnv() // every GIT_CONFIG* scrubbed, gates-free PATH
	bin := resolveOutside("git", "")
	key := bin
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME"} {
		v, set := os.LookupEnv(k) // unset and empty resolve differently in git
		key += fmt.Sprintf("\x00%t:%s", set, v)
	}
	var cands []string
	fi, statErr := os.Stat(bin)
	if bin != "" && statErr == nil {
		gitGlobalScopeCache.Lock()
		e, ok := gitGlobalScopeCache.m[key]
		gitGlobalScopeCache.Unlock()
		if ok && e.size == fi.Size() && e.mtime.Equal(fi.ModTime()) {
			cands = e.cands
		}
	}
	if cands == nil {
		c, err := gitGlobalScopeCandidates(bin, env)
		if err != nil {
			return "", err
		}
		cands = c
		if bin != "" && statErr == nil {
			gitGlobalScopeCache.Lock()
			if gitGlobalScopeCache.m == nil {
				gitGlobalScopeCache.m = map[string]gitGlobalScopeEntry{}
			}
			gitGlobalScopeCache.m[key] = gitGlobalScopeEntry{fi.Size(), fi.ModTime(), cands}
			gitGlobalScopeCache.Unlock()
		}
	}
	return chooseGitGlobalScope(cands, func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
}

// GitGlobalScopeDegraded is the launch line for a launch whose pin carries
// no GIT_CONFIG_GLOBAL row, "" when it carries one. The launch proceeds: the
// session reads exactly the global scope it would outside posse, and the
// line says which inlets that leaves open.
func GitGlobalScopeDegraded() string {
	p, err := gitGlobalScope()
	if err == nil {
		if _, serr := gitGlobalPinValue(p); serr != nil {
			return fmt.Sprintf("DEGRADED: GIT_CONFIG_GLOBAL names the operator's own %s, not a read-only snapshot of it, because %v — a `git config --global` in this session WRITES that file, whatever HOME it sets (ranger-base-161t2)", p, serr)
		}
		return ""
	}
	if errors.Is(err, errGitGlobalScopeTwoFiles) {
		files := strings.TrimPrefix(err.Error(), errGitGlobalScopeTwoFiles.Error()+": ")
		return fmt.Sprintf("DEGRADED: %v; GIT_CONFIG_GLOBAL inlet NOT pinned (%s). No single value reads both, and /dev/null would misattribute every commit, so GIT_CONFIG_GLOBAL, XDG_CONFIG_HOME and HOME stay open to git in this session; merge the two files to close them (ranger-base-obdnk)", errGitGlobalScopeTwoFiles, files)
	}
	return fmt.Sprintf("DEGRADED: GIT_CONFIG_GLOBAL inlet NOT pinned because %v. GIT_CONFIG_GLOBAL, XDG_CONFIG_HOME and HOME stay open to git in this session (ranger-base-obdnk)", err)
}

// gitGlobalSnapshotName is the snapshot's file name. git prints it in the
// refusal a write meets, so it says what the file is.
const gitGlobalSnapshotName = "read-only-snapshot.gitconfig"

// errGitGlobalIncludes: a file whose includes would resolve differently
// from a copy's directory (ranger-base-161t2).
var errGitGlobalIncludes = errors.New("it carries an [include]/[includeIf] section, whose relative paths would resolve against the snapshot's directory instead of its own")

// gitGlobalPinValue is the GIT_CONFIG_GLOBAL row's value for the resolved
// global file: its read-only snapshot, or the file itself with the error
// that says why there is no snapshot. Both the settings pin and the hook-skip
// reference read it, so the two cannot name different files.
//
// The snapshot lives under SeatWallRoot, NOT posseCacheDir: `.cache/posse`
// is write-granted to every caged seat (homeCacheGrants — it is the box's
// lock dir), and a snapshot there is a file one caged seat could rewrite
// and every sibling launched over the same bytes would then read as its
// global scope, core.hooksPath included — the exec inlet obdnk closed,
// reopened between seats. The wall root is the directory no render grants
// (ADR 0100 D2(a) refuses one at or above it), and it reads through the
// seat's read-only `/`.
//
// PURE: it names a snapshot that is already on disk and verifies, and never
// makes one. Every reader renders the settings pin — `posse gates`, the
// parity report, a status line — and a reader that wrote a host path in the
// HOME it was typed from would break ranger-base-qd9ei's rule that only a
// LAUNCH makes host paths. PrepareGitGlobalSnapshot is the launch's half.
// An unprepared snapshot falls back to the file itself: the row stays
// closed, and the launch, which prepares first, never sees that arm.
func gitGlobalPinValue(file string) (string, error) {
	want, snap, err := gitGlobalSnapshotPath(file, SeatWallRoot(os.Getenv("HOME")))
	if err != nil {
		return file, err
	}
	if err := gitGlobalSnapshotVerify(snap, want); err != nil {
		return file, fmt.Errorf("no launch has prepared its read-only snapshot (%v)", err)
	}
	return snap, nil
}

// PrepareGitGlobalSnapshot writes (or re-verifies) the read-only snapshot
// of this launch's resolved global file, so the settings pin rendered after
// it names the snapshot. Called by a LAUNCH, before it renders the line, and
// by nothing that only reads (ranger-base-qd9ei). Nothing to do — nil — when
// the box has no single global file: there is no row to snapshot.
func PrepareGitGlobalSnapshot() error {
	p, err := gitGlobalScope()
	if err != nil {
		return nil
	}
	_, err = gitGlobalSnapshot(p, SeatWallRoot(os.Getenv("HOME")))
	return err
}

// gitGlobalSnapshotPath is the snapshot's path for file under root, and the
// bytes it must hold. A missing file is empty bytes.
func gitGlobalSnapshotPath(file, root string) (want []byte, snap string, err error) {
	if root == "" {
		return nil, "", errors.New("there is no HOME to hold the snapshot under")
	}
	want, err = os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	if gitIncludesSection(want) {
		return nil, "", errGitGlobalIncludes
	}
	sum := sha256.Sum256(want)
	return want, filepath.Join(root, "git-global", hex.EncodeToString(sum[:])[:20], gitGlobalSnapshotName), nil
}

// gitGlobalSnapshotMode is the snapshot's mode, and the second half of its
// wall: the lock directory refuses every LOCK-taking config write, and the
// mode is the whole of what refuses `git config --global --edit`, which takes
// no lock at all (ranger-base-k2cbh, pinned in
// gitglobalsnapshotmode_qa_test.go). One constant because the writer that
// sets it and the verify that reads it must not be able to disagree.
const gitGlobalSnapshotMode fs.FileMode = 0o444

// gitGlobalSnapshotVerify: the lock is a directory, the snapshot is a regular
// file at gitGlobalSnapshotMode, and the bytes are want.
//
// THE MODE IS ASKED HERE, and not only written at the rewrite
// (ranger-base-r9slg). A mode-only tamper — the bytes left alone, 0444 turned
// to 0644 — was the one tamper nothing saw and nothing healed: the rewrite
// arm in gitGlobalSnapshot is skipped by a content-identical snapshot, so the
// chmod that lived only there never ran, and this verify read the lock and the
// bytes and passed. MEASURED 2026-11-20: the next launch returned nil over a
// 0644 snapshot, every reader kept naming it, and `git config --global --edit`
// under the rendered row rewrote it at rc 0 (`git var GIT_AUTHOR_IDENT` then
// answering "Author identity unknown"). Asked here it degrades for every
// READER — the row falls back to the operator's own file, which is writable
// but is at least the file the operator can see — and it is what makes the
// launch's heal below reachable.
//
// Lstat, not Stat, and IsRegular: a symlink planted at the snapshot's name
// carries mode 0777 and would resolve to whatever it points at, which is the
// same tamper wearing a different hat.
func gitGlobalSnapshotVerify(snap string, want []byte) error {
	lock := snap + ".lock"
	if fi, err := os.Lstat(lock); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory, so git can take its lock", lock)
	}
	fi, err := os.Lstat(snap)
	if err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file (%v)", snap, err)
	}
	if perm := fi.Mode().Perm(); perm != gitGlobalSnapshotMode {
		return fmt.Errorf("%s is mode %#o, not %#o, so `git config --global --edit` can rewrite it under the rendered row", snap, perm, gitGlobalSnapshotMode)
	}
	if got, err := os.ReadFile(snap); err != nil || !bytes.Equal(got, want) {
		return fmt.Errorf("%s does not read back as a copy of the global file", snap)
	}
	return nil
}

// gitIncludesSection reports whether a git config text has an include or
// includeIf section header. Section names are case-insensitive in git.
func gitIncludesSection(b []byte) bool {
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.ToLower(strings.TrimSpace(l))
		if strings.HasPrefix(l, "[include") {
			return true
		}
	}
	return false
}

// gitGlobalSnapshot writes (or re-verifies) the read-only snapshot of file
// under root and returns its path. Only PrepareGitGlobalSnapshot calls it
// outside a test: it is the one host write here (ranger-base-qd9ei). A missing file snapshots as empty:
// that is the none-exists shape, where git would have CREATED the
// operator's ~/.gitconfig on the first --global write. Content-addressed,
// so concurrent launches over the same bytes agree on one path, and a
// snapshot someone edited is rewritten by the next launch over it.
func gitGlobalSnapshot(file, root string) (string, error) {
	want, snap, err := gitGlobalSnapshotPath(file, root)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(snap)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if got, rerr := os.ReadFile(snap); rerr != nil || !bytes.Equal(got, want) {
		tmp, err := os.CreateTemp(dir, ".snap-*")
		if err != nil {
			return "", err
		}
		_, werr := tmp.Write(want)
		cerr := tmp.Close()
		if werr == nil {
			werr = cerr
		}
		if werr == nil {
			werr = os.Chmod(tmp.Name(), gitGlobalSnapshotMode)
		}
		if werr == nil {
			werr = os.Rename(tmp.Name(), snap)
		}
		if werr != nil {
			_ = os.Remove(tmp.Name())
			return "", werr
		}
	} else if fi, serr := os.Lstat(snap); serr == nil && fi.Mode().IsRegular() && fi.Mode().Perm() != gitGlobalSnapshotMode {
		// THE MODE-ONLY TAMPER'S HEAL (ranger-base-r9slg). The rewrite above
		// is skipped by a snapshot whose BYTES are already right, and the
		// chmod lived only inside it — so a snapshot someone had turned 0644
		// stayed 0644 through every later launch, with `--edit` the one write
		// route still open through it. Content and mode are healed the same
		// way now: by the next launch over the same bytes.
		//
		// A chmod and not a rewrite, because the bytes are already the ones
		// this launch wants and a rename would hand every concurrent launch
		// over the same path a new inode for nothing. The error is returned
		// rather than swallowed: PrepareGitGlobalSnapshot's caller prints the
		// DEGRADED line, which is the honest answer when posse can neither
		// trust the snapshot nor repair it.
		if err := os.Chmod(snap, gitGlobalSnapshotMode); err != nil {
			return "", err
		}
	}
	// The lock directory is the wall: git's config lock is O_EXCL on this
	// exact name. A plain file there would do the same until someone read it
	// as a stale lock and removed it; a directory is not mistaken for one.
	lock := snap + ".lock"
	if err := os.Mkdir(lock, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	// Read back, never trust the writes.
	if err := gitGlobalSnapshotVerify(snap, want); err != nil {
		return "", err
	}
	return snap, nil
}
