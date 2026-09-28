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
awk '/^data_ceiling_patterns:/{f=1;next} f&&/^[^ ]/{f=0} f&&NF{sub(/^[ ]*[a-z-]+:[ ]*/,""); print}' "$cfg" | while read -r re; do
  n=$(grep -cE "$re" "${TMPDIR:-/tmp}/ceiling-staged.txt" || true)
  echo "self-match hits=$n"
  [ "$n" -eq 0 ] || { echo "ceiling-fill: a value matches its own definition; refusing to stamp" >&2; exit 1; }
done
rm -f "${TMPDIR:-/tmp}/ceiling-staged.txt"
posse gates install-hooks "$repo" | tail -1 | cut -c1-72
git commit -qm 'config: data ceiling armed (ceiling-fill.sh)' -- "$rel"
git log --oneline | head -1
