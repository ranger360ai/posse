#!/usr/bin/env python3
"""The bd argv gate — a Claude Code PreToolUse hook that resolves bd's verb.

Why this exists (ranger-base-3bqn, from az93): a `Bash(bd <verb>:*)`
permission rule matches a TOKEN PREFIX of the typed command, so any of bd's
global flags in front of the verb moves the verb out of the prefix and every
such rule misses:

    bd daemon             --help   ->  refused by the rule
    bd --no-daemon daemon --help   ->  ran, exit 0        (measured, az93)

This hook parses the command instead of matching its prefix: it splits the
line into segments, resolves each segment's command word, and — where that
word is bd — skips bd's global options (consuming the four that take a
separate value) to find the real verb. The decision is then taken on the
resolved verb, which no reordering changes.

Contract (claude 2.1.251, read out of the shipped bundle):
  - stdin is the tool call as JSON; `tool_input.command` is the whole typed
    line, compound parts included.
  - stdout `hookSpecificOutput.permissionDecision: "deny"` blocks the call.
  - Emitting NO decision is not "allow": it falls through to the normal
    permission pipeline. That is what this hook does for everything it is
    not about, so it never widens anyone's grants.
  - A hook that exits non-zero (other than 2) is FAIL-OPEN — "show stderr to
    user only but continue with tool call". Exit 2 blocks. So every path in
    here that cannot reach a decision exits 2, and the sh wrapper beside this
    file covers the case where this file cannot run at all.

What it does NOT hold: a verb reached through shell indirection this file
cannot read (an alias, `eval`, a substitution, a script that calls bd). Those
are refused when bd is visible in the line, and are invisible when they are
not. This is an L0 politeness layer with a parser, not a cage — see ADR 0014
§5. The wall is the L1 shim (option-aware, every runtime) and the L2 cage.
"""

import json
import os
import re
import shlex
import sys

# ── Policy ───────────────────────────────────────────────────────────────

# bd 0.49.1 global options that take their value as a SEPARATE argument.
# These must be consumed in pairs or the value is mistaken for the verb
# (`bd --db /tmp/x daemon stop`). Booleans and `--opt=value` need no entry.
# MEASURED from `bd --help` on 0.49.1 (0d99d153).
BD_VALUE_OPTS = {"--actor", "--db", "--dolt-auto-commit", "--lock-timeout"}

# The allow-list. Default is deny: a verb absent here is refused, which is
# what closes the hidden-command hole (`bd daemons`, az93) that no enumerated
# deny list can close. Read-mostly verbs and single-issue mutations are here;
# anything that rewrites, deletes, migrates, repairs, configures, installs,
# or ships the store elsewhere is not.
ALLOWED = {
    "activity", "agent", "audit", "blocked", "children", "close", "comment",
    "comments", "count", "create", "create-form", "defer", "dep", "duplicate",
    "edit", "epic", "export", "formula", "gate", "graph", "help", "history",
    "human", "info", "label", "lint", "list", "mol", "move", "onboard",
    "orphans", "preflight", "prime", "q", "quickstart", "ready", "refile",
    "reopen", "resolve-conflicts", "restore", "search", "set-state", "show",
    "slot", "stale", "state", "status", "supersede", "swarm", "sync", "types",
    "undefer", "update", "upgrade", "version", "where", "worktree",
}

# Carve-outs inside an allowed verb, keyed by verb. Each entry names the
# subcommands that stay refused and the option tokens that stay refused
# anywhere in the segment. Spell an opt in its bare `--flag` form only: the
# scan in verdict() strips `=value`, so one entry covers both spellings.
SUBDENY = {
    "dep": {"subs": {"relate"}, "opts": set()},
    "sync": {"subs": set(), "opts": {"--full"}},
}

# Verbs whose refusal has a standing reason worth printing.
WHY = {
    "daemon": "daemon lifecycle is the operator's (bd 0.49.1 leaks one per db)",
    "daemons": "hidden plural of `daemon`; `stop`/`killall` reach the fleet's live daemons",
    "admin": "`admin reset` removes all beads data and configuration",
    "delete": "deletes issues and rewrites references",
    "doctor": "`--fix`/`--clean`/`--source=jsonl` repair, migrate and delete",
    "hook": "runs git-hook logic incl. a JSONL import over the db",
    "hooks": "installs/removes bd's git hooks",
    "import": "ingests foreign rows into the store",
    "init": "re-initializes the store",
    "migrate": "database migration",
    "rename": "rewrites an issue ID and every reference to it",
    "rename-prefix": "rewrites the prefix of every issue in the database",
    "repair": "rewrites the database",
    "repo": "mutates multi-repo hydration config",
    "federation": "peer-to-peer federation",
    "config": "`config set`/`config unset` mutate the store's configuration",
    "jira": "synchronizes the private store to an external tracker (egress)",
    "linear": "synchronizes the private store to an external tracker (egress)",
    "mail": "delegates to an external mail provider (egress)",
    "setup": "writes agent instruction files",
    "ship": "publishes a capability for cross-project dependencies",
    "duplicates": "can merge issues in bulk",
}

# Command words that wrap another command; the real command word is the next
# token that is not one of their own options.
WRAPPERS = {"env", "command", "builtin", "exec", "nohup", "time", "nice",
            "stdbuf", "setsid", "doas", "sudo"}

# Constructs whose contents this parser cannot see. If bd is mentioned in a
# segment carrying one of these, the segment is refused rather than guessed.
OPAQUE = ("$(", "`", "${", "eval ", "eval\t", "<(", ">(")

SEPARATORS = ("&&", "||", ";;", ";", "|", "&", "\n")

# A redirection as shlex hands it over: optional fd digits, the operator, and
# the target either glued on (`2>&1`, `>/tmp/o`) or standing as the next token
# (`> /tmp/o`). Punctuation, never the thing being run — before ranger-base-4txk
# neither resolver skipped it, so `bd --help > /tmp/o` resolved to the verb `>`
# and was refused, while `> /tmp/o bd daemon stop` resolved to the command word
# `>` and was WAVED THROUGH (both MEASURED against the shipped gate).
REDIRECT = re.compile(r"^\d*(?:>>|>\||>&|>|<<<|<<|<&|<>|<)")

# What a command substitution collapses to for the tokenizer. shlex has no
# idea `$(a | b)` is one word, so its contents used to tokenize into command
# words of their own.
SUBST_WORD = "$__subst__"

# bd as a WORD in a line this parser reads only as text. Same expression the
# sh wrapper uses for its own fail-closed fallback; keep them spelled alike.
BD_WORD = re.compile(r"(^|[^A-Za-z0-9_.\-])bd([^A-Za-z0-9_\-]|$)")

STRIP_QUOTING = {ord(c): None for c in "\\'\""}

GATE = "bd-argv-gate"


def mentions_bd(command):
    """True when a DECODED command line names bd as a word.

    Two spellings, because the shell has two: written out, or concatenated out
    of quoting the parser would have resolved with shlex — `b\\d`, `b''d`,
    `b"d"` (ranger-base-hthx).
    """
    return bool(BD_WORD.search(command)) or bool(
        BD_WORD.search(command.translate(STRIP_QUOTING)))


def mentions_bd_in_payload(raw):
    """The same question of a payload nobody could decode — escapes and all.

    Reached only where this file has already failed to read the tool call, so
    the JSON escapes are still sitting there as text. Testing that text
    directly is what made the sh wrapper fail OPEN on any bd call that was not
    on the FIRST line of the command (ranger-base-1lvm): the harness writes the
    newline as `\\` `n`, and the `n` reads as a word character in front of the
    `bd`. Decode, then ask.

    `str.replace` and `re.sub` both scan left to right over non-overlapping
    matches, which is exactly a JSON string decoder's scan: `\\\\n` is consumed
    as the escaped backslash and never mistaken for the newline escape. Keep
    this spelled like the sed pipeline in bd-argv-gate.sh — the wrapper's
    fallback and this path have to agree, and `make verify-bd-argv-gate` sweeps
    them against each other.
    """
    def decode_u(t):
        # \u0062 and \u0064 are the ONLY four-hex spellings that can produce a
        # `b` or a `d`, so everything else becomes a separator with no loss.
        t = t.replace("\\u0062", "b").replace("\\u0064", "d")
        return re.sub(r"\\u....", " ", t, flags=re.S)

    # A. a literal bd: every escape becomes a separator, which can only create
    #    word boundaries and can never hide a `bd`.
    separated = re.sub(r"\\.", " ", decode_u(raw.replace("\\\\", " ")), flags=re.S)
    # B. a concatenated bd: `\\` and `\"` decode to a backslash and a quote,
    #    which the strip then deletes, so the concatenation survives.
    joined = re.sub(r"\\.", " ",
                    decode_u(raw.replace("\\\\", "")).replace('\\"', ""), flags=re.S)
    # C. a backslash we get here can be a JSON escape (A and B) or, in a payload
    #    nobody encoded, the shell's own quoting — and having failed to read the
    #    payload we cannot tell which. So ask under the other reading too. This
    #    is the test that shipped before; it costs nothing on top of A and B for
    #    a real JSON payload (MEASURED: A|B is a strict superset over 47275 real
    #    command lines) and it is what still catches `b\d` in a payload that is
    #    not JSON at all.
    return (bool(BD_WORD.search(separated))
            or bool(BD_WORD.search(joined.translate(STRIP_QUOTING)))
            or bool(BD_WORD.search(raw.translate(STRIP_QUOTING))))


def in_redirect(cur, command, i):
    """True when the `&`/`|` at command[i] is redirection, not a separator.

    `2>&1` is one redirection; splitting it left the fragment `bd 2>` behind,
    whose resolved verb was `2>` (ranger-base-4txk). `&>file` and `>|file` are
    the same mistake spelled differently.
    """
    if cur and cur[-1] in "<>":
        return True                     # 2>&1, >&2, >|f, <&3
    return command.startswith("&>", i)  # &>f, &>>f


def segments(command):
    """Split a command line into pipeline/list segments, quotes respected.

    Returns (segment, opaque) pairs: `opaque` marks a segment this parser
    read only approximately (an unterminated quote, a substitution).

    A command substitution is NOT split: its contents are one word to the
    shell, and tearing them apart at their pipes invented fragments whose
    command word was whatever followed — `$PATH` in the fleet's standard
    PATH-stripping preamble, which the `$`-variable arm then refused on any
    line that also named bd (ranger-base-4txk). `(subshell)` groups are
    deliberately still split: they are real segments, and tokens() strips
    their parens.
    """
    out, cur, quote, depth, tick, i = [], [], None, 0, False, 0
    while i < len(command):
        ch = command[i]
        if quote:
            cur.append(ch)
            if ch == quote:
                quote = None
            elif ch == "\\" and quote == '"' and i + 1 < len(command):
                i += 1
                cur.append(command[i])
            i += 1
            continue
        if ch in "'\"":
            quote = ch
            cur.append(ch)
            i += 1
            continue
        if ch == "\\" and i + 1 < len(command):
            cur.append(ch)
            i += 1
            cur.append(command[i])
            i += 1
            continue
        if command.startswith(("$(", "<(", ">("), i):
            depth += 1
            cur.append(command[i:i + 2])
            i += 2
            continue
        if depth:
            depth += {"(": 1, ")": -1}.get(ch, 0)
            cur.append(ch)
            i += 1
            continue
        if ch == "`":
            tick = not tick
            cur.append(ch)
            i += 1
            continue
        if tick:
            cur.append(ch)
            i += 1
            continue
        hit = next((s for s in SEPARATORS if command.startswith(s, i)), None)
        if hit in ("&", "&&", "|", "||") and in_redirect(cur, command, i):
            hit = None
        if hit:
            out.append("".join(cur))
            cur = []
            i += len(hit)
            continue
        cur.append(ch)
        i += 1
    out.append("".join(cur))
    return [(s, quote is not None) for s in out if s.strip()]


def mask_subst(text):
    """Collapse each UNQUOTED command substitution to one opaque word.

    shlex tokenizes on whitespace and knows nothing of `$( )`, so
    `NEWPATH=$(echo "$PATH" | tr ':' '\\n')` came apart into `NEWPATH=$(echo`,
    `$PATH`, `|`, … and the resolver read `$PATH` as a command word
    (ranger-base-4txk). A QUOTED substitution needs no masking — shlex already
    keeps it in one token — and the contents are unreadable to this parser
    either way: OPAQUE refuses any segment carrying one that mentions bd, so
    what is masked here is only ever a segment bd is not in.

    The placeholder keeps its `$`, so a substitution standing where a command
    word goes is still an indirection this gate will not vouch for.
    """
    out, quote, depth, i = [], None, 0, 0
    while i < len(text):
        ch = text[i]
        keep = depth == 0
        if quote:
            if keep:
                out.append(ch)
            if ch == quote:
                quote = None
            elif ch == "\\" and quote == '"' and i + 1 < len(text):
                i += 1
                if keep:
                    out.append(text[i])
            i += 1
            continue
        if ch in "'\"":
            quote = ch
            if keep:
                out.append(ch)
            i += 1
            continue
        if ch == "\\" and i + 1 < len(text):
            if keep:
                out.append(text[i:i + 2])
            i += 2
            continue
        if depth == 0 and text.startswith(("$(", "<(", ">("), i):
            depth = 1
            out.append(SUBST_WORD)
            i += 2
            continue
        if depth:
            depth += {"(": 1, ")": -1}.get(ch, 0)
            i += 1
            continue
        if ch == "`":                   # the older spelling of the same thing
            end = text.find("`", i + 1)
            out.append(SUBST_WORD)
            i = len(text) if end < 0 else end + 1
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def tokens(segment):
    """Tokenize one segment. Returns None when it cannot be read."""
    stripped = mask_subst(segment).strip().lstrip("({ \t").rstrip(")} \t")
    try:
        return shlex.split(stripped)
    except ValueError:
        return None


def is_bd(word):
    """True when this command word names bd — by any path spelling."""
    return os.path.basename(word) == "bd"


def skip_redirect(toks, i):
    """Index past the redirection at toks[i], or None when it is not one."""
    m = REDIRECT.match(toks[i])
    if not m:
        return None
    # A bare operator takes the NEXT token as its target (`> /tmp/o`); a glued
    # one carries its own (`>/tmp/o`, `2>&1`). Consuming the target matters for
    # the fence, not just for tidiness: without it `bd > /tmp/o daemon stop`
    # resolves to the verb `/tmp/o`, which is not `daemon`.
    return i + (1 if m.end() < len(toks[i]) else 2)


def command_word(toks):
    """Resolve the command word, skipping redirections, assignments, wrappers.

    Returns (word, rest) or (None, None) when the segment has no command
    word this parser will vouch for.
    """
    i = 0
    while i < len(toks):
        t = toks[i]
        nxt = skip_redirect(toks, i)
        if nxt is not None:
            i = nxt                     # `> /tmp/o bd daemon stop`
            continue
        if "=" in t and not t.startswith("-") and t.split("=", 1)[0].isidentifier():
            i += 1                      # FOO=bar bd …
            continue
        if os.path.basename(t) in WRAPPERS:
            i += 1                      # env -i, nice -n 5, …
            while i < len(toks) and toks[i].startswith("-"):
                # `env -u NAME` / `nice -n 5` take a value; `env -i` does not.
                if toks[i] in ("-u", "-n", "-S", "-C") and i + 1 < len(toks):
                    i += 2
                else:
                    i += 1
            continue
        return t, toks[i + 1:]
    return None, None


def resolve_verb(args):
    """Skip bd's global options and redirections; (verb, rest) or (None, [])."""
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--":
            i += 1
            break
        nxt = skip_redirect(args, i)
        if nxt is not None:
            i = nxt                     # `bd --help > /tmp/o` is still usage
            continue
        if a in BD_VALUE_OPTS:
            if i + 1 >= len(args):
                return None, []         # dangling; bd's own usage error
            i += 2
            continue
        if a.startswith("-"):
            i += 1
            continue
        break
    if i >= len(args):
        return None, []
    return args[i], args[i + 1:]


def verdict(command):
    """Return None to stay out of the way, or a refusal reason."""
    # Whether bd is named ANYWHERE on the line decides how suspicious an
    # indirection is: `$PYTHON script.py` is someone's build, `BD=bd; $BD
    # daemon stop` is this gate's business (MEASURED escaping an earlier cut).
    line_mentions_bd = bool(BD_WORD.search(command))
    for segment, unterminated in segments(command):
        toks = tokens(segment)
        # A quoted `sh -c "bd daemon stop"` is ONE token whose basename is not
        # bd, so bd-ness is read off the segment TEXT, not off the tokens.
        mentions_bd = bool(BD_WORD.search(segment))
        if unterminated or toks is None:
            if mentions_bd:
                return ("this segment could not be parsed (%s) and mentions bd: %s"
                        % ("unterminated quote" if unterminated else "bad quoting",
                           segment.strip()))
            continue
        if any(o in segment for o in OPAQUE) and mentions_bd:
            return ("bd behind a construct this gate cannot read (%s); "
                    "type the command directly: %s"
                    % (next(o for o in OPAQUE if o in segment).strip(), segment.strip()))
        word, rest = command_word(toks)
        if word is None:
            continue
        if word.startswith("$") and line_mentions_bd:
            return ("bd behind a variable this gate cannot resolve (%s); "
                    "type the command directly: %s" % (word, segment.strip()))
        if not is_bd(word):
            # bd as a later word of some other command (`sh -c 'bd …'`,
            # `xargs bd …`) is indirection this parser cannot follow.
            if mentions_bd and os.path.basename(word) in ("sh", "bash", "zsh", "xargs", "watch", "script"):
                return ("bd behind %s, which this gate cannot follow: %s"
                        % (os.path.basename(word), segment.strip()))
            continue
        verb, tail = resolve_verb(rest)
        if verb is None:
            continue                    # `bd`, `bd --json` — bd prints usage
        if verb not in ALLOWED:
            why = WHY.get(verb)
            return ("`bd %s` is not on the gate's allow-list%s"
                    % (verb, " — " + why if why else ""))
        sub = SUBDENY.get(verb)
        if sub:
            nxt, _ = resolve_verb(tail)
            if nxt in sub["subs"]:
                return "`bd %s %s` is refused" % (verb, nxt)
            # `--flag=value` is the SAME FLAG to pflag, so an exact-token
            # scan missed it: `sync --full` was refused and `sync --full=true`
            # — pull, merge, export, commit, PUSH — was waved through
            # (MEASURED, ranger-base-il8u; `list --limit 1 --json=true` prints
            # exactly what `--json` prints). The L1 shim grew the same arm one
            # layer down (ranger-base-vct2); this is that hole at the layer the
            # operator's PreToolUse hook actually runs.
            #
            # The value is NOT read. `--full=false` disables the flag, but
            # deciding that means reimplementing strconv.ParseBool's spellings
            # of false, and a fence that argues about truthiness is one
            # respelling from being wrong. So every `--full=...` is refused and
            # the bare flag is the spelling that works — the wall standing
            # wider than the rule, which is the same trade vct2 made.
            #
            # Prefix abbreviation is not a third spelling to cover: pflag
            # rejects it. `list --limit 1 --jso` is "Error: unknown flag:
            # --jso" (MEASURED, 0.49.1) — unlike git's parse-options, where
            # every unambiguous prefix is the flag.
            bad = [t for t in tail
                   if t in sub["opts"] or t.split("=", 1)[0] in sub["opts"]]
            if bad:
                return "`bd %s %s` is refused" % (verb, bad[0])
    return None


def main():
    raw = sys.stdin.read()
    try:
        payload = json.loads(raw)
        # Registered with matcher "Bash", so this is belt-and-braces: a call
        # for another tool is not this gate's business at all.
        if payload.get("tool_name") not in (None, "Bash"):
            return 0
        command = payload["tool_input"]["command"]
    except Exception:
        # Cannot read the call. Fail closed only where bd is in play, so a
        # broken harness contract does not wedge every Bash call on the box.
        # The payload is undecoded text here, so the test has to decode it
        # (ranger-base-1lvm).
        if mentions_bd_in_payload(raw):
            sys.stderr.write("%s: could not read the tool call; refusing (fail closed)\n" % GATE)
            return 2
        return 0
    if not isinstance(command, str):
        if mentions_bd_in_payload(raw):
            sys.stderr.write("%s: tool_input.command is not a string; refusing (fail closed)\n" % GATE)
            return 2
        return 0
    try:
        reason = verdict(command)
    except Exception as exc:            # a parser bug must not be an opening
        # `command` is decoded here, but a concatenated spelling still hides
        # from a bare word match (ranger-base-hthx), so ask both ways.
        if mentions_bd(command):
            sys.stderr.write("%s: parser error (%s); refusing (fail closed)\n" % (GATE, exc))
            return 2
        return 0
    if reason is None:
        return 0                        # silence: the normal rules still apply
    json.dump({"hookSpecificOutput": {
        "hookEventName": "PreToolUse",
        "permissionDecision": "deny",
        "permissionDecisionReason":
            "%s: %s. The fence is on the RESOLVED verb, so reordering bd's "
            "global flags does not move it. If this verb is legitimate work, "
            "file it — the allow-list is scripts/bd-argv-gate.py in posse and "
            "the operator owns the edit." % (GATE, reason),
    }}, sys.stdout)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
