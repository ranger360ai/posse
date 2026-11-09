package treepins

// QA pin for ranger-base-uxadf — the census door for the hash spelling: a
// shipped runnable that hashes must pick its hasher by PROBE, or say in this
// file why the box it runs on always has the one it names.
//
// WHY A CENSUS AT ALL. ranger-base-sydnc was `shasum` not being on the
// cutover host's PATH. Arch — what ADR 0080's always-on Linux host runs —
// ships it with perl at /usr/bin/core_perl/shasum, off the default PATH, and
// carries coreutils' `sha256sum` instead; ubuntu and macOS carry both, which
// is why CI and the laptop were green over it for as long as either script
// had existed. Two shipped scripts then hashed everything to the EMPTY
// STRING and compared the empty strings EQUAL: a detective control that
// printed `fresh` over a planted stale hook, and a test wrapper that ran a
// stale binary and reported its result. Neither `set -e` nor `set -o
// pipefail` catches that class — a command substitution whose command is not
// found yields the empty string and the status of the assignment it feeds is
// discarded.
//
// Both are FIXED AND LANDED — ranger-base-sydnc, commit e2499fcc, with
// behaviour pins (internal/treepins/hashportability_qa_test.go) that run them
// on a PATH with no `shasum`. This file shipped with them in hshPending
// because e2499fcc had not landed when it was written and uxadf's own commit
// (972338dc) was still unpushed; sydnc landed first, the pending arm reddened
// over both rows exactly as it is built to, and ranger-base-0z9ta deleted
// them. hshPending is empty and stays here for the next finding whose fix is
// in flight.
//
// This file is the cheap second door either way: behaviour pins hold the
// scripts they name, and the next script to hash is not one of them.
//
// THE RULE. Over the shipped runnables — cgScannedDirs, the one spelling of
// that directory list, held complete by
// TestEveryShippedRunnableFileIsInsideTheCredentialScan — every runnable line
// naming `shasum` or `sha256sum` must sit inside a `command -v` probe for the
// hasher, and a file whose lines all do must name BOTH spellings. A probe
// for one spelling with no fallback is the bug with a conditional in front of
// it. Anything else is a finding unless this file declares the path, with a
// reason, in hshExempt or hshPending.
//
// WHAT THIS DOOR CANNOT SEE, so that nobody reads it as more than it is: the
// probe's third arm. `shasum` absent AND `sha256sum` absent must be a loud
// stop and never a value, because an empty hash must not compare equal to
// another empty hash — and that is behaviour, not text. The fix landed under
// sydnc has that arm in both scripts, and its pins run them on a box with
// neither. scripts/release-artifacts.sh is a two-way probe with no such arm,
// and that is NOT the sydnc class: that script runs under `set -eu`, so the
// function's 127 propagates out of the `(cd "$OUT" && sum … >checksums.txt)`
// subshell and the script dies there, naming the command it could not find —
// exit 127, `shasum: command not found`, and a zero-byte checksums.txt the
// redirection had already created. MEASURED 2026-11-09, bash 3.2.57 on
// darwin, by running that `sum()` definition and that redirection under
// `set -e` against a PATH holding neither spelling. Loud is the whole
// difference from the sydnc class, which was silent and said `fresh`. No bead
// is filed on it. This census is green over all of that on purpose: it
// asks where the spelling is decided, not what the decision does.
//
// NOT IN THE SUBJECT, measured 2026-11-09 and named here because the next
// reader will grep and find them. etc/cleanroom/Dockerfile.{arch,debian,
// fedora,rhel} each run `sha256sum -c -`, bare, and are right to: those lines
// execute INSIDE a Linux image the Dockerfile itself builds, where
// `sha256sum` is coreutils and cannot be absent. They are not runnables of
// this repo by cgRunnable's definition — no `.sh` extension, no shebang — so
// they are outside the walk rather than exempted by it. docs/runbooks/
// release.md names both spellings and says which box each belongs to; it is
// prose, and prose that quotes a command is prose.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// The two spellings. Word-bounded, so `sha256_file` and `sha256_stdin` — the
// wrapper names the fixed scripts call from their measuring paths — are not
// mentions of a hasher. That is the whole point of the fix: after it, the
// hasher's name appears only where the spelling is chosen.
var hshHasher = regexp.MustCompile(`\b(shasum|sha256sum)\b`)

// `command -v` is the probe. Not `which`, not `type`, not `hash`: one
// spelling, and it is the one all three probing scripts in this tree use.
// A new script that probes some other way is a finding whose remedy is to
// spell it the way its neighbours do.
var hshCommandV = regexp.MustCompile(`\bcommand\s+-v\b`)

// Shared with brewguard's reader in shape and for its reason: a `<<` inside a
// quoted string opens a heredoc that is not there, and everything to the
// terminator is then read as a body. That is the strict direction here —
// mentions in a body are still taken, and taken as UNPROBED.
var hshHeredoc = regexp.MustCompile(`<<(-?)\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

// hshMention is one runnable line naming a hasher, and whether the reader
// placed it inside a probe.
type hshMention struct {
	line   int
	text   string
	probed bool
}

func (m hshMention) String() string { return fmt.Sprintf("%d: %s", m.line, m.text) }

// hshWords splits a line into shell-ish words. `$` and `=` are word
// characters here on purpose, so `$fi` and `fi=1` are the tokens "$fi" and
// "fi=1" and neither is the keyword `fi` — the structure reader below counts
// tokens equal to `if` and `fi`, and a variable named after a keyword must
// not move the depth, whether it is being read or assigned. `elif` is its own
// token for the same reason, which is why it is matched by name rather than
// by a word-bounded `if`.
func hshWords(code string) []string {
	return strings.FieldsFunc(code, func(r rune) bool {
		return !(r == '_' || r == '$' || r == '=' || unicode.IsLetter(r) || unicode.IsDigit(r))
	})
}

func hshCount(words []string, want string) int {
	n := 0
	for _, w := range words {
		if w == want {
			n++
		}
	}
	return n
}

func hshHas(words []string, want string) bool { return hshCount(words, want) > 0 }

// hshRead returns one mention per runnable line of `path` that names a
// hasher, and `lost` naming the first line at which the structure reader
// stopped being able to place anything.
//
// THE ONE THING IT DECIDES is whether a mention sits inside a probe region: a
// conditional whose head names `command -v` together with a hasher, from that
// head to its matching `fi`. Depth is counted ONLY inside such a region, from
// zero — never across the whole file — so an `if` or a `fi` in a quoted
// string somewhere else in the script cannot move it. A region that never
// closes sets `lost`, and from there on every mention is reported unprobed,
// because a reader that has lost the structure must not be the thing that
// says a bare call was fine.
func hshRead(path string) (mentions []hshMention, lost string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	hereTerm, hereDash := "", false
	depth := 0 // >0 only inside a probe region
	for i, line := range strings.Split(string(b), "\n") {
		code := strings.TrimSpace(line)
		add := func(probed bool) {
			if hshHasher.MatchString(code) {
				mentions = append(mentions, hshMention{line: i + 1, text: code, probed: probed})
			}
		}
		if hereTerm != "" {
			// A body is data this script hands to something else. Its
			// mentions are taken and never probed; its words are not
			// structure.
			if line == hereTerm || (hereDash && code == hereTerm) {
				hereTerm = ""
			} else {
				add(false)
			}
			continue
		}
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		words := hshWords(code)
		if m := hshHeredoc.FindStringSubmatch(code); m != nil {
			hereTerm, hereDash = m[2], m[1] == "-"
		}
		if depth > 0 {
			// Inside a region: this line is probed whatever it says, and
			// its own openers and closers move the depth.
			add(true)
			depth += hshCount(words, "if") - hshCount(words, "fi")
			if depth < 0 {
				depth = 0
			}
			continue
		}
		head := hshCommandV.MatchString(code) && hshHasher.MatchString(code) &&
			(hshHas(words, "if") || hshHas(words, "elif"))
		if !head {
			add(false)
			continue
		}
		add(true)
		// A one-line probe (`sum() { if ...; else ...; fi; }`) opens and
		// closes on its own line, and the region is that line alone. An
		// `elif` head opens no `if` of its own — the enclosing conditional
		// did — so it starts at one and is closed by that conditional's `fi`.
		depth = hshCount(words, "if") - hshCount(words, "fi")
		if hshHas(words, "elif") && !hshHas(words, "if") {
			depth++
		}
		if depth < 0 {
			depth = 0
		}
	}
	if depth > 0 {
		lost = fmt.Sprintf("%s: a `command -v` probe region opened and never closed", path)
		for j := range mentions {
			mentions[j].probed = false
		}
	}
	return mentions, lost, nil
}

// hshScan walks the shipped runnables under `root` and returns every file
// with at least one hasher mention, plus the number of runnable files it
// opened. The count is the positive witness: an assertion over the files
// that hash is satisfied by a walk that read none, and a walk that read none
// is silent about it.
func hshScan(t *testing.T, root string, dirs ...string) (found map[string][]hshMention, lost []string, scanned int) {
	t.Helper()
	found = map[string][]hshMention{}
	for _, d := range dirs {
		err := filepath.Walk(filepath.Join(root, d), func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || !cgRunnable(path) {
				return nil
			}
			scanned++
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)
			ms, badness, readErr := hshRead(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}
			if badness != "" {
				lost = append(lost, badness)
			}
			if len(ms) > 0 {
				found[rel] = ms
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", filepath.Join(root, d), err)
		}
	}
	sort.Strings(lost)
	return found, lost, scanned
}

// hshExempt: paths whose bare hasher call stays, and why the box that runs
// them always has the spelling they name. Not a waiver — each row has to
// answer the question the census asks, which is "where does this run".
var hshExempt = map[string]string{
	"scripts/macos-install-probe.sh":          "darwin-only by construction and by assertion: `[ \"$(uname -s)\" = Darwin ] || die` at :161, and exit 2 — nothing measured — on anything else. `shasum` is in /usr/bin on every macOS this probe can run on. It is also on no roster: `make macos-install-probe` is the only route to it, on demand before a release, and neither `make test` nor scripts/verify-box.sh reaches it.",
	"docs/adr/0059-packed-refs-lock.probe.sh": "a hand-run ADR instrument, darwin-only by what it measures: every arm runs under `sandbox-exec` with a hand-rendered seatbelt profile, which exists on no other platform. Its `shasum < packed-refs` reads a digest to compare against itself inside one run, on this box, and no Makefile target or script runs this file.",
	"docs/adr/0076-flag-trust.probe.sh":       "a hand-run ADR instrument for one amendment's measurement on this box (`sh docs/adr/0076-flag-trust.probe.sh [seed] [flag] [userhook] [ask]`, from the repo root, by the operator). It hashes one config.toml before and after a run to say whether the file moved; both digests come from the same `shasum` in the same run, so there is no second box for the spelling to differ on. No Makefile target or script runs it.",
	"docs/adr/0076-reporter.probe.sh":         "the same instrument class as 0076-flag-trust.probe.sh and the same reading — a before/after digest of one codex home's config.toml within a single hand-run — and nothing in this tree runs it.",
	"docs/adr/0076-trust-ask.probe.sh":        "the same instrument class as 0076-flag-trust.probe.sh and the same reading — a before/after digest of one home's config.toml within a single hand-run — and nothing in this tree runs it.",
}

// hshPending: a finding whose fix is WRITTEN AND UNLANDED. Each row names the
// bead that holds the fix, and the row's job is to delete itself: the census
// below fails over a pending path that is no longer a finding, so landing
// that bead reds this file and the remedy is to drop the row.
//
// This is not hshExempt and must never be read as it. An exempt path has an
// answer to "where does this run"; a pending path has a bug, measured, with a
// fix in flight. A row here that outlives its bead is this door being held
// open, and the arm that deletes it is the one that says so.
//
// It is EMPTY today, and that is the mechanism having worked rather than
// never having been used: the two rows it shipped with — scripts/gotest.sh
// and scripts/verify-hook-freshness.sh, the ranger-base-sydnc finding — were
// deleted under ranger-base-0z9ta when e2499fcc landed and this arm reddened
// over them.
var hshPending = map[string]string{}

// hshFile is the path this file's findings point the reader at.
const hshFile = "internal/treepins/hashspelling_qa_test.go"

// THE CENSUS.
func TestQAEveryShippedScriptThatHashesPicksItsHasherByProbe(t *testing.T) {
	t.Parallel()
	found, lost, scanned := hshScan(t, ".", cgScannedDirs...)

	// Reader sanity before the arms trust it. The floor sits well under the
	// live count (77 runnable files under scripts/, plugin/, etc/ and docs/,
	// MEASURED 2026-11-09) so an ordinary landing does not move it.
	if scanned < 50 {
		t.Fatalf("the walk of %v opened only %d runnable file(s) — the walk is broken, not the tree, and a clean result here would not be evidence", cgScannedDirs, scanned)
	}
	// And the other half of the same witness: this repo hashes. A census of
	// hash call sites that finds none has stopped reading, and the reason it
	// would stop is the one `\b` in hshHasher.
	if len(found) < 3 {
		t.Fatalf("the census found hasher mentions in only %d file(s) — this repo hashes in more places than that, so the matcher is asleep: %v", len(found), found)
	}
	for _, l := range lost {
		t.Errorf("%s — the structure reader cannot place this file's probe, so every mention in it is reported UNPROBED and it will read as a finding until the region closes. Fix the script, or if the reader is wrong, fix it in %s and add the shape to its control arm.", l, hshFile)
	}

	paths := make([]string, 0, len(found))
	for p := range found {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		ms := found[path]
		exemptReason, isExempt := hshExempt[path]
		pendingReason, isPending := hshPending[path]
		if isExempt && isPending {
			t.Errorf("%s is in hshExempt AND hshPending in %s — one place, one answer, and these two say opposite things about whether it is a bug", path, hshFile)
			continue
		}
		var bare []string
		for _, m := range ms {
			if !m.probed {
				bare = append(bare, m.String())
			}
		}
		if isExempt {
			if len(strings.TrimSpace(exemptReason)) < 60 {
				t.Errorf("%s is exempted from the hash-spelling census with no reason worth reading (%q) — an exemption nobody can check is the door being dropped quietly", path, exemptReason)
			}
			if len(bare) == 0 {
				t.Errorf("hshExempt names %s and every hasher mention in it is already inside a probe — delete the row rather than leave a reason nobody will check again", path)
			}
			continue
		}
		if isPending {
			if len(strings.TrimSpace(pendingReason)) < 60 {
				t.Errorf("%s is in hshPending with no reason worth reading (%q) — a pending row that does not name the bead holding the fix is an exemption with a nicer word on it", path, pendingReason)
			}
			if len(bare) == 0 {
				t.Errorf("hshPending names %s and every hasher mention in it now sits inside a probe — THE FIX LANDED. Delete the row: that is what this arm is for, and leaving it is this door held open for a path that no longer needs it.\n  the row says: %s", path, pendingReason)
			}
			continue
		}
		if len(bare) > 0 {
			t.Errorf("%s calls a hasher by name on %d runnable line(s) outside any `command -v` probe:\n  %s\n"+
				"`shasum` is NOT on every box's PATH — Arch carries `sha256sum` and keeps shasum off it (ranger-base-sydnc) — and a command substitution whose command is not found yields the EMPTY STRING with the status of the assignment discarded, which `set -e` and `set -o pipefail` both let through. So a bare call does not fail here; it compares two empty strings EQUAL, or keys a cache on nothing.\n"+
				"Pick the spelling once by probe, the way scripts/release-artifacts.sh:375 and the two scripts hashportability_qa_test.go runs do:\n"+
				"  if command -v sha256sum >/dev/null 2>&1; then sha256_file() { sha256sum \"$1\"; }\n"+
				"  elif command -v shasum >/dev/null 2>&1; then sha256_file() { shasum -a 256 \"$1\"; }\n"+
				"  else <stop loudly — a digest that did not happen is an error, never a value>\n"+
				"  fi\n"+
				"— and call the wrapper from everywhere else. If this file runs only on a box that always has the spelling it names, say WHICH box and WHY in hshExempt in %s.",
				path, len(bare), strings.Join(bare, "\n  "), hshFile)
			continue
		}
		// Every mention probed. Then the probe has to be a probe: one that
		// only ever looks for `shasum` is the ranger-base-sydnc bug with a
		// conditional in front of it, and one that only looks for
		// `sha256sum` is the same bug facing macOS.
		var names string
		for _, m := range ms {
			names += m.text + "\n"
		}
		for _, want := range []string{"shasum", "sha256sum"} {
			if !regexp.MustCompile(`\b` + want + `\b`).MatchString(names) {
				t.Errorf("%s decides its hasher inside a `command -v` probe and never names %q — a probe with no fallback is not a probe. One of the two spellings is missing on some box the fleet runs on: `sha256sum` on macOS, `shasum` on Arch (ranger-base-sydnc). Lines read:\n  %s", path, want, strings.TrimSpace(names))
			}
		}
	}

	// Stale rows in the other direction: a declared path the census no
	// longer finds at all.
	for _, decl := range []struct {
		name string
		rows map[string]string
	}{{"hshExempt", hshExempt}, {"hshPending", hshPending}} {
		for p := range decl.rows {
			if _, ok := found[p]; !ok {
				t.Errorf("%s names %s and the census finds no hasher mention in it — the file stopped hashing, was renamed, or moved out of %v. Delete the row; a declaration over nothing is also how this census would go blind without saying so.", decl.name, p, cgScannedDirs)
			}
		}
	}
}

// THE CONTROL. Every shape the reader decides, planted, so the census cannot
// be green because the reader stopped reading. Without this arm, hshRead
// could answer `probed` for everything and nothing in the suite would say so.
func TestQATheHashSpellingReaderPlacesEveryProbeShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		// want is one entry per expected mention, in line order: true =
		// the reader placed it inside a probe.
		want []bool
		lost bool
	}{
		{
			name: "the one-line probe this tree already ships",
			body: "#!/bin/sh\nsum() { if command -v sha256sum >/dev/null 2>&1; then sha256sum \"$@\"; else shasum -a 256 \"$@\"; fi; }\nsum x\n",
			want: []bool{true},
		},
		{
			name: "the three-arm block probe",
			body: "#!/bin/sh\nif command -v sha256sum >/dev/null 2>&1; then\n\tf() { sha256sum \"$1\"; }\nelif command -v shasum >/dev/null 2>&1; then\n\tf() { shasum -a 256 \"$1\"; }\nelse\n\tdie \"neither sha256sum nor shasum is on PATH\"\nfi\nf x\n",
			want: []bool{true, true, true, true, true},
		},
		{
			name: "a nested conditional inside the probe does not close it early",
			body: "#!/bin/sh\nif command -v sha256sum >/dev/null 2>&1; then\n\tif [ -n \"$X\" ]; then f() { sha256sum \"$1\"; }; fi\nelse\n\tf() { shasum -a 256 \"$1\"; }\nfi\n",
			want: []bool{true, true, true},
		},
		{
			name: "THE BUG: a bare call in the measuring path",
			body: "#!/bin/sh\nh=$(shasum -a 256 \"$1\")\n",
			want: []bool{false},
		},
		{
			name: "a bare call AFTER a probe that closed is not covered by it",
			body: "#!/bin/sh\nif command -v sha256sum >/dev/null 2>&1; then\n\tf() { sha256sum \"$1\"; }\nfi\nh=$(shasum -a 256 \"$1\")\n",
			want: []bool{true, true, false},
		},
		{
			name: "a comment naming the hasher is not a call",
			body: "#!/bin/sh\n# `shasum` stays: hashing IS the measurement.\nf x\n",
			want: nil,
		},
		{
			name: "a heredoc body is data, and its mentions are taken unprobed",
			body: "#!/bin/sh\ncat <<EOF\nshasum -a 256 \"$1\"\nEOF\n",
			want: []bool{false},
		},
		{
			name: "a dash heredoc terminator is still a terminator",
			body: "#!/bin/sh\ncat <<-EOF\nshasum x\n\tEOF\nh=$(shasum -a 256 \"$1\")\n",
			want: []bool{false, false},
		},
		{
			name: "a variable named for a keyword does not move the depth",
			body: "#!/bin/sh\nif command -v sha256sum >/dev/null 2>&1; then\n\tfi=1; echo \"$fi\"\n\tf() { sha256sum \"$1\"; }\nfi\nh=$(shasum -a 256 \"$1\")\n",
			want: []bool{true, true, false},
		},
		{
			name: "an unclosed probe region is lost, and a lost file probes nothing",
			body: "#!/bin/sh\nif command -v sha256sum >/dev/null 2>&1; then\n\tf() { sha256sum \"$1\"; }\n",
			want: []bool{false, false},
			lost: true,
		},
		{
			name: "a probe head that is the LAST line is still unclosed, and so lost",
			body: "#!/bin/sh\nif command -v sha256sum >/dev/null 2>&1; then\n",
			want: []bool{false},
			lost: true,
		},
		{
			name: "a conditional that is not a hasher probe covers nothing",
			body: "#!/bin/sh\nif [ -n \"$X\" ]; then\n\th=$(shasum -a 256 \"$1\")\nfi\n",
			want: []bool{false},
		},
		{
			name: "the wrapper's own name is not a mention of a hasher",
			body: "#!/bin/sh\nh=$(sha256_file \"$1\")\ng=$(sha256_stdin <\"$1\")\n",
			want: nil,
		},
		{
			name: "a probe spelled with `which` is not a probe this door accepts",
			body: "#!/bin/sh\nif which sha256sum >/dev/null 2>&1; then\n\tf() { sha256sum \"$1\"; }\nfi\n",
			want: []bool{false, false},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(t.TempDir(), "probe.sh")
			if err := os.WriteFile(p, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			ms, lost, err := hshRead(p)
			if err != nil {
				t.Fatal(err)
			}
			if (lost != "") != c.lost {
				t.Errorf("lost=%q, want lost=%v", lost, c.lost)
			}
			if len(ms) != len(c.want) {
				t.Fatalf("read %d mention(s), want %d — the matcher disagrees with the shape, so the census is deciding something other than what this case describes:\n  %v", len(ms), len(c.want), ms)
			}
			for i, m := range ms {
				if m.probed != c.want[i] {
					t.Errorf("mention %d (%s): probed=%v, want %v", i+1, m, m.probed, c.want[i])
				}
			}
		})
	}
}
