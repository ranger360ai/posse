#!/usr/bin/env python3
"""Apply JSON-escaped literal substitutions to a JSONL stream.

argv[1] is a pairs file: alternating lines, each a JSON string literal
(python-escaped, which is how queue-scrub.sh hands the values over).
  line 2n   = the OLD text (the container's current value)
  line 2n+1 = the NEW text (its replacement)

Both are re-escaped the way Go's encoding/json escapes a string, because the
file being edited is a bd export and bd is Go: non-ASCII stays raw UTF-8, and
`<`, `>`, `&`, U+2028 and U+2029 go to their \\uXXXX forms. Getting this wrong
does not corrupt anything -- the substring simply is not found -- so the
caller's hit count is the check, and a count of zero means the escaping, not
the absence of the text.

The replacement is a literal byte substitution inside the .jsonl: the record is
never reserialised, so a generated export stays byte-identical everywhere else.
Writes the stream to stdout, a count per pair to stderr.
"""
import json
import sys

_GO_ESCAPES = {
    "<": "\\u003c",
    ">": "\\u003e",
    "&": "\\u0026",
    " ": "\\u2028",
    " ": "\\u2029",
}
# python short-escapes these two; Go spells them out
_PY_ONLY = {"\\b": "\\u0008", "\\f": "\\u000c"}


def go_escape(s: str) -> str:
    """s as Go's encoding/json would write it, without the quotes."""
    out = json.dumps(s, ensure_ascii=False)[1:-1]
    for a, b in _PY_ONLY.items():
        out = out.replace(a, b)
    # applied after escaping: a literal backslash is already doubled, so no
    # character below can be part of an escape sequence we just produced.
    for a, b in _GO_ESCAPES.items():
        out = out.replace(a, b)
    return out


def main() -> int:
    with open(sys.argv[1], encoding="utf-8") as fh:
        lines = [ln.rstrip("\n") for ln in fh if ln.strip()]
    if len(lines) % 2:
        sys.exit("pairs file must hold an even number of JSON string literals")

    pairs = []
    for i in range(0, len(lines), 2):
        old = go_escape(json.loads(lines[i]))
        new = go_escape(json.loads(lines[i + 1]))
        if not old:
            sys.exit("empty OLD in pair %d" % (i // 2))
        pairs.append((old, new))

    counts = [0] * len(pairs)
    out = sys.stdout
    for line in sys.stdin:
        for idx, (old, new) in enumerate(pairs):
            if old in line:
                counts[idx] += line.count(old)
                line = line.replace(old, new)
        out.write(line)
    out.flush()
    for idx, n in enumerate(counts):
        print("pair %d: %d substitution(s)" % (idx, n), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
