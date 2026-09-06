package posse

// The herdr backend: posse session semantics over herdr workspaces.
//
//   posse session        →  herdr workspace (label = session name)
//   session dir        →  workspace cwd
//   env sets           →  per-workspace env injection (workspace create --env)
//   command / persona  →  typed into the root pane's shell (pane run)
//   active / status    →  herdr's live agent state: working | blocked | idle
//
// Everything posse knows about a session (emoji, env-set names, persona,
// and which workspace/pane herdr gave it) lives in a flat-YAML meta file
// under state/herdr/ — never in the multiplexer, so herdr stays swappable.
// Workspaces created outside posse still show up in listings (no meta file,
// fallback emoji); meta files whose workspace died are pruned on read.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type HerdrBackend struct {
	App  *App
	H    Herdr
	Warn io.Writer // where degraded-launch notices go (nil = stderr)
	// ClaudeConfig is the claude config the launch seeds directory trust
	// into (trust.go, rangerhq-w4uf) — the ONE file a launch writes that
	// lies outside RHQ_HOME and the session dir, which is why it is a field
	// and not a resolution done down in the seed. NewHerdrBackend fills it
	// with the operator's real one; a backend built without it (every test
	// backend) seeds nothing, so a test can never leave a temp dir behind in
	// the operator's ~/.claude.json. The empty case is a no-op, deliberately
	// not a fallback: a launch that silently writes a file nobody named is
	// worse than one that hits a dialog somebody can see.
	ClaudeConfig string
	// Bd is the beads runner the reap guard reads the store of record with
	// (ADR 0013 §4). Zero value = the ambient binary, resolved on use.
	Bd Bd
	// ClaudeHistory is claude's submitted-prompt log — the store of record
	// for whether a composer is holding a prompt or echoing one already
	// sent (sentline.go, ranger-base-2hvtv). Same discipline as
	// ClaudeConfig above and for the same reason: NewHerdrBackend fills it
	// with the operator's real one, a backend built without it reads NO
	// store, and the empty case is a no-op rather than a fallback — a test
	// must never read the operator's own prompt history, and a reading
	// nobody named is worse than the reading this bead started from.
	ClaudeHistory string
	// PaneModes makes Sessions() read each session's pane for the permission
	// mode it is actually in (permissionmode.go). Off by default because it
	// costs one herdr call per readable session and Sessions() is also the
	// cockpit's per-tick refresh; the surfaces that RENDER the mode — `posse
	// list` and `posse gates <persona>` — turn it on. Off, the field stays
	// PaneModeUnread, which every renderer shows as an unknown rather than as
	// a blank.
	PaneModes bool
}

func (b *HerdrBackend) warn(format string, args ...any) {
	fmt.Fprintf(b.warnWriter(), format, args...)
}

// warnWriter is the same stream warn writes to, for the callees that take
// an io.Writer to report a config typo on (App.ModelProbeTTL).
func (b *HerdrBackend) warnWriter() io.Writer {
	if b.Warn == nil {
		return os.Stderr
	}
	return b.Warn
}

type NewSessionOpts struct {
	Name    string
	Dir     string   // "" → config default_dir → $HOME
	Cmd     string   // typed into the root pane's shell, not wrapped
	Emoji   string   // "" → emoji map
	Envs    []string // "" list → config default_env if that file exists
	Agent   string   // persona name; its command wins over Cmd
	Runtime string   // launch profile override (CLI --runtime / recipe runtime:) — over the PID's own
	Tier    string   // model tier override (CLI --tier / dispatch) — over the PID's tier: (ADR 0003)
	// Model is an EXACT model id typed on `posse new --model` (ADR 0053).
	// It is the operator's canary: the id replaces only what {model}
	// renders, and everything else — PID, gates, skills, cage, env sets,
	// reasoning effort — is the ordinary persona launch. Accepted only with
	// Agent, an explicitly typed Runtime and an explicitly typed Tier
	// (CheckExactModel), and it makes the launch print the exact-model line
	// where an ordinary launch prints the tier availability verdict, because
	// asking the PROVIDER about THIS id is the point of the session (D3).
	//
	// "" for every other launch, and dispatch never sets it: a pass-wide
	// model experiment is a different risk boundary (D5).
	Model string
	// AllowDegraded launches even when the parity check finds gates no wall
	// layer realizes on this runtime × cage; the session is marked degraded.
	// Never set by dispatch on its own (ADR 0002 §4).
	AllowDegraded bool
	Cage          string // cage tier override (CLI --cage / dispatch) — over the PID's cage:
	// Crew marks the session as one the operator made to talk to, so
	// dispatch leaves it alone (ADR 0008). `posse new` and recipes set it;
	// dispatch's own CreateSession never does.
	Crew bool
	// ByHand says the OPERATOR asked for this one launch, at the keyboard,
	// naming what to launch: `posse new`, `posse up`, `posse recipe`, and
	// the cockpit's `d`. It is the launch's provenance, not the session's —
	// Crew is who the session BELONGS to afterwards, and the two differ in
	// both directions: `d` dispatches fleet work (Crew false) that a human
	// nonetheless typed, and a relaunch reads Crew back off a meta nobody
	// is standing at.
	//
	// The load guard in planLaunch is its only reader (ranger-base-jfe5z):
	// the guard exists so an UNATTENDED loop cannot pile work onto a box
	// that can no longer fork, and an operator typing one command has
	// already made that judgement himself. A pass, its refills and the
	// pulse leave this false and are refused exactly as before.
	ByHand bool
	// Worktree asks for the session's own git worktree, index and HEAD
	// instead of the shared checkout at Dir (rangerhq-09o2, worktree.go).
	// Dispatch sets it for every fleet launch; `posse new` and recipes do
	// not, because a crew session is the operator's own conversation in the
	// operator's own checkout (ADR 0008). A dir that is not a git repo, or a
	// repo on a detached HEAD, warns and launches in the shared checkout.
	Worktree bool
	// PromptFile is the ADR 0013 §2 argv delivery: the assembled work
	// prompt, already written to this path, appended to the rendered launch
	// line as `"$(cat <file>)"` so the CLI takes it as its first user turn.
	// Set by dispatch, and only for a runtime that declares `prompt: argv`;
	// "" is every interactive launch and every typed dispatch.
	PromptFile string
	// Bead is the id this session was dispatched to work (ADR 0013 §4). It
	// is a POINTER, never a status: the bead's own store answers whether it
	// is finished, so nothing here can disagree with it (ADR 0011). Set by
	// dispatch; "" for every interactive launch, and what the reap guard
	// reads to know there is a bead to ask about at all.
	Bead string
}

func NewHerdrBackend(a *App) *HerdrBackend {
	return &HerdrBackend{App: a, H: NewHerdr(), ClaudeConfig: ClaudeConfigFile(),
		ClaudeHistory: claudeHistoryPath(), Bd: NewBd()}
}

// ─── meta files ──────────────────────────────────────────────────────────────

type HerdrMeta struct {
	Name        string
	Workspace   string
	Pane        string // root pane at creation — where the command was typed
	Emoji       string
	Envs        string // env-set names ("a+b") — names only, never values
	Agent       string
	Runtime     string    // launch profile the persona command was rendered for (ADR 0002)
	Tier        string    // model tier it was rendered at (ADR 0003)
	Model       string    // the EXACT model id this canary session was launched on ("" = the tier's own, ADR 0053 D4)
	Dir         string    // working directory the session was created in (seatbelt re-render on relaunch)
	Repo        string    // the main checkout Dir is a session worktree of ("" = Dir IS the checkout)
	Branch      string    // the session branch the launcher merges back (rangerhq-09o2; "" = no worktree)
	Cmd         string    // raw --cmd, sessions without a persona only (persona lines are re-rendered, never replayed)
	Cage        string    // cage tier the session got (ADR 0002 §4)
	Sockets     string    // container: host sockets the PID declared and the cage mounted ("" = none)
	Degraded    string    // "; "-joined gates the wall does not realize here ("" = full parity)
	TurnFailure string    // provider refused the dispatch turn before model work began ("" = none observed)
	Bead        string    // the bead dispatch launched this session to work (ADR 0013 §4; "" = not a dispatched bead session)
	Crew        bool      // the operator's own session — dispatch treats it as if it did not exist (ADR 0008)
	Socket      string    // the herdr server this session was created against (see SocketID)
	Gen         string    // the herdr server *generation* that issued Workspace (see ServerGen); "" = unknown
	Launched    time.Time // when the persona/recipe command was last typed into Pane
	// Prompted is when a work prompt was last SENT to this session — the run
	// record's half of PromptGrace (ADR 0011 §3). It was per-process memory
	// (`Dispatcher.lastPrompt`) until this landed, so the cockpit's `d` and a
	// running pass could not see each other's prompts and both prompted one
	// bead. Written by every launcher under the launch lock (ADR 0011 §1), so
	// the read-modify-write below cannot interleave with another launcher's.
	//
	// Distinct from Launched, which is when the CLI's own command line was
	// typed: a resumed session is prompted again without being launched
	// again, and it is the prompt herdr's status lags behind.
	Prompted time.Time

	// How L3 was realized in this session's repo, and whose hooks run after
	// ours (ADR 0052 D3). "redirect" = the per-session hooks dir posse
	// rendered and the launch env aims git at, because the repo's dispatch
	// path is employer-managed and posse writes nothing there; "" = the
	// ordinary install into the repo's own .git/hooks. Recorded because the
	// dir is per-session state that is removed with the session: after the
	// kill nothing on the box can answer "was this session's wall the
	// redirect, and what did it chain into" except the record.
	HooksMode    string
	ManagedHooks string

	// Unreadable is the one field that is not IN the record: it is readMeta's
	// verdict on the bytes, true when the file was there and carried no
	// record at all. Never written, and false on a record built by hand — the
	// default has to be "this is a record", because a literal is one.
	//
	// It exists because "no workspace recorded" and "read nothing at all"
	// were one value, and only one of them means the session is gone
	// (ranger-base-82e40). A reader that cannot tell them apart answers a
	// live session's unreadable meta with `recipe` — which is deliberately
	// not withheld — and the seat reads free.
	Unreadable bool
}

// SocketID names the herdr server a session belongs to: the resolved path of
// its api socket. It is recorded in the meta so a pass talking to a
// *different* herdr — a named session, a scratch server, a socket exported
// for one command — can tell "this workspace died" apart from "I am asking
// the wrong server" (rangerhq-snd).
//
// It RESOLVES rather than reads, and that is rangerhq-y4z. This was
// os.Getenv("HERDR_SOCKET_PATH"), with "" documented as "herdr's default
// socket" — but herdr injects the concrete path into every pane it opens, so
// a session created inside one recorded /Users/x/.config/herdr/herdr.sock
// while `posse` from a plain terminal read "". Those name the same server and
// the guard compared them as two, which cost the default-socket board both
// halves of the meta rule at once: a genuinely dead workspace was never
// pruned (its meta immortal, a refusal printed on every listing), and
// backfillServer could never stamp one, because it will not stamp a socket
// this pass cannot name. Resolving the empty value to the path herdr would
// have used makes the two comparable, and the sockets a meta records
// comparable with each other.
//
// The two layouts herdr resolves to are ones this shop has measured rather
// than guessed: ~/.config/herdr/herdr.sock for the default server, and
// ~/.config/herdr/sessions/<name>/herdr.sock for a named session
// (rangerhq-6bg7's scratch server ran on one). A HERDR_SESSION with no socket
// path is therefore resolved, not defaulted — pointing at the default
// server's socket there would name a server posse is not talking to.
//
// "" now means only that the socket cannot be named at all (no $HOME), and
// the asymmetry is deliberate: an unnameable socket costs the comparison —
// nothing is proven ours, so nothing is deleted and no name is opened —
// while a wrongly named one would forge it. A meta recording "" is the same
// unknown one field older: written before `socket:` existed (cannotAnswerFor).
func SocketID() string {
	if p := os.Getenv("HERDR_SOCKET_PATH"); p != "" {
		// Clean and expand, because the comparison is on the string: the
		// same socket spelled ~/... in one pass and /Users/... in the next
		// is one server, and reading it as two is this bead's whole defect.
		return filepath.Clean(ExpandTilde(p))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if s := os.Getenv("HERDR_SESSION"); s != "" {
		return filepath.Join(home, ".config", "herdr", "sessions", s, "herdr.sock")
	}
	return filepath.Join(home, ".config", "herdr", "herdr.sock")
}

// EnvWorkspace is the workspace id herdr injects into the env of every pane
// it opens (measured 2026-09-04 in a dispatched pane: HERDR_WORKSPACE_ID sits
// beside HERDR_PANE_ID, HERDR_TAB_ID and HERDR_SOCKET_PATH, and its value is
// the same string that session meta records as `workspace:`).
//
// It is the one env name that tells posse WHICH SESSION it was run from, and
// that is why the self case is not read off EnvPersona. That says only which
// persona, and one persona can be sitting in more than one session at a time
// — a conversation the operator marked crew and a dispatched seat per repo —
// so a name check would refuse a relaunch typed in a sibling of the target.
const EnvWorkspace = "HERDR_WORKSPACE_ID"

// CallerRunsInside answers whether THIS posse process is running inside the
// session `m` describes: the caller's pane belongs to that session's
// workspace, on that session's herdr server.
//
// It exists because a session cannot land ITSELF (ranger-base-521). The
// landing turn waits for the target's agent to go idle, and when the caller
// is the target that never happens — it is working precisely because it is
// running the command that waits. The old answer was a full timeout and then
// generic advice; this is the question that answer was missing.
//
// BOTH halves are required and neither is enough alone. A workspace id is
// unique per server and nothing else, so an id from another herdr server
// (a named session, a scratch server) names a different workspace with the
// same string. And the socket alone says only that two processes talk to one
// server, which every session on the box does.
//
// An unnameable or unrecorded server is NOT self, deliberately — SocketID's
// own asymmetry, applied one layer up: nothing is proven ours, so nothing is
// refused. The cost is that a meta written before `socket:` existed keeps
// the OLD BEHAVIOUR ON BOTH ARMS of the refusal this answers
// (relaunch.go:125), and the two halves are not priced alike
// (ranger-base-eaq7n, from ranger-base-f34bo; the paragraph said "the old
// timeout" until afad67d had already made that the smaller half):
//
//   - the landing arm keeps the old timeout — the wait runs to its bound and
//     then offers a longer one, which is words, not loss;
//   - `--no-land` keeps a way through to closeRecorded, and that is a LOSS:
//     the caller dies inside the close call, the session is destroyed, its
//     name is freed and nothing is left running to recreate it
//     (scripts/verify-self-close.sh measured exactly that).
//
// The trade stands anyway: the alternative is refusing a relaunch on a
// comparison this pass could not make, and a refusal nobody can argue with
// had better be one this pass can prove. The population is also empty — every
// live meta under ~/.config/posse/state/herdr carries `socket:` (measured
// 2026-09-05, 7 of 7) — so this is what the branch would cost, not what it
// has cost. What makes it a trade rather than a hole is that a meta only
// lands here by being older than `socket:`; anything written since names its
// server and is answered on the comparison.
//
// No generation fence (ServerGen), and that is the narrow window this
// accepts: herdr's allocator is max(live id)+1 recomputed at every server
// process start, so a DEAD id can be reissued to a stranger
// (scripts/verify-id-recycle.sh). The caller's own id cannot be — it is live
// by construction, this process is sitting in it — so the only way to a
// wrong yes is a target whose workspace is already dead and whose id this
// caller now wears. A fence would cost more than it buys: a live handoff
// moves the generation without closing a single workspace, so requiring
// m.Gen == ServerGen() would stop detecting the self case after every herdr
// update, which is exactly when a long session is being refreshed.
func CallerRunsInside(m *HerdrMeta) bool {
	if m == nil || m.Workspace == "" || m.Socket == "" {
		return false
	}
	if os.Getenv(EnvWorkspace) != m.Workspace {
		return false
	}
	return SocketID() == m.Socket
}

// ServerGen names the herdr server *generation* posse is talking to: the
// device, inode and bind time of its api socket file. herdr recreates that
// file on both a restart and a live handoff, and those are exactly the
// moments its workspace-id allocator is recomputed as max(live id)+1
// (measured, rangerhq-6bg7). So a workspace id is only comparable inside one
// generation, and this is the fence that says which one issued it. Purely
// local: one stat(2), no herdr call, on a path posse already records.
//
// THE TIME IS LOAD-BEARING, and its absence shipped as a defect in the linux
// builds (ranger-base-fjj). This fence read dev:ino alone, above a comment
// asserting that a recreated file gets a new inode. That is an APFS fact,
// not a portable one: Linux hands the just-freed inode straight back.
// Measured in a golang:1.26 container, unlink and recreate at the same path:
//
//	overlayfs (and the ext4 under it)	66:587500 -> 66:587500	recycled
//	tmpfs				95:3     -> 95:4		not recycled
//	APFS				...072   -> ...073	not recycled
//
// so on Linux a restart did not move the fence, and a fence that does not
// move fails OPEN — it reads a stranger's workspace as ours and clears the
// meta, which is the incident it was built to prevent (rangerhq-yt1p).
// Intermittently, too: inode reuse is opportunistic, so the guard worked
// until it did not.
//
// A socket file's mtime is the moment it was bound and never moves again —
// nothing writes to a socket, and herdr's own socket on a server up since
// the 10th still reads the 10th. So two generations now name the same token
// only if the inode was recycled AND both sockets were bound inside one
// filesystem timestamp tick (the kernel stamps these from a coarse clock —
// 1ms on a CONFIG_HZ=1000 runner). A false grant additionally requires a
// meta stamped with the earlier generation, so that tick would have to
// contain a whole posse pass: a herdr round trip and a meta write. It does
// not fit.
//
// mtime and not btime, deliberately. For a file nothing ever writes to they
// are the same instant, so btime discriminates no better — and it is the one
// of the two that is not always there (statx leaves STATX_BTIME unset on
// NFS, and the container above reports it on overlayfs only because ext4
// backs it). Falling back to "unknown" on those filesystems would retire the
// rename arm of notOurWorkspace for no gain in soundness.
//
// A timestamp that moves for some unrelated reason costs nothing: an
// unrecognised generation reads as "not proven ours", which every caller
// answers by doing nothing to the file. This fence's failure direction is
// refusal, never a delete.
//
// "" means the generation is unknown — the socket cannot be named, or is not
// there — and unknown is never read as a match (notOurWorkspace).
func ServerGen() string {
	// The path to stat and the name of the server are one thing since
	// rangerhq-y4z: this used to be a private resolver beside a SocketID()
	// that only read the environment, and the two disagreeing on the default
	// socket was the defect. One resolution, so the fence and the comparison
	// can never again be about different servers.
	p := SocketID()
	if p == "" {
		return ""
	}
	st, err := os.Stat(p)
	if err != nil {
		return ""
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return genToken(fmt.Sprintf("%d:%d", sys.Dev, sys.Ino), st.ModTime())
}

// genToken renders one generation from the api socket's file identity and
// the moment it was bound. It is split out from the stat so that the case
// which broke this fence — the same device and inode, a later bind — is
// pinned by a test on every platform, including the one whose filesystem
// will not reproduce it. The defect shipped because every test of it ran on
// a filesystem that hid it (ranger-base-fjj).
func genToken(file string, bound time.Time) string {
	return fmt.Sprintf("%s:%d", file, bound.UnixNano())
}

func (b *HerdrBackend) metaDir() string { return filepath.Join(b.App.StateDir, "herdr") }

// metaTempPattern names replaceMeta's half-written file. The meta dir is also
// the session NAMESPACE — metaNames reads every `*.yaml` in it as a session —
// so this must be a name no listing can mistake for one: dot-prefixed, and
// with no `.yaml` suffix. A rename removes it on the ordinary path, so what
// the rule is really for is the litter a kill or a power cut leaves between
// the create and the rename: a name, once, is a phantom seat forever.
const metaTempPattern = ".meta-*"

func (b *HerdrBackend) metaPath(name string) string {
	return filepath.Join(b.metaDir(), name+".yaml")
}

// readMeta reads one session record. The bool answers only "is there a file
// here" — a name with no meta — and is deliberately unchanged: every caller
// that reads it as "this home does not hold that session" (nameFree,
// MarkCrew's foreign case, mustNotOrphan) is asking about the FILE.
//
// Whether the file carried a RECORD is a second question, and the record
// answers it itself (HerdrMeta.Unreadable). The two are not the same, and
// reading them as one is ranger-base-82e40: a file with nothing in it read as
// a record with no workspace, which is a `recipe` — a session already gone.
//
// It is ONE read. Every field used to be its own YamlGet, and YamlGet opens
// the file, so a record was 24 separate opens: with the atomic write above
// each of them still sees a whole record, but not necessarily the SAME one,
// so a meta rewritten mid-read could be assembled half from each. One read,
// one record.
func (b *HerdrBackend) readMeta(name string) (*HerdrMeta, bool) {
	p := b.metaPath(name)
	if _, err := os.Stat(p); err != nil {
		return nil, false
	}
	// One read, and every field still names the reader it calls rather than
	// a local helper wrapping it. A gate censuses who may declare an exact
	// model by grepping this package for the reader's own name and the key
	// beside it (ADR 0053 D5, exactmodel_qa_test.go); a `get` closure hid
	// this file's one legitimate read from that scan, and the census went
	// silent rather than red — MEASURED on this bead, and the same trap for
	// any later gate written the same way.
	lines := readLines(p)
	return &HerdrMeta{
		Name: name,
		// writeMeta emits `name:` first and always, with a session name that
		// is never empty, so bytes carrying no readable name are bytes that
		// carry no record: an empty file, a file this pass may not read, or
		// one truncated before its first line landed. Asked of the file's own
		// field rather than of the argument above, which is where the name
		// came from and would say "readable" about anything.
		Unreadable:   yamlGetLines(lines, "name") == "",
		Workspace:    yamlGetLines(lines, "workspace"),
		Pane:         yamlGetLines(lines, "pane"),
		Emoji:        yamlGetLines(lines, "emoji"),
		Envs:         yamlGetLines(lines, "envs"),
		Agent:        yamlGetLines(lines, "agent"),
		Runtime:      yamlGetLines(lines, "runtime"),
		Tier:         yamlGetLines(lines, "tier"),
		Model:        yamlGetLines(lines, "model"),
		Dir:          yamlGetLines(lines, "dir"),
		Repo:         yamlGetLines(lines, "repo"),
		Branch:       yamlGetLines(lines, "branch"),
		Cmd:          yamlGetLines(lines, "cmd"),
		Cage:         yamlGetLines(lines, "cage"),
		Sockets:      yamlGetLines(lines, "sockets"),
		Degraded:     yamlGetLines(lines, "degraded"),
		TurnFailure:  yamlGetLines(lines, "turn_failure"),
		Bead:         yamlGetLines(lines, "bead"),
		HooksMode:    yamlGetLines(lines, "hooks_mode"),
		ManagedHooks: yamlGetLines(lines, "managed_hooks"),
		Crew:         yamlGetLines(lines, "crew") == "true",
		Socket:       yamlGetLines(lines, "socket"),
		Gen:          yamlGetLines(lines, "gen"),
		Launched:     parseLaunched(yamlGetLines(lines, "launched")),
		Prompted:     parseLaunched(yamlGetLines(lines, "prompted")),
	}, true
}

func parseLaunched(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{} // legacy meta: never recorded → old enough for anything
	}
	return t
}

func (b *HerdrBackend) writeMeta(m *HerdrMeta) error {
	if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
		return err
	}
	var s strings.Builder
	fmt.Fprintf(&s, "name: %s\n", m.Name)
	fmt.Fprintf(&s, "workspace: %s\n", m.Workspace)
	fmt.Fprintf(&s, "pane: %s\n", m.Pane)
	fmt.Fprintf(&s, "emoji: %s\n", m.Emoji)
	if m.Envs != "" {
		fmt.Fprintf(&s, "envs: %s\n", m.Envs)
	}
	if m.Agent != "" {
		fmt.Fprintf(&s, "agent: %s\n", m.Agent)
	}
	if m.Runtime != "" {
		fmt.Fprintf(&s, "runtime: %s\n", m.Runtime)
	}
	if m.Tier != "" {
		fmt.Fprintf(&s, "tier: %s\n", m.Tier)
	}
	// ADR 0053 D4: the store of record for an exact-model canary. Written
	// only when one was typed, so an ordinary session's record — and every
	// record written before this field existed — is byte-for-byte what it
	// was, and reads back as a session running its tier's own model.
	if m.Model != "" {
		fmt.Fprintf(&s, "model: %s\n", m.Model)
	}
	if m.Dir != "" {
		fmt.Fprintf(&s, "dir: %s\n", m.Dir)
	}
	// The run record's half of per-session worktrees (ADR 0011 §3): where
	// the work is and where it has to go. A kill or a merge reads these
	// rather than re-deriving a path from the session name, so a session
	// created before this landed — no `repo:`, no `branch:` — is correctly
	// read as one that shares the checkout and has nothing to merge.
	if m.Repo != "" {
		fmt.Fprintf(&s, "repo: %s\n", m.Repo)
	}
	if m.Branch != "" {
		fmt.Fprintf(&s, "branch: %s\n", m.Branch)
	}
	if m.Cmd != "" {
		fmt.Fprintf(&s, "cmd: %s\n", m.Cmd)
	}
	if m.Cage != "" {
		fmt.Fprintf(&s, "cage: %s\n", m.Cage)
	}
	// What the cage was opened for, recorded because it is a claim the tier
	// gives away: a caged persona holding the herdr socket can prompt or
	// close every other pane (ADR 0002 §3).
	if m.Sockets != "" {
		fmt.Fprintf(&s, "sockets: %s\n", m.Sockets)
	}
	if m.Degraded != "" {
		fmt.Fprintf(&s, "degraded: %s\n", m.Degraded)
	}
	// A live CLI can settle idle after the provider refused the turn before
	// any model ran. Keep that outcome beside the live status so a later
	// listing does not present the session as healthy.
	if m.TurnFailure != "" {
		fmt.Fprintf(&s, "turn_failure: %s\n", m.TurnFailure)
	}
	// Which bead this session was dispatched to work (ADR 0013 §4). The
	// reap guard is the only reader: without it a kill cannot tell a
	// session whose work is finished from one still holding an open bead
	// and an uncommitted tree (ranger-base-0fb).
	if m.Bead != "" {
		fmt.Fprintf(&s, "bead: %s\n", m.Bead)
	}
	// ADR 0052 D3. hooks_mode is a mode word; managed_hooks is a path, and
	// an absolute path is NOT a single token — git accepts a core.hooksPath
	// carrying a newline, a " #" or a trailing blank — so the flat-YAML
	// reader, which truncates a newline (ranger-base-ujdg) and MANGLES four
	// more shapes besides (ranger-base-m6szh), is guarded at the launch:
	// planLaunch refuses a managed path that is not one line, and then any
	// path flatScalarRoundTrip says this record would read back as something
	// else, before any record is written (ranger-base-buvq4,
	// ranger-base-m6szh). Nothing else sets it. The guard is the launch's
	// and not this writer's on purpose: refusing generically here would
	// change the outcome of launches whose repo path is pathological today,
	// which is ranger-base-kn68j's decision to make for every field at once.
	if m.HooksMode != "" {
		fmt.Fprintf(&s, "hooks_mode: %s\n", m.HooksMode)
	}
	if m.ManagedHooks != "" {
		fmt.Fprintf(&s, "managed_hooks: %s\n", m.ManagedHooks)
	}
	if m.Crew {
		fmt.Fprintf(&s, "crew: true\n")
	}
	if m.Socket != "" {
		fmt.Fprintf(&s, "socket: %s\n", m.Socket)
	}
	if m.Gen != "" {
		fmt.Fprintf(&s, "gen: %s\n", m.Gen)
	}
	if !m.Launched.IsZero() {
		fmt.Fprintf(&s, "launched: %s\n", m.Launched.UTC().Format(time.RFC3339Nano))
	}
	// When a work prompt was last sent (ADR 0011 §3). PromptGrace reads this
	// rather than a per-process map, so a pass and the cockpit refuse each
	// other's fresh prompts instead of both prompting one bead.
	if !m.Prompted.IsZero() {
		fmt.Fprintf(&s, "prompted: %s\n", m.Prompted.UTC().Format(time.RFC3339Nano))
	}
	return b.replaceMeta(m.Name, s.String())
}

// replaceMeta puts the rendered record at the meta's path in one step, so no
// reader can ever see a half-written one (ranger-base-82e40).
//
// This was os.WriteFile, which TRUNCATES and then writes. Every reader of
// these files runs in another process and takes no lock — listSessions holds
// none, and writeMeta is called on ordinary passes (backfillServer, NoteBead,
// the Prompted stamp) and not only at a launch — so the truncate is a window
// in which a LIVE session's meta reads as a record with no workspace. That is
// not a harmless empty read: listSessions classifies a meta naming no
// workspace as a `recipe`, a session already gone, and deliberately does not
// withhold it, so personaActive finds the seat in neither the rows nor the
// withheld list and reports it FREE. A fresh Run then seats a second bead
// into the live session — ADR 0028 §3's occupancy and ADR 0022's single
// writer defeated through a door ranger-base-5kiu4's fix does not cover.
//
// A rename is the whole fix and needs no lock: readers see either the old
// record or the new one, never a state between them, and that holds for every
// reader of these metas rather than for one path. The temp file is made in
// the meta dir itself — a rename is only atomic within a filesystem — and is
// dot-prefixed with no `.yaml` suffix so metaNames cannot mistake one for a
// session name while it exists, and so a crash between create and rename
// leaves litter rather than a phantom seat.
func (b *HerdrBackend) replaceMeta(name, body string) error {
	dir := b.metaDir()
	f, err := os.CreateTemp(dir, metaTempPattern)
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// CreateTemp opens at 0600; these records are world-readable like every
	// meta written before this landed. They carry env-set NAMES and never
	// values (see HerdrMeta.Envs), so the mode is continuity, not a grant.
	if err := os.Chmod(tmp, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, b.metaPath(name)); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ─── the crew marker (ADR 0008) ───────────────────────────────────────────────

// CrewTag is what a crew session wears in `posse list` and the cockpit.
const CrewTag = "👤"

// TurnFailureTag marks a live session whose provider refused its dispatch
// turn before model work began. Herdr's idle/done status remains true of the
// CLI process; this tag carries the separate work outcome.
const TurnFailureTag = "🛑turn-failed"

// NoBeadTag marks a session no bead can be read for, so that an operator
// looking at it can see WHY it is judged the way it is rather than conclude
// the reaper broke (ranger-base-kftx, option b).
//
// The sweep's population is "sessions carrying a `bead:` pointer", never
// "sessions whose NAME ends in something that looks like a bead id", and
// that has to stay true: sessionSanitizeRe folds `.` into `-`, so a session
// name is a LOSSY encoding of a bead id and can never be inverted back into
// one. A session wearing a per-bead name with no pointer is therefore
// outside the POINTER sweep permanently — nothing later can supply the id,
// because NoteBead only stamps a session dispatch resumes into, and a closed
// bead is never dispatched again. Since ranger-base-f6lk that is no longer
// the same thing as outside every sweep: such a session is reaped by its own
// arm past `reap_unpointed_after:`, on age and a provably empty tree — the
// strictest evidence any arm demands, precisely because no store can be asked
// about it. The tag is what says which arm a reader is looking at.
//
// MEASURED on the live fleet 2026-08-27: three such sessions, all launched
// within a second of each other, sat over closed beads while the sweep said
// nothing about them. At HEAD nothing can create that shape — every hand
// path (`posse new`, `posse up`/`local`, a recipe) marks the session CREW,
// which ADR 0008 keeps out of the pointer sweep anyway, and dispatch always passes
// `Bead:` — so what wears this tag is a meta written by a binary from before
// the pointer landed (4793e00, 2026-08-26). The tag is how those become
// visible instead of silent while the fleet's installed binary catches up.
const NoBeadTag = "🏷️no-bead"

// UnpointedBeadSession is what NoBeadTag marks: a persona session that is
// not the operator's, not the persona's reusable repo slot, and carries no
// bead pointer for any sweep to ask about. Crew is excluded because a crew
// session is not MISSING a pointer — ADR 0008 answers for it on its own
// account, and it is already tagged as such.
//
// Since ranger-base-f6lk this is not only the tag's predicate but the
// population of the sweep's unpointed arm, and the two must stay one
// definition: a session the listing explains one way and the sweep treats
// another is the silence kftx removed, put back.
func UnpointedBeadSession(s HerdrSession) bool {
	return !s.Crew && !s.Foreign && s.Bead == "" &&
		s.Agent != "" && s.Dir != "" && s.Name != SessionFor(s.Agent, s.Dir)
}

// EnvPersona is set in every persona session's env by CreateSession: its
// presence in *posse's own* env means posse was run by a persona, not by the
// operator.
const EnvPersona = "RHQ_PERSONA"

// EnvLaunchHome is set in every session's env (ADR 0031 §1) to the home the
// launcher created it under. Unlike RHQ_HOME it is not an address — nothing
// resolves paths through it — it is a record of origin that survives a
// persona overriding RHQ_HOME for a scratch run, which is what init (ADR
// 0031 §2) compares the target home against.
const EnvLaunchHome = "RHQ_LAUNCH_HOME"

// SetCrew marks a session as the operator's conversation — dispatch then
// treats it as if it did not exist — or releases it back to the fleet
// (ADR 0008). The mark lives in the session meta, so it dies with the
// session and a session posse did not create cannot carry one.
func (b *HerdrBackend) SetCrew(name string, crew bool) error {
	if !b.HasSession(name) {
		return Die("no such session: %s", name)
	}
	m, ok := b.readMeta(name)
	if !ok {
		return Die("%s was not created by posse (no session meta) — nothing to mark crew", name)
	}
	if m.Crew == crew {
		return nil
	}
	m.Crew = crew
	return b.writeMeta(m)
}

// CrewMarkMissed is the one sentence both operator prompt paths say when
// the mark did not land, and the reason it is a sentence rather than a
// silence (rangerhq-sk6p).
//
// The shield is the meta, not the prompt. An operator who typed into a
// session has every reason to believe ADR 0008 engaged — that is what
// prompting means everywhere else — and the only tell that it did not was
// a missing 👤 in a list they may never open. A mark that cannot be
// recorded is exactly the case where the belief is wrong, so it is the case
// that has to speak. SetCrew and cockpit `o` already Die with a reason;
// this is the same fact said where a prompt cannot fail over it.
//
// It names why rather than assuming the foreign case: a workspace with no
// meta and a meta that would not write are both "unrecorded", and an
// operator reading the line needs to know which one they have.
func CrewMarkMissed(name, why string) string {
	return fmt.Sprintf("the crew mark was NOT recorded on %s (%s) — dispatch reads ADR 0008's shield from the session meta, not from the prompt, so nothing marks this conversation as yours", name, why)
}

// MarkCrew records that the operator just started a conversation with this
// session (cockpit `p`). Best effort by design — a prompt must never fail
// over its marker — so it returns the line to show instead of an error:
// "" when the mark is recorded, CrewMarkMissed when it is not.
func (b *HerdrBackend) MarkCrew(name string) string {
	m, ok := b.readMeta(name)
	if !ok {
		// The foreign case, in ForeignKillRefusal's vocabulary: no meta is
		// not "an unmanaged row", it is the evidence the row is somebody
		// else's — and a mark this home cannot write is one the home that
		// DOES hold the meta will not read.
		return CrewMarkMissed(name, "this posse home holds no session meta for it, so it belongs to another instance or was made in herdr by hand")
	}
	if m.Crew {
		return ""
	}
	m.Crew = true
	if err := b.writeMeta(m); err != nil {
		return CrewMarkMissed(name, err.Error())
	}
	return ""
}

// MarkCrewOnOperatorPrompt is `posse prompt`'s half of the same rule: the
// operator's shell has no RHQ_PERSONA, a persona's session does. So a
// person starting a conversation marks the session crew, and a persona
// handing work to another persona (an orchestrator persona) marks nothing —
// otherwise the dispatch primitive would quietly retire the fleet.
//
// A persona's prompt returns "": nothing was meant to be recorded, so there
// is nothing the operator's mental model got wrong. Only a mark that was
// owed and missed has a line.
func (b *HerdrBackend) MarkCrewOnOperatorPrompt(name string) string {
	if os.Getenv(EnvPersona) != "" {
		return ""
	}
	return b.MarkCrew(name)
}

// MarkTurnFailure persists a provider refusal found after a dispatch turn
// settles, or clears the marker when message is empty and a later first
// answer proves work can run again. It is an outcome marker, not a liveness
// override: the CLI really is still idle and may be inspected normally.
func (b *HerdrBackend) MarkTurnFailure(name, message string) error {
	m, ok := b.readMeta(name)
	if !ok {
		return Die("no session meta for %s", name)
	}
	if m.TurnFailure == message {
		return nil
	}
	m.TurnFailure = message
	return b.writeMeta(m)
}

// metaNames is the session NAMESPACE this box holds a meta for, and the
// error is part of the answer (ranger-base-jzxrh).
//
// It used to drop it, and a meta DIRECTORY that cannot be read then
// answered exactly like one with nothing in it. Both readers turn "no
// names" into "no sessions" and dispatch turns that into a FREE seat: the
// fourth door onto one double-seating, after ranger-base-6swlr,
// ranger-base-5kiu4 and ranger-base-3yqyg — and the one under the third,
// whose repair answers out of this list.
//
// A dir that does not EXIST is deliberately not that abstention. A fresh
// install, or a state dir nothing has written yet, really does hold no
// sessions, and answering that with an error would stop every lane in a new
// shop from hiring (ranger-base-ifjgm in the safe direction, over the whole
// shop). So ErrNotExist is the empty answer it has always been, and every
// other ReadDir failure — EACCES on the dir alone, EIO on a mount that went
// away, EMFILE under fd exhaustion — is returned as one.
func (b *HerdrBackend) metaNames() ([]string, error) {
	ents, err := os.ReadDir(b.metaDir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			out = append(out, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	return out, nil
}

// ─── sessions ────────────────────────────────────────────────────────────────

type HerdrSession struct {
	Name        string
	WorkspaceID string
	PaneID      string // "" for workspaces not created by posse
	Emoji       string
	Envs        string
	Runtime     string // persona sessions: the launch profile (ADR 0002)
	Tier        string // persona sessions: the model tier (ADR 0003)
	Model       string // persona sessions: the exact model id the operator named ("" = the tier's own; ADR 0053)
	Cage        string // persona sessions: cage tier
	Sockets     string // persona sessions: host sockets the cage mounted ("" = none)
	Degraded    string // persona sessions: gates the wall does not realize ("" = full parity)
	TurnFailure string // provider refused the dispatch turn before model work began
	Bead        string // the bead dispatch launched this session to work ("" = not a dispatched bead session; ADR 0013 §4)
	Crew        bool   // the operator's own session — dispatch skips it entirely (ADR 0008)
	Dir         string // working directory (from meta; "" for foreign sessions)
	Repo        string // the checkout Dir is a session worktree of ("" = Dir IS the checkout; rangerhq-09o2)
	Agent       string
	Status      string // herdr agent state; "" when no agent detected
	Focused     bool
	Foreign     bool // exists in herdr but wasn't created by posse
	// PermissionMode is the mode this session's PANE is showing — read off
	// the live screen tail, never from the launch template, the meta or the
	// command line in the pane's own scrollback (permissionmode.go, ADR 0035
	// §3). It is three-valued per runtime and its zero value is
	// PaneModeUnread, which is not a mode and not "fine": only a listing that
	// asked for the read (HerdrBackend.PaneModes) carries a reading.
	PermissionMode PaneMode
}

// Sessions merges herdr's live workspace list with the meta files. Meta
// files pointing at dead workspaces are pruned; foreign workspaces are
// listed under their label so the cockpit shows the whole herd.
//
// It drops listSessions' withheld list, which is the right reading for
// every caller that wants "what is here": a withheld session is not here.
// A caller reasoning about what is GONE must use listSessions instead —
// see its doc.
func (b *HerdrBackend) Sessions() ([]HerdrSession, error) {
	out, _, err := b.listSessions()
	return out, err
}

// listSessions is Sessions with the one thing the listing knows about
// itself and the return value never carried: WHICH session metas it
// WITHHELD — kept on disk, deliberately left out, and warned about on
// stderr (ranger-base-6swlr).
//
// The guards below have four ways to withhold a meta whose session may be
// perfectly alive, and all four return a nil error: emptyBoard and
// cannotAnswerFor (both `kept`), `spared` — prunable() could not prove
// death — and `strangers`, a live workspace under a recycled id. Every one
// of them means "this listing cannot answer for that session", and a
// caller that reads absence-from-the-listing as DEATH is then acting on a
// refusal to answer. reconcileSeats did exactly that and released dispatch
// seats into their own live sessions: one persona, two beads.
//
// `recipes` is not withheld. A meta naming no workspace is not a session
// this listing declined to answer for; it is a session already gone, whose
// recipe was kept for `posse relaunch` (rangerhq-v52t).
//
// It is the meta NAMES, which is the same namespace the listing's own rows
// carry (HerdrSession.Name) and the namespace every seat prefix is written
// in — so a caller asking about one seat can ask about one seat. That is
// what personaActive needs (ranger-base-5kiu4): the whole-listing count
// this first returned could only say "something is unreadable", which is
// an honest abstention for reconcileSeats' whole pass and far too blunt
// for a seat walk, where it would stall every lane in the shop on one
// stale meta. Callers that only want "did this listing abstain at all"
// take len().
func (b *HerdrBackend) listSessions() ([]HerdrSession, []string, error) {
	wss, err := b.H.Workspaces()
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]HerdrWorkspace{}
	for _, ws := range wss {
		byID[ws.WorkspaceID] = ws
	}

	// herdr reports workspace agent_status "unknown" even for plain shells;
	// only show a status when herdr actually detects an agent in there.
	agents, err := b.H.Agents()
	if err != nil {
		return nil, nil, err
	}
	hasAgent := map[string]bool{}
	for _, ag := range agents {
		hasAgent[ag.WorkspaceID] = true
	}
	status := func(ws HerdrWorkspace) string {
		if !hasAgent[ws.WorkspaceID] {
			return ""
		}
		return ws.AgentStatus
	}

	// Pruning is destructive and unrecoverable: the meta file is everything
	// posse knows about a session (persona, env sets, crew mark, workspace and
	// pane ids), and state/ is deliberately outside git. Two situations look
	// exactly like "every workspace died" and never are (rangerhq-snd, where
	// a pass with the real RHQ_HOME talked to a scratch herdr and deleted the
	// whole fleet's metas in one read):
	//
	//   - an empty workspace listing — a herdr that just came up, or one that
	//     never held these sessions at all;
	//   - a different herdr server — the meta was written against another
	//     socket, so this server's listing says nothing about it;
	//   - a meta with no socket recorded at all — written before the field
	//     existed, or by a binary that predates it. Nothing on disk says this
	//     server ever held that workspace, and absence of evidence that it
	//     lived here is not evidence that it died (rangerhq-8fq: without this
	//     arm the guard cannot fire for any meta written before it landed,
	//     and rangerhq-snd replays against a scratch server that holds one
	//     workspace of its own).
	//
	// All three refuse and say so on stderr; the session is left out of the
	// listing (its workspace is not here, and claiming a status would be a
	// lie) and its meta kept. A genuinely dead workspace is pruned by the
	// next read against its own server.
	//
	// Past the socket guards, absence from the listing is still only a
	// snapshot, and prunable() makes it prove death before any delete
	// (ADR 0011 §2, rangerhq-9nso).
	sock, gen := SocketID(), ServerGen()
	kept := 0
	var withheld []string   // every meta the guards below left out, by name (metaNames order: os.ReadDir sorts)
	var spared []string     // missing from the listing, but not proven dead — with why
	var strangers []string  // in the listing under an id another workspace now holds
	var recipes []string    // metas naming no workspace: the session is gone, its recipe kept
	var unreadable []string // files carrying no record at all: not gone, unanswerable
	var out []HerdrSession
	claimed := map[string]bool{} // workspace ids owned by a meta file
	// An unreadable meta dir is not an empty herd. Walking a nil slice here
	// would build a CLEAN, empty listing — no rows, nothing withheld, no
	// error — out of a directory nothing could be read from, and every
	// caller reads that as "this box holds no sessions"
	// (ranger-base-jzxrh). An error instead puts personaActive on its own
	// abstention arm, which is already the right answer.
	names, err := b.metaNames()
	if err != nil {
		return nil, nil, fmt.Errorf("read session meta dir %s: %w", b.metaDir(), err)
	}
	for _, name := range names {
		m, ok := b.readMeta(name)
		if !ok {
			continue
		}
		// A file carrying no record at all is not a recipe. It reads like
		// one — no workspace, nothing to prove dead — and that is exactly
		// the trap: `recipe` is a positive claim that the session is GONE,
		// and it is the one classification below that is not withheld, so
		// making it about a live session hands its seat away
		// (ranger-base-82e40). What can be said about these bytes is that
		// this listing cannot answer for the name, which is what withheld
		// means. The meta is kept, like every other withheld one.
		if m.Unreadable {
			unreadable = append(unreadable, name)
			withheld = append(withheld, name)
			continue
		}
		// A meta naming no workspace is not a session that might be alive
		// somewhere — it is a recipe deliberately left behind by a relaunch
		// whose recreate failed after the kill (rangerhq-v52t). It has
		// nothing to be listed as and nothing to prove dead, so it is
		// reported as what it is rather than through the guards below.
		if m.Workspace == "" {
			recipes = append(recipes, name)
			continue
		}
		ws, live := byID[m.Workspace]
		// Present in the listing is not the same as ours. herdr recycles
		// workspace ids across a server restart and a handoff, and the ids
		// it hands out again are precisely the ones stale metas hold
		// (rangerhq-6bg7) — so a listing row matching this meta's id can be
		// a stranger's workspace. Listing the session over it would put its
		// name on somebody else's pane, and every addressing path (Resolve,
		// AgentTarget, KillSession) reads this listing: a prompting pass
		// would then type into that pane, and `posse kill` would close it.
		//
		// So it is left out of the listing and its file is KEPT, exactly
		// like a spared meta: "not mine" must never mean "delete the meta"
		// (rangerhq-yt1p). The id is left unclaimed, so the workspace itself
		// is still listed — under its own meta, or as foreign.
		if live {
			if why := b.notOurWorkspace(m, ws, gen); why != "" {
				strangers = append(strangers, fmt.Sprintf("%s: %s", name, why))
				withheld = append(withheld, name)
				continue
			}
		}
		if !live {
			// An empty listing is the prune's own arm, and only the prune's:
			// refusing to DELETE costs a kept file, so it is worth paying for
			// the board it covers. The write cannot pay it — see mustNotOrphan
			// (rangerhq-7dn4). The unstamped and wrong-server arms are
			// cannotAnswerFor's, and both halves ask them (rangerhq-y4z).
			if emptyBoard(sock, len(wss)) != "" || cannotAnswerFor(m, sock) != "" {
				kept++
				withheld = append(withheld, name)
				continue
			}
			dead, why := b.prunable(m, gen)
			if !dead {
				spared = append(spared, fmt.Sprintf("%s: %s", name, why))
				withheld = append(withheld, name)
				continue
			}
			// Proven dead — by evidence gathered OUTSIDE the launcher lock,
			// which is a fact about the instant it was read and not about
			// the instant the unlink lands (rangerhq-3a5t). reclaim re-proves
			// it under the lock, where no create can be in flight.
			if why := b.reclaim(name, sock, gen); why != "" {
				spared = append(spared, fmt.Sprintf("%s: %s", name, why))
				withheld = append(withheld, name)
			}
			continue
		}
		claimed[m.Workspace] = true
		b.backfillServer(m, ws, sock, gen)
		out = append(out, HerdrSession{
			Name: name, WorkspaceID: m.Workspace, PaneID: m.Pane,
			Emoji: m.Emoji, Envs: m.Envs, Agent: m.Agent, Runtime: m.Runtime, Tier: m.Tier, Model: m.Model,
			Cage: m.Cage, Sockets: m.Sockets, Degraded: m.Degraded, TurnFailure: m.TurnFailure, Bead: m.Bead, Crew: m.Crew, Dir: m.Dir,
			Repo: m.Repo, Status: status(ws), Focused: ws.Focused,
		})
	}
	for _, ws := range wss {
		if claimed[ws.WorkspaceID] {
			continue
		}
		name := ws.Label
		if name == "" {
			name = ws.WorkspaceID
		}
		out = append(out, HerdrSession{
			Name: name, WorkspaceID: ws.WorkspaceID,
			Emoji: FallbackEmoji, Status: status(ws),
			Focused: ws.Focused, Foreign: true,
		})
	}
	if kept > 0 {
		b.warn("posse: %d session meta file(s) kept, not listed: this herdr (%s) does not hold their workspaces\n",
			kept, socketLabel(sock))
	}
	if len(spared) > 0 {
		b.warn("posse: %d session meta file(s) kept, not listed: missing from this listing but not dead (rangerhq-9nso) — %s\n",
			len(spared), strings.Join(spared, "; "))
	}
	if len(strangers) > 0 {
		b.warn("posse: %d session meta file(s) kept, not listed: another workspace holds the id they recorded, so the name would address a stranger's pane (rangerhq-yt1p) — %s\n"+
			"  repair by matching each meta's filename against the workspace `label` in `herdr workspace list --json` and rewriting workspace:/pane: to it (NOTES, handoff post-flight), or delete the meta in %s to discard the session\n",
			len(strangers), strings.Join(strangers, "; "), b.metaDir())
	}
	if len(unreadable) > 0 {
		b.warn("posse: %d session meta file(s) kept, not listed: the file is there and carries no record, so this listing cannot say whether the session is alive (ranger-base-82e40) — %s\n"+
			"  a session whose meta reads like this holds its dispatch seat until the file is repaired or deleted in %s\n",
			len(unreadable), strings.Join(unreadable, ", "), b.metaDir())
	}
	if len(recipes) > 0 {
		b.warn("posse: %d session(s) closed without a replacement, recipe kept: %s — rebuild with `posse relaunch <name>`, or delete %s to discard\n",
			len(recipes), strings.Join(recipes, ", "), b.metaDir())
	}
	b.fillPaneModes(out)
	sortHerdrSessions(out)
	return out, withheld, nil
}

// fillPaneModes reads each session's pane for the permission mode it is in.
// It runs only when the caller asked for it (see HerdrBackend.PaneModes) and
// it is the ONLY writer of HerdrSession.PermissionMode: the meta records what
// the launch asked for, which is the claim this field exists to check.
//
// WHICH reader parses a pane is posse's own measurement of that CLI's screen,
// reached through the runtime's name (permissionmode.go, paneReaderFor). That
// is ADR 0057's narrow exception to ADR 0017 §3 and nothing wider: it selects
// a DISPLAY OBSERVATION and no launch, guard or dispatch decision reads it.
// The `pane_mode:` declaration this replaced had zero external declarations in
// its one-day life (measured, ranger-base-yi2f8), and loading a runtime yaml
// per listing to learn which of three readers to call was the whole cost of
// the seam — this function no longer opens one at all.
//
// A runtime measured to render nothing (codex) and a runtime nobody has
// measured are both answered without a herdr call, and they are answered
// DIFFERENTLY: `mode:—` is a measurement, `mode:?` with a why naming the
// runtime is not. A listing never renders this column blank.
func (b *HerdrBackend) fillPaneModes(out []HerdrSession) {
	if !b.PaneModes {
		return
	}
	for i := range out {
		s := &out[i]
		switch {
		case s.Runtime == "":
			s.PermissionMode = PaneMode{Why: "this session records no runtime — posse did not launch it, so nothing says what its pane would render"}
		case !PaneModeReadsPane(s.Runtime):
			// codex's measured NEVER, or an unmeasured CLI's loud unread:
			// both are correct answers to no pane at all, which is what
			// makes skipping the read free rather than lossy.
			s.PermissionMode = ReadPaneMode(s.Runtime, "")
		case s.PaneID == "":
			s.PermissionMode = PaneMode{Why: "this session records no pane, so there is no screen to read"}
		default:
			text, err := b.H.PaneRead(s.PaneID, paneModeReadLines)
			if err != nil {
				s.PermissionMode = PaneMode{Why: fmt.Sprintf("pane read failed: %v", err)}
				break
			}
			s.PermissionMode = ReadPaneMode(s.Runtime, text)
		}
	}
}

// PruneGrace is how long a meta is immune to the inferential prune: a
// workspace created less than this ago may legitimately be missing from a
// listing another process took before it existed. Five minutes is far wider
// than the observed race (a create-to-listing window of seconds) and far
// narrower than any interval over which a workspace's death goes unnoticed
// — the next read prunes it.
const PruneGrace = 5 * time.Minute

// prunable decides whether a meta whose workspace is missing from the
// listing may be deleted. It is called only after the socket guards above
// have established that this server is the one that would know.
//
// cannotAnswerFor reports why THIS herdr server is not the one that would
// know whether m's workspace is alive, or "" when it is. A per-id query is
// only evidence about a session when the server being asked is the server
// that holds it; otherwise its truthful workspace_not_found is an answer
// about an id it never held, and no evidence at all about the session
// (rangerhq-8fq).
//
// Both halves of the meta rule ask through here — the prune before it may
// delete a meta (Sessions), and the create before it may overwrite one
// (mustNotOrphan). They used to disagree: on the identical board the delete
// kept the file and the create destroyed it one line later (rangerhq-jeu2).
// One predicate is what keeps them from drifting apart again.
//
// What the two callers DO with a refusal is not the same, and cannot be. On
// the delete side "this server cannot know" means keep the file — doing
// nothing is already safe. On the write side doing nothing means refusing
// the create, because proceeding is what destroys the record.
//
// The prune keeps ONE class this does not name, applied at its call site
// because on the write side it is not true (see emptyBoard, mustNotOrphan):
// an EMPTY workspace listing. Refusing to delete costs a kept file, which the
// next listing takes back; refusing to write costs the name.
//
// A meta recording no socket at all used to be the second such class — the
// prune refused it, the create asked about it anyway — and closing
// rangerhq-y4z is what let the two converge. While SocketID() read the
// environment, a `posse` from a plain terminal wrote AND read unstamped
// metas, so refusing here would have made every name on the default board
// unusable the moment its session died. Now every create stamps a resolved
// path, so socket: "" says only what it always meant: written before the
// field existed, by a binary that named no server. Nothing on disk says this
// server ever held that workspace, and absence of evidence that it lived here
// is not evidence that it died — nor that it is free (rangerhq-8fq,
// rangerhq-jeu2). One predicate, both halves, no arm left to drift.
func cannotAnswerFor(m *HerdrMeta, sock string) string {
	if m.Socket == "" {
		return "it records no socket, and nothing on disk says this server ever held it"
	}
	if m.Socket != sock {
		return fmt.Sprintf("the meta was written against %s and this pass is talking to %s", socketLabel(m.Socket), socketLabel(sock))
	}
	return ""
}

// emptyBoard is the prune's second own arm: this herdr listed no workspaces
// at all. It is the belt added with the socket field itself after a
// pass on a scratch server deleted eleven live sessions' metas in one read
// (rangerhq-snd) — an empty listing looks exactly like "everything died".
//
// It is NOT asked by the write, and the asymmetry is the same one the
// unstamped arm above is: what the two callers pay for a refusal.
//
//   - Its two readings are "a server that just came up" and "one that never
//     held this session". The second is what the socket comparison already
//     decides, and decides better — a meta naming THIS socket says plainly
//     that this server held it. The first does not survive its own evidence
//     either: herdr restores workspaces across a restart (measured,
//     rangerhq-snd), so a server answering on this socket with an empty board
//     is an empty board, not a server mid-re-attach.
//   - The costs are not comparable. On the delete side an empty board costs
//     a kept file, and the belt is worth that for the readings it does catch.
//     On the write side it costs the NAME: a session's name is unusable while
//     the board is empty, which is exactly the board left behind when the last
//     session on it dies, and `posse relaunch <name>` — the recovery command —
//     is refused for the whole fleet at once, since a restart empties the
//     listing for every meta simultaneously (rangerhq-7dn4).
//
// So the write asks the socket comparison and then the per-id query, which
// is the strong evidence anyway: only herdr's own workspace_not_found, from
// the server the meta names, opens a name (ADR 0011 §2).
func emptyBoard(sock string, listed int) string {
	if listed == 0 {
		return fmt.Sprintf("this herdr (%s) lists no workspaces at all — a server that just came up, or one that never held this session (rangerhq-snd)", socketLabel(sock))
	}
	return ""
}

// notOurWorkspace reports why the workspace herdr just answered with is not
// the one m recorded — "" when nothing here disproves it.
//
// This is the identity half of the two questions a per-id query conflates
// (ADR 0011's liveness-vs-identity canon, in its pid-recycling shape): that
// an id answers proves A workspace holds it, never that it is the one the
// meta recorded. herdr's allocator is max(live id)+1, recomputed from the
// live set at every server process start — a restart and a handoff both —
// so every id above the live high-water is free real estate on the far side,
// and that is exactly the set of ids stale metas hold (rangerhq-6bg7).
//
// Two anchors, and neither works alone:
//
//   - the LABEL. posse creates every workspace with --label
//     <instance tag><session name> (startPlanned, rangerhq-ouf9) and the
//     meta's filename is that name, so a workspace whose label is not one
//     this home would have written for the meta's name is not the meta's
//     workspace. Not conclusive on its own: renaming a workspace in herdr
//     breaks the label without changing whose workspace it is.
//   - the GENERATION. Within one server process an id is never re-issued
//     (measured, same probe). So when the meta's gen: is this server's, the
//     id cannot have been recycled, and a label mismatch can only be a
//     rename — the workspace is still ours, and calling it a stranger would
//     hide a live session from its own name.
//
// Across a generation boundary the two readings — renamed, or recycled to a
// stranger — leave identical evidence, and herdr offers no third anchor:
// `workspace get` carries no creation time, `api snapshot` no server pid or
// boot id, and terminal_id / the root pane's shell_pid are regenerated when
// a pane's terminal is rebuilt, so they call a legitimately restored
// workspace a stranger. The ambiguous case therefore reads as "not proven
// ours", which every caller answers by doing nothing to the file. An unknown
// generation — a meta written before the field, or a pass that cannot name
// its socket — is ambiguous in the same way and is never read as a match:
// the absent-socket arm the prune already takes, applied to the fence.
//
// An empty label is not evidence of a stranger. Positive evidence only: a
// herdr that stopped reporting labels would otherwise turn the entire fleet
// into strangers in one release, and unlabelled workspaces are not the ones
// posse's own creates hand out.
func (b *HerdrBackend) notOurWorkspace(m *HerdrMeta, ws HerdrWorkspace, gen string) string {
	if ws.Label == "" || b.App.labelWearsName(ws.Label, m.Name) {
		return ""
	}
	if gen != "" && m.Gen == gen {
		return "" // same server generation: ids are not re-issued, so this is a rename
	}
	return fmt.Sprintf("workspace %s is labelled %q, not %q, and %s — herdr re-issues workspace ids across a server restart or handoff (rangerhq-6bg7)",
		m.Workspace, ws.Label, b.App.WorkspaceLabel(m.Name), genLabel(m.Gen, gen))
}

// genLabel says how much the fence knows, in the voice of a warning line.
func genLabel(metaGen, gen string) string {
	switch {
	case metaGen == "" || gen == "":
		return "which server generation issued that id is unrecorded"
	default:
		return "the id was issued by another generation of this herdr server"
	}
}

// idEvidence asks this herdr server about the workspace id m recorded and
// says what the answer proves about m's *session*. Both halves of the meta
// rule ask through here — the prune before it may delete a meta, and the
// create before it may overwrite one — because they used to disagree on the
// identical board (rangerhq-jeu2), and one predicate is what keeps them from
// drifting apart again. What the two callers DO with a refusal is not the
// same and cannot be: on the delete side "not proven dead" means keep the
// file, on the write side it means refuse, because there proceeding is what
// destroys the record.
//
//   - dead: herdr's own workspace_not_found. A workspace never changes its
//     id while it exists — the ids that survive a restart or a handoff are
//     the live ones, unchanged (rangerhq-6bg7) — so an id nothing holds is a
//     session that is gone, in this generation and any other.
//   - not dead: why says what the answer proved instead — alive and ours,
//     alive and a stranger's, or no answer at all. Silence is never death
//     (ADR 0011 §2).
func (b *HerdrBackend) idEvidence(m *HerdrMeta, gen string) (dead bool, why string) {
	if m.Workspace == "" {
		return false, "it names no workspace, and absence of a name is not death"
	}
	ws, found, err := b.H.WorkspaceGet(m.Workspace)
	if err != nil {
		return false, fmt.Sprintf("herdr did not answer for workspace %s (%v), and silence is not evidence", m.Workspace, err)
	}
	if !found {
		return true, ""
	}
	if stranger := b.notOurWorkspace(m, ws, gen); stranger != "" {
		return false, stranger
	}
	return false, fmt.Sprintf("workspace %s is alive", m.Workspace)
}

// The listing is a snapshot, and rangerhq-9nso is what happens when a
// snapshot is read as a fact about another store: three concurrent dispatch
// passes, each holding a workspace list taken before the others' workspaces
// existed, deleted each other's fresh metas — live sessions with running
// agents lost their identity, and with it the pane a prompting pass needs.
// So a prune must prove death, never infer it (ADR 0011 §2):
//
//   - (a) a meta younger than PruneGrace is never pruned. This is the whole
//     of the observed window, and it costs nothing: dispatch stamps
//     launched: before it types the command, so the race window is covered
//     by the file's own record.
//   - (d) absence is confirmed by asking herdr for that workspace by id,
//     now. A meta with no workspace recorded, or a query that errors, or a
//     workspace that answers: all keep the file. Only herdr saying
//     workspace_not_found is death.
//
// A meta with no launched: (written before the field, or a session created
// with no command) passes (a) — nothing on disk says it is young — and
// rests entirely on (d), which is the stronger guard anyway.
//
// (d) is asked through idEvidence, the predicate the create side asks too.
// A workspace that answers keeps the file whether it is ours or a stranger
// squatting a recycled id; the reason is carried out so the listing's
// warning can say which, since only one of the two is repairable.
func (b *HerdrBackend) prunable(m *HerdrMeta, gen string) (dead bool, why string) {
	if time.Since(m.Launched) < PruneGrace {
		return false, fmt.Sprintf("its meta is younger than the %s prune grace", PruneGrace)
	}
	return b.idEvidence(m, gen)
}

// sessionGone says whether THIS herdr server can prove the session named
// here is gone, and when it cannot, why not. It coins no liveness rule: it
// is the question ADR 0011 §2's prune already answers before it destroys a
// session's record, asked about a session NAME rather than about a meta —
// because ADR 0058's retire destroys that session's worktree on the same
// evidence, and a second liveness rule written beside this one is how the
// two ends of a reclaim drift apart (rangerhq-jeu2 is that bug, inside this
// file).
//
// Every reading that is not proof of death KEEPS the session alive for the
// caller: a herdr that did not answer, an empty board, a meta written
// against another socket or carrying no record at all, a workspace that
// answers — ours OR a stranger's on a recycled id. Silence is never death.
//
// The one place it goes past the prune is a name with NO META, which is most
// of the population this exists for: a kill removes the meta and leaves the
// tree standing (rangerhq-09o2), so the trees that strand are exactly the
// ones with no record left to ask about. There is no workspace id to query,
// so the evidence is this server's own listing — no row carries the name,
// and the board is not empty. ADR 0058 records that as an ASSUMPTION and
// names what would break it (a second posse home sharing one worktree root),
// because the prune's own (d) rests on the same premise: this box's herdr is
// the one that would know.
func (b *HerdrBackend) sessionGone(name string) (dead bool, why string) {
	if name == "" {
		return false, "it has no session name to ask about"
	}
	wss, err := b.H.Workspaces()
	if err != nil {
		return false, fmt.Sprintf("herdr did not answer for the workspace list (%v), and silence is not evidence", err)
	}
	sock, gen := SocketID(), ServerGen()
	// The prune's own belt, and for the same reason: an empty listing looks
	// exactly like "everything died" and is also what a server that just
	// came up looks like (rangerhq-snd). Asked before the meta, because it
	// is a fact about the SERVER and holds whether or not a meta survives.
	if why := emptyBoard(sock, len(wss)); why != "" {
		return false, why
	}
	m, ok := b.readMeta(name)
	if !ok {
		for _, ws := range wss {
			// labelWearsName and not WorkspaceLabel, deliberately over-
			// inclusive: its doc warns that the bare-name arm can read
			// another instance's untagged namesake as ours, and here that
			// costs a standing directory, where the harm it warns about
			// (ranger-base-rcwx) was a create and a relaunch being blocked.
			// Any row wearing this name, however spelled, keeps the tree.
			if b.App.labelWearsName(ws.Label, name) {
				return false, fmt.Sprintf("workspace %s is listed under this name with no meta of its own", ws.WorkspaceID)
			}
		}
		return true, ""
	}
	if m.Unreadable {
		// listSessions' own reading of these bytes: not a session already
		// gone, but a name this pass cannot answer for (ranger-base-82e40).
		return false, "its meta carries no readable record, and unreadable is not gone"
	}
	for _, ws := range wss {
		if ws.WorkspaceID == m.Workspace {
			if stranger := b.notOurWorkspace(m, ws, gen); stranger != "" {
				return false, stranger
			}
			return false, fmt.Sprintf("workspace %s is alive", m.Workspace)
		}
	}
	if why := cannotAnswerFor(m, sock); why != "" {
		return false, why
	}
	return b.prunable(m, gen)
}

// reclaim is the unlink itself: the only line in posse that destroys a
// session's record on inference rather than on the operator's word.
//
// prunable() proving death is necessary and is not sufficient, because
// proof and act were two steps over a file another actor writes. Between
// them a create for the SAME name — a launcher's, or a `posse new` that
// took no lock — can legitimately pass mustNotOrphan (the old workspace
// really is dead) and writeMeta a fresh meta at this very path. The Remove
// then deletes the NEW record: a live session loses its identity, which is
// the rangerhq-9nso damage shape reached through the write/delete
// interleave instead of two deletes. The window is milliseconds, and
// CWE-367's own note applies — a narrow race is worse to debug, not safer.
//
// So the act is taken under the launcher lock ADR 0011 §1 already built,
// and the check is taken again INSIDE it. Both halves matter: locking the
// unlink alone would still act on evidence read before the lock, which is
// the same race one step over. Under the lock a create for this name is
// either finished (and the re-read sees its meta, which is young and names
// a live workspace, so prunable spares it) or has not begun (CreateSession
// takes the same lock, underLaunchLock).
//
// A contended lock is answered by sparing the file, never by waiting and
// never by acting: a held lock means a launcher is running, a listing must
// not block behind it (the cockpit reads this on its select loop), and the
// next quiet pass prunes. That answer also makes nesting safe by
// construction — flock is per open file description, so a pass that already
// holds the lock cannot take it again, and its own listings spare every
// meta rather than deadlocking. Nothing is lost: a kept meta is taken back
// by the next read, and the prune is the half of the meta rule whose
// refusal costs a file (prunable, mustNotOrphan).
//
// It returns why the file was kept, "" when it was pruned — or when the
// re-read finds nothing there at all, which is not a sparing: somebody else
// already did the delete this pass had proved.
func (b *HerdrBackend) reclaim(name, sock, gen string) string {
	lock, why := tryLockLaunches(b.App)
	if lock == nil {
		// The reason is carried, not fixed: a lock this pass could not even
		// open is not a launcher holding it, and a sparing that says so is
		// the difference between a quiet pass and a broken box
		// (ranger-base-zppcv). The lock is still named, because the sparing
		// is read as a lock answer whichever of the four it was.
		return "the launch lock was not taken (" + why + "), so the unlink is not taken now (ADR 0011 §1, rangerhq-3a5t) — the next quiet pass prunes it"
	}
	defer lock.Release()
	// The whole check again, on the file as it is now. The listing-shaped
	// arm (emptyBoard) is deliberately not re-asked: it is a fact about the
	// snapshot this pass holds, already answered above, and re-asking it
	// would mean a second `workspace list` inside the lock to learn nothing
	// about THIS meta. What can have changed under a concurrent create is
	// the FILE, and every arm below reads it.
	m, ok := b.readMeta(name)
	if !ok {
		return ""
	}
	if m.Workspace == "" {
		return "it now names no workspace: a relaunch left its recipe here while this pass was proving the old workspace dead"
	}
	if why := cannotAnswerFor(m, sock); why != "" {
		return why
	}
	if dead, why := b.prunable(m, gen); !dead {
		return why
	}
	os.Remove(b.metaPath(name)) // workspace died — stale meta
	return ""
}

// backfillServer records which herdr server — and which generation of it —
// a meta belongs to, the moment this pass finds its workspace live: holding
// the workspace is the proof the meta file never carried. Without it a meta
// written before `socket:` existed is refused forever (it is never pruned,
// and every listing repeats the refusal) until its session is recreated one
// at a time — so the guard above and this are one fix, not two
// (rangerhq-8fq). `gen:` is the same story one field later: every meta
// written before it exists carries none, and a restart or handoff makes even
// a stamped one stale.
//
// Only a pass that knows a concrete socket can stamp one, and since
// rangerhq-y4z that is every pass with a $HOME: SocketID() resolves the
// default server to its own path rather than to "", so a session created
// outside a herdr pane is stamped like any other and the backfill works on
// the default board — where before it could never fire, leaving those metas
// unstamped and so unprunable forever. SocketID() == "" now means the socket
// cannot be named at all, and a pass that cannot name its server must not
// claim a meta for it.
//
// The generation is stamped only on POSITIVE identity — the workspace this
// server holds under that id wears the meta's own name as its label. The
// weaker readings notOurWorkspace tolerates (an unlabelled workspace: no
// evidence either way) must not stamp, because a forged fence is worse than
// an absent one: it would make the next generation trust a recycled id.
// Best effort, like the socket half: a listing must never fail over a
// bookkeeping write.
func (b *HerdrBackend) backfillServer(m *HerdrMeta, ws HerdrWorkspace, sock, gen string) {
	stampSock := m.Socket == "" && sock != ""
	stampGen := gen != "" && m.Gen != gen && ws.Label == m.Name
	if !stampSock && !stampGen {
		return
	}
	if stampSock {
		m.Socket = sock
	}
	if stampGen {
		m.Gen = gen
	}
	_ = b.writeMeta(m)
}

// socketLabel names a socket in a refusal. "" is no longer "the default
// socket" — since rangerhq-y4z that server has a path like any other — so it
// is the one case left where nothing can be named: a pass with no $HOME, or
// (through cannotAnswerFor's mismatch arm, which "" never reaches) a meta
// from before the field.
func socketLabel(sock string) string {
	if sock == "" {
		return "a socket posse cannot name"
	}
	return sock
}

func sortHerdrSessions(s []HerdrSession) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Name < s[j-1].Name; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Resolve finds one session by name: posse-created first (meta file), then
// foreign workspaces by label. Ambiguous foreign labels are an error.
func (b *HerdrBackend) Resolve(name string) (*HerdrSession, error) {
	sessions, err := b.Sessions()
	if err != nil {
		return nil, err
	}
	var found *HerdrSession
	for i := range sessions {
		if sessions[i].Name != name {
			continue
		}
		if !sessions[i].Foreign {
			return &sessions[i], nil
		}
		if found != nil {
			return nil, Die("label '%s' matches several herdr workspaces (%s, %s) — rename one in herdr", name, found.WorkspaceID, sessions[i].WorkspaceID)
		}
		found = &sessions[i]
	}
	if found == nil {
		return nil, Die("no such session: %s", name)
	}
	return found, nil
}

func (b *HerdrBackend) HasSession(name string) bool {
	s, err := b.Resolve(name)
	return err == nil && s != nil
}

// mustNotOrphan is rangerhq-9nso's rule applied to the destructive *write*
// beside the destructive delete it hardened. HasSession answers out of
// Sessions(), and a meta whose workspace is missing from that listing
// snapshot is deliberately left OUT of the listing — spared, but invisible
// (9nso's own scope note). So a racing pass reads "no such session",
// creates a SECOND workspace under the same label, and writeMeta()s over
// the only record of the first: nothing on disk names that workspace any
// more, posse can no longer address it by name, it shows up as a foreign row,
// and the pane a prompting pass needs is gone while its agent keeps
// running. state/ is outside git, so the overwrite is exactly as
// unrecoverable as the os.Remove 9nso closed (rangerhq-cpeh).
//
// Same fact, same evidence, asked the same way: a per-id query answered
// now, never a listing. Only herdr's own workspace_not_found makes a name's
// meta free real estate. A workspace that answers is a live session and
// refuses the create. A query that errors is silence, and inferring death
// from silence is the move ADR 0011 §2 forbids on the delete side — so it
// refuses too, which is also the recoverable direction: a refused create is
// an error message, an overwrite is a lost session.
//
// The prune's 5m grace (guard (a)) is deliberately NOT applied here. It
// exists because absence from a listing is weak evidence about a young
// workspace; this asks herdr directly, so a session whose workspace really
// did die a minute ago may be recreated under its name at once, as always.
//
// The launch lock (rangerhq-tzdf, ADR 0011 §1) closes this window for two
// dispatch passes, which re-read a fresh listing inside the lock, and since
// rangerhq-3a5t for an operator's `posse new` too: CreateSession takes the
// same lock around this check and the writeMeta it authorizes, so the
// unlocked create the paragraph above used to name is gone, and so is the
// prune racing it from the other side (reclaim). It is still a
// serialization and not a guard on the write itself — the guard is here.
func (b *HerdrBackend) mustNotOrphan(name string) error {
	m, ok := b.readMeta(name)
	if !ok || m.Workspace == "" {
		return nil // no record, or one naming no workspace: nothing to orphan
	}
	// Same evidence, asked the same way — which means asked of the same
	// server. The prune reaches its per-id query only behind the socket
	// guards, and prunable()'s own comment says it is "called only after the
	// socket guards above have established that this server is the one that
	// would know". Without them here the create asked whatever herdr posse
	// happens to be pointed at now about a workspace the meta may say
	// plainly belongs to another server, and read that server's truthful
	// not_found as free real estate: on the identical board the prune kept
	// the file and the create overwrote it one line later, destroying the
	// only record of a session alive elsewhere (rangerhq-jeu2).
	//
	// The listing is not read at all, and that is the point: whether this
	// workspace is in it is the snapshot read as a fact about another store,
	// which is the whole of ADR 0011 §2. A name with no meta, or one naming
	// no workspace, has already returned above, so the ordinary create still
	// pays nothing beyond the per-id query below.
	//
	// ONE arm of the prune's is deliberately not taken here, because a
	// refusal costs the two halves different things and only this half can
	// lose a name. It used to be two.
	//
	// The unstamped meta was the other, and rangerhq-y4z closed it. While
	// SocketID() read $HERDR_SOCKET_PATH raw, `posse` from a plain terminal
	// wrote AND read metas recording "", so refusing one here would have cost
	// every name on the default board: a dead session's name could never be
	// reused without deleting its meta by hand, which is the operator's own
	// ordinary path. That was the whole reason the arm was skipped, and it
	// was skipped at the price of the one board where the create still could
	// not tell "same server" from "no idea" — a pre-field legacy meta whose
	// workspace is alive on a NON-default herdr, asked from the default one.
	// SocketID() now resolves, every create stamps a concrete path, and
	// socket: "" means only "written before the field existed". So the arm is
	// taken, inside cannotAnswerFor where both halves ask it, and the two
	// halves are one predicate again.
	//
	// An EMPTY listing is the same trade one bead later (rangerhq-7dn4), and
	// it bites harder. It only ever changes the answer when the sockets
	// MATCH — a mismatch is already refused on the line below, and every
	// unstamped/named combination is a mismatch — so the board it governs is
	// the one where the meta names this very socket, i.e. where the socket
	// evidence says this server IS the one that would know. Refusing there
	// costs the name for as long as the board is empty, which is precisely
	// the board the last session on it leaves behind, and it costs relaunch
	// fleet-wide after a herdr restart, because a restart empties the listing
	// for every meta at once. See emptyBoard for why the reading it protects
	// against ("a server that just came up") does not survive its own
	// evidence: herdr restores workspaces across a restart.
	//
	// Dropping it takes the listing call with it. Nothing else here read the
	// listing, and its failure branch was a weaker copy of the per-id query
	// below: if herdr will not answer, the query errors too, and silence on
	// the write side is already a refusal.
	sock := SocketID()
	if why := cannotAnswerFor(m, sock); why != "" {
		return Die("session '%s' has a meta naming workspace %s and %s — refusing to overwrite the only record of a session that may be alive on another herdr; the prune keeps this same file for the same reason (rangerhq-8fq). Point posse at the herdr that holds it, or remove %s by hand.", name, m.Workspace, why, b.metaPath(name))
	}
	// The per-id query, through the predicate the prune asks — including its
	// identity arm. An id that answers alive is not this session unless the
	// workspace wearing it is (rangerhq-yt1p): herdr re-issues ids across a
	// restart and a handoff, so a stranger's workspace can answer for the id
	// this meta records. That answer proves nothing about the session, and
	// "proves nothing" on the write side is a refusal — the same board where
	// the prune keeps the file (rangerhq-jeu2).
	//
	// It refuses rather than repairing the meta onto the label-matched
	// workspace. A repair is only ever right when the session is alive under
	// a different id, and a workspace does not change its id while it
	// exists; the operator's post-flight repair (NOTES) stays a deliberate,
	// visible act, and a refusal costs a retry where a wrong repair would
	// point the name at a workspace nobody proved was the session's.
	dead, why := b.idEvidence(m, ServerGen())
	if dead {
		return nil
	}
	return Die("session '%s' has a session meta and %s — refusing to overwrite the only record of a session that may still be alive (try: posse attach %s, or posse list; if it really is stale, remove %s by hand)",
		name, why, name, b.metaPath(name))
}

// nameSyntax is nameFree's first guard on its own, so a create can ask it
// before it takes the launcher lock: it reads no state, which is exactly
// what makes it safe to ask outside the serialization the other two need.
func nameSyntax(name string) error {
	if !ValidName(name) {
		return Die("bad session name '%s' (letters, digits, - and _; may not start with -)", name)
	}
	return nil
}

// nameFree is the trio of guards every create passes: a usable name, no
// live session already wearing it, and no meta this create would orphan.
// Relaunch runs it too — after its kill, which is the only moment the name
// is free — so the guards live here rather than inside CreateSession.
func (b *HerdrBackend) nameFree(name string) error {
	if err := nameSyntax(name); err != nil {
		return err
	}
	if err := b.nameNotTaken(name); err != nil {
		return err
	}
	return b.mustNotOrphan(name)
}

// nameNotTaken is the middle guard: is there already a session, or a herdr
// workspace, that this create would collide with?
//
// It asks about the LABEL this create would write, not about every string
// that answers to <name> (ranger-base-rcwx). HasSession — which this used
// to be — answers out of Resolve, and Resolve addresses a FOREIGN row by
// its displayed label, which is the whole label, tag and all. That is right
// for addressing and wrong for a create: under `instance:` a foreign
// workspace labelled bare `smoke` cannot be in the way of a create this
// home labels `work/smoke`, and refusing over it put the collision
// rangerhq-ouf9 exists to remove back on the one ordering the fleet has —
// the tagged home meeting an UNTAGGED one's bare row. It also refused the
// symmetric case in only one direction: the untagged home met `work/smoke`
// and created happily.
//
// So the question is asked as WorkspaceLabel asks it, on this home's own
// rendering, and the answers line up with what herdr will actually hold:
//
//   - one of OUR sessions under this name — the ordinary "already exists";
//   - a foreign row wearing the exact label this create would write — two
//     homes sharing a tag, or a row of ours whose meta is gone. Today that
//     one slipped through (its displayed name is `<tag>/<name>`, which is
//     not <name>, so Resolve never matched it) and herdr took two
//     workspaces under one label.
//
// The refusal names the collider by its DISPLAYED name, because that is
// what `posse list` prints and what `posse attach` resolves: for a foreign
// row under a tag the two differ from the session name, and pointing the
// operator at the bare name would resolve to nothing.
func (b *HerdrBackend) nameNotTaken(name string) error {
	// A listing this pass cannot read decides nothing here — as before,
	// when Sessions() errors the create goes on to mustNotOrphan, whose
	// per-id query is the guard on the destructive half, and to a herdr
	// call that will fail loudly on its own.
	sessions, err := b.Sessions()
	if err != nil {
		return nil
	}
	label := b.App.WorkspaceLabel(name)
	for _, s := range sessions {
		taken := (!s.Foreign && s.Name == name) || (s.Foreign && s.Name == label)
		if taken {
			return Die("session '%s' already exists (try: posse attach %s)", name, s.Name)
		}
	}
	return nil
}

// launchPlan is everything a launch resolves before herdr is touched: the
// command line to type, the environment to inject, and the meta fields that
// describe what was resolved. Splitting it out is what lets a caller about
// to destroy something prove the replacement is buildable BEFORE the
// destructive step, and then build it from the very plan it verified —
// relaunch's kill-before-recreate lost sessions to a recreate that could
// never have succeeded (rangerhq-v52t).
type launchPlan struct {
	Dir      string
	Repo     string // main checkout Dir is a session worktree of ("" = Dir is the checkout)
	Branch   string // the session branch to merge back ("" = nothing to merge)
	Cmd      string
	Emoji    string
	Envs     []string
	Vars     []EnvVar
	Runtime  string
	Tier     string
	Model    string // the exact model id this launch rendered ("" = the tier's own; ADR 0053)
	Cage     string
	Sockets  string
	Degraded string
	// ADR 0052 D3, both "" on an ordinary launch: how L3 was realized
	// ("redirect" = the per-session hooks dir the env aims git at) and the
	// employer's hooks dir it forwards into.
	HooksMode    string
	ManagedHooks string
}

// planLaunch resolves a launch without touching herdr: persona, runtime,
// tier, cage, parity, skills, seatbelt, gates, env sets, working directory.
// Every refusal a launch can raise for reasons that are knowable in advance
// is raised here. Its side effects are the renders a launch always redoes
// (gates, skills, seatbelt profiles, the memory dir) — idempotent by
// design, so planning twice costs nothing and changes nothing.
func (b *HerdrBackend) planLaunch(o NewSessionOpts) (*launchPlan, error) {
	a := b.App

	// ADR 0053 D1, first because it is pure: an exact model with a missing
	// companion, or an id posse cannot render as one token, is a refusal
	// that reads no state at all. Asking it here — above the load guard,
	// the worktree, the gates and the skills render — is what makes "refuse
	// before a workspace or session record exists" true by construction on
	// every launch path rather than on the one the flag is typed at.
	if err := CheckExactModel(o); err != nil {
		return nil, err
	}

	// The load guard (ranger-base-innx) is first, before this function's
	// renders and before anything is asserted about the home, because the
	// question it answers — can this box still fork? — decides whether the
	// rest is worth doing. Every launch path funnels through here: `posse
	// new`, a recipe, a cockpit key, dispatch, and relaunch, which plans
	// before it kills (rangerhq-v52t) and so refuses with the old session
	// still alive rather than losing it to a box that cannot start the
	// replacement.
	//
	// It applies to the FLEET, never to the crew (ranger-base-jfe5z,
	// operator ruling 2026-09-04): the guard exists so an unattended loop
	// cannot pile work onto a box that can no longer fork, and it has no
	// standing to overrule an operator who typed one command having seen
	// the box himself. o.ByHand is that fact, carried down the launch path
	// from the four entry points a human types at. The reading is still
	// TAKEN on both arms — silence would trade one bad behaviour for
	// another — so an interactive launch prints the same witness and the
	// same culprits as a WARNING and proceeds, marking nothing degraded.
	// The refusal below is what a dispatch pass, its refills and the pulse
	// still get, unchanged; `load_guard: 0` still turns the whole thing off.
	// Its advice half is LoadGuardEscape, which names the re-promote that
	// knob needs on a home whose config.yaml a manifest attests
	// (ranger-base-6s00n) — the fleet ceiling is a legitimate config change,
	// but on a promoted home an unattested one refuses every later dispatch.
	if why := a.LoadHigh(b.warnWriter()); why != "" {
		if o.ByHand {
			b.warn("posse: %s — launching %s anyway: this guard holds the FLEET back, not a launch you typed (ranger-base-jfe5z)%s\n",
				why, o.Name, a.LoadCulpritLine())
		} else {
			return nil, Die("%s — refusing to launch %s into a saturated box; wait for it to drain, or %s%s",
				why, o.Name, a.LoadGuardEscape(), a.LoadCulpritLine())
		}
	}
	a.TightenEnvPerms(os.Stderr)    // every launch re-asserts 700/600 on envs/
	a.TightenSecretPerms(os.Stderr) // and on secrets/, which no launch reads

	// The instance tag is checked here, with the other refusals that are
	// knowable before anything is touched (rangerhq-ouf9). It gates every
	// create because every create plans first — `posse new`, a recipe, a
	// cockpit key, dispatch, and relaunch, which plans before it kills. A
	// tag posse cannot render is refused rather than ignored: ignoring it
	// would put this home's sessions back under the bare labels the key was
	// set to move them off, on the one server another instance is watching.
	if _, err := a.Instance(); err != nil {
		return nil, err
	}

	// ADR 0015 §3, the launch verify: the promoted constitution is hashed
	// against its manifest before anything is launched with it. A DISPATCHED
	// session refuses — nobody is watching that launch, and the fix is one
	// operator command (`posse promote`), so fail-closed is cheap. An
	// interactive launch warns DEGRADED and proceeds: the operator IS the
	// oversight there, and refusing to open a session is how they would fix
	// it. `envs/` is not in the promoted set and never reaches this check
	// (§7): a hand-edited env set is a supported live path, not a mismatch.
	// No manifest = nothing was ever promoted here = nothing to check, which
	// is what keeps every pre-0015 home launching.
	if v := a.VerifyPromoted(); !v.OK() {
		if o.Bead != "" {
			return nil, constitutionRefusal{Die("%s\n  dispatch refuses to launch on a constitution nobody promoted (ADR 0015 §3)\n"+
				"  the operator clears it with: posse promote", v.Line())}
		}
		b.warn("posse: DEGRADED — %s (ADR 0015 §3; clear it with `posse promote`)\n", v.Line())
	}

	dir := o.Dir
	if dir == "" {
		dir = a.CfgGet("default_dir", os.Getenv("HOME"))
	}
	dir = ExpandTilde(dir)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, Die("directory not found: %s", dir)
	}

	// The session's own tree, before anything else reads `dir`: the gates,
	// the hooks, the parity probe, the skills render, the seatbelt profile
	// and claude's directory trust are all path-scoped, and every one of
	// them must name the tree the persona will actually work in
	// (rangerhq-09o2). Doing this first is what makes that true by
	// construction rather than by nine remembered call sites.
	repo, branch := "", ""
	var tree *SessionTree
	if o.Worktree {
		t, err := a.EnsureSessionTree(dir, o.Name, b.warnWriter())
		if err != nil {
			return nil, err
		}
		if t != nil {
			tree = t
			dir, repo, branch = t.Path, t.Repo, t.Branch
			// The branch's own copy of `bead:` (worktree.go beadKey): the
			// meta below carries the same pointer and is removed by every
			// kill, and the landing sweep has to find a tree whose session
			// is already gone (ranger-base-nurl). Best effort — a launch
			// must not fail because a git config write did not.
			if err := recordBead(t.Repo, t.Branch, o.Bead); err != nil {
				b.warn("posse: %s not stamped with bead %s (%v) — a later pass cannot tell what it is holding\n", t.Branch, o.Bead, err)
			}
		}
	}

	emoji := o.Emoji
	if emoji == "" {
		emoji = a.EmojiFor(o.Name)
	}

	cmd := o.Cmd
	var ag *AgentFile
	var rt *Runtime
	caged := false // the session really runs inside the L4 cage (ADR 0002 §3)
	// ADR 0052 D2: the session hooks dir a managed repo's L3 was rendered
	// into, "" when this launch has none. Declared here because the render
	// happens with the other L3 work and the env that aims git at it is
	// assembled with the rest of the launch vars, far below.
	hooksRedirectDir := ""
	// ADR 0052 D3: what the render just built, handed to the probe that
	// judges this launch. nil for every launch that rendered nothing, which
	// is the probe reading git's own dispatch path as it always has.
	var hooksProbe *l3Redirect
	managedHooksPath := ""
	runtime, tier, gatesDir, cage, degraded := "", "", "", "", ""
	if o.Agent != "" {
		var err error
		if ag, err = a.LoadAgent(o.Agent); err != nil {
			return nil, err
		}
		if err := ag.EnsureMemoryDir(); err != nil {
			return nil, err
		}
	}

	// Env sets: explicit ones (--env-file, recipe env_files) always; the
	// persona's own `envs:` list on top for persona sessions; config
	// default_env only for a session without a persona. The rule itself
	// lives in LaunchEnvSets (envs.go) because the ADR 0019 seam selects
	// the same sets by the same rule when it reads the session credential
	// this launch is about to export (ADR 0039 D3d as amended) — two copies
	// would drift silently.
	//
	// Computed HERE, at the first line where both inputs are final, because
	// the availability preflight a few lines down is handed this same list:
	// the catalog it reads is read with the mint of the sets THIS launch
	// realizes (ADR 0039 D3d as amended, item 3), and the `vars` loop far
	// below exports the values out of those same sets. NAMES only — the
	// values are read where they are used, by `vars` for the launch and by
	// the seam at the moment of the probe.
	envs := a.LaunchEnvSets(o.Envs, ag)

	if o.Agent != "" {
		var err error
		// Runtime: CLI/recipe override > PID runtime: > config default >
		// claude. The PID's own command: applies only on its own runtime.
		own := a.ResolveRuntime("", ag)
		runtime = a.ResolveRuntime(o.Runtime, ag)
		rt, err = a.LoadRuntime(runtime)
		if err != nil {
			return nil, err
		}
		// ADR 0053 D1's last precondition, asked at the first line where the
		// runtime is loaded: a runtime with no model flag has nowhere to put
		// the typed id. Dropping it would open the session on the tier map's
		// own model while `model:` said otherwise — the silent substitution
		// D3 exists to refuse, one layer down.
		if o.Model != "" && rt.ModelFlag == "" {
			return nil, Die("%s declares no model flag, so --model %s cannot be rendered onto its launch line — the session would open on the tier's own model with nothing saying so (ADR 0053 D1)", rt.Name, o.Model)
		}
		tier = a.ResolveTier(o.Tier, ag)
		if !ValidTier(tier) {
			return nil, Die("unknown tier %q (strong | standard | fast)", tier)
		}
		// Availability preflight (rangerhq-oay, modelavail.go): the tier is
		// resolved, so the model id it names is known — ask whether this
		// account can actually run it, and SAY so when it cannot. Loud,
		// never silent, never a refusal of its own, and since ADR 0003 §3
		// never a substitution either: the pair below this call is the pair
		// resolved above it, always. `runtime` and `tier` are read-only from
		// here down, which is what "availability never chooses a
		// replacement" amounts to at this line.
		//
		// It still runs BEFORE the parity check, and now for a simpler
		// reason than it used to have: the wall and the PID's `tier_floor:`
		// rule on the asked-for pair because that is the only pair a launch
		// can open on.
		//
		// ADR 0053 D3 is the one exception, and it is above the call rather
		// than inside it: an exact-model launch is not running the tier's
		// model, so a verdict about the tier's model would describe a launch
		// nobody made. The line printed instead says what is being asked
		// and of whom: the provider, about the exact id, rather than the
		// catalog about the tier's.
		if o.Model != "" {
			b.warn("posse: %s\n", ExactModelLine(o.Name, runtime, tier, o.Model))
		} else if line := a.TierPreflightFrom(envs, o.Agent, runtime, tier, b.warnWriter()).Line; line != "" {
			// UNKNOWN or unavailable, and both launch the asked-for id and
			// say so (ADR 0039 D3c, ADR 0003 §3). The line is the whole
			// bound on the risk, and it is now the whole product of the
			// preflight: nothing here writes it to the meta, because a mark
			// that outlives the launch is a claim about a session, and the
			// only claim posse keeps about a session's pair is `runtime:`
			// and `tier:` — what it really opened on.
			b.warn("posse: %s\n", line)
		}
		// ADR 0013 §2 layer 2 (ranger-base-a9y9, escaped as ranger-base-9r33):
		// a first-run dialog whose DEFAULT ACTION MUTATES THE MACHINE is a
		// launch REFUSE until the operator's own config silences it. Asked
		// HERE for one reason since ADR 0003 §3, where it used to have two:
		// it is above every render below, so a refused launch cuts no
		// worktree, writes no gates and seeds no trust. The second reason
		// was that the availability preflight above could still have moved
		// `rt`, making this the first line where it named the runtime the
		// session would really open on. Nothing moves it now (see the note
		// on that call), so `rt` has been that runtime since it was loaded,
		// and this position buys only the ordering.
		//
		// The dispatched/interactive split is ADR 0015 §3's, twelve lines up,
		// and here it is not merely the same shape: the operator's remedy for
		// codex's update menu is to ANSWER it, in their own codex session, and
		// a posse that refused to open one would have walled off the only way
		// to clear its own refusal. So a launch nobody is watching refuses,
		// and the operator's own is told what it is about to open on and left
		// to decide — which is also the escape hatch, and the reason there is
		// no config key for one.
		//
		// Dispatch does not normally reach this arm: launchSession refuses
		// above the claim, because by the time a launch is being planned the
		// argv path has already taken the bead (dispatch.go). This is the
		// backstop for every other path that carries a bead — the cockpit's
		// `d` on a session it must create, a recipe, a relaunch.
		if line := DangerLine(rt); line != "" {
			if o.Bead != "" {
				return nil, DangerRefusal(rt, line)
			}
			b.warn("posse: DEGRADED — %s launch opens on %s; an interactive launch proceeds because answering that screen is what you would open a session to do (ADR 0013 §2)\n", rt.Name, line)
		}
		// Enforcement parity (ADR 0002 §4): the cage the session gets is the
		// best available tier (shims today); the PID may demand more. Any
		// gate no wall layer realizes refuses the launch unless degradation
		// was allowed explicitly — then it launches marked.
		if ag.Cage != "" && !ValidCage(ag.Cage) {
			return nil, Die("%s: cage: %q is not shims | seatbelt | container", o.Agent, ag.Cage)
		}
		if o.Cage != "" && !ValidCage(o.Cage) {
			return nil, Die("--cage %q is not shims | seatbelt | container", o.Cage)
		}
		// `sockets:` is a container-tier key and an opt-in, not a gate: a
		// name that is known is mounted and recorded, a name that is not is
		// a PID error here rather than a silent no-op inside (ADR 0002 §5).
		if err := CheckSockets(ag); err != nil {
			return nil, err
		}
		if ag.TierFloor != "" && !ValidTier(ag.TierFloor) {
			return nil, Die("%s: tier_floor: %q is not strong | standard | fast", o.Agent, ag.TierFloor)
		}
		cage = ResolveCage(o.Cage, ag)
		// A container tier this host cannot provide is a degradation like
		// any other: refused, or launched on the host and marked. What it
		// never is, is a launch that pretends to be caged.
		caged = cage == CageContainer && a.ContainerAvailable()
		// L3 is reconciled before the directory-aware parity check so parity
		// can execute the slots and report what they do. Install errors stay
		// best-effort — a legitimate foreign chain is expected to make install
		// refuse — but a slot that does not actually gate the probe is now a
		// visible degradation instead of a swallowed error.
		//
		// The commit-guard slot is probed BEFORE InstallCommitGuardHook
		// overwrites it (ranger-base-2mogn): install-then-probe always sees
		// this binary's own fresh render, so a repo IN the launch rotation
		// whose stamp had drifted from config self-heals SILENTLY and the
		// operator never learns the wall was wrong, or for how long.
		// preHeal is compared, not enforced — CheckParityIn below still
		// judges the launch on the POST-heal state; this only reports what
		// was actually on disk a moment earlier, so a real drift is a
		// finding rather than a repair nobody heard about.
		preHeal := a.probeL3Hooks(dir, deniesGitPush(ag.Deny))
		// ADR 0052 D1: on a MANAGED hooks path the install step writes
		// nothing at all — not the two renders, not a chain — and says so
		// on the launch. The pre-heal comparison is skipped with it: the
		// slots there are the employer's, so "the wall was WRONG before this
		// launch re-stamped it" would name a drift nobody caused and a
		// re-stamp that did not happen. The wall itself is bead 2's render.
		mh, _ := managedHooksDir(dir)
		if mh.Managed {
			// ADR 0052 D3 records this path in the session meta, a flat file
			// whose reader stops at the first newline (ranger-base-ujdg). git
			// accepts a core.hooksPath that is not one line and the
			// dispatcher would run it; what posse cannot do is RECORD it —
			// the path's tail would read back as meta fields of its own,
			// `crew: true` among them, on a session that was never crew
			// (ranger-base-buvq4). Refused here, before a render, a workspace
			// or a record exists. Only where the record would be written:
			// the container tier below applies no redirect and records none.
			if !caged && strings.ContainsAny(mh.Dir, "\n\r") {
				return nil, Die("posse: %s — managed hooks path %q is not one line; the session record cannot carry it (ADR 0052 D3)", o.Name, mh.Dir)
			}
			// One line is necessary and not sufficient. The reader that
			// stops at a newline also cuts the value at " #", strips a
			// wrapping pair of double quotes, trims it and reads "~"/"null"
			// as unset — each a legal path, each recorded WRONG rather than
			// truncated, and none of them rescuable by any encoding this
			// reader can decode (ranger-base-m6szh, escaped from
			// ranger-base-buvq4, MEASURED). Refused on the same ground and
			// at the same place: a record that quietly says a different path
			// than the one this launch is running is worse than a launch
			// that did not happen. Asked of the reader rather than of a list
			// of its rules, so the guard cannot fall behind it again.
			if got, ok := flatScalarRoundTrip(mh.Dir); !caged && !ok {
				return nil, Die("posse: %s — managed hooks path %q cannot be recorded: the session record's flat-YAML reader would read it back as %q (ADR 0052 D3)", o.Name, mh.Dir, got)
			}
			b.warn("posse: %s in %s — %s\n", o.Name, AbbrevHome(dir), mh.line())
			// ADR 0052 D2: the wall posse may not install THERE is rendered
			// here instead — its own dir, per session, and the session's env
			// aims git at it. Rendered before CheckParityIn, so the probe
			// that judges this launch reads the dir this launch just built.
			//
			// Not at the container tier. The env would cross the boundary
			// (CageEnvNames forwards every var name shaped like one), and
			// the dir it names is not on the mount list — a core.hooksPath
			// pointing at a path that does not exist inside is not "ours
			// missing", it is EVERY hook skipped, the employer's included,
			// which is the one thing ADR 0052 promises never to do. So a
			// caged launch on a managed repo keeps today's behaviour and
			// says which half it did not get.
			if caged {
				b.warn("posse: %s — the session redirect is not applied at the container tier: %s is not inside the cage, and a core.hooksPath naming a path the cage does not have skips every hook there, the managed ones too (ADR 0052 D2)\n", o.Name, AbbrevHome(a.SessionHooksDir(o.Name)))
			} else {
				red, err := a.RenderSessionHooks(o.Name, dir, mh, deniesGitPush(ag.Deny))
				if err != nil {
					return nil, err
				}
				hooksRedirectDir, hooksProbe, managedHooksPath = red.Dir, red.probe(), red.Managed
				b.warn("posse: %s — L3 rendered at %s: %s dispatched into %s\n", o.Name, AbbrevHome(red.Dir), strings.Join(red.Slots, ", "), AbbrevHome(red.Managed))
				for _, skip := range red.Skipped {
					b.warn("posse: %s — not forwarded from %s: %s\n", o.Name, AbbrevHome(red.Managed), skip)
				}
			}
		} else {
			if deniesGitPush(ag.Deny) {
				InstallPrePushHook(dir)
			}
			a.InstallCommitGuardHook(dir)
		}
		parity := a.checkParityIn(ag, rt, cage, tier, dir, hooksProbe)
		if !mh.Managed && preHeal.Repo && !preHeal.CommitGuard {
			b.warn("posse: %s launch found the L3 prepare-commit-msg wall in %s WRONG before this launch just silently re-stamped it — %s\n", o.Name, AbbrevHome(dir), preHeal.CommitGuardDegraded)
		}
		if len(parity.Degraded) > 0 {
			// ADR 0003 §3: at fast the operator's consent is not on offer —
			// the wall is the only thing left holding the PID's gates.
			if !o.AllowDegraded || parity.NoDegrade {
				return nil, degradedError{parity}
			}
			degraded = strings.Join(parity.Degraded, "; ")
			b.warn("posse: %s launches DEGRADED on %s @ %s — %s\n", o.Name, rt.Name, cage, degraded)
		}
		// Skills (ADR 0007 §2): rendered fresh here, like the gates — the
		// tree {skills} points at for claude, the session dir's
		// .agents/skills/ links for codex and grok. A name that resolves to
		// nothing refuses the launch rather than binding a dangling symlink.
		if _, err := a.RenderSkillsFor(ag, rt, dir); err != nil {
			return nil, err
		}
		// Directory trust (rangerhq-w4uf): on claude the session dir has to
		// be trusted BEFORE the command is typed, or the CLI opens on a
		// modal instead of a composer and the launch is over — the same
		// failure the codex line's trust flag exists to prevent, one
		// runtime over and with no flag to type it on. Not at the container
		// tier: the config a caged claude reads is the cage HOME's, and
		// SeedCageHome seeds it there with the same keys.
		if !caged {
			if _, err := SeedClaudeTrust(b.ClaudeConfig, rt, dir); err != nil {
				return nil, err
			}
		}
		// The store of record must be writable, and under ADR 0012 D3-C it is
		// usually NOT under the session dir: <dir>/.beads holds a redirect and
		// the database lives in the instance repo it names. A self-sandboxing
		// runtime confines writes to its workspace, so unless that target is
		// named here every `bd close` and `bd comments add` from the session
		// is denied — which is exactly what happened to five dispatched codex
		// sessions before anyone noticed the beads were silent, not the agent
		// (ranger-base-0fb). Runtimes posse cages itself ignore this.
		//
		// launchWritableRoots is the whole list — the store, its git dirs
		// when a redirect moves it out (ranger-base-xqwr: `bd sync` commits
		// the JSONL and dies on index.lock without them), and this tree's
		// own git dirs (rangerhq-09o2) — and it is the SAME function ADR
		// 0013 §4's reachability row judges this line with, so the row and
		// the launch cannot disagree about what "writable" meant.
		cmd = ag.RenderCommandForModel(rt, own, tier, o.Model, launchWritableRoots(dir)...)
		// The PID channel is a launch guarantee too, and this is the one
		// place it can be checked: the line exists now, and nothing after
		// this point can put back what a voiding flag discards
		// (ranger-base-64qx). A rendered line naming one of this runtime's
		// PIDVoid flags would open a session carrying every native rulebook
		// and no persona, so it is refused rather than launched marked —
		// `degraded` is for a gate the wall could not realize, and a
		// persona that is not in the session is not a weaker persona.
		// ADR 0053 D1's refusal, asked where it can be MEASURED rather than
		// assumed: the id is on the line, or the launch does not happen.
		// rt.ModelFlag above says the runtime has somewhere to put it; this
		// says the rendered template actually did. The two are different
		// questions because {model} is a PLACEHOLDER — a template that never
		// mentions it renders a perfectly good launch line with the operator's
		// canary silently missing, and the PID's own command: is exactly such
		// a template when the launch is on the persona's own runtime.
		//
		// Refused, not patched: appending the flag ourselves would build the
		// second, drifting runtime template ADR 0053 rejects --cmd for.
		if o.Model != "" {
			if want := rt.ExactModelText(o.Model); want == "" || !strings.Contains(cmd, want) {
				return nil, Die("%s: the rendered %s launch line does not carry --model %s — the template it was rendered from has no {model} for the id to land in, so the session would open on the tier's own model with the record saying otherwise (ADR 0053 D1)\n"+
					"  add {model} to that command: template, or launch this canary on a runtime whose template has one",
					o.Agent, rt.Name, o.Model)
			}
		}
		// ranger-base-k62e, and the same shape as the refusal below it: a
		// self-sandboxing runtime whose line names a root it will refuse
		// opens a session that can run NO command — codex validates its
		// writable roots before it applies the sandbox, and one bad root
		// refuses the whole set. Asked here because here is where the line
		// exists; refused rather than warned because the alternative is the
		// silence ranger-base-c02a cost a P1 for.
		if err := writableRootRefusal(o.Agent, rt, cmd); err != nil {
			return nil, err
		}
		if f := rt.PIDVoided(cmd); f != "" {
			return nil, Die("%s: the rendered %s launch line names %s, which makes %s discard the PID this line delivers — the session would open carrying every native rulebook and no persona at all (measured, ranger-base-64qx; docs/adr/0013-rules-precedence-probe.md)\n"+
				"  drop %s from this PID's command:, or fold the PID into the override text yourself — that replaces the runtime's own system prompt, which is a decision, not a default",
				o.Agent, rt.Name, f, rt.Name, f)
		}
		// ADR 0013 §2, and the reason it is HERE and not further down: the
		// prompt is an argument to the RUNTIME, so it goes on the runtime's
		// line before any wall wraps it. Appended after the seatbelt prefix
		// it would be an argument to sandbox-exec; appended after the cage
		// it would be one to `docker run`.
		if o.PromptFile != "" {
			if rt.PromptMode() != PromptArgv {
				return nil, Die("%s declares prompt: %s — a work prompt cannot ride on its launch line (ADR 0013 §2)", rt.Name, rt.PromptMode())
			}
			cmd += ArgvPromptSuffix(o.PromptFile)
		}
		// L2 seatbelt: the runtime runs under sandbox-exec with a profile
		// rendered from the PID; the outer shell expands $(cat {file}) first.
		if seatbeltWallRendered(cage, rt) {
			// state_dir: (ADR 0012 D4) — the runtime's own state tree joins
			// the writable set. Without it a third-party CLI runs under a
			// sandbox that makes its config read-only, which it reports as a
			// first-run flow that never sticks rather than as a denial.
			prof, err := a.RenderSeatbelt(ag, dir, rt.StateDirs...)
			if err != nil {
				return nil, err
			}
			cmd = SeatbeltPrefix(prof) + cmd
		}
		// L1 gates (ADR 0002 §3, ADR 0009): shims rendered fresh from deny:,
		// PATH prefixed on the typed line and SHELL/GROK_SHELL pointed at the
		// gate shell — every persona session, every runtime. Not at the
		// container tier: those shims exec host paths and that gate shell is
		// the host's zsh, so typing them in front of the engine would put a
		// dead wall on the line. The cage renders its own inside instead —
		// `posse gates wrap` on the engine's inner line (cageinner.go).
		if !caged {
			cmd, gatesDir, _, err = a.WrapWithGates(o.Agent, rt, ag.Deny, cmd)
			if err != nil {
				return nil, err
			}
			// The one shim that is aimed at the RUNTIME as well as at the
			// persona: the gates dir leads the PATH of the runtime process
			// itself, so a deny over the binary this CLI reads and writes
			// its own credential with gates its login and its refresh
			// (ranger-base-eupf). ADR 0042 D1 keeps that deny — it is what
			// holds the operator's rotating pair to one writer — and D2 puts
			// a precondition where ranger-base-eupf's warning stood: the
			// session runs on the mint posse injects, so the launch needs it
			// among the names it is about to inject and refuses without it.
			// Applied below, at the first point the env sets are resolved;
			// with the mint present nothing is said at all.
		} else {
			// The cage renders its own inside, so the var the session's tools
			// read — the pre-push hook above all, which appends its refusal to
			// $RHQ_GATES_DIR/refusals.log — has to name the path they will
			// find them at. That path is the image's, not the host's; the one
			// file behind it that is the host's is mounted there (CageMounts).
			gatesDir = CageGatesDir(o.Agent)
		}
		if emoji == "" || emoji == a.EmojiFor(o.Name) {
			if e := a.EmojiExact(runtime); e != "" {
				emoji = e // the cockpit shows what the persona runs on
			}
		}
	} else {
		// The other half of the same guarantee (rangerhq-oaya). A launch
		// with no persona never went near RenderCommandFor, so its line
		// named no mode at all and the session took the CLI's default —
		// which is exactly the thing that moved under the fleet once
		// already (rangerhq-qs5r). The runtime is unnamed here, so the line
		// itself is asked: see EnsureUnattendedLine for what that can and
		// cannot recover, and why a mode already typed is left alone.
		cmd = EnsureUnattendedLine(cmd)
	}

	// The values, at last: `envs` was computed above the preflight (which
	// reads the catalog with the credential these same sets carry), and
	// this is the first line that needs what is IN them.
	var vars []EnvVar
	for _, n := range envs {
		vs, err := a.EnvSetVars(n)
		if err != nil {
			return nil, err
		}
		vars = append(vars, vs...)
	}

	// The other half of the seatbelt's credential read-deny (ADR 0019 D2
	// item 3, ranger-base-x5f6p): the profile was rendered from the
	// LAUNCHER's environment a hundred lines above, and the env sets are
	// overlaid on the child below, so a set exporting either config-dir
	// name would move the runtime's credential write past a wall already
	// written. Asked HERE because this is the first line at which `vars`
	// exists, and asked through the shared helper because RelaunchAgent
	// renders a persona line too and must refuse the same state
	// (ranger-base-179hy). The whole argument lives on the helper.
	if err := credentialDirEnvSetRefusal(cage, rt, vars); err != nil {
		return nil, err
	}

	// env_required (ADR 0012 D4): the runtime declared variable NAMES a
	// session on it cannot work without, and the env sets are resolved, so
	// this is the first moment the question can be answered. It refuses
	// rather than warning because the alternative is the failure it exists
	// to name — a pane that opens, fails to authenticate, and reads to
	// herdr as an agent sitting idle.
	//
	// Checked here and not at the top: `vars` is what the session gets, and
	// an operator who exported the name in their own shell has supplied it
	// just as legitimately as an env set (MissingEnv looks in both). Names
	// only — nothing on this path reads a value.
	if missing := MissingEnv(rt, vars); len(missing) > 0 {
		return nil, EnvRequiredError(rt, missing)
	}

	// The credential precondition of the gates block above (ADR 0042 D2),
	// asked here because this is the first line at which the answer exists:
	// the question is which env-set names this launch injects. Same key and
	// same question as the caged precondition one tier up (CheckCageCredential
	// via WrapInCage, below) — a shimmed runtime and a caged one both
	// authenticate with the mint and nothing else. Not under `degraded`: a
	// session that cannot authenticate is not a weaker session, so this is
	// an error --allow-degraded cannot waive.
	if ag != nil && !caged {
		if err := CheckCredGate(o.Agent, rt, ag.Deny, filepath.Join(gatesDir, "bin"), CageEnvNames(vars)); err != nil {
			return nil, err
		}
	}

	// RHQ_HOME rides every session, persona or crew, because any rhq/bd
	// tool run inside resolves its instance from this var (falling back to
	// ~/.config/rhq otherwise) — a second RHQ_HOME's session addressing the
	// wrong instance's config, queue, and skills silently (ADR 0012 §D2,
	// rangerhq-ysly). Appended after the env-set vars so this instance's
	// identity is authoritative over anything an env file happened to set.
	//
	// POSSE_HOME rides alongside it for exactly one release (ranger-base-mlc
	// Q2 ruling): newApp() reads either, RHQ_HOME winning when both are set,
	// so an installed L3 hook or shim reading the old name does not go quiet
	// mid-transition. Drop POSSE_HOME's sibling read once the window closes.
	vars = append(vars, EnvVar{"RHQ_HOME", a.Home}, EnvVar{"POSSE_HOME", a.Home})

	// ADR 0052 D2, the other half of the render above: git's own
	// config-in-env form, so every git in this session — including one
	// invoked by absolute path (M2) — dispatches its hooks from the dir
	// posse rendered, and runs the employer's after ours. Appended after
	// the env sets so a count of the operator's own is read and extended
	// rather than overwritten.
	if hooksRedirectDir != "" {
		vars = append(vars, gitConfigHooksPathVars(vars, hooksRedirectDir)...)
	}

	// RHQ_LAUNCH_HOME (ADR 0031 §1): the record of where this session was
	// born, stamped alongside RHQ_HOME but never overridden by a persona's
	// scratch run — that is the whole reason init (ADR 0031 §2) compares
	// against it instead of RHQ_HOME.
	vars = append(vars, EnvVar{EnvLaunchHome, a.Home})

	// BEADS_DIR (ADR 0055 D1-D2): the store of record rides the session
	// env, so the one-graph clause of rangerhq-09o2 stops depending on
	// which mode bd happens to be in. bd in NO-DB mode resolves $BEADS_DIR
	// else $cwd/.beads on both the read and the write-back (cmd/bd/nodb.go,
	// main.go's PersistentPostRun) and never calls FindBeadsDir — so it
	// consults neither the `.beads/redirect` posse seeds nor the worktree's
	// main repo, and a bead filed from a session worktree lands in that
	// worktree's own issues.jsonl while `bd where` names the main store.
	// The fork is invisible to a read, because the worktree's checked-out
	// jsonl carries the main rows by construction; only a write tells them
	// apart (measured 2026-09-04 on bd 0.50.3 — worktree.go's WHAT WAS
	// MEASURED block, no-db bullet). No-db has four doors and posse opens
	// one of them itself: the container tier's inner bd is always `--no-db`
	// (CageBdFlags, cageinner.go), so at that tier the fork is the shipped
	// configuration on EVERY store class.
	//
	// beadsHome(dir) is the answer three other consumers of this same `dir`
	// already take (ADR 0012 D3-C): the seatbelt writable set
	// (RenderSeatbelt → seatbelt.go), the cage mount (CageMounts) and the
	// codex launch line. One resolver, four consumers — bd's own resolution
	// is now the fourth, so the grant, the mount, the launch line and the
	// store bd writes to cannot disagree.
	//
	// EVERY session, persona or crew, on every runtime: which store this
	// directory belongs to is a fact about the directory, not about the
	// persona, which is why this sits outside the `ag != nil` block below
	// rather than beside BD_ACTOR inside it. CageEnvNames names it too, so
	// the container tier forwards it — by name, from the pane's own env,
	// which is also what carries it through a relaunch, where `vars` holds
	// the env sets and nothing else.
	//
	// Set ONLY where the resolved directory exists — the same decline
	// seedBeadsRedirect makes for a repo with no `.beads` (worktree.go):
	// there is nothing to keep unforked, and where bd creates a store on
	// its first write is bd's business, in the directory bd chooses.
	//
	// No store-class detector, no `--no-db` argv grep, no refusal (D3): the
	// fix is mode-independent, so a detector would only add a reader of
	// bd's configuration to posse — a copy of its own isNoDbModeConfigured,
	// covering one door of four, going stale on its own schedule.
	//
	// Appended after the env sets for the reason RHQ_HOME is: the store
	// this launch resolved is authoritative over anything a set happened to
	// carry. A persona that genuinely needs ANOTHER repo's graph sheds it
	// for that one call — `env -u BEADS_DIR bd …` (AGENTS.md). A store
	// under one of bd's unsafe prefixes would make every bd call in the
	// session refuse loudly by name ("BEADS_DIR points to unsafe
	// location"); that refusal is preferred to a launch-time copy of bd's
	// prefix list (ADR 0055 Consequences).
	if home := beadsHome(dir); isDirPath(home) {
		vars = append(vars, EnvVar{"BEADS_DIR", home})
	}

	if ag != nil {
		// The persona's durable identity rides the environment: BD_ACTOR
		// makes every bd call inside the session attribute to the persona
		// (claims, closes, mail), and the memory dir is there for tooling.
		// RHQ_PERSONA_DIR/POSSE_PERSONA_DIR: same both-names window as
		// RHQ_HOME/POSSE_HOME above — a seeded PID's `Read
		// $RHQ_PERSONA_DIR/ORDERS.md` keeps resolving during the transition.
		vars = append(vars,
			EnvVar{"BD_ACTOR", o.Agent},
			EnvVar{EnvPersona, o.Agent},
			EnvVar{"RHQ_PERSONA_DIR", ag.MemoryDir},
			EnvVar{"POSSE_PERSONA_DIR", ag.MemoryDir},
			EnvVar{"RHQ_RUNTIME", runtime},
			EnvVar{"RHQ_TIER", tier},
			EnvVar{"RHQ_GATES_DIR", gatesDir},
			EnvVar{"RHQ_CAGE", cage},
			// ADR 0007 §2's exit hatch: even a runtime posse cannot point
			// at the skills can be told where they are, by the PID's body or
			// by a wrapper.
			EnvVar{"RHQ_SKILLS_DIR", a.SkillsDir()},
		)
		if len(ag.Skills) > 0 {
			vars = append(vars, EnvVar{"RHQ_SKILLS", strings.Join(ag.Skills, "\n")})
		}
		// The PID's tool lists also ride the env (newline-joined) — ADR
		// 0001's exit hatch: {allow}/{deny} render for claude's flags, and
		// a wrapper for any other runtime applies these its own way.
		if len(ag.Allow) > 0 {
			vars = append(vars, EnvVar{"RHQ_TOOLS_ALLOW", strings.Join(ag.Allow, "\n")})
		}
		if len(ag.Deny) > 0 {
			vars = append(vars, EnvVar{"RHQ_TOOLS_DENY", strings.Join(ag.Deny, "\n")})
		}
	}

	// Which HEAD this session's tree works on, decided from the tier and
	// nowhere else (ranger-base-t4f1). A caged worktree session commits on a
	// DETACHED head — that is what lets the container tier's common-dir mount
	// grant no ref write at all — and the launcher splices the work back onto
	// the branch at close. An uncaged launch into a tree posse detached puts
	// it back on its branch here. Asked after `caged` is resolved, because
	// the PID's demand and what this host can actually provide are different
	// answers and only the second decides what the session gets.
	if tree != nil {
		if err := PrepareSessionHead(tree, caged, b.warnWriter()); err != nil {
			return nil, err
		}
	}

	// L4: the runtime command becomes the *inner* command of an engine
	// template (cage.go). Last, so the env names it forwards are the ones
	// the session actually gets — the operator's container credential
	// included, which is the precondition this refuses without.
	if caged {
		var err error
		if cmd, err = a.WrapInCage(ag, rt, o.Name, dir, cmd, CageEnvNames(vars), o.PromptFile); err != nil {
			return nil, err
		}
	}

	sockets := ""
	if ag != nil && caged {
		sockets = CageSocketTag(ag)
	}
	// ADR 0052 D3: how L3 was realized here, on the record. "redirect" is
	// the only value that is ever written — its absence is the ordinary
	// `.git/hooks` install — so a reader can tell the two apart without
	// re-classifying a dispatch path that may have changed since.
	hooksMode := ""
	if hooksRedirectDir != "" {
		hooksMode = "redirect"
	}
	return &launchPlan{
		Dir: dir, Repo: repo, Branch: branch, Cmd: cmd, Emoji: emoji, Envs: envs, Vars: vars,
		Runtime: runtime, Tier: tier, Model: o.Model, Cage: cage, Sockets: sockets, Degraded: degraded,
		HooksMode: hooksMode, ManagedHooks: managedHooksPath,
	}, nil
}

// startPlanned turns a verified plan into a live session: the workspace,
// the meta that records it, and the command typed into its root pane. This
// is the whole creative half of a launch — everything past it has already
// been decided by planLaunch. Callers guard the name with nameFree first;
// relaunch does so after its kill.
//
// It returns the workspace it created even when it then fails, because a
// caller cleaning up after a partial launch must be able to tell "nothing
// exists" from "a workspace is up and something later went wrong" — the
// second is not a state to write over (rangerhq-v52t).
func (b *HerdrBackend) startPlanned(o NewSessionOpts, p *launchPlan) (string, error) {
	// The label is the session name plus this home's instance tag, and the
	// meta below is written under the NAME (rangerhq-ouf9): the label is
	// what herdr's one shared list shows, the name is what posse addresses.
	// planLaunch has already refused a malformed tag, so nothing here can
	// create a workspace under a label this home would not recognise.
	wsID, rootPane, err := b.H.CreateWorkspace(b.App.WorkspaceLabel(o.Name), p.Dir, p.Vars)
	if err != nil {
		return "", err
	}
	meta := &HerdrMeta{
		Name: o.Name, Workspace: wsID, Pane: rootPane,
		Emoji: p.Emoji, Envs: strings.Join(p.Envs, "+"), Agent: o.Agent, Runtime: p.Runtime, Tier: p.Tier, Model: p.Model,
		Dir: p.Dir, Repo: p.Repo, Branch: p.Branch,
		Cage: p.Cage, Sockets: p.Sockets, Degraded: p.Degraded, Bead: o.Bead, Crew: o.Crew,
		HooksMode: p.HooksMode, ManagedHooks: p.ManagedHooks,
		// Which server, and which generation of it, issued this workspace
		// id — the id alone identifies nothing across a restart or a
		// handoff (rangerhq-yt1p).
		Socket: SocketID(), Gen: ServerGen(),
	}
	if o.Agent == "" {
		// A persona's line is re-rendered from the PID at every launch; a
		// plain session's --cmd exists nowhere else, so relaunch needs it
		// recorded (flat-YAML: one line, no " #").
		meta.Cmd = o.Cmd
	}
	if p.Cmd != "" {
		meta.Launched = time.Now()
	}
	if err := b.writeMeta(meta); err != nil {
		return wsID, err
	}
	if p.Cmd != "" {
		// The pane was created an instant ago and its shell is still
		// starting: a line over PaneLineMax typed now is lost, not delayed
		// (rangerhq-ybec). PaneLine keeps what is typed short.
		line, err := b.App.PaneLine(o.Name, p.Cmd)
		if err != nil {
			return wsID, err
		}
		if err := b.H.PaneRun(rootPane, line); err != nil {
			return wsID, err
		}
	}
	return wsID, nil
}

// CreateSession mirrors the tmux CreateSession contract (defaults, env sets,
// persona precedence) on the herdr backend: guard the name, resolve the
// launch, start it. This is the entry for a caller that holds no launcher
// lock — `posse new`, LaunchRecipe, a relaunch's own tail; a launcher that
// already holds one calls createSession and hands it over.
// The whole body runs under the launcher lock (ADR 0011 §1). nameFree's
// mustNotOrphan is the check and startPlanned's writeMeta is the act, and
// mustNotOrphan's own doc named the gap between them as the hole it could
// not close: "it does not cover an operator's `posse new` racing a pass —
// that takes no lock". This closes it, and with it the other side of the
// same window — a prune that has just proved this name's old workspace dead
// and is about to unlink the meta this create is writing (rangerhq-3a5t,
// reclaim). Under LaunchBead the lock is already held and underLaunchLock
// runs the body directly; from `posse new` it is taken here, which makes
// `posse new` the launcher ADR 0011 §1 always said it was.
func (b *HerdrBackend) CreateSession(o NewSessionOpts) error {
	return b.createSession(o, nil)
}

// createSession is CreateSession told which lock it is inside. held is the
// launcher lock the CALLER holds, or nil for one that holds none — including
// a caller on a goroutine of a process whose lock is held somewhere else,
// which waits for it rather than reading it as its own (ranger-base-deaz).
func (b *HerdrBackend) createSession(o NewSessionOpts, held *LaunchLock) error {
	// The name's SYNTAX is checked outside the lock, and it is the one
	// check that can be: it reads no state, so no concurrent actor can
	// change its answer. Taking the launcher lock to reject `posse new -x`
	// would queue a typo behind a running pass and leave a lock file in an
	// RHQ_HOME nothing has launched in yet (cmd/posse: help and a refused
	// name leave no state behind). Everything below it reads the meta dir.
	if err := nameSyntax(o.Name); err != nil {
		return err
	}
	return underLaunchLock(b.App, b.warnWriter(), held, func(*LaunchLock) error {
		if err := b.nameFree(o.Name); err != nil {
			return err
		}
		p, err := b.planLaunch(o)
		if err != nil {
			return err
		}
		_, err = b.startPlanned(o, p)
		return err
	})
}

// RelaunchAgent re-types the persona command into a live session's root
// pane after its agent process has gone (crash, /exit, the operator closing
// the CLI): the herdr workspace survives as a bare shell that still carries
// the launch env (BD_ACTOR, RHQ_PERSONA_DIR, tool lists), so re-running the
// command there is a full persona restart without a new workspace
// (rangerhq-vk2). Refuses — returns false, nil — when the session is not an
// posse persona session, when herdr currently detects an agent in it, or when
// the last launch is younger than grace: a CLI that is still starting up is
// invisible to detection for a few seconds, and typing a second command
// into it would land inside its input box.
func (b *HerdrBackend) RelaunchAgent(name string, grace time.Duration) (bool, error) {
	m, ok := b.readMeta(name)
	if !ok || m.Agent == "" || m.Pane == "" {
		return false, nil
	}
	if time.Since(m.Launched) < grace {
		return false, nil
	}
	// A pane id is only as good as the workspace id it hangs off: herdr
	// re-issues those across a restart and a handoff, so the pane a stale
	// meta remembers can be a stranger's (rangerhq-yt1p). Sessions() leaves
	// such a meta out of the listing — but resolving the NAME does not ask
	// that question. Resolve falls back to FOREIGN workspaces by label, and
	// herdr auto-labels a workspace basename(cwd), so an unrelated namesake
	// answers for a session whose own meta was just dropped as a stranger's:
	// the guard passes on w99 while the pane still points at w1
	// (rangerhq-w4zp). So the row must be this meta's own, and the command
	// goes into ITS pane — a guard only guards the line it precedes
	// (rangerhq-i2g9), and the way to keep that true is for the guard and
	// the action to name one workspace, not two.
	//
	// !Foreign is the arm that fires on the namesake, and it is the only one
	// a test can reach: notOurWorkspace() reads label == name as ours, so a
	// FOREIGN row named `name` can never carry this meta's workspace id, and
	// the two arms cannot disagree on any board one listing can produce. The
	// id comparison covers the board a listing cannot show — the meta being
	// rewritten between the readMeta above and the read inside Sessions()
	// (mustNotOrphan's race, rangerhq-jeu2/cpeh). There s.PaneID would be a
	// pane the rest of this function's m — agent, runtime, cage, envs, and
	// the writeMeta below — no longer describes, so it refuses and lets the
	// next pass read one consistent meta.
	//
	// It costs one listing on a path that only runs when a persona's CLI has
	// already died; typing a persona command into somebody else's pane costs
	// their session, and AgentTarget's error does not tell the two apart.
	s, err := b.Resolve(name)
	if err != nil || s.Foreign || s.WorkspaceID != m.Workspace {
		return false, nil
	}
	if _, err := b.AgentTarget(name); err == nil {
		return false, nil // an agent is there after all
	}
	// A real relaunch from here down — one of ADR 0025 §4's fold points.
	// The old container is gone (its CLI crashed or exited, which is why
	// this path fired at all) but the spool it wrote to is a host file and
	// outlives it; fold whatever it holds before a new container starts
	// appending to the same path. Best effort, same as every other fold
	// site: a fold failure must not block reviving a dead CLI.
	if err := b.App.FoldRefusalsSpool(m.Agent, name); err != nil {
		b.warn("fold refusals spool for %s: %v\n", name, err)
	}
	ag, err := b.App.LoadAgent(m.Agent)
	if err != nil {
		return false, err
	}
	if err := ag.EnsureMemoryDir(); err != nil {
		return false, err
	}
	// Same runtime the session was created for (an override at creation
	// must survive a crash restart), rendered exactly as CreateSession did.
	rt, err := b.App.LoadRuntime(m.Runtime)
	if err != nil {
		return false, err
	}
	m.Launched = time.Now()
	if err := b.writeMeta(m); err != nil {
		return false, err
	}
	tier := m.Tier
	if tier == "" {
		tier = b.App.ResolveTier("", ag)
	}
	if _, err := b.App.RenderSkillsFor(ag, rt, m.Dir); err != nil {
		return false, err
	}
	// Same reason as the launch, and the same reason RelaunchAgent renders
	// everything else again rather than trusting the meta: this re-types a
	// full persona command into a live pane, so it has to arrive at a
	// promptable screen (rangerhq-w4uf). Idempotent — a dir seeded at
	// launch costs this path one read.
	if m.Cage != CageContainer || !b.App.ContainerAvailable() {
		if _, err := SeedClaudeTrust(b.ClaudeConfig, rt, m.Dir); err != nil {
			return false, err
		}
	}
	// The same roots planLaunch names, from the same resolver, for the same
	// reason: a self-sandboxing runtime confines writes to its workspace, so
	// unnamed the store of record, that store repo's git dirs and this
	// tree's own are all denied — a revived session is back in the
	// ranger-base-0fb shape, silent beads and a `bd close` that cannot land
	// (ranger-base-qdtw). This path is on the unattended one
	// (dispatch.launchSession), and ADR 0013 §4's reachability row does not
	// cover it: the row runs at CheckParity time against the LAUNCH line,
	// and a relaunch renders its own. Two spellings of "writable" is a
	// crashed CLI coming back degraded with nothing saying so.
	// m.Model rides through for the same reason the runtime and tier do: a
	// relaunch revives THE SAME session, and an exact-model canary that came
	// back on its tier's model would be a launch the operator never asked
	// for wearing a record that says otherwise (ADR 0053 D4). "" on every
	// ordinary session renders what it always did.
	inner := ag.RenderCommandForModel(rt, b.App.ResolveRuntime("", ag), tier, m.Model, launchWritableRoots(m.Dir)...)
	// The launch's root refusal on the one other path that renders a
	// persona line (ranger-base-k62e). Reachable for the reason PIDVoided is
	// — the roots are re-resolved from the box as it is NOW, so a link that
	// went dangling while the session ran arrives here — and this path
	// retypes into a LIVE pane, where the alternative is a revived session
	// in which every command fails.
	if err := writableRootRefusal(m.Agent, rt, inner); err != nil {
		return false, err
	}
	// Same refusal as planLaunch's, on the one other path that renders a
	// persona line (ranger-base-64qx). It is reachable even though the
	// create was refused: a PID edited after its session opened is
	// re-rendered here, and this path retypes into a LIVE pane, so without
	// it a crashed CLI comes back as a session with no persona in it.
	if f := rt.PIDVoided(inner); f != "" {
		return false, Die("%s: the rendered %s line names %s, which makes %s discard the PID — refusing to retype a persona session that would carry none (ranger-base-64qx)", m.Agent, rt.Name, f, rt.Name)
	}
	// The same measured guarantee as the launch's (ADR 0053 D1), on the one
	// other path that renders a persona line. Reachable for the same reason
	// PIDVoided is: the PID and the runtime file are re-read from disk, so a
	// {model} edited out of either since the session opened arrives here —
	// and this path retypes into a LIVE pane, where the alternative is a
	// canary silently coming back on its tier's model.
	if m.Model != "" {
		if want := rt.ExactModelText(m.Model); want == "" || !strings.Contains(inner, want) {
			return false, Die("%s: the rendered %s line does not carry --model %s — refusing to retype the canary session %s on its tier's own model (ADR 0053 D1)", m.Agent, rt.Name, m.Model, m.Name)
		}
	}
	// The same question planLaunch asks, through the same function
	// (ranger-base-179hy). Spelled inline here until it was found drifted
	// from the predicate e241b14 extracted for exactly these two sites: the
	// copy had no `rt != nil` arm, and — worse — nothing tied it to the
	// refusal the answer is FOR, twenty lines below.
	if seatbeltWallRendered(m.Cage, rt) {
		prof, err := b.App.RenderSeatbelt(ag, m.Dir, rt.StateDirs...)
		if err != nil {
			return false, err
		}
		inner = SeatbeltPrefix(prof) + inner
	}
	// Same shape as CreateSession, and for the same reason: at the container
	// tier the wall is rendered inside the cage, so the host's gate prefix
	// never goes on the line. The workspace still carries the launch env, so
	// the engine's `-e NAME` forwards find the same values they did then;
	// the env sets are re-read for their names (and for the credential
	// check, which must refuse a relaunch exactly as it refused the launch).
	//
	// The names are re-read on BOTH arms now (ranger-base-az23f): the
	// container forwards them with `-e NAME`, and the shims arm asks the
	// same list ADR 0042 D2's credential precondition asks. A set that has
	// gone missing since the launch refuses here for the same reason it
	// refuses there — a revived session whose env the meta no longer
	// describes is the thing this whole function re-renders to avoid.
	var vars []EnvVar
	for _, n := range strings.Split(m.Envs, "+") {
		if n == "" {
			continue
		}
		vs, err := b.App.EnvSetVars(n)
		if err != nil {
			return false, err
		}
		vars = append(vars, vs...)
	}

	// The credential-dir refusal planLaunch makes, on the one other path
	// that renders a persona line (ranger-base-179hy). Reachable for the
	// same reason the mint check below is: the sets are re-read BY NAME, so
	// a CLAUDE_CONFIG_DIR (or CLAUDE_SECURESTORAGE_CONFIG_DIR) exported into
	// one after the session opened arrives here — and the profile above was
	// re-rendered from the LAUNCHER's environment, which never saw it. This
	// is the UNATTENDED path (dispatch.launchSession), so it is the one
	// where nobody is present to read the refusal that never came and the
	// revived runtime just writes its credential outside the wall.
	if err := credentialDirEnvSetRefusal(m.Cage, rt, vars); err != nil {
		return false, err
	}
	cmd := inner
	if m.Cage == CageContainer && b.App.ContainerAvailable() {
		// No prompt file: a relaunch restarts a persona whose CLI died, it
		// does not re-dispatch the bead (ADR 0013 §2 — resume is a typed
		// prompt into a live session, never a second argv delivery).
		if cmd, err = b.App.WrapInCage(ag, rt, m.Name, m.Dir, inner, CageEnvNames(vars), ""); err != nil {
			return false, err
		}
	} else {
		var gatesDir string
		if cmd, gatesDir, _, err = b.App.WrapWithGates(m.Agent, rt, ag.Deny, inner); err != nil {
			return false, err
		}
		// Same precondition as planLaunch's, on the one other path that
		// renders a persona line (ranger-base-az23f, the shape
		// ranger-base-64qx set for this exact pair). This is the UNATTENDED
		// path — dispatch.launchSession — so it is the one where nobody is
		// present to read a refusal that never came. Reachable even though
		// the create was refused: the PID is re-read from disk, so a deny
		// added to it after the session opened arrives here first, and the
		// env SETS are re-read by name, so a mint edited or rotated out of
		// one since the launch does too. Either way the alternative is a
		// crashed CLI coming back as a session that cannot authenticate.
		if err := CheckCredGate(m.Agent, rt, ag.Deny, filepath.Join(gatesDir, "bin"), CageEnvNames(vars)); err != nil {
			return false, err
		}
	}
	// Same limit as the launch: this pane's shell has long since started, so
	// canonical mode is not the wall here — but the tty is still one, and a
	// line long enough is lost on a settled pane too (rangerhq-ybec).
	line, err := b.App.PaneLine(m.Name, cmd)
	if err != nil {
		return false, err
	}
	if err := b.H.PaneRun(s.PaneID, line); err != nil {
		return false, err
	}
	return true, nil
}

// RuntimeTierTag is the listing suffix for a persona session: "" for the
// defaults (claude/strong), else "@runtime/tier" so the cockpit shows what
// the persona runs on and at what spend (ADR 0002/0003).
//
// The tier half is the DISPLAY tier (ADR 0013 §6), not the resolved one: a
// session dispatched at `standard` on grok reads `@grok/default`, because
// grok maps no model id and the name would otherwise be a guarantee nobody
// makes. That is also why this is a method — the mapping is a property of
// the runtime, and a declared runtime's map lives under RHQ_HOME.
//
// The suppression is keyed on the DISPLAYED tier too. Only a mapped
// `strong` on claude renders "" here; a default tier that stopped being
// mapped would surface as a tag rather than vanish into the empty string,
// which is the direction this section exists to protect.
// RuntimeTierModelTag is RuntimeTierTag for a session that named an EXACT
// model (ADR 0053 D4): "@<runtime>/<tier>=<model>". A record with no
// `model:` — an ordinary session, or one written before the field existed —
// falls through to RuntimeTierTag and renders exactly as it always has.
//
// Two things the tag does differently when a model is named, both because
// the model is the fact on display:
//
//   - it is never suppressed. The default pair renders "" precisely because
//     it says nothing; "@claude/strong=some-id" says the one thing an
//     operator needs to see, so hiding it would hide the canary.
//   - the tier is the operator's own word, not DisplayTier's. §6 rewrites an
//     unmapped tier to `default` so a tier name is not read as a guarantee
//     about a model — here the model is named outright, so there is no
//     guarantee to protect and rewriting would only hide what was typed.
func (a *App) RuntimeTierModelTag(runtime, tier, model string) string {
	if model == "" {
		return a.RuntimeTierTag(runtime, tier)
	}
	if runtime == "" {
		runtime = DefaultRuntime
	}
	if tier == "" {
		tier = DefaultTier
	}
	return "@" + runtime + "/" + tier + ModelTag(model)
}

func (a *App) RuntimeTierTag(runtime, tier string) string {
	if runtime == "" {
		runtime = DefaultRuntime
	}
	if tier == "" {
		tier = DefaultTier
	}
	shown := a.DisplayTier(runtime, tier)
	if runtime == DefaultRuntime && shown == DefaultTier {
		return ""
	}
	return "@" + runtime + "/" + shown
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range in {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func (b *HerdrBackend) KillSession(name string) error {
	_, err := b.KillSessionAndLand(name)
	return err
}

// NoteBead records which bead a session is working (ADR 0013 §4), for a
// session this pass did not create and so did not stamp at launch. It is a
// pointer and only a pointer — the bead's own store still answers for its
// status — so writing it back costs nothing when it is already right, and
// a session with no meta is not one posse made and gets none.
func (b *HerdrBackend) NoteBead(name, id string) {
	m, ok := b.readMeta(name)
	if !ok || id == "" || m.Bead == id {
		return
	}
	m.Bead = id
	_ = b.writeMeta(m)
	// The branch keeps its own copy, because the meta does not survive a
	// kill and the tree does (worktree.go beadKey, ranger-base-nurl). Same
	// best-effort rule as the write above.
	if m.Repo != "" && m.Branch != "" {
		_ = recordBead(m.Repo, m.Branch, id)
	}
}

// NoteBeadFromPrompt is NoteBead for a hand-dispatch (`posse prompt <name>
// "<text>"`, ranger-base-v674's second gap): that path never gets a bead id
// argument, so a session someone launched by hand with `posse new` and then
// worked with `posse prompt` never carries the `bead:` pointer autoReapPass
// needs, and sits forever (s.Bead == "" is skipped there on purpose — a
// session with no bead pointer is not Dial F's to judge). It parses the same
// "Work beads issue <id> …" header dispatch's own prompts always carry
// (workPromptRe, shared with the cost scanner and turn-failure reader) and
// stamps only when the text matches; an operator just chatting with a
// session leaves its bead pointer untouched.
func (b *HerdrBackend) NoteBeadFromPrompt(name, text string) {
	if m := workPromptRe.FindStringSubmatch(text); m != nil {
		b.NoteBead(name, m[1])
	}
}

// MarkPrompted records that a work prompt was just sent to this session
// (ADR 0011 §3) — the persisted half of PromptGrace, so the next reader is
// any launcher rather than only this process.
//
// Best effort, deliberately: a prompt that landed must never be reported as
// a failure because its record could not be written, and a session posse did
// not create has no record to write. The cost of a missed write is the
// behaviour that shipped before this — the in-process map still holds — so
// failing quiet here degrades to the old guard rather than to none.
//
// Callers hold the launcher lock (ADR 0011 §1), which is what makes the
// read-modify-write safe: no other launcher is between this read and write.
func (b *HerdrBackend) MarkPrompted(name string, at time.Time) {
	m, ok := b.readMeta(name)
	if !ok || at.IsZero() || at.Before(m.Prompted) {
		return
	}
	m.Prompted = at
	_ = b.writeMeta(m)
}

// RunHolder is the holder join as a LOOKUP (ADR 0011 §3): the live session
// whose own run record says it was created to work this bead in this repo.
//
// It replaces asking herdr about a name pattern. A name answers "is there a
// session that WOULD be called this", which is a guess about who holds the
// bead — the lwx/v330 class, where a holder living under the other name in
// the join went unseen and dispatch launched a twin beside it. `bead:` is
// what dispatch itself wrote at create, so the record answers directly.
//
// The CHECKOUT is compared, not the working directory: one bead id is only
// unique within its repo, and a per-session worktree's Dir is not the repo
// dispatch names (rangerhq-09o2), so the record's repo: is what a dispatch
// dir matches against. Foreign sessions carry no record and never match. A
// session with no agent (Status "") still matches: it holds the bead and is
// relaunched in place, which is exactly what the name walk did.
//
// The persona is part of the key because it is part of both names this
// replaces — a session is <persona>'s slot or <persona>'s bead session, and
// a record pointing at somebody ELSE's session is not the join's answer: it
// is a session running another PID, and prompting it with this bead's work
// would be the reassignment nobody asked for.
func (b *HerdrBackend) RunHolder(dir, persona, bead string) (*HerdrSession, bool) {
	if dir == "" || persona == "" || bead == "" {
		return nil, false
	}
	sessions, err := b.Sessions()
	if err != nil {
		return nil, false
	}
	for i := range sessions {
		s := &sessions[i]
		if !s.Foreign && s.Bead == bead && s.Agent == persona && s.Checkout() == dir {
			return s, true
		}
	}
	return nil, false
}

// CrewHolder is ADR 0030 §1's tiebreak: the assignee's own live conversation
// in the bead's repo, asked only at the one moment a claim is genuinely
// ambiguous — an in_progress bead no live session holds under any name (run
// record, Dial F name, slot). The typed route (a crew session made and
// worked by hand) stamps no `bead:` and carries no naming-convention name,
// so RunHolder and the name walk both come back empty for it; this is the
// question that comes next, and only next — callers ask it after those have
// already answered "nobody", never in place of them.
//
// First match, same shape as RunHolder: a live session's Crew mark, its
// Agent, and its Checkout against the repo. Not keyed to the bead, because
// the whole point is that this session carries no pointer to one — asking
// "does the assignee have ANY live conversation here" is the question
// ADR 0030 answers, not "does this one name the bead".
func (b *HerdrBackend) CrewHolder(dir, persona string) (*HerdrSession, bool) {
	if dir == "" || persona == "" {
		return nil, false
	}
	sessions, err := b.Sessions()
	if err != nil {
		return nil, false
	}
	for i := range sessions {
		s := &sessions[i]
		if s.Crew && s.Agent == persona && s.Checkout() == dir {
			return s, true
		}
	}
	return nil, false
}

// Checkout is the repo a session's work belongs to: the checkout its own
// worktree hangs off, or its working directory when it shares one.
func (s *HerdrSession) Checkout() string {
	if s.Repo != "" {
		return s.Repo
	}
	return s.Dir
}

// ForeignKillRefusal is the one sentence every destructive path says about a
// foreign row, and the check itself: nil for a session this home owns, the
// refusal for one it does not.
//
// A foreign row is a live herdr workspace this RHQ_HOME holds no session
// meta for — another instance's session, or a workspace made in herdr by
// hand. The meta dir is the ownership record (ADR 0012's instance
// boundary), so its absence is not "an unmanaged row we may tidy": it is
// the whole evidence that the row is somebody else's. Read-only paths keep
// the fallback — peek, focus and the listing exist to show the whole herd —
// and only the paths that END something ask this.
//
// It names the workspace id because the name is precisely what is NOT
// unique across instances: the id is what an operator carries to `herdr
// workspace list` or to the other home to find out whose it is.
//
// The override is a flag rather than a prompt because destructive paths run
// unattended (plugin/autostart.sh replaces a restored husk by killing it by
// name), and a caller that means it must be able to say so in the command
// itself. autostart deliberately does NOT say it: its husk replacement aims
// at this home's own session, so on a shared server the refusal is the
// right outcome and surfaces as its existing "still present after kill —
// not started".
//
// What no override can repair is the other side's bookkeeping: the owning
// home's state/herdr/<name>.yaml still points at the workspace this just
// closed, and that file is outside this home — its own next listing prunes
// it (prunable, ADR 0011 §2). One more reason the refusal, not the
// override, is the default.
func ForeignKillRefusal(s *HerdrSession) error {
	if s == nil || !s.Foreign {
		return nil
	}
	return Die("NOT killed: %s is a foreign workspace (%s) — this posse home holds no session meta for it, so it belongs to another instance or was made in herdr by hand; close it where it lives, or `posse kill %s --foreign` to close it from here anyway", s.Name, s.WorkspaceID, s.Name)
}

// KillLanding is what a kill did with a session's own git worktree
// (rangerhq-09o2). Nil Tree means the session shared the checkout — every
// session before per-session worktrees, and every crew session — and there
// was nothing of its own to land.
type KillLanding struct {
	Tree  *SessionTree
	Merge MergeOutcome
	Kept  string // why the worktree was left in place ("" = it was removed)
	// Memory is what the kill did with the persona's standing orders
	// (ranger-base-qxvh). Nil is the quiet majority: no persona, no memory
	// dir, or nothing in it a commit does not already hold.
	Memory *MemoryLanding
}

// KillSessionAndLand ends a session and retires the tree it worked in: kill
// the workspace, merge the session branch onto the repo's branch, then
// remove the worktree and the branch.
//
// The order is deliberate. The kill goes first, so nothing is still writing
// into the tree while it is being read; the merge goes before the removal,
// so the removal has nothing left to destroy; and the removal REFUSES while
// anything would be lost — uncommitted files, or commits the repo's branch
// does not have — because a session worktree is the only copy of what is in
// it. A reap that destroys work is the failure this whole feature exists to
// prevent, so a kill that cannot land its work keeps the tree and says so.
//
// It never forces. An operator who really wants a tree with unmerged work
// gone can say so to git, which is a decision a human should have to make
// in their own words.
func (b *HerdrBackend) KillSessionAndLand(name string) (*KillLanding, error) {
	return b.killAndLand(name, KillOpts{})
}

// ForceKillSessionAndLand is the same kill with the ADR 0013 §4 reap guard
// stood down: the operator has read the refusal and said kill it anyway.
// Nothing else changes — the landing still refuses to destroy a tree that
// holds work (RemoveSessionTree), so a forced kill of a dirty session
// worktree still keeps the tree and says so.
func (b *HerdrBackend) ForceKillSessionAndLand(name string) (*KillLanding, error) {
	return b.killAndLand(name, KillOpts{Force: true})
}

// KillSessionAndLandOpts is the same kill with every refusal the operator
// may stand down spelled out — the form `posse kill` itself calls, since it
// is the one caller that has flags to carry.
func (b *HerdrBackend) KillSessionAndLandOpts(name string, o KillOpts) (*KillLanding, error) {
	return b.killAndLand(name, o)
}

// KillOpts is the consent a kill was given: one field per refusal it may
// stand down, and the zero value refuses both ways. They are separate flags
// because they are separate facts, and reading one refusal is no evidence
// about the other: --force says "I have looked at MY session's unfinished
// work", --foreign says "I mean the row that is not this home's". A foreign
// row carries no meta and so never reaches the reap guard at all, which is
// exactly why --force must not carry it — a flag typed from habit about
// one's own dirty tree would otherwise close another instance's live agent
// (rangerhq-selx).
type KillOpts struct {
	Force   bool // ADR 0013 §4's reap guard: dirty tree + open bead is killed anyway
	Foreign bool // the ownership refusal: a workspace this home holds no meta for is closed anyway

	// Land spends one bounded turn asking the agent to write its lessons
	// down before the workspace closes — relaunch's landing turn, on the
	// path that actually destroys sessions (ranger-base-qxvh).
	//
	// It is opt-IN at this layer and opt-OUT at the command line
	// (`posse kill --no-land`), and the asymmetry is deliberate. A turn is
	// real tokens and real minutes, and two of this method's callers cannot
	// afford either: the cockpit runs a kill on its single select loop,
	// where a ten-minute turn is a frozen TUI, and the auto-reaper runs
	// inside a dispatch pass, where N of them in a row is the fleet stalled
	// behind a sweep. Both reap sessions whose bead is CLOSED and whose
	// agent has settled — a persona that already ran its own wrap-up — so
	// what they need from this feature is the COMMIT, which is not optional
	// and happens on every kill regardless of this field. `posse kill` by
	// hand is the one caller that catches a session mid-anything, which is
	// where a turn earns what it costs.
	Land bool
	// Timeout bounds that turn (0 → DefaultLandTimeout, relaunch's own
	// bound for the same turn with the same prompt).
	Timeout time.Duration
	// Out is where the turn's progress is said. A kill that has become slow
	// must say why while it is being slow; nil says nothing, which is what
	// every caller that does not take a turn wants.
	Out io.Writer
}

func (b *HerdrBackend) killAndLand(name string, opts KillOpts) (*KillLanding, error) {
	// Before anything: the meta is the only record of which tree and branch
	// belong to this session, and the kill below deletes it (ADR 0011 §3).
	m, hadMeta := b.readMeta(name)

	// The reap guard, before the workspace is closed and before the meta is
	// dropped: a session still holding an open bead over an uncommitted
	// tree is not killed (ADR 0013 §4, reapguard.go). It runs on the shared
	// checkout too, which is where the near-miss was — there is no session
	// branch there for the landing below to refuse over.
	if hadMeta && !opts.Force {
		if why := b.ReapRefusal(m); why != "" {
			return nil, Die("NOT killed: %s — look first (posse attach %s), or `posse kill %s --force`", why, name, name)
		}
	}

	s, err := b.Resolve(name)
	if err != nil {
		return nil, err
	}
	// The ownership check (rangerhq-selx), after the row is found and
	// before it is closed. Resolve falls back to foreign workspaces by
	// label on purpose, and for `posse peek`/`posse prompt` that is the
	// point — but a destructive path that follows the fallback closes a
	// live agent belonging to whoever does own it. Measured across two
	// RHQ_HOMEs on one herdr: instance A's `posse kill m1-collide` closed
	// instance B's workspace, exit 0, no warning.
	//
	// Ordered after the reap guard above rather than before it, because
	// the guard reads the meta and Resolve is what makes a row foreign.
	// The two overlap on one board only — a meta whose workspace is
	// missing from this listing while a stranger's row wears its label
	// (rangerhq-yt1p/9nso) — where the guard may speak first. Both refuse,
	// and neither closes anything, which is the property that matters.
	if !opts.Foreign {
		if err := ForeignKillRefusal(s); err != nil {
			return nil, err
		}
	}

	// The landing turn, while the agent is still alive and before anything
	// is closed (ranger-base-qxvh). It is gated on the persona having
	// memory that no commit holds, which is one `git status` and is the
	// difference between a feature that lands what exists and one that
	// taxes every reap for a turn with nothing to say. The cost of that
	// gate is named where it is paid: a session that learned something and
	// never wrote a line of it down is not prompted to.
	//
	// A turn that does not settle inside the bound stops the kill, exactly
	// as it stops a relaunch. The alternative is closing a workspace whose
	// agent may be mid-commit, which is the loss this whole path exists to
	// prevent — and a kill that refuses says so and names both ways
	// through, where a kill that quietly skipped the turn would not.
	if opts.Land && hadMeta && m.Agent != "" && len(b.App.MemoryDirtyPaths(m.Agent)) > 0 {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = DefaultLandTimeout
		}
		w := opts.Out
		if w == nil {
			w = io.Discard
		}
		settled, err := b.landThePlane(w, m, timeout, KillLandingPrompt(m))
		if err != nil {
			return nil, err
		}
		if !settled {
			return nil, Die("NOT killed: %s is still working after %s — let it finish and kill it again (or --timeout %s, or --no-land to close it without the turn)",
				name, timeout, (2 * timeout).String())
		}
	}

	if err := b.H.CloseWorkspace(s.WorkspaceID); err != nil {
		return nil, err
	}
	if !s.Foreign {
		os.Remove(b.metaPath(name))
		b.App.DropPaneLine(name)
		// ADR 0052 D2: the session's rendered hooks dir goes with its other
		// per-session records. Nothing outside the session ever reads it —
		// only that session's env named it — and leaving it would leave a
		// stale render of a wall no live session is behind. A foreign row is
		// another home's session and its dir is that home's to remove.
		b.App.RemoveSessionHooks(name)
	}
	l := &KillLanding{}
	if !hadMeta || s.Foreign {
		return l, nil
	}
	// A session close is one of ADR 0025 §4's fold points: whatever the
	// inner shims wrote to this session's spool after the last sweep, fold
	// it in now rather than leave it for the next pass to find. Best
	// effort, the same as the sweep's own — a fold failure is not a reason
	// to fail a kill that has already closed the workspace.
	if err := b.App.FoldRefusalsSpool(m.Agent, name); err != nil {
		b.warn("fold refusals spool for %s: %v\n", name, err)
	}
	// The commit, after the workspace is closed and so after the last
	// writer to that file is gone. It runs on every kill — --no-land, the
	// cockpit, the auto-reaper — because it is the durability this bead is
	// about and it costs one git process; only the TURN above is optional.
	// A foreign row is somebody else's session and carries no persona of
	// ours, which is why it returns above this line.
	l.Memory = b.App.LandPersonaMemory(m.Agent, "posse kill "+name, m.Bead)
	t := SessionTreeOf(m)
	if t == nil {
		return l, nil
	}
	l.Tree = t
	// Moving the repo's branch is a launcher act (ADR 0011 §1) — the same
	// check-then-act against a store a dispatch pass is also writing. The
	// lock is taken around the landing ONLY, never around the kill: a kill
	// must stay instant, and nothing about closing a workspace is shared.
	//
	// And it is taken WITHOUT waiting. The cockpit's `k` runs this on the
	// TUI's single select loop, where blocking behind a firing pass would
	// freeze the cockpit for as long as the pass holds the lock. Losing the
	// race costs nothing but time: the tree and its branch are kept, the
	// line says so, and `posse worktrees --land` lands it afterwards. What
	// it must never do is merge unserialized.
	lock, why := tryLockLaunches(b.App)
	if lock == nil {
		l.Kept = why + " — not landed; `posse worktrees --land` finishes it"
		return l, nil
	}
	defer lock.Release()
	o, err := MergeSessionWork(t)
	l.Merge = o
	// The third site of ADR 0041 §1–§2 (closeddirty.go) and of the merge-back
	// handoff, and the last chance anything has to write either: a close
	// nobody watched whose branch carried no commits is invisible to the
	// sweep, and this is where its tree is finally read. Before the returns
	// below, because a merge that failed leaves the dirt exactly where it was
	// — and after them the reading is gone. The bead is asked fresh, because
	// a kill lands OPEN beads too (the reap guard refuses that pair only up
	// to `--force`), and both records are about a CLOSED bead's tree: a
	// persona's work in progress is not a finding.
	b.noteUnlandedOnKill(m, t, o)
	if err != nil {
		l.Kept = err.Error()
		return l, nil
	}
	if !o.Merged {
		l.Kept = o.Reason
		return l, nil
	}
	if err := RemoveSessionTree(t, false); err != nil {
		l.Kept = err.Error()
	}
	return l, nil
}

// Lines is everything a kill is worth saying out loud, in the order it
// happened: what became of the persona's memory, then what became of the
// session's tree. Empty when neither had anything to report, which is the
// ordinary kill.
//
// It exists because a kill grew a SECOND thing worth reporting
// (ranger-base-qxvh) and Line() can only carry one. Line() stays what it
// was — the tree's sentence — so every caller and test that asks it that
// question still gets that answer.
func (l *KillLanding) Lines() []string {
	if l == nil {
		return nil
	}
	var out []string
	if ln := l.Memory.Line(); ln != "" {
		out = append(out, ln)
	}
	if ln := l.Line(); ln != "" {
		out = append(out, ln)
	}
	return out
}

// Line is the one sentence a kill's TREE landing is worth saying out loud.
// "" when the session had no tree of its own and there is nothing to report.
func (l *KillLanding) Line() string {
	if l == nil || l.Tree == nil {
		return ""
	}
	t := l.Tree
	switch {
	case l.Kept != "" && len(l.Merge.Equivalent) > 0:
		// The merge found nothing to land and RemoveSessionTree still
		// refused. Since ranger-base-as19 that is every equivalence whose
		// evidence is not a measurement of CONTENT (measuredOnBase), and
		// there are two of those: git's `-x` trailer, which records a
		// human's decision, and a replay paired by author identity, author
		// date and subject (ranger-base-emgdb), which is an identity match.
		// Neither can say what the landing kept, so the branch stays the
		// last copy of its patches. Both facts, in that order, so the
		// refusal reads as the belt it is rather than as lost work
		// (ranger-base-g2xf).
		return fmt.Sprintf("%s KEPT: %s; %s", AbbrevHome(t.Path), l.Merge.EquivalentNote(), l.Kept)
	case l.Kept != "":
		return fmt.Sprintf("%s KEPT: %s", AbbrevHome(t.Path), l.Kept)
	case l.Merge.Commits == 0:
		return fmt.Sprintf("%s had no commits — worktree and %s removed", AbbrevHome(t.Path), t.Branch)
	default:
		how := "fast-forwarded"
		if l.Merge.Rebased {
			how = "rebased and fast-forwarded"
		}
		return fmt.Sprintf("%d commit(s) %s onto %s; worktree and %s removed", l.Merge.Commits, how, t.Base, t.Branch)
	}
}

// FocusSession is the herdr twin of attach: re-aim the herdr UI at the
// session's workspace (meaningful when you're looking at herdr).
func (b *HerdrBackend) FocusSession(name string) error {
	s, err := b.Resolve(name)
	if err != nil {
		return err
	}
	return b.H.FocusWorkspace(s.WorkspaceID)
}

// AgentTarget picks the promptable target for a session: the pane of the
// agent herdr has detected inside its workspace (root pane preferred, for
// sessions with several agents). This is the dispatch loop's addressing.
func (b *HerdrBackend) AgentTarget(name string) (string, error) {
	s, err := b.Resolve(name)
	if err != nil {
		return "", err
	}
	agents, err := b.H.Agents()
	if err != nil {
		return "", err
	}
	var first string
	for _, ag := range agents {
		if ag.WorkspaceID != s.WorkspaceID {
			continue
		}
		if s.PaneID != "" && ag.PaneID == s.PaneID {
			return ag.PaneID, nil
		}
		if first == "" {
			first = ag.PaneID
		}
	}
	if first == "" {
		return "", Die("no agent detected in session %s (herdr sees no claude/codex/... there yet)", name)
	}
	return first, nil
}

// ─── CLI bodies ──────────────────────────────────────────────────────────────

func (b *HerdrBackend) CmdList(w interface{ Write([]byte) (int, error) }) error {
	// This listing RENDERS the permission mode, so it pays for the pane reads
	// (permissionmode.go, ADR 0035 §3). It is set here rather than by the
	// caller because a `posse list` that silently printed `mode:?` for every
	// row would be the blank this column exists to replace.
	was := b.PaneModes
	b.PaneModes = true
	defer func() { b.PaneModes = was }()
	sessions, err := b.Sessions()
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "HERDR SESSIONS\n")
	if len(sessions) == 0 {
		fmt.Fprintf(w, "  (none)\n")
		return nil
	}
	for _, s := range sessions {
		mark := "○"
		if s.Focused {
			mark = "●"
		}
		status := s.Status
		if status == "" {
			status = "-"
		}
		line := fmt.Sprintf("  %s %s %s  %s", mark, s.Emoji, s.Name, status)
		if s.Agent != "" {
			line += "  🎭" + s.Agent + b.App.RuntimeTierModelTag(s.Runtime, s.Tier, s.Model)
			if tag := CageTag(s.Cage, s.Sockets); tag != "" {
				line += "  " + tag
			}
			if s.Degraded != "" {
				line += "  ⚠️degraded"
			}
			if s.TurnFailure != "" {
				line += "  " + TurnFailureTag
			}
			// What the PANE says the session's permission mode is — the
			// compensating control ADR 0035 §3 names, and never a blank: the
			// three unknowns are three different facts and each renders as
			// its own token (PaneMode.Tag). ADR 0035 §4 governs the wording:
			// this names the mode and nothing else, and the status column to
			// its left stays the separate fact about whether it is blocked.
			line += "  " + s.PermissionMode.Tag()
		}
		if s.Crew {
			line += "  " + CrewTag
		}
		if UnpointedBeadSession(s) {
			line += "  " + NoBeadTag
		}
		if s.Envs != "" {
			line += "  🔑" + s.Envs // names only — never values
		}
		if s.Foreign {
			line += "  (herdr)"
		}
		fmt.Fprintf(w, "%s\n", line)
	}
	return nil
}

// LaunchRecipe is the herdr twin of the tmux LaunchRecipe: same messages,
// but slot preferences don't exist here — herdr owns layout.
func (b *HerdrBackend) LaunchRecipe(w interface{ Write([]byte) (int, error) }, rname string) error {
	r, err := b.App.LoadRecipe(rname)
	if err != nil {
		return err
	}
	if b.HasSession(r.Name) {
		fmt.Fprintf(w, "session %s already exists — reusing it\n", r.Name)
		return nil
	}
	err = b.CreateSession(NewSessionOpts{
		Name: r.Name, Dir: r.Dir, Cmd: r.Command,
		Emoji: r.Emoji, Envs: r.EnvFiles, Agent: r.Agent, Runtime: r.Runtime, Tier: r.Tier,
		// A recipe is the operator launching something to sit in (ADR 0008),
		// and one they typed just now (ranger-base-jfe5z).
		Crew: true, ByHand: true,
	})
	if err != nil {
		return err
	}
	m, _ := b.readMeta(r.Name)
	emoji := ""
	if m != nil {
		emoji = m.Emoji
	}
	fmt.Fprintf(w, "launched recipe %s → session %s %s\n", rname, emoji, r.Name)
	fmt.Fprintf(w, "focus it with: posse attach %s\n", r.Name)
	return nil
}
