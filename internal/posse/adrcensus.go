package posse

// The ADR 0051 sha census. Since the 2026-09-05 operator ruling
// (ranger-base-bp0yj) this is not a gate: `posse gates adr-census` is an
// on-demand audit with one caller, cmd/posse/main.go, and no commit-time
// arm. It was cut out of gates.go unchanged (ranger-base-tn3u2).

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// adrShaPredicate is the ADR 0051 predicate, and since the 2026-09-05
// simplification it has exactly ONE caller: `posse gates adr-census`
// (AdrCensusScript), the on-demand audit. It used to have two — the
// prepare-commit-msg hook rendered the same text as a commit-time refusal —
// and that arm was deleted by operator ruling (ADR 0051, ranger-base-bp0yj).
// Committing an ADR now invokes no object lookup, no ancestry classification
// and no patch-id comparison at all. A citation helps find work; it does not
// prove landing, and the audit says so on request rather than at the wall.
//
// It stays a function of its own, called by one script, because ADR 0051's
// "retain the existing predicate" is exactly that: the reference script under
// scripts/ was once a SECOND copy of this rule in prose and was retired for
// it (ranger-base-gyrko). One text with one caller cannot drift; one text
// re-typed into a second reader can, which is what this shape prevents.
//
// LINE SOURCES ARE THE CALLER'S. Two shell functions are defined before this
// text runs, and the census defines both as the whole file:
//
//	posse_adr_judged FILE   the lines being judged — the census: every line
//	posse_adr_record FILE   the record a twin may sit in — the same file
//
// and sets posse_adr_files (newline-separated). posse_adr_judge then reads
// the base, classifies every token, and leaves three newline-separated
// verdict lists for the caller to render as a census:
//
//	posse_adr_base       the base branch, or EMPTY when the main checkout is
//	                     detached — it judged nothing and said so on stderr
//	posse_adr_ancestors  "TOK FILE"       a judged token that IS on the base
//	posse_adr_admitted   "TOK TWIN FILE"  a non-ancestor with a twin in the record
//	posse_adr_refused    "TOK FILE"       a non-ancestor with no twin — the verdict
//
// The token comes first and the path last, so a path with a space in it
// splits cleanly under ${l%% *} and ${l#* }.
//
// THE RADIUS IS THE FILE and not the line, and that was measured rather than
// chosen — a line radius refused every bracketed prose mention ADR 0051's
// own amendment wrote, because 76-column prose wraps the bracket onto the
// next line, and a paragraph radius needs a markdown block parser inside a
// shell script. The radius carries no safety anyway: the twin's NONEXISTENCE
// before landing is what carried it when this was a gate; as an audit it is
// a finding's scope and nothing rests on it.
//
// AN EMPTY PATCH-ID IS NOT A TWIN. `git diff-tree -p` prints nothing for a
// commit with no diff of its own — a root commit, a merge, an `--allow-empty`
// commit — so `git patch-id` prints nothing, and two empty patch-ids compare
// equal (MEASURED, git 2.50.1). Without the guard, an empty stale commit is
// admitted beside any empty ancestor, and a repo's ROOT commit is an empty
// ancestor that any ADR may legitimately cite. An empty patch-id is no
// answer, not a match.
//
// THE BASE IS THE MAIN CHECKOUT'S BRANCH, ASKED OF GIT, NEVER GUESSED. From
// a session worktree `git rev-parse --git-common-dir` is the main .git and
// its HEAD is the branch the operator has checked out (measured:
// refs/heads/main). When that HEAD is DETACHED the command answers nothing,
// and the predicate judges nothing and says so on stderr — ADR 0019's
// composite rule, no fallthrough on a read failure: a gate that cannot find
// its base does not fall through to a refusal it cannot justify. In a shared
// checkout the same command returns that checkout's own branch, and a sha
// the operator just made IS an ancestor of it — correct, since the
// operator's commits never pass through the launcher and never re-sha.
//
// TOKENS THAT DO NOT RESOLVE ARE PASSED, deliberately: they are prose, or
// another repo's, and this predicate cannot judge them. That is its whole
// false-positive budget — `deadbee` in a sentence costs one cat-file. A
// census over a PRUNED object store therefore judges nothing and its
// summary says "0 distinct tokens", never "clean" (ADR 0051 Consequences).
//
// NO ESCAPES for a blockquote or a fenced block, and no override env either —
// an audit that can be told to look away reports on the records nobody was
// worried about. The twin admission is the one exemption, and it is the shape
// a record whose SUBJECT is a stale sha already has.
//
// COST, MEASURED: cat-file plus merge-base is ~50 ms per token; the patch-id
// pair is ~60 ms and is computed only for a file that holds BOTH a
// non-ancestor and an ancestor, so the common case pays nothing extra. The
// record scan runs only for a file with a non-ancestor, for the same reason.
//
// This text is no longer inside any rendered hook, so scripts/cleanroom.sh's
// HOOK_DEPS no longer has to carry its commands: the clean-room probe walks
// the hooks, and `posse gates adr-census` is a command the operator types.
func adrShaPredicate() string {
	return `
# ─── the ADR sha predicate (ADR 0051, on-demand audit) ──────────────────────
# Rendered from one Go function into posse gates adr-census, and into nothing
# else: the commit-time arm that once shared this text was removed by operator
# ruling (ranger-base-bp0yj). The caller defined posse_adr_judged FILE (the
# lines judged) and posse_adr_record FILE (the record a twin may sit in) and
# set posse_adr_files; posse_adr_judge leaves posse_adr_base (empty:
# detached, judged nothing) and three "TOK ... FILE" lists: posse_adr_ancestors,
# posse_adr_admitted, posse_adr_refused.
#
# AN EMPTY PATCH-ID IS NO ANSWER. git diff-tree -p prints nothing for a
# commit with no diff of its own — a root commit, a merge, an empty
# commit — so patch-id prints nothing and two empties compare EQUAL,
# which would admit an empty stale commit beside the repo's own root
# (measured). The caller tests for emptiness; this just reports it.
posse_adr_pid() {
  git diff-tree -p "$1" 2>/dev/null | git patch-id --stable 2>/dev/null | cut -d' ' -f1
}
posse_adr_judge() {
  posse_adr_ancestors=''
  posse_adr_admitted=''
  posse_adr_refused=''
  # The branch the MAIN checkout has checked out, asked of git rather than
  # guessed: --git-common-dir is the main .git from a session worktree and
  # this repo's own .git in a shared checkout. Detached there and this
  # answers nothing, which is the one arm below that judges nothing.
  posse_adr_base=$(git --git-dir="$(git rev-parse --git-common-dir 2>/dev/null)" symbolic-ref -q HEAD 2>/dev/null)
  if [ -z "$posse_adr_base" ]; then
    echo "posse gates adr-census judged nothing — the main checkout's HEAD is detached, so this audit has no base branch to measure ancestry against (ADR 0051). Nothing here is a verdict: unjudged is not clean." >&2
    return 0
  fi
  # -f: the file loop splits an unquoted expansion and a path with a glob
  # character in it has to stay a path. IFS is a newline only, so every
  # list below — paths and tokens alike — is newline-separated and one
  # split rule serves both.
  set -f
  posse_adr_ifs=$IFS
  IFS='
'
  for posse_adr_f in $posse_adr_files; do
    # The tokens being JUDGED. Deduplicated: an ADR quotes one sha many
    # times and each judgement is ~50 ms. EITHER CASE of hex: git resolves
    # a sha case-insensitively (cat-file -e and merge-base both take the
    # uppercase spelling, measured), so a stale sha typed in capitals was a
    # commit to git and prose to a lowercase-only class, so it went unjudged
    # and the census said nothing about it (ranger-base-0fz98). The token
    # keeps its own spelling: the census greps the file for it.
    #
    # grep -a on BOTH token sources: one NUL byte in a record makes the
    # stream binary, grep answers "Binary file (standard input) matches"
    # instead of the tokens, and the census, which cat's the whole file,
    # judged nothing at all (ranger-base-h137b).
    posse_adr_anc=''
    posse_adr_non=''
    for posse_adr_t in $(posse_adr_judged "$posse_adr_f" | grep -aoE '\b[0-9a-fA-F]{7,40}\b' | sort -u); do
      # Does not resolve to a commit HERE: prose, or another repo's. Not
      # this predicate's to judge, and passed.
      git cat-file -e "$posse_adr_t^{commit}" 2>/dev/null || continue
      if git merge-base --is-ancestor "$posse_adr_t" "$posse_adr_base" 2>/dev/null; then
        # Landed. This is the shape D2 blesses.
        posse_adr_anc="$posse_adr_anc$posse_adr_t
"
        posse_adr_ancestors="$posse_adr_ancestors$posse_adr_t $posse_adr_f
"
      else
        posse_adr_non="$posse_adr_non$posse_adr_t
"
      fi
    done
    [ -n "$posse_adr_non" ] || continue
    # Only now, and only for this file: the candidate twins, from the whole
    # RECORD rather than the judged lines, because a record about stale
    # shas usually already carries them (D5).
    posse_adr_anc=''
    for posse_adr_t in $(posse_adr_record "$posse_adr_f" | grep -aoE '\b[0-9a-fA-F]{7,40}\b' | sort -u); do
      git cat-file -e "$posse_adr_t^{commit}" 2>/dev/null || continue
      git merge-base --is-ancestor "$posse_adr_t" "$posse_adr_base" 2>/dev/null || continue
      posse_adr_anc="$posse_adr_anc$posse_adr_t
"
    done
    for posse_adr_t in $posse_adr_non; do
      posse_adr_want=$(posse_adr_pid "$posse_adr_t")
      posse_adr_twin=''
      if [ -n "$posse_adr_want" ]; then
        for posse_adr_a in $posse_adr_anc; do
          if [ "$(posse_adr_pid "$posse_adr_a")" = "$posse_adr_want" ]; then
            posse_adr_twin=$posse_adr_a
            break
          fi
        done
      fi
      if [ -n "$posse_adr_twin" ]; then
        posse_adr_admitted="$posse_adr_admitted$posse_adr_t $posse_adr_twin $posse_adr_f
"
      else
        posse_adr_refused="$posse_adr_refused$posse_adr_t $posse_adr_f
"
      fi
    done
  done
  IFS=$posse_adr_ifs
  set +f
}
`
}

// AdrCensusScript is `posse gates adr-census`: adrShaPredicate, byte for
// byte, run over every line of every file it is handed, as both line
// sources. Since the 2026-09-05 simplification it is the ONLY reader of that
// predicate and the only place ADR 0051 is enforced at all — an audit the
// operator or a reviewer asks for, never a wall a commit walks into
// (ranger-base-bp0yj). It replaced scripts/adr-sha-census.sh, which was a
// second copy of the rule (ranger-base-gyrko).
//
// WHAT IT IS AND IS NOT EVIDENCE OF. Its findings are review inputs with
// their coverage stated. A token that does not resolve here is UNJUDGED, not
// clean; a detached main checkout means the whole run judged nothing; an
// empty patch-id is not a twin. And a clean census does not establish that
// anything LANDED — that is ADR 0006's block and nothing here substitutes
// for it.
//
// Output, per file: ADMITTED and REFUSE lines (an ancestor is judged and
// counted but not printed), then one summary line —
//
//	posse gates adr-census: base main judged N distinct tokens: A ancestors, T admitted by twin, R refused
//
// — and exit 1 when R > 0. The summary carries the judged count so that a
// zero over a PRUNED object store reads as "judged 0", never as clean
// (ADR 0051 Consequences, amended). When the main checkout is detached the
// predicate judged nothing, said so on stderr, and the census exits 0 with
// no verdict lines and no summary: there is nothing to summarize.
func AdrCensusScript() string {
	return `#!/bin/sh
# posse gates adr-census — ADR 0051's on-demand audit, and the only reader of
# the predicate below since the commit-time arm was removed by operator ruling
# (ranger-base-bp0yj). Both line sources are the whole file: every line is
# judged and every line may hold a twin.
set -u
posse_adr_judged() {
  cat "$1"
}
posse_adr_record() {
  cat "$1"
}
posse_adr_files=$(printf '%s\n' "$@")
` + adrShaPredicate() + `
posse_adr_judge
[ -n "$posse_adr_base" ] || exit 0
posse_adr_branch=${posse_adr_base#refs/heads/}
set -f
IFS='
'
for posse_adr_l in $posse_adr_admitted; do
  posse_adr_t=${posse_adr_l%% *}
  posse_adr_l=${posse_adr_l#* }
  posse_adr_twin=${posse_adr_l%% *}
  posse_adr_f=${posse_adr_l#* }
  # ranger-base-xu8ng: printf, not echo, same reason as ranger-base-xfh1n's
  # REFUSE-line fix (ranger-base-23mvz): posse_adr_f is a record path off the
  # loop's line; echo expands backslash escapes in its operand on the shells
  # that run this hook (macOS /bin/sh is bash 3.2 with xpg_echo on when
  # invoked as sh, a Linux /bin/sh is usually dash, same by spec), so a path
  # holding \n, \t, \\ or \c prints mangled or truncated.
  printf '%s\n' "ADMITTED $posse_adr_f $posse_adr_t twin $posse_adr_twin"
done
for posse_adr_l in $posse_adr_refused; do
  posse_adr_t=${posse_adr_l%% *}
  posse_adr_f=${posse_adr_l#* }
  posse_adr_lines=$(grep -anE "\b$posse_adr_t\b" "$posse_adr_f" | cut -d: -f1 | tr '\n' ',' | sed 's/,$//')
  # ranger-base-xfh1n: printf, not echo, same reason as ranger-base-23mvz.
  # posse_adr_f is a record path off the loop's line; echo expands backslash
  # escapes in its operand on the shells that run this hook (macOS /bin/sh is
  # bash 3.2 with xpg_echo on when invoked as sh, a Linux /bin/sh is usually
  # dash, same by spec), so a path holding \n, \t, \\ or \c prints mangled or
  # truncated.
  printf '%s\n' "REFUSE $posse_adr_f:$posse_adr_lines $posse_adr_t resolves here but is not on $posse_adr_branch and no landed twin is in the record — cite the bead id (git log --grep), or put the twin beside it (ADR 0051 D2/D5)"
done
posse_adr_j=$(printf '%s' "$posse_adr_ancestors" | grep -c .)
posse_adr_a=$(printf '%s' "$posse_adr_admitted" | grep -c .)
posse_adr_r=$(printf '%s' "$posse_adr_refused" | grep -c .)
echo "posse gates adr-census: base $posse_adr_branch judged $((posse_adr_j+posse_adr_a+posse_adr_r)) distinct tokens: $posse_adr_j ancestors, $posse_adr_a admitted by twin, $posse_adr_r refused"
[ "$posse_adr_r" -eq 0 ]
`
}

// AdrCensusDefault is what the census walks when handed no files: every
// record under the repo root, resolved from dir's toplevel. docs/adr holds
// two file classes (ranger-base-bvich) — the *.md decisions and the
// *.probe.sh supplements a record hands its reproduction to — and both are
// records the census can find a stale sha in, so both globs are walked. A
// slice, not one pattern, because Go has no const array/slice.
var AdrCensusDefault = []string{"docs/adr/*.md", "docs/adr/*.probe.sh"}

// RunAdrCensus runs AdrCensusScript over files, relative to dir, writing
// the census to stdout and the predicate's own stderr (the detached
// notice) to stderr. With no files it walks AdrCensusDefault from dir's git
// toplevel, so `posse gates adr-census` answers the same from any directory
// of the checkout; explicit paths are the caller's and are taken relative to
// dir. refused is the script's own exit 1 — a census with at least one
// REFUSE line — and is not an error: the census ran and said what it found.
// err is everything else: no repo, no files, sh missing.
func RunAdrCensus(dir string, files []string, stdout, stderr io.Writer) (refused bool, err error) {
	return runAdrCensus(dir, files, nil, stdout, stderr)
}

// runAdrCensus is RunAdrCensus with the child's environment as a parameter:
// nil inherits, which is what the command does; a fixture passes the same
// walled PATH and HOME its git runner uses, so the census it measures reads
// the fixture's config and not the box's.
func runAdrCensus(dir string, files []string, env []string, stdout, stderr io.Writer) (refused bool, err error) {
	if len(files) == 0 {
		top, terr := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
		if terr != nil {
			return false, fmt.Errorf("adr-census: %s is not inside a git repository", dir)
		}
		dir = strings.TrimSpace(string(top))
		var matches []string
		for _, glob := range AdrCensusDefault {
			m, gerr := filepath.Glob(filepath.Join(dir, filepath.FromSlash(glob)))
			if gerr != nil {
				return false, gerr
			}
			matches = append(matches, m...)
		}
		if len(matches) == 0 {
			return false, fmt.Errorf("adr-census: no %s under %s — nothing to judge", strings.Join(AdrCensusDefault, " or "), dir)
		}
		sort.Strings(matches)
		for _, m := range matches {
			rel, rerr := filepath.Rel(dir, m)
			if rerr != nil {
				return false, rerr
			}
			files = append(files, filepath.ToSlash(rel))
		}
	}
	cmd := exec.Command("sh", append([]string{"-c", AdrCensusScript(), "posse-gates-adr-census"}, files...)...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("adr-census: %w", err)
	}
	return false, nil
}
