package rhq

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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PromotedPaths is the promoted set: what `posse promote` copies out of the
// constitution and what the launch verify hashes, relative to both the
// constitution source directory and the home. `config.yaml` is a file, the
// other three are trees.
//
// It is a var and not a const list so a test can prove the exclusions below
// are not in it; nothing mutates it at runtime.
var PromotedPaths = []string{"agents", "config.yaml", "recipes", "skills"}

// NotPromoted names what lives at the home and is never promoted — the
// list exists so the exclusion is greppable and testable, not so anything
// reads it in the copy path (the copy path only ever walks PromotedPaths).
var NotPromoted = []string{"envs", "state", "personas"}

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
	// config.yaml, recipes/ and skills/. "" resolves it, in order: the
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

	files, err := HashPromotedSet(src)
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
	if err := a.copyPromotedSet(w, src, files); err != nil {
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
	if len(specs) == 0 {
		return Die("%s holds none of %s — is it the constitution directory?", AbbrevHome(src), strings.Join(PromotedPaths, ", "))
	}
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

// promotePathspecs is the promoted set as pathspecs relative to the repo
// root — what git status and git diff are scoped to. Absent paths are left
// out rather than passed as pathspecs that match nothing.
func promotePathspecs(repo, src string) ([]string, error) {
	rel, err := filepath.Rel(absResolve(repo), absResolve(src))
	if err != nil {
		return nil, err
	}
	var specs []string
	for _, p := range PromotedPaths {
		if _, err := os.Stat(filepath.Join(src, p)); err == nil {
			specs = append(specs, path.Join(filepath.ToSlash(rel), p))
		}
	}
	return specs, nil
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
// constitution no longer has. The removal is bounded to PromotedPaths — it
// can no more reach `envs/` than the copy can — and every removal is
// printed, because a file leaving the fleet's prose is as much a change as
// one arriving.
func (a *App) copyPromotedSet(w io.Writer, src string, files map[string]string) error {
	names := make([]string, 0, len(files))
	for p := range files {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, rel := range names {
		from := filepath.Join(src, filepath.FromSlash(rel))
		to := filepath.Join(a.Home, filepath.FromSlash(rel))
		info, err := os.Stat(from)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(to, b, info.Mode().Perm()); err != nil {
			return err
		}
		// WriteFile's mode applies only when it CREATES the file, so a
		// second promote over an existing home would keep whatever mode
		// that file already had. The promoted copy is the constitution's,
		// modes included.
		if err := os.Chmod(to, info.Mode().Perm()); err != nil {
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
		"  during the cutover window this is expected here; docs/runbooks/home-cutover.md step 3 clears it\n",
		name, AbbrevHome(a.EnvsDir))
}

// SeedPromoteManifest gives a freshly seeded home a manifest of its own, so
// the launch verify has something true to check on a box that has never
// promoted anything — without it, a `posse init` install would either have
// to skip the verify forever or fail it on first launch. It hashes what is
// actually on disk (init copies only what is missing, so the seed's contents
// and the home's need not be the same), never overwrites an existing
// manifest, and marks the result seeded: a real manifest with no commit
// behind it, which is the honest description of a fresh install.
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
