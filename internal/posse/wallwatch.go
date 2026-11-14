package posse

// `posse wall-watch` (ranger-base-ihnnl, design ranger-base-0qpsw D1+D2):
// the sidecar a bwrap seat's launch line starts in front of bwrap. It holds
// the seat to the list of WALL mounts its render put over a name — Hide,
// Private, Masked, Deny, ReadMask (bwrapArgsWatched) — and when one of them
// leaves the seat's mount table it kills the seat and files a P1 security
// bead.
//
// A mount over a name does not survive a host rename or unlink of that name:
// the kernel detaches it in every namespace, the seat's included, and the
// seat then reaches the host's object there. Nothing in the seat's own view
// says so. The watcher is how it is said.
//
// THE LINE, AND WHY THE WATCHER IS BWRAP'S PARENT. The design asks for the
// watcher "beside" bwrap with the seat pid from --info-fd. It is spelled as
// a wrapper that starts bwrap itself — DIVERGED in shape, kept to the
// reason — because:
//   - the pane line is POSIX shell, and a sibling needs a process
//     substitution to share a pipe with bwrap, which is not;
//   - as the parent it can hand bwrap `--block-fd` as well, so the seat's
//     command does not exec until the watcher has read the seat's mount
//     table and found every wall in it — no window at launch;
//   - bwrap runs with --die-with-parent, so a watcher that dies takes the
//     seat with it. A detector that dies says nothing; this one cannot die
//     and leave its seat running.
//
// THE RESPONSE IS A KILL AND NEVER A REPAIR (D2): the kill closes every fd
// the seat holds, which a re-bind cannot. What D2 accepts as residual is on
// ranger-base-0qpsw; the dispatch backstop is ranger-base-zd4d4.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// WallWatchVerb is argv[1] of the sidecar. An entry point and not a catalog
// verb, for the cage launcher's reason: the pane runs it, nobody types it,
// and it reads no config.
const WallWatchVerb = "wall-watch"

// The kinds of wall a watched mount is. Printed on the bead, so a reader
// knows which part of the render came off.
const (
	WallWatchHide    = "hide"
	WallWatchPrivate = "private"
	WallWatchMasked  = "masked"
	WallWatchDeny    = "deny"
	// WallWatchReadMask is step 5's credential read masks: the ssh dir's
	// tmpfs and the /dev/null binds over another runtime's credential files
	// (ranger-base-w6ip0's decision, cut as ranger-base-y4cm6). Its own kind
	// and not Deny's, because what the reader must do with it differs: a
	// read mask that came off handed the seat a CREDENTIAL in cleartext, so
	// the D2 bead's reviewer judges that credential for ROTATION and not
	// only the writes after the detach time.
	WallWatchReadMask = "read-mask"
)

// wallWatchDedupe is D2's window: a later detection of the same host path
// inside it is a comment on the bead already filed, not a second P1. An
// operator action that renames one shared name reaches every live seat at
// once, and one bead per seat would bury the one fact that matters.
const wallWatchDedupe = 10 * time.Minute

// WallWatchMount is one wall the seat is held to.
type WallWatchMount struct {
	Kind string
	Path string
}

// wallWatchBin is the posse the launch line names. A variable so a render
// pin can name a fixed path; os.Executable in production, the same answer
// the cage launcher's symlink gets.
var wallWatchBin = func() (string, error) { return os.Executable() }

// IsWallWatch says whether this process is the sidecar.
func IsWallWatch(argv []string) bool { return len(argv) > 1 && argv[1] == WallWatchVerb }

// WallWatchPrefix is the launch line's bwrap prefix with the watcher in
// front of the binary:
//
//	3<'<args>' '<posse>' wall-watch --mounts '<mounts>' --seat '<persona>' --dir '<cwd>' -- '<bwrap>' --args 3 -- <runtime cmd>
//
// fd 3 stays the args file, opened by the pane shell exactly as BwrapPrefix
// opens it; the watcher hands it down to bwrap as fd 3.
func WallWatchPrefix(self, mounts, seat, dir, bin, args string) string {
	return "3<" + shellQuote(args) + " " + shellQuote(self) + " " + WallWatchVerb +
		" --mounts " + shellQuote(mounts) + " --seat " + shellQuote(seat) + " --dir " + shellQuote(dir) +
		" -- " + shellQuote(bin) + " --args 3 -- "
}

// wallWatchMountsBytes is the mounts file: kind NUL path NUL per wall, the
// args file's own separator for its reason (bwrapArgsBytes) — a path may
// hold a newline.
//
// EACH PATH IS WRITTEN AS THE KERNEL WILL SPELL IT in the seat's mount
// table: resolved, so a home behind a symlink is compared under the name
// the mount actually sits at. A path that cannot be resolved is written as
// rendered; the watcher's launch check then refuses it loudly rather than
// watching the wrong spelling.
func wallWatchMountsBytes(ms []WallWatchMount) []byte {
	var b bytes.Buffer
	for _, m := range ms {
		p := m.Path
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		b.WriteString(m.Kind)
		b.WriteByte(0)
		b.WriteString(p)
		b.WriteByte(0)
	}
	return b.Bytes()
}

// parseWallWatchMounts reads that file back.
func parseWallWatchMounts(b []byte) ([]WallWatchMount, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if b[len(b)-1] != 0 {
		return nil, Die("wall-watch: the mounts file does not end in a NUL — it is truncated or not one posse wrote")
	}
	f := strings.Split(string(b[:len(b)-1]), "\x00")
	if len(f)%2 != 0 {
		return nil, Die("wall-watch: the mounts file has %d fields, and every wall is two (kind, path)", len(f))
	}
	var out []WallWatchMount
	for i := 0; i < len(f); i += 2 {
		if f[i] == "" || !filepath.IsAbs(f[i+1]) {
			return nil, Die("wall-watch: the mounts file names %q as a %q wall, and every wall is a kind and an absolute path", f[i+1], f[i])
		}
		out = append(out, WallWatchMount{Kind: f[i], Path: f[i+1]})
	}
	return out, nil
}

// wallWatchOpts is the sidecar's argv.
type wallWatchOpts struct {
	Mounts string
	Seat   string
	Dir    string
	Bwrap  []string // the bwrap binary and its own argv, after `--`
}

func parseWallWatchArgv(argv []string) (wallWatchOpts, error) {
	usage := Die("usage: posse %s --mounts <file> --seat <persona> --dir <cwd> -- <bwrap> <bwrap args…>  (rendered on a bwrap seat's launch line, not typed by hand)", WallWatchVerb)
	var o wallWatchOpts
	rest := argv
	if len(rest) > 1 && rest[1] == WallWatchVerb {
		rest = rest[2:]
	}
	for len(rest) > 0 {
		switch rest[0] {
		case "--":
			o.Bwrap = rest[1:]
			rest = nil
			continue
		case "--mounts", "--seat", "--dir":
			if len(rest) < 2 {
				return o, usage
			}
			switch rest[0] {
			case "--mounts":
				o.Mounts = rest[1]
			case "--seat":
				o.Seat = rest[1]
			case "--dir":
				o.Dir = rest[1]
			}
			rest = rest[2:]
		default:
			return o, usage
		}
	}
	if o.Mounts == "" || len(o.Bwrap) == 0 {
		return o, usage
	}
	return o, nil
}

// wallWatchBwrapArgv is bwrap's argv with the two fds the watcher owns put
// in front of the rendered options: the info fd that hands back the seat's
// pid, and the block fd the seat waits on until the watcher has checked its
// walls. fd 3 (the args file) is the pane shell's and passes through.
func wallWatchBwrapArgv(bw []string) []string {
	out := []string{bw[0], "--info-fd", "4", "--block-fd", "5"}
	return append(out, bw[1:]...)
}

// mountTable is one read of /proc/<pid>/mountinfo: each mount point (field
// five), with the kernel's octal escapes (a space is \040) undone so it
// compares against a path as written, mapped to the ids (field one) of the
// mounts at it — more than one where mounts are stacked at the same point.
type mountTable map[string][]string

func parseMountinfo(data []byte) mountTable {
	out := mountTable{}
	for _, ln := range bytes.Split(data, []byte{'\n'}) {
		f := bytes.Fields(ln)
		if len(f) < 5 {
			continue
		}
		p := unescapeMountinfo(string(f[4]))
		out[p] = append(out[p], string(f[0]))
	}
	return out
}

func unescapeMountinfo(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// wallWatchBaseline is the launch read: the ids of the mounts at each
// watched point once the seat is set up, and the watched points with no
// mount at all — a wall that is already off before the command runs.
//
// IDS AND NOT POINTS are what the watch compares from here on. A wall can
// sit on top of another mount at the same point (the seat's /tmp over the
// host's /tmp mount, which `--ro-bind / /` carries in), and a point-only
// diff would read the lower mount as the wall still being there.
func wallWatchBaseline(watch []WallWatchMount, t mountTable) (base map[string][]string, missing []WallWatchMount) {
	base = map[string][]string{}
	for _, m := range watch {
		if _, done := base[m.Path]; done || wallWatchListed(missing, m.Path) {
			continue
		}
		ids := t[m.Path]
		if len(ids) == 0 {
			missing = append(missing, m)
			continue
		}
		base[m.Path] = append([]string(nil), ids...)
	}
	return base, missing
}

// wallWatchDetached is D1's diff: the watched walls any of whose launch
// mounts is no longer at its point, each path once, in list order.
func wallWatchDetached(watch []WallWatchMount, base map[string][]string, t mountTable) []WallWatchMount {
	var out []WallWatchMount
	for _, m := range watch {
		if wallWatchListed(out, m.Path) {
			continue
		}
		now := map[string]bool{}
		for _, id := range t[m.Path] {
			now[id] = true
		}
		for _, id := range base[m.Path] {
			if !now[id] {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

func wallWatchListed(ms []WallWatchMount, p string) bool {
	for _, m := range ms {
		if m.Path == p {
			return true
		}
	}
	return false
}

// wallWatchRun is one watched seat's inputs, every one of them a parameter
// so the live pin can run a real bwrap under it in-process.
type wallWatchRun struct {
	Opts           wallWatchOpts
	Args           *os.File // handed to bwrap as fd 3
	Stdin          *os.File
	Stdout, Stderr io.Writer
	Store          wallWatchStore
	Lock           string
	Signals        bool // relay SIGTERM/SIGHUP to bwrap (production)
	Now            func() time.Time
	OnDetach       func(wallWatchDetach) // the pin's tap; nil in production
	PollInterval   time.Duration
}

// procStatStateParent reads the state and ppid out of /proc/<pid>/stat,
// past the comm field, which is parenthesised and may hold spaces and
// parentheses of its own.
func procStatStateParent(s string) (byte, int, bool) {
	i := strings.LastIndexByte(s, ')')
	if i < 0 || i+4 > len(s) {
		return 0, 0, false
	}
	var state byte
	var ppid int
	if _, err := fmt.Sscanf(s[i+2:], "%c %d", &state, &ppid); err != nil {
		return 0, 0, false
	}
	return state, ppid, true
}

// wallWatchDetach is one detection: what came off, from which seat, when,
// and what the watcher did about it.
type wallWatchDetach struct {
	Seat     string
	Dir      string
	BwrapPid int
	SeatPid  int
	At       time.Time
	Missing  []WallWatchMount
	Killed   []int
	// AtLaunch is a wall that was not there when the watcher first read the
	// table, before the seat's command ran.
	AtLaunch bool
	// KillErr is the kill's own failure, if the seat could not be ended.
	KillErr string
	// Backstop is set when the dispatch watch tick found the detach rather
	// than the seat's own watcher (ranger-base-zd4d4, 0qpsw D3), and says
	// whether that watcher was still alive at the time.
	Backstop string
}

// wallWatchStore is the slice of Bd the filing uses — an interface so the
// dedupe can be pinned without a bd.
type wallWatchStore interface {
	OpenLabeledAny(dir string, labels ...string) ([]BdIssue, error)
	Create(dir string, n BdNew) (string, error)
	Comment(dir, id, text, actor string) error
}

// wallWatchMarker is the whole line the bead for one host path carries, so
// a later detection of the same path finds it.
func wallWatchMarker(path string) string {
	return "wall-watch detached: " + path + " (ranger-base-0qpsw D2)"
}

// wallWatchLockPath is the box-wide lock the find-or-file runs under, so two
// watchers that see the same operator action in the same instant do not
// both find no bead and both file. Beside the suite slots.
func wallWatchLockPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "posse", "wall-watch.lock")
}

// fileWallWatchDetach is D2's record: one P1 security bead per detection,
// unless a bead filed in the last ten minutes already names the same host
// path, in which case the detection is a comment on it. A detection with
// several paths can do both — a comment for the paths already filed, one
// new bead for the rest.
//
// A STORE THAT CANNOT BE READ STILL GETS THE BEAD. The dedupe is a courtesy
// to the reader; a seat that lost a wall and left no record is the failure
// this exists to end, so a twin is the cheap failure.
func fileWallWatchDetach(st wallWatchStore, d wallWatchDetach, now time.Time, lock string) (filed string, commented []string, err error) {
	if lock != "" {
		if mkErr := os.MkdirAll(filepath.Dir(lock), 0o755); mkErr == nil {
			if f, oerr := os.OpenFile(lock, os.O_RDWR|os.O_CREATE, 0o644); oerr == nil {
				defer f.Close()
				_ = flock(f, syscall.LOCK_EX)
			}
		}
	}
	open, rerr := st.OpenLabeledAny(d.Dir, "security")
	byBead := map[string][]WallWatchMount{}
	var fresh []WallWatchMount
	for _, m := range d.Missing {
		best := BdIssue{}
		if rerr == nil {
			for _, is := range open {
				if !landedDescriptionCarries(is.Description, wallWatchMarker(m.Path)) {
					continue
				}
				if now.Sub(is.Created) > wallWatchDedupe {
					continue
				}
				if best.ID == "" || is.Created.After(best.Created) {
					best = is
				}
			}
		}
		if best.ID == "" {
			fresh = append(fresh, m)
			continue
		}
		byBead[best.ID] = append(byBead[best.ID], m)
	}
	var errs []string
	ids := make([]string, 0, len(byBead))
	for id := range byBead {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if cerr := st.Comment(d.Dir, id, wallWatchComment(d, byBead[id]), VerifyActor); cerr != nil {
			errs = append(errs, fmt.Sprintf("comment on %s: %v", id, cerr))
			continue
		}
		commented = append(commented, id)
	}
	if len(fresh) > 0 {
		id, cerr := st.Create(d.Dir, BdNew{
			Title:       wallWatchTitle(d, fresh),
			Description: wallWatchDescription(d, fresh),
			Labels:      []string{"security"},
			Priority:    "1",
			Type:        "bug",
			Actor:       VerifyActor,
		})
		if cerr != nil {
			errs = append(errs, fmt.Sprintf("create: %v", cerr))
		}
		filed = id
	}
	if rerr != nil {
		errs = append(errs, fmt.Sprintf("the open security beads could not be read, so this detection was not deduped (%v)", rerr))
	}
	if len(errs) > 0 {
		err = errors.New(strings.Join(errs, "; "))
	}
	return filed, commented, err
}

func wallWatchPaths(ms []WallWatchMount) string {
	var ps []string
	for _, m := range ms {
		ps = append(ps, AbbrevHome(m.Path))
	}
	return strings.Join(ps, ", ")
}

func wallWatchTitle(d wallWatchDetach, ms []WallWatchMount) string {
	return fmt.Sprintf("security: wall-watch killed seat %s — the mount over %s came off", d.Seat, wallWatchPaths(ms))
}

func wallWatchDescription(d wallWatchDetach, ms []WallWatchMount) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Filed by posse wall-watch (ranger-base-0qpsw D2). A wall this seat's render put over a name left the seat's mount table, so the seat could reach the HOST object at that name from then on. The watcher killed the seat.\n\n")
	fmt.Fprintf(&b, "seat: %s\nsession dir: %s\nbwrap pid: %d · seat pid: %d\ndetached at: %s\n", d.Seat, d.Dir, d.BwrapPid, d.SeatPid, d.At.UTC().Format(time.RFC3339Nano))
	if d.AtLaunch {
		fmt.Fprintf(&b, "when: at launch, before the seat's command ran (it never ran)\n")
	}
	if d.Backstop != "" {
		fmt.Fprintf(&b, "found by: the dispatch watch tick, not the seat's watcher (ranger-base-0qpsw D3) — %s. The detach time above is when the tick SAW it; the detach itself may be up to one tick interval earlier, so review from then.\n", d.Backstop)
	}
	b.WriteString("\nwalls that came off:\n")
	for _, m := range ms {
		fmt.Fprintf(&b, "  - %s (%s)\n", m.Path, m.Kind)
	}
	fmt.Fprintf(&b, "\nkilled: %s\n", wallWatchKilled(d))
	b.WriteString("\nREVIEW: every write to the host object at each path above after the detach time — the seat could make them until the kill. Find what renamed or removed the name; dispatch relaunches the seat behind a fresh render.\n")
	if wallWatchHasKind(ms, WallWatchReadMask) {
		b.WriteString("ROTATE: a " + WallWatchReadMask + " above is what kept a CREDENTIAL unreadable — another runtime's credential file, or the ssh keys this seat may not read. The seat could READ it in cleartext from the detach time until the kill, so judge every credential under those paths for rotation, not only the writes.\n")
	}
	b.WriteString("Later detections of the same path within 10 minutes are comments on this bead.\n\n")
	for _, m := range ms {
		b.WriteString(wallWatchMarker(m.Path))
		b.WriteByte('\n')
	}
	return b.String()
}

func wallWatchHasKind(ms []WallWatchMount, kind string) bool {
	for _, m := range ms {
		if m.Kind == kind {
			return true
		}
	}
	return false
}

func wallWatchComment(d wallWatchDetach, ms []WallWatchMount) string {
	return fmt.Sprintf("wall-watch: seat %s (%s) lost the same wall at %s — %s; killed: %s", d.Seat, AbbrevHome(d.Dir), d.At.UTC().Format(time.RFC3339Nano), wallWatchPaths(ms), wallWatchKilled(d))
}

// ── The backstop (ranger-base-zd4d4, design ranger-base-0qpsw D3) ──────────
//
// The dispatch watch tick runs the SAME diff as the watcher
// (wallWatchDetached) over every live bwrap seat. It can only do that with
// the watcher's launch reading in hand — the diff is by mount id, and the
// ids are the ones the seat had when its command was released — so the
// watcher writes that reading down: one record per seat, in the persona's
// gates dir beside the mounts file the launch handed it, which is a dir no
// seat may write. The watcher removes its record when it returns; a record
// whose watcher is gone is exactly what the backstop is for.
//
// Per record, per tick:
//   - no process left in the seat's mount namespace → the seat is over;
//     the record is dropped once its watcher is gone too;
//   - a wall missing from the seat's table → D2's response, unchanged: the
//     seat is killed and the detach filed (and deduped) as the watcher
//     would have filed it;
//   - walls intact but the watcher gone → ONE bead for that seat, because a
//     detector that dies says nothing (ranger-base-7idk4's lesson), and the
//     tick keeps diffing the seat for as long as it lives.

// wallWatchRecordDir is where a watcher started with this mounts file keeps
// its record: beside the mounts file, so the launch's own dir decides it and
// the sidecar still reads no config.
func wallWatchRecordDir(mounts string) string {
	return filepath.Join(filepath.Dir(mounts), "wall-watch")
}

// wallWatchRecordGlob is every record under one state dir: the gates dir of
// every persona.
func wallWatchRecordGlob(stateDir string) string {
	return filepath.Join(stateDir, "gates", "*", "wall-watch", "*.json")
}

// wallWatchRecord is one watched seat, as its watcher saw it at release.
type wallWatchRecord struct {
	Seat string `json:"seat"`
	Dir  string `json:"dir"`
	// WatcherPid and WatcherStart are the watcher's identity: the start time
	// (/proc/<pid>/stat field 22) is what tells a live watcher from a
	// recycled pid.
	WatcherPid   int    `json:"watcher_pid"`
	WatcherStart string `json:"watcher_start"`
	BwrapPid     int    `json:"bwrap_pid"`
	SeatPid      int    `json:"seat_pid"`
	MntNS        uint64 `json:"mnt_namespace"`
	// Watch and Base are wallWatchDetached's two inputs besides the table.
	Watch    []WallWatchMount    `json:"watch"`
	Base     map[string][]string `json:"base"`
	Released time.Time           `json:"released"`
	// OrphanFiled is the bead the backstop filed when it found this seat
	// running with its watcher gone, so it is filed once per seat.
	OrphanFiled string `json:"orphan_filed,omitempty"`
}

// writeWallWatchRecord writes a record whole or not at all: a reader
// mid-write must never see half of one.
func writeWallWatchRecord(path string, r wallWatchRecord) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readWallWatchRecord(path string) (wallWatchRecord, error) {
	var r wallWatchRecord
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%s is not a wall-watch record: %v", AbbrevHome(path), err)
	}
	if r.MntNS == 0 || r.WatcherPid <= 0 {
		return r, fmt.Errorf("%s names no mount namespace or no watcher, so no seat can be found from it", AbbrevHome(path))
	}
	return r, nil
}

// wallWatchOrphanMarker is the line the dead-watcher bead carries.
func wallWatchOrphanMarker(r wallWatchRecord) string {
	return fmt.Sprintf("wall-watch gone: watcher %d of seat %s (ranger-base-0qpsw D3)", r.WatcherPid, r.Seat)
}

func wallWatchOrphanTitle(r wallWatchRecord) string {
	return fmt.Sprintf("security: the wall-watch of seat %s is gone and the seat still runs", r.Seat)
}

func wallWatchOrphanDescription(r wallWatchRecord, live []int, at time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Filed by the dispatch watch tick (ranger-base-0qpsw D3). This bwrap seat's `posse wall-watch` sidecar is no longer running, and processes in the seat's mount namespace still are. Its walls were all still up when the tick looked, so nothing has been killed.\n\n")
	fmt.Fprintf(&b, "seat: %s\nsession dir: %s\nwatcher pid: %d (gone) · bwrap pid: %d · seat pid: %d\nreleased at: %s\nfound at: %s\nstill running: %s\n",
		r.Seat, r.Dir, r.WatcherPid, r.BwrapPid, r.SeatPid,
		r.Released.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano), wallWatchPidList(live))
	b.WriteString("\nWHAT THIS MEANS: from the moment the watcher ended, a host rename over one of this seat's walls is caught by the watch tick and not within 250 ms — the seat's window is the tick interval. The tick keeps diffing this seat and kills it on a detach as the watcher would have.\n")
	b.WriteString("REVIEW: find what ended the watcher (it does not exit while its seat runs), and end the seat if it should not be running.\n\n")
	b.WriteString(wallWatchOrphanMarker(r))
	b.WriteByte('\n')
	return b.String()
}

func wallWatchPidList(ps []int) string {
	var s []string
	for _, p := range ps {
		s = append(s, strconv.Itoa(p))
	}
	if len(s) == 0 {
		return "none"
	}
	return "pids " + strings.Join(s, " ")
}

func wallWatchKilled(d wallWatchDetach) string {
	if d.KillErr != "" {
		return "NOT CONFIRMED — " + d.KillErr
	}
	var ps []string
	for _, p := range d.Killed {
		ps = append(ps, strconv.Itoa(p))
	}
	if len(ps) == 0 {
		return "nothing was left to kill"
	}
	return "pids " + strings.Join(ps, " ")
}

// wallWatchProbe is what the backstop asks the live process table. An
// interface so the decisions below are pinned without a seat; the Linux
// answer is wallWatchProcProbe.
type wallWatchProbe interface {
	// WatcherAlive: the record's watcher pid is running AND is the process
	// that wrote the record (same start time).
	WatcherAlive(r wallWatchRecord) bool
	// SeatPids: every live process of the seat — in its mount namespace, or
	// descended from one that is.
	SeatPids(r wallWatchRecord) []int
	// Table: the seat's mount table, read through the first of pids that
	// still answers.
	Table(pids []int) (mountTable, error)
	// Kill: end the seat as the watcher's D2 does.
	Kill(r wallWatchRecord) (killed []int, errText string)
}

// wallWatchBackstop is one tick of D3 over every record under stateDir. It
// returns the lines worth printing; a tick that found every seat holding
// its walls under a live watcher returns none.
func wallWatchBackstop(stateDir string, st wallWatchStore, lock string, now func() time.Time, p wallWatchProbe) []string {
	paths, err := filepath.Glob(wallWatchRecordGlob(stateDir))
	if err != nil {
		return []string{fmt.Sprintf("⛨ wall-watch backstop: cannot list the seat records (%v) — no bwrap seat was checked this tick", err)}
	}
	var out []string
	for _, path := range paths {
		r, err := readWallWatchRecord(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // its watcher returned between the glob and the read
			}
			out = append(out, fmt.Sprintf("⛨ wall-watch backstop: %v — that seat is NOT checked", err))
			continue
		}
		alive := p.WatcherAlive(r)
		pids := p.SeatPids(r)
		if len(pids) == 0 {
			// The seat is over. A watcher still alive owns its record and
			// removes it on its way out; a dead one cannot.
			if !alive {
				_ = os.Remove(path)
			}
			continue
		}
		t, terr := p.Table(pids)
		if terr != nil {
			if len(p.SeatPids(r)) == 0 {
				continue // it ended between the census and the read
			}
			out = append(out, fmt.Sprintf("⛨ wall-watch backstop: cannot read seat %s's mount table (%v) — its walls are NOT checked this tick", r.Seat, terr))
			continue
		}
		if gone := wallWatchDetached(r.Watch, r.Base, t); len(gone) > 0 {
			d := wallWatchDetach{Seat: r.Seat, Dir: r.Dir, BwrapPid: r.BwrapPid, SeatPid: r.SeatPid, At: now(), Missing: gone}
			d.Backstop = fmt.Sprintf("its watcher (pid %d) was gone", r.WatcherPid)
			if alive {
				d.Backstop = fmt.Sprintf("its watcher (pid %d) was alive and had not acted", r.WatcherPid)
			}
			d.Killed, d.KillErr = p.Kill(r)
			out = append(out, fmt.Sprintf("⛨ wall-watch backstop: KILLED seat %s — the wall over %s came off; %s (ranger-base-0qpsw D3)", r.Seat, wallWatchPaths(gone), d.Backstop))
			filed, commented, ferr := fileWallWatchDetach(st, d, now(), lock)
			if filed != "" {
				out = append(out, "  filed "+filed)
			}
			for _, id := range commented {
				out = append(out, "  recorded on "+id+" (the same path, inside ten minutes)")
			}
			if ferr != nil {
				out = append(out, fmt.Sprintf("  ⚠ the record is incomplete: %v — this line is the record", ferr))
			}
			if !alive {
				_ = os.Remove(path)
			}
			continue
		}
		if alive || r.OrphanFiled != "" {
			continue
		}
		// Walls intact, watcher gone: say so once for this seat.
		id, cerr := st.Create(r.Dir, BdNew{
			Title:       wallWatchOrphanTitle(r),
			Description: wallWatchOrphanDescription(r, pids, now()),
			Labels:      []string{"security"},
			Priority:    "1",
			Type:        "bug",
			Actor:       VerifyActor,
		})
		if cerr != nil || id == "" {
			out = append(out, fmt.Sprintf("⛨ wall-watch backstop: seat %s runs with its watcher (pid %d) gone, and the bead could not be filed (%v) — this line is the record; the tick keeps diffing the seat", r.Seat, r.WatcherPid, cerr))
			continue
		}
		r.OrphanFiled = id
		line := fmt.Sprintf("⛨ wall-watch backstop: seat %s runs with its watcher (pid %d) gone — filed %s; the tick keeps diffing the seat", r.Seat, r.WatcherPid, id)
		if werr := writeWallWatchRecord(path, r); werr != nil {
			line += fmt.Sprintf(" (⚠ could not mark the record filed, %v — the next tick files again)", werr)
		}
		out = append(out, line)
	}
	return out
}
