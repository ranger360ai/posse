#!/bin/sh
# ADR 0076 probe — (1) which herdr client verbs posse calls, from where;
# (2) what Rex's program-status reading (OSC 7501) holds for a report, for a
# hook-shaped writer, and for a writer with no controlling tty.
# Usage: sh docs/adr/0076-multiplexer-slot.probe.sh        (from the repo root)
#        REX=<path to rex> to override the CLI (it is not on PATH by default).
# Part 2 creates ONE Rex session labelled lcxr3-probe-$$ and kills only that.
# No runtime is launched and no model is called; the claude reading in the ADR
# was taken by hand (see its Measurements section).
set -u

echo "== part 1: herdr client census (non-test .go under cmd/ and internal/)"
python3 -I - <<'EOF'
import os, re, collections
files = []
for d in ("cmd", "internal"):
    for dp, _, fs in os.walk(d):
        files += [os.path.join(dp, f) for f in fs if f.endswith(".go") and not f.endswith("_test.go")]
src = {f: open(f).read() for f in files}
client = re.findall(r'^func \(h Herdr\) (\w+)\(', src["internal/posse/herdr.go"], re.M)
pat = re.compile(r'(\.H|\bh|\bH|[hH]erdr\w*)\.(' + "|".join(client) + r')\(')
by = collections.defaultdict(set)
for f, s in src.items():
    if f == "internal/posse/herdr.go":
        continue
    for m in pat.finditer(s):
        by[m.group(2)].add(os.path.basename(f))
for k in client:
    if by[k]:
        print(f"  {k:18} {len(by[k]):2} files  {' '.join(sorted(by[k]))}")
print("  verbs called outside herdr.go:", sum(1 for k in client if by[k]), "of", len(client))
print("  files calling the client:", len(set().union(*by.values())))
print("  files holding/building Herdr or HerdrBackend:",
      sum(1 for s in src.values() if re.search(r'\bHerdr\{|NewHerdr\(|HerdrBackend\b', s)))
print("  non-test .go files:", len(files))
EOF

echo "== part 2: Rex program status"
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "  rex not found at \$REX; skipping part 2"; exit 0; }
"$REX" version | head -1
S=lcxr3-probe-$$
B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
trap '"$REX" kill "$S" >/dev/null 2>&1' EXIT
pause() { python3 -I -c 'import time; time.sleep(1)'; }
status() { "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}"; echo; }
say() { "$REX" send -s "$S" -b "$B" "$2"; "$REX" send-key -s "$S" -b "$B" enter; pause; printf '  %s: ' "$1"; status; }
say "A state=working            " "printf '\\033]7501;state=working\\033\\\\'"
say "B blocked, kind=permission " "printf '\\033]7501;state=blocked:kind=permission:app=probe\\033\\\\'"
say "C msg byte outside the set: that pair skipped, rest applied" "printf '\\033]7501;state=idle:msg=not-base64!\\033\\\\'"
say "C2 msg in the set but not base64: whole report dropped     " "printf '\\033]7501;state=blocked:msg=needs-ok\\033\\\\'"
say "D msg base64               " "printf '\\033]7501;state=idle:app=probe:msg=aGVsbG8=\\033\\\\'"
say "E hook shape: stdout to a file, report to /dev/tty" \
    "(printf '\\033]7501;state=working:app=hooked\\033\\\\' > /dev/tty) > /dev/null 2>&1 < /dev/null"
say "F no controlling tty (setsid child; expect unchanged)" \
    "python3 -I -c 'import os
if os.fork()==0:
    os.setsid()
    try: open(\"/dev/tty\",\"w\").write(\"\\033]7501;state=error\\033\\\\\")
    except OSError as e: print(\"child:\", e)
    os._exit(0)
os.wait()'"
"$REX" capture -s "$S" -b "$B" --trim | grep 'child:' | sed 's/^/  /'
