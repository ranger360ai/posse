#!/bin/sh
# ceiling-fill.sh — fill `data_ceiling_patterns:` in $RHQ_HOME/config.yaml from
# four answers typed at the terminal, prove no value matches its own definition
# line, re-stamp the hooks and commit the config (ADR 0050; ranger-base-3gdqv).
#
# Why a script: a ceiling value is the vocabulary the wall keeps out, so it is
# typed on the instance's own box and never pasted through a transcript. Each
# value is escaped to POSIX ERE (no \d \s \b — bracket classes only) and its
# first character is bracketed so the regex cannot match the line that defines
# it: a value that matches itself refuses the commit of config.yaml, class-only,
# with no way to say why (measured 2026-09-27, the `FILL-ME` placeholder).
#
# Usage, from the instance repo root (the directory that holds posse/config.yaml
# or wherever RHQ_HOME points): scripts/ceiling-fill.sh
set -e
cfg="${RHQ_HOME:-posse}/config.yaml"
[ -f "$cfg" ] || { echo "ceiling-fill: no config at $cfg (set RHQ_HOME or run from the instance repo root)" >&2; exit 1; }
# Both through realpath: macOS hands out /var for /private/var, and a relative
# path computed across that seam climbs out of the repo (measured).
cfg=$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$cfg")
repo=$(git -C "$(dirname "$cfg")" rev-parse --show-toplevel)
repo=$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$repo")
python3 - "$cfg" <<'PY'
import re,sys
p=sys.argv[1]; s=open(p).read()
if 'data_ceiling_patterns:' not in s or not re.search(r'^data_ceiling_patterns:', s, re.M):
    s=s.rstrip('\n')+'\ndata_ceiling_patterns:\n  restricted-banner: FILL-ME\n  restricted-host: FILL-ME\n  restricted-export: FILL-ME\n  attachment-marker: FILL-ME\n'
else:
    s=re.sub(r'^(\s+(restricted-banner|restricted-host|restricted-export|attachment-marker)):.*$', r'\1: FILL-ME', s, flags=re.M)
open(p,'w').write(s)
PY
printf 'banner text as it reads in a document: '; read -r BANNER
printf 'restricted system hostname: '; read -r HOST
printf 'export file-name prefix (before the date): '; read -r EXPORT_PREFIX
printf 'attachment marker text: '; read -r ATTACH
python3 - "$cfg" "$BANNER" "$HOST" "$EXPORT_PREFIX" "$ATTACH" <<'PY'
import re,sys
def rx(s, tail=''):
    parts=[re.escape(w).replace('\\ ','') for w in s.split()]
    body='[[:space:]]+'.join(parts).replace('\\.', '[.]').replace('\\-', '[-]')
    return '['+body[0]+']'+body[1:]+tail
p,b,h,e,a=sys.argv[1:6]
vals={'restricted-banner':rx(b),'restricted-host':rx(h),'restricted-export':rx(e,'[-][0-9]{8}[.](csv|xlsx)'),'attachment-marker':rx(a)}
out=[]
for line in open(p):
    m=re.match(r'^(\s+)([a-z-]+):\s*FILL-ME\s*$', line)
    out.append(f"{m.group(1)}{m.group(2)}: {vals[m.group(2)]}\n" if m and m.group(2) in vals else line)
open(p,'w').write(''.join(out))
PY
cd "$repo"
rel=$(python3 -c 'import os,sys; print(os.path.relpath(sys.argv[1], sys.argv[2]))' "$cfg" "$repo")
git add -- "$rel"
git diff --cached -U0 --no-color --no-ext-diff -- "$rel" | grep '^+' | grep -v '^+++' > "${TMPDIR:-/tmp}/ceiling-staged.txt" || true
# The PATTERN has to be the value the HOOK will judge by, which is yamlClean's
# reading of the line and not "everything after the key". An entry carrying a
# trailing comment — and this file's own documentation in examples/config.yaml
# shows one, because a comment repeating the literal refuses the commit on its
# own — yielded the regexp `<value>  # <comment>`, which the staged lines do not
# contain, so the guard below read hits=0 and stamped the exact shape it exists
# to refuse (ranger-base-yplnv, escaped from ranger-base-l2569). The four keys
# this script writes itself were never at risk: the rewrite above replaces the
# whole line and rx() brackets the first character. Any OTHER class the operator
# added by hand was. internal/posse/yamlflat.go is the authority for this
# reading, and TestQACeilingFillReadsTheValueTheHookReads runs THESE bytes
# against it.
python3 - "$cfg" <<'PY_CEILING_VALUES' > "${TMPDIR:-/tmp}/ceiling-patterns.txt"
import sys
def clean(v):                        # yamlClean: comment, trim, one quote pair
    for i in range(1, len(v)):
        if v[i] == '#' and v[i-1] in ' \t':
            v = v[:i-1]
            break
    v = v.strip()
    if len(v) >= 2 and v[0] == '"' and v[-1] == '"':
        v = v[1:-1]
    return v
inmap = False
for ln in open(sys.argv[1]).read().split('\n'):   # YamlMapPairsRaw
    if not inmap:
        inmap = ln.startswith('data_ceiling_patterns:')
        continue
    if ln and ln[0] not in ' \t#':
        break
    t = ln.lstrip(' \t')
    if not t or t == ln or t[0] == '#':
        continue
    i = t.find(':')
    if i > 0:
        print(clean(t[i+1:]))
PY_CEILING_VALUES
# Redirected, not piped: `exit 1` in the body of a piped `while` ends a
# SUBSHELL, and only `set -e` over the pipeline's status carried the refusal out.
while read -r re; do
  [ -n "$re" ] || continue
  n=$(grep -cE "$re" "${TMPDIR:-/tmp}/ceiling-staged.txt" || true)
  echo "self-match hits=$n"
  [ "$n" -eq 0 ] || { echo "ceiling-fill: a value matches its own definition; refusing to stamp" >&2; exit 1; }
done < "${TMPDIR:-/tmp}/ceiling-patterns.txt"
rm -f "${TMPDIR:-/tmp}/ceiling-staged.txt" "${TMPDIR:-/tmp}/ceiling-patterns.txt"
# The stamp report in FULL, and the hook's own reading is the one that decides.
# `tail -1 | cut -c1-72` kept the last line only, truncated to 72 characters, so
# the one sentence that explains a class-only refusal — which is the only
# explanation there is (ADR 0050 D6) — was not guaranteed to reach the operator
# before the commit ran (ranger-base-yplnv). A failing stamp now stops the
# script too; under the pipeline it was `tail`'s exit status that `set -e` read.
stamp=$(posse gates install-hooks "$repo" 2>&1) || { printf '%s\n' "$stamp" >&2; exit 1; }
printf '%s\n' "$stamp"
case "$stamp" in
  *"matches its own definition line"*)
    echo "ceiling-fill: the hook's own reading says a value matches its own definition line — refusing to commit the config; follow the remedy above and re-run" >&2
    exit 1 ;;
esac
git commit -qm 'config: data ceiling armed (ceiling-fill.sh)' -- "$rel"
git log --oneline | head -1
