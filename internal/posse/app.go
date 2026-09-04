package posse

// App holds resolved paths and the substrate runners. posse is the
// business harness of the Ranger work system: it binds personas, env sets,
// and recipes to herdr (presentation/oversight) and beads (work graph).
// The tmux-era implementation lives on the tmux-reference branch.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	Version       = "0.4.0"
	FallbackEmoji = "⚙️"
)

// Build and VersionString live in version.go.

type App struct {
	Home       string
	ConfigPath string
	RecipesDir string
	EnvsDir    string
	// SecretsDir is the harness-credential store (secrets.go, ADR 0019 D1):
	// the other reader class, sibling to EnvsDir and never its fallback.
	// Both hold plaintext KEY=VALUE credentials under the home; what
	// separates them is that an env set is injected into the sessions a PID
	// names and this one is injected into nothing, ever.
	SecretsDir string
	StateDir   string
	AgentsDir  string
	// ExamplesDir is the reference shelf: what `posse init` lays down to
	// be READ rather than loaded. Only agents/ lives under it, and that
	// asymmetry is the point — a persona is the one seeded thing the
	// engine picks up by name, so shipping the examples into AgentsDir
	// made every generic a live lane that outranked the operator's own
	// crew alphabetically (ranger-base-qajs). Nothing reads this path;
	// ListAgents never looks here.
	ExamplesDir string
	// ModelLister is the account's model catalog reader (modelavail.go).
	// nil = NewModelLister, which is what every real launch uses; it is a
	// field for the same reason Dispatcher.Plan is one — so a test can hand
	// the availability preflight a fake endpoint instead of the operator's
	// keychain and the live API. A zero-value lister answers "not
	// configured", which the preflight reads as UNKNOWN and launches on,
	// so the hermetic default is also the fail-open one.
	ModelLister *ModelLister
	// Now is the clock the model-catalog preflight dates its reading by
	// (ModelCache.Now). nil = time.Now, which is what every real launch
	// uses. It is a field for the reason Dispatcher.Now is one: the
	// preflight's line quotes the reading's age to the whole minute
	// ("read 48h00m ago"), so a pin on that line over a wall-clock fixture
	// tolerates under 60s between the write and the render — a window a
	// loaded parallel run has been measured past (ranger-base-5hjyh).
	Now func() time.Time
	// Load1 reads the box's 1-minute load average for the load guard
	// (loadguard.go). nil = SysLoad1, the real box, which is what every
	// real launch uses. Tests set it for the reason newTestBackend sets
	// ModelLister: a guard whose reading comes from the machine the suite
	// happens to be running on is red per-day, not per-commit.
	Load1 func() (float64, error)
	// CIRead takes the ci-watch reading — is the gate red on the branch
	// dispatch merges into (ciwatch.go, ranger-base-x9e34). nil = ReadCI,
	// which forks `gh` against the real GitHub, and is what every real pass
	// uses. It is a seam for the reason Load1 is one, and for a sharper one:
	// a reading whose answer comes from whatever GitHub says at the moment
	// the suite runs is red per-day, not per-commit — and this reading FILES
	// AND CLOSES BEADS, so a pin that could not fix it would be writing into
	// the live store.
	CIRead func(CIQuery) CIState
	// TopCPU reads the box's process table for the load guard's culprit
	// line (loadguard.go). nil = SysTopCPU, one bounded `ps` on the real
	// box, which is what every real refusal uses. It is a field for the
	// same reason Load1 is one — a witness assembled from whatever the
	// suite's own machine happens to be running is red per-day, not
	// per-commit — and it is read ONLY on a pass the guard is already
	// skipping.
	TopCPU func() ([]Proc, error)
	// ReapOrphans ends the leaked gate-shell children the load guard's
	// kill arm selected, and reports what happened to each pid
	// (loadguardkill.go, ranger-base-gvp2p). nil = sysReapOrphans, the real
	// TERM/grace/KILL on the real box. It is a seam for the reason TopCPU
	// is one, and more urgently: this is the one arm of posse that signals
	// a process it did not start, and no unit pin may be able to reach the
	// machine the suite is running on.
	ReapOrphans func(targets []Proc) map[int]string
	// WorktreeRootDefault is what WorktreeRoot resolves to when config says
	// nothing. Empty = DefaultWorktreeRoot(), which is what every real App
	// has and every real launch takes; config still wins over it and the
	// under-$HOME rule still runs on the result (worktree.go). It is a
	// field for the reason ModelLister and Load1 are fields — a default
	// that otherwise names one process-wide directory, here
	// ~/.posse/worktrees, which two concurrent tests cutting a session tree
	// land in together (ADR 0047 D2). Nothing product-facing reads it.
	WorktreeRootDefault string
}

var legacyHomeNotices sync.Map

func NewApp() *App { return newApp(os.Stderr) }

func newApp(stderr io.Writer) *App {
	// RHQ_HOME wins when both are set (ranger-base-mlc Q2 ruling): an
	// operator's own exported RHQ_HOME, or an installed hook/shim that has
	// not been re-rendered since the both-names window opened, must not be
	// silently overridden by the new name. Drop the POSSE_HOME fallback read
	// once the window closes.
	home := os.Getenv("RHQ_HOME")
	if home == "" {
		home = os.Getenv("POSSE_HOME")
	}
	if home == "" {
		configDir := filepath.Join(os.Getenv("HOME"), ".config")
		preferred := filepath.Join(configDir, "posse")
		legacy := filepath.Join(configDir, "rhq")
		home = preferred
		if _, err := os.Stat(preferred); os.IsNotExist(err) {
			if st, err := os.Stat(legacy); err == nil && st.IsDir() {
				home = legacy
				legacyHomeNotice(stderr, preferred, legacy)
			}
		}
	}
	return NewAppAt(home)
}

// NewAppAt is an App rooted at an explicit home, with every path in the
// struct derived from it. One function knows the home's shape, so a caller
// that needs a path under the home asks the App instead of spelling
// `~/.config/...` itself — the seatbelt's state grant used to be spelled,
// and spelled the pre-0015 home (ADR 0015 §2, ranger-base-cpyb).
func NewAppAt(home string) *App {
	return &App{
		Home:        home,
		ConfigPath:  filepath.Join(home, "config.yaml"),
		RecipesDir:  filepath.Join(home, "recipes"),
		EnvsDir:     filepath.Join(home, "envs"),
		SecretsDir:  filepath.Join(home, "secrets"),
		StateDir:    filepath.Join(home, "state"),
		AgentsDir:   filepath.Join(home, "agents"),
		ExamplesDir: filepath.Join(home, "examples"),
	}
}

func legacyHomeNotice(stderr io.Writer, preferred, legacy string) {
	if stderr == nil {
		return
	}
	// Keep the fallback read-only: a durable marker in either home would
	// mutate the legacy instance or make the empty preferred home win on the
	// next command. One process says the transition once for each path pair.
	key := preferred + "\x00" + legacy
	if _, loaded := legacyHomeNotices.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	fmt.Fprintf(stderr, "posse: %s does not exist; using existing home %s (nothing moved)\n", preferred, legacy)
}

// CfgGet reads a top-level config scalar with a default.
func (a *App) CfgGet(key, def string) string {
	if v := YamlGet(a.ConfigPath, key); v != "" {
		return v
	}
	return def
}

// Coordinator is the persona the instance names as its exception handler
// (ADR 0033 §1): the one role dispatch never hires, because coordinator
// authority — session direction and push — must exist only in a session a
// human opened. One fact, one store: config `coordinator:`, the operator's
// file. Absent = no coordinator = pre-0033 behavior, so the engine ships
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

// ExpandTilde and AbbrevHome are the two halves of one convention, and both
// refuse to invent a home when $HOME is unset (ranger-base-a3t1). Empty home
// used to make every absolute path a child of it — HasPrefix(p, ""+"/") is
// true for all of them, so /etc/x PRINTED as ~/etc/x — and made every ~/...
// RESOLVE to the filesystem root: measured, `env -i posse beads check --dir
// ~/tmp` censused /tmp and answered "no lost beads" — a clean verdict over a
// directory nobody named. Unknown home: p unchanged, "~" stays "~".
// Reachable only where the environment is scrubbed (`env -i posse ...`, a
// unit file with no HOME); nothing posse itself ships invokes posse that way.
func ExpandTilde(p string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return p
	}
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
	if home == "" {
		return p
	}
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
