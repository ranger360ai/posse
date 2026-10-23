#!/usr/bin/env python3
r"""python-syntax.py — every python file this repo tracks still COMPILES, under
the python its own shebang names, with no warning (ranger-base-66xfe).

  scripts/python-syntax.py              sweep this repo
  scripts/python-syntax.py --census     path<TAB>interpreter, one row per file
  scripts/python-syntax.py --self-test  prove the sweep can still say no
  scripts/python-syntax.py --root DIR   sweep DIR instead (--self-test uses it)

WHY THIS EXISTS. scripts/shell-syntax.sh is the shell half of this question
and has been wired as `make verify-shell-syntax` since ranger-base-g4z8m.
There was no python half, and a python file fails differently from a shell
one: an invalid escape sequence does not stop it parsing, it prints a
SyntaxWarning on stderr and keeps exit 0. So nothing in this tree could say
no to it, and nothing did.

THE DEFECT (ranger-base-cyx1z). scripts/pattern-kill-census.py:665-666 carried
two "\;" literals from the day those arms were written. Homebrew python3 went
to 3.15 on 2026-10-22 (ranger-base-sigtb) and they became visible — two
SyntaxWarnings on every run of the census, and on every `make test`, which
runs it as `verify-pattern-kill-census`. The gate stayed green the whole time,
because the gate was reading the exit code and the warning is not in it. The
only thing that caught the pair was a human reading stderr.

WHY A WARNING IS A FINDING HERE AND NOT A NOTE. The text says it: such
sequences "will not work in the future". Today a warning, in some later python
a SyntaxError — and then the green verify-* target that ran that script is a
hard red for every seat on the box, arriving with a version bump nobody
connects to it. The cheap moment to pay is now, while the whole class is
closed: MEASURED 2026-10-22, darwin arm64, python 3.15.0, 12 tracked python
files and after the cyx1z fix zero warnings across all of them. This is a
door, not a cleanup.

THE CORPUS IS TRACKED FILES, TWO RULES, transposed from shell-syntax.sh's:
every tracked *.py, plus every tracked file whose first line is a python
shebang. Either rule alone has a hole — a module that is imported and never
executed carries no shebang, and a script named for what it does rather than
for its language carries no .py.

Tracked, not found: `git ls-files` is how every other tree-wide pin here
defines the tree, and it is what ships. It also lists the staged half of a
work in progress, so `git add` then `make test` covers a script the same
commit is about to carry.

WHERE THIS DIVERGES FROM THE SHELL HALF, and why. There, a .sh with no shebang
is a FINDING: nothing can say which parser applies to it, so nothing can say
it parses. Here the extension already says which language, and a .py with no
shebang is the ordinary shape of an imported module — so it is swept under
`python3`, the box's own, rather than named. What IS a finding is a .py whose
shebang names something that is not a python: two things in the file disagree
about what it is, and nothing can say which one is right.

THE SHEBANG PICKS THE COMPILER, because the shebang is what picks the
interpreter at run time, and the warning is a property of the version. The
same "\;" is a DeprecationWarning before python 3.12 and a SyntaxWarning from
3.12 on; the sweep records whatever its interpreter raises and does not care
which name it has. All 12 of this repo's python files say
`#!/usr/bin/env python3` today, so the grouping costs one subprocess, and it
will name the box it disagrees on if one ever says otherwise.

EXIT STATUS, in the house form:
  0  every file in the corpus compiled, with no warning
  1  at least one FINDING — a file that does not compile, a file that compiles
     with a warning, or a .py whose shebang names a non-python
  2  NOTHING WAS MEASURED, or an ERROR: an empty corpus, a root that is no git
     work tree, a shebang naming an interpreter this box lacks, or a worker
     that did not come back with an answer. An empty corpus is not a pass; it
     is a green over an empty room.

READ-ONLY. `compile()` builds a code object and never executes it, and the
self-test has an arm that says so: a fixture whose body would drop a canary
file leaves none.

BOOTSTRAP. This file is in the corpus it sweeps, so it checks itself. If it
ever stops compiling it does not report that — it fails to start, loudly, with
a traceback, which is the same answer arriving by a different road.
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile

# The worker. It runs under the interpreter the shebang named, which is not
# necessarily the one running this file, so it is source text and not an
# import. `-I` is passed with it: isolated mode drops the cwd from sys.path and
# ignores PYTHONWARNINGS, so neither a planted module nor an inherited filter
# can move the verdict.
#
# simplefilter("always") and not "error": a warning turned into an exception
# stops the compile at the first one, and a file with three invalid escapes
# would then report one. Recording gets them all, with their line numbers.
WORKER = r'''
import json, sys, warnings

out = []
for path in sys.argv[1:]:
    try:
        src = open(path, "rb").read()
    except OSError as exc:
        out.append({"path": path, "kind": "error", "line": 0,
                    "cls": type(exc).__name__, "msg": str(exc)})
        continue
    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        try:
            compile(src, path, "exec")
        except SyntaxError as exc:
            out.append({"path": path, "kind": "finding", "line": exc.lineno or 0,
                        "cls": type(exc).__name__, "msg": exc.msg or str(exc)})
            continue
        except ValueError as exc:
            # source null bytes, and anything else compile() refuses without
            # calling it a SyntaxError
            out.append({"path": path, "kind": "finding", "line": 0,
                        "cls": type(exc).__name__, "msg": str(exc)})
            continue
    for w in caught:
        out.append({"path": path, "kind": "finding", "line": w.lineno or 0,
                    "cls": w.category.__name__, "msg": str(w.message)})
json.dump(out, sys.stdout)
'''

DEFAULT_INTERP = "python3"

NOTE = r"""
A warning here is a finding because its own text says the sequence "will not
work in the future": the file runs today and stops compiling on some later
python, and then whatever target runs it is red for every seat on the box.
The usual cause is a backslash in a plain string that python does not know —
"\;", "\d", "\s" in a regex written without the r prefix. Make it a raw
string, or double the backslash (ranger-base-cyx1z).
"""


def interp_of(line):
    """The PYTHON basename a shebang line names, or None.

    Answers for `#!/usr/bin/env python3`, `#!/usr/bin/python3.12`,
    `#!/usr/bin/env -S python3 -u` and `#!/usr/bin/env PY=1 python3`; answers
    None for a shell shebang and for a file with no shebang at all.
    """
    rest = line.rstrip("\r\n")
    if not rest.startswith("#!"):
        return None
    for word in rest[2:].split():
        base = word.rsplit("/", 1)[-1]
        if base == "env" or base.startswith("-") or "=" in base:
            continue
        if base.startswith("python"):
            return base
        return None
    return None


def first_line(path):
    """The first line, decoded forgivingly. A file with no trailing newline
    still yields its first line, and a binary blob yields something harmless
    rather than raising."""
    try:
        with open(path, "rb") as fh:
            raw = fh.readline(4096)
    except OSError:
        return ""
    return raw.decode("utf-8", "replace")


def is_worktree(root):
    try:
        done = subprocess.run(["git", "-C", root, "rev-parse",
                               "--is-inside-work-tree"],
                              capture_output=True, text=True)
    except OSError:
        return False
    return done.returncode == 0


def corpus(root):
    """[(path, interp_or_None)] — the tracked files to check, in git's order.

    interp is None only for a .py whose shebang names a non-python, which the
    sweep reports and the census prints as `?`.
    """
    try:
        done = subprocess.run(["git", "-C", root, "ls-files", "-z"],
                              capture_output=True, text=True)
    except OSError:
        return []
    if done.returncode != 0:
        return []
    out = []
    for name in done.stdout.split("\0"):
        if not name:
            continue
        full = os.path.join(root, name)
        if not os.path.isfile(full):
            continue
        head = first_line(full)
        said = interp_of(head)
        if name.endswith(".py"):
            # No shebang at all is the ordinary imported-module shape, so it
            # gets the box's python3. A shebang naming something else is the
            # file disagreeing with itself, and that is interp None.
            if said:
                out.append((name, said))
            elif not head.startswith("#!"):
                out.append((name, DEFAULT_INTERP))
            else:
                out.append((name, None))
        elif said:
            out.append((name, said))
    return out


def indent(text):
    return "".join("  %s\n" % line for line in text.splitlines())


def _run_worker(exe, root, paths):
    """[record] from one interpreter over one batch, or a string naming why
    there is no answer."""
    try:
        done = subprocess.run([exe, "-I", "-c", WORKER] + paths,
                              cwd=root, capture_output=True, text=True)
    except OSError as exc:
        return "could not be run: %s" % exc
    if done.returncode != 0:
        return "exited %d: %s" % (done.returncode,
                                  (done.stderr or done.stdout).strip())
    try:
        return json.loads(done.stdout)
    except ValueError:
        return "answered something that is not json: %r" % done.stdout[:400]


def _measure(root, entries, out):
    """[(path, line, cls, msg)] findings, and a count of ERRORs printed."""
    by_interp = {}
    for path, interp in entries:
        by_interp.setdefault(interp, []).append(path)

    findings = []
    errors = 0
    for interp in sorted(by_interp):
        paths = by_interp[interp]
        exe = shutil.which(interp)
        if not exe:
            for path in paths:
                out.write("ERROR %s\n  its shebang names %s, which is not on "
                          "PATH on this box\n" % (path, interp))
                errors += 1
            continue
        # Chunked so a corpus larger than this one cannot meet ARG_MAX.
        for i in range(0, len(paths), 200):
            got = _run_worker(exe, root, paths[i:i + 200])
            if isinstance(got, str):
                out.write("ERROR %s\n  %s\n" % (interp, got))
                errors += 1
                continue
            for rec in got:
                if rec["kind"] == "error":
                    out.write("ERROR %s\n  %s: %s\n"
                              % (rec["path"], rec["cls"], rec["msg"]))
                    errors += 1
                else:
                    findings.append((rec["path"], rec["line"], rec["cls"],
                                     rec["msg"]))
    return findings, errors


def census(root, out):
    if not is_worktree(root):
        sys.stderr.write("python-syntax.py: %s is not a git work tree\n" % root)
        return 2
    for path, interp in corpus(root):
        out.write("%s\t%s\n" % (path, interp or "?"))
    return 0


def sweep(root, out):
    if not is_worktree(root):
        sys.stderr.write("python-syntax.py: %s is not a git work tree\n" % root)
        return 2

    entries = corpus(root)
    if not entries:
        sys.stderr.write("python-syntax.py: no python file found under %s — "
                         "a sweep over nothing is not a pass\n" % root)
        return 2

    findings = []
    errors = 0
    checkable = []
    for path, interp in entries:
        if interp is None:
            out.write("FINDING %s\n  a .py whose shebang names no python — the "
                      "name and the shebang disagree, so nothing can say which "
                      "compiler applies\n" % path)
            findings.append(None)
        else:
            checkable.append((path, interp))

    got, errors = _measure(root, checkable, out)
    for path, line, cls, msg in got:
        out.write("FINDING %s:%s (%s)\n" % (path, line or "?", cls))
        out.write(indent(msg))
    findings.extend(got)

    if findings:
        out.write(NOTE)
    if errors:
        return 2
    if findings:
        return 1
    out.write("python-syntax.py: %d python files compile clean\n" % len(entries))
    return 0


# --- self-test --------------------------------------------------------------
#
# Each arm plants a throwaway git work tree and sweeps it with this same
# sweep(), so the fixture is measured by the producer rather than by a second
# reading of it. `git init` and `git add` only — nothing here commits, and no
# arm touches the repo it is run from.

class _SelfTest:
    def __init__(self, tmp):
        self.tmp = tmp
        self.ok = 0
        self.bad = 0
        self.ran = 0

    def repo(self, name):
        d = os.path.join(self.tmp, name)
        os.makedirs(d, exist_ok=True)
        subprocess.run(["git", "-C", d, "init", "-q"], capture_output=True)
        return d

    def add(self, d, rel, body):
        full = os.path.join(d, rel)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w") as fh:
            fh.write(body)
        os.chmod(full, 0o755)
        subprocess.run(["git", "-C", d, "add", "--", rel], capture_output=True)
        return full

    def sweep(self, d):
        import io
        buf = io.StringIO()
        rc = sweep(d, buf)
        return rc, buf.getvalue()

    def census(self, d):
        import io
        buf = io.StringIO()
        rc = census(d, buf)
        return rc, buf.getvalue()

    def say(self, good, what):
        self.ran += 1
        if good:
            self.ok += 1
            print("self-test PASS: %s" % what)
        else:
            self.bad += 1
            print("self-test FAIL: %s" % what)


# The probe for arm 1. Whether THIS python complains about an invalid escape
# is the interpreter's property and not this repo's, so the arm asks the
# interpreter and pins that the sweep answers the same — the shape
# shell-syntax.sh's arm 1 was rewritten into after ranger-base-ftr3e put
# ci.yml red on main for asserting one box's verdict everywhere. `-W error`
# and not `-W error::SyntaxWarning`: before python 3.12 the same literal
# raises DeprecationWarning, and an arm keyed on the 3.12 name would say the
# sweep invented a finding on an older box.
PROBE = ("import sys\n"
         "p = sys.argv[1]\n"
         "compile(open(p, 'rb').read(), p, 'exec')\n")


def _probe_warns(path):
    exe = shutil.which(DEFAULT_INTERP)
    if not exe:
        return None
    done = subprocess.run([exe, "-I", "-W", "error", "-c", PROBE, path],
                          capture_output=True, text=True)
    return done.returncode != 0


# The fixture bodies for arms 1 and 2, written raw and hoisted out of the arms
# rather than spelled inline. This file is in the corpus it sweeps, so a
# fixture whose backslash is one escape short plants an invalid escape in THIS
# file, and the sweep then reports its own instrument. Arm 1's fixture must
# CONTAIN a lone backslash before the semicolon; arm 2's must contain two.
ESCAPE_FIXTURE = ('#!/usr/bin/env python3\n'
                  + r'SEP = "a\;b"' + '\n'
                  'print(SEP)\n')
CONTROL_FIXTURE = ('#!/usr/bin/env python3\n'
                   + r'SEP = "a\\;b"' + '\n'
                   'print(SEP)\n')

ARMS = 10


def self_test():
    tmp = tempfile.mkdtemp(prefix="posse-python-syntax.")
    try:
        return _self_test(_SelfTest(tmp))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def _self_test(st):
    # Arm 1 — the bead shape. An invalid escape sequence in a plain string:
    # the file COMPILES and the warning is the whole of the finding, which is
    # the difference between this check and the shell one.
    d = st.repo("escape")
    path = st.add(d, "census.py", ESCAPE_FIXTURE)
    warns = _probe_warns(path)
    rc, out = st.sweep(d)
    if warns is None:
        st.say(False, "the arm-1 probe could not run: no %s on PATH"
                      % DEFAULT_INTERP)
    elif warns:
        st.say(rc == 1 and "FINDING" in out and "census.py" in out,
               "this python warns on an invalid escape, and the sweep reports "
               "it (exit 1); got exit %d, output: %s" % (rc, out))
    else:
        st.say(rc == 0 and "1 python files compile clean" in out,
               "this python is silent on an invalid escape, and the sweep "
               "invents no finding (exit 0); got exit %d, output: %s"
               % (rc, out))

    # Arm 2 — the control that must come out the other way: the same file, the
    # escape doubled. Without this, arm 1 passes on a sweep that reds
    # everything.
    d = st.repo("control")
    st.add(d, "census.py", CONTROL_FIXTURE)
    rc, out = st.sweep(d)
    st.say(rc == 0 and "1 python files compile clean" in out,
           "the same literal with the escape doubled is clean (exit 0); got "
           "exit %d, output: %s" % (rc, out))

    # Arm 3 — the wider class, so this is a compile check and not one pattern.
    d = st.repo("broken")
    st.add(d, "broken.py", '#!/usr/bin/env python3\ndef f(:\n    pass\n')
    rc, out = st.sweep(d)
    st.say(rc == 1 and "FINDING" in out and "broken.py" in out,
           "a real syntax error is caught; got exit %d, output: %s" % (rc, out))

    # Arm 4 — an empty corpus exits 2, not 0. A lister that stopped matching
    # would otherwise turn every arm above, and the tree sweep itself, green
    # on nothing at all.
    d = st.repo("empty")
    rc, out = st.sweep(d)
    st.say(rc == 2, "a corpus of nothing exits 2 rather than passing; got "
                    "exit %d, output: %s" % (rc, out))

    # Arm 5 — the corpus is not keyed on the extension. A tracked file with a
    # python shebang and no .py is swept too, which is the hole a name-keyed
    # census leaves.
    d = st.repo("noext")
    st.add(d, "bin/hook", '#!/usr/bin/env python3\nif True\n    pass\n')
    rc, out = st.sweep(d)
    st.say(rc == 1 and "bin/hook" in out and "SyntaxError" in out,
           "a python shebang without a .py is in the corpus, and is COMPILED "
           "there rather than merely named; got exit %d, output: %s"
           % (rc, out))

    # Arm 6 — and it is not keyed on the shebang either. A .py with none is an
    # imported module, swept under the box's python3 rather than skipped.
    d = st.repo("noshebang")
    st.add(d, "mod.py", 'def f(:\n    pass\n')
    rc, out = st.sweep(d)
    st.say(rc == 1 and "mod.py" in out and "SyntaxError" in out,
           "a .py with no shebang is COMPILED under the box python3, not "
           "skipped and not filed as a disagreement; got exit %d, output: %s"
           % (rc, out))

    # Arm 7 — a .py whose shebang names a NON-python is a finding. The two
    # rules above both claim this file and they disagree, and a sweep that
    # quietly picked one would be saying something it cannot know.
    d = st.repo("wrongbang")
    st.add(d, "odd.py", '#!/usr/bin/env bash\necho not python at all\n')
    rc, out = st.sweep(d)
    st.say(rc == 1 and "odd.py" in out and "names no python" in out,
           "a .py whose shebang names a shell is a finding that SAYS so, not "
           "a guess; got exit %d, output: %s" % (rc, out))

    # Arm 8 — a shebang naming a python this box lacks is an ERROR that names
    # it, not a silent pass and not a quiet fallback. Reached by scrubbing
    # PATH down to git — the one other command the sweep runs — rather than by
    # naming a version, because any version string this arm picked would
    # either exist on some box or stop existing on this one.
    d = st.repo("missing")
    st.add(d, "ok.py", '#!/usr/bin/env python3\nprint("hi")\n')
    nopy = os.path.join(st.tmp, "nopy")
    os.makedirs(nopy, exist_ok=True)
    git = shutil.which("git")
    link = os.path.join(nopy, "git")
    if git and not os.path.exists(link):
        os.symlink(git, link)
    saved = os.environ.get("PATH", "")
    os.environ["PATH"] = nopy
    try:
        rc, out = st.sweep(d)
    finally:
        os.environ["PATH"] = saved
    st.say(rc == 2 and "ERROR" in out and "python3" in out,
           "a shebang naming a python this box lacks is an ERROR (exit 2); "
           "got exit %d, output: %s" % (rc, out))

    # Arm 9 — the sweep COMPILES and does not RUN. compile() is the whole of
    # that promise, and a change from compile() to an exec or an import would
    # be green everywhere above.
    canary = os.path.join(st.tmp, "canary")
    d = st.repo("readonly")
    st.add(d, "effect.py", '#!/usr/bin/env python3\n'
                           'open(%r, "w").close()\n' % canary)
    rc, out = st.sweep(d)
    if os.path.exists(canary):
        st.say(False, "the sweep EXECUTED a file it was only asked to compile")
    else:
        st.say(rc == 0, "a swept file is compiled and not executed; got exit "
                        "%d, output: %s" % (rc, out))

    # Arm 10 — the shebang picks the compiler, and the census says which.
    # Pinning the MAPPING: every python on this box agrees about today's
    # corpus, so no fixture can tell two of them apart here.
    d = st.repo("interp")
    st.add(d, "named.py", '#!/usr/bin/env python3\nprint(1)\n')
    st.add(d, "bare.py", 'print(2)\n')
    st.add(d, "shelly.py", '#!/bin/sh\necho 3\n')
    rc, out = st.census(d)
    rows = set(out.splitlines())
    st.say(rc == 0 and {"named.py\tpython3", "bare.py\tpython3",
                        "shelly.py\t?"} <= rows,
           "the census follows the shebang, defaults a bare .py and marks a "
           "disagreement; got exit %d, output: %s" % (rc, out))

    # The reader on the arms themselves. st.ran counts the arms that actually
    # REPORTED, so an arm that returned early is the difference between this
    # number and ARMS, rather than a line of output nobody counted.
    if st.ran != ARMS:
        print("self-test FAIL: %d of %d arms reported; the rest never ran"
              % (st.ran, ARMS))
        return 1
    if st.bad:
        return 1
    print("self-test: %d arms, all clean" % st.ran)
    return 0


def main(argv=None):
    here = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--root", default=here, help="sweep DIR instead of this repo")
    ap.add_argument("--census", action="store_true",
                    help="path<TAB>interpreter, one row per file")
    ap.add_argument("--self-test", action="store_true",
                    help="prove the sweep can still say no")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    if args.census:
        return census(args.root, sys.stdout)
    return sweep(args.root, sys.stdout)


if __name__ == "__main__":
    sys.exit(main())
