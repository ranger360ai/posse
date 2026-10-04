#!/usr/bin/env python3
# Write the defer DATE that a `no-db: true` beads store threw away
# (ranger-base-bwp7h). The other half of scripts/verify-nodb-defer.py: that
# one finds dateless parks, this one dates them.
#
#   scripts/nodb-set-defer.py <repo-or-.beads-dir> <id>=<YYYY-MM-DD> ... [--apply]
#
# WHY THIS EXISTS AT ALL. In a store configured `no-db: true`, on the pinned
# bd (etc/bd/version-pin.toml), `bd defer <id> --until <date>` sets the status
# to "deferred", prints `* Deferred <id>` and writes NO `defer_until`; so does
# `bd update <id> --defer <date>`; and re-deferring a record that already
# carries a date leaves the OLD date in place and still reports success. The
# value is accepted, reported as success, and dropped. The READER is whole —
# a date written into the JSONL is printed by `bd show`, emitted by
# `bd show --json` and `bd list --json`, and survives later bd writes — so in
# such a store the date is set here and nowhere else. docs/notes.d/
# ranger-base-bwp7h.md has the measurements and why no bd version fixes it.
#
# THE DIVISION OF LABOUR, and the reason for the refusals below:
#   bd sets the STATUS      — `bd defer <id>` does that correctly even here.
#   this sets the DATE      — only on a record bd has already parked.
# So it refuses a record that is not already status "deferred" (run `bd defer
# <id>` first and come back), it refuses a store that is NOT in no-db mode
# (there `bd defer <id> --until <date>` works and is the right verb), and it
# refuses to overwrite a date that is already there without --replace, which
# is what re-parking needs because `bd defer --until` cannot move one.
#
# HOW IT WRITES. It splices text and verifies by PARSING; it never
# re-serialises the file. A json round-trip of a bd-written JSONL is not
# byte-identical — Go and Python escape differently, 13 of 16 lines changed on
# the store this was built for — so a re-serialising edit would churn every
# record it touched. The key goes immediately before the top-level "labels",
# which is where bd's own writer puts it, and the file is replaced atomically
# with its mode preserved. Dry run unless --apply.
#
# AFTER IT WRITES, read it back with bd and not with this: `bd show <id>`
# should carry a `Deferred: <date>` field, and
# `scripts/verify-nodb-defer.py <store>` should stop naming the id. The file is
# not the proof; what bd says it reads is.

import json
import os
import shutil
import subprocess
import sys
import tempfile

ISO = "%sT00:00:00%s"


def local_offset():
    """The ±HH:MM bd itself writes: midnight local, explicit offset."""
    import time
    off = -(time.altzone if time.daylight and time.localtime().tm_isdst else time.timezone)
    sign = "+" if off >= 0 else "-"
    off = abs(off)
    return "%s%02d:%02d" % (sign, off // 3600, (off % 3600) // 60)


def top_level_key_pos(line, key):
    """Offset of the `"<key>":` that sits at object depth 1, or -1.

    Depth-aware and string-aware: a `"labels":` inside a comment body or a
    description is at depth 2 or inside a string, and must not be found.
    """
    needle = '"%s":' % key
    depth = 0
    i = 0
    n = len(line)
    while i < n:
        c = line[i]
        if c == '"':
            if depth == 1 and line.startswith(needle, i):
                return i
            j = i + 1
            while j < n:
                if line[j] == "\\":
                    j += 2
                    continue
                if line[j] == '"':
                    break
                j += 1
            i = j + 1
            continue
        if c in "{[":
            depth += 1
        elif c in "}]":
            depth -= 1
        i += 1
    return -1


def is_nodb(beads_dir):
    """True / False / None when config.yaml cannot be read."""
    try:
        for line in open(os.path.join(beads_dir, "config.yaml"),
                         encoding="utf-8", errors="replace"):
            s = line.strip()
            if s.startswith("#"):
                continue
            if s.startswith("no-db:"):
                return s.split(":", 1)[1].split("#")[0].strip() == "true"
    except OSError:
        return None
    return False


def resolve(store):
    d = store
    if os.path.basename(d.rstrip("/")) != ".beads":
        d = os.path.join(d, ".beads")
    return d, os.path.join(d, "issues.jsonl")


def parse_dates(args):
    out = {}
    for a in args:
        if "=" not in a:
            return None, "not an <id>=<date> pair: %r" % a
        i, d = a.split("=", 1)
        parts = d.split("-")
        if len(parts) != 3 or not all(p.isdigit() for p in parts) or \
                len(parts[0]) != 4 or len(parts[1]) != 2 or len(parts[2]) != 2:
            return None, "%s: date must be YYYY-MM-DD, got %r" % (i, d)
        out[i] = ISO % (d, local_offset())
    return out, None


def apply_dates(jsonl, dates, replace, out):
    """(new-file-text or None, changed, refusals). Nothing is written here."""
    try:
        raw = open(jsonl, encoding="utf-8").read()
    except OSError as e:
        print("  cannot read %s: %s" % (jsonl, e), file=out)
        return None, 0, 1
    lines = raw.split("\n")
    seen = set()
    new = []
    changed = 0
    refused = 0
    for line in lines:
        if not line.strip():
            new.append(line)
            continue
        try:
            rec = json.loads(line)
        except ValueError:
            print("  a line of %s is not JSON — REFUSED, the whole file is left "
                  "alone" % jsonl, file=out)
            return None, 0, refused + 1
        rid = rec.get("id")
        want = dates.get(rid)
        if want is None:
            new.append(line)
            continue
        seen.add(rid)
        have = rec.get("defer_until")
        if rec.get("status") != "deferred":
            print("  %-14s status is %r, not 'deferred' — REFUSED. `bd defer %s` "
                  "first: bd sets the status correctly even here, and this sets "
                  "only the date it dropped."
                  % (rid, rec.get("status"), rid), file=out)
            refused += 1
            new.append(line)
            continue
        if have and not replace:
            print("  %-14s already carries %s — left alone. Pass --replace to "
                  "re-park it (`bd defer --until` cannot move a date that is "
                  "already there)." % (rid, have), file=out)
            new.append(line)
            continue
        if have == want:
            print("  %-14s already carries %s — nothing to do." % (rid, have), file=out)
            new.append(line)
            continue
        if have:
            spliced = line.replace('"defer_until":"%s"' % have,
                                   '"defer_until":"%s"' % want, 1)
            if spliced == line:
                print("  %-14s could not find its own defer_until to replace — "
                      "REFUSED" % rid, file=out)
                refused += 1
                new.append(line)
                continue
        else:
            pos = top_level_key_pos(line, "labels")
            if pos < 0:
                print("  %-14s has no top-level \"labels\" key to place the date "
                      "before — REFUSED" % rid, file=out)
                refused += 1
                new.append(line)
                continue
            spliced = line[:pos] + '"defer_until":"%s",' % want + line[pos:]
        # Verify by parsing, never by re-serialising: the spliced record must
        # differ from the original in defer_until and in nothing else.
        try:
            got = json.loads(spliced)
        except ValueError:
            print("  %-14s splice did not parse — REFUSED" % rid, file=out)
            refused += 1
            new.append(line)
            continue
        if got.get("defer_until") != want:
            print("  %-14s splice did not take — REFUSED" % rid, file=out)
            refused += 1
            new.append(line)
            continue
        a = dict(got)
        b = dict(rec)
        a.pop("defer_until", None)
        b.pop("defer_until", None)
        if a != b:
            print("  %-14s splice changed another field — REFUSED" % rid, file=out)
            refused += 1
            new.append(line)
            continue
        print("  %-14s %s%s" % (rid, "" if not have else "%s -> " % have, want), file=out)
        changed += 1
        new.append(spliced)
    for rid in sorted(set(dates) - seen):
        print("  %-14s no such record in this store — REFUSED" % rid, file=out)
        refused += 1
    return "\n".join(new), changed, refused


def write_atomic(path, text):
    d = os.path.dirname(os.path.abspath(path))
    fd, tmp = tempfile.mkstemp(dir=d, prefix=".nodb-set-defer.")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            fh.write(text)
        os.chmod(tmp, os.stat(path).st_mode & 0o7777)
        os.replace(tmp, path)
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise


# ─── self-test ──────────────────────────────────────────────────────────────

def self_test(out):
    fails = []

    def check(label, got, want):
        ok = got == want
        print("  %-56s %s (got %r)" % (label, "ok" if ok else "FAIL", got), file=out)
        if not ok:
            fails.append(label)

    d = tempfile.mkdtemp(prefix="nodb-set-defer-selftest-")
    devnull = open(os.devnull, "w")
    off = local_offset()
    try:
        def plant(name, lines, nodb=True):
            b = os.path.join(d, name, ".beads")
            os.makedirs(b, exist_ok=True)
            with open(os.path.join(b, "config.yaml"), "w") as fh:
                fh.write('issue-prefix: "x"\n' + ("no-db: true\n" if nodb else ""))
            p = os.path.join(b, "issues.jsonl")
            with open(p, "w") as fh:
                fh.write("\n".join(lines) + "\n")
            return os.path.join(d, name), p

        dep = '{"id":"x-1","title":"a","status":"deferred","labels":["q"],"comments":[{"text":"\\"labels\\": not this one"}]}'
        opn = '{"id":"x-2","title":"b","status":"open","labels":["q"]}'
        dated = '{"id":"x-3","title":"c","status":"deferred","defer_until":"2026-01-01T00:00:00%s","labels":["q"]}' % off
        nolab = '{"id":"x-4","title":"d","status":"deferred"}'

        _, p = plant("ok", [dep, opn])
        txt, ch, rf = apply_dates(p, {"x-1": ISO % ("2026-10-09", off)}, False, devnull)
        check("dates a deferred record", (ch, rf), (1, 0))
        r = json.loads(txt.strip().split("\n")[0])
        check("the date lands, nothing else moves", r.get("defer_until"), ISO % ("2026-10-09", off))
        check("and it lands immediately before \"labels\"",
              list(r.keys()).index("defer_until") + 1 == list(r.keys()).index("labels"), True)
        # the nested `"labels":` inside a comment body must not be the anchor
        check("a nested \"labels\" in a comment is not the anchor",
              json.loads(txt.strip().split("\n")[0])["comments"][0]["text"],
              '"labels": not this one')

        txt, ch, rf = apply_dates(p, {"x-2": ISO % ("2026-10-09", off)}, False, devnull)
        check("refuses a record that is not deferred", (ch, rf), (0, 1))

        _, p3 = plant("dated", [dated])
        txt, ch, rf = apply_dates(p3, {"x-3": ISO % ("2026-10-16", off)}, False, devnull)
        check("leaves an existing date alone without --replace", (ch, rf), (0, 0))
        txt, ch, rf = apply_dates(p3, {"x-3": ISO % ("2026-10-16", off)}, True, devnull)
        check("--replace moves it", (ch, rf), (1, 0))
        check("and moves it to the new date",
              json.loads(txt.strip())["defer_until"], ISO % ("2026-10-16", off))
        txt, ch, rf = apply_dates(p3, {"x-3": ISO % ("2026-01-01", off)}, True, devnull)
        check("--replace with the date it already has is a no-op", (ch, rf), (0, 0))

        _, p4 = plant("nolabels", [nolab])
        txt, ch, rf = apply_dates(p4, {"x-4": ISO % ("2026-10-09", off)}, False, devnull)
        check("refuses a record with no top-level \"labels\"", (ch, rf), (0, 1))

        _, p5 = plant("absent", [dep])
        txt, ch, rf = apply_dates(p5, {"x-99": ISO % ("2026-10-09", off)}, False, devnull)
        check("refuses an id that is not in the store", (ch, rf), (0, 1))

        _, p6 = plant("broken", [dep, "{not json"])
        txt, ch, rf = apply_dates(p6, {"x-1": ISO % ("2026-10-09", off)}, False, devnull)
        check("one unparseable line leaves the WHOLE file alone", (txt, ch), (None, 0))

        # the mode guard, and the date-shape guard
        store, _ = plant("hasdb", [dep], nodb=False)
        check("a store with a database reads as not-no-db", is_nodb(resolve(store)[0]), False)
        check("a date that is not YYYY-MM-DD is refused",
              parse_dates(["x-1=2026-10-9"])[0], None)
        check("a bare id with no date is refused", parse_dates(["x-1"])[0], None)
        check("a good pair parses", parse_dates(["x-1=2026-10-09"])[0],
              {"x-1": ISO % ("2026-10-09", off)})

        # and the write path: atomic replace keeps the mode and the content
        store7, p7 = plant("writeme", [dep, opn])
        os.chmod(p7, 0o600)
        txt, ch, rf = apply_dates(p7, {"x-1": ISO % ("2026-10-09", off)}, False, devnull)
        write_atomic(p7, txt)
        back = open(p7, encoding="utf-8").read()
        check("the written file parses and carries the date",
              json.loads(back.strip().split("\n")[0]).get("defer_until"),
              ISO % ("2026-10-09", off))
        check("and keeps the file mode", oct(os.stat(p7).st_mode & 0o777), oct(0o600))
        check("and keeps every other record byte-identical",
              back.strip().split("\n")[1], opn)
    finally:
        devnull.close()
        shutil.rmtree(d, ignore_errors=True)

    print(file=out)
    if fails:
        print("self-test: %d arm(s) FAILED: %s" % (len(fails), ", ".join(fails)), file=out)
        return 1
    print("self-test: all arms ok", file=out)
    return 0


def main(argv):
    out = sys.stdout
    flags = [a for a in argv[1:] if a.startswith("--")]
    args = [a for a in argv[1:] if not a.startswith("--")]
    for f in flags:
        if f not in ("--apply", "--replace", "--self-test"):
            print("nodb-set-defer: unknown flag %s" % f, file=out)
            return 2
    if "--self-test" in flags:
        return self_test(out)
    if len(args) < 2:
        print(__doc__ or "", file=out)
        print("usage: nodb-set-defer.py <repo-or-.beads-dir> <id>=<YYYY-MM-DD> ... "
              "[--replace] [--apply]", file=out)
        return 2

    store, rest = args[0], args[1:]
    beads, jsonl = resolve(store)
    if not os.path.exists(jsonl):
        print("nodb-set-defer: no such store: %s" % jsonl, file=out)
        return 2
    nodb = is_nodb(beads)
    if nodb is False:
        print("nodb-set-defer: %s is NOT in no-db mode — it has a database, where "
              "`bd defer <id> --until <date>` writes the date correctly. Use bd; "
              "this is for the mode that drops it." % beads, file=out)
        return 2
    if nodb is None:
        print("nodb-set-defer: cannot read %s/config.yaml, so the store's mode is "
              "unknown. Refusing to hand-edit a store that may not need it." % beads,
              file=out)
        return 2

    dates, err = parse_dates(rest)
    if err:
        print("nodb-set-defer: %s" % err, file=out)
        return 2

    print("nodb-set-defer: %s  [no-db: true]" % jsonl, file=out)
    text, changed, refused = apply_dates(jsonl, dates, "--replace" in flags, out)
    print("  %d of %d named record(s) would change, %d refused"
          % (changed, len(dates), refused), file=out)
    if text is None:
        return 2
    if "--apply" not in flags:
        print("  (dry run — pass --apply to write)", file=out)
        return 1 if refused else 0
    if changed == 0:
        print("  nothing to write", file=out)
        return 1 if refused else 0
    write_atomic(jsonl, text)
    print("  wrote %s atomically" % jsonl, file=out)
    print("  now read it back with bd, not with this: `bd show <id>` must carry a "
          "`Deferred:` field, and `scripts/verify-nodb-defer.py %s` must stop "
          "naming the ids." % store, file=out)
    return 1 if refused else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
