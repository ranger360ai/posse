package posse

// The ssh identity files a seat that may not push is masked off
// (ranger-base-ighvi).
//
// THE HOLE. The bwrap wall is a write-scope wall: `--ro-bind / /` leaves
// every file the account can read readable, and its only read masks were the
// runtime credentials (credentialReadDenyLiterals). A box that pushes with
// passphrase-less deploy keys therefore handed every bwrap seat a credential
// that writes its remotes, and an ssh transport that does not spell
// `git push` goes around every push gate posse has — the PID deny, the L1
// shim and the pre-push hook all read the verb, never the key.
//
// THE LIST IS SSH'S OWN, read the way ssh reads it (ranger-base-7pf1h: the
// wall reads what the reader reads). Not a glob over ~/.ssh: a key can live
// anywhere a config line names, and a glob guesses which files are keys.
// What ssh would try is (a) every IdentityFile in the user and system
// configs, through Include, in every Host/Match block — wider than any one
// host's resolution, which is the safe direction for a mask — and (b) the
// default identity names ssh falls back to when a host names none.
//
// THE COST, accepted with the fix: a seat behind the mask cannot fetch over
// an ssh remote whose key is masked either — a deploy key is one credential
// for both directions. A seat that needs a fetch uses an https remote.
//
// AND AN ALLOWLIST OVER ~/.ssh (ranger-base-12ggv, closing ranger-base-nl2ha).
// The list above cannot see a key that no config names and that is not at a
// default name, so the same seats also read nothing under ~/.ssh but ssh's
// config and known_hosts (SeatbeltCarveOut.SSHDir). Still not a glob: the
// allowlist names what a seat may read, never which files are keys, so a key
// copied there under any name is covered. The two halves are both needed —
// the allowlist is the directory, the list is everywhere a config points.
//
// Not closed here:
// - ranger-base-w4q8r

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// sshSystemConfig is ssh's system-wide client config, read after the user's.
const sshSystemConfig = "/etc/ssh/ssh_config"

// sshDefaultIdentities are the names ssh tries under ~/.ssh when a host names
// no IdentityFile (ssh(1), -i; readconf.c fill_default_options).
var sshDefaultIdentities = []string{"id_rsa", "id_ecdsa", "id_ecdsa_sk", "id_ed25519", "id_ed25519_sk", "id_xmss", "id_dsa"}

// sshIncludeDepth is ssh's own limit on nested Include (READCONF_MAX_DEPTH).
const sshIncludeDepth = 16

// bwrapMasksSSHKeys answers which seats get the mask: every seat except the
// pusher. A PID that denies push gets it (deny wins, ADR 0001), and so does a
// PID that says nothing about push either way, because the key is a push
// whether or not the verb was ever named — the shop's push belongs to the
// seat whose PID grants it (ADR 0033 §2), and to no other.
//
// DIVERGED from the bead's letter, "every seat whose PID denies push", in the
// safe direction: deniesGitPush is deliberately narrow (it reads two rule
// shapes and fails safe for the HOOK it installs), and narrow here would
// fail OPEN — a PID whose deny is spelled some third way would keep the key.
func bwrapMasksSSHKeys(ag *AgentFile) bool {
	if ag == nil {
		return true
	}
	return deniesGitPush(ag.Deny) || grantsGitPush(ag.Allow) == ""
}

// sshKeyReadDeny is the list a seat's wall read-denies for ssh: every key
// liveSSHIdentityFiles resolves, for a seat bwrapMasksSSHKeys says is not the
// pusher, and nothing for the pusher. ONE function for both mechanisms
// (ranger-base-7m3up): bwrapRender renders it as /dev/null binds, and
// RenderSeatbelt as `(deny file-read* (literal …))` through the same
// Carve.DenyRead, so the two walls cannot disagree about which keys a seat
// reads. Not inside SeatbeltCarveOut because the resolution reads ssh's
// configs and can refuse, and that function has no error to refuse with.
func sshKeyReadDeny(ag *AgentFile, cwd string) ([]string, error) {
	if !bwrapMasksSSHKeys(ag) {
		return nil, nil
	}
	return liveSSHIdentityFiles(cwd)
}

// withSSHKeyReadDeny adds sshKeyReadDeny's list to carve.DenyRead and returns
// the keys it added, so a report can name them as keys. For the same seats it
// fills carve.SSHDir with ~/.ssh for both homes (ranger-base-12ggv): the
// allowlist covers what the list cannot — a key no config names — and the
// list still covers what the allowlist cannot, a configured key OUTSIDE
// ~/.ssh. One helper for every render, so no wall gets one half.
func withSSHKeyReadDeny(ag *AgentFile, cwd string, carve *SeatbeltCarveOut) ([]string, error) {
	keys, err := sshKeyReadDeny(ag, cwd)
	if err != nil {
		return nil, err
	}
	carve.DenyRead = dedupeStrings(append(carve.DenyRead, keys...))
	if bwrapMasksSSHKeys(ag) {
		for _, h := range liveSSHHomes() {
			carve.SSHDir = append(carve.SSHDir, absResolve(filepath.Join(h, ".ssh")))
		}
		carve.SSHDir = dedupeStrings(carve.SSHDir)
	}
	return keys, nil
}

// sshEnv is what the expansion needs from the account, as values so a pin
// can drive every arm without the box's own config.
type sshEnv struct {
	Home   string // the home ssh's `~` and %d expand to
	User   string // %u
	UID    int    // %i
	Host   string // %l (and %L, its first label)
	Getenv func(string) string
	Cwd    string // a relative IdentityFile is opened relative to ssh's cwd: the seat's
	System string // the system config; "" reads none
}

// sshIdentityFiles is the list for one account: every IdentityFile the user
// and system configs name, the private half of any that names a `.pub`, and
// the default names, all absolute. A path that is not there is kept — the
// render skips it (BwrapArgs step 5), and keeping it costs nothing.
//
// "COULD NOT ASK" REFUSES, the render's own doctrine (bwrapStatRefusal): a
// config that is there and cannot be read is a wall that would be built
// without knowing where the keys are. Not there is not an error; that is
// what a box without ssh looks like.
func sshIdentityFiles(e sshEnv) ([]string, error) {
	var out []string
	if e.Home != "" {
		sshDir := filepath.Join(e.Home, ".ssh")
		for _, n := range sshDefaultIdentities {
			out = append(out, filepath.Join(sshDir, n))
		}
		named, err := sshConfigIdentities(e, filepath.Join(sshDir, "config"), sshDir, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, named...)
	}
	if e.System != "" {
		named, err := sshConfigIdentities(e, e.System, filepath.Dir(e.System), 0)
		if err != nil {
			return nil, err
		}
		out = append(out, named...)
	}
	var abs []string
	for _, p := range out {
		if !filepath.IsAbs(p) {
			if e.Cwd == "" {
				continue // no cwd to open it against, so no file to name
			}
			p = filepath.Join(e.Cwd, p)
		}
		abs = append(abs, filepath.Clean(p))
		if strings.HasSuffix(p, ".pub") {
			// A config may name the PUBLIC half to pick an agent identity;
			// the private half beside it is the file worth masking.
			abs = append(abs, filepath.Clean(strings.TrimSuffix(p, ".pub")))
		}
	}
	return shallowestFirst(abs), nil
}

// sshConfigIdentities reads one config file and everything it Includes.
// incBase is where a relative Include resolves: ~/.ssh for the user config,
// /etc/ssh for the system one (ssh_config(5), Include).
func sshConfigIdentities(e sshEnv, path, incBase string, depth int) ([]string, error) {
	if depth > sshIncludeDepth {
		return nil, Die("ssh config %s: Include nested deeper than ssh's own limit of %d, so the wall cannot say which keys it names (ranger-base-ighvi)", path, sshIncludeDepth)
	}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		if errors.Is(err, syscall.EISDIR) {
			return nil, nil // ssh skips a directory a glob matched
		}
		return nil, Die("bwrap render: cannot read the ssh config %s: %v — the wall masks the keys ssh would use, and it is not rendered without knowing which they are (ranger-base-ighvi)", path, err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		words := sshSplit(line)
		if len(words) == 0 {
			continue
		}
		key, args := strings.ToLower(words[0]), words[1:]
		switch key {
		case "identityfile":
			if len(args) == 0 || strings.EqualFold(args[0], "none") {
				continue
			}
			out = append(out, sshExpandIdentity(e, args[0])...)
		case "include":
			for _, a := range args {
				p := sshExpandEnv(e, sshTilde(e, a))
				if !filepath.IsAbs(p) {
					p = filepath.Join(incBase, p)
				}
				matches, err := filepath.Glob(p)
				if err != nil {
					return nil, Die("ssh config %s: Include %q is not a pattern ssh can expand: %v (ranger-base-ighvi)", path, a, err)
				}
				for _, m := range matches {
					named, err := sshConfigIdentities(e, m, incBase, depth+1)
					if err != nil {
						return nil, err
					}
					out = append(out, named...)
				}
			}
		}
	}
	return out, nil
}

// sshSplit is one config line as ssh's argv_split reads it: the keyword may
// be followed by `=`, words are split on blanks, double or single quotes
// group, and an unquoted word starting `#` ends the line.
func sshSplit(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	// keyword=value and keyword = value are both legal: one `=` after the
	// keyword is a separator.
	if i := strings.IndexAny(line, " \t="); i > 0 {
		rest := strings.TrimLeft(line[i:], " \t")
		rest = strings.TrimPrefix(rest, "=")
		line = line[:i] + " " + rest
	}
	var words []string
	var cur strings.Builder
	in, quote := false, byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote, in = c, true
		case c == ' ' || c == '\t':
			if in {
				words = append(words, cur.String())
				cur.Reset()
				in = false
			}
		case c == '#' && !in:
			return words
		case c == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
			in = true
		default:
			cur.WriteByte(c)
			in = true
		}
	}
	if in {
		words = append(words, cur.String())
	}
	return words
}

// sshTilde expands a leading `~` or `~/` to the account's home. `~user` is
// left alone and then reads as relative, which the caller resolves against
// the cwd — a mask over a path that is not there, which costs nothing.
func sshTilde(e sshEnv, p string) string {
	if e.Home == "" {
		return p
	}
	if p == "~" {
		return e.Home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(e.Home, p[2:])
	}
	return p
}

// sshExpandEnv is ssh's `${VAR}` expansion. An unset variable expands to
// nothing here; ssh refuses the line instead, so either way no key is read
// at that spelling.
func sshExpandEnv(e sshEnv, p string) string {
	getenv := e.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	var b strings.Builder
	for {
		i := strings.Index(p, "${")
		if i < 0 {
			b.WriteString(p)
			return b.String()
		}
		j := strings.Index(p[i:], "}")
		if j < 0 {
			b.WriteString(p)
			return b.String()
		}
		b.WriteString(p[:i])
		b.WriteString(getenv(p[i+2 : i+j]))
		p = p[i+j+1:]
	}
}

// sshExpandIdentity is one IdentityFile value as the files it can name.
// The account tokens (%d %u %i %l %L %%) expand to their one value. The
// per-connection ones (%h %r %n %p %C %j %k) have a value per host, so each
// becomes a `*` and the filesystem answers which files match — a config
// keyed on the host name names every host's key that way, and a mask is
// rather over every one of them than under any.
func sshExpandIdentity(e sshEnv, raw string) []string {
	p := sshExpandEnv(e, sshTilde(e, raw))
	if !strings.Contains(p, "%") {
		return []string{p}
	}
	short := e.Host
	if i := strings.IndexByte(short, '.'); i >= 0 {
		short = short[:i]
	}
	var b strings.Builder
	wild := false
	for i := 0; i < len(p); i++ {
		if p[i] != '%' || i+1 >= len(p) {
			b.WriteString(sshGlobEscape(string(p[i])))
			continue
		}
		i++
		switch p[i] {
		case '%':
			b.WriteString(sshGlobEscape("%"))
		case 'd':
			b.WriteString(sshGlobEscape(e.Home))
		case 'u':
			b.WriteString(sshGlobEscape(e.User))
		case 'i':
			b.WriteString(strconv.Itoa(e.UID))
		case 'l':
			b.WriteString(sshGlobEscape(e.Host))
		case 'L':
			b.WriteString(sshGlobEscape(short))
		default:
			b.WriteString("*")
			wild = true
		}
	}
	pat := b.String()
	if !wild {
		// No per-connection token: the pattern is a literal path, escaped.
		return []string{sshGlobUnescape(pat)}
	}
	if !filepath.IsAbs(pat) && e.Cwd != "" {
		pat = filepath.Join(e.Cwd, pat)
	}
	matches, _ := filepath.Glob(pat) // the only error is a bad pattern, and every metachar was escaped
	return matches
}

func sshGlobEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`)
	return r.Replace(s)
}

func sshGlobUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// liveSSHIdentityFiles is sshIdentityFiles for this account, for both homes
// the question has: the passwd entry, which is the one ssh expands `~` and
// finds its config by, and $HOME, which every other reader on the box uses.
// On a normal host they are one directory and one list.
func liveSSHIdentityFiles(cwd string) ([]string, error) {
	name := os.Getenv("USER")
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	host, _ := os.Hostname()
	var out []string
	system := sshSystemConfig
	for _, h := range liveSSHHomes() {
		got, err := sshIdentityFiles(sshEnv{Home: h, User: name, UID: os.Getuid(), Host: host, Cwd: cwd, System: system})
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
		system = "" // read once
	}
	var abs []string
	for _, p := range out {
		abs = append(abs, absResolve(p))
	}
	return shallowestFirst(abs), nil
}

// liveSSHHomes is the two homes the ssh question has, passwd's first, non-empty
// and once each: the list liveSSHIdentityFiles walks and the dirs SSHDir hides.
func liveSSHHomes() []string {
	homes := []string{os.Getenv("HOME")}
	if u, err := user.Current(); err == nil {
		homes = append([]string{u.HomeDir}, homes...)
	}
	return dedupeStrings(homes)
}
