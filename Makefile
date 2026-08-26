# posse — herdr-native build & test. The tmux-era implementation (bash + Go,
# with its own suite) lives on the tmux-reference branch.
#
# Three artifacts, one promotion step:
#   make build    → bin/posse-go          dev build of the working tree
#   make release  → bin/posse-release     clean build of HEAD, in a temp worktree
#   make install  → $(BINDIR)/posse       the fleet's binary (PATH + plugin)
# `install` is denied to fleet personas (.claude/settings.json); a human runs it.
#
# install builds `release`, never `build`: personas share this checkout, so the
# working tree usually holds somebody's unfinished edits. See scripts/clean-build.sh.

GOBIN  ?= go
BINDIR ?= $(HOME)/.local/bin

# Stamp the dev build so `posse version` / the cockpit say which build is live.
# The release build stamps itself — it knows its sha is clean.
GIT_SHA   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_DIRTY := $(shell test -n "$$(git status --porcelain 2>/dev/null)" && echo -dirty)
LDFLAGS   := -X github.com/ranger360ai/posse/internal/rhq.Build=$(GIT_SHA)$(GIT_DIRTY)

.PHONY: build release install deploy test test-linux vet fmt link-plugin install-detection verify-detection verify-prune-guard verify-id-recycle verify-grok-pin audit-silent-reverts release-artifacts tap-formula cleanroom cleanroom-verify cleanroom-shell cleanroom-reset

build:
	$(GOBIN) build -ldflags '$(LDFLAGS)' -o bin/posse-go ./cmd/posse

# Build HEAD in a throwaway worktree, so the working tree cannot reach it.
release:
	GOBIN='$(GOBIN)' scripts/clean-build.sh bin/posse-release

# Promote the release build to the live binary. install(1) unlinks the target
# before writing, so running cockpits/dispatches keep their old inode.
#
# THE `rhq` SYMLINK IS TRANSITION MECHANICS, NOT A SECOND NAME (rangerhq-tyay).
# On promote day an instance is still full of the old spelling — persona standing
# orders, permission allowlists, saved recipes (the live dispatch loop's own
# recipe names an absolute .../plugin/bin/rhq), the operator's fingers. So the
# rename ships with a same-inode alias beside the binary and nothing breaks at
# the moment of promotion. It is scheduled for removal, not for documenting:
# `posse` is the command. Retiring the link is the operator's call, and the
# thing to check first is `grep -rn '\brhq\b' ~/.config/rhq` plus this repo's
# .claude/settings.json. Relative on purpose — $(BINDIR) may be moved or
# symlinked itself, and a relative link follows it.
install: release
	install -d $(BINDIR)
	install -m 0755 bin/posse-release $(BINDIR)/posse
	ln -sfn posse $(BINDIR)/rhq
	@echo "installed: $(BINDIR)/posse"
	@echo "  alias   : $(BINDIR)/rhq -> posse (transition only, rangerhq-tyay)"
	@echo "  promoted: $$(git rev-parse --short HEAD) $$(git log -1 --format=%s HEAD)"
	@echo "  version : $$($(BINDIR)/posse version)"
	@which posse >/dev/null 2>&1 && [ "$$(which posse)" != "$(BINDIR)/posse" ] && \
		echo "  WARNING : PATH resolves posse to $$(which posse), not the binary just installed" || :

deploy: install

# ------------------------------------------------------------------ shipping
#
# `release` above builds ONE binary for THIS machine and is the promote
# rehearsal. These two build what the world downloads, and they are a
# different thing: four cross-compiled tarballs plus the Homebrew formula
# that names their sha256s (rangerhq-i0n0).
#
# Neither target tags, publishes, or talks to GitHub — they write dist/ and
# stop. The release itself is .github/workflows/release.yml, fired by the
# operator pushing a `vX.Y.Z` tag, and it drafts rather than publishes.
# docs/runbooks/release.md is the whole procedure including the two clicks
# that are the operator's.
#
#   make release-artifacts VERSION=v0.3.0   -> dist/*.tar.gz + dist/checksums.txt
#   make tap-formula       VERSION=v0.3.0   -> dist/posse.rb (needs checksums)
#
# VERSION must agree with internal/rhq.Version; release-artifacts.sh refuses
# the build otherwise, because a binary whose `posse version` contradicts its
# own download URL is worse than no release.
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null)

release-artifacts:
	@[ -n "$(VERSION)" ] || { echo "make release-artifacts: VERSION=vX.Y.Z (HEAD carries no exact tag)" >&2; exit 2; }
	scripts/release-artifacts.sh --version '$(VERSION)'

tap-formula: release-artifacts
	scripts/tap-formula.sh --version '$(VERSION)' --checksums dist/checksums.txt --out dist/posse.rb
	@echo "--- dist/posse.rb (copy into ranger360ai/homebrew-tap as Formula/posse.rb) ---"
	@cat dist/posse.rb

# Unit tests are hermetic: the test binary re-execs as a fake herdr.
# The silent-revert audit runs here (0.2s) because rangerhq-8rtf is precisely
# the failure a green suite does not report: the commit that removed the fix
# also re-skipped its regression pin. A script that has to be REMEMBERED is
# the same objection rangerhq-2f5r raised about the private-index discipline,
# so it gets a trigger rather than a runbook entry.
test:
	$(GOBIN) test ./...
	@scripts/audit-silent-reverts.sh --quiet

vet:
	$(GOBIN) vet ./...

# The same gate, on Linux, from a mac (ranger-base-dbe). The suite had only
# ever been run on darwin, and two defects lived in that gap — ranger-base-fjj
# (ServerGen fences herdr generations on an inode number, which ext4 recycles
# and APFS does not: a runtime bug in the linux tarballs, 9 tests red) and
# ranger-base-gaf (a test hardcoding /bin/zsh). Both were found on a release
# rehearsal, because .github/workflows/release.yml was the first thing that had
# ever run the suite on Linux — on a tag, which is the worst place to learn it.
#
# `go vet ./... && make test` in a throwaway golang container, repo mounted
# READ-ONLY and running as you, so it cannot leave anything in the tree.
# ~35s cold, ~2s warm. IMAGE= / PLATFORM= overrides and a --shell in the
# script; docker required.
test-linux:
	scripts/test-linux.sh

fmt:
	gofmt -w cmd internal embed.go

# Register the cockpit plugin with the running herdr (local dev link).
# The manifest runs ./bin/posse relative to the plugin root; that is a symlink
# to the *installed* binary, so the popup never runs an unpromoted build.
# plugin/bin/rhq is the same transition alias as $(BINDIR)/rhq, and here it is
# load-bearing rather than habitual: a session recipe records the ABSOLUTE
# command it was launched with, and the fleet's own dispatch --watch loop is
# recorded as `<repo>/plugin/bin/rhq dispatch --watch`. Dropping this link
# would break relaunching the loop that dispatches everything else.
link-plugin:
	mkdir -p plugin/bin
	ln -sfn $(BINDIR)/posse plugin/bin/posse
	ln -sfn $(BINDIR)/posse plugin/bin/rhq
	herdr plugin link $(CURDIR)/plugin

# Install every agent-detection override in etc/herdr/agent-detection into
# herdr and reload it in the running server. Each override exists because
# herdr's stock manifest reports a screen that is HOLDING THE KEYBOARD as
# idle — no rule matches and detection falls through to
# default_known_agent_idle_fallback — so a dispatched prompt is typed into
# that screen instead of the composer, silently:
#   codex.toml  the "Hooks need review" dialog and every codex modal sharing
#               its "esc to go back" footer          (rangerhq-7ia)
#   grok.toml   the startup splash: New worktree / Resume session menu,
#               changelog line, "Help improve Grok"  (rangerhq-37c)
# Local overrides shadow both the cached remote and the bundled manifest and
# survive `herdr update`; verify-detection warns when either moves past our
# fork point.
HERDR_DETECTION_DIR ?= $(HOME)/.config/herdr/agent-detection
HERDR_DETECTION_SRC := $(wildcard etc/herdr/agent-detection/*.toml)

install-detection:
	install -d $(HERDR_DETECTION_DIR)
	for f in $(HERDR_DETECTION_SRC); do install -m 0644 "$$f" $(HERDR_DETECTION_DIR)/; done
	herdr server reload-agent-manifests >/dev/null
	@for f in $(HERDR_DETECTION_SRC); do echo "installed: $(HERDR_DETECTION_DIR)/$$(basename $$f)"; done
	@$(MAKE) --no-print-directory verify-detection

verify-detection:
	scripts/verify-detection.sh

# The promote gate for the session-meta prune guard (rangerhq-m15): plants
# ghost metas in a scratch RHQ_HOME and lists them against the LIVE herdr,
# which is only ever read. Pass RHQ=<path> to test a candidate before it is
# promoted; the default is whatever PATH resolves, i.e. the fleet's binary.
# (POSSE= was RHQ= before the command rename, rangerhq-tyay.)
POSSE ?= $(shell command -v posse)

verify-prune-guard:
	scripts/verify-prune-guard.sh $(POSSE)

# herdr workspace-id recycling (rangerhq-6bg7 / rangerhq-6bbz). Scratch
# --session server only; the fleet default server is snapshotted, never
# aimed at. Allocator is max(live)+1 recomputed at every process start.
verify-id-recycle:
	scripts/verify-id-recycle.sh

# The grok version pin (rangerhq-y7jr). grok ships `[cli] auto_update = true`
# and a leader process that downloads a new binary and relaunches itself
# mid-life, so the fleet runtime can roll forward with no review — retiring
# version-verified findings (consent-record has no server handler in 1.0.5;
# CLI --permission-mode beats config permission_mode) that fleet safety rests
# on. etc/grok/version-pin.toml is the declaration; this asserts the live
# ~/.grok/config.toml still matches it, and prints the security re-audit list
# when upstream stable moves past the pin. Lifting the pin is the operator's.
verify-grok-pin:
	scripts/verify-grok-pin.sh

# Silent-revert audit (rangerhq-8rtf). ef8d35f, a landed P1 fix, was undone by
# a `bd sync` commit and re-landed 3h52m later; `go test ./...` stayed GREEN the
# whole time, because the same commit that removed the fix restored the t.Skip
# on its regression pin. No test, gate or status line reports that class — a
# human found it by accident reading history for another reason. This is the
# detector: one `git log --raw` pass flagging any commit that puts a path back
# to a state it held before its immediately preceding change — where ABSENCE is
# a state, so the deletion of a file added earlier counts (rangerhq-ypn1: when
# the change landed from the private index was an ADD, the stale index undoes it
# by DELETING, and that half used to score clean). Triaged commits live in
# scripts/silent-reverts.allow; anything untriaged exits 1.
# Run `scripts/audit-silent-reverts.sh --self-test` to prove the detector still
# fires — it plants the real mechanism (a private GIT_INDEX_FILE commit followed
# by a shared-index commit) in a throwaway repo, in BOTH its shapes, and asserts
# the flag; it also plants a plain move and asserts silence.
audit-silent-reverts:
	scripts/audit-silent-reverts.sh

# ---------------------------------------------------------------------------
# clean-room test environment (ranger-base-5zh)
#
# A throwaway Debian container with a DEFAULT PATH and nothing from this
# project — the machine on which the PUBLIC install story gets tested. Its
# value is in what it does NOT contain: ~/go/bin is deliberately NOT on PATH
# in there, because that omission is the P1 under test (ranger-base-253).
# `make cleanroom-verify` asserts that and every other guarantee; run it
# before a test pass. Full runbook: etc/cleanroom/README.md.
# ---------------------------------------------------------------------------
cleanroom:
	scripts/cleanroom.sh start

cleanroom-verify:
	scripts/cleanroom.sh verify

cleanroom-shell:
	scripts/cleanroom.sh shell

cleanroom-reset:
	scripts/cleanroom.sh reset
