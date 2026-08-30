package rhq

// Landing persona memory (ranger-base-qxvh): the commit that makes a
// persona's standing orders durable, taken at the one event that would
// otherwise lose them.
//
// THE DEFECT, measured over three readings in one week (ranger-base-s2bq):
// 203 lines uncommitted on 2026-08-25, 1419 on 2026-08-26, 1538 by the time
// a human noticed and landed them by hand. Every persona that runs appends
// lessons to $RHQ_PERSONA_DIR/ORDERS.md — ADR 0015 §5 keeps that write LIVE
// and ungated on purpose, "on the agents to manage themselves" — and nothing
// ever committed the result. So the backlog rebuilt itself from zero the
// moment it was cleared.
//
// Persona memory is the one artifact with NO other copy. A bead has the
// queue behind it and code has git; a lesson appended to ORDERS.md exists in
// exactly one place, on disk in a shared checkout, until something commits
// it. ~30 sessions were reaped in a single day on this instance and each one
// was a chance to lose it.
//
// WHY POSSE COMMITS IT AND NOT THE PERSONA. All three of the actors that
// could plausibly have done it were measured and cannot (ranger-base-s2bq —
// do not re-derive):
//
//   - a seatbelt-caged persona could not write the file at all (fixed
//     separately, 23c4e54);
//   - the bd daemon exports its jsonl on a timer and never commits anything,
//     measured 10+ minutes stale;
//   - a persona in its own worktree — the common case, since worktrees are
//     default-on — has the auto-mode classifier refuse a content commit
//     outside the session's own cwd, and the memory dir is outside it.
//
// So the WRITE stays the persona's (§5 is a ruling, not an oversight; the
// landing turn only ever ASKS) and the COMMIT is the launcher's. That split
// is the whole mechanism.
//
// WHAT IT WILL NOT DO. It is path-limited to one persona's own memory dir,
// which is narrower than the `rhq/personas` the parent bead requires and is
// what keeps it clear of `rhq/agents` — the constitution, which ADR 0015
// gates behind `posse promote` and which no sweep may ever take. It scans
// what it is about to add for credential shapes and refuses the commit
// rather than publishing one. It never pushes: pushing stays the operator's.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// MemoryLanding is what a kill did with a persona's memory. Nil means there
// was nothing to do — no persona, no memory dir, no git repo around it, or
// nothing in it that a commit does not already hold — which is the quiet
// majority of kills and says nothing out loud.
type MemoryLanding struct {
	Persona string
	Dir     string   // the memory dir, as the home spells it
	Paths   []string // repo-relative paths the commit took, or would have
	SHA     string   // short sha of the commit ("" when none was made)
	Held    string   // a credential shape stopped it; the file is untouched
	Failed  string   // git refused; the file is untouched
}

// memoryScanMax bounds what the credential scan will read out of one file.
// Persona memory is prose — the largest ORDERS.md on the instance that
// motivated this is under 200KB — so a file past this bound is not the thing
// this feature is for, and the safe answer to "I cannot read it all" is to
// hold the commit and say which file, rather than to commit unscanned bytes.
const memoryScanMax = 4 << 20

// memoryChange is one path a commit of this memory dir would take.
type memoryChange struct {
	Path      string // repo-relative, as git spells it
	Untracked bool   // git has never seen it, so the whole file is new content
}

// MemoryDirtyPaths lists what a persona's memory holds that no commit does.
//
// Empty covers four shapes that all mean "nothing to land", and it is
// deliberately not an error in any of them: the persona has no memory dir
// yet, the dir is clean, the home keeps `personas/` outside git at all (the
// default install — posse must not require the operator to have made one a
// checkout), or git could not answer. It is one git process, which is what
// licenses asking it on the kill path before deciding to spend a turn.
func (a *App) MemoryDirtyPaths(persona string) []string {
	var paths []string
	for _, c := range a.memoryChanges(persona) {
		paths = append(paths, c.Path)
	}
	return paths
}

func (a *App) memoryChanges(persona string) []memoryChange {
	// ValidName is where the path-limiting stops being a convention and
	// becomes a fact. Everything below rests on the memory dir being UNDER
	// PersonasDir, and filepath.Join resolves `..` cheerfully: a persona
	// named `../agents` joins to the constitution, and this would then run
	// git from inside it and commit `.` — the one thing the parent bead
	// states in capitals, reached without a single unscoped git command.
	// LoadAgent does not ask (it only needs a file to exist), so this asks.
	// It is posse's own name predicate and not a new one, so a name this
	// refuses is a name `posse agent new` would never have made.
	if !ValidName(persona) {
		return nil
	}
	dir := filepath.Join(a.PersonasDir(), persona)
	// `-z` and not the plain porcelain, because these paths are read back
	// as filenames and not just shown: the default format QUOTES a path
	// with an odd byte in it and collapses an untracked directory to
	// `dir/`, and both would send the scan below at a name no file has.
	// `--untracked-files=all` is what un-collapses it.
	//
	// The pathspec is `.` against a git run from INSIDE the memory dir —
	// queuejsonl.go's trick — so the scope is this one persona's directory
	// without this code having to work out where the repo starts. That is
	// also the whole path-limiting story: nothing outside the dir can enter
	// this list, so nothing outside it can enter the commit.
	out, err := gitRaw(dir, "status", "--porcelain", "--untracked-files=all", "-z", "--", ".")
	if err != nil {
		return nil
	}
	return porcelainZChanges(out)
}

// porcelainZChanges reads `status --porcelain -z` records. A record is two
// status columns, a space, then the path — so four bytes is the shortest
// real one and anything shorter is the empty tail the last NUL leaves.
//
// A rename or a copy spends a SECOND record on its source path: `-z` drops
// the ` -> ` spelling the human format uses and emits the two names as
// separate fields. Consuming that field is not tidiness — left in place it
// would be read as a record whose first four bytes are a path's, and the
// scan would then be aimed at a filename made of somebody's directory name.
func porcelainZChanges(out []byte) []memoryChange {
	recs := strings.Split(string(out), "\x00")
	var changes []memoryChange
	for i := 0; i < len(recs); i++ {
		r := recs[i]
		if len(r) < 4 {
			continue
		}
		if r[0] == 'R' || r[0] == 'C' {
			i++ // the source path, not a record of its own
		}
		changes = append(changes, memoryChange{Path: r[3:], Untracked: strings.HasPrefix(r, "??")})
	}
	return changes
}

// LandPersonaMemory commits a persona's standing orders where they live.
//
// why is the event that occasioned it, and it goes in the commit message
// because a commit nobody can trace to a cause is the next person's
// mystery: this is the launcher writing on a persona's behalf, at a moment
// the persona is being destroyed, and the message is the only place that is
// legible afterwards.
//
// It returns what it did rather than an error, and the caller is a kill:
// nothing here is a reason not to kill a session. A commit that cannot be
// made leaves the file exactly where it was — which is no worse than every
// build before this one — and the line it returns is how that gets said out
// loud instead of silently.
//
// It takes no launcher lock, and does not need one. Two kills landing two
// personas' memory into one checkout at the same moment contend for git's
// own index.lock, and the loser reports git's refusal and leaves its
// persona's file untouched — which the NEXT kill of that persona lands,
// because the evidence is the file itself and nothing consumed it. The
// failure is self-healing in the direction that matters: memory is never
// destroyed by losing this race, only deferred. Serializing it under the
// launcher lock would instead put every kill behind whatever pass holds it,
// for a store the launcher lock says nothing about.
func (a *App) LandPersonaMemory(persona, why, bead string) *MemoryLanding {
	changes := a.memoryChanges(persona)
	if len(changes) == 0 {
		return nil
	}
	dir := filepath.Join(a.PersonasDir(), persona)
	l := &MemoryLanding{Persona: persona, Dir: dir}
	for _, c := range changes {
		l.Paths = append(l.Paths, c.Path)
	}
	if held := scanMemoryChanges(dir, changes); held != "" {
		l.Held = held
		return l
	}
	// The add is not optional and it is not `git add -A`: a path git has
	// never seen has no index entry, so the path-limited commit below
	// matches nothing and fails with "did not match any file(s) known to
	// git" (rangerhq-4pbt, and AGENTS.md says it in the same words). Scoped
	// to `.` from inside the memory dir, it can stage nothing else.
	if _, err := git(dir, "add", "--", "."); err != nil {
		l.Failed = err.Error()
		return l
	}
	subject := fmt.Sprintf("memory: land %s's standing orders (%s)", persona, why)
	body := "Committed by posse on the persona's behalf: a session was closed and its\n" +
		"memory had lines no commit held. ADR 0015 §5 leaves the WRITE to the persona;\n" +
		"the commit is the launcher's, because a persona in its own worktree cannot\n" +
		"make it (ranger-base-qxvh).\n"
	if bead != "" {
		body += "\nBead: " + bead + "\n"
	}
	// Path-limited, and in a checkout that may be shared with every other
	// persona: this form takes the WORKING TREE version of the paths it
	// names and ignores whatever anyone else has staged (ADR 0022, measured
	// on git 2.39.3 in ranger-base-nor). The bare form would take their
	// work under this message.
	if _, err := git(dir, "commit", "-m", subject, "-m", body, "--", "."); err != nil {
		l.Failed = err.Error()
		return l
	}
	sha, err := git(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		l.Failed = err.Error()
		return l
	}
	l.SHA = sha
	return l
}

// Line is the one sentence a memory landing is worth saying out loud, or ""
// when there is nothing to say. Every arm names the persona, because the
// session being killed is not always called after it.
func (l *MemoryLanding) Line() string {
	switch {
	case l == nil:
		return ""
	case l.Held != "":
		return fmt.Sprintf("%s memory NOT committed: %s — read it and land it by hand", l.Persona, l.Held)
	case l.Failed != "":
		return fmt.Sprintf("%s memory NOT committed: %s", l.Persona, l.Failed)
	default:
		return fmt.Sprintf("%s memory committed %s (%s)", l.Persona, l.SHA, dirtyList(l.Paths))
	}
}

// ─── the credential scan ─────────────────────────────────────────────────────

// memoryCredShapes is the scan a human ran by hand over the 1538-line batch
// that motivated this, carried here so the mechanism does not lose it: the
// shapes grepped for were sk-ant keys, JWTs, bearer tokens, accessToken and
// password assignments, and PEM private keys.
//
// Every pattern needs a credential-shaped VALUE and not just the word.
// Persona memory is prose ABOUT this system, so the words themselves are
// everywhere in it — a scan keyed on "password" or "Bearer" would hold the
// commit on a sentence, and a hold that fires on prose is the original
// defect wearing a safety label. Measured against all fifteen live ORDERS
// files (20,200 lines): zero matches, save one deliberate leak canary
// recorded verbatim in a persona's own notes, which is a true positive by
// shape and exactly what the added-lines rule below stops re-firing on.
var memoryCredShapes = []struct {
	What string
	Re   *regexp.Regexp
}{
	{"an Anthropic key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{16,}`)},
	// Both segments, because a JWT's header and payload are both base64 of
	// a `{"` and so both begin `eyJ`. One segment alone matches ordinary
	// base64 and would fire on any pasted blob.
	{"a JWT", regexp.MustCompile(`eyJ[A-Za-z0-9_=-]{8,}\.eyJ[A-Za-z0-9_=-]{8,}\.`)},
	{"a bearer token", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{20,}`)},
	{"an assigned secret", regexp.MustCompile(`(?i)\b(access_?token|api_?key|secret|token|password|passwd)\b\s*[:=]\s*["']?[A-Za-z0-9._~+/=-]{20,}`)},
	{"a private key", regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
}

// scanMemoryChanges looks at what this commit would ADD and nothing else,
// and returns why it must not be made, or "".
//
// ADDED LINES, not whole files, and that is the load-bearing choice. A
// persona's memory legitimately quotes credential SHAPES — the fleet's
// security persona keeps a leak canary in its own notes, spelled out — and
// those lines are already committed. Scanning whole files would hold that
// persona's every future commit forever on prose git has held for weeks,
// which is the defect this feature exists to end, arrived at from the other
// side. The delta is also exactly what the human check it replaces looked
// at: what is new since the last landing.
//
// It never echoes what it matched. The whole point is that the bytes are
// suspected of being a credential, and a refusal that prints them has
// published it into a terminal, a log and this process's own output.
func scanMemoryChanges(dir string, changes []memoryChange) string {
	for _, c := range changes {
		if !c.Untracked {
			continue
		}
		// A path git has never seen has no HEAD side to diff against, so
		// the whole file is added content and is read from disk.
		full := filepath.Join(memoryRepoRoot(dir), c.Path)
		st, err := os.Stat(full)
		switch {
		case err != nil:
			continue // it went away between the status and here; git will say so
		case st.Size() > memoryScanMax:
			return fmt.Sprintf("%s is %d bytes, past the %d the scan reads, so it was not checked for credentials",
				c.Path, st.Size(), memoryScanMax)
		}
		body, err := os.ReadFile(full)
		if err != nil {
			return fmt.Sprintf("%s could not be read for the credential scan (%v)", c.Path, err)
		}
		if what, n := firstCredShape(strings.Split(string(body), "\n")); what != "" {
			return fmt.Sprintf("%s:%d looks like %s", c.Path, n, what)
		}
	}
	// One diff for every tracked path at once. Worktree against HEAD and
	// not against the index, because that is the comparison the commit
	// itself performs — a path-limited commit takes the worktree version
	// — and because nothing has been staged yet: the scan runs BEFORE the
	// add, so a refusal leaves the index as untouched as the file.
	out, err := gitRaw(dir, "diff", "HEAD", "--unified=0", "--", ".")
	if err != nil {
		return fmt.Sprintf("the credential scan could not read the diff (%v)", err)
	}
	if file, n, what := firstCredShapeInDiff(string(out)); what != "" {
		return fmt.Sprintf("%s:%d looks like %s", file, n, what)
	}
	return ""
}

// memoryRepoRoot is the checkout the memory dir sits in, which is where
// git's repo-relative paths are rooted. "" if it cannot be found, which
// leaves the join above pointing at a relative path that will not stat —
// and a path that does not stat is skipped, not committed unscanned.
func memoryRepoRoot(dir string) string {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return root
}

// firstCredShape is the scan over plain lines, 1-indexed like an editor.
func firstCredShape(lines []string) (string, int) {
	for i, ln := range lines {
		for _, s := range memoryCredShapes {
			if s.Re.MatchString(ln) {
				return s.What, i + 1
			}
		}
	}
	return "", 0
}

// firstCredShapeInDiff walks a unified diff and scans only its `+` lines,
// keeping the file and the NEW line number so the refusal points at
// something an operator can open.
func firstCredShapeInDiff(diff string) (file string, line int, what string) {
	hunk := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)
	var cur string
	var n int
	for _, ln := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(ln, "+++ b/"):
			cur = strings.TrimPrefix(ln, "+++ b/")
		case strings.HasPrefix(ln, "+++ "), strings.HasPrefix(ln, "--- "):
			// the other header half, and /dev/null for a deletion
		case strings.HasPrefix(ln, "@@ "):
			if m := hunk.FindStringSubmatch(ln); m != nil {
				n, _ = strconv.Atoi(m[1])
			}
		case strings.HasPrefix(ln, "+"):
			body := ln[1:]
			for _, s := range memoryCredShapes {
				if s.Re.MatchString(body) {
					return cur, n, s.What
				}
			}
			n++
		}
	}
	return "", 0, ""
}
