package posse

// Flat-YAML subset parsing — the Go twin of the awk yaml_* functions in
// bin/posse. Same deliberate limits (see NOTES.md): top-level scalars, inline
// and block lists, one-level maps, '#' comments, a wrapping pair of double
// quotes stripped, "null"/"~"/empty mean unset. bin/posse (tmux-reference
// branch) was the behavioral spec; one deliberate divergence: awk stripped
// a leading and a trailing '"' independently, so a value that merely ends
// in one (`command: ... "$(cat {file})"`) lost it (rangerhq-nvq). Here the
// quotes go only as a matched pair.

import (
	"os"
	"strings"
)

func yamlClean(s string) string {
	if i := findComment(s); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

// findComment matches the awk rule: a comment starts at whitespace + '#'.
func findComment(s string) int {
	for i := 1; i < len(s); i++ {
		if s[i] == '#' && (s[i-1] == ' ' || s[i-1] == '\t') {
			return i - 1
		}
	}
	return -1
}

func readLines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "\n")
}

// YamlGet returns the scalar value of a top-level key ("" if unset).
func YamlGet(path, key string) string { return yamlGetLines(readLines(path), key) }

func yamlGetLines(lines []string, key string) string {
	for _, ln := range lines {
		if !strings.HasPrefix(ln, key+":") {
			continue
		}
		v := yamlClean(ln[len(key)+1:])
		if v == "null" || v == "~" {
			return ""
		}
		return v
	}
	return ""
}

// YamlList returns list items of a top-level key (inline or block form).
func YamlList(path, key string) []string { return yamlListLines(readLines(path), key) }

func yamlListLines(lines []string, key string) []string {
	var out []string
	inblock := false
	for _, ln := range lines {
		if !inblock {
			if !strings.HasPrefix(ln, key+":") {
				continue
			}
			v := yamlClean(ln[len(key)+1:])
			if strings.HasPrefix(v, "[") { // inline: key: [a, b]
				v = strings.TrimPrefix(v, "[")
				v = strings.TrimSuffix(v, "]")
				for _, part := range strings.Split(v, ",") {
					if x := yamlClean(part); x != "" {
						out = append(out, x)
					}
				}
				return out
			}
			inblock = true
			continue
		}
		trimmed := strings.TrimLeft(ln, " \t")
		if trimmed != ln && strings.HasPrefix(trimmed, "-") {
			x := yamlClean(strings.TrimLeft(strings.TrimPrefix(trimmed, "-"), " \t"))
			if x != "" {
				out = append(out, x)
			}
			continue
		}
		if ln != "" && ln[0] != ' ' && ln[0] != '\t' && ln[0] != '#' {
			break
		}
	}
	return out
}

// YamlMapPairs returns ordered subkey/value pairs of a one-level map.
func YamlMapPairs(path, mapkey string) [][2]string {
	var out [][2]string
	inmap := false
	for _, ln := range readLines(path) {
		if !inmap {
			if strings.HasPrefix(ln, mapkey+":") {
				inmap = true
			}
			continue
		}
		if ln != "" && ln[0] != ' ' && ln[0] != '\t' && ln[0] != '#' {
			break
		}
		trimmed := strings.TrimLeft(ln, " \t")
		if trimmed == "" || trimmed == ln || trimmed[0] == '#' {
			continue
		}
		i := strings.Index(trimmed, ":")
		if i <= 0 {
			continue
		}
		out = append(out, [2]string{trimmed[:i], yamlClean(trimmed[i+1:])})
	}
	return out
}

// yamlHasKey reports whether a top-level key is present at all (so a
// present-but-empty map can be told from an absent one).
func yamlHasKey(path, key string) bool {
	for _, ln := range readLines(path) {
		if strings.HasPrefix(ln, key+":") {
			return true
		}
	}
	return false
}

// YamlKeysWithPrefix returns the top-level keys that begin with prefix, in
// file order, once each. It is the reader for a family of keys whose members
// this harness does not know in advance — `plan_guard_<window>:`, where the
// window names belong to whichever provider adapter is installed
// (planusage.go, ADR 0012 D4).
//
// Top-level means what it means everywhere else in this file: a line whose
// first character is not space, tab or '#'.
func YamlKeysWithPrefix(path, prefix string) []string {
	var out []string
	seen := map[string]bool{}
	for _, ln := range readLines(path) {
		if ln == "" || ln[0] == ' ' || ln[0] == '\t' || ln[0] == '#' {
			continue
		}
		i := strings.Index(ln, ":")
		if i <= 0 {
			continue
		}
		k := ln[:i]
		if !strings.HasPrefix(k, prefix) || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}
