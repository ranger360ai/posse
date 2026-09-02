package posse

// Gates L2 — the seatbelt tier (ADR 0002 §3): the runtime command runs
// under `sandbox-exec -f RHQ_HOME/state/gates/<persona>/seatbelt.sb`, a
// profile that denies file-write* everywhere except what a persona
// session legitimately writes: the repo (unless the PID denies Edit/
// Write — then only its .beads/), the persona's memory dir, the LAUNCHING
// runtime's own state and no other runtime's (state_dir:, ADR 0012 D4 —
// narrowed from the union of all three built-ins by ranger-base-9fl),
// posse's own state dir under the home it resolved, TMPDIR, the gates dir
// (for refusals.log), /dev, the atomic-write siblings of any grant that
// names a FILE (SeatbeltSiblings, ranger-base-cypy1), and the PID's
// `writable:` extras. What it never grants is the rest of the home:
// after ADR 0015 §2 that is the promoted constitution, and a promoted copy
// stays in force because no session can write it. This is the only
// runtime-proof file gate: it realizes Edit/Write-class denies on any
// runtime, model behind it notwithstanding. sandbox-exec is deprecated by
// Apple but is what codex itself ships on today; its successor is the
// container tier.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// SeatbeltAvailable reports whether this HOST can run the seatbelt tier.
// Deliberately a host question and not a process one: what it gates is a
// session posse is about to launch, and that session's `sandbox-exec` is
// typed into a herdr pane (startPlanned's PaneRun), so it runs in herdr's
// process tree and not in this one's sandbox. A posse command run inside a
// caged persona session may not apply a profile itself and can still launch
// a caged session — those are two different questions, and the second one
// is sandboxApplyRefusal below (ranger-base-heur).
func SeatbeltAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func init() {
	if SeatbeltAvailable() {
		AvailableCages[CageSeatbelt] = true
	}
}

// sandboxApplyProbeProfile is what the probe below applies: everything
// allowed and one deny on a path that is not there, so the child it wraps
// is constrained in no way that matters and the only thing measured is
// whether the kernel accepted the apply at all.
//
// The deny is not decoration. MEASURED over all four corners on darwin
// 25.4.0 (ranger-base-xjw9, TestQASandboxApplyProbeGrid): a deny in EITHER
// profile refuses the nested apply, so under a lenient allow-default
// wrapper a lenient probe reports "sandboxable" while every profile
// SeatbeltProfile actually emits — it carries `(deny file-write*)`
// unconditionally — is refused. A probe has to be shaped like the thing it
// predicts.
const sandboxApplyProbeProfile = `(version 1)
(allow default)
(deny file-write* (subpath "/nonexistent-rhq-sandbox-apply-probe"))
`

// sandboxApplyRefusal is "" when THIS PROCESS may apply a seatbelt profile,
// and the kernel's own words when it may not:
//
//	$ sandbox-exec -p '(version 1)(allow default)' /usr/bin/true
//	sandbox-exec: sandbox_apply: Operation not permitted
//
// A posse command run inside a caged persona session is in exactly that
// position — sandbox-exec is still on PATH there (measured, ranger-base-xjw9)
// and the kernel refuses the nested apply outright. Every check posse
// performs BY applying a profile has to ask this before it believes its own
// result: the reachability probe (reachability.go) reported the refusal as
// a store of record the profile denies, which is a finding about the grant
// drawn from a measurement that never happened, and it degraded the launch
// (ranger-base-heur).
//
// A variable so a test can drive both answers on a kernel that only gives
// one; liveSandboxApplyRefusal is what production runs.
var sandboxApplyRefusal = liveSandboxApplyRefusal

var (
	sandboxApplyOnce sync.Once
	sandboxApplyWhy  string
)

// liveSandboxApplyRefusal asks the kernel, once per process: the answer can
// only change by this process gaining a sandbox, which posse never does to
// itself, and the reach probe would otherwise pay for a fork per target.
func liveSandboxApplyRefusal() string {
	sandboxApplyOnce.Do(func() {
		if !SeatbeltAvailable() {
			return
		}
		out, err := exec.Command("sandbox-exec", "-p", sandboxApplyProbeProfile, "/usr/bin/true").CombinedOutput()
		if err == nil {
			return
		}
		// Only an apply refusal is ours to abstain on. Anything else — no
		// /usr/bin/true, a profile this OS version rejects — is a real
		// problem with this host, and the check that meets it should report
		// it as itself rather than disappear into an abstention.
		if s := strings.TrimSpace(string(out)); isSandboxApplyRefusal(s) {
			sandboxApplyWhy = s
		}
	})
	return sandboxApplyWhy
}

// isSandboxApplyRefusal separates the kernel refusing the APPLY from the
// sandboxed child being refused a WRITE. The first is sandbox-exec's own
// `sandbox_apply: Operation not permitted` and means nothing was measured;
// the second is the shell's `Operation not permitted` on a path, and is the
// finding the reachability row exists to make. Both carry the same three
// words, which is how one was read as the other.
func isSandboxApplyRefusal(s string) bool {
	return strings.Contains(s, "sandbox_apply")
}

// sbQuote quotes a path for SBPL (double-quoted string, backslash-escaped).
func sbQuote(p string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(p) + `"`
}

// sbSiblingRegex renders the SBPL pattern matching every path that extends
// p with a dot and something — `<p>.lock`, `<p>.tmp.<pid>.<hex>`,
// `<p>.backup` — and nothing else. NOT p itself: the base is granted by its
// own `(subpath …)` entry or it is not granted at all, so this adds
// siblings and never a root (siblingBases, and the property
// ConstitutionGrants below is extended to check).
//
// The dot is load-bearing and MEASURED (2026-09-02, darwin 25.4.0, four
// arms on one fixture): a bare prefix `^<p>` also grants `<p>Xlock` and
// `<p>` itself — the shape that would hand a `~/.codex` grant this box's
// `~/.codexbar` and a `~/.grok` grant its `~/.grokbot`, both of which
// exist. Anchored at `\.` those are refused and only the sibling
// namespace is allowed.
//
// `#"…"` is NOT the same lexer as `"…"`, also measured: it passes
// backslashes through raw to the regex engine (an sbQuote-style `\\.`
// reaches it as an escaped backslash and matches nothing), and it has no
// escape for a `"` at all — a quote in the path ends the literal and
// sandbox-exec refuses the whole profile at parse time ("unbound
// variable"). So the base is regexp-quoted, never sbQuote'd, and a path
// carrying a `"` is dropped upstream rather than rendered here.
func sbSiblingRegex(p string) string {
	return `#"^` + regexp.QuoteMeta(p) + `\."`
}

// SeatbeltCarveOut is the profile's trailing block: what stays unwritable
// however wide the allow block above it got (ranger-base-h15, and ADR 0014
// §3's slot — a PID's path-scoped denies join Deny here).
//
// It exists because the allow block cannot be made narrow enough. `cwd` is
// granted whole to any PID that does not deny Edit/Write, and the home is a
// symlink INTO the constitution repo — so a session dispatched into that
// repo is handed `rhq/agents`, the PIDs every gate is rendered from, inside
// an ordinary project grant (ranger-base-6ne, and measured again on
// ranger-base-0djg). Narrowing the grant is not available: the session is
// there to work in that tree.
//
// The four lists are four different SBPL shapes and are kept apart for
// that reason, not for tidiness:
//
//   - Deny: subpaths, denied after the allow. MEASURED (2026-08-28, macOS
//     26.4): a trailing deny beats an enclosing allow for touch, `sed -i`,
//     python, rm, mkdir — and a `subpath` naming a FILE (promoted.json)
//     denies that file. Same last-match-wins as ADR 0014 §3 measured for
//     the path-scoped case.
//
//   - Seal: literals on the directories an allowed rename could carry a
//     denied tree out from under its own deny. `mv rhq rhq2` is a write on
//     `rhq`, which the allow grants and no subpath deny below it names, and
//     the constitution is then writable at a path the profile never heard
//     of — MEASURED, allowed before the seal and refused after. A literal
//     deny on the directory does not stop writes INSIDE it (measured:
//     touch/mkdir/rm in `rhq` still pass), so this costs nothing.
//
//   - Keep: literals re-allowed after the deny, because the gates dir the
//     deny closes is also where the session's own audit trail lands: L1's
//     `refusals.log` and the gate shell's `shell.log`. Both are appended to
//     from INSIDE the cage, and both are created on first append —
//     measured: a `literal` allow under a denied subpath creates and
//     appends. Nothing else in that directory (the shims, seatbelt.sb, or
//     another persona's dir) comes back.
//
//   - DenyRead: literals denied `file-read*` (ranger-base-hw18, ADR 0019
//     D2 item 3) — not a subtree, because the only thing worth walling is
//     the credential file itself, not the directory it sits in (a
//     runtime's own state dir is still granted `file-write*` above and
//     needs no read wall at all). Computed by credentialReadDenyLiterals,
//     which is where the runtime-aware reasoning and the GOOS shape live.
//     Unlike Deny/Seal/Keep this list answers no question about the
//     allow block above it: nothing in this profile ever allows a read,
//     so there is nothing for it to outvote — it stands alone in its own
//     `(deny file-read* …)` block (SeatbeltProfile) rather than joining
//     the write carve-out's last-match-wins trick.
//
// A hardlink needs no rule of its own, and the profile deliberately does
// not carry one: `ln <denied>/x ./x` into a granted directory is REFUSED by
// the file-write* deny already — measured, under both spellings of the
// path, with the error on the destination. An earlier reading of this said
// otherwise; that measurement had renamed the tree in an earlier probe, so
// the source it linked was no longer under the deny.
//
// The residual the tier cannot close is unchanged and named in ADR 0025:
// SBPL cannot tell an append from a truncation, so the session a log
// records can still forge it. The deny takes the log out of reach of every
// OTHER persona's session, which is what item 2 of the bead asked for.
type SeatbeltCarveOut struct {
	Deny     []string // subpaths no grant above may reach
	Seal     []string // directories whose rename would carry a Deny path away
	Keep     []string // literals re-allowed after the deny
	DenyRead []string // literal files no session may file-read*, whatever grants it
}

// Empty reports whether the block renders nothing.
func (c SeatbeltCarveOut) Empty() bool {
	return len(c.Deny) == 0 && len(c.Seal) == 0 && len(c.DenyRead) == 0
}

// SeatbeltProfile renders the SBPL profile text: the default deny, the
// allow block, and then the carve-out the allow block cannot outvote.
//
// `siblings` is the atomic-write sibling namespace of a grant that names a
// FILE (SeatbeltSiblings, ranger-base-cypy1) — rendered as regexes inside
// the same allow block, because a `(subpath …)` naming a file covers the
// file and nothing beside it.
func SeatbeltProfile(persona string, writable, siblings []string, carve SeatbeltCarveOut, createOnly ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, ";; posse seatbelt for %s — rendered from the PID at launch; do not edit (rangerhq-5vt)\n", persona)
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n")
	b.WriteString("(allow file-write*\n")
	for _, p := range writable {
		if p == "" {
			continue
		}
		fmt.Fprintf(&b, "  (subpath %s)\n", sbQuote(p))
	}
	// The scratch names beside a granted FILE: `<p>.lock`, `<p>.tmp.…`.
	// Without these a CLI that replaces its config atomically loses every
	// write and says nothing (ranger-base-cypy1).
	for _, p := range siblings {
		if p == "" {
			continue
		}
		fmt.Fprintf(&b, "  (regex %s)   ; atomic-write siblings of %s\n", sbSiblingRegex(p), filepath.Base(p))
	}
	b.WriteString("  (regex #\"^/dev/\")\n")
	b.WriteString("  (literal \"/dev/null\")\n")
	b.WriteString(")\n")
	// Create-only, and above the carve-out so the carve-out still outvotes
	// it: the directories git must MAKE to write a ref it may write
	// (sessionRefDirs, ranger-base-uuze). A subpath here would grant every
	// sibling session's ref; this grants the mkdir and nothing inside.
	if len(createOnly) > 0 {
		b.WriteString(";; create-only (ranger-base-uuze): making these paths is allowed,\n;; writing anything inside them is not.\n")
		b.WriteString("(allow file-write-create\n")
		for _, p := range createOnly {
			if p == "" {
				continue
			}
			fmt.Fprintf(&b, "  (literal %s)\n", sbQuote(p))
		}
		b.WriteString(")\n")
	}
	if len(carve.Deny) > 0 || len(carve.Seal) > 0 {
		b.WriteString(";; the carve-out (ranger-base-h15): LAST match wins in SBPL, so these\n;; override every grant above them, cwd included.\n")
		b.WriteString("(deny file-write*\n")
		for _, p := range carve.Deny {
			fmt.Fprintf(&b, "  (subpath %s)\n", sbQuote(p))
		}
		for _, p := range carve.Seal {
			fmt.Fprintf(&b, "  (literal %s)   ; rename seal\n", sbQuote(p))
		}
		b.WriteString(")\n")
		if len(carve.Keep) > 0 {
			b.WriteString("(allow file-write*\n")
			for _, p := range carve.Keep {
				fmt.Fprintf(&b, "  (literal %s)\n", sbQuote(p))
			}
			b.WriteString(")\n")
		}
	}
	if len(carve.DenyRead) > 0 {
		// No allow-block to outvote (ranger-base-hw18's own note): nothing
		// above ever allows file-read*, so this needs no last-match-wins
		// positioning and stands in its own block rather than joining the
		// write carve-out above.
		b.WriteString(";; credential read deny (ranger-base-hw18, ADR 0019 D2 item 3)\n")
		b.WriteString("(deny file-read*\n")
		for _, p := range carve.DenyRead {
			fmt.Fprintf(&b, "  (literal %s)\n", sbQuote(p))
		}
		b.WriteString(")\n")
	}
	return b.String()
}

// SeatbeltWritable computes the writable set for a persona session:
// cwd unless the PID denies Edit or Write (then only cwd/.beads so bd can
// still claim/comment/close), the store of record when a redirect puts it
// in another repo, memory dir, the LAUNCHING runtime's state dirs (the
// stateDirs argument, and no other runtime's — ranger-base-9fl), posse's
// own state dir, TMPDIR, the gates dir, plus the PID's writable: extras
// (relative to cwd).
//
// What it does NOT return is the sibling namespace of an entry that names
// a file: `~/.claude.json` is granted here, and `~/.claude.json.lock` and
// `~/.claude.json.tmp.<pid>.<hex>` — the two paths claude actually writes —
// are not, because `subpath` is component-aware. That is SeatbeltSiblings,
// kept apart because it renders as a regex and not as a subpath, and
// because every reader of this slice treats its elements as subpaths.
//
// It hangs off App because one of those paths is under the home, and after
// ADR 0015 §2 the home is a real directory holding the promoted
// constitution beside `state/`. A grant spelled as a literal path — which
// this was, `~/.config/rhq/state` — is a grant that names the wrong home
// the day the home moves, and the profile then silently loses the state
// dir it meant to open (ranger-base-cpyb). Ask the App: it resolved the
// home this process is actually running against.
func (a *App) SeatbeltWritable(ag *AgentFile, cwd, gatesDir string, stateDirs ...string) []string {
	home, _ := os.UserHomeDir()
	deniesFiles := deniesFileWrite(ag.Deny)
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		out = append(out, absResolve(p))
	}
	if cwd != "" {
		if deniesFiles {
			add(filepath.Join(cwd, ".beads"))
			add(filepath.Join(cwd, ".git")) // index refresh, hooks' own logs — never a push
		} else {
			add(cwd)
		}
		// A session worktree's `.git` is a FILE, and its index, HEAD, objects
		// and refs live outside the tree entirely (rangerhq-09o2). Granting
		// cwd alone leaves a persona that cannot commit in its own tree —
		// the same shape as the redirect grant below, for the session's own
		// repo instead of the store of record. Empty in the main checkout,
		// and narrowed below the common dir (ranger-base-m2wf).
		for _, g := range sessionGitGrants(cwd) {
			add(g)
		}
		// The store of record is not under cwd when a redirect moves it
		// (ADR 0012 D3-C): cwd/.beads holds a path and the database, its
		// jsonl, socket and lock live in the instance repo it names. The
		// two grants above then cover a directory bd never writes, and
		// every mutation lands outside the profile — measured
		// (ranger-base-rhw): `bd sync` and `bd export` fail on the db file
		// ("operation not permitted"), and a commit of anything in that
		// repo — the persona's own ORDERS.md included — fails on
		// .git/index.lock. Grant the resolved directory and that repo's
		// git dirs, and nothing else: the instance tree stays unwritable,
		// which is the point of the tier. Not conditional on deniesFiles —
		// cwd's subpath does not reach another repo either way.
		//
		// realizeCodex names the same directory for the runtimes that cage
		// themselves (runtime.go, ranger-base-0fb); this is that grant at
		// L2, same resolver, same one-hop bound (ranger-base-7kw).
		if home := beadsHome(cwd); !underDir(cwd, home) {
			add(home)
			for _, g := range beadsGitDirs(home) {
				add(g)
			}
		}
	}
	// §5's named exception: memory is not law. `home/personas` is a symlink
	// into the constitution repo, and this is the one grant that follows it
	// — absResolve resolves the link, so the profile matches the REAL
	// directory and the spelling cannot dodge the wall in either direction:
	// another persona's dir is not granted under either name, and the
	// session's own dir is granted under both.
	add(ag.MemoryDir)
	add(gatesDir)
	// posse's own state, derived from the home. Everything else under the
	// home — the promoted set, its manifest, `envs/` — is deliberately
	// absent, and ConstitutionGrants below is how that is checked rather
	// than asserted (ADR 0015 §2/§3/§7).
	add(a.StateDir)
	// The generic caches every CLI on this box writes through. These are
	// NOT runtime state and stay a literal: they belong to npm, to macOS and
	// to the XDG layout, not to any engine.
	//
	// The Go toolchain's local telemetry counters are that class too, and
	// they were 18% of one day's denial volume before this line: `go`,
	// `compile`, `link` and `asm` each mmap a counter file under
	// `local/` on every invocation and each denied write is logged
	// (~2,300/day across two caged sessions, ranger-base-gr3ow). The
	// build succeeds either way — the counters are local-only and nothing
	// reads them — so this buys quiet, not function.
	//
	// `local` and NOT its parent, deliberately: `telemetry/mode` sits
	// beside it and is the only thing that decides whether the toolchain
	// UPLOADS those counters to telemetry.go.dev. Granting the parent
	// would let a session flip the operator's box from "local" to "on" —
	// an egress the operator never chose (crew guardrail 4). Granted this
	// way the counters are writable and the switch is not.
	//
	// The env route the bead proposed does not exist: GOTELEMETRY is a
	// derived, non-settable `go env` value read from that mode file, and
	// `go env -w GOTELEMETRY=off` is refused by name. Measured — a build
	// with GOTELEMETRY=off in the environment writes exactly the four
	// counter files a build without it does. The only lever that stops
	// the writes is the mode file, which is account-global and hand-
	// applied; this is the versioned one.
	for _, d := range []string{"Library/Caches", "Library/Logs", ".cache", ".npm", ".local/share", "Library/Application Support/go/telemetry/local"} {
		if home != "" {
			add(filepath.Join(home, d))
		}
	}
	// The LAUNCHING runtime's own state dirs, and no other runtime's.
	// `~/.claude ~/.claude.json ~/.codex ~/.grok` were spelled here as a
	// literal until ADR 0012 D4, which is why a third-party CLI declared in
	// runtimes/<name>.yaml got a READ-ONLY state dir under `cage: seatbelt`
	// and no line anywhere said so: it re-ran its first-run flow every
	// launch, or died on a config write.
	//
	// D4 made the key declarable but kept granting the UNION of the
	// built-ins on top, so a claude session could write ~/.codex and
	// ~/.grok. That is not a config inconvenience, it is an integrity
	// vector (ranger-base-9fl, from the ADR 0019 posture review): swap
	// ~/.grok/auth.json for a token on an attacker-controlled account and
	// the NEXT grok session on this box sends its transcripts there — an
	// exfil channel that outlives the session that planted it, and one no
	// read deny can close because the planting is a WRITE. The read half is
	// already narrowed the same way (credentialReadDenyLiterals): a runtime
	// is denied every credential store but its own. This is that rule on
	// the write side, and the two now agree on which runtime the session is.
	//
	// So: stateDirs, nothing else. A caller with no runtime in hand grants
	// no runtime state at all — the fail-closed direction, and visible as
	// the first-run flow D4 already named rather than as a silent grant.
	// Every production caller resolves the launch's runtime and passes
	// rt.StateDirs (planLaunch and RelaunchAgent in herdrback.go, the reach
	// probe in reachability.go, `posse gates` in cmd/posse/main.go). A
	// built-in's declaration IS overlayable from runtimes/<name>.yaml since
	// ranger-base-otoq8 (state_dir names where THIS box's CLI keeps its
	// state, which is an instance fact by ADR 0021 D1), so ~/.claude can be
	// moved — or, with an explicit empty list, removed — from a claude
	// launch by configuration. Loudly: that file is the promoted config
	// root (ADR 0039 D2), no session can write it, and `runtime check`
	// credits the key to it by name. The failure it buys is the visible
	// one D4 already named — a CLI re-running its first-run flow — not a
	// silent grant.
	for _, d := range stateDirs {
		add(ExpandTilde(d))
	}
	if t := os.Getenv("TMPDIR"); t != "" {
		add(t)
	}
	add("/private/tmp")
	add("/tmp")
	out = append(out, pidWritableExtras(ag, cwd)...)
	return dedupeStrings(out)
}

// SeatbeltSiblings names the grants whose atomic-write SIBLINGS a caged
// session must be able to create, and is the whole of ranger-base-cypy1.
//
// A `state_dir:` entry may name a FILE — `~/.claude.json` is one, and the
// key has said "directories (and single files)" since ADR 0012 D4. The
// writable set renders every entry as `(subpath …)`, and `subpath` is
// component-aware: `(subpath "$HOME/.claude.json")` covers that path and
// what is under it, which for a file is nothing. But claude does not WRITE
// that file. It creates `$HOME/.claude.json.tmp.<pid>.<hex>` beside it,
// holds `$HOME/.claude.json.lock`, and renames the temp into place — and
// `$HOME` is granted by nothing, deliberately. So both siblings are refused
// at the kernel, the rename never happens, and the CLI prints `Added stdio
// MCP server …`, prints `File modified: <path>`, and exits 0 with the file
// unchanged. Every in-session write from a caged session — MCP adds,
// seenNotifications, tips, history, projects[<dir>] — has been lost this
// way for as long as the tier has existed, silently.
//
// MEASURED on this box, 24h of `log show` (2026-09-02): the two sibling
// paths are the #1 and #2 denials by volume, 97 `.claude.json.lock` and 88
// `.claude.json.tmp.<pid>.<hex>`, ahead of everything else in $HOME
// combined. The lock denials ranger-base-gr3ow asked about are the SYMPTOM,
// not the cause: with the containing directory writable and the lock path
// denied last-match-wins the write still lands (3/3), so a fix aimed at the
// logged path would have fixed nothing.
//
// The namespace and not the two names, because the two names are the CLI's
// to change: `~/.claude.json.backup` is already a third one on this box,
// and a release that renames the temp scheme puts this bug back with no
// line anywhere saying so. `<p>.` + anything is exactly the scratch space a
// writer of `<p>` owns; nothing else can be spelled into it without also
// being spelled into `<p>`.
//
// Three narrowings, each one measured rather than argued:
//
//   - the SEPARATOR dot. A bare `^<p>` prefix also grants `<p>` itself and
//     `<p>Xanything`; on this box that is a `~/.codex` grant reaching
//     `~/.codexbar` and a `~/.grok` grant reaching `~/.grokbot`, which are
//     real directories belonging to other tools (sbSiblingRegex).
//   - DIRECTORY entries get nothing. `~/.claude`, `~/.codex` and `~/.grok`
//     are granted whole already and no CLI replaces a state TREE by rename,
//     so a sibling grant there would be reach with no writer behind it —
//     and it is the reach that ranger-base-9fl spent a bead removing. A
//     path that does not exist yet gets the grant: it cannot be told apart
//     from a file, the CLI is about to create it, and the alternative is a
//     grant that appears only on boxes that have run the CLI before.
//   - a base the carve-out DENIES gets nothing. The siblings sit in the
//     base's own directory, so a deny covering that directory covers them
//     too — but a deny naming the base FILE (a PID's `Edit(~/.claude.json)`,
//     ADR 0014 §3) would not, and a wall that leaves the scratch namespace
//     open is a wall with a note beside it. Dropped instead, which is the
//     tier's own fail-closed direction.
//
// The rejected alternative, recorded because it is the obvious one: point
// the caged launch at `CLAUDE_CONFIG_DIR=$HOME/.claude`, where the config
// would sit inside an already-granted DIRECTORY and every sibling name the
// CLI ever invents is covered for free. It is refused because it MOVES the
// operator's file: an uncaged `claude` reads `~/.claude.json` and a caged
// one would read another, so MCP servers, history and projects[] diverge
// between the operator's own sessions and the fleet's, and posse's own
// trust seeding writes the first path (trust.go). A grant is the change
// that leaves the operator's config where the operator put it.
func SeatbeltSiblings(stateDirs []string, carve SeatbeltCarveOut) []string {
	var out []string
	for _, d := range stateDirs {
		p := absResolve(ExpandTilde(d))
		if p == "" {
			continue
		}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			continue
		}
		// `#"…"` has no escape for a quote: rendering one ends the literal
		// and sandbox-exec refuses the whole profile, which would turn a
		// lost config write into a dead pane. Drop it — the base keeps its
		// own subpath grant and the behaviour is today's.
		if strings.Contains(p, `"`) {
			continue
		}
		if writeGranted(carve.Deny, p) {
			continue
		}
		out = append(out, p)
	}
	return dedupeStrings(out)
}

// pidWritableExtras resolves the PID's `writable:` extras for a session in
// cwd. ADR 0014 §4 promotes the key from "seatbelt extras" to allow-list
// paths at BOTH tiers, so this is one function and not two: L2 adds these
// to the profile's writable set and L4 mounts them read-write over a `:ro`
// repo, and the day the two readers disagree is the day `posse gates`
// prints a grant one tier holds and the other silently drops. Same shape as
// ranger-base-4ks made `wholeTreeWriteDeny` for the deny side.
//
// `~` expands, a relative extra joins the session dir, and the result goes
// through the same resolver as every other path in the profile so a
// symlinked spelling cannot dodge the grant in either direction. An extra
// with no session dir to join against is dropped rather than guessed at —
// the same answer `Resolve` gives a relative subtree glob.
func pidWritableExtras(ag *AgentFile, cwd string) []string {
	if ag == nil {
		return nil
	}
	var out []string
	for _, w := range ag.Writable {
		w = ExpandTilde(w)
		if !filepath.IsAbs(w) {
			if cwd == "" {
				continue
			}
			w = filepath.Join(cwd, w)
		}
		out = append(out, absResolve(w))
	}
	return dedupeStrings(out)
}

// sessionGitGrants is LinkedGitDirs narrowed to what a worktree session
// actually writes (ranger-base-m2wf, from the posture check on
// ranger-base-sipu). LinkedGitDirs names two directories, and the second is
// the COMMON git dir — the operator's main checkout's `.git`, shared with
// every other session on that repo. Granted whole, as it was, a persona
// dispatched into an isolated tree could:
//
//   - move any ref in the repo it was dispatched into, `refs/heads/main`
//     included. `git update-ref` is not `git push`, so L1's push shim never
//     sees it and L3's pre-push never fires; the launcher's own ff-merge of
//     the session branch fires no hook either (worktree.go). Un-gated, and
//     the whole point of the worktree model was that it could not happen.
//   - overwrite the shared `hooks/` slots, disarming L3 for the repo and for
//     every other worktree on it, persistently. The carve-out already denies
//     that path (sessionHooksDirs); this takes it out of the grant as well,
//     so the deny is no longer the only thing standing there.
//
// What a commit in a linked worktree really writes outside its own tree,
// MEASURED under the narrowed profile (seatbeltworktreegit_qa_test.go):
// `<common>/objects`, `<common>/logs`, and its OWN loose ref
// `<common>/refs/heads/<branch>` with the `.lock` git renames onto it.
// Everything else there — `config`, `packed-refs`, `hooks/`, other refs,
// other sessions' `worktrees/<name>` dirs — stays under the default deny.
//
// The ref is granted as that pair of subpaths rather than as the posture
// check's prescribed `(regex #"^<common>/refs/heads/<branch>")`: the pair is
// strictly narrower (the regex is a prefix match, so it would also cover a
// sibling branch whose name extends this one) and it stays inside the
// writable set, which is a []string every caller of SeatbeltWritable reads
// as subpaths. A detached HEAD has no branch and gets neither entry —
// a commit there moves only the per-worktree HEAD, which is inside the
// per-worktree dir this still grants whole.
//
// The ref's PARENT directories are not in this list and cannot be: they are
// granted for creation only, in a shape a subpath cannot say. See
// sessionRefDirs below — without them this list is enough to commit only
// while nothing has packed the repo's refs.
//
// LinkedGitDirs itself is deliberately unchanged: it is also what codex and
// grok are handed as `--add-dir` roots (herdrback.go), and `--add-dir` is
// directory-granular. It cannot name a single ref, so any grant that lets
// those runtimes commit exposes `refs/heads` whole. That gap is ACCEPTED
// and stated rather than closed — closing it needs an alternate object/ref
// store spliced on completion, which is a design, not a narrowing.
func sessionGitGrants(cwd string) []string {
	dirs := LinkedGitDirs(cwd)
	if len(dirs) != 2 {
		return dirs // main checkout: cwd's own grant already covers .git
	}
	own, common := dirs[0], dirs[1]
	out := []string{own, filepath.Join(common, "objects"), filepath.Join(common, "logs")}
	if b := repoBranch(cwd); b != "" {
		ref := filepath.Join(common, "refs", "heads", b)
		out = append(out, ref, ref+".lock")
	}
	return out
}

// sessionRefDirs names the directories git must CREATE to write that ref,
// and is the create-only half of the same grant (ranger-base-uuze).
//
// The leaf grant above is a file two directories deep: the fleet's branches
// are `posse/<session>`, so the loose ref is `refs/heads/posse/<session>`
// and git has to `mkdir refs/heads/posse` before it can take the lock.
// That directory is not the ref and no grant named it. While it happens to
// exist a commit works — which is why the narrowing shipped green — but
// `git pack-refs --all` deletes the loose refs and then prunes the emptied
// directory, and `git gc` packs refs by default and runs itself at
// gc.auto. One gc leaves EVERY live session on the repo unable to commit,
// at `fatal: cannot lock ref 'HEAD': unable to create directory for …`,
// with the commit lost; a session cannot repair it from inside, because
// making that directory is the write it is refused.
//
// MEASURED three ways on one fixture, varying only the directory
// (2026-08-30, darwin 25.4.0), with `refs/heads` itself unwritable so only
// this create is in question: loose ref present and the directory there —
// commit lands; refs packed and the directory pruned — the fatal above,
// nothing committed; refs packed and the directory recreated by hand —
// commit lands. So the missing write is the mkdir and nothing else.
//
// Granted as `file-write-create` on the directory rather than as another
// writable subpath, because a subpath would hand back `refs/heads/posse`
// whole — every other session's branch ref, which is most of what
// ranger-base-m2wf narrowed away. Creating the path is allowed; writing
// anything inside it is not, and the leaf pair above is what makes the
// session's own ref writable. `refs/heads` itself is never named: git
// leaves it in place across a pack (measured), and a create grant on it
// would let a session cut a top-level branch.
//
// Every ancestor between `refs/heads` and the leaf, not just the first,
// because git creates them all and a branch may carry more than one slash.
// A detached HEAD and a slashless branch get nothing — there is no
// intermediate directory to make.
func sessionRefDirs(cwd string) []string {
	dirs := LinkedGitDirs(cwd)
	if len(dirs) != 2 {
		return nil
	}
	b := repoBranch(cwd)
	if b == "" {
		return nil
	}
	base := filepath.Join(dirs[1], "refs", "heads")
	parts := strings.Split(b, "/")
	var out []string
	d := base
	for _, part := range parts[:len(parts)-1] {
		d = filepath.Join(d, part)
		if !underDir(base, d) || absResolve(d) == absResolve(base) {
			return nil // a name that climbs out is not a ref this grants
		}
		out = append(out, absResolve(d))
	}
	return out
}

// packed-refs.lock is DECLARED, not granted (ranger-base-msex, second
// finding on ranger-base-uuze). Every commit under the narrowed grant —
// packed or not, first commit or fifth — prints
//
//	error: Unable to create '<common>/packed-refs.lock': Operation not permitted
//
// on stderr and then succeeds. MEASURED (2026-08-30, darwin 25.4.0): it is
// unconditional, not a packing effect — reproduced on a fresh unpacked
// fixture's very first commit, where the session's ref has never been
// packed at all. Git's ref-transaction takes this lock speculatively on
// every ref update to check the packed backend, needs nothing from it here
// (the session's ref lives loose), and falls back cleanly when refused —
// which is why the commit still lands.
//
// The tempting fix is a createOnly grant beside sessionRefDirs, the same
// shape as the ref's parent directory. MEASURED and REJECTED: create-only
// buys the create but not the delete, and git's own cleanup is an unlink of
// that same lock file once it decides packed-refs needs no change. Refused,
// that unlink leaves the lock FILE ITSELF behind in the shared common dir —
// unlike the refusal it replaces, a stray lock is not self-healing, and it
// is not symmetric either. A session's own later commits are UNAFFECTED:
// each one retries the same create, finds the file already there, prints a
// scarier line — `error: … packed-refs.lock: File exists … Another git
// process seems to be running … remove the file manually to continue` — and
// still lands, exit 0, because an ordinary commit's ref update never
// actually needs that lock. What the stray file DOES break, hard, is
// anything that does need it: the operator's own unsandboxed `git gc` and
// `git pack-refs` on the shared repo both die at rc=128 with the identical
// message, for as long as the file sits there — which is until a human
// notices and removes it by hand, since no session may write there to
// unstick it either. Reproduced with a hand-planted lock file, no sandbox
// involved, so this is git's own locking discipline, not a profile quirk.
//
// So the grant trades a benign, self-repairing stderr line for a silent,
// human-interrupt-shaped landmine in shared state — worse than what it
// silences, and worse in a way that would not surface until whoever runs
// gc next. No grant is added; the stderr line stays.
// TestQAWorktreeCommitLeavesNoStrayPackedRefsLock pins the accepted
// behaviour and TestQAPackedRefsLockCreateGrantIsUnsafe pins why the
// tempting grant must not come back.
//
// `git gc` from inside a session dying separately at `gc.pid.lock:
// Operation not permitted` is not this: a session should not gc the shared
// repo, and that refusal is the point, not a gap — noted here only because
// the same bead raised both and this is where the first one's answer lives.

// SeatbeltCarveOut computes the trailing block for a session: the three
// artifact classes ranger-base-6ne walked through, ENUMERATED at the
// artifact level and never at a repo root.
//
// That enumeration is the constraint, not a style: the store of record
// lives in the same tree as the constitution, and the writable set follows
// `.beads/redirect` into it so a caged persona can claim, comment and close
// (ranger-base-rhw). MEASURED on the security persona's live profile
// (2026-08-26):
// add the instance repo's `.beads` to this list and `touch` in it is refused
// — the deny beats the redirect grant exactly as it beats cwd — and the
// record stage dies with no observable, because parity grades denies and a
// cage that denies too much still prints "all gates realized". So: name
// `agents`/`state/gates`/the hook slots, never the tree they sit in, and if
// that ever changes, re-allow beadsHome(cwd) and beadsGitDirs after the
// deny.
//
//  1. The constitution — the promoted set, `envs/` and the manifest, read
//     off HomeConstitutionPaths so this list and the detector below cannot
//     drift apart. `personas/` is not in it: §5's exception is memory, and
//     memory is not law.
//
//  2. `state/gates` — every persona's L1 shims, rendered seatbelt profiles
//     and refusals logs. Rendered fresh at launch, so persistence was
//     already bounded; what was not bounded is WITHIN the session, where
//     rewriting a shim defeats L1 (the Bash(security:*) keychain tripwire
//     of ranger-base-khu, say) and erases the record that it did.
//
//  3. The L3 hook slots, asked of git rather than derived: `core.hooksPath`
//     moves them, and a worktree's live in the common git dir the profile
//     grants for commits (ranger-base-flz7 / rangerhq-b38m). Both repos a
//     session writes get it — its own, and the store of record's when a
//     redirect points there, since that repo's `.git` is granted too and
//     its prepare-commit-msg slot is what stamps the beads visibility
//     guard.
//
//     ADR 0038 adds the rest of that one rule — the persistent state that
//     tells a LATER, unsandboxed git which code to run is no session's to
//     write. `.git/config` and its `config.lock` sibling
//     (sessionGitConfigFiles), because `core.hooksPath` in it moves the
//     slot the line above denies and `fsmonitor`/`filter.*.clean`/an alias
//     are commands in their own right; and a linked worktree's identity
//     chain (sessionWorktreeIdentityFiles), because those files select
//     WHICH config and hooks that git reads. Not the transient
//     `git -c core.hooksPath=` form: no file is written, so no file deny
//     reaches it, and ADR 0038 item 3 names it a residual rather than
//     letting anyone read this block as "the redirect is closed".
//
//  4. ADR 0014 §3 — the PID's OWN path-scoped denies. `deny:
//     [Edit(docs/adr/**)]` is a subtree file-write deny (ADR 0014 §1), and
//     this block is the only place L2 can say it: the rule names a
//     directory INSIDE cwd, and cwd is granted whole to any PID that does
//     not deny Edit/Write bare, so nothing in the allow block can express
//     it. `parity.go` has claimed "L2 trailing deny (subpath …)" since
//     ranger-base-4ks; until this list existed, that claim was a row about
//     a profile line nobody rendered.
//
//     Only Subtree rules join it. A bare spelling (`Edit(**)`) is the
//     whole-tree rule and is realized by SeatbeltWritable omitting cwd, not
//     by a subpath; a file filter (`Edit(**/*.md)`) is unrealized at every
//     tier, and emitting a directory deny for it would be the wall claiming
//     a rule it does not hold. Both fall out of Resolve returning "".
//
//     `writable:` extras overlapping one of these lose, and lose HERE
//     rather than by arithmetic on the allow block: the deny is below the
//     grant, so last-match-wins is what makes deny-wins (ADR 0001) true at
//     this tier. `posse agent check` warns; the profile just refuses.
//
//  5. Known credential-store literals (ranger-base-hw18, ADR 0019 D2's
//     third store) — never a subtree, a file-read* deny on the file
//     itself. See credentialReadDenyLiterals for the runtime-aware,
//     GOOS-shaped reasoning; stateDirs is the launching runtime's own
//     state_dir declaration (ADR 0012 D4), the same value RenderSeatbelt
//     already carries, threaded through here so a runtime can still read
//     its own credential.
//
// writable is the allow block this block will follow: the seal needs to
// know which ancestors a grant made renamable.
func (a *App) SeatbeltCarveOut(ag *AgentFile, cwd, gatesDir string, writable []string, stateDirs ...string) SeatbeltCarveOut {
	var c SeatbeltCarveOut
	add := func(dst *[]string, p string) {
		if p != "" {
			*dst = append(*dst, absResolve(p))
		}
	}
	if a.Home != "" {
		for _, p := range a.HomeConstitutionPaths() {
			add(&c.Deny, p)
		}
	}
	if a.StateDir != "" {
		add(&c.Deny, filepath.Join(a.StateDir, "gates"))
	}
	for _, h := range sessionHooksDirs(cwd) {
		add(&c.Deny, h)
	}
	// ADR 0038: the file that can move that hooks dir, and the chain that
	// selects which config file a later git reads at all.
	for _, p := range sessionGitConfigFiles(cwd) {
		add(&c.Deny, p)
	}
	for _, p := range sessionWorktreeIdentityFiles(cwd) {
		add(&c.Deny, p)
	}
	for _, d := range pidDeniedSubtrees(ag, cwd) {
		add(&c.Deny, d.Path)
	}
	c.Deny = dedupeStrings(c.Deny)
	// The session's own audit trail, by literal so no new file joins it.
	// Both are already inside a grant (the gates dir), so this re-allows
	// rather than widens.
	if gatesDir != "" {
		for _, f := range []string{"refusals.log", "shell.log"} {
			add(&c.Keep, filepath.Join(gatesDir, f))
		}
	}
	c.Seal = renameSeal(c.Deny, writable)
	for _, p := range credentialReadDenyLiterals(runtime.GOOS, stateDirs) {
		add(&c.DenyRead, p)
	}
	c.DenyRead = dedupeStrings(c.DenyRead)
	return c
}

// credentialReadDenyLiterals names the file-read denies ranger-base-hw18
// asks for: known credential-store files a caged session should never be
// able to read (ADR 0019 D2 item 3), minus whichever one belongs to the
// runtime THIS session is launching on — stateDirs is that runtime's own
// state_dir declaration (rt.StateDirs, ADR 0012 D4), the same value
// RenderSeatbelt already threads through, matched by the same literal
// spelling builtinRuntimes declares it in. A runtime denied its own
// credential cannot authenticate itself; every other runtime's credential
// file is never this session's business, ADR 0019's own words for it.
//
// `~/.claude/.credentials.json` is the one exception denied even for a
// claude-launched session, and only on darwin: D2 names it a recurring
// UNOWNED byproduct there, never the store of record (the keychain is,
// D2 store 1), and a caged claude session authenticates with
// CLAUDE_CODE_OAUTH_TOKEN when it needs a credential injected at all
// (cageCredential, ADR 0002 §4) rather than by reading this file — so
// denying its read costs claude nothing on darwin. That is backwards on
// any other platform: D2 names the same path the store of record there,
// so denying it would strand a future linux session's own credential
// behind a wall posse itself put up — goos is a parameter, not
// `runtime.GOOS` read here directly, so the branch a linux box would take
// is provable from a darwin one too (credential.go's meterStore made the
// same call, for the same reason).
func credentialReadDenyLiterals(goos string, stateDirs []string) []string {
	own := func(dir string) bool {
		for _, d := range stateDirs {
			if d == dir {
				return true
			}
		}
		return false
	}
	var out []string
	if goos == "darwin" {
		out = append(out, ExpandTilde("~/.claude/.credentials.json"))
	}
	if !own("~/.codex") {
		out = append(out, ExpandTilde("~/.codex/auth.json"))
	}
	if !own("~/.grok") {
		out = append(out, ExpandTilde("~/.grok/auth.json"))
	}
	return out
}

// sessionHooksDirs names where git dispatches hooks for the repos a session
// can write: cwd's, and the store of record's when a redirect puts it in
// another repo. Asked of git (hooksDir), because a derived path is a
// statement about a file and not about git's behavior.
func sessionHooksDirs(cwd string) []string {
	if cwd == "" {
		return nil
	}
	var out []string
	if h, err := hooksDir(cwd); err == nil {
		out = append(out, h)
	}
	if home := beadsHome(cwd); home != "" && !underDir(cwd, home) {
		if h, err := hooksDir(filepath.Dir(home)); err == nil {
			out = append(out, h)
		}
	}
	return dedupeStrings(out)
}

// sessionGitConfigFiles is the other half of that deny (ADR 0038 decision
// 1). The hooks deny alone was UNSOUND — ADR 0023 non-goal 3 said so and
// left it open — because `core.hooksPath` moves the slot: plant a value in
// `.git/config` pointing at a directory the session may write, and the
// denied slot is simply not the one git dispatches from any more. And
// hooksPath is only the nearest spelling; `core.fsmonitor`, a
// `filter.*.clean`, an alias — each is a command a LATER, UNSANDBOXED git
// runs, whether that is the operator's daily git in the checkout, the next
// launch's L3 probe, or the launcher's own `git -C <worktree> rebase` at
// land time (worktree.go). Plain `git config core.hooksPath …` is not a bd
// verb, so no PID denies that spelling: where `.git` is writable today, any
// session can plant the persistent redirect.
//
// So: the same two repos sessionHooksDirs walks — cwd's, and the store of
// record's when a redirect puts it in another repo whose `.git` is granted
// too — and the same doctrine, asked of git (`gitPath`) rather than joined
// onto a git dir, because in a linked worktree the config git reads is the
// COMMON one and a derived path would have said otherwise.
//
// `config.lock` joins it as the SIBLING of the answered path, not as
// another question put to git: the lock git takes is the lockfile beside
// whatever config file it resolved, so the sibling is right by construction
// at any git version. Denying it is what makes the refusal land at lock
// creation — `git config` fails before it has written anything, so there is
// no half-written config and no stray `config.lock` left in shared state
// for the operator's next `git config` to trip over. That is the
// packed-refs.lock discipline (ranger-base-msex) applied before the fact
// rather than after.
//
// The measured cost of all of it is nothing (ranger-base-j5s0's table): the
// only in-cage-reachable config writers are bd verbs already denied at L1
// crew-wide, and posse's own `recordBead` stamping of `branch.*.posseBase`
// runs in the UNSANDBOXED launcher, which no profile touches.
func sessionGitConfigFiles(cwd string) []string {
	if cwd == "" {
		return nil
	}
	var out []string
	add := func(dir string) {
		c, err := gitPath(dir, "config")
		if err != nil {
			return
		}
		out = append(out, c, c+".lock")
	}
	add(cwd)
	if home := beadsHome(cwd); home != "" && !underDir(cwd, home) {
		add(filepath.Dir(home))
	}
	return dedupeStrings(out)
}

// sessionWorktreeIdentityFiles names the files that select WHICH config and
// hooks a later git reads for this session's tree (ADR 0038 decision 2).
// Denying the config file above is only worth what the chain pointing at it
// is worth: `<worktree>/.git` is a one-line pointer at the per-worktree git
// dir, and `gitdir`, `commondir` and `config.worktree` inside that dir say
// where the tree is, which common dir it belongs to, and what extra config
// applies. The per-worktree git dir is granted WHOLE (sessionGitGrants — it
// holds the index and HEAD a commit writes), so all three are writable
// today, and `<worktree>/.git` sits in cwd, which is granted whole too.
//
// The reachable escape is not hypothetical: worktree.go runs
// `git -C <worktree> rebase` at land time, UNSANDBOXED, inside the tree the
// session just had. Point `commondir` at a directory the session may write
// and that git reads a config and a hooks dir of the session's choosing —
// the config deny walked around for exactly the git that matters most.
//
// Cost was ASSUMED zero rather than measured (these are written once, by
// `git worktree add`, and rewritten only by `git worktree move`/`repair`,
// which no session legitimately runs), so ADR 0038 item 2 asks for
// measurement by execution and for any literal a legitimate writer turns
// out to need to be dropped and RECORDED. MEASURED in
// seatbeltgitidentity_qa_test.go, and measured without a sandbox so the
// reading holds in a caged session too: a whole session's life — add,
// commit, checkout, status, rev-parse, the store of record's commit, and
// the launcher's own land-time rebase in the tree — leaves all four files
// byte-, inode- and mtime-identical, with a wrong arm showing the
// instrument does see a write. Nothing was dropped. The sandbox arms that
// grade the WALL are a separate question and skip inside a caged session
// (ranger-base-xjw9), which is why the cost half does not depend on them.
//
// Only a linked worktree has any of this. A main checkout's `.git` is a
// directory inside cwd with no pointer file and no `gitdir`/`commondir`,
// and its `config.worktree` is inert unless `extensions.worktreeConfig` is
// set — which is a write to the config file this same carve-out denies.
//
// The store of record's own identity chain is NOT here, deliberately: ADR
// 0038 item 2 is about the tree the session was dispatched into, and posse
// only ever makes the store a main checkout (`bd worktree create` writes
// the redirect from the worktree INTO the store, never the other way). A
// store that was itself a linked worktree would need this treatment too;
// that shape does not exist, and inventing a grant-shaped answer for it
// here would be a wall nobody has measured.
func sessionWorktreeIdentityFiles(cwd string) []string {
	dirs := LinkedGitDirs(cwd)
	if len(dirs) != 2 {
		return nil
	}
	var out []string
	// The pointer FILE lives at the worktree top level, which is cwd for a
	// dispatched session but need not be for `posse gates <persona>` run
	// from a subdirectory — so ask git rather than joining onto cwd, the
	// same reason gitPath exists.
	if top := worktreeTop(cwd); top != "" {
		out = append(out, filepath.Join(top, ".git"))
	}
	for _, n := range []string{"gitdir", "commondir", "config.worktree"} {
		out = append(out, filepath.Join(dirs[0], n))
	}
	return dedupeStrings(out)
}

// worktreeTop is the working tree's root as git reports it. "" when git
// cannot answer — a bare repo, or no repo at all — and the caller then
// names no pointer file rather than guessing at one.
func worktreeTop(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	top := strings.TrimSpace(string(out))
	if top == "" {
		return ""
	}
	if !filepath.IsAbs(top) {
		top = filepath.Join(dir, top)
	}
	return top
}

// pidDeniedSubtrees resolves the PID's path-scoped write denies for a
// session in cwd — ADR 0014 §3's list, in the order the PID wrote them.
//
// Resolve does the deciding and this does not second-guess it: `~` expands,
// a relative glob joins the session dir, the result goes through the same
// resolver as every other path in the profile (so a symlinked spelling
// cannot dodge the wall in either direction), and everything that is not a
// subtree — the bare spellings, the file filters — comes back "" and is
// dropped. What is left is exactly the set parity.go's `L2 trailing deny
// (subpath …)` row names.
//
// Nothing here asks whether the path is inside a grant. A deny outside one
// is already covered by the profile's default deny and costs a line; a deny
// INSIDE one is the whole point. The one thing the caller must not do is
// put these anywhere but the trailing block: deny-before-allow leaks
// (MEASURED, ADR 0014 §3).
func pidDeniedSubtrees(ag *AgentFile, cwd string) []pidDeny {
	if ag == nil {
		return nil
	}
	var out []pidDeny
	for _, d := range pathScopedWrites(ag.Deny) {
		if p := d.Resolve(cwd); p != "" {
			out = append(out, pidDeny{Rule: d.Rule, Path: p})
		}
	}
	return out
}

// pidDeny is one such rule beside the directory it resolved to. The pair
// travels together because `posse gates` prints both: a resolved path with
// no rule beside it is a line an operator cannot check against the PID.
type pidDeny struct {
	Rule string // the rule as the PID wrote it, e.g. "Edit(docs/adr/**)"
	Path string // the resolved directory the profile denies
}

// renameSeal names the directories that must be denied as LITERALS so a
// rename cannot carry a denied tree out from under its own deny. A subpath
// deny is a statement about a PATH: `mv rhq rhq2` is a write on `rhq`,
// which no deny below it names, and `rhq2/agents` is then writable — the
// whole carve-out walked around in one command, silently and reversibly
// (measured, and refused once the literal is there).
//
// It walks up from each denied path while BOTH the directory and its own
// parent are granted, because a rename needs write on the source and on the
// destination beside it: the first ancestor whose parent no grant covers
// cannot be renamed anywhere, and the walk stops there rather than emitting
// denies for every directory up to /. In the ordinary session — cwd is a
// project, the home is elsewhere — that is one entry or none; it is the
// session dispatched INTO the constitution repo that grows the list, which
// is the session this bead is about.
func renameSeal(denied, writable []string) []string {
	var out []string
	for _, d := range denied {
		for p := filepath.Dir(d); ; p = filepath.Dir(p) {
			parent := filepath.Dir(p)
			if parent == p || !writeGranted(writable, p) || !writeGranted(writable, parent) {
				break
			}
			out = append(out, p)
		}
	}
	return dedupeStrings(out)
}

// writeGranted reports whether any entry of a writable set covers p — the
// question the sandbox asks of the allow block, not string equality.
func writeGranted(writable []string, p string) bool {
	for _, w := range writable {
		if underDir(w, p) {
			return true
		}
	}
	return false
}

// HomeConstitutionPaths names what at the home is prose in force, and so
// must be in NO session's writable set (ADR 0015 §2/§3): the promoted set,
// the manifest that anchors it, and — §7 — the secret env values, which are
// not promoted but are no session's to write either. `secrets/` joins them
// on the same terms one class up (ADR 0019 D1): a store a session may not
// be handed is not a store a session may edit.
//
// The three things at the home that are deliberately NOT here: `state/`,
// which is granted above because it is what a session's runtime data IS;
// `personas/<self>`, §5's named exception; and the gates dir, which lives
// under state/.
func (a *App) HomeConstitutionPaths() []string {
	var out []string
	for _, p := range PromotedPaths {
		out = append(out, filepath.Join(a.Home, p))
	}
	return append(out, a.EnvsDir, a.SecretsDir, a.PromoteManifestPath())
}

// ConstitutionGrants reports which of those a writable set reaches. Empty
// is the property ADR 0015 §2 claims — "seatbelt never grants the home's
// constitution area" — and returning the offenders rather than a bool is
// what lets `posse gates` PRINT the answer: a wall an operator can read off
// the output is a wall that gets checked, and this one replaced a carve-out
// list nobody could audit.
//
// Containment is tested both ways round. A grant that covers the area is
// the obvious breach; a grant that lands INSIDE it (a PID's `writable:`
// naming `~/.config/posse/agents/x`) is the same breach spelled smaller.
// The sibling regexes are checked too, and by their own rule rather than
// by underDir (ranger-base-cypy1). `<base>.<suffix>` is not UNDER `<base>`
// — filepath.Rel says `../<base>.<suffix>` — so a base whose sibling
// namespace happens to contain a constitution path would pass this check
// while granting it. `~/.config/posse/config` as a `state_dir:` is the
// shape: its siblings include `config.yaml`. Prefix-matched here, which is
// exactly the reach sbSiblingRegex renders.
func (a *App) ConstitutionGrants(writable []string, siblings ...string) []string {
	var out []string
	for _, p := range a.HomeConstitutionPaths() {
		hit := false
		for _, w := range writable {
			if underDir(w, p) || underDir(p, w) {
				hit = true
				break
			}
		}
		for _, b := range siblings {
			if hit {
				break
			}
			if strings.HasPrefix(absResolve(p), absResolve(b)+".") {
				hit = true
			}
		}
		if hit {
			out = append(out, p)
		}
	}
	return out
}

// absResolve is the path the sandbox will match on: absolute, with
// symlinks resolved over the longest existing prefix.
func absResolve(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return resolveExisting(p)
}

// underDir reports whether p is dir or inside it, compared as the sandbox
// sees them — /tmp and its /private/tmp real path are the same directory.
func underDir(dir, p string) bool {
	rel, err := filepath.Rel(absResolve(dir), absResolve(p))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// beadsGitDirs names the git directories a session writes when the store of
// record lives in another repo: bd's own `bd sync` commit takes
// index.lock in the per-worktree git dir, hooks and refs live in the
// common one, and outside a worktree those are one path. `git rev-parse`
// is the only thing that can tell them apart — a worktree's .git is a
// file, and beadsHome follows a redirect into one. <repo>/.git leads
// regardless, so a target git cannot answer for still gets the grant it
// needs.
func beadsGitDirs(home string) []string {
	root := filepath.Dir(home)
	out := []string{filepath.Join(root, ".git")}
	for _, flag := range []string{"--git-dir", "--git-common-dir"} {
		b, err := exec.Command("git", "-C", root, "rev-parse", flag).Output()
		if err != nil {
			continue
		}
		g := strings.TrimSpace(string(b))
		if g == "" {
			continue
		}
		if !filepath.IsAbs(g) {
			g = filepath.Join(root, g)
		}
		out = append(out, g)
	}
	return dedupeStrings(out)
}

// RenderSeatbelt writes the profile for a persona session and returns its
// path (RHQ_HOME/state/gates/<persona>/seatbelt.sb).
func (a *App) RenderSeatbelt(ag *AgentFile, cwd string, stateDirs ...string) (string, error) {
	gatesDir := a.GatesDir(ag.Name)
	if err := os.MkdirAll(gatesDir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(gatesDir, "seatbelt.sb")
	writable := a.SeatbeltWritable(ag, cwd, gatesDir, stateDirs...)
	carve := a.SeatbeltCarveOut(ag, cwd, gatesDir, writable, stateDirs...)
	prof := SeatbeltProfile(ag.Name, writable, SeatbeltSiblings(stateDirs, carve), carve, sessionRefDirs(cwd)...)
	return p, os.WriteFile(p, []byte(prof), 0o644)
}

// SeatbeltReport renders the profile for a launch in cwd and prints the
// writable set it grants, then ADR 0015 §2's structural claim CHECKED
// against that set rather than restated: the home holds a promoted copy of
// the constitution, and what keeps a promoted copy in force is that no
// session can write it.
//
// It prints rather than asserts because the property replaced a carve-out
// deny-list, and a deny-list's whole failure mode is that nobody can tell
// by looking whether it is still complete. This one is one line under the
// set it is a property of; `posse gates <persona>` is where an operator
// reads it (ADR 0015 verification items 5 and 6).
func (a *App) SeatbeltReport(ag *AgentFile, cwd string, out io.Writer, stateDirs ...string) error {
	prof, err := a.RenderSeatbelt(ag, cwd, stateDirs...)
	if err != nil {
		return err
	}
	gatesDir := a.GatesDir(ag.Name)
	writable := a.SeatbeltWritable(ag, cwd, gatesDir, stateDirs...)
	carve := a.SeatbeltCarveOut(ag, cwd, gatesDir, writable, stateDirs...)
	siblings := SeatbeltSiblings(stateDirs, carve)
	fmt.Fprintf(out, "  %s rendered for cwd %s (writable set below):\n", AbbrevHome(prof), AbbrevHome(cwd))
	for _, w := range writable {
		fmt.Fprintf(out, "    w %s\n", AbbrevHome(w))
	}
	// A sibling grant is a grant, and it is not a subpath — so it gets its
	// own marker rather than a "w" line an operator would read as the
	// directory being writable (ranger-base-cypy1, same reason as "+").
	for _, b := range siblings {
		fmt.Fprintf(out, "    ~ %s.* (atomic-write siblings only — the .lock and .tmp.<pid> names beside a granted FILE; %s itself is the w line above)\n",
			AbbrevHome(b), AbbrevHome(b))
	}
	// Create-only grants are grants, so an operator reads them here too —
	// under their own marker, because "w" would say the directory is
	// writable and it is not (ranger-base-uuze).
	for _, d := range sessionRefDirs(cwd) {
		fmt.Fprintf(out, "    + %s (create only — the directory git makes for the session's ref; nothing inside it is writable)\n", AbbrevHome(d))
	}
	// The carve-out under the set it takes back from, for the same reader:
	// a deny that only a profile knows about is a deny nobody checks.
	//
	// A path the PID's own rule put there is named by that rule. The two
	// lists render into one block on purpose (ADR 0014 §3), but they answer
	// different questions for an operator: posse's entries are a wall the
	// PID cannot spell, and this one is the PID's own line, printed back at
	// the tier that realizes it. Reading `Edit(docs/adr/**)` off `posse
	// gates` beside the directory it resolved to is how a relative glob
	// that joined the wrong session dir gets caught before a launch.
	// Every rule that resolved to the path, not the last one: ADR 0014 §1's
	// union is normally written as all three tool names over one directory,
	// and printing one of them would tell an operator that deleting it
	// re-opens the tree.
	byRule := map[string][]string{}
	for _, d := range pidDeniedSubtrees(ag, cwd) {
		byRule[d.Path] = append(byRule[d.Path], d.Rule)
	}
	for _, p := range carve.Deny {
		if r := byRule[p]; len(r) > 0 {
			fmt.Fprintf(out, "    x %s (trailing deny — the PID's %s; ADR 0014 §3)\n", AbbrevHome(p), strings.Join(r, ", "))
			continue
		}
		fmt.Fprintf(out, "    x %s (trailing deny — beats every grant above; ranger-base-h15)\n", AbbrevHome(p))
	}
	for _, p := range carve.Seal {
		fmt.Fprintf(out, "    x %s (rename seal only — writes inside it are unaffected)\n", AbbrevHome(p))
	}
	for _, p := range carve.Keep {
		fmt.Fprintf(out, "    w %s (re-allowed after the deny: the session's own audit trail)\n", AbbrevHome(p))
	}
	for _, p := range carve.DenyRead {
		fmt.Fprintf(out, "    r %s (file-read* deny — credential-store literal; ranger-base-hw18, ADR 0019 D2)\n", AbbrevHome(p))
	}
	// A grant that reaches the constitution is still named, deny or no
	// deny: the carve-out is a wall, not a licence to grant it. What the
	// deny changes is the verdict — a grant it covers is refused at the
	// kernel, so it is a PID to fix rather than a hole to close tonight.
	if bad := a.ConstitutionGrants(writable, siblings...); len(bad) > 0 {
		open := false
		for _, p := range bad {
			if writeGranted(carve.Deny, p) {
				fmt.Fprintf(out, "    ✗ GRANT REACHES THE CONSTITUTION: %s — refused by the trailing deny below it (ADR 0015 §2, ranger-base-h15); fix the PID\n", AbbrevHome(p))
				continue
			}
			fmt.Fprintf(out, "    ✗ GRANT REACHES THE CONSTITUTION: %s (ADR 0015 §2)\n", AbbrevHome(p))
			open = true
		}
		// Either way the all-clear below would be a lie — it says the
		// constitution is in no grant, and it is in one.
		if !open {
			fmt.Fprintf(out, "    every grant above that reaches the constitution is taken back by the deny; nothing at %s is writable — ADR 0015 §2/§7\n", AbbrevHome(a.Home))
		}
		return nil
	}
	fmt.Fprintf(out, "    constitution at %s (agents/, config.yaml, recipes/, skills/, envs/, %s): in no grant above — ADR 0015 §2/§7\n",
		AbbrevHome(a.Home), PromoteManifestFile)
	fmt.Fprintf(out, "    memory %s is granted and no other persona's is — ADR 0015 §5\n", AbbrevHome(ag.MemoryDir))
	return nil
}

// SeatbeltPrefix is typed between the PATH assignment and the runtime
// command: the pane shell expands "$(cat {file})" first, then execs
// sandbox-exec, which execs the runtime inside the profile.
func SeatbeltPrefix(profile string) string {
	return "sandbox-exec -f " + shellQuote(profile) + " "
}

// resolveExisting resolves symlinks on the longest existing prefix of p
// and re-joins the rest: sandbox-exec matches real paths (/private/tmp,
// /private/var), and a path that does not exist yet (a fresh .git,
// .beads) must still land inside the allowed subtree.
func resolveExisting(p string) string {
	rest := []string{}
	cur := p
	for {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				real = filepath.Join(real, rest[i])
			}
			return real
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = append(rest, filepath.Base(cur))
		cur = parent
	}
}
