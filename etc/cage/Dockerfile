# The posse cage image — ADR 0002 §3 L4, bead rangerhq-9fv.
#
# This is the whole userland a caged persona gets: the runtime CLI it runs
# on, the tools a persona actually types, and LINUX builds of bd and posse,
# because a persona that cannot claim or close a bead is not doing the
# work (both are macOS binaries on the host; .beads/bd.sock itself passes
# through the repo mount fine).
#
# Nothing host-specific belongs in here. The repo, the persona's memory,
# its PID and its HOME arrive as bind mounts at launch, at the same paths
# they have on the host — see internal/posse/cage.go (CageMounts).
#
# Built by `posse cage build [dir]`, which cross-compiles the two Go
# binaries into bin/ next to a copy of this file and hands that directory
# to the engine. Building it by hand means staging bin/posse and bin/bd
# yourself first.

FROM node:22-slim

# The persona's toolbox: git because the work is a repo, ripgrep because
# every runtime shells out to it, procps because process questions get
# asked, ca-certificates because everything else is TLS.
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep procps less \
    && rm -rf /var/lib/apt/lists/*

# The runtime CLIs. claude is the only runtime whose container credential
# is decided (rangerhq-kiz: CLAUDE_CODE_OAUTH_TOKEN, minted by the
# operator), and cage.go refuses to launch the others until theirs is — so
# it is the only one installed by default. Add one when its lane gets
# there:  posse cage build --runtimes "@anthropic-ai/claude-code @openai/codex"
ARG RUNTIMES="@anthropic-ai/claude-code"
RUN npm i -g $RUNTIMES && claude --version

# The repo arrives as a bind mount owned by the host operator (VirtioFS
# maps root-inside to operator-outside), so git inside sees a "dubious
# ownership" repo unless it is told not to care — the one safe.directory
# line the rangerhq-89a spike measured.
RUN git config --system --add safe.directory '*'

# Linux bd and posse, cross-built on the host by `posse cage build`. bd is
# pinned to the version the fleet's databases are written by (0.49.1 —
# 1.2.x is the Dolt migration and cannot read them).
COPY bin/ /usr/local/bin/
RUN bd --version && posse version
