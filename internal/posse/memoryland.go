package posse

// Landing persona memory (ranger-base-qxvh): the commit that makes a
// persona's standing orders durable, taken at the one event that would
// otherwise lose them.
//
// THE DEFECT, measured over three readings in one week (ranger-base-s2bq):
// 203 lines uncommitted on 2026-08-25, 1419 on 2026-08-26, 1538 by the time
// a human noticed and landed them by hand. Every persona that runs appends
// lessons to $POSSE_PERSONA_DIR/ORDERS.md — ADR 0015 §5 keeps that write LIVE
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
// which is narrower than the `posse/personas` the parent bead requires and is
// what keeps it clear of `posse/agents` — the constitution, which ADR 0015
// gates behind `posse promote` and which no sweep may ever take. It scans
// what it is about to add for credential shapes and refuses the commit
// rather than publishing one. It never pushes: pushing stays the operator's.
//
// WHAT IT TAKES INSIDE THAT DIR is everything the dir holds, less what the
// persona's own `.gitignore` there excludes — the one EnsureMemoryDir seeds
// (see memoryIgnoreSeed). The memory dir is where a persona works and not
// only where it writes prose, so evidence files accrete in it and were
// landing as standing orders (ranger-base-c9m7). The filter is git's own
// rather than a list of blessed names here, because an allowlist stops
// landing a persona's real notes SILENTLY — measured: four of the 29 files
// tracked under that instance's personas/ are deliberate work that an
// ORDERS.md-and-pending allowlist would have dropped. An ignore is visible
// in the dir it governs and the persona can edit it.

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
//
// THE OTHER RACE, which is the opposite shape and is not self-healing: this
// landing SUCCEEDS and is then quietly un-recorded. Before this function
// existed the shared checkout's HEAD moved only when a human or a persona
// typed a commit; now every kill on the box appends one here, at a moment no
// session can predict. So a history-rewriting git verb in that checkout —
// `commit --amend`, `rebase`, `reset` — is unsafe for anyone: an amend
// rebuilds whatever HEAD is NOW, and what is now may be another persona's
// landing. Path-limiting does not save it, because a pathspec governs what
// is ADDED from the working tree and never what the base tree already holds.
// Nothing of the content is lost when that happens — the blob is identical
// either way — but the commit that said which kill landed those lines is,
// and `git log` then names the wrong persona and the wrong bead.
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
// files (6,107 lines on 2026-09-02, after the 08-30 compaction), and over
// every other file in those persona dirs beside them (7,109 lines in all,
// which is what this scan actually reads): zero matches for every shape
// below, the widened ones included. The earlier 20,200-line figure and the
// one leak canary it counted are both gone with that compaction; the
// added-lines rule below is what stops a canary a persona keeps from
// re-firing on every future commit.
//
// TWO SPELLINGS AND SIX VENDORS (ranger-base-vd1bo). The assigned-secret
// shape carried a \b on each side of the key word. Underscore is a word
// character, so neither ever fires inside an env-var name: GH_TOKEN=,
// AWS_SECRET_ACCESS_KEY=, client_secret= and refresh_token= — the last
// being the field name in the claude credential file itself — all read as
// prose. Both \b are therefore gone and a [A-Za-z0-9_-]* run absorbs
// whatever the name wraps the key word in. The optional quote before the
// separator is the JSON and quoted-YAML form, `"api_key": "…"`, where a
// quote sits between the key and its colon.
//
// The vendor shapes below are the runtimes and services this fleet actually
// reaches — codex, grok, GitHub, Slack, AWS, Linear — none of which spell a
// key sk-ant. Each catches a value pasted BARE, with no key word beside it
// to hang the assigned-secret shape on; that is the shape an env dump takes
// once it has been through a terminal. The sk- shape is deliberately the
// generic one rather than sk-proj-: the same prefix is what Mistral,
// DeepSeek, Together and OpenAI-compatible gateways all use, and it costs
// nothing, measured — the whole widening matches zero lines of the live
// corpus above and zero of the 32,108 lines of markdown in this repo.
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
	{"an assigned secret", regexp.MustCompile(`(?i)(access[_-]?token|api[_-]?key|secret|token|password|passwd)[A-Za-z0-9_-]*["']?\s*[:=]\s*["']?[A-Za-z0-9._~+/=-]{20,}`)},
	{"a private key", regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
	// Bare values, in table order after the shapes above so a line that
	// carries both a key word and a vendor value keeps reporting the
	// assigned secret it always did.
	{"a vendor API key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
	{"an xAI key", regexp.MustCompile(`\bxai-[A-Za-z0-9]{20,}`)},
	// github_pat_ carries no \b on purpose: the character before it is
	// usually the `_` of GITHUB_TOKEN=, and \b does not fire between two
	// word characters — the same defect this bead fixed one line up.
	{"a GitHub token", regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`)},
	{"a GitHub token", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	{"a Slack token", regexp.MustCompile(`\bxox[abeprs]-[A-Za-z0-9-]{20,}`)},
	// ASIA beside AKIA: the STS twin is what a temporary-credential env
	// dump carries, and it is the one a persona is likelier to have.
	{"an AWS access key id", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"a Linear key", regexp.MustCompile(`\blin_api_[A-Za-z0-9]{20,}`)},
}

// memoryDiff is a `git diff` argv with the FORMAT stated on it rather than
// left to whatever configuration the box happens to carry. Both diff arms
// of the scan below parse git's output, so every setting that changes what
// a diff looks like is an input to a credential check — and each of these,
// ALONE, empties the scan while the commit still reports success (measured
// on git 2.50.1, ranger-base-r5wpk):
//
//	--no-color     color.ui / color.diff = always: every body line starts
//	               with an SGR escape, so none of them starts with `+`.
//	--no-ext-diff  diff.external, the GIT_EXTERNAL_DIFF env var, and a
//	               diff.<driver>.command reached through a `diff=<driver>`
//	               line in the memory dir's own .gitattributes: git prints
//	               whatever that program prints, which is not a diff.
//	--no-textconv  diff.<driver>.textconv through the same attribute.
//	--no-relative  diff.relative = true: paths come out relative to the
//	               memory dir, so the binary hold joins one under the
//	               checkout ROOT, fails to stat it, reads a modification
//	               as a deletion and commits it unscanned.
//	--src-prefix   diff.noprefix / diff.mnemonicPrefix / diff.srcPrefix:
//	--dst-prefix   the `+++ b/` header stops saying `b/`, so a refusal
//	               names no file. Attribution only — but the refusal is
//	               the whole product here.
//
// The realistic setter is the operator's global gitconfig, not an attacker:
// a persona that wants a credential into a git object can commit one
// directly. This is a guard against accident, and the accident's shape is
// the worst one available — fail-open under a success line.
//
// One list for both arms on purpose. Only --no-relative is load-bearing for
// numstat today (color, ext-diff and textconv leave it alone, measured);
// the rest cost nothing there and keep the two arms from ever disagreeing
// about which paths exist and which of them are binary.
func memoryDiff(rest ...string) []string {
	return append([]string{"diff",
		"--no-color", "--no-ext-diff", "--no-textconv", "--no-relative",
		"--src-prefix=a/", "--dst-prefix=b/",
	}, rest...)
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
//
// Three arms, and only one of them commits: read the untracked files whole,
// hold on any tracked path whose added content git will not spell out, and
// scan the diff of the rest. The middle arm is not an optimization — a diff
// git declines to write is silently indistinguishable here from a diff with
// nothing in it, and the file passes unscanned.
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
	if held := memoryUnreadableChange(dir); held != "" {
		return held
	}
	// One diff for every tracked path at once. Worktree against HEAD and
	// not against the index, because that is the comparison the commit
	// itself performs — a path-limited commit takes the worktree version
	// — and because nothing has been staged yet: the scan runs BEFORE the
	// add, so a refusal leaves the index as untouched as the file.
	out, err := gitRaw(dir, memoryDiff("HEAD", "--unified=0", "--", ".")...)
	if err != nil {
		return fmt.Sprintf("the credential scan could not read the diff (%v)", err)
	}
	if file, n, what := firstCredShapeInDiff(string(out)); what != "" {
		return fmt.Sprintf("%s:%d looks like %s", file, n, what)
	}
	return ""
}

// memoryUnreadableChange names a tracked path whose added content the diff
// scan below cannot read, or "". It is the tracked half of the arm the
// memoryScanMax bound already gives untracked files: the safe answer to "I
// cannot read it" is to hold the commit and say which file, never to commit
// unscanned bytes.
//
// Git decides binary by CONTENT and not by name — a NUL byte in the first
// 8000 is enough — and for such a path a diff carries no `+` lines at all,
// only "Binary files a/x and b/x differ". So this is not a story about
// `.bin` files: a persona pasting raw terminal capture into its own
// ORDERS.md is one NUL from it, and from that commit on that file is diffed
// as binary and never scanned again.
//
// CONTENT is not the only route, which is why this asks git rather than
// reading the bytes itself. A `.gitattributes` line marking a path `-diff`
// or `binary` gets the same treatment from an ordinary text file — measured
// on git 2.50.1: an ORDERS.md with `-diff` set diffs as "Binary files …
// differ" and counts `-` `-` here. An operator setting that to keep memory
// out of conflict resolution would otherwise have silently turned the
// credential scan off for the one file it exists for.
//
// `--numstat -z` is git saying it in the one form that needs no unquoting:
// a binary path's two counts are both `-`, and `-z` emits the path raw
// where the default format C-quotes an odd byte and spells a rename
// `old => new`. A rename spends the two names on the NEXT two fields
// instead, which have to be consumed whatever the counts say — left in
// place they would be read as records of their own.
//
// A DELETION counts `-` `-` as well, and a deleted file adds nothing, so it
// must not hold: what separates the two is the working tree, where the file
// a deletion names is gone. That reading is the only thing standing between
// this arm and passing every binary file, so a checkout root it cannot
// resolve holds rather than skips. A pure rename of a binary file holds too,
// since nothing here can tell it from a rename that also edited the bytes.
func memoryUnreadableChange(dir string) string {
	out, err := gitRaw(dir, memoryDiff("HEAD", "--numstat", "-z", "--", ".")...)
	if err != nil {
		return fmt.Sprintf("the credential scan could not read the diff (%v)", err)
	}
	// Resolved lazily: the overwhelming majority of memory dirs hold no
	// binary change at all, and this would otherwise spend a git process on
	// every kill to answer a question nothing asks.
	root := ""
	recs := strings.Split(string(out), "\x00")
	for i := 0; i < len(recs); i++ {
		f := strings.SplitN(recs[i], "\t", 3)
		if len(f) != 3 {
			continue // the empty tail the last NUL leaves
		}
		path := f[2]
		if path == "" { // a rename or a copy: the source, then the target
			if i+2 >= len(recs) {
				break
			}
			path = recs[i+2]
			i += 2
		}
		if f[0] != "-" || f[1] != "-" {
			continue
		}
		if root == "" {
			if root = memoryRepoRoot(dir); root == "" {
				// Without the root, nothing below can tell a modification
				// from a deletion — and of the two answers available then,
				// the silent one is the one this bead is about.
				return fmt.Sprintf("%s could not be located for the credential scan, so it was not checked for credentials", path)
			}
		}
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			continue // it adds nothing because it is gone; git will say so
		}
		return fmt.Sprintf("%s is binary to git, so the scan could not read what it adds and it was not checked for credentials", path)
	}
	return ""
}

// memoryRepoRoot is the checkout the memory dir sits in, which is where
// git's repo-relative paths are rooted. "" if it cannot be found, and the
// two callers answer that differently ON PURPOSE, so read this before
// copying either: memoryUnreadableChange HOLDS, because it cannot otherwise
// tell a binary modification from a deletion. The untracked arm above
// CONTINUES — the relative path it is left with does not stat, and a path
// that does not stat is skipped — and a skipped untracked file is then
// committed with nothing scanned. That is a fail-open, it predates
// ranger-base-38a1, and it is handed to the fleet's security persona rather
// than widened into that bead. The window is narrow: memoryChanges has just
// run a successful `git status` from the same dir, so a rev-parse that fails
// here means the checkout went away mid-kill.
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
//
// `--- ` and `+++ ` are read as headers only where the format puts them:
// between a `diff --git` line and the first `@@` hunk of that file. Prefix
// alone is not enough, because a diff renders an added line by prepending
// `+` — so a memory line that itself begins with `++ ` arrives here as
// `+++ …`, and taking that for the header half meant the one line carrying
// the credential was the one line never scanned (ranger-base-txd57).
func firstCredShapeInDiff(diff string) (file string, line int, what string) {
	hunk := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)
	var cur string
	var n int
	header := false // between `diff --git` and this file's first hunk
	for _, ln := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(ln, "diff --git "):
			// The one line the format puts a header after. `cur` and `n`
			// are not reset here on purpose: every file whose diff carries
			// a `+` line also carries the `+++ b/` and `@@` that set them,
			// so a reset would be a line no fixture can reach.
			header = true
		case header && strings.HasPrefix(ln, "+++ b/"):
			cur = strings.TrimPrefix(ln, "+++ b/")
		case header && (strings.HasPrefix(ln, "+++ ") || strings.HasPrefix(ln, "--- ")):
			// the other header half, and /dev/null for a deletion
		case strings.HasPrefix(ln, "@@ "):
			header = false
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
