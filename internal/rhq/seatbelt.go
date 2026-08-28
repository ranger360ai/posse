package rhq

// Gates L2 — the seatbelt tier (ADR 0002 §3): the runtime command runs
// under `sandbox-exec -f RHQ_HOME/state/gates/<persona>/seatbelt.sb`, a
// profile that denies file-write* everywhere except what a persona
// session legitimately writes: the repo (unless the PID denies Edit/
// Write — then only its .beads/), the persona's memory dir, the runtime's
// own state (~/.claude, ~/.codex, ~/.grok), posse's own state dir under the
// home it resolved, TMPDIR, the gates dir (for refusals.log), /dev, and the
// PID's `writable:` extras. What it never grants is the rest of the home:
// after ADR 0015 §2 that is the promoted constitution, and a promoted copy
// stays in force because no session can write it. This is the only
// runtime-proof file gate: it realizes Edit/Write-class denies on any
// runtime, model behind it notwithstanding. sandbox-exec is deprecated by
// Apple but is what codex itself ships on today; its successor is the
// container tier.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// SeatbeltAvailable reports whether this host can run the seatbelt tier.
func SeatbeltAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func init() {
	if SeatbeltAvailable() {
		AvailableCages[CageSeatbelt] = true
	}
}

// sbQuote quotes a path for SBPL (double-quoted string, backslash-escaped).
func sbQuote(p string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(p) + `"`
}

// SeatbeltProfile renders the SBPL profile text.
func SeatbeltProfile(persona string, writable []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, ";; posse seatbelt for %s — rendered from the PID at launch; do not edit (rangerhq-5vt)\n", persona)
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n")
	b.WriteString("(allow file-write*\n")
	for _, p := range writable {
		if p == "" {
			continue
		}
		fmt.Fprintf(&b, "  (subpath %s)\n", sbQuote(p))
	}
	b.WriteString("  (regex #\"^/dev/\")\n")
	b.WriteString("  (literal \"/dev/null\")\n")
	b.WriteString(")\n")
	return b.String()
}

// SeatbeltWritable computes the writable set for a persona session:
// cwd unless the PID denies Edit or Write (then only cwd/.beads so bd can
// still claim/comment/close), the store of record when a redirect puts it
// in another repo, memory dir, the runtime state dirs, posse's own state
// dir, TMPDIR, the gates dir, plus the PID's writable: extras (relative to
// cwd).
//
// It hangs off App because one of those paths is under the home, and after
// ADR 0015 §2 the home is a real directory holding the promoted
// constitution beside `state/`. A grant spelled as a literal path — which
// this was, `~/.config/rhq/state` — is a grant that names the wrong home
// the day the home moves, and the profile then silently loses the state
// dir it meant to open (ranger-base-cpyb). Ask the App: it resolved the
// home this process is actually running against.
func (a *App) SeatbeltWritable(ag *AgentFile, cwd, gatesDir string, stateDirs ...string) []string {
	home, _ := os.UserHomeDir()
	deniesFiles := deniesFileWrite(ag.Deny)
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		out = append(out, absResolve(p))
	}
	if cwd != "" {
		if deniesFiles {
			add(filepath.Join(cwd, ".beads"))
			add(filepath.Join(cwd, ".git")) // index refresh, hooks' own logs — never a push
		} else {
			add(cwd)
		}
		// A session worktree's `.git` is a FILE, and its index, HEAD, objects
		// and refs live outside the tree entirely (rangerhq-09o2). Granting
		// cwd alone leaves a persona that cannot commit in its own tree —
		// the same shape as the redirect grant below, for the session's own
		// repo instead of the store of record. Empty in the main checkout.
		for _, g := range LinkedGitDirs(cwd) {
			add(g)
		}
		// The store of record is not under cwd when a redirect moves it
		// (ADR 0012 D3-C): cwd/.beads holds a path and the database, its
		// jsonl, socket and lock live in the instance repo it names. The
		// two grants above then cover a directory bd never writes, and
		// every mutation lands outside the profile — measured
		// (ranger-base-rhw): `bd sync` and `bd export` fail on the db file
		// ("operation not permitted"), and a commit of anything in that
		// repo — the persona's own ORDERS.md included — fails on
		// .git/index.lock. Grant the resolved directory and that repo's
		// git dirs, and nothing else: the instance tree stays unwritable,
		// which is the point of the tier. Not conditional on deniesFiles —
		// cwd's subpath does not reach another repo either way.
		//
		// realizeCodex names the same directory for the runtimes that cage
		// themselves (runtime.go, ranger-base-0fb); this is that grant at
		// L2, same resolver, same chained-redirect bound.
		if home := beadsHome(cwd); !underDir(cwd, home) {
			add(home)
			for _, g := range beadsGitDirs(home) {
				add(g)
			}
		}
	}
	// §5's named exception: memory is not law. `home/personas` is a symlink
	// into the constitution repo, and this is the one grant that follows it
	// — absResolve resolves the link, so the profile matches the REAL
	// directory and the spelling cannot dodge the wall in either direction:
	// another persona's dir is not granted under either name, and the
	// session's own dir is granted under both.
	add(ag.MemoryDir)
	add(gatesDir)
	// posse's own state, derived from the home. Everything else under the
	// home — the promoted set, its manifest, `envs/` — is deliberately
	// absent, and ConstitutionGrants below is how that is checked rather
	// than asserted (ADR 0015 §2/§3/§7).
	add(a.StateDir)
	// The generic caches every CLI on this box writes through. These are
	// NOT runtime state and stay a literal: they belong to npm, to macOS and
	// to the XDG layout, not to any engine.
	for _, d := range []string{"Library/Caches", "Library/Logs", ".cache", ".npm", ".local/share"} {
		if home != "" {
			add(filepath.Join(home, d))
		}
	}
	// The runtimes' own state dirs. `~/.claude ~/.claude.json ~/.codex
	// ~/.grok` were spelled here as a literal until ADR 0012 D4, which is
	// why a third-party CLI declared in runtimes/<name>.yaml got a READ-ONLY
	// state dir under `cage: seatbelt` and no line anywhere said so: it
	// re-ran its first-run flow every launch, or died on a config write.
	//
	// The union of the built-ins, not just the launching runtime's: that is
	// what the literal granted, and narrowing it is a separate decision with
	// its own blast radius (a persona on one engine that shells out to
	// another). stateDirs is the LAUNCHING runtime's declaration on top —
	// a caller with no runtime in hand passes none and gets exactly today's
	// set.
	for _, rt := range builtinRuntimes {
		for _, d := range rt.StateDirs {
			add(ExpandTilde(d))
		}
	}
	for _, d := range stateDirs {
		add(ExpandTilde(d))
	}
	if t := os.Getenv("TMPDIR"); t != "" {
		add(t)
	}
	add("/private/tmp")
	add("/tmp")
	for _, w := range ag.Writable {
		w = ExpandTilde(w)
		if !filepath.IsAbs(w) && cwd != "" {
			w = filepath.Join(cwd, w)
		}
		add(w)
	}
	return dedupeStrings(out)
}

// HomeConstitutionPaths names what at the home is prose in force, and so
// must be in NO session's writable set (ADR 0015 §2/§3): the promoted set,
// the manifest that anchors it, and — §7 — the secret env values, which are
// not promoted but are no session's to write either.
//
// The three things at the home that are deliberately NOT here: `state/`,
// which is granted above because it is what a session's runtime data IS;
// `personas/<self>`, §5's named exception; and the gates dir, which lives
// under state/.
func (a *App) HomeConstitutionPaths() []string {
	var out []string
	for _, p := range PromotedPaths {
		out = append(out, filepath.Join(a.Home, p))
	}
	return append(out, filepath.Join(a.Home, "envs"), a.PromoteManifestPath())
}

// ConstitutionGrants reports which of those a writable set reaches. Empty
// is the property ADR 0015 §2 claims — "seatbelt never grants the home's
// constitution area" — and returning the offenders rather than a bool is
// what lets `posse gates` PRINT the answer: a wall an operator can read off
// the output is a wall that gets checked, and this one replaced a carve-out
// list nobody could audit.
//
// Containment is tested both ways round. A grant that covers the area is
// the obvious breach; a grant that lands INSIDE it (a PID's `writable:`
// naming `~/.config/posse/agents/x`) is the same breach spelled smaller.
func (a *App) ConstitutionGrants(writable []string) []string {
	var out []string
	for _, p := range a.HomeConstitutionPaths() {
		for _, w := range writable {
			if underDir(w, p) || underDir(p, w) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// absResolve is the path the sandbox will match on: absolute, with
// symlinks resolved over the longest existing prefix.
func absResolve(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return resolveExisting(p)
}

// underDir reports whether p is dir or inside it, compared as the sandbox
// sees them — /tmp and its /private/tmp real path are the same directory.
func underDir(dir, p string) bool {
	rel, err := filepath.Rel(absResolve(dir), absResolve(p))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// beadsGitDirs names the git directories a session writes when the store of
// record lives in another repo: bd's own `bd sync` commit takes
// index.lock in the per-worktree git dir, hooks and refs live in the
// common one, and outside a worktree those are one path. `git rev-parse`
// is the only thing that can tell them apart — a worktree's .git is a
// file, and beadsHome follows a redirect into one (bd worktree create
// writes the chained form). <repo>/.git leads regardless, so a target git
// cannot answer for still gets the grant it needs.
func beadsGitDirs(home string) []string {
	root := filepath.Dir(home)
	out := []string{filepath.Join(root, ".git")}
	for _, flag := range []string{"--git-dir", "--git-common-dir"} {
		b, err := exec.Command("git", "-C", root, "rev-parse", flag).Output()
		if err != nil {
			continue
		}
		g := strings.TrimSpace(string(b))
		if g == "" {
			continue
		}
		if !filepath.IsAbs(g) {
			g = filepath.Join(root, g)
		}
		out = append(out, g)
	}
	return dedupeStrings(out)
}

// RenderSeatbelt writes the profile for a persona session and returns its
// path (RHQ_HOME/state/gates/<persona>/seatbelt.sb).
func (a *App) RenderSeatbelt(ag *AgentFile, cwd string, stateDirs ...string) (string, error) {
	gatesDir := a.GatesDir(ag.Name)
	if err := os.MkdirAll(gatesDir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(gatesDir, "seatbelt.sb")
	prof := SeatbeltProfile(ag.Name, a.SeatbeltWritable(ag, cwd, gatesDir, stateDirs...))
	return p, os.WriteFile(p, []byte(prof), 0o644)
}

// SeatbeltReport renders the profile for a launch in cwd and prints the
// writable set it grants, then ADR 0015 §2's structural claim CHECKED
// against that set rather than restated: the home holds a promoted copy of
// the constitution, and what keeps a promoted copy in force is that no
// session can write it.
//
// It prints rather than asserts because the property replaced a carve-out
// deny-list, and a deny-list's whole failure mode is that nobody can tell
// by looking whether it is still complete. This one is one line under the
// set it is a property of; `posse gates <persona>` is where an operator
// reads it (ADR 0015 verification items 5 and 6).
func (a *App) SeatbeltReport(ag *AgentFile, cwd string, out io.Writer, stateDirs ...string) error {
	prof, err := a.RenderSeatbelt(ag, cwd, stateDirs...)
	if err != nil {
		return err
	}
	writable := a.SeatbeltWritable(ag, cwd, a.GatesDir(ag.Name), stateDirs...)
	fmt.Fprintf(out, "  %s rendered for cwd %s (writable set below):\n", AbbrevHome(prof), AbbrevHome(cwd))
	for _, w := range writable {
		fmt.Fprintf(out, "    w %s\n", AbbrevHome(w))
	}
	if bad := a.ConstitutionGrants(writable); len(bad) > 0 {
		for _, p := range bad {
			fmt.Fprintf(out, "    ✗ GRANT REACHES THE CONSTITUTION: %s (ADR 0015 §2)\n", AbbrevHome(p))
		}
		return nil
	}
	fmt.Fprintf(out, "    constitution at %s (agents/, config.yaml, recipes/, skills/, envs/, %s): in no grant above — ADR 0015 §2/§7\n",
		AbbrevHome(a.Home), PromoteManifestFile)
	fmt.Fprintf(out, "    memory %s is granted and no other persona's is — ADR 0015 §5\n", AbbrevHome(ag.MemoryDir))
	return nil
}

// SeatbeltPrefix is typed between the PATH assignment and the runtime
// command: the pane shell expands "$(cat {file})" first, then execs
// sandbox-exec, which execs the runtime inside the profile.
func SeatbeltPrefix(profile string) string {
	return "sandbox-exec -f " + shellQuote(profile) + " "
}

// resolveExisting resolves symlinks on the longest existing prefix of p
// and re-joins the rest: sandbox-exec matches real paths (/private/tmp,
// /private/var), and a path that does not exist yet (a fresh .git,
// .beads) must still land inside the allowed subtree.
func resolveExisting(p string) string {
	rest := []string{}
	cur := p
	for {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				real = filepath.Join(real, rest[i])
			}
			return real
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = append(rest, filepath.Base(cur))
		cur = parent
	}
}
