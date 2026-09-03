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
entry appears, walk only that new subtree — then keep re-walking the entries
it has already seen, round-robin, aiming to complete one full sweep every
REWALK_SWEEP_SECONDS. The re-walk is not decoration (ranger-base-nunx9): a
single walk at first-sighting sees a directory at the instant it is listed,
so every executable written into it afterwards is invisible for the rest of
the run. `git init` writes its 14 hooks in one burst inside a directory it
has just created and is caught either way; `RenderGates` writes shims deep
inside a `t.TempDir()` that appeared seconds earlier and was missed entirely.
Measured before the re-walk landed: 2 of 4 planted executables reported, at
exit 0, with the control green.

Two controls, because one of them could not fail on the bug above:
  * `control_seen`      — a directory that appears WITH its executable in it.
                          Grades the walk.
  * `control_late_seen` — a directory that appears EMPTY and is populated a
                          second later. Grades the re-walk, and is the arm
                          that reads 0 on the pre-nunx9 code.

Dedupe is by (st_dev, st_ino), never by path: `RenderGates` removes its bin
dir and rewrites it, so the same path is a brand-new inode and a brand-new
assessment (nw9zg: the unit of cost is the file, not the path). A re-walk
that finds the same inode again counts it once.

What it still cannot see, stated rather than discovered: only entries that
APPEAR during the run are re-walked. A file written into a top-level entry
that already existed at t0 is missed, and on this box the baseline is ~31k
entries — sweeping those is the 116-second `find` this instrument exists to
replace. Point ROOT at a fresh tree, or read the numbers as a floor.

Usage:
    scripts/gk-inflight-census.py [ROOT] [SECONDS] [--paths OUT]

ROOT defaults to `getconf DARWIN_USER_TEMP_DIR`, SECONDS to 30 and must be at
least 10 for the late control to be gradeable. Prints a JSON summary; BOTH
`control_seen` and `control_late_seen` must be 1 or the reading means nothing.

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
CONTROL_LATE_PREFIX = "GK-INFLIGHT-CONTROL-LATE-"

# One full round-robin sweep of the entries seen so far, every this many
# seconds. Self-scaling: the per-entry interval is this divided by the number
# of known entries, so 316 new entries (the observed 30 s figure on $TMPDIR)
# re-walk 6 ms apart and three entries re-walk 0.7 s apart. It bounds
# detection latency for a late write at ~1 sweep regardless of population,
# and it is the reason this loop does not spin: generating load to measure
# this box is against standing orders.
REWALK_SWEEP_SECONDS = 2.0


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


def walk_execs(path, acc, when, seen_files, phase):
    """Append every not-yet-counted executable regular file under `path`.

    `seen_files` is a set of (st_dev, st_ino) — see the module docstring for
    why the key is the inode and not the path. `phase` is "first" for the walk
    at first sighting and "rewalk" for a round-robin re-visit; it is what
    separates the population the old single-walk poller could see from the one
    it could not.
    """
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
                    key = (st.st_dev, st.st_ino)
                    if key in seen_files:
                        continue
                    seen_files.add(key)
                    acc.append((when, fp, phase))
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

    if secs < 10:
        print(
            "SECONDS=%g is under 10: the late control is planted at the "
            "half-way mark and needs a sweep after it to be gradeable"
            % secs,
            file=sys.stderr,
        )

    baseline = len(seen)
    seen_files = set()
    execs = []
    new_entries = []
    # Only entries that APPEAR during the run are re-walked; see the module
    # docstring for why the ~31k baseline entries are not.
    rewalk_order = []
    rewalk_cursor = 0
    rewalks = 0
    polls = 0
    control_dir = None
    control_late_dir = None
    control_late_at = None
    plant_at = time.time() + secs / 2
    next_rewalk = time.time() + REWALK_SWEEP_SECONDS
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
            rewalk_order.append(name)
            walk_execs(os.path.join(root, name), execs, when, seen_files, "first")

        now = time.time()
        if rewalk_order and now >= next_rewalk:
            name = rewalk_order[rewalk_cursor % len(rewalk_order)]
            rewalk_cursor += 1
            rewalks += 1
            walk_execs(
                os.path.join(root, name), execs, now, seen_files, "rewalk"
            )
            next_rewalk = now + REWALK_SWEEP_SECONDS / len(rewalk_order)

        if control_dir is None and now >= plant_at:
            # Arm A: appears WITH its executable already in it. Graded by the
            # walk at first sighting.
            control_dir = os.path.join(root, CONTROL_PREFIX + str(os.getpid()))
            try:
                os.mkdir(control_dir)
                probe = os.path.join(control_dir, "control-probe.sh")
                with open(probe, "w") as fh:
                    fh.write("#!/bin/sh\nexit 0\n")
                os.chmod(probe, 0o755)
            except OSError:
                control_dir = ""
            # Arm B: appears EMPTY, populated a second later. Only a re-walk
            # can catch it, which is the whole point of this arm.
            control_late_dir = os.path.join(
                root, CONTROL_LATE_PREFIX + str(os.getpid())
            )
            try:
                os.mkdir(control_late_dir)
                control_late_at = now + max(0.25, min(1.0, secs / 12.0))
            except OSError:
                control_late_dir = ""
                control_late_at = None

        if control_late_at is not None and now >= control_late_at:
            control_late_at = None
            try:
                probe = os.path.join(control_late_dir, "control-late-probe.sh")
                with open(probe, "w") as fh:
                    fh.write("#!/bin/sh\nexit 0\n")
                os.chmod(probe, 0o755)
            except OSError:
                control_late_dir = ""

    # CONTROL_PREFIX is a prefix of CONTROL_LATE_PREFIX, so arm A must be
    # counted by exclusion or it swallows arm B.
    control_late_seen = sum(1 for _t, p, _ph in execs if CONTROL_LATE_PREFIX in p)
    control_seen = (
        sum(1 for _t, p, _ph in execs if CONTROL_PREFIX in p) - control_late_seen
    )
    samples = sum(1 for _t, p, _ph in execs if p.endswith(".sample"))
    late = sum(1 for _t, _p, ph in execs if ph == "rewalk")

    for d in (control_dir, control_late_dir):
        if not d:
            continue
        try:
            for dirpath, _dd, filenames in os.walk(d, topdown=False):
                for name in filenames:
                    os.remove(os.path.join(dirpath, name))
                os.rmdir(dirpath)
        except OSError:
            pass

    if paths_out:
        with open(paths_out, "w") as fh:
            for when, p, phase in execs:
                fh.write("%.3f\t%s\t%s\n" % (when, p, phase))

    print(
        json.dumps(
            {
                "root": root,
                "seconds": secs,
                "polls": polls,
                "baseline_entries": baseline,
                "new_top_level": len(new_entries),
                "new_exec_files": len(execs),
                # git init's hook samples: created, never executed, never
                # assessed. See docs/notes.d/ranger-base-fq3hc.md §3 — and
                # read the split there against the re-walk caveat in §1, since
                # the poller that produced it walked each entry once.
                "of_which_git_hook_samples": samples,
                "of_which_other": len(execs) - samples,
                # Files a re-walk found, i.e. written into a directory AFTER
                # it was first listed. The pre-nunx9 poller reported none of
                # these and did not know it.
                "of_which_found_by_rewalk": late,
                "rewalk_entries": len(rewalk_order),
                "rewalks": rewalks,
                "control_seen": control_seen,
                "control_late_seen": control_late_seen,
            },
            indent=1,
        )
    )
    if control_seen != 1 or control_late_seen != 1:
        which = []
        if control_seen != 1:
            which.append("the walk (control_seen=%d)" % control_seen)
        if control_late_seen != 1:
            which.append("the re-walk (control_late_seen=%d)" % control_late_seen)
        print(
            "control not caught by %s — this reading measured nothing"
            % " and ".join(which),
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
