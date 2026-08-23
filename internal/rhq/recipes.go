package rhq

// Recipes: recipes/<name>.yaml — a saved session launch (flat-YAML subset).
// Keys: name, purpose, dir, env_files, command, agent, emoji.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Recipe struct {
	Name     string
	Purpose  string
	Dir      string
	Command  string
	Emoji    string
	Agent    string
	Runtime  string // launch profile override for the persona (ADR 0002)
	Tier     string // model tier override for the persona (ADR 0003)
	EnvFiles []string
	Path     string
}

func (a *App) recipePath(name string) (string, error) {
	for _, ext := range []string{".yaml", ".yml"} {
		p := filepath.Join(a.RecipesDir, name+ext)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", Die("no such recipe: %s (looked in %s)", name, a.RecipesDir)
}

func (a *App) LoadRecipe(rname string) (*Recipe, error) {
	f, err := a.recipePath(rname)
	if err != nil {
		return nil, err
	}
	r := &Recipe{
		Name:     YamlGet(f, "name"),
		Purpose:  YamlGet(f, "purpose"),
		Dir:      YamlGet(f, "dir"),
		Command:  YamlGet(f, "command"),
		Emoji:    YamlGet(f, "emoji"),
		Agent:    YamlGet(f, "agent"),
		Runtime:  YamlGet(f, "runtime"),
		Tier:     YamlGet(f, "tier"),
		EnvFiles: YamlList(f, "env_files"),
		Path:     f,
	}
	if r.Name == "" {
		r.Name = rname
	}
	return r, nil
}

func (a *App) ListRecipes() []string {
	ents, _ := os.ReadDir(a.RecipesDir)
	var out []string
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !(strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml")) {
			continue
		}
		n = strings.TrimSuffix(strings.TrimSuffix(n, ".yaml"), ".yml")
		out = append(out, n)
	}
	return out
}

// WriteRecipe writes the flat-YAML recipe file. A legacy slot_preference key
// (from the tmux era) is dropped on rewrite; LoadRecipe never reads it.
func (a *App) WriteRecipe(r *Recipe) error {
	if err := os.MkdirAll(a.RecipesDir, 0o755); err != nil {
		return err
	}
	nullable := func(s string) string {
		if s == "" {
			return "null"
		}
		return s
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", r.Name)
	if r.Purpose != "" {
		fmt.Fprintf(&b, "purpose: %s\n", r.Purpose)
	}
	fmt.Fprintf(&b, "dir: %s\n", r.Dir)
	fmt.Fprintf(&b, "env_files: [%s]\n", strings.Join(r.EnvFiles, ", "))
	fmt.Fprintf(&b, "command: %s\n", nullable(r.Command))
	if r.Agent != "" {
		fmt.Fprintf(&b, "agent: %s\n", r.Agent)
	}
	if r.Runtime != "" {
		fmt.Fprintf(&b, "runtime: %s\n", r.Runtime)
	}
	if r.Tier != "" {
		fmt.Fprintf(&b, "tier: %s\n", r.Tier)
	}
	fmt.Fprintf(&b, "emoji: %s\n", r.Emoji)
	return os.WriteFile(filepath.Join(a.RecipesDir, r.Name+".yaml"), []byte(b.String()), 0o644)
}
