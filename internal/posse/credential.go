package posse

// The credential seam (ADR 0019, ranger-base-x584): the one place posse
// acquires a credential, and the only place that knows where a credential
// lives on this platform.
//
//	Read(runtime, purpose) → (secret, meta{Source, ExpiresAt}, error)
//
// Two purposes. `session` is what an authenticated caged session needs
// injected: a scoped, long-lived mint the operator placed in an env set by
// their own hand — posse-owned, store of record is the home, no platform
// dependency at all. `meter` is what posse presents to the provider's usage
// and models endpoints: the runtime's OWN rotating OAuth token, read where
// the runtime's login loop writes it. posse is a reader of that one, never a
// writer and never a copier — every second copy of a rotating credential is
// a snapshot that disagrees with the source exactly when it matters, which
// is what `default.env` was and what 401'd the fleet twice (ADR 0019's
// Context has the receipts).
//
// The meter store is per platform and chosen with `runtime.GOOS` at run
// time, not with build tags: `make test-linux` is a release gate and every
// branch of the switch must compile and be testable from either box.
//
//   - darwin: the macOS keychain item, read by execing /usr/bin/security
//     ABSOLUTELY (ranger-base-ypf5) — a PATH lookup here resolved to the
//     calling persona's own Bash(security:*) shim and refused posse's own
//     monitoring read.
//   - anything else: `.credentials.json` under the directory the runtime's
//     own secure storage writes it to (CredentialsFile, and it is NOT the
//     home by definition), Claude Code's own store of record where there is
//     no keychain, fed through the SAME envelope parser. One fixture, two
//     paths, one diagnosis (ADR 0019 V7): the shape diagnostics
//     ranger-base-okbr bought with an hour of stopped shop apply to both,
//     because there is only one of them.
//
// Nothing here quotes a credential. The errors name stores, key NAMES and
// shapes; the values never appear — that rule is inherited from the code
// this file collected and is the reason those errors are the shape they are.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// CredPurpose is what a credential is FOR, which is what decides where it is
// read from. The two are different kinds, not two settings of one kind: a
// session mint is posse-owned and lives where the operator put it; a meter
// token is runtime-owned and lives where the runtime put it.
type CredPurpose string

const (
	CredSession CredPurpose = "session"
	CredMeter   CredPurpose = "meter"
)

// CredMeta is everything the seam knows about a credential except its bytes.
//
// ExpiresAt's zero value is "cannot tell", and that is an honest answer the
// callers must render as itself — never as "fresh" (ADR 0019 D5). Nothing
// expiry-shaped gates anything: the read's success or failure stays the only
// actuator, and expiry is a warning (ranger-base-k6ha surfaces it).
type CredMeta struct {
	// Source names the store this answer came from, in the words an
	// operator would use to go look at it.
	Source string
	// Shape is the name of the credential layout that answered, when the
	// store holds a structured envelope. ranger-base-okbr fix 1: a shape
	// other than the first declared should be reportable rather than
	// silently relied on.
	Shape string
	// ExpiresAt is the credential's own expiry when the store carries one.
	ExpiresAt time.Time
}

// NoSource is the seam saying this (runtime, purpose, platform) has no store
// at all — not a failed read, not an outage: nothing here to read.
//
// It is a distinct type for the same reason NoPlanAdapter and GateRefusal
// are. Under ADR 0018 a *blind* meter has a clock on it that eventually
// stops an unattended fleet, and blindness is defined as "a source exists
// and the read failed". Reporting structural absence as blindness is a brake
// with no release — a Linux box with no login would park every on-meter bead
// forever on a condition no retry can change. So absence is UNCONFIGURED:
// the guard runs off, says so once, and the blind clock never starts
// (ranger-base-vmqg does that rendering).
//
// Store names what this platform would have to have, and Arm is what would
// arm it in the operator's own verbs — the two things a witness line needs
// to be actionable rather than merely true.
type NoSource struct {
	Runtime string
	Purpose CredPurpose
	GOOS    string
	Store   string
	Arm     string
}

func (e *NoSource) Error() string {
	s := fmt.Sprintf("no %s credential source for %s on %s", e.Purpose, e.Runtime, e.GOOS)
	if e.Store != "" {
		s += ": " + e.Store
	}
	if e.Arm != "" {
		s += " — " + e.Arm
	}
	return s
}

// NoSourceReason reads err as structural absence, or nil. It is the one
// place the two ways a NoSource arrives are read as one thing:
//
//   - on its own, from a read — the store was there when the adapter was
//     chosen and gone when the token was wanted, or the caller never asked
//     the availability question at all;
//   - inside a *NoPlanAdapter, from the availability check that caught it
//     BEFORE a reader was built (planusage.go PlanAdapter).
//
// Same platform, same store, same one-command fix. Which moment noticed is
// an implementation detail of the guard's plumbing, and the operator's
// answer must not depend on it — a race between two code paths deciding
// whether a fleet parks is the shape of bug this whole outcome class exists
// to remove.
//
// The *NoPlanAdapter arm is asked FIRST and asked by its own rule
// (soleNoSource): with several adapters, one missing a credential and one
// missing something else, a plain errors.As would find the NoSource and
// license a sentence that is not true. Structural absence is the answer
// only when it is the whole answer.
func NoSourceReason(err error) *NoSource {
	if err == nil {
		return nil
	}
	var na *NoPlanAdapter
	if errors.As(err, &na) {
		return na.soleNoSource()
	}
	var ns *NoSource
	if errors.As(err, &ns) {
		return ns
	}
	return nil
}

// ReadCredential is the seam. Nothing else in posse may acquire a credential
// (ADR 0019 D1); a vault, when it is priced (ranger-base-epz8), is a third
// answer to this call and not a second migration.
//
// It hangs off *App for one reason: the session half's expiry lives in the
// env sets under the posse home, and a seam that cannot find the home
// returns a permanently zero ExpiresAt for half the credentials it serves
// (ADR 0019 V5's round-trip, left open by ranger-base-h207 and closed by
// ranger-base-k6ha). The meter half needs no home and keeps its own
// home-free entry points below.
//
// A nil *App is a legal receiver here — the stamp lookup answers "cannot
// tell" without a home rather than panicking, because "no expiry known" is
// already a first-class answer and a seam is not the place to acquire a new
// way to crash.
func (a *App) ReadCredential(rt *Runtime, p CredPurpose) (string, CredMeta, error) {
	if rt == nil {
		return "", CredMeta{}, Die("credential read: no runtime named")
	}
	switch p {
	case CredSession:
		return a.readSessionCredential(rt)
	case CredMeter:
		return readMeterCredential(rt.Name)
	}
	return "", CredMeta{}, Die("unknown credential purpose %q", p)
}

// MeterToken adapts the meter half to the `func() (string, error)` the two
// HTTP readers hold in a field (AnthropicPlanReader.Token, ModelLister.Token
// — fields so tests can inject a fake). It takes the runtime's NAME because
// that is the whole of what a meter store is chosen by, and because both
// readers are constructed without an App and so have no *Runtime to hand.
//
// This is what replaced KeychainToken at both call sites.
func MeterToken(rt string) func() (string, error) {
	return func() (string, error) {
		tok, _, err := readMeterCredential(rt)
		return tok, err
	}
}

// MeterUnavailable answers a plan adapter's Unavailable question (ADR 0012
// D4) from the seam: can a meter credential be read on this machine at all?
// nil means there is a store to try — not that the read will succeed, which
// is the reader's business and blindness if it fails.
func MeterUnavailable(rt string) error {
	store, ns := meterStore(rt, runtime.GOOS)
	if ns != nil {
		return ns
	}
	if ns := store.absent(); ns != nil {
		return ns
	}
	return nil
}

// ─── the session credential: an operator's mint, in an env set ───────────────

// readSessionCredential wraps the env-set lookup that already exists
// (CageCredential, ADR 0002 §4 / rangerhq-kiz) — behaviour unchanged, one
// caller more. The value comes from THIS process's environment, which is
// where the launch realized the PID's `envs:`.
//
// The expiry comes from the `# expires=` stamp `posse refresh` wrote beside
// the variable in the env set the launch read it out of (ADR 0019 V5). No
// stamp is the zero time, which is "cannot tell" and warns nothing.
func (a *App) readSessionCredential(rt *Runtime) (string, CredMeta, error) {
	name := CageCredential(rt)
	if name == "" {
		return "", CredMeta{}, &NoSource{
			Runtime: rt.Name, Purpose: CredSession, GOOS: runtime.GOOS,
			Store: "the env set variable that would carry it is undecided",
			Arm:   "codex and grok keep plain auth.json files and rangerhq-kiz left their container shape open; decide it (cage_cred: names one for a template-only runtime)",
		}
	}
	v := os.Getenv(name)
	if v == "" {
		return "", CredMeta{}, Die("%s names this runtime's session credential and it is not in this process's environment — mint it once with `claude setup-token`, put it in an env set (mode 600, never in the repo), and name that set in the PID's envs:", name)
	}
	return v, CredMeta{Source: "env set variable " + name, ExpiresAt: a.sessionExpiry(name, v)}, nil
}

// sessionExpiry finds the stamp that is true of THIS value.
//
// A launched session has a value in its environment and no memory of which
// env set it came out of; several sets may carry the same variable, and
// their stamps are about their own values. So the match is on the VALUE:
// the stamp of a set that holds some other mint under the same name is a
// date about a credential this process is not holding, and reporting it
// would be worse than reporting nothing — "cannot tell" is already an
// honest answer here and a wrong date is not.
//
// The comparison is in memory and one-way: nothing is stored, logged or
// returned but a date. When two sets carry the identical value they are
// carrying one credential, so the soonest stamp any of them claims is the
// soonest posse may rely on, and that is what is returned.
func (a *App) sessionExpiry(key, value string) time.Time {
	if a == nil || a.EnvsDir == "" || value == "" {
		return time.Time{}
	}
	var soonest time.Time
	for _, n := range a.ListEnvSets() {
		b, err := os.ReadFile(filepath.Join(a.EnvsDir, n+".env"))
		if err != nil {
			continue
		}
		holds := false
		for _, v := range parseEnvLines(string(b)) {
			if v.Key == key && v.Value == value {
				holds = true
				break
			}
		}
		if !holds {
			continue
		}
		st, ok := readStamps(string(b), key)
		if !ok || st.Expires.IsZero() {
			continue
		}
		if soonest.IsZero() || st.Expires.Before(soonest) {
			soonest = st.Expires
		}
	}
	return soonest
}

// ─── the meter credential: the runtime's own store, per platform ─────────────

// runtimeStore is one platform's store of record for a runtime-owned
// credential: what to call it, whether it is structurally there, and how to
// get the envelope out of it.
type runtimeStore struct {
	// Name is the store as an operator would name it when going to look.
	Name string
	// Absent reports structural absence — nil when the store is there, and
	// nil when this platform cannot tell without performing the read, which
	// is darwin's case: the only way to ask the keychain whether an item
	// exists is to read it, and a read is what Read is. There, a missing
	// item is indistinguishable from an unreadable one and is reported as
	// the read failure it arrives as.
	Absent func() *NoSource
	// Read hands back the credential envelope, bytes unexamined.
	Read func() ([]byte, error)
	// Note is appended to every error this store produces. It carries the
	// non-darwin path's honest disclaimer (ADR 0019 V1) and is empty for a
	// path that has been run against a live login.
	Note string
	// Fix is the one-line move an operator makes when this store is there
	// and did not yield a credential — ADR 0019 D2's unreadable row, which
	// is the class that MUST NOT be reported as staleness. It differs by
	// store, which is why it lives here and not on the error type: the
	// keychain's cause is a per-binary ACL and a plain file's is not.
	Fix string
}

func (s runtimeStore) absent() *NoSource {
	if s.Absent == nil {
		return nil
	}
	return s.Absent()
}

// failRead is a failure of the READ itself — `security` exited non-zero, the
// file would not open — given this store's note, this store's class and this
// store's one-line fix, so the sentence an operator gets names the move and
// not only the symptom (ADR 0019 D2's unreadable row, bead rangerhq-pwpx).
//
// The note is attached without hiding the error: %w keeps errors.As working,
// which is what GateRefusal and RateLimit are read with.
//
// A GATE REFUSAL is the one thing that gets neither: the item was never
// reached, so it is not an outage of this store and must not wear this
// store's diagnosis (2026-08-24, GateRefusal's own header has the receipt).
func (s runtimeStore) failRead(err error) error {
	if err == nil {
		return nil
	}
	err = s.note(err)
	var g *GateRefusal
	if errors.As(err, &g) {
		return err
	}
	return &CredUnreadable{Store: s.Name, Fix: s.Fix, Err: err}
}

// failShape is a failure of the ENVELOPE — the store answered and what it
// answered is not a credential posse can find. Same class, and deliberately
// NO fix.
//
// ADR 0019 V7 is why: the shape diagnostics ranger-base-okbr bought with an
// hour of stopped shop are ONE piece of code for both platforms, and the
// store's name is the only thing that may differ between the two sentences
// (credseam_test.go pins that byte for byte). A per-store fix here would
// fork it — and would fork it into a wrong sentence, because a keychain that
// answered with a renamed key did not lose an ACL and re-granting one fixes
// nothing. The move for this failure is already the last clause of the
// diagnosis itself: teach credShapes the new name.
func (s runtimeStore) failShape(err error) error {
	if err == nil {
		return nil
	}
	return &CredUnreadable{Store: s.Name, Err: s.note(err)}
}

func (s runtimeStore) note(err error) error {
	if s.Note == "" {
		return err
	}
	return fmt.Errorf("%w%s", err, s.Note)
}

// CredUnreadable is a store of record that IS there and did not hand back a
// credential: `security` exited non-zero, the item is gone, the envelope is
// not JSON, or it holds no token in any shape posse knows.
//
// It is a distinct type for the reason AuthFailure and RateLimit are (ADR
// 0019 D2, bead rangerhq-pwpx). The three things it is NOT are each a
// different next move: it is not staleness (nothing an interactive refresh
// clears — saying "refresh" here sent the operator at the wrong half of the
// system on 2026-08-24), it is not structural absence (*NoSource, which is
// the guard OFF and has no clock), and it is not posse's own gate refusing
// the read (*GateRefusal, which is not a credential condition at all).
//
// Fix is the store's own one-line move and rides on the sentence, so the
// 80% of the runbook is in the error whether or not the runbook page is
// there (ADR 0019 D5).
type CredUnreadable struct {
	// Store is the store as an operator would name it when going to look.
	Store string
	// Fix is the one-line move. Empty is allowed and prints nothing extra:
	// a store with no known move says the symptom and stops.
	Fix string
	// Err is what went wrong. It never quotes the credential — that rule is
	// this file's and this type does not relax it.
	Err error
}

func (e *CredUnreadable) Error() string {
	if e.Fix == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%v — %s", e.Err, e.Fix)
}

func (e *CredUnreadable) Unwrap() error { return e.Err }

// CredUnreadableReason is AuthFailureReason's sibling for this class: the
// *CredUnreadable inside err, or nil.
func CredUnreadableReason(err error) *CredUnreadable {
	if err == nil {
		return nil
	}
	var cu *CredUnreadable
	if errors.As(err, &cu) {
		return cu
	}
	return nil
}

// meterUnconfirmed is ADR 0019 V1 stated where it bites: the non-darwin
// adapter is built from a MEASURED envelope shape but has not yet been run
// against a live Linux login, and until ranger-base's probe bead runs, an
// error from it says so rather than implying a confirmed path failed.
const meterUnconfirmed = " (posse's non-darwin credential path is built but not yet confirmed against a live login — ADR 0019 V1)"

// meterStore picks the store of record for rt's meter credential on goos.
// goos is a parameter and not `runtime.GOOS` so the switch is total in a
// test on either box: no build tags means no branch that only one OS can
// reach, and none that only one OS can prove.
func meterStore(rt, goos string) (runtimeStore, *NoSource) {
	if rt != "claude" {
		return runtimeStore{}, &NoSource{
			Runtime: rt, Purpose: CredMeter, GOOS: goos,
			Arm: "posse ships no usage-endpoint adapter for this runtime (ADR 0012 D4), so there is no credential for it to want; the plan guard runs off",
		}
	}
	if goos == "darwin" {
		return keychainStore(), nil
	}
	return credentialsFileStore(goos), nil
}

// readMeterCredential is the meter half of Read: pick the platform's store,
// then read it.
func readMeterCredential(rt string) (string, CredMeta, error) {
	store, ns := meterStore(rt, runtime.GOOS)
	if ns != nil {
		return "", CredMeta{}, ns
	}
	return readStore(store)
}

// readStore is that read with the store already chosen: structural absence
// answers as itself, then the envelope is fetched and parsed.
//
// It is separate from readMeterCredential so a test of ONE platform's
// adapter can run on either box — the keychain adapter's gate-refusal and
// wrong-shape tests stub a `security` on PATH and ask this, so `make
// test-linux` proves the darwin branch too and neither branch is a code path
// only one OS can reach (ADR 0019 D2).
func readStore(store runtimeStore) (string, CredMeta, error) {
	if ns := store.absent(); ns != nil {
		return "", CredMeta{}, ns
	}
	blob, err := store.Read()
	if err != nil {
		return "", CredMeta{}, store.failRead(err)
	}
	tok, meta, err := credentialToken(store.Name, blob)
	if err != nil {
		return "", CredMeta{}, store.failShape(err)
	}
	return tok, meta, nil
}

// KeychainService is the macOS keychain item Claude Code stores its OAuth
// credentials under.
const KeychainService = "Claude Code-credentials"

// securityBin is macOS's `security`, named ABSOLUTELY and not looked up on
// PATH (ranger-base-ypf5, part B of ranger-base-r64).
//
// Every persona launch prepends that persona's L1 shim dir to PATH
// (gates.go) and every crew PID denies Bash(security:*) — so while this
// resolved on PATH, a `posse` command typed inside a persona pane had its
// OWN monitoring read refused by its own gate: the plan guard went blind and
// the launch preflight went UNKNOWN. The deny aims at what a persona may
// run; posse is not the gated party. An absolute path walks past L1 by
// design (the documented class in the gates.go header: L1 matches the typed
// word), and the wall for a read is L4, which posse does not run inside.
//
// It is /usr/bin/security and nothing else: that is where the base system
// puts it, the path is SIP-protected, and a configurable one would be a
// place to point our credential read at an attacker's binary for no benefit
// anyone asked for.
const securityBin = "/usr/bin/security"

// keychainStore is the darwin adapter: the read that used to be
// KeychainToken/KeychainCredential, moved here as it stood. Errors never
// quote the command's output — that output is the credential blob.
func keychainStore() runtimeStore { return keychainStoreAt(securityBin) }

// keychainStoreAt is that adapter with the binary named explicitly. Only a
// test passes anything but securityBin: a stub cannot be planted at an
// absolute path, so the tests that pin the refusal parse and the envelope
// shapes name their stub here instead of putting one on a PATH this code no
// longer reads.
func keychainStoreAt(bin string) runtimeStore {
	return runtimeStore{
		Name: fmt.Sprintf("keychain item %q", KeychainService),
		// ADR 0019 D2's unreadable row, in the operator's own verbs. The
		// ACL is hypothetical on purpose — it is the cause that has
		// actually bitten (three times on 2026-08-24, every one of them a
		// `make install`) and the message says "may", because a keychain
		// that answered and held nothing usable is the same class and is
		// fixed by the second half of the same line.
		Fix: "this binary's keychain ACL may have been dropped by `make install`; grant access when prompted, or run `claude` once",
		Read: func() ([]byte, error) {
			out, err := keychainCmd(bin).Output()
			if err != nil {
				// GateRefusal stays after part B removed its cause: it is
				// what stops the 08-24 misdiagnosis returning if this ever
				// regresses to a PATH lookup, and it names the command by
				// the word a deny rule is spelled with.
				if g := gateRefusal(filepath.Base(bin), err); g != nil {
					return nil, g
				}
				return nil, Die("keychain item %q unreadable", KeychainService)
			}
			return out, nil
		},
	}
}

// keychainCmd is the one place the read's argv is built. It is its own
// function so a test can ask what binary the adapter RESOLVES TO without
// running it: exec.Command LookPaths a bare name and records the answer in
// .Path, so a regression to `security` shows up there as a shim's path
// rather than /usr/bin/security — and the real keychain is never read to
// find that out.
func keychainCmd(bin string) *exec.Cmd {
	return exec.Command(bin, "find-generic-password", "-s", KeychainService, "-w")
}

// CredentialsFile is where Claude Code keeps the same OAuth envelope on a
// platform with no keychain. It is NOT read on darwin: there the keychain is
// the store of record and the file is a recurring unowned byproduct — some
// darwin auth flow regenerates it on its own schedule, but MEASURED it then
// sits frozen for days while the keychain keeps rotating (ADR 0019 D2 store
// 3 / amended rejected-alternatives entry) — so reading it would invert the
// store of record on the one platform whose record lives elsewhere.
//
// The DIRECTORY is credentialDir's, not `$HOME/.claude`: this function
// assumed the home for as long as it existed, which made posse blind to a
// live credential file on any box whose operator sets either config-dir
// variable — a false NoSource ("log in once with `claude`") while claude is
// logged in and rotating a token one directory over (ranger-base-wd4be).
func CredentialsFile() (string, error) {
	dir, err := credentialDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".credentials.json"), nil
}

// credentialDir is the directory the runtime's secure storage writes into,
// MEASURED off the shipped 2.1.258 bundle rather than assumed. Verbatim, the
// darwin-arm64 binary at byte 158045445 (the linux-x64 one measured the same
// on ranger-base-ydjz; the two bundles DO differ elsewhere, so both were
// read):
//
//	function TS(){let n=process.env.CLAUDE_SECURESTORAGE_CONFIG_DIR
//	if(n!==void 0)return(n||l(o(),".claude")).normalize("NFC")
//	return Se()}
//
// So, in order:
//
//   - CLAUDE_SECURESTORAGE_CONFIG_DIR set and non-empty — that directory.
//   - CLAUDE_SECURESTORAGE_CONFIG_DIR set and EMPTY — `~/.claude`, and NOT
//     the configuration directory. `n!==void 0` is a presence test and
//     `n||…` is a truthiness test, so an empty value enters the branch and
//     then falls to the home: setting the variable to nothing SHADOWS
//     CLAUDE_CONFIG_DIR rather than deferring to it. That is the one arm of
//     this resolution nobody would guess, which is why it has its own pin.
//   - otherwise the configuration directory — ClaudeConfigDirIn, the same
//     rule trust.go follows for the trust file.
//
// The home is resolved once, only when it is needed: a set, non-empty
// CLAUDE_SECURESTORAGE_CONFIG_DIR is an answer on a box with no home at all,
// and reporting "no home directory" there would be a diagnosis of the wrong
// thing.
func credentialDir() (string, error) {
	sec, set := os.LookupEnv("CLAUDE_SECURESTORAGE_CONFIG_DIR")
	if set && sec != "" {
		return sec, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", Die("no home directory for this user, so the runtime's credentials file cannot be located")
	}
	if set {
		return filepath.Join(home, ".claude"), nil
	}
	return ClaudeConfigDirIn(home), nil
}

// credentialFileCandidates names every path this process's environment says
// the runtime's credentials file could be sitting at: `~/.claude`'s, and
// CredentialsFile's — the directory the runtime writes NEXT — when the two
// differ. Deduped, home first, so a caller rendering a wall from it gets a
// stable order.
//
// The home is in the list unconditionally, and it is not made redundant by
// the resolver. Two reasons, and each one alone is enough:
//
//   - a PRESENT-BUT-EMPTY CLAUDE_SECURESTORAGE_CONFIG_DIR resolves to the
//     home and shadows CLAUDE_CONFIG_DIR, so on that arm the home IS the
//     answer and no variable says so.
//   - whatever the runtime wrote in the home before a variable moved the
//     write is still sitting there. ADR 0019 D2 calls that file a recurring
//     unowned byproduct and ranger-base-xjj9 measured it regenerating 8h06m
//     after a delete: it does not leave because the write moved on.
//
// This is the shape scripts/verify-credential-paths.sh scans in, one dir
// short: the sweep also scans CLAUDE_CONFIG_DIR's when an empty
// CLAUDE_SECURESTORAGE_CONFIG_DIR shadows it, because a DETECTIVE control
// looks wherever a file could have been left. This list follows the
// resolver instead, because its consumer is a wall on where the runtime
// writes — and following the resolver is what keeps the wall and the reader
// from ever disagreeing about that (ranger-base-x5f6p).
//
// A resolver error is not an error here: it means no home AND no
// secure-storage override, so there is no path to name, and a caller
// building a deny list from an empty answer denies nothing rather than
// refusing a launch over a file that cannot exist.
func credentialFileCandidates() []string {
	var out []string
	add := func(p string) {
		if p == "" || strings.HasPrefix(p, "~") {
			return // no home to expand against; naming `~/…` to a sandbox is naming a file in the cwd
		}
		for _, q := range out {
			if q == p {
				return
			}
		}
		out = append(out, p)
	}
	add(ExpandTilde("~/.claude/.credentials.json"))
	if p, err := CredentialsFile(); err == nil {
		add(p)
	}
	return out
}

// credentialDirVars names the environment variables credentialDir's
// resolution above reads. It exists for the one caller that has to ask
// ABOUT the variables rather than read them: the launcher renders the
// seatbelt's credential read-deny from THIS process's environment, and
// overlays the session's env sets on the child afterwards, so a set
// exporting either name would move the caged runtime's write past a wall
// already written (ranger-base-x5f6p). Spelled a second time rather than
// threaded through credentialDir, whose literals are the transcription of a
// measured bundle and are worth reading as one — and pinned against the
// resolver's behavior, so the copy cannot drift into a list of names
// nothing honors.
var credentialDirVars = []string{"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR"}

// credentialDirVarsIn reports which of those an env set's variables carry,
// in credentialDirVars order. Names only — no value is read, copied or
// reported, which is what keeps this callable from a launch path that must
// never learn what an env set holds.
func credentialDirVarsIn(vars []EnvVar) []string {
	var out []string
	for _, name := range credentialDirVars {
		for _, v := range vars {
			if v.Key == name {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

// credentialsFileStore is the non-darwin adapter. A file that is not there
// is NoSource — the runtime has never logged in here, which is a structural
// condition an operator fixes with one command and no retry ever will. A
// file that is there and unreadable is an ordinary read failure, which is
// blindness, because something IS there.
func credentialsFileStore(goos string) runtimeStore {
	p, perr := CredentialsFile()
	name := "the Claude Code credentials file " + p
	if perr != nil {
		name = "the Claude Code credentials file under the runtime's config directory"
	}
	// Not the keychain's sentence: there is no per-binary ACL on a plain
	// file, and telling an operator to re-grant one would send them looking
	// for a thing this platform does not have.
	s := runtimeStore{Name: name, Note: meterUnconfirmed,
		Fix: "log in once with `claude` — its own login loop writes that file and posse reads it there"}
	s.Absent = func() *NoSource {
		if perr != nil {
			return nil // the read reports it; this is not absence, it is not knowing
		}
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			return &NoSource{
				Runtime: "claude", Purpose: CredMeter, GOOS: goos, Store: name,
				// The disclaimer rides on the Arm rather than through fail():
				// a NoSource is read by its TYPE, and wrapping it in an
				// fmt.Errorf would survive errors.As and not a type assert.
				Arm: "log in once with `claude` — its own login loop writes that file and posse reads it there; posse never writes a rotating credential (ADR 0019 D4)" + meterUnconfirmed,
			}
		}
		return nil
	}
	s.Read = func() ([]byte, error) {
		if perr != nil {
			return nil, perr
		}
		b, err := os.ReadFile(p)
		if err != nil {
			// The path is the diagnosis and the contents are the credential:
			// os.ReadFile's own message carries the first and not the second,
			// but it is not ours, so it is restated rather than wrapped.
			return nil, Die("%s is unreadable", name)
		}
		return b, nil
	}
	return s
}

// ─── the envelope, parsed one way for every platform ─────────────────────────

// GateRefusal is one of posse's OWN L1 gate shims refusing a command posse
// itself ran (ranger-base-r64). Every persona launch prepends that persona's
// shim dir to PATH (gates.go) and every crew PID denies Bash(security:*), so
// while the keychain read resolved on PATH, a `posse` command typed inside a
// persona pane got that persona's refusal shim and exit 1.
//
// The keychain read no longer resolves on PATH (ranger-base-ypf5), so this
// type is now a REGRESSION GUARD rather than a live diagnosis: it is what
// stops the misdiagnosis below returning if any read here ever goes back to
// a bare command name, and it stays the diagnosis for anything posse execs
// that still does.
//
// It is a distinct type because the two things it is NOT are both worse than
// it: it is not a credential outage (the item was never reached), and it is
// not an availability answer. Reporting it as "keychain item unreadable" is
// byte-identical to a real outage — on 2026-08-24 that reading is what got
// plan_guard_blind_max: 0 set for hours, switching off the shop's only
// automated brake on a diagnosis that was wrong.
//
// Cmd is the shimmed binary; Rule is the deny: line that refused it, "" when
// the shim's stderr did not name one.
type GateRefusal struct {
	Cmd  string
	Rule string
}

func (e *GateRefusal) Error() string {
	if e.Rule != "" {
		return fmt.Sprintf("keychain read refused by a posse gate shim: %s (deny: %s) — posse's own gate, not a credential outage", e.Cmd, e.Rule)
	}
	return fmt.Sprintf("keychain read refused by a posse gate shim: %s — posse's own gate, not a credential outage", e.Cmd)
}

// gateRefusal reads an exec failure as a shim refusal, or returns nil. The
// shim writes "refused by posse gate: <cmd> <argv> (deny: <rule>)" to stderr
// and exits 1, and .Output() hands that stderr back on *exec.ExitError.
//
// Only the command name and the rule are lifted out. The rest of the line is
// argv, and this file's standing rule is that nothing it returns quotes a
// command's own bytes.
func gateRefusal(cmd string, err error) *GateRefusal {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return nil
	}
	for _, line := range strings.Split(string(ee.Stderr), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "refused by posse gate: ")
		if !ok {
			continue
		}
		g := &GateRefusal{Cmd: cmd}
		// The deny group is the tail of the line, and the rule inside it
		// has parens of its own — Bash(security:*) — so the closing one is
		// the LAST character, not the first ")" after the marker.
		if i := strings.LastIndex(rest, "(deny: "); i >= 0 {
			g.Rule = strings.TrimSuffix(rest[i+len("(deny: "):], ")")
		}
		return g
	}
	return nil
}

// credShape is one credential layout posse knows how to read, named by the
// dotted path it takes. The list is ordered and the FIRST shape that yields
// a non-empty token wins, so adding a shape can never change which token an
// item that already works hands back.
type credShape struct {
	Name string
	// Token digs the token out of the decoded top level, or returns "".
	Token func(map[string]json.RawMessage) string
	// Expires digs the credential's own expiry out of the same envelope, or
	// returns the zero time for "cannot tell" — including for a value it
	// cannot make sense of. A guessed date is worse than no date: ADR 0019
	// D5 says an unknown expiry is reported as unknown, never as fresh.
	Expires func(map[string]json.RawMessage) time.Time
}

// credShapes is that order. One entry today — the OAuth envelope Claude
// Code has always written. When a login writes a different one, append it
// here and the failure below stops being reached; do not reorder.
//
// It stayed at one entry through the 2026-08-26 outage (ranger-base-okbr)
// BECAUSE the shape was measured rather than guessed. posse's own line, once
// the naming fix below was promoted, reported claudeAiOauth's keys as
// [accessToken expiresAt rateLimitTier refreshToken refreshTokenExpiresAt
// scopes subscriptionType] — accessToken present, so nothing had been
// renamed and there was nothing here to teach. The token was empty: an
// incomplete credential, fixed by re-authenticating, and the shop came back
// with no change to this file. Guessing a shape and appending it would have
// been a second wrong diagnosis on top of the first.
var credShapes = []credShape{
	{
		Name: "claudeAiOauth.accessToken",
		Token: func(top map[string]json.RawMessage) string {
			var env struct {
				AccessToken string `json:"accessToken"`
			}
			if err := json.Unmarshal(top["claudeAiOauth"], &env); err != nil {
				return ""
			}
			return env.AccessToken
		},
		Expires: func(top map[string]json.RawMessage) time.Time {
			var env struct {
				ExpiresAt int64 `json:"expiresAt"`
			}
			if err := json.Unmarshal(top["claudeAiOauth"], &env); err != nil {
				return time.Time{}
			}
			return unixMillis(env.ExpiresAt)
		},
	},
}

// unixMillis reads the envelope's expiresAt, which is ASSUMED to be
// milliseconds since the epoch — the unit the known envelope carries, and
// the one this field has to be for the value to be a date at all.
//
// The assumption is bounded rather than trusted: a number that does not land
// between 2000 and 2100 read as milliseconds is not a date posse understands,
// and "cannot tell" is what it becomes. That is what keeps a unit that turns
// out to be seconds from being rendered as 1970 and warned about forever.
func unixMillis(n int64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	t := time.UnixMilli(n)
	if t.Year() < 2000 || t.Year() > 2100 {
		return time.Time{}
	}
	return t.UTC()
}

// credentialToken reads a store's blob as one of the declared shapes. store
// is the store's own name, and the ONLY thing that differs between the two
// platforms' diagnoses — the reading, the verdict and the fix are one piece
// of code for both (ADR 0019 V7). The word "keychain" reaches an error here
// only because a darwin store is called one (ADR 0019 V2).
//
// The failure here is the fourth credential-failure class (ranger-base-okbr):
// the store is present, readable, and valid JSON of a shape we do not know.
// "has no claudeAiOauth.accessToken" was true and useless — it did not say
// what the store DOES hold, which cost an hour of outage. So this names the
// key NAMES it actually found. The values are the credential and never
// appear; the names are schema, and safeKeys covers the one case where a
// store of the wrong shape is keyed BY something that is not a name.
func credentialToken(store string, out []byte) (string, CredMeta, error) {
	var top map[string]json.RawMessage
	// nil map, no error: that is `null`, which decodes into a map and is
	// not an object. Anything that is not an object is the same diagnosis.
	if err := json.Unmarshal(out, &top); err != nil || top == nil {
		return "", CredMeta{}, Die("%s is not the expected JSON (%s, want a JSON object)", store, jsonKind(out))
	}
	for _, s := range credShapes {
		if t := s.Token(top); t != "" {
			return t, CredMeta{Source: store, Shape: s.Name, ExpiresAt: s.Expires(top)}, nil
		}
	}
	return "", CredMeta{}, Die("%s holds no token in any shape posse knows (tried %s) — %s",
		store, credShapeNames(), foundShape(top))
}

func credShapeNames() string {
	names := make([]string, 0, len(credShapes))
	for _, s := range credShapes {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}

// foundShape describes what the store DOES contain, in key names only: the
// top level, and — when the envelope we look for is there but empty — that
// envelope too, since "no claudeAiOauth at all" and "claudeAiOauth without
// the token" are different diagnoses with different fixes.
//
// It also says WHICH of those two it is rather than leaving the reader to
// diff a key list by eye. On 2026-08-26 the operator's reading came back
// with claudeAiOauth present (ranger-base-8i7l), which narrowed the defect
// to exactly this fork: the field is there and empty (the credential is
// incomplete — a login problem, nothing to change here), or the field is
// gone (renamed — a line in credShapes). Naming the fork is the difference
// between a line an operator can act on and one they have to interpret.
func foundShape(top map[string]json.RawMessage) string {
	desc := "its top-level keys are " + safeKeys(top)
	raw, ok := top["claudeAiOauth"]
	if !ok {
		return desc + "; posse's envelope (claudeAiOauth) is not among them, so this store holds some other credential structure"
	}
	var inner map[string]json.RawMessage
	// nil map, no error: claudeAiOauth is `null`, which is not an object.
	if err := json.Unmarshal(raw, &inner); err != nil || inner == nil {
		return desc + ", and claudeAiOauth is " + jsonKind(raw) + ", not an object"
	}
	return desc + ", and claudeAiOauth's keys are " + safeKeys(inner) + " — " + tokenVerdict(raw, inner)
}

// tokenVerdict states which side of the fork we are on once posse's own
// envelope has been found. There are three sides, not two: the field is gone
// (renamed — a line in credShapes), the field is there and empty (an
// incomplete credential — a login fixes it), or the field is there and is not
// a string at all (the envelope restructured it — credShapes again, and no
// number of logins can help). The shape's Token func returns "" for the last
// two alike, so a verdict that reads only key presence calls a restructured
// field an empty one and sends the operator to re-authenticate forever about
// a change only this file can absorb (ranger-base-6ai5).
//
// It asks the PARSER for the field rather than looking the name up in the key
// map, because those two do not agree: encoding/json matches field names
// case-insensitively, so posse reads `AccessToken` fine while an exact map
// lookup calls it renamed and sends the operator to teach credShapes a name
// it already knows. Unmarshalling into a RawMessage yields the value the
// shape's Token func actually saw, under the same rules; decoding THAT into a
// string takes the same fork Token took, and jsonKind names what it found
// without printing a byte of it.
func tokenVerdict(raw json.RawMessage, inner map[string]json.RawMessage) string {
	var env struct {
		AccessToken  json.RawMessage `json:"accessToken"`
		RefreshToken json.RawMessage `json:"refreshToken"`
	}
	// raw decoded as an object at the call site, so this cannot fail.
	if err := json.Unmarshal(raw, &env); err != nil || env.AccessToken == nil {
		return "accessToken is not among them, so the field was renamed or dropped: teach credShapes the new name"
	}
	// The key list above is spelled the way the store spells it, which need
	// not be the way posse spells it. Say so, or the verdict looks like it is
	// about a key the reader cannot find in the list.
	spelling := ""
	if _, exact := inner["accessToken"]; !exact {
		spelling = " (posse matches field names case-insensitively, so the differently-cased key above is that field)"
	}
	// The same decode the shape's Token func does, into the same type — so
	// the fork here cannot disagree with the read that got us here. It is
	// also why null lands on the empty side: unmarshalling null into a string
	// is a no-op, so Token saw "" and no error, and the credential really is
	// incomplete. Everything else is a value Token could not read at all.
	var tok string
	if err := json.Unmarshal(env.AccessToken, &tok); err != nil {
		return "accessToken is present and is " + jsonKind(env.AccessToken) +
			", not a string, so the shape changed and re-authenticating cannot fix it: teach credShapes to read it" + spelling
	}
	// A string reaching here is necessarily the empty one — a non-empty one
	// would have been returned as the token and this line never reached.
	v := "accessToken is present but empty, so this is an incomplete credential and not a shape posse cannot read: re-authenticate rather than change posse"
	if env.RefreshToken != nil {
		v += " (a refreshToken is present, so a refresh that did not complete fits)"
	}
	return v + spelling
}

// maxKeyName is the longest key this file will repeat back. Well above every
// schema name a credential envelope uses and well under any token, because a
// long key means the object is keyed by a value and this file does not print
// values.
//
// The measured margin is smaller than it looks. The bound was 24 when the
// longest name we had seen was `subscriptionType` (16); the 2026-08-26
// reading brought `refreshTokenExpiresAt` (21), a key we had never seen, and
// this envelope adds keys under us. A name we elide is a name the operator
// needed, so the headroom is worth more than the tightness — and 32 is still
// far under any credential this file could be handed (`sk-ant-oat01-…` runs
// past 100 bytes).
const maxKeyName = 32

// maxKeysShown bounds the line — a store with hundreds of keys is not a
// credential and the count says so better than the list would.
const maxKeysShown = 12

// safeKeys renders an object's key names, sorted, for a diagnostic. A name
// that is not name-shaped — too long, or not printable ASCII without spaces
// — is reported by its size instead of its bytes, because the one way key
// names could carry a secret is an object keyed BY one.
func safeKeys(obj map[string]json.RawMessage) string {
	if len(obj) == 0 {
		return "[] (an empty object)"
	}
	names := make([]string, 0, len(obj))
	for k := range obj {
		names = append(names, k)
	}
	sort.Strings(names)
	shown := names
	extra := 0
	if len(shown) > maxKeysShown {
		shown, extra = shown[:maxKeysShown], len(names)-maxKeysShown
	}
	for i, k := range shown {
		if !nameShaped(k) {
			shown[i] = fmt.Sprintf("<%d bytes, not a name>", len(k))
		}
	}
	s := "[" + strings.Join(shown, " ") + "]"
	if extra > 0 {
		s += fmt.Sprintf(" (+%d more)", extra)
	}
	return s
}

func nameShaped(k string) bool {
	if k == "" || len(k) > maxKeyName {
		return false
	}
	for _, r := range k {
		if r <= ' ' || r > '~' {
			return false
		}
	}
	return true
}

// jsonKind names what a blob decodes to, and nothing about what is in it.
func jsonKind(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return "not JSON"
	}
	switch v.(type) {
	case map[string]any:
		return "a JSON object"
	case []any:
		return "a JSON array"
	case string:
		return "a JSON string"
	case float64:
		return "a JSON number"
	case bool:
		return "a JSON boolean"
	default:
		return "JSON null"
	}
}
