// Package rules implements Switchboard's rule matching.
package rules

import (
	"regexp"
	"strings"

	"switchboard/internal/config"
)

// Match returns the index of the first rule matching (source, url), or
// (-1, false) if none match. Matching is case-insensitive. A rule with
// neither source nor url set never matches.
func Match(rules []config.Rule, source, url string) (int, bool) {
	s := strings.ToLower(source)
	u := strings.ToLower(url)
	for i, r := range rules {
		if r.Source == "" && r.URL == "" {
			continue
		}
		if r.Source != "" && strings.ToLower(r.Source) != s {
			continue
		}
		if r.URL != "" && !Glob(strings.ToLower(r.URL), u) {
			continue
		}
		return i, true
	}
	return -1, false
}

// Glob reports whether s matches pattern, where * matches any run of
// characters and ? matches any single character. The pattern must cover the
// whole string (write *foo* for a substring match).
func Glob(pattern, s string) bool {
	var b strings.Builder
	b.WriteString(`^`)
	for _, ch := range pattern {
		switch ch {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteString(`$`)
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(s)
}
