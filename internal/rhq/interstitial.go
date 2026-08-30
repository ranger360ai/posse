package rhq

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
// file is "unknown", never "no".

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// grokHome / codexHome honour the CLIs' own home overrides, because a
// probe that reads the wrong file is worse than one that says unknown.
// $HOME and not os.UserHomeDir(), to match AbbrevHome: the two have to
// agree or the path printed is not the path read.
func grokHome() string {
	if v := os.Getenv("GROK_HOME"); v != "" {
		return v
	}
	return filepath.Join(os.Getenv("HOME"), ".grok")
}

func codexHome() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	return filepath.Join(os.Getenv("HOME"), ".codex")
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
	p := filepath.Join(grokHome(), "config.toml")
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
	p := filepath.Join(grokHome(), "config.toml")
	v, ok := tomlFlag(p, "auto_update")
	if !ok {
		return Silence{Why: "[cli] auto_update not set in " + AbbrevHome(p) + " — pin not applied (make verify-grok-pin)"}
	}
	return Silence{Silenced: v == "false", Why: "[cli] auto_update = " + v + " in " + AbbrevHome(p)}
}

// codexUpdateProbe: codex's update menu is silenced for one release at a
// time — "3. Skip until next version" writes dismissed_version into
// ~/.codex/version.json, and the menu returns when latest_version moves
// past it. So this is a probe with a shelf life, which is the point of
// printing the two versions rather than a bare yes.
func codexUpdateProbe() Silence {
	p := filepath.Join(codexHome(), "version.json")
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
	if v.Dismissed == "" {
		return Silence{Why: "dismissed_version unset in " + AbbrevHome(p) + " (latest " + v.Latest + ") — the menu draws on next launch"}
	}
	if v.Dismissed == v.Latest {
		return Silence{Silenced: true, Why: "dismissed_version " + v.Dismissed + " = latest_version " + v.Latest + " — silenced until the next release"}
	}
	return Silence{Why: "dismissed_version " + v.Dismissed + " but latest_version " + v.Latest + " — the menu is back"}
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
// rangerhq-9py0).
var CodexInterstitials = []Interstitial{{
	Screen:  `"Update available! → 1. Update now  2. Skip  3. Skip until next version", footed "Press enter to continue". herdr reads it blocked (update_menu, etc/herdr/agent-detection/codex.toml — before that rule it fell through to idle with no rule matched), so a launch fails by name instead of waiting it out. Text sent to the untouched menu is discarded, not buffered: nothing typed there reaches a composer.`,
	Where:   "~/.codex/version.json",
	Key:     "dismissed_version",
	Silence: "the OPERATOR picks \"3. Skip until next version\" (arrow DOWN twice, verify the caret moved, THEN Enter). It silences one release; the menu returns when latest_version moves.",
	Danger:  "the default-selected option is \"1. Update now\", which runs `brew upgrade --cask codex` — a pinned tool rolled forward with no decision, through a Homebrew this box has broken before (rangerhq-y5on)",
	Probe:   codexUpdateProbe,
}}

// ClaudeInterstitials — measured on claude 2.1.241 (rangerhq-w4uf; four
// herdr scratch panes, no API turn). The one entry is the table's declared
// exception: posse WRITES this key at launch, because it has no once-per-
// machine spelling — trust is per session directory, so every new repo,
// worktree and scratch dir the fleet starts in draws the modal again, and
// there is no flag, no settings key and no `claude project` subcommand to
// answer it on the line. See trust.go for the measurement and for what the
// grant hands the session dir.
var ClaudeInterstitials = []Interstitial{{
	Screen:  `"Quick safety check: Is this a project you created or one you trust?" — full-screen, "1. Yes, I trust this folder / 2. No, exit", footed "Enter to confirm · Esc to cancel". herdr reads it blocked (live_blocked_form), so dispatch waits it out rather than typing into it.`,
	Where:   "~/.claude.json (or $CLAUDE_CONFIG_DIR/.claude.json, or the config dir's .config.json when it exists)",
	Key:     `projects["<session dir>"].hasTrustDialogAccepted`,
	Silence: "the LAUNCH seeds it, per session dir, merged into the operator's file and only when the dir is not already trusted (SeedClaudeTrust) — the same grant posse types on codex's line, and the CLI's own documented alternative to answering the dialog by hand.",
	Seeded:  true,
	Probe:   claudeTrustProbe,
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
// Three exclusions, and each is a different kind of "not this rule":
//
//   - Danger == "" — the default action is safe. grok's consent banner is
//     the case: answering it wrong is a visibility decision, which is why
//     posse still never answers it, but arriving at it mutates nothing.
//   - Seeded — the launch writes this key itself (claude's directory
//     trust), so there is nothing for the operator to have done first.
//   - the reading is UNKNOWN — either Probe == nil, because posse cannot
//     read this CLI's config format at all (every interstitial declared in
//     a runtimes/<name>.yaml is here, so an operator who declares a screen
//     documents it and does not thereby wall their own launches), or the
//     probe ran and could not read the file (Silence.Unknown).
//
// That last exclusion is the one worth arguing with, because "REFUSE until
// that config silences it" reads like silence must be SHOWN. Two things
// answer it. A refusal whose own words are "cannot tell whether the update
// menu is silenced" walls a box for something nobody measured — and the
// screen is not unguarded meanwhile: herdr names it `blocked` by its own
// rule (etc/herdr/agent-detection/codex.toml, update_menu), so a launch
// that does meet it fails by name instead of being typed into. So this
// refuses on a reading, never on ignorance (ranger-base-9r33).
//
// The line carries the probe's own words (which for codex are the two
// version numbers, because the dismissal expires) and the operator's
// action, since a refusal an operator cannot clear from the line is a dead
// end.
func DangerUnsilenced(rt *Runtime) []string {
	if rt == nil {
		return nil
	}
	var lines []string
	for _, in := range rt.Interstitials {
		if in.Danger == "" || in.Seeded || in.Probe == nil {
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
