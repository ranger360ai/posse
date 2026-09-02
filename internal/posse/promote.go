package posse

// `posse promote` — the constitution's `make install` (ADR 0015 §2/§3).
//
// The mechanism repo has a promotion step and the constitution did not:
// `~/.config/rhq` was a symlink into the instance repo, one inode, so a
// saved PID was in force at the next launch, uncommitted edits included.
// This file is the step that was missing. The home stops being the repo and
// becomes a COPY of it, taken from a commit, recorded in a manifest, and
// re-checked at every launch.
//
// Three things the home holds that promote deliberately does NOT write, each
// for a different reason, and each spelled in code rather than in prose so a
// later edit has to argue with a symbol:
//
//	envs/      secret values, gitignored by the constitution repo's own
//	           policy — there is no commit to promote them from, and a copy
//	           path that widens 0600 publishes tokens to every process of
//	           this user (ADR 0015 §7). Promote never creates, copies, or
//	           touches it. `posse init` seeds it, TightenEnvPerms re-asserts
//	           its modes, and the seatbelt never grants it.
//	state/     machine-local runtime data, session-writable by design.
//	personas/  persona memory (ORDERS), whose write loop is a session end —
//	           excluded from promotion and the manifest on purpose (§5).
//
// The fence is spelled twice, and neither spelling is the wall: every crew
// PID denies `Bash(posse promote:*)` (an L1-realizable named verb), and
// promote itself refuses under the persona env marker. What NOTICES a
// constitution that changed without a promotion is the manifest — the launch
// verify below, which is detection at every tier including the shims tier
// seven of eight personas actually run on (the shape ranger-base-5na asked
// for, with the trust anchor it lacked).

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PromotedPaths is the promoted set: what `posse promote` copies out of the
// constitution and what the launch verify hashes, relative to both the
// constitution source directory and the home. `config.yaml` is a file, the
// other four are trees.
//
// `runtimes` joined 2026-09-01 (ADR 0039 D2, ranger-base-ight8). The
// per-key runtime overlay (ADR 0021) is read at every launch and is the
// only thing that makes a tier's current model current — it was the one
// launch-read fact at the home that no manifest attested to, written by no
// code path and holding no secret, so it belongs here on the same terms as
// `config.yaml`. Every consequence of the addition is a reader of this
// list: the copy, the removal, the manifest, the launch verify, the
// seatbelt's HomeConstitutionPaths and the commit wall's
// ConstitutionRepoPaths. That is the point of the list.
//
// It is a var and not a const list so a test can prove the exclusions below
// are not in it; nothing mutates it at runtime.
var PromotedPaths = []string{"agents", "config.yaml", "recipes", "runtimes", "skills"}

// NotPromoted names what lives at the home and is never promoted — the
// list exists so the exclusion is greppable and testable, not so anything
// reads it in the copy path (the copy path only ever walks PromotedPaths).
var NotPromoted = []string{ConstitutionEnvsDir, "state", "personas"}

// ConstitutionEnvsDir is `envs/` as both the home and the constitution repo
// spell it. It is NOT promoted (§7: secret values, gitignored, no commit to
// promote them from) and it is still constitution — HomeConstitutionPaths
// keeps it out of every seatbelt grant for that reason, and
// ConstitutionRepoPaths keeps it out of a persona's commits for the same
// one. Named rather than repeated so the two lists cannot drift.
const ConstitutionEnvsDir = "envs"

// ConstitutionSourceDir is where the constitution's promoted set lives
// inside the constitution REPO — `<repo>/posse/agents`,
// `<repo>/posse/skills` and so on, the layout `posse promote` reads from and
// the one the commit wall's path class is spelled with (ranger-base-ak3e).
// It moved from `rhq` by ADR 0046: the same name on both sides of the
// promote copy, so a manifest key reads the same in either tree.
const ConstitutionSourceDir = "posse"

// ConstitutionRepoPaths is the constitution class as a REPO spells it,
// slash-separated and relative to the repo top: every PromotedPaths entry
// under ConstitutionSourceDir, plus `envs/`. Read by the commit wall's
// constitution arm at hook-render time so that adding a path to
// PromotedPaths widens the wall in the same edit (ranger-base-ak3e) — the
// mirror of HomeConstitutionPaths, which does the same for the home.
//
// `envs/` is in it even though promote never writes it: a persona commit
// that adds `posse/envs/foo.env` to the constitution repo is a secret in a
// git history whether or not any promote would copy it.
func ConstitutionRepoPaths() []string {
	out := make([]string, 0, len(PromotedPaths)+1)
	for _, p := range PromotedPaths {
		out = append(out, path.Join(ConstitutionSourceDir, p))
	}
	return append(out, path.Join(ConstitutionSourceDir, ConstitutionEnvsDir))
}

// ConstitutionRepoMarker is how a hook decides, at commit time, that the
// repo it is in IS the constitution repo: its top level has this directory.
// `posse/agents` and not the whole promoted set, because it is the one member
// no constitution can be missing — a PID dir IS what makes a tree the
// fleet's law — and asking for a tree rather than a file keeps a stray
// `posse/config.yaml` in some unrelated repo from widening the wall there.
const ConstitutionRepoMarker = ConstitutionSourceDir + "/agents"

// ConstitutionClassIn is the class as the repo at top spells it, and is the
// GO reading of exactly what the commit wall's constitution arm renders into
// sh (gates.go constitutionGuardBody): the two settings files in every repo,
// plus ConstitutionRepoPaths in a repo whose top level carries
// ConstitutionRepoMarker.
//
// Two readers, one list, on purpose (ranger-base-ak3e). The hook is the L3
// arm and stands down to `env -i`; the launcher's land path is the belt
// behind it, runs operator-side and cannot be env-scrubbed by the session it
// is judging. A belt spelled from a second list is a belt that drifts, and
// the day it drifts is the day the shim tier is the only one holding.
func ConstitutionClassIn(top string) []string {
	class := []string{ClaudeProjectConfig, ClaudeProjectConfigLocal}
	if fi, err := os.Stat(filepath.Join(top, filepath.FromSlash(ConstitutionRepoMarker))); err == nil && fi.IsDir() {
		class = append(class, ConstitutionRepoPaths()...)
	}
	return class
}

// InConstitutionClass names the class member a repo-relative, slash-separated
// path falls under, "" when none. Exact match or directory prefix — one rule,
// so `posse/config.yaml` (a file) and `posse/agents` (a tree) need no second
// case. This is the Go spelling of the hook's `case "$p" in "$m"|"$m"/*)`
// arm.
func InConstitutionClass(class []string, p string) string {
	for _, m := range class {
		if p == m || strings.HasPrefix(p, m+"/") {
			return m
		}
	}
	return ""
}

// ConstitutionTouched names every (path, class member) pair in paths, in the
// order the paths came, each rendered as the refusal prints it. Empty is the
// ordinary case and the only one that lands.
func ConstitutionTouched(class, paths []string) []string {
	var out []string
	for _, p := range paths {
		if m := InConstitutionClass(class, p); m != "" {
			out = append(out, p+" (class: "+m+")")
		}
	}
	return out
}

// PromoteManifestFile is the manifest's name at the home, BESIDE the
// promoted copy: not under state/, which stays session-writable, and so
// would be a trust anchor any session could rewrite.
const PromoteManifestFile = "promoted.json"

// promoteManifestVersion is bumped when the file's shape changes. A manifest
// from the future is not read and not guessed at — it degrades the launch
// the same way a mismatch does, because posse cannot say what it covers.
const promoteManifestVersion = 1

// PromoteManifest is what the home records about its own constitution.
// Seeded means `posse init` wrote it from the embedded examples — a fresh
// box has a real manifest (so the launch verify never fires on a clean
// install) and no commit behind it, which is the honest difference.
//
// What `seeded` does NOT mean, and what nothing may read it as: that posse
// laid these bytes down. SeedPromoteManifest hashes whatever is on disk at
// the moment it runs, and the homes it has already run on include older ones
// full of the operator's own files, personas they adopted in place included
// — init stamped every home that had none until ranger-base-h7cd stopped it
// (init.go: only a home init actually seeded gets one now; an existing
// instance keeps whatever manifest it already has). The manifest is an ANCHOR
// ("these bytes were here when posse started watching"), never provenance
// ("posse wrote these bytes"). Provenance for a seed file has its own answer,
// isShippedExample in exampledigests.go; taking it off Seeded instead retired
// an operator's persona out of routing in two inits (ranger-base-rgx0).
type PromoteManifest struct {
	Version    int               `json:"version"`
	PromotedAt string            `json:"promoted_at"`
	Seeded     bool              `json:"seeded,omitempty"`
	Source     string            `json:"source,omitempty"` // constitution dir promoted from
	Repo       string            `json:"repo,omitempty"`   // its git root
	SHA        string            `json:"sha,omitempty"`    // the commit promoted
	Files      map[string]string `json:"files"`            // slash path → sha256 hex
}

// PromoteManifestPath is where the manifest sits.
func (a *App) PromoteManifestPath() string { return filepath.Join(a.Home, PromoteManifestFile) }

// ReadPromoteManifest reads the manifest at path. A missing file is (nil,
// nil): the home was never promoted or seeded, and there is nothing to
// verify against — every install that predates ADR 0015 is in that state
// and must keep launching.
func ReadPromoteManifest(p string) (*PromoteManifest, error) {
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m PromoteManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, Die("%s is not readable as a promote manifest: %v", AbbrevHome(p), err)
	}
	if m.Version > promoteManifestVersion {
		return nil, Die("%s is version %d; this posse understands %d", AbbrevHome(p), m.Version, promoteManifestVersion)
	}
	return &m, nil
}

func (m *PromoteManifest) write(p string) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

// sha256File is the per-file hash the manifest records.
func sha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashPromotedSet hashes the promoted set under root, keyed by slash-
// separated path relative to root. A promoted path that is absent
// contributes nothing — an instance with no skills/ is not a mismatch, it
// is an instance with no skills.
//
// Only regular files are hashed. A symlink or a device node inside the
// promoted set is not prose posse can attest to, so it is reported as an
// entry with no hash — which is a mismatch on both sides of the comparison
// and therefore never silently blessed.
func HashPromotedSet(root string) (map[string]string, error) {
	out := map[string]string{}
	for _, rel := range PromotedPaths {
		p := filepath.Join(root, rel)
		st, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			sum, err := hashEntry(p, st)
			if err != nil {
				return nil, err
			}
			out[rel] = sum
			continue
		}
		err = filepath.WalkDir(p, func(fp string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			sum, err := hashEntry(fp, info)
			if err != nil {
				return err
			}
			r, err := filepath.Rel(root, fp)
			if err != nil {
				return err
			}
			out[filepath.ToSlash(r)] = sum
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// notRegular is the hash recorded for an entry posse cannot attest to. It is
// not a hex string, so it can never collide with a real one.
const notRegular = "not-a-regular-file"

func hashEntry(p string, info os.FileInfo) (string, error) {
	if !info.Mode().IsRegular() {
		return notRegular, nil
	}
	return sha256File(p)
}

// ─── the launch verify ───────────────────────────────────────────────────────

// PromoteVerdict is one launch's reading of the home against its manifest.
// The zero value is OK: no manifest, nothing promoted, nothing to say.
type PromoteVerdict struct {
	Manifest *PromoteManifest
	Changed  []string      // hashed differently than the manifest records
	Missing  []string      // the manifest names it; the home does not have it
	Added    []string      // the home has it; the manifest does not name it
	Err      error         // the manifest or the home could not be read at all
	Elapsed  time.Duration // what the check cost (ADR 0015 marks this ASSUMED-negligible)
}

// OK is whether the launch may proceed unmarked.
func (v PromoteVerdict) OK() bool {
	return v.Err == nil && len(v.Changed) == 0 && len(v.Missing) == 0 && len(v.Added) == 0
}

// Line names the mismatch in one line, worst class first and at most three
// paths per class — a launch refusal has to be readable in a dispatch log,
// and "which file" is the operator's next question either way.
func (v PromoteVerdict) Line() string {
	if v.Err != nil {
		return "constitution manifest unreadable: " + v.Err.Error()
	}
	if v.OK() {
		return "constitution matches its manifest"
	}
	var parts []string
	for _, c := range []struct {
		label string
		paths []string
	}{
		{"changed", v.Changed},
		{"missing", v.Missing},
		{"unpromoted", v.Added},
	} {
		if len(c.paths) == 0 {
			continue
		}
		shown := c.paths
		more := ""
		if len(shown) > 3 {
			more = fmt.Sprintf(" (+%d more)", len(shown)-3)
			shown = shown[:3]
		}
		parts = append(parts, fmt.Sprintf("%s %s%s", c.label, strings.Join(shown, ", "), more))
	}
	return "constitution does not match its manifest: " + strings.Join(parts, "; ")
}

// VerifyPromoted hashes the home's promoted set and compares it with the
// manifest. `envs/` is not in the promoted set and never reaches this
// comparison (ADR 0015 §7): a corrupted or hand-edited env file must NOT
// trip a launch, because editing an env set at the home is a supported live
// path (WriteEnvSet) and a verify that fires on correct routine behaviour
// trains everyone to ignore it. The controls envs/ gets instead are
// TightenEnvPerms on every read and a seatbelt that never grants the dir.
//
// No manifest = no promotion has happened here = OK. That is what keeps a
// pre-ADR-0015 home, and a `RHQ_HOME` pointed at testdata, launching.
func (a *App) VerifyPromoted() PromoteVerdict {
	start := time.Now()
	v := PromoteVerdict{}
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil {
		v.Err, v.Elapsed = err, time.Since(start)
		return v
	}
	if m == nil {
		v.Elapsed = time.Since(start)
		return v
	}
	v.Manifest = m
	have, err := HashPromotedSet(a.Home)
	if err != nil {
		v.Err, v.Elapsed = err, time.Since(start)
		return v
	}
	for p, want := range m.Files {
		got, ok := have[p]
		switch {
		case !ok:
			v.Missing = append(v.Missing, p)
		case got != want || want == notRegular:
			v.Changed = append(v.Changed, p)
		}
	}
	for p := range have {
		if _, ok := m.Files[p]; !ok {
			v.Added = append(v.Added, p)
		}
	}
	sort.Strings(v.Changed)
	sort.Strings(v.Missing)
	sort.Strings(v.Added)
	v.Elapsed = time.Since(start)
	return v
}

// ─── promotion ───────────────────────────────────────────────────────────────

// PromoteOpts is what `posse promote` was asked for.
type PromoteOpts struct {
	// Source is the constitution directory — the one holding agents/,
	// config.yaml, recipes/, runtimes/ and skills/. "" resolves it, in order: the
	// manifest's own record of where the last promote came from, then
	// config `constitution:`. There is no cwd default: promoting the wrong
	// tree writes the fleet's prose.
	Source string
	// DryRun prints the diff and everything that would change, and writes
	// nothing — the ratification read, separated from the act.
	DryRun bool
}

// CmdPromote copies the constitution into the home and records what it
// copied. Operator-run: it refuses under the persona env marker.
func (a *App) CmdPromote(w io.Writer, o PromoteOpts) error {
	if p := os.Getenv(EnvPersona); p != "" {
		return Die("posse promote is the operator's (ADR 0015 §3): refusing under %s=%s", EnvPersona, p)
	}
	// A manifest posse cannot read must not block the one command that
	// replaces it — promote IS the fix for a broken manifest, and the launch
	// verify is already refusing dispatch by the time anyone runs this. Say
	// so and carry on with no baseline; only the ratification diff is poorer.
	prev, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil {
		fmt.Fprintf(w, "note: %v — promoting without a baseline to diff against\n", err)
		prev = nil
	}
	src, err := a.resolvePromoteSource(o.Source, prev)
	if err != nil {
		return err
	}
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return Die("constitution not found: %s", AbbrevHome(src))
	}
	// The home may not BE the constitution. Before the cutover
	// `~/.config/rhq` is a symlink onto the instance repo — promoting there
	// would copy a tree onto itself and then "remove what the source no
	// longer has" from the source. Refuse both directions.
	if st, err := os.Lstat(a.Home); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return Die("%s is a symlink; the promoted home must be a real directory (ADR 0015 §2) — remove the link first", AbbrevHome(a.Home))
	}
	if underDir(a.Home, src) || underDir(src, a.Home) {
		return Die("%s and the home %s are the same tree — nothing to promote", AbbrevHome(src), AbbrevHome(a.Home))
	}

	repo, sha, err := promoteRef(src)
	if err != nil {
		return err
	}
	if err := promoteCleanGate(w, repo, src); err != nil {
		return err
	}

	// The diff comes first, because it is the thing being ratified: what
	// this promote puts in force that the last one did not.
	printPromoteDiff(w, repo, src, prev, sha)

	set, files, err := promotedAtCommit(repo, src, sha)
	if err != nil {
		return err
	}
	if bad := notRegularIn(files); len(bad) > 0 {
		return Die("not a regular file, so it cannot be promoted: %s", strings.Join(bad, ", "))
	}

	if o.DryRun {
		fmt.Fprintf(w, "dry run: %d files from %s @ %s would be promoted into %s\n",
			len(files), AbbrevHome(src), short(sha), AbbrevHome(a.Home))
		for _, p := range promoteRemovals(a.Home, files) {
			fmt.Fprintf(w, "  would remove %s (not in the constitution)\n", p)
		}
		return nil
	}

	if err := os.MkdirAll(a.Home, 0o755); err != nil {
		return err
	}
	if err := a.copyPromotedSet(w, set, files); err != nil {
		return err
	}
	m := &PromoteManifest{
		Version:    promoteManifestVersion,
		PromotedAt: time.Now().UTC().Format(time.RFC3339),
		Source:     src,
		Repo:       repo,
		SHA:        sha,
		Files:      files,
	}
	if err := m.write(a.PromoteManifestPath()); err != nil {
		return err
	}
	fmt.Fprintf(w, "promoted %s @ %s → %s (%d files, manifest %s)\n",
		AbbrevHome(src), short(sha), AbbrevHome(a.Home), len(files), PromoteManifestFile)
	// ADR 0015 §7's tripwire, and the reason it is here rather than in the
	// runbook: the failure it catches is an instance coming up on the far
	// side of the cutover window with no env sets and no error at all.
	warnDanglingDefaultEnv(w, a)
	// The second tripwire, and here for the same reason as the first: this
	// is the operator's regular touch point, and the thing it catches is
	// invisible from anywhere a session runs. The L3 walls are copies of a
	// render compiled into the binary, and only a session create or a typed
	// `posse gates install-hooks` refreshes one — so the repo that holds the
	// constitution, which holds no session, is the repo whose wall goes
	// stale first and is noticed last (ranger-base-ixv4: it waved a
	// promoted-class commit through hours after the constitution arm
	// shipped). Read-only: it names what to re-render and re-renders
	// nothing.
	a.ReportHookWall(w, "promote")
	return nil
}

// resolvePromoteSource settles which tree is the constitution. There is no
// "current directory" rung on purpose — a promote that guesses is a promote
// that puts some checkout's agents/ in front of the whole fleet.
func (a *App) resolvePromoteSource(arg string, prev *PromoteManifest) (string, error) {
	if arg != "" {
		return absResolve(ExpandTilde(arg)), nil
	}
	if prev != nil && prev.Source != "" {
		return absResolve(prev.Source), nil
	}
	if v := a.CfgGet("constitution", ""); v != "" {
		return absResolve(ExpandTilde(v)), nil
	}
	return "", Die("no constitution named: pass the directory (posse promote <dir>) or set `constitution:` in %s\n"+
		"  it is the directory holding %s", AbbrevHome(a.ConfigPath), strings.Join(PromotedPaths, ", "))
}

// promoteRef is the source's git root and the commit being promoted.
func promoteRef(src string) (repo, sha string, err error) {
	repo, err = git(src, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", Die("%s is not in a git repo, so there is no commit to promote from (ADR 0015 §3)", AbbrevHome(src))
	}
	sha, err = git(src, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	return repo, sha, nil
}

// promoteCleanGate is §3's precondition — uncommitted prose can never be in
// force — narrowed to the paths that ARE the prose.
//
// DIVERGED from ADR 0015 §3's literal wording ("the constitution repo's
// working tree is clean"), and the divergence is the ADR's own two
// carve-outs biting: the repo also holds `.beads` (§4 moves it out, and bd
// rewrites issues.jsonl continuously until then) and `personas/` (§5,
// persona memory, written at every session end and deliberately NOT
// promoted). Both were dirty in the live constitution repo the hour this
// was built, and neither is prose this promote puts in force. A whole-tree
// gate would therefore be unsatisfiable in practice — a gate nobody can
// pass is a gate that gets bypassed. So: a hard refusal on a dirty PROMOTED
// path, which is exactly the attributability the ADR asked for, and a loud
// warning naming anything else dirty, so the operator still sees the tree
// they are promoting out of. ranger-base-o943 files the amendment.
func promoteCleanGate(w io.Writer, repo, src string) error {
	specs, err := promotePathspecs(repo, src)
	if err != nil {
		return err
	}
	// specs is now always len(PromotedPaths) — ranger-base-70ry dropped the
	// os.Stat that used to make it empty when the working tree held none of
	// them. The "is there anything to promote?" refusal moved to
	// promotedAtCommit, which asks the commit instead of the working tree
	// and already carries the true version of it.
	//
	// --ignored=matching so a promoted path that git is forbidden to carry
	// (the shape ranger-base-h56a found in envs/) is a refusal here rather
	// than a manifest entry with no commit behind it.
	out, err := git(repo, append([]string{"status", "--porcelain", "--ignored=matching", "--"}, specs...)...)
	if err != nil {
		return err
	}
	if dirty := strings.TrimSpace(out); dirty != "" {
		return Die("the constitution's working tree is dirty, so there is nothing attributable to promote (ADR 0015 §3):\n%s\n"+
			"  commit it in %s, then promote", indentLines(dirty, "  "), AbbrevHome(repo))
	}
	// `git status` cannot report a path git has been told to stop watching
	// (`update-index --skip-worktree` / `--assume-unchanged`), so neither can
	// the gate above. Since ranger-base-znma promote reads the commit, so
	// such a path is no longer a way to put unratified prose in force — but
	// a local edit there is silently NOT what goes into force, and an
	// operator who ran `--assume-unchanged` on `config.yaml` months ago
	// deserves to be told that in one line rather than to wonder.
	if hidden := unwatchedPaths(repo, specs); len(hidden) > 0 {
		fmt.Fprintf(w, "note: git is not watching %s (update-index --skip-worktree/--assume-unchanged)\n"+
			"  promote puts the COMMIT's bytes in force, so any local edit there is not promoted\n",
			strings.Join(hidden, ", "))
	}
	// Everything else in the repo: reported, never a refusal.
	rest, err := git(repo, "status", "--porcelain")
	if err == nil {
		if s := strings.TrimSpace(rest); s != "" {
			fmt.Fprintf(w, "note: %s has uncommitted changes outside the promoted set (not promoted, not blocking):\n%s\n",
				AbbrevHome(repo), indentLines(s, "  "))
		}
	}
	return nil
}

// unwatchedPaths names the promoted paths whose index entry carries the
// skip-worktree ('S') or assume-unchanged (a lowercase tag) bit — the two
// ways `git status` is told to stop answering for a file. A read, never a
// gate: enumerating the ways git can be told to look away is exactly the
// game promotedAtCommit stops playing.
func unwatchedPaths(repo string, specs []string) []string {
	out, err := git(repo, append([]string{"ls-files", "-v", "--"}, specs...)...)
	if err != nil {
		return nil
	}
	var hidden []string
	for _, ln := range strings.Split(out, "\n") {
		if len(ln) < 3 || ln[1] != ' ' {
			continue
		}
		if tag := ln[0]; tag == 'S' || (tag >= 'a' && tag <= 'z') {
			hidden = append(hidden, strings.TrimSpace(ln[2:]))
		}
	}
	sort.Strings(hidden)
	return hidden
}

// promotePathspecs is the promoted set as pathspecs relative to the repo
// root — what git status, git diff and git ls-tree are scoped to.
//
// It used to `os.Stat` each of PromotedPaths under `src` and drop the ones
// the WORKING TREE did not have. That is ranger-base-70ry: a working tree
// narrowed by `git sparse-checkout` (or a path merely `rm -rf`'d) reports a
// path absent that the commit still carries, so every downstream consumer
// — the clean gate, unwatchedPaths, promotedAtCommit, the diff — was scoped
// to a subset of the promoted set and never knew the rest existed. All four
// consumers tolerate a pathspec that matches nothing (measured: `git
// status`, `git ls-files -v`, `git ls-tree` and `git diff` all exit 0 and
// simply omit it), so there is no working-tree read left to do here at all
// — the commit is the only thing that gets to decide the set.
func promotePathspecs(repo, src string) ([]string, error) {
	rel, err := filepath.Rel(absResolve(repo), absResolve(src))
	if err != nil {
		return nil, err
	}
	specs := make([]string, 0, len(PromotedPaths))
	for _, p := range PromotedPaths {
		specs = append(specs, path.Join(filepath.ToSlash(rel), p))
	}
	return specs, nil
}

// ─── the promoted set, read out of the commit ────────────────────────────────

// promotedFile is one file of the promoted set AS THE COMMIT CARRIES IT:
// the blob's bytes, the mode git records, and the sha256 the manifest will
// name. Nothing here was read from the working tree.
type promotedFile struct {
	Rel  string      // slash path relative to the constitution dir
	Mode os.FileMode // 0644 or 0755 — the only two a git blob can mean
	Body []byte      // the blob's bytes, no filters applied
	Sum  string      // sha256 of Body, or notRegular
	oid  string      // the blob it came from, for the batch read
}

// promotedAtCommit is ADR 0015 §3's invariant made structural rather than
// gated: "the promoted bytes equal the bytes at the recorded SHA."
//
// It used to be HashPromotedSet(src) — a walk of the constitution's WORKING
// TREE, trusted because `git status` had just called that tree clean. It is
// not the same claim. `git update-index --skip-worktree` (and
// `--assume-unchanged`) tell git to stop reporting a file's working-tree
// state, and neither the clean gate nor the launch verify can then see the
// difference: promote writes the edited bytes into force and the manifest
// records a SHA whose blob disagrees, while the ratification diff prints
// "re-promoting the same commit". That is ranger-base-znma, and it is why
// this reads the object store instead. There is no flag that makes
// `cat-file` return an edit nobody committed.
//
// The gate stays where it was, for the thing it can still answer honestly:
// an uncommitted edit git DOES report is a refusal naming the path, because
// silently promoting the old bytes under the operator's nose is its own
// kind of lie.
func promotedAtCommit(repo, src, sha string) ([]promotedFile, map[string]string, error) {
	specs, err := promotePathspecs(repo, src)
	if err != nil {
		return nil, nil, err
	}
	prefix, err := promoteRelPrefix(repo, src)
	if err != nil {
		return nil, nil, err
	}
	out, err := gitRaw(repo, append([]string{"ls-tree", "-r", "-z", "--full-tree", sha, "--"}, specs...)...)
	if err != nil {
		return nil, nil, err
	}
	var set []promotedFile
	var oids []string
	for _, ent := range strings.Split(string(out), "\x00") {
		if ent == "" {
			continue
		}
		f, oid, err := parseLsTreeEntry(ent, prefix)
		if err != nil {
			return nil, nil, err
		}
		set = append(set, f)
		if oid != "" {
			oids = append(oids, oid)
		}
	}
	if len(set) == 0 {
		return nil, nil, Die("%s carries none of %s at %s — there is nothing to promote",
			AbbrevHome(repo), strings.Join(PromotedPaths, ", "), short(sha))
	}
	bodies, err := gitCatBlobs(repo, oids)
	if err != nil {
		return nil, nil, err
	}
	files := make(map[string]string, len(set))
	for i := range set {
		f := &set[i]
		if f.Sum == notRegular {
			files[f.Rel] = notRegular
			continue
		}
		b, ok := bodies[f.oid]
		if !ok {
			return nil, nil, Die("%s: %s is in %s but its blob could not be read", f.Rel, short(f.oid), short(sha))
		}
		sum := sha256.Sum256(b)
		f.Body, f.Sum = b, hex.EncodeToString(sum[:])
		files[f.Rel] = f.Sum
	}
	sort.Slice(set, func(i, j int) bool { return set[i].Rel < set[j].Rel })
	return set, files, nil
}

// parseLsTreeEntry reads one `ls-tree -r -z` record — `<mode> SP <type> SP
// <oid> TAB <path>` — into the promoted file it describes. A path git does
// not carry as a plain blob (a symlink at 120000, a submodule at 160000)
// comes back as notRegular, the same sentinel the working-tree walk uses,
// so the one refusal in CmdPromote covers both readings.
func parseLsTreeEntry(ent, prefix string) (promotedFile, string, error) {
	head, name, ok := strings.Cut(ent, "\t")
	if !ok {
		return promotedFile{}, "", Die("git ls-tree said something posse cannot read: %q", ent)
	}
	fields := strings.Fields(head)
	if len(fields) != 3 {
		return promotedFile{}, "", Die("git ls-tree said something posse cannot read: %q", ent)
	}
	rel := name
	if prefix != "" {
		if !strings.HasPrefix(name, prefix) {
			return promotedFile{}, "", Die("git ls-tree returned %q, which is outside the constitution at %q", name, prefix)
		}
		rel = strings.TrimPrefix(name, prefix)
	}
	f := promotedFile{Rel: rel}
	switch fields[0] {
	case "100644":
		f.Mode = 0o644
	case "100755":
		f.Mode = 0o755
	default:
		f.Sum = notRegular
		return f, "", nil
	}
	f.oid = fields[2]
	return f, f.oid, nil
}

// promoteRelPrefix is the constitution dir as git spells it inside the repo
// — "" when the constitution IS the repo root, "posse/" when it is a
// subdirectory. ls-tree answers in full repo-relative names; the manifest
// keys on paths relative to the constitution, the same as the home's.
func promoteRelPrefix(repo, src string) (string, error) {
	rel, err := filepath.Rel(absResolve(repo), absResolve(src))
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "", nil
	}
	return rel + "/", nil
}

// gitCatBlobs reads blob bodies straight out of the object store, in one
// `cat-file --batch` process rather than one per file. `--batch` applies no
// smudge, no eol conversion and no `export-subst` — which `git archive`
// would, and which is why this is not that: the bytes written into the home
// have to be the blob's bytes, or the manifest's sha256 attests to nothing.
//
// The stream is `<oid> SP <type> SP <size> LF <body> LF` per request, in
// request order.
func gitCatBlobs(repo string, oids []string) (map[string][]byte, error) {
	bodies := map[string][]byte{}
	if len(oids) == 0 {
		return bodies, nil
	}
	want := make([]string, 0, len(oids))
	seen := map[string]bool{}
	for _, o := range oids {
		if !seen[o] {
			seen[o], want = true, append(want, o)
		}
	}
	cmd := exec.Command("git", "-C", repo, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(want, "\n") + "\n")
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, Die("git cat-file --batch: %s", msg)
	}
	br := bufio.NewReader(bytes.NewReader(out))
	for _, oid := range want {
		hdr, err := br.ReadString('\n')
		if err != nil {
			return nil, Die("git cat-file --batch ended before %s", short(oid))
		}
		fields := strings.Fields(hdr)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, Die("git cat-file --batch says %s is %s", short(oid), strings.TrimSpace(hdr))
		}
		n, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, Die("git cat-file --batch gave %s no readable size: %q", short(oid), hdr)
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(br, body); err != nil {
			return nil, Die("git cat-file --batch truncated %s: %v", short(oid), err)
		}
		if _, err := br.ReadByte(); err != nil {
			return nil, Die("git cat-file --batch truncated %s: %v", short(oid), err)
		}
		bodies[oid] = body
	}
	return bodies, nil
}

// gitRaw is `git` without the trimming — ls-tree's -z records end in NUL and
// a promoted path is allowed to be spelled with whitespace.
func gitRaw(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, Die("git %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

// printPromoteDiff is the ratification surface: what the constitution's
// promoted paths did between the commit in force and the one being put in
// force. A first promote has no previous commit and says so.
func printPromoteDiff(w io.Writer, repo, src string, prev *PromoteManifest, sha string) {
	specs, err := promotePathspecs(repo, src)
	if err != nil || len(specs) == 0 {
		return
	}
	if prev == nil || prev.SHA == "" {
		what := "no previous promote is recorded"
		if prev != nil && prev.Seeded {
			what = "the home was seeded by `posse init`, not promoted from a commit"
		}
		fmt.Fprintf(w, "diff: %s — promoting %s whole\n", what, short(sha))
		return
	}
	if prev.SHA == sha {
		fmt.Fprintf(w, "diff: already at %s — re-promoting the same commit\n", short(sha))
		return
	}
	args := append([]string{"diff", prev.SHA + ".." + sha, "--"}, specs...)
	out, err := git(repo, args...)
	if err != nil {
		fmt.Fprintf(w, "diff: %s..%s could not be read (%v) — ratify by hand before trusting this promote\n", short(prev.SHA), short(sha), err)
		return
	}
	fmt.Fprintf(w, "diff %s..%s -- %s\n", short(prev.SHA), short(sha), strings.Join(specs, " "))
	if strings.TrimSpace(out) == "" {
		fmt.Fprintf(w, "  (no change to the promoted set)\n")
		return
	}
	fmt.Fprintln(w, out)
}

// copyPromotedSet writes the promoted set into the home and removes what the
// constitution no longer has. What it writes is what `promotedAtCommit`
// read out of the object store — never the working tree, so there is no
// path here that could pick up a byte the recorded SHA does not carry
// (ranger-base-znma). The removal is bounded to PromotedPaths — it can no
// more reach `envs/` than the copy can — and every removal is printed,
// because a file leaving the fleet's prose is as much a change as one
// arriving.
func (a *App) copyPromotedSet(w io.Writer, set []promotedFile, files map[string]string) error {
	for _, f := range set {
		to := filepath.Join(a.Home, filepath.FromSlash(f.Rel))
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(to, f.Body, f.Mode); err != nil {
			return err
		}
		// WriteFile's mode applies only when it CREATES the file, so a
		// second promote over an existing home would keep whatever mode
		// that file already had. The promoted copy is the constitution's,
		// modes included.
		if err := os.Chmod(to, f.Mode); err != nil {
			return err
		}
	}
	for _, rel := range promoteRemovals(a.Home, files) {
		if err := os.Remove(filepath.Join(a.Home, filepath.FromSlash(rel))); err != nil {
			return err
		}
		fmt.Fprintf(w, "  removed %s (not in the constitution)\n", rel)
	}
	return nil
}

// promoteRemovals is what the home holds inside the promoted set that the
// constitution being promoted does not, sorted.
func promoteRemovals(home string, files map[string]string) []string {
	have, err := HashPromotedSet(home)
	if err != nil {
		return nil
	}
	var out []string
	for p := range have {
		if _, ok := files[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func notRegularIn(files map[string]string) []string {
	var out []string
	for p, h := range files {
		if h == notRegular {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// warnDanglingDefaultEnv is ADR 0015 §7's tripwire. The promoted config.yaml
// may name a `default_env:` that the home has no env set for — which is
// exactly the state the cutover window passes through between the first
// promote and the hand-carry of the env files, and exactly the state that
// otherwise comes up silently. NAMES ONLY: an env set's values never reach
// any output posse writes.
func warnDanglingDefaultEnv(w io.Writer, a *App) {
	name := a.CfgGet("default_env", "")
	if name == "" {
		return
	}
	if _, err := a.envFilePath(name); err == nil {
		return
	}
	fmt.Fprintf(w, "warning: config default_env: names %q and %s has no such env set\n"+
		"  promote never carries env values (ADR 0015 §7 — they are gitignored secrets)\n"+
		"  during the cutover window this is expected here; the instance tree's\n"+
		"  docs/runbooks/home-cutover.md step 3 clears it\n",
		name, AbbrevHome(a.EnvsDir))
}

// SeedPromoteManifest gives a freshly seeded home a manifest of its own, so
// the launch verify has something true to check on a box that has never
// promoted anything — without it, a `posse init` install would either have
// to skip the verify forever or fail it on first launch. It hashes what is
// actually on disk (init copies only what is missing, so the seed's contents
// and the home's need not be the same — this is the anchor/provenance split
// on PromoteManifest.Seeded), never overwrites an existing manifest, and
// marks the result seeded: a real manifest with no commit behind it, which is
// the honest description of a fresh install.
//
// WHICH home is freshly seeded is the CALLER'S question, not this function's:
// by the time init has copied anything, the home it found is unrecoverable
// from disk. initFrom decides it (an empty promoted set and no manifest) and
// only calls this then — stamping a home that already had a constitution arms
// ADR 0015 §3 over prose nobody ratified, and the refusal lands on the
// unattended fleet at the operator's next config edit (ranger-base-h7cd).
func (a *App) SeedPromoteManifest() error {
	if m, err := ReadPromoteManifest(a.PromoteManifestPath()); err != nil || m != nil {
		return err
	}
	files, err := HashPromotedSet(a.Home)
	if err != nil {
		return err
	}
	m := &PromoteManifest{
		Version:    promoteManifestVersion,
		PromotedAt: time.Now().UTC().Format(time.RFC3339),
		Seeded:     true,
		Files:      files,
	}
	return m.write(a.PromoteManifestPath())
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func indentLines(s, pad string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = pad + ln
	}
	return strings.Join(lines, "\n")
}
