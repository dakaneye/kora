package github

import (
	"encoding/json"
	"os"
	"testing"
)

func TestParsePRResponse_Success(t *testing.T) {
	data, err := os.ReadFile("testdata/pr_response.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	metadata, err := ParsePRResponse(json.RawMessage(data), "org/repo")
	if err != nil {
		t.Fatalf("ParsePRResponse failed: %v", err)
	}

	// Verify basic fields
	if metadata["repo"] != "org/repo" {
		t.Errorf("repo = %v, want org/repo", metadata["repo"])
	}
	if metadata["number"] != 123 {
		t.Errorf("number = %v, want 123", metadata["number"])
	}
	if metadata["state"] != "open" {
		t.Errorf("state = %v, want open", metadata["state"])
	}
	if metadata["author_login"] != "janedev" {
		t.Errorf("author_login = %v, want janedev", metadata["author_login"])
	}
	if metadata["is_draft"] != false {
		t.Errorf("is_draft = %v, want false", metadata["is_draft"])
	}
	if metadata["mergeable"] != "mergeable" {
		t.Errorf("mergeable = %v, want mergeable", metadata["mergeable"])
	}

	// Verify counts
	if metadata["additions"] != 250 {
		t.Errorf("additions = %v, want 250", metadata["additions"])
	}
	if metadata["deletions"] != 50 {
		t.Errorf("deletions = %v, want 50", metadata["deletions"])
	}
	if metadata["files_changed_count"] != 8 {
		t.Errorf("files_changed_count = %v, want 8", metadata["files_changed_count"])
	}
	if metadata["comments_count"] != 5 {
		t.Errorf("comments_count = %v, want 5", metadata["comments_count"])
	}
	if metadata["unresolved_threads"] != 2 {
		t.Errorf("unresolved_threads = %v, want 2", metadata["unresolved_threads"])
	}

	// Verify milestone
	if metadata["milestone"] != "v2.0" {
		t.Errorf("milestone = %v, want v2.0", metadata["milestone"])
	}

	// Verify labels
	labels, ok := metadata["labels"].([]string)
	if !ok {
		t.Fatalf("labels is not []string")
	}
	if len(labels) != 2 {
		t.Errorf("labels len = %d, want 2", len(labels))
	}
	if labels[0] != "enhancement" {
		t.Errorf("labels[0] = %v, want enhancement", labels[0])
	}

	// Verify assignees
	assignees, ok := metadata["assignees"].([]string)
	if !ok {
		t.Fatalf("assignees is not []string")
	}
	if len(assignees) != 2 {
		t.Errorf("assignees len = %d, want 2", len(assignees))
	}

	// Verify files_changed
	files, ok := metadata["files_changed"].([]map[string]any)
	if !ok {
		t.Fatalf("files_changed is not []map[string]any")
	}
	if len(files) != 2 {
		t.Errorf("files_changed len = %d, want 2", len(files))
	}
	if files[0]["path"] != "internal/models/event.go" {
		t.Errorf("files[0].path = %v, want internal/models/event.go", files[0]["path"])
	}

	// Verify review_requests with type differentiation
	reviewRequests, ok := metadata["review_requests"].([]map[string]any)
	if !ok {
		t.Fatalf("review_requests is not []map[string]any")
	}
	if len(reviewRequests) != 2 {
		t.Errorf("review_requests len = %d, want 2", len(reviewRequests))
	}
	// First is user
	if reviewRequests[0]["type"] != "user" {
		t.Errorf("review_requests[0].type = %v, want user", reviewRequests[0]["type"])
	}
	if reviewRequests[0]["login"] != "reviewer1" {
		t.Errorf("review_requests[0].login = %v, want reviewer1", reviewRequests[0]["login"])
	}
	// Second is team
	if reviewRequests[1]["type"] != "team" {
		t.Errorf("review_requests[1].type = %v, want team", reviewRequests[1]["type"])
	}
	if reviewRequests[1]["team_slug"] != "org/core-team" {
		t.Errorf("review_requests[1].team_slug = %v, want org/core-team", reviewRequests[1]["team_slug"])
	}

	// Verify reviews
	reviews, ok := metadata["reviews"].([]map[string]any)
	if !ok {
		t.Fatalf("reviews is not []map[string]any")
	}
	if len(reviews) != 2 {
		t.Errorf("reviews len = %d, want 2", len(reviews))
	}
	if reviews[0]["state"] != "approved" {
		t.Errorf("reviews[0].state = %v, want approved", reviews[0]["state"])
	}
	if reviews[1]["state"] != "changes_requested" {
		t.Errorf("reviews[1].state = %v, want changes_requested", reviews[1]["state"])
	}

	// Verify CI checks
	if metadata["ci_rollup"] != "failure" {
		t.Errorf("ci_rollup = %v, want failure", metadata["ci_rollup"])
	}
	ciChecks, ok := metadata["ci_checks"].([]map[string]any)
	if !ok {
		t.Fatalf("ci_checks is not []map[string]any")
	}
	if len(ciChecks) != 3 {
		t.Errorf("ci_checks len = %d, want 3", len(ciChecks))
	}
	// Find the failed test check
	var foundFailedTest bool
	for _, check := range ciChecks {
		if check["name"] == "test" && check["conclusion"] == "failure" {
			foundFailedTest = true
			break
		}
	}
	if !foundFailedTest {
		t.Error("expected to find failed test check")
	}

	// Verify linked issues
	linkedIssues, ok := metadata["linked_issues"].([]string)
	if !ok {
		t.Fatalf("linked_issues is not []string")
	}
	if len(linkedIssues) != 2 {
		t.Errorf("linked_issues len = %d, want 2", len(linkedIssues))
	}
}

func TestParsePRResponse_NotFound(t *testing.T) {
	data := []byte(`{"repository": {"pullRequest": null}}`)
	_, err := ParsePRResponse(json.RawMessage(data), "org/repo")
	if err == nil {
		t.Error("expected error for null pullRequest")
	}
}

func TestParsePRResponse_InvalidJSON(t *testing.T) {
	_, err := ParsePRResponse(json.RawMessage(`not json`), "org/repo")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParsePRResponse_BodyTruncation(t *testing.T) {
	// Create a response with a very long body
	longBody := make([]byte, 600)
	for i := range longBody {
		longBody[i] = 'a'
	}

	data := []byte(`{
		"repository": {
			"pullRequest": {
				"number": 1,
				"title": "test",
				"state": "OPEN",
				"isDraft": false,
				"body": "` + string(longBody) + `",
				"author": {"login": "test"},
				"assignees": {"nodes": []},
				"labels": {"nodes": []},
				"files": {"nodes": []},
				"reviewRequests": {"nodes": []},
				"reviews": {"nodes": []},
				"reviewThreads": {"nodes": []},
				"comments": {"totalCount": 0},
				"commits": {"nodes": []},
				"closingIssuesReferences": {"nodes": []}
			}
		}
	}`)

	metadata, err := ParsePRResponse(json.RawMessage(data), "org/repo")
	if err != nil {
		t.Fatalf("ParsePRResponse failed: %v", err)
	}

	body, ok := metadata["body"].(string)
	if !ok {
		t.Fatal("body is not string")
	}
	if len(body) > 500 {
		t.Errorf("body length = %d, want <= 500", len(body))
	}
}

func TestParsePRResponse_MissingOptionalFields(t *testing.T) {
	data := []byte(`{
		"repository": {
			"pullRequest": {
				"number": 1,
				"title": "test",
				"state": "OPEN",
				"isDraft": false,
				"body": "test",
				"author": {"login": "test"},
				"assignees": {"nodes": []},
				"labels": {"nodes": []},
				"files": {"nodes": []},
				"reviewRequests": {"nodes": []},
				"reviews": {"nodes": []},
				"reviewThreads": {"nodes": []},
				"comments": {"totalCount": 0},
				"commits": {"nodes": []},
				"closingIssuesReferences": {"nodes": []}
			}
		}
	}`)

	metadata, err := ParsePRResponse(json.RawMessage(data), "org/repo")
	if err != nil {
		t.Fatalf("ParsePRResponse failed: %v", err)
	}

	// Milestone should be missing
	if _, exists := metadata["milestone"]; exists {
		t.Error("expected milestone to be missing")
	}

	// ci_rollup should be missing (no commits with status)
	if _, exists := metadata["ci_rollup"]; exists {
		t.Error("expected ci_rollup to be missing")
	}
}
