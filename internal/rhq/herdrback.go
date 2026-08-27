package rhq

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
	"fmt"
	"io"
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
	// AllowDegraded launches even when the parity check finds gates no wall
	// layer realizes on this runtime × cage; the session is marked degraded.
	// Never set by dispatch on its own (ADR 0002 §4).
	AllowDegraded bool
	Cage          string // cage tier override (CLI --cage / dispatch) — over the PID's cage:
	// Crew marks the session as one the operator made to talk to, so
	// dispatch leaves it alone (ADR 0008). `posse new` and recipes set it;
	// dispatch's own CreateSession never does.
	Crew bool
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
	return &HerdrBackend{App: a, H: NewHerdr(), ClaudeConfig: ClaudeConfigFile(), Bd: NewBd()}
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
	Dir         string    // working directory the session was created in (seatbelt re-render on relaunch)
	Repo        string    // the main checkout Dir is a session worktree of ("" = Dir IS the checkout)
	Branch      string    // the session branch the launcher merges back (rangerhq-09o2; "" = no worktree)
	Cmd         string    // raw --cmd, sessions without a persona only (persona lines are re-rendered, never replayed)
	Cage        string    // cage tier the session got (ADR 0002 §4)
	Sockets     string    // container: host sockets the PID declared and the cage mounted ("" = none)
	Degraded    string    // "; "-joined gates the wall does not realize here ("" = full parity)
	Fallback    string    // the availability preflight's line when the tier did not get its model ("" = it did) — rangerhq-oay
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
}

// SocketID names the herdr server a session belongs to: $HERDR_SOCKET_PATH,
// or "" for herdr's default socket. It is recorded in the meta so a pass
// talking to a *different* herdr — a named session, a scratch server, a
// socket exported for one command — can tell "this workspace died" apart
// from "I am asking the wrong server" (rangerhq-snd).
func SocketID() string { return os.Getenv("HERDR_SOCKET_PATH") }

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
	p := herdrSocketPath()
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

// herdrSocketPath names the api socket file posse's herdr calls go to, or ""
// when it cannot be named for certain. The asymmetry is deliberate: an
// unnameable socket costs the generation fence (unknown, so nothing is
// trusted), while a wrongly named one would forge it.
//
// HERDR_SOCKET_PATH is exact. Without it herdr resolves the socket itself,
// and the two layouts it uses are ones this shop has measured rather than
// guessed: ~/.config/herdr/herdr.sock for the default server, and
// ~/.config/herdr/sessions/<name>/herdr.sock for a named session
// (rangerhq-6bg7's scratch server ran on one). A HERDR_SESSION with no
// socket path is therefore resolved, not defaulted — pointing at the default
// server's socket there would fence against a server posse is not talking to.
//
// This is NOT rangerhq-y4z: the socket *comparison* in cannotAnswerFor still
// reads SocketID() exactly as it did, and "" still compares unequal to the
// default path there. This resolves a path to stat, nothing else.
func herdrSocketPath() string {
	if p := SocketID(); p != "" {
		return ExpandTilde(p)
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

func (b *HerdrBackend) metaDir() string { return filepath.Join(b.App.StateDir, "herdr") }

func (b *HerdrBackend) metaPath(name string) string {
	return filepath.Join(b.metaDir(), name+".yaml")
}

func (b *HerdrBackend) readMeta(name string) (*HerdrMeta, bool) {
	p := b.metaPath(name)
	if _, err := os.Stat(p); err != nil {
		return nil, false
	}
	return &HerdrMeta{
		Name:        name,
		Workspace:   YamlGet(p, "workspace"),
		Pane:        YamlGet(p, "pane"),
		Emoji:       YamlGet(p, "emoji"),
		Envs:        YamlGet(p, "envs"),
		Agent:       YamlGet(p, "agent"),
		Runtime:     YamlGet(p, "runtime"),
		Tier:        YamlGet(p, "tier"),
		Dir:         YamlGet(p, "dir"),
		Repo:        YamlGet(p, "repo"),
		Branch:      YamlGet(p, "branch"),
		Cmd:         YamlGet(p, "cmd"),
		Cage:        YamlGet(p, "cage"),
		Sockets:     YamlGet(p, "sockets"),
		Degraded:    YamlGet(p, "degraded"),
		Fallback:    YamlGet(p, "fallback"),
		TurnFailure: YamlGet(p, "turn_failure"),
		Bead:        YamlGet(p, "bead"),
		Crew:        YamlGet(p, "crew") == "true",
		Socket:      YamlGet(p, "socket"),
		Gen:         YamlGet(p, "gen"),
		Launched:    parseLaunched(YamlGet(p, "launched")),
		Prompted:    parseLaunched(YamlGet(p, "prompted")),
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
	// What the account would not serve, and what ran instead (rangerhq-oay).
	// A session whose tier silently became a cheaper model is the one thing
	// the listing could not show, and the tier: above already names the
	// substitute — this is the line that says it was a substitute.
	if m.Fallback != "" {
		fmt.Fprintf(&s, "fallback: %s\n", m.Fallback)
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
	return os.WriteFile(b.metaPath(m.Name), []byte(s.String()), 0o644)
}

// ─── the crew marker (ADR 0008) ───────────────────────────────────────────────

// CrewTag is what a crew session wears in `posse list` and the cockpit.
const CrewTag = "👤"

// FallbackTag is what a session wears when the model its tier named was not
// available on the account and the launch substituted another (ADR 0003 §1,
// rangerhq-oay). It rides BESIDE the @runtime/tier tag rather than replacing
// it, because that tag already names the substitute: the session really is
// running at what it says, and this is the mark that says it was not asked
// for. `fallback:` in the session meta carries the whole line.
const FallbackTag = "⤵️fallback"

// TurnFailureTag marks a live session whose provider refused its dispatch
// turn before model work began. Herdr's idle/done status remains true of the
// CLI process; this tag carries the separate work outcome.
const TurnFailureTag = "🛑turn-failed"

// EnvPersona is set in every persona session's env by CreateSession: its
// presence in *posse's own* env means posse was run by a persona, not by the
// operator.
const EnvPersona = "RHQ_PERSONA"

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

// MarkCrew records that the operator just started a conversation with this
// session (cockpit `p`). Best effort by design: a prompt must never fail
// over its marker, and a foreign workspace has no meta to mark.
func (b *HerdrBackend) MarkCrew(name string) {
	if m, ok := b.readMeta(name); ok && !m.Crew {
		m.Crew = true
		_ = b.writeMeta(m)
	}
}

// MarkCrewOnOperatorPrompt is `posse prompt`'s half of the same rule: the
// operator's shell has no RHQ_PERSONA, a persona's session does. So a
// person starting a conversation marks the session crew, and a persona
// handing work to another persona (an orchestrator persona) marks nothing —
// otherwise the dispatch primitive would quietly retire the fleet.
func (b *HerdrBackend) MarkCrewOnOperatorPrompt(name string) {
	if os.Getenv(EnvPersona) != "" {
		return
	}
	b.MarkCrew(name)
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

func (b *HerdrBackend) metaNames() []string {
	ents, _ := os.ReadDir(b.metaDir())
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			out = append(out, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	return out
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
	Cage        string // persona sessions: cage tier
	Sockets     string // persona sessions: host sockets the cage mounted ("" = none)
	Degraded    string // persona sessions: gates the wall does not realize ("" = full parity)
	Fallback    string // persona sessions: the tier's model was unavailable and this is what ran instead ("" = it got what it asked for)
	TurnFailure string // provider refused the dispatch turn before model work began
	Bead        string // the bead dispatch launched this session to work ("" = not a dispatched bead session; ADR 0013 §4)
	Crew        bool   // the operator's own session — dispatch skips it entirely (ADR 0008)
	Dir         string // working directory (from meta; "" for foreign sessions)
	Repo        string // the checkout Dir is a session worktree of ("" = Dir IS the checkout; rangerhq-09o2)
	Agent       string
	Status      string // herdr agent state; "" when no agent detected
	Focused     bool
	Foreign     bool // exists in herdr but wasn't created by posse
}

// Sessions merges herdr's live workspace list with the meta files. Meta
// files pointing at dead workspaces are pruned; foreign workspaces are
// listed under their label so the cockpit shows the whole herd.
func (b *HerdrBackend) Sessions() ([]HerdrSession, error) {
	wss, err := b.H.Workspaces()
	if err != nil {
		return nil, err
	}
	byID := map[string]HerdrWorkspace{}
	for _, ws := range wss {
		byID[ws.WorkspaceID] = ws
	}

	// herdr reports workspace agent_status "unknown" even for plain shells;
	// only show a status when herdr actually detects an agent in there.
	agents, err := b.H.Agents()
	if err != nil {
		return nil, err
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
	var spared []string    // missing from the listing, but not proven dead — with why
	var strangers []string // in the listing under an id another workspace now holds
	var recipes []string   // metas naming no workspace: the session is gone, its recipe kept
	var out []HerdrSession
	claimed := map[string]bool{} // workspace ids owned by a meta file
	for _, name := range b.metaNames() {
		m, ok := b.readMeta(name)
		if !ok {
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
			if why := notOurWorkspace(m, ws, gen); why != "" {
				strangers = append(strangers, fmt.Sprintf("%s: %s", name, why))
				continue
			}
		}
		if !live {
			// m.Socket == "" and an empty listing are the prune's own arms,
			// and only the prune's: refusing to DELETE costs a kept file, so
			// they are worth paying for the pre-field metas and the wrong-
			// server board they cover. The write cannot pay either — see
			// mustNotOrphan (rangerhq-jeu2, rangerhq-7dn4).
			if m.Socket == "" || emptyBoard(sock, len(wss)) != "" || cannotAnswerFor(m, sock) != "" {
				kept++
				continue
			}
			dead, why := b.prunable(m, gen)
			if !dead {
				spared = append(spared, fmt.Sprintf("%s: %s", name, why))
				continue
			}
			// Proven dead — by evidence gathered OUTSIDE the launcher lock,
			// which is a fact about the instant it was read and not about
			// the instant the unlink lands (rangerhq-3a5t). reclaim re-proves
			// it under the lock, where no create can be in flight.
			if why := b.reclaim(name, sock, gen); why != "" {
				spared = append(spared, fmt.Sprintf("%s: %s", name, why))
			}
			continue
		}
		claimed[m.Workspace] = true
		b.backfillServer(m, ws, sock, gen)
		out = append(out, HerdrSession{
			Name: name, WorkspaceID: m.Workspace, PaneID: m.Pane,
			Emoji: m.Emoji, Envs: m.Envs, Agent: m.Agent, Runtime: m.Runtime, Tier: m.Tier,
			Cage: m.Cage, Sockets: m.Sockets, Degraded: m.Degraded, Fallback: m.Fallback, TurnFailure: m.TurnFailure, Bead: m.Bead, Crew: m.Crew, Dir: m.Dir,
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
	if len(recipes) > 0 {
		b.warn("posse: %d session(s) closed without a replacement, recipe kept: %s — rebuild with `posse relaunch <name>`, or delete %s to discard\n",
			len(recipes), strings.Join(recipes, ", "), b.metaDir())
	}
	sortHerdrSessions(out)
	return out, nil
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
// The prune keeps two classes this does not name, and both are applied at
// its call site rather than here, because on the write side neither is true
// (see mustNotOrphan): a meta with no socket recorded at all, and an EMPTY
// workspace listing. Refusing to delete costs a kept file, which the next
// listing takes back; refusing to write costs the name.
func cannotAnswerFor(m *HerdrMeta, sock string) string {
	if m.Socket != sock {
		return fmt.Sprintf("the meta was written against %s and this pass is talking to %s", socketLabel(m.Socket), socketLabel(sock))
	}
	return ""
}

// emptyBoard is the prune's second own arm: this herdr listed no workspaces
// at all. It is the belt gilfoyle added with the socket field itself after a
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
//   - the LABEL. posse creates every workspace with --label <session name>
//     (CreateWorkspace) and the meta's filename is that name, so a workspace
//     whose label is not the meta's name is not the meta's workspace. Not
//     conclusive on its own: renaming a workspace in herdr breaks the label
//     without changing whose workspace it is.
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
func notOurWorkspace(m *HerdrMeta, ws HerdrWorkspace, gen string) string {
	if ws.Label == "" || ws.Label == m.Name {
		return ""
	}
	if gen != "" && m.Gen == gen {
		return "" // same server generation: ids are not re-issued, so this is a rename
	}
	return fmt.Sprintf("workspace %s is labelled %q, not %q, and %s — herdr re-issues workspace ids across a server restart or handoff (rangerhq-6bg7)",
		m.Workspace, ws.Label, m.Name, genLabel(m.Gen, gen))
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
	if stranger := notOurWorkspace(m, ws, gen); stranger != "" {
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
	lock, ok := tryLockLaunches(b.App)
	if !ok {
		return "a launcher holds the launch lock, so the unlink is not taken now (ADR 0011 §1, rangerhq-3a5t) — the next quiet pass prunes it"
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
	if m.Socket == "" {
		return "it now records no socket, and nothing on disk says this server ever held it"
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
// Only a pass that knows a concrete socket can stamp one: SocketID() == ""
// means herdr's default server, which on disk cannot be told apart from
// "nothing recorded" — that identity is rangerhq-y4z's to fix, and until it
// does, a meta whose session was created outside a herdr pane stays
// unstamped, and so unprunable.
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

func socketLabel(sock string) string {
	if sock == "" {
		return "default socket"
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
	// TWO arms of the prune's are deliberately not taken here, for the same
	// reason twice: a refusal costs the two halves different things, and only
	// this half can lose a name.
	//
	// An unstamped meta is refused there and asked about here, when this pass
	// is unstamped too. It is not the copy-paste gap it looks like. Every
	// create stamps Socket: SocketID(), so socket: "" says the meta was
	// written by a pass that was itself on the default server — the server
	// being asked. That is evidence the two name the SAME server, not absence
	// of evidence, and reading it as two servers is rangerhq-y4z's misfire.
	// On the prune side that misfire costs a kept file. Here it would cost
	// every name: `posse` from a plain terminal has HERDR_SOCKET_PATH unset,
	// so it writes and reads unstamped metas, and a dead session's name could
	// never be reused without deleting its meta by hand. A meta unstamped
	// against a NAMED socket still refuses — that is the mismatch arm. What
	// stays open is the pre-field legacy meta on a multi-server board;
	// rangerhq-y4z closes it by making "" and the default path one server, at
	// which point this arm can be taken verbatim.
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
	if b.HasSession(name) {
		return Die("session '%s' already exists (try: posse attach %s)", name, name)
	}
	return b.mustNotOrphan(name)
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
	Cage     string
	Sockets  string
	Degraded string
	Fallback string // the availability preflight's line ("" = the tier got the model it asked for)
}

// planLaunch resolves a launch without touching herdr: persona, runtime,
// tier, cage, parity, skills, seatbelt, gates, env sets, working directory.
// Every refusal a launch can raise for reasons that are knowable in advance
// is raised here. Its side effects are the renders a launch always redoes
// (gates, skills, seatbelt profiles, the memory dir) — idempotent by
// design, so planning twice costs nothing and changes nothing.
func (b *HerdrBackend) planLaunch(o NewSessionOpts) (*launchPlan, error) {
	a := b.App

	// The load guard (ranger-base-innx) is first, before this function's
	// renders and before anything is asserted about the home, because the
	// question it answers — can this box still fork? — decides whether the
	// rest is worth doing. Every launch path funnels through here: `posse
	// new`, a recipe, a cockpit key, dispatch, and relaunch, which plans
	// before it kills (rangerhq-v52t) and so refuses with the old session
	// still alive rather than losing it to a box that cannot start the
	// replacement.
	if why := a.LoadHigh(b.warnWriter()); why != "" {
		return nil, Die("%s — refusing to launch %s into a saturated box; wait for it to drain, or set config load_guard: 0 to launch anyway",
			why, o.Name)
	}
	a.TightenEnvPerms(os.Stderr) // every launch re-asserts 700/600 on envs/

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
			return nil, Die("%s\n  dispatch refuses to launch on a constitution nobody promoted (ADR 0015 §3)\n"+
				"  the operator clears it with: posse promote", v.Line())
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
	if o.Worktree {
		t, err := a.EnsureSessionTree(dir, o.Name, b.warnWriter())
		if err != nil {
			return nil, err
		}
		if t != nil {
			dir, repo, branch = t.Path, t.Repo, t.Branch
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
	runtime, tier, gatesDir, cage, degraded, fallback := "", "", "", "", "", ""
	if o.Agent != "" {
		var err error
		if ag, err = a.LoadAgent(o.Agent); err != nil {
			return nil, err
		}
		if err := ag.EnsureMemoryDir(); err != nil {
			return nil, err
		}
		// Runtime: CLI/recipe override > PID runtime: > config default >
		// claude. The PID's own command: applies only on its own runtime.
		own := a.ResolveRuntime("", ag)
		runtime = a.ResolveRuntime(o.Runtime, ag)
		rt, err = a.LoadRuntime(runtime)
		if err != nil {
			return nil, err
		}
		tier = a.ResolveTier(o.Tier, ag)
		if !ValidTier(tier) {
			return nil, Die("unknown tier %q (strong | standard | fast)", tier)
		}
		// Availability preflight (rangerhq-oay, modelavail.go): the tier is
		// resolved, so the model id it names is known — ask whether this
		// account can actually run it, and substitute per `tier_fallback:`
		// when it cannot. Loud, never silent, and never a refusal of its
		// own (rule 3). It runs BEFORE the parity check on purpose: what
		// the wall and the PID's tier_floor: must rule on is the pair that
		// would really launch, not the one that was asked for.
		if pf := a.TierPreflight(o.Agent, runtime, tier, b.warnWriter()); pf.Fell() {
			fallback = pf.Line
			b.warn("posse: %s\n", pf.Line)
			runtime, tier = pf.Runtime, pf.Tier
			if rt, err = a.LoadRuntime(runtime); err != nil {
				return nil, err
			}
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
		if deniesGitPush(ag.Deny) {
			InstallPrePushHook(dir)
		}
		a.InstallCommitGuardHook(dir)
		parity := a.CheckParityIn(ag, rt, cage, tier, dir)
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
		// The writable roots a self-sandboxing runtime is told about: the
		// store of record when a redirect moves it, and — in a session
		// worktree — the git dirs that hold this tree's index and the
		// repo's objects, which sit outside the tree (rangerhq-09o2).
		cmd = ag.RenderCommandFor(rt, own, tier, append([]string{beadsHome(dir)}, LinkedGitDirs(dir)...)...)
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
		if cage == CageSeatbelt && AvailableCages[CageSeatbelt] && !rt.SelfSandbox {
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

	// Env sets: explicit ones (--env-file, recipe env_files) always; the
	// persona's own `envs:` list on top for persona sessions. Config
	// default_env is applied only to sessions without a persona — an env
	// set is readable by the agent in that session (and by every tool it
	// runs), so what an autonomous persona receives must be an explicit
	// choice, never a silent default (rangerhq-f2b).
	envs := append([]string(nil), o.Envs...)
	if ag != nil {
		envs = append(envs, ag.Envs...)
	} else if len(envs) == 0 {
		if defenv := a.CfgGet("default_env", ""); defenv != "" {
			if _, err := os.Stat(filepath.Join(a.EnvsDir, defenv+".env")); err == nil {
				envs = []string{defenv}
			}
		}
	}
	envs = dedupeStrings(envs)
	var vars []EnvVar
	for _, n := range envs {
		vs, err := a.EnvSetVars(n)
		if err != nil {
			return nil, err
		}
		vars = append(vars, vs...)
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

	// RHQ_HOME rides every session, persona or crew, because any rhq/bd
	// tool run inside resolves its instance from this var (falling back to
	// ~/.config/rhq otherwise) — a second RHQ_HOME's session addressing the
	// wrong instance's config, queue, and skills silently (ADR 0012 §D2,
	// rangerhq-ysly). Appended after the env-set vars so this instance's
	// identity is authoritative over anything an env file happened to set.
	vars = append(vars, EnvVar{"RHQ_HOME", a.Home})

	if ag != nil {
		// The persona's durable identity rides the environment: BD_ACTOR
		// makes every bd call inside the session attribute to the persona
		// (claims, closes, mail), and the memory dir is there for tooling.
		vars = append(vars,
			EnvVar{"BD_ACTOR", o.Agent},
			EnvVar{EnvPersona, o.Agent},
			EnvVar{"RHQ_PERSONA_DIR", ag.MemoryDir},
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
	return &launchPlan{
		Dir: dir, Repo: repo, Branch: branch, Cmd: cmd, Emoji: emoji, Envs: envs, Vars: vars,
		Runtime: runtime, Tier: tier, Cage: cage, Sockets: sockets, Degraded: degraded,
		Fallback: fallback,
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
	wsID, rootPane, err := b.H.CreateWorkspace(o.Name, p.Dir, p.Vars)
	if err != nil {
		return "", err
	}
	meta := &HerdrMeta{
		Name: o.Name, Workspace: wsID, Pane: rootPane,
		Emoji: p.Emoji, Envs: strings.Join(p.Envs, "+"), Agent: o.Agent, Runtime: p.Runtime, Tier: p.Tier,
		Dir: p.Dir, Repo: p.Repo, Branch: p.Branch,
		Cage: p.Cage, Sockets: p.Sockets, Degraded: p.Degraded, Fallback: p.Fallback, Bead: o.Bead, Crew: o.Crew,
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
// launch, start it.
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
	// The name's SYNTAX is checked outside the lock, and it is the one
	// check that can be: it reads no state, so no concurrent actor can
	// change its answer. Taking the launcher lock to reject `posse new -x`
	// would queue a typo behind a running pass and leave a lock file in an
	// RHQ_HOME nothing has launched in yet (cmd/posse: help and a refused
	// name leave no state behind). Everything below it reads the meta dir.
	if err := nameSyntax(o.Name); err != nil {
		return err
	}
	return underLaunchLock(b.App, b.warnWriter(), func() error {
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
	inner := ag.RenderCommandFor(rt, b.App.ResolveRuntime("", ag), tier)
	if m.Cage == CageSeatbelt && AvailableCages[CageSeatbelt] && !rt.SelfSandbox {
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
	cmd := inner
	if m.Cage == CageContainer && b.App.ContainerAvailable() {
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
		// No prompt file: a relaunch restarts a persona whose CLI died, it
		// does not re-dispatch the bead (ADR 0013 §2 — resume is a typed
		// prompt into a live session, never a second argv delivery).
		if cmd, err = b.App.WrapInCage(ag, rt, m.Name, m.Dir, inner, CageEnvNames(vars), ""); err != nil {
			return false, err
		}
	} else if cmd, _, _, err = b.App.WrapWithGates(m.Agent, rt, ag.Deny, inner); err != nil {
		return false, err
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

// runtimeTierTag is the listing suffix for a persona session: "" for the
// defaults (claude/strong), else "@runtime/tier" so the cockpit shows what
// the persona runs on and at what spend (ADR 0002/0003).
func RuntimeTierTag(runtime, tier string) string {
	if (runtime == "" || runtime == DefaultRuntime) && (tier == "" || tier == DefaultTier) {
		return ""
	}
	if runtime == "" {
		runtime = DefaultRuntime
	}
	if tier == "" {
		tier = DefaultTier
	}
	return "@" + runtime + "/" + tier
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

// Checkout is the repo a session's work belongs to: the checkout its own
// worktree hangs off, or its working directory when it shares one.
func (s *HerdrSession) Checkout() string {
	if s.Repo != "" {
		return s.Repo
	}
	return s.Dir
}

// KillLanding is what a kill did with a session's own git worktree
// (rangerhq-09o2). Nil Tree means the session shared the checkout — every
// session before per-session worktrees, and every crew session — and there
// was nothing of its own to land.
type KillLanding struct {
	Tree  *SessionTree
	Merge MergeOutcome
	Kept  string // why the worktree was left in place ("" = it was removed)
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
	return b.killAndLand(name, false)
}

// ForceKillSessionAndLand is the same kill with the ADR 0013 §4 reap guard
// stood down: the operator has read the refusal and said kill it anyway.
// Nothing else changes — the landing still refuses to destroy a tree that
// holds work (RemoveSessionTree), so a forced kill of a dirty session
// worktree still keeps the tree and says so.
func (b *HerdrBackend) ForceKillSessionAndLand(name string) (*KillLanding, error) {
	return b.killAndLand(name, true)
}

func (b *HerdrBackend) killAndLand(name string, force bool) (*KillLanding, error) {
	// Before anything: the meta is the only record of which tree and branch
	// belong to this session, and the kill below deletes it (ADR 0011 §3).
	m, hadMeta := b.readMeta(name)

	// The reap guard, before the workspace is closed and before the meta is
	// dropped: a session still holding an open bead over an uncommitted
	// tree is not killed (ADR 0013 §4, reapguard.go). It runs on the shared
	// checkout too, which is where the near-miss was — there is no session
	// branch there for the landing below to refuse over.
	if hadMeta && !force {
		if why := b.ReapRefusal(m); why != "" {
			return nil, Die("NOT killed: %s — look first (posse attach %s), or `posse kill %s --force`", why, name, name)
		}
	}

	s, err := b.Resolve(name)
	if err != nil {
		return nil, err
	}
	if err := b.H.CloseWorkspace(s.WorkspaceID); err != nil {
		return nil, err
	}
	if !s.Foreign {
		os.Remove(b.metaPath(name))
		b.App.DropPaneLine(name)
	}
	if !hadMeta || s.Foreign {
		return &KillLanding{}, nil
	}
	t := SessionTreeOf(m)
	if t == nil {
		return &KillLanding{}, nil
	}
	l := &KillLanding{Tree: t}
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
	lock, ok := tryLockLaunches(b.App)
	if !ok {
		l.Kept = "a launcher is running — not landed; `posse worktrees --land` finishes it"
		return l, nil
	}
	defer lock.Release()
	o, err := MergeSessionWork(t)
	l.Merge = o
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

// Line is the one sentence a kill's landing is worth saying out loud. ""
// when the session had no tree of its own and there is nothing to report.
func (l *KillLanding) Line() string {
	if l == nil || l.Tree == nil {
		return ""
	}
	t := l.Tree
	switch {
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
			line += "  🎭" + s.Agent + RuntimeTierTag(s.Runtime, s.Tier)
			if tag := CageTag(s.Cage, s.Sockets); tag != "" {
				line += "  " + tag
			}
			if s.Degraded != "" {
				line += "  ⚠️degraded"
			}
			if s.Fallback != "" {
				line += "  " + FallbackTag
			}
			if s.TurnFailure != "" {
				line += "  " + TurnFailureTag
			}
		}
		if s.Crew {
			line += "  " + CrewTag
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
		// A recipe is the operator launching something to sit in (ADR 0008).
		Crew: true,
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
