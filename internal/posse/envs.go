package posse

// Env sets: envs/<name>.env, plain KEY=VALUE lines (leading "export "
// tolerated, '#' comments skipped). Values are passed to tmux verbatim —
// no shell expansion — and never displayed anywhere; listings show names
// only. See "Env sets and secrets" in NOTES.md.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type EnvVar struct{ Key, Value string }

func (a *App) envFilePath(name string) (string, error) {
	if !storeName(name) {
		return "", Die("env set name must be a file stem, not a path: %q", name)
	}
	f := filepath.Join(a.EnvsDir, name+".env")
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	f = filepath.Join(a.EnvsDir, name) // allow names with or without .env
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	return "", Die("env set not found: %s (looked in %s)", name, a.EnvsDir)
}

// storeName reports whether name may be resolved inside a credential store
// directory. A set name is a file stem, never a path: a PID's `envs:` is
// prose the operator (and, below the promotion gate, a persona) writes, and
// `envs: [../secrets/plan-guard]` must not be a way to inject a HARNESS
// credential into a session — ADR 0019 D1's one-hand rule is the invariant,
// and this is where the two directories stay two.
func storeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

func parseEnvLines(data string) []EnvVar {
	var vars []EnvVar
	for _, line := range strings.Split(data, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		if i := strings.Index(line, "="); i > 0 {
			vars = append(vars, EnvVar{line[:i], line[i+1:]})
		}
	}
	return vars
}

// EnvSetVars reads one env set's variables, tightening the file's mode
// first if it has drifted from 600 (rangerhq-f2b).
func (a *App) EnvSetVars(name string) ([]EnvVar, error) {
	f, err := a.envFilePath(name)
	if err != nil {
		return nil, err
	}
	a.TightenEnvPerms(os.Stderr)
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	return parseEnvLines(string(b)), nil
}

// TightenEnvPerms restores envs/ to 700 and every env set to 600, noting
// each drifted path on w (names only, never contents). `posse init` sets the
// modes once, but files copied or edited by hand drift to 644 — and an env
// set is a plaintext secret store, so every launch re-asserts the modes.
// Cheap, idempotent, silent when nothing drifted (rangerhq-f2b).
func (a *App) TightenEnvPerms(w io.Writer) {
	tightenCredentialDir(w, a.EnvsDir, "env sets are readable by every process in their session")
}

// tightenCredentialDir is that belt for one credential store: the directory
// to 700, every .env file in it to 600, one line per path it had to fix.
// Both stores are plaintext credentials under the home and drift the same
// way, so they tighten the same way (ADR 0019 D1's perms parity) — the only
// thing that differs is `why`, the half-sentence that tells the operator
// what a widened mode would have exposed. A store that is not there is not
// a drift: nothing to fix, nothing to say.
func tightenCredentialDir(w io.Writer, dir, why string) {
	fix := func(p string, want os.FileMode) {
		st, err := os.Stat(p)
		if err != nil {
			return
		}
		if got := st.Mode().Perm(); got&0o077 != 0 {
			if err := os.Chmod(p, want); err == nil {
				fmt.Fprintf(w, "posse: %s was %04o — tightened to %04o (%s)\n", AbbrevHome(p), got, want, why)
			}
		}
	}
	fix(dir, 0o700)
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".env") {
			fix(filepath.Join(dir, e.Name()), 0o600)
		}
	}
}

// ListEnvSets returns env set names (envs/*.env, extension stripped), sorted.
func (a *App) ListEnvSets() []string {
	ents, _ := os.ReadDir(a.EnvsDir)
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".env") {
			out = append(out, strings.TrimSuffix(e.Name(), ".env"))
		}
	}
	return out
}

// WriteEnvSet rewrites (or creates) an env set, preserving nothing but the
// given vars — the TUI editor round-trips comments away by design; use
// $EDITOR to keep hand-written structure. Files stay 600 in a 700 dir.
func (a *App) WriteEnvSet(name string, vars []EnvVar) error {
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		return err
	}
	var b strings.Builder
	for _, v := range vars {
		b.WriteString(v.Key + "=" + v.Value + "\n")
	}
	f := filepath.Join(a.EnvsDir, name+".env")
	return os.WriteFile(f, []byte(b.String()), 0o600)
}

// EnsureEnvSet creates an empty env set file (700 dir / 600 file) if missing
// and returns its path — for handoff to $EDITOR.
func (a *App) EnsureEnvSet(name string) (string, error) {
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(a.EnvsDir, name+".env")
	if _, err := os.Stat(p); err != nil {
		if err := os.WriteFile(p, []byte("# KEY=VALUE lines; '#' comments; values are injected, never displayed\n"), 0o600); err != nil {
			return "", err
		}
	}
	return p, nil
}

func (a *App) DeleteEnvSet(name string) error {
	f, err := a.envFilePath(name)
	if err != nil {
		return err
	}
	return os.Remove(f)
}
