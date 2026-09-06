package posse

// Gates L1 (ADR 0002 §3): the wall for shell-verb denies, outside any
// runtime. At every persona launch the PID's deny: rules of the shape
// Bash(<cmd> <prefix>:*) / Bash(<cmd> <prefix>) / Bash(<cmd>) are rendered
// into RHQ_HOME/state/gates/<persona>/bin/<cmd> — a POSIX sh shim that
// refuses when argv matches (message, exit 1, a line in refusals.log) and
// otherwise execs the real binary resolved at render time. Rendered fresh
// each launch: the PID is the source of truth, nothing hand-edited there
// survives. The shim dir is prepended ON THE TYPED COMMAND LINE
// (PATH=<bin>:$PATH <cmd>) rather than in the workspace env, because macOS
// path_helper reorders PATH when the pane shell starts. On every runtime,
// claude included: --disallowedTools is the polite refusal in front of the
// shim's hard one (L0Spellings widens the PID's deny list so it fires on
// the same argv this shim does — rangerhq-3mc). Known holes are why L3
// (pre-push hook, rangerhq-8s4) exists for git push: /usr/bin/git,
// command -p, and a *git alias* — `git p` where the operator's gitconfig
// says alias.p = push reaches the shim as the token `p`, and resolving it
// would mean running `git config --get alias.<tok>` per invocation, in
// POSIX sh, against whatever repo the options point at. L3 catches it in
// hooked repos; elsewhere it is the seatbelt/container tier's, not this
// matcher's.
//
// A deny naming a subcommand (Bash(git push:*)) is NOT matched at argv[1]:
// git and its kind accept global options before the subcommand, so
// `git -C <repo> push` walked straight past a positional matcher and out
// of every repo without our pre-push hook (rangerhq-2zm). The shim skips
// the command's leading global options — consuming the values of the ones
// that take a separate argument, from a per-command table — and matches
// the first non-option token. Commands with no table are matched
// best-effort and parity says so rather than claiming the gate.
//
// THE HARNESS IS NOT EXEMPT. The shim dir goes on the PATH of the pane, so
// `posse` typed inside a persona pane inherits it and every binary POSSE
// itself execs by BARE NAME resolves through the shims too. Measured
// (ranger-base-r64): posse's own keychain read — `security
// find-generic-password`, the darwin credential adapter in credential.go —
// was refused by the crew's Bash(security:*) deny, blinding the plan guard
// and silently UNKNOWN-ing the launch preflight. That is not a hole in the
// wall (nothing leaked; the deny worked), but it means a gate rule aimed at
// a persona also aims at us.
//
// That adapter now execs /usr/bin/security absolutely (ranger-base-ypf5), so
// it is no longer gated: the deny aims at what a persona may run, not at
// posse's own monitoring reads, and an absolute path is the documented way
// past L1. The GateRefusal type it reads off the stderr line below stays —
// it is the regression guard that keeps a return to a bare name from being
// misread as a credential outage again, and it remains the diagnosis for
// anything here that still execs a shimmable binary by name.
//
// EVERY LAYER HERE MATCHES ON THE TYPED WORD, so a command with two names
// on PATH is two commands to this matcher. The build gives `posse` exactly
// one: the `rhq` alias `make install`/`make link-plugin` used to write for
// continuity across the rename (rangerhq-tyay) is no longer created
// (ranger-base-igup). Two inodes from before that change survive on the
// operator's box until ranger-base-6y83's window, and a rule spelled
// Bash(posse …) does not fire on `rhq …` while they do. The rule outlives
// them: any second name reaching this binary — alias, wrapper, leftover
// symlink — is a second command here, so a PID that denies the harness by
// name must be spelled for every name that resolves to it.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// shimRule is one deny rule as the shim sees it: the words that must lead
// argv (after the command), whether the match is exact (no further args)
// or a prefix (`:*`), an optional qualifier that makes the match NEGATIVE,
// and the original rule text for the message.
type shimRule struct {
	Words  []string
	Exact  bool
	Unless string // `... unless <tok>`: refuse unless argv carries <tok> with an operand
	Rule   string
}

// Verb reports whether the rule keys on a subcommand — its first word is a
// plain token, not an option. Those are matched after the command's global
// options are skipped; rules that lead with an option (Bash(rm -rf /)) are
// a literal argv prefix and are matched where they are written.
func (r shimRule) Verb() bool {
	return len(r.Words) > 0 && !strings.HasPrefix(r.Words[0], "-")
}

// Lead and OptTail split a verb rule into the words matched BY POSITION and
// the trailing long options matched by MEMBERSHIP in what follows them.
//
// `Bash(bd sync --full:*)` rendered as a test of $1 against `sync` and $2
// against `--full`, which reads the flag as if it had a position. cobra
// does not give it one: `--full` is a plain flag on `bd sync`, so any other
// flag in front of it moved it out of $2 and past the wall — `bd sync
// --push --full` and `bd sync --dry-run --full` both RAN (**MEASURED** on
// the rendered shim, ranger-base-vct2). Same class as rangerhq-2zm one
// level down: there it was a global option before the VERB, here it is any
// option before the FLAG, and `bd sync --full` is the one spelling of sync
// that commits AND pushes.
//
// The split is where the option run starts, so a rule naming a
// sub-subcommand (`bd dep relate --x`) keeps `dep relate` positional.
//
// Only LONG options are taken this way. A short one clusters (`-qf` is `-q
// -f`), and a cluster interacts with the verb's own value-taking short
// options in ways no rule in ADR 0015 §3 needs today; such a rule keeps the
// positional matcher and matcherFor stops calling it faithful, rather than
// getting a membership test that quietly misses `-qf`.
func (r shimRule) Lead() []string {
	if !r.Verb() {
		return r.Words
	}
	for i, w := range r.Words {
		if strings.HasPrefix(w, "-") {
			return r.Words[:i]
		}
	}
	return r.Words
}

// optWords is everything after the lead — the option run, plus anything
// written behind it.
func (r shimRule) optWords() []string {
	return r.Words[len(r.Lead()):]
}

// OptTail is the option run when every word in it is a LONG option, else
// nil. Nil is the signal to match the whole rule positionally, exactly as
// before — and matcherFor reads the same two calls to tell a rule that has
// no option run (faithful) from one whose run it declined (best-effort).
func (r shimRule) OptTail() []string {
	if !r.Verb() || len(r.optWords()) == 0 {
		return nil
	}
	for _, w := range r.optWords() {
		if !strings.HasPrefix(w, "--") {
			return nil
		}
	}
	return r.optWords()
}

// globalValueOpts lists, per command, the options that may appear BEFORE
// the subcommand AND take their value as a separate argument — the ones
// that must be consumed in pairs or the value is mistaken for the
// subcommand (`git -C <repo> push`). Options taking `--opt=value`, and
// boolean ones (-p, --no-pager, --bare, --literal-pathspecs…), need no
// entry: any other leading `-token` is skipped singly.
//
// `git --exec-path` is deliberately absent: bare, it prints and exits
// without consuming the next word, so treating it as a pair would hide a
// following `push` from the matcher.
//
// An entry with NO options is meaningful and not a placeholder: it declares
// that the command has no global option taking a separate value, which makes
// its subcommand rules exactly matchable and lets parity claim them realized.
// posse is such a command, and it is one by construction — `main()` reads
// argv[1] as the subcommand with no global flag parsing at all, so
// `posse -x promote` is not "promote behind an option", it is the unknown
// command `-x` (**MEASURED**, cmd/posse/main.go). Without the entry every PID
// carrying ADR 0015 §3's `Bash(posse promote:*)` launched DEGRADED on every
// runtime × cage (measured on the live gates report, ranger-base-o943).
var globalValueOpts = map[string][]string{
	"git":   {"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix", "--config-env", "--attr-source"},
	"posse": {},
	// bd 0.49.1 (0d99d153) has eighteen global options — not the seven the
	// az93 write-up counted — and exactly these four take their value as a
	// separate word (**MEASURED**, `bd --help`). The other fourteen are
	// booleans and skip singly. Without this entry `bd --db /tmp/x daemon
	// stop` resolves to the verb `/tmp/x` and the shim waves it through,
	// which is the same hole in the wall that a `Bash(bd daemon:*)`
	// permission rule has in the typed line (ranger-base-3bqn, from az93).
	"bd": {"--actor", "--db", "--dolt-auto-commit", "--lock-timeout"},
}

// verbValueOpts is globalValueOpts one level in: per "<cmd> <lead words>",
// the options of THAT SUBCOMMAND which take their value as a separate word.
// The membership scan for an OptTail rule must consume them in pairs or an
// option's VALUE is mistaken for the denied flag — `bd sync -m --full` is a
// commit message, not a full sync (**MEASURED**, `bd sync --help`, bd
// 0.49.1: `-m/--message string` and `--set-mode string` are its only two).
//
// Keyed the way the rule is written, like qualifierSpoilers: `Bash(bd sync
// --full:*)` looks up "bd sync".
//
// A MISSING entry is not a hole and does not cost the parity claim: nothing
// is paired, so an option's value that happens to be spelled like the
// denied flag is refused too. The wall stands wider than the rule — the
// cheap error of this class, one respelling away — rather than not standing.
// `Bash(git push --force:*)`, the other flag rule ADR 0001 ships, has no
// entry for exactly this reason: git's parse-options separates required
// from OPTIONAL arguments (`--repo=<r>` takes a following word,
// `--force-with-lease` does not), and no entry is better than one guessed.
// Whoever measures git's belongs here. An entry with NO options declares,
// like posse's above, that the subcommand has none.
//
// The other residual this table cannot fix is SPELLING, not position: git
// accepts every unambiguous prefix of a long option, so a rule naming a git
// flag may have to carry the abbreviations too. That is the spoilers'
// `LongMin` + longArms mechanism (ranger-base-l1at, landed), and a DENIED
// flag has no equivalent because the one denied git flag ADR 0001 ships
// needs none — **MEASURED**, not derived (ranger-base-0zln, git 2.50.1 /
// Apple Git-155).
//
// It could not be measured on `push`: every PID carrying the rule denies
// the verb, so `git push --forc` is refused by this shim before git's
// parse-options ever sees it — the gate working, not the answer. It was
// measured on `git switch`, whose table has the same shape and the same
// option NAME, with the exact option declared TENTH after its longer
// sibling (`-C, --force-create` at position 2, `-f, --force` at 12):
//
//	git switch --force          fatal: missing branch or commit argument
//	git switch --forc           error: ambiguous option: forc (could be
//	                            --force-create or --force)      [exit 129]
//	git switch --for/--fo/--f   the same ambiguity error
//	git switch --force-c        resolves to --force-create
//
// Two rules, both measured there. An EXACT long name wins outright even
// when a longer sibling is declared ahead of it, so `--force` reaches the
// command. And a proper prefix matching more than one name is refused BY
// GIT, exit 129, before the command runs. `git push`'s table carries three
// names under `--force` — `--force`, `--force-with-lease`,
// `--force-if-includes`, all three present in this binary (`strings`) and
// in the git-push(1) it ships — plus `--follow-tags`, so every proper
// prefix of `--force` (`--forc`, `--for`, `--fo`, `--f`) is ambiguous and
// git rejects it itself. The wall does not have to. This does not rest on
// push's table being exactly as documented either: options can only ADD
// ambiguity, never remove it, so ONE `--force-*` sibling suffices and
// there are two.
//
// So the spelling set for `--force` closes at the long form and
// `--force=value`, which is what renderFlagIn already emits. `--force=1`
// is not a git spelling (`error: option 'force' takes no value`, measured
// the same way): refusing it is the wall standing wider than the rule, not
// a way through it. Do not build longArms for a denied flag until some
// rule needs it, and not then without a per-option measurement.
//
// What is NOT closed by any of that is a spelling the RULE does not name.
// At least FOUR force-push and none carries the token `--force`
// (**MEASURED**, git-push(1) as this install ships it, read rather than run
// because every PID carrying the rule denies the verb) — a floor, not a
// count: it is however many this comment's author found in one manpage, not
// a claim that git-push(1) holds no more:
//
//   - `git push -f`. `-f, --force` registers `-f` as a SEPARATE short name,
//     so it is not a prefix of anything and the ambiguity result above says
//     nothing at all about it. It clusters, too: `-qf`.
//   - `git push --force-with-lease`, which disables the same ancestry check
//     under another name. (`--force-if-includes` is NOT a fifth: the page
//     calls it "an ancillary option along with --force-with-lease" — a
//     safety check on a force that is already happening.)
//   - `git push origin +main`: "All of the rules described above about
//     what's not allowed as an update can be overridden by adding an the
//     optional leading + to a refspec (or using --force command line
//     option)."
//   - `git push --mirror origin`: "locally updated refs will be FORCE
//     UPDATED on the remote end ... This is the default if the
//     configuration option remote.<remote>.mirror is set" (ranger-base-e7eo).
//     Under that config a bare `git push origin` force-updates with no
//     option and no refspec in the argv at all — the `+main` lesson one
//     step further out, and the strongest argument that the wall belongs on
//     the verb.
//
// That is the rule's scope rather than the shim's fidelity, so matcherFor
// may still claim it.
//
// ranger-base-zs6b DECIDED it, and decided NOT to widen the wall here. An
// alias set per denied option cannot close this hole: a wall that decides
// how an OPTION is spelled has nothing to match in `+main`. Measured, not
// argued — building the alias arms moves `-f`, `-qf` and
// `--force-with-lease`, and leaves `+main` walking through. It buys every
// spelling that IS an option and still not the one that is not, and then
// READS like a force wall, which is worse than a residual written down.
//
// The rule that means "no force-push" is `Bash(git push:*)`, and every PID
// in examples/agents carries it — asserted, not merely observed
// (TestExampleAgentsArePIDs; ADR 0001). The flag rule beside it is a label
// on the refusal, never the wall. Both halves are pinned in gates_test.go
// (TestForceFlagRuleLeavesSpellingsThatTheVerbRuleCloses) and, for `--mirror`
// and the bare push it can mean, forcespelling_qa_test.go
// (TestQAMirrorIsAFourthForcePushSpellingThatOnlyTheVerbRuleCloses): the
// residual passes the flag rule, the same argv refuses under the verb rule.
// L3 bounds the whole thing — PrePushHook matches the RULE, not the argv, so
// a session carrying either rule loses every push whatever the spelling.
//
// bd's parser abbreviates nothing — `bd list --js` answers `unknown flag:
// --js` (**MEASURED**) — which is why the spelling set for `--full` closes
// at the long form and `--full=value`.
var verbValueOpts = map[string][]string{
	"bd sync": {"-m", "--message", "--set-mode"},
}

// verbValueOptsFor returns the value-taking options of r's subcommand, with
// any option the rule itself denies removed. Denying a value-taking flag
// (`Bash(bd sync --set-mode:*)`) must not render a scan that shifts past
// that very flag as somebody's value — the deny wins over the pairing.
func verbValueOptsFor(cmd string, r shimRule) []string {
	denied := map[string]bool{}
	for _, o := range r.OptTail() {
		denied[o] = true
	}
	var out []string
	for _, o := range verbValueOpts[verbKey(cmd, r)] {
		if !denied[o] {
			out = append(out, o)
		}
	}
	return out
}

// verbKey is the verbValueOpts key for r under cmd: the command plus the
// rule's positional lead ("bd sync", "bd dep relate").
func verbKey(cmd string, r shimRule) string {
	return strings.TrimSpace(cmd + " " + strings.Join(r.Lead(), " "))
}

// spoiler names the options that SATISFY a negative rule's qualifier and
// still do the very thing the rule refuses. `unless <tok>` asks whether
// argv carries the token with an operand, which is a proxy — for `git
// commit` it is a proxy for "path-limited" — and a proxy has two kinds of
// error. A false positive (`git commit -m x --pathspec-from-file=list` IS
// path-limited and is refused) costs a respelling and leaves a way through.
// A false negative is the wall not being there. These are the false
// negatives, measured per option, and the shim refuses them by name.
//
// Opts are matched before the qualifier's own operand: after `--` every
// word is a path, and a file named `-i` is a file. Short options are
// single-letter and match inside a cluster — `git commit -im x -- b.txt`
// sweeps exactly as `-i` does (measured, git 2.39.3), and so do `-pm x`,
// `-qp` and `-sp` (measured, git 2.50.1, ranger-base-myai).
//
// What that cluster pattern must NOT reach is an option's VALUE, which is
// what ValueOpts is for (ranger-base-v3cu).
type spoiler struct {
	Opts []string // `-x` (single letter, matches in a cluster) or `--long`
	// ValueOpts are the subcommand's own options that take their value as a
	// SEPARATE word, so the scan consumes them in pairs instead of reading
	// the value as an option — the same pairing renderFlagIn does with
	// verbValueOpts, one level in. Without it `git commit -m '-i am a
	// message' -- a.txt` is refused: the message matches the `-*i*` cluster
	// arm before the scan reaches `--`, and that commit is path-limited,
	// carries no -i, and does not sweep (**MEASURED**, git 2.50.1 —
	// ranger-base-v3cu, the bug this field closes).
	//
	// Membership is a MEASUREMENT and the direction of error is not
	// symmetric. An option listed here that does NOT eat the next word is a
	// HOLE: `git commit -u -i -- f` would shift past a real `--include`.
	// One that is missing costs a false positive — a refusal one respelling
	// away. TestQAValueOptsAreGitsRequiredValueOptions asks git itself,
	// per option, which is which.
	//
	// A long option here needs a LongMin like a spoiler does, for the same
	// reason and with the opposite consequence: an abbreviation with no arm
	// is a false positive, not a hole.
	//
	// It pairs the separate-word form only. A GLUED value (`-mi`,
	// `-m'fix typo'`) is not a pair — git takes the rest of the token — and
	// renderSpoiled skips those tokens singly, which covers the value
	// option written FIRST in its token. A value option behind a boolean in
	// the same cluster (`-qmfix typo`) still reads as a cluster and is
	// refused: a glob cannot say "no earlier letter in this token also took
	// a value", and that residual fails closed with the safe form one space
	// away.
	ValueOpts []string
	// LongMin is the shortest abbreviation git resolves to a long option in
	// Opts or ValueOpts, keyed by that option. git's parse-options accepts
	// any UNAMBIGUOUS PREFIX, so `--inc` IS `--include` and an arm spelling
	// the option in full misses every abbreviation on the way to it
	// (ranger-base-l1at). Every long option in either list needs an entry;
	// without one, a spoiler's abbreviations walk past the wall and a value
	// option's abbreviations take their value back into the scan.
	LongMin map[string]string
	Why     string // completes "…, and without <opts> — <why>"
}

// qualifierSpoilers is keyed by command and subcommand, the way the rule is
// written: `Bash(git commit unless --)` looks up "git commit".
var qualifierSpoilers = map[string]spoiler{
	"git commit": {
		// Every `git commit` option that takes the shared index while
		// carrying a pathspec. The set is measured, not reasoned about, and
		// TestQASpoilerTableCoversEveryCommitOption keeps it from going
		// stale under a git that grows one more (ranger-base-myai).
		Opts: []string{"-i", "--include", "-p", "--patch", "--interactive"},
		// Every `git commit` option that takes its value as a SEPARATE
		// word, and only those: measured one option at a time against the
		// real git (`git commit --dry-run <opt>` answers "requires a
		// value" for exactly these), git 2.50.1 / Apple Git-155,
		// ranger-base-v3cu. `-S/--gpg-sign` and `-u/--untracked-files` are
		// deliberately ABSENT — their argument is OPTIONAL, so they take
		// the rest of their own token and never the next word, and pairing
		// them would shift the scan past a real `-i`. The `--no-` spellings
		// are absent for the same reason: `--no-message` takes no value.
		//
		// The last three are the UNION over the two gits in play, not a
		// property of one of them: `git commit` grew `-U/--unified` and
		// `--inter-hunk-context` (the context width of the `-v` diff) after
		// 2.50.1, and all three answer "requires a value" on git 2.55.0 —
		// the version both ci.yml runners now carry (measured 2026-09-05
		// from the bottle, ranger-base-tiidc). Carrying them is safe on the
		// older git in the direction that matters: 2.50.1 does not have the
		// option at all, so the pairing can only shift the scan inside a
		// command git itself refuses with `unknown switch` before any index
		// is touched. Leaving them out is NOT safe on the newer one — the
		// value is read as an option and a path-limited commit is refused.
		// qaCommitOptsSince carries the same fact on the pin side.
		ValueOpts: []string{"-c", "-C", "-F", "-m", "-t",
			"--author", "--cleanup", "--date", "--file", "--fixup", "--message",
			"--pathspec-from-file", "--reedit-message", "--reuse-message",
			"--squash", "--template", "--trailer",
			"-U", "--unified", "--inter-hunk-context"},
		// Measured, git 2.50.1, one prefix at a time against the real git
		// (qaGitResolves): `--inc` resolves to `--include`, `--patc` to
		// `--patch`, `--int` to `--interactive`. `--in`/`--i` are ambiguous
		// between the first and the last, and `--pat` is ambiguous with
		// `--pathspec-from-file`, so git rejects those itself and the wall
		// does not have to. The value options' minima were measured the
		// same way, on the same git, and are read the same way: shorter
		// than these, git calls ambiguous and refuses itself, so the wall
		// never sees them (ranger-base-v3cu).
		LongMin: map[string]string{
			"--include": "--inc", "--patch": "--patc", "--interactive": "--int",
			"--author": "--au", "--cleanup": "--c", "--date": "--da",
			"--file": "--fil", "--fixup": "--fix", "--message": "--m",
			"--pathspec-from-file": "--pathspec-fr", "--reedit-message": "--ree",
			"--reuse-message": "--reu", "--squash": "--sq", "--template": "--te",
			"--trailer": "--tr",
			// Measured on git 2.55.0, the only git that has these:
			// `--uni` resolves (`--un`/`--u` are ambiguous with
			// `--untracked-files`), and `--inter-` resolves — every prefix
			// up to and including `--inter` is ambiguous with
			// `--interactive`, which is why `--interactive`'s own `--int`
			// is now SHORTER than that git's boundary and stays: an
			// over-short minimum renders arms git refuses itself
			// (ranger-base-90y3c, TestQASpoilerLongMinIsGitsBoundary).
			"--unified": "--uni", "--inter-hunk-context": "--inter-",
		},
		Why: "-i/--include commits the shared index ON TOP of the named paths\n" +
			"  (rangerhq-ojnw); -p/--patch/--interactive commit it INSTEAD of them,\n" +
			"  because a fleet Bash call has no TTY and the selector at EOF picks\n" +
			"  nothing, so the commit is the other persona's staged work and only\n" +
			"  that (ranger-base-myai)",
	},
}

// qualifierPrereqs is the other git-specific fact the L1 hint carries, and
// it is a PREREQUISITE of the safe form rather than a hole in it
// (rangerhq-4pbt). `unless --` demands a pathspec, and a pathspec only
// matches a file git already has an index entry for — so the one form this
// rule permits cannot introduce a NEW file. Measured, git 2.39.3: `git
// commit -F - -- <untracked>` answers `error: pathspec '<p>' did not match
// any file(s) known to git` and exits 1 before either wall is reached, so
// neither layer gets to say anything. The persona's obvious next reach is
// `git add` + `git commit`, which IS refused — by the same message that
// just failed them. Two refusals and no way through is how the private
// GIT_INDEX_FILE recipe gets reinvented (rangerhq-8rtf, rangerhq-2f5r), so
// the route is named here rather than left to be rediscovered.
//
// Two lines, not the whole recipe: the L1 shim refuses the UNQUALIFIED form
// and cannot know whether a new file is involved, so this rides on every
// such refusal and has to earn its width. The full form, with the residual
// it opens, is in the L3 hook — which is git-specific by construction.
//
// Keyed like the spoilers, by command and subcommand.
var qualifierPrereqs = map[string]string{
	"git commit": "  a NEW file has no index entry yet, so no pathspec matches it: run\n" +
		"  `git add -- <the new paths>` first, scoped and never bare, then the\n" +
		"  same path-limited commit (measured, rangerhq-4pbt).",
}

// prereqFor returns the prerequisite line for r under cmd, or "". Only a
// negative rule has a qualifier with a prerequisite.
func prereqFor(cmd string, r shimRule) string {
	if r.Unless == "" {
		return ""
	}
	return qualifierPrereqs[strings.TrimSpace(cmd+" "+strings.Join(r.Words, " "))]
}

// spoilersFor returns the spoiler table entry for r under cmd. Only a
// negative rule has a qualifier to spoil.
func spoilersFor(cmd string, r shimRule) spoiler {
	if r.Unless == "" {
		return spoiler{}
	}
	return qualifierSpoilers[strings.TrimSpace(cmd+" "+strings.Join(r.Words, " "))]
}

// spoiledFunc is the name of the sh helper rendered for r's spoilers, or ""
// when it has none. Named for the subcommand so the rendered shim reads:
// `posse_spoiled_commit "$@"`.
func spoiledFunc(cmd string, r shimRule) string {
	if len(spoilersFor(cmd, r).Opts) == 0 {
		return ""
	}
	return shIdent("posse_spoiled", r.Words...)
}

// shIdent builds a sh function name from a prefix and some rule words, with
// everything outside [A-Za-z0-9] mapped to `_` so an option or a hyphenated
// subcommand (`rename-prefix`) cannot render an unnameable function.
func shIdent(prefix string, words ...string) string {
	name := prefix
	for _, w := range words {
		name += "_" + strings.Map(func(c rune) rune {
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
				return c
			}
			return '_'
		}, w)
	}
	return name
}

// longArms are the case patterns that catch one long spoiler: every
// abbreviation git resolves to it, shortest first, ending with the option
// spelled out. min is the shortest prefix git accepts (spoiler.LongMin);
// with none, or one that is not a prefix, only the literal is caught — the
// arm is never guessed wider than a measurement.
func longArms(long, min string) []string {
	if min == "" || min == long || !strings.HasPrefix(long, min) {
		return []string{long}
	}
	var out []string
	for n := len(min); n <= len(long); n++ {
		out = append(out, long[:n])
	}
	return out
}

// flagInFunc names the sh helper that answers "does this subcommand's own
// argv carry <opt>?" for one rule and one of its OptTail options.
func flagInFunc(r shimRule, opt string) string {
	return shIdent("posse_flagin", append(append([]string{}, r.Lead()...), opt)...)
}

// renderFlagIn writes that helper: walk the segment the verb matcher is
// looking at, looking for opt.
//
// It walks the whole segment, lead words included, rather than shifting
// past them. It reads the same — a lead word is a plain token, so it is
// neither the denied option nor one of the value-taking ones — and it keeps
// the helper independent of how many words the rule's lead has, which its
// NAME (flagInFunc) does not record.
//
// Two things it must NOT call a match, both measured on bd 0.49.1:
//
//   - an option's VALUE. `bd sync -m --full` is the commit message
//     "--full", so the verb's value-taking options are consumed in pairs
//     from verbValueOpts, exactly as globalValueOpts pairs the command's.
//   - an operand. pflag stops parsing at `--`, so `bd sync -- --full` hands
//     `--full` to the subcommand as a positional and no full sync happens.
//
// …and one it MUST, which the positional matcher also missed: `--full=true`
// is the same flag (**MEASURED**: `bd list --limit 1 --json=true` prints
// the JSON that `--json` does). `--full=false` is then refused too — the
// cheap error of this class, one respelling away, versus the expensive one
// of the wall not being there.
//
// The spelling set is closed for this parser: pflag takes no abbreviations
// (`bd list --js` answers `unknown flag: --js`, **MEASURED**), which is what
// separates it from git's parse-options, where a literal arm misses every
// abbreviation on the way to the option (ranger-base-l1at). A rule naming a
// flag on a parser that DOES abbreviate needs those arms here, the way
// renderSpoiled gets them from longArms. The one git flag rule ADR 0001
// ships needs none even so: git refuses every proper prefix of `--force` as
// ambiguous itself, and `--force=1` as taking no value — **MEASURED**, see
// verbValueOpts (ranger-base-0zln).
//
// Nor does it get ALIAS arms (`-f`, `--force-with-lease`): decided against,
// measured, in the same comment. They would not close the hole they look
// like they close — `git push origin +main` force-pushes with no option to
// spell — and `Bash(git push:*)` does (ranger-base-zs6b).
//
// The arms are baked in rather than passed in a variable, following
// renderSpoiled: a glob pattern that reaches `case` through an expansion is
// one more place for the shim to mean something other than it says.
func renderFlagIn(name string, valueOpts []string, opt string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s() {\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n", name)
	if len(valueOpts) > 0 {
		quoted := make([]string, 0, len(valueOpts))
		for _, o := range valueOpts {
			quoted = append(quoted, shQuote(o))
		}
		fmt.Fprintf(&b, "      %s)\n        [ $# -ge 2 ] || return 1\n        shift 2\n        continue ;;\n", strings.Join(quoted, "|"))
	}
	b.WriteString("      --) return 1 ;;\n") // past here every word is an operand
	fmt.Fprintf(&b, "      %s|%s*) return 0 ;;\n", shQuote(opt), shQuote(opt+"="))
	b.WriteString("    esac\n    shift\n  done\n  return 1\n}\n")
	return b.String()
}

// renderSpoiled writes the sh helper: does argv, up to the first `--`,
// carry one of the spoiling options? The case arms are baked in rather than
// passed in a variable, because a glob pattern reaching `case` through an
// unquoted expansion is also a pathname expansion, and one of these
// patterns would happily match a file in the caller's cwd.
//
// Arm order is the whole design, and every line of it is load-bearing:
//
//  1. `--` first: past it every word is a path, and a file named `-i` is a
//     file.
//  2. the spoilers, so a spelling that is BOTH a spoiler and something else
//     is refused before any arm can skip it.
//  3. the value options in their separate-word form, paired — before the
//     `--*` skip, so a LONG option's value is consumed rather than left for
//     the cluster arm to read (ranger-base-v3cu).
//  4. `--*`, so `--signoff` is not read as a cluster carrying `-i`.
//  5. the value options' GLUED form, skipped singly: git takes the rest of
//     `-mi` as the message, so the token holds no option after the first.
//  6. the cluster arms, last, because everything above has already claimed
//     the tokens they would misread.
func renderSpoiled(name string, sp spoiler) string {
	var longs, shorts []string
	for _, o := range sp.Opts {
		if strings.HasPrefix(o, "--") {
			longs = append(longs, o)
			continue
		}
		shorts = append(shorts, strings.TrimPrefix(o, "-"))
	}
	// The separate-word arms: every long value option's abbreviation ladder,
	// then the short ones exactly as written. `-m` is an arm, `-m*` is not —
	// a glued value is not a pair and must not shift twice.
	var pairs, vShorts []string
	for _, o := range sp.ValueOpts {
		if strings.HasPrefix(o, "--") {
			pairs = append(pairs, longArms(o, sp.LongMin[o])...)
			continue
		}
		vShorts = append(vShorts, o)
	}
	pairs = append(pairs, vShorts...)

	var b strings.Builder
	fmt.Fprintf(&b, "%s() {\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n", name)
	b.WriteString("      --) return 1 ;;\n") // past here every word is a path
	for _, l := range longs {
		fmt.Fprintf(&b, "      %s) return 0 ;;\n", strings.Join(longArms(l, sp.LongMin[l]), "|"))
	}
	if len(pairs) > 0 {
		// The value is the next word, whatever it is spelled like. A value
		// option with nothing after it is not a pair and not a spoiler
		// either: git rejects it before the shim's opinion matters.
		fmt.Fprintf(&b, "      %s)\n        [ $# -ge 2 ] || return 1\n        shift 2\n        continue ;;\n", strings.Join(pairs, "|"))
	}
	if len(shorts) > 0 || len(vShorts) > 0 {
		// Long options are done above: without this arm `--signoff` would
		// match the cluster pattern for `-i` and be refused.
		b.WriteString("      --*) ;;\n")
	}
	if len(vShorts) > 0 {
		glued := make([]string, 0, len(vShorts))
		for _, o := range vShorts {
			glued = append(glued, o+"*")
		}
		fmt.Fprintf(&b, "      %s) ;;\n", strings.Join(glued, "|"))
	}
	for _, s := range shorts {
		fmt.Fprintf(&b, "      -*%s*) return 0 ;;\n", s)
	}
	b.WriteString("    esac\n    shift\n  done\n  return 1\n}\n")
	return b.String()
}

// matcherFor names how the shim matches r for cmd, and reports whether
// that matcher realizes the rule faithfully — what parity may claim.
func matcherFor(cmd string, r shimRule) (kind string, faithful bool) {
	switch {
	case len(r.Words) == 0:
		return "whole verb", true
	case !r.Verb():
		return "literal argv prefix", true
	case r.Unless != "" && globalValueOpts[cmd] != nil:
		return "subcommand, option-aware, negative match", true
	case len(r.optWords()) > 0 && r.OptTail() == nil:
		// A short option in the tail: matched where it is written, which a
		// cluster walks past. Parity says so rather than claiming it (see
		// shimRule.Lead for why the membership scan stops at long options).
		return "subcommand, positional flag, best-effort", false
	case r.OptTail() != nil && globalValueOpts[cmd] != nil:
		// A MISSING verbValueOpts entry does not cost the claim. It only
		// means nothing is paired at that level, so an option's value that
		// happens to be spelled like the denied flag is refused too — the
		// wall standing wider than the rule, not a way through it.
		return "subcommand, option-aware, flag anywhere in the segment", true
	case globalValueOpts[cmd] != nil:
		return "subcommand, option-aware", true
	default:
		return "subcommand, best-effort", false
	}
}

// matcherWhy is what parity prints instead of a claim when matcherFor says
// best-effort. Two different holes reach it and they have different exits,
// so one canned sentence for both would send the reader to the wrong table.
func matcherWhy(cmd string, r shimRule) string {
	if len(r.optWords()) > 0 && r.OptTail() == nil {
		return fmt.Sprintf("L1 shim matches %q by position and a short option clusters (`-qf` is `-q -f`), so a cluster carries it past (spell the rule with the long option) — best-effort only", r.optWords()[0])
	}
	return fmt.Sprintf("L1 shim has no global-option table for %s, so an option taking a separate value before %q hides it (deny the whole verb: Bash(%s)) — best-effort only", cmd, r.Words[0], cmd)
}

// ParseShimRules groups the shell-verb denies by command. Rules that are
// not Bash(...) — Edit, Write, WebFetch, mcp__* — are other layers'.
func ParseShimRules(deny []string) map[string][]shimRule {
	out := map[string][]shimRule{}
	for _, r := range deny {
		if !strings.HasPrefix(r, "Bash(") || !strings.HasSuffix(r, ")") {
			continue
		}
		body := strings.TrimSuffix(strings.TrimPrefix(r, "Bash("), ")")
		exact := true
		if strings.HasSuffix(body, ":*") {
			body = strings.TrimSuffix(body, ":*")
			exact = false
		}
		words := strings.Fields(body)
		if len(words) == 0 || !ValidName(strings.ReplaceAll(words[0], ".", "-")) {
			continue // not a plain command name; nothing to shim
		}
		cmd := words[0]
		rule := shimRule{Words: words[1:], Exact: exact, Rule: r}
		// A NEGATIVE rule: `Bash(git commit unless --)` refuses `git commit`
		// UNLESS argv carries `--` with at least one word after it. The
		// operand matters — `git commit --` with an empty pathspec commits
		// the shared index like the bare form does (measured, rangerhq-lmq9).
		// It is inherently a prefix match: `git commit -m x` must be caught.
		if n := len(rule.Words); n >= 3 && rule.Words[n-2] == "unless" {
			rule.Unless = rule.Words[n-1]
			rule.Words = rule.Words[:n-2]
			rule.Exact = false
		}
		if len(rule.Words) == 0 {
			rule.Exact = false // Bash(cmd) / Bash(cmd:*): the whole verb
		}
		out[cmd] = append(out[cmd], rule)
	}
	return out
}

// L0Spellings widens a PID's deny list into the rule spellings a
// claude-dialect matcher (claude --disallowedTools) needs in order to
// refuse the same argv the L1 shim does. L0 is politeness, never the wall
// (ADR 0002 §3) — but a polite refusal that fires on `bd` and not on
// `bd show x` is the friction the design meant to provide going missing
// exactly where it is wanted (rangerhq-3mc).
//
// Claude matches a Bash rule three ways (dialect verified on claude
// 2.1.234, the prefix form re-measured on 2.1.241, and the CLI's own
// `--disallowedTools` splitter does not split inside the parens, so a rule
// with spaces reaches the matcher whole):
//
//	Bash(git push)    exact    — the whole command, and nothing else
//	Bash(git push:*)  prefix   — a prefix of the argv TOKENS, not of the
//	                             command string
//	Bash(git * push)  wildcard — `*` is `.*` over the whole string,
//	                             anchored both ends, whitespace collapsed
//
// The prefix form matches per word, not per character (ranger-base-g8e,
// claude 2.1.241): under `Bash(sed -n:*)`, `sed -ni 1p f.txt` does NOT
// match though the command string starts with the rule's text, and
// `sed -n -i.bak …` does. Either way the option spellings walk past —
// `git -C x push` neither starts with the string `git push` nor leads with
// those two tokens — and a subcommand rule now says nothing about them.
//
// It used to. An option-blind widening rode alongside every subcommand
// PREFIX rule, and it shipped as a pair (rangerhq-3mc): `<cmd> -* <words>`
// for the bare spelling, `<cmd> -* <words> *` for the same verb carrying
// further arguments. Both halves are gone, removed one at a time for one
// reason. `*` is `.*`, so `-*` is not "an option run": it is a `-` and
// then ANYTHING, a nested subcommand or an option's value included.
//
//   - the exact half matched any `git -…` command whose LAST word was one
//     of the words — `git -C <r> log --grep push` and `git -C <r> stash
//     push` refused live, `--grep=push` running (claude 2.1.234, grok
//     1.0.5). rangerhq-ky3.
//   - the wildcard half matched any ` push ` token after a leading global
//     option, whether or not that token was the subcommand — `git -C <r>
//     stash push -m wip` (nested verb with arguments) and `git -C push
//     status -s` (`push` is -C's value) refused live on claude 2.1.239 and
//     grok 1.0.5, with `git stash push`, no leading option, running as the
//     control. rangerhq-vr6j.
//
// The L1 shim refuses none of those: it skips the global options and
// matches the FIRST non-option token, so the wildcard half was blocking
// argv the wall deliberately lets through
// (TestShimSkipsGlobalOptionsBeforeSubcommand pins both sides). And no
// spelling separates them from a real push: the dialect has neither
// negation nor a way to say "option tokens only" (a glob has no repetition
// of a group), so `git -C <r> push origin main` and `git -C <r> stash push
// -m wip` are the same pattern. At L0 a false positive is a hard block the
// model cannot ask its way past — the ground rangerhq-3mc rejected a
// single `Bash(git -* push*)` on — so the half goes and its coverage goes
// with it.
//
// The cost, stated: a subcommand deny reaches the spelling the PID wrote
// and nothing else. `git <globals> push …`, with further arguments or
// without, draws no polite refusal at all now, only L1's hard one. L0 is
// politeness, never the wall.
//
// A whole-verb rule (Bash(bd)) is the other half of the same miss: claude
// reads it as *exact*, so `bd show x` walks past a rule the shim reads as
// the whole verb. It gets `Bash(<cmd>:*)` alongside — a token-prefix rule
// with no wildcard in it, which is why it carries none of the above.
//
// Only deny lists go through this. Widening an allow list would grant more
// than the PID says; allow is friction, and stays the PID's words.
//
// And only claude's realizer calls it. The pair that made it unshippable
// on grok is gone (it matches a *shell-parsed* segment, quotes off, so
// `git -C <r> log --author "push me"` was refused there and runs on claude
// — rangerhq-625), but what is left is still claude's: a rule with no
// wildcard is a PREFIX on grok rather than an exact match, so the negative
// rule's spelling below, `Bash(git commit)`, would refuse the very
// `git commit -- <path>` the PID allows. So realizeGrok types the PID's
// own rules and L1 is the wall there (ADR 0009).
func L0Spellings(deny []string) []string {
	out := make([]string, 0, len(deny))
	seen := map[string]bool{}
	add := func(rule string) {
		if !seen[rule] {
			seen[rule] = true
			out = append(out, rule)
		}
	}
	for _, rule := range deny {
		cmd := shimCommand(rule)
		if cmd == "" {
			add(rule) // Edit, Write, mcp__*, or not a plain command name
			continue
		}
		r := ParseShimRules([]string{rule})[cmd][0]
		// A negative rule (`Bash(git commit unless --)`) has no spelling in
		// claude's dialect at all — it has no negation — and the rule text
		// itself would reach the matcher as a literal, matching nothing. What
		// CAN be said is the shape that is unsafe whatever follows: the bare
		// form with no leading options, EXACT so it cannot swallow a commit
		// that does carry the qualifier. Anything longer might be the safe
		// form, and refusing it at L0 would refuse the very form the wall is
		// pointing at.
		// ranger-base-xll2: the option-blind twin, `Bash(cmd -* words)`, used
		// to ride alongside — the same shape rangerhq-ky3 removed from the
		// verb branch above, carrying the same false positive here (`git -C
		// <r> log --grep commit` was refused, measured; `--grep=commit` ran).
		// Unlike the verb branch, this one has no ` *` fallback: a trailing
		// wildcard would also catch the SAFE form (anything after `commit`,
		// including `-- <pathspec>`), so there was no narrower spelling to
		// fall back to — only drop-it-or-keep-it. The rangerhq-3mc/ky3
		// standard (at L0 a false positive is a hard block the model cannot
		// ask its way past) decides it the same way here: drop it. Stated
		// cost: `git <globals> commit -m x` with no `--` draws no polite L0
		// refusal, only L1's hard one (TestShimNegativeMatchUnless's "behind
		// git's global options" row pins that L1 still refuses it).
		if r.Unless != "" {
			words := strings.Join(r.Words, " ")
			add("Bash(" + cmd + " " + words + ")")
			continue
		}
		add(rule)
		if len(r.Words) == 0 {
			add("Bash(" + cmd + ":*)")
		}
		// And nothing else. A subcommand rule used to get an option-blind
		// wildcard here; both halves of that pair are gone (rangerhq-ky3,
		// rangerhq-vr6j — the block above says what they refused and why no
		// narrower spelling exists). A rule leading with an option
		// (Bash(rm -rf /)) is a literal argv prefix in both matchers — it is
		// already spelled where it means.
	}
	return out
}

// GatesDir is RHQ_HOME/state/gates/<persona>.
func (a *App) GatesDir(persona string) string {
	return filepath.Join(a.StateDir, "gates", persona)
}

// RenderGates writes the persona's shims and gate shell fresh and returns
// the gates dir, its bin dir and the gate shell's path. Existing shims are
// removed first so a rule dropped from the PID stops being enforced.
// refusals.log and shell.log are kept across renders.
func (a *App) RenderGates(persona string, deny []string) (gatesDir, binDir, shell string, err error) {
	gatesDir = a.GatesDir(persona)
	binDir = filepath.Join(gatesDir, "bin")
	if err := os.RemoveAll(binDir); err != nil {
		return "", "", "", err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", "", "", err
	}
	rules := ParseShimRules(deny)
	cmds := make([]string, 0, len(rules))
	for c := range rules {
		cmds = append(cmds, c)
	}
	sort.Strings(cmds)
	log := filepath.Join(gatesDir, "refusals.log")
	// The refusal path's own `date`, resolved exactly the way each shimmed
	// binary is — see refusalTimestamp for why a bare name is not safe here.
	dateBin := resolveOutside("date", binDir)
	for _, c := range cmds {
		real := resolveOutside(c, binDir)
		script := renderShim(persona, c, real, log, dateBin, rules[c])
		if err := os.WriteFile(filepath.Join(binDir, c), []byte(script), 0o755); err != nil {
			return "", "", "", err
		}
	}
	shell, err = renderGateShell(persona, gatesDir, binDir)
	if err != nil {
		return "", "", "", err
	}
	return gatesDir, binDir, shell, nil
}

// PathOutsideGates is $PATH with binDir and every other posse gates bin dir
// dropped — the search path for the real binaries the shims stand in front
// of. Also how a process gets out from behind its own session's wall: the
// test suite runs inside a persona pane, and its git-push tests were being
// answered by that session's own L1 shim instead of by the code under test
// (rangerhq-8sd). Pass "" when there is no shim dir of one's own to drop.
func PathOutsideGates(binDir string) string {
	var keep []string
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == "" || p == binDir || strings.Contains(p, string(filepath.Separator)+"gates"+string(filepath.Separator)) {
			continue
		}
		keep = append(keep, p)
	}
	return strings.Join(keep, string(os.PathListSeparator))
}

// resolveOutside finds cmd on PATH ignoring binDir (and any other posse
// gates bin) — the real binary the shim execs. "" when not found: the shim
// then searches PATH itself at run time, still skipping its own dir.
func resolveOutside(cmd, binDir string) string {
	// Searched by hand rather than by swapping $PATH around an
	// exec.LookPath. The process environment is one variable shared by
	// every goroutine, so that swap was a window in which any concurrent
	// caller resolved against a PATH it never asked for — harmless while
	// this package's tests ran strictly one at a time, a race the moment
	// any of them calls t.Parallel (ranger-base-i7fa). Same answer, no
	// global touched: PathOutsideGates is a read.
	if strings.ContainsRune(cmd, filepath.Separator) {
		return absLookPath(cmd)
	}
	for _, dir := range filepath.SplitList(PathOutsideGates(binDir)) {
		cand := filepath.Join(dir, cmd)
		if !strings.ContainsRune(cand, filepath.Separator) {
			// A "." entry on PATH: keep the candidate a path, or
			// LookPath reads it as a bare name and searches $PATH.
			cand = "." + string(filepath.Separator) + cand
		}
		if abs := absLookPath(cand); abs != "" {
			return abs
		}
	}
	return ""
}

// absLookPath is exec.LookPath on a path that already names a directory,
// resolved to an absolute one. "" when it is not an executable file.
func absLookPath(path string) string {
	real, err := exec.LookPath(path)
	if err != nil {
		return ""
	}
	if abs, err := filepath.Abs(real); err == nil {
		return abs
	}
	return real
}

func shQuote(s string) string { return shellQuote(s) }

// whereHints names the commands whose refusal a HUMAN plausibly triggers
// himself, and which therefore earn a line saying WHERE the command does
// run. The operator's terminal is a persona pane: the `!` prefix runs in the
// current session, whose PATH leads with this shim dir, so the crew's
// keychain tripwire refused the operator's own credential read and the line
// named the rule and stopped (ranger-base-kn99, raised on ranger-base-okbr).
//
// A table and not a blanket, because the line is only honest where the
// refused reader might BE the operator. `security` is a tripwire on the crew
// — nothing in a pane should read the keychain, and the operator reads it
// constantly. A deny like Bash(git push:*) is the opposite: a control on an
// action that is the launcher's by design, where "run it outside posse"
// reads as the escape ranger-base-khu declined on purpose. One line per
// entry, deliberately: each entry is a judgment someone made.
var whereHints = map[string]bool{"security": true}

// ─── The runtime's own credential binary (ranger-base-eupf, ADR 0042) ──────

// CredGateCollision reports the PID deny rule that renders an L1 shim over
// the RUNTIME's own credential binary, or "" when there is none.
//
// The wall is doing exactly what it says; the finding is its BLAST RADIUS.
// The gates dir is prepended on the typed line (ADR 0002 §3), so it leads
// the PATH of the runtime process itself, not only of the persona's shells
// — and a runtime whose credential path execs its binary by BARE NAME
// therefore reads its own credential through this persona's refusal shim.
// MEASURED in a live session, 2026-08-29: `claude -p` under a PID denying
// that binary answers "Not logged in", and the same line with the gates dir
// stripped from PATH answers normally. The refusals were not a persona
// reaching for keys — 875 of them across the crew's logs since 2026-08-24,
// at a steady couple an hour, are the runtime asking for its own token and
// being told no.
//
// Two things follow, and only the first has ever been seen. A session
// starts on the token it already had, so the refusal shows up as a nested
// `claude` reporting itself logged out; at token EXPIRY the same wall sits
// in front of the refresh's write-back, which is the one moment this
// matters and the one nobody has watched.
//
// Why not a carve-out. The obvious narrowing — let the read-only forms
// through, refuse the rest — does not survive measurement of what the
// runtime actually runs (see Runtime.CredBin): the credential WRITE goes
// primarily through that binary's stdin batch mode, which takes its
// commands on stdin. An argv matcher cannot see them. A shim that let the
// batch mode through to keep refresh working would not be a narrowed deny,
// it would be no deny at all — and one that let only the reads through
// would fix the logged-out symptom while leaving expiry exactly where it
// is. ADR 0042 D1 settled the question this detector was built to raise:
// the deny stands, because the shim in front of the runtime's read is what
// keeps the operator's rotating pair single-writer. So the collision is no
// longer something a launch announces — it is a PRECONDITION on the launch
// (CheckCredGate below, ADR 0042 D2), and this function is unchanged: it
// still answers only "does this PID shim this runtime's credential binary".
//
// binDir is the persona's shim dir, used only to resolve the real binary
// the way the shim would: a `security` deny on a box that has no such
// binary shims nothing and collides with nothing, which is what keeps this
// from warning on a platform where the runtime reads a file instead.
func CredGateCollision(rt *Runtime, deny []string, binDir string) string {
	if rt == nil || rt.CredBin == "" {
		return ""
	}
	rules := ParseShimRules(deny)[rt.CredBin]
	if len(rules) == 0 || resolveOutside(rt.CredBin, binDir) == "" {
		return ""
	}
	// The whole-binary rule if the PID carries one — that is the rule that
	// refuses every form — else the first, in the PID's own order.
	for _, r := range rules {
		if len(r.Words) == 0 {
			return r.Rule
		}
	}
	return rules[0].Rule
}

// CheckCredGate is the launch precondition ADR 0042 D2 puts in the
// warning's place, at every renderer of a persona line: a launch whose PID
// shims the runtime's own credential binary launches only with that
// runtime's session mint among the env-set names it is about to inject,
// and REFUSES otherwise.
//
// Why a refusal now, where ranger-base-eupf shipped a warning. The warning
// said the session's credential was "frozen at whatever it started with"
// and offered dropping the rule from the PID; ADR 0042 measured both
// sentences false. Nothing is frozen — a crew runtime never held the
// operator's rotating pair in the first place, it runs on the session mint
// posse injects (D1), and the shim in front of its read is what keeps that
// pair single-writer. And dropping the rule is the one move D1 forbids. So
// the collision is not a cost to be announced: with the mint present it is
// the design and the launch says NOTHING, and without the mint the session
// cannot authenticate at all (MEASURED: "Not logged in") — which is a
// launch worth refusing rather than spending.
//
// Not waivable by --allow-degraded, and it is a plain error for exactly
// that reason: `degraded` is for a gate the wall could not realize, and a
// session that cannot authenticate is not a weaker session.
//
// names is what the session's env sets carry, asked the same way the caged
// precondition asks it (CheckCageCredential, cage.go) — same question, same
// key, one tier down.
func CheckCredGate(persona string, rt *Runtime, deny []string, binDir string, names []string) error {
	rule := CredGateCollision(rt, deny, binDir)
	if rule == "" {
		return nil
	}
	if CheckCageCredential(rt, names) == nil {
		return nil
	}
	return CredGateRefusal(persona, rt, rule)
}

// CredGateRefusal is the sentence that refusal says. It names the four
// things a reader needs to act: the rule in the PID's own spelling, the
// binary it shims, the key that is missing, and how the key is minted.
func CredGateRefusal(persona string, rt *Runtime, rule string) error {
	key := CageCredential(rt)
	if key == "" {
		return Die("posse: %s launches on %s under %s, which shims `%s` — the binary %s reads its own credential with — and no session credential is decided for %s (ADR 0042 D1/D2, rangerhq-kiz).\n"+
			"  The rule stays: the operator's store of record keeps ONE writer and it is not a crew process. Decide this runtime's session credential (`cage_cred:` for a template-only runtime) before launching a persona that carries the rule.\n",
			persona, rt.Name, rule, rt.CredBin, rt.Name, rt.Name)
	}
	return Die("posse: %s launches on %s under %s, which shims `%s` — the binary %s reads AND WRITES its own credential with — and %s is in none of this session's env sets (ADR 0042 D2).\n"+
		"  That shim is the design, not an accident: a crew runtime authenticates with the session mint posse injects and never with the operator's store of record, which keeps one writer (ADR 0042 D1). Without the mint this session opens logged out and cannot refresh at expiry, so posse refuses the launch rather than spending it.\n"+
		"  Mint it once by hand (on claude: `claude setup-token`), put it in an env set (mode 600, never in the repo), and name that set in this PID's envs: or pass --env-file. ANTHROPIC_API_KEY is metered spending and is not the session credential.\n"+
		"  Not waivable by --allow-degraded: this is not a gate the wall could not realize, it is a session that cannot authenticate.\n",
		persona, rt.Name, rule, rt.CredBin, rt.Name, key)
}

// whereHint is that line, or "" when cmd is not one a human types himself.
// It names the pane rather than offering a way past — a persona has none,
// and saying so is what keeps this from reading as an escape hatch.
func whereHint(persona, cmd string) string {
	if !whereHints[cmd] {
		return ""
	}
	return fmt.Sprintf("  this shell is %s's pane: posse's gate dir leads its PATH, so\n"+
		"  every shell in it is gated and a persona has no way past that.\n"+
		"  operator: run %s in a terminal outside posse.", persona, cmd)
}

// ruleHint is what the refusal says under the rule it names: for a NEGATIVE
// rule the form that is not refused, spelled out of the rule itself, and for
// a command a human plausibly types himself the pane he typed it in and
// where it does run. Both when both apply. Derived rather than written per
// rule so the grammar stays general — the git-specific advice ("-F -",
// "not '.'") belongs to the git-specific L3 hook, not here. The two keyed
// tables are the one seam where a command-specific fact reaches this layer,
// and both hold a fact about the QUALIFIER rather than advice about the
// command: what satisfies it and lies (qualifierSpoilers), and what it
// presumes of its own operand (qualifierPrereqs).
func ruleHint(persona, cmd string, r shimRule) string {
	var lines []string
	if r.Unless != "" {
		words := strings.Join(r.Words, " ")
		if words != "" {
			words += " "
		}
		hint := fmt.Sprintf("  safe form: %s %s… %s <operand> [<operand>…]", cmd, words, r.Unless)
		if sp := spoilersFor(cmd, r); len(sp.Opts) > 0 {
			// The reason gets its own line: the option list is as long as
			// the option set that spoils, and it grows (ranger-base-myai).
			hint += fmt.Sprintf(", and without %s —\n  %s", strings.Join(sp.Opts, "/"), sp.Why)
		}
		lines = append(lines, hint)
		if pre := prereqFor(cmd, r); pre != "" {
			lines = append(lines, pre)
		}
	}
	if w := whereHint(persona, cmd); w != "" {
		lines = append(lines, w)
	}
	return strings.Join(lines, "\n")
}

// setVars renders the assignment of the refusal's two variables for r. A
// multi-line hint stays one assignment: the newlines sit inside the single
// quotes shQuote puts round them, and `echo` in posse_refuse prints them.
func setVars(persona, cmd string, r shimRule) string {
	return fmt.Sprintf("RHQ_GATE_RULE=%s; RHQ_GATE_HINT=%s", shQuote(r.Rule), shQuote(ruleHint(persona, cmd, r)))
}

// ruleCond renders the test for one rule against the positional params in
// scope (the raw argv, or what is left after the globals are skipped).
func ruleCond(cmd string, r shimRule) string {
	conds := []string{}
	// A trailing long-option run is matched by MEMBERSHIP in what follows
	// the lead, never by position: a flag has no position (ranger-base-vct2).
	positional := r.Words
	if r.OptTail() != nil {
		positional = r.Lead()
	}
	for i, w := range positional {
		conds = append(conds, fmt.Sprintf("[ \"$%d\" = %s ]", i+1, shQuote(w)))
	}
	for _, o := range r.OptTail() {
		conds = append(conds, fmt.Sprintf("%s \"$@\"", flagInFunc(r, o)))
	}
	if r.Exact {
		conds = append(conds, fmt.Sprintf("[ \"$#\" -eq %d ]", len(r.Words)))
	}
	if r.Unless != "" {
		q := fmt.Sprintf("! posse_qualified %s \"$@\"", shQuote(r.Unless))
		if fn := spoiledFunc(cmd, r); fn != "" {
			// Refuse when the qualifier is missing OR when it is there and
			// lying: `git commit -i -m x -- b.txt` carries a pathspec and
			// commits the shared index anyway.
			q = fmt.Sprintf("{ %s || %s \"$@\"; }", q, fn)
		}
		conds = append(conds, q)
	}
	if len(conds) == 0 {
		return "true"
	}
	return strings.Join(conds, " && ")
}

// refusalTimestamp is the shell expression posse_refuse expands for the log
// line's time. It is rendered from an ABSOLUTE path, not the bare name
// `date`, because the shim dir leads the session's PATH by construction
// (ADR 0009 §1): under a PID carrying `Bash(date:*)` the bare form made the
// refusal path call this persona's OWN date shim, which refused, logged,
// and called `date` again — an unbounded fork chain on the one verb the
// refusal itself uses (ranger-base-hr5x). Every other command in the shim
// is a shell builtin or an $RHQ_GATE_* expansion, so this is the whole
// cycle: with it broken, a bare `date` elsewhere in the gates costs one
// refused child, not a fork storm.
//
// "" when `date` is nowhere outside the gates — vanishingly unlikely, and
// the answer is a line that keeps its shape with the time unknown rather
// than one that reopens the loop.
func refusalTimestamp(dateBin string) string {
	if dateBin == "" {
		return "-"
	}
	return "$(" + shQuote(dateBin) + " -u +%Y-%m-%dT%H:%M:%SZ)"
}

// quotedStamp is refusalTimestamp for a site that is assembled INSIDE a
// double-quoted shell assignment and evaluated later — the gate shell's
// user-command note, whose text becomes an argv word the runtime's shell
// evals after the snapshot replay. The `$` must survive that first layer of
// quoting, so it is escaped exactly like the `\$PATH` beside it.
func quotedStamp(dateBin string) string {
	return strings.ReplaceAll(refusalTimestamp(dateBin), "$", `\$`)
}

// renderShim writes the POSIX sh shim for one command.
func renderShim(persona, cmd, real, log, dateBin string, rules []shimRule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#!/bin/sh\n# posse gate for %s — rendered from the PID's deny: at launch; do not edit (%s)\n", persona, gateShimMarker)
	fmt.Fprintf(&b, "RHQ_GATE_LOG=%s\n", shQuote(log))
	fmt.Fprintf(&b, "posse_refuse() {\n  echo \"refused by posse gate: %s $* (deny: $RHQ_GATE_RULE)\" >&2\n", cmd)
	b.WriteString("  [ -n \"$RHQ_GATE_HINT\" ] && echo \"$RHQ_GATE_HINT\" >&2\n")
	fmt.Fprintf(&b, "  echo \"%s %s $* (deny: $RHQ_GATE_RULE)\" >> \"$RHQ_GATE_LOG\" 2>/dev/null\n  exit 1\n}\n", refusalTimestamp(dateBin), cmd)
	// A negative rule needs one helper: does argv carry the qualifier WITH
	// an operand? The bare token is not enough — `git commit --` with an
	// empty pathspec commits the shared index exactly like the bare form
	// (measured, rangerhq-lmq9).
	for _, r := range rules {
		if r.Unless == "" {
			continue
		}
		b.WriteString("posse_qualified() {\n  posse_q=$1; shift\n  while [ $# -gt 0 ]; do\n    if [ \"$1\" = \"$posse_q\" ]; then\n      [ $# -gt 1 ] && return 0\n      return 1\n    fi\n    shift\n  done\n  return 1\n}\n")
		break
	}
	// And a second helper per rule whose qualifier has known false
	// negatives — options that carry the qualifier and do the unsafe thing
	// anyway (rangerhq-ojnw).
	seenSpoiled := map[string]bool{}
	for _, r := range rules {
		fn := spoiledFunc(cmd, r)
		if fn == "" || seenSpoiled[fn] {
			continue
		}
		seenSpoiled[fn] = true
		b.WriteString(renderSpoiled(fn, spoilersFor(cmd, r)))
	}
	// And one per option a rule matches by membership rather than position.
	seenFlagIn := map[string]bool{}
	for _, r := range rules {
		for _, o := range r.OptTail() {
			fn := flagInFunc(r, o)
			if seenFlagIn[fn] {
				continue
			}
			seenFlagIn[fn] = true
			b.WriteString(renderFlagIn(fn, verbValueOptsFor(cmd, r), o))
		}
	}
	// Rules that lead with an option are a literal argv prefix: matched
	// where they are written. Rules that name a subcommand are matched
	// after the command's global options are skipped (rangerhq-2zm).
	var verbs []shimRule
	for _, r := range rules {
		if r.Verb() {
			verbs = append(verbs, r)
			continue
		}
		fmt.Fprintf(&b, "if %s; then %s; posse_refuse \"$@\"; fi\n", ruleCond(cmd, r), setVars(persona, cmd, r))
	}
	if len(verbs) > 0 {
		fmt.Fprintf(&b, "# Skip %s's leading global options, then match the first non-option\n", cmd)
		b.WriteString("# token: 'git -C <repo> push' is still a push (rangerhq-2zm).\n")
		b.WriteString("posse_verb_match() {\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n")
		if pairs := globalValueOpts[cmd]; len(pairs) > 0 {
			quoted := make([]string, 0, len(pairs))
			for _, o := range pairs {
				quoted = append(quoted, shQuote(o))
			}
			fmt.Fprintf(&b, "      %s)\n        [ $# -ge 2 ] || break\n        shift 2 ;;\n", strings.Join(quoted, "|"))
		}
		b.WriteString("      -*) shift ;;\n      *) break ;;\n    esac\n  done\n")
		for _, r := range verbs {
			fmt.Fprintf(&b, "  if %s; then %s; return 0; fi\n", ruleCond(cmd, r), setVars(persona, cmd, r))
		}
		b.WriteString("  return 0\n}\n")
		// Called directly, not through $(...): the matcher sets the rule and
		// its hint as globals, and a subshell would drop them. `shift` inside
		// a function touches only that function's positional params.
		b.WriteString("posse_verb_match \"$@\"\n")
		b.WriteString("if [ -n \"$RHQ_GATE_RULE\" ]; then posse_refuse \"$@\"; fi\n")
	}
	if real != "" {
		fmt.Fprintf(&b, "exec %s \"$@\"\n", shQuote(real))
	} else {
		// Not on PATH at render time: search at run time, skipping our own dir.
		fmt.Fprintf(&b, "self=$(cd \"$(dirname \"$0\")\" && pwd)\nIFS=:\nfor d in $PATH; do\n  [ \"$d\" = \"$self\" ] && continue\n  if [ -x \"$d/%s\" ]; then unset IFS; exec \"$d/%s\" \"$@\"; fi\ndone\necho \"posse gate: %s: real binary not found\" >&2\nexit 127\n", cmd, cmd, cmd)
	}
	return b.String()
}

// ─── The gate shell (ADR 0009) ───────────────────────────────────────────────

// A runtime that re-execs a *login* shell per command (grok 1.0.5) hands
// PATH to macOS path_helper, which demotes the gates dir below /usr/bin —
// the typed prefix of ADR 0002 §3 is undone before the command runs and
// the shim never fires (rangerhq-vjl). So the shell itself becomes ours:
// next to the shims we render RHQ_HOME/state/gates/<persona>/shell/<base>,
// a POSIX sh wrapper that guards PATH inside the -c string (and inside a
// runtime's user-command slot, which runs after the snapshot replay) and
// then execs the real shell. The typed line points SHELL/GROK_SHELL at it
// on every runtime — uniform, so a future runtime that starts snapshotting
// a login shell inherits the fix instead of a silent regression.
//
// The guard tests PRECEDENCE, not presence. A presence test reads as the
// idempotent spelling, and ADR 0009 §1 wrote it that way, but it is a
// no-op exactly when it is needed: the typed line already puts the gates
// dir on PATH, so path_helper *demotes* it rather than dropping it, and
// `command -v git` still answered /usr/bin/git in a live grok session with
// the wrapper installed (rangerhq-e43). Re-prepending when the dir is not
// already first costs at most a duplicate entry, which PATH lookup ignores.
//
// A mis-parse is a LOUD failure (the persona's shell breaks), never a
// silent bypass. Runtime.NoGateShell (gate_shell: false) is the exit hatch
// for a runtime that chokes on a wrapper; parity then falls back to
// unrealized for Bash(...) denies there.

// gateShellScript is docs/adr/0009-gate-shell.probe.sh — the shape verified
// on grok 1.0.5, claude and codex 0.147 — with three placeholders rendered
// per persona. Keep it in step with the probe rather than re-deriving it.
const gateShellScript = `#!/bin/sh
# posse gate shell for __PERSONA__ — rendered at launch from the PID; do not edit (ADR 0009).
# Stands in for the login shell a runtime re-execs.
G=__GATES_BIN__   # rendered: RHQ_HOME/state/gates/<persona>/bin
REAL=__REAL__
LOG=__GATES_DIR__/shell.log
PRE="_rgp=; _rgr=\"\$PATH:\"; while [ -n \"\$_rgr\" ]; do _rge=\${_rgr%%:*}; _rgr=\${_rgr#*:}; case \"\$_rge\" in ''|*/gates/*) ;; *) _rgp=\"\$_rgp:\$_rge\";; esac; done; PATH=\"$G\$_rgp\"; export PATH; unset _rgp _rgr _rge; "
# The guard REBUILDS PATH rather than testing it, because it has two jobs.
# (a) This persona's gates dir must be FIRST, not merely present: the typed
# line already puts it on PATH, so path_helper (via /etc/zprofile, which runs
# before this -c string) demotes it below /usr/bin instead of dropping it
# (rangerhq-e43). (b) NO OTHER persona's gates dir may be on PATH at all: a
# persona hand-launched from another persona's pane inherits that pane's PATH,
# whose head is the LAUNCHING persona's shim dir, and a verb only THAT PID
# denies has no shim of ours to shadow it — so the launched session is refused
# by a rule it does not carry (rangerhq-v553). "Ours" is spelled the way
# PathOutsideGates spells it, a 'gates' path ELEMENT, so /opt/gateskeeper is
# not ours here either; $G is dropped by that same test and re-prepended,
# which is what makes (a) and (b) one loop.
eval "$PRE"   # the wrapper's own env too: exec hands it to REAL, and an
              # interactive/login shell with no -c string sees it nowhere else.
# Walk argv like the shell does: leading -x/+x words are options (-o/-O/+o/+O
# and --rcfile/--init-file consume a value; '--' ends them). If a -c was
# seen, the first operand is the command string: prefix it. If the operand
# after that (argv0) is '--', the next one is grok's user-command slot: prefix
# it too, so the guard runs after the snapshot replay. Everything else passes.
n=$#; i=0; st=opts; cflag=0
while [ $i -lt $n ]; do
  a=$1; shift; i=$((i+1))
  case $st in
    opts)
      case "$a" in
        --) st=str ;;
        -o|+o|-O|+O|--rcfile|--init-file) st=optval ;;
        -[!-]*) case "${a#-}" in *[!a-zA-Z]*) ;; *c*) cflag=1;; esac ;;
        --*|+*) ;;
        *) if [ $cflag -eq 1 ]; then a="$PRE$a"; st=argv0; else st=done; fi ;;
      esac ;;
    optval) st=opts ;;
    str)  if [ $cflag -eq 1 ]; then a="$PRE$a"; st=argv0; else st=done; fi ;;
    argv0) if [ "$a" = "--" ]; then st=usercmd; else st=done; fi ;;
    usercmd) a="case \"\$PATH:\" in \"$G\":*) ;; *) echo \"__STAMP__ gates dir not first in replayed PATH; re-prepended (path_helper/rc reorder?)\" >> '$LOG' 2>/dev/null;; esac; $PRE$a"; st=done ;;
    done) ;;
  esac
  set -- "$@" "$a"
done
exec "$REAL" "$@"
`

// The two placeholder-free ends of PRE above, as they appear in the argv of
// anything the gate shell exec'd — read by the load guard's orphan report
// (loadguard.go), which is the one consumer that must recognise our own
// preamble in a stranger's `ps` row.
//
// A forked subshell never execs, so it keeps its parent's whole -c string:
// that is what makes the preamble a reliable "this process came out of a
// gated command" marker, and it is the same property that sent teau's first
// diagnosis into the preamble when the argv was read as a stack. Both ends
// are needed and neither is decoration — the HEAD says the string is ours,
// and the TAIL is where our text stops and the persona's command begins, so
// the report can show what the process actually was. Everything between them
// interpolates $G (the persona's gates bin) and cannot be matched literally.
//
// TestGateShellPreambleEndsMatchTheScript renders the script and pins both
// against it, so a preamble edit that forgets this pair fails there rather
// than silently turning the report off.
const (
	gateShellPreambleHead = `_rgp=; _rgr="$PATH:"; while [ -n "$_rgr" ]; do `
	gateShellPreambleTail = `; export PATH; unset _rgp _rgr _rge; `
)

// realShell resolves the shell the gate wrapper execs and the basename it
// must be installed under: $SHELL when it is a bash or zsh that is really
// there, else the first of zsh/bash on PATH, else /bin/sh. The basename
// matters — a runtime that picks its snapshot dialect from the shell's
// name (grok does) must still pick right through the wrapper.
//
// The search is not decoration. At `cage: container` this same renderer
// runs INSIDE the image (rangerhq-6so), where $SHELL is unset and
// /bin/zsh does not exist — and a wrapper whose REAL cannot be exec'd is a
// dead gate shell, which is a shell verb that is not refused. Resolution
// happens where the binaries are, on both sides of the boundary; the host
// keeps its old answer because $SHELL is set there and /bin/zsh exists.
//
// A candidate that is itself a rendered wrapper is REFUSED, at both arms.
// This is the whole of ranger-base-f0ay: a wrapper is installed as
// state/gates/<persona>/shell/zsh, so it has a shell's basename and stats
// like one, and every property this function used to test was true of it.
// A render running while $SHELL was another persona's wrapper therefore
// captured that wrapper as REAL, and wrappers chained persona-to-persona
// instead of ending at a shell. On 2026-08-27 the chain closed into a
// two-node cycle and every spawn entering it exec-looped, each hop
// prepending its PRE guard to the -c string until E2BIG ~40 minutes later:
// the fleet-wide Bash wedge. Refusing costs nothing when $SHELL is honest
// and falls through to the search, which sheds gates dirs already.
func realShell(binDir string) (real, base string) {
	if s := os.Getenv("SHELL"); s != "" {
		switch b := filepath.Base(s); b {
		case "bash", "zsh":
			if st, err := os.Stat(s); err == nil && !st.IsDir() && !isGateWrapper(s) {
				return s, b
			}
		}
	}
	for _, b := range []string{"zsh", "bash"} {
		if p := resolveOutside(b, binDir); p != "" && !isGateWrapper(p) {
			return p, b
		}
	}
	// Every image has /bin/sh, and the wrapper script is POSIX sh — so this
	// is a working gate shell, not a placeholder. It costs the dialect a
	// runtime might infer from the name, which is why it is last.
	return "/bin/sh", "sh"
}

// gateShellMarker is the wrapper's own signature, carried on its second
// line. Content is the only thing that tells a wrapper apart from the
// shell it stands in front of: it is NAMED zsh on purpose, and it is a
// readable executable file, so name and stat both say "shell". The marker
// travels with the file — across a second RHQ_HOME, and across the cage
// render at CageStateRoot, neither of which is under this host's state
// dir. TestGateShellScriptCarriesItsMarker pins it to the script.
const gateShellMarker = "posse gate shell"

// isGateWrapper reports whether p is one of our rendered gate shells and
// so must never be exec'd by another one (ranger-base-f0ay).
//
// Two tests, either sufficient, because they fail in opposite directions:
// the path test recognizes a wrapper whose content we cannot read, and the
// content test recognizes one that a second RHQ_HOME put somewhere this
// process would not think to look. A `gates` PATH ELEMENT is how
// PathOutsideGates already spells "ours", and it is the same test — so
// /opt/gateskeeper/bin/zsh is not one of ours here either. False on error:
// an unreadable candidate is refused a line later by os.Stat anyway, and
// the render-time assertion is the backstop for whatever slips past.
func isGateWrapper(p string) bool {
	if p == "" {
		return false
	}
	sep := string(filepath.Separator)
	if strings.Contains(p, sep+"gates"+sep) {
		return true
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head, _ := io.ReadAll(io.LimitReader(f, 512))
	return strings.Contains(string(head), gateShellMarker)
}

// renderGateShell writes gates/<persona>/shell/<basename> and returns its
// path. The dir is cleared first, so a wrapper left by a different $SHELL
// does not linger.
func renderGateShell(persona, gatesDir, binDir string) (string, error) {
	real, base := realShell(binDir)
	return writeGateShell(persona, gatesDir, binDir, real, base)
}

// writeGateShell is renderGateShell's second half, split off so the
// invariant below can be asserted against a REAL of the test's choosing —
// realShell alone refuses the wrapper it is handed, and a defense that
// only its own caller can reach is a defense nothing pins.
//
// THE INVARIANT: a wrapper's REAL must resolve OUTSIDE every gates dir.
// Refusing the render is the loud failure ADR 0009 asks for everywhere
// else in this file — the launch fails, someone reads why. Writing the
// chain link instead is silent until the day it closes a cycle and takes
// the fleet down for two hours with zero bytes of output (2026-08-27).
// It is asserted before the dir is cleared: a render this refuses must
// not also be the thing that removes a working wrapper.
func writeGateShell(persona, gatesDir, binDir, real, base string) (string, error) {
	if isGateWrapper(real) {
		return "", fmt.Errorf("refusing to render %s's gate shell: REAL would be a gate wrapper (%s), not a shell — wrappers must exec a real shell, never each other (ADR 0009 §1; ranger-base-f0ay). $SHELL=%s: this render is running under a gated context",
			persona, real, os.Getenv("SHELL"))
	}
	dir := filepath.Join(gatesDir, "shell")
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	script := strings.NewReplacer(
		"__PERSONA__", persona,
		"__GATES_BIN__", shQuote(binDir),
		"__REAL__", shQuote(real),
		"__GATES_DIR__", shQuote(gatesDir),
		// The note's own timestamp, resolved here for the reason
		// refusalTimestamp gives — and the note is exactly the moment the
		// bare form was worst: it fires when the gates dir is NOT first in
		// the replayed PATH, so `date` is looked up against a PATH some
		// other element leads, before PRE has rebuilt it.
		"__STAMP__", quotedStamp(resolveOutside("date", binDir)),
	).Replace(gateShellScript)
	p := filepath.Join(dir, base)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		return "", err
	}
	return p, nil
}

// GatePrefix is what goes in front of the typed persona command so the
// shims win the PATH race: PATH=<bin>:$PATH, plus SHELL/GROK_SHELL pointing
// at the gate shell (ADR 0009 §2) so a runtime that re-execs a login shell
// re-execs ours. shell == "" (Runtime.NoGateShell) drops the two vars.
func GatePrefix(binDir, shell string) string {
	p := "PATH=" + shQuote(binDir) + `:"$PATH" `
	if shell != "" {
		p += "SHELL=" + shQuote(shell) + " GROK_SHELL=" + shQuote(shell) + " "
	}
	return p
}

// WrapWithGates renders the persona's gates and returns the command with
// the gate prefix typed in front, plus the gates dir for RHQ_GATES_DIR and
// the gate shell's path. The wrapper is rendered whatever the runtime; rt
// only decides whether the typed line points SHELL/GROK_SHELL at it
// (Runtime.NoGateShell drops them — ADR 0009 §2). rt may be nil.
func (a *App) WrapWithGates(persona string, rt *Runtime, deny []string, cmd string) (wrapped, gatesDir, shell string, err error) {
	gatesDir, binDir, shell, err := a.RenderGates(persona, deny)
	if err != nil {
		return "", "", "", err
	}
	typed := shell
	if rt != nil && rt.NoGateShell {
		typed = ""
	}
	return GatePrefix(binDir, typed) + cmd, gatesDir, shell, nil
}

// ─── L3: git pre-push hook ───────────────────────────────────────────────────

// prePushMarker identifies our hook so install replaces its own and never
// a foreign one.
const prePushMarker = "# posse-gate"

// legacyPrePushMarker / legacySharedIndexMarker are the pre-rename
// spellings (rangerhq-tyay). Ownership is a question about a file written
// by an EARLIER binary, so it cannot be asked in the new vocabulary alone:
// a repo hooked before the rename carries `# rhq-gate`, and matching only
// the new marker would make `posse gates install-hooks` refuse it as a
// stranger's hook and hookInstalled report the L3 wall missing on a repo
// that has it. Recognized, therefore replaced in place; the hook written
// back always wears the new marker, so this is a one-way door per repo.
const (
	legacyPrePushMarker     = "# rhq-gate"
	legacySharedIndexMarker = "# rhq-gate shared-index"
)

// ownsHook reports whether body is one of ours — this hook's marker in
// either spelling. Matched longest-first is unnecessary: each slot is
// asked only about its own marker pair, and no hook carries the other's.
func ownsHook(body, marker, legacy string) bool {
	return strings.Contains(body, marker) || strings.Contains(body, legacy)
}

// hookStampFunc is the timestamp helper both L3 hooks call for their
// refusals.log lines. They used to spell `date` bare, and a hook runs
// inside the persona's session: the gate shim dir leads that PATH by
// construction (ADR 0009 §1), so under a PID carrying Bash(date:*) the
// bare name reached the persona's OWN date shim, which refused. The line
// was written with no time in front of it and a stray "refused by posse
// gate: date" landed on stderr in the middle of a git command
// (ranger-base-l97n, the residual of ranger-base-hr5x — bounded at one
// refused child there, since hr5x took the cycle out of the shim itself).
//
// The lookup skips every gates dir, spelled the way PathOutsideGates
// spells it (a 'gates' path ELEMENT, so /opt/gateskeeper is not ours), and
// walks PATH by chopping rather than by IFS-splitting it: the same shape
// the gate shell's PRE uses, and for the same reasons — no IFS to restore
// for the caller and no glob in a PATH element to expand.
//
// PATH ITSELF IS LEFT ALONE. Scrubbing the gates dirs off it for the
// hook's duration is the shorter fix and the wrong one: installHook chains
// a foreign hook behind ours, and that hook would then run with the
// persona's fence taken off its PATH. The timestamp is cosmetic; the fence
// is not.
//
// "-" when date is nowhere outside the gates — a line that keeps its shape
// with the time unknown, never one that re-enters a shim.
const hookStampFunc = `# The refusal lines below timestamp with posse_stamp and never with a bare
# 'date': this hook runs inside a persona session whose PATH leads with that
# session's gate shim dir, so the bare name is answered by the session's own
# gate (ranger-base-l97n). The lookup skips every gates dir and leaves PATH
# itself alone — a chained foreign hook inherits this PATH.
posse_stamp() {
  posse_sd=''; posse_sr="$PATH:"
  while [ -n "$posse_sr" ]; do
    posse_se=${posse_sr%%:*}; posse_sr=${posse_sr#*:}
    case "$posse_se" in ''|*/gates/*) continue ;; esac
    if [ -x "$posse_se/date" ]; then posse_sd=$posse_se/date; break; fi
  done
  if [ -n "$posse_sd" ]; then "$posse_sd" -u +%Y-%m-%dT%H:%M:%SZ; else printf '%s' -; fi
}
`

// PrePushHook is the L3 wall for the one verb that is a hard risk line:
// a pre-push hook that refuses when RHQ_TOOLS_DENY (newline-separated,
// exported into every persona session by CreateSession) carries a rule
// matching git push — Bash(git push:*), Bash(git push --force:*),
// Bash(git:*), Bash(git). Catches /usr/bin/git push and subprocess pushes
// that keep the env — cooperative class at every tier (ADR 0025 §1): it
// cannot see through `env -i` (nothing in-process can), `--no-verify`
// skips it outright, and `-c core.hooksPath=` redirects past it with zero
// writes (measured, ranger-base-3csb). At cage: container the push's
// EFFECT can still die at an enforced layer (mount :ro / egress proxy)
// where the launch is configured for it (ADR 0025 §3) — the verb gate
// itself never gets stronger just because the process is caged.
const PrePushHook = `#!/bin/sh
` + prePushMarker + ` — installed by posse gates install-hooks; refuses git push in persona
# sessions whose PID denies it (RHQ_TOOLS_DENY). Foreign hooks are never
# overwritten; remove this file to uninstall. ADR 0002 §3 (rangerhq-8s4).
[ -n "$RHQ_TOOLS_DENY" ] || exit 0
` + hookStampFunc + `# Split the rules with a for-loop over IFS=newline, NOT with
# 'printf | while read': the right side of a pipeline is a subshell, so the
# refusal's 'exit 1' would exit only the subshell and reach git solely by
# being the script's last statement. One line appended after it printed the
# refusal and exited 0 — git pushed (rangerhq-kk6e). 'set -f' keeps the ':*'
# in a rule from globbing; every path out of here exits explicitly, so
# appended text is inert whatever the verdict.
set -f
IFS='
'
for rule in $RHQ_TOOLS_DENY; do
  body=${rule#Bash(}
  [ "$body" = "$rule" ] && continue
  body=${body%)}
  body=${body%:\*}
  case "$body" in
    git|"git push"|"git push "*)
      echo "refused by posse gate: git push (deny: $rule) — pre-push hook, session ${RHQ_PERSONA:-?}" >&2
      if [ -n "$RHQ_GATES_DIR" ]; then
        echo "$(posse_stamp) git push [pre-push hook] (deny: $rule) session ${RHQ_PERSONA:-?}" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
      fi
      exit 1
      ;;
  esac
done
exit 0
`

// hooksDir is WHERE GIT DISPATCHES THIS REPO'S HOOKS, which is not always
// `<git-common-dir>/hooks`: `core.hooksPath` overrides that outright, at any
// config level, and the slot under the git dir then stays inert. Deriving the
// path ourselves made every L3 claim a statement about a file rather than
// about git's behavior — install wrote where git would not read
// (rangerhq-b38m), and the probe exec'd a hook git would never run and
// reported `behavior probed` over a wall that was not there
// (ranger-base-flz7). Executing a script is only evidence if git is the one
// who would execute it.
//
// So ask git instead of reconstructing its answer. `--git-path hooks` is the
// same lookup git's own `find_hook()` performs, and it settles three things at
// once — all MEASURED on this host's git 2.39.3 rather than reasoned from the
// docs:
//
//   - core.hooksPath wins when set, absolute or relative;
//   - with it unset, a linked worktree still resolves to the COMMON hooks dir
//     (`<main>/.git/hooks`, never the per-worktree `worktrees/<n>/hooks`), so
//     a worktree keeps getting the hooks of the repo it belongs to rather than
//     none — the property the old spelling was chosen for, preserved for free;
//   - a relative value comes back rewritten relative to the CWD (`../myhooks`
//     from a subdirectory), so joining it onto `dir` is right at any depth.
//     git resolves relative hooksPath against the worktree top-level, and this
//     is what saves us from having to know that.
func hooksDir(dir string) (string, error) { return gitPath(dir, "hooks") }

// gitPath is that doctrine with the name as a parameter, because ADR 0038
// asks the same question about a second file: `--git-path config` is the
// lookup git's own `git_path()` performs, and it knows things a join does
// not. MEASURED on this host (git 2.50.1, darwin 25.4.0) from a linked
// worktree: `config` comes back as the COMMON dir's — `<main>/.git/config`,
// never the per-worktree `worktrees/<n>/config` — which is the file every
// git in that repo actually reads, while `config.worktree`, `gitdir` and
// `commondir` come back per-worktree. Deriving either would have been a
// coin flip dressed as a path.
//
// A relative answer joins against `dir` exactly as it did for hooks: git
// rewrites a relative value against the CWD it was asked from, so the join
// is right at any depth.
func gitPath(dir, name string) (string, error) {
	p, err := gitPathRaw(dir, name)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return p, nil
}

// gitPathRaw is gitPath before the join: git's answer exactly as git spelled
// it. Only one question needs it — ADR 0052 D1 asks whether the dispatch path
// is ABSOLUTE, and after the join every answer is (a relative `core.hooksPath`
// comes back joined onto dir and is indistinguishable from a value the
// operator wrote out in full). Everything else wants the joined form and
// should keep calling gitPath.
func gitPathRaw(dir, name string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-path", name).Output()
	if err != nil {
		return "", Die("%s is not a git repository", dir)
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", Die("%s is not a git repository", dir)
	}
	return p, nil
}

// ─── a MANAGED hooks path (ADR 0052 D1) ──────────────────────────────────────
//
// A managed box points every git on it at one absolute, root-owned hooks
// directory outside every repo — a secret-scanner integration owning
// `core.hooksPath`. posse's answer there is to write NOTHING: `install-hooks`
// used to reach os.WriteFile and hand the operator
// `open <dir>/pre-push: permission denied`, because installHook swallows the
// read error on a missing slot and falls straight through to the create
// (ranger-base-yt6m0, the operator's cold install). The employer's control is
// not fought, bypassed, or chained behind silently; the L3 wall is realized
// instead by the per-session hooks dir the session env aims git at (ADR 0052
// D2), and every caller that would write says so with managedHooks.line().

// managedHooks is managedHooksDir's verdict about one repo's dispatch path,
// carrying what the report line has to name: the directory, and the owner and
// mode that say whose it is.
type managedHooks struct {
	// Dir is git's dispatch path, joined as gitPath would join it — the
	// same string hooksDir(dir) returns, so a caller that has this verdict
	// does not have to ask git twice.
	Dir     string
	Managed bool
	Owner   string // the directory's owner uid, "?" when the stat cannot say
	Mode    string // its permission bits, e.g. 0555
}

// line is the one thing posse prints about a managed path, identical from
// install-hooks, the launcher and the hook-wall sweep. One line, on purpose:
// Degraded and Skip values flow into flat-file session meta, where an embedded
// newline truncates on read-back (ranger-base-ujdg).
func (m managedHooks) line() string {
	return fmt.Sprintf("L3: managed hooks path %s (owner %s, mode %s) — posse's wall is not installed there; realized by session redirect (ADR 0052)",
		AbbrevHome(m.Dir), m.Owner, m.Mode)
}

// managedHooksError is the verdict installHook returns INSTEAD of attempting
// the create, so a caller reading the error gets the classification rather
// than errno's account of it.
type managedHooksError struct{ m managedHooks }

func (e managedHooksError) Error() string { return e.m.line() }

// ManagedHooksPath is managedHooksDir for the CLI: the report line and
// whether the repo at dir is managed. A directory git cannot answer for is
// not managed — the caller's ordinary path reports that better than this can.
func ManagedHooksPath(dir string) (string, bool) {
	m, err := managedHooksDir(dir)
	if err != nil || !m.Managed {
		return "", false
	}
	return m.line(), true
}

// managedHooksDir classifies the repo at dir's hook dispatch path BEFORE
// anything writes to it. MANAGED iff all three hold (ADR 0052 D1):
//
//   - git's `--git-path hooks` answer is ABSOLUTE. A relative
//     `core.hooksPath` is a path inside the operator's own tree, resolved by
//     git against the worktree top-level — nobody else's directory, whatever
//     its mode. Asked of gitPathRaw, because the join makes every answer
//     absolute.
//   - it is NOT under the repo's common git dir, and not inside the worktree.
//     `.git/hooks` with its write bit off is a repo the operator locked, not
//     an employer's wall, and posse's refusal there should stay the one it
//     has always given.
//   - this uid cannot create a file in it — measured, by ONE create probe of
//     a dot-file that is removed on success. Never by opening a slot: a slot
//     may be a FIFO whose open never returns (ranger-base-92n5p), and a hook
//     posse is refusing to touch is the last file to go poking at.
//
// Any subset keeps today's behaviour: the chain prescription and a degraded
// launch. The probe's error is read narrowly — permission and a read-only
// filesystem are "cannot create"; ENOENT, ENOSPC and the rest are not, so a
// full disk does not silently reclassify every foreign hooks path on the box
// as somebody's managed one.
//
// What the probe measures is "THIS process cannot create a file here", which
// is the fact every caller acts on and is not always a mode bit: the same
// answer comes back for a path a seatbelt profile denies (ADR 0025 §2). That
// is the honest verdict for the write, and the line names the owner and mode
// so a reader can see which it was.
func managedHooksDir(dir string) (managedHooks, error) {
	raw, err := gitPathRaw(dir, "hooks")
	if err != nil {
		return managedHooks{}, err
	}
	if !filepath.IsAbs(raw) {
		return managedHooks{Dir: filepath.Join(dir, raw)}, nil
	}
	m := managedHooks{Dir: raw}
	for _, flag := range []string{"--git-common-dir", "--show-toplevel"} {
		// A bare repo has no top level and answers with an error; that is
		// one leg not applying, not a failure to classify.
		p, err := git(dir, "rev-parse", flag)
		if err != nil || p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if underDir(p, m.Dir) {
			return m, nil
		}
	}
	st, err := os.Stat(m.Dir)
	if err != nil || !st.IsDir() {
		// Nothing there to be managed by anyone. A hooks path that does not
		// exist is today's MkdirAll's business, and it reports its own
		// failure in its own words.
		return m, nil
	}
	m.Owner, m.Mode = ownerAndMode(st)
	f, err := os.CreateTemp(m.Dir, ".posse-write-probe-*")
	if err == nil {
		name := f.Name()
		f.Close()
		os.Remove(name)
		return m, nil
	}
	m.Managed = cannotCreate(err)
	return m, nil
}

// ownerAndMode is the pair the report line names, formatted once so every
// caller says it the same way.
func ownerAndMode(fi os.FileInfo) (owner, mode string) {
	mode = fmt.Sprintf("%04o", fi.Mode().Perm())
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return strconv.FormatUint(uint64(st.Uid), 10), mode
	}
	return "?", mode
}

// cannotCreate reads the create probe's error as an answer about PERMISSION
// rather than about the moment. EROFS is spelled out because it is not
// os.ErrPermission and is the same fact — a mount posse may not write.
func cannotCreate(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EROFS)
}

// chainHookDispatcher is the dispatcher chainDispatcher tells the operator
// to install when another tool already owns a hook slot. Keeping the runnable
// body in one place also lets installHook recognize that exact arrangement:
// the dispatcher and the other tool's hook stay foreign, while the
// marker-owned posse hook behind them can still be refreshed on every launch.
// `theirs-<slot>` is the generic name the printed prescription reaches for
// first (chainNeighbourName picks another when that one is taken), and the
// operator renames the file after the tool that owns it, so recognition goes
// through chainHookDispatcherWith.
func chainHookDispatcher(slot string) string {
	return chainHookDispatcherWith(slot, "theirs-"+slot)
}

// chainHookDispatcherWith renders that dispatcher handing off to a named
// sibling hook. The name is a parameter because the prescription is a shape,
// not one filename: INSTALL.md §9 walks the arrangement with bd's hooks moved
// to `bd-<slot>`, and an operator who followed those words to the letter has
// the prescribed chain (ranger-base-r5ba).
func chainHookDispatcherWith(slot, neighbor string) string {
	return chainRender(slot, neighbor, true)
}

// legacyChainHookDispatcherWith is the same dispatcher as it was rendered
// before rangerhq-xo65 — no guard, an unconditional exec. It is still a
// dispatcher that runs our gate as its own process and honors its exit
// status, so a repo carrying one is still L3-certified and its posse-<slot>
// is still ours to refresh; it is only the NEIGHBOUR handoff that is unsafe.
// Recognizing it is what lets installHook upgrade it in place instead of
// abandoning every repo chained before the fix.
func legacyChainHookDispatcherWith(slot, neighbor string) string {
	return chainRender(slot, neighbor, false)
}

// chainRender renders the dispatcher body. guard adds the line that makes a
// missing neighbour a DEGRADE instead of a dead repo: `exec` on a file that
// is not there exits 126/127, and a prepare-commit-msg that exits non-zero
// blocks EVERY commit in the repo — including the ones the gate itself
// passes: the path-limited form it names as the way through, a merge, a
// rebase continue, and every commit in a linked worktree, where the gate
// stands down outright. The gate refuses one form and leaves a way out; an
// exec failure refuses all of them and leaves none (it keys on nothing —
// not on RHQ_PERSONA, which the gate itself stopped keying on in
// rangerhq-lt2w). That state is reachable without doing anything wrong: the mv
// that creates bd-<slot> fails silently in a pasted block if `bd hooks
// install` never took that slot (older bd planted no prepare-commit-msg at
// all; `--beads`/`--shared` write elsewhere entirely). With the guard, the
// slot degrades to "gate only" — which is the whole of what posse promises
// there anyway (rangerhq-xo65).
func chainRender(slot, neighbor string, guard bool) string {
	return chainRenderPath(slot, "$d/"+neighbor, guard)
}

// chainRenderPath is chainRender with the neighbour spelled OUT — the whole
// word the body puts inside the double quotes, `$d/theirs-pre-push` for a
// sibling and an absolute path for the redirect dispatcher ADR 0052 D2
// renders, whose neighbour is the managed hooks dir's own slot and is not a
// sibling of anything. Byte-identical to what this function always rendered
// when the caller passes `$d/`+name, which is what chainRender is.
func chainRenderPath(slot, neighbor string, guard bool) string {
	stdin := ""
	if slot == "pre-push" {
		// git feeds pre-push the ref list on stdin. Ours does not read it;
		// keeping ours off it leaves it intact for the hook we exec into.
		stdin = " </dev/null"
	}
	guarded := ""
	if guard {
		guarded = fmt.Sprintf("[ -x \"%s\" ] || exit 0\n", neighbor)
	}
	return fmt.Sprintf(`#!/bin/sh
d=$(dirname "$0")
"$d/posse-%[1]s" "$@"%[2]s || exit $?
%[4]sexec "%[3]s" "$@"
`, slot, stdin, neighbor, guarded)
}

// isChainHookDispatcher reports whether body is the prescribed dispatcher for
// slot, whatever the neighboring hook is called. Everything but that one
// filename must match byte for byte — the point of the check is that the file
// demonstrably runs `posse-<slot>` first and honors its exit status, which is
// what makes refreshing that sibling honest rather than a claim about a hook
// nothing calls. The name itself must be a plain sibling filename: no path
// separator, no whitespace, nothing the shell would read as anything but a
// file in the same hooks dir. Two renders are accepted: the current one and
// the pre-rangerhq-xo65 one without the neighbour guard — both dispatch our
// gate and honor its status, which is the whole of what this decides.
func isChainHookDispatcher(body, slot string) bool {
	_, ok := chainDispatcherNeighbour(body, slot)
	return ok
}

// chainDispatcherNeighbour returns the neighbouring hook a dispatcher body
// hands off to, and whether body is a dispatcher at all. The name now appears
// twice (guard and exec), so it is read off the trailing exec line and the
// whole body is then compared against the render for that name — byte for
// byte, guarded form or the pre-rangerhq-xo65 unguarded one. Equality is the
// check; the extraction only says which render to compare against.
func chainDispatcherNeighbour(body, slot string) (string, bool) {
	const pre, post = "\nexec \"$d/", "\" \"$@\"\n"
	i := strings.LastIndex(body, pre)
	if i < 0 || !strings.HasSuffix(body, post) || len(body) < i+len(pre)+len(post) {
		return "", false
	}
	name := body[i+len(pre) : len(body)-len(post)]
	if name == "" || strings.ContainsAny(name, "/\\\"'`$ \t\n") || name == "." || name == ".." {
		return "", false
	}
	if body != chainHookDispatcherWith(slot, name) && body != legacyChainHookDispatcherWith(slot, name) {
		return "", false
	}
	return name, true
}

// chainDispatcher renders the only chaining form that holds when the slot
// is already taken by a foreign hook: each hook in its own file, ours run
// as its OWN PROCESS with its exit status checked. Never "append ours to
// theirs" — our refusal exits, so nothing after it would run anyway, and
// the appended form used to discard the refusal outright while still
// printing it (rangerhq-kk6e). Same words as INSTALL.md §9, which walks
// both slots at once; this names the one slot that refused.
func chainDispatcher(dir, hooks, slot string) string {
	probe := `t=$(mktemp); RHQ_PERSONA=probe ./` + slot + ` "$t"; echo $?; rm -f "$t"`
	if slot == "pre-push" {
		probe = `RHQ_PERSONA=probe RHQ_TOOLS_DENY='Bash(git push:*)' \
  sh -c 'printf "refs/heads/main a refs/heads/main b\n" | ./` + slot + ` origin x'; echo $?`
	}
	// Step 1 of the block below is a `cd`, so every path printed after it has
	// to still mean the same thing from the hooks directory. The repo
	// argument echoed AS TYPED does not: a relative one resolves against the
	// operator's old cwd, so after the cd install-hooks refuses ("<repo> is
	// not a git repository"), the gate is never written, and the mv and the
	// heredoc below still build a dispatcher around a file that is not there
	// — a slot that looks chained and exits 127 on every push
	// (ranger-base-87c9). Print it resolved, the way the cd line prints the
	// hooks dir git resolved. Abs only fails if the cwd is gone, and then the
	// argument as typed is the best that is left.
	repo := dir
	if abs, err := filepath.Abs(repo); err == nil {
		repo = abs
	}
	neighbour := chainNeighbourName(hooks, slot)
	collision := ""
	if neighbour != "theirs-"+slot {
		collision = fmt.Sprintf(`
theirs-%[1]s is already taken. A bare mv onto an existing file destroys it
without a word, and here that file is a hook — so the slot moves aside to
%[2]s instead. Nothing below touches theirs-%[1]s; read it
once the chain has run and delete it yourself if it is dead.
`, slot, neighbour)
	}
	// Flush-left and cd'd into the hooks dir on purpose: this is meant to
	// be pasted, and an indented heredoc body would write a shebang with
	// leading spaces and never reach its terminator.
	return fmt.Sprintf(`Chain it — each hook in its own file, ours dispatched first and its exit
status checked (INSTALL.md §9). Appending to ours is not a chain: our
refusal is an exit, so nothing pasted after it runs.
%[6]s
cd %[1]s
mv %[2]s %[7]s
posse gates install-hooks %[3]s
mv %[2]s posse-%[2]s
cat > %[2]s <<'EOF'
%[4]sEOF
chmod +x %[2]s

Then verify by running the slot, not by reading it — from that same dir:

%[5]s

It must print "refused by posse gate" and exit 1. A slot that prints the
refusal and exits 0 is not installed.`, AbbrevHome(hooks), slot, AbbrevHome(repo), chainHookDispatcherWith(slot, neighbour), probe, collision, neighbour)
}

// chainNeighbourName is the name chainDispatcher's first step moves the
// occupied slot to. `theirs-<slot>` is what INSTALL.md §9 calls it, but the
// step is a bare `mv`, and a bare `mv` onto an existing file destroys it
// silently. That name is taken in exactly the arrangement where an operator
// following posse's own instructions ends up re-reading this prescription: a
// slot already holding posse's dispatcher, with theirs-<slot> beside it. The
// paste used to overwrite the third party's hook with the dispatcher, which
// then `exec`s into itself forever — invisibly, because the loop sits past
// the gate's refusal and only a PERMITTED push ever reaches it
// (ranger-base-q32o). So the name is chosen against the directory rather than
// assumed: a free name destroys nothing, and the dispatcher takes any plain
// sibling filename (chainDispatcherNeighbour), so the chain it builds is
// recognized and refreshed like any other.
func chainNeighbourName(hooks, slot string) string {
	name := "theirs-" + slot
	for i := 2; i < 100; i++ {
		if _, err := os.Lstat(filepath.Join(hooks, name)); os.IsNotExist(err) {
			break
		}
		name = fmt.Sprintf("theirs-%s-%d", slot, i)
	}
	return name
}

// bdShimMarker identifies bd's own hook shim: `bd hooks install` plants a
// known, fixed, two-line `exec bd hooks run <slot> "$@"` body wearing this
// header in pre-push, prepare-commit-msg, pre-commit, post-merge and
// post-checkout (verified against the bd 0.49.1 binary). It is a foreign
// hook, but not a foreign hook of UNKNOWN shape — install-hooks --chain may
// take it over rather than refuse it (rangerhq-f2p5, rangerhq-mgdk).
const bdShimMarker = "# bd-shim v1"

func isBdShim(body string) bool {
	return strings.Contains(body, bdShimMarker)
}

// installHook writes one of our hooks into the repo at dir and returns its
// path. Refuses to overwrite a hook that is not ours; replaces ours in
// place (ADR 0002 §3: foreign hooks are never overwritten). chain, when
// true, additionally lets bd's own shim through: rather than refuse, it is
// moved aside and chained (see chainBdShim). A genuinely unknown hook is
// still refused whether or not chain is set.
func installHook(dir, slot, marker, legacy, script string, chain bool) (string, error) {
	// ADR 0052 D1: classify before touching. On a managed hooks path every
	// line below this is a write posse must not attempt — MkdirAll included —
	// and the typed verdict is what the caller reports instead of the create's
	// `permission denied`. m.Dir is git's dispatch path, so this replaces the
	// hooksDir call rather than adding a question.
	m, err := managedHooksDir(dir)
	if err != nil {
		return "", err
	}
	if m.Managed {
		return "", managedHooksError{m}
	}
	hooks := m.Dir
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(hooks, slot)
	if err := refuseNonRegularHook(p); err != nil {
		return "", err
	}
	if b, err := os.ReadFile(p); err == nil && !ownsHook(string(b), marker, legacy) {
		// A chain made from our printed prescription is deliberately foreign
		// at the slot: overwriting it would discard the other tool's hook. Its
		// posse-* member is ours, though, and must not become a frozen copy of
		// an older gate just because it lives behind the dispatcher. So behind
		// the exact dispatcher we prescribe, a marker-owned member is
		// refreshed and a MISSING one is restored; only a member that is there
		// and foreign stops us, and it stops us with its own words.
		chained := filepath.Join(hooks, "posse-"+slot)
		// Both branches below read that member, and one of them writes it.
		// Same rule as the slot itself: a special file there is foreign and
		// opening it can never return (ranger-base-92n5p).
		if err := refuseNonRegularHook(chained); err != nil {
			return "", err
		}
		if neighbour, isChain := chainDispatcherNeighbour(string(b), slot); isChain {
			owned, readErr := os.ReadFile(chained)
			switch {
			case readErr == nil && ownsHook(string(owned), marker, legacy):
				// The ordinary refresh.
			case os.IsNotExist(readErr):
				// RESTORE. The slot is our own dispatcher byte for byte, so
				// the file it runs first is ours to write and there is
				// nothing there to overwrite. This state is reached by
				// posse's own instructions: in a chained repo the marker line
				// saying "remove this file to uninstall" lives in
				// posse-<slot>, not in the slot, and removing it leaves every
				// push exiting 127. Re-running install-hooks is the operator's
				// natural repair, and it used to refuse and print the chain
				// prescription instead — whose first step, a bare `mv <slot>
				// theirs-<slot>`, destroyed the third party's hook and left a
				// dispatcher exec'ing into itself forever (ranger-base-q32o).
				// Re-chaining was never the repair here; restoring the gate is.
			case readErr != nil:
				return "", readErr
			default:
				return "", Die("%s is posse's chain dispatcher, but %s is not a posse hook — not overwriting.\nThe dispatcher runs that file first: restore posse's %s gate there, or delete the dispatcher and re-run install-hooks to build the chain afresh.", AbbrevHome(p), AbbrevHome(chained), slot)
			}
			if err := os.WriteFile(chained, []byte(script), 0o755); err != nil {
				return "", err
			}
			// Upgrade a chain written before rangerhq-xo65 while we are
			// here. Its unconditional `exec` kills every commit in the
			// repo the moment its neighbour is missing, and a repo
			// chained by an earlier posse would otherwise carry that
			// forever. Safe because the body matched our own render byte
			// for byte: we know exactly what it is, and we write back the
			// same shape, same neighbour, plus the guard.
			if string(b) == legacyChainHookDispatcherWith(slot, neighbour) {
				if err := os.WriteFile(p, []byte(chainHookDispatcherWith(slot, neighbour)), 0o755); err != nil {
					return "", err
				}
			}
			return chained, nil
		}
		// Everything below this point puts our gate at posse-<slot>: the
		// prescription's third step is a bare `mv <slot> posse-<slot>` and
		// chainBdShim writes that path outright. Neither name is free to
		// change the way theirs-<slot> is (ranger-base-q32o) — installHook's
		// own recognizer above reads exactly `posse-<slot>`, so a chain built
		// around any other name would not be recognized or refreshed. When
		// that file is there and is NOT ours, then, there is no arrangement
		// left that keeps it: `mv` destroys it without a word and so does the
		// WriteFile. Refuse, name it, and print no paste block — the same
		// answer the dispatcher-over-a-foreign-member case above already
		// gives (ranger-base-hd56). No posse instruction creates a foreign
		// posse-<slot>, so this is the operator's own file and moving it is
		// the operator's call.
		if owned, readErr := os.ReadFile(chained); readErr == nil && !ownsHook(string(owned), marker, legacy) {
			return "", Die("%s exists and is not a posse hook, and neither is %s — not overwriting.\nChaining this slot puts posse's %s gate at %s: move that file aside yourself, then re-run install-hooks for the chain prescription.", AbbrevHome(p), AbbrevHome(chained), slot, AbbrevHome(chained))
		}
		if chain && isBdShim(string(b)) {
			return chainBdShim(hooks, slot, script)
		}
		return "", Die("%s exists and is not a posse hook — not overwriting.\n%s", AbbrevHome(p), chainDispatcher(dir, hooks, slot))
	}
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		return "", err
	}
	return p, nil
}

// chainBdShim performs, in-process, the arrangement install-hooks otherwise
// only prints as a prescription to paste by hand (INSTALL.md §9): bd's shim
// is moved aside to bd-<slot>, ours is written to posse-<slot>, and the
// process-and-status dispatcher (never appended-to — rangerhq-kk6e) is
// written into the real slot, exec'ing into bd's shim last so it still gets
// the argv and stdin git handed the slot.
func chainBdShim(hooks, slot, script string) (string, error) {
	p := filepath.Join(hooks, slot)
	neighbor := "bd-" + slot
	bdPath := filepath.Join(hooks, neighbor)
	if _, err := os.Stat(bdPath); err == nil {
		// Already taken — by an earlier chain, or something else entirely.
		// Guessing which is wrong often enough that refusing is the honest
		// answer; the manual prescription (INSTALL.md §9) still applies.
		return "", Die("%s already exists — not overwriting; chain by hand (INSTALL.md §9)", AbbrevHome(bdPath))
	}
	if err := os.Rename(p, bdPath); err != nil {
		return "", err
	}
	chained := filepath.Join(hooks, "posse-"+slot)
	if err := os.WriteFile(chained, []byte(script), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, []byte(chainHookDispatcherWith(slot, neighbor)), 0o755); err != nil {
		return "", err
	}
	return chained, nil
}

// hookInstalled reports whether the repo at dir carries our hook in slot —
// either directly, or, since install-hooks --chain (and the manual
// prescription it mirrors) never puts our marker in the slot itself, behind
// a recognized chain dispatcher whose posse-<slot> member is ours.
func hookInstalled(dir, slot, marker, legacy string) bool {
	hooks, err := hooksDir(dir)
	if err != nil {
		return false
	}
	// Nothing but a regular file can be our hook, and asking by reading is
	// not free: open(2) on a FIFO with no writer never returns, so this
	// question used to hang the launcher rather than answer it
	// (ranger-base-92n5p). A special file at either path is "not installed",
	// which is the same answer an absent one gets.
	top := filepath.Join(hooks, slot)
	if !isRegularFile(top) {
		return false
	}
	b, err := os.ReadFile(top)
	if err != nil {
		return false
	}
	body := string(b)
	if ownsHook(body, marker, legacy) {
		return true
	}
	if isChainHookDispatcher(body, slot) {
		chained := filepath.Join(hooks, "posse-"+slot)
		if !isRegularFile(chained) {
			return false
		}
		owned, err := os.ReadFile(chained)
		return err == nil && ownsHook(string(owned), marker, legacy)
	}
	return false
}

// InstallPrePushHook writes the hook into the repo at dir (its common git
// dir, so worktrees share it). Returns the hook path. Refuses to overwrite
// a hook that is not ours; replaces ours in place.
func InstallPrePushHook(dir string) (string, error) {
	return installHook(dir, "pre-push", prePushMarker, legacyPrePushMarker, PrePushHook, false)
}

// InstallPrePushHookChained is InstallPrePushHook, but a slot occupied by
// bd's own shim is chained rather than refused (rangerhq-mgdk).
func InstallPrePushHookChained(dir string) (string, error) {
	return installHook(dir, "pre-push", prePushMarker, legacyPrePushMarker, PrePushHook, true)
}

// PrePushHookInstalled reports whether the repo at dir has a hook carrying
// our ownership marker. It is an install/replacement fact, not enforcement
// evidence: a foreign dispatcher can enforce the gate without the marker,
// and a marker-bearing file can be rewritten to exit 0.
func PrePushHookInstalled(dir string) bool {
	return hookInstalled(dir, "pre-push", prePushMarker, legacyPrePushMarker)
}

// deniesGitPush reports whether the PID's deny list carries a rule the
// pre-push hook would act on.
//
// It keys on ParseShimRules, so it reads only the two rule shapes that
// parser returns and is blind to the other push-granting spellings
// grantsGitPush below now covers (bare `Bash`, `Bash(*)`, `Bash(git *
// push)`, `Bash(git -C <repo> push)`). Deliberately left narrow: a deny
// rule this misses is a hook we do not install and a wall we therefore do
// not claim — blindness here fails SAFE, where the same blindness in the
// allow: alarm failed open, which is the whole asymmetry ranger-base-b2os
// was filed on. Widening it would change which repos get a pre-push hook
// and what parity claims realized; that is a separate change with a
// separate blast radius.
func deniesGitPush(deny []string) bool {
	for cmd, rules := range ParseShimRules(deny) {
		if cmd != "git" {
			continue
		}
		for _, r := range rules {
			if len(r.Words) == 0 || r.Words[0] == "push" {
				return true
			}
		}
	}
	return false
}

// grantsGitPush returns the PID's allow: rule (verbatim) that grants git
// push, or "" if none — ADR 0033 §2's coordinator's defining permission,
// and the whole trigger of §5's drift alarm.
//
// It used to key on ParseShimRules, the L1 shim's parser. That parser
// answers a narrower question than this one: it returns only rules of the
// shape Bash(<plain command name> …), because a rule it cannot render into
// a shim is not its business. Four spellings that DO grant push therefore
// came back "" and raised no warning (**MEASURED** on posse 0.3.0+53c8cb6,
// ranger-base-b2os): bare `Bash` — the broadest grant a PID can carry —
// `Bash(*)`, `Bash(git * push)`, the shape L0Spellings itself GENERATED
// until rangerhq-vr6j and a shape a PID can still be hand-written in, and
// `Bash(git -C /repo push)`. A hand-edit granting a persona
// bare `Bash` landed in exactly the state §5 exists to make visible, and
// the alarm stayed quiet.
//
// So the question is asked directly instead: can this rule permit a `git
// push` invocation, by any of the three forms claude matches a Bash rule
// (exact, `:*` prefix, `*` wildcard — see L0Spellings)? The alarm is
// advisory, so it over-approximates on purpose: an unrecognized grant is
// silence, and silence is the defect being fixed.
func grantsGitPush(allow []string) string {
	for _, rule := range allow {
		if grantsGitPushRule(rule) {
			return rule
		}
	}
	return ""
}

// grantsGitPushRule is grantsGitPush for one rule.
func grantsGitPushRule(rule string) bool {
	if rule == "Bash" {
		return true // every Bash command, push with them
	}
	if !strings.HasPrefix(rule, "Bash(") || !strings.HasSuffix(rule, ")") {
		return false // Edit, Write, WebFetch, mcp__* — other layers'
	}
	body := strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), ")")
	prefix := strings.HasSuffix(body, ":*")
	words := strings.Fields(strings.TrimSuffix(body, ":*"))
	if len(words) == 0 {
		return false // Bash() grants nothing
	}
	// The LAST word of a `:*` rule is a string prefix, not a whole word:
	// `Bash(git pus:*)` matches `git push` and `Bash(gi:*)` matches all of
	// git. Anywhere else a word is a word.
	if !reachesWord(words[0], "git", prefix && len(words) == 1) {
		return false
	}
	if strings.Contains(words[0], "*") {
		// The wildcard sits in front of the subcommand, so it can absorb
		// it: `Bash(* log)` is `^.* log$`, which matches `git push origin
		// log`. Whatever is written behind it cannot make that not a push.
		return true
	}
	rest := words[1:]
	if len(rest) == 0 {
		return true // the whole verb: Bash(git), Bash(git:*), Bash(*)
	}
	// Walk to the word standing where the SUBCOMMAND stands, consuming
	// git's global options on the way — and consuming in PAIRS the ones
	// that take a separate value, or `Bash(git -C /repo push)` reads
	// `/repo` as the subcommand and the rule looks like `git /repo`. Same
	// table, and the same reason, as the L1 shim's own walk.
	for i := 0; i < len(rest); i++ {
		w := rest[i]
		if strings.HasPrefix(w, "-") {
			if strings.Contains(w, "*") {
				return true // again in front of the subcommand slot
			}
			for _, o := range globalValueOpts["git"] {
				if w == o {
					i++ // its value is the next word, not the subcommand
					break
				}
			}
			continue
		}
		// The subcommand slot. Only one subcommand is a push — which is
		// what keeps `Bash(git stash push:*)` and `Bash(git log
		// --grep=push)` quiet — but a token carrying a wildcard reaches
		// push whenever its literal head does, and so does a trailing
		// partial word.
		return reachesWord(w, "push", prefix && i == len(rest)-1)
	}
	// Every word was a global option. A prefix rule leaves the subcommand
	// open (`git -C x push` starts with `git -C x`); an exact one matches
	// that command line and nothing longer.
	return prefix
}

// reachesWord reports whether a claude Bash-rule token can stand where the
// command word want stands. `*` is `.*` over the WHOLE command string
// (L0Spellings documents the dialect), so a token carrying one is bounded
// only by the literal text in front of the wildcard: `p*s` matches `push
// origin refs`, `l*g` reaches no push at all. partial is the last word of a
// `:*` rule, which is a string prefix rather than a whole word.
func reachesWord(tok, want string, partial bool) bool {
	if i := strings.Index(tok, "*"); i >= 0 {
		return strings.HasPrefix(want, tok[:i])
	}
	if partial {
		return strings.HasPrefix(want, tok)
	}
	return tok == want
}

// ─── L3: the commit guard (prepare-commit-msg) ───────────────────────────────

// sharedIndexMarker identifies our prepare-commit-msg hook. The slot now
// carries five walls — the data ceiling, the beads visibility guard, the
// constitution-path guard, the ADR sha-stamp guard and the shared-index
// guard — and the marker
// still says `shared-index` ON PURPOSE: ownership is a question about a file
// an EARLIER binary wrote, and a repo hooked before
// this bead carries this exact string. Renaming the value would convert
// every already-hooked repo into a repo we refuse to touch, which is the
// rangerhq-tyay lesson learned twice. The name of the const may move; the
// string may not.
const sharedIndexMarker = "# posse-gate shared-index"

// sharedIndexBody is the second half of the prepare-commit-msg hook (the
// beads visibility guard above it is the first): the L3 wall for the
// failure rangerhq-nyqj measured: every persona the loop dispatches gets the SAME checkout, so
// the repo is one working tree and one .git/index shared by the crew, and
// an unqualified commit takes whatever anyone else has staged. It has
// happened: a routine `bd sync:` commit that carried eight files another
// persona had staged for a different bead.
//
// The discriminator is GIT_INDEX_FILE, and the obvious test is wrong
// (measured against all four forms, twice, independently):
//
//	git add … && git commit    .git/index                  sweeps
//	git commit -- <paths>      .git/next-index-<pid>.lock   safe
//	git commit -a              .git/index.lock              SWEEPS, worst
//	git commit --  (no paths)  .git/index                  sweeps
//
// So the name has to be `next-index-*` specifically: "is it a temporary
// index" waves `-a` through, and `-a` takes every persona's modified
// tracked file. The empty pathspec is why the L1 grammar's `unless`
// requires an operand too.
//
// AND THE NAME IS NOT ENOUGH — rangerhq-cqq1. GIT_INDEX_FILE is the
// caller's environment variable, so a glob on its name is a wall one
// spelling wide: `GIT_INDEX_FILE=$(mktemp -d)/next-index-mine` was refused
// as `…/index` and waved through as `…/next-index-mine`, same recipe,
// landing the commit and leaving the shared index stale — rangerhq-8rtf end
// to end, under a persona. The exemption now asks what git actually does,
// measured (git 2.39.3, main repo and linked worktree): the temp index is
// an absolute path, it lives in `git rev-parse --git-dir` (the PER-WORKTREE
// dir, not the common one — verified in a linked worktree), and it is named
// for git's own pid. So: basename `next-index-<digits>[.lock]`, and its
// directory, resolved with `pwd -P`, equal to the resolved git dir.
//
// NOT the pid itself, tempting as it is — measured, the hook's $PPID is
// git's pid and matches the filename exactly, which would close the class
// outright. It cannot be used: under the hook chain INSTALL.md documents
// (a dispatcher that runs `"$d/posse-prepare-commit-msg" "$@"`) the gate is
// the dispatcher's child, so its $PPID is the wrapper, not git — verified.
// A pid check would refuse the crew's only safe route in every chained
// install. Location + name shape is the tight end of what is safe here.
//
// The residual, stated plainly: `GIT_INDEX_FILE=$GIT_DIR/next-index-1` is
// still exempt, and still leaves the shared index stale. That is a private
// index deliberately placed inside the repo's own git dir, not a temp file
// that happens to be spelled right — one is a decision, the other was an
// accident waiting on a glob.
//
// THE SLOT IS prepare-commit-msg, NOT pre-commit, for two measured reasons.
// pre-commit is bd's flush hook — worktree-aware, reinstalled by bd — and
// ADR 0002 §3 says a foreign hook is never overwritten; a wall a third-party
// tool silently replaces on its next install is not a wall. And
// `git commit --no-verify` skips pre-commit while prepare-commit-msg still
// runs, so this slot is the stronger of the two. Both verified.
//
// NOT KEYED ON RHQ_PERSONA — the wall applies to every shell in the shared
// checkout, the operator's own included (OPERATOR RULING 2026-08-28,
// rangerhq-lt2w). It was keyed on it until then, the way the pre-push gate
// keys on RHQ_TOOLS_DENY, so that "the operator's own commits in the same
// tree are untouched" (rangerhq-lmq9). What retired that: the exemption's
// second half. An unqualified commit does not only SWEEP, it also REVERTS —
// it restages every path from a shared index that a private-index commit
// left holding pre-fix blobs, which is how ef8d35f was undone for 3h52m by
// dcca7b5, a hand-typed `bd sync:` commit (rangerhq-8rtf). A private index is
// not the only producer of that stale entry: so is the form THIS WALL
// PRESCRIBES, whenever the pre-commit hook stages a path the pathspec does not
// name (rangerhq-be7k — bd's flush, measured, internal/posse/staleindex_qa_test.go).
// git refreshes the real index for the pathspec only, and writes it before it
// calls the hook. That producer is not refusable here — the wall's slot runs
// before the commit, and the form is the one we want — so the wall's job
// against it is unchanged and is the whole of it: refuse the two carriers that
// spring the stale entry, which are the unqualified form and `-i`. Neither
// `-a` nor any pathspec commit carries it (measured, same pin). `bd sync` itself
// does not commit ("Does NOT stage or commit - that's the user's job",
// bd 0.49.1), so the reverting half was never bd's to fix: it is the
// unqualified form, and the operator's `bd sync:` commits are exactly that
// form. A wall that stops the crew from making the stale index but lets the
// operator spring it is half a wall.
//
// WHAT THIS COSTS, stated rather than discovered: every hand commit in a
// hooked, non-worktree checkout now needs a pathspec, and there is no
// override env for this arm (the visibility guard above has one; this one
// does not — adding it would be re-spelling the carve-out, which is the
// decision that was just taken away). The way through is always there: the
// path-limited form in the ordinary case, and the three states where git
// refuses a pathspec (merge, cherry-pick, rebase) keep their exemptions
// below on their own merits. A linked worktree still stands down entirely.
//
// THE EXEMPTION IS "GIT REFUSES A PATHSPEC HERE", NOT "AN OPERATION IS IN
// PROGRESS" — the two are not the same set, and the difference was a hole
// (ranger-base-08a2). Measured on git 2.39.3, macOS 26.4.1, each state
// probed for both what git accepts and what the hook sees:
//
//	state                     pathspec?   git's own completion   verdict
//	MERGE_HEAD (conflict)     fatal       $2=merge or message    exempt
//	MERGE_HEAD (--no-commit)  fatal       $2=merge or message    exempt
//	CHERRY_PICK_HEAD          fatal       $2=message             exempt
//	rebase-merge              ACCEPTED    $2=message, no marker  exempt, residual
//	REVERT_HEAD (conflict)    ACCEPTED    $2=merge               REFUSED
//	REVERT_HEAD (--no-commit) ACCEPTED    n/a                    REFUSED
//	SQUASH_MSG ($2=squash)    ACCEPTED    n/a                    REFUSED
//
// So merge and cherry-pick keep their exemption on its stated merits: there
// is no safe form to name, and both carry git's completion even when the
// persona types the message ($2=message mid-merge), which is why the MARKER
// and not `case "$2"` is what holds them.
//
// REVERT_HEAD and the `case "$2" in merge|squash)` arm are gone. Both waved
// an UNQUALIFIED commit through a state where a pathspec works — measured,
// `git revert --no-commit <sha>` then `git add <another persona's paths> &&
// git commit -m x` landed them, which is rangerhq-nyqj exactly, inside the
// window rangerhq-lrnp's own blessed recipe opens. `git merge --squash` is
// the same shape one arm over. Neither strands anything: `git revert
// --continue` is refused now, and the path-limited commit finishes a
// conflicted revert outright — verified end to end, REVERT_HEAD, MERGE_MSG
// and AUTO_MERGE all cleared, tree clean, a following `--continue` correctly
// reporting "no cherry-pick or revert in progress". A rebase is the one
// exemption that is wider than it wants to be, and the hook says so where it
// stands. `git commit --amend` is NOT one of them either — it takes a
// pathspec and sweeps without one, so it is refused.
//
// A CLEAN `git revert` IS REFUSED, and that is the verdict rather than an
// oversight (rangerhq-lrnp). Measured on git 2.39.3: it writes no
// REVERT_HEAD, no sequencer and no GIT_REFLOG_ACTION before the hook runs —
// $2 is "message" and GIT_INDEX_FILE is .git/index, i.e. at this slot it is
// indistinguishable from `git commit -m`. The two signals that DO exist are
// both unusable as an exemption, and each would take the wall down silently
// rather than narrow it (measured, this is the rangerhq-cqq1 lesson again):
// AUTO_MERGE outlives the revert that wrote it and is still there for the
// next plain commit, and MERGE_MSG outlives a revert this hook refuses. Nor
// is REVERT_HEAD an answer for the states that DO write one: see the table
// above — a pathspec works there, so an exemption buys nothing and costs
// the wall.
//
// So the way through is NAMED IN THE REFUSAL instead, and it is two steps
// (verified end to end under the gate): `git revert --no-commit <sha>`, then
// `git commit -F - -- <the paths it touched>`. That second commit needs no
// exemption: a path-limited commit gets its own next-index temp file even
// mid-revert, so it passes on its own merits.
//
// And because git stages the revert BEFORE this hook can refuse it, the
// refusal names what is sitting in the shared index and how to undo it
// path-limited. That dirt is bounded: `git revert` only starts from an index
// that matches HEAD ("your local changes would be overwritten by revert"),
// so what is staged at refusal time is the revert and nothing of anyone
// else's — which is also why the hook must not "clean up" itself. A hook
// that ran `git reset` behind the persona would be the destructive act the
// wall exists to prevent.
//
// THE NOTES.md ARM (ADR 0022 §3, ranger-base-hokh) sits inside this same
// body, ahead of the next-index-<pid> exemption below: everything above it
// asks whether the commit's FORM is safe (a genuine path-limited commit
// exempted, a sweep refused), and a path-limited commit on NOTES.md is the
// one form that has to be refused anyway — that file has no single writer
// in this tree (ranger-base-yuwy, 808da1b), so the very form the rest of
// this wall treats as safe is the failure here. It reads the to-be-committed
// set with `git diff --cached --name-only --no-renames HEAD` under the hook's
// inherited GIT_INDEX_FILE, which is right for a path-limited commit's own
// next-index the same way it is for the constitution arm. --no-renames is
// load-bearing (ranger-base-x9xbk): with rename detection on — the default —
// a detected move prints only its DESTINATION, so `git mv NOTES.md
// ARCHIVE.md` walked straight through. The constitution arm had the same
// blind spot on its own class and no longer has it (ranger-base-qdxe), so
// every reader of the staged set in this file now spells the diff the same
// way — that is a property with a pin on it, not a coincidence
// (TestQAHookReadersAllDisableMoveDetection). Unkeyed like the rest of this
// wall, for the same reason
// (rangerhq-lt2w): the tree does not care who typed the commit. It runs
// AFTER MERGE_HEAD/CHERRY_PICK_HEAD/rebase exit 0 above, so those three
// exemptions stand exactly as they did — a NOTES.md change reached by one of
// them is exempt, deliberately: this bead's Verification list does not ask
// for more, and none of the three has a pathspec-based way through to prefer
// instead.
const sharedIndexBody = `
# ─── the shared-index guard (rangerhq-lmq9) ───────────────────────────────
# No RHQ_PERSONA test: this wall covers every shell in the shared checkout,
# the operator's own included (rangerhq-lt2w). The tree is what makes the
# commit unsafe, and the tree does not care who typed it.
posse_gitdir=$(git rev-parse --git-dir 2>/dev/null) || exit 0
# ranger-base-58to: the two prescribed lines below are commands, meant to be
# pasted into a shell — so the paths in them have to survive that paste.
# Each path is wrapped in single quotes with any embedded quote escaped the
# POSIX way ('\'') — round-trips through a shell byte-for-byte, and a command
# substitution planted in a filename stays inert.
#
# THE PATHS COME OUT OF git -z (ranger-base-qg0k8), not out of --name-only
# with core.quotePath=false. core.quotePath ONLY governs non-ASCII bytes:
# git C-quotes a path holding a double quote, a backslash or a control byte
# whatever quotePath says, so the earlier reader wrapped git's ALREADY-QUOTED
# spelling in single quotes and shipped the literal C-escape into the
# pathspec. Measured on git 2.50.1, for files named q"uote.md, back\slash.md
# and tab<TAB>here.md, the prescribed lines died with three "did not match any
# file(s) known to git" — and because git validates pathspecs all-or-nothing,
# the correctly-spelled paths on the same line were not committed or restored
# either. --name-only -z emits the raw path bytes with no quoting at all, for
# every byte class, and needs no quotePath override at all.
#
# --no-renames (ranger-base-pp7k1) for the same reason the NOTES.md arm
# below passes it (ranger-base-x9xbk): rename detection is ON by default, and
# for a detected rename --name-only prints only ONE side of the pair, so both
# prescribed lines get built from half the staged set. Measured on git 2.50.1
# over a 200-line file moved with 'git mv' and reverted, the reader printed
# old.md alone and the undo it prescribed exited 0 — no error at all — leaving
# a staged deletion of new.md in the SHARED index. That is worse than the
# quoting defect above: that one exited 1 and said so, this one reports
# success and leaves the persona believing the tree is clean, three lines
# above the sentence telling them not to reach for a hard reset. The finish
# line had the matching hole, committing one side of a rename.
#
# UNLIKE ranger-base-x9xbk, the fixture size is NOT what makes this visible
# (measured, git 2.50.1): git's 50% similarity threshold gates a move that
# also EDITS the file, and a revert of a 'git mv' is byte-identical on both
# sides, so git's exact-rename pass pairs it at ANY size — a one-line file
# collapses just as an 800-line one does. What the pin asserts instead is
# that git actually reported the rename (R100) before it measures the lines.
#
# tr '\0' '\n' turns the NUL-delimited list back into lines because POSIX sh
# has no NUL-delimited read (-d is a bashism) and command substitution eats
# NULs anyway. That is not a step backwards: newline-delimited is what the
# code here has always been, and a path holding a literal newline is already
# outside what a pasted one-line command can round-trip. tr is coreutils/base
# on every box, the same tier as the sed below — not the diffutils tier that
# made 'cmp' vanish silently (ranger-base-rmgz).
posse_qcached() {
  git diff --cached --name-only --no-renames -z HEAD 2>/dev/null | tr '\0' '\n' | while IFS= read -r posse_p; do
    printf "'%s' " "$(printf '%s' "$posse_p" | sed "s/'/'\\\\''/g")"
  done
}
# THE EXEMPTION ASKS ONE QUESTION: is there a safe form to point at? "An
# operation is in progress" is not that question (ranger-base-08a2). git
# refuses a pathspec outright in exactly two states — MERGE_HEAD and
# CHERRY_PICK_HEAD ("cannot do a partial commit during a merge" / "during a
# cherry-pick"), measured in both the conflicted and the --no-commit form —
# so refusing there would leave no way through rather than a safer one.
# These are also the two that carry git's own completion when the persona
# types the message: 'git commit -m mine' mid-merge arrives as $2=message,
# so the marker, not "$2", is what has to hold it.
for posse_f in MERGE_HEAD CHERRY_PICK_HEAD; do
  if [ -e "$posse_gitdir/$posse_f" ]; then exit 0; fi
done
# A rebase is the third exemption and the only one WIDER than it wants to be.
# A pathspec IS accepted mid-rebase (measured), but a rebase has commits left
# to replay, so 'git rebase --continue' is the only way on and it reaches
# this slot as $2=message with $GIT_DIR/index — indistinguishable from a
# typed 'git commit'. GIT_REFLOG_ACTION=rebase (continue) does discriminate
# and is unusable for the same reason GIT_INDEX_FILE's name was: it is the
# caller's to spell (rangerhq-cqq1). So the residual is stated rather than
# closed: during a rebase, an unqualified commit is exempt. It is bounded
# by the crew PIDs, which forbid rewriting history in the shared checkout at
# all — a rebase there is already out of bounds — and, for the operator, by
# a rebase being a deliberate act rather than a routine one.
for posse_f in rebase-merge rebase-apply; do
  if [ -e "$posse_gitdir/$posse_f" ]; then exit 0; fi
done
# A LINKED WORKTREE HAS NO SHARED INDEX (rangerhq-09o2, measured on this
# hook). git keeps a per-worktree index in the per-worktree git dir, so
# in a session worktree there is nothing for an unqualified commit to sweep —
# the wall would refuse a form that is safe, under a message ("shared by every
# persona") that is no longer true of that tree. The discriminator is git's
# own: --git-dir is the per-worktree dir and --git-common-dir the shared one,
# and they differ only in a linked worktree. Resolved with pwd -P because one
# is relative in the main repo and both are absolute in a worktree.
posse_common=$(git rev-parse --git-common-dir 2>/dev/null) || exit 0
posse_gd=$(CDPATH= cd -P -- "$posse_gitdir" 2>/dev/null && pwd -P)
posse_cd=$(CDPATH= cd -P -- "$posse_common" 2>/dev/null && pwd -P)
if [ -n "$posse_gd" ] && [ -n "$posse_cd" ] && [ "$posse_gd" != "$posse_cd" ]; then
  exit 0
fi
# ─── the NOTES.md guard (ADR 0022 §3, ranger-base-hokh) ───────────────────
# NOTES.md has no single writer in this tree, so a genuine path-limited
# commit on it — the very form the next-index exemption below waves through
# — is exactly the failure ADR 0022 closes: two personas' same-afternoon
# edits, first commit takes both, silently, under the wrong bead id
# (ranger-base-yuwy, 808da1b). This arm runs BEFORE that exemption on
# purpose: the exemption asks whether the commit's FORM is safe, and this
# file needs refusing regardless of form. Unkeyed, like the rest of this
# wall since the operator ruling on rangerhq-lt2w (ranger-base-5imp): the
# tree is what makes the commit unsafe, not who typed it, and a per-arm
# RHQ_PERSONA test would re-spell the carve-out the ruling retired.
# --no-renames (ranger-base-x9xbk): rename detection is ON by default (git
# 2.9+ treats an unset diff.renames as true), and for a detected move
# --name-only prints ONLY the destination path. Without the flag a staged
# 'git mv NOTES.md ARCHIVE.md' never puts the string NOTES.md in front of the
# arm below and the commit lands — carrying away whatever another persona had
# uncommitted in that file, under the mover's message and bead id, which is
# the ADR 0022 sweep itself and not a spelling nit. A small fixture hides it:
# git only pairs the removal with the add at 50% similarity or better, so it
# is a realistic NOTES.md that makes it visible. With the flag git reports the
# removal and the add separately and the exact-string arm sees NOTES.md again.
# The land belt already reads its own diff this way (worktree.go).
posse_notes_staged=$(git diff --cached --name-only --no-renames HEAD 2>/dev/null)
posse_notes_hit=0
if [ -n "$posse_notes_staged" ]; then
  posse_notes_ifs=$IFS
  IFS='
'
  for posse_notes_p in $posse_notes_staged; do
    if [ "$posse_notes_p" = "NOTES.md" ]; then posse_notes_hit=1; fi
  done
  IFS=$posse_notes_ifs
fi
if [ "$posse_notes_hit" = 1 ]; then
  {
    echo "refused by posse gate: a commit changing NOTES.md in the shared checkout — prepare-commit-msg hook, session ${RHQ_PERSONA:-operator}"
    echo "ADR 0022: this tree has no single writer for NOTES.md, so a"
    echo "path-limited commit here is the sweep, not the isolation — it takes"
    echo "the file as it is ON DISK, another persona's half-written edit"
    echo "included, under your message and your bead id (ranger-base-yuwy)."
    echo "  write it instead: docs/notes.d/<bead-id>.md — sole writer by"
    echo "  construction, commit it path-limited, fold it into NOTES.md later"
    echo "  as ordinary work in a worktree."
    echo "  or edit NOTES.md from a session worktree, where same-file"
    echo "  divergence surfaces at land time as a reported conflict, never a"
    echo "  silent sweep."
  } >&2
  if [ -n "$RHQ_GATES_DIR" ]; then
    echo "$(posse_stamp) NOTES.md commit in the shared checkout [prepare-commit-msg hook]" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
  fi
  exit 1
fi
# Only a genuine path-limited commit gets a next-index-<pid> temporary index;
# 'git commit -a' gets .git/index.lock, which is a temporary index too, and so
# does 'git commit -i -- <paths>' (rangerhq-ojnw) — one arm, two forms.
# The NAME does not settle it: GIT_INDEX_FILE is the caller's to spell, and
# <tmpdir>/next-index-mine walked straight through the earlier glob
# (rangerhq-cqq1). git makes its own inside $GIT_DIR and names it after its
# own pid, so require the LOCATION and the pid shape as well — an exemption
# that is a fact about git rather than about a string.
posse_idx="${GIT_INDEX_FILE:-}"
posse_base="${posse_idx##*/}"
posse_dir="${posse_idx%/*}"
if [ "$posse_dir" = "$posse_idx" ]; then posse_dir="."; fi
posse_idxdir=$(CDPATH= cd -P -- "$posse_dir" 2>/dev/null && pwd -P)
posse_realdir=$(CDPATH= cd -P -- "$posse_gitdir" 2>/dev/null && pwd -P)
posse_pid="${posse_base#next-index-}"
posse_pid="${posse_pid%.lock}"
case "$posse_base" in
  next-index-*)
    case "$posse_pid" in
      ''|*[!0-9]*) ;;
      *)
        if [ -n "$posse_idxdir" ] && [ -n "$posse_realdir" ] &&
           [ "$posse_idxdir" = "$posse_realdir" ]; then exit 0; fi
        ;;
    esac
    ;;
esac
# Name the form. git's own index and its two lock files live in $GIT_DIR;
# anything else GIT_INDEX_FILE points at is a hand-rolled private index,
# and saying so beats calling it "an unqualified git commit".
posse_form="an unqualified git commit"
if [ -n "$posse_idx" ] && [ -n "$posse_realdir" ]; then
  if [ "$posse_idxdir" = "$posse_realdir" ]; then
    case "$posse_base" in
      index) ;;
      index.lock) posse_form="git commit -a or -i" ;;
      *) posse_form="a commit from a private GIT_INDEX_FILE" ;;
    esac
  else
    posse_form="a commit from a private GIT_INDEX_FILE"
  fi
fi
{
  echo "refused by posse gate: $posse_form — prepare-commit-msg hook, session ${RHQ_PERSONA:-operator}"
  echo "This working tree's .git/index is shared by every persona (rangerhq-nyqj):"
  echo "an unqualified commit takes whatever anyone else has staged, -a takes"
  echo "every persona's modified tracked file, and -i takes the shared index ON"
  echo "TOP of the paths you named."
  echo "  safe form: git commit -F - -- <paths>"
  echo "  name your own paths, not '.' — a pathspec of '.' sweeps the tree too."
  echo "  a named path commits the file as it is ON DISK, not what you staged"
  echo "  (rangerhq-lvu9): if another persona is editing it, you commit their"
  echo "  half-written lines too, under your message. 'git diff HEAD -- <paths>'"
  echo "  first: it shows what the commit will take, staged edits included — the"
  echo "  bare two-dot 'git diff' compares the tree to the INDEX, so it is empty"
  echo "  over an edit someone has staged and sees none of it. Even clean, this"
  echo "  form bounds the PATHS, not the CONTENT; only a private worktree closes"
  echo "  the rest (rangerhq-09o2)."
  echo "  a NEW file cannot be committed by that form alone (rangerhq-4pbt): a"
  echo "  pathspec only matches a file git already has an index entry for, so an"
  echo "  untracked path answers \"did not match any file(s) known to git\" and"
  echo "  never reaches this hook. Two steps, and the add is scoped:"
  echo "    git add -- <the new paths>"
  echo "    git commit -F - -- <all your paths>"
  echo "  never a bare 'git add -A' or 'git add .' — those stage every persona's"
  echo "  new and modified file into this shared index, which is what the rule"
  echo "  above exists to keep you out of. The window between the add and the"
  echo "  commit is the residual: your staged entry is in the shared index until"
  echo "  the commit takes it, so keep the two adjacent and name the new paths in"
  echo "  the commit too."
  if [ "$posse_form" = "a commit from a private GIT_INDEX_FILE" ]; then
    echo "A private index also leaves the shared .git/index holding the PRE-FIX blobs"
    echo "for every path you just committed, so the next unqualified commit reverts"
    echo "you silently (rangerhq-8rtf). Naming it next-index-* does not change that."
  fi
  # rangerhq-lrnp: a clean 'git revert' reaches this hook with no marker to
  # exempt it AND with its change already staged in the shared index, so this
  # refusal is landing on a tree git has changed. Say so, and name both ways
  # out. The test is git's own message file: git primes it from MERGE_MSG for
  # the commits it drives, and a stale MERGE_MSG does not match a message you
  # typed (measured both ways). It only WORDS the refusal — a false positive
  # costs a confusing sentence, never an opening in the wall, which is why
  # MERGE_MSG is trusted here and not above.
  # ranger-base-rmgz: 'cmp' is diffutils, missing by default on Fedora,
  # AlmaLinux/RHEL and Arch (measured, three of four clean-room distros) — its
  # absence made this whole paragraph vanish silently. Command substitution
  # compares the same way cmp -s would: both sides have their trailing
  # newlines stripped equally, so the comparison stays honest.
  if [ -f "$posse_gitdir/MERGE_MSG" ] && [ "$(cat "$1")" = "$(cat "$posse_gitdir/MERGE_MSG")" ]; then
    posse_staged=$(posse_qcached)
    echo "git prepared this commit itself (revert): it staged the change into the"
    echo "shared index BEFORE this hook could refuse, so the change is sitting there"
    echo "now. It starts bounded — git revert only begins from an index matching"
    echo "HEAD — so these paths are the revert's, plus anything you staged after it:"
    # ranger-base-23mvz: printf, not echo. The reader emits the raw path
    # bytes (ranger-base-qg0k8) and echo would undo that work on the way out:
    # macOS /bin/sh is bash 3.2 with xpg_echo on when invoked as sh, and a
    # Linux /bin/sh is usually dash, both of which EXPAND backslash escapes
    # in echo's operand. A path holding \n, \t, \r, \\ or \c reached the
    # persona mangled, or (\n, \c) broke the printed line in two, and git
    # validates pathspecs all-or-nothing so the correctly-spelled paths on
    # the same line died with it. Same reason the constitution arm prints
    # $posse_cls_hit with printf.
    printf '%s\n' "  finish it:  git commit -F - -- $posse_staged"
    printf '%s\n' "  or undo it: git restore --source=HEAD --staged --worktree -- $posse_staged"
    echo "  next time:  git revert --no-commit <sha>, then the path-limited commit."
    echo "Never 'git reset --hard' here: this tree is shared, and it is not yours."
  elif [ -e "$posse_gitdir/REVERT_HEAD" ]; then
    # ranger-base-08a2: a revert the persona is finishing with a message of
    # their own. REVERT_HEAD is no longer an exemption — a pathspec IS
    # accepted mid-revert — so it is free to word the refusal, which is all
    # it was ever safe for.
    posse_staged=$(posse_qcached)
    echo "A revert is in progress (REVERT_HEAD) and its change is already staged in"
    echo "the shared index — along with anything else that is staged there, which is"
    echo "why this form is refused and not exempted: a pathspec works here."
    # printf for the same reason as the arm above (ranger-base-23mvz).
    printf '%s\n' "  staged now: $posse_staged"
    echo "  finish it:  git commit -F - -- <the paths that are yours>"
    echo "That commit ends the revert on its own — no 'git revert --continue' after"
    echo "it, and REVERT_HEAD, MERGE_MSG and AUTO_MERGE go with it (measured)."
  fi
} >&2
if [ -n "$RHQ_GATES_DIR" ]; then
  echo "$(posse_stamp) $posse_form [prepare-commit-msg hook] (shared index)" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
fi
exit 1
`

// commitGuardHead is the hook's shebang and its marker line — the two lines
// that decide ownership, kept away from either wall's body.
const commitGuardHead = `#!/bin/sh
` + sharedIndexMarker + ` — installed by posse gates install-hooks. Five walls
# in one slot: the data ceiling (ADR 0050 — this instance's config
# ` + DataCeilingConfigKey + `: over every staged file, every added path
# and the commit message, under EVERY visibility stamp), the beads
# visibility guard (rangerhq-hrz, extended by ADR 0024 D2 checks 1+2+3 to a
# docs-genre allowlist, an OpsPatterns scan over staged markdown, and a scan
# for this box's own identity literals and this instance's config patterns
# over every staged file, every added path and the commit message), the
# constitution-path guard (ranger-base-ak3e), the ADR sha-stamp guard
# (ADR 0051 D4/D5, ranger-base-glewr) and the shared-index commit guard
# (rangerhq-lmq9).
# Foreign hooks are never overwritten; remove this file to uninstall.
# ADR 0002 §3.
`

// publicDocsGenrePattern is PublicDocsGenres rendered as a shell case
// pattern — a `|`-joined alternation. Every genre name matches
// opsClassRE-adjacent boring characters (letters, digits, a dot), so none of
// them needs shell quoting to stay a literal alternative rather than a glob.
func publicDocsGenrePattern() string {
	return strings.Join(PublicDocsGenres, "|")
}

// markdownPathspecArgs is MarkdownPathspecs rendered as the pathspec
// operands of check 2's `git diff` — each single-quoted, because `:(icase)`
// carries a `(` and a `*` that a shell would otherwise take for its own
// (ranger-base-4b1z4). One Go list, one rendered arm.
func markdownPathspecArgs() string {
	out := make([]string, 0, len(MarkdownPathspecs))
	for _, p := range MarkdownPathspecs {
		out = append(out, shQuote(p))
	}
	return strings.Join(out, " ")
}

// opsClassOnlyArg is posse_check's third argument, the disclosure switch
// (ADR 0048 D2, ranger-base-8114t). It is a word rather than a flag because
// the rendered hook is read by people: `posse_check 'client-acme' '<ere>'
// class-only` says in the file itself what the refusal will and will not
// print. Only a CONFIGURED pattern gets it: an instance visibility pattern,
// and — always, for the reason ADR 0050 gives — a data-ceiling pattern.
const opsClassOnlyArg = "class-only"

// opsCheckCall renders one posse_check call. classOnly is true for a pattern
// that came from config beads_visibility_patterns:, whose value is one
// deployment's confidential vocabulary, and for every pattern from config
// data_ceiling_patterns:, whose matched text may not exist in a local file
// at all — a refusal is a local file (ADR 0050 Context). The refusal then
// names the class and a hit count and never the ERE or the text it matched.
// A shipped OpsPattern and a derived identity literal keep ADR 0024 D2's
// shape, where the matched string is what makes the refusal actionable.
func opsCheckCall(indent, class, ere string, classOnly bool) string {
	line := indent + "posse_check " + shQuote(class) + " " + shQuote(ere)
	if classOnly {
		line += " " + opsClassOnlyArg
	}
	return line + "\n"
}

// visibilityGuardBody renders the first wall against a repo whose beads db
// carries the given visibility. THE VERDICT IS STAMPED AT INSTALL TIME
// rather than read at commit time, and that is a deliberate trade: the hook
// is POSIX sh and the config is flat-YAML, so a commit-time read would mean
// a second parser in a second language — the one thing NOTES.md says this
// repo stopped doing. It is stamped fresh by `posse gates install-hooks`
// AND by every persona launch into the repo (herdrback.go), so a mark the
// operator changes is live on the next dispatch; a repo nobody launches
// into keeps the mark it was hooked with, and the hook says which one it is.
// That holds for a repo chained per INSTALL.md §9 too, where the slot is a
// foreign dispatcher and our hook lives behind it as posse-<slot>: both
// installers go through installHook, which refreshes that marker-owned
// member in place (ranger-base-i5f4, ranger-base-r5ba), stamp included —
// pinned in TestInstallCommitGuardRestampsAChainedHookWhenTheMarkChanges
// (rangerhq-qm6c). What it does NOT yet hold for is a call from a linked
// worktree: the hook lands in the shared repo but the mark is looked up
// under the worktree's own path, which no config key names (ranger-base-up22).
//
// The block always renders, gated on the stamp, so the hook FILE is the
// record of what it was stamped with — a human reads it and knows.
//
// ADR 0024 D2 adds three more checks to this same arm, same gate
// (posse_beads_visibility = public), same reason for the slot: check 1 is
// the docs-genre allowlist (a staged NEW file under docs/ must sit in an
// allowlisted subdirectory, move detection off so a move into an unlisted
// genre is a new file like any other); check 2 is OpsPatterns over the ADDED
// lines of every staged markdown file — MarkdownPathspecs, .md and
// .markdown, case-insensitive — any path, the same list and the same posse_check
// function the beads-jsonl check above uses, just pointed at a different
// staged set, which is what "same list, both readers" (visibility.go) means
// here: one Go slice, one shell function, two call sites. Check 3 is the
// identity literals this box's own render derived (DeriveIdentityLiterals)
// — rendered as escaped EREs (identityLiteralERE) so the SAME posse_check
// function serves all three, over the ADDED lines of every staged file,
// code included, AND over the ADDED staged paths (ranger-base-dmsbu:
// a filename is where an operator-shaped artifact puts the operator, and a
// pure move has no added lines at all) — and, over those staged paths
// ALONE, this box's crew names (DeriveCrewLiterals, ranger-base-cdxpf: a
// filename is also where a SEAT ships, and ADR 0012 D2 leaves the crew
// standing in a line where nothing exempts them in a name).
//
// ADR 0048 D2 then MOVED this instance's own config patterns
// (OpsPatternSet.Extra, config beads_visibility_patterns:) out of check 2
// and into check 3's scope — both its arms — while the shipped OpsPatterns
// stay markdown-only in check 2. The scope argument ADR 0024 D2 made is
// about the SHIPPED list, whose own source and tests are byte-identical to
// hits; a config pattern is never in source, so it has check 3's property
// and gets check 3's reach. Check 0 still scans the effective list, shipped
// and configured both: a mis-routed bead is what that arm is for.
func visibilityGuardBody(visibility string, set OpsPatternSet, identity []IdentityLiteral) string {
	// TWO LISTS, TWO SCOPES (ADR 0048 D2). Check 0 (the beads jsonl) scans
	// the effective list — shipped plus this instance's own — because a
	// mis-routed BEAD is what that arm exists for and its remedy is
	// bead-shaped. Check 2 (markdown) scans the SHIPPED list alone: an
	// instance pattern moved to check 3's scope, which already covers every
	// staged file including markdown, so leaving it here too would only
	// scan the same line twice and refuse it with the wrong remedy.
	//
	// The instance entries in check 0's list are rendered CLASS-ONLY
	// (ranger-base-8114t): check 0 prints its $posse_bad to a terminal
	// exactly as check 3 does, so the same rule applies — the class and a
	// count, never the pattern and never the text it matched.
	var checks, shippedChecks strings.Builder
	for _, p := range OpsPatterns {
		shippedChecks.WriteString(opsCheckCall("    ", p.Class, p.ERE, false))
	}
	checks.WriteString(shippedChecks.String())
	for _, p := range set.Extra {
		checks.WriteString(opsCheckCall("    ", p.Class, p.ERE, true))
	}
	// A config pattern this instance asked for and did not get is recorded
	// HERE, in the file, for the same reason the stamp is: a human reading
	// the hook has to be able to tell what it is checking from what someone
	// meant it to check. Class names only — an instance's pattern IS its
	// confidential vocabulary, and this file is generated, read and pasted.
	var rejects strings.Builder
	if len(set.CeilingRejected) > 0 {
		fmt.Fprintf(&rejects, "# data ceiling patterns REFUSED at stamp time (config %s:), not in force below:\n", DataCeilingConfigKey)
		for _, r := range set.CeilingRejected {
			fmt.Fprintf(&rejects, "#   %s\n", r)
		}
	}
	if len(set.Rejected) > 0 {
		fmt.Fprintf(&rejects, "# instance patterns REFUSED at stamp time (config %s:), not in force below:\n", OpsPatternsConfigKey)
		for _, r := range set.Rejected {
			fmt.Fprintf(&rejects, "#   %s\n", r)
		}
	}
	// THE CEILING RENDERS ABOVE THE STAMP GATE (ADR 0050 D2). The question
	// it asks — may this content exist in a local file here at all? — is not
	// about where the repo goes, so the stamp is not consulted: a private
	// repo runs it, a public repo runs it FIRST, so a line that trips both
	// the ceiling and a visibility list is refused with the stricter remedy.
	// The helpers ($posse_base, posse_check) are defined above the gate for
	// the same reason: both blocks read one definition. Nothing renders when
	// the ceiling list is empty, which is every instance that has not
	// configured one.
	return `
# ─── the beads visibility guard (rangerhq-hrz), extended by ADR 0024 D2 ───
# A bead belongs in a public repo's db only when any deployer of this
# software could have written it; everything describing ONE deployment goes
# in that instance's private db (NOTES.md, "Privacy model"). ADR 0024 D1
# extends the same test to every artifact, not only beads — which is what
# checks 1 and 2 below enforce. This is a pattern lint, not a boundary —
# same class as the allowlist. The boundary is the routing rule plus repo
# visibility; the lint exists so a mis-routed artifact is a refusal at
# commit time instead of a public one.
#
# The data ceiling (ADR 0050) sits ABOVE the visibility gate below and runs
# under every stamp: visibility says where content may go, the ceiling says
# whether it may exist in a local file here at all.
#
# The slot is prepare-commit-msg and not pre-commit for the reason the wall
# below documents: pre-commit is bd's own flush hook, reinstalled silently
# by bd, and a wall a third-party tool replaces on its next install is not
# a wall. This one also survives --no-verify.
` + rejects.String() + `posse_beads_visibility=` + shQuote(visibility) + `

# Compare against HEAD, or against the empty tree in a repo with no commit
# yet — a first commit is exactly when a db (or a docs tree) arrives whole.
# Shared by every check in both blocks below.
posse_base=$(git hash-object -t tree /dev/null 2>/dev/null)
if git rev-parse --verify -q HEAD >/dev/null 2>&1; then posse_base=HEAD; fi

# A function, not a 'while read' over a pipeline: the right side of a
# pipeline is a subshell and the assignment would not survive it — the
# rangerhq-kk6e lesson, which cost a push. Shared by every check below:
# each sets $posse_added and $posse_bad, then calls this over whichever
# list its scope gets — one Go slice per scope, one shell function.
#
# $3 IS THE DISCLOSURE SWITCH (ADR 0048 D2, ranger-base-8114t), and only a
# CONFIGURED pattern sets it — an instance visibility pattern, and every
# data-ceiling pattern (ADR 0050). A shipped OpsPattern carries public text
# — it is in this repo's own source — so showing a writer the string they
# tripped on costs nothing, and a refusal that names only a class is a
# puzzle. A configured pattern is the opposite: the value IS the thing the
# wall exists to keep out — one deployment's confidential vocabulary, or
# content that may not exist in a local file at all — and a refusal is read
# in a terminal, pasted onto a bead and quoted in a transcript. So for those
# the wall says WHICH class was hit and HOW OFTEN and nothing else; the
# words that tripped it stay in the staged tree of whoever wrote them, where
# they already are.
posse_check() {
  if [ -n "$3" ]; then
    posse_n=$(printf '%s\n' "$posse_added" | grep -cE "$2" 2>/dev/null)
    [ "${posse_n:-0}" -gt 0 ] || return 0
    posse_bad="$posse_bad  $1: $posse_n hit(s) — pattern and matched text withheld: a configured class's value is the thing being kept out, so the refusal carries the class alone (ADR 0048 D2, ADR 0050 D2)
"
    return 0
  fi
  posse_m=$(printf '%s\n' "$posse_added" | grep -oE "$2" 2>/dev/null | head -3 | tr '\n' ' ')
  [ -n "$posse_m" ] || return 0
  posse_bad="$posse_bad  $1: $2
    matched: $posse_m
"
}
` + dataCeilingCheck(set.Ceiling) + `
if [ "$posse_beads_visibility" = ` + shQuote(VisibilityPublic) + ` ]; then
  # ─── check 0: the beads db (rangerhq-hrz) ───────────────────────────────
  # ADDED lines only, and every .beads jsonl: the db and the deletion ledger
  # beside it (rangerhq-fuom), which holds whole bead records and inherits
  # the repo's visibility exactly as the db does. GIT_INDEX_FILE is
  # inherited, so this reads the same index the commit will.
  posse_added=$(git diff --cached -U0 ` + diffReaderShape + ` "$posse_base" -- '.beads/*.jsonl' 2>/dev/null |
    grep -a '^+' | grep -av '^+++')
  if [ -n "$posse_added" ]; then
    posse_bad=''
` + checks.String() + `    if [ -n "$posse_bad" ]; then
      if [ "${` + VisibilityOverrideEnv + `:-}" = ` + shQuote(VisibilityOverrideValue) + ` ]; then
        echo "posse gate: visibility guard OVERRIDDEN by ` + VisibilityOverrideEnv + ` — ops-class content is going into a public repo's beads db" >&2
        if [ -n "$RHQ_GATES_DIR" ]; then
          echo "$(posse_stamp) beads visibility guard OVERRIDDEN [prepare-commit-msg hook]" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
        fi
      else
        {
          echo "refused by posse gate: ops-class content in a public repo's beads db — prepare-commit-msg hook, session ${RHQ_PERSONA:-?}"
          echo ` + shQuote(VisibilityRule) + `
          echo "matched in the staged .beads/*.jsonl additions:"
          printf '%s' "$posse_bad"
          echo ` + shQuote(VisibilityWayThrough) + `
          echo "  this repo's beads db is marked: public (stamped by posse gates install-hooks"
          echo "  from config beads_visibility:; an unmarked repo is treated as public)"
          echo "  override, operator-typed, never passed by dispatch:"
          echo "    ` + VisibilityOverrideEnv + `=` + VisibilityOverrideValue + ` git commit -F - -- <paths>"
        } >&2
        if [ -n "$RHQ_GATES_DIR" ]; then
          echo "$(posse_stamp) beads visibility guard [prepare-commit-msg hook] (public repo)" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
        fi
        exit 1
      fi
    fi
  fi

  # ─── check 1: docs-genre allowlist (ADR 0024 D2) ────────────────────────
  # Staged NEW files under docs/ only — 'A' entries; a MODIFIED existing
  # file already cleared this the day it was added. name-status, not -z:
  # docs/ paths are this repo's own and ASCII by convention, and the
  # tab-delimited form is what the cut below expects — a path carrying a
  # literal newline is the same residual constitutionGuardBody's -z form
  # already accepts elsewhere in this hook, just not paid for here.
  # --no-renames (ranger-base-60azj), the same flag and the same reason the
  # NOTES.md arm and the shared-index reader below carry it
  # (ranger-base-x9xbk, gates.go ~2134 — cited, not re-explained): rename
  # detection is ON by default and pairs a source and a destination that are
  # BOTH inside this 'docs/*' pathspec into ONE R100 entry, which '^A' never
  # matches. Without the flag 'git mv docs/adr/x.md docs/rca/x.md' committed
  # clean and the public tree gained the docs/rca/ ADR 0024 D1 says it must
  # never have. The asymmetry is what hid it: a source OUTSIDE docs/ is
  # hidden by the pathspec, git degrades the pair to an A, and the wall
  # fired correctly — only docs/ -> docs/ moves slipped. With the flag the
  # removal and the add are reported separately and the destination is an A
  # entry like any other new file (MEASURED, git 2.50.1).
  posse_docs_hits=$(git diff --cached --name-status --no-renames "$posse_base" -- 'docs/*' 2>/dev/null | grep -E '^A[[:space:]]')
  if [ -n "$posse_docs_hits" ]; then
    posse_docs_bad=''
    posse_docs_ifs=$IFS
    IFS='
'
    for posse_docs_line in $posse_docs_hits; do
      posse_docs_path=$(printf '%s\n' "$posse_docs_line" | cut -f2-)
      case "$posse_docs_path" in
        docs/*/*)
          posse_docs_genre=${posse_docs_path#docs/}
          posse_docs_genre=${posse_docs_genre%%/*}
          ;;
        *)
          posse_docs_genre='(none — staged directly under docs/, no subdirectory)'
          ;;
      esac
      case "$posse_docs_genre" in
        ` + publicDocsGenrePattern() + `) ;;
        *)
          posse_docs_bad="$posse_docs_bad  $posse_docs_path    (genre: $posse_docs_genre)
"
          ;;
      esac
    done
    IFS=$posse_docs_ifs
    if [ -n "$posse_docs_bad" ]; then
      if [ "${` + VisibilityOverrideEnv + `:-}" = ` + shQuote(VisibilityOverrideValue) + ` ]; then
        echo "posse gate: docs-genre allowlist OVERRIDDEN by ` + VisibilityOverrideEnv + ` — a new docs/ file outside the allowlist is going into a public repo" >&2
        if [ -n "$RHQ_GATES_DIR" ]; then
          echo "$(posse_stamp) docs-genre allowlist OVERRIDDEN [prepare-commit-msg hook]" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
        fi
      else
        {
          echo "refused by posse gate: a new docs/ file outside the public genre allowlist — prepare-commit-msg hook, session ${RHQ_PERSONA:-?}"
          echo ` + shQuote(DocsGenreRule) + `
          echo "today's allowlist: ` + publicDocsGenrePattern() + `"
          echo "staged new file(s):"
          printf '%s' "$posse_docs_bad"
          echo ` + shQuote(DocsGenreWayThrough) + `
          echo "  this repo's beads db is marked: public (stamped by posse gates install-hooks"
          echo "  from config beads_visibility:; an unmarked repo is treated as public)"
          echo "  override, operator-typed, never passed by dispatch:"
          echo "    ` + VisibilityOverrideEnv + `=` + VisibilityOverrideValue + ` git commit -F - -- <paths>"
        } >&2
        if [ -n "$RHQ_GATES_DIR" ]; then
          echo "$(posse_stamp) docs-genre allowlist [prepare-commit-msg hook] (public repo)" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
        fi
        exit 1
      fi
    fi
  fi

  # ─── check 2: the SHIPPED OpsPatterns over staged markdown (0024 D2) ────
  # Every staged markdown file, any path — NOT code: the detector's own
  # source and tests are byte-identical to hits (the assembled plan-brand
  # names in visibility.go exist because of exactly this), and a wall
  # carrying an allowlist of its own files is a wall with a hole list.
  # The SHIPPED list only, since ADR 0048 D2: this instance's own config
  # patterns are never in source, so they are scanned by check 3 below over
  # every staged file and path instead — markdown included, which is
  # why they are not scanned twice here.
  # WHICH SPELLINGS is MarkdownPathspecs (visibility.go), one Go list
  # rendered here: git pathspec matching is case-sensitive, so the earlier
  # bare '*.md' never saw docs/adr/x.MD or x.markdown at all and one
  # character walked ops content into a public tree (ranger-base-4b1z4,
  # measured). ':(icase)' is git's own magic for it.
  posse_added=$(git diff --cached -U0 ` + diffReaderShape + ` "$posse_base" -- ` + markdownPathspecArgs() + ` 2>/dev/null |
    grep -a '^+' | grep -av '^+++')
  if [ -n "$posse_added" ]; then
    posse_bad=''
` + shippedChecks.String() + `    if [ -n "$posse_bad" ]; then
      if [ "${` + VisibilityOverrideEnv + `:-}" = ` + shQuote(VisibilityOverrideValue) + ` ]; then
        echo "posse gate: markdown ops-content scan OVERRIDDEN by ` + VisibilityOverrideEnv + ` — ops-class prose is going into a public repo" >&2
        if [ -n "$RHQ_GATES_DIR" ]; then
          echo "$(posse_stamp) markdown ops-content scan OVERRIDDEN [prepare-commit-msg hook]" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
        fi
      else
        {
          echo "refused by posse gate: ops-class content in staged markdown in a public repo — prepare-commit-msg hook, session ${RHQ_PERSONA:-?}"
          echo ` + shQuote(OpsProseRule) + `
          echo "matched in the staged markdown additions:"
          printf '%s' "$posse_bad"
          echo ` + shQuote(OpsProseWayThrough) + `
          echo "  this repo's beads db is marked: public (stamped by posse gates install-hooks"
          echo "  from config beads_visibility:; an unmarked repo is treated as public)"
          echo "  override, operator-typed, never passed by dispatch:"
          echo "    ` + VisibilityOverrideEnv + `=` + VisibilityOverrideValue + ` git commit -F - -- <paths>"
        } >&2
        if [ -n "$RHQ_GATES_DIR" ]; then
          echo "$(posse_stamp) markdown ops-content scan [prepare-commit-msg hook] (public repo)" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
        fi
        exit 1
      fi
    fi
  fi
` + identityGuardCheck(identity, set.Extra) + `fi
`
}

// visGuardRefusal is check 3's refusal shape, written once. SIX call sites
// — the CONTENT arm, the PATH arm and, since ranger-base-qk8i9, the
// MESSAGE arm, each for the derived identity literals and for the instance
// patterns ADR 0048 D2 moved into this scope — differ only in their words,
// and the words are the part a reader of a refusal needs to be right; the
// shape (override branch, one refusals.log line each way, exit 1) is the
// same wall every time.
//
// WHAT NEVER GOES IN: a pattern's VALUE. The header line and both
// refusals.log lines name the CLASS only — an instance pattern IS this
// deployment's confidential vocabulary (visibility.go, OpsPatternsConfigKey),
// and refusals.log is a file that outlives the terminal it printed to.
//
// Nor does the value ride $posse_bad, which is printed to the same terminal
// and gets pasted onto beads the same way (ranger-base-8114t, measured on
// this instance's own pattern): posse_check renders an instance pattern
// class-only, so the two INSTANCE refusals here carry a class, a hit count
// and — on the path arm — the staged path, which is the writer's own
// artifact and the only thing that says WHICH file. The two IDENTITY
// refusals keep ADR 0024 D2's shape, matched text included: a derived
// literal is the operator's own username or email, and the box it names is
// the box reading the refusal — and that stays true of the MESSAGE arm: the
// text it prints back is a message the same writer just typed, on the box
// the literal names.
type visGuardRefusal struct {
	badVar       string // the shell variable this arm accumulated its hits in
	label        string // what refusals.log calls this scan
	logTail      string // what follows the label on the refusal's log line
	overrideAt   string // "" or " (staged path)", on the OVERRIDDEN log line
	overrideWhat string // the clause after the em dash in the override echo
	header       string // the refusal's first line, after "refused by posse gate: "
	rule         string // the rule it names — a refusal that names no rule is a regex saying no
	matched      string // the line that introduces the matched text
	wayThrough   string
	// keptModeVar is "" or the name of the shell variable messageArm sets to
	// the live template-KEEPING cleanup mode (ranger-base-b21e0). Non-empty
	// on the MESSAGE refusals alone, because that variable exists only inside
	// messageArm and because it is only the message the mode decides the read
	// of; the content and path arms read a diff and a listing and no cleanup
	// mode touches either. When it is set the refusal appends
	// MessageKeptTemplateNote after its remedy — and only when the variable
	// is non-empty AT HOOK TIME, which is the mode being one of the three
	// AND git being about to append a template.
	keptModeVar string
	// footer is the two lines after the remedy that say what this wall
	// was stamped with. Empty means visPublicFooter — the stamp line the
	// gated checks have always printed; the ceiling, which runs under every
	// stamp, prints its own (ADR 0050 D2).
	footer [2]string
}

// visPublicFooter is what every check INSIDE the visibility gate says
// about the stamp it ran under: it is only ever reached when the repo is
// stamped public.
var visPublicFooter = [2]string{
	"this repo's beads db is marked: public (stamped by posse gates install-hooks",
	"from config beads_visibility:; an unmarked repo is treated as public)",
}

// render is the refusal at base indentation ind — the indent of its own
// `if`. Check 3 renders inside the visibility gate (ind is four spaces);
// the data ceiling renders above it (two).
func (r visGuardRefusal) render(ind string) string {
	footer := r.footer
	if footer == [2]string{} {
		footer = visPublicFooter
	}
	i1, i2, i3 := ind+"  ", ind+"    ", ind+"      "
	return ind + `if [ -n "$` + r.badVar + `" ]; then
` + i1 + `if [ "${` + VisibilityOverrideEnv + `:-}" = ` + shQuote(VisibilityOverrideValue) + ` ]; then
` + i2 + `echo "posse gate: ` + r.label + ` OVERRIDDEN by ` + VisibilityOverrideEnv + ` — ` + r.overrideWhat + `" >&2
` + i2 + `if [ -n "$RHQ_GATES_DIR" ]; then
` + i3 + `echo "$(posse_stamp) ` + r.label + ` OVERRIDDEN [prepare-commit-msg hook]` + r.overrideAt + `" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
` + i2 + `fi
` + i1 + `else
` + i2 + `{
` + i3 + `echo "refused by posse gate: ` + r.header + ` — prepare-commit-msg hook, session ${RHQ_PERSONA:-?}"
` + i3 + `echo ` + shQuote(r.rule) + `
` + i3 + `echo "` + r.matched + `"
` + i3 + `printf '%s' "$` + r.badVar + `"
` + i3 + `echo ` + shQuote(r.wayThrough) + `
` + r.keptModeNote(i3) + i3 + `echo "  ` + footer[0] + `"
` + i3 + `echo "  ` + footer[1] + `"
` + i3 + `echo "  override, operator-typed, never passed by dispatch:"
` + i3 + `echo "    ` + VisibilityOverrideEnv + `=` + VisibilityOverrideValue + ` git commit -F - -- <paths>"
` + i2 + `} >&2
` + i2 + `if [ -n "$RHQ_GATES_DIR" ]; then
` + i3 + `echo "$(posse_stamp) ` + r.label + ` [prepare-commit-msg hook] ` + r.logTail + `" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
` + i2 + `fi
` + i2 + `exit 1
` + i1 + `fi
` + ind + `fi
`
}

// keptModeNote is the mode clause of the remedy, at the echo indent ind, or
// "" for a refusal that has no kept-mode variable (every content and path
// arm). It renders INSIDE the refusal's `{ ... } >&2` group and between the
// remedy and the stamp lines, because it modifies the remedy: a writer who
// reads "rewrite the commit message" and stops there has been told to do
// something that may not be doable.
//
// The mode is printed from the variable rather than baked in — one hook file
// serves whatever ~/.gitconfig says today, and the whole point of the note is
// to name the setting the writer did not know was live. The "scissors" test is
// a branch rather than a case in the note itself because that mode puts git's
// block on the far side of its cut line, which the read stops at
// (ranger-base-xfgcn): verbatim and whitespace scan git's block and LAND it
// (MessageKeptTemplateNote + MessageKeptLandsNote, so the writer knows which
// lines the wall could have read and that the wall is not in their way),
// scissors scans neither (MessageScissorsNote, which says what IS above the
// cut and therefore what tripped this). Both paragraphs are about the same
// bytes, so under scissors they are replaced rather than added to.
func (r visGuardRefusal) keptModeNote(ind string) string {
	if r.keptModeVar == "" {
		return ""
	}
	v := "$" + r.keptModeVar
	return ind + `if [ -n "` + v + `" ]; then
` + ind + `  echo "git's cleanup mode here is \"` + v + `\" (config commit.cleanup), and it decided this read:"
` + ind + `  if [ "` + v + `" = scissors ]; then
` + ind + `    echo ` + shQuote(MessageScissorsNote) + `
` + ind + `  else
` + ind + `    echo ` + shQuote(MessageKeptTemplateNote) + `
` + ind + `    echo ` + shQuote(MessageKeptLandsNote) + `
` + ind + `  fi
` + ind + `fi
`
}

// The sets of words the two-arm scans refuse in. Named rather than inlined
// so the SOURCES are visibly parallel: the same two arms, the same
// override, the same log shape, a different rule and a different remedy —
// which is the whole content of ADR 0048 D2, and of ADR 0050 D2 again one
// block up.
const (
	identityScanLabel    = "identity literal scan"
	instanceScanLabel    = "instance pattern scan"
	crewScanLabel        = "crew name scan"
	dataCeilingScanLabel = "data ceiling scan"
	stagedPathMatched    = "matched in the staged added path(s) — the FILENAME, not its content:"
	stagedLineMatched    = "matched in the staged additions:"
	commitMessageMatched = "matched in the commit message:"
)

// dataCeilingStampTail is what the ceiling's refusals.log lines carry after
// the label: the stamp the scan ran under, so a reader can tell a
// private-repo ceiling hit from a public-repo one without opening the hook
// (ADR 0050 D2). It is the shell variable, not the render-time value —
// expanded inside the hook's own double-quoted echo, so the log line says
// what the FILE was stamped with, which is the same thing.
const dataCeilingStampTail = "stamp: $posse_beads_visibility"

// visScanSource is one list the scan runs: how its checks render at an
// indent, the accumulator its path arm folds into, and the three refusals —
// content, path, message — it speaks in. The first two are twoArmScan's;
// the third is messageArm's, and a source whose wall does not scan the
// message leaves it zero (nothing in this file does today — both walls
// scan all three since ranger-base-o2v6n and ranger-base-qk8i9).
type visScanSource struct {
	checks  func(indent string) string
	pathVar string
	content visGuardRefusal
	path    visGuardRefusal
	message visGuardRefusal
	// pathSkip narrows this source's PATH arm to the paths its own rule
	// reaches, and nil — the zero — is "every staged path", which is what
	// three of the four sources want. It renders inside the per-path loop,
	// ahead of this source's checks, and sets $posse_skip; the source's
	// block then runs only for a path the filter leaves standing. Per
	// SOURCE and not per scan, because the exemption belongs to one rule:
	// the crew names reach only the trees ADR 0012 D6 puts inside App.A 5
	// (crewPathSkip), while an operator identity literal in the same path
	// is refused exactly as it always was.
	pathSkip func(indent string) string
}

// twoArmScan renders the shape ranger-base-uzgkz built for check 3 and ADR
// 0050 D2 reuses for the ceiling, ONCE: the ADDED lines of every staged
// file, then the ADDED staged paths, each source accumulating into its
// own variable and refusing in its own words. ind is the base indentation
// of the block (check 3 sits inside the visibility gate; the ceiling sits
// above it); title names the block in the second arm's banner; head is the
// first arm's own comment, already rendered at ind.
//
// TWO SOURCES, ONE SCAN, TWO REFUSALS — the rule for check 3 and the
// reason this is a list: every source runs over the SAME $posse_added and
// the same per-path loop — one `git diff`, one listing — but each
// accumulates into its own variable and refuses in its own words. Merging
// them would be shorter and would tell a writer that an instance codename
// in a comment is "an operator identity literal", which is a refusal that
// names the wrong rule and sends them to the wrong remedy.
//
// checks() is rendered at two indents rather than once into a variable
// because the path arm needs it INSIDE a loop: posse_check accumulates the
// class and the matched text but not the subject, so the only way to name
// the offending path in the refusal — which is what distinguishes a path
// hit from a content hit for the reader — is to run the same matcher over
// one path at a time.
func twoArmScan(ind, title, head string, sources []visScanSource) string {
	i1, i2 := ind+"  ", ind+"    "
	var content, loop, pathRefusals, pathInit strings.Builder
	for _, s := range sources {
		// A source may sit out an arm, and one does: the crew names are
		// PATHS ONLY (ranger-base-cdxpf, IdentityLiteral.PathsOnly says
		// why). An absent arm is an absent refusal — the zero
		// visGuardRefusal, whose badVar is "" — and skipping it here is
		// what keeps its checks from being rendered over a subject its
		// rule does not govern.
		if s.content.badVar != "" {
			content.WriteString(i1 + "posse_bad=''\n" + s.checks(i1) + s.content.render(i1))
		}
		// A source may govern a SUBSET of the staged paths (pathSkip), and
		// one does. The filter renders INSIDE the loop and ahead of this
		// source's own checks, so the listing, the loop and every other
		// source still see every path — what is narrowed is one rule's
		// reach, not the scan's.
		bi := i2
		if s.pathSkip != nil {
			loop.WriteString(s.pathSkip(i2) + i2 + `if [ -z "$posse_skip" ]; then` + "\n")
			bi = i2 + "  "
		}
		loop.WriteString(bi + "posse_bad=''\n" + s.checks(bi) + pathAccum(bi, s.pathVar))
		if s.pathSkip != nil {
			loop.WriteString(i2 + "fi\n")
		}
		pathRefusals.WriteString(s.path.render(i1))
		pathInit.WriteString(i1 + s.pathVar + "=''\n")
	}
	// No source scans content: no diff, and no `if` with an empty body —
	// which is not a shell program at all.
	contentArm := ""
	if content.Len() > 0 {
		contentArm = shComment(ind, `--text and grep -a, both load-bearing (ranger-base-h137b): git
classifies a text file carrying one NUL byte as BINARY and prints
"Binary files ... differ" for it, never a '+' line — so this reader
judged a markdown file with captured output appended to it, and said
nothing. --text restores the lines; -a stops grep collapsing the
NUL-bearing stream to "Binary file (standard input) matches". The
$(...) capture strips the NULs, so nothing downstream sees one.`) +
			ind + `posse_added=$(git diff --cached -U0 ` + diffReaderShape + ` "$posse_base" 2>/dev/null |
` + i1 + `grep -a '^+' | grep -av '^+++')
` + ind + `if [ -n "$posse_added" ]; then
` + content.String() + ind + `fi

`
	}
	return head + contentArm +
		shComment(ind, `─── `+title+`, second arm: the same patterns over ADDED staged PATHS ─────
Every flag below is load-bearing and measured (git 2.50.1):
  --no-renames  with move detection ON — git's default since 2.9 —
                --diff-filter=A prints NOTHING for a pure move, so the
                destination of a git mv is only an added ENTRY with the
                flag. Same flag, same reason, as the NOTES.md arm and the
                shared-index reader (ranger-base-x9xbk, gates.go ~2134,
                which explains it once) and as check 1.
  --diff-filter=A  added entries only: not deleted (a deletion carries a
                path away) and not modified (already in history, check
                1's precedent).
  core.quotePath=false  so a literal carrying a non-ASCII byte is matched
                raw rather than against git's octal-escaped spelling —
                the same reason constitutionGuardBody reads paths
                unquoted. RESIDUAL, stated: git still C-quotes a path
                holding a double quote, a backslash or a control byte
                whatever quotePath says (ranger-base-qg0k8), so such a
                path is matched in its escaped spelling here. It can only
                mis-match a literal that itself carries one of those
                bytes; none of the sources produces one.
-f and IFS=newline: the loop splits an unquoted expansion, so a path with
a glob character stays a path and a path with spaces stays one word —
the constitution arm's spelling, for the same reason. A path holding a
LITERAL newline splits into two subjects, which is check 1's accepted
residual and fail-safe here: each half is still scanned, and no derived
literal contains a newline, so the split can only over-match.`) +
		ind + `posse_ipaths=$(git -c core.quotePath=false diff --cached --name-only --no-renames --diff-filter=A "$posse_base" 2>/dev/null)
` + ind + `if [ -n "$posse_ipaths" ]; then
` + pathInit.String() + i1 + `set -f
` + i1 + `posse_iifs=$IFS
` + i1 + `IFS='
'
` + i1 + `for posse_ip in $posse_ipaths; do
` + i2 + `posse_added=$posse_ip
` + loop.String() + i1 + `done
` + i1 + `IFS=$posse_iifs
` + i1 + `set +f
` + pathRefusals.String() + ind + `fi
`
}

// shComment renders text as a shell comment block at ind, one `# ` per
// line — so a block that is rendered at two indents carries its prose once.
func shComment(ind, text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(ind + "# " + line + "\n")
	}
	return b.String()
}

// dataCeilingCheck renders the data ceiling's block (ADR 0050 D2): this
// instance's config data_ceiling_patterns: over the ADDED lines of every
// staged file, the ADDED staged paths, and every line of the commit
// MESSAGE — check 3's three subjects, through the same two renderers (the
// ceiling's message arm since ADR 0050 D2 as amended 2026-09-03,
// ranger-base-pqlxr; check 3's since ranger-base-1nbtn; ceilingMessageHead
// below carries the reasons). All three ABOVE the visibility gate, so they
// run whatever the repo's stamp, where check 3's three run inside it: same
// subjects, different gate and different remedy, which is why the ceiling
// refuses first.
// "" when the list is empty: an instance that configured no ceiling pays
// for no diff and for no read of the message file.
//
// ALWAYS class-only, and not as a courtesy: a refusal is itself a local
// file — the terminal, the transcript, the pane capture, refusals.log —
// and a ceiling refusal that printed the matched text would breach the
// ceiling by the wall's own hand (ADR 0050 Context). Its words are its own:
// the rule says the content may not exist here, the remedy says remove the
// paste and keep the cite (there is no private db to re-file into), and
// the footer names the stamp this repo carries rather than asserting
// "public", because the block did not read it.
func dataCeilingCheck(ceiling []OpsPattern) string {
	if len(ceiling) == 0 {
		return ""
	}
	checks := func(indent string) string {
		var b strings.Builder
		for _, p := range ceiling {
			b.WriteString(opsCheckCall(indent, p.Class, p.ERE, true))
		}
		return b.String()
	}
	footer := [2]string{
		"this wall runs under every visibility stamp — this repo's beads db is stamped: $posse_beads_visibility",
		"(stamped by posse gates install-hooks from config beads_visibility:; the ceiling did not read it)",
	}
	src := visScanSource{
		checks:  checks,
		pathVar: "posse_dbad",
		content: visGuardRefusal{
			badVar:       "posse_bad",
			label:        dataCeilingScanLabel,
			logTail:      "(" + dataCeilingStampTail + ")",
			overrideAt:   " (" + dataCeilingStampTail + ")",
			overrideWhat: "content above this instance's data ceiling is going into a local file",
			header:       "data-ceiling content in a staged file",
			rule:         DataCeilingRule,
			matched:      stagedLineMatched,
			wayThrough:   DataCeilingWayThrough,
			footer:       footer,
		},
		path: visGuardRefusal{
			badVar:       "posse_dbad",
			label:        dataCeilingScanLabel,
			logTail:      "(" + dataCeilingStampTail + ", staged path)",
			overrideAt:   " (" + dataCeilingStampTail + ", staged path)",
			overrideWhat: "content above this instance's data ceiling is going into a local file in a staged PATH",
			header:       "data-ceiling content in a staged PATH",
			rule:         DataCeilingRule,
			matched:      stagedPathMatched,
			wayThrough:   DataCeilingWayThrough,
			footer:       footer,
		},
	}
	head := "\n" + shComment("", `─── the data ceiling (ADR 0050): this instance's `+DataCeilingConfigKey+`: ────
Runs under EVERY visibility stamp — it sits above the posse_beads_visibility
gate on purpose. A visibility pattern asks whether content may be PUBLIC and
is inert in a repo stamped private; a ceiling pattern asks whether content
may exist in a local file on this instance AT ALL — a restricted-tier
banner, a restricted system's hostname, its export file-name shape — and
the answer does not depend on where the repo goes. The system of record's
id is the sanctioned citation; the content behind it is not.
THREE ARMS, and check 3 below scans the same three: ADDED lines of every
staged file (any path, code included), then ADDED staged paths, then every
line of the commit MESSAGE, which this hook is already holding in "$1"
(the ceiling's arm since ADR 0050 D2 as amended 2026-09-03,
ranger-base-pqlxr; check 3's since ranger-base-1nbtn). Same matcher, same
override, class-only ALWAYS: a refusal is itself a local file.
What separates the two walls is not the subject but the GATE and the
REMEDY: these three run here, above posse_beads_visibility, under every
stamp, where check 3's run inside it — so a repo stamped private runs
check 3 not at all and this block in full.
Refused FIRST so a line that trips both this list and a visibility list is
refused with the stricter remedy — there is no private db to re-file it in.`)
	msg := visGuardRefusal{
		badVar:       "posse_bad",
		label:        dataCeilingScanLabel,
		logTail:      "(" + dataCeilingStampTail + ", commit message)",
		overrideAt:   " (" + dataCeilingStampTail + ", commit message)",
		overrideWhat: "content above this instance's data ceiling is going into the commit MESSAGE",
		header:       "data-ceiling content in the commit MESSAGE",
		rule:         DataCeilingRule,
		matched:      commitMessageMatched,
		wayThrough:   DataCeilingMessageWayThrough,
		keptModeVar:  "posse_kept",
		footer:       footer,
	}
	src.message = msg
	return twoArmScan("", "the data ceiling", head, []visScanSource{src}) + messageArm("", ceilingMessageHead, []visScanSource{src})
}

// messageArm renders the THIRD arm both walls have: the commit MESSAGE,
// every line of it, as given (the ceiling's since ADR 0050 D2 as amended
// 2026-09-03, ranger-base-pqlxr; check 3's since ADR 0024 D2 / ADR 0048 D2
// as amended 2026-09-03, ranger-base-1nbtn, built in ranger-base-qk8i9).
// ONE reader of "$1" for both, for the reason twoArmScan is one function:
// the walls differ in their words and their gate, never in what they read.
// It renders AFTER the two-arm scan — the ceiling's above the visibility
// gate, check 3's inside it — so the order a reader gets is content, path,
// message, and the ceiling's three before the gate line.
//
// One `cat` per wall, not per source: the sources share $posse_added and
// each resets its own $posse_bad before its checks, exactly as the content
// arm does. head is the arm's own banner, already rendered at ind.
//
// WHY THE MESSAGE IS A SUBJECT AT ALL. D5 says the ceiling guards the durable,
// replicated copy and excludes the working tree, the transcript and the
// pane capture. A commit message is none of those: it lands in the commit
// object and replicates with the branch. It is also the most-quoted
// artifact in this shop — a persona's message cites the context it worked
// from, which is the paste shape exactly. MEASURED on ranger-base-zikpp: a
// ceiling-matching message committed clean while the same bytes in a staged
// file were refused.
//
// NO NEW GIT COMMAND, and no new dependency: the message file is $1 and
// `cat` is already this hook's (the MERGE_MSG compare in the shared-index
// arm reads the same file the same way).
//
// $1 SURVIVES THE CHAIN, and it is the one thing this arm needs that the
// other two do not: posse's own dispatcher runs `"$d/posse-<slot>" "$@"`
// (chainRender, and the note at gates.go ~2345 says so where the chain is
// built), so a chained install hands the member the same argv git
// handed the slot. RESIDUAL, stated: behind a FOREIGN dispatcher (INSTALL.md
// §9) this arm sees whatever that dispatcher passes on. A dispatcher that
// drops the argument leaves the arm reading nothing — it cannot refuse
// wrongly, only stay silent, which is the same failure shape the whole hook
// has when it is not installed at all.
//
// WHAT IS READ IS WHAT GIT WILL KEEP, and those are not the same bytes on
// every path (ranger-base-h3s6q finding 2; measured, git 2.50.1). Exactly one
// thing decides how the file is read, and it is git's CLEANUP MODE — the
// thing that decides whether git throws comment lines away before it writes
// the object. Nothing else here reads it, so the arm asks for it by name:
//
//	strip                       git removes every comment line. Read through
//	                            stripspace: the template is not the object.
//	verbatim, whitespace,       git removes no comment line — under
//	scissors                    "verbatim" not even its own template (MEASURED
//	                            below). Read WHOLE.
//	default, unset, absent      "default" is not a mode, it is "strip if the
//	                            message is edited, whitespace if not", and
//	                            "$2" is the only handle this hook has on
//	                            edited-ness: "message" is -m and -F, where git
//	                            appends NO template and KEEPS a '#'-leading
//	                            line, so a pasted markdown heading commits and
//	                            the read is whole; anything else is the editor
//	                            path, a commit.template, a merge, a squash, an
//	                            --amend, where git has already written the
//	                            "On branch <name>" line, the status block
//	                            listing staged, unstaged and UNTRACKED paths
//	                            and a merge's "# Conflicts:" list, and strips
//	                            every one of them — read through stripspace.
//
// "$2" WAS THE WHOLE ANSWER UNTIL ranger-base-6y3z2 AND IT IS A PROXY, not
// the mode: it says whether git will EDIT the message, which picks the mode
// only while the mode is "default". Set commit.cleanup and the proxy breaks
// in both directions, MEASURED at the shell, git 2.50.1 (Apple Git-155):
//
//	verbatim + editor   git KEPT its whole template in the commit object —
//	                    the Please-enter paragraph, "On branch main", the
//	                    staged list and the UNTRACKED file list, one of which
//	                    is a file NAME. The arm read that file through
//	                    stripspace and stripped exactly those lines out of the
//	                    scan, so a class carried by a branch name, an
//	                    untracked path or a conflict list reached a PUBLIC
//	                    repo's commit object with no wall speaking. Fail-OPEN,
//	                    and reachable from ~/.gitconfig alone.
//	strip + -m          git stripped a '#'-leading line the arm had read whole
//	                    and refused over. Over-refusal, the cheap direction:
//	                    the writer clears it by rewriting.
//
// WHAT THE HOOK CAN AND CANNOT SEE OF THE MODE, measured in a hook that
// printed both: `git config --get commit.cleanup` answers for ~/.gitconfig,
// the repo config AND `git -c commit.cleanup=... commit` (git exports that
// one in GIT_CONFIG_PARAMETERS, and `git config` reads it back). It does not
// answer for `git commit --cleanup=...`: rc 1, empty, nothing in the
// environment carries it. RESIDUAL, stated: the flag form is invisible here
// and the arm reads the path the way "default" would. The config form needs
// no intent — the flag has to be typed — and git's own template says which
// mode is live ("Lines starting with '#' will be kept" against "will be
// ignored"), so a later layer that wants the flag can read git's sentence;
// this one does not, because a localized sentence is the same kind of copy
// as the literal '#' rejected below and it drifts the same way.
//
// AN INVALID VALUE IS NOT THIS ARM'S PROBLEM: git rejects one outright
// ("fatal: Invalid cleanup mode nonsense", and the value is case-sensitive —
// "Verbatim" is fatal too), so the commit never reaches an object. Anything
// unrecognized here lands in the "default" row, which is where the arm was
// before this change.
//
// THE COST OF THE FAIL-CLOSED SIDE, stated because it is h3s6q's complaint
// wearing a config: under verbatim, whitespace or scissors the editor path
// now reads git's template again, so ONE untracked file whose name carries a
// class refuses those writers' editor commits before the editor opens. For
// verbatim and whitespace the bytes genuinely land and the refusal is true;
// under scissors the whole-file read also scans the block BELOW git's cut
// line, which git truncates, and that is over-refusal on top of over-refusal.
// Matching that cut line here would be a second copy of git's rule, so it is
// not matched.
//
// WHAT THE REFUSAL SAYS ABOUT IT (ranger-base-b21e0, the message half of the
// cost above). The verdict was already true; the REMEDY was not, because
// "rewrite the commit message" names a rewrite that has not happened yet when
// this hook runs before the editor opens. So the refusal now names the live
// mode and what actually clears it — take the class out of the repo, or
// leave commit.cleanup at its default — and says which side of the line the
// mode puts the writer on: verbatim and whitespace LAND the block
// (MessageKeptLandsNote), scissors puts it below a cut line this read now
// stops at, so under that mode git's block is neither scanned nor kept and
// the note says what IS above the cut instead (MessageScissorsNote, rewritten
// by ranger-base-xfgcn). It is
// keptModeNote below, driven by $posse_kept, which messageArm sets only when
// the config named one of the three AND "$2" is not "message": on -m/-F git
// appends no template and every line is the writer's own, so the old remedy
// is doable exactly as written and the note stays quiet. RESIDUAL, stated:
// `git commit -m ... -e` opens an editor and DOES get a template while "$2"
// is still "message" (MEASURED, git 2.50.1: source "message", the Please-enter
// block in the file, and the whole block landed in the object), so that one
// writer gets the unqualified remedy — the quiet direction, and the same shape
// as the --cleanup flag being invisible. Closing it would mean asking whether
// the file HOLDS comment lines, and the only reader of that which is not a
// second copy of git's rule is stripspace itself — which cannot separate a
// template git wrote from a '#' line the writer typed with -m, so it would buy
// this path by mis-firing on that one.
// The layer that could tell a typed line from a written one is still the
// commit-msg hook this file names as missing; the remedy did not need it.
//
// WHAT THE FULL-FILE READ COST, measured before the split: ONE untracked
// file whose NAME carried a ceiling class — never staged, never typed —
// refused every editor commit in the repo, with the remedy "rewrite the
// commit message", which cannot clear a hit that is not in the message. A
// branch named for a class did the same to every editor commit while the
// identical commit with -m landed: same content, opposite verdicts, decided
// by which commit form was typed. Nothing escaped — the direction is
// fail-closed — what it cost was a writer who cannot do what the refusal
// tells them.
//
// WHY THIS IS NOT `grep -v` OVER '^#'. The comment character is the writer's
// config (core.commentChar, core.commentString since git 2.45), so a literal
// '#' here is a second copy of git's rule and a copy is a thing that drifts.
// `git stripspace --strip-comments` reads the same config git does —
// measured with core.commentChar=';', where it strips a template a '^#'
// grep would have handed straight to the scan.
//
// THE ONE VALUE IT DOES NOT READ THE SAME WAY: `auto` (ranger-base-vzx2n,
// found verifying h3s6q's close). `auto` is not a character, it is a
// DECISION git makes per message — commit.c's adjust_comment_line_char picks
// a character that starts no line of the message it is about to write — and
// `git stripspace` has no message to decide against, so it answers with a
// plain '#'. MEASURED, git 2.50.1: with core.commentChar=auto over a body
// carrying a '#'-leading line, git wrote its own template in ';' and KEPT
// the '#' line, while stripspace kept the ';' template and STRIPPED the '#'
// line. The two readers part in both directions at once, and the second one
// is a fail-open in a wall whose whole subject is text that may not land.
//
// SO FOR `auto` THE CHARACTER COMES FROM THE FILE, NOT THE CONFIG. git has
// already made the choice by the time this hook runs — it writes its
// template before prepare-commit-msg (measured) — so the block it wrote is
// the answer, written down. What is looked for is the LAST line of "$1"
// that is a bare comment character on its own — one of commit.c's ten
// candidates and nothing else on the line (`#;@!$%^&|:`, read out of the
// git binary under test, not copied from memory). git's block always has
// such a line between its paragraphs, and the character on it is the
// character git chose. It is accepted when at least four lines of the file
// start with it — the smallest block measured was five, `merge --edit`'s;
// the plain editor commit's was eleven, `-v`'s fifteen and `--amend`'s
// seventeen. The read is then stripspace with THAT character, which
// stripspace does resolve.
//
// WHY THE BARE LINE AND NOT THE FILE'S LAST LINE (ranger-base-vl9g8, found
// verifying vzx2n's own fix). The first cut of this took the last line of
// "$1" and used its first character. That is git's block only when git's
// block is last, and under `commit -v` / commit.verbose=true it is not: git
// appends the scissors marker and then the staged DIFF, so "$1" ends in
// `+line`, no character is detected, and the whole file — git's status
// block, untracked filenames and all — went to the scan (the diff itself is
// off the read since ranger-base-xfgcn, below; the character detection is
// still the reason a `-v` file's block has to be found by something other
// than its last line). That is exactly
// h3s6q's over-refusal returning, one common flag away, with the remedy
// "rewrite the commit message" that clears none of it. A bare comment-char
// line cannot appear in a unified diff (every diff line carries a ' ', '+',
// '-', '@', '\' or a header word in column one), so taking the LAST one
// finds git's block under `-v` and without it alike, and a body line still
// loses to the block that comes after it. MEASURED, git 2.50.1, over five
// shapes of "$1": plain editor commit, `-v`, `--amend`, `merge --edit`, and
// commit.status=false.
//
// The last line of that grep is taken with `sed -n '$p'` and not with
// `tail -1`: the hooks may call only the commands scripts/cleanroom.sh names
// in HOOK_DEPS and probes on every distro, sed is already one of them, and
// TestHookDepsNamesEveryCommandTheRenderedHooksCall reds on a command that
// is not (it did, on `tail`).
//
// NO BLOCK, NO STRIP, and that is not a hedge — it is what git does. Under
// `auto` the character is chosen so that it starts no line of the message,
// so the ONLY lines git strips are the ones git itself appended. Where it
// appended none — commit.status=false, where "$1" is the commit.template
// body alone and git writes no block at all (measured), `--amend --no-edit`
// — git strips nothing of the body, the whole file is exactly what it will
// keep, and the whole file is what is read. Every way the detection can
// drift lands in that same read, which is the fail-CLOSED side.
//
// RESIDUAL, stated: a message with no block of git's that itself carries a
// bare '#', ';', '@' … line and four or more lines starting with that same
// character would be read as a block and stripped. That one is fail-OPEN,
// and it is the price of not carrying a second copy of
// adjust_comment_line_char here; the shape is not one a writer types by
// accident, and it is narrower than the hole it replaces, which needed only
// one '#'-leading line.
//
// EVERY PATH IS STILL SCANNED, and $2 still decides nothing about WHETHER:
// the hook distrusts it for the shared-index arm's exemptions (the table
// above says why — $2 is "message" mid-merge and for a clean revert alike).
// That is also what keeps --amend inside: git hands the hook HEAD's message
// there, so a message REUSED after the ceiling was configured is scanned.
//
// RESIDUAL, stated: `--amend --no-edit` and `-C` append no template and
// clean with "whitespace", so a '#'-leading line in the message being reused
// does land and this arm no longer reads it. That text is already in a
// commit object in this repo — the wall it had to pass is the commit that
// first wrote it — and refusing the amend would not take it back out.
//
// THE ONE PATH IT DOES NOT REACH, stated (ADR 0050 D5, and ADR 0024 D2 for
// check 3): a message typed in the EDITOR. prepare-commit-msg runs before
// the editor opens, so on that path the file holds git's template and
// whatever was already in it (a commit.template body, MERGE_MSG) and never a
// word the writer is about to type. The editor path is the operator's own
// hand, which is above both walls already; the second layer for it is a
// commit-msg hook, and the trigger for filing it is the first "commit
// message" line under either label in refusals.log.
//
// THE READ STOPS AT GIT'S CUT LINE (ranger-base-xfgcn, found verifying
// vl9g8's close, and the second half of the cost ranger-base-dgh7y named and
// was closed without). stripspace removes COMMENT lines; the diff
// `commit -v` appends below the scissors marker is not comment-prefixed, so
// it survived every strip and reached the scan whole. What that cost is not
// a longer status block: the sibling arm reads `git diff --cached -U0` —
// ADDED lines, zero context — while git writes that same diff with THREE
// lines of context, so this arm refused over an UNCHANGED line within three
// of a staged hunk, and over the REMOVAL of a classed line, which is the one
// remediation the ceiling's own refusal demands. Both under "rewrite the
// commit message", which clears neither. One config key the writer owns
// (commit.verbose=true) or one flag, with no intent.
//
// SO THE FILE IS CUT WHERE GIT CUTS IT, and only where git cuts it. git
// truncates at its cut line when the commit is verbose or when
// commit.cleanup is `scissors`, and writes that line in exactly those two
// cases. MEASURED, git 2.50.1: under commit.verbose=true the bytes below the
// line are gone from the object under EVERY cleanup mode — `verbatim` and
// `whitespace` included, which keep every byte above it — and under
// `scissors` git puts its whole status block below the line, so nothing it
// wrote is read there at all.
//
// `-v` IS A FLAG as well as a config key and a hook cannot see argv, so what
// is read is what git wrote into the file. The line matched is git's own:
// one comment prefix, one space, and the marker exactly as git spells it,
// never on a line beginning ' ', '+', '-', '@' or '\' — a unified diff
// cannot carry the marker in column one, so a staged file that contains one
// (the pins in verbosescissors_qa_test.go do) cannot move the cut. The FIRST
// such line wins because that is git's rule and not a guess at it: MEASURED,
// git 2.50.1, with the marker forged into a commit.template body under `-v`,
// git truncated at the FORGED line and its own diff went with it —
// wt_status_locate_end takes the first, and so does this.
//
// The marker is git's constant written down, which is the copy this file
// argues against everywhere else. It is taken on the terms the argument
// allows: if git ever respells it the match finds nothing, no cut is made,
// the read is the whole file again — the fail-CLOSED side, exactly where
// this arm was before — and the verbose pins go red saying so.
//
// AND IT IS LICENSED, because that line's presence is not by itself git's
// truncation: a commit.template body carrying the marker is NOT truncated
// under any other mode — the text below it landed in the object (MEASURED,
// git 2.50.1) — so cutting there unguarded takes exactly that text off the
// scan. The first licence is `commit.cleanup=scissors`, read from the
// config this arm has already asked for, and it stands. The second WAS a
// `diff --` line below the marker, "which only a verbose commit appends" —
// and that was a writer-typed shape (ranger-base-d94zl, found verifying
// xfgcn's close): nothing asked who wrote the file, a `diff --` line is four
// characters, and git keeps every byte below a marker it did not write. So
// `git commit -F msg -- path` with the marker at column one, one `diff --`
// line and a classed line under it landed the class in the object, read by
// nothing — the crew's own commit form, no config, no intent beyond typing
// the marker. Measured on the ceiling; check 3 renders through this same
// function and was open the same way.
//
// WHAT ONLY GIT CAN BE ASKED is the commit itself, not the file
// (ranger-base-gyrnp). Everything git writes below its cut line is two
// comment lines and the STAGED DIFF, and the staged diff is bytes the index
// already holds — the sibling arm's subject where they are additions, and a
// tree object's where they are context or removals. So the block below the
// marker is read MINUS the lines of `git diff --cached`, and whatever is
// left is message: under `-v` that is git's two comment lines, which the
// mode's read then strips or keeps exactly as it does the block above; from
// a writer it is every line they typed that the index does not carry, and a
// class there is refused as it was before the cut existed. Nothing here
// asks who wrote the file, because the file cannot say. A writer who pastes
// the staged diff itself under a forged marker has put nothing below it
// that is not in the index already; a writer who adds one line to that
// paste has put that line back on the scan.
//
// THE REFERENCE IS RENDERED THE WAY `-v` RENDERS IT, measured line-for-line
// against what git wrote into "$1" (git 2.50.1, inside the hook): `git diff
// --cached --no-color --no-ext-diff --no-relative <base>`, where <base> is
// $posse_base — HEAD, or the empty tree in a repo with no commit — except
// under `--amend`, where git diffs the index against HEAD^1 and so does this
// ("$2" is `commit` and "$3" is `HEAD` there; `-c <sha>` is `commit` with
// another "$3" and diffs against HEAD, measured). Only what `-v` itself
// pins is pinned: `-v` writes no color and runs no external diff (measured:
// diff.external set, and the block git wrote never called it), so those two
// are forced off; everything else — diff.context, diff.noprefix,
// diff.mnemonicPrefix, textconv (measured: `-v` honours it), rename
// detection — is read from the same config by both sides, because a pinned
// prefix would MISMATCH a writer whose config git itself honoured. No -U0
// and no --text: the sibling arm's shape is a different diff.
//
// AND EVERY WAY THE TWO CAN DISAGREE LANDS ON THE FAIL-CLOSED SIDE, which
// the guard's own defect showed to be the direction that must not be got
// wrong: a line git wrote that the reference does not carry is left on the
// scan, never taken off it. An empty reference — nothing staged, a base
// that does not resolve, `git diff` failing — matches nothing (measured,
// BSD and GNU grep both), so the whole block below the marker is read.
// status.renames set apart from diff.renames, `-vv` (git appends the
// UNSTAGED diff too, under i/ w/ prefixes, and the cached one under c/ i/),
// `-c HEAD` (read as an amend, diffed by git against HEAD), a root commit
// amended under `-v` (git diffs against the empty tree; HEAD^1 does not
// resolve and the arm falls back to that same tree): each leaves lines on
// the scan that git will throw away, which is the over-refusal xfgcn
// removed returning for that config and no other.
//
// THE SET DIFFERENCE IS grep's, because the hooks may call only what
// scripts/cleanroom.sh HOOK_DEPS names on every distro (no awk, no comm, no
// diff): the reference is the pattern list on stdin (-F, -x: whole lines,
// literally), "$1" is the subject, and the line numbers -n prints are what
// confine the subtraction to the block BELOW the cut — the writer's own
// lines above it are never subtracted, whatever they coincide with. The
// loop over those numbers runs inside the capture, so nothing is assigned
// on the right of a pipeline (the rangerhq-kk6e lesson, at the top of this
// hook).
//
// RESIDUALS, stated. Fail-CLOSED: `-v` as a FLAG on `-F`/`-m` — git
// truncates at a marker the writer typed (measured) and appends no diff,
// so the arm reads what git throws away; a core.commentChar of '-' or '+'
// hides git's cut line from the match (neither is one of commit.c's ten
// `auto` candidates); `--cleanup=scissors` as a flag is invisible like every
// other; a marker forged ABOVE git's own under `-v` — git truncates at the
// first (measured, wt_status_locate_end takes the first) and the lines
// between the two are read here, because none of them is a line of the
// staged diff; and the drift cases above. Fail-OPEN, and bounded: a line
// below a forged marker that IS a line of the staged diff is not scanned
// even where git keeps it — a context or removed line carrying a class,
// which is content already in a tree object of this repo, the same bound
// `--amend --no-edit` states above; an ADDED line there is the sibling
// arm's subject and is refused by it.
func messageArm(ind, head string, sources []visScanSource) string {
	i1, i2, i3 := ind+"  ", ind+"    ", ind+"      "
	var body strings.Builder
	for _, s := range sources {
		// Absent arm, absent refusal — twoArmScan's rule, read over the
		// third subject: the crew names are PATHS ONLY, so they say
		// nothing about a message that names the persona who wrote it
		// (ranger-base-cdxpf).
		if s.message.badVar == "" {
			continue
		}
		body.WriteString(i2 + "posse_bad=''\n" + s.checks(i2) + s.message.render(i2))
	}
	if body.Len() == 0 {
		return ""
	}
	return head + ind + `if [ -f "${1:-}" ]; then
` + i1 + `posse_clean=$(git config --get commit.cleanup 2>/dev/null) || posse_clean=''
` + i1 + `posse_kept=''
` + i1 + `posse_cut=$(grep -nE '^[^ +@\-][^ ]* ------------------------ >8 ------------------------$' "$1" 2>/dev/null | sed -n '1p' | cut -d: -f1)
` + i1 + `posse_rest=''
` + i1 + `if [ -n "$posse_cut" ] && [ "$posse_clean" != scissors ]; then
` + i2 + `posse_vbase=$posse_base
` + i2 + `if [ "${2:-}" = commit ] && [ "${3:-}" = HEAD ]; then
` + i3 + `posse_vbase=$(git rev-parse --verify -q 'HEAD^1' 2>/dev/null) || posse_vbase=$(git hash-object -t tree /dev/null 2>/dev/null)
` + i2 + `fi
` + i2 + `posse_rest=$(git diff --cached --no-color --no-ext-diff --no-relative "$posse_vbase" 2>/dev/null |
` + i3 + `grep -anvxFf - "$1" 2>/dev/null |
` + i3 + `while IFS= read -r posse_l; do
` + i3 + `  if [ "${posse_l%%:*}" -gt "$posse_cut" ]; then printf '%s\n' "${posse_l#*:}"; fi
` + i3 + `done)
` + i1 + `fi
` + i1 + `case "$posse_clean" in
` + i1 + `  strip) posse_clean=strip ;;
` + i1 + `  verbatim|whitespace|scissors)
` + i2 + `if [ "${2:-}" != "message" ]; then posse_kept=$posse_clean; fi
` + i2 + `posse_clean=whole ;;
` + i1 + `  *) if [ "${2:-}" = "message" ]; then posse_clean=whole; else posse_clean=strip; fi ;;
` + i1 + `esac
` + i1 + `if [ -n "$posse_cut" ]; then
` + i2 + `posse_msg=$(head -n "$((posse_cut - 1))" "$1" 2>/dev/null; printf '%s\n' "$posse_rest")
` + i1 + `else
` + i2 + `posse_msg=$(cat "$1" 2>/dev/null)
` + i1 + `fi
` + i1 + `if [ "$posse_clean" = whole ]; then
` + i2 + `posse_cc=whole
` + i1 + `else
` + i2 + `posse_cc=$(git config --get core.commentString 2>/dev/null) ||
` + i2 + `  posse_cc=$(git config --get core.commentChar 2>/dev/null) || posse_cc=''
` + i2 + `if [ "$posse_cc" != auto ]; then
` + i3 + `posse_cc=''
` + i2 + `else
` + i3 + `posse_cc=$(printf '%s\n' "$posse_msg" | grep -axE '[#;@!$%^&|:]' | sed -n '$p')
` + i3 + `[ -n "$posse_cc" ] &&
` + i3 + `  [ "$(printf '%s\n' "$posse_msg" | cut -c1 | grep -caxF "$posse_cc")" -ge 4 ] || posse_cc=whole
` + i2 + `fi
` + i1 + `fi
` + i1 + `if [ "$posse_cc" = whole ]; then
` + i2 + `posse_added=$posse_msg
` + i1 + `elif [ -n "$posse_cc" ]; then
` + i2 + `posse_added=$(printf '%s\n' "$posse_msg" | git -c core.commentChar="$posse_cc" stripspace --strip-comments 2>/dev/null)
` + i1 + `else
` + i2 + `posse_added=$(printf '%s\n' "$posse_msg" | git stripspace --strip-comments 2>/dev/null)
` + i1 + `fi
` + i1 + `if [ -n "$posse_added" ]; then
` + body.String() + i1 + `fi
` + ind + `fi
`
}

// ceilingMessageHead is the ceiling's message-arm banner (ADR 0050 D2 as
// amended 2026-09-03). Check 3's is checkThreeMessageHead below: the arm is
// one mechanism, and the two walls say different things about it because a
// writer who trips one is sent to a different remedy than a writer who
// trips the other.
var ceilingMessageHead = "\n" + shComment("", `─── the data ceiling, third arm: the commit MESSAGE (ADR 0050 D2) ──────
"$1" is the message file git is about to take — the same file the
shared-index arm below compares against MERGE_MSG. WHAT IS READ IS WHAT
GIT WILL KEEP, and git's CLEANUP MODE is what decides which bytes those
are, so the mode is what is asked for. Under "strip" git removes every
comment line — the "On branch" line, the status block listing staged,
unstaged and UNTRACKED paths, a merge's "# Conflicts:" list — so the read
is "git stripspace --strip-comments" and the scan judges only what will
survive into the object (ranger-base-h3s6q: a full-file read refused over
an untracked file's NAME, with a remedy no rewrite could clear). Under
"verbatim", "whitespace" or "scissors" git removes none of it, its own
template included, so the file is read WHOLE. With commit.cleanup unset
the mode is "default", which is "strip if the message is edited" — and
"$2" is the handle on that: "message" is -m/-F, where git appends no
template and KEEPS a '#'-leading line, so a pasted markdown heading lands
in the object and the read is whole; every other path is read stripped
(ranger-base-6y3z2: keying on "$2" alone let git's own template land
unscanned under commit.cleanup=verbatim, measured).
core.commentChar=auto is not a character but a choice git makes per
message, and stripspace answers '#' for it whatever git chose — so
wherever the read strips, the character is taken from the template git
already wrote — the LAST line of "$1" that is a bare comment character on
its own, which is git's block under "commit -v" and without it alike — and
handed to stripspace; where git appended no block it strips nothing and
the file is read whole (ranger-base-vzx2n, ranger-base-vl9g8).
The refusal's remedy differs from the staged file's — rewrite the message,
cite the id — because the text is not in a file the writer can edit; it is
still in .git/COMMIT_EDITMSG, local and unreplicated, until the next
commit overwrites it (measured). Where the mode is one of the three that
KEEP git's template the remedy says so and names it, because on that path
the hit can be in a block the writer never typed and no rewrite of their
own text clears (ranger-base-b21e0).
A message typed in the EDITOR is not scanned here and cannot be: this hook
runs before the editor opens, so the file holds git's template and
whatever was already in it, never the words about to be typed (measured,
git 2.50.1). That path is the operator's own hand, above the ceiling
already; the second layer for it is a commit-msg hook.`)

// identityGuardCheck renders check 3's block: this box's own identity
// literals (ADR 0024 D2) AND this instance's config patterns (ADR 0048 D2)
// against the ADDED LINES of every staged file, code included, AND
// against the ADDED staged PATHS — plus this box's crew names (ADR 0012 D2
// and App.A 5, ranger-base-cdxpf) against the ADDED staged PATHS ALONE, and
// only under the trees ADR 0012 D6 puts INSIDE App.A 5 (crewPathSkip,
// ranger-base-p7e0z). The crew names arrive in the same `identity` slice,
// flagged PathsOnly, and are partitioned out below. "" only when BOTH
// lists are empty — a box
// that derived nothing (no git email, no .beads/redirect, an unset $HOME)
// and configured nothing skips the block whole rather than paying for a
// full `git diff` that can never find a match. An empty identity with a
// non-empty pattern list still renders (ADR 0048 D2): the two sources are
// independent, and the derivation failing is not a reason to stand the
// operator's own patterns down.
//
// WHY THE PATTERNS ARE HERE AND NOT IN CHECK 2 (ADR 0048 D2). ADR 0024 D2
// kept check 2 off code because the SHIPPED list's own source and tests are
// byte-identical to hits, and a wall carrying an allowlist of its own files
// is a wall with a hole list. That argument is about the shipped list. A
// config pattern is never in source — it lives in the operator's config and
// in this rendered hook, both untracked — so it shares check 3's property,
// no legitimate public use anywhere, and belongs in check 3's scope. Five
// of the seven recurrences ADR 0048 measures were in .go files, which check
// 2 does not read.
//
// TWO SOURCES, ONE SCAN, TWO REFUSALS. Both lists run over the SAME
// $posse_added and the same per-path loop — one `git diff`, one listing —
// but each accumulates into its own variable and refuses in its own words.
// Merging them would be shorter and would tell a writer that an instance
// codename in a comment is "an operator identity literal", which is a
// refusal that names the wrong rule and sends them to the wrong remedy.
//
// TWO ARMS, ONE LITERAL SET (ranger-base-dmsbu, from ranger-base-wlsv1).
// "ADDED lines" was the mechanism, not the intent: a filename is exactly
// where an operator-shaped artifact puts the operator, and two shapes
// committed clean in a public-stamped repo with the content arm alone —
// a new docs/runbooks/<username>.md whose CONTENT is spotless, and a pure
// `git mv` of an already-clean file to <username>.txt, which yields no plus
// lines at all, +++ header included. The path analogue of an added LINE is
// an added ENTRY, which is check 1's rule for docs/ verbatim: a modified
// existing path cleared this the day it was added, and a deletion carries a
// path AWAY — refusing that is the wrong direction, and the lint is not a
// purge.
//
// checks() is rendered at two indents rather than once into a variable
// because the path arm needs it INSIDE a loop: posse_check accumulates the
// class and the matched text but not the subject, so the only way to name
// the offending path in the refusal — which is what distinguishes a path
// hit from a content hit for the reader — is to run the same matcher over
// one path at a time. Same rendered literal set, same posse_check, same
// override, same refusals.log shape, public-stamped repositories only.
func identityGuardCheck(identity []IdentityLiteral, extra []OpsPattern) string {
	if len(identity) == 0 && len(extra) == 0 {
		return ""
	}
	// THREE SOURCES OUT OF TWO ARGUMENTS. The derived slice carries two
	// kinds and the literal itself says which (IdentityLiteral.PathsOnly):
	// an operator identity literal has no legitimate public use in a line,
	// a path or a message alike, and a crew persona name is refused in a
	// PATH only (ranger-base-cdxpf). Partitioned here rather than at the
	// call sites so every renderer of this hook — the two installers, the
	// session-hooks redirect and the L3 probe — is handed ONE list and
	// cannot pass a different partition of it than the wall renders.
	var idLits, crew []IdentityLiteral
	for _, lit := range identity {
		if lit.PathsOnly {
			crew = append(crew, lit)
			continue
		}
		idLits = append(idLits, lit)
	}

	// Identity literals are regexp-ESCAPED fixed strings; an instance
	// pattern is already an ERE in the two-reader dialect (validateOpsERE).
	// Both end up as one posse_check call, which is why the same shell
	// function serves checks 0, 2 and 3.
	idChecks := func(indent string) string {
		var b strings.Builder
		for _, lit := range idLits {
			b.WriteString(opsCheckCall(indent, lit.Class, identityLiteralERE(lit.Value), false))
		}
		return b.String()
	}
	// Case-insensitive, boundary-free (crewLiteralERE), and NOT class-only:
	// the refusal names the persona and the path, because the reader is the
	// instance the name belongs to and the remedy is a rename they have to
	// be able to make.
	crewChecks := func(indent string) string {
		var b strings.Builder
		for _, lit := range crew {
			b.WriteString(opsCheckCall(indent, lit.Class, crewLiteralERE(lit.Value), false))
		}
		return b.String()
	}
	exChecks := func(indent string) string {
		var b strings.Builder
		for _, p := range extra {
			b.WriteString(opsCheckCall(indent, p.Class, p.ERE, true))
		}
		return b.String()
	}

	// The sources, in the order derived-then-configured — the order All()
	// uses and the order a hook file reads in. Each is skipped whole when
	// its list is empty.
	var sources []visScanSource
	if len(idLits) > 0 {
		sources = append(sources, visScanSource{
			checks:  idChecks,
			pathVar: "posse_ibad",
			content: visGuardRefusal{
				badVar:       "posse_bad",
				label:        identityScanLabel,
				logTail:      "(public repo)",
				overrideWhat: "an operator identifier is going into a public repo",
				header:       "an operator identity literal in a staged file",
				rule:         IdentityRule,
				matched:      stagedLineMatched,
				wayThrough:   IdentityWayThrough,
			},
			path: visGuardRefusal{
				badVar:       "posse_ibad",
				label:        identityScanLabel,
				logTail:      "(public repo, staged path)",
				overrideAt:   " (staged path)",
				overrideWhat: "an operator identifier is going into a public repo in a staged PATH",
				header:       "an operator identity literal in a staged PATH",
				rule:         IdentityRule,
				matched:      stagedPathMatched,
				wayThrough:   IdentityWayThrough,
			},
			message: visGuardRefusal{
				badVar:       "posse_bad",
				label:        identityScanLabel,
				logTail:      "(public repo, commit message)",
				overrideAt:   " (commit message)",
				overrideWhat: "an operator identifier is going into a public repo in the commit MESSAGE",
				header:       "an operator identity literal in the commit MESSAGE",
				rule:         IdentityRule,
				matched:      commitMessageMatched,
				wayThrough:   IdentityMessageWayThrough,
				keptModeVar:  "posse_kept",
			},
		})
	}
	if len(extra) > 0 {
		sources = append(sources, visScanSource{
			checks:  exChecks,
			pathVar: "posse_cbad",
			content: visGuardRefusal{
				badVar:       "posse_bad",
				label:        instanceScanLabel,
				logTail:      "(public repo)",
				overrideWhat: "instance-defined content is going into a public repo",
				header:       "an instance-defined visibility class in a staged file",
				rule:         OpsInstanceRule,
				matched:      stagedLineMatched,
				wayThrough:   OpsInstanceWayThrough,
			},
			path: visGuardRefusal{
				badVar:       "posse_cbad",
				label:        instanceScanLabel,
				logTail:      "(public repo, staged path)",
				overrideAt:   " (staged path)",
				overrideWhat: "instance-defined content is going into a public repo in a staged PATH",
				header:       "an instance-defined visibility class in a staged PATH",
				rule:         OpsInstanceRule,
				matched:      stagedPathMatched,
				wayThrough:   OpsInstanceWayThrough,
			},
			message: visGuardRefusal{
				badVar:       "posse_bad",
				label:        instanceScanLabel,
				logTail:      "(public repo, commit message)",
				overrideAt:   " (commit message)",
				overrideWhat: "instance-defined content is going into a public repo in the commit MESSAGE",
				header:       "an instance-defined visibility class in the commit MESSAGE",
				rule:         OpsInstanceRule,
				matched:      commitMessageMatched,
				wayThrough:   OpsInstanceMessageWayThrough,
				keptModeVar:  "posse_kept",
			},
		})
	}

	// The crew names, PATHS ONLY (ranger-base-cdxpf) and ONE TREE
	// (crewPathSkip, ranger-base-p7e0z). Third source, third set of words:
	// a persona name is not an operator identity literal and not an
	// instance-defined class, and a writer refused here is sent to ADR 0012
	// D2's remedy — name the file for the role — not to restate-and-cite.
	// It renders LAST of the three so a path tripping more than one is
	// refused in the operator's words first; those are the stricter rule
	// (an identity literal is refused in a line as well, and in every tree)
	// and this one is a rename.
	if len(crew) > 0 {
		sources = append(sources, visScanSource{
			checks:   crewChecks,
			pathVar:  "posse_nbad",
			pathSkip: crewPathSkip,
			path: visGuardRefusal{
				badVar:       "posse_nbad",
				label:        crewScanLabel,
				logTail:      "(public repo, staged path)",
				overrideAt:   " (staged path)",
				overrideWhat: "a crew persona name is going into a public repo in a staged PATH",
				header:       "a crew persona name in a staged PATH",
				rule:         CrewRule,
				matched:      stagedPathMatched,
				wayThrough:   CrewWayThrough,
			},
		})
	}
	head := "\n" + shComment("  ", `─── check 3: identity literals, instance patterns, crew names ──────────
(ADR 0024 D2 for the derived literals, ADR 0048 D2 for the config ones,
ADR 0012 D2 / App.A 5 for the crew names.)
The literals are derived from THIS box at render time — whoami, git
config user.email, and the instance repo path (dirname of
.beads/redirect's target, both ~-relative and absolute) — never a
shipped constant, never a commit: only this rendered hook file carries
them (identityGuardCheck's own caller, DeriveIdentityLiterals,
visibility.go). The instance patterns come from the operator's config
(`+OpsPatternsConfigKey+`:) and are untracked for the same reason.
The crew names are derived the same way and from the same box — the
PIDs in this home's agents/, less every name posse itself ships as an
example role (DeriveCrewLiterals) — and they are the one source that
does NOT scan all three subjects: PATHS ONLY, case-insensitively and
with no word boundary. A persona name in a staged LINE or in a commit
message is legitimate in the places ADR 0012 D2 leaves it (docs/, the
root narrative, a D6-grandfathered id, the message naming who wrote
it); a file NAME under a tree that ships as CODE has nothing to exempt
it, which is the shape that rode main for a day (ranger-base-o3g6a).
D6's edge is the TREE and not the syntax, so it bounds this arm's
subject as well as the other two: docs/ and the repo root's narrative
files are skipped here too (the filter is rendered inside the loop
below, ranger-base-p7e0z), because a path there is the development
record and refusing it would refuse what the constitution allows.
Rendered as regexp-escaped fixed strings and as EREs respectively, so
the SAME matcher checks 0 and 2 already call above covers this too.
THREE SUBJECTS for those two, in this order: the ADDED lines of ALL
staged files, any path, code included — unlike check 2, which is
markdown-only; then the ADDED staged PATHS; then every line of the
commit MESSAGE (ADR 0024 D2 / ADR 0048 D2 as amended 2026-09-03,
ranger-base-1nbtn). A source with nothing to say about a subject
renders no arm for it at all, which is why a hook whose only check-3
source is the crew names carries no content arm and no message arm —
what is not in the file is not a rule this box left out. Neither an
operator's identity nor one instance's confidential vocabulary has a
legitimate public use anywhere, so the detector-source residual check 2
accepts does not apply here — and a commit message is content that
replicates with the branch, not the commit METADATA the wall still does
not read (the author field is whatever user.email resolves to, and that
is the operator's to set).
The shipped OpsPatterns in check 2 above do NOT scan the message, and
that is a census rather than an oversight: over the 1136 messages then
on main the shipped list hit 29, 22 of them the software's own
vocabulary — fixture figures, blessed defaults, documented key values —
and a message has no shape table to disposition the residue by
(ranger-base-1nbtn, pinned by TestQAShippedPatternsDoNotScanTheCommit-
Message).`)
	return twoArmScan("  ", "check 3", head, sources) +
		messageArm("  ", checkThreeMessageHead, sources)
}

// crewPathSkip is the crew arm's TREE filter (ranger-base-p7e0z, fixing what
// ranger-base-cdxpf landed): the staged paths ADR 0012 D6 puts OUTSIDE
// App.A 5 are skipped before this source's checks run.
//
// THE DEFECT IT FIXES. cdxpf gave check 3 a crew source with no root filter
// at all, so the wall enforced App.A 5 past that rule's own stated edge and
// its refusal told the writer to rename a file the constitution lets stand.
// MEASURED on the box it was filed from: over 841 tracked paths the staffed
// PIDs hit exactly ONE — docs/adr/00NN-<seat>-pulse.md, tracked on main and
// standing — and ZERO under the trees the rule governs. Adding, renaming or
// re-adding that ADR was refused, with only the override as the way through.
//
// WHY A DENYLIST, where the shipped pin (qibShippedRoots) is an allowlist.
// The asymmetry is deliberate and it is about which way each one fails: a
// WALK must enumerate what to read, so a root it forgets is merely unread
// and a later pin can widen it; a WALL that enumerated what to refuse would
// leave a NEW top-level tree unguarded until somebody remembered to add it,
// which is the failure this whole check exists against. So this skips what
// D6 excludes by name and guards everything else — .github/, plugin/,
// scripts/, www/ and any tree added tomorrow included.
//
// THE `*/*` ARM IS LOAD-BEARING and sits BETWEEN the other two on purpose:
// without it `*.md` matches every markdown file in the tree, and what D6
// exempts is the repo ROOT's narrative, not markdown anywhere. RESIDUAL,
// stated: every root .md is exempt, not a fixed list of today's names — D6
// enumerates none, and "the root narrative files" is what a root .md is.
// `case` still matches under `set -f`, which disables pathname expansion
// and not pattern matching (measured).
func crewPathSkip(ind string) string {
	return shComment(ind, `─── the crew arm's tree filter (ADR 0012 D6) ───────────────────────────
App.A 5 reaches the trees that SHIP AS CODE: "every line cmd/, internal/,
and etc/ ship", plus examples/, which embed.go carries into the binary.
D6 names the other side in as many words -- "The edge is the tree, not
the syntax: docs/ and the root narrative files are the development
record, where the crew are historical actors and the no-mass-sweep
convention above governs them as it governs ids." A path under those is
outside this rule, and refusing it would send a writer to rename a file
the constitution lets stand.
A DENYLIST, unlike the shipped pin's allowlist: a new top-level tree is
guarded the day it appears rather than the day somebody remembers it.
The `+"`*/*`"+` arm sits between the other two so that `+"`*.md`"+` reaches the repo
ROOT only -- the exemption is the root narrative, not markdown anywhere.
THIS SOURCE ONLY: an operator identity literal or an instance class in
the same path is refused exactly as before.`) +
		ind + `posse_skip=''
` + ind + `case $posse_ip in
` + ind + `  docs/*) posse_skip=1 ;;
` + ind + `  */*) ;;
` + ind + `  *.md) posse_skip=1 ;;
` + ind + `esac
`
}

// checkThreeMessageHead is check 3's message-arm banner (ADR 0024 D2 check
// 3 and ADR 0048 D2, both as amended 2026-09-03, ranger-base-1nbtn). The
// mechanism is messageArm's and is shared with the ceiling; what differs is
// what a writer is told, and check 3 has two things to tell them the
// ceiling does not: this arm is INSIDE the visibility gate, and the
// ceiling's message arm has already run and refuses first.
var checkThreeMessageHead = "\n" + shComment("  ", `─── check 3, third arm: the commit MESSAGE ─────────────────────────────
"$1" is the message file git is about to take — the same file the ceiling
above and the shared-index arm below already read, through the same
reader. WHAT IS READ IS WHAT GIT WILL KEEP, and git's CLEANUP MODE
decides which bytes those are. Under "strip" git removes every comment
line — the "On branch" line, the status block, a merge's "# Conflicts:"
list — so the read is "git stripspace --strip-comments" and the scan
judges only what will survive into the object (ranger-base-h3s6q). Under
"verbatim", "whitespace" or "scissors" git removes none of it and the
file is read WHOLE (ranger-base-6y3z2: under commit.cleanup=verbatim
git kept its own template, an UNTRACKED file's NAME included, in a PUBLIC
repo's commit object while this arm stripped exactly those lines out of
the scan). With commit.cleanup unset the mode is "default" and "$2" is
the handle: "message" is -m/-F, where git appends no template and keeps a
'#'-leading line, so a pasted heading lands and the read is whole.
Wherever the read strips, the comment character is taken from the
template git already wrote if core.commentChar is "auto", because for
that one value stripspace answers '#' whatever git chose — the last bare
comment-character line of "$1", which finds git's block under "commit -v"
too, where the file ends in the diff (ranger-base-vzx2n,
ranger-base-vl9g8). Inside the gate with the rest of
check 3: an identity literal or an instance class is about where content
may GO, so a private-stamped repo runs none of this — which is the one
thing that separates this arm from the ceiling's, and why the ceiling's
runs first and refuses with the stricter remedy when both would speak.
The remedy differs from the staged file's — rewrite the message — because
the text is not in a file the writer can edit; it is still in
.git/COMMIT_EDITMSG, local and unreplicated, until the next commit
overwrites it (measured). Where the mode KEEPS git's template the remedy
names the mode and what clears it, because the hit can be in a block the
writer never typed (ranger-base-b21e0).
A message typed in the EDITOR is not scanned here and cannot be: this
hook runs before the editor opens, so the file holds git's template and
whatever was already in it, never the words about to be typed (measured,
git 2.50.1). That path is the operator's own hand; the second layer for
it is a commit-msg hook.`)

// pathAccum is how one path's hits are folded into an arm's accumulator:
// posse_check keeps the class and the matched text but not the subject, so
// the path is prepended here, once per source. ind is the loop body's
// indent.
func pathAccum(ind, bad string) string {
	return ind + `if [ -n "$posse_bad" ]; then
` + ind + `  ` + bad + `="$` + bad + `  $posse_ip
$posse_bad"
` + ind + `fi
`
}

// ─── L3: the constitution-path arm of the commit guard ───────────────────────

// constitutionGuardBody renders the commit wall's THIRD arm (ranger-base-ak3e,
// fixing ranger-base-7pq0): a persona-marked commit whose to-be-committed set
// touches the constitution class is refused.
//
// THE HOLE IT CLOSES, measured live. 9dfbbd4 in the constitution repo edited
// all eleven `rhq/agents/*.md` crew PIDs from an uncaged persona session and
// nothing refused it. Every wall that exists checks something else: the PID
// deny list fences COMMANDS (`Bash(posse promote:*)`), the shared-index arm
// below checks the commit's FORM, the land path checks bead and branch state,
// and the only path-CLASS check in the tree is `ConstitutionGrants`, which is
// seatbelt's and so is EPERM under `cage: seatbelt` and prose everywhere else.
// Seven of eight personas run at shims (ADR 0002 §3). Under shims there was no
// path-class check anywhere, which is ADR 0015 §2's "drafting is open to
// personas, promotion is the operator's" holding for the taking-effect path
// and not for the commit a promote then reads.
//
// WHY THIS SLOT. Same reasons the shared-index arm names: prepare-commit-msg
// is posse-owned (pre-commit belongs to beads, which reinstalls it silently —
// ADR 0002 §3), it is already installed in the constitution repo, it is
// refreshed at every launch, and it survives `git commit --no-verify`, which
// pre-commit does not.
//
// WHY IT KEYS ON RHQ_PERSONA and the shared-index arm does not. That arm is
// about the TREE — a shared index sweeps whoever types into it, the operator
// included (rangerhq-lt2w). This one is about WHO: ADR 0015 §2/§3 splits
// drafting from ratification by actor, and the operator IS the ratifying
// actor. The marker is the same one `posse promote` already refuses on
// (EnvPersona, herdrback.go) and the same one MarkCrewOnOperatorPrompt reads
// to tell an operator shell from a persona's, so no third spelling of "is
// this a persona" is being coined here.
//
// THE TO-BE-COMMITTED SET is `git diff --cached --name-only --no-renames`
// under the hook's
// INHERITED GIT_INDEX_FILE, which is what makes it right for both forms this
// wall leaves standing: a path-limited commit's temporary next-index IS the
// index git hands the hook, so what that diff reports is what the commit will
// take — not what the shared index happens to hold. `--no-renames` because
// with move detection on --name-only prints only a rename's destination, so
// a move OUT of the class was invisible here and landed (ranger-base-qdxe);
// the land belt already read its own diff that way. `-z`, so a path with a
// quote or a space in it is not read through git's own path quoting, and the
// base is the empty tree in a repo with no commit yet (a first commit is
// exactly when a constitution arrives whole). `diff.relative` is pinned off:
// git runs hooks from the top level, but that config exists and a
// repo-relative class has to be compared against repo-relative paths.
//
// THE CLASS is two lists, because the two have different reasons:
//
//	(a) in the CONSTITUTION repo only — a repo whose top level has
//	    ConstitutionRepoMarker ON DISK **or** in the base tree the commit is
//	    diffed against (ranger-base-jex3: a worktree-only detector is one the
//	    persona owns, and `rm -rf` on the marker dir — never staged, so
//	    nothing on the branch records it — stood the whole promoted set
//	    down) — every ConstitutionRepoPaths entry: `posse/<p>` for each
//	    PromotedPaths entry, plus `posse/envs`. This is the
//	    prose that becomes law at the next `posse promote`, and it is
//	    rendered FROM PromotedPaths at hook-render time so a path added to
//	    the promoted set widens this wall in the same edit rather than in a
//	    later bead.
//	(b) in EVERY hooked repo — `.claude/settings.json` and its `.local`
//	    sibling. ranger-base-az93 is why: the deny list that fences a
//	    persona's destructive verbs lives in that file, so a persona that can
//	    commit it un-fences itself, in any repo it is dispatched into. It is
//	    not "the constitution" by ADR 0015's list; it is the same class of
//	    thing by blast radius, which is what a wall is allowed to care about.
//
// A class member matches a staged path exactly or as a directory prefix, so
// one rule covers `posse/config.yaml` (a file) and `posse/agents` (a tree).
//
// WHAT IT DOES NOT DO: it does not reset, unstage or otherwise touch the
// tree. The shared-index arm below already says why in its own words — a hook
// that cleaned up behind a persona would be the destructive act the wall
// exists to prevent — and here the staged diff is the very thing the
// prescribed route asks the persona to keep hold of.
//
// THE RESIDUAL, stated rather than discovered. This is L3, the shim tier:
// `env -i` scrubs RHQ_PERSONA and the arm stands down, which is the exact
// residual PrePushHook already documents for its own marker; `core.hooksPath`
// re-points the slot, the same tier of residual. A THIRD one was on this list
// and has been closed rather than documented: removing the marker directory
// from the session worktree before a path-limited commit stood the promoted
// half of the class down (ranger-base-jex3, measured; the belt compensated,
// which is why it was P2 and not P1). The marker is now read from the
// worktree OR the base tree, so the route needs a marker removal the base
// tree records — which is itself a class path, and refused. Two things sit
// behind it and neither is claimed to be this: under `cage: seatbelt` the
// constitution area is not writable at all (ConstitutionGrants, measured
// empty), and the promote manifest is the DETECTOR at every tier — a
// constitution that changed without a promotion is caught at the next launch
// verify, whoever changed it and however. A pre-push arm would add nothing:
// personas cannot push (PrePushHook, and every crew PID denies the verb) and
// a push is operator-lane. A nested `.claude/settings.json` —
// `sub/.claude/…` rather than the repo root's — is outside the class as
// written, and outside what any runtime reads for a session dispatched at a
// repo root.
func constitutionGuardBody() string {
	// One member per line, and a leading newline so EVERY member is a whole
	// line of the rendered hook whichever block it lands in — that is what
	// TestQAConstitutionWallRenderNamesEveryPromotedPath reads, and a member
	// welded onto the end of a shell assignment would slip past it. Blank
	// lines cost nothing: IFS is a newline, which is IFS whitespace, so the
	// splitting below collapses runs of them.
	class := func(paths []string) string {
		var b strings.Builder
		for _, p := range paths {
			b.WriteString("\n")
			b.WriteString(p)
		}
		b.WriteString("\n")
		return b.String()
	}
	return `
# ─── the constitution-path guard (ranger-base-ak3e) ───────────────────────
# ADR 0015 §2/§3: personas DRAFT the constitution, the operator puts it in
# force. Every other wall in this system checks a command, a commit's form or
# a bead's state; this one checks the PATH CLASS, which is what 9dfbbd4 walked
# through (ranger-base-7pq0). It runs ABOVE the shared-index arm on purpose:
# that arm stands down in a linked worktree and mid-merge, and a dispatched
# worktree is exactly where a persona's constitution commit comes from.
if [ -n "${` + EnvPersona + `:-}" ]; then
  # Every hooked repo: the file that carries this session's own deny list
  # (ranger-base-az93). Then, in the constitution repo only, the promoted set.
  posse_cls='` + class([]string{ClaudeProjectConfig, ClaudeProjectConfigLocal}) + `'
  posse_cls_top=$(git rev-parse --show-toplevel 2>/dev/null)
  # The index the COMMIT will use, which is the one git handed this hook —
  # a path-limited commit's own next-index included. Empty tree when there is
  # no HEAD yet. Read BEFORE the marker test below, which needs it.
  posse_cls_base=$(git hash-object -t tree /dev/null 2>/dev/null)
  if git rev-parse --verify -q HEAD >/dev/null 2>&1; then posse_cls_base=HEAD; fi
  # THE MARKER IS NOT THE PERSONA'S TO REMOVE (ranger-base-jex3). The
  # worktree test alone was a detector a persona owns: ` + "`rm -rf " + ConstitutionRepoMarker + "`" + `
  # in the session worktree — never staged, so nothing on the branch records
  # it — dropped the whole promoted set out of the class and the identical
  # refused commit landed (measured, control and attack). The base tree still
  # carries the marker, so the OR closes it while keeping BOTH directions the
  # worktree arm was there for: repo genesis (no HEAD, but the dir is on
  # disk) and the deliberate "an unrelated repo can be made to answer this,
  # and is then refused there". cat-file -t rather than -e so the base arm
  # says exactly what [ -d ] says — a BLOB at that path is not a
  # constitution, the same way replacing the dir with a file was not.
  if [ -n "$posse_cls_top" ] && { [ -d "$posse_cls_top/` + ConstitutionRepoMarker + `" ] ||
      [ "$(git cat-file -t "$posse_cls_base:` + ConstitutionRepoMarker + `" 2>/dev/null)" = tree ]; }; then
    posse_cls="${posse_cls}` + class(ConstitutionRepoPaths()) + `"
  fi
  # --no-renames (ranger-base-qdxe), the flag the land belt (worktree.go) and
  # the two arms below already carry and this one did not: rename detection
  # is ON by default and --name-only prints only a rename's DESTINATION, so
  # ` + "`git mv " + ConstitutionRepoMarker + "/x.md notes.md`" + ` showed this arm nothing in the
  # class and committed at exit 0 — the PID gone from the constitution repo,
  # and ` + "`git mv .claude/settings.json settings-draft.json`" + ` the same in EVERY
  # hooked repo. It needs no git mv: detection pairs any staged delete with a
  # similar staged add, so copy-then-remove did it too. A persona that can
  # delete the file un-fences itself as surely as one that can edit it
  # (ranger-base-az93). With the flag git reports the removal and the add
  # separately and the class member is a staged path again.
  posse_cls_staged=$(git -c diff.relative=false diff --cached --name-only --no-renames -z "$posse_cls_base" 2>/dev/null | tr '\0' '\n')
  posse_cls_hit=''
  if [ -n "$posse_cls_staged" ]; then
    # -f: both loops split an unquoted expansion, and a staged path with a
    # glob character in it has to stay a path rather than become a pattern.
    # IFS is a newline only, so a path with spaces in it stays one word.
    set -f
    posse_cls_ifs=$IFS
    IFS='
'
    for posse_cls_p in $posse_cls_staged; do
      for posse_cls_m in $posse_cls; do
        case "$posse_cls_p" in
          "$posse_cls_m"|"$posse_cls_m"/*)
            posse_cls_hit="$posse_cls_hit    $posse_cls_p    (class: $posse_cls_m)
" ;;
        esac
      done
    done
    IFS=$posse_cls_ifs
    set +f
  fi
  if [ -n "$posse_cls_hit" ]; then
    {
      echo "refused by posse gate: a persona commit touching the constitution — prepare-commit-msg hook, session ${` + EnvPersona + `}"
      echo "ADR 0015 §2/§3: a persona DRAFTS the constitution; the operator puts it"
      echo "in force (posse promote). These paths are the law this session runs"
      echo "under — the ` + PromotedProse("and") + ` a promote copies into the home,"
      echo "the env files beside them, and the settings file carrying this session's"
      echo "own deny list (ranger-base-az93: a persona that can commit that file"
      echo "un-fences itself)."
      echo "staged now, and in the class:"
      printf '%s' "$posse_cls_hit"
      echo "the way through — stage the intended diff, the operator applies it:"
      echo "  write what you MEAN under a path OUTSIDE the class (az93's went to"
      echo "  docs/notes.d/az93-settings.json — an allowlisted genre, ADR 0024 D2"
      echo "  check 1: docs/rca/ has no public home), commit that path, and say on"
      echo "  the bead which class path it replaces and why. The operator reviews"
      echo "  it, puts it in place, and — for the constitution — runs posse promote,"
      echo "  which is the step that makes prose law (ADR 0015 §3)."
      echo "Nothing here has been reset, unstaged or cleaned up: this hook does not"
      echo "touch your tree, and what you staged is still exactly where you put it."
    } >&2
    if [ -n "$RHQ_GATES_DIR" ]; then
      echo "$(posse_stamp) constitution path in a persona commit [prepare-commit-msg hook] (${` + EnvPersona + `})" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
    fi
    exit 1
  fi
fi
`
}

// ─── the ADR sha-stamp guard (ADR 0051 D4/D5, ranger-base-glewr) ─────────────

// AdrPathspec is WHICH FILES the sha-stamp arm owns, as a git pathspec, in
// one place for the reason MarkdownPathspecs is (ranger-base-4b1z4): the
// wall's scope is a decision, and a decision spelled inline in the rendered
// hook is one nothing can measure or widen. `:(icase)` because git pathspec
// matching is case-sensitive and `docs/ADR/` is a directory a private repo
// can hold — the same one character that walked ops content into a public
// tree the last time this hook trusted a literal.
const AdrPathspec = ":(icase)docs/adr/*"

// AdrShaRule is the rule the refusal names — the teaching, not the verdict.
// A sentence in a record does not reach a writer at the moment they paste a
// sha; a refusal does.
const AdrShaRule = `ADR 0051 D1/D2: the citation of record for a landed change is the BEAD ID.
The launcher lands a session tree with merge --ff-only and REBASES first when
the base branch has moved, which mints a new sha — measured, 48 of 134
landings. So the only sha a persona can copy is one that 36% of the time names
an object on no ref, prunable by gc, and an ancestor of nothing. A bead id
survives the rebase: git log --grep <id> answers on every clone, forever.`

// AdrShaWayThrough is the remedy, and there are exactly two — the id, or the
// twin (ADR 0051 D5). The second is what a record whose SUBJECT is a stale
// sha uses; it cannot be typed by a build close, because a sha minted in a
// session tree has no twin on the base branch until the launcher lands it.
const AdrShaWayThrough = `the way through, either one: cite the bead id — "landed (ranger-base-xxxxx)",
with the commit subject beside it when several commits carry the id — or, if
this record's SUBJECT is the stale sha itself (a census, an incident writeup,
a table of what went stale), put the landed twin beside it in the same file
and the check admits the pair (ADR 0051 D5). A twin is a commit that IS on the
base branch and carries the same patch-id; a sha your own session tree just
minted has none until the launcher lands it, which is why a build close can
only ever take the first route.`

// adrShaPredicate is THE predicate ADR 0051 D4 specifies as amended by D5
// (ranger-base-mlfie, 2026-09-03) — one shell text, rendered into TWO
// places from this one function: the prepare-commit-msg hook's sha-stamp arm
// (adrShaGuardBody) and `posse gates adr-census` (AdrCensusScript). D4 says
// why there is one and not two: the hook and the census must not disagree
// about what is exempt, and a second copy of the rule — in prose, in a
// script — is the thing the amendment exists to prevent. The reference
// script under scripts/ was that second copy and retired when this landed
// (ranger-base-gyrko).
//
// ONE PREDICATE, TWO LINE SOURCES. The caller defines two shell functions
// before this text runs, and they are the WHOLE difference between the modes:
//
//	posse_adr_judged FILE   the lines being judged — the hook: the ADDED lines
//	                        of the staged file; the census: every line of it
//	posse_adr_record FILE   the record a twin may sit in — the hook: the whole
//	                        staged blob; the census: the same file
//
// and sets posse_adr_files (newline-separated). posse_adr_judge then reads
// the base, classifies every token, and leaves three newline-separated
// verdict lists for the caller to render as a refusal or as a census:
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
// shell hook. The radius carries no safety anyway: the twin's NONEXISTENCE
// before landing is what carries it.
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
// NO ESCAPES for a blockquote or a fenced block, and no override env either
// (ADR 0051 Alternatives, and mlfie's ruling, which priced an explicit
// marker and rejected it: free to type, and it teaches the override). D5 IS
// the way through for the one record that needs one, and it is a way through
// nothing minted in a session tree can take.
//
// COST, MEASURED: cat-file plus merge-base is ~50 ms per token; the patch-id
// pair is ~60 ms and is computed only for a file that holds BOTH a
// non-ancestor and an ancestor, so the common case pays nothing extra. The
// record scan runs only for a file with a non-ancestor, for the same reason.
//
// Every command here is in scripts/cleanroom.sh's HOOK_DEPS (git, grep, sort,
// cut) — the hook is what the clean-room probe walks, and this text is in it.
func adrShaPredicate() string {
	return `
# ─── the ADR sha-stamp predicate (ADR 0051 D4/D5) — ONE TEXT, TWO LINE SOURCES
# This block is rendered from one Go function into the prepare-commit-msg
# hook AND into posse gates adr-census, so the gate and the census cannot
# disagree about what is exempt. The caller defined posse_adr_judged FILE
# (the lines judged) and posse_adr_record FILE (the record a twin may sit
# in) and set posse_adr_files; posse_adr_judge leaves posse_adr_base (empty:
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
  #
  # Two statements and not one nested command substitution: the HOOK_DEPS
  # scanner (hookdeps_qa_test.go) reads a command substitution inside an
  # argument as closing the command it sits in, and named symbolic-ref as a
  # PATH command the clean room must probe for. The shape below says the same
  # thing to sh and the truth to the scanner. Filed as ranger-base-8lfbn;
  # fold these two back into one when that lands.
  posse_adr_common=$(git rev-parse --git-common-dir 2>/dev/null)
  posse_adr_base=$(git --git-dir="$posse_adr_common" symbolic-ref -q HEAD 2>/dev/null)
  if [ -z "$posse_adr_base" ]; then
    echo "posse gate: ADR sha-stamp check judged nothing — the main checkout's HEAD is detached, so this check has no base branch to measure ancestry against (ADR 0051 D4). Cite the bead id." >&2
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
    # commit to git and prose to a lowercase-only class — in the hook and
    # the census alike, which is why the two-way pin could not see it
    # (ranger-base-0fz98). The token keeps its own spelling: the census
    # greps the file for it.
    #
    # grep -a on BOTH token sources: one NUL byte in a record makes the
    # stream binary, grep answers "Binary file (standard input) matches"
    # instead of the tokens, and this arm judged nothing — in the hook AND
    # in the census, which cat's the whole file (ranger-base-h137b). The
    # hook's reader carries --text for the same reason, one layer up.
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

// diffReaderShape pins the shape of every `git diff --cached -U0` the hook
// reads ADDED lines out of, against the WRITER's git config — which the
// hook inherits, and which `git -c` reaches it through (GIT_CONFIG_PARAMETERS
// is exported to prepare-commit-msg, measured). Four ordinary settings each
// blank a reader that greps for '+' lines and '+++ b/' headers, and every
// one of them SILENTLY: the reader sees no files and judges nothing, with no
// "judged nothing" line (ranger-base-0fz98, git 2.50.1):
//
//	diff.noprefix=true         the header is "+++ docs/adr/x.md"
//	diff.mnemonicPrefix=true   the header is "+++ i/docs/adr/x.md"
//	color.ui=always            every line opens with an escape sequence
//	diff.external=CMD          no unified diff at all
//
// The prefix half is load-bearing for the ADR sha-stamp arm alone (its file
// list is cut out of the '+++ b/' headers); the colour and external halves
// are load-bearing for every reader — check 0, check 2, check 3 and the ADR
// arm. One string for all of them, so the readers cannot drift on which
// settings they survive. Rendered as flags rather than `-c` overrides so a
// rendered command still reads as the one the comment beside it describes.
//
// --text IS THE FIFTH, AND IT IS NOT A CONFIG SETTING (ranger-base-h137b,
// measured): git classifies a file BINARY on its own bytes, and prints
// "Binary files a/x and b/x differ" for it — never a '+' line, never a
// '+++ b/' header. One NUL byte in a markdown file is enough, and appending
// captured terminal output to a NOTES or RCA file is how it gets there; no
// config, no .gitattributes, no intent required. Every reader then judged
// nothing and said nothing, with git's own summary reading "0 insertions(+)".
// The same blanking is reachable from the writer's config alone —
// core.attributesFile naming a file that says `*.md -diff` — which the four
// flags above do NOT close and --text does.
//
// --no-textconv IS THE SIXTH, AND IT IS THE ONE WITH NO TELL AT ALL
// (ranger-base-h3s6q finding 1, measured). git applies a
// diff.<driver>.textconv driver BEFORE the diff and --no-ext-diff does not
// cover it, so the reader is handed the CONVERTED text: a driver that drops
// the classed line leaves a diff that looks perfectly ordinary. It is
// reachable from the WRITER's config alone — core.attributesFile naming a
// file that says `* diff=redact`, plus diff.redact.textconv — with nothing
// staged, no .gitattributes in the tree and no intent required. Unlike the
// --text case there is not even a "0 insertions(+)" tell: git's own commit
// summary is computed WITHOUT textconv, so it reports the real line count
// while every '^+' reader judged text the driver had already emptied.
// Measured on this box: the data ceiling committed a classed line clean.
// This shop had closed the same class once already on the other reader —
// ranger-base-r5wpk put --no-textconv on memoryDiff (memoryland.go) for
// exactly these routes; the wall's reader did not get the same list.
//
// NOT --no-relative, and that is measured rather than assumed
// (ranger-base-h3s6q): git runs this hook with the working directory at the
// TOP of the worktree even when the commit is typed from a subdirectory, so
// diff.relative changes nothing any reader here sees. memoryDiff carries it
// because that arm runs against the memory dir, which is a different
// question.
//
// STATED RESIDUAL: --text also hands a genuine blob's bytes to the pattern
// scan, so a real binary can in principle match an ERE and be refused, and
// every staged byte now flows through $posse_added and one grep per class
// per arm. That is the fail-safe direction: the alternative is deciding
// which text-shaped files are "really" prose, which is the guess that put
// this hole here.
//
// IT IS NOT THE CHEAP ONE, and this comment said it was. MEASURED
// (ranger-base-h3s6q finding 3; git 2.50.1, darwin/arm64, one configured
// class, through the real rendered hook): ~0.55 s/MB of staged blob, linear
// — 1 MB 0.81s, 5 MB 2.91s, 10 MB 5.61s, 20 MB 10.6s — against 0.38s for the
// same 20 MB commit with --text off the content readers, a factor of 28. A
// three-line markdown commit in the same repo is 0.27s, so that is the floor
// it is added to. The diff is not where the time goes: the reader itself is
// 0.12s on that blob, and the cost is the $(...) capture of the whole thing
// into $posse_added and the `printf | grep -cE` per class in posse_check
// (~2947), so it scales with the CLASS COUNT as well as the file size.
// ACCEPTED with the number written down rather than capped: a size cap is a
// mechanism written down as a rule, and that is exactly how this hole got
// here the first time. Who pays is a hooked repo that commits assets; posse
// itself has none. If a cap is ever wanted it needs its own bead — and an
// elapsed-seconds assertion belongs to the box, which is the charter
// scripts/test-times.sh already argues (Makefile, verify-test-times).
//
// --text ALONE ONLY MOVES THE SILENCE. With a NUL in the stream grep prints
// "Binary file (standard input) matches" and the scan downstream matches
// nothing, so every '^+' reader below greps with -a. Measured both ways on
// this box; the pins are in binaryreader_qa_test.go.
const diffReaderShape = "--no-color --no-ext-diff --no-textconv --src-prefix=a/ --dst-prefix=b/ --text"

// adrShaGuardBody renders the hook's arm: adrShaPredicate with the hook's
// two line sources — the ADDED lines of each staged docs/adr file are judged,
// the whole staged blob is the record — and the refusal that names the
// token, because the token is the remedy.
//
// A SIBLING OF THE VISIBILITY GUARD'S CHECK 2, NOT A MEMBER OF IT. It shares
// that check's reader shape — the ADDED lines of a staged pathspec, -U0,
// rename detection left ON so a pure move does not re-present a whole file
// as additions — and nothing else. Its scope is narrower (docs/adr only),
// its predicate is git object identity rather than a regex, it is not gated
// on the repo's visibility mark (a stale stamp is wrong in a private tree
// too), and it PRINTS what it matched, which posse_check exists to avoid
// doing for an instance pattern. Folding it into posse_check would have made
// it a regex saying no.
//
// TWO LINE SOURCES, ONE PREDICATE (D4 as amended). The non-ancestors come
// from the ADDED lines — this arm judges what the commit is writing, not
// what the file already said. The candidate twins come from the WHOLE STAGED
// BLOB (`git show :<path>`), because the twin is usually already in the
// record: re-flowing one row of a stale→landed table adds the stale sha and
// adds nothing else. `posse gates adr-census` is the same predicate text with
// every line of the working-tree file as both sources, and the two are
// pinned to agree fixture for fixture (TestQAAdrShaStampAgreesWithTheCensus).
func adrShaGuardBody() string {
	return `
# ─── the ADR sha-stamp guard (ADR 0051 D4/D5, ranger-base-glewr) ──────────
# A record that stamps a landed sha can only ever name the writer's own
# session tree, and the launcher rebases a third of those trees before it
# lands them — measured, 48 of 134 — which mints a new sha and leaves the
# record naming an object on no ref. 12 of 32 shas in this repo's own
# docs/adr were unreachable when this was written. The citation of record is
# the bead id; a sha is a MEASUREMENT against the base branch, or it sits
# beside its landed twin in a record whose subject IS the staleness (D5).
# Unkeyed and ungated: a stale stamp is wrong whoever typed it and whatever
# the repo's visibility mark says.
#
# Same reader as the visibility guard's check 2 — ADDED lines, -U0, rename
# detection left ON so a pure move of an ADR does not re-present every line
# in it as an addition — pointed at this arm's own, narrower scope. The file
# list comes out of that one reader's own +++ headers rather than a second
# --name-only pass, which would need --no-renames and would then re-judge
# every line of a moved record.
posse_adr_head=$(git hash-object -t tree /dev/null 2>/dev/null)
if git rev-parse --verify -q HEAD >/dev/null 2>&1; then posse_adr_head=HEAD; fi
# The hook's two line sources (the census defines the same two over whole
# files): the lines this commit is WRITING are judged, and the whole staged
# blob is the record a twin may sit in.
posse_adr_judged() {
  git diff --cached -U0 ` + diffReaderShape + ` "$posse_adr_head" -- "$1" 2>/dev/null | grep -a '^+' | grep -av '^+++'
}
posse_adr_record() {
  git show ":$1" 2>/dev/null
}
` + adrShaPredicate() + `
posse_adr_files=$(git diff --cached -U0 ` + diffReaderShape + ` "$posse_adr_head" -- ` + shQuote(AdrPathspec) + ` 2>/dev/null |
  grep -a '^+++ b/' | cut -c7- | sort -u)
if [ -n "$posse_adr_files" ]; then
  posse_adr_judge
  if [ -n "$posse_adr_refused" ]; then
    posse_adr_bad=''
    set -f
    posse_adr_ifs=$IFS
    IFS='
'
    for posse_adr_l in $posse_adr_refused; do
      posse_adr_bad="$posse_adr_bad  ${posse_adr_l%% *}    in ${posse_adr_l#* }
"
    done
    IFS=$posse_adr_ifs
    set +f
    {
      echo "refused by posse gate: a staged docs/adr line names a sha that is not on $posse_adr_base and has no landed twin in the record — prepare-commit-msg hook, session ${RHQ_PERSONA:-?}"
      echo ` + shQuote(AdrShaRule) + `
      echo "added under docs/adr, resolves in this clone, NOT an ancestor of $posse_adr_base:"
      printf '%s' "$posse_adr_bad"
      echo ` + shQuote(AdrShaWayThrough) + `
      echo "the same predicate over whole files, to check a record before you commit it:"
      echo "  posse gates adr-census <file>"
      echo "Nothing here has been reset, unstaged or cleaned up: this hook does not"
      echo "touch your tree, and what you staged is still exactly where you put it."
    } >&2
    if [ -n "$RHQ_GATES_DIR" ]; then
      echo "$(posse_stamp) ADR sha-stamp guard [prepare-commit-msg hook] (base $posse_adr_base)" >> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null
    fi
    exit 1
  fi
fi
`
}

// AdrCensusScript is `posse gates adr-census`: the hook's own predicate text
// (adrShaPredicate, byte for byte) run over every line of every file it is
// handed, as both line sources. It is ADR 0051 D3's verify and D4's census —
// "one predicate, two line sources" — and it replaced scripts/adr-sha-census.sh,
// which was a second copy of the rule (ranger-base-gyrko).
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
# posse gates adr-census — ADR 0051 D3's verify, rendered from the SAME Go
# function as the prepare-commit-msg hook's sha-stamp arm (ranger-base-gyrko).
# Both line sources are the whole file: every line is judged and every line
# may hold a twin.
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
  echo "ADMITTED ${posse_adr_l#* } $posse_adr_t twin ${posse_adr_l%% *}"
done
for posse_adr_l in $posse_adr_refused; do
  posse_adr_t=${posse_adr_l%% *}
  posse_adr_f=${posse_adr_l#* }
  posse_adr_lines=$(grep -anE "\b$posse_adr_t\b" "$posse_adr_f" | cut -d: -f1 | tr '\n' ',' | sed 's/,$//')
  echo "REFUSE $posse_adr_f:$posse_adr_lines $posse_adr_t resolves here but is not on $posse_adr_branch and no landed twin is in the record — cite the bead id (git log --grep), or put the twin beside it (ADR 0051 D2/D5)"
done
posse_adr_j=$(printf '%s' "$posse_adr_ancestors" | grep -c .)
posse_adr_a=$(printf '%s' "$posse_adr_admitted" | grep -c .)
posse_adr_r=$(printf '%s' "$posse_adr_refused" | grep -c .)
echo "posse gates adr-census: base $posse_adr_branch judged $((posse_adr_j+posse_adr_a+posse_adr_r)) distinct tokens: $posse_adr_j ancestors, $posse_adr_a admitted by twin, $posse_adr_r refused"
[ "$posse_adr_r" -eq 0 ]
`
}

// AdrCensusDefault is what the census walks when handed no files: the
// records under the repo root, resolved from dir's toplevel.
const AdrCensusDefault = "docs/adr/*.md"

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
		matches, gerr := filepath.Glob(filepath.Join(dir, filepath.FromSlash(AdrCensusDefault)))
		if gerr != nil {
			return false, gerr
		}
		if len(matches) == 0 {
			return false, fmt.Errorf("adr-census: no %s under %s — nothing to judge", AdrCensusDefault, dir)
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

// CommitGuardHook is the whole prepare-commit-msg hook for a repo whose
// beads db carries the given visibility. Three arms, in this order and for
// this reason:
//
//	visibility     a mis-routed bead is a public artifact whoever typed it
//	constitution   a persona commit that would rewrite the law it runs under
//	shared index   a commit form that takes another persona's staged work
//
// The first and last apply to the operator's own commits too — the last one
// since rangerhq-lt2w, because a stale shared index reverts whoever springs
// it, and the operator's unqualified `bd sync:` commits are the form that
// sprang it. The middle one is the one arm keyed on RHQ_PERSONA, because
// ADR 0015 §2/§3 splits drafting from ratification BY ACTOR and the
// operator is the ratifying actor.
//
// Order is load-bearing at one join: the constitution arm must sit ABOVE
// the shared-index arm, which exits 0 in a linked worktree and mid-merge —
// and a dispatched worktree is where a persona's commits come from.
//
// identity is variadic so every existing call site that renders a hook with
// no interest in ADR 0024 D2 check 3 (a byte-exact fixture for a marker or
// ordering test, say) keeps compiling unchanged; a caller that must match
// what InstallCommitGuardHook actually writes has to pass the SAME slice
// (a.commitGuardLiterals(dir): the derived identity literals AND this box's
// crew names) or the two renders diverge on check 3 alone. It carries both
// kinds because a variadic parameter can only be one — and because the
// literal itself says which arms it belongs in (IdentityLiteral.PathsOnly),
// so nothing is lost by shipping them in one slice.
func CommitGuardHook(visibility string, set OpsPatternSet, identity ...IdentityLiteral) string {
	return commitGuardHead + hookStampFunc + visibilityGuardBody(visibility, set, identity) +
		constitutionGuardBody() + adrShaGuardBody() + sharedIndexBody
}

// hookRepo answers WHICH REPO the hook file belongs to — the question
// installHook already asks git (hooksDir: a linked worktree resolves to the
// COMMON hooks dir), asked of the visibility mark too. `beads_visibility:`
// keys name repos the operator declared, never "any tree that shares its
// objects", so a session worktree path can only ever fall through to
// unmarked→public. Since every path-scoped thing at launch names the tree
// the persona works in (rangerhq-09o2), the one thing that is NOT scoped to
// that tree — the shared hook — was being stamped from it, and every
// Worktree:true dispatch into a private repo restamped its shared hook
// public (ranger-base-up22).
//
// A main checkout, a non-repository and anything git cannot answer for come
// back unchanged; a linked worktree comes back as its main checkout, and a
// worktree of a BARE repo as the bare repo itself, which is the directory a
// config key would have to name there (there is no checkout above it).
func hookRepo(dir string) string {
	if LinkedGitDirs(dir) == nil {
		return dir
	}
	common, err := git(dir, "rev-parse", "--git-common-dir")
	if err != nil || common == "" {
		return dir
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(dir, common)
	}
	common = filepath.Clean(common)
	if filepath.Base(common) == ".git" {
		return filepath.Dir(common)
	}
	return common
}

// commitGuardLiterals is the ONE derivation every renderer of the commit
// guard uses: ADR 0024 D2 check 3's identity literals off hookRepo(dir),
// then this box's crew names (ADR 0012 D2, ranger-base-cdxpf), which are
// the home's and not the repo's. Both are box-derived and neither is ever
// committed.
//
// One function because the renders are compared BYTE FOR BYTE: the two
// installers write the hook, RenderSessionHooks writes the same bytes into
// a session hooks dir, and the L3 probe decides "ours" by rendering it
// again (ADR 0023). A second site deriving a different set reads as "ours
// but stale" on every launch — the defect ranger-base-up22 fixed for the
// visibility mark and the same one waiting behind a second literal source.
func (a *App) commitGuardLiterals(dir string) ([]IdentityLiteral, error) {
	identity, err := DeriveIdentityLiterals(hookRepo(dir))
	if err != nil {
		return nil, err
	}
	return append(identity, a.DeriveCrewLiterals()...), nil
}

// InstallCommitGuardHook writes the guard into the repo at dir (its common
// git dir, so worktrees share it), stamped with what config says about that
// repo's beads db — hookRepo, because the file lands in the shared repo and
// the mark is the shared repo's, not the calling tree's. Refuses to
// overwrite a foreign prepare-commit-msg; replaces ours in place. Returns
// the hook path and the visibility it stamped, so the caller can say which
// wall the operator just got.
//
// ADR 0024 D2 check 3's identity literals are derived HERE, from hookRepo's
// answer, same as the visibility mark — never read at commit time, for the
// same reason the mark is not (visibilityGuardBody's comment). A literal
// that cannot be rendered (a single quote) refuses the WHOLE install, the
// same init-panic class validateOpsERE holds the shipped pattern list to:
// better an install that says why it did not happen than a hook that
// renders wrong.
func (a *App) InstallCommitGuardHook(dir string) (path, visibility, source string, err error) {
	visibility, source = a.BeadsVisibility(hookRepo(dir))
	identity, err := a.commitGuardLiterals(dir)
	if err != nil {
		return "", "", "", err
	}
	path, err = installHook(dir, "prepare-commit-msg", sharedIndexMarker, legacySharedIndexMarker, CommitGuardHook(visibility, a.OpsPatternSet(), identity...), false)
	return path, visibility, source, err
}

// InstallCommitGuardHookChained is InstallCommitGuardHook, but a slot
// occupied by bd's own shim is chained rather than refused (rangerhq-mgdk).
func (a *App) InstallCommitGuardHookChained(dir string) (path, visibility, source string, err error) {
	visibility, source = a.BeadsVisibility(hookRepo(dir))
	identity, err := a.commitGuardLiterals(dir)
	if err != nil {
		return "", "", "", err
	}
	path, err = installHook(dir, "prepare-commit-msg", sharedIndexMarker, legacySharedIndexMarker, CommitGuardHook(visibility, a.OpsPatternSet(), identity...), true)
	return path, visibility, source, err
}

// CommitGuardHookInstalled reports whether the repo at dir has a hook carrying
// our ownership marker. See PrePushHookInstalled: parity must probe behavior.
func CommitGuardHookInstalled(dir string) bool {
	return hookInstalled(dir, "prepare-commit-msg", sharedIndexMarker, legacySharedIndexMarker)
}

// l3HookProbe is launch-time evidence about the two hook slots. Repo is false
// for a non-git session directory, where L3 is not applicable. PrePush is true
// without running that arm when the PID does not deny git push; the
// prepare-commit-msg arm always runs because its visibility, constitution and
// shared-index guards protect every persona session, independent of PID rule
// text.
//
// ADR 0023: a slot counts only when IDENTITY and BEHAVIOR both hold — the
// file at the dispatch path (`git rev-parse --git-path hooks`) is
// byte-for-byte our current render (or the prescribed chain to it), and our
// own render, exec'd fresh from a private temp file, still refuses. The file
// at the dispatch path is never exec'd, so a planted hook has nothing to lie
// to: there is no question being asked of it. *Degraded names why, when a
// slot does not count — foreign (no marker of ours), stale (marker present,
// bytes differ), or a renderer regression (identity holds, our own render
// failed to refuse).
//
// This is deliberately a snapshot, not a claim that the hook cannot change
// after the probe (TOCTOU/CWE-367). The seatbelt hook carve-out can prevent a
// caged session from changing it; cage: shims has no file-write boundary.
type l3HookProbe struct {
	Repo        bool
	PrePush     bool
	CommitGuard bool
	HooksDir    string

	// Managed is the employer's hooks dir this launch forwards into, and
	// is the flag for ADR 0052 D3's redirect mode: non-empty exactly when
	// HooksDir is the SESSION hooks dir posse rendered rather than the path
	// git dispatches from in the launcher's own environment. Parity reads
	// it to say which dir was probed and whose hooks run after ours.
	Managed string

	// PrePushDegraded and CommitGuardDegraded are full, ready-to-display
	// lines naming why the slot did not count. Empty when the slot counts
	// (or, for PrePushDegraded, when the PID does not deny git push).
	PrePushDegraded     string
	CommitGuardDegraded string

	// Forward carries redirect mode's own arm (ADR 0052 D3): one
	// ready-to-display line per managed hook this session's git would
	// SKIP, because the dir it dispatches from does not carry that slot's
	// dispatcher (M4). Not a posse gate — the loss is the employer's hook,
	// which is the one thing ADR 0052 promises never to cause — so it is
	// its own list rather than either slot's Degraded line.
	Forward []string
}

// l3Identity reports whether the file at hooks/slot is byte-for-byte render
// (+x) — our current render, dispatched — or the prescribed chain dispatcher
// (+x) with posse-<slot> byte-for-byte render (+x). When it is neither, stale
// distinguishes "carries our marker but the bytes differ" (reinstall fixes
// it) from "no marker of ours at all" (foreign; installHook will not touch
// it — ranger-base-3c3). path names the file the verdict is actually about —
// the dispatch-path file itself, or posse-<slot> behind a chain dispatcher —
// so a degraded line can name the file to fix rather than the slot in
// general.
func l3Identity(hooks, slot, render, marker, legacy string) (identity, stale bool, path string) {
	top := filepath.Join(hooks, slot)
	if !isRegularFile(top) {
		return false, false, top
	}
	body, err := os.ReadFile(top)
	if err == nil {
		if identityMatch(top, render) {
			return true, false, top
		}
		if isChainHookDispatcher(string(body), slot) {
			if fi, statErr := os.Stat(top); statErr == nil && fi.Mode()&0o111 != 0 {
				chained := filepath.Join(hooks, "posse-"+slot)
				if identityMatch(chained, render) {
					return true, false, chained
				}
				if isRegularFile(chained) {
					if cb, cerr := os.ReadFile(chained); cerr == nil && ownsHook(string(cb), marker, legacy) {
						return false, true, chained
					}
				}
				return false, false, chained
			}
		} else if ownsHook(string(body), marker, legacy) {
			return false, true, top
		}
	}
	return false, false, top
}

// isRegularFile answers the question l3Identity must ask before it reads:
// a FIFO or other special file at the dispatch path is never our render, and
// os.ReadFile on a FIFO with no writer never returns (ranger-base-gs9r).
// os.Stat follows symlinks, so a symlink to a regular file still passes.
func isRegularFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// refuseNonRegularHook stops installHook before it opens a hook path that is
// not a regular file. os.ReadFile on a FIFO with no writer never returns —
// and neither does the WriteFile a failed read would fall through to — so one
// mkfifo in a checkout's hooks dir hung every launch into it, silently and
// with no deadline anywhere above (ranger-base-92n5p; ranger-base-gs9r taught
// the probe the same lesson two functions up). A special file is never our
// render and never the chain dispatcher, so it is a foreign hook, and ADR
// 0002 §3 says foreign hooks are refused rather than overwritten: name it,
// name what it is, and print the move the operator can act on. The chain
// prescription is not that move — its verify step runs the slot, which a FIFO
// cannot be. os.Stat follows symlinks, so a link to a regular file installs
// as it always has; a missing path (a dangling symlink included) is not this
// case at all and is left to the reads below, which have always answered it.
func refuseNonRegularHook(p string) error {
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().IsRegular() {
		return nil
	}
	return Die("%s exists and is %s, not a posse hook — not overwriting.\nA git hook is a regular file: move that one aside (or delete it), then re-run install-hooks.", AbbrevHome(p), fileTypeName(fi.Mode()))
}

// fileTypeName names what is at a path when it is not a regular file, in
// words a refusal can print. os.FileMode.String() would render "p---------",
// which tells an operator holding a hung launcher nothing.
func fileTypeName(m os.FileMode) string {
	switch {
	case m.IsDir():
		return "a directory"
	case m&os.ModeNamedPipe != 0:
		return "a named pipe"
	case m&os.ModeSocket != 0:
		return "a socket"
	case m&os.ModeCharDevice != 0:
		return "a character device"
	case m&os.ModeDevice != 0:
		return "a device"
	case m&os.ModeSymlink != 0:
		return "a symlink"
	default:
		return "not a regular file"
	}
}

// identityMatch is byte-exact content plus the execute bit — the degenerate
// check that DETERMINES behavior instead of hinting at it (ADR 0023,
// amending ADR 0002 §3's "behavior, not the marker" doctrine). Non-regular
// files (a FIFO, a device) never match — ranger-base-gs9r: an executable
// FIFO passed the directory and execute-bit checks here and then blocked
// forever on the ReadFile below.
func identityMatch(path, render string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 || !fi.Mode().IsRegular() {
		return false
	}
	body, err := os.ReadFile(path)
	return err == nil && string(body) == render
}

// One shell invocation exercises both slots. Each render must refuse the
// exact operation it gates with exit 1; marker text and refusal output are
// not evidence. Hook output is discarded and RHQ_GATES_DIR is blank so a
// launch probe never forges a refusal-log entry. $1/$2 are private temp
// files carrying OUR OWN render (execOwnRenders), 0700 inside a 0700
// directory — never the file at the dispatch path — so this exec never runs
// bytes a session did not just write (ADR 0023 Decision 2).
const l3HookProbeScript = `
unset GIT_INDEX_FILE RHQ_VISIBILITY_OVERRIDE
posse_push_bad=0
if [ -n "$1" ]; then
  printf 'refs/heads/main a refs/heads/main b\n' |
    RHQ_PERSONA=probe RHQ_TOOLS_DENY='Bash(git push:*)' RHQ_GATES_DIR= \
    "$1" origin probe >/dev/null 2>&1
  posse_status=$?
  if [ "$posse_status" -ne 1 ]; then posse_push_bad=1; fi
fi
RHQ_PERSONA=probe RHQ_TOOLS_DENY= RHQ_GATES_DIR= \
  "$2" "$3" >/dev/null 2>&1
posse_status=$?
posse_commit_bad=0
if [ "$posse_status" -ne 1 ]; then posse_commit_bad=2; fi
exit $((posse_push_bad + posse_commit_bad))
`

// execOwnRenders writes one render per slot into a private 0700 directory
// (probeScratchDir) and execs THOSE under l3HookProbeScript — never the file
// at the dispatch path. This half of the probe catches a renderer regression
// (a broken /bin/sh, a bad render) rather than anything about what is planted
// at the dispatch path; identity (l3Identity) is what says whether the
// dispatch path is ours.
//
// The slot files are named for their slots rather than randomly: the random
// part is the directory, and inside a 0700 directory there is no name to
// enumerate.
func execOwnRenders(dir string, wantPrePush bool, commitRender string) (prePushOK, commitOK bool) {
	scratch, err := probeScratchDir()
	if err != nil {
		return false, false
	}
	defer os.RemoveAll(scratch)

	var pushTemp string
	if wantPrePush {
		f, err := writeTempRender(scratch, "pre-push", PrePushHook)
		if err != nil {
			return false, false
		}
		pushTemp = f
	}
	commitTemp, err := writeTempRender(scratch, "prepare-commit-msg", commitRender)
	if err != nil {
		return false, false
	}

	// The message file the commit render is handed. Not exec'd, and inside
	// the same 0700 directory for the same reason.
	msgPath := filepath.Join(scratch, "commit-msg")
	if err := os.WriteFile(msgPath, nil, 0o600); err != nil {
		return false, false
	}

	cmd := exec.Command("sh", "-c", l3HookProbeScript, "posse-hook-probe", pushTemp, commitTemp, msgPath)
	// The probe runs in the MAIN checkout, not in dir, since per-session
	// worktrees (rangerhq-09o2). The shared-index arm deliberately stands
	// down in a linked worktree — that tree's index is private and there is
	// nothing to sweep — so probing there would read a wall that is right to
	// be quiet as a wall that is not there, and degrade every launch into the
	// repo. The question the probe asks is whether the wall is installed and
	// refuses, and the checkout where it applies is where to ask it. A
	// persona that walks into the shared checkout meets the armed hook, which
	// is exactly what this proves.
	cmd.Dir = dir
	if repo, ok := MainCheckout(dir); ok {
		cmd.Dir = repo
	}
	err = cmd.Run()
	code := 0
	if err != nil {
		code = -1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	if code < 0 {
		return false, false
	}
	return !wantPrePush || code&1 == 0, code&2 == 0
}

// probeScratchDir is the private directory execOwnRenders builds this
// launch's renders in, mode 0700, removed by its caller's RemoveAll.
//
// It exists because of WHERE the exec happens: the launcher runs these bytes
// in its own UNSANDBOXED context at every launch, and $TMPDIR is writable by
// every caged session on this box at the same uid (ADR 0002's seatbelt
// writable set). A render written straight into $TMPDIR under a name a
// session can enumerate, at 0755, leaves a thin version of the escalation ADR
// 0023's Context set out to remove. Nothing about the probe needs either
// property: the launcher execs the file as its own uid, so 0700 is enough,
// and inside a 0700 directory the name cannot be enumerated at all. Decision
// 2 then holds by construction rather than by the width of a window
// (ranger-base-t5vh).
func probeScratchDir() (string, error) {
	d, err := os.MkdirTemp("", "posse-l3-probe-")
	if err != nil {
		return "", err
	}
	// MkdirTemp asks for 0700, but umask can only narrow what it gets. The
	// chmod makes the mode exact rather than whatever the launching shell
	// left, which is what the pin asserts.
	if err := os.Chmod(d, 0o700); err != nil {
		os.RemoveAll(d)
		return "", err
	}
	return d, nil
}

// writeTempRender writes one render into the probe's private directory at
// mode 0700 — readable, writable and executable by the launcher's uid, which
// is the only uid that touches it, and by nothing else. NOT 0755: see
// probeScratchDir.
func writeTempRender(dir, slot, body string) (string, error) {
	name := filepath.Join(dir, slot)
	if err := os.WriteFile(name, []byte(body), 0o700); err != nil {
		return "", err
	}
	// WriteFile's perm is also umask-narrowed, and only on create.
	if err := os.Chmod(name, 0o700); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

// l3DegradeLine is the full, ready-to-display Degraded entry for one slot:
// what posse cannot vouch for (foreign / stale / renderer regression), the
// remedy, and what protection is lost by consequence. ONE LINE: Degraded
// flows into session meta, a flat-file format with no quoting for embedded
// newlines (yamlflat.go) — a multi-line value silently truncates on
// read-back (measured, ranger-base-ujdg). The foreign case points at `posse
// gates install-hooks`, which prints the full paste-able chain prescription
// itself (installHook's Die, same text chainDispatcher renders) — this
// includes a foreign hook that happens to refuse everything: honest,
// because a black-box probe cannot tell that from a hook that refuses only
// the probe (the escape this ADR closes), and the launcher no longer runs
// it to find out.
func l3DegradeLine(hooks, slot, path, consequence string, identity, stale bool) string {
	switch {
	case !identity && stale:
		return fmt.Sprintf("L3 %s hook — %s — ours but stale — run `posse gates install-hooks`; %s", slot, AbbrevHome(path), consequence)
	case !identity:
		return fmt.Sprintf("L3 %s hook — %s — foreign hook, posse cannot vouch for a hook it did not write; %s (run `posse gates install-hooks` to see the chain prescription)", slot, AbbrevHome(path), consequence)
	default:
		return fmt.Sprintf("L3 %s hook — %s — our own render did not refuse the operation (renderer regression); %s", slot, AbbrevHome(path), consequence)
	}
}

func (a *App) probeL3Hooks(dir string, wantPrePush bool) l3HookProbe {
	return a.probeL3HooksIn(dir, wantPrePush, nil)
}

// probeL3HooksIn is probeL3Hooks with ADR 0052 D3's redirect mode: red
// non-nil says this launch's git does NOT dispatch from `git rev-parse
// --git-path hooks` — the session env aims it at the dir posse rendered
// (D2) — so identity is asked THERE, of files posse wrote, rather than at
// the managed path, of files posse may not write and never will. The
// behavior half (execOwnRenders) is unchanged: it execs our own render from
// a private temp file and never the file at any dispatch path, so it has
// nothing to learn from where git would have found one.
func (a *App) probeL3HooksIn(dir string, wantPrePush bool, red *l3Redirect) l3HookProbe {
	hooks, err := hooksDir(dir)
	if err != nil {
		return l3HookProbe{}
	}
	r := l3HookProbe{Repo: true, PrePush: !wantPrePush, HooksDir: hooks}
	if red != nil {
		hooks = red.Hooks
		r.HooksDir, r.Managed = red.Hooks, red.Managed
	}

	// hookRepo for the same reason the installers use it, doubled: identity
	// is byte-for-byte, so a probe that resolves the mark differently from
	// the install two lines above it in herdrback reads every worktree
	// launch in a marked repo as "ours but stale" (ranger-base-up22).
	visibility, _ := a.BeadsVisibility(hookRepo(dir))
	// The SAME set, and the SAME derived literals — identity AND crew — the
	// install stamps with, or an instance that adds a pattern, or that
	// staffs a new lane, reads as "ours but stale" on every launch (the identity
	// half of ADR 0023 is byte-for-byte). A derivation error here (a
	// literal with a single quote) is not this probe's to report — an
	// install that hit it already failed loudly — so it degrades to no
	// literals rather than propagating.
	identity, _ := a.commitGuardLiterals(dir)
	commitRender := CommitGuardHook(visibility, a.OpsPatternSet(), identity...)

	var prePushIdentity, prePushStale bool
	var prePushPath string
	if wantPrePush {
		prePushIdentity, prePushStale, prePushPath = l3IdentityIn(red, hooks, "pre-push", PrePushHook, prePushMarker, legacyPrePushMarker)
	}
	commitIdentity, commitStale, commitPath := l3IdentityIn(red, hooks, "prepare-commit-msg", commitRender, sharedIndexMarker, legacySharedIndexMarker)

	prePushBehavior, commitBehavior := execOwnRenders(dir, wantPrePush, commitRender)

	r.PrePush = !wantPrePush || (prePushIdentity && prePushBehavior)
	r.CommitGuard = commitIdentity && commitBehavior

	if wantPrePush && !r.PrePush {
		r.PrePushDegraded = l3DegradeLineIn(red, hooks, "pre-push", prePushPath, "this layer is not realized", prePushIdentity, prePushStale)
	}
	if !r.CommitGuard {
		r.CommitGuardDegraded = l3DegradeLineIn(red, hooks, "prepare-commit-msg", commitPath, "the data ceiling, beads visibility, constitution-path, ADR sha-stamp and shared-index guards are not realized", commitIdentity, commitStale)
	}
	// The forward-completeness arm, and the reason it is separate from the
	// two slots above: what a missing dispatcher costs is not a posse gate
	// but the EMPLOYER's hook (M4 — a slot the redirect dir lacks is
	// skipped), so it can be true while both of posse's slots hold.
	if red != nil {
		r.Forward = redirectForwardGaps(red.Hooks, red.Managed, wantPrePush)
	}
	return r
}

// deniesUnqualifiedCommit reports whether the PID's deny list carries the
// L1 half of the same wall — a negative `git commit` rule. The hook itself
// is not keyed on it (the shared tree, not the PID, is what makes the
// commit unsafe); parity reads this to name the second layer.
func deniesUnqualifiedCommit(deny []string) bool {
	for _, r := range ParseShimRules(deny)["git"] {
		if r.Unless != "" && len(r.Words) > 0 && r.Words[0] == "commit" {
			return true
		}
	}
	return false
}
