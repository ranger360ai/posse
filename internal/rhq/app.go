package rhq

// App holds resolved paths and the substrate runners. posse is the
// business harness of the Ranger work system: it binds personas, env sets,
// and recipes to herdr (presentation/oversight) and beads (work graph).
// The tmux-era implementation lives on the tmux-reference branch.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	Version       = "0.3.0"
	FallbackEmoji = "⚙️"
)

// Build is the git SHA (+ "-dirty") stamped by the Makefile via -ldflags.
// "dev" means the binary was built some other way.
var Build = "dev"

// VersionString is what `posse version` and the cockpit header show.
func VersionString() string { return Version + "+" + Build }

type App struct {
	Home       string
	ConfigPath string
	RecipesDir string
	EnvsDir    string
	StateDir   string
	AgentsDir  string
	// ModelLister is the account's model catalog reader (modelavail.go).
	// nil = NewModelLister, which is what every real launch uses; it is a
	// field for the same reason Dispatcher.Plan is one — so a test can hand
	// the availability preflight a fake endpoint instead of the operator's
	// keychain and the live API. A zero-value lister answers "not
	// configured", which the preflight reads as UNKNOWN and launches on,
	// so the hermetic default is also the fail-open one.
	ModelLister *ModelLister
}

func NewApp() *App {
	home := os.Getenv("RHQ_HOME")
	if home == "" {
		home = filepath.Join(os.Getenv("HOME"), ".config", "rhq")
	}
	return &App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		RecipesDir: filepath.Join(home, "recipes"),
		EnvsDir:    filepath.Join(home, "envs"),
		StateDir:   filepath.Join(home, "state"),
		AgentsDir:  filepath.Join(home, "agents"),
	}
}

// CfgGet reads a top-level config scalar with a default.
func (a *App) CfgGet(key, def string) string {
	if v := YamlGet(a.ConfigPath, key); v != "" {
		return v
	}
	return def
}

// Coordinator is the persona the instance names as its exception handler
// (ADR 0018 §1): the one role dispatch never hires, because coordinator
// authority — session direction and push — must exist only in a session a
// human opened. One fact, one store: config `coordinator:`, the operator's
// file. Absent = no coordinator = pre-0018 behavior, so the engine ships
// carrying no crew name (the rangerhq-gk4k bug class).
func (a *App) Coordinator() string { return a.CfgGet("coordinator", "") }

// ─── small utilities ─────────────────────────────────────────────────────────

// A name may not begin with '-': it would read as a flag everywhere it is
// typed back (rangerhq-qv5 — `posse new --help` used to create a workspace
// literally named '--help').
var validNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)

func ValidName(name string) bool {
	return name != "" && validNameRe.MatchString(name)
}

func ExpandTilde(p string) string {
	home := os.Getenv("HOME")
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return home + "/" + strings.TrimPrefix(p, "~/")
	}
	return p
}

func AbbrevHome(p string) string {
	home := os.Getenv("HOME")
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~/" + strings.TrimPrefix(p, home+"/")
	}
	return p
}

func Die(format string, args ...any) error { return fmt.Errorf(format, args...) }

// EmojiFor consults config: exact match in the emoji map, then substring
// match, then default_emoji, then the fallback. (Explicit recipe emoji wins
// upstream of this.)
func (a *App) EmojiFor(name string) string {
	pairs := YamlMapPairs(a.ConfigPath, "emoji")
	for _, kv := range pairs {
		if kv[0] == name {
			return kv[1]
		}
	}
	for _, kv := range pairs {
		if strings.Contains(name, kv[0]) {
			return kv[1]
		}
	}
	return a.CfgGet("default_emoji", FallbackEmoji)
}
