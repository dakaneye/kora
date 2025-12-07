package github

import (
	"encoding/json"
	"os"
	"testing"
)

func TestParseIssueResponse_Success(t *testing.T) {
	data, err := os.ReadFile("testdata/issue_response.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	metadata, err := ParseIssueResponse(json.RawMessage(data), "org/repo")
	if err != nil {
		t.Fatalf("ParseIssueResponse failed: %v", err)
	}

	// Verify basic fields
	if metadata["repo"] != "org/repo" {
		t.Errorf("repo = %v, want org/repo", metadata["repo"])
	}
	if metadata["number"] != 456 {
		t.Errorf("number = %v, want 456", metadata["number"])
	}
	if metadata["state"] != "open" {
		t.Errorf("state = %v, want open", metadata["state"])
	}
	if metadata["author_login"] != "reporter1" {
		t.Errorf("author_login = %v, want reporter1", metadata["author_login"])
	}
	if metadata["comments_count"] != 3 {
		t.Errorf("comments_count = %v, want 3", metadata["comments_count"])
	}

	// Verify milestone
	if metadata["milestone"] != "Q1-2026" {
		t.Errorf("milestone = %v, want Q1-2026", metadata["milestone"])
	}

	// Verify labels
	labels, ok := metadata["labels"].([]string)
	if !ok {
		t.Fatalf("labels is not []string")
	}
	if len(labels) != 2 {
		t.Errorf("labels len = %d, want 2", len(labels))
	}
	if labels[0] != "bug" {
		t.Errorf("labels[0] = %v, want bug", labels[0])
	}

	// Verify assignees
	assignees, ok := metadata["assignees"].([]string)
	if !ok {
		t.Fatalf("assignees is not []string")
	}
	if len(assignees) != 1 {
		t.Errorf("assignees len = %d, want 1", len(assignees))
	}
	if assignees[0] != "developer1" {
		t.Errorf("assignees[0] = %v, want developer1", assignees[0])
	}

	// Verify comments
	comments, ok := metadata["comments"].([]map[string]any)
	if !ok {
		t.Fatalf("comments is not []map[string]any")
	}
	if len(comments) != 3 {
		t.Errorf("comments len = %d, want 3", len(comments))
	}
	if comments[0]["author"] != "developer1" {
		t.Errorf("comments[0].author = %v, want developer1", comments[0]["author"])
	}
	if comments[1]["body"] != "This is blocking the release. Please prioritize." {
		t.Errorf("comments[1].body = %v", comments[1]["body"])
	}

	// Verify reactions
	reactions, ok := metadata["reactions"].(map[string]int)
	if !ok {
		t.Fatalf("reactions is not map[string]int")
	}
	if reactions["+1"] != 2 {
		t.Errorf("reactions[+1] = %d, want 2", reactions["+1"])
	}
	if reactions["heart"] != 1 {
		t.Errorf("reactions[heart] = %d, want 1", reactions["heart"])
	}
	if reactions["eyes"] != 1 {
		t.Errorf("reactions[eyes] = %d, want 1", reactions["eyes"])
	}
	// -1 should not be present (count is 0)
	if _, exists := reactions["-1"]; exists {
		t.Error("reactions[-1] should not exist when count is 0")
	}

	// Verify timeline_summary
	timeline, ok := metadata["timeline_summary"].([]map[string]any)
	if !ok {
		t.Fatalf("timeline_summary is not []map[string]any")
	}
	if len(timeline) != 4 {
		t.Errorf("timeline_summary len = %d, want 4", len(timeline))
	}
	// First event is AssignedEvent
	if timeline[0]["type"] != "assigned" {
		t.Errorf("timeline[0].type = %v, want assigned", timeline[0]["type"])
	}
	if timeline[0]["actor"] != "pm1" {
		t.Errorf("timeline[0].actor = %v, want pm1", timeline[0]["actor"])
	}
	// Second and third are LabeledEvents
	if timeline[1]["type"] != "labeled" {
		t.Errorf("timeline[1].type = %v, want labeled", timeline[1]["type"])
	}
	if timeline[1]["label"] != "bug" {
		t.Errorf("timeline[1].label = %v, want bug", timeline[1]["label"])
	}
	// Fourth is CrossReferencedEvent with source URL
	if timeline[3]["type"] != "crossreferenced" {
		t.Errorf("timeline[3].type = %v, want crossreferenced", timeline[3]["type"])
	}
	if timeline[3]["source_url"] != "https://github.com/org/repo/pull/789" {
		t.Errorf("timeline[3].source_url = %v", timeline[3]["source_url"])
	}
}

func TestParseIssueResponse_NotFound(t *testing.T) {
	data := []byte(`{"repository": {"issue": null}}`)
	_, err := ParseIssueResponse(json.RawMessage(data), "org/repo")
	if err == nil {
		t.Error("expected error for null issue")
	}
}

func TestParseIssueResponse_InvalidJSON(t *testing.T) {
	_, err := ParseIssueResponse(json.RawMessage(`not json`), "org/repo")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseIssueResponse_BodyTruncation(t *testing.T) {
	// Create a response with a very long body
	longBody := make([]byte, 600)
	for i := range longBody {
		longBody[i] = 'b'
	}

	data := []byte(`{
		"repository": {
			"issue": {
				"number": 1,
				"title": "test",
				"state": "OPEN",
				"body": "` + string(longBody) + `",
				"author": {"login": "test"},
				"assignees": {"nodes": []},
				"labels": {"nodes": []},
				"comments": {"totalCount": 0, "nodes": []},
				"reactions": {"totalCount": 0},
				"reactionGroups": [],
				"timelineItems": {"nodes": []}
			}
		}
	}`)

	metadata, err := ParseIssueResponse(json.RawMessage(data), "org/repo")
	if err != nil {
		t.Fatalf("ParseIssueResponse failed: %v", err)
	}

	body, ok := metadata["body"].(string)
	if !ok {
		t.Fatal("body is not string")
	}
	if len(body) > 500 {
		t.Errorf("body length = %d, want <= 500", len(body))
	}
}

func TestParseIssueResponse_MissingOptionalFields(t *testing.T) {
	data := []byte(`{
		"repository": {
			"issue": {
				"number": 1,
				"title": "test",
				"state": "OPEN",
				"body": "test",
				"author": {"login": "test"},
				"assignees": {"nodes": []},
				"labels": {"nodes": []},
				"comments": {"totalCount": 0, "nodes": []},
				"reactions": {"totalCount": 0},
				"reactionGroups": [],
				"timelineItems": {"nodes": []}
			}
		}
	}`)

	metadata, err := ParseIssueResponse(json.RawMessage(data), "org/repo")
	if err != nil {
		t.Fatalf("ParseIssueResponse failed: %v", err)
	}

	// Milestone should be missing
	if _, exists := metadata["milestone"]; exists {
		t.Error("expected milestone to be missing")
	}

	// Empty slices should still be present
	comments, ok := metadata["comments"].([]map[string]any)
	if !ok {
		t.Fatal("comments is not []map[string]any")
	}
	if len(comments) != 0 {
		t.Errorf("comments len = %d, want 0", len(comments))
	}
}

func TestConvertReactionContent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"THUMBS_UP", "+1"},
		{"THUMBS_DOWN", "-1"},
		{"LAUGH", "laugh"},
		{"HOORAY", "hooray"},
		{"CONFUSED", "confused"},
		{"HEART", "heart"},
		{"ROCKET", "rocket"},
		{"EYES", "eyes"},
		{"UNKNOWN", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := convertReactionContent(tt.input)
			if result != tt.expected {
				t.Errorf("convertReactionContent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
