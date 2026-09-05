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
//   - darwin: the runtime's own composite store, mirrored (ADR 0019 D2 as
//     amended 2026-09-01) — the keychain item, read by execing
//     /usr/bin/security ABSOLUTELY (ranger-base-ypf5; a PATH lookup here
//     resolved to the calling persona's own Bash(security:*) shim and
//     refused posse's own monitoring read), and the credentials file below
//     it on exit 44 and on nothing else.
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
	"crypto/sha256"
	"encoding/hex"
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
// It hangs off *App because the session half IS the home: its store is the
// env sets under it and so is the expiry stamp beside each value (ADR 0019
// V5's round-trip, left open by ranger-base-h207 and closed by
// ranger-base-k6ha; the store itself became the home's when ADR 0039 D3d was
// amended). The meter half needs no home and keeps its own home-free entry
// points below.
//
// A nil *App is a legal receiver here — a seam is not the place to acquire a
// new way to crash. What it answers is the refusal, not a value: a caller
// with no home has no session store to open, and saying so is the honest
// form of "cannot tell" now that the store is a directory rather than an
// environment that every process has.
func (a *App) ReadCredential(rt *Runtime, p CredPurpose) (string, CredMeta, error) {
	if rt == nil {
		return "", CredMeta{}, Die("credential read: no runtime named")
	}
	switch p {
	case CredSession:
		// The persona-less list — the cockpit's (`posse runtimes`, `posse
		// gates`) and every caller that is not a launch. A caller that IS
		// one names its sets: ReadSessionCredentialFrom.
		return a.readSessionCredential(rt, a.LaunchEnvSets(nil, nil))
	case CredMeter:
		return readMeterCredential(rt.Name)
	}
	return "", CredMeta{}, Die("unknown credential purpose %q", p)
}

// MeterToken adapts the meter half to the `func() (string, CredMeta, error)`
// the two HTTP readers hold in a field (AnthropicPlanReader.Token,
// ModelLister.Token — fields so tests can inject a fake). It takes the
// runtime's NAME because that is the whole of what a meter store is chosen
// by, and because both readers are constructed without an App and so have no
// *Runtime to hand.
//
// This is what replaced KeychainToken at both call sites.
//
// The META rides along because on darwin the store that answered is no
// longer a constant (ADR 0019 D2 as amended): a composite read falls through
// to the credentials file, and a 401 on a token from there means something
// different from a 401 on a token from the keychain — so the one surface
// that renders the failure has to know which store it presented (V9). It is
// one return of one read rather than a second lookup on purpose: two reads
// of a rotating credential is how the sentence and the token come to
// disagree, which is the whole of ADR 0019's Context.
func MeterToken(rt string) func() (string, CredMeta, error) {
	return func() (string, CredMeta, error) {
		return readMeterCredential(rt)
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

// ReadSessionCredentialFrom is the seam's session half for a caller that
// knows which env sets its launch realizes — `TierPreflight` handing the
// catalog probe the mint THIS launch is about to export (ADR 0039 D3d as
// amended, ranger-base-q3n4e). `ReadCredential(rt, CredSession)` is the same
// read with the persona-less list, and both land on one reader below.
//
// sets are set NAMES in launch order, as LaunchEnvSets computes them.
func (a *App) ReadSessionCredentialFrom(rt *Runtime, sets []string) (string, CredMeta, error) {
	if rt == nil {
		return "", CredMeta{}, Die("credential read: no runtime named")
	}
	return a.readSessionCredential(rt, sets)
}

// readSessionCredential reads the operator's mint out of the env set FILES
// under the home — the store of record ADR 0019 D1 already names — and
// never out of this process's environment.
//
// The environment arm was retracted 2026-09-05 (ADR 0039 D3d as amended, on
// ranger-base-q3n4e), and retracted rather than kept beside the file. It
// read true and answered nothing: the only process that ever holds this
// value in its environment is a launched RUNTIME, which scrubs it from its
// children, and every posse surface that asks the question — `sessionRows`,
// `ExpiringCredentials`, `sessionExpiry`, and now the catalog probe — is a
// posse process. MEASURED on this instance: the mint sits in two env sets
// under the home, a dispatched session carries sibling variables of the same
// set, and the mint itself is absent from `os.Environ` entirely. An arm no
// caller can satisfy is not a second store.
//
// WHICH set: the caller's, in launch order, and the value is the LAST
// assignment of the name across that list. That is the rule `readStamps`
// already ascribes to a launch WITHIN one file ("the last one is the value a
// launch ends up exporting"), extended across the files one launch reads in
// order — so the seam answers with the value the launch's own `vars` loop
// would end up exporting, which is the whole point of preferring it.
//
// The expiry comes from the `# expires=` stamp `posse refresh` wrote beside
// the variable (ADR 0019 V5), matched on the VALUE by sessionExpiry — the
// same source as before, unchanged. No stamp is the zero time, which is
// "cannot tell" and warns nothing.
func (a *App) readSessionCredential(rt *Runtime, sets []string) (string, CredMeta, error) {
	name := CageCredential(rt)
	if name == "" {
		return "", CredMeta{}, &NoSource{
			Runtime: rt.Name, Purpose: CredSession, GOOS: runtime.GOOS,
			Store: "the env set variable that would carry it is undecided",
			Arm:   "codex and grok keep plain auth.json files and rangerhq-kiz left their container shape open; decide it (cage_cred: names one for a template-only runtime)",
		}
	}
	v, set := a.lastEnvAssignment(name, sets)
	if v == "" {
		return "", CredMeta{}, Die("%s names this runtime's session credential and it is in %s — mint it once with `claude setup-token`, put it in an env set (mode 600, never in the repo), and name that set in the PID's envs:", name, noSetPhrase(sets))
	}
	return v, CredMeta{Source: "env set " + set + " variable " + name, ExpiresAt: a.sessionExpiry(name, v)}, nil
}

// noSetPhrase names the sets that were looked in, because "it is not there"
// is only actionable if the operator can tell which files posse opened —
// the same launch list, in the same order. Names only: an env set name is
// prose the operator wrote, and no value goes near this sentence.
func noSetPhrase(sets []string) string {
	if len(sets) == 0 {
		return "no env set (this read names none)"
	}
	return "none of the env sets this read names (" + strings.Join(sets, ", ") + ")"
}

// lastEnvAssignment is the one read underneath both entry points: walk the
// launch's sets in order, keep the last assignment of key seen, and report
// which set it came out of. One loop covers both halves of the rule — later
// assignment within a file, later set across the list — because
// parseEnvLines keeps a file's assignments in file order.
//
// A set that is missing, unreadable, or names a path rather than a file stem
// is SKIPPED, not fatal: the launch itself refuses those (EnvSetVars returns
// the error and planLaunch stops), and a probe is not the surface that
// should be the first to say so. The read goes through envFilePath for the
// containment guard it carries — a name is a file stem, and where it
// resolves must be under EnvsDir (ADR 0019 D1's one-hand rule).
//
// It does NOT go through EnvSetVars: that tightens the store's modes and
// writes a line to stderr for each path it fixes, which is right on a launch
// and wrong on a read that happens behind a catalog probe.
//
// A nil *App, or one with no home, has nothing to open and says so by
// finding nothing — the seam's degenerate caller answers, it does not panic.
func (a *App) lastEnvAssignment(key string, sets []string) (value, set string) {
	if a == nil || a.EnvsDir == "" {
		return "", ""
	}
	for _, n := range sets {
		f, err := a.envFilePath(n)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, v := range parseEnvLines(string(b)) {
			if v.Key == key {
				value, set = v.Value, n
			}
		}
	}
	return value, set
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
	// Fallthrough is the composite's SECOND read, and darwin's alone (ADR
	// 0019 D2 as amended 2026-09-01, ranger-base-v3qi4). Claude Code's
	// darwin secure storage is one store whose own name is
	// `keychain-with-plaintext-fallback`: the keychain item, then the
	// credentials file when the item did not answer.
	//
	// Given the primary's read error it answers the WHOLE call — a
	// credential out of the second store under the second store's own name,
	// so a 401 on it names where it came from, or the primary's own failure
	// unchanged for every error that does not fall through.
	//
	// It is a hook and not a second store field because the fall-through is
	// a rule about WHICH failures rather than a chain: exit 36 and a gate
	// refusal must never reach the file, and a chain has no place to say so.
	Fallthrough func(err error) (string, CredMeta, error)
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
		// The composite's second store, when this platform has one and this
		// failure is the one that falls through to it. Everything else —
		// and every store with no fallthrough at all — is the read failure
		// it arrived as.
		if store.Fallthrough != nil {
			return store.Fallthrough(err)
		}
		return "", CredMeta{}, store.failRead(err)
	}
	tok, meta, err := credentialToken(store.Name, blob)
	if err != nil {
		return "", CredMeta{}, store.failShape(err)
	}
	return tok, meta, nil
}

// KeychainService is the DEFAULT spelling of the macOS keychain item Claude
// Code stores its OAuth credentials under — the whole name exactly when the
// environment names no configuration directory, and the head of it when one
// does. The item posse actually reads is keychainItem's answer, and every
// sentence prints that (ADR 0019 D2 store 1, ranger-base-ig4op): the
// constant survives as the spelling, not as the read.
const KeychainService = "Claude Code-credentials"

// keychainItem is the name posse asks the keychain for, and the one clause a
// derived name may have to carry with it.
//
// MEASURED 2026-09-02 off the same darwin-arm64 2.1.258 bundle credentialDir
// was transcribed from, one statement after it (ADR 0019 D2 store 1,
// ranger-base-ig4op): the item is `Claude Code-credentials` exactly when the
// environment names no directory, and otherwise that name, `-`, and the
// first 8 hex digits of sha256 over the directory STRING. The runtime hashes
// with node's sha256 over the string's UTF-8 bytes, which is this.
//
// Three rules ride here, and each one is a way this can be wrong:
//
//   - Default-ness is a property of the ENVIRONMENT, not of the path.
//     `CLAUDE_CONFIG_DIR=$HOME/.claude` names the default directory and
//     STILL suffixes the item, because the runtime tests the variable and
//     never the path. So the answer is credentialDirNamed's bool, and never
//     a comparison of dir against the home — that comparison is the mutant
//     V11 exists to kill.
//   - The hash is over the string as the variable SPELLS it: a trailing
//     slash hashes as typed, and it is the DIRECTORY, never the file path
//     CredentialsFile builds out of it (a different string, and a name no
//     keychain ever held).
//   - posse does not normalize. The runtime NFC-normalizes the value; Go's
//     standard library has no NFC and ADR 0019 prices x/text and declines
//     it. For an ASCII value NFC is the identity (MEASURED), so the derived
//     name is exact wherever the operator typed an ASCII path. For anything
//     else posse hashes the bytes as spelled and the note says so, so an
//     item that is not found there names its first suspect instead of
//     reading as an empty keychain.
//
// A resolver error is the constant: a box with no home and no secure-storage
// override has no directory string to hash, and the default spelling is the
// honest answer. That arm also swallows a CLAUDE_CONFIG_DIR set on a
// homeless box — credentialDir already errors rather than answering there,
// and this follows the resolver rather than growing a second one.
func keychainItem() (name, note string) {
	dir, named, err := credentialDirNamed()
	if err != nil || !named {
		return KeychainService, ""
	}
	sum := sha256.Sum256([]byte(dir))
	name = KeychainService + "-" + hex.EncodeToString(sum[:])[:8]
	for i := 0; i < len(dir); i++ {
		if dir[i] >= 0x80 {
			return name, " (non-ASCII directory: posse hashed it as spelled and the runtime hashes its NFC form, so an item not found here may be that difference rather than an empty keychain)"
		}
	}
	return name, ""
}

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

// keychainACLFix is ADR 0019 D2's unreadable row in the operator's own
// verbs, for every failure of the keychain read that is not "no such item".
// The ACL is hypothetical on purpose — it is the cause that has actually
// bitten (three times on 2026-08-24, every one of them a `make install`) and
// the message says "may", because a keychain that answered and held nothing
// usable is the same class and is fixed by the second half of the same line.
const keychainACLFix = "this binary's keychain ACL may have been dropped by `make install`; grant access when prompted, or run `claude` once"

// keychainFallbackFix is that move with the second cause the composite made
// visible (ADR 0019 D2 as amended, V9). `security` exiting 44 is two
// different facts wearing one exit code, and they are repaired at opposite
// ends: the item really is gone and the runtime is living on its fallback
// credentials file, or the item is there and THIS binary may no longer read
// it. An operator told only the second goes and re-grants an ACL on an empty
// keychain; told only the first, they `/login` a keychain that was never the
// problem.
//
// ASSUMED, and operator-measurable only because every crew PID denies
// `security` (ADR 0019 V10): a dropped posse ACL is believed to answer 36,
// not 44, in which case this sentence is reached only by a genuinely empty
// keychain and its first cause is the true one. It names both anyway — the
// cost of the extra clause is a longer line, and the cost of guessing wrong
// is the operator repairing the wrong end of the composite.
const keychainFallbackFix = "the item did not answer this binary, which is two different things: it really is gone and claude is running on its fallback credentials file — repair the keychain (unlock it, grant access), then `/login` in claude — or this binary's keychain ACL may have been dropped by `make install`; grant access when prompted, or run `claude` once"

// errSecItemNotFound is `security`'s exit for "no such item in this
// keychain" — the ONE exit the composite falls through to the file on (ADR
// 0019 D2's narrowing).
//
// 36 (user interaction not allowed) is deliberately NOT here, though the
// runtime's own composite treats it as null too: the keychain ACL is per
// binary, so posse's 36 speaks about posse's binary and not about the
// keychain's contents. Mirroring the runtime's rule literally would read a
// frozen S2 file after every `make install` and re-create the 2026-08-24
// misdiagnosis with a new sentence.
//
// The runtime's third null — exit 0 with no output — is not here either, and
// for a different reason: it is not a failure at all, so it never reaches
// this question. `security … -w` on an item it found prints the password, so
// an empty answer is an item holding an empty credential, which is the okbr
// diagnosis (an incomplete credential, fixed by re-authenticating) and not a
// keychain with nothing in it. Falling through would answer a login problem
// with another store's token.
const errSecItemNotFound = 44

// keychainExit is a non-zero `security` exit carried past the sentence. The
// sentence is byte-for-byte what it was — the operator gets no number they
// cannot act on — and the composite gets to ask WHICH failure this was
// without running the read a second time, which is how the read and the
// decision about the read stay one read.
type keychainExit struct {
	item string
	code int
}

func (e *keychainExit) Error() string { return fmt.Sprintf("keychain item %q unreadable", e.item) }

// keychainItemNotFound reports the one exit that falls through.
func keychainItemNotFound(err error) bool {
	var ke *keychainExit
	return errors.As(err, &ke) && ke.code == errSecItemNotFound
}

// execExitCode is a failed command's exit status, or -1 for a failure that
// is not one. A `security` that could not be executed at all did not answer
// 44 and must not be read as though it had.
func execExitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// keychainStore is the darwin adapter: the runtime's own composite store,
// mirrored (ADR 0019 D2 as amended 2026-09-01, ranger-base-v3qi4). Errors
// never quote the command's output — that output is the credential blob.
func keychainStore() runtimeStore { return keychainStoreAt(securityBin) }

// keychainStoreAt is that adapter with the binary named explicitly. Only a
// test passes anything but securityBin: a stub cannot be planted at an
// absolute path, so the tests that pin the refusal parse and the envelope
// shapes name their stub here instead of putting one on a PATH this code no
// longer reads.
func keychainStoreAt(bin string) runtimeStore {
	// The name is derived once, here, and every sentence this store produces
	// prints THAT — the store's own name, the seam's Source (credentialToken
	// is handed this Name), the unreadable sentence and the refresh report's
	// row. An operator with a suffixed item sees the suffix to match in
	// Keychain Access, and a second derivation is how the read and the
	// sentence about it would come to disagree (ADR 0019 D2 store 1).
	item, note := keychainItem()
	s := runtimeStore{
		Name: fmt.Sprintf("keychain item %q", item) + note,
		Fix:  keychainACLFix,
		Read: func() ([]byte, error) {
			out, err := keychainCmd(bin, item).Output()
			if err != nil {
				// GateRefusal stays after part B removed its cause: it is
				// what stops the 08-24 misdiagnosis returning if this ever
				// regresses to a PATH lookup, and it names the command by
				// the word a deny rule is spelled with. It is asked FIRST
				// and it never falls through: the item was not reached, so
				// nothing about the keychain's contents was learned.
				if g := gateRefusal(filepath.Base(bin), err); g != nil {
					return nil, g
				}
				return nil, &keychainExit{item: item, code: execExitCode(err)}
			}
			return out, nil
		},
	}
	// The composite's read order, with D2's one narrowing: the file, only on
	// 44, and only when it is there. `s` is captured rather than passed in
	// so the primary this speaks for cannot be a different value from the
	// one that failed.
	s.Fallthrough = func(err error) (string, CredMeta, error) {
		if !keychainItemNotFound(err) {
			return "", CredMeta{}, s.failRead(err)
		}
		p, ok := keychainFallbackFile()
		if !ok {
			// 44 with no file stays CredUnreadable and stays blind, with
			// ADR 0018's clock (D2, "Not changed"): the launcher box is a
			// logged-in box by construction, so a vanished item there is an
			// incident and not an unconfigured platform, and *NoSource here
			// would be a guard that switches itself off. What it gains is
			// the second cause, which is the whole of V9's first sentence.
			blind := s
			blind.Fix = keychainFallbackFix
			return "", CredMeta{}, blind.failRead(err)
		}
		return readStore(keychainFallbackStore(p))
	}
	return s
}

// credentialsFileFallback is what the composite's SECOND store is called,
// and so what the seam's Source says when a read fell through to it — ADR
// 0019 D2 as amended, verbatim.
//
// It is a name of its own rather than the non-darwin store's, and the
// difference is load-bearing: the same file off darwin is the store of
// record and its sentences say so, while here it is the store the runtime
// fell back to, which is a fact about the keychain. One name for both would
// make a 401 unreadable at exactly the moment it matters.
const credentialsFileFallback = "the Claude Code credentials file (keychain fallback)"

// keychainFallbackFile is the composite's second store as a path, plus
// whether it is sitting there now.
//
// The existence question is the COMPOSITE's, and deliberately not the file
// store's Absent, because absence here is not structural absence. Off darwin
// a missing credentials file means the runtime has never logged in on this
// box — *NoSource, the guard off, no clock. On darwin the keychain is the
// store of record, so a keychain answering 44 with no file beside it is an
// incident on a box that is logged in by construction: blind, and ADR 0018's
// clock runs.
//
// A resolver error is "not there": no home and no secure-storage override is
// no path to open, and the keychain's own failure is the honest diagnosis.
func keychainFallbackFile() (string, bool) {
	p, err := CredentialsFile()
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// keychainFallbackStore is that file as a store: the SAME parser, the same
// diagnostics, one name apart (ADR 0019 V7).
func keychainFallbackStore(p string) runtimeStore {
	return runtimeStore{
		Name: credentialsFileFallback,
		// Absent stays nil. The composite asked that question already, and a
		// file that vanishes between the two is a read failure — blind —
		// never *NoSource.
		//
		// Note stays empty too: meterUnconfirmed is V1's disclaimer about
		// the NON-darwin adapter, which has never been run against a live
		// login. This path's premise — that the runtime writes and reads
		// this file on darwin — is MEASURED off the shipped bundle and off
		// this box (ranger-base-xjj9/1lza), so borrowing that sentence here
		// would disclaim something that was measured.
		Fix:  keychainFallbackFix,
		Read: credentialsFileRead(p, credentialsFileFallback),
	}
}

// keychainCmd is the one place the read's argv is built. It is its own
// function so a test can ask what binary the adapter RESOLVES TO without
// running it: exec.Command LookPaths a bare name and records the answer in
// .Path, so a regression to `security` shows up there as a shim's path
// rather than /usr/bin/security — and the real keychain is never read to
// find that out.
//
// The item is an ARGUMENT and not the constant: the name the environment
// derives is the name the read must ask for, and this is the one argv that
// carries it (ADR 0019 V12).
func keychainCmd(bin, item string) *exec.Cmd {
	return exec.Command(bin, "find-generic-password", "-s", item, "-w")
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
//
// The resolution itself is credentialDirNamed's, which answers the same
// directory plus whether a variable named it — the bit the keychain item's
// name needs. This signature is unchanged because seatbelt.go's wall calls
// it (ranger-base-7pf1h).
func credentialDir() (string, error) {
	dir, _, err := credentialDirNamed()
	return dir, err
}

// credentialDirNamed is that same resolution with the one further bit the
// item's NAME needs: whether a VARIABLE named the directory, which is what
// decides the keychain item's spelling one statement later in the runtime's
// own module (ADR 0019 D2 store 1, ranger-base-ig4op).
//
// It is one function and not two derivations on purpose. The file path and
// the item name are the same resolution read twice by the runtime, and a
// second copy here is how the wall, the reader and the sentence come to
// disagree about which directory is in play — the class ranger-base-x5f6p
// already paid for once.
//
// Named is an ENVIRONMENT property, never a path property:
//
//   - CLAUDE_SECURESTORAGE_CONFIG_DIR set and non-empty — named, and the
//     directory is that value verbatim.
//   - CLAUDE_SECURESTORAGE_CONFIG_DIR set and EMPTY — NOT named. It shadows
//     CLAUDE_CONFIG_DIR for the name exactly as it does for the file, so an
//     empty value beside a set config dir is the default item.
//   - otherwise CLAUDE_CONFIG_DIR non-empty — named. The test is on the
//     VARIABLE and not on the directory it yields: `CLAUDE_CONFIG_DIR` set
//     to the home's own `.claude` names the default directory and still
//     suffixes the item, because the runtime never looks at the path.
//   - otherwise — not named, and the directory is the home's `.claude`.
//
// credentialDir keeps its signature (seatbelt.go calls it, ranger-base-7pf1h)
// and derives from this, so there is exactly one resolver.
func credentialDirNamed() (dir string, named bool, err error) {
	sec, set := os.LookupEnv("CLAUDE_SECURESTORAGE_CONFIG_DIR")
	if set && sec != "" {
		return sec, true, nil
	}
	home, herr := os.UserHomeDir()
	if herr != nil || home == "" {
		return "", false, Die("no home directory for this user, so the runtime's credentials file cannot be located")
	}
	if set {
		return filepath.Join(home, ".claude"), false, nil
	}
	// The value comes from ClaudeConfigDirIn — one spelling of the config-dir
	// rule, trust.go's — and only the named-ness reads the variable, because
	// there is nothing in the directory it returns that says whether an
	// operator typed it.
	return ClaudeConfigDirIn(home), os.Getenv("CLAUDE_CONFIG_DIR") != "", nil
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
		if !credentialDenyable(p) {
			return
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

// credentialDirEnvSetRefusal is the other half of the seatbelt's credential
// read-deny (ADR 0019 D2 item 3, ranger-base-x5f6p): the refusal a launch
// owes once its env sets are resolved and its profile is already written.
//
// That deny names the file the runtime's own resolution points at —
// CLAUDE_SECURESTORAGE_CONFIG_DIR, then CLAUDE_CONFIG_DIR, then `~/.claude`
// (credentialDir) — read out of the LAUNCHER's environment, because the
// profile is rendered in the launching process. Env sets are not in that
// environment: they are overlaid on the child long after the profile is
// written. So a set exporting either name moves the runtime's credential
// write to a directory the wall never heard of, and every other layer
// reports a healthy launch.
//
// The session does NOT inherit the launching process's environment, and an
// earlier draft of this comment said it did (ranger-base-r68d8). The session
// is a herdr workspace: CreateWorkspace hands the pane only the vars named
// explicitly, the pane is a child of the herdr DAEMON, and the runtime is
// typed into that pane's LOGIN shell — so what it reads is the daemon's
// environment plus whatever the login rc exports, the same three-way split
// ADR 0013 already records for PATH. Nothing makes these two names an
// exception: an `export CLAUDE_SECURESTORAGE_CONFIG_DIR=…` in a login rc
// reaches the runtime and never reaches the launcher, and the launcher's own
// `CLAUDE_CONFIG_DIR=… posse new` reaches the launcher and not the pane.
//
// What holds the wall and the write together is therefore not inheritance
// but the PIN (credentialDirPin, ADR 0019 D2 store 3, ranger-base-rq83c):
// the launch line carries both names in its `--settings` payload, resolved
// on the launch path, and the runtime applies each settings scope's env
// block OVER process.env in the order user, project, local, flag, policy —
// so the flag-scope payload lands after the pane's environment as surely as
// it lands after a settings file the persona wrote. MEASURED on claude
// 2.1.259; a launcher that merely EXPORTED the directory into the child
// would have lost to both. credentialpanesplit_qa_test.go pins that coupling
// from the rendered line, with the unpinned pane as its control.
//
// Refused rather than patched. Adding the set's directory to the deny would
// mean resolving the env sets before the profile is rendered, which reorders
// every refusal between here and there; refusing is additive, fail-closed,
// and it costs nothing measured — 0 env files carried either name when this
// was written. It is also the right shape after the pin rather than a
// leftover before it: an env set's variable is a scope the settings payload
// already outranks, so an APPEND there would be a wall that loses. The
// remedy is in the message and it is a real one: exported where the launcher
// can see it, the variable moves the deny and the pin together, and the pin
// is what moves the write.
//
// Only where the wall exists (seatbeltWallRendered). A shims-tier session
// has no file-read deny for a variable to walk past, and a caged one renders
// its own profile inside, so refusing either would be a wall over nothing.
//
// NAMES only. An env set naming the directory the launcher already resolved
// would change nothing and is refused anyway, and that is the deliberate
// half: telling those two cases apart means reading the set's VALUE on a
// launch path, and an env set's values are the one thing those paths are
// careful never to learn (credentialDirVarsIn returns the name it looked
// for, never the key's value). A refusal an operator clears in one line is
// the cheaper mistake.
//
// One function, not one spelling per launch path (ranger-base-179hy). Two
// paths render a persona line — planLaunch and RelaunchAgent — and the
// second was written without this refusal, so an env set edited after the
// launch moved the credential write of an UNATTENDED revival, where nobody
// is present to read the refusal that never came. The guard and the scan
// travel together now: a third launch path omits the pair or calls it, and
// TestQAEveryLaunchPathThatRendersASeatbeltRefusesACredentialDirEnvSet says
// which.
func credentialDirEnvSetRefusal(cage string, rt *Runtime, vars []EnvVar) error {
	if !seatbeltWallRendered(cage, rt) {
		return nil
	}
	names := credentialDirVarsIn(vars)
	if len(names) == 0 {
		return nil
	}
	return Die("env set exports %s, and the seatbelt's credential read-deny for this launch was already rendered from the launcher's own environment — the session would write its credential to a directory the sandbox never walled (ADR 0019 D2, ranger-base-x5f6p). Export it in the launching shell instead, where the deny follows it, or drop it from the env set", strings.Join(names, " and "))
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
	read := credentialsFileRead(p, name)
	s.Read = func() ([]byte, error) {
		if perr != nil {
			return nil, perr
		}
		return read()
	}
	return s
}

// credentialsFileRead reads the runtime's credentials file under the name
// that will carry the diagnosis. Two stores share it — the non-darwin store
// of record, and the darwin composite's fallback — and they differ only in
// what they are CALLED, which is the same rule credentialToken already keeps
// for the envelope (ADR 0019 V7): one fixture, two paths, one diagnosis.
func credentialsFileRead(p, name string) func() ([]byte, error) {
	return func() ([]byte, error) {
		b, err := os.ReadFile(p)
		if err != nil {
			// The path is the diagnosis and the contents are the credential:
			// os.ReadFile's own message carries the first and not the second,
			// but it is not ours, so it is restated rather than wrapped.
			return nil, Die("%s is unreadable", name)
		}
		return b, nil
	}
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

// credentialDirPin is the env block a launch hands the claude runtime so
// that a ~/.claude/settings.json the persona can write cannot move where
// that runtime reads and writes its OAuth credential (ranger-base-rq83c).
//
// WHY A PIN AND NOT AN EXPORT, measured on claude 2.1.259 in a scratch HOME
// (never the operator's), with `claude auth status --json` as the readout:
// its `projectsDirectory` is the resolved config dir, and `loggedIn` is
// true exactly when the resolved credential dir holds an envelope — a
// planted FAKE one here, never a live token.
//
//	arm                                            projectsDirectory  loggedIn
//	nothing set                                    $HOME/.claude      false
//	CLAUDE_CONFIG_DIR=<attacker> in process env    <attacker>         —
//	the same key in ~/.claude/settings.json `env`  <attacker>         true
//	both, fighting: process env vs settings.json   <attacker>         true
//	settings.json attack + this pin via --settings $HOME/.claude      false
//
// Row four is why the launcher exporting these variables is not the fix:
// the runtime Object.assign's each settings scope's `env` over process.env
// at startup, so the settings value wins over anything the launcher put in
// the child's environment. Row five is the fix: the scope order is
// userSettings, projectSettings, localSettings, flagSettings,
// policySettings, applied in that order, so `--settings` (flagSettings)
// lands AFTER the user's file and a persona cannot reach it — argv is not
// a file it can write. policySettings is higher still and would also cover
// the operator's OWN uncaged claude, but its only live source is the
// root-owned OS path; that half is the operator's and is filed separately.
// Two dead ends, both measured rather than assumed: `--managed-settings`
// (the SDK parent tier) does not carry `env` through its restrictive-only
// filter, and CLAUDE_CODE_MANAGED_SETTINGS_PATH is inert in 2.1.259 — the
// resolver's override hook is a stub that returns undefined.
//
// WHICH VALUES, and why not simply "$HOME/.claude" for both. The runtime
// reads these two variables twice, for two different answers — the
// credentials file's directory, and the KEYCHAIN ITEM'S NAME:
//
//	z_() = CLAUDE_SECURESTORAGE_CONFIG_DIR present ? (it || $HOME/.claude)
//	                                               : the config dir
//	Gx() = "Claude Code<suffix>" + (named ? "-"+sha256(dir)[:8] : ""), where
//	       `named` is whether a VARIABLE named the directory, never whether
//	       the path is the default one
//
// So a pin that merely names the right DIRECTORY can still rename the item
// out from under the operator's login (ranger-base-ig4op / ranger-base-mx4q6).
// The rule here is therefore not "the safe value" but "the value this
// environment already resolves to", and it is spelled through
// credentialDirNamed — the same function keychainItem derives that name
// from. That is what makes the invariant structural rather than lucky:
// pinning `dir` when a variable named it and EMPTY when none did leaves
// `named` and `dir` where they were, so keychainItem answers the same name
// before and after the pin, and CredentialsFile the same path. Written out,
// the four arms it covers:
//
//   - SECURESTORAGE set to a directory: pinned verbatim (named, dir = it).
//   - SECURESTORAGE set but EMPTY: pinned empty — presence, not truthiness;
//     it means $HOME/.claude to the runtime and must not become a path.
//   - SECURESTORAGE unset with CLAUDE_CONFIG_DIR set: pinned to the config
//     dir, which is what z_() already falls back to and what Gx already
//     hashes.
//   - neither set: pinned EMPTY, the one value that keeps the item
//     unsuffixed. Pinning $HOME/.claude here would add a hash suffix that
//     is not there today.
//
// CLAUDE_CONFIG_DIR is pinned to ClaudeConfigDirIn's answer, which is an
// absolute path or nothing: the runtime resolves that one with `??`, not
// `||`, so an empty value there is a config dir of "" — relative to the
// cwd — and is never a value to pin. On a box that exports it empty (a
// broken state either way) the pin moves the runtime onto posse's answer,
// $HOME/.claude, instead of "" — the divergence ranger-base-e9xba carries.
//
// Read from THIS process's environment, like credentialReadDenyLiterals
// above it and for the same reason: the seatbelt's credential read-deny is
// rendered from the launcher's env, so a pin taken from the same place
// cannot leave the wall denying one path while the runtime writes another.
// A launch whose env set carries either name is refused before it gets
// here (credentialDirVarsIn), so the two can never disagree.
//
// No home is no pin, not an error — the same call credentialFileCandidates
// makes: with no home there is no path to name and nothing to protect.
func credentialDirPin() []EnvVar {
	dir, named, err := credentialDirNamed()
	if err != nil {
		return nil
	}
	home, herr := os.UserHomeDir()
	if herr != nil || home == "" {
		return nil
	}
	sec := ""
	if named {
		sec = dir
	}
	return []EnvVar{
		{Key: "CLAUDE_SECURESTORAGE_CONFIG_DIR", Value: sec},
		{Key: "CLAUDE_CONFIG_DIR", Value: ClaudeConfigDirIn(home)},
	}
}

// credentialDirPinJSON is the whole settings pin as a payload on its own —
// what a launch line carrying no settings flag of its own gets appended
// (EnsureSettingsPin). `{}` when there is no pin to make, which is a flag
// the CLI accepts and a line that changes nothing.
//
// It reads settingsPin, not credentialDirPin: since ranger-base-rflee the
// pin is the credential dirs AND the transport/exec inlets (inletpin.go),
// and since ranger-base-i7cy4 the command-string fields beside them
// (fieldpin.go). A hand-written `command:` deserves the same guarantee for
// all three. The name is the one the runtime field and its pins already use.
func credentialDirPinJSON() string {
	pin := settingsPin()
	if len(pin) == 0 {
		return "{}"
	}
	env := make(map[string]string, len(pin))
	for _, v := range pin {
		env[v.Key] = v.Value
	}
	out := map[string]any{"env": env}
	for _, f := range fieldPin() {
		out[f.Key] = f.Value
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "{}"
	}
	return string(b)
}
