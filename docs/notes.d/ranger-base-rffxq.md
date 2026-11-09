# The verify-chain bound at any batch size (ranger-base-rffxq)

2026-11-09. Darwin 25.6.0 arm64 laptop, main at 71b1f718. The store is one
shop's `issues.jsonl` (3,353 beads), read directly. bd is never called.
The decision is ADR 0006 §3, as amended. This file is how the numbers in
its Consequences were made.

## The question

`verify_chain_max:` counts verify rounds along a close's lineage. Before
this amendment the walk stopped at a verify bead that answers N > 1 closes,
because nothing written says which of the N a findings bundle answered. At
`verify_batch: 4` every verify bead is batched, so the count was always 1
and the bound never fired (ranger-base-bu8rz).

## What was measured

1. **minkey.py** counts every non-verify close two ways: with the walk
   continuing through a batch's SHALLOWEST source (a floor), and with the
   old stop.
2. **sim.py** asks how many of the store's batched verify beads mix
   lineage depths. It then replays every close any verify bead answered, in
   close order, at `verify_batch` 1 and 4 with a 24h age. Each replay uses
   four ways to group: one group; grouped by depth; round 0 against round 1
   or more; and each deep close on its own.

Both load `scripts/chain-depth-census.py` for its parsing, so they read
markers and edges exactly as the census and the harness do. Run them as
`python3 -I <script> scripts/chain-depth-census.py <issues.jsonl> [since]`.

Output, verbatim:

```
$ minkey.py
non-verify: 2639
MIN {0: 2058, 1: 521, 2: 49, 3: 8, 4: 3}  >=3: 11
LINEAGE(today) {0: 2058, 1: 529, 2: 41, 3: 8, 4: 3}  >=3: 11
  min=4 lin=4 ranger-base-z4vx the silent-revert self-test's rig guard is dead code, so the move nega
  min=4 lin=4 ranger-base-if3g NOTES.md still says the private-index class is 'out of the crew's reac
  min=4 lin=4 ranger-base-hegxj ranger-base-6z06r: 2 findings -- the fourth arm's equality is pinned a
  min=3 lin=3 rangerhq-ypn1 the silent-revert detector is blind to the ADD-ONLY half of its own me
  min=3 lin=3 rangerhq-cqq1 the commit wall's private-index refusal is one FILENAME wide — GIT_IND
  min=3 lin=3 rangerhq-9jk1 relaunch: keepRecipe blanks the record the guard just refused to overw
min>lin: 8  min<lin: 0
closed since 09-15: 540 MIN {0: 448, 1: 89, 2: 3} LIN {0: 448, 1: 90, 2: 2}
$ sim.py
batched verify beads: homogeneous 88 {1: 5, 0: 83} mixed 263
closes replayed: 1669 since 2000 depth mix {0: 1144, 1: 467, 2: 47, 3: 8, 4: 3}
  N=1 uniform         verify beads 1669  wait h median   0.0 p90   0.0
  N=1 by-depth        verify beads 1669  wait h median   0.0 p90   0.0
  N=1 depth0 vs >=1   verify beads 1669  wait h median   0.0 p90   0.0
  N=1 deep unbatched  verify beads 1669  wait h median   0.0 p90   0.0

  N=4 uniform         verify beads  423  wait h median   0.3 p90   1.5
  N=4 by-depth        verify beads  438  wait h median   0.5 p90   3.9
  N=4 depth0 vs >=1   verify beads  429  wait h median   0.5 p90   3.2
  N=4 deep unbatched  verify beads  815  wait h median   1.0 p90  24.0
$ sim.py ... 2026-09-15
batched verify beads: homogeneous 88 {1: 5, 0: 83} mixed 263
closes replayed: 388 since 2026-09-15 depth mix {0: 299, 1: 86, 2: 3}
  N=1 uniform         verify beads  388  wait h median   0.0 p90   0.0
  N=1 by-depth        verify beads  388  wait h median   0.0 p90   0.0
  N=1 depth0 vs >=1   verify beads  388  wait h median   0.0 p90   0.0
  N=1 deep unbatched  verify beads  388  wait h median   0.0 p90   0.0

  N=4 uniform         verify beads  102  wait h median   0.6 p90   4.3
  N=4 by-depth        verify beads  109  wait h median   1.0 p90  14.0
  N=4 depth0 vs >=1   verify beads  107  wait h median   1.0 p90  13.2
  N=4 deep unbatched  verify beads  167  wait h median   1.2 p90  24.0
```

One flaw in the replay, and it changes nothing above: the "deep unbatched"
row's waits are wrong. That row gives each deep close a group of its own but
still waits for a full batch, so those closes flush at the 24h age. Its
verify-bead count is right, and the count is the only number the ADR quotes
from that row.

## Reading it

- Most of the store's batched verify beads are mixed: 263 against 88. So a
  shallowest-source walk with no other change stays shallow. Since
  2026-09-15 it reads a deepest of 2, and the bound of 3 still never fires.
  Grouping by depth is what makes the floor exact.
- Grouping by depth costs +3.5% verify beads over the whole history and
  +6.9% since 2026-09-15. Filing each deep close on its own costs +93% and
  +64%.
- ASSUMED: the replay's depths come from today's graph, which mixed batches
  have pulled down. With grouping in place, the depth mix should look like
  the `verify_batch: 1` era's. The whole-history replay is the closer
  estimate.

## The scripts

The implementation bead folds both into `scripts/chain-depth-census.py` as
its third key and a `--replay` mode. Until then, here they are as run.

```python
# minkey.py
import sys, collections, importlib.util, datetime
spec = importlib.util.spec_from_file_location("c", sys.argv[1]); c = importlib.util.module_from_spec(spec); spec.loader.exec_module(c)
beads = c.load(sys.argv[2]); par = c.parents(beads)
memo = {}
def dmin(i, seen):
    # rounds above i; at a batched verify bead, continue through the SHALLOWEST source (a floor)
    q = c.verify_parent(beads, par, i, seen)
    if q is None: return 0
    srcs = [s for s in c.verify_sources(beads, q) if s not in seen and s != q]
    if not srcs: return 1
    return 1 + min(dmin(s, seen | {q, s}) for s in srcs)
def dlin(i): return len(c.depth(beads, par, {}, i, False))
rows=[]
for i in beads:
    if c.is_verify(beads, i): continue
    rows.append((dmin(i,{i}), dlin(i), i))
print("non-verify:", len(rows))
for name, k in (("MIN", 0), ("LINEAGE(today)", 1)):
    h = collections.Counter(r[k] for r in rows)
    print(name, dict(sorted(h.items())), " >=3:", sum(v for d,v in h.items() if d>=3))
rows.sort(reverse=True)
for d, l, i in rows[:6]: print("  min=%d lin=%d %s %s" % (d, l, i, (beads[i].get('title') or '')[:70]))
# how many closes does MIN read deeper than today's walk
print("min>lin:", sum(1 for r in rows if r[0]>r[1]), " min<lin:", sum(1 for r in rows if r[0]<r[1]))
# closes after 2026-09-15 (batched era)
late=[r for r in rows if (beads[r[2]].get('closed_at') or '')>='2026-09-15']
print("closed since 09-15:", len(late), "MIN", dict(sorted(collections.Counter(r[0] for r in late).items())), "LIN", dict(sorted(collections.Counter(r[1] for r in late).items())))

# sim.py
import sys, collections, importlib.util
from datetime import datetime, timedelta
spec = importlib.util.spec_from_file_location("c", sys.argv[1]); c = importlib.util.module_from_spec(spec); spec.loader.exec_module(c)
beads = c.load(sys.argv[2]); par = c.parents(beads)
def dmin(i, seen):
    q = c.verify_parent(beads, par, i, seen)
    if q is None: return 0
    srcs = [s for s in c.verify_sources(beads, q) if s not in seen]
    if not srcs: return 1
    return 1 + min(dmin(s, seen | {q, s}) for s in srcs)
def ts(s): return datetime.fromisoformat(s.replace('Z','+00:00')[:19])
# mixed-ness of actual batched verify beads
mixed=homog=0; byround=collections.Counter()
for q,b in beads.items():
    if not c.is_verify(beads,q): continue
    s=c.verify_sources(beads,q)
    if len(s)<2: continue
    ds={dmin(x,{x}) for x in s if x in beads}
    if len(ds)>1: mixed+=1
    else: homog+=1; byround[next(iter(ds))]+=1
print("batched verify beads: homogeneous", homog, dict(byround), "mixed", mixed)
# replay: candidates = every close some verify bead answers, in close order
cands=set()
for q in beads:
    if c.is_verify(beads,q): cands.update(c.verify_sources(beads,q))
cl=sorted((ts(beads[i]['closed_at']), i) for i in cands if i in beads and beads[i].get('closed_at'))
since = sys.argv[3] if len(sys.argv)>3 else '2000'
cl=[x for x in cl if x[0].isoformat()>=since]
def replay(N, age, key):
    # passes are event-driven: flush full batches immediately, tail flushed when oldest reaches age
    pend=collections.defaultdict(list); beads_filed=0; waits=[]
    events=cl+[(cl[-1][0]+timedelta(days=30),None)]
    for t,i in events:
        for k in list(pend):
            while pend[k] and t-pend[k][0][0]>=age:
                g=pend[k][:N]; del pend[k][:N]; beads_filed+=1; waits+=[ (g[0][0]+age - x[0]).total_seconds()/3600 for x in g]
        if i is None: break
        k=key(i); pend[k].append((t,i))
        if len(pend[k])>=N:
            g=pend[k][:N]; del pend[k][:N]; beads_filed+=1; waits+=[(t-x[0]).total_seconds()/3600 for x in g]
    waits.sort()
    return beads_filed, waits[len(waits)//2], waits[int(len(waits)*.9)]
D={i:dmin(i,{i}) for _,i in cl}
print("closes replayed:", len(cl), "since", since, "depth mix", dict(sorted(collections.Counter(D.values()).items())))
for N in (1,4):
    for name,key in (("uniform",lambda i:0),("by-depth",lambda i:D[i]),("depth0 vs >=1",lambda i:min(D[i],1)),("deep unbatched",lambda i:0 if D[i]==0 else i)):
        f,m,p=replay(N,timedelta(hours=24),key)
        print(f"  N={N} {name:15s} verify beads {f:4d}  wait h median {m:5.1f} p90 {p:5.1f}")
    if N==1: print()
```
