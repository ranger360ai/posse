package posse

import (
	"fmt"
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
//	                         redirect (<StateDir>/hooks/<session>); and a
//	                         GIT_CONFIG_GLOBAL / GIT_CONFIG_SYSTEM /
//	                         GIT_CONFIG_NOSYSTEM that changes what hook
//	                         config git sees (below).
//	hookskip:config-write    `git config` writing, unsetting, renaming or
//	                         removing one of those keys or their sections, at
//	                         any scope, legacy or subcommand form; and
//	                         `git config --edit`/`-e`/`edit` outright.
//
// include.* and includeIf.* are not in the bead's list and are here on
// purpose: `-c include.path=<file>` loads a file that can set core.hooksPath,
// so leaving them out would leave every other arm one indirection wide.
//
// THE SCOPE OVERRIDES ARE JUDGED BY CONTENT, NOT BY VALUE. The bead's
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
// RESIDUALS, past L1 by class (ADR 0025 — a cooperative wall: it holds
// against the ordinary path and says so): an absolute /usr/bin/git, `env -i`
// or `command -p git`; hand-editing .git/config or ~/.gitconfig; a git ALIAS
// that expands to a hook verb plus a skip flag (the alias hole this file's
// preamble already names); and HOME / XDG_CONFIG_HOME pointed elsewhere,
// which moves the global scope without naming any GIT_CONFIG_* variable — the
// shim has no notion of the "real" home to compare against. An over-refusal
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
	}, "|")
}

// hookSectionArms are hookKeyArms plus the bare section names a
// `--rename-section`/`--remove-section` can take. `core` is in it because
// removing or renaming the whole section drops core.hooksPath with it.
func hookSectionArms() string {
	return hookKeyArms() + "|" + strings.Join([]string{ciGlob("core"), ciGlob("hook"), ciGlob("include")}, "|")
}

// hookSkipRegexp is what the scope comparison asks `git config
// --get-regexp` for. git lowercases section and variable names in what it
// matches against, so the lowercase pattern is the whole set.
const hookSkipRegexp = `^(core\.hookspath|hook\.|include\.|includeif\.)`

// renderHookSkipArm writes the arm. real is the git the shim execs ("" when
// none resolved at render — the scope comparison then fails CLOSED, since
// there is no git to ask). hooksRoot is <StateDir>/hooks, the parent of every
// ADR 0052 D2 redirect dir; "" allows no redirect at all.
func renderHookSkipArm(real, dateBin, hooksRoot string) string {
	var b strings.Builder
	b.WriteString("# ranger-base-r3pri: hook-skipping forms are refused on every seat, every\n")
	b.WriteString("# runtime, independent of the PID (ADR 0052). Cooperative (ADR 0025): an\n")
	b.WriteString("# absolute /usr/bin/git, env -i, an alias, HOME moved, or a hand-edited\n")
	b.WriteString("# config file are past this wall by class — see hookskip.go.\n")
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

	// The scope comparison: the hook-relevant keys git sees with the
	// override, against the ones it sees with the override unset.
	q := "false"
	if real != "" {
		q = shQuote(real)
	}
	fmt.Fprintf(&b, "posse_hs_scope() {\n  posse_hs_have=$(%s config --$1 --get-regexp %s 2>/dev/null)\n", q, shQuote(hookSkipRegexp))
	b.WriteString("  case \"$1\" in\n    system) case \"${GIT_CONFIG_NOSYSTEM:-}\" in ''|0|[Ff][Aa][Ll][Ss][Ee]|[Nn][Oo]|[Oo][Ff][Ff]) ;; *) posse_hs_have= ;; esac ;;\n  esac\n")
	fmt.Fprintf(&b, "  posse_hs_want=$(unset GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_NOSYSTEM; %s config --$1 --get-regexp %s 2>/dev/null)\n", q, shQuote(hookSkipRegexp))
	if real == "" {
		// No git to ask: an override is present and nothing can say what it
		// hides, so it is treated as hiding something.
		b.WriteString("  return 1\n}\n")
	} else {
		b.WriteString("  [ \"$posse_hs_have\" = \"$posse_hs_want\" ]\n}\n")
	}

	// The env arm, for hook-running verbs.
	b.WriteString("posse_hs_env() {\n")
	fmt.Fprintf(&b, "  case \"${GIT_CONFIG_PARAMETERS:-}\" in\n    *%s*|*\\'%s*|*\\'%s*|*\\'%s*) posse_hs_refuse hookskip:config-env 'GIT_CONFIG_PARAMETERS names a hook-moving key' ;;\n  esac\n",
		ciGlob("hookspath"), ciGlob("hook."), ciGlob("include."), ciGlob("includeif."))
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
	b.WriteString("  if [ -n \"${GIT_CONFIG_GLOBAL+x}\" ] && ! posse_hs_scope global; then\n    posse_hs_refuse hookskip:config-env 'GIT_CONFIG_GLOBAL changes the hook config git sees in global scope'\n  fi\n")
	b.WriteString("  if { [ -n \"${GIT_CONFIG_SYSTEM+x}\" ] || [ -n \"${GIT_CONFIG_NOSYSTEM:-}\" ]; } && ! posse_hs_scope system; then\n    posse_hs_refuse hookskip:config-env 'GIT_CONFIG_SYSTEM/GIT_CONFIG_NOSYSTEM changes the hook config git sees in system scope'\n  fi\n")
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
	b.WriteString("posse_hs_check() {\n  while [ $# -gt 0 ]; do\n    case \"$1\" in\n")
	b.WriteString("      -c|--config-env)\n        [ $# -ge 2 ] || break\n        posse_hs_key \"$2\" && posse_hs_refuse hookskip:config-override \"$1 $2 moves or adds a hook\"\n        shift 2 ;;\n")
	b.WriteString("      --config-env=*)\n        posse_hs_key \"${1#--config-env=}\" && posse_hs_refuse hookskip:config-override \"$1 moves or adds a hook\"\n        shift ;;\n")
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
	b.WriteString("  [ $# -gt 0 ] || return 0\n  posse_hs_verb=$1; shift\n")
	b.WriteString("  case \"$posse_hs_verb\" in\n")
	for _, v := range hookSkipNoVerifyVerbs {
		fmt.Fprintf(&b, "    %s) %s \"$@\" && posse_hs_refuse hookskip:no-verify '%s skips its hooks' ;;\n",
			v, shIdent("posse_hs_nv", v), strings.Join(hookSkipNoVerify(v).Opts, "/"))
	}
	b.WriteString("    config) posse_hs_config \"$@\" || posse_hs_refuse hookskip:config-write 'git config writes a hook-moving key' ;;\n")
	b.WriteString("  esac\n")
	verbs := make([]string, 0, len(hookSkipVerbs))
	for _, v := range hookSkipVerbs {
		verbs = append(verbs, shQuote(v))
	}
	// Recorded, not run: the env arm execs git to compare scopes, and that
	// must wait until every deny rule has had its say (renderHookSkipEnvCall).
	fmt.Fprintf(&b, "  case \"$posse_hs_verb\" in %s) posse_hs_envverb=1 ;; esac\n", strings.Join(verbs, "|"))
	b.WriteString("  return 0\n}\n")
	b.WriteString("posse_hs_argv=\"$*\"; posse_hs_envverb=\nposse_hs_check \"$@\"\n")
	return b.String()
}

// renderHookSkipEnvCall is the env arm's call site, emitted by renderShim
// AFTER the PID's deny rules. The argv checks above are pure sh and run first;
// this one execs the real git (posse_hs_scope) whenever a GIT_CONFIG_* scope
// variable is set — on a claude seat that is every call, since the inlet pin
// sets GIT_CONFIG_SYSTEM — and a command the PID refuses must never reach
// the real binary for any reason (TestRefusalNamesWhereTheCommandDoesRun
// caught exactly that: a denied push ran git config through the leak canary).
func renderHookSkipEnvCall() string {
	return "[ -n \"$posse_hs_envverb\" ] && posse_hs_env\n"
}

// HookSkipGate is the parity row the arm is reported under (CheckParity).
// It names no PID rule because it answers to none.
const HookSkipGate = "git hook-skip forms (-n/--no-verify, -c/env/config hooksPath)"
