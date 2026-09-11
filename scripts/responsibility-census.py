#!/usr/bin/env python3
"""What a long Go file is actually made of: code, prose, and shell, measured.

WHY THIS EXISTS (ranger-base-zv1cu). Two outside reviews priced five files by
`wc -l` — gates.go 5,796, visibility.go 1,342, backup.go 1,249, ciwatch.go
1,202, govern.go 1,101 — and asked whether the next 400 lines in each should
be written. `wc -l` cannot answer that, because these files are not mostly
code: visibility.go is 410 lines of Go under 670 lines of comment and 185
lines of refusal prose, and nearly half of gates.go is a shell program
written inside Go string literals. A file whose growth is comment is a file
explaining an invariant; a file whose growth is raw string is a file that
grew a second language; a file whose growth is code is the only one growing
a subsystem. The census page needs the three counted apart. This is the
instrument — re-run it, the files move.

WHAT IT READS. Go source, nothing else. No build, no go/ast, no toolchain:
`python3 scripts/responsibility-census.py internal/posse/gates.go` from any
checkout, and any file may carry a `:START-END` suffix to count one region
(`internal/posse/gates.go:2572-5392`). Line numbers are 1-based and both
ends are inclusive, so they are the numbers an editor shows.

THE THREE THINGS IT HAD TO GET RIGHT, each of which was wrong in a draft and
each of which changed the answer:

  1. A BACKTICK IS NOT ALWAYS A STRING. Go's only multi-line literal is the
     backtick raw string, so finding those is the whole job — but gates.go
     writes `git commit -F -` inside line comments and '`' inside quoted
     strings, and a scanner that keys on the character alone reads the rest
     of the file as one 3,000-line literal. The scan is a state machine over
     line comments, block comments, interpreted strings, runes and raw
     strings, in that precedence, and a backtick only opens a literal from
     the code state.

  2. ONE LINE, ONE CLASS. A line can hold the end of a raw string and the
     start of code (`}` + `` ` `` + `+ ident +` is ordinary in this tree).
     Every such line is counted ONCE, under the first class that claims it,
     comment > rawstr > blank > code, so the four columns sum to the range's
     length and a total can be trusted.

  3. ONLY A LITERAL THAT SPANS LINES IS A SECOND LANGUAGE. A backtick
     string written on one line is a regex, a marker or a path — ordinary
     Go, counted as code. `rawstr` is the lines of the literals that carry
     newlines: the shell programs, nothing else.

  4. A BLANK LINE INSIDE A HEREDOC IS SHELL. Blank means whitespace-only in
     Go; a whitespace-only line inside a raw string is a blank line of the
     shell program being rendered, and counting it as Go blank credited
     gates.go with 1,200 lines of "whitespace" it does not have.

WHAT IT DOES NOT MEASURE. Statements, branches, coupling, or whether any of
it should exist. It counts maintenance surface by kind. The verdicts are the
page's (docs/notes.d/ranger-base-zv1cu.md); the numbers are this script's.
"""

import sys

CODE, LINE_COMMENT, BLOCK_COMMENT, RAW, STR, RUNE = range(6)


def classify(text):
    """Per-line class for a whole Go source: 'cmt', 'rawstr', 'blank', 'code'.

    Returns a list indexed by line number - 1.
    """
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    kinds = [None] * len(lines)
    state = CODE
    raw_start = 0  # first line of the raw string being scanned

    def mark(i, kind):
        # One line, one class: comment > rawstr > blank > code (note 2).
        order = {"cmt": 0, "rawstr": 1, "blank": 2, "code": 3}
        if kinds[i] is None or order[kind] < order[kinds[i]]:
            kinds[i] = kind

    for i, line in enumerate(lines):
        j = 0
        saw_code = False
        while j < len(line):
            c = line[j]
            nxt = line[j + 1] if j + 1 < len(line) else ""
            if state == CODE:
                if c == "/" and nxt == "/":
                    mark(i, "cmt")
                    state = LINE_COMMENT
                    break
                if c == "/" and nxt == "*":
                    mark(i, "cmt")
                    state = BLOCK_COMMENT
                    j += 2
                    continue
                if c == "`":
                    # Not marked yet: a literal that closes on this same
                    # line is ordinary code (note 3).
                    raw_start = i
                    state = RAW
                    j += 1
                    continue
                if c == '"':
                    state = STR
                elif c == "'":
                    state = RUNE
                if not c.isspace():
                    saw_code = True
                j += 1
                continue
            if state == BLOCK_COMMENT:
                mark(i, "cmt")
                if c == "*" and nxt == "/":
                    state = CODE
                    j += 2
                    continue
                j += 1
                continue
            if state == RAW:
                if c == "`":
                    if i > raw_start:
                        for k in range(raw_start, i + 1):
                            mark(k, "rawstr")
                    state = CODE
                    saw_code = True
                    j += 1
                    continue
                j += 1
                continue
            if state in (STR, RUNE):
                # Neither can span a line; an unterminated one is a compile
                # error, so the newline below returns the scan to code.
                saw_code = True
                if c == "\\":
                    j += 2
                    continue
                if (state == STR and c == '"') or (state == RUNE and c == "'"):
                    state = CODE
                j += 1
                continue
        if state == RAW and i > raw_start:
            mark(i, "rawstr")
        if state == LINE_COMMENT:
            state = CODE
        elif state in (STR, RUNE):
            state = CODE
        if kinds[i] is None:
            mark(i, "code" if saw_code or line.strip() else "blank")
        elif kinds[i] == "code" and not line.strip():
            mark(i, "blank")
    return kinds


def count(path, lo, hi):
    kinds = classify(open(path, encoding="utf-8").read())
    hi = min(hi, len(kinds))
    tally = {"cmt": 0, "rawstr": 0, "blank": 0, "code": 0}
    for i in range(lo - 1, hi):
        tally[kinds[i]] += 1
    return tally


def main(argv):
    if not argv:
        print(__doc__.strip().split("\n")[0], file=sys.stderr)
        print("usage: responsibility-census.py <file.go[:start-end]> ...", file=sys.stderr)
        return 2
    print("%-34s %6s %6s %6s %6s %6s" % ("file", "total", "cmt", "rawstr", "blank", "code"))
    for spec in argv:
        path, lo, hi = spec, 1, 1 << 30
        if ":" in spec:
            path, _, rng = spec.rpartition(":")
            a, _, b = rng.partition("-")
            lo, hi = int(a), int(b or a)
        t = count(path, lo, hi)
        total = t["cmt"] + t["rawstr"] + t["blank"] + t["code"]
        print("%-34s %6d %6d %6d %6d %6d" % (spec, total, t["cmt"], t["rawstr"], t["blank"], t["code"]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
