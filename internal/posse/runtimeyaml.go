package posse

// runtimes/<name>.yaml, v2 (ADR 0012 D4). The declarable surface a third
// party fills to onboard an engine posse has never heard of. D4's audit
// counted 17 touchpoints a yaml could not supply; this file carries the
// realizer-ADJACENT ones — the keys that change what the launch RENDERS or
// what the parity matrix may CLAIM, as opposed to the dispatch-contract
// keys (prompt:/record:/…) that live inline in LoadRuntime.
//
// Three rules hold for every key here, and they are the ones ADR 0013 §2
// wrote for prompt:/record: rather than new ones:
//
//   - ABSENT is the loud default, reached by doing nothing.
//   - PRESENT-BUT-WRONG refuses the load, naming the key and the spelling
//     that would work. A typo that reads as a declaration is the silence
//     this contract exists to remove.
//   - UNKNOWN is warned about. Until this file existed an unrecognized key
//     was dropped in silence, which is how `skils_flag:` becomes a persona
//     that cannot launch and `slef_sandbox:` becomes a seatbelt that
//     refuses to nest — a dead wall with a config file that looks right.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// printfFlag turns a declared flag value into the printf form {model} and
// {skills} render with.
//
// A value naming %s is used VERBATIM, which is the whole point: the shape
// posse renders is `fmt.Sprintf(form, shellQuote(value))`, and before this
// the form was always `f + " %s"` — a space between flag and value with no
// spelling that removed it. A glued dialect (`-c model=<id>`,
// `--model=<id>`) was therefore unexpressible from yaml, so every engine
// with that dialect had to hardcode its model in `command:` and forfeit
// per-tier mapping entirely (rangerhq-5p0d). The built-in codex runtime has
// carried the glued form in Go source since tiering landed; this is the
// same form, declarable.
//
// A bare flag keeps the historical separated shape, so every yaml written
// against the old rule renders exactly as it did.
//
// Anything else refuses. `-c model=%d` and `%s=%s` are not flags with a
// value in them, they are Sprintf format strings that would render
// `%!d(string=…)` or `%!s(MISSING)` onto a launch line — a rendered line
// nobody typed, which is worse than the gap it was fixing.
func printfFlag(key, val string) (string, error) {
	bare := strings.ReplaceAll(val, "%%", "") // an escaped percent is not a verb
	switch {
	case strings.Count(bare, "%") == 1 && strings.Contains(bare, "%s"):
		return val, nil
	case strings.Contains(bare, "%"):
		return "", fmt.Errorf("%s: %q — a printf form takes exactly one %%s and no other verb (glued: %q; separated: %q)",
			key, val, "-c model=%s", "--model")
	}
	return val + " %s", nil
}

// runtimeBool reads a declared boolean. Absent is the zero value; only
// `true` and `false` spell it. `skills_cwd: yes` refuses rather than
// reading as false, because a false read here is a launch that silently
// binds no skills at all.
func runtimeBool(path, key string) (bool, error) {
	switch v := YamlGet(path, key); v {
	case "":
		return false, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s: %q — want true or false", key, v)
	}
}

// runtimeFlagValue validates a key whose value posse APPENDS to a rendered
// launch line (`unattended:`). Two refusals, each for what a wrong value
// DOES rather than for how it looks:
//
//   - the first word must be the flag itself. A bare word is appended as a
//     POSITIONAL, and a positional on an interactive CLI is usually its
//     prompt — so the declaration meant to stop the session asking for
//     approval would instead hand it work nobody wrote.
//   - no shell punctuation anywhere. The rendered line is handed to a
//     shell, so a `;`, `|`, `&`, backquote or `$(` here does not configure
//     the CLI: it runs a second command with the whole session env, from a
//     file posse appends to every launch on this runtime.
//
// Quotes and backslashes are deliberately allowed: `--mode='a b'` is a
// legitimate spelling, and an unbalanced one breaks the launch loudly
// rather than running anything.
func runtimeFlagValue(key, val string) (string, error) {
	f := strings.Fields(val)
	if len(f) == 0 || !strings.HasPrefix(f[0], "-") {
		return "", fmt.Errorf("%s: %q — must begin with the flag itself (`-a never`, `--permission-mode auto`): posse appends this to the launch line, where a bare word lands as a positional and an interactive CLI reads that as its prompt", key, val)
	}
	if i := strings.IndexAny(val, ";|&<>()`$\n\r"); i >= 0 {
		return "", fmt.Errorf("%s: %q — %q is shell punctuation, and this value is appended to a rendered shell line: it would run a command with the session env rather than configure the CLI", key, val, string(val[i]))
	}
	return val, nil
}

// runtimeRelPath validates a key that names a file *inside the session
// directory*. Absolute is refused because the key's whole meaning is
// "relative to wherever this session starts"; a `..` element is refused
// because a check scoped to the session dir that reads outside it is not
// the check its callers think they are running.
func runtimeRelPath(key, val string) (string, error) {
	if filepath.IsAbs(val) {
		return "", fmt.Errorf("%s: %q — must be relative to the session directory, not an absolute path", key, val)
	}
	c := filepath.Clean(val)
	if c == "." || c == ".." || strings.HasPrefix(c, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: %q — must name a file inside the session directory (no .. elements)", key, val)
	}
	return c, nil
}

// runtimeYamlKeys is every key LoadRuntime reads. It is the ONLY list; the
// unknown-key warning below is generated from it, so a key that is parsed
// and not listed here warns on its own file — which is the cheapest
// possible way to keep the two in step.
//
// model_<tier> is expanded from Tiers rather than spelled, so a new tier
// cannot arrive as an unknown key.
func runtimeYamlKeys() []string {
	keys := []string{
		"command",
		"model_flag",
		"skills_flag", "skills_cwd",
		"self_sandbox",
		"project_config", "project_config_keys",
		"unattended",
		"cage_cred", "egress", "gate_shell",
		"prompt", "startup_wait", "record", "record_why", "native_rules", "turn_outcome",
		"rules_precedence", "rules_precedence_why",
		"state_dir", "env_required",
	}
	for _, t := range Tiers {
		keys = append(keys, "model_"+t)
	}
	return keys
}

// runtimeNoticeWriter is where LoadRuntime says it dropped a key. A var for
// noticeWriter's reason (beads.go): the SILENCE half — that a clean yaml
// warns nothing — is not observable if the line goes straight to stderr.
var runtimeNoticeWriter io.Writer = os.Stderr

var runtimeKeyNotices sync.Map

// warnUnknownRuntimeKeys names the top-level keys in a runtime yaml that
// nothing reads. A warning and not a refusal, deliberately: the file is the
// operator's own config root, a future posse may add keys an older one does
// not know, and refusing an instance's whole launch profile over a stray
// line is a bigger outage than the one being prevented. Loud is the
// requirement; fatal is not.
//
// Said once per (file, key set), for legacyHomeNotice's reason: LoadRuntime
// is called several times per command — `posse runtimes` walks every
// profile and a launch loads the chosen one again — and a notice that
// repeats is a notice that gets filtered out. Keying on the key set as well
// as the path means a long-lived process (the cockpit) warns again after
// the file is edited.
func warnUnknownRuntimeKeys(w io.Writer, name, path string) {
	if w == nil {
		return
	}
	known := map[string]bool{}
	for _, k := range runtimeYamlKeys() {
		known[k] = true
	}
	var unknown []string
	for _, k := range YamlKeysWithPrefix(path, "") {
		// The interstitial_<slug>: family is open by construction — the slug
		// names the screen and only the profile's author knows it — so it is
		// matched by prefix rather than listed. Its SUBkeys are closed and
		// refuse (declaredInterstitials); this loop only sees top level.
		if strings.HasPrefix(k, interstitialPrefix) {
			continue
		}
		if !known[k] {
			unknown = append(unknown, k+":")
		}
	}
	if len(unknown) == 0 {
		return
	}
	if _, loaded := runtimeKeyNotices.LoadOrStore(path+"\x00"+strings.Join(unknown, ","), struct{}{}); loaded {
		return
	}
	fmt.Fprintf(w, "posse: runtime %s: %s declares %s — nothing reads %s, so the declaration never arrives (`posse runtime check %s` prints what did)\n",
		name, AbbrevHome(path), strings.Join(unknown, " "), plural(len(unknown), "that key", "those keys"), name)
	fmt.Fprintf(w, "       known keys: %s, %s<name>:\n", strings.Join(runtimeYamlKeys(), ", "), interstitialPrefix)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// --- the preflight keys (ADR 0012 D4, rangerhq-tr8k) ---
//
// Three declarations a third party could not make, each of which broke a
// launch in a way nothing said out loud: where the CLI keeps its state (so
// `cage: seatbelt` does not make it read-only), what the session env must
// carry before a launch is worth attempting, and which first-run screen a
// fresh pane opens on.

// interstitialPrefix is the key family for a declared first-run screen:
//
//	interstitial_update:
//	  screen: "Update available! → 1. Update now  2. Skip"
//	  where: ~/.mycli/version.json
//	  key: dismissed_version
//	  silence: "the OPERATOR picks 3, once per release"
//	  danger: "the default option runs a package upgrade"
//
// A family rather than a list because flat-YAML has no list-of-maps (see
// yamlflat.go's deliberate limits), and the same shape `plan_guard_<window>:`
// already uses for a set whose members posse does not know in advance.
const interstitialPrefix = "interstitial_"

// interstitialSubkeys is the whole map. UNKNOWN IS A REFUSAL here, unlike a
// top-level key: the enclosing key is already recognized, so a `silense:`
// line is not "a newer posse may know this" — it is a dismissal that names
// no instruction, printed under a screen the operator has to answer by hand.
var interstitialSubkeys = []string{"screen", "where", "key", "silence", "danger"}

// runtimeScalarOrList reads a key that takes one value or several. The
// bead's spelling is `state_dir: <path>`, and a scalar read through YamlList
// returns nothing at all (yamlListLines enters block mode and then finds no
// `- ` items), so the singular form has to be tried first — otherwise the
// documented spelling parses as absent, which is the exact silence these
// keys exist to remove.
func runtimeScalarOrList(path, key string) []string {
	if v := YamlGet(path, key); v != "" && !strings.HasPrefix(v, "[") {
		return []string{v}
	}
	return YamlList(path, key)
}

// runtimeStateDirs validates `state_dir:`. Absolute or ~-prefixed only: the
// grant it feeds is a seatbelt literal, the session cwd is already writable,
// and "relative to the CLI's idea of home" is not something this can
// resolve — a relative path here would silently grant a directory under the
// session's tree and leave the real state dir read-only, which is the bug
// with an extra step.
func runtimeStateDirs(path string) ([]string, error) {
	var out []string
	for _, v := range runtimeScalarOrList(path, "state_dir") {
		if !strings.HasPrefix(v, "~") && !filepath.IsAbs(v) {
			return nil, fmt.Errorf("state_dir: %q — must be absolute or ~-prefixed (it is a path on this machine, not one relative to the session)", v)
		}
		if e := ExpandTilde(v); strings.Contains(e, "..") {
			return nil, fmt.Errorf("state_dir: %q — no .. elements; name the directory itself", v)
		}
		out = append(out, v)
	}
	return out, nil
}

// runtimeEnvRequired validates `env_required:`. NAMES ONLY, and that is
// enforced rather than documented: a `FOO=bar` entry refuses, because the
// one guarantee this key makes is that posse never reads, prints or
// forwards a secret's value — and a list an operator can put a value in is
// a list that ends up in a `posse runtime check` someone pastes into a bead.
func runtimeEnvRequired(path string) ([]string, error) {
	var out []string
	for _, v := range runtimeScalarOrList(path, "env_required") {
		if strings.ContainsAny(v, "=$ \t") {
			return nil, fmt.Errorf("env_required: %q — names only, never values (posse never reads what these hold, and this refusal is elided at the first character that is not part of a name)", elideEnvValue(v))
		}
		for i, r := range v {
			if !envNameRune(i, r) {
				return nil, fmt.Errorf("env_required: %q — not an environment variable name (elided at the first character that is not one)", elideEnvValue(v))
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// envNameRune is the character rule for an environment variable NAME, and
// it is one function because two readers need the same answer: the
// validator above, which refuses an entry, and elideEnvValue, which decides
// how much of a refused entry is safe to print. Two spellings of this rule
// would drift, and the direction that drifts is the one that prints.
func envNameRune(i int, r rune) bool {
	return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9')
}

// elideEnvValue is what a refusal prints in place of the entry. The
// guarantee above — posse never reads, prints or forwards a value — was
// broken by the sentence that makes it: the refusal echoed the whole entry,
// so `AWS_SECRET_ACCESS_KEY=wJalr…` arrived complete in `posse runtime
// check` output, which is output meant to be pasted into a bead, and beads
// sync to a store repo (ranger-base-60lj).
//
// What survives is the leading run of name characters plus the first
// character that is not one — the operator has to see WHICH entry and which
// separator made it a value — and "..." for everything after it, which is
// the part that may be a secret. No arm is exempt: `=`, `$`, a space and a
// stray `-` all cut the same way, because `=` is not the only spelling a
// value arrives in and a rule with an exception leaks on the day the
// exception was wrong. An entry with nothing after the offending character
// (`FOO=`) is printed whole, because nothing was elided.
func elideEnvValue(v string) string {
	for i := 0; i < len(v); i++ {
		// Byte-indexed on purpose: any non-ASCII byte fails envNameRune at
		// its first byte, so this cuts in the same place the rune-indexed
		// validator refuses, and never mid-name.
		if envNameRune(i, rune(v[i])) {
			continue
		}
		if i+1 < len(v) {
			return v[:i+1] + "..."
		}
		return v
	}
	return v
}

// declaredInterstitials reads the interstitial_<slug>: family.
//
// Probe stays nil for every declared entry, which prints as "state unknown"
// rather than as "not silenced": posse cannot read an unknown CLI's config
// format, and a probe it guessed at would answer "no" for a screen the
// operator silenced years ago. The three built-ins keep their measured Go
// probes (interstitial.go) — a declaration names the screen, it does not
// claim to have looked.
//
// Seeded is likewise never declarable. Seeding is posse WRITING the
// operator's config, and the one place it does that was argued to a
// standstill in rangerhq-w4uf; a yaml key that turned it on for an
// arbitrary file would hand that decision to whoever wrote the profile.
func declaredInterstitials(path string) ([]Interstitial, error) {
	var out []Interstitial
	known := map[string]bool{}
	for _, k := range interstitialSubkeys {
		known[k] = true
	}
	for _, key := range YamlKeysWithPrefix(path, interstitialPrefix) {
		in := Interstitial{}
		for _, kv := range YamlMapPairs(path, key) {
			if !known[kv[0]] {
				return nil, fmt.Errorf("%s: has %s: — a declared screen takes %s", key, kv[0], strings.Join(interstitialSubkeys, ", "))
			}
			switch kv[0] {
			case "screen":
				in.Screen = kv[1]
			case "where":
				in.Where = kv[1]
			case "key":
				in.Key = kv[1]
			case "silence":
				in.Silence = kv[1]
			case "danger":
				in.Danger = kv[1]
			}
		}
		// A screen with no key silences nothing, and a key with no file is
		// not a place. Both halves are what `runtime check` prints for an
		// onboarder to go and set; an entry missing either is a note, not a
		// dismissal, so it refuses instead of printing a blank.
		if in.Screen == "" || in.Key == "" || in.Where == "" {
			return nil, fmt.Errorf("%s: needs screen:, where: and key: — the screen, the file that silences it, and the key in that file", key)
		}
		if in.Silence == "" {
			in.Silence = "the OPERATOR sets " + in.Key + " in " + in.Where + " — this profile did not say how, and posse never writes it"
		}
		out = append(out, in)
	}
	return out, nil
}
