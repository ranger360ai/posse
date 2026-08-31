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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
// `-qp` and `-sp` (measured, git 2.50.1, ranger-base-myai). That costs the
// one false positive of the same class as above: `-mi`/`-mp` is the message
// "i"/"p", and it is refused. The safe form is one space away.
type spoiler struct {
	Opts []string // `-x` (single letter, matches in a cluster) or `--long`
	// LongMin is the shortest abbreviation git resolves to a long option in
	// Opts, keyed by that option. git's parse-options accepts any
	// UNAMBIGUOUS PREFIX, so `--inc` IS `--include` and an arm spelling the
	// option in full misses every abbreviation on the way to it
	// (ranger-base-l1at). Every long option in Opts needs an entry; without
	// one only the literal is refused and the abbreviations walk past.
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
		// Measured, git 2.50.1, one prefix at a time against the real git
		// (qaGitResolves): `--inc` resolves to `--include`, `--patc` to
		// `--patch`, `--int` to `--interactive`. `--in`/`--i` are ambiguous
		// between the first and the last, and `--pat` is ambiguous with
		// `--pathspec-from-file`, so git rejects those itself and the wall
		// does not have to.
		LongMin: map[string]string{"--include": "--inc", "--patch": "--patc", "--interactive": "--int"},
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
func renderSpoiled(name string, sp spoiler) string {
	var longs, shorts []string
	for _, o := range sp.Opts {
		if strings.HasPrefix(o, "--") {
			longs = append(longs, o)
			continue
		}
		shorts = append(shorts, strings.TrimPrefix(o, "-"))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s() {\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n", name)
	b.WriteString("      --) return 1 ;;\n") // past here every word is a path
	for _, l := range longs {
		fmt.Fprintf(&b, "      %s) return 0 ;;\n", strings.Join(longArms(l, sp.LongMin[l]), "|"))
	}
	if len(shorts) > 0 {
		// Long options are done above: without this arm `--signoff` would
		// match the cluster pattern for `-i` and be refused.
		b.WriteString("      --*) ;;\n")
		for _, s := range shorts {
			fmt.Fprintf(&b, "      -*%s*) return 0 ;;\n", s)
		}
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
// (ADR 0002 §3) — but a polite refusal that fires on `git push` and not on
// `git -C <repo> push` is the friction the design meant to provide going
// missing exactly where it is wanted (rangerhq-3mc).
//
// Claude matches a Bash rule three ways (verified on claude 2.1.234, and
// the CLI's own `--disallowedTools` splitter does not split inside the
// parens, so a rule with spaces reaches the matcher whole):
//
//	Bash(git push)    exact    — the whole command, and nothing else
//	Bash(git push:*)  prefix   — a literal prefix of the command string
//	Bash(git * push)  wildcard — `*` is `.*` over the whole string,
//	                             anchored both ends, whitespace collapsed
//
// The prefix form is why the option spellings walk past: `git -C x push`
// does not start with `git push`. So each subcommand PREFIX rule also gets
// one option-blind wildcard, `<cmd> -* <words> *` — a leading option,
// anything, then the words as their own tokens, then anything. The
// trailing ` *` rather than `<words>*`: keeping the token boundary
// explicit is what leaves `git --no-pager log -- push.txt` and `git -c …
// commit -m "push it"` alone (both verified running, along with the nine
// option spellings verified refused).
//
// It was a pair until rangerhq-ky3: `<cmd> -* <words>` rode alongside for
// the bare spelling, `git -C <r> push` with nothing after it. `*` is `.*`,
// so a rule ENDING in the words matches any `git -…` command whose last
// word is one of them — `git -C <r> log --grep push` was refused live
// (claude 2.1.234, and grok 1.0.5 on the same rule text), `git -C <r>
// stash push` with it, while `--grep=push` ran. No spelling separates the
// two: the real bare form and the false positive are both `git -`,
// anything, ` push` at the end, and the dialect has neither negation nor a
// way to say "option tokens only" (a glob has no repetition of a group).
// At L0 a false positive is a hard block the model cannot ask its way past
// — the ground rangerhq-3mc rejected a single `Bash(git -* push*)` on — so
// the exact half goes and its coverage goes with it. The cost, stated:
// `git <globals> push` with no further args draws no polite refusal, only
// L1's hard one (TestShimSkipsGlobalOptionsBeforeSubcommand). L0 is
// politeness, never the wall.
//
// A whole-verb rule (Bash(bd)) is the other half of the same miss: claude
// reads it as *exact*, so `bd show x` walks past a rule the shim reads as
// the whole verb. It gets `Bash(<cmd>:*)` alongside.
//
// Only deny lists go through this. Widening an allow list would grant more
// than the PID says; allow is friction, and stays the PID's words.
//
// And only claude's realizer calls it. Grok speaks the same dialect — the
// wildcard included — but matches a *shell-parsed* segment, quotes off, so
// the pair there also refuses `git -C <r> log --author "push me"`; the
// false positive costs more than the politeness buys, and L1 is the wall
// on grok either way (rangerhq-625).
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
		switch {
		case len(r.Words) == 0:
			add("Bash(" + cmd + ":*)")
		case r.Verb() && !r.Exact:
			// Only the prefix rule gets it: on an exact rule a trailing ` *`
			// would refuse `git -C <r> push origin main`, which the PID's
			// exact rule — and the shim reading it — do not.
			add("Bash(" + cmd + " -* " + strings.Join(r.Words, " ") + " *)")
		}
		// A rule leading with an option (Bash(rm -rf /)) is a literal argv
		// prefix in both matchers — it is already spelled where it means.
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
	old := os.Getenv("PATH")
	os.Setenv("PATH", PathOutsideGates(binDir))
	defer os.Setenv("PATH", old)
	real, err := exec.LookPath(cmd)
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
	fmt.Fprintf(&b, "#!/bin/sh\n# posse gate for %s — rendered from the PID's deny: at launch; do not edit (rangerhq-9ha)\n", persona)
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
// Bash(git:*), Bash(git). Catches /usr/bin/git push, subprocess pushes,
// and anything that dodged the L1 shim but kept the env. It cannot see
// through `env -i` (nothing in-process can); that is the container tier's.
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
func hooksDir(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return "", Die("%s is not a git repository", dir)
	}
	hooks := strings.TrimSpace(string(out))
	if hooks == "" {
		return "", Die("%s is not a git repository", dir)
	}
	if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(dir, hooks)
	}
	return hooks, nil
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
	stdin := ""
	if slot == "pre-push" {
		// git feeds pre-push the ref list on stdin. Ours does not read it;
		// keeping ours off it leaves it intact for the hook we exec into.
		stdin = " </dev/null"
	}
	guarded := ""
	if guard {
		guarded = fmt.Sprintf("[ -x \"$d/%s\" ] || exit 0\n", neighbor)
	}
	return fmt.Sprintf(`#!/bin/sh
d=$(dirname "$0")
"$d/posse-%[1]s" "$@"%[2]s || exit $?
%[4]sexec "$d/%[3]s" "$@"
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
	hooks, err := hooksDir(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(hooks, slot)
	if b, err := os.ReadFile(p); err == nil && !ownsHook(string(b), marker, legacy) {
		// A chain made from our printed prescription is deliberately foreign
		// at the slot: overwriting it would discard the other tool's hook. Its
		// posse-* member is ours, though, and must not become a frozen copy of
		// an older gate just because it lives behind the dispatcher. So behind
		// the exact dispatcher we prescribe, a marker-owned member is
		// refreshed and a MISSING one is restored; only a member that is there
		// and foreign stops us, and it stops us with its own words.
		chained := filepath.Join(hooks, "posse-"+slot)
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
	b, err := os.ReadFile(filepath.Join(hooks, slot))
	if err != nil {
		return false
	}
	body := string(b)
	if ownsHook(body, marker, legacy) {
		return true
	}
	if isChainHookDispatcher(body, slot) {
		owned, err := os.ReadFile(filepath.Join(hooks, "posse-"+slot))
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
// push, or "" if none — ADR 0033 §2's coordinator's defining permission. It
// reuses deniesGitPush's rule-shape parser: the same Bash(git push...) shape
// means the same thing whether it appears in allow: or deny:.
func grantsGitPush(allow []string) string {
	for cmd, rules := range ParseShimRules(allow) {
		if cmd != "git" {
			continue
		}
		for _, r := range rules {
			if len(r.Words) == 0 || r.Words[0] == "push" {
				return r.Rule
			}
		}
	}
	return ""
}

// ─── L3: the commit guard (prepare-commit-msg) ───────────────────────────────

// sharedIndexMarker identifies our prepare-commit-msg hook. The slot now
// carries three walls — the beads visibility guard, the constitution-path
// guard and the shared-index guard — and the marker still says
// `shared-index` ON PURPOSE: ownership is a
// question about a file an EARLIER binary wrote, and a repo hooked before
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
// set the same way the constitution arm does — `git diff --cached
// --name-only HEAD` under the hook's inherited GIT_INDEX_FILE, which is
// right for a path-limited commit's own next-index the same way it is
// there. Unkeyed like the rest of this wall, for the same reason
// (rangerhq-lt2w): the tree does not care who typed the commit. It runs
// ahead of MERGE_HEAD/CHERRY_PICK_HEAD/rebase exiting 0 above, so those
// three exemptions stand exactly as they did — this bead's Verification
// list does not ask for more, and none of the three has a pathspec-based
// way through to prefer instead.
const sharedIndexBody = `
# ─── the shared-index guard (rangerhq-lmq9) ───────────────────────────────
# No RHQ_PERSONA test: this wall covers every shell in the shared checkout,
# the operator's own included (rangerhq-lt2w). The tree is what makes the
# commit unsafe, and the tree does not care who typed it.
posse_gitdir=$(git rev-parse --git-dir 2>/dev/null) || exit 0
# ranger-base-58to: the two prescribed lines below are commands, meant to be
# pasted into a shell — so the paths in them have to survive that paste.
# 'git diff --name-only' C-quotes any path with a non-ASCII byte
# (core.quotePath is on by default) and leaves a path with a space bare;
# neither is what a shell will accept back. core.quotePath=false turns the
# quoting off, and each path is then wrapped in single quotes with any
# embedded quote escaped the POSIX way ('\'') — round-trips through a shell
# byte-for-byte. Newline-delimited, like the code it replaces: a path with a
# literal newline in it is already outside what --name-only can round-trip.
posse_qcached() {
  git -c core.quotePath=false diff --cached --name-only HEAD 2>/dev/null | while IFS= read -r posse_p; do
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
posse_notes_staged=$(git diff --cached --name-only HEAD 2>/dev/null)
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
  if [ -f "$posse_gitdir/MERGE_MSG" ] && cmp -s "$1" "$posse_gitdir/MERGE_MSG"; then
    posse_staged=$(posse_qcached)
    echo "git prepared this commit itself (revert): it staged the change into the"
    echo "shared index BEFORE this hook could refuse, so the change is sitting there"
    echo "now. It starts bounded — git revert only begins from an index matching"
    echo "HEAD — so these paths are the revert's, plus anything you staged after it:"
    echo "  finish it:  git commit -F - -- $posse_staged"
    echo "  or undo it: git restore --source=HEAD --staged --worktree -- $posse_staged"
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
    echo "  staged now: $posse_staged"
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
` + sharedIndexMarker + ` — installed by posse gates install-hooks. Three walls
# in one slot: the beads visibility guard (rangerhq-hrz), the constitution-path
# guard (ranger-base-ak3e) and the shared-index commit guard (rangerhq-lmq9).
# Foreign hooks are never overwritten; remove this file to uninstall.
# ADR 0002 §3.
`

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
func visibilityGuardBody(visibility string, set OpsPatternSet) string {
	var checks strings.Builder
	for _, p := range set.All() {
		fmt.Fprintf(&checks, "    posse_check %s %s\n", shQuote(p.Class), shQuote(p.ERE))
	}
	// A config pattern this instance asked for and did not get is recorded
	// HERE, in the file, for the same reason the stamp is: a human reading
	// the hook has to be able to tell what it is checking from what someone
	// meant it to check. Class names only — an instance's pattern IS its
	// confidential vocabulary, and this file is generated, read and pasted.
	var rejects strings.Builder
	if len(set.Rejected) > 0 {
		fmt.Fprintf(&rejects, "# instance patterns REFUSED at stamp time (config %s:), not in force below:\n", OpsPatternsConfigKey)
		for _, r := range set.Rejected {
			fmt.Fprintf(&rejects, "#   %s\n", r)
		}
	}
	return `
# ─── the beads visibility guard (rangerhq-hrz) ────────────────────────────
# A bead belongs in a public repo's db only when any deployer of this
# software could have written it; everything describing ONE deployment goes
# in that instance's private db (NOTES.md, "Privacy model"). This is a
# pattern lint, not a boundary — same class as the allowlist. The boundary
# is the routing rule plus repo visibility; the lint exists so a mis-routed
# bead is a refusal at commit time instead of a public artifact.
#
# The slot is prepare-commit-msg and not pre-commit for the reason the wall
# below documents: pre-commit is bd's own flush hook, reinstalled silently
# by bd, and a wall a third-party tool replaces on its next install is not
# a wall. This one also survives --no-verify.
` + rejects.String() + `posse_beads_visibility=` + shQuote(visibility) + `
if [ "$posse_beads_visibility" = ` + shQuote(VisibilityPublic) + ` ]; then
  # Compare against HEAD, or against the empty tree in a repo with no
  # commit yet — a first commit is exactly when a db arrives whole.
  posse_base=$(git hash-object -t tree /dev/null 2>/dev/null)
  if git rev-parse --verify -q HEAD >/dev/null 2>&1; then posse_base=HEAD; fi
  # ADDED lines only, and every .beads jsonl: the db and the deletion ledger
  # beside it (rangerhq-fuom), which holds whole bead records and inherits
  # the repo's visibility exactly as the db does. GIT_INDEX_FILE is
  # inherited, so this reads the same index the commit will.
  posse_added=$(git diff --cached -U0 "$posse_base" -- '.beads/*.jsonl' 2>/dev/null |
    grep '^+' | grep -v '^+++')
  if [ -n "$posse_added" ]; then
    posse_bad=''
    # A function, not a 'while read' over a pipeline: the right side of a
    # pipeline is a subshell and the assignment would not survive it — the
    # rangerhq-kk6e lesson, which cost a push.
    posse_check() {
      posse_m=$(printf '%s\n' "$posse_added" | grep -oE "$2" 2>/dev/null | head -3 | tr '\n' ' ')
      [ -n "$posse_m" ] || return 0
      posse_bad="$posse_bad  $1: $2
    matched: $posse_m
"
    }
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
fi
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
// THE TO-BE-COMMITTED SET is `git diff --cached --name-only` under the hook's
// INHERITED GIT_INDEX_FILE, which is what makes it right for both forms this
// wall leaves standing: a path-limited commit's temporary next-index IS the
// index git hands the hook, so what that diff reports is what the commit will
// take — not what the shared index happens to hold. `-z`, so a path with a
// quote or a space in it is not read through git's own path quoting, and the
// base is the empty tree in a repo with no commit yet (a first commit is
// exactly when a constitution arrives whole). `diff.relative` is pinned off:
// git runs hooks from the top level, but that config exists and a
// repo-relative class has to be compared against repo-relative paths.
//
// THE CLASS is two lists, because the two have different reasons:
//
//	(a) in the CONSTITUTION repo only — a repo whose top level has
//	    ConstitutionRepoMarker — every ConstitutionRepoPaths entry:
//	    `rhq/<p>` for each PromotedPaths entry, plus `rhq/envs`. This is the
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
// one rule covers `rhq/config.yaml` (a file) and `rhq/agents` (a tree).
//
// WHAT IT DOES NOT DO: it does not reset, unstage or otherwise touch the
// tree. The shared-index arm below already says why in its own words — a hook
// that cleaned up behind a persona would be the destructive act the wall
// exists to prevent — and here the staged diff is the very thing the
// prescribed route asks the persona to keep hold of.
//
// THE RESIDUAL, stated rather than discovered. This is L3, the shim tier:
// `env -i` scrubs RHQ_PERSONA and the arm stands down, which is the exact
// residual PrePushHook already documents for its own marker. Two things sit
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
  if [ -n "$posse_cls_top" ] && [ -d "$posse_cls_top/` + ConstitutionRepoMarker + `" ]; then
    posse_cls="${posse_cls}` + class(ConstitutionRepoPaths()) + `"
  fi
  # The index the COMMIT will use, which is the one git handed this hook —
  # a path-limited commit's own next-index included. Empty tree when there is
  # no HEAD yet.
  posse_cls_base=$(git hash-object -t tree /dev/null 2>/dev/null)
  if git rev-parse --verify -q HEAD >/dev/null 2>&1; then posse_cls_base=HEAD; fi
  posse_cls_staged=$(git -c diff.relative=false diff --cached --name-only -z "$posse_cls_base" 2>/dev/null | tr '\0' '\n')
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
      echo "under — the PIDs, config, recipes and skills a promote copies into the"
      echo "home, the env files beside them, and the settings file carrying this"
      echo "session's own deny list (ranger-base-az93: a persona that can commit"
      echo "that file un-fences itself)."
      echo "staged now, and in the class:"
      printf '%s' "$posse_cls_hit"
      echo "the way through — stage the intended diff, the operator applies it:"
      echo "  write what you MEAN under a path OUTSIDE the class (az93's went to"
      echo "  docs/rca/az93-settings.json), commit that path, and say on the bead"
      echo "  which class path it replaces and why. The operator reviews it, puts"
      echo "  it in place, and — for the constitution — runs posse promote, which"
      echo "  is the step that makes prose law (ADR 0015 §3)."
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
func CommitGuardHook(visibility string, set OpsPatternSet) string {
	return commitGuardHead + hookStampFunc + visibilityGuardBody(visibility, set) +
		constitutionGuardBody() + sharedIndexBody
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

// InstallCommitGuardHook writes the guard into the repo at dir (its common
// git dir, so worktrees share it), stamped with what config says about that
// repo's beads db — hookRepo, because the file lands in the shared repo and
// the mark is the shared repo's, not the calling tree's. Refuses to
// overwrite a foreign prepare-commit-msg; replaces ours in place. Returns
// the hook path and the visibility it stamped, so the caller can say which
// wall the operator just got.
func (a *App) InstallCommitGuardHook(dir string) (path, visibility, source string, err error) {
	visibility, source = a.BeadsVisibility(hookRepo(dir))
	path, err = installHook(dir, "prepare-commit-msg", sharedIndexMarker, legacySharedIndexMarker, CommitGuardHook(visibility, a.OpsPatternSet()), false)
	return path, visibility, source, err
}

// InstallCommitGuardHookChained is InstallCommitGuardHook, but a slot
// occupied by bd's own shim is chained rather than refused (rangerhq-mgdk).
func (a *App) InstallCommitGuardHookChained(dir string) (path, visibility, source string, err error) {
	visibility, source = a.BeadsVisibility(hookRepo(dir))
	path, err = installHook(dir, "prepare-commit-msg", sharedIndexMarker, legacySharedIndexMarker, CommitGuardHook(visibility, a.OpsPatternSet()), true)
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

	// PrePushDegraded and CommitGuardDegraded are full, ready-to-display
	// lines naming why the slot did not count. Empty when the slot counts
	// (or, for PrePushDegraded, when the PID does not deny git push).
	PrePushDegraded     string
	CommitGuardDegraded string
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
				if cb, cerr := os.ReadFile(chained); cerr == nil && ownsHook(string(cb), marker, legacy) {
					return false, true, chained
				}
				return false, false, chained
			}
		} else if ownsHook(string(body), marker, legacy) {
			return false, true, top
		}
	}
	return false, false, top
}

// identityMatch is byte-exact content plus the execute bit — the degenerate
// check that DETERMINES behavior instead of hinting at it (ADR 0023,
// amending ADR 0002 §3's "behavior, not the marker" doctrine).
func identityMatch(path, render string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
		return false
	}
	body, err := os.ReadFile(path)
	return err == nil && string(body) == render
}

// One shell invocation exercises both slots. Each render must refuse the
// exact operation it gates with exit 1; marker text and refusal output are
// not evidence. Hook output is discarded and RHQ_GATES_DIR is blank so a
// launch probe never forges a refusal-log entry. $1/$2 are private temp
// files carrying OUR OWN render (execOwnRenders) — never the file at the
// dispatch path — so this exec never runs bytes a session did not just
// write (ADR 0023 Decision 2).
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

// execOwnRenders writes render to a private temp file per slot and execs
// THAT under l3HookProbeScript — never the file at the dispatch path. This
// half of the probe catches a renderer regression (a broken /bin/sh, a bad
// render) rather than anything about what is planted at the dispatch path;
// identity (l3Identity) is what says whether the dispatch path is ours.
func execOwnRenders(dir string, wantPrePush bool, commitRender string) (prePushOK, commitOK bool) {
	var pushTemp string
	if wantPrePush {
		f, err := writeTempRender(PrePushHook)
		if err != nil {
			return false, false
		}
		defer os.Remove(f)
		pushTemp = f
	}
	commitTemp, err := writeTempRender(commitRender)
	if err != nil {
		return false, false
	}
	defer os.Remove(commitTemp)

	msg, err := os.CreateTemp("", "posse-prepare-commit-msg-probe-")
	if err != nil {
		return false, false
	}
	msg.Close()
	defer os.Remove(msg.Name())

	cmd := exec.Command("sh", "-c", l3HookProbeScript, "posse-hook-probe", pushTemp, commitTemp, msg.Name())
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

func writeTempRender(body string) (string, error) {
	f, err := os.CreateTemp("", "posse-l3-render-")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	f.Close()
	if err := os.Chmod(name, 0o755); err != nil {
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
	hooks, err := hooksDir(dir)
	if err != nil {
		return l3HookProbe{}
	}
	r := l3HookProbe{Repo: true, PrePush: !wantPrePush, HooksDir: hooks}

	// hookRepo for the same reason the installers use it, doubled: identity
	// is byte-for-byte, so a probe that resolves the mark differently from
	// the install two lines above it in herdrback reads every worktree
	// launch in a marked repo as "ours but stale" (ranger-base-up22).
	visibility, _ := a.BeadsVisibility(hookRepo(dir))
	// The SAME set the install stamps with, or an instance that adds a
	// pattern reads as "ours but stale" on every launch (the identity half
	// of ADR 0023 is byte-for-byte).
	commitRender := CommitGuardHook(visibility, a.OpsPatternSet())

	var prePushIdentity, prePushStale bool
	var prePushPath string
	if wantPrePush {
		prePushIdentity, prePushStale, prePushPath = l3Identity(hooks, "pre-push", PrePushHook, prePushMarker, legacyPrePushMarker)
	}
	commitIdentity, commitStale, commitPath := l3Identity(hooks, "prepare-commit-msg", commitRender, sharedIndexMarker, legacySharedIndexMarker)

	prePushBehavior, commitBehavior := execOwnRenders(dir, wantPrePush, commitRender)

	r.PrePush = !wantPrePush || (prePushIdentity && prePushBehavior)
	r.CommitGuard = commitIdentity && commitBehavior

	if wantPrePush && !r.PrePush {
		r.PrePushDegraded = l3DegradeLine(hooks, "pre-push", prePushPath, "this layer is not realized", prePushIdentity, prePushStale)
	}
	if !r.CommitGuard {
		r.CommitGuardDegraded = l3DegradeLine(hooks, "prepare-commit-msg", commitPath, "the beads visibility, constitution-path and shared-index guards are not realized", commitIdentity, commitStale)
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
