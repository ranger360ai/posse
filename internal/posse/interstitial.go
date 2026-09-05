package posse

// Instance interstitials (ADR 0013 §2, layer 2) — the first-run dialogs a
// runtime draws that make a launched session un-promptable, and the
// operator-owned config key that silences each.
//
// POSSE DOCUMENTS THESE KEYS AND NEVER WRITES THEM, and the reason is not
// tidiness. Two of the three answers are the operator's alone:
//
//   - grok's "Help improve Grok" banner. `[Opt in]` lets xAI retain prompts
//     and traces from sessions working in the operator's PRIVATE repos —
//     a visibility line (crew guardrail 4), and no persona's to cross. The
//     privacy-preserving answer is `[Opt out]`, and it is a click.
//   - codex's update menu. Its DEFAULT-SELECTED option runs
//     `brew upgrade --cask codex` — an unreviewed roll-forward of the
//     operator's tooling, which is exactly what the grok pin exists to
//     prevent (rangerhq-y7jr). Enter on arrival would take it.
//
// So: a first-run dialog whose default action mutates the machine is a
// launch REFUSE until that config silences it, and nothing blind-sends
// Enter (ADR 0013 §2; measured in ranger-base-3j8). The layer-3
// declared-keystroke table stays Esc-only and last-resort.
//
// The probes below are READ-ONLY and best-effort: they answer "has the
// operator already silenced this on this machine", so `posse runtime check`
// can tell an onboarder what a launch would hit. An unreadable or absent
// file — or, where a probe asks the CLI itself, a CLI that will not say what
// it is — is "unknown", never "no".

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// grokHome / codexHome honour the CLIs' own home overrides, because a
// probe that reads the wrong file is worse than one that says unknown.
// $HOME and not os.UserHomeDir(), to match AbbrevHome: the two have to
// agree or the path printed is not the path read.
//
// And "" when neither the override nor $HOME names one, which is that same
// rule one step on. filepath.Join DROPS an empty element, so
// Join(os.Getenv("HOME"), ".grok") is ".grok" — a directory under whatever
// cwd the process happens to have, not a home. A repo carrying its own
// .grok/config.toml would then have ITS answer reported as the operator's,
// which is the wrong file this comment already refuses to read
// (ranger-base-58b5, from ranger-base-a3t1). Same shape as
// claudeHistoryPath: no home, no path, and the caller says unknown.
//
// os.UserHomeDir() is not a second source to fall back to. MEASURED,
// go1.26.5 darwin/arm64, `env -i`: UserHomeDir="" err="$HOME is not
// defined" — on unix it IS $HOME, so a fallback to it reads the same
// nothing.
func grokHome() string {
	if v := os.Getenv("GROK_HOME"); v != "" {
		return v
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".grok")
}

func codexHome() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".codex")
}

// noHomeSilence is what a probe answers when it has no home to read under:
// UNKNOWN, and never "not silenced". The difference is the one these probes
// are built on — a missing FILE is a reading, because these CLIs write these
// keys themselves and a config without one is a box that has not been told,
// while a missing HOME is no reading at all. Unknown is non-blocking
// everywhere (DangerUnsilenced skips it): posse does not wall a launch on
// what it did not read.
func noHomeSilence(override, tail string) Silence {
	return Silence{Unknown: true, Why: "$HOME is unset and " + override + " names no home either — nothing to read, so this " + tail}
}

// tomlFlag reads a bare `key = value` line from a TOML file. Deliberately
// line-level and section-blind: these two keys are unique in grok's
// config.toml, and a real TOML parser for a yes/no probe would be a
// dependency bought for nothing. A key that ever stops being unique there
// makes this wrong, which is why the caller prints the file and the key it
// read rather than a bare verdict.
func tomlFlag(path, key string) (val string, found bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, ln := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, key) {
			continue
		}
		rest := strings.TrimSpace(t[len(key):])
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(rest, "="))
		if i := strings.Index(v, "#"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		return strings.Trim(v, `"`), true
	}
	return "", false
}

// grokPrivacyProbe: has the coding-data consent banner been answered?
//
// It reports only that the banner is ANSWERED, never which way — grok
// writes the ACK, not the choice, and the choice is the operator's data
// decision either way. MEASURED on 1.0.5: the value is an RFC3339 stamp
// ("2026-08-24T21:35:58Z"), not a bool, which is why this tests for
// "present and not false" rather than for "true". A bool test read a live
// acked config as un-acked.
func grokPrivacyProbe() Silence {
	h := grokHome()
	if h == "" {
		return noHomeSilence("GROK_HOME", "cannot tell whether the banner has been answered")
	}
	p := filepath.Join(h, "config.toml")
	v, ok := tomlFlag(p, "privacy_banner_acked")
	if !ok || v == "" || v == "false" {
		// No config and an unanswered config are the same reading here, and
		// it is a reading rather than a shrug: grok writes the ack itself,
		// so a file that does not carry it is a banner that has not been
		// answered on this box.
		return Silence{Why: "privacy_banner_acked unset in " + AbbrevHome(p) + " — the banner re-arms every launch"}
	}
	return Silence{Silenced: true, Why: "privacy_banner_acked = " + v + " in " + AbbrevHome(p) + " (the ack, not the answer)"}
}

// grokAutoUpdateProbe: the fleet pin (etc/grok/version-pin.toml,
// rangerhq-y7jr) already kills grok's update check AND the shared leader's
// mid-life self-update, so grok has no update interstitial while it holds.
// `make verify-grok-pin` is the assertion; this is the cheap read.
func grokAutoUpdateProbe() Silence {
	h := grokHome()
	if h == "" {
		return noHomeSilence("GROK_HOME", "cannot tell whether the pin is applied")
	}
	p := filepath.Join(h, "config.toml")
	v, ok := tomlFlag(p, "auto_update")
	if !ok {
		return Silence{Why: "[cli] auto_update not set in " + AbbrevHome(p) + " — pin not applied (make verify-grok-pin)"}
	}
	return Silence{Silenced: v == "false", Why: "[cli] auto_update = " + v + " in " + AbbrevHome(p)}
}

// codexUpdateProbe: three silences, and only one of them expires.
//
// The DURABLE one is the fleet pin (etc/codex/version-pin.toml,
// ranger-base-poj5): check_for_update_on_startup = false in the operator's
// ~/.codex/config.toml, and codex never draws the menu again. Measured in a
// four-arm tmux rig against a version.json that was DUE a menu — key absent,
// key true, and an unrelated key all drew it; only this key at false did
// not. It is checked first because it outranks the others: with the startup
// check off, what version.json happens to say is not a reading about any
// screen the operator will see.
//
// The PER-RELEASE one is "3. Skip until next version", which writes
// dismissed_version into ~/.codex/version.json and lapses the moment
// latest_version moves past it. That is the shelf life this probe prints
// both numbers for, and on 2026-08-30 it is what walled every codex dispatch:
// the tap moved to 0.151.0 against a dismissal of 0.149.1, and DangerUnsilenced
// refuses on a reading of "no".
//
// The third is the one this probe read past for its whole life
// (ranger-base-cohw): THE BOX IS ALREADY AT THE LATEST RELEASE. codex draws
// the menu when a release NEWER than the running one exists, so an operator
// who UPDATED instead of dismissing has nothing to be offered — and comparing
// only the two fields of version.json called that box un-silenced forever.
// MEASURED 2026-08-29 on codex-cli 0.150.1 against
// {"latest_version":"0.150.1","dismissed_version":"0.149.1"}: this probe said
// "the menu is back" while two independent witnesses (a peeked launch pane
// with no "Update available", and herdr reading both live codex sessions
// idle rather than blocked on its own update_menu rule) said no menu drew.
// ADR 0013 §2 makes that reading a LAUNCH REFUSE, so the box that is MOST
// up to date was the one that could not launch.
//
// So the installed version is read — by the same reader the drift check uses,
// and only when the two cheap arms above have not already answered, because
// it costs a subprocess. If it cannot be read, or cannot be compared, the
// answer is UNKNOWN and not "no": without it there is no reading about the
// menu here at all, and posse refuses on a reading, never on ignorance
// (ranger-base-9r33). The probe keeps printing its numbers rather than a bare
// yes, because the dismissal arm still has a shelf life.
func codexUpdateProbe() Silence { return codexUpdateSilence(codexInstalledVersion) }

// codexInstalledVersion is the seam, and the seam is the READER — never the
// permission to read (ranger-base-02zr). ProbeCLIVersion is the same reader
// the parity drift check uses: resolved outside the gates dirs so a shim is
// never measured, memoized per process, and "" when codex is not there or
// will not answer. It returns codex's whole line ("codex-cli 0.150.1"), so
// the parsing under test lives here rather than in whatever a test hands in.
var codexInstalledVersion = func() string { return ProbeCLIVersion("codex") }

func codexUpdateSilence(installed func() string) Silence {
	h := codexHome()
	if h == "" {
		return noHomeSilence("CODEX_HOME", "cannot tell whether the update menu is silenced")
	}
	// The pin is a value, not a presence: a key that is present and true is
	// the menu ARMED, so this must never read as "someone mentioned it".
	cp := filepath.Join(h, "config.toml")
	if v, ok := tomlFlag(cp, "check_for_update_on_startup"); ok && v == "false" {
		return Silence{Silenced: true, Why: "check_for_update_on_startup = false in " + AbbrevHome(cp) + " — the menu is never drawn (fleet pin, make verify-codex-pin)"}
	}
	p := filepath.Join(h, "version.json")
	b, err := os.ReadFile(p)
	if err != nil {
		// UNKNOWN, and this is the one probe where the difference is
		// load-bearing: it is the only DANGER entry, so a reading of "no"
		// here refuses launches (DangerUnsilenced). codex writes this file
		// itself when it first checks for a release, so its absence is a
		// box that has not been told anything yet, not a box carrying a
		// menu — and posse does not wall a launch on what it did not read.
		return Silence{Unknown: true, Why: "unreadable " + AbbrevHome(p) + " — cannot tell whether the update menu is silenced"}
	}
	var v struct {
		Latest    string `json:"latest_version"`
		Dismissed string `json:"dismissed_version"`
	}
	if json.Unmarshal(b, &v) != nil {
		return Silence{Unknown: true, Why: "unparseable " + AbbrevHome(p) + " — cannot tell whether the update menu is silenced"}
	}
	// The operator's own answer, for exactly this release. First of the two
	// version arms because it is free: it settles the box without asking
	// codex anything.
	if v.Dismissed != "" && v.Dismissed == v.Latest {
		return Silence{Silenced: true, Why: "dismissed_version " + v.Dismissed + " = latest_version " + v.Latest + " — silenced until the next release"}
	}
	if v.Latest == "" {
		// Nothing to measure a dismissal or an install against. Reachable
		// on a version.json codex has written some other shape into, and
		// the old code read it as "the menu is back" — a refusal on a field
		// that was not there.
		return Silence{Unknown: true, Why: "no latest_version in " + AbbrevHome(p) + " — nothing to compare the installed codex against, so this cannot tell whether the update menu is silenced"}
	}
	line := installed()
	inst := versionNumber(line)
	if inst == "" {
		why := "could not read the installed codex version"
		if line != "" {
			why += " out of " + strconv.Quote(line)
		}
		return Silence{Unknown: true, Why: why + " (dismissed_version " + dismissedOrUnset(v.Dismissed) + ", latest_version " + v.Latest + " in " + AbbrevHome(p) +
			") — the menu draws only when a release NEWER than the running one exists, so this cannot tell whether it is silenced"}
	}
	cmp, ok := versionCmp(inst, v.Latest)
	if !ok {
		return Silence{Unknown: true, Why: "installed codex " + inst + " and latest_version " + v.Latest + " in " + AbbrevHome(p) +
			" cannot be compared — cannot tell whether the update menu is silenced"}
	}
	if cmp >= 0 {
		return Silence{Silenced: true, Why: "codex " + inst + " is installed and latest_version is " + v.Latest + " — there is nothing newer to offer, so the menu does not draw (dismissed_version " + dismissedOrUnset(v.Dismissed) + " is moot)"}
	}
	if v.Dismissed == "" {
		return Silence{Why: "dismissed_version unset in " + AbbrevHome(p) + " (installed " + inst + ", latest " + v.Latest + ") — the menu draws on next launch"}
	}
	return Silence{Why: "dismissed_version " + v.Dismissed + " but latest_version " + v.Latest + " and codex " + inst + " is installed — the menu is back"}
}

// dismissedOrUnset keeps the two numbers this probe prints from rendering as
// a blank where a version should be: an empty dismissal is a fact about the
// box, and "dismissed_version  is moot" reads like a truncated line.
func dismissedOrUnset(d string) string {
	if d == "" {
		return "unset"
	}
	return d
}

// versionNumber pulls the release number out of a CLI's own version line —
// codex prints "codex-cli 0.150.1", and a bare "0.150.1" is what several
// others print. The FIRST dotted-numeric field wins, not the last: a
// trailing "(3e1eaa3)" or a build date must not be taken for the release.
// Two segments minimum, so a stray "2" in a banner is not a version. "" when
// the line carries no such field, which every caller reads as "cannot tell",
// never as "old".
func versionNumber(line string) string {
	for _, f := range strings.Fields(line) {
		f = strings.TrimPrefix(f, "v")
		if parts, ok := versionParts(f); ok && len(parts) >= 2 {
			return f
		}
	}
	return ""
}

// versionParts splits a dotted numeric version. Digits only, and every
// segment required: a pre-release tail ("0.151.0-rc.1") is deliberately NOT
// comparable here, because guessing at its order is exactly the kind of
// answer this probe must not invent.
func versionParts(s string) ([]int, bool) {
	if s == "" {
		return nil, false
	}
	var out []int
	for _, seg := range strings.Split(s, ".") {
		if seg == "" {
			return nil, false
		}
		for i := 0; i < len(seg); i++ {
			if seg[i] < '0' || seg[i] > '9' {
				return nil, false
			}
		}
		n, err := strconv.Atoi(seg)
		if err != nil {
			return nil, false // a segment too long for an int
		}
		out = append(out, n)
	}
	return out, true
}

// versionCmp orders two dotted versions (-1, 0, 1), false when either is not
// one. Segment-wise and numeric, because the strings do not order: "0.150.1"
// sorts BEFORE "0.99.0" as text and after it as a release. A missing segment
// is a zero, so "0.150" and "0.150.0" are the same release.
func versionCmp(a, b string) (int, bool) {
	pa, oka := versionParts(strings.TrimPrefix(a, "v"))
	pb, okb := versionParts(strings.TrimPrefix(b, "v"))
	if !oka || !okb {
		return 0, false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		x, y := 0, 0
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		switch {
		case x < y:
			return -1, true
		case x > y:
			return 1, true
		}
	}
	return 0, true
}

// GrokInterstitials — measured on grok 1.0.5 (ranger-base-3j8,
// rangerhq-37c/7sbo/sz7u, pin rangerhq-y7jr).
var GrokInterstitials = []Interstitial{{
	Screen:  `"Help improve Grok  [Opt out] [Opt in]" consent banner above the composer`,
	Where:   "~/.grok/config.toml",
	Key:     "[privacy] privacy_banner_acked",
	Silence: "the OPERATOR clicks [Opt out] once, in their own grok session. Never [Opt in]: it donates prompts and traces from private-repo sessions to training (guardrail 4, visibility).",
	Probe:   grokPrivacyProbe,
}, {
	Screen:  "New worktree / Resume session / Quit startup menu, plus the changelog line",
	Where:   "~/.grok/config.toml (declared in etc/grok/version-pin.toml)",
	Key:     "[cli] auto_update = false, maximum_version",
	Silence: "already applied — the fleet pin kills the update check and the shared leader's mid-life self-update. `make verify-grok-pin` asserts it; NOTES.md \"grok substrate\" is the runbook for lifting it.",
	Probe:   grokAutoUpdateProbe,
}}

// CodexInterstitials — measured on codex-cli 0.147.0 (ranger-base-3j8,
// rangerhq-9py0); the durable silence and the cask pin re-measured on
// 0.150.1 (ranger-base-poj5).
var CodexInterstitials = []Interstitial{{
	Screen:  `"Update available! → 1. Update now  2. Skip  3. Skip until next version", footed "Press enter to continue". herdr reads it blocked (update_menu, etc/herdr/agent-detection/codex.toml — before that rule it fell through to idle with no rule matched), so a launch fails by name instead of waiting it out. Text sent to the untouched menu is discarded, not buffered: nothing typed there reaches a composer.`,
	Where:   "~/.codex/config.toml (declared in etc/codex/version-pin.toml), else ~/.codex/version.json",
	Key:     "check_for_update_on_startup = false, else dismissed_version",
	Silence: "already applied — the fleet pin sets check_for_update_on_startup = false and the menu is never drawn again. `make verify-codex-pin` asserts it, together with the `brew pin --cask codex` that makes \"1. Update now\" fail instead of upgrade. Without the pin there are two silences and both expire: the OPERATOR picking \"3. Skip until next version\" (arrow DOWN twice, verify the caret moved, THEN Enter), which lasts exactly one release, and the box simply BEING at the latest release, since codex has nothing newer to offer than what is running — which lasts until the next one ships (ranger-base-cohw).",
	Danger:  "the default-selected option is \"1. Update now\", which runs `brew upgrade --cask codex` — a pinned tool rolled forward with no decision, through a Homebrew this box has broken before (rangerhq-y5on). The cask pin makes that command exit 1 rather than upgrade, so the danger is what the screen ATTEMPTS, which is why it stays declared: the pin is a second thing that has to hold, not a reason to stop reading this one.",
	Probe:   codexUpdateProbe,
}}

// ClaudeInterstitials — the trust modal measured on claude 2.1.241
// (rangerhq-w4uf; four herdr scratch panes, no API turn), the outside-read
// notice on 2.1.258 (ranger-base-d3fwo, read out of the installed binary
// after the coordinator answered a live one by hand). Both entries are the
// table's declared exception: posse WRITES these keys at launch rather than
// naming them and refusing, because neither has a spelling a launch can
// type. Trust is per session directory, so every new repo, worktree and
// scratch dir the fleet starts in draws the modal again; the outside-read
// notice is per config dir, and the only settings key it names silences it
// by refusing the read it was asking about. See trust.go for both
// measurements and for what the trust grant hands the session dir.
var ClaudeInterstitials = []Interstitial{{
	Screen:  `"Quick safety check: Is this a project you created or one you trust?" — full-screen, "1. Yes, I trust this folder / 2. No, exit", footed "Enter to confirm · Esc to cancel". herdr reads it blocked (live_blocked_form), so dispatch waits it out rather than typing into it.`,
	Where:   "~/.claude.json (or $CLAUDE_CONFIG_DIR/.claude.json, or the config dir's .config.json when it exists)",
	Key:     `projects["<session dir>"].hasTrustDialogAccepted`,
	Silence: "the LAUNCH seeds it, per session dir, merged into the operator's file and only when the dir is not already trusted (SeedClaudeTrust) — the same grant posse types on codex's line, and the CLI's own documented alternative to answering the dialog by hand.",
	Seeded:  true,
	Probe:   claudeTrustProbe,
}, {
	Screen:  `"Read outside the working directories … Allow reads outside the working directories? 1. Yes, keep allowing / 2. No, block … / 3. No, ask again next time" — an auto-mode session's FIRST file-tool read of a path outside its working directories, so it lands mid-turn on a session that already looked healthy. herdr reads it blocked; a persona cannot answer it, and one sat on it until the coordinator sent the keystroke by hand (ranger-base-d3fwo).`,
	Where:   "~/.claude.json (or $CLAUDE_CONFIG_DIR/.claude.json) — top level, not per project",
	Key:     `hasSeenAutoModeOutsideReadPrompt`,
	Silence: "the LAUNCH seeds it (SeedClaudeTrust on the host, SeedCageHome in a cage), which is exactly what picking \"1. Yes, keep allowing\" writes and all it writes. The key the SCREEN names — permissions.blockReadsOutsideWorkingDirectories — is not the silence: false leaves the notice armed (the CLI's guard tests strictly true) and true silences it by refusing the read. See trust.go for the measurement.",
	Seeded:  true,
	Probe:   claudeOutsideReadProbe,
}}

// DangerUnsilenced is ADR 0013 §2's launch rule, in one place because three
// surfaces have to agree about it: the dispatch loop refuses before it
// claims (dispatch.go launchSession), every other launch path refuses or
// warns from planLaunch (herdrback.go), and `posse runtime check` reports
// the same reading as a BLOCKING gap (runtimepreflight.go). It returns one
// line per declared screen of rt whose default action mutates the machine
// and which this box cannot show to be silenced — empty when a launch here
// meets no such screen.
//
// Two exclusions, and each is a different kind of "not this rule":
//
//   - Danger == "" — the default action is safe. grok's consent banner is
//     the case: answering it wrong is a visibility decision, which is why
//     posse still never answers it, but arriving at it mutates nothing.
//   - Seeded — the launch writes this key itself (claude's directory
//     trust), so there is nothing for the operator to have done first.
//
// Everything else with Danger set yields a line, and the two readings that
// yield one are not the same sentence:
//
//   - the probe RAN and read "not silenced" — the codex case, and the line
//     carries the probe's own words (for codex, the two version numbers,
//     because the dismissal expires) beside the operator's action.
//   - there is NO probe, which is every screen declared in a
//     runtimes/<name>.yaml: posse cannot read an unknown CLI's config
//     format, so it can never read that key as silenced and the refusal
//     does not lift by silencing alone. The line says so, and names the
//     one thing that does lift it — dropping `danger:` from the profile.
//
// That second one reverses ranger-base-9r33, which excluded Probe == nil
// with "declaring a screen documents it and never walls the declarer's own
// launches", and it is worth saying why rather than just doing it
// (ranger-base-vbp3, ADR 0013 §2 amended). The exclusion made the rule
// unreachable for the only runtimes that can newly meet it: the built-ins
// are all measured, claude's screen is Seeded and codex/grok deliver by
// argv, so the FIRST typed-delivery runtime with a machine-mutating dialog
// is by construction a declared one — probe-less, and dispatched onto the
// menu while `runtime check` printed LAUNCH REFUSE about it. And it is
// still a reading rather than ignorance: `danger:` is not posse guessing
// at a config it cannot parse, it is the OPERATOR's own written statement
// that this screen's default action mutates their machine. Declaring it is
// choosing the wall; a declared screen without `danger:` still walls
// nothing.
//
// The UNKNOWN reading stays excluded, and that is the one worth arguing
// with, because "REFUSE until that config silences it" reads like silence
// must be SHOWN. A refusal whose own words are "cannot tell whether the
// update menu is silenced" walls a box for something nobody measured — and
// the screen is not unguarded meanwhile: herdr names it `blocked` by its
// own rule (etc/herdr/agent-detection/codex.toml, update_menu), so a launch
// that does meet it fails by name instead of being typed into. So this
// refuses on a reading, never on ignorance (ranger-base-9r33).
//
// Every line carries the operator's action, since a refusal an operator
// cannot clear from the line is a dead end.
func DangerUnsilenced(rt *Runtime) []string {
	if rt == nil {
		return nil
	}
	var lines []string
	for _, in := range rt.Interstitials {
		if in.Danger == "" || in.Seeded {
			continue
		}
		if in.Probe == nil {
			lines = append(lines, in.Key+" in "+in.Where+" CANNOT BE SHOWN SILENCED — "+
				declaredIn(rt)+" declares this screen and posse has no probe for "+rt.Name+
				", so nothing here ever reads that key"+
				". Its DEFAULT ACTION MUTATES THE MACHINE: "+in.Danger+
				". To silence it: "+in.Silence+
				" — and then drop danger: from that declaration, which is the whole of what this refusal is made of")
			continue
		}
		sil := in.Probe()
		if sil.Silenced || sil.Unknown {
			continue
		}
		lines = append(lines, in.Key+" is NOT silenced — "+sil.Why+
			". Its DEFAULT ACTION MUTATES THE MACHINE: "+in.Danger+
			". To silence it: "+in.Silence)
	}
	return lines
}

// declaredIn names the file a probe-less screen was declared in, for the
// refusal line. Empty Path is a built-in, which has no yaml — unreachable
// today, since all three built-ins carry measured Go probes, and worded so
// that a built-in that ever grows a probe-less Danger entry still prints a
// sentence rather than a bare " declares this screen".
func declaredIn(rt *Runtime) string {
	if rt.Path == "" {
		return "this runtime"
	}
	return AbbrevHome(rt.Path)
}

// DangerLine is DangerUnsilenced as the one sentence a refusal or a warning
// leads with ("" = nothing to say). Joined with "; " rather than newlines
// because both callers embed it in a message that adds its own indented
// lines under it.
func DangerLine(rt *Runtime) string {
	return strings.Join(DangerUnsilenced(rt), "; ")
}

// DangerRefusal is the launch refusal itself, so the dispatch loop and
// planLaunch cannot drift into saying different things about the same
// screen. It names the runtime, what is unsilenced, and the one place an
// operator goes to see the whole grid.
func DangerRefusal(rt *Runtime, line string) error {
	return Die("%s launch refused: %s\n"+
		"  ADR 0013 §2: a first-run dialog whose default action mutates the machine is a launch REFUSE until the operator's own config silences it, and posse never answers one — nothing blind-sends Enter\n"+
		"  the whole grid, with what this box reads today: posse runtime check %s",
		rt.Name, line, rt.Name)
}
