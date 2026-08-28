## `posse refresh` — the one human-gated credential write (ranger-base-h207)

ADR 0019 D4 amends "posse never mints, refreshes, or writes a credential" to
"never **autonomously**". `posse refresh` (`internal/rhq/refresh.go`) is the
whole of that exception, and everything about it is shaped so the exception
cannot widen by accident.

**The gate is on the verb, not on a branch of it.** The command refuses
without a TTY and refuses under `RHQ_PERSONA` — *including the no-argument
report*, which writes nothing. That is deliberate: the second spelling of
this gate is `Bash(posse refresh:*)` in every crew PID, and a deny rule does
not know about arguments. A binary that gates itself more narrowly than the
rule that spells it is a gate with a hole in it. That PID half lives in the
constitution repo; **this repo ships the mechanism and does not edit PIDs**.

**Three things it will not write, each for its own reason.**

| refused | why |
|---|---|
| a `meter` credential | the rotating OAuth token has one writer — the runtime's own login loop. Every second copy is a snapshot that disagrees with the source exactly when it matters; that copy existed once in `default.env` and 401'd the fleet twice. `posse refresh <rt> meter` prints where the store of record is and writes nothing. |
| `ANTHROPIC_API_KEY`, by name | metered spending, rejected on the money line (rangerhq-kiz). |
| an `sk-ant-api…` value, by shape | the same key pasted into the session variable spends the same money wearing the right label. A shape check, not an authority — but the failure it catches is silent and looks exactly like success. |

**The mint is run, not scraped.** `claude setup-token` runs with stdin,
stdout and stderr **inherited** — posse captures nothing — and then posse
asks for the token it printed. Two steps where one would look neater, and the
reason is that a captured stdout is a pipe: an interactive flow would render
into a buffer nobody is watching while the operator stares at a dead
terminal, and whether its prompts land on stdout or stderr is somebody else's
implementation detail. The paste step is also the whole of `--paste` (the
headless box), so exactly one code path ever holds a token.

**Stamps are comments, and an unknown date stays unknown.** The write puts
`# minted=YYYY-MM-DD` (and `# expires=` *only* when `--expires` gives one)
directly above the variable. They are comments, so `parseEnvLines`, `posse
envs` and a shell that sources the file all see exactly what they saw before,
and a stamp can never be injected as a variable. posse has no way to ask a
setup-token when it dies: with no `--expires` the report says **"cannot
tell"**, which is an answer, and it warns nothing (ADR 0019 D5). A stamp
belongs to the variable it sits *directly* above — a stamp read off the wrong
variable is a date reported about a credential it is not true of.

**Writes never widen.** A temp file in the same directory at 0600, renamed
over; the destination is refused unless it is a regular file (a symlink there
would aim a credential write at a path posse did not choose).
`TightenEnvPerms` is the second belt, not the first — the writer's own mode is
pinned separately, because a test that only checks the file's final mode is
green over a 0644 write that a later pass tightened.

**What is NOT here.** The session credential's `ExpiresAt` does not yet reach
the seam's own `CredMeta`: `readSessionCredential` is a package function with
no `*App`, so it cannot find the env set the value came from, and the report
reads the stamps itself. Surfacing it through the seam (and in the cockpit
header and the dispatch line) is ranger-base-k6ha.
