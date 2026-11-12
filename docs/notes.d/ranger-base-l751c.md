# claude's server-policy caches, masked and never seeded (ranger-base-l751c)

*2026-11-12 · dinesh · census ranger-base-h5vij · touches the carve
(internal/posse/wallexecconfig.go, seatbelt.go) and the bwrap render*

## What this closes

claude's config dir holds stored copies of server-fetched policy.
`remote-settings.json` is the account's managed settings, which claude loads
at POLICY tier (env, permissions, hooks) for an account it deems eligible:
any API-key login against api.anthropic.com, and team or enterprise OAuth.
`policy-limits.json` is the org's policy limits. Beside them sit their
signature, stamp and consent files. A caged seat could write all of them,
because the config dir is its state grant. A copy written by a seat was then
obeyed by every later claude on that config dir, the uncaged ones included.

The ten names are claudeManagedCacheFileNames. They are the set claude
2.1.322's own sandbox write-denies.

## Measured

MEASURED 2026-11-12, D1 host, claude 2.1.322, bwrap 0.13.1, unbilled. The
harness: a scratch CLAUDE_CONFIG_DIR, an API-key login with no base-URL
override (a custom base URL reads as ineligible, `custom_base_url`), inside
`unshare -rnm`. In there, `/etc/hosts` and `/etc/nsswitch.conf` point
api.anthropic.com at a local TLS fake, using a throwaway CA passed through
NODE_EXTRA_CA_CERTS. The netns has only lo, so nothing leaves the box. The
fake fails the settings fetch, so the stored copy is the only source. It then
asks for one Bash call that prints the env it got.

| config dir | gate flag in `.claude.json` | planted copy obeyed |
|---|---|---|
| open | `tengu_fizzy_widget: false` | **yes** |
| open | absent (fresh cache) | **yes** |
| open | `true` | no |
| /dev/null over every member | `false` | no; the turn exits 0 |

- **The trust gate cannot protect this cache against a seat.** It is a
  server flag, and its served value is cached in `.claude.json`'s
  `cachedGrowthBookFeatures`. That file is written by the seat every turn and
  cannot be carved (ranger-base-nv3el). It is also open when the cache is
  fresh. This answers hoover's question on the bead.
- Masked, with the server serving settings: the turn exits 0 and the served
  settings are applied for the run.
- Empty 0600 members, with no mask and an uncaged claude: read as "no cached
  settings", and the turn exits 0.
- bwrap's own mount point for an absent name is a 0444 empty file, and it
  stays on the host. claude rewrites these caches in place
  (`O_WRONLY|O_CREAT|O_TRUNC`, no rename; read from the binary), so 0444
  would stop the operator's own claude from ever caching again. So the
  launcher makes the mount point itself, as an empty 0600 file
  (PrepMaskPoint).

## The fix

- `SeatbeltCarveOut.Mask` / `Masked()`: file-shaped denies that bwrap
  covers with `--ro-bind /dev/null <name>`, whether or not the file is
  there. They are never seeded, because a `{}` would be a stored answer of
  posse's making. Seatbelt renders them as the plain write deny they also are,
  which needs no name.
- A link or a directory at a masked name refuses the render, because a bwrap
  mount follows a link and leaves the name open to a rename.
- Pins: `go test ./internal/posse -run 'WallExecConfigMask'`. With the
  producer's `add(&c.Mask, f)` removed, all three red. The live one reds on
  write, append, replace and remove of every absent member: that is the hole
  as it stood.

## Not measured

- The darwin half. It is an SBPL literal deny and is rendered, but not run.
- A real claude under posse's FULL render as an eligible account. The harness
  above used a hand-built bwrap line with the same `--ro-bind /dev/null`
  masks. The full render's join is the live pin, which uses `sh` and not
  claude.
