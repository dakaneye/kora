// Package codeowners provides parsing and matching for GitHub CODEOWNERS files.
//
// CODEOWNERS files define ownership rules for files and directories in a repository.
// This package implements the GitHub CODEOWNERS format specification including
// glob pattern matching and "last match wins" semantics.
package codeowners

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
)

// Rule represents a single ownership rule from a CODEOWNERS file.
type Rule struct {
	// Pattern is the file path pattern (glob) that this rule matches.
	Pattern string

	// Owners is the list of users/teams that own files matching this pattern.
	// Users are prefixed with @ (e.g., "@username").
	// Teams are prefixed with @org/ (e.g., "@org/team-name").
	Owners []string
}

// Ruleset is a collection of parsed CODEOWNERS rules.
type Ruleset struct {
	// Rules contains all parsed rules in order of appearance.
	// Per GitHub semantics, the last matching rule wins.
	Rules []Rule
}

// Parse parses CODEOWNERS file content and returns a Ruleset.
// Empty content or content with only comments returns an empty Ruleset.
//
// The parser follows GitHub CODEOWNERS format:
//   - Lines starting with # are comments
//   - Empty lines are ignored
//   - Each non-comment line is: pattern owner1 owner2 ...
//   - Pattern is a path or glob pattern
//   - Owners are space-separated usernames or team references
func Parse(content []byte) (*Ruleset, error) {
	rs := &Ruleset{
		Rules: make([]Rule, 0),
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule := parseLine(line)
		if rule != nil {
			rs.Rules = append(rs.Rules, *rule)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return rs, nil
}

// parseLine parses a single CODEOWNERS line into a Rule.
// Returns nil if the line has no pattern or no owners.
func parseLine(line string) *Rule {
	// Split on whitespace
	fields := strings.Fields(line)
	if len(fields) < 2 {
		// Need at least pattern and one owner
		return nil
	}

	pattern := fields[0]
	owners := make([]string, 0, len(fields)-1)

	// Validate and collect owners
	for _, owner := range fields[1:] {
		// Skip inline comments
		if strings.HasPrefix(owner, "#") {
			break
		}
		// Owners should start with @ or be email addresses
		// GitHub accepts both, we'll accept any non-empty string
		if owner != "" {
			owners = append(owners, owner)
		}
	}

	if len(owners) == 0 {
		return nil
	}

	return &Rule{
		Pattern: pattern,
		Owners:  owners,
	}
}

// Match returns the owners for a given path using "last match wins" semantics.
// Returns nil if no rules match the path.
//
// The path should be relative to the repository root, using forward slashes.
func (r *Ruleset) Match(path string) []string {
	if r == nil || len(r.Rules) == 0 {
		return nil
	}

	// Normalize path: remove leading slash, use forward slashes
	path = normalizePath(path)

	var lastMatch []string
	for _, rule := range r.Rules {
		if matchPattern(rule.Pattern, path) {
			lastMatch = rule.Owners
		}
	}

	return lastMatch
}

// normalizePath normalizes a filepath for matching.
func normalizePath(path string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")
	// Ensure forward slashes
	path = strings.ReplaceAll(path, "\\", "/")
	return path
}

// matchPattern checks if a filepath matches a CODEOWNERS pattern.
//
// Pattern syntax:
//   - * matches any sequence of characters (not including /)
//   - ** matches any sequence including /
//   - ? matches a single character
//   - [abc] matches character classes
//   - Leading / anchors to repository root
//   - Trailing / matches directories
//   - Patterns without / match files anywhere
func matchPattern(pattern, path string) bool {
	// Normalize pattern
	origPattern := pattern
	pattern = strings.TrimPrefix(pattern, "/")

	// Handle trailing slash (directory match)
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")

		// Special case: ** or **/ matches everything
		if pattern == "**" {
			return true
		}

		// For directory patterns, path must be inside that directory
		// If pattern contains **, we need glob matching
		if strings.Contains(pattern, "**") {
			return matchDoublestar(pattern, path) || matchDoublestar(pattern+"/", path)
		}

		if !strings.HasPrefix(path, pattern+"/") && path != pattern {
			return false
		}
		return true
	}

	// Check if pattern contains path separator
	hasPathSep := strings.Contains(pattern, "/")

	// Pattern without "/" matches anywhere in the path
	if !hasPathSep && !strings.HasPrefix(origPattern, "/") {
		// Match against each component and also the full path
		return matchGlob(pattern, path) || matchBasename(pattern, path)
	}

	// Pattern with "/" or anchored: match against full path
	return matchGlob(pattern, path)
}

// matchBasename checks if pattern matches the basename of path
// or matches path when it appears as a suffix component.
func matchBasename(pattern, path string) bool {
	// Try matching basename
	base := filepath.Base(path)
	if matchGlob(pattern, base) {
		return true
	}

	// Try matching as suffix (e.g., "*.go" should match "foo/bar.go")
	// Split path into components and try matching each
	parts := strings.Split(path, "/")
	for i := range parts {
		// Build suffix from this point
		suffix := strings.Join(parts[i:], "/")
		if matchGlob(pattern, suffix) {
			return true
		}
	}

	return false
}

// matchGlob performs glob matching supporting *, **, ?, and [].
func matchGlob(pattern, path string) bool {
	// Handle ** (match any path segments)
	if strings.Contains(pattern, "**") {
		return matchDoublestar(pattern, path)
	}

	// Use filepath.Match for standard glob patterns
	// But we need to handle paths with / specially since filepath.Match
	// doesn't match / with *
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}

// matchDoublestar handles ** patterns that can match across directory boundaries.
func matchDoublestar(pattern, path string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")

	if len(parts) == 1 {
		// No ** found, use standard matching
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			return false
		}
		return matched
	}

	// Handle leading **
	if parts[0] == "" {
		// Pattern starts with **
		suffix := strings.TrimPrefix(pattern, "**")
		suffix = strings.TrimPrefix(suffix, "/")
		if suffix == "" {
			// Pattern is just ** or **/
			return true
		}
		// Try matching suffix against path and all suffixes
		return matchAnySuffix(suffix, path)
	}

	// Handle pattern like "foo/**/bar"
	prefix := parts[0]
	suffix := strings.Join(parts[1:], "**")

	// Remove trailing/leading slashes from prefix/suffix
	prefix = strings.TrimSuffix(prefix, "/")
	suffix = strings.TrimPrefix(suffix, "/")

	// Path must start with prefix
	if prefix != "" {
		if !strings.HasPrefix(path, prefix) && !matchGlobPrefix(prefix, path) {
			return false
		}
	}

	// If suffix is empty, prefix match is sufficient
	if suffix == "" {
		return true
	}

	// Try matching suffix against remaining path
	// The ** can match 0 or more path segments
	if prefix == "" {
		return matchAnySuffix(suffix, path)
	}

	// Find where prefix ends in path
	remaining := strings.TrimPrefix(path, prefix)
	remaining = strings.TrimPrefix(remaining, "/")

	// Recursively handle remaining suffix (which may contain more **)
	if strings.Contains(suffix, "**") {
		return matchDoublestar(suffix, remaining)
	}

	return matchAnySuffix(suffix, remaining)
}

// matchGlobPrefix checks if path starts with a glob pattern.
func matchGlobPrefix(pattern, path string) bool {
	// For simple prefix matching
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) > len(pathParts) {
		return false
	}

	for i, pp := range patternParts {
		matched, err := filepath.Match(pp, pathParts[i])
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// matchAnySuffix tries to match pattern against path or any suffix of path.
func matchAnySuffix(pattern, path string) bool {
	// Try matching against full path
	if matchGlob(pattern, path) {
		return true
	}

	// Try matching against each suffix
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], "/")
		if matchGlob(pattern, suffix) {
			return true
		}
	}

	return false
}
