package posse

// The persona mode (ADR 0062 D1): the PID channel for a CLI that has no
// launch-time system flag at all.
//
// Bob is the member, and the shape of the problem is bob's. `bob chat` takes
// no `--system-prompt`, no `--rules`, and — MEASURED 2026-10-01,
// ranger-base-5jjtn — no `-p` that submits a turn, so ADR 0060 D3's channel
// was dead and `BobCommand` carried no `{file}`: a dispatched seat spent a
// worktree, a pane and a `startup_wait` on a session carrying every native
// rulebook and no persona.
//
// What bob DOES have is a CUSTOM MODE, and a custom mode's `roleDefinition`
// is the `role_definition` section of bob's own system prompt — a
// launch-time system channel after all, reached through the filesystem
// rather than through argv. MEASURED 2026-10-03 (docs/notes.d/
// ranger-base-f1ytb.md §1-2, bob 2.0.5): bob loads
// `<workspace>/.bob/{custom_modes.yaml,plugins/*/custom_modes.yaml}`, `--mode
// <slug>` selects one, the slug alphabet is `^[a-zA-Z0-9-]+$`, an omitted
// `groups` is `[]` (a mode with NO tools), and `hidden` is a schema field.
//
// SO THE PID IS A FILE posse writes into the session tree, and the line
// selects it. That tree is where `.agents/skills` already lives (ADR 0007),
// and every property this channel needs is one the skills tree already has:
// session-local, written by the same call sites, excluded from `git status`
// with `.git/info/exclude`, mounted into the cage with the session dir,
// retired with the seat, and REFUSING rather than overwriting a path posse
// did not write.
//
// THE DIMENSION IS DECLARED, NEVER KEYED ON THE NAME (ADR 0017 §3): a
// runtime whose PID channel is a mode file carries a `PersonaMode` channel
// (runtime.go), and `{mode}` renders to its flag on such a runtime and to
// nothing on every other. Four readers share that one declaration —
// agents.go renders `{mode}` from it, this file writes the file and reads
// the pane's footer back, parity.go exempts posse's own entry from the
// project-config trust check, and pidchannel.go's `{mode}` is the
// placeholder it renders.
//
// AND THE TAKING IS AN OBSERVABLE (D2). `--mode <unknown>` is NOT a refusal
// on bob 2.0.5: it prints `(ℹ) Unknown mode "<slug>". Falling back to
// "agent" mode.` and opens in Agent Mode — one grey line and a session with
// no persona in it (MEASURED, notes §2). That is 5jjtn's `.allowUnknownOption()`
// class one layer up, so the launch does not trust the flag it typed: it
// reads the pane's own footer before the work prompt, and refuses to type at
// a bob that fell back.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PersonaModeChannel declares a runtime's persona-mode PID channel — the
// paths posse writes in the session tree, the flag that selects the mode,
// and the footer the pane prints when it took.
//
// Data and not behaviour, because there is one member and its YAML shape is
// bob's own. The day a second CLI arrives with a different file format, the
// format becomes a field here rather than an `rt.Name` branch anywhere.
type PersonaModeChannel struct {
	// Dir is the directory posse writes, relative to the session dir and
	// slash-separated. It is what `.git/info/exclude` gets, and the
	// namespace that keeps posse's file out of the way of the repo's own:
	// bob reads `plugins/*/custom_modes.yaml`, so `plugins/posse/` is a
	// subdirectory nobody else has a reason to use.
	Dir string
	// Trusted is the project-config entry Dir lives under — the glob the
	// CLI actually reads, which `bobProjectConfig` has to name because a
	// repo shipping its own entry there hands the CLI a system prompt and a
	// tool-group grant (ADR 0062 D1.4). posse's own Dir is exempt from that
	// check; everything else under Trusted is not.
	Trusted string
	// File is the modes file's name inside Dir.
	File string
	// GlobalRoot is the CLI's OWN configuration directory under the
	// operator's home, relative to it and slash-separated. It is NOT a path
	// posse writes — it is the one path posse must never write, and it is
	// here because the hazard is arithmetic rather than a name: bob reads
	// `plugins/*/custom_modes.yaml` under BOTH the workspace and the home,
	// so the very glob that makes Dir a session-local channel makes the
	// same Dir a GLOBAL one when the session directory happens to be the
	// home (MEASURED 2026-10-03, ranger-base-4mrmc F2; the home glob is in
	// docs/notes.d/ranger-base-f1ytb.md §1). A global-scope mode is
	// returned for an untrusted workspace too, so there is no second switch
	// behind which such a write would be inert.
	//
	// Declared and not derived, for ADR 0017 §3's reason: the next CLI's
	// config root is `~/.config/<cli>` as readily as `~/.<cli>`, and a
	// guard that recomputed it from Dir would be guessing at the half of
	// the pair that is NOT posse's to choose.
	GlobalRoot string
	// Flag is the printf form `{mode}` renders with, given the slug.
	Flag string
	// Prefix namespaces the slug, so posse's mode cannot collide with one
	// the operator or the repo declared.
	Prefix string
	// Groups are the tool groups the mode declares. MEASURED by reading the
	// 2.0.5 bundle: an omitted `groups` is `[]`, which is a mode with no
	// tools at all — so this is never left to a default.
	Groups []string
	// Footer is the printf form of the footer line the pane prints when the
	// mode TOOK, given the mode's name. The whole of D2's reading.
	Footer string
	// Fallback is the footer the pane prints when the slug was unknown and
	// the CLI fell back to its built-in mode: the reading that refuses.
	Fallback string
	// FallbackSentence is the CLI's own words for that fallback, quoted in
	// the refusal when it is still on screen.
	FallbackSentence string
}

// bobPersonaMode is the one member, MEASURED on bob 2.0.5 (notes §1-2).
var bobPersonaMode = &PersonaModeChannel{
	Dir:        ".bob/plugins/posse",
	Trusted:    ".bob/plugins",
	File:       "custom_modes.yaml",
	GlobalRoot: ".bob",
	Flag:       "--mode %s",
	Prefix:     "posse-",
	// The built-in agent mode's ten, read out of the bundle. `command` is
	// the spelling the mode file takes; bob maps it to `execute` itself.
	Groups:   []string{"read", "edit", "command", "browser", "mcp", "skill", "todo", "artifact", "subagent", "mode"},
	Footer:   "%s Mode",
	Fallback: "Agent Mode",
	// Quoted from the pane, 2026-10-03: `(ℹ) Unknown mode "nosuch-f1ytb".
	// Falling back to "agent" mode.` — matched on the invariant half, since
	// the slug in the middle is this launch's own.
	FallbackSentence: "Unknown mode",
}

// personaModeMarker is how posse recognises its own file. The never-clobber
// rule needs a reading, not a guess: a path posse did not write is the
// operator's or the repo's, and overwriting it is the one thing ADR 0007's
// skills tree refuses for.
const personaModeMarker = "# posse: this persona's PID, as a custom mode for this session only (ADR 0062 D1)"

// PersonaModeSlug is the mode's slug for this persona on this runtime, or ""
// when the runtime has no persona-mode channel.
//
// The persona name is reduced to the slug alphabet the CLI accepts
// (`^[a-zA-Z0-9-]+$`, MEASURED off the 2.0.5 schema). posse's own persona
// names are `^[A-Za-z0-9_][A-Za-z0-9_-]*$` (app.go), so `_` is the only
// character that is ever reduced and the result is never empty — but the
// reduction is written for the alphabet and not for that overlap, because
// the alphabet is the CLI's and the name rule is ours. Two personas whose
// names differ only by `_` vs `-` reduce to one slug; they cannot collide,
// because one session directory holds one persona's mode file.
func (rt *Runtime) PersonaModeSlug(persona string) string {
	c := rt.PersonaMode
	if c == nil || persona == "" {
		return ""
	}
	return c.Prefix + reduceToSlug(persona)
}

// PersonaModeText is what `{mode}` renders to — the flag that selects this
// persona's mode — and "" on a runtime with no such channel, where the
// placeholder vanishes with its space like `{skills}` and `{allow}`.
func (rt *Runtime) PersonaModeText(persona string) string {
	slug := rt.PersonaModeSlug(persona)
	if slug == "" {
		return ""
	}
	return fmt.Sprintf(rt.PersonaMode.Flag, slug)
}

// PersonaModeFile is the modes file this launch would write, absolute under
// the session dir, or "" when there is no channel or no session dir.
func (rt *Runtime) PersonaModeFile(dir string) string {
	if rt.PersonaMode == nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, filepath.FromSlash(rt.PersonaMode.Dir), rt.PersonaMode.File)
}

// reduceToSlug maps a persona name into the CLI's slug alphabet: everything
// outside `[a-zA-Z0-9-]` becomes `-`, runs collapse, and the ends are
// trimmed so the result cannot start or end with the separator. A name that
// reduces to nothing at all answers "persona", which is a slug and not a
// silently empty `--mode ` — the flag's own value going missing is the
// failure class this whole file is about.
func reduceToSlug(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(collapseDashes(b.String()), "-")
	if out == "" {
		return "persona"
	}
	return out
}

func collapseDashes(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// RenderPersonaModeFor writes the persona's PID into the session tree as
// this runtime's custom mode, and returns the file it wrote ("" when the
// runtime has no persona-mode channel — every runtime but bob today).
//
// RenderSkillsFor's sibling, called from the same places and for the same
// reason: a launch materializes what the line is about to point at. The
// three rules are the skills tree's three, one file instead of a tree of
// symlinks:
//
//   - NEVER CLOBBER. A file posse cannot recognise as its own refuses the
//     launch by name. So does one the repo TRACKS, marker or no marker: a
//     repo can ship any bytes it likes, posse's header included, and a
//     tracked path is the operator's file whatever it says inside.
//   - REWRITE, don't union. Unlike the skills tree this file belongs to the
//     PERSONA and not to the repo — one session directory, one persona —
//     so every launch writes the PID as it is NOW. A PID edited between a
//     create and a relaunch arrives.
//   - EXCLUDE. `.git/info/exclude`, never the repo's `.gitignore` (ADR
//     0007): the dir is session-local and must not show in anybody's diff.
//
// WHAT "THE PID BODY VERBATIM" IS, read as the same bytes `{file}` delivers:
// the WHOLE file, frontmatter included, and not `AgentFile.Body`. Every
// other runtime's channel is `"$(cat {file})"`, so this is the one reading
// under which a persona arrives the same on bob as on claude — and PIDs
// reference their own frontmatter in their prose (the deny list, the intents
// table), which a body-only render would silently drop. The file is re-read
// from disk here rather than taken off the loaded AgentFile for the same
// reason the relaunch re-renders everything: the bytes on disk are what the
// other runtimes are handed.
func (a *App) RenderPersonaModeFor(ag *AgentFile, rt *Runtime, dir string) (string, error) {
	c := rt.PersonaMode
	if c == nil {
		return "", nil
	}
	if dir == "" {
		return "", Die("%s: %s delivers the PID as a custom mode in the session's working directory and this session has none", ag.Name, rt.Name)
	}
	body, err := os.ReadFile(ag.Path)
	if err != nil {
		return "", Die("%s: cannot read the PID to render it as %s's mode: %v", ag.Name, rt.Name, err)
	}
	if strings.TrimSpace(string(body)) == "" {
		return "", Die("%s: %s is empty — a mode whose roleDefinition is empty is a session with no persona in it, which is the harm this channel exists to stop (ADR 0062 D1)", ag.Name, AbbrevHome(ag.Path))
	}
	path := rt.PersonaModeFile(dir)
	// BEFORE the clobber read, and the order is the rule's correctness.
	// Under the operator's CLI home the clobber check answers the wrong
	// question twice: on the operator's own file it refuses with "move it
	// aside", which is the one remedy that would LET the write happen; and
	// on a file an older posse put there it recognises its own marker and
	// waves the write through. Neither reading is about where the path is.
	if err := refusePersonaModeGlobalWrite(ag, rt, dir, os.Getenv("HOME")); err != nil {
		return "", err
	}
	if err := a.refusePersonaModeClobber(ag, rt, dir, path); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	text := personaModeYAML(c, rt.PersonaModeSlug(ag.Name), ag.Name, string(body))
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	excludeFromGit(dir, c.Dir)
	return path, nil
}

// refusePersonaModeGlobalWrite is the SESSION-TREE rule: posse's mode file
// is this seat's PID channel, so a launch whose session directory would put
// that file inside the CLI's own configuration root refuses instead of
// writing it (ADR 0062 D1, whose Consequences say "no write under the home"
// and whose Alternatives rejected `~/.bob/plugins/posse-<persona>/
// custom_modes.yaml` by name, on these costs exactly).
//
// IT IS REACHED BY ACCIDENT AND NOT BY MISCONFIGURATION, which is why it
// needs a guard rather than a doc line (ranger-base-4mrmc F2). `posse new`
// has no worktree option at all, so `o.Worktree` is false and its session
// directory is `--dir`, else `default_dir`, else — and this is the part that
// needs nobody to have set anything wrong — **`$HOME`**, which is the
// fallback `CfgGet` is handed in herdrback.go. So one interactive `posse new
// --runtime bob -a <persona>` with no `--dir`, on any install that has not
// pointed `default_dir` somewhere else, names the home as the workspace. And
// `$HOME/.bob/plugins/posse/custom_modes.yaml` is not a workspace file at
// all: it is in bob's GLOBAL modes glob, which makes that persona's whole
// PID a mode in EVERY bob session on that box, in every workspace, trusted
// or untrusted, selectable from the picker — `hidden` buys nothing, MEASURED
// FALSE in the same bead — and outliving the seat, because nothing in this
// tree ever removes the file.
//
// REFUSE, and not warn or sweep. Warning is what a channel does when the
// session is merely degraded; here proceeding IS the harm, it lands on the
// operator's own files rather than on this seat, and `posse new` is
// interactive by definition — so the one path that reaches this would be
// the one path a warning does not stop. Sweeping means deleting under the
// operator's CLI home a file posse cannot prove it wrote, which is a larger
// liberty than the write it would be undoing — and ranger-base-4mrmc looked
// before the fact and found nothing had landed yet, so there was never
// anything to sweep.
//
// CONTAINMENT, deliberately wider than the glob, and `underDir`'s
// containment rather than a prefix test. The glob is
// `<home>/<GlobalRoot>/plugins/*/<File>`; this refuses anywhere under
// `<home>/<GlobalRoot>`. The extra ground is writes that are not read as
// modes at all, and posse has no business under that root either way — a
// predicate that tracked the glob exactly would have to be re-derived every
// time the CLI widens what it reads, and would read green in the window
// before anybody noticed. seatbelt.go's `underDir` is the comparison
// because it is the one in this package that resolves symlinks over the
// deepest existing ancestor: the file does not exist yet (that is the
// point), and on darwin a textual prefix test reads a `/tmp` or `/var` home
// as outside itself.
func refusePersonaModeGlobalWrite(ag *AgentFile, rt *Runtime, dir, home string) error {
	c := rt.PersonaMode
	if c == nil || c.GlobalRoot == "" || home == "" {
		// No channel, no declared root, or no home to own one: nothing to
		// be inside of. The empty home is AbbrevHome's and ExpandTilde's
		// shape for the same question — a scrubbed environment is not a
		// second place to go looking for a home.
		return nil
	}
	file := rt.PersonaModeFile(dir)
	root := filepath.Join(home, filepath.FromSlash(c.GlobalRoot))
	if file == "" || !underDir(root, file) {
		return nil
	}
	return Die("%s: this session's directory is %s, so %s's mode file would be written to %s — which is under %s, the directory %s reads its GLOBAL modes from, not a workspace at all. That PID would be a mode in every %s session on this box, in every workspace, and would outlive this seat. ADR 0062 D1 puts this channel in the SESSION tree and rejected a write under the CLI home by name. Give the session a directory of its own (`--dir <path>`), or point `default_dir` at one",
		ag.Name, AbbrevHome(dir), ag.Name, AbbrevHome(file), AbbrevHome(root), rt.Name, rt.Name)
}

// refusePersonaModeClobber is the never-clobber rule. Two ways a path is not
// ours, and the message names the path either way, because the operator's
// next move is the same one: move it aside.
func (a *App) refusePersonaModeClobber(ag *AgentFile, rt *Runtime, dir, path string) error {
	rel := rt.PersonaMode.Dir + "/" + rt.PersonaMode.File
	if gitTracks(dir, rel) {
		return Die("%s: %s is TRACKED by the repo in %s — posse will not overwrite a file the repo ships (%s's PID would be rendered there as a custom mode). Move it aside, or launch %s elsewhere",
			ag.Name, rel, AbbrevHome(dir), ag.Name, rt.Name)
	}
	// Lstat before the read, because the read FOLLOWS. A symlink at posse's
	// path is somebody's pointer at a file somewhere else — the operator's
	// own ~/.bob modes file is the obvious one — and writing through it
	// would rewrite that file while every reading here said posse was
	// rewriting its own. A directory, a fifo and anything else that is not
	// a regular file are refused for the same reason: this writer only ever
	// replaces a plain file it wrote.
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return Die("%s: %s exists and is not a regular file (%s) — posse replaces only its own file here, never something else's pointer at one (move it aside)",
			ag.Name, AbbrevHome(path), fileTypeName(fi.Mode()))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil // not there, or unreadable — the write below says so
	}
	if !strings.HasPrefix(string(b), personaModeMarker) {
		return Die("%s: %s exists and was not written by posse — not overwriting (move it aside). It is %s's persona-mode channel: posse writes the PID there as a custom mode and the launch line selects it",
			ag.Name, AbbrevHome(path), rt.Name)
	}
	return nil
}

// gitTracks reports whether the repo containing dir has an index entry for
// the slash-separated path rel, resolved against dir. Best effort, exactly
// like excludeFromGit: a directory that is not a repo tracks nothing, and a
// git that cannot answer is not evidence that the path is the repo's.
//
// `--error-unmatch` so the question is asked of the INDEX rather than of the
// output's emptiness — `ls-files` prints nothing both for an untracked path
// and for a pathspec git declined to walk.
func gitTracks(dir, rel string) bool {
	return exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", "--", rel).Run() == nil
}

// personaModeYAML renders the one-mode file.
//
// Hand-rolled because this tree has no YAML library and yamlflat.go is the
// precedent. Three things are easy to get wrong and are therefore each
// written down:
//
//  1. `name` is EMITTED QUOTED. A persona name is `[A-Za-z0-9_-]` today and
//     would be safe bare; quoting costs nothing and does not depend on a
//     rule enforced in another file.
//  2. `roleDefinition` is a BLOCK SCALAR, so the PID arrives verbatim with
//     no escaping at all. Its content is indented six spaces, and a `|`
//     takes its indentation from the first non-empty line — so a PID whose
//     first content line is itself indented would silently take a deeper
//     indent for the whole block. That case, and only that case, gets the
//     explicit indentation indicator `|2` (two MORE than the four of the
//     key, per YAML 1.2 §8.1.1.1). The plain `|` is what every real PID
//     renders through, and it is the spelling the probe measured.
//  3. A `|` clips to exactly one trailing newline, so a PID ending in blank
//     lines renders the same as one that does not. That is a difference
//     from "verbatim" worth knowing and not worth fighting: trailing blank
//     lines are not persona.
func personaModeYAML(c *PersonaModeChannel, slug, name, body string) string {
	const indent = "      " // six: the block content under a key at four
	var b strings.Builder
	// The marker is the FIRST line and carries nothing that varies, because
	// the never-clobber rule recognises posse's own file by that prefix: a
	// version in it would make every file an older posse wrote unreadable
	// as ours. The renderer stamp goes on the line below, where ranger-base-vso72
	// wants it — a seat reading a persona that is not the one it expects can
	// then tell "relaunch pending" from "install pending" (ranger-base-mmhhc).
	b.WriteString(personaModeMarker + "\n")
	b.WriteString("# " + RenderedByPosse() + ", fresh at every launch; read by nothing but this\n")
	b.WriteString("# session's CLI and excluded in .git/info/exclude. Not the repo's file; do not edit.\n")
	b.WriteString("customModes:\n")
	fmt.Fprintf(&b, "  - slug: %s\n", slug)
	fmt.Fprintf(&b, "    name: %s\n", yamlQuote(name))
	b.WriteString("    roleDefinition: " + personaModeScalarHeader(body) + "\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteString("\n") // a truly empty line, never indented whitespace
			continue
		}
		b.WriteString(indent + line + "\n")
	}
	fmt.Fprintf(&b, "    groups: [%s]\n", strings.Join(c.Groups, ", "))
	// hidden: MEASURED FALSE 2026-10-03 (ranger-base-4mrmc F1, bob 2.0.5) —
	// it hides NOTHING. bob parses the YAML `hidden` and carries it onto
	// the mode object, and nothing reads it: both consumers take
	// `runtime.getModes({workspace})`, which filters only tool groups and
	// duplicate ids, and the picker maps every row. (`hiddenFromUser` is a
	// DIFFERENT field, filtered in `isModeEnabled`, and belongs to
	// provider/builtin modes — which a workspace modes file never becomes.)
	// So Shift+Tab cycles Agent → Plan → Ask → `<persona>`, `/mode` lists
	// it 4/4, and `Tab to view mode` prints the whole PID, frontmatter and
	// `deny:` list included. 5jjtn's `.allowUnknownOption()` class a third
	// time, in the schema this time.
	//
	// KEPT ANYWAY, and the comment is the reason it can be: it is the one
	// honest declaration of what posse means by this mode, it costs nothing
	// on a CLI that ignores it, and the day bob implements the field posse
	// gets the behaviour without a second landing. What must NOT happen is
	// a reader taking it for a confidentiality boundary — there is no
	// "loaded but hidden" state to reach for, the only switch that takes a
	// workspace mode out of the list is workspace trust, and that same
	// switch stops the mode being SELECTED, which is the channel. Whether
	// ADR 0062 D1's choice of the session tree survives losing this is the
	// architect's: ranger-base-er6mt, from ranger-base-se81d.
	b.WriteString("    hidden: true\n")
	return b.String()
}

// personaModeScalarHeader is `|` or `|2`, per rule 2 above.
func personaModeScalarHeader(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			return "|2"
		}
		return "|"
	}
	return "|"
}

// yamlQuote double-quotes a scalar, escaping the two characters that can
// end it early. Enough for a persona name; deliberately not a general YAML
// emitter, because nothing here emits a general scalar.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// ── D2: the mode taking is read off the pane, before the first keystroke ──

// PersonaModeReading is what one look at a pane says about the mode.
type PersonaModeReading int

const (
	// PersonaModeUnread: neither footer is on screen. Not evidence of a
	// fallback — the footer may not be drawn yet — and not evidence of the
	// mode either.
	PersonaModeUnread PersonaModeReading = iota
	// PersonaModeTaken: the footer names this persona's mode.
	PersonaModeTaken
	// PersonaModeFellBack: the CLI is in its built-in mode. The slug it was
	// handed was not loaded, so the session carries every native rulebook
	// and no persona at all.
	PersonaModeFellBack
)

// ReadPersonaMode classifies one pane snapshot for this persona.
//
// The footer is asked FIRST, and that order is the whole correctness of the
// reading. bob prints its fallback sentence once, at startup, and the
// footer continuously — so on a pane that fell back and then scrolled, the
// sentence is gone and the footer is the evidence; and a pane whose mode
// TOOK can still be showing the sentence from an earlier run of the CLI in
// the same shell. The footer is what the mode is; the sentence is only how
// it is quoted.
//
// AMBIGUITY IS NOT TAKEN. A persona whose mode name renders the same footer
// as the fallback — a persona literally named after the CLI's built-in mode
// — would make the two readings indistinguishable, so that pane reads
// Unread and the gate says so rather than guessing in the direction that
// types.
func ReadPersonaMode(c *PersonaModeChannel, name, screen string) PersonaModeReading {
	if c == nil || screen == "" {
		return PersonaModeUnread
	}
	want := fmt.Sprintf(c.Footer, name)
	if want == c.Fallback {
		return PersonaModeUnread
	}
	if strings.Contains(screen, want) {
		return PersonaModeTaken
	}
	if strings.Contains(screen, c.Fallback) || strings.Contains(screen, c.FallbackSentence) {
		return PersonaModeFellBack
	}
	return PersonaModeUnread
}

// personaModeQuote returns the CLI's own fallback line off the screen, when
// it is still there, so the refusal quotes the machine rather than
// paraphrasing it (the ADR 0062 D2 wording). "" when it has scrolled away.
func personaModeQuote(c *PersonaModeChannel, screen string) string {
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, c.FallbackSentence) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// PersonaModeFellBackRefusal is the sentence a launch refuses with. It names
// the slug that was not loaded, quotes the CLI when the CLI is still saying
// it, and spells the harm in PIDVoided's words — because it is PIDVoided's
// harm, reached one layer further out: not a flag that voids the PID, but a
// mode file the CLI never read.
func PersonaModeFellBackRefusal(rt *Runtime, persona, session, quoted string) error {
	line := ""
	if quoted != "" {
		line = fmt.Sprintf("\n  %s says: %s", rt.Name, quoted)
	}
	return Die("%s opened in %s's built-in mode, not %s — so %s's PID never reached it and the session carries every native rulebook and no persona at all. No work prompt was typed.%s\n"+
		"  An unknown mode slug is a FALLBACK and not a refusal on this CLI: it prints one line and runs anyway (MEASURED 2026-10-03, ADR 0062 D2), which is why this is read off the pane. Check %s in the session dir, then relaunch",
		session, rt.Name, rt.PersonaModeText(persona), persona, line, rt.PersonaMode.Dir+"/"+rt.PersonaMode.File)
}

// PaneReader is the one thing AwaitPersonaMode needs from herdr, named so
// the gate can be exercised without one.
type PaneReader interface {
	PaneRead(paneID string, lines int) (string, error)
}

// personaModeReadLines bounds the snapshot. The footer is the pane's last
// line and the fallback sentence is near the top of a fresh session; forty
// is what the probe read and what `agent explain` previews.
const personaModeReadLines = 40

// AwaitPersonaMode is D2: before a work prompt is typed into a pane whose
// PID arrived as a mode, read the pane's own footer and refuse to type if
// the mode fell back.
//
// THE GATE DOES EXACTLY ONE THING. It never types, never presses a key, and
// never answers a screen — it reads, up to the launch's own patience, and
// returns an error only for the one reading that is positive evidence of
// harm.
//
// WHAT AN UNREAD PANE DOES, and why it is not a refusal. The reading can
// fail three ways that are not a fallback: herdr cannot read the pane, the
// CLI draws no footer yet, or the CLI changed its footer in a release
// nobody has measured. None of those is evidence that the mode was NOT
// loaded, and a launch that refused on a diagnostic it could not take would
// be the shape awaitSettled and AwaitPromptable both already rejected
// (promptready.go: "a prompt refused because a diagnostic call failed is
// worse than the race it guards"). So it proceeds, OUT LOUD, naming what it
// could not read. The asymmetry is deliberate: the measured hazard is a
// silent fallback, and silence is what both arms refuse to be.
//
// Returns the note for the caller to print ("" when there is nothing to
// say) and an error only on PersonaModeFellBack.
func AwaitPersonaMode(h PaneReader, rt *Runtime, persona, session, pane string, wait time.Duration, poll time.Duration) (string, error) {
	c := rt.PersonaMode
	if c == nil {
		return "", nil
	}
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	deadline := time.Now().Add(wait)
	var lastErr error
	var screen string
	for {
		text, err := h.PaneRead(pane, personaModeReadLines)
		if err != nil {
			lastErr = err
		} else {
			screen = text
			switch ReadPersonaMode(c, persona, text) {
			case PersonaModeTaken:
				return "", nil
			case PersonaModeFellBack:
				return "", PersonaModeFellBackRefusal(rt, persona, session, personaModeQuote(c, text))
			}
		}
		if !time.Now().Add(poll).Before(deadline) {
			break
		}
		time.Sleep(poll)
	}
	why := fmt.Sprintf("the footer never named %q", fmt.Sprintf(c.Footer, persona))
	if lastErr != nil && screen == "" {
		why = fmt.Sprintf("the pane could not be read at all (%v)", lastErr)
	}
	return fmt.Sprintf("could not read whether %s took %s in %s within %s — %s. Prompting anyway: an unread footer is no evidence either way, and a launch refused on a diagnostic it could not take is worse than the race it guards (ADR 0062 D2)",
		rt.Name, rt.PersonaModeText(persona), session, wait, why), nil
}
