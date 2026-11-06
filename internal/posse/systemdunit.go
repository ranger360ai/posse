package posse

// The watch loop as a systemd user unit (ADR 0080 D5), and the config key
// that says which of the two mechanisms owns it (ADR 0080 amendment D9).
//
// WHY A UNIT AT ALL. `posse dispatch --watch` is the fleet's one always-on
// process: it arms the backups and the coordinator pulse, and nothing is
// delivered without it. Its only autostart today is herdr's `[[startup]]`
// hook (plugin/autostart.sh), so a host whose multiplexer is Rex has no
// autostart at all — ADR 0080 D5 makes it a systemd *user* unit there, one
// per RHQ_HOME, ordered After=/Requires= the multiplexer server's own unit
// so it cannot start in a world with no socket. That ordering is the herdr
// hook's existing property, kept, and it is the whole reason the unit names
// another unit rather than just running.
//
// WHY posse RENDERS IT AND NEVER ENABLES IT. The argv has to track the
// binary and the home's own `autostart_*` keys, which is exactly what the
// hook already does in shell — so the render is posse's. Enabling a unit is
// a change to a deployed system, and this verb prints the `systemctl --user`
// line instead of running it (ADR 0080 D5: "posse never runs systemctl
// enable itself").
//
// WHY THE OWNER IS A KEY AND NOT A GUESS (D9). The two mechanisms cannot
// both arm: a `dispatch --watch` that cannot take state/dispatch-watch.lock
// exits NON-ZERO (watch.go), and herdr's server is up before the unit that
// is `After=` it — so a linked hook that found an armed home would win the
// lock and this unit's own loop would fail on every start. `autostart_via:`
// states the ownership instead of either side detecting the other, and the
// three readers below and in plugin/autostart.sh and govern.go (G7) read it
// the same way over one input table (systemdunit_test.go).

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ─── autostart_via: who owns this home's watch loop ─────────────────────────

const (
	// AutostartViaKey is the instance-config key that names the owner.
	AutostartViaKey = "autostart_via"
	// AutostartViaHerdr is herdr's [[startup]] hook: the default, and what
	// every home on a herdr-only host keeps.
	AutostartViaHerdr = "herdr"
	// AutostartViaSystemd is the user unit this file renders.
	AutostartViaSystemd = "systemd"
)

// AutostartUnitKey names the multiplexer server's own systemd user unit —
// the one this home's loop is ordered behind.
//
// IT HAS NO DEFAULT ON PURPOSE (ADR 0080 D5: "name from config, not
// hardcoded"). posse has never measured what Rex's or herdr's Linux unit is
// called, and the two mechanisms a guess could pick between fail in opposite
// directions: a name no unit wears makes `Requires=` refuse the start
// outright, and a name some OTHER unit wears orders the loop behind the
// wrong process. Absent, the render refuses and says where to read the real
// name.
const AutostartUnitKey = "autostart_multiplexer_unit"

// ValidAutostartVia is the closed set: the two mechanisms that exist. A
// third name is a refusal and never a fallback — defaulting a name posse
// cannot read back to herdr would arm the hook on a host whose operator
// typed something else, which is the one direction D9 exists to close.
func ValidAutostartVia(v string) bool {
	return v == AutostartViaHerdr || v == AutostartViaSystemd
}

// AutostartVia is the validated owner of this home's watch loop.
//
// ABSENT AND EMPTY BOTH MEAN herdr, and they mean it for the reason every
// other defaulted key in this family reads them that way: yamlClean and the
// hook's cfg() both map a bare key, `null` and `~` to the empty string, so
// "" is the one reading the two readers cannot disagree about. This is NOT
// the `autostart_interval:` rule, where presence is the arm switch and a
// bare key is a broken arm — that key has no default to fall back on and
// this one does.
//
// A MALFORMED VALUE IS REFUSED, not defaulted, which is the one place this
// key departs from `autostart_max_interval:`. That key has a cap to fall
// back on and a typo in it costs one backoff; this key decides WHO ARMS, and
// a typo silently read as herdr would arm the hook on a systemd host and
// take the lock the unit needs.
func (a *App) AutostartVia() (string, error) {
	v := strings.TrimSpace(a.CfgGet(AutostartViaKey, ""))
	if v == "" {
		return AutostartViaHerdr, nil
	}
	if !ValidAutostartVia(v) {
		return "", AutostartViaRefusal(v, a.ConfigPath)
	}
	return v, nil
}

// AutostartViaRefusal is the sentence a value that names neither mechanism
// gets. One renderer, two callers, because each names the config path its
// own surface names: AutostartVia hands it the absolute path a refusal at
// the keyboard should quote, and `posse status`'s G7 hands it AbbrevHome,
// which is what every other row of that table prints. A second sentence
// would be the drift this file's header is about.
func AutostartViaRefusal(value, cfg string) error {
	return Die("bad %s '%s' in %s (want %s or %s — ADR 0080 D9) — the key says which mechanism owns this home's `dispatch --watch` loop, and a name that is neither cannot be read as either: the herdr startup hook refuses it and arms nothing, and `posse systemd` refuses to render a unit",
		AutostartViaKey, value, cfg, AutostartViaHerdr, AutostartViaSystemd)
}

// ─── the loop's argv, which the hook also builds ────────────────────────────

// autostartMaxBeadsFallback is the `-n` the hook falls back to, as a string
// because that is what reaches the argv either way. The number itself is
// examples/config.yaml's documented default and plugin/autostart.sh's
// `case` literal; nothing here is its source of truth, and
// TestSystemdUnitExecStartIsTheHooksArgv is what keeps the two in step.
const autostartMaxBeadsFallback = "3"

// AutostartWatchArgs is the `posse dispatch --watch …` argv this home's
// `autostart_*` keys describe — the SAME argv plugin/autostart.sh builds in
// shell, in the same order, with the same defaults and the same
// named-and-dropped arms.
//
// TWO BUILDERS OF ONE ARGV is a thing this shop normally refuses (one
// renderer, or a check grades a row nobody launches). It is unavoidable
// here: the hook is read fresh from the checkout by herdr at server start
// and cannot call into Go, and the unit render cannot call into bash and
// have a deterministic file. So the duplication is held by a test instead of
// by a function — one input table, both builders, compared argv to argv
// (systemdunit_test.go).
//
// The notes are the hook's stderr lines: a value posse cannot read on a key
// that HAS a default is named and dropped, never silently obeyed and never
// fatal, because standing a whole fleet down over an unreadable backoff cap
// is the more expensive of the two failures (ranger-base-x8y8).
//
// A BROKEN ARM IS FATAL HERE, which is the render's half of the bead's
// "a broken arm refuses at render": absent, bare, or unparseable
// `autostart_interval:` all return an error and no argv. `dispatch --watch`
// has no default interval and dies on an empty argument, so the only choice
// is where that failure lands — in the deployer's terminal, or in a unit
// that restarts five times and goes to `failed`.
func (a *App) AutostartWatchArgs() (args []string, notes []string, err error) {
	cfg := a.ConfigPath
	interval := a.CfgGet("autostart_interval", "")
	switch {
	case interval == "" && !yamlHasKey(cfg, "autostart_interval"):
		return nil, nil, Die("%s names no autostart_interval: — the key's PRESENCE is the arm switch, and there is no loop to render a unit for until it names an interval (30s, 5m, or bare seconds)", cfg)
	case interval == "":
		return nil, nil, Die("autostart_interval: in %s is present but empty — give it an interval (30s, 5m, or bare seconds), or comment the key out to disarm", cfg)
	}
	if _, bad := ParseInterval(interval); bad != nil {
		return nil, nil, Die("autostart_interval: '%s' in %s is not an interval — use 30s, 5m, or bare seconds; nothing rendered", interval, cfg)
	}
	args = []string{"dispatch", "--watch", interval}

	if v := a.CfgGet("autostart_max_interval", ""); v != "" {
		if _, bad := ParseInterval(v); bad != nil {
			notes = append(notes, fmt.Sprintf("autostart_max_interval: '%s' in %s is not an interval — use 30s, 5m, or bare seconds; using posse's default cap (8x %s)", v, cfg, interval))
		} else {
			args = append(args, "--max-interval", v)
		}
	}

	beads := a.CfgGet("autostart_max_beads", "")
	switch {
	case beads == "":
		beads = autostartMaxBeadsFallback
	case !autostartIsCount(beads):
		// Never silently obeyed: a malformed count would reach `-n` as
		// Atoi's 0, which is unbounded — the one thing this default exists
		// to prevent (rangerhq-v83).
		notes = append(notes, fmt.Sprintf("autostart_max_beads: '%s' is not a count — using %s", beads, autostartMaxBeadsFallback))
		beads = autostartMaxBeadsFallback
	}
	args = append(args, "-n", beads)

	switch resume := a.CfgGet("autostart_resume", ""); resume {
	case "", "true", "yes", "1":
		args = append(args, "--resume")
	case "false", "no", "0":
	default:
		notes = append(notes, fmt.Sprintf("autostart_resume: '%s' is not true/false — using true", resume))
		args = append(args, "--resume")
	}

	switch a.CfgGet("autostart_dry_run", "") {
	case "true", "yes", "1":
		args = append(args, "--dry-run")
	}
	return args, notes, nil
}

// autostartIsCount is the hook's `*[!0-9]*` arm: a count is digits and
// nothing else. An empty string is not a count, and the caller above never
// asks. (Not `allDigits`: hookdeps_qa_test.go already declares that name,
// and arm 2 compiles it beside this file.)
func autostartIsCount(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ─── the unit ───────────────────────────────────────────────────────────────

// systemdUnitSuffixes is systemd's own closed set of unit types
// (systemd.unit(5)). `autostart_multiplexer_unit:` is checked against it so
// a value that is not a unit name at all is refused at render rather than
// written into an `After=` that systemd reads as a unit it will never find.
var systemdUnitSuffixes = []string{
	".service", ".socket", ".target", ".device", ".mount", ".automount",
	".swap", ".path", ".timer", ".slice", ".scope",
}

// WatchUnitDir is where a systemd user unit of this account lives:
// $XDG_CONFIG_HOME/systemd/user, else ~/.config/systemd/user. Both are read
// from the environment rather than from the App, because a user unit is
// keyed per USER and not per home (ADR 0080 D12) — several homes write
// several units into this one directory, which is why the name below has to
// carry the home's `instance:` tag.
func WatchUnitDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "systemd", "user")
}

// WatchUnit is one rendered unit: what to call it, where it goes, what is in
// it, what the operator still has to type, and whatever the render had to
// name and drop on the way.
type WatchUnit struct {
	Name   string   // posse-dispatch-<instance>.service
	Path   string   // WatchUnitDir()/Name
	Body   string   // the file, rendered
	Enable []string // the systemctl --user lines posse prints and never runs
	Notes  []string // the named-and-dropped lines, for stderr
}

// WatchUnitName is what this home's unit is called: posse-dispatch.service,
// carrying the home's `instance:` tag when it has one.
//
// THE TAG IS WHAT KEEPS TWO HOMES APART. A systemd user unit is keyed per
// USER (ADR 0080 D12), so every home on the account writes into one
// directory, and ADR 0080 D8 sets `instance:` on every home of the host for
// exactly this reason. An untagged home still gets a name — a single-home
// host is a legitimate reader of this verb — and the render says so on
// stderr.
//
// It answers over a config it cannot refuse, so a malformed tag reads as
// untagged here: G7 names this unit in a row about a loop that is not
// running, and that row must not fail to print because another key is wrong
// (the Multiplexer/MultiplexerName split, same reason). RenderWatchUnit asks
// a.Instance() itself and refuses there, which is where it costs something.
func WatchUnitName(a *App) string {
	tag, err := a.Instance()
	if err != nil || tag == "" {
		return "posse-dispatch.service"
	}
	return "posse-dispatch-" + tag + ".service"
}

// AutostartRefuser is the clause `posse status`'s G7 row uses for "and
// nothing will arm this home whatever happens next": the mechanism that owns
// the loop is the mechanism that refuses a broken arm, and the two refuse in
// different words at different moments. One sentence per owner, rendered
// here beside both readers, so the row cannot describe the herdr hook on a
// host where `posse systemd` is what the operator will run.
func AutostartRefuser(via string) string {
	if via == AutostartViaSystemd {
		return "`posse systemd` refuses it and renders nothing"
	}
	return "the herdr startup hook refuses it and arms nothing"
}

// RenderWatchUnit builds this home's unit. exe is the posse the unit should
// run — its own absolute path at the call site, so the argv tracks the
// binary (ADR 0080 D5) — and nothing here stats it: a render is a render,
// and a unit written before the binary is promoted is still the right unit.
//
// Every refusal below is a refusal at RENDER, which is the bead's own done
// condition. Reading them in order: the owner (D9), then the ordering D5
// makes load-bearing, then the arm grammar, then the values that would end
// up inside the file.
func RenderWatchUnit(a *App, exe string) (*WatchUnit, error) {
	via, err := a.AutostartVia()
	if err != nil {
		return nil, err
	}
	if via != AutostartViaSystemd {
		return nil, Die("%s does not say `%s: %s`, so this home's watch loop is not systemd's to own (ADR 0080 D9) — the herdr startup hook arms it, and a unit rendered beside that hook would lose the lock to it and fail on every start (plugin/autostart.sh, watch.go). Set `%s: %s` in %s on a host where systemd owns the loop",
			a.ConfigPath, AutostartViaKey, AutostartViaSystemd, AutostartViaKey, AutostartViaSystemd, a.ConfigPath)
	}

	mux := strings.TrimSpace(a.CfgGet(AutostartUnitKey, ""))
	if mux == "" {
		return nil, Die("%s names no %s: — ADR 0080 D5 orders this loop After=/Requires= the multiplexer server's own user unit so it cannot start in a world with no socket, and posse will not guess that unit's name (a name no unit wears refuses the start; a name another unit wears orders the loop behind the wrong process). Read it off the host — `systemctl --user list-units --type=service` — and set `%s: <unit>` in %s",
			a.ConfigPath, AutostartUnitKey, AutostartUnitKey, a.ConfigPath)
	}
	if err := validUnitName(mux); err != nil {
		return nil, Die("bad %s '%s' in %s: %v", AutostartUnitKey, mux, a.ConfigPath, err)
	}

	args, notes, err := a.AutostartWatchArgs()
	if err != nil {
		return nil, err
	}

	tag, err := a.Instance()
	if err != nil {
		return nil, err
	}
	name := WatchUnitName(a)
	if tag == "" {
		// Not a refusal: `instance:` is procedure (ADR 0080 D8), and a
		// single-home host is a legitimate reader of this verb. It is a
		// note because the NAME is what keeps two homes' units apart, and
		// the second untagged home would otherwise render the first's name.
		notes = append(notes, fmt.Sprintf("%s names no instance: — this unit is called %s, which is not specific to this home. A second home on this account would render the same name (ADR 0080 D8 sets instance: on every home for exactly this reason)", a.ConfigPath, name))
	}

	// autostart_dir: is EXPANDED HERE and is not by the hook, which is a
	// divergence and the right one: the hook hands the value to `posse new
	// --dir`, where ExpandTilde runs, and systemd expands nothing at all —
	// a literal `~` in WorkingDirectory= is a relative path and the unit
	// refuses to start. The seed documents `autostart_dir: ~`, so this is
	// the common value and not a corner.
	dir := ExpandTilde(a.CfgGet("autostart_dir", ""))
	if dir == "" {
		dir = os.Getenv("HOME")
	}
	// WorkingDirectory= takes an absolute path and systemd refuses the unit
	// without one. Nothing above can produce one from an empty HOME, and a
	// unit written with an empty or relative value fails at start with a
	// message about the directory rather than about the config — so it is
	// refused here, where the key that would fix it can be named.
	if !filepath.IsAbs(dir) {
		return nil, Die("the loop has no absolute working directory: autostart_dir: in %s is %q and HOME resolves to %q — systemd's WorkingDirectory= takes an absolute path and refuses the unit without one, so name the directory in autostart_dir:",
			a.ConfigPath, a.CfgGet("autostart_dir", ""), os.Getenv("HOME"))
	}

	who := tag
	if who == "" {
		who = AbbrevHome(a.Home)
	}

	execStart, err := unitExec(append([]string{exe}, args...))
	if err != nil {
		return nil, err
	}
	home, err := unitArg(a.Home)
	if err != nil {
		return nil, Die("RHQ_HOME %s cannot go in a unit file: %v", a.Home, err)
	}
	workdir, err := unitArg(dir)
	if err != nil {
		return nil, Die("autostart_dir: %s cannot go in a unit file: %v", dir, err)
	}
	desc, err := unitLiteral(who)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, `# posse dispatch --watch, for the crew home at %s
#
# RENDERED BY `+"`posse systemd`"+` — do not hand-edit: re-render it instead, so
# the argv keeps tracking the binary and this home's autostart_* keys
# (ADR 0080 D5). posse wrote this file and will NOT enable it; run the
# systemctl lines the command printed.
#
# WHY A UNIT AND NOT HERDR'S STARTUP HOOK: this home says
# `+"`%s: %s`"+`, which stands plugin/autostart.sh down by name
# (ADR 0080 D9). Both mechanisms must never arm, because a dispatch --watch
# that cannot take the home's state/dispatch-watch.lock exits non-zero, and
# herdr is up before the unit that is After= it.
#
# THE LOOP WRITES ITS OWN LOG at <RHQ_HOME>/state/dispatch-watch.log and
# rotates it (ranger-base-n00wn), so nothing here tees into it; journalctl's
# copy is a duplicate, not the record.
#
# A USER SERVICE DOES NOT INHERIT YOUR SHELL'S PATH — systemd gives it a
# minimal one — and posse shells out to bd and git. Check
# `+"`systemctl --user show-environment`"+` and add an Environment=PATH= drop-in if
# either lives somewhere systemd cannot see.
[Unit]
Description=posse dispatch --watch (%s)
After=%s
Requires=%s

[Service]
Type=simple
WorkingDirectory=%s
Environment=RHQ_HOME=%s
ExecStart=%s
# on-failure, per ADR 0080 D5. A loop that loses the lock exits non-zero, so
# if something else is already arming this home the unit hits systemd's start
# limit and goes to `+"`failed`"+` — which is the visible failure D9 wants, and not
# a restart storm. RestartSec keeps the retries off the box in the meantime.
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=default.target
`,
		AbbrevHome(a.Home),
		AutostartViaKey, AutostartViaSystemd,
		desc, mux, mux, workdir, home, execStart)

	return &WatchUnit{
		Name: name,
		Path: filepath.Join(WatchUnitDir(), name),
		Body: b.String(),
		Enable: []string{
			"systemctl --user daemon-reload",
			"systemctl --user enable --now " + name,
			"systemctl --user status " + name,
		},
		Notes: notes,
	}, nil
}

// validUnitName is the `autostart_multiplexer_unit:` grammar: a systemd unit
// name, one word, ending in one of systemd's own unit-type suffixes.
//
// It is deliberately NOT systemd's full escaping grammar (templates,
// instances, escaped paths). What it has to catch is the three shapes that
// would end up inside the file and break it or point it somewhere else: a
// value with whitespace in it, which systemd reads as two directives' worth
// of tokens; a value carrying a `%`, which systemd expands where this value
// lands; and a value that is not a unit name at all.
//
// THIS VALUE IS INTERPOLATED RAW, which is why the `%` is refused here and
// not doubled downstream. After=/Requires= take unit names, and a unit name
// as systemctl prints it carries no `%` at all (systemd.unit(5): the
// characters a unit name may hold are alphanumerics, `:`, `-`, `_`, `.`,
// `\` and `@`) — so a `%` is not a name needing an escape, it is not a name.
// Doubling it would render `rex%%i.service` — which is a unit nobody has
// either, and the reason this refuses rather than escapes.
func validUnitName(v string) error {
	if err := unitPrintable(v); err != nil {
		return err
	}
	if strings.ContainsAny(v, " \t") {
		return Die("a unit name is one word with no whitespace")
	}
	// systemd expands specifiers in After=/Requires= — its own shipped
	// templates rely on it (`serial-getty@.service` is After=dev-%i.device)
	// — and the unit this renders is not a template, so `%i` would expand to
	// the empty instance and `%n` to this unit's own name. That is verbatim
	// the second failure AutostartUnitKey's no-default decision exists to
	// prevent: a name another unit wears orders the loop behind the wrong
	// process (ranger-base-hmet1, from ranger-base-zml8t).
	if strings.Contains(v, "%") {
		return Die("a unit name carries no `%%`: systemd reads it as a specifier and expands it in After=/Requires= (`%%i` to the empty instance, `%%n` to this unit's own name), so the loop would be ordered behind a unit nobody named — name the server's unit as systemctl prints it")
	}
	for _, suffix := range systemdUnitSuffixes {
		if strings.HasSuffix(v, suffix) && len(v) > len(suffix) {
			return nil
		}
	}
	return Die("it ends in no systemd unit type (%s) — name the server's unit as systemctl prints it, suffix included", strings.Join(systemdUnitSuffixes, ", "))
}

// unitPrintable refuses a control character — a newline above all. Every value
// below is interpolated into a unit file, where a newline is not a value at
// all: it ends the directive and starts a new one, so a path carrying one
// would inject directives nobody wrote. Flat-YAML cannot produce one (the
// reader splits on it), and a path from the environment is not supposed to
// either — which is why this is a refusal and not an escape.
func unitPrintable(v string) error {
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return Die("it carries a control character (%q)", r)
		}
	}
	return nil
}

// unitArg renders one value for a directive that takes a single argument —
// WorkingDirectory=, Environment=. `%` is systemd's specifier escape, so a
// literal one is doubled; whitespace means the value has to be quoted or
// systemd reads it as more than one argument.
func unitArg(v string) (string, error) {
	if err := unitPrintable(v); err != nil {
		return "", err
	}
	return unitQuote(strings.ReplaceAll(v, "%", "%%")), nil
}

// unitQuote is the whitespace rule, shared by unitArg and unitCmdArg:
// whitespace (or a quote, or a backslash) means systemd would otherwise read
// the value as more than one argument, or re-read the escape.
func unitQuote(v string) string {
	if strings.ContainsAny(v, " \t\"'\\") {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
	}
	return v
}

// unitCmdArg is unitArg for one argument of a COMMAND LINE (ExecStart=),
// which is the one place systemd also re-reads `$`: systemd.service(5)
// ("Command lines") gives command lines basic environment-variable
// substitution — `$FOO` and `${FOO}` — and says a literal dollar sign is
// written `$$`. Quoting does not stand it down: the substitution happens
// after the line is split, so a `$` inside the quotes the whitespace rule
// added is still a variable.
//
// DOUBLED HERE AND NOT IN unitArg, because the two are not the same
// directive. WorkingDirectory= and Environment= are not command lines and
// substitute nothing, so a `$$` written into either would reach the process
// as two characters — which is why this is a second function and not a line
// added to the first (ranger-base-hmet1, from ranger-base-zml8t).
//
// WHAT IT PROTECTS, stated narrowly: of everything that reaches an ExecStart
// here only the binary's own path can carry a `$` at all — every config value
// in the argv is ParseInterval-validated or digits-only — so the reachable
// case is a posse installed under a path with a dollar in it, and the unit
// SETS Environment=RHQ_HOME=, so a `${RHQ_HOME}` in that path would expand
// against a variable this very file defines. REPORTED from systemd.service(5)
// and not MEASURED: this shop has no systemd (darwin), and the man page's
// rule for the FIRST token of a command line differs ("the program to
// execute may not be a variable"), so whether systemd substitutes inside
// argv[0] rather than refusing the unit is unread. Doubling is the answer
// that is right either way for a literal dollar, and ranger-base-offid's
// shakedown is where the other half gets measured.
func unitCmdArg(v string) (string, error) {
	if err := unitPrintable(v); err != nil {
		return "", err
	}
	v = strings.ReplaceAll(v, "%", "%%")
	v = strings.ReplaceAll(v, "$", "$$")
	return unitQuote(v), nil
}

// unitLiteral is unitArg for a directive that takes the rest of the line as
// free text (Description=): the specifier still expands, so `%` is still
// doubled, but nothing is quoted or split.
func unitLiteral(v string) (string, error) {
	if err := unitPrintable(v); err != nil {
		return "", err
	}
	return strings.ReplaceAll(v, "%", "%%"), nil
}

// unitExec renders an ExecStart command line: each argument through
// unitCmdArg, joined by single spaces. The binary path comes first and is
// escaped and quoted by the same rule as every other argument, so a posse
// installed under a path with a space or a dollar in it still launches.
func unitExec(argv []string) (string, error) {
	out := make([]string, 0, len(argv))
	for _, arg := range argv {
		q, err := unitCmdArg(arg)
		if err != nil {
			return "", Die("argument %q cannot go in a unit file: %v", arg, err)
		}
		out = append(out, q)
	}
	return strings.Join(out, " "), nil
}

// ─── the verb ───────────────────────────────────────────────────────────────

// CmdSystemdUnit is `posse systemd [--write] [--force]`.
//
// STDOUT IS THE UNIT and stderr is the notes, so `posse systemd > unit` is a
// file and not a file with a warning in it (the `posse runtime filing`
// split, for the same reason). Under --write stdout is what happened
// instead, because there is nothing to pipe.
//
// --write REFUSES TO OVERWRITE A UNIT IT DID NOT JUST RENDER. Two homes on
// one account write into one directory (ADR 0080 D12) and an untagged home
// renders a name that is not its own, so a silent overwrite is how one
// crew's loop ends up pointed at another crew's home. --force is the
// operator saying they looked.
func (a *App) CmdSystemdUnit(write, force bool, out, errw io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return Die("cannot read this posse's own path for ExecStart=: %v — ADR 0080 D5 wants the installed binary's absolute path, and a unit naming a relative one would run whatever systemd's working directory holds", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	u, err := RenderWatchUnit(a, exe)
	if err != nil {
		return err
	}
	for _, note := range u.Notes {
		fmt.Fprintf(errw, "posse systemd: %s\n", note)
	}
	if !write {
		fmt.Fprint(out, u.Body)
		fmt.Fprintf(errw, "\nposse systemd: this is the unit for %s — `posse systemd --write` puts it at %s\n", AbbrevHome(a.Home), AbbrevHome(u.Path))
		for _, line := range u.Enable {
			fmt.Fprintf(errw, "posse systemd: then: %s\n", line)
		}
		return nil
	}
	// WatchUnitDir reads the environment, so a session with neither
	// XDG_CONFIG_HOME nor HOME would otherwise write `systemd/user/<unit>`
	// relative to the cwd — a file systemd will never read, reported as a
	// successful install.
	if !filepath.IsAbs(u.Path) {
		return Die("cannot say where this account's systemd user units live: neither XDG_CONFIG_HOME nor HOME is set, so the unit would land at %q, relative to wherever this ran. Print it instead (`posse systemd`) and put it where `systemctl --user` reads it", u.Path)
	}
	if err := os.MkdirAll(filepath.Dir(u.Path), 0o755); err != nil {
		return err
	}
	if old, err := os.ReadFile(u.Path); err == nil && string(old) != u.Body && !force {
		return Die("%s already exists and says something else — read it before you replace it (several homes on one account write into one directory, ADR 0080 D12), then re-run with --force", u.Path)
	}
	if err := os.WriteFile(u.Path, []byte(u.Body), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote %s\n", u.Path)
	fmt.Fprintf(out, "posse does not enable it — that is yours (ADR 0080 D5):\n")
	for _, line := range u.Enable {
		fmt.Fprintf(out, "  %s\n", line)
	}
	return nil
}
