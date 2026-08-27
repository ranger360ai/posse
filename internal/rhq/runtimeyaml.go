package rhq

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
		"project_config",
		"cage_cred", "egress", "gate_shell",
		"prompt", "startup_wait", "record", "record_why", "native_rules",
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
	fmt.Fprintf(w, "       known keys: %s\n", strings.Join(runtimeYamlKeys(), ", "))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
