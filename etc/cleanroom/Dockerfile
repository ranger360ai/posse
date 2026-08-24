# syntax=docker/dockerfile:1
#
# Clean-room image for testing the PUBLIC install story — ranger-base-5zh.
#
# THIS IMAGE IS A TEST INSTRUMENT. Its value is in what it does NOT contain.
# Read ranger-base-253 before changing anything here. The P1 under test is that
# `go install` writes the binary to $(go env GOPATH)/bin — normally ~/go/bin —
# which is NOT on a default PATH, so the next advertised command dies with
# "command not found".
#
#   *** DO NOT PUT ~/go/bin (or $GOPATH/bin, or $GOBIN) ON PATH. ***
#   *** That omission IS the bug under test. Add it and the P1 goes invisible. ***
#
# For the same reason this image does NOT use the official `golang` images:
# they ship `ENV PATH=/go/bin:/usr/local/go/bin:$PATH` and would silently
# defeat the entire exercise.
#
# Also absent on purpose — each one hides a defect from a new user:
#   herdr · bd · RHQ_HOME · ~/.config/rhq · a warmed Go module cache ·
#   any copy of this repo · any file from the dev box.
#
# What IS present, and it is a short list on purpose (see the runbook: an
# undocumented prerequisite is a FINDING, so you need to know what you were
# handed for free): ca-certificates, curl, git, make, less.

FROM debian:trixie-slim

# Generic newcomer baseline. Nothing here is from this project.
#   git  — the repo instructions start with a clone
#   make — the README short form is `make build`
#   curl, ca-certificates — fetching the Go toolchain, and TLS to the proxy
#   less — reading files during a test run
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      git \
      less \
      make \
 && rm -rf /var/lib/apt/lists/*

# Go toolchain, obtained exactly the way a newcomer does: the official
# linux-arm64 archive from go.dev/dl, checksum-verified, unpacked to
# /usr/local/go. This is https://go.dev/doc/install verbatim.
ARG GO_VERSION=1.27.0
ARG GO_ARCH=arm64
ARG GO_SHA256=51798d2c42d0e1c6ed7fd9f48728b4193abac9e8aad6dbac2fe96a81f5909bda
RUN curl -fsSLO "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" \
 && echo "${GO_SHA256}  go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" | sha256sum -c - \
 && rm -rf /usr/local/go \
 && tar -C /usr/local -xzf "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" \
 && rm "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"

# PATH. Read this block before editing it.
#
# The ONLY addition to the default PATH is /usr/local/go/bin, which is the one
# line https://go.dev/doc/install tells a newcomer to add. It is placed in
# /etc/profile.d so it reaches login shells the ordinary way, exactly as a
# newcomer's own edit would.
#
# There is deliberately NO `ENV PATH=...` in this Dockerfile: an ENV PATH would
# leak into non-login `docker exec` shells and give a PATH no real user has.
# Enter this container through a LOGIN shell (scripts/cleanroom.sh does).
RUN printf '%s\n' 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh \
 && chmod 0644 /etc/profile.d/go.sh

# An ordinary unprivileged user with an ordinary home directory. Nothing is
# seeded into it — no dotfile edits, no RHQ_HOME, no ~/.config/rhq, no ~/go.
RUN useradd --create-home --shell /bin/bash --uid 1000 tester

# No GOPATH, GOBIN, GOPROXY, GOFLAGS, GOPRIVATE or GONOSUMDB are set anywhere.
# `go install` must resolve through the real public proxy with real defaults,
# so the fetch under test is the public path and not a local replay.

# PID 1 idles as root purely so `cleanroom shell` can hand out a genuine
# `su - tester` login session. Nothing is ever run as root by the test.
CMD ["sleep", "infinity"]
