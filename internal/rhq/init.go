package rhq

// CmdInit seeds RHQ_HOME from the example instance, never overwriting.
//
// The seed tree has two possible sources (ADR 0012 D5). examples/ beside the
// binary wins when it is there: that is a dev build run out of the checkout,
// where an edit to examples/ must take effect without a rebuild. Otherwise
// the copy embedded at build time is used — which is the whole point of the
// embed, because a release binary on a fresh laptop has no repo to read.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ranger360ai/posse"
)

// seedSource resolves the seed tree for a binary living in exeDir, and names
// where it came from — worth printing, because "embedded" and "a directory
// you can edit" behave differently on the next run.
func seedSource(exeDir string) (fs.FS, string) {
	if exeDir != "" {
		dir := filepath.Join(exeDir, "..", "examples")
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return os.DirFS(dir), dir
		}
	}
	return posse.Seed, "embedded"
}

func (a *App) CmdInit(w io.Writer) error {
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		exeDir = filepath.Dir(exe)
	}
	src, from := seedSource(exeDir)
	return a.initFrom(w, src, from)
}

// initFrom copies src into RHQ_HOME. Paths inside src are slash-separated
// (io/fs), paths under Home are the platform's — hence path vs filepath.
func (a *App) initFrom(w io.Writer, src fs.FS, from string) error {
	for _, d := range []string{a.Home, a.RecipesDir, a.EnvsDir, a.StateDir, a.AgentsDir, a.SkillsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	copyIfMissing := func(fromPath, to string, mode os.FileMode) error {
		if _, err := os.Stat(to); err == nil {
			return nil
		}
		b, err := fs.ReadFile(src, fromPath)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, mode)
	}
	if err := copyIfMissing("config.yaml", a.ConfigPath, 0o644); err != nil {
		return err
	}
	copyDir := func(fromDir, toDir string, mode os.FileMode) error {
		ents, _ := fs.ReadDir(src, fromDir)
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			if err := copyIfMissing(path.Join(fromDir, e.Name()), filepath.Join(toDir, e.Name()), mode); err != nil {
				return err
			}
		}
		return nil
	}
	// A skill is a directory (SKILL.md plus references/), so skills is the
	// one seed root that must be walked rather than listed. It is also the
	// one that may be absent — examples/skills is a later bead (ADR 0012
	// D2) — and an absent seed root seeds nothing, quietly.
	copyTree := func(fromDir, toDir string, mode os.FileMode) error {
		if st, err := fs.Stat(src, fromDir); err != nil || !st.IsDir() {
			return nil
		}
		return fs.WalkDir(src, fromDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(strings.TrimPrefix(p, fromDir), "/")
			if rel == "" {
				return nil
			}
			dst := filepath.Join(toDir, filepath.FromSlash(rel))
			if d.IsDir() {
				return os.MkdirAll(dst, 0o755)
			}
			return copyIfMissing(p, dst, mode)
		})
	}
	if err := copyDir("recipes", a.RecipesDir, 0o644); err != nil {
		return err
	}
	if err := copyDir("envs", a.EnvsDir, 0o600); err != nil {
		return err
	}
	if err := copyDir("agents", a.AgentsDir, 0o644); err != nil {
		return err
	}
	if err := copyTree("skills", a.SkillsDir(), 0o644); err != nil {
		return err
	}
	// Env sets hold secrets: keep them out of reach of other local users.
	os.Chmod(a.EnvsDir, 0o700)
	ents, _ := os.ReadDir(a.EnvsDir)
	for _, e := range ents {
		if !e.IsDir() {
			os.Chmod(filepath.Join(a.EnvsDir, e.Name()), 0o600)
		}
	}
	fmt.Fprintf(w, "initialized %s (seed: %s)\n", a.Home, from)
	return nil
}
