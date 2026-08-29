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
# anywhere in the segment.
SUBDENY = {
    "dep": {"subs": {"relate"}, "opts": set()},
    "sync": {"subs": set(), "opts": {"--full"}},
    "config": {"subs": set(), "opts": set()},  # verb not allowed at all
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

# bd as a WORD in a line this parser reads only as text. Same expression the
# sh wrapper uses for its own fail-closed fallback; keep them spelled alike.
BD_WORD = re.compile(r"(^|[^A-Za-z0-9_.\-])bd([^A-Za-z0-9_\-]|$)")

GATE = "bd-argv-gate"


def segments(command):
    """Split a command line into pipeline/list segments, quotes respected.

    Returns (segment, opaque) pairs: `opaque` marks a segment this parser
    read only approximately (an unterminated quote, a substitution).
    """
    out, cur, quote, i = [], [], None, 0
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
        hit = next((s for s in SEPARATORS if command.startswith(s, i)), None)
        if hit:
            out.append("".join(cur))
            cur = []
            i += len(hit)
            continue
        cur.append(ch)
        i += 1
    out.append("".join(cur))
    return [(s, quote is not None) for s in out if s.strip()]


def tokens(segment):
    """Tokenize one segment. Returns None when it cannot be read."""
    stripped = segment.strip().lstrip("({ \t").rstrip(")} \t")
    try:
        return shlex.split(stripped)
    except ValueError:
        return None


def is_bd(word):
    """True when this command word names bd — by any path spelling."""
    return os.path.basename(word) == "bd"


def command_word(toks):
    """Resolve the command word, skipping assignments and wrappers.

    Returns (word, rest) or (None, None) when the segment has no command
    word this parser will vouch for.
    """
    i = 0
    while i < len(toks):
        t = toks[i]
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
    """Skip bd's global options; return (verb, rest) or (None, [])."""
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--":
            i += 1
            break
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
            bad = [t for t in tail if t in sub["opts"]]
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
        if BD_WORD.search(raw):
            sys.stderr.write("%s: could not read the tool call; refusing (fail closed)\n" % GATE)
            return 2
        return 0
    if not isinstance(command, str):
        if BD_WORD.search(raw):
            sys.stderr.write("%s: tool_input.command is not a string; refusing (fail closed)\n" % GATE)
            return 2
        return 0
    try:
        reason = verdict(command)
    except Exception as exc:            # a parser bug must not be an opening
        if BD_WORD.search(command):
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
