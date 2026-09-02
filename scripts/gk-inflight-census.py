#!/usr/bin/env python3
"""Census transient executable files in a temp root, by polling.

Why this exists (ranger-base-fq3hc, docs/notes.d/ranger-base-fq3hc.md):
a recursive `find` is NOT a usable census over $TMPDIR on this box. One pass
takes ~116 seconds over ~292k executable files, and `-newermt`/`-newercm` can
only ever match files that still exist when find reaches them. The population
that drives Gatekeeper churn here lives for milliseconds inside a t.TempDir()
that is removed at test end, so a 120-second find census reports ~zero and is
believed. This poller found 605 new executable files in 30 seconds where that
census found one in 120.

The method: listdir ONE directory (cheap, no stat), and the instant a new
entry appears, walk only that new subtree. It plants its own transient
executable halfway through and reports whether it caught it — a poller that
cannot be shown to detect is not evidence.

Usage:
    scripts/gk-inflight-census.py [ROOT] [SECONDS] [--paths OUT]

ROOT defaults to `getconf DARWIN_USER_TEMP_DIR`, SECONDS to 30. Prints a JSON
summary; `control_seen` must be 1 or the reading means nothing.

Pair it with the assessment rate over the same window:

    /usr/bin/log show --start '<t0>' --end '<t1>' --style compact \\
      --predicate 'process == "syspolicyd" AND eventMessage CONTAINS "GK performScan"' \\
      | grep -c performScan

Note `log` is a zsh builtin; spell /usr/bin/log. Read-only apart from the one
control directory it creates and removes under ROOT.
"""

import json
import os
import subprocess
import sys
import time

CONTROL_PREFIX = "GK-INFLIGHT-CONTROL-"


def default_root():
    try:
        out = subprocess.run(
            ["getconf", "DARWIN_USER_TEMP_DIR"], capture_output=True, text=True
        )
        if out.returncode == 0 and out.stdout.strip():
            return out.stdout.strip()
    except OSError:
        pass
    return os.environ.get("TMPDIR", "/tmp")


def walk_execs(path, acc, when):
    """Append every executable regular file under `path` to acc."""
    try:
        if not os.path.isdir(path):
            return
        for dirpath, _dirnames, filenames in os.walk(path):
            for name in filenames:
                fp = os.path.join(dirpath, name)
                try:
                    st = os.lstat(fp)
                except OSError:
                    continue
                if st.st_mode & 0o111 and os.path.isfile(fp):
                    acc.append((when, fp))
    except OSError:
        pass


def main(argv):
    args = [a for a in argv[1:] if not a.startswith("--")]
    # An empty ROOT means "use the default", not "read the empty path".
    root = args[0] if args and args[0] else default_root()
    secs = float(args[1]) if len(args) > 1 else 30.0
    paths_out = None
    if "--paths" in argv:
        paths_out = argv[argv.index("--paths") + 1]

    try:
        seen = set(os.listdir(root))
    except OSError as exc:
        print("cannot read root %s: %s" % (root, exc), file=sys.stderr)
        return 2

    baseline = len(seen)
    execs = []
    new_entries = []
    polls = 0
    control_dir = None
    plant_at = time.time() + secs / 2
    end = time.time() + secs

    while time.time() < end:
        polls += 1
        # Cap the poll rate. On a big root listdir alone paces this at ~35 Hz,
        # but on a small one the loop spins a whole core, and generating load
        # to measure this box is against standing orders.
        time.sleep(0.002)
        try:
            current = os.listdir(root)
        except OSError:
            continue
        for name in current:
            if name in seen:
                continue
            seen.add(name)
            when = time.time()
            new_entries.append((when, name))
            walk_execs(os.path.join(root, name), execs, when)
        if control_dir is None and time.time() >= plant_at:
            control_dir = os.path.join(root, CONTROL_PREFIX + str(os.getpid()))
            try:
                os.mkdir(control_dir)
                probe = os.path.join(control_dir, "control-probe.sh")
                with open(probe, "w") as fh:
                    fh.write("#!/bin/sh\nexit 0\n")
                os.chmod(probe, 0o755)
            except OSError:
                control_dir = ""

    control_seen = sum(1 for _t, p in execs if CONTROL_PREFIX in p)
    samples = sum(1 for _t, p in execs if p.endswith(".sample"))

    if control_dir:
        try:
            for dirpath, _d, filenames in os.walk(control_dir, topdown=False):
                for name in filenames:
                    os.remove(os.path.join(dirpath, name))
                os.rmdir(dirpath)
        except OSError:
            pass

    if paths_out:
        with open(paths_out, "w") as fh:
            for when, p in execs:
                fh.write("%.3f\t%s\n" % (when, p))

    print(
        json.dumps(
            {
                "root": root,
                "seconds": secs,
                "polls": polls,
                "baseline_entries": baseline,
                "new_top_level": len(new_entries),
                "new_exec_files": len(execs),
                # git init's 14 hook samples per repo: created, never executed,
                # never assessed. 88% of the population and 0% of the cost.
                "of_which_git_hook_samples": samples,
                "of_which_other": len(execs) - samples,
                "control_seen": control_seen,
            },
            indent=1,
        )
    )
    if control_seen != 1:
        print(
            "control not caught — this reading measured nothing", file=sys.stderr
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
