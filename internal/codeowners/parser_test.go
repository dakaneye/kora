package codeowners

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name    string
		content string
		want    *Ruleset
		wantErr bool
	}{
		{
			name:    "empty file",
			content: "",
			want:    &Ruleset{Rules: []Rule{}},
			wantErr: false,
		},
		{
			name:    "only comments",
			content: "# This is a comment\n# Another comment\n",
			want:    &Ruleset{Rules: []Rule{}},
			wantErr: false,
		},
		{
			name:    "only whitespace",
			content: "   \n\t\n  \n",
			want:    &Ruleset{Rules: []Rule{}},
			wantErr: false,
		},
		{
			name:    "single rule with one owner",
			content: "* @global-owner",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*", Owners: []string{"@global-owner"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "single rule with multiple owners",
			content: "*.js @frontend-team @ui-team",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*.js", Owners: []string{"@frontend-team", "@ui-team"}},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple rules",
			content: `# Global owners
* @global-owner

# Frontend team owns all JS files
*.js @frontend-team
src/components/** @ui-team @frontend-team

# Backend owns Go files
*.go @backend-team
/internal/auth/ @security-team
`,
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*", Owners: []string{"@global-owner"}},
					{Pattern: "*.js", Owners: []string{"@frontend-team"}},
					{Pattern: "src/components/**", Owners: []string{"@ui-team", "@frontend-team"}},
					{Pattern: "*.go", Owners: []string{"@backend-team"}},
					{Pattern: "/internal/auth/", Owners: []string{"@security-team"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "line with inline comment",
			content: "*.go @backend # Go files owned by backend",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*.go", Owners: []string{"@backend"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "pattern with no owners is skipped",
			content: "*.go\n*.js @frontend",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*.js", Owners: []string{"@frontend"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "email owners",
			content: "docs/** docs@example.com @docs-team",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "docs/**", Owners: []string{"docs@example.com", "@docs-team"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "complex patterns",
			content: "src/**/*.test.js @test-team\n/build/** @build-team\napps/*/config.yaml @platform",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "src/**/*.test.js", Owners: []string{"@test-team"}},
					{Pattern: "/build/**", Owners: []string{"@build-team"}},
					{Pattern: "apps/*/config.yaml", Owners: []string{"@platform"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "team references",
			content: "*.go @org/backend-team @org/go-reviewers",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*.go", Owners: []string{"@org/backend-team", "@org/go-reviewers"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "windows line endings",
			content: "*.go @backend\r\n*.js @frontend\r\n",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*.go", Owners: []string{"@backend"}},
					{Pattern: "*.js", Owners: []string{"@frontend"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "tabs as separators",
			content: "*.go\t@backend\t@devops",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*.go", Owners: []string{"@backend", "@devops"}},
				},
			},
			wantErr: false,
		},
		{
			name:    "mixed whitespace",
			content: "  *.go   @backend  @devops  ",
			want: &Ruleset{
				Rules: []Rule{
					{Pattern: "*.go", Owners: []string{"@backend", "@devops"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRuleset_Match(t *testing.T) {
	tests := []struct {
		name     string
		rules    []Rule
		filepath string
		want     []string
	}{
		{
			name:     "nil ruleset returns nil",
			rules:    nil,
			filepath: "foo.go",
			want:     nil,
		},
		{
			name:     "empty ruleset returns nil",
			rules:    []Rule{},
			filepath: "foo.go",
			want:     nil,
		},
		{
			name: "wildcard matches everything",
			rules: []Rule{
				{Pattern: "*", Owners: []string{"@global"}},
			},
			filepath: "anything.txt",
			want:     []string{"@global"},
		},
		{
			name: "extension pattern",
			rules: []Rule{
				{Pattern: "*.go", Owners: []string{"@go-team"}},
			},
			filepath: "main.go",
			want:     []string{"@go-team"},
		},
		{
			name: "extension pattern in subdirectory",
			rules: []Rule{
				{Pattern: "*.go", Owners: []string{"@go-team"}},
			},
			filepath: "internal/auth/auth.go",
			want:     []string{"@go-team"},
		},
		{
			name: "no match returns nil",
			rules: []Rule{
				{Pattern: "*.go", Owners: []string{"@go-team"}},
			},
			filepath: "script.js",
			want:     nil,
		},
		{
			name: "last match wins",
			rules: []Rule{
				{Pattern: "*", Owners: []string{"@global"}},
				{Pattern: "*.go", Owners: []string{"@go-team"}},
				{Pattern: "internal/**", Owners: []string{"@internal-team"}},
			},
			filepath: "internal/auth/auth.go",
			want:     []string{"@internal-team"},
		},
		{
			name: "directory pattern with trailing slash",
			rules: []Rule{
				{Pattern: "docs/", Owners: []string{"@docs-team"}},
			},
			filepath: "docs/README.md",
			want:     []string{"@docs-team"},
		},
		{
			name: "anchored pattern",
			rules: []Rule{
				{Pattern: "/internal/auth/", Owners: []string{"@security-team"}},
			},
			filepath: "internal/auth/auth.go",
			want:     []string{"@security-team"},
		},
		{
			name: "anchored pattern does not match nested",
			rules: []Rule{
				{Pattern: "/internal/", Owners: []string{"@internal-team"}},
			},
			filepath: "src/internal/foo.go",
			want:     nil,
		},
		{
			name: "doublestar matches any depth",
			rules: []Rule{
				{Pattern: "src/**", Owners: []string{"@src-team"}},
			},
			filepath: "src/foo/bar/baz.go",
			want:     []string{"@src-team"},
		},
		{
			name: "doublestar with extension",
			rules: []Rule{
				{Pattern: "**/*.test.js", Owners: []string{"@test-team"}},
			},
			filepath: "src/components/Button.test.js",
			want:     []string{"@test-team"},
		},
		{
			name: "question mark single char",
			rules: []Rule{
				{Pattern: "?.go", Owners: []string{"@single-char"}},
			},
			filepath: "a.go",
			want:     []string{"@single-char"},
		},
		{
			name: "question mark requires exact count",
			rules: []Rule{
				{Pattern: "?.go", Owners: []string{"@single-char"}},
			},
			filepath: "ab.go",
			want:     nil,
		},
		{
			name: "character class",
			rules: []Rule{
				{Pattern: "[abc].go", Owners: []string{"@abc-team"}},
			},
			filepath: "a.go",
			want:     []string{"@abc-team"},
		},
		{
			name: "character class no match",
			rules: []Rule{
				{Pattern: "[abc].go", Owners: []string{"@abc-team"}},
			},
			filepath: "d.go",
			want:     nil,
		},
		{
			name: "path with leading slash normalized",
			rules: []Rule{
				{Pattern: "*.go", Owners: []string{"@go-team"}},
			},
			filepath: "/internal/auth.go",
			want:     []string{"@go-team"},
		},
		{
			name: "complex real-world example",
			rules: []Rule{
				{Pattern: "*", Owners: []string{"@global-owner"}},
				{Pattern: "*.js", Owners: []string{"@frontend-team"}},
				{Pattern: "src/components/**", Owners: []string{"@ui-team", "@frontend-team"}},
				{Pattern: "*.go", Owners: []string{"@backend-team"}},
				{Pattern: "/internal/auth/", Owners: []string{"@security-team"}},
			},
			filepath: "internal/auth/provider.go",
			want:     []string{"@security-team"},
		},
		{
			name: "complex real-world example - js file",
			rules: []Rule{
				{Pattern: "*", Owners: []string{"@global-owner"}},
				{Pattern: "*.js", Owners: []string{"@frontend-team"}},
				{Pattern: "src/components/**", Owners: []string{"@ui-team", "@frontend-team"}},
				{Pattern: "*.go", Owners: []string{"@backend-team"}},
				{Pattern: "/internal/auth/", Owners: []string{"@security-team"}},
			},
			filepath: "src/components/Button.js",
			want:     []string{"@ui-team", "@frontend-team"},
		},
		{
			name: "complex real-world example - random file",
			rules: []Rule{
				{Pattern: "*", Owners: []string{"@global-owner"}},
				{Pattern: "*.js", Owners: []string{"@frontend-team"}},
				{Pattern: "src/components/**", Owners: []string{"@ui-team", "@frontend-team"}},
				{Pattern: "*.go", Owners: []string{"@backend-team"}},
				{Pattern: "/internal/auth/", Owners: []string{"@security-team"}},
			},
			filepath: "README.md",
			want:     []string{"@global-owner"},
		},
		{
			name: "doublestar at start matches any prefix",
			rules: []Rule{
				{Pattern: "**/config.yaml", Owners: []string{"@config-team"}},
			},
			filepath: "apps/myapp/config.yaml",
			want:     []string{"@config-team"},
		},
		{
			name: "doublestar at start matches root",
			rules: []Rule{
				{Pattern: "**/config.yaml", Owners: []string{"@config-team"}},
			},
			filepath: "config.yaml",
			want:     []string{"@config-team"},
		},
		{
			name: "pattern with single star in middle",
			rules: []Rule{
				{Pattern: "apps/*/config.yaml", Owners: []string{"@platform"}},
			},
			filepath: "apps/myapp/config.yaml",
			want:     []string{"@platform"},
		},
		{
			name: "single star does not cross directories",
			rules: []Rule{
				{Pattern: "apps/*/config.yaml", Owners: []string{"@platform"}},
			},
			filepath: "apps/myapp/nested/config.yaml",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rs *Ruleset
			if tt.rules != nil {
				rs = &Ruleset{Rules: tt.rules}
			}
			got := rs.Match(tt.filepath)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Ruleset.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatch_LastMatchWins(t *testing.T) {
	// This test specifically verifies the "last match wins" semantics
	// which is critical for CODEOWNERS behavior
	content := `
# Default owners
* @default-team

# More specific rules override
*.go @go-team
internal/** @internal-team
internal/auth/** @security-team
`
	rs, err := Parse([]byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	tests := []struct {
		filepath string
		want     []string
	}{
		{"README.md", []string{"@default-team"}},
		{"main.go", []string{"@go-team"}},
		{"internal/config/config.go", []string{"@internal-team"}},
		{"internal/auth/provider.go", []string{"@security-team"}},
	}

	for _, tt := range tests {
		t.Run(tt.filepath, func(t *testing.T) {
			got := rs.Match(tt.filepath)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Match(%q) = %v, want %v", tt.filepath, got, tt.want)
			}
		})
	}
}

func TestParseLine(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name string
		line string
		want *Rule
	}{
		{
			name: "valid rule",
			line: "*.go @backend",
			want: &Rule{Pattern: "*.go", Owners: []string{"@backend"}},
		},
		{
			name: "pattern only",
			line: "*.go",
			want: nil,
		},
		{
			name: "empty line",
			line: "",
			want: nil,
		},
		{
			name: "inline comment stops parsing",
			line: "*.go @backend # comment",
			want: &Rule{Pattern: "*.go", Owners: []string{"@backend"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLine(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo/bar.go", "foo/bar.go"},
		{"/foo/bar.go", "foo/bar.go"},
		{"foo\\bar.go", "foo/bar.go"},
		{"/foo\\bar.go", "foo/bar.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePath(tt.input)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchPattern_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{
			name:    "empty pattern matches nothing",
			pattern: "",
			path:    "foo.go",
			want:    false,
		},
		{
			name:    "empty path",
			pattern: "*",
			path:    "",
			want:    true,
		},
		{
			name:    "pattern equals path",
			pattern: "foo.go",
			path:    "foo.go",
			want:    true,
		},
		{
			name:    "exact path with directory",
			pattern: "internal/auth/auth.go",
			path:    "internal/auth/auth.go",
			want:    true,
		},
		{
			name:    "pattern with special regex chars",
			pattern: "file[1].go",
			path:    "file1.go",
			want:    true,
		},
		{
			name:    "doublestar only",
			pattern: "**",
			path:    "any/path/here.go",
			want:    true,
		},
		{
			name:    "doublestar slash",
			pattern: "**/",
			path:    "any/path/here",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkParse(b *testing.B) {
	content := []byte(`
# Global owners
* @global-owner

# Frontend
*.js @frontend-team
*.ts @frontend-team
*.tsx @frontend-team
src/components/** @ui-team

# Backend
*.go @backend-team
/internal/** @internal-team
/cmd/** @cli-team

# Documentation
*.md @docs-team
/docs/** @docs-team

# Configuration
*.yaml @devops
*.json @devops
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(content)
	}
}

func BenchmarkMatch(b *testing.B) {
	rs := &Ruleset{
		Rules: []Rule{
			{Pattern: "*", Owners: []string{"@global"}},
			{Pattern: "*.js", Owners: []string{"@frontend"}},
			{Pattern: "*.go", Owners: []string{"@backend"}},
			{Pattern: "src/**", Owners: []string{"@src-team"}},
			{Pattern: "internal/**", Owners: []string{"@internal-team"}},
			{Pattern: "internal/auth/**", Owners: []string{"@security-team"}},
		},
	}

	paths := []string{
		"README.md",
		"main.go",
		"src/app.js",
		"internal/auth/provider.go",
		"deep/nested/path/file.ts",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			rs.Match(p)
		}
	}
}
