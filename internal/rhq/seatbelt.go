package rhq

// Gates L2 — the seatbelt tier (ADR 0002 §3): the runtime command runs
// under `sandbox-exec -f RHQ_HOME/state/gates/<persona>/seatbelt.sb`, a
// profile that denies file-write* everywhere except what a persona
// session legitimately writes: the repo (unless the PID denies Edit/
// Write — then only its .beads/), the persona's memory dir, the runtime's
// own state (~/.claude, ~/.codex, ~/.grok), TMPDIR, the gates dir (for
// refusals.log), /dev, and the PID's `writable:` extras. This is the only
// runtime-proof file gate: it realizes Edit/Write-class denies on any
// runtime, model behind it notwithstanding. sandbox-exec is deprecated by
// Apple but is what codex itself ships on today; its successor is the
// container tier.

import (
	"fmt"
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
// in another repo, memory dir, the runtime state dirs, TMPDIR, the gates
// dir, plus the PID's writable: extras (relative to cwd).
func SeatbeltWritable(ag *AgentFile, cwd, gatesDir string) []string {
	home, _ := os.UserHomeDir()
	deniesFiles := false
	for _, r := range ag.Deny {
		if r == "Edit" || r == "Write" || r == "NotebookEdit" {
			deniesFiles = true
		}
	}
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
	add(ag.MemoryDir)
	add(gatesDir)
	for _, d := range []string{".claude", ".claude.json", ".codex", ".grok", "Library/Caches", "Library/Logs", ".cache", ".npm", ".local/share", ".config/rhq/state"} {
		if home != "" {
			add(filepath.Join(home, d))
		}
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
func (a *App) RenderSeatbelt(ag *AgentFile, cwd string) (string, error) {
	gatesDir := a.GatesDir(ag.Name)
	if err := os.MkdirAll(gatesDir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(gatesDir, "seatbelt.sb")
	prof := SeatbeltProfile(ag.Name, SeatbeltWritable(ag, cwd, gatesDir))
	return p, os.WriteFile(p, []byte(prof), 0o644)
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
