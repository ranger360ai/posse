package posse

import (
	"fmt"
	"os"
	"strings"
)

// ─── The hook-skip arm (ranger-base-r3pri, from ranger-base-zwkgb) ───────────
//
// WHAT IT REFUSES. Every spelling a seat could use to make git skip a hook —
// an employer-managed commit scanner above all (ADR 0052: "posse must never
// cause the managed hook to be skipped") — refused by the L1 git shim with a
// named rule and a refusals.log line:
//
//	hookskip:no-verify       commit -n/--no-verify (and every prefix down to
//	                         --no-v, and -n bundled: -nm), am -n/--no-verify,
//	                         merge/pull/rebase/push --no-verify and prefixes.
//	                         NOT push -n (that is --dry-run), NOT merge/
//	                         rebase/cherry-pick/revert -n (no-stat/no-commit).
//	hookskip:config-override -c / --config-env (both spellings) on ANY verb
//	                         whose key is core.hooksPath, hook.*, include.*
//	                         or includeIf.* — compared case-insensitively.
//	hookskip:config-env      on a hook-running verb: GIT_CONFIG_PARAMETERS
//	                         naming one of those keys; a GIT_CONFIG_KEY_<n>
//	                         below GIT_CONFIG_COUNT naming one, unless it is
//	                         core.hooksPath at posse's own ADR 0052 D2
//	                         redirect (<StateDir>/hooks/<session>); a
//	                         GIT_CONFIG_SYSTEM / GIT_CONFIG_NOSYSTEM that
//	                         changes what hook config git sees; and a
//	                         GIT_CONFIG_GLOBAL, HOME or XDG_CONFIG_HOME whose
//	                         global scope differs from the launcher's
//	                         (ADR 0079, below).
//	hookskip:config-write    `git config` writing, unsetting, renaming or
//	                         removing one of those keys or their sections, at
//	                         any scope, legacy or subcommand form; and
//	                         `git config --edit`/`-e`/`edit` outright.
//	hookskip:alias           (ranger-base-z8kym) a verb that is not a git
//	                         builtin is looked up as an alias, under the
//	                         call's own global options and env, after the
//	                         deny rules. A `!` (shell) alias is refused
//	                         outright; any other expansion is spliced into
//	                         argv in place of the verb and walked by every
//	                         arm above again, chains included, to depth 10.
//	                         Each expansion then meets the PID's verb deny
//	                         rules too (ranger-base-d4bk4, renderAliasDeny).
//
// alias.* IS A HOOK-RELEVANT KEY TOO (ranger-base-z8kym). `-c alias.ci="commit
// -n"`, the same in GIT_CONFIG_PARAMETERS or GIT_CONFIG_KEY_<n>, and `git
// config alias.ci "commit -n"` each define a verb that reaches commit -n
// without spelling it, so every arm that refuses a hooksPath key refuses an
// alias key under the same rule. Seats have no need to define aliases.
//
// WHY EVERY `!` ALIAS AND NOT ONLY AN ABSOLUTE-PATH ONE. The bead expected a
// `!git commit -n` alias to reach this shim again through PATH. It does not.
// MEASURED 2026-11-03, git 2.50.1 (Apple Git-155): git puts its exec-path
// ahead of PATH for a shell alias, that dir holds a `git` binary, and the
// inner bare `git` runs it and never this shim. A shell alias can run any git
// with any flag, so the arm refuses it outright.
//
// include.* and includeIf.* are not in the bead's list and are here on
// purpose: `-c include.path=<file>` loads a file that can set core.hooksPath,
// so leaving them out would leave every other arm one indirection wide.
//
// THE SYSTEM SCOPE OVERRIDES ARE JUDGED BY CONTENT, NOT BY VALUE. The bead's
// fallback was "differs from what the launch set", and the shim cannot know
// that: it is rendered per persona, not per runtime, and the claude inlet pin
// sets GIT_CONFIG_SYSTEM while no other runtime does — and ranger-base-t301k
// is changing that pin's value in flight. So the shim asks git directly: the
// hook-relevant keys (core.hookspath, hook.*, include.*, includeif.*) that
// the default scope carries, against the ones the overridden scope carries.
// Equal is allowed — `GIT_CONFIG_GLOBAL=/dev/null` on a box with no global
// hook config hides nothing, which is every hermetic test fixture in this
// repo — and different is refused, which is the one case that skips (or
// re-points) a hook. Two consequences, both deliberate:
//
//   - Today's claude pin (GIT_CONFIG_SYSTEM=/dev/null) is REFUSED on a commit
//     wherever system scope carries a hooksPath or hook.* — which is exactly
//     the employer box ranger-base-zwkgb is about. Loudly refusing that
//     commit is the correct interim behaviour until t301k pins the resolved
//     system path, at which point the two answers are equal and it passes.
//   - It costs two extra `git config` execs on a hook-running verb, and only
//     when one of those variables is present in the environment.
//
// GLOBAL SCOPE IS JUDGED AGAINST THE LAUNCHER'S FILES (ADR 0079,
// ranger-base-32ll0). It used to be the same unset-baseline compare as system
// scope, and that baseline was computed under the call's own HOME: MEASURED
// `HOME=evil GIT_CONFIG_GLOBAL=evil/.gitconfig git commit` was allowed with
// the managed hook unread, and a moved HOME or XDG_CONFIG_HOME was not judged
// at all. Now the render bakes in the launcher's global-scope files
// (gitGlobalScopeCandidates, the function obdnk's GIT_CONFIG_GLOBAL pin uses),
// its HOME and XDG_CONFIG_HOME, and that pin. A hook verb whose HOME, XDG or
// GIT_CONFIG_GLOBAL differs from those compares the global-scope hook keys git
// sees (posse_hs_gscope) against the ones the launcher's files carry, and
// refuses on a difference. The ordinary call moves none of the three and
// execs nothing extra. See hookSkipGlobalRef and its renderers below.
//
// RESIDUALS, past L1 by class (ADR 0025 — a cooperative wall: it holds
// against the ordinary path and says so): an absolute /usr/bin/git, `env -i`
// or `command -p git`; hand-editing .git/config or ~/.gitconfig; an alias
// expansion this arm splits differently from git — it unquotes the way
// split_cmdline does (posse_hs_split, ranger-base-e09l0; the IFS split it
// replaced left a quoted '-n' unmatched while git ran -n), which the PID's
// verb deny rules now judge as well (renderAliasDeny, ranger-base-d4bk4); a
// hand-edited copy of the baked reference in the shim file itself; and a
// hook-relevant config value carrying a newline, which the global compare
// cannot judge and so refuses (an over-refusal). A moved HOME /
// XDG_CONFIG_HOME is no longer a residual (ADR 0079). An over-refusal
// one respelling away is the accepted cost in every arm: `-c` values,
// commit clusters like `-uno` (the `-n` scan cannot tell `-u`'s glued value
// from a cluster), and any `git config` read whose form the scan cannot prove
// is a read.
//
// INDEPENDENT OF THE PID. RenderGates renders a git shim for every seat now,
// whether or not the PID denies any git verb, so this arm (and the
// sequencer audit beside it) reaches a PID that gates nothing in git.
//
// RUNTIME-INDEPENDENT. The shim is rendered once per persona and typed in
// front of every runtime's command line (WrapWithGates), so the same arm
// stands on claude, codex and the rest; TestQAHookSkipRefusedOnEveryRuntime
// executes it through each.

// hookSkipVerbs are the git verbs that run a hook a scanner could live in —
// the ones a config or scope override is judged on. -c is judged on EVERY
// verb instead (it costs nothing and closes `git -c core.hooksPath=x <alias>`).
var hookSkipVerbs = []string{"commit", "merge", "am", "push", "pull", "rebase", "cherry-pick", "revert"}

// hookSkipBuiltins are verbs the alias lookup skips: git runs a builtin
// before it looks for an alias, so no alias of these names can run. It is a
// SUBSET of `git --list-cmds=builtins` (git 2.50.1), long-standing ones only,
// chosen for the hot path. Leaving a builtin out costs one `git config` exec;
// listing a non-builtin would let its alias through, so a name goes in only
// when every git a seat could carry has it as a builtin.
// A word list, not a []string literal: TestQANoProductGitDiffReaderIsBare
// reads a string slice holding "diff" as a git argv.
var hookSkipBuiltins = strings.Fields(`
	add am apply blame branch cat-file checkout cherry-pick clone commit
	config diff fetch for-each-ref grep help init log ls-files ls-remote
	ls-tree merge merge-base mv pull push rebase reflog remote reset restore
	rev-list rev-parse revert rm show show-ref stash status switch
	symbolic-ref tag var version worktree
`)

// hookSkipNoVerify is, per verb, the spoiler table for its hook-skipping
// option. MEASURED 2026-11-03, git 2.50.1, `git <verb> -h`: commit and am
// spell it `-n, --no-verify`; merge, pull, rebase and push have only the
// long form, and on push, merge, rebase, cherry-pick and revert `-n` means
// something else (--dry-run, --no-stat, --no-commit) and is NOT refused.
//
// LongMin is --no-v for every verb on purpose, not a measured boundary: git
// accepts any unique prefix and `--no-verbose` shares the stem up to
// --no-ver, so --no-v/--no-ve/--no-ver are ambiguous on some verbs and git
// refuses them itself. Refusing them anyway is the safe direction (the
// bead: over-refusing is acceptable, under-refusing is not).
func hookSkipNoVerify(verb string) spoiler {
	long := map[string]string{"--no-verify": "--no-v"}
	switch verb {
	case "commit":
		c := qualifierSpoilers["git commit"]
		lm := map[string]string{"--no-verify": "--no-v"}
		for k, v := range c.LongMin {
			lm[k] = v
		}
		return spoiler{Opts: []string{"-n", "--no-verify"}, ValueOpts: c.ValueOpts, LongMin: lm}
	case "am":
		return spoiler{Opts: []string{"-n", "--no-verify"}, LongMin: long}
	case "merge", "pull", "rebase", "push":
		return spoiler{Opts: []string{"--no-verify"}, LongMin: long}
	}
	return spoiler{}
}

// hookSkipNoVerifyVerbs is the order the per-verb helpers are rendered in.
var hookSkipNoVerifyVerbs = []string{"commit", "am", "merge", "pull", "rebase", "push"}

// ciGlob spells s as a case-insensitive sh glob: `core` → `[Cc][Oo][Rr][Ee]`.
// git compares section and variable names case-insensitively, so a literal
// arm would miss `-c CORE.HOOKSPATH=`.
func ciGlob(s string) string {
	var b strings.Builder
	for _, c := range s {
		lo, up := strings.ToLower(string(c)), strings.ToUpper(string(c))
		if lo != up {
			b.WriteString("[" + up + lo + "]")
			continue
		}
		if c == '*' || c == '?' || c == '[' {
			b.WriteString(`\` + string(c))
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// hookKeyArms are the case patterns that name a hook-relevant config KEY
// (the part before `=` in a -c value). Subsection names are case-sensitive
// in git, but every key here is guarded by its section, so a section glob is
// the whole match.
func hookKeyArms() string {
	return strings.Join([]string{
		ciGlob("core.hookspath"),
		ciGlob("hook.") + "*",
		ciGlob("include.") + "*",
		ciGlob("includeif.") + "*",
		ciGlob("alias.") + "*",
	}, "|")
}

// hookSectionArms are hookKeyArms plus the bare section names a
// `--rename-section`/`--remove-section` can take. `core` is in it because
// removing or renaming the whole section drops core.hooksPath with it.
func hookSectionArms() string {
	return hookKeyArms() + "|" + strings.Join([]string{ciGlob("core"), ciGlob("hook"), ciGlob("include"), ciGlob("alias")}, "|")
}

// hookSkipRegexp is what the scope comparison asks `git config
// --get-regexp` for. git lowercases section and variable names in what it
// matches against, so the lowercase pattern is the whole set.
const hookSkipRegexp = `^(core\.hookspath|hook\.|include\.|includeif\.|alias\.)`

// renderHookSkipArm writes the arm. real is the git the shim execs ("" when
// none resolved at render — the scope comparison then fails CLOSED, since
// there is no git to ask). hooksRoot is <StateDir>/hooks, the parent of every
// ADR 0052 D2 redirect dir; "" allows no redirect at all.
func renderHookSkipArm(real, dateBin, hooksRoot string, ref hookSkipGlobalRef) string {
	var b strings.Builder
	b.WriteString("# ranger-base-r3pri: hook-skipping forms are refused on every seat, every\n")
	b.WriteString("# runtime, independent of the PID (ADR 0052). Cooperative (ADR 0025): an\n")
	b.WriteString("# absolute /usr/bin/git, env -i, or a hand-edited config file are\n")
	b.WriteString("# past this wall by class; aliases are resolved (ranger-base-z8kym), and a moved\n")
	b.WriteString("# HOME/XDG_CONFIG_HOME/GIT_CONFIG_GLOBAL is judged against the launcher's global\n")
	b.WriteString("# files (ADR 0079) — see hookskip.go.\n")
	b.WriteString("posse_hs_refuse() {\n  posse_hs_rule=$1; posse_hs_why=$2\n")
	b.WriteString("  echo \"refused by posse gate: git $posse_hs_argv — $posse_hs_why (rule: $posse_hs_rule, ranger-base-r3pri)\" >&2\n")
	b.WriteString("  echo \"  a hook a scanner may live in (an employer's managed hook above all) must run on every commit and push from a seat: posse never causes it to be skipped (ADR 0052). Retype without the skip; if the hook itself is broken, say so on your bead and stop.\" >&2\n")
	fmt.Fprintf(&b, "  echo \"%s git $posse_hs_argv (hookskip: $posse_hs_rule)\" >> \"$RHQ_GATE_LOG\" 2>/dev/null\n  exit 1\n}\n", refusalTimestamp(dateBin))

	// Per-verb -n/--no-verify scanners, built by the same renderer the
	// path-limited commit rule's spoiler scan uses, so `-m n`, `-mn` and
	// `-nm` read the way they do there.
	for _, v := range hookSkipNoVerifyVerbs {
		b.WriteString(renderSpoiled(shIdent("posse_hs_nv", v), hookSkipNoVerify(v)))
	}

	// One -c / --config-env value: refuse when its key is hook-relevant.
	fmt.Fprintf(&b, "posse_hs_key() {\n  case \"${1%%%%=*}\" in\n    %s) return 0 ;;\n  esac\n  return 1\n}\n", hookKeyArms())

	// The SYSTEM scope comparison: the hook-relevant keys git sees with the
	// override, against the ones it sees with the override unset. (The GLOBAL
	// half moved to posse_hs_gscope, ADR 0079.)
	q := "false"
	if real != "" {
		q = shQuote(real)
	}
	fmt.Fprintf(&b, "posse_hs_scope() {\n  posse_hs_have=$(%s config --system --get-regexp %s 2>/dev/null)\n", q, shQuote(hookSkipRegexp))
	b.WriteString("  case \"${GIT_CONFIG_NOSYSTEM:-}\" in ''|0|[Ff][Aa][Ll][Ss][Ee]|[Nn][Oo]|[Oo][Ff][Ff]) ;; *) posse_hs_have= ;; esac\n")
	fmt.Fprintf(&b, "  posse_hs_want=$(unset GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_NOSYSTEM; %s config --system --get-regexp %s 2>/dev/null)\n", q, shQuote(hookSkipRegexp))
	if real == "" {
		// No git to ask: an override is present and nothing can say what it
		// hides, so it is treated as hiding something.
		b.WriteString("  return 1\n}\n")
	} else {
		b.WriteString("  [ \"$posse_hs_have\" = \"$posse_hs_want\" ]\n}\n")
	}
	b.WriteString(renderHookSkipGlobalScope(real, ref))

	// The env arm, for hook-running verbs.
	b.WriteString("posse_hs_env() {\n")
	fmt.Fprintf(&b, "  case \"${GIT_CONFIG_PARAMETERS:-}\" in\n    *%s*|*\\'%s*|*\\'%s*|*\\'%s*|*\\'%s*) posse_hs_refuse hookskip:config-env 'GIT_CONFIG_PARAMETERS names a hook-moving key or an alias' ;;\n  esac\n",
		ciGlob("hookspath"), ciGlob("hook."), ciGlob("include."), ciGlob("includeif."), ciGlob("alias."))
	b.WriteString("  posse_hs_n=${GIT_CONFIG_COUNT:-0}\n  case \"$posse_hs_n\" in ''|*[!0-9]*) posse_hs_n=0 ;; esac\n  posse_hs_i=0\n")
	b.WriteString("  while [ \"$posse_hs_i\" -lt \"$posse_hs_n\" ]; do\n")
	b.WriteString("    eval \"posse_hs_k=\\${GIT_CONFIG_KEY_$posse_hs_i:-}; posse_hs_v=\\${GIT_CONFIG_VALUE_$posse_hs_i:-}\"\n")
	b.WriteString("    if posse_hs_key \"$posse_hs_k\"; then\n")
	allow := "false"
	if hooksRoot != "" {
		// The ADR 0052 D2 redirect: core.hooksPath at one session dir under
		// <StateDir>/hooks — posse's own dispatcher, which forwards to the
		// managed hooks and so skips nothing.
		allow = fmt.Sprintf("{ case \"$posse_hs_k\" in %s) case \"$posse_hs_v\" in %s/*/*) false ;; %s/?*) true ;; *) false ;; esac ;; *) false ;; esac; }",
			ciGlob("core.hookspath"), shQuote(hooksRoot), shQuote(hooksRoot))
	}
	fmt.Fprintf(&b, "      %s || posse_hs_refuse hookskip:config-env \"GIT_CONFIG_KEY_$posse_hs_i names $posse_hs_k, and not at posse's own ADR 0052 redirect\"\n", allow)
	b.WriteString("    fi\n    posse_hs_i=$((posse_hs_i+1))\n  done\n")
	b.WriteString(renderHookSkipGlobalGate(ref))
	b.WriteString("  if { [ -n \"${GIT_CONFIG_SYSTEM+x}\" ] || [ -n \"${GIT_CONFIG_NOSYSTEM:-}\" ]; } && ! posse_hs_scope; then\n    posse_hs_refuse hookskip:config-env 'GIT_CONFIG_SYSTEM/GIT_CONFIG_NOSYSTEM changes the hook config git sees in system scope'\n  fi\n")
	b.WriteString("}\n")

	// `git config` writes. Its args come after the verb.
	b.WriteString("posse_hs_config() {\n  posse_hs_w=; posse_hs_r=; posse_hs_np=0; posse_hs_hit=; posse_hs_sub=\n")
	b.WriteString("  while [ $# -gt 0 ]; do\n    case \"$1\" in\n")
	b.WriteString("      --) shift; break ;;\n")
	b.WriteString("      -e|--edit|--edi|--ed) posse_hs_refuse hookskip:config-write 'git config --edit can write any key, core.hooksPath included' ;;\n")
	b.WriteString("      -f|--file|--blob|--type|--default|--comment|--value) [ $# -ge 2 ] && shift ;;\n")
	b.WriteString("      --get*|-l|--list) posse_hs_r=1 ;;\n")
	// Options that neither write nor take a separate value. Anything else
	// long — an abbreviated --unset, --add, --replace-all — is a write
	// suspect, so an unknown option can only make the scan refuse more.
	b.WriteString("      --global|--system|--local|--worktree|--file=*|--blob=*|--type=*|--default=*|--comment=*|--value=*|--show-origin|--show-scope|--includes|--no-includes|-z|--null|--name-only|--bool|--int|--bool-or-int|--path|--expiry-date|--no-type|--fixed-value|--all|--regexp|--url=*) ;;\n")
	b.WriteString("      --*) posse_hs_w=1 ;;\n")
	b.WriteString("      -*e*) posse_hs_refuse hookskip:config-write 'git config -e can write any key, core.hooksPath included' ;;\n")
	b.WriteString("      -*) ;;\n")
	b.WriteString("      *)\n        if [ $posse_hs_np -eq 0 ] && [ -z \"$posse_hs_sub\" ]; then\n          case \"$1\" in\n")
	b.WriteString("            get|list) posse_hs_sub=$1; posse_hs_r=1; shift; continue ;;\n")
	b.WriteString("            edit) posse_hs_refuse hookskip:config-write 'git config edit can write any key, core.hooksPath included' ;;\n")
	b.WriteString("            set|unset|rename-section|remove-section) posse_hs_sub=$1; posse_hs_w=1; shift; continue ;;\n")
	b.WriteString("          esac\n        fi\n")
	fmt.Fprintf(&b, "        posse_hs_np=$((posse_hs_np+1))\n        case \"$1\" in %s) posse_hs_hit=1 ;; esac ;;\n", hookSectionArms())
	b.WriteString("    esac\n    shift\n  done\n")
	b.WriteString("  # A positional after `--` is still a key or a value.\n  while [ $# -gt 0 ]; do\n")
	fmt.Fprintf(&b, "    posse_hs_np=$((posse_hs_np+1))\n    case \"$1\" in %s) posse_hs_hit=1 ;; esac\n    shift\n  done\n", hookSectionArms())
	b.WriteString("  [ -n \"$posse_hs_hit\" ] || return 0\n")
	// Legacy form with no action option: one positional is a get, two or
	// more is a set. git refuses two actions at once, so a read action
	// present means no write happens.
	b.WriteString("  [ -z \"$posse_hs_w\" ] && [ -n \"$posse_hs_r\" ] && return 0\n")
	b.WriteString("  [ -z \"$posse_hs_w\" ] && [ -z \"$posse_hs_sub\" ] && [ $posse_hs_np -le 1 ] && return 0\n")
	b.WriteString("  return 1\n}\n")

	// The walk: global options first (checking -c/--config-env as it goes),
	// then the verb and its own arguments.
	b.WriteString("posse_hs_check() {\n  posse_hs_all=$#; posse_hs_noalias=\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n")
	// Options that print and exit before any verb runs: an alias lookup under
	// them would read their output as an expansion.
	b.WriteString("      -h|--help|-v|--version|--exec-path|--html-path|--man-path|--info-path|--list-cmds=*) posse_hs_noalias=1; shift ;;\n")
	b.WriteString("      -c|--config-env)\n        [ $# -ge 2 ] || break\n        posse_hs_key \"$2\" && posse_hs_refuse hookskip:config-override \"$1 $2 moves or adds a hook, or defines an alias\"\n        shift 2 ;;\n")
	b.WriteString("      --config-env=*)\n        posse_hs_key \"${1#--config-env=}\" && posse_hs_refuse hookskip:config-override \"$1 moves or adds a hook, or defines an alias\"\n        shift ;;\n")
	if pairs := globalValueOpts["git"]; len(pairs) > 0 {
		var quoted []string
		for _, o := range pairs {
			if o == "-c" || o == "--config-env" {
				continue
			}
			quoted = append(quoted, shQuote(o))
		}
		fmt.Fprintf(&b, "      %s)\n        [ $# -ge 2 ] || break\n        shift 2 ;;\n", strings.Join(quoted, "|"))
	}
	b.WriteString("      -*) shift ;;\n      *) break ;;\n    esac\n  done\n")
	b.WriteString("  [ $# -gt 0 ] || return 0\n  posse_hs_ng=$((posse_hs_all - $#)); posse_hs_verb=$1; shift\n")
	// Not a builtin: maybe an alias. Recorded, and resolved after the deny
	// rules (renderHookSkipEnvCall), because resolving it execs git.
	bi := make([]string, 0, len(hookSkipBuiltins))
	for _, v := range hookSkipBuiltins {
		bi = append(bi, shQuote(v))
	}
	fmt.Fprintf(&b, "  posse_hs_aliasverb=\n  case \"$posse_hs_verb\" in %s) ;; -*) ;; *) [ -n \"$posse_hs_noalias\" ] || posse_hs_aliasverb=1 ;; esac\n", strings.Join(bi, "|"))
	b.WriteString("  case \"$posse_hs_verb\" in\n")
	for _, v := range hookSkipNoVerifyVerbs {
		fmt.Fprintf(&b, "    %s) %s \"$@\" && posse_hs_refuse hookskip:no-verify '%s skips its hooks' ;;\n",
			v, shIdent("posse_hs_nv", v), strings.Join(hookSkipNoVerify(v).Opts, "/"))
	}
	b.WriteString("    config) posse_hs_config \"$@\" || posse_hs_refuse hookskip:config-write 'git config writes a hook-moving key or an alias' ;;\n")
	b.WriteString("  esac\n")
	verbs := make([]string, 0, len(hookSkipVerbs))
	for _, v := range hookSkipVerbs {
		verbs = append(verbs, shQuote(v))
	}
	// Recorded, not run: the env arm execs git to compare scopes, and that
	// must wait until every deny rule has had its say (renderHookSkipEnvCall).
	fmt.Fprintf(&b, "  case \"$posse_hs_verb\" in %s) posse_hs_envverb=1 ;; esac\n", strings.Join(verbs, "|"))
	b.WriteString("  return 0\n}\n")
	b.WriteString(renderHookSkipAlias(real))
	b.WriteString("posse_hs_argv=\"$*\"; posse_hs_envverb=\nposse_hs_check \"$@\"\n")
	return b.String()
}

// renderHookSkipAlias writes the alias resolver (ranger-base-z8kym).
// posse_hs_lookup asks the real git for alias.<verb> under the call's own
// global options (the first posse_hs_ng args, so `-C <repo>` reads that
// repo's config) and env. posse_hs_unalias splices the expansion in place of
// the verb and re-walks posse_hs_check until the verb is a builtin, no
// alias, or one already seen (a loop, which git refuses on its own). A chain
// of more than 10 distinct aliases is refused rather than judged. With no real git there is nothing to ask and nothing that could run
// the alias either, so the resolver is a no-op.
func renderHookSkipAlias(real string) string {
	if real == "" {
		return "posse_hs_unalias() { :; }\n"
	}
	var b strings.Builder
	b.WriteString("posse_hs_lookup() {\n  posse_hs_k=0\n  for posse_hs_a in \"$@\"; do\n")
	b.WriteString("    [ $posse_hs_k -eq 0 ] && set --\n")
	b.WriteString("    [ $posse_hs_k -lt $posse_hs_ng ] && set -- \"$@\" \"$posse_hs_a\"\n")
	b.WriteString("    posse_hs_k=$((posse_hs_k+1))\n  done\n")
	fmt.Fprintf(&b, "  posse_hs_x=$(%s \"$@\" config --get \"alias.$posse_hs_verb\" 2>/dev/null) || posse_hs_x=\n}\n", shQuote(real))
	b.WriteString(hookSkipSplit)
	b.WriteString("posse_hs_unalias() {\n  posse_hs_d=0; posse_hs_seen=' '\n  while [ -n \"$posse_hs_aliasverb\" ]; do\n")
	// A verb seen before is a loop, and git refuses a loop itself ("alias
	// loop detected") without running anything, so it is handed to git.
	b.WriteString("    case \"$posse_hs_seen\" in *\" $posse_hs_verb \"*) return 0 ;; esac\n    posse_hs_seen=\"$posse_hs_seen$posse_hs_verb \"\n")
	b.WriteString("    posse_hs_d=$((posse_hs_d+1))\n")
	b.WriteString("    [ $posse_hs_d -le 10 ] || posse_hs_refuse hookskip:alias 'an alias chain deeper than 10 cannot be judged'\n")
	b.WriteString("    posse_hs_lookup \"$@\"\n    [ -n \"$posse_hs_x\" ] || return 0\n")
	b.WriteString("    case \"$posse_hs_x\" in\n      !*) posse_hs_refuse hookskip:alias \"alias $posse_hs_verb is a shell alias, which runs git past this gate (git puts its exec-path ahead of PATH)\" ;;\n    esac\n")
	// Rebuild argv: the globals, the expansion (split and unquoted the way
	// git's split_cmdline does it, by posse_hs_split), then the rest. The
	// for list is expanded once, before the first set --.
	b.WriteString("    posse_hs_split \"$posse_hs_x\" || posse_hs_refuse hookskip:alias \"alias $posse_hs_verb has an unclosed quote or a trailing backslash, which git refuses and this gate cannot judge\"\n")
	b.WriteString("    posse_hs_k=0\n    for posse_hs_a in \"$@\"; do\n      [ $posse_hs_k -eq 0 ] && set --\n")
	b.WriteString("      if [ $posse_hs_k -eq $posse_hs_ng ]; then eval \"set -- \\\"\\$@\\\" $posse_hs_q\"\n      else set -- \"$@\" \"$posse_hs_a\"; fi\n")
	b.WriteString("      posse_hs_k=$((posse_hs_k+1))\n    done\n")
	// Each expansion is judged by this arm, then by the PID's deny rules
	// (posse_hs_pid, rendered by renderShim — ranger-base-d4bk4). This arm
	// goes first for the same reason it does on the typed argv: its reason
	// is the more specific one. posse_hs_av keeps the alias name for the
	// refusal, since posse_hs_check moves posse_hs_verb on.
	b.WriteString("    posse_hs_av=$posse_hs_verb\n    posse_hs_check \"$@\"\n    posse_hs_pid \"$@\"\n  done\n}\n")
	return b.String()
}

// hookSkipSplit is git's split_cmdline (alias.c, git 2.50.1) in POSIX sh:
// outside quotes a run of space, tab, CR or LF ends a word; ' and " open a
// quote that the same character closes; a backslash outside single quotes
// takes the next character literally. An empty quoted word and a trailing
// separator each still make a word, as they do in git. An unclosed quote
// or a trailing backslash returns 1 — git refuses those itself.
//
// The result is posse_hs_q: every word single-quoted (a quote inside is
// closed, backslash-escaped and reopened), so `eval "set -- \"\$@\" $posse_hs_q"` appends exactly those
// words and cannot run, glob or split anything (ranger-base-e09l0: the IFS
// splice it replaces left `'-n'` as `'-n'` while git ran `-n`).
const hookSkipSplit = `posse_hs_split() {
  posse_hs_s=$1; posse_hs_q="'"; posse_hs_qt=
  posse_hs_ws=$(printf ' \t\r\n.'); posse_hs_ws=${posse_hs_ws%.}
  while [ -n "$posse_hs_s" ]; do
    posse_hs_c=${posse_hs_s%"${posse_hs_s#?}"}; posse_hs_s=${posse_hs_s#?}
    if [ -z "$posse_hs_qt" ]; then
      case "$posse_hs_c" in
        [$posse_hs_ws])
          while :; do case "$posse_hs_s" in [$posse_hs_ws]*) posse_hs_s=${posse_hs_s#?} ;; *) break ;; esac; done
          posse_hs_q="$posse_hs_q' '"; continue ;;
        \'|\") posse_hs_qt=$posse_hs_c; continue ;;
      esac
    elif [ "$posse_hs_c" = "$posse_hs_qt" ]; then
      posse_hs_qt=; continue
    fi
    if [ "$posse_hs_c" = '\' ] && [ "$posse_hs_qt" != "'" ]; then
      [ -n "$posse_hs_s" ] || return 1
      posse_hs_c=${posse_hs_s%"${posse_hs_s#?}"}; posse_hs_s=${posse_hs_s#?}
    fi
    case "$posse_hs_c" in
      \') posse_hs_q="$posse_hs_q'\\''" ;;
      *) posse_hs_q="$posse_hs_q$posse_hs_c" ;;
    esac
  done
  [ -z "$posse_hs_qt" ] || return 1
  posse_hs_q="$posse_hs_q'"
}
`

// renderHookSkipEnvCall is the env arm's call site, emitted by renderShim
// AFTER the PID's deny rules. The argv checks above are pure sh and run first;
// this one execs the real git (posse_hs_scope) whenever a GIT_CONFIG_SYSTEM
// scope variable is set — on a claude seat that is every call, since the
// inlet pin sets GIT_CONFIG_SYSTEM — and (posse_hs_gscope) whenever HOME,
// XDG_CONFIG_HOME or GIT_CONFIG_GLOBAL differs from the launcher's (ADR
// 0079), and a command the PID refuses must never reach
// the real binary for any reason (TestRefusalNamesWhereTheCommandDoesRun
// caught exactly that: a denied push ran git config through the leak canary).
//
// The alias resolver goes here too, for the same reason (it execs git). The
// env arm runs first on an alias candidate as well, so an alias the env
// itself defines is refused under hookskip:config-env and not by its
// expansion. The env arm reads only the env, never the verb, so one pass in
// front of the resolver covers whatever verb the expansion turns out to be.
//
// THE ONE EXEC A DENIED COMMAND MAKES (ranger-base-d4bk4). The PID's deny
// rules judge an alias's expansion too (renderAliasDeny), and the expansion
// is only known once the real git has answered `config --get alias.<verb>`.
// So a command refused BY ITS EXPANSION has reached git exactly once, for
// that read-only lookup, and never for its own verb. A deny matched on the
// typed argv still reaches git zero times, since a builtin verb is never
// looked up. TestQAPIDDenyAliasLookupIsTheOnlyExec pins both.
func renderHookSkipEnvCall() string {
	return "if [ -n \"$posse_hs_envverb\" ] || [ -n \"$posse_hs_aliasverb\" ]; then posse_hs_env; fi\n" +
		"[ -n \"$posse_hs_aliasverb\" ] && posse_hs_unalias \"$@\"\n"
}

// renderAliasDeny writes posse_hs_pid, which posse_hs_unalias calls on each
// expansion it splices (ranger-base-d4bk4). An alias from the operator's own
// gitconfig (`alias.c = commit`) reaches the shim as the token `c`, and the
// PID's verb rules matched only that token, so `git c -m x` was a bare commit
// under `Bash(git commit unless --)`. posse_hs_pid runs the same
// posse_verb_match on the expansion and refuses under the same rule, naming
// the alias in the hint. A PID with no verb rules renders no posse_verb_match,
// and posse_hs_pid is then a no-op.
//
// Only VERB rules are re-run. A rule that leads with an option is a literal
// prefix of the typed argv (renderShim). An expansion replaces the verb and
// leaves the globals in front of it alone, so that prefix is unchanged.
func renderAliasDeny(haveVerbRules bool) string {
	if !haveVerbRules {
		return "posse_hs_pid() { :; }\n"
	}
	var b strings.Builder
	b.WriteString("posse_hs_pid() {\n  posse_verb_match \"$@\"\n  [ -n \"$RHQ_GATE_RULE\" ] || return 0\n")
	b.WriteString("  posse_hs_h=\"  git $posse_hs_av is an alias for '$posse_hs_x': the deny rule judged its expansion (ranger-base-d4bk4)\"\n")
	b.WriteString("  [ -n \"$RHQ_GATE_HINT\" ] && posse_hs_h=\"$posse_hs_h\n$RHQ_GATE_HINT\"\n")
	b.WriteString("  RHQ_GATE_HINT=$posse_hs_h\n  posse_refuse \"$@\"\n}\n")
	return b.String()
}

// HookSkipGate is the parity row the arm is reported under (CheckParity).
// It names no PID rule because it answers to none.
const HookSkipGate = "git hook-skip forms (-n/--no-verify, -c/env/config hooksPath)"

// ─── The global-scope reference (ADR 0079, ranger-base-32ll0) ────────────────
//
// hookSkipGlobalRef is what "global scope not moved" means for this shim: the
// LAUNCHER's global-scope files and the env they were resolved from, baked in
// at render. It replaces the unset-baseline GIT_CONFIG_GLOBAL arm, whose
// baseline was computed under the call's own (possibly moved) HOME — MEASURED
// a bypass at rc 0 (ADR 0079 Context 2).
type hookSkipGlobalRef struct {
	// Cands are the files git reads as global scope with GIT_CONFIG_GLOBAL
	// unset, in git's order (gitGlobalScopeCandidates). Empty when the
	// launcher has neither HOME nor XDG_CONFIG_HOME (ADR 0079 D4).
	Cands []string
	// The launcher's HOME and XDG_CONFIG_HOME. Unset is not empty: git
	// resolves the two differently.
	Home, XDG       string
	HomeSet, XDGSet bool
	// Pin is obdnk's GIT_CONFIG_GLOBAL row value, "" when the launch pins
	// none (a two-file box, or no candidates). Derived from Cands by the same
	// chooseGitGlobalScope, so the claude pin and this reference cannot drift.
	Pin string
}

// envLookupLast reads k from an env list the way exec does: last one wins.
func envLookupLast(env []string, k string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], k+"="); ok {
			return v, true
		}
	}
	return "", false
}

// resolveHookSkipGlobalRef resolves the reference from env (nil means
// os.Environ(), which is what every real launch uses; see App.GitScopeEnv).
// real is the git the shim execs; "" leaves the candidates to git's
// documented rule, and the arm then fails closed anyway (renderHookSkipGlobalScope).
func resolveHookSkipGlobalRef(real string, env []string) hookSkipGlobalRef {
	if env == nil {
		env = os.Environ()
	}
	var r hookSkipGlobalRef
	r.Home, r.HomeSet = envLookupLast(env, "HOME")
	r.XDG, r.XDGSet = envLookupLast(env, "XDG_CONFIG_HOME")
	// Every GIT_CONFIG* shed, as gitGlobalScope does: the question is which
	// files git reads with the variable UNSET.
	var scrub []string
	for _, kv := range env {
		if !strings.HasPrefix(kv, "GIT_CONFIG") {
			scrub = append(scrub, kv)
		}
	}
	cands, err := gitGlobalScopeCandidates(real, scrub)
	if err != nil {
		return r // no candidates: an empty reference (ADR 0079 D4)
	}
	r.Cands = cands
	if p, err := chooseGitGlobalScope(cands, func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}); err == nil {
		r.Pin = p
	}
	return r
}

// hookSkipGlobalRef is the reference RenderGates bakes into this App's git
// shims, resolved from a.GitScopeEnv.
func (a *App) hookSkipGlobalRef(real string) hookSkipGlobalRef {
	return resolveHookSkipGlobalRef(real, a.GitScopeEnv)
}

// shSetState renders the "${V+x}:${V-}" string a variable's set-ness and
// value compare as, for the baked side of the D3 gate.
func shSetState(v string, set bool) string {
	if !set {
		return shQuote(":")
	}
	return shQuote("x:" + v)
}

// renderHookSkipGlobalScope writes posse_hs_gscope (ADR 0079 D2): 0 when the
// hook-relevant keys the call's git sees in GLOBAL scope equal the ones the
// launcher's global files carry, 1 otherwise.
//
//   - have: `git config --show-scope --get-regexp`, every scope, keeping the
//     `global` lines in pure sh. Not `git config --global`: MEASURED, that
//     reads only ~/.gitconfig when the XDG file exists too (Context 3).
//   - want: `git config --includes --file <c> --get-regexp` per baked
//     candidate, concatenated; a missing file contributes nothing.
//
// TWO ADDITIONS TO D2, both fail-closed:
//
//   - The want reads run under the LAUNCHER's HOME and XDG_CONFIG_HOME, not
//     the call's. An `include.path = ~/x` in the operator's global file
//     expands `~` from HOME, so a want read under the moved HOME would follow
//     the attacker's include on both sides and call them equal.
//   - A value carrying a newline can forge a `global<TAB>` line in the
//     non -z listing (MEASURED 2026-11-04, git 2.50.1: a local alias value
//     "a\nglobal\tcore.hookspath /m" prints exactly that line). -z cannot be
//     read in sh, since command substitution drops NULs. So the entries are
//     counted with `--name-only` (a key cannot hold a newline) and a listing
//     with more lines than entries is refused as unjudgeable. Cost: one exec.
//
// Cost when it runs: 2 + len(Cands) execs. It runs only when the D3 gate
// (renderHookSkipGlobalGate) says something moved.
func renderHookSkipGlobalScope(real string, ref hookSkipGlobalRef) string {
	if real == "" {
		// No git to ask (ADR 0079 D4: fails closed, as before).
		return "posse_hs_gscope() {\n  return 1\n}\n"
	}
	q, re := shQuote(real), shQuote(hookSkipRegexp)
	var b strings.Builder
	b.WriteString("posse_hs_gscope() {\n")
	fmt.Fprintf(&b, "  posse_hs_gall=$(%s config --show-scope --get-regexp %s 2>/dev/null)\n", q, re)
	fmt.Fprintf(&b, "  posse_hs_gnames=$(%s config --show-scope --name-only --get-regexp %s 2>/dev/null)\n", q, re)
	// Split on newline in a subshell, so IFS and set -f stay its own, and
	// with no here-document (sh may need a temp file for one, and a cage
	// that cannot write one would read nothing: fail-open). The subshell's
	// status says whether the count held.
	b.WriteString("  posse_hs_have=$(IFS='\n'; set -f; posse_hs_gn=0; posse_hs_gl=0\n")
	b.WriteString("    for posse_hs_l in $posse_hs_gnames; do posse_hs_gn=$((posse_hs_gn+1)); done\n")
	b.WriteString("    for posse_hs_l in $posse_hs_gall; do\n      posse_hs_gl=$((posse_hs_gl+1))\n")
	// `(pattern)`, not `pattern)`: bash 3.2 (darwin's /bin/sh) misparses an
	// unbalanced `)` from a case arm inside a command substitution.
	b.WriteString("      case \"$posse_hs_l\" in (global\"\t\"*) printf '%s\\n' \"${posse_hs_l#global\t}\" ;; esac\n    done\n")
	b.WriteString("    # More lines than entries: a value carries a newline and could forge a global line.\n")
	b.WriteString("    [ \"$posse_hs_gl\" -eq \"$posse_hs_gn\" ]) || return 1\n")
	// want, under the launcher's HOME and XDG_CONFIG_HOME.
	b.WriteString("  posse_hs_want=$(")
	for _, kv := range []struct {
		k, v string
		set  bool
	}{{"HOME", ref.Home, ref.HomeSet}, {"XDG_CONFIG_HOME", ref.XDG, ref.XDGSet}} {
		if kv.set {
			fmt.Fprintf(&b, "%s=%s; export %s; ", kv.k, shQuote(kv.v), kv.k)
		} else {
			fmt.Fprintf(&b, "unset %s; ", kv.k)
		}
	}
	if len(ref.Cands) == 0 {
		b.WriteString(":")
	} else {
		cs := make([]string, 0, len(ref.Cands))
		for _, c := range ref.Cands {
			cs = append(cs, shQuote(c))
		}
		fmt.Fprintf(&b, "for posse_hs_c in %s; do %s config --includes --file \"$posse_hs_c\" --get-regexp %s 2>/dev/null; done", strings.Join(cs, " "), q, re)
	}
	b.WriteString(")\n")
	b.WriteString("  [ \"$posse_hs_have\" = \"$posse_hs_want\" ]\n}\n")
	return b.String()
}

// renderHookSkipGlobalGate writes the env arm's global-scope check (ADR 0079
// D3): a string test that decides whether posse_hs_gscope runs at all, so the
// ordinary call — HOME, XDG_CONFIG_HOME and GIT_CONFIG_GLOBAL as rendered —
// execs no extra git.
//
// DIVERGED from D3 in one case, toward running the compare more: D3 skips it
// when GIT_CONFIG_GLOBAL is the baked pin, whatever HOME says, because git
// ignores HOME for the file lookup there. git does NOT ignore HOME inside the
// file: `include.path = ~/x` in the pinned file expands from the call's HOME.
// So a moved HOME/XDG runs the compare under the pin too. That costs execs
// only on a call that moved HOME, which the ordinary call never does, and
// pin 6's "pinned GIT_CONFIG_GLOBAL plus HOME=<empty>" is still allowed,
// because the contents are equal.
func renderHookSkipGlobalGate(ref hookSkipGlobalRef) string {
	var b strings.Builder
	b.WriteString("  posse_hs_gm=\n")
	if ref.Pin == "" {
		b.WriteString("  [ -n \"${GIT_CONFIG_GLOBAL+x}\" ] && posse_hs_gm=1\n")
	} else {
		fmt.Fprintf(&b, "  [ -n \"${GIT_CONFIG_GLOBAL+x}\" ] && [ \"$GIT_CONFIG_GLOBAL\" != %s ] && posse_hs_gm=1\n", shQuote(ref.Pin))
	}
	fmt.Fprintf(&b, "  [ \"${HOME+x}:${HOME-}\" = %s ] || posse_hs_gm=1\n", shSetState(ref.Home, ref.HomeSet))
	fmt.Fprintf(&b, "  [ \"${XDG_CONFIG_HOME+x}:${XDG_CONFIG_HOME-}\" = %s ] || posse_hs_gm=1\n", shSetState(ref.XDG, ref.XDGSet))
	b.WriteString("  if [ -n \"$posse_hs_gm\" ] && ! posse_hs_gscope; then\n    posse_hs_refuse hookskip:config-env 'GIT_CONFIG_GLOBAL, HOME or XDG_CONFIG_HOME changes the hook config git sees in global scope, against the launcher'\"'\"'s global files (ADR 0079)'\n  fi\n")
	return b.String()
}
