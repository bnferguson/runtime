package oidcauth

import (
	"strings"
	"time"
)

// globMatch is like path.Match but treats '*' as matching everything including '/'.
// This is necessary because OIDC subject claims contain slashes
// (e.g. "repo:acme/web-app:ref:refs/heads/main").
func globMatch(pattern, str string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Consume consecutive stars
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			// Try matching remaining pattern at every position
			for i := 0; i <= len(str); i++ {
				if globMatch(pattern, str[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(str) == 0 {
				return false
			}
			pattern = pattern[1:]
			str = str[1:]
		default:
			if len(str) == 0 || pattern[0] != str[0] {
				return false
			}
			pattern = pattern[1:]
			str = str[1:]
		}
	}
	return len(str) == 0
}

// Claims represents validated OIDC token claims.
type Claims struct {
	Issuer   string
	Subject  string
	Audience []string
	Expiry   time.Time
	Extra    map[string]any
}

// ClaimCondition represents a single claim matching rule.
type ClaimCondition struct {
	Key     string
	Pattern string
}

// MatchesSubjectPattern checks if the token's subject matches a glob pattern.
// Uses globMatch which matches '*' across '/' characters, unlike path.Match.
func (c *Claims) MatchesSubjectPattern(pattern string) bool {
	if pattern == "" {
		return true
	}
	return globMatch(pattern, c.Subject)
}

// MatchesClaimConditions checks if all conditions match. A pattern with commas
// (e.g. "push,workflow_dispatch") means the claim value must match any one of
// the alternatives.
func (c *Claims) MatchesClaimConditions(conditions []ClaimCondition) bool {
	for _, cond := range conditions {
		claimValue, ok := c.getClaimString(cond.Key)
		if !ok {
			return false
		}

		alternatives := strings.Split(cond.Pattern, ",")
		matched := false
		for _, alt := range alternatives {
			alt = strings.TrimSpace(alt)
			if globMatch(alt, claimValue) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (c *Claims) getClaimString(key string) (string, bool) {
	if c.Extra == nil {
		return "", false
	}
	v, ok := c.Extra[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
