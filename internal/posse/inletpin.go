package posse

import "os"

// The transport/exec half of the settings pin (ranger-base-rflee, widening
// ranger-base-rq83c).
//
// rq83c pinned the two credential-dir variables. The audit behind this file
// (ranger-base-0w16r) found those were one instance of a larger primitive:
// a settings file's `env` block is assigned over process.env at startup and
// is inherited by every child the session execs, so ANY variable a lower
// settings scope names reaches the tools. The credential dirs were the two
// that moved a credential; these are the ones that move CODE and TRAFFIC.
//
// The runtime's own bundled text calls this class "unsandboxed-exec
// inlets"; the name here is its word, not a new one.
//
// WHAT A PIN CAN AND CANNOT DO. A settings payload can only SET keys, never
// remove them, so this is a denylist and its coverage is exactly the names
// below. That is a real limit and it is the reason the list is written as a
// table with a measurement against each row rather than as a blob: a name
// that is not here is not covered, and a reader has to be able to see that.
//
// WHY THE VALUES ARE NOT ALL "". rq83c's empty-string rule is per key, not
// a house style (ranger-base-ig4op). Two independent reasons drive the
// values below, both measured on this box on 2026-09-05:
//
//   - An empty value is not neutral to every reader. GIT_SSH_COMMAND="" is
//     not "no ssh command", it is the command `` — `git ls-remote ssh://…`
//     under it dies with `error: cannot run : No such file or directory`,
//     while GIT_SSH_COMMAND=ssh is byte-identical to unset. Pinning that
//     one empty would have broken every ssh remote in the fleet.
//   - An empty value may not REACH. The operator measured
//     (ranger-base-sn0w8, 2026-09-04, claude 2.1.260) that an empty string
//     in the root-owned policy tier does not take, while a non-empty one
//     overrides even the process environment. The launcher's own pin rides
//     at flag scope, where rq83c measured "" honored — but this same table
//     is what the policy-tier file is rendered from, and the policy tier is
//     the only end that covers the operator's OWN uncaged session. So where
//     a non-empty spelling is neutral, it is preferred: it is the spelling
//     that works at both ends.
//
// Where no non-empty neutral exists the row says so and is pinned empty,
// which is honest about being flag-scope-effective and possibly a no-op in
// the policy file. DYLD_INSERT_LIBRARIES is the sharp case: /dev/null and
// " " both abort the child outright (exit 134, dyld "tried: … not a file"),
// so "" is the only value that is not a fleet-wide outage.
//
// COST ON THIS BOX: none, measured. A census of all of these names found
// none set in the launcher's environment, in launchd's user environment, in
// any shell rc, or in the user-scope settings.json; and none of the git
// config keys a pin here would shadow (core.sshcommand, core.pager,
// diff.external) is set either. The pin is purely preventive today, so it
// overrides nothing the operator is using.
//
// NOT COVERED HERE, deliberately: the command-string FIELDS a settings file
// also carries (processWrapper, apiKeyHelper, proxyAuthHelper,
// awsAuthRefresh, statusLine.command, hooks). An `env` pin does not reach a
// field, and the blanket `disableAllHooks` the audit proposed would remove
// the crew's own bd argv gate and herdr's state reporter along with an
// attacker's hook. That half needs a per-field pin or an operator ruling,
// and is filed separately rather than half-done here.
//
// ALSO NOT COVERED, and these two are inlets that FIRE: GIT_CONFIG_COUNT
// with its GIT_CONFIG_KEY_n/VALUE_n indices, and GIT_CONFIG_GLOBAL. Both
// were named in ranger-base-rflee's fix spec as part of "GIT_CONFIG_*" and
// both were left out of the table silently, which is the defect
// ranger-base-44or9 is about — under this file's own contract a name that is
// not here is not covered, and a reader has to be able to SEE that. They are
// still not pinned, but now for a measured reason each (2026-09-05, git
// 2.50.1 Apple Git-155):
//
//   - GIT_CONFIG_COUNT has no pinnable value at all. `0` closes the inlet —
//     and closes posse's OWN L3 hooks redirect with it, because ADR 0052 D2
//     aims every session's git at its rendered hooks dir through exactly
//     this mechanism (gitConfigHooksPathVars, appended to the pane env at
//     herdrback.go). A settings `env` block is assigned OVER process.env, so
//     a pinned `0` would zero the count the pane set and leave KEY_0/VALUE_0
//     stranded: measured, the redirected post-checkout fires with the pane
//     vars alone and goes quiet the moment `0` is laid over them. That is
//     the bd argv gate and the employer's managed hooks, off, fleet-wide.
//     Any value ≥ 1 is worse — it names indices the session never received
//     and git dies `missing config key GIT_CONFIG_KEY_0`, rc 128, on every
//     command (the ranger-base-buvq4 outage, recorded in hooksredirect.go).
//   - GIT_CONFIG_GLOBAL has no neutral spelling, empty or not. Both
//     /dev/null and "" close the inlet by replacing ~/.gitconfig wholesale,
//     and this box's ~/.gitconfig is where user.name and user.email live. It
//     does not fail loudly: git falls back to a gecos-and-hostname ident and
//     commits at rc 0, measured — the author line came out as the account's
//     gecos name at `<login>@<hostname>.local`, not as the configured
//     address, with no warning on either stream. A pin whose cost is every
//     crew commit silently misattributed is not a pin. Worse, it would
//     switch off a guardrail: DeriveIdentityLiterals (visibility.go) walks
//     `git config --get-all user.email` for every scope to build the wall
//     that keeps the box's addresses out of a public repo, and under a
//     /dev/null global that command exits 1 with no output — the renderer's
//     own "nothing to say" branch — so the wall would come out with no
//     e-mail literal in it, silently. Measured both arms in this repo.
//     ORDERS.md already had the same effect from the other end: a
//     hook-freshness reference rendered under that variable loses three
//     `posse_check 'email'` literals and reads every repo as STALE. The
//     only neutral value is the path git already resolves to, which is
//     per-box, and the shipped drop-in is a constant this table is held
//     equal to — so there is nothing to write in it.
//
// So the GIT_CONFIG_* family is NARROWED here, not closed: an attacker who
// can set a lower-scope `env` block still has GIT_CONFIG_COUNT and
// GIT_CONFIG_GLOBAL. Closing either costs something the operator has to
// accept, so it is filed for them as ranger-base-37y0z rather than decided
// here. TestQATheInletPinCoversTheGitConfigFamilyOrSaysWhyNot holds this
// paragraph against the live behaviour: it fails if either name stops being
// disclosed, and it fails if either gap closes and the prose stops being
// true.

// inletPin is the transport/exec half of what a launch pins. Each row's
// value is the one measured to leave this box's behaviour exactly where it
// already is while denying the inlet — the same rule credentialDirPin
// follows ("the value this environment already resolves to"), applied to
// the readers a session execs — bash, sh, dyld, git and node — and to
// claude itself, which reads the NODE_* and transport names too.
//
// The measurements, one line per row, all by execution on 2026-09-05
// (darwin 25.4.0, bash 3.2.57, git 2.50.1, node v25.2.1, claude 2.1.261).
// Each is a three-arm probe — unset, attack, pin — because a pin whose
// attack arm never fired has measured nothing (ranger-base: "probe needs a
// failing wrong arm"):
//
// READ THE READER OFF EACH ROW BEFORE TRUSTING IT. Neutrality is per
// reader, and the NODE_* rows have TWO that matter and they disagree:
// homebrew node v25.2.1, and the node-compatible runtime inside claude
// itself, which on 2.1.261 is a Bun 1.4.1 compiled binary (`strings` on the
// executable names bun-v1.4.1; there is no node in it at all).
// NODE_EXTRA_CA_CERTS is the row where they diverged — node forgiving what
// bun does not — and shipping the node-only measurement took the operator's
// client off the endpoint for every session on the box (ranger-base-xxdn4).
// Where a row below is about reaching the API, the arm that counts is the
// real claude binary and nothing else.
//
//	BASH_ENV=/dev/null              attack sourced a marker script into
//	                                `bash -c :`; /dev/null quiet, bash still
//	                                runs, stderr silent.
//	ENV=/dev/null                   same, via `sh -ic :` — this one needs an
//	                                interactive sh, so it is the weaker
//	                                inlet of the pair, not an unreachable one.
//	DYLD_INSERT_LIBRARIES=""        attack ran a constructor inside homebrew
//	                                node and jq (adhoc-signed, so dyld honors
//	                                it; Apple's SIP-protected /usr/bin/git
//	                                strips it). "" quiet. No non-empty
//	                                neutral exists: /dev/null and " " abort
//	                                the child (exit 134).
//	LD_PRELOAD=""                   the linux twin of the row above. Inert on
//	                                darwin and therefore NOT measured here;
//	                                pinned because it costs a launch nothing
//	                                and the fleet is not darwin by contract.
//	NODE_OPTIONS=" "                attack was --require of a marker script
//	                                into `node -e ''`; " " quiet and node
//	                                still runs. A single space is non-empty
//	                                (so it reaches the policy tier) and
//	                                parses to no options.
//	NODE_EXTRA_CA_CERTS=""          THE ROW THAT BROKE THE OPERATOR'S CLIENT,
//	                                and the reason this table now names its
//	                                reader per row. /dev/null was measured
//	                                neutral against node v25.2.1, which warns
//	                                ("Ignoring extra certs … load failed")
//	                                and carries on with its built-in roots.
//	                                claude 2.1.261 is NOT node: it is a Bun
//	                                1.4.1 compiled binary, and its HTTPS
//	                                agent warns the same and then dies —
//	                                every request fails with
//	                                `FailedToOpenSocket`, which is the
//	                                "having trouble reaching api" the
//	                                operator saw (ranger-base-xxdn4).
//	                                Measured 2026-09-05 against the real
//	                                binary, --bare + a junk API key, where a
//	                                healthy transport answers 401 and a
//	                                broken one answers FailedToOpenSocket:
//	                                  /dev/null            BREAKS
//	                                  an empty regular file BREAKS
//	                                  a MISSING path        401 (healthy)
//	                                  ""                    401 (healthy)
//	                                  /etc/ssl/cert.pem     401 (healthy)
//	                                So the trigger is not the path being
//	                                bogus, it is the file READING and
//	                                yielding zero PEM blocks: the CA list
//	                                becomes empty and the agent is left with
//	                                no trust anchors at all. A missing path
//	                                skips the load and keeps them.
//	                                Pinned "" — flag-scope-effective and
//	                                expected to be a no-op at the policy
//	                                tier, on the same standing as the
//	                                proxies. A non-empty neutral DOES exist
//	                                (a path that does not exist, which
//	                                reaches the policy tier and adds no CA)
//	                                and is NOT taken: it rests on that path
//	                                staying absent, and it prints two
//	                                "Ignoring extra certs" warning lines at
//	                                every claude start. Pointing the row at a
//	                                real bundle reaches too, but appends a
//	                                root set rather than adding nothing, so
//	                                it widens trust instead of pinning it.
//	                                Both are the operator's to overrule.
//	NODE_TLS_REJECT_UNAUTHORIZED=1  against a self-signed TLS server on
//	                                loopback: unset REJECTED, "0" ACCEPTED
//	                                (this is the row that completes an MITM),
//	                                "1" and "" both REJECTED. "1" is pinned
//	                                because it is the non-empty one.
//	GIT_SSH_COMMAND=ssh             attack ran a marker script as the ssh
//	                                transport; "" BREAKS ssh outright; `ssh`
//	                                reproduces unset exactly.
//	GIT_EXTERNAL_DIFF=""            NOT NEUTRAL, and the row whose own
//	                                measurement was taken with the one
//	                                arm that could not see the cost. The
//	                                attack was real — a marker script
//	                                named here RAN as the diff driver of
//	                                `git diff HEAD~1` — but the arm that
//	                                cleared "" was --shortstat, which is
//	                                one of the formats that never reaches
//	                                a driver at all. Git does not read
//	                                set-but-empty as unset: it execs "",
//	                                which is the GIT_SSH_COMMAND="" trap
//	                                two rows up, walked into. Re-measured
//	                                2026-09-06 with a MARKER driver, so
//	                                what is graded is whether the driver
//	                                was invoked and not whether "" was
//	                                harmless:
//	                                  DRIVER RUNS, so "" is rc 128
//	                                  `error: cannot run :` — git diff,
//	                                  --cached/--staged, git diff <rev>,
//	                                  -U0, --exit-code; and show/log -p
//	                                  --ext-diff, which is opt-in.
//	                                  DRIVER NEVER RUNS, so "" is
//	                                  byte-identical to unset — --stat
//	                                  --shortstat --numstat --name-only
//	                                  --name-status --raw --check
//	                                  --quiet --no-ext-diff; git show,
//	                                  git log -p and format-patch (the
//	                                  log family defaults ext-diff OFF);
//	                                  diff-tree -p, diff-index -p,
//	                                  stash show -p, range-diff.
//	                                So the price is the `git diff`
//	                                PORCELAIN asking for patch format and
//	                                nothing else — NOT "all patch output",
//	                                which is why a count of what this row
//	                                breaks has to grade the FORMAT and not
//	                                the verb. No neutral spelling exists:
//	                                every value is EXECUTED, and git's way
//	                                of saying "use the internal diff" is
//	                                the --no-ext-diff FLAG, which an env
//	                                pin cannot supply. So this row is
//	                                fail-closed LOUDLY, not quiet, and it
//	                                is flag-scope-effective — "" does not
//	                                take at the policy tier
//	                                (ranger-base-sn0w8), so it costs
//	                                posse-launched seats and covers the
//	                                operator's uncaged sessions not at all.
//	                                posse's own `git diff` readers each
//	                                state a format and are immune
//	                                (memoryDiff, ranger-base-xw51s); a
//	                                bare `git diff` anywhere else in a
//	                                seat is not. KEEPING it at that price
//	                                or moving it to ALSO NOT COVERED
//	                                beside GIT_CONFIG_GLOBAL is the
//	                                operator's, filed as ranger-base-5sph1
//	                                — disclosed here, not decided here,
//	                                exactly as the GIT_CONFIG_* pair above.
//	GIT_PAGER=""                    the honest row: the attack arm stayed
//	                                QUIET, because git does not page without
//	                                a TTY and a fleet Bash call is a pipe. So
//	                                no inlet was demonstrated on this box.
//	                                Pinned anyway — "" still printed the log
//	                                — but as defence in depth for a
//	                                TTY-wrapped call, not as a measured fix.
//	GIT_CONFIG_SYSTEM=/dev/null     attack pointed it at a config naming
//	                                core.hooksPath; the attacker's
//	                                post-checkout FIRED under the three git
//	                                rows above, /dev/null quiet. Neutral
//	                                here by a byte-identical `git config
//	                                --list --show-origin`, because Apple git
//	                                2.50.1 reads its own bundled
//	                                /Library/Developer/CommandLineTools/…/
//	                                gitconfig by a path this variable does
//	                                not govern — so osxkeychain and
//	                                init.defaultBranch survive the pin. That
//	                                bundled file is NOT system scope, though,
//	                                and the suppression this row performs is
//	                                not conditional on the platform: the row
//	                                empties system scope on darwin exactly as
//	                                it does elsewhere. WHICH FILE that is, is
//	                                a property of the git BINARY and not of
//	                                one path. git reads its own build-time
//	                                sysconfdir, so /usr/bin/git here and a
//	                                distro git both read /etc/gitconfig,
//	                                while a Homebrew git on apple silicon
//	                                reads
//	                                /opt/homebrew/etc/gitconfig
//	                                — that second one is CI's macos runner,
//	                                and asserting the first as universal is
//	                                what redded ci.yml (measured 2026-09-06
//	                                against the git 2.55.0 bottle,
//	                                ranger-base-33r36). `git config --system
//	                                --list --show-origin` names the file YOUR
//	                                git wanted, and that answer beats this
//	                                row. What is zero here is a MISSING FILE,
//	                                a fact about this box rather than about
//	                                the platform (ranger-base-sv8x4).
//	                                AND READ THAT SUPPRESSION AGAINST THE
//	                                GIT_CONFIG_GLOBAL PARAGRAPH ABOVE: it
//	                                reaches the same guardrail, one scope
//	                                instead of every scope.
//	                                DeriveIdentityLiterals (visibility.go)
//	                                walks `git config --get-all user.email`
//	                                across all of them to build the wall
//	                                that keeps this box's addresses out of a
//	                                public repo, and a scope this row
//	                                emptied contributes nothing through the
//	                                renderer's own "nothing to say" branch —
//	                                no error, one fewer e-mail literal, and
//	                                that is a WALL rendered short, not a
//	                                log line. Measured 2026-09-05, scratch
//	                                repo with global scope closed so only
//	                                the system arm moves: unset, that
//	                                address is not in the wall; this
//	                                variable at a config holding it, it IS;
//	                                /dev/null, gone again, rc 0. Zero on
//	                                THIS box — system scope is empty and
//	                                there is no /etc/gitconfig — so the row
//	                                stays pinned; on a box whose user.email
//	                                lives in system scope it is the identity
//	                                half of the wall, one address short,
//	                                and that is the operator's call on
//	                                ranger-base-zz08i (ranger-base-nn161).
//	GIT_CONFIG_PARAMETERS=""        the family member nobody named — not
//	                                rflee's fix spec, not the verify bead —
//	                                and it is the same inlet: an attacker's
//	                                `'core.hooksPath'='<dir>'` FIRED the hook
//	                                under all three git rows above, "" quiet
//	                                and the config listing byte-identical to
//	                                unset. No non-empty neutral exists: " "
//	                                is `error: bogus format in
//	                                GIT_CONFIG_PARAMETERS`, rc 128, on EVERY
//	                                git command. So this row is
//	                                flag-scope-effective and expected to be a
//	                                no-op in the policy file. It does not
//	                                cost `git -c` anything: git sets this
//	                                name itself for its own subprograms, over
//	                                whatever it inherited (measured, alias
//	                                included).
//	ANTHROPIC_BASE_URL=…anthropic.com   read from the bundle, which resolves
//	                                the endpoint as `ANTHROPIC_BASE_URL ||
//	                                CLAUDE_CODE_API_BASE_URL || "https://
//	                                api.anthropic.com"`. Because that is a
//	                                `||` chain, a NON-EMPTY value in the
//	                                first name short-circuits it and closes
//	                                the second name too, whatever an attacker
//	                                put there. Confirmed reaching by
//	                                execution: a flag-scope pin naming a
//	                                loopback port sent the session's request
//	                                to that port.
//	CLAUDE_CODE_API_BASE_URL=""     already shadowed by the row above; pinned
//	                                falsy rather than to a URL because other
//	                                readers of the name may not share that
//	                                default.
//	HTTPS_PROXY / HTTP_PROXY /      no non-empty neutral exists — there is no
//	  ALL_PROXY and the three       URL that spells "no proxy" — so these are
//	  lowercase spellings           pinned empty and are flag-scope-effective.
//	                                The lowercase names are pinned because
//	                                node, curl and git all read them and the
//	                                audit listed only the uppercase three.
//	CLAUDE_CODE_CERT_STORE /        absence is the safe state and "" is its
//	  CLIENT_CERT / CLIENT_KEY      only spelling; same caveat as the proxies.
//
// Order is fixed and alphabetical-by-group rather than incidental, so the
// rendered launch line is stable across launches and a diff of it is
// readable.
func inletPin() []EnvVar {
	return []EnvVar{
		// Exec inlets: a shell, the dynamic linker, and node.
		{Key: "BASH_ENV", Value: os.DevNull},
		{Key: "ENV", Value: os.DevNull},
		{Key: "DYLD_INSERT_LIBRARIES", Value: ""},
		{Key: "LD_PRELOAD", Value: ""},
		{Key: "NODE_OPTIONS", Value: " "},

		// Exec inlets git opens whenever a session runs git. Two more
		// of the GIT_CONFIG_* family are inlets and are NOT here; the
		// ALSO NOT COVERED paragraph above carries the measurement
		// that keeps each of them out.
		{Key: "GIT_SSH_COMMAND", Value: "ssh"},
		{Key: "GIT_EXTERNAL_DIFF", Value: ""},
		{Key: "GIT_PAGER", Value: ""},
		{Key: "GIT_CONFIG_SYSTEM", Value: os.DevNull},
		{Key: "GIT_CONFIG_PARAMETERS", Value: ""},

		// Transport: where the bearer goes, and who is trusted to
		// terminate the TLS it goes over.
		{Key: "ANTHROPIC_BASE_URL", Value: anthropicBaseURL},
		{Key: "CLAUDE_CODE_API_BASE_URL", Value: ""},
		{Key: "HTTPS_PROXY", Value: ""},
		{Key: "HTTP_PROXY", Value: ""},
		{Key: "ALL_PROXY", Value: ""},
		{Key: "https_proxy", Value: ""},
		{Key: "http_proxy", Value: ""},
		{Key: "all_proxy", Value: ""},
		{Key: "NODE_EXTRA_CA_CERTS", Value: ""},
		{Key: "NODE_TLS_REJECT_UNAUTHORIZED", Value: "1"},
		{Key: "CLAUDE_CODE_CERT_STORE", Value: ""},
		{Key: "CLAUDE_CODE_CLIENT_CERT", Value: ""},
		{Key: "CLAUDE_CODE_CLIENT_KEY", Value: ""},
	}
}

// anthropicBaseURL is the endpoint the runtime falls back to with neither
// base-URL variable set — read out of the 2.1.261 bundle rather than
// assumed, because pinning the WRONG default here would repoint the fleet
// as surely as an attacker would.
const anthropicBaseURL = "https://api.anthropic.com"

// settingsPin is everything a launch pins into the runtime's environment:
// the credential dirs (ranger-base-rq83c) and the transport/exec inlets
// (ranger-base-rflee). Credential dirs come first so the rendered order
// stays the one rq83c's pins already assert.
//
// No home is no credential-dir pin, and that stays true here — but the
// inlet rows do not depend on a home directory, so they are pinned anyway.
// A box with no home is a box whose launch still should not hand a persona
// the exec inlets.
func settingsPin() []EnvVar {
	return append(credentialDirPin(), inletPin()...)
}
