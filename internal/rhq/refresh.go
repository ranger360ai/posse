package rhq

// `posse refresh` — the one credential WRITE in posse, and the operator's
// own hand is the only thing that performs it (ADR 0019 D4, bead
// ranger-base-h207).
//
// The standing rule was "posse never mints, refreshes, or writes a
// credential". ADR 0019 amends it to "never AUTONOMOUSLY", and this file is
// the whole of the exception. Everything about it is built so that the
// exception cannot widen by accident:
//
//   - It refuses without a TTY and it refuses under the persona env marker.
//     Both, for the whole verb — the report writes nothing, but the gate is
//     on the command and not on its branches, because that is what the deny
//     line an operator writes can express.
//   - The gate is spelled twice, as `posse promote`'s is. The second
//     spelling is `Bash(posse refresh:*)` in every crew PID, which is the
//     INSTANCE's side: this repo ships the mechanism and does not edit the
//     constitution's personas. If you are reading this because you added a
//     crew persona, that deny line is yours to add.
//   - The mint it runs is the RUNTIME's own (`claude setup-token`), whose
//     browser flow is the human gate. posse does not stand in it.
//   - For a `meter` credential it writes NOTHING, ever. The rotating OAuth
//     token has one writer — the runtime's own login loop — and every second
//     copy of it is a snapshot that disagrees with the source exactly when it
//     matters. That copy existed once, in `default.env`, and 401'd the fleet
//     twice (ADR 0019's Context has the dates).
//   - It never writes a metered credential. `ANTHROPIC_API_KEY` is spending,
//     rejected on the money line by rangerhq-kiz and restated here as both a
//     refused NAME and a refused VALUE shape — a metered key pasted into the
//     session variable would be the same sin wearing the right label.
//
// Nothing here prints a credential. The report names stores, variable NAMES,
// env set names and dates; `posse envs` already keeps that rule and this
// command is the reason the rule now has stamps to read.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"
)

// RefreshExpiryWindow is how long before an expiry posse starts saying so.
// Fourteen days is ADR 0019 D5's window and it is shared with the cockpit
// header and the dispatch line (ranger-base-k6ha), so the report and the
// warning cannot disagree about when "soon" starts.
const RefreshExpiryWindow = 14 * 24 * time.Hour

// stampDate is the format both stamps are written and read in. A date and
// not a timestamp: what an operator knows about a mint is the day they made
// it, and a fabricated time of day would be a precision nobody measured.
const stampDate = "2006-01-02"

// mintedStamp and expiresStamp are the comment lines refresh writes above
// the variable it sets. They are comments so every existing reader — the
// env-set parser, `posse envs`, a shell that sources the file — sees exactly
// what it saw before, and a stamp is not a variable anything can inject.
const (
	mintedStamp  = "# minted="
	expiresStamp = "# expires="
)

// meteredCredentialNames are credentials that are metered spending. A
// persona is never the one who decides to spend (rangerhq-kiz), and neither
// is a command that writes a file without one in the room: refresh will not
// write one under any name and will not write one it recognizes by shape.
var meteredCredentialNames = map[string]bool{"ANTHROPIC_API_KEY": true}

// meteredKeyPrefix is the prefix Anthropic's metered API keys carry, as
// against a setup-token's `sk-ant-oat…`. It is a shape check and not an
// authority: a key that has been renamed slips past it. It is here because
// the failure it catches — a metered key pasted into the session variable —
// is silent, spends money, and looks exactly like success.
const meteredKeyPrefix = "sk-ant-api"

// runtimeMint is the runtime's OWN mint command, per runtime. claude's is
// `claude setup-token`, whose browser flow is the human gate ADR 0019 D4
// leans the entire amendment on. A runtime with no entry has no mint posse
// knows, and refresh says so and points at --paste rather than guessing a
// command line for someone else's CLI.
var runtimeMint = map[string][]string{"claude": {"claude", "setup-token"}}

// RefreshOpts is `posse refresh`'s argument surface, plus the four seams the
// tests replace. The seams are unexported: nothing outside this package can
// route the mint, the paste, the clock or the TTY answer somewhere else, and
// a test in this package does not have to own a terminal to pin what the
// command does with one.
type RefreshOpts struct {
	// Runtime is the launch profile whose credential is being refreshed.
	// Empty = the report over every runtime, which is the no-argument form
	// and the runbook's front door.
	Runtime string
	// Purpose defaults to CredSession — the only kind posse may write.
	// CredMeter is accepted and answered with the store-of-record
	// instruction, because "refresh the meter credential" is a thing an
	// operator will type and being told where that lives beats an
	// unknown-argument error.
	Purpose CredPurpose
	// EnvSet names the env set to write into. Empty resolves to the one set
	// that already holds this variable, and refuses when that is ambiguous.
	EnvSet string
	// Paste skips the mint: the token was minted somewhere else (a
	// browser-capable box) and is being carried to this one.
	Paste bool
	// Expires is the operator's own knowledge of the mint's lifetime,
	// YYYY-MM-DD. Absent means absent: no `# expires=` stamp is written and
	// the report says "cannot tell". posse has no way to ask a setup-token
	// when it dies, and a guessed date is worse than no date (ADR 0019 D5).
	Expires string

	goos  string
	tty   func() bool
	mint  func(w io.Writer, rt *Runtime) error
	ask   func(w io.Writer, prompt string) (string, error)
	clock func() time.Time
}

func (o RefreshOpts) os() string {
	if o.goos != "" {
		return o.goos
	}
	return runtime.GOOS
}

func (o RefreshOpts) now() time.Time {
	if o.clock != nil {
		return o.clock()
	}
	return time.Now()
}

func (o RefreshOpts) isTTY() bool {
	if o.tty != nil {
		return o.tty()
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// CmdRefresh is the command. The gate is the first thing in it and it is
// unconditional: both refusals apply to the report too.
func (a *App) CmdRefresh(w io.Writer, o RefreshOpts) error {
	if p := os.Getenv(EnvPersona); p != "" {
		return Die("posse refresh is the operator's hand and nothing else (ADR 0019 D4): refusing under %s=%s — every crew PID also denies Bash(posse refresh:*), and this is the same rule spelled in the binary", EnvPersona, p)
	}
	if !o.isTTY() {
		return Die("posse refresh is interactive-only (ADR 0019 D4): stdin is not a terminal, so there is no human here to be the gate — run it from your own shell")
	}
	if o.Runtime == "" {
		return a.refreshReport(w, o)
	}
	rt, err := a.LoadRuntime(o.Runtime)
	if err != nil {
		return err
	}
	switch o.Purpose {
	case "", CredSession:
		return a.refreshSession(w, rt, o)
	case CredMeter:
		return refreshMeter(w, rt, o)
	}
	return Die("unknown credential purpose %q (session, meter)", o.Purpose)
}

// ─── the report: every (runtime, purpose), its source, its expiry, its fix ───

// CredRow is one line of the report: one credential this box either has, or
// structurally cannot have, and what to do about it.
type CredRow struct {
	Runtime string
	Purpose CredPurpose
	// Source is where this credential lives, in the words an operator would
	// use to go look at it — or why there is no such place.
	Source string
	// Expiry is rendered, never raw: "cannot tell" is an answer and is
	// never rendered as freshness (ADR 0019 D5).
	Expiry string
	// Action is the operator's next verb, or "" when there is none.
	Action string
}

// CredReport is the no-argument form's data: every runtime, both purposes.
// It is separate from the rendering so a test can assert the ANSWER rather
// than the layout, and so the cockpit header (ranger-base-k6ha) has one
// place to read the same facts from.
//
// It reads the meter store, which on darwin means execing `security`. That
// is the store of record and reading it is the whole point; it is also why
// this is an operator-run command and not something a pass does.
func (a *App) CredReport(o RefreshOpts) []CredRow {
	now, goos := o.now(), o.os()
	var rows []CredRow
	for _, name := range a.ListRuntimes() {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			rows = append(rows, CredRow{Runtime: name, Source: "this runtime does not load", Expiry: "—", Action: err.Error()})
			continue
		}
		rows = append(rows, a.sessionRows(rt, now)...)
		rows = append(rows, meterRow(rt, goos, now))
	}
	return rows
}

// sessionRows reports the session credential from the ENV SETS ON DISK, not
// from this process's environment. The seam's session read looks at the
// environment because that is where a launched session's credential is; the
// operator's shell has none, and the question they are asking is about the
// files (credential.go's readSessionCredential names this file as the place
// that answers the on-disk half).
//
// One row per env set that holds it, because two sets holding the same
// variable is a fact worth seeing rather than a tie to break silently — the
// PID's `envs:` order decides which one a launch gets, and posse's report is
// not the place to guess at that.
func (a *App) sessionRows(rt *Runtime, now time.Time) []CredRow {
	key := CageCredential(rt)
	if key == "" {
		return []CredRow{{
			Runtime: rt.Name, Purpose: CredSession,
			Source: "undecided — this runtime has no session credential name",
			Expiry: "—",
			Action: "codex and grok keep plain auth.json files and rangerhq-kiz left their container shape open; decide it (cage_cred: in runtimes/" + rt.Name + ".yaml for a template-only runtime)",
		}}
	}
	sites := a.envSetsWith(key)
	if len(sites) == 0 {
		return []CredRow{{
			Runtime: rt.Name, Purpose: CredSession,
			Source: fmt.Sprintf("%s is in no env set under %s", key, AbbrevHome(a.EnvsDir)),
			Expiry: "—",
			Action: fmt.Sprintf("mint it: posse refresh %s --env-set <set>", rt.Name),
		}}
	}
	var rows []CredRow
	for _, s := range sites {
		row := CredRow{
			Runtime: rt.Name, Purpose: CredSession,
			Source: fmt.Sprintf("env set %s (%s)", s.Set, key),
			Expiry: renderExpiry(s.Expires, now),
		}
		if s.Minted.IsZero() {
			row.Source += ", minted date unstamped"
		} else {
			row.Source += ", minted " + s.Minted.Format(stampDate)
		}
		switch {
		case s.Expires.IsZero():
			row.Action = fmt.Sprintf("nothing to do — posse cannot tell when this one dies, and says so rather than calling it fresh (stamp it with `posse refresh %s --env-set %s --expires <YYYY-MM-DD>` when you know)", rt.Name, s.Set)
		case !s.Expires.After(now):
			row.Action = fmt.Sprintf("expired — re-mint: posse refresh %s --env-set %s", rt.Name, s.Set)
		case s.Expires.Sub(now) <= RefreshExpiryWindow:
			row.Action = fmt.Sprintf("re-mint within the window: posse refresh %s --env-set %s", rt.Name, s.Set)
		default:
			row.Action = "nothing to do"
		}
		if len(sites) > 1 {
			row.Action += " (more than one env set holds this variable — the PID's envs: order decides which a launch gets)"
		}
		rows = append(rows, row)
	}
	return rows
}

// meterRow reports the runtime-owned credential without ever holding it: the
// token is read and dropped, because what the report is asking is whether
// the read WORKS and when the thing dies.
func meterRow(rt *Runtime, goos string, now time.Time) CredRow {
	row := CredRow{Runtime: rt.Name, Purpose: CredMeter, Expiry: "—"}
	store, ns := meterStore(rt.Name, goos)
	if ns != nil {
		row.Source = fmt.Sprintf("no store on %s", goos)
		if ns.Store != "" {
			row.Source = ns.Store
		}
		row.Action = ns.Arm
		return row
	}
	row.Source = store.Name
	_, meta, err := readStore(store)
	if err != nil {
		var missing *NoSource
		if errors.As(err, &missing) {
			row.Action = missing.Arm
			return row
		}
		row.Expiry = "unreadable"
		row.Action = err.Error() + " — posse never writes this one (ADR 0019 D4); log in once with `claude` and its own login loop rewrites it"
		return row
	}
	row.Expiry = renderExpiry(meta.ExpiresAt, now)
	row.Action = "nothing posse may do — the runtime's login loop is this credential's only writer; run `claude` once to refresh it"
	return row
}

func (a *App) refreshReport(w io.Writer, o RefreshOpts) error {
	fmt.Fprintf(w, "posse refresh · credentials this box can see · %s · %s\n\n", o.os(), AbbrevHome(a.Home))
	for _, r := range a.CredReport(o) {
		fmt.Fprintf(w, "%s · %s · %s\n", r.Runtime, r.Purpose, r.Source)
		fmt.Fprintf(w, "    expiry: %s\n", r.Expiry)
		if r.Action != "" {
			fmt.Fprintf(w, "    action: %s\n", r.Action)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "this command is the operator's: it refuses under %s and without a TTY, and every crew PID\n", EnvPersona)
	fmt.Fprintln(w, "adds Bash(posse refresh:*) to deny: — that half lives in the constitution, not in this repo.")
	return nil
}

// renderExpiry says what is known and nothing more. A zero time is "cannot
// tell", which is the honest answer for every credential posse cannot ask;
// it is never rendered as freshness, and it warns nothing (ADR 0019 D5).
func renderExpiry(t, now time.Time) string {
	if t.IsZero() {
		return "cannot tell"
	}
	d := t.Sub(now)
	switch {
	case d <= 0:
		return "EXPIRED " + t.Format(stampDate)
	case d < 48*time.Hour:
		return fmt.Sprintf("%s (in %dh)", t.Format(stampDate), int(d.Hours()))
	default:
		return fmt.Sprintf("%s (in %dd)", t.Format(stampDate), int(d.Hours()/24))
	}
}

// ─── the write: a session mint, into an env set, by the operator's hand ──────

func (a *App) refreshSession(w io.Writer, rt *Runtime, o RefreshOpts) error {
	key := CageCredential(rt)
	if key == "" {
		return Die("runtime %s has no session credential name decided — codex and grok keep plain auth.json files and rangerhq-kiz left their container shape open; set cage_cred: in runtimes/%s.yaml before refreshing it", rt.Name, rt.Name)
	}
	if meteredCredentialNames[key] {
		return Die("%s is metered spending and posse does not write it (rangerhq-kiz, ADR 0019 D4): refusing", key)
	}
	expires, err := parseStampDate(o.Expires)
	if err != nil {
		return err
	}
	set, err := a.resolveRefreshSet(o.EnvSet, key)
	if err != nil {
		return err
	}
	tok, err := o.acquire(w, rt, key)
	if err != nil {
		return err
	}
	if err := checkSessionToken(tok, key); err != nil {
		return err
	}
	minted := o.now()
	p, err := a.setEnvVarStamped(set, key, tok, minted, expires)
	if err != nil {
		return err
	}
	a.TightenEnvPerms(w)
	fmt.Fprintf(w, "wrote %s into env set %s (%s), mode 0600 in a 0700 directory\n", key, set, AbbrevHome(p))
	fmt.Fprintf(w, "  %s%s\n", mintedStamp, minted.Format(stampDate))
	if expires.IsZero() {
		fmt.Fprintln(w, "  no expires stamp: posse cannot ask a setup-token when it dies, and will report this one as \"cannot tell\" rather than as fresh — pass --expires <YYYY-MM-DD> when you know")
	} else {
		fmt.Fprintf(w, "  %s%s\n", expiresStamp, expires.Format(stampDate))
	}
	fmt.Fprintf(w, "name that env set in the persona's PID envs: for a launch to see it (posse envs lists key names, never values)\n")
	return nil
}

// refreshMeter is the branch that writes nothing, and the reason it exists
// as a branch at all is that an operator will type it. Being told where the
// credential actually lives is the answer; refusing the argument is not.
func refreshMeter(w io.Writer, rt *Runtime, o RefreshOpts) error {
	store, ns := meterStore(rt.Name, o.os())
	if ns != nil {
		fmt.Fprintf(w, "%s has no meter credential on %s: %s\n", rt.Name, o.os(), ns.Error())
		return nil
	}
	fmt.Fprintf(w, "posse writes nothing here (ADR 0019 D4). %s's meter credential is the rotating OAuth token\n", rt.Name)
	fmt.Fprintf(w, "and its store of record is %s, whose only writer is the runtime's own login loop.\n", store.Name)
	fmt.Fprintf(w, "To refresh it: run `%s` once and let it log in. posse reads it there and never copies it —\n", rt.Name)
	fmt.Fprintln(w, "a copy of a rotating credential is a snapshot that disagrees with the source exactly when it matters.")
	return nil
}

// acquire gets the token in front of the operator, and this is the one
// design decision in the file worth arguing with, so here is the argument.
//
// The mint is run with stdin, stdout and stderr INHERITED — posse captures
// nothing. `claude setup-token` is an interactive terminal flow, and a
// captured stdout is a pipe: the flow would then be rendering into a buffer
// nobody is watching while the operator stares at a dead terminal. Whether
// its prompts land on stdout or stderr is somebody else's implementation
// detail, and betting the one credential command on it is the kind of guess
// that wedges a terminal.
//
// So posse runs the mint for the operator and then asks for the token it
// printed. Two steps where one would look neater, and the second step is
// also the whole of the --paste path (ADR 0019 D4's headless box), so there
// is exactly one code path that ever holds a token.
func (o RefreshOpts) acquire(w io.Writer, rt *Runtime, key string) (string, error) {
	if !o.Paste {
		argv, ok := runtimeMint[rt.Name]
		if !ok {
			return "", Die("posse knows no mint command for runtime %s — mint its session token by hand and re-run with --paste", rt.Name)
		}
		fmt.Fprintf(w, "running %s's own mint (%s) — its browser flow is the human gate, and posse is not in it\n", rt.Name, strings.Join(argv, " "))
		if err := o.minter()(w, rt); err != nil {
			return "", err
		}
		fmt.Fprintln(w, "the mint printed its token to your terminal; posse does not read your terminal.")
	}
	return o.asker()(w, fmt.Sprintf("paste the %s value (it is not echoed): ", key))
}

func (o RefreshOpts) minter() func(io.Writer, *Runtime) error {
	if o.mint != nil {
		return o.mint
	}
	return runRuntimeMint
}

func (o RefreshOpts) asker() func(io.Writer, string) (string, error) {
	if o.ask != nil {
		return o.ask
	}
	return askSecret
}

// runRuntimeMint execs the runtime's mint with the operator's own terminal
// on all three descriptors. Nothing is captured, so nothing can be logged.
func runRuntimeMint(w io.Writer, rt *Runtime) error {
	argv, ok := runtimeMint[rt.Name]
	if !ok {
		return Die("posse knows no mint command for runtime %s", rt.Name)
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return Die("%s is not on PATH, so its own mint cannot be run here — mint on a box that has it and bring the token over with --paste", argv[0])
	}
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return Die("%s exited without minting (%v) — nothing was written", strings.Join(argv, " "), err)
	}
	return nil
}

// askSecret reads one line from the terminal without echoing it.
func askSecret(w io.Writer, prompt string) (string, error) {
	fmt.Fprint(w, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(w)
	if err != nil {
		return "", Die("could not read the token from the terminal: %v", err)
	}
	return string(b), nil
}

// checkSessionToken is the last gate before a value reaches a file. It
// refuses on the money line, and it refuses anything that cannot BE an env
// set line — a value with a newline in it would write a second line the
// parser reads as another variable, or as nothing.
//
// Its errors name shapes and prefixes. They never quote the value.
func checkSessionToken(tok, key string) error {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return Die("no token was given — nothing was written")
	}
	if strings.ContainsAny(tok, "\n\r") {
		return Die("that value has a line break in it, and an env set line is one KEY=VALUE — nothing was written")
	}
	if strings.HasPrefix(tok, meteredKeyPrefix) {
		return Die("that value starts with %s…, which is a metered API key: a persona is never the one who decides to spend (rangerhq-kiz), and %s takes a setup-token minted with `claude setup-token` — nothing was written", meteredKeyPrefix, key)
	}
	return nil
}

func parseStampDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(stampDate, s)
	if err != nil {
		return time.Time{}, Die("--expires wants a date as YYYY-MM-DD, got %q", s)
	}
	return t.UTC(), nil
}

// resolveRefreshSet decides which env set is written. An explicit --env-set
// is taken as given (and created if it is not there — minting into a new set
// is an ordinary thing to do). Without one, the set that already holds this
// variable is the obvious answer when there is exactly one; anything else is
// refused by name, because the failure mode of guessing here is a token
// written where no launch will look for it.
func (a *App) resolveRefreshSet(explicit, key string) (string, error) {
	if explicit != "" {
		if !ValidName(explicit) {
			return "", Die("bad env set name '%s'", explicit)
		}
		return explicit, nil
	}
	sites := a.envSetsWith(key)
	switch len(sites) {
	case 0:
		return "", Die("no env set under %s holds %s yet, so there is nothing to refresh in place — name the set to write: posse refresh <runtime> --env-set <set>", AbbrevHome(a.EnvsDir), key)
	case 1:
		return sites[0].Set, nil
	}
	names := make([]string, 0, len(sites))
	for _, s := range sites {
		names = append(names, s.Set)
	}
	return "", Die("%d env sets hold %s (%s) — name the one to write: posse refresh <runtime> --env-set <set>", len(sites), key, strings.Join(names, ", "))
}

// ─── env set stamps: read and written as comments, beside the variable ───────

// envSite is one env set that holds a given variable, and what its stamps
// say about the value posse is not looking at.
type envSite struct {
	Set             string
	Minted, Expires time.Time
}

// envSetsWith finds every env set holding key, with its stamps, sorted by
// set name so a report has a stable order.
func (a *App) envSetsWith(key string) []envSite {
	var out []envSite
	names := a.ListEnvSets()
	sort.Strings(names)
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(a.EnvsDir, n+".env"))
		if err != nil {
			continue
		}
		if st, ok := readStamps(string(b), key); ok {
			st.Set = n
			out = append(out, st)
		}
	}
	return out
}

// readStamps finds key's assignment and reads the run of stamp comments
// immediately above it. "Immediately" is the whole rule: a blank line or any
// other assignment between a stamp and a variable means that stamp belongs
// to something else, and a stamp read off the wrong variable is a date
// reported about a credential it is not true of.
//
// The assignment is recognized exactly as parseEnvLines recognizes one, so
// the line posse rewrites is the line a launch reads.
func readStamps(data, key string) (envSite, bool) {
	var pending envSite
	for _, line := range strings.Split(data, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			pending = envSite{}
			continue
		}
		if strings.HasPrefix(t, "#") {
			if d, ok := strings.CutPrefix(t, mintedStamp); ok {
				pending.Minted, _ = time.Parse(stampDate, strings.TrimSpace(d))
			}
			if d, ok := strings.CutPrefix(t, expiresStamp); ok {
				pending.Expires, _ = time.Parse(stampDate, strings.TrimSpace(d))
			}
			continue
		}
		if assignsKey(line, key) {
			return pending, true
		}
		pending = envSite{}
	}
	return envSite{}, false
}

// assignsKey reads a line the way parseEnvLines does: "export " tolerated,
// KEY= at the front. Anything else is not this variable's line.
func assignsKey(line, key string) bool {
	return strings.HasPrefix(strings.TrimPrefix(line, "export "), key+"=")
}

// setEnvVarStamped writes key=value into env set `set`, replacing that one
// line and its stamps and preserving every other byte of the file — comments
// and hand-written structure included. WriteEnvSet, the TUI's writer, round-
// trips comments away by design; a command whose whole product is a stamped
// comment cannot use it.
//
// The write is atomic and never widens: a temp file in the same directory,
// mode 0600, renamed over. A partial write here is a session that cannot
// authenticate, and a 0644 window is the secret readable by every process on
// the box for as long as the window lasts.
func (a *App) setEnvVarStamped(set, key, value string, minted, expires time.Time) (string, error) {
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(a.EnvsDir, set+".env")
	if st, err := os.Lstat(p); err == nil && !st.Mode().IsRegular() {
		// A directory or a symlink where the env set goes. Named here rather
		// than left to the read below, whose "is a directory" is true and
		// says nothing about what posse was trying to do — and because a
		// symlink is a write posse would be aiming somewhere it did not
		// choose, with a credential in it.
		return "", Die("%s is not a regular file (%s), so posse will not write a credential there", AbbrevHome(p), st.Mode().Type())
	}
	old, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	next := replaceStamped(string(old), key, value, minted, expires)
	tmp, err := os.CreateTemp(a.EnvsDir, "."+set+".env.*")
	if err != nil {
		return "", err
	}
	// The temp holds the credential until the rename takes it away. Every
	// failure below leaves it behind without this, and none of those failures
	// is reachable from a unit test here (they are a write, a close and a
	// rename inside one directory posse just created) — so this line is
	// hygiene the suite cannot witness, and saying so beats a green test that
	// proves nothing about it.
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.WriteString(next); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return "", err
	}
	return p, nil
}

// replaceStamped is that rewrite as a pure function of the file's text, so
// the preservation property is testable without a filesystem.
func replaceStamped(old, key, value string, minted, expires time.Time) string {
	block := []string{mintedStamp + minted.Format(stampDate)}
	if !expires.IsZero() {
		block = append(block, expiresStamp+expires.Format(stampDate))
	}
	block = append(block, key+"="+value)

	lines := strings.Split(old, "\n")
	for i, line := range lines {
		if !assignsKey(line, key) {
			continue
		}
		// Drop the stamps this command last wrote above it; a comment that
		// is not one of ours is the operator's and stays where they put it.
		start := i
		for start > 0 {
			t := strings.TrimSpace(lines[start-1])
			if strings.HasPrefix(t, mintedStamp) || strings.HasPrefix(t, expiresStamp) {
				start--
				continue
			}
			break
		}
		out := append([]string{}, lines[:start]...)
		out = append(out, block...)
		out = append(out, lines[i+1:]...)
		return strings.Join(out, "\n")
	}
	// Not there yet: append, keeping exactly one trailing newline.
	body := strings.TrimRight(old, "\n")
	if body != "" {
		body += "\n"
	}
	return body + strings.Join(block, "\n") + "\n"
}
