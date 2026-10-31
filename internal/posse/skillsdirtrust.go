package posse

// ─── the cwd root's PRECONDITION on the session directory (ADR 0007 §6f) ────
//
// ADR 0007 §6a–6c make a cwd skills root a fact per RUNTIME and per CLI
// VERSION: a declared path, a `skills_verified:` line that says which
// release was measured reading it, and a drift report when the installed CLI
// moves off that line. grok's root is also a fact per DIRECTORY, and nothing
// declared that until this file.
//
// MEASURED 2026-10-31, grok 1.0.52 (0920c8169e27), darwin 25.6.0, two scratch
// git repos with IDENTICAL content under ONE fixture $HOME whose
// trusted_folders.toml named only the first (ranger-base-kwbh1, re-measuring
// ranger-base-9f2o6's finding with the two arms differing in the trust entry
// alone):
//
//	                        trusted repo                     untrusted repo
//	Project trusted:        yes                              no
//	Skills                  (1)  qm-alpha  project            (None)
//	Permissions Source      <repo>/.claude/settings.json      (none)
//	Permissions loaded      2                                0
//
// So an untrusted session directory reads NONE of the links the launch wrote,
// says nothing about why, and leaves the realized `skills:` binding silently
// empty — ADR 0007 §3's refusal arriving through the accepted path, which is
// ranger-base-65rc's defect with a different cause (there the plugin root was
// unreadable; here the root is right and the folder is not trusted).
//
// POSSE DOES NOT SEED IT, and that is this file's decision rather than its
// gap. The three sibling grants are the contrast:
//
//   - codex's `trust_level` and bob's `--trust` are ARGV — per directory,
//     session-scoped, visible in `ps`, and nothing is written to the
//     operator's box.
//   - claude's `hasTrustDialogAccepted` IS written (trust.go), and what makes
//     that narrow is what it buys: without it the session parks on a
//     full-screen modal and never becomes promptable, so the seed buys the
//     LAUNCH, and claude offers no line to type it on.
//
// grok has neither shape. There is no flag and no overlay — ADR 0007 §2
// measured the GROK_CONFIG/GROK_CONFIG_PATH overlay allowlisted to `models`,
// `features`, `toolset` and `shell_environment_policy`, unable to add a
// discovery source — so the only channel is a PERSISTENT, box-wide entry in
// the operator's own `~/.grok/trusted_folders.toml`, which outlives every
// session that would have wanted it. And the launch it would buy is not a
// dead one: an untrusted grok comes up fine, takes its prompt, and holds no
// skills. Last, the grant is not scoped to skills — the table above measured
// folder trust also deciding whether grok reads the repo's OWN
// `.claude/settings.json` as a permissions source, a repo→box authority
// channel grok's built-in declares nothing about (`ProjectConfig` is empty,
// and projectConfigRow's own `missing:` line calls that the silent one).
// Writing a grant on the operator's box that opens a channel nobody has
// measured is not posse's to do before somebody measures it; filed as
// ranger-base-1g8gf for the security lane (P1 — the channel is reachable on
// every grok launch in a trusted checkout, and grok's built-in declares
// ProjectConfig: nil, which projectConfigRow's own `missing:` line calls the
// silent one).
//
// So the remedy is the OPERATOR's, once per checkout, and the degrade line
// says so in those words.
//
// THE READING ASKS THE CLI AND NEVER MODELS IT. `grok inspect` is documented
// as "Show the configuration Grok discovers for this directory", prints
// `Project trusted: yes|no` as its fourth Environment line, and MEASURED on
// this box 2026-10-31: 0.061s wall, exit 0, no network, and `~/.grok`
// byte-identical before and after (a stat census over every file under it).
// That is cheap enough for the launch path and it is grok's own answer.
//
// The alternative — read `~/.grok/trusted_folders.toml` and decide here —
// was rejected on a measurement, not on taste. In
// `<this worktree>/.tmp-trustprobe/deep`, with NO entry for that path and no
// trusted ancestor of it anywhere in the table, grok answers
// `Project trusted: yes`: it resolves the GIT ROOT, and for a linked worktree
// it resolves on to the main checkout, which is what the table names
// (`~/src/posse`). Plus the key must be the path grok RESOLVES — a
// `t.TempDir()` repo is `/var/folders/…` and grok reads it as
// `/private/var/folders/…`, and the unresolved spelling matches nothing while
// reading exactly like no entry at all (ranger-base-9f2o6). A second
// implementation of that rule here would disagree with grok silently, and in
// the direction that matters: reading "trusted" over a folder grok will not
// read, which is the state this whole file exists to make loud.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// errGrokNotResolved: nothing to ask. Distinct from an exec that failed,
// because the reading an onboarder needs is different — grok is not on the
// PATH this process can see, which on a gated seat may not be the PATH the
// launch resolves on either (ranger-base-385x).
var errGrokNotResolved = errors.New("grok does not resolve on this PATH, outside the gates dirs")

// grokInspectTimeout bounds the one child this file starts — the deadline
// AND the WaitDelay, for the reasons grokInspectOutput's doc gives. A
// constant rather than a literal because it is now written twice, and two
// literals for one number is how the second one goes stale.
const grokInspectTimeout = 5 * time.Second

// SkillsDirTrust declares that a runtime's SkillsRoot is read only in a
// session directory the CLI itself considers trusted (ADR 0007 §6f).
//
// It is BUILT-IN ONLY, with no `runtimes/*.yaml` key, and that is a decision
// rather than an omission: the declaration without a Probe is a permanent
// UNKNOWN, and an UNKNOWN degrades nothing (below), so a declarable key would
// buy an onboarder a sentence that reads like a guarantee and changes no
// launch. A key is worth adding the day a second runtime has both a measured
// precondition and a probe posse can write for it.
type SkillsDirTrust struct {
	// What the precondition is, in the CLI's own vocabulary — the words a
	// reader will find in that CLI's own output or docs.
	What string
	// Where it is held: the surface that carries the grant, operator-owned
	// (posse never writes it — see this file's header).
	Where string
	// Grant: what makes it hold, and WHOSE act that is. This is the remedy
	// the degrade line prints, so it names a thing a person can do.
	Grant string
	// Probe asks the CLI whether dir meets the precondition. Read-only and
	// best-effort; nil = posse cannot cheaply tell, which is UNKNOWN and
	// never "no".
	Probe func(dir string) DirTrust
}

// DirTrust is what a read-only probe can honestly say about one session
// directory — three states, and the third is the one that decides whether a
// launch degrades. Silence's shape (runtime.go) and Silence's reason: posse
// refuses on a reading, never on ignorance (ranger-base-9r33).
type DirTrust struct {
	Trusted bool   // the CLI answered that it trusts this directory
	Unknown bool   // posse could not get an answer — "cannot tell", never "no"
	Why     string // the reading, in the probe's own words, for the line that prints it
}

// grokInspectOutput is the seam, and the seam is the READER — never the
// permission to read (ranger-base-02zr, codexInstalledVersion's rule). It
// runs `grok inspect` with dir as its cwd and hands back the whole stdout, so
// the parsing under test lives in grokDirTrust rather than in whatever a test
// injects.
//
// Resolved and PATH-scrubbed through the same two helpers readCLIVersionAt
// uses, for the same reason: a gates-dir shim is posse's own wrapper and
// never the CLI whose answer is wanted. The timeout is grokInspectTimeout,
// the same 5s cliVersionTimeout carries, for the same reason — a launch that
// hangs on a diagnostic read is worse than one that reports UNKNOWN. 5s
// against a measured 0.061s. (It was this file's own addition when
// ProbeCLIVersion had no deadline at all; that was fixed under
// ranger-base-fnn1o, where the number is measured and argued.)
//
// WaitDelay as well as the context, readCLIVersionWithin's rule and for its
// reason: os/exec's Wait does not return while a GRANDCHILD still holds the
// child's pipes, so killing grok does not on its own bound this call. That
// is not a hypothetical here — grok carries a self-updater, this read sits
// in FRONT of a launch, and the measured shape of the hang is a CLI that
// prints a line and leaves a forked child on stdout (ranger-base-fnn1o's
// fixture, ranger-base-2cttm). One duration for both, which bounds the whole
// call at twice it.
var grokInspectOutput = func(dir string) (string, error) {
	path := resolveOutside("grok", "")
	if path == "" {
		return "", errGrokNotResolved
	}
	ctx, cancel := context.WithTimeout(context.Background(), grokInspectTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "inspect")
	cmd.Dir = dir
	cmd.WaitDelay = grokInspectTimeout
	cmd.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return "", err
	}
	return string(out), nil
}

// grokDirTrust reads grok's own `Project trusted:` line for dir.
//
// The line is grok's, so the three answers are grok's too: `yes`, `no`, and
// anything else — a renamed line, a reworded value, a build that stops
// printing it — which is UNKNOWN and says the word it actually found. A probe
// that folded an unrecognised value into "no" would degrade every launch on
// the release that reworded it.
func grokDirTrust(dir string) DirTrust {
	out, err := grokInspectOutput(dir)
	return grokDirTrustFrom(dir, out, err)
}

// grokDirTrustFrom is the parse, split from the exec so the LIVE arm can hand
// it the bytes grok actually printed (TestQALiveSkillDiscoveryPerRuntime
// already runs `grok inspect` under a trusted and an untrusted fixture $HOME,
// and a probe pinned against its own fixture string would be pinned against
// nothing). Production reads the same function over the same bytes.
func grokDirTrustFrom(dir, out string, err error) DirTrust {
	if err != nil {
		return DirTrust{Unknown: true, Why: "`grok inspect` in " + AbbrevHome(dir) + " would not answer (" + err.Error() + ") — cannot tell whether grok trusts this directory"}
	}
	val, found := inspectField(out, "Project trusted:")
	switch {
	case !found:
		return DirTrust{Unknown: true, Why: "`grok inspect` in " + AbbrevHome(dir) + " printed no `Project trusted:` line — this grok does not report the precondition, so nothing here reads it"}
	case val == "yes":
		return DirTrust{Trusted: true, Why: "`grok inspect` in " + AbbrevHome(dir) + " reports `Project trusted: yes`"}
	case val == "no":
		return DirTrust{Why: "`grok inspect` in " + AbbrevHome(dir) + " reports `Project trusted: no`"}
	default:
		return DirTrust{Unknown: true, Why: "`grok inspect` in " + AbbrevHome(dir) + " reports `Project trusted: " + val + "`, which is neither yes nor no — an unrecognised value is unknown, never untrusted"}
	}
}

// inspectField pulls one `<label> <value>` out of `grok inspect`'s tree
// output, whose rows are drawn `  └ Project trusted: yes`. Matched on the
// label anywhere in the line rather than on the box-drawing prefix, because
// the prefix is decoration a release may redraw and the label is the fact.
func inspectField(out, label string) (string, bool) {
	for _, ln := range strings.Split(out, "\n") {
		if i := strings.Index(ln, label); i >= 0 {
			return strings.TrimSpace(ln[i+len(label):]), true
		}
	}
	return "", false
}

// GrokSkillsDirTrust is grok's declaration. See this file's header for the
// measurement, for why posse does not seed the grant, and for why the probe
// asks grok instead of reading its table.
var GrokSkillsDirTrust = &SkillsDirTrust{
	What:  "`grok inspect` reports `Project trusted: yes` for the session directory — grok reads the cwd skills root ONLY there, and an untrusted folder reads none of the links the launch wrote and says nothing about why",
	Where: "~/.grok/trusted_folders.toml — OPERATOR-owned, and posse never writes it: the grant is box-wide, it outlives the session that wanted it, and it also decides whether grok reads the repo's own .claude/settings.json as a permissions source (MEASURED 2026-10-31; the undeclared half of that is ranger-base-1g8gf)",
	Grant: "the OPERATOR accepts grok's folder trust once per checkout, in their own grok session in that directory. A git worktree and a subdirectory both inherit the enclosing checkout's grant (MEASURED 2026-10-31), so one acceptance per repo covers every seat posse opens in it",
	Probe: grokDirTrust,
}

// skillsGateKey is the Realized/Degraded key for a PID's skills binding, in
// ONE place because two layers now write it: CheckParity puts the row in, and
// the directory-aware layer below takes it out again when the CLI will not
// read the root. Two spellings of this string would make that removal a
// silent no-op (ranger-base-kwbh1).
func skillsGateKey(ag *AgentFile) string {
	return "skills: " + strings.Join(ag.Skills, ", ")
}

// applySkillsDirTrust is ADR 0007 §6f at the launch: the directory-aware half
// of the skills row, beside ProjectConfigTrust and for the same reason — which
// directory a session starts in is a fact about a concrete launch, not about a
// persona, so CheckParity's dir-independent matrix cannot hold it.
//
// Three arms, and only one of them touches the verdict:
//
//   - TRUSTED: the Realized detail carries the reading. That is the fact
//     ranger-base-9f2o6 had to measure by hand to find out why every grok
//     launch on this box gets its skills, and a session that prints it never
//     has to be measured for again.
//   - NOT TRUSTED: the row leaves Realized and the launch DEGRADES (ADR 0007
//     §3 — `skills:` is checked like a gate). Realized must not keep a row
//     for a root the CLI will not read: that row IS the lie 65rc landed on,
//     a clean launch with the binding realized and the persona holding
//     nothing. Degraded and never Unrealized, like every other skills arm:
//     nothing here is a wall claim.
//   - UNKNOWN: the detail carries the reading and the verdict is untouched.
//     Walling a launch because `grok inspect` would not run — a cage, a
//     missing CLI, a release that reworded its own line — refuses on
//     ignorance, which is the one thing DangerUnsilenced's UNKNOWN exclusion
//     settled for this tree (ranger-base-9r33).
//
// It says nothing at all where §6b already speaks: an UNPROVEN root (no
// skills_verified:) is the louder finding and has already degraded this
// launch, and a second line about a precondition on a root nobody has
// measured reading is noise over the top of it.
func (a *App) applySkillsDirTrust(p *Parity, ag *AgentFile, rt *Runtime, dir string) {
	if dir == "" || ag == nil || rt == nil || len(ag.Skills) == 0 {
		return
	}
	if rt.SkillsDirTrust == nil || rt.SkillsDirTrust.Probe == nil {
		return
	}
	if rt.SkillsRoot == "" || rt.SkillsVerified == "" {
		return
	}
	key := skillsGateKey(ag)
	row, realized := p.Realized[key]
	if !realized {
		// Another arm already decided this binding (no surface at all, or a
		// PID whose skills the runtime cannot take). Nothing to add.
		return
	}
	t := rt.SkillsDirTrust.Probe(dir)
	if !t.Trusted && !t.Unknown {
		delete(p.Realized, key)
		p.Degraded = append(p.Degraded, SkillsDirTrustDegrade(rt, ag, t))
		return
	}
	row.Detail += " — " + t.Why
	p.Realized[key] = row
}

// SkillsDirTrustDegrade is the sentence a NOT-TRUSTED reading prints. Shaped
// like projectConfigTrustMessage: what the runtime does, what was read, what
// it costs, and the remedy — because a degrade an operator cannot clear from
// the line itself is a dead end.
func SkillsDirTrustDegrade(rt *Runtime, ag *AgentFile, t DirTrust) string {
	return "skills: " + strings.Join(ag.Skills, ", ") + " — " + t.Why +
		", and " + rt.Name + " reads " + rt.SkillsRoot +
		" ONLY in a trusted folder (ADR 0007 §6f): the launch writes the links and this session reads NONE of them, with nothing said in the pane about why." +
		" The way through: " + rt.SkillsDirTrust.Grant + "." +
		" It is held in " + rt.SkillsDirTrust.Where
}
