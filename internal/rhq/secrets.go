package rhq

// Harness credentials: secrets/<name>.env, dir 700, files 600.
//
// ADR 0019 D1 (the instance-private credential architecture, accepted
// 2026-08-28, option (b) — ~/src/rangerhq/docs/adr/0019-credential-
// architecture.md, not in this public tree) splits credentials into two
// reader classes, and the split is the trust model, not a mechanism:
//
//	envs/<set>.env    a SESSION credential. Injected into exactly the
//	                  sessions whose PID `envs:` names, so the persona and
//	                  every tool it runs may hold, quote and log it.
//	secrets/<n>.env   a HARNESS credential. Read by posse's own processes,
//	                  never injected into any session, never listable by
//	                  `posse envs`, and nameable by no PID key.
//
// The one-hand rule: everything under envs/ may reach a session; nothing
// under secrets/ ever does. That is an INJECTION claim, not a
// confidentiality one — below the container tier any session runs as this
// uid and can `cat` a 600 file. What the split buys is scoped mint and
// individual revocation: a leak burns one credential, not the account. The
// wall for secrecy is the container tier.
//
// Two things this file deliberately does NOT have, both load-bearing:
//
//   - a lister. `posse envs` reads ListEnvSets, which reads EnvsDir and
//     nothing else; the loader here reads SecretsDir and nothing else. The
//     two share no directory, which is how ADR 0019 P3 is a unit test
//     rather than a promise (secrets_test.go).
//   - a resident. The plan guard is not a consumer: P1 measured HTTP 403
//     for every operator-mintable credential at the usage endpoint, so the
//     meter token stays the runtime's own (credential.go). `posse init`
//     seeds the empty directory and never a plan-guard.env. When a harness
//     credential does arrive — a second provider's meter, a webhook — it
//     reaches its caller through the ReadCredential seam, which is the one
//     place posse acquires a credential; this file is where that seam's
//     harness half will read from, not a second acquisition path.

import (
	"io"
	"os"
	"path/filepath"
)

// secretFilePath resolves secrets/<name>.env (or a bare `name`, the same
// grammar envFilePath allows).
func (a *App) secretFilePath(name string) (string, error) {
	if !storeName(name) {
		return "", Die("harness secret name must be a file stem, not a path: %q", name)
	}
	f := filepath.Join(a.SecretsDir, name+".env")
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	f = filepath.Join(a.SecretsDir, name)
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	return "", Die("harness secret not found: %s (looked in %s)", name, AbbrevHome(a.SecretsDir))
}

// SecretVars reads one harness secret file's variables, tightening the
// file's mode first if it has drifted (TightenEnvPerms parity, rangerhq-f2b).
//
// Same KEY=VALUE grammar as an env set, same parser — one grammar for the
// operator to know — and a separate reader, because what differs between the
// two classes is not how the bytes are spelled but who may see them. A
// missing or empty secrets/ is not an error condition of the harness: it is
// the shipped state (ADR 0019 P6), and this returns the ordinary not-found
// so a caller can say what it wanted rather than crashing.
func (a *App) SecretVars(name string) ([]EnvVar, error) {
	f, err := a.secretFilePath(name)
	if err != nil {
		return nil, err
	}
	a.TightenSecretPerms(os.Stderr)
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	return parseEnvLines(string(b)), nil
}

// TightenSecretPerms restores secrets/ to 700 and every file in it to 600,
// noting each drifted path on w (names only, never contents) — the same
// belt TightenEnvPerms is, on the store one class up. `posse init` sets the
// modes once; a file the operator copied or edited by hand drifts to 644,
// and every launch re-asserts. Cheap, idempotent, silent when nothing
// drifted, and a no-op when the directory is not there.
func (a *App) TightenSecretPerms(w io.Writer) {
	tightenCredentialDir(w, a.SecretsDir, "harness credentials are posse's own and reach no session")
}
