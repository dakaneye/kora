package slack

import (
	"testing"
	"time"
)

func TestParseSlackTimestamp(t *testing.T) {
	//nolint:govet // test struct alignment not relevant
	tests := []struct {
		name    string
		ts      string
		want    time.Time
		wantErr bool
	}{
		{
			name: "valid timestamp",
			ts:   "1234567890.123456",
			want: time.Unix(1234567890, 123456000).UTC(),
		},
		{
			name: "valid timestamp with short microseconds",
			ts:   "1234567890.123",
			want: time.Unix(1234567890, 123000000).UTC(),
		},
		{
			name: "timestamp with zeros",
			ts:   "0.000000",
			want: time.Unix(0, 0).UTC(),
		},
		{
			name:    "empty timestamp",
			ts:      "",
			wantErr: true,
		},
		{
			name:    "missing decimal",
			ts:      "1234567890",
			wantErr: true,
		},
		{
			name:    "invalid seconds",
			ts:      "abc.123456",
			wantErr: true,
		},
		{
			name:    "invalid microseconds",
			ts:      "1234567890.xyz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSlackTimestamp(tt.ts)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSlackTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseSlackTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "short title unchanged",
			title: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "exactly 100 chars unchanged",
			title: "This is exactly one hundred characters long string that should not be truncated at all by function",
			want:  "This is exactly one hundred characters long string that should not be truncated at all by function",
		},
		{
			name:  "long title truncated",
			title: "This is a very long title that exceeds one hundred characters and should be truncated with ellipsis at the end",
			want:  "This is a very long title that exceeds one hundred characters and should be truncated with ellips...",
		},
		{
			name:  "newlines removed",
			title: "Hello\nworld\ntest",
			want:  "Hello world test",
		},
		{
			name:  "multiple spaces collapsed",
			title: "Hello    world",
			want:  "Hello world",
		},
		{
			name:  "empty title handled",
			title: "",
			want:  "(empty message)",
		},
		{
			name:  "whitespace only",
			title: "   \n\n   ",
			want:  "(empty message)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.title)
			if got != tt.want {
				t.Errorf("truncateTitle() = %q, want %q", got, tt.want)
			}
			// Verify length constraint
			if len(got) > 100 {
				t.Errorf("truncateTitle() returned string of length %d, want <= 100", len(got))
			}
		})
	}
}

func TestStripMrkdwn(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "user mention with name",
			text: "Hello <@U12345|john>, how are you?",
			want: "Hello @john, how are you?",
		},
		{
			name: "user mention without name",
			text: "Hello <@U12345>, how are you?",
			want: "Hello @user, how are you?",
		},
		{
			name: "channel mention with name",
			text: "Check out <#C12345|general>",
			want: "Check out #general",
		},
		{
			name: "channel mention without name",
			text: "Check out <#C12345>",
			want: "Check out #channel",
		},
		{
			name: "URL with text",
			text: "Visit <https://example.com|Example Site>",
			want: "Visit Example Site",
		},
		{
			name: "URL without text",
			text: "Visit <https://example.com>",
			want: "Visit https://example.com",
		},
		{
			name: "special mention here",
			text: "Attention <!here>",
			want: "Attention @here",
		},
		{
			name: "special mention channel",
			text: "Attention <!channel>",
			want: "Attention @channel",
		},
		{
			name: "multiple mentions",
			text: "<@U123|alice> and <@U456|bob> discussed <#C789|engineering>",
			want: "@alice and @bob discussed #engineering",
		},
		{
			name: "plain text unchanged",
			text: "Hello world",
			want: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMrkdwn(tt.text)
			if got != tt.want {
				t.Errorf("stripMrkdwn() = %q, want %q", got, tt.want)
			}
		})
	}
}
