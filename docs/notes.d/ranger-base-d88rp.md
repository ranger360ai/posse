# ranger-base-d88rp — the trust key is NFC, and posse's was not

A repo whose path carries a **decomposed** codepoint got a
`projects[<NFD spelling>]` seed while claude reads `projects[<NFC spelling>]`.
Different JSON keys, so the grant is inert and the seat opens on the trust
dialog and never takes its prompt — ranger-base-elf2v's failure one spelling
axis over, and it escaped that close.

## What the close got wrong, and why

`docs/notes.d/ranger-base-elf2v.md` §3 read the normalization straight out of
the shipped bundle — it transcribes `tn` returning `Nn(u)` / `Nn(V(u))` and
defines `Nn` as `t.normalize("NFC")` — and then concluded

> **Nothing in that chain realpaths anything** — the answer is git's own
> spelling, out of the pointer files. That is why the fix must not "correct"
> it

which is true of realpath and true of CASE, and false of Unicode. The same
inference went into three comments: `claudeProjectKey`'s ("nothing normalizes
Unicode … posse deliberately does not"), `ClaudeConfigDirIn`'s cross-reference
to it, and the pin's ("its canonical-root function NFC-normalizes and nothing
else — so posse must not either"). All three are corrected under this bead.

The reading error is worth naming, because it is the one a second source
would have caught. `tn` is the HOP, and it is not the whole key. `N0e` is:

```js
function N0e(){ … let n=Ee(), r=CI(n); return r?cB(r):cB(Nn(Xe(n))) }
function cB(e){ let r=u(e); if(O()==="windows") return r.replaceAll("\\","/"); return r }
```

Every `return e` bail inside `tn` is unnormalized, so reading `tn` alone puts
the normalization on the hop arms only. It is not there: it is on the whole
key, one frame out. Measured below rather than re-derived, because `u` is the
one identifier in that chain this note cannot name.

## MEASURED

2026-10-03, this box, darwin 25.4.0 (APFS), claude 2.1.288 darwin-arm64,
posse at `eddd029e`. Keys asked of the CLI with `scripts/claude-trust-key.sh`
— `claude config list` in a dir holding a `.claude/settings.json` with a
`permissions.allow` entry names the key it wants, with no API turn and no
TTY. Fixture directory name: `re` U+0301 `po` (NFD), pointer files planted by
hand in the NFD spelling, which is what git writes (git stores path bytes and
normalizes nothing).

| cwd shape | claude's key | composed? |
|---|---|---|
| repo root, `.git` is a DIRECTORY (`tn` bails, EISDIR) | `<root>/répo` | **yes** |
| linked worktree of that repo (`tn` hops) | `<root>/répo` | **yes** |
| worktree of a BARE repo (`tn` hops, bare arm) | `<root>/répo-bare.git` | **yes** |
| dir in no repo | `<root>/répo-plain` | **yes** |
| `.git` copied, back-pointer does not round-trip (`tn` bails) | `<root>/répo-stray` | **yes** |
| repo root with an ASCII leaf under an NFD **parent** | `<root>/répo-parent/ascii-leaf` | **yes** |

So: NFC over the whole key string, on every arm, bails included — not over
`tn`'s two hop returns.

End to end, same run, in the NFD repo root with a scratch
`CLAUDE_CONFIG_DIR` holding exactly one `projects[]` key:

| key seeded | `claude config list` untrusted line |
|---|---|
| posse's, after this fix (NFC) | **silenced** |
| posse's, before this fix (NFD) | not silenced |
| none | not silenced |

## The fix

`norm.NFC.String` on the answer of `claudeProjectKey`, which is the single
funnel both tiers take — `ClaudeTrustKey` on the host, `claudeProjectKey`
directly in `SeedCageHome`. On the KEY and on nothing else: the value is a
JSON map key (its only callers index `projects`), so posse never stats it and
normalizing it cannot name a directory that is not there.

`ClaudeConfigDirIn` keeps the spelling it was handed, and that is the same
reason reversed — it answers a path posse OPENS, and on a filesystem that
preserves the difference the composed spelling of a decomposed directory is a
path that is not there. Mirroring the runtime there would draw a credential
wall over a file nothing uses. The divergence left standing is the runtime's
own to carry.

Pin: `TestTrustKeyIsNFCWhereClaudesIs` (internal/posse/trust_test.go), seven
arms. Four of them red without the `norm.NFC.String` call (verified by
removing it), and the ASCII and already-composed arms are what say the fix
moved nothing for the fleet as it stands.

## The dependency, and ADR 0019

Go's standard library has no NFC, so this takes
`golang.org/x/text/unicode/norm` — the first new module dependency since
`x/sys` and `x/term`.

ADR 0019 **priced x/text and declined it**, for the keychain item hash: posse
hashes the credential directory string as spelled while the runtime hashes
its NFC form, and the cost of being wrong there is a diagnostic note
(`keychainItem` emits one for any non-ASCII directory). That decline is not
disturbed here and the credential seam is unchanged — but its stated reason
was availability, and availability is now a choice. Two comments say "ADR 0019
prices x/text and declines it" (internal/posse/credential.go, and
credseam_test.go's decomposed arm); whether ADR 0019 should restate the
decline, or normalize the hash now that it can, is handed to richard as
`-l architecture`.

MEASURED price, this box, `go build ./cmd/posse`: 13,590,754 → 13,694,722
bytes, **+103,968 (+0.76%)**. The module resolved from the local module cache
with `GOPROXY=off`.
