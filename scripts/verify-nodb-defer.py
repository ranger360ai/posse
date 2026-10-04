#!/usr/bin/env python3
# Does a `no-db: true` beads store still lose every defer DATE, and is any
# store on this box holding a dateless park? (ranger-base-bwp7h)
#
# THE DEFECT. On the pinned bd (etc/bd/version-pin.toml), a store configured
# `no-db: true` — JSONL as the source of truth, no database — accepts
# `bd defer <id> --until <date>`, prints `* Deferred <id>`, sets the record's
# status to "deferred", and writes NO `defer_until`. `bd update <id> --defer
# <date>` does the same. The value is accepted, reported as success, and
# dropped. The database class does not do this.
#
# WHY IT MATTERS AND WHY A DATE IS NOT DECORATION. posse's readers key the
# park on the DATE and never on the status string (internal/posse/beads.go
# BdIssue.DeferUntil, govern.go's G3 arm, interrupted.go deferredNow): a
# future date is an answer someone already gave and the answer is a date, and
# a past one is a park that expired with nobody revisiting it. bd itself
# re-surfaces neither — MEASURED 2026-10-03, a deferred bead with a date four
# weeks past is still absent from `bd ready` in BOTH store classes — so the
# date is the only thing that ever makes a park loud again. Dateless, a park
# is indefinite: silent on the day it was supposed to come back, and silent
# after.
#
# WHAT THE WORKAROUND RESTS ON, measured the same day on the pinned bd: the
# no-db READER is whole. A `defer_until` written into the JSONL by hand is
# printed by `bd show` ("Deferred: <date>"), emitted by `bd show --json` and
# `bd list --json`, and SURVIVES a later unrelated bd write to the same record
# (`bd comments add` rewrote the file and kept it). Only the defer WRITER is
# blind. So the hand-written date is durable — but `bd defer --until <new>`
# cannot move it either: re-deferring a record that already carries a date
# leaves the OLD date in place and reports success, which is the same bug
# wearing a worse face. In a no-db store, re-parking is a JSONL edit.
#
# THERE IS NO VERSION TO WAIT FOR. 0.50.3 is the last 0.50.x; the next release
# (0.51.0) is the Dolt-native cleanup, whose phases 5 and 6 "Remove JSONL sync
# layer" and "Remove SQLite backend entirely". MEASURED 2026-10-03: the 1.x
# binary on this box has no `--no-db` flag at all and answers `defer` on a
# `no-db: true` store with `Error: no beads database found`. So the mode this
# defect lives in does not exist past the pin — reporting it upstream asks for
# a fix to a deleted code path, and "park on the version that fixes it" and
# "take the store off no-db" are the same migration, which is the operator's.
#
# ─── the two arms ───────────────────────────────────────────────────────────
# A · CAPABILITY, always run. Builds a throwaway no-db store, defers a record
#     in it with the bd under test, and reads the JSONL back.
#       date absent   -> the defect is still present. The verdict this check
#                        exists to carry: every hand-written date and every
#                        note about one is still load-bearing.
#       date present  -> the defect is GONE: exit 1, loudly. Either the pin
#                        moved or something else is answering as `bd`, and the
#                        workaround can be retired — a check that stayed green
#                        through that would leave the shop hand-editing JSONL
#                        for a bug that no longer exists.
#       store refused -> the no-db MODE is gone (0.51.0 phase 5/6): exit 1.
#     It is an arm that must be able to come out either way, so --self-test
#     runs it against a stub that drops the date, a stub that writes it and a
#     stub that refuses, and requires a different verdict from each.
#
# B · STATE, over the stores named on argv or in $POSSE_NODB_STORES (colon-
#     separated; this repo is stamped public and names no private store's
#     path). Counts records with status "deferred" and no defer_until and
#     names their ids. Those are the indefinite parks: nothing re-surfaces
#     them and the only record of the intended date is wherever a human wrote
#     it down.
#     **It fails only for a store in no-db mode**, and that asymmetry is the
#     measurement, not a hedge. In a store with a database, dateless-deferred
#     is a sentence someone meant — `bd defer <id>` with no `--until` is the
#     icebox, and the queue store's own roadmap beads sit there on purpose.
#     In a no-db store the same record is ambiguous by construction: a park
#     somebody dated and a park somebody did not read back identically,
#     because the writer threw the date away. So a db store's dateless parks
#     are REPORTED and a no-db store's are a failure. A store whose
#     config.yaml cannot be read counts as no-db — the direction that is loud.
#     With no store named it says so and says that it is NOT a pass — a
#     verdict over zero stores is the shape that reads green while a failed
#     producer hands it nothing.
#
#     `--arm-a-only` runs A and not B, for the door in a checkout that has no
#     store to name — the stores on this box are private and this repo is
#     stamped public, so `make verify-nodb-defer` would otherwise be exit 2
#     every time it was typed. It says in its own output that no store was
#     read. Handed a store anyway, on argv or in the environment, it is
#     REFUSED (exit 2) rather than skipping it: a flag that can quietly drop
#     a store is the zero-stores trap wearing a different hat.
#
# Read-only except for its own temp dir. It defers nothing anybody owns: arm A
# builds its store from scratch and removes it, arm B only reads.

import contextlib
import json
import os
import shutil
import subprocess
import sys
import tempfile

PIN = "etc/bd/version-pin.toml"

# Arm A's verdicts.
DEFECT_PRESENT = "defect-present"
DEFECT_GONE = "defect-gone"
MODE_GONE = "mode-gone"
UNUSABLE = "unusable"


def run(argv, cwd=None, env=None):
    """Exit status, stdout+stderr. No shell, no timeout(1) (there is none on
    this box); bd's own calls here are sub-second and never block on a lock,
    because the store arm A hands it is one nothing else has ever opened."""
    p = subprocess.run(argv, cwd=cwd, env=env, stdout=subprocess.PIPE,
                       stderr=subprocess.STDOUT, text=True)
    return p.returncode, p.stdout


def read_records(path):
    """(records, malformed-line-count). A malformed line is counted, never
    skipped silently — arm B's whole output is a count."""
    recs, bad = [], 0
    try:
        raw = open(path, encoding="utf-8", errors="replace").read()
    except OSError as e:
        return None, str(e)
    for line in raw.split("\n"):
        if not line.strip():
            continue
        try:
            recs.append(json.loads(line))
        except ValueError:
            bad += 1
    return recs, bad


# ─── arm A ──────────────────────────────────────────────────────────────────

def arm_a(bd, out):
    """Build a no-db store, defer in it, say what came back."""
    root = tempfile.mkdtemp(prefix="nodb-defer-")
    try:
        beads = os.path.join(root, ".beads")
        os.mkdir(beads)
        os.chmod(beads, 0o700)
        # bd resolves .beads from the GIT ROOT and follows one redirect hop,
        # so an un-init'd temp dir inside a worktree would resolve to the
        # worktree's own store. Hence git init, and hence the canary below:
        # the arm refuses to run against a store it did not build.
        rc, o = run(["git", "init", "-q", root])
        if rc != 0:
            print("  arm A: git init failed in the scratch dir: %s" % o.strip(), file=out)
            return UNUSABLE
        with open(os.path.join(beads, "config.yaml"), "w") as fh:
            fh.write('issue-prefix: "ndf"\nno-db: true\n')
        rec = {"id": "ndf-1", "title": "a park with a date",
               "status": "open", "priority": 0, "issue_type": "task",
               "created_at": "2026-01-01T00:00:00-04:00",
               "updated_at": "2026-01-01T00:00:00-04:00",
               "labels": ["question"]}
        jsonl = os.path.join(beads, "issues.jsonl")
        with open(jsonl, "w") as fh:
            fh.write(json.dumps(rec, separators=(",", ":")) + "\n")

        env = dict(os.environ)
        # BEADS_DIR would point every call at the store of record for the
        # directory this session was launched into (ADR 0055), which is
        # exactly the store this arm must not touch.
        env.pop("BEADS_DIR", None)
        env.pop("BEADS_DB", None)

        rc, o = run([bd, "where"], cwd=root, env=env)
        if beads not in o:
            print("  arm A: could not isolate a scratch store — `bd where` in "
                  "the scratch dir did not name it. Refusing to defer anything.",
                  file=out)
            print("         bd where said: %s" % " ".join(o.split())[:200], file=out)
            return UNUSABLE

        rc, o = run([bd, "defer", "ndf-1", "--until", "2099-01-01"], cwd=root, env=env)
        said = " ".join(o.split())
        if "no beads database found" in o:
            print("  arm A: this bd refuses a `no-db: true` store outright — the "
                  "JSONL-only MODE is gone (0.51.0 phase 5/6), not just the "
                  "defer date.", file=out)
            print("         bd said: %s" % said[:200], file=out)
            return MODE_GONE

        recs, bad = read_records(jsonl)
        if recs is None:
            print("  arm A: could not read the scratch JSONL back: %s" % bad, file=out)
            return UNUSABLE
        if len(recs) != 1:
            print("  arm A: scratch store holds %d records, expected 1 — not a "
                  "store this arm built." % len(recs), file=out)
            return UNUSABLE
        got = recs[0]
        date = got.get("defer_until")
        print("  arm A: 1 record, `bd defer --until 2099-01-01` exit %d, said %r"
              % (rc, said[:60]), file=out)
        print("         status=%r defer_until=%r" % (got.get("status"), date), file=out)
        if date:
            return DEFECT_GONE
        return DEFECT_PRESENT
    finally:
        shutil.rmtree(root, ignore_errors=True)


# ─── arm B ──────────────────────────────────────────────────────────────────

def arm_b(stores, out):
    """Dateless parks per store.

    Returns (nodb_findings, db_findings, stores-read, records-read). Only the
    first list is a failure; see the header.
    """
    nodb_findings = []
    db_findings = []
    read = 0
    total = 0
    for store in stores:
        path = store
        if os.path.isdir(path):
            if os.path.basename(path.rstrip("/")) != ".beads":
                path = os.path.join(path, ".beads")
            path = os.path.join(path, "issues.jsonl")
        recs, bad = read_records(path)
        if recs is None:
            print("  arm B: %s — unreadable: %s" % (path, bad), file=out)
            continue
        read += 1
        total += len(recs)
        nodb = is_nodb(path)
        mode = "no-db: true" if nodb else "has a db"
        if nodb is None:
            nodb, mode = True, "mode UNKNOWN, read as no-db"
        deferred = [r for r in recs if r.get("status") == "deferred"]
        dateless = [r for r in deferred if not r.get("defer_until")]
        print("  arm B: %s — %d records%s, %d deferred, %d of those DATELESS   [%s]"
              % (path, len(recs), "" if not bad else " (+%d malformed)" % bad,
                 len(deferred), len(dateless), mode),
              file=out)
        for r in dateless:
            (nodb_findings if nodb else db_findings).append(
                (path, r.get("id"), r.get("title", "")))
    return nodb_findings, db_findings, read, total


def is_nodb(jsonl_path):
    cfg = os.path.join(os.path.dirname(jsonl_path), "config.yaml")
    try:
        for line in open(cfg, encoding="utf-8", errors="replace"):
            s = line.strip()
            if s.startswith("#"):
                continue
            if s.startswith("no-db:"):
                return s.split(":", 1)[1].split("#")[0].strip() == "true"
    except OSError:
        return None
    return False


# ─── self-test ──────────────────────────────────────────────────────────────

STUB = """#!/usr/bin/env python3
import json, os, sys
mode = %r
a = sys.argv[1:]
if a and a[0] == 'where':
    print(os.path.join(os.getcwd(), '.beads'))
    sys.exit(0)
if mode == 'refuse':
    sys.stderr.write('Error: no beads database found\\n')
    sys.exit(1)
p = os.path.join(os.getcwd(), '.beads', 'issues.jsonl')
r = json.loads(open(p).read().strip())
r['status'] = 'deferred'
if mode == 'writes-date':
    r['defer_until'] = '2099-01-01T00:00:00-04:00'
open(p, 'w').write(json.dumps(r, separators=(',', ':')) + '\\n')
print('* Deferred ' + r['id'])
"""


def write_stub(d, name, mode):
    p = os.path.join(d, name)
    with open(p, "w") as fh:
        fh.write(STUB % mode)
    os.chmod(p, 0o755)
    return p


def self_test(out):
    """Every arm, with the case that would have let it pass wrongly."""
    fails = []

    def check(label, got, want):
        ok = got == want
        print("  %-52s %s (got %r)" % (label, "ok" if ok else "FAIL", got), file=out)
        if not ok:
            fails.append(label)

    d = tempfile.mkdtemp(prefix="nodb-defer-selftest-")
    devnull = open(os.devnull, "w")
    try:
        # arm A, three stubs, three verdicts. A rig that cannot tell them
        # apart would report "defect present" over a bd that fixed it.
        check("arm A vs a bd that drops the date",
              arm_a(write_stub(d, "bd-drops", "drops"), devnull), DEFECT_PRESENT)
        check("arm A vs a bd that writes the date",
              arm_a(write_stub(d, "bd-writes", "writes-date"), devnull), DEFECT_GONE)
        check("arm A vs a bd that refuses a no-db store",
              arm_a(write_stub(d, "bd-refuses", "refuse"), devnull), MODE_GONE)

        # arm B: planted stores.
        def plant(name, recs, nodb=True):
            b = os.path.join(d, name, ".beads")
            os.makedirs(b, exist_ok=True)
            with open(os.path.join(b, "config.yaml"), "w") as fh:
                fh.write('issue-prefix: "x"\n' + ("no-db: true\n" if nodb else ""))
            with open(os.path.join(b, "issues.jsonl"), "w") as fh:
                for r in recs:
                    fh.write(json.dumps(r, separators=(",", ":")) + "\n")
            return os.path.join(d, name)

        def rec(i, status, date=None):
            r = {"id": i, "title": "t", "status": status}
            if date:
                r["defer_until"] = date
            return r

        two = plant("two-dateless", [rec("a-1", "deferred"), rec("a-2", "deferred"),
                                     rec("a-3", "deferred", "2026-10-09T00:00:00-04:00"),
                                     rec("a-4", "open")])
        f, g, s, n = arm_b([two], devnull)
        check("arm B finds both dateless parks in a no-db store", (len(f), len(g), s, n), (2, 0, 1, 4))

        allgood = plant("all-dated", [rec("b-1", "deferred", "2026-10-09T00:00:00-04:00"),
                                      rec("b-2", "open")])
        f, g, s, n = arm_b([allgood], devnull)
        check("arm B passes a store whose parks all carry dates", (len(f), len(g), s, n), (0, 0, 1, 2))

        # The same two records in a store WITH a database are the icebox, not
        # a defect — the arm that would have failed the queue store's own
        # roadmap beads.
        dbstore = plant("db-store", [rec("c-1", "deferred"), rec("c-2", "deferred")], nodb=False)
        f, g, s, n = arm_b([dbstore], devnull)
        check("arm B reports but does not fail a db store's dateless parks", (len(f), len(g), s, n), (0, 2, 1, 2))

        empty = plant("empty", [])
        f, g, s, n = arm_b([empty], devnull)
        check("arm B over an EMPTY store reads 0 records", (len(f), len(g), s, n), (0, 0, 1, 0))

        f, g, s, n = arm_b([os.path.join(d, "no-such-store")], devnull)
        check("arm B over a store that is not there reads 0 stores", (len(f), len(g), s, n), (0, 0, 0, 0))

        # A store whose config.yaml is unreadable must read as no-db, which is
        # the loud direction: the quiet one would file a real dateless
        # park as icebox.
        unk = plant("unknown-mode", [rec("d-1", "deferred")])
        os.remove(os.path.join(unk, ".beads", "config.yaml"))
        f, g, s, n = arm_b([unk], devnull)
        check("arm B reads an unknown mode as no-db", (len(f), len(g)), (1, 0))

        # the empty-input guard itself: no store named must not be a pass.
        rc = gate_verdict(DEFECT_PRESENT, [], 0, devnull)
        check("no store named is exit 2, never a pass", rc, 2)
        rc = gate_verdict(DEFECT_PRESENT, [], 1, devnull)
        check("one clean store named is exit 0", rc, 0)
        rc = gate_verdict(DEFECT_PRESENT, [("p", "x-1", "t")], 1, devnull)
        check("a dateless park in a no-db store is exit 1", rc, 1)
        rc = gate_verdict(DEFECT_PRESENT, [], 1, devnull, [("p", "x-1", "t")])
        check("a dateless park in a db store alone is exit 0", rc, 0)
        rc = gate_verdict(DEFECT_GONE, [], 1, devnull)
        check("the defect going away is exit 1", rc, 1)

        # --arm-a-only: a pass on the BINARY alone, and it must still carry
        # arm A's alarm, must never claim a store, and must refuse to be the
        # thing that skipped one.
        rc = gate_verdict(DEFECT_PRESENT, [], 0, devnull, (), True)
        check("--arm-a-only with the defect present is exit 0", rc, 0)
        rc = gate_verdict(DEFECT_GONE, [], 0, devnull, (), True)
        check("--arm-a-only does NOT mask the defect going away", rc, 1)
        rc = gate_verdict(MODE_GONE, [], 0, devnull, (), True)
        check("--arm-a-only does NOT mask the mode going away", rc, 1)
        rc = gate_verdict(UNUSABLE, [], 0, devnull, (), True)
        check("--arm-a-only over an unusable arm A is exit 2", rc, 2)

        # the refusal, at the main() level, because it must happen before arm
        # A runs: on argv, and via the environment the door reads.
        # main() writes to stdout; the self-test's own frame owns that.
        with contextlib.redirect_stdout(devnull):
            rc = main(["x", "--arm-a-only", os.path.join(d, "all-dated")])
        check("--arm-a-only WITH a store named on argv is refused", rc, 2)
        prev = os.environ.get("POSSE_NODB_STORES")
        os.environ["POSSE_NODB_STORES"] = os.path.join(d, "all-dated")
        try:
            with contextlib.redirect_stdout(devnull):
                rc = main(["x", "--arm-a-only"])
        finally:
            if prev is None:
                del os.environ["POSSE_NODB_STORES"]
            else:
                os.environ["POSSE_NODB_STORES"] = prev
        check("--arm-a-only with POSSE_NODB_STORES set is refused", rc, 2)
    finally:
        devnull.close()
        shutil.rmtree(d, ignore_errors=True)

    print(file=out)
    if fails:
        print("self-test: %d arm(s) FAILED: %s" % (len(fails), ", ".join(fails)), file=out)
        return 1
    print("self-test: all arms ok", file=out)
    return 0


# ─── verdict ────────────────────────────────────────────────────────────────

def gate_verdict(verdict, findings, stores_read, out, db_findings=(),
                 arm_a_only=False):
    if verdict == UNUSABLE:
        print("verify-nodb-defer: arm A could not run — defect status UNKNOWN, "
              "which is not 'fixed'.", file=out)
        return 2
    if verdict == DEFECT_GONE:
        print("verify-nodb-defer: the pinned bd NO LONGER drops the defer date in "
              "a no-db store.", file=out)
        print("  That is good news and a stale-doc alarm, not a pass: the pin "
              "moved, or something other than the pinned binary is answering as "
              "`bd` (check `make verify-bd-pin`). Retire the hand-written-date "
              "workaround in %s and in docs/notes.d/ranger-base-bwp7h.md before "
              "greening this." % PIN, file=out)
        return 1
    if verdict == MODE_GONE:
        print("verify-nodb-defer: this bd has no JSONL-only mode at all — it "
              "refuses a `no-db: true` store.", file=out)
        print("  The pin is at a version that has one (etc/bd/version-pin.toml). "
              "A bd that refuses it is past 0.51.0, which is a MIGRATION and the "
              "operator's: `make verify-bd-pin`.", file=out)
        return 1
    # DEFECT_PRESENT
    if arm_a_only:
        print("verify-nodb-defer: the defect is still present in the pinned bd "
              "(arm A). Arm B did NOT run.", file=out)
        print("  This door checks the BINARY, not any store: it is the half that "
              "needs no path, so it can pass in a checkout with no private store "
              "on it. Nothing here says a store is clean. To check real stores: "
              "`POSSE_NODB_STORES=<colon-separated> make verify-nodb-defer`, or "
              "`scripts/verify-nodb-defer.py <repo-or-.beads-dir> ...`.", file=out)
        return 0
    if stores_read == 0:
        print("verify-nodb-defer: the defect is still present in the pinned bd — "
              "a no-db store drops every defer date.", file=out)
        print("  arm B read NO store, so it found nothing, and finding nothing "
              "over zero stores is NOT a pass. Name them: "
              "`scripts/verify-nodb-defer.py <repo-or-.beads-dir> ...` or "
              "POSSE_NODB_STORES=<colon-separated>.", file=out)
        return 2
    if db_findings:
        print("verify-nodb-defer: %d dateless park(s) in a store WITH a database — "
              "reported, not failed: there `bd defer <id>` with no `--until` is the "
              "icebox and means what it says." % len(db_findings), file=out)
        for path, i, title in db_findings:
            print("  %-12s in %s   %s" % (i, path, title[:60]), file=out)
    if findings:
        print("verify-nodb-defer: %d DATELESS park(s) in a no-db store — deferred "
              "with no date, so nothing re-surfaces them:" % len(findings), file=out)
        for path, i, title in findings:
            print("  %-12s in %s   %s" % (i, path, title[:60]), file=out)
        print("  In a no-db store `bd defer --until <date>` cannot fix these and "
              "`bd defer --until` cannot move a date that is already there: write "
              "defer_until into the JSONL record (bd's own writer puts it "
              "immediately before \"labels\"), then read it back with `bd show`.",
              file=out)
        return 1
    print("verify-nodb-defer: the defect is still present in the pinned bd (arm A), "
          "and none of the %d store(s) read has a dateless park in no-db mode "
          "(arm B) — the per-store lines above say which class each one is."
          % stores_read, file=out)
    return 0


def main(argv):
    out = sys.stdout
    args = [a for a in argv[1:] if not a.startswith("--")]
    flags = [a for a in argv[1:] if a.startswith("--")]
    for f in flags:
        if f not in ("--self-test", "--arm-a-only"):
            print("verify-nodb-defer: unknown flag %s" % f, file=out)
            return 2
    if "--self-test" in flags:
        return self_test(out)

    bd = os.environ.get("BD", "bd")
    if not args:
        env = os.environ.get("POSSE_NODB_STORES", "")
        args = [s for s in env.split(":") if s]

    # --arm-a-only is for the door that has no store to name. Handed one
    # anyway, it would read as a pass over a store it never opened — which is
    # the exact shape the zero-stores guard exists to refuse. Refuse it here,
    # before arm A spends a `bd defer`.
    arm_a_only = "--arm-a-only" in flags
    if arm_a_only and args:
        print("verify-nodb-defer: --arm-a-only was given WITH %d store(s) to "
              "check (%s). That combination would skip them and exit 0, so it "
              "is refused: drop the flag to check them, or drop the stores."
              % (len(args), ", ".join(args)), file=out)
        return 2

    print("verify-nodb-defer: bd=%s" % (shutil.which(bd) or bd), file=out)
    rc, o = run([bd, "version"])
    print("  version: %s" % (o.strip().split("\n")[0] if o.strip() else "(silent)"), file=out)
    verdict = arm_a(bd, out)
    if arm_a_only:
        findings, db_findings, stores_read = [], [], 0
    else:
        findings, db_findings, stores_read, records = arm_b(args, out)
        if args:
            print("  arm B: %d store(s) read, %d record(s)" % (stores_read, records), file=out)
    print(file=out)
    return gate_verdict(verdict, findings, stores_read, out, db_findings,
                        arm_a_only)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
