## The cockpit is a herdr plugin

`plugin/herdr-plugin.toml` declares a popup pane running `posse cockpit` — a
session-modal overlay (like herdr's lazygit example). Register it against
the running herdr with `make link-plugin`; open it from herdr's plugin UI,
a bound key, or `herdr plugin pane open --plugin posse.cockpit
--entrypoint cockpit`. Plugins get the full herdr CLI as their API
(`HERDR_BIN_PATH`, `HERDR_SOCKET_PATH` injected), so the cockpit is just
`posse` running in a pane. Pane commands run with cwd = plugin root, and the
manifest runs `./bin/posse` — a gitignored symlink `make link-plugin` points at
the installed binary (`$BINDIR/posse`) — so the popup runs the promoted build,
not whatever PATH or a persona's last `make build` produced (rangerhq-8te).

The cockpit is a small raw-mode TUI (only dep: `golang.org/x/term`):
sessions sorted blocked-first with live agent state, the aggregated ready
queue beneath. Keys: `j/k`/arrows move, `tab` jumps sections, `enter`
focuses the selected session's workspace and exits (the popup closes,
revealing it), `p` prompts its agent inline, `v` peeks its terminal tail,
`x` kills (y confirms), `c` claims the selected bead, `r` refreshes, `q`
quits. Non-tty stdin falls back to a display-only refresh loop (that's
what tests and pipes see).

