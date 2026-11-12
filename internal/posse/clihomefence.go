package posse

// The CLI-home fence a test binary puts round the runtimes' credential and
// config dirs, DERIVED from the resolvers and checked by ASKING them
// (ranger-base-vxir7).
//
// WHY IT LIVES HERE AND NOT IN A TEST FILE. Three test binaries need it —
// internal/posse, cmd/posse and internal/treepins — and test code cannot be
// shared across packages. Each of them used to fence these dirs by spelling
// variable NAMES in its own TestMain: internal/posse cleared three and missed
// CLAUDE_SECURESTORAGE_CONFIG_DIR (ranger-base-u3kft), the other two named
// one and missed it too (ranger-base-6sddu). That variable outranks every
// one they did name, and nothing redded, because a fence gap only shows on a
// seat whose launcher exports the variable and CI exports none.
//
// TWO HALVES, and the second is the one that holds:
//
//  1. CLIHomeOverrideVars is the list, derived from the resolvers' own
//     enumerations (credentialDirVars, pinned against credentialDirNamed's
//     behaviour; the runtime-home consts) rather than spelled again by each
//     binary. FenceCLIHomes UNSETS every one of them before it applies the
//     binary's own rows. Unset and never "": an empty
//     CLAUDE_SECURESTORAGE_CONFIG_DIR is a third state that shadows
//     CLAUDE_CONFIG_DIR and resolves to the home (credentialDirNamed).
//  2. CLIHomeFenceEscapes then asks the real resolvers where they land. A
//     variable no list knows about, or a resolver that picks up a new input,
//     shows up as an answer outside the fence — the code answers, not a
//     hand-kept list.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CLIHomeOverrideVars names every environment variable posse's runtime-home
// and credential-dir resolvers honour: credential.go's credentialDirVars
// (which carries CLAUDE_CONFIG_DIR) and the two runtime homes that are not in
// it. A fresh slice each call, so a caller cannot edit the resolvers' list.
func CLIHomeOverrideVars() []string {
	return append(append([]string{}, credentialDirVars...), CodexHomeVar, GrokHomeVar)
}

// cliHomeResolutions is every resolver the fence is about, asked in this
// process's environment, keyed by the resolver's name. An answer that cannot
// be had (no home at all) is "" with ok false, which the fence reads as an
// escape: an unknown location is not a fenced one.
func cliHomeResolutions() []struct {
	name   string
	answer string
	ok     bool
} {
	home := os.Getenv("HOME")
	cred, cerr := CredentialsFile()
	cfgFile := ClaudeConfigFile()
	_, cfgNamed := runtimeHomeOverride(ClaudeConfigDirVar)
	return []struct {
		name   string
		answer string
		ok     bool
	}{
		{"CredentialsFile", cred, cerr == nil},
		{"ClaudeConfigDirIn", ClaudeConfigDirIn(home), home != "" || cfgNamed},
		{"ClaudeConfigFile", cfgFile, cfgFile != ""},
		{"codexHomeIn", codexHomeIn(home), codexHomeIn(home) != ""},
		{"grokHomeIn", grokHomeIn(home), grokHomeIn(home) != ""},
	}
}

// CLIHomeFenceEscapes names each resolver whose answer, in this process's
// environment, is not root or under it. Empty means fenced. It returns
// resolver NAMES only and never the answers: an escaped answer is, by
// definition, a path into somebody's live store.
func CLIHomeFenceEscapes(root string) []string {
	// A relative or empty root fences nothing: every answer would be read
	// against whatever cwd the binary happens to have.
	fenced := filepath.IsAbs(root)
	var out []string
	for _, r := range cliHomeResolutions() {
		// pathUnder is worktree.go's, symlink-resolved, because macOS hands
		// out temp dirs behind /var -> /private/var.
		if !fenced || !r.ok || !pathUnder(r.answer, root) {
			out = append(out, r.name)
		}
	}
	return out
}

// FenceCLIHomes unsets every CLIHomeOverrideVars name, applies rows (a
// binary's own fence: the variables it points INTO root), and then asks the
// resolvers. A non-nil error names the escaped resolvers and the variable
// NAMES in force — never their values — and the caller is expected to stop
// the binary on it.
func FenceCLIHomes(root string, rows map[string]string) error {
	for _, name := range CLIHomeOverrideVars() {
		if err := os.Unsetenv(name); err != nil {
			return fmt.Errorf("CLI-home fence: unset %s: %v", name, err)
		}
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := os.Setenv(k, rows[k]); err != nil {
			return fmt.Errorf("CLI-home fence: set %s: %v", k, err)
		}
	}
	if esc := CLIHomeFenceEscapes(root); len(esc) > 0 {
		return fmt.Errorf("CLI-home fence: %s answer outside the fence %s; runtime-home variable names in force: %s",
			strings.Join(esc, ", "), root, strings.Join(cliHomeNamesInForce(), " "))
	}
	return nil
}

// cliHomeNamesInForce lists, by NAME only, every variable in the environment
// that could plausibly move a runtime's home: the honoured list, and any
// path-shaped name (…_DIR, …_HOME, …_PATH, …_FILE) carrying a runtime's
// prefix — the second half is what makes an escape through a variable no list
// knows about legible. Path-shaped only, because a seat carries a dozen
// CLAUDE_CODE_* session variables that would bury the one that matters.
func cliHomeNamesInForce() []string {
	seen := map[string]bool{}
	for _, name := range CLIHomeOverrideVars() {
		if _, ok := os.LookupEnv(name); ok {
			seen[name] = true
		}
	}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		pathy := false
		for _, s := range []string{"_DIR", "_HOME", "_PATH", "_FILE"} {
			pathy = pathy || strings.HasSuffix(name, s)
		}
		for _, p := range []string{"CLAUDE", "CODEX", "GROK"} {
			if pathy && strings.Contains(name, p) {
				seen[name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return []string{"(none)"}
	}
	return out
}
